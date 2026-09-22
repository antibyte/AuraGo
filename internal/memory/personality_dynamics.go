package memory

import (
	"crypto/sha256"
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	PersonalityDynamicsVersion        = 1
	PersonalityLoadHalfLife           = 12 * time.Hour
	PersonalityFrictionHalfLife       = 24 * time.Hour
	personalityMinGain                = 0.5
	personalityMaxGain                = 1.25
	personalityRelationshipConfidence = 0.8
	personalityMaxStimuli             = 32
)

// PersonalityObservation is host-owned evidence. Model output may supply only
// the bounded semantic fields, never provenance, identity, or revision guards.
type PersonalityObservation struct {
	ID            string
	Source        string
	Target        string
	Confidence    float64
	At            time.Time
	Event         *AffectEvent
	Mood          Mood
	TraitDeltas   map[string]float64
	AffinityDelta float64
	Signal        string // praise, criticism, repair, or empty
	Human         bool
	Explicit      bool
	BeginTurn     bool
	Semantic      bool
	Basis         *PersonalitySnapshot
	Emotion       *EmotionState
	Meta          *PersonalityMeta
}

// PersonalityDynamics is the content-free, public view of the evolving state.
type PersonalityDynamics struct {
	Version     int       `json:"version"`
	Revision    uint64    `json:"revision"`
	Load        float64   `json:"load"`
	Friction    float64   `json:"friction"`
	Familiarity float64   `json:"familiarity"`
	Trend       string    `json:"trend"`
	Reasons     []string  `json:"reasons"`
	Recoveries  int       `json:"recoveries"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// PersonalitySnapshot is a detached, consistent view used by one prompt or
// helper request. Callers must treat it as immutable after handing it off.
type PersonalitySnapshot struct {
	Affect   AffectState
	Traits   PersonalityTraits
	Dynamics PersonalityDynamics
	Epoch    uint64
	Persona  string
	TurnID   string
}

type personalityStimulus struct {
	Count float64   `json:"count"`
	Gain  float64   `json:"gain"`
	At    time.Time `json:"at"`
}

type personalityDynamicsState struct {
	Version      int                            `json:"version"`
	Revision     uint64                         `json:"revision"`
	Epoch        uint64                         `json:"epoch"`
	Persona      string                         `json:"persona"`
	TurnID       string                         `json:"turn_id"`
	Load         float64                        `json:"load"`
	Friction     float64                        `json:"friction"`
	Trend        string                         `json:"trend"`
	Reasons      []string                       `json:"reasons"`
	Recoveries   int                            `json:"recoveries"`
	HumanEvents  int                            `json:"human_events"`
	UpdatedAt    time.Time                      `json:"updated_at"`
	PendingMood  Mood                           `json:"pending_mood"`
	PendingCount int                            `json:"pending_count"`
	Stimuli      map[string]personalityStimulus `json:"stimuli"`
	Meta         PersonalityMeta                `json:"meta"`
}

func newPersonalityDynamics() personalityDynamicsState {
	return personalityDynamicsState{Version: PersonalityDynamicsVersion, Trend: "steady", Reasons: []string{}, Meta: DefaultPersonalityMeta(), Stimuli: map[string]personalityStimulus{}}
}

func projectPersonalityDynamics(state personalityDynamicsState, now time.Time) personalityDynamicsState {
	state.Reasons = append([]string{}, state.Reasons...)
	stimuli := make(map[string]personalityStimulus, len(state.Stimuli))
	for key, value := range state.Stimuli {
		stimuli[key] = value
	}
	state.Stimuli = stimuli
	elapsed := now.Sub(state.UpdatedAt)
	if !state.UpdatedAt.IsZero() && elapsed > 0 {
		state.Load *= math.Exp2(-elapsed.Hours() / PersonalityLoadHalfLife.Hours())
		state.Friction *= math.Exp2(-elapsed.Hours() / PersonalityFrictionHalfLife.Hours())
		if state.Load < 0.02 && state.Friction < 0.02 {
			state.Trend = "steady"
		}
	}
	state.Load = clampFinite(state.Load, 0, 1, 0)
	state.Friction = clampFinite(state.Friction, 0, 1, 0)
	return state
}

func (state personalityDynamicsState) view(traits PersonalityTraits) PersonalityDynamics {
	return PersonalityDynamics{Version: state.Version, Revision: state.Revision, Load: state.Load, Friction: state.Friction,
		Familiarity: clampFinite(traits[TraitAffinity], 0, 1, traitDefault), Trend: state.Trend,
		Reasons: append([]string{}, state.Reasons...), Recoveries: state.Recoveries, UpdatedAt: state.UpdatedAt}
}

func personalityObservationKey(source, id string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(source+"\x00"+id)))
}

func personalityStimulusKey(observation PersonalityObservation) string {
	cause := "semantic"
	if observation.Event != nil {
		cause = observation.Event.CauseCode
	}
	// Detail is not retained. Its digest distinguishes independent operational
	// problems without storing user text, tool output, or credentials here.
	detail := ""
	if observation.Event != nil && !observation.Human {
		detail = observation.Event.Detail
	}
	return personalityObservationKey(observation.Source, cause+"\x00"+detail)
}

func adaptivePersonalityWeight(state *personalityDynamicsState, observation PersonalityObservation, now time.Time) float64 {
	key := personalityStimulusKey(observation)
	trace, found := state.Stimuli[key]
	if !found {
		trace.Gain = 1
	}
	elapsed := now.Sub(trace.At)
	if elapsed > 0 {
		trace.Count *= math.Exp2(-elapsed.Seconds() / AffectRepeatWindow.Seconds())
		trace.Gain += (1 - trace.Gain) * (1 - math.Exp2(-elapsed.Hours()/4))
	}
	trace.Gain = clampFinite(trace.Gain, personalityMinGain, personalityMaxGain, 1)
	weight := trace.Gain / (1 + 0.35*trace.Count)
	trace.Count = math.Min(20, trace.Count+1)
	if trace.Count >= 4 {
		trace.Gain = math.Max(personalityMinGain, trace.Gain*0.9)
	}
	trace.At = now
	if !found && len(state.Stimuli) >= personalityMaxStimuli {
		oldestKey := ""
		var oldest time.Time
		for candidate, entry := range state.Stimuli {
			if oldestKey == "" || entry.At.Before(oldest) || (entry.At.Equal(oldest) && candidate < oldestKey) {
				oldestKey, oldest = candidate, entry.At
			}
		}
		delete(state.Stimuli, oldestKey)
	}
	state.Stimuli[key] = trace
	return clampFinite(weight, 0.1, personalityMaxGain, 1)
}

func personalityMoodTransition(state *personalityDynamicsState, current, candidate Mood, explicit, confirmation bool) Mood {
	if candidate == "" {
		return current
	}
	if candidate == MoodCreative || candidate == MoodAnalytical || candidate == MoodFocused || explicit || current == candidate {
		state.PendingMood, state.PendingCount = "", 0
		return candidate
	}
	if !confirmation {
		return current
	}
	if state.PendingMood != candidate {
		state.PendingMood, state.PendingCount = candidate, 1
	} else {
		state.PendingCount++
	}
	if state.PendingCount >= 2 {
		state.PendingMood, state.PendingCount = "", 0
		return candidate
	}
	return current
}

func relationshipObservation(observation PersonalityObservation) bool {
	if !observation.Human || observation.Target != "agent" || clampFinite(observation.Confidence, 0, 1, 0) < personalityRelationshipConfidence {
		return false
	}
	switch observation.Signal {
	case "praise", "criticism", "repair":
		return true
	}
	return false
}

// integratePersonalityObservation is deterministic. Time, evidence and prior
// state are explicit; no model, random sampling, or wall-clock reads occur here.
func integratePersonalityObservation(current AffectState, state personalityDynamicsState, observation PersonalityObservation, relationshipApplied bool, now time.Time) (AffectState, personalityDynamicsState, float64) {
	state = projectPersonalityDynamics(state, now)
	previousLoad := state.Load
	meta := state.Meta.Normalized()
	if observation.Meta != nil {
		meta = observation.Meta.Normalized()
		state.Meta = meta
	}
	next := DecayAffect(current, now)
	candidate := next.Mood
	weightScale := 1.0
	if observation.Event != nil && !observation.Semantic {
		weightScale = adaptivePersonalityWeight(&state, observation, now)
		event := *observation.Event
		weight := event.Weight
		if weight <= 0 {
			weight = AffectDefaultWeight
		}
		weight = clampFinite(weight*weightScale*meta.Volatility, 0, 0.6, 0)
		// Do not stack the old single-cause repeat discount on the new family
		// ledger, and preserve explicit zero volatility without a weight floor.
		next.Valence = clampFinite(next.Valence+(clampFinite(event.Valence, -1, 1, 0)-next.Valence)*weight, -1, 1, 0)
		next.Arousal = clampFinite(next.Arousal+(clampFinite(event.Arousal, 0, 1, AffectRestArousal)-next.Arousal)*weight, 0, 1, AffectRestArousal)
		if weight > 0 {
			next.CauseCode = event.CauseCode
			candidate = DeriveMoodFromAffect(next.Valence, next.Arousal)
			if event.CauseCode == AffectCauseConversation {
				candidate = selectAffectMood(next.Valence, next.Arousal, current.Mood)
			}
			if event.Valence < 0 {
				state.Load += (1 - state.Load) * weight * math.Abs(event.Valence) * 0.8
			} else {
				state.Load *= 1 - weight*math.Max(0, event.Valence)*0.6
			}
		}
		state.Reasons = append(state.Reasons, event.CauseCode)
		if len(state.Reasons) > 4 {
			state.Reasons = state.Reasons[len(state.Reasons)-4:]
		}
	}
	if observation.Emotion != nil {
		semantic := IntegrateEmotionAffect(next, *observation.Emotion, now)
		// Volatility zero also suppresses semantic numerical movement.
		next.Valence += (semantic.Valence - next.Valence) * math.Min(1, meta.Volatility)
		next.Arousal += (semantic.Arousal - next.Arousal) * math.Min(1, meta.Volatility)
		candidate = selectAffectMood(next.Valence, next.Arousal, semantic.Mood)
		if next.CauseCode == "" {
			next.CauseCode = "emotion_synthesis"
		}
	}
	if observation.Mood != "" {
		candidate = selectAffectMood(next.Valence, next.Arousal, observation.Mood)
	}
	affinityDelta := 0.0
	if !relationshipApplied && relationshipObservation(observation) && meta.Volatility > 0 {
		amount := clampFinite(observation.Confidence, 0, 1, 0) * weightScale * math.Min(1, meta.Volatility)
		before := state.Friction
		switch observation.Signal {
		case "criticism":
			state.Friction += (1 - state.Friction) * 0.25 * amount
		case "praise":
			state.Friction *= 1 - 0.2*amount
		case "repair":
			state.Friction *= 1 - 0.45*amount
		}
		if before > 0.08 && state.Friction < before {
			state.Recoveries = min(1000, state.Recoveries+1)
		}
		affinityDelta = clampFinite(observation.AffinityDelta, -0.015, 0.03, 0) * amount
		state.HumanEvents = min(100000, state.HumanEvents+1)
	}
	state.Load = clampFinite(state.Load, 0, 1, 0)
	state.Friction = clampFinite(state.Friction, 0, 1, 0)
	switch {
	case state.Load < previousLoad-0.005:
		state.Trend = "recovering"
	case state.Load >= 0.25 || state.Friction >= 0.2:
		state.Trend = "strained"
	default:
		state.Trend = "steady"
	}
	next.Mood = personalityMoodTransition(&state, current.Mood, candidate, observation.Explicit, !observation.Semantic)
	next.UpdatedAt = now
	state.UpdatedAt = now
	return next, state, affinityDelta
}

// PersonalityDynamicsHint uses only trusted numeric state, never free-form
// reasons or model text. It complements the selected persona within its voice.
func PersonalityDynamicsHint(snapshot PersonalitySnapshot) string {
	var hints []string
	if snapshot.Dynamics.Friction >= 0.2 {
		if snapshot.Dynamics.Familiarity >= 0.6 {
			hints = append(hints, "Keep the familiar warmth while acknowledging recent friction calmly.")
		} else {
			hints = append(hints, "Use a patient, constructive tone after recent friction.")
		}
	}
	if snapshot.Dynamics.Load >= 0.25 {
		hints = append(hints, "Sound composed and specific despite recent setbacks; keep the same care and task quality.")
	}
	if snapshot.Dynamics.Trend == "recovering" {
		hints = append(hints, "Let measured relief show after confirmed progress; avoid repeated apologies.")
	}
	return strings.Join(hints, " ")
}
