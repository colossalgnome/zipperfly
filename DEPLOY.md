# Deployment Guide

This guide covers deploying zipperfly using Docker Compose with a complete production-ready stack.

## Architecture

The Docker Compose setup includes:

- **Caddy** - Reverse proxy with automatic HTTPS (Let's Encrypt)
- **Zipperfly** - Download service (Go application)
- **PostgreSQL** - Database for download records
- **MinIO** - S3-compatible object storage

```
                    ┌──────────────────┐
Internet ───────────▶   Caddy:80/443   │ (Automatic HTTPS)
                    └────────┬─────────┘
                             │
                    ┌────────▼─────────┐
                    │  Zipperfly:8080  │
                    └────┬─────────┬───┘
                         │         │
                ┌────────▼──┐  ┌───▼──────┐
                │ PostgreSQL│  │  MinIO   │
                └───────────┘  └──────────┘
```

## Quick Start

### 1. Configure Your Domain

Edit `Caddyfile` and replace `downloads.example.com` with your actual domain:

```caddyfile
yourdomain.com {
    reverse_proxy zipperfly:8080 {
        # ... configuration ...
    }
}
```

### 2. Set Environment Variables

Edit `docker-compose.yml` to configure:

**Security (Important!):**
- Change PostgreSQL password: `POSTGRES_PASSWORD`
- Change MinIO credentials: `MINIO_ROOT_USER`, `MINIO_ROOT_PASSWORD`
- Update zipperfly's database connection: `DB_URL`
- Update S3 credentials: `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`

**Optional Settings:**
- Enable signature verification: `ENFORCE_SIGNING=true` and set `SIGNING_SECRET`
- Configure metrics auth: `METRICS_USERNAME`, `METRICS_PASSWORD`
- Adjust resource limits: `MAX_ACTIVE_DOWNLOADS`, `MAX_FILES_PER_REQUEST`, `RATE_LIMIT_PER_IP`

### 3. Initialize Database Schema

The `init.sql` file will automatically create the `downloads` table on first startup with all optional columns.

**Schema Options:**
- **Full schema** (`init.sql`): Includes all optional columns (name, callback, password, custom_headers)
- **Minimal schema** (`init-minimal.sql`): Only required columns (id, bucket, objects)

Zipperfly automatically detects which columns exist at startup, so you can:
- Start with minimal schema and add columns later
- Skip features you don't need (e.g., omit password column if you don't use encryption)
- Use existing tables without modification

Review and modify the schema file based on your needs.

### 4. Launch Services

```bash
# Start all services in background
docker-compose up -d

# View logs
docker-compose logs -f

# Check service status
docker-compose ps
```

### 5. Upload Files to MinIO

Access MinIO console at `http://localhost:9001`:
- Login with credentials from docker-compose.yml (default: minioadmin/minioadmin)
- Create a bucket (e.g., "downloads")
- Upload your files

Or use the MinIO CLI:

```bash
# Install mc (MinIO client)
brew install minio/stable/mc  # macOS
# or download from https://min.io/docs/minio/linux/reference/minio-mc.html

# Configure
mc alias set local http://localhost:9000 minioadmin minioadmin

# Create bucket
mc mb local/downloads

# Upload files
mc cp /path/to/files/* local/downloads/
```

### 6. Create Download Records

```bash
# Connect to PostgreSQL
docker-compose exec postgres psql -U zipperfly -d zipperfly

# Insert a download record
INSERT INTO downloads (id, bucket, objects, name)
VALUES (
    gen_random_uuid(),
    'downloads',
    '["file1.pdf", "file2.txt"]'::jsonb,
    'my-download'
);

# Get the generated UUID
SELECT id, bucket, name FROM downloads ORDER BY created_at DESC LIMIT 1;
```

### 7. Test the Download

```bash
# Replace <uuid> with the ID from step 6
curl -o test.zip https://yourdomain.com/download/<uuid>

# Verify ZIP contents
unzip -l test.zip
```

## Production Checklist

Before deploying to production:

- [ ] **DNS**: Point your domain to the server
- [ ] **Firewall**: Allow ports 80 (HTTP) and 443 (HTTPS)
- [ ] **Secrets**: Change all default passwords and secrets
- [ ] **Backups**: Configure volume backups for `postgres-data` and `minio-data`
- [ ] **Monitoring**: Set up Prometheus to scrape `/metrics` endpoint
- [ ] **Logging**: Configure log aggregation (e.g., Loki, ELK stack)
- [ ] **Alerts**: Set up alerts for health check failures
- [ ] **Resources**: Adjust resource limits based on expected load
- [ ] **Testing**: Run integration tests against the deployment

## Scaling

### Horizontal Scaling

Run multiple zipperfly instances:

```bash
docker-compose up -d --scale zipperfly=3
```

Caddy will automatically load balance across all instances.

### Database Optimization

For high traffic, consider:
- Increasing `DB_MAX_CONNECTIONS` in docker-compose.yml
- Using managed PostgreSQL (AWS RDS, Google Cloud SQL, etc.)
- Adding read replicas for very high read loads (requires code changes)

### Storage Optimization

For production, consider:
- Using cloud S3 (AWS, R2, Spaces) instead of MinIO
- Enabling CDN for frequently accessed files
- Implementing lifecycle policies to archive old files

## Monitoring

### Health Checks

```bash
# Zipperfly health
curl https://yourdomain.com/health

# Expected response:
# {"status":"healthy","version":"1.0.0","checks":{"database":"ok","storage":"ok"}}
```

### Metrics

Access Prometheus metrics:

```bash
curl https://yourdomain.com/metrics
```

Key metrics to monitor:
- `zipperfly_downloads_total{status}` - Download success/failure rate
- `zipperfly_active_downloads` - Current load
- `zipperfly_missing_files_total` - Storage issues
- `zipperfly_requests_total{status_code}` - HTTP status distribution

### Logs

```bash
# View all logs
docker-compose logs -f

# View specific service
docker-compose logs -f zipperfly

# View last 100 lines
docker-compose logs --tail=100 zipperfly
```

## Maintenance

### Updating Zipperfly

```bash
# Rebuild image with latest code
docker-compose build zipperfly

# Restart service (zero-downtime with multiple instances)
docker-compose up -d --no-deps zipperfly
```

### Database Backup

```bash
# Backup PostgreSQL
docker-compose exec postgres pg_dump -U zipperfly zipperfly > backup.sql

# Restore
docker-compose exec -T postgres psql -U zipperfly -d zipperfly < backup.sql
```

### Storage Backup

```bash
# Backup MinIO data (use MinIO client)
mc mirror local/downloads /backup/minio/downloads/

# Or backup the Docker volume
docker run --rm \
  -v egress_minio-data:/data \
  -v $(pwd)/backup:/backup \
  alpine tar czf /backup/minio-data.tar.gz /data
```

### Cleanup Old Records

Add a cron job to clean up old download records:

```sql
-- Delete records older than 30 days
DELETE FROM downloads WHERE created_at < NOW() - INTERVAL '30 days';
```

## Troubleshooting

### Caddy Cannot Obtain Certificate

**Problem**: Let's Encrypt fails to issue certificate

**Solutions**:
- Ensure ports 80 and 443 are open and accessible from internet
- Verify DNS points to correct server IP
- Check Caddy logs: `docker-compose logs caddy`
- For testing, use HTTP by editing Caddyfile to use `http://yourdomain.com`

### Zipperfly Cannot Connect to Database

**Problem**: "database connect error"

**Solutions**:
- Verify database container is running: `docker-compose ps postgres`
- Check database credentials in docker-compose.yml match
- View database logs: `docker-compose logs postgres`
- Ensure `depends_on` health checks are working

### MinIO Connection Errors

**Problem**: "storage fetch error"

**Solutions**:
- Verify MinIO is healthy: `curl http://localhost:9000/minio/health/live`
- Check S3 credentials in docker-compose.yml
- Ensure bucket exists: `mc ls local/`
- Verify files are uploaded: `mc ls local/bucket-name/`

### High Memory Usage

**Problem**: Container using too much memory

**Solutions**:
- Reduce `MAX_CONCURRENT_FETCHES` (default: 10)
- Lower `MAX_ACTIVE_DOWNLOADS` limit
- Add memory limits to docker-compose.yml:
  ```yaml
  deploy:
    resources:
      limits:
        memory: 512M
  ```

## Security Hardening

### Enable HMAC Signing

Require signed requests to prevent unauthorized downloads:

```yaml
# docker-compose.yml
ENFORCE_SIGNING: "true"
SIGNING_SECRET: "your-secret-key-here"  # Use strong random key
```

Generate signatures in your application before redirecting users.

### Restrict File Extensions

Prevent serving executable files:

```yaml
BLOCKED_EXTENSIONS: ".exe,.sh,.bat,.cmd,.ps1"
```

Or whitelist only specific types:

```yaml
ALLOWED_EXTENSIONS: ".pdf,.jpg,.png,.txt,.zip"
```

### Add Rate Limiting

Protect against abuse:

```yaml
RATE_LIMIT_PER_IP: 10  # 10 requests/second per IP
```

### Use Secrets

For production, use Docker secrets instead of environment variables:

```yaml
secrets:
  db_password:
    file: ./secrets/db_password.txt
  s3_secret:
    file: ./secrets/s3_secret.txt

services:
  zipperfly:
    secrets:
      - db_password
      - s3_secret
```

## Alternative Configurations

### Using AWS S3

Replace MinIO with AWS S3:

```yaml
# docker-compose.yml - remove minio service
# Update zipperfly environment:
S3_ENDPOINT: https://s3.amazonaws.com
S3_REGION: us-east-1
S3_FORCE_PATH_STYLE: "false"
S3_ACCESS_KEY_ID: your-aws-key
S3_SECRET_ACCESS_KEY: your-aws-secret
```

### Using MySQL

Replace PostgreSQL with MySQL:

```yaml
# Comment out postgres service, uncomment mysql
DB_URL: mysql://zipperfly:password@mysql:3306/zipperfly
```

Don't forget to update init.sql with MySQL-compatible syntax.

### Using Redis

Use Redis for ultra-fast record lookups:

```yaml
DB_URL: redis://redis:6379/0
KEY_PREFIX: zipperfly_downloads_
```

Store records as JSON in Redis:

```bash
redis-cli SET zipperfly_downloads_<uuid> '{"bucket":"downloads","objects":["file1.pdf"],"name":"test"}'
```

## Support

- **Documentation**: See README.md and IMPLEMENTATION.md
- **Issues**: https://github.com/yourusername/zipperfly/issues
- **Metrics**: Monitor `/metrics` endpoint for insights
