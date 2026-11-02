package personabooster

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"testing"
	"time"
)

type constSource uint64

func (c constSource) Uint64() uint64 { return uint64(c) }

// lowRand draws 0 (first option, minimum durations, random starts fire);
// highRand draws just below 1 (last option, maximum durations, no random start).
func lowRand() *rand.Rand  { return rand.New(constSource(0)) }
func highRand() *rand.Rand { return rand.New(constSource(math.MaxUint64)) }

var engineNow = time.Date(2026, 11, 1, 15, 0, 0, 0, time.UTC)

func engineCatalog(t *testing.T) *Catalog {
	t.Helper()
	cat, err := ParseCatalog([]byte(validCatalogYAML))
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func engineInput(now time.Time, signals ...string) Input {
	return Input{Persona: "sample", Now: now, Signals: signals, Intensity: IntensityStrong, Cooldown: 20 * time.Minute, NewTurn: true}
}

func TestDecideStartsPhaseFromSignal(t *testing.T) {
	state, ev := Decide(State{}, engineInput(engineNow, CueCriticism), engineCatalog(t), lowRand())
	if ev.Kind != EventStarted || state.PhaseID != "rage" || state.Trigger != CueCriticism || state.Priority != PriorityConfront || !state.Hostile {
		t.Fatalf("state=%+v event=%+v", state, ev)
	}
	if state.VariantID != "hot" || state.TurnsLeft != 5 || !state.ExpiresAt.Equal(engineNow.Add(15*time.Minute)) {
		t.Fatalf("lowRand must draw the first variant and minimum durations: %+v", state)
	}
	if state.Persona != "sample" || state.LastVariant["rage"] != "hot" {
		t.Fatalf("persona/variant bookkeeping: %+v", state)
	}
}

func TestDecideEndsPhaseByTimeAndTurns(t *testing.T) {
	cat := engineCatalog(t)
	started, _ := Decide(State{}, engineInput(engineNow, CueCriticism), cat, lowRand())
	byTime, ev := Decide(started, engineInput(engineNow.Add(15*time.Minute)), cat, lowRand())
	if ev.Kind != EventEnded || ev.Reason != ReasonExpiredTime || ev.EndedPhaseID != "rage" || byTime.Active() {
		t.Fatalf("time expiry: state=%+v event=%+v", byTime, ev)
	}
	if !byTime.CooldownUntil.Equal(engineNow.Add(35 * time.Minute)) {
		t.Fatalf("cooldown = %v", byTime.CooldownUntil)
	}
	state := started
	for turn := 1; turn <= 5; turn++ {
		state, ev = Decide(state, engineInput(engineNow.Add(time.Duration(turn)*time.Minute)), cat, lowRand())
		if turn < 5 && !state.Active() {
			t.Fatalf("phase ended early at turn %d", turn)
		}
	}
	if ev.Kind != EventEnded || ev.Reason != ReasonExpiredTurns || state.Active() {
		t.Fatalf("turn expiry: state=%+v event=%+v", state, ev)
	}
}

func TestDecideReplacesOnlyWithHigherPriorityAfterTwoTurns(t *testing.T) {
	cat := engineCatalog(t)
	state, _ := Decide(State{}, engineInput(engineNow, CuePraise), cat, lowRand())
	if state.PhaseID != "hype" {
		t.Fatalf("expected hype, got %+v", state)
	}
	state, ev := Decide(state, engineInput(engineNow.Add(time.Minute), CueCriticism), cat, lowRand())
	if state.PhaseID != "hype" || ev.Kind != EventNone {
		t.Fatalf("replaced after one turn: state=%+v event=%+v", state, ev)
	}
	state, ev = Decide(state, engineInput(engineNow.Add(2*time.Minute), CueCriticism), cat, lowRand())
	if ev.Kind != EventReplaced || ev.EndedPhaseID != "hype" || ev.Reason != ReasonReplaced || state.PhaseID != "rage" || state.TurnsElapsed != 0 {
		t.Fatalf("not replaced after two turns: state=%+v event=%+v", state, ev)
	}
}

func TestDecideCalmSignalClearsHostileAndBlocksIt(t *testing.T) {
	cat := engineCatalog(t)
	rage, _ := Decide(State{}, engineInput(engineNow, CueCriticism), cat, lowRand())
	calmed, ev := Decide(rage, engineInput(engineNow.Add(time.Minute), CueDistress), cat, lowRand())
	if ev.Kind != EventEnded || ev.Reason != ReasonCalm || calmed.Active() {
		t.Fatalf("distress did not end the hostile phase: state=%+v event=%+v", calmed, ev)
	}
	if !calmed.HostileBlockUntil.Equal(engineNow.Add(time.Minute + HostileBlock)) {
		t.Fatalf("hostile block = %v", calmed.HostileBlockUntil)
	}
	if blocked, ev := Decide(calmed, engineInput(engineNow.Add(2*time.Hour), CueCriticism), cat, lowRand()); blocked.Active() || ev.Kind != EventNone {
		t.Fatalf("hostile phase started during the block: %+v", blocked)
	}
	if after, _ := Decide(calmed, engineInput(engineNow.Add(7*time.Hour), CueCriticism), cat, lowRand()); after.PhaseID != "rage" {
		t.Fatalf("hostile phase still blocked after 6h: %+v", after)
	}
	hype, _ := Decide(State{}, engineInput(engineNow, CuePraise), cat, lowRand())
	if kept, _ := Decide(hype, engineInput(engineNow.Add(time.Minute), CueDistress), cat, lowRand()); kept.PhaseID != "hype" {
		t.Fatalf("distress must not end a non-hostile phase: %+v", kept)
	}
	if stopped, ev := Decide(hype, engineInput(engineNow.Add(time.Minute), SignalTextStop), cat, lowRand()); stopped.Active() || ev.Reason != ReasonCalm {
		t.Fatalf("stop must end every phase: state=%+v event=%+v", stopped, ev)
	}
}

func TestDecideEndsOnCounterTrigger(t *testing.T) {
	cat := engineCatalog(t)
	rage, _ := Decide(State{}, engineInput(engineNow, CueCriticism), cat, lowRand())
	ended, ev := Decide(rage, engineInput(engineNow.Add(time.Minute), CuePraise), cat, lowRand())
	if ev.Kind != EventEnded || ev.Reason != ReasonEndsOn || ended.Active() {
		t.Fatalf("praise did not end rage (and cooldown must block hype): state=%+v event=%+v", ended, ev)
	}
}

func TestDecideCooldownBlocksLowPriorityStarts(t *testing.T) {
	cat := engineCatalog(t)
	cooling := State{Persona: "sample", CooldownUntil: engineNow.Add(10 * time.Minute)}
	if blocked, _ := Decide(cooling, engineInput(engineNow, CuePraise), cat, lowRand()); blocked.Active() {
		t.Fatalf("priority 40 started during cooldown: %+v", blocked)
	}
	if strong, _ := Decide(cooling, engineInput(engineNow, CueCriticism), cat, lowRand()); strong.PhaseID != "rage" {
		t.Fatalf("priority 70 must pass the cooldown: %+v", strong)
	}
	if later, _ := Decide(cooling, engineInput(engineNow.Add(10*time.Minute), CuePraise), cat, lowRand()); later.PhaseID != "hype" {
		t.Fatalf("cooldown did not expire: %+v", later)
	}
}

func TestDecideSpontaneousStart(t *testing.T) {
	cat := engineCatalog(t)
	in := engineInput(engineNow)
	in.Spontaneous = true
	armed, _ := Decide(State{Persona: "sample"}, in, cat, lowRand())
	if armed.Active() || !armed.LastSpontaneousCheck.Equal(engineNow) {
		t.Fatalf("the first check must only arm the clock: %+v", armed)
	}
	in.Now = engineNow.Add(2 * time.Hour)
	fired, ev := Decide(armed, in, cat, lowRand())
	if ev.Kind != EventStarted || fired.PhaseID != "hype" || fired.Trigger != TriggerSpontaneous || fired.Priority != PrioritySpontaneous {
		t.Fatalf("random start: state=%+v event=%+v", fired, ev)
	}
	if missed, _ := Decide(armed, in, cat, highRand()); missed.Active() || !missed.LastSpontaneousCheck.Equal(in.Now) {
		t.Fatalf("a high draw must not start a phase: %+v", missed)
	}
	in.Spontaneous = false
	if off, _ := Decide(armed, in, cat, lowRand()); off.Active() {
		t.Fatalf("spontaneous=false started a phase: %+v", off)
	}
}

func TestDecideNeverRepeatsVariant(t *testing.T) {
	cat := engineCatalog(t)
	state, _ := Decide(State{}, engineInput(engineNow, CueCriticism), cat, lowRand())
	state, _ = Decide(state, engineInput(engineNow.Add(time.Minute), CuePraise), cat, lowRand())
	state.CooldownUntil = time.Time{}
	again, _ := Decide(state, engineInput(engineNow.Add(2*time.Minute), CueCriticism), cat, lowRand())
	if again.PhaseID != "rage" || again.VariantID != "cold" {
		t.Fatalf("variant repeated or wrong: %+v", again)
	}
}

func TestDecidePersonaChangeResetsState(t *testing.T) {
	cat := engineCatalog(t)
	state, _ := Decide(State{}, engineInput(engineNow, CueCriticism), cat, lowRand())
	state.Sessions = map[string]SessionCounter{"default": {Turns: 7}}
	in := engineInput(engineNow.Add(time.Minute))
	in.Persona = "other"
	reset, ev := Decide(state, in, cat, lowRand())
	if ev.Kind != EventEnded || ev.Reason != ReasonPersonaChanged || reset.Active() || reset.Persona != "other" || reset.LastVariant != nil || !reset.HostileBlockUntil.IsZero() {
		t.Fatalf("state=%+v event=%+v", reset, ev)
	}
	if reset.Sessions["default"].Turns != 7 {
		t.Fatal("session bookkeeping belongs to the conversation, not to the persona")
	}
}

func TestDecideIntensityScalesDurations(t *testing.T) {
	cat := engineCatalog(t)
	in := engineInput(engineNow, CueCriticism)
	in.Intensity = IntensityWild
	if wild, _ := Decide(State{}, in, cat, highRand()); wild.TurnsLeft != 21 || !wild.ExpiresAt.Equal(engineNow.Add(68*time.Minute)) {
		t.Fatalf("wild: %+v", wild)
	}
	in.Intensity = IntensityMild
	if mild, _ := Decide(State{}, in, cat, lowRand()); mild.TurnsLeft != 3 || !mild.ExpiresAt.Equal(engineNow.Add(9*time.Minute)) {
		t.Fatalf("mild: %+v", mild)
	}
}

func TestTrackSessionAndLongSession(t *testing.T) {
	s := TrackSession(State{}, "default", engineNow)
	s = TrackSession(s, "default", engineNow.Add(30*time.Minute))
	if c := s.Sessions["default"]; c.Turns != 2 || !c.StartedAt.Equal(engineNow) || !c.LastTurnAt.Equal(engineNow.Add(30*time.Minute)) {
		t.Fatalf("session = %+v", s.Sessions)
	}
	if LongSession(s, "default", engineNow.Add(30*time.Minute)) || !LongSession(s, "default", engineNow.Add(2*time.Hour)) {
		t.Fatal("long session by age")
	}
	if LongSession(s, "telegram", engineNow.Add(2*time.Hour)) {
		t.Fatal("an unknown session cannot be long")
	}
	idleAt := engineNow.Add(30*time.Minute + SessionIdleReset)
	if idle := TrackSession(s, "default", idleAt); idle.Sessions["default"].Turns != 1 || !idle.Sessions["default"].StartedAt.Equal(idleAt) {
		t.Fatalf("idle gap did not restart the session: %+v", idle.Sessions)
	}
	other := TrackSession(s, "telegram", engineNow.Add(31*time.Minute))
	if c := other.Sessions["telegram"]; c.Turns != 1 {
		t.Fatalf("new session id: %+v", other.Sessions)
	}
	if c := other.Sessions["default"]; c.Turns != 2 || !c.StartedAt.Equal(engineNow) {
		t.Fatalf("another session id disturbed the default session: %+v", other.Sessions)
	}
	if len(s.Sessions) != 1 {
		t.Fatalf("TrackSession modified the map of its input: %+v", s.Sessions)
	}
	c := s.Sessions["default"]
	c.Turns = LongSessionTurns
	s.Sessions["default"] = c
	if !LongSession(s, "default", engineNow.Add(31*time.Minute)) {
		t.Fatal("long session by turns")
	}
}

func TestLongSessionCountsEachSessionSeparately(t *testing.T) {
	state := State{}
	for step := 0; step <= 36; step++ { // alternate every 5 minutes for 3 hours
		now := engineNow.Add(time.Duration(step) * 5 * time.Minute)
		id := "web"
		if step%2 == 1 {
			id = "telegram"
		}
		state = TrackSession(state, id, now)
		if step == 11 && (LongSession(state, "web", now) || LongSession(state, "telegram", now)) {
			t.Fatalf("long before 2 h: %+v", state.Sessions)
		}
	}
	end := engineNow.Add(3 * time.Hour)
	for _, id := range []string{"web", "telegram"} {
		if !LongSession(state, id, end) {
			t.Errorf("session %q never became long: %+v", id, state.Sessions[id])
		}
	}
}

func TestTrackSessionKeepsTheMostRecentSessions(t *testing.T) {
	state := State{}
	for i := 0; i < 12; i++ {
		state = TrackSession(state, fmt.Sprintf("s%02d", i), engineNow.Add(time.Duration(i)*time.Minute))
	}
	if len(state.Sessions) != MaxTrackedSessions {
		t.Fatalf("tracked %d sessions, want %d", len(state.Sessions), MaxTrackedSessions)
	}
	for i := 0; i < 12; i++ {
		if _, kept := state.Sessions[fmt.Sprintf("s%02d", i)]; kept != (i >= 12-MaxTrackedSessions) {
			t.Errorf("session s%02d kept = %v", i, kept)
		}
	}
	// The oldest kept session returns and is recent again, so a newcomer
	// displaces the next oldest one.
	state = TrackSession(state, "s04", engineNow.Add(20*time.Minute))
	state = TrackSession(state, "s12", engineNow.Add(21*time.Minute))
	if len(state.Sessions) != MaxTrackedSessions {
		t.Fatalf("sessions = %+v", state.Sessions)
	}
	if _, kept := state.Sessions["s04"]; !kept {
		t.Fatal("a session that just returned was evicted")
	}
	if _, kept := state.Sessions["s05"]; kept {
		t.Fatal("the oldest session survived")
	}
}

func TestStateLoadsOldSingleSessionJSON(t *testing.T) {
	const old = `{"persona":"punk","phase_id":"rage","variant_id":"hot","session_id":"default",` +
		`"session_started_at":"2026-11-01T10:00:00Z","session_turns":9,"last_turn_at":"2026-11-01T14:50:00Z","last_turn_id":"t1"}`
	var state State
	if err := json.Unmarshal([]byte(old), &state); err != nil {
		t.Fatal(err)
	}
	if state.PhaseID != "rage" || state.LastTurnID != "t1" || len(state.Sessions) != 0 {
		t.Fatalf("state = %+v", state)
	}
	if LongSession(state, "default", engineNow) {
		t.Fatal("legacy fields must not count")
	}
	state = TrackSession(state, "default", engineNow)
	if state.Sessions["default"].Turns != 1 {
		t.Fatalf("sessions = %+v", state.Sessions)
	}
}
