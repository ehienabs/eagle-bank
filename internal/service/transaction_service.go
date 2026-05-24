package service

import (
	"context"
	"database/sql"
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

// TransactionService handles transaction business logic
type TransactionService struct {
	db          *repository.DB
	txnRepo     *repository.TransactionRepository
	accountRepo *repository.AccountRepository
	eventRepo   *repository.EventRepository
	producer    *kafka.Producer
	logger      *logger.Logger
	metrics     *metrics.Metrics
}

// NewTransactionService creates a new TransactionService
func NewTransactionService(
	db *repository.DB,
	txnRepo *repository.TransactionRepository,
	accountRepo *repository.AccountRepository,
	eventRepo *repository.EventRepository,
	producer *kafka.Producer,
) *TransactionService {
	return &TransactionService{
		db:          db,
		txnRepo:     txnRepo,
		accountRepo: accountRepo,
		eventRepo:   eventRepo,
		producer:    producer,
		logger:      logger.Default(),
		metrics:     metrics.Default(),
	}
}

// CreateTransaction creates a new transaction (deposit or withdrawal)
func (s *TransactionService) CreateTransaction(ctx context.Context, userID, accountNumber string, req *domain.CreateTransactionRequest) (*domain.Transaction, error) {
	ctx, span := tracing.StartServiceSpan(ctx, "TransactionService", "CreateTransaction")
	defer span.End()

	span.SetAttributes(
		tracing.UserIDAttr(userID),
		tracing.AccountNumberAttr(accountNumber),
		tracing.TransactionTypeAttr(string(req.Type)),
		tracing.AmountAttr(req.Amount),
	)

	// Validate input
	if err := req.Validate(); err != nil {
		span.SetError(err)
		return nil, err
	}

	// Convert amount to cents
	amountCents := domain.AmountToCents(req.Amount)

	// Apply an explicit deadline so a slow DB cannot hold the goroutine for
	// the full HTTP write timeout. The caller's context is still the parent,
	// so a client disconnect cancels this too.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Retry on PostgreSQL serialization failures (SQLSTATE 40001). These
	// mean "no change was made, try again" — they are safe to retry.
	const maxAttempts = 3
	var transaction *domain.Transaction
	for attempt := 0; attempt < maxAttempts; attempt++ {
		var txErr error
		transaction, txErr = s.attemptTransaction(ctx, userID, accountNumber, amountCents, req)
		if txErr == nil {
			break
		}
		if !repository.IsSerializationFailure(txErr) || attempt == maxAttempts-1 {
			span.SetError(txErr)
			return nil, txErr
		}
		s.logger.Warn("serialization failure, retrying",
			"attempt", attempt+1,
			"account_number", accountNumber,
		)
		// Exponential backoff: 10ms, 20ms between attempts.
		time.Sleep(time.Duration(1<<uint(attempt)) * 10 * time.Millisecond)
	}

	// Create and publish event
	event, err := domain.NewEvent(
		"evt-"+uuid.New().String()[:8],
		transaction.ID,
		domain.AggregateTypeTransaction,
		domain.EventTypeTransactionCompleted,
		1,
		domain.TransactionCompletedPayload{
			TransactionID: transaction.ID,
			AccountNumber: transaction.AccountNumber,
			UserID:        userID,
			Type:          transaction.Type,
			Amount:        transaction.Amount,
			Currency:      transaction.Currency,
			Reference:     transaction.Reference,
			BalanceBefore: transaction.BalanceBefore,
			BalanceAfter:  transaction.BalanceAfter,
			Status:        string(transaction.Status),
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
				s.logger.WithError(err).Error("failed to publish transaction event")
			}
		}()
	}

	// Record metrics
	s.metrics.RecordTransaction(string(req.Type), req.Amount, nil)

	span.SetAttributes(tracing.TransactionIDAttr(transaction.ID))
	span.SetOK()

	s.logger.Info("transaction created",
		"transaction_id", transaction.ID,
		"account_number", accountNumber,
		"type", req.Type,
		"amount", req.Amount,
		"new_balance", domain.CentsToAmount(transaction.BalanceAfter),
	)

	return transaction, nil
}

// attemptTransaction executes one attempt of the full DB transaction:
// lock account row → validate balance → insert transaction → update balance → commit.
// It is called by CreateTransaction and may be retried on serialization failures.
func (s *TransactionService) attemptTransaction(ctx context.Context, userID, accountNumber string, amountCents int64, req *domain.CreateTransactionRequest) (*domain.Transaction, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})
	if err != nil {
		return nil, errors.InternalWrap(err, "failed to begin transaction")
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Get account with pessimistic row lock.
	account, err := s.accountRepo.GetByAccountNumberForUpdate(ctx, tx, accountNumber)
	if err != nil {
		return nil, err
	}

	if account.UserID != userID {
		return nil, errors.Forbidden("you do not have access to this account")
	}

	var newBalance int64
	switch req.Type {
	case domain.TransactionTypeDeposit:
		newBalance = account.Balance + amountCents
		if newBalance > domain.MaxBalance {
			return nil, errors.InvalidInput("deposit would exceed maximum balance")
		}
	case domain.TransactionTypeWithdrawal:
		if account.Balance < amountCents {
			return nil, errors.InsufficientFunds()
		}
		newBalance = account.Balance - amountCents
	default:
		return nil, errors.InvalidInput("invalid transaction type")
	}

	now := time.Now()
	transaction := &domain.Transaction{
		ID:            "tan-" + uuid.New().String()[:8],
		AccountID:     account.ID,
		AccountNumber: account.AccountNumber,
		UserID:        userID,
		Type:          req.Type,
		Amount:        amountCents,
		Currency:      req.Currency,
		Reference:     req.Reference,
		BalanceBefore: account.Balance,
		BalanceAfter:  newBalance,
		Status:        domain.TransactionStatusCompleted,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.txnRepo.Create(ctx, tx, transaction); err != nil {
		return nil, err
	}

	if err := s.accountRepo.UpdateBalance(ctx, tx, account.ID, newBalance, account.Version); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, errors.InternalWrap(err, "failed to commit transaction")
	}

	return transaction, nil
}

// GetTransaction retrieves a transaction by ID
func (s *TransactionService) GetTransaction(ctx context.Context, userID, accountNumber, transactionID string) (*domain.Transaction, error) {
	ctx, span := tracing.StartServiceSpan(ctx, "TransactionService", "GetTransaction")
	defer span.End()

	span.SetAttributes(
		tracing.UserIDAttr(userID),
		tracing.AccountNumberAttr(accountNumber),
		tracing.TransactionIDAttr(transactionID),
	)

	// Get transaction
	transaction, err := s.txnRepo.GetByIDAndAccountNumber(ctx, transactionID, accountNumber)
	if err != nil {
		span.SetError(err)
		return nil, err
	}

	// Check ownership
	if transaction.UserID != userID {
		return nil, errors.Forbidden("you do not have access to this transaction")
	}

	span.SetOK()
	return transaction, nil
}

// ListTransactions retrieves all transactions for an account
func (s *TransactionService) ListTransactions(ctx context.Context, userID, accountNumber string) ([]*domain.Transaction, error) {
	ctx, span := tracing.StartServiceSpan(ctx, "TransactionService", "ListTransactions")
	defer span.End()

	span.SetAttributes(
		tracing.UserIDAttr(userID),
		tracing.AccountNumberAttr(accountNumber),
	)

	// Verify account ownership first
	account, err := s.accountRepo.GetByAccountNumber(ctx, accountNumber)
	if err != nil {
		span.SetError(err)
		return nil, err
	}

	if account.UserID != userID {
		return nil, errors.Forbidden("you do not have access to this account")
	}

	// Get transactions
	transactions, err := s.txnRepo.ListByAccountNumber(ctx, accountNumber)
	if err != nil {
		span.SetError(err)
		return nil, err
	}

	span.SetOK()
	return transactions, nil
}
