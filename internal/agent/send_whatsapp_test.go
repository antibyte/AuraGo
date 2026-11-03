package agent

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

// recordingWhatsAppSender is the owner sender the server would publish. It is a pointer, so it
// is comparable as the interface requires.
type recordingWhatsAppSender struct {
	ready bool
	res   tools.WhatsAppSendResult
	err   error

	calls int
	text  string
	file  string
}

func (f *recordingWhatsAppSender) Ready() bool { return f.ready }

func (f *recordingWhatsAppSender) SendOwnerMessage(_ context.Context, text, filePath string) (tools.WhatsAppSendResult, error) {
	f.calls++
	f.text, f.file = text, filePath
	return f.res, f.err
}

// publishWhatsAppSender publishes s for the test and withdraws it afterwards. The hook is
// process-wide state, so the callers are not parallel.
func publishWhatsAppSender(t *testing.T, s tools.WhatsAppOwnerSender) {
	t.Helper()
	tools.SetWhatsAppOwnerSender(s)
	t.Cleanup(func() { tools.SetWhatsAppOwnerSender(nil) })
}

func whatsAppDispatchConfig(readonly bool) *config.Config {
	cfg := &config.Config{}
	cfg.WhatsApp.Enabled = true
	cfg.WhatsApp.ReadOnly = readonly
	return cfg
}

// TestSendWhatsAppReachesItsDispatchCase proves that the outer case list of dispatchComm names
// send_whatsapp: a call the dispatch does not route would come back unhandled.
func TestSendWhatsAppReachesItsDispatchCase(t *testing.T) {
	dc := &DispatchContext{Cfg: whatsAppDispatchConfig(true), Logger: slog.Default()}
	out, handled := dispatchComm(context.Background(), ToolCall{Action: "send_whatsapp", Params: map[string]interface{}{"message": "x"}}, dc)
	if !handled || !strings.Contains(out, "read-only mode") {
		t.Fatalf("handled=%v output=%s", handled, out)
	}

	dc.Cfg = &config.Config{}
	out, handled = dispatchComm(context.Background(), ToolCall{Action: "send_whatsapp", Params: map[string]interface{}{"message": "x"}}, dc)
	if !handled || !strings.Contains(out, "not enabled") {
		t.Fatalf("disabled: handled=%v output=%s", handled, out)
	}
}

// Not parallel: the owner sender is process-wide state.
func TestSendWhatsAppDispatchUsesThePublishedOwnerSender(t *testing.T) {
	dc := &DispatchContext{Cfg: whatsAppDispatchConfig(false), Logger: slog.Default()}
	run := func(params map[string]interface{}) string {
		t.Helper()
		out, handled := dispatchMessagingCases(context.Background(), ToolCall{Action: "send_whatsapp", Params: params}, dc)
		if !handled {
			t.Fatalf("send_whatsapp was not handled: %s", out)
		}
		return out
	}

	fake := &recordingWhatsAppSender{ready: true, res: tools.WhatsAppSendResult{Kind: "document", File: "a.pdf"}}
	publishWhatsAppSender(t, fake)

	out := run(map[string]interface{}{"message": "hi", "file_path": "a.pdf"})
	if fake.calls != 1 || fake.text != "hi" || fake.file != "a.pdf" {
		t.Fatalf("sender got calls=%d text=%q file=%q", fake.calls, fake.text, fake.file)
	}
	if !strings.Contains(out, `"channel":"whatsapp"`) || !strings.Contains(out, `"document":"a.pdf"`) || !strings.Contains(out, `"status":"success"`) {
		t.Fatalf("output = %s", out)
	}

	// A stray path argument names no file (the send_telegram rule).
	fake.calls, fake.res = 0, tools.WhatsAppSendResult{Kind: "text", TextSent: true}
	out = run(map[string]interface{}{"message": "hi", "path": "a.pdf"})
	if fake.calls != 1 || fake.file != "" || strings.Contains(out, `"document"`) {
		t.Fatalf("a stray path sent a file: file=%q output=%s", fake.file, out)
	}

	// A file_path that is no string fails before anything is sent.
	fake.calls = 0
	out = run(map[string]interface{}{"message": "hi", "file_path": []interface{}{"a"}})
	if fake.calls != 0 || !strings.Contains(out, "file_path must be one file path string") {
		t.Fatalf("calls=%d output=%s", fake.calls, out)
	}

	// Neither text nor file.
	out = run(map[string]interface{}{})
	if fake.calls != 0 || !strings.Contains(out, "message is required") {
		t.Fatalf("calls=%d output=%s", fake.calls, out)
	}

	// No published sender.
	tools.SetWhatsAppOwnerSender(nil)
	out = run(map[string]interface{}{"message": "hi"})
	if !strings.Contains(out, "not linked or not running") {
		t.Fatalf("output = %s", out)
	}
}

// Not parallel: the owner sender is process-wide state.
func TestSendWhatsAppIsOfferedOnlyWhenEnabledLinkedAndWritable(t *testing.T) {
	resolve := func(cfg *config.Config) resolvedToolFeatureState {
		return resolveToolFeatureState(RunConfig{Config: cfg}, buildToolingPolicy(cfg, ""))
	}
	offered := func(state resolvedToolFeatureState) bool {
		for _, name := range builtinToolNames(state.ToolFlags) {
			if name == "send_whatsapp" {
				return true
			}
		}
		return false
	}

	publishWhatsAppSender(t, nil)
	if state := resolve(&config.Config{}); state.ToolFlags.WhatsAppEnabled || offered(state) {
		t.Fatal("a disabled WhatsApp offers send_whatsapp")
	}
	if state := resolve(whatsAppDispatchConfig(false)); state.ToolFlags.WhatsAppEnabled || offered(state) {
		t.Fatal("send_whatsapp is offered without a published sender")
	}

	fake := &recordingWhatsAppSender{ready: true}
	publishWhatsAppSender(t, fake)
	state := resolve(whatsAppDispatchConfig(false))
	if !state.ToolFlags.WhatsAppEnabled || !offered(state) {
		t.Fatal("an enabled, linked WhatsApp does not offer send_whatsapp")
	}
	if !state.ToolFlags.NotificationEnabled {
		t.Fatal("WhatsApp as the only channel must still offer send_notification")
	}
	// The prompt sees the same flag, and it takes part in the prompt cache key.
	cfg := whatsAppDispatchConfig(false)
	promptFlags := buildPromptContextFlags(RunConfig{Config: cfg}, buildToolingPolicy(cfg, ""), promptContextOptions{})
	if !promptFlags.WhatsAppEnabled || !slices.Contains(collectEnabledTools(&promptFlags), "whatsapp") {
		t.Fatal("the prompt flags do not carry WhatsAppEnabled")
	}
	if state := resolve(whatsAppDispatchConfig(true)); state.ToolFlags.WhatsAppEnabled || offered(state) {
		t.Fatal("a read-only WhatsApp offers send_whatsapp")
	}

	fake.ready = false
	if state := resolve(whatsAppDispatchConfig(false)); state.ToolFlags.WhatsAppEnabled || offered(state) {
		t.Fatal("a sender that is not ready offers send_whatsapp")
	}

	if (ToolFeatureFlags{}).Key() == (ToolFeatureFlags{WhatsAppEnabled: true}).Key() {
		t.Fatal("the tool feature key ignores WhatsAppEnabled")
	}
}

func TestSendWhatsAppPolicyPrecheckAndRisk(t *testing.T) {
	// A call without text is refused; a file needs no text.
	out, blocked := precheckMessagingToolArgs(ToolCall{IsTool: true, Action: "send_whatsapp"}, RunConfig{MessageSource: "web_chat"}, "default")
	if !blocked || !strings.Contains(out, "message is required") {
		t.Fatalf("empty call: blocked=%v output=%s", blocked, out)
	}
	for name, tc := range map[string]ToolCall{
		"file_path param": {IsTool: true, Action: "send_whatsapp", Params: map[string]interface{}{"file_path": "a.pdf"}},
		"message only":    {IsTool: true, Action: "send_whatsapp", Params: map[string]interface{}{"message": "m"}},
	} {
		if out, blocked := precheckMessagingToolArgs(tc, RunConfig{MessageSource: "web_chat"}, "default"); blocked {
			t.Errorf("%s: blocked: %s", name, out)
		}
	}

	call := ToolCall{Action: "send_whatsapp", Params: map[string]interface{}{"message": "m"}}
	action := DescribeAction(call, nil)
	if !action.Mutating || !action.HighRisk {
		t.Fatalf("action = %+v, want mutating and high risk", action)
	}
	if !egressPolicyTool(action, toolCallArguments(call)) {
		t.Fatal("send_whatsapp is not treated as egress")
	}
	if got := ToolCategoryForName("send_whatsapp"); got != "communication" {
		t.Fatalf("category = %q", got)
	}
}

func TestGuardianSeesSendWhatsAppMessageAndFile(t *testing.T) {
	params := c06GuardianParams(t, "send_whatsapp", `{"message":"hello","file_path":"secrets/keys.txt"}`)
	if params["message"] != "hello" || params["file_path"] != "secrets/keys.txt" {
		t.Fatalf("params = %#v", params)
	}
	if _, ok := params["title"]; ok {
		t.Fatalf("send_whatsapp has no title: %#v", params)
	}

	// The text keeps its head and tail within the same 300 bytes as send_telegram.
	long := strings.Repeat("ä", 9000)
	if got := c06GuardianParams(t, "send_whatsapp", mustJSON(t, map[string]any{"message": long}))["message"]; got == "" || len(got) > 300 {
		t.Fatalf("message is %d bytes", len(got))
	}

	// A different file or text gives a different Guardian cache key.
	other := c06GuardianParams(t, "send_whatsapp", `{"message":"hello","file_path":"notes/todo.txt"}`)
	if c06GuardianKey("send_whatsapp", params) == c06GuardianKey("send_whatsapp", other) {
		t.Fatal("two different files share one Guardian cache key")
	}

	// A stray path is no file of send_whatsapp.
	for _, arguments := range []string{
		`{"message":"m","path":"secrets/keys.txt"}`,
		`{"message":"m","filepath":"secrets/keys.txt"}`,
	} {
		if stray := c06GuardianParams(t, "send_whatsapp", arguments); stray["file_path"] != "" || stray["path_scope"] != "" {
			t.Errorf("%s: a stray path became the file: %#v", arguments, stray)
		}
	}
	if both := c06GuardianParams(t, "send_whatsapp", `{"message":"m","path":"secrets/keys.txt","file_path":"notes/todo.txt"}`); both["file_path"] != "notes/todo.txt" {
		t.Errorf("file_path next to a stray path: %#v", both)
	}
}

// TestSendNotificationOffersTheWhatsAppChannel proves that the send_notification schema names
// the channel, with and without the CYD channel, and says so in its description.
func TestSendNotificationOffersTheWhatsAppChannel(t *testing.T) {
	for name, ff := range map[string]ToolFeatureFlags{
		"notification": {NotificationEnabled: true},
		"with cyd":     {NotificationEnabled: true, CydEnabled: true},
	} {
		var found bool
		for _, s := range builtinToolSchemas(ff) {
			if s.Function == nil || s.Function.Name != "send_notification" {
				continue
			}
			found = true
			if !strings.Contains(s.Function.Description, "WhatsApp") {
				t.Errorf("%s: description = %q", name, s.Function.Description)
			}
			raw, err := json.Marshal(s.Function.Parameters)
			if err != nil {
				t.Fatal(err)
			}
			var params struct {
				Properties struct {
					Channel struct {
						Enum []string `json:"enum"`
					} `json:"channel"`
				} `json:"properties"`
			}
			if err := json.Unmarshal(raw, &params); err != nil {
				t.Fatal(err)
			}
			enum := params.Properties.Channel.Enum
			at := slices.Index(enum, "whatsapp")
			if at < 0 || at > slices.Index(enum, "all") {
				t.Errorf("%s: channel enum = %v, want whatsapp before all", name, enum)
			}
		}
		if !found {
			t.Errorf("%s: send_notification is not offered", name)
		}
	}
}

// Not parallel: the owner sender is process-wide state.
func TestSendWhatsAppConfiguredButNotLinkedNeedsSetup(t *testing.T) {
	publishWhatsAppSender(t, nil)
	call := func(cfg *config.Config) ToolDispatchResult {
		cfg.Directories.SkillsDir = t.TempDir()
		dc := &DispatchContext{Cfg: cfg, SessionID: t.Name(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
		tc := ToolCall{Action: "send_whatsapp", Params: map[string]interface{}{"message": "m"}}
		return DispatchToolCallResult(context.Background(), &tc, dc, "fixture")
	}

	// Enabled but not linked: needs setup, and the text says what is missing.
	result := call(whatsAppDispatchConfig(false))
	if result.Status != ToolResultNeedsSetup || !strings.Contains(result.Output, "not linked or not running") || strings.Contains(result.Output, "administrator") {
		t.Fatalf("enabled, not linked: %+v", result)
	}
	// Switched off in the configuration: the tool is disabled, and that is for an administrator.
	result = call(&config.Config{})
	if result.Status != ToolResultDenied || !strings.Contains(result.Output, "tool_disabled") {
		t.Fatalf("disabled: %+v", result)
	}
}
