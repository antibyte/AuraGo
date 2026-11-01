package agent

import (
	"path/filepath"
	"testing"

	"aurago/internal/config"
	"aurago/internal/findit"
	"aurago/internal/tools"
)

func TestFindItRuntimeAvailabilityUsesRunGrantAndKeepsReadOnlyAccess(t *testing.T) {
	store, err := findit.Open(filepath.Join(t.TempDir(), "findit.db"))
	if err != nil {
		t.Fatalf("open FindIt store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	live := &config.Config{}
	live.FindIt.Enabled = true
	live.FindIt.AgentAccess = true
	live.VirtualDesktop.Enabled = true
	tools.SetFindItStore(store, func() *config.Config { return live })
	t.Cleanup(func() { tools.WithdrawFindItStore(store) })

	run := live.Clone()
	run.FindIt.AgentAccess = false
	if reason := toolRuntimeUnavailable(&DispatchContext{Cfg: run}, "findit"); reason == "" {
		t.Fatal("FindIt was runtime-available to a run without the agent grant")
	}

	run = live.Clone()
	run.FindIt.ReadOnly = true
	run.VirtualDesktop.ReadOnly = true
	live.FindIt.ReadOnly = true
	live.VirtualDesktop.ReadOnly = true
	if reason := toolRuntimeUnavailable(&DispatchContext{Cfg: run}, "findit"); reason != "" {
		t.Fatalf("read-only FindIt reader was blocked: %s", reason)
	}
}
