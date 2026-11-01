package personabooster

import (
	"strings"
	"testing"
)

func TestRenderPhaseBlock(t *testing.T) {
	cat := engineCatalog(t)
	state := State{Persona: "sample", PhaseID: "rage", VariantID: "cold"}
	want := "Temporary phase: Rage – Cold. For tone it takes precedence over the mood directives above.\nYour anger has gone cold.\n" + LimitsLine
	if got := RenderPhase(cat, state, IntensityStrong); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	if wild := RenderPhase(cat, state, IntensityWild); !strings.Contains(wild, "Your anger has gone cold.\n"+WildLine+"\n"+LimitsLine) {
		t.Fatalf("wild block:\n%s", wild)
	}
	if RenderPhase(cat, State{Persona: "sample"}, IntensityStrong) != "" || RenderPhase(cat, State{PhaseID: "gone", VariantID: "x"}, IntensityStrong) != "" {
		t.Fatal("inactive or unknown phases must render nothing")
	}
}

func TestRenderAmplifier(t *testing.T) {
	cat := engineCatalog(t)
	if got := RenderAmplifier(cat, IntensityStrong); got != "Speak with more edge than usual." {
		t.Fatalf("amplifier = %q", got)
	}
	if RenderAmplifier(cat, IntensityMild) != "" || RenderAmplifier(nil, IntensityWild) != "" {
		t.Fatal("mild intensity and missing catalogs drop the amplifier")
	}
}
