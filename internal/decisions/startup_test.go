package decisions

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

func TestRuntimeReadinessAllowsSlowStartupAndBoundsHungProbe(t *testing.T) {
	for _, mode := range []string{"slow", "hung", "cancel", "exit"} {
		t.Run(mode, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				start := time.Now()
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				if mode == "cancel" {
					time.AfterFunc(3*time.Second, cancel)
				}
				err := waitRuntimeReady(ctx, runtimeStartupTimeout, func(probeCtx context.Context) (bool, error) {
					switch mode {
					case "slow":
						return time.Since(start) > 60*time.Second, nil
					case "exit":
						return false, runtimeStartupError("model_pin_mismatch")
					default:
						<-probeCtx.Done()
						return false, nil
					}
				})
				switch mode {
				case "slow":
					if err != nil || time.Since(start) <= time.Minute {
						t.Fatalf("slow start: %v after %v", err, time.Since(start))
					}
				case "hung":
					if ErrorCode(err) != CodeRuntimeStartTimeout || time.Since(start) != runtimeStartupTimeout {
						t.Fatalf("unbounded probe: %v after %v", err, time.Since(start))
					}
				case "cancel":
					if ErrorCode(err) != CodeCancelled || !errors.Is(err, context.Canceled) || time.Since(start) != 3*time.Second {
						t.Fatalf("cancellation: %v after %v", err, time.Since(start))
					}
				case "exit":
					if ErrorCode(err) != CodeModelIntegrityFailed || time.Since(start) != 0 {
						t.Fatalf("delayed failure: %v", err)
					}
				}
			})
		})
	}
}

func TestStartupDiagnosticsParseBoundedFragmentedLines(t *testing.T) {
	buffer := &nativeStartupStderr{}
	input := "private backend diagnostic\n" + strings.Repeat("x", 129) + "AURAGO_DECISIONS_STARTUP_ERROR code=model_pin_mismatch\n" +
		"AURAGO_DECISIONS_STARTUP phase=loading_model\r\n" +
		"AURAGO_DECISIONS_STARTUP phase=private/path\n" +
		"AURAGO_DECISIONS_STARTUP_ERROR code=offload_unavailable /private/path\n"
	for _, b := range []byte(input) {
		_, _ = buffer.Write([]byte{b})
	}
	if buffer.code() != "" || buffer.progress() != "loading_model" {
		t.Fatal("diagnostics escaped allowlist")
	}
	_, _ = buffer.Write([]byte("AURAGO_DECISIONS_STARTUP_ERROR code=offload_unavailable"))
	if buffer.code() != "" {
		t.Fatal("incomplete error accepted")
	}
	_, _ = buffer.Write([]byte("\n"))
	if buffer.code() != "offload_unavailable" {
		t.Fatal("error after phase lost")
	}
	for _, code := range []string{CodeRuntimeStartTimeout, CodeRuntimeInvalidConfiguration, CodeModelIntegrityFailed} {
		if !IsNonRetryable(NewError(code, nil)) {
			t.Fatalf("flow may retry %s", code)
		}
	}
}
