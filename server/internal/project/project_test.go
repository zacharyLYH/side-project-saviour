package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newStore(t *testing.T) *FileStore {
	t.Helper()
	dataDir := filepath.Join(t.TempDir(), "data")
	if err := os.MkdirAll(filepath.Join(dataDir, "projects"), 0o700); err != nil {
		t.Fatal(err)
	}
	return Open(dataDir)
}

func TestCRUD(t *testing.T) {
	s := newStore(t)
	p := Project{Name: "hello", Repo: "https://github.com/x/hello", Branch: "main",
		Actions: []Action{{Label: "Test", Command: "go test"}}, Env: map[string]string{"FOO": "bar"}}

	if err := s.Create("abc", p); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := s.Get("abc")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "hello" || got.Repo != p.Repo || got.Branch != "main" ||
		len(got.Actions) != 1 || got.Env["FOO"] != "bar" {
		t.Fatalf("round trip mismatch: %+v", got)
	}

	p.Name = "hello2"
	if err := s.Update("abc", p); err != nil {
		t.Fatalf("update: %v", err)
	}
	if got, _ := s.Get("abc"); got.Name != "hello2" {
		t.Fatalf("update not applied: %+v", got)
	}

	entries, err := s.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != "abc" || entries[0].Name != "hello2" {
		t.Fatalf("index wrong: %+v", entries)
	}

	if err := s.Delete("abc"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get("abc"); err == nil {
		t.Fatal("get after delete should fail")
	}
	if entries, _ := s.List(); len(entries) != 0 {
		t.Fatalf("index not updated after delete: %+v", entries)
	}
}

func TestFileModesAndNoTempLeftovers(t *testing.T) {
	s := newStore(t)
	if err := s.Create("abc", Project{Name: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Update("abc", Project{Name: "y"}); err != nil {
		t.Fatal(err)
	}

	for _, p := range []string{
		filepath.Join(s.dataDir, "projects", "abc", "project.json"),
		filepath.Join(s.dataDir, "projects.json"),
	} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatalf("stat %s: %v", p, err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s perms = %o, want 600", p, info.Mode().Perm())
		}
	}

	// no .tmp files left behind after atomic writes
	filepath.WalkDir(s.dataDir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(p, ".tmp") {
			t.Fatalf("leftover temp file: %s", p)
		}
		return nil
	})
}
