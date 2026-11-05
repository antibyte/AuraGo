package render

import (
	"context"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCheckOutputRefusesWrongCountsAndExtraAudio(t *testing.T) {
	// Packets must equal the frames, and audio must lie within 25 ms of video.
	good := ProbeResult{VideoPackets: 90, VideoSeconds: 3, AudioSeconds: 3.01}
	if err := CheckOutput(good, 90); err != nil {
		t.Errorf("a good file was refused: %v", err)
	}
	if err := CheckOutput(good, 91); err == nil {
		t.Error("a wrong packet count passed")
	}
	if err := CheckOutput(ProbeResult{VideoPackets: 90, VideoSeconds: 3, AudioSeconds: 4}, 90); err == nil {
		t.Error("1 s of extra audio passed")
	}
	if err := CheckOutput(ProbeResult{VideoPackets: 90, VideoSeconds: 3, AudioSeconds: 2.9}, 90); err == nil {
		t.Error("audio 100 ms short passed")
	}
}

func TestProbeOutputHasBothWhitelistsAndItsBounds(t *testing.T) {
	orig := runGateProbe
	t.Cleanup(func() { runGateProbe = orig })
	var calls []string
	runGateProbe = func(ctx context.Context, path string, args []string, limit int) ([]byte, error) {
		deadline, ok := ctx.Deadline()
		// The timeout is the caller's: 150 s is not the 120 s floor.
		if !ok || time.Until(deadline) > 150*time.Second || time.Until(deadline) < 130*time.Second {
			t.Errorf("deadline = %v, %v", deadline, ok)
		}
		if limit != 8<<20 {
			t.Errorf("output cap = %d", limit)
		}
		joined := strings.Join(args, " ")
		calls = append(calls, joined)
		switch {
		case strings.Contains(joined, "-count_packets"):
			return []byte("90\n"), nil
		case strings.Contains(joined, "v:0"):
			return []byte("3.000000\n"), nil
		}
		return []byte("3.000400\n"), nil
	}
	got, err := ProbeOutput(context.Background(), "ffprobe", "out.mp4", 150*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got.VideoPackets != 90 || got.VideoSeconds != 3 || got.AudioSeconds != 3.0004 {
		t.Errorf("result = %+v", got)
	}
	want := []string{
		"-v error -protocol_whitelist file -format_whitelist mov -count_packets -select_streams v:0 -show_entries stream=nb_read_packets -of csv=p=0 -i out.mp4",
		"-v error -protocol_whitelist file -format_whitelist mov -select_streams v:0 -show_entries stream=duration -of csv=p=0 -i out.mp4",
		"-v error -protocol_whitelist file -format_whitelist mov -select_streams a:0 -show_entries stream=duration -of csv=p=0 -i out.mp4",
	}
	if strings.Join(calls, "\n") != strings.Join(want, "\n") {
		t.Errorf("calls =\n%s\nwant\n%s", strings.Join(calls, "\n"), strings.Join(want, "\n"))
	}
}

func TestProbeOutputReadsARealFile(t *testing.T) {
	ffmpeg, ffprobe := testBinary(t, "ffmpeg"), testBinary(t, "ffprobe")
	const w, h, fps, n = 160, 90, 30, 45
	dir := t.TempDir()
	out := filepath.Join(dir, "out.mp4")
	red := solidPNG(t, w, h, color.NRGBA{R: 255, A: 255})
	frames := make([][]byte, n)
	for i := range frames {
		frames[i] = red
	}
	ff, err := encodeFrames(t, FFmpegConfig{Path: ffmpeg, Transport: TransportPNG, Width: w, Height: h, FPS: fps, Frames: n, Output: out}, frames)
	if err != nil {
		t.Fatalf("encode: %v (%s)", err, ff.Message())
	}
	got, err := ProbeOutput(context.Background(), ffprobe, out, postEncodeMinLimit)
	if err != nil {
		t.Fatal(err)
	}
	if got.VideoPackets != n {
		t.Errorf("packets = %d, want %d", got.VideoPackets, n)
	}
	if err := CheckOutput(got, n); err != nil {
		t.Errorf("a normal encode was refused: %v (%+v)", err, got)
	}

	bad := filepath.Join(dir, "bad.mp4")
	if err := os.WriteFile(bad, []byte("this is not a video"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = ProbeOutput(context.Background(), ffprobe, bad, postEncodeMinLimit)
	if err == nil {
		t.Fatal("a broken file passed the probe")
	}
	if strings.Contains(err.Error(), dir) || strings.Contains(err.Error(), filepath.ToSlash(dir)) {
		t.Errorf("the error holds the staging path: %v", err)
	}
}

func TestPostEncodeLimitScalesWithTheFileSize(t *testing.T) {
	// The larger of 120 s and the size at 10 MiB/s: they meet at 1200 MiB, and 2 GiB, the largest file allowed, gets 204.8 s.
	for _, c := range []struct {
		size int64
		want time.Duration
	}{
		{0, 120 * time.Second},
		{1200 << 20, 120 * time.Second},
		{1500 << 20, 150 * time.Second},
		{maxOutputBytes, 204800 * time.Millisecond},
		{maxOutputBytes * 8, 204800 * time.Millisecond},
	} {
		if got := postEncodeLimit(c.size); got != c.want {
			t.Errorf("postEncodeLimit(%d) = %v, want %v", c.size, got, c.want)
		}
	}
}

func TestFFmpegWaitEndsAtTheGivenLimit(t *testing.T) {
	ffmpeg := testBinary(t, "ffmpeg")
	ff, err := StartFFmpeg(context.Background(), FFmpegConfig{Path: ffmpeg, Transport: TransportPNG, Width: 64, Height: 64, FPS: 30, Frames: 5, Output: filepath.Join(t.TempDir(), "x.mp4")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ff.Discard)
	// The frame input stays open, so FFmpeg cannot end by itself and only the limit stops the wait.
	start := time.Now()
	err = ff.Wait(context.Background(), 300*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "did not finish") {
		t.Fatalf("wait = %v", err)
	}
	if took := time.Since(start); took > 30*time.Second {
		t.Errorf("the wait took %v", took)
	}
}

func TestRunPublishesNothingWhenTheFrameGateRefuses(t *testing.T) {
	orig := runGateProbe
	t.Cleanup(func() { runGateProbe = orig })
	runGateProbe = func(ctx context.Context, path string, args []string, limit int) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), "-count_packets") {
			// One packet short of the 24 frames the page sends.
			return []byte("23\n"), nil
		}
		return orig(ctx, path, args, limit)
	}
	fx := newRunFixture(t)
	fx.playing(playOptions{frames: 24})
	job := fx.render(t)
	if job.Status != StatusFailed || job.Error == nil || job.Error.Code != CodeRenderFailed || !strings.Contains(job.Error.Message, "23") {
		t.Fatalf("job = %+v error %+v", job, job.Error)
	}
	if names := fx.pub.names(); len(names) != 0 {
		t.Errorf("a refused file published %v", names)
	}
}
