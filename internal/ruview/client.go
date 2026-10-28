// Package ruview owns the pinned RuView sensing integration. It never treats
// service readiness or a synthetic frame as evidence of a physical sensor.
package ruview

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

const Revision = "a71634ae8a606bba5463489450c69e8b42de114e"
const Version = "v3122"
const FirmwareVersion = "0.8.12"
const TokenKey = "ruview_api_token"
const ManagedTokenKey = "ruview_managed_token"
const maxResponseBytes = 8 << 20

// errInvalidReply marks a 2xx reply that is not JSON or exceeds the size limit,
// so callers can tell it from a failed request.
var errInvalidReply = errors.New("RuView response is incomplete or invalid")

// newClient pins every dial to a checked address of the saved origin. Proxy
// environment variables cannot turn an explicit LAN grant into a public relay.
func newClient(base *url.URL, allowPrivate bool) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.ResponseHeaderTimeout = 10 * time.Second
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		expectedPort := base.Port()
		if expectedPort == "" {
			expectedPort = "80"
			if base.Scheme == "https" {
				expectedPort = "443"
			}
		}
		if err != nil || !strings.EqualFold(host, base.Hostname()) || port != expectedPort {
			return nil, fmt.Errorf("RuView origin changed")
		}
		addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil || len(addrs) == 0 {
			return nil, fmt.Errorf("RuView hostname resolution failed")
		}
		for _, a := range addrs {
			if security.IsRestrictedNetworkIP(a.IP) && !(allowPrivate && (a.IP.IsPrivate() || a.IP.IsLoopback())) {
				return nil, fmt.Errorf("RuView address is blocked by network policy")
			}
		}
		return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, net.JoinHostPort(addrs[0].IP.String(), port))
	}
	return &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
}

func target(c *config.Config) (*url.URL, error) {
	if err := config.ValidateRuViewConfig(c.RuView, c.Runtime.IsDocker); err != nil {
		return nil, err
	}
	raw := c.RuView.URL
	if c.RuView.Mode != "external" {
		raw = fmt.Sprintf("http://127.0.0.1:%d", c.RuView.HTTPPort)
		if c.RuView.Mode == "docker" && c.Runtime.IsDocker {
			raw = "http://aurago-ruview:3000"
		}
	}
	return url.Parse(strings.TrimRight(raw, "/"))
}

func (m *Manager) client(c *config.Config) (*http.Client, *url.URL, string, error) {
	u, err := target(c)
	if err != nil {
		return nil, nil, "", err
	}
	key := TokenKey
	if c.RuView.Mode != "external" {
		key = ManagedTokenKey
	}
	token := ""
	if m.vault != nil {
		token, _ = m.vault.ReadSecret(key)
	}
	security.RegisterSensitive(token)
	return newClient(u, c.RuView.Mode != "external" || c.RuView.AllowPrivate), u, token, nil
}

func (m *Manager) request(ctx context.Context, c *config.Config, method, path string, body io.Reader) (json.RawMessage, error) {
	client, base, token, err := m.client(c)
	if err != nil {
		return nil, fail(CodeInvalidConfig, err.Error(), err)
	}
	defer client.CloseIdleConnections()
	base.Path = path
	req, err := http.NewRequestWithContext(ctx, method, base.String(), body)
	if err != nil {
		return nil, fail(CodeInvalidConfig, "RuView request: "+err.Error(), err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		var netErr net.Error
		switch {
		case ctx.Err() != nil:
			return nil, ctxError(ctx.Err())
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			return nil, ctxError(err)
		case errors.As(err, &netErr) && netErr.Timeout():
			return nil, &Error{Code: CodeTimeout, HTTP: HTTPStatus(CodeTimeout), msg: "RuView service did not answer in time"}
		}
		return nil, fail(CodeUnreachable, "RuView service could not be reached", nil)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		code := CodeUpstreamError
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			code = CodeUpstreamAuth
		}
		e := fail(code, fmt.Sprintf("RuView returned HTTP %d", resp.StatusCode), nil)
		e.Detail = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return nil, e
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctxError(ctx.Err())
		}
		return nil, &Error{Code: CodeContractUnsupported, HTTP: HTTPStatus(CodeContractUnsupported), msg: errInvalidReply.Error(), cause: errInvalidReply}
	}
	if len(data) > maxResponseBytes || !json.Valid(data) {
		return nil, &Error{Code: CodeContractUnsupported, HTTP: HTTPStatus(CodeContractUnsupported), msg: errInvalidReply.Error(), cause: errInvalidReply}
	}
	return json.RawMessage(security.Scrub(string(data))), nil
}
