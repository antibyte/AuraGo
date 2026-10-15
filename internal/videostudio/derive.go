package videostudio

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
)

// MaxReverseSeconds bounds the source range of one reverse job.
const MaxReverseSeconds = 300

// ErrReverseRangeTooLong reports a reverse range longer than MaxReverseSeconds.
var ErrReverseRangeTooLong = errors.New("reverse range is longer than the reverse limit")

// ReverseOptions describes one reverse job. Offset and Frames select the source range in project frames.
type ReverseOptions struct {
	FFmpegPath     string
	Input          string // private staged file
	Output         string
	Offset         int
	Frames         int
	Rate           Rate
	Width, Height  int // source size; it picks the chunk length
	Encoder        Encoder
	MaxOutputBytes int64
	Progress       func(float64)
}

// MaxReverseFrames is the longest source range one reverse job accepts, in project frames.
func MaxReverseFrames(rate Rate) int {
	return int(int64(MaxReverseSeconds) * rate.Num / rate.Den)
}

// ReverseChunkFrames is the length of one reversed chunk: one second at 1440p and above, two seconds below. Each chunk
// is reversed in memory, so a larger picture gets shorter chunks.
func ReverseChunkFrames(rate Rate, width, height int) int {
	seconds := int64(2)
	if int64(width)*int64(height) >= 2560*1440 {
		seconds = 1
	}
	return max(1, int(seconds*rate.Num/rate.Den))
}

// ReverseClip writes the source range backwards as an H.264/AAC MP4. A failed hardware encoder retries the whole job
// with libx264, as previews do.
func ReverseClip(ctx context.Context, opts ReverseOptions) error {
	opts.Progress = monotonicProgress(opts.Progress)
	err := reverseOnce(ctx, opts)
	if err == nil || !opts.Encoder.Hardware() || ctx.Err() != nil || errors.Is(err, ErrReverseRangeTooLong) {
		return err
	}
	opts.Encoder = Encoder{Quality: opts.Encoder.Quality}
	return reverseOnce(ctx, opts)
}

func reverseOnce(ctx context.Context, opts ReverseOptions) error {
	if opts.Offset < 0 || opts.Frames < 1 {
		return fmt.Errorf("reverse range is empty")
	}
	if opts.Frames > MaxReverseFrames(opts.Rate) {
		return ErrReverseRangeTooLong
	}
	ffmpeg, err := resolveBinary(opts.FFmpegPath, "ffmpeg")
	if err != nil {
		return err
	}
	inputPath, err := ensureRegularLocalFile(opts.Input)
	if err != nil {
		return fmt.Errorf("open reverse source: %w", err)
	}
	outputPath, err := validateNewOutput(opts.Output, inputPath)
	if err != nil {
		return err
	}
	source, err := Probe(ctx, ffmpeg, inputPath)
	if err != nil {
		return fmt.Errorf("inspect reverse source: %w", err)
	}
	if source.Kind != AssetVideo {
		return fmt.Errorf("reverse source must contain video")
	}
	rate := opts.Rate
	maxOutput := opts.MaxOutputBytes
	if maxOutput <= 0 {
		maxOutput = defaultMaxRenderOutputBytes
	}
	workDir, err := os.MkdirTemp(filepath.Dir(outputPath), "reverse-")
	if err != nil {
		return fmt.Errorf("create reverse work directory: %w", err)
	}
	defer os.RemoveAll(workDir)

	// Source chunks run forwards. Playback starts with the last chunk and every chunk is reversed on its own, so the
	// output segments are the chunks' positions in playback order.
	chunk := ReverseChunkFrames(rate, opts.Width, opts.Height)
	var sourceChunks []Segment
	for start := 0; start < opts.Frames; start += chunk {
		sourceChunks = append(sourceChunks, Segment{Start: start, End: min(opts.Frames, start+chunk)})
	}
	last := len(sourceChunks) - 1
	playback := make([]Segment, len(sourceChunks))
	position := 0
	for k := range sourceChunks {
		length := sourceChunks[last-k].End - sourceChunks[last-k].Start
		playback[k] = Segment{Start: position, End: position + length}
		position += length
	}
	total := float64(position)
	report := opts.Progress
	videoNames := make([]string, len(playback))
	var audioNames []string
	var audioSegments []Segment
	for k, seg := range playback {
		chunkSource := sourceChunks[last-k]
		videoNames[k] = fmt.Sprintf("v%04d.ts", k)
		base, span := float64(seg.Start), float64(seg.End-seg.Start)
		if err := renderReverseVideo(ctx, ffmpeg, inputPath, opts, chunkSource, filepath.Join(workDir, videoNames[k]), func(p float64) {
			report(0.9 * (base + p*span) / total)
		}); err != nil {
			return reverseErr(ctx, err)
		}
		if source.HasAudio {
			name := fmt.Sprintf("a%04d.wav", k)
			if err := renderReverseAudio(ctx, ffmpeg, inputPath, opts, chunkSource, filepath.Join(workDir, name), nil); err != nil {
				return reverseErr(ctx, err)
			}
			audioNames = append(audioNames, name)
			audioSegments = append(audioSegments, seg)
		}
	}
	if err := muxSegments(ctx, ffmpeg, workDir, videoNames, playback, audioNames, audioSegments, rate, outputPath, maxOutput, func(p float64) {
		report(0.9 + 0.1*p)
	}); err != nil {
		_ = os.Remove(outputPath)
		return reverseErr(ctx, err)
	}
	if info, err := os.Stat(outputPath); err == nil && info.Size() >= maxOutput {
		_ = os.Remove(outputPath)
		return fmt.Errorf("%w of %d bytes", ErrOutputLimit, maxOutput)
	}
	if err := ensureOutputSize(outputPath, maxOutput); err != nil {
		_ = os.Remove(outputPath)
		return err
	}
	verified, err := Probe(ctx, ffmpeg, outputPath)
	if err != nil {
		_ = os.Remove(outputPath)
		return fmt.Errorf("verify reversed output: %w", err)
	}
	expectedUs := rate.Seconds(opts.Frames) * 1e6
	tolerance := math.Max(2*1e6*float64(rate.Den)/float64(rate.Num), 50_000)
	if verified.Kind != AssetVideo || math.Abs(float64(verified.DurationUs)-expectedUs) > tolerance {
		_ = os.Remove(outputPath)
		return fmt.Errorf("reversed output length does not match the requested range")
	}
	report(1)
	return nil
}

// reverseErr prefers the context's error: a cancelled job reports cancellation, not the FFmpeg it stopped.
func reverseErr(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return err
}

// reverseSeekFrame starts a seeked input one second early (as segmentLayers does), never before the source.
func reverseSeekFrame(first int, rate Rate) int {
	return max(0, first-int((rate.Num+rate.Den-1)/rate.Den))
}

// renderReverseVideo writes one source chunk, reversed, as MPEG-TS. Frames are picked by their absolute source
// position, padded so the chunk always has its full length, trimmed to the chunk and then reversed.
func renderReverseVideo(ctx context.Context, ffmpeg, input string, opts ReverseOptions, chunk Segment, output string, progress func(float64)) error {
	rate := opts.Rate
	first, last := opts.Offset+chunk.Start, opts.Offset+chunk.End
	frames := chunk.End - chunk.Start
	seekFrame := reverseSeekFrame(first, rate)
	inputs := []segmentInput{{path: input, seek: rate.Seconds(seekFrame), limit: rate.SecondsFloat(float64(last-seekFrame)) + 1}}
	graph := fmt.Sprintf("[0:V:0]fps=%s,tpad=stop_mode=clone:stop=%d,trim=start_pts=%d:end_pts=%d,setpts=PTS-STARTPTS,reverse,setpts=N/FRAME_RATE/TB,format=yuv420p%s[vs_video_out]",
		rate, frames, first, last, opts.Encoder.filterTail())
	graphPath, err := writeGraph(filepath.Dir(output), graph)
	if err != nil {
		return err
	}
	defer os.Remove(graphPath)
	args := append([]string{"-hide_banner", "-nostdin", "-loglevel", "error", "-copyts", "-start_at_zero"}, opts.Encoder.globalArgs()...)
	args = append(args, segmentInputArgs(inputs, rate)...)
	args = append(args,
		"-filter_threads", strconv.Itoa(threadLimit), "-filter_complex_threads", strconv.Itoa(threadLimit),
		"-filter_complex_script", graphPath, "-map", "[vs_video_out]", "-map_metadata", "-1",
		"-frames:v", strconv.Itoa(frames), "-r", rate.String())
	args = append(args, reverseVideoArgs(opts.Encoder, rate)...)
	args = append(args, "-threads", strconv.Itoa(threadLimit), "-an", "-progress", "pipe:1", "-nostats", "-f", "mpegts", output)
	return runFFmpeg(ctx, ffmpeg, args, rate.Seconds(frames), progress, inputPaths(inputs), output)
}

// reverseVideoArgs are the encoder's arguments without its BT.709 colour tags. The renderer converts every frame to
// BT.709 before it tags the stream; a reversed chunk keeps the source's pixels, so it keeps the source's colour
// properties too (FFmpeg copies them from the frames). With the tags, FFmpeg 7.1 and later would convert the
// pixels of an untagged or BT.601 source to match them, and the reversed clip would no longer look like its source.
func reverseVideoArgs(encoder Encoder, rate Rate) []string {
	args := encoder.videoArgs(rate)
	kept := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-colorspace", "-color_primaries", "-color_trc":
			i++ // and its value
			continue
		}
		kept = append(kept, args[i])
	}
	return kept
}

// renderReverseAudio writes one source chunk's sound, reversed, as float PCM with the chunk's exact sample count.
func renderReverseAudio(ctx context.Context, ffmpeg, input string, opts ReverseOptions, chunk Segment, output string, progress func(float64)) error {
	rate := opts.Rate
	first, last := opts.Offset+chunk.Start, opts.Offset+chunk.End
	seekFrame := reverseSeekFrame(first, rate)
	inputs := []segmentInput{{path: input, audioOnly: true, seek: rate.Seconds(seekFrame), limit: rate.SecondsFloat(float64(last-seekFrame)) + 1}}
	base := rate.SampleAt(seekFrame)
	start, end := rate.SampleAt(first)-base, rate.SampleAt(last)-base
	graph := fmt.Sprintf("[0:a:0]aresample=%d,asetpts=PTS-STARTPTS,apad=whole_len=%d,atrim=start_sample=%d:end_sample=%d,asetpts=PTS-STARTPTS,areverse,aformat=sample_fmts=fltp:channel_layouts=stereo[vs_audio_out]",
		AudioSampleRate, end, start, end)
	graphPath, err := writeGraph(filepath.Dir(output), graph)
	if err != nil {
		return err
	}
	defer os.Remove(graphPath)
	args := append([]string{"-hide_banner", "-nostdin", "-loglevel", "error"}, segmentInputArgs(inputs, rate)...)
	args = append(args, "-filter_complex_threads", strconv.Itoa(threadLimit), "-filter_complex_script", graphPath,
		"-map", "[vs_audio_out]", "-map_metadata", "-1", "-c:a", "pcm_f32le", "-ar", strconv.Itoa(AudioSampleRate), "-ac", "2",
		"-progress", "pipe:1", "-nostats", "-f", "wav", output)
	return runFFmpeg(ctx, ffmpeg, args, rate.Seconds(last-first), progress, inputPaths(inputs), output)
}

// ExtractStill writes the source frame that the renderer shows at project frame frame as a PNG. The frame is picked
// with the renderer's sampling (fps on absolute source positions), and colours use the BT.709 matrix the renderer
// assumes for HD sources.
func ExtractStill(ctx context.Context, ffmpegPath, input, output string, frame int, rate Rate) error {
	if frame < 0 {
		return fmt.Errorf("still frame must not be negative")
	}
	ffmpeg, err := resolveBinary(ffmpegPath, "ffmpeg")
	if err != nil {
		return err
	}
	inputPath, err := ensureRegularLocalFile(input)
	if err != nil {
		return fmt.Errorf("open still source: %w", err)
	}
	outputPath, err := validateNewOutput(output, inputPath)
	if err != nil {
		return err
	}
	seekFrame := reverseSeekFrame(frame, rate)
	filter := fmt.Sprintf("fps=%s,trim=start_pts=%d:end_pts=%d,setpts=N/FRAME_RATE/TB,scale=in_color_matrix=bt709:out_range=pc,format=rgb24", rate, frame, frame+1)
	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error", "-copyts", "-start_at_zero",
		"-protocol_whitelist", allowedProtocols, "-format_whitelist", allowedDemuxers,
		"-ss", fmt.Sprintf("%.6f", rate.Seconds(seekFrame)), "-t", rate.SecondsArg(frame - seekFrame + 1),
		"-i", inputPath, "-map", "0:V:0", "-vf", filter, "-frames:v", "1", "-an", "-c:v", "png", "-f", "image2", outputPath}
	if _, err := runSmallCommand(ctx, ffmpeg, args, 1<<20); err != nil {
		_ = os.Remove(outputPath)
		return fmt.Errorf("capture still frame: %w", err)
	}
	info, err := os.Stat(outputPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 {
		_ = os.Remove(outputPath)
		return fmt.Errorf("the source has no frame %d", frame)
	}
	return nil
}
