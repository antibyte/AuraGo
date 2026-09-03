package localllm

import (
	"bytes"
	"context"
	"encoding/json"
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
		// A real model turn always takes priority over background cache
		// qualification. This also aborts a qualification that started while a
		// long-running tool was executing between two agent turns.
		m.cancelPromptCacheWarmForRequest()
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

		req, seed, stream, _, err := preparePromptCacheRequest(req)
		if err != nil {
			return nil, &UnavailableError{Code: errorCode(err), Err: err}
		}
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
		// Start may invalidate the previous runtime fingerprint and seed.
		m.rememberPromptSeed(seed)
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
		cacheEnabled := m.promptCacheReady(seed)
		if cacheEnabled {
			if cachedReq, cacheErr := enablePromptCacheRequest(req); cacheErr == nil {
				req = cachedReq
			} else {
				m.markPromptCacheDegraded("prompt_cache_request_failed")
				cacheEnabled = false
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
		observation := promptCacheObservationPlan{CacheEnabled: cacheEnabled}
		if applied != nil {
			observation.Generation = applied.Generation
		}
		if seed != nil {
			observation.Generation = seed.Generation
			observation.SeedFingerprint = seed.Fingerprint
		}
		resp.Body = &releaseBody{
			ReadCloser: newCacheObservingBody(resp.Body, stream, func(payload []byte, firstByte time.Duration, complete bool) {
				m.observePromptCacheResponse(observation, payload, stream, firstByte, complete)
				if complete && applied != nil {
					m.schedulePromptCacheQualification(*applied, seed)
				}
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
	body    io.ReadCloser
	stream  bool
	observe func([]byte, time.Duration, bool)
	started time.Time

	mu         sync.Mutex
	reads      sync.WaitGroup
	closing    bool
	complete   bool
	firstByte  time.Duration
	buffer     bytes.Buffer
	closeOnce  sync.Once
	finishOnce sync.Once
	closeErr   error
}

func newCacheObservingBody(body io.ReadCloser, stream bool, observe func([]byte, time.Duration, bool), started time.Time) io.ReadCloser {
	return &cacheObservingBody{body: body, stream: stream, observe: observe, started: started}
}

func (body *cacheObservingBody) Read(payload []byte) (int, error) {
	body.mu.Lock()
	if body.closing {
		body.mu.Unlock()
		return 0, io.ErrClosedPipe
	}
	body.reads.Add(1)
	body.mu.Unlock()
	n, err := body.body.Read(payload)
	body.mu.Lock()
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
		body.complete = !body.stream || promptCacheSSETerminal(body.buffer.Bytes())
	}
	body.mu.Unlock()
	body.reads.Done()
	if err == io.EOF {
		body.finish()
	}
	return n, err
}

func (body *cacheObservingBody) Close() error {
	body.closeOnce.Do(func() {
		body.mu.Lock()
		body.closing = true
		body.mu.Unlock()
		body.closeErr = body.body.Close()
		body.reads.Wait()
		body.finish()
	})
	return body.closeErr
}

func (body *cacheObservingBody) finish() {
	body.finishOnce.Do(func() {
		body.mu.Lock()
		payload := append([]byte(nil), body.buffer.Bytes()...)
		firstByte := body.firstByte
		complete := body.complete
		body.mu.Unlock()
		if body.observe != nil {
			body.observe(payload, firstByte, complete)
		}
	})
}

func promptCacheSSETerminal(payload []byte) bool {
	for _, line := range bytes.Split(payload, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		data := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if bytes.Equal(data, []byte("[DONE]")) {
			return true
		}
		var chunk struct {
			Usage json.RawMessage `json:"usage"`
		}
		if json.Unmarshal(data, &chunk) == nil && len(chunk.Usage) > 0 &&
			!bytes.Equal(bytes.TrimSpace(chunk.Usage), []byte("null")) {
			return true
		}
	}
	return false
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
