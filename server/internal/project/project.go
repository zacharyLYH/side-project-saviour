// Package project stores one JSON file per project under
// $DATA_DIR/projects/<id>/project.json, plus a projects.json index so the
// list renders without scanning containers. Writes are atomic (temp +
// rename) with owner-only permissions.
package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// Project holds only what cannot be defaulted (PRD §8).
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

type index struct {
	Projects []Entry `json:"projects"`
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
// project, plus the projects.json index. Writes are atomic (temp + rename)
// with owner-only permissions.
type FileStore struct {
	dataDir string
	mu      sync.Mutex // serializes index read-modify-write
}

// Open returns a FileStore rooted at $DATA_DIR. The projects directory must
// already exist (data.Bootstrap).
func Open(dataDir string) *FileStore {
	return &FileStore{dataDir: dataDir}
}

// Create writes a new project and adds it to the index.
func (s *FileStore) Create(id string, p Project) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Join(s.dataDir, "projects", id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create project dir: %w", err)
	}
	if err := writeJSON(filepath.Join(dir, "project.json"), p); err != nil {
		return fmt.Errorf("write project: %w", err)
	}
	return s.indexAdd(id, p.Name)
}

// Get reads one project.
func (s *FileStore) Get(id string) (Project, error) {
	var p Project
	if err := readJSON(filepath.Join(s.dataDir, "projects", id, "project.json"), &p); err != nil {
		return Project{}, err
	}
	return p, nil
}

// Update replaces a project's file, keeping the index's name in sync.
func (s *FileStore) Update(id string, p Project) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := writeJSON(filepath.Join(s.dataDir, "projects", id, "project.json"), p); err != nil {
		return fmt.Errorf("write project: %w", err)
	}
	return s.indexSetName(id, p.Name)
}

// Delete removes a project's directory and index entry.
func (s *FileStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.RemoveAll(filepath.Join(s.dataDir, "projects", id)); err != nil {
		return fmt.Errorf("remove project: %w", err)
	}
	return s.indexRemove(id)
}

// List returns the projects index (id + name), sorted by name.
func (s *FileStore) List() ([]Entry, error) {
	idx, err := s.readIndex()
	if err != nil {
		return nil, err
	}
	out := append([]Entry(nil), idx.Projects...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *FileStore) readIndex() (index, error) {
	var idx index
	if err := readJSON(filepath.Join(s.dataDir, "projects.json"), &idx); err != nil {
		if os.IsNotExist(err) {
			return index{Projects: []Entry{}}, nil
		}
		return index{}, err
	}
	return idx, nil
}

func (s *FileStore) indexAdd(id, name string) error {
	idx, err := s.readIndex()
	if err != nil {
		return err
	}
	idx.Projects = append(idx.Projects, Entry{ID: id, Name: name})
	return s.writeIndex(idx)
}

func (s *FileStore) indexSetName(id, name string) error {
	idx, err := s.readIndex()
	if err != nil {
		return err
	}
	for i := range idx.Projects {
		if idx.Projects[i].ID == id {
			idx.Projects[i].Name = name
		}
	}
	return s.writeIndex(idx)
}

func (s *FileStore) indexRemove(id string) error {
	idx, err := s.readIndex()
	if err != nil {
		return err
	}
	out := idx.Projects[:0]
	for _, e := range idx.Projects {
		if e.ID != id {
			out = append(out, e)
		}
	}
	idx.Projects = out
	return s.writeIndex(idx)
}

func (s *FileStore) writeIndex(idx index) error {
	return writeJSON(filepath.Join(s.dataDir, "projects.json"), idx)
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
