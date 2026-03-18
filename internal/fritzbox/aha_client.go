// Package fritzbox – AHA-HTTP client for Fritz!Box Smart Home.
// AHA-HTTP (AVM Home Automation) uses SID-authenticated GET requests to
// /webservices/homeautoswitch.lua for controlling smart home devices.
package fritzbox

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// AHAClient performs AHA-HTTP calls against the Fritz!Box smart home API.
// Authentication is done via a SID session (Lua-Login).
type AHAClient struct {
	baseURL    string
	sid        *SIDAuth
	httpClient *http.Client
}

// newAHAClient creates an AHA-HTTP client.
func newAHAClient(baseURL, username, password string, timeout time.Duration) *AHAClient {
	httpClient := &http.Client{Timeout: timeout}
	// newSIDAuth uses its own internal http.Client for the login handshake.
	sidAuth := newSIDAuth(baseURL, username, password, nil)
	return &AHAClient{
		baseURL:    baseURL,
		sid:        sidAuth,
		httpClient: httpClient,
	}
}

// Command executes an AHA command against the smarthome API.
// ain is the actor identification number (AIN) of the device (may be empty for list commands).
// cmd is the AVM command name, e.g. "getdevicelistinfos", "setswitchon", "getswitchstate".
// params are optional additional query parameters.
// Returns the raw response body (plain text or XML depending on command).
func (c *AHAClient) Command(ain, cmd string, params map[string]string) (string, error) {
	sid, err := c.sid.SID()
	if err != nil {
		return "", fmt.Errorf("aha: get sid: %w", err)
	}

	q := url.Values{}
	q.Set("sid", sid)
	q.Set("switchcmd", cmd)
	if ain != "" {
		q.Set("ain", ain)
	}
	for k, v := range params {
		q.Set(k, v)
	}

	reqURL := c.baseURL + "/webservices/homeautoswitch.lua?" + q.Encode()
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("aha: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("aha: request %s: %w", cmd, err)
	}
	defer resp.Body.Close()

	// SID may have expired – retry once after re-authentication.
	if resp.StatusCode == http.StatusForbidden {
		c.sid.Invalidate()
		sid, err = c.sid.SID()
		if err != nil {
			return "", fmt.Errorf("aha: re-auth: %w", err)
		}
		q.Set("sid", sid)
		reqURL = c.baseURL + "/webservices/homeautoswitch.lua?" + q.Encode()
		req, err = http.NewRequest(http.MethodGet, reqURL, nil)
		if err != nil {
			return "", fmt.Errorf("aha: build retry request: %w", err)
		}
		resp2, err2 := c.httpClient.Do(req)
		if err2 != nil {
			return "", fmt.Errorf("aha: retry %s: %w", cmd, err2)
		}
		defer resp2.Body.Close()
		resp = resp2
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("aha: HTTP %d for command %s", resp.StatusCode, cmd)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("aha: read response: %w", err)
	}
	return strings.TrimSpace(string(body)), nil
}
