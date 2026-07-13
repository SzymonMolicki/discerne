package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"discerne/backend/internal/config"
	"discerne/backend/internal/database"
	"discerne/backend/internal/quizdb"
	httptransport "discerne/backend/internal/transport/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load(os.Environ())
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	databaseURL := database.URLFromEnvironment()
	if databaseURL == "" {
		logger.Error("missing database url")
		os.Exit(1)
	}

	startupCtx, startupCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer startupCancel()

	pool, err := pgxpool.New(startupCtx, databaseURL)
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(startupCtx); err != nil {
		logger.Error("ping database", "error", err)
		os.Exit(1)
	}

	quizStore := quizdb.NewStore(pool)

	server := &http.Server{
		Addr:              cfg.HTTPAddress,
		Handler:           httptransport.NewRouter(cfg, logger, quizStore),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("api server starting", "address", cfg.HTTPAddress, "timezone", cfg.AppTimezone.String())
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("api server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("api server shutdown failed", "error", err)
		os.Exit(1)
	}

	logger.Info("api server stopped")
}
