package tools

import (
	"context"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/findit"
)

func TestFindItReadRequiresTheRunGrantAndKeepsReadOnlyAccess(t *testing.T) {
	store := openFindItToolTestStore(t)
	live := findItToolTestConfig()
	SetFindItStore(store, func() *config.Config { return live })
	t.Cleanup(func() { WithdrawFindItStore(store) })

	deniedRun := findItToolTestConfig()
	deniedRun.FindIt.AgentAccess = false
	got := ExecuteFindIt(context.Background(), deniedRun, findit.QueryRequest{Operation: "search", Entity: "items"})
	if !strings.Contains(got, `"status":"error"`) || !strings.Contains(got, `"code":"policy_denied"`) {
		t.Fatalf("run without the FindIt grant returned inventory: %s", got)
	}

	readonlyRun := findItToolTestConfig()
	readonlyRun.FindIt.ReadOnly = true
	readonlyRun.VirtualDesktop.ReadOnly = true
	live.FindIt.ReadOnly = true
	live.VirtualDesktop.ReadOnly = true
	got = ExecuteFindIt(context.Background(), readonlyRun, findit.QueryRequest{Operation: "search", Entity: "items"})
	if !strings.Contains(got, `"status":"success"`) {
		t.Fatalf("read-only FindIt access was denied: %s", got)
	}
}
