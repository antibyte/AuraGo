package server

import (
	"context"

	"aurago/internal/config"
	"aurago/internal/storymaker/render"
	"aurago/internal/videostudio"
)

// storymakerMeasureMusic measures a stored music file, so that the preview can play the bed at the level of the export. The
// export measures again for itself, so this is best effort: no FFmpeg, a failed meter or a silent file gives nil and never
// fails the job. The music panel then says that the preview's level is approximate.
func storymakerMeasureMusic(ctx context.Context, rt *storymakerRuntime, cfg *config.Config, id, file string) *float64 {
	ffmpeg, err := videostudio.ResolveFFmpeg(cfg.VideoStudio.FFmpegPath)
	if err != nil {
		return nil
	}
	r, _, err := rt.projects.OpenAudio(ctx, id, "music", file)
	if err != nil {
		return nil
	}
	defer r.Close()
	reading, err := render.MeterStored(ctx, ffmpeg, r)
	if err != nil || reading.Silent || reading.I < -100 || reading.I > 20 {
		return nil
	}
	return &reading.I
}
