// Package harness loads harness plugins from $DATA_DIR/harnesses/*.json.
// Builtins are seeded as real editable files, so builtins and user plugins
// share one code path.
package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Harness is a CLI plugin: a global entry in "+ New Session".
type Harness struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	Install string `json:"install,omitempty"`
	Auth    Auth   `json:"auth,omitempty"`
}

// Auth describes how a harness gets its credentials (PRD §7).
type Auth struct {
	Env        []string `json:"env,omitempty"`        // env vars that carry API keys
	DeviceFlow bool     `json:"deviceFlow,omitempty"` // CLI prints a URL + code to log in
	Upload     bool     `json:"upload,omitempty"`     // needs a credential file uploaded
}

// Loader lists the available harness plugins. Defined as an interface so
// consumers can be tested with a mock instead of real plugin files.
type Loader interface {
	List() ([]Harness, error)
}

// DirLoader scans a directory of harness plugin files.
type DirLoader struct {
	dir string
}

// New returns a DirLoader for dir (normally $DATA_DIR/harnesses).
func New(dir string) *DirLoader {
	return &DirLoader{dir: dir}
}

// List returns every plugin in the directory, sorted by name. A plugin that
// fails validation fails the whole list: a broken plugin should be visible,
// not silently dropped.
func (l *DirLoader) List() ([]Harness, error) {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return nil, fmt.Errorf("read harnesses dir: %w", err)
	}
	var out []Harness
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		h, err := loadFile(filepath.Join(l.dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func loadFile(path string) (Harness, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Harness{}, err
	}
	var h Harness
	if err := json.Unmarshal(raw, &h); err != nil {
		return Harness{}, fmt.Errorf("invalid JSON: %w", err)
	}
	if strings.TrimSpace(h.Name) == "" || strings.TrimSpace(h.Command) == "" {
		return Harness{}, fmt.Errorf("name and command are required")
	}
	return h, nil
}

// Builtins seeded into a fresh harnesses dir (PRD §5). Real, editable files;
// install/auth are best-effort defaults the user can change.
var builtins = []Harness{
	{Name: "Terminal", Command: "bash"},
	{Name: "OpenCode", Command: "opencode", Install: "npm i -g opencode-ai", Auth: Auth{Env: []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY"}}},
	{Name: "Freebuff", Command: "freebuff", Install: "npm i -g freebuff", Auth: Auth{Env: []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY"}}},
	{Name: "Aider", Command: "aider", Install: "python3 -m pip install -U aider-chat", Auth: Auth{Env: []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY"}}},
}

// SeedBuiltins writes the builtin plugin files, never overwriting existing
// files (the user may have edited them). Returns the names written.
func SeedBuiltins(dir string) ([]string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create harnesses dir: %w", err)
	}
	var written []string
	for _, b := range builtins {
		path := filepath.Join(dir, pluginFile(b))
		if _, err := os.Stat(path); err == nil {
			continue // already there (possibly user-edited) — leave it alone
		}
		raw, err := json.MarshalIndent(b, "", "  ")
		if err != nil {
			return nil, err
		}
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, append(raw, '\n'), 0o600); err != nil {
			return nil, fmt.Errorf("seed %s: %w", b.Name, err)
		}
		if err := os.Rename(tmp, path); err != nil {
			return nil, fmt.Errorf("seed %s: %w", b.Name, err)
		}
		written = append(written, b.Name)
	}
	return written, nil
}

func pluginFile(h Harness) string {
	return strings.ToLower(h.Name) + ".json"
}
