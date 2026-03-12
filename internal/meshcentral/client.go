package meshcentral

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client handles communication with a MeshCentral server.
type Client struct {
	url        string
	username   string
	password   string // Used if login_token is empty
	loginToken string // Optional token from vault
	insecure   bool

	ws          *websocket.Conn
	wsMu        sync.RWMutex // Protects ws for concurrent access
	sessionID   string       // login cookie if using user/pass
	authCookies []*http.Cookie
	reqID       int
	mu          sync.Mutex
	done        chan struct{} // Signals goroutines to stop

	// Channels for routing responses
	pendingReqs map[string]chan map[string]interface{}
	reqsMu      sync.RWMutex
}

// NewClient creates a new MeshCentral client.
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
	}
}

// Connect authenticates and opens the WebSocket connection.
//
// Authentication strategy (in priority order):
//  1. Login Token  — passed as ?auth=<token> on the WebSocket URL.
//     This is the official MeshCentral API mechanism; no HTTP round-trip needed.
//  2. Username + Password — HTTP POST to login.ashx to obtain a session cookie.
//     Falls back to ?user=<u>&pass=<p> on the WS URL if login.ashx is unreachable
//     (e.g. reverse-proxy that only exposes /control.ashx).
//  3. Raw session token (loginToken set, no ~t: prefix) — sent as meshcom cookie.
func (c *Client) Connect() error {
	// Build the WebSocket URL: scheme normalisation + /control.ashx
	u, err := url.Parse(c.url + "/control.ashx")
	if err != nil {
		return err
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	}

	header := http.Header{}

	switch {
	case c.loginToken != "":
		// ── Strategy 1: Login Token ──────────────────────────────────────────
		// MeshCentral login tokens are passed as ?auth=<token> on the WS URL.
		// This is the primary, recommended API authentication method and works
		// regardless of whether login.ashx is accessible.
		q := u.Query()
		q.Set("auth", c.loginToken)
		u.RawQuery = q.Encode()

	case c.username != "":
		// ── Strategy 2: Username + Password ─────────────────────────────────
		// Try HTTP login first so we get a proper session cookie.
		if err := c.login(c.username); err == nil {
			// HTTP login succeeded; build cookie header from obtained cookies.
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
		} else {
			// HTTP login unavailable (login.ashx 404 / reverse-proxy) — fall back
			// to WS query-param auth: ?user=<u>&pass=<p>
			q := u.Query()
			q.Set("user", c.username)
			q.Set("pass", c.password)
			u.RawQuery = q.Encode()
		}

	default:
		// No credentials configured — attempt unauthenticated connection.
	}

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
		return fmt.Errorf("websocket dial failed: %v", err)
	}

	// Pre-register BOTH possible handshake channels BEFORE readPump starts.
	// MeshCentral sends either action="userinfo" or action="event" /
	// eventType="serverinfo" immediately upon WebSocket connect.  If we only
	// register one channel at a time (sequentially) the other message arrives
	// while no consumer is waiting and the non-blocking send in readPump
	// silently drops it, causing a guaranteed timeout on the second wait.
	userinfoC := make(chan map[string]interface{}, 5)
	serverinfoC := make(chan map[string]interface{}, 5)
	c.reqsMu.Lock()
	c.pendingReqs["userinfo"] = userinfoC
	c.pendingReqs["event_serverinfo"] = serverinfoC
	c.reqsMu.Unlock()

	c.wsMu.Lock()
	c.ws = ws
	c.wsMu.Unlock()
	
	go c.readPump()
	go c.pingPump()

	// Wait for whichever handshake message the server sends first.
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	select {
	case <-userinfoC:
		// connected — server sent userinfo
	case <-serverinfoC:
		// connected — server sent event_serverinfo
	case <-timer.C:
		return fmt.Errorf("failed to receive initial meshcentral handshake: timeout waiting for userinfo or serverinfo")
	}

	return nil
}

// login performs the HTTP POST to /login.ashx to get the auth cookie.
// MeshCentral requires a per-session CSRF nonce (embedded in the login page
// as  random="<base64>"  in the JS) to be replayed as "rn" in the POST body.
func (c *Client) login(loginUser string) error {
	u, err := url.Parse(c.url + "/login.ashx")
	if err != nil {
		return err
	}
	switch u.Scheme {
	case "ws":
		u.Scheme = "http"
	case "wss":
		u.Scheme = "https"
	}
	loginURL := u.String()

	// Build rootURL with HTTP(S) scheme so loginViaForm can make HTTP requests.
	// Without the scheme fix, wss:// URLs would cause an "unsupported protocol scheme" error.
	rootU, _ := url.Parse(strings.TrimSuffix(c.url, "/") + "/")
	switch rootU.Scheme {
	case "ws":
		rootU.Scheme = "http"
	case "wss":
		rootU.Scheme = "https"
	}
	rootURL := rootU.String()

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: c.insecure},
		},
		Timeout: 10 * time.Second,
	}

	// Step 1: GET the login page to extract the CSRF nonce.
	var nonce string
	getResp, err := httpClient.Get(loginURL)
	if err == nil {
		defer getResp.Body.Close()
		pageBytes, _ := io.ReadAll(getResp.Body)
		// MeshCentral embeds the nonce as: random="<base64value>"
		if m := regexp.MustCompile(`random="([^"]+)"`).FindSubmatch(pageBytes); len(m) == 2 {
			nonce = string(m[1])
		}
	}

	// Step 2: POST credentials including the nonce.
	payload := map[string]interface{}{
		"u":  loginUser,
		"p":  c.password,
		"a":  1, // Web/Agent login
		"rn": nonce,
	}
	bodyData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", loginURL, bytes.NewBuffer(bodyData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		if formErr := c.loginViaForm(rootURL, loginUser, c.password, httpClient); formErr == nil {
			return nil
		} else {
			// Return the (human-readable) discovery error, not the raw HTML 404 body.
			return formErr
		}
	}

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

	return fmt.Errorf("no auth cookies found in login response")
}

// loginViaForm is called when the primary POST to /login.ashx returns 404.
// It probes multiple candidate base URLs:
//  1. The redirect-discovered path from rootURL (catches reverse-proxy setups that
//     redirect the root to the real MeshCentral install path).
//  2. Common sub-paths: /meshcentral/, /mesh/, /mc/
//
// On success it writes auth cookies and updates c.url so the subsequent WebSocket
// dial uses the correct base path automatically.
func (c *Client) loginViaForm(rootURL, loginUser, password string, httpClient *http.Client) error {
	parsedRoot, err := url.Parse(rootURL)
	if err != nil {
		return fmt.Errorf("invalid root URL: %w", err)
	}

	// Build the list of candidate base URLs (all with trailing slash).
	var candidates []string

	// 1. Follow redirect from configured root to auto-discover the actual install path.
	if getResp, getErr := httpClient.Get(rootURL); getErr == nil {
		io.Copy(io.Discard, io.LimitReader(getResp.Body, 4096)) //nolint:errcheck
		getResp.Body.Close()
		finalU := getResp.Request.URL
		basePath := finalU.Path
		if idx := strings.LastIndex(basePath, "/"); idx >= 0 {
			basePath = basePath[:idx+1]
		} else {
			basePath = "/"
		}
		candidates = append(candidates, fmt.Sprintf("%s://%s%s", finalU.Scheme, finalU.Host, basePath))
	}

	// 2. Common MeshCentral sub-paths relative to the configured host.
	hostBase := fmt.Sprintf("%s://%s", parsedRoot.Scheme, parsedRoot.Host)
	for _, sub := range []string{"/meshcentral/", "/mesh/", "/mc/"} {
		candidates = append(candidates, hostBase+sub)
	}

	var triedURLs []string
	var lastErr error

	for _, base := range candidates {
		base = strings.TrimSuffix(base, "/") + "/"
		loginURL := base + "login.ashx"
		triedURLs = append(triedURLs, loginURL)

		// GET login page to extract CSRF nonce; skip this candidate if it returns non-200.
		var nonce string
		if getResp, getErr := httpClient.Get(loginURL); getErr == nil {
			pageBytes, _ := io.ReadAll(io.LimitReader(getResp.Body, 32*1024))
			getResp.Body.Close()
			if getResp.StatusCode != http.StatusOK {
				lastErr = fmt.Errorf("HTTP %d at %s", getResp.StatusCode, loginURL)
				continue
			}
			if m := regexp.MustCompile(`random="([^"]+)"`).FindSubmatch(pageBytes); len(m) == 2 {
				nonce = string(m[1])
			}
		} else {
			lastErr = fmt.Errorf("GET %s: %v", loginURL, getErr)
			continue
		}

		// POST credentials.
		payload := map[string]interface{}{"u": loginUser, "p": password, "a": 1, "rn": nonce}
		bodyData, _ := json.Marshal(payload)
		postReq, reqErr := http.NewRequest("POST", loginURL, bytes.NewBuffer(bodyData))
		if reqErr != nil {
			lastErr = reqErr
			continue
		}
		postReq.Header.Set("Content-Type", "application/json")

		postResp, postErr := httpClient.Do(postReq)
		if postErr != nil {
			lastErr = fmt.Errorf("POST %s: %v", loginURL, postErr)
			continue
		}
		io.Copy(io.Discard, io.LimitReader(postResp.Body, 4096)) //nolint:errcheck
		postResp.Body.Close()

		if postResp.StatusCode == http.StatusOK {
			if cookies := postResp.Cookies(); len(cookies) > 0 {
				c.authCookies = append([]*http.Cookie(nil), cookies...)
				for _, ck := range cookies {
					if ck.Name == "meshcom" {
						c.sessionID = ck.Value
						break
					}
				}
				// Update c.url to the discovered base so the WebSocket also uses the correct path.
				if parsedBase, pErr := url.Parse(strings.TrimSuffix(base, "/")); pErr == nil {
					switch parsedBase.Scheme {
					case "http":
						parsedBase.Scheme = "ws"
					case "https":
						parsedBase.Scheme = "wss"
					}
					c.url = parsedBase.String()
				}
				return nil
			}
			lastErr = fmt.Errorf("no auth cookies in response from %s", loginURL)
		} else {
			lastErr = fmt.Errorf("HTTP %d at %s", postResp.StatusCode, loginURL)
		}
	}

	tried := strings.Join(triedURLs, ", ")
	if lastErr != nil {
		return fmt.Errorf("login.ashx not found at the configured URL or common sub-paths (tried: %s). "+
			"Last error: %v — Set the MeshCentral URL in config to the exact sub-path "+
			"(e.g. https://host/meshcentral)", tried, lastErr)
	}
	return fmt.Errorf("no candidate login.ashx endpoint found (tried: %s)", tried)
}

// Close gracefully closes the WebSocket.
func (c *Client) Close() {
	close(c.done)
	
	c.wsMu.Lock()
	if c.ws != nil {
		_ = c.ws.Close()
		c.ws = nil
	}
	c.wsMu.Unlock()
	
	// Clean up pending request channels to prevent memory leaks
	c.reqsMu.Lock()
	for _, ch := range c.pendingReqs {
		close(ch)
	}
	c.pendingReqs = make(map[string]chan map[string]interface{})
	c.reqsMu.Unlock()
}

// readPump reads messages from the WebSocket and routes them.
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

		// MeshCentral sometimes sends purely string payloads or empty objects
		if len(msg) < 2 || msg[0] != '{' {
			continue
		}

		var data map[string]interface{}
		if err := json.Unmarshal(msg, &data); err != nil {
			continue
		}

		action, _ := data["action"].(string)

		c.reqsMu.RLock()
		ch := c.pendingReqs[action]
		if action == "event" {
			if eventType, ok := data["eventType"].(string); ok {
				ch = c.pendingReqs["event_"+eventType]
			}
		}
		c.reqsMu.RUnlock()
		
		if ch != nil {
			// Non-blocking send with done channel check
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
		return nil, fmt.Errorf("client disconnected while waiting for action %s", action)
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
			
			if err := ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// --- High Level API Methods --- //

// ListDeviceGroups requests the meshes/groups list.
func (c *Client) ListDeviceGroups() ([]interface{}, error) {
	err := c.Send(map[string]interface{}{
		"action": "meshes",
	})
	if err != nil {
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
	cmd := map[string]interface{}{
		"action": "nodes",
	}
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
		// MeshCentral sometimes packages results under a meshid key if queried specific
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
func (c *Client) WakeOnLan(nodeIDs []string) error {
	return c.Send(map[string]interface{}{
		"action":  "wakeonlan",
		"nodeids": nodeIDs,
	})
}

// PowerAction sends a power action (reset, sleep, poweroff) to a specific node.
func (c *Client) PowerAction(nodeIDs []string, powerAction int) error {
	// Power actions: 1=Sleep, 2=Hibernate, 3=PowerOff, 4=Reset
	return c.Send(map[string]interface{}{
		"action":     "poweraction",
		"nodeids":    nodeIDs,
		"actiontype": powerAction,
	})
}

// RunCommand attempts to execute a shell command on the device via the MeshAgent.
func (c *Client) RunCommand(nodeID, command string) error {
	// Send to MeshAgent runcommand
	return c.Send(map[string]interface{}{
		"action": "runcommand",
		"nodeid": nodeID,
		"run":    command,
	})
}
