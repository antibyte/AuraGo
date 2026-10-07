package gamemaker

import (
	"context"
	"testing"
)

func TestUpdatePolicyRevokesActiveJobContexts(t *testing.T) {
	s := newTestService(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.mu.Lock()
	s.jobCancels["inflight"] = cancel
	s.mu.Unlock()
	s.UpdatePolicy(Policy{Enabled: true, ReadOnly: true, AllowEdit: true})
	if ctx.Err() == nil {
		t.Fatal("active job survived readonly")
	}
	s.UpdatePolicy(Policy{Enabled: true, AllowEdit: true})
	if ctx.Err() == nil {
		t.Fatal("old job revived")
	}
}
