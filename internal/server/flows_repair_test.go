package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFlowRepairMissionAPI(t *testing.T) {
	s, token := newFlowsTestServer(t)
	rec := createTestFlow(t, s, greetFlowJSON)
	if err := s.MissionManagerV2.DeleteFlowMission(rec.MissionID); err != nil {
		t.Fatal(err)
	}
	path := "/api/desktop/flows/" + rec.ID + "/repair-mission"
	body := fmt.Sprintf(`{"base_revision":%d,"base_live_revision":%d}`, rec.DraftRevision, rec.LiveRevision)
	for range 2 {
		w := flowsCall(t, s, http.MethodPost, path, token, body)
		if w.Code != http.StatusOK {
			t.Fatalf("repair = %d %s", w.Code, w.Body.String())
		}
	}
	mission, ok := s.MissionManagerV2.Get(rec.MissionID)
	if !ok || mission.FlowID != rec.ID || mission.Enabled {
		t.Fatalf("repaired mission = %+v", mission)
	}
	current, err := s.Flows.GetFlow(context.Background(), rec.ID)
	if err != nil || current.MissionID != rec.MissionID || current.DraftRevision != rec.DraftRevision {
		t.Fatalf("flow after repair = %+v, %v", current, err)
	}
	w := flowsCall(t, s, http.MethodPost, path, token, `{"base_revision":999,"base_live_revision":0}`)
	if w.Code != http.StatusConflict || flowsBody(t, w)["code"] != "FLOW_REVISION_CONFLICT" {
		t.Fatalf("stale repair = %d %s", w.Code, w.Body.String())
	}
}

func TestFlowMissionRepairAuditTypeIsRegistered(t *testing.T) {
	if !strings.Contains(readDashboardMarkupForTest(t), `<option value="flow_mission_repair" data-i18n="dashboard.audit_type_flow_mission_repair">`) {
		t.Error("the dashboard audit filter does not list flow_mission_repair")
	}
	locales, err := filepath.Glob(filepath.Join("..", "..", "ui", "lang", "dashboard", "*.json"))
	if err != nil || len(locales) != 16 {
		t.Fatalf("dashboard locales = %v, %v", locales, err)
	}
	for _, path := range locales {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var labels map[string]string
		if err := json.Unmarshal(data, &labels); err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(labels["dashboard.audit_type_flow_mission_repair"]) == "" {
			t.Errorf("%s has no repair audit label", path)
		}
	}
}
