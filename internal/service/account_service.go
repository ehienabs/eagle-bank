package service

import (
	"context"
	"time"

	"github.com/ehienabs/eagle-bank/internal/domain"
	"github.com/ehienabs/eagle-bank/internal/kafka"
	"github.com/ehienabs/eagle-bank/internal/repository"
	"github.com/ehienabs/eagle-bank/pkg/errors"
	"github.com/ehienabs/eagle-bank/pkg/logger"
	"github.com/ehienabs/eagle-bank/pkg/metrics"
	"github.com/ehienabs/eagle-bank/pkg/tracing"
	"github.com/google/uuid"
)

// AccountService handles account business logic
type AccountService struct {
	db          *repository.DB
	accountRepo *repository.AccountRepository
	eventRepo   *repository.EventRepository
	producer    *kafka.Producer
	logger      *logger.Logger
	metrics     *metrics.Metrics
}

// NewAccountService creates a new AccountService
func NewAccountService(
	db *repository.DB,
	accountRepo *repository.AccountRepository,
	eventRepo *repository.EventRepository,
	producer *kafka.Producer,
) *AccountService {
	return &AccountService{
		db:          db,
		accountRepo: accountRepo,
		eventRepo:   eventRepo,
		producer:    producer,
		logger:      logger.Default(),
		metrics:     metrics.Default(),
	}
}

// CreateAccount creates a new bank account
func (s *AccountService) CreateAccount(ctx context.Context, userID string, req *domain.CreateAccountRequest) (*domain.Account, error) {
	ctx, span := tracing.StartServiceSpan(ctx, "AccountService", "CreateAccount")
	defer span.End()

	span.SetAttributes(tracing.UserIDAttr(userID))

	// Validate input
	if err := req.Validate(); err != nil {
		span.SetError(err)
		return nil, err
	}

	// Generate unique account number
	accountNumber, err := s.accountRepo.GenerateAccountNumber(ctx)
	if err != nil {
		span.SetError(err)
		return nil, err
	}

	// Create account
	now := time.Now()
	account := &domain.Account{
		ID:            "acc-" + uuid.New().String()[:8],
		AccountNumber: accountNumber,
		UserID:        userID,
		Name:          req.Name,
		AccountType:   req.AccountType,
		Balance:       0,
		Currency:      domain.DefaultCurrency,
		SortCode:      domain.DefaultSortCode,
		CreatedAt:     now,
		UpdatedAt:     now,
		Version:       1,
	}

	// Persist account
	if err := s.accountRepo.Create(ctx, account); err != nil {
		span.SetError(err)
		return nil, err
	}

	// Create and publish event
	event, err := domain.NewEvent(
		"evt-"+uuid.New().String()[:8],
		account.ID,
		domain.AggregateTypeAccount,
		domain.EventTypeAccountCreated,
		1,
		domain.AccountCreatedPayload{
			AccountNumber: account.AccountNumber,
			UserID:        account.UserID,
			Name:          account.Name,
			AccountType:   account.AccountType,
			Balance:       account.Balance,
			Currency:      account.Currency,
			SortCode:      account.SortCode,
		},
		&domain.EventMetadata{
			UserID:  userID,
			TraceID: tracing.TraceIDFromContext(ctx),
			SpanID:  tracing.SpanIDFromContext(ctx),
		},
	)
	if err == nil {
		go func() {
			pCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.producer.PublishEvent(pCtx, event); err != nil {
				s.logger.WithError(err).Error("failed to publish account created event")
			}
		}()
	}

	s.metrics.IncAccountsCreated()
	span.SetAttributes(tracing.AccountNumberAttr(accountNumber))
	span.SetOK()

	s.logger.Info("account created",
		"account_number", account.AccountNumber,
		"user_id", userID,
	)

	return account, nil
}

// GetAccount retrieves an account by account number
func (s *AccountService) GetAccount(ctx context.Context, userID, accountNumber string) (*domain.Account, error) {
	ctx, span := tracing.StartServiceSpan(ctx, "AccountService", "GetAccount")
	defer span.End()

	span.SetAttributes(
		tracing.UserIDAttr(userID),
		tracing.AccountNumberAttr(accountNumber),
	)

	account, err := s.accountRepo.GetByAccountNumber(ctx, accountNumber)
	if err != nil {
		span.SetError(err)
		return nil, err
	}

	// Check ownership
	if account.UserID != userID {
		return nil, errors.Forbidden("you do not have access to this account")
	}

	span.SetOK()
	return account, nil
}

// ListAccounts retrieves all accounts for a user
func (s *AccountService) ListAccounts(ctx context.Context, userID string) ([]*domain.Account, error) {
	ctx, span := tracing.StartServiceSpan(ctx, "AccountService", "ListAccounts")
	defer span.End()

	span.SetAttributes(tracing.UserIDAttr(userID))

	accounts, err := s.accountRepo.ListByUserID(ctx, userID)
	if err != nil {
		span.SetError(err)
		return nil, err
	}

	span.SetOK()
	return accounts, nil
}

// UpdateAccount updates an account
func (s *AccountService) UpdateAccount(ctx context.Context, userID, accountNumber string, req *domain.UpdateAccountRequest) (*domain.Account, error) {
	ctx, span := tracing.StartServiceSpan(ctx, "AccountService", "UpdateAccount")
	defer span.End()

	span.SetAttributes(
		tracing.UserIDAttr(userID),
		tracing.AccountNumberAttr(accountNumber),
	)

	// Get existing account
	account, err := s.accountRepo.GetByAccountNumber(ctx, accountNumber)
	if err != nil {
		span.SetError(err)
		return nil, err
	}

	// Check ownership
	if account.UserID != userID {
		return nil, errors.Forbidden("you do not have access to this account")
	}

	// Apply updates
	if req.Name != nil {
		account.Name = *req.Name
	}
	if req.AccountType != nil {
		if !req.AccountType.IsValid() {
			return nil, errors.InvalidInput("invalid account type")
		}
		account.AccountType = *req.AccountType
	}

	// Persist updates
	if err := s.accountRepo.Update(ctx, account); err != nil {
		span.SetError(err)
		return nil, err
	}

	// Create and publish event
	event, err := domain.NewEvent(
		"evt-"+uuid.New().String()[:8],
		account.ID,
		domain.AggregateTypeAccount,
		domain.EventTypeAccountUpdated,
		account.Version,
		domain.AccountUpdatedPayload{
			AccountNumber: account.AccountNumber,
			Name:          req.Name,
			AccountType:   req.AccountType,
		},
		&domain.EventMetadata{
			UserID:  userID,
			TraceID: tracing.TraceIDFromContext(ctx),
			SpanID:  tracing.SpanIDFromContext(ctx),
		},
	)
	if err == nil {
		go func() {
			if err := s.producer.PublishEvent(context.Background(), event); err != nil {
				s.logger.WithError(err).Error("failed to publish account updated event")
			}
		}()
	}

	span.SetOK()
	return account, nil
}

// DeleteAccount deletes an account
func (s *AccountService) DeleteAccount(ctx context.Context, userID, accountNumber string) error {
	ctx, span := tracing.StartServiceSpan(ctx, "AccountService", "DeleteAccount")
	defer span.End()

	span.SetAttributes(
		tracing.UserIDAttr(userID),
		tracing.AccountNumberAttr(accountNumber),
	)

	// Get existing account
	account, err := s.accountRepo.GetByAccountNumber(ctx, accountNumber)
	if err != nil {
		span.SetError(err)
		return err
	}

	// Check ownership
	if account.UserID != userID {
		return errors.Forbidden("you do not have access to this account")
	}

	// Delete account (will fail if balance is not zero)
	if err := s.accountRepo.Delete(ctx, accountNumber); err != nil {
		span.SetError(err)
		return err
	}

	// Create and publish event
	event, err := domain.NewEvent(
		"evt-"+uuid.New().String()[:8],
		account.ID,
		domain.AggregateTypeAccount,
		domain.EventTypeAccountDeleted,
		account.Version+1,
		domain.AccountDeletedPayload{
			AccountNumber: account.AccountNumber,
			UserID:        account.UserID,
		},
		&domain.EventMetadata{
			UserID:  userID,
			TraceID: tracing.TraceIDFromContext(ctx),
			SpanID:  tracing.SpanIDFromContext(ctx),
		},
	)
	if err == nil {
		go func() {
			if err := s.producer.PublishEvent(context.Background(), event); err != nil {
				s.logger.WithError(err).Error("failed to publish account deleted event")
			}
		}()
	}

	s.metrics.IncAccountsDeleted()
	span.SetOK()

	s.logger.Info("account deleted",
		"account_number", accountNumber,
		"user_id", userID,
	)

	return nil
}
