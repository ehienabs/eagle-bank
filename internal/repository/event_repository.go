package repository

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/ehienabs/eagle-bank/internal/domain"
	"github.com/ehienabs/eagle-bank/pkg/errors"
)

// EventRepository handles event persistence
type EventRepository struct {
	db *DB
}

// NewEventRepository creates a new EventRepository
func NewEventRepository(db *DB) *EventRepository {
	return &EventRepository{db: db}
}

// Append appends an event to the event store
func (r *EventRepository) Append(ctx context.Context, event *domain.Event) error {
	query := `
		INSERT INTO events (
			id, aggregate_id, aggregate_type, event_type, version,
			payload, metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	metadataJSON, err := json.Marshal(event.Metadata)
	if err != nil {
		return errors.InternalWrap(err, "failed to marshal metadata")
	}

	_, err = r.db.ExecContext(ctx, query,
		event.ID,
		event.AggregateID,
		event.AggregateType,
		event.EventType,
		event.Version,
		event.Payload,
		metadataJSON,
		event.Timestamp,
	)

	if err != nil {
		if isDuplicateKeyError(err) {
			return errors.Conflict("event already exists")
		}
		return errors.InternalWrap(err, "failed to append event")
	}

	return nil
}

// AppendTx appends an event within a transaction
func (r *EventRepository) AppendTx(ctx context.Context, tx *Tx, event *domain.Event) error {
	query := `
		INSERT INTO events (
			id, aggregate_id, aggregate_type, event_type, version,
			payload, metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	metadataJSON, err := json.Marshal(event.Metadata)
	if err != nil {
		return errors.InternalWrap(err, "failed to marshal metadata")
	}

	_, err = tx.ExecContext(ctx, query,
		event.ID,
		event.AggregateID,
		event.AggregateType,
		event.EventType,
		event.Version,
		event.Payload,
		metadataJSON,
		event.Timestamp,
	)

	if err != nil {
		if isDuplicateKeyError(err) {
			return errors.Conflict("event already exists")
		}
		return errors.InternalWrap(err, "failed to append event")
	}

	return nil
}

// GetByAggregateID retrieves all events for an aggregate
func (r *EventRepository) GetByAggregateID(ctx context.Context, aggregateID string) ([]*domain.Event, error) {
	query := `
		SELECT id, aggregate_id, aggregate_type, event_type, version,
			payload, metadata, created_at
		FROM events
		WHERE aggregate_id = $1
		ORDER BY version ASC
	`

	rows, err := r.db.QueryContext(ctx, query, aggregateID)
	if err != nil {
		return nil, errors.InternalWrap(err, "failed to get events")
	}
	defer rows.Close()

	var events []*domain.Event
	for rows.Next() {
		event, err := r.scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.InternalWrap(err, "failed to iterate events")
	}

	return events, nil
}

// GetLatestVersion gets the latest version for an aggregate
func (r *EventRepository) GetLatestVersion(ctx context.Context, aggregateID string) (int, error) {
	query := `
		SELECT COALESCE(MAX(version), 0)
		FROM events
		WHERE aggregate_id = $1
	`

	var version int
	err := r.db.QueryRowContext(ctx, query, aggregateID).Scan(&version)
	if err != nil {
		return 0, errors.InternalWrap(err, "failed to get latest version")
	}

	return version, nil
}

// scanEvent scans a single event row
func (r *EventRepository) scanEvent(rows *sql.Rows) (*domain.Event, error) {
	event := &domain.Event{}
	var metadataJSON []byte

	err := rows.Scan(
		&event.ID,
		&event.AggregateID,
		&event.AggregateType,
		&event.EventType,
		&event.Version,
		&event.Payload,
		&metadataJSON,
		&event.Timestamp,
	)

	if err != nil {
		return nil, errors.InternalWrap(err, "failed to scan event")
	}

	if len(metadataJSON) > 0 {
		var metadata domain.EventMetadata
		if err := json.Unmarshal(metadataJSON, &metadata); err == nil {
			event.Metadata = &metadata
		}
	}

	return event, nil
}

// AuditLogRepository handles audit log persistence
type AuditLogRepository struct {
	db *DB
}

// NewAuditLogRepository creates a new AuditLogRepository
func NewAuditLogRepository(db *DB) *AuditLogRepository {
	return &AuditLogRepository{db: db}
}

// Append appends an entry to the audit log
func (r *AuditLogRepository) Append(ctx context.Context, event *domain.Event, userID, ipAddress string) error {
	query := `
		INSERT INTO audit_log (
			id, event_id, aggregate_id, aggregate_type, event_type,
			payload, user_id, ip_address, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	auditID := "audit-" + event.ID
	_, err := r.db.ExecContext(ctx, query,
		auditID,
		event.ID,
		event.AggregateID,
		event.AggregateType,
		event.EventType,
		event.Payload,
		userID,
		ipAddress,
		event.Timestamp,
	)

	if err != nil {
		if isDuplicateKeyError(err) {
			// Already logged, ignore
			return nil
		}
		return errors.InternalWrap(err, "failed to append audit log")
	}

	return nil
}
