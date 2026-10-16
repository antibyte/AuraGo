package decisions

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

const runtimeStartupTimeout = 5 * time.Minute
const runtimeStartupProbeTimeout = 2 * time.Second

// The readiness deadline must not become the resident process's lifetime.
func waitRuntimeReady(ctx context.Context, timeout time.Duration, probe func(context.Context) (bool, error)) error {
	readyCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return NewError(CodeCancelled, ctx.Err())
		}
		if readyCtx.Err() != nil {
			return NewError(CodeRuntimeStartTimeout, readyCtx.Err())
		}
		probeCtx, probeCancel := context.WithTimeout(readyCtx, runtimeStartupProbeTimeout)
		ready, err := probe(probeCtx)
		probeCancel()
		if ctx.Err() != nil {
			return NewError(CodeCancelled, ctx.Err())
		}
		if readyCtx.Err() != nil {
			return NewError(CodeRuntimeStartTimeout, readyCtx.Err())
		}
		if err != nil {
			return err
		}
		if ready {
			return nil
		}
		select {
		case <-readyCtx.Done():
		case <-ticker.C:
		}
	}
}

type nativeStartupFailure struct{ code string }

func (failure nativeStartupFailure) Error() string { return failure.code }

// Only complete, bounded allowlisted lines may reach status or error responses.
type nativeStartupStderr struct {
	mu      sync.Mutex
	line    []byte
	discard bool
	failure string
	phase   string
}

func (buffer *nativeStartupStderr) Write(data []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	for _, value := range data {
		if value == '\n' {
			if !buffer.discard {
				line := strings.TrimSuffix(string(buffer.line), "\r")
				if code, ok := strings.CutPrefix(line, "AURAGO_DECISIONS_STARTUP_ERROR code="); ok {
					if _, allowed := nativeStartupCodes[code]; allowed {
						buffer.failure = code
					}
				}
				if phase, ok := strings.CutPrefix(line, "AURAGO_DECISIONS_STARTUP phase="); ok {
					switch phase {
					case "verifying_model", "verifying_projector", "loading_model", "loading_projector", "warming_up":
						buffer.phase = phase
					}
				}
			}
			buffer.line, buffer.discard = buffer.line[:0], false
		} else if len(buffer.line) < 128 && !buffer.discard {
			buffer.line = append(buffer.line, value)
		} else {
			buffer.discard = true
		}
	}
	return len(data), nil
}

func (buffer *nativeStartupStderr) code() string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.failure
}

func (buffer *nativeStartupStderr) progress() string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.phase
}

var nativeStartupCodes = map[string]struct{}{
	"invalid_configuration": {}, "model_pin_mismatch": {}, "projector_pin_mismatch": {}, "model_load_failed": {},
	"projector_load_failed": {}, "device_unavailable": {}, "offload_unavailable": {}, "unauthorized_cpu_compute": {}, "runtime_start_failed": {},
}

func runtimeStartupError(code string) error {
	if _, allowed := nativeStartupCodes[code]; !allowed {
		return NewError(CodeRuntimeStartFailed, nil)
	}
	publicCode := CodeRuntimeStartFailed
	switch code {
	case "model_pin_mismatch", "projector_pin_mismatch":
		publicCode = CodeModelIntegrityFailed
	case "invalid_configuration":
		publicCode = CodeRuntimeInvalidConfiguration
	}
	return NewError(publicCode, nativeStartupFailure{code: code})
}

func terminalRuntimeStartError(err error) bool {
	switch ErrorCode(err) {
	case CodeCancelled, CodeRuntimeStartTimeout, CodeModelIntegrityFailed, CodeRuntimeInvalidConfiguration:
		return true
	}
	return false
}

func runtimeFailureReason(err error) string {
	var startupFailure nativeStartupFailure
	if errors.As(err, &startupFailure) {
		if _, allowed := nativeStartupCodes[startupFailure.code]; allowed {
			return startupFailure.code
		}
	}
	return ErrorCode(err)
}
