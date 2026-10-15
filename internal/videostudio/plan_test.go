package videostudio

import (
	"fmt"
	"reflect"
	"testing"
)

func planTestClip(id string, start, duration int, transition *Transition) Clip {
	return Clip{ID: id, AssetID: "video", Start: start, Duration: duration, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, Transition: transition}
}

func TestPlanVideoSegmentsAvoidTransitionsAndBoundInputs(t *testing.T) {
	project := validProject()
	project.Assets[0].DurationFrames = 3000
	project.Tracks[0].Clips = []Clip{
		planTestClip("a", 0, 60, &Transition{Type: TransitionDissolve, Duration: 10}),
		planTestClip("b", 50, 60, nil),
		planTestClip("c", 110, 60, nil),
	}
	got, err := PlanVideoSegments(project, 0, 170, 55)
	if err != nil {
		t.Fatal(err)
	}
	if want := []Segment{{0, 60}, {60, 115}, {115, 170}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("segments = %v, want %v (55 lies inside the 50–60 dissolve)", got, want)
	}
	// A requested range may start inside a transition; the renderer adds pre-roll for it.
	if got, err := PlanVideoSegments(project, 55, 170, 1000); err != nil || !reflect.DeepEqual(got, []Segment{{55, 170}}) {
		t.Fatalf("range inside a transition = %v, %v", got, err)
	}

	busy := validProject()
	busy.Assets[0].DurationFrames = 3000
	busy.Tracks[0].Clips = nil
	for i := 0; i < 80; i++ {
		busy.Tracks[0].Clips = append(busy.Tracks[0].Clips, planTestClip(fmt.Sprintf("c%d", i), i*2, 2, nil))
	}
	segments, err := PlanVideoSegments(busy, 0, 160, 1000)
	if err != nil {
		t.Fatal(err)
	}
	next := 0
	for _, seg := range segments {
		if seg.Start != next {
			t.Fatalf("segments are not contiguous: %v", segments)
		}
		next = seg.End
		count := 0
		for _, clip := range busy.Tracks[0].Clips {
			if clip.Start < seg.End && clip.Start+clip.Duration > seg.Start {
				count++
			}
		}
		if count > MaxSegmentInputs {
			t.Fatalf("segment %v needs %d inputs", seg, count)
		}
	}
	if next != 160 || len(segments) < 3 {
		t.Fatalf("busy segments = %v", segments)
	}
}

func TestPlanAudioSegmentsAvoidFades(t *testing.T) {
	project := validProject()
	project.Assets = append(project.Assets, Asset{ID: "tone", Name: "tone.wav", Kind: AssetAudio, DurationFrames: 900, HasAudio: true})
	project.Tracks = append(project.Tracks, Track{ID: "music", Name: "Music", Kind: TrackAudio, Clips: []Clip{
		{ID: "m", AssetID: "tone", Start: 0, Duration: 90, Volume: 1, FadeIn: 20, FadeOut: 20},
	}})
	got, err := PlanAudioSegments(project, 0, 90, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := []Segment{{0, 20}, {20, 30}, {30, 40}, {40, 50}, {50, 60}, {60, 70}, {70, 90}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("audio segments = %v, want %v", got, want)
	}
}

func TestExpandRangeCoversNestedSpans(t *testing.T) {
	got := expandRange(Segment{55, 65}, []span{{50, 60}, {58, 70}})
	if got != (Segment{50, 70}) {
		t.Fatalf("expandRange = %v, want {50 70}", got)
	}
	if got := expandRange(Segment{60, 70}, []span{{50, 60}, {70, 80}}); got != (Segment{60, 70}) {
		t.Fatalf("boundaries on span edges were widened: %v", got)
	}
}
