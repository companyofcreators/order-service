package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/companyofcreators/order-service/internal/app"
	"github.com/companyofcreators/order-service/internal/config"
	httpHandler "github.com/companyofcreators/order-service/internal/interfaces/http"
	"github.com/companyofcreators/order-service/internal/pkg"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Initialize logger
	pkg.InitLogger(cfg.LogLevel, cfg.LogFormat)

	pkg.Logger.InfoContext(context.Background(), "starting order service",
		"env", cfg.Env,
		"address", cfg.HTTPAddress,
	)

	// Build container
	container, err := app.NewContainer(cfg)
	if err != nil {
		pkg.Logger.ErrorContext(context.Background(), "failed to initialize container", "error", err.Error())
		os.Exit(1)
	}

	// Setup router
	router := httpHandler.NewRouter(container.OrderService)

	// Create HTTP server
	srv := &http.Server{
		Addr:         cfg.HTTPAddress,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		pkg.Logger.Info("http server listening", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			pkg.Logger.ErrorContext(context.Background(), "server failed", "error", err.Error())
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	pkg.Logger.Info("received shutdown signal")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := srv.Shutdown(ctx); err != nil {
		pkg.Logger.ErrorContext(ctx, "server forced to shutdown", "error", err.Error())
	}

	// Shutdown container (DB, Kafka)
	container.Shutdown(ctx)
}
