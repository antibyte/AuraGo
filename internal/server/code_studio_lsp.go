package server

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/tools"

	"github.com/gorilla/websocket"
)

// Code Studio LSP bridge. GET /api/code-studio/lsp?lang=go|typescript|python|cpp
// upgrades to a WebSocket and pipes it to one language server, a non-TTY exec
// in the Code Studio container. The page only names a language; the command
// comes from a fixed table. Every text frame carries one JSON-RPC message in
// both directions: the bridge strips the LSP Content-Length headers from the
// server's stdout and adds them on the way to its stdin.

const (
	// codeStudioLSPSessionEnv tags every process of one session: the server and
	// the children that inherit its environment, so the cleanup finds them all.
	codeStudioLSPSessionEnv = "AURAGO_LSP_SESSION"

	// codeStudioLSPMaxSessions bounds the language servers of the process.
	codeStudioLSPMaxSessions = 8

	// codeStudioLSPHeaderLimit bounds the header block of one server message.
	codeStudioLSPHeaderLimit = 8 << 10
	// codeStudioLSPMessageLimit bounds the body of one server message.
	codeStudioLSPMessageLimit = 32 << 20
	// codeStudioLSPReadBuffer is larger than the header limit, so a header
	// line that fits the limit always fits the read buffer.
	codeStudioLSPReadBuffer = codeStudioLSPHeaderLimit + 1024
	// codeStudioLSPFrameSlack is added to the max file size for page frames:
	// a didOpen carries a whole file plus its JSON envelope.
	codeStudioLSPFrameSlack = 64 << 10
	// codeStudioLSPStderrTail is how much stderr the debug log keeps.
	codeStudioLSPStderrTail = 2 << 10

	// codeStudioLSPExitWait is how long the cleanup waits for the server to
	// exit after its stdin closed.
	codeStudioLSPExitWait = 2 * time.Second
	// codeStudioLSPCheckTimeout bounds the `command -v` probe.
	codeStudioLSPCheckTimeout = 15 * time.Second
	// codeStudioLSPCloseWriteWait bounds the write of one close frame.
	codeStudioLSPCloseWriteWait = time.Second
	// codeStudioLSPPageWriteWait bounds the write of one data frame to the page.
	codeStudioLSPPageWriteWait = 10 * time.Second

	// Close codes of the browser contract (plan 2b). A server process that
	// exits closes with websocket.CloseInternalServerErr (1011).
	codeStudioLSPCloseNotInstalled = 4404
	codeStudioLSPCloseTooMany      = 4429
	codeStudioLSPCloseStartFailed  = 4500
)

// readCodeStudioLSPMessage reads one base-protocol message: header lines up
// to an empty line (at most codeStudioLSPHeaderLimit bytes in total), then
// exactly Content-Length body bytes (at most codeStudioLSPMessageLimit). It
// returns io.EOF only when the stream ends between two messages. r must
// buffer at least codeStudioLSPReadBuffer bytes.
func readCodeStudioLSPMessage(r *bufio.Reader) ([]byte, error) {
	length := -1
	headerBytes := 0
	for {
		line, err := r.ReadSlice('\n')
		headerBytes += len(line)
		if headerBytes > codeStudioLSPHeaderLimit || errors.Is(err, bufio.ErrBufferFull) {
			return nil, fmt.Errorf("language server header exceeds %d bytes", codeStudioLSPHeaderLimit)
		}
		if err != nil {
			if errors.Is(err, io.EOF) && headerBytes > 0 {
				return nil, io.ErrUnexpectedEOF
			}
			return nil, err
		}
		text := strings.TrimRight(string(line), "\r\n")
		if text == "" {
			break
		}
		name, value, found := strings.Cut(text, ":")
		if !found {
			return nil, fmt.Errorf("malformed language server header %q", text)
		}
		if !strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || n < 0 {
			return nil, fmt.Errorf("invalid language server Content-Length %q", strings.TrimSpace(value))
		}
		if n > codeStudioLSPMessageLimit {
			return nil, fmt.Errorf("language server message exceeds %d bytes", codeStudioLSPMessageLimit)
		}
		length = n
	}
	if length < 0 {
		return nil, errors.New("language server message has no Content-Length")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, codeStudioLSPUnexpectedEOF(err)
	}
	return body, nil
}

// writeCodeStudioLSPMessage writes body with its Content-Length header in one write.
func writeCodeStudioLSPMessage(w io.Writer, body []byte) error {
	frame := make([]byte, 0, len(body)+32)
	frame = append(frame, "Content-Length: "...)
	frame = strconv.AppendInt(frame, int64(len(body)), 10)
	frame = append(frame, "\r\n\r\n"...)
	frame = append(frame, body...)
	_, err := w.Write(frame)
	return err
}

// codeStudioLSPUnexpectedEOF reports an end of stream inside a message as
// io.ErrUnexpectedEOF.
func codeStudioLSPUnexpectedEOF(err error) error {
	if errors.Is(err, io.EOF) {
		return io.ErrUnexpectedEOF
	}
	return err
}

// codeStudioLSPDemux turns a non-TTY Docker attach stream (8-byte frame
// headers: stream type, three zero bytes, big-endian payload size) into the
// server's stdout. Payloads of any other stream go to stderr.
type codeStudioLSPDemux struct {
	src       io.Reader
	stderr    io.Writer
	header    [8]byte
	remaining uint32
}

func (d *codeStudioLSPDemux) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for d.remaining == 0 {
		if _, err := io.ReadFull(d.src, d.header[:]); err != nil {
			return 0, err
		}
		size := binary.BigEndian.Uint32(d.header[4:8])
		if d.header[0] == 1 {
			d.remaining = size
			continue
		}
		dst := d.stderr
		if dst == nil {
			dst = io.Discard
		}
		if _, err := io.CopyN(dst, d.src, int64(size)); err != nil {
			return 0, codeStudioLSPUnexpectedEOF(err)
		}
	}
	if uint32(len(p)) > d.remaining {
		p = p[:d.remaining]
	}
	n, err := d.src.Read(p)
	d.remaining -= uint32(n)
	if errors.Is(err, io.EOF) && d.remaining > 0 {
		err = io.ErrUnexpectedEOF
	}
	return n, err
}

// codeStudioLSPTail keeps the last codeStudioLSPStderrTail bytes written to
// it. The bridge discards stderr and logs only this tail at debug level.
type codeStudioLSPTail struct {
	mu  sync.Mutex
	buf []byte
}

func (t *codeStudioLSPTail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if over := len(t.buf) - codeStudioLSPStderrTail; over > 0 {
		t.buf = append(t.buf[:0], t.buf[over:]...)
	}
	return len(p), nil
}

func (t *codeStudioLSPTail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.ToValidUTF8(string(t.buf), "")
}

// codeStudioLSPServer is one row of the fixed command table.
type codeStudioLSPServer struct {
	Lang    string
	Binary  string
	Command []string
}

// codeStudioLSPServerFor maps the page's lang parameter to a command. The
// page can only name a language; it never chooses the command.
func codeStudioLSPServerFor(lang string) (codeStudioLSPServer, bool) {
	switch lang {
	case "go":
		return codeStudioLSPServer{Lang: lang, Binary: "gopls", Command: []string{"gopls"}}, true
	case "typescript":
		return codeStudioLSPServer{Lang: lang, Binary: "typescript-language-server", Command: []string{"typescript-language-server", "--stdio"}}, true
	case "python":
		return codeStudioLSPServer{Lang: lang, Binary: "pyright-langserver", Command: []string{"pyright-langserver", "--stdio"}}, true
	case "cpp":
		// No background index: clangd never writes .cache/clangd into the project.
		return codeStudioLSPServer{Lang: lang, Binary: "clangd", Command: []string{"clangd", "--background-index=false"}}, true
	}
	return codeStudioLSPServer{}, false
}

// codeStudioLSPSessionCounter bounds the LSP sessions of the process. One
// AuraGo process serves one Server, so a package-level counter is server-wide
// and needs no new Server field.
type codeStudioLSPSessionCounter struct {
	mu     sync.Mutex
	active int
}

var codeStudioLSPSessions codeStudioLSPSessionCounter

// acquire takes a session slot. It returns false when all slots are taken;
// the release function is safe to call more than once.
func (c *codeStudioLSPSessionCounter) acquire() (func(), bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active >= codeStudioLSPMaxSessions {
		return nil, false
	}
	c.active++
	var once sync.Once
	return func() {
		once.Do(func() {
			c.mu.Lock()
			c.active--
			c.mu.Unlock()
		})
	}, true
}

func (c *codeStudioLSPSessionCounter) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.active
}

// codeStudioLSPStream is the attach stream of one language-server exec:
// reads return the multiplexed stdout and stderr frames, writes go to stdin,
// and CloseWrite closes stdin only.
type codeStudioLSPStream interface {
	io.ReadWriteCloser
	CloseWrite() error
}

// codeStudioLSPRunner is the streaming-exec seam of the bridge. The Docker
// adapter implements it; tests use an in-memory fake.
type codeStudioLSPRunner interface {
	// StartLSP starts cmd as a non-TTY exec with stdin, stdout and stderr
	// attached and the session tag nonce in its environment.
	StartLSP(ctx context.Context, containerID string, cmd []string, nonce string) (codeStudioLSPStream, error)
	// EndLSP ends every process of the container that carries the tag nonce.
	EndLSP(ctx context.Context, containerID, nonce string) error
}

var (
	_ codeStudioLSPRunner = codeStudioDockerAdapter{}
	_ codeStudioLSPStream = codeStudioLSPDockerStream{}
)

// codeStudioLSPExecPayload is the exec create body of one language server.
// GOTOOLCHAIN=local keeps gopls from fetching a toolchain while the network
// is off.
func codeStudioLSPExecPayload(cmd []string, nonce string) map[string]interface{} {
	return map[string]interface{}{
		"AttachStdin":  true,
		"AttachStdout": true,
		"AttachStderr": true,
		"Cmd":          cmd,
		"Env":          []string{codeStudioLSPSessionEnv + "=" + nonce, "GOTOOLCHAIN=local"},
		"Tty":          false,
		"User":         "developer",
		"WorkingDir":   codeStudioWorkspaceRoot,
	}
}

// StartLSP creates the exec and attaches its raw stream. The stream outlives
// ctx; the bridge closes it.
func (a codeStudioDockerAdapter) StartLSP(ctx context.Context, containerID string, cmd []string, nonce string) (codeStudioLSPStream, error) {
	body, err := json.Marshal(codeStudioLSPExecPayload(cmd, nonce))
	if err != nil {
		return nil, fmt.Errorf("encode language server exec: %w", err)
	}
	data, code, err := tools.DockerRequestContext(ctx, a.cfg, http.MethodPost, "/containers/"+url.PathEscape(containerID)+"/exec", string(body))
	if err != nil {
		return nil, err
	}
	if code != http.StatusCreated {
		return nil, fmt.Errorf("docker language server exec create failed: %s", strings.TrimSpace(string(data)))
	}
	var created struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal(data, &created); err != nil || created.ID == "" {
		return nil, fmt.Errorf("parse docker language server exec id: %v", err)
	}
	stream, err := openCodeStudioExecStdinStream(ctx, a.cfg, created.ID)
	if err != nil {
		return nil, err
	}
	return codeStudioLSPDockerStream{stream}, nil
}

// codeStudioLSPDockerStream adds CloseWrite to the hijacked exec connection.
// openCodeStudioExecStdinStream already refuses connections that cannot
// half-close.
type codeStudioLSPDockerStream struct{ *bufferedDockerRawConn }

func (s codeStudioLSPDockerStream) CloseWrite() error {
	if closer, ok := s.Conn.(interface{ CloseWrite() error }); ok {
		return closer.CloseWrite()
	}
	return errors.New("docker connection cannot close stdin")
}

// codeStudioLSPEndScript ends every process whose environment carries the
// session tag passed as $1: the language server and the children it started
// (tsserver, go list, ...). TERM first, KILL one second later for what is
// left. The script's own processes carry no tag. It runs as developer, the
// user of the servers, and needs tr and grep like containerTerminalEndScript.
const codeStudioLSPEndScript = `tag="` + codeStudioLSPSessionEnv + `=$1"
end_tagged() {
  for d in /proc/[0-9]*; do
    tr '\000' '\n' 2>/dev/null < "$d/environ" | grep -qxF "$tag" || continue
    kill -s "$1" "${d#/proc/}" 2>/dev/null
  done
}
end_tagged TERM
sleep 1
end_tagged KILL`

// EndLSP runs codeStudioLSPEndScript as a detached helper exec.
func (a codeStudioDockerAdapter) EndLSP(ctx context.Context, containerID, nonce string) error {
	if containerID == "" || nonce == "" {
		return errors.New("language server session has no end tag")
	}
	body, err := json.Marshal(map[string]interface{}{
		"AttachStdout": false,
		"AttachStderr": false,
		"Cmd":          []string{"/bin/sh", "-c", codeStudioLSPEndScript, "aurago-end-lsp", nonce},
		"Tty":          false,
		"User":         "developer",
	})
	if err != nil {
		return fmt.Errorf("encode language server end exec: %w", err)
	}
	data, code, err := tools.DockerRequestContext(ctx, a.cfg, http.MethodPost, "/containers/"+url.PathEscape(containerID)+"/exec", string(body))
	if err != nil {
		return err
	}
	if code != http.StatusCreated {
		return fmt.Errorf("docker language server end failed: %s", strings.TrimSpace(string(data)))
	}
	var created struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal(data, &created); err != nil || created.ID == "" {
		return fmt.Errorf("parse docker language server end exec id: %v", err)
	}
	data, code, err = tools.DockerRequestContext(ctx, a.cfg, http.MethodPost, "/exec/"+url.PathEscape(created.ID)+"/start", `{"Detach":true,"Tty":false}`)
	if err != nil {
		return err
	}
	if code != http.StatusOK && code != http.StatusCreated && code != http.StatusNoContent {
		return fmt.Errorf("docker language server end start failed: %s", strings.TrimSpace(string(data)))
	}
	return nil
}

// handleLSP is GET /api/code-studio/lsp?lang=go|typescript|python|cpp, a
// WebSocket to one language server in the Code Studio container. The gate
// chain is handleTerminal's, in the same order. Step 1 names the execute
// operation explicitly: requirePermission derives the operation from the
// path, and only paths that end in /terminal count as execute there, so a
// plain requirePermission would let a live session survive a switch to
// read-only.
func (h codeStudioHandlers) handleLSP(w http.ResponseWriter, r *http.Request) {
	if !requireDesktopOperation(h.server, w, r, desktopScopeWrite, desktopExecute) {
		return
	}
	if h.rejectReadOnly(w) {
		return
	}
	done, admitted := h.server.admitDockerTerminal(w, r)
	if !admitted {
		return
	}
	defer done()
	if !h.isAuthenticated(r) {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !desktop.SameHostWebSocketOrigin(r) {
		jsonError(w, "forbidden websocket origin", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	lsp, ok := codeStudioLSPServerFor(r.URL.Query().Get("lang"))
	if !ok {
		jsonError(w, "unknown language server", http.StatusBadRequest)
		return
	}
	b, err := h.backendFor(r)
	if err != nil {
		writeCodeStudioBackendError(w, err)
		return
	}
	local, ok := b.(*codeStudioLocalBackend)
	if !ok { // Plan D: language servers over SSH
		writeCodeStudioRemoteError(w, &codeStudioRemoteError{Status: http.StatusNotImplemented,
			Code: "lsp_unavailable", Message: "language servers are not available in this workspace yet"})
		return
	}
	release, err := local.container.HoldTerminal(local.containerID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer release()
	conn, err := codeStudioWSUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	h.serveCodeStudioLSP(r.Context(), conn, local.containerID, lsp)
}

// serveCodeStudioLSP runs one language-server session on an upgraded socket:
// session slot, install check, start, the two pipes, then the cleanup.
func (h codeStudioHandlers) serveCodeStudioLSP(ctx context.Context, conn *websocket.Conn, containerID string, lsp codeStudioLSPServer) {
	logger := h.codeStudioLogger()
	release, ok := codeStudioLSPSessions.acquire()
	if !ok {
		closeCodeStudioLSP(conn, codeStudioLSPCloseTooMany, "too many language servers")
		return
	}
	defer release()
	runner, ok := h.docker.(codeStudioLSPRunner)
	if !ok {
		closeCodeStudioLSP(conn, codeStudioLSPCloseStartFailed, "language server runner is unavailable")
		return
	}
	installed, err := h.codeStudioLSPInstalled(ctx, containerID, lsp.Binary)
	if err != nil {
		logger.Warn("code studio language server check failed", "lang", lsp.Lang, "error", err)
		closeCodeStudioLSP(conn, codeStudioLSPCloseStartFailed, "language server check failed")
		return
	}
	if !installed {
		closeCodeStudioLSP(conn, codeStudioLSPCloseNotInstalled, lsp.Binary+" is not installed")
		return
	}
	nonce, err := newContainerTerminalNonce()
	if err != nil {
		closeCodeStudioLSP(conn, codeStudioLSPCloseStartFailed, "language server start failed")
		return
	}
	stream, err := runner.StartLSP(ctx, containerID, lsp.Command, nonce)
	if err != nil {
		logger.Warn("code studio language server start failed", "lang", lsp.Lang, "error", err)
		closeCodeStudioLSP(conn, codeStudioLSPCloseStartFailed, "language server start failed")
		// A start request that failed late may still have started the process.
		h.endCodeStudioLSP(runner, containerID, nonce, lsp.Lang)
		return
	}
	tail := &codeStudioLSPTail{}
	serverDone := make(chan struct{})
	h.pipeCodeStudioLSP(ctx, conn, stream, tail, serverDone)
	// The page is gone: close stdin, give the server time to exit, then end
	// whatever still carries the session tag.
	_ = stream.CloseWrite()
	select {
	case <-serverDone:
	case <-time.After(codeStudioLSPExitWait):
	}
	_ = stream.Close()
	<-serverDone
	h.endCodeStudioLSP(runner, containerID, nonce, lsp.Lang)
	logger.Debug("code studio language server ended", "lang", lsp.Lang, "stderr_tail", tail.String())
}

// pipeCodeStudioLSP copies server messages to the page and page frames to the
// server until the page leaves, the server exits or ctx ends. The server side
// closes serverDone when it stops; the function returns when the page side
// stops, with the socket closed.
func (h codeStudioHandlers) pipeCodeStudioLSP(ctx context.Context, conn *websocket.Conn, stream codeStudioLSPStream, tail *codeStudioLSPTail, serverDone chan<- struct{}) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// A canceled session (failed ping, request end) must not wait for a blocked
	// stdin write: closing the stream frees the session slot and the process.
	stop := context.AfterFunc(ctx, func() { _ = conn.Close(); _ = stream.Close() })
	defer stop()
	conn.SetReadLimit(h.maxFileSizeBytes() + codeStudioLSPFrameSlack)
	pongWait := dockerManagerTerminalServeOptions.PongWait
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(pongWait)) })
	go func() {
		ticker := time.NewTicker(dockerManagerTerminalServeOptions.PingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(containerTerminalPingWriteWait)); err != nil {
					cancel()
					return
				}
			}
		}
	}()

	var pageDone atomic.Bool
	go func() {
		defer close(serverDone)
		reader := bufio.NewReaderSize(&codeStudioLSPDemux{src: stream, stderr: tail}, codeStudioLSPReadBuffer)
		for {
			body, err := readCodeStudioLSPMessage(reader)
			if err != nil {
				if !pageDone.Load() {
					reason := "language server exited"
					if !errors.Is(err, io.EOF) {
						reason = "language server stream failed"
					}
					closeCodeStudioLSP(conn, websocket.CloseInternalServerErr, reason)
				}
				return
			}
			_ = conn.SetWriteDeadline(time.Now().Add(codeStudioLSPPageWriteWait))
			if err := conn.WriteMessage(websocket.TextMessage, body); err != nil {
				_ = conn.Close()
				return
			}
		}
	}()

	for {
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if messageType != websocket.TextMessage {
			closeCodeStudioLSP(conn, websocket.CloseUnsupportedData, "text frames only")
			break
		}
		if !json.Valid(payload) {
			closeCodeStudioLSP(conn, websocket.CloseInvalidFramePayloadData, "invalid JSON")
			break
		}
		if deadliner, ok := stream.(containerTerminalWriteDeadliner); ok {
			_ = deadliner.SetWriteDeadline(time.Now().Add(containerTerminalWriteTimeout))
		}
		if err := writeCodeStudioLSPMessage(stream, payload); err != nil {
			closeCodeStudioLSP(conn, websocket.CloseInternalServerErr, "language server input failed")
			break
		}
	}
	pageDone.Store(true)
	_ = conn.Close()
}

// codeStudioLSPInstalled runs `command -v` for the server binary with the
// image PATH. Exit 0 means installed; another exit code means missing.
func (h codeStudioHandlers) codeStudioLSPInstalled(ctx context.Context, containerID, name string) (bool, error) {
	result, err := h.docker.Exec(ctx, containerID, []string{"sh", "-c", `command -v "$1" >/dev/null`, "sh", name}, codeStudioLSPCheckTimeout)
	if err != nil {
		return false, err
	}
	if result.ExitCode < 0 {
		return false, errors.New("the exit code of the language server probe is unknown")
	}
	return result.ExitCode == 0, nil
}

// endCodeStudioLSP ends the tagged processes. It uses its own context: the
// request context may already be canceled when the session ends.
func (h codeStudioHandlers) endCodeStudioLSP(runner codeStudioLSPRunner, containerID, nonce, lang string) {
	ctx, cancel := context.WithTimeout(context.Background(), containerTerminalEndTimeout)
	defer cancel()
	if err := runner.EndLSP(ctx, containerID, nonce); err != nil {
		h.codeStudioLogger().Warn("code studio language server cleanup failed", "lang", lang, "error", err)
	}
}

// closeCodeStudioLSP sends a close frame with code and reason, then closes the socket.
func closeCodeStudioLSP(conn *websocket.Conn, code int, reason string) {
	_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), time.Now().Add(codeStudioLSPCloseWriteWait))
	_ = conn.Close()
}
