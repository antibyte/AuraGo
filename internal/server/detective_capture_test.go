package server

import (
	"aurago/internal/agent"
	"aurago/internal/detective"
	"aurago/internal/security"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDetectiveCaptureBindsOnlyRetrievedDocuments(t *testing.T) {
	svc, err := detective.New(detective.Options{Path: filepath.Join(t.TempDir(), "case.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	svc.SetRunner(detectiveTestRunner(func(ctx context.Context, job *detective.Session) error {
		_, done, err := job.Activate(ctx)
		if err != nil {
			return err
		}
		defer done()
		fixtures := []struct {
			call agent.ToolCall
			out  string
		}{
			{agent.ToolCall{Action: "api_request", URL: "https://example.org/actual"}, `{"status":"success","status_code":200,"body":"A result containing https://example.org/not-read"}`},
			{agent.ToolCall{Action: "ddg_search"}, `{"status":"success","results":[{"url":"https://example.org/search","snippet":"Search hint"}]}`},
			{agent.ToolCall{Action: "virtual_browser", Operation: "inspect"}, `{"status":"ok","data":{"result":{"data":{"url":"https://example.org/visible","title":"Visible document","text":"Verified browser text"}}}}`},
			{agent.ToolCall{Action: "mcp_call", Operation: "call", Params: map[string]any{"server": "fixture", "tool_name": "read"}}, "Retrieved private document"},
			{agent.ToolCall{Action: "composio_call", Operation: "execute_tool"}, "Tool Output: " + security.IsolateExternalData(`{"status":"success","result":{"content":"Private record"}}`)},
		}
		for _, f := range fixtures {
			if out := detectiveCapture(job, f.call, f.out); !strings.Contains(out, "Server-recorded research sources") {
				t.Errorf("no receipt: %s", f.call.Action)
			}
		}
		before, _ := job.Snapshot()
		detectiveCapture(job, agent.ToolCall{Action: "api_request", URL: "https://example.org/failure"}, `{"status":"success","status_code":404,"body":"not evidence"}`)
		for _, failure := range []string{`{"status":"error","message":"failed"}`, `{"isError":true,"content":"failed"}`, `{"status": "error", "message": "bad "quoted" failure"}`} {
			detectiveCapture(job, agent.ToolCall{Action: "mcp_call"}, failure)
		}
		after, _ := job.Snapshot()
		if len(after.Sources) != 5 || len(before.Sources) != 5 {
			t.Fatalf("wrong sources: %+v", after.Sources)
		}
		for _, s := range after.Sources {
			if s.URL == "https://example.org/not-read" {
				t.Fatal("nested URL became a source")
			}
			if s.Method == "ddg_search" && s.Status != "search_hit" {
				t.Fatal("search promoted to read")
			}
		}
		source := after.Sources[2]
		if _, err = job.AddFinding(detective.Finding{SourceID: source.ID, Text: "Browser finding", Quote: "Verified browser text"}); err != nil {
			t.Fatal(err)
		}
		return nil
	}))
	c, _ := svc.Create(detective.Request{Topic: "Receipts"})
	_, err = svc.Start(c.ID, "start", "", "capture")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		c, _ = svc.Get(c.ID)
		if c.Run.Status == "partial" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(c.Sources) != 5 {
		b, _ := json.Marshal(c)
		t.Fatalf("capture incomplete: %s", b)
	}
}
