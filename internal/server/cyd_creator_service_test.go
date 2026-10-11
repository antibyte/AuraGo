package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/cydcreator"
	"aurago/internal/security"
)

// cydFakeBuilder succeeds every build with consistent artefacts.
type cydFakeBuilder struct {
	mu     sync.Mutex
	status cydcreator.ToolchainStatus
	pulls  int
}

func (f *cydFakeBuilder) Status(context.Context) cydcreator.ToolchainStatus {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.status
}

func (f *cydFakeBuilder) Pull(context.Context) error {
	f.mu.Lock()
	f.pulls++
	f.mu.Unlock()
	return nil
}

func (f *cydFakeBuilder) Build(_ context.Context, req cydcreator.BuildRequest, onLog func(string)) (cydcreator.BuildOutput, error) {
	onLog("Building " + req.Env)
	files := map[string][]byte{"merged.bin": []byte("merged"), "firmware.bin": []byte("firmware"), "firmware.elf": []byte("elf")}
	result := cydcreator.BuildResultFile{OK: true, Env: req.Env, Files: map[string]cydcreator.BuildResultArtifact{}}
	for name, data := range files {
		result.Files[name] = cydcreator.BuildResultArtifact{Size: int64(len(data)), SHA256: sha256HexForTest(data)}
	}
	return cydcreator.BuildOutput{ExitCode: 0, Result: result, Files: files, Log: "Building " + req.Env + "\n"}, nil
}

func (f *cydFakeBuilder) Addr2Line(_ context.Context, req cydcreator.Addr2LineRequest) ([]cydcreator.Frame, error) {
	frames := make([]cydcreator.Frame, 0, len(req.Addresses))
	for _, a := range req.Addresses {
		frames = append(frames, cydcreator.Frame{Address: a, Function: "app_loop()", File: "src/app/app_main.cpp", Line: 7})
	}
	return frames, nil
}

func cydFakeGenerate(in cydcreator.GenerateInput) (cydcreator.GenerateOutput, error) {
	issues := cydcreator.Validate(in.Project, cydcreator.ValidateOptions{})
	if cydcreator.HasErrors(issues) {
		return cydcreator.GenerateOutput{Issues: issues}, cydcreator.NewError(cydcreator.CodeInvalidLayout, "the layout has errors", nil)
	}
	files := map[string][]byte{"platformio.ini": []byte("[env]\n"), "src/generated/ui.cpp": []byte("// " + in.Project.Name + "\n")}
	for rel, data := range in.AppFiles {
		files[rel] = data
	}
	return cydcreator.GenerateOutput{Files: files, Issues: issues}, nil
}

type cydTestFixture struct {
	s                  *Server
	svc                *cydcreator.Service
	builder            *cydFakeBuilder
	dbPath             string
	read, write, admin string
}

func newCYDTestServer(t *testing.T) *cydTestFixture {
	t.Helper()
	s, read, write := testDesktopPermissionServer(t)
	admin, _, err := s.TokenManager.Create("desktop admin", []string{desktopScopeAdmin}, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	s.Cfg.CYDCreator = config.DefaultCYDCreatorConfig()
	s.Cfg.CYDCreator.Enabled = true
	s.Cfg.Docker.Enabled = true
	dir := t.TempDir()
	builder := &cydFakeBuilder{status: cydcreator.ToolchainStatus{DockerAvailable: true, Image: "test:1", Present: true}}
	dbPath := filepath.Join(dir, "data", "cyd.db")
	svc, err := cydcreator.NewService(cydcreator.ServiceConfig{
		WorkspaceDir: filepath.Join(dir, "desktop"), DataDir: filepath.Join(dir, "data"), DBPath: dbPath,
		Builder: builder, Generate: cydFakeGenerate, IDs: &cydcreator.SeqIDs{}, Logger: s.Logger,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	s.CYDCreator = svc
	return &cydTestFixture{s: s, svc: svc, builder: builder, dbPath: dbPath, read: read, write: write, admin: admin}
}

// cydLockProjectForTest simulates plan 3 taking the project lock: it sets
// cc_projects.locked_by_job through a second connection to the database.
func cydLockProjectForTest(t *testing.T, f *cydTestFixture, projectID, jobID string) {
	t.Helper()
	db, err := sql.Open("sqlite", f.dbPath+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`UPDATE cc_projects SET locked_by_job=? WHERE id=?`, jobID, projectID); err != nil {
		t.Fatal(err)
	}
}

func TestCYDCreatorCapabilityAndConfigReload(t *testing.T) {
	f := newCYDTestServer(t)
	caps := desktopCapabilities{s: f.s}
	if !caps.HasCapability("cyd_creator") {
		t.Fatal("capability off while enabled with Docker")
	}
	f.s.Cfg.Docker.Enabled = false
	if caps.HasCapability("cyd_creator") {
		t.Fatal("capability on without Docker")
	}
	f.s.Cfg.Docker.Enabled = true
	next := *f.s.Cfg
	next.CYDCreator.KeepBuildsPerProject = 9
	next.CYDCreator.BuildTimeoutSeconds = 120
	f.s.replaceConfigSnapshot(&next)
	if got := f.svc.Limits(); got.KeepBuildsPerProject != 9 || got.BuildTimeout != 120*time.Second {
		t.Fatalf("limits after reload = %+v", got)
	}
	none := &Server{Cfg: f.s.Cfg}
	if (desktopCapabilities{s: none}).HasCapability("cyd_creator") {
		t.Fatal("capability on without a service")
	}
	base := config.DefaultCYDCreatorConfig()
	for name, mutate := range map[string]func(*config.CYDCreatorConfig){
		"image": func(c *config.CYDCreatorConfig) { c.BuilderImage = "x:2" }, "memory": func(c *config.CYDCreatorConfig) { c.MemoryMB = 4096 },
		"cpu": func(c *config.CYDCreatorConfig) { c.CPUCores = 4 }, "idle": func(c *config.CYDCreatorConfig) { c.IdleStopMinutes = 5 },
	} {
		changed := base
		mutate(&changed)
		if !cydCreatorRuntimeConfigChanged(base, changed) {
			t.Errorf("%s change must need a restart", name)
		}
	}
	live := base
	live.BuildTimeoutSeconds, live.KeepBuildsPerProject, live.Enabled, live.OTA.PublicURL = 100, 2, true, "https://x"
	if cydCreatorRuntimeConfigChanged(base, live) {
		t.Error("live settings must not need a restart")
	}
}

func TestCYDCreatorConfigDefaultsAndProviderReference(t *testing.T) {
	cfg := &config.Config{CYDCreator: config.DefaultCYDCreatorConfig()}
	raw := map[string]interface{}{"cyd_creator": map[string]interface{}{"memory_mb": 4096, "ota": map[string]interface{}{"public_url": "https://x"}}}
	injectCYDCreatorDefaults(raw, cfg)
	section := raw["cyd_creator"].(map[string]interface{})
	ota := section["ota"].(map[string]interface{})
	if section["memory_mb"] != 4096 || section["build_timeout_seconds"] != 300 || section["cpu_cores"] != float64(2) ||
		section["builder_image"] != config.DefaultCYDCreatorBuilderImage || section["enabled"] != false || section["model"] != "" ||
		ota["public_url"] != "https://x" || ota["heartbeat_seconds"] != 60 || ota["enabled"] != true {
		t.Fatalf("injected section = %v", section)
	}
	empty := map[string]interface{}{}
	injectCYDCreatorDefaults(empty, cfg)
	if got := len(empty["cyd_creator"].(map[string]interface{})); got != 12 {
		t.Fatalf("empty config got %d cyd_creator keys, want 12", got)
	}
	cfg.CYDCreator.Model = "fast"
	refs := providerReferences(cfg, "fast")
	if len(refs) != 1 || refs[0].Path != "cyd_creator.model" || refs[0].Role != "cyd_creator" {
		t.Fatalf("provider references = %+v", refs)
	}
}

// An unnormalized section (a snapshot that did not pass config.Load) must
// still reach the service with non-zero timeouts: an all-zero ServiceLimits
// would become the defaults and switch OTA back on.
func TestCYDCreatorLimitsAreNormalized(t *testing.T) {
	got := cydCreatorLimits(config.CYDCreatorConfig{OTA: config.CYDCreatorOTAConfig{Enabled: false}})
	if got.BuildTimeout != 300*time.Second || got.ColdBuildTimeout != 600*time.Second || got.JobTimeout != 20*time.Minute ||
		got.KeepBuildsPerProject != 5 || got.OTA.HeartbeatSeconds != 60 || got.OTA.Enabled {
		t.Fatalf("limits of an empty section = %+v", got)
	}
	got = cydCreatorLimits(config.CYDCreatorConfig{BuildTimeoutSeconds: 5, ColdBuildTimeoutSeconds: 1, MaxRepairAttempts: 2,
		Model: " fast ", OTA: config.CYDCreatorOTAConfig{Enabled: true, PublicURL: "https://user:pw@cyd.example:8443/"}})
	if got.BuildTimeout != 60*time.Second || got.ColdBuildTimeout != 60*time.Second || got.MaxRepairAttempts != 2 ||
		got.Model != "fast" || !got.OTA.Enabled || got.OTA.PublicURL != "" {
		t.Fatalf("limits of an out-of-range section = %+v", got)
	}
}

// cydInitTestConfig is a loaded-like config with an agent workspace, a desktop
// workspace inside it and a data directory next to it.
func cydInitTestConfig(t *testing.T) *config.Config {
	t.Helper()
	root := t.TempDir()
	cfg := &config.Config{CYDCreator: config.DefaultCYDCreatorConfig()}
	cfg.CYDCreator.Enabled = true
	cfg.Docker.Enabled = true
	cfg.Docker.Host = "tcp://127.0.0.1:1" // never reached: no runtime Docker grant in this test
	cfg.Directories.WorkspaceDir = filepath.Join(root, "agent_workspace", "workdir")
	cfg.Directories.DataDir = filepath.Join(root, "data")
	cfg.VirtualDesktop.WorkspaceDir = filepath.Join(root, "agent_workspace", "virtual_desktop")
	cfg.SQLite.CYDCreatorPath = filepath.Join(root, "data", "cyd_creator.db")
	return cfg
}

func TestInitCYDCreatorWiresTheTrustRecordAndShutsDown(t *testing.T) {
	cfg := cydInitTestConfig(t)
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	t.Cleanup(s.shutdownCYDCreator)
	s.initCYDCreator()
	if s.CYDCreator == nil || s.cydBuilder == nil {
		t.Fatal("CYD Creator not initialized")
	}
	if got, want := s.cydBuilder.settings.TrustFile, filepath.Join(cfg.Directories.DataDir, "cyd_builder_image.json"); got != want {
		t.Fatalf("builder trust file = %q, want %q", got, want)
	}
	if cydcreator.DefaultService() != s.CYDCreator {
		t.Fatal("service not published")
	}
	if s.cydCreatorProblem != "" {
		t.Fatalf("problem recorded for a running service: %q", s.cydCreatorProblem)
	}
	if !s.cydCreatorCapability() {
		t.Fatal("capability off after init")
	}
	s.shutdownCYDCreator()
	if cydcreator.DefaultService() != nil {
		t.Fatal("default service survives the shutdown")
	}
	if _, err := s.CYDCreator.ListProjects(context.Background()); err == nil {
		t.Fatal("service still serves after the shutdown")
	}
	if !s.cydBuilder.closed {
		t.Fatal("builder not closed")
	}
}

func TestInitCYDCreatorRefusesADatabaseTheAgentCanWrite(t *testing.T) {
	for name, place := range map[string]func(*config.Config) string{
		"agent workspace": func(c *config.Config) string {
			return filepath.Join(filepath.Dir(c.Directories.WorkspaceDir), "state", "cyd.db")
		},
		"agent workdir":     func(c *config.Config) string { return filepath.Join(c.Directories.WorkspaceDir, "cyd.db") },
		"desktop workspace": func(c *config.Config) string { return filepath.Join(c.VirtualDesktop.WorkspaceDir, "cyd.db") },
	} {
		t.Run(name, func(t *testing.T) {
			cfg := cydInitTestConfig(t)
			cfg.SQLite.CYDCreatorPath = place(cfg)
			if problem := cydCreatorStorageProblem(cfg, cfg.VirtualDesktop.WorkspaceDir); problem == "" {
				t.Fatal("no storage problem reported")
			}
			s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
			t.Cleanup(s.shutdownCYDCreator)
			s.initCYDCreator()
			if s.CYDCreator != nil || s.cydBuilder != nil || cydcreator.DefaultService() != nil {
				t.Fatal("CYD Creator started with an agent-writable database directory")
			}
			if _, err := os.Stat(cfg.SQLite.CYDCreatorPath); !os.IsNotExist(err) {
				t.Fatalf("database created anyway: %v", err)
			}
			if s.cydCreatorProblem == "" || strings.Contains(s.cydCreatorProblem, filepath.Dir(cfg.SQLite.CYDCreatorPath)) {
				t.Fatalf("recorded problem = %q, want a generic reason without paths", s.cydCreatorProblem)
			}
		})
	}
	// The data directory is protected from the agent's file tools even when
	// it lies inside agent_workspace, so a database there is accepted.
	cfg := cydInitTestConfig(t)
	cfg.Directories.DataDir = filepath.Join(filepath.Dir(cfg.Directories.WorkspaceDir), "data")
	cfg.SQLite.CYDCreatorPath = filepath.Join(cfg.Directories.DataDir, "cyd_creator.db")
	if problem := cydCreatorStorageProblem(cfg, cfg.VirtualDesktop.WorkspaceDir); problem != "" {
		t.Fatalf("protected data directory refused: %s", problem)
	}
	if problem := cydCreatorStorageProblem(cydInitTestConfig(t), ""); problem != "" {
		t.Fatalf("default layout refused: %s", problem)
	}
}

func TestCYDCreatorConfigSaveRejectsInvalidSection(t *testing.T) {
	for name, section := range map[string]string{
		"credentials": `{"ota":{"public_url":"https://user:pw@cyd.example"}}`,
		"query":       `{"ota":{"public_url":"https://cyd.example/?x=1"}}`,
		"ipv6":        `{"ota":{"public_url":"http://[fd00::1]:8088"}}`,
		"port":        `{"ota":{"public_url":"https://cyd.example:70000"}}`,
		"image":       `{"builder_image":"Bad Image"}`,
		"timeout":     `{"build_timeout_seconds":5}`,
	} {
		for _, setup := range []bool{false, true} {
			t.Run(name+map[bool]string{false: "/config", true: "/setup"}[setup], func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "config.yaml")
				original := []byte("server:\n  host: 127.0.0.1\ncyd_creator:\n  enabled: false\n")
				if err := os.WriteFile(path, original, 0o600); err != nil {
					t.Fatal(err)
				}
				cfg, err := config.Load(path)
				if err != nil {
					t.Fatal(err)
				}
				cfg.ConfigPath = path
				s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
				if setup {
					var patch map[string]interface{}
					if err := json.Unmarshal([]byte(`{"cyd_creator":`+section+`}`), &patch); err != nil {
						t.Fatal(err)
					}
					if _, err := applyConfigPatch(s, patch); err == nil || !strings.Contains(err.Error(), "cyd_creator.") {
						t.Fatalf("invalid section accepted: %v", err)
					}
				} else {
					rec := httptest.NewRecorder()
					handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{"cyd_creator":`+section+`}`)))
					if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "cyd_creator.") {
						t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
					}
				}
				stored, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if string(stored) != string(original) {
					t.Fatal("invalid cyd_creator section changed the persisted file")
				}
			})
		}
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  host: 127.0.0.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConfigPath = path
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	reloaded, err := applyConfigPatch(s, map[string]interface{}{"cyd_creator": map[string]interface{}{
		"ota": map[string]interface{}{"public_url": "https://cyd.example:8443/aura"}}})
	if err != nil {
		t.Fatalf("valid section refused: %v", err)
	}
	if got := reloaded.CYDCreator.OTA.PublicURL; got != "https://cyd.example:8443/aura" {
		t.Fatalf("saved public_url = %q", got)
	}
}

func TestValidateCYDCreatorSaveChecksOnlyChangedValues(t *testing.T) {
	invalid := config.CYDCreatorConfig{BuilderImage: "Bad Image", BuildTimeoutSeconds: 5, ColdBuildTimeoutSeconds: 4000,
		MemoryMB: 1, CPUCores: 99, IdleStopMinutes: 9999, MaxRepairAttempts: 11, JobTimeoutMinutes: 1, KeepBuildsPerProject: 99,
		OTA: config.CYDCreatorOTAConfig{HeartbeatSeconds: 1, PublicURL: "https://user:pw@cyd.example"}}
	if err := validateCYDCreatorSave(invalid, invalid); err != nil {
		t.Fatalf("unchanged hand-edited values blocked the save: %v", err)
	}
	spaced := invalid
	spaced.BuilderImage, spaced.OTA.PublicURL = " Bad Image ", "https://user:pw@cyd.example "
	if err := validateCYDCreatorSave(invalid, spaced); err != nil {
		t.Fatalf("an unchanged value with different spacing blocked the save: %v", err)
	}
	for name, mutate := range map[string]func(*config.CYDCreatorConfig){
		"image":      func(c *config.CYDCreatorConfig) { c.BuilderImage = "Other Bad" },
		"build":      func(c *config.CYDCreatorConfig) { c.BuildTimeoutSeconds = 6 },
		"memory":     func(c *config.CYDCreatorConfig) { c.MemoryMB = 2 },
		"cpu":        func(c *config.CYDCreatorConfig) { c.CPUCores = 0.1 },
		"repairs":    func(c *config.CYDCreatorConfig) { c.MaxRepairAttempts = -1 },
		"public url": func(c *config.CYDCreatorConfig) { c.OTA.PublicURL = "http://[fd00::1]:8088" },
	} {
		candidate := invalid
		mutate(&candidate)
		if err := validateCYDCreatorSave(invalid, candidate); err == nil {
			t.Errorf("%s: a changed invalid value was accepted", name)
		}
	}
	valid := invalid
	valid.KeepBuildsPerProject = 9
	if err := validateCYDCreatorSave(invalid, valid); err != nil {
		t.Fatalf("a valid change next to unchanged invalid values was refused: %v", err)
	}
	// The cold-versus-warm rule runs when either timeout changed, also against
	// the unchanged one.
	base := config.DefaultCYDCreatorConfig()
	warmer := base
	warmer.BuildTimeoutSeconds = 700
	if err := validateCYDCreatorSave(base, warmer); err == nil || !strings.Contains(err.Error(), "cold_build_timeout_seconds") {
		t.Fatalf("build timeout above the unchanged cold timeout = %v", err)
	}
	colder := base
	colder.ColdBuildTimeoutSeconds = 200
	if err := validateCYDCreatorSave(base, colder); err == nil || !strings.Contains(err.Error(), "cold_build_timeout_seconds") {
		t.Fatalf("cold timeout below the unchanged build timeout = %v", err)
	}
	if got := cydCreatorSectionFromRaw(map[string]interface{}{"cyd_creator": map[string]interface{}{
		"build_timeout_seconds": 5, "ota": map[string]interface{}{"public_url": "https://x"}}}); got.BuildTimeoutSeconds != 5 || got.OTA.PublicURL != "https://x" {
		t.Fatalf("decoded raw section = %+v", got)
	}
	if got := cydCreatorSectionFromRaw(map[string]interface{}{}); got != (config.CYDCreatorConfig{}) {
		t.Fatalf("missing raw section = %+v, want the zero value", got)
	}
}

// cydConfigServer loads configYAML from a temporary config.yaml into a server
// with a vault, as the config save paths need.
func cydConfigServer(t *testing.T, configYAML string) *Server {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(configYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	vault, err := security.NewVault(strings.Repeat("57", 32), filepath.Join(dir, "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConfigPath = path
	return &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Vault: vault}
}

func cydSaveConfig(t *testing.T, s *Server, payload string) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(payload)))
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	return rec.Code, resp
}

func cydRestartReasons(resp map[string]any) []string {
	var out []string
	reasons, _ := resp["restart_reason"].([]any)
	for _, r := range reasons {
		if text, ok := r.(string); ok {
			out = append(out, text)
		}
	}
	return out
}

// A config.yaml with hand-edited invalid cyd_creator values (config.Load
// normalizes them) must not make every other save fail; a save that changes
// them to invalid values still does.
func TestCYDCreatorConfigSaveKeepsWorkingWithAnInvalidUnchangedSection(t *testing.T) {
	const invalidYAML = "server:\n  host: 127.0.0.1\nagent:\n  system_language: Deutsch\ncyd_creator:\n  enabled: false\n" +
		"  build_timeout_seconds: 5\n  builder_image: 'Bad Image'\n  ota:\n    public_url: 'https://user:pw@cyd.example'\n"
	s := cydConfigServer(t, invalidYAML)
	for _, payload := range []string{
		`{"agent":{"system_language":"English"}}`,
		`{"cyd_creator":{"keep_builds_per_project":9}}`,
		`{"cyd_creator":{"build_timeout_seconds":5,"builder_image":"Bad Image","ota":{"public_url":"https://user:pw@cyd.example"}}}`,
	} {
		if code, resp := cydSaveConfig(t, s, payload); code != http.StatusOK {
			t.Fatalf("save %s = %d %v, want 200", payload, code, resp)
		}
	}
	for _, payload := range []string{
		`{"cyd_creator":{"build_timeout_seconds":7}}`,
		`{"cyd_creator":{"builder_image":"Other Bad"}}`,
		`{"cyd_creator":{"ota":{"public_url":"https://a:b@other.example"}}}`,
		`{"cyd_creator":{"build_timeout_seconds":700}}`, // above the cold timeout (600)
	} {
		if code, resp := cydSaveConfig(t, s, payload); code != http.StatusBadRequest {
			t.Fatalf("save %s = %d %v, want 400", payload, code, resp)
		}
	}

	setup := cydConfigServer(t, invalidYAML)
	if _, err := applyConfigPatch(setup, map[string]interface{}{"agent": map[string]interface{}{"system_language": "English"}}); err != nil {
		t.Fatalf("setup save next to an unchanged invalid cyd_creator section: %v", err)
	}
	if _, err := applyConfigPatch(setup, map[string]interface{}{"cyd_creator": map[string]interface{}{"build_timeout_seconds": 7}}); err == nil ||
		!strings.Contains(err.Error(), "cyd_creator.build_timeout_seconds") {
		t.Fatalf("setup save of an invalid cyd_creator value = %v", err)
	}
}

func TestCYDCreatorRestartReasons(t *testing.T) {
	f := newCYDTestServer(t)
	oldCfg := *f.s.Cfg
	for name, mutate := range map[string]func(*config.Config){
		"memory":      func(c *config.Config) { c.CYDCreator.MemoryMB = 4096 },
		"docker host": func(c *config.Config) { c.Docker.Host = "tcp://127.0.0.1:2375" },
	} {
		newCfg := oldCfg
		mutate(&newCfg)
		if got := f.s.cydCreatorRestartReason(&oldCfg, &newCfg); got != "CYD Creator builder" {
			t.Errorf("%s change with a running service: reason %q", name, got)
		}
		if got := (&Server{}).cydCreatorRestartReason(&oldCfg, &newCfg); got == "CYD Creator builder" {
			t.Errorf("%s change without a service asks to restart the builder", name)
		}
	}
	live := oldCfg
	live.CYDCreator.KeepBuildsPerProject = 7
	if got := f.s.cydCreatorRestartReason(&oldCfg, &live); got != "" {
		t.Errorf("live change: reason %q", got)
	}

	// Without a service: switching on asks for a restart only when the new
	// configuration would not be refused again.
	cfg := cydInitTestConfig(t)
	off := *cfg
	off.CYDCreator.Enabled = false
	none := &Server{Cfg: cfg}
	if got := none.cydCreatorRestartReason(&off, cfg); got != "CYD Creator service" {
		t.Errorf("switching on with a usable database path: reason %q", got)
	}
	refused := *cfg
	refused.SQLite.CYDCreatorPath = filepath.Join(cfg.Directories.WorkspaceDir, "cyd.db")
	if got := none.cydCreatorRestartReason(&off, &refused); got != "" {
		t.Errorf("switching on with an agent-writable database path: reason %q", got)
	}

	// Through the config handler.
	s := cydConfigServer(t, "server:\n  host: 127.0.0.1\ndocker:\n  enabled: true\ncyd_creator:\n  enabled: true\n")
	s.CYDCreator = f.svc
	code, resp := cydSaveConfig(t, s, `{"cyd_creator":{"memory_mb":4096}}`)
	if code != http.StatusOK || !slices.Contains(cydRestartReasons(resp), "CYD Creator builder") {
		t.Fatalf("memory change = %d %v, want a builder restart", code, resp)
	}
	code, resp = cydSaveConfig(t, s, `{"cyd_creator":{"keep_builds_per_project":7}}`)
	if code != http.StatusOK || slices.ContainsFunc(cydRestartReasons(resp), func(r string) bool { return strings.HasPrefix(r, "CYD Creator") }) {
		t.Fatalf("live change = %d %v, want no CYD Creator restart", code, resp)
	}
	if got := f.svc.Limits().KeepBuildsPerProject; got != 7 {
		t.Fatalf("live limit after the save = %d, want 7", got)
	}
	agentDB := filepath.Join(t.TempDir(), "agent_workspace", "state", "cyd.db")
	workdir := filepath.Join(filepath.Dir(filepath.Dir(agentDB)), "workdir")
	off2 := cydConfigServer(t, "server:\n  host: 127.0.0.1\ndirectories:\n  workspace_dir: '"+workdir+"'\nsqlite:\n  cyd_creator_path: '"+agentDB+"'\n")
	code, resp = cydSaveConfig(t, off2, `{"cyd_creator":{"enabled":true}}`)
	if code != http.StatusOK || slices.Contains(cydRestartReasons(resp), "CYD Creator service") {
		t.Fatalf("switching on with an agent-writable database = %d %v, want no restart promise", code, resp)
	}
}

func TestCYDCreatorCapabilityNeedsWritableDocker(t *testing.T) {
	f := newCYDTestServer(t)
	f.s.Cfg.Docker.ReadOnly = true
	if (desktopCapabilities{s: f.s}).HasCapability("cyd_creator") {
		t.Fatal("capability on while Docker is read-only")
	}
}

func TestInitCYDCreatorDisabledCreatesNothing(t *testing.T) {
	cfg := cydInitTestConfig(t)
	cfg.CYDCreator.Enabled = false
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	t.Cleanup(s.shutdownCYDCreator)
	s.initCYDCreator()
	if s.CYDCreator != nil || s.cydBuilder != nil || s.cydCreatorProblem != "" {
		t.Fatal("a disabled CYD Creator was initialized or reported a problem")
	}
	if _, err := os.Stat(cfg.SQLite.CYDCreatorPath); !os.IsNotExist(err) {
		t.Fatalf("database created while disabled: %v", err)
	}
}
