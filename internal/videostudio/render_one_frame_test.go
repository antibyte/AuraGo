package videostudio

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// oneFramePieceProject places index clips (source frame N is grey N) so that windows cut one-frame pieces in front of
// concat inputs:
//   - video: a (0–50, source 100…) dissolves over 10 frames into b (40–60, source 10…), a hard cut to c (60–70,
//     source 150…), a one-frame gap at 70, d (71–80, source 200…) with a one-frame dissolve into e (79–90, source 50…);
//   - pip: a quarter box in the top-left corner from frame 1 (source 30…), so the track opens with a one-frame gap.
func oneFramePieceProject(t *testing.T, asset Asset) Project {
	t.Helper()
	clip := func(id string, start, offset, duration int) Clip {
		return Clip{ID: id, AssetID: "index", Start: start, Offset: offset, Duration: duration, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain}
	}
	a, b, c, d, e := clip("a", 0, 100, 50), clip("b", 40, 10, 20), clip("c", 60, 150, 10), clip("d", 71, 200, 9), clip("e", 79, 50, 11)
	a.Transition = &Transition{Type: TransitionDissolve, Duration: 10}
	d.Transition = &Transition{Type: TransitionDissolve, Duration: 1}
	pip := clip("pip", 1, 30, 88)
	pip.Width, pip.Height = 0.25, 0.25
	project := Project{Version: CurrentVersion, Name: "One-frame pieces", Width: 720, Height: 720, FPS: 25,
		Assets: []Asset{AssetAtRate(asset, Rate{25, 1})},
		Tracks: []Track{
			{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{a, b, c, d, e}},
			{ID: "pip", Name: "Picture in picture", Kind: TrackVideo, Clips: []Clip{pip}},
		}}
	if err := Validate(project); err != nil {
		t.Fatalf("one-frame piece project invalid: %v", err)
	}
	return project
}

// shownSource is the source frame (its grey level) that track shows at timeline frame f: ok is false where no clip
// is, and source is -1 inside a transition, whose mixed frames only the parity checks compare.
func shownSource(track Track, f int) (source int, ok bool) {
	covering := 0
	for _, clip := range track.Clips {
		if clip.Start <= f && f < clip.Start+clip.Duration {
			covering++
			source = clip.Offset + f - clip.Start
		}
	}
	if covering > 1 {
		return -1, true
	}
	return source, covering == 1
}

// One-frame pieces in front of a concat input (a transition head, a clip piece, a gap, a one-frame dissolve) keep
// every following frame in place: each timeline frame shows its own source frame in the single pass, and windows that
// cut such pieces, and 7- and 13-frame export segmentations, render the same frames.
func TestSegmentedRenderOneFramePieces(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	root := t.TempDir()
	paths := map[string]string{"index": filepath.Join(root, "index.mkv")}
	project := oneFramePieceProject(t, makeIndexFixture(t, ctx, ffmpeg, paths["index"]))
	video, pip := project.Tracks[0], project.Tracks[1]
	const frames = 90
	at := func(x, y int) int { return (y*project.Width + x) * 3 }
	single := renderRawRGB(t, ctx, ffmpeg, project, paths, Segment{Start: 0, End: frames})
	var wrong []string
	for f, frame := range single {
		want, _ := shownSource(video, f)
		if want >= 0 && int(frame[at(360, 360)]) != want {
			wrong = append(wrong, fmt.Sprintf("%d: video shows %d, want %d", f, frame[at(360, 360)], want))
		}
		if source, ok := shownSource(pip, f); ok {
			want = source
		}
		if want >= 0 && int(frame[at(90, 90)]) != want {
			wrong = append(wrong, fmt.Sprintf("%d: pip corner shows %d, want %d", f, frame[at(90, 90)], want))
		}
	}
	if len(wrong) > 0 {
		t.Fatalf("single pass shows the wrong source frames:\n%s", strings.Join(wrong, "\n"))
	}
	for _, window := range []Segment{
		{Start: 39, End: 52}, // one frame of a before the dissolve into b at 40
		{Start: 59, End: 66}, // one frame of b before the hard cut to c at 60
		{Start: 69, End: 76}, // one frame of c, then the one-frame gap at 70
		{Start: 70, End: 77}, // the one-frame gap opens the window
		{Start: 78, End: 85}, // one frame of d before the one-frame dissolve at 79
		{Start: 79, End: 85}, // the one-frame dissolve opens the window
	} {
		part := renderRawRGB(t, ctx, ffmpeg, project, paths, window)
		var differing []int
		for i, frame := range part {
			if !bytes.Equal(frame, single[window.Start+i]) {
				differing = append(differing, window.Start+i)
			}
		}
		if len(differing) > 0 {
			t.Errorf("window %d–%d: frames %v differ from the single pass", window.Start, window.End, differing)
		}
	}
	render := func(name string, segmentFrames int) []string {
		output := filepath.Join(root, name+".mp4")
		if err := RenderWithOptions(ctx, RenderOptions{FFmpegPath: ffmpeg, Project: project, AssetPaths: paths, Output: output,
			SegmentFrames: segmentFrames, Encoder: Encoder{Lossless: true}}); err != nil {
			t.Fatalf("render %s: %v", name, err)
		}
		return frameHashes(t, ctx, ffmpeg, output)
	}
	exported := render("single", 100000)
	if len(exported) != frames {
		t.Fatalf("export has %d frames, want %d", len(exported), frames)
	}
	// 13-frame segments start at 39 and 78, 7-frame segments at 50, 57, 64, 71 and 78 (cuts skip the dissolve).
	for _, size := range []int{7, 13} {
		if got := render(fmt.Sprintf("segments-%d", size), size); !reflect.DeepEqual(got, exported) {
			t.Errorf("%d-frame segments differ from the single pass", size)
		}
	}
}

// Only a concat with a one-frame input before its last one is renumbered (setpts=N); every other layer keeps its graph.
func TestVisualLayerRenumbersOnlyAfterOneFramePieces(t *testing.T) {
	clip := func(id string, start, duration int) Clip {
		return Clip{ID: id, AssetID: "v", Start: start, Duration: duration, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain}
	}
	dissolve := func(c Clip, frames int) Clip {
		c.Transition = &Transition{Type: TransitionDissolve, Duration: frames}
		return c
	}
	for _, c := range []struct {
		name       string
		clips      []Clip
		window     Segment
		renumbered int
	}{
		{"hard cut", []Clip{clip("x", 0, 20), clip("y", 20, 20)}, Segment{0, 40}, 0},
		{"two-frame piece before a cut", []Clip{clip("x", 0, 20), clip("y", 20, 20)}, Segment{18, 30}, 0},
		{"one-frame piece before a cut", []Clip{clip("x", 0, 20), clip("y", 20, 20)}, Segment{19, 30}, 1},
		{"two-frame gap at the start", []Clip{clip("x", 2, 20)}, Segment{0, 22}, 0},
		{"one-frame gap at the start", []Clip{clip("x", 1, 20)}, Segment{0, 21}, 1},
		{"one-frame gap between clips", []Clip{clip("x", 0, 20), clip("y", 21, 20)}, Segment{0, 41}, 0},
		{"one-frame piece, then a gap", []Clip{clip("x", 0, 20), clip("y", 22, 20)}, Segment{19, 30}, 1},
		{"dissolve", []Clip{dissolve(clip("x", 0, 20), 5), clip("y", 15, 20)}, Segment{0, 35}, 0},
		{"one-frame dissolve head", []Clip{dissolve(clip("x", 0, 20), 5), clip("y", 15, 20)}, Segment{14, 30}, 1},
		{"one-frame dissolve", []Clip{dissolve(clip("x", 0, 20), 1), clip("y", 19, 20)}, Segment{0, 39}, 1},
		{"one-frame dissolve at the window end", []Clip{dissolve(clip("x", 0, 20), 1), clip("y", 19, 20)}, Segment{0, 20}, 0},
	} {
		project := Project{Version: CurrentVersion, Name: "Layer", Width: 1280, Height: 720, FPS: 25,
			Assets: []Asset{{ID: "v", Name: "v.mkv", Kind: AssetVideo, DurationFrames: 250, Width: 32, Height: 32}},
			Tracks: []Track{{ID: "t", Name: "Video", Kind: TrackVideo, Clips: c.clips}}}
		if err := Validate(project); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		sources := map[string]renderSource{"v": {asset: project.Assets[0], path: "v.mkv", videoEnd: 10}}
		_, graph, _, err := buildVideoSegment(project, c.window, sources, ProjectRate(project), 1280, 720, "")
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Count(graph, ",settb=1/25,setpts=N["); got != c.renumbered {
			t.Errorf("%s: %d renumbered concats, want %d:\n%s", c.name, got, c.renumbered, graph)
		}
	}
}
