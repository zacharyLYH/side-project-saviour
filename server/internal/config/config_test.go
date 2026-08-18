package config

import (
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
