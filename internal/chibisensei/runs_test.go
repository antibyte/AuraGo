package chibisensei

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
)

func newTestRun(t *testing.T, st *Store, projectID string) Run {
	t.Helper()
	id := uuid.NewString()
	r := Run{ID: id, ProjectID: projectID, ProfileID: "gemma3-270m", ProfileSHA: "abc", Preset: "quick",
		Hyper: Hyper{Epochs: 1, LearningRate: 5e-5, PerDeviceBatch: 4, GradAccum: 4, MaxSeqLen: 512},
		Remote: RunRemote{ProfileStatus: "experimental", Method: "full", Split: PlanSplit{Train: 40, Eval: 10},
			EstimateMinutes: 30, SessionTimeoutMinutes: 60, Owner: "tester",
			DatasetSlug: DatasetSlug(id), NotebookSlug: NotebookSlug(projectID, id)}}
	if err := st.CreateRun(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetRun(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func setRunState(t *testing.T, st *Store, id, state string) Run {
	t.Helper()
	r, err := st.UpdateRun(context.Background(), id, NewStateRunEvent(state),
		func(r *Run) error { r.State = state; return nil })
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRunCreateOneActiveAndRoundTrip(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	p1, _ := st.CreateProject(ctx, ProjectInput{Name: "A", Recipe: RecipeAssistant})
	p2, _ := st.CreateProject(ctx, ProjectInput{Name: "B", Recipe: RecipeAssistant})
	r := newTestRun(t, st, p1.ID)
	if r.State != RunQueued || r.Backend != BackendKaggle || r.Remote.Live != LiveUnknown || r.JobJSON != nil ||
		len(r.Leftovers) != 0 || r.Hyper.MaxSeqLen != 512 || r.Remote.Method != "full" {
		t.Fatalf("round trip: %+v", r)
	}
	other := Run{ID: uuid.NewString(), ProjectID: p2.ID}
	if err := st.CreateRun(ctx, other); !errors.Is(err, ErrRunActive) {
		t.Fatalf("second active run: %v", err)
	}
	if err := st.CreateRun(ctx, Run{ID: uuid.NewString(), ProjectID: uuid.NewString()}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown project: %v", err)
	}
	active, _ := st.ActiveRuns(ctx)
	if len(active) != 1 || active[0].ID != r.ID {
		t.Fatalf("active: %+v", active)
	}
	setRunState(t, st, r.ID, RunFailed)
	if err := st.CreateRun(ctx, other); err != nil {
		t.Fatalf("after terminal: %v", err)
	}
	ev, _ := st.ListRunEvents(ctx, r.ID, 0, 0)
	if len(ev) != 2 || ev[0].Kind != "state" || string(ev[0].Data) != `{"to":"queued"}` {
		t.Fatalf("events: %+v", ev)
	}
}

func TestUpdateRunIdentityRollbackAndSilentUpdate(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	p, _ := st.CreateProject(ctx, ProjectInput{Name: "A", Recipe: RecipeAssistant})
	r := newTestRun(t, st, p.ID)
	got, err := st.UpdateRun(ctx, r.ID, RunEvent{}, func(x *Run) error {
		x.ID, x.ProjectID = "changed", "changed"
		x.Remote.LastLogSeq = 42
		x.Remote.PutStep(KaggleStep{Step: StepSaveKernel, RequestID: "req-1"})
		x.JobJSON = []byte(`{"v":1}`)
		return nil
	})
	if err != nil || got.ID != r.ID || got.ProjectID != p.ID {
		t.Fatalf("identity: %+v %v", got, err)
	}
	again, _ := st.GetRun(ctx, r.ID)
	if again.Remote.LastLogSeq != 42 || again.Remote.Step(StepSaveKernel) == nil || string(again.JobJSON) != `{"v":1}` {
		t.Fatalf("stored: %+v", again.Remote)
	}
	if ev, _ := st.ListRunEvents(ctx, r.ID, 0, 0); len(ev) != 1 {
		t.Fatalf("silent update wrote an event: %d", len(ev))
	}
	boom := errors.New("boom")
	if _, err := st.UpdateRun(ctx, r.ID, NewRunEvent("warn", nil), func(x *Run) error { x.State = RunFailed; return boom }); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if again, _ = st.GetRun(ctx, r.ID); again.State != RunQueued {
		t.Fatalf("rollback: %s", again.State)
	}
	if _, err := st.UpdateRun(ctx, r.ID, RunEvent{}, func(x *Run) error { x.State = "bogus"; return nil }); err == nil {
		t.Fatal("unknown state accepted")
	}
	if _, err := st.UpdateRun(ctx, uuid.NewString(), RunEvent{}, func(*Run) error { return nil }); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestRunJSONShape(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	p, _ := st.CreateProject(ctx, ProjectInput{Name: "A", Recipe: RecipeAssistant})
	r := newTestRun(t, st, p.ID)
	decode := func(v any) map[string]any {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		return m
	}
	full := decode(r)
	for _, k := range []string{"stage", "cancel", "result"} {
		if v, ok := full[k]; !ok || v != nil {
			t.Errorf("%s = %v (present %v), want null", k, v, ok)
		}
	}
	if l, ok := full["leftovers"].([]any); !ok || len(l) != 0 {
		t.Errorf("leftovers = %v", full["leftovers"])
	}
	remote := full["remote"].(map[string]any)
	if jobs, ok := remote["kaggle_jobs"].([]any); !ok || len(jobs) != 0 || full["hyper"] == nil {
		t.Errorf("full form: %v", full)
	}
	loss := 0.5
	r.CancelRequested = true
	r.Remote.Submitted = "2026-11-22T12:00:00Z"
	r.Remote.Result = &RunResult{Steps: 10, TrainLoss: &loss}
	r.Remote.PutStep(KaggleStep{Step: StepSaveKernel, JobID: "ab"})
	full = decode(r)
	c := full["cancel"].(map[string]any)
	if c["stop_url"] != "https://www.kaggle.com/code/tester/"+r.Remote.NotebookSlug || c["time_limit_minutes"].(float64) != 60 {
		t.Errorf("cancel = %v", c)
	}
	if res := full["result"].(map[string]any); res["versions"] == nil || res["eval_loss_base"] != nil {
		t.Errorf("result = %v", res)
	}
	sum := decode(r.Summary())
	if _, ok := sum["hyper"]; ok {
		t.Error("summary has hyper")
	}
	if _, ok := sum["remote"].(map[string]any)["kaggle_jobs"]; ok {
		t.Error("summary has kaggle_jobs")
	}
	if _, ok := sum["result"].(map[string]any)["versions"]; ok {
		t.Error("summary has result.versions")
	}
}

func TestRunLastStateAndEventShapes(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	p, _ := st.CreateProject(ctx, ProjectInput{Name: "A", Recipe: RecipeAssistant})
	r := newTestRun(t, st, p.ID)
	if r.Remote.LastState != RunQueued {
		t.Fatalf("last_state = %q", r.Remote.LastState)
	}
	b, _ := json.Marshal(r)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if m["last_state"] != "queued" {
		t.Errorf("run json last_state = %v", m["last_state"])
	}
	b, _ = json.Marshal(r.Summary())
	m = nil
	_ = json.Unmarshal(b, &m)
	if m["last_state"] != "queued" {
		t.Errorf("summary json last_state = %v", m["last_state"])
	}
	for _, c := range []struct {
		ev   RunEvent
		kind string
		want string
	}{
		{NewStateRunEvent("running"), "state", `{"to":"running"}`},
		{NewStageRunEvent("train", "start"), "stage", `{"stage":"train","state":"start"}`},
		{NewWarnRunEvent("c", "m"), "warn", `{"code":"c","message":"m"}`},
		{NewErrorRunEvent("s", "c", "m"), "error", `{"code":"c","message":"m","stage":"s"}`},
		{NewKaggleRunEvent("save_kernel", "j"), "kaggle", `{"job_id":"j","step":"save_kernel"}`},
		{NewLogRunEvent("x"), "log", `{"text":"x"}`},
	} {
		if c.ev.Kind != c.kind || string(c.ev.Data) != c.want {
			t.Errorf("%s: %s", c.ev.Kind, c.ev.Data)
		}
	}
}

func TestRunEventsCap(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	p, _ := st.CreateProject(ctx, ProjectInput{Name: "A", Recipe: RecipeAssistant})
	r := newTestRun(t, st, p.ID)
	old := maxRunEvents
	maxRunEvents = 10
	t.Cleanup(func() { maxRunEvents = old })
	for i := 0; i < 15; i++ {
		d, _ := json.Marshal(map[string]int{"n": i})
		if err := st.AddRunEvent(ctx, r.ID, RunEvent{Kind: "log", Data: d}); err != nil {
			t.Fatal(err)
		}
	}
	ev, _ := st.ListRunEvents(ctx, r.ID, 0, 0)
	if len(ev) != 10 || string(ev[0].Data) != `{"n":5}` {
		t.Fatalf("events: %d %+v", len(ev), ev)
	}
	for i := 1; i < len(ev); i++ {
		if ev[i].ID <= ev[i-1].ID {
			t.Fatal("ids not increasing")
		}
	}
	if more, _ := st.ListRunEvents(ctx, r.ID, ev[7].ID, 0); len(more) != 2 {
		t.Fatalf("after: %d", len(more))
	}
	if err := st.AddRunEvent(ctx, r.ID, RunEvent{Kind: "log", Data: []byte("{")}); err == nil {
		t.Fatal("invalid data accepted")
	}
}

func TestMetricsUpsertReplaceList(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	p, _ := st.CreateProject(ctx, ProjectInput{Name: "A", Recipe: RecipeAssistant})
	r := newTestRun(t, st, p.ID)
	if err := st.UpsertMetrics(ctx, r.ID, []Metric{{Step: 10, Kind: "train", Loss: 2}, {Step: 10, Kind: "eval", Loss: 2.5},
		{Step: 20, Kind: "train", Loss: math.NaN()}, {Step: 30, Kind: "bogus", Loss: 1}}); err != nil {
		t.Fatal(err)
	}
	if m, _ := st.ListMetrics(ctx, r.ID, 0); len(m) != 2 {
		t.Fatalf("points: %+v", m)
	}
	_ = st.UpsertMetrics(ctx, r.ID, []Metric{{Step: 10, Kind: "train", Loss: 1.5}})
	m, _ := st.ListMetrics(ctx, r.ID, 10)
	if len(m) != 2 || m[1].Kind != "train" || m[1].Loss != 1.5 {
		t.Fatalf("inclusive/update: %+v", m)
	}
	_ = st.ReplaceMetrics(ctx, r.ID, []Metric{{Step: 5, Kind: "train", Loss: 3}})
	if m, _ = st.ListMetrics(ctx, r.ID, 0); len(m) != 1 || m[0].Step != 5 {
		t.Fatalf("replace: %+v", m)
	}
	if err := st.UpsertMetrics(ctx, r.ID, nil); err != nil {
		t.Fatal(err)
	}
}

func TestArtifactsReportsConsentsSettings(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	p, _ := st.CreateProject(ctx, ProjectInput{Name: "A", Recipe: RecipeAssistant})
	r := newTestRun(t, st, p.ID)
	rel := "models/" + r.ID + "/model-q8_0.gguf"
	if err := st.AddArtifact(ctx, Artifact{RunID: r.ID, ProjectID: p.ID, Kind: "gguf_tuned", RelPath: rel, Bytes: 1}); err != nil {
		t.Fatal(err)
	}
	id := ArtifactID(r.ID, "gguf_tuned")
	if !ValidID(id) || id != ArtifactID(r.ID, "gguf_tuned") {
		t.Fatalf("id %q", id)
	}
	a, err := st.GetArtifact(ctx, id)
	if err != nil || a.Name != "model-q8_0.gguf" {
		t.Fatalf("artifact: %+v %v", a, err)
	}
	_ = st.AddArtifact(ctx, Artifact{RunID: r.ID, ProjectID: p.ID, Kind: "gguf_tuned", RelPath: rel, Bytes: 9})
	if l, _ := st.ListArtifacts(ctx, r.ID); len(l) != 1 || l[0].Bytes != 9 {
		t.Fatalf("replace: %+v", l)
	}
	for _, bad := range []Artifact{{Kind: "bogus", RelPath: "a"}, {Kind: "adapter", RelPath: "../x"}, {Kind: "adapter", RelPath: "a\x5cb"}} {
		bad.RunID, bad.ProjectID = r.ID, p.ID
		if st.AddArtifact(ctx, bad) == nil {
			t.Errorf("accepted %+v", bad)
		}
	}
	if _, err := st.GetArtifact(ctx, uuid.NewString()); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	_ = st.SaveReport(ctx, r.ID, "auto", map[string]any{"kind": "auto"})
	_ = st.SaveReport(ctx, r.ID, "auto", map[string]any{"kind": "auto", "samples": 3})
	body, ok, _ := st.GetReport(ctx, r.ID, "auto")
	var rep map[string]any
	_ = json.Unmarshal(body, &rep)
	if !ok || rep["samples"] != float64(3) {
		t.Fatalf("report: %s %v", body, ok)
	}
	if _, ok, _ := st.GetReport(ctx, r.ID, "judge"); ok {
		t.Fatal("judge report found")
	}
	if st.SaveReport(ctx, r.ID, "x", map[string]any{}) == nil {
		t.Fatal("kind x accepted")
	}
	c1, err := st.RecordConsent(ctx, "gemma3-270m", "gemma", "1.0")
	if err != nil {
		t.Fatal(err)
	}
	later := time.Date(2026, 11, 22, 13, 0, 0, 0, time.UTC)
	st.now = func() time.Time { return later }
	c2, _ := st.RecordConsent(ctx, "gemma3-270m", "gemma", "1.0")
	if c1.AcceptedAt != c2.AcceptedAt {
		t.Fatalf("consent not idempotent: %v %v", c1, c2)
	}
	if ok, _ := st.HasConsent(ctx, "gemma3-270m", "gemma", "1.0"); !ok {
		t.Fatal("no consent")
	}
	if ok, _ := st.HasConsent(ctx, "gemma3-270m", "gemma", "2.0"); ok {
		t.Fatal("consent for 2.0")
	}
	if l, _ := st.ListConsents(ctx); len(l) != 1 {
		t.Fatalf("consents: %v", l)
	}
	if _, ok, _ := st.GetSetting(ctx, "kaggle_project_id"); ok {
		t.Fatal("setting exists")
	}
	_ = st.SetSetting(ctx, "kaggle_project_id", "a")
	_ = st.SetSetting(ctx, "kaggle_project_id", "b")
	if v, ok, _ := st.GetSetting(ctx, "kaggle_project_id"); !ok || v != "b" {
		t.Fatalf("setting %q", v)
	}
}

func TestDeleteRunAndProjectGuards(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	p, _ := st.CreateProject(ctx, ProjectInput{Name: "A", Recipe: RecipeAssistant})
	r := newTestRun(t, st, p.ID)
	if err := st.DeleteRun(ctx, r.ID); !errors.Is(err, ErrRunNotTerminal) {
		t.Fatal(err)
	}
	if _, err := st.SetArchived(ctx, p.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteProject(ctx, p.ID, "A"); !errors.Is(err, ErrRunActive) {
		t.Fatalf("delete with active run: %v", err)
	}
	setRunState(t, st, r.ID, RunCancelled)
	_ = st.AddRunEvent(ctx, r.ID, NewLogRunEvent("x"))
	_ = st.UpsertMetrics(ctx, r.ID, []Metric{{Step: 1, Kind: "train", Loss: 1}})
	_ = st.AddArtifact(ctx, Artifact{RunID: r.ID, ProjectID: p.ID, Kind: "status", RelPath: "runs/x/chibi-status.json"})
	_ = st.SaveReport(ctx, r.ID, "auto", map[string]any{"kind": "auto"})
	if ids, _ := st.ListRunIDs(ctx, p.ID); len(ids) != 1 || ids[0] != r.ID {
		t.Fatalf("ids: %v", ids)
	}
	if err := st.DeleteRun(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetRun(ctx, r.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if ev, _ := st.ListRunEvents(ctx, r.ID, 0, 0); len(ev) != 0 {
		t.Fatal("events remain")
	}
	if m, _ := st.ListMetrics(ctx, r.ID, 0); len(m) != 0 {
		t.Fatal("metrics remain")
	}
	if a, _ := st.ListArtifacts(ctx, r.ID); len(a) != 0 {
		t.Fatal("artifacts remain")
	}
	if _, ok, _ := st.GetReport(ctx, r.ID, "auto"); ok {
		t.Fatal("report remains")
	}
	if err := st.DeleteRun(ctx, r.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := st.DeleteProject(ctx, p.ID, "A"); err != nil {
		t.Fatal(err)
	}
}

func TestComputeStepsTraining(t *testing.T) {
	p := Project{}
	p.DataRev, p.GoalText, p.ProfileID, p.Recipe = 3, "g", "gemma3-270m", RecipeAssistant
	rev := sql.NullInt64{Int64: 3, Valid: true}
	for _, c := range []struct {
		name  string
		ready bool
		runs  projectRuns
		want  string
	}{
		{"none ready", true, projectRuns{}, "ready"},
		{"none not ready", false, projectRuns{}, "todo"},
		{"active", false, projectRuns{Latest: RunRunning}, "in_progress"},
		{"failed", true, projectRuns{Latest: RunFailed}, "blocked"},
		{"cancelled", true, projectRuns{Latest: RunCancelled}, "blocked"},
		{"succeeded", true, projectRuns{Latest: RunSucceeded, Succeeded: true}, "done"},
		{"failed after success", true, projectRuns{Latest: RunFailed, Succeeded: true}, "done"},
		{"active after success", true, projectRuns{Latest: RunQueued, Succeeded: true}, "in_progress"},
	} {
		if got := computeSteps(p, 10, rev, c.ready, c.runs).Training; got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
}

func TestComputeStepsTest(t *testing.T) {
	p := Project{}
	p.DataRev, p.GoalText, p.ProfileID, p.Recipe = 3, "g", "gemma3-270m", RecipeAssistant
	rev := sql.NullInt64{Int64: 3, Valid: true}
	for _, c := range []struct {
		name string
		runs projectRuns
		want string
	}{
		{"todo without a run", projectRuns{}, "todo"},
		{"todo while running", projectRuns{Latest: RunRunning}, "todo"},
		{"todo after a failure", projectRuns{Latest: RunFailed}, "todo"},
		{"ready", projectRuns{Latest: RunSucceeded, Succeeded: true}, "ready"},
		{"ready after a later failure", projectRuns{Latest: RunFailed, Succeeded: true}, "ready"},
		{"done", projectRuns{Latest: RunSucceeded, Succeeded: true, ReportSeen: true}, "done"},
		{"seen flag alone is not enough", projectRuns{Latest: RunFailed, ReportSeen: true}, "todo"},
	} {
		st := computeSteps(p, 10, rev, true, c.runs)
		if st.Test != c.want || st.Use != "todo" {
			t.Errorf("%s: test %s use %s, want %s", c.name, st.Test, st.Use, c.want)
		}
	}
}

func TestProjectTrainingStepFromStore(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	p, _ := st.CreateProject(ctx, ProjectInput{Name: "A", Recipe: RecipeAssistant})
	r := newTestRun(t, st, p.ID)
	check := func(want string) {
		t.Helper()
		got, err := st.GetProject(ctx, p.ID)
		if err != nil || got.Steps.Training != want {
			t.Fatalf("get: %q want %q (%v)", got.Steps.Training, want, err)
		}
		l, _ := st.ListProjects(ctx, false)
		if len(l) != 1 || l[0].Steps.Training != want {
			t.Fatalf("list: %+v want %q", l, want)
		}
	}
	check("in_progress")
	setRunState(t, st, r.ID, RunFailed)
	check("blocked")
	r2 := newTestRun(t, st, p.ID)
	setRunState(t, st, r2.ID, RunSucceeded)
	check("done")
}
