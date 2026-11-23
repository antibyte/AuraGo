package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"aurago/internal/chibisensei"
	"aurago/internal/config"

	"gopkg.in/yaml.v3"
)

// chibiReadyKaggle grants everything a Chibi run needs (contract U3) in the live config.
func chibiReadyKaggle(t *testing.T, s *Server) {
	t.Helper()
	cfg := *s.ConfigSnapshot()
	k := config.DefaultKaggleConfig()
	k.Enabled, k.ReadOnly = true, false
	k.AllowCreate, k.AllowRemoteExecute, k.AllowGPU, k.AllowInternet, k.AllowDownload, k.AllowDelete = true, true, true, true, true, true
	k.AllowedMachineShapes = []string{"NvidiaTeslaT4"}
	k.RemoteTimeoutMinutes = 240
	cfg.Kaggle = k
	cfg.Agent.AllowFilesystemWrite, cfg.Agent.AllowNetworkRequests = true, true
	s.replaceConfigSnapshot(&cfg)
}

// chibiReadyProject creates a qwen35 assistant project with 40 rows and a ready check result.
func chibiReadyProject(t *testing.T, s *Server, token string) string {
	t.Helper()
	pid := chibiNewProject(t, s, token, map[string]any{"name": "Bot", "recipe": "assistant", "profile_id": "qwen35-0p8b"})
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, "/projects/"+pid+"/rows", map[string]any{"rows": chibiPairs(40, true)}), http.StatusCreated, "")
	w := chibiDo(t, s, token, http.MethodPost, "/projects/"+pid+"/checks/run", nil)
	chibiWant(t, w, http.StatusOK, "")
	if chibiBody(t, w)["ready"] != true {
		t.Fatalf("the dataset is not ready: %s", w.Body.String())
	}
	return pid
}

type chibiPreflightAnswer struct {
	Ready     bool                        `json:"ready"`
	Admin     bool                        `json:"admin"`
	Preset    string                      `json:"preset"`
	CheckedAt string                      `json:"checked_at"`
	Items     []chibisensei.PreflightItem `json:"items"`
	Fix       *struct {
		Flags []string `json:"flags"`
	} `json:"fix"`
}

func chibiGetPreflight(t *testing.T, s *Server, token, pid string) chibiPreflightAnswer {
	t.Helper()
	w := chibiDo(t, s, token, http.MethodGet, "/projects/"+pid+"/preflight?preset=quick", nil)
	chibiWant(t, w, http.StatusOK, "")
	var a chibiPreflightAnswer
	if err := json.Unmarshal(w.Body.Bytes(), &a); err != nil {
		t.Fatal(err)
	}
	return a
}

func (a chibiPreflightAnswer) item(t *testing.T, id string) chibisensei.PreflightItem {
	t.Helper()
	for _, it := range a.Items {
		if it.ID == id {
			return it
		}
	}
	t.Fatalf("preflight has no item %q", id)
	return chibisensei.PreflightItem{}
}

func TestChibiSenseiPreflightKaggleDisabled(t *testing.T) {
	s, _, token := chibiAPIFixture(t)
	stub := chibiUseStubKaggle(t, s)
	pid := chibiReadyProject(t, s, token)
	pf := chibiGetPreflight(t, s, token, pid)
	if pf.Ready || pf.Preset != "quick" || len(pf.Items) != 22 || pf.CheckedAt == "" {
		t.Fatalf("preflight = %+v", pf)
	}
	if it := pf.item(t, "kaggle_enabled"); it.OK || it.Level != "block" || it.Flag != "kaggle.enabled" {
		t.Fatalf("kaggle_enabled = %+v", it)
	}
	if it := pf.item(t, "dataset_ready"); !it.OK {
		t.Fatalf("dataset_ready = %+v", it)
	}
	if it := pf.item(t, "kaggle_connection"); it.OK || it.Args["skipped"] != true {
		t.Fatalf("kaggle_connection = %+v", it)
	}
	if pf.Fix == nil || !slices.Contains(pf.Fix.Flags, "kaggle.enabled") {
		t.Fatalf("fix = %+v", pf.Fix)
	}
	if n := stub.calls("account"); n != 0 {
		t.Fatalf("the preflight called Kaggle %d times while Kaggle is off", n)
	}
}

func TestChibiSenseiPreflightReady(t *testing.T) {
	s, _, token := chibiAPIFixture(t)
	stub := chibiUseStubKaggle(t, s)
	chibiReadyKaggle(t, s)
	pid := chibiReadyProject(t, s, token)
	pf := chibiGetPreflight(t, s, token, pid)
	if !pf.Ready || pf.Fix != nil {
		t.Fatalf("preflight = %+v", pf)
	}
	for _, it := range pf.Items {
		if it.Level == "block" && !it.OK {
			t.Fatalf("block item %q fails: %+v", it.ID, it)
		}
	}
	if it := pf.item(t, "kaggle_connection"); it.Args["account"] != "tester" {
		t.Fatalf("kaggle_connection = %+v", it)
	}
	if it := pf.item(t, "kaggle_quota"); it.Level != "block" || !it.OK || it.Args["hours"] != float64(30) {
		t.Fatalf("kaggle_quota = %+v", it)
	}
	chibiGetPreflight(t, s, token, pid)
	chibiGetPreflight(t, s, token, pid)
	if n := stub.calls("account"); n != 1 {
		t.Fatalf("%d connection tests for three preflights, want 1 (cache)", n)
	}
	cfg := *s.ConfigSnapshot()
	cfg.ChibiSensei.Enabled = false
	s.replaceConfigSnapshot(&cfg)
	chibiWant(t, chibiDo(t, s, token, http.MethodGet, "/projects/"+pid+"/preflight?preset=quick", nil), http.StatusForbidden, "chibi_disabled")
}

func TestChibiSenseiPreflightEnable(t *testing.T) {
	s, _, writeToken := chibiAPIFixture(t)
	adminToken, _, err := s.TokenManager.Create("chibi admin", []string{"admin", desktopScopeRead, desktopScopeWrite}, nil)
	if err != nil {
		t.Fatal(err)
	}
	dataDir := s.ConfigSnapshot().Directories.DataDir
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	yamlText := "server:\n  host: 127.0.0.1\ndirectories:\n  data_dir: " + strconv.Quote(dataDir) +
		"\nvirtual_desktop:\n  enabled: true\nchibi_sensei:\n  enabled: true\nkaggle:\n  enabled: false\n"
	if err := os.WriteFile(cfgPath, []byte(yamlText), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := *s.ConfigSnapshot()
	cfg.ConfigPath = cfgPath
	loaded, err := config.Load(cfgPath) // the fixture snapshot is not loaded from a file; the save validates the whole snapshot
	if err != nil {
		t.Fatal(err)
	}
	cfg.LocalLLM = loaded.LocalLLM
	s.replaceConfigSnapshot(&cfg)

	// Same origin: a header-less caller needs a matching Origin (login off).
	open := *s.ConfigSnapshot()
	open.Auth.Enabled = false
	s.replaceConfigSnapshot(&open)
	body := `{"flags":["kaggle.allow_tpu"]}`
	chibiWant(t, chibiDo(t, s, "", http.MethodPost, "/preflight/enable", body, "Origin", "https://evil.example"), http.StatusForbidden, "csrf_check_failed")
	chibiWant(t, chibiDo(t, s, "", http.MethodPost, "/preflight/enable", body, "Origin", "http://example.com"), http.StatusUnprocessableEntity, "validation_failed")

	chibiWant(t, chibiDo(t, s, writeToken, http.MethodPost, "/preflight/enable", body), http.StatusForbidden, "admin_required")
	chibiWant(t, chibiDo(t, s, adminToken, http.MethodPost, "/preflight/enable", body), http.StatusUnprocessableEntity, "validation_failed")
	chibiWant(t, chibiDo(t, s, adminToken, http.MethodPost, "/preflight/enable", `{"flags":[]}`), http.StatusUnprocessableEntity, "validation_failed")

	w := chibiDo(t, s, adminToken, http.MethodPost, "/preflight/enable",
		`{"flags":["kaggle.enabled","kaggle.allow_gpu","kaggle.allowed_machine_shapes","kaggle.allow_gpu"]}`)
	chibiWant(t, w, http.StatusOK, "")
	applied, _ := chibiBody(t, w)["applied"].([]any)
	if want := []any{"kaggle.enabled", "kaggle.allow_gpu", "kaggle.allowed_machine_shapes"}; !slices.Equal(applied, want) {
		t.Fatalf("applied = %v, want %v", applied, want)
	}
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	var saved struct {
		Kaggle struct {
			Enabled  bool     `yaml:"enabled"`
			AllowGPU bool     `yaml:"allow_gpu"`
			Shapes   []string `yaml:"allowed_machine_shapes"`
		} `yaml:"kaggle"`
	}
	if err := yaml.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	if !saved.Kaggle.Enabled || !saved.Kaggle.AllowGPU || !slices.Equal(saved.Kaggle.Shapes, []string{"NvidiaTeslaT4"}) {
		t.Fatalf("saved kaggle section = %+v", saved.Kaggle)
	}
	k := s.ConfigSnapshot().Kaggle
	if !k.Enabled || !k.AllowGPU || !slices.Equal(k.AllowedMachineShapes, []string{"NvidiaTeslaT4"}) {
		t.Fatalf("live kaggle config = %+v", k)
	}
}

func TestChibiSenseiConsentAndMeta(t *testing.T) {
	s, _, token := chibiAPIFixture(t)
	chibiUseStubKaggle(t, s)
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, "/consents", map[string]any{"profile_id": "nope"}), http.StatusUnprocessableEntity, "profile_unknown")

	profile := func(id string) map[string]any {
		w := chibiDo(t, s, token, http.MethodGet, "/meta", nil)
		chibiWant(t, w, http.StatusOK, "")
		for _, p := range chibiBody(t, w)["profiles"].([]any) {
			if m := p.(map[string]any); m["id"] == id {
				return m
			}
		}
		t.Fatalf("meta has no profile %q", id)
		return nil
	}
	gemma := profile("gemma3-270m")["license"].(map[string]any)
	if gemma["accepted"] != false || gemma["version"] != "1.0" {
		t.Fatalf("gemma license = %v", gemma)
	}
	qwen := profile("qwen35-0p8b")
	if qwen["license"].(map[string]any)["accepted"] != true || len(qwen["methods"].([]any)) == 0 || qwen["max_seq_cap"].(float64) <= 0 {
		t.Fatalf("qwen profile = %v", qwen)
	}
	meta := chibiBody(t, chibiDo(t, s, token, http.MethodGet, "/meta", nil))
	tr := meta["training"].(map[string]any)
	if tr["machine_shape"] != "NvidiaTeslaT4" || tr["backend"] != "kaggle" || len(meta["run_states"].([]any)) != 13 {
		t.Fatalf("training = %v, run_states = %v", tr, meta["run_states"])
	}

	var first string
	for i := 0; i < 2; i++ {
		w := chibiDo(t, s, token, http.MethodPost, "/consents", map[string]any{"profile_id": "gemma3-270m"})
		chibiWant(t, w, http.StatusCreated, "")
		c := chibiBody(t, w)["consent"].(map[string]any)
		at, _ := c["accepted_at"].(string)
		if c["license_id"] != "gemma" || c["license_version"] != "1.0" || at == "" {
			t.Fatalf("consent = %v", c)
		}
		if i == 0 {
			first = at
		} else if at != first {
			t.Fatalf("a repeat changed accepted_at: %q then %q", first, at)
		}
	}
	if got := profile("gemma3-270m")["license"].(map[string]any)["accepted"]; got != true {
		t.Fatalf("gemma accepted after consent = %v", got)
	}
}
