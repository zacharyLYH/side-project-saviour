// Package httpapi wires the server's HTTP surface. Handlers live here, not
// in cmd/server, so main stays a thin bootstrap and the growing route set
// (projects in Phase 6, ...) doesn't bloat it.
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"sps/internal/auth"
	"sps/internal/events"
)

// EventLog is what handlers need from the event log: read history and append
// new lines. *events.Log satisfies it; tests use a real temp log.
type EventLog interface {
	events.Reader
	Append(typ string, data map[string]any) (events.Event, error)
}

// Deps carries everything New needs. Grows with each phase (docker in
// Phase 6, project store, ...) instead of stretching New's signature.
type Deps struct {
	Events  EventLog
	Version string
	Auth    *auth.Service
}

// New returns the HTTP handler for the whole server. Login/PIN routes are
// public; everything else under /api requires a valid session cookie.
func New(d Deps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": d.Version})
	})
	mux.HandleFunc("GET /api/ping", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"pong": "true", "version": d.Version})
	})

	mux.HandleFunc("POST /api/auth/request-pin", handleRequestPIN(d))
	mux.HandleFunc("POST /api/auth/verify", handleVerify(d))
	mux.HandleFunc("POST /api/auth/logout", handleLogout(d))
	mux.Handle("GET /api/auth/me", d.Auth.RequireAuth(http.HandlerFunc(handleMe(d))))
	mux.Handle("GET /api/events", d.Auth.RequireAuth(http.HandlerFunc(handleEvents(d.Events))))
	return mux
}

func handleRequestPIN(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			return
		}
		body.Email = strings.TrimSpace(body.Email)
		err := d.Auth.RequestPIN(r.Context(), body.Email)
		switch {
		case errors.Is(err, auth.ErrNotConfiguredEmail):
			// Deliberately identical to success: don't reveal whether an
			// address is configured.
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		case errors.Is(err, auth.ErrRateLimited):
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many requests, try again later"})
		case err != nil:
			slog.Error("request pin", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		default:
			d.Events.Append("login.pin.sent", map[string]any{"email": body.Email, "delivery": d.Auth.MailerName})
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		}
	}
}

func handleVerify(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Email string `json:"email"`
			Pin   string `json:"pin"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			return
		}
		token, err := d.Auth.Verify(strings.TrimSpace(body.Email), strings.TrimSpace(body.Pin))
		switch {
		case errors.Is(err, auth.ErrRateLimited):
			d.Events.Append("login.failure", map[string]any{"email": body.Email, "reason": "rate_limited"})
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many attempts, try again later"})
		case errors.Is(err, auth.ErrInvalidPIN):
			d.Events.Append("login.failure", map[string]any{"email": body.Email, "reason": "invalid_pin"})
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid pin"})
		case err != nil:
			slog.Error("verify pin", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		default:
			d.Events.Append("login.success", map[string]any{"email": body.Email})
			d.Auth.SetCookie(w, r, token)
			writeJSON(w, http.StatusOK, map[string]any{"email": body.Email})
		}
	}
}

func handleLogout(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		d.Auth.ClearCookie(w, r)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

func handleMe(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"email": d.Auth.Email(r)})
	}
}

func handleEvents(ev events.Reader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		after := int64(0)
		if v := r.URL.Query().Get("after"); v != "" {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "after must be a number"})
				return
			}
			after = n
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
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
