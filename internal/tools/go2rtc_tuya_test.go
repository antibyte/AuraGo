package tools

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestGo2RTCTuyaPrivateRequestAndCapability(t *testing.T) {
	capable := false
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/go2rtc/proxy/api" {
			if capable {
				w.Write([]byte(`{"aurago_tuya":"post-v1"}`))
			} else {
				w.Write([]byte(`{"version":"stock"}`))
			}
			return
		}
		calls++
		user, password, ok := r.BasicAuth()
		if !ok || user != "aurago" || password != "fixture" {
			t.Error("private API auth lost")
		}
		body, _ := io.ReadAll(r.Body)
		if r.Method != "POST" || r.URL.RawQuery != "" || !strings.Contains(string(body), "secretABC") || r.Header.Get("Content-Type") != "application/json" {
			t.Error("credentials left POST body")
		}
		w.Write([]byte(`{"status":"error","error_code":"tuya_auth_failed","raw":"secretABC"}`))
	}))
	defer server.Close()
	u, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(u.Port())
	m := NewGo2RTCManager(&config.Config{Go2RTC: config.Go2RTCConfig{Enabled: true, URL: server.URL, APIHostPort: port, APIPassword: "fixture"}}, nil, nil, nil)
	defer m.Close()
	c := config.Go2RTCTuyaCredentials{Email: "email@example.org", Password: "secretABC"}
	a := config.Go2RTCTuyaAccount{Method: "smart", Region: "eu-central"}
	result := m.TuyaDiscover(context.Background(), a, c)
	if result.ErrorCode != "tuya_sidecar_unsupported" || calls != 0 {
		t.Fatal("stock image received login")
	}
	capable = true
	result = m.TuyaDiscover(context.Background(), a, c)
	if result.ErrorCode != "tuya_auth_failed" || calls != 1 || strings.Contains(go2RTCJSON(result), "secretABC") {
		t.Fatal("unexpected or unsafe Tuya result")
	}
}

func TestGo2RTCTuyaAccountLimitIncludesNativeProbes(t *testing.T) {
	m := NewGo2RTCManager(&config.Config{}, nil, nil, nil)
	defer m.Close()
	release, ok := m.claimTuya("home")
	if !ok {
		t.Fatal("could not reserve account")
	}
	if result := m.TuyaTest(context.Background(), "", "home"); result.ErrorCode != "tuya_busy" {
		t.Fatal("second native/REST probe admitted")
	}
	if result := m.TuyaDiscover(context.Background(), config.Go2RTCTuyaAccount{ID: "home"}, config.Go2RTCTuyaCredentials{}); result.ErrorCode != "tuya_busy" {
		t.Fatal("discovery bypassed account limit")
	}
	release()
}

func TestGo2RTCTuyaCancelledCheckKeepsConfirmedAPIHealth(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(strconv.FormatBool(deadline), func(t *testing.T) {
			started := make(chan struct{})
			var phase atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch phase.Load() {
				case 0:
					w.Write([]byte(`{"aurago_tuya":"post-v1","aurago_webrtc_lifecycle":"ws-close-v1"}`))
				case 1:
					close(started)
					<-r.Context().Done()
				default:
					http.Error(w, "offline", http.StatusServiceUnavailable)
				}
			}))
			defer server.Close()
			u, _ := url.Parse(server.URL)
			port, _ := strconv.Atoi(u.Port())
			m := NewGo2RTCManager(&config.Config{Go2RTC: config.Go2RTCConfig{Enabled: true, URL: server.URL, APIHostPort: port, APIPassword: "fixture"}}, nil, nil, nil)
			defer m.Close()
			if status, err := m.Test(context.Background()); err != nil || !status.APIUsable {
				t.Fatal("fixture API not healthy", err)
			}
			phase.Store(1)
			ctx, cancel := context.WithCancel(context.Background())
			if deadline {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 400*time.Millisecond)
			}
			defer cancel()
			done := make(chan Go2RTCTuyaResult, 1)
			go func() {
				done <- m.TuyaDiscover(ctx, config.Go2RTCTuyaAccount{ID: "home", Method: "smart", Region: "eu-central"}, config.Go2RTCTuyaCredentials{Email: "fixture@example.org", Password: "fixture-password"})
			}()
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				t.Fatal("health request not started")
			}
			if !deadline {
				cancel()
			}
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("abandoned check did not finish")
			}
			status := m.currentStatus()
			if !status.APIUsable || !status.TuyaSetup || !status.WebRTCSessionBound || status.LastError != "" {
				t.Fatal("abandoned check changed confirmed API health or capabilities")
			}
			phase.Store(2)
			if status, err := m.Test(context.Background()); err == nil || status.APIUsable {
				t.Fatal("a genuine API failure did not update health")
			}
		})
	}
}
