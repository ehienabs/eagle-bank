package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/ehienabs/eagle-bank/internal/domain"
	"github.com/ehienabs/eagle-bank/pkg/errors"
)

// UserRepository handles user data access
type UserRepository struct {
	db *DB
}

// NewUserRepository creates a new UserRepository
func NewUserRepository(db *DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create creates a new user
func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (
			id, name, email, phone_number, password_hash,
			address_line1, address_line2, address_line3,
			address_town, address_county, address_postcode,
			created_at, updated_at, version
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	_, err := r.db.ExecContext(ctx, query,
		user.ID,
		user.Name,
		user.Email,
		user.PhoneNumber,
		user.PasswordHash,
		user.Address.Line1,
		user.Address.Line2,
		user.Address.Line3,
		user.Address.Town,
		user.Address.County,
		user.Address.Postcode,
		user.CreatedAt,
		user.UpdatedAt,
		user.Version,
	)

	if err != nil {
		// Check for duplicate email
		if isDuplicateKeyError(err) {
			return errors.DuplicateEmail(user.Email)
		}
		return errors.InternalWrap(err, "failed to create user")
	}

	return nil
}

// GetByID retrieves a user by ID
func (r *UserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	query := `
		SELECT id, name, email, phone_number, password_hash,
			address_line1, address_line2, address_line3,
			address_town, address_county, address_postcode,
			created_at, updated_at, deleted_at, version
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`

	user := &domain.User{}
	var line2, line3 sql.NullString
	var deletedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.PhoneNumber,
		&user.PasswordHash,
		&user.Address.Line1,
		&line2,
		&line3,
		&user.Address.Town,
		&user.Address.County,
		&user.Address.Postcode,
		&user.CreatedAt,
		&user.UpdatedAt,
		&deletedAt,
		&user.Version,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.UserNotFound(id)
		}
		return nil, errors.InternalWrap(err, "failed to get user")
	}

	user.Address.Line2 = line2.String
	user.Address.Line3 = line3.String
	if deletedAt.Valid {
		user.DeletedAt = &deletedAt.Time
	}

	return user, nil
}

// GetByEmail retrieves a user by email
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT id, name, email, phone_number, password_hash,
			address_line1, address_line2, address_line3,
			address_town, address_county, address_postcode,
			created_at, updated_at, deleted_at, version
		FROM users
		WHERE email = $1 AND deleted_at IS NULL
	`

	user := &domain.User{}
	var line2, line3 sql.NullString
	var deletedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.PhoneNumber,
		&user.PasswordHash,
		&user.Address.Line1,
		&line2,
		&line3,
		&user.Address.Town,
		&user.Address.County,
		&user.Address.Postcode,
		&user.CreatedAt,
		&user.UpdatedAt,
		&deletedAt,
		&user.Version,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.UserNotFound(email)
		}
		return nil, errors.InternalWrap(err, "failed to get user by email")
	}

	user.Address.Line2 = line2.String
	user.Address.Line3 = line3.String
	if deletedAt.Valid {
		user.DeletedAt = &deletedAt.Time
	}

	return user, nil
}

// Update updates a user
func (r *UserRepository) Update(ctx context.Context, user *domain.User) error {
	query := `
		UPDATE users SET
			name = $1,
			email = $2,
			phone_number = $3,
			address_line1 = $4,
			address_line2 = $5,
			address_line3 = $6,
			address_town = $7,
			address_county = $8,
			address_postcode = $9,
			updated_at = $10,
			version = version + 1
		WHERE id = $11 AND deleted_at IS NULL AND version = $12
		RETURNING version
	`

	var newVersion int
	err := r.db.QueryRowContext(ctx, query,
		user.Name,
		user.Email,
		user.PhoneNumber,
		user.Address.Line1,
		user.Address.Line2,
		user.Address.Line3,
		user.Address.Town,
		user.Address.County,
		user.Address.Postcode,
		time.Now(),
		user.ID,
		user.Version,
	).Scan(&newVersion)

	if err != nil {
		if err == sql.ErrNoRows {
			return errors.UserNotFound(user.ID)
		}
		if isDuplicateKeyError(err) {
			return errors.DuplicateEmail(user.Email)
		}
		return errors.InternalWrap(err, "failed to update user")
	}

	user.Version = newVersion
	return nil
}

// Delete soft-deletes a user
func (r *UserRepository) Delete(ctx context.Context, id string) error {
	// Check if user has accounts
	var accountCount int
	countQuery := `SELECT COUNT(*) FROM accounts WHERE user_id = $1 AND deleted_at IS NULL`
	err := r.db.QueryRowContext(ctx, countQuery, id).Scan(&accountCount)
	if err != nil {
		return errors.InternalWrap(err, "failed to check user accounts")
	}

	if accountCount > 0 {
		return errors.New(errors.CodeUserHasAccounts, "cannot delete user with active accounts")
	}

	query := `
		UPDATE users SET
			deleted_at = $1,
			updated_at = $1
		WHERE id = $2 AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, time.Now(), id)
	if err != nil {
		return errors.InternalWrap(err, "failed to delete user")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.InternalWrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return errors.UserNotFound(id)
	}

	return nil
}

// ExistsByEmail checks if a user with the given email exists
func (r *UserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND deleted_at IS NULL)`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, email).Scan(&exists)
	if err != nil {
		return false, errors.InternalWrap(err, "failed to check email existence")
	}

	return exists, nil
}

// isDuplicateKeyError checks if error is a duplicate key violation
func isDuplicateKeyError(err error) bool {
	// PostgreSQL unique violation error code is 23505
	return err != nil && (contains(err.Error(), "23505") || contains(err.Error(), "duplicate key"))
}

// IsSerializationFailure reports whether err is a PostgreSQL serialization failure
// (SQLSTATE 40001). These errors are safe to retry — the transaction had no effect.
func IsSerializationFailure(err error) bool {
	return err != nil && contains(err.Error(), "40001")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
