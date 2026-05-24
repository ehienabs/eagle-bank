package repository

import (
	"context"
	"database/sql"

	"github.com/ehienabs/eagle-bank/internal/domain"
	"github.com/ehienabs/eagle-bank/pkg/errors"
)

// TransactionRepository handles transaction data access
type TransactionRepository struct {
	db *DB
}

// NewTransactionRepository creates a new TransactionRepository
func NewTransactionRepository(db *DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

// Create creates a new transaction within a DB transaction
func (r *TransactionRepository) Create(ctx context.Context, tx *Tx, transaction *domain.Transaction) error {
	query := `
		INSERT INTO transactions (
			id, account_id, user_id, type, amount, currency,
			reference, balance_before, balance_after, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err := tx.ExecContext(ctx, query,
		transaction.ID,
		transaction.AccountID,
		transaction.UserID,
		transaction.Type,
		transaction.Amount,
		transaction.Currency,
		transaction.Reference,
		transaction.BalanceBefore,
		transaction.BalanceAfter,
		transaction.Status,
		transaction.CreatedAt,
		transaction.UpdatedAt,
	)

	if err != nil {
		return errors.InternalWrap(err, "failed to create transaction")
	}

	return nil
}

// GetByID retrieves a transaction by ID
func (r *TransactionRepository) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	query := `
		SELECT t.id, t.account_id, a.account_number, t.user_id, t.type, t.amount,
			t.currency, t.reference, t.balance_before, t.balance_after, t.status,
			t.created_at, t.updated_at
		FROM transactions t
		JOIN accounts a ON t.account_id = a.id
		WHERE t.id = $1
	`

	return r.scanTransaction(r.db.QueryRowContext(ctx, query, id))
}

// GetByIDAndAccountNumber retrieves a transaction by ID and account number
func (r *TransactionRepository) GetByIDAndAccountNumber(ctx context.Context, id, accountNumber string) (*domain.Transaction, error) {
	query := `
		SELECT t.id, t.account_id, a.account_number, t.user_id, t.type, t.amount,
			t.currency, t.reference, t.balance_before, t.balance_after, t.status,
			t.created_at, t.updated_at
		FROM transactions t
		JOIN accounts a ON t.account_id = a.id
		WHERE t.id = $1 AND a.account_number = $2
	`

	return r.scanTransaction(r.db.QueryRowContext(ctx, query, id, accountNumber))
}

// ListByAccountNumber retrieves all transactions for an account
func (r *TransactionRepository) ListByAccountNumber(ctx context.Context, accountNumber string) ([]*domain.Transaction, error) {
	query := `
		SELECT t.id, t.account_id, a.account_number, t.user_id, t.type, t.amount,
			t.currency, t.reference, t.balance_before, t.balance_after, t.status,
			t.created_at, t.updated_at
		FROM transactions t
		JOIN accounts a ON t.account_id = a.id
		WHERE a.account_number = $1
		ORDER BY t.created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, accountNumber)
	if err != nil {
		return nil, errors.InternalWrap(err, "failed to list transactions")
	}
	defer rows.Close()

	var transactions []*domain.Transaction
	for rows.Next() {
		txn, err := r.scanTransactionRows(rows)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, txn)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.InternalWrap(err, "failed to iterate transactions")
	}

	return transactions, nil
}

// ListByUserID retrieves all transactions for a user
func (r *TransactionRepository) ListByUserID(ctx context.Context, userID string) ([]*domain.Transaction, error) {
	query := `
		SELECT t.id, t.account_id, a.account_number, t.user_id, t.type, t.amount,
			t.currency, t.reference, t.balance_before, t.balance_after, t.status,
			t.created_at, t.updated_at
		FROM transactions t
		JOIN accounts a ON t.account_id = a.id
		WHERE t.user_id = $1
		ORDER BY t.created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, errors.InternalWrap(err, "failed to list transactions")
	}
	defer rows.Close()

	var transactions []*domain.Transaction
	for rows.Next() {
		txn, err := r.scanTransactionRows(rows)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, txn)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.InternalWrap(err, "failed to iterate transactions")
	}

	return transactions, nil
}

// scanTransaction scans a single transaction row
func (r *TransactionRepository) scanTransaction(row *sql.Row) (*domain.Transaction, error) {
	txn := &domain.Transaction{}
	var reference sql.NullString

	err := row.Scan(
		&txn.ID,
		&txn.AccountID,
		&txn.AccountNumber,
		&txn.UserID,
		&txn.Type,
		&txn.Amount,
		&txn.Currency,
		&reference,
		&txn.BalanceBefore,
		&txn.BalanceAfter,
		&txn.Status,
		&txn.CreatedAt,
		&txn.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.TransactionNotFound("")
		}
		return nil, errors.InternalWrap(err, "failed to scan transaction")
	}

	txn.Reference = reference.String
	return txn, nil
}

// scanTransactionRows scans a single transaction from rows
func (r *TransactionRepository) scanTransactionRows(rows *sql.Rows) (*domain.Transaction, error) {
	txn := &domain.Transaction{}
	var reference sql.NullString

	err := rows.Scan(
		&txn.ID,
		&txn.AccountID,
		&txn.AccountNumber,
		&txn.UserID,
		&txn.Type,
		&txn.Amount,
		&txn.Currency,
		&reference,
		&txn.BalanceBefore,
		&txn.BalanceAfter,
		&txn.Status,
		&txn.CreatedAt,
		&txn.UpdatedAt,
	)

	if err != nil {
		return nil, errors.InternalWrap(err, "failed to scan transaction")
	}

	txn.Reference = reference.String
	return txn, nil
}
