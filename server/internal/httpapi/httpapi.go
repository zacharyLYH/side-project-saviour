// Package httpapi wires the server's HTTP surface. Handlers live here, not
// in cmd/server, so main stays a thin bootstrap and the growing route set
// doesn't bloat it.
package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"sps/internal/auth"
	"sps/internal/docker"
	"sps/internal/events"
	"sps/internal/project"
	"sps/internal/session"
)

// EventLog is what handlers need from the event log: read history and append
// new lines. *events.Log satisfies it; tests use a real temp log.
type EventLog interface {
	events.Reader
	Append(typ string, data map[string]any) (events.Event, error)
}

// Deps carries everything New needs. Grows over time instead of
// stretching New's signature.
type Deps struct {
	Events   EventLog
	Version  string
	Auth     *auth.Service
	Projects *project.Service
	Sessions *session.Service
}

// New returns the HTTP handler for the whole server. Login/PIN routes are
// public; everything else under /api requires a valid session cookie.
func New(d Deps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": d.Version})
	})

	mux.HandleFunc("POST /api/auth/request-pin", handleRequestPIN(d))
	mux.HandleFunc("POST /api/auth/verify", handleVerify(d))
	mux.HandleFunc("POST /api/auth/logout", handleLogout(d))
	mux.Handle("GET /api/auth/me", d.Auth.RequireAuth(http.HandlerFunc(handleMe(d))))
	mux.Handle("GET /api/events", d.Auth.RequireAuth(http.HandlerFunc(handleEvents(d.Events))))

	if d.Projects != nil {
		mux.Handle("GET /api/projects", d.Auth.RequireAuth(http.HandlerFunc(handleListProjects(d))))
		mux.Handle("POST /api/projects", d.Auth.RequireAuth(http.HandlerFunc(handleCreateProject(d))))
		mux.Handle("GET /api/projects/{id}", d.Auth.RequireAuth(http.HandlerFunc(handleGetProject(d))))
		mux.Handle("DELETE /api/projects/{id}", d.Auth.RequireAuth(http.HandlerFunc(handleDeleteProject(d))))
		for _, op := range []string{"start", "stop", "restart"} {
			mux.Handle("POST /api/projects/{id}/"+op, d.Auth.RequireAuth(http.HandlerFunc(handleProjectOp(d, op))))
		}
	}

	if d.Sessions != nil {
		mux.Handle("GET /api/projects/{id}/sessions", d.Auth.RequireAuth(http.HandlerFunc(handleListSessions(d))))
		mux.Handle("POST /api/projects/{id}/sessions", d.Auth.RequireAuth(http.HandlerFunc(handleCreateSession(d))))
		mux.Handle("GET /ws/projects/{id}/sessions/{name}", d.Auth.RequireAuth(http.HandlerFunc(handleTerminal(d))))
	}
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

// handleCreateProject runs the create pipeline synchronously: sandbox up,
// repo cloned (when given), ready.
func handleCreateProject(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			RepoURL string `json:"repoUrl"`
			Branch  string `json:"branch"`
		}
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
				return
			}
		}
		id, p, err := d.Projects.Create(r.Context(), strings.TrimSpace(body.RepoURL), strings.TrimSpace(body.Branch))
		switch {
		case errors.Is(err, project.ErrInvalidInput):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		case err != nil:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		default:
			writeJSON(w, http.StatusCreated, map[string]any{
				"id": id, "name": p.Name, "repo": p.Repo, "branch": p.Branch,
			})
		}
	}
}

func handleListProjects(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries, err := d.Projects.List()
		if err != nil {
			slog.Error("list projects", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		if entries == nil {
			entries = []project.Entry{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"projects": entries})
	}
}

func handleGetProject(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, status, err := d.Projects.Get(r.Context(), r.PathValue("id"))
		switch {
		case errors.Is(err, project.ErrNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such project"})
		case err != nil:
			slog.Error("get project", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		default:
			writeJSON(w, http.StatusOK, map[string]any{
				"id": r.PathValue("id"), "name": p.Name, "repo": p.Repo,
				"branch": p.Branch, "status": status.State,
			})
		}
	}
}

// handleProjectOp serves POST /{id}/start|stop|restart.
func handleProjectOp(d Deps, op string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		ctx := r.Context()
		var err error
		switch op {
		case "start":
			err = d.Projects.Start(ctx, id)
		case "stop":
			err = d.Projects.Stop(ctx, id)
		case "restart":
			err = d.Projects.Restart(ctx, id)
		}
		writeServiceErr(w, err)
		if err == nil {
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		}
	}
}

func handleDeleteProject(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scope := project.Scope(r.URL.Query().Get("scope"))
		if scope == "" {
			scope = project.ScopeAll
		}
		err := d.Projects.Delete(r.Context(), r.PathValue("id"), scope)
		switch {
		case err == nil:
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		case errors.Is(err, project.ErrInvalidScope):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		default:
			writeServiceErr(w, err)
		}
	}
}

// writeServiceErr maps service errors to statuses; on a mapped error it
// writes the response, otherwise it leaves it to the caller.
func writeServiceErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, project.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such project"})
	case errors.Is(err, docker.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "container not found"})
	case err != nil:
		slog.Error("project op", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
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
