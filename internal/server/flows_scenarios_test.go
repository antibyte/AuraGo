package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/security"
)

func TestFlowScenariosAPI(t *testing.T) {
	s, token := newFlowsTestServer(t)
	var err error
	s.Vault, err = security.NewVault(strings.Repeat("a", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	rec := createTestFlow(t, s, greetFlowJSON)
	base := "/api/desktop/flows/" + rec.ID
	body := fmt.Sprintf(`{"base_revision":%d,"expected_version":0,"scenario":{"name":"Case","trigger_node":"n_aaaaaaaa","trigger_data":{"name":"private-sample"},"fixtures":{"n_bbbbbbbb":{"output":{"greeting":"fixture-result"},"ports":["out"]}},"assertions":[{"node_id":"n_bbbbbbbb","output":{"greeting":"fixture-result"},"ports":["out"]}]}}`, rec.DraftRevision)
	created := flowsCall(t, s, http.MethodPost, base+"/scenarios", token, body)
	if created.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", created.Code, created.Body.String())
	}
	item := flowsBody(t, created)["scenario"].(map[string]any)
	id := item["id"].(string)
	readToken, _, err := s.TokenManager.Create("read only", []string{desktopScopeRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if w := flowsCall(t, s, http.MethodGet, base+"/scenarios", readToken, ""); w.Code != http.StatusForbidden {
		t.Fatalf("read-only bearer read private scenarios: %d %s", w.Code, w.Body.String())
	}
	list := flowsCall(t, s, http.MethodGet, base+"/scenarios", token, "")
	if list.Code != http.StatusOK || strings.Contains(list.Body.String(), "private-sample") || strings.Contains(list.Body.String(), "fixture-result") {
		t.Fatalf("collection leaked sample: %d %s", list.Code, list.Body.String())
	}
	rename := flowsCall(t, s, http.MethodPatch, base+"/scenarios/"+id, token, `{"name":"Renamed","expected_version":1}`)
	if rename.Code != http.StatusOK {
		t.Fatalf("rename = %d %s", rename.Code, rename.Body.String())
	}
	loaded := flowsCall(t, s, http.MethodGet, base+"/scenarios/"+id, token, "")
	if loaded.Code != http.StatusOK || !strings.Contains(loaded.Body.String(), "private-sample") || !strings.Contains(loaded.Body.String(), "fixture-result") {
		t.Fatalf("renamed case lost data: %d %s", loaded.Code, loaded.Body.String())
	}
	simBody := fmt.Sprintf(`{"base_revision":%d,"scenario_id":%q,"scenario_version":2}`, rec.DraftRevision, id)
	simulated := flowsCall(t, s, http.MethodPost, base+"/simulate", token, simBody)
	var result struct {
		Mode       string `json:"mode"`
		Assertions []struct {
			Passed bool `json:"passed"`
		} `json:"assertions"`
	}
	if err := json.Unmarshal(simulated.Body.Bytes(), &result); err != nil || simulated.Code != http.StatusOK || result.Mode != "simulation" || len(result.Assertions) != 1 || !result.Assertions[0].Passed {
		t.Fatalf("scenario simulation = %d %s, %v", simulated.Code, simulated.Body.String(), err)
	}
	invalid := fmt.Sprintf(`{"base_revision":%d,"scenario_id":%q,"scenario_version":2,"trigger_data":{}}`, rec.DraftRevision, id)
	if w := flowsCall(t, s, http.MethodPost, base+"/simulate", token, invalid); w.Code != http.StatusBadRequest {
		t.Fatalf("mixed inputs = %d %s", w.Code, w.Body.String())
	}
	if w := flowsCall(t, s, http.MethodDelete, base+"/scenarios/"+id, token, `{"expected_version":1}`); w.Code != http.StatusConflict {
		t.Fatalf("stale delete = %d %s", w.Code, w.Body.String())
	}
	if w := flowsCall(t, s, http.MethodDelete, base+"/scenarios/"+id, token, `{"expected_version":2}`); w.Code != http.StatusOK {
		t.Fatalf("delete = %d %s", w.Code, w.Body.String())
	}
	if _, err := s.Vault.ReadSecret(flowScenarioVaultKey); err != nil {
		t.Fatal("deleting the last case removed its key", err)
	}
	if _, err := s.Flows.GetFlow(context.Background(), rec.ID); err != nil {
		t.Fatal("deleting a scenario removed its flow", err)
	}
}
