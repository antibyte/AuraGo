package server

import (
	"context"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/videostudio"
)

// The encoder test takes up to a few seconds per candidate; reuse the result until the configuration changes,
// ten minutes pass, or a render had to fall back to software.
var videoStudioEncoderCache struct {
	mu      sync.Mutex
	key     string
	status  videostudio.EncoderStatus
	checked time.Time
}

func videoStudioEncoder(ctx context.Context, cfg *config.Config) videostudio.EncoderStatus {
	key := cfg.VideoStudio.FFmpegPath + "\x00" + cfg.VideoStudio.HardwareEncoding + "\x00" + cfg.VideoStudio.HardwareDevice
	videoStudioEncoderCache.mu.Lock()
	defer videoStudioEncoderCache.mu.Unlock()
	if videoStudioEncoderCache.key == key && time.Since(videoStudioEncoderCache.checked) < 10*time.Minute {
		return videoStudioEncoderCache.status
	}
	status := videostudio.SelectEncoder(ctx, cfg.VideoStudio.FFmpegPath, cfg.VideoStudio.HardwareEncoding, cfg.VideoStudio.HardwareDevice)
	if ctx.Err() != nil {
		// An aborted request cut the test short; its libx264 verdict says nothing about the hardware.
		return status
	}
	videoStudioEncoderCache.key, videoStudioEncoderCache.status, videoStudioEncoderCache.checked = key, status, time.Now()
	return status
}

// invalidateVideoStudioEncoder makes the next request test the encoders again.
func invalidateVideoStudioEncoder() {
	videoStudioEncoderCache.mu.Lock()
	videoStudioEncoderCache.checked = time.Time{}
	videoStudioEncoderCache.mu.Unlock()
}

func resetVideoStudioEncoderCache() {
	videoStudioEncoderCache.mu.Lock()
	videoStudioEncoderCache.key, videoStudioEncoderCache.status, videoStudioEncoderCache.checked = "", videostudio.EncoderStatus{}, time.Time{}
	videoStudioEncoderCache.mu.Unlock()
}
