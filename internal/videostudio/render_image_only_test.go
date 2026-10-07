package videostudio

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRenderImageOnlyProjectWithEmptyTracks(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	root := t.TempDir()
	imagePath := filepath.Join(root, "overlay.png")
	imageFixture := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			imageFixture.Set(x, y, color.RGBA{R: 12, G: 220, B: 24, A: 255})
		}
	}
	file, err := os.Create(imagePath)
	if err != nil {
		t.Fatalf("create image fixture: %v", err)
	}
	if err := png.Encode(file, imageFixture); err != nil {
		_ = file.Close()
		t.Fatalf("encode image fixture: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close image fixture: %v", err)
	}

	asset, err := Probe(ctx, ffmpeg, imagePath)
	if err != nil {
		t.Fatalf("Probe(image) error = %v", err)
	}
	asset.ID = "overlay-asset"
	project := Project{
		Version: 1, Name: "Image-only starter", Width: 1280, Height: 720, FPS: 30,
		Assets: []Asset{asset},
		Tracks: []Track{
			{ID: "video-1", Name: "Video 1", Kind: TrackVideo},
			{ID: "video-2", Name: "Video 2", Kind: TrackVideo},
			{ID: "audio-1", Name: "Audio 1", Kind: TrackAudio},
			{ID: "audio-2", Name: "Audio 2", Kind: TrackAudio},
			{ID: "overlay-1", Name: "Overlay 1", Kind: TrackOverlay, Clips: []Clip{{
				ID: "overlay-clip", AssetID: asset.ID, Start: 0, Offset: 0, Duration: 150,
				X: 0, Y: 0, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain,
			}}},
		},
	}
	if err := Validate(project); err != nil {
		t.Fatalf("Validate(image-only project) error = %v", err)
	}

	outputPath := filepath.Join(root, "image-only.mp4")
	if err := Render(ctx, ffmpeg, project, map[string]string{asset.ID: imagePath}, outputPath, nil); err != nil {
		t.Fatalf("Render(image-only project) error = %v", err)
	}
	rendered, err := Probe(ctx, ffmpeg, outputPath)
	if err != nil {
		t.Fatalf("Probe(rendered image-only project) error = %v", err)
	}
	if rendered.Kind != AssetVideo || rendered.Width != 1280 || rendered.Height != 720 || rendered.HasAudio || rendered.DurationFrames < 149 || rendered.DurationFrames > 152 {
		t.Fatalf("Probe(rendered image-only project) = %+v", rendered)
	}
	pixel := decodedPixel(t, ctx, ffmpeg, outputPath, "1.000000", 640, 360)
	if pixel[1] < pixel[0]*3 || pixel[1] < pixel[2]*3 {
		t.Fatalf("center frame pixel = %v, want green overlay over black canvas", pixel)
	}
}
