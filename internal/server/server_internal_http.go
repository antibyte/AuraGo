package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurago/internal/config"
)

func DedicatedInternalLoopbackPort(cfg *config.Config) int {
	if cfg == nil {
		return 0
	}
	if cfg.CloudflareTunnel.LoopbackPort > 0 {
		return cfg.CloudflareTunnel.LoopbackPort
	}
	if !cfg.Server.HTTPS.Enabled {
		return 0
	}
	port := cfg.Server.Port
	if port <= 0 || port == cfg.Server.HTTPS.HTTPSPort {
		return 0
	}
	if cfg.Server.HTTPS.HTTPPort > 0 && port == cfg.Server.HTTPS.HTTPPort {
		return 0
	}
	return port
}

// InternalAPIURL returns the base URL for internal (loopback) API calls.
// HTTPS instances prefer a dedicated 127.0.0.1 plain-HTTP listener so scheduled
// automation is not coupled to the public TLS listener. If no dedicated
// listener is configured, the function falls back to the active public scheme.
// This is the single source of truth for all internal API URL construction.
func InternalAPIURL(cfg *config.Config) string {
	if port := DedicatedInternalLoopbackPort(cfg); port > 0 {
		return fmt.Sprintf("http://127.0.0.1:%d", port)
	}

	scheme := "http"
	port := cfg.Server.Port
	if cfg.Server.HTTPS.Enabled {
		scheme = "https"
		if cfg.Server.HTTPS.HTTPSPort > 0 {
			port = cfg.Server.HTTPS.HTTPSPort
		} else {
			port = 443
		}
	}
	return fmt.Sprintf("%s://127.0.0.1:%d", scheme, port)
}

// NewInternalHTTPClient returns an http.Client configured for internal loopback
// API calls. It skips TLS verification for fallback HTTPS self-calls because
// InternalAPIURL always resolves to 127.0.0.1 and the server may use a
// self-signed certificate.
func NewInternalHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // SECURE: Only for 127.0.0.1 internal API
			},
			ForceAttemptHTTP2: false,
			DisableKeepAlives: true,
		},
	}
}

// DoInternalRequestWithStartupRetry retries short-lived loopback connection
// refusals during server startup. Mission triggers can fire before the internal
// listener has finished binding; treating that race as a hard mission failure
// makes otherwise valid startup missions fail spuriously.
func DoInternalRequestWithStartupRetry(ctx context.Context, client *http.Client, method, rawURL string, body []byte, headers http.Header, maxWait time.Duration) (*http.Response, error) {
	if client == nil {
		client = http.DefaultClient
	}
	deadline := time.Now().Add(maxWait)
	for {
		req, err := http.NewRequestWithContext(ctx, method, rawURL, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header = headers.Clone()
		resp, err := client.Do(req)
		if err == nil {
			return resp, nil
		}
		if maxWait <= 0 || !isLoopbackConnectionRefused(rawURL, err) || time.Now().After(deadline) {
			return nil, err
		}
		sleep := 500 * time.Millisecond
		if remaining := time.Until(deadline); remaining < sleep {
			sleep = remaining
		}
		if sleep <= 0 {
			return nil, err
		}
		timer := time.NewTimer(sleep)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func isLoopbackConnectionRefused(rawURL string, err error) bool {
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "connection refused") {
		return false
	}
	parsed, parseErr := url.Parse(rawURL)
	if parseErr != nil {
		return false
	}
	host := parsed.Hostname()
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// i18nStore holds the parsed translations from ui/lang/ keyed by language code.
// Each value is the raw JSON string for that language, ready for template injection.
// Deprecated: These variables are kept for backward compatibility with tests.
// Use the i18n package functions directly instead.
