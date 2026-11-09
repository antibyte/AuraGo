package render

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"reflect"
	"testing"

	"aurago/internal/storymaker"
)

func stillBytes(t *testing.T, seed int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 36))
	for y := 0; y < 36; y++ {
		for x := 0; x < 64; x++ {
			v := uint8((x*4 + seed*40) % 256)
			img.Set(x, y, color.RGBA{v, v, v, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestStillFramesPicksTheHoldFrameInsideTheScene(t *testing.T) {
	summary := storymaker.CompileSummary{
		DurationMS: 6000,
		Scenes:     []storymaker.CompileScene{{ID: "s1", StartMS: 0, EndMS: 2000}, {ID: "s2", StartMS: 1800, EndMS: 6000}, {ID: "s3", StartMS: 5800, EndMS: 6000}},
		Layout: []storymaker.LayoutScene{
			{ID: "s1", Index: 0, HoldS: 1.0}, {ID: "s2", Index: 1, HoldS: 99}, {ID: "ghost", Index: 2, HoldS: 1}, {ID: "s3", Index: 3, HoldS: 0.1},
		},
	}
	frames, ids, indexes := stillFrames(summary, 30, 0)
	// s1: 30. s2: the hold is clamped to the last frame of the scene, 179. ghost: not in the summary. s3: starts at frame 174, hold 3 frames.
	if !reflect.DeepEqual(frames, []int{30, 179, 177}) || !reflect.DeepEqual(ids, []string{"s1", "s2", "s3"}) || !reflect.DeepEqual(indexes, []int{0, 1, 3}) {
		t.Fatalf("frames %v ids %v indexes %v", frames, ids, indexes)
	}
	// A page that offers only 100 frames cannot show frame 179.
	frames, ids, _ = stillFrames(summary, 30, 100)
	if !reflect.DeepEqual(frames, []int{30}) || !reflect.DeepEqual(ids, []string{"s1"}) {
		t.Fatalf("with a frame limit of 100: frames %v ids %v", frames, ids)
	}
}

func TestCompileCheckTakesStillsInTheSameRun(t *testing.T) {
	var captured []int
	page := &fakePage{crashed: make(chan struct{})}
	page.capture = func(n int) []byte {
		captured = append(captured, n)
		return stillBytes(t, n)
	}
	page.script = play(t, playOptions{frames: 90, mutateRep: func(rep map[string]any) {
		rep["layout"] = []any{map[string]any{"id": "s1", "index": 0, "hold_s": 1.0, "actors": []any{}, "props": []any{}, "texts": []any{}, "caption": nil, "missing": 0, "static": false}}
	}})
	runner, id := compileCheckRunner(t, page)
	got, err := runner.CompileCheck(context.Background(), id, Settings{Limits: storymaker.DefaultLimits(), Stills: true})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Available || !reflect.DeepEqual(captured, []int{30}) {
		t.Fatalf("summary %+v, captured %v", got, captured)
	}
	if len(got.Stills) != 1 || got.Stills[0].SceneID != "s1" || len(got.Sheet) == 0 {
		t.Fatalf("stills %+v, sheet %d bytes", got.Stills, len(got.Sheet))
	}
	if _, err := png.Decode(bytes.NewReader(got.Sheet)); err != nil {
		t.Fatalf("the sheet is not a PNG: %v", err)
	}
	if !page.closed.Load() {
		t.Error("the page must be closed")
	}
}

func TestCompileCheckWithoutStillsTakesNone(t *testing.T) {
	var captured int
	page := &fakePage{crashed: make(chan struct{})}
	page.capture = func(n int) []byte { captured++; return stillBytes(t, n) }
	page.script = play(t, playOptions{frames: 90, mutateRep: func(rep map[string]any) {
		rep["layout"] = []any{map[string]any{"id": "s1", "index": 0, "hold_s": 1.0, "actors": []any{}, "props": []any{}, "texts": []any{}}}
	}})
	runner, id := compileCheckRunner(t, page)
	got, err := runner.CompileCheck(context.Background(), id, Settings{Limits: storymaker.DefaultLimits()})
	if err != nil {
		t.Fatal(err)
	}
	if captured != 0 || len(got.Sheet) != 0 || len(got.Stills) != 0 || len(got.Layout) != 1 {
		t.Fatalf("captured %d, summary %+v", captured, got)
	}
}
