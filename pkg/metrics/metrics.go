// Package metrics provides a production-ready Prometheus metrics solution
// with HTTP, database, Kafka, and business-specific metrics.
package metrics

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics for the application
type Metrics struct {
	// HTTP metrics
	httpRequestsTotal    *prometheus.CounterVec
	httpRequestDuration  *prometheus.HistogramVec
	httpRequestsInFlight prometheus.Gauge
	httpResponseSize     *prometheus.HistogramVec

	// Database metrics
	dbQueryTotal       *prometheus.CounterVec
	dbQueryDuration    *prometheus.HistogramVec
	dbConnectionsOpen  prometheus.Gauge
	dbConnectionsInUse prometheus.Gauge

	// Kafka metrics
	kafkaMessagesProduced *prometheus.CounterVec
	kafkaMessagesConsumed *prometheus.CounterVec
	kafkaProduceDuration  *prometheus.HistogramVec
	kafkaConsumeDuration  *prometheus.HistogramVec
	kafkaConsumerLag      *prometheus.GaugeVec

	// Business metrics
	transactionsTotal    *prometheus.CounterVec
	transactionAmount    *prometheus.HistogramVec
	accountsCreated      prometheus.Counter
	accountsDeleted      prometheus.Counter
	usersRegistered      prometheus.Counter
	authAttempts         *prometheus.CounterVec
	activeUsers          prometheus.Gauge
	accountBalance       *prometheus.GaugeVec

	// Event processing metrics
	eventsPublished     *prometheus.CounterVec
	eventsProcessed     *prometheus.CounterVec
	eventProcessingTime *prometheus.HistogramVec
	eventRetries        *prometheus.CounterVec

	// Rate limiting metrics
	rateLimitHits   *prometheus.CounterVec
	rateLimitTokens *prometheus.GaugeVec

	// Circuit breaker metrics
	circuitBreakerState   *prometheus.GaugeVec
	circuitBreakerTrips   *prometheus.CounterVec

	// Cache metrics
	cacheHits   *prometheus.CounterVec
	cacheMisses *prometheus.CounterVec

	namespace string
	subsystem string
}

// Config holds metrics configuration
type Config struct {
	Namespace   string
	Subsystem   string
	ServiceName string
}

// DefaultConfig returns default metrics configuration
func DefaultConfig() Config {
	return Config{
		Namespace:   "eagle_bank",
		Subsystem:   "api",
		ServiceName: "eagle-bank",
	}
}

// New creates a new Metrics instance with all metrics registered
func New(cfg Config) *Metrics {
	m := &Metrics{
		namespace: cfg.Namespace,
		subsystem: cfg.Subsystem,
	}

	// Define histogram buckets
	httpDurationBuckets := []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}
	dbDurationBuckets := []float64{.0001, .0005, .001, .005, .01, .025, .05, .1, .25, .5, 1}
	kafkaDurationBuckets := []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5}
	amountBuckets := []float64{1, 10, 50, 100, 500, 1000, 5000, 10000}

	// HTTP metrics
	m.httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	m.httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   httpDurationBuckets,
		},
		[]string{"method", "path", "status"},
	)

	m.httpRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "http_requests_in_flight",
			Help:      "Current number of HTTP requests being processed",
		},
	)

	m.httpResponseSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "http_response_size_bytes",
			Help:      "HTTP response size in bytes",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 7),
		},
		[]string{"method", "path"},
	)

	// Database metrics
	m.dbQueryTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: "db",
			Name:      "queries_total",
			Help:      "Total number of database queries",
		},
		[]string{"operation", "table", "status"},
	)

	m.dbQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: cfg.Namespace,
			Subsystem: "db",
			Name:      "query_duration_seconds",
			Help:      "Database query duration in seconds",
			Buckets:   dbDurationBuckets,
		},
		[]string{"operation", "table"},
	)

	m.dbConnectionsOpen = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: cfg.Namespace,
			Subsystem: "db",
			Name:      "connections_open",
			Help:      "Current number of open database connections",
		},
	)

	m.dbConnectionsInUse = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: cfg.Namespace,
			Subsystem: "db",
			Name:      "connections_in_use",
			Help:      "Current number of database connections in use",
		},
	)

	// Kafka metrics
	m.kafkaMessagesProduced = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: "kafka",
			Name:      "messages_produced_total",
			Help:      "Total number of messages produced to Kafka",
		},
		[]string{"topic", "status"},
	)

	m.kafkaMessagesConsumed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: "kafka",
			Name:      "messages_consumed_total",
			Help:      "Total number of messages consumed from Kafka",
		},
		[]string{"topic", "consumer_group", "status"},
	)

	m.kafkaProduceDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: cfg.Namespace,
			Subsystem: "kafka",
			Name:      "produce_duration_seconds",
			Help:      "Kafka message produce duration in seconds",
			Buckets:   kafkaDurationBuckets,
		},
		[]string{"topic"},
	)

	m.kafkaConsumeDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: cfg.Namespace,
			Subsystem: "kafka",
			Name:      "consume_duration_seconds",
			Help:      "Kafka message consume/processing duration in seconds",
			Buckets:   kafkaDurationBuckets,
		},
		[]string{"topic", "consumer_group"},
	)

	m.kafkaConsumerLag = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: cfg.Namespace,
			Subsystem: "kafka",
			Name:      "consumer_lag",
			Help:      "Kafka consumer lag (messages behind)",
		},
		[]string{"topic", "partition", "consumer_group"},
	)

	// Business metrics
	m.transactionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: "business",
			Name:      "transactions_total",
			Help:      "Total number of transactions",
		},
		[]string{"type", "status"},
	)

	m.transactionAmount = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: cfg.Namespace,
			Subsystem: "business",
			Name:      "transaction_amount_gbp",
			Help:      "Transaction amount in GBP",
			Buckets:   amountBuckets,
		},
		[]string{"type"},
	)

	m.accountsCreated = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: "business",
			Name:      "accounts_created_total",
			Help:      "Total number of accounts created",
		},
	)

	m.accountsDeleted = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: "business",
			Name:      "accounts_deleted_total",
			Help:      "Total number of accounts deleted",
		},
	)

	m.usersRegistered = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: "business",
			Name:      "users_registered_total",
			Help:      "Total number of users registered",
		},
	)

	m.authAttempts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: "business",
			Name:      "auth_attempts_total",
			Help:      "Total number of authentication attempts",
		},
		[]string{"result"},
	)

	m.activeUsers = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: cfg.Namespace,
			Subsystem: "business",
			Name:      "active_users",
			Help:      "Current number of active users",
		},
	)

	m.accountBalance = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: cfg.Namespace,
			Subsystem: "business",
			Name:      "account_balance_total_gbp",
			Help:      "Total balance across all accounts in GBP",
		},
		[]string{"account_type"},
	)

	// Event processing metrics
	m.eventsPublished = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: "events",
			Name:      "published_total",
			Help:      "Total number of events published",
		},
		[]string{"event_type", "status"},
	)

	m.eventsProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: "events",
			Name:      "processed_total",
			Help:      "Total number of events processed",
		},
		[]string{"event_type", "handler", "status"},
	)

	m.eventProcessingTime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: cfg.Namespace,
			Subsystem: "events",
			Name:      "processing_duration_seconds",
			Help:      "Event processing duration in seconds",
			Buckets:   kafkaDurationBuckets,
		},
		[]string{"event_type", "handler"},
	)

	m.eventRetries = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: "events",
			Name:      "retries_total",
			Help:      "Total number of event processing retries",
		},
		[]string{"event_type", "handler"},
	)

	// Rate limiting metrics
	m.rateLimitHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: "ratelimit",
			Name:      "hits_total",
			Help:      "Total number of rate limit hits",
		},
		[]string{"endpoint", "user_type"},
	)

	m.rateLimitTokens = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: cfg.Namespace,
			Subsystem: "ratelimit",
			Name:      "available_tokens",
			Help:      "Current number of available rate limit tokens",
		},
		[]string{"endpoint"},
	)

	// Circuit breaker metrics
	m.circuitBreakerState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: cfg.Namespace,
			Subsystem: "circuit_breaker",
			Name:      "state",
			Help:      "Current circuit breaker state (0=closed, 1=half-open, 2=open)",
		},
		[]string{"name"},
	)

	m.circuitBreakerTrips = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: "circuit_breaker",
			Name:      "trips_total",
			Help:      "Total number of circuit breaker trips",
		},
		[]string{"name"},
	)

	// Cache metrics
	m.cacheHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: "cache",
			Name:      "hits_total",
			Help:      "Total number of cache hits",
		},
		[]string{"cache_name"},
	)

	m.cacheMisses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: "cache",
			Name:      "misses_total",
			Help:      "Total number of cache misses",
		},
		[]string{"cache_name"},
	)

	return m
}

var defaultMetrics *Metrics

// Init initializes the default global metrics
func Init(cfg Config) {
	defaultMetrics = New(cfg)
}

// Default returns the default metrics instance
func Default() *Metrics {
	if defaultMetrics == nil {
		Init(DefaultConfig())
	}
	return defaultMetrics
}

// Handler returns the Prometheus HTTP handler
func Handler() http.Handler {
	return promhttp.Handler()
}

// HTTP metric recording methods

// RecordHTTPRequest records an HTTP request
func (m *Metrics) RecordHTTPRequest(method, path string, status int, duration time.Duration, responseSize int) {
	statusStr := strconv.Itoa(status)
	m.httpRequestsTotal.WithLabelValues(method, path, statusStr).Inc()
	m.httpRequestDuration.WithLabelValues(method, path, statusStr).Observe(duration.Seconds())
	m.httpResponseSize.WithLabelValues(method, path).Observe(float64(responseSize))
}

// IncHTTPInFlight increments in-flight requests
func (m *Metrics) IncHTTPInFlight() {
	m.httpRequestsInFlight.Inc()
}

// DecHTTPInFlight decrements in-flight requests
func (m *Metrics) DecHTTPInFlight() {
	m.httpRequestsInFlight.Dec()
}

// Database metric recording methods

// RecordDBQuery records a database query
func (m *Metrics) RecordDBQuery(operation, table string, duration time.Duration, err error) {
	status := "success"
	if err != nil {
		status = "error"
	}
	m.dbQueryTotal.WithLabelValues(operation, table, status).Inc()
	m.dbQueryDuration.WithLabelValues(operation, table).Observe(duration.Seconds())
}

// SetDBConnections sets the database connection counts
func (m *Metrics) SetDBConnections(open, inUse int) {
	m.dbConnectionsOpen.Set(float64(open))
	m.dbConnectionsInUse.Set(float64(inUse))
}

// Kafka metric recording methods

// RecordKafkaProduce records a Kafka produce operation
func (m *Metrics) RecordKafkaProduce(topic string, duration time.Duration, err error) {
	status := "success"
	if err != nil {
		status = "error"
	}
	m.kafkaMessagesProduced.WithLabelValues(topic, status).Inc()
	m.kafkaProduceDuration.WithLabelValues(topic).Observe(duration.Seconds())
}

// RecordKafkaConsume records a Kafka consume operation
func (m *Metrics) RecordKafkaConsume(topic, consumerGroup string, duration time.Duration, err error) {
	status := "success"
	if err != nil {
		status = "error"
	}
	m.kafkaMessagesConsumed.WithLabelValues(topic, consumerGroup, status).Inc()
	m.kafkaConsumeDuration.WithLabelValues(topic, consumerGroup).Observe(duration.Seconds())
}

// SetKafkaConsumerLag sets the Kafka consumer lag
func (m *Metrics) SetKafkaConsumerLag(topic, partition, consumerGroup string, lag int64) {
	m.kafkaConsumerLag.WithLabelValues(topic, partition, consumerGroup).Set(float64(lag))
}

// Business metric recording methods

// RecordTransaction records a transaction
func (m *Metrics) RecordTransaction(txnType string, amount float64, err error) {
	status := "success"
	if err != nil {
		status = "error"
	}
	m.transactionsTotal.WithLabelValues(txnType, status).Inc()
	if err == nil {
		m.transactionAmount.WithLabelValues(txnType).Observe(amount)
	}
}

// IncAccountsCreated increments accounts created counter
func (m *Metrics) IncAccountsCreated() {
	m.accountsCreated.Inc()
}

// IncAccountsDeleted increments accounts deleted counter
func (m *Metrics) IncAccountsDeleted() {
	m.accountsDeleted.Inc()
}

// IncUsersRegistered increments users registered counter
func (m *Metrics) IncUsersRegistered() {
	m.usersRegistered.Inc()
}

// RecordAuthAttempt records an authentication attempt
func (m *Metrics) RecordAuthAttempt(success bool) {
	result := "success"
	if !success {
		result = "failure"
	}
	m.authAttempts.WithLabelValues(result).Inc()
}

// SetActiveUsers sets the active users gauge
func (m *Metrics) SetActiveUsers(count int) {
	m.activeUsers.Set(float64(count))
}

// SetAccountBalance sets the account balance for a type
func (m *Metrics) SetAccountBalance(accountType string, balance float64) {
	m.accountBalance.WithLabelValues(accountType).Set(balance)
}

// Event metric recording methods

// RecordEventPublished records an event publication
func (m *Metrics) RecordEventPublished(eventType string, err error) {
	status := "success"
	if err != nil {
		status = "error"
	}
	m.eventsPublished.WithLabelValues(eventType, status).Inc()
}

// RecordEventProcessed records an event processing
func (m *Metrics) RecordEventProcessed(eventType, handler string, duration time.Duration, err error) {
	status := "success"
	if err != nil {
		status = "error"
	}
	m.eventsProcessed.WithLabelValues(eventType, handler, status).Inc()
	m.eventProcessingTime.WithLabelValues(eventType, handler).Observe(duration.Seconds())
}

// RecordEventRetry records an event retry
func (m *Metrics) RecordEventRetry(eventType, handler string) {
	m.eventRetries.WithLabelValues(eventType, handler).Inc()
}

// Rate limiting metric recording methods

// RecordRateLimitHit records a rate limit hit
func (m *Metrics) RecordRateLimitHit(endpoint, userType string) {
	m.rateLimitHits.WithLabelValues(endpoint, userType).Inc()
}

// SetRateLimitTokens sets the available rate limit tokens
func (m *Metrics) SetRateLimitTokens(endpoint string, tokens float64) {
	m.rateLimitTokens.WithLabelValues(endpoint).Set(tokens)
}

// Circuit breaker metric recording methods

// SetCircuitBreakerState sets the circuit breaker state
func (m *Metrics) SetCircuitBreakerState(name string, state int) {
	m.circuitBreakerState.WithLabelValues(name).Set(float64(state))
}

// RecordCircuitBreakerTrip records a circuit breaker trip
func (m *Metrics) RecordCircuitBreakerTrip(name string) {
	m.circuitBreakerTrips.WithLabelValues(name).Inc()
}

// Cache metric recording methods

// RecordCacheHit records a cache hit
func (m *Metrics) RecordCacheHit(cacheName string) {
	m.cacheHits.WithLabelValues(cacheName).Inc()
}

// RecordCacheMiss records a cache miss
func (m *Metrics) RecordCacheMiss(cacheName string) {
	m.cacheMisses.WithLabelValues(cacheName).Inc()
}

// Timer is a helper for timing operations
type Timer struct {
	start time.Time
}

// NewTimer creates a new timer
func NewTimer() *Timer {
	return &Timer{start: time.Now()}
}

// Elapsed returns the elapsed time
func (t *Timer) Elapsed() time.Duration {
	return time.Since(t.start)
}

// contextKey for metrics in context
type contextKey struct{}

// WithContext returns a new context with metrics attached
func WithContext(ctx context.Context, m *Metrics) context.Context {
	return context.WithValue(ctx, contextKey{}, m)
}

// FromContext extracts metrics from context
func FromContext(ctx context.Context) *Metrics {
	if m, ok := ctx.Value(contextKey{}).(*Metrics); ok {
		return m
	}
	return Default()
}
