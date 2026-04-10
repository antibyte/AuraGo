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
	action string        // original action name for debugging
	ch     chan response // buffered channel for the response
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

	// reqid-based request/response routing (replaces action-based routing)
	pendingReqs map[int]*pendingRequest // keyed by reqid
	reqsMu      sync.RWMutex

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
		url:         urlStr,
		username:    username,
		password:    password,
		loginToken:  loginToken,
		insecure:    insecure,
		pendingReqs: make(map[int]*pendingRequest),
		done:        make(chan struct{}),
		logger:      slog.Default(),
	}, nil
}

// SetLogger sets a custom logger for the client.
func (c *Client) SetLogger(l *slog.Logger) {
	c.logger = l
}

// log logs a message at Info level.
func (c *Client) log(msg string, args ...interface{}) {
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

	c.log("[MeshCentral] Connecting to %s", c.url)

	header := http.Header{}

	// Authentication Strategy Selection
	switch {
	// Strategy 1: Login Token + Password
	// The loginToken field contains the token name (with or without ~t: prefix)
	// The password field contains the actual token secret
	case c.loginToken != "" && c.password != "":
		c.log("[MeshCentral] Auth: Login Token")
		loginUser := c.loginToken
		if !strings.HasPrefix(loginUser, "~t:") {
			loginUser = "~t:" + loginUser
		}
		if err := c.httpLogin(loginUser); err != nil {
			return fmt.Errorf("login with token failed: %w", err)
		}
		c.addAuthCookies(header)

	// Strategy 2: Username + Password
	case c.username != "" && c.password != "":
		c.log("[MeshCentral] Auth: Username/Password")
		if err := c.httpLogin(c.username); err != nil {
			return fmt.Errorf("login with username/password failed: %w", err)
		}
		c.addAuthCookies(header)

	// Strategy 3: Unauthenticated (will likely fail)
	default:
		c.log("[MeshCentral] Auth: None (unauthenticated)")
	}

	// Dial WebSocket with authentication cookies
	dialer := websocket.Dialer{
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: c.insecure},
		HandshakeTimeout: 10 * time.Second,
	}

	ws, resp, err := dialer.Dial(u.String(), header)
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

	// Verify connection by requesting serverinfo
	reqid, err := c.Send(map[string]interface{}{"action": "serverinfo"})
	if err != nil {
		return fmt.Errorf("failed to send serverinfo request: %w", err)
	}

	res, err := c.WaitForReq(reqid, "serverinfo", 10*time.Second)
	if err != nil {
		return fmt.Errorf("serverinfo verification failed: %w", err)
	}

	c.log("[MeshCentral] Connected (server version: %v)", res["serverVersion"])
	return nil
}

// httpLogin performs HTTP POST to /login to obtain session cookies.
// For token auth: username should be "~t:tokenname", password is the token secret.
func (c *Client) httpLogin(loginUser string) error {
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
	if getResp, err := httpClient.Get(loginURL); err == nil {
		pageBytes, _ := io.ReadAll(io.LimitReader(getResp.Body, 32*1024))
		getResp.Body.Close()
		if m := regexp.MustCompile(`random="([^"]+)"`).FindSubmatch(pageBytes); len(m) == 2 {
			nonce = string(m[1])
		}
	} else {
		c.log("[MeshCentral] Warning: failed to fetch CSRF nonce: %v", err)
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

	req, err := http.NewRequest("POST", loginURL, bytes.NewBuffer(bodyData))
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
		c.reqsMu.Lock()
		for reqid, pr := range c.pendingReqs {
			select {
			case pr.ch <- response{err: fmt.Errorf("client closed")}:
			default:
			}
			close(pr.ch)
			delete(c.pendingReqs, reqid)
		}
		c.reqsMu.Unlock()

		c.log("[MeshCentral] Client closed")
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
			c.log("[MeshCentral] WebSocket read error: %v", err)
			return
		}

		// Skip non-JSON messages
		if len(msg) < 2 || msg[0] != '{' {
			continue
		}

		var data map[string]interface{}
		if err := json.Unmarshal(msg, &data); err != nil {
			c.log("[MeshCentral] Failed to parse JSON message: %v", err)
			continue
		}

		// Route by reqid if present (ensures correct request/response matching)
		var reqid int
		if rid, ok := data["reqid"].(float64); ok {
			reqid = int(rid)
		}

		action, _ := data["action"].(string)

		c.reqsMu.Lock()
		var delivered bool
		if reqid > 0 {
			// Primary routing: by reqid (exact match, race-free)
			if pr, ok := c.pendingReqs[reqid]; ok {
				select {
				case pr.ch <- response{data: data}:
					delivered = true
					c.log("[MeshCentral] Delivered response to reqid %d (action=%s)", reqid, action)
				case <-c.done:
					c.reqsMu.Unlock()
					return
				default:
					// Channel full, skip (shouldn't happen with buffered channels)
				}
			}
		}
		c.reqsMu.Unlock()

		// Fallback: route events by eventType if no reqid match
		if !delivered && action == "event" {
			if eventType, ok := data["eventType"].(string); ok {
				c.log("[MeshCentral] Received event", "eventType", eventType)
				// Note: Events don't have reqid, so we can't route them properly
				// without additional event subscription mechanism
			}
		}

		// Debug logging for unhandled non-event messages
		if !delivered && action != "" && action != "ping" && action != "pong" && action != "serverinfo" {
			c.log("[MeshCentral] Unmatched message", "action", action, "reqid", reqid)
		}
	}
}

// Send sends a JSON command to the server and registers it for response routing.
// Returns the assigned reqid for correlation with the response.
func (c *Client) Send(cmd map[string]interface{}) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.wsMu.RLock()
	ws := c.ws
	c.wsMu.RUnlock()

	if ws == nil {
		return 0, fmt.Errorf("not connected")
	}

	c.reqID++
	reqid := c.reqID
	cmd["reqid"] = reqid

	return reqid, ws.WriteJSON(cmd)
}

// WaitForReq waits for a response with the given reqid.
// This is the low-level primitive that readPump delivers to via reqid routing.
func (c *Client) WaitForReq(reqid int, action string, timeout time.Duration) (map[string]interface{}, error) {
	c.reqsMu.Lock()
	if c.pendingReqs[reqid] == nil {
		c.pendingReqs[reqid] = &pendingRequest{
			action: action,
			ch:     make(chan response, 1),
		}
	}
	pr := c.pendingReqs[reqid]
	c.reqsMu.Unlock()

	select {
	case res := <-pr.ch:
		if res.err != nil {
			return nil, res.err
		}
		return res.data, nil
	case <-time.After(timeout):
		c.reqsMu.Lock()
		if c.pendingReqs[reqid] == pr {
			delete(c.pendingReqs, reqid)
		}
		c.reqsMu.Unlock()
		return nil, fmt.Errorf("timeout waiting for reqid %d (action=%s)", reqid, action)
	case <-c.done:
		return nil, fmt.Errorf("client disconnected")
	}
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

// ListDeviceGroups requests the meshes/groups list.
func (c *Client) ListDeviceGroups() ([]interface{}, error) {
	reqid, err := c.Send(map[string]interface{}{"action": "meshes"})
	if err != nil {
		return nil, fmt.Errorf("failed to send meshes request: %w", err)
	}

	res, err := c.WaitForReq(reqid, "meshes", 10*time.Second)
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

	reqid, err := c.Send(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to send nodes request: %w", err)
	}

	res, err := c.WaitForReq(reqid, "nodes", 10*time.Second)
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
	_, err := c.Send(map[string]interface{}{
		"action":  "wakeonlan",
		"nodeids": nodeIDs,
	})
	if err != nil {
		return "", fmt.Errorf("failed to send wake-on-LAN request: %w", err)
	}
	// WOL is fire-and-forget, no response expected
	return "Wake-on-LAN packet sent", nil
}

// PowerAction sends a power action (reset, sleep, poweroff) to a specific node.
// Power actions: 1=Sleep, 2=Hibernate, 3=PowerOff, 4=Reset
// Returns success message or error.
func (c *Client) PowerAction(nodeIDs []string, powerAction int) (string, error) {
	_, err := c.Send(map[string]interface{}{
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
	reqid, err := c.Send(map[string]interface{}{
		"action": "runcommand",
		"nodeid": nodeID,
		"run":    command,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send runcommand request: %w", err)
	}

	// Wait for runcommand response using reqid-based routing
	res, err := c.WaitForReq(reqid, "runcommand", 30*time.Second)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// Shell starts an interactive shell session on the device via the MeshAgent.
// This uses the WebSocket-based shell protocol which provides bidirectional
// communication. Returns the output or error.
func (c *Client) Shell(nodeID, command string) (map[string]interface{}, error) {
	reqid, err := c.Send(map[string]interface{}{
		"action": "shell",
		"nodeid": nodeID,
		"data":   command + "\n",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send shell request: %w", err)
	}

	// Wait for shell response using reqid-based routing
	res, err := c.WaitForReq(reqid, "shell", 30*time.Second)
	if err != nil {
		return nil, err
	}

	return res, nil
}
