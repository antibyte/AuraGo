package personabooster

import (
	"math"
	"math/rand/v2"
	"time"
)

// Intensity values accepted by the config.
const (
	IntensityMild   = "mild"
	IntensityStrong = "strong"
	IntensityWild   = "wild"
)

// Lifecycle constants from the design.
const (
	HostileBlock      = 6 * time.Hour
	MinReplaceTurns   = 2
	MaxSpontaneousGap = 6 * time.Hour
	SessionIdleReset  = time.Hour
	LongSessionAge    = 2 * time.Hour
	LongSessionTurns  = 40

	// MaxTrackedSessions bounds State.Sessions; the most recently used remain.
	MaxTrackedSessions = 8
)

// SessionCounter counts the user turns of one conversation session (web chat,
// Telegram and so on).
type SessionCounter struct {
	StartedAt  time.Time `json:"started_at,omitzero"`
	Turns      int       `json:"turns,omitempty"`
	LastTurnAt time.Time `json:"last_turn_at,omitzero"`
}

// State is the persisted booster state. Phase fields belong to Persona; the
// session counters describe the conversations and survive a persona change.
type State struct {
	Persona              string                    `json:"persona"`
	PhaseID              string                    `json:"phase_id,omitempty"`
	VariantID            string                    `json:"variant_id,omitempty"`
	Trigger              string                    `json:"trigger,omitempty"`
	Priority             int                       `json:"priority,omitempty"`
	Hostile              bool                      `json:"hostile,omitempty"`
	StartedAt            time.Time                 `json:"started_at,omitzero"`
	ExpiresAt            time.Time                 `json:"expires_at,omitzero"`
	TurnsLeft            int                       `json:"turns_left,omitempty"`
	TurnsElapsed         int                       `json:"turns_elapsed,omitempty"`
	CooldownUntil        time.Time                 `json:"cooldown_until,omitzero"`
	HostileBlockUntil    time.Time                 `json:"hostile_block_until,omitzero"`
	LastSpontaneousCheck time.Time                 `json:"last_spontaneous_check,omitzero"`
	LastVariant          map[string]string         `json:"last_variant,omitempty"`
	Sessions             map[string]SessionCounter `json:"sessions,omitempty"`
	LastTurnID           string                    `json:"last_turn_id,omitempty"`
}

// Active reports whether a phase is running.
func (s State) Active() bool { return s.PhaseID != "" }

// Input describes one decision step.
type Input struct {
	Persona     string
	Now         time.Time
	Signals     []string // trigger names ("signal:…", "cue:…")
	Intensity   string
	Spontaneous bool
	Cooldown    time.Duration
	NewTurn     bool // once per user turn: counts turns and allows random starts
}

// EventKind classifies what a decision step changed.
type EventKind string

const (
	EventNone     EventKind = ""
	EventStarted  EventKind = "started"
	EventEnded    EventKind = "ended"
	EventReplaced EventKind = "replaced"
)

// Reasons a phase ended.
const (
	ReasonExpiredTime    = "expired_time"
	ReasonExpiredTurns   = "expired_turns"
	ReasonEndsOn         = "ends_on"
	ReasonCalm           = "calm"
	ReasonPersonaChanged = "persona_changed"
	ReasonPhaseMissing   = "phase_missing"
	ReasonReplaced       = "replaced"
)

// Event describes a decision step for logging.
type Event struct {
	Kind           EventKind
	Persona        string
	PhaseID        string
	VariantID      string
	Trigger        string
	Reason         string
	EndedPhaseID   string
	EndedVariantID string
}

type candidate struct {
	phase    Phase
	weight   float64
	priority int
	trigger  string
}

// Decide advances the state by one step. It reads the time only from in.Now
// and draws randomness only from rng, so tests are deterministic.
func Decide(state State, in Input, cat *Catalog, rng *rand.Rand) (State, Event) {
	if cat == nil || rng == nil {
		return state, Event{}
	}
	now := in.Now
	var ended Event
	if state.Persona != in.Persona {
		if state.Active() {
			ended = Event{Persona: state.Persona, EndedPhaseID: state.PhaseID, EndedVariantID: state.VariantID, Reason: ReasonPersonaChanged}
		}
		state = State{
			Persona: in.Persona, Sessions: state.Sessions, LastTurnID: state.LastTurnID,
		}
	}
	state.LastVariant = cloneVariantMap(state.LastVariant)
	end := func(reason string) {
		ended = Event{Persona: state.Persona, EndedPhaseID: state.PhaseID, EndedVariantID: state.VariantID, Reason: reason}
		state = clearPhase(state)
		state.CooldownUntil = now.Add(in.Cooldown)
	}

	active := activeTriggers(in.Signals)
	top := 0
	for name := range active {
		top = max(top, Priority(name))
	}

	if state.Active() {
		if in.NewTurn {
			state.TurnsElapsed++
			state.TurnsLeft--
		}
		phase, known := cat.FindPhase(state.PhaseID)
		switch {
		case !known:
			end(ReasonPhaseMissing)
		case !now.Before(state.ExpiresAt):
			end(ReasonExpiredTime)
		case state.TurnsLeft <= 0:
			end(ReasonExpiredTurns)
		case top >= PriorityCalm && (state.Hostile || active[SignalTextStop] || active[CueWantsCalm]):
			end(ReasonCalm)
		case endsOn(phase, active):
			end(ReasonEndsOn)
		}
	}
	if top >= PriorityCalm {
		state.HostileBlockUntil = now.Add(HostileBlock)
	}

	started := false
	if cand, ok := signalCandidate(cat, state, active, now, rng); ok {
		switch {
		case !state.Active():
			state, started = startPhase(state, cand, in, rng), true
		case cand.priority >= PriorityCalm || (cand.priority > state.Priority && state.TurnsElapsed >= MinReplaceTurns):
			ended = Event{Persona: state.Persona, EndedPhaseID: state.PhaseID, EndedVariantID: state.VariantID, Reason: ReasonReplaced}
			state, started = startPhase(state, cand, in, rng), true
		}
	}
	if in.NewTurn {
		if !state.Active() && in.Spontaneous && !state.LastSpontaneousCheck.IsZero() && !now.Before(state.CooldownUntil) {
			if cand, ok := spontaneousCandidate(cat, state, in, rng); ok {
				state, started = startPhase(state, cand, in, rng), true
			}
		}
		state.LastSpontaneousCheck = now
	}
	return state, decisionEvent(state, ended, started)
}

// TrackSession counts one user turn of the given session. Only that session
// changes: a first turn or an idle gap of SessionIdleReset starts it anew, and
// the other sessions keep their counters. Beyond MaxTrackedSessions the session
// with the oldest last turn is forgotten.
func TrackSession(state State, sessionID string, now time.Time) State {
	sessions := make(map[string]SessionCounter, len(state.Sessions)+1)
	for id, counter := range state.Sessions {
		sessions[id] = counter
	}
	counter := sessions[sessionID]
	if counter.LastTurnAt.IsZero() || now.Sub(counter.LastTurnAt) >= SessionIdleReset {
		counter = SessionCounter{StartedAt: now}
	}
	counter.Turns++
	counter.LastTurnAt = now
	sessions[sessionID] = counter
	for len(sessions) > MaxTrackedSessions {
		oldest, found := "", false
		for id, other := range sessions {
			if !found || other.LastTurnAt.Before(sessions[oldest].LastTurnAt) ||
				(other.LastTurnAt.Equal(sessions[oldest].LastTurnAt) && id < oldest) {
				oldest, found = id, true
			}
		}
		delete(sessions, oldest)
	}
	state.Sessions = sessions
	return state
}

// LongSession reports the design condition for one session: active for 2 h or
// 40 user turns. Call it after TrackSession for the same turn.
func LongSession(state State, sessionID string, now time.Time) bool {
	counter, ok := state.Sessions[sessionID]
	if !ok {
		return false
	}
	if counter.Turns >= LongSessionTurns {
		return true
	}
	return !counter.StartedAt.IsZero() && now.Sub(counter.StartedAt) >= LongSessionAge
}

func signalCandidate(cat *Catalog, state State, active map[string]bool, now time.Time, rng *rand.Rand) (candidate, bool) {
	var best []candidate
	bestPriority := 0
	for _, phase := range cat.Phases {
		if phase.ID == state.PhaseID || (phase.Hostile && now.Before(state.HostileBlockUntil)) {
			continue
		}
		c := candidate{phase: phase}
		for _, name := range sortedTriggerNames(phase.Triggers) {
			if name == TriggerSpontaneous || !active[name] {
				continue
			}
			c.weight += phase.Triggers[name]
			if p := Priority(name); p > c.priority {
				c.priority, c.trigger = p, name
			}
		}
		if c.weight <= 0 || (c.priority < PriorityConfront && now.Before(state.CooldownUntil)) {
			continue
		}
		switch {
		case c.priority > bestPriority:
			best, bestPriority = []candidate{c}, c.priority
		case c.priority == bestPriority:
			best = append(best, c)
		}
	}
	if len(best) == 0 {
		return candidate{}, false
	}
	return weightedPick(best, rng), true
}

func spontaneousCandidate(cat *Catalog, state State, in Input, rng *rand.Rand) (candidate, bool) {
	gap := min(in.Now.Sub(state.LastSpontaneousCheck), MaxSpontaneousGap)
	chance := min(cat.SpontaneousPerHour*spontaneousFactor(in.Intensity), 1)
	if gap <= 0 || chance <= 0 {
		return candidate{}, false
	}
	if rng.Float64() >= 1-math.Pow(1-chance, gap.Hours()) {
		return candidate{}, false
	}
	var cands []candidate
	for _, phase := range cat.Phases {
		weight := phase.Triggers[TriggerSpontaneous]
		if weight <= 0 || (phase.Hostile && in.Now.Before(state.HostileBlockUntil)) {
			continue
		}
		cands = append(cands, candidate{phase: phase, weight: weight, priority: PrioritySpontaneous, trigger: TriggerSpontaneous})
	}
	if len(cands) == 0 {
		return candidate{}, false
	}
	return weightedPick(cands, rng), true
}

func startPhase(state State, cand candidate, in Input, rng *rand.Rand) State {
	variant := pickVariant(cand.phase, state.LastVariant[cand.phase.ID], rng)
	factor := durationFactor(in.Intensity)
	minutes := drawDuration(cand.phase.Minutes, factor, rng)
	turns := drawDuration(cand.phase.Turns, factor, rng)
	state.PhaseID, state.VariantID = cand.phase.ID, variant.ID
	state.Trigger, state.Priority, state.Hostile = cand.trigger, cand.priority, cand.phase.Hostile
	state.StartedAt = in.Now
	state.ExpiresAt = in.Now.Add(time.Duration(minutes) * time.Minute)
	state.TurnsLeft, state.TurnsElapsed = turns, 0
	if state.LastVariant == nil {
		state.LastVariant = map[string]string{}
	}
	state.LastVariant[cand.phase.ID] = variant.ID
	return state
}

func pickVariant(phase Phase, last string, rng *rand.Rand) Variant {
	options := make([]Variant, 0, len(phase.Variants))
	for _, variant := range phase.Variants {
		if variant.ID != last {
			options = append(options, variant)
		}
	}
	if len(options) == 0 {
		options = phase.Variants
	}
	index := min(int(rng.Float64()*float64(len(options))), len(options)-1)
	return options[index]
}

// drawDuration draws uniformly from [min, max] and scales by the intensity
// factor. Only Float64 is used so constant test sources never loop.
func drawDuration(bounds []int, factor float64, rng *rand.Rand) int {
	if len(bounds) != 2 {
		return 1
	}
	low, high := bounds[0], bounds[1]
	n := min(low+int(rng.Float64()*float64(high-low+1)), high)
	return max(1, int(math.Round(float64(n)*factor)))
}

func weightedPick(cands []candidate, rng *rand.Rand) candidate {
	total := 0.0
	for _, c := range cands {
		total += c.weight
	}
	r := rng.Float64() * total
	for _, c := range cands {
		if r < c.weight {
			return c
		}
		r -= c.weight
	}
	return cands[len(cands)-1]
}

func decisionEvent(state State, ended Event, started bool) Event {
	ev := ended
	switch {
	case started && ended.EndedPhaseID != "":
		ev.Kind = EventReplaced
	case started:
		ev.Kind = EventStarted
	case ended.EndedPhaseID != "":
		ev.Kind = EventEnded
	default:
		return Event{}
	}
	if started {
		ev.Persona, ev.PhaseID, ev.VariantID, ev.Trigger = state.Persona, state.PhaseID, state.VariantID, state.Trigger
	}
	return ev
}

func clearPhase(state State) State {
	state.PhaseID, state.VariantID, state.Trigger = "", "", ""
	state.Priority, state.Hostile = 0, false
	state.StartedAt, state.ExpiresAt = time.Time{}, time.Time{}
	state.TurnsLeft, state.TurnsElapsed = 0, 0
	return state
}

func activeTriggers(signals []string) map[string]bool {
	active := make(map[string]bool, len(signals))
	for _, name := range signals {
		if name != TriggerSpontaneous && KnownTrigger(name) {
			active[name] = true
		}
	}
	return active
}

func endsOn(phase Phase, active map[string]bool) bool {
	for _, name := range phase.EndsOn {
		if active[name] {
			return true
		}
	}
	return false
}

func cloneVariantMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func durationFactor(intensity string) float64 {
	switch intensity {
	case IntensityMild:
		return 0.6
	case IntensityWild:
		return 1.5
	default:
		return 1
	}
}

func spontaneousFactor(intensity string) float64 {
	switch intensity {
	case IntensityMild:
		return 0.5
	case IntensityWild:
		return 1.5
	default:
		return 1
	}
}
