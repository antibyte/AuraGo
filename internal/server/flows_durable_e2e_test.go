package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/flows"
	"aurago/internal/tools"
)

func TestDurableHTTPWaitAndGenerationFencedStop(t *testing.T) {
	s := c16Server(t, true, nil)
	s.Cfg.Flows.DurableEnabled = true
	s.Cfg.Auth.Enabled = false
	history, err := tools.InitMissionHistoryDB(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = history.Close() })
	s.MissionManagerV2.SetHistoryDB(history)
	s.MissionHistoryDB = history
	var completionCallbacks atomic.Int64
	s.MissionManagerV2.SetCompletionCallback(func(string, string, string) { completionCallbacks.Add(1) })
	s.initFlows()
	if s.Flows == nil {
		t.Fatal("flow service unavailable")
	}
	if err := s.MissionManagerV2.Start(); err != nil {
		t.Fatal(err)
	}
	s.startFlows(context.Background())
	t.Cleanup(func() { s.shutdownFlows(context.Background()) })
	doc := strings.Replace(waitFlowJSON, `"logic.wait"`, `"logic.durable_wait"`, 1)
	doc = strings.Replace(doc, `"mode":"duration","seconds":60`, `"seconds":60`, 1)
	rec := c16PublishedFlow(t, s, doc)
	if err := s.Flows.SetEnabled(context.Background(), rec.ID, true); err != nil {
		t.Fatal(err)
	}
	request := func(method, path, body, origin, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://example.test"+path, bytes.NewBufferString(body))
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		s.handleFlows(w, r)
		return w
	}
	start := request(http.MethodPost, "/api/desktop/flows/"+rec.ID+"/run", "", "http://example.test", "")
	if start.Code != http.StatusAccepted {
		t.Fatalf("start = %d %s", start.Code, start.Body.String())
	}
	var started struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(start.Body.Bytes(), &started); err != nil || started.RunID == "" {
		t.Fatalf("start response = %s, %v", start.Body.String(), err)
	}
	deadline := time.Now().Add(10 * time.Second)
	var status *flows.DurableRunStatus
	for time.Now().Before(deadline) {
		status, err = s.Flows.DurableRunStatus(context.Background(), started.RunID)
		if err == nil && status != nil && status.State == "waiting" && status.Generation > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if status == nil || status.State != "waiting" || status.Generation <= 0 {
		t.Fatalf("durable wait = %+v, %v", status, err)
	}
	path := "/api/desktop/flows/runs/" + started.RunID + "/durable/cancel"
	body := fmt.Sprintf(`{"generation":%d}`, status.Generation)
	if w := request(http.MethodPost, path, body, "http://foreign.test", ""); w.Code != http.StatusForbidden {
		t.Fatalf("foreign origin stop = %d %s", w.Code, w.Body.String())
	}
	readToken, _, err := s.TokenManager.Create("durable read", []string{desktopScopeRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if w := request(http.MethodPost, path, body, "", readToken); w.Code != http.StatusForbidden {
		t.Fatalf("read-only bearer stop = %d %s", w.Code, w.Body.String())
	}
	adminToken, _, err := s.TokenManager.Create("durable admin", []string{desktopScopeAdmin}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if w := request(http.MethodPost, path, fmt.Sprintf(`{"generation":%d}`, status.Generation+1), "", adminToken); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "FLOW_DURABLE_GENERATION_CONFLICT") {
		t.Fatalf("stale stop = %d %s", w.Code, w.Body.String())
	}
	if w := request(http.MethodPost, path, body+` {}`, "", adminToken); w.Code != http.StatusBadRequest {
		t.Fatalf("trailing body = %d %s", w.Code, w.Body.String())
	}
	for _, invalid := range []string{`{}`, `{"generation":null}`, `{"generation":-1}`, `{"generation":1,"extra":true}`} {
		if w := request(http.MethodPost, path, invalid, "", adminToken); w.Code != http.StatusBadRequest {
			t.Fatalf("invalid stop body %s = %d", invalid, w.Code)
		}
	}
	if w := request(http.MethodPost, path, `{"generation":1,"extra":"`+strings.Repeat("x", 5000)+`"}`, "", adminToken); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize stop = %d %s", w.Code, w.Body.String())
	}
	if w := request(http.MethodPost, path, body, "", adminToken); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"accepted":true`) {
		t.Fatalf("admin bearer stop = %d %s", w.Code, w.Body.String())
	}
	detail, err := s.Flows.Run(context.Background(), started.RunID, false)
	if err != nil || detail.Run.ID != started.RunID || detail.Run.Status != flows.RunCancelled {
		t.Fatalf("authoritative stopped run = %+v, %v", detail, err)
	}
	second := request(http.MethodPost, "/api/desktop/flows/"+rec.ID+"/run", "", "http://example.test", "")
	if second.Code != http.StatusAccepted || json.Unmarshal(second.Body.Bytes(), &started) != nil || started.RunID == "" {
		t.Fatalf("second start = %d %s", second.Code, second.Body.String())
	}
	deadline = time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		status, err = s.Flows.DurableRunStatus(context.Background(), started.RunID)
		if err == nil && status != nil && status.State == "waiting" && status.Generation > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if status == nil || status.State != "waiting" || status.Generation <= 0 {
		t.Fatalf("second durable wait = %+v, %v", status, err)
	}
	cfg := s.ConfigSnapshot().Clone()
	cfg.Flows.Enabled = false
	cfg.Flows.DurableEnabled = false
	cfg.Tools.Missions.ReadOnly = true
	cfg.VirtualDesktop.ReadOnly = true
	s.CfgMu.Lock()
	s.replaceConfigSnapshot(cfg)
	s.CfgMu.Unlock()
	blocked, err := s.Flows.DurableRunStatus(context.Background(), started.RunID)
	if err != nil || blocked == nil || blocked.State != "blocked" || blocked.Generation != status.Generation || !blocked.CancelAvailable {
		t.Fatalf("live-disable did not preserve the checkpoint: %+v, %v", blocked, err)
	}
	path = "/api/desktop/flows/runs/" + started.RunID + "/durable/cancel"
	body = fmt.Sprintf(`{"generation":%d}`, status.Generation)
	if w := request(http.MethodPost, path, body, "http://example.test", ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"accepted":true`) {
		t.Fatalf("session stop under readonly and app lock = %d %s", w.Code, w.Body.String())
	}
	// Restore only the in-memory test policy, then delete a run that was already
	// projected as waiting. No completion chain may be fabricated by deletion.
	cfg = cfg.Clone()
	cfg.Flows.Enabled, cfg.Flows.DurableEnabled = true, true
	cfg.Tools.Missions.ReadOnly, cfg.VirtualDesktop.ReadOnly = false, false
	s.CfgMu.Lock()
	s.replaceConfigSnapshot(cfg)
	s.CfgMu.Unlock()
	third := request(http.MethodPost, "/api/desktop/flows/"+rec.ID+"/run", "", "http://example.test", "")
	if third.Code != http.StatusAccepted || json.Unmarshal(third.Body.Bytes(), &started) != nil || started.RunID == "" {
		t.Fatalf("third start = %d %s", third.Code, third.Body.String())
	}
	deadline = time.Now().Add(10 * time.Second)
	var projected *tools.MissionRun
	for time.Now().Before(deadline) {
		projected, err = tools.GetMissionRun(history, tools.DurableHistoryPrefix+started.RunID)
		if err == nil && projected != nil && projected.Status == "waiting" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if projected == nil || projected.Status != "waiting" {
		t.Fatalf("waiting history projection = %+v, %v", projected, err)
	}
	if err := s.Flows.DeleteFlow(context.Background(), rec.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.MissionManagerV2.Get(rec.MissionID); ok {
		t.Fatal("deleted flow mission remains active")
	}
	projected, err = tools.GetMissionRun(history, tools.DurableHistoryPrefix+started.RunID)
	if err != nil || projected == nil || projected.Status != "waiting" || completionCallbacks.Load() != 0 {
		t.Fatalf("deleted source fabricated history/completion: %+v, %v, callbacks %d", projected, err, completionCallbacks.Load())
	}
	page, err := s.MissionManagerV2.QueryMissionHistory(context.Background(), tools.MissionHistoryFilter{MissionID: rec.MissionID})
	if err != nil {
		t.Fatal(err)
	}
	var orphan *tools.MissionRun
	for _, entry := range page.Entries {
		if entry.ID == tools.DurableHistoryPrefix+started.RunID {
			orphan = entry
			break
		}
	}
	if orphan == nil || orphan.Status != "unknown" || orphan.LastKnownStatus != "waiting" {
		t.Fatalf("deleted source was not reported unknown: %+v", orphan)
	}
	historyResponse := httptest.NewRecorder()
	serveMissionHistory(s).ServeHTTP(historyResponse, httptest.NewRequest(http.MethodGet, "/api/missions/v2/history?mission_id="+rec.MissionID, nil))
	if historyResponse.Code != http.StatusOK || !strings.Contains(historyResponse.Body.String(), `"status":"unknown"`) || !strings.Contains(historyResponse.Body.String(), `"last_known_status":"waiting"`) {
		t.Fatalf("visible history after delete = %d %s", historyResponse.Code, historyResponse.Body.String())
	}
	if err := s.MissionManagerV2.EnsureRestoredFlowMission(rec.MissionID, rec.ID, "restored", nil, true); err != nil {
		t.Fatal(err)
	}
	mission, ok := s.MissionManagerV2.Get(rec.MissionID)
	if !ok || mission.Status != tools.MissionStatusIdle || mission.RunCount != 0 {
		t.Fatalf("deleted durable reservation survived mission restoration: %+v", mission)
	}
}

func TestDurableHTTPHistoryOutageReconcilesAfterDefaultOffRestart(t *testing.T) {
	s := c16Server(t, true, nil)
	s.Cfg.Flows.DurableEnabled = true
	s.Cfg.Auth.Enabled = false
	historyPath := filepath.Join(t.TempDir(), "history.db")
	history, err := tools.InitMissionHistoryDB(historyPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := history.Close(); err != nil {
		t.Fatal(err)
	}
	s.MissionManagerV2.SetHistoryDB(history)
	var callbacks atomic.Int64
	s.MissionManagerV2.SetCompletionCallback(func(string, string, string) { callbacks.Add(1) })
	s.initFlows()
	if s.Flows == nil {
		t.Fatal("flow service unavailable")
	}
	if err := s.MissionManagerV2.Start(); err != nil {
		t.Fatal(err)
	}
	s.startFlows(context.Background())
	t.Cleanup(func() {
		s.shutdownFlows(context.Background())
		_ = history.Close()
	})
	doc := strings.Replace(waitFlowJSON, `"logic.wait"`, `"logic.durable_wait"`, 1)
	doc = strings.Replace(doc, `"mode":"duration","seconds":60`, `"seconds":1`, 1)
	rec := c16PublishedFlow(t, s, doc)
	if err := s.Flows.SetEnabled(context.Background(), rec.ID, true); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "http://example.test/api/desktop/flows/"+rec.ID+"/run", nil)
	r.Header.Set("Origin", "http://example.test")
	w := httptest.NewRecorder()
	s.handleFlows(w, r)
	var started struct {
		RunID string `json:"run_id"`
	}
	if w.Code != http.StatusAccepted || json.Unmarshal(w.Body.Bytes(), &started) != nil || started.RunID == "" {
		t.Fatalf("start = %d %s", w.Code, w.Body.String())
	}
	deadline := time.Now().Add(15 * time.Second)
	var before *flows.RunDetail
	sawWaiting := false
	for time.Now().Before(deadline) {
		before, err = s.Flows.Run(context.Background(), started.RunID, false)
		if err == nil && before.Run.Status == flows.RunWaiting {
			sawWaiting = true
		}
		if err == nil && before.Run.Status == flows.RunSuccess {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !sawWaiting || before == nil || before.Run.Status != flows.RunSuccess {
		t.Fatalf("authoritative run did not finish once after waiting: %+v, %v", before, err)
	}
	if err := (flowMissionBridge{s: s}).ProjectDurableRun(context.Background(), started.RunID); err == nil {
		t.Fatal("closed history database falsely reported a successful projection")
	}
	s.shutdownFlows(context.Background())
	cfg := s.ConfigSnapshot().Clone()
	cfg.Flows.DurableEnabled = false
	s.CfgMu.Lock()
	s.replaceConfigSnapshot(cfg)
	s.CfgMu.Unlock()
	history, err = tools.InitMissionHistoryDB(historyPath)
	if err != nil {
		t.Fatal(err)
	}
	s.MissionManagerV2.SetHistoryDB(history)
	s.MissionHistoryDB = history
	s.initFlows()
	if s.Flows == nil {
		t.Fatal("default-off flow service unavailable")
	}
	s.startFlows(context.Background())
	deadline = time.Now().Add(10 * time.Second)
	var projected *tools.MissionRun
	for time.Now().Before(deadline) {
		projected, err = tools.GetMissionRun(history, tools.DurableHistoryPrefix+started.RunID)
		if err == nil && projected != nil && projected.Status == "success" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if projected == nil || projected.Status != "success" {
		t.Fatalf("terminal history was not reconstructed: %+v, %v", projected, err)
	}
	if err := (flowMissionBridge{s: s}).ProjectDurableRun(context.Background(), started.RunID); err != nil {
		t.Fatalf("repeated projection: %v", err)
	}
	after, err := s.Flows.Run(context.Background(), started.RunID, false)
	if err != nil || after.Run.ID != before.Run.ID || after.Run.Status != flows.RunSuccess || !reflect.DeepEqual(after.Steps, before.Steps) {
		t.Fatalf("reconciliation changed the authoritative run: %+v, %v", after, err)
	}
	runs, err := s.Flows.Runs(context.Background(), rec.ID, flows.RunFilter{})
	if err != nil || len(runs) != 1 || callbacks.Load() != 0 {
		t.Fatalf("reconciliation repeated execution/completion: runs=%d callbacks=%d err=%v", len(runs), callbacks.Load(), err)
	}
	mission, ok := s.MissionManagerV2.Get(rec.MissionID)
	if !ok || mission.RunCount != 0 {
		t.Fatalf("durable projection changed mission accounting: %+v", mission)
	}
}
