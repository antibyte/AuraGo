package goldenframe

import "aurago/internal/videostudio"

// Foundation goldens (plan 2a, Task 13): the picture the preview drew before phase 2. That is placement, rotation,
// opacity, contain and cover, straight-alpha layers and the four transitions (fade through black at three points).
// The GL compositor and the 2D painter each draw every case, and each result is compared with the export's lossless frame.
//
// Thresholds (index §4.12): geometry, opacity and fit use ThresholdGeometry, transitions ThresholdTransition. A case
// with another threshold states the reason on the line above it.
//
// Paused seeks (plan 2a open question 12): TestDesktopVideoStudioGoldenFrames/seek_frame_starts showed the export's
// frame at exact frame starts (25 fps project, frames 1, 3, 6, 7, 12, 20 and 24, both painters, Chrome 155.0.8059.80 as logged).
// Paused seeks stay at frame starts (sourcePosition in video-studio-preview.js).
func init() { registerCases(foundationCases) }

const (
	// foundationFPS equals the generated videos' 25 fps: timeline frame n shows source frame n in export and preview,
	// so no frame-rate conversion enters a comparison.
	foundationFPS = 25
	// foundationTransition is 20 frames long, so p = 0.25, 0.5 and 0.75 fall on whole frames (5, 10 and 15 frames in).
	foundationTransition = 20
)

// foundationEdit changes one property of a clip built by foundationClip.
type foundationEdit func(*videostudio.Clip)

func foundationCases(m Media) []Case {
	gradient, portrait := foundationAsset(m, "gradient"), foundationAsset(m, "portrait")
	logo, red, blue := foundationAsset(m, "logo"), foundationAsset(m, "red"), foundationAsset(m, "blue")
	base := func(clips ...videostudio.Clip) videostudio.Track {
		return foundationTrack("v1", videostudio.TrackVideo, clips...)
	}
	upper := func(clips ...videostudio.Clip) videostudio.Track {
		return foundationTrack("v2", videostudio.TrackVideo, clips...)
	}
	overlay := func(clips ...videostudio.Clip) videostudio.Track {
		return foundationTrack("o1", videostudio.TrackOverlay, clips...)
	}
	// background is the moving gradient under the layer cases, long enough for every frame they use.
	background := foundationClip("bg", gradient, 0, 60)
	cases := []Case{
		foundationCase("geometry_full", ThresholdGeometry, 12, []videostudio.Asset{gradient},
			base(foundationClip("g", gradient, 0, 40))),
		foundationCase("geometry_box", ThresholdGeometry, 20, []videostudio.Asset{gradient},
			base(foundationClip("g", gradient, 0, 40, foundationBox(0.1, 0.15, 0.5, 0.5)))),
		foundationCase("geometry_rotate30", ThresholdGeometry, 30, []videostudio.Asset{gradient},
			base(foundationClip("g", gradient, 0, 40, foundationBox(0.2, 0.2, 0.6, 0.6), foundationRotation(30)))),
		foundationCase("geometry_logo_pip", ThresholdGeometry, 10, []videostudio.Asset{gradient, logo},
			base(background), overlay(foundationClip("logo", logo, 0, 40, foundationBox(0.6, 0.6, 0.3, 0.3)))),
		foundationCase("opacity_half_over_video", ThresholdGeometry, 16, []videostudio.Asset{gradient, red},
			base(background), overlay(foundationClip("red", red, 0, 40, foundationBox(0.25, 0.25, 0.5, 0.5), foundationOpacity(0.5)))),
		foundationCase("fit_contain_portrait", ThresholdGeometry, 15, []videostudio.Asset{portrait},
			base(foundationClip("p", portrait, 0, 40))),
		foundationCase("fit_cover_portrait", ThresholdGeometry, 15, []videostudio.Asset{portrait},
			base(foundationClip("p", portrait, 0, 40, foundationCover))),
		foundationCase("transition_dissolve_mid", ThresholdTransition, foundationFrame(0.5), []videostudio.Asset{gradient, portrait},
			base(foundationPair(videostudio.TransitionDissolve, gradient, portrait, nil, nil)...)),
	}
	// Fade through black on an upper track: the black is an opaque layer, so the gradient below darkens too.
	for _, point := range []struct {
		name string
		p    float64
	}{{"transition_black_25", 0.25}, {"transition_black_50", 0.5}, {"transition_black_75", 0.75}} {
		cases = append(cases, foundationCase(point.name, ThresholdTransition, foundationFrame(point.p), []videostudio.Asset{gradient, red, blue},
			base(background), upper(foundationPair(videostudio.TransitionBlack, red, blue,
				[]foundationEdit{foundationBox(0.1, 0.1, 0.4, 0.4)}, []foundationEdit{foundationBox(0.5, 0.5, 0.4, 0.4)})...)))
	}
	cases = append(cases,
		foundationCase("transition_wipeleft_mid", ThresholdTransition, foundationFrame(0.5), []videostudio.Asset{gradient, portrait},
			base(foundationPair(videostudio.TransitionWipeLeft, gradient, portrait, nil, nil)...)),
		foundationCase("transition_wiperight_mid", ThresholdTransition, foundationFrame(0.5), []videostudio.Asset{gradient, portrait},
			base(foundationPair(videostudio.TransitionWipeRight, gradient, portrait, nil, nil)...)),
		// Premultiplied layer semantics: where both overlay clips are transparent the gradient stays untouched.
		foundationCase("transition_dissolve_overlay_over_video", ThresholdTransition, foundationFrame(0.5), []videostudio.Asset{gradient, logo, red},
			base(background), overlay(foundationPair(videostudio.TransitionDissolve, logo, red,
				[]foundationEdit{foundationBox(0.05, 0.1, 0.5, 0.5)}, []foundationEdit{foundationBox(0.45, 0.4, 0.5, 0.5)})...)),
	)
	return cases
}

func foundationAsset(m Media, name string) videostudio.Asset {
	return videostudio.AssetAtRate(m.Assets[name], videostudio.Rate{Num: foundationFPS, Den: 1})
}

func foundationCase(name string, threshold float64, frame int, assets []videostudio.Asset, tracks ...videostudio.Track) Case {
	return Case{
		Name: name, Frame: frame, Threshold: threshold, Painters: []string{"gl", "2d"},
		Project: videostudio.Project{Version: videostudio.CurrentVersion, Name: name, Width: 1280, Height: 720, FPS: foundationFPS, Assets: assets, Tracks: tracks},
	}
}

func foundationTrack(id string, kind videostudio.TrackKind, clips ...videostudio.Clip) videostudio.Track {
	return videostudio.Track{ID: id, Kind: kind, Name: id, Clips: clips}
}

// foundationClip is a full-canvas, opaque, contained clip of asset at timeline frames [start, start+duration).
func foundationClip(id string, asset videostudio.Asset, start, duration int, edits ...foundationEdit) videostudio.Clip {
	clip := videostudio.Clip{ID: id, AssetID: asset.ID, Start: start, Duration: duration, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: videostudio.FitContain}
	for _, edit := range edits {
		edit(&clip)
	}
	return clip
}

// foundationPair joins clip a (frames 0–39) to clip b (frames 20–59) with a transition over frames 20–39, so timeline
// frame foundationFrame(p) is transition progress p.
func foundationPair(kind videostudio.TransitionType, a, b videostudio.Asset, editA, editB []foundationEdit) []videostudio.Clip {
	into := func(clip *videostudio.Clip) {
		clip.Transition = &videostudio.Transition{Type: kind, Duration: foundationTransition}
	}
	return []videostudio.Clip{
		foundationClip("a", a, 0, 40, append(append([]foundationEdit{}, editA...), into)...),
		foundationClip("b", b, 20, 40, editB...),
	}
}

func foundationFrame(p float64) int { return 20 + int(p*foundationTransition) }

func foundationBox(x, y, width, height float64) foundationEdit {
	return func(clip *videostudio.Clip) { clip.X, clip.Y, clip.Width, clip.Height = x, y, width, height }
}

func foundationRotation(degrees float64) foundationEdit {
	return func(clip *videostudio.Clip) { clip.Rotation = degrees }
}

func foundationOpacity(value float64) foundationEdit {
	return func(clip *videostudio.Clip) { clip.Opacity = value }
}

func foundationCover(clip *videostudio.Clip) { clip.Fit = videostudio.FitCover }
