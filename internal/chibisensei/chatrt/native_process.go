package chatrt

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/llamacpp"
	"aurago/internal/security"
)

// Health polling; variables so tests can shorten them.
var (
	healthInterval = 250 * time.Millisecond
	healthLimit    = 3 * time.Minute
	stopWait       = 10 * time.Second
)

// procHandle is the OS-specific handle of a started child: done closes when it exited, kill ends it.
type procHandle struct {
	done chan struct{}
	kill func() error
	pid  int
	err  error // exit error; valid after done
}

// nativeProcess is one running llama-server child.
type nativeProcess struct {
	handle  *procHandle
	baseURL string
	key     string
	client  *http.Client
	logs    *logBuffer
	backend string
	gpu     bool
	once    sync.Once
	stopErr error
}

// startServer starts one child on a fresh loopback port and waits until it is healthy. bindFailure is true when
// the child died because its port was taken, so the caller may retry once with a new port.
func startServer(ctx context.Context, server string, roots []string, key string, spec LaunchSpec) (proc *nativeProcess, bindFailure bool, err error) {
	port, err := llamacpp.LoopbackPort()
	if err != nil {
		return nil, false, err
	}
	cmd := newServerCommand(server, roots, key, serverArgs(spec, port))
	logs := newLogBuffer(64 << 10)
	cmd.Stdout, cmd.Stderr = logs, logs
	handle, err := startProcess(cmd)
	if err != nil {
		return nil, false, fmt.Errorf("start llama-server: %s", security.Scrub(err.Error()))
	}
	p := &nativeProcess{
		handle:  handle,
		baseURL: "http://127.0.0.1:" + strconv.Itoa(port),
		key:     key,
		client:  newLoopbackClient(),
		logs:    logs,
		backend: spec.Backend,
	}
	if err := waitHealthy(ctx, handle.done, p.baseURL, logs.String); err != nil {
		_ = p.Stop()
		lower := strings.ToLower(logs.String())
		exited := errors.Is(err, errProcessExited)
		return nil, exited && (strings.Contains(lower, "bind") || strings.Contains(lower, "address already in use")), err
	}
	p.gpu = spec.Backend != "cpu" && llamacpp.GPUVerified(spec.Backend, logs.String())
	return p, false, nil
}

// waitHealthy polls the public /health endpoint (no key): 200 is ready, 503 means the model still loads. It
// ends when done closes, ctx ends or healthLimit passes. Errors carry the scrubbed log tail, never the key.
func waitHealthy(ctx context.Context, done <-chan struct{}, baseURL string, logTail func() string) error {
	client := newLoopbackClient()
	defer client.CloseIdleConnections()
	deadline := time.NewTimer(healthLimit)
	defer deadline.Stop()
	tick := time.NewTicker(healthInterval)
	defer tick.Stop()
	tail := func() string {
		if logTail == nil {
			return ""
		}
		return security.Scrub(truncateTail(logTail(), 2000))
	}
	for {
		select {
		case <-done:
			return fmt.Errorf("llama-server exited: %w: %s", errProcessExited, tail())
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("llama-server did not become healthy within %s: %s", healthLimit, tail())
		case <-tick.C:
			reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, baseURL+"/health", nil)
			if err != nil {
				cancel()
				return fmt.Errorf("build health request: %w", err)
			}
			resp, err := client.Do(req)
			cancel()
			if err != nil {
				continue
			}
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
	}
}

func truncateTail(s string, limit int) string {
	if len(s) > limit {
		return "..." + s[len(s)-limit:]
	}
	return s
}

func (p *nativeProcess) Chat(ctx context.Context, req ChatRequest, onDelta func(string)) (ChatStats, error) {
	select {
	case <-p.handle.done:
		return ChatStats{}, errProcessExited
	default:
	}
	stats, err := streamChat(ctx, p.client, p.baseURL, p.key, req, onDelta)
	if err != nil {
		select {
		case <-p.handle.done:
			return ChatStats{}, fmt.Errorf("%w: %v", errProcessExited, err)
		default:
		}
	}
	return stats, err
}

func (p *nativeProcess) GPUVerified() bool     { return p.gpu }
func (p *nativeProcess) Done() <-chan struct{} { return p.handle.done }

// Stop kills the child by its handle and waits for it. It is idempotent.
func (p *nativeProcess) Stop() error {
	p.once.Do(func() {
		_ = p.handle.kill()
		select {
		case <-p.handle.done:
		case <-time.After(stopWait):
			p.stopErr = fmt.Errorf("llama-server did not exit within %s", stopWait)
		}
		p.client.CloseIdleConnections()
	})
	return p.stopErr
}

// logTail returns the scrubbed tail of the child's log (tests and diagnostics).
func (p *nativeProcess) logTail() string { return security.Scrub(truncateTail(p.logs.String(), 2000)) }
