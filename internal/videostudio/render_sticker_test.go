package videostudio

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRenderBundledSticker1080p(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	source, err := filepath.Abs(filepath.Join("..", "..", "ui", "img", "stickers", "emotion-joy.png"))
	if err != nil {
		t.Fatal(err)
	}
	asset, err := Probe(ctx, ffmpeg, source)
	if err != nil {
		t.Fatal(err)
	}
	asset.ID = "sticker"
	project := Project{Version: CurrentVersion, Name: "Transparent sticker", Width: 1920, Height: 1080, FPS: 30, Assets: []Asset{asset},
		Tracks: []Track{{ID: "overlay", Name: "Sticker", Kind: TrackOverlay, Clips: []Clip{{
			ID: "clip", AssetID: asset.ID, Start: 0, Duration: 120, X: .4, Y: .35, Width: .135, Height: .24, Opacity: 1, Volume: 1, Fit: FitContain,
		}}}}}
	if err := Validate(project); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "sticker-1080p.mp4")
	if err := Render(ctx, ffmpeg, project, map[string]string{asset.ID: source}, output, nil); err != nil {
		t.Fatal(err)
	}
	rendered, err := Probe(ctx, ffmpeg, output)
	if err != nil {
		t.Fatal(err)
	}
	if rendered.Width != 1920 || rendered.Height != 1080 || rendered.DurationFrames < 119 || rendered.DurationFrames > 122 {
		t.Fatalf("wrong export: %+v", rendered)
	}
	frame, err := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-nostdin", "-loglevel", "error", "-ss", "1.0", "-i", output, "-frames:v", "1", "-f", "rawvideo", "-pix_fmt", "rgb24", "pipe:1").Output()
	if err != nil {
		t.Fatal(err)
	}
	if len(frame) != 1920*1080*3 {
		t.Fatalf("unexpected decoded frame size: %d", len(frame))
	}
	pixel := func(x, y int) []byte { offset := (y*1920 + x) * 3; return frame[offset : offset+3] }
	transparent := pixel(770, 380)
	for _, value := range transparent {
		if value > 20 {
			t.Fatalf("transparent corner became opaque: %v", transparent)
		}
	}
	visible := pixel(898, 508)
	if int(visible[0])+int(visible[1])+int(visible[2]) < 100 {
		t.Fatalf("sticker did not render: %v", visible)
	}
}
