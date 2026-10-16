// Package goldenframe is the golden-frame harness of Video Studio phase 2 (plan index §4.12): SSIM with the agreed
// thresholds, generated media, and the registered cases with their project builders. The export side renders each
// case with videostudio.RenderFrame, the browser side with the preview engine; tests compare the two. Only tests
// import this package.
package goldenframe

import (
	"sort"

	"aurago/internal/videostudio"
)

// Every case project uses this canvas and rate.
const (
	ProjectWidth  = 1280
	ProjectHeight = 720
	ProjectFPS    = 25
)

// Painter names of Case.Painters (the preview's painter option).
const (
	PainterGL = "gl"
	Painter2D = "2d"
)

// Case is one golden frame.
type Case struct {
	// Name is "<group>_<detail>"; groups: geometry, opacity, fit, transition, keyframe, ramp, color, lut, crop, flip,
	// blur, sharpen, vignette, key, blend, mask, anim.
	Name      string
	Project   videostudio.Project
	Frame     int
	Threshold float64
	Painters  []string // PainterGL and/or Painter2D
}

var caseSets []func(Media) []Case

// registerCases adds a case set. Each slice registers its own from an init function in its cases file
// (cases_foundation.go, cases_2b.go, …); shared files are not edited.
func registerCases(set func(Media) []Case) { caseSets = append(caseSets, set) }

// Cases returns every registered case set, in registration order (Go runs init functions in file-name order).
func Cases(m Media) []Case {
	var all []Case
	for _, set := range caseSets {
		all = append(all, set(m)...)
	}
	return all
}

// Project returns a case project at ProjectWidth x ProjectHeight and ProjectFPS holding every media asset in id order,
// with durations expressed at ProjectFPS, and the given tracks (array order is paint order, first at the bottom).
func (m Media) Project(name string, tracks ...videostudio.Track) videostudio.Project {
	rate := videostudio.Rate{Num: ProjectFPS, Den: 1}
	ids := make([]string, 0, len(m.Assets))
	for id := range m.Assets {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	assets := make([]videostudio.Asset, 0, len(ids))
	for _, id := range ids {
		assets = append(assets, videostudio.AssetAtRate(m.Assets[id], rate))
	}
	return videostudio.Project{Version: videostudio.CurrentVersion, Name: name, Width: ProjectWidth, Height: ProjectHeight, FPS: ProjectFPS,
		Assets: assets, Tracks: append([]videostudio.Track{}, tracks...)}
}

// VideoTrack returns a visible video track named after its id.
func VideoTrack(id string, clips ...videostudio.Clip) videostudio.Track {
	return videostudio.Track{ID: id, Kind: videostudio.TrackVideo, Name: id, Clips: append([]videostudio.Clip{}, clips...)}
}

// OverlayTrack returns a visible overlay track (image clips only) named after its id.
func OverlayTrack(id string, clips ...videostudio.Clip) videostudio.Track {
	return videostudio.Track{ID: id, Kind: videostudio.TrackOverlay, Name: id, Clips: append([]videostudio.Clip{}, clips...)}
}

// NewClip returns an enabled full-frame contain clip at full opacity; cases adjust the fields they test.
func NewClip(id, assetID string, start, duration int) videostudio.Clip {
	return videostudio.Clip{ID: id, AssetID: assetID, Start: start, Duration: duration, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: videostudio.FitContain}
}
