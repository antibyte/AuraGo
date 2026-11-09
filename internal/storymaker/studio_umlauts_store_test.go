package storymaker

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func storedScene(t *testing.T, ps *ProjectStore, projectID, sceneID string) Scene {
	t.Helper()
	raw, _, err := ps.LoadStory(context.Background(), projectID)
	if err != nil {
		t.Fatal(err)
	}
	s := mustDecode(t, string(raw))
	for _, sc := range s.Scenes {
		if sc.ID == sceneID {
			return sc
		}
	}
	t.Fatalf("no scene %s", sceneID)
	return Scene{}
}

func TestSceneWriteRepairsUmlautsAndTellsTheModel(t *testing.T) {
	studio, ps, projectID := newStudioWriteRig(t, nil)
	studio.SetPhase(PhaseFill, "s1")
	put := studioWriteCall(t, studio, ToolStory, "put_scene",
		`{"scene_id":"s1","data":{"environment":"env.office","speaker":"anna","narration":"Eine {1}Firewall pruft jedes Paket.","actors":[{"cast":"anna","at":"left","pose":"present"}],"beats":[{"do":"show","text":"Pruft alles","at":"right","entry":"pop","cue":1}]}}`)
	if put["status"] != "ok" {
		t.Fatalf("put_scene = %v", put)
	}
	fixes, _ := put["repaired"].([]any)
	if len(fixes) != 2 || put["repaired_note"] == nil {
		t.Fatalf("the answer must carry repaired and a note: %v", put)
	}
	sc := storedScene(t, ps, projectID, "s1")
	if sc.Narration != "Eine {1}Firewall prüft jedes Paket." || sc.Beats[0].Text != "Prüft alles" {
		t.Fatalf("stored narration %q, beat text %q", sc.Narration, sc.Beats[0].Text)
	}
	if studio.Repairs() != 2 {
		t.Fatalf("Repairs = %d, want 2", studio.Repairs())
	}

	// patch_scene repairs the new narration too.
	patch := studioWriteCall(t, studio, ToolStory, "patch_scene", `{"scene_id":"s1","data":{"narration":"Sie durfen {1}wahrend der Zeit durch."}}`)
	if patch["status"] != "ok" || patch["repaired"] == nil {
		t.Fatalf("patch_scene = %v", patch)
	}
	if got := storedScene(t, ps, projectID, "s1").Narration; got != "Sie dürfen {1}während der Zeit durch." {
		t.Fatalf("stored narration %q", got)
	}
	if studio.Repairs() != 4 {
		t.Fatalf("Repairs = %d, want 4", studio.Repairs())
	}

	// A text without a loss has no repair note and does not change the count.
	clean := studioWriteCall(t, studio, ToolStory, "patch_scene", `{"scene_id":"s1","data":{"narration":"Eine {1}Firewall prüft jedes Paket."}}`)
	if clean["repaired"] != nil {
		t.Fatalf("no repair expected: %v", clean)
	}
	if studio.Repairs() != 4 {
		t.Fatalf("Repairs = %d, want 4", studio.Repairs())
	}
}

func TestSceneWriteLeavesOtherLanguagesAndUeSpellingsAlone(t *testing.T) {
	studio, ps, projectID := newStudioWriteRig(t, nil)
	studio.SetPhase(PhaseFill, "s1")
	studio.job.Language = "en"
	put := studioWriteCall(t, studio, ToolStory, "put_scene",
		`{"scene_id":"s1","data":{"environment":"env.office","speaker":"anna","narration":"The firewall pruft each {1}packet.","actors":[{"cast":"anna","at":"left","pose":"present"}],"beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop","cue":1}]}}`)
	if put["status"] != "ok" || put["repaired"] != nil {
		t.Fatalf("an English job repairs nothing: %v", put)
	}
	if got := storedScene(t, ps, projectID, "s1").Narration; got != "The firewall pruft each {1}packet." {
		t.Fatalf("stored narration %q", got)
	}
	studio.job.Language = "de"
	put = studioWriteCall(t, studio, ToolStory, "patch_scene", `{"scene_id":"s1","data":{"narration":"Eine {1}Firewall prueft und schuetzt das Netz."}}`)
	if put["repaired"] != nil {
		t.Fatalf("the ue spelling is left alone: %v", put)
	}
}

// A refused outline call can leave repairs pending (the first scene was repaired, the second id is a duplicate). A later
// scene write without a loss must not report or count them. A one-scene outline would not do: it is refused before any text
// is repaired.
func TestSceneWriteDropsRepairsLeftByARefusedOutline(t *testing.T) {
	studio, _, _ := newStudioWriteRig(t, nil)
	studio.SetPhase(PhaseOutline, "")
	refused := studioWriteCall(t, studio, ToolStory, "outline",
		`{"data":{"template":"concept_explainer","target_seconds":60,"scenes":[{"id":"s1","narration":"Eine Firewall pruft das."},{"id":"s1","narration":"Noch eine Zeile."}]}}`)
	if refused["status"] == "ok" {
		t.Fatalf("the outline must be refused: %v", refused)
	}
	studio.SetPhase(PhaseFill, "s1")
	clean := studioWriteCall(t, studio, ToolStory, "patch_scene", `{"scene_id":"s1","data":{"narration":"Eine {1}Firewall prüft das."}}`)
	if clean["status"] != "ok" {
		t.Fatalf("patch_scene = %v", clean)
	}
	if _, has := clean["repaired"]; has {
		t.Fatalf("stale repairs leaked into a clean scene write: %v", clean)
	}
	if studio.Repairs() != 0 {
		t.Fatalf("Repairs = %d, want 0", studio.Repairs())
	}
}

// A write that is refused after the repair step stored nothing: it neither counts nor carries a repaired note. A code scene
// without a bundle is such a refusal (writeScene repairs the fields first and checks the bundle afterwards).
func TestSceneWriteRefusedAfterTheRepairStepCountsNothing(t *testing.T) {
	studio, _, _ := newStudioWriteRig(t, nil)
	studio.SetPhase(PhaseFill, "s1")
	refused := studioWriteCall(t, studio, ToolStory, "put_scene", `{"scene_id":"s1","data":{"kind":"code","narration":"Eine Firewall pruft jedes Paket."}}`)
	if refused["status"] != "refused" || refused["code"] != CodeCodeError {
		t.Fatalf("a code scene without a bundle must be refused: %v", refused)
	}
	if _, has := refused["repaired"]; has {
		t.Fatalf("a refused write carries no repaired key: %v", refused)
	}
	if studio.Repairs() != 0 {
		t.Fatalf("Repairs = %d after a refused write, want 0", studio.Repairs())
	}

	// The same text as a valid scene is stored, reported and counted once.
	put := studioWriteCall(t, studio, ToolStory, "put_scene",
		`{"scene_id":"s1","data":{"environment":"env.office","speaker":"anna","narration":"Eine {1}Firewall pruft jedes Paket.","actors":[{"cast":"anna","at":"left","pose":"present"}],"beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop","cue":1}]}}`)
	if put["status"] != "ok" {
		t.Fatalf("put_scene = %v", put)
	}
	if fixes, _ := put["repaired"].([]any); len(fixes) != 1 {
		t.Fatalf("the stored write must report one repair: %v", put)
	}
	if studio.Repairs() != 1 {
		t.Fatalf("Repairs = %d, want 1", studio.Repairs())
	}
}

// The answer lists at most 20 distinct repairs and says how many there were in all, for a scene write and for the outline.
func TestRepairedListIsCappedAndTotalled(t *testing.T) {
	var keys []string
	for key := range germanUmlautMap() {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	if len(keys) < 25 {
		t.Fatalf("the map has only %d entries", len(keys))
	}
	text := strings.Join(keys[:25], " ") + "."
	check := func(name string, answer map[string]any) {
		t.Helper()
		if answer["status"] != "ok" {
			t.Fatalf("%s = %v", name, answer)
		}
		if listed, _ := answer["repaired"].([]any); len(listed) != 20 {
			t.Fatalf("%s lists %d repairs, want 20", name, len(listed))
		}
		if answer["repaired_total"] != float64(25) || answer["repaired_note"] == nil {
			t.Fatalf("%s: repaired_total %v, note %v", name, answer["repaired_total"], answer["repaired_note"])
		}
	}

	studio, _, _ := newStudioWriteRig(t, nil)
	studio.SetPhase(PhaseFill, "s1")
	check("put_scene", studioWriteCall(t, studio, ToolStory, "put_scene", `{"scene_id":"s1","data":{"environment":"env.office","narration":"`+text+`"}}`))
	if studio.Repairs() != 25 {
		t.Fatalf("Repairs = %d, want 25: the cap limits the list, not the count", studio.Repairs())
	}

	studio, _, _ = newStudioWriteRig(t, nil)
	studio.SetPhase(PhaseOutline, "")
	check("outline", studioWriteCall(t, studio, ToolStory, "outline",
		`{"data":{"template":"concept_explainer","target_seconds":60,"scenes":[{"id":"s1","narration":"`+text+`"},{"id":"s2","narration":"Sie darf Pakete durchlassen, die erlaubt sind."},{"id":"s3","narration":"Das ist der ganze Tag."}]}}`))
	if studio.Repairs() != 25 {
		t.Fatalf("outline Repairs = %d, want 25", studio.Repairs())
	}
}

func TestOutlineRepairsUmlautsBeforeTheWordCount(t *testing.T) {
	studio, ps, projectID := newStudioWriteRig(t, nil)
	studio.SetPhase(PhaseOutline, "")
	out := studioWriteCall(t, studio, ToolStory, "outline",
		`{"data":{"template":"concept_explainer","target_seconds":60,"scenes":[{"id":"s1","narration":"Eine Firewall pruft jedes Paket."},{"id":"s2","narration":"Sie darf Pakete durchlassen, die erlaubt sind."},{"id":"s3","narration":"Das schutzt dich wahrend des ganzen Tages."}]}}`)
	if out["status"] != "ok" || out["repaired"] == nil {
		t.Fatalf("outline = %v", out)
	}
	if got := storedScene(t, ps, projectID, "s1").Narration; got != "Eine Firewall prüft jedes Paket." {
		t.Fatalf("stored outline narration %q", got)
	}
	if got := storedScene(t, ps, projectID, "s3").Narration; got != "Das schutzt dich während des ganzen Tages." {
		t.Fatalf("stored outline narration %q (schutzt is a word of its own and stays)", got)
	}
	if studio.Repairs() != 2 {
		t.Fatalf("Repairs = %d, want 2", studio.Repairs())
	}
}
