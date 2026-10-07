package server

import (
	"bytes"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/tools"
)

// FF1: a panic while the flow service is set up (flows are on by default) is logged and
// leaves the server running without flows: s.Flows stays nil, the capability and the API
// are off, the store is closed again and Mission Control gets no flow hooks.
func TestFF1InitFlowsSurvivesAPanic(t *testing.T) {
	s, _, _ := testDesktopPermissionServer(t)
	dir := t.TempDir()
	var logs bytes.Buffer
	s.Logger = slog.New(slog.NewTextHandler(&logs, nil))
	s.Cfg.Flows.Enabled = true
	s.Cfg.Tools.Missions.Enabled = true
	s.Cfg.Directories.DataDir = dir
	s.Cfg.Directories.WorkspaceDir = filepath.Join(dir, "workspace")
	s.Cfg.SQLite.GameMakerPath = filepath.Join(dir, "game_maker.db")
	tools.ConfigureRuntimePermissions(tools.RuntimePermissionsFromConfig(s.Cfg))
	s.MissionManagerV2 = tools.NewMissionManagerV2(dir, nil)
	flowsInitTestHook = func() { panic("ff1 injected panic") }
	t.Cleanup(func() { flowsInitTestHook = nil })

	s.initFlows()

	if s.Flows != nil || s.flowsCatalog != nil {
		t.Fatal("a failed set-up must leave the flow service unset")
	}
	if (desktopCapabilities{s: s}).HasCapability("flows") {
		t.Fatal("the flows capability must be off")
	}
	if !strings.Contains(logs.String(), "level=ERROR") || !strings.Contains(logs.String(), "ff1 injected panic") {
		t.Fatalf("log = %s", logs.String())
	}
	token := ff1Token(t, s, "ff1 init", desktopScopeRead)
	if w := flowsCall(t, s, http.MethodGet, "/api/desktop/flows", token, ""); w.Code != http.StatusServiceUnavailable ||
		flowsBody(t, w)["code"] != "FLOWS_DISABLED" {
		t.Fatalf("flows API after a failed set-up = %d %s", w.Code, w.Body.String())
	}
	// The store was closed: on Windows an open SQLite file cannot be removed.
	if err := os.Remove(s.flowsDBPath()); err != nil && !os.IsNotExist(err) {
		t.Fatalf("the flow store is still open: %v", err)
	}
	// Without hooks a flow mission never runs as an agent mission (tools.ErrFlowsUnavailable).
	if err := s.MissionManagerV2.Start(); err != nil {
		t.Fatalf("missions: %v", err)
	}
	t.Cleanup(s.MissionManagerV2.Stop)
	id, err := s.MissionManagerV2.CreateFlowMission("flow_ff1aaaaaaa", "FF1")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.MissionManagerV2.SyncFlowMission(id, "FF1", nil); err != nil {
		t.Fatal(err)
	}
	if err := s.MissionManagerV2.SetFlowMissionEnabled(id, true); err != nil {
		t.Fatal(err)
	}
	if err := s.MissionManagerV2.RunNow(id); err == nil || !strings.Contains(err.Error(), tools.ErrFlowsUnavailable.Error()) {
		t.Fatalf("RunNow of a flow mission without hooks = %v", err)
	}
}
