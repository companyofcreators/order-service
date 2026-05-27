package main

import (
	"context"
	"crypto/rsa"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/companyofcreators/order-service/internal/app"
	"github.com/companyofcreators/order-service/internal/config"
	httpHandler "github.com/companyofcreators/order-service/internal/interfaces/http"
	wshandler "github.com/companyofcreators/order-service/internal/interfaces/ws"
	"github.com/companyofcreators/order-service/internal/pkg"
	"github.com/companyofcreators/order-service/pkg/header_auth"
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

	// Load JWT public key for WebSocket authentication
	jwtPublicKey, err := loadJWTPublicKey(cfg.JWTPublicKeyPath)
	if err != nil {
		pkg.Logger.ErrorContext(context.Background(), "failed to load JWT public key", "error", err.Error())
		os.Exit(1)
	}

	// Create WebSocket handler
	wsHandler := wshandler.NewHandler(container.WSHub, jwtPublicKey, pkg.Logger, cfg.WSAllowedOrigin)

	// Setup router
	headerSigner := header_auth.NewHeaderSigner(cfg.HeaderHMACKey)
	roleChecker := app.NewUserClient(cfg.UserServiceURL, headerSigner, pkg.Logger)
	router := httpHandler.NewRouter(container.OrderService, headerSigner, wsHandler.Upgrade, roleChecker)

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
		pkg.Logger.Info("http server listening",
			slog.String("addr", srv.Addr),
			slog.String("ws_endpoint", "ws://"+cfg.HTTPAddress+"/ws"),
		)
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

func loadJWTPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return jwt.ParseRSAPublicKeyFromPEM(data)
}
