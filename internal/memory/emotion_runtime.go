package memory

import (
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// emotionSynthesisRuntime is shared by all runs using one memory store. Clients
// and provider settings remain request-local on EmotionSynthesizer.
type emotionSynthesisRuntime struct {
	mu        sync.RWMutex
	sfGroup   singleflight.Group
	lastState *EmotionState
	lastCall  time.Time
	inFlight  bool
}

// BindMemory restores continuity after a restart and shares the cooldown and
// in-flight reservation across chat turns. Call before publishing the instance.
func (es *EmotionSynthesizer) BindMemory(stm *SQLiteMemory) error {
	if es == nil || stm == nil {
		return nil
	}
	stm.emotionRuntimeMu.Lock()
	defer stm.emotionRuntimeMu.Unlock()
	if stm.emotionRuntime == nil {
		latest, err := stm.GetLatestEmotion()
		if err != nil {
			return fmt.Errorf("restore emotion continuity: %w", err)
		}
		runtime := &emotionSynthesisRuntime{}
		if latest != nil {
			stamp, err := parseAffectTimestamp(latest.Timestamp)
			if err != nil {
				return fmt.Errorf("restore emotion timestamp: %w", err)
			}
			state := &EmotionState{
				Description: latest.Description, PrimaryMood: Mood(latest.PrimaryMood),
				SecondaryMood: latest.SecondaryMood, Valence: latest.Valence, Arousal: latest.Arousal,
				Confidence: latest.Confidence, Cause: latest.Cause, Source: latest.Source,
				RecommendedResponseStyle: latest.RecommendedResponseStyle, Timestamp: stamp,
			}
			if err := validateEmotionState(state); err != nil {
				return fmt.Errorf("restore emotion state: %w", err)
			}
			runtime.lastState, runtime.lastCall = state, stamp
		}
		stm.emotionRuntime = runtime
	}
	es.emotionSynthesisRuntime = stm.emotionRuntime
	return nil
}

func (es *EmotionSynthesizer) beginSynthesis(now time.Time) bool {
	es.mu.Lock()
	defer es.mu.Unlock()
	if es.inFlight || (!es.lastCall.IsZero() && now.Sub(es.lastCall) < es.minInterval) {
		return false
	}
	es.lastCall = now
	es.inFlight = true
	return true
}

func (es *EmotionSynthesizer) endSynthesis() {
	es.mu.Lock()
	es.inFlight = false
	es.mu.Unlock()
}

// ApplyMoodSuggestion admits a canonical V2 working style without inventing a
// numerical emotional change. Full synthesizer observations use the atomic
// history/affect write below.
func (s *SQLiteMemory) ApplyMoodSuggestion(mood Mood, now time.Time) error {
	if now.IsZero() {
		now = time.Now()
	}
	s.affectMu.Lock()
	defer s.affectMu.Unlock()
	current, err := s.GetAffectStateAt(now)
	if err != nil {
		return fmt.Errorf("load affect for mood: %w", err)
	}
	current.Mood = selectAffectMood(current.Valence, current.Arousal, mood)
	current.UpdatedAt = now
	if err := s.saveAffectState(current); err != nil {
		return err
	}
	return s.LogMood(current.Mood, "semantic mood")
}

func (s *SQLiteMemory) persistSynthesizedEmotion(state *EmotionState, trigger string) error {
	s.affectMu.Lock()
	defer s.affectMu.Unlock()
	now := time.Now()
	current, err := s.loadRawAffectState()
	if err != nil {
		return fmt.Errorf("load affect for synthesis: %w", err)
	}
	next := IntegrateEmotionAffect(current, *state, now)
	state.Valence, state.Arousal, state.PrimaryMood = next.Valence, next.Arousal, next.Mood
	state.Timestamp = now
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin emotion update: %w", err)
	}
	defer tx.Rollback()
	if err := saveAffectStateWith(tx, next); err != nil {
		return err
	}
	if err := insertEmotionStateHistoryWith(tx, *state, trigger); err != nil {
		return fmt.Errorf("store emotion history: %w", err)
	}
	if err := insertMoodLogWith(tx, next.Mood, trigger); err != nil {
		return fmt.Errorf("store emotion mood: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit emotion update: %w", err)
	}
	s.personalityCacheMu.Lock()
	s.moodCacheAt = time.Time{}
	s.personalityCacheMu.Unlock()
	return nil
}
