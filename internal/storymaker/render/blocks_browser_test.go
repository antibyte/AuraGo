package render

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/storymaker"
)

// TestCompileCheckBlocksBrowser renders the fixture film blocks.json (most block kinds, a board, an office pinboard and a code
// scene) in 16:9 and 9:16 with stills and checks that no block is off screen, too small or under the caption. The contact sheets
// go to AURAGO_STILLS_OUT when it is set, so they can be looked at.
func TestCompileCheckBlocksBrowser(t *testing.T) {
	chrome := browserSmoke(t)
	fx := newRealFixture(t, chrome)
	fx.rebuild(t)
	out := os.Getenv("AURAGO_STILLS_OUT")
	ctx := context.Background()
	lf := func(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }
	raw, err := os.ReadFile(filepath.Join("..", "testdata", "stories", "blocks.json"))
	if err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join("..", "testdata", "scenes", "blocks.js"))
	if err != nil {
		t.Fatal(err)
	}
	for _, aspect := range []string{"16:9", "9:16"} {
		var doc map[string]any
		if err := json.Unmarshal(lf(raw), &doc); err != nil {
			t.Fatal(err)
		}
		doc["format"].(map[string]any)["aspect"] = aspect
		if aspect == "9:16" {
			// Two blocks side by side are too narrow in portrait: the pair scenes (4 and 7) show their first block at full.
			scenes := doc["scenes"].([]any)
			for _, i := range []int{3, 6} {
				sc := scenes[i].(map[string]any)
				beats := sc["beats"].([]any)[:1]
				beats[0].(map[string]any)["at"] = "full"
				sc["beats"] = beats
			}
		}
		data, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fx.projects.SaveStory(ctx, fx.projectID, data, ""); err != nil {
			t.Fatal(err)
		}
		if _, res, err := fx.projects.SaveCodeScene(ctx, fx.projectID, "s8", lf(src), ""); err != nil || len(res.Errors) != 0 {
			t.Fatalf("%s: SaveCodeScene: %v %+v", aspect, err, res)
		}
		settings := fx.settings
		settings.Stills = true
		got, err := fx.runner.CompileCheck(ctx, fx.projectID, settings)
		if err != nil {
			t.Fatal(err)
		}
		if !got.Available || got.LayoutError != "" || len(got.Layout) != 8 || len(got.Stills) != 8 || len(got.Sheet) == 0 {
			t.Fatalf("%s: reason %q available %v, layout error %q, %d layout scenes, %d stills, sheet %d bytes, issues %+v", aspect, got.Reason, got.Available, got.LayoutError, len(got.Layout), len(got.Stills), len(got.Sheet), got.Issues)
		}
		for _, is := range got.Issues {
			if is.Severity == storymaker.SeverityError {
				t.Errorf("%s: error issue %s in %s: %s", aspect, is.Code, is.SceneID, is.Message)
			}
		}
		for i, sc := range got.Layout {
			t.Logf("%s: %s code %v, %d blocks: %+v", aspect, sc.ID, sc.Code, len(sc.Blocks), sc.Blocks)
			if len(sc.Blocks) == 0 {
				t.Errorf("%s: scene %d (%s) has no block", aspect, i+1, sc.ID)
			}
			if (i == 7) != sc.Code {
				t.Errorf("%s: scene %d (%s) code = %v", aspect, i+1, sc.ID, sc.Code)
			}
		}
		if out != "" {
			if err := os.WriteFile(filepath.Join(out, "blocks-"+strings.ReplaceAll(aspect, ":", "x")+".png"), got.Sheet, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		story, _, err := storymaker.DecodeStory(data, storymaker.DefaultLimits())
		if err != nil {
			t.Fatal(err)
		}
		storymaker.ApplyDefaults(&story)
		for _, is := range storymaker.LayoutIssues(got.Layout, storymaker.StoryLayoutOptions(&story)) {
			t.Logf("%s: %s in %s: %s", aspect, is.Code, is.SceneID, is.Message)
			switch is.Code {
			case storymaker.CodeOffscreen, storymaker.CodeBlockTooSmall:
				t.Errorf("%s: %s in %s: %s", aspect, is.Code, is.SceneID, is.Message)
			case storymaker.CodeCaptionOverlap:
				if strings.Contains(is.Message, "block") {
					t.Errorf("%s: %s in %s: %s", aspect, is.Code, is.SceneID, is.Message)
				}
			}
		}
	}
}
