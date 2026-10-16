package goldenframe

import (
	"slices"
	"testing"

	"aurago/internal/videostudio"
)

// foundationTestMedia stands in for MakeMedia: the five generated assets with their probed shapes and no files.
func foundationTestMedia() Media {
	video := func(id string, width, height int, seconds int64) videostudio.Asset {
		us := seconds * 1_000_000
		return videostudio.Asset{ID: id, Name: id + ".mp4", Kind: videostudio.AssetVideo, Width: width, Height: height, DurationUs: us, VideoDurationUs: us,
			DurationFrames: videostudio.Rate{Num: 30, Den: 1}.FramesAt(us), SourceFPS: "25/1", Codec: "h264"}
	}
	image := func(id string, width, height int) videostudio.Asset {
		return videostudio.Asset{ID: id, Name: id + ".png", Kind: videostudio.AssetImage, Width: width, Height: height}
	}
	m := Media{Paths: map[string]string{}, Files: map[string]string{}, Assets: map[string]videostudio.Asset{
		"gradient": video("asset-gradient", 1280, 720, 3), "portrait": video("asset-portrait", 720, 1280, 2),
		"logo": image("asset-logo", 640, 360), "red": image("asset-red", 320, 180), "blue": image("asset-blue", 320, 180),
	}}
	for _, asset := range m.Assets {
		m.Paths[asset.ID] = asset.Name
		m.Files[asset.ID] = asset.Name
	}
	return m
}

// foundationProgressAt is the transition progress p = (frame - incoming start) / duration of the case's transition.
func foundationProgressAt(c Case) (float64, bool) {
	for _, track := range c.Project.Tracks {
		for i, clip := range track.Clips {
			if clip.Transition != nil && i+1 < len(track.Clips) {
				return float64(c.Frame-track.Clips[i+1].Start) / float64(clip.Transition.Duration), true
			}
		}
	}
	return 0, false
}

func TestFoundationCasesCoverTodaysFeatures(t *testing.T) {
	want := map[string]float64{
		"geometry_full": ThresholdGeometry, "geometry_box": ThresholdGeometry, "geometry_rotate30": ThresholdGeometry,
		"geometry_logo_pip": ThresholdGeometry, "opacity_half_over_video": ThresholdGeometry,
		"fit_contain_portrait": ThresholdGeometry, "fit_cover_portrait": ThresholdGeometry,
		"transition_dissolve_mid": ThresholdTransition, "transition_black_25": ThresholdTransition,
		"transition_black_50": ThresholdTransition, "transition_black_75": ThresholdTransition,
		"transition_wipeleft_mid": ThresholdTransition, "transition_wiperight_mid": ThresholdTransition,
		"transition_dissolve_overlay_over_video": ThresholdTransition,
	}
	progress := map[string]float64{
		"transition_dissolve_mid": 0.5, "transition_black_25": 0.25, "transition_black_50": 0.5, "transition_black_75": 0.75,
		"transition_wipeleft_mid": 0.5, "transition_wiperight_mid": 0.5, "transition_dissolve_overlay_over_video": 0.5,
	}
	m := foundationTestMedia()
	seen := map[string]bool{}
	for _, c := range foundationCases(m) {
		threshold, known := want[c.Name]
		if !known {
			t.Errorf("unexpected foundation case %q", c.Name)
			continue
		}
		if seen[c.Name] {
			t.Errorf("duplicate foundation case %q", c.Name)
		}
		seen[c.Name] = true
		if c.Threshold != threshold {
			t.Errorf("%s: threshold %.2f, want %.2f", c.Name, c.Threshold, threshold)
		}
		if !slices.Equal(c.Painters, []string{"gl", "2d"}) {
			t.Errorf("%s: painters %v, want [gl 2d]", c.Name, c.Painters)
		}
		if err := videostudio.Validate(c.Project); err != nil {
			t.Errorf("%s: invalid project: %v", c.Name, err)
		}
		if c.Frame < 0 || c.Frame >= videostudio.EnabledDuration(c.Project) {
			t.Errorf("%s: frame %d lies outside the project", c.Name, c.Frame)
		}
		if p, ok := progress[c.Name]; ok {
			if got, found := foundationProgressAt(c); !found || got != p {
				t.Errorf("%s: frame %d is transition progress %v (found %v), want %v", c.Name, c.Frame, got, found, p)
			}
		}
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("foundation case %q is missing", name)
		}
	}
	registered := map[string]bool{}
	for _, c := range Cases(m) {
		registered[c.Name] = true
	}
	for name := range want {
		if !registered[name] {
			t.Errorf("Cases does not list %q: foundationCases is not registered", name)
		}
	}
}
