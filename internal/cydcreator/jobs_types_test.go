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
	five := make([]Attachment, maxAttachments+1)
	for i := range five {
		five[i] = Attachment{Kind: "log", Text: "x"}
	}
	if _, err := validAttachments(five); ErrorCode(err) != CodeBadRequest {
		t.Fatalf("%d attachments: %v", len(five), err)
	}
}

func TestParseJobPlanLimitsAndCharacters(t *testing.T) {
	reasonOf := func(t *testing.T, body string) string {
		t.Helper()
		_, err := ParseJobPlan([]byte(body))
		typed := wantCode(t, err, CodeBadRequest)
		reason, _ := typed.Details["reason"].(string)
		return reason
	}
	// Size: the decoded object is capped at 32 KiB, the raw input at 64 KiB
	// before a string-wrapped plan is unwrapped.
	padded := func(pad string, n int) string {
		return `{"summary":"x",` + strings.Repeat(pad, n) + `"screens":["a"],"wifi":"off"}`
	}
	if _, err := ParseJobPlan([]byte(padded(" ", 30<<10))); err != nil {
		t.Fatalf("plan below 32 KiB: %v", err)
	}
	if r := reasonOf(t, padded(" ", 32<<10)); !strings.Contains(r, "32 KiB") {
		t.Fatalf("plan above 32 KiB: reason %q", r)
	}
	wrapped, _ := json.Marshal(padded("\n", 20<<10)) // ~40 KiB once escaped, ~20 KiB decoded
	if len(wrapped) <= maxPlanBytes {
		t.Fatalf("wrapped test plan has only %d bytes", len(wrapped))
	}
	if _, err := ParseJobPlan(wrapped); err != nil {
		t.Fatalf("string-wrapped plan below the input cap: %v", err)
	}
	huge, _ := json.Marshal(padded("\n", 40<<10))
	if r := reasonOf(t, string(huge)); !strings.Contains(r, "32 KiB") {
		t.Fatalf("string-wrapped plan above 64 KiB: reason %q", r)
	}
	// Echoed model input is clipped in the message and in details.reason.
	longField := `{"summary":"x","screens":["a"],"wifi":"off","` + strings.Repeat("y", 5000) + `":1}`
	if r := reasonOf(t, longField); len(r) > maxDiagMessageBytes+len("…") {
		t.Fatalf("unknown field reason has %d bytes", len(r))
	}
	_, err := ParseJobPlan([]byte(`{"summary":"x","screens":["a"],"wifi":"off","libraries":["` + strings.Repeat("z", 5000) + `"]}`))
	if typed := wantCode(t, err, CodeBadRequest); len(typed.Message) > 200 {
		t.Fatalf("unknown library message has %d bytes", len(typed.Message))
	}
	// Libraries: no duplicates, never more entries than the catalog.
	if r := reasonOf(t, `{"summary":"x","screens":["a"],"wifi":"off","libraries":["arduinojson","arduinojson"]}`); !strings.Contains(r, "twice") {
		t.Fatalf("duplicate library: reason %q", r)
	}
	ids := make([]string, 0, len(Libraries())+1)
	for _, l := range Libraries() {
		ids = append(ids, l.ID)
	}
	ids = append(ids, ids[0])
	list, _ := json.Marshal(ids)
	if r := reasonOf(t, `{"summary":"x","screens":["a"],"wifi":"off","libraries":`+string(list)+`}`); !strings.Contains(r, "entries") {
		t.Fatalf("too many libraries: reason %q", r)
	}
	// Characters: the summary may span lines, entries may not; control,
	// format and separator characters are refused everywhere.
	plan, err := ParseJobPlan([]byte(`{"summary":"line one\nline two","screens":["a"],"wifi":"off"}`))
	if err != nil || !strings.Contains(plan.Summary, "\n") {
		t.Fatalf("multi-line summary = %q, %v", plan.Summary, err)
	}
	reasonOf(t, `{"summary":"a\tb","screens":["a"],"wifi":"off"}`)
	for name, r := range map[string]rune{"line separator": 0x2028, "paragraph separator": 0x2029,
		"bidi override": 0x202E, "zero width space": 0x200B, "byte order mark": 0xFEFF} {
		text, _ := json.Marshal("a" + string(r) + "b")
		if _, err := ParseJobPlan([]byte(`{"summary":"x","screens":[` + string(text) + `],"wifi":"off"}`)); ErrorCode(err) != CodeBadRequest {
			t.Errorf("%s in an entry: %v", name, err)
		}
		if _, err := ParseJobPlan([]byte(`{"summary":` + string(text) + `,"screens":["a"],"wifi":"off"}`)); ErrorCode(err) != CodeBadRequest {
			t.Errorf("%s in the summary: %v", name, err)
		}
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
	env.svc.postSystemMessage(p.ID, "ccj_1", "job_cancelled", nil)
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
	if _, err := env.svc.AddUserMessage(ctx, p.ID, "", "   ", nil); ErrorCode(err) != CodeBadRequest {
		t.Fatalf("empty message: %v", err)
	}
	if _, err := env.svc.AddUserMessage(ctx, p.ID, "", "", []Attachment{{Kind: "log", Text: " \n "}}); ErrorCode(err) != CodeBadRequest {
		t.Fatalf("empty message with a blank attachment: %v", err)
	}
	if m, err := env.svc.AddUserMessage(ctx, p.ID, "", "", []Attachment{{Kind: "log", Text: "Guru Meditation Error"}}); err != nil || m.Text != "" || len(m.Attachments) != 1 {
		t.Fatalf("attachment-only message = %+v, %v", m, err)
	}
}

func TestJobAndMessageListsAreCappedAndKeepInsertionOrder(t *testing.T) {
	env, p := newProjectEnv(t)
	ctx := context.Background()
	var jobIDs []string
	for i := 0; i < maxListedJobs+5; i++ {
		id := env.svc.ids.NewID("ccj")
		jobIDs = append(jobIDs, id)
		if err := env.svc.insertJob(ctx, Job{ID: id, ProjectID: p.ID, Mode: JobModeChange, Status: JobFailed,
			Phase: PhaseCoding, Prompt: "p", CreatedAt: formatTime(env.svc.now())}); err != nil {
			t.Fatal(err)
		}
	}
	jobs, err := env.svc.ListJobs(ctx, p.ID)
	if err != nil || len(jobs) != maxListedJobs || jobs[0].ID != jobIDs[len(jobIDs)-1] || jobs[maxListedJobs-1].ID != jobIDs[5] {
		t.Fatalf("jobs = %d, first %q, %v", len(jobs), jobs[0].ID, err)
	}
	// Same millisecond: insertion order wins over the (random) id order.
	same := "2099-01-01T00:00:00.000Z"
	for _, id := range []string{"ccj_zz", "ccj_aa"} {
		if err := env.svc.insertJob(ctx, Job{ID: id, ProjectID: p.ID, Mode: JobModeChange, Status: JobFailed,
			Phase: PhaseCoding, Prompt: "p", CreatedAt: same}); err != nil {
			t.Fatal(err)
		}
	}
	if jobs, err = env.svc.ListJobs(ctx, p.ID); err != nil || jobs[0].ID != "ccj_aa" || jobs[1].ID != "ccj_zz" {
		t.Fatalf("same-millisecond jobs = %q, %q, %v", jobs[0].ID, jobs[1].ID, err)
	}

	var msgIDs []string
	for i := 0; i < maxListedMessages+5; i++ {
		m, err := env.svc.addMessage(ctx, p.ID, "", MessageRoleSystem, "line", nil, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		msgIDs = append(msgIDs, m.ID)
	}
	list, err := env.svc.ListMessages(ctx, p.ID)
	if err != nil || len(list) != maxListedMessages || list[0].ID != msgIDs[5] || list[len(list)-1].ID != msgIDs[len(msgIDs)-1] {
		t.Fatalf("messages = %d, first %q, %v", len(list), list[0].ID, err)
	}
	for _, id := range []string{"ccm_zz", "ccm_aa"} {
		if _, err := env.svc.db.Exec(`INSERT INTO cc_messages(id,project_id,job_id,role,text,attachments,created_at)
			VALUES(?,?,'','system','x','[]',?)`, id, p.ID, same); err != nil {
			t.Fatal(err)
		}
	}
	if list, err = env.svc.ListMessages(ctx, p.ID); err != nil || list[len(list)-2].ID != "ccm_zz" || list[len(list)-1].ID != "ccm_aa" {
		t.Fatalf("same-millisecond messages end with %q, %q, %v", list[len(list)-2].ID, list[len(list)-1].ID, err)
	}
}

func TestSaveJobEmitsOnlyStoredStates(t *testing.T) {
	env, p := newProjectEnv(t)
	ctx := context.Background()
	events, cancel := env.svc.Subscribe(p.ID)
	defer cancel()
	j := Job{ID: "ccj_s", ProjectID: p.ID, Mode: JobModeChange, Status: JobQueued, Phase: PhaseQueued, Prompt: "p",
		CreatedAt: formatTime(env.svc.now())}
	if err := env.svc.insertJob(ctx, j); err != nil {
		t.Fatal(err)
	}
	if err := env.svc.saveJob(Job{ID: "ccj_gone", ProjectID: p.ID, Status: JobSucceeded}); ErrorCode(err) != CodeJobNotFound {
		t.Fatalf("saveJob of a missing job: %v", err)
	}
	j.Status, j.Phase = JobRunning, PhaseCoding
	if err := env.svc.saveJob(j); err != nil {
		t.Fatal(err)
	}
	ev := collectEvents(t, events, 1, 2*time.Second, EventJobStatus)[0]
	if ev.JobID != "ccj_s" || !strings.Contains(string(ev.Data), `"status":"running"`) {
		t.Fatalf("first job_status = %s / %s", ev.JobID, ev.Data)
	}
	if got, err := env.svc.GetJob(ctx, "ccj_s"); err != nil || got.Status != JobRunning || got.Phase != PhaseCoding {
		t.Fatalf("stored job = %+v, %v", got, err)
	}
}

func TestSetProjectLockOnUnknownProject(t *testing.T) {
	env, _ := newProjectEnv(t)
	if err := env.svc.setProjectLock(context.Background(), "ccp_missing", "ccj_1"); ErrorCode(err) != CodeNotFound {
		t.Fatalf("lock of an unknown project: %v", err)
	}
	var n int
	if err := env.svc.db.QueryRow(`SELECT COUNT(*) FROM cc_events WHERE project_id='ccp_missing'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("events of the unknown project = %d, %v", n, err)
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
	// An unknown code emits nothing; the next step is the first event.
	env.svc.emitStep(p.ID, "ccj_1", StepInfo, "no_such_code", nil)
	env.svc.emitStep(p.ID, "ccj_1", "reasoning", "build_failed_at", map[string]any{"n": 1,
		"location": "src/app/" + strings.Repeat("a", 300), "message": "line one\nline two " + strings.Repeat("x", 300)})
	ev := collectEvents(t, events, 1, 2*time.Second, EventJobStep)[0]
	var d struct {
		JobID string         `json:"job_id"`
		Kind  string         `json:"kind"`
		Code  string         `json:"code"`
		Vars  map[string]any `json:"vars"`
		Text  string         `json:"text"`
	}
	_ = json.Unmarshal(ev.Data, &d)
	message, _ := d.Vars["message"].(string)
	if d.JobID != "ccj_1" || d.Kind != StepInfo || d.Code != "build_failed_at" || strings.Contains(d.Text, "\n") || len([]rune(d.Text)) != 200 ||
		!strings.HasPrefix(d.Text, "Build 1 failed: src/app/aaa") || !strings.HasPrefix(message, "line one line two") || len([]rune(message)) != maxStepVarRunes {
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
