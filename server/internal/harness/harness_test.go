package harness

import (
	"os"
	"path/filepath"
	"testing"
)

func writePlugin(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestListLoadsPlugins(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "a.json", `{"name":"A","command":"a"}`)
	writePlugin(t, dir, "b.json", `{"name":"B","command":"b","install":"npm i -g b","auth":{"env":["K1","K2"],"deviceFlow":true}}`)
	writePlugin(t, dir, "notes.txt", "not a plugin")

	got, err := New(dir).List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d plugins, want 2: %+v", len(got), got)
	}
	if got[0].Name != "A" || got[1].Command != "b" {
		t.Fatalf("unexpected plugins: %+v", got)
	}
	b := got[1]
	if len(b.Auth.Env) != 2 || b.Auth.Env[0] != "K1" || !b.Auth.DeviceFlow {
		t.Fatalf("auth not parsed: %+v", b.Auth)
	}
}

func TestListRejectsBadSchemas(t *testing.T) {
	cases := []struct{ name, content string }{
		{"missing-name.json", `{"command":"x"}`},
		{"missing-command.json", `{"name":"X"}`},
		{"invalid-json.json", `not json`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writePlugin(t, dir, tc.name, tc.content)
			if _, err := New(dir).List(); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

func TestSeedBuiltinsIdempotent(t *testing.T) {
	dir := t.TempDir()
	first, err := SeedBuiltins(dir)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if len(first) != 4 {
		t.Fatalf("seeded %d, want 4: %v", len(first), first)
	}

	// second seed writes nothing and leaves files untouched
	second, err := SeedBuiltins(dir)
	if err != nil {
		t.Fatalf("second seed: %v", err)
	}
	if len(second) != 0 {
		t.Fatalf("second seed wrote %v, want none", second)
	}

	// seeded files load through the same loader as user plugins
	got, err := New(dir).List()
	if err != nil {
		t.Fatalf("list seeded: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("loaded %d, want 4: %+v", len(got), got)
	}
}

func TestSeedDoesNotOverwriteUserEdits(t *testing.T) {
	dir := t.TempDir()
	if _, err := SeedBuiltins(dir); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "terminal.json")
	edited := `{"name":"Terminal","command":"zsh"}`
	if err := os.WriteFile(path, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := SeedBuiltins(dir); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != edited {
		t.Fatalf("seed overwrote user edit: %s", raw)
	}
}
