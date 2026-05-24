package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"
	"github.com/ehienabs/eagle-bank/internal/config"
	"github.com/ehienabs/eagle-bank/internal/domain"
	"github.com/ehienabs/eagle-bank/internal/repository"
	"github.com/ehienabs/eagle-bank/pkg/errors"
	"github.com/ehienabs/eagle-bank/pkg/logger"
	"github.com/ehienabs/eagle-bank/pkg/metrics"
	"github.com/ehienabs/eagle-bank/pkg/tracing"
)

// AuthService handles authentication and authorization
type AuthService struct {
	userRepo *repository.UserRepository
	cfg      config.AuthConfig
	logger   *logger.Logger
	metrics  *metrics.Metrics
}

// NewAuthService creates a new AuthService
func NewAuthService(userRepo *repository.UserRepository, cfg config.AuthConfig) *AuthService {
	return &AuthService{
		userRepo: userRepo,
		cfg:      cfg,
		logger:   logger.Default(),
		metrics:  metrics.Default(),
	}
}

// Claims represents JWT claims
type Claims struct {
	UserID string `json:"sub"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// TokenPair represents an access and refresh token pair
type TokenPair struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken,omitempty"`
	ExpiresIn    int64  `json:"expiresIn"`
	TokenType    string `json:"tokenType"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// Login authenticates a user and returns a token pair
func (s *AuthService) Login(ctx context.Context, req *LoginRequest) (*TokenPair, error) {
	ctx, span := tracing.StartServiceSpan(ctx, "AuthService", "Login")
	defer span.End()

	// Get user by email
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.IsNotFound(err) {
			s.metrics.RecordAuthAttempt(false)
			return nil, errors.InvalidCredentials()
		}
		span.SetError(err)
		return nil, err
	}

	// Verify password
	if !s.VerifyPassword(req.Password, user.PasswordHash) {
		s.metrics.RecordAuthAttempt(false)
		return nil, errors.InvalidCredentials()
	}

	// Generate tokens
	accessToken, err := s.generateAccessToken(user)
	if err != nil {
		span.SetError(err)
		return nil, err
	}

	refreshToken, err := s.generateRefreshToken(user)
	if err != nil {
		span.SetError(err)
		return nil, err
	}

	s.metrics.RecordAuthAttempt(true)
	span.SetOK()

	s.logger.Info("user logged in", "user_id", user.ID)

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.cfg.JWTAccessExpiry.Seconds()),
		TokenType:    "Bearer",
	}, nil
}

// ValidateToken validates a JWT token and returns the claims
func (s *AuthService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.cfg.JWTSecret), nil
	})

	if err != nil {
		if strings.Contains(err.Error(), "expired") {
			return nil, errors.TokenExpired()
		}
		return nil, errors.InvalidToken()
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.InvalidToken()
	}

	return claims, nil
}

// RefreshToken refreshes an access token using a refresh token
func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	ctx, span := tracing.StartServiceSpan(ctx, "AuthService", "RefreshToken")
	defer span.End()

	claims, err := s.ValidateToken(refreshToken)
	if err != nil {
		span.SetError(err)
		return nil, err
	}

	// Get fresh user data
	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		span.SetError(err)
		return nil, err
	}

	// Generate new access token
	accessToken, err := s.generateAccessToken(user)
	if err != nil {
		span.SetError(err)
		return nil, err
	}

	span.SetOK()

	return &TokenPair{
		AccessToken: accessToken,
		ExpiresIn:   int64(s.cfg.JWTAccessExpiry.Seconds()),
		TokenType:   "Bearer",
	}, nil
}

// generateAccessToken generates an access token for a user
func (s *AuthService) generateAccessToken(user *domain.User) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID: user.ID,
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.cfg.JWTIssuer,
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.JWTAccessExpiry)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWTSecret))
}

// generateRefreshToken generates a refresh token for a user
func (s *AuthService) generateRefreshToken(user *domain.User) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID: user.ID,
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.cfg.JWTIssuer,
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.JWTRefreshExpiry)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWTSecret))
}

// HashPassword hashes a password using Argon2id
func (s *AuthService) HashPassword(password string) (string, error) {
	salt := make([]byte, s.cfg.Argon2SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", errors.InternalWrap(err, "failed to generate salt")
	}

	hash := argon2.IDKey(
		[]byte(password),
		salt,
		s.cfg.Argon2Time,
		s.cfg.Argon2Memory,
		s.cfg.Argon2Parallelism,
		s.cfg.Argon2KeyLength,
	)

	// Encode as: $argon2id$v=19$m=65536,t=3,p=4$<salt>$<hash>
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		s.cfg.Argon2Memory,
		s.cfg.Argon2Time,
		s.cfg.Argon2Parallelism,
		b64Salt,
		b64Hash,
	), nil
}

// VerifyPassword verifies a password against a hash
func (s *AuthService) VerifyPassword(password, encodedHash string) bool {
	// Parse the encoded hash
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false
	}

	var version int
	var memory, time uint32
	var parallelism uint8

	_, err := fmt.Sscanf(parts[2], "v=%d", &version)
	if err != nil {
		return false
	}

	_, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &parallelism)
	if err != nil {
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}

	// Compute hash with same parameters
	computedHash := argon2.IDKey(
		[]byte(password),
		salt,
		time,
		memory,
		parallelism,
		uint32(len(expectedHash)),
	)

	// Compare using constant-time comparison
	return subtle.ConstantTimeCompare(expectedHash, computedHash) == 1
}

// GetUserIDFromContext extracts user ID from context (set by auth middleware)
func GetUserIDFromContext(ctx context.Context) string {
	if userID, ok := ctx.Value(userIDKey).(string); ok {
		return userID
	}
	return ""
}

// userIDKey is the context key for user ID
type contextKeyType string

const userIDKey contextKeyType = "user_id"

// ContextWithUserID returns a context with user ID
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}
