package desktopstore

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// deepTutorPython finds a Python 3 interpreter; the merge script runs inside
// the DeepTutor image, so the test is skipped on hosts without one.
func deepTutorPython(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"python3", "python", "py"} {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		probe := exec.Command(path, "-I", "-c", "import sys; sys.exit(0 if sys.version_info >= (3, 8) else 1)")
		if probe.Run() == nil {
			return path
		}
	}
	t.Skip("no Python 3 interpreter on PATH")
	return ""
}

type mergeRun struct {
	python string
	dir    string
	env    map[string]string
}

func newMergeRun(t *testing.T) *mergeRun {
	return &mergeRun{python: deepTutorPython(t), dir: t.TempDir(), env: map[string]string{
		"AURAGO_DT_RELAY_URL":   "http://relay.test:8088/api/store-llm/deeptutor/v1",
		"AURAGO_DT_RELAY_TOKEN": "token-one-" + strings.Repeat("a", 32),
		"AURAGO_DT_VISION":      "1",
		"AURAGO_DT_EMBEDDINGS":  "0",
		"AURAGO_DT_CONTEXT":     "128000",
	}}
}

// run executes the embedded script exactly like the wrapper does
// (python -c "$AURAGO_DT_MERGE_PY") and returns stdout and stderr together.
func (m *mergeRun) run(t *testing.T) string {
	t.Helper()
	cmd := exec.Command(m.python, "-I", "-c", deepTutorMergeScript)
	cmd.Env = append(os.Environ(), "AURAGO_DT_SETTINGS_DIR="+m.dir)
	for key, value := range m.env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("merge script failed: %v\n%s", err, out)
	}
	if token := m.env["AURAGO_DT_RELAY_TOKEN"]; token != "" && strings.Contains(string(out), token) {
		t.Fatalf("merge script printed the relay token: %s", out)
	}
	return string(out)
}

func (m *mergeRun) catalogPath() string { return filepath.Join(m.dir, "model_catalog.json") }
func (m *mergeRun) markerPath() string  { return filepath.Join(m.dir, ".aurago-managed.json") }

func (m *mergeRun) catalog(t *testing.T) map[string]any {
	t.Helper()
	return readJSONFile(t, m.catalogPath())
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("%s is not valid JSON: %v\n%s", path, err, raw)
	}
	return out
}

func (m *mergeRun) writeCatalog(t *testing.T, catalog map[string]any) {
	t.Helper()
	raw, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(m.catalogPath(), raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func catalogService(catalog map[string]any, name string) map[string]any {
	services, _ := catalog["services"].(map[string]any)
	service, _ := services[name].(map[string]any)
	return service
}

func catalogProfile(catalog map[string]any, service, id string) map[string]any {
	profiles, _ := catalogService(catalog, service)["profiles"].([]any)
	for _, item := range profiles {
		if profile, _ := item.(map[string]any); profile["id"] == id {
			return profile
		}
	}
	return nil
}

func profileModel(profile map[string]any) map[string]any {
	models, _ := profile["models"].([]any)
	if len(models) == 0 {
		return nil
	}
	model, _ := models[0].(map[string]any)
	return model
}

const (
	mergeLLMProfile       = "llm-profile-a0a0a0a0"
	mergeLLMModel         = "llm-model-a0a0a0a0"
	mergeEmbeddingProfile = "embedding-profile-a0a0a0a0"
	mergeEmbeddingModel   = "embedding-model-a0a0a0a0"
)

func TestDeepTutorMergeScriptFreshInstallCreatesActiveManagedProfiles(t *testing.T) {
	// The merge asks the relay for one embedding to learn the dimension.
	var mu sync.Mutex
	var gotAuth, gotPath string
	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotAuth, gotPath = r.Header.Get("Authorization"), r.URL.Path
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.1,0.2,0.3,0.4]}]}`))
	}))
	defer relay.Close()

	m := newMergeRun(t)
	m.env["AURAGO_DT_EMBEDDINGS"] = "1"
	m.env["AURAGO_DT_RELAY_URL"] = relay.URL + DeepTutorRelayPath
	m.run(t)
	mu.Lock()
	if gotPath != DeepTutorRelayPath+"/embeddings" || gotAuth != "Bearer "+m.env["AURAGO_DT_RELAY_TOKEN"] {
		t.Fatalf("probe request = %q with auth %q", gotPath, gotAuth)
	}
	mu.Unlock()

	catalog := m.catalog(t)
	if catalog["version"] != float64(1) {
		t.Fatalf("version = %v", catalog["version"])
	}
	if connections, ok := catalog["connections"].([]any); !ok || len(connections) != 0 {
		t.Fatalf("connections = %#v", catalog["connections"])
	}
	for _, name := range []string{"llm", "task", "embedding", "stt", "tts", "imagegen", "videogen"} {
		service := catalogService(catalog, name)
		for _, key := range []string{"active_profile_id", "active_model_id", "profiles"} {
			if _, ok := service[key]; !ok {
				t.Fatalf("service %s lacks %s: %#v", name, key, service)
			}
		}
	}
	search := catalogService(catalog, "search")
	if _, hasModel := search["active_model_id"]; hasModel || search["profiles"] == nil {
		t.Fatalf("search service shape = %#v", search)
	}

	llm := catalogProfile(catalog, "llm", mergeLLMProfile)
	if llm == nil {
		t.Fatalf("managed llm profile missing: %#v", catalog)
	}
	for key, want := range map[string]any{
		"name": "AuraGo", "binding": "custom", "base_url": m.env["AURAGO_DT_RELAY_URL"], "api_key": m.env["AURAGO_DT_RELAY_TOKEN"],
		"api_version": "", "api_format": "openai_chat", "wire_api": "chat_completions",
	} {
		if llm[key] != want {
			t.Fatalf("llm profile %s = %#v, want %#v", key, llm[key], want)
		}
	}
	model := profileModel(llm)
	capabilities, _ := model["capabilities"].(map[string]any)
	if model["id"] != mergeLLMModel || model["name"] != "AuraGo" || model["model"] != "aurago" || model["context_window"] != float64(128000) ||
		capabilities["tools"] != true || capabilities["vision"] != true || capabilities["json_output"] != true || capabilities["reasoning"] != false {
		t.Fatalf("llm model = %#v", model)
	}
	if service := catalogService(catalog, "llm"); service["active_profile_id"] != mergeLLMProfile || service["active_model_id"] != mergeLLMModel {
		t.Fatalf("llm active = %v / %v", service["active_profile_id"], service["active_model_id"])
	}

	embedding := catalogProfile(catalog, "embedding", mergeEmbeddingProfile)
	if embedding == nil {
		t.Fatal("managed embedding profile missing although the relay offers embeddings")
	}
	embeddingModel := profileModel(embedding)
	if embedding["base_url"] != m.env["AURAGO_DT_RELAY_URL"]+"/embeddings" || embedding["binding"] != "custom" || embedding["api_key"] != m.env["AURAGO_DT_RELAY_TOKEN"] ||
		embeddingModel["id"] != mergeEmbeddingModel || embeddingModel["model"] != "aurago-embedding" || embeddingModel["dimension"] != "4" {
		t.Fatalf("embedding profile = %#v", embedding)
	}
	if service := catalogService(catalog, "embedding"); service["active_profile_id"] != mergeEmbeddingProfile || service["active_model_id"] != mergeEmbeddingModel {
		t.Fatalf("embedding active = %v / %v", service["active_profile_id"], service["active_model_id"])
	}

	marker := readJSONFile(t, m.markerPath())
	if fp, _ := marker["fingerprint"].(string); len(fp) != 64 {
		t.Fatalf("marker = %#v", marker)
	}
	if runtime.GOOS != "windows" {
		for _, path := range []string{m.catalogPath(), m.markerPath()} {
			if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
				t.Fatalf("%s mode = %v (%v), want 0600", path, info, err)
			}
		}
	}
	leftovers, _ := filepath.Glob(filepath.Join(m.dir, ".aurago-*.tmp"))
	if len(leftovers) != 0 {
		t.Fatalf("temp files left behind: %v", leftovers)
	}
}

func TestDeepTutorMergeScriptRestartIsANoOpAndKeepsUserChoices(t *testing.T) {
	m := newMergeRun(t)
	m.run(t)
	// A relay without embeddings leaves the embedding service alone.
	if embedding := catalogService(m.catalog(t), "embedding"); len(embedding["profiles"].([]any)) != 0 || embedding["active_profile_id"] != nil {
		t.Fatalf("embedding service = %#v", embedding)
	}

	// The user renames the profile, switches to their own and deletes nothing.
	catalog := m.catalog(t)
	catalogProfile(catalog, "llm", mergeLLMProfile)["name"] = "My renamed AuraGo"
	llm := catalogService(catalog, "llm")
	llm["profiles"] = append(llm["profiles"].([]any), map[string]any{
		"id": "llm-profile-user0001", "name": "Own", "binding": "openai", "base_url": "https://api.example.test/v1", "api_key": "sk-own",
		"models": []any{map[string]any{"id": "llm-model-user0001", "name": "own", "model": "gpt-own"}},
	})
	llm["active_profile_id"], llm["active_model_id"] = "llm-profile-user0001", "llm-model-user0001"
	m.writeCatalog(t, catalog)
	before, _ := os.ReadFile(m.catalogPath())
	markerBefore, _ := os.ReadFile(m.markerPath())

	out := m.run(t)
	if !strings.Contains(out, "already applied") {
		t.Fatalf("second run did not report a no-op: %s", out)
	}
	after, _ := os.ReadFile(m.catalogPath())
	markerAfter, _ := os.ReadFile(m.markerPath())
	if string(before) != string(after) || string(markerBefore) != string(markerAfter) {
		t.Fatal("a restart with the same settings rewrote the catalog or the marker")
	}

	// DeepTutor silently resets an unreadable catalog; the marker alone must
	// not be taken as proof that the profile still exists.
	if err := os.WriteFile(m.catalogPath(), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	out = m.run(t)
	if strings.Contains(out, "already applied") || catalogProfile(m.catalog(t), "llm", mergeLLMProfile) == nil {
		t.Fatalf("a damaged catalog was not repaired: %s", out)
	}
}

func TestDeepTutorMergeScriptKeepsAnotherActiveProfileAndUnknownContent(t *testing.T) {
	m := newMergeRun(t)
	m.writeCatalog(t, map[string]any{
		"version":     1,
		"connections": []any{map[string]any{"id": "conn-1", "name": "Shared"}},
		"custom_top":  map[string]any{"keep": "me"},
		"services": map[string]any{
			"llm": map[string]any{
				"active_profile_id": "llm-profile-user0001", "active_model_id": "llm-model-user0001", "future_key": 7,
				"profiles": []any{map[string]any{
					"id": "llm-profile-user0001", "name": "My OpenAI", "binding": "openai", "base_url": "https://api.openai.com/v1", "api_key": "sk-user",
					"models": []any{map[string]any{"id": "llm-model-user0001", "name": "gpt", "model": "gpt-4o"}},
				}},
			},
			"search": map[string]any{"active_profile_id": "search-profile-1", "profiles": []any{map[string]any{"id": "search-profile-1", "provider": "brave"}}},
		},
	})
	m.run(t)

	catalog := m.catalog(t)
	llm := catalogService(catalog, "llm")
	if llm["active_profile_id"] != "llm-profile-user0001" || llm["active_model_id"] != "llm-model-user0001" || llm["future_key"] != float64(7) {
		t.Fatalf("the user's active llm profile was not kept: %#v", llm)
	}
	if user := catalogProfile(catalog, "llm", "llm-profile-user0001"); user == nil || user["api_key"] != "sk-user" || user["base_url"] != "https://api.openai.com/v1" {
		t.Fatalf("the user's profile changed: %#v", user)
	}
	if catalogProfile(catalog, "llm", mergeLLMProfile) == nil {
		t.Fatal("the managed profile was not added next to the user's")
	}
	if top, _ := catalog["custom_top"].(map[string]any); top["keep"] != "me" {
		t.Fatalf("unknown top-level content lost: %#v", catalog["custom_top"])
	}
	if connections, _ := catalog["connections"].([]any); len(connections) != 1 {
		t.Fatalf("connections changed: %#v", catalog["connections"])
	}
	if search := catalogService(catalog, "search"); search["active_profile_id"] != "search-profile-1" {
		t.Fatalf("search service changed: %#v", search)
	}
	if task := catalogService(catalog, "task"); task == nil || task["profiles"] == nil {
		t.Fatalf("missing service shells were not added: %#v", catalog["services"])
	}
}

func TestDeepTutorMergeScriptChangedTokenUpdatesOnlyTheManagedConnection(t *testing.T) {
	m := newMergeRun(t)
	m.env["AURAGO_DT_EMBEDDINGS"] = "1"
	m.env["AURAGO_DT_RELAY_URL"] = "http://127.0.0.1:1/api/store-llm/deeptutor/v1"
	m.run(t)

	catalog := m.catalog(t)
	llm := catalogProfile(catalog, "llm", mergeLLMProfile)
	llm["name"] = "Tutor model"
	profileModel(llm)["capabilities"].(map[string]any)["vision"] = false
	profileModel(llm)["context_window"] = float64(32000)
	llm["extra_headers"] = map[string]any{"X-User": "kept"}
	// The user deleted the managed embedding profile.
	embedding := catalogService(catalog, "embedding")
	embedding["profiles"], embedding["active_profile_id"], embedding["active_model_id"] = []any{}, nil, nil
	m.writeCatalog(t, catalog)

	m.env["AURAGO_DT_RELAY_TOKEN"] = "token-two-" + strings.Repeat("b", 32)
	m.env["AURAGO_DT_RELAY_URL"] = "http://127.0.0.1:1/relay2/api/store-llm/deeptutor/v1"
	out := m.run(t)
	if strings.Contains(out, "already applied") {
		t.Fatalf("a new token must re-apply the connection: %s", out)
	}

	catalog = m.catalog(t)
	llm = catalogProfile(catalog, "llm", mergeLLMProfile)
	capabilities := profileModel(llm)["capabilities"].(map[string]any)
	if llm["api_key"] != m.env["AURAGO_DT_RELAY_TOKEN"] || llm["base_url"] != m.env["AURAGO_DT_RELAY_URL"] {
		t.Fatalf("connection fields not updated: %#v", llm)
	}
	if llm["name"] != "Tutor model" || capabilities["vision"] != false || profileModel(llm)["context_window"] != float64(32000) ||
		llm["extra_headers"].(map[string]any)["X-User"] != "kept" || profileModel(llm)["model"] != "aurago" {
		t.Fatalf("user-edited fields were touched: %#v", llm)
	}
	if catalogProfile(catalog, "embedding", mergeEmbeddingProfile) != nil {
		t.Fatal("a managed profile the user deleted must not come back on a connection update")
	}
	if strings.Contains(string(mustRead(t, m.markerPath())), "token-two") {
		t.Fatal("the marker must hold a fingerprint, not the token")
	}
}

func TestDeepTutorMergeScriptBacksUpACorruptCatalogAndStartsFromDefaults(t *testing.T) {
	m := newMergeRun(t)
	const garbage = `{"services": {"llm": [broken`
	if err := os.WriteFile(m.catalogPath(), []byte(garbage), 0o600); err != nil {
		t.Fatal(err)
	}
	out := m.run(t)
	if !strings.Contains(out, "corrupt") {
		t.Fatalf("the backup was not announced: %s", out)
	}
	backups, _ := filepath.Glob(m.catalogPath() + ".corrupt-*")
	if len(backups) != 1 || string(mustRead(t, backups[0])) != garbage {
		t.Fatalf("corrupt backup = %v", backups)
	}
	catalog := m.catalog(t)
	if catalogProfile(catalog, "llm", mergeLLMProfile) == nil || catalogService(catalog, "llm")["active_profile_id"] != mergeLLMProfile || catalogService(catalog, "task") == nil {
		t.Fatalf("defaults + managed profile expected after a corrupt file: %#v", catalog)
	}
}

func TestDeepTutorMergeScriptWithoutRelaySettingsChangesNothing(t *testing.T) {
	m := newMergeRun(t)
	delete(m.env, "AURAGO_DT_RELAY_TOKEN")
	out := m.run(t)
	if !strings.Contains(out, "missing") {
		t.Fatalf("missing settings were not reported: %s", out)
	}
	if _, err := os.Stat(m.catalogPath()); !os.IsNotExist(err) {
		t.Fatalf("a catalog was written without a token: %v", err)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
