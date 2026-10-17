package videostudio

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
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

	inputs, graph, files, err := buildVideoSegment(project, Segment{55, 80}, sources, rate, 1280, 720, "")
	if files.commands != nil || files.luts != nil {
		t.Fatalf("a project without keyframes or LUTs needs no graph files: %+v", files)
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

// A render loads the LUT assets that on lut effects of enabled clips on visible visual tracks reference
// (LoadLUTFile). Each layer clip then carries its tables by stack index (built-in looks included) and its decoded
// picture size.
func TestCollectRenderSourcesLoadsLUTs(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	paths := map[string]string{
		"img":    write("logo.png", "png"),
		"used":   write("used.cube", lutTestCube(2, lutIdentity, "\n")),
		"off":    write("off.cube", "not a cube"),
		"hidden": write("hidden.cube", "not a cube"),
	}
	lut := func(on bool, ref string) EffectInstance {
		return EffectInstance{ID: "lut", On: on, Params: map[string]any{"intensity": 1.0, "lut": ref}}
	}
	clip := func(id string, effects ...EffectInstance) Clip {
		return Clip{ID: id, AssetID: "img", Duration: 25, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, Effects: effects}
	}
	project := Project{Version: CurrentVersion, Name: "LUTs", Width: 1280, Height: 720, FPS: 25,
		Assets: []Asset{
			{ID: "img", Name: "logo.png", Kind: AssetImage, Width: 64, Height: 32},
			{ID: "used", Name: "used.cube", Kind: AssetLUT, LUTSize: 2},
			{ID: "off", Name: "off.cube", Kind: AssetLUT, LUTSize: 2},
			{ID: "hidden", Name: "hidden.cube", Kind: AssetLUT, LUTSize: 2},
		},
		Tracks: []Track{
			{ID: "v", Name: "Video", Kind: TrackVideo, Clips: []Clip{clip("c", lut(true, "used"), lut(false, "off"), lut(true, "builtin:warm"))}},
			{ID: "h", Name: "Hidden", Kind: TrackOverlay, Hidden: true, Clips: []Clip{clip("o", lut(true, "hidden"))}},
		}}
	sources, err := collectRenderSources(context.Background(), "ffmpeg-not-needed", project, paths)
	if err != nil {
		t.Fatal(err)
	}
	if source := sources["used"]; source.lut == nil || source.lut.Size != 2 {
		t.Fatalf("used LUT source = %+v", source)
	}
	if _, ok := sources["off"]; ok {
		t.Fatal("the LUT of an off entry was loaded")
	}
	if _, ok := sources["hidden"]; ok {
		t.Fatal("the LUT of a hidden track was loaded")
	}
	if source := sources["img"]; source.decodedWidth != 64 || source.decodedHeight != 32 {
		t.Fatalf("image decoded size %dx%d", source.decodedWidth, source.decodedHeight)
	}
	layers, _, err := segmentLayers(project, Segment{Start: 0, End: 25}, sources, ProjectRate(project))
	if err != nil || len(layers) != 1 || len(layers[0]) != 1 {
		t.Fatalf("layers %+v, %v", layers, err)
	}
	item := layers[0][0]
	warm, _ := BuiltinLook("warm")
	if len(item.luts) != 2 || item.luts[0] != sources["used"].lut || item.luts[2] != warm || item.pictureWidth != 64 || item.pictureHeight != 32 {
		t.Fatalf("layer clip luts %v, picture %dx%d", item.luts, item.pictureWidth, item.pictureHeight)
	}
	// A LUT that does not parse fails the render before FFmpeg runs, and so does a table that was not staged.
	broken := map[string]string{"img": paths["img"], "used": paths["off"]}
	if _, err := collectRenderSources(context.Background(), "ffmpeg-not-needed", project, broken); err == nil || !strings.Contains(err.Error(), `"used"`) {
		t.Fatalf("a broken LUT asset: %v", err)
	}
	delete(sources, "used")
	if _, _, err := segmentLayers(project, Segment{Start: 0, End: 25}, sources, ProjectRate(project)); err == nil {
		t.Fatal("a missing LUT source was accepted")
	}
}

// blendGraphProject is a 1280x720, 25 fps project with a full-canvas base clip on the video track (layer 0) and the given
// clips on an overlay track (layer 1). Its one video asset is never decoded: these tests only build graphs.
func blendGraphProject(top ...Clip) (Project, map[string]renderSource) {
	project := Project{Version: CurrentVersion, Name: "Blend graph", Width: 1280, Height: 720, FPS: 25,
		Assets: []Asset{{ID: "v", Name: "v.mp4", Kind: AssetVideo, DurationFrames: 900, Width: 64, Height: 36}},
		Tracks: []Track{
			{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{{ID: "base", AssetID: "v", Duration: 30, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain}}},
			{ID: "top", Name: "Top", Kind: TrackOverlay, Clips: top},
		}}
	return project, map[string]renderSource{"v": {asset: project.Assets[0], path: "v.mp4", videoEnd: 36}}
}

// blendGraphClip is a clip of blendGraphProject's overlay track in a box of half the canvas.
func blendGraphClip(id string, start, duration int, blend string, transition *Transition) Clip {
	return Clip{ID: id, AssetID: "v", Start: start, Duration: duration, X: 0.1, Y: 0.1, Width: 0.5, Height: 0.5, Opacity: 1, Volume: 1,
		Fit: FitContain, Blend: blend, Transition: transition}
}

// twoCGraph is buildVideoSegment's composition at plan 2c (7e93ec73c), rebuilt from its unchanged parts:
//   - the black base canvas;
//   - buildVisualLayer per layer;
//   - one overlay per layer;
//   - the BT.709 tail at the project size.
//
// A layer whose frames are all normal must still produce exactly this (index §4.10, risk 18).
func twoCGraph(t *testing.T, project Project, seg Segment, sources map[string]renderSource) string {
	t.Helper()
	rate := ProjectRate(project)
	window := expandRange(seg, videoForbiddenSpans(project))
	layers, _, err := segmentLayers(project, window, sources, rate)
	if err != nil {
		t.Fatal(err)
	}
	width, height, frames := project.Width, project.Height, window.End-window.Start
	filters := []string{fmt.Sprintf("color=c=black:s=%dx%d:r=%s:d=%s,trim=end_frame=%d,setpts=PTS-STARTPTS,format=rgba[vs_base0]", width, height, rate, rate.SecondsArg(frames), frames)}
	output, names := "vs_base0", &lutNames{}
	for index, layer := range layers {
		label, layerFilters, _ := buildVisualLayer(layer, index, width, height, rate, names)
		next := fmt.Sprintf("vs_base%d", index+1)
		filters = append(filters, layerFilters...)
		filters = append(filters, fmt.Sprintf("[%s][%s]overlay=eof_action=pass:repeatlast=0:shortest=0:format=auto[%s]", output, label, next))
		output = next
	}
	filters = append(filters, fmt.Sprintf("[%s]trim=start_frame=%d:end_frame=%d,setpts=PTS-STARTPTS,scale=out_color_matrix=bt709:out_range=tv,format=yuv420p[vs_video_out]",
		output, seg.Start-window.Start, seg.End-window.Start))
	return strings.Join(filters, ";\n")
}

// TestSegmentGraphBlendSnapshots pins how track layers composite (index §4.10, §4.22):
//   - All-normal layers are the 2c graph byte for byte, also next to a blended layer.
//   - A layer with one mode composites without a split. It is padded with transparent frames when it ends before the
//     window. Dodge takes the layer first.
//   - A mixed layer splits into runs, trims them and concatenates one window-long sublayer per mode. Normal composites
//     first, then the modes in registry order.
func TestSegmentGraphBlendSnapshots(t *testing.T) {
	dissolve := &Transition{Type: TransitionDissolve, Duration: 4}
	graphOf := func(project Project, sources map[string]renderSource, seg Segment) string {
		t.Helper()
		_, graph, _, err := buildVideoSegment(project, seg, sources, ProjectRate(project), project.Width, project.Height, "")
		if err != nil {
			t.Fatal(err)
		}
		return graph
	}

	// All normal: a transition, a gap, a pre-rolled window inside the transition and a window starting in a gap.
	normal, sources := blendGraphProject(blendGraphClip("a", 0, 10, "", dissolve), blendGraphClip("b", 6, 10, "", nil), blendGraphClip("c", 20, 6, "", nil))
	for _, seg := range []Segment{{0, 30}, {8, 9}, {18, 24}} {
		if got, want := graphOf(normal, sources, seg), twoCGraph(t, normal, seg, sources); got != want {
			t.Fatalf("all-normal graph of %v changed:\n got %s\nwant %s", seg, got, want)
		}
	}

	// A normal layer under a blended one keeps its lines. Of the blended layer only the overlay line is replaced.
	blended, sources := blendGraphProject(blendGraphClip("a", 0, 10, "multiply", dissolve), blendGraphClip("b", 6, 10, "multiply", nil), blendGraphClip("c", 20, 6, "screen", nil))
	got := graphOf(blended, sources, Segment{0, 30})
	reference := twoCGraph(t, normal, Segment{0, 30}, sources)
	if cut := strings.Index(reference, "[vs_base1]") + len("[vs_base1]"); !strings.HasPrefix(got, reference[:cut]) {
		t.Fatalf("the normal base layer changed:\n got %s\nwant prefix %s", got, reference[:cut])
	}
	for _, line := range strings.Split(reference, ";\n") {
		if strings.HasSuffix(line, "[vs_base2]") && strings.Contains(line, "overlay=") {
			if strings.Contains(got, line) {
				t.Fatalf("the blended layer kept its normal overlay: %s", line)
			}
			continue
		}
		if !strings.Contains(got, line) {
			t.Fatalf("blended graph lacks the 2c line %s:\n%s", line, got)
		}
	}

	// One mode over the whole window: no split, no pad; dodge takes the layer first.
	single, sources := blendGraphProject(blendGraphClip("d", 0, 30, "dodge", nil))
	got = graphOf(single, sources, Segment{0, 30})
	wantSingle := strings.Join([]string{
		"[vs_base1]format=gbrp,split=2[vs_bl_1_1_b0][vs_bl_1_1_b1]",
		"[vs_clip_1_0]split=2[vs_bl_1_1_l0][vs_bl_1_1_l1]",
		"[vs_bl_1_1_l0]format=gbrp[vs_bl_1_1_lg]",
		"[vs_bl_1_1_l1]alphaextract[vs_bl_1_1_la]",
		"[vs_bl_1_1_lg][vs_bl_1_1_b0]blend=c0_mode=dodge:c1_mode=dodge:c2_mode=dodge[vs_bl_1_1_e]",
		"[vs_bl_1_1_b1][vs_bl_1_1_e][vs_bl_1_1_la]maskedmerge=planes=7,format=rgba[vs_base2]",
	}, ";\n")
	if !strings.Contains(got, wantSingle) || strings.Contains(got, "vs_bl_1_0_") || strings.Contains(got, "_run]") || strings.Contains(got, "all_opacity") || strings.Contains(got, "sendcmd") {
		t.Fatalf("single-mode layer:\n%s", got)
	}

	// One mode, but the layer ends at 20 of 30 frames: transparent frames pad it to the window before the composite.
	early, sources := blendGraphProject(blendGraphClip("m", 0, 20, "multiply", nil))
	got = graphOf(early, sources, Segment{0, 30})
	wantEarly := strings.Join([]string{
		"color=c=black@0:s=1280x720:r=25:d=0.400000,trim=end_frame=10,setpts=PTS-STARTPTS,format=rgba[vs_bl_1_0_tail]",
		"[vs_clip_1_0][vs_bl_1_0_tail]concat=n=2:v=1:a=0,settb=1/25[vs_bl_1_0_window]",
		"[vs_base1]format=gbrp,split=2[vs_bl_1_1_b0][vs_bl_1_1_b1]",
		"[vs_bl_1_0_window]split=2[vs_bl_1_1_l0][vs_bl_1_1_l1]",
	}, ";\n")
	if !strings.Contains(got, wantEarly) || strings.Contains(got, "_run]") {
		t.Fatalf("padded single-mode layer:\n%s", got)
	}

	// Multiply → gap → screen: the gap joins multiply and the tail joins screen. split, one trim per run, one window-long
	// sublayer per mode with transparent stretches, multiply's composite feeding screen's.
	mixed, sources := blendGraphProject(blendGraphClip("m1", 0, 10, "multiply", nil), blendGraphClip("m2", 14, 10, "screen", nil))
	got = graphOf(mixed, sources, Segment{0, 30})
	wantMixed := strings.Join([]string{
		"color=c=black@0:s=1280x720:r=25:d=0.240000,trim=end_frame=6,setpts=PTS-STARTPTS,format=rgba[vs_bl_1_0_tail]",
		"[vs_join_1_1][vs_bl_1_0_tail]concat=n=2:v=1:a=0,settb=1/25[vs_bl_1_0_window]",
		"[vs_bl_1_0_window]split=2[vs_bl_1_1_in][vs_bl_1_2_in]",
		"[vs_bl_1_1_in]trim=start_frame=0:end_frame=14,setpts=PTS-STARTPTS[vs_bl_1_1_run]",
		"[vs_bl_1_2_in]trim=start_frame=14:end_frame=30,setpts=PTS-STARTPTS[vs_bl_1_2_run]",
		"color=c=black@0:s=1280x720:r=25:d=0.640000,trim=end_frame=16,setpts=PTS-STARTPTS,format=rgba[vs_bl_1_2_gap1]",
		"[vs_bl_1_1_run][vs_bl_1_2_gap1]concat=n=2:v=1:a=0,settb=1/25[vs_bl_1_1_sub]",
		"color=c=black@0:s=1280x720:r=25:d=0.560000,trim=end_frame=14,setpts=PTS-STARTPTS,format=rgba[vs_bl_1_1_gap2]",
		"[vs_bl_1_1_gap2][vs_bl_1_2_run]concat=n=2:v=1:a=0,settb=1/25[vs_bl_1_2_sub]",
		"[vs_base1]format=gbrp,split=2[vs_bl_1_1_b0][vs_bl_1_1_b1]",
		"[vs_bl_1_1_sub]split=2[vs_bl_1_1_l0][vs_bl_1_1_l1]",
		"[vs_bl_1_1_l0]format=gbrp[vs_bl_1_1_lg]",
		"[vs_bl_1_1_l1]alphaextract[vs_bl_1_1_la]",
		"[vs_bl_1_1_b0][vs_bl_1_1_lg]blend=c0_mode=multiply:c1_mode=multiply:c2_mode=multiply[vs_bl_1_1_e]",
		"[vs_bl_1_1_b1][vs_bl_1_1_e][vs_bl_1_1_la]maskedmerge=planes=7,format=rgba[vs_bl_1_1_out]",
		"[vs_bl_1_1_out]format=gbrp,split=2[vs_bl_1_2_b0][vs_bl_1_2_b1]",
		"[vs_bl_1_2_sub]split=2[vs_bl_1_2_l0][vs_bl_1_2_l1]",
		"[vs_bl_1_2_l0]format=gbrp[vs_bl_1_2_lg]",
		"[vs_bl_1_2_l1]alphaextract[vs_bl_1_2_la]",
		"[vs_bl_1_2_b0][vs_bl_1_2_lg]blend=c0_mode=screen:c1_mode=screen:c2_mode=screen[vs_bl_1_2_e]",
		"[vs_bl_1_2_b1][vs_bl_1_2_e][vs_bl_1_2_la]maskedmerge=planes=7,format=rgba[vs_base2]",
		"[vs_base2]trim=start_frame=0:end_frame=30",
	}, ";\n")
	if !strings.Contains(got, wantMixed) {
		t.Fatalf("mixed layer:\n%s", got)
	}

	// A dissolve between different modes is normal in its overlap, and normal composites first with the 2c overlay.
	transition, sources := blendGraphProject(blendGraphClip("m", 0, 14, "multiply", dissolve), blendGraphClip("s", 10, 14, "screen", nil))
	got = graphOf(transition, sources, Segment{0, 30})
	for _, want := range []string{
		"[vs_bl_1_0_window]split=3[vs_bl_1_1_in][vs_bl_1_2_in][vs_bl_1_3_in]",
		"[vs_bl_1_1_gap1][vs_bl_1_2_run][vs_bl_1_3_gap1]concat=n=3:v=1:a=0,settb=1/25[vs_bl_1_1_sub]",
		"[vs_base1][vs_bl_1_1_sub]overlay=eof_action=pass:repeatlast=0:shortest=0:format=auto[vs_bl_1_1_out]",
		"[vs_bl_1_1_out]format=gbrp,split=2[vs_bl_1_2_b0][vs_bl_1_2_b1]",
		"[vs_bl_1_2_b0][vs_bl_1_2_lg]blend=c0_mode=multiply:c1_mode=multiply:c2_mode=multiply[vs_bl_1_2_e]",
		"[vs_bl_1_2_out]format=gbrp,split=2[vs_bl_1_3_b0][vs_bl_1_3_b1]",
		"[vs_bl_1_3_b1][vs_bl_1_3_e][vs_bl_1_3_la]maskedmerge=planes=7,format=rgba[vs_base2]",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("transition layer lacks %s:\n%s", want, got)
		}
	}
}
