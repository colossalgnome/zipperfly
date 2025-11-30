-- MySQL test schema
CREATE TABLE IF NOT EXISTS downloads (
    id VARCHAR(255) PRIMARY KEY,
    bucket VARCHAR(255),
    objects JSON NOT NULL,
    name VARCHAR(255),
    callback VARCHAR(500),
    password VARCHAR(255),
    custom_headers JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Insert test data
INSERT IGNORE INTO downloads (id, bucket, objects, name, callback, password, custom_headers) VALUES
    ('test-basic', 'test-bucket', '["document.txt"]', 'basic-download', NULL, NULL, NULL),
    ('test-multi', 'test-bucket', '["document.txt", "data.json", "data.csv"]', 'multi-file-download', NULL, NULL, NULL),
    ('test-binary', 'test-bucket', '["binary.dat"]', 'binary-download', NULL, NULL, NULL),
    ('test-all', 'test-bucket', '["document.txt", "data.json", "data.csv", "binary.dat"]', 'all-files', NULL, NULL, NULL),
    ('test-missing', 'test-bucket', '["document.txt", "nonexistent.txt"]', 'with-missing', NULL, NULL, NULL),
    ('test-password', 'test-bucket', '["document.txt"]', 'password-protected', NULL, 'secret123', NULL),
    ('test-callback', 'test-bucket', '["document.txt"]', 'with-callback', 'http://localhost:8888/callback', NULL, NULL),
    ('test-headers', 'test-bucket', '["document.txt"]', 'custom-headers', NULL, NULL, '{"X-Custom-Header": "test-value", "X-Request-Source": "integration-test"}');
