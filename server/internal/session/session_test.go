package session

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"

	"sps/internal/docker"
	dockermocks "sps/mocks/docker"
)

func TestValidName(t *testing.T) {
	cases := map[string]bool{
		"main":                  true,
		"a":                     true,
		"A-b_9":                 true,
		"":                      false,
		"-lead":                 false,
		"has space":             false,
		"sl/ash":                false,
		"unicode-é":             false,
		strings.Repeat("x", 64): true,
		strings.Repeat("x", 65): false,
	}
	for name, want := range cases {
		if got := ValidName(name); got != want {
			t.Errorf("ValidName(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestList(t *testing.T) {
	listCmd := []string{"tmux", "list-sessions", "-F", "#{session_name}"}

	t.Run("returns names in order", func(t *testing.T) {
		d := dockermocks.NewMockClient(t)
		d.EXPECT().Exec(mock.Anything, "c1", listCmd, false).
			Return(docker.ExecResult{ExitCode: 0, Output: "main\nwork\n"}, nil)
		got, err := New(d).List(context.Background(), "c1")
		if err != nil || len(got) != 2 || got[0].Name != "main" || got[1].Name != "work" {
			t.Fatalf("got %+v err=%v", got, err)
		}
	})

	t.Run("no tmux server is an empty list, not an error", func(t *testing.T) {
		d := dockermocks.NewMockClient(t)
		d.EXPECT().Exec(mock.Anything, "c1", listCmd, false).
			Return(docker.ExecResult{ExitCode: 1, Output: "no server running on /tmp/tmux-0/default"}, nil)
		got, err := New(d).List(context.Background(), "c1")
		if err != nil || len(got) != 0 {
			t.Fatalf("got %+v err=%v, want empty list", got, err)
		}
	})

	t.Run("other nonzero exit surfaces the output", func(t *testing.T) {
		d := dockermocks.NewMockClient(t)
		d.EXPECT().Exec(mock.Anything, "c1", listCmd, false).
			Return(docker.ExecResult{ExitCode: 70, Output: "tmux: exploded"}, nil)
		if _, err := New(d).List(context.Background(), "c1"); err == nil || !strings.Contains(err.Error(), "exploded") {
			t.Fatalf("err = %v, want the tmux output", err)
		}
	})

	t.Run("engine error propagates", func(t *testing.T) {
		d := dockermocks.NewMockClient(t)
		d.EXPECT().Exec(mock.Anything, "c1", listCmd, false).
			Return(docker.ExecResult{}, errors.New("engine on fire"))
		if _, err := New(d).List(context.Background(), "c1"); err == nil {
			t.Fatal("expected engine error")
		}
	})
}

func TestExists(t *testing.T) {
	hasCmd := []string{"tmux", "has-session", "-t", "main"}

	t.Run("exit 0 means exists", func(t *testing.T) {
		d := dockermocks.NewMockClient(t)
		d.EXPECT().Exec(mock.Anything, "c1", hasCmd, false).Return(docker.ExecResult{ExitCode: 0}, nil)
		got, err := New(d).Exists(context.Background(), "c1", "main")
		if err != nil || !got {
			t.Fatalf("got %v err=%v, want true", got, err)
		}
	})

	t.Run("exit 1 means missing", func(t *testing.T) {
		d := dockermocks.NewMockClient(t)
		d.EXPECT().Exec(mock.Anything, "c1", hasCmd, false).Return(docker.ExecResult{ExitCode: 1}, nil)
		got, err := New(d).Exists(context.Background(), "c1", "main")
		if err != nil || got {
			t.Fatalf("got %v err=%v, want false", got, err)
		}
	})

	t.Run("other exits are engine failures", func(t *testing.T) {
		d := dockermocks.NewMockClient(t)
		d.EXPECT().Exec(mock.Anything, "c1", hasCmd, false).Return(docker.ExecResult{ExitCode: 2, Output: "boom"}, nil)
		if _, err := New(d).Exists(context.Background(), "c1", "main"); err == nil {
			t.Fatal("expected error for exit 2")
		}
	})
}

func TestCreate(t *testing.T) {
	t.Run("rejects invalid names without touching docker", func(t *testing.T) {
		d := dockermocks.NewMockClient(t) // no expectations: any Exec call panics the mock
		if err := New(d).Create(context.Background(), "c1", "-evil"); !errors.Is(err, ErrInvalidName) {
			t.Fatalf("err = %v, want ErrInvalidName", err)
		}
	})

	t.Run("starts a detached shell in /workspace", func(t *testing.T) {
		d := dockermocks.NewMockClient(t)
		d.EXPECT().Exec(mock.Anything, "c1",
			[]string{"tmux", "new-session", "-d", "-s", "work", "-c", "/workspace"}, false).
			Return(docker.ExecResult{ExitCode: 0}, nil)
		if err := New(d).Create(context.Background(), "c1", "work"); err != nil {
			t.Fatalf("create: %v", err)
		}
	})

	t.Run("duplicate name surfaces the tmux message", func(t *testing.T) {
		d := dockermocks.NewMockClient(t)
		d.EXPECT().Exec(mock.Anything, "c1", mock.Anything, false).
			Return(docker.ExecResult{ExitCode: 1, Output: "duplicate session: work"}, nil)
		err := New(d).Create(context.Background(), "c1", "work")
		if err == nil || !strings.Contains(err.Error(), "duplicate session") {
			t.Fatalf("err = %v, want duplicate-session detail", err)
		}
	})
}

// TestAttachWiring pins the argv and TTY mode of the attach exec — the
// contract the browser transport depends on.
func TestAttachWiring(t *testing.T) {
	d := dockermocks.NewMockClient(t)
	done := make(chan docker.ExecDone, 1)
	done <- docker.ExecDone{ExitCode: 7}
	d.EXPECT().Attach(mock.Anything, "c1",
		[]string{"tmux", "attach", "-t", "main"},
		mock.Anything, mock.Anything, mock.Anything, true).
		Return("exec-9", done, nil)

	var stdin strings.Reader
	var stdout, stderr strings.Builder
	eid, gotDone, err := New(d).Attach(context.Background(), "c1", "main", &stdin, &stdout, &stderr)
	if err != nil || eid != "exec-9" {
		t.Fatalf("got (%q, %v), want exec-9 passthrough", eid, err)
	}
	if out := <-gotDone; out.ExitCode != 7 {
		t.Fatalf("done = %+v, want exit code 7", out)
	}

	d.EXPECT().ResizeTTY(mock.Anything, "exec-9", 40, 120).Return(nil)
	if err := New(d).Resize(context.Background(), "exec-9", 40, 120); err != nil {
		t.Fatalf("resize: %v", err)
	}
}
