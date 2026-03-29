package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"aurago/internal/memory"
)

func newTestPlanHandlerServer(t *testing.T) *Server {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { stm.Close() })
	return &Server{ShortTermMem: stm, Logger: logger}
}

func seedPlanForServer(t *testing.T, s *Server) *memory.Plan {
	t.Helper()
	plan, err := s.ShortTermMem.CreatePlan("default", "Server Plan", "desc", "request", 2, []memory.PlanTaskInput{
		{Title: "Inspect", Kind: "reasoning"},
		{Title: "Patch", Kind: "tool", DependsOn: []string{"1"}},
	})
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}
	return plan
}

func TestHandlePlansListReturnsPlans(t *testing.T) {
	s := newTestPlanHandlerServer(t)
	seedPlanForServer(t, s)

	req := httptest.NewRequest(http.MethodGet, "/api/plans?session_id=default&status=all", nil)
	rec := httptest.NewRecorder()
	handlePlansList(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"status":"ok"`) || !strings.Contains(body, "Server Plan") {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestHandlePlanByIDAdvanceAndBlock(t *testing.T) {
	s := newTestPlanHandlerServer(t)
	plan := seedPlanForServer(t, s)
	plan, err := s.ShortTermMem.SetPlanStatus(plan.ID, memory.PlanStatusActive, "")
	if err != nil {
		t.Fatalf("SetPlanStatus active: %v", err)
	}

	blockReqBody, _ := json.Marshal(map[string]string{"reason": "waiting for review"})
	blockReq := httptest.NewRequest(http.MethodPost, "/api/plans/"+plan.ID+"/tasks/"+plan.Tasks[0].ID+"/block", bytes.NewReader(blockReqBody))
	blockRec := httptest.NewRecorder()
	handlePlanByID(s).ServeHTTP(blockRec, blockReq)
	if blockRec.Code != http.StatusOK {
		t.Fatalf("block status = %d, want 200; body=%s", blockRec.Code, blockRec.Body.String())
	}
	if !strings.Contains(blockRec.Body.String(), `"blocked_reason":"waiting for review"`) {
		t.Fatalf("expected blocked reason in body: %s", blockRec.Body.String())
	}

	unblockReqBody, _ := json.Marshal(map[string]string{"note": "review done"})
	unblockReq := httptest.NewRequest(http.MethodPost, "/api/plans/"+plan.ID+"/tasks/"+plan.Tasks[0].ID+"/unblock", bytes.NewReader(unblockReqBody))
	unblockRec := httptest.NewRecorder()
	handlePlanByID(s).ServeHTTP(unblockRec, unblockReq)
	if unblockRec.Code != http.StatusOK {
		t.Fatalf("unblock status = %d, want 200; body=%s", unblockRec.Code, unblockRec.Body.String())
	}

	advanceReqBody, _ := json.Marshal(map[string]string{"result": "inspection finished"})
	advanceReq := httptest.NewRequest(http.MethodPost, "/api/plans/"+plan.ID+"/advance", bytes.NewReader(advanceReqBody))
	advanceRec := httptest.NewRecorder()
	handlePlanByID(s).ServeHTTP(advanceRec, advanceReq)
	if advanceRec.Code != http.StatusOK {
		t.Fatalf("advance status = %d, want 200; body=%s", advanceRec.Code, advanceRec.Body.String())
	}
	if !strings.Contains(advanceRec.Body.String(), "inspection finished") {
		t.Fatalf("expected result summary in body: %s", advanceRec.Body.String())
	}
}

func TestHandlePlanSplitReorderAndArchive(t *testing.T) {
	s := newTestPlanHandlerServer(t)
	plan := seedPlanForServer(t, s)
	plan, err := s.ShortTermMem.SetPlanStatus(plan.ID, memory.PlanStatusActive, "")
	if err != nil {
		t.Fatalf("SetPlanStatus active: %v", err)
	}

	splitReqBody, _ := json.Marshal(map[string]interface{}{
		"items": []map[string]string{
			{"title": "Inspect logs"},
			{"title": "Verify result"},
		},
	})
	splitReq := httptest.NewRequest(http.MethodPost, "/api/plans/"+plan.ID+"/tasks/"+plan.Tasks[0].ID+"/split", bytes.NewReader(splitReqBody))
	splitRec := httptest.NewRecorder()
	handlePlanByID(s).ServeHTTP(splitRec, splitReq)
	if splitRec.Code != http.StatusOK {
		t.Fatalf("split status = %d, want 200; body=%s", splitRec.Code, splitRec.Body.String())
	}
	updated, err := s.ShortTermMem.GetPlan(plan.ID)
	if err != nil {
		t.Fatalf("GetPlan after split: %v", err)
	}
	order := []string{updated.Tasks[0].ID, updated.Tasks[2].ID, updated.Tasks[1].ID, updated.Tasks[3].ID}
	reorderReqBody, _ := json.Marshal(map[string]interface{}{"task_ids": order})
	reorderReq := httptest.NewRequest(http.MethodPost, "/api/plans/"+plan.ID+"/reorder", bytes.NewReader(reorderReqBody))
	reorderRec := httptest.NewRecorder()
	handlePlanByID(s).ServeHTTP(reorderRec, reorderReq)
	if reorderRec.Code != http.StatusOK {
		t.Fatalf("reorder status = %d, want 200; body=%s", reorderRec.Code, reorderRec.Body.String())
	}

	completed, err := s.ShortTermMem.SetPlanStatus(plan.ID, memory.PlanStatusCompleted, "done")
	if err != nil {
		t.Fatalf("SetPlanStatus completed: %v", err)
	}
	archiveReq := httptest.NewRequest(http.MethodPost, "/api/plans/"+completed.ID+"/archive", bytes.NewReader([]byte(`{}`)))
	archiveRec := httptest.NewRecorder()
	handlePlanByID(s).ServeHTTP(archiveRec, archiveReq)
	if archiveRec.Code != http.StatusOK {
		t.Fatalf("archive status = %d, want 200; body=%s", archiveRec.Code, archiveRec.Body.String())
	}
	if !strings.Contains(archiveRec.Body.String(), `"archived":true`) {
		t.Fatalf("expected archived plan in body: %s", archiveRec.Body.String())
	}
}

func TestHandlePlanByIDMissingPlanReturnsGenericError(t *testing.T) {
	s := newTestPlanHandlerServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/plans/missing-plan", nil)
	rec := httptest.NewRecorder()

	handlePlanByID(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"message":"Plan not found"`) || strings.Contains(strings.ToLower(body), "sql") {
		t.Fatalf("expected generic not-found error, got %s", body)
	}
}
