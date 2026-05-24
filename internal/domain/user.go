// Package domain contains the core business domain models and logic.
package domain

import (
	"regexp"
	"time"

	"github.com/ehienabs/eagle-bank/pkg/errors"
)

// User represents a bank user
type User struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Email        string     `json:"email"`
	PhoneNumber  string     `json:"phoneNumber"`
	PasswordHash string     `json:"-"` // Never expose password hash
	Address      Address    `json:"address"`
	CreatedAt    time.Time  `json:"createdTimestamp"`
	UpdatedAt    time.Time  `json:"updatedTimestamp"`
	DeletedAt    *time.Time `json:"-"`
	Version      int        `json:"-"`
}

// Address represents a user's address
type Address struct {
	Line1    string `json:"line1"`
	Line2    string `json:"line2,omitempty"`
	Line3    string `json:"line3,omitempty"`
	Town     string `json:"town"`
	County   string `json:"county"`
	Postcode string `json:"postcode"`
}

// CreateUserRequest represents the request to create a user
type CreateUserRequest struct {
	Name        string  `json:"name" validate:"required"`
	Email       string  `json:"email" validate:"required,email"`
	PhoneNumber string  `json:"phoneNumber" validate:"required"`
	Password    string  `json:"password,omitempty" validate:"omitempty,min=8"`
	Address     Address `json:"address" validate:"required"`
}

// UpdateUserRequest represents the request to update a user
type UpdateUserRequest struct {
	Name        *string  `json:"name,omitempty"`
	Email       *string  `json:"email,omitempty" validate:"omitempty,email"`
	PhoneNumber *string  `json:"phoneNumber,omitempty"`
	Address     *Address `json:"address,omitempty"`
}

// UserResponse represents the API response for a user
type UserResponse struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Email            string    `json:"email"`
	PhoneNumber      string    `json:"phoneNumber"`
	Address          Address   `json:"address"`
	CreatedTimestamp time.Time `json:"createdTimestamp"`
	UpdatedTimestamp time.Time `json:"updatedTimestamp"`
}

// ToResponse converts a User to UserResponse
func (u *User) ToResponse() *UserResponse {
	return &UserResponse{
		ID:               u.ID,
		Name:             u.Name,
		Email:            u.Email,
		PhoneNumber:      u.PhoneNumber,
		Address:          u.Address,
		CreatedTimestamp: u.CreatedAt,
		UpdatedTimestamp: u.UpdatedAt,
	}
}

// Validation patterns
var (
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	phoneRegex = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)
	userIDRegex = regexp.MustCompile(`^usr-[A-Za-z0-9]+$`)
)

// Validate validates the CreateUserRequest
func (r *CreateUserRequest) Validate() error {
	var fieldErrors []errors.FieldError

	if r.Name == "" {
		fieldErrors = append(fieldErrors, errors.FieldError{
			Field:   "name",
			Message: "name is required",
			Type:    "required",
		})
	}

	if r.Email == "" {
		fieldErrors = append(fieldErrors, errors.FieldError{
			Field:   "email",
			Message: "email is required",
			Type:    "required",
		})
	} else if !emailRegex.MatchString(r.Email) {
		fieldErrors = append(fieldErrors, errors.FieldError{
			Field:   "email",
			Message: "email format is invalid",
			Type:    "format",
		})
	}

	if r.PhoneNumber == "" {
		fieldErrors = append(fieldErrors, errors.FieldError{
			Field:   "phoneNumber",
			Message: "phoneNumber is required",
			Type:    "required",
		})
	} else if !phoneRegex.MatchString(r.PhoneNumber) {
		fieldErrors = append(fieldErrors, errors.FieldError{
			Field:   "phoneNumber",
			Message: "phoneNumber must be in E.164 format (e.g., +441234567890)",
			Type:    "format",
		})
	}

	if err := r.Address.Validate(); err != nil {
		if appErr, ok := err.(*errors.Error); ok {
			fieldErrors = append(fieldErrors, appErr.Details()...)
		}
	}

	if len(fieldErrors) > 0 {
		return errors.InvalidInput("validation failed").WithDetails(fieldErrors...)
	}

	return nil
}

// Validate validates an Address
func (a *Address) Validate() error {
	var fieldErrors []errors.FieldError

	if a.Line1 == "" {
		fieldErrors = append(fieldErrors, errors.FieldError{
			Field:   "address.line1",
			Message: "address line1 is required",
			Type:    "required",
		})
	}

	if a.Town == "" {
		fieldErrors = append(fieldErrors, errors.FieldError{
			Field:   "address.town",
			Message: "address town is required",
			Type:    "required",
		})
	}

	if a.County == "" {
		fieldErrors = append(fieldErrors, errors.FieldError{
			Field:   "address.county",
			Message: "address county is required",
			Type:    "required",
		})
	}

	if a.Postcode == "" {
		fieldErrors = append(fieldErrors, errors.FieldError{
			Field:   "address.postcode",
			Message: "address postcode is required",
			Type:    "required",
		})
	}

	if len(fieldErrors) > 0 {
		return errors.InvalidInput("address validation failed").WithDetails(fieldErrors...)
	}

	return nil
}

// ValidateUserID validates a user ID format
func ValidateUserID(id string) bool {
	return userIDRegex.MatchString(id)
}

// IsDeleted returns true if the user is soft-deleted
func (u *User) IsDeleted() bool {
	return u.DeletedAt != nil
}
