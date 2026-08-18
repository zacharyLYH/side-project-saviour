// Command server is the Side Project Saviour control plane: it serves the
// HTTP + WebSocket API and talks to Docker, tmux, and git on the host.
// Phase 2: bootstraps the data dir, seeds harness builtins, and exposes the
// event log over /api/events.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"sps/internal/config"
	"sps/internal/data"
	"sps/internal/events"
	"sps/internal/harness"
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

	srv := &http.Server{Addr: cfg.Bind, Handler: newHandler(ev)}

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

func newHandler(ev events.Reader) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": version})
	})
	mux.HandleFunc("GET /api/ping", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"pong": "true", "version": version})
	})
	mux.HandleFunc("GET /api/events", func(w http.ResponseWriter, r *http.Request) {
		after := int64(0)
		if v := r.URL.Query().Get("after"); v != "" {
			var err error
			after, err = strconv.ParseInt(v, 10, 64)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "after must be a number"})
				return
			}
		}
		limit := 100
		if v := r.URL.Query().Get("limit"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "limit must be a non-negative number"})
				return
			}
			limit = n
		}
		if limit > 1000 {
			limit = 1000
		}
		list, err := ev.Read(after, limit)
		if err != nil {
			slog.Error("read events", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"events": list})
	})
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
