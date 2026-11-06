package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestStorymakerBrokerWithholdsProse(t *testing.T) {
	const secret = "SECRET-MODEL-PROSE"
	dir := t.TempDir()
	eventLog := newAgentEventLog(dir)
	jobID := uuid.NewString()
	broker := newStorymakerBroker(eventLog, "project-1", jobID)

	broker.SendLLMStreamDelta(secret, "", "", 0, "")
	broker.SendLLMStreamDelta("", "storymaker_story", "call-1", 0, "")
	broker.SendLLMStreamDone("stop")
	broker.SendThinkingBlock("provider", secret, "delta")
	broker.SendJSON(`{"text":"` + secret + `"}`)
	broker.Send("thinking", secret)
	broker.Send("tool_call", secret)
	broker.Send("tool_start", secret)
	broker.Send("tool_output", secret)
	broker.Send("model_progress", secret)
	broker.Send("model_progress", "waiting")
	broker.Send("model_progress", "waiting") // the throttle drops the repeat
	broker.Send("model_progress", "retrying")
	broker.SendTokenUpdate(120, 30, 150, 0, 0, true, false, "provider_usage")
	raw, err := os.ReadFile(filepath.Join(dir, jobID+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), secret) {
		t.Fatalf("the event file holds model prose:\n%s", raw)
	}
	lines := eventLog.after(jobID, 0)
	if len(lines) != 3 {
		t.Fatalf("%d events, want model waiting, model retrying and usage:\n%s", len(lines), raw)
	}
	type storedEvent struct {
		Type        string `json:"type"`
		Status      string `json:"status"`
		TotalTokens int    `json:"total_tokens"`
		Estimated   bool   `json:"estimated"`
	}
	want := []storedEvent{
		{Type: agentEventModel, Status: "waiting"},
		{Type: agentEventModel, Status: "retrying"},
		{Type: agentEventUsage, TotalTokens: 150, Estimated: true},
	}
	for i, line := range lines {
		var got storedEvent
		if err := json.Unmarshal(line.Data, &got); err != nil {
			t.Fatal(err)
		}
		if got != want[i] {
			t.Fatalf("event %d = %+v, want %+v", i, got, want[i])
		}
	}
}
