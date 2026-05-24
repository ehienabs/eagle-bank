package domain

import (
	"fmt"
	"regexp"
	"time"

	"github.com/ehienabs/eagle-bank/pkg/errors"
)

// TransactionType represents the type of transaction
type TransactionType string

const (
	TransactionTypeDeposit    TransactionType = "deposit"
	TransactionTypeWithdrawal TransactionType = "withdrawal"
)

// TransactionStatus represents the status of a transaction
type TransactionStatus string

const (
	TransactionStatusPending   TransactionStatus = "pending"
	TransactionStatusCompleted TransactionStatus = "completed"
	TransactionStatusFailed    TransactionStatus = "failed"
)

// Transaction represents a bank transaction
type Transaction struct {
	ID            string            `json:"id"`
	AccountID     string            `json:"-"`
	AccountNumber string            `json:"accountNumber"`
	UserID        string            `json:"userId"`
	Type          TransactionType   `json:"type"`
	Amount        int64             `json:"-"` // Stored in cents
	Currency      string            `json:"currency"`
	Reference     string            `json:"reference,omitempty"`
	BalanceBefore int64             `json:"-"`
	BalanceAfter  int64             `json:"-"`
	Status        TransactionStatus `json:"status"`
	CreatedAt     time.Time         `json:"createdTimestamp"`
	UpdatedAt     time.Time         `json:"-"`
}

// CreateTransactionRequest represents the request to create a transaction
type CreateTransactionRequest struct {
	Amount    float64         `json:"amount" validate:"required,gt=0,lte=10000"`
	Currency  string          `json:"currency" validate:"required,eq=GBP"`
	Type      TransactionType `json:"type" validate:"required,oneof=deposit withdrawal"`
	Reference string          `json:"reference,omitempty"`
}

// TransactionResponse represents the API response for a transaction
type TransactionResponse struct {
	ID               string          `json:"id"`
	Amount           float64         `json:"amount"`
	Currency         string          `json:"currency"`
	Type             TransactionType `json:"type"`
	Reference        string          `json:"reference,omitempty"`
	UserID           string          `json:"userId,omitempty"`
	CreatedTimestamp time.Time       `json:"createdTimestamp"`
}

// ListTransactionsResponse represents the API response for listing transactions
type ListTransactionsResponse struct {
	Transactions []*TransactionResponse `json:"transactions"`
}

// Validation patterns
var (
	transactionIDRegex = regexp.MustCompile(`^tan-[A-Za-z0-9]+$`)
)

// ToResponse converts a Transaction to TransactionResponse
func (t *Transaction) ToResponse() *TransactionResponse {
	return &TransactionResponse{
		ID:               t.ID,
		Amount:           float64(t.Amount) / 100.0, // Convert cents to pounds
		Currency:         t.Currency,
		Type:             t.Type,
		Reference:        t.Reference,
		UserID:           t.UserID,
		CreatedTimestamp: t.CreatedAt,
	}
}

// AmountInPounds returns the amount in pounds
func (t *Transaction) AmountInPounds() float64 {
	return float64(t.Amount) / 100.0
}

// Validate validates the CreateTransactionRequest
func (r *CreateTransactionRequest) Validate() error {
	var fieldErrors []errors.FieldError

	if r.Amount <= 0 {
		fieldErrors = append(fieldErrors, errors.FieldError{
			Field:   "amount",
			Message: "amount must be greater than 0",
			Type:    "min",
		})
	} else if r.Amount > 10000 {
		fieldErrors = append(fieldErrors, errors.FieldError{
			Field:   "amount",
			Message: "amount must be at most 10000",
			Type:    "max",
		})
	}

	if r.Currency == "" {
		fieldErrors = append(fieldErrors, errors.FieldError{
			Field:   "currency",
			Message: "currency is required",
			Type:    "required",
		})
	} else if r.Currency != "GBP" {
		fieldErrors = append(fieldErrors, errors.FieldError{
			Field:   "currency",
			Message: "currency must be GBP",
			Type:    "enum",
		})
	}

	if r.Type == "" {
		fieldErrors = append(fieldErrors, errors.FieldError{
			Field:   "type",
			Message: "type is required",
			Type:    "required",
		})
	} else if !r.Type.IsValid() {
		fieldErrors = append(fieldErrors, errors.FieldError{
			Field:   "type",
			Message: fmt.Sprintf("type must be one of: %s, %s", TransactionTypeDeposit, TransactionTypeWithdrawal),
			Type:    "enum",
		})
	}

	if len(fieldErrors) > 0 {
		return errors.InvalidInput("validation failed").WithDetails(fieldErrors...)
	}

	return nil
}

// IsValid checks if the transaction type is valid
func (tt TransactionType) IsValid() bool {
	switch tt {
	case TransactionTypeDeposit, TransactionTypeWithdrawal:
		return true
	default:
		return false
	}
}

// IsDeposit returns true if the transaction is a deposit
func (t *Transaction) IsDeposit() bool {
	return t.Type == TransactionTypeDeposit
}

// IsWithdrawal returns true if the transaction is a withdrawal
func (t *Transaction) IsWithdrawal() bool {
	return t.Type == TransactionTypeWithdrawal
}

// IsCompleted returns true if the transaction is completed
func (t *Transaction) IsCompleted() bool {
	return t.Status == TransactionStatusCompleted
}

// IsFailed returns true if the transaction is failed
func (t *Transaction) IsFailed() bool {
	return t.Status == TransactionStatusFailed
}

// ValidateTransactionID validates a transaction ID format
func ValidateTransactionID(id string) bool {
	return transactionIDRegex.MatchString(id)
}

// AmountToCents converts a float amount to cents
func AmountToCents(amount float64) int64 {
	return int64(amount * 100)
}

// CentsToAmount converts cents to a float amount
func CentsToAmount(cents int64) float64 {
	return float64(cents) / 100.0
}

// Constants
const (
	MaxTransactionAmount = 1000000 // £10,000.00 in cents
	MinTransactionAmount = 1       // £0.01 in cents
)
