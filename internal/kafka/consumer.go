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

// EventHandler is a function that handles domain events
type EventHandler func(ctx context.Context, event *domain.Event) error

// Consumer is a Kafka consumer group with observability and concurrency
type Consumer struct {
	client     sarama.ConsumerGroup
	cfg        config.KafkaConfig
	handlers   map[domain.EventType][]EventHandler
	metrics    *metrics.Metrics
	logger     *logger.Logger
	workerPool chan struct{}
	wg         sync.WaitGroup
	mu         sync.RWMutex
	closed     bool
	cancelFunc context.CancelFunc
}

// ConsumerOption configures a Consumer
type ConsumerOption func(*Consumer)

// WithConsumerMetrics sets custom metrics
func WithConsumerMetrics(m *metrics.Metrics) ConsumerOption {
	return func(c *Consumer) {
		c.metrics = m
	}
}

// WithConsumerLogger sets custom logger
func WithConsumerLogger(l *logger.Logger) ConsumerOption {
	return func(c *Consumer) {
		c.logger = l
	}
}

// NewConsumer creates a new Kafka consumer
func NewConsumer(cfg config.KafkaConfig, opts ...ConsumerOption) (*Consumer, error) {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Version = sarama.V3_0_0_0
	saramaConfig.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{
		sarama.NewBalanceStrategyRoundRobin(),
	}
	saramaConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
	if cfg.ConsumerOffsetReset == "latest" {
		saramaConfig.Consumer.Offsets.Initial = sarama.OffsetNewest
	}
	saramaConfig.Consumer.Offsets.AutoCommit.Enable = cfg.ConsumerAutoCommit
	saramaConfig.Consumer.MaxProcessingTime = 30 * time.Second

	client, err := sarama.NewConsumerGroup(cfg.Brokers, cfg.ConsumerGroup, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka consumer group: %w", err)
	}

	c := &Consumer{
		client:     client,
		cfg:        cfg,
		handlers:   make(map[domain.EventType][]EventHandler),
		metrics:    metrics.Default(),
		logger:     logger.Default(),
		workerPool: make(chan struct{}, cfg.ConsumerWorkers),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// RegisterHandler registers a handler for an event type
func (c *Consumer) RegisterHandler(eventType domain.EventType, handler EventHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[eventType] = append(c.handlers[eventType], handler)
}

// Start starts consuming messages from the specified topics
func (c *Consumer) Start(ctx context.Context, topics []string) error {
	ctx, cancel := context.WithCancel(ctx)
	c.cancelFunc = cancel

	handler := &consumerGroupHandler{
		consumer: c,
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		const (
			initialBackoff = 100 * time.Millisecond
			maxBackoff     = 30 * time.Second
		)
		backoff := initialBackoff
		for {
			select {
			case <-ctx.Done():
				return
			default:
				err := c.client.Consume(ctx, topics, handler)
				if err != nil {
					c.logger.Error("consumer group error", "error", err, "retry_backoff", backoff)
					select {
					case <-ctx.Done():
						return
					case <-time.After(backoff):
					}
					if backoff < maxBackoff {
						backoff *= 2
					}
				} else {
					// Normal session end (rebalance) — reset backoff.
					backoff = initialBackoff
				}
			}
		}
	}()

	// Handle errors
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-c.client.Errors():
				c.logger.Error("consumer error", "error", err)
			}
		}
	}()

	c.logger.Info("kafka consumer started", "topics", topics, "group", c.cfg.ConsumerGroup)
	return nil
}

// Stop stops the consumer
func (c *Consumer) Stop() error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()

	if c.cancelFunc != nil {
		c.cancelFunc()
	}

	c.wg.Wait()
	return c.client.Close()
}

// consumerGroupHandler implements sarama.ConsumerGroupHandler
type consumerGroupHandler struct {
	consumer *Consumer
}

func (h *consumerGroupHandler) Setup(session sarama.ConsumerGroupSession) error {
	h.consumer.logger.Info("consumer group session started",
		"member_id", session.MemberID(),
		"generation", session.GenerationID(),
	)
	return nil
}

func (h *consumerGroupHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	h.consumer.logger.Info("consumer group session ended",
		"member_id", session.MemberID(),
	)
	return nil
}

func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		h.consumer.processMessage(session.Context(), session, msg)
	}
	return nil
}

// processMessage processes a single Kafka message
func (c *Consumer) processMessage(ctx context.Context, session sarama.ConsumerGroupSession, msg *sarama.ConsumerMessage) {
	// Acquire worker slot
	c.workerPool <- struct{}{}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer func() { <-c.workerPool }()

		timer := metrics.NewTimer()

		// Extract trace context from headers
		carrier := &consumerHeaderCarrier{headers: msg.Headers}
		ctx = tracing.ExtractTraceContext(ctx, carrier)

		// Start consumer span
		ctx, span := tracing.StartKafkaConsumerSpan(ctx, msg.Topic, c.cfg.ConsumerGroup)
		defer span.End()

		// Parse event
		var event domain.Event
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			c.logger.Error("failed to unmarshal event",
				"error", err,
				"topic", msg.Topic,
				"partition", msg.Partition,
				"offset", msg.Offset,
			)
			span.SetError(err)
			c.metrics.RecordKafkaConsume(msg.Topic, c.cfg.ConsumerGroup, timer.Elapsed(), err)
			// Mark message as processed to avoid reprocessing invalid messages
			session.MarkMessage(msg, "")
			return
		}

		// Get handlers for this event type
		c.mu.RLock()
		handlers := c.handlers[event.EventType]
		c.mu.RUnlock()

		if len(handlers) == 0 {
			c.logger.Debug("no handlers for event type",
				"event_type", event.EventType,
				"event_id", event.ID,
			)
			session.MarkMessage(msg, "")
			return
		}

		// Execute handlers — each is retried up to 3 times with exponential
		// backoff before the failure is accepted. The message is always marked
		// afterwards to avoid blocking the consumer group; events that exhaust
		// retries are logged for external alerting / DLQ routing.
		const maxHandlerAttempts = 3
		var handlerErr error
		for _, handler := range handlers {
			handlerTimer := metrics.NewTimer()
			for attempt := 0; attempt < maxHandlerAttempts; attempt++ {
				if err := handler(ctx, &event); err != nil {
					if attempt == maxHandlerAttempts-1 {
						handlerErr = err
						c.logger.Error("handler failed after retries",
							"error", err,
							"event_type", event.EventType,
							"event_id", event.ID,
							"attempts", maxHandlerAttempts,
						)
						c.metrics.RecordEventProcessed(string(event.EventType), "handler", handlerTimer.Elapsed(), err)
					} else {
						backoff := time.Duration(1<<uint(attempt)) * 50 * time.Millisecond
						c.logger.Warn("handler error, retrying",
							"error", err,
							"event_type", event.EventType,
							"event_id", event.ID,
							"attempt", attempt+1,
							"backoff", backoff,
						)
						select {
						case <-ctx.Done():
							handlerErr = ctx.Err()
						case <-time.After(backoff):
						}
						if handlerErr != nil {
							break
						}
					}
				} else {
					c.metrics.RecordEventProcessed(string(event.EventType), "handler", handlerTimer.Elapsed(), nil)
					break
				}
			}
		}

		if handlerErr != nil {
			span.SetError(handlerErr)
		} else {
			span.SetOK()
		}

		c.metrics.RecordKafkaConsume(msg.Topic, c.cfg.ConsumerGroup, timer.Elapsed(), handlerErr)

		// Always mark the message so the consumer group makes progress.
		// Persistent failures are surfaced via error logs and metrics above.
		session.MarkMessage(msg, "")

		c.logger.Debug("message processed",
			"topic", msg.Topic,
			"partition", msg.Partition,
			"offset", msg.Offset,
			"event_type", event.EventType,
			"event_id", event.ID,
			"duration", timer.Elapsed(),
		)
	}()
}

// ConsumerGroupHealth returns the health status of the consumer group
type ConsumerGroupHealth struct {
	Healthy     bool             `json:"healthy"`
	Topics      []string         `json:"topics"`
	Group       string           `json:"group"`
	MemberCount int              `json:"memberCount"`
	Lag         map[string]int64 `json:"lag,omitempty"`
}

// Health returns the health status of the consumer
func (c *Consumer) Health(ctx context.Context) *ConsumerGroupHealth {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return &ConsumerGroupHealth{
		Healthy: !c.closed,
		Group:   c.cfg.ConsumerGroup,
	}
}

// consumerHeaderCarrier implements propagation.TextMapCarrier for Kafka consumer message headers
type consumerHeaderCarrier struct {
	headers []*sarama.RecordHeader
}

func (c *consumerHeaderCarrier) Get(key string) string {
	for _, h := range c.headers {
		if string(h.Key) == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c *consumerHeaderCarrier) Set(key string, value string) {
	// Consumer headers are read-only, but we need to implement the interface
	c.headers = append(c.headers, &sarama.RecordHeader{
		Key:   []byte(key),
		Value: []byte(value),
	})
}

func (c *consumerHeaderCarrier) Keys() []string {
	keys := make([]string, len(c.headers))
	for i, h := range c.headers {
		keys[i] = string(h.Key)
	}
	return keys
}

// Ensure consumerHeaderCarrier implements propagation.TextMapCarrier
var _ propagation.TextMapCarrier = (*consumerHeaderCarrier)(nil)
