package videostudio

import (
	"slices"
	"testing"
)

// TestBlendRuns pins the effective mode of every window frame (index §4.22) on layers that segmentLayers builds:
//   - clip runs;
//   - shared and mixed transitions;
//   - gaps that join their neighbours;
//   - a disabled next clip, whose transition is dropped;
//   - one-frame windows.
func TestBlendRuns(t *testing.T) {
	dissolve := &Transition{Type: TransitionDissolve, Duration: 4}
	clip := blendGraphClip
	disabled := clip("b", 10, 14, "screen", nil)
	disabled.Disabled = true
	for _, c := range []struct {
		name   string
		clips  []Clip
		window Segment
		want   []blendRun
	}{
		{"one clip", []Clip{clip("a", 0, 30, "multiply", nil)}, Segment{0, 30}, []blendRun{{"multiply", 0, 30}}},
		{"clip runs", []Clip{clip("a", 0, 10, "", nil), clip("b", 10, 10, "multiply", nil), clip("c", 20, 10, "screen", nil)}, Segment{0, 30},
			[]blendRun{{BlendNormal, 0, 10}, {"multiply", 10, 20}, {"screen", 20, 30}}},
		{"gaps join neighbours", []Clip{clip("a", 4, 6, "multiply", nil), clip("b", 16, 6, "screen", nil)}, Segment{0, 30},
			[]blendRun{{"multiply", 0, 16}, {"screen", 16, 30}}},
		{"shared transition", []Clip{clip("a", 0, 14, "multiply", dissolve), clip("b", 10, 14, "multiply", nil), clip("c", 24, 6, "", nil)}, Segment{0, 30},
			[]blendRun{{"multiply", 0, 24}, {BlendNormal, 24, 30}}},
		{"mixed transition", []Clip{clip("a", 0, 14, "multiply", dissolve), clip("b", 10, 14, "screen", nil)}, Segment{0, 30},
			[]blendRun{{"multiply", 0, 10}, {BlendNormal, 10, 14}, {"screen", 14, 30}}},
		{"normal into a mode", []Clip{clip("a", 0, 14, "", dissolve), clip("b", 10, 14, "screen", nil)}, Segment{0, 30},
			[]blendRun{{BlendNormal, 0, 14}, {"screen", 14, 30}}},
		{"disabled next clip", []Clip{clip("a", 0, 14, "multiply", dissolve), disabled, clip("c", 24, 6, "screen", nil)}, Segment{0, 30},
			[]blendRun{{"multiply", 0, 24}, {"screen", 24, 30}}},
		{"one-frame window inside a clip", []Clip{clip("a", 0, 30, "multiply", nil)}, Segment{5, 6}, []blendRun{{"multiply", 0, 1}}},
		{"window starting in a gap", []Clip{clip("a", 0, 10, "multiply", nil), clip("b", 14, 16, "screen", nil)}, Segment{11, 20}, []blendRun{{"screen", 0, 9}}},
		{"window ending in a gap", []Clip{clip("a", 0, 10, "multiply", nil), clip("b", 20, 10, "screen", nil)}, Segment{5, 15}, []blendRun{{"multiply", 0, 10}}},
	} {
		project, sources := blendGraphProject(c.clips...)
		layers, _, err := segmentLayers(project, c.window, sources, ProjectRate(project))
		if err != nil || len(layers) != 2 {
			t.Fatalf("%s: %d layers, %v", c.name, len(layers), err)
		}
		if got := blendRuns(layers[1], c.window.End-c.window.Start); !slices.Equal(got, c.want) {
			t.Errorf("%s: runs %+v, want %+v", c.name, got, c.want)
		}
	}

	// A one-frame window inside a transition composes the whole overlap (expandRange), which is normal for mixed modes.
	project, sources := blendGraphProject(clip("a", 0, 14, "multiply", dissolve), clip("b", 10, 14, "screen", nil))
	window := expandRange(Segment{11, 12}, videoForbiddenSpans(project))
	layers, _, err := segmentLayers(project, window, sources, ProjectRate(project))
	if err != nil || window != (Segment{10, 14}) || len(layers) != 2 {
		t.Fatalf("window %v, %d layers, %v", window, len(layers), err)
	}
	if got := blendRuns(layers[1], window.End-window.Start); !slices.Equal(got, []blendRun{{BlendNormal, 0, 4}}) {
		t.Fatalf("one-frame window in a mixed transition: %+v", got)
	}
	// A layer without frames composites normal (the identity).
	if got := blendRuns(nil, 5); !slices.Equal(got, []blendRun{{BlendNormal, 0, 5}}) {
		t.Fatalf("empty layer: %+v", got)
	}
	if clipBlendMode(Clip{}) != BlendNormal || clipBlendMode(Clip{Blend: "multiply"}) != "multiply" || clipBlendMode(Clip{Blend: "bogus"}) != BlendNormal {
		t.Fatal("clipBlendMode: an absent mode is normal, a registry mode is kept, an unknown one falls back to normal")
	}
}
