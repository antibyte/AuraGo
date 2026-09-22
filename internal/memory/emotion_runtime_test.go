package memory

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"
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

type blockingEmotionClient struct {
	calls   atomic.Int32
	started chan struct{}
	release chan struct{}
}

func (c *blockingEmotionClient) CreateChatCompletion(ctx context.Context, _ openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	c.calls.Add(1)
	select {
	case c.started <- struct{}{}:
	default:
	}
	select {
	case <-ctx.Done():
		return openai.ChatCompletionResponse{}, ctx.Err()
	case <-c.release:
	}
	return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: `{"description":"Feeling inspired.","primary_mood":"creative","valence":0.5,"arousal":0.6,"confidence":0.8}`}}}}, nil
}

func TestEmotionRuntimeReservesAcrossConcurrentRuns(t *testing.T) {
	stm := newTestPersonalityDB(t)
	client := &blockingEmotionClient{started: make(chan struct{}, 1), release: make(chan struct{})}
	first := NewEmotionSynthesizer(client, "test", 60, 100, "English", slog.Default())
	second := NewEmotionSynthesizer(client, "test", 60, 100, "English", slog.Default())
	if err := first.BindMemory(stm); err != nil {
		t.Fatal(err)
	}
	if err := second.BindMemory(stm); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); _, _ = first.SynthesizeEmotion(ctx, stm, EmotionInput{UserMessage: "first"}) }()
	select {
	case <-client.started:
	case <-ctx.Done():
		t.Fatal("synthesis did not start")
	}
	_, _ = second.SynthesizeEmotion(ctx, stm, EmotionInput{UserMessage: "second"})
	close(client.release)
	wg.Wait()
	if client.calls.Load() != 1 {
		t.Fatalf("concurrent runs made %d requests", client.calls.Load())
	}
	if second.GetLastEmotion() == nil {
		t.Fatal("completed state was not shared")
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

func TestCancelledEmotionCannotChangeAffect(t *testing.T) {
	stm := newTestPersonalityDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := &blockingEmotionClient{started: make(chan struct{}, 1), release: make(chan struct{})}
	es := NewEmotionSynthesizer(client, "test", 60, 100, "English", slog.Default())
	_, err := es.SynthesizeEmotion(ctx, stm, EmotionInput{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want cancellation, got %v", err)
	}
	if es.GetLastEmotion() != nil {
		t.Fatal("cancelled state was published")
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
