// Command api runs the ledger HTTP service.
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

	"github.com/1SenaMetth/ledger/internal/config"
	"github.com/1SenaMetth/ledger/internal/httpapi"
	"github.com/1SenaMetth/ledger/internal/storage/postgres"
)

// Injected at build time via -ldflags. See the Makefile.
var (
	version = "dev"
	commit  = "none"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		// The logger is not configured yet, so write plainly.
		return err
	}

	setupLogging(cfg)
	slog.Info("starting", "version", version, "commit", commit, "env", cfg.Env)

	// Cancelled on SIGINT or SIGTERM. Kubernetes sends SIGTERM before it kills
	// a pod, which is what makes graceful shutdown possible.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.NewPool(ctx, postgres.Config{
		URL:      cfg.Database.URL,
		MaxConns: cfg.Database.MaxConns,
		MinConns: cfg.Database.MinConns,
	})
	if err != nil {
		return err
	}
	defer pool.Close()
	slog.Info("database connected", "max_conns", cfg.Database.MaxConns)

	srv := &httpapi.Server{DB: pool, Version: version, Commit: commit}

	httpServer := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("http server listening", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		slog.Info("shutdown signal received", "timeout", cfg.ShutdownTimeout)
	}

	// Stop accepting new connections and give in-flight requests time to finish.
	// Without this, a rolling deploy drops every request that was mid-flight.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed, forcing close", "error", err)
		return httpServer.Close()
	}

	slog.Info("shutdown complete")
	return nil
}

func setupLogging(cfg config.Config) {
	level := slog.LevelDebug
	if cfg.IsProduction() {
		level = slog.LevelInfo
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level}
	if cfg.IsProduction() {
		// JSON in production so log aggregators can parse it.
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler).With("service", "ledger-api"))
}
