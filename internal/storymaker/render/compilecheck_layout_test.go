package render

import (
	"encoding/json"
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
