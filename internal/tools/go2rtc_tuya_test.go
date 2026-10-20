package tools

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

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
