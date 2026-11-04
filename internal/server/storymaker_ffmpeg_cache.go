package server

import (
	"context"
	"sync"
	"time"

	"aurago/internal/videostudio"
)

// storymakerFFmpegCacheFor is how long the status endpoint reuses an FFmpeg check, a ready one or a failed one.
const storymakerFFmpegCacheFor = 30 * time.Second

// storymakerFFmpegCache keeps the last FFmpeg check of the status endpoint (it spawns ffmpeg, 0.3 to 0.5 s). The entry is
// keyed on the configured path, so a changed path is checked at once. Rendering does not use it.
type storymakerFFmpegCache struct {
	mu     sync.Mutex
	path   string
	at     time.Time
	result videostudio.RuntimeStatus
	have   bool

	check func(context.Context, string) videostudio.RuntimeStatus // videostudio.CheckRuntime; replaced in tests
	now   func() time.Time                                        // time.Now; replaced in tests
}

func newStorymakerFFmpegCache() *storymakerFFmpegCache {
	return &storymakerFFmpegCache{check: videostudio.CheckRuntime, now: time.Now}
}

// status returns the cached check of path or runs a new one. Concurrent calls wait for the one that runs.
func (c *storymakerFFmpegCache) status(ctx context.Context, path string) videostudio.RuntimeStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.have && c.path == path && c.now().Sub(c.at) < storymakerFFmpegCacheFor {
		return c.result
	}
	result := c.check(ctx, path)
	if ctx.Err() != nil {
		return result // an aborted check says nothing about FFmpeg
	}
	c.path, c.at, c.result, c.have = path, c.now(), result, true
	return result
}
