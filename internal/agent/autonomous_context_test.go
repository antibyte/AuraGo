package agent

import (
	"database/sql"
	"log/slog"
	"math"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/planner"
	"aurago/internal/prompts"
)

func TestIsAutonomousAgentRunRecognizesHeartbeat(t *testing.T) {
	tests := []struct {
		name      string
		runCfg    RunConfig
		sessionID string
		want      bool
	}{
		{name: "heartbeat source", runCfg: RunConfig{MessageSource: "heartbeat"}, sessionID: "default", want: true},
		{name: "heartbeat session", runCfg: RunConfig{}, sessionID: "heartbeat", want: true},
		{name: "planner notification source", runCfg: RunConfig{MessageSource: "planner_notification"}, sessionID: "default", want: true},
		{name: "uptime kuma source", runCfg: RunConfig{MessageSource: "uptime_kuma"}, sessionID: "default", want: true},
		{name: "follow up source", runCfg: RunConfig{MessageSource: "follow_up"}, sessionID: "default", want: true},
		{name: "cron source", runCfg: RunConfig{MessageSource: "cron"}, sessionID: "default", want: true},
		{name: "mqtt relay source", runCfg: RunConfig{MessageSource: "mqtt"}, sessionID: "mqtt", want: true},
		{name: "frigate relay source", runCfg: RunConfig{MessageSource: "frigate"}, sessionID: "frigate", want: true},
		{name: "web chat default", runCfg: RunConfig{MessageSource: "web_chat"}, sessionID: "default", want: false},
		{name: "mission is autonomous", runCfg: RunConfig{MessageSource: "mission", IsMission: true}, sessionID: "mission-1", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAutonomousAgentRun(tt.runCfg, tt.sessionID); got != tt.want {
				t.Fatalf("isAutonomousAgentRun(%+v, %q) = %v, want %v", tt.runCfg, tt.sessionID, got, tt.want)
			}
		})
	}
}

func TestShouldRunTurnSideEffectsBlocksHeartbeatArtifacts(t *testing.T) {
	if shouldRunTurnSideEffects(RunConfig{MessageSource: "heartbeat"}, "heartbeat", prompts.ContextFlags{}) {
		t.Fatal("heartbeat runs must not write activity, memory analysis, journal, or reuse artifacts")
	}
	if shouldRunTurnSideEffects(RunConfig{MessageSource: "planner_notification"}, "default", prompts.ContextFlags{}) {
		t.Fatal("planner notification runs must not write normal chat side effects")
	}
	if shouldRunTurnSideEffects(RunConfig{MessageSource: "uptime_kuma"}, "default", prompts.ContextFlags{}) {
		t.Fatal("uptime kuma runs must not write normal chat side effects")
	}
	if shouldRunTurnSideEffects(RunConfig{MessageSource: "follow_up"}, "default", prompts.ContextFlags{}) {
		t.Fatal("follow-up runs must not write normal chat side effects")
	}
	if shouldRunTurnSideEffects(RunConfig{MessageSource: "cron"}, "default", prompts.ContextFlags{}) {
		t.Fatal("cron runs must not write normal chat side effects")
	}
	if shouldRunTurnSideEffects(RunConfig{MessageSource: "mqtt"}, "mqtt", prompts.ContextFlags{}) {
		t.Fatal("MQTT relay runs must not write shared chat side effects")
	}
	if shouldRunTurnSideEffects(RunConfig{MessageSource: "frigate"}, "frigate", prompts.ContextFlags{}) {
		t.Fatal("Frigate relay runs must not write shared chat side effects")
	}
	if shouldRunTurnSideEffects(RunConfig{IsMission: true, MessageSource: "mission"}, "mission-1", prompts.ContextFlags{IsMission: true}) {
		t.Fatal("mission runs must not write normal chat side effects")
	}
	if !shouldRunTurnSideEffects(RunConfig{MessageSource: "web_chat"}, "default", prompts.ContextFlags{}) {
		t.Fatal("regular web chat should keep normal side effects")
	}
	if shouldRunTurnSideEffects(RunConfig{IsMaintenance: true, MessageSource: "maintenance"}, "maintenance", prompts.ContextFlags{}) {
		t.Fatal("maintenance runs must not write normal chat side effects")
	}
	if shouldRunTurnSideEffects(RunConfig{MessageSource: "maintenance"}, "maintenance", prompts.ContextFlags{}) {
		t.Fatal("maintenance runs must not write normal chat side effects")
	}
	if shouldRunTurnSideEffects(RunConfig{MessageSource: "sip", SuppressTurnSideEffects: true}, "sip-call-1", prompts.ContextFlags{}) {
		t.Fatal("privacy-sensitive transient sessions must not write normal chat side effects")
	}
	if !shouldRunTurnSideEffects(RunConfig{MessageSource: "sip"}, "sip-call-1", prompts.ContextFlags{}) {
		t.Fatal("SIP source alone must preserve the historical side-effect default")
	}
}

func TestRelayRunsSuppressDerivedSideEffectsButKeepOperationalIssueRecording(t *testing.T) {
	plannerDB := new(sql.DB)
	for _, source := range []string{"mqtt", "frigate"} {
		runCfg := RunConfig{
			MessageSource:           source,
			SessionID:               source,
			PlannerDB:               plannerDB,
			SuppressTurnSideEffects: true,
		}
		if shouldRunTurnSideEffects(runCfg, source, prompts.ContextFlags{}) {
			t.Fatalf("%s relay should suppress derived turn side effects", source)
		}
		if !shouldRecordOperationalIssueForRun(runCfg) {
			t.Fatalf("%s relay should retain operational issue recording", source)
		}
		if shouldConsiderOperationalIssueReminder(runCfg, "background failure") {
			t.Fatalf("%s relay should not deliver operational issue reminders", source)
		}
	}
}

func TestRelayOperationalIssueRecordsWithoutAffectSideEffect(t *testing.T) {
	plannerDB, err := planner.InitDB(filepath.Join(t.TempDir(), "planner.db"))
	if err != nil {
		t.Fatalf("planner.InitDB: %v", err)
	}
	defer plannerDB.Close()
	stm, err := memory.NewSQLiteMemory(":memory:", slog.Default())
	if err != nil {
		t.Fatalf("memory.NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	cfg := &config.Config{}
	cfg.Personality.Engine = true
	runCfg := RunConfig{
		Config:                  cfg,
		ShortTermMem:            stm,
		PlannerDB:               plannerDB,
		SessionID:               "mqtt",
		MessageSource:           "mqtt",
		SuppressTurnSideEffects: true,
	}
	before, err := stm.GetAffectState()
	if err != nil {
		t.Fatalf("initial affect state: %v", err)
	}
	recordOperationalIssue(runCfg, planner.OperationalIssue{
		Source:     "mqtt",
		Context:    "mqtt",
		Title:      "MQTT relay failed",
		Detail:     "broker unavailable",
		Severity:   "error",
		Reference:  "relay",
		OccurredAt: time.Now(),
	}, slog.Default())
	after, err := stm.GetAffectState()
	if err != nil {
		t.Fatalf("final affect state: %v", err)
	}
	if after.CauseCode != before.CauseCode || math.Abs(after.Valence-before.Valence) > 1e-9 || math.Abs(after.Arousal-before.Arousal) > 1e-9 {
		t.Fatalf("relay operational issue changed affect state: before=%+v after=%+v", before, after)
	}
	page, err := planner.ListOperationalIssues(plannerDB, planner.OperationalIssueListFilter{Status: "active", Source: "mqtt", Limit: 10})
	if err != nil {
		t.Fatalf("planner.ListOperationalIssues: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Title != "MQTT relay failed" {
		t.Fatalf("relay operational issue records = %+v, want one persisted issue", page.Items)
	}
}
