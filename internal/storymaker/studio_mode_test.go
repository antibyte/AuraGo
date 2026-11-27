package storymaker

import (
	"context"
	"errors"
	"slices"
	"testing"
)

// newFreeStudioRig makes a free project with two outline scenes, s1 and s2, and no cast, and a Studio on it.
func newFreeStudioRig(t *testing.T) (*Studio, *ProjectStore, string) {
	t.Helper()
	ctx := context.Background()
	ps, _, _ := newTestStore(t)
	proj, story, _, err := ps.CreateWithOptions(ctx, "Firewall", "", 0, nil, CreateOptions{Mode: ModeFree})
	if err != nil {
		t.Fatal(err)
	}
	story.Scenes = []Scene{
		{ID: "s1", Narration: "Eine Firewall schuetzt das Netz."},
		{ID: "s2", Narration: "Sie prueft jedes Paket."},
	}
	ApplyDefaults(&story)
	data, err := CanonicalJSON(story)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ps.SaveStory(ctx, proj.ID, data, ""); err != nil {
		t.Fatal(err)
	}
	studio, err := NewStudio(StudioDeps{
		Projects: ps,
		Catalog:  loadReal(t),
		Limits:   DefaultLimits(),
		Voices:   func(context.Context) []VoiceProvider { return nil },
		Compile: func(context.Context, string) (CompileSummary, error) {
			return CompileSummary{Available: false, Reason: "no browser in tests"}, nil
		},
		ReadOnly: func() bool { return false },
	}, StudioJob{JobID: "job-1", ProjectID: proj.ID, Kind: AgentKindCreate, Mode: ModeFree, Language: "de", Audience: "general", TargetSeconds: 60, VoiceAvailable: true})
	if err != nil {
		t.Fatal(err)
	}
	return studio, ps, proj.ID
}

func TestFreePhaseTablesOfferOnlyIconSearchAndNoCast(t *testing.T) {
	if got := PhaseToolsFor(ModeFree, PhaseCast); len(got) != 0 {
		t.Fatalf("free cast tools = %v, want none", got)
	}
	for _, phase := range []string{PhaseFill, PhaseCheck, PhaseRevise} {
		want := []string{"search", "reference"}
		if phase == PhaseCheck {
			want = []string{"reference"}
		}
		if got := PhaseOperationsFor(ModeFree, phase, ToolCatalog); !slices.Equal(got, want) {
			t.Errorf("free %s catalog operations = %v, want %v", phase, got, want)
		}
	}
	if slices.Contains(PhaseOperationsFor(ModeFree, PhaseFill, ToolStory), "set_meta") {
		t.Error("free fill must not offer set_meta")
	}
	if !slices.Contains(PhaseOperationsFor(ModeFree, PhaseFill, ToolStory), "put_code") {
		t.Error("free fill must offer put_code")
	}
	if !slices.Equal(PhaseOperationsFor(ModeCatalog, PhaseFill, ToolCatalog), PhaseOperations(PhaseFill, ToolCatalog)) ||
		!slices.Equal(PhaseToolsFor("", PhaseCast), PhaseTools(PhaseCast)) {
		t.Error("the catalog mode must equal the old tables")
	}
}

func TestFreeStudioRefusesCatalogSearchInCheckAndTheCastPhase(t *testing.T) {
	ctx := context.Background()
	s, _, _ := newFreeStudioRig(t)
	s.SetPhase(PhaseCheck, "")
	res := s.Call(ctx, StudioCall{Tool: ToolCatalog, Operation: "search", Args: map[string]any{"kind": "prop", "query": "lock"}})
	if got := answerOf(t, res); got["code"] != "operation_not_in_phase" {
		t.Fatalf("catalog search in a free project: %s", res.JSON)
	}
	s.SetPhase(PhaseCast, "")
	res = s.Call(ctx, StudioCall{Tool: ToolStory, Operation: "set_meta", Args: map[string]any{"data": map[string]any{"title": "x"}}})
	if got := answerOf(t, res); got["code"] != "operation_not_in_phase" {
		t.Fatalf("cast phase in a free project: %s", res.JSON)
	}
}

func TestNewStudioRejectsAnUnknownMode(t *testing.T) {
	ps, _, _ := newTestStore(t)
	proj, _, _, err := ps.Create(context.Background(), "Firewall", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewStudio(StudioDeps{Projects: ps}, StudioJob{JobID: "job-1", ProjectID: proj.ID, Kind: AgentKindCreate, Mode: "junk"})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("NewStudio with mode junk = %v, want ErrInvalidArgument", err)
	}
}
