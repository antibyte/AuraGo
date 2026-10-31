package agent

import (
	"context"
	"testing"

	"aurago/internal/config"
	"aurago/internal/decisions"
	"aurago/internal/decisions/consumers"
)

type fakeConsumers struct {
	available bool
	calls     int
	evaluate  func(context.Context, decisions.Request) (decisions.Response, error)
}

var _ consumers.Service = (*fakeConsumers)(nil)

func (f *fakeConsumers) Snapshot(_ *config.Config, id consumers.ID) consumers.Snapshot {
	if !f.available {
		return consumers.Snapshot{Consumer: id, Code: consumers.CodeConsumerDisabled}
	}
	return consumers.Snapshot{Consumer: id, Available: true, Code: consumers.CodeAvailable,
		Current: func() bool { return true },
		Evaluate: func(ctx context.Context, r decisions.Request) (decisions.Response, error) {
			f.calls++
			return f.evaluate(ctx, r)
		}}
}

func TestRunConfigCarriesDecisionConsumers(t *testing.T) {
	f := &fakeConsumers{available: true}
	run := RunConfig{DecisionConsumers: f}
	if got := run.DecisionConsumers.Snapshot(nil, consumers.MailTriage); !got.Available {
		t.Fatal("expected available snapshot")
	}
	if ConsumersFrom(nil) != nil {
		t.Fatal("nil service must yield nil consumers")
	}
}
