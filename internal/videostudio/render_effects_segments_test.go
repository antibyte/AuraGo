package videostudio

import (
	"context"
	"fmt"
	"image/color"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// The project combines a video and a still:
//   - the video has keyframed colour (curve and matrix commands), keyframed LUT intensity (blend in T), keyframed
//     sharpen (cas commands) and keyframed vignette (eval=frame);
//   - the still's vignette alpha wrapper is a two-input stage, so its commanded sharpen and its opacity commands read
//     command group 1.
//
// One segment and 7- and 13-frame segments render the same frames, at 25 and 30000/1001 fps (index §4.11), on local
// FFmpeg 8.1.1 and in the 5.1 container.
func TestSegmentedRenderEffects(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	root := t.TempDir()
	paths := map[string]string{"motion": filepath.Join(root, "motion.mp4"), "logo": filepath.Join(root, "logo.png")}
	makeLavfiVideoFixture(t, ctx, ffmpeg, paths["motion"], "testsrc2=s=320x180:r=25:d=6")
	writeFramePNG(t, paths["logo"], 120, 80, color.NRGBA{R: 40, G: 200, B: 90, A: 200})
	keys := func(from, to float64) []Keyframe {
		return []Keyframe{{F: 0, V: from, E: EaseInOut}, {F: 50, V: to, E: EaseLinear}}
	}
	for _, ntsc := range []bool{false, true} {
		project := Project{Version: CurrentVersion, Name: "Effect segments", Width: 720, Height: 720, FPS: 25}
		if ntsc {
			project.FPS, project.NTSC = 30, true
		}
		rate := ProjectRate(project)
		for _, id := range []string{"motion", "logo"} {
			asset, err := Probe(ctx, ffmpeg, paths[id])
			if err != nil {
				t.Fatalf("Probe(%s): %v", id, err)
			}
			asset.ID = id
			project.Assets = append(project.Assets, AssetAtRate(asset, rate))
		}
		video := Clip{ID: "a", AssetID: "motion", Duration: 80, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, Effects: []EffectInstance{
			{ID: "color", On: true, Params: map[string]any{"contrast": 1.1}, Keyframes: map[string][]Keyframe{"exposure": keys(-0.5, 0.5), "saturation": keys(1.4, 0.6)}},
			{ID: "lut", On: true, Params: map[string]any{"lut": "builtin:teal_orange"}, Keyframes: map[string][]Keyframe{"intensity": keys(0.2, 1)}},
			{ID: "sharpen", On: true, Params: map[string]any{}, Keyframes: map[string][]Keyframe{"strength": keys(0, 0.8)}},
			{ID: "vignette", On: true, Params: map[string]any{"size": 1.2}, Keyframes: map[string][]Keyframe{"amount": keys(0.2, 0.7)}},
		}}
		logo := Clip{ID: "logo", AssetID: "logo", Start: 8, Duration: 70, X: 0.55, Y: 0.1, Width: 0.3, Height: 0.25, Opacity: 1, Volume: 1, Fit: FitContain,
			Keyframes: map[string][]Keyframe{"opacity": keys(0.3, 1)},
			Effects: []EffectInstance{
				{ID: "vignette", On: true, Params: map[string]any{"amount": 0.6, "size": 0.8}},
				{ID: "sharpen", On: true, Params: map[string]any{}, Keyframes: map[string][]Keyframe{"strength": keys(0.1, 0.9)}},
			}}
		project.Tracks = []Track{
			{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{video}},
			{ID: "overlay", Name: "Overlay", Kind: TrackOverlay, Clips: []Clip{logo}},
		}
		if err := Validate(project); err != nil {
			t.Fatalf("NTSC %t: effect segment project invalid: %v", ntsc, err)
		}
		render := func(name string, segmentFrames int) []string {
			output := filepath.Join(root, fmt.Sprintf("%s-%t.mp4", name, ntsc))
			if err := RenderWithOptions(ctx, RenderOptions{FFmpegPath: ffmpeg, Project: project, AssetPaths: paths, Output: output,
				SegmentFrames: segmentFrames, Encoder: Encoder{Lossless: true}}); err != nil {
				t.Fatalf("render %s (NTSC %t): %v", name, ntsc, err)
			}
			return frameHashes(t, ctx, ffmpeg, output)
		}
		single := render("single", 100000)
		if len(single) != 80 {
			t.Fatalf("NTSC %t: %d frames, want 80", ntsc, len(single))
		}
		for _, size := range []int{7, 13} {
			if got := render(fmt.Sprintf("segments-%d", size), size); !reflect.DeepEqual(got, single) {
				t.Fatalf("NTSC %t: %d-frame segments differ from the single pass", ntsc, size)
			}
		}
	}
}
