package videostudio

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// lutStackClip is a clip whose effect stack is one lut entry per ref (intensity 1: a lut3d alone).
func lutStackClip(id string, start, duration int, refs ...string) Clip {
	clip := Clip{ID: id, AssetID: "v", Start: start, Duration: duration, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain}
	for _, ref := range refs {
		clip.Effects = append(clip.Effects, EffectInstance{ID: "lut", On: true, Params: map[string]any{"lut": ref, "intensity": 1.0}})
	}
	return clip
}

// repeatRef is n copies of ref.
func repeatRef(ref string, n int) []string {
	refs := make([]string, n)
	for i := range refs {
		refs[i] = ref
	}
	return refs
}

// lutStagingProject has one track per entry of tracks; every track holds the clips of its entry.
func lutStagingProject(tracks ...[]Clip) (Project, map[string]renderSource) {
	project := Project{Version: CurrentVersion, Name: "LUTs", Width: 1280, Height: 720, FPS: 30,
		Assets: []Asset{{ID: "v", Name: "v.mp4", Kind: AssetVideo, DurationFrames: 900, Width: 64, Height: 36}}}
	for i, clips := range tracks {
		project.Tracks = append(project.Tracks, Track{ID: fmt.Sprintf("t%d", i), Name: fmt.Sprintf("Video %d", i), Kind: TrackVideo, Clips: clips})
	}
	return project, map[string]renderSource{"v": {asset: project.Assets[0], path: "v.mp4", videoEnd: 30}}
}

// stagedCubes builds seg and stages its graph; it returns the graph text and the names of the .cube files next to it.
func stagedCubes(t *testing.T, project Project, seg Segment, sources map[string]renderSource) (string, []string) {
	t.Helper()
	rate := ProjectRate(project)
	_, graph, files, err := buildVideoSegment(project, seg, sources, rate, project.Width, project.Height, "")
	if err != nil {
		t.Fatal(err)
	}
	workDir, _, err := stageVideoGraph(t.TempDir(), graph, rate, files)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(workDir)
	matches, err := filepath.Glob(filepath.Join(workDir, "*.cube"))
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(matches))
	for i, match := range matches {
		names[i] = filepath.Base(match)
	}
	return graph, names
}

// TestLUTTablesAreStagedOncePerProcess pins the fix of the staging amplification (index §4.11): every lut3d of one
// FFmpeg process that uses a table reads one file, whatever the number of clips and entries.
func TestLUTTablesAreStagedOncePerProcess(t *testing.T) {
	// Eight tracks, one clip each, eight lut entries of one built-in look: 64 lut3d filters, one file.
	var tracks [][]Clip
	for i := 0; i < 8; i++ {
		tracks = append(tracks, []Clip{lutStackClip(fmt.Sprintf("c%d", i), 0, 20, repeatRef("builtin:warm", 8)...)})
	}
	project, sources := lutStagingProject(tracks...)
	graph, names := stagedCubes(t, project, Segment{Start: 0, End: 20}, sources)
	if len(names) != 1 || names[0] != "vs_lut_0.cube" {
		t.Fatalf("staged %v, want vs_lut_0.cube alone", names)
	}
	if got := strings.Count(graph, "lut3d=file=vs_lut_0.cube:interp=trilinear"); got != MaxSegmentLUTInstances {
		t.Fatalf("%d lut3d filters read vs_lut_0.cube, want %d:\n%s", got, MaxSegmentLUTInstances, graph)
	}
	if strings.Count(graph, "lut3d=") != MaxSegmentLUTInstances {
		t.Fatalf("lut3d filters that read another file:\n%s", graph)
	}

	// Two tables are two files, numbered in the order the graph first uses them; a project LUT asset is one shared table.
	asset := &LUT3D{Size: 2, DomainMax: [3]float64{1, 1, 1}, Data: make([]float32, 2*2*2*3)}
	sources["lut"] = renderSource{asset: Asset{ID: "lut", Name: "grade.cube", Kind: AssetLUT, LUTSize: 2}, lut: asset}
	project.Tracks[0].Clips[0] = lutStackClip("mixed", 0, 20, "lut", "builtin:warm", "lut", "builtin:cool")
	graph, names = stagedCubes(t, project, Segment{Start: 0, End: 20}, sources)
	if len(names) != 3 || names[0] != "vs_lut_0.cube" || names[1] != "vs_lut_1.cube" || names[2] != "vs_lut_2.cube" {
		t.Fatalf("three tables staged as %v", names)
	}
	// Track 0 comes first: the asset (0), warm (1), the asset again (0), cool (2); the other tracks read warm.
	first := func(name string) int { return strings.Index(graph, "lut3d=file="+name) }
	if !(0 <= first("vs_lut_0.cube") && first("vs_lut_0.cube") < first("vs_lut_1.cube") && first("vs_lut_1.cube") < first("vs_lut_2.cube")) {
		t.Fatalf("tables are not numbered in order of first use:\n%s", graph)
	}
	if strings.Count(graph, "lut3d=file=vs_lut_0.cube") != 2 || strings.Count(graph, "lut3d=file=vs_lut_2.cube") != 1 ||
		strings.Count(graph, "lut3d=file=vs_lut_1.cube") != 1+7*8 {
		t.Fatalf("lut3d filters per file:\n%s", graph)
	}
}

// TestLUTInstancesAreBoundedPerProcess pins MaxSegmentLUTInstances: the planner splits a busy span in time, refuses one
// that no cut separates, and the build refuses it too, because RenderFrame builds without the planner.
func TestLUTInstancesAreBoundedPerProcess(t *testing.T) {
	// 32 clips of 8 entries one after another: the planner cuts until a segment holds at most 8 clips (64 instances), and
	// every segment stages exactly one file.
	var clips []Clip
	for i := 0; i < 32; i++ {
		clips = append(clips, lutStackClip(fmt.Sprintf("c%d", i), i*4, 4, repeatRef("lut", 8)...))
	}
	project, sources := lutStagingProject(clips)
	sources["lut"] = renderSource{asset: Asset{ID: "lut", Name: "grade.cube", Kind: AssetLUT, LUTSize: 2},
		lut: &LUT3D{Size: 2, DomainMax: [3]float64{1, 1, 1}, Data: make([]float32, 2*2*2*3)}}
	segments, err := PlanVideoSegments(project, 0, 128, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(segments) < 4 {
		t.Fatalf("256 lut effects in %d segments: %v", len(segments), segments)
	}
	next := 0
	for _, seg := range segments {
		if seg.Start != next {
			t.Fatalf("segments are not contiguous: %v", segments)
		}
		next = seg.End
		if count := countLUTInstances(project, seg); count > MaxSegmentLUTInstances {
			t.Fatalf("segment %v needs %d lut effects", seg, count)
		}
		if _, names := stagedCubes(t, project, seg, sources); len(names) != 1 || names[0] != "vs_lut_0.cube" {
			t.Fatalf("segment %v staged %v", seg, names)
		}
	}
	if next != 128 {
		t.Fatalf("segments end at %d: %v", next, segments)
	}

	// 32 tracks play at once, 8 entries each: no cut helps. The planner and the build both refuse.
	var tall [][]Clip
	for i := 0; i < 32; i++ {
		tall = append(tall, []Clip{lutStackClip(fmt.Sprintf("c%d", i), 0, 30, repeatRef("builtin:warm", 8)...)})
	}
	project, sources = lutStagingProject(tall...)
	if _, err := PlanVideoSegments(project, 0, 30, 1000); !errors.Is(err, ErrTooManyInputs) || !strings.Contains(err.Error(), "lut effects") {
		t.Fatalf("planner: %v", err)
	}
	_, _, _, err = buildVideoSegment(project, Segment{Start: 0, End: 30}, sources, ProjectRate(project), 1280, 720, "")
	if !errors.Is(err, ErrTooManyInputs) || !strings.Contains(err.Error(), "256 lut effects") {
		t.Fatalf("build: %v", err)
	}
	// F4 (plan 2f Task 6): both refusals and the staging cap are ErrTooManyLUTs, which wraps ErrTooManyInputs, so a job
	// failure can name the LUT cap while every errors.Is(err, ErrTooManyInputs) check keeps holding.
	if _, err := PlanVideoSegments(project, 0, 30, 1000); !errors.Is(err, ErrTooManyLUTs) || !errors.Is(err, ErrTooManyInputs) {
		t.Fatalf("planner: %v is not ErrTooManyLUTs", err)
	}
	if !errors.Is(err, ErrTooManyLUTs) {
		t.Fatalf("build: %v is not ErrTooManyLUTs", err)
	}
	table := &LUT3D{Size: 2, DomainMax: [3]float64{1, 1, 1}, Data: make([]float32, 2*2*2*3)}
	if _, _, err := stageVideoGraph(t.TempDir(), "graph", ProjectRate(project), graphFiles{luts: tooManyLUTFiles(table)}); !errors.Is(err, ErrTooManyLUTs) || !errors.Is(err, ErrTooManyInputs) || !strings.Contains(err.Error(), "LUT tables") {
		t.Fatalf("staging: %v", err)
	}
	if !errors.Is(ErrTooManyLUTs, ErrTooManyInputs) || !strings.Contains(ErrTooManyLUTs.Error(), "too many simultaneous LUT effects") {
		t.Fatalf("ErrTooManyLUTs = %v", ErrTooManyLUTs)
	}
	// Too many clips without any lut effect stay a plain ErrTooManyInputs.
	var crowd [][]Clip
	for i := 0; i <= MaxSegmentInputs; i++ {
		crowd = append(crowd, []Clip{lutStackClip(fmt.Sprintf("p%d", i), 0, 30)})
	}
	crowded, _ := lutStagingProject(crowd...)
	if _, err := PlanVideoSegments(crowded, 0, 30, 1000); !errors.Is(err, ErrTooManyInputs) || errors.Is(err, ErrTooManyLUTs) {
		t.Fatalf("clips alone: %v, want ErrTooManyInputs without ErrTooManyLUTs", err)
	}

	// The cap counts on lut entries with a table of enabled clips on visible visual tracks, nothing else.
	project, _ = lutStagingProject(tall[:8]...)
	extra := lutStackClip("extra", 0, 30, repeatRef("builtin:cool", 8)...)
	extra.Effects[0].On = false
	extra.Effects[1].Params["lut"] = ""
	extra.Effects[2] = EffectInstance{ID: "color", On: true, Params: map[string]any{}}
	project.Tracks = append(project.Tracks, Track{ID: "x", Name: "Extra", Kind: TrackVideo, Clips: []Clip{extra}})
	disabled := lutStackClip("off", 0, 30, repeatRef("builtin:warm", 8)...)
	disabled.Disabled = true
	project.Tracks = append(project.Tracks,
		Track{ID: "d", Name: "Disabled", Kind: TrackVideo, Clips: []Clip{disabled}},
		Track{ID: "h", Name: "Hidden", Kind: TrackVideo, Hidden: true, Clips: []Clip{lutStackClip("hid", 0, 30, repeatRef("builtin:warm", 8)...)}})
	if got := countLUTInstances(project, Segment{Start: 0, End: 30}); got != 64+5 {
		t.Fatalf("counted %d lut effects, want 69", got)
	}
	project.Tracks = project.Tracks[:8]
	if got := countLUTInstances(project, Segment{Start: 0, End: 30}); got != 64 {
		t.Fatalf("counted %d lut effects, want 64", got)
	}
	if segments, err := PlanVideoSegments(project, 0, 30, 1000); err != nil || len(segments) != 1 {
		t.Fatalf("exactly the cap: %v, %v", segments, err)
	}
}
