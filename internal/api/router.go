package api

import (
	"github.com/gin-gonic/gin"
	"github.com/ehienabs/eagle-bank/internal/api/handler"
	"github.com/ehienabs/eagle-bank/internal/api/middleware"
	"github.com/ehienabs/eagle-bank/internal/config"
	"github.com/ehienabs/eagle-bank/internal/service"
	"github.com/ehienabs/eagle-bank/pkg/logger"
	"github.com/ehienabs/eagle-bank/pkg/metrics"
)

// Router holds all route handlers
type Router struct {
	engine         *gin.Engine
	cfg            *config.Config
	userHandler    *handler.UserHandler
	accountHandler *handler.AccountHandler
	txnHandler     *handler.TransactionHandler
	authService    *service.AuthService
	metrics        *metrics.Metrics
	logger         *logger.Logger
}

// RouterConfig holds router configuration
type RouterConfig struct {
	Config         *config.Config
	UserHandler    *handler.UserHandler
	AccountHandler *handler.AccountHandler
	TxnHandler     *handler.TransactionHandler
	AuthService    *service.AuthService
	Metrics        *metrics.Metrics
	Logger         *logger.Logger
}

// NewRouter creates a new router with all routes configured
func NewRouter(cfg *RouterConfig) *Router {
	// Set Gin mode based on environment
	if cfg.Config.App.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()

	log := cfg.Logger
	if log == nil {
		log = logger.Default()
	}

	r := &Router{
		engine:         engine,
		cfg:            cfg.Config,
		userHandler:    cfg.UserHandler,
		accountHandler: cfg.AccountHandler,
		txnHandler:     cfg.TxnHandler,
		authService:    cfg.AuthService,
		metrics:        cfg.Metrics,
		logger:         log,
	}

	r.setupMiddleware()
	r.setupRoutes()

	return r
}

// Engine returns the underlying Gin engine
func (r *Router) Engine() *gin.Engine {
	return r.engine
}

// setupMiddleware configures global middleware
func (r *Router) setupMiddleware() {
	// Recovery middleware (handles panics)
	r.engine.Use(middleware.RecoveryMiddleware(r.logger))

	// Request ID middleware
	r.engine.Use(middleware.RequestIDMiddleware())

	// CORS middleware
	if r.cfg.Server.CORSEnabled {
		r.engine.Use(middleware.CORSMiddleware(
			r.cfg.Server.CORSAllowedOrigins,
			r.cfg.Server.CORSAllowedMethods,
			r.cfg.Server.CORSAllowedHeaders,
		))
	}

	// Logging middleware
	r.engine.Use(middleware.LoggingMiddleware(r.logger))

	// Metrics middleware
	r.engine.Use(middleware.MetricsMiddleware(r.metrics))

	// Tracing middleware
	r.engine.Use(middleware.TracingMiddleware())
}

// setupRoutes configures all API routes
func (r *Router) setupRoutes() {
	// Health check endpoints (no auth required)
	r.engine.GET("/health", handler.HealthCheck)
	r.engine.GET("/ready", handler.ReadinessCheck)
	r.engine.GET("/live", handler.LivenessCheck)

	// Metrics endpoint (no auth required, but should be protected in production)
	r.engine.GET("/metrics", handler.MetricsHandler(r.metrics))

	// API v1 routes
	v1 := r.engine.Group("/v1")

	// Rate limiter for API routes
	if r.cfg.Server.RateLimitEnabled {
		// Convert requests per duration to requests per second
		requestsPerSecond := float64(r.cfg.Server.RateLimitRequests) / r.cfg.Server.RateLimitDuration.Seconds()
		rateLimiter := middleware.NewRateLimiter(requestsPerSecond, r.cfg.Server.RateLimitBurst)
		v1.Use(rateLimiter.Middleware())
	}

	// Public routes (no auth required)
	public := v1.Group("")
	{
		// User registration
		public.POST("/users", r.userHandler.CreateUser)

		// Login
		public.POST("/login", r.userHandler.Login)
	}

	// Protected routes (auth required)
	protected := v1.Group("")
	protected.Use(middleware.AuthMiddleware(r.authService))
	{
		// User routes
		users := protected.Group("/users")
		{
			users.GET("/:userId", r.userHandler.GetUser)
			users.PATCH("/:userId", r.userHandler.UpdateUser)
			users.DELETE("/:userId", r.userHandler.DeleteUser)
		}

		// Account routes
		accounts := protected.Group("/accounts")
		{
			accounts.POST("", r.accountHandler.CreateAccount)
			accounts.GET("", r.accountHandler.ListAccounts)
			accounts.GET("/:accountNumber", r.accountHandler.GetAccount)
			accounts.DELETE("/:accountNumber", r.accountHandler.DeleteAccount)

			// Transaction routes (nested under accounts)
			accounts.POST("/:accountNumber/transactions", r.txnHandler.CreateTransaction)
			accounts.GET("/:accountNumber/transactions", r.txnHandler.ListTransactions)
			accounts.GET("/:accountNumber/transactions/:transactionId", r.txnHandler.GetTransaction)
		}
	}
}
