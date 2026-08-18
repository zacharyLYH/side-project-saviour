package data

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBootstrapCreatesTree(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	if err := Bootstrap(root); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	for _, sub := range []string{"", "projects", "harnesses", "ssh"} {
		p := root
		if sub != "" {
			p = filepath.Join(root, sub)
		}
		info, err := os.Stat(p)
		if err != nil {
			t.Fatalf("stat %s: %v", p, err)
		}
		if !info.IsDir() {
			t.Fatalf("%s is not a dir", p)
		}
		if info.Mode().Perm() != 0o700 {
			t.Fatalf("%s perms = %o, want 700", p, info.Mode().Perm())
		}
	}
}

func TestBootstrapIdempotent(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	if err := Bootstrap(root); err != nil {
		t.Fatal(err)
	}
	if err := Bootstrap(root); err != nil {
		t.Fatalf("second bootstrap: %v", err)
	}
}
