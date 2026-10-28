package agent

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"
)

func gameMakerPhaseToolNames(stage string) []string {
	var names []string
	for _, definition := range GameMakerPhaseToolSchemas(stage, "2d") {
		names = append(names, definition.Function.Name)
	}
	return names
}

// announcementState is a mid-task state whose action-promise reply triggers the
// announcement recovery (see TestActionPromiseWithoutToolTriggersLedgerAwareRecovery).
func announcementState(t *testing.T, native bool, allowed []string) *agentLoopState {
	t.Helper()
	cfg := &config.Config{}
	cfg.Agent.AnnouncementDetector.Enabled = true
	cfg.Agent.AnnouncementDetector.MaxRetries = 2
	logger := slog.New(slog.NewTextHandler(testDiscardWriter{}, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	return &agentLoopState{
		ctx:                context.Background(),
		broker:             NoopBroker{},
		currentLogger:      logger,
		useNativeFunctions: native,
		lastUserMsg:        "ja, prüfe docker jetzt",
		recoverySession:    NewRecoverySessionState(logger, NoopBroker{}, cfg),
		runCfg: RunConfig{
			Config: cfg, SessionID: "sess-nudge", MessageSource: "web_chat",
			ShortTermMem: stm, HistoryManager: memory.NewEphemeralHistoryManager(), AllowedTools: allowed,
		},
	}
}

// recoveryNudge runs one recovery round for a prose-only action promise and
// returns the user message it injected.
func recoveryNudge(t *testing.T, s *agentLoopState) string {
	t.Helper()
	content := "Ich prüfe Docker jetzt und melde mich gleich."
	parsed := ParsedToolResponse{Content: content, SanitizedContent: content}
	if _, _, shouldContinue, _ := handleAgentLoopRecoveries(s, content, ToolCall{}, parsed, true, emotionBehaviorPolicy{}); !shouldContinue || len(s.req.Messages) == 0 {
		t.Fatal("the prose-only reply was not nudged")
	}
	return s.req.Messages[len(s.req.Messages)-1].Content
}

func TestGameMakerAnnouncementNudgeIsStudioSpecific(t *testing.T) {
	planning, building := gameMakerPhaseToolNames("planning"), gameMakerPhaseToolNames("building")
	for _, test := range []struct {
		name    string
		native  bool
		allowed []string
		want    []string
		absent  []string
	}{
		{"text planning", false, planning, []string{
			"Your last reply contained no Game Maker tool call.", "game_maker_asset, game_maker_file, game_maker_project",
			"Pass the operation (e.g. set_design, read, replace) in the operation argument of its tool",
			`{"action":"game_maker_project","operation":"set_design","design":{...}}`,
			"Planning ends only when a design is accepted",
		}, []string{"game_maker_validate", "one short sentence", "native"}},
		{"text building", false, building, []string{
			"game_maker_asset, game_maker_file, game_maker_project, game_maker_validate",
			`{"action":"game_maker_file","operation":"read","path":"src/main.ts"}`,
			"If this phase's work is done, reply with one short sentence; the server then checks the saved files.",
		}, []string{"Planning ends", "native"}},
		{"native planning", true, planning, []string{
			"Your last reply contained no Game Maker tool call.", "game_maker_asset, game_maker_file, game_maker_project",
			"in the operation argument of its tool; use the native function-calling mechanism.",
		}, []string{`{"action"`, "raw JSON"}},
		// The Studio adds code_graph to its scope when project knowledge is enabled.
		{"text building with code_graph", false, append([]string{"code_graph"}, building...), []string{
			"code_graph, game_maker_asset, game_maker_file, game_maker_project, game_maker_validate",
			`{"action":"game_maker_file","operation":"read","path":"src/main.ts"}`,
			"If this phase's work is done, reply with one short sentence; the server then checks the saved files.",
		}, []string{"Planning ends", "native"}},
		{"native planning with code_graph", true, append([]string{"code_graph"}, planning...), []string{
			"Your last reply contained no Game Maker tool call.", "code_graph, game_maker_asset, game_maker_file, game_maker_project",
			"in the operation argument of its tool; use the native function-calling mechanism.",
		}, []string{`{"action"`, "raw JSON"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := announcementState(t, test.native, test.allowed)
			s.recentTools = []string{"game_maker_project"}
			got := recoveryNudge(t, s)
			if got != formatGameMakerAnnouncementFeedback(test.native, test.allowed) {
				t.Fatalf("the Studio run did not get the Studio nudge: %q", got)
			}
			for _, want := range test.want {
				if !strings.Contains(got, want) {
					t.Errorf("nudge lacks %q: %s", want, got)
				}
			}
			for _, absent := range append(test.absent, "<done/>", "discover_tools", "file_editor", "already ran") {
				if strings.Contains(got, absent) {
					t.Errorf("nudge must not contain %q: %s", absent, got)
				}
			}
		})
	}
}

// The text-mode example in the nudge must be a call the text parser accepts as
// written: "action" is the canonical key and top-level fields become arguments.
func TestGameMakerAnnouncementNudgeExampleParses(t *testing.T) {
	building := formatGameMakerAnnouncementFeedback(false, gameMakerPhaseToolNames("building"))
	example := building[strings.Index(building, `{"action"`) : strings.LastIndex(building, "}")+1]
	if tc := ParseToolCall(example); tc.Action != "game_maker_file" || tc.Operation != "read" || tc.Params["path"] != "src/main.ts" {
		t.Fatalf("building example is not parsed as a call: %+v", tc)
	}
	planning := formatGameMakerAnnouncementFeedback(false, gameMakerPhaseToolNames("planning"))
	example = strings.Replace(planning[strings.Index(planning, `{"action"`):strings.Index(planning, "}}")+2], "{...}", `{"base":"shooter"}`, 1)
	tc := ParseToolCall(example)
	if design, _ := tc.Params["design"].(map[string]interface{}); tc.Action != "game_maker_project" || tc.Operation != "set_design" || design["base"] != "shooter" {
		t.Fatalf("planning example is not parsed as a call: %+v", tc)
	}
}

// The main agent (no scope) and mixed scopes keep the generic nudge.
func TestGameMakerNudgesLeaveOtherRunsUntouched(t *testing.T) {
	scopes := map[string][]string{
		"main agent":       nil,
		"empty scope":      {},
		"mixed scope":      {"game_maker_file", "execute_shell"},
		"other tools":      {"docker", "filesystem"},
		"code graph only":  {"code_graph"},
		"code graph mixed": {"code_graph", "game_maker_file", "execute_shell"},
	}
	for name, allowed := range scopes {
		for _, native := range []bool{false, true} {
			s := announcementState(t, native, allowed)
			s.recentTools = []string{"docker"}
			got := recoveryNudge(t, s)
			if want := FormatAnnouncementFeedback(native, []string{"docker"}); got != want || !strings.Contains(got, "<done/>") || !strings.Contains(got, "'docker' already ran") {
				t.Fatalf("%s native=%v: generic nudge changed: %q", name, native, got)
			}
		}
	}
	// A pre-existing literal pins the exact text.
	s := announcementState(t, false, nil)
	if got := recoveryNudge(t, s); got != "ERROR: Your last response was text-only — it contained neither a tool call (raw JSON starting with {) nor the <done/> completion signal. If you want to call a tool, output the raw JSON object NOW (starting with {). Append <done/> only if the requested work is fully complete and no more tool calls are needed." {
		t.Fatalf("generic text-mode nudge changed: %q", got)
	}
}

// The XML-format note names discover_tools and file_editor, tools a Studio run
// does not have; it must not send the model to them.
func TestGameMakerXMLFallbackNoteStaysInsideStudio(t *testing.T) {
	long := "<tool_call><function=game_maker_file>" + strings.Repeat("x", 600) + "</function></tool_call>"
	tc := ToolCall{Action: "game_maker_file", IsTool: true, XMLFallbackDetected: true}
	building := gameMakerPhaseToolNames("building")
	for name, test := range map[string]struct {
		allowed []string
		native  bool
	}{
		"studio text": {building, false}, "studio native": {building, true}, "studio code graph": {append([]string{"code_graph"}, building...), false},
		"main agent": {nil, false}, "mixed scope": {append([]string{"execute_shell"}, building...), false},
	} {
		s := announcementState(t, test.native, test.allowed)
		if _, _, _, handled := handleAgentLoopRecoveries(s, long, tc, ParsedToolResponse{}, false, emotionBehaviorPolicy{}); !handled || len(s.req.Messages) == 0 {
			t.Fatalf("%s: XML fallback was not handled", name)
		}
		note := s.req.Messages[len(s.req.Messages)-1].Content
		studio := strings.HasPrefix(name, "studio")
		if strings.Contains(note, "discover_tools") == studio || strings.Contains(note, "file_editor") == studio || !strings.Contains(note, "do NOT repeat it") {
			t.Fatalf("%s: unexpected XML fallback note: %q", name, note)
		}
	}
}

// Through the real loop (no loop-state ctx needed): a text-mode Studio planning run whose model first
// answers with prose is nudged with the Studio text, then submits its design.
func TestGameMakerTextOnlyReplyGetsStudioNudgeInTheLoop(t *testing.T) {
	fixture := loadTextModeReplayFixture(t)
	glm := fixture.Models[1]
	prose := "I will look at the planning tools first:\n- get_plan\n- set_design"
	result := replayGameMakerPlanning(t, glm.Dimension, []string{prose, glm.Replies["set_design"]}, func(run *RunConfig) {
		run.Config.Agent.AnnouncementDetector.Enabled, run.Config.Agent.AnnouncementDetector.MaxRetries = true, 2
	})
	requireTextModeWithoutDenial(t, result)
	want := formatGameMakerAnnouncementFeedback(false, gameMakerPhaseToolNames("planning"))
	if !strings.Contains(result.shown, want) || strings.Contains(result.shown, "<done/>") || !result.accepted {
		t.Fatalf("accepted=%v; the model did not see the Studio nudge:\n%s", result.accepted, result.shown)
	}
}
