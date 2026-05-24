-- Migration: Create events table
-- Description: Event store for event sourcing and audit trail

CREATE TABLE IF NOT EXISTS events (
    id VARCHAR(36) PRIMARY KEY,
    aggregate_id VARCHAR(36) NOT NULL,
    aggregate_type VARCHAR(50) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    event_version INTEGER NOT NULL DEFAULT 1,
    payload JSONB NOT NULL,
    metadata JSONB,
    published BOOLEAN NOT NULL DEFAULT FALSE,
    published_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_aggregate_type CHECK (aggregate_type IN ('user', 'account', 'transaction'))
);

-- Indexes for event sourcing queries
CREATE INDEX idx_events_aggregate ON events(aggregate_id, aggregate_type);
CREATE INDEX idx_events_aggregate_version ON events(aggregate_id, event_version);
CREATE INDEX idx_events_event_type ON events(event_type);
CREATE INDEX idx_events_created_at ON events(created_at DESC);
CREATE INDEX idx_events_published ON events(published) WHERE published = FALSE;
CREATE INDEX idx_events_payload ON events USING GIN (payload);

-- Composite index for replay queries
CREATE INDEX idx_events_aggregate_created ON events(aggregate_id, created_at);

COMMENT ON TABLE events IS 'Event store for domain events (event sourcing)';
COMMENT ON COLUMN events.aggregate_id IS 'ID of the aggregate (user, account, transaction)';
COMMENT ON COLUMN events.aggregate_type IS 'Type of aggregate: user, account, transaction';
COMMENT ON COLUMN events.event_type IS 'Type of event: user.created, transaction.completed, etc.';
COMMENT ON COLUMN events.payload IS 'Event data as JSON';
COMMENT ON COLUMN events.metadata IS 'Additional context: user_id, trace_id, span_id';
