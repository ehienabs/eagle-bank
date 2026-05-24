// Package service provides business logic layer
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

// UserService handles user business logic
type UserService struct {
	userRepo    *repository.UserRepository
	eventRepo   *repository.EventRepository
	producer    *kafka.Producer
	authService *AuthService
	logger      *logger.Logger
	metrics     *metrics.Metrics
}

// NewUserService creates a new UserService
func NewUserService(
	userRepo *repository.UserRepository,
	eventRepo *repository.EventRepository,
	producer *kafka.Producer,
	authService *AuthService,
) *UserService {
	return &UserService{
		userRepo:    userRepo,
		eventRepo:   eventRepo,
		producer:    producer,
		authService: authService,
		logger:      logger.Default(),
		metrics:     metrics.Default(),
	}
}

// CreateUser creates a new user
func (s *UserService) CreateUser(ctx context.Context, req *domain.CreateUserRequest) (*domain.User, error) {
	ctx, span := tracing.StartServiceSpan(ctx, "UserService", "CreateUser")
	defer span.End()

	// Validate input
	if err := req.Validate(); err != nil {
		span.SetError(err)
		return nil, err
	}

	// Check if email already exists
	exists, err := s.userRepo.ExistsByEmail(ctx, req.Email)
	if err != nil {
		span.SetError(err)
		return nil, err
	}
	if exists {
		return nil, errors.DuplicateEmail(req.Email)
	}

	// Hash password if provided
	var passwordHash string
	if req.Password != "" {
		passwordHash, err = s.authService.HashPassword(req.Password)
		if err != nil {
			span.SetError(err)
			return nil, err
		}
	}

	// Create user
	now := time.Now()
	user := &domain.User{
		ID:           "usr-" + uuid.New().String()[:8],
		Name:         req.Name,
		Email:        req.Email,
		PhoneNumber:  req.PhoneNumber,
		PasswordHash: passwordHash,
		Address:      req.Address,
		CreatedAt:    now,
		UpdatedAt:    now,
		Version:      1,
	}

	// Persist user
	if err := s.userRepo.Create(ctx, user); err != nil {
		span.SetError(err)
		return nil, err
	}

	// Create and publish event
	event, err := domain.NewEvent(
		"evt-"+uuid.New().String()[:8],
		user.ID,
		domain.AggregateTypeUser,
		domain.EventTypeUserCreated,
		1,
		domain.UserCreatedPayload{
			UserID:      user.ID,
			Name:        user.Name,
			Email:       user.Email,
			PhoneNumber: user.PhoneNumber,
			Address:     user.Address,
		},
		&domain.EventMetadata{
			TraceID: tracing.TraceIDFromContext(ctx),
			SpanID:  tracing.SpanIDFromContext(ctx),
		},
	)
	if err == nil {
		// Store event
		if err := s.eventRepo.Append(ctx, event); err != nil {
			s.logger.WithError(err).Warn("failed to store event")
		}

		// Publish event asynchronously
		go func() {
			pCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.producer.PublishEvent(pCtx, event); err != nil {
				s.logger.WithError(err).Error("failed to publish user created event")
			}
		}()
	}

	s.metrics.IncUsersRegistered()
	span.SetOK()

	s.logger.Info("user created",
		"user_id", user.ID,
		"email", user.Email,
	)

	return user, nil
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(ctx context.Context, id string) (*domain.User, error) {
	ctx, span := tracing.StartServiceSpan(ctx, "UserService", "GetUser")
	defer span.End()

	span.SetAttributes(tracing.UserIDAttr(id))

	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		span.SetError(err)
		return nil, err
	}

	span.SetOK()
	return user, nil
}

// UpdateUser updates a user
func (s *UserService) UpdateUser(ctx context.Context, id string, req *domain.UpdateUserRequest) (*domain.User, error) {
	ctx, span := tracing.StartServiceSpan(ctx, "UserService", "UpdateUser")
	defer span.End()

	span.SetAttributes(tracing.UserIDAttr(id))

	// Get existing user
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		span.SetError(err)
		return nil, err
	}

	// Apply updates
	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.Email != nil {
		// Check if new email is already taken by another user
		if *req.Email != user.Email {
			exists, err := s.userRepo.ExistsByEmail(ctx, *req.Email)
			if err != nil {
				span.SetError(err)
				return nil, err
			}
			if exists {
				return nil, errors.DuplicateEmail(*req.Email)
			}
			user.Email = *req.Email
		}
	}
	if req.PhoneNumber != nil {
		user.PhoneNumber = *req.PhoneNumber
	}
	if req.Address != nil {
		user.Address = *req.Address
	}

	// Persist updates
	if err := s.userRepo.Update(ctx, user); err != nil {
		span.SetError(err)
		return nil, err
	}

	// Create and publish event
	event, err := domain.NewEvent(
		"evt-"+uuid.New().String()[:8],
		user.ID,
		domain.AggregateTypeUser,
		domain.EventTypeUserUpdated,
		user.Version,
		domain.UserUpdatedPayload{
			UserID:      user.ID,
			Name:        req.Name,
			Email:       req.Email,
			PhoneNumber: req.PhoneNumber,
			Address:     req.Address,
		},
		&domain.EventMetadata{
			TraceID: tracing.TraceIDFromContext(ctx),
			SpanID:  tracing.SpanIDFromContext(ctx),
		},
	)
	if err == nil {
		go func() {
			if err := s.producer.PublishEvent(context.Background(), event); err != nil {
				s.logger.WithError(err).Error("failed to publish user updated event")
			}
		}()
	}

	span.SetOK()
	return user, nil
}

// DeleteUser deletes a user
func (s *UserService) DeleteUser(ctx context.Context, id string) error {
	ctx, span := tracing.StartServiceSpan(ctx, "UserService", "DeleteUser")
	defer span.End()

	span.SetAttributes(tracing.UserIDAttr(id))

	// Check if user exists
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		span.SetError(err)
		return err
	}

	// Delete user (will fail if user has accounts)
	if err := s.userRepo.Delete(ctx, id); err != nil {
		span.SetError(err)
		return err
	}

	// Create and publish event
	event, err := domain.NewEvent(
		"evt-"+uuid.New().String()[:8],
		user.ID,
		domain.AggregateTypeUser,
		domain.EventTypeUserDeleted,
		user.Version+1,
		domain.UserDeletedPayload{
			UserID: user.ID,
		},
		&domain.EventMetadata{
			TraceID: tracing.TraceIDFromContext(ctx),
			SpanID:  tracing.SpanIDFromContext(ctx),
		},
	)
	if err == nil {
		go func() {
			if err := s.producer.PublishEvent(context.Background(), event); err != nil {
				s.logger.WithError(err).Error("failed to publish user deleted event")
			}
		}()
	}

	span.SetOK()
	s.logger.Info("user deleted", "user_id", id)

	return nil
}
