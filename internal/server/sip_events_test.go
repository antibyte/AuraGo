package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/sipphone"
)

type sipEventsTestRecorder struct {
	*httptest.ResponseRecorder
	cancel context.CancelFunc
}

func (w *sipEventsTestRecorder) Flush() {
	w.ResponseRecorder.Flush()
	w.cancel()
}

func TestSIPEventsScrubSecretsAcrossRoutes(t *testing.T) {
	for _, route := range []string{"/api/sip/events", "/api/desktop/integrations/sip/events"} {
		for _, tc := range []struct {
			name     string
			bindHost string
			secret   bool
			want     string
		}{
			{"escaped_secret", "sip-event-sensitive<&>\"", true, "[redacted]:5060"},
			{"benign_snapshot", "127.0.0.1", false, "127.0.0.1:5060"},
		} {
			t.Run(route+"/"+tc.name, func(t *testing.T) {
				if tc.secret {
					t.Cleanup(security.RegisterScopedSensitiveExact(tc.bindHost))
				}
				var cfg config.SIPConfig
				config.ApplySIPDefaults(&cfg)
				// Keep the endpoint stopped while exercising JSON escapes in its snapshot.
				cfg.BindHost = tc.bindHost
				manager, err := sipphone.NewManager(cfg, t.TempDir(), nil, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = manager.Close() })
				s, _, _ := testDesktopPermissionServer(t)
				s.Cfg.SIP = cfg
				s.SIPPhone = manager
				token, _, err := s.TokenManager.Create("sip events", []string{"admin", desktopScopeAdmin}, nil)
				if err != nil {
					t.Fatal(err)
				}
				mux := http.NewServeMux()
				registerSIPHandlers(mux, s)
				mux.HandleFunc("/api/desktop/integrations/", desktopIntegrationHandler(s, mux))
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				request := httptest.NewRequest(http.MethodGet, "https://aurago.local"+route, nil).WithContext(ctx)
				request.Header.Set("Authorization", "Bearer "+token)
				recorder := &sipEventsTestRecorder{httptest.NewRecorder(), cancel}
				mux.ServeHTTP(recorder, request)
				if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "text/event-stream; charset=utf-8" {
					t.Fatalf("unexpected SSE response: status=%d content-type=%q", recorder.Code, recorder.Header().Get("Content-Type"))
				}
				frame := strings.TrimSuffix(recorder.Body.String(), "\n\n")
				lines := strings.Split(frame, "\n")
				if len(lines) != 3 || lines[1] != "event: sip" || !strings.HasPrefix(lines[2], "data: ") {
					t.Fatalf("invalid SIP SSE frame: %q", frame)
				}
				data := strings.TrimPrefix(lines[2], "data: ")
				var event sipphone.Event
				if err := json.Unmarshal([]byte(data), &event); err != nil {
					t.Fatalf("invalid SIP event JSON: %v", err)
				}
				if event.Sequence != 1 || lines[0] != fmt.Sprintf("id: %d", event.Sequence) || event.Type != "snapshot" || event.Timestamp.IsZero() {
					t.Fatalf("SIP event envelope changed: id=%q sequence=%d type=%q timestamp=%v", lines[0], event.Sequence, event.Type, event.Timestamp)
				}
				if event.Status == nil || event.Status.BindAddress != tc.want {
					t.Fatalf("unexpected SIP snapshot status: %+v", event.Status)
				}
				if !tc.secret {
					original, err := json.Marshal(event)
					if err != nil {
						t.Fatal(err)
					}
					if data != string(original) {
						t.Fatal("SIP event without secrets changed serialization")
					}
				}
			})
		}
	}
}
