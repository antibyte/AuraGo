package videostudio

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestStageVideoGraphWritesAndRemoves(t *testing.T) {
	root := t.TempDir()
	rate := Rate{Num: 25, Den: 1}
	tracks := []SendCmdTrack{{Target: "colorchannelmixer@vs_opacity_0_1", Option: "aa", Values: []float64{0, 0.5}}}
	workDir, graphPath, err := stageVideoGraph(root, "[0:V:0]null[vs_video_out]", rate, graphFiles{commands: []sendCmdFile{{name: "vs_cmd_0_1.txt", tracks: tracks}}})
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(workDir) || filepath.Dir(workDir) != root || !strings.HasPrefix(filepath.Base(workDir), "vsg-") || graphPath != filepath.Join(workDir, "graph.txt") {
		t.Fatalf("staged %q with graph %q under %q", workDir, graphPath, root)
	}
	if graph, err := os.ReadFile(graphPath); err != nil || string(graph) != "[0:V:0]null[vs_video_out]" {
		t.Fatalf("graph.txt = %q, %v", graph, err)
	}
	want, _ := sendCmdScript(rate, tracks)
	if script, err := os.ReadFile(filepath.Join(workDir, "vs_cmd_0_1.txt")); err != nil || string(script) != want {
		t.Fatalf("command file = %q, %v; want %q", script, err, want)
	}
	if err := os.RemoveAll(workDir); err != nil {
		t.Fatal(err)
	}
	// Names other than vs_cmd_<L>_<C>[_<g>].txt, repeated names and invalid tracks fail, and a failed staging leaves nothing.
	for i, bad := range [][]sendCmdFile{
		{{name: "../vs_cmd_0_1.txt", tracks: tracks}},
		{{name: "graph.txt", tracks: tracks}},
		{{name: "vs_cmd_0_1.txt", tracks: tracks}, {name: "vs_cmd_0_1.txt", tracks: tracks}},
		{{name: "vs_cmd_0_2.txt", tracks: []SendCmdTrack{{Target: "overlay@vs_place_0_2", Option: "x", Values: []float64{1}}}}},
	} {
		if _, _, err := stageVideoGraph(root, "graph", rate, graphFiles{commands: bad}); err == nil {
			t.Errorf("bad command files %d were staged", i)
		}
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatalf("failed staging left %d entries (%v)", len(entries), err)
	}
}

// The graph names its command file by bare name; FFmpeg finds it only in the staging directory, and the commands reach
// their frames there (alpha 255, then 255·0.5, then 255·0.25).
func TestRunFFmpegInUsesWorkingDirectory(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	rate := Rate{Num: 25, Den: 1}
	tracks := []SendCmdTrack{{Target: "colorchannelmixer@vs_opacity_0_0", Option: "aa", Values: []float64{1, 0.5, 0.25}}}
	graph := "[0:v]format=rgba,sendcmd=f=vs_cmd_0_0.txt,colorchannelmixer@vs_opacity_0_0=aa=1,alphaextract[vs_video_out]"
	root := t.TempDir()
	workDir, graphPath, err := stageVideoGraph(root, graph, rate, graphFiles{commands: []sendCmdFile{{name: "vs_cmd_0_0.txt", tracks: tracks}}})
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(workDir)
	args := func(output string) []string {
		return []string{"-hide_banner", "-nostdin", "-loglevel", "error", "-f", "lavfi", "-i", "color=c=red:s=16x16:r=25:d=0.12",
			"-filter_complex_script", graphPath, "-map", "[vs_video_out]", "-frames:v", "3", "-f", "rawvideo", "-pix_fmt", "gray", output}
	}
	output := filepath.Join(root, "alpha.raw")
	if err := runFFmpegIn(ctx, workDir, ffmpeg, args(output), 0.12, nil, nil, output); err != nil {
		t.Fatalf("FFmpeg in the staging directory: %v", err)
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
	cwd := filepath.Join(root, "cwd.raw")
	if err := runFFmpeg(ctx, ffmpeg, args(cwd), 0.12, nil, nil, cwd); err == nil {
		t.Fatal("FFmpeg found the command file outside the staging directory")
	}
}

// resolveBinary returns absolute paths (exec.LookPath refuses relative PATH entries with exec.ErrDot), and a staging
// directory never runs a relative binary.
func TestResolveBinaryReturnsAbsolutePaths(t *testing.T) {
	root := t.TempDir()
	name := "ffmpeg"
	if runtime.GOOS == "windows" {
		name = "ffmpeg.exe"
	}
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, name), []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	t.Setenv("PATH", bin)
	for _, configured := range []string{"", filepath.Join("bin", name)} {
		got, err := resolveBinary(configured, "ffmpeg")
		if err != nil || !filepath.IsAbs(got) || filepath.Base(got) != name {
			t.Fatalf("resolveBinary(%q) = %q, %v; want an absolute path", configured, got, err)
		}
	}
	t.Setenv("PATH", "bin")
	if got, err := resolveBinary("", "ffmpeg"); err == nil && !filepath.IsAbs(got) {
		t.Fatalf("resolveBinary with a relative PATH entry = %q", got)
	}
	if err := runFFmpegIn(context.Background(), root, filepath.Join("bin", name), nil, 0, nil, nil, ""); err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("a relative binary in a staging directory = %v", err)
	}
}

// LUT files are staged next to the graph (index §4.11, §4.20):
//   - names vs_lut_<L>_<C>_<I>.cube, mode 0600;
//   - Go-written without TITLE or DOMAIN lines, with any domain resampled onto [0, 1];
//   - bad names, a missing or incomplete table and repeated names fail and leave nothing behind.
func TestStageVideoGraphWritesLUTFiles(t *testing.T) {
	root := t.TempDir()
	rate := Rate{Num: 25, Den: 1}
	unit, err := ParseCube(strings.NewReader(lutTestCube(3, func(r, g, b float64) [3]float64 { return [3]float64{r * r, g, 1 - b} }, "\n")))
	if err != nil {
		t.Fatal(err)
	}
	wide, err := ParseCube(strings.NewReader("LUT_3D_INPUT_RANGE 0 2\n" + lutTestCube(2, func(r, g, b float64) [3]float64 { return [3]float64{2 * r, 2 * g, 2 * b} }, "\n")))
	if err != nil {
		t.Fatal(err)
	}
	tracks := []SendCmdTrack{{Target: "cas@vs_sharpen_0_1_2", Option: "strength", Values: []float64{0.5, 1}}}
	files := graphFiles{
		commands: []sendCmdFile{{name: "vs_cmd_0_1.txt", tracks: tracks}, {name: "vs_cmd_0_1_1.txt", tracks: tracks}},
		luts:     []lutFile{{name: "vs_lut_0_1_0.cube", lut: unit}, {name: "vs_lut_0_1_3.cube", lut: wide}},
	}
	workDir, _, err := stageVideoGraph(root, "graph", rate, files)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"graph.txt", "vs_cmd_0_1.txt", "vs_cmd_0_1_1.txt", "vs_lut_0_1_0.cube", "vs_lut_0_1_3.cube"} {
		info, err := os.Stat(filepath.Join(workDir, name))
		if err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0o600) {
			t.Fatalf("%s: %v, %v", name, info, err)
		}
	}
	read := func(name string) (*LUT3D, string) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(workDir, name))
		if err != nil {
			t.Fatal(err)
		}
		table, err := ParseCube(strings.NewReader(string(data)))
		if err != nil {
			t.Fatalf("%s does not parse: %v", name, err)
		}
		return table, string(data)
	}
	if staged, text := read("vs_lut_0_1_0.cube"); !reflect.DeepEqual(staged, unit) || strings.Contains(text, "TITLE") || strings.Contains(text, "DOMAIN") {
		t.Fatalf("unit table staged as %+v:\n%.80s", staged, text)
	}
	if staged, text := read("vs_lut_0_1_3.cube"); strings.Contains(text, "DOMAIN") || staged.DomainMin != [3]float64{} || staged.DomainMax != [3]float64{1, 1, 1} || !reflect.DeepEqual(staged.Data, unitDomainLUT(wide).Data) {
		t.Fatalf("wide table staged as %+v:\n%.80s", staged, text)
	}
	if err := os.RemoveAll(workDir); err != nil {
		t.Fatal(err)
	}
	for i, bad := range []graphFiles{
		{luts: []lutFile{{name: "vs_lut_0_1.cube", lut: unit}}},
		{luts: []lutFile{{name: "../vs_lut_0_1_0.cube", lut: unit}}},
		{luts: []lutFile{{name: "vs_lut_0_1_0.CUBE", lut: unit}}},
		{luts: []lutFile{{name: "vs_lut_0_1_0.cube.txt", lut: unit}}},
		{luts: []lutFile{{name: "vs_lut_0_1_0.cube", lut: unit}, {name: "vs_lut_0_1_0.cube", lut: unit}}},
		{luts: []lutFile{{name: "vs_lut_0_1_0.cube"}}},
		{luts: []lutFile{{name: "vs_lut_0_1_0.cube", lut: &LUT3D{Size: 2, DomainMax: [3]float64{1, 1, 1}, Data: make([]float32, 3)}}}},
		{commands: []sendCmdFile{{name: "vs_cmd_0_1_1_2.txt", tracks: tracks}}},
	} {
		if _, _, err := stageVideoGraph(root, "graph", rate, bad); err == nil {
			t.Errorf("bad graph files %d were staged", i)
		}
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatalf("staging left %d entries (%v)", len(entries), err)
	}
}
