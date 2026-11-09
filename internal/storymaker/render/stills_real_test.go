package render

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/storymaker"
)

// TestCompileCheckStillsOfTheRealRuns renders the compile check with stills for three stories and checks the geometry that
// batches B and D promise: the real firewall run without composition v2 has the caption over the feet, the same run with v2 has
// the feet above the caption, and the varied story has no run of look-alike scenes. The contact sheets go to
// AURAGO_STILLS_OUT when it is set, so they can be looked at.
func TestCompileCheckStillsOfTheRealRuns(t *testing.T) {
	chrome := browserSmoke(t)
	fx := newRealFixture(t, chrome)
	fx.rebuild(t)
	out := os.Getenv("AURAGO_STILLS_OUT")
	ctx := context.Background()

	load := func(name string, compose string) []byte {
		raw, err := os.ReadFile(filepath.Join("..", "testdata", "stories", name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		delete(doc, "voice")
		delete(doc, "music")
		style, _ := doc["style"].(map[string]any)
		if compose != "" {
			style["compose"] = compose
		} else {
			delete(style, "compose")
		}
		data, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	for _, tc := range []struct{ name, story, compose string }{
		{"firewall-v1", "agent-firewall-run", ""},
		{"firewall-v2", "agent-firewall-run", "v2"},
		{"varied-v2", "agent-varied", "v2"},
	} {
		if _, err := fx.projects.SaveStory(ctx, fx.projectID, load(tc.story, tc.compose), ""); err != nil {
			t.Fatal(err)
		}
		settings := fx.settings
		settings.Stills = true
		got, err := fx.runner.CompileCheck(ctx, fx.projectID, settings)
		if err != nil {
			t.Fatal(err)
		}
		if !got.Available || got.LayoutError != "" || len(got.Layout) != 7 || len(got.Stills) != 7 || len(got.Sheet) == 0 {
			t.Fatalf("%s: available %v, layout error %q, %d layout blocks, %d stills, sheet %d bytes, issues %+v", tc.name, got.Available, got.LayoutError, len(got.Layout), len(got.Stills), len(got.Sheet), got.Issues)
		}
		if out != "" {
			if err := os.WriteFile(filepath.Join(out, tc.name+".png"), got.Sheet, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		for i := 1; i < len(got.Stills); i++ {
			t.Logf("%s: scenes %s and %s differ by %d of 64 bits", tc.name, got.Stills[i-1].SceneID, got.Stills[i].SceneID, storymaker.HashDistance(got.Stills[i-1].Hash, got.Stills[i].Hash))
		}
		issues := storymaker.LayoutIssues(got.Layout, storymaker.LayoutOptions{CaptionRule: true})
		overlaps, others := 0, 0
		for _, is := range issues {
			if is.Code == storymaker.CodeCaptionOverlap {
				overlaps++
			} else {
				others++
			}
			t.Logf("%s: %s in %s: %s", tc.name, is.Code, is.SceneID, is.Message)
		}
		lookAlike := storymaker.LookAlikeIssues(got.Stills)
		t.Logf("%s: %d caption overlaps, %d other layout findings, %d look-alike runs", tc.name, overlaps, others, len(lookAlike))
		switch tc.name {
		case "firewall-v1":
			if overlaps == 0 {
				t.Errorf("a story without composition v2 has its feet in the caption band: expected caption_overlap findings")
			}
			if len(lookAlike) == 0 {
				t.Errorf("the real run has seven alike scenes: expected scenes_look_alike")
			}
		case "firewall-v2":
			if overlaps != 0 {
				t.Errorf("composition v2 must keep feet and prop bases out of the caption band, got %d overlaps", overlaps)
			}
			if len(lookAlike) == 0 {
				t.Errorf("the same seven scenes under v2 still share one stage and one actor spot: expected scenes_look_alike")
			}
		case "varied-v2":
			if overlaps != 0 {
				t.Errorf("the varied story under v2 has %d caption overlaps", overlaps)
			}
			if len(lookAlike) != 0 {
				t.Errorf("the varied story must not be reported as look-alike: %+v", lookAlike)
			}
		}
	}
}
