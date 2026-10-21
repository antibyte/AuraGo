package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/sipphone"
	"aurago/internal/voice"

	"github.com/pion/webrtc/v4"
)

func TestClassicTelephoneLiveTestRejectsInvalidSourceRates(t *testing.T) {
	for _, rate := range []int{3999, 192001} {
		recognizer := &sipLiveTestRecognizer{}
		backend := &voice.ClassicBackend{Synthesizer: &sipLiveTestSynthesizer{sampleRate: rate}, Recognizer: recognizer}
		if err := runClassicSIPAgentLiveTest(context.Background(), &sipLiveTestClient{}, backend, "phone-model", "en"); err == nil || recognizer.calls != 0 {
			t.Fatalf("invalid source rate %d reached ASR", rate)
		}
	}
}

func TestSIPAgentLiveTestRequiresConfirmationAndHonorsQuota(t *testing.T) {
	s := &Server{Cfg: telephoneAgentTestConfig(t)}
	limiter := &sipRequestLimiter{windows: make(map[string]*sipRequestWindow)}
	request := func(body string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "https://aurago.local/api/sip/agent/test", strings.NewReader(body))
		r.Header.Set("Origin", "https://aurago.local")
		return r
	}
	recorder := httptest.NewRecorder()
	handleSIPAgentTest(s, limiter).ServeHTTP(recorder, request(`{"live":true}`))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unconfirmed live test status = %d", recorder.Code)
	}
	for range 3 {
		if !limiter.allow(request(`{}`), 3, time.Hour) {
			t.Fatal("confirmation failure consumed the live-test quota")
		}
	}
	recorder = httptest.NewRecorder()
	handleSIPAgentTest(s, limiter).ServeHTTP(recorder, request(`{"live":true,"confirm_live":true}`))
	if recorder.Code != http.StatusTooManyRequests || recorder.Header().Get("Retry-After") == "" {
		t.Fatalf("exhausted quota status = %d", recorder.Code)
	}
}

func TestSIPAgentEmptyTargetsBlockCallsButNotPipelineChecks(t *testing.T) {
	cfg := telephoneAgentTestConfig(t)
	cfg.SIP.Permissions.OriginateOutbound = true
	s := &Server{Cfg: cfg}
	if !slices.Contains(sipAgentBlockers(s, cfg.SIP, cfg.SIP.Voice), "outbound_destinations_empty") {
		t.Fatal("empty targets did not block outbound readiness")
	}
	recorder := httptest.NewRecorder()
	handleSIPAgentTest(s, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/sip/agent/test", strings.NewReader(`{}`)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("target policy blocked the pipeline check: %d", recorder.Code)
	}
	cfg.SIP.Outbound.AllowedUsers = []string{"101"}
	if slices.Contains(sipAgentBlockers(s, cfg.SIP, cfg.SIP.Voice), "outbound_destinations_empty") {
		t.Fatal("explicit target did not clear the blocker")
	}
}

func TestSIPAgentPUTKeepsUnusedProviderInheritanceHonest(t *testing.T) {
	s, vault := newProviderTestServer(t, `
providers:
  - id: phone-agent
    type: ollama
    base_url: http://127.0.0.1:11434/v1
    model: phone-model
llm:
  provider: phone-agent
whisper:
  provider: ""
realtime_speech:
  profiles:
    - id: phone-live
      provider: gemini
      model: gemini-live-test
      enabled: true
`)
	if err := vault.WriteSecret(config.RealtimeSpeechProfileAPIKeyVaultKey("phone-live"), "test-live-profile-key"); err != nil {
		t.Fatal(err)
	}
	s.Cfg.ApplyVaultSecrets(vault)
	s.Cfg.Directories.SkillsDir = t.TempDir()
	s.Cfg.Directories.ToolsDir = t.TempDir()
	voiceCfg := s.Cfg.SIP.Voice
	voiceCfg.Backend = "gemini_live"
	voiceCfg.RealtimeProfileID = "phone-live"
	body, err := json.Marshal(sipAgentPayload{InboundRoute: "reject", Voice: voiceCfg})
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{http.MethodPut, http.MethodGet} {
		recorder := httptest.NewRecorder()
		handleSIPAgent(s).ServeHTTP(recorder, httptest.NewRequest(method, "/api/sip/agent", bytes.NewReader(body)))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status %d: %s", method, recorder.Code, recorder.Body.String())
		}
		var response struct {
			Inherited map[string]bool `json:"inherited"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if !response.Inherited["asr_provider_id"] || response.Inherited["agent_provider_id"] {
			t.Fatalf("%s reported false materialization: %+v", method, response.Inherited)
		}
	}
}

func TestSIPBrowserPermissionDenialKeepsOfferedSession(t *testing.T) {
	for _, action := range []string{"dial", "answer"} {
		for _, readonly := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/readonly=%t", action, readonly), func(t *testing.T) {
				s := newSIPLifecycleTestServer(t)
				s.Cfg.SIP.ReadOnly = readonly
				// Exercise read-only and granular permission denial independently.
				s.Cfg.SIP.Permissions.AnswerInbound = readonly
				s.Cfg.SIP.Permissions.OriginateOutbound = readonly
				s.Cfg.SIP.BrowserMedia.Enabled = true
				s.Cfg.SIP.BrowserMedia.BindHost = "127.0.0.1"
				s.Cfg.SIP.BrowserMedia.UDPPort = 0
				manager, err := sipphone.NewManager(s.Cfg.SIP, t.TempDir(), nil, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer manager.Close()
				s.SIPPhone = manager
				service, err := sipphone.NewBrowserMediaService(s.Cfg.SIP, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer service.Close()
				s.SIPBrowserMedia = service
				client, err := webrtc.NewPeerConnection(webrtc.Configuration{})
				if err != nil {
					t.Fatal(err)
				}
				defer client.Close()
				if _, err := client.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
					t.Fatal(err)
				}
				offer, err := client.CreateOffer(nil)
				if err != nil {
					t.Fatal(err)
				}
				request := httptest.NewRequest(http.MethodPost, "https://aurago.local/api/sip/calls", nil)
				request.Header.Set("Origin", "https://aurago.local")
				request.Header.Set(sipBrowserClientIDHeader, "client-permission-test")
				owner, clientID, err := sipBrowserRequestIdentity(s, request)
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				session, err := s.SIPBrowserMedia.CreateSession(ctx, owner, clientID, offer.SDP)
				if err != nil {
					t.Fatal(err)
				}
				body := `{"target":"sip:101@192.0.2.10","media_mode":"browser","browser_session_id":"` + session.ID + `"}`
				if action == "answer" {
					body = `{"browser_session_id":"` + session.ID + `"}`
					request.URL.Path = "/api/sip/calls/not-yet-answered/answer"
				}
				request.Body = io.NopCloser(strings.NewReader(body))
				request.ContentLength = int64(len(body))
				recorder := httptest.NewRecorder()
				if action == "dial" {
					handleSIPCalls(s).ServeHTTP(recorder, request)
				} else {
					handleSIPCallAction(s).ServeHTTP(recorder, request)
				}
				if recorder.Code != http.StatusForbidden {
					t.Fatalf("permission denial status %d: %s", recorder.Code, recorder.Body.String())
				}
				peer, err := s.SIPBrowserMedia.ClaimSession(owner, clientID, session.ID)
				if err != nil {
					t.Fatalf("denial consumed the offered session: %v", err)
				}
				peer.Detach("")
			})
		}
	}
}
