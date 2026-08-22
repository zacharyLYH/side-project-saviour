// Package project stores one JSON file per project under
// $DATA_DIR/projects/<id>/project.json. The directory listing is the index;
// there is no separate summary file to keep in sync. Writes are atomic
// (temp + rename) with owner-only permissions.
package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Project holds only what cannot be defaulted.
type Project struct {
	Name    string            `json:"name"`
	Repo    string            `json:"repo"`
	Branch  string            `json:"branch,omitempty"`
	Actions []Action          `json:"actions,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// Action is a button: {label, command} run in a session.
type Action struct {
	Label   string `json:"label"`
	Command string `json:"command"`
}

// Entry is one row of the projects index.
type Entry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Store is what consumers need: CRUD over projects plus the index. Defined
// as an interface so callers can be tested with a mock instead of real disk.
type Store interface {
	Create(id string, p Project) error
	Get(id string) (Project, error)
	Update(id string, p Project) error
	Delete(id string) error
	List() ([]Entry, error)
}

// FileStore reads and writes projects under a data dir: one JSON file per
// project. The projects directory listing is the index; there is no separate
// summary state to keep consistent. Writes are atomic (temp + rename) with
// owner-only permissions.
type FileStore struct {
	dataDir string
}

// Open returns a FileStore rooted at $DATA_DIR. The projects directory must
// already exist (data.Bootstrap).
func Open(dataDir string) *FileStore {
	return &FileStore{dataDir: dataDir}
}

// Create writes a new project file.
func (s *FileStore) Create(id string, p Project) error {
	dir := filepath.Join(s.dataDir, "projects", id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create project dir: %w", err)
	}
	if err := writeJSON(filepath.Join(dir, "project.json"), p); err != nil {
		return fmt.Errorf("write project: %w", err)
	}
	return nil
}

// Get reads one project.
func (s *FileStore) Get(id string) (Project, error) {
	var p Project
	if err := readJSON(filepath.Join(s.dataDir, "projects", id, "project.json"), &p); err != nil {
		return Project{}, err
	}
	return p, nil
}

// Update replaces a project's file.
func (s *FileStore) Update(id string, p Project) error {
	if err := writeJSON(filepath.Join(s.dataDir, "projects", id, "project.json"), p); err != nil {
		return fmt.Errorf("write project: %w", err)
	}
	return nil
}

// Delete removes a project's directory.
func (s *FileStore) Delete(id string) error {
	if err := os.RemoveAll(filepath.Join(s.dataDir, "projects", id)); err != nil {
		return fmt.Errorf("remove project: %w", err)
	}
	return nil
}

// List scans the projects directory and returns every project as an entry,
// sorted by name. Each project.json is the single source of truth — no
// separate index to fall out of sync. A project dir without a readable
// project.json is skipped (never half-created); anything else fails loudly.
func (s *FileStore) List() ([]Entry, error) {
	entries, err := os.ReadDir(filepath.Join(s.dataDir, "projects"))
	if err != nil {
		if os.IsNotExist(err) {
			return []Entry{}, nil
		}
		return nil, fmt.Errorf("list projects: %w", err)
	}
	var out []Entry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		var p Project
		if err := readJSON(filepath.Join(s.dataDir, "projects", e.Name(), "project.json"), &p); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read project %s: %w", e.Name(), err)
		}
		out = append(out, Entry{ID: e.Name(), Name: p.Name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// writeJSON writes v to path atomically (temp + rename) with owner-only perms.
func writeJSON(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readJSON(path string, v any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}
