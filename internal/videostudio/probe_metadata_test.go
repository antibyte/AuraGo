package videostudio

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestProbeReportsExactMediaMetadata(t *testing.T) {
	if got := normalizedFrameRate("60000/2002", ""); got != "30000/1001" {
		t.Fatalf("normalizedFrameRate reduced to %q", got)
	}
	if got := normalizedFrameRate("0/0", "25/1"); got != "25/1" {
		t.Fatalf("normalizedFrameRate fallback = %q", got)
	}
	if safeCodecName("H264") != "h264" || safeCodecName("bad codec") != "" {
		t.Fatal("safeCodecName accepted or rejected the wrong names")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	root := t.TempDir()
	videoPath, shortPath, audioPath := filepath.Join(root, "red.mp4"), filepath.Join(root, "short.mp4"), filepath.Join(root, "tone.wav")
	makeVideoFixture(t, ctx, ffmpeg, videoPath, "red")
	makeShortVideoLongAudioFixture(t, ctx, ffmpeg, shortPath)
	makeAudioFixture(t, ctx, ffmpeg, audioPath)

	video, err := Probe(ctx, ffmpeg, videoPath)
	if err != nil {
		t.Fatal(err)
	}
	if video.Codec != "h264" || video.SourceFPS != "30/1" || video.SizeBytes <= 0 || video.DurationUs < 950_000 || video.DurationUs > 1_050_000 || video.VideoDurationUs != video.DurationUs || video.DurationFrames != 30 {
		t.Fatalf("video metadata = %+v", video)
	}
	short, err := Probe(ctx, ffmpeg, shortPath)
	if err != nil {
		t.Fatal(err)
	}
	if short.DurationUs < 1_900_000 || short.VideoDurationUs < 900_000 || short.VideoDurationUs > 1_100_000 {
		t.Fatalf("short video with long audio = %+v", short)
	}
	audio, err := Probe(ctx, ffmpeg, audioPath)
	if err != nil {
		t.Fatal(err)
	}
	if audio.Codec != "pcm_s16le" || audio.SourceFPS != "" || audio.VideoDurationUs != 0 || audio.DurationUs != 2_000_000 || audio.DurationFrames != 60 {
		t.Fatalf("audio metadata = %+v", audio)
	}
}
