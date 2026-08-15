package memory

import (
	"math"
	"testing"
	"time"
)

func TestDeriveMoodFromAffectBands(t *testing.T) {
	tests := []struct {
		name    string
		valence float64
		arousal float64
		want    Mood
	}{
		{name: "frustrated", valence: -0.5, arousal: 0.8, want: MoodFrustrated},
		{name: "cautious", valence: -0.3, arousal: 0.55, want: MoodCautious},
		{name: "concerned", valence: -0.25, arousal: 0.3, want: MoodConcerned},
		{name: "relaxed", valence: 0.4, arousal: 0.3, want: MoodRelaxed},
		{name: "playful", valence: 0.4, arousal: 0.7, want: MoodPlayful},
		{name: "focused", valence: 0.05, arousal: 0.6, want: MoodFocused},
		{name: "curious rest", valence: 0, arousal: 0.35, want: MoodCurious},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := DeriveMoodFromAffect(tc.valence, tc.arousal); got != tc.want {
				t.Fatalf("DeriveMoodFromAffect(%v, %v) = %q, want %q", tc.valence, tc.arousal, got, tc.want)
			}
		})
	}
}

func TestIntegrateAffectOpsIssueLowersValence(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	start := RestAffect(now)
	event, ok := affectEventByCause(AffectCauseOpsIssueOpened)
	if !ok {
		t.Fatal("missing ops_issue_opened catalog entry")
	}
	next := IntegrateAffect(start, event, now)
	if next.Valence >= start.Valence {
		t.Fatalf("valence = %.3f, want lower than rest %.3f", next.Valence, start.Valence)
	}
	if next.Arousal <= start.Arousal {
		t.Fatalf("arousal = %.3f, want higher than rest %.3f", next.Arousal, start.Arousal)
	}
	if next.CauseCode != AffectCauseOpsIssueOpened {
		t.Fatalf("cause = %q", next.CauseCode)
	}
}

func TestIntegrateAffectMixesOpposingEvents(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	opened, _ := affectEventByCause(AffectCauseOpsIssueOpened)
	positive, _ := affectEventByCause(AffectCausePositiveFeedback)
	afterIssue := IntegrateAffect(RestAffect(now), opened, now)
	afterPraise := IntegrateAffect(afterIssue, positive, now.Add(2*time.Minute))
	if afterPraise.Valence <= afterIssue.Valence {
		t.Fatalf("praise did not raise valence: before=%.3f after=%.3f", afterIssue.Valence, afterPraise.Valence)
	}
	if afterPraise.Valence >= positive.Valence {
		t.Fatalf("last-write-wins detected: valence=%.3f jumped to praise target %.3f", afterPraise.Valence, positive.Valence)
	}
}

func TestDecayAffectApproachesRestOverHalfLives(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	tense := AffectState{Valence: -0.8, Arousal: 0.9, Mood: MoodFrustrated, CauseCode: AffectCauseOpsIssueOpened, UpdatedAt: now}
	later := DecayAffect(tense, now.Add(24*time.Hour))
	if math.Abs(later.Valence-AffectRestValence) > 0.05 {
		t.Fatalf("valence after 24h = %.3f, want near rest", later.Valence)
	}
	if math.Abs(later.Arousal-AffectRestArousal) > 0.05 {
		t.Fatalf("arousal after 24h = %.3f, want near rest", later.Arousal)
	}
}

func TestUserReturnAfterLongAbsenceRaisesValenceFromDecayedNegative(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	opened, _ := affectEventByCause(AffectCauseOpsIssueOpened)
	afterIssue := IntegrateAffect(RestAffect(now), opened, now)
	later := now.Add(24 * time.Hour)
	decayed := DecayAffect(afterIssue, later)
	ret, _ := affectEventByCause(AffectCauseUserReturn)
	afterReturn := IntegrateAffect(afterIssue, ret, later)
	if afterReturn.Valence <= decayed.Valence {
		t.Fatalf("user return did not raise valence above decayed state: decayed=%.3f returned=%.3f", decayed.Valence, afterReturn.Valence)
	}
}

func TestAffectStateActiveRequiresCauseOrDeparture(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	if RestAffect(now).Active() {
		t.Fatal("rest affect should not be active")
	}
	driven := RestAffect(now)
	driven.CauseCode = AffectCauseOpsIssueOpened
	if !driven.Active() {
		t.Fatal("cause-coded affect should be active")
	}
}

func TestBindEmotionStateToAffectClampsLLMSuggestion(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	affect := AffectState{Valence: -0.4, Arousal: 0.6, Mood: MoodCautious, UpdatedAt: now}
	state := &EmotionState{Valence: 0.9, Arousal: 0.1, PrimaryMood: MoodPlayful, Description: "I feel wildly delighted about this."}
	BindEmotionStateToAffect(state, affect)
	if state.Valence > affect.Valence+AffectLLMValenceDelta+1e-9 {
		t.Fatalf("valence = %.3f, exceeds clamp around %.3f", state.Valence, affect.Valence)
	}
	if state.Arousal < affect.Arousal-AffectLLMArousalDelta-1e-9 {
		t.Fatalf("arousal = %.3f, exceeds clamp around %.3f", state.Arousal, affect.Arousal)
	}
	if state.PrimaryMood != MoodCautious {
		t.Fatalf("mood = %q, want affect mood", state.PrimaryMood)
	}
}

func TestApplyAffectEventPersistsAndLogsMood(t *testing.T) {
	stm := newTestPersonalityDB(t)
	now := time.Date(2026, 8, 15, 18, 0, 0, 0, time.UTC)
	event, _ := affectEventByCause(AffectCauseOpsIssueOpened)
	event.Source = "ops"
	event.Detail = "maintenance failed"
	got, err := stm.ApplyAffectEvent(event, now)
	if err != nil {
		t.Fatalf("ApplyAffectEvent: %v", err)
	}
	if got.Valence >= 0 {
		t.Fatalf("stored valence = %.3f, want negative", got.Valence)
	}
	loaded, err := stm.GetAffectStateAt(now)
	if err != nil {
		t.Fatalf("GetAffectStateAt: %v", err)
	}
	if math.Abs(loaded.Valence-got.Valence) > 0.001 {
		t.Fatalf("loaded valence = %.3f, want %.3f", loaded.Valence, got.Valence)
	}
	if stm.GetCurrentMood() != got.Mood {
		t.Fatalf("mood log = %q, want %q", stm.GetCurrentMood(), got.Mood)
	}
}

func TestListAffectEventsReturnsNewestFirst(t *testing.T) {
	stm := newTestPersonalityDB(t)
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	first, _ := affectEventByCause(AffectCauseOpsIssueOpened)
	first.Detail = "backup failed"
	if _, err := stm.ApplyAffectEvent(first, now); err != nil {
		t.Fatalf("first event: %v", err)
	}
	second, _ := affectEventByCause(AffectCausePositiveFeedback)
	second.Detail = "thanks"
	if _, err := stm.ApplyAffectEvent(second, now.Add(time.Minute)); err != nil {
		t.Fatalf("second event: %v", err)
	}
	events, err := stm.ListAffectEvents(10)
	if err != nil {
		t.Fatalf("ListAffectEvents: %v", err)
	}
	if len(events) < 2 {
		t.Fatalf("events = %#v, want at least 2", events)
	}
	if events[0].CauseCode != AffectCausePositiveFeedback {
		t.Fatalf("newest cause = %q", events[0].CauseCode)
	}
}

func TestAffectEventForTriggerRejectsEmpty(t *testing.T) {
	if _, ok := AffectEventForTrigger("", "", "chat"); ok {
		t.Fatal("empty trigger should not map")
	}
}
