package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/flows"
	"aurago/internal/tools"
)

const greetFlowJSON = `{"schema":1,"name":"Gruss","nodes":[
 {"id":"n_aaaaaaaa","key":"start","type":"trigger.manual","type_version":1,"label":"Start","position":{"x":0,"y":0},"params":{"data":{"name":"Welt"}}},
 {"id":"n_bbbbbbbb","key":"greet","type":"logic.set","type_version":1,"label":"Gruss","position":{"x":300,"y":0},
  "params":{"fields":[{"name":"greeting","value":"Hallo {{trigger.data.name}}"}]}}],
 "edges":[{"id":"e_aaaaaaaa","source":{"node":"n_aaaaaaaa","port":"out"},"target":{"node":"n_bbbbbbbb","port":"in"}}]}`

const waitFlowJSON = `{"schema":1,"name":"Pause","nodes":[
 {"id":"n_aaaaaaaa","key":"start","type":"trigger.manual","type_version":1,"label":"Start","position":{"x":0,"y":0},"params":{}},
 {"id":"n_bbbbbbbb","key":"pause","type":"logic.wait","type_version":1,"label":"Pause","position":{"x":300,"y":0},
  "params":{"mode":"duration","seconds":60}}],
 "edges":[{"id":"e_aaaaaaaa","source":{"node":"n_aaaaaaaa","port":"out"},"target":{"node":"n_bbbbbbbb","port":"in"}}]}`

// newFlowsTestServer builds a server with a real flow service and mission manager and
// returns it with a desktop token that may read, write and administer (flows writes other
// than validate need desktop:admin, see flowsRequiredScope).
func newFlowsTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	s, _, _ := testDesktopPermissionServer(t)
	dir := t.TempDir()
	s.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	s.Cfg.Flows.Enabled = true
	s.Cfg.Tools.Missions.Enabled = true
	s.Cfg.Directories.DataDir = dir
	s.Cfg.Directories.WorkspaceDir = filepath.Join(dir, "workspace")
	s.Cfg.Directories.ToolsDir = filepath.Join(dir, "tools")
	s.Cfg.Directories.SkillsDir = filepath.Join(dir, "skills")
	s.Cfg.SQLite.GameMakerPath = filepath.Join(dir, "game_maker.db")
	tools.ConfigureRuntimePermissions(tools.RuntimePermissionsFromConfig(s.Cfg))
	s.MissionManagerV2 = tools.NewMissionManagerV2(dir, nil)
	s.initFlows()
	if s.Flows == nil {
		t.Fatal("flows were not initialised")
	}
	if err := s.MissionManagerV2.Start(); err != nil {
		t.Fatalf("missions: %v", err)
	}
	t.Cleanup(s.MissionManagerV2.Stop)
	s.startFlows(context.Background())
	t.Cleanup(func() { s.shutdownFlows(context.Background()) })
	token, _, err := s.TokenManager.Create("flows test", []string{desktopScopeRead, desktopScopeWrite, desktopScopeAdmin}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return s, token
}

// createTestFlow creates a flow from a JSON document and returns its record.
func createTestFlow(t *testing.T, s *Server, docJSON string) *flows.FlowRecord {
	t.Helper()
	doc, err := flows.ParseFlow([]byte(docJSON))
	if err != nil {
		t.Fatalf("ParseFlow: %v", err)
	}
	rec, err := s.Flows.CreateFlow(context.Background(), flows.CreateRequest{Import: doc})
	if err != nil {
		t.Fatalf("CreateFlow: %v", err)
	}
	return rec
}

func TestInitFlowsWiresMissionControlAndCapability(t *testing.T) {
	s, _ := newFlowsTestServer(t)
	if !(desktopCapabilities{s: s}).HasCapability("flows") {
		t.Fatal("capability flows must be present")
	}
	rec := createTestFlow(t, s, greetFlowJSON)
	m, ok := s.MissionManagerV2.Get(rec.MissionID)
	if !ok || m.ExecutionType != tools.ExecutionFlow || m.FlowID != rec.ID {
		t.Fatalf("flow mission = %+v %v", m, ok)
	}
	s.Cfg.Flows.Enabled = false
	if (desktopCapabilities{s: s}).HasCapability("flows") {
		t.Fatal("capability flows must follow flows.enabled")
	}
}

func TestMissionControlCancelsFlowRuns(t *testing.T) {
	s, _ := newFlowsTestServer(t)
	ctx := context.Background()
	rec := createTestFlow(t, s, waitFlowJSON)
	if _, _, err := s.Flows.Publish(ctx, rec.ID, rec.DraftRevision); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	started, err := s.Flows.RunNow(ctx, rec.ID)
	if err != nil {
		t.Fatalf("RunNow: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		if m, _ := s.MissionManagerV2.Get(rec.MissionID); m.Status == tools.MissionStatusRunning {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the flow mission never ran")
		}
		time.Sleep(10 * time.Millisecond)
	}
	w := httptest.NewRecorder()
	handleMissionCancelV2(s, w, httptest.NewRequest(http.MethodPost, "/api/missions/v2/"+rec.MissionID+"/cancel", nil), rec.MissionID)
	if w.Code != http.StatusAccepted {
		t.Fatalf("cancel = %d %s", w.Code, w.Body.String())
	}
	for {
		detail, _ := s.Flows.Run(ctx, started.RunID, false)
		if detail != nil && detail.Run.Status == flows.RunCancelled {
			break
		}
		if time.Now().After(deadline.Add(3 * time.Second)) {
			t.Fatalf("run not cancelled: %+v", detail)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
