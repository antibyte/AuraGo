package server

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/chibisensei"

	"github.com/google/uuid"
)

// chibiPrepare runs prepare and returns the plan token.
func chibiPrepare(t *testing.T, s *Server, token, pid string) string {
	t.Helper()
	w := chibiDo(t, s, token, http.MethodPost, "/projects/"+pid+"/runs/prepare", map[string]any{"preset": "quick"})
	chibiWant(t, w, http.StatusOK, "")
	tok, _ := chibiBody(t, w)["plan_token"].(string)
	if tok == "" {
		t.Fatalf("prepare: %s", w.Body.String())
	}
	return tok
}

func TestChibiSenseiRunPrepareAndStart(t *testing.T) {
	s, readToken, token := chibiAPIFixture(t)
	chibiUseStubKaggle(t, s)
	chibiReadyKaggle(t, s)
	pid := chibiReadyProject(t, s, token)
	base := "/projects/" + pid

	chibiWant(t, chibiDo(t, s, readToken, http.MethodPost, base+"/runs/prepare", map[string]any{"preset": "quick"}), http.StatusForbidden, "")
	w := chibiDo(t, s, token, http.MethodPost, base+"/runs/prepare", map[string]any{"preset": "quick"})
	chibiWant(t, w, http.StatusOK, "")
	var prep struct {
		PlanToken    string           `json:"plan_token"`
		ExpiresInS   int              `json:"expires_in_s"`
		RunID        string           `json:"run_id"`
		Plan         chibisensei.Plan `json:"plan"`
		Destinations struct {
			Owner, Dataset, Notebook string
			MachineShape             string `json:"machine_shape"`
			Private                  bool
		} `json:"destinations"`
		Data struct {
			Rows  chibisensei.PlanSplit `json:"rows"`
			Bytes int64                 `json:"bytes"`
		} `json:"data"`
		License struct {
			RequiresConsent bool `json:"requires_consent"`
			Accepted        bool `json:"accepted"`
		} `json:"license"`
		Time struct {
			Quota *float64 `json:"quota_hours_remaining"`
		} `json:"time"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &prep); err != nil {
		t.Fatal(err)
	}
	if prep.PlanToken == "" || prep.ExpiresInS != 900 || !chibisensei.ValidID(prep.RunID) ||
		prep.Destinations.Owner != "tester" || prep.Destinations.Dataset != "tester/"+chibisensei.DatasetSlug(prep.RunID) ||
		prep.Destinations.Notebook != "tester/"+chibisensei.NotebookSlug(pid, prep.RunID) || !prep.Destinations.Private ||
		prep.Data.Rows != prep.Plan.Split || prep.Data.Bytes <= 0 || prep.License.RequiresConsent || !prep.License.Accepted ||
		prep.Time.Quota == nil || *prep.Time.Quota != 30 {
		t.Fatalf("prepare = %s", w.Body.String())
	}

	// A row change after prepare expires the plan.
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, base+"/rows", map[string]any{"rows": chibiPairs(1, false)}), http.StatusCreated, "")
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, base+"/runs", map[string]any{"plan_token": prep.PlanToken}), http.StatusConflict, "plan_expired")
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, base+"/checks/run", nil), http.StatusOK, "")

	tok1, tok2 := chibiPrepare(t, s, token, pid), chibiPrepare(t, s, token, pid)
	w = chibiDo(t, s, token, http.MethodPost, base+"/runs", map[string]any{"plan_token": tok1})
	chibiWant(t, w, http.StatusAccepted, "")
	var started struct {
		Run chibisensei.RunView `json:"run"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	if started.Run.State != chibisensei.RunQueued || started.Run.ProjectID != pid || started.Run.LastState != chibisensei.RunQueued {
		t.Fatalf("started run = %s", w.Body.String())
	}
	// A token works once; a second prepared start meets the active run.
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, base+"/runs", map[string]any{"plan_token": tok1}), http.StatusConflict, "plan_expired")
	w = chibiDo(t, s, token, http.MethodPost, base+"/runs", map[string]any{"plan_token": tok2})
	chibiWant(t, w, http.StatusConflict, "run_active")
	if !strings.Contains(w.Body.String(), `"missing":["run_active"]`) {
		t.Fatalf("run_active refusal = %s", w.Body.String())
	}
	project := chibiBody(t, chibiDo(t, s, token, http.MethodGet, base, nil))["project"].(map[string]any)
	if steps := project["steps"].(map[string]any); steps["training"] != "in_progress" {
		t.Fatalf("steps = %v", steps)
	}

	// Read-only desktop: prepare and start are refused, cancel is not.
	cfg := *s.ConfigSnapshot()
	cfg.VirtualDesktop.ReadOnly = true
	s.replaceConfigSnapshot(&cfg)
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, base+"/runs/prepare", map[string]any{"preset": "quick"}), http.StatusForbidden, "desktop_readonly")
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, base+"/runs", map[string]any{"plan_token": "x"}), http.StatusForbidden, "desktop_readonly")
	w = chibiDo(t, s, token, http.MethodPost, "/runs/"+started.Run.ID+"/cancel", nil)
	chibiWant(t, w, http.StatusOK, "")
	var cancelled struct {
		Run chibisensei.RunView `json:"run"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &cancelled); err != nil || !cancelled.Run.CancelRequested || cancelled.Run.Cancel == nil {
		t.Fatalf("cancel = %v %s", err, w.Body.String())
	}
	chibiWant(t, chibiDo(t, s, token, http.MethodGet, "/runs/"+chibiFakeID, nil), http.StatusNotFound, "run_not_found")
}

func TestChibiSenseiRunPreflightRefusal(t *testing.T) {
	s, _, token := chibiAPIFixture(t)
	chibiUseStubKaggle(t, s)
	pid := chibiReadyProject(t, s, token)
	w := chibiDo(t, s, token, http.MethodPost, "/projects/"+pid+"/runs/prepare", map[string]any{"preset": "quick"})
	chibiWant(t, w, http.StatusConflict, "preflight_failed")
	if !strings.Contains(w.Body.String(), `"kaggle_enabled"`) {
		t.Fatalf("missing list = %s", w.Body.String())
	}
}

// chibiStoredRun creates a finished run of pid directly in the store, with metrics, one artifact file, a report and a
// log tail (B4 layout: models/<run>/model-q8_0.gguf, runs/<run>/out/log-tail.txt).
func chibiStoredRun(t *testing.T, rt *chibiSenseiRuntime, pid string) string {
	t.Helper()
	ctx := context.Background()
	id := uuid.NewString()
	if err := rt.store.CreateRun(ctx, chibisensei.Run{ID: id, ProjectID: pid, ProfileID: "qwen35-0p8b", Preset: "quick"}); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.store.UpdateRun(ctx, id, chibisensei.NewStateRunEvent(chibisensei.RunSucceeded), func(r *chibisensei.Run) error {
		r.State, r.Remote.LastState, r.Remote.Finished = chibisensei.RunSucceeded, chibisensei.RunCleaning, "2026-11-23T10:00:00Z"
		r.Remote.Stage = &chibisensei.RunStage{Name: "manifest", State: "done", Step: 20, MaxSteps: 20, Epoch: 1}
		r.Remote.Result = &chibisensei.RunResult{Steps: 20, Epochs: 1, StopReason: "done", GPU: "Tesla T4"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := rt.store.UpsertMetrics(ctx, id, []chibisensei.Metric{
		{Step: 10, Kind: "train", Loss: 2, TS: "2026-11-23T09:50:00Z"},
		{Step: 20, Kind: "train", Loss: 1, TS: "2026-11-23T09:55:00Z"}}); err != nil {
		t.Fatal(err)
	}
	model := []byte("GGUF-test")
	if err := os.MkdirAll(chibisensei.ModelDir(rt.dataDir, id), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chibisensei.ModelDir(rt.dataDir, id), "model-q8_0.gguf"), model, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(model)
	if err := rt.store.AddArtifact(ctx, chibisensei.Artifact{RunID: id, ProjectID: pid, Kind: "gguf_tuned",
		RelPath: "models/" + id + "/model-q8_0.gguf", Bytes: int64(len(model)), SHA256: hex.EncodeToString(sum[:])}); err != nil {
		t.Fatal(err)
	}
	if err := rt.store.SaveReport(ctx, id, "auto", chibisensei.Report{RunID: id, Kind: "auto", Samples: 20, SmallSample: true}); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(chibisensei.RunDir(rt.dataDir, id), "out")
	if err := os.MkdirAll(out, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "log-tail.txt"), []byte("step 20 done\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestChibiSenseiRunReadsAndDelete(t *testing.T) {
	s, _, token := chibiAPIFixture(t)
	chibiUseStubKaggle(t, s)
	pid := chibiNewProject(t, s, token, map[string]any{"name": "Bot", "recipe": "assistant", "profile_id": "qwen35-0p8b"})
	rt, release, err := s.acquireChibiSensei()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	id := chibiStoredRun(t, rt, pid)

	w := chibiDo(t, s, token, http.MethodGet, "/projects/"+pid+"/runs", nil)
	chibiWant(t, w, http.StatusOK, "")
	runs, _ := chibiBody(t, w)["runs"].([]any)
	if len(runs) != 1 || runs[0].(map[string]any)["id"] != id {
		t.Fatalf("run list = %s", w.Body.String())
	}
	if _, has := runs[0].(map[string]any)["hyper"]; has {
		t.Fatalf("the summary carries hyper: %s", w.Body.String())
	}

	w = chibiDo(t, s, token, http.MethodGet, "/runs/"+id, nil)
	chibiWant(t, w, http.StatusOK, "")
	run := chibiBody(t, w)["run"].(map[string]any)
	if run["state"] != "succeeded" || run["hyper"] == nil {
		t.Fatalf("run = %s", w.Body.String())
	}

	w = chibiDo(t, s, token, http.MethodGet, "/runs/"+id+"/metrics?since_step=15", nil)
	chibiWant(t, w, http.StatusOK, "")
	m := chibiBody(t, w)
	if pts, _ := m["metrics"].([]any); len(pts) != 1 || pts[0].(map[string]any)["step"] != float64(20) ||
		m["max_steps"] != float64(20) || m["live"] != "unknown" {
		t.Fatalf("metrics = %s", w.Body.String())
	}

	w = chibiDo(t, s, token, http.MethodGet, "/runs/"+id+"/events", nil)
	chibiWant(t, w, http.StatusOK, "")
	if evs, _ := chibiBody(t, w)["events"].([]any); len(evs) < 2 || evs[0].(map[string]any)["kind"] != "state" {
		t.Fatalf("events = %s", w.Body.String())
	}

	w = chibiDo(t, s, token, http.MethodGet, "/runs/"+id+"/artifacts", nil)
	chibiWant(t, w, http.StatusOK, "")
	arts, _ := chibiBody(t, w)["artifacts"].([]any)
	if len(arts) != 1 {
		t.Fatalf("artifacts = %s", w.Body.String())
	}
	art := arts[0].(map[string]any)
	if _, has := art["rel_path"]; art["name"] != "model-q8_0.gguf" || has {
		t.Fatalf("artifact = %v", art)
	}
	aid := art["id"].(string)

	w = chibiDo(t, s, token, http.MethodGet, "/runs/"+id+"/artifacts/"+aid+"/file", nil)
	chibiWant(t, w, http.StatusOK, "")
	if cd := w.Header().Get("Content-Disposition"); w.Body.String() != "GGUF-test" || !strings.Contains(cd, "attachment") ||
		!strings.Contains(cd, "chibi-"+chibisensei.Run12(id)) || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("file = %q %q", w.Body.String(), cd)
	}
	chibiWant(t, chibiDo(t, s, token, http.MethodGet, "/runs/"+id+"/artifacts/"+chibiFakeID+"/file", nil), http.StatusNotFound, "artifact_not_found")

	w = chibiDo(t, s, token, http.MethodGet, "/runs/"+id+"/report", nil)
	chibiWant(t, w, http.StatusOK, "")
	if rep, _ := chibiBody(t, w)["report"].(map[string]any); rep["samples"] != float64(20) {
		t.Fatalf("report = %s", w.Body.String())
	}

	w = chibiDo(t, s, token, http.MethodGet, "/runs/"+id+"/log", nil)
	chibiWant(t, w, http.StatusOK, "")
	if lg := chibiBody(t, w); lg["tail"] != "step 20 done\n" || lg["bytes"] != float64(13) {
		t.Fatalf("log = %s", w.Body.String())
	}

	chibiWant(t, chibiDo(t, s, token, http.MethodPost, "/runs/"+id+"/delete", nil), http.StatusNoContent, "")
	for _, dir := range []string{chibisensei.RunDir(rt.dataDir, id), chibisensei.ModelDir(rt.dataDir, id)} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("%s survived the delete: %v", dir, err)
		}
	}
	chibiWant(t, chibiDo(t, s, token, http.MethodGet, "/runs/"+id, nil), http.StatusNotFound, "run_not_found")

	// A run that has not ended cannot be deleted and blocks the project delete.
	other := uuid.NewString()
	ctx := context.Background()
	if err := rt.store.CreateRun(ctx, chibisensei.Run{ID: other, ProjectID: pid, ProfileID: "qwen35-0p8b", Preset: "quick"}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(chibisensei.RunDir(rt.dataDir, other), "data"), 0o700); err != nil {
		t.Fatal(err)
	}
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, "/runs/"+other+"/delete", nil), http.StatusConflict, "run_not_terminal")
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, "/projects/"+pid+"/archive", map[string]any{"archived": true}), http.StatusOK, "")
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, "/projects/"+pid+"/delete", map[string]any{"confirm_name": "Bot"}), http.StatusConflict, "run_active")
	if _, err := rt.store.UpdateRun(ctx, other, chibisensei.NewStateRunEvent(chibisensei.RunCancelled), func(r *chibisensei.Run) error {
		r.State = chibisensei.RunCancelled
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, "/projects/"+pid+"/delete", map[string]any{"confirm_name": "Bot"}), http.StatusNoContent, "")
	if _, err := os.Stat(chibisensei.RunDir(rt.dataDir, other)); !os.IsNotExist(err) {
		t.Fatalf("the run files of the deleted project survived: %v", err)
	}
}

func TestChibiSenseiEventsSendActiveRun(t *testing.T) {
	s, _, token := chibiAPIFixture(t)
	chibiUseStubKaggle(t, s)
	pid := chibiNewProject(t, s, token, map[string]any{"name": "Bot", "recipe": "assistant", "profile_id": "qwen35-0p8b"})
	rt, release, err := s.acquireChibiSensei()
	if err != nil {
		t.Fatal(err)
	}
	runID := uuid.NewString()
	err = rt.store.CreateRun(context.Background(), chibisensei.Run{ID: runID, ProjectID: pid, ProfileID: "qwen35-0p8b", Preset: "quick"})
	release()
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(handleChibiSensei(s))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+chibiSenseiAPIBase+"/projects/"+pid+"/events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	sc := bufio.NewScanner(res.Body)
	sawRows, event := false, ""
	for sc.Scan() {
		line := sc.Text()
		if name, ok := strings.CutPrefix(line, "event: "); ok {
			event = name
			sawRows = sawRows || name == "rows"
			continue
		}
		if event == "run" && strings.HasPrefix(line, "data: ") {
			if !sawRows || !strings.Contains(line, runID) {
				t.Fatalf("run event before rows or for another run: %s", line)
			}
			return
		}
	}
	t.Fatalf("the stream ended without a run event: %v", sc.Err())
}
