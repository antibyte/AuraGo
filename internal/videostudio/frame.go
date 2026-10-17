package videostudio

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	frameTimeout     = 2 * time.Minute
	maxFramePNGBytes = int64(1 << 30)
)

// FrameOptions selects one timeline frame for RenderFrame.
type FrameOptions struct {
	FFmpegPath string
	Project    Project
	// AssetPaths maps project asset IDs to private staged files, as for RenderOptions.
	AssetPaths map[string]string
	Frame      int // timeline frame
	Width      int // 0 keeps the project size (with Height)
	Height     int
	Output     string // new .png path
}

// RenderFrame writes one timeline frame as an 8-bit RGB PNG through the export's segment graph
// (buildVideoSegmentWith with videoTail{rgb: true}), before any YUV/4:2:0 conversion. Golden-frame tests compare it
// with the browser preview. A frame inside a transition composes the whole transition window (expandRange) and keeps
// only that frame, exactly as a video segment does.
func RenderFrame(ctx context.Context, opts FrameOptions) error {
	project := opts.Project
	if err := Validate(project); err != nil {
		return fmt.Errorf("invalid Video Studio project: %w", err)
	}
	duration := EnabledDuration(project)
	if opts.Frame < 0 || opts.Frame >= duration {
		return fmt.Errorf("frame %d is outside the timeline (0–%d)", opts.Frame, duration)
	}
	if err := checkSourceAspects(project, opts.Frame, opts.Frame+1); err != nil {
		return err
	}
	width, height := opts.Width, opts.Height
	if width == 0 && height == 0 {
		width, height = project.Width, project.Height
	}
	if width < 2 || height < 2 || width%2 != 0 || height%2 != 0 || width > MaxSourceDimension || height > MaxSourceDimension {
		return fmt.Errorf("frame size %dx%d is not supported", width, height)
	}
	outputPath, err := validateNewOutput(opts.Output, "")
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Ext(outputPath), ".png") {
		return fmt.Errorf("frame output must be a .png file")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	ffmpeg, err := resolveBinary(opts.FFmpegPath, "ffmpeg")
	if err != nil {
		return err
	}
	frameCtx, cancel := context.WithTimeout(ctx, frameTimeout)
	defer cancel()
	sources, err := collectRenderSources(frameCtx, ffmpeg, project, opts.AssetPaths)
	if err != nil {
		return err
	}
	for _, source := range sources {
		if filepath.Clean(source.path) == filepath.Clean(outputPath) {
			return fmt.Errorf("output must not overwrite an input asset")
		}
	}
	rate := ProjectRate(project)
	inputs, graph, commands, err := buildVideoSegmentWith(project, Segment{Start: opts.Frame, End: opts.Frame + 1}, sources, rate, videoTail{width: width, height: height, rgb: true})
	if err != nil {
		return err
	}
	if len(inputs) > MaxSegmentInputs {
		return fmt.Errorf("%w at frame %d", ErrTooManyInputs, opts.Frame)
	}
	workDir, graphPath, err := stageVideoGraph(filepath.Dir(outputPath), graph, rate, commands)
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)
	// -copyts -start_at_zero as for video segments: seeked inputs keep the source's own timeline.
	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error", "-copyts", "-start_at_zero"}
	args = append(args, segmentInputArgs(inputs, rate)...)
	args = append(args,
		"-filter_threads", strconv.Itoa(threadLimit), "-filter_complex_threads", strconv.Itoa(threadLimit),
		"-filter_complex_script", graphPath, "-map", "[vs_video_out]", "-map_metadata", "-1",
		"-frames:v", "1", "-c:v", "png", "-pix_fmt", "rgb24", "-update", "1", "-f", "image2", outputPath)
	if err := runFFmpegIn(frameCtx, workDir, ffmpeg, args, rate.Seconds(1), nil, inputPaths(inputs), outputPath); err != nil {
		return err
	}
	return ensureOutputSize(outputPath, maxFramePNGBytes)
}
