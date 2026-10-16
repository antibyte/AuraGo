package videostudio

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestVideoSegmentGraphSeeksAndPreRolls(t *testing.T) {
	clip := func(id string, start, offset, duration int, transition *Transition) Clip {
		return Clip{ID: id, AssetID: "v", Start: start, Offset: offset, Duration: duration, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, Transition: transition}
	}
	project := Project{Version: CurrentVersion, Name: "Graph", Width: 1280, Height: 720, FPS: 30, NTSC: true,
		Assets: []Asset{{ID: "v", Name: "v.mp4", Kind: AssetVideo, DurationFrames: 900, Width: 64, Height: 36}},
		Tracks: []Track{{ID: "t", Name: "Video", Kind: TrackVideo, Clips: []Clip{
			clip("a", 0, 100, 60, &Transition{Type: TransitionDissolve, Duration: 10}),
			clip("b", 50, 0, 60, nil),
		}}}}
	rate := ProjectRate(project)
	sources := map[string]renderSource{"v": {asset: project.Assets[0], path: "v.mp4", videoEnd: 30}}

	inputs, graph, commands, err := buildVideoSegment(project, Segment{55, 80}, sources, rate, 1280, 720, "")
	if commands != nil {
		t.Fatalf("a project without keyframes needs no command files: %+v", commands)
	}
	if err != nil {
		t.Fatal(err)
	}
	// 55 lies inside the dissolve (50–60): the window starts at 50 and the composed frames are trimmed by 5.
	for _, want := range []string{"trim=start_frame=5:end_frame=30", "fps=30000/1001", "settb=1001/30000", "xfade=transition=fade:duration=0.333667:offset=0.000000"} {
		if !strings.Contains(graph, want) {
			t.Fatalf("graph lacks %q:\n%s", want, graph)
		}
	}
	// a enters the window 50 frames late at source frame 150: its input seeks one second (30 frames) earlier and
	// keeps absolute timestamps, so the graph picks frame 150 on the source's own cadence.
	if len(inputs) != 2 || math.Abs(inputs[0].seek-rate.Seconds(120)) > 1e-9 || inputs[0].limit <= rate.Seconds(40) {
		t.Fatalf("inputs = %+v", inputs)
	}
	if !strings.Contains(graph, "[0:V:0]setpts='if(eq(N,0),min(PTS,floor(5.005000000/TB)),PTS)',fps=30000/1001,setpts='max(PTS-150,-10)'") {
		t.Fatalf("graph does not pick source frame 150 by absolute position:\n%s", graph)
	}
	// The graph converts to BT.709 itself instead of leaving RGBA to the encoder's scaler (BT.601 before FFmpeg 7.1).
	if !strings.HasSuffix(graph, ",setpts=PTS-STARTPTS,scale=out_color_matrix=bt709:out_range=tv,format=yuv420p[vs_video_out]") {
		t.Fatalf("graph does not end in an explicit BT.709 conversion:\n%s", graph)
	}

	// A position past the decodable video seeks to one second before its end and shifts the rest in the graph.
	sources["v"] = renderSource{asset: project.Assets[0], path: "v.mp4", videoEnd: 2}
	inputs, graph, _, err = buildVideoSegment(project, Segment{0, 20}, sources, rate, 640, 360, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 1 || inputs[0].seek > 1.0001 || !strings.Contains(graph, "setpts='max(PTS-100,-20)'") ||
		!strings.HasSuffix(graph, ",scale=w=640:h=360:flags=bicubic:out_color_matrix=bt709:out_range=tv,format=yuv420p[vs_video_out]") {
		t.Fatalf("late clip: inputs %+v graph:\n%s", inputs, graph)
	}
}

func TestAudioSegmentGraphFades(t *testing.T) {
	project := Project{Version: CurrentVersion, Name: "Audio", Width: 1280, Height: 720, FPS: 30,
		Assets: []Asset{{ID: "tone", Name: "tone.wav", Kind: AssetAudio, DurationFrames: 900, HasAudio: true}},
		Tracks: []Track{{ID: "music", Name: "Music", Kind: TrackAudio, Clips: []Clip{
			{ID: "m", AssetID: "tone", Start: 10, Offset: 30, Duration: 100, Volume: 0.5, FadeIn: 20, FadeOut: 30},
		}}}}
	rate := ProjectRate(project)
	sources := map[string]renderSource{"tone": {asset: project.Assets[0], path: "tone.wav"}}

	inputs, graph, err := buildAudioSegment(project, Segment{40, 70}, sources, rate)
	if err != nil {
		t.Fatal(err)
	}
	// Entered after its fade-in and still playing after the segment: no fades, seek to source frame 60.
	if len(inputs) != 1 || strings.Contains(graph, "afade") || math.Abs(inputs[0].seek-2.0) > 1e-9 || !strings.Contains(graph, "atrim=start_sample=0:end_sample=48000") {
		t.Fatalf("middle segment: inputs %+v graph:\n%s", inputs, graph)
	}
	_, graph, _ = buildAudioSegment(project, Segment{0, 40}, sources, rate)
	if !strings.Contains(graph, "afade=t=in:st=0:d=0.666667") || strings.Contains(graph, "afade=t=out") || !strings.Contains(graph, "adelay=16000S") {
		t.Fatalf("first segment graph:\n%s", graph)
	}
	// A boundary inside the fade-out widens the window to the whole fade and trims it back.
	_, graph, _ = buildAudioSegment(project, Segment{70, 95}, sources, rate)
	if !strings.Contains(graph, "afade=t=out:st=0.333333:d=1.000000") || !strings.Contains(graph, "atrim=start_sample=0:end_sample=40000") {
		t.Fatalf("fade-out segment graph:\n%s", graph)
	}
	empty := project
	empty.Tracks = []Track{{ID: "music", Name: "Music", Kind: TrackAudio}}
	if inputs, graph, _ := buildAudioSegment(empty, Segment{0, 30}, sources, rate); len(inputs) != 0 || !strings.Contains(graph, "anullsrc") {
		t.Fatalf("silent segment: %d inputs, graph:\n%s", len(inputs), graph)
	}
}

func TestSegmentLayersCarryWholeClipAndRamp(t *testing.T) {
	ramp := []RampPoint{{F: 0, Speed: 1, E: EaseLinear}, {F: 20, Speed: 2, E: EaseLinear}}
	project := Project{Version: CurrentVersion, Name: "Layers", Width: 1280, Height: 720, FPS: 25,
		Assets: []Asset{{ID: "v", Name: "v.mp4", Kind: AssetVideo, DurationFrames: 900, Width: 64, Height: 36}},
		Tracks: []Track{{ID: "t", Name: "Video", Kind: TrackVideo, Clips: []Clip{
			{ID: "a", AssetID: "v", Start: 0, Duration: 40, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain},
			{ID: "b", AssetID: "v", Start: 40, Offset: 5, Duration: 60, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, Ramp: ramp},
		}}}}
	sources := map[string]renderSource{"v": {asset: project.Assets[0], path: "v.mp4", videoEnd: 36}}
	layers, _, err := segmentLayers(project, Segment{Start: 30, End: 70}, sources, ProjectRate(project))
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != 1 || len(layers[0]) != 2 {
		t.Fatalf("layers = %+v", layers)
	}
	a, b := layers[0][0], layers[0][1]
	// Window frame k of a clip is absolute frame at+k: a entered the window at 30, b starts inside it at 40.
	if a.at != 30 || a.whole.ID != "a" || a.whole.Start != 0 || a.whole.Duration != 40 || a.clip.Duration != 10 || a.pieces != nil {
		t.Fatalf("a = whole %+v, at %d, local %d frames, pieces %v", a.whole, a.at, a.clip.Duration, a.pieces)
	}
	if b.at != 40 || b.whole.ID != "b" || b.whole.Duration != 60 || !reflect.DeepEqual(b.pieces, rampPieces(ramp, 0, 60)) {
		t.Fatalf("b = whole %+v, at %d, pieces %v", b.whole, b.at, b.pieces)
	}
}
