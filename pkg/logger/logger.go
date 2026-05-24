// Package logger provides a production-ready structured logging solution
// with support for OpenTelemetry tracing, Kubernetes-native JSON output,
// and contextual logging with request correlation.
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"runtime"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// Level represents log levels
type Level = slog.Level

const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

// Logger wraps slog.Logger with additional functionality
type Logger struct {
	*slog.Logger
	serviceName string
	version     string
}

// Config holds logger configuration
type Config struct {
	Level       Level
	ServiceName string
	Version     string
	Environment string
	Output      io.Writer
	AddSource   bool
}

// DefaultConfig returns default logger configuration
func DefaultConfig() Config {
	return Config{
		Level:       LevelInfo,
		ServiceName: "eagle-bank",
		Version:     "unknown",
		Environment: "development",
		Output:      os.Stdout,
		AddSource:   true,
	}
}

// contextKey is used for storing logger in context
type contextKey struct{}

var defaultLogger *Logger

// New creates a new Logger instance
func New(cfg Config) *Logger {
	if cfg.Output == nil {
		cfg.Output = os.Stdout
	}

	opts := &slog.HandlerOptions{
		Level:     cfg.Level,
		AddSource: cfg.AddSource,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Customize time format for Kubernetes
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(a.Value.Time().Format(time.RFC3339Nano))
			}
			// Rename source to caller for clarity
			if a.Key == slog.SourceKey {
				a.Key = "caller"
			}
			return a
		},
	}

	handler := slog.NewJSONHandler(cfg.Output, opts)

	// Wrap handler with default attributes
	wrappedHandler := handler.WithAttrs([]slog.Attr{
		slog.String("service", cfg.ServiceName),
		slog.String("version", cfg.Version),
		slog.String("environment", cfg.Environment),
	})

	logger := &Logger{
		Logger:      slog.New(wrappedHandler),
		serviceName: cfg.ServiceName,
		version:     cfg.Version,
	}

	return logger
}

// Init initializes the default global logger
func Init(cfg Config) {
	defaultLogger = New(cfg)
	slog.SetDefault(defaultLogger.Logger)
}

// Default returns the default logger
func Default() *Logger {
	if defaultLogger == nil {
		Init(DefaultConfig())
	}
	return defaultLogger
}

// FromContext extracts logger from context or returns default
func FromContext(ctx context.Context) *Logger {
	if l, ok := ctx.Value(contextKey{}).(*Logger); ok {
		return l
	}
	return Default()
}

// WithContext returns a new context with the logger attached
func WithContext(ctx context.Context, l *Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}

// WithTraceContext adds OpenTelemetry trace context to log entries
func (l *Logger) WithTraceContext(ctx context.Context) *Logger {
	spanCtx := trace.SpanContextFromContext(ctx)
	if !spanCtx.IsValid() {
		return l
	}

	return &Logger{
		Logger: l.Logger.With(
			slog.String("trace_id", spanCtx.TraceID().String()),
			slog.String("span_id", spanCtx.SpanID().String()),
		),
		serviceName: l.serviceName,
		version:     l.version,
	}
}

// WithRequestID adds request ID to log entries
func (l *Logger) WithRequestID(requestID string) *Logger {
	return &Logger{
		Logger: l.Logger.With(
			slog.String("request_id", requestID),
		),
		serviceName: l.serviceName,
		version:     l.version,
	}
}

// WithUserID adds user ID to log entries
func (l *Logger) WithUserID(userID string) *Logger {
	return &Logger{
		Logger: l.Logger.With(
			slog.String("user_id", userID),
		),
		serviceName: l.serviceName,
		version:     l.version,
	}
}

// WithFields adds multiple fields to log entries
func (l *Logger) WithFields(fields map[string]any) *Logger {
	attrs := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		attrs = append(attrs, k, v)
	}
	return &Logger{
		Logger:      l.Logger.With(attrs...),
		serviceName: l.serviceName,
		version:     l.version,
	}
}

// WithError adds error context to log entries
func (l *Logger) WithError(err error) *Logger {
	if err == nil {
		return l
	}
	return &Logger{
		Logger: l.Logger.With(
			slog.String("error", err.Error()),
		),
		serviceName: l.serviceName,
		version:     l.version,
	}
}

// WithCaller adds caller information
func (l *Logger) WithCaller(skip int) *Logger {
	_, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return l
	}
	return &Logger{
		Logger: l.Logger.With(
			slog.String("caller", file),
			slog.Int("line", line),
		),
		serviceName: l.serviceName,
		version:     l.version,
	}
}

// Audit logs an audit event (always logged regardless of level)
func (l *Logger) Audit(ctx context.Context, action string, attrs ...any) {
	allAttrs := append([]any{
		"audit", true,
		"action", action,
	}, attrs...)
	l.WithTraceContext(ctx).InfoContext(ctx, "AUDIT: "+action, allAttrs...)
}

// Helper functions for default logger

// Debug logs at debug level using default logger
func Debug(msg string, args ...any) {
	Default().Debug(msg, args...)
}

// Info logs at info level using default logger
func Info(msg string, args ...any) {
	Default().Info(msg, args...)
}

// Warn logs at warn level using default logger
func Warn(msg string, args ...any) {
	Default().Warn(msg, args...)
}

// Error logs at error level using default logger
func Error(msg string, args ...any) {
	Default().Error(msg, args...)
}

// DebugContext logs at debug level with context
func DebugContext(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).WithTraceContext(ctx).DebugContext(ctx, msg, args...)
}

// InfoContext logs at info level with context
func InfoContext(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).WithTraceContext(ctx).InfoContext(ctx, msg, args...)
}

// WarnContext logs at warn level with context
func WarnContext(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).WithTraceContext(ctx).WarnContext(ctx, msg, args...)
}

// ErrorContext logs at error level with context
func ErrorContext(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).WithTraceContext(ctx).ErrorContext(ctx, msg, args...)
}
