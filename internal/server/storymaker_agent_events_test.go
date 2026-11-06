package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"

	"github.com/google/uuid"
)

type storymakerAgentStreamEvent struct {
	JobID   string `json:"job_id"`
	Seq     int    `json:"seq"`
	At      string `json:"at"`
	Type    string `json:"type"`
	Phase   string `json:"phase"`
	Outcome string `json:"outcome"`
}

// openStorymakerAgentEvents opens the event stream of a project and returns its agent events in the
// order they arrive.
func openStorymakerAgentEvents(t *testing.T, s *Server, token, projectID string) <-chan storymakerAgentStreamEvent {
	t.Helper()
	srv := httptest.NewServer(handleStorymaker(s))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+storymakerAPIBase+"/projects/"+projectID+"/events", nil)
	if err != nil {
		cancel()
		srv.Close()
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		srv.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		res.Body.Close()
		srv.Close()
	})
	events := make(chan storymakerAgentStreamEvent, 16)
	go func() {
		defer close(events)
		scanner := bufio.NewScanner(res.Body)
		scanner.Buffer(make([]byte, 64<<10), 1<<20)
		var name string
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case strings.HasPrefix(line, "event: "):
				name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: ") && name == "agent":
				var event storymakerAgentStreamEvent
				if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event) == nil {
					events <- event
				}
			}
		}
	}()
	return events
}

func nextStorymakerAgentEvent(t *testing.T, events <-chan storymakerAgentStreamEvent) storymakerAgentStreamEvent {
	t.Helper()
	select {
	case event, ok := <-events:
		if !ok {
			t.Fatal("the event stream ended")
		}
		return event
	case <-time.After(10 * time.Second):
		t.Fatal("no agent event arrived")
	}
	return storymakerAgentStreamEvent{}
}

func TestAgentEventLogCapsAndReplays(t *testing.T) {
	// One job file keeps its newest lines, and a restart continues the numbers.
	dir := t.TempDir()
	eventLog := newAgentEventLog(dir)
	jobID := uuid.NewString()
	total := agentEventMaxLines + 10
	fields := map[string]any{"tool": storymaker.ToolStory, "status": "ok"}
	for i := 0; i < total; i++ {
		eventLog.emit("project-1", jobID, agentEventTool, fields)
	}
	raw, err := os.ReadFile(filepath.Join(dir, jobID+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if got := bytes.Count(raw, []byte{'\n'}); got != agentEventMaxLines {
		t.Fatalf("file has %d lines, want %d", got, agentEventMaxLines)
	}
	eventLog = newAgentEventLog(dir)
	replay := eventLog.after(jobID, 0)
	if len(replay) != agentEventMaxLines || replay[0].Seq != total-agentEventMaxLines+1 || replay[len(replay)-1].Seq != total {
		t.Fatalf("replay has %d events, from %d to %d", len(replay), replay[0].Seq, replay[len(replay)-1].Seq)
	}
	eventLog.emit("project-1", jobID, agentEventTool, fields)
	if more := eventLog.after(jobID, total); len(more) != 1 || more[0].Seq != total+1 {
		t.Fatalf("events after the restart = %+v", more)
	}

	// A start keeps the newest files.
	filesDir := t.TempDir()
	base := time.Now()
	names := make([]string, agentEventMaxFiles+3)
	for i := range names {
		names[i] = uuid.NewString() + ".jsonl"
		path := filepath.Join(filesDir, names[i])
		if err := os.WriteFile(path, []byte("{\"seq\":1}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		at := base.Add(-time.Duration(i) * time.Minute)
		if err := os.Chtimes(path, at, at); err != nil {
			t.Fatal(err)
		}
	}
	newAgentEventLog(filesDir)
	entries, err := os.ReadDir(filesDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != agentEventMaxFiles {
		t.Fatalf("%d files stayed, want %d", len(entries), agentEventMaxFiles)
	}
	if _, err := os.Stat(filepath.Join(filesDir, names[agentEventMaxFiles-1])); err != nil {
		t.Fatalf("the newest files must stay: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filesDir, names[agentEventMaxFiles])); !os.IsNotExist(err) {
		t.Fatalf("the oldest files must go: %v", err)
	}

	// The stream replays on connect and then sends new events.
	s, _, token := storymakerAPIFixture(t)
	s.storymakerAgentRunOverride = storymakerBlockingRun
	p := createStorymakerProject(t, s, token)
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	job, _, err := rt.agent.Enqueue(render.EnqueueRequest{ProjectID: p.Project.ID, Kind: render.KindAgent, Coalesce: true})
	if err != nil {
		t.Fatal(err)
	}
	rt.agentLog.emit(p.Project.ID, job.ID, agentEventPhase, map[string]any{"phase": storymaker.PhaseOutline, "attempt": 1})
	events := openStorymakerAgentEvents(t, s, token, p.Project.ID)
	first := nextStorymakerAgentEvent(t, events)
	if first.JobID != job.ID || first.Seq != 1 || first.Type != agentEventPhase || first.Phase != storymaker.PhaseOutline || first.At == "" {
		t.Fatalf("replayed event = %+v", first)
	}
	rt.agentLog.emit(p.Project.ID, job.ID, agentEventOutcome, map[string]any{"outcome": storymaker.OutcomeIncomplete})
	second := nextStorymakerAgentEvent(t, events)
	if second.Seq != 2 || second.Type != agentEventOutcome || second.Outcome != storymaker.OutcomeIncomplete {
		t.Fatalf("live event = %+v", second)
	}
}
