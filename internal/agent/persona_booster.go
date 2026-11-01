package agent

import (
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/personabooster"
	"aurago/internal/prompts"
)

// personaBoosterPrompt is the prompt-facing result of one booster turn.
type personaBoosterPrompt struct {
	Enabled   bool
	Amplifier string
	Phase     string
}

// apply copies the turn result into the prompt flags. The loop calls it on
// every iteration because flags can be reloaded from the loop state.
func (p personaBoosterPrompt) apply(flags *prompts.ContextFlags) {
	flags.PersonaBoosterEnabled = p.Enabled
	flags.PersonaAmplifier = p.Amplifier
	flags.PersonaPhase = p.Phase
}

// personaBoosterTurn carries the turn-start inputs that RunConfig lacks.
type personaBoosterTurn struct {
	Now             time.Time
	InactivityHours float64
	MoodAvailable   bool
}

// newPersonaBoosterRand is replaced in tests for deterministic draws.
var newPersonaBoosterRand = func() *rand.Rand {
	return rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
}

// personaBoosterToolErrors keeps the consecutive tool errors at the end of the
// previous turn per session; the run-local recovery counter is zero at turn start.
var personaBoosterToolErrors sync.Map

func rememberPersonaBoosterToolErrors(sessionID string, count int) {
	if sessionID != "" {
		personaBoosterToolErrors.Store(sessionID, max(count, 0))
	}
}

func lastPersonaBoosterToolErrors(sessionID string) int {
	if value, ok := personaBoosterToolErrors.Load(sessionID); ok {
		if count, ok := value.(int); ok {
			return count
		}
	}
	return 0
}

// preparePersonaBoosterTurn renders the static amplifier whenever the booster
// is on and the persona has a catalog, and advances the phase state once per
// eligible human turn.
func preparePersonaBoosterTurn(runCfg RunConfig, flags prompts.ContextFlags, logger *slog.Logger, turn personaBoosterTurn) personaBoosterPrompt {
	cfg, stm := runCfg.Config, runCfg.ShortTermMem
	if cfg == nil || !cfg.Personality.PersonaBooster.Enabled {
		return personaBoosterPrompt{}
	}
	logger = personaBoosterLogger(logger)
	persona := effectivePersonalityID(flags.CorePersonality)
	cat, ok := loadPersonaBoosterCatalog(cfg, persona, logger)
	if !ok {
		return personaBoosterPrompt{}
	}
	booster := cfg.Personality.PersonaBooster
	out := personaBoosterPrompt{Enabled: true, Amplifier: personabooster.RenderAmplifier(cat, booster.Intensity)}
	if stm == nil || !personaBoosterTurnEligible(runCfg, flags) {
		return out
	}
	now := turn.Now
	if now.IsZero() {
		now = time.Now()
	}
	signals := personabooster.DetectTextSignals(runCfg.HumanInput.Text)
	if turn.MoodAvailable {
		if snapshot, err := stm.GetPersonalitySnapshotAt(now); err == nil {
			signals = append(signals, personabooster.MoodSignals(personabooster.MoodInput{
				Mood: string(snapshot.Affect.Mood), Arousal: snapshot.Affect.Arousal,
				Load: snapshot.Dynamics.Load, Friction: snapshot.Dynamics.Friction,
				ConsecutiveToolErrors: lastPersonaBoosterToolErrors(runCfg.SessionID),
			})...)
		}
	}
	rng := newPersonaBoosterRand()
	var event personabooster.Event
	state, err := stm.UpdatePersonaBoosterState(func(state personabooster.State) personabooster.State {
		newTurn := state.LastTurnID != runCfg.HumanInput.TurnID
		if newTurn {
			state = personabooster.TrackSession(state, runCfg.SessionID, now)
			state.LastTurnID = runCfg.HumanInput.TurnID
		}
		turnSignals := append(append([]string(nil), signals...),
			personabooster.TimeSignals(now, turn.InactivityHours, personabooster.LongSession(state, now))...)
		next, ev := personabooster.Decide(state, personabooster.Input{
			Persona: persona, Now: now, Signals: turnSignals, Intensity: booster.Intensity,
			Spontaneous: booster.Spontaneous, Cooldown: time.Duration(booster.CooldownMinutes) * time.Minute,
			NewTurn: newTurn,
		}, cat, rng)
		event = ev
		return next
	})
	if err != nil {
		logger.Warn("[PersonaBooster] State update failed", "error", err)
		return out
	}
	logPersonaBoosterEvent(logger, event, "turn")
	out.Phase = personabooster.RenderPhase(cat, state, booster.Intensity)
	return out
}

// personaBoosterTurnEligible excludes missions, co-agents, autonomous and
// maintenance runs, transient sessions, prepared or frozen prompts and runs
// without a human turn.
func personaBoosterTurnEligible(runCfg RunConfig, flags prompts.ContextFlags) bool {
	if runCfg.PreparedPrompt != nil || runCfg.StableSystemPrompt || isCoAgentSession(runCfg.SessionID) {
		return false
	}
	if !runCfg.HumanInput.Valid() || runCfg.HumanInput.SessionID != runCfg.SessionID {
		return false
	}
	return shouldRunTurnSideEffects(runCfg, runCfg.SessionID, flags)
}

// loadPersonaBoosterCatalog returns the persona's catalog. For the generic
// catalog only the phases that fit the profile meta remain.
func loadPersonaBoosterCatalog(cfg *config.Config, persona string, logger *slog.Logger) (*personabooster.Catalog, bool) {
	if persona == "" || persona == "neutral" {
		return nil, false
	}
	cat, _, ok := prompts.LoadPersonaPhaseCatalog(cfg.Directories.PromptsDir, persona, logger)
	if !ok {
		return nil, false
	}
	if cat.Persona == personabooster.GenericPersona {
		meta := prompts.GetCorePersonalityMeta(cfg.Directories.PromptsDir, persona)
		filtered := *cat
		filtered.Phases = cat.EligiblePhases(personabooster.ProfileMeta{
			Volatility: meta.Volatility, EmpathyBias: meta.EmpathyBias, ConflictResponse: meta.ConflictResponse,
		})
		cat = &filtered
	}
	return cat, true
}

func logPersonaBoosterEvent(logger *slog.Logger, event personabooster.Event, source string) {
	if event.Kind == personabooster.EventNone {
		return
	}
	phase, variant, trigger := event.PhaseID, event.VariantID, event.Trigger
	if event.Kind == personabooster.EventEnded {
		phase, variant, trigger = event.EndedPhaseID, event.EndedVariantID, event.Reason
	}
	logger.Info("[PersonaBooster] phase "+string(event.Kind),
		"persona", event.Persona, "phase", phase, "variant", variant, "trigger", trigger,
		"ended_phase", event.EndedPhaseID, "reason", event.Reason, "source", source)
}

func personaBoosterLogger(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return slog.Default()
	}
	return logger
}
