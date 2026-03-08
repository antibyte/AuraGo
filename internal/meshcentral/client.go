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
	sessionID   string // login cookie if using user/pass
	authCookies []*http.Cookie
	reqID       int
	mu          sync.Mutex

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
	}
}

// Connect authenticates and opens the WebSocket connection.
func (c *Client) Connect() error {
	// 1. Authenticate to get session cookie
	needsLogin := false
	loginUser := c.username

	if strings.HasPrefix(c.loginToken, "~t:") {
		// It's a MeshCentral Login Token Username
		loginUser = c.loginToken
		needsLogin = true
	} else if c.loginToken == "" && c.username != "" {
		needsLogin = true
	}

	if needsLogin {
		if err := c.login(loginUser); err != nil {
			return fmt.Errorf("login failed: %v", err)
		}
	}

	// 2. Connect to WebSocket
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

	dialer := websocket.Dialer{
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: c.insecure},
		HandshakeTimeout: 10 * time.Second,
	}

	header := http.Header{}
	if c.loginToken != "" && !strings.HasPrefix(c.loginToken, "~t:") {
		// If it's a raw cookie token (not a MeshCentral login token username), use it as cookie
		cookie := &http.Cookie{Name: "meshcom", Value: c.loginToken}
		header.Set("Cookie", cookie.String())
	} else if len(c.authCookies) > 0 {
		cookieParts := make([]string, 0, len(c.authCookies))
		for _, ck := range c.authCookies {
			if ck != nil && ck.Name != "" && ck.Value != "" {
				cookieParts = append(cookieParts, ck.Name+"="+ck.Value)
			}
		}
		if len(cookieParts) > 0 {
			header.Set("Cookie", strings.Join(cookieParts, "; "))
		}
	} else if c.sessionID != "" {
		cookie := &http.Cookie{Name: "meshcom", Value: c.sessionID}
		header.Set("Cookie", cookie.String())
	}

	ws, resp, err := dialer.Dial(u.String(), header)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("websocket dial failed: HTTP %d - %s", resp.StatusCode, string(body))
		}
		return fmt.Errorf("websocket dial failed: %v", err)
	}

	c.ws = ws
	go c.readPump()

	// Wait for initial handshake message to confirm connection.
	// Some MeshCentral setups emit "userinfo" but no "serverinfo" event.
	if _, err = c.WaitForAction("userinfo", 5*time.Second); err != nil {
		if _, err2 := c.WaitForEvent("serverinfo", 2*time.Second); err2 != nil {
			return fmt.Errorf("failed to receive initial meshcentral handshake (userinfo/serverinfo): %v / %v", err, err2)
		}
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
	rootURL := strings.TrimSuffix(c.url, "/") + "/"

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
		if err := c.loginViaForm(rootURL, loginUser, c.password, httpClient); err == nil {
			return nil
		}
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		hint := ""
		if resp.StatusCode == http.StatusNotFound {
			hint = " — Hint: The MeshCentral base URL in config.yaml may be wrong. " +
				"Set it to the root of the MeshCentral installation. " +
				"If MeshCentral is on a sub-path, include it (e.g. https://host/meshcentral). " +
				"The client appends /login.ashx automatically."
		}
		return fmt.Errorf("login HTTP %d: %s%s", resp.StatusCode, string(body), hint)
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

func (c *Client) loginViaForm(rootURL, loginUser, password string, httpClient *http.Client) error {
	resp, err := httpClient.Get(rootURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	form := url.Values{}
	form.Set("action", "login")
	form.Set("username", loginUser)
	form.Set("password", password)
	form.Set("urlargs", "")

	postReq, err := http.NewRequest("POST", rootURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	postResp, err := httpClient.Do(postReq)
	if err != nil {
		return err
	}
	defer postResp.Body.Close()

	if cookies := postResp.Cookies(); len(cookies) > 0 {
		c.authCookies = append([]*http.Cookie(nil), cookies...)
		for _, cookie := range cookies {
			if cookie.Name == "meshcom" {
				c.sessionID = cookie.Value
				break
			}
		}
		return nil
	}

	body, _ := io.ReadAll(postResp.Body)
	return fmt.Errorf("form login HTTP %d: %s", postResp.StatusCode, string(body))
}

// Close gracefully closes the WebSocket.
func (c *Client) Close() {
	if c.ws != nil {
		_ = c.ws.Close()
		c.ws = nil
	}
}

// readPump reads messages from the WebSocket and routes them.
func (c *Client) readPump() {
	for {
		if c.ws == nil {
			break
		}
		_, msg, err := c.ws.ReadMessage()
		if err != nil {
			break
		}

		// Log everything briefly for debugging BEFORE JSON parse
		logMsg := string(msg)
		if len(logMsg) > 200 {
			logMsg = logMsg[:200] + "..."
		}
		fmt.Printf("[MeshCentral] RAW RX: %s\n", logMsg)

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
		if action != "" && c.pendingReqs[action] != nil {
			// Non-blocking send
			select {
			case c.pendingReqs[action] <- data:
			default:
			}
		} else if action == "event" {
			// Route events if someone is waiting for them
			if eventType, ok := data["eventType"].(string); ok && c.pendingReqs["event_"+eventType] != nil {
				select {
				case c.pendingReqs["event_"+eventType] <- data:
				default:
				}
			}
		}
		c.reqsMu.RUnlock()
	}
}

// Send sends a JSON command to the server.
func (c *Client) Send(cmd map[string]interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.ws == nil {
		return fmt.Errorf("not connected")
	}

	c.reqID++
	cmd["reqid"] = c.reqID

	return c.ws.WriteJSON(cmd)
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
		return nil, fmt.Errorf("timeout waiting for action %s", action)
	}
}

// WaitForEvent waits for an event response with the given eventType string.
func (c *Client) WaitForEvent(eventType string, timeout time.Duration) (map[string]interface{}, error) {
	return c.WaitForAction("event_"+eventType, timeout)
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
