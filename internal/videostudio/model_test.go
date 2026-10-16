package videostudio

import (
	"fmt"
	"math"
	"testing"
)

func validProject() Project {
	return Project{
		Version: CurrentVersion, Name: "Test", Width: 1280, Height: 720, FPS: 30,
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
		{"wrong fps", func(p *Project) { p.FPS = 23 }},
		{"too long", func(p *Project) { p.Tracks[0].Clips[0].Start = MaxDurationFrames(*p) }},
		{"ntsc at 25 fps", func(p *Project) { p.FPS = 25; p.NTSC = true }},
		{"bad source fps", func(p *Project) { p.Assets[0].SourceFPS = "30fps" }},
		{"bad codec", func(p *Project) { p.Assets[0].Codec = "H 264" }},
		{"negative microseconds", func(p *Project) { p.Assets[0].DurationUs = -1 }},
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

func TestValidateV2RatesSizesAndLimits(t *testing.T) {
	for _, fps := range []int{24, 25, 30, 50, 60} {
		p := validProject()
		p.FPS = fps
		if err := Validate(p); err != nil {
			t.Fatalf("%d fps rejected: %v", fps, err)
		}
	}
	for _, fps := range []int{24, 30, 60} {
		p := validProject()
		p.FPS, p.NTSC = fps, true
		if err := Validate(p); err != nil {
			t.Fatalf("NTSC %d fps rejected: %v", fps, err)
		}
	}
	for _, size := range CanvasSizes() {
		p := validProject()
		p.Width, p.Height = size[0], size[1]
		if err := Validate(p); err != nil {
			t.Fatalf("canvas %dx%d rejected: %v", size[0], size[1], err)
		}
	}
	if len(CanvasSizes()) != 11 {
		t.Fatalf("CanvasSizes() has %d entries, want 11", len(CanvasSizes()))
	}

	tracks := validProject()
	for i := 1; i < MaxVideoTracks; i++ {
		tracks.Tracks = append(tracks.Tracks, Track{ID: fmt.Sprintf("v%d", i), Name: "Video", Kind: TrackVideo})
	}
	for i := 0; i < MaxAudioTracks; i++ {
		tracks.Tracks = append(tracks.Tracks, Track{ID: fmt.Sprintf("a%d", i), Name: "Audio", Kind: TrackAudio})
	}
	for i := 0; i < MaxOverlayTracks; i++ {
		tracks.Tracks = append(tracks.Tracks, Track{ID: fmt.Sprintf("o%d", i), Name: "Overlay", Kind: TrackOverlay})
	}
	if err := Validate(tracks); err != nil {
		t.Fatalf("eight tracks per kind rejected: %v", err)
	}
	tracks.Tracks = append(tracks.Tracks, Track{ID: "v-extra", Name: "Video", Kind: TrackVideo})
	if err := Validate(tracks); err == nil {
		t.Fatal("a ninth video track was accepted")
	}

	// The hour limit is in seconds: a 59-minute clip fits at 60 fps.
	long := validProject()
	long.FPS = 60
	long.Assets[0].DurationFrames, long.Assets[0].DurationUs = MaxDurationFrames(long), 3_600_000_000
	long.Tracks[0].Clips[0].Duration = 59 * 60 * 60
	if err := Validate(long); err != nil {
		t.Fatalf("59 minutes at 60 fps rejected: %v", err)
	}
	long.Tracks[0].Clips[0].Start = 2 * 60 * 60
	if err := Validate(long); err == nil {
		t.Fatal("a 61-minute timeline was accepted")
	}

	// Segments bound FFmpeg inputs now; a project may use more than 32 assets.
	many := validProject()
	many.Assets, many.Tracks[0].Clips = nil, nil
	for i := 0; i < 33; i++ {
		id := fmt.Sprintf("video-%d", i)
		many.Assets = append(many.Assets, Asset{ID: id, Name: id + ".mp4", Kind: AssetVideo, DurationFrames: 30, Width: 64, Height: 36})
		many.Tracks[0].Clips = append(many.Tracks[0].Clips, Clip{ID: "clip-" + id, AssetID: id, Start: i * 30, Duration: 30, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain})
	}
	if err := Validate(many); err != nil {
		t.Fatalf("33 distinct assets rejected: %v", err)
	}
}

func TestUpgradeVersionOne(t *testing.T) {
	v1 := validProject()
	v1.Version = 1
	if err := Validate(v1); err == nil {
		t.Fatal("Validate accepted an unconverted version 1 project")
	}
	got, err := Upgrade(v1)
	if err != nil {
		t.Fatalf("Upgrade() error = %v", err)
	}
	if got.Version != CurrentVersion || got.Assets[0].DurationUs != 10_000_000 {
		t.Fatalf("Upgrade() = version %d, duration %d us", got.Version, got.Assets[0].DurationUs)
	}
	if v1.Assets[0].DurationUs != 0 {
		t.Fatal("Upgrade() mutated its input")
	}
	if err := Validate(got); err != nil {
		t.Fatalf("upgraded project invalid: %v", err)
	}
	again, err := Upgrade(got)
	if err != nil || again.Version != CurrentVersion {
		t.Fatalf("Upgrade(current) = %d, %v", again.Version, err)
	}
	notThirty := v1
	notThirty.FPS = 25
	if _, err := Upgrade(notThirty); err == nil {
		t.Fatal("a version 1 project at 25 fps was upgraded")
	}
	if _, err := Upgrade(Project{Version: CurrentVersion + 1}); err == nil {
		t.Fatal("a version newer than CurrentVersion was upgraded")
	}
}

func TestAssetAtRate(t *testing.T) {
	video := Asset{Kind: AssetVideo, DurationUs: 2_000_000, DurationFrames: 60}
	if got := AssetAtRate(video, Rate{60, 1}).DurationFrames; got != 120 {
		t.Fatalf("video at 60 fps = %d frames", got)
	}
	image := Asset{Kind: AssetImage, Width: 10, Height: 10}
	if got := AssetAtRate(image, Rate{60, 1}).DurationFrames; got != 0 {
		t.Fatalf("image at 60 fps = %d frames", got)
	}
	legacy := Asset{Kind: AssetAudio, DurationFrames: 90, HasAudio: true}
	if got := AssetAtRate(legacy, Rate{60, 1}).DurationFrames; got != 90 {
		t.Fatalf("asset without microseconds changed to %d frames", got)
	}
}
