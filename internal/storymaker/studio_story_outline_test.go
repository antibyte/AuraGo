package storymaker

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
)

func callOutline(t *testing.T, s *Studio, template string, narrations ...string) (StudioResult, map[string]any) {
	t.Helper()
	scenes := make([]any, len(narrations))
	for i, narration := range narrations {
		scenes[i] = map[string]any{"id": fmt.Sprintf("s%d", i+1), "narration": narration}
	}
	res := s.Call(context.Background(), StudioCall{Tool: ToolStory, Operation: "outline", Args: map[string]any{
		"data": map[string]any{"template": template, "target_seconds": 60, "scenes": scenes},
	}})
	return res, answerOf(t, res)
}

func outlineRowAt(answer map[string]any, i int) map[string]any {
	return answer["scenes"].([]any)[i].(map[string]any)
}

func TestStudioOutlineBudgetsAndLock(t *testing.T) {
	ctx := context.Background()
	// 60 s at 2.3 words per second is 138 target words. 138 / 19 rounds to 7 scenes.
	wantWords := math.Round(60 * 2.3)
	wantScenes := math.Round(wantWords / 19)

	s, ps, id := fixtureStudio(t, nil)
	s.SetPhase(PhaseOutline, "")
	if s.PhaseDone() {
		t.Fatal("the phase is done before an outline")
	}
	// Placeholder narration is refused, and the phase stays open.
	if res, answer := callOutline(t, s, "concept_explainer", "...", "Vier fuenf"); res.Status != "refused" || answer["code"] != "invalid_data" || s.PhaseDone() {
		t.Fatalf("placeholder narration: %s", res.JSON)
	}
	res, answer := callOutline(t, s, "concept_explainer", "Eins zwei drei", "Vier fuenf")
	if res.Status != "ok" || answer["rev"] == "" || answer["target_words"] != wantWords || answer["suggested_scenes"] != wantScenes || answer["total_words"] != float64(5) {
		t.Fatalf("outline: %s", res.JSON)
	}
	first, second := outlineRowAt(answer, 0), outlineRowAt(answer, 1)
	if first["id"] != "s1" || first["words"] != float64(3) || first["word_budget"] != float64(19) || second["words"] != float64(2) {
		t.Fatalf("rows: %s", res.JSON)
	}
	if !s.PhaseDone() {
		t.Error("an accepted outline must end the phase")
	}
	proj, err := ps.Get(ctx, id)
	if err != nil || proj.TargetSeconds != 60 || proj.Audience != "Einsteiger" || proj.Template != "concept_explainer" {
		t.Errorf("project record: %+v (%v)", proj, err)
	}

	// Other templates divide the target words by the scenes: 138 / 2 is 69.
	res, answer = callOutline(t, s, "how_to", "Eins zwei drei", "Vier fuenf")
	if res.Status != "ok" || outlineRowAt(answer, 0)["word_budget"] != math.Round(wantWords/2) {
		t.Fatalf("how_to: %s", res.JSON)
	}

	data, etag, err := ps.LoadStory(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	st := mustDecode(t, string(data))
	if len(st.Scenes) != 2 || st.Scenes[1].ID != "s2" || st.Scenes[1].Narration != "Vier fuenf" {
		t.Fatalf("saved scenes: %+v", st.Scenes)
	}
	st.Scenes[0].Beats = []Beat{{Do: "wait", Seconds: 1}}
	filled, err := CanonicalJSON(st)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ps.SaveStory(ctx, id, filled, etag); err != nil {
		t.Fatal(err)
	}
	res, answer = callOutline(t, s, "how_to", "Eins", "Zwei")
	if res.Status != "refused" || answer["code"] != "outline_locked" {
		t.Fatalf("a filled scene must lock the outline: %s", res.JSON)
	}

	// 138 * 1.15 is 158.7 words, so 170 words are too long: refused twice, accepted the third time.
	long, ps2, id2 := fixtureStudio(t, nil)
	long.SetPhase(PhaseOutline, "")
	words := strings.TrimSpace(strings.Repeat("wort ", 85))
	for attempt := 1; attempt <= 2; attempt++ {
		res, answer = callOutline(t, long, "concept_explainer", words, words)
		if res.Status != "refused" || answer["code"] != CodeOverBudget || answer["total_words"] != float64(170) || outlineRowAt(answer, 0)["words"] != float64(85) {
			t.Fatalf("attempt %d: %s", attempt, res.JSON)
		}
		if long.PhaseDone() {
			t.Fatalf("attempt %d: a refused outline must not end the phase", attempt)
		}
	}
	data, _, err = ps2.LoadStory(ctx, id2)
	if err != nil {
		t.Fatal(err)
	}
	if got := mustDecode(t, string(data)); len(got.Scenes) != 1 {
		t.Fatalf("a refused outline changed the story: %+v", got.Scenes)
	}
	res, answer = callOutline(t, long, "concept_explainer", words, words)
	if res.Status != "ok" || !strings.Contains(string(res.JSON), `"code":"`+CodeOverTarget+`"`) {
		t.Fatalf("third attempt: %s", res.JSON)
	}
	if !long.PhaseDone() {
		t.Error("the accepted third attempt must end the phase")
	}
}
