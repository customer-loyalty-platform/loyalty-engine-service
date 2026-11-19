package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"loyalty-engine-service/config"
	deliveryHTTP "loyalty-engine-service/internal/delivery/http"
	"loyalty-engine-service/internal/pkg/database"
	"loyalty-engine-service/internal/pkg/jwt"
	"loyalty-engine-service/internal/pkg/logger"

	"go.uber.org/zap"
)

func main() {
	cfg := config.LoadConfig()

	log := logger.NewLogger(cfg.App.Env)
	defer log.Sync()

	log.Info("Initializing loyalty-engine-service...",
		zap.String("env", cfg.App.Env),
		zap.String("port", cfg.App.Port),
	)

	// Context for application lifecycle
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initialize Database Pool
	dbConfig := database.Config{
		MasterDSN:   cfg.Database.MasterDSN,
		ReplicaDSN:  cfg.Database.ReplicaDSN,
		MaxConns:    cfg.Database.MaxConns,
		MinConns:    cfg.Database.MinConns,
		MaxConnIdle: cfg.Database.MaxConnIdle,
		MaxConnLife: cfg.Database.MaxConnLife,
	}

	db, err := database.NewPostgresClient(ctx, dbConfig, log)
	if err != nil {
		log.Fatal("Failed to initialize database client", zap.Error(err))
	}
	defer db.Close()

	// Initialize Token Validator
	tokenValidator := jwt.NewTokenValidator(cfg.JWT.Secret)

	// Build Router & Dependencies
	router := deliveryHTTP.NewRouter(deliveryHTTP.RouterDependencies{
		DB:             db,
		Logger:         log,
		TokenValidator: tokenValidator,
		Env:            cfg.App.Env,
	})

	srv := &http.Server{
		Addr:         ":" + cfg.App.Port,
		Handler:      router,
		ReadTimeout:  cfg.App.ReadTimeout,
		WriteTimeout: cfg.App.WriteTimeout,
		IdleTimeout:  cfg.App.IdleTimeout,
	}

	// Start HTTP Server
	go func() {
		log.Info("loyalty-engine-service HTTP server listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("HTTP server failed to listen", zap.Error(err))
		}
	}()

	// Wait for termination signal
	<-ctx.Done()
	log.Info("Shutdown signal received, gracefully terminating...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("Server forced to shutdown", zap.Error(err))
	}

	log.Info("loyalty-engine-service stopped cleanly")
}

