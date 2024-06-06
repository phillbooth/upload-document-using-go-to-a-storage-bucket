package main

import (
    "bytes"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "mime/multipart"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "time"

    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/credentials"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/s3"
    "github.com/joho/godotenv"
    "github.com/labstack/echo/v4"
    "github.com/labstack/echo/v4/middleware"
)

const MaxFileSize = 1.5 * 1024 * 1024 // 1.5 MB

var AllowedExtensions = map[string]bool{
    ".pdf":  true,
    ".doc":  true,
    ".docx": true,
    ".odt":  true,
    ".rtf":  true,
    ".wps":  true,
    ".wpd":  true,
}

var (
    DoSpacesEndpoint     string
    DoSpacesRegion       string
    DoSpacesAccessKey    string
    DoSpacesSecretKey    string
    DoSpacesBucketName   string
    DoSecretKeyDoFunctions string
    s3Client             *s3.S3
)

func init() {
    if err := godotenv.Load(); err != nil {
        log.Fatal("Error loading .env file")
    }

    DoSpacesEndpoint = os.Getenv("DO_SPACES_ENDPOINT")
    DoSpacesRegion = os.Getenv("DO_SPACES_REGION")
    DoSpacesAccessKey = os.Getenv("DO_SPACES_ACCESS_KEY")
    DoSpacesSecretKey = os.Getenv("DO_SPACES_SECRET_KEY")
    DoSpacesBucketName = os.Getenv("DO_SPACES_BUCKET_NAME")
    DoSecretKeyDoFunctions = os.Getenv("DO_SECRET_KEY_DO_FUNCTIONS")

    sess := session.Must(session.NewSession(&aws.Config{
        Region:      aws.String(DoSpacesRegion),
        Endpoint:    aws.String(DoSpacesEndpoint),
        Credentials: credentials.NewStaticCredentials(DoSpacesAccessKey, DoSpacesSecretKey, ""),
    }))
    s3Client = s3.New(sess)
}

func allowedFile(filename string) bool {
    ext := strings.ToLower(filepath.Ext(filename))
    return AllowedExtensions[ext]
}

func generateToken(userUUID, filePath, fileURL string) string {
    data := fmt.Sprintf("%s:%s:%s", userUUID, filePath, fileURL)
    h := hmac.New(sha256.New, []byte(DoSecretKeyDoFunctions))
    h.Write([]byte(data))
    return hex.EncodeToString(h.Sum(nil))
}

func scanFileForViruses(filePath string) bool {
    cmd := exec.Command("clamdscan", filePath)
    var out bytes.Buffer
    cmd.Stdout = &out
    err := cmd.Run()
    if err != nil {
        fmt.Println("Error scanning file:", err)
        return false
    }
    return strings.Contains(out.String(), "OK")
}

func convertToPDF(filePath string) (string, error) {
    outputPath := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".pdf"
    cmd := exec.Command("libreoffice", "--headless", "--convert-to", "pdf", filePath, "--outdir", filepath.Dir(filePath))
    err := cmd.Run()
    if err != nil {
        return "", err
    }
    os.Remove(filePath)
    return outputPath, nil
}

func uploadToS3(filePath string) (string, error) {
    file, err := os.Open(filePath)
    if err != nil {
        return "", err
    }
    defer file.Close()

    key := filepath.Base(filePath)
    _, err = s3Client.PutObject(&s3.PutObjectInput{
        Bucket: aws.String(DoSpacesBucketName),
        Key:    aws.String(key),
        Body:   file,
        ACL:    aws.String("public-read"),
    })
    if err != nil {
        return "", err
    }

    fileURL := fmt.Sprintf("%s/%s/%s", DoSpacesEndpoint, DoSpacesBucketName, key)
    return fileURL, nil
}

func uploadFileHandler(c echo.Context) error {
    userUUID := c.FormValue("userUUID")
    firstName := c.FormValue("firstName")
    lastName := c.FormValue("lastName")

    if userUUID == "" || firstName == "" || lastName == "" {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "User UUID, First Name, and Last Name are required"})
    }

    file, err := c.FormFile("cvFile")
    if err != nil {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "No file part"})
    }

    if file.Filename == "" {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "No selected file"})
    }

    if !allowedFile(file.Filename) {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "File type not allowed"})
    }

    src, err := file.Open()
    if err != nil {
        return err
    }
    defer src.Close()

    tempDir, err := os.MkdirTemp("", "upload")
    if err != nil {
        return err
    }
    defer os.RemoveAll(tempDir)

    timestamp := time.Now().Format("2006-01-02-15-04-05")
    sanitizedFirstName := strings.ReplaceAll(firstName, " ", "_")
    sanitizedLastName := strings.ReplaceAll(lastName, " ", "_")
    newFilename := fmt.Sprintf("%s-%s-%s%s", sanitizedFirstName, sanitizedLastName, timestamp, filepath.Ext(file.Filename))
    filePath := filepath.Join(tempDir, newFilename)

    dst, err := os.Create(filePath)
    if err != nil {
        return err
    }
    defer dst.Close()

    if _, err = io.Copy(dst, src); err != nil {
        return err
    }

    fileInfo, err := dst.Stat()
    if err != nil {
        return err
    }

    if fileInfo.Size() > MaxFileSize {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "File size exceeds limit"})
    }

    if !scanFileForViruses(filePath) {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "File might be infected"})
    }

    if filepath.Ext(filePath) != ".pdf" {
        filePath, err = convertToPDF(filePath)
        if err != nil {
            return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to convert file to PDF"})
        }
    }

    fileURL, err := uploadToS3(filePath)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to upload file to S3"})
    }

    token := generateToken(userUUID, filePath, fileURL)
    return c.JSON(http.StatusOK, map[string]interface{}{
        "file_url":  fileURL,
        "file_path": filePath,
        "token":     token,
    })
}

func main() {
    e := echo.New()
    e.Use(middleware.Logger())
    e.Use(middleware.Recover())

    e.POST("/upload", uploadFileHandler)

    e.Start(":8080")
}
