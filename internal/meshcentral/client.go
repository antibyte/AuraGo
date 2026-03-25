package meshcentral

import (
	"bytes"
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

	// Channels for routing responses
	pendingReqs map[string]chan map[string]interface{}
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
func NewClient(urlStr, username, password, loginToken string, insecure bool) *Client {
	urlStr = strings.TrimSuffix(urlStr, "/")
	return &Client{
		url:         urlStr,
		username:    username,
		password:    password,
		loginToken:  loginToken,
		insecure:    insecure,
		pendingReqs: make(map[string]chan map[string]interface{}),
		done:        make(chan struct{}),
		logger:      slog.Default(),
	}
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
func (c *Client) Connect() error {
	// Build the WebSocket URL
	baseURL := strings.TrimSuffix(c.url, "/")
	u, err := url.Parse(baseURL + "/control.ashx")
	if err != nil {
		return err
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
			return fmt.Errorf("login failed: %w", err)
		}
		c.addAuthCookies(header)

	// Strategy 2: Username + Password
	case c.username != "" && c.password != "":
		c.log("[MeshCentral] Auth: Username/Password")
		if err := c.httpLogin(c.username); err != nil {
			return fmt.Errorf("login failed: %w", err)
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
			return fmt.Errorf("websocket dial failed: HTTP %d - %s", resp.StatusCode, string(body))
		}
		return fmt.Errorf("websocket dial failed: %w", err)
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
	if err := c.Send(map[string]interface{}{"action": "serverinfo"}); err != nil {
		return fmt.Errorf("failed to send serverinfo request: %w", err)
	}

	res, err := c.WaitForAction("serverinfo", 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to receive serverinfo: %w", err)
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
		return err
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
		return err
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
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

	return fmt.Errorf("no auth cookies received")
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
			_ = c.ws.Close()
			c.ws = nil
		}
		c.wsMu.Unlock()

		// Clean up pending request channels
		c.reqsMu.Lock()
		for _, ch := range c.pendingReqs {
			close(ch)
		}
		c.pendingReqs = make(map[string]chan map[string]interface{})
		c.reqsMu.Unlock()
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
			return
		}

		// Skip non-JSON messages
		if len(msg) < 2 || msg[0] != '{' {
			continue
		}

		var data map[string]interface{}
		if err := json.Unmarshal(msg, &data); err != nil {
			continue
		}

		action, _ := data["action"].(string)

		// Debug logging for troubleshooting
		if action != "" && action != "ping" && action != "pong" {
			c.log("[MeshCentral] Received message", "action", action)
		}

		c.reqsMu.RLock()
		ch := c.pendingReqs[action]
		if action == "event" {
			if eventType, ok := data["eventType"].(string); ok {
				c.log("[MeshCentral] Received event", "eventType", eventType)
				ch = c.pendingReqs["event_"+eventType]
			}
		}
		c.reqsMu.RUnlock()

		if ch != nil {
			select {
			case ch <- data:
			case <-c.done:
				return
			default:
			}
		}
	}
}

// Send sends a JSON command to the server.
func (c *Client) Send(cmd map[string]interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.wsMu.RLock()
	ws := c.ws
	c.wsMu.RUnlock()

	if ws == nil {
		return fmt.Errorf("not connected")
	}

	c.reqID++
	cmd["reqid"] = c.reqID

	return ws.WriteJSON(cmd)
}

// WaitForAction waits for a response with the given action string.
func (c *Client) WaitForAction(action string, timeout time.Duration) (map[string]interface{}, error) {
	c.reqsMu.Lock()
	if c.pendingReqs[action] == nil {
		c.pendingReqs[action] = make(chan map[string]interface{}, 5)
	}
	waitCh := c.pendingReqs[action]
	c.reqsMu.Unlock()

	select {
	case res := <-waitCh:
		return res, nil
	case <-time.After(timeout):
		// Clean up the channel to prevent memory leaks
		c.reqsMu.Lock()
		if c.pendingReqs[action] == waitCh {
			delete(c.pendingReqs, action)
		}
		c.reqsMu.Unlock()
		return nil, fmt.Errorf("timeout waiting for action %s", action)
	case <-c.done:
		return nil, fmt.Errorf("client disconnected")
	}
}

// WaitForEvent waits for an event response with the given eventType string.
func (c *Client) WaitForEvent(eventType string, timeout time.Duration) (map[string]interface{}, error) {
	return c.WaitForAction("event_"+eventType, timeout)
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
	if err := c.Send(map[string]interface{}{"action": "meshes"}); err != nil {
		return nil, err
	}

	res, err := c.WaitForAction("meshes", 10*time.Second)
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

	if err := c.Send(cmd); err != nil {
		return nil, err
	}

	res, err := c.WaitForAction("nodes", 10*time.Second)
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
	if err := c.Send(map[string]interface{}{
		"action":  "wakeonlan",
		"nodeids": nodeIDs,
	}); err != nil {
		return "", err
	}
	// WOL is fire-and-forget, no response expected
	return "Wake-on-LAN packet sent", nil
}

// PowerAction sends a power action (reset, sleep, poweroff) to a specific node.
// Power actions: 1=Sleep, 2=Hibernate, 3=PowerOff, 4=Reset
// Returns success message or error.
func (c *Client) PowerAction(nodeIDs []string, powerAction int) (string, error) {
	if err := c.Send(map[string]interface{}{
		"action":     "poweraction",
		"nodeids":    nodeIDs,
		"actiontype": powerAction,
	}); err != nil {
		return "", err
	}
	// Power action is fire-and-forget, no immediate response expected
	return "Power action sent", nil
}

// RunCommand executes a shell command on the device via the MeshAgent.
// Waits for command completion and returns the result.
func (c *Client) RunCommand(nodeID, command string) (map[string]interface{}, error) {
	if err := c.Send(map[string]interface{}{
		"action": "runcommand",
		"nodeid": nodeID,
		"run":    command,
	}); err != nil {
		return nil, err
	}

	// Wait for runcommand response (MeshCentral sends response with same action)
	res, err := c.WaitForAction("runcommand", 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("timeout waiting for command response: %w", err)
	}

	return res, nil
}

// Shell starts an interactive shell session on the device via the MeshAgent.
// This uses the WebSocket-based shell protocol which provides bidirectional
// communication. Returns the output or error.
func (c *Client) Shell(nodeID, command string) (map[string]interface{}, error) {
	if err := c.Send(map[string]interface{}{
		"action": "shell",
		"nodeid": nodeID,
		"data":   command + "\n",
	}); err != nil {
		return nil, err
	}

	// Wait for shell response
	res, err := c.WaitForAction("shell", 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("timeout waiting for shell response: %w", err)
	}

	return res, nil
}
