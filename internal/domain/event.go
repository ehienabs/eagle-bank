package domain

import (
	"encoding/json"
	"time"
)

// EventType represents the type of domain event
type EventType string

const (
	// User events
	EventTypeUserCreated EventType = "UserCreated"
	EventTypeUserUpdated EventType = "UserUpdated"
	EventTypeUserDeleted EventType = "UserDeleted"

	// Account events
	EventTypeAccountCreated EventType = "AccountCreated"
	EventTypeAccountUpdated EventType = "AccountUpdated"
	EventTypeAccountDeleted EventType = "AccountDeleted"

	// Transaction events
	EventTypeTransactionCreated   EventType = "TransactionCreated"
	EventTypeTransactionCompleted EventType = "TransactionCompleted"
	EventTypeTransactionFailed    EventType = "TransactionFailed"
)

// AggregateType represents the type of aggregate
type AggregateType string

const (
	AggregateTypeUser        AggregateType = "User"
	AggregateTypeAccount     AggregateType = "Account"
	AggregateTypeTransaction AggregateType = "Transaction"
)

// Event represents a domain event
type Event struct {
	ID            string          `json:"eventId"`
	AggregateID   string          `json:"aggregateId"`
	AggregateType AggregateType   `json:"aggregateType"`
	EventType     EventType       `json:"eventType"`
	Version       int             `json:"version"`
	Timestamp     time.Time       `json:"timestamp"`
	Payload       json.RawMessage `json:"payload"`
	Metadata      *EventMetadata  `json:"metadata,omitempty"`
}

// EventMetadata contains additional event metadata
type EventMetadata struct {
	UserID      string `json:"userId,omitempty"`
	RequestID   string `json:"requestId,omitempty"`
	IPAddress   string `json:"ipAddress,omitempty"`
	UserAgent   string `json:"userAgent,omitempty"`
	TraceID     string `json:"traceId,omitempty"`
	SpanID      string `json:"spanId,omitempty"`
	Idempotency string `json:"idempotencyKey,omitempty"`
}

// UserCreatedPayload is the payload for UserCreated events
type UserCreatedPayload struct {
	UserID      string  `json:"userId"`
	Name        string  `json:"name"`
	Email       string  `json:"email"`
	PhoneNumber string  `json:"phoneNumber"`
	Address     Address `json:"address"`
}

// UserUpdatedPayload is the payload for UserUpdated events
type UserUpdatedPayload struct {
	UserID      string   `json:"userId"`
	Name        *string  `json:"name,omitempty"`
	Email       *string  `json:"email,omitempty"`
	PhoneNumber *string  `json:"phoneNumber,omitempty"`
	Address     *Address `json:"address,omitempty"`
}

// UserDeletedPayload is the payload for UserDeleted events
type UserDeletedPayload struct {
	UserID string `json:"userId"`
}

// AccountCreatedPayload is the payload for AccountCreated events
type AccountCreatedPayload struct {
	AccountNumber string      `json:"accountNumber"`
	UserID        string      `json:"userId"`
	Name          string      `json:"name"`
	AccountType   AccountType `json:"accountType"`
	Balance       int64       `json:"balance"`
	Currency      string      `json:"currency"`
	SortCode      string      `json:"sortCode"`
}

// AccountUpdatedPayload is the payload for AccountUpdated events
type AccountUpdatedPayload struct {
	AccountNumber string       `json:"accountNumber"`
	Name          *string      `json:"name,omitempty"`
	AccountType   *AccountType `json:"accountType,omitempty"`
}

// AccountDeletedPayload is the payload for AccountDeleted events
type AccountDeletedPayload struct {
	AccountNumber string `json:"accountNumber"`
	UserID        string `json:"userId"`
}

// TransactionCreatedPayload is the payload for TransactionCreated events
type TransactionCreatedPayload struct {
	TransactionID string          `json:"transactionId"`
	AccountNumber string          `json:"accountNumber"`
	UserID        string          `json:"userId"`
	Type          TransactionType `json:"type"`
	Amount        int64           `json:"amount"`
	Currency      string          `json:"currency"`
	Reference     string          `json:"reference,omitempty"`
}

// TransactionCompletedPayload is the payload for TransactionCompleted events
type TransactionCompletedPayload struct {
	TransactionID string          `json:"transactionId"`
	AccountNumber string          `json:"accountNumber"`
	UserID        string          `json:"userId"`
	Type          TransactionType `json:"type"`
	Amount        int64           `json:"amount"`
	Currency      string          `json:"currency"`
	Reference     string          `json:"reference,omitempty"`
	BalanceBefore int64           `json:"balanceBefore"`
	BalanceAfter  int64           `json:"balanceAfter"`
	Status        string          `json:"status"`
}

// TransactionFailedPayload is the payload for TransactionFailed events
type TransactionFailedPayload struct {
	TransactionID string          `json:"transactionId"`
	AccountNumber string          `json:"accountNumber"`
	UserID        string          `json:"userId"`
	Type          TransactionType `json:"type"`
	Amount        int64           `json:"amount"`
	Currency      string          `json:"currency"`
	Reference     string          `json:"reference,omitempty"`
	Reason        string          `json:"reason"`
}

// NewEvent creates a new event with the given parameters
func NewEvent(id string, aggregateID string, aggregateType AggregateType, eventType EventType, version int, payload any, metadata *EventMetadata) (*Event, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &Event{
		ID:            id,
		AggregateID:   aggregateID,
		AggregateType: aggregateType,
		EventType:     eventType,
		Version:       version,
		Timestamp:     time.Now().UTC(),
		Payload:       payloadBytes,
		Metadata:      metadata,
	}, nil
}

// UnmarshalPayload unmarshals the event payload into the given target
func (e *Event) UnmarshalPayload(target any) error {
	return json.Unmarshal(e.Payload, target)
}

// Topic returns the Kafka topic for this event type
func (e *Event) Topic() string {
	switch e.AggregateType {
	case AggregateTypeUser:
		return "eagle-bank.user.events"
	case AggregateTypeAccount:
		return "eagle-bank.account.events"
	case AggregateTypeTransaction:
		return "eagle-bank.transaction.events"
	default:
		return "eagle-bank.unknown.events"
	}
}

// Key returns the partition key for this event (aggregate ID)
func (e *Event) Key() string {
	return e.AggregateID
}
