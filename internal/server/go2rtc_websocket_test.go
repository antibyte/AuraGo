package server

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/jpeg"
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
	"github.com/gorilla/websocket"
)

func cameraTestJPEG() []byte {
	var out bytes.Buffer
	_ = jpeg.Encode(&out, image.NewRGBA(image.Rect(0, 0, 8, 4)), nil)
	return out.Bytes()
}

func TestGo2RTCFullRouteBlocksSendingViewer(t *testing.T) {
	forwarded := make(chan bool, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/go2rtc/proxy/api" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"version":"1.9.14-aurago1","aurago_webrtc_lifecycle":"ws-close-v1"}`))
			return
		}
		ws, err := (&websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer ws.Close()
		ws.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, _, err = ws.ReadMessage()
		forwarded <- err == nil
	}))
	defer upstream.Close()
	u, _ := url.Parse(upstream.URL)
	port, _ := strconv.Atoi(u.Port())
	cfg := &config.Config{}
	cfg.Auth.Enabled = true
	cfg.Auth.PasswordHash = "configured"
	cfg.Auth.SessionSecret = "fixture"
	cfg.Go2RTC = config.Go2RTCConfig{Enabled: true, WebRTC: config.Go2RTCWebRTCConfig{Enabled: true}, URL: upstream.URL, APIHostPort: port, APIPassword: "fixture-password", Streams: []config.Go2RTCStreamConfig{{ID: "door", Enabled: true, Source: "rtsp://192.168.1.2/live"}}}
	vault, err := security.NewVault(strings.Repeat("44", 32), t.TempDir()+"/vault.bin")
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := security.NewTokenManager(vault, t.TempDir()+"/tokens.bin")
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := tokens.Create("viewer", []string{go2RTCViewScope}, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Cfg: cfg, TokenManager: tokens, Go2RTC: tools.NewGo2RTCManager(cfg, nil, nil, nil)}
	if _, err := s.Go2RTC.Test(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Go2RTC.Close)
	mux := http.NewServeMux()
	registerGo2RTCRoutes(mux, s)
	server := httptest.NewServer(authMiddleware(s, mux))
	defer server.Close()
	headers := http.Header{"Authorization": {"Bearer " + token}}
	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/api/go2rtc/proxy/api/ws?src=door", headers)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	ws.SetReadDeadline(time.Now().Add(3 * time.Second))
	if err := ws.WriteJSON(map[string]any{"type": "webrtc/offer", "value": cameraOffer("audio", "sendonly")}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ws.ReadMessage(); err == nil {
		t.Fatal("sending viewer was not disconnected")
	}
	if <-forwarded {
		t.Fatal("sending SDP reached upstream")
	}
	req := httptest.NewRequest("POST", server.URL+"/api/go2rtc/proxy/api/webrtc?src=door", strings.NewReader(cameraOffer("video", "sendonly")))
	req.Header = headers
	rr := httptest.NewRecorder()
	authMiddleware(s, mux).ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("viewer POST = %d", rr.Code)
	}
}

func cameraOffer(kind, direction string) string {
	return "v=0\r\no=- 1 1 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\nm=" + kind + " 9 UDP/TLS/RTP/SAVPF 0\r\nc=IN IP4 0.0.0.0\r\na=" + direction + "\r\n"
}

func TestGo2RTCSignalDirections(t *testing.T) {
	for _, tc := range []struct {
		kind, direction string
		talk, allowed   bool
	}{{"video", "recvonly", false, true}, {"audio", "recvonly", false, true}, {"audio", "sendonly", false, false}, {"video", "sendrecv", false, false}, {"audio", "sendonly", true, true}, {"video", "sendonly", true, false}, {"audio", "recvonly", true, false}} {
		err := validateGo2RTCOffer(cameraOffer(tc.kind, tc.direction), tc.talk)
		if (err == nil) != tc.allowed {
			t.Fatalf("%+v: %v", tc, err)
		}
	}
	if validateGo2RTCOffer(cameraOffer("audio", ""), false) == nil {
		t.Fatal("default sendrecv accepted")
	}
	value, _ := json.Marshal(map[string]any{"type": "webrtc", "value": map[string]any{"sdp": cameraOffer("video", "recvonly"), "ice_servers": []any{}}})
	if _, err := validateGo2RTCMessage(value, false); err == nil {
		t.Fatal("arbitrary v2 configuration accepted")
	}
}
