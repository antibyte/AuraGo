package meshcentral

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// pendingRequest holds a channel for a request awaiting a response.
type pendingRequest struct {
	action         string        // original action name for debugging
	reqid          int           // optional legacy reqid correlation
	responseID     string        // optional MeshCentral responseid correlation
	actionFallback string        // optional action-only response correlation
	ch             chan response // buffered channel for the response
}

// response wraps the response data with any parsing/transport error.
type response struct {
	data map[string]interface{}
	err  error
}

// Client handles communication with a MeshCentral server via WebSocket.
// It supports authentication via login tokens or username/password.
type Client struct {
	url        string
	username   string // Regular username (when not using login token)
	password   string // Regular password OR token password (when loginToken is set)
	loginToken string // Login token username (e.g., "~t:automation" or "automation")
	insecure   bool

	ws          *websocket.Conn
	wsMu        sync.RWMutex // Protects ws for concurrent access
	sessionID   string       // login cookie if using user/pass
	authCookies []*http.Cookie
	reqID       int
	mu          sync.Mutex
	done        chan struct{} // Signals goroutines to stop
	closeOnce   sync.Once     // Ensures Close() is idempotent

	pendingReqs        map[int]*pendingRequest    // keyed by legacy reqid
	pendingResponseIDs map[string]*pendingRequest // keyed by MeshCentral responseid
	pendingActions     map[string][]*pendingRequest
	reqsMu             sync.RWMutex
	serverInfo         map[string]interface{}

	// Logger for debug output
	logger *slog.Logger
}

// NewClient creates a new MeshCentral client.
// Parameters:
//   - urlStr: The MeshCentral server URL (e.g., "https://mesh.example.com")
//   - username: Regular username for authentication
//   - password: Regular password OR token password (when loginToken is set)
//   - loginToken: Login token name (e.g., "automation" or "~t:automation")
//   - insecure: Skip TLS certificate verification
//
// Returns an error if the URL is invalid.
func NewClient(urlStr, username, password, loginToken string, insecure bool) (*Client, error) {
	urlStr = strings.TrimSuffix(urlStr, "/")

	// Validate URL
	if urlStr == "" {
		return nil, fmt.Errorf("URL is required")
	}
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("URL must use http or https scheme, got: %s", u.Scheme)
	}

	return &Client{
		url:                urlStr,
		username:           username,
		password:           password,
		loginToken:         loginToken,
		insecure:           insecure,
		pendingReqs:        make(map[int]*pendingRequest),
		pendingResponseIDs: make(map[string]*pendingRequest),
		pendingActions:     make(map[string][]*pendingRequest),
		done:               make(chan struct{}),
		logger:             slog.Default(),
	}, nil
}

// SetLogger sets a custom logger for the client.
func (c *Client) SetLogger(l *slog.Logger) {
	c.logger = l
}

// logf logs a formatted message at Info level.
func (c *Client) logf(format string, args ...interface{}) {
	if c.logger != nil {
		c.logger.Info(fmt.Sprintf(format, args...))
	}
}

// logAttrs logs a structured message at Info level.
func (c *Client) logAttrs(msg string, args ...interface{}) {
	if c.logger != nil {
		c.logger.Info(msg, args...)
	}
}

/*
MESHCENTRAL AUTHENTICATION FLOW
===============================

This client supports two authentication methods:

1. LOGIN TOKEN AUTH (Recommended for automation)
   - The "loginToken" field contains the token username (e.g., "~t:automation" or just "automation")
   - The "password" field contains the actual token secret
   - Flow:
     a) POST to /login with form-data: username=~t:automation&password=TOKEN_SECRET
     b) Server responds with session cookies
     c) Use cookies for WebSocket connection to /control.ashx

2. USERNAME/PASSWORD AUTH
   - Standard login with username and password
   - Flow:
     a) POST to /login with JSON: {"u": "username", "p": "password", "a": 1}
     b) Server responds with session cookies
     c) Use cookies for WebSocket connection to /control.ashx

IMPORTANT NOTES:
- The /login endpoint expects application/x-www-form-urlencoded for token login
- The WebSocket endpoint is /control.ashx (not /api/...)
- Session cookies (meshcom) must be included in WebSocket handshake
- First message after connect should be {"action": "serverinfo"} to verify auth
*/

// Connect authenticates and opens the WebSocket connection to MeshCentral.
// Uses default timeouts and no context cancellation support.
// For cancellation support, use ConnectContext instead.
func (c *Client) Connect() error {
	return c.ConnectContext(context.Background())
}

// ConnectContext authenticates and opens the WebSocket connection to MeshCentral.
// Supports context cancellation for timeouts and graceful shutdown.
func (c *Client) ConnectContext(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Build the WebSocket URL
	baseURL := strings.TrimSuffix(c.url, "/")
	u, err := url.Parse(baseURL + "/control.ashx")
	if err != nil {
		return fmt.Errorf("failed to parse WebSocket URL: %w", err)
	}

	// Convert http/https to ws/wss for WebSocket
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	}

	c.logf("[MeshCentral] Connecting to %s", c.url)

	header := http.Header{}

	// Authentication Strategy Selection
	switch {
	// Strategy 1: Login Token + Password
	// The loginToken field contains the token name (with or without ~t: prefix)
	// The password field contains the actual token secret
	case c.loginToken != "" && c.password != "":
		c.logf("[MeshCentral] Auth: Login Token")
		loginUser := c.loginToken
		if !strings.HasPrefix(loginUser, "~t:") {
			loginUser = "~t:" + loginUser
		}
		if err := c.httpLogin(ctx, loginUser); err != nil {
			return fmt.Errorf("login with token failed: %w", err)
		}
		c.addAuthCookies(header)

	// Strategy 2: Username + Password
	case c.username != "" && c.password != "":
		c.logf("[MeshCentral] Auth: Username/Password")
		if err := c.httpLogin(ctx, c.username); err != nil {
			return fmt.Errorf("login with username/password failed: %w", err)
		}
		c.addAuthCookies(header)

	// Strategy 3: Unauthenticated (will likely fail)
	default:
		c.logf("[MeshCentral] Auth: None (unauthenticated)")
	}

	// Dial WebSocket with authentication cookies
	dialer := websocket.Dialer{
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: c.insecure},
		HandshakeTimeout: 10 * time.Second,
	}

	ws, resp, err := dialer.DialContext(ctx, u.String(), header)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("WebSocket handshake failed: HTTP %d - %s", resp.StatusCode, string(body))
		}
		return fmt.Errorf("WebSocket dial failed: %w", err)
	}

	c.wsMu.Lock()
	c.ws = ws
	c.wsMu.Unlock()

	// Session keepalive timeout: connection closes if server stops responding to pings within 90s.
	const sessionTimeout = 90 * time.Second
	ws.SetReadDeadline(time.Now().Add(sessionTimeout))
	ws.SetPongHandler(func(string) error {
		return ws.SetReadDeadline(time.Now().Add(sessionTimeout))
	})

	go c.readPump()
	go c.pingPump()

	res, err := c.WaitForServerInfoContext(ctx, 10*time.Second)
	if err != nil {
		c.Close()
		return fmt.Errorf("serverinfo verification failed: %w", err)
	}

	c.logf("[MeshCentral] Connected (server version: %v)", res["serverVersion"])
	return nil
}

// httpLogin performs HTTP POST to /login to obtain session cookies.
// For token auth: username should be "~t:tokenname", password is the token secret.
func (c *Client) httpLogin(ctx context.Context, loginUser string) error {
	baseURL := strings.TrimSuffix(c.url, "/")
	u, err := url.Parse(baseURL + "/login")
	if err != nil {
		return fmt.Errorf("failed to parse login URL: %w", err)
	}

	// Ensure HTTP scheme for login request
	switch u.Scheme {
	case "ws":
		u.Scheme = "http"
	case "wss":
		u.Scheme = "https"
	}

	loginURL := u.String()

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: c.insecure},
		},
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects automatically
		},
	}

	// Get CSRF nonce from login page
	var nonce string
	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, loginURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create login page request: %w", err)
	}
	if getResp, err := httpClient.Do(getReq); err == nil {
		pageBytes, _ := io.ReadAll(io.LimitReader(getResp.Body, 32*1024))
		getResp.Body.Close()
		if m := regexp.MustCompile(`random="([^"]+)"`).FindSubmatch(pageBytes); len(m) == 2 {
			nonce = string(m[1])
		}
	} else {
		c.logf("[MeshCentral] Warning: failed to fetch CSRF nonce: %v", err)
	}

	// Determine content type based on auth method
	var bodyData []byte
	contentType := "application/json"

	if strings.HasPrefix(loginUser, "~t:") {
		// Token login uses form data: username=~t:name&password=secret
		formData := url.Values{}
		formData.Set("username", loginUser)
		formData.Set("password", c.password)
		bodyData = []byte(formData.Encode())
		contentType = "application/x-www-form-urlencoded"
	} else {
		// Regular login uses JSON
		payload := map[string]interface{}{
			"u":  loginUser,
			"p":  c.password,
			"a":  1, // Web/Agent login
			"rn": nonce,
		}
		bodyData, _ = json.Marshal(payload)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL, bytes.NewBuffer(bodyData))
	if err != nil {
		return fmt.Errorf("failed to create login request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login HTTP %d: %s", resp.StatusCode, string(body))
	}

	if cookies := resp.Cookies(); len(cookies) > 0 {
		c.authCookies = append([]*http.Cookie(nil), cookies...)
		for _, cookie := range cookies {
			if cookie.Name == "meshcom" {
				c.sessionID = cookie.Value
				break
			}
		}
		return nil
	}

	return fmt.Errorf("no auth cookies received from login endpoint")
}

// addAuthCookies adds authentication cookies to the WebSocket header.
func (c *Client) addAuthCookies(header http.Header) {
	if len(c.authCookies) > 0 {
		parts := make([]string, 0, len(c.authCookies))
		for _, ck := range c.authCookies {
			if ck != nil && ck.Name != "" && ck.Value != "" {
				parts = append(parts, ck.Name+"="+ck.Value)
			}
		}
		if len(parts) > 0 {
			header.Set("Cookie", strings.Join(parts, "; "))
		}
	} else if c.sessionID != "" {
		header.Set("Cookie", (&http.Cookie{Name: "meshcom", Value: c.sessionID}).String())
	}
}

// IsConnected returns true if the WebSocket connection is open.
func (c *Client) IsConnected() bool {
	c.wsMu.RLock()
	ws := c.ws
	c.wsMu.RUnlock()
	return ws != nil
}

// Close gracefully closes the WebSocket connection. Safe to call multiple times.
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.done)

		c.wsMu.Lock()
		if c.ws != nil {
			c.ws.Close()
			c.ws = nil
		}
		c.wsMu.Unlock()

		// Clean up pending request channels and notify waiters of disconnect
		c.failPendingRequests(fmt.Errorf("client closed"))

		c.logf("[MeshCentral] Client closed")
	})
}

// readPump reads messages from the WebSocket and routes them to registered channels.
func (c *Client) readPump() {
	for {
		select {
		case <-c.done:
			return
		default:
		}

		c.wsMu.RLock()
		ws := c.ws
		c.wsMu.RUnlock()

		if ws == nil {
			return
		}

		_, msg, err := ws.ReadMessage()
		if err != nil {
			c.logf("[MeshCentral] WebSocket read error: %v", err)
			c.wsMu.Lock()
			if c.ws == ws {
				c.ws = nil
			}
			c.wsMu.Unlock()
			c.failPendingRequests(fmt.Errorf("websocket disconnected: %w", err))
			return
		}

		// Skip non-JSON messages
		if len(msg) < 2 || msg[0] != '{' {
			continue
		}

		var data map[string]interface{}
		if err := json.Unmarshal(msg, &data); err != nil {
			c.logf("[MeshCentral] Failed to parse JSON message: %v", err)
			continue
		}

		// Route by reqid if present (ensures correct request/response matching)
		var reqid int
		if rid, ok := data["reqid"].(float64); ok {
			reqid = int(rid)
		}

		action, _ := data["action"].(string)
		responseID, _ := data["responseid"].(string)

		if action == "serverinfo" {
			if info := extractServerInfo(data); info != nil {
				c.reqsMu.Lock()
				c.serverInfo = info
				c.reqsMu.Unlock()
			}
		}

		c.reqsMu.Lock()
		var delivered bool
		if reqid > 0 {
			if pr, ok := c.pendingReqs[reqid]; ok {
				delivered = deliverPending(pr, response{data: data})
				if delivered {
					c.logf("[MeshCentral] Delivered response to reqid %d (action=%s)", reqid, action)
				}
			}
		}
		if !delivered && responseID != "" {
			if pr, ok := c.pendingResponseIDs[responseID]; ok {
				delivered = deliverPending(pr, response{data: data})
				if delivered {
					c.logAttrs("[MeshCentral] Delivered response to responseid", "responseid", responseID, "action", action)
				}
			}
		}
		if !delivered && action != "" {
			if queue := c.pendingActions[action]; len(queue) > 0 {
				pr := queue[0]
				c.pendingActions[action] = queue[1:]
				if len(c.pendingActions[action]) == 0 {
					delete(c.pendingActions, action)
				}
				delivered = deliverPending(pr, response{data: data})
				if delivered {
					c.logAttrs("[MeshCentral] Delivered action-only response", "action", action)
				}
			}
		}
		c.reqsMu.Unlock()

		// Fallback: route events by eventType if no reqid match
		if !delivered && action == "event" {
			if eventType, ok := data["eventType"].(string); ok {
				c.logAttrs("[MeshCentral] Received event", "eventType", eventType)
				// Note: Events don't have reqid, so we can't route them properly
				// without additional event subscription mechanism
			}
		}

		// Debug logging for unhandled non-event messages
		if !delivered && action != "" && action != "ping" && action != "pong" && action != "serverinfo" {
			c.logAttrs("[MeshCentral] Unmatched message", "action", action, "reqid", reqid)
		}
	}
}

func (c *Client) failPendingRequests(err error) {
	c.reqsMu.Lock()
	defer c.reqsMu.Unlock()
	seen := make(map[*pendingRequest]bool)
	for _, pr := range c.pendingReqs {
		if seen[pr] {
			continue
		}
		seen[pr] = true
		select {
		case pr.ch <- response{err: err}:
		default:
		}
	}
	for _, pr := range c.pendingResponseIDs {
		if seen[pr] {
			continue
		}
		seen[pr] = true
		select {
		case pr.ch <- response{err: err}:
		default:
		}
	}
	for _, queue := range c.pendingActions {
		for _, pr := range queue {
			if seen[pr] {
				continue
			}
			seen[pr] = true
			select {
			case pr.ch <- response{err: err}:
			default:
			}
		}
	}
	c.pendingReqs = make(map[int]*pendingRequest)
	c.pendingResponseIDs = make(map[string]*pendingRequest)
	c.pendingActions = make(map[string][]*pendingRequest)
}

// Send sends a JSON command to the server and registers it for response routing.
// Returns the assigned reqid for correlation with the response.
func (c *Client) Send(cmd map[string]interface{}) (int, error) {
	reqid, _, _, err := c.send(cmd, false, false, true)
	return reqid, err
}

func (c *Client) sendForResponse(cmd map[string]interface{}, actionFallback bool) (*pendingRequest, error) {
	_, _, pr, err := c.send(cmd, true, actionFallback, true)
	return pr, err
}

func (c *Client) sendNoResponse(cmd map[string]interface{}) error {
	_, _, _, err := c.send(cmd, false, false, false)
	return err
}

func (c *Client) send(cmd map[string]interface{}, withResponseID, actionFallback, registerPending bool) (int, string, *pendingRequest, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.wsMu.RLock()
	ws := c.ws
	c.wsMu.RUnlock()

	if ws == nil {
		return 0, "", nil, fmt.Errorf("not connected")
	}

	c.reqID++
	reqid := c.reqID
	cmd["reqid"] = reqid
	action, _ := cmd["action"].(string)
	responseID, _ := cmd["responseid"].(string)
	if withResponseID && responseID == "" {
		responseID = fmt.Sprintf("aurago-%d", reqid)
		cmd["responseid"] = responseID
	}

	var pr *pendingRequest
	if registerPending {
		c.reqsMu.Lock()
		if withResponseID || actionFallback {
			pr = &pendingRequest{
				action:     action,
				reqid:      reqid,
				responseID: responseID,
				ch:         make(chan response, 1),
			}
			c.pendingReqs[reqid] = pr
			if responseID != "" {
				c.pendingResponseIDs[responseID] = pr
			}
			if actionFallback && action != "" {
				pr.actionFallback = action
				c.pendingActions[action] = append(c.pendingActions[action], pr)
			}
		} else {
			pr = &pendingRequest{
				action: action,
				reqid:  reqid,
				ch:     make(chan response, 1),
			}
			c.pendingReqs[reqid] = pr
		}
		c.reqsMu.Unlock()
	}

	if err := ws.WriteJSON(cmd); err != nil {
		if pr != nil {
			c.reqsMu.Lock()
			c.removePendingLocked(pr)
			c.reqsMu.Unlock()
		}
		return 0, "", nil, err
	}
	return reqid, responseID, pr, nil
}

// WaitForReq waits for a response with the given reqid.
// This is the low-level primitive that readPump delivers to via reqid routing.
func (c *Client) WaitForReq(reqid int, action string, timeout time.Duration) (map[string]interface{}, error) {
	return c.WaitForReqContext(context.Background(), reqid, action, timeout)
}

// WaitForReqContext waits for a response with the given reqid or context cancellation.
func (c *Client) WaitForReqContext(ctx context.Context, reqid int, action string, timeout time.Duration) (map[string]interface{}, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	c.reqsMu.Lock()
	if c.pendingReqs[reqid] == nil {
		c.pendingReqs[reqid] = &pendingRequest{
			action: action,
			reqid:  reqid,
			ch:     make(chan response, 1),
		}
	}
	pr := c.pendingReqs[reqid]
	c.reqsMu.Unlock()

	return c.waitForPending(ctx, pr, timeout)
}

func (c *Client) waitForPending(ctx context.Context, pr *pendingRequest, timeout time.Duration) (map[string]interface{}, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case res := <-pr.ch:
		c.reqsMu.Lock()
		c.removePendingLocked(pr)
		close(pr.ch)
		c.reqsMu.Unlock()
		if res.err != nil {
			return nil, res.err
		}
		return res.data, nil
	case <-timer.C:
		c.reqsMu.Lock()
		c.removePendingLocked(pr)
		close(pr.ch)
		c.reqsMu.Unlock()
		return nil, fmt.Errorf("timeout waiting for MeshCentral response (action=%s, responseid=%s, reqid=%d)", pr.action, pr.responseID, pr.reqid)
	case <-ctx.Done():
		c.reqsMu.Lock()
		c.removePendingLocked(pr)
		close(pr.ch)
		c.reqsMu.Unlock()
		return nil, ctx.Err()
	case <-c.done:
		c.reqsMu.Lock()
		c.removePendingLocked(pr)
		close(pr.ch)
		c.reqsMu.Unlock()
		return nil, fmt.Errorf("client disconnected")
	}
}

func (c *Client) removePendingLocked(pr *pendingRequest) {
	if pr == nil {
		return
	}
	if pr.reqid > 0 && c.pendingReqs[pr.reqid] == pr {
		delete(c.pendingReqs, pr.reqid)
	}
	if pr.responseID != "" && c.pendingResponseIDs[pr.responseID] == pr {
		delete(c.pendingResponseIDs, pr.responseID)
	}
	if pr.actionFallback != "" {
		queue := c.pendingActions[pr.actionFallback]
		for i, queued := range queue {
			if queued == pr {
				queue = append(queue[:i], queue[i+1:]...)
				break
			}
		}
		if len(queue) == 0 {
			delete(c.pendingActions, pr.actionFallback)
		} else {
			c.pendingActions[pr.actionFallback] = queue
		}
	}
}

func deliverPending(pr *pendingRequest, res response) bool {
	if pr == nil {
		return false
	}
	delivered := false
	func() {
		defer func() {
			if recover() != nil {
				delivered = false
			}
		}()
		select {
		case pr.ch <- res:
			delivered = true
		default:
		}
	}()
	return delivered
}

// WaitForAction waits for a response with the given action string.
// It sends the command, then waits for the response using reqid-based routing.
// DEPRECATED: Prefer Send + WaitForReq for explicit reqid control.
func (c *Client) WaitForAction(action string, timeout time.Duration) (map[string]interface{}, error) {
	cmd := map[string]interface{}{"action": action}
	reqid, err := c.Send(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to send %s request: %w", action, err)
	}
	return c.WaitForReq(reqid, action, timeout)
}

// WaitForServerInfoContext waits for MeshCentral's initial serverinfo frame.
func (c *Client) WaitForServerInfoContext(ctx context.Context, timeout time.Duration) (map[string]interface{}, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	c.reqsMu.Lock()
	if c.serverInfo != nil {
		info := cloneMap(c.serverInfo)
		c.reqsMu.Unlock()
		return info, nil
	}
	pr := &pendingRequest{
		action:         "serverinfo",
		actionFallback: "serverinfo",
		ch:             make(chan response, 1),
	}
	c.pendingActions["serverinfo"] = append(c.pendingActions["serverinfo"], pr)
	c.reqsMu.Unlock()

	res, err := c.waitForPending(ctx, pr, timeout)
	if err != nil {
		return nil, err
	}
	info := extractServerInfo(res)
	if info == nil {
		return nil, fmt.Errorf("invalid response format for serverinfo")
	}
	return info, nil
}

// pingPump sends periodic ping messages to keep the connection alive.
func (c *Client) pingPump() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			c.wsMu.RLock()
			ws := c.ws
			c.wsMu.RUnlock()

			if ws == nil {
				return
			}

			_ = ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				_ = ws.SetWriteDeadline(time.Time{})
				return
			}
			_ = ws.SetWriteDeadline(time.Time{})
		}
	}
}

// --- High Level API Methods --- //

// ServerInfo returns the MeshCentral server information received at connect time.
func (c *Client) ServerInfo() (map[string]interface{}, error) {
	return c.WaitForServerInfoContext(context.Background(), 10*time.Second)
}

// ListDeviceGroups requests the meshes/groups list.
func (c *Client) ListDeviceGroups() ([]interface{}, error) {
	pr, err := c.sendForResponse(map[string]interface{}{"action": "meshes"}, true)
	if err != nil {
		return nil, fmt.Errorf("failed to send meshes request: %w", err)
	}

	res, err := c.waitForPending(context.Background(), pr, 10*time.Second)
	if err != nil {
		return nil, err
	}

	if meshes, ok := res["meshes"].([]interface{}); ok {
		return meshes, nil
	}
	return nil, fmt.Errorf("invalid response format for meshes")
}

// ListDevices requests the nodes/devices list.
func (c *Client) ListDevices(meshID string) ([]interface{}, error) {
	cmd := map[string]interface{}{"action": "nodes"}
	if meshID != "" {
		cmd["meshid"] = meshID
	}

	pr, err := c.sendForResponse(cmd, true)
	if err != nil {
		return nil, fmt.Errorf("failed to send nodes request: %w", err)
	}

	res, err := c.waitForPending(context.Background(), pr, 10*time.Second)
	if err != nil {
		return nil, err
	}

	if nodes, ok := res["nodes"].([]interface{}); ok {
		return nodes, nil
	}

	// Complex case: nodes dictionary organized by meshID
	if nodesMap, ok := res["nodes"].(map[string]interface{}); ok {
		var allNodes []interface{}
		for _, meshNodes := range nodesMap {
			if list, ok := meshNodes.([]interface{}); ok {
				allNodes = append(allNodes, list...)
			}
		}
		return allNodes, nil
	}

	return nil, fmt.Errorf("invalid response format for nodes")
}

// WakeOnLan sends a WOL magic packet to a specific node.
// Returns success message or error.
func (c *Client) WakeOnLan(nodeIDs []string) (string, error) {
	err := c.sendNoResponse(map[string]interface{}{
		"action":  "wakedevices",
		"nodeids": nodeIDs,
	})
	if err != nil {
		return "", fmt.Errorf("failed to send wake-on-LAN request: %w", err)
	}
	// WOL is fire-and-forget, no response expected
	return "Wake-on-LAN packet sent", nil
}

// PowerAction sends a MeshCentral actiontype to one or more nodes.
// Common action types: 2=off, 3=reset, 4=sleep, 302=AMT on, 308=AMT off, 310=AMT reset.
// Returns success message or error.
func (c *Client) PowerAction(nodeIDs []string, powerAction int) (string, error) {
	err := c.sendNoResponse(map[string]interface{}{
		"action":     "poweraction",
		"nodeids":    nodeIDs,
		"actiontype": powerAction,
	})
	if err != nil {
		return "", fmt.Errorf("failed to send power action request: %w", err)
	}
	// Power action is fire-and-forget, no immediate response expected
	return "Power action sent", nil
}

// RunCommand executes a shell command on the device via the MeshAgent.
// Waits for command completion and returns the result.
func (c *Client) RunCommand(nodeID, command string) (map[string]interface{}, error) {
	pr, err := c.sendForResponse(map[string]interface{}{
		"action":    "runcommands",
		"nodeids":   []string{nodeID},
		"type":      0,
		"cmds":      command,
		"runAsUser": 0,
		"reply":     true,
	}, false)
	if err != nil {
		return nil, fmt.Errorf("failed to send runcommands request: %w", err)
	}

	res, err := c.waitForPending(context.Background(), pr, 30*time.Second)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// ListEvents returns MeshCentral audit events, optionally filtered by node or user.
func (c *Client) ListEvents(nodeID, userID string, limit int) ([]interface{}, error) {
	cmd := map[string]interface{}{"action": "events"}
	if nodeID != "" {
		cmd["nodeid"] = nodeID
	}
	if userID != "" {
		cmd["user"] = userID
	}
	if limit > 0 {
		cmd["limit"] = limit
	}

	pr, err := c.sendForResponse(cmd, false)
	if err != nil {
		return nil, fmt.Errorf("failed to send events request: %w", err)
	}
	res, err := c.waitForPending(context.Background(), pr, 10*time.Second)
	if err != nil {
		return nil, err
	}
	if events, ok := res["events"].([]interface{}); ok {
		return events, nil
	}
	return nil, fmt.Errorf("invalid response format for events")
}

// DeviceInfo collects device details from the same control actions MeshCtrl uses.
func (c *Client) DeviceInfo(nodeID string) (map[string]interface{}, error) {
	requests := []struct {
		key string
		cmd map[string]interface{}
	}{
		{key: "nodes", cmd: map[string]interface{}{"action": "nodes", "id": nodeID}},
		{key: "network", cmd: map[string]interface{}{"action": "getnetworkinfo", "nodeid": nodeID}},
		{key: "lastconnect", cmd: map[string]interface{}{"action": "lastconnect", "nodeid": nodeID}},
		{key: "sysinfo", cmd: map[string]interface{}{"action": "getsysinfo", "nodeid": nodeID, "nodeinfo": true}},
	}

	pending := make([]struct {
		key string
		pr  *pendingRequest
	}, 0, len(requests))
	for _, req := range requests {
		pr, err := c.sendForResponse(req.cmd, req.key == "nodes")
		if err != nil {
			return nil, fmt.Errorf("failed to send %s request: %w", req.cmd["action"], err)
		}
		pending = append(pending, struct {
			key string
			pr  *pendingRequest
		}{key: req.key, pr: pr})
	}

	result := make(map[string]interface{}, len(pending))
	for _, item := range pending {
		res, err := c.waitForPending(context.Background(), item.pr, 10*time.Second)
		if err != nil {
			return nil, err
		}
		result[item.key] = res
	}
	return result, nil
}

// Shell is intentionally unsupported until the MeshRelay tunnel protocol is implemented.
func (c *Client) Shell(nodeID, command string) (map[string]interface{}, error) {
	return nil, fmt.Errorf("unsupported: MeshCentral shell requires a meshrelay.ashx tunnel, which is not implemented")
}

func extractServerInfo(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return nil
	}
	if info, ok := data["serverinfo"].(map[string]interface{}); ok {
		return cloneMap(info)
	}
	info := make(map[string]interface{})
	for k, v := range data {
		if k == "action" || k == "reqid" || k == "responseid" {
			continue
		}
		info[k] = v
	}
	if len(info) == 0 {
		return nil
	}
	return info
}

func cloneMap(in map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
