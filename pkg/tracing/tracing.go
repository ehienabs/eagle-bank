// Package tracing provides a production-ready OpenTelemetry tracing solution
// with support for HTTP, database, Kafka, and custom spans.
package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// Config holds tracing configuration
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	OTLPEndpoint   string
	SampleRate     float64
	Enabled        bool
}

// DefaultConfig returns default tracing configuration
func DefaultConfig() Config {
	return Config{
		ServiceName:    "eagle-bank",
		ServiceVersion: "unknown",
		Environment:    "development",
		OTLPEndpoint:   "localhost:4317",
		SampleRate:     1.0, // 100% sampling in dev
		Enabled:        true,
	}
}

// Tracer wraps OpenTelemetry tracer with convenience methods
type Tracer struct {
	tracer   trace.Tracer
	provider *sdktrace.TracerProvider
}

var defaultTracer *Tracer

// Init initializes the tracing system
func Init(ctx context.Context, cfg Config) (*Tracer, error) {
	if !cfg.Enabled {
		// Return a no-op tracer
		defaultTracer = &Tracer{
			tracer: otel.Tracer(cfg.ServiceName),
		}
		return defaultTracer, nil
	}

	// Create OTLP exporter
	client := otlptracegrpc.NewClient(
		otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlptracegrpc.WithInsecure(), // For local development; use TLS in production
	)

	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create resource with service information
	// Use only explicit attributes to avoid schema version conflicts from auto-detectors
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceVersion(cfg.ServiceVersion),
		semconv.DeploymentEnvironment(cfg.Environment),
		attribute.String("service.namespace", "eagle-bank"),
	)

	// Create sampler
	var sampler sdktrace.Sampler
	if cfg.SampleRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if cfg.SampleRate <= 0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	}

	// Create trace provider
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Set global provider
	otel.SetTracerProvider(provider)

	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	defaultTracer = &Tracer{
		tracer:   provider.Tracer(cfg.ServiceName),
		provider: provider,
	}

	return defaultTracer, nil
}

// Shutdown gracefully shuts down the tracer
func (t *Tracer) Shutdown(ctx context.Context) error {
	if t.provider != nil {
		return t.provider.Shutdown(ctx)
	}
	return nil
}

// Default returns the default tracer
func Default() *Tracer {
	if defaultTracer == nil {
		defaultTracer = &Tracer{
			tracer: otel.Tracer("eagle-bank"),
		}
	}
	return defaultTracer
}

// Start starts a new span
func (t *Tracer) Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, spanName, opts...)
}

// Span represents common span operations
type Span struct {
	span trace.Span
}

// WrapSpan wraps an OpenTelemetry span
func WrapSpan(span trace.Span) *Span {
	return &Span{span: span}
}

// SetAttributes sets attributes on the span
func (s *Span) SetAttributes(attrs ...attribute.KeyValue) {
	s.span.SetAttributes(attrs...)
}

// SetError records an error on the span
func (s *Span) SetError(err error) {
	if err != nil {
		s.span.RecordError(err)
		s.span.SetStatus(codes.Error, err.Error())
	}
}

// SetOK marks the span as successful
func (s *Span) SetOK() {
	s.span.SetStatus(codes.Ok, "")
}

// End ends the span
func (s *Span) End() {
	s.span.End()
}

// AddEvent adds an event to the span
func (s *Span) AddEvent(name string, attrs ...attribute.KeyValue) {
	s.span.AddEvent(name, trace.WithAttributes(attrs...))
}

// Helper functions for common tracing operations

// StartSpan starts a new span with the default tracer
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, *Span) {
	ctx, span := Default().Start(ctx, name, opts...)
	return ctx, WrapSpan(span)
}

// StartHTTPSpan starts a span for HTTP operations
func StartHTTPSpan(ctx context.Context, method, path string) (context.Context, *Span) {
	ctx, span := Default().Start(ctx, fmt.Sprintf("HTTP %s %s", method, path),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	span.SetAttributes(
		semconv.HTTPMethod(method),
		semconv.HTTPRoute(path),
	)
	return ctx, WrapSpan(span)
}

// StartDBSpan starts a span for database operations
func StartDBSpan(ctx context.Context, operation, table string) (context.Context, *Span) {
	ctx, span := Default().Start(ctx, fmt.Sprintf("DB %s %s", operation, table),
		trace.WithSpanKind(trace.SpanKindClient),
	)
	span.SetAttributes(
		semconv.DBSystemPostgreSQL,
		semconv.DBOperation(operation),
		semconv.DBSQLTable(table),
	)
	return ctx, WrapSpan(span)
}

// StartKafkaProducerSpan starts a span for Kafka producer operations
func StartKafkaProducerSpan(ctx context.Context, topic string) (context.Context, *Span) {
	ctx, span := Default().Start(ctx, fmt.Sprintf("Kafka Produce %s", topic),
		trace.WithSpanKind(trace.SpanKindProducer),
	)
	span.SetAttributes(
		semconv.MessagingSystem("kafka"),
		semconv.MessagingDestinationName(topic),
		semconv.MessagingOperationPublish,
	)
	return ctx, WrapSpan(span)
}

// StartKafkaConsumerSpan starts a span for Kafka consumer operations
func StartKafkaConsumerSpan(ctx context.Context, topic, consumerGroup string) (context.Context, *Span) {
	ctx, span := Default().Start(ctx, fmt.Sprintf("Kafka Consume %s", topic),
		trace.WithSpanKind(trace.SpanKindConsumer),
	)
	span.SetAttributes(
		semconv.MessagingSystem("kafka"),
		semconv.MessagingDestinationName(topic),
		semconv.MessagingKafkaConsumerGroup(consumerGroup),
		semconv.MessagingOperationReceive,
	)
	return ctx, WrapSpan(span)
}

// StartServiceSpan starts a span for service layer operations
func StartServiceSpan(ctx context.Context, service, method string) (context.Context, *Span) {
	ctx, span := Default().Start(ctx, fmt.Sprintf("%s.%s", service, method),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	span.SetAttributes(
		attribute.String("service.name", service),
		attribute.String("service.method", method),
	)
	return ctx, WrapSpan(span)
}

// Common attribute helpers

// UserIDAttr creates a user ID attribute
func UserIDAttr(userID string) attribute.KeyValue {
	return attribute.String("user.id", userID)
}

// AccountNumberAttr creates an account number attribute
func AccountNumberAttr(accountNumber string) attribute.KeyValue {
	return attribute.String("account.number", accountNumber)
}

// TransactionIDAttr creates a transaction ID attribute
func TransactionIDAttr(txnID string) attribute.KeyValue {
	return attribute.String("transaction.id", txnID)
}

// RequestIDAttr creates a request ID attribute
func RequestIDAttr(requestID string) attribute.KeyValue {
	return attribute.String("request.id", requestID)
}

// TransactionTypeAttr creates a transaction type attribute
func TransactionTypeAttr(txnType string) attribute.KeyValue {
	return attribute.String("transaction.type", txnType)
}

// AmountAttr creates an amount attribute
func AmountAttr(amount float64) attribute.KeyValue {
	return attribute.Float64("transaction.amount", amount)
}

// EventTypeAttr creates an event type attribute
func EventTypeAttr(eventType string) attribute.KeyValue {
	return attribute.String("event.type", eventType)
}

// SpanFromContext extracts the span from context
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// ContextWithSpan returns a context with the given span
func ContextWithSpan(ctx context.Context, span trace.Span) context.Context {
	return trace.ContextWithSpan(ctx, span)
}

// TraceIDFromContext extracts the trace ID from context
func TraceIDFromContext(ctx context.Context) string {
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		return spanCtx.TraceID().String()
	}
	return ""
}

// SpanIDFromContext extracts the span ID from context
func SpanIDFromContext(ctx context.Context) string {
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		return spanCtx.SpanID().String()
	}
	return ""
}

// InjectTraceContext injects trace context into a carrier
func InjectTraceContext(ctx context.Context, carrier propagation.TextMapCarrier) {
	otel.GetTextMapPropagator().Inject(ctx, carrier)
}

// ExtractTraceContext extracts trace context from a carrier
func ExtractTraceContext(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}
