package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
)

func TestHandleMirageStatusDisabled(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	rec := httptest.NewRecorder()
	handleMirageStatus(s)(rec, httptest.NewRequest(http.MethodGet, "/api/mirage/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["status"] != "disabled" {
		t.Fatalf("body = %s (%v)", rec.Body.String(), err)
	}
}

func TestHandleMirageStatusRejectsOtherMethods(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	rec := httptest.NewRecorder()
	handleMirageStatus(s)(rec, httptest.NewRequest(http.MethodPost, "/api/mirage/status", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code = %d, want 405", rec.Code)
	}
}

func TestMirageInputsChanged(t *testing.T) {
	a, b := &config.Config{}, &config.Config{}
	if mirageInputsChanged(a, b) {
		t.Fatal("equal configs must not report a change")
	}
	b.Vision.Mirage.GPU = "nvidia"
	if !mirageInputsChanged(a, b) {
		t.Fatal("a GPU change must be reported")
	}
}
