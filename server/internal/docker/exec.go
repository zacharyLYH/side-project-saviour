package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	dockerclient "github.com/fsouza/go-dockerclient"
)

// ExecResult is the outcome of a run-to-completion exec.
type ExecResult struct {
	ExitCode int
	Output   string // stdout (+ stderr when tty) combined
}

// Exec runs cmd in the container to completion and captures its output.
func (d *Docker) Exec(ctx context.Context, id string, cmd []string, tty bool) (ExecResult, error) {
	e, err := d.c.CreateExec(dockerclient.CreateExecOptions{
		Context: ctx, Container: id, Cmd: cmd, AttachStdout: true, AttachStderr: true, Tty: tty,
	})
	if err != nil {
		return ExecResult{}, fmt.Errorf("create exec in %s: %w", id, err)
	}
	var buf bytes.Buffer
	if err := d.c.StartExec(e.ID, dockerclient.StartExecOptions{
		Context: ctx, OutputStream: &buf, ErrorStream: &buf, Tty: tty, RawTerminal: tty,
	}); err != nil {
		return ExecResult{}, fmt.Errorf("start exec in %s: %w", id, err)
	}
	ins, err := d.c.InspectExec(e.ID)
	if err != nil {
		return ExecResult{}, fmt.Errorf("inspect exec in %s: %w", id, err)
	}
	return ExecResult{ExitCode: ins.ExitCode, Output: buf.String()}, nil
}

// AttachExec runs cmd interactively: stdin/stdout/stderr are wired through
// the exec's connection (a TTY when tty is set). It returns the exec id (for
// ResizeTTY) and the exit code once the command finishes.
func (d *Docker) AttachExec(ctx context.Context, id string, cmd []string, stdin io.Reader, stdout, stderr io.Writer, tty bool) (string, int, error) {
	e, err := d.c.CreateExec(dockerclient.CreateExecOptions{
		Context: ctx, Container: id, Cmd: cmd,
		AttachStdin: stdin != nil, AttachStdout: true, AttachStderr: true, Tty: tty,
	})
	if err != nil {
		return "", 0, fmt.Errorf("create exec in %s: %w", id, err)
	}
	if err := d.c.StartExec(e.ID, dockerclient.StartExecOptions{
		Context: ctx, InputStream: stdin, OutputStream: stdout, ErrorStream: stderr, Tty: tty, RawTerminal: tty,
	}); err != nil {
		return "", 0, fmt.Errorf("start exec in %s: %w", id, err)
	}
	ins, err := d.c.InspectExec(e.ID)
	if err != nil {
		return "", 0, fmt.Errorf("inspect exec in %s: %w", id, err)
	}
	return e.ID, ins.ExitCode, nil
}

// ResizeTTY resizes the TTY of a running interactive exec (the web terminal).
func (d *Docker) ResizeTTY(ctx context.Context, execID string, height, width int) error {
	if err := d.c.ResizeExecTTY(execID, height, width); err != nil {
		return fmt.Errorf("resize exec %s: %w", execID, err)
	}
	return nil
}

// Logs streams the container's last tail lines ("" or "all" = everything) to out.
func (d *Docker) Logs(ctx context.Context, id, tail string, out io.Writer) error {
	if tail == "" {
		tail = "all"
	}
	if err := d.c.Logs(dockerclient.LogsOptions{
		Context: ctx, Container: id, OutputStream: out, Stdout: true, Stderr: true, Tail: tail,
	}); err != nil {
		return fmt.Errorf("logs %s: %w", id, err)
	}
	return nil
}

// Port is a TCP port listening inside a container.
type Port struct {
	Number int
}

// Ports lists listening TCP ports by running ss inside the container.
func (d *Docker) Ports(ctx context.Context, id string) ([]Port, error) {
	res, err := d.Exec(ctx, id, []string{"ss", "-tlnp"}, false)
	if err != nil {
		return nil, err
	}
	return parsePorts(res.Output), nil
}

// parsePorts extracts listening TCP ports from `ss -tlnp` output. Pure, so
// it is unit-tested without a container.
func parsePorts(output string) []Port {
	seen := map[int]bool{}
	var out []Port
	for _, line := range strings.Split(output, "\n") {
		f := strings.Fields(line)
		li := -1
		for i, s := range f {
			if s == "LISTEN" {
				li = i
				break
			}
		}
		if li < 0 || li+4 >= len(f) {
			continue
		}
		local := f[li+3] // "0.0.0.0:8000" or "[::]:8000"
		port := local[strings.LastIndex(local, ":")+1:]
		n, err := strconv.Atoi(strings.Trim(port, "[]"))
		if err != nil || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, Port{Number: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out
}
