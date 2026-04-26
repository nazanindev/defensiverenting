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

	dbpkg "github.com/nazanin212/bostontenantsrights/db"
	"github.com/nazanin212/bostontenantsrights/internal/config"
	apphttp "github.com/nazanin212/bostontenantsrights/internal/http"
	"github.com/nazanin212/bostontenantsrights/internal/store"
)

func main() {
	cfg := config.Load()

	logLevel := slog.LevelInfo
	if cfg.LogLevel == "debug" {
		logLevel = slog.LevelDebug
	}

	var logHandler slog.Handler
	if cfg.IsDevelopment() {
		logHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	} else {
		logHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	}
	logger := slog.New(logHandler)
	slog.SetDefault(logger)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pg, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect to database", slog.Any("err", err))
		os.Exit(1)
	}
	defer pg.Close()

	if err := dbpkg.Migrate(ctx, pg.Pool()); err != nil {
		logger.Error("run migrations", slog.Any("err", err))
		os.Exit(1)
	}
	logger.Info("migrations applied")

	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      apphttp.NewRouter(pg, logger),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		logger.Info("server listening", slog.String("addr", cfg.ListenAddr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", slog.Any("err", err))
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", slog.Any("err", err))
	}
	logger.Info("shutdown complete")
}
