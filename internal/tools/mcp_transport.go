package tools

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/security"

	"github.com/gorilla/websocket"
)

const mcpNetworkRequestTimeout = 60 * time.Second

type stdioMCPTransport struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    *bufio.Reader
	stderrBuf *safeBuffer
	mu        sync.Mutex
	nextID    int64
	closeOnce sync.Once
}

func newStdioMCPTransport(cmd *exec.Cmd, stdin io.WriteCloser, stdout io.Reader, stderrBuf *safeBuffer) *stdioMCPTransport {
	return &stdioMCPTransport{
		cmd:       cmd,
		stdin:     stdin,
		stdout:    bufio.NewReaderSize(stdout, 1024*1024),
		stderrBuf: stderrBuf,
	}
}

func (t *stdioMCPTransport) Send(method string, params interface{}) (*jsonRPCResponse, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	id := atomic.AddInt64(&t.nextID, 1)
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	if _, err := t.stdin.Write(append(data, '\n')); err != nil {
		return nil, fmt.Errorf("write to stdin: %w", err)
	}

	for {
		line, err := t.stdout.ReadBytes('\n')
		if err != nil {
			if t.stderrBuf != nil && t.stderrBuf.Len() > 0 {
				return nil, fmt.Errorf("read from stdout: %w (stderr: %s)", err, mcpStderrSnippet(t.stderrBuf))
			}
			return nil, fmt.Errorf("read from stdout: %w", err)
		}
		line = []byte(strings.TrimSpace(string(line)))
		if len(line) == 0 {
			continue
		}

		var resp jsonRPCResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			continue
		}
		if resp.ID != nil && *resp.ID == id {
			return &resp, nil
		}
	}
}

func (t *stdioMCPTransport) Notify(method string, params interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	notif := jsonRPCNotification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	data, err := json.Marshal(notif)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}
	if _, err := t.stdin.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write notification to stdin: %w", err)
	}
	return nil
}

func (t *stdioMCPTransport) Close() {
	t.closeOnce.Do(func() {
		if t.stdin != nil {
			_ = t.stdin.Close()
		}
		if t.cmd == nil {
			return
		}
		done := make(chan error, 1)
		go func() { done <- t.cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			if t.cmd.Process != nil {
				_ = t.cmd.Process.Kill()
			}
		}
	})
}

func newNetworkMCPConn(srv MCPServerConfig, logger *slog.Logger) (*mcpConn, error) {
	endpoint, headers, err := resolveMCPNetworkURLAndHeaders(srv)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(endpoint) == "" {
		return nil, fmt.Errorf("url is required for MCP server %q when transport=%s", srv.Name, mcpTransportMode(srv))
	}

	var transport mcpTransport
	switch mcpTransportMode(srv) {
	case "streamable_http":
		transport, err = newHTTPMCPTransport(endpoint, headers)
	case "sse":
		transport, err = newSSEMCPTransport(endpoint, headers)
	case "websocket":
		transport, err = newWebSocketMCPTransport(endpoint, headers)
	default:
		err = fmt.Errorf("unsupported MCP network transport %q", srv.Transport)
	}
	if err != nil {
		return nil, err
	}

	logger.Info("[MCP] Network server transport connected", "name", srv.Name, "transport", mcpTransportMode(srv))
	return &mcpConn{
		name:      srv.Name,
		transport: transport,
		runtime:   mcpTransportMode(srv),
		hostDir:   srv.HostWorkdir,
		contDir:   srv.ContainerWorkdir,
	}, nil
}

type httpMCPTransport struct {
	endpoint  string
	headers   map[string]string
	client    *http.Client
	mu        sync.Mutex
	nextID    int64
	sessionID string
}

func newHTTPMCPTransport(endpoint string, headers map[string]string) (*httpMCPTransport, error) {
	if err := validateMCPURL(endpoint, "streamable_http"); err != nil {
		return nil, err
	}
	return &httpMCPTransport{
		endpoint: endpoint,
		headers:  headers,
		client:   &http.Client{Timeout: mcpNetworkRequestTimeout},
	}, nil
}

func (t *httpMCPTransport) Send(method string, params interface{}) (*jsonRPCResponse, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	id := atomic.AddInt64(&t.nextID, 1)
	req := jsonRPCRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	resp, err := t.postJSON(req)
	if err != nil {
		return nil, err
	}
	if resp.ID == nil {
		return nil, fmt.Errorf("MCP HTTP response did not include request id")
	}
	return resp, nil
}

func (t *httpMCPTransport) Notify(method string, params interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	notif := jsonRPCNotification{JSONRPC: "2.0", Method: method, Params: params}
	_, err := t.postJSON(notif)
	return err
}

func (t *httpMCPTransport) Close() {}

func (t *httpMCPTransport) postJSON(payload interface{}) (*jsonRPCResponse, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, t.endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create HTTP MCP request: %w", err)
	}
	applyMCPHeaders(req.Header, t.headers)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if t.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", t.sessionID)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP MCP request: %s", security.Scrub(err.Error()))
	}
	defer resp.Body.Close()
	if sessionID := strings.TrimSpace(resp.Header.Get("Mcp-Session-Id")); sessionID != "" {
		t.sessionID = sessionID
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("HTTP MCP request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if resp.StatusCode == http.StatusAccepted || resp.ContentLength == 0 {
		return &jsonRPCResponse{}, nil
	}
	var rpcResp jsonRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		if isJSONRPCNotification(payload) {
			return &jsonRPCResponse{}, nil
		}
		return nil, fmt.Errorf("decode HTTP MCP response: %w", err)
	}
	return &rpcResp, nil
}

type websocketMCPTransport struct {
	conn      *websocket.Conn
	mu        sync.Mutex
	nextID    int64
	pending   map[int64]chan *jsonRPCResponse
	closed    chan struct{}
	closeOnce sync.Once
}

func newWebSocketMCPTransport(endpoint string, headers map[string]string) (*websocketMCPTransport, error) {
	if err := validateMCPURL(endpoint, "websocket"); err != nil {
		return nil, err
	}
	header := http.Header{}
	applyMCPHeaders(header, headers)
	conn, _, err := websocket.DefaultDialer.Dial(endpoint, header)
	if err != nil {
		return nil, fmt.Errorf("connect websocket MCP transport: %s", security.Scrub(err.Error()))
	}
	t := &websocketMCPTransport{
		conn:    conn,
		pending: make(map[int64]chan *jsonRPCResponse),
		closed:  make(chan struct{}),
	}
	go t.readLoop()
	return t, nil
}

func (t *websocketMCPTransport) Send(method string, params interface{}) (*jsonRPCResponse, error) {
	id := atomic.AddInt64(&t.nextID, 1)
	ch := make(chan *jsonRPCResponse, 1)

	t.mu.Lock()
	t.pending[id] = ch
	req := jsonRPCRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	if err := t.conn.WriteJSON(req); err != nil {
		delete(t.pending, id)
		t.mu.Unlock()
		return nil, fmt.Errorf("write websocket MCP request: %w", err)
	}
	t.mu.Unlock()

	select {
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("websocket MCP connection closed")
		}
		return resp, nil
	case <-time.After(mcpNetworkRequestTimeout):
		t.mu.Lock()
		delete(t.pending, id)
		t.mu.Unlock()
		return nil, fmt.Errorf("websocket MCP request timed out")
	case <-t.closed:
		return nil, fmt.Errorf("websocket MCP connection closed")
	}
}

func (t *websocketMCPTransport) Notify(method string, params interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	notif := jsonRPCNotification{JSONRPC: "2.0", Method: method, Params: params}
	if err := t.conn.WriteJSON(notif); err != nil {
		return fmt.Errorf("write websocket MCP notification: %w", err)
	}
	return nil
}

func (t *websocketMCPTransport) Close() {
	t.closeOnce.Do(func() {
		_ = t.conn.Close()
		close(t.closed)
		t.mu.Lock()
		for id, ch := range t.pending {
			delete(t.pending, id)
			close(ch)
		}
		t.mu.Unlock()
	})
}

func (t *websocketMCPTransport) readLoop() {
	for {
		var resp jsonRPCResponse
		if err := t.conn.ReadJSON(&resp); err != nil {
			t.Close()
			return
		}
		if resp.ID == nil {
			continue
		}
		t.mu.Lock()
		ch := t.pending[*resp.ID]
		delete(t.pending, *resp.ID)
		t.mu.Unlock()
		if ch != nil {
			ch <- &resp
		}
	}
}

type sseMCPTransport struct {
	endpointReady chan string
	endpoint      string
	client        *http.Client
	headers       map[string]string
	body          io.Closer
	mu            sync.Mutex
	nextID        int64
	pending       map[int64]chan *jsonRPCResponse
	closed        chan struct{}
	closeOnce     sync.Once
}

func newSSEMCPTransport(endpoint string, headers map[string]string) (*sseMCPTransport, error) {
	if err := validateMCPURL(endpoint, "sse"); err != nil {
		return nil, err
	}
	t := &sseMCPTransport{
		endpointReady: make(chan string, 1),
		client:        &http.Client{Timeout: 0},
		headers:       headers,
		pending:       make(map[int64]chan *jsonRPCResponse),
		closed:        make(chan struct{}),
	}
	if err := t.open(endpoint); err != nil {
		return nil, err
	}
	select {
	case messageEndpoint := <-t.endpointReady:
		t.endpoint = resolveSSEMessageEndpoint(endpoint, messageEndpoint)
		if t.endpoint == "" {
			t.Close()
			return nil, fmt.Errorf("SSE MCP endpoint event did not include a valid message URL")
		}
		return t, nil
	case <-time.After(10 * time.Second):
		t.Close()
		return nil, fmt.Errorf("SSE MCP endpoint event timed out")
	}
}

func (t *sseMCPTransport) open(endpoint string) error {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create SSE MCP request: %w", err)
	}
	applyMCPHeaders(req.Header, t.headers)
	req.Header.Set("Accept", "text/event-stream")
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("connect SSE MCP transport: %s", security.Scrub(err.Error()))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		return fmt.Errorf("connect SSE MCP transport failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	t.body = resp.Body
	go t.readLoop(resp.Body)
	return nil
}

func (t *sseMCPTransport) Send(method string, params interface{}) (*jsonRPCResponse, error) {
	id := atomic.AddInt64(&t.nextID, 1)
	ch := make(chan *jsonRPCResponse, 1)

	t.mu.Lock()
	t.pending[id] = ch
	t.mu.Unlock()

	req := jsonRPCRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	if err := t.postMessage(req); err != nil {
		t.mu.Lock()
		delete(t.pending, id)
		t.mu.Unlock()
		return nil, err
	}

	select {
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("SSE MCP connection closed")
		}
		return resp, nil
	case <-time.After(mcpNetworkRequestTimeout):
		t.mu.Lock()
		delete(t.pending, id)
		t.mu.Unlock()
		return nil, fmt.Errorf("SSE MCP request timed out")
	case <-t.closed:
		return nil, fmt.Errorf("SSE MCP connection closed")
	}
}

func (t *sseMCPTransport) Notify(method string, params interface{}) error {
	notif := jsonRPCNotification{JSONRPC: "2.0", Method: method, Params: params}
	return t.postMessage(notif)
}

func (t *sseMCPTransport) Close() {
	t.closeOnce.Do(func() {
		if t.body != nil {
			_ = t.body.Close()
		}
		close(t.closed)
		t.mu.Lock()
		for id, ch := range t.pending {
			delete(t.pending, id)
			close(ch)
		}
		t.mu.Unlock()
	})
}

func (t *sseMCPTransport) postMessage(payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, t.endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create SSE MCP message request: %w", err)
	}
	applyMCPHeaders(req.Header, t.headers)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: mcpNetworkRequestTimeout}).Do(req)
	if err != nil {
		return fmt.Errorf("post SSE MCP message: %s", security.Scrub(err.Error()))
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("post SSE MCP message failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (t *sseMCPTransport) readLoop(body io.Reader) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	eventName := ""
	var dataLines []string

	flushEvent := func() {
		data := strings.Join(dataLines, "\n")
		switch eventName {
		case "endpoint":
			select {
			case t.endpointReady <- data:
			default:
			}
		case "message", "":
			if strings.TrimSpace(data) == "" {
				return
			}
			var resp jsonRPCResponse
			if err := json.Unmarshal([]byte(data), &resp); err != nil || resp.ID == nil {
				return
			}
			t.mu.Lock()
			ch := t.pending[*resp.ID]
			delete(t.pending, *resp.ID)
			t.mu.Unlock()
			if ch != nil {
				ch <- &resp
			}
		}
		eventName = ""
		dataLines = nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			flushEvent()
			continue
		}
		if strings.HasPrefix(line, "event:") {
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	t.Close()
}

func applyMCPHeaders(dst http.Header, headers map[string]string) {
	for key, value := range headers {
		if strings.TrimSpace(key) == "" {
			continue
		}
		dst.Set(key, value)
	}
}

func validateMCPURL(rawURL, transport string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("invalid MCP %s url", transport)
	}
	switch transport {
	case "websocket":
		if parsed.Scheme != "ws" && parsed.Scheme != "wss" {
			return fmt.Errorf("MCP websocket url must use ws:// or wss://")
		}
	default:
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return fmt.Errorf("MCP %s url must use http:// or https://", transport)
		}
	}
	return nil
}

func resolveSSEMessageEndpoint(streamURL, endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}
	if parsed, err := url.Parse(endpoint); err == nil && parsed.IsAbs() {
		return endpoint
	}
	base, err := url.Parse(streamURL)
	if err != nil {
		return ""
	}
	rel, err := url.Parse(endpoint)
	if err != nil {
		return ""
	}
	return base.ResolveReference(rel).String()
}

func isJSONRPCNotification(payload interface{}) bool {
	_, ok := payload.(jsonRPCNotification)
	return ok
}
