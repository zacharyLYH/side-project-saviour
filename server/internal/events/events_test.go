package events

import (
	"path/filepath"
	"testing"
)

func TestAppendReadPagination(t *testing.T) {
	l, err := Open(filepath.Join(t.TempDir(), "events.log"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer l.Close()

	for i := 0; i < 5; i++ {
		if _, err := l.Append("test", map[string]any{"n": i}); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	all, err := l.Read(0, 0)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(all) != 5 || all[0].ID != 1 || all[4].ID != 5 {
		t.Fatalf("unexpected events: %+v", all)
	}

	page, err := l.Read(2, 2)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(page) != 2 || page[0].ID != 3 || page[1].ID != 4 {
		t.Fatalf("unexpected page: %+v", page)
	}

	tail, err := l.Read(4, 100)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(tail) != 1 || tail[0].ID != 5 {
		t.Fatalf("unexpected tail: %+v", tail)
	}
}

func TestOpenContinuesIDs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.log")

	l, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := l.Append("a", nil); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}

	l2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer l2.Close()
	ev, err := l2.Append("b", nil)
	if err != nil {
		t.Fatal(err)
	}
	if ev.ID != 2 {
		t.Fatalf("id = %d, want 2 (monotonic across restarts)", ev.ID)
	}
}
