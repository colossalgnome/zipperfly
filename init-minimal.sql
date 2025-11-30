-- init-minimal.sql - Minimal PostgreSQL schema for zipperfly
-- This demonstrates that zipperfly works with just the required columns
-- Optional columns (name, callback, password, custom_headers) are not needed

-- Create downloads table with only required columns
CREATE TABLE IF NOT EXISTS downloads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bucket TEXT NOT NULL,
    objects JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create index on created_at for cleanup queries
CREATE INDEX IF NOT EXISTS idx_downloads_created_at ON downloads(created_at);

-- Example: Insert a test record
-- INSERT INTO downloads (id, bucket, objects)
-- VALUES (
--     '01234567-89ab-cdef-0123-456789abcdef',
--     'test-bucket',
--     '["file1.txt", "file2.txt"]'::jsonb
-- );

-- Note: This minimal schema works perfectly with zipperfly!
-- The application will detect that optional columns don't exist and skip them.
-- All optional features (passwords, callbacks, custom headers, custom names) will be disabled,
-- but core ZIP download functionality works fine.
