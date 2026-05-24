// Package errors provides a production-ready error handling solution
// with error codes, stack traces, HTTP status mapping, and structured error responses.
package errors

import (
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strings"
)

// Code represents an error code for categorization
type Code string

const (
	// General errors
	CodeUnknown          Code = "UNKNOWN"
	CodeInternal         Code = "INTERNAL_ERROR"
	CodeInvalidInput     Code = "INVALID_INPUT"
	CodeNotFound         Code = "NOT_FOUND"
	CodeAlreadyExists    Code = "ALREADY_EXISTS"
	CodeUnauthorized     Code = "UNAUTHORIZED"
	CodeForbidden        Code = "FORBIDDEN"
	CodeConflict         Code = "CONFLICT"
	CodeRateLimited      Code = "RATE_LIMITED"
	CodeTimeout          Code = "TIMEOUT"
	CodeUnavailable      Code = "SERVICE_UNAVAILABLE"
	CodeUnprocessable    Code = "UNPROCESSABLE_ENTITY"

	// Domain-specific errors
	CodeInsufficientFunds   Code = "INSUFFICIENT_FUNDS"
	CodeAccountNotFound     Code = "ACCOUNT_NOT_FOUND"
	CodeUserNotFound        Code = "USER_NOT_FOUND"
	CodeTransactionNotFound Code = "TRANSACTION_NOT_FOUND"
	CodeInvalidAccountType  Code = "INVALID_ACCOUNT_TYPE"
	CodeInvalidAmount       Code = "INVALID_AMOUNT"
	CodeAccountHasBalance   Code = "ACCOUNT_HAS_BALANCE"
	CodeUserHasAccounts     Code = "USER_HAS_ACCOUNTS"
	CodeDuplicateEmail      Code = "DUPLICATE_EMAIL"
	CodeInvalidCredentials  Code = "INVALID_CREDENTIALS"
	CodeTokenExpired        Code = "TOKEN_EXPIRED"
	CodeInvalidToken        Code = "INVALID_TOKEN"
)

// httpStatusMap maps error codes to HTTP status codes
var httpStatusMap = map[Code]int{
	CodeUnknown:             http.StatusInternalServerError,
	CodeInternal:            http.StatusInternalServerError,
	CodeInvalidInput:        http.StatusBadRequest,
	CodeNotFound:            http.StatusNotFound,
	CodeAlreadyExists:       http.StatusConflict,
	CodeUnauthorized:        http.StatusUnauthorized,
	CodeForbidden:           http.StatusForbidden,
	CodeConflict:            http.StatusConflict,
	CodeRateLimited:         http.StatusTooManyRequests,
	CodeTimeout:             http.StatusGatewayTimeout,
	CodeUnavailable:         http.StatusServiceUnavailable,
	CodeUnprocessable:       http.StatusUnprocessableEntity,
	CodeInsufficientFunds:   http.StatusUnprocessableEntity,
	CodeAccountNotFound:     http.StatusNotFound,
	CodeUserNotFound:        http.StatusNotFound,
	CodeTransactionNotFound: http.StatusNotFound,
	CodeInvalidAccountType:  http.StatusBadRequest,
	CodeInvalidAmount:       http.StatusBadRequest,
	CodeAccountHasBalance:   http.StatusConflict,
	CodeUserHasAccounts:     http.StatusConflict,
	CodeDuplicateEmail:      http.StatusConflict,
	CodeInvalidCredentials:  http.StatusUnauthorized,
	CodeTokenExpired:        http.StatusUnauthorized,
	CodeInvalidToken:        http.StatusUnauthorized,
}

// Frame represents a single stack frame
type Frame struct {
	Function string `json:"function"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

// StackTrace represents the call stack
type StackTrace []Frame

// Error represents a structured error with code, message, and stack trace
type Error struct {
	code       Code
	message    string
	cause      error
	stack      StackTrace
	details    []FieldError
	metadata   map[string]any
	retryable  bool
	httpStatus int
}

// FieldError represents a validation error for a specific field
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Type    string `json:"type"`
}

// captureStack captures the current stack trace
func captureStack(skip int) StackTrace {
	const maxDepth = 32
	var pcs [maxDepth]uintptr
	n := runtime.Callers(skip+2, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])

	var stack StackTrace
	for {
		frame, more := frames.Next()
		// Skip runtime and standard library frames
		if !strings.Contains(frame.File, "runtime/") {
			stack = append(stack, Frame{
				Function: frame.Function,
				File:     frame.File,
				Line:     frame.Line,
			})
		}
		if !more {
			break
		}
	}
	return stack
}

// New creates a new Error with the given code and message
func New(code Code, message string) *Error {
	e := &Error{
		code:    code,
		message: message,
		stack:   captureStack(1),
	}
	if status, ok := httpStatusMap[code]; ok {
		e.httpStatus = status
	} else {
		e.httpStatus = http.StatusInternalServerError
	}
	return e
}

// Newf creates a new Error with a formatted message
func Newf(code Code, format string, args ...any) *Error {
	return New(code, fmt.Sprintf(format, args...))
}

// Wrap wraps an existing error with additional context
func Wrap(err error, code Code, message string) *Error {
	if err == nil {
		return nil
	}
	e := &Error{
		code:    code,
		message: message,
		cause:   err,
		stack:   captureStack(1),
	}
	if status, ok := httpStatusMap[code]; ok {
		e.httpStatus = status
	} else {
		e.httpStatus = http.StatusInternalServerError
	}
	return e
}

// Wrapf wraps an existing error with a formatted message
func Wrapf(err error, code Code, format string, args ...any) *Error {
	return Wrap(err, code, fmt.Sprintf(format, args...))
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.code, e.message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.code, e.message)
}

// Code returns the error code
func (e *Error) Code() Code {
	return e.code
}

// Message returns the error message
func (e *Error) Message() string {
	return e.message
}

// Cause returns the underlying error
func (e *Error) Cause() error {
	return e.cause
}

// Unwrap implements errors.Unwrap for error chain support
func (e *Error) Unwrap() error {
	return e.cause
}

// Stack returns the stack trace
func (e *Error) Stack() StackTrace {
	return e.stack
}

// Details returns field-level error details
func (e *Error) Details() []FieldError {
	return e.details
}

// HTTPStatus returns the HTTP status code for this error
func (e *Error) HTTPStatus() int {
	return e.httpStatus
}

// IsRetryable returns whether the error is retryable
func (e *Error) IsRetryable() bool {
	return e.retryable
}

// Metadata returns additional error metadata
func (e *Error) Metadata() map[string]any {
	return e.metadata
}

// WithDetails adds field-level error details
func (e *Error) WithDetails(details ...FieldError) *Error {
	e.details = append(e.details, details...)
	return e
}

// WithMetadata adds metadata to the error
func (e *Error) WithMetadata(key string, value any) *Error {
	if e.metadata == nil {
		e.metadata = make(map[string]any)
	}
	e.metadata[key] = value
	return e
}

// WithRetryable marks the error as retryable
func (e *Error) WithRetryable(retryable bool) *Error {
	e.retryable = retryable
	return e
}

// WithHTTPStatus overrides the HTTP status code
func (e *Error) WithHTTPStatus(status int) *Error {
	e.httpStatus = status
	return e
}

// Is checks if the target error matches this error's code
func (e *Error) Is(target error) bool {
	if t, ok := target.(*Error); ok {
		return e.code == t.code
	}
	return false
}

// Helper functions for common errors

// NotFound creates a not found error
func NotFound(resource string) *Error {
	return Newf(CodeNotFound, "%s not found", resource)
}

// InvalidInput creates an invalid input error
func InvalidInput(message string) *Error {
	return New(CodeInvalidInput, message)
}

// InvalidInputf creates an invalid input error with formatting
func InvalidInputf(format string, args ...any) *Error {
	return Newf(CodeInvalidInput, format, args...)
}

// Unauthorized creates an unauthorized error
func Unauthorized(message string) *Error {
	return New(CodeUnauthorized, message)
}

// Forbidden creates a forbidden error
func Forbidden(message string) *Error {
	return New(CodeForbidden, message)
}

// Internal creates an internal error
func Internal(message string) *Error {
	return New(CodeInternal, message)
}

// InternalWrap wraps an error as internal
func InternalWrap(err error, message string) *Error {
	return Wrap(err, CodeInternal, message)
}

// Conflict creates a conflict error
func Conflict(message string) *Error {
	return New(CodeConflict, message)
}

// RateLimited creates a rate limited error
func RateLimited(message string) *Error {
	return New(CodeRateLimited, message).WithRetryable(true)
}

// InsufficientFunds creates an insufficient funds error
func InsufficientFunds() *Error {
	return New(CodeInsufficientFunds, "insufficient funds for this transaction")
}

// AccountNotFound creates an account not found error
func AccountNotFound(accountNumber string) *Error {
	return Newf(CodeAccountNotFound, "account %s not found", accountNumber)
}

// UserNotFound creates a user not found error
func UserNotFound(userID string) *Error {
	return Newf(CodeUserNotFound, "user %s not found", userID)
}

// TransactionNotFound creates a transaction not found error
func TransactionNotFound(txnID string) *Error {
	return Newf(CodeTransactionNotFound, "transaction %s not found", txnID)
}

// DuplicateEmail creates a duplicate email error
func DuplicateEmail(email string) *Error {
	return New(CodeDuplicateEmail, "email already registered")
}

// InvalidCredentials creates an invalid credentials error
func InvalidCredentials() *Error {
	return New(CodeInvalidCredentials, "invalid email or password")
}

// TokenExpired creates a token expired error
func TokenExpired() *Error {
	return New(CodeTokenExpired, "authentication token has expired")
}

// InvalidToken creates an invalid token error
func InvalidToken() *Error {
	return New(CodeInvalidToken, "invalid authentication token")
}

// Utility functions

// GetCode extracts the error code from any error
func GetCode(err error) Code {
	var e *Error
	if errors.As(err, &e) {
		return e.Code()
	}
	return CodeUnknown
}

// GetHTTPStatus extracts the HTTP status from any error
func GetHTTPStatus(err error) int {
	var e *Error
	if errors.As(err, &e) {
		return e.HTTPStatus()
	}
	return http.StatusInternalServerError
}

// IsCode checks if an error has a specific code
func IsCode(err error, code Code) bool {
	return GetCode(err) == code
}

// IsNotFound checks if an error is a not found error
func IsNotFound(err error) bool {
	code := GetCode(err)
	return code == CodeNotFound ||
		code == CodeAccountNotFound ||
		code == CodeUserNotFound ||
		code == CodeTransactionNotFound
}

// IsValidation checks if an error is a validation error
func IsValidation(err error) bool {
	code := GetCode(err)
	return code == CodeInvalidInput ||
		code == CodeInvalidAccountType ||
		code == CodeInvalidAmount
}

// IsRetryableError checks if an error is retryable
func IsRetryableError(err error) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.IsRetryable()
	}
	return false
}

// ToResponse converts an error to an API response format
type ErrorResponse struct {
	Message string       `json:"message"`
	Code    Code         `json:"code,omitempty"`
	Details []FieldError `json:"details,omitempty"`
}

// ToResponse converts an error to ErrorResponse
func ToResponse(err error) ErrorResponse {
	var e *Error
	if errors.As(err, &e) {
		resp := ErrorResponse{
			Message: e.Message(),
			Code:    e.Code(),
		}
		if len(e.Details()) > 0 {
			resp.Details = e.Details()
		}
		return resp
	}
	return ErrorResponse{
		Message: err.Error(),
		Code:    CodeUnknown,
	}
}
