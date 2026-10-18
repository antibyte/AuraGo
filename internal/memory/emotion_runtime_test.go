package memory

import (
	"log/slog"
	"math"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestEmotionInfluenceSurvivesStorageAndRuntimeOverlay(t *testing.T) {
	for _, mood := range []Mood{MoodCreative, MoodAnalytical} {
		t.Run(string(mood), func(t *testing.T) {
			stm := newTestPersonalityDB(t)
			event, _ := AffectEventForTrigger(EmotionTriggerConversation, "", "chat")
			before, err := stm.ApplyAffectEvent(event, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			es := NewEmotionSynthesizer(nil, "test", 60, 100, "English", slog.Default())
			proposal := &EmotionState{Description: "A fresh idea is worth exploring.", PrimaryMood: mood, Valence: 0.9, Arousal: 0.8, Confidence: 0.9}
			if err := es.ApplyExternalState(stm, proposal, "creative task"); err != nil {
				t.Fatal(err)
			}
			persisted, err := stm.GetAffectState()
			if err != nil {
				t.Fatal(err)
			}
			state := es.GetLastEmotion()
			OverlayAffectOnEmotion(state, persisted)
			if state.PrimaryMood != mood || stm.GetCurrentMood() != mood {
				t.Fatalf("semantic mood lost: %+v", state)
			}
			if state.Valence <= before.Valence || state.Valence > before.Valence+AffectLLMValenceDelta+0.001 {
				t.Fatalf("invalid persisted influence: %+v", state)
			}
			if math.Abs(state.Valence-es.GetLastEmotion().Valence) > 0.001 {
				t.Fatal("runtime overlay discarded the accepted delta")
			}
			next, err := stm.ApplyAffectEvent(event, time.Now().Add(time.Minute))
			if err != nil {
				t.Fatal(err)
			}
			if next.Mood != mood || next.Valence <= 0 {
				t.Fatalf("ordinary next turn flattened the accepted state: %+v", next)
			}
		})
	}
}

func TestPositiveFeedbackHasVisibleMoodAndExpiredCauseDoesNotDominate(t *testing.T) {
	now := time.Now()
	event, _ := AffectEventForTrigger(EmotionTriggerPositiveFeedback, "", "chat")
	state := IntegrateAffect(RestAffect(now), event, now)
	if state.Mood != MoodRelaxed && state.Mood != MoodPlayful {
		t.Fatalf("positive feedback stayed neutral: %+v", state)
	}
	for i := 1; i < 100; i++ {
		state = IntegrateAffect(state, event, now.Add(time.Duration(i)*time.Minute))
	}
	if state.Mood == MoodCurious {
		t.Fatal("repeated praise returned to curious")
	}
	if expired := DecayAffect(state, now.Add(30*24*time.Hour)); expired.Active() {
		t.Fatalf("expired cause is still active: %+v", expired)
	}
}

func TestEmotionRuntimeRestoresAcrossTurnsAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "emotion.db")
	stm, err := NewSQLiteMemory(path, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	first := NewEmotionSynthesizer(nil, "one", 60, 100, "English", slog.Default())
	if err := first.BindMemory(stm); err != nil {
		t.Fatal(err)
	}
	if err := first.ApplyExternalState(stm, &EmotionState{Description: "Glad that worked.", PrimaryMood: MoodCreative, Valence: 0.3, Arousal: 0.4, Confidence: 0.8}, "test"); err != nil {
		t.Fatal(err)
	}
	second := NewEmotionSynthesizer(nil, "two", 60, 100, "English", slog.Default())
	if err := second.BindMemory(stm); err != nil {
		t.Fatal(err)
	}
	if can, last := second.CanSynthesizeNow(time.Now()); can || last == nil || last.Description != "Glad that worked." {
		t.Fatalf("new turn lost continuity: can=%v last=%+v", can, last)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewSQLiteMemory(path, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	third := NewEmotionSynthesizer(nil, "three", 60, 100, "English", slog.Default())
	if err := third.BindMemory(reopened); err != nil {
		t.Fatal(err)
	}
	if can, last := third.CanSynthesizeNow(time.Now()); can || last == nil {
		t.Fatalf("restart lost continuity: can=%v last=%+v", can, last)
	}
}

func TestEmotionRuntimeReservesAcrossConcurrentRuns(t *testing.T) {
	stm := newTestPersonalityDB(t)
	first := NewEmotionSynthesizer(nil, "", 60, 100, "English", slog.Default())
	second := NewEmotionSynthesizer(nil, "", 60, 100, "English", slog.Default())
	for _, es := range []*EmotionSynthesizer{first, second} {
		if err := es.BindMemory(stm); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	for _, es := range []*EmotionSynthesizer{first, second} {
		wg.Add(1)
		go func(es *EmotionSynthesizer) {
			defer wg.Done()
			_ = es.ApplyExternalState(stm, &EmotionState{Description: "Feeling inspired.", PrimaryMood: MoodCreative, Valence: .5, Arousal: .6, Confidence: .8}, "chat")
		}(es)
	}
	wg.Wait()
	entries, err := stm.GetEmotionHistory(24)
	if err != nil || len(entries) != 1 || second.GetLastEmotion() == nil {
		t.Fatalf("shared cooldown/history: %d %v", len(entries), err)
	}
}

func TestEmotionStorageFailureDoesNotPartiallyChangeAffect(t *testing.T) {
	stm := newTestPersonalityDB(t)
	before, err := stm.GetAffectState()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stm.db.Exec("DROP TABLE emotion_history"); err != nil {
		t.Fatal(err)
	}
	es := NewEmotionSynthesizer(nil, "test", 60, 100, "English", slog.Default())
	err = es.ApplyExternalState(stm, &EmotionState{Description: "A new idea.", PrimaryMood: MoodCreative, Valence: 0.9, Arousal: 0.9}, "")
	if err == nil {
		t.Fatal("expected persistence failure")
	}
	after, err := stm.GetAffectState()
	if err != nil {
		t.Fatal(err)
	}
	if before.Valence != after.Valence || before.Arousal != after.Arousal || es.GetLastEmotion() != nil {
		t.Fatal("failed transaction changed affect or cache")
	}
}

func TestEmotionMoodWriteFailureRollsBackStateAndHistory(t *testing.T) {
	stm := newTestPersonalityDB(t)
	before, err := stm.GetAffectState()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stm.db.Exec("DROP TABLE mood_log"); err != nil {
		t.Fatal(err)
	}
	es := NewEmotionSynthesizer(nil, "test", 60, 100, "English", slog.Default())
	err = es.ApplyExternalState(stm, &EmotionState{Description: "A new idea.", PrimaryMood: MoodCreative, Valence: .9, Arousal: .9}, "")
	if err == nil {
		t.Fatal("expected mood persistence failure")
	}
	after, err := stm.GetAffectState()
	if err != nil {
		t.Fatal(err)
	}
	latest, err := stm.GetLatestEmotion()
	if err != nil {
		t.Fatal(err)
	}
	if after.Valence != before.Valence || after.Arousal != before.Arousal || latest != nil || es.GetLastEmotion() != nil {
		t.Fatal("mood write failure left a partial state")
	}
}
