package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/flows"
	"aurago/internal/security"
	"aurago/internal/tools"
)

// c16Server builds a server like newFlowsTestServer with the given flows switch and
// logger (nil discards); it initialises and starts nothing.
func c16Server(t *testing.T, enabled bool, logger *slog.Logger) *Server {
	t.Helper()
	s, _, _ := testDesktopPermissionServer(t)
	dir := t.TempDir()
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	s.Logger = logger
	s.Cfg.Flows.Enabled = enabled
	s.Cfg.Tools.Missions.Enabled = true
	s.Cfg.Directories.DataDir = dir
	s.Cfg.Directories.WorkspaceDir = filepath.Join(dir, "workspace")
	s.Cfg.Directories.ToolsDir = filepath.Join(dir, "tools")
	s.Cfg.Directories.SkillsDir = filepath.Join(dir, "skills")
	s.Cfg.SQLite.GameMakerPath = filepath.Join(dir, "game_maker.db")
	tools.ConfigureRuntimePermissions(tools.RuntimePermissionsFromConfig(s.Cfg))
	s.MissionManagerV2 = tools.NewMissionManagerV2(dir, nil)
	t.Cleanup(s.MissionManagerV2.Stop)
	return s
}

// c16StartedServer is c16Server with flows on, initialised and started like the server
// does; the cleanup shuts the flows down.
func c16StartedServer(t *testing.T, logger *slog.Logger) *Server {
	t.Helper()
	s := c16Server(t, true, logger)
	s.initFlows()
	if s.Flows == nil {
		t.Fatal("flows were not initialised")
	}
	if err := s.MissionManagerV2.Start(); err != nil {
		t.Fatalf("missions: %v", err)
	}
	s.startFlows(context.Background())
	t.Cleanup(func() { s.shutdownFlows(context.Background()) })
	return s
}

// c16PublishedFlow creates and publishes a flow.
func c16PublishedFlow(t *testing.T, s *Server, docJSON string) *flows.FlowRecord {
	t.Helper()
	rec := createTestFlow(t, s, docJSON)
	pub, _, err := s.Flows.Publish(context.Background(), rec.ID, rec.DraftRevision)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	return pub
}

// c16WaitStatus waits until the flow mission has the status.
func c16WaitStatus(t *testing.T, s *Server, missionID, status string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if m, ok := s.MissionManagerV2.Get(missionID); ok && m.Status == status {
			return
		}
		if time.Now().After(deadline) {
			m, _ := s.MissionManagerV2.Get(missionID)
			t.Fatalf("mission %s status = %+v, want %q", missionID, m, status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestC16FlowsRunInTheLocalZone(t *testing.T) {
	s := c16StartedServer(t, nil)
	if got := s.Flows.Location(); got != time.Local {
		t.Fatalf("flow location = %v, want time.Local (the cron manager's zone)", got)
	}
}

func TestC16FlowsRuntimeConfigChanged(t *testing.T) {
	base := config.FlowsConfig{Enabled: true}
	for _, tc := range []struct {
		name   string
		mutate func(*config.FlowsConfig)
		want   bool
	}{
		{"nothing", func(*config.FlowsConfig) {}, false},
		{"switched off", func(c *config.FlowsConfig) { c.Enabled = false }, true},
		{"parallel runs", func(c *config.FlowsConfig) { c.MaxParallelRuns = 4 }, true},
		{"parallel nodes", func(c *config.FlowsConfig) { c.MaxParallelNodesPerRun = 2 }, true},
		{"retention days", func(c *config.FlowsConfig) { c.RunRetentionDays = 7 }, true},
		{"runs per flow", func(c *config.FlowsConfig) { c.MaxRunsPerFlow = 50 }, true},
		{"default written out", func(c *config.FlowsConfig) { c.MaxParallelRuns = 8; c.MaxRunsPerFlow = 200 }, false},
		{"beyond the clamp", func(c *config.FlowsConfig) { c.MaxParallelRuns = 999 }, true},
		{"ai provider (live)", func(c *config.FlowsConfig) { c.AIProvider = "other" }, false},
		{"agent options (live)", func(c *config.FlowsConfig) { c.Agent.ReadOnly = true; c.Agent.AllowPublish = true }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := base
			tc.mutate(&changed)
			if got := flowsRuntimeConfigChanged(base, changed); got != tc.want {
				t.Fatalf("flowsRuntimeConfigChanged = %v, want %v", got, tc.want)
			}
		})
	}
	beyond := base
	beyond.MaxParallelRuns = 999
	clamped := base
	clamped.MaxParallelRuns = 32
	if flowsRuntimeConfigChanged(beyond, clamped) {
		t.Fatal("two values with the same effective limit must not ask for a restart")
	}
}

// c16SaveConfig sends a config update to a server loaded from configYAML and returns the
// decoded answer.
func c16SaveConfig(t *testing.T, configYAML, update string) map[string]any {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(tmpDir, "vault.bin"))
	if err != nil {
		t.Fatalf("init vault: %v", err)
	}
	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	loaded.ConfigPath = configPath
	s := &Server{Cfg: loaded, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Vault: vault}
	rec := httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(update)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp
}

func c16RestartReasons(resp map[string]any) []string {
	var out []string
	reasons, _ := resp["restart_reason"].([]any)
	for _, r := range reasons {
		if s, ok := r.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func TestC16ConfigSaveAsksForARestartWhenFlowsChange(t *testing.T) {
	const base = "flows:\n  enabled: true\n"
	for _, update := range []string{
		`{"flows":{"max_parallel_runs":4}}`,
		`{"flows":{"enabled":false}}`,
		`{"flows":{"run_retention_days":7}}`,
	} {
		resp := c16SaveConfig(t, base, update)
		if resp["needs_restart"] != true || !containsString(c16RestartReasons(resp), "EasyDrag flows") {
			t.Fatalf("update %s: answer %v, want a restart for EasyDrag flows", update, resp)
		}
	}
	resp := c16SaveConfig(t, base, `{"flows":{"agent":{"read_only":true}}}`)
	if containsString(c16RestartReasons(resp), "EasyDrag flows") {
		t.Fatalf("agent options are read live; answer %v", resp)
	}
}

func TestC16DisabledFlowsLeaveFlowMissionsInert(t *testing.T) {
	for _, tc := range []struct {
		name    string
		enabled bool
		prepare func(t *testing.T, s *Server)
	}{
		{name: "flows switched off", enabled: false, prepare: func(*testing.T, *Server) {}},
		{name: "store cannot be opened", enabled: true, prepare: func(t *testing.T, s *Server) {
			// A directory where flows.db belongs makes OpenStore fail.
			if err := os.MkdirAll(s.flowsDBPath(), 0o700); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := c16Server(t, tc.enabled, nil)
			tc.prepare(t, s)
			mm := s.MissionManagerV2
			// missions.json still holds a published, enabled flow mission.
			missionID, err := mm.CreateFlowMission("flow_c16aaaaaa", "Alter Flow")
			if err != nil {
				t.Fatal(err)
			}
			if err := mm.SyncFlowMission(missionID, "Alter Flow", []tools.FlowTriggerSpec{{NodeID: "n_aaaaaaaa", TriggerType: tools.FlowTriggerManual}}); err != nil {
				t.Fatal(err)
			}
			if err := mm.SetFlowMissionEnabled(missionID, true); err != nil {
				t.Fatal(err)
			}
			agent := make(chan string, 4)
			mm.SetCallback(func(_ string, id string) { agent <- id })

			s.initFlows()
			if s.Flows != nil {
				t.Fatal("initFlows created a flow service")
			}
			if err := mm.Start(); err != nil {
				t.Fatalf("missions: %v", err)
			}
			s.startFlows(context.Background())
			defer s.shutdownFlows(context.Background())
			if (desktopCapabilities{s: s}).HasCapability("flows") {
				t.Fatal("capability flows without a flow service")
			}
			w := httptest.NewRecorder()
			handleMissionRunV2(s, w, httptest.NewRequest(http.MethodPost, "/api/missions/v2/"+missionID+"/run", nil), missionID)
			if w.Code < 400 || !strings.Contains(w.Body.String(), "flows are not available") {
				t.Fatalf("run now = %d %s, want a clear refusal", w.Code, w.Body.String())
			}
			select {
			case id := <-agent:
				t.Fatalf("the agent ran mission %s", id)
			case <-time.After(150 * time.Millisecond):
			}
			if m, _ := mm.Get(missionID); m.Status != tools.MissionStatusIdle || m.RunCount != 0 {
				t.Fatalf("flow mission after the refused run: %+v", m)
			}
		})
	}
}

func TestC16ShutdownReportsActiveRunsToMissionControl(t *testing.T) {
	s := c16StartedServer(t, nil)
	rec := c16PublishedFlow(t, s, waitFlowJSON)
	started, err := s.Flows.RunNow(context.Background(), rec.ID)
	if err != nil {
		t.Fatalf("RunNow: %v", err)
	}
	c16WaitStatus(t, s, rec.MissionID, tools.MissionStatusRunning)
	begin := time.Now()
	s.shutdownFlows(context.Background())
	if waited := time.Since(begin); waited > flowsShutdownTimeout {
		t.Fatalf("shutdown took %s", waited)
	}
	m, _ := s.MissionManagerV2.Get(rec.MissionID)
	if m.Status != tools.MissionStatusIdle || m.RunCount != 1 || m.LastResult != tools.MissionResultError {
		t.Fatalf("flow mission after shutdown: status %q runs %d result %q", m.Status, m.RunCount, m.LastResult)
	}
	if _, err := s.Flows.RunNow(context.Background(), rec.ID); err == nil {
		t.Fatalf("a run started after shutdown (first run %s)", started.RunID)
	}
}

func TestC16StartupReconcilesMissionControl(t *testing.T) {
	logs := &c15LogBuffer{}
	s := c16Server(t, true, c15Logger(logs))
	s.initFlows()
	if s.Flows == nil {
		t.Fatal("flows were not initialised")
	}
	ctx := context.Background()
	t.Cleanup(func() { s.shutdownFlows(ctx) })
	mm := s.MissionManagerV2
	if err := mm.Start(); err != nil {
		t.Fatalf("missions: %v", err)
	}
	pub := c16PublishedFlow(t, s, greetFlowJSON)
	bridge := flowMissionBridge{s: s}
	bound, err := flows.BindTriggers(pub.Live, s.Flows.Registry(), s.Flows.Location(), pub.PublishedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !bridge.FlowMissionInSync(pub.MissionID, pub.Live.Name, bound) {
		t.Fatal("a freshly published flow mission is not in sync")
	}
	// A crash after the store publish left the mission behind; another flow is gone.
	if err := mm.SyncFlowMission(pub.MissionID, "Alt", nil); err != nil {
		t.Fatal(err)
	}
	if bridge.FlowMissionInSync(pub.MissionID, pub.Live.Name, bound) {
		t.Fatal("a diverged mission counts as in sync")
	}
	orphan, err := mm.CreateFlowMission("flow_c16gone00", "Waise")
	if err != nil {
		t.Fatal(err)
	}
	if got := bridge.FlowMissions(); got[pub.MissionID] != pub.ID || got[orphan] != "flow_c16gone00" {
		t.Fatalf("FlowMissions = %v", got)
	}

	s.startFlows(ctx)
	deadline := time.Now().Add(3 * time.Second)
	for !strings.Contains(logs.String(), "flow missions reconciled") {
		if time.Now().After(deadline) {
			t.Fatalf("no reconciliation after start:\n%s", logs.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	m, _ := mm.Get(pub.MissionID)
	if m.Name != "Gruss" || len(m.FlowTriggers) != 1 || m.FlowTriggers[0].TriggerType != tools.FlowTriggerManual {
		t.Fatalf("mission after reconcile: %+v", m)
	}
	if out := logs.String(); !strings.Contains(out, "mission_id="+orphan) || !strings.Contains(out, "whose flow is gone") {
		t.Fatalf("the orphan mission was not reported:\n%s", out)
	}
	if _, ok := mm.Get(orphan); !ok {
		t.Fatal("the orphan mission was deleted")
	}
}

func TestC16HooksAfterShutdownOnlyLog(t *testing.T) {
	logs := &c15LogBuffer{}
	s := c16StartedServer(t, c15Logger(logs))
	rec := c16PublishedFlow(t, s, greetFlowJSON)
	if err := s.Flows.SetEnabled(context.Background(), rec.ID, true); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	s.shutdownFlows(context.Background())

	hooks := flowMissionHooks{s: s}
	if err := hooks.StartFlowRun(rec.MissionID, "", "manual", ""); err == nil {
		t.Fatal("StartFlowRun after shutdown succeeded")
	}
	if err := s.MissionManagerV2.RunNow(rec.MissionID); err == nil {
		t.Fatal("Mission Control started a flow run after shutdown")
	}
	if _, ok := hooks.NextFlowRun(rec.MissionID); ok {
		t.Fatal("NextFlowRun answered from a closed store")
	}
	hooks.FlowEnabledChanged(rec.MissionID, false)
	hooks.FlowMissionDeleted(rec.MissionID)
	out := logs.String()
	for _, want := range []string{"Flow timers could not follow Mission Control", "The flow of a deleted mission could not be removed"} {
		if !strings.Contains(out, want) {
			t.Fatalf("log lacks %q:\n%s", want, out)
		}
	}
	// The mission list still renders: broadcastMissionState asks every flow mission's next run.
	if payloads := missionPayloads(s.MissionManagerV2, s.MissionManagerV2.List()); len(payloads) == 0 {
		t.Fatal("no mission payloads after shutdown")
	}
}

// c16Cancel sends Mission Control's cancel for missionID with ctx.
func c16Cancel(s *Server, ctx context.Context, missionID string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/missions/v2/"+missionID+"/cancel", nil).WithContext(ctx)
	handleMissionCancelV2(s, w, req, missionID)
	return w
}

func TestC16MissionControlCancelMapsFlowErrors(t *testing.T) {
	s := c16StartedServer(t, nil)
	ctx := context.Background()
	mm := s.MissionManagerV2

	// The mission names a flow the store does not hold.
	orphan, err := mm.CreateFlowMission("flow_c16gone00", "Ohne Flow")
	if err != nil {
		t.Fatal(err)
	}
	mm.FlowRunStarted(orphan, "manual", "")
	if w := c16Cancel(s, ctx, orphan); w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), "does not exist") {
		t.Fatalf("cancel without a flow = %d %s", w.Code, w.Body.String())
	}

	// The flow exists but has no live run.
	idle := createTestFlow(t, s, greetFlowJSON)
	mm.FlowRunStarted(idle.MissionID, "manual", "")
	if w := c16Cancel(s, ctx, idle.MissionID); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "cannot be cancelled yet") {
		t.Fatalf("cancel without runs = %d %s", w.Code, w.Body.String())
	}

	// Two flows hold the same mission.
	twin := createTestFlow(t, s, greetFlowJSON)
	if err := s.Flows.Store().SetMissionID(ctx, twin.ID, idle.MissionID); err != nil {
		t.Fatal(err)
	}
	if w := c16Cancel(s, ctx, idle.MissionID); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "several flows") {
		t.Fatalf("cancel of an ambiguous mission = %d %s", w.Code, w.Body.String())
	}
}

func TestC16MissionControlCancelSurvivesAClientDisconnect(t *testing.T) {
	s := c16StartedServer(t, nil)
	ctx := context.Background()
	rec := c16PublishedFlow(t, s, waitFlowJSON)
	started, err := s.Flows.RunNow(ctx, rec.ID)
	if err != nil {
		t.Fatalf("RunNow: %v", err)
	}
	c16WaitStatus(t, s, rec.MissionID, tools.MissionStatusRunning)
	gone, cancel := context.WithCancel(ctx)
	cancel() // the client went away before the handler ran
	if w := c16Cancel(s, gone, rec.MissionID); w.Code != http.StatusAccepted {
		t.Fatalf("cancel = %d %s", w.Code, w.Body.String())
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		detail, _ := s.Flows.Run(ctx, started.RunID, false)
		if detail != nil && detail.Run.Status == flows.RunCancelled {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("run not cancelled: %+v", detail)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// c16CronRequest sends one request to a dashboard cron handler.
func c16CronRequest(h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(w, req)
	return w
}

func TestC16DashboardRefusesFlowCronJobs(t *testing.T) {
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{SchedulerEnabled: true, MissionsEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)
	cronMgr := tools.NewCronManager(t.TempDir())
	t.Cleanup(func() { _ = cronMgr.Close() })
	mm := tools.NewMissionManagerV2(t.TempDir(), nil)
	t.Cleanup(mm.Stop)
	missionID, err := mm.CreateFlowMission("flow_c16cron00", "Zeitplan")
	if err != nil {
		t.Fatal(err)
	}
	flowJob := "mission_" + missionID + "__n_aaaaaaaa"
	claimed := "mission_" + missionID + "__n_bbbbbbbb"
	if _, err := cronMgr.ManageScheduleWithSource("add", flowJob, "0 9 * * *", "EasyDrag flow trigger", "en", "flow"); err != nil {
		t.Fatal(err)
	}
	if _, err := cronMgr.ManageScheduleWithSource("add", "agent_job", "0 8 * * *", "run agent", "en", ""); err != nil {
		t.Fatal(err)
	}
	s := &Server{CronManager: cronMgr, MissionManagerV2: mm, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	dashboard, byID, legacy := handleDashboardCronjobs(s), handleDashboardCronjobByID(s), handleCronAPI(s)

	w := c16CronRequest(dashboard, http.MethodGet, "/api/dashboard/cronjobs", "")
	var list struct {
		Jobs []map[string]any `json:"jobs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || w.Code != http.StatusOK {
		t.Fatalf("list = %d %s (%v)", w.Code, w.Body.String(), err)
	}
	for _, job := range list.Jobs {
		managed, has := job["managed_by"]
		if job["id"] == flowJob && managed != "easydrag" {
			t.Fatalf("flow job not marked: %v", job)
		}
		if job["id"] == "agent_job" && has {
			t.Fatalf("agent job marked: %v", job)
		}
	}

	refused := []struct {
		name                 string
		handler              http.Handler
		method, target, body string
	}{
		{"dashboard edit", dashboard, http.MethodPut, "/api/dashboard/cronjobs",
			`{"id":"` + flowJob + `","cron_expr":"0 10 * * *","task_prompt":"other"}`},
		{"dashboard toggle", dashboard, http.MethodPut, "/api/dashboard/cronjobs",
			`{"id":"` + flowJob + `","cron_expr":"0 9 * * *","task_prompt":"EasyDrag flow trigger","disabled":true}`},
		{"dashboard delete", byID, http.MethodDelete, "/api/dashboard/cronjobs/" + flowJob, ""},
		{"legacy add over a flow job", legacy, http.MethodPost, "/api/cron",
			`{"id":"` + flowJob + `","cron_expr":"0 7 * * *","task_prompt":"planted"}`},
		{"legacy add under a claimed id", legacy, http.MethodPost, "/api/cron",
			`{"id":"` + claimed + `","cron_expr":"0 7 * * *","task_prompt":"planted"}`},
		{"legacy edit", legacy, http.MethodPut, "/api/cron",
			`{"id":"` + flowJob + `","cron_expr":"0 10 * * *","task_prompt":"other","disabled":true}`},
		{"legacy delete", legacy, http.MethodDelete, "/api/cron?id=" + flowJob, ""},
	}
	for _, tc := range refused {
		w := c16CronRequest(tc.handler, tc.method, tc.target, tc.body)
		if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), flowCronManagedMessage) {
			t.Fatalf("%s = %d %s, want 409", tc.name, w.Code, w.Body.String())
		}
	}
	jobs := cronMgr.GetJobs()
	if len(jobs) != 2 {
		t.Fatalf("jobs after the refused changes: %+v", jobs)
	}
	for _, job := range jobs {
		if job.ID == flowJob && (job.CronExpr != "0 9 * * *" || job.Disabled || !job.IsFlowJob() || job.TaskPrompt != "EasyDrag flow trigger") {
			t.Fatalf("the flow job changed: %+v", job)
		}
	}
	// Other jobs stay editable.
	if w := c16CronRequest(byID, http.MethodDelete, "/api/dashboard/cronjobs/agent_job", ""); w.Code != http.StatusOK {
		t.Fatalf("delete agent job = %d %s", w.Code, w.Body.String())
	}
}

func TestC16FlowsDatabaseIsBackedUpAndProtected(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{}
	cfg.Directories.DataDir = filepath.Join(dir, "data")
	cfg.SQLite.GameMakerPath = filepath.Join(dir, "desktop", "game_maker.db")
	s := &Server{Cfg: cfg}
	want := filepath.Join(dir, "desktop", config.FlowsDBFilename)
	if got := s.flowsDBPath(); got != want {
		t.Fatalf("flowsDBPath = %q, want %q", got, want)
	}
	if !containsString(config.SQLiteDatabasePaths(cfg), want) {
		t.Fatalf("backups miss flows.db: %v", config.SQLiteDatabasePaths(cfg))
	}
	if protected := config.SQLiteProtectedPaths(cfg); !containsString(protected, want) || !containsString(protected, want+"-wal") {
		t.Fatalf("agent file tools may touch flows.db: %v", protected)
	}
	bare := &Server{Cfg: &config.Config{}}
	if got := bare.flowsDBPath(); got != filepath.Join("data", "flows.db") {
		t.Fatalf("flowsDBPath without a Game Maker path = %q", got)
	}
	for _, path := range config.SQLiteDatabasePaths(bare.Cfg) {
		if filepath.Base(path) == config.FlowsDBFilename {
			t.Fatalf("a relative flows.db is listed: %v", config.SQLiteDatabasePaths(bare.Cfg))
		}
	}
}
