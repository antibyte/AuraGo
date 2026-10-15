package server

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestVideoStudioEncoderCacheFollowsConfig(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg required")
	}
	resetVideoStudioEncoderCache()
	t.Cleanup(resetVideoStudioEncoderCache)
	cfg := &config.Config{}
	cfg.VideoStudio.FFmpegPath = ffmpeg
	cfg.VideoStudio.HardwareEncoding = "off"
	first := videoStudioEncoder(context.Background(), cfg)
	if first.Name != "libx264" || first.Hardware {
		t.Fatalf("off = %+v", first)
	}
	cfg.VideoStudio.HardwareEncoding = "nvenc"
	cfg.VideoStudio.FFmpegPath = ffmpeg + "-missing"
	if second := videoStudioEncoder(context.Background(), cfg); second.Name != "libx264" || second.Note == "" {
		t.Fatalf("a changed configuration reused the cached encoder: %+v", second)
	}
	invalidateVideoStudioEncoder()
	cfg.VideoStudio.FFmpegPath, cfg.VideoStudio.HardwareEncoding = ffmpeg, "off"
	if third := videoStudioEncoder(context.Background(), cfg); third.Name != "libx264" || third.Note != "" {
		t.Fatalf("after invalidation = %+v", third)
	}

	// A selection cut short by an aborted request is not cached.
	resetVideoStudioEncoderCache()
	cfg.VideoStudio.HardwareEncoding = "nvenc"
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if aborted := videoStudioEncoder(canceled, cfg); aborted.Hardware || !strings.Contains(aborted.Note, "context canceled") {
		t.Fatalf("aborted selection = %+v", aborted)
	}
	if retried := videoStudioEncoder(context.Background(), cfg); strings.Contains(retried.Note, "context canceled") {
		t.Fatalf("an aborted selection was cached: %+v", retried)
	}
}
