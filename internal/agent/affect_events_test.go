package agent

import (
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/planner"
)

func TestEmitAffectFromTriggerLowersValenceOnOpsIssue(t *testing.T) {
	stm := newTestEmotionBehaviorMemory(t)
	cfg := &config.Config{}
	cfg.Personality.Engine = true

	emitAffectFromTrigger(stm, cfg, nil, memory.EmotionTriggerType(memory.AffectCauseOpsIssueOpened), "disk full", "ops")
	state, err := stm.GetAffectStateAt(time.Now())
	if err != nil {
		t.Fatalf("GetAffectStateAt: %v", err)
	}
	if state.Valence >= 0 {
		t.Fatalf("valence = %.3f, want negative after ops issue", state.Valence)
	}
	if state.CauseCode != memory.AffectCauseOpsIssueOpened {
		t.Fatalf("cause = %q", state.CauseCode)
	}
}

func TestSyncOperationalIssueAffectOpensAndResolves(t *testing.T) {
	stm := newTestEmotionBehaviorMemory(t)
	cfg := &config.Config{}
	cfg.Personality.Engine = true
	db := newPlannerTestDB(t)
	defer db.Close()

	syncOperationalIssueAffect(stm, cfg, db, nil)
	rest, _ := stm.GetAffectStateAt(time.Now())
	if rest.CauseCode == memory.AffectCauseOpsIssueOpened {
		t.Fatal("empty planner should not open an ops affect")
	}

	fingerprint, err := planner.RecordOperationalIssue(db, planner.OperationalIssue{
		Source:   "maintenance",
		Context:  "nightly",
		Title:    "Backup failed",
		Detail:   "rsync exit 1",
		Severity: "error",
		Kind:     planner.OperationalIssueKindRuntimeFailure,
	})
	if err != nil {
		t.Fatalf("RecordOperationalIssue: %v", err)
	}
	syncOperationalIssueAffect(stm, cfg, db, nil)
	opened, _ := stm.GetAffectStateAt(time.Now())
	if opened.CauseCode != memory.AffectCauseOpsIssueOpened {
		t.Fatalf("cause after open = %q", opened.CauseCode)
	}
	if opened.Valence >= rest.Valence {
		t.Fatalf("valence did not drop: before=%.3f after=%.3f", rest.Valence, opened.Valence)
	}

	if _, err := planner.ResolveOperationalIssue(db, fingerprint, "fixed", time.Now()); err != nil {
		t.Fatalf("ResolveOperationalIssue: %v", err)
	}
	syncOperationalIssueAffect(stm, cfg, db, nil)
	resolved, _ := stm.GetAffectStateAt(time.Now())
	if resolved.CauseCode != memory.AffectCauseOpsIssueResolved {
		t.Fatalf("cause after resolve = %q", resolved.CauseCode)
	}
	if resolved.Valence <= opened.Valence {
		t.Fatalf("valence did not recover: opened=%.3f resolved=%.3f", opened.Valence, resolved.Valence)
	}
}

func TestEmitAffectSkippedWhenPersonalityDisabled(t *testing.T) {
	stm := newTestEmotionBehaviorMemory(t)
	cfg := &config.Config{}
	emitAffectFromTrigger(stm, cfg, nil, memory.EmotionTriggerType(memory.AffectCauseOpsIssueOpened), "ignored", "ops")
	state, err := stm.GetAffectStateAt(time.Now())
	if err != nil {
		t.Fatalf("GetAffectStateAt: %v", err)
	}
	if state.CauseCode != "" {
		t.Fatalf("cause = %q, want empty when engine disabled", state.CauseCode)
	}
}
