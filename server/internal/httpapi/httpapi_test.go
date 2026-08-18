package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"sps/internal/events"
	eventsmocks "sps/mocks/events"
)

func newTestLog(t *testing.T) *events.Log {
	t.Helper()
	l, err := events.Open(filepath.Join(t.TempDir(), "events.log"))
	if err != nil {
		t.Fatalf("open event log: %v", err)
	}
	t.Cleanup(func() { l.Close() })
	return l
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestHealth(t *testing.T) {
	rec := get(t, New(newTestLog(t), "dev"), "/health")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status field = %q, want %q", body["status"], "ok")
	}
}

func TestPing(t *testing.T) {
	rec := get(t, New(newTestLog(t), "dev"), "/api/ping")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["pong"] != "true" {
		t.Fatalf("pong field = %q, want %q", body["pong"], "true")
	}
}

func TestMethodNotAllowed(t *testing.T) {
	rec := httptest.NewRecorder()
	New(newTestLog(t), "dev").ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/health", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestEventsAPI(t *testing.T) {
	ev := newTestLog(t)
	for i := 0; i < 5; i++ {
		if _, err := ev.Append("test", map[string]any{"n": i}); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	h := New(ev, "dev")

	decode := func(rec *httptest.ResponseRecorder) []events.Event {
		t.Helper()
		var body struct {
			Events []events.Event `json:"events"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		return body.Events
	}

	all := decode(get(t, h, "/api/events"))
	if len(all) != 5 || all[0].ID != 1 {
		t.Fatalf("unexpected events: %+v", all)
	}

	page := decode(get(t, h, "/api/events?after=2&limit=2"))
	if len(page) != 2 || page[0].ID != 3 || page[1].ID != 4 {
		t.Fatalf("unexpected page: %+v", page)
	}

	bad := get(t, h, "/api/events?after=abc")
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", bad.Code)
	}
}

// TestEventsAPIWithMockReader proves the generated mock substitutes for the
// real event log: the handler gets its events from the mock, not from disk.
func TestEventsAPIWithMockReader(t *testing.T) {
	m := eventsmocks.NewMockReader(t)
	m.EXPECT().Read(int64(3), 100).Return([]events.Event{
		{ID: 4, Type: "test", Time: time.Now().UTC()},
	}, nil)

	rec := get(t, New(m, "dev"), "/api/events?after=3")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body struct {
		Events []events.Event `json:"events"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(body.Events) != 1 || body.Events[0].ID != 4 {
		t.Fatalf("unexpected events: %+v", body.Events)
	}
}
