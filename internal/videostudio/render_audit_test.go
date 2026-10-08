package videostudio

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// concat (gaps and adjacent clips) outputs a 1/1000000 timebase that xfade rejects unless it is reset to 1/30.
func TestRenderTransitionAfterGap(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	root := t.TempDir()
	redPath, bluePath := filepath.Join(root, "red.mp4"), filepath.Join(root, "blue.mp4")
	makeVideoFixture(t, ctx, ffmpeg, redPath, "red")
	makeVideoFixture(t, ctx, ffmpeg, bluePath, "blue")
	red, err := Probe(ctx, ffmpeg, redPath)
	if err != nil {
		t.Fatal(err)
	}
	blue, err := Probe(ctx, ffmpeg, bluePath)
	if err != nil {
		t.Fatal(err)
	}
	red.ID, blue.ID = "red", "blue"
	clip := func(id, asset string, start int, transition *Transition) Clip {
		return Clip{ID: id, AssetID: asset, Start: start, Duration: 30, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, Transition: transition}
	}
	cases := map[string][]Clip{
		"leading gap": {clip("a", "red", 15, &Transition{Type: TransitionDissolve, Duration: 10}), clip("b", "blue", 35, nil)},
		"adjacent clips before a transition": {
			clip("a", "red", 0, nil),
			clip("b", "blue", 30, &Transition{Type: TransitionDissolve, Duration: 10}),
			clip("c", "red", 50, nil),
		},
		"gap before a later transition": {
			clip("a", "red", 0, nil),
			clip("b", "blue", 40, &Transition{Type: TransitionWipeLeft, Duration: 10}),
			clip("c", "red", 60, nil),
		},
	}
	for name, clips := range cases {
		t.Run(name, func(t *testing.T) {
			project := Project{Version: 1, Name: name, Width: 720, Height: 720, FPS: 30, Assets: []Asset{red, blue},
				Tracks: []Track{{ID: "video", Name: "Video", Kind: TrackVideo, Clips: clips}}}
			if err := Validate(project); err != nil {
				t.Fatalf("fixture project invalid: %v", err)
			}
			output := filepath.Join(root, name+".mp4")
			if err := Render(ctx, ffmpeg, project, map[string]string{"red": redPath, "blue": bluePath}, output, nil); err != nil {
				t.Fatalf("Render(%s) error = %v", name, err)
			}
		})
	}
}

// Canvas and CSS rotate positive angles clockwise; the export must turn the same way.
func TestRenderRotationMatchesPreviewDirection(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	root := t.TempDir()
	sourcePath := filepath.Join(root, "split.mp4")
	makeSplitVideoFixture(t, ctx, ffmpeg, sourcePath)
	asset, err := Probe(ctx, ffmpeg, sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	asset.ID = "split"
	project := Project{Version: 1, Name: "Rotation direction", Width: 720, Height: 720, FPS: 30, Assets: []Asset{asset},
		Tracks: []Track{{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{{
			ID: "clip", AssetID: "split", Start: 0, Duration: 30, X: 0.25, Y: 0.25, Width: 0.5, Height: 0.5,
			Rotation: 90, Opacity: 1, Volume: 1, Fit: FitCover,
		}}}}}
	output := filepath.Join(root, "rotation.mp4")
	if err := Render(ctx, ffmpeg, project, map[string]string{"split": sourcePath}, output, nil); err != nil {
		t.Fatalf("Render(rotation) error = %v", err)
	}
	// Clockwise by 90 degrees: the red left half ends up on top, the blue right half at the bottom.
	top := decodedPixel(t, ctx, ffmpeg, output, "0.500000", 360, 300)
	bottom := decodedPixel(t, ctx, ffmpeg, output, "0.500000", 360, 420)
	if top[0] < 150 || top[2] > 80 || bottom[2] < 150 || bottom[0] > 80 {
		t.Fatalf("rotation 90 = top %v bottom %v, want red on top and blue at the bottom (clockwise like the canvas preview)", top, bottom)
	}
}
