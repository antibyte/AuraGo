package videostudio

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestStageVideoGraphWritesAndRemoves(t *testing.T) {
	root := t.TempDir()
	rate := Rate{Num: 25, Den: 1}
	tracks := []SendCmdTrack{{Target: "colorchannelmixer@vs_opacity_0_1", Option: "aa", Values: []float64{0, 0.5}}}
	workDir, graphPath, err := stageVideoGraph(root, "[0:V:0]null[vs_video_out]", rate, []sendCmdFile{{name: "vs_cmd_0_1.txt", tracks: tracks}})
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
	// Names other than vs_cmd_<L>_<C>.txt, repeated names and invalid tracks fail, and a failed staging leaves nothing.
	for i, bad := range [][]sendCmdFile{
		{{name: "../vs_cmd_0_1.txt", tracks: tracks}},
		{{name: "graph.txt", tracks: tracks}},
		{{name: "vs_cmd_0_1.txt", tracks: tracks}, {name: "vs_cmd_0_1.txt", tracks: tracks}},
		{{name: "vs_cmd_0_2.txt", tracks: []SendCmdTrack{{Target: "overlay@vs_place_0_2", Option: "x", Values: []float64{1}}}}},
	} {
		if _, _, err := stageVideoGraph(root, "graph", rate, bad); err == nil {
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
	workDir, graphPath, err := stageVideoGraph(root, graph, rate, []sendCmdFile{{name: "vs_cmd_0_0.txt", tracks: tracks}})
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
