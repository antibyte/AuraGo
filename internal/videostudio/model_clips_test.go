package videostudio

import (
	"fmt"
	"testing"
)

// speedTestProject is validProject with a 300-frame video asset and the clip under test.
func speedTestProject(mutate func(*Project)) Project {
	project := validProject()
	project.Assets[0].DurationFrames = 300
	mutate(&project)
	return project
}

// linkedTestProject puts count contiguous 30-frame clips in one link group.
func linkedTestProject(count int) Project {
	return speedTestProject(func(p *Project) {
		p.Tracks[0].Clips = nil
		for i := 0; i < count; i++ {
			p.Tracks[0].Clips = append(p.Tracks[0].Clips, Clip{ID: fmt.Sprintf("c%d", i), AssetID: "video", Start: i * 30, Duration: 30,
				Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, Link: "group"})
		}
	})
}

func TestValidateSpeedLinksDetachAndLabels(t *testing.T) {
	still := func(p *Project) {
		p.Assets = append(p.Assets, Asset{ID: "still", Name: "still.png", Kind: AssetImage, Width: 64, Height: 64})
	}
	withAudio := func(p *Project) { p.Assets[0].HasAudio = true }
	tests := []struct {
		name    string
		project Project
		valid   bool
	}{
		{"speed 2 reads two source frames per frame", speedTestProject(func(p *Project) { p.Tracks[0].Clips[0].Speed = 2; p.Tracks[0].Clips[0].Duration = 150 }), true},
		{"speed 2 needs the source for every frame", speedTestProject(func(p *Project) { p.Tracks[0].Clips[0].Speed = 2; p.Tracks[0].Clips[0].Duration = 151 }), false},
		{"speed below 0.25 is refused", speedTestProject(func(p *Project) { p.Tracks[0].Clips[0].Speed = 0.1 }), false},
		{"speed 16 is the maximum", speedTestProject(func(p *Project) { p.Tracks[0].Clips[0].Speed = 16; p.Tracks[0].Clips[0].Duration = 19 }), true},
		{"speed above 16 is refused", speedTestProject(func(p *Project) { p.Tracks[0].Clips[0].Speed = 16.5 }), false},
		{"images keep speed 1", speedTestProject(func(p *Project) { still(p); p.Tracks[0].Clips[0].AssetID = "still"; p.Tracks[0].Clips[0].Speed = 2 }), false},
		{"images play on video tracks", speedTestProject(func(p *Project) {
			still(p)
			p.Tracks[0].Clips[0].AssetID = "still"
			p.Tracks[0].Clips[0].Duration = 300
		}), true},
		{"detached audio needs sound on the video", speedTestProject(func(p *Project) { p.Tracks[0].Clips[0].AudioDetached = true }), false},
		{"detached audio on a video clip with sound", speedTestProject(func(p *Project) { withAudio(p); p.Tracks[0].Clips[0].AudioDetached = true }), true},
		{"detached audio only on video tracks", speedTestProject(func(p *Project) {
			withAudio(p)
			p.Tracks = append(p.Tracks, Track{ID: "music", Name: "Music", Kind: TrackAudio, Clips: []Clip{{ID: "song", AssetID: "video", Start: 40, Duration: 10, Volume: 1}}})
			p.Tracks[1].Clips[0].AudioDetached = true
		}), false},
		{"labels come from the palette", speedTestProject(func(p *Project) { p.Tracks[0].Clips[0].Label = "magenta" }), false},
		{"teal is a label", speedTestProject(func(p *Project) { p.Tracks[0].Clips[0].Label = "teal" }), true},
		{"seventeen clips cannot share a link", linkedTestProject(17), false},
		{"sixteen clips may share a link", linkedTestProject(16), true},
	}
	for _, test := range tests {
		if err := Validate(test.project); (err == nil) != test.valid {
			t.Fatalf("%s: Validate() = %v, want valid=%t", test.name, err, test.valid)
		}
	}
	if got := ClipSpeed(Clip{}); got != 1 {
		t.Fatalf("ClipSpeed(absent) = %v, want 1", got)
	}
	disabled := validProject()
	disabled.Tracks[0].Clips[0].Disabled = true
	if got := EnabledDuration(disabled); got != 0 {
		t.Fatalf("EnabledDuration() = %d, want 0 when every clip is disabled", got)
	}
	if got := Duration(disabled); got != 30 {
		t.Fatalf("Duration() = %d, want 30: disabled clips still count for limits", got)
	}
}
