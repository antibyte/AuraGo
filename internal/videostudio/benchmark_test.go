package videostudio

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Run with AURAGO_VIDEO_STUDIO_BENCHMARK=1 go test ./internal/videostudio -run TestTenMinuteRenderPerformanceOptIn -count=1.
func TestTenMinuteRenderPerformanceOptIn(t *testing.T) {
	if os.Getenv("AURAGO_VIDEO_STUDIO_BENCHMARK") != "1" {
		t.Skip("set AURAGO_VIDEO_STUDIO_BENCHMARK=1 to run the local 10-minute render")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Hour)
	defer cancel()
	root := t.TempDir()
	sourcePath := filepath.Join(root, "synthetic.mp4")
	args := []string{
		"-hide_banner", "-nostdin", "-loglevel", "error",
		"-f", "lavfi", "-i", "color=c=0x173044:s=640x360:r=30:d=600",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=600",
		"-t", "600", "-c:v", "libx264", "-preset", "ultrafast", "-crf", "30",
		"-pix_fmt", "yuv420p", "-c:a", "aac", "-b:a", "64k", "-shortest", "-movflags", "+faststart", sourcePath,
	}
	if output, err := exec.CommandContext(ctx, ffmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("create 10-minute synthetic source: %v: %s", err, output)
	}
	asset, err := Probe(ctx, ffmpeg, sourcePath)
	if err != nil {
		t.Fatalf("Probe(synthetic source) error = %v", err)
	}
	asset.ID = "synthetic"
	if asset.Kind != AssetVideo || !asset.HasAudio || asset.DurationFrames < MaxDurationFrames-1 {
		t.Fatalf("synthetic source metadata = %+v", asset)
	}
	project := Project{
		Version: 1, Name: "10-minute local render check", Width: 1920, Height: 1080, FPS: FramesPerSecond,
		Assets: []Asset{asset},
		Tracks: []Track{{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{{
			ID: "full-length", AssetID: asset.ID, Start: 0, Duration: MaxDurationFrames,
			Width: 1, Height: 1, Opacity: 1, Volume: 0.5, Fit: FitContain,
		}}}},
	}
	outputPath := filepath.Join(root, "ten-minute-1080p.mp4")
	started := time.Now()
	if err := Render(ctx, ffmpeg, project, map[string]string{asset.ID: sourcePath}, outputPath, nil); err != nil {
		t.Fatalf("Render(10-minute 1080p) error = %v", err)
	}
	elapsed := time.Since(started)
	rendered, err := Probe(ctx, ffmpeg, outputPath)
	if err != nil {
		t.Fatalf("Probe(10-minute output) error = %v", err)
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("stat 10-minute output: %v", err)
	}
	t.Logf("10-minute 1080p synthetic render: elapsed=%s, output_duration=%.3fs, bytes=%d", elapsed.Round(time.Millisecond), float64(rendered.DurationFrames)/FramesPerSecond, info.Size())
	if rendered.Width != 1920 || rendered.Height != 1080 || rendered.DurationFrames < MaxDurationFrames-1 || !rendered.HasAudio {
		t.Fatalf("rendered output metadata = %+v", rendered)
	}
}
