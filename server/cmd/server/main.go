// Command server is the Side Project Saviour control plane: it serves the
// HTTP + WebSocket API and talks to Docker, tmux, and git on the host.
// This file only boots: config, data dir, event log, harness seeding. The
// HTTP surface lives in internal/httpapi.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"sps/internal/config"
	"sps/internal/data"
	"sps/internal/events"
	"sps/internal/harness"
	"sps/internal/httpapi"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "err", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(logger)

	if err := data.Bootstrap(cfg.DataDir); err != nil {
		slog.Error("bootstrap data dir", "err", err)
		os.Exit(1)
	}

	ev, err := events.Open(filepath.Join(cfg.DataDir, "events.log"))
	if err != nil {
		slog.Error("open event log", "err", err)
		os.Exit(1)
	}
	defer ev.Close()

	seeded, err := harness.SeedBuiltins(filepath.Join(cfg.DataDir, "harnesses"))
	if err != nil {
		slog.Error("seed harnesses", "err", err)
		os.Exit(1)
	}

	ev.Append("boot", map[string]any{"version": version})
	if len(seeded) > 0 {
		ev.Append("harness.seed", map[string]any{"written": seeded})
	}
	logger.Info("data dir ready", "data_dir", cfg.DataDir, "seeded_harnesses", seeded)

	srv := &http.Server{Addr: cfg.Bind, Handler: httpapi.New(ev, version)}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server listening", "addr", cfg.Bind, "data_dir", cfg.DataDir)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "err", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		logger.Info("signal received, shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "err", err)
			os.Exit(1)
		}
		logger.Info("shutdown complete")
	}
}
