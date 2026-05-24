-- Rollback: Drop idempotency_keys table

DROP FUNCTION IF EXISTS cleanup_expired_idempotency_keys();
DROP TABLE IF EXISTS idempotency_keys;
