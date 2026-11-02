package a2a

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
)

func TestBridgeQueuedTaskSharesShutdownAndSessionScope(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	registry := agent.NewCoAgentRegistry(1, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	active, _, err := registry.RegisterWorker("local", "hold capacity", "owner", cancel, 2, false)
	if err != nil {
		t.Fatal(err)
	}
	manager := NewClientManager(&config.Config{}, logger)
	manager.status["remote"] = &RemoteAgentStatus{Available: true}
	bridge := NewBridge(manager, registry, logger)
	id, err := bridge.SpawnRemote(context.Background(), "owner", "remote", "queued fixture")
	if err != nil {
		t.Fatal(err)
	}
	status, err := registry.GetStatusForSession("owner", id)
	if err != nil || status["state"] != string(agent.CoAgentQueued) {
		t.Fatalf("queued remote: %v %v", status, err)
	}
	if _, err := registry.GetStatusForSession("foreign", id); err == nil {
		t.Fatal("foreign session saw remote task")
	}
	stopped := make(chan struct{})
	go func() { registry.Shutdown(); close(stopped) }()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("shutdown did not cancel work")
	}
	select {
	case <-stopped:
		t.Fatal("shutdown did not wait for actual worker end")
	default:
	}
	registry.WorkerDone(active)
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("queued bridge worker did not drain")
	}
	if _, err := bridge.SpawnRemote(context.Background(), "owner", "remote", "late"); err == nil {
		t.Fatal("bridge accepted work after shutdown")
	}
	status, err = registry.GetStatusForSession("owner", id)
	if err != nil || status["state"] != string(agent.CoAgentCancelled) {
		t.Fatalf("remote result after shutdown: %v %v", status, err)
	}
}
