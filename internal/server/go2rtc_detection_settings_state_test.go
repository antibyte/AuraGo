package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func TestGo2RTCAppStateExposesDetectionSettingsToAdminsOnly(t *testing.T) {
	cfg := &config.Config{}
	cfg.Go2RTC = config.Go2RTCConfig{
		Enabled: true, URL: "http://127.0.0.1:1", APIHostPort: 1,
		Streams: []config.Go2RTCStreamConfig{{ID: "front-door", Name: "Front door", Enabled: true, Source: "rtsp://192.168.1.20/live", SourceConfigured: true}},
		Detection: config.Go2RTCDetectionConfig{
			Enabled: true, Model: "accurate", Backend: "gpu", RetentionDays: 7, MaxStorageMB: 500,
			Schedule: []config.Go2RTCDetectionWindow{{Days: []string{"mon", "tue"}, From: "22:00", To: "06:00"}},
		},
	}
	server := &Server{Cfg: cfg, Go2RTC: tools.NewGo2RTCManager(cfg, nil, nil, nil)}
	adminState := func() (string, *config.Go2RTCDetectionConfig) {
		t.Helper()
		recorder := httptest.NewRecorder()
		handleGo2RTCAppState(server).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/go2rtc/app/state", nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("admin state status = %d; body=%s", recorder.Code, recorder.Body.String())
		}
		var body struct {
			DetectionSettings *config.Go2RTCDetectionConfig `json:"detection_settings"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return recorder.Body.String(), body.DetectionSettings
	}

	_, got := adminState()
	if got == nil || !got.Enabled || got.Model != "accurate" || got.Backend != "gpu" || got.RetentionDays != 7 || got.MaxStorageMB != 500 ||
		len(got.Schedule) != 1 || got.Schedule[0].From != "22:00" || got.Schedule[0].To != "06:00" || strings.Join(got.Schedule[0].Days, ",") != "mon,tue" {
		t.Fatalf("detection_settings = %+v", got)
	}

	viewerRequest := httptest.NewRequest(http.MethodGet, "/api/go2rtc/app/state", nil)
	viewerRequest.Header.Set("Authorization", "Bearer viewer-scope-only")
	viewer := httptest.NewRecorder()
	handleGo2RTCAppState(server).ServeHTTP(viewer, viewerRequest)
	if viewer.Code != http.StatusOK || strings.Contains(viewer.Body.String(), "detection_settings") {
		t.Fatalf("viewer state = %d %s", viewer.Code, viewer.Body.String())
	}

	// A config without a detection block answers with the slice 2 defaults and an
	// empty schedule list (never null), so the dialog can save without wiping anything.
	cfg.Go2RTC.Detection = config.Go2RTCDetectionConfig{}
	server.Go2RTC.Configure(cfg)
	raw, defaults := adminState()
	if defaults == nil || defaults.Enabled || defaults.Model != config.Go2RTCDetectionDefaultModel || defaults.Backend != config.Go2RTCDetectionDefaultBackend ||
		defaults.RetentionDays != config.Go2RTCDetectionDefaultRetentionDays || defaults.MaxStorageMB != config.Go2RTCDetectionDefaultMaxStorageMB ||
		!strings.Contains(raw, `"schedule":[]`) {
		t.Fatalf("detection_settings for an empty block = %+v (%s)", defaults, raw)
	}
}

// The settings dialog saves the whole global block, so it must read the published
// config, not the manager's copy, which a config-page save reconfigures in a
// goroutine and which can therefore briefly lag behind (and lose a schedule).
func TestGo2RTCAppStateReadsDetectionSettingsFromPublishedConfig(t *testing.T) {
	stale := &config.Config{}
	stale.Go2RTC = config.Go2RTCConfig{
		Enabled: true, URL: "http://127.0.0.1:1", APIHostPort: 1,
		Detection: config.Go2RTCDetectionConfig{Enabled: false, Model: "standard", Backend: "auto", RetentionDays: 14, MaxStorageMB: 2048},
	}
	published := &config.Config{}
	published.Go2RTC = stale.Go2RTC
	published.Go2RTC.Detection = config.Go2RTCDetectionConfig{
		Enabled: true, Model: "accurate", Backend: "cpu", RetentionDays: 30, MaxStorageMB: 900,
		Schedule: []config.Go2RTCDetectionWindow{{Days: []string{"sat"}, From: "08:00", To: "18:00"}},
	}
	server := &Server{Cfg: stale, Go2RTC: tools.NewGo2RTCManager(stale, nil, nil, nil)}
	server.cfgSnapshot.Store(published)

	recorder := httptest.NewRecorder()
	handleGo2RTCAppState(server).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/go2rtc/app/state", nil))
	var body struct {
		DetectionSettings *config.Go2RTCDetectionConfig `json:"detection_settings"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	got := body.DetectionSettings
	if got == nil || !got.Enabled || got.Model != "accurate" || got.Backend != "cpu" || got.RetentionDays != 30 || got.MaxStorageMB != 900 ||
		len(got.Schedule) != 1 || strings.Join(got.Schedule[0].Days, ",") != "sat" {
		t.Fatalf("detection_settings = %+v, want the published block", got)
	}
}
