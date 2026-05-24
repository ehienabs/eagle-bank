package domain

import (
	"fmt"
	"regexp"
	"time"

	"github.com/ehienabs/eagle-bank/pkg/errors"
)

// AccountType represents the type of bank account
type AccountType string

const (
	AccountTypePersonal AccountType = "personal"
)

// Account represents a bank account
type Account struct {
	ID            string      `json:"id"`
	AccountNumber string      `json:"accountNumber"`
	UserID        string      `json:"userId"`
	Name          string      `json:"name"`
	AccountType   AccountType `json:"accountType"`
	Balance       int64       `json:"-"`              // Stored in cents, not exposed directly
	Currency      string      `json:"currency"`
	SortCode      string      `json:"sortCode"`
	CreatedAt     time.Time   `json:"createdTimestamp"`
	UpdatedAt     time.Time   `json:"updatedTimestamp"`
	DeletedAt     *time.Time  `json:"-"`
	Version       int         `json:"-"`
}

// CreateAccountRequest represents the request to create an account
type CreateAccountRequest struct {
	Name        string      `json:"name" validate:"required"`
	AccountType AccountType `json:"accountType" validate:"required,oneof=personal"`
}

// UpdateAccountRequest represents the request to update an account
type UpdateAccountRequest struct {
	Name        *string      `json:"name,omitempty"`
	AccountType *AccountType `json:"accountType,omitempty" validate:"omitempty,oneof=personal"`
}

// AccountResponse represents the API response for an account
type AccountResponse struct {
	AccountNumber    string      `json:"accountNumber"`
	SortCode         string      `json:"sortCode"`
	Name             string      `json:"name"`
	AccountType      AccountType `json:"accountType"`
	Balance          float64     `json:"balance"`
	Currency         string      `json:"currency"`
	CreatedTimestamp time.Time   `json:"createdTimestamp"`
	UpdatedTimestamp time.Time   `json:"updatedTimestamp"`
}

// ListAccountsResponse represents the API response for listing accounts
type ListAccountsResponse struct {
	Accounts []*AccountResponse `json:"accounts"`
}

// Validation patterns
var (
	accountNumberRegex = regexp.MustCompile(`^01\d{6}$`)
)

// ToResponse converts an Account to AccountResponse
func (a *Account) ToResponse() *AccountResponse {
	return &AccountResponse{
		AccountNumber:    a.AccountNumber,
		SortCode:         a.SortCode,
		Name:             a.Name,
		AccountType:      a.AccountType,
		Balance:          float64(a.Balance) / 100.0, // Convert cents to pounds
		Currency:         a.Currency,
		CreatedTimestamp: a.CreatedAt,
		UpdatedTimestamp: a.UpdatedAt,
	}
}

// BalanceInPounds returns the balance in pounds (with 2 decimal places)
func (a *Account) BalanceInPounds() float64 {
	return float64(a.Balance) / 100.0
}

// Validate validates the CreateAccountRequest
func (r *CreateAccountRequest) Validate() error {
	var fieldErrors []errors.FieldError

	if r.Name == "" {
		fieldErrors = append(fieldErrors, errors.FieldError{
			Field:   "name",
			Message: "name is required",
			Type:    "required",
		})
	}

	if r.AccountType == "" {
		fieldErrors = append(fieldErrors, errors.FieldError{
			Field:   "accountType",
			Message: "accountType is required",
			Type:    "required",
		})
	} else if !r.AccountType.IsValid() {
		fieldErrors = append(fieldErrors, errors.FieldError{
			Field:   "accountType",
			Message: fmt.Sprintf("accountType must be one of: %s", AccountTypePersonal),
			Type:    "enum",
		})
	}

	if len(fieldErrors) > 0 {
		return errors.InvalidInput("validation failed").WithDetails(fieldErrors...)
	}

	return nil
}

// IsValid checks if the account type is valid
func (at AccountType) IsValid() bool {
	switch at {
	case AccountTypePersonal:
		return true
	default:
		return false
	}
}

// ValidateAccountNumber validates an account number format
func ValidateAccountNumber(accountNumber string) bool {
	return accountNumberRegex.MatchString(accountNumber)
}

// IsDeleted returns true if the account is soft-deleted
func (a *Account) IsDeleted() bool {
	return a.DeletedAt != nil
}

// CanDelete checks if the account can be deleted
func (a *Account) CanDelete() error {
	if a.Balance != 0 {
		return errors.New(errors.CodeAccountHasBalance, "cannot delete account with non-zero balance")
	}
	return nil
}

// CanWithdraw checks if the account can withdraw the specified amount
func (a *Account) CanWithdraw(amountCents int64) error {
	if amountCents <= 0 {
		return errors.New(errors.CodeInvalidAmount, "amount must be positive")
	}
	if a.Balance < amountCents {
		return errors.InsufficientFunds()
	}
	return nil
}

// Deposit adds money to the account
func (a *Account) Deposit(amountCents int64) error {
	if amountCents <= 0 {
		return errors.New(errors.CodeInvalidAmount, "amount must be positive")
	}
	// Check for overflow
	if a.Balance > 0 && amountCents > (1<<62)-a.Balance {
		return errors.New(errors.CodeInvalidAmount, "deposit would cause overflow")
	}
	a.Balance += amountCents
	a.UpdatedAt = time.Now()
	a.Version++
	return nil
}

// Withdraw removes money from the account
func (a *Account) Withdraw(amountCents int64) error {
	if err := a.CanWithdraw(amountCents); err != nil {
		return err
	}
	a.Balance -= amountCents
	a.UpdatedAt = time.Now()
	a.Version++
	return nil
}

// Constants
const (
	DefaultCurrency = "GBP"
	DefaultSortCode = "10-10-10"
	MaxBalance      = 1000000 // 10,000 GBP in cents (£10,000.00)
)
