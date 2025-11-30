# Zipper Fly Egress Server

A lightweight, standalone Go microservice for streaming ZIP archives on-the-fly from S3-compatible storage or local
filesystems. It fetches a list of objects based on a record stored in a database (Postgres/MySQL) or Redis, zips them
without buffering to disk, and streams the response to the client. Ideal for download endpoints where you want to avoid
proxying large files through your main app.

## Features
- **On-the-Fly Zipping**: Streams multiple files into a ZIP response with constant memory usage (parallel fetches with
  bounded concurrency).
- **Multiple Storage Backends**:
    - **S3-Compatible**: AWS S3, Cloudflare R2, MinIO, DigitalOcean Spaces, etc.
    - **Local Filesystem**: NFS, Samba, or any mounted filesystem
- **Database Backends**: Supports Postgres, MySQL, or Redis for record storage (file list, bucket, etc.).
- **Security Options**: Optional HMAC signing and expiry for requests; basic auth for /metrics.
- **TLS Support**: Automatic Let's Encrypt cert generation for standalone HTTPS.
- **Callbacks**: Optional POST callback on completion/error.
- **Customization**: ENV-driven config for filename defaults, sanitization, key prefixes, etc.
- **Monitoring**: Prometheus /metrics endpoint.

## Project Structure
```
egress/
├── cmd/
│   └── server/           # Application entry point
│       └── main.go
├── internal/
│   ├── auth/            # Signature verification
│   ├── config/          # Configuration loading
│   ├── database/        # Database backends (postgres, mysql, redis)
│   ├── handlers/        # HTTP handlers and middleware
│   ├── metrics/         # Prometheus metrics
│   ├── models/          # Data structures
│   ├── server/          # HTTP server setup
│   └── storage/         # S3 client initialization
├── .env.example         # Example configuration
└── README.md
```

## Installation
1. Clone the repo:
   ```bash
   git clone https://github.com/yourusername/s3-zip-egress.git
   cd s3-zip-egress
   ```

2. Build the binary:
   ```bash
   go mod tidy
   go build -o bin/egress ./cmd/server
   ```

3. Configure:
   ```bash
   # Copy example config
   cp .env.example .env

   # Edit with your settings
   nano .env
   ```

4. Run:
   ```bash
   # Using .env file (automatic)
   ./bin/egress

   # Using custom config file
   ./bin/egress --config /path/to/config.env

   # Using environment variables only
   export DB_URL=postgres://...
   export S3_ENDPOINT=...
   ./bin/egress
   ```

## Configuration
The server supports multiple configuration methods with the following priority:

1. **Command-line flag**: `--config /path/to/file.env`
2. **CONFIG_FILE env var**: `CONFIG_FILE=/path/to/file.env ./bin/egress`
3. **.env file**: Automatically loaded if present in working directory
4. **OS environment variables**: Standard exported env vars

All settings are via environment variables. Required ones depend on your setup.

### Core
- `DB_URL`: Connection string (scheme determines backend: postgres/mysql/redis)
    - Postgres: `postgres://user:pass@host:5432/db` or `postgresql://...`
    - MySQL: `mysql://user:pass@host:3306/db`
    - Redis: `redis://localhost:6379/0`
- `TABLE_NAME`: SQL table name (default: "downloads")
- `ID_FIELD`: SQL column for ID lookup (default: "id")
- `KEY_PREFIX`: Redis key prefix (e.g., "laravel_downloads_")

### Storage Configuration

You can use either S3-compatible storage or local filesystem storage.

**Storage Type** (auto-detected if not specified):
- `STORAGE_TYPE`: Either "s3" or "local"
    - Defaults to "s3" if `S3_ENDPOINT` or `S3_ACCESS_KEY_ID` is set
    - Defaults to "local" if `STORAGE_PATH` is set

**S3-Compatible Storage**:
- `S3_ENDPOINT`: Custom S3 endpoint (e.g., "https://abc123.r2.cloudflarestorage.com" for R2)
- `S3_REGION`: Region (default: "auto")
- `S3_FORCE_PATH_STYLE`: Set to "true" for path-style access (e.g., for MinIO); default "false"
- `S3_ACCESS_KEY_ID`: Access key
- `S3_SECRET_ACCESS_KEY`: Secret key

**Local Filesystem Storage**:
- `STORAGE_PATH`: Base directory path (e.g., "/mnt/files" or "/var/data")
    - The "bucket" field in database records is optional for local storage
    - If set, bucket is treated as a path prefix: e.g., `foo/bar/baz` within `STORAGE_PATH`
    - Example: If `STORAGE_PATH=/mnt/files`, bucket is `uploads/2024`, and object is `file.pdf`, the full path
      is `/mnt/files/uploads/2024/file.pdf`
    - If bucket is empty, files are read directly from `STORAGE_PATH`
    - Path traversal is prevented for security

### Security & Features
- `ENFORCE_SIGNING`: "true" to require signatures (default: false)
- `SIGNING_SECRET`: Shared secret for HMAC
- `APPEND_YMD`: "true" to append "-YYYYMMDD" to default filenames
- `SANITIZE_FILENAMES`: "true" to clean object names in ZIP
- `IGNORE_MISSING`: "true" to skip missing files instead of failing (default: false)
    - If false: download fails on first missing file
    - If true: skips missing files, creates ZIP with available files only
    - Only fails if ALL requested files are missing
- `MAX_CONCURRENT_FETCHES`: Max parallel fetches per request (default: 10)
- `PORT`: Listen port (default: 8080; 443 for HTTPS)

### HTTPS & Let's Encrypt
- `ENABLE_HTTPS`: "true" for auto-TLS with Let's Encrypt
- `LETSENCRYPT_DOMAINS`: Comma-separated domains (e.g., "example.com")
- `LETSENCRYPT_CACHE_DIR`: Cert cache path (default: "./certs")
- `LETSENCRYPT_EMAIL`: Email for renewal notices

### Metrics
- `METRICS_USERNAME`: Username for basic auth on /metrics (optional)
- `METRICS_PASSWORD`: Password for basic auth on /metrics (optional)

### Docker
Build and run:
```
docker build -t s3-zip-egress .
docker run -p 8080:8080 --env-file .env s3-zip-egress
```

For HTTPS, expose 80/443 and persist certs volume.

## Usage
1. **Prep Record**: In your app (e.g., Laravel), insert a record with ID (e.g., UUID), bucket, objects (array of keys),
   optional name/callback.
   Example (Postgres):
   ```
   INSERT INTO downloads (id, bucket, objects, name, callback)
        VALUES (
          '019ad1fc-a742-709e-81e2-59eff89576a5',
          'my-bucket',
          '["file1.txt", "file2.txt"]'::jsonb,
          'myfiles.zip',
          'https://callback.example.com'
        );
   ```

2. **Generate URL**: Optionally sign/expire, then send to client (e.g., redirect or JS location.href).
   Example (unsigned): `https://your-egress.com/019ad1fc-a742-709e-81e2-59eff89576a5`
   Signed: Add `?expiry=1764460487&signature=...`

3. **Client Download**: Browser GET triggers stream. Callback (if set) POSTs status on finish.

## Record Schema
**For SQL**: Columns `id` (or custom), `bucket` (text), `objects` (jsonb/text), `name` (text, optional), `callback`
(text, optional).

**For Redis**: JSON object with keys "bucket", "objects" (array), "name", "callback".

**Field Meanings**:
- `bucket`: For S3, the bucket name (required). For local storage, optional path prefix within `STORAGE_PATH`.
- `objects`: Array of object keys/file paths to include in ZIP.

Extra fields are ignored.

### Examples for Local Storage
If using local filesystem storage with `STORAGE_PATH=/mnt/files`:

**With bucket prefix:**
```sql
INSERT INTO downloads (id, bucket, objects, name)
VALUES (
           '019ad1fc-a742-709e-81e2-59eff89576a5',
           'uploads/2024',  -- files are in /mnt/files/uploads/2024/
           '["document.pdf", "photo.jpg"]'::jsonb,
           'myfiles'
       );
```

**Without bucket (files in root of STORAGE_PATH):**
```sql
INSERT INTO downloads (id, bucket, objects, name)
VALUES (
           '019ad1fc-a742-709e-81e2-59eff89576a5',
           '',  -- or NULL - files are in /mnt/files/
           '["document.pdf", "photo.jpg"]'::jsonb,
           'myfiles'
       );
```

## Monitoring

The server exposes Prometheus metrics on the `/metrics` endpoint. See [METRICS.md](METRICS.md) for detailed
documentation.

**Key metrics:**
- `zipperfly_downloads_total{status}` - Download outcomes (completed, partial, failed)
- `zipperfly_files_fetch_total{result}` - Individual file fetch results (success, missing, error)
- `zipperfly_missing_files_total` - Count of missing files encountered
- `zipperfly_request_duration_seconds` - Request latency
- `zipperfly_outgoing_bytes` / `zipperfly_incoming_bytes` - Bandwidth tracking

## Deployment Notes
- **Scaling**: Stateless; run multiple instances behind a load balancer.
- **Logs**: Structured logging via Zap (JSON format in production).
- **Metrics**: Prometheus metrics on `/metrics` endpoint (see METRICS.md).
- **Concurrency**: Default 10 concurrent fetches per request; adjust with `MAX_CONCURRENT_FETCHES`.

## Contributing
Fork, PRs welcome! Issues for bugs/features.

## License
MIT License. See LICENSE file.