package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/testutil"
)

func TestHandlePaperlessTestUsesRequestContext(t *testing.T) {
	started := make(chan struct{})
	upstream := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Token paperless-token" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		close(started)
		<-r.Context().Done()
	}))
	defer upstream.Close()

	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := vault.WriteSecret("paperless_ngx_api_token", "paperless-token"); err != nil {
		t.Fatal(err)
	}
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default(), Vault: vault}
	s.Cfg.PaperlessNGX.Enabled = true
	s.Cfg.PaperlessNGX.URL = upstream.URL

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/api/paperless/test", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handlePaperlessTest(s).ServeHTTP(rec, req)
		close(done)
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("paperless test did not reach upstream")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled request did not stop the paperless test")
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v body=%s", err, rec.Body.String())
	}
	if body["status"] != "error" {
		t.Fatalf("status = %#v, body=%s", body["status"], rec.Body.String())
	}
}
