
## Usage

Run the Go application:
- `go run main.go`

## Endpoints

### POST /upload

Upload a document, scan it for viruses, convert it to PDF, rename it, and upload it to DigitalOcean Spaces.

**Request Parameters:**

- `cvFile`: The file to be uploaded (multipart/form-data)
- `userUUID`: The user's UUID (form-data)
- `firstName`: The user's first name (form-data)
- `lastName`: The user's last name (form-data)

**Response:**

- `file_url`: The URL of the uploaded file
- `file_path`: The path of the uploaded file
- `token`: HMAC token for the file

**Example:**

```bash
curl -X POST http://localhost:8080/upload \
  -F "cvFile=@path/to/your/file.docx" \
  -F "userUUID=some-uuid" \
  -F "firstName=John" \
  -F "lastName=Doe"


------------------

DO_SPACES_ENDPOINT=<your-digitalocean-spaces-endpoint>
DO_SPACES_REGION=<your-digitalocean-spaces-region>
DO_SPACES_ACCESS_KEY=<your-digitalocean-spaces-access-key>
DO_SPACES_SECRET_KEY=<your-digitalocean-spaces-secret-key>
DO_SPACES_BUCKET_NAME=<your-digitalocean-spaces-bucket-name>
DO_SECRET_KEY_DO_FUNCTIONS=<your-secret-key-for-hmac>

-----------------


## Code Explanation
### Imports and Setup

    Import necessary packages for handling HTTP requests, file operations, AWS S3 integration, and environment variables.
    Load environment variables from the .env file using godotenv.

## Constants and Environment Variables

    Define the maximum file size and allowed file extensions.
    Load environment variables for DigitalOcean Spaces credentials and other configurations.

### S3 Client

    Initialize the DigitalOcean Spaces (S3) client using AWS SDK.

## Helper Functions
### File Extension Check

    Check if the file extension is allowed by comparing it to a predefined list of allowed extensions.

##H MAC Token Generation

    Generate an HMAC token for the file using user UUID, file path, and file URL.

## Virus Scanning

    Scan the file for viruses using clamdscan by executing a shell command.

## PDF Conversion

    Convert the file to PDF using LibreOffice by executing a shell command.
    Remove the original file after conversion.

## File Upload to S3

    Upload the file to DigitalOcean Spaces using the S3 client.
    Return the file URL after successful upload.

### Main Endpoint
## /upload Route

    Handle file upload requests.
    Validate and sanitize input.
    Save the file to a temporary directory.
    Check the file size.
    Scan the file for viruses.
    Convert the file to PDF if necessary.
    Upload the file to DigitalOcean Spaces.
    Generate and return a response with the file URL, path, and HMAC token.

### Running the Application

    Start the Echo web server on port 8080.
