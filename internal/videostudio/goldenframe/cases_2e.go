package goldenframe

import (
	"strconv"

	"aurago/internal/videostudio"
)

// Transition and animation goldens (plan 2e, Task 13; index §4.12, §4.24).
//
// Transitions. There is one case per 2e transition; the 2a four are in cases_foundation.go.
//   - Setup: the chart (frames 0-39) into the detail chart (frames 20-59) on one video track, over a 20-frame overlap.
//     Both are Go-written 640x360 PNGs contained at exactly 2x, so no yuv420p conversion enters a comparison.
//   - Painters: GL only. The 2D painter draws every 2e transition as a dissolve (index §4.5).
//   - Overlay cases: a logo pair on an overlay track over the moving gradient. circlecrop's opaque fill covers the
//     gradient (risk 28); the slideleft push moves the logos' transparent surroundings.
//   - Progress: 50 % (overlap frame 10 of 20), unless txPercent names another point and why. No case sits on overlap
//     frame 0 or on the last frame, so the documented quirks (D3) never enter a golden:
//     the 1 px of B of wiperight/wipedown, the rectcrop border, the slideleft/slideup out-of-range column and row, and
//     the radial wedge. The xfade probe fixture (Task 10) and the GL probe test (Task 11) cover them.
//   - Threshold: ThresholdTransition.
//   - Delta: the baseline is the same pair dissolving at the same frame (index §4.12). The transitions that move picture
//     content take DeltaMask's bounds, the others DeltaBlend's (txMoving).
//
// Animations. The animated clip is the logo in the box (0.2, 0.2, 0.6, 0.6) on an overlay track over the chart.
//   - Timing: it starts at frame 10 and lasts 50 frames. Its animation lasts 20 frames (0.8 s; AnimFrameRange(25 fps)
//     is 5..75). In-progress p is frame 10 + 20p, out-progress p is frame 59 − 20p (index §4.24: the clip's first and
//     last frames are fully hidden).
//   - In cases: one per preset, at p = 0.5, except the four slides at p = 0.3. At 0.5 a slide is only 1.875 % of the
//     canvas off its rest (14 px vertically, 24 px horizontally), which a preview that dropped the offset could pass.
//     At 0.3 it is 5.145 % (38 px, 66 px).
//   - anim_blur_in_in animates detail.png instead of the logo. The logo is smooth, so the cascade (S = 7.2 px on a
//     360 px still) would barely change it, and neither SSIM nor the delta could tell a missing blur.
//   - Two out cases (slide_left, pop) and six combined cases (mask, keyframes, rotation, a dissolve, a ramp, blur_in on
//     video).
//   - anim_blur_in_video uses the ticker video, cover-fitted into a tall box. Its 32 px stripes show the blur; the
//     gradient video's ~1000 px features would not.
//   - Painters: GL and 2D, except the blur and mask cases, which are GL only (the 2D painter draws no blur and no mask,
//     index §4.5).
//   - Thresholds: ThresholdGeometry; ThresholdDetail for the blur and mask cases (index §4.12).
//   - Delta: the baseline is the same project without any animation (index §4.12). Fade cases take DeltaBlend's bounds,
//     because their change is a smooth opacity. Every other case takes DeltaMask's: the change of a moving, scaled,
//     revealed or blurred layer sits at its edges and in its detail. There the export's bicubic and the preview's
//     bilinear sampling differ by a fraction of a pixel (delta.go).
//
// cases_2e_test.go pins every case's animation state at its frame from the law of §4.24 (anLaw). A case with another
// threshold or other delta bounds states the measured reason on the line above it.
//
// cases_2e.go sorts between cases_2d.go and cases_foundation.go, so these cases register fourth. The order has no
// meaning.
func init() { registerCases(transitionAnimCases) }

// txDuration is the overlap of every transition pair: overlap frame i (0..19) is timeline frame 20 + i.
const txDuration = 20

// txFoundation lists the 2a transitions; cases_foundation.go has their cases.
var txFoundation = map[videostudio.TransitionType]bool{
	videostudio.TransitionDissolve: true, videostudio.TransitionBlack: true, videostudio.TransitionWipeLeft: true, videostudio.TransitionWipeRight: true,
}

// txPercent is the progress, in percent of the overlap, of the transitions whose mid frame (50 %) does not serve:
//   - circlecrop: at 50 % P = 0.5 gives r = 0, so the frame is black. At 30 % (P = 0.7) r = f(0.4³·R) ≈ 47 px of A
//     shows inside black.
//   - rectcrop: at 50 % zw = zh = 0, black again. At 30 % a rectangle of A, about 0.4 of the frame wide and high, shows
//     inside black.
//   - zoomin: at 50 % zf = 0, so every pixel samples A's centre texel (a flat frame). At 30 % zf = 0.352 and w = 1: A
//     magnified 2.84x about the centre.
//   - pixelize: at 50 % sq = 36 px is a whole number, so x/sq is whole on every block edge. There GL's division may land
//     just below it and move the column into the neighbouring block. At 25 % (P = 0.75) sq = 18.72 px, and the outgoing
//     picture weighs 0.75.
var txPercent = map[string]int{"circlecrop": 30, "rectcrop": 30, "zoomin": 30, "pixelize": 25}

// txMoving lists the transitions that move picture content: the pushes, the squeeze, the zoom and the pixelize mosaic.
// Their change against the dissolve carries the small difference between the export's bicubic and the preview's
// bilinear layers to new places, so they take DeltaMask's r bound (0.95). The other transitions choose or mix the two
// layers in place and take DeltaBlend's (0.98).
var txMoving = map[string]bool{"pixelize": true, "slideleft": true, "slideright": true, "slideup": true, "slidedown": true, "squeezeh": true, "zoomin": true}

func txDelta(id string) Delta {
	if txMoving[id] {
		return DeltaMask
	}
	return DeltaBlend
}

// txInto gives the outgoing clip a transition of type kind over txDuration frames.
func txInto(kind videostudio.TransitionType) pcEdit {
	return func(clip *videostudio.Clip) {
		clip.Transition = &videostudio.Transition{Type: kind, Duration: txDuration}
	}
}

// txPair is the chart (frames 0-39) into the detail chart (frames 20-59) on one video track.
func txPair(kind videostudio.TransitionType) []videostudio.Track {
	return []videostudio.Track{VideoTrack("v1", pcClip("a", MediaChart, 0, 40, txInto(kind)), pcClip("b", MediaDetail, 20, 40))}
}

// txOverlayPair puts two logos on an overlay track over the moving gradient (frames 0-59):
//   - a (frames 0-39) in the centred box (0.15, 0.15, 0.7, 0.7);
//   - b (frames 20-59) in the box (0.3, 0.25, 0.6, 0.6).
func txOverlayPair(kind videostudio.TransitionType) []videostudio.Track {
	return []videostudio.Track{VideoTrack("v1", pcClip("bg", MediaGradient, 0, 60)),
		OverlayTrack("o1", pcClip("a", MediaLogo, 0, 40, pcBox(0.15, 0.15, 0.7, 0.7), txInto(kind)), pcClip("b", MediaLogo, 20, 40, pcBox(0.3, 0.25, 0.6, 0.6)))}
}

// txCase is a GL-only transition case at pct % of the overlap whose Delta baseline is the same pair dissolving.
func txCase(m Media, name string, pct int, pair func(videostudio.TransitionType) []videostudio.Track, kind videostudio.TransitionType, delta Delta) Case {
	c := pcCase(m, name, txDuration+pct*txDuration/100, ThresholdTransition, []string{PainterGL}, pair(kind)...)
	return withDelta(c, delta, m.Project(name+"_baseline", pair(videostudio.TransitionDissolve)...))
}

func transitionAnimCases(m Media) []Case {
	var cases []Case
	for _, spec := range videostudio.EffectRegistry().Transitions {
		if txFoundation[spec.ID] {
			continue
		}
		id := string(spec.ID)
		pct := txPercent[id]
		if pct == 0 {
			pct = 50
		}
		cases = append(cases, txCase(m, "transition_"+id+"_"+strconv.Itoa(pct), pct, txPair, spec.ID, txDelta(id)))
	}
	cases = append(cases,
		// circlecrop at 30 % (txPercent): the opaque centre of logo a inside a 47 px circle; the black fill covers the
		// gradient everywhere else.
		txCase(m, "transition_circlecrop_overlay", 30, txOverlayPair, "circlecrop", txDelta("circlecrop")),
		// slideleft at 50 % (s = 640 px): a's right half on the left of the canvas, b's left half on the right, both over
		// the gradient.
		txCase(m, "transition_slideleft_overlay", 50, txOverlayPair, "slideleft", txDelta("slideleft")),
	)
	return append(cases, animCases(m)...)
}

// The animated clips start at anStart and last anLength frames. Every animation lasts anSpan frames, except in
// anim_pop_ramp.
const anStart, anLength, anSpan = 10, 50, 20

// anIn and anOut set a clip's in or out animation (a registry preset, timeline frames).
func anIn(preset string, frames int) pcEdit {
	return func(clip *videostudio.Clip) { clip.AnimIn = &videostudio.ClipAnim{Preset: preset, Duration: frames} }
}

func anOut(preset string, frames int) pcEdit {
	return func(clip *videostudio.Clip) { clip.AnimOut = &videostudio.ClipAnim{Preset: preset, Duration: frames} }
}

// anOverlay puts the still assetID in the box (0.2, 0.2, 0.6, 0.6) on an overlay track over the chart, at frames
// anStart..anStart+anLength-1. The box is 768x432, the media's own 16:9.
func anOverlay(assetID string, edits ...pcEdit) []videostudio.Track {
	return []videostudio.Track{VideoTrack("v1", pcClip("bg", MediaChart, 0, 60)),
		OverlayTrack("o1", pcClip("l", assetID, anStart, anLength, cdWith([]pcEdit{pcBox(0.2, 0.2, 0.6, 0.6)}, edits...)...))}
}

// anStill returns tracks with AnimIn and AnimOut cleared on every clip. The clip slices are new, so the case keeps its
// animations.
func anStill(tracks []videostudio.Track) []videostudio.Track {
	out := make([]videostudio.Track, len(tracks))
	for i, track := range tracks {
		track.Clips = append([]videostudio.Clip{}, track.Clips...)
		for j := range track.Clips {
			track.Clips[j].AnimIn, track.Clips[j].AnimOut = nil, nil
		}
		out[i] = track
	}
	return out
}

// anCase is an animation case whose Delta baseline is the same project with every animation removed.
func anCase(m Media, name string, frame int, threshold float64, painters []string, delta Delta, tracks []videostudio.Track) Case {
	return withDelta(pcCase(m, name, frame, threshold, painters, tracks...), delta, m.Project(name+"_baseline", anStill(tracks)...))
}

func animCases(m Media) []Case {
	gl, both := []string{PainterGL}, []string{PainterGL, Painter2D}
	logo := func(edits ...pcEdit) []videostudio.Track { return anOverlay(MediaLogo, edits...) }
	return []Case{
		// fade in at p = 0.5 (frame 20): opacity 0.5.
		anCase(m, "anim_fade_in", 20, ThresholdGeometry, both, DeltaBlend, logo(anIn("fade", anSpan))),
		// slide_up in at p = 0.3 (frame 16). The box centre sits 38 px below its rest and moves up: the offset is 0.05145
		// of the canvas height, rounded to even px. Opacity 0.3.
		anCase(m, "anim_slide_up_in", 16, ThresholdGeometry, both, DeltaMask, logo(anIn("slide_up", anSpan))),
		// slide_down in at p = 0.3: 38 px above its rest; opacity 0.3.
		anCase(m, "anim_slide_down_in", 16, ThresholdGeometry, both, DeltaMask, logo(anIn("slide_down", anSpan))),
		// slide_left in at p = 0.3: 66 px right of its rest (0.05145 of the canvas width); opacity 0.3.
		anCase(m, "anim_slide_left_in", 16, ThresholdGeometry, both, DeltaMask, logo(anIn("slide_left", anSpan))),
		// slide_right in at p = 0.3: 66 px left of its rest; opacity 0.3.
		anCase(m, "anim_slide_right_in", 16, ThresholdGeometry, both, DeltaMask, logo(anIn("slide_right", anSpan))),
		// pop in at p = 0.5 (frame 20). Scale 0.6 + 0.48·e_out(0.5/0.7) = 1.0688, so the 768x432 box is 820x462 about the
		// same centre. Opacity min(1, 0.5/0.3) = 1.
		anCase(m, "anim_pop_in", 20, ThresholdGeometry, both, DeltaMask, logo(anIn("pop", anSpan))),
		// wipe in at p = 0.5: box columns [0, 384) of 768 show (R = round(0.5·768)).
		anCase(m, "anim_wipe_in", 20, ThresholdGeometry, both, DeltaMask, logo(anIn("wipe", anSpan))),
		// Threshold ThresholdDetail, not ThresholdGeometry: a blur (index §4.12). blur_in at p = 0.5 on the detail chart:
		// m1 = round(255·0.5/0.75) = 170 and m2 = 0, so the picture is MM(g(7.2), g(1.8), 170) at opacity 0.5.
		anCase(m, "anim_blur_in_in", 20, ThresholdDetail, gl, DeltaMask, anOverlay(MediaDetail, anIn("blur_in", anSpan))),
		// slide_left out at p = 0.3 (frame 53): the box leaves to the left and is 66 px left of its rest; opacity 0.3.
		anCase(m, "anim_slide_left_out", 53, ThresholdGeometry, both, DeltaMask, logo(anOut("slide_left", anSpan))),
		// pop out at p = 0.5 (frame 49): scale 1.0688 again, opacity 1.
		anCase(m, "anim_pop_out", 49, ThresholdGeometry, both, DeltaMask, logo(anOut("pop", anSpan))),
		// Threshold ThresholdDetail: a mask (index §4.12). The chart in the box carries the ellipse mask (0.1, 0.1, 0.8,
		// 0.8; feather 0.03) over the blue still and pops in at p = 0.5. The mask follows the 820x462 box (box space; pop
		// takes the pad-canvas path).
		anCase(m, "anim_pop_mask", 20, ThresholdDetail, gl, DeltaMask, []videostudio.Track{VideoTrack("v1", pcClip("bg", MediaBlue, 0, 60)),
			OverlayTrack("o1", pcClip("l", MediaChart, anStart, anLength, pcBox(0.2, 0.2, 0.6, 0.6),
				cdMask(videostudio.MaskEllipse, 0.1, 0.1, 0.8, 0.8, 0, 0.03, false), anIn("pop", anSpan)))}),
		// x keys 0.05 -> 0.35 over source frames 0..40. The still starts at frame 10, so frame 16 is source 6 and x is
		// 0.095 (the static x is 0.2). Under them a slide_right in at p = 0.3: the keyed box, then 66 px left of it.
		anCase(m, "anim_slide_keyframed", 16, ThresholdGeometry, both, DeltaMask, logo(pcKeys("x", pcKey(0, 0.05), pcKey(40, 0.35)), anIn("slide_right", anSpan))),
		// The logo turned 20 degrees with a wipe out at p = 0.5 (frame 49): box columns [384, 768) show, in the turned box.
		anCase(m, "anim_wipe_rotated", 49, ThresholdGeometry, both, DeltaMask, logo(func(clip *videostudio.Clip) { clip.Rotation = 20 }, anOut("wipe", anSpan))),
		// The chart (frames 0-39) dissolves into the detail chart (frames 20-59), which fades in over 20 frames. Frame 30
		// is dissolve progress 0.5 and b's in-progress 0.5 (opacity 0.5): each clip's animation plays inside the
		// transition.
		anCase(m, "anim_fade_dissolve", 30, ThresholdGeometry, both, DeltaBlend, []videostudio.Track{VideoTrack("v1",
			pcClip("a", MediaChart, 0, 40, cdDissolve(txDuration)), pcClip("b", MediaDetail, 20, 40, anIn("fade", anSpan)))}),
		// The ticker in the box on a video track with ramp_up_mid's ramp: 1x -> 3x, eased in and out over clip frames
		// 0..50, so frame 25 shows source 32.8125. A pop in of 60 frames on the 70-frame clip plays over half of it,
		// 35 frames (the clamp, index §4.24). Frame 25 is p = 25/35: scale 1.08 − 0.08·sm(0.0476) = 1.0795, opacity 1.
		// Spans count timeline frames, not source frames.
		anCase(m, "anim_pop_ramp", 25, ThresholdGeometry, both, DeltaMask, []videostudio.Track{VideoTrack("v1", pcClip("k", MediaTicker, 0, 70, pcBox(0.2, 0.2, 0.6, 0.6),
			pcEdit(kfRamp(kfPoint(0, 1, videostudio.EaseInOut), kfPoint(50, 3, videostudio.EaseLinear))), anIn("pop", 60)))}),
		// Threshold ThresholdDetail: a blur. blur_in on video uses the opaque cascade (index §4.24). The ticker is
		// cover-fitted into the tall box (0.25, 0.1, 0.5, 0.8) on a second video track over the chart. At p = 0.5:
		// m1 170, m2 0 (S = 3.6 px on the 180 px ticker), opacity 0.5.
		anCase(m, "anim_blur_in_video", 20, ThresholdDetail, gl, DeltaMask, []videostudio.Track{VideoTrack("v1", pcClip("bg", MediaChart, 0, 60)),
			VideoTrack("v2", pcClip("k", MediaTicker, anStart, anLength, pcBox(0.25, 0.1, 0.5, 0.8), pcCover, anIn("blur_in", anSpan)))}),
	}
}
