// Package weathervane reports live container state — for now just per-
// project container status; docker stats, tmux sessions, and port probes
// land here later (see docs/plan.md).
package weathervane

import (
	"context"
	"errors"

	"sps/internal/docker"
)

// State values for a project's container.
const (
	StateMissing = "missing" // no container (never created, or scope=container deleted)
	StateRunning = "running"
)

// Status is the live state of one container.
type Status struct {
	State string
}

// Container inspects name and maps it to a Status. A missing container is
// a valid status, not an error.
func Container(ctx context.Context, d docker.Client, name string) (Status, error) {
	c, err := d.Inspect(ctx, name)
	if errors.Is(err, docker.ErrNotFound) {
		return Status{State: StateMissing}, nil
	}
	if err != nil {
		return Status{}, err
	}
	if c.Running {
		return Status{State: StateRunning}, nil
	}
	return Status{State: c.Status}, nil
}
