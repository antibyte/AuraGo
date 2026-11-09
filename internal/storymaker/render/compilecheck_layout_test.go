package render

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// goodReport() has s1 and s2 as storyboard scenes and s3 as a code scene.
func reportWithLayout(t *testing.T, extra map[string]any) []byte {
	t.Helper()
	m := goodReport()
	m["layout"] = []any{
		map[string]any{"id": "s1", "index": 0, "hold_s": 1.2, "actors": []any{map[string]any{"id": "anna", "box": []float64{0.2, 0.3, 0.1, 0.45}}},
			"props": []any{}, "texts": []any{}, "caption": []float64{0.2, 0.79, 0.6, 0.13}, "missing": 0, "static": false},
		map[string]any{"id": "s2", "index": 1, "hold_s": 2, "actors": []any{}, "props": []any{}, "texts": []any{}, "caption": nil, "missing": 0, "static": true},
		map[string]any{"id": "ghost", "index": 9, "actors": []any{}, "props": []any{}, "texts": []any{}},
		map[string]any{"id": "s1", "index": 0, "actors": []any{}, "props": []any{}, "texts": []any{}},
	}
	for k, v := range extra {
		m[k] = v
	}
	body, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestSummarizeCompileReportKeepsTheLayoutOfTheStoryScenes(t *testing.T) {
	got := summarizeCompileReport(reportWithLayout(t, nil), reportExpectation(), 0)
	if !got.Available {
		t.Fatalf("summary = %+v", got)
	}
	if got.StoryboardScenes != 2 || got.CodeScenes != 1 {
		t.Fatalf("scene kinds: storyboard %d code %d", got.StoryboardScenes, got.CodeScenes)
	}
	if len(got.Layout) != 2 || got.Layout[0].ID != "s1" || got.Layout[1].ID != "s2" {
		t.Fatalf("a block for an unknown scene and a repeated block are dropped: %+v", got.Layout)
	}
	if got.Layout[0].Caption == nil || got.Layout[0].Caption[1] != 0.79 || got.Layout[1].Caption != nil || !got.Layout[1].Static {
		t.Fatalf("layout = %+v", got.Layout)
	}
}

func TestSummarizeCompileReportWithoutLayoutOrWithAnError(t *testing.T) {
	plain, _ := json.Marshal(goodReport())
	got := summarizeCompileReport(plain, reportExpectation(), 0)
	if !got.Available || len(got.Layout) != 0 || got.StoryboardScenes != 2 || got.LayoutError != "" {
		t.Fatalf("a report without layout: %+v", got)
	}
	failed := summarizeCompileReport(reportWithLayout(t, map[string]any{"layout": nil, "layout_error": "The layout run took too long."}), reportExpectation(), 0)
	if failed.LayoutError != "The layout run took too long." || len(failed.Layout) != 0 {
		t.Fatalf("layout error: %+v", failed)
	}
}

func TestBootDocAsksForLayoutOnlyWhenTheParamsDo(t *testing.T) {
	on, _ := json.Marshal(bootDoc{Layout: true})
	off, _ := json.Marshal(bootDoc{})
	if string(on) == string(off) {
		t.Fatal("layout must change the boot document")
	}
	var m map[string]any
	_ = json.Unmarshal(off, &m)
	if _, present := m["layout"]; present {
		t.Fatalf("an export's boot.json must not mention layout: %s", off)
	}
}

// The ids in a layout block are strings the page wrote. They reach the model through the findings, so they lose the render
// prefix and token like every other page text, and the number of boxes per block is bounded.
func TestSummarizeCompileReportCleansAndBoundsTheLayoutBlocks(t *testing.T) {
	want := reportExpectation()
	want.Prefix, want.Token = "/r/job-7f3a/", "tok9d2c"
	box := []float64{0.1, 0.1, 0.1, 0.1}
	many := func(n int, mk func(i int) map[string]any) []any {
		out := make([]any, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, mk(i))
		}
		return out
	}
	m := goodReport()
	m["layout"] = []any{
		map[string]any{"id": "s1", "index": 0, "hold_s": 1,
			"actors": []any{
				map[string]any{"id": "an" + want.Token + "na", "box": box},
				map[string]any{"id": "ben" + want.Prefix, "box": box},
				map[string]any{"id": strings.Repeat("x", 100), "box": box},
			},
			"props": []any{map[string]any{"id": "p" + want.Prefix + "1", "item": "prop." + want.Token + "lock", "scale": 1, "box": box}},
			"texts": []any{map[string]any{"id": "t" + want.Token + want.Prefix + "1", "box": box}}},
		map[string]any{"id": "s2", "index": 1, "hold_s": 1,
			"actors": many(100, func(i int) map[string]any { return map[string]any{"id": fmt.Sprintf("a%d", i), "box": box} }),
			"props": many(100, func(i int) map[string]any {
				return map[string]any{"id": fmt.Sprintf("p%d", i), "item": "prop.lock", "box": box}
			}),
			"texts": many(100, func(i int) map[string]any { return map[string]any{"id": fmt.Sprintf("t%d", i), "box": box} })},
	}
	body, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	got := summarizeCompileReport(body, want, 0)
	if !got.Available || len(got.Layout) != 2 {
		t.Fatalf("summary = %+v", got)
	}
	first := got.Layout[0]
	if first.Actors[0].ID != "anna" || first.Actors[1].ID != "ben" || first.Actors[2].ID != strings.Repeat("x", 60)+"..." {
		t.Fatalf("actor ids: %+v", first.Actors)
	}
	if first.Props[0].ID != "p1" || first.Props[0].Item != "prop.lock" || first.Texts[0].ID != "t1" {
		t.Fatalf("prop and text ids: %+v %+v", first.Props, first.Texts)
	}
	if first.Actors[0].Box != [4]float64{0.1, 0.1, 0.1, 0.1} || first.Props[0].Scale != 1 {
		t.Fatalf("the geometry stays: %+v", first)
	}
	second := got.Layout[1]
	if len(second.Actors) != 64 || len(second.Props) != 64 || len(second.Texts) != 64 {
		t.Fatalf("a block is cut to 64 boxes: %d actors, %d props, %d texts", len(second.Actors), len(second.Props), len(second.Texts))
	}
	if second.Actors[63].ID != "a63" || second.Props[0].ID != "p0" || second.Texts[63].ID != "t63" {
		t.Fatalf("the first 64 are kept: %+v", second.Actors[63])
	}
}
