package server

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/flows"
	"aurago/internal/security"
	"aurago/internal/tools"
)

// c13LogBuffer collects log output for assertions.
type c13LogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *c13LogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *c13LogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// c13Exec runs a curated node's Execute with the invoker.
func c13Exec(t *testing.T, typ string, env flows.StaticEnv, inv flows.ToolInvoker, params map[string]any, runID string) (flows.ExecResult, error) {
	t.Helper()
	reg := flows.NewRegistry()
	if err := flows.RegisterCatalog(reg, env); err != nil {
		t.Fatal(err)
	}
	def, ok := reg.Lookup(typ)
	if !ok {
		t.Fatalf("%s is not registered", typ)
	}
	return def.Execute(context.Background(), flows.ExecInput{Params: params, Run: flows.RunInfo{ID: runID, FlowID: "flow_aaaaaaaaaa"},
		Node: &flows.Node{ID: "n_aaaaaaaa", Type: typ, Params: params}, Services: &flows.Services{Tools: inv}})
}

// B8: an api_request of a flow logs neither its headers nor the query of its URL; every
// value the dispatcher logs is bounded.
func TestC13APIRequestArgumentsAreNotLogged(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer api.Close()
	cfg := &config.Config{}
	cfg.Agent.AllowNetworkRequests = true
	logs := &c13LogBuffer{}
	inv, s, _ := c13Invoker(cfg, map[string]bool{"api_request": true}, func(ctx context.Context, tc *agent.ToolCall, dc *agent.DispatchContext) agent.ToolDispatchResult {
		return agent.DispatchToolCallResult(ctx, tc, dc, "")
	})
	s.Logger = slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	s.bindRuntimePermissions()
	t.Cleanup(func() { tools.SetRuntimePermissionResolver(nil) })

	resp, err := inv.InvokeTool(context.Background(), c13Request("api_request", map[string]any{"method": "GET",
		"url": api.URL + "/v1/items?api_key=c13-query-secret&x=1", "headers": map[string]any{"X-Api-Key": "c13-header-secret"}}))
	if err != nil || resp.IsError {
		t.Fatalf("api_request = %+v, %v", resp, err)
	}
	out := logs.String()
	if !strings.Contains(out, "LLM requested generic API request") {
		t.Fatalf("the dispatcher's log line is missing, so the test checks nothing: %q", out)
	}
	for _, secret := range []string{"c13-query-secret", "c13-header-secret"} {
		if strings.Contains(out, secret) {
			t.Errorf("the log holds %s: %s", secret, out)
		}
	}
	if !strings.Contains(out, "/v1/items?[redacted]") {
		t.Errorf("the URL is not logged in its redacted form: %s", out)
	}
}

func TestC13FlowLogHandlerBoundsValues(t *testing.T) {
	logs := &c13LogBuffer{}
	logger := flowDispatchLogger(slog.New(slog.NewTextHandler(logs, nil)))
	long := strings.Repeat("x", 5000)
	logger.With("path", long).Info("op", "title", long, "url", "https://user:pw@example.com/a?token=t#f", "headers",
		map[string]string{"Authorization": "Bearer c13"}, "err", errors.New(long), slog.Group("g", "inner", long))
	out := logs.String()
	if strings.Contains(out, strings.Repeat("x", flowLogRunes+1)) {
		t.Errorf("a value is longer than %d runes: %.400s", flowLogRunes, out)
	}
	for _, leak := range []string{"pw@", "token=t", "#f", "Bearer c13"} {
		if strings.Contains(out, leak) {
			t.Errorf("the log holds %q: %s", leak, out)
		}
	}
	if !strings.Contains(out, "https://example.com/a?[redacted]") || !strings.Contains(out, "headers=[omitted]") {
		t.Errorf("log = %s", out)
	}
}

// B: send_notification's priority "normal" does not fit ToolCall.Priority (an int); the call
// still goes out with the priority in Params, as on the agent's invoke path.
func TestC13ArgumentsThatDoNotFitTypedFieldsStillDispatch(t *testing.T) {
	inv, _, calls := c13Invoker(&config.Config{}, map[string]bool{"send_notification": true},
		c13Answer(agent.ToolDispatchResult{Output: `Tool Output: {"results":[{"channel":"ntfy","status":"sent"}],"status":"success"}`, Status: agent.ToolResultSuccess}))
	res, err := c13Exec(t, flows.TypePush, flows.StaticEnv{"send_notification": {}}, inv,
		map[string]any{"channel": "ntfy", "message": "hello", "priority": "high"}, "run_c13")
	if err != nil || res.Output["sent"] != true {
		t.Fatalf("notify.push = %+v, %v", res, err)
	}
	c := (*calls)[0]
	if c.tc.Action != "send_notification" || c.tc.Params["priority"] != "high" || c.tc.Params["message"] != "hello" || c.tc.Params["channel"] != "ntfy" {
		t.Fatalf("tool call = %+v", c.tc)
	}
	// The PDF skill keeps its routing on that path too.
	inv, _, calls = c13Invoker(&config.Config{}, map[string]bool{"execute_skill": true}, nil)
	inv.s.Cfg.Tools.PDFExtractor.Enabled = true
	if _, err := inv.InvokeTool(context.Background(), c13Request(flows.PDFExtractorTool, map[string]any{"filepath": "a.pdf", "priority": "x", "skill": 3})); err != nil {
		t.Fatal(err)
	}
	if c := (*calls)[0]; c.tc.Action != "execute_skill" || c.tc.Skill != flows.PDFExtractorTool || c.tc.SkillArgs["filepath"] != "a.pdf" {
		t.Fatalf("skill call = %+v", c.tc)
	}
}

// B9: mqtt_publish gets qos as an int and retain as a bool.
func TestC13MQTTTypes(t *testing.T) {
	cases := []struct {
		qos, retain       any
		wantQoS           any
		wantRetain, isSet bool
	}{
		{1, true, 1, true, true},
		{float64(2), "true", 2, true, true},
		{"1", "false", 1, false, true},
		{int64(0), false, 0, false, true},
		{float64(1.5), "maybe", float64(1.5), false, false},
	}
	for _, c := range cases {
		inv, _, calls := c13Invoker(&config.Config{}, map[string]bool{"mqtt_publish": true}, nil)
		args := map[string]any{"topic": "a/b", "payload": "x", "qos": c.qos, "retain": c.retain}
		if _, err := inv.InvokeTool(context.Background(), c13Request("mqtt_publish", args)); err != nil {
			t.Fatal(err)
		}
		p := (*calls)[0].tc.Params
		if p["qos"] != c.wantQoS {
			t.Errorf("qos %#v reached the tool as %#v", c.qos, p["qos"])
		}
		if r, ok := p["retain"].(bool); c.isSet && (!ok || r != c.wantRetain) {
			t.Errorf("retain %#v reached the tool as %#v", c.retain, p["retain"])
		}
		if args["qos"] != c.qos || args["retain"] != c.retain {
			t.Error("the node's arguments were changed")
		}
	}
	// Through the curated node.
	inv, _, calls := c13Invoker(&config.Config{}, map[string]bool{"mqtt_publish": true}, nil)
	if _, err := c13Exec(t, flows.TypeMQTTPublish, flows.StaticEnv{"mqtt_publish": {}}, inv,
		map[string]any{"topic": "a/b", "payload": "on", "qos": "2", "retain": true}, "run_c13"); err != nil {
		t.Fatal(err)
	}
	p := (*calls)[0].tc.Params
	if q, ok := p["qos"].(int); !ok || q != 2 {
		t.Errorf("qos = %#v", p["qos"])
	}
	if r, ok := p["retain"].(bool); !ok || !r {
		t.Errorf("retain = %#v", p["retain"])
	}
}

// c13TextSentAnswer is what tools.SendTelegramFile answers when the text went out as a
// message and the document then failed.
const c13TextSentAnswer = `Tool Output: {"message":"the text was sent as a message, but the document was not: upload failed","results":[{"channel":"telegram","detail":"the text was sent as a message, but the document was not: upload failed","status":"error"}],"status":"error","text_sent":true}`

// B10: a Telegram text that went out while the document failed is never sent twice.
func TestC13TelegramTextIsNotSentTwice(t *testing.T) {
	inv, _, calls := c13Invoker(&config.Config{}, map[string]bool{"send_telegram": true},
		c13Answer(agent.ToolDispatchResult{Output: c13TextSentAnswer, Status: agent.ToolResultFailed, IsError: true}))
	params := map[string]any{"message": strings.Repeat("long text ", 200), "file": "report.pdf"}
	_, err := c13Exec(t, flows.TypeTelegram, flows.StaticEnv{"send_telegram": {}}, inv, params, "run_one")
	var ne *flows.NodeError
	if !errors.As(err, &ne) || ne.Code != "FLOW_TOOL_DENIED" || !strings.Contains(ne.Message, "the text was sent") {
		t.Fatalf("first attempt: %v", err)
	}
	// A retry of the same node in the same run is refused without a dispatch.
	_, err = c13Exec(t, flows.TypeTelegram, flows.StaticEnv{"send_telegram": {}}, inv, params, "run_one")
	if !errors.As(err, &ne) || ne.Code != "FLOW_TOOL_DENIED" || !strings.Contains(ne.Message, "not sent twice") || len(*calls) != 1 {
		t.Fatalf("retry: %v, dispatched %d", err, len(*calls))
	}
	// Another run sends again.
	if _, err := c13Exec(t, flows.TypeTelegram, flows.StaticEnv{"send_telegram": {}}, inv, params, "run_two"); err == nil || len(*calls) != 2 {
		t.Fatalf("another run: %v, dispatched %d", err, len(*calls))
	}

	// The engine retries a timed-out attempt whatever its code: the memory still holds.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	inv, _, calls = c13Invoker(&config.Config{}, map[string]bool{"send_telegram": true},
		func(ctx context.Context, _ *agent.ToolCall, _ *agent.DispatchContext) agent.ToolDispatchResult {
			<-ctx.Done()
			return agent.ToolDispatchResult{Output: c13TextSentAnswer, Status: agent.ToolResultCancelled, IsError: true}
		})
	req := c13Request("send_telegram", map[string]any{"message": "x", "file_path": "report.pdf"})
	if _, err := inv.InvokeTool(ctx, req); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timed-out attempt: %v", err)
	}
	resp, err := inv.InvokeTool(context.Background(), req)
	if err != nil || resp.Status != "denied" || len(*calls) != 1 {
		t.Fatalf("retry after the timeout: %+v, %v, dispatched %d", resp, err, len(*calls))
	}

	// A failure without text_sent is retried normally.
	inv, _, calls = c13Invoker(&config.Config{}, map[string]bool{"send_telegram": true},
		c13Answer(agent.ToolDispatchResult{Output: `Tool Output: {"message":"upload failed","results":[{"channel":"telegram","status":"error","detail":"upload failed"}],"status":"error"}`, Status: agent.ToolResultFailed, IsError: true}))
	for range 2 {
		if resp, err := inv.InvokeTool(context.Background(), req); err != nil || resp.Status != "failed" {
			t.Fatalf("plain failure: %+v, %v", resp, err)
		}
	}
	if len(*calls) != 2 {
		t.Fatalf("a plain failure was not retried: %d calls", len(*calls))
	}
}

func TestC13TextSentMemoryForgetsOldRuns(t *testing.T) {
	inv := &flowToolInvoker{}
	now := time.Now()
	inv.rememberTextSent("old", now.Add(-flowTextSentRetention-time.Minute))
	inv.rememberTextSent("new", now)
	if inv.textAlreadySent("old") || !inv.textAlreadySent("new") || inv.textAlreadySent("") {
		t.Fatalf("memory = %v", inv.textSent)
	}
	inv.rememberTextSent("", now)
	if len(inv.textSent) != 1 {
		t.Fatalf("an empty key was remembered: %v", inv.textSent)
	}
}

// B11: the secrets a call reads are registered with the scrubber and redacted from its
// output. Review M8: credentials are registered for good; the ntfy topic, a private value
// but no credential, only for the call. The values are unique to this test.
func TestC13CallSecretsAreRedacted(t *testing.T) {
	cfg := &config.Config{}
	cfg.Telegram.BotToken = "123456789:C13-telegram-bot-token"
	cfg.Notifications.Ntfy.Topic = "c13-private-ntfy-topic"
	cfg.Notifications.Ntfy.Token = "short"
	cfg.HomeAssistant.AccessToken = "c13-home-assistant-token"
	output := `Tool Output: {"status":"error","message":"POST https://api.telegram.org/bot123456789:C13-telegram-bot-token/sendDocument failed; ` +
		`https://ntfy.sh/c13-private-ntfy-topic unreachable; short stays"}`
	scrubbedDuringCall := false
	inv, _, _ := c13Invoker(cfg, map[string]bool{"send_notification": true, "home_assistant": true},
		func(context.Context, *agent.ToolCall, *agent.DispatchContext) agent.ToolDispatchResult {
			scrubbedDuringCall = !strings.Contains(security.Scrub(cfg.Notifications.Ntfy.Topic), cfg.Notifications.Ntfy.Topic)
			return agent.ToolDispatchResult{Output: output, Status: agent.ToolResultFailed, IsError: true}
		})
	resp, err := inv.InvokeTool(context.Background(), c13Request("send_notification", map[string]any{"message": "x"}))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{cfg.Telegram.BotToken, cfg.Notifications.Ntfy.Topic} {
		if strings.Contains(resp.Output, secret) {
			t.Errorf("the output holds %q: %s", secret, resp.Output)
		}
	}
	if strings.Contains(security.Scrub("x "+cfg.Telegram.BotToken+" y"), cfg.Telegram.BotToken) {
		t.Error("the bot token is not registered with the global scrubber")
	}
	if !scrubbedDuringCall {
		t.Error("the ntfy topic was not scrubbed while the tool ran")
	}
	if !strings.Contains(security.Scrub("x "+cfg.Notifications.Ntfy.Topic+" y"), cfg.Notifications.Ntfy.Topic) {
		t.Error("the ntfy topic stayed in the process-wide scrubber after the call")
	}
	if !strings.Contains(resp.Output, "short stays") {
		t.Errorf("a value under %d bytes was redacted: %s", flowSecretMinBytes, resp.Output)
	}
	// A Home Assistant call registers the Home Assistant token.
	if _, err := inv.InvokeTool(context.Background(), c13Request("home_assistant", map[string]any{"operation": "get_state", "entity_id": "light.a"})); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(security.Scrub(cfg.HomeAssistant.AccessToken), cfg.HomeAssistant.AccessToken) {
		t.Error("the Home Assistant token is not registered")
	}
}
