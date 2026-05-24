// Package kafka provides Kafka producer and consumer with observability support.
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/ehienabs/eagle-bank/internal/config"
	"github.com/ehienabs/eagle-bank/internal/domain"
	"github.com/ehienabs/eagle-bank/pkg/logger"
	"github.com/ehienabs/eagle-bank/pkg/metrics"
	"github.com/ehienabs/eagle-bank/pkg/tracing"
	"go.opentelemetry.io/otel/propagation"
)

// Producer is a Kafka producer with observability
type Producer struct {
	producer  sarama.AsyncProducer
	cfg       config.KafkaConfig
	metrics   *metrics.Metrics
	logger    *logger.Logger
	wg        sync.WaitGroup // internal goroutines (handleSuccesses/handleErrors)
	publishWg sync.WaitGroup // in-flight PublishEvent calls
	closed    bool
	mu        sync.RWMutex
}

// ProducerOption configures a Producer
type ProducerOption func(*Producer)

// WithProducerMetrics sets custom metrics
func WithProducerMetrics(m *metrics.Metrics) ProducerOption {
	return func(p *Producer) {
		p.metrics = m
	}
}

// WithProducerLogger sets custom logger
func WithProducerLogger(l *logger.Logger) ProducerOption {
	return func(p *Producer) {
		p.logger = l
	}
}

// NewProducer creates a new Kafka producer
func NewProducer(cfg config.KafkaConfig, opts ...ProducerOption) (*Producer, error) {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Version = sarama.V3_0_0_0

	// Producer settings
	saramaConfig.Producer.Return.Successes = true
	saramaConfig.Producer.Return.Errors = true
	saramaConfig.Producer.RequiredAcks = sarama.WaitForAll
	saramaConfig.Producer.Retry.Max = cfg.ProducerRetries
	saramaConfig.Producer.Flush.Bytes = cfg.ProducerBatchSize
	saramaConfig.Producer.Flush.Frequency = time.Duration(cfg.ProducerLingerMs) * time.Millisecond

	// Enable idempotent producer for exactly-once semantics
	saramaConfig.Producer.Idempotent = true
	saramaConfig.Net.MaxOpenRequests = 1 // Required for idempotent producer

	producer, err := sarama.NewAsyncProducer(cfg.Brokers, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka producer: %w", err)
	}

	p := &Producer{
		producer: producer,
		cfg:      cfg,
		metrics:  metrics.Default(),
		logger:   logger.Default(),
	}

	for _, opt := range opts {
		opt(p)
	}

	// Start success/error handlers
	p.wg.Add(2)
	go p.handleSuccesses()
	go p.handleErrors()

	return p, nil
}

// handleSuccesses handles successful message deliveries
func (p *Producer) handleSuccesses() {
	defer p.wg.Done()
	for msg := range p.producer.Successes() {
		p.logger.Debug("kafka message delivered",
			"topic", msg.Topic,
			"partition", msg.Partition,
			"offset", msg.Offset,
		)
	}
}

// handleErrors handles failed message deliveries
func (p *Producer) handleErrors() {
	defer p.wg.Done()
	for err := range p.producer.Errors() {
		p.logger.Error("kafka message delivery failed",
			"topic", err.Msg.Topic,
			"error", err.Error(),
		)
		p.metrics.RecordKafkaProduce(err.Msg.Topic, 0, err.Err)
	}
}

// PublishEvent publishes a domain event to Kafka
func (p *Producer) PublishEvent(ctx context.Context, event *domain.Event) error {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return fmt.Errorf("producer is closed")
	}
	p.publishWg.Add(1)
	p.mu.RUnlock()
	defer p.publishWg.Done()

	timer := metrics.NewTimer()

	// Start tracing span
	ctx, span := tracing.StartKafkaProducerSpan(ctx, event.Topic())
	defer span.End()

	// Serialize event
	value, err := json.Marshal(event)
	if err != nil {
		span.SetError(err)
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Create message with trace context in headers
	msg := &sarama.ProducerMessage{
		Topic: event.Topic(),
		Key:   sarama.StringEncoder(event.Key()),
		Value: sarama.ByteEncoder(value),
		Headers: []sarama.RecordHeader{
			{Key: []byte("event_type"), Value: []byte(event.EventType)},
			{Key: []byte("aggregate_type"), Value: []byte(event.AggregateType)},
			{Key: []byte("event_id"), Value: []byte(event.ID)},
		},
		Timestamp: event.Timestamp,
	}

	// Inject trace context into headers
	carrier := &kafkaHeaderCarrier{headers: &msg.Headers}
	tracing.InjectTraceContext(ctx, carrier)

	// Send message
	select {
	case p.producer.Input() <- msg:
		p.metrics.RecordKafkaProduce(event.Topic(), timer.Elapsed(), nil)
		p.metrics.RecordEventPublished(string(event.EventType), nil)
		span.SetOK()
		return nil
	case <-ctx.Done():
		span.SetError(ctx.Err())
		return ctx.Err()
	}
}

// PublishEvents publishes multiple events
func (p *Producer) PublishEvents(ctx context.Context, events []*domain.Event) error {
	for _, event := range events {
		if err := p.PublishEvent(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

// Close closes the producer, waiting for all in-flight publishes to finish first.
func (p *Producer) Close() error {
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()

	// Wait for any goroutines still inside PublishEvent to finish.
	p.publishWg.Wait()

	if err := p.producer.Close(); err != nil {
		return err
	}
	p.wg.Wait()
	return nil
}

// kafkaHeaderCarrier implements propagation.TextMapCarrier for Kafka headers
type kafkaHeaderCarrier struct {
	headers *[]sarama.RecordHeader
}

func (c *kafkaHeaderCarrier) Get(key string) string {
	for _, h := range *c.headers {
		if string(h.Key) == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c *kafkaHeaderCarrier) Set(key string, value string) {
	*c.headers = append(*c.headers, sarama.RecordHeader{
		Key:   []byte(key),
		Value: []byte(value),
	})
}

func (c *kafkaHeaderCarrier) Keys() []string {
	keys := make([]string, len(*c.headers))
	for i, h := range *c.headers {
		keys[i] = string(h.Key)
	}
	return keys
}

// Ensure kafkaHeaderCarrier implements propagation.TextMapCarrier
var _ propagation.TextMapCarrier = (*kafkaHeaderCarrier)(nil)
