package videostudio

import (
	"fmt"
	"math"
	"testing"
)

func validProject() Project {
	return Project{
		Version: 1, Name: "Test", Width: 1280, Height: 720, FPS: 30,
		Assets: []Asset{{ID: "video", Name: "video.mp4", Kind: AssetVideo, DurationFrames: 300, Width: 640, Height: 360}},
		Tracks: []Track{{
			ID: "track", Name: "Video", Kind: TrackVideo,
			Clips: []Clip{{
				ID: "clip", AssetID: "video", Start: 0, Duration: 30,
				Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain,
			}},
		}},
	}
}

func TestDurationAndValidate(t *testing.T) {
	project := validProject()
	if got := Duration(project); got != 30 {
		t.Fatalf("Duration() = %d, want 30", got)
	}
	if err := Validate(project); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	project.Tracks[0].Clips[0].Start = 60
	if got := Duration(project); got != 90 {
		t.Fatalf("Duration() after move = %d, want 90", got)
	}
}

func TestValidateTransitionMustMatchOnlyAdjacentOverlap(t *testing.T) {
	project := validProject()
	project.Tracks[0].Clips = []Clip{
		{ID: "out", AssetID: "video", Start: 0, Duration: 60, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, Transition: &Transition{Type: TransitionDissolve, Duration: 10}},
		{ID: "in", AssetID: "video", Start: 50, Duration: 30, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain},
	}
	if err := Validate(project); err != nil {
		t.Fatalf("valid overlap rejected: %v", err)
	}

	project.Tracks[0].Clips[0].Transition.Duration = 9
	if err := Validate(project); err == nil {
		t.Fatal("transition with an overlap mismatch was accepted")
	}
	project.Tracks[0].Clips[0].Transition.Duration = 10
	project.Tracks[0].Clips[1].Start = 60
	if err := Validate(project); err == nil {
		t.Fatal("transition without an explicit overlap was accepted")
	}
	project.Tracks[0].Clips[0].Transition = nil
	project.Tracks[0].Clips[1].Start = 50
	if err := Validate(project); err == nil {
		t.Fatal("overlap without a transition was accepted")
	}
}

func TestValidateRejectsTripleOverlap(t *testing.T) {
	project := validProject()
	project.Tracks[0].Clips = []Clip{
		{ID: "a", AssetID: "video", Start: 0, Duration: 90, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, Transition: &Transition{Type: TransitionDissolve, Duration: 30}},
		{ID: "b", AssetID: "video", Start: 60, Duration: 90, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, Transition: &Transition{Type: TransitionDissolve, Duration: 70}},
		{ID: "c", AssetID: "video", Start: 80, Duration: 30, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain},
	}
	if err := Validate(project); err == nil {
		t.Fatal("three overlapping clips were accepted")
	}
}

func TestValidateBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Project)
	}{
		{"unsupported size", func(p *Project) { p.Width = 640 }},
		{"wrong fps", func(p *Project) { p.FPS = 24 }},
		{"too long", func(p *Project) { p.Tracks[0].Clips[0].Duration = MaxDurationFrames + 1 }},
		{"start exceeds timeline bound", func(p *Project) { p.Tracks[0].Clips[0].Start = math.MaxInt }},
		{"offset exceeds source bound", func(p *Project) { p.Tracks[0].Clips[0].Offset = math.MaxInt }},
		{"source offset addition wraps", func(p *Project) {
			p.Assets[0].DurationFrames = math.MaxInt
			p.Tracks[0].Clips[0].Offset = math.MaxInt - 5
			p.Tracks[0].Clips[0].Duration = 10
		}},
		{"out of bounds", func(p *Project) { p.Tracks[0].Clips[0].X = 0.2 }},
		{"non-finite opacity", func(p *Project) { p.Tracks[0].Clips[0].Opacity = math.NaN() }},
		{"excessive source dimensions", func(p *Project) { p.Assets[0].Width = MaxSourceDimension + 1 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project := validProject()
			test.change(&project)
			if err := Validate(project); err == nil {
				t.Fatal("invalid project was accepted")
			}
		})
	}
}

func TestDurationSaturatesOnIntegerOverflow(t *testing.T) {
	project := validProject()
	project.Tracks[0].Clips[0].Start = math.MaxInt - 5
	if got := Duration(project); got != math.MaxInt {
		t.Fatalf("Duration() = %d, want saturation at MaxInt", got)
	}
	if err := Validate(project); err == nil {
		t.Fatal("project with overflowing timeline end was accepted")
	}
}

func TestValidateLimitsDistinctRenderInputs(t *testing.T) {
	project := validProject()
	project.Assets = make([]Asset, MaxRenderInputs+1)
	clips := make([]Clip, MaxRenderInputs+1)
	for i := range project.Assets {
		id := fmt.Sprintf("video-%d", i)
		project.Assets[i] = Asset{ID: id, Name: id + ".mp4", Kind: AssetVideo, DurationFrames: 30, Width: 64, Height: 36}
		clips[i] = Clip{ID: fmt.Sprintf("clip-%d", i), AssetID: id, Start: i * 30, Duration: 30, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain}
	}
	project.Tracks[0].Clips = clips
	if err := Validate(project); err == nil {
		t.Fatal("project with too many render inputs was accepted")
	}
}
