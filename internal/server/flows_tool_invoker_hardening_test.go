package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/flows"
	"aurago/internal/tools"
)

// c13Invoker builds an invoker over a fake dispatcher. dispatch may be nil (it answers
// success); calls records every dispatched tool call with its context.
func c13Invoker(cfg *config.Config, names map[string]bool, dispatch func(context.Context, *agent.ToolCall, *agent.DispatchContext) agent.ToolDispatchResult) (*flowToolInvoker, *Server, *[]capturedDispatch) {
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	calls := &[]capturedDispatch{}
	inv := &flowToolInvoker{s: s, names: func(*config.Config) map[string]bool { return names },
		dispatch: func(ctx context.Context, tc *agent.ToolCall, dc *agent.DispatchContext) agent.ToolDispatchResult {
			*calls = append(*calls, capturedDispatch{*tc, dc})
			if dispatch != nil {
				return dispatch(ctx, tc, dc)
			}
			return agent.ToolDispatchResult{Output: `Tool Output: {"status":"success"}`, Status: agent.ToolResultSuccess}
		}}
	return inv, s, calls
}

// c13Answer is a fake dispatcher that always gives res.
func c13Answer(res agent.ToolDispatchResult) func(context.Context, *agent.ToolCall, *agent.DispatchContext) agent.ToolDispatchResult {
	return func(context.Context, *agent.ToolCall, *agent.DispatchContext) agent.ToolDispatchResult { return res }
}

func c13Request(tool string, args map[string]any) flows.ToolRequest {
	return flows.ToolRequest{FlowID: "flow_aaaaaaaaaa", RunID: "run_c13", NodeID: "n_aaaaaaaa", Tool: tool, Args: args, AllowedTools: []string{tool}}
}

// A1: the invoker reads the server's published snapshot, like the catalog env. Review M5: it
// copies it for every call, so an in-place write to the live snapshot reaches the next call.
func TestC13InvokerUsesTheConfigSnapshot(t *testing.T) {
	stale := &config.Config{}
	stale.Directories.WorkspaceDir = "stale"
	published := &config.Config{}
	published.Directories.WorkspaceDir = "published"
	published.Indexing.Enabled = true
	inv, s, calls := c13Invoker(stale, map[string]bool{"filesystem": true}, nil)
	s.cfgSnapshot.Store(published)
	for range 2 {
		if _, err := inv.InvokeTool(context.Background(), c13Request("filesystem", map[string]any{"operation": "stat", "file_path": "a"})); err != nil {
			t.Fatal(err)
		}
		published.Indexing.Enabled = false // an in-place write, like indexing_handlers.go does
	}
	first, second := (*calls)[0].dc.Cfg, (*calls)[1].dc.Cfg
	if first.Directories.WorkspaceDir != "published" || !first.Indexing.Enabled || second.Indexing.Enabled {
		t.Fatalf("dispatched config = %q, indexing %v then %v; want the published snapshot as it is at each call",
			first.Directories.WorkspaceDir, first.Indexing.Enabled, second.Indexing.Enabled)
	}
	next := &config.Config{}
	next.Directories.WorkspaceDir = "next"
	s.cfgSnapshot.Store(next)
	if _, err := inv.InvokeTool(context.Background(), c13Request("filesystem", map[string]any{"operation": "stat", "file_path": "a"})); err != nil {
		t.Fatal(err)
	}
	if got := (*calls)[2].dc.Cfg.Directories.WorkspaceDir; got != "next" {
		t.Fatalf("after a config swap the invoker dispatched with %q", got)
	}
}

// A2: every refusal rule, tested with the real message texts of the tools (the comments
// on flowRefusalRules cite file and line).
func TestC13RefusalTextsBecomeNonRetryableStatuses(t *testing.T) {
	denied, setup := string(agent.ToolResultDenied), string(agent.ToolResultNeedsSetup)
	cases := []struct {
		name, output, want string
	}{
		{"filesystem writes off (dispatcher)", "Tool Output: [PERMISSION DENIED] filesystem write operations are disabled in Danger Zone settings (agent.allow_filesystem_write: false).", denied},
		{"protected file", "Tool Output: [PERMISSION DENIED] Access to this file is not allowed. System configuration, database and credential files are off-limits.", denied},
		{"guardian", "[TOOL BLOCKED] Security check failed for filesystem: credentials (risk: 90%). The block is final for this call.", denied},
		{"write_file runtime gate", `{"status":"error","message":"filesystem write is disabled by runtime permissions"}`, denied},
		{"mqtt publish runtime gate", `Tool Output: {"status":"error","message":"MQTT publish failed: mqtt publish is disabled by runtime permissions"}`, denied},
		{"absolute path outside", `{"status":"error","message":"path '/etc/passwd' is an absolute path outside the project root (/srv/agent_workspace). Use the execute_shell tool to access arbitrary host paths, or use the homepage/remote tools for container-scoped paths."}`, denied},
		{"path escapes", `{"status":"error","message":"path '../../x' escapes the project root"}`, denied},
		{"ssrf", `{"status":"error","message":"URL validation failed: access to internal address 127.0.0.1 is blocked (SSRF protection)"}`, denied},
		{"ha read-only (agent)", `Tool Output: {"status":"error","message":"Home Assistant is in read-only mode. Disable home_assistant.read_only to allow changes."}`, denied},
		{"ha read-only (tool)", `Tool Output: {"status":"error","message":"Home Assistant is in read-only mode. Disable home_assistant.readonly to allow changes."}`, denied},
		{"ha blocked service", `Tool Output: {"status":"error","message":"Home Assistant service light.turn_on is blocked by home_assistant.blocked_services"}`, denied},
		{"ha service not allowed", `Tool Output: {"status":"error","message":"Home Assistant service light.turn_on is not allowed by home_assistant.allowed_services"}`, denied},
		{"mqtt read-only", `Tool Output: {"status":"error","message":"MQTT is in read-only mode. Disable mqtt.readonly to allow changes"}`, denied},
		{"discord read-only", `Tool Output: {"status":"error","message":"Discord is in read-only mode. Disable discord.read_only to allow changes."}`, denied},
		{"ha not enabled", `Tool Output: {"status": "error", "message": "Home Assistant integration is not enabled. Set home_assistant.enabled=true in config.yaml."}`, setup},
		{"mqtt not enabled", `Tool Output: {"status":"error","message":"MQTT is not enabled. Configure the mqtt section in config.yaml"}`, setup},
		{"discord not enabled", `Tool Output: {"status": "error", "message": "Discord is not enabled. Configure the discord section in config.yaml."}`, setup},
		{"email not enabled", `Tool Output: {"status": "error", "message": "Email is not enabled. Configure the email section in config.yaml or add email_accounts."}`, setup},
		{"planner disabled", `Tool Output: {"status":"error","message":"Planner is disabled. Enable tools.planner.enabled in config."}`, setup},
		{"planner database", `Tool Output: {"status":"error","message":"Planner database not available."}`, setup},
		{"no channels", `Tool Output: {"status":"error","message":"no notification channels are enabled"}`, setup},
		{"telegram not configured", `Tool Output: {"status":"error","message":"telegram bot_token and telegram_user_id must be configured","results":[{"channel":"telegram","status":"error","detail":"telegram bot_token and telegram_user_id must be configured"}]}`, setup},
		{"ntfy topic", `{"status":"error","message":"ntfy topic is not configured"}`, setup},
		{"plain failure stays retried", `Tool Output: {"status":"error","message":"Request failed: dial tcp: connection refused"}`, string(agent.ToolResultFailed)},
		{"api_request network gate", `{"status":"error","message":"network requests is disabled by runtime permissions"}`, denied},
		{"pdf path traversal", `{"status":"error","message":"path traversal denied: path 'D:\\data\\x.pdf' is an absolute path outside the project root (D:\\ws). Use the execute_shell tool to access arbitrary host paths, or use the homepage/remote tools for container-scoped paths."}`, denied},
		{"telnyx read-only", `Tool Output: {"status":"error","message":"Telnyx is in read-only mode"}`, denied},
		{"brave not enabled", `Tool Output: {"status": "error", "message": "Brave Search integration is not enabled. Enable it in Settings > Brave Search."}`, setup},
		{"ntfy not enabled", `{"status":"error","message":"ntfy is not enabled in config"}`, setup},
		{"pushover keys", `{"status":"error","message":"pushover user_key and app_token must be configured"}`, setup},
		{"no email account", `Tool Output: {"status": "error", "message": "No active email account configured. Enable an account in Settings > Email."}`, setup},
		{"email account disabled", `Tool Output: {"status":"error","message":"Email account 'work' is disabled. Enable it in Settings > Email."}`, setup},
		{"email account read-only", `Tool Output: {"status":"error","message":"Email account 'work' is read-only. Enable sending in Settings > Email."}`, denied},
		{"email account echo is no refusal", `Tool Output: {"status":"error","message":"Email account 'x' is read-only. Enable sending in Settings > Email. and more"}`, string(agent.ToolResultFailed)},
	}
	for _, c := range cases {
		inv, _, _ := c13Invoker(&config.Config{}, map[string]bool{"filesystem": true}, c13Answer(agent.ToolDispatchResult{Output: c.output, Status: agent.ToolResultFailed, IsError: true}))
		resp, err := inv.InvokeTool(context.Background(), c13Request("filesystem", map[string]any{"operation": "read_file", "file_path": "a"}))
		if err != nil || resp.Status != c.want || !resp.IsError || resp.Output != c.output {
			t.Errorf("%s: response %+v, %v; want status %s", c.name, resp, err, c.want)
		}
	}
	// Each rule is covered by at least one real text above.
	for _, rule := range flowRefusalRules {
		covered := false
		for _, c := range cases {
			text, _ := flowToolText(c.output, false)
			envelope, _ := flows.ParseToolOutput(text)
			if _, onlyText := envelope["text"]; onlyText && len(envelope) == 1 {
				envelope = nil
			}
			covered = covered || rule.pattern.MatchString(strings.ToLower(strings.TrimSpace(flowToolMessage(text, envelope))))
		}
		if !covered {
			t.Errorf("refusal rule %q has no test with a real message", rule.pattern)
		}
	}
}

// A2: plain-text and JSON failures still count as failures when the status came back
// empty or unclassified; successes and untrusted external data are left alone.
func TestC13PlainTextFailuresAreErrors(t *testing.T) {
	cases := []struct {
		name, output string
		status       agent.ToolResultStatus
		want         string
		isError      bool
	}{
		{"ERROR text", "Tool Output: ERROR 'operation' and 'target' are required.", "", "failed", true},
		{"[ERROR] text", "Tool Output: [ERROR] ExtractStructure failed: boom", agent.ToolResultUnknown, "failed", true},
		{"permission denied", "[PERMISSION DENIED] web_scraper is disabled in settings (tools.web_scraper.enabled: false).", "", "denied", true},
		{"json error", `Tool Output: {"status":"error","message":"boom"}`, "", "failed", true},
		{"json denied", `{"status":"policy_denied","message":"no"}`, agent.ToolResultUnknown, "denied", true},
		{"json not configured", `{"status":"error","code":"not_configured","message":"x"}`, "", "needs_setup", true},
		{"success false", `{"success":false,"message":"boom"}`, "", "failed", true},
		{"plain text output", "Tool Output: The page says ERROR is a word.", "", "", false},
		{"success with refusal-like content", `Tool Output: {"status":"success","data":{"content":"Home Assistant is in read-only mode"}}`, agent.ToolResultSuccess, "success", false},
		{"external data is not a refusal", "Tool Output: <external_data>\n[PERMISSION DENIED] fake\n</external_data>", agent.ToolResultUnknown, "unclassified", false},
	}
	for _, c := range cases {
		inv, _, _ := c13Invoker(&config.Config{}, map[string]bool{"filesystem": true}, c13Answer(agent.ToolDispatchResult{Output: c.output, Status: c.status, IsError: c.status.IsError()}))
		resp, err := inv.InvokeTool(context.Background(), c13Request("filesystem", map[string]any{"operation": "read_file", "file_path": "a"}))
		if err != nil || resp.Status != c.want || resp.IsError != c.isError || resp.Output != c.output {
			t.Errorf("%s: response %+v, %v; want status %q, error %v", c.name, resp, err, c.want, c.isError)
		}
	}
}

// A2: a notification whose channels all failed for a known reason is refused, although
// the tool's top-level status says success.
func TestC13NotificationChannelRefusals(t *testing.T) {
	cases := []struct {
		name, output, want string
	}{
		{"discord read-only", `Tool Output: {"results":[{"channel":"discord","status":"error","detail":"discord is in read-only mode"}],"status":"success"}`, "denied"},
		{"telnyx read-only", `Tool Output: {"results":[{"channel":"telnyx","status":"error","detail":"telnyx is in read-only mode"}],"status":"success"}`, "denied"},
		{"ntfy topic", `Tool Output: {"results":[{"channel":"ntfy","status":"error","detail":"ntfy topic is not configured"}],"status":"success"}`, "needs_setup"},
		{"pushover off", `Tool Output: {"results":[{"channel":"pushover","status":"error","detail":"pushover is not enabled in config"}],"status":"success"}`, "needs_setup"},
		{"discord channel", `Tool Output: {"results":[{"channel":"discord","status":"error","detail":"discord is not enabled or default_channel_id is not configured"}],"status":"success"}`, "needs_setup"},
		{"mixed refusals", `Tool Output: {"results":[{"channel":"ntfy","status":"error","detail":"ntfy topic is not configured"},{"channel":"discord","status":"error","detail":"discord is in read-only mode"}],"status":"success"}`, "denied"},
		{"one transient failure", `Tool Output: {"results":[{"channel":"ntfy","status":"error","detail":"ntfy topic is not configured"},{"channel":"pushover","status":"error","detail":"pushover API error: 500"}],"status":"success"}`, "success"},
		{"one channel sent", `Tool Output: {"results":[{"channel":"ntfy","status":"sent"},{"channel":"discord","status":"error","detail":"discord is in read-only mode"}],"status":"success"}`, "success"},
	}
	for _, tool := range []string{"send_notification", "send_telegram"} {
		for _, c := range cases {
			inv, _, _ := c13Invoker(&config.Config{}, map[string]bool{tool: true}, c13Answer(agent.ToolDispatchResult{Output: c.output, Status: agent.ToolResultSuccess}))
			resp, err := inv.InvokeTool(context.Background(), c13Request(tool, map[string]any{"message": "hi"}))
			if err != nil || resp.Status != c.want || resp.IsError != (c.want != "success") {
				t.Errorf("%s %s: %+v, %v; want %s", tool, c.name, resp, err, c.want)
				continue
			}
			if c.want == "success" && resp.Output != c.output {
				t.Errorf("%s %s: a success answer was rewritten: %s", tool, c.name, resp.Output)
			}
			if c.want != "success" {
				out, ne := flows.ParseToolOutput(resp.Output)
				if ne == nil || out["message"] == "" || out["results"] == nil {
					t.Errorf("%s %s: rewritten answer %s lacks the error, the message or the results", tool, c.name, resp.Output)
				}
			}
		}
	}
}

// A2: the codes the flow engine sees, through a curated node's Execute and callTool.
func TestC13CuratedNodesSeeNonRetryableCodes(t *testing.T) {
	reg := flows.NewRegistry()
	env := flows.StaticEnv{"home_assistant": {}, "mqtt_publish": {}}
	if err := flows.RegisterCatalog(reg, env); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		typ, tool string
		params    map[string]any
		output    string
		want      string
	}{
		{flows.TypeHomeAssistant, "home_assistant", map[string]any{"entity": "light.a", "service": "turn_on"},
			`Tool Output: {"status":"error","message":"Home Assistant is in read-only mode. Disable home_assistant.read_only to allow changes."}`, "FLOW_TOOL_DENIED"},
		{flows.TypeHomeAssistant, "home_assistant", map[string]any{"entity": "light.a", "service": "turn_on"},
			`Tool Output: {"status": "error", "message": "Home Assistant integration is not enabled. Set home_assistant.enabled=true in config.yaml."}`, "FLOW_NODE_UNAVAILABLE"},
		{flows.TypeMQTTPublish, "mqtt_publish", map[string]any{"topic": "a/b", "payload": "x"},
			`Tool Output: {"status":"error","message":"MQTT is in read-only mode. Disable mqtt.readonly to allow changes"}`, "FLOW_TOOL_DENIED"},
		{flows.TypeMQTTPublish, "mqtt_publish", map[string]any{"topic": "a/b", "payload": "x"},
			`Tool Output: {"status":"error","message":"MQTT publish failed: broker went away"}`, "FLOW_TOOL_ERROR"},
	}
	for _, c := range cases {
		def, ok := reg.Lookup(c.typ)
		if !ok {
			t.Fatalf("%s is not registered", c.typ)
		}
		inv, _, _ := c13Invoker(&config.Config{}, map[string]bool{c.tool: true}, c13Answer(agent.ToolDispatchResult{Output: c.output, Status: agent.ToolResultFailed, IsError: true}))
		_, err := def.Execute(context.Background(), flows.ExecInput{Params: c.params, Run: flows.RunInfo{ID: "run_c13", FlowID: "flow_aaaaaaaaaa"},
			Node: &flows.Node{ID: "n_aaaaaaaa", Type: c.typ, Params: c.params}, Services: &flows.Services{Tools: inv}})
		var ne *flows.NodeError
		if !errors.As(err, &ne) || ne.Code != c.want {
			t.Errorf("%s with %s: error %v, want %s", c.typ, c.output, err, c.want)
		}
	}
}

// A2: a call whose context ended without success returns the context's error; a success
// that arrives after the deadline stays a success.
func TestC13CancelledContextIsNoToolError(t *testing.T) {
	// The run is cancelled while the tool runs.
	ctx, cancel := context.WithCancel(context.Background())
	inv, _, _ := c13Invoker(&config.Config{}, map[string]bool{"send_telegram": true},
		func(context.Context, *agent.ToolCall, *agent.DispatchContext) agent.ToolDispatchResult {
			cancel()
			return agent.ToolDispatchResult{Output: `Tool Output: {"status":"error","message":"context canceled"}`, Status: agent.ToolResultCancelled, IsError: true}
		})
	if _, err := inv.InvokeTool(ctx, c13Request("send_telegram", map[string]any{"message": "hi"})); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled: got %v", err)
	}
	// The node's deadline passes while the tool runs; the tool reports a plain error.
	ctx, cancelDeadline := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancelDeadline()
	inv, _, _ = c13Invoker(&config.Config{}, map[string]bool{"send_telegram": true},
		func(ctx context.Context, _ *agent.ToolCall, _ *agent.DispatchContext) agent.ToolDispatchResult {
			<-ctx.Done()
			return agent.ToolDispatchResult{Output: `Tool Output: {"status":"error","message":"upload failed"}`, Status: agent.ToolResultFailed, IsError: true}
		})
	if _, err := inv.InvokeTool(ctx, c13Request("send_telegram", map[string]any{"message": "hi"})); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("deadline: got %v", err)
	}
	// A context that ended before the call refuses it without a dispatch.
	inv, _, calls := c13Invoker(&config.Config{}, map[string]bool{"send_telegram": true}, nil)
	if _, err := inv.InvokeTool(ctx, c13Request("send_telegram", map[string]any{"message": "hi"})); !errors.Is(err, context.DeadlineExceeded) || len(*calls) != 0 {
		t.Errorf("ended before: %v, dispatched %d", err, len(*calls))
	}
	ctx, cancel = context.WithCancel(context.Background())
	inv, _, _ = c13Invoker(&config.Config{}, map[string]bool{"send_email": true},
		func(context.Context, *agent.ToolCall, *agent.DispatchContext) agent.ToolDispatchResult {
			cancel()
			return agent.ToolDispatchResult{Output: `Tool Output: {"status":"success","message":"Email sent"}`, Status: agent.ToolResultSuccess}
		})
	resp, err := inv.InvokeTool(ctx, c13Request("send_email", map[string]any{"to": "a@b.c", "body": "x"}))
	if err != nil || resp.Status != "success" || resp.IsError {
		t.Fatalf("a mail sent before the cancel must stay sent: %+v, %v", resp, err)
	}
}

// A3: a flow's api_request to the configured local Ollama is refused by the SSRF check;
// the agent keeps its exception.
func TestC13FlowsGetNoLocalOllamaException(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "")
	var hits atomic.Int32
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer ollama.Close()
	cfg := &config.Config{}
	cfg.Agent.AllowNetworkRequests = true
	cfg.Ollama.URL = ollama.URL
	inv, s, _ := c13Invoker(cfg, map[string]bool{"api_request": true}, func(ctx context.Context, tc *agent.ToolCall, dc *agent.DispatchContext) agent.ToolDispatchResult {
		return agent.DispatchToolCallResult(ctx, tc, dc, "")
	})
	s.bindRuntimePermissions()
	t.Cleanup(func() { tools.SetRuntimePermissionResolver(nil) })

	for _, path := range []string{"/api/tags", "/api/delete", "/api/pull"} {
		resp, err := inv.InvokeTool(context.Background(), c13Request("api_request", map[string]any{"method": "POST", "url": ollama.URL + path}))
		if err != nil || resp.Status != "denied" || !resp.IsError || !strings.Contains(resp.Output, "URL validation failed") {
			t.Errorf("flow request to %s: %+v, %v", path, resp, err)
		}
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("a flow reached the local Ollama %d times", n)
	}

	// The agent's own dispatch (any other message source) keeps the exception.
	dc := inv.dispatchContext(cfg, "agent", "api_request")
	dc.MessageSource = "web_chat"
	tc := &agent.ToolCall{Action: "api_request", IsTool: true, Params: map[string]any{"method": "GET", "url": ollama.URL + "/api/tags"}}
	if res := agent.DispatchToolCallResult(context.Background(), tc, dc, ""); res.IsError || hits.Load() != 1 {
		t.Fatalf("the agent lost its local Ollama exception: %+v, hits %d", res, hits.Load())
	}
}

// A4/A5: flows dispatch with every summary mode off and without the preferred MCP web
// search; the server's configuration keeps its settings.
func TestC13SummaryModesAndMCPSearchAreOffForFlows(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.WebScraper.SummaryMode = true
	cfg.Tools.Wikipedia.SummaryMode = true
	cfg.Tools.DDGSearch.SummaryMode = true
	cfg.Tools.PDFExtractor.SummaryMode = true
	cfg.Tools.PDFExtractor.Enabled = true
	cfg.Agent.AllowMCP = true
	cfg.MCP.Enabled = true
	cfg.MCP.PreferredCapabilities.WebSearch = config.MCPPreferredToolSelection{Server: "search", Tool: "web_search"}
	cfg.MCP.Servers = []config.MCPServer{{Name: "search"}}
	names := map[string]bool{"web_scraper": true, "wikipedia_search": true, "ddg_search": true, "execute_skill": true}
	inv, _, calls := c13Invoker(cfg, names, nil)
	cfg.BraveSearch.Enabled, cfg.BraveSearch.APIKey = true, "brave-key"
	for _, tool := range []string{"web_scraper", "wikipedia_search", "ddg_search", flows.BraveSearchTool, flows.PDFExtractorTool} {
		if _, err := inv.InvokeTool(context.Background(), c13Request(tool, map[string]any{"query": "q"})); err != nil {
			t.Fatal(err)
		}
	}
	if len(*calls) != 5 {
		t.Fatalf("dispatched %d calls", len(*calls))
	}
	for _, c := range *calls {
		d := c.dc.Cfg
		if d == cfg || d.Tools.WebScraper.SummaryMode || d.Tools.Wikipedia.SummaryMode || d.Tools.DDGSearch.SummaryMode || d.Tools.PDFExtractor.SummaryMode {
			t.Fatalf("%s dispatched with a summary mode on", c.tc.Action)
		}
		if d.MCP.PreferredCapabilities.WebSearch != (config.MCPPreferredToolSelection{}) {
			t.Fatalf("%s dispatched with the preferred MCP web search", c.tc.Action)
		}
		// Everything else is the snapshot's.
		if !d.Tools.PDFExtractor.Enabled || !d.MCP.Enabled || len(d.MCP.Servers) != 1 || d.BraveSearch.APIKey != "brave-key" {
			t.Fatalf("%s dispatched with a changed configuration", c.tc.Action)
		}
		// The search tools ask CallPreferredMCPWebSearch, which now leaves the call to them.
		if _, used, _ := tools.CallPreferredMCPWebSearch(context.Background(), d, "q", 3, "", "", slog.New(slog.NewTextHandler(io.Discard, nil))); used {
			t.Fatalf("%s: the preferred MCP web search still answers flows", c.tc.Action)
		}
	}
	if !cfg.Tools.WebScraper.SummaryMode || !cfg.Tools.Wikipedia.SummaryMode || !cfg.Tools.DDGSearch.SummaryMode || !cfg.Tools.PDFExtractor.SummaryMode ||
		cfg.MCP.PreferredCapabilities.WebSearch.Server != "search" {
		t.Fatal("the server's configuration was changed")
	}
	if _, used, _ := tools.CallPreferredMCPWebSearch(context.Background(), cfg, "q", 3, "", "", slog.New(slog.NewTextHandler(io.Discard, nil))); !used {
		t.Fatal("control: the agent's configuration must still prefer the MCP web search")
	}
}

// A4 end to end: the curated web.read node with web_scraper's summary mode on reads the
// page through the real dispatcher and never calls the summary model.
func TestC13WebReadNodeSpendsNoSummaryTokens(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	var llmHits atomic.Int32
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		llmHits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"x","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"SUMMARY"},"finish_reason":"stop"}]}`))
	}))
	defer llm.Close()
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><head><title>C13 page</title></head><body><p>" + strings.Repeat("The flow reads this page in full. ", 40) + "</p></body></html>"))
	}))
	defer page.Close()

	cfg := &config.Config{}
	cfg.Agent.AllowNetworkRequests = true
	cfg.Tools.WebScraper.Enabled = true
	cfg.Tools.WebScraper.SummaryMode = true
	cfg.Tools.WebScraper.SummaryBaseURL = llm.URL
	cfg.Tools.WebScraper.SummaryAPIKey = "c13-summary-key"
	cfg.Tools.WebScraper.SummaryModel = "summary-model"
	cfg.Directories.WorkspaceDir = t.TempDir()
	inv, s, calls := c13Invoker(cfg, map[string]bool{"web_scraper": true}, func(ctx context.Context, tc *agent.ToolCall, dc *agent.DispatchContext) agent.ToolDispatchResult {
		return agent.DispatchToolCallResult(ctx, tc, dc, "")
	})
	s.bindRuntimePermissions()
	t.Cleanup(func() { tools.SetRuntimePermissionResolver(nil) })

	reg := flows.NewRegistry()
	if err := flows.RegisterCatalog(reg, flows.StaticEnv{"web_scraper": {}}); err != nil {
		t.Fatal(err)
	}
	def, ok := reg.Lookup(flows.TypeWebRead)
	if !ok {
		t.Fatal("web.read is not registered")
	}
	params := map[string]any{"url": page.URL, "mode": "static"}
	res, err := def.Execute(context.Background(), flows.ExecInput{Params: params, Run: flows.RunInfo{ID: "run_c13", FlowID: "flow_aaaaaaaaaa"},
		Node: &flows.Node{ID: "n_aaaaaaaa", Type: flows.TypeWebRead, Params: params}, Services: &flows.Services{Tools: inv}})
	if err != nil {
		t.Fatalf("web.read failed: %v", err)
	}
	if content, _ := res.Output["content"].(string); !strings.Contains(content, "reads this page in full") || strings.Contains(content, "SUMMARY") {
		t.Fatalf("web.read output = %.300v", res.Output)
	}
	if n := llmHits.Load(); n != 0 {
		t.Fatalf("the summary model was called %d times for a flow", n)
	}
	if len(*calls) != 1 || (*calls)[0].dc.Cfg.Tools.WebScraper.SummaryMode {
		t.Fatalf("calls = %d", len(*calls))
	}
}

// A6: document_creator always runs with block_remote_content, as a JSON bool.
func TestC13DocumentCreatorAlwaysBlocksRemoteContent(t *testing.T) {
	for _, given := range []any{nil, false, "false", true} {
		inv, _, calls := c13Invoker(&config.Config{}, map[string]bool{"document_creator": true}, nil)
		args := map[string]any{"operation": "html_to_pdf", "content": "<p>x</p>"}
		if given != nil {
			args["block_remote_content"] = given
		}
		if _, err := inv.InvokeTool(context.Background(), c13Request("document_creator", args)); err != nil {
			t.Fatal(err)
		}
		if v, ok := (*calls)[0].tc.Params["block_remote_content"].(bool); !ok || !v {
			t.Errorf("given %v: block_remote_content = %#v", given, (*calls)[0].tc.Params["block_remote_content"])
		}
		if given == nil {
			if _, set := args["block_remote_content"]; set {
				t.Error("the node's arguments were changed")
			}
		}
	}
}

// A7: services of the script, shell_command, python_script and hassio domains are refused
// for flows unless home_assistant.allowed_services lists them.
func TestC13HomeAssistantScriptDomainsAreDefaultDenied(t *testing.T) {
	refused := []map[string]any{
		{"operation": "call_service", "domain": "script", "service": "turn_on", "entity_id": "script.backup"},
		{"operation": "call_service", "domain": "shell_command", "service": "backup", "entity_id": "light.a"},
		{"operation": "call_service", "domain": "python_script", "service": "run", "entity_id": "light.a"},
		{"operation": "call_service", "domain": "hassio", "service": "host_reboot", "entity_id": "light.a"},
		{"operation": "service", "domain": "", "service": "Shell_Command.Backup", "entity_id": "light.a"},
		{"operation": "call_service", "domain": " SCRIPT ", "service": "script.turn_on", "entity_id": "script.a"},
	}
	for _, args := range refused {
		inv, _, calls := c13Invoker(&config.Config{}, map[string]bool{"home_assistant": true}, nil)
		resp, err := inv.InvokeTool(context.Background(), c13Request("home_assistant", args))
		if err != nil || resp.Status != "denied" || !resp.IsError || len(*calls) != 0 || !strings.Contains(resp.Output, "allowed_services") {
			t.Errorf("%v: %+v, %v, dispatched %d", args, resp, err, len(*calls))
		}
	}
	cfg := &config.Config{}
	cfg.HomeAssistant.AllowedServices = []string{"script.turn_on", " Shell_Command.backup "}
	allowed := []map[string]any{
		{"operation": "call_service", "domain": "script", "service": "turn_on", "entity_id": "script.backup"},
		{"operation": "call_service", "domain": "shell_command", "service": "backup", "entity_id": "light.a"},
		{"operation": "call_service", "domain": "light", "service": "turn_on", "entity_id": "light.a"},
		{"operation": "get_state", "entity_id": "script.backup"},
	}
	for _, args := range allowed {
		inv, _, calls := c13Invoker(cfg, map[string]bool{"home_assistant": true}, nil)
		resp, err := inv.InvokeTool(context.Background(), c13Request("home_assistant", args))
		if err != nil || resp.IsError || len(*calls) != 1 {
			t.Errorf("%v: %+v, %v, dispatched %d", args, resp, err, len(*calls))
		}
	}
	// Not listed: python_script.run stays refused although other services are listed.
	inv, _, calls := c13Invoker(cfg, map[string]bool{"home_assistant": true}, nil)
	if resp, _ := inv.InvokeTool(context.Background(), c13Request("home_assistant", refused[2])); resp.Status != "denied" || len(*calls) != 0 {
		t.Fatalf("python_script.run = %+v", resp)
	}
}
