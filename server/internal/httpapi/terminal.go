// Terminal transport (Phase 7): a WebSocket bridging browser keystrokes to
// `tmux attach` inside a project container. Strict pre-flight — unknown
// project, stopped/missing container, or missing session are plain HTTP
// errors before the upgrade; after the upgrade everything speaks frames.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"

	"sps/internal/docker"
	"sps/internal/project"
	"sps/internal/session"
	"sps/internal/weathervane"
)

// wsIn is a browser→server frame. input carries raw keystrokes; resize
// applies the terminal dimensions to the exec TTY.
type wsIn struct {
	Type string `json:"type"` // "input" | "resize"
	Data string `json:"data,omitempty"`
	Rows int    `json:"rows,omitempty"`
	Cols int    `json:"cols,omitempty"`
}

// wsOut is a server→browser frame. output is raw pty bytes; exit ends the
// stream with the attach's exit code.
type wsOut struct {
	Type string `json:"type"` // "output" | "exit"
	Data string `json:"data,omitempty"`
	Code int    `json:"code,omitempty"`
}

// Same-origin need not be re-checked here: the session cookie is
// SameSite=Strict, so a cross-site handshake cannot carry it, and RequireAuth
// already rejected unauthenticated upgrades.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool { return true },
}

// handleTerminal upgrades and bridges. Layout: one goroutine runs the
// blocking tmux attach; one reads browser frames (input → pty stdin, resize
// → ResizeTTY); the main goroutine waits for either side to end and closes
// the other down.
func handleTerminal(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, name := r.PathValue("id"), r.PathValue("name")
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		// Pre-flight while we can still answer with real HTTP statuses.
		if !session.ValidName(name) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid session name"})
			return
		}
		if _, status, err := d.Projects.Get(ctx, id); err != nil {
			writeServiceErr(w, err)
			return
		} else if status.State == weathervane.StateMissing {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "container not found"})
			return
		} else if status.State != weathervane.StateRunning {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "container not running"})
			return
		}
		exists, err := d.Sessions.Exists(ctx, project.ContainerName(id), name)
		if err != nil {
			slog.Error("terminal", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		if !exists {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such session"})
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return // upgrade response already written
		}
		defer conn.Close()
		_, _ = d.Events.Append("terminal.attach", map[string]any{"id": id, "session": name})
		defer func() { _, _ = d.Events.Append("terminal.detach", map[string]any{"id": id, "session": name}) }()

		runTerminal(ctx, cancel, d.Sessions, conn, project.ContainerName(id), name)
	}
}

// runTerminal pumps frames between the websocket and the attach exec until
// either end closes. The exec runs against ctx: when the browser goes away,
// cancel drops the docker connection but leaves the tmux session alive in
// the container — reconnect attaches to the same scrollback.
func runTerminal(ctx context.Context, cancel context.CancelFunc, svc *session.Service, conn *websocket.Conn, container, name string) {
	defer cancel()

	inR, inW := io.Pipe()
	defer inW.Close() // signals the attach on ws-side shutdown

	var wmu sync.Mutex // gorilla writes are not concurrent-safe
	out := writerFunc(func(p []byte) (int, error) {
		wmu.Lock()
		defer wmu.Unlock()
		frame, err := json.Marshal(wsOut{Type: "output", Data: string(p)})
		if err != nil {
			return 0, err
		}
		if err := conn.WriteMessage(websocket.TextMessage, frame); err != nil {
			return 0, err
		}
		return len(p), nil
	})

	// exitFrame sends the terminal-closing frame; conn writes share wmu with
	// pty output (gorilla writes are not concurrent-safe).
	exitFrame := func(code int, detail string) {
		wmu.Lock()
		defer wmu.Unlock()
		frame, _ := json.Marshal(wsOut{Type: "exit", Code: code, Data: detail})
		_ = conn.WriteMessage(websocket.TextMessage, frame)
	}

	done := make(chan docker.ExecDone, 1)
	execID, attachDone, err := svc.Attach(ctx, container, name, inR, out, out)
	if err != nil {
		exitFrame(-1, err.Error())
		return
	}
	go func() { done <- <-attachDone }()

	resize := func(rows, cols int) {
		if rows > 0 && cols > 0 {
			_ = svc.Resize(ctx, execID, rows, cols)
		}
	}

	closed := make(chan struct{})
	go func() {
		defer close(closed)
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				return // browser went away or sent garbage at the transport level
			}
			var f wsIn
			if err := json.Unmarshal(raw, &f); err != nil {
				continue // tolerate junk frames; never kill the terminal for one bad line
			}
			switch f.Type {
			case "input":
				if _, err := io.WriteString(inW, f.Data); err != nil {
					return
				}
			case "resize":
				resize(f.Rows, f.Cols)
			}
		}
	}()

	select {
	case out_ := <-done:
		if out_.Err != nil {
			exitFrame(-1, out_.Err.Error())
		} else {
			exitFrame(out_.ExitCode, "")
		}
		// The session ended server-side; close the socket so the reader
		// goroutine unblocks and this handler can return.
		_ = conn.Close()
	case <-closed:
		// browser disconnected; cancel drops the attach, session survives
	}
	<-closed // let the reader drain before closing the pipe via defer
}

// writerFunc adapts a function to io.Writer so pty output can be marshalled
// into frames as it streams.
type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

func handleListSessions(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if _, _, err := d.Projects.Get(r.Context(), id); err != nil {
			writeServiceErr(w, err)
			return
		}
		sessions, err := d.Sessions.List(r.Context(), project.ContainerName(id))
		if err != nil {
			slog.Error("terminal", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		if sessions == nil {
			sessions = []session.Entry{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions})
	}
}

func handleCreateSession(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !session.ValidName(body.Name) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid session name"})
			return
		}
		if _, _, err := d.Projects.Get(r.Context(), id); err != nil {
			writeServiceErr(w, err)
			return
		}
		if err := d.Sessions.Create(r.Context(), project.ContainerName(id), body.Name); err != nil {
			if errors.Is(err, session.ErrInvalidName) {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			// tmux exits nonzero with "duplicate session" when the name is
			// taken. Create doubles as the terminal flow's ensure call, so
			// an existing session is success (200), not a conflict.
			if strings.Contains(err.Error(), "duplicate session") {
				_, _ = d.Events.Append("session.create", map[string]any{"id": id, "name": body.Name, "existing": true})
				writeJSON(w, http.StatusOK, map[string]any{"name": body.Name})
				return
			}
			slog.Error("terminal", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		_, _ = d.Events.Append("session.create", map[string]any{"id": id, "name": body.Name})
		writeJSON(w, http.StatusCreated, map[string]any{"name": body.Name})
	}
}
