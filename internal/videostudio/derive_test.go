package videostudio

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
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
	if err := ExtractStill(ctx, StillOptions{FFmpegPath: ffmpeg, Input: source, Output: still, Rate: Rate{Num: 30, Den: 1}, Offset: 37}); err != nil {
		t.Fatalf("ExtractStill: %v", err)
	}
	if got, want := stillRGB(t, ctx, ffmpeg, still), sourceFrameRGB(t, ctx, ffmpeg, source, 37); len(got) == 0 || !bytes.Equal(got, want) {
		t.Fatalf("still frame 37 differs from the decoded source frame 37 (%d and %d bytes)", len(got), len(want))
	}
}

// A 30 fps source in a 25 fps project at 1.5x: the still of clip frame n is exactly the source frame the render shows
// there. Rounding the source position down to a project frame and sampling that at 1x picks a neighbouring frame for
// some n (n = 4 here), the case this guards.
func TestExtractStillAtSpeedMatchesRender(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	root := t.TempDir()
	source := filepath.Join(root, "steps.mp4")
	// Source frame N has luma 16 + 7N, so one decoded pixel names its source frame even after the render's scaling.
	video := "color=c=black:s=64x32:r=30:d=1,format=yuv420p,geq=lum='16+7*N':cb=128:cr=128"
	if out, err := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-nostdin", "-loglevel", "error", "-f", "lavfi", "-i", video,
		"-c:v", "libx264", "-preset", "ultrafast", "-qp", "0", "-pix_fmt", "yuv420p", "-f", "mp4", source).CombinedOutput(); err != nil {
		t.Fatalf("step fixture: %v: %s", err, out)
	}
	rate := Rate{Num: 25, Den: 1}
	asset, err := Probe(ctx, ffmpeg, source)
	if err != nil {
		t.Fatal(err)
	}
	asset = AssetAtRate(asset, rate)
	asset.ID = "steps"
	const offset, duration, speed = 2, 12, 1.5
	project := Project{Version: CurrentVersion, Name: "Still", Width: 720, Height: 720, FPS: 25, Assets: []Asset{asset},
		Tracks: []Track{{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{{ID: "c", AssetID: "steps", Offset: offset,
			Duration: duration, Speed: speed, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain}}}}}
	rendered := renderTestProject(t, ctx, ffmpeg, root, "speed", map[string]string{"steps": source}, project)
	// The 2x2 luma block at the canvas centre of every rendered frame (raw yuv420p: 4 luma, 1 Cb and 1 Cr byte).
	centre := rawFrame(t, ctx, ffmpeg, "-i", rendered, "-vf", "crop=2:2:360:360", "-f", "rawvideo", "-pix_fmt", "yuv420p", "pipe:1")
	if len(centre) < duration*6 {
		t.Fatalf("render decoded %d bytes, want %d frames", len(centre), duration)
	}
	sourceIndex := func(luma byte) int { return int(math.Round((float64(luma) - 16) / 7)) }
	neighbourCase := false
	for n := 0; n < duration; n++ {
		shown := sourceIndex(centre[n*6])
		still := filepath.Join(root, fmt.Sprintf("still-%d.png", n))
		if err := ExtractStill(ctx, StillOptions{FFmpegPath: ffmpeg, Input: source, Output: still, Rate: rate, Offset: offset, Frame: n, Speed: speed}); err != nil {
			t.Fatalf("ExtractStill clip frame %d: %v", n, err)
		}
		if got, want := stillRGB(t, ctx, ffmpeg, still), sourceFrameRGB(t, ctx, ffmpeg, source, shown); !bytes.Equal(got, want) {
			t.Fatalf("still of clip frame %d is not source frame %d, the frame the render shows", n, shown)
		}
		if fpsPick(math.Floor(offset+float64(n)*speed), 30, 25) != shown {
			neighbourCase = true
		}
	}
	if !neighbourCase {
		t.Fatal("no clip frame where sampling floor(offset + n*speed) at 1x picks another frame; the test lost its case")
	}
}

// fpsPick is the source frame FFmpeg's fps filter (round=near) shows at output frame f: the last source frame j whose
// timestamp j/sourceFPS rounds to an output frame at or before f.
func fpsPick(f float64, sourceFPS, projectFPS int) int {
	j := 0
	for math.Round(float64((j+1)*projectFPS)/float64(sourceFPS)) <= f {
		j++
	}
	return j
}

// stillRGB decodes a captured PNG to raw RGB24.
func stillRGB(t *testing.T, ctx context.Context, ffmpeg, still string) []byte {
	t.Helper()
	return rawFrame(t, ctx, ffmpeg, "-i", still, "-f", "rawvideo", "-pix_fmt", "rgb24", "pipe:1")
}

// sourceFrameRGB decodes source frame index the way ExtractStill converts it.
func sourceFrameRGB(t *testing.T, ctx context.Context, ffmpeg, source string, index int) []byte {
	t.Helper()
	return rawFrame(t, ctx, ffmpeg, "-i", source, "-vf", fmt.Sprintf("select=eq(n\\,%d),scale=in_color_matrix=bt709:out_range=pc,format=rgb24", index),
		"-frames:v", "1", "-f", "rawvideo", "pipe:1")
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

// TestReverseChunkFramesFitsMemoryBudget: a chunk's decoded yuv420p frames (1.5 bytes per pixel) stay within 256 MiB,
// the time-based length stays the upper bound and a chunk always holds at least one frame.
func TestReverseChunkFramesFitsMemoryBudget(t *testing.T) {
	r30, r60, ntsc := Rate{Num: 30, Den: 1}, Rate{Num: 60, Den: 1}, Rate{Num: 30000, Den: 1001}
	for _, tc := range []struct {
		name          string
		rate          Rate
		width, height int
		want          int
	}{
		{"720p30 keeps two seconds", r30, 1280, 720, 60},
		{"720p NTSC keeps two seconds", ntsc, 1280, 720, 59},
		{"1440p30 keeps one second", r30, 2560, 1440, 30},
		{"1080p60 fits the budget", r60, 1920, 1080, 86}, // floor(268435456 / 3110400)
		{"4K60 fits the budget", r60, 3840, 2160, 21},    // floor(268435456 / 12441600)
		{"4K30 fits the budget", r30, 3840, 2160, 21},
		{"64 MP fits the budget", r30, 8000, 8000, 2}, // floor(268435456 / 96000000)
		{"unknown size counts as the largest", r30, 0, 0, 2},
	} {
		got := ReverseChunkFrames(tc.rate, tc.width, tc.height)
		if got != tc.want {
			t.Errorf("%s: ReverseChunkFrames(%v, %d, %d) = %d, want %d", tc.name, tc.rate, tc.width, tc.height, got, tc.want)
		}
		if tc.width > 0 && got > 1 && float64(got)*float64(tc.width*tc.height)*reverseBytesPerPixel > reverseBufferBytes {
			t.Errorf("%s: %d frames exceed the %d-byte budget", tc.name, got, reverseBufferBytes)
		}
	}
	if got := reverseChunkFrames(r60, 3840, 2160, 1); got != 1 {
		t.Fatalf("a budget below one frame gives %d frames, want 1", got)
	}
	if got := reverseChunkFrames(r30, 64, 32, 1<<40); got != 60 {
		t.Fatalf("a huge budget gives %d frames, want the two-second length 60", got)
	}
}
