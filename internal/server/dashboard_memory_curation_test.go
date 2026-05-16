package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"
)

func TestHandleDashboardMemoryCurationDryRunAndApply(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	if err := stm.UpsertMemoryMetaWithDetails("doc-confirm", memory.MemoryMetaUpdate{
		ExtractionConfidence: 0.96,
		VerificationStatus:   "unverified",
		SourceReliability:    0.92,
	}); err != nil {
		t.Fatalf("UpsertMemoryMetaWithDetails: %v", err)
	}
	if err := stm.RecordMemoryUsage("doc-confirm", "ltm_retrieved", "sess-1", 0.9, true); err != nil {
		t.Fatalf("RecordMemoryUsage: %v", err)
	}

	cfg := &config.Config{}
	cfg.MemoryAnalysis.AutoConfirm = 0.92
	s := &Server{ShortTermMem: stm, Cfg: cfg, Logger: logger}

	dryReq := httptest.NewRequest(http.MethodPost, "/api/dashboard/memory/curation/dry-run", bytes.NewReader([]byte(`{"limit":100}`)))
	dryRec := httptest.NewRecorder()
	handleDashboardMemoryCuration(s).ServeHTTP(dryRec, dryReq)
	if dryRec.Code != http.StatusOK {
		t.Fatalf("dry-run status = %d, want 200; body=%s", dryRec.Code, dryRec.Body.String())
	}
	var dryBody map[string]interface{}
	if err := json.Unmarshal(dryRec.Body.Bytes(), &dryBody); err != nil {
		t.Fatalf("decode dry-run: %v", err)
	}
	if int(dryBody["auto_confirm_count"].(float64)) != 1 {
		t.Fatalf("auto_confirm_count = %v, want 1", dryBody["auto_confirm_count"])
	}

	badApply := httptest.NewRequest(http.MethodPost, "/api/dashboard/memory/curation/apply", bytes.NewReader([]byte(`{"limit":100}`)))
	badRec := httptest.NewRecorder()
	handleDashboardMemoryCuration(s).ServeHTTP(badRec, badApply)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("apply without confirm status = %d, want 400", badRec.Code)
	}

	applyReq := httptest.NewRequest(http.MethodPost, "/api/dashboard/memory/curation/apply", bytes.NewReader([]byte(`{"limit":100,"confirm":"APPLY_MEMORY_CURATION"}`)))
	applyRec := httptest.NewRecorder()
	handleDashboardMemoryCuration(s).ServeHTTP(applyRec, applyReq)
	if applyRec.Code != http.StatusOK {
		t.Fatalf("apply status = %d, want 200; body=%s", applyRec.Code, applyRec.Body.String())
	}
	metas, err := stm.GetAllMemoryMeta(10, 0)
	if err != nil {
		t.Fatalf("GetAllMemoryMeta: %v", err)
	}
	if metas[0].VerificationStatus != "confirmed" {
		t.Fatalf("VerificationStatus = %q, want confirmed", metas[0].VerificationStatus)
	}
}
