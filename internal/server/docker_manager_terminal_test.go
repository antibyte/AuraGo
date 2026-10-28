package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/dockerutil"
	"aurago/internal/testutil"

	"github.com/gorilla/websocket"
)

// dmTermReq is one request the fake Engine of the Docker Manager terminal saw.
type dmTermReq struct {
	method, path, query, body string
}

// dmTermEngine speaks the exec part of the Engine API for the container "web":
// exec create (ids exec-1, exec-2, ...), a hijacked exec start that echoes the
// terminal input after a "ready$ " prompt, a detached start, and resize.
type dmTermEngine struct {
	mu     sync.Mutex
	reqs   []dmTermReq
	execs  int
	client *dockerutil.Client
	hijack atomic.Int32 // hijacked streams that ended
}

func newDMTermEngine(t *testing.T) *dmTermEngine {
	t.Helper()
	fe := &dmTermEngine{}
	server := testutil.NewHTTPServer(t, http.HandlerFunc(fe.serve))
	t.Cleanup(server.Close)
	fe.client = dockerutil.NewClient("tcp://"+server.Listener.Addr().String(), 5*time.Second)
	t.Cleanup(fe.client.CloseIdleConnections)
	return fe
}

var dmTermVersionPrefix = regexp.MustCompile(`^/v1\.\d+`)

func (fe *dmTermEngine) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	path := dmTermVersionPrefix.ReplaceAllString(r.URL.Path, "")
	if path != "/version" {
		fe.mu.Lock()
		fe.reqs = append(fe.reqs, dmTermReq{r.Method, path, r.URL.RawQuery, string(body)})
		fe.mu.Unlock()
	}
	switch {
	case path == "/version":
		_, _ = io.WriteString(w, `{"ApiVersion":"1.45","MinAPIVersion":"1.24"}`)
	case r.Method == http.MethodPost && path == "/containers/web/exec":
		fe.mu.Lock()
		fe.execs++
		id := "exec-" + string(rune('0'+fe.execs))
		fe.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"Id":"`+id+`"}`)
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/resize"):
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/start") && strings.Contains(string(body), `"Detach":true`):
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/start"):
		conn, buf, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		defer fe.hijack.Add(1)
		_, _ = buf.WriteString("HTTP/1.1 101 UPGRADED\r\nContent-Type: application/vnd.docker.raw-stream\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\r\nready$ ")
		_ = buf.Flush()
		chunk := make([]byte, 256)
		for {
			n, err := buf.Read(chunk)
			if n > 0 {
				_, _ = conn.Write(chunk[:n])
			}
			if err != nil {
				return
			}
		}
	default:
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"message":"page not found"}`)
	}
}

func (fe *dmTermEngine) requests() []dmTermReq {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	return append([]dmTermReq(nil), fe.reqs...)
}

func (fe *dmTermEngine) find(method, path string) []dmTermReq {
	var out []dmTermReq
	for _, r := range fe.requests() {
		if r.method == method && r.path == path {
			out = append(out, r)
		}
	}
	return out
}

func dmTermReadFull(t *testing.T, r io.Reader, n int) string {
	t.Helper()
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		t.Fatalf("read %d bytes: %v (got %q)", n, err, buf)
	}
	return string(buf)
}

func TestDockerManagerTerminalSessionStartsATaggedLoginShell(t *testing.T) {
	fe := newDMTermEngine(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session, err := newDockerManagerTerminalSession(ctx, fe.client, "web", 100, 40)
	if err != nil {
		t.Fatalf("newDockerManagerTerminalSession: %v", err)
	}
	defer session.Close()

	creates := fe.find(http.MethodPost, "/containers/web/exec")
	if len(creates) != 1 {
		t.Fatalf("%d exec creates, want 1", len(creates))
	}
	var body struct {
		AttachStdin, AttachStdout, AttachStderr, Tty bool
		Cmd, Env                                     []string
	}
	if err := json.Unmarshal([]byte(creates[0].body), &body); err != nil {
		t.Fatalf("exec create body %q: %v", creates[0].body, err)
	}
	wantCmd := []string{"/bin/sh", "-c", "if command -v bash >/dev/null 2>&1; then exec bash -l; fi; exec sh -l"}
	if !slices.Equal(body.Cmd, wantCmd) || !body.Tty || !body.AttachStdin || !body.AttachStdout || !body.AttachStderr {
		t.Fatalf("exec create = %+v, want the login shell command on a TTY with everything attached", body)
	}
	if len(body.Env) != 2 || body.Env[0] != "TERM=xterm-256color" || !regexp.MustCompile(`^AURAGO_TERMINAL_SESSION=[0-9a-f]{32}$`).MatchString(body.Env[1]) {
		t.Fatalf("exec env = %v, want TERM and the 32-hex session tag", body.Env)
	}
	if starts := fe.find(http.MethodPost, "/exec/exec-1/start"); len(starts) != 1 || starts[0].body != `{"Detach":false,"Tty":true}` {
		t.Fatalf("exec starts = %+v", starts)
	}
	if rs := fe.find(http.MethodPost, "/exec/exec-1/resize"); len(rs) != 1 || rs[0].query != "h=40&w=100" {
		t.Fatalf("resizes = %+v, want the initial size 100x40", rs)
	}

	if got := dmTermReadFull(t, session, len("ready$ ")); got != "ready$ " {
		t.Fatalf("first output = %q", got)
	}
	if _, err := session.Write([]byte("ls\r")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got := dmTermReadFull(t, session, 3); got != "ls\r" {
		t.Fatalf("echo = %q", got)
	}
	deadliner, ok := session.(containerTerminalWriteDeadliner)
	if !ok {
		t.Fatal("the session must support the write deadline of the serve loop")
	}
	if err := deadliner.SetWriteDeadline(time.Now().Add(-time.Second)); err != nil {
		t.Fatalf("SetWriteDeadline: %v", err)
	}
	var netErr net.Error
	if _, err := session.Write([]byte("x")); !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("write after an expired deadline = %v, want a timeout", err)
	}
	if _, ok := session.(containerTerminalEnder); !ok {
		t.Fatal("the session must support End")
	}
}

func TestDockerManagerTerminalSessionEndRunsTheTaggedHangupExec(t *testing.T) {
	fe := newDMTermEngine(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session, err := newDockerManagerTerminalSession(ctx, fe.client, "web", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.(containerTerminalEnder).End(ctx); err != nil {
		t.Fatalf("End: %v", err)
	}

	creates := fe.find(http.MethodPost, "/containers/web/exec")
	if len(creates) != 2 {
		t.Fatalf("%d exec creates, want the shell and the end helper", len(creates))
	}
	var shell, helper struct {
		Cmd, Env         []string
		Tty, AttachStdin bool
	}
	_ = json.Unmarshal([]byte(creates[0].body), &shell)
	if err := json.Unmarshal([]byte(creates[1].body), &helper); err != nil {
		t.Fatal(err)
	}
	nonce := strings.TrimPrefix(shell.Env[1], containerTerminalSessionEnv+"=")
	if len(helper.Cmd) != 5 || helper.Cmd[0] != "/bin/sh" || helper.Cmd[1] != "-c" || helper.Cmd[2] != containerTerminalEndScript ||
		helper.Cmd[3] != "aurago-end-session" || helper.Cmd[4] != nonce {
		t.Fatalf("end exec Cmd = %q, want the shared hangup script with this session's tag as an argument", helper.Cmd)
	}
	if strings.Contains(helper.Cmd[2], nonce) || helper.Tty || helper.AttachStdin {
		t.Fatalf("end exec = %+v: the tag must be an argument and the exec must not take a TTY or stdin", helper)
	}
	if starts := fe.find(http.MethodPost, "/exec/exec-2/start"); len(starts) != 1 || starts[0].body != `{"Detach":true,"Tty":false}` {
		t.Fatalf("end exec start = %+v, want a detached start", starts)
	}
	// 0x0 falls back to the Containers page's default size.
	if rs := fe.find(http.MethodPost, "/exec/exec-1/resize"); len(rs) != 1 || rs[0].query != "h=30&w=120" {
		t.Fatalf("initial resize = %+v, want 120x30", rs)
	}
	if err := (&dockerManagerTerminalSession{c: fe.client, ref: "web"}).End(ctx); err == nil {
		t.Fatal("a session without a tag must not end anything")
	}
}

func TestDockerManagerTerminalSessionResizeClampsLikeTheContainersPage(t *testing.T) {
	fe := newDMTermEngine(t)
	ctx := context.Background()
	session, err := newDockerManagerTerminalSession(ctx, fe.client, "web", 100, 40)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.Resize(ctx, 9999, 0); err != nil {
		t.Fatal(err)
	}
	if err := session.Resize(ctx, 1, 9999); err != nil {
		t.Fatal(err)
	}
	var queries []string
	for _, r := range fe.find(http.MethodPost, "/exec/exec-1/resize") {
		queries = append(queries, r.query)
	}
	if want := []string{"h=40&w=100", "h=30&w=500", "h=200&w=2"}; !slices.Equal(queries, want) {
		t.Fatalf("resize queries = %v, want %v", queries, want)
	}
}

func TestDockerManagerTerminalEnvMatchesTheContainersPagePayload(t *testing.T) {
	nonce, _ := newContainerTerminalNonce()
	want := containerTerminalExecPayload([]string{"/bin/sh"}, nonce)["Env"].([]string)
	if got := dockerManagerTerminalEnv(nonce); !slices.Equal(got, want) {
		t.Fatalf("env = %v, want the Containers page's %v", got, want)
	}
}

func TestDockerManagerTerminalServesTheSharedProtocolOverAWebSocket(t *testing.T) {
	fe := newDMTermEngine(t)
	handlerDone := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(handlerDone)
		session, err := newDockerManagerTerminalSession(r.Context(), fe.client, "web", 80, 24)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		conn, err := containerTerminalUpgrader.Upgrade(w, r, nil)
		if err != nil {
			_ = session.Close()
			return
		}
		serveDockerManagerTerminal(r.Context(), conn, session)
	}))
	defer ts.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+ts.URL[len("http"):], nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	readText := func(n int) string {
		t.Helper()
		var got []byte
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		for len(got) < n {
			_, payload, err := conn.ReadMessage()
			if err != nil {
				t.Fatalf("read: %v (got %q)", err, got)
			}
			got = append(got, payload...)
		}
		return string(got)
	}
	if got := readText(len("ready$ ")); got != "ready$ " {
		t.Fatalf("prompt = %q", got)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, []byte("pwd\r")); err != nil {
		t.Fatal(err)
	}
	if got := readText(4); got != "pwd\r" {
		t.Fatalf("echo = %q", got)
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"resize","cols":90,"rows":20}`)); err != nil {
		t.Fatal(err)
	}
	dmTermWaitFor(t, "the resize", func() bool {
		rs := fe.find(http.MethodPost, "/exec/exec-1/resize")
		return len(rs) == 2 && rs[1].query == "h=20&w=90"
	})
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"end"}`)); err != nil {
		t.Fatal(err)
	}
	dmTermWaitFor(t, "the end helper", func() bool {
		return len(fe.find(http.MethodPost, "/exec/exec-2/start")) == 1
	})
	if n := len(fe.find(http.MethodPost, "/containers/web/exec")); n != 2 {
		t.Fatalf("%d exec creates, want the shell and the end helper", n)
	}
	_ = conn.Close()
	select {
	case <-handlerDone:
	case <-time.After(5 * time.Second):
		t.Fatal("the terminal handler did not return after the browser left")
	}
	dmTermWaitFor(t, "the exec stream to close", func() bool { return fe.hijack.Load() == 1 })
}
