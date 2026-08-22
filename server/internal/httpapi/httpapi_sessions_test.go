package httpapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/mock"

	"sps/internal/docker"
	"sps/internal/project"
	"sps/internal/session"
	dockermocks "sps/mocks/docker"
)

func newSessionDeps(t *testing.T) (Deps, *dockermocks.MockClient, *bytes.Buffer, string) {
	t.Helper()
	d, md, pinOut, dataDir := newProjectDeps(t)
	d.Sessions = session.New(md)
	return d, md, pinOut, dataDir
}

func seedProject(t *testing.T, dataDir, id string) {
	t.Helper()
	if err := project.Open(dataDir).Create(id, project.Project{Name: "x"}); err != nil {
		t.Fatal(err)
	}
}

func TestSessionsRequireAuth(t *testing.T) {
	d, _, _, dataDir := newSessionDeps(t)
	seedProject(t, dataDir, "abc")
	h := New(d)
	if rec := get(t, h, "/api/projects/abc/sessions"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("list unauthed: %d, want 401", rec.Code)
	}
	if rec := post(t, h, "/api/projects/abc/sessions", `{"name":"main"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("create unauthed: %d, want 401", rec.Code)
	}
	req := httptest.NewRequest(http.MethodGet, "/ws/projects/abc/sessions/main", nil)
	req.Header.Set("Upgrade", "websocket")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("ws unauthed: %d, want 401 (no upgrade)", rec.Code)
	}
}

func TestListSessionsEmptyIsArray(t *testing.T) {
	d, md, pinOut, dataDir := newSessionDeps(t)
	seedProject(t, dataDir, "abc")
	md.EXPECT().Inspect(mock.Anything, "sps-abc").Return(docker.Container{Running: true}, nil)
	md.EXPECT().Exec(mock.Anything, "sps-abc",
		[]string{"tmux", "list-sessions", "-F", "#{session_name}"}, false).
		Return(docker.ExecResult{ExitCode: 1, Output: "no server running on /tmp/tmux-0/default"}, nil)

	h := New(d)
	cookie := loginCookie(t, h, pinOut)
	rec := authedGet(t, h, cookie, "/api/projects/abc/sessions")
	want := "{\"sessions\":[]}\n"
	if rec.Code != http.StatusOK || rec.Body.String() != want {
		t.Fatalf("got %d %q, want 200 %q", rec.Code, rec.Body, want)
	}
}

func TestCreateSessionLifecycle(t *testing.T) {
	d, md, pinOut, dataDir := newSessionDeps(t)
	seedProject(t, dataDir, "abc")
	md.EXPECT().Inspect(mock.Anything, "sps-abc").Return(docker.Container{Running: true}, nil)
	md.EXPECT().Exec(mock.Anything, "sps-abc",
		[]string{"tmux", "new-session", "-d", "-s", "work", "-c", "/workspace"}, false).
		Return(docker.ExecResult{ExitCode: 0}, nil).Once()

	h := New(d)
	cookie := loginCookie(t, h, pinOut)

	rec := authedPost(t, h, cookie, "/api/projects/abc/sessions", `{"name":"work"}`)
	want := "{\"name\":\"work\"}\n"
	if rec.Code != http.StatusCreated || rec.Body.String() != want {
		t.Fatalf("create: got %d %q, want 201 %q", rec.Code, rec.Body, want)
	}
	ev := lastEvent(t, d)
	if ev.Type != "session.create" || ev.Data["id"] != "abc" || ev.Data["name"] != "work" {
		t.Fatalf("unexpected event: %+v", ev)
	}

	// invalid names are rejected before any docker call
	if rec := authedPost(t, h, cookie, "/api/projects/abc/sessions", `{"name":"-evil"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid name: %d, want 400", rec.Code)
	}

	// duplicate name is success for an ensure call (200), with the engine
	// detail hidden
	md.EXPECT().Inspect(mock.Anything, "sps-abc").Return(docker.Container{Running: true}, nil)
	md.EXPECT().Exec(mock.Anything, "sps-abc", mock.Anything, false).
		Return(docker.ExecResult{ExitCode: 1, Output: "duplicate session: work"}, nil)
	rec = authedPost(t, h, cookie, "/api/projects/abc/sessions", `{"name":"work"}`)
	want = "{\"name\":\"work\"}\n"
	if rec.Code != http.StatusOK || rec.Body.String() != want {
		t.Fatalf("duplicate: got %d %q, want 200 %q", rec.Code, rec.Body, want)
	}
}

func TestCreateSessionGhostProject404(t *testing.T) {
	d, _, pinOut, _ := newSessionDeps(t)
	h := New(d)
	cookie := loginCookie(t, h, pinOut)
	rec := authedPost(t, h, cookie, "/api/projects/ghost/sessions", `{"name":"main"}`)
	want := "{\"error\":\"no such project\"}\n"
	if rec.Code != http.StatusNotFound || rec.Body.String() != want {
		t.Fatalf("got %d %q, want 404 %q", rec.Code, rec.Body, want)
	}
}

// Terminal pre-flight happens before the upgrade, so failures are plain HTTP.
func TestTerminalPreflight(t *testing.T) {
	d, md, pinOut, dataDir := newSessionDeps(t)
	seedProject(t, dataDir, "abc")
	h := New(d)
	cookie := loginCookie(t, h, pinOut)

	wsGet := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Upgrade", "websocket")
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	// invalid session name → 400 without touching the store or docker
	if rec := wsGet("/ws/projects/abc/sessions/-x"); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad name: %d, want 400", rec.Code)
	}

	// ghost project → 404 before any docker call
	if rec := wsGet("/ws/projects/ghost/sessions/main"); rec.Code != http.StatusNotFound {
		t.Fatalf("ghost project: %d, want 404", rec.Code)
	}

	// missing session → strict 404, no upgrade
	md.EXPECT().Inspect(mock.Anything, "sps-abc").Return(docker.Container{Running: true}, nil)
	md.EXPECT().Exec(mock.Anything, "sps-abc", []string{"tmux", "has-session", "-t", "main"}, false).
		Return(docker.ExecResult{ExitCode: 1}, nil)
	rec := wsGet("/ws/projects/abc/sessions/main")
	want := "{\"error\":\"no such session\"}\n"
	if rec.Code != http.StatusNotFound || rec.Body.String() != want {
		t.Fatalf("missing session: got %d %q, want 404 %q", rec.Code, rec.Body, want)
	}
	if strings.Contains(rec.Header().Get("Upgrade"), "websocket") {
		t.Fatal("must not upgrade when the session is missing")
	}
}

// fakePty echoes every line back; a line containing "quit" ends the attach
// with exit code 3. Deterministic stand-in for tmux in unit tests.
func fakePty(stdin io.Reader, stdout io.Writer, done chan<- docker.ExecDone) {
	r := bufio.NewReader(stdin)
	for {
		line, rerr := r.ReadString('\n')
		if line != "" {
			fmt.Fprint(stdout, line)
		}
		if strings.Contains(line, "quit") {
			fmt.Fprint(stdout, "bye")
			done <- docker.ExecDone{ExitCode: 3}
			return
		}
		if rerr != nil {
			done <- docker.ExecDone{ExitCode: 0}
			return
		}
	}
}

// TestTerminalRoundTrip drives a full session over the bridge against a fake
// pty: input is echoed back as output frames, resize reaches ResizeTTY while
// the session runs, and quitting ends with an exit frame carrying the real
// code. Attach/detach events land in the log.
func TestTerminalRoundTrip(t *testing.T) {
	d, md, pinOut, dataDir := newSessionDeps(t)
	seedProject(t, dataDir, "abc")

	resized := make(chan [2]int, 8)
	md.EXPECT().Inspect(mock.Anything, "sps-abc").Return(docker.Container{Running: true}, nil)
	md.EXPECT().Exec(mock.Anything, "sps-abc", []string{"tmux", "has-session", "-t", "main"}, false).
		Return(docker.ExecResult{ExitCode: 0}, nil)
	md.EXPECT().ResizeTTY(mock.Anything, "exec-1", 40, 120).RunAndReturn(
		func(_ context.Context, _ string, rows, cols int) error {
			resized <- [2]int{rows, cols}
			return nil
		})
	md.EXPECT().Attach(mock.Anything, "sps-abc",
		[]string{"tmux", "attach", "-t", "main"},
		mock.Anything, mock.Anything, mock.Anything, true).
		RunAndReturn(func(ctx context.Context, _ string, _ []string, stdin io.Reader, stdout, stderr io.Writer, _ bool) (string, <-chan docker.ExecDone, error) {
			done := make(chan docker.ExecDone, 1)
			go func() { // fake pty: echo lines; "quit" ends the attach
				r := bufio.NewReader(stdin)
				for {
					line, rerr := r.ReadString('\n')
					if line != "" {
						fmt.Fprint(stdout, line)
					}
					if strings.Contains(line, "quit") {
						fmt.Fprint(stdout, "bye")
						done <- docker.ExecDone{ExitCode: 3}
						return
					}
					if rerr != nil {
						done <- docker.ExecDone{ExitCode: 0}
						return
					}
				}
			}()
			return "exec-1", done, nil
		})

	h := New(d)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	url := wsURL(srv, "abc", "main")

	conn := dialWS(t, srv, url, loginCookie(t, h, pinOut))
	defer conn.Close()

	send := func(f wsIn) {
		t.Helper()
		raw, _ := json.Marshal(f)
		if err := conn.WriteMessage(websocket.TextMessage, raw); err != nil {
			t.Fatalf("send: %v", err)
		}
	}
	readFrame := func(want string) wsOut {
		t.Helper()
		if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Fatalf("deadline: %v", err)
		}
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				t.Fatalf("read waiting for %q frame: %v", want, err)
			}
			var f wsOut
			if json.Unmarshal(raw, &f) != nil || f.Type != want {
				continue // tolerate junk frames, as the handler does
			}
			return f
		}
	}

	send(wsIn{Type: "resize", Rows: 40, Cols: 120})
	send(wsIn{Type: "input", Data: "echo hi\n"})
	if f := readFrame("output"); !strings.Contains(f.Data, "echo hi") {
		t.Fatalf("echo lost: %+v", f)
	}
	select {
	case dims := <-resized:
		if dims[0] != 40 || dims[1] != 120 {
			t.Fatalf("resize = %v, want [40 120]", dims)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("resize frame never reached ResizeTTY")
	}

	send(wsIn{Type: "input", Data: "quit\n"})
	readFrame("output") // the echoed quit line
	exit := readFrame("exit")
	if exit.Code != 3 {
		t.Fatalf("exit code = %d, want 3 from the fake pty", exit.Code)
	}

	assertEvent(t, d, "terminal.attach")
	waitForEvent(t, d, "terminal.detach")
}

// TestTerminalSurvivesAbruptClientDrop proves the server side unwinds when
// the browser vanishes mid-session: the attach sees EOF on its stdin pipe,
// the handler returns, and the detach event still lands.
func TestTerminalSurvivesAbruptClientDrop(t *testing.T) {
	d, md, pinOut, dataDir := newSessionDeps(t)
	seedProject(t, dataDir, "abc")

	attachEnded := make(chan struct{})
	md.EXPECT().Inspect(mock.Anything, "sps-abc").Return(docker.Container{Running: true}, nil)
	md.EXPECT().Exec(mock.Anything, "sps-abc", []string{"tmux", "has-session", "-t", "main"}, false).
		Return(docker.ExecResult{ExitCode: 0}, nil)
	md.EXPECT().Attach(mock.Anything, "sps-abc", mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, true).
		RunAndReturn(func(ctx context.Context, _ string, _ []string, stdin io.Reader, stdout, stderr io.Writer, _ bool) (string, <-chan docker.ExecDone, error) {
			done := make(chan docker.ExecDone, 1)
			go func() {
				io.Copy(io.Discard, stdin) // unblocks when the server closes its pipe end
				close(attachEnded)
				done <- docker.ExecDone{ExitCode: 0}
			}()
			return "exec-2", done, nil
		})

	h := New(d)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	cookie := loginCookie(t, h, pinOut)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL(srv, "abc", "main"),
		http.Header{"Cookie": []string{cookie.Name + "=" + cookie.Value}})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	sendRaw(t, conn, `{"type":"input","data":"ping"}`)

	// vanish without a close handshake
	_ = conn.UnderlyingConn().SetReadDeadline(time.Now())
	_ = conn.Close()

	select {
	case <-attachEnded:
	case <-time.After(5 * time.Second):
		t.Fatal("server never tore down the attach after the client vanished")
	}

	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		evs, _ := d.Events.Read(0, 0)
		for _, e := range evs {
			if e.Type == "terminal.detach" {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("no terminal.detach event after abrupt drop")
}

func wsURL(srv *httptest.Server, id, name string) string {
	return "ws://" + srv.Listener.Addr().String() + "/ws/projects/" + id + "/sessions/" + name
}

func dialWS(t *testing.T, srv *httptest.Server, url string, cookie *http.Cookie) *websocket.Conn {
	t.Helper()
	conn, resp, err := websocket.DefaultDialer.Dial(url,
		http.Header{"Cookie": []string{cookie.Name + "=" + cookie.Value}})
	if err != nil {
		t.Fatalf("dial %s: %v (resp %v)", url, err, resp)
	}
	return conn
}

func sendRaw(t *testing.T, conn *websocket.Conn, raw string) {
	t.Helper()
	if err := conn.WriteMessage(websocket.TextMessage, []byte(raw)); err != nil {
		t.Fatalf("send: %v", err)
	}
}

// assertEvent requires typ now; waitForEvent polls briefly — deferred
// bookkeeping on the server side can land just after the client's last read.
func assertEvent(t *testing.T, d Deps, typ string) {
	t.Helper()
	if !hasEventType(d, typ) {
		t.Fatalf("no %q event", typ)
	}
}

func waitForEvent(t *testing.T, d Deps, typ string) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		if hasEventType(d, typ) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no %q event within 5s", typ)
}

func hasEventType(d Deps, typ string) bool {
	evs, err := d.Events.Read(0, 0)
	if err != nil {
		return false
	}
	for _, e := range evs {
		if e.Type == typ {
			return true
		}
	}
	return false
}
