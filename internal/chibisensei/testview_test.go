package chibisensei

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testRevision = "0123456789abcdef0123456789abcdef01234567"

type modelFixture struct {
	t       *testing.T
	st      *Store
	dataDir string
	run     Run
}

// newModelFixture makes a succeeded run with a valid tuned GGUF (arch "gemma3") registered as an artifact.
func newModelFixture(t *testing.T) *modelFixture {
	t.Helper()
	ctx := context.Background()
	st := newTestStore(t)
	p, err := st.CreateProject(ctx, ProjectInput{Name: "A", Recipe: RecipeAssistant})
	if err != nil {
		t.Fatal(err)
	}
	r := newTestRun(t, st, p.ID)
	job := `{"recipe":"specialist","task_kind":"classify","labels":["pos","neg"],"profile":{"base":{"revision":"` +
		testRevision + `"}},"deps":{"llama_cpp":{"tag":"b9994"}}}`
	r, err = st.UpdateRun(ctx, r.ID, RunEvent{}, func(x *Run) error {
		x.State, x.JobJSON = RunSucceeded, []byte(job)
		x.Remote.GGUF = &GGUFInfo{Version: 3, Tensors: 1, Architecture: "gemma3"}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	fx := &modelFixture{t: t, st: st, dataDir: t.TempDir(), run: r}
	fx.put("models/"+r.ID+"/model-q8_0.gguf", fakeGGUF("gemma3"))
	fx.artifact("gguf_tuned", "models/"+r.ID+"/model-q8_0.gguf", int64(len(fakeGGUF("gemma3"))))
	return fx
}

func (fx *modelFixture) put(rel string, body []byte) {
	fx.t.Helper()
	p := filepath.Join(ChibiDir(fx.dataDir), filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		fx.t.Fatal(err)
	}
	if err := os.WriteFile(p, body, 0o600); err != nil {
		fx.t.Fatal(err)
	}
}

func (fx *modelFixture) artifact(kind, rel string, size int64) {
	fx.t.Helper()
	if err := fx.st.AddArtifact(context.Background(), Artifact{RunID: fx.run.ID, ProjectID: fx.run.ProjectID, Kind: kind,
		RelPath: rel, Bytes: size, SHA256: strings.Repeat("a", 64)}); err != nil {
		fx.t.Fatal(err)
	}
}

func (fx *modelFixture) models() (ModelFile, ModelFile) {
	fx.t.Helper()
	tuned, base, err := RunModels(context.Background(), fx.st, fx.dataDir, fx.run)
	if err != nil {
		fx.t.Fatal(err)
	}
	return tuned, base
}

func TestRunModels(t *testing.T) {
	tunedRel := func(fx *modelFixture) string { return "models/" + fx.run.ID + "/model-q8_0.gguf" }

	t.Run("tuned available and base skipped", func(t *testing.T) {
		fx := newModelFixture(t)
		tuned, base := fx.models()
		want := filepath.Join(ChibiDir(fx.dataDir), filepath.FromSlash(tunedRel(fx)))
		if !tuned.Available || tuned.Reason != "" || tuned.Path != want || tuned.Key != "tuned:"+fx.run.ID ||
			tuned.Bytes != int64(len(fakeGGUF("gemma3"))) {
			t.Errorf("tuned = %+v", tuned)
		}
		if base.Available || base.Reason != "skipped" || base.Path != "" {
			t.Errorf("base = %+v", base)
		}
		b, _ := json.Marshal(tuned)
		if strings.Contains(string(b), "tuned:") || strings.Contains(string(b), fx.dataDir) {
			t.Errorf("key or path leaks into JSON: %s", b)
		}
	})
	t.Run("wrong size is missing", func(t *testing.T) {
		fx := newModelFixture(t)
		fx.artifact("gguf_tuned", tunedRel(fx), 7)
		if tuned, _ := fx.models(); tuned.Available || tuned.Reason != "missing" {
			t.Errorf("tuned = %+v", tuned)
		}
	})
	t.Run("bad header is invalid", func(t *testing.T) {
		fx := newModelFixture(t)
		bad := []byte(strings.Repeat("x", 64))
		fx.put(tunedRel(fx), bad)
		fx.artifact("gguf_tuned", tunedRel(fx), int64(len(bad)))
		if tuned, _ := fx.models(); tuned.Available || tuned.Reason != "invalid" {
			t.Errorf("tuned = %+v", tuned)
		}
	})
	t.Run("architecture mismatch is invalid", func(t *testing.T) {
		fx := newModelFixture(t)
		other := fakeGGUF("llama")
		fx.put(tunedRel(fx), other)
		fx.artifact("gguf_tuned", tunedRel(fx), int64(len(other)))
		if tuned, _ := fx.models(); tuned.Available || tuned.Reason != "invalid" {
			t.Errorf("tuned = %+v", tuned)
		}
	})
	t.Run("symlink is invalid", func(t *testing.T) {
		fx := newModelFixture(t)
		real := filepath.Join(fx.dataDir, "outside.gguf")
		if err := os.WriteFile(real, fakeGGUF("gemma3"), 0o600); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(ChibiDir(fx.dataDir), filepath.FromSlash(tunedRel(fx)))
		if err := os.Remove(link); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(real, link); err != nil {
			t.Skipf("no symlink: %v", err)
		}
		if tuned, _ := fx.models(); tuned.Available || tuned.Reason != "invalid" {
			t.Errorf("tuned = %+v", tuned)
		}
	})
	t.Run("no artifact row and no chibi directory", func(t *testing.T) {
		fx := newModelFixture(t)
		if err := os.RemoveAll(ChibiDir(fx.dataDir)); err != nil {
			t.Fatal(err)
		}
		tuned, base := fx.models()
		if tuned.Available || tuned.Reason != "missing" || base.Reason != "skipped" {
			t.Errorf("tuned %+v base %+v", tuned, base)
		}
		if err := fx.st.deleteArtifact(context.Background(), ArtifactID(fx.run.ID, "gguf_tuned")); err != nil {
			t.Fatal(err)
		}
		if tuned, _ := fx.models(); tuned.Reason != "missing" {
			t.Errorf("no row: %+v", tuned)
		}
	})
	t.Run("invalid registered path", func(t *testing.T) {
		if got := checkModel(nil, "x", "../escape.gguf", -1, "", "k"); got.Reason != "invalid" || got.Available {
			t.Errorf("escape = %+v", got)
		}
	})
	t.Run("base from the run artifact", func(t *testing.T) {
		fx := newModelFixture(t)
		rel := "runs/" + fx.run.ID + "/base.gguf"
		body := fakeGGUF("gemma3")
		fx.put(rel, body)
		fx.artifact("gguf_base", rel, int64(len(body)))
		_, base := fx.models()
		if !base.Available || base.Key != "base:"+rel || base.Bytes != int64(len(body)) {
			t.Errorf("base = %+v", base)
		}
	})
	t.Run("base from the shared row, then gone, then wrong architecture", func(t *testing.T) {
		fx := newModelFixture(t)
		rel := "base/gemma3-270m-" + testRevision[:12] + "-llamacpp-b9994.gguf"
		fx.put(rel, fakeGGUF("gemma3"))
		row := BaseGGUF{ProfileID: fx.run.ProfileID, BaseRevision: testRevision, Converter: BaseGGUFConverter("b9994"),
			RelPath: rel, SHA256: strings.Repeat("b", 64)}
		if err := fx.st.PutBaseGGUF(context.Background(), row); err != nil {
			t.Fatal(err)
		}
		if _, base := fx.models(); !base.Available || base.Key != "base:"+rel {
			t.Errorf("shared base = %+v", base)
		}
		fx.put(rel, fakeGGUF("llama"))
		if _, base := fx.models(); base.Available || base.Reason != "invalid" {
			t.Errorf("other architecture = %+v", base)
		}
		if err := os.Remove(filepath.Join(ChibiDir(fx.dataDir), filepath.FromSlash(rel))); err != nil {
			t.Fatal(err)
		}
		if _, base := fx.models(); base.Available || base.Reason != "missing" {
			t.Errorf("file gone = %+v", base)
		}
	})
	t.Run("unparsable job.json means no base", func(t *testing.T) {
		fx := newModelFixture(t)
		run, err := fx.st.UpdateRun(context.Background(), fx.run.ID, RunEvent{}, func(x *Run) error {
			x.JobJSON = []byte(`"not an object"`)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		fx.run = run
		if tuned, base := fx.models(); !tuned.Available || base.Reason != "skipped" {
			t.Errorf("tuned %+v base %+v", tuned, base)
		}
	})
	t.Run("a failed run", func(t *testing.T) {
		fx := newModelFixture(t)
		fx.run = setRunState(t, fx.st, fx.run.ID, RunFailed)
		if _, _, err := RunModels(context.Background(), fx.st, fx.dataDir, fx.run); !errors.Is(err, ErrRunNotSucceeded) {
			t.Errorf("err = %v", err)
		}
	})
}

func TestRunTask(t *testing.T) {
	fx := newModelFixture(t)
	recipe, kind, labels, err := RunTask(fx.run)
	if err != nil || recipe != RecipeSpecialist || kind != "classify" || len(labels) != 2 {
		t.Errorf("RunTask = %q %q %v %v", recipe, kind, labels, err)
	}
	if _, _, _, err := RunTask(Run{JobJSON: []byte(`"x"`)}); err == nil {
		t.Error("bad job.json accepted")
	}
}

func TestMarkReportSeen(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	p, _ := st.CreateProject(ctx, ProjectInput{Name: "A", Recipe: RecipeAssistant})
	r := newTestRun(t, st, p.ID)
	if err := st.MarkReportSeen(ctx, r.ID); !errors.Is(err, ErrRunNotSucceeded) {
		t.Fatalf("queued run: %v", err)
	}
	if err := st.MarkReportSeen(ctx, "00000000-0000-4000-8000-000000000000"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown run: %v", err)
	}
	setRunState(t, st, r.ID, RunSucceeded)
	step := func() string {
		t.Helper()
		got, err := st.GetProject(ctx, p.ID)
		if err != nil {
			t.Fatal(err)
		}
		return got.Steps.Test
	}
	if got := step(); got != "ready" {
		t.Fatalf("before = %q", got)
	}
	before, _ := st.ListRunEvents(ctx, r.ID, 0, 1000)
	if err := st.MarkReportSeen(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.MarkReportSeen(ctx, r.ID); err != nil {
		t.Fatalf("second call: %v", err)
	}
	after, _ := st.ListRunEvents(ctx, r.ID, 0, 1000)
	if len(after) != len(before) {
		t.Errorf("events %d -> %d", len(before), len(after))
	}
	if got := step(); got != "done" {
		t.Fatalf("after = %q", got)
	}
	list, _ := st.ListProjects(ctx, false)
	if len(list) != 1 || list[0].Steps.Test != "done" {
		t.Errorf("list steps = %+v", list)
	}
	got, _ := st.GetRun(ctx, r.ID)
	if !got.Remote.ReportSeen || !got.View().ReportSeen {
		t.Error("flag not stored")
	}
}

func TestRunViewSmokeAndReportSeen(t *testing.T) {
	r := Run{ID: "r1", ProjectID: "p1"}
	for name, b := range map[string]any{"view": r.View(), "summary": r.Summary()} {
		raw, _ := json.Marshal(b)
		if !strings.Contains(string(raw), `"smoke":null`) || !strings.Contains(string(raw), `"report_seen":false`) {
			t.Errorf("%s = %s", name, raw)
		}
	}
	r.Remote.Smoke = &SmokeResult{State: "ok", MS: 812, Text: "Hello"}
	r.Remote.ReportSeen = true
	for name, b := range map[string]any{"view": r.View(), "summary": r.Summary()} {
		raw, _ := json.Marshal(b)
		if !strings.Contains(string(raw), `"smoke":{"state":"ok","ms":812,"text":"Hello"}`) ||
			!strings.Contains(string(raw), `"report_seen":true`) {
			t.Errorf("%s = %s", name, raw)
		}
	}
	// The view holds a copy: changing it must not change the run.
	v := r.View()
	v.Smoke.State = "failed"
	if r.Remote.Smoke.State != "ok" {
		t.Error("view shares the smoke pointer")
	}
}

func TestProfileSummarySamplingAndTokens(t *testing.T) {
	prof, ok := mustCatalog(t).Get("qwen35-0p8b")
	if !ok {
		t.Fatal("profile")
	}
	s := prof.Summary()
	if s.Sampling != (ProfileSampling{Temperature: 0.7, TopP: 0.8, TopK: 20, RepeatPenalty: 1.0, PresencePenalty: 1.5}) {
		t.Errorf("sampling = %+v", s.Sampling)
	}
	if s.MaxNewTokens.Assistant != 256 || s.MaxNewTokens.Specialist != 64 {
		t.Errorf("max_new_tokens = %+v", s.MaxNewTokens)
	}
	raw, _ := json.Marshal(s)
	if !strings.Contains(string(raw), `"max_new_tokens":{"assistant":256,"specialist":64}`) ||
		!strings.Contains(string(raw), `"sampling":{"temperature":0.7,"top_p":0.8,"top_k":20,"repeat_penalty":1,"presence_penalty":1.5}`) {
		t.Errorf("json = %s", raw)
	}
}
