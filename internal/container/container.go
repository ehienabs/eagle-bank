package container

import (
	"go.uber.org/dig"

	"github.com/ehienabs/eagle-bank/internal/api"
	"github.com/ehienabs/eagle-bank/internal/api/handler"
	"github.com/ehienabs/eagle-bank/internal/config"
	"github.com/ehienabs/eagle-bank/internal/kafka"
	"github.com/ehienabs/eagle-bank/internal/repository"
	"github.com/ehienabs/eagle-bank/internal/service"
	"github.com/ehienabs/eagle-bank/pkg/logger"
	"github.com/ehienabs/eagle-bank/pkg/metrics"
)

// Build constructs the DI container from the pre-loaded config.
// logger.Init and metrics.Init must be called before Build so that
// Default() singletons are available.
func Build(cfg *config.Config) (*dig.Container, error) {
	c := dig.New()

	providers := []interface{}{
		// Supply the pre-loaded config so it is only loaded once.
		func() *config.Config { return cfg },

		// Sub-config extractors.
		func(c *config.Config) config.DatabaseConfig { return c.Database },
		func(c *config.Config) config.KafkaConfig { return c.Kafka },
		func(c *config.Config) config.AuthConfig { return c.Auth },

		// Singleton accessors — Init must be called in main before Build.
		func() *logger.Logger { return logger.Default() },
		func() *metrics.Metrics { return metrics.Default() },

		// Infrastructure.
		repository.NewDB,
		provideKafkaProducer,

		// Repositories.
		repository.NewUserRepository,
		repository.NewAccountRepository,
		repository.NewTransactionRepository,
		repository.NewEventRepository,

		// Services.
		service.NewAuthService,
		service.NewUserService,
		service.NewAccountService,
		service.NewTransactionService,

		// Handlers.
		handler.NewUserHandler,
		handler.NewAccountHandler,
		handler.NewTransactionHandler,

		// Router.
		provideRouterConfig,
		api.NewRouter,
	}

	for _, p := range providers {
		if err := c.Provide(p); err != nil {
			return nil, err
		}
	}

	return c, nil
}

// provideKafkaProducer wraps kafka.NewProducer to drop the variadic options
// parameter, which dig does not support on constructor functions.
func provideKafkaProducer(cfg config.KafkaConfig) (*kafka.Producer, error) {
	return kafka.NewProducer(cfg)
}

// provideRouterConfig assembles the RouterConfig struct that api.NewRouter expects.
func provideRouterConfig(
	cfg *config.Config,
	userHandler *handler.UserHandler,
	accountHandler *handler.AccountHandler,
	txnHandler *handler.TransactionHandler,
	authService *service.AuthService,
	m *metrics.Metrics,
	log *logger.Logger,
) *api.RouterConfig {
	return &api.RouterConfig{
		Config:         cfg,
		UserHandler:    userHandler,
		AccountHandler: accountHandler,
		TxnHandler:     txnHandler,
		AuthService:    authService,
		Metrics:        m,
		Logger:         log,
	}
}
