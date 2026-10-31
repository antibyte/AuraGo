package desktop

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
)

type countedCodeDocker struct {
	*fakeCodeContainerDocker
	inspects atomic.Int32
	lists    atomic.Int32
	entered  chan struct{}
	resume   chan struct{}
}

func (d *countedCodeDocker) InspectContainer(ctx context.Context, id string) (CodeDockerInspect, error) {
	d.inspects.Add(1)
	if d.entered != nil {
		d.entered <- struct{}{}
		<-d.resume
	}
	return d.fakeCodeContainerDocker.InspectContainer(ctx, id)
}

func (d *countedCodeDocker) ListContainers(ctx context.Context, all bool) ([]CodeDockerContainer, error) {
	d.lists.Add(1)
	return d.fakeCodeContainerDocker.ListContainers(ctx, all)
}

func TestCodeContainerCachedParallelAccessDoesOneInspect(t *testing.T) {
	d := &countedCodeDocker{fakeCodeContainerDocker: &fakeCodeContainerDocker{}}
	svc := NewCodeContainerService(Config{WorkspaceDir: t.TempDir(), CodeStudio: CodeStudioConfig{Enabled: true}}, nil)
	svc.autoStopAfter = 0
	svc.SetDockerClient(d)
	if _, err := svc.EnsureContainer(context.Background()); err != nil {
		t.Fatal(err)
	}
	beforeExecs, beforeLists, beforeInspects := len(d.execs), d.lists.Load(), d.inspects.Load()
	d.entered, d.resume = make(chan struct{}, 8), make(chan struct{})
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			id, err := svc.EnsureContainer(context.Background())
			if err != nil || id != "created-1" {
				t.Errorf("acquire id=%q err=%v", id, err)
			}
		})
	}
	for range 8 {
		<-d.entered
	}
	// Docker is deliberately stalled; the state lock must still be available.
	if !svc.mu.TryLock() {
		t.Error("Docker I/O held the state write lock")
	} else {
		svc.mu.Unlock()
	}
	close(d.resume)
	wg.Wait()
	if d.inspects.Load()-beforeInspects != 8 || d.lists.Load() != beforeLists || len(d.execs) != beforeExecs {
		t.Fatalf("cached access: inspects=%d lists=%d execs=%d", d.inspects.Load()-beforeInspects, d.lists.Load()-beforeLists, len(d.execs)-beforeExecs)
	}
	if id, err := svc.ContainerID(context.Background()); err != nil || id != "created-1" || d.lists.Load() != beforeLists {
		t.Fatalf("cached ID = %s: %v", id, err)
	}
}

func TestCodeContainerTerminalLeasesInvalidateIdleCallbacks(t *testing.T) {
	d := &fakeCodeContainerDocker{}
	svc := NewCodeContainerService(Config{WorkspaceDir: t.TempDir(), CodeStudio: CodeStudioConfig{Enabled: true}}, nil)
	svc.autoStopAfter = 0
	svc.SetDockerClient(d)
	id, err := svc.EnsureContainer(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	oldGeneration := svc.timerGeneration
	first, err := svc.HoldTerminal(id)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.HoldTerminal(id)
	if err != nil {
		t.Fatal(err)
	}
	if svc.Status(context.Background()).ActiveTerminalSessions != 2 || svc.stopTimer != nil {
		t.Fatal("live terminals did not suspend idle stop")
	}
	first()
	first()
	if svc.activeSessions != 1 {
		t.Fatal("release is not idempotent")
	}
	if err := svc.stopContainerLocked(context.Background(), oldGeneration); err != nil || !svc.IsRunning() {
		t.Fatal("stale idle callback stopped a live terminal")
	}
	second()
	if svc.activeSessions != 0 || svc.timerGeneration == oldGeneration {
		t.Fatal("last release did not refresh idle generation")
	}
	readyGeneration := svc.timerGeneration
	third, err := svc.HoldTerminal(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.stopContainerLocked(context.Background(), readyGeneration); err != nil || !svc.IsRunning() {
		t.Fatal("delayed callback stopped a new terminal")
	}
	third()
	if err := svc.stopContainerLocked(context.Background(), svc.timerGeneration); err != nil || svc.IsRunning() {
		t.Fatal("current idle callback did not stop the container")
	}
	if _, err := svc.HoldTerminal(id); err == nil {
		t.Fatal("stopped container accepted a terminal")
	}
}
