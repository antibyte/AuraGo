package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
)

func TestStorymakerAgentJobInterruptedAfterRestart(t *testing.T) {
	s, _, token := storymakerAPIFixture(t)
	started := make(chan struct{}, 1)
	s.storymakerAgentRunOverride = func(ctx context.Context, job render.Job, payload any, h *render.Handle) (render.Output, error) {
		h.SetAgent(storymaker.AgentJobInfo{Kind: "create", Phase: storymaker.PhaseFill, ScenesDone: 1, ScenesTotal: 3})
		started <- struct{}{}
		<-ctx.Done()
		return render.Output{}, ctx.Err()
	}
	p := createStorymakerProject(t, s, token)
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	job, _, err := rt.agent.Enqueue(render.EnqueueRequest{ProjectID: p.Project.ID, Kind: render.KindAgent, Coalesce: true})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("the agent job did not start")
	}

	// jobs.json as a crash leaves it: the job still reads as running. A clean close would rewrite it.
	path := filepath.Join(storymakerDir(rt.dataDir), "agent", "jobs.json")
	crashed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s.closeStorymaker()
	if err := os.WriteFile(path, crashed, 0o600); err != nil {
		t.Fatal(err)
	}
	s.storymakerMu.Lock()
	s.storymakerClosed = false
	s.storymakerMu.Unlock()
	rt, err = s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}

	got, manager, ok := rt.jobByID(job.ID)
	if !ok || manager != rt.agent {
		t.Fatalf("job %s was not found in the agent queue", job.ID)
	}
	if got.Status != render.StatusInterrupted || got.Error == nil || got.Error.Code != render.CodeInterrupted {
		t.Fatalf("job after the restart = %+v", got)
	}
	if got.Agent == nil || got.Agent.Phase != storymaker.PhaseFill || got.Agent.ScenesDone != 1 || got.Agent.ScenesTotal != 3 {
		t.Fatalf("agent state after the restart = %+v", got.Agent)
	}

	select {
	case first, ok := <-openStorymakerEvents(t, s, token, p.Project.ID):
		if !ok || first.ID != job.ID || first.Agent == nil {
			t.Fatalf("first stream job = %+v", first)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the stream did not send the agent job")
	}
}

func TestStorymakerAgentUsageFailsClosed(t *testing.T) {
	dir := t.TempDir()
	usage := newStorymakerAgentUsage(dir)
	const maxDaily = 2
	for i := 0; i < maxDaily; i++ {
		if err := usage.reserve(maxDaily); err != nil {
			t.Fatalf("reservation %d: %v", i+1, err)
		}
	}
	if count, err := usage.today(maxDaily); err != nil || count != maxDaily {
		t.Fatalf("today = %d, %v, want %d", count, err, maxDaily)
	}
	assertRefused := func(when string) {
		t.Helper()
		var jobErr *render.JobError
		if err := usage.reserve(maxDaily); !errors.As(err, &jobErr) || jobErr.Code != storymakerCodeDailyLimit {
			t.Fatalf("%s: reserve = %v, want %s", when, err, storymakerCodeDailyLimit)
		}
	}
	assertRefused("at the limit")

	if err := os.WriteFile(filepath.Join(dir, "usage.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	assertRefused("with an unreadable count")
	if _, err := os.Stat(filepath.Join(dir, "usage.bad.json")); err != nil {
		t.Fatalf("the unreadable count was not kept: %v", err)
	}
	assertRefused("after the unreadable count was replaced")
}
