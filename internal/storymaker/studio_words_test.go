package storymaker

import (
	"context"
	"testing"
)

func TestLintWordsFindsTellsAndDashes(t *testing.T) {
	german := &Story{Scenes: []Scene{
		{ID: "s1", Narration: "Jeder Moment zählt, dein Büro ist leer.", Beats: []Beat{{Do: "show", Text: "Mehr als nur Papier"}}},
		{ID: "s2", Narration: "Die Seiten 1–3 sind wichtig. Das {1}ist nahtlos — fast."},
	}}
	got := LintWords(german, "de-DE")
	// s1 narration, s1 show text, s2 phrase and s2 em dash. The en dash between 1 and 3 is not flagged.
	if len(got) != 4 {
		t.Fatalf("de: %d issues, want 4: %+v", len(got), got)
	}
	for _, is := range got {
		if is.Code != CodeWordsTell || is.Severity != SeverityWarning || is.Fix == "" {
			t.Errorf("issue %+v is not a words_tell warning with a fix", is)
		}
	}

	english := &Story{Scenes: []Scene{{ID: "s1", Narration: "Little {1}did we know. A breathtaking office."}}}
	if got := LintWords(english, "en"); len(got) != 2 {
		t.Fatalf("en: %d issues, want 2 (a marker inside a phrase does not hide it): %+v", len(got), got)
	}

	// French gets the dash check only, so the German phrases pass and the em dash of s2 is found.
	got = LintWords(german, "fr-FR")
	if len(got) != 1 || got[0].SceneID != "s2" || got[0].Code != CodeWordsTell {
		t.Fatalf("fr: %+v, want one dash issue on s2", got)
	}
}

func TestStudioCheckAdvisesWordsTell(t *testing.T) {
	s, _, _ := fixtureStudio(t, nil)
	s.SetPhase(PhaseOutline, "")
	if res, _ := callOutline(t, s, "concept_explainer", "Jeder Moment zählt, das Büro ist leer.", "Ein zweiter Satz."); res.Status != "ok" {
		t.Fatalf("outline: %s", res.JSON)
	}
	report, err := s.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, is := range report.Advice {
		if is.Code == CodeWordsTell {
			return
		}
	}
	t.Fatalf("advice = %+v, want a words_tell item", report.Advice)
}
