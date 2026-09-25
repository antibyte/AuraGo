package server

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestNewspaperAgentMailChallengeOutcome(t *testing.T) {
	for _, tc := range []struct {
		name       string
		provider   int
		response   string
		firstCode  string
		secondCode string
		calls      int
	}{
		{"sender unavailable", 0, "", "email_send_safe", "email_send_safe", 0},
		{"rejected", http.StatusForbidden, `{"error":"provider details must stay private"}`, "agentmail_rejected", "agentmail_rejected", 2},
		{"bounce blocked", http.StatusForbidden, `{"code":"message_rejected","message":"Message rejected: Recipient(s) blocked: reader@example.org (bounced)","fix":"provider details must stay private"}`, "agentmail_bounce_blocked", "agentmail_bounce_blocked", 2},
		{"uncertain", http.StatusServiceUnavailable, `{"error":"provider details must stay private"}`, "email_send_uncertain", "challenge_pending", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodPost || r.URL.Path != "/v0/inboxes/test-inbox/messages/send" {
					t.Errorf("unexpected AgentMail request: %s %s", r.Method, r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.provider)
				fmt.Fprint(w, tc.response)
			}))
			defer provider.Close()
			s, _, _ := testDesktopPermissionServer(t)
			s.Cfg.VirtualDesktop.Enabled = true
			s.Cfg.Newspaper.Enabled = true
			s.Cfg.Newspaper.AllowEmail = true
			s.Cfg.AgentMail.Enabled = true
			if tc.provider != 0 {
				s.Cfg.AgentMail.APIKey = "test-key"
			}
			s.Cfg.AgentMail.InboxID = "test-inbox"
			s.Cfg.AgentMail.BaseURL = provider.URL
			service, err := newspaper.New(newspaper.Options{
				Path:   filepath.Join(t.TempDir(), "newspaper.db"),
				Policy: func() newspaper.Policy { return newspaper.Policy{Enabled: true} },
				Research: func(context.Context, newspaper.Profile, time.Time, func(newspaper.Progress)) (newspaper.Draft, error) {
					return newspaper.Draft{}, nil
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			defer service.Close()
			s.Newspaper = service
			profile, err := service.Profile(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			profile.EmailTo = "reader@example.org"
			profile.EmailAccountID = "agentmail"
			if _, err := service.SaveProfile(context.Background(), profile); err != nil {
				t.Fatal(err)
			}
			adminToken, _, err := s.TokenManager.Create("newspaper admin", []string{desktopScopeAdmin}, nil)
			if err != nil {
				t.Fatal(err)
			}
			request := func() (int, map[string]any, string) {
				r := httptest.NewRequest(http.MethodPost, "/api/desktop/newspaper/email/challenge", strings.NewReader(`{}`))
				r.Header.Set("Authorization", "Bearer "+adminToken)
				w := httptest.NewRecorder()
				s.handleNewspaper(w, r)
				var body map[string]any
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if strings.Contains(w.Body.String(), "provider details") || strings.Contains(w.Body.String(), "test-key") || strings.Contains(w.Body.String(), "reader@example.org") {
					t.Fatal("provider error or key leaked to the client")
				}
				return w.Code, body, w.Header().Get("Retry-After")
			}
			firstStatus, first, _ := request()
			secondStatus, second, retryAfter := request()
			if firstStatus != http.StatusBadGateway || first["code"] != tc.firstCode || second["code"] != tc.secondCode || calls != tc.calls {
				t.Fatalf("outcomes: first=%d %#v, second=%d %#v, provider calls=%d", firstStatus, first, secondStatus, second, calls)
			}
			if tc.provider == http.StatusForbidden && (secondStatus != http.StatusBadGateway || first["provider_status"] != float64(http.StatusForbidden)) {
				t.Fatalf("rejection did not expose safe HTTP status: %#v, %d", first, secondStatus)
			}
			if tc.provider == http.StatusServiceUnavailable && (secondStatus != http.StatusConflict || retryAfter != "60") {
				t.Fatalf("uncertain send lost the cooldown: %#v, %d, retry after %q", second, secondStatus, retryAfter)
			}
		})
	}
}
