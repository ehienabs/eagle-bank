-- Migration: Create idempotency_keys table
-- Description: Idempotency tracking for API requests to prevent duplicate processing

CREATE TABLE IF NOT EXISTS idempotency_keys (
    id VARCHAR(36) PRIMARY KEY,
    key VARCHAR(255) NOT NULL UNIQUE,
    user_id VARCHAR(36) NOT NULL,
    request_path VARCHAR(255) NOT NULL,
    request_method VARCHAR(10) NOT NULL,
    request_hash VARCHAR(64) NOT NULL,
    response_code INTEGER,
    response_body JSONB,
    locked_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,

    CONSTRAINT chk_request_method CHECK (request_method IN ('POST', 'PUT', 'PATCH', 'DELETE'))
);

-- Indexes for idempotency lookups
CREATE INDEX idx_idempotency_keys_key ON idempotency_keys(key);
CREATE INDEX idx_idempotency_keys_user ON idempotency_keys(user_id);
CREATE INDEX idx_idempotency_keys_expires ON idempotency_keys(expires_at);
CREATE INDEX idx_idempotency_keys_created ON idempotency_keys(created_at);

-- Composite index for quick key lookup by user
CREATE INDEX idx_idempotency_keys_user_key ON idempotency_keys(user_id, key);

-- Function to clean up expired idempotency keys
CREATE OR REPLACE FUNCTION cleanup_expired_idempotency_keys()
RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    DELETE FROM idempotency_keys
    WHERE expires_at < NOW()
    RETURNING 1 INTO deleted_count;

    RETURN COALESCE(deleted_count, 0);
END;
$$ LANGUAGE plpgsql;

COMMENT ON TABLE idempotency_keys IS 'Idempotency tracking for safe request retries';
COMMENT ON COLUMN idempotency_keys.key IS 'Client-provided idempotency key';
COMMENT ON COLUMN idempotency_keys.request_hash IS 'SHA-256 hash of request body for validation';
COMMENT ON COLUMN idempotency_keys.locked_at IS 'Timestamp when processing started (for concurrent request handling)';
COMMENT ON COLUMN idempotency_keys.completed_at IS 'Timestamp when processing completed';
COMMENT ON COLUMN idempotency_keys.expires_at IS 'When this idempotency key expires (typically 24 hours)';
