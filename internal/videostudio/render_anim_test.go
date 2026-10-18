package videostudio

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"math"
	"path/filepath"
	"testing"
	"time"
)

// animRender describes what renderAnimFrames draws: a PNG still of pictureWidth × pictureHeight px on a canvasWidth ×
// canvasHeight canvas at rate.
type animRender struct {
	still                       string
	pictureWidth, pictureHeight int
	canvasWidth, canvasHeight   int
	rate                        Rate
}

// renderAnimFrames renders the layer RGBA of clip, a clip of r's still, for the absolute timeline frames
// [from, from+frames), through one clip chain (renderClipFramesRGBA: canvas size, 4 bytes per pixel, straight alpha).
// The window is built the way segmentLayers builds one: the window-local clip, the whole clip and at = from. A window
// that starts inside the clip therefore shows what a later segment shows. The window must lie inside the clip. extra
// goes in front of the graph (thread limits). Tasks 7 and 8 use renderClipFramesRGBA and their own renderAnimGraph
// instead (a hand-built graph, not one clip chain).
func renderAnimFrames(t *testing.T, ctx context.Context, ffmpeg string, r animRender, clip Clip, from, frames int, extra ...string) [][]byte {
	t.Helper()
	if from < clip.Start || from+frames > clip.Start+clip.Duration {
		t.Fatalf("window %d+%d lies outside the clip %d+%d", from, frames, clip.Start, clip.Duration)
	}
	local := clip
	local.Start, local.Duration, local.Transition = 0, frames, nil
	item := layerClip{clip: local, whole: clip, at: from, image: true, pictureWidth: r.pictureWidth, pictureHeight: r.pictureHeight,
		sourceAspect: float64(r.pictureWidth) / float64(r.pictureHeight)}
	chain := clipChain{item: item, canvasWidth: r.canvasWidth, canvasHeight: r.canvasHeight, rate: r.rate}
	return renderClipFramesRGBA(t, ctx, ffmpeg, chain, r.still, frames, extra...)
}

// alphaBounds is the bounding box of the pixels of an RGBA layer frame whose alpha is above 0, and the alpha-weighted
// centroid of their pixel centres (x + 0.5, y + 0.5). ok is false for a transparent frame.
func alphaBounds(frame []byte, width, height int) (bounds image.Rectangle, cx, cy float64, ok bool) {
	var sum, sx, sy float64
	for y := range height {
		for x := range width {
			a := float64(frame[(y*width+x)*4+3])
			if a == 0 {
				continue
			}
			pixel := image.Rect(x, y, x+1, y+1)
			if !ok {
				bounds, ok = pixel, true
			} else {
				bounds = bounds.Union(pixel)
			}
			sum, sx, sy = sum+a, sx+a*(float64(x)+0.5), sy+a*(float64(y)+0.5)
		}
	}
	if ok {
		cx, cy = sx/sum, sy/sum
	}
	return bounds, cx, cy, ok
}

// animReferenceRect is the Go box of an animated clip at a timeline frame, not cut to the canvas: clipBox over ValueAt
// (keyframedBox), then animBox with animAt.
func animReferenceRect(t *testing.T, clip Clip, frame, canvasWidth, canvasHeight int) image.Rectangle {
	t.Helper()
	rest := keyframedBox(t, clip, frame, canvasWidth, canvasHeight)
	cx, cy, w, h := animBox(rest.Min.X, rest.Min.Y, rest.Dx(), rest.Dy(), animAt(clip, frame), canvasWidth, canvasHeight)
	return image.Rect(cx-w/2, cy-h/2, cx+w/2, cy+h/2)
}

// TestRenderAnimBoxFollowsReference renders slides and pops of a red still. It compares every frame's alpha with
// animBox (index §4.24 tolerances: extent exact, alpha centroid ±0.5 px):
//   - cover: the visible box (the animated box cut to the canvas), exactly;
//   - contain: inside the box, centred, filling one axis;
//   - both: the alpha-weighted centroid within 0.5 px of the box centre while the box lies on the canvas, and a
//     transparent frame where the opacity rounds to 0.
//
// A window that starts 7 frames into the clip renders the same frames byte for byte (absolute frames are baked in).
// Cases:
//   - contain with pop in (the 1.08× peak at frame 7: 346×194) and slide_down out;
//   - cover with slide_right in and pop out;
//   - a cover box that leaves the canvas (slide_left out at the left edge);
//   - a full-canvas pop past the canvas.
//
// This runs on local FFmpeg and in the 5.1.9 container (/tmp/vsp2-2e-t6).
func TestRenderAnimBoxFollowsReference(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	r := animRender{still: filepath.Join(t.TempDir(), "red.png"), pictureWidth: 320, pictureHeight: 180, canvasWidth: 640, canvasHeight: 360, rate: Rate{Num: 25, Den: 1}}
	writeFramePNG(t, r.still, r.pictureWidth, r.pictureHeight, color.NRGBA{R: 255, A: 255})
	clip := func(fit FitMode, x, y, w, h float64, in, out *ClipAnim) Clip {
		return Clip{ID: "a", AssetID: "red", Duration: 40, X: x, Y: y, Width: w, Height: h, Opacity: 1, Volume: 1, Fit: fit, AnimIn: in, AnimOut: out}
	}
	canvas := image.Rect(0, 0, r.canvasWidth, r.canvasHeight)
	for _, tc := range []struct {
		name string
		clip Clip
		peak image.Point // the box size on frame 7 (zero: not checked)
	}{
		{"contain pop in slide_down out", clip(FitContain, 0.2, 0.2, 0.5, 0.5, &ClipAnim{Preset: "pop", Duration: 10}, &ClipAnim{Preset: "slide_down", Duration: 10}), image.Pt(346, 194)},
		{"cover slide_right in pop out", clip(FitCover, 0.1, 0.3, 0.6, 0.4, &ClipAnim{Preset: "slide_right", Duration: 10}, &ClipAnim{Preset: "pop", Duration: 10}), image.Point{}},
		{"cover slide_left out off the canvas", clip(FitCover, 0, 0.3, 0.4, 0.4, nil, &ClipAnim{Preset: "slide_left", Duration: 12}), image.Point{}},
		{"cover full-canvas pop past the canvas", clip(FitCover, 0, 0, 1, 1, &ClipAnim{Preset: "pop", Duration: 10}, nil), image.Pt(692, 388)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frames := renderAnimFrames(t, ctx, ffmpeg, r, tc.clip, 0, tc.clip.Duration)
			worst := 0.0 // the largest centroid offset (px), logged so a run shows how close the export is
			for k, frame := range frames {
				want := animReferenceRect(t, tc.clip, k, r.canvasWidth, r.canvasHeight)
				if k == 7 && tc.peak != (image.Point{}) && want.Size() != tc.peak {
					t.Fatalf("the pop peak box is %v, want %v (1.08×)", want.Size(), tc.peak)
				}
				visible := want.Intersect(canvas)
				got, cx, cy, ok := alphaBounds(frame, r.canvasWidth, r.canvasHeight)
				if math.Round(255*clipOpacityAt(tc.clip, k)) == 0 || visible.Empty() {
					if ok {
						t.Fatalf("frame %d: alpha in %v, want a transparent frame (opacity %v, box %v)", k, got, clipOpacityAt(tc.clip, k), want)
					}
					continue
				}
				if !ok {
					t.Fatalf("frame %d is transparent, want the box %v", k, visible)
				}
				switch {
				case tc.clip.Fit == FitCover && got != visible:
					t.Fatalf("frame %d: alpha in %v, want the visible box %v (box %v)", k, got, visible, want)
				case tc.clip.Fit == FitContain && want.In(canvas):
					left, right, top, bottom := got.Min.X-want.Min.X, want.Max.X-got.Max.X, got.Min.Y-want.Min.Y, want.Max.Y-got.Max.Y
					if !got.In(want) || left != right || top != bottom || (left != 0 && top != 0) {
						t.Fatalf("frame %d: alpha in %v is not a centred fit filling the box %v", k, got, want)
					}
				case tc.clip.Fit == FitContain && !got.In(visible):
					t.Fatalf("frame %d: alpha in %v outside the visible box %v", k, got, visible)
				}
				if want.In(canvas) {
					centreX, centreY := float64(want.Min.X+want.Max.X)/2, float64(want.Min.Y+want.Max.Y)/2
					worst = math.Max(worst, math.Max(math.Abs(cx-centreX), math.Abs(cy-centreY)))
					if math.Abs(cx-centreX) > 0.5 || math.Abs(cy-centreY) > 0.5 {
						t.Fatalf("frame %d: alpha centroid (%.2f, %.2f), want the box centre (%.1f, %.1f) ±0.5", k, cx, cy, centreX, centreY)
					}
				}
			}
			t.Logf("%s: largest alpha centroid offset %.3f px", tc.name, worst)
			late := renderAnimFrames(t, ctx, ffmpeg, r, tc.clip, 7, tc.clip.Duration-7)
			for i, frame := range late {
				if !bytes.Equal(frame, frames[7+i]) {
					t.Fatalf("frame %d differs when its window starts at frame 7", 7+i)
				}
			}
		})
	}
}

// TestRenderAnimOpacityFollowsReference checks the layer alpha at the animated box centre on every frame. It must equal
// round(255·clipOpacityAt) within 1 level (index §4.24), where clipOpacityAt is the keyed opacity times the animation's,
// commanded per window frame.
//   - Keyed opacity (0.4 → 1) times fade in 12 and fade out 10, on a full-canvas box.
//   - slide_up in (opacity p) and pop out (opacity min(1, p/0.3)) at a static opacity of 0.9.
func TestRenderAnimOpacityFollowsReference(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	r := animRender{still: filepath.Join(t.TempDir(), "red.png"), pictureWidth: 640, pictureHeight: 360, canvasWidth: 640, canvasHeight: 360, rate: Rate{Num: 25, Den: 1}}
	writeFramePNG(t, r.still, r.pictureWidth, r.pictureHeight, color.NRGBA{R: 255, A: 255})
	for _, tc := range []struct {
		name string
		clip Clip
	}{
		{"keyed opacity times fade in and out", Clip{ID: "a", AssetID: "red", Duration: 40, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain,
			Keyframes: map[string][]Keyframe{"opacity": {{F: 0, V: 0.4, E: EaseLinear}, {F: 39, V: 1, E: EaseLinear}}},
			AnimIn:    &ClipAnim{Preset: "fade", Duration: 12}, AnimOut: &ClipAnim{Preset: "fade", Duration: 10}}},
		{"slide_up in and pop out at opacity 0.9", Clip{ID: "a", AssetID: "red", Duration: 40, X: 0.25, Y: 0.25, Width: 0.5, Height: 0.5, Opacity: 0.9, Volume: 1, Fit: FitContain,
			AnimIn: &ClipAnim{Preset: "slide_up", Duration: 10}, AnimOut: &ClipAnim{Preset: "pop", Duration: 10}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frames := renderAnimFrames(t, ctx, ffmpeg, r, tc.clip, 0, tc.clip.Duration)
			worst := 0.0
			for k, frame := range frames {
				box := animReferenceRect(t, tc.clip, k, r.canvasWidth, r.canvasHeight)
				x, y := (box.Min.X+box.Max.X)/2, (box.Min.Y+box.Max.Y)/2
				opacity := clipOpacityAt(tc.clip, k)
				got, want := float64(frame[(y*r.canvasWidth+x)*4+3]), math.Round(255*opacity)
				worst = math.Max(worst, math.Abs(got-want))
				if math.Abs(got-want) > 1 {
					keyed, _ := ValueAt(tc.clip, "opacity", k)
					t.Fatalf("frame %d: alpha %v at the box centre (%d, %d), want %v (keyed %v × animation %v)", k, got, x, y, want, keyed, animAt(tc.clip, k).Opacity)
				}
			}
			t.Logf("%s: largest alpha difference %v level", tc.name, worst)
		})
	}
}
