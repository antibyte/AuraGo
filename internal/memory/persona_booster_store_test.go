package memory

import (
	"testing"
	"time"

	"aurago/internal/personabooster"
)

func TestPersonaBoosterStateRoundTripUpdateAndReset(t *testing.T) {
	stm := newTestPersonalityDB(t)
	if state, err := stm.LoadPersonaBoosterState(); err != nil || state.Persona != "" {
		t.Fatalf("fresh state = %+v, err = %v", state, err)
	}
	now := time.Date(2026, 11, 1, 22, 0, 0, 0, time.UTC)
	want := personabooster.State{
		Persona: "punk", PhaseID: "rage", VariantID: "cold_fury", Trigger: personabooster.CueCriticism,
		Priority: 70, Hostile: true, StartedAt: now, ExpiresAt: now.Add(30 * time.Minute), TurnsLeft: 6,
		LastVariant: map[string]string{"rage": "cold_fury"},
	}
	if err := stm.SavePersonaBoosterState(want); err != nil {
		t.Fatal(err)
	}
	got, err := stm.LoadPersonaBoosterState()
	if err != nil || got.PhaseID != "rage" || !got.ExpiresAt.Equal(want.ExpiresAt) || got.LastVariant["rage"] != "cold_fury" {
		t.Fatalf("loaded = %+v, err = %v", got, err)
	}
	updated, err := stm.UpdatePersonaBoosterState(func(s personabooster.State) personabooster.State {
		s.TurnsLeft--
		return s
	})
	if err != nil || updated.TurnsLeft != 5 {
		t.Fatalf("updated = %+v, err = %v", updated, err)
	}
	if again, _ := stm.LoadPersonaBoosterState(); again.TurnsLeft != 5 {
		t.Fatalf("update not persisted: %+v", again)
	}
	if _, err := stm.ResetPersonalityDynamics(now); err != nil {
		t.Fatal(err)
	}
	if cleared, err := stm.LoadPersonaBoosterState(); err != nil || cleared.Persona != "" || cleared.PhaseID != "" {
		t.Fatalf("reset kept the booster state: %+v, err = %v", cleared, err)
	}
}

func TestPersonaBoosterStateWorksWithoutPersonalityTables(t *testing.T) {
	stm := newInMemorySQLiteMemory(t)
	if _, err := stm.UpdatePersonaBoosterState(func(s personabooster.State) personabooster.State {
		s.Persona = "punk"
		return s
	}); err != nil {
		t.Fatal(err)
	}
	if got, err := stm.LoadPersonaBoosterState(); err != nil || got.Persona != "punk" {
		t.Fatalf("state = %+v, err = %v", got, err)
	}
}
