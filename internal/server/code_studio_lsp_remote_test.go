package server

import (
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

// lspTestProcess is a fake language-server process: the stream of the existing
// bridge tests plus the process-only methods.
type lspTestProcess struct{ *lspTestStream }

func (p lspTestProcess) Stderr() io.Reader { return strings.NewReader("") }
func (p lspTestProcess) Wait() error       { return nil }

// lspRemoteTestBackend fakes an SSH workspace: Info, Exec and StartProcess of
// codeStudioBackend (every other method panics on the nil embed).
type lspRemoteTestBackend struct {
	codeStudioBackend
	info     codeStudioWorkspaceInfo
	realRoot string
	missing  bool
	streams  chan *lspTestStream

	mu     sync.Mutex
	execs  [][]string
	starts [][]string
	cwds   []string
}

func newLSPRemoteTestBackend() *lspRemoteTestBackend {
	return &lspRemoteTestBackend{
		info:     codeStudioWorkspaceInfo{ID: "ws_7f3k", Kind: codeStudioWorkspaceKindSSH, Label: "proj", Host: "build-box", User: "dev", Root: "/home/dev/my proj"},
		realRoot: "/srv/dev/my proj",
		streams:  make(chan *lspTestStream, 4),
	}
}

func (b *lspRemoteTestBackend) Info() codeStudioWorkspaceInfo { return b.info }

func (b *lspRemoteTestBackend) Exec(_ context.Context, argv []string, _ string, _ []byte, _ time.Duration) (codeStudioExecResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.execs = append(b.execs, append([]string(nil), argv...))
	if len(argv) >= 3 && strings.Contains(argv[2], "command -v") {
		if b.missing {
			return codeStudioExecResult{ExitCode: codeStudioRemoteLSPMissingExit, Output: b.realRoot + "\n"}, nil
		}
		return codeStudioExecResult{Output: b.realRoot + "\n"}, nil
	}
	return codeStudioExecResult{}, nil
}

func (b *lspRemoteTestBackend) StartProcess(_ context.Context, argv []string, vcwd string) (codeStudioProcess, error) {
	stream := newLSPTestStream()
	b.mu.Lock()
	b.starts = append(b.starts, append([]string(nil), argv...))
	b.cwds = append(b.cwds, vcwd)
	b.mu.Unlock()
	b.streams <- stream
	return lspTestProcess{stream}, nil
}

func (b *lspRemoteTestBackend) snapshot() (execs, starts [][]string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([][]string(nil), b.execs...), append([][]string(nil), b.starts...)
}

func dialCodeStudioRemoteLSP(t *testing.T, backend *lspRemoteTestBackend, lang string) *websocket.Conn {
	t.Helper()
	h := codeStudioHandlers{server: testCodeStudioServerWithFakeCodeContainer(t, 1)}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lsp, ok := codeStudioLSPServerFor(r.URL.Query().Get("lang"))
		if !ok {
			http.Error(w, "unknown language server", http.StatusBadRequest)
			return
		}
		conn, err := codeStudioWSUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		h.serveCodeStudioLSP(r.Context(), conn, newCodeStudioRemoteLSPHost(backend), lsp)
	}))
	t.Cleanup(srv.Close)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/?lang="+lang, http.Header{"Origin": {srv.URL}})
	if err != nil {
		t.Fatalf("dial remote lsp: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func lspRawFrame(body string) string {
	return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
}

func TestCodeStudioLSPRemoteRewritesURIsBothWays(t *testing.T) {
	backend := newLSPRemoteTestBackend()
	conn := dialCodeStudioRemoteLSP(t, backend, "go")
	var stream *lspTestStream
	select {
	case stream = <-backend.streams:
	case <-time.After(5 * time.Second):
		t.Fatal("the remote language server was not started")
	}
	_, starts := backend.snapshot()
	if len(starts) != 1 || len(starts[0]) != 6 || starts[0][0] != "/bin/sh" || starts[0][1] != "-c" || starts[0][3] != "aurago-lsp" ||
		!strings.Contains(starts[0][2], codeStudioLSPSessionEnv+"=") || len(starts[0][4]) != 32 || !slices.Equal(starts[0][5:], []string{"gopls"}) {
		t.Fatalf("remote start argv = %q", starts)
	}

	request := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"rootUri":"file:///workspace","rootPath":"/workspace","workspaceFolders":[{"uri":"file:///workspace","name":"workspace"}]}}`
	if err := conn.WriteMessage(websocket.TextMessage, []byte(request)); err != nil {
		t.Fatal(err)
	}
	wantIn := lspRawFrame(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"rootUri":"file:///home/dev/my%20proj","rootPath":"/home/dev/my proj","workspaceFolders":[{"uri":"file:///home/dev/my%20proj","name":"workspace"}]}}`)
	if got := codeStudioLSPReadStdin(t, stream, len(wantIn)); got != wantIn {
		t.Fatalf("stdin = %q\nwant %q", got, wantIn)
	}

	// The server reports the symlink-resolved folder (pwd -P); the page still sees /workspace.
	if _, err := stream.outW.Write([]byte(lspRawFrame(`{"jsonrpc":"2.0","method":"textDocument/publishDiagnostics","params":{"uri":"file:///srv/dev/my%20proj/main.go","diagnostics":[]}}`))); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	want := `{"jsonrpc":"2.0","method":"textDocument/publishDiagnostics","params":{"uri":"file:///workspace/main.go","diagnostics":[]}}`
	if _, data, err := conn.ReadMessage(); err != nil || string(data) != want {
		t.Fatalf("page frame = %q, %v; want %q", data, err, want)
	}

	_ = conn.Close()
	waitCodeStudioLSP(t, "the remote cleanup ends the tagged processes", func() bool {
		execs, _ := backend.snapshot()
		for _, argv := range execs {
			if len(argv) == 5 && argv[3] == "aurago-end-lsp" && len(argv[4]) == 32 {
				return true
			}
		}
		return false
	})
}

func TestCodeStudioLSPRemoteMissingServerNamesTheHost(t *testing.T) {
	backend := newLSPRemoteTestBackend()
	backend.missing = true
	conn := dialCodeStudioRemoteLSP(t, backend, "go")
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, _, err := conn.ReadMessage()
	var closeErr *websocket.CloseError
	if !errors.As(err, &closeErr) || closeErr.Code != codeStudioLSPCloseNotInstalled || closeErr.Text != "gopls is not installed on dev@build-box" {
		t.Fatalf("close = %v; want 4404 naming the host", err)
	}
	if _, starts := backend.snapshot(); len(starts) != 0 {
		t.Fatalf("a missing server was started: %q", starts)
	}
}

func TestCodeStudioLSPUnknownWorkspaceIs404BeforeUpgrade(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	docker := newLSPTestDocker()
	h := codeStudioHandlers{server: s, docker: docker}
	req := codeStudioLSPUpgradeRequest("go")
	req.URL.RawQuery = "lang=go&ws=ws_nope"
	rec := httptest.NewRecorder()
	h.handleLSP(rec, req)
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "workspace_not_found") {
		t.Fatalf("status %d %q; want 404 workspace_not_found", rec.Code, rec.Body.String())
	}
	if probes, starts, _ := docker.snapshot(); len(probes)+len(starts) != 0 {
		t.Fatalf("a remote request reached the container: probes=%v starts=%v", probes, starts)
	}
}
