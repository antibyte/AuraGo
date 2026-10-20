package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/memory"
)

func TestPersonalityDesktopHumanInputPreservesRawAndEmptySources(t *testing.T) {
	for _, raw := range []string{"Please check this image", ""} {
		t.Run(raw, func(t *testing.T) {
			s := newTestDesktopChatServer(t)
			turn, err := prepareDesktopAgentTurnWithOptions(context.Background(), s, "Enriched image description from a model", desktopChatContext{}, false, desktopAgentTurnOptions{TaskRoutingText: &raw})
			if err != nil {
				t.Fatal(err)
			}
			input := turn.runCfg.HumanInput
			if !input.Valid() || input.Text != raw || input.SessionID != turn.runCfg.SessionID {
				t.Fatalf("enrichment replaced the human source: %+v", input)
			}
		})
	}
}

func personalityFileServer(t *testing.T) (*Server, *sql.DB) {
	t.Helper()
	s := newTestServerWithPersonalityState(t)
	path := filepath.Join(t.TempDir(), "memory.db")
	stm, err := memory.NewSQLiteMemory(path, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	s.ShortTermMem = stm
	t.Cleanup(func() { _ = stm.Close() })
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return s, db
}

func TestPersonalityFeedbackEventContract(t *testing.T) {
	s, db := personalityFileServer(t)
	post := func(body string, want int) {
		t.Helper()
		rec := httptest.NewRecorder()
		handlePersonalityFeedback(s)(rec, httptest.NewRequest(http.MethodPost, "/api/personality/feedback", strings.NewReader(body)))
		if rec.Code != want {
			t.Fatalf("got %d want %d: %s", rec.Code, want, rec.Body.String())
		}
	}
	occurred := time.Now().UTC().Format(time.RFC3339Nano)
	payload := `{"type":"positive","event_id":"stable","occurred_at":"` + occurred + `"}`
	post(`{"type":"positive","event_id":"new"}`, 400)
	post(`{"type":"positive","event_id":"bad","occurred_at":"yesterday"}`, 400)
	post(payload, 200)
	first, _ := s.ShortTermMem.GetPersonalitySnapshotAt(time.Now())
	post(payload, 200)
	second, _ := s.ShortTermMem.GetPersonalitySnapshotAt(time.Now())
	if second.Dynamics.Revision != first.Dynamics.Revision {
		t.Fatal("repeated feedback applied twice")
	}
	post(strings.Replace(payload, `"positive"`, `"negative"`, 1), 409)
	post(`{"type":"positive","event_id":"stable"}`, 400)
	old := time.Now().Add(-memory.PersonalityObservationLifetime - time.Second).UTC().Format(time.RFC3339Nano)
	post(`{"type":"positive","event_id":"old","occurred_at":"`+old+`"}`, 410)
	post(`{"type":"positive","event_id":"zero","occurred_at":"0001-01-01T00:00:00Z"}`, 410)
	future := time.Now().Add(6 * time.Minute).UTC().Format(time.RFC3339Nano)
	post(`{"type":"positive","event_id":"future","occurred_at":"`+future+`"}`, 400)
	// Known pre-upgrade IDs are no-op replays without a supplied timestamp.
	if _, err := db.Exec(`UPDATE personality_observations SET primary_hash=''`); err != nil {
		t.Fatal(err)
	}
	post(`{"type":"positive","event_id":"stable"}`, 200)
	if _, err := db.Exec(`UPDATE personality_observations SET occurred_at=?`, old); err != nil {
		t.Fatal(err)
	}
	post(`{"type":"positive","event_id":"stable"}`, 410)
	post(`{"type":"positive"}`, 200)
}

func TestPersonalityLegacyReviewAPIAndAdminPolicy(t *testing.T) {
	s, db := personalityFileServer(t)
	if _, err := db.Exec(`INSERT INTO personality_legacy_review(kind,payload,fingerprint) VALUES('profile','{"category":"prefs","key":"format","value":"concise"}','profile-v1'),('emotion','{"description":"Old feeling"}','emotion-v1')`); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	handlePersonalityLegacy(s)(rec, httptest.NewRequest("GET", "/api/personality/legacy?limit=1", nil))
	var page struct {
		Entries []memory.PersonalityLegacyEntry `json:"entries"`
		HasMore bool                            `json:"has_more"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil || len(page.Entries) != 1 || !page.HasMore {
		t.Fatalf("pagination: %s %v", rec.Body.String(), err)
	}
	post := func(body string, want int) {
		t.Helper()
		rec := httptest.NewRecorder()
		handlePersonalityLegacyReview(s)(rec, httptest.NewRequest("POST", "/api/personality/legacy/review", strings.NewReader(body)))
		if rec.Code != want {
			t.Fatalf("review %d want %d: %s", rec.Code, want, rec.Body.String())
		}
	}
	post(`{"id":1,"fingerprint":"stale","action":"adopt"}`, 409)
	legacy, err := s.ShortTermMem.ListPersonalityLegacy(t.Context(), 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	body := func(entry memory.PersonalityLegacyEntry, action string) string {
		data, _ := json.Marshal(map[string]any{"id": entry.ID, "fingerprint": entry.Fingerprint, "action": action})
		return string(data)
	}
	post(body(legacy[1], "adopt"), 400)
	if _, err := db.Exec(`UPDATE personality_legacy_review SET payload='{"category":"prefs","key":"format","value":"terse"}' WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	post(body(legacy[0], "adopt"), 409)
	legacy, err = s.ShortTermMem.ListPersonalityLegacy(t.Context(), 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	post(body(legacy[0], "adopt"), 200)
	post(body(legacy[0], "discard"), 409)
	entries, _ := s.ShortTermMem.GetProfileEntries("prefs")
	if len(entries) != 1 || entries[0].Source != "manual" {
		t.Fatal("adoption skipped profile validation")
	}
	for _, path := range []string{"/api/personality/legacy", "/api/personality/legacy/review"} {
		if !isAdminProtectedPath(path) {
			t.Fatal("legacy review lacks admin gate")
		}
		s.Cfg.Auth.Enabled = true
		blocked := httptest.NewRecorder()
		requireAdmin(s, handlePersonalityLegacy(s)).ServeHTTP(blocked, httptest.NewRequest("GET", path, nil))
		if blocked.Code < 400 {
			t.Fatal("unauthenticated legacy access")
		}
	}
}

func TestPersonalityConfigLegacySecretsAreHiddenAndRejected(t *testing.T) {
	s, _ := newProviderTestServer(t, "server:\n  host: 127.0.0.1\n")
	legacyYAML := []byte("personality:\n  v2_api_key: legacy-personality-synthetic\nagent:\n  personality_v2_api_key: legacy-agent-synthetic\n")
	if err := os.WriteFile(s.Cfg.ConfigPath, legacyYAML, 0600); err != nil {
		t.Fatal(err)
	}
	for _, handler := range []http.HandlerFunc{handleGetConfig(s), handleGetConfigSchema(s)} {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))
		if rec.Code != 200 || strings.Contains(rec.Body.String(), "legacy-personality-synthetic") || strings.Contains(rec.Body.String(), "legacy-agent-synthetic") {
			t.Fatal("GET disclosed legacy credential or failed")
		}
	}
	for _, body := range []string{`{"personality":{"v2_api_key":"new-synthetic"}}`, `{"agent":{"personality_v2_api_key":"new-synthetic"}}`} {
		rec := httptest.NewRecorder()
		handleUpdateConfig(s)(rec, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(body)))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("legacy secret patch returned %d", rec.Code)
		}
		var patch map[string]interface{}
		_ = json.Unmarshal([]byte(body), &patch)
		if _, err := applyConfigPatch(s, patch); err == nil {
			t.Fatal("setup patch accepted a legacy credential")
		}
	}
	if after, err := os.ReadFile(s.Cfg.ConfigPath); err != nil || string(after) != string(legacyYAML) {
		t.Fatal("rejected patch changed saved config")
	}
	for _, key := range []string{"v2_api_key", "personality_v2_api_key"} {
		value := map[string]interface{}{"section": map[string]interface{}{key: "legacy-credential"}}
		maskSensitiveFields(value)
		if value["section"].(map[string]interface{})[key] == "legacy-credential" {
			t.Fatal("GET disclosed legacy credential")
		}
	}
}
