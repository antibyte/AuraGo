package videostudio

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Run on a host with a VAAPI render node:
// AURAGO_VIDEO_STUDIO_VAAPI=1 [AURAGO_VIDEO_STUDIO_DEVICE=/dev/dri/renderD128] go test ./internal/videostudio -run TestVAAPIRenderOptIn -count=1 -v
func TestVAAPIRenderOptIn(t *testing.T) {
	if os.Getenv("AURAGO_VIDEO_STUDIO_VAAPI") != "1" {
		t.Skip("set AURAGO_VIDEO_STUDIO_VAAPI=1 on a host with a VAAPI render node")
	}
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	status := SelectEncoder(ctx, ffmpeg, "vaapi", os.Getenv("AURAGO_VIDEO_STUDIO_DEVICE"))
	if !status.Hardware {
		t.Fatalf("VAAPI unavailable: %s", status.Note)
	}
	root := t.TempDir()
	source := filepath.Join(root, "motion.mp4")
	makeLavfiVideoFixture(t, ctx, ffmpeg, source, "testsrc2=s=1920x1080:r=30:d=12")
	asset, err := Probe(ctx, ffmpeg, source)
	if err != nil {
		t.Fatal(err)
	}
	asset.ID = "motion"
	project := Project{Version: CurrentVersion, Name: "VAAPI", Width: 1920, Height: 1080, FPS: 30, Assets: []Asset{asset},
		Tracks: []Track{{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{{ID: "m", AssetID: "motion", Duration: 360, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain}}}}}
	output := filepath.Join(root, "vaapi.mp4")
	started := time.Now()
	if err := RenderWithOptions(ctx, RenderOptions{FFmpegPath: ffmpeg, Project: project, AssetPaths: map[string]string{"motion": source}, Output: output,
		Encoder: status.Encoder, OnEncoderFallback: func(err error) { t.Fatalf("VAAPI render fell back: %v", err) }}); err != nil {
		t.Fatal(err)
	}
	t.Logf("12 s 1080p VAAPI render took %s on %s", time.Since(started).Round(time.Millisecond), status.Device)
	if got, err := Probe(ctx, ffmpeg, output); err != nil || got.Codec != "h264" || got.Width != 1920 {
		t.Fatalf("VAAPI output = %+v, %v", got, err)
	}
	if err := PreparePreviewWith(ctx, ffmpeg, source, filepath.Join(root, "preview.mp4"), status.Encoder); err != nil {
		t.Fatalf("VAAPI preview: %v", err)
	}
}
