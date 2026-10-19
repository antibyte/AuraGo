package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"
)

func TestGo2RTCTalkPermissionsAndLifecycle(t *testing.T) {
	cfg := &config.Config{}
	cfg.Auth.Enabled = true
	cfg.Auth.PasswordHash = "configured"
	cfg.Auth.SessionSecret = "fixture"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"version":"1.9.14-aurago1","aurago_webrtc_lifecycle":"ws-close-v1"}`))
	}))
	defer upstream.Close()
	u, _ := url.Parse(upstream.URL)
	port, _ := strconv.Atoi(u.Port())
	cfg.Go2RTC = config.Go2RTCConfig{Enabled: true, URL: upstream.URL, APIHostPort: port, APIPassword: "fixture", WebRTC: config.Go2RTCWebRTCConfig{Enabled: true}, Streams: []config.Go2RTCStreamConfig{{ID: "door", Enabled: true, TalkEnabled: true, Source: "rtsp://192.168.1.2/live"}}}
	vault, err := security.NewVault(strings.Repeat("55", 32), t.TempDir()+"/vault.bin")
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := security.NewTokenManager(vault, t.TempDir()+"/tokens.bin")
	if err != nil {
		t.Fatal(err)
	}
	viewer, _, _ := tokens.Create("viewer", []string{go2RTCViewScope}, nil)
	talker, _, _ := tokens.Create("talker", []string{go2RTCTalkScope}, nil)
	other, _, _ := tokens.Create("other", []string{go2RTCTalkScope}, nil)
	s := &Server{Cfg: cfg, TokenManager: tokens, Go2RTC: tools.NewGo2RTCManager(cfg, nil, nil, nil)}
	t.Cleanup(s.Go2RTC.Close)
	mux := http.NewServeMux()
	registerGo2RTCRoutes(mux, s)
	handler := authMiddleware(s, mux)
	request := func(method, path, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(`{}`))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if got := request("POST", "/api/go2rtc/talk/door", viewer).Code; got != 403 {
		t.Fatalf("viewer created talk session: %d", got)
	}
	response := request("POST", "/api/go2rtc/talk/door", talker)
	if response.Code != 200 {
		t.Fatalf("talk create: %d %s", response.Code, response.Body.String())
	}
	var created struct {
		Session string `json:"session"`
	}
	if json.Unmarshal(response.Body.Bytes(), &created) != nil || created.Session == "" {
		t.Fatal("missing talk session")
	}
	if got := request("POST", "/api/go2rtc/talk/door", talker).Code; got != 409 {
		t.Fatalf("parallel talk admitted: %d", got)
	}
	if got := request("DELETE", "/api/go2rtc/talk/door?session="+created.Session, other).Code; got != 403 {
		t.Fatalf("foreign identity accepted: %d", got)
	}
	s.go2RTCTalkMu.Lock()
	session := s.go2RTCTalks["door"]
	s.go2RTCTalkMu.Unlock()
	if got := request("DELETE", "/api/go2rtc/talk/door?session="+created.Session, talker).Code; got != 200 {
		t.Fatalf("talk release: %d", got)
	}
	select {
	case <-session.lease.Context.Done():
	case <-time.After(time.Second):
		t.Fatal("released microphone session still live")
	}
	response = request("POST", "/api/go2rtc/talk/door", talker)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	s.go2RTCTalkMu.Lock()
	session = s.go2RTCTalks["door"]
	session.timer.Reset(time.Millisecond)
	s.go2RTCTalkMu.Unlock()
	select {
	case <-session.lease.Context.Done():
	case <-time.After(time.Second):
		t.Fatal("expired heartbeat session still live")
	}
	if _, err := s.Go2RTC.AcquireMedia(context.Background(), "unknown", "main", false); err == nil {
		t.Fatal("unknown camera accepted")
	}
}
