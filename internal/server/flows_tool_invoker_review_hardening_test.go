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
