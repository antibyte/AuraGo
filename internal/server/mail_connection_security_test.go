package server

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"aurago/internal/agentmail"
	"aurago/internal/config"
	"aurago/internal/mailer"
)

func TestMailConnectionErrorsDoNotExposeProviderData(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code string
	}{
		{&agentmail.APIError{StatusCode: 401, Message: "PRIVATE_CREDENTIAL", Body: "PRIVATE_BODY"}, "auth_failed"},
		{&agentmail.APIError{StatusCode: 429, Body: "PRIVATE_BODY"}, "rate_limited"},
		{&agentmail.APIError{StatusCode: 503, Body: "PRIVATE_BODY"}, "provider_rejected"},
		{fmt.Errorf("PRIVATE_HOST: %w", context.DeadlineExceeded), "timeout"},
		{x509.UnknownAuthorityError{}, "certificate_invalid"},
		{&textproto.Error{Code: 535, Msg: "PRIVATE_CREDENTIAL"}, "auth_failed"},
		{mailer.ErrMailTarget, "target_blocked"},
		{errors.New("PRIVATE_BODY"), "connection_failed"},
	} {
		code := mailConnectionErrorCode(tc.err)
		if code != tc.code {
			t.Fatalf("code=%s, want %s", code, tc.code)
		}
		w := httptest.NewRecorder()
		writeMailConnectionError(w, code)
		if strings.Contains(w.Body.String(), "PRIVATE") || w.Code != http.StatusBadGateway {
			t.Fatal("provider data reached public response")
		}
	}
}

func TestAgentMailStatusConcurrentReload(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	s.ConfigSnapshot()
	service := agentmail.NewService(agentmail.ServiceConfig{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 500; i++ {
			cfg := &config.Config{}
			cfg.AgentMail.Enabled = i%2 == 0
			s.CfgMu.Lock()
			s.Cfg = cfg
			s.cfgSnapshot.Store(cfg)
			s.CfgMu.Unlock()
			s.AgentMailMu.Lock()
			if i%2 == 0 {
				s.AgentMailService = service
			} else {
				s.AgentMailService = nil
			}
			s.AgentMailMu.Unlock()
		}
	}()
	defer func() { <-done }()
	handler := handleAgentMailStatus(s)
	for i := 0; i < 500; i++ {
		w := httptest.NewRecorder()
		handler(w, httptest.NewRequest(http.MethodGet, "/api/agentmail/status", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status during reload: %d", w.Code)
		}
	}
}

func TestAgentMailDeleteCapabilityRequiresSeparateGrant(t *testing.T) {
	for _, readonly := range []bool{false, true} {
		for _, allowDelete := range []bool{false, true} {
			cfg := &config.Config{}
			cfg.AgentMail = config.AgentMailConfig{Enabled: true, InboxID: "fixture", ReadOnly: readonly, AllowDelete: allowDelete}
			s := &Server{Cfg: cfg}
			accounts, err := s.mailerSourceAccounts(context.Background())
			if err != nil || len(accounts) != 1 {
				t.Fatalf("source enumeration: %v", err)
			}
			has := func(want string) bool {
				for _, capability := range accounts[0].Capabilities {
					if capability == want {
						return true
					}
				}
				return false
			}
			if has("send") != !readonly || has("delete") != (allowDelete && !readonly) {
				t.Fatalf("unexpected grants: readonly=%t delete=%t caps=%v", readonly, allowDelete, accounts[0].Capabilities)
			}
		}
	}
}

func TestMailConnectionProbesBoundConcurrencyAndRate(t *testing.T) {
	var gate mailProbeGate
	now := time.Now()
	first, ok := gate.begin(now)
	if !ok {
		t.Fatal("first probe refused")
	}
	if _, ok := gate.begin(now); ok {
		t.Fatal("cooldown was not enforced")
	}
	second, ok := gate.begin(now.Add(time.Second))
	if !ok {
		t.Fatal("second concurrent probe refused after cooldown")
	}
	if _, ok := gate.begin(now.Add(2 * time.Second)); ok {
		t.Fatal("third concurrent probe was admitted")
	}
	first()
	third, ok := gate.begin(now.Add(2 * time.Second))
	if !ok {
		t.Fatal("released probe slot was not reusable")
	}
	second()
	third()
}
