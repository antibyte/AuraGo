package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestGo2RTCDiagnosticsReportsSanitizedStageFailures(t *testing.T) {
	for _, test := range []struct{ name, stage, code string }{
		{"success", "complete", ""}, {"api", "api", "api_unavailable"},
		{"snapshot", "snapshot", "snapshot_unavailable"}, {"hardware", "hardware", "hardware_not_verified"},
	} {
		t.Run(test.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/api"):
					if test.name == "api" {
						http.Error(w, "rtsp://fixture-user:fixture-secret@192.168.1.2/live", 502)
						return
					}
					w.Write([]byte(`{"version":"1.9.14-aurago1"}`))
				case strings.HasSuffix(r.URL.Path, "/api/streams"):
					json.NewEncoder(w).Encode(map[string]any{Go2RTCRuntimeStreamName("one"): map[string]any{"producers": []any{map[string]any{"codec_name": "H264", "url": "rtsp://fixture-user:fixture-secret@192.168.1.2/live"}}}})
				case strings.HasSuffix(r.URL.Path, "/frame.jpeg"):
					if test.name == "snapshot" {
						http.Error(w, "rtsp://fixture-user:fixture-secret@192.168.1.2/live", 502)
						return
					}
					w.Header().Set("Content-Type", "image/jpeg")
					w.Write(go2RTCTestJPEG(80, 40))
				default:
					http.NotFound(w, r)
				}
			}))
			defer upstream.Close()
			u, _ := url.Parse(upstream.URL)
			port, _ := strconv.Atoi(u.Port())
			cfg := &config.Config{Go2RTC: config.Go2RTCConfig{Enabled: true, URL: upstream.URL, APIHostPort: port, APIPassword: "fixture", Streams: []config.Go2RTCStreamConfig{{ID: "one", Enabled: true, Source: "rtsp://fixture-user:fixture-secret@192.168.1.2/live"}}}}
			cfg.Docker.Host = upstream.URL
			if test.name == "hardware" {
				cfg.Go2RTC.Hardware.Backend = "vaapi"
				cfg.Go2RTC.Streams[0].CodecProfile = "h264_opus"
			}
			m := NewGo2RTCManager(cfg, nil, nil, nil)
			defer m.Close()
			result := m.Diagnose(context.Background(), "one")
			if result.Stage != test.stage || result.ErrorCode != test.code || (result.Status == "ok") != (test.code == "") {
				t.Fatalf("wrong diagnosis: %+v", result)
			}
			if test.name == "success" && (result.Width != 80 || result.Height != 40 || result.EffectiveBackend != "original" || result.Bytes == 0) {
				t.Fatalf("missing successful image metadata: %+v", result)
			}
			encoded, _ := json.Marshal(result)
			if strings.Contains(string(encoded), "fixture-secret") || strings.Contains(string(encoded), "rtsp://") {
				t.Fatalf("diagnosis exposed the private source: %s", encoded)
			}
		})
	}
}
