package videostudio

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

// utf16Count counts UTF-16 code units with the standard library, independently of utf16Units.
func utf16Count(s string) int { return len(utf16.Encode([]rune(s))) }

// volumeRoot is the root of the volume the temporary files live on ("C:\" on Windows, "/" elsewhere).
func volumeRoot() string { return filepath.VolumeName(os.TempDir()) + string(filepath.Separator) }

// specialDirName is a directory name with every character that filter-graph escaping has to protect, except ':' and
// the backslash, which no Windows name may hold.
const specialDirName = "a b'c,d;e[f]g=h%i"

// deepTestDir creates nested directories below t.TempDir() until the path is longer than units UTF-16 units. Every name
// holds the characters of specialDirName.
func deepTestDir(t *testing.T, units int) string {
	t.Helper()
	dir := t.TempDir()
	for i := 0; utf16Count(dir) <= units; i++ {
		dir = filepath.Join(dir, fmt.Sprintf("%s-%d", specialDirName, i))
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

// dirOfUnits creates nested directories below t.TempDir() whose path is exactly units UTF-16 units long.
func dirOfUnits(t *testing.T, units int) string {
	t.Helper()
	dir := t.TempDir()
	for utf16Count(dir) < units {
		need := units - utf16Count(dir) // a separator plus a name
		if need < 2 {
			t.Fatalf("%q is already %d units long", dir, utf16Count(dir))
		}
		name := min(need-1, 60)
		if need-1-name == 1 { // the rest must hold a separator and at least one character
			name--
		}
		dir = filepath.Join(dir, strings.Repeat("x", name))
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestFilterPath(t *testing.T) {
	cases := []struct{ name, path, want string }{
		{"plain", "/srv/aurago/vs_lut_0.cube", `/srv/aurago/vs_lut_0.cube`},
		{"drive letter", "C:/Users/aurago/vs_cmd_0_0.txt", `C\\:/Users/aurago/vs_cmd_0_0.txt`},
		{"space", "C:/My Videos/vs_cmd_0_0.txt", `C\\:/My Videos/vs_cmd_0_0.txt`},
		{"apostrophe", "/srv/it's/vs_cmd_0_0.txt", `/srv/it\\\'s/vs_cmd_0_0.txt`},
		{"comma and semicolon", "/srv/a,b;c/vs_cmd_0_0.txt", `/srv/a\,b\;c/vs_cmd_0_0.txt`},
		{"brackets", "/srv/[x]/vs_cmd_0_0.txt", `/srv/\[x\]/vs_cmd_0_0.txt`},
		{"equals and percent", "/srv/a=b%c/vs_cmd_0_0.txt", `/srv/a=b%c/vs_cmd_0_0.txt`},
		{"every special character", "C:/x/" + specialDirName + "/vs_lut_0.cube", `C\\:/x/a b\\\'c\,d\;e\[f\]g=h%i/vs_lut_0.cube`},
		{"non-ASCII", "/srv/Vidéo/日本/𝄞/vs_cmd_0_0.txt", "/srv/Vidéo/日本/𝄞/vs_cmd_0_0.txt"},
	}
	if runtime.GOOS == "windows" {
		cases = append(cases, struct{ name, path, want string }{"backslashes", `C:\data\vid eo\vs_cmd_0_0.txt`, `C\\:/data/vid eo/vs_cmd_0_0.txt`})
	} else {
		cases = append(cases, struct{ name, path, want string }{"backslash", `/srv/a\b/vs_cmd_0_0.txt`, `/srv/a\\\\b/vs_cmd_0_0.txt`})
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := filterPath(c.path)
			if err != nil || got != c.want {
				t.Fatalf("filterPath(%q) = %q, %v; want %q", c.path, got, err, c.want)
			}
		})
	}
	for _, bad := range []string{"/srv/a\nb/x.txt", "/srv/a\rb/x.txt", "/srv/a\tb/x.txt", "/srv/a\x00b/x.txt", "/srv/a\x1fb/x.txt", "/srv/a\x7fb/x.txt"} {
		if got, err := filterPath(bad); err == nil {
			t.Errorf("filterPath(%q) = %q, want an error for the control character", bad, got)
		}
	}
}

// The limit counts UTF-16 code units, not bytes or runes; a longer directory runs FFmpeg in its volume root.
func TestFFmpegWorkDirLimit(t *testing.T) {
	root := volumeRoot()
	// dirOf is root, then filler while it fits, then ASCII padding up to exactly units UTF-16 units.
	dirOf := func(units int, filler string) string {
		dir := root
		for utf16Count(dir+filler) <= units {
			dir += filler
		}
		for utf16Count(dir) < units {
			dir += "a"
		}
		return dir
	}
	for _, c := range []struct {
		name, filler string
		units        int
		bare         bool
	}{
		{"ASCII at the limit", "a", maxWorkDirUnits, true},
		{"ASCII one over", "a", maxWorkDirUnits + 1, false},
		{"BMP runes are one unit but two bytes", "é", maxWorkDirUnits, true},
		{"BMP runes one over", "é", maxWorkDirUnits + 1, false},
		{"supplementary runes are two units", "𝄞", maxWorkDirUnits, true},
		{"supplementary runes one over", "𝄞", maxWorkDirUnits + 1, false},
		{"supplementary runes far over", "𝄞", 2 * maxWorkDirUnits, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := dirOf(c.units, c.filler)
			if got := utf16Count(dir); got != c.units {
				t.Fatalf("test directory has %d units, want %d", got, c.units)
			}
			runDir, bare := ffmpegWorkDir(dir)
			want := root
			if c.bare {
				want = dir
			}
			if runDir != want || bare != c.bare {
				t.Fatalf("ffmpegWorkDir(%d units) = %q, %t; want %q, %t", c.units, runDir, bare, want, c.bare)
			}
		})
	}
	if runtime.GOOS == "windows" {
		unc := `\\server\share\` + strings.Repeat("a", maxWorkDirUnits)
		if runDir, bare := ffmpegWorkDir(unc); bare || runDir != `\\server\share\` {
			t.Fatalf("ffmpegWorkDir(UNC path) = %q, %t; want the share root", runDir, bare)
		}
	}
}

func TestAbsoluteStagedGraph(t *testing.T) {
	root, rootEscaped := string(filepath.Separator), "/"
	if runtime.GOOS == "windows" {
		root, rootEscaped = `C:\`, `C\\:/`
	}
	workDir := filepath.Join(root, "x y'z", "vsg-1")
	escapedDir := rootEscaped + `x y\\\'z/vsg-1/`
	files := graphFiles{
		commands: []sendCmdFile{{name: "vs_cmd_0_1.txt"}, {name: "vs_cmd_0_1_1.txt"}, {name: "vs_cmd_2_0_m1.txt"}},
		luts:     []lutFile{{name: "vs_lut_1.cube"}, {name: "vs_lut_10.cube"}},
	}
	graph := "[0:v]sendcmd=f=vs_cmd_0_1.txt,format=rgba[a];[a]sendcmd=f=vs_cmd_0_1_1.txt,lut3d=file=vs_lut_1.cube:interp=trilinear," +
		"lut3d=file=vs_lut_10.cube:interp=trilinear[b];[b]sendcmd=f=vs_cmd_2_0_m1.txt,null[vs_video_out]"
	want := "[0:v]sendcmd=f=" + escapedDir + "vs_cmd_0_1.txt,format=rgba[a];[a]sendcmd=f=" + escapedDir + "vs_cmd_0_1_1.txt,lut3d=file=" +
		escapedDir + "vs_lut_1.cube:interp=trilinear,lut3d=file=" + escapedDir + "vs_lut_10.cube:interp=trilinear[b];[b]sendcmd=f=" +
		escapedDir + "vs_cmd_2_0_m1.txt,null[vs_video_out]"
	if got, err := absoluteStagedGraph(graph, workDir, files); err != nil || got != want {
		t.Fatalf("absoluteStagedGraph:\n got %q (%v)\nwant %q", got, err, want)
	}
	// Nothing to rewrite: the graph is returned as it is.
	if got, err := absoluteStagedGraph("[0:v]null[vs_video_out]", workDir, graphFiles{}); err != nil || got != "[0:v]null[vs_video_out]" {
		t.Fatalf("graph without staged files = %q, %v", got, err)
	}
	// A directory whose own name looks like a bare reference is not mistaken for a leftover one.
	odd := filepath.Join(root, "x=vs_cmd_y=vs_lut_z", "vsg-1")
	if got, err := absoluteStagedGraph("sendcmd=f=vs_cmd_0_1.txt", odd, graphFiles{commands: files.commands[:1]}); err != nil || !strings.Contains(got, "x=vs_cmd_y=vs_lut_z") {
		t.Fatalf("directory with a bare-looking name = %q, %v", got, err)
	}
	// A bare staged name that no file covers, and a path that cannot be escaped, are errors.
	for name, c := range map[string]struct{ graph, dir string }{
		"uncovered command file": {"sendcmd=f=vs_cmd_9_9.txt", workDir},
		"uncovered LUT file":     {"lut3d=file=vs_lut_7.cube:interp=trilinear", workDir},
		"covered and uncovered":  {"sendcmd=f=vs_cmd_0_1.txt,sendcmd=f=vs_cmd_0_2.txt", workDir},
		"control character":      {"sendcmd=f=vs_cmd_0_1.txt", filepath.Join(root, "a\nb", "vsg-1")},
	} {
		if got, err := absoluteStagedGraph(c.graph, c.dir, files); err == nil {
			t.Errorf("%s: graph %q was accepted", name, got)
		}
	}
}

// Under a root longer than a Windows working directory the staged graph names every file by escaped absolute path, and
// the files are written where it says. A short root keeps the bare names.
func TestStageVideoGraphDeepRoot(t *testing.T) {
	rate := Rate{Num: 25, Den: 1}
	table, err := ParseCube(strings.NewReader(lutTestCube(2, lutIdentity, "\n")))
	if err != nil {
		t.Fatal(err)
	}
	tracks := []SendCmdTrack{{Target: "colorchannelmixer@vs_opacity_0_0", Option: "aa", Values: []float64{1, 0.5}}}
	files := graphFiles{
		commands: []sendCmdFile{{name: "vs_cmd_0_0.txt", tracks: tracks}, {name: "vs_cmd_0_0_1.txt", tracks: tracks}},
		luts:     []lutFile{{name: "vs_lut_0.cube", lut: table}},
	}
	graph := "[0:v]sendcmd=f=vs_cmd_0_0.txt,lut3d=file=vs_lut_0.cube:interp=trilinear,sendcmd=f=vs_cmd_0_0_1.txt[vs_video_out]"

	workDir, graphPath, err := stageVideoGraph(t.TempDir(), graph, rate, files)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(graphPath); err != nil || string(got) != graph {
		t.Fatalf("short root: graph.txt = %q, %v; want the bare graph", got, err)
	}
	if err := os.RemoveAll(workDir); err != nil {
		t.Fatal(err)
	}

	deep := deepTestDir(t, maxWorkDirUnits+50)
	workDir, graphPath, err = stageVideoGraph(deep, graph, rate, files)
	if err != nil {
		t.Fatal(err)
	}
	if _, bare := ffmpegWorkDir(workDir); bare || utf16Count(workDir) <= maxWorkDirUnits {
		t.Fatalf("staging directory %q (%d units) is not over the limit", workDir, utf16Count(workDir))
	}
	got, err := os.ReadFile(graphPath)
	if err != nil {
		t.Fatal(err)
	}
	want := graph
	for _, name := range []string{"vs_cmd_0_0.txt", "vs_cmd_0_0_1.txt", "vs_lut_0.cube"} {
		escaped, err := filterPath(filepath.Join(workDir, name))
		if err != nil {
			t.Fatal(err)
		}
		want = strings.Replace(want, "="+name, "="+escaped, 1)
		if info, err := os.Stat(filepath.Join(workDir, name)); err != nil || info.Size() == 0 {
			t.Fatalf("%s was not written: %v, %v", name, info, err)
		}
	}
	if string(got) != want || strings.Contains(string(got), "=vs_cmd_") || strings.Contains(string(got), "=vs_lut_") {
		t.Fatalf("graph.txt under a deep root:\n got %q\nwant %q", got, want)
	}
	if !strings.Contains(string(got), `a b\\\'c\,d\;e\[f\]g=h%i-0`) {
		t.Fatalf("directory names are not escaped in %q", got)
	}
	if err := os.RemoveAll(workDir); err != nil {
		t.Fatal(err)
	}

	// A graph that names a file no entry stages fails, and nothing is left behind.
	if _, _, err := stageVideoGraph(deep, graph+"sendcmd=f=vs_cmd_9_9.txt", rate, files); err == nil {
		t.Fatal("a graph with an unstaged bare reference was staged")
	}
	if entries, err := os.ReadDir(deep); err != nil || len(entries) != 0 {
		t.Fatalf("failed staging left %d entries (%v)", len(entries), err)
	}
}

// The graph of a segment that needs both kinds of staged file: an opacity command script and a LUT table.
func deepStagingProject(t *testing.T, ctx context.Context, ffmpeg, root string) (Project, map[string]string) {
	t.Helper()
	paths := map[string]string{"motion": filepath.Join(root, "motion.mp4")}
	makeLavfiVideoFixture(t, ctx, ffmpeg, paths["motion"], "testsrc2=s=160x90:r=25:d=3")
	project := Project{Version: CurrentVersion, Name: "Deep staging", Width: 720, Height: 720, FPS: 25}
	asset, err := Probe(ctx, ffmpeg, paths["motion"])
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	asset.ID = "motion"
	project.Assets = []Asset{AssetAtRate(asset, ProjectRate(project))}
	project.Tracks = []Track{{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{{
		ID: "a", AssetID: "motion", Duration: 40, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain,
		Keyframes: map[string][]Keyframe{"opacity": {{F: 0, V: 0.2, E: EaseLinear}, {F: 39, V: 1, E: EaseLinear}}},
		Effects:   []EffectInstance{{ID: "lut", On: true, Params: map[string]any{"lut": "builtin:teal_orange", "intensity": 0.7}}},
	}}}}
	if err := Validate(project); err != nil {
		t.Fatalf("deep staging project invalid: %v", err)
	}
	sources, err := collectRenderSources(ctx, ffmpeg, project, paths)
	if err != nil {
		t.Fatal(err)
	}
	_, graph, files, err := buildVideoSegmentWith(project, Segment{Start: 5, End: 6}, sources, ProjectRate(project), videoTail{width: 720, height: 720, rgb: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(graph, "sendcmd=f=vs_cmd_") || !strings.Contains(graph, "lut3d=file=vs_lut_") || len(files.commands) == 0 || len(files.luts) == 0 {
		t.Fatalf("the project does not stage a command file and a LUT file:\n%s", graph)
	}
	return project, paths
}

// A frame renders to the same PNG whether its output sits in a short directory or in one whose vsg-* staging directory
// is too long for a Windows working directory (the process then runs in the volume root).
func TestRenderFrameDeepStagingDirectory(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	project, paths := deepStagingProject(t, ctx, ffmpeg, t.TempDir())
	short, deep := t.TempDir(), deepTestDir(t, 320)
	for _, frame := range []int{5, 30} {
		render := func(dir string) []byte {
			output := filepath.Join(dir, fmt.Sprintf("frame-%d.png", frame))
			if err := RenderFrame(ctx, FrameOptions{FFmpegPath: ffmpeg, Project: project, AssetPaths: paths, Frame: frame, Output: output}); err != nil {
				t.Fatalf("RenderFrame(frame %d) in %d units: %v", frame, utf16Count(dir), err)
			}
			data, err := os.ReadFile(output)
			if err != nil || len(data) == 0 {
				t.Fatalf("frame %d: %d bytes, %v", frame, len(data), err)
			}
			return data
		}
		if !bytes.Equal(render(short), render(deep)) {
			t.Fatalf("frame %d differs between a short and a deep output directory", frame)
		}
	}
	for _, dir := range []string{short, deep} {
		if entries, err := os.ReadDir(dir); err != nil || len(entries) != 2 {
			t.Fatalf("%d entries left in the output directory (%v), want the two PNGs", len(entries), err)
		}
	}
}

// A segmented export renders the same frames whether its output sits in a short directory or in one whose segments-* and
// vsg-* directories are too long for a Windows working directory.
func TestRenderSegmentsDeepStagingDirectory(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	project, paths := deepStagingProject(t, ctx, ffmpeg, t.TempDir())
	render := func(dir string) []string {
		output := filepath.Join(dir, "export.mp4")
		if err := RenderWithOptions(ctx, RenderOptions{FFmpegPath: ffmpeg, Project: project, AssetPaths: paths, Output: output,
			SegmentFrames: 15, Encoder: Encoder{Lossless: true}}); err != nil {
			t.Fatalf("export in %d units: %v", utf16Count(dir), err)
		}
		return frameHashes(t, ctx, ffmpeg, output)
	}
	short := render(t.TempDir())
	if len(short) != 40 {
		t.Fatalf("%d frames, want 40", len(short))
	}
	deepDir := deepTestDir(t, 320)
	if deep := render(deepDir); strings.Join(deep, "\n") != strings.Join(short, "\n") {
		t.Fatal("the export in a deep directory differs from the export in a short one")
	}
	if entries, err := os.ReadDir(deepDir); err != nil || len(entries) != 1 {
		t.Fatalf("%d entries left in the output directory (%v), want the export", len(entries), err)
	}
}

// A staging directory of exactly maxWorkDirUnits units is still a working directory: FFmpeg starts there and opens the
// bare command and LUT files, whose full paths are longer than MAX_PATH.
func TestRunFFmpegInBareNamesAtWorkDirLimit(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	dir := dirOfUnits(t, maxWorkDirUnits)
	if runDir, bare := ffmpegWorkDir(dir); !bare || runDir != dir {
		t.Fatalf("ffmpegWorkDir(%d units) = %q, %t", utf16Count(dir), runDir, bare)
	}
	rate := Rate{Num: 25, Den: 1}
	tracks := []SendCmdTrack{{Target: "colorchannelmixer@vs_opacity_0_0", Option: "aa", Values: []float64{1, 0.5, 0.25}}}
	if err := WriteSendCmd(filepath.Join(dir, "vs_cmd_0_0.txt"), rate, tracks); err != nil {
		t.Fatal(err)
	}
	table, err := ParseCube(strings.NewReader(lutTestCube(2, lutIdentity, "\n")))
	if err != nil {
		t.Fatal(err)
	}
	if err := writeStagedLUT(filepath.Join(dir, "vs_lut_0.cube"), table); err != nil {
		t.Fatal(err)
	}
	if units := utf16Count(filepath.Join(dir, "vs_lut_0.cube")); units <= 260 {
		t.Fatalf("the LUT path has %d units, want more than MAX_PATH", units)
	}
	graph := "[0:v]format=rgba,sendcmd=f=vs_cmd_0_0.txt,colorchannelmixer@vs_opacity_0_0=aa=1,lut3d=file=vs_lut_0.cube:interp=trilinear,alphaextract[vs_video_out]"
	graphPath := filepath.Join(dir, "graph.txt")
	if err := os.WriteFile(graphPath, []byte(graph), 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "alpha.raw")
	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error", "-f", "lavfi", "-i", "color=c=red:s=16x16:r=25:d=0.12",
		"-filter_complex_script", graphPath, "-map", "[vs_video_out]", "-frames:v", "3", "-f", "rawvideo", "-pix_fmt", "gray", output}
	if err := runFFmpegIn(ctx, dir, ffmpeg, args, 0.12, nil, nil, output); err != nil {
		t.Fatalf("FFmpeg in a %d-unit staging directory: %v", utf16Count(dir), err)
	}
	data, err := os.ReadFile(output)
	if err != nil || len(data) != 3*256 {
		t.Fatalf("alpha frames: %d bytes, %v", len(data), err)
	}
	for k, want := range []int{255, 128, 64} {
		if got := int(data[k*256]); got < want-1 || got > want+1 {
			t.Fatalf("frame %d alpha %d, want %d (±1)", k, got, want)
		}
	}
}
