package storymaker

import "testing"

func TestPutSceneRemovesClosingMarkers(t *testing.T) {
	studio, ps, projectID := newStudioWriteRig(t, nil)
	studio.job.Language = "en"
	studio.SetPhase(PhaseFill, "s1")
	out := studioWriteCall(t, studio, ToolStory, "put_scene", `{"scene_id":"s1","data":{"environment":"env.office","narration":"Daten{/1}. Mehr {1}Text"}}`)
	if out["status"] == "refused" {
		t.Fatalf("put_scene = %v", out)
	}
	if got := storedScene(t, ps, projectID, "s1").Narration; got != "Daten. Mehr {1}Text" {
		t.Fatalf("stored narration %q", got)
	}

	studio, ps, projectID = newStudioWriteRig(t, nil)
	studio.job.Language = "en"
	studio.SetPhase(PhaseOutline, "")
	out = studioWriteCall(t, studio, ToolStory, "outline",
		`{"data":{"template":"concept_explainer","target_seconds":60,"scenes":[{"id":"s1","narration":"Eine Firewall pruft jedes Paket."},{"id":"s2","narration":"Sie darf Pakete{/2} durchlassen, die erlaubt sind."},{"id":"s3","narration":"Das ist der ganze Tag."}]}}`)
	if out["status"] != "ok" {
		t.Fatalf("outline = %v", out)
	}
	if got := storedScene(t, ps, projectID, "s2").Narration; got != "Sie darf Pakete durchlassen, die erlaubt sind." {
		t.Fatalf("stored outline narration %q", got)
	}
}

func TestRepairTextClosingMarkerSpacing(t *testing.T) {
	studio, _, _ := newStudioWriteRig(t, nil)
	studio.job.Language = "en"
	for in, want := range map[string]string{
		"Daten {/1}.":            "Daten.",
		"Daten{/1}. Mehr":        "Daten. Mehr",
		"Mehr {1}Text {/1}hier":  "Mehr {1}Text hier",
		"a {/2} b":               "a b",
		"Pakete{/2} durchlassen": "Pakete durchlassen",
		"Ende {/3}":              "Ende",
		"Wort{/1}Wort":           "WortWort",
	} {
		if got := studio.repairText(in); got != want {
			t.Errorf("repairText(%q) = %q, want %q", in, got, want)
		}
	}
}
