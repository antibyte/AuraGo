package videostudio

import (
	"context"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWritePosterFitsLongestSideAndSkipsAudio(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	dir := t.TempDir()
	video := filepath.Join(dir, "clip.mp4")
	makeVideoFixture(t, ctx, ffmpeg, video, "blue")
	asset, err := Probe(ctx, ffmpeg, video)
	if err != nil {
		t.Fatal(err)
	}
	poster := filepath.Join(dir, "poster.jpg")
	if err := WritePoster(ctx, ffmpeg, video, poster, asset); err != nil {
		t.Fatalf("poster: %v", err)
	}
	file, err := os.Open(poster)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	cfg, err := jpeg.DecodeConfig(file)
	if err != nil {
		t.Fatalf("poster is not a JPEG: %v", err)
	}
	if max(cfg.Width, cfg.Height) != PosterLongestSide {
		t.Fatalf("poster is %dx%d, want a longest side of %d", cfg.Width, cfg.Height, PosterLongestSide)
	}
	still := filepath.Join(dir, "still.png")
	makeImageFixture(t, ctx, ffmpeg, still)
	image, err := Probe(ctx, ffmpeg, still)
	if err != nil {
		t.Fatal(err)
	}
	if err := WritePoster(ctx, ffmpeg, still, filepath.Join(dir, "still-poster.jpg"), image); err != nil {
		t.Fatalf("image poster: %v", err)
	}
	tone := filepath.Join(dir, "tone.m4a")
	makeAudioFixture(t, ctx, ffmpeg, tone)
	audio, err := Probe(ctx, ffmpeg, tone)
	if err != nil {
		t.Fatal(err)
	}
	if err := WritePoster(ctx, ffmpeg, tone, filepath.Join(dir, "audio.jpg"), audio); err == nil {
		t.Fatal("audio assets must not get a poster")
	}
}
