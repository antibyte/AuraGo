package localllm

import (
	"bytes"
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
		releaseSlot, err := m.acquirePromptSlot(req.Context())
		if err != nil {
			return nil, err
		}
		slotOwned := true
		defer func() {
			if slotOwned {
				releaseSlot()
			}
		}()

		req, seed, stream, err := preparePromptCacheRequest(req)
		if err != nil {
			return nil, &UnavailableError{Code: errorCode(err), Err: err}
		}
		m.rememberPromptSeed(seed)
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
		err = m.Start(startCtx)
		cancel()
		if err != nil {
			return nil, err
		}
		key, err := m.runtimeKey()
		if err != nil {
			return nil, &UnavailableError{Code: "runtime_key_unavailable", Err: err}
		}
		m.mu.Lock()
		var applied *runtimePlan
		if m.appliedPlan != nil {
			copyPlan := *m.appliedPlan
			applied = &copyPlan
		}
		m.mu.Unlock()
		if applied != nil {
			m.ensurePromptCacheWarm(req.Context(), *applied, key)
		}
		if m.promptCacheReady(seed) {
			if cachedReq, cacheErr := enablePromptCacheRequest(req); cacheErr == nil {
				req = cachedReq
			} else {
				m.mu.Lock()
				m.status.PromptCache.State = "degraded"
				m.status.PromptCache.ErrorCode = "prompt_cache_request_failed"
				m.mu.Unlock()
			}
		}
		req = req.Clone(req.Context())
		req.Header = req.Header.Clone()
		req.Header.Set("Authorization", "Bearer "+key)
		started := time.Now()
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
		resp.Body = &releaseBody{
			ReadCloser: newCacheObservingBody(resp.Body, func(payload []byte, firstByte time.Duration) {
				m.observePromptCacheResponse(payload, stream, firstByte)
			}, started),
			release: func() {
				m.release()
				releaseSlot()
			},
		}
		release = false
		slotOwned = false
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

type cacheObservingBody struct {
	body      io.ReadCloser
	observe   func([]byte, time.Duration)
	started   time.Time
	firstByte time.Duration
	buffer    bytes.Buffer
	once      sync.Once
}

func newCacheObservingBody(body io.ReadCloser, observe func([]byte, time.Duration), started time.Time) io.ReadCloser {
	return &cacheObservingBody{body: body, observe: observe, started: started}
}

func (body *cacheObservingBody) Read(payload []byte) (int, error) {
	n, err := body.body.Read(payload)
	if n > 0 {
		if body.firstByte == 0 {
			body.firstByte = time.Since(body.started)
		}
		if body.buffer.Len() < maxLocalRequestBytes {
			remaining := maxLocalRequestBytes - body.buffer.Len()
			if n < remaining {
				remaining = n
			}
			_, _ = body.buffer.Write(payload[:remaining])
		}
	}
	if err == io.EOF {
		body.finish()
	}
	return n, err
}

func (body *cacheObservingBody) Close() error {
	body.finish()
	return body.body.Close()
}

func (body *cacheObservingBody) finish() {
	body.once.Do(func() {
		if body.observe != nil {
			body.observe(body.buffer.Bytes(), body.firstByte)
		}
	})
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
