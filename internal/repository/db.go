// Package repository provides data access layer with PostgreSQL.
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ehienabs/eagle-bank/internal/config"
	"github.com/ehienabs/eagle-bank/pkg/logger"
	"github.com/ehienabs/eagle-bank/pkg/metrics"
	"github.com/ehienabs/eagle-bank/pkg/tracing"
	_ "github.com/lib/pq"
)

// DB wraps *sql.DB with observability
type DB struct {
	*sql.DB
	metrics *metrics.Metrics
	logger  *logger.Logger
}

// NewDB creates a new database connection pool
func NewDB(cfg config.DatabaseConfig) (*DB, error) {
	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{
		DB:      db,
		metrics: metrics.Default(),
		logger:  logger.Default(),
	}, nil
}

// ExecContext executes a query with metrics and tracing
func (db *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	ctx, span := tracing.StartDBSpan(ctx, "exec", extractTable(query))
	defer span.End()

	timer := metrics.NewTimer()
	result, err := db.DB.ExecContext(ctx, query, args...)
	db.metrics.RecordDBQuery("exec", extractTable(query), timer.Elapsed(), err)

	if err != nil {
		span.SetError(err)
		return nil, err
	}

	span.SetOK()
	return result, nil
}

// QueryContext executes a query with metrics and tracing
func (db *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	ctx, span := tracing.StartDBSpan(ctx, "query", extractTable(query))
	defer span.End()

	timer := metrics.NewTimer()
	rows, err := db.DB.QueryContext(ctx, query, args...)
	db.metrics.RecordDBQuery("query", extractTable(query), timer.Elapsed(), err)

	if err != nil {
		span.SetError(err)
		return nil, err
	}

	span.SetOK()
	return rows, nil
}

// QueryRowContext executes a query row with metrics and tracing
func (db *DB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	ctx, span := tracing.StartDBSpan(ctx, "query_row", extractTable(query))
	defer span.End()

	timer := metrics.NewTimer()
	row := db.DB.QueryRowContext(ctx, query, args...)
	db.metrics.RecordDBQuery("query_row", extractTable(query), timer.Elapsed(), nil)

	span.SetOK()
	return row
}

// BeginTx starts a transaction with metrics
func (db *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*Tx, error) {
	ctx, span := tracing.StartDBSpan(ctx, "begin_tx", "")
	defer span.End()

	tx, err := db.DB.BeginTx(ctx, opts)
	if err != nil {
		span.SetError(err)
		return nil, err
	}

	span.SetOK()
	return &Tx{
		Tx:      tx,
		metrics: db.metrics,
		logger:  db.logger,
	}, nil
}

// Tx wraps *sql.Tx with observability
type Tx struct {
	*sql.Tx
	metrics *metrics.Metrics
	logger  *logger.Logger
}

// ExecContext executes a query within a transaction
func (tx *Tx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	ctx, span := tracing.StartDBSpan(ctx, "tx_exec", extractTable(query))
	defer span.End()

	timer := metrics.NewTimer()
	result, err := tx.Tx.ExecContext(ctx, query, args...)
	tx.metrics.RecordDBQuery("tx_exec", extractTable(query), timer.Elapsed(), err)

	if err != nil {
		span.SetError(err)
		return nil, err
	}

	span.SetOK()
	return result, nil
}

// QueryContext executes a query within a transaction
func (tx *Tx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	ctx, span := tracing.StartDBSpan(ctx, "tx_query", extractTable(query))
	defer span.End()

	timer := metrics.NewTimer()
	rows, err := tx.Tx.QueryContext(ctx, query, args...)
	tx.metrics.RecordDBQuery("tx_query", extractTable(query), timer.Elapsed(), err)

	if err != nil {
		span.SetError(err)
		return nil, err
	}

	span.SetOK()
	return rows, nil
}

// QueryRowContext executes a query row within a transaction
func (tx *Tx) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	ctx, span := tracing.StartDBSpan(ctx, "tx_query_row", extractTable(query))
	defer span.End()

	timer := metrics.NewTimer()
	row := tx.Tx.QueryRowContext(ctx, query, args...)
	tx.metrics.RecordDBQuery("tx_query_row", extractTable(query), timer.Elapsed(), nil)

	span.SetOK()
	return row
}

// Commit commits the transaction
func (tx *Tx) Commit() error {
	return tx.Tx.Commit()
}

// Rollback rolls back the transaction
func (tx *Tx) Rollback() error {
	return tx.Tx.Rollback()
}

// UpdateStats updates database connection pool metrics
func (db *DB) UpdateStats() {
	stats := db.DB.Stats()
	db.metrics.SetDBConnections(stats.OpenConnections, stats.InUse)
}

// extractTable extracts the primary table name from a SQL query.
// It matches the first identifier after FROM, INTO, UPDATE, or DELETE FROM.
var extractTableRe = regexp.MustCompile(`(?i)(?:FROM|INTO|UPDATE|DELETE\s+FROM)\s+"?(\w+)"?`)

func extractTable(query string) string {
	if m := extractTableRe.FindStringSubmatch(query); len(m) > 1 {
		return strings.ToLower(m[1])
	}
	return "unknown"
}

// Health checks database health
func (db *DB) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return db.PingContext(ctx)
}
