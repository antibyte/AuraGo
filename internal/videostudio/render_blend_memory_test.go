package videostudio

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"
)

// benchmarkVideoSegment renders seg of project through the export's video path with FFmpeg's -benchmark and returns the
// peak memory the process reports ("bench: maxrss=<n>KiB"). It mirrors renderVideoSegment (graph, staging, arguments,
// encoder, MPEG-TS output) and differs in three arguments only: -benchmark, -loglevel info (the bench lines are info
// messages) and no -progress. FFmpeg's maxrss is the peak resident set on Linux and the peak commit (private bytes) on
// Windows, so compare figures within one platform.
func benchmarkVideoSegment(t *testing.T, ctx context.Context, ffmpeg string, project Project, seg Segment, sources map[string]renderSource, root string) int64 {
	t.Helper()
	rate := ProjectRate(project)
	encoder := Encoder{}
	inputs, graph, files, err := buildVideoSegment(project, seg, sources, rate, project.Width, project.Height, encoder.filterTail())
	if err != nil {
		t.Fatal(err)
	}
	workDir, graphPath, err := stageVideoGraph(root, graph, rate, files)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(workDir)
	output := filepath.Join(root, "segment.ts")
	defer os.Remove(output)
	args := append([]string{"-hide_banner", "-nostdin", "-loglevel", "info", "-benchmark", "-copyts", "-start_at_zero"}, encoder.globalArgs()...)
	args = append(args, segmentInputArgs(inputs, rate)...)
	args = append(args,
		"-filter_threads", strconv.Itoa(threadLimit), "-filter_complex_threads", strconv.Itoa(threadLimit),
		"-filter_complex_script", graphPath, "-map", "[vs_video_out]", "-map_metadata", "-1",
		"-frames:v", strconv.Itoa(seg.End-seg.Start), "-r", rate.String())
	args = append(args, encoder.videoArgs(rate)...)
	args = append(args, "-threads", strconv.Itoa(threadLimit), "-an", "-nostats", "-f", "mpegts", output)
	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("segment run: %v\n%s", err, out)
	}
	match := regexp.MustCompile(`bench: maxrss=(\d+)`).FindSubmatch(out)
	if match == nil {
		t.Fatalf("FFmpeg printed no maxrss line:\n%s", out)
	}
	kib, err := strconv.ParseInt(string(match[1]), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return kib
}

// Run with AURAGO_VIDEO_STUDIO_BENCHMARK=1 go test ./internal/videostudio -run '^TestBlendSplitMemoryRecordOptIn$' -count=1 -v.
//
// TestBlendSplitMemoryRecordOptIn records the peak memory of one 10 s 1080p30 export segment (300 frames, one segment
// like the default export length) whose top video track changes its blend mode inside the window (plan 2d review,
// reader claim). It asserts nothing.
//
// A layer with a mode change is split once per run (splitBlendLayers: split, one trim per run, one concat per mode), and
// FFmpeg's split pushes every frame to all outputs. The claim was that a later run's branch keeps the earlier frames
// queued until its concat reaches it, at 8.3 MB per rgba frame (1.2 GB for a change at 5 s, 2.5 GB at 9 s). The 2c
// transition split (appendTransition) has the same shape. Cases, all on a second video track over a full-frame video:
//   - a: one clip (0 to 10 s), normal;
//   - b: normal 0 to 5 s, then multiply 5 to 10 s on the same track;
//   - c: normal 0 to 9 s, then multiply 9 to 10 s;
//   - d: two normal clips with a 1 s dissolve starting at 5 s (the 2c transition split);
//   - e: one multiply clip (0 to 10 s), the control for the blend composite without any split.
//
// Each case runs three times with the default encoder and the export's thread limit; the log has the lowest and the
// highest peak and the difference of the highest to case a.
func TestBlendSplitMemoryRecordOptIn(t *testing.T) {
	if os.Getenv("AURAGO_VIDEO_STUDIO_BENCHMARK") != "1" {
		t.Skip("set AURAGO_VIDEO_STUDIO_BENCHMARK=1 to record the peak memory of blend splits")
	}
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Minute)
	defer cancel()
	root := t.TempDir()
	paths := map[string]string{"back": filepath.Join(root, "back.mp4"), "front": filepath.Join(root, "front.mp4")}
	makeLavfiVideoFixture(t, ctx, ffmpeg, paths["back"], "testsrc2=s=1920x1080:r=30:d=10")
	makeLavfiVideoFixture(t, ctx, ffmpeg, paths["front"], "testsrc=s=1920x1080:r=30:d=10")
	const frames = 300
	project := Project{Version: CurrentVersion, Name: "Blend split memory", Width: 1920, Height: 1080, FPS: 30}
	rate := ProjectRate(project)
	for _, id := range []string{"back", "front"} {
		asset, err := Probe(ctx, ffmpeg, paths[id])
		if err != nil {
			t.Fatalf("Probe(%s): %v", id, err)
		}
		asset.ID = id
		project.Assets = append(project.Assets, AssetAtRate(asset, rate))
	}
	clip := func(id string, start, duration int, blend string, transition *Transition) Clip {
		return Clip{ID: id, AssetID: "front", Start: start, Offset: start, Duration: duration, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, Blend: blend, Transition: transition}
	}
	type memoryCase struct {
		name  string
		clips []Clip
	}
	cases := []memoryCase{
		{"a: one clip, normal", []Clip{clip("a", 0, frames, "", nil)}},
		{"b: normal 0-5 s, multiply 5-10 s", []Clip{clip("b1", 0, 150, "", nil), clip("b2", 150, 150, "multiply", nil)}},
		{"c: normal 0-9 s, multiply 9-10 s", []Clip{clip("c1", 0, 270, "", nil), clip("c2", 270, 30, "multiply", nil)}},
		{"d: dissolve of 1 s starting at 5 s, normal", []Clip{clip("d1", 0, 180, "", &Transition{Type: TransitionDissolve, Duration: 30}), clip("d2", 150, 150, "", nil)}},
		{"e: one clip, multiply (no split)", []Clip{clip("e", 0, frames, "multiply", nil)}},
	}
	major := ffmpegMajor(t, ffmpeg)
	var baseline int64
	for _, c := range cases {
		p := project
		p.Name = c.name
		p.Tracks = []Track{
			{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{{ID: "back", AssetID: "back", Duration: frames, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain}}},
			{ID: "front", Name: "Front", Kind: TrackVideo, Clips: c.clips},
		}
		if err := Validate(p); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		plan, err := PlanVideoSegments(p, 0, frames, frames)
		if err != nil || len(plan) != 1 || plan[0] != (Segment{Start: 0, End: frames}) {
			t.Fatalf("%s: segments %v, %v; want one segment of %d frames", c.name, plan, err, frames)
		}
		sources, err := collectRenderSources(ctx, ffmpeg, p, paths)
		if err != nil {
			t.Fatal(err)
		}
		var low, high int64
		for run := range 3 {
			peak := benchmarkVideoSegment(t, ctx, ffmpeg, p, plan[0], sources, root)
			if run == 0 || peak < low {
				low = peak
			}
			high = max(high, peak)
		}
		if baseline == 0 {
			baseline = high
		}
		t.Logf("FFmpeg %d, %s: maxrss %d to %d MiB over three runs, %+d MiB over case a", major, c.name, low>>10, high>>10, (high-baseline)>>10)
	}
}
