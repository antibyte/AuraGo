package videostudio

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	maxRenderOutputBytes = int64(2 << 30)
	renderTimeout        = 4 * time.Hour
	inputThreadLimit     = 1
	threadLimit          = 4
)

type renderInput struct {
	asset Asset
	path  string
	index int
}

// Render produces a bounded H.264/AAC MP4 from private staged local assets.
// assetPaths is keyed by project asset ID and is owned by the server staging layer.
func Render(ctx context.Context, ffmpegPath string, project Project, assetPaths map[string]string, output string, progress func(float64)) error {
	if err := Validate(project); err != nil {
		return fmt.Errorf("invalid Video Studio project: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	ffmpeg, err := resolveBinary(ffmpegPath, "ffmpeg")
	if err != nil {
		return err
	}
	outputPath, err := validateRenderOutput(output)
	if err != nil {
		return err
	}
	duration := Duration(project)
	if duration <= 0 {
		return fmt.Errorf("project has no clips to render")
	}
	inputs, err := collectRenderInputs(project, assetPaths)
	if err != nil {
		return err
	}
	for _, input := range inputs {
		if filepath.Clean(input.path) == filepath.Clean(outputPath) {
			return fmt.Errorf("output must not overwrite an input asset")
		}
	}

	graph, videoLabel, audioLabel, err := buildRenderGraph(project, duration, inputs)
	if err != nil {
		return err
	}
	graphFile, err := os.CreateTemp("", "aurago-videostudio-filter-*.txt")
	if err != nil {
		return fmt.Errorf("create FFmpeg filter graph: %w", err)
	}
	graphPath := graphFile.Name()
	defer os.Remove(graphPath)
	if _, err := graphFile.WriteString(graph); err != nil {
		_ = graphFile.Close()
		return fmt.Errorf("write FFmpeg filter graph: %w", err)
	}
	if err := graphFile.Close(); err != nil {
		return fmt.Errorf("close FFmpeg filter graph: %w", err)
	}

	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error"}
	for _, input := range inputs {
		args = append(args, "-protocol_whitelist", allowedProtocols, "-format_whitelist", allowedDemuxers, "-threads", strconv.Itoa(inputThreadLimit))
		if input.asset.Kind == AssetImage {
			args = append(args, "-stream_loop", "-1", "-f", "image2", "-pattern_type", "none", "-framerate", "30")
		}
		args = append(args, "-i", input.path)
	}
	args = append(args,
		"-filter_threads", strconv.Itoa(threadLimit),
		"-filter_complex_threads", strconv.Itoa(threadLimit),
		"-filter_complex_script", graphPath,
		"-map", "["+videoLabel+"]",
		"-map_metadata", "-1",
		"-t", seconds(duration),
		"-r", "30", "-c:v", "libx264", "-preset", "veryfast", "-crf", "23",
		"-threads", strconv.Itoa(threadLimit), "-pix_fmt", "yuv420p",
	)
	if audioLabel != "" {
		args = append(args, "-map", "["+audioLabel+"]", "-c:a", "aac", "-b:a", "192k", "-ar", "48000", "-ac", "2")
	} else {
		args = append(args, "-an")
	}
	args = append(args,
		"-movflags", "+faststart", "-max_muxing_queue_size", "512",
		"-fs", strconv.FormatInt(maxRenderOutputBytes, 10),
		"-progress", "pipe:1", "-nostats", "-f", "mp4", outputPath,
	)

	renderCtx, cancel := context.WithTimeout(ctx, renderTimeout)
	defer cancel()
	cmd := exec.CommandContext(renderCtx, ffmpeg, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("open FFmpeg progress stream: %w", err)
	}
	var stderr cappedBuffer
	stderr.limit = 1 << 20
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start FFmpeg render: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 4096), 64<<10)
	lastProgress := 0.0
	for scanner.Scan() {
		if progress == nil {
			continue
		}
		value, ok := renderProgress(scanner.Text(), duration)
		if ok && value > lastProgress {
			lastProgress = value
			progress(value)
		}
	}
	scanErr := scanner.Err()
	if scanErr != nil {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}
	waitErr := cmd.Wait()
	if err := renderCtx.Err(); err != nil {
		_ = os.Remove(outputPath)
		return err
	}
	if scanErr != nil {
		_ = os.Remove(outputPath)
		return fmt.Errorf("read FFmpeg progress: %w", scanErr)
	}
	if waitErr != nil {
		_ = os.Remove(outputPath)
		message := strings.TrimSpace(stderr.String())
		message = scrubPaths(message, outputPath, inputs)
		if message == "" {
			return fmt.Errorf("FFmpeg render failed: %w", waitErr)
		}
		return fmt.Errorf("FFmpeg render failed: %s", message)
	}
	if err := ensureOutputSize(outputPath, maxRenderOutputBytes); err != nil {
		_ = os.Remove(outputPath)
		return err
	}
	verified, err := Probe(ctx, ffmpeg, outputPath)
	if err != nil {
		_ = os.Remove(outputPath)
		return fmt.Errorf("verify rendered output: %w", err)
	}
	if verified.Kind != AssetVideo || verified.Width != project.Width || verified.Height != project.Height || verified.DurationFrames < duration-1 || verified.DurationFrames > duration+2 || (audioLabel != "") != verified.HasAudio {
		_ = os.Remove(outputPath)
		return fmt.Errorf("rendered output metadata does not match the project timeline")
	}
	if progress != nil {
		progress(1)
	}
	return nil
}

func collectRenderInputs(project Project, assetPaths map[string]string) ([]renderInput, error) {
	used := make(map[string]struct{})
	assets := make(map[string]Asset, len(project.Assets))
	for _, asset := range project.Assets {
		assets[asset.ID] = asset
	}
	for _, track := range project.Tracks {
		for _, clip := range track.Clips {
			visualUsed := track.Kind != TrackAudio && !track.Hidden
			audioUsed := !track.Hidden && !track.Muted && clip.Volume > 0 &&
				((track.Kind == TrackAudio) || (track.Kind == TrackVideo && assets[clip.AssetID].HasAudio))
			if visualUsed || audioUsed {
				used[clip.AssetID] = struct{}{}
			}
		}
	}
	inputs := make([]renderInput, 0, len(used))
	for _, asset := range project.Assets {
		if _, ok := used[asset.ID]; !ok {
			continue
		}
		stagedPath, ok := assetPaths[asset.ID]
		if !ok {
			return nil, fmt.Errorf("staged path is missing for asset %q", asset.ID)
		}
		path, err := ensureRegularLocalFile(stagedPath)
		if err != nil {
			return nil, fmt.Errorf("asset %q is not a private staged file: %w", asset.ID, err)
		}
		inputs = append(inputs, renderInput{asset: asset, path: path, index: len(inputs)})
	}
	return inputs, nil
}

func buildRenderGraph(project Project, duration int, inputs []renderInput) (string, string, string, error) {
	inputByAsset := make(map[string]renderInput, len(inputs))
	for _, input := range inputs {
		inputByAsset[input.asset.ID] = input
	}
	filters := make([]string, 0, len(project.Tracks)*4+len(project.Assets)*6)
	width, height := project.Width, project.Height
	durationSeconds := seconds(duration)
	filters = append(filters,
		fmt.Sprintf("color=c=black:s=%dx%d:r=30:d=%s,trim=end_frame=%d,setpts=PTS-STARTPTS,format=rgba[vs_base0]", width, height, durationSeconds, duration),
	)
	videoOutput := "vs_base0"
	visualTrackIndex := 0
	for _, track := range project.Tracks {
		if track.Kind == TrackAudio || track.Hidden || len(track.Clips) == 0 {
			continue
		}
		trackLabel, trackFilters, err := buildVisualTrack(track, visualTrackIndex, width, height, inputByAsset)
		if err != nil {
			return "", "", "", err
		}
		filters = append(filters, trackFilters...)
		out := fmt.Sprintf("vs_base%d", visualTrackIndex+1)
		filters = append(filters, fmt.Sprintf("[%s][%s]overlay=eof_action=pass:repeatlast=0:shortest=0:format=auto[%s]", videoOutput, trackLabel, out))
		videoOutput = out
		visualTrackIndex++
	}

	var audioLabels []string
	for trackIndex, track := range project.Tracks {
		if track.Muted || track.Hidden || len(track.Clips) == 0 {
			continue
		}
		for clipIndex, clip := range track.Clips {
			asset := inputByAsset[clip.AssetID].asset
			if clip.Volume <= 0 || !asset.HasAudio || track.Kind == TrackOverlay || (track.Kind == TrackAudio && asset.Kind != AssetAudio && asset.Kind != AssetVideo) {
				continue
			}
			input, ok := inputByAsset[clip.AssetID]
			if !ok {
				continue
			}
			label := fmt.Sprintf("vs_audio_%d_%d", trackIndex, clipIndex)
			filters = append(filters, buildAudioClipFilter(input, clip, label))
			audioLabels = append(audioLabels, "["+label+"]")
		}
	}
	if len(audioLabels) == 0 {
		return strings.Join(filters, ";\n"), videoOutput, "", nil
	}
	projectDuration := seconds(duration)
	filters = append(filters, fmt.Sprintf("%samix=inputs=%d:duration=longest:dropout_transition=0:normalize=0,alimiter=limit=0.95,apad=whole_dur=%s,atrim=duration=%s,asetpts=PTS-STARTPTS[vs_audio_out]", strings.Join(audioLabels, ""), len(audioLabels), projectDuration, projectDuration))
	return strings.Join(filters, ";\n"), videoOutput, "vs_audio_out", nil
}

func buildVisualTrack(track Track, trackIndex, canvasWidth, canvasHeight int, inputs map[string]renderInput) (string, []string, error) {
	clips := append([]Clip(nil), track.Clips...)
	sort.Slice(clips, func(i, j int) bool {
		if clips[i].Start == clips[j].Start {
			return clips[i].ID < clips[j].ID
		}
		return clips[i].Start < clips[j].Start
	})
	filters := make([]string, 0, len(clips)*4)
	sequence := 0
	newJoinLabel := func() string {
		label := fmt.Sprintf("vs_join_%d_%d", trackIndex, sequence)
		sequence++
		return label
	}
	clipLabels := make([]string, len(clips))
	for clipIndex, clip := range clips {
		input, ok := inputs[clip.AssetID]
		if !ok {
			return "", nil, fmt.Errorf("staged path is missing for visual asset %q", clip.AssetID)
		}
		label := fmt.Sprintf("vs_clip_%d_%d", trackIndex, clipIndex)
		clipLabels[clipIndex] = label
		filters = append(filters, buildVisualClipFilter(trackIndex, clipIndex, clip, input, canvasWidth, canvasHeight, label))
	}
	combined := clipLabels[0]
	combinedEnd := clips[0].Start + clips[0].Duration
	if clips[0].Start > 0 {
		gap := gapLabel(trackIndex, 0)
		filters = append(filters, transparentGapFilter(gap, clips[0].Start, canvasWidth, canvasHeight))
		combined = newJoinLabel()
		filters = append(filters, fmt.Sprintf("[%s][%s]concat=n=2:v=1:a=0[%s]", gapLabel(trackIndex, 0), clipLabels[0], combined))
	}
	for i := 1; i < len(clips); i++ {
		clip, previous := clips[i], clips[i-1]
		if previous.Transition != nil {
			transition := mapTransition(previous.Transition.Type)
			out := newJoinLabel()
			filters = append(filters, fmt.Sprintf("[%s][%s]xfade=transition=%s:duration=%s:offset=%s,format=rgba[%s]", combined, clipLabels[i], transition, seconds(previous.Transition.Duration), seconds(clip.Start), out))
			combined = out
			combinedEnd = clip.Start + clip.Duration
			continue
		}
		if clip.Start > combinedEnd {
			gap := gapLabel(trackIndex, i)
			filters = append(filters, transparentGapFilter(gap, clip.Start-combinedEnd, canvasWidth, canvasHeight))
			withGap := newJoinLabel()
			filters = append(filters, fmt.Sprintf("[%s][%s]concat=n=2:v=1:a=0[%s]", combined, gap, withGap))
			combined = withGap
		}
		out := newJoinLabel()
		filters = append(filters, fmt.Sprintf("[%s][%s]concat=n=2:v=1:a=0[%s]", combined, clipLabels[i], out))
		combined = out
		combinedEnd = clip.Start + clip.Duration
	}
	return combined, filters, nil
}

func buildVisualClipFilter(trackIndex, clipIndex int, clip Clip, input renderInput, canvasWidth, canvasHeight int, output string) string {
	boxWidth := evenPixels(clip.Width*float64(canvasWidth), canvasWidth)
	boxHeight := evenPixels(clip.Height*float64(canvasHeight), canvasHeight)
	x := evenPosition(clip.X*float64(canvasWidth), boxWidth, canvasWidth)
	y := evenPosition(clip.Y*float64(canvasHeight), boxHeight, canvasHeight)
	scale := fmt.Sprintf("scale=w=%d:h=%d:force_original_aspect_ratio=decrease:force_divisible_by=2,pad=w=%d:h=%d:x=(ow-iw)/2:y=(oh-ih)/2:color=black@0", boxWidth, boxHeight, boxWidth, boxHeight)
	if clip.Fit == FitCover {
		scale = fmt.Sprintf("scale=w=%d:h=%d:force_original_aspect_ratio=increase:force_divisible_by=2,crop=w=%d:h=%d", boxWidth, boxHeight, boxWidth, boxHeight)
	}
	rotation := ""
	if math.Abs(clip.Rotation) > 0.000001 {
		angle := fmt.Sprintf("(%.6f*PI/180)", clip.Rotation)
		rotation = fmt.Sprintf(",rotate=%s:ow=rotw(%s):oh=roth(%s):c=black@0", angle, angle, angle)
	}
	// Bound pre-offset PTS and clone padding to a small multiple of this clip's duration.
	offsetPTS := fmt.Sprintf("setpts='max(PTS-%d,-%d)'", clip.Offset, clip.Duration)
	source := fmt.Sprintf(
		"[%d:V:0]setpts=PTS-STARTPTS,fps=30,%s,tpad=stop_mode=clone:stop=%d,trim=start_pts=0:end_pts=%d,setpts=PTS-STARTPTS,%s,format=rgba%s",
		input.index, offsetPTS, clip.Duration*2, clip.Duration, scale, rotation,
	)
	if clip.Opacity < 0.999999 {
		source += fmt.Sprintf(",colorchannelmixer=aa=%.6f", clip.Opacity)
	}
	layer := fmt.Sprintf("vs_layer_%d_%d", trackIndex, clipIndex)
	background := fmt.Sprintf("vs_clip_bg_%d_%d", trackIndex, clipIndex)
	positionX, positionY := fmt.Sprintf("%d", x), fmt.Sprintf("%d", y)
	if math.Abs(clip.Rotation) > 0.000001 {
		positionX = fmt.Sprintf("'%d+%d/2-overlay_w/2'", x, boxWidth)
		positionY = fmt.Sprintf("'%d+%d/2-overlay_h/2'", y, boxHeight)
	}
	return fmt.Sprintf("%s[%s];color=c=black@0:s=%dx%d:r=30:d=%s,trim=end_frame=%d,setpts=PTS-STARTPTS,format=rgba[%s];[%s][%s]overlay=x=%s:y=%s:format=auto:shortest=1[%s]", source, layer, canvasWidth, canvasHeight, seconds(clip.Duration), clip.Duration, background, background, layer, positionX, positionY, output)
}

func buildAudioClipFilter(input renderInput, clip Clip, output string) string {
	start := seconds(clip.Offset)
	duration := seconds(clip.Duration)
	filter := fmt.Sprintf("[%d:a:0]atrim=start=%s:duration=%s,asetpts=PTS-STARTPTS,aresample=48000,aformat=sample_fmts=fltp:channel_layouts=stereo,volume=%.6f", input.index, start, duration, clip.Volume)
	if clip.FadeIn > 0 {
		filter += fmt.Sprintf(",afade=t=in:st=0:d=%s", seconds(clip.FadeIn))
	}
	if clip.FadeOut > 0 {
		filter += fmt.Sprintf(",afade=t=out:st=%s:d=%s", seconds(clip.Duration-clip.FadeOut), seconds(clip.FadeOut))
	}
	filter += fmt.Sprintf(",adelay=%dS:all=1[%s]", clip.Start*(48000/FramesPerSecond), output)
	return filter
}

func transparentGapFilter(label string, frames, width, height int) string {
	return fmt.Sprintf("color=c=black@0:s=%dx%d:r=30:d=%s,trim=end_frame=%d,setpts=PTS-STARTPTS,format=rgba[%s]", width, height, seconds(frames), frames, label)
}

func mapTransition(value TransitionType) string {
	switch value {
	case TransitionBlack:
		return "fadeblack"
	case TransitionWipeLeft:
		return "wiperight"
	case TransitionWipeRight:
		return "wipeleft"
	default:
		return "fade"
	}
}

func seconds(frames int) string {
	return fmt.Sprintf("%.6f", float64(frames)/FramesPerSecond)
}

func evenPixels(value float64, bound int) int {
	pixels := int(math.Round(value/2) * 2)
	if pixels < 2 {
		return 2
	}
	if pixels > bound {
		return bound
	}
	return pixels
}

func evenPosition(value float64, box, bound int) int {
	position := int(math.Round(value/2) * 2)
	maxPosition := bound - box
	if position < 0 {
		return 0
	}
	if position > maxPosition {
		position = maxPosition
	}
	return position
}

func gapLabel(trackIndex, sequence int) string {
	return fmt.Sprintf("vs_gap_%d_%d", trackIndex, sequence)
}

func validateRenderOutput(output string) (string, error) {
	path, err := validateNewOutput(output, "")
	if err != nil {
		return "", err
	}
	return path, nil
}

func renderProgress(line string, durationFrames int) (float64, bool) {
	key, value, ok := strings.Cut(line, "=")
	if !ok {
		return 0, false
	}
	var elapsed float64
	switch key {
	case "out_time_us":
		micros, err := strconv.ParseFloat(value, 64)
		if err != nil || micros < 0 {
			return 0, false
		}
		elapsed = micros / 1e6
	case "out_time":
		parsed, err := time.Parse("15:04:05.999999", value)
		if err != nil {
			return 0, false
		}
		elapsed = float64(parsed.Hour()*3600+parsed.Minute()*60+parsed.Second()) + float64(parsed.Nanosecond())/1e9
	default:
		return 0, false
	}
	return math.Min(1, math.Max(0, elapsed/(float64(durationFrames)/FramesPerSecond))), true
}

func scrubPaths(message, output string, inputs []renderInput) string {
	message = strings.ReplaceAll(message, output, "<output>")
	for _, input := range inputs {
		message = strings.ReplaceAll(message, input.path, "<staged-asset>")
	}
	return message
}
