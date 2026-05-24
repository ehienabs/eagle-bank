package repository

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"time"

	"github.com/ehienabs/eagle-bank/internal/domain"
	"github.com/ehienabs/eagle-bank/pkg/errors"
)

// AccountRepository handles account data access
type AccountRepository struct {
	db *DB
}

// NewAccountRepository creates a new AccountRepository
func NewAccountRepository(db *DB) *AccountRepository {
	return &AccountRepository{db: db}
}

// Create creates a new account
func (r *AccountRepository) Create(ctx context.Context, account *domain.Account) error {
	query := `
		INSERT INTO accounts (
			id, account_number, user_id, name, account_type,
			balance, currency, sort_code, created_at, updated_at, version
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err := r.db.ExecContext(ctx, query,
		account.ID,
		account.AccountNumber,
		account.UserID,
		account.Name,
		account.AccountType,
		account.Balance,
		account.Currency,
		account.SortCode,
		account.CreatedAt,
		account.UpdatedAt,
		account.Version,
	)

	if err != nil {
		if isDuplicateKeyError(err) {
			return errors.Conflict("account number already exists")
		}
		return errors.InternalWrap(err, "failed to create account")
	}

	return nil
}

// GetByAccountNumber retrieves an account by account number
func (r *AccountRepository) GetByAccountNumber(ctx context.Context, accountNumber string) (*domain.Account, error) {
	query := `
		SELECT id, account_number, user_id, name, account_type,
			balance, currency, sort_code, created_at, updated_at, deleted_at, version
		FROM accounts
		WHERE account_number = $1 AND deleted_at IS NULL
	`

	return r.scanAccount(r.db.QueryRowContext(ctx, query, accountNumber))
}

// GetByAccountNumberForUpdate retrieves an account with a row lock for update
func (r *AccountRepository) GetByAccountNumberForUpdate(ctx context.Context, tx *Tx, accountNumber string) (*domain.Account, error) {
	query := `
		SELECT id, account_number, user_id, name, account_type,
			balance, currency, sort_code, created_at, updated_at, deleted_at, version
		FROM accounts
		WHERE account_number = $1 AND deleted_at IS NULL
		FOR UPDATE
	`

	return r.scanAccountTx(tx.QueryRowContext(ctx, query, accountNumber))
}

// GetByID retrieves an account by ID
func (r *AccountRepository) GetByID(ctx context.Context, id string) (*domain.Account, error) {
	query := `
		SELECT id, account_number, user_id, name, account_type,
			balance, currency, sort_code, created_at, updated_at, deleted_at, version
		FROM accounts
		WHERE id = $1 AND deleted_at IS NULL
	`

	return r.scanAccount(r.db.QueryRowContext(ctx, query, id))
}

// ListByUserID retrieves all accounts for a user
func (r *AccountRepository) ListByUserID(ctx context.Context, userID string) ([]*domain.Account, error) {
	query := `
		SELECT id, account_number, user_id, name, account_type,
			balance, currency, sort_code, created_at, updated_at, deleted_at, version
		FROM accounts
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, errors.InternalWrap(err, "failed to list accounts")
	}
	defer rows.Close()

	var accounts []*domain.Account
	for rows.Next() {
		account, err := r.scanAccountRows(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.InternalWrap(err, "failed to iterate accounts")
	}

	return accounts, nil
}

// Update updates an account
func (r *AccountRepository) Update(ctx context.Context, account *domain.Account) error {
	query := `
		UPDATE accounts SET
			name = $1,
			account_type = $2,
			updated_at = $3,
			version = version + 1
		WHERE id = $4 AND deleted_at IS NULL AND version = $5
		RETURNING version
	`

	var newVersion int
	err := r.db.QueryRowContext(ctx, query,
		account.Name,
		account.AccountType,
		time.Now(),
		account.ID,
		account.Version,
	).Scan(&newVersion)

	if err != nil {
		if err == sql.ErrNoRows {
			return errors.AccountNotFound(account.AccountNumber)
		}
		return errors.InternalWrap(err, "failed to update account")
	}

	account.Version = newVersion
	return nil
}

// UpdateBalance updates an account balance within a transaction
func (r *AccountRepository) UpdateBalance(ctx context.Context, tx *Tx, accountID string, newBalance int64, version int) error {
	query := `
		UPDATE accounts SET
			balance = $1,
			updated_at = $2,
			version = version + 1
		WHERE id = $3 AND deleted_at IS NULL AND version = $4
	`

	result, err := tx.ExecContext(ctx, query, newBalance, time.Now(), accountID, version)
	if err != nil {
		return errors.InternalWrap(err, "failed to update balance")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.InternalWrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return errors.Conflict("account was modified by another transaction")
	}

	return nil
}

// Delete soft-deletes an account
func (r *AccountRepository) Delete(ctx context.Context, accountNumber string) error {
	// First check if balance is zero
	account, err := r.GetByAccountNumber(ctx, accountNumber)
	if err != nil {
		return err
	}

	if account.Balance != 0 {
		return errors.New(errors.CodeAccountHasBalance, "cannot delete account with non-zero balance")
	}

	query := `
		UPDATE accounts SET
			deleted_at = $1,
			updated_at = $1
		WHERE account_number = $2 AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, time.Now(), accountNumber)
	if err != nil {
		return errors.InternalWrap(err, "failed to delete account")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.InternalWrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return errors.AccountNotFound(accountNumber)
	}

	return nil
}

// GenerateAccountNumber generates a unique account number
func (r *AccountRepository) GenerateAccountNumber(ctx context.Context) (string, error) {
	// Account number format: 01XXXXXX (8 digits starting with 01)
	for attempts := 0; attempts < 10; attempts++ {
		// Generate 6 random digits
		number := rand.Intn(1000000)
		accountNumber := fmt.Sprintf("01%06d", number)

		// Check if it exists
		exists, err := r.accountNumberExists(ctx, accountNumber)
		if err != nil {
			return "", err
		}

		if !exists {
			return accountNumber, nil
		}
	}

	return "", errors.Internal("failed to generate unique account number after 10 attempts")
}

func (r *AccountRepository) accountNumberExists(ctx context.Context, accountNumber string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM accounts WHERE account_number = $1)`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, accountNumber).Scan(&exists)
	if err != nil {
		return false, errors.InternalWrap(err, "failed to check account number existence")
	}

	return exists, nil
}

// scanAccount scans a single account row
func (r *AccountRepository) scanAccount(row *sql.Row) (*domain.Account, error) {
	account := &domain.Account{}
	var deletedAt sql.NullTime

	err := row.Scan(
		&account.ID,
		&account.AccountNumber,
		&account.UserID,
		&account.Name,
		&account.AccountType,
		&account.Balance,
		&account.Currency,
		&account.SortCode,
		&account.CreatedAt,
		&account.UpdatedAt,
		&deletedAt,
		&account.Version,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.AccountNotFound("")
		}
		return nil, errors.InternalWrap(err, "failed to scan account")
	}

	if deletedAt.Valid {
		account.DeletedAt = &deletedAt.Time
	}

	return account, nil
}

// scanAccountTx scans a single account row from a transaction
func (r *AccountRepository) scanAccountTx(row *sql.Row) (*domain.Account, error) {
	account := &domain.Account{}
	var deletedAt sql.NullTime

	err := row.Scan(
		&account.ID,
		&account.AccountNumber,
		&account.UserID,
		&account.Name,
		&account.AccountType,
		&account.Balance,
		&account.Currency,
		&account.SortCode,
		&account.CreatedAt,
		&account.UpdatedAt,
		&deletedAt,
		&account.Version,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.AccountNotFound("")
		}
		return nil, errors.InternalWrap(err, "failed to scan account")
	}

	if deletedAt.Valid {
		account.DeletedAt = &deletedAt.Time
	}

	return account, nil
}

// scanAccountRows scans a single account from rows
func (r *AccountRepository) scanAccountRows(rows *sql.Rows) (*domain.Account, error) {
	account := &domain.Account{}
	var deletedAt sql.NullTime

	err := rows.Scan(
		&account.ID,
		&account.AccountNumber,
		&account.UserID,
		&account.Name,
		&account.AccountType,
		&account.Balance,
		&account.Currency,
		&account.SortCode,
		&account.CreatedAt,
		&account.UpdatedAt,
		&deletedAt,
		&account.Version,
	)

	if err != nil {
		return nil, errors.InternalWrap(err, "failed to scan account")
	}

	if deletedAt.Valid {
		account.DeletedAt = &deletedAt.Time
	}

	return account, nil
}
