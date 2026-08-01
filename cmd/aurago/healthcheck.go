package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/config"
)

func printTsNetStateDir(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("config is unavailable")
	}
	stateDir := strings.TrimSpace(cfg.Tailscale.TsNet.StateDir)
	if stateDir == "" {
		stateDir = filepath.Join("data", "tsnet")
	}
	absolute, err := filepath.Abs(stateDir)
	if err != nil {
		return fmt.Errorf("resolve tsnet state directory: %w", err)
	}
	fmt.Fprintln(os.Stdout, filepath.Clean(absolute))
	return nil
}

func runCLIHealthcheck(cfg *config.Config, timeout time.Duration, requireTsNet bool) error {
	if cfg == nil {
		return fmt.Errorf("config is unavailable")
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	host := strings.TrimSpace(cfg.Server.Host)
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		host = "127.0.0.1"
	}
	host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
	scheme := "http"
	port := cfg.Server.Port
	if cfg.Server.HTTPS.Enabled {
		scheme = "https"
		port = cfg.Server.HTTPS.HTTPSPort
	}
	if port <= 0 {
		return fmt.Errorf("server port is not configured")
	}
	path := "/api/ready"
	if requireTsNet {
		path += "?require=tsnet"
	}
	endpoint := fmt.Sprintf("%s://%s%s", scheme, net.JoinHostPort(host, fmt.Sprint(port)), path)

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if scheme == "https" {
		// This request never leaves the configured local bind address. Local
		// self-signed deployments cannot be verified against public roots.
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true} // #nosec G402 -- loopback/local readiness probe only
	}
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			if lastErr == nil {
				lastErr = context.DeadlineExceeded
			}
			return fmt.Errorf("healthcheck timed out after %s: %w", timeout, lastErr)
		}
		requestTimeout := min(5*time.Second, remaining)
		requestCtx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint, nil)
		if err != nil {
			cancel()
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			var readiness struct {
				ErrorCode string `json:"error_code"`
			}
			if resp.Body != nil {
				_ = json.NewDecoder(io.LimitReader(resp.Body, 8<<10)).Decode(&readiness)
				_ = resp.Body.Close()
			}
			cancel()
			if resp.StatusCode == http.StatusOK {
				fmt.Fprintln(os.Stdout, "ready")
				return nil
			}
			if readiness.ErrorCode != "" {
				lastErr = fmt.Errorf("readiness returned HTTP %d (%s)", resp.StatusCode, readiness.ErrorCode)
			} else {
				lastErr = fmt.Errorf("readiness returned HTTP %d", resp.StatusCode)
			}
		} else {
			cancel()
			lastErr = err
		}
		remaining = time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("healthcheck timed out after %s: %w", timeout, lastErr)
		}
		time.Sleep(min(500*time.Millisecond, remaining))
	}
}
