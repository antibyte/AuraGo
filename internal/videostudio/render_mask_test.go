package videostudio

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"maps"
	"math"
	"path/filepath"
	"testing"
	"time"
)

// maskDistance is law L's signed distance (box px, > 0 outside the mask) of a clip's mask at a timeline frame, for a
// box of boxWidth × boxHeight px. It reads maskAt with the feather at its maximum and without invert: F = boxHeight/2,
// where m = 0.5 − s/F holds for |s| < F/2.
func maskDistance(clip Clip, frame, boxWidth, boxHeight int) func(x, y float64) float64 {
	wide := *clip.Mask
	wide.Feather, wide.Invert = 0.5, false
	probe := clip
	probe.Mask = &wide
	probe.Keyframes = maps.Clone(clip.Keyframes)
	delete(probe.Keyframes, "mask.feather")
	frameData, _ := maskFrameAt(probe, frame, boxWidth, boxHeight)
	ramp := 0.5 * float64(boxHeight)
	return func(x, y float64) float64 { return (0.5 - maskAt(frameData, x, y)) * ramp }
}

// The mask export matches law L (index §4.22) on an opaque white still, whose layer alpha is the mask.
//   - Static form (full-resolution geq once, loop) on a 640x360 box of a 1280x720 canvas: rectangle, ellipse, rotated,
//     feathered, inverted, and a mask reaching past the box. Every pixel lies within 1 level of 255·maskAt (geq
//     truncates), and pixels outside the box are 0.
//   - Keyframed form (quarter-resolution geq in T, bilinear upscale): the spike's moving, rotating ellipse on a 1920x1080
//     box at F = 1 px and F = 4 px, one FFmpeg process per checked frame with the window starting there.
//   - The mean difference stays within 2 levels.
//   - The 50 % contour (output 120..135, 60 px from the box edge) lies within 1.3 px (F = 1) and 0.55 px (F = 4)
//     of the true edge: the spike's maxima of 1.25 and 0.49 px, rounded, plus 0.05 px for this expression's
//     evaluation order.
func TestRenderMaskMatchesFormula(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	root := t.TempDir()
	rate := Rate{Num: 25, Den: 1}
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}

	small := filepath.Join(root, "white-640.png")
	writeFramePNG(t, small, 640, 360, white)
	for i, mask := range []Mask{
		{Shape: MaskRect, X: 0.1, Y: 0.15, W: 0.5, H: 0.6},
		{Shape: MaskEllipse, X: 0.2, Y: 0.1, W: 0.6, H: 0.7, Feather: 0.05},
		{Shape: MaskEllipse, X: 0.15, Y: 0.2, W: 0.7, H: 0.5, Rotation: 30, Feather: 0.02},
		{Shape: MaskRect, X: 0.2, Y: 0.2, W: 0.55, H: 0.5, Rotation: -20, Feather: 0.1, Invert: true},
		{Shape: MaskEllipse, X: -0.2, Y: 0.3, W: 0.9, H: 0.9, Feather: 0.03, Invert: true},
	} {
		clip := Clip{ID: "white", AssetID: "white", Duration: 1, X: 0.25, Y: 0.25, Width: 0.5, Height: 0.5, Opacity: 1, Volume: 1, Fit: FitContain, Mask: &mask}
		chain := clipChain{item: layerClip{clip: clip, whole: clip, image: true, pictureWidth: 640, pictureHeight: 360}, canvasWidth: 1280, canvasHeight: 720, rate: rate}
		if maskPlanOf(chain).perFrame {
			t.Fatalf("mask %d: a mask without keys on a static box must use the static form", i)
		}
		layer := renderClipFramesRGBA(t, ctx, ffmpeg, chain, small, 1)[0]
		frame, ok := maskFrameAt(clip, 0, 640, 360)
		if !ok {
			t.Fatalf("mask %d: maskFrameAt found no mask", i)
		}
		worst, at := 0.0, image.Point{}
		for y := range 720 {
			for x := range 1280 {
				want := 0.0
				if x >= 320 && x < 960 && y >= 180 && y < 540 {
					want = 255 * maskAt(frame, float64(x-320)+0.5, float64(y-180)+0.5)
				}
				if d := math.Abs(float64(layer[(y*1280+x)*4+3]) - want); d > worst {
					worst, at = d, image.Pt(x, y)
				}
			}
		}
		if worst > 1 {
			t.Fatalf("static mask %d %+v: alpha off by %.2f at %v, want within 1 level", i, mask, worst, at)
		}
		t.Logf("static mask %d: largest difference %.2f", i, worst)
	}

	const boxWidth, boxHeight = 1920, 1080
	full := filepath.Join(root, "white-1080.png")
	writeFramePNG(t, full, boxWidth, boxHeight, white)
	line := func(from, to float64) []Keyframe {
		return []Keyframe{{F: 0, V: from, E: EaseLinear}, {F: 50, V: to, E: EaseLinear}}
	}
	for _, feather := range []struct{ px, maxShift float64 }{{1, 1.3}, {4, 0.55}} {
		// centre (700+520p, 540−100p), radii 480+320p and 280+200p, rotation 0.7p rad, p = frame/50 (spike S5)
		clip := Clip{ID: "white", AssetID: "white", Duration: 50, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain,
			Mask: &Mask{Shape: MaskEllipse, X: 220.0 / boxWidth, Y: 260.0 / boxHeight, W: 960.0 / boxWidth, H: 560.0 / boxHeight, Feather: feather.px / boxHeight},
			Keyframes: map[string][]Keyframe{
				"mask.x": line(220.0/boxWidth, 420.0/boxWidth), "mask.y": line(260.0/boxHeight, -40.0/boxHeight),
				"mask.w": line(960.0/boxWidth, 1600.0/boxWidth), "mask.h": line(560.0/boxHeight, 960.0/boxHeight),
				"mask.rotation": line(0, 0.7*180/math.Pi),
			}}
		worstMean, worstShift := 0.0, 0.0
		for _, n := range []int{0, 10, 20, 30, 40, 49} {
			local := clip
			local.Duration = 1
			chain := clipChain{item: layerClip{clip: local, whole: clip, at: n, image: true, pictureWidth: boxWidth, pictureHeight: boxHeight}, canvasWidth: boxWidth, canvasHeight: boxHeight, rate: rate}
			if plan := maskPlanOf(chain); !plan.perFrame || plan.gridWidth != 480 || plan.gridHeight != 270 {
				t.Fatalf("keyed mask plan %+v, want the quarter-resolution form 480x270", plan)
			}
			layer := renderClipFramesRGBA(t, ctx, ffmpeg, chain, full, 1)[0]
			frame, _ := maskFrameAt(clip, n, boxWidth, boxHeight)
			distance := maskDistance(clip, n, boxWidth, boxHeight)
			sum := 0.0
			for y := range boxHeight {
				for x := range boxWidth {
					got := float64(layer[(y*boxWidth+x)*4+3])
					px, py := float64(x)+0.5, float64(y)+0.5
					sum += math.Abs(got - 255*maskAt(frame, px, py))
					if x >= 60 && x < boxWidth-60 && y >= 60 && y < boxHeight-60 && got >= 120 && got <= 135 {
						worstShift = math.Max(worstShift, math.Abs(distance(px, py)))
					}
				}
			}
			worstMean = math.Max(worstMean, sum/(boxWidth*boxHeight))
		}
		t.Logf("keyed mask at F = %v px: worst mean difference %.3f levels, worst 50 %% edge shift %.3f px", feather.px, worstMean, worstShift)
		if worstMean > 2 || worstShift > feather.maxShift {
			t.Fatalf("keyed mask at F = %v px: mean %.3f levels (max 2), edge shift %.3f px (max %v)", feather.px, worstMean, worstShift, feather.maxShift)
		}
	}
}

// A mask belongs to its box (index §4.22, decision 3). With keyframed geometry the mask stream is the pad canvas with
// the box centred; orient turns it with the clip, and place moves it with the box. On every frame of one FFmpeg process,
// the layer's alpha (an opaque white still) must have:
//   - the centroid of the Go law over the frame's box (maskFrameAt, maskAt), turned with the clip's rotation about the
//     box centre and placed at the box centre (boxExprs), within 1 px;
//   - the law's area, within 3 %.
//
// The cover case also has crop keyframes, which change the picture under the mask but never the mask. A window that
// starts inside the clip renders the same frames byte for byte, because the mask's T has the window start baked in.
func TestRenderMaskFollowsKeyframedBox(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	white := filepath.Join(t.TempDir(), "white.png")
	writeFramePNG(t, white, 320, 180, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	rate := Rate{Num: 25, Den: 1}
	const frames, canvasWidth, canvasHeight = 12, 640, 360
	for _, c := range []struct {
		name string
		clip Clip
	}{
		{"contain, rotating box, static mask", Clip{ID: "box", AssetID: "white", Duration: frames, Opacity: 1, Volume: 1, Fit: FitContain,
			Mask: &Mask{Shape: MaskEllipse, X: 0.3, Y: 0.3, W: 0.4, H: 0.4, Rotation: 10, Feather: 0.05},
			Keyframes: map[string][]Keyframe{"x": linearKeys(0.1, 0.3, 11), "y": linearKeys(0.1, 0.2, 11), "width": linearKeys(0.4, 0.6, 11),
				"height": linearKeys(0.4, 0.6, 11), "rotation": linearKeys(0, 25, 11)}}},
		{"cover with crop keys, keyed mask", Clip{ID: "box", AssetID: "white", Duration: frames, Height: 0.6, Opacity: 1, Volume: 1, Fit: FitCover,
			Mask: &Mask{Shape: MaskRect, X: 0.2, Y: 0.25, W: 0.3, H: 0.5, Feather: 0.02},
			Keyframes: map[string][]Keyframe{"width": linearKeys(0.5, 0.7, 11), "crop.l": linearKeys(0, 0.2, 11),
				"mask.x": linearKeys(0.2, 0.35, 11), "mask.rotation": linearKeys(0, -15, 11)}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			chain := func(at, duration int) clipChain {
				local := c.clip
				local.Start, local.Duration = 0, duration
				return clipChain{item: layerClip{clip: local, whole: c.clip, at: at, image: true, pictureWidth: 320, pictureHeight: 180, sourceAspect: 16.0 / 9},
					canvasWidth: canvasWidth, canvasHeight: canvasHeight, rate: rate}
			}
			whole := chain(0, frames)
			if plan := maskPlanOf(whole); !plan.perFrame {
				t.Fatalf("keyframed geometry must use the per-frame form: %+v", plan)
			}
			layers := renderClipFramesRGBA(t, ctx, ffmpeg, whole, white, frames)
			g := whole.geometry()
			step := float64(rate.Den) / float64(rate.Num)
			for k, layer := range layers {
				bw, bh := g.widths[k], g.heights[k]
				mf, ok := maskFrameAt(c.clip, k, bw, bh)
				if !ok {
					t.Fatal("maskFrameAt found no mask")
				}
				var area, sumX, sumY float64
				for y := range bh {
					for x := range bw {
						m := maskAt(mf, float64(x)+0.5, float64(y)+0.5)
						area += m
						sumX += m * (float64(x) + 0.5)
						sumY += m * (float64(y) + 0.5)
					}
				}
				offsetX, offsetY := sumX/area-float64(bw)/2, sumY/area-float64(bh)/2
				rotation, _ := ValueAt(c.clip, "rotation", k)
				phi := rotation * math.Pi / 180
				vars := map[string]float64{"t": float64(k) * step}
				wantX := g.box.cx.eval(vars) + offsetX*math.Cos(phi) - offsetY*math.Sin(phi)
				wantY := g.box.cy.eval(vars) + offsetX*math.Sin(phi) + offsetY*math.Cos(phi)
				var gotArea, gotX, gotY float64
				for y := range canvasHeight {
					for x := range canvasWidth {
						a := float64(layer[(y*canvasWidth+x)*4+3]) / 255
						gotArea += a
						gotX += a * (float64(x) + 0.5)
						gotY += a * (float64(y) + 0.5)
					}
				}
				gotX, gotY = gotX/gotArea, gotY/gotArea
				if math.Hypot(gotX-wantX, gotY-wantY) > 1 || math.Abs(gotArea-area) > 0.03*area {
					t.Fatalf("frame %d: centroid (%.2f, %.2f), area %.0f; want (%.2f, %.2f), area %.0f", k, gotX, gotY, gotArea, wantX, wantY, area)
				}
			}
			late := renderClipFramesRGBA(t, ctx, ffmpeg, chain(5, frames-5), white, frames-5)
			for i, layer := range late {
				if !bytes.Equal(layer, layers[5+i]) {
					t.Fatalf("frame %d differs when its window starts at frame 5", 5+i)
				}
			}
		})
	}
}
