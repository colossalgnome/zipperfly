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
- **Security Options**:
    - Optional HMAC signing and expiry for requests
    - Basic auth for /metrics endpoint
    - Password-protected ZIPs with AES-256 encryption
    - File extension filtering (allow/block lists)
- **Resource Limits**: Configurable max files per request and max file size
- **Custom Headers**: Per-request custom HTTP headers from database
- **TLS Support**: Automatic Let's Encrypt cert generation for standalone HTTPS.
- **Callbacks**: Optional POST callback on completion/error with retry logic.
- **Customization**: ENV-driven config for filename defaults, sanitization, key prefixes, etc.
- **Monitoring**: Prometheus /metrics endpoint with comprehensive metrics.

## Project Structure
```
zipperfly/
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
   git clone https://github.com/colossalgnome/zipperfly.git
   cd zipperfly
   ```

2. Build the binary:
   ```bash
   go mod tidy
   go build -o bin/zipperfly ./cmd/server
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
   ./bin/zipperfly

   # Using custom config file
   ./bin/zipperfly --config /path/to/config.env

   # Using environment variables only
   export DB_URL=postgres://...
   export S3_ENDPOINT=...
   ./bin/zipperfly
   ```

## Configuration
The server supports multiple configuration methods with the following priority:

1. **Command-line flag**: `--config /path/to/file.env`
2. **CONFIG_FILE env var**: `CONFIG_FILE=/path/to/file.env ./bin/zipperfly`
3. **.env file**: Automatically loaded if present in working directory
4. **OS environment variables**: Standard exported env vars

All settings are via environment variables. Required ones depend on your setup.

### Core
- `DB_URL`: Connection string (scheme determines backend: postgres/mysql/redis)
    - Postgres: `postgres://user:pass@host:5432/db` or `postgresql://...`
    - MySQL: `mysql://user:pass@host:3306/db`
    - Redis: `redis://localhost:6379/0`
- `DB_MAX_CONNECTIONS`: Maximum database connections (default: 20)
    - Small pool is efficient - each request only does one quick lookup by ID
    - Sizing: 5 (tiny), 10 (small), 20 (medium), 50 (large deployments)
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

### Resource Limits
- `MAX_ACTIVE_DOWNLOADS`: Maximum concurrent download requests (0 = unlimited, default: 0)
    - Protects server from overload during traffic spikes
    - Requests beyond limit receive 503 Service Unavailable
    - Example: `MAX_ACTIVE_DOWNLOADS=100`
- `MAX_FILES_PER_REQUEST`: Maximum number of files per download (0 = unlimited, default: 0)
- `RATE_LIMIT_PER_IP`: Rate limit per IP address in requests/second (0 = unlimited, default: 0)
    - Prevents abuse from individual clients
    - Uses token bucket algorithm (allows bursts of 1 request)
    - Requests exceeding limit receive 429 Too Many Requests
    - Example: `RATE_LIMIT_PER_IP=10` (10 requests/sec per IP)
    - Works with reverse proxies (checks X-Forwarded-For, X-Real-IP)

### File Extension Filtering
- `ALLOWED_EXTENSIONS`: Comma-separated list of allowed extensions (empty = allow all)
    - Example: `ALLOWED_EXTENSIONS=.pdf,.txt,.jpg`
    - If specified, only files with these extensions are included
- `BLOCKED_EXTENSIONS`: Comma-separated list of blocked extensions
    - Example: `BLOCKED_EXTENSIONS=.exe,.sh,.bat`
    - Takes precedence over allowed list

### Password-Protected ZIPs
- `ALLOW_PASSWORD_PROTECTED`: "true" to enable password-protected ZIPs (default: false)
    - Requires `password` field in download record
    - Uses AES-256 encryption for ZIP entries
    - Maintains streaming performance (no buffering)

### HTTPS & Let's Encrypt
- `ENABLE_HTTPS`: "true" for auto-TLS with Let's Encrypt
- `LETSENCRYPT_DOMAINS`: Comma-separated domains (e.g., "example.com")
- `LETSENCRYPT_CACHE_DIR`: Cert cache path (default: "./certs")
- `LETSENCRYPT_EMAIL`: Email for renewal notices

### Metrics
- `METRICS_USERNAME`: Username for basic auth on /metrics (optional)
- `METRICS_PASSWORD`: Password for basic auth on /metrics (optional)

### Docker

#### Quick Start with Docker Compose (Recommended)

The easiest way to run zipperfly is with Docker Compose, which includes:
- **Zipperfly** download service
- **PostgreSQL** database
- **MinIO** S3-compatible storage
- **Caddy** reverse proxy with automatic HTTPS

Note: In production you'd ideally use a redis instance your web application writes to, and point 
      at a legit cloud-based storage solution (S3, Backblaze, Cloudflare R2, DigitalOcean Spaces, etc)
      and only run zipperfly and a reverse proxy on your egress server. Use the docker-compose for local
      development and testing, or as a guide for getting started with your production config.

1. **Edit configuration:**
   ```bash
   # Edit docker-compose.yml and set your environment variables
   # Edit Caddyfile and replace 'downloads.example.com' with your domain
   nano docker-compose.yml
   nano Caddyfile
   ```

2. **Start all services:**
   ```bash
   docker-compose up -d
   ```

3. **Check logs:**
   ```bash
   docker-compose logs -f zipperfly
   ```

4. **Create a download record:**
   ```bash
   # Connect to PostgreSQL
   docker-compose exec postgres psql -U zipperfly -d zipperfly

   # Insert a test record
   INSERT INTO downloads (id, bucket, objects, name)
   VALUES (
     '01234567-89ab-cdef-0123-456789abcdef',
     'test-bucket',
     '["file1.txt", "file2.txt"]'::jsonb,
     'myfiles'
   );
   ```

5. **Access the service:**
   - Download endpoint: `https://downloads.example.com/download/01234567-89ab-cdef-0123-456789abcdef`
   - MinIO console: `http://localhost:9001` (admin UI for managing files)
   - Metrics: `https://downloads.example.com/metrics`

#### Standalone Docker

Build and run zipperfly as a standalone container:

```bash
# Build the image
docker build -t zipperfly .

# Run with environment file
docker run -d \
  --name zipperfly \
  -p 8080:8080 \
  --env-file .env \
  zipperfly

# Or with individual environment variables
docker run -d \
  --name zipperfly \
  -p 8080:8080 \
  -e DB_URL=postgres://user:pass@host:5432/db \
  -e S3_ENDPOINT=https://s3.amazonaws.com \
  -e S3_ACCESS_KEY_ID=your-key \
  -e S3_SECRET_ACCESS_KEY=your-secret \
  zipperfly
```

#### Production Deployment

For production with HTTPS:

1. **Update Caddyfile** with your domain:
   ```caddyfile
   yourdomain.com {
       reverse_proxy zipperfly:8080
   }
   ```

2. **Set environment variables** in docker-compose.yml (use secrets for sensitive values)

3. **Start services:**
   ```bash
   docker-compose up -d
   ```

Caddy will automatically obtain and renew Let's Encrypt certificates.

#### Using MySQL or Redis Instead

To use MySQL instead of PostgreSQL, edit docker-compose.yml:

```yaml
# Comment out postgres service, uncomment mysql
# Update DB_URL in zipperfly environment:
DB_URL: mysql://zipperfly:zipperfly@mysql:3306/zipperfly
```

For Redis:
```yaml
DB_URL: redis://redis:6379/0
KEY_PREFIX: zipperfly_downloads_
```

#### Scaling

Run multiple zipperfly instances behind Caddy:

```bash
docker-compose up -d --scale zipperfly=3
```

Caddy will automatically load balance across all instances.

## Usage
1. **Prep Record**: In your app (e.g., Laravel), insert a record with ID (e.g., UUID), bucket, objects (array of keys),
   optional name/callback/password/custom_headers.

   **Basic Example (Postgres):**
   ```sql
   INSERT INTO downloads (id, bucket, objects, name, callback)
   VALUES (
     '019ad1fc-a742-709e-81e2-59eff89576a5',
     'my-bucket',
     '["file1.txt", "file2.txt"]'::jsonb,
     'myfiles',
     'https://callback.example.com'
   );
   ```

   **With Password Protection:**
   ```sql
   INSERT INTO downloads (id, bucket, objects, name, password)
   VALUES (
     '019ad1fc-a742-709e-81e2-59eff89576a5',
     'my-bucket',
     '["file1.txt", "file2.txt"]'::jsonb,
     'secure-files',
     'MySecretPassword123'
   );
   ```

   **With Custom Headers (e.g., CDN caching):**
   ```sql
   INSERT INTO downloads (id, bucket, objects, name, custom_headers)
   VALUES (
     '019ad1fc-a742-709e-81e2-59eff89576a5',
     'my-bucket',
     '["file1.txt", "file2.txt"]'::jsonb,
     'cached-files',
     '{"Cache-Control": "max-age=3600", "X-Custom-Header": "value"}'::jsonb
   );
   ```

2. **Generate URL**: Optionally sign/expire, then send to client (e.g., redirect or JS location.href).
   Example (unsigned): `https://your-egress.com/019ad1fc-a742-709e-81e2-59eff89576a5`
   Signed: Add `?expiry=1764460487&signature=...`

3. **Client Download**: Browser GET triggers stream. Callback (if set) POSTs status on finish.

## Record Schema

### Required Columns/Fields
The following fields are **required** in your database:
- `id` - Unique identifier (UUID or custom field name via `ID_FIELD`)
- `bucket` - Storage bucket name (text)
- `objects` - Array of file paths (JSON/JSONB for SQL, array for Redis)

### Optional Columns/Fields
The following fields are **optional** - zipperfly will detect which columns exist and adapt:
- `name` - Custom ZIP filename (text, optional)
- `callback` - Webhook URL for completion notification (text, optional)
- `password` - ZIP password for encryption (text, optional)
- `custom_headers` - HTTP response headers (JSON/JSONB map, optional)

**Backward Compatibility:** You can use a minimal schema with just `id`, `bucket`, and `objects`. Zipperfly automatically detects which optional columns exist at startup and only queries available columns. This means you can start with a simple schema and add optional columns later without code changes.

**For SQL (PostgreSQL/MySQL)**:
```sql
-- Minimal schema (works fine!)
CREATE TABLE downloads (
    id UUID PRIMARY KEY,
    bucket TEXT NOT NULL,
    objects JSONB NOT NULL
);

-- Full schema (with all features)
CREATE TABLE downloads (
    id UUID PRIMARY KEY,
    bucket TEXT NOT NULL,
    objects JSONB NOT NULL,
    name TEXT,
    callback TEXT,
    password TEXT,
    custom_headers JSONB
);
```

**For Redis**: JSON object with keys "bucket", "objects" (array), and optionally "name", "callback", "password", "custom_headers".

**Field Meanings**:
- `bucket`: For S3, the bucket name (required). For local storage, optional path prefix within `STORAGE_PATH`.
- `objects`: Array of object keys/file paths to include in ZIP.
- `name`: Optional custom filename for the ZIP (without .zip extension).
- `callback`: Optional HTTP endpoint to POST completion status.
- `password`: Optional password for ZIP encryption (requires `ALLOW_PASSWORD_PROTECTED=true`).
- `custom_headers`: Optional map of custom HTTP headers to include in the response (e.g., `{"Cache-Control": "max-age=3600"}`).

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