package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"
)

func TestMailReceiptsAdminAPI(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{}
	cfg.Directories.DataDir = dir
	cfg.WebConfig.Enabled = true
	cfg.Auth.Enabled, cfg.Auth.SessionSecret, cfg.Auth.PasswordHash = true, "receipt-session-secret", "fixture-hash"
	cfg.Discord.Enabled, cfg.Discord.DefaultChannelID = true, "private-recipient"
	vault := newTestVault(t, filepath.Join(dir, "vault.bin"), "03")
	tokens, err := security.NewTokenManager(vault, filepath.Join(dir, "tokens.json"))
	if err != nil {
		t.Fatal(err)
	}
	readToken, _, err := tokens.Create("reader", []string{"read"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	adminToken, _, err := tokens.Create("admin", []string{"admin"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Cfg: cfg, Logger: slog.Default(), TokenManager: tokens}
	mux := http.NewServeMux()
	s.registerConfigAPIRoutes(mux, nil)
	handler := authMiddleware(s, mux)
	request := func(method, path, payload, auth, origin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://example.com"+path, strings.NewReader(payload))
		if auth == "session" {
			r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
		} else if auth != "" {
			r.Header.Set("Authorization", "Bearer "+auth)
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if !isAdminProtectedPath(mailReceiptsRoute) || !isAdminProtectedPath(mailReceiptsRoute+"/id/resolve") {
		t.Fatal("route missing central admin classification")
	}
	if w := request("GET", mailReceiptsRoute, "", "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated = %d %s", w.Code, w.Body.String())
	}
	if w := request("GET", mailReceiptsRoute, "", readToken, ""); w.Code != http.StatusForbidden {
		t.Fatalf("read token = %d", w.Code)
	}
	ctx := tools.WithMailNotificationPolicy(context.Background(), dir, "private-event-identity", true)
	tools.SendNotificationContext(ctx, cfg, slog.Default(), "discord", "private-mail-subject", "private-mail-content", "high", func(string, string) error { return errors.New("synthetic uncertain outcome") })
	w := request("GET", mailReceiptsRoute, "", adminToken, "")
	var page tools.MailReceiptPage
	if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-store" || json.Unmarshal(w.Body.Bytes(), &page) != nil || len(page.Receipts) != 1 {
		t.Fatalf("metadata = %d %s", w.Code, w.Body.String())
	}
	for _, content := range []string{"private-mail-subject", "private-mail-content", "private-recipient", "private-event-identity", "synthetic uncertain outcome"} {
		if strings.Contains(w.Body.String(), content) {
			t.Fatal("content leaked from metadata API")
		}
	}
	receipt := page.Receipts[0]
	path := mailReceiptsRoute + "/" + receipt.ID + "/resolve"
	body := `{"revision":"` + receipt.Revision + `","resolution":"sent"}`
	for _, origin := range []string{"", "https://foreign.example"} {
		if w := request("POST", path, body, "session", origin); w.Code != http.StatusForbidden {
			t.Fatalf("browser origin %q = %d", origin, w.Code)
		}
	}
	if w := request("POST", path, body, "session", "http://example.com"); w.Code != http.StatusNoContent {
		t.Fatalf("resolve = %d %s", w.Code, w.Body.String())
	}
	if w := request("POST", path, body, adminToken, ""); w.Code != http.StatusConflict {
		t.Fatalf("stale revision = %d", w.Code)
	}
	if w := request("GET", mailReceiptsRoute+"?limit=51", "", adminToken, ""); w.Code != http.StatusBadRequest {
		t.Fatal("unbounded page accepted")
	}
	if w := request("GET", mailReceiptsRoute+"?channel=telegram", "", adminToken, ""); w.Code != http.StatusOK || strings.Contains(w.Body.String(), receipt.ID) {
		t.Fatal("channel filter ignored")
	}
	for _, invalid := range []string{`{"revision":"` + receipt.Revision + `","resolution":"retry"}`, body + ` {}`, `{"revision":"` + receipt.Revision + `","resolution":"sent","message":"forbidden"}`} {
		if w := request("POST", path, invalid, adminToken, ""); w.Code != http.StatusBadRequest {
			t.Fatalf("invalid resolution = %d", w.Code)
		}
	}
	activeCtx := tools.WithMailNotificationPolicy(context.Background(), dir, "active-event", true)
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		tools.SendNotificationContext(activeCtx, cfg, slog.Default(), "discord", "fixture", "fixture", "high", func(string, string) error { close(entered); <-release; return nil })
	}()
	defer func() { close(release); <-done }()
	<-entered
	w = request("GET", mailReceiptsRoute, "", adminToken, "")
	if json.Unmarshal(w.Body.Bytes(), &page) != nil {
		t.Fatal("invalid active metadata")
	}
	for _, receipt := range page.Receipts {
		if receipt.Status == "active" {
			path := mailReceiptsRoute + "/" + receipt.ID + "/resolve"
			body := `{"revision":"` + receipt.Revision + `","resolution":"not_sent"}`
			if w := request("POST", path, body, adminToken, ""); w.Code != http.StatusConflict {
				t.Fatal("active delivery resolved")
			}
			return
		}
	}
	t.Fatal("active attempt missing")
}
