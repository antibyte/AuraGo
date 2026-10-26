package agent

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

func TestGuardianQuarantineStopsTheCallAndAllowStillRuns(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fail   string
		calls  int32
		denied bool
	}{
		{name: "quarantine", fail: "quarantine", calls: 0, denied: true},
		{name: "allow", fail: "allow", calls: 1, denied: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			previous := actionApprovals
			actionApprovals = newActionApprovalManager()
			t.Cleanup(func() {
				CancelActionApprovals(t.Name())
				actionApprovals = previous
			})

			var calls atomic.Int32
			cfg := &config.Config{}
			cfg.Docker.Enabled = true
			cfg.Agent.ActionPolicy.Enabled = false
			cfg.LLMGuardian.Enabled = true
			cfg.LLMGuardian.FailSafe = tc.fail
			cfg.LLMGuardian.DefaultLevel = "high"
			cfg.LLMGuardian.TimeoutSecs = 1
			cfg.LLMGuardian.BaseURL = "http://127.0.0.1:1"
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			guardian := security.NewLLMGuardian(cfg, logger)
			if guardian == nil {
				t.Fatal("expected an enabled guardian")
			}
			dc := &DispatchContext{
				Cfg:         cfg,
				SessionID:   t.Name(),
				Logger:      logger,
				LLMGuardian: guardian,
				ExecutionHooks: &ExecutionHooks{HandleTool: func(context.Context, ToolCall) (string, bool) {
					calls.Add(1)
					return `{"status":"success"}`, true
				}},
			}
			found := false
			for _, schema := range dispatchCatalogSchemas(dc) {
				found = found || schema.Function != nil && schema.Function.Name == "docker"
			}
			if !found {
				t.Fatal("docker must be in the catalog so policy does not refuse before the guardian")
			}

			call := actionSecurityCall()
			result := DispatchToolCallResult(context.Background(), &call, dc, "remove the fixture container")
			if got := calls.Load(); got != tc.calls {
				t.Fatalf("handler calls = %d, want %d; output %s", got, tc.calls, result.Output)
			}
			if strings.Contains(result.Output, "127.0.0.1") || strings.Contains(strings.ToLower(result.Output), "fail-safe") {
				t.Fatalf("scanner reason leaked into the tool result: %s", result.Output)
			}
			if tc.denied {
				if result.Status != ToolResultDenied || !result.IsError {
					t.Fatalf("quarantine status = %+v", result)
				}
				if !call.GuardianBlocked || call.GuardianBlockReason != "quarantine" {
					t.Fatalf("guardian flags = blocked %v reason %q", call.GuardianBlocked, call.GuardianBlockReason)
				}
				if !strings.Contains(result.Output, guardianQuarantineNotice) {
					t.Fatalf("fixed notice missing: %s", result.Output)
				}
				return
			}
			if result.Status == ToolResultDenied {
				t.Fatalf("allow fail-safe denied the call: %s", result.Output)
			}
		})
	}
}
