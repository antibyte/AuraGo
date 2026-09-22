package memory

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func observePersonality(t *testing.T, stm *SQLiteMemory, observation PersonalityObservation) PersonalitySnapshot {
	t.Helper()
	snapshot, err := stm.ApplyPersonalityObservation(observation)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestPersonalityDynamicsReadDoesNotBypassHysteresis(t *testing.T) {
	stm := newTestPersonalityDB(t)
	now := time.Now().UTC()
	event := AffectEvent{CauseCode: AffectCauseToolErrorStreak, Valence: -1, Arousal: 1, Weight: .6}
	first := observePersonality(t, stm, PersonalityObservation{ID: "first", Source: "tool", At: now, Event: &event})
	for _, elapsed := range []time.Duration{time.Second, time.Minute} {
		read, err := stm.GetPersonalitySnapshotAt(now.Add(elapsed))
		if err != nil {
			t.Fatal(err)
		}
		affect, err := stm.GetAffectStateAt(now.Add(elapsed))
		if err != nil {
			t.Fatal(err)
		}
		if read.Affect.Mood != first.Affect.Mood || affect.Mood != first.Affect.Mood {
			t.Fatal("read bypassed mood confirmation")
		}
	}
	creative := observePersonality(t, stm, PersonalityObservation{ID: "work", At: now.Add(time.Second), Mood: MoodCreative})
	if creative.Affect.Mood != MoodCreative || creative.Affect.Valence >= 0 {
		t.Fatal("working mode lost independence from negative affect")
	}
}

func TestPersonalityDynamicsSemanticRelationshipHabituates(t *testing.T) {
	stm := newTestPersonalityDB(t)
	now := time.Now()
	event, _ := affectEventByCause(AffectCauseConversation)
	var firstGain, lastGain float64
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("turn-%d", i)
		primary := observePersonality(t, stm, PersonalityObservation{ID: id, Source: "chat", Human: true, BeginTurn: true, At: now, Event: &event})
		after := observePersonality(t, stm, PersonalityObservation{ID: id, Semantic: true, Basis: &primary, Source: "helper", Human: true, Signal: "criticism", Target: "agent", Confidence: 1, At: now})
		gain := (after.Dynamics.Friction - primary.Dynamics.Friction) / (1 - primary.Dynamics.Friction)
		if i == 0 {
			firstGain = gain
		}
		lastGain = gain
	}
	if lastGain >= firstGain*.5 {
		t.Fatalf("semantic repeats did not habituate: %f -> %f", firstGain, lastGain)
	}
}

func TestPersonalityDynamicsSnapshotInvalidatesCachedNarration(t *testing.T) {
	stm := newTestPersonalityDB(t)
	if _, err := stm.ApplyPersonalityObservation(PersonalityObservation{Emotion: &EmotionState{Description: "Earlier emotion", Confidence: .8}, Semantic: true}); err != nil {
		t.Fatal(err)
	}
	es := NewEmotionSynthesizer(nil, "", 60, 100, "English", slog.Default())
	if err := es.BindMemory(stm); err != nil {
		t.Fatal(err)
	}
	if es.GetLastEmotion() == nil {
		t.Fatal("expected persisted narration")
	}
	if _, err := stm.ResetPersonalityDynamics(time.Now()); err != nil {
		t.Fatal(err)
	}
	if es.GetLastEmotion() != nil {
		t.Fatal("reset left cached narration active")
	}
}

func TestPersonalityDynamicsSeparatesFamiliarityFrictionAndOperationalLoad(t *testing.T) {
	stm := newTestPersonalityDB(t)
	if err := stm.SetTrait(TraitAffinity, 0.85); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	criticism, _ := affectEventByCause(AffectCauseNegativeFeedback)
	observation := PersonalityObservation{ID: "human-1", Source: "feedback", Target: "agent", Human: true, Explicit: true, Signal: "criticism", Confidence: 1, At: now, Event: &criticism, AffinityDelta: -0.1}
	first := observePersonality(t, stm, observation)
	if first.Dynamics.Friction <= 0 || first.Dynamics.Familiarity < 0.8 {
		t.Fatalf("criticism destroyed established familiarity: %+v", first.Dynamics)
	}
	op, _ := affectEventByCause(AffectCauseToolErrorStreak)
	for i := 0; i < 8; i++ {
		observePersonality(t, stm, PersonalityObservation{ID: fmt.Sprint("tool-", i), Source: "tool", Target: "agent", Signal: "criticism", Confidence: 1, AffinityDelta: -1, Event: &op, At: now})
	}
	after, err := stm.GetPersonalitySnapshotAt(now)
	if err != nil {
		t.Fatal(err)
	}
	if after.Dynamics.Load <= first.Dynamics.Load || after.Dynamics.Familiarity != first.Dynamics.Familiarity || after.Dynamics.Friction != first.Dynamics.Friction {
		t.Fatalf("technical errors changed the relationship: before=%+v after=%+v", first.Dynamics, after.Dynamics)
	}
	positive, _ := affectEventByCause(AffectCausePositiveFeedback)
	repair := PersonalityObservation{ID: "human-2", Source: "feedback", Target: "agent", Human: true, Signal: "repair", Confidence: 1, At: now, Event: &positive, AffinityDelta: 0.03}
	healed := observePersonality(t, stm, repair)
	if healed.Dynamics.Friction >= after.Dynamics.Friction || healed.Dynamics.Friction == 0 || healed.Dynamics.Recoveries != 1 {
		t.Fatalf("repair must be confirmed and gradual: %+v", healed.Dynamics)
	}
	duplicate := observePersonality(t, stm, repair)
	if !reflect.DeepEqual(duplicate, healed) {
		t.Fatal("duplicate event changed the state")
	}
}

func TestPersonalityDynamicsRequiresConfidentHumanRelationalEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, target string
		human        bool
		confidence   float64
	}{
		{"uncertain sarcasm", "agent", true, 0.5}, {"angry about situation", "task", true, 1}, {"tool text", "agent", false, 1}, {"invalid confidence", "agent", true, math.NaN()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stm := newTestPersonalityDB(t)
			got := observePersonality(t, stm, PersonalityObservation{Source: "chat", Target: tc.target, Human: tc.human, Confidence: tc.confidence, Signal: "criticism", AffinityDelta: -1})
			if got.Dynamics.Friction != 0 || got.Dynamics.Familiarity != 0.5 {
				t.Fatalf("unreliable observation affected relationship: %+v", got.Dynamics)
			}
		})
	}
}

func TestPersonalityDynamicsHysteresisAndWorkingModes(t *testing.T) {
	stm := newTestPersonalityDB(t)
	event := AffectEvent{CauseCode: AffectCausePositiveFeedback, Valence: 1, Arousal: 0.2, Weight: 0.6}
	now := time.Now()
	first := observePersonality(t, stm, PersonalityObservation{ID: "a", At: now, Event: &event})
	if first.Affect.Mood != MoodCurious {
		t.Fatalf("one weak event changed mood: %s", first.Affect.Mood)
	}
	second := observePersonality(t, stm, PersonalityObservation{ID: "b", At: now.Add(time.Second), Event: &event})
	if second.Affect.Mood != MoodRelaxed {
		t.Fatalf("two confirming events did not change mood: %s", second.Affect.Mood)
	}
	creative := observePersonality(t, stm, PersonalityObservation{ID: "c", At: now.Add(2 * time.Second), Mood: MoodCreative})
	if creative.Affect.Mood != MoodCreative {
		t.Fatal("hysteresis blocked an explicit working mode")
	}
	strong := observePersonality(t, stm, PersonalityObservation{ID: "d", At: now.Add(3 * time.Second), Mood: MoodCautious, Explicit: true})
	if strong.Affect.Mood != MoodCautious {
		t.Fatal("strong explicit feedback was delayed")
	}
}

func TestPersonalityDynamicsHabituationRecoversAndDistinguishesProblems(t *testing.T) {
	now := time.Now()
	state := newPersonalityDynamics()
	event := AffectEvent{CauseCode: AffectCauseOpsIssueOpened, Detail: "storage"}
	observation := PersonalityObservation{Source: "ops", Event: &event}
	first := adaptivePersonalityWeight(&state, observation, now)
	last := first
	for i := 0; i < 20; i++ {
		last = adaptivePersonalityWeight(&state, observation, now.Add(time.Duration(i)*time.Second))
	}
	if last >= first/2 {
		t.Fatalf("repeated stimuli failed to habituate: %f -> %f", first, last)
	}
	event.Detail = "network"
	independent := adaptivePersonalityWeight(&state, observation, now.Add(time.Minute))
	if independent < 0.99 {
		t.Fatal("an independent problem inherited habituation")
	}
	event.Detail = "storage"
	recovered := adaptivePersonalityWeight(&state, observation, now.Add(24*time.Hour))
	if recovered < 0.95 {
		t.Fatalf("sensitivity did not recover: %f", recovered)
	}
	for i := 0; i < 100; i++ {
		event.Detail = fmt.Sprint(i)
		adaptivePersonalityWeight(&state, observation, now.Add(time.Duration(i)*time.Minute))
	}
	if len(state.Stimuli) > personalityMaxStimuli {
		t.Fatal("unbounded stimulus ledger")
	}
}

func TestPersonalityDynamicsReadsDecayWithoutMutationAndHandleClockReversal(t *testing.T) {
	stm := newTestPersonalityDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	event, _ := affectEventByCause(AffectCauseNegativeFeedback)
	first := observePersonality(t, stm, PersonalityObservation{ID: "a", Source: "feedback", Target: "agent", Human: true, Signal: "criticism", Confidence: 1, At: now, Event: &event})
	later, err := stm.GetPersonalitySnapshotAt(now.Add(12 * time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(later.Dynamics.Load-first.Dynamics.Load/2) > 1e-8 || math.Abs(later.Dynamics.Friction-first.Dynamics.Friction/math.Sqrt(2)) > 1e-8 {
		t.Fatal("incorrect independent time scales")
	}
	again, err := stm.GetPersonalitySnapshotAt(now)
	if err != nil || !reflect.DeepEqual(first, again) {
		t.Fatalf("read changed persisted state: %v", err)
	}
	reversed := observePersonality(t, stm, PersonalityObservation{ID: "b", At: now.Add(-time.Hour), Event: &event})
	if reversed.Dynamics.UpdatedAt.Before(now) {
		t.Fatal("clock reversal moved the state backwards")
	}
}

func TestPersonalityDynamicsZeroVolatilityRemainsZero(t *testing.T) {
	stm := newTestPersonalityDB(t)
	meta := DefaultPersonalityMeta()
	meta.Volatility = 0
	event, _ := affectEventByCause(AffectCauseNegativeFeedback)
	got := observePersonality(t, stm, PersonalityObservation{Source: "feedback", Target: "agent", Human: true, Signal: "criticism", Confidence: 1, Event: &event, Meta: &meta, AffinityDelta: -0.1})
	if got.Affect.Valence != 0 || got.Affect.Arousal != AffectRestArousal || got.Dynamics.Load != 0 || got.Dynamics.Friction != 0 || got.Dynamics.Familiarity != 0.5 {
		t.Fatalf("zero volatility was lost: %+v", got)
	}
}

func TestPersonalityDynamicsSemanticEnrichmentDoesNotRepeatPrimaryEvent(t *testing.T) {
	stm := newTestPersonalityDB(t)
	now := time.Now()
	event, _ := affectEventByCause(AffectCauseNegativeFeedback)
	first := observePersonality(t, stm, PersonalityObservation{ID: "turn", Source: "chat", Target: "agent", Human: true, BeginTurn: true, Signal: "criticism", Confidence: 1, At: now, Event: &event, AffinityDelta: -0.01})
	semantic := PersonalityObservation{ID: "turn", Source: "helper", Target: "agent", Human: true, Signal: "criticism", Confidence: 1, At: now, Event: &event, AffinityDelta: -0.1, Semantic: true, Basis: &first, Emotion: &EmotionState{Description: "Working calmly through this setback.", PrimaryMood: MoodCautious, Valence: -1, Arousal: 1}}
	second := observePersonality(t, stm, semantic)
	if second.Dynamics.Load != first.Dynamics.Load || second.Dynamics.Friction != first.Dynamics.Friction || second.Dynamics.Familiarity != first.Dynamics.Familiarity {
		t.Fatal("enrichment counted the same event twice")
	}
	if math.Abs(second.Affect.Valence-first.Affect.Valence) > AffectLLMValenceDelta+1e-9 {
		t.Fatal("semantic affect exceeded its numeric clamp")
	}
	var count int
	if err := stm.db.QueryRow(`SELECT count(*) FROM affect_events`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate primary timeline event: %d, %v", count, err)
	}
}

func TestPersonalityDynamicsRejectsSupersededResetAndPersonaResults(t *testing.T) {
	for _, change := range []string{"turn", "reset", "persona"} {
		t.Run(change, func(t *testing.T) {
			stm := newTestPersonalityDB(t)
			first := observePersonality(t, stm, PersonalityObservation{ID: "a", Human: true, BeginTurn: true})
			switch change {
			case "turn":
				observePersonality(t, stm, PersonalityObservation{ID: "b", Human: true, BeginTurn: true})
			case "reset":
				if _, err := stm.ResetPersonalityDynamics(time.Now()); err != nil {
					t.Fatal(err)
				}
			case "persona":
				if err := stm.SetPersonalityContext("friend", DefaultPersonalityMeta()); err != nil {
					t.Fatal(err)
				}
			}
			_, err := stm.ApplyPersonalityObservation(PersonalityObservation{ID: "a", Semantic: true, Basis: &first, Emotion: &EmotionState{Description: "A stale feeling.", Valence: 1}})
			if !errors.Is(err, ErrStalePersonalityObservation) {
				t.Fatalf("stale analysis was accepted: %v", err)
			}
		})
	}
}

func TestPersonalityDynamicsTransactionRollsBackEveryProjection(t *testing.T) {
	for _, table := range []string{"affect_events", "mood_log", "emotion_history", "personality_observations"} {
		t.Run(table, func(t *testing.T) {
			stm := newTestPersonalityDB(t)
			before, _ := stm.GetPersonalitySnapshotAt(time.Now())
			if _, err := stm.db.Exec("CREATE TRIGGER reject_personality_insert BEFORE INSERT ON " + table + " BEGIN SELECT RAISE(ABORT,'injected failure'); END"); err != nil {
				t.Fatal(err)
			}
			event, _ := affectEventByCause(AffectCauseNegativeFeedback)
			_, err := stm.ApplyPersonalityObservation(PersonalityObservation{ID: "failed", Event: &event, TraitDeltas: map[string]float64{TraitConfidence: -0.1}, Emotion: &EmotionState{Description: "This must never be committed."}})
			if err == nil {
				t.Fatal("expected injected failure")
			}
			after, _ := stm.GetPersonalitySnapshotAt(time.Now())
			if after.Dynamics.Revision != before.Dynamics.Revision || after.Affect.Valence != before.Affect.Valence || after.Traits[TraitConfidence] != before.Traits[TraitConfidence] {
				t.Fatal("transaction left partial state")
			}
		})
	}
}

func TestPersonalityDynamicsConcurrentDuplicateIsAppliedOnce(t *testing.T) {
	stm := newTestPersonalityDB(t)
	event, _ := affectEventByCause(AffectCausePositiveFeedback)
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := stm.ApplyPersonalityObservation(PersonalityObservation{ID: "shared", Event: &event})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	state, err := stm.GetPersonalitySnapshotAt(time.Now())
	if err != nil || state.Dynamics.Revision != 1 {
		t.Fatalf("duplicate concurrency changed revision: %+v, %v", state.Dynamics, err)
	}
}

func TestPersonalityDynamicsConcurrentChannelsPreservePrimaryEvidence(t *testing.T) {
	stm := newTestPersonalityDB(t)
	if err := stm.SetPersonalityContext("friend", DefaultPersonalityMeta()); err != nil {
		t.Fatal(err)
	}
	basis, err := stm.GetPersonalitySnapshotAt(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	event, _ := affectEventByCause(AffectCausePositiveFeedback)
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, err := stm.ApplyPersonalityObservation(PersonalityObservation{ID: fmt.Sprintf("channel-%d", id), Human: true, BeginTurn: true, Basis: &basis, Event: &event})
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	after, _ := stm.GetPersonalitySnapshotAt(time.Now())
	if after.Dynamics.Revision != basis.Dynamics.Revision+12 {
		t.Fatalf("concurrent primary evidence lost: %+v", after.Dynamics)
	}
	_, err = stm.ApplyPersonalityObservation(PersonalityObservation{ID: "old-helper", Semantic: true, Basis: &basis})
	if !errors.Is(err, ErrStalePersonalityObservation) {
		t.Fatalf("superseded enrichment was accepted: %v", err)
	}
	if _, err = stm.ResetPersonalityDynamics(time.Now()); err != nil {
		t.Fatal(err)
	}
	_, err = stm.ApplyPersonalityObservation(PersonalityObservation{ID: "old-primary", BeginTurn: true, Human: true, Basis: &after, Event: &event})
	if !errors.Is(err, ErrStalePersonalityObservation) {
		t.Fatalf("pre-reset primary evidence was accepted: %v", err)
	}
}

func TestPersonalityDynamicsMigrationBacksUpAndRestartRetainsState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "personality.db")
	stm, err := NewSQLiteMemory(path, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if err = stm.SetTrait(TraitAffinity, 0.83); err != nil {
		t.Fatal(err)
	}
	if err = stm.SetTrait(TraitCuriosity, 0); err != nil {
		t.Fatal(err)
	}
	if _, err = stm.db.Exec(`DROP TABLE personality_dynamics; DROP TABLE personality_observations;`); err != nil {
		t.Fatal(err)
	}
	if err = stm.InitPersonalityDynamics(); err != nil {
		t.Fatal(err)
	}
	backups, err := filepath.Glob(path + ".personality-dynamics-v1-*.bak")
	if err != nil || len(backups) != 1 {
		t.Fatalf("migration backup missing: %v %v", backups, err)
	}
	now := time.Now().Truncate(time.Second)
	event, _ := affectEventByCause(AffectCauseNegativeFeedback)
	before := observePersonality(t, stm, PersonalityObservation{ID: "persistent", Event: &event, At: now})
	if err = stm.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewSQLiteMemory(path, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	after, err := reopened.GetPersonalitySnapshotAt(now)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("restart lost state: before=%+v after=%+v err=%v", before, after, err)
	}
	if after.Dynamics.Familiarity != 0.83 {
		t.Fatal("migration changed affinity")
	}
	if after.Traits[TraitCuriosity] != 0 {
		t.Fatal("restart reset an existing zero trait")
	}
	duplicate := observePersonality(t, reopened, PersonalityObservation{ID: "persistent", Event: &event, At: now})
	if duplicate.Dynamics.Revision != before.Dynamics.Revision {
		t.Fatal("restart lost deduplication")
	}
}
