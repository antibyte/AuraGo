package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/security"
	"aurago/internal/webhooks"
)

func TestWebhookReplayAPIValidatesEffectiveFormatAndPreservesOmission(t *testing.T) {
	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := webhooks.NewManager(filepath.Join(t.TempDir(), "webhooks.json"), filepath.Join(t.TempDir(), "webhooks.log"))
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Vault: vault}
	for _, format := range []string{`{"replay_protection":true}`, `{"replay_protection":true,"signature_header":"X-Sig","signature_algo":"sha1","signature_secret":"fixture"}`} {
		w := httptest.NewRecorder()
		handleCreateWebhook(s, m).ServeHTTP(w, httptest.NewRequest("POST", "/api/webhooks", strings.NewReader(`{"slug":"bad-hook","format":`+format+`}`)))
		if w.Code != 400 {
			t.Fatalf("invalid format: %d %s", w.Code, w.Body)
		}
	}
	w := httptest.NewRecorder()
	handleCreateWebhook(s, m).ServeHTTP(w, httptest.NewRequest("POST", "/api/webhooks", strings.NewReader(`{"name":"Hook","slug":"good-hook","format":{"replay_protection":true,"signature_header":"X-Sig","signature_algo":"sha256","signature_secret":"fixture"}}`)))
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	var created webhooks.Webhook
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		body    string
		status  int
		enabled bool
	}{
		{`{"name":"renamed","format":{"description":"keep protection"}}`, 200, true},
		{`{"format":{"signature_algo":"sha1"}}`, 400, true},
		{`{"format":{"signature_secret":""}}`, 400, true},
		{`{"format":{"replay_protection":false,"signature_algo":"sha1"}}`, 200, false},
	} {
		w = httptest.NewRecorder()
		handleUpdateWebhook(s, m).ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/api/webhooks/"+created.ID, strings.NewReader(tt.body)))
		if w.Code != tt.status {
			t.Fatalf("patch %s: %d %s", tt.body, w.Code, w.Body)
		}
		got, err := m.Get(created.ID)
		if err != nil || got.Format.ReplayProtection != tt.enabled {
			t.Fatalf("saved protection: %+v %v", got, err)
		}
	}
}
