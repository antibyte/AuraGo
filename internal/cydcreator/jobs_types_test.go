package cydcreator

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

const testPlanJSON = `{"summary":"Weather display with two screens","screens":["home","details"],"widgets":["temperature label"],
"data_sources":["open-meteo"],"wifi":"optional","aurago_link":false,"libraries":["arduinojson"]}`

func TestParseJobPlan(t *testing.T) {
	plan, err := ParseJobPlan([]byte(testPlanJSON))
	if err != nil || plan.WiFi != "optional" || len(plan.Screens) != 2 || plan.Libraries[0] != "arduinojson" {
		t.Fatalf("plan = %+v, %v", plan, err)
	}
	wrapped, _ := json.Marshal(testPlanJSON)
	if _, err := ParseJobPlan(wrapped); err != nil {
		t.Fatalf("string-wrapped plan: %v", err)
	}
	for name, body := range map[string]string{
		"unknown field":   `{"summary":"x","screens":["a"],"wifi":"off","extra":1}`,
		"no screens":      `{"summary":"x","screens":[],"wifi":"off"}`,
		"bad wifi":        `{"summary":"x","screens":["a"],"wifi":"maybe"}`,
		"link needs wifi": `{"summary":"x","screens":["a"],"wifi":"off","aurago_link":true}`,
		"unknown library": `{"summary":"x","screens":["a"],"wifi":"off","libraries":["lodash"]}`,
		"empty summary":   `{"summary":" ","screens":["a"],"wifi":"off"}`,
		"newline entry":   `{"summary":"x","screens":["a\nb"],"wifi":"off"}`,
		"trailing data":   `{"summary":"x","screens":["a"],"wifi":"off"} {}`,
		"not an object":   `[1]`,
	} {
		if _, err := ParseJobPlan([]byte(body)); ErrorCode(err) != CodeBadRequest {
			t.Errorf("%s: err = %v, want CYD_BAD_REQUEST", name, err)
		}
	}
}

func TestValidAttachments(t *testing.T) {
	if _, err := validAttachments([]Attachment{{Kind: "log", Text: strings.Repeat("x", MaxAttachmentBytes+1)}}); ErrorCode(err) != CodeBadRequest {
		t.Fatalf("oversized attachment: %v", err)
	}
	if _, err := validAttachments([]Attachment{{Kind: "image", Text: "x"}}); ErrorCode(err) != CodeBadRequest {
		t.Fatalf("unknown kind: %v", err)
	}
	list, err := validAttachments([]Attachment{{Kind: "log", Text: "Backtrace: 0x400d1234"}, {Kind: "log", Text: "  "}})
	if err != nil || len(list) != 1 {
		t.Fatalf("attachments = %+v, %v", list, err)
	}
}

func TestMessagesAreStoredAndListed(t *testing.T) {
	env, p := newProjectEnv(t)
	ctx := context.Background()
	events, cancel := env.svc.Subscribe(p.ID)
	defer cancel()
	m, err := env.svc.AddUserMessage(ctx, p.ID, "ccj_1", "  Show the weather  ", []Attachment{{Kind: "log", Text: "boot"}})
	if err != nil || m.Text != "Show the weather" || m.Role != MessageRoleUser || len(m.Attachments) != 1 {
		t.Fatalf("message = %+v, %v", m, err)
	}
	env.svc.postMessage(p.ID, "ccj_1", MessageRoleSystem, "The job was cancelled.")
	env.svc.postMessage(p.ID, "ccj_1", MessageRoleAgent, "   ")
	list, err := env.svc.ListMessages(ctx, p.ID)
	if err != nil || len(list) != 2 || list[0].ID != m.ID || list[1].Role != MessageRoleSystem || list[1].Attachments == nil {
		t.Fatalf("messages = %+v, %v", list, err)
	}
	ev := collectEvents(t, events, 1, 2*time.Second, EventJobMessage)[0]
	if ev.JobID != "ccj_1" || !strings.Contains(string(ev.Data), `"text":"Show the weather"`) {
		t.Fatalf("job_message = %s", ev.Data)
	}
	if _, err := env.svc.AddUserMessage(ctx, p.ID, "", strings.Repeat("x", MaxPromptRunes+1), nil); ErrorCode(err) != CodeBadRequest {
		t.Fatalf("long message: %v", err)
	}
	if _, err := env.svc.AddUserMessage(ctx, "ccp_missing", "", "x", nil); ErrorCode(err) != CodeNotFound {
		t.Fatalf("missing project: %v", err)
	}
	if _, err := env.svc.ListMessages(ctx, "ccp_missing"); ErrorCode(err) != CodeNotFound {
		t.Fatalf("list missing project: %v", err)
	}
}

func TestSetProjectLockEmitsLockChanged(t *testing.T) {
	env, p := newProjectEnv(t)
	ctx := context.Background()
	events, cancel := env.svc.Subscribe(p.ID)
	defer cancel()
	if err := env.svc.setProjectLock(ctx, p.ID, "ccj_7"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.WriteFile(ctx, p.ID, "src/app/x.cpp", []byte("int x;"), "*", OriginUser); ErrorCode(err) != CodeProjectLocked {
		t.Fatalf("user write while locked: %v", err)
	}
	if _, err := env.svc.WriteFile(ctx, p.ID, "src/app/x.cpp", []byte("int x;"), "*", JobOrigin("ccj_7")); err != nil {
		t.Fatalf("job write while locked: %v", err)
	}
	if err := env.svc.setProjectLock(ctx, p.ID, ""); err != nil {
		t.Fatal(err)
	}
	got := collectEvents(t, events, 2, 2*time.Second, EventLockChanged)
	if string(got[0].Data) != `{"locked_by_job":"ccj_7"}` || string(got[1].Data) != `{"locked_by_job":""}` {
		t.Fatalf("lock events = %s / %s", got[0].Data, got[1].Data)
	}
}

func TestGetAndListJobs(t *testing.T) {
	env, p := newProjectEnv(t)
	ctx := context.Background()
	plan := &JobPlan{Summary: "x", Screens: []string{"home"}, WiFi: "off"}
	for i, id := range []string{"ccj_a", "ccj_b"} {
		j := Job{ID: id, ProjectID: p.ID, Mode: JobModeChange, Status: JobFailed, Phase: PhaseCoding, Prompt: "p",
			CreatedAt: formatTime(env.svc.now()), Plan: plan}
		if i == 1 {
			j.Plan = nil
		}
		if err := env.svc.insertJob(ctx, j); err != nil {
			t.Fatal(err)
		}
	}
	jobs, err := env.svc.ListJobs(ctx, p.ID)
	if err != nil || len(jobs) != 2 || jobs[0].ID != "ccj_b" || jobs[1].Plan == nil || jobs[1].Plan.Summary != "x" || jobs[0].Plan != nil {
		t.Fatalf("jobs = %+v, %v", jobs, err)
	}
	if _, err := env.svc.GetJob(ctx, "ccj_missing"); ErrorCode(err) != CodeJobNotFound {
		t.Fatalf("missing job: %v", err)
	}
	if _, err := env.svc.ListJobs(ctx, "ccp_missing"); ErrorCode(err) != CodeNotFound {
		t.Fatalf("missing project: %v", err)
	}
	data, _ := json.Marshal(jobs[0])
	if strings.Contains(string(data), `"prompt"`) || !strings.Contains(string(data), `"plan":null`) {
		t.Fatalf("job JSON = %s", data)
	}
}

func TestEmitStepIsBounded(t *testing.T) {
	env, p := newProjectEnv(t)
	events, cancel := env.svc.Subscribe(p.ID)
	defer cancel()
	env.svc.emitStep(p.ID, "ccj_1", "reasoning", "line one\nline two "+strings.Repeat("x", 300))
	ev := collectEvents(t, events, 1, 2*time.Second, EventJobStep)[0]
	var d struct{ JobID, Kind, Text string }
	_ = json.Unmarshal(ev.Data, &d)
	if d.Kind != StepInfo || strings.Contains(d.Text, "\n") || len([]rune(d.Text)) != 200 || !strings.HasPrefix(d.Text, "line one line two") {
		t.Fatalf("step = %+v", d)
	}
}

// newProjectEnv is a service with one project (no runner).
func newProjectEnv(t *testing.T, opts ...testServiceOptions) (*testEnv, ProjectSummary) {
	t.Helper()
	env := newTestService(t, opts...)
	p, err := env.svc.CreateProject(context.Background(), CreateProjectInput{Name: "Wetter", Board: "esp32-2432s028r", Theme: "night"})
	if err != nil {
		t.Fatal(err)
	}
	return env, p
}

func TestJobContextRoundTrip(t *testing.T) {
	ctx := WithJobContext(context.Background(), "ccj_9")
	if JobIDFromContext(ctx) != "ccj_9" || JobIDFromContext(context.Background()) != "" {
		t.Fatal("job context lost")
	}
}
