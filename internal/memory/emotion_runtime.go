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
				DynamicsRevision: latest.DynamicsRevision, DynamicsEpoch: latest.DynamicsEpoch,
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
	es.memory = stm
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
	_, err := s.ApplyPersonalityObservation(PersonalityObservation{Source: "mood", Target: "task", Mood: mood, At: now})
	return err
}

func (s *SQLiteMemory) persistSynthesizedEmotion(state *EmotionState, trigger string) error {
	_, err := s.ApplyPersonalityObservation(PersonalityObservation{ID: state.ObservationID, Basis: state.Basis, Source: "synthesis", Target: "task", Semantic: true, Emotion: state})
	return err
}

// ApplyObservation commits a helper's mood, traits and optional narration as
// one unit. A narration cooldown never prevents a valid personality update.
func (es *EmotionSynthesizer) ApplyObservation(stm *SQLiteMemory, observation PersonalityObservation) (PersonalitySnapshot, bool, error) {
	if es == nil {
		observation.Emotion = nil
		snapshot, err := stm.ApplyPersonalityObservation(observation)
		return snapshot, false, err
	}
	es.mu.Lock()
	defer es.mu.Unlock()
	if es.inFlight || (!es.lastCall.IsZero() && time.Since(es.lastCall) < es.minInterval) {
		observation.Emotion = nil
	}
	if observation.Emotion != nil {
		copy := *observation.Emotion
		if err := validateEmotionState(&copy); err != nil {
			return PersonalitySnapshot{}, false, err
		}
		observation.Emotion = &copy
	}
	snapshot, err := stm.ApplyPersonalityObservation(observation)
	if err != nil {
		return PersonalitySnapshot{}, false, err
	}
	if observation.Emotion != nil {
		state := *observation.Emotion
		state.Basis, state.ObservationID = nil, ""
		es.lastCall, es.lastState = time.Now(), &state
	}
	return snapshot, observation.Emotion != nil, nil
}
