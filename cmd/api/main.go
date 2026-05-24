package main

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	"github.com/ehienabs/eagle-bank/internal/api"
	"github.com/ehienabs/eagle-bank/internal/config"
	"github.com/ehienabs/eagle-bank/internal/container"
	"github.com/ehienabs/eagle-bank/internal/kafka"
	"github.com/ehienabs/eagle-bank/internal/repository"
	"github.com/ehienabs/eagle-bank/pkg/logger"
	"github.com/ehienabs/eagle-bank/pkg/metrics"
	"github.com/ehienabs/eagle-bank/pkg/tracing"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize global singletons before building the container so that
	// logger.Default() and metrics.Default() are ready for the providers.
	logger.Init(cfg.Logger)
	log := logger.Default()

	log.Info("starting eagle-bank API",
		"version", cfg.App.Version,
		"environment", cfg.App.Environment,
	)

	ctx := context.Background()

	tracer, err := tracing.Init(ctx, cfg.Tracing)
	if err != nil {
		log.Error("failed to initialize tracing", "error", err)
		os.Exit(1)
	}
	defer tracer.Shutdown(ctx)

	metrics.Init(cfg.Metrics)

	// Build the DI container.
	c, err := container.Build(cfg)
	if err != nil {
		log.Error("failed to build DI container", "error", err)
		os.Exit(1)
	}

	// Resolve the router and infrastructure resources that need lifecycle management.
	if err := c.Invoke(func(router *api.Router, db *repository.DB, producer *kafka.Producer) {
		defer db.Close()
		defer producer.Close()

		log.Info("connected to database",
			"host", cfg.Database.Host,
			"database", cfg.Database.Database,
		)
		log.Info("connected to Kafka",
			"brokers", cfg.Kafka.Brokers,
		)

		if cfg.Server.PProfEnabled {
			go func() {
				pprofAddr := fmt.Sprintf(":%d", cfg.Server.PProfPort)
				log.Info("pprof server starting", "port", cfg.Server.PProfPort)
				if err := http.ListenAndServe(pprofAddr, nil); err != nil {
					log.Error("pprof server error", "error", err)
				}
			}()
		}

		srv := &http.Server{
			Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
			Handler:      router.Engine(),
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
			IdleTimeout:  cfg.Server.IdleTimeout,
		}

		go func() {
			log.Info("HTTP server starting",
				"port", cfg.Server.Port,
				"read_timeout", cfg.Server.ReadTimeout,
				"write_timeout", cfg.Server.WriteTimeout,
			)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error("HTTP server error", "error", err)
				os.Exit(1)
			}
		}()

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit

		log.Info("shutting down server...")

		shutdownCtx, cancel := context.WithTimeout(ctx, cfg.Server.ShutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error("server forced to shutdown", "error", err)
		}

		log.Info("server stopped")
	}); err != nil {
		log.Error("application error", "error", err)
		os.Exit(1)
	}
}
