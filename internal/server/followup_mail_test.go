package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/security"
)

func TestMailIngressDoesNotConsumeDefaultFollowUpChain(t *testing.T) {
	s := newOperatorCommandTestServer(t)
	handler := handleChatCompletions(s, nil)
	muFollowUp.Lock()
	previousDefault, previousMail := followUpDepths["default"], followUpDepths["external-email"]
	delete(followUpDepths, "default")
	delete(followUpDepths, "external-email")
	muFollowUp.Unlock()
	t.Cleanup(func() {
		muFollowUp.Lock()
		defer muFollowUp.Unlock()
		delete(followUpDepths, "default")
		delete(followUpDepths, "external-email")
		if previousDefault > 0 {
			followUpDepths["default"] = previousDefault
		}
		if previousMail > 0 {
			followUpDepths["external-email"] = previousMail
		}
	})
	post := func(mail bool, token string) int {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"messages":[{"role":"user","content":"/help"}]}`))
		req.RemoteAddr = "127.0.0.1:43210"
		req.Header.Set("X-Internal-FollowUp", "true")
		req.Header.Set("X-Internal-Token", token)
		if mail {
			req.Header.Set("X-External-Source", string(security.ExternalEmail))
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}
	for range 10 {
		if got := post(false, s.internalToken); got != http.StatusOK {
			t.Fatalf("initial follow-up status = %d", got)
		}
	}
	for range 15 {
		if got := post(true, s.internalToken); got != http.StatusOK {
			t.Fatalf("independent mail status = %d, want 200 even with a full default chain", got)
		}
	}
	for range 2 {
		if got := post(false, s.internalToken); got != http.StatusTooManyRequests {
			t.Fatalf("default chain status = %d, want 429", got)
		}
	}
	if got := post(true, "wrong-token"); got == http.StatusOK {
		t.Fatal("mail provenance must still require the process token")
	}
	muFollowUp.Lock()
	defer muFollowUp.Unlock()
	if followUpDepths["default"] != 10 || followUpDepths["external-email"] != 0 {
		t.Fatalf("follow-up slots leaked: %v", followUpDepths)
	}
}
