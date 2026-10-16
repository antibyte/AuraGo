package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/tools"

	"github.com/gorilla/websocket"
)

type terminalTestDocker struct {
	fakeCodeStudioDockerAPI
	commands []string
	shells   [][]string
}

func (d *terminalTestDocker) Exec(_ context.Context, _ string, cmd []string, _ time.Duration) (codeStudioExecResult, error) {
	d.shells = append(d.shells, cmd)
	command := cmd[2]
	d.commands = append(d.commands, command)
	if strings.HasSuffix(command, " && pwd -P") {
		for _, dir := range []string{"/workspace", "/workspace/src", "/workspace/with space"} {
			if command == "cd "+shellQuote(dir)+" && pwd -P" {
				return codeStudioExecResult{Output: dir + "\n"}, nil
			}
		}
		if command == "cd '/workspace/escape' && pwd -P" {
			return codeStudioExecResult{Output: "/etc\n"}, nil
		}
		return codeStudioExecResult{ExitCode: 1, Output: "no such directory\n"}, nil
	}
	return codeStudioExecResult{Output: "ok\n"}, nil
}

func TestCodeStudioTerminalInputAndDirectories(t *testing.T) {
	for _, tc := range []struct {
		name, cwd, wantCWD string
		frames, commands   []string
	}{
		{"absolute", "/workspace/src", "/workspace", []string{"cd /workspace\rpwd\r"}, []string{"cd '/workspace' && pwd -P", "cd '/workspace' && pwd"}},
		{"parent", "/workspace/src", "/workspace", []string{"cd ..\npwd\n"}, []string{"cd '/workspace' && pwd -P", "cd '/workspace' && pwd"}},
		{"default", "/workspace/src", "/workspace", []string{"cd\npwd\n"}, []string{"cd '/workspace' && pwd -P", "cd '/workspace' && pwd"}},
		{"quoted", "/workspace", "/workspace/with space", []string{"cd 'with space'\r\npwd\n"}, []string{"cd '/workspace/with space' && pwd -P", "cd '/workspace/with space' && pwd"}},
		{"missing", "/workspace", "/workspace", []string{"cd missing\npwd\n"}, []string{"cd '/workspace/missing' && pwd -P", "cd '/workspace' && pwd"}},
		{"escape", "/workspace", "/workspace", []string{"cd /etc\ncd ..\npwd\n"}, []string{"cd '/workspace' && pwd"}},
		{"symlink escape", "/workspace", "/workspace", []string{"cd escape\npwd\n"}, []string{"cd '/workspace/escape' && pwd -P", "cd '/workspace' && pwd"}},
		{"compound", "/workspace/src", "/workspace/src", []string{"cd /workspace && pwd\n"}, []string{"cd '/workspace/src' && cd /workspace && pwd"}},
		{"batch and split CRLF", "/workspace", "/workspace", []string{"echo one\necho tw", "o\r", "\necho three\r"}, []string{"cd '/workspace' && echo one", "cd '/workspace' && echo two", "cd '/workspace' && echo three"}},
		{"backspace", "/workspace", "/workspace", []string{"echo ü\bö\n"}, []string{"cd '/workspace' && echo ö"}},
		{"split UTF8", "/workspace", "/workspace", []string{"echo \xc3", "\xbc\n"}, []string{"cd '/workspace' && echo ü"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			docker := &terminalTestDocker{}
			h := codeStudioHandlers{docker: docker}
			session := newCodeStudioTerminalSession(tc.cwd)
			for _, input := range tc.frames {
				for {
					line, _, complete, err := session.consume(input)
					input = ""
					if err != nil {
						t.Fatal(err)
					}
					if !complete {
						break
					}
					h.executeTerminalLine(context.Background(), "test", session, line)
				}
			}
			if session.cwd != tc.wantCWD || !reflect.DeepEqual(docker.commands, tc.commands) {
				t.Fatalf("cwd=%q commands=%q; want cwd=%q commands=%q", session.cwd, docker.commands, tc.wantCWD, tc.commands)
			}
			for _, cmd := range docker.shells {
				if len(cmd) != 3 || cmd[0] != "sh" || cmd[1] != "-c" {
					t.Fatalf("terminal must preserve the image PATH, got %q", cmd)
				}
			}
		})
	}
}

func TestNormalizeCodeStudioExecCommandPreservesEnvironment(t *testing.T) {
	for _, cwd := range []string{"", "/workspace", "/workspace/with space"} {
		cmd, err := normalizeCodeStudioExecCommand("go version", nil, cwd)
		if err != nil {
			t.Fatal(err)
		}
		want := "go version"
		if cwd != "" {
			want = "cd " + shellQuote(cwd) + " && " + want
		}
		if !reflect.DeepEqual(cmd, []string{"sh", "-c", want}) {
			t.Fatalf("exec must preserve the image PATH and cwd, got %q", cmd)
		}
	}
	args := []string{"sh", "-lc", "echo explicit login shell"}
	cmd, err := normalizeCodeStudioExecCommand("", args, "")
	if err != nil || !reflect.DeepEqual(cmd, args) {
		t.Fatalf("explicit arguments changed: %q, %v", cmd, err)
	}
}

func TestCodeStudioTerminalBoundsAndCancellation(t *testing.T) {
	session := newCodeStudioTerminalSession("/workspace")
	if _, _, _, err := session.consume(strings.Repeat("x", codeStudioTerminalInputLimit)); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := session.consume("x"); err == nil {
		t.Fatal("input limit did not include the incomplete line")
	}
	docker := &terminalTestDocker{}
	h := codeStudioHandlers{docker: docker}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h.executeTerminalLine(ctx, "test", session, "echo must-not-run")
	if len(docker.commands) != 0 {
		t.Fatal("cancelled terminal executed another command")
	}
}

func TestCodeStudioTerminalWebSocketDrainsPastedLines(t *testing.T) {
	docker := &terminalTestDocker{}
	h := codeStudioHandlers{docker: docker}
	finished := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(finished)
		conn, err := codeStudioWSUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		h.runLineTerminal(r.Context(), conn, "test")
	}))
	defer srv.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), http.Header{"Origin": {srv.URL}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if err := conn.WriteMessage(websocket.TextMessage, []byte("cd src\npwd\necho done\r\npartial")); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	for !strings.HasSuffix(output.String(), "/workspace/src $ partial") {
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("pasted commands did not finish: %v; output=%q", err, output.String())
		}
		output.Write(data)
	}
	_ = conn.Close()
	<-finished
	want := []string{"cd '/workspace/src' && pwd -P", "cd '/workspace/src' && pwd", "cd '/workspace/src' && echo done"}
	if !reflect.DeepEqual(docker.commands, want) {
		t.Fatalf("commands=%q want=%q", docker.commands, want)
	}
}

type codeStudioTTYCreate struct {
	AttachStdin  bool     `json:"AttachStdin"`
	AttachStdout bool     `json:"AttachStdout"`
	AttachStderr bool     `json:"AttachStderr"`
	Cmd          []string `json:"Cmd"`
	Env          []string `json:"Env"`
	Tty          bool     `json:"Tty"`
	User         string   `json:"User"`
	WorkingDir   string   `json:"WorkingDir"`
}

// codeStudioTTYEngine records Code Studio terminal execs and hijacks the
// shell start into a raw stream.
type codeStudioTTYEngine struct {
	mu       sync.Mutex
	creates  []codeStudioTTYCreate
	resizes  []string
	hangups  int
	bashCode int
	streams  chan net.Conn
}

func newCodeStudioTTYEngine(bashCode int) *codeStudioTTYEngine {
	return &codeStudioTTYEngine{bashCode: bashCode, streams: make(chan net.Conn, 1)}
}

func (e *codeStudioTTYEngine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	path := r.URL.Path
	switch {
	case r.Method == http.MethodPost && strings.Contains(path, "/containers/") && strings.HasSuffix(path, "/exec"):
		var created codeStudioTTYCreate
		if err := json.Unmarshal(body, &created); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		e.mu.Lock()
		e.creates = append(e.creates, created)
		e.mu.Unlock()
		if created.Tty && slices.Contains(created.Cmd, "/bin/bash") && e.bashCode != 0 {
			http.Error(w, `{"message":"bash missing"}`, e.bashCode)
			return
		}
		id := "shell-1"
		if len(created.Cmd) >= 4 && created.Cmd[3] == "aurago-end-session" {
			e.mu.Lock()
			e.hangups++
			id = fmt.Sprintf("hangup-%d", e.hangups)
			e.mu.Unlock()
		}
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, `{"Id":%q}`, id)
	case r.Method == http.MethodPost && strings.Contains(path, "/exec/") && strings.HasSuffix(path, "/start"):
		if strings.Contains(path, "/exec/hangup-") || strings.Contains(string(body), `"Detach":true`) {
			w.WriteHeader(http.StatusOK)
			return
		}
		conn, err := hijackCodeStudioTTY(w)
		if err != nil {
			return
		}
		e.streams <- conn
	case r.Method == http.MethodPost && strings.Contains(path, "/resize"):
		e.mu.Lock()
		e.resizes = append(e.resizes, r.URL.RawQuery)
		e.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "unexpected "+path, http.StatusConflict)
	}
}

func hijackCodeStudioTTY(w http.ResponseWriter) (net.Conn, error) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "no hijack", http.StatusInternalServerError)
		return nil, fmt.Errorf("no hijack")
	}
	conn, bufrw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}
	if _, err := bufrw.WriteString("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\r\n"); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := bufrw.Flush(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func (e *codeStudioTTYEngine) shellCreates() []codeStudioTTYCreate {
	e.mu.Lock()
	defer e.mu.Unlock()
	var shells []codeStudioTTYCreate
	for _, created := range e.creates {
		if created.Tty {
			shells = append(shells, created)
		}
	}
	return shells
}

func waitCodeStudioTTY(t *testing.T, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if done() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the code studio tty")
}

func dialCodeStudioTerminal(t *testing.T, h codeStudioHandlers) *websocket.Conn {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := codeStudioWSUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		h.serveCodeStudioTerminal(r.Context(), conn, "code-1")
	}))
	t.Cleanup(srv.Close)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), http.Header{"Origin": {srv.URL}})
	if err != nil {
		t.Fatalf("dial terminal: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func assertCodeStudioShellCreate(t *testing.T, created codeStudioTTYCreate, cmd []string) {
	t.Helper()
	if !created.Tty || !created.AttachStdin || !created.AttachStdout || !created.AttachStderr {
		t.Fatalf("attach/tty = %+v", created)
	}
	if created.User != "developer" || created.WorkingDir != "/workspace" {
		t.Fatalf("user/dir = %q %q", created.User, created.WorkingDir)
	}
	if !slices.Equal(created.Cmd, cmd) || slices.Contains(created.Cmd, "-l") {
		t.Fatalf("cmd = %q, want %q without -l", created.Cmd, cmd)
	}
	if !slices.Contains(created.Env, "TERM=xterm-256color") {
		t.Fatalf("env = %q", created.Env)
	}
	tagged := false
	for _, env := range created.Env {
		if strings.HasPrefix(env, containerTerminalSessionEnv+"=") && len(env) == len(containerTerminalSessionEnv)+1+32 {
			tagged = true
		}
	}
	if !tagged {
		t.Fatalf("env = %q, want a session tag", created.Env)
	}
}

func TestCodeStudioTerminalPayloadDoesNotChangeTheSharedTerminalPayload(t *testing.T) {
	shared := containerTerminalExecPayload([]string{"/bin/sh"}, "abc")
	if _, ok := shared["User"]; ok || shared["WorkingDir"] != nil {
		t.Fatalf("shared payload gained code studio fields: %v", shared)
	}
	studio := codeStudioTerminalExecPayload([]string{"/bin/bash"}, "abc")
	if studio["User"] != "developer" || studio["WorkingDir"] != codeStudioWorkspaceRoot || studio["Tty"] != true {
		t.Fatalf("code studio payload = %v", studio)
	}
	again := containerTerminalExecPayload([]string{"/bin/sh"}, "def")
	if _, ok := again["User"]; ok || again["WorkingDir"] != nil {
		t.Fatal("shared payload gained code studio fields")
	}
}

func TestCodeStudioTerminalAttachesATTY(t *testing.T) {
	engine := newCodeStudioTTYEngine(0)
	host := newContainerDockerAPI(t, engine.ServeHTTP)
	h := codeStudioHandlers{docker: codeStudioDockerAdapter{cfg: tools.DockerConfig{Host: host}}}
	conn := dialCodeStudioTerminal(t, h)
	var dockerConn net.Conn
	select {
	case dockerConn = <-engine.streams:
	case <-time.After(3 * time.Second):
		t.Fatal("tty stream did not start")
	}
	t.Cleanup(func() { _ = dockerConn.Close() })

	shells := engine.shellCreates()
	if len(shells) != 1 {
		t.Fatalf("shell creates = %+v", shells)
	}
	assertCodeStudioShellCreate(t, shells[0], []string{"/bin/bash"})

	_ = dockerConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := conn.WriteMessage(websocket.TextMessage, []byte("echo nope\n")); err != nil {
		t.Fatal(err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, []byte("echo hi\n")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, len("echo hi\n"))
	n, err := io.ReadFull(dockerConn, buf)
	if err != nil || string(buf[:n]) != "echo hi\n" {
		t.Fatalf("tty input = %q, %v", buf[:n], err)
	}

	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"resize","cols":100,"rows":40}`)); err != nil {
		t.Fatal(err)
	}
	waitCodeStudioTTY(t, func() bool {
		engine.mu.Lock()
		defer engine.mu.Unlock()
		return slices.Contains(engine.resizes, "h=30&w=120") && slices.Contains(engine.resizes, "h=40&w=100")
	})

	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"end"}`)); err != nil {
		t.Fatal(err)
	}
	waitCodeStudioTTY(t, func() bool {
		engine.mu.Lock()
		defer engine.mu.Unlock()
		return engine.hangups == 1
	})
	engine.mu.Lock()
	defer engine.mu.Unlock()
	var hangup *codeStudioTTYCreate
	for i := range engine.creates {
		if len(engine.creates[i].Cmd) >= 4 && engine.creates[i].Cmd[3] == "aurago-end-session" {
			hangup = &engine.creates[i]
		}
	}
	if hangup == nil || hangup.Tty || !strings.Contains(hangup.Cmd[2], "kill -HUP") {
		t.Fatalf("hangup exec = %+v", hangup)
	}
}

func TestCodeStudioTerminalCloseDoesNotEndTheShell(t *testing.T) {
	engine := newCodeStudioTTYEngine(0)
	host := newContainerDockerAPI(t, engine.ServeHTTP)
	h := codeStudioHandlers{docker: codeStudioDockerAdapter{cfg: tools.DockerConfig{Host: host}}}
	conn := dialCodeStudioTerminal(t, h)
	var dockerConn net.Conn
	select {
	case dockerConn = <-engine.streams:
	case <-time.After(3 * time.Second):
		t.Fatal("tty stream did not start")
	}
	t.Cleanup(func() { _ = dockerConn.Close() })
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	_ = dockerConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 8)
	n, err := dockerConn.Read(buf)
	if n > 0 || err == nil {
		t.Fatalf("close wrote %q into the tty (%v)", buf[:n], err)
	}
	time.Sleep(100 * time.Millisecond)
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if engine.hangups != 0 {
		t.Fatalf("hangups = %d, want 0", engine.hangups)
	}
}

func TestCodeStudioTerminalUsesSHWhenBashCannotBeCreated(t *testing.T) {
	engine := newCodeStudioTTYEngine(http.StatusNotFound)
	host := newContainerDockerAPI(t, engine.ServeHTTP)
	session, err := openCodeStudioTerminalSession(context.Background(), tools.DockerConfig{Host: host}, "code-1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	select {
	case conn := <-engine.streams:
		t.Cleanup(func() { _ = conn.Close() })
	case <-time.After(3 * time.Second):
		t.Fatal("sh tty did not start")
	}
	shells := engine.shellCreates()
	if len(shells) != 2 || !slices.Equal(shells[0].Cmd, []string{"/bin/bash"}) {
		t.Fatalf("shell creates = %+v, want a refused bash create before sh", shells)
	}
	assertCodeStudioShellCreate(t, shells[1], []string{"/bin/sh"})
}

func TestCodeStudioTerminalFallsBackToLineMode(t *testing.T) {
	var logs bytes.Buffer
	docker := &terminalTestDocker{}
	h := codeStudioHandlers{
		server: &Server{Logger: slog.New(slog.NewTextHandler(&logs, nil))},
		docker: docker,
	}
	conn := dialCodeStudioTerminal(t, h)
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	kind, data, err := conn.ReadMessage()
	if err != nil || kind != websocket.TextMessage || string(data) != codeStudioTerminalLineModeMessage {
		t.Fatalf("line notice = %d %q, %v", kind, data, err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte("echo hi\n")); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	for !strings.Contains(output.String(), "ok\r\n") {
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("line mode did not answer: %v; output=%q", err, output.String())
		}
		output.Write(data)
	}
	if strings.Count(logs.String(), "code studio terminal fell back to line mode") != 1 {
		t.Fatalf("fallback log = %q", logs.String())
	}
	if docker.commands[0] != "cd '/workspace' && echo hi" {
		t.Fatalf("commands = %q", docker.commands)
	}
}
