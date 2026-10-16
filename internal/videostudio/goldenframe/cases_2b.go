package goldenframe

import "aurago/internal/videostudio"

// Keyframe and speed-ramp goldens (plan 2b, Task 8; index §4.12, §4.18). Every case animates one ticker clip (id "k").
// The ticker's stripes move 4 px per source frame, so a keyframe sampled at the wrong source position, or a ramp that
// shows a neighbouring source frame, lowers SSIM across the whole clip and not only at an edge. The GL compositor and
// the 2D painter each draw every case. RenderFrame renders the export's frame as the one-frame segment
// [Frame, Frame+1), so each case also exercises the absolute-frame baking of the keyframe expressions and the ramp's
// seek window. The segment tests of plan 2b cover longer windows.
//
// Thresholds: every case uses ThresholdGeometry (index §4.12, "keyframed transform"). A case with another threshold
// states the measured reason on the line above it.
//
// Frames: keyframed paths are sampled at sourceAt(clip, frame) (index §4.18). cases_2b_test.go lists the expected
// values. Ramp cases start at timeline frame 0 with offset 0. Their frames are chosen so that S(frame) is at least 0.1
// source frames away from a whole frame. Then neither the export's 1 ms snapping tolerance nor float rounding in the
// preview's seek can show a neighbouring source frame.
//
// cases_2b.go sorts before cases_foundation.go, so these cases register (and run) first. The order has no meaning.
func init() {
	registerCases(keyframeCases)
	registerMedia(MediaTicker, tickerMedia)
}

// MediaTicker names ticker.mp4: 320x180, 25 fps, 8 s (200 frames), all-intra H.264, BT.709 (lavfiVideo). Sinusoidal
// vertical stripes with a 32 px period scroll 4 px (an eighth of a period) per frame over a soft vertical gradient.
// Every frame differs from its neighbours across the whole picture: one source frame of error scores SSIM 0.85 to 0.89.
//
// The stripes are luma stripes (amplitude 60, the same in all three channels) with a small per-channel colour tint
// (amplitude 12, phase-shifted per channel). A first version phase-shifted the three channels by 2.1 rad each (colour
// amplitude about 100). Its stripes are almost pure chroma, and export and preview convert yuv420p to RGB differently:
// ffmpeg's scaler and Chrome's video decoder place the chroma samples about 0.4 output px apart (R and B shifted in
// opposite directions) and G differs by 1.4 levels on average. That is unrelated to keyframes or ramps. A full-frame
// ticker clip without any animation scored SSIM min 0.968 (R 0.973, G 0.993, B 0.968) on both painters
// (Chrome with SwiftShader, FFmpeg 8.1.1, 2026-10-17), under ThresholdGeometry, while the smooth gradient media of the
// foundation cases never shows it. With the tinted luma stripes the same case scores 0.995 and the lowest 2b case 0.993.
const MediaTicker = "ticker"

var tickerMedia = lavfiVideo("ticker.mp4", "nullsrc=s=320x180:r=25:d=8,format=gbrp,"+
	"geq=r=128+60*sin(2*PI*(X+4*N)/32)+12*sin(2*PI*(X+4*N)/32+2.1)+40*(Y/H-0.5):"+
	"g=128+60*sin(2*PI*(X+4*N)/32)+12*sin(2*PI*(X+4*N)/32+4.2):"+
	"b=128+60*sin(2*PI*(X+4*N)/32)+12*sin(2*PI*(X+4*N)/32+1)-40*(Y/H-0.5)")

// kfEdit changes one property of a clip built by kfClip.
type kfEdit func(*videostudio.Clip)

// kfClip is a ticker clip at timeline frames [start, start+duration) with NewClip's defaults and the given edits.
func kfClip(id string, start, duration int, edits ...kfEdit) videostudio.Clip {
	clip := NewClip(id, MediaTicker, start, duration)
	for _, edit := range edits {
		edit(&clip)
	}
	return clip
}

func kfBox(x, y, width, height float64) kfEdit {
	return func(clip *videostudio.Clip) { clip.X, clip.Y, clip.Width, clip.Height = x, y, width, height }
}

func kfOffset(offset int) kfEdit { return func(clip *videostudio.Clip) { clip.Offset = offset } }

func kfSpeed(speed float64) kfEdit { return func(clip *videostudio.Clip) { clip.Speed = speed } }

// kfKeys sets the keyframes of one base path.
func kfKeys(path string, keys ...videostudio.Keyframe) kfEdit {
	return func(clip *videostudio.Clip) {
		if clip.Keyframes == nil {
			clip.Keyframes = map[string][]videostudio.Keyframe{}
		}
		clip.Keyframes[path] = keys
	}
}

func kfKey(f int, v float64, e videostudio.Easing) videostudio.Keyframe {
	return videostudio.Keyframe{F: f, V: v, E: e}
}

func kfRamp(points ...videostudio.RampPoint) kfEdit {
	return func(clip *videostudio.Clip) { clip.Ramp = points }
}

func kfPoint(f int, speed float64, e videostudio.Easing) videostudio.RampPoint {
	return videostudio.RampPoint{F: f, Speed: speed, E: e}
}

func kfCase(m Media, name string, frame int, tracks ...videostudio.Track) Case {
	return Case{Name: name, Project: m.Project(name, tracks...), Frame: frame, Threshold: ThresholdGeometry, Painters: []string{PainterGL, Painter2D}}
}

func keyframeCases(m Media) []Case {
	const (
		linear    = videostudio.EaseLinear
		easeInOut = videostudio.EaseInOut
		easeOut   = videostudio.EaseOut
		hold      = videostudio.EaseHold
	)
	// background is the moving gradient under the cases that test opacity, so transparency shows.
	background := VideoTrack("v1", NewClip("bg", MediaGradient, 0, 60))
	return []Case{
		// x 0.05 -> 0.55 and y 0.10 -> 0.50 over source frames 0..50: frame 20 is (0.25, 0.26). The static box at
		// (0.6, 0.6) is never drawn while the paths are animated.
		kfCase(m, "keyframe_position_linear", 20, VideoTrack("v1", kfClip("k", 0, 60, kfBox(0.6, 0.6, 0.4, 0.4),
			kfKeys("x", kfKey(0, 0.05, linear), kfKey(50, 0.55, linear)),
			kfKeys("y", kfKey(0, 0.1, linear), kfKey(50, 0.5, linear))))),
		// width and height 0.3 -> 0.8, eased in and out over frames 0..40: frame 15 (t = 0.375) is 0.3 + 0.5 x 4t^3 = 0.40546875.
		kfCase(m, "keyframe_scale_ease_in_out", 15, VideoTrack("v1", kfClip("k", 0, 60, kfBox(0.1, 0.1, 0.3, 0.3),
			kfKeys("width", kfKey(0, 0.3, easeInOut), kfKey(40, 0.8, linear)),
			kfKeys("height", kfKey(0, 0.3, easeInOut), kfKey(40, 0.8, linear))))),
		// rotation 0 -> 90, eased out over frames 0..50: frame 20 (t = 0.4) is 90 x (1 - 0.6^3) = 70.56 degrees. The box
		// stays inside the canvas at every angle.
		kfCase(m, "keyframe_rotation_ease_out", 20, VideoTrack("v1", kfClip("k", 0, 60, kfBox(0.3, 0.25, 0.4, 0.5),
			kfKeys("rotation", kfKey(0, 0, easeOut), kfKey(50, 90, linear))))),
		// opacity holds 0.4 until frame 40 (linear would give 0.85 at frame 30). The static opacity is 1.
		kfCase(m, "keyframe_opacity_hold", 30, background, VideoTrack("v2", kfClip("k", 0, 60, kfBox(0.2, 0.2, 0.6, 0.6),
			kfKeys("opacity", kfKey(0, 0.4, hold), kfKey(40, 1, linear))))),
		// The clip starts at timeline frame 10 on source frame 20. Frame 15 shows source frame 25, before the first key
		// (40), so x keeps the first value 0.1 (the static x is 0.6).
		kfCase(m, "keyframe_before_first", 15, VideoTrack("v1", kfClip("k", 10, 50, kfOffset(20), kfBox(0.6, 0.3, 0.4, 0.4),
			kfKeys("x", kfKey(40, 0.1, linear), kfKey(70, 0.5, linear))))),
		// Offset 10 at 2x: frame 15 shows source frame 10 + 2 x 15 = 40, 60 % into keys at source 10..60, so x is 0.34 and
		// opacity 0.8. Keys follow the content, not the timeline.
		kfCase(m, "keyframe_combined_speed2", 15, background, VideoTrack("v2", kfClip("k", 0, 40, kfOffset(10), kfSpeed(2), kfBox(0.5, 0.3, 0.4, 0.4),
			kfKeys("x", kfKey(10, 0.1, linear), kfKey(60, 0.5, linear)),
			kfKeys("opacity", kfKey(10, 0.5, linear), kfKey(60, 1, linear))))),
		// 1x -> 3x eased in and out over clip frames 0..50 (sub-piece node speeds 1, 1.125, 2, 2.875, 3): S(25) = 32.8125.
		kfCase(m, "ramp_up_mid", 25, VideoTrack("v1", kfClip("k", 0, 70,
			kfRamp(kfPoint(0, 1, easeInOut), kfPoint(50, 3, linear))))),
		// 1x eased down to 0.25x by frame 40, held to 60, eased back to 1x by 100: S(50) = 25 + 2.5 = 27.5.
		kfCase(m, "ramp_slow_middle_mid", 50, VideoTrack("v1", kfClip("k", 0, 100,
			kfRamp(kfPoint(0, 1, easeInOut), kfPoint(40, 0.25, linear), kfPoint(60, 0.25, easeInOut), kfPoint(100, 1, linear))))),
		// 3x eased out to 0.5x over frames 0..40, then 0.5x: S(60) = 46.5625 + 10 = 56.5625 (after the last point).
		kfCase(m, "ramp_down_late", 60, VideoTrack("v1", kfClip("k", 0, 80,
			kfRamp(kfPoint(0, 3, easeOut), kfPoint(40, 0.5, linear))))),
		// A ramp 1x -> 2x over frames 0..50 (S(t) = t + t^2/100) with x keys in source frames: frame 35 shows source
		// 47.25, so x is 0.05 + 0.5 x 47.25 / 80 = 0.3453125. Keyframes follow the ramped content.
		kfCase(m, "ramp_keyframed", 35, VideoTrack("v1", kfClip("k", 0, 70, kfBox(0.5, 0.3, 0.4, 0.4),
			kfRamp(kfPoint(0, 1, linear), kfPoint(50, 2, linear)),
			kfKeys("x", kfKey(0, 0.05, linear), kfKey(80, 0.55, linear))))),
	}
}
