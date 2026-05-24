// Package config provides configuration management for the Eagle Bank application.
// It supports environment variables, with Kubernetes-friendly defaults.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ehienabs/eagle-bank/pkg/logger"
	"github.com/ehienabs/eagle-bank/pkg/metrics"
	"github.com/ehienabs/eagle-bank/pkg/tracing"
)

// Config holds all application configuration
type Config struct {
	App      AppConfig
	Server   ServerConfig
	Database DatabaseConfig
	Kafka    KafkaConfig
	Auth     AuthConfig
	Logger   logger.Config
	Metrics  metrics.Config
	Tracing  tracing.Config
	Redis    RedisConfig
}

// AppConfig holds application-level configuration
type AppConfig struct {
	Name        string
	Version     string
	Environment string
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Host            string
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	MetricsPort     int
	HealthPort      int
	PProfPort       int
	PProfEnabled    bool

	// Rate limiting
	RateLimitEnabled  bool
	RateLimitRequests int           // requests per duration
	RateLimitDuration time.Duration // duration window
	RateLimitBurst    int           // burst size

	// CORS
	CORSEnabled        bool
	CORSAllowedOrigins []string
	CORSAllowedMethods []string
	CORSAllowedHeaders []string
}

// DatabaseConfig holds PostgreSQL configuration
type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// DSN returns the PostgreSQL connection string
func (c DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode,
	)
}

// KafkaConfig holds Kafka configuration
type KafkaConfig struct {
	Brokers       []string
	ConsumerGroup string
	Topics        KafkaTopics

	// Producer config
	ProducerBatchSize int
	ProducerLingerMs  int
	ProducerRetries   int
	ProducerAcks      string // "0", "1", "-1" (all)

	// Consumer config
	ConsumerOffsetReset    string // "earliest", "latest"
	ConsumerAutoCommit     bool
	ConsumerMaxPollRecords int
	ConsumerWorkers        int // Number of concurrent consumer workers
}

// KafkaTopics holds Kafka topic names
type KafkaTopics struct {
	UserEvents        string
	AccountEvents     string
	TransactionEvents string
	AuditEvents       string
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	JWTSecret         string
	JWTAccessExpiry   time.Duration
	JWTRefreshExpiry  time.Duration
	JWTIssuer         string
	Argon2Memory      uint32
	Argon2Time        uint32
	Argon2Parallelism uint8
	Argon2SaltLength  uint32
	Argon2KeyLength   uint32
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Host         string
	Port         int
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	MaxRetries   int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// Address returns the Redis address
func (c RedisConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{}

	// App config
	cfg.App = AppConfig{
		Name:        getEnv("APP_NAME", "eagle-bank"),
		Version:     getEnv("APP_VERSION", "1.0.0"),
		Environment: getEnv("APP_ENV", "development"),
	}

	// Server config
	cfg.Server = ServerConfig{
		Host:               getEnv("SERVER_HOST", "0.0.0.0"),
		Port:               getEnvInt("SERVER_PORT", 8080),
		ReadTimeout:        getEnvDuration("SERVER_READ_TIMEOUT", 30*time.Second),
		WriteTimeout:       getEnvDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:        getEnvDuration("SERVER_IDLE_TIMEOUT", 120*time.Second),
		ShutdownTimeout:    getEnvDuration("SERVER_SHUTDOWN_TIMEOUT", 30*time.Second),
		MetricsPort:        getEnvInt("METRICS_PORT", 9090),
		HealthPort:         getEnvInt("HEALTH_PORT", 8081),
		PProfPort:          getEnvInt("PPROF_PORT", 6060),
		PProfEnabled:       getEnvBool("PPROF_ENABLED", false),
		RateLimitEnabled:   getEnvBool("RATE_LIMIT_ENABLED", true),
		RateLimitRequests:  getEnvInt("RATE_LIMIT_REQUESTS", 100),
		RateLimitDuration:  getEnvDuration("RATE_LIMIT_DURATION", time.Minute),
		RateLimitBurst:     getEnvInt("RATE_LIMIT_BURST", 20),
		CORSEnabled:        getEnvBool("CORS_ENABLED", true),
		CORSAllowedOrigins: getEnvSlice("CORS_ALLOWED_ORIGINS", []string{"*"}),
		CORSAllowedMethods: getEnvSlice("CORS_ALLOWED_METHODS", []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}),
		CORSAllowedHeaders: getEnvSlice("CORS_ALLOWED_HEADERS", []string{"Authorization", "Content-Type", "X-Request-ID", "Idempotency-Key"}),
	}

	// Database config
	cfg.Database = DatabaseConfig{
		Host:            getEnv("DB_HOST", "localhost"),
		Port:            getEnvInt("DB_PORT", 5432),
		User:            getEnv("DB_USER", "eagle_bank"),
		Password:        getEnv("DB_PASSWORD", "eagle_bank_password"),
		Database:        getEnv("DB_NAME", "eagle_bank"),
		SSLMode:         getEnv("DB_SSL_MODE", "disable"),
		MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
		MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 10),
		ConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		ConnMaxIdleTime: getEnvDuration("DB_CONN_MAX_IDLE_TIME", 1*time.Minute),
	}

	// Kafka config
	cfg.Kafka = KafkaConfig{
		Brokers:                getEnvSlice("KAFKA_BROKERS", []string{"localhost:9092"}),
		ConsumerGroup:          getEnv("KAFKA_CONSUMER_GROUP", "eagle-bank-api"),
		ProducerBatchSize:      getEnvInt("KAFKA_PRODUCER_BATCH_SIZE", 16384),
		ProducerLingerMs:       getEnvInt("KAFKA_PRODUCER_LINGER_MS", 5),
		ProducerRetries:        getEnvInt("KAFKA_PRODUCER_RETRIES", 3),
		ProducerAcks:           getEnv("KAFKA_PRODUCER_ACKS", "-1"),
		ConsumerOffsetReset:    getEnv("KAFKA_CONSUMER_OFFSET_RESET", "earliest"),
		ConsumerAutoCommit:     getEnvBool("KAFKA_CONSUMER_AUTO_COMMIT", false),
		ConsumerMaxPollRecords: getEnvInt("KAFKA_CONSUMER_MAX_POLL_RECORDS", 500),
		ConsumerWorkers:        getEnvInt("KAFKA_CONSUMER_WORKERS", 5),
		Topics: KafkaTopics{
			UserEvents:        getEnv("KAFKA_TOPIC_USER_EVENTS", "eagle-bank.user.events"),
			AccountEvents:     getEnv("KAFKA_TOPIC_ACCOUNT_EVENTS", "eagle-bank.account.events"),
			TransactionEvents: getEnv("KAFKA_TOPIC_TRANSACTION_EVENTS", "eagle-bank.transaction.events"),
			AuditEvents:       getEnv("KAFKA_TOPIC_AUDIT_EVENTS", "eagle-bank.audit.events"),
		},
	}

	// Auth config
	cfg.Auth = AuthConfig{
		JWTSecret:         getEnv("JWT_SECRET", "change-me-in-production"),
		JWTAccessExpiry:   getEnvDuration("JWT_ACCESS_EXPIRY", 15*time.Minute),
		JWTRefreshExpiry:  getEnvDuration("JWT_REFRESH_EXPIRY", 7*24*time.Hour),
		JWTIssuer:         getEnv("JWT_ISSUER", "eagle-bank"),
		Argon2Memory:      uint32(getEnvInt("ARGON2_MEMORY", 65536)),
		Argon2Time:        uint32(getEnvInt("ARGON2_TIME", 3)),
		Argon2Parallelism: uint8(getEnvInt("ARGON2_PARALLELISM", 4)),
		Argon2SaltLength:  uint32(getEnvInt("ARGON2_SALT_LENGTH", 16)),
		Argon2KeyLength:   uint32(getEnvInt("ARGON2_KEY_LENGTH", 32)),
	}

	// Logger config
	logLevel := logger.LevelInfo
	switch strings.ToLower(getEnv("LOG_LEVEL", "info")) {
	case "debug":
		logLevel = logger.LevelDebug
	case "warn", "warning":
		logLevel = logger.LevelWarn
	case "error":
		logLevel = logger.LevelError
	}

	cfg.Logger = logger.Config{
		Level:       logLevel,
		ServiceName: cfg.App.Name,
		Version:     cfg.App.Version,
		Environment: cfg.App.Environment,
		AddSource:   getEnvBool("LOG_ADD_SOURCE", true),
	}

	// Metrics config
	cfg.Metrics = metrics.Config{
		Namespace:   getEnv("METRICS_NAMESPACE", "eagle_bank"),
		Subsystem:   getEnv("METRICS_SUBSYSTEM", "api"),
		ServiceName: cfg.App.Name,
	}

	// Tracing config
	cfg.Tracing = tracing.Config{
		ServiceName:    cfg.App.Name,
		ServiceVersion: cfg.App.Version,
		Environment:    cfg.App.Environment,
		OTLPEndpoint:   getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
		SampleRate:     getEnvFloat64("OTEL_SAMPLE_RATE", 1.0),
		Enabled:        getEnvBool("OTEL_ENABLED", true),
	}

	// Redis config
	cfg.Redis = RedisConfig{
		Host:         getEnv("REDIS_HOST", "localhost"),
		Port:         getEnvInt("REDIS_PORT", 6379),
		Password:     getEnv("REDIS_PASSWORD", ""),
		DB:           getEnvInt("REDIS_DB", 0),
		PoolSize:     getEnvInt("REDIS_POOL_SIZE", 10),
		MinIdleConns: getEnvInt("REDIS_MIN_IDLE_CONNS", 5),
		MaxRetries:   getEnvInt("REDIS_MAX_RETRIES", 3),
		DialTimeout:  getEnvDuration("REDIS_DIAL_TIMEOUT", 5*time.Second),
		ReadTimeout:  getEnvDuration("REDIS_READ_TIMEOUT", 3*time.Second),
		WriteTimeout: getEnvDuration("REDIS_WRITE_TIMEOUT", 3*time.Second),
	}

	return cfg, nil
}

// MustLoad loads configuration and panics on error
func MustLoad() *Config {
	cfg, err := Load()
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}
	return cfg
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvFloat64(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			return floatVal
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getEnvSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}

// IsDevelopment returns true if running in development environment
func (c *Config) IsDevelopment() bool {
	return c.App.Environment == "development" || c.App.Environment == "dev"
}

// IsProduction returns true if running in production environment
func (c *Config) IsProduction() bool {
	return c.App.Environment == "production" || c.App.Environment == "prod"
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Auth.JWTSecret == "change-me-in-production" && c.IsProduction() {
		return fmt.Errorf("JWT_SECRET must be set in production")
	}
	if c.Database.Password == "eagle_bank_password" && c.IsProduction() {
		return fmt.Errorf("DB_PASSWORD must be set in production")
	}
	return nil
}
