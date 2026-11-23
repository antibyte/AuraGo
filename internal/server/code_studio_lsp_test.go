package server

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestCodeStudioLSPRejectsOversizeMessages(t *testing.T) {
	read := func(raw string) ([]byte, error) {
		return readCodeStudioLSPMessage(bufio.NewReaderSize(strings.NewReader(raw), codeStudioLSPReadBuffer))
	}
	if body, err := read("Content-Length: 2\r\n\r\n{}"); err != nil || string(body) != "{}" {
		t.Fatalf("plain message = %q, %v", body, err)
	}
	// A header block over 8 KiB is refused, even when it ends properly.
	header := "X-Padding: " + strings.Repeat("a", codeStudioLSPHeaderLimit) + "\r\nContent-Length: 2\r\n\r\n{}"
	if _, err := read(header); err == nil || !strings.Contains(err.Error(), "header exceeds") {
		t.Fatalf("oversize header: err = %v", err)
	}
	// A body over 32 MiB is refused from its Content-Length, before any body byte is read.
	body := fmt.Sprintf("Content-Length: %d\r\n\r\n{}", codeStudioLSPMessageLimit+1)
	if _, err := read(body); err == nil || !strings.Contains(err.Error(), "message exceeds") {
		t.Fatalf("oversize body: err = %v", err)
	}
}

// lspTestStream is an in-memory language-server exec: the bridge reads
// Docker-framed output from out and writes stdin into in.
type lspTestStream struct {
	outR *io.PipeReader
	outW *io.PipeWriter
	inR  *io.PipeReader
	inW  *io.PipeWriter

	mu        sync.Mutex
	stdinShut bool
	closed    bool
}

func newLSPTestStream() *lspTestStream {
	outR, outW := io.Pipe()
	inR, inW := io.Pipe()
	return &lspTestStream{outR: outR, outW: outW, inR: inR, inW: inW}
}

func (s *lspTestStream) Read(p []byte) (int, error)  { return s.outR.Read(p) }
func (s *lspTestStream) Write(p []byte) (int, error) { return s.inW.Write(p) }

// CloseWrite closes stdin. Like a real language server, the fake then exits:
// its output ends.
func (s *lspTestStream) CloseWrite() error {
	s.mu.Lock()
	s.stdinShut = true
	s.mu.Unlock()
	_ = s.inW.Close()
	return s.outW.Close()
}

func (s *lspTestStream) Close() error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	_ = s.inW.Close()
	return s.outR.Close()
}

// send writes one Docker attach frame (1 stdout, 2 stderr) to the bridge.
func (s *lspTestStream) send(t *testing.T, stream byte, payload string) {
	t.Helper()
	var frame bytes.Buffer
	writeAttachFrame(t, &frame, stream, []byte(payload))
	if _, err := s.outW.Write(frame.Bytes()); err != nil {
		t.Fatalf("write language server output: %v", err)
	}
}

type lspTestStart struct {
	cmd   []string
	nonce string
}

// lspTestDocker fakes the Docker side of the bridge: the `command -v` probe
// (Exec), the language-server exec (StartLSP) and the cleanup (EndLSP).
type lspTestDocker struct {
	fakeCodeStudioDockerAPI
	missing bool
	streams chan *lspTestStream

	mu     sync.Mutex
	probes [][]string
	starts []lspTestStart
	ends   []string
}

func newLSPTestDocker() *lspTestDocker {
	return &lspTestDocker{streams: make(chan *lspTestStream, 4)}
}

func (d *lspTestDocker) Exec(_ context.Context, _ string, cmd []string, _ time.Duration) (codeStudioExecResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.probes = append(d.probes, append([]string(nil), cmd...))
	if d.missing {
		return codeStudioExecResult{ExitCode: 127}, nil
	}
	return codeStudioExecResult{}, nil
}

func (d *lspTestDocker) StartLSP(_ context.Context, _ string, cmd []string, nonce string) (codeStudioLSPStream, error) {
	stream := newLSPTestStream()
	d.mu.Lock()
	d.starts = append(d.starts, lspTestStart{cmd: append([]string(nil), cmd...), nonce: nonce})
	d.mu.Unlock()
	d.streams <- stream
	return stream, nil
}

func (d *lspTestDocker) EndLSP(_ context.Context, _ string, nonce string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ends = append(d.ends, nonce)
	return nil
}

func (d *lspTestDocker) snapshot() ([][]string, []lspTestStart, []string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([][]string(nil), d.probes...), append([]lspTestStart(nil), d.starts...), append([]string(nil), d.ends...)
}

// dialCodeStudioLSP opens the bridge through the full handler and gate chain.
func dialCodeStudioLSP(t *testing.T, docker *lspTestDocker, lang string) *websocket.Conn {
	t.Helper()
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	h := codeStudioHandlers{server: s, docker: docker}
	srv := httptest.NewServer(http.HandlerFunc(h.handleLSP))
	t.Cleanup(srv.Close)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/api/code-studio/lsp?lang="+lang, http.Header{"Origin": {srv.URL}})
	if err != nil {
		t.Fatalf("dial lsp %s: %v", lang, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func waitCodeStudioLSPStream(t *testing.T, docker *lspTestDocker) *lspTestStream {
	t.Helper()
	select {
	case stream := <-docker.streams:
		return stream
	case <-time.After(5 * time.Second):
		t.Fatal("the language server was not started")
		return nil
	}
}

func expectCodeStudioLSPClose(t *testing.T, conn *websocket.Conn, code int) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		_, data, err := conn.ReadMessage()
		if err == nil {
			t.Logf("frame before close: %q", data)
			continue
		}
		var closeErr *websocket.CloseError
		if !errors.As(err, &closeErr) || closeErr.Code != code {
			t.Fatalf("close = %v, want code %d", err, code)
		}
		return
	}
}

// codeStudioLSPReadStdin reads n bytes of the fake server's stdin, or all of
// it up to EOF when n < 0, and fails after five seconds.
func codeStudioLSPReadStdin(t *testing.T, stream *lspTestStream, n int) string {
	t.Helper()
	done := make(chan string, 1)
	go func() {
		if n < 0 {
			data, _ := io.ReadAll(stream.inR)
			done <- string(data)
			return
		}
		data := make([]byte, n)
		read, _ := io.ReadFull(stream.inR, data)
		done <- string(data[:read])
	}()
	select {
	case data := <-done:
		return data
	case <-time.After(5 * time.Second):
		t.Fatal("timed out reading the language server stdin")
		return ""
	}
}

func waitCodeStudioLSP(t *testing.T, what string, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out: %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func codeStudioLSPUpgradeRequest(lang string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/code-studio/lsp?lang="+lang, nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-Websocket-Version", "13")
	req.Header.Set("Sec-Websocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	return req
}

func TestCodeStudioLSPFramesMessagesBothWays(t *testing.T) {
	docker := newLSPTestDocker()
	conn := dialCodeStudioLSP(t, docker, "go")
	stream := waitCodeStudioLSPStream(t, docker)

	// Server to page: one header split over two stdout frames, a stderr frame in
	// between, and two messages in one frame. Each body becomes one text frame.
	first := `{"jsonrpc":"2.0","id":1,"result":{"capabilities":{"hoverProvider":true}}}`
	second := `{"jsonrpc":"2.0","method":"window/logMessage","params":{"type":3,"message":"ready"}}`
	stream.send(t, 1, "Content-Le")
	stream.send(t, 2, "gopls: loading packages\n")
	stream.send(t, 1, fmt.Sprintf("ngth: %d\r\n\r\n%sContent-Length: %d\r\nContent-Type: application/vscode-jsonrpc; charset=utf-8\r\n\r\n%s", len(first), first, len(second), second))
	for _, want := range []string{first, second} {
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		kind, data, err := conn.ReadMessage()
		if err != nil || kind != websocket.TextMessage || string(data) != want {
			t.Fatalf("frame = %d %q, %v; want text %q", kind, data, err, want)
		}
	}

	// Page to server: one text frame becomes one Content-Length message on stdin.
	request := `{"jsonrpc":"2.0","id":2,"method":"textDocument/hover","params":{"textDocument":{"uri":"file:///workspace/main.go"},"position":{"line":0,"character":0}}}`
	if err := conn.WriteMessage(websocket.TextMessage, []byte(request)); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(request), request)
	if got := codeStudioLSPReadStdin(t, stream, len(want)); got != want {
		t.Fatalf("stdin = %q, want %q", got, want)
	}
}

func TestCodeStudioLSPRejectsInvalidJSONFrame(t *testing.T) {
	docker := newLSPTestDocker()
	conn := dialCodeStudioLSP(t, docker, "go")
	stream := waitCodeStudioLSPStream(t, docker)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"jsonrpc":"2.0","id":1,`)); err != nil {
		t.Fatal(err)
	}
	expectCodeStudioLSPClose(t, conn, websocket.CloseInvalidFramePayloadData)
	if got := codeStudioLSPReadStdin(t, stream, -1); got != "" {
		t.Fatalf("invalid frame reached stdin: %q", got)
	}
}

func TestCodeStudioLSPRejectsUnknownLanguageBeforeUpgrade(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	docker := newLSPTestDocker()
	h := codeStudioHandlers{server: s, docker: docker}
	for _, lang := range []string{"", "rust", "GO", "gopls"} {
		rec := httptest.NewRecorder()
		h.handleLSP(rec, codeStudioLSPUpgradeRequest(lang))
		if rec.Code != http.StatusBadRequest || rec.Header().Get("Upgrade") != "" {
			t.Fatalf("lang %q: status %d upgrade %q; want 400 before the upgrade", lang, rec.Code, rec.Header().Get("Upgrade"))
		}
	}
	if probes, starts, _ := docker.snapshot(); len(probes)+len(starts) != 0 {
		t.Fatalf("an unknown language reached docker: probes=%v starts=%v", probes, starts)
	}
}

func TestCodeStudioLSPRejectsReadOnlyDesktop(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	s.CfgMu.Lock()
	s.Cfg.VirtualDesktop.ReadOnly = true
	s.CfgMu.Unlock()
	docker := newLSPTestDocker()
	h := codeStudioHandlers{server: s, docker: docker}
	rec := httptest.NewRecorder()
	h.handleLSP(rec, codeStudioLSPUpgradeRequest("go"))
	if rec.Code != http.StatusForbidden || rec.Header().Get("Upgrade") != "" {
		t.Fatalf("read-only status %d upgrade %q; want 403 before the upgrade", rec.Code, rec.Header().Get("Upgrade"))
	}
	if probes, starts, _ := docker.snapshot(); len(probes)+len(starts) != 0 {
		t.Fatalf("a read-only desktop reached docker: probes=%v starts=%v", probes, starts)
	}
}

func TestCodeStudioLSPMissingBinaryCloses4404(t *testing.T) {
	docker := newLSPTestDocker()
	docker.missing = true
	conn := dialCodeStudioLSP(t, docker, "typescript")
	expectCodeStudioLSPClose(t, conn, codeStudioLSPCloseNotInstalled)
	probes, starts, _ := docker.snapshot()
	want := []string{"sh", "-c", `command -v "$1" >/dev/null`, "sh", "typescript-language-server"}
	if len(probes) != 1 || !slices.Equal(probes[0], want) {
		t.Fatalf("probes = %q, want %q", probes, want)
	}
	if len(starts) != 0 {
		t.Fatalf("a missing server was started: %v", starts)
	}
}

func TestCodeStudioLSPSessionCapCloses4429(t *testing.T) {
	if codeStudioLSPMaxSessions != 8 {
		t.Fatalf("session cap = %d, want 8", codeStudioLSPMaxSessions)
	}
	waitCodeStudioLSP(t, "no LSP session left from an earlier test", func() bool { return codeStudioLSPSessions.count() == 0 })
	for i := 0; i < codeStudioLSPMaxSessions; i++ {
		release, ok := codeStudioLSPSessions.acquire()
		if !ok {
			t.Fatalf("slot %d refused", i+1)
		}
		t.Cleanup(release)
	}
	docker := newLSPTestDocker()
	conn := dialCodeStudioLSP(t, docker, "python")
	expectCodeStudioLSPClose(t, conn, codeStudioLSPCloseTooMany)
	if probes, starts, _ := docker.snapshot(); len(probes)+len(starts) != 0 {
		t.Fatalf("a refused session reached docker: probes=%v starts=%v", probes, starts)
	}
}

func TestCodeStudioLSPCleansUpOnClose(t *testing.T) {
	docker := newLSPTestDocker()
	conn := dialCodeStudioLSP(t, docker, "go")
	stream := waitCodeStudioLSPStream(t, docker)
	_, starts, _ := docker.snapshot()
	if len(starts) != 1 || !slices.Equal(starts[0].cmd, []string{"gopls"}) || len(starts[0].nonce) != 32 {
		t.Fatalf("starts = %+v, want one gopls exec with a session tag", starts)
	}
	nonce := starts[0].nonce
	payload := codeStudioLSPExecPayload(starts[0].cmd, nonce)
	env, _ := payload["Env"].([]string)
	if payload["User"] != "developer" || payload["WorkingDir"] != codeStudioWorkspaceRoot || payload["Tty"] != false ||
		payload["AttachStdin"] != true || !slices.Contains(env, codeStudioLSPSessionEnv+"="+nonce) || !slices.Contains(env, "GOTOOLCHAIN=local") {
		t.Fatalf("exec payload = %v", payload)
	}

	// The tab goes away without a close frame.
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	waitCodeStudioLSP(t, "the cleanup ends the tagged processes", func() bool {
		_, _, ends := docker.snapshot()
		return len(ends) == 1
	})
	if _, _, ends := docker.snapshot(); ends[0] != nonce {
		t.Fatalf("cleanup tag = %q, want %q", ends[0], nonce)
	}
	stream.mu.Lock()
	stdinShut, closed := stream.stdinShut, stream.closed
	stream.mu.Unlock()
	if !stdinShut || !closed {
		t.Fatalf("stdin closed = %v, stream closed = %v; want both before the cleanup", stdinShut, closed)
	}
	waitCodeStudioLSP(t, "the session slot is released", func() bool { return codeStudioLSPSessions.count() == 0 })
}
