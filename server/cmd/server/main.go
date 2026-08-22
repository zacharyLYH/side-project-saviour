// Command server is the Side Project Saviour control plane: it serves the
// HTTP API and talks to Docker on the host. (tmux/git session management is
// not built yet.) This file only boots: config, data dir, event log,
// harness seeding. The HTTP surface lives in internal/httpapi.
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

	"sps/internal/auth"
	"sps/internal/config"
	"sps/internal/data"
	"sps/internal/docker"
	"sps/internal/events"
	"sps/internal/harness"
	"sps/internal/httpapi"
	"sps/internal/project"
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

	authSvc, err := newAuthService(cfg)
	if err != nil {
		slog.Error("init auth", "err", err)
		os.Exit(1)
	}

	dkr, err := docker.New(cfg.DockerSock)
	if err != nil {
		slog.Error("init docker client", "err", err)
		os.Exit(1)
	}
	// Docker being down must not take auth or the event log with it: warn
	// now, fail per-operation when a project pipeline actually needs it.
	if err := dkr.Ping(context.Background()); err != nil {
		slog.Warn("docker engine unreachable", "err", err)
	}

	svc := project.NewService(project.Open(cfg.DataDir), dkr, ev)

	ev.Append("boot", map[string]any{"version": version})
	if len(seeded) > 0 {
		ev.Append("harness.seed", map[string]any{"written": seeded})
	}
	logger.Info("data dir ready", "data_dir", cfg.DataDir, "seeded_harnesses", seeded)

	srv := &http.Server{Addr: cfg.Bind, Handler: httpapi.New(httpapi.Deps{
		Events: ev, Version: version, Auth: authSvc, Projects: svc,
	})}

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

// newAuthService builds the auth service: signing secret from SPS_JWT_SECRET
// or a generated, persisted file under the data dir; PIN delivery by SMTP
// when credentials are configured, console (server log) otherwise.
func newAuthService(cfg *config.Config) (*auth.Service, error) {
	secret := []byte(cfg.JWTSecret)
	if len(secret) == 0 {
		var err error
		secret, err = auth.LoadOrCreateSecret(filepath.Join(cfg.DataDir, "jwt-secret"))
		if err != nil {
			return nil, err
		}
	}
	var mailer auth.Mailer = auth.ConsoleMailer{Out: os.Stderr}
	name := "console"
	if cfg.SMTPUser != "" && cfg.SMTPPass != "" {
		mailer = auth.SmtpMailer{Host: cfg.SMTPHost, Port: cfg.SMTPPort, User: cfg.SMTPUser, Password: cfg.SMTPPass, From: cfg.SMTPFrom}
		name = "smtp"
	}
	svc := auth.New(cfg.LoginEmail, secret, mailer)
	svc.MailerName = name
	slog.Info("auth ready", "login_email", cfg.LoginEmail, "mailer", name)
	return svc, nil
}
