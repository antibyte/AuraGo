package agent

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
	"aurago/internal/tools"
)

// useAvailableGo2RTCManager publishes a go2rtc manager whose authenticated API answers, like
// TestGo2RTCToolCapabilityRequiresReachableRuntimeAPI.
func useAvailableGo2RTCManager(t *testing.T, cfg *config.Config) {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/go2rtc/proxy/api" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"version": "1.9.14"})
	}))
	t.Cleanup(upstream.Close)
	parsed, _ := url.Parse(upstream.URL)
	port, _ := strconv.Atoi(parsed.Port())
	cfg.Go2RTC.URL, cfg.Go2RTC.APIHostPort, cfg.Go2RTC.APIPassword = upstream.URL, port, "detection-test-password"
	manager := tools.NewGo2RTCManager(cfg, nil, nil, nil)
	if _, err := manager.Test(context.Background()); err != nil {
		t.Fatalf("prime go2rtc manager: %v", err)
	}
	previous := tools.DefaultGo2RTCManager()
	tools.SetDefaultGo2RTCManager(manager)
	t.Cleanup(func() { tools.SetDefaultGo2RTCManager(previous) })
}

func TestGo2RTCDetectionOperationsRouteThroughTheTool(t *testing.T) {
	cfg := detectionTestConfig()
	useAvailableGo2RTCManager(t, cfg)
	useDetectionAPI(t, &fakeDetectionAPI{status: detectionStatusView{Enabled: true, Detector: detectionDetectorView{State: "ready"}, Arm: detectionArmView{Mode: "off"}}})
	dc := &DispatchContext{Cfg: cfg, Logger: testLogger}
	out, handled := dispatchPlatform(context.Background(), ToolCall{Action: "go2rtc", Params: map[string]interface{}{"operation": "detection_status"}}, dc)
	if m := decodeDetectionOutput(t, out); !handled || m["status"] != "ok" || m["operation"] != "detection_status" {
		t.Fatalf("dispatch = %v %s", handled, out)
	}
	out, _ = dispatchPlatform(context.Background(), ToolCall{Action: "go2rtc", Operation: "teleport"}, dc)
	if !strings.Contains(out, "detection_status, set_arm_mode, list_detection_events, analyze_detection_event, show_detection_event") {
		t.Fatalf("unknown operation hint = %s", out)
	}
	cfg.Go2RTC.AgentAccess = false
	out, _ = dispatchPlatform(context.Background(), ToolCall{Action: "go2rtc", Operation: "detection_status"}, dc)
	if !strings.Contains(out, "not enabled for agent access") {
		t.Fatalf("the go2rtc gate must apply first: %s", out)
	}
}

func TestSafeExplicitRetryCoversReadOnlyDetectionOperations(t *testing.T) {
	for _, op := range []string{"detection_status", "list_detection_events", "analyze_detection_event", "show_detection_event"} {
		if !safeExplicitRetryTool("go2rtc", op) {
			t.Errorf("%s must be retryable", op)
		}
	}
	if safeExplicitRetryTool("go2rtc", "set_arm_mode") {
		t.Error("set_arm_mode changes state and must not be retried automatically")
	}
}
