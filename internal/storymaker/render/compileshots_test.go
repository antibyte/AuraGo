package render

import (
	"bytes"
	"context"
	"image/jpeg"
	"reflect"
	"testing"

	"aurago/internal/storymaker"
)

func TestShotFramesPicksTheMomentOfEachRequest(t *testing.T) {
	summary := storymaker.CompileSummary{
		DurationMS: 6000,
		Scenes:     []storymaker.CompileScene{{ID: "s1", StartMS: 0, EndMS: 2000}, {ID: "s2", StartMS: 1800, EndMS: 6000}, {ID: "s3", StartMS: 5800, EndMS: 6000}},
		Layout:     []storymaker.LayoutScene{{ID: "s1", Index: 0, HoldS: 1.0}},
	}
	reqs := []storymaker.ShotRequest{
		{SceneID: "s1"},                        // the hold frame: 30
		{SceneID: "s2"},                        // no layout: 70 percent of 54..180 is 54+88 = 142
		{SceneID: "s3", AtS: 99, HasAt: true},  // clamped to the last frame of the scene: 179
		{SceneID: "s1", AtS: 0.5, HasAt: true}, // 15 frames in
		{SceneID: "ghost"},                     // not in the summary
	}
	frames, ids, at := shotFrames(summary, reqs, 30, 0)
	if !reflect.DeepEqual(frames, []int{30, 142, 179, 15}) || !reflect.DeepEqual(ids, []string{"s1", "s2", "s3", "s1"}) || !reflect.DeepEqual(at, []int64{1000, 2933, 167, 500}) {
		t.Fatalf("frames %v ids %v at %v", frames, ids, at)
	}
	// A page that offers only 100 frames cannot show frames 142 and 179.
	frames, ids, _ = shotFrames(summary, reqs, 30, 100)
	if !reflect.DeepEqual(frames, []int{30, 15}) || !reflect.DeepEqual(ids, []string{"s1", "s1"}) {
		t.Fatalf("with a frame limit of 100: frames %v ids %v", frames, ids)
	}
}

func TestCompileCheckTakesShots(t *testing.T) {
	run := func(t *testing.T, sheet bool) (storymaker.CompileSummary, []int) {
		var captured []int
		page := &fakePage{crashed: make(chan struct{})}
		page.capture = func(n int) []byte {
			captured = append(captured, n)
			return stillBytes(t, n)
		}
		page.script = play(t, playOptions{frames: 90})
		runner, id := compileCheckRunner(t, page)
		got, err := runner.CompileCheck(context.Background(), id, Settings{
			Limits: storymaker.DefaultLimits(), ShotSheet: sheet,
			Shots: []storymaker.ShotRequest{{SceneID: "s1", AtS: 1, HasAt: true}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if !page.closed.Load() {
			t.Error("the page must be closed")
		}
		return got, captured
	}
	t.Run("one picture per scene", func(t *testing.T) {
		got, captured := run(t, false)
		if !got.Available || !reflect.DeepEqual(captured, []int{30}) || len(got.Shots) != 1 {
			t.Fatalf("summary %+v, captured %v", got, captured)
		}
		shot := got.Shots[0]
		if shot.SceneID != "s1" || shot.AtMS != 1000 || shot.Width != 64 || shot.Height != 36 {
			t.Fatalf("shot %+v", shot)
		}
		if _, err := jpeg.Decode(bytes.NewReader(shot.Image)); err != nil {
			t.Fatalf("the picture is not a JPEG: %v", err)
		}
		if len(got.ShotSheet) != 0 || len(got.Sheet) != 0 || len(got.Stills) != 0 {
			t.Errorf("no sheet and no stills were asked for: %+v", got)
		}
	})
	t.Run("one sheet", func(t *testing.T) {
		got, _ := run(t, true)
		if len(got.Shots) != 1 || got.Shots[0].SceneID != "s1" || len(got.Shots[0].Image) != 0 {
			t.Fatalf("shots %+v", got.Shots)
		}
		if _, err := jpeg.Decode(bytes.NewReader(got.ShotSheet)); err != nil || got.ShotSheetWidth == 0 || got.ShotSheetHeight == 0 {
			t.Fatalf("sheet %d bytes %dx%d: %v", len(got.ShotSheet), got.ShotSheetWidth, got.ShotSheetHeight, err)
		}
	})
}
