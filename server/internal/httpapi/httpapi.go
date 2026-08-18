// Package httpapi wires the server's HTTP surface. Handlers live here, not
// in cmd/server, so main stays a thin bootstrap and the growing route set
// (auth in Phase 4, projects in Phase 6, ...) doesn't bloat it.
package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"sps/internal/events"
)

// New returns the HTTP handler for the whole server. ev is the event log
// reader; version is the build-time version string injected by main.
func New(ev events.Reader, version string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": version})
	})
	mux.HandleFunc("GET /api/ping", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"pong": "true", "version": version})
	})
	mux.HandleFunc("GET /api/events", handleEvents(ev))
	return mux
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
