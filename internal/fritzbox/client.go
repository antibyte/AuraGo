// Package fritzbox provides a high-level client facade for the Fritz!Box router.
// It combines the TR-064 SOAP client (Digest Auth) and the AHA-HTTP client
// (SID session auth) into a single unified entry point.
package fritzbox

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"aurago/internal/config"
)

// Client is the unified Fritz!Box client.
// Use NewClient to construct it; call Close when done to logout the SID session.
type Client struct {
	Cfg    config.Config // full config available for feature checks
	webURL string        // web interface base URL (port 80/443) for SID-authenticated requests
	tr     *TR064Client
	aha    *AHAClient
	sid    *SIDAuth // kept for explicit logout
}

// NewClient constructs a Client from the application config.
// The password must already be populated (loaded from vault by the caller).
func NewClient(cfg config.Config) (*Client, error) {
	fb := cfg.FritzBox
	if !fb.Enabled {
		return nil, fmt.Errorf("fritzbox: integration is not enabled")
	}
	if fb.Host == "" {
		return nil, fmt.Errorf("fritzbox: host is empty")
	}

	baseURL := buildBaseURL(fb.Host, fb.Port, fb.HTTPS)
	webURL := buildWebURL(fb.Host, fb.HTTPS, fb.WebPort) // SID/AHA use the web interface port, not TR-064 (49000)
	timeout := time.Duration(fb.Timeout) * time.Second

	tr := newTR064Client(baseURL, fb.Username, fb.Password, timeout, fb.InsecureSkipVerify)
	aha := newAHAClient(webURL, fb.Username, fb.Password, timeout, fb.InsecureSkipVerify)

	return &Client{
		Cfg:    cfg,
		webURL: webURL,
		tr:     tr,
		aha:    aha,
		sid:    aha.sid,
	}, nil
}

// Close logs out the AHA session and releases resources.
func (c *Client) Close() {
	if c.sid != nil {
		c.sid.Logout()
	}
}

// ──────────────────────────────────────────────
// TR-064 proxy – exported for service files
// ──────────────────────────────────────────────

// SOAP calls the TR-064 SOAP interface.
func (c *Client) SOAP(serviceType, controlURL, action string, args map[string]string) (map[string]string, error) {
	return c.tr.CallAction(serviceType, controlURL, action, args)
}

// ──────────────────────────────────────────────
// AHA-HTTP proxy – exported for service files
// ──────────────────────────────────────────────

// AHA calls the AHA-HTTP smart home API.
func (c *Client) AHA(ain, cmd string, params map[string]string) (string, error) {
	return c.aha.Command(ain, cmd, params)
}

// ──────────────────────────────────────────────
// Feature guards – runtime permission checks
// ──────────────────────────────────────────────

func (c *Client) SystemEnabled() bool     { return c.Cfg.FritzBox.System.Enabled }
func (c *Client) SystemReadOnly() bool    { return c.Cfg.FritzBox.System.ReadOnly }
func (c *Client) NetworkEnabled() bool    { return c.Cfg.FritzBox.Network.Enabled }
func (c *Client) NetworkReadOnly() bool   { return c.Cfg.FritzBox.Network.ReadOnly }
func (c *Client) TelephonyEnabled() bool  { return c.Cfg.FritzBox.Telephony.Enabled }
func (c *Client) TelephonyReadOnly() bool { return c.Cfg.FritzBox.Telephony.ReadOnly }
func (c *Client) SmartHomeEnabled() bool  { return c.Cfg.FritzBox.SmartHome.Enabled }
func (c *Client) SmartHomeReadOnly() bool { return c.Cfg.FritzBox.SmartHome.ReadOnly }
func (c *Client) StorageEnabled() bool    { return c.Cfg.FritzBox.Storage.Enabled }
func (c *Client) StorageReadOnly() bool   { return c.Cfg.FritzBox.Storage.ReadOnly }
func (c *Client) TVEnabled() bool         { return c.Cfg.FritzBox.TV.Enabled }
func (c *Client) TVReadOnly() bool        { return c.Cfg.FritzBox.TV.ReadOnly }

// ──────────────────────────────────────────────
// URL helper
// ──────────────────────────────────────────────

// buildBaseURL constructs the scheme+host base URL for Fritz!Box TR-064 calls.
func buildBaseURL(host string, port int, useHTTPS bool) string {
	scheme := "http"
	if useHTTPS {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, host, port)
}

// buildWebURL constructs the base URL for the Fritz!Box web interface (login_sid.lua, AHA-HTTP).
// webPort 0 means the scheme's default port (80 for HTTP, 443 for HTTPS).
func buildWebURL(host string, useHTTPS bool, webPort int) string {
	if useHTTPS {
		return fmt.Sprintf("https://%s", hostWithOptionalPort(host, webPort))
	}
	return fmt.Sprintf("http://%s", hostWithOptionalPort(host, webPort))
}

func hostWithOptionalPort(host string, port int) string {
	if port <= 0 {
		return host
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func newHTTPTransport(insecureSkipVerify bool) http.RoundTripper {
	base := http.DefaultTransport.(*http.Transport).Clone()
	if insecureSkipVerify {
		base.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // User-configured for trusted Fritz!Box LAN certificates.
	}
	return base
}
