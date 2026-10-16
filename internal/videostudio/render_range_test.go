package videostudio

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestRenderRangeProducesTheRequestedDuration(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	dir := t.TempDir()
	project, paths := segmentFixtureProject(t, ctx, ffmpeg, dir)
	const start = 15
	end := min(75, EnabledDuration(project))
	output := filepath.Join(dir, "range.mp4")
	if err := RenderWithOptions(ctx, RenderOptions{FFmpegPath: ffmpeg, Project: project, AssetPaths: paths, Output: output, Start: start, End: end}); err != nil {
		t.Fatalf("range render: %v", err)
	}
	probed, err := Probe(ctx, ffmpeg, output)
	if err != nil {
		t.Fatal(err)
	}
	frames := ProjectRate(project).FramesAt(probed.DurationUs)
	if diff := frames - (end - start); diff < -1 || diff > 1 {
		t.Fatalf("range output has %d frames, want %d (±1)", frames, end-start)
	}
}
