-- init.sql - PostgreSQL schema initialization for zipperfly
-- This file is automatically executed when the database container starts

-- Create downloads table
CREATE TABLE IF NOT EXISTS downloads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bucket TEXT NOT NULL,
    objects JSONB NOT NULL,
    name TEXT,
    callback TEXT,
    password TEXT,
    custom_headers JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create index on created_at for cleanup queries
CREATE INDEX IF NOT EXISTS idx_downloads_created_at ON downloads(created_at);

-- Example: Insert a test record (optional - remove in production)
-- INSERT INTO downloads (id, bucket, objects, name)
-- VALUES (
--     '01234567-89ab-cdef-0123-456789abcdef',
--     'test-bucket',
--     '["file1.txt", "file2.txt"]'::jsonb,
--     'example-download'
-- );

-- Optional: Create a cleanup function for old records
-- CREATE OR REPLACE FUNCTION cleanup_old_downloads()
-- RETURNS void AS $$
-- BEGIN
--     DELETE FROM downloads WHERE created_at < NOW() - INTERVAL '30 days';
-- END;
-- $$ LANGUAGE plpgsql;

-- Optional: Create a trigger to update updated_at
-- CREATE OR REPLACE FUNCTION update_updated_at_column()
-- RETURNS TRIGGER AS $$
-- BEGIN
--     NEW.updated_at = NOW();
--     RETURN NEW;
-- END;
-- $$ LANGUAGE plpgsql;
--
-- CREATE TRIGGER update_downloads_updated_at
-- BEFORE UPDATE ON downloads
-- FOR EACH ROW
-- EXECUTE FUNCTION update_updated_at_column();
