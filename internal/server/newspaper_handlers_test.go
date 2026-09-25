package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/newspaper"
)

func TestNewspaperHTTPAuthOriginAndReadOnly(t *testing.T) {
	s, readToken, _ := testDesktopPermissionServer(t)
	s.Cfg.VirtualDesktop.Enabled = true
	s.Cfg.Newspaper.Enabled = true
	service, err := newspaper.New(newspaper.Options{
		Path:   filepath.Join(t.TempDir(), "newspaper.db"),
		Policy: func() newspaper.Policy { return newspaper.Policy{Enabled: true, ReadOnly: s.Cfg.Newspaper.ReadOnly} },
		Research: func(context.Context, newspaper.Profile, time.Time, func(newspaper.Progress)) (newspaper.Draft, error) {
			return newspaper.Draft{}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	s.Newspaper = service
	adminToken, _, err := s.TokenManager.Create("newspaper admin", []string{desktopScopeAdmin}, nil)
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path, token, origin string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(`{}`))
		if token == "session" {
			r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
		} else if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		s.handleNewspaper(w, r)
		return w
	}
	if w := request(http.MethodGet, "/api/desktop/newspaper/profile", "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous read: %d", w.Code)
	}
	if w := request(http.MethodGet, "/api/desktop/newspaper/profile", readToken, ""); w.Code != http.StatusForbidden {
		t.Fatalf("read-scope access: %d", w.Code)
	}
	if w := request(http.MethodGet, "/api/desktop/newspaper/profile", adminToken, ""); w.Code != http.StatusOK {
		t.Fatalf("admin read: %d %s", w.Code, w.Body.String())
	}
	if w := request(http.MethodPost, "/api/desktop/newspaper/editions", "session", ""); w.Code != http.StatusForbidden {
		t.Fatalf("missing origin: %d", w.Code)
	}
	if w := request(http.MethodPost, "/api/desktop/newspaper/editions", "session", "http://foreign.example"); w.Code != http.StatusForbidden {
		t.Fatalf("foreign origin: %d", w.Code)
	}
	s.Cfg.Newspaper.ReadOnly = true
	if w := request(http.MethodPost, "/api/desktop/newspaper/editions", adminToken, "http://example.com"); w.Code != http.StatusForbidden {
		t.Fatalf("read-only research: %d %s", w.Code, w.Body.String())
	}
	if w := request(http.MethodPost, "/api/desktop/newspaper/email/challenge", adminToken, "http://example.com"); w.Code != http.StatusForbidden {
		t.Fatalf("read-only email challenge: %d", w.Code)
	}
}

func TestNewspaperTelegramChunksKeepCompleteText(t *testing.T) {
	text := "A short article.\nhttps://example.org/source?id=42\nAnother story.\n"
	chunks, err := newspaperTelegramChunks(text, 35)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(chunks, "") != text {
		t.Fatalf("text was changed: %#v", chunks)
	}
	if _, err = newspaperTelegramChunks(strings.Repeat("x", 36), 35); err == nil {
		t.Fatal("overlong line was silently truncated")
	}
}
