package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/flows"
	"aurago/internal/security"
	"aurago/internal/tools"
)

// Tests from the 1c-13 quality review (items I1, I2 and I4).

// I2: text a remote server controls never picks a non-retryable status: transport errors
// that echo it, text after another subject, and answers the tool marked as third-party data.
func TestC13RemoteTextPicksNoStatus(t *testing.T) {
	cases := []struct {
		name, output string
		status       agent.ToolResultStatus
		want         string
	}{
		{"redirect location echoed by a transport error",
			`Tool Output: {"status":"error","message":"Request failed: Get \"http://blocked.invalid/x?m=Feature is not enabled here\": access to internal address is blocked (SSRF protection)"}`,
			agent.ToolResultFailed, "failed"},
		{"tool-marked external text", "Tool Output: <external_data>\nERROR remote says: this API is in read-only mode\n</external_data>", agent.ToolResultFailed, "failed"},
		{"refusal text after another subject", `Tool Output: {"status":"error","message":"ERROR remote says: Home Assistant is in read-only mode"}`, agent.ToolResultFailed, "failed"},
		{"broker error", `Tool Output: {"status":"error","message":"MQTT publish failed: broker says MQTT is in read-only mode"}`, agent.ToolResultFailed, "failed"},
		{"Home Assistant echo", `Tool Output: {"status":"error","message":"Home Assistant API error (HTTP 403): Home Assistant integration is not enabled"}`, agent.ToolResultFailed, "failed"},
		{"service call echo", `Tool Output: {"status":"error","message":"Service call failed: planner is disabled. x"}`, agent.ToolResultFailed, "failed"},
		{"notification channel echo", `Tool Output: {"results":[{"channel":"ntfy","status":"error","detail":"ntfy returned HTTP 403: ntfy topic is not configured"}],"status":"success"}`, agent.ToolResultSuccess, "success"},
	}
	for _, c := range cases {
		inv, _, _ := c13Invoker(&config.Config{}, map[string]bool{"send_notification": true}, c13Answer(agent.ToolDispatchResult{Output: c.output, Status: c.status, IsError: c.status.IsError()}))
		resp, err := inv.InvokeTool(context.Background(), c13Request("send_notification", map[string]any{"message": "x"}))
		if err != nil || resp.Status != c.want || resp.Output != c.output {
			t.Errorf("%s: %+v, %v; want %s", c.name, resp, err, c.want)
		}
	}
}

// I2: the server's Guardian wraps every answer once, AuraGo's own refusals included; that
// layer is presentation and the refusal still maps. A second layer is the tool's own marking
// of remote content and maps nothing.
func TestC13RefusalsMapThroughTheGuardiansWrapper(t *testing.T) {
	cfg := &config.Config{}
	cfg.HomeAssistant.Enabled = true
	cfg.HomeAssistant.ReadOnly = true
	inv, s, _ := c13Invoker(cfg, map[string]bool{"home_assistant": true, "mqtt_publish": true}, func(ctx context.Context, tc *agent.ToolCall, dc *agent.DispatchContext) agent.ToolDispatchResult {
		return agent.DispatchToolCallResult(ctx, tc, dc, "")
	})
	s.Guardian = security.NewGuardian(slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.initConfigSnapshot()
	s.bindRuntimePermissions()
	t.Cleanup(func() { tools.SetRuntimePermissionResolver(nil) })

	resp, err := inv.InvokeTool(context.Background(), c13Request("home_assistant", map[string]any{"operation": "call_service", "domain": "light", "service": "turn_on", "entity_id": "light.a"}))
	if err != nil || resp.Status != "denied" || !strings.Contains(resp.Output, "<external_data>") {
		t.Fatalf("Home Assistant read-only through the Guardian: %+v, %v", resp, err)
	}
	resp, err = inv.InvokeTool(context.Background(), c13Request("mqtt_publish", map[string]any{"topic": "a", "payload": "b"}))
	if err != nil || resp.Status != "needs_setup" {
		t.Fatalf("MQTT off through the Guardian: %+v, %v", resp, err)
	}

	double := "[Tool Output]\n" + security.IsolateExternalData("Tool Output: "+security.IsolateExternalData(`{"status":"error","message":"Home Assistant is in read-only mode"}`))
	inv, s, _ = c13Invoker(&config.Config{}, map[string]bool{"home_assistant": true}, c13Answer(agent.ToolDispatchResult{Output: double, Status: agent.ToolResultFailed, IsError: true}))
	s.Guardian = security.NewGuardian(slog.New(slog.NewTextHandler(io.Discard, nil)))
	resp, err = inv.InvokeTool(context.Background(), c13Request("home_assistant", map[string]any{"operation": "get_state", "entity_id": "light.a"}))
	if err != nil || resp.Status != "failed" {
		t.Fatalf("tool-marked remote text behind the Guardian: %+v, %v", resp, err)
	}
}

// I1: a .easydrag that is a symlink or junction is refused, and the sweep touches nothing
// it leads to.
func TestC13ScratchFolderMustBeAPlainFolder(t *testing.T) {
	links := map[string]func(link, target string) error{
		"symlink to .":        func(link, _ string) error { return os.Symlink(".", link) },
		"symlink to projects": func(link, _ string) error { return os.Symlink("projects", link) },
	}
	if runtime.GOOS == "windows" {
		links["junction to projects"] = func(link, target string) error {
			out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput()
			if err != nil {
				return &os.PathError{Op: "mklink", Path: link, Err: os.ErrPermission}
			}
			_ = out
			return nil
		}
	}
	for name, makeLink := range links {
		t.Run(name, func(t *testing.T) {
			cfg, workspace, docs := c13DocsConfig(t)
			if err := os.WriteFile(filepath.Join(docs, "report.pdf"), []byte("%PDF-c13"), 0o644); err != nil {
				t.Fatal(err)
			}
			important := filepath.Join(workspace, "projects", "important")
			if err := os.MkdirAll(important, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(important, "data.txt"), []byte("keep"), 0o644); err != nil {
				t.Fatal(err)
			}
			past := time.Now().Add(-48 * time.Hour)
			for _, dir := range []string{important, filepath.Join(workspace, "projects")} {
				if err := os.Chtimes(dir, past, past); err != nil {
					t.Fatal(err)
				}
			}
			if err := makeLink(filepath.Join(workspace, flowScratchDir), filepath.Join(workspace, "projects")); err != nil {
				t.Skipf("cannot create the link here: %v", err)
			}
			inv, _, calls := c13Invoker(cfg, map[string]bool{"execute_skill": true}, nil)
			resp, err := inv.InvokeTool(context.Background(), c13Request(flows.PDFExtractorTool, map[string]any{"filepath": "/files/documents/report.pdf"}))
			if err != nil || resp.Status != "denied" || len(*calls) != 0 || !strings.Contains(resp.Output, "not a plain folder") {
				t.Fatalf("%+v, %v, dispatched %d", resp, err, len(*calls))
			}
			if _, err := os.Stat(filepath.Join(important, "data.txt")); err != nil {
				t.Fatalf("the sweep removed a workspace folder: %v", err)
			}
			if entries, _ := os.ReadDir(filepath.Join(workspace, "projects")); len(entries) != 1 {
				t.Fatalf("something was written through the link: %v", entries)
			}
		})
	}
	// A file at .easydrag is refused too.
	cfg, workspace, docs := c13DocsConfig(t)
	if err := os.WriteFile(filepath.Join(docs, "report.pdf"), []byte("%PDF-c13"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, flowScratchDir), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	inv, _, calls := c13Invoker(cfg, map[string]bool{"execute_skill": true}, nil)
	if resp, _ := inv.InvokeTool(context.Background(), c13Request(flows.PDFExtractorTool, map[string]any{"filepath": "/files/documents/report.pdf"})); resp.Status != "denied" || len(*calls) != 0 {
		t.Fatalf("a file at .easydrag: %+v", resp)
	}
}

// M3: URLs in any value lose query and user info, header maps under any key are left out,
// and registered secrets are scrubbed.
func TestC13FlowLogHandlerCleansEveryValue(t *testing.T) {
	logs := &c13LogBuffer{}
	logger := flowDispatchLogger(slog.New(slog.NewTextHandler(logs, nil)))
	security.RegisterSensitive("c13-log-registered-secret")
	urlErr := &os.PathError{Op: "Get", Path: "https://api.example.com/v1?api_key=C13SECRET1", Err: os.ErrDeadlineExceeded}
	logger.Warn("fetch https://u:C13SECRET5@h.example/p?x=1 failed", "source", "https://cam.example.com/snap?token=C13SECRET2", "error", urlErr,
		"request_headers", map[string]string{"Authorization": "Bearer C13SECRET3"}, "note", "uses c13-log-registered-secret")
	logger.WithGroup("g").With("Upstream_Header", "C13SECRET6").Info("y", "anything", []string{"https://h/?k=C13SECRET4"})
	out := logs.String()
	for _, leak := range []string{"C13SECRET1", "C13SECRET2", "C13SECRET3", "C13SECRET4", "C13SECRET5", "C13SECRET6", "c13-log-registered-secret"} {
		if strings.Contains(out, leak) {
			t.Errorf("the log holds %s: %s", leak, out)
		}
	}
	for _, want := range []string{"https://cam.example.com/snap?[redacted]", "https://api.example.com/v1?[redacted]", "request_headers=[omitted]"} {
		if !strings.Contains(out, want) {
			t.Errorf("the log lacks %q: %s", want, out)
		}
	}
}

// M4: the Home Assistant default-deny reads the call as decodeHomeAssistantArgs does, so keys
// in another case do not slip past it.
func TestC13HomeAssistantDenyReadsCaseVariantKeys(t *testing.T) {
	for _, args := range []map[string]any{
		{"operation": "call_service", "Domain": "script", "service": "turn_on"},
		{"OPERATION": "call_service", "domain": "shell_command", "service": "backup"},
		{"Operation": "service", "DOMAIN": "Hassio", "Service": "host_reboot"},
	} {
		inv, _, calls := c13Invoker(&config.Config{}, map[string]bool{"home_assistant": true}, nil)
		resp, err := inv.InvokeTool(context.Background(), c13Request("home_assistant", args))
		if err != nil || resp.Status != "denied" || len(*calls) != 0 {
			t.Errorf("%v: %+v, %v, dispatched %d", args, resp, err, len(*calls))
		}
	}
}

// M6: an argument that does not fit its typed field is left out of the typed decode only;
// every other typed field is filled and Params keeps everything.
func TestC13PartialTypedDecode(t *testing.T) {
	inv, _, calls := c13Invoker(&config.Config{}, map[string]bool{"send_notification": true}, nil)
	if _, err := inv.InvokeTool(context.Background(), c13Request("send_notification", map[string]any{"message": "hello", "title": "T", "priority": "high", "channel": "ntfy"})); err != nil {
		t.Fatal(err)
	}
	tc := (*calls)[0].tc
	if tc.Message != "hello" || tc.Title != "T" || tc.Priority != 0 || tc.Params["priority"] != "high" || tc.Params["channel"] != "ntfy" {
		t.Fatalf("tool call = %+v", tc)
	}
}

// M7: the dispatcher's classifier: exit codes and pending answers.
func TestC13AgentClassifierCases(t *testing.T) {
	cases := []struct {
		output, want string
		isError      bool
	}{
		{`Tool Output: {"exit_code":2,"output":"boom"}`, "failed", true},
		{`Tool Output: {"exit_code":0,"output":"ok"}`, "success", false},
		{`Tool Output: {"status":"pending","message":"queued"}`, "deferred", false},
	}
	for _, c := range cases {
		inv, _, _ := c13Invoker(&config.Config{}, map[string]bool{"filesystem": true}, c13Answer(agent.ToolDispatchResult{Output: c.output}))
		resp, err := inv.InvokeTool(context.Background(), c13Request("filesystem", map[string]any{"operation": "stat", "file_path": "a"}))
		if err != nil || resp.Status != c.want || resp.IsError != c.isError {
			t.Errorf("%s: %+v, %v", c.output, resp, err)
		}
	}
}

// M7/M9: a served documents path with a query is bridged, and file_path that is empty gives
// way to path, as the filesystem decoder reads them.
func TestC13BridgeServedQueryAndEmptyFilePath(t *testing.T) {
	cfg, workspace, docs := c13DocsConfig(t)
	if err := os.WriteFile(filepath.Join(docs, "notes.txt"), []byte("c13 notes"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range []map[string]any{
		{"operation": "read_file", "file_path": "/files/documents/notes.txt?inline=1"},
		{"operation": "read_file", "file_path": "", "path": "/files/documents/notes.txt"},
	} {
		var seen []string
		key := "file_path"
		if args["path"] != nil {
			key = "path"
		}
		inv, _, calls := c13Invoker(cfg, map[string]bool{"filesystem": true}, c13CopyCheck(key, &seen))
		resp, err := inv.InvokeTool(context.Background(), c13Request("filesystem", args))
		if err != nil || resp.IsError || len(*calls) != 1 || !strings.HasPrefix(seen[0], filepath.Join(workspace, flowScratchDir)) || seen[1] != "c13 notes" {
			t.Errorf("%v: %+v, %v, seen %v", args, resp, err, seen)
		}
	}
}

// I4: the production binding. initConfigSnapshot binds AuthorizationSnapshots, the runtime
// gates follow the snapshot; the flow copy keeps its summary switch-off under the
// intersection, and a snapshot that revokes network access reaches the next flow call.
func TestC13ProductionConfigBinding(t *testing.T) {
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
		_, _ = w.Write([]byte("<html><body><p>" + strings.Repeat("Bound page text. ", 40) + "</p></body></html>"))
	}))
	defer page.Close()

	cfg := &config.Config{}
	cfg.Agent.AllowNetworkRequests = true
	cfg.Tools.WebScraper.Enabled = true
	cfg.Tools.WebScraper.SummaryMode = true
	cfg.Tools.WebScraper.SummaryBaseURL = llm.URL
	cfg.Tools.WebScraper.SummaryAPIKey = "c13-binding-summary-key"
	cfg.Tools.WebScraper.SummaryModel = "m"
	cfg.HomeAssistant.AllowedServices = []string{"light.turn_on"}
	cfg.Directories.WorkspaceDir = t.TempDir()
	inv, s, _ := c13Invoker(cfg, map[string]bool{"web_scraper": true, "api_request": true}, func(ctx context.Context, tc *agent.ToolCall, dc *agent.DispatchContext) agent.ToolDispatchResult {
		return agent.DispatchToolCallResult(ctx, tc, dc, "")
	})
	s.initConfigSnapshot()
	s.bindRuntimePermissions()
	t.Cleanup(func() { tools.SetRuntimePermissionResolver(nil) })

	resp, err := inv.InvokeTool(context.Background(), c13Request("web_scraper", map[string]any{"url": page.URL, "mode": "static"}))
	if err != nil || resp.IsError || resp.Status != "success" || !strings.Contains(resp.Output, "Bound page text") || llmHits.Load() != 0 {
		t.Fatalf("bound web_scraper: %+v, %v, model calls %d", resp, err, llmHits.Load())
	}
	next := *cfg
	next.Agent.AllowNetworkRequests = false
	s.replaceConfigSnapshot(&next)
	resp, err = inv.InvokeTool(context.Background(), c13Request("api_request", map[string]any{"url": page.URL, "method": "GET"}))
	if err != nil || resp.Status != "denied" || !resp.IsError {
		t.Fatalf("api_request after the revoke: %+v, %v", resp, err)
	}
}
