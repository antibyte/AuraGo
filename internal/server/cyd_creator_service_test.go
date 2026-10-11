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
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/cydcreator"
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
