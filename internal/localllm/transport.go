package localllm

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"
)

// RoundTripper starts AuraGo-Qwen on demand and holds the active request count
// until the response or SSE body is closed.
func (m *Manager) RoundTripper(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if err := m.acquireRequest(); err != nil {
			return nil, err
		}
		release := true
		defer func() {
			if release {
				m.release()
			}
		}()

		startCtx, cancel := context.WithTimeout(req.Context(), 180*time.Second)
		err := m.Start(startCtx)
		cancel()
		if err != nil {
			return nil, err
		}
		key, err := m.runtimeKey()
		if err != nil {
			return nil, &UnavailableError{Code: "runtime_key_unavailable", Err: err}
		}
		req = req.Clone(req.Context())
		req.Header = req.Header.Clone()
		req.Header.Set("Authorization", "Bearer "+key)
		resp, err := base.RoundTrip(req)
		if err != nil {
			if req.Context().Err() != nil {
				return nil, req.Context().Err()
			}
			return nil, &UnavailableError{Code: "local_transport_unavailable", Err: err}
		}
		if isLocalUnavailableStatus(resp.StatusCode) {
			_ = resp.Body.Close()
			return nil, &UnavailableError{Code: "local_runtime_unavailable"}
		}
		resp.Body = &releaseBody{ReadCloser: resp.Body, release: m.release}
		release = false
		return resp, nil
	})
}

func isLocalUnavailableStatus(status int) bool {
	return status == http.StatusBadGateway ||
		status == http.StatusServiceUnavailable ||
		status == http.StatusGatewayTimeout
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

type releaseBody struct {
	io.ReadCloser
	once    sync.Once
	release func()
}

func (body *releaseBody) Close() error {
	err := body.ReadCloser.Close()
	body.once.Do(body.release)
	return err
}

func (m *Manager) acquireRequest() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.shuttingDown {
		return &UnavailableError{Code: "local_llm_shutting_down"}
	}
	m.activeRequests++
	m.status.ActiveRequests = m.activeRequests
	m.status.IdleDeadline = nil
	return nil
}

func (m *Manager) acquire() {
	_ = m.acquireRequest()
}

func (m *Manager) release() {
	m.mu.Lock()
	if m.activeRequests > 0 {
		m.activeRequests--
	}
	m.status.ActiveRequests = m.activeRequests
	m.lastRelease = time.Now()
	if m.activeRequests == 0 {
		reconcile := false
		stopForDisable := false
		if m.pendingCfg != nil {
			m.cfg = *m.pendingCfg
			m.pendingCfg = nil
			m.status.PendingRestart = true
			reconcile = m.status.State == "running"
			stopForDisable = !m.cfg.Enabled
		}
		if m.shuttingDown {
			m.status.IdleDeadline = nil
		} else {
			deadline := m.lastRelease.Add(time.Duration(m.cfg.IdleTimeoutMinutes) * time.Minute)
			m.status.IdleDeadline = &deadline
		}
		m.mu.Unlock()
		if reconcile {
			go func() {
				if stopForDisable {
					_ = m.Stop(context.Background(), false)
				} else {
					_ = m.Recreate(context.Background())
				}
			}()
		}
		return
	}
	m.mu.Unlock()
}
