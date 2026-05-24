-- Migration: Create audit_log table
-- Description: Compliance audit trail for regulatory requirements

CREATE TABLE IF NOT EXISTS audit_log (
    id VARCHAR(36) PRIMARY KEY,
    entity_type VARCHAR(50) NOT NULL,
    entity_id VARCHAR(36) NOT NULL,
    action VARCHAR(50) NOT NULL,
    actor_id VARCHAR(36),
    actor_type VARCHAR(20) NOT NULL DEFAULT 'user',
    old_values JSONB,
    new_values JSONB,
    ip_address INET,
    user_agent TEXT,
    request_id VARCHAR(36),
    trace_id VARCHAR(36),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_entity_type CHECK (entity_type IN ('user', 'account', 'transaction')),
    CONSTRAINT chk_actor_type CHECK (actor_type IN ('user', 'system', 'admin'))
);

-- Indexes for audit queries
CREATE INDEX idx_audit_log_entity ON audit_log(entity_type, entity_id);
CREATE INDEX idx_audit_log_actor ON audit_log(actor_id) WHERE actor_id IS NOT NULL;
CREATE INDEX idx_audit_log_action ON audit_log(action);
CREATE INDEX idx_audit_log_created_at ON audit_log(created_at DESC);
CREATE INDEX idx_audit_log_request_id ON audit_log(request_id) WHERE request_id IS NOT NULL;
CREATE INDEX idx_audit_log_trace_id ON audit_log(trace_id) WHERE trace_id IS NOT NULL;

-- Composite index for entity history
CREATE INDEX idx_audit_log_entity_created ON audit_log(entity_type, entity_id, created_at DESC);

-- Partitioning hint: Consider partitioning by created_at for large datasets
-- ALTER TABLE audit_log PARTITION BY RANGE (created_at);

COMMENT ON TABLE audit_log IS 'Immutable audit trail for compliance and regulatory requirements';
COMMENT ON COLUMN audit_log.old_values IS 'Previous state of the entity (for updates)';
COMMENT ON COLUMN audit_log.new_values IS 'New state of the entity';
COMMENT ON COLUMN audit_log.actor_type IS 'Type of actor: user, system, admin';
