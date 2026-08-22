// Package session manages tmux sessions inside a project's container:
// list, create, existence checks, and interactive attach. It only knows
// plain-shell tmux; Phase 8 layers harness launching on top.
package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"sps/internal/docker"
)

// ErrInvalidName means a session name outside the allowed shape.
var ErrInvalidName = errors.New("invalid session name")

// nameRe keeps names boring so they are safe as exec args, URL path
// segments, and event payloads.
var nameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

// ValidName reports whether name may be used as a tmux session name.
func ValidName(name string) bool { return nameRe.MatchString(name) }

// Entry is one tmux session.
type Entry struct {
	Name string `json:"name"`
}

// Service runs tmux in a project container via Docker exec.
type Service struct {
	dkr docker.Client
}

// New wires the service to a Docker client.
func New(dkr docker.Client) *Service { return &Service{dkr: dkr} }

// List returns the sessions from `tmux list-sessions`. No server running
// (a fresh container) is an empty list, not an error.
func (s *Service) List(ctx context.Context, container string) ([]Entry, error) {
	res, err := s.dkr.Exec(ctx, container, []string{"tmux", "list-sessions", "-F", "#{session_name}"}, false)
	if err != nil {
		return nil, err
	}
	if res.ExitCode != 0 && strings.Contains(res.Output, "no server running") {
		return []Entry{}, nil
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("list sessions: %s", strings.TrimSpace(res.Output))
	}
	var out []Entry
	for _, line := range strings.Split(strings.TrimSpace(res.Output), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, Entry{Name: line})
		}
	}
	return out, nil
}

// Exists reports whether the named session exists (`tmux has-session`,
// exit 1 = no).
func (s *Service) Exists(ctx context.Context, container, name string) (bool, error) {
	res, err := s.dkr.Exec(ctx, container, []string{"tmux", "has-session", "-t", name}, false)
	if err != nil {
		return false, err
	}
	switch res.ExitCode {
	case 0:
		return true, nil
	case 1:
		return false, nil
	default:
		return false, fmt.Errorf("has-session %s: %s", name, strings.TrimSpace(res.Output))
	}
}

// Create starts a detached session running a plain shell in /workspace.
// A duplicate name fails with the engine's message surfaced.
func (s *Service) Create(ctx context.Context, container, name string) error {
	if !ValidName(name) {
		return fmt.Errorf("%w: %q", ErrInvalidName, name)
	}
	res, err := s.dkr.Exec(ctx, container, []string{"tmux", "new-session", "-d", "-s", name, "-c", "/workspace"}, false)
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("create session %s: %s", name, strings.TrimSpace(res.Output))
	}
	return nil
}

// Attach runs `tmux attach -t name` with a TTY wired through stdin/stdout/
// stderr. It returns immediately with the exec id (needed for Resize); the
// outcome arrives on done when the client detaches or the session ends.
func (s *Service) Attach(ctx context.Context, container, name string, stdin io.Reader, stdout, stderr io.Writer) (execID string, done <-chan docker.ExecDone, err error) {
	return s.dkr.Attach(ctx, container, []string{"tmux", "attach", "-t", name}, stdin, stdout, stderr, true)
}

// Resize resizes the TTY of a running attach.
func (s *Service) Resize(ctx context.Context, execID string, rows, cols int) error {
	return s.dkr.ResizeTTY(ctx, execID, rows, cols)
}
