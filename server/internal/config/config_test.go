package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := load([]string{"SPS_LOGIN_EMAIL=me@example.com"})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.DataDir != "./data" || cfg.Bind != ":8080" || cfg.DockerSock != "unix:///var/run/docker.sock" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.SMTPHost != "smtp.gmail.com" || cfg.SMTPPort != 587 {
		t.Fatalf("unexpected SMTP defaults: %+v", cfg)
	}
}

func TestLoadAllFields(t *testing.T) {
	cfg, err := load([]string{
		"SPS_DATA_DIR=/srv/sps",
		"SPS_BIND=127.0.0.1:9000",
		"SPS_LOGIN_EMAIL=me@example.com",
		"SPS_JWT_SECRET=" + strings.Repeat("x", 32),
		"SMTP_HOST=smtp.gmail.com",
		"SMTP_PORT=587",
		"SMTP_USER=me@gmail.com",
		"SMTP_PASSWORD=app-password",
		"SMTP_FROM=me@gmail.com",
		"SPS_DOCKER_SOCK=tcp://127.0.0.1:2375",
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	want := Config{DataDir: "/srv/sps", Bind: "127.0.0.1:9000", LoginEmail: "me@example.com",
		JWTSecret: strings.Repeat("x", 32), SMTPHost: "smtp.gmail.com", SMTPPort: 587,
		SMTPUser: "me@gmail.com", SMTPPass: "app-password", SMTPFrom: "me@gmail.com",
		DockerSock: "tcp://127.0.0.1:2375"}
	if *cfg != want {
		t.Fatalf("got %+v, want %+v", *cfg, want)
	}
}

func TestLoadRejectsMissingLoginEmail(t *testing.T) {
	_, err := load(nil)
	if err == nil || !strings.Contains(err.Error(), "SPS_LOGIN_EMAIL") {
		t.Fatalf("expected missing-email error, got %v", err)
	}
}

func TestLoadRejectsUnknownVariable(t *testing.T) {
	for _, unknown := range []string{"SPS_BOGUS_SETTING=1", "SMTP_BOGUS=1"} {
		if _, err := load([]string{unknown}); err == nil || !strings.Contains(err.Error(), strings.SplitN(unknown, "=", 2)[0]) {
			t.Fatalf("expected unknown-variable error for %q, got %v", unknown, err)
		}
	}
}

func TestAppendDotEnv(t *testing.T) {
	dir := t.TempDir()
	dotenv := filepath.Join(dir, ".env")
	if err := os.WriteFile(dotenv, []byte(`
# comment line
SPS_LOGIN_EMAIL=from-dotenv@example.com
SPS_BIND = "127.0.0.1:9999"
SMTP_USER=user@example.com
=no-name
KEY_NO_EQ
`), 0o600); err != nil {
		t.Fatal(err)
	}

	env := []string{"SPS_BIND=:8080", "SPS_LOGIN_EMAIL=real@example.com"}
	if err := appendDotEnv(&env, dotenv); err != nil {
		t.Fatalf("appendDotEnv: %v", err)
	}

	got := map[string]string{}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		got[k] = v
	}
	// Real environment wins.
	if got["SPS_LOGIN_EMAIL"] != "real@example.com" {
		t.Fatalf("real env lost: %v", got)
	}
	if got["SPS_BIND"] != ":8080" {
		t.Fatalf("real env lost for bind: %v", got)
	}
	// Dotenv fills gaps and trims quotes/whitespace.
	if got["SMTP_USER"] != "user@example.com" {
		t.Fatalf("dotenv value missing: %v", got)
	}
	if got["SPS_BIND"] != ":8080" {
		t.Fatalf("bind overwritten: %v", got)
	}
}

func TestAppendDotEnvMissingFile(t *testing.T) {
	env := []string{"SPS_LOGIN_EMAIL=me@example.com"}
	if err := appendDotEnv(&env, filepath.Join(t.TempDir(), "nope.env")); err != nil {
		t.Fatalf("missing file should be ignored, got %v", err)
	}
	if len(env) != 1 {
		t.Fatalf("env mutated: %v", env)
	}
}

func TestValidateRejectsBadValues(t *testing.T) {
	email := "SPS_LOGIN_EMAIL=me@example.com"
	cases := []struct {
		name string
		env  []string
	}{
		{"bad bind", []string{email, "SPS_BIND=8080"}},
		{"bad email", []string{"SPS_LOGIN_EMAIL=not-an-email"}},
		{"short jwt secret", []string{email, "SPS_JWT_SECRET=short"}},
		{"bad smtp port", []string{email, "SMTP_PORT=abc"}},
		{"out of range smtp port", []string{email, "SMTP_PORT=70000"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := load(tc.env); err == nil {
				t.Fatalf("expected error for %v", tc.env)
			}
		})
	}
}
