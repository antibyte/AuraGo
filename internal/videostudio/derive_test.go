package videostudio

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestReverseClipOrderAndChunkSeams(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	root := t.TempDir()
	source := filepath.Join(root, "counter.mp4")
	makeFrameCounterFixture(t, ctx, ffmpeg, source, 5)
	output := filepath.Join(root, "reversed.mp4")
	// A 64x32 source is below 1440p, so chunks hold two seconds: 150 frames cross two seams.
	if err := ReverseClip(ctx, ReverseOptions{FFmpegPath: ffmpeg, Input: source, Output: output, Offset: 0, Frames: 150,
		Rate: Rate{Num: 30, Den: 1}, Width: 64, Height: 32, Encoder: Encoder{Lossless: true}}); err != nil {
		t.Fatalf("ReverseClip: %v", err)
	}
	want := frameHashes(t, ctx, ffmpeg, source)
	got := frameHashes(t, ctx, ffmpeg, output)
	if len(got) != 150 || len(want) < 150 {
		t.Fatalf("frames: reversed %d, source %d", len(got), len(want))
	}
	for k := 0; k < 150; k++ {
		if got[k] != want[149-k] {
			t.Fatalf("reversed frame %d is not source frame %d", k, 149-k)
		}
	}
}

func TestExtractStillExactFrame(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	root := t.TempDir()
	source := filepath.Join(root, "counter.mp4")
	makeFrameCounterFixture(t, ctx, ffmpeg, source, 2)
	still := filepath.Join(root, "still.png")
	if err := ExtractStill(ctx, ffmpeg, source, still, 37, Rate{Num: 30, Den: 1}); err != nil {
		t.Fatalf("ExtractStill: %v", err)
	}
	got := rawFrame(t, ctx, ffmpeg, "-i", still, "-f", "rawvideo", "-pix_fmt", "rgb24", "pipe:1")
	want := rawFrame(t, ctx, ffmpeg, "-i", source, "-vf", "select=eq(n\\,37),scale=in_color_matrix=bt709:out_range=pc,format=rgb24",
		"-frames:v", "1", "-f", "rawvideo", "pipe:1")
	if len(got) == 0 || !bytes.Equal(got, want) {
		t.Fatalf("still frame 37 differs from the decoded source frame 37 (%d and %d bytes)", len(got), len(want))
	}
}

// rawFrame returns the raw output of one FFmpeg command.
func rawFrame(t *testing.T, ctx context.Context, ffmpeg string, args ...string) []byte {
	t.Helper()
	full := append([]string{"-hide_banner", "-nostdin", "-loglevel", "error"}, args...)
	out, err := exec.CommandContext(ctx, ffmpeg, full...).Output()
	if err != nil {
		t.Fatalf("ffmpeg %v: %v", args, err)
	}
	return out
}

func TestReverseRangeLimitAndChunkLength(t *testing.T) {
	rate := Rate{Num: 30, Den: 1}
	if got := MaxReverseFrames(rate); got != 9000 {
		t.Fatalf("MaxReverseFrames = %d, want 9000", got)
	}
	err := ReverseClip(context.Background(), ReverseOptions{Input: "unused.mp4", Output: "unused-out.mp4", Frames: 9001, Rate: rate})
	if !errors.Is(err, ErrReverseRangeTooLong) {
		t.Fatalf("a 9001-frame reverse returned %v, want ErrReverseRangeTooLong", err)
	}
	if got := ReverseChunkFrames(rate, 2560, 1440); got != 30 {
		t.Fatalf("1440p chunk = %d frames, want 30", got)
	}
	if got := ReverseChunkFrames(rate, 1280, 720); got != 60 {
		t.Fatalf("720p chunk = %d frames, want 60", got)
	}
}
