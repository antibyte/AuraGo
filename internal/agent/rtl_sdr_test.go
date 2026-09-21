package agent

import (
	"aurago/internal/config"
	"aurago/internal/tools"
	"context"
	"encoding/json"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestRTLSDRNativeArgumentsPreserveTuningAndSchedule(t *testing.T) {
	native := openai.ToolCall{ID: "call-radio", Type: "function", Function: openai.FunctionCall{Name: "rtl_sdr", Arguments: `{"operation":"schedule","name":"News","tuning":{"mode":"nfm","frequency_hz":145500000},"duration_seconds":600,"start_at":"2030-01-02T12:00:00+01:00","timezone":"Europe/Berlin","repeat":"daily","transcribe":true}`}}
	call := NativeToolCallToToolCall(native, nil)
	data, err := json.Marshal(call.Params)
	if err != nil {
		t.Fatal(err)
	}
	var req tools.RTLSDRRequest
	if err = json.Unmarshal(data, &req); err != nil {
		t.Fatal(err)
	}
	if req.Tuning.Frequency != 145500000 || !req.Tuning.AGC || req.Tuning.Squelch != -100 || req.Timezone != "Europe/Berlin" || !req.Transcribe || req.Duration != 600 {
		t.Fatalf("arguments lost at native dispatch: %+v", req)
	}
	if err = req.Tuning.Validate(); err != nil || req.Tuning.Bandwidth != 12500 {
		t.Fatalf("mode default invalid: %+v %v", req.Tuning, err)
	}
}

func TestRTLSDRDiscoveryAndRevocation(t *testing.T) {
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	cfg.RTLSDR.Enabled = true
	cfg.RTLSDR.AllowAgent = true
	cfg.Docker.Enabled = true
	flags := buildToolFlagsFromConfig(cfg)
	if !flags.RTLSDREnabled {
		t.Fatal("enabled radio not discoverable")
	}
	current := *cfg
	current.RTLSDR.AllowAgent = false
	cfg.AuthorizationSnapshots = func() (*config.Config, *config.Config) { return cfg, &current }
	effective, ok := dispatchAuthorization(cfg)
	if !ok || effective.RTLSDR.AllowAgent {
		t.Fatal("live grant not intersected")
	}
	if !revokedNativeTools(cfg, effective)["rtl_sdr"] {
		t.Fatal("removed radio schema still dispatchable")
	}
	for _, wrapped := range []bool{false, true} {
		dc := &DispatchContext{Cfg: cfg, SessionID: t.Name()}
		call := ToolCall{Action: "rtl_sdr", Operation: "scan"}
		if wrapped {
			setRunDiscoverToolsState(dc, dispatchCatalogSchemas(dc), nil)
			call = ToolCall{Action: "invoke_tool", Params: map[string]interface{}{"tool_name": "rtl_sdr", "arguments": map[string]interface{}{"operation": "scan"}}}
		}
		result := DispatchToolCallResult(context.Background(), &call, dc, "scan radio")
		if !result.IsError {
			t.Fatalf("revoked tool executed: %+v", result)
		}
	}
	ClearDiscoverToolsState(t.Name())
	output, handled := dispatchPlatform(context.Background(), ToolCall{Action: "rtl_sdr", Operation: "status"}, &DispatchContext{Cfg: effective})
	if !handled || !strings.Contains(output, "sdr_disabled") {
		t.Fatalf("radio handler permission bypass: %s", output)
	}
}
