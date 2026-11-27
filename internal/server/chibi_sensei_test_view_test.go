package server

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"aurago/internal/chibisensei"
	"aurago/internal/chibisensei/chatrt/chatrttest"
)

const chibiTestRevision = "0123456789abcdef0123456789abcdef01234567"

// chibiTinyGGUF is a minimal valid GGUF v3 file: one tensor, the architecture key and the aligned tensor data.
func chibiTinyGGUF(arch string) []byte {
	var b bytes.Buffer
	w := func(v any) { _ = binary.Write(&b, binary.LittleEndian, v) }
	str := func(s string) { w(uint64(len(s))); b.WriteString(s) }
	b.WriteString("GGUF")
	w(uint32(3))
	w(uint64(1))
	w(uint64(2))
	str("general.architecture")
	w(uint32(8))
	str(arch)
	str("general.name")
	w(uint32(8))
	str("chibi-fake")
	str("t")
	w(uint32(1))
	w(uint64(1))
	w(uint32(0))
	w(uint64(0))
	for b.Len()%32 != 0 {
		b.WriteByte(0)
	}
	w(float32(0))
	return b.Bytes()
}

func chibiPutDataFile(t *testing.T, dataDir, rel string, body []byte) {
	t.Helper()
	p := filepath.Join(chibisensei.ChibiDir(dataDir), filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, body, 0o600); err != nil {
		t.Fatal(err)
	}
}

// chibiSucceededTestRun creates a succeeded assistant run of pid with a real tiny tuned GGUF, optionally the shared
// base GGUF with its artifact, an evaluation examples file with the given number of lines (the first one differs
// between base and tuned) and a report.
func chibiSucceededTestRun(t *testing.T, rt *chibiSenseiRuntime, pid, profileID string, withBase bool, samples int) string {
	t.Helper()
	return chibiSucceededTestRunRecipe(t, rt, pid, profileID, "assistant", withBase, samples)
}

func chibiSucceededTestRunRecipe(t *testing.T, rt *chibiSenseiRuntime, pid, profileID, recipe string, withBase bool, samples int) string {
	t.Helper()
	ctx := context.Background()
	const arch = "chibiarch"
	id := uuid.NewString()
	if err := rt.store.CreateRun(ctx, chibisensei.Run{ID: id, ProjectID: pid, ProfileID: profileID, Preset: "quick"}); err != nil {
		t.Fatal(err)
	}
	job := `{"recipe":"` + recipe + `","task_kind":null,"labels":[],"profile":{"base":{"revision":"` + chibiTestRevision +
		`"}},"deps":{"llama_cpp":{"tag":"b9994"}}}`
	if _, err := rt.store.UpdateRun(ctx, id, chibisensei.NewStateRunEvent(chibisensei.RunSucceeded), func(r *chibisensei.Run) error {
		r.State, r.Remote.LastState, r.Remote.Finished = chibisensei.RunSucceeded, chibisensei.RunCleaning, "2026-11-23T10:00:00Z"
		r.JobJSON = []byte(job)
		r.Remote.GGUF = &chibisensei.GGUFInfo{Version: 3, Tensors: 1, Architecture: arch}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	tuned := chibiTinyGGUF(arch)
	tunedRel := "models/" + id + "/model-q8_0.gguf"
	chibiPutDataFile(t, rt.dataDir, tunedRel, tuned)
	if err := rt.store.AddArtifact(ctx, chibisensei.Artifact{RunID: id, ProjectID: pid, Kind: "gguf_tuned", RelPath: tunedRel,
		Bytes: int64(len(tuned)), SHA256: strings.Repeat("a", 64)}); err != nil {
		t.Fatal(err)
	}
	if withBase {
		baseRel := "base/" + profileID + "-" + chibiTestRevision[:12] + "-llamacpp-b9994.gguf"
		chibiPutDataFile(t, rt.dataDir, baseRel, tuned)
		if err := rt.store.AddArtifact(ctx, chibisensei.Artifact{RunID: id, ProjectID: pid, Kind: "gguf_base", RelPath: baseRel,
			Bytes: int64(len(tuned)), SHA256: strings.Repeat("b", 64)}); err != nil {
			t.Fatal(err)
		}
	}
	if samples > 0 {
		var buf bytes.Buffer
		for i := 0; i < samples; i++ {
			base, tunedOut := "same answer", "same answer"
			if i == 0 {
				base, tunedOut = "base answer", "tuned answer"
			}
			line, _ := json.Marshal(map[string]any{"id": fmt.Sprint(i),
				"prompt_messages": []map[string]string{{"role": "user", "content": fmt.Sprintf("question %d", i+1)}},
				"expected":        "expected answer", "base_output": base, "tuned_output": tunedOut, "base_ms": 10, "tuned_ms": 12, "truncated": false})
			buf.Write(line)
			buf.WriteByte('\n')
		}
		chibiPutDataFile(t, rt.dataDir, "runs/"+id+"/out/eval_samples.jsonl", buf.Bytes())
	}
	if err := rt.store.SaveReport(ctx, id, "auto", chibisensei.Report{RunID: id, Kind: "auto", Samples: samples, SmallSample: true}); err != nil {
		t.Fatal(err)
	}
	return id
}

// chibiTestFixture is the API fixture with the fake chat launcher and one project.
func chibiTestFixture(t *testing.T, installed bool) (*Server, string, *chatrttest.Launcher, *chibiSenseiRuntime, string) {
	t.Helper()
	s, _, token := chibiAPIFixture(t)
	fake := chatrttest.New(installed)
	s.chibiSenseiLauncherOverride = fake
	chibiUseStubKaggle(t, s)
	pid := chibiNewProject(t, s, token, map[string]any{"name": "Bot", "recipe": "assistant", "profile_id": "qwen35-0p8b"})
	rt, release, err := s.acquireChibiSensei()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	return s, token, fake, rt, pid
}

func TestChibiSenseiTestView(t *testing.T) {
	s, token, _, rt, pid := chibiTestFixture(t, true)
	failed := chibiNewFailedRun(t, rt, pid)
	chibiWant(t, chibiDo(t, s, token, http.MethodGet, "/runs/"+failed+"/test", nil), http.StatusConflict, "run_not_succeeded")
	chibiWant(t, chibiDo(t, s, token, http.MethodGet, "/runs/"+chibiFakeID+"/test", nil), http.StatusNotFound, "run_not_found")

	withBase := chibiSucceededTestRun(t, rt, pid, "qwen35-0p8b", true, 3)
	w := chibiDo(t, s, token, http.MethodGet, "/runs/"+withBase+"/test", nil)
	chibiWant(t, w, http.StatusOK, "")
	body := chibiBody(t, w)
	models := body["models"].(map[string]any)
	wantBytes := float64(len(chibiTinyGGUF("chibiarch")))
	for _, kind := range []string{"tuned", "base"} {
		m := models[kind].(map[string]any)
		if m["available"] != true || m["bytes"] != wantBytes || m["reason"] != "" {
			t.Fatalf("%s model = %v", kind, m)
		}
	}
	chat := body["chat"].(map[string]any)
	if chat["max_tokens"] != float64(256) || chat["system_role"] != "native" || chat["sampling"].(map[string]any)["top_k"] != float64(20) {
		t.Fatalf("chat = %v", chat)
	}
	if body["samples_total"] != float64(3) || body["report"] == nil || body["report_seen"] != false || body["run"].(map[string]any)["id"] != withBase {
		t.Fatalf("view = %s", w.Body.String())
	}
	for _, leak := range []string{`"path"`, `"key"`, "tuned:", "base:", rt.dataDir} {
		if strings.Contains(w.Body.String(), leak) {
			t.Fatalf("the test view leaks %q: %s", leak, w.Body.String())
		}
	}

	noBase := chibiSucceededTestRun(t, rt, pid, "gemma3-270m", false, 0)
	w = chibiDo(t, s, token, http.MethodGet, "/runs/"+noBase+"/test", nil)
	chibiWant(t, w, http.StatusOK, "")
	body = chibiBody(t, w)
	models = body["models"].(map[string]any)
	if b := models["base"].(map[string]any); b["available"] != false || b["reason"] != "skipped" {
		t.Fatalf("base = %v", b)
	}
	if body["samples_total"] != float64(0) {
		t.Fatalf("samples_total = %v without a file", body["samples_total"])
	}

	spec := chibiSucceededTestRunRecipe(t, rt, pid, "qwen35-0p8b", "specialist", false, 0)
	w = chibiDo(t, s, token, http.MethodGet, "/runs/"+spec+"/test", nil)
	chibiWant(t, w, http.StatusOK, "")
	if got := chibiBody(t, w)["chat"].(map[string]any)["max_tokens"]; got != float64(64) {
		t.Fatalf("specialist max_tokens = %v, want 64", got)
	}
}

func TestChibiSenseiSamplesRoute(t *testing.T) {
	s, token, _, rt, pid := chibiTestFixture(t, true)
	id := chibiSucceededTestRun(t, rt, pid, "qwen35-0p8b", true, 5)

	w := chibiDo(t, s, token, http.MethodGet, "/runs/"+id+"/samples?limit=2", nil)
	chibiWant(t, w, http.StatusOK, "")
	var page chibisensei.SamplePage
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil || len(page.Samples) != 2 || page.Total != 5 || page.Limit != 2 {
		t.Fatalf("page = %v %+v", err, page)
	}
	w = chibiDo(t, s, token, http.MethodGet, "/runs/"+id+"/samples?filter=different", nil)
	chibiWant(t, w, http.StatusOK, "")
	page = chibisensei.SamplePage{}
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil || len(page.Samples) != 1 || page.Total != 1 || page.Samples[0].Index != 0 {
		t.Fatalf("different page = %v %+v", err, page)
	}
	for _, q := range []string{"limit=0", "limit=51", "filter=x", "offset=-1"} {
		w = chibiDo(t, s, token, http.MethodGet, "/runs/"+id+"/samples?"+q, nil)
		chibiWant(t, w, http.StatusUnprocessableEntity, "validation_failed")
		if !strings.Contains(w.Body.String(), "chibi_sensei.chat.invalid") {
			t.Fatalf("%s: issue key = %s", q, w.Body.String())
		}
	}
	chibiWant(t, chibiDo(t, s, token, http.MethodGet, "/runs/"+id+"/samples?limit=abc", nil), http.StatusBadRequest, "invalid_request")
	noFile := chibiSucceededTestRun(t, rt, pid, "qwen35-0p8b", false, 0)
	chibiWant(t, chibiDo(t, s, token, http.MethodGet, "/runs/"+noFile+"/samples", nil), http.StatusNotFound, "samples_missing")
	chibiWant(t, chibiDo(t, s, token, http.MethodGet, "/runs/"+chibiFakeID+"/samples", nil), http.StatusNotFound, "run_not_found")
}

func TestChibiSenseiReportSeen(t *testing.T) {
	s, token, _, rt, pid := chibiTestFixture(t, true)
	id := chibiSucceededTestRun(t, rt, pid, "qwen35-0p8b", false, 1)
	steps := func() map[string]any {
		return chibiBody(t, chibiDo(t, s, token, http.MethodGet, "/projects/"+pid, nil))["project"].(map[string]any)["steps"].(map[string]any)
	}
	if got := steps()["test"]; got != "ready" {
		t.Fatalf("steps.test before = %v, want ready", got)
	}
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, "/runs/"+id+"/report/seen", nil), http.StatusNoContent, "")
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, "/runs/"+id+"/report/seen", nil), http.StatusNoContent, "")
	if got := steps()["test"]; got != "done" {
		t.Fatalf("steps.test after = %v, want done", got)
	}
	if run := chibiBody(t, chibiDo(t, s, token, http.MethodGet, "/runs/"+id, nil))["run"].(map[string]any); run["report_seen"] != true {
		t.Fatalf("run = %v", run)
	}

	failed := chibiNewFailedRun(t, rt, pid)
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, "/runs/"+failed+"/report/seen", nil), http.StatusConflict, "run_not_succeeded")
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, "/runs/"+chibiFakeID+"/report/seen", nil), http.StatusNotFound, "run_not_found")

	// Default write classification: a read-only desktop refuses it (amendment A13).
	cfg := *s.ConfigSnapshot()
	cfg.VirtualDesktop.ReadOnly = true
	s.replaceConfigSnapshot(&cfg)
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, "/runs/"+id+"/report/seen", nil), http.StatusForbidden, "desktop_readonly")
}

func TestChibiSenseiMetaRuntimeAndChat(t *testing.T) {
	s, token, _, _, _ := chibiTestFixture(t, true)
	w := chibiDo(t, s, token, http.MethodGet, "/meta", nil)
	chibiWant(t, w, http.StatusOK, "")
	body := chibiBody(t, w)
	rtm := body["runtime"].(map[string]any)
	if rtm["build"] != "b9994" || rtm["enabled"] != true || rtm["backend"] != "auto" {
		t.Fatalf("runtime = %v", rtm)
	}
	chat := body["chat"].(map[string]any)
	if chat["max_targets"] != float64(2) || chat["max_turns"] != float64(20) || chat["max_chars"] != float64(16000) || chat["max_tokens_cap"] != float64(1024) {
		t.Fatalf("chat = %v", chat)
	}
	found := false
	for _, p := range body["profiles"].([]any) {
		pm := p.(map[string]any)
		if pm["id"] == "qwen35-0p8b" {
			found = pm["sampling"].(map[string]any)["top_k"] == float64(20)
		}
	}
	if !found {
		t.Fatalf("profile sampling.top_k missing: %s", w.Body.String())
	}
}
