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

func TestGo2RTCToolSchemaIsGatedAndReadOnly(t *testing.T) {
	if names := toolNames(BuildNativeToolSchemas(t.TempDir(), nil, ToolFeatureFlags{}, nil)); containsName(names, "go2rtc") {
		t.Fatal("go2rtc schema present while capability is disabled")
	}
	schemas := BuildNativeToolSchemas(t.TempDir(), nil, ToolFeatureFlags{Go2RTCEnabled: true}, nil)
	if names := toolNames(schemas); !containsName(names, "go2rtc") {
		t.Fatal("go2rtc schema missing while capability is enabled")
	}
	properties := nativeToolProperties(t, schemas, "go2rtc")
	for _, forbidden := range []string{"url", "source", "image", "container_name", "web_ui_enabled"} {
		if _, ok := properties[forbidden]; ok {
			t.Fatalf("go2rtc schema exposes forbidden property %q: %#v", forbidden, properties)
		}
	}
	for _, required := range []string{"operation", "stream_id", "width", "height", "rotate", "cache_seconds", "prompt"} {
		if _, ok := properties[required]; !ok {
			t.Fatalf("go2rtc schema missing property %q: %#v", required, properties)
		}
	}
	operation, ok := properties["operation"].(map[string]interface{})
	if !ok {
		t.Fatalf("operation schema = %#v", properties["operation"])
	}
	for _, required := range []string{"status", "list_streams", "stream_status", "snapshot", "analyze_snapshot", "show_live_stream"} {
		if !containsInterfaceString(operation["enum"], required) {
			t.Fatalf("go2rtc operation enum missing %q: %#v", required, operation["enum"])
		}
	}
	encoded := strings.ToLower(go2RTCJSONForTest(properties))
	for _, forbidden := range []string{"restart", "start_service", "stop_service", "add_stream", "delete_stream"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("go2rtc schema advertises mutation %q: %s", forbidden, encoded)
		}
	}
}

func TestManagedGo2RTCContainerAllowsOnlySanitizedDockerReads(t *testing.T) {
	for _, operation := range []string{"inspect", "inspect_container", "stats", "port"} {
		if !go2RTCDockerOperationSafe(operation) {
			t.Fatalf("safe Docker operation %q was blocked", operation)
		}
	}
	for _, operation := range []string{"start", "stop", "restart", "remove", "logs", "exec", "cp", "top", "connect", "disconnect"} {
		if go2RTCDockerOperationSafe(operation) {
			t.Fatalf("sensitive Docker operation %q was allowed for managed go2rtc", operation)
		}
	}
}

func go2RTCJSONForTest(value interface{}) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func TestGo2RTCToolCapabilityRequiresReachableRuntimeAPI(t *testing.T) {
	previous := tools.DefaultGo2RTCManager()
	defer tools.SetDefaultGo2RTCManager(previous)

	cfg := &config.Config{}
	cfg.Go2RTC.Enabled = true
	cfg.Go2RTC.AgentAccess = true
	tools.SetDefaultGo2RTCManager(nil)
	flags := buildToolFeatureFlags(RunConfig{Config: cfg}, buildToolingPolicy(cfg, ""))
	if flags.Go2RTCEnabled {
		t.Fatal("go2rtc capability enabled without a runtime manager")
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/go2rtc/proxy/api" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"version": "1.9.14"})
	}))
	defer upstream.Close()
	parsed, _ := url.Parse(upstream.URL)
	port, _ := strconv.Atoi(parsed.Port())
	cfg.Go2RTC.URL = upstream.URL
	cfg.Go2RTC.APIHostPort = port
	cfg.Go2RTC.APIPassword = "internal-password"
	manager := tools.NewGo2RTCManager(cfg, nil, nil, nil)
	if _, err := manager.Test(context.Background()); err != nil {
		t.Fatalf("prime go2rtc manager status: %v", err)
	}
	tools.SetDefaultGo2RTCManager(manager)
	flags = buildToolFeatureFlags(RunConfig{Config: cfg}, buildToolingPolicy(cfg, ""))
	if !flags.Go2RTCEnabled {
		t.Fatal("go2rtc capability missing with enabled config and reachable authenticated API")
	}
}
