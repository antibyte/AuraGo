package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/storymaker"
)

const storymakerToolTestProject = "11111111-2222-4333-8444-555555555555"

type fakeStorymakerFacade struct {
	available bool
	created   storymaker.AgentCreateRequest
	renderErr error
}

func (f *fakeStorymakerFacade) Available() bool { return f.available }

func (f *fakeStorymakerFacade) Create(_ context.Context, req storymaker.AgentCreateRequest) (storymaker.AgentJobRef, error) {
	f.created = req
	return storymaker.AgentJobRef{ProjectID: storymakerToolTestProject, JobID: "job-1", State: "queued", ETAHintSeconds: 235}, nil
}

func (f *fakeStorymakerFacade) Status(context.Context, string) (storymaker.AgentStatus, error) {
	return storymaker.AgentStatus{State: storymaker.OutcomeReadyUnverified}, nil
}

func (f *fakeStorymakerFacade) Revise(_ context.Context, projectID, _ string) (storymaker.AgentJobRef, error) {
	return storymaker.AgentJobRef{ProjectID: projectID, JobID: "job-2", State: "queued"}, nil
}

func (f *fakeStorymakerFacade) Render(context.Context, string) (string, error) {
	return "render-1", f.renderErr
}

func (f *fakeStorymakerFacade) RenderSilent(context.Context, string) (string, int, error) {
	return "render-2", 3, nil
}

func (f *fakeStorymakerFacade) List(context.Context) ([]storymaker.AgentProjectRow, error) {
	return nil, nil
}

func storymakerToolCall(t *testing.T, dc *DispatchContext, params map[string]interface{}) map[string]any {
	t.Helper()
	out, handled := dispatchPlatform(context.Background(), ToolCall{Action: "storymaker", Params: params}, dc)
	if !handled || !strings.HasPrefix(out, "Tool Output: ") {
		t.Fatalf("dispatch = %v %s", handled, out)
	}
	var answer map[string]any
	if err := json.Unmarshal([]byte(strings.TrimPrefix(out, "Tool Output: ")), &answer); err != nil {
		t.Fatalf("answer is not JSON: %v\n%s", err, out)
	}
	return answer
}

func TestStorymakerChatToolDispatchesAndOpens(t *testing.T) {
	fake := &fakeStorymakerFacade{available: true}
	storymaker.SetAgentFacade(fake)
	t.Cleanup(func() { storymaker.SetAgentFacade(nil) })
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.AllowAgentControl = true
	cfg.Tools.VirtualDesktop.Enabled = true
	broker := &retroGamesJSONBroker{}
	dc := &DispatchContext{Cfg: cfg, Logger: testLogger, Broker: broker}

	created := storymakerToolCall(t, dc, map[string]interface{}{
		"operation": "create", "topic": "Was ist Phishing?", "format": "9:16", "length_seconds": float64(45),
		"language": "de", "voice": map[string]interface{}{"provider": "piper", "voice": "de-1"},
	})
	if created["status"] != "success" || created["project_id"] != storymakerToolTestProject || created["job_id"] != "job-1" || created["state"] != "queued" || created["eta_hint_seconds"] != float64(235) {
		t.Fatalf("create = %v", created)
	}
	if fake.created.Topic != "Was ist Phishing?" || fake.created.Aspect != "9:16" || fake.created.LengthSeconds != 45 || fake.created.Voice == nil || fake.created.Voice.Provider != "piper" {
		t.Fatalf("create request = %+v", fake.created)
	}

	opened := storymakerToolCall(t, dc, map[string]interface{}{"operation": "open", "project_id": storymakerToolTestProject})
	if opened["opened"] != true {
		t.Fatalf("open = %v", opened)
	}
	broker.mu.Lock()
	broadcasts := append([]string(nil), broker.json...)
	broker.mu.Unlock()
	if len(broadcasts) != 1 {
		t.Fatalf("broadcasts = %d", len(broadcasts))
	}
	var msg struct {
		Type    string `json:"type"`
		Payload struct {
			Type    string            `json:"type"`
			Payload map[string]string `json:"payload"`
		} `json:"payload"`
	}
	if err := json.Unmarshal([]byte(broadcasts[0]), &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Type != "virtual_desktop_event" || msg.Payload.Type != "open_app" || msg.Payload.Payload["app_id"] != "storymaker" || msg.Payload.Payload["project_id"] != storymakerToolTestProject {
		t.Fatalf("broadcast = %+v", msg)
	}

	if bad := storymakerToolCall(t, dc, map[string]interface{}{"operation": "open", "project_id": "../other"}); bad["code"] != "project_not_found" {
		t.Fatalf("bad id = %v", bad)
	}
	cfg.VirtualDesktop.AllowAgentControl = false
	if denied := storymakerToolCall(t, dc, map[string]interface{}{"operation": "open", "project_id": storymakerToolTestProject}); denied["code"] != "desktop_agent_control_disabled" {
		t.Fatalf("no agent control = %v", denied)
	}
	broker.mu.Lock()
	broadcastCount := len(broker.json)
	broker.mu.Unlock()
	if broadcastCount != 1 {
		t.Fatalf("a refused open must not broadcast, got %d", broadcastCount)
	}

	fake.renderErr = &storymaker.AgentError{Code: "narration_incomplete", Message: "m", NextAction: "call render with silent:true"}
	refused := storymakerToolCall(t, dc, map[string]interface{}{"operation": "render", "project_id": storymakerToolTestProject})
	if refused["status"] != "error" || refused["code"] != "narration_incomplete" || !strings.Contains(refused["next_action"].(string), "silent") {
		t.Fatalf("refused render = %v", refused)
	}
	silent := storymakerToolCall(t, dc, map[string]interface{}{"operation": "render", "project_id": storymakerToolTestProject, "silent": true})
	if silent["render_job_id"] != "render-2" || silent["speech_removed"] != true || silent["scenes_changed"] != float64(3) {
		t.Fatalf("silent render = %v", silent)
	}

	storymaker.SetAgentFacade(nil)
	if off := storymakerToolCall(t, dc, map[string]interface{}{"operation": "list"}); off["code"] != "agent_disabled" {
		t.Fatalf("no facade = %v", off)
	}
}
