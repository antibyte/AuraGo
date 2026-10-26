package videostudio

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"aurago/internal/fileutil"
)

const (
	defaultMaxRenderOutputBytes = int64(2 << 30)
	renderTimeout               = 4 * time.Hour
	inputThreadLimit            = 1
	threadLimit                 = 4
	defaultSegmentSeconds       = 10
	intermediateDemuxers        = "concat,mpegts,wav"
)

var (
	// ErrInsufficientDiskSpace reports that the staging volume cannot hold a render's intermediate files.
	ErrInsufficientDiskSpace = errors.New("insufficient disk space for rendering")
	// ErrTooManyInputs reports a span that needs more simultaneous inputs than one FFmpeg process may open.
	ErrTooManyInputs = errors.New("too many simultaneous clips for one render segment")
	// ErrTooManyLUTs reports a span that needs more simultaneous lut effects than one FFmpeg process may load
	// (MaxSegmentLUTInstances). It wraps ErrTooManyInputs, so errors.Is with either holds (plan 2f, F4).
	ErrTooManyLUTs = fmt.Errorf("%w: too many simultaneous LUT effects", ErrTooManyInputs)
	// ErrOutputLimit reports an export that reached RenderOptions.MaxOutputBytes.
	ErrOutputLimit = errors.New("rendered output reached the size limit")
)

// SegmentError identifies the render segment that failed.
type SegmentError struct {
	Index      int
	Start, End float64 // seconds on the timeline
	Audio      bool
	Err        error
}

// Location names the failed segment, for example "video segment 12 (2:00–2:10)".
func (e *SegmentError) Location() string {
	kind := "video"
	if e.Audio {
		kind = "audio"
	}
	return fmt.Sprintf("%s segment %d (%s–%s)", kind, e.Index+1, clockText(e.Start), clockText(e.End))
}

func (e *SegmentError) Error() string { return e.Location() + ": " + e.Err.Error() }

func (e *SegmentError) Unwrap() error { return e.Err }

func clockText(seconds float64) string {
	total := int(math.Floor(seconds))
	return fmt.Sprintf("%d:%02d", total/60, total%60)
}

// RenderOptions describes one export.
type RenderOptions struct {
	FFmpegPath string
	Project    Project
	// AssetPaths maps project asset IDs to private staged files owned by the server staging layer.
	AssetPaths map[string]string
	Output     string
	// Start and End select the timeline range [Start, End); End 0 means the end of the last clip.
	Start, End int
	// Width and Height scale the output; 0 keeps the project canvas.
	Width, Height int
	// SegmentFrames is the target segment length; 0 means ten seconds.
	SegmentFrames int
	// MaxOutputBytes bounds the final file; 0 means 2 GiB.
	MaxOutputBytes int64
	Encoder        Encoder
	// OnEncoderFallback is called when a hardware encoder failed and the render restarts with libx264.
	OnEncoderFallback func(error)
	Progress          func(float64)
}

// Render produces a bounded H.264/AAC MP4 of the whole project.
func Render(ctx context.Context, ffmpegPath string, project Project, assetPaths map[string]string, output string, progress func(float64)) error {
	return RenderWithOptions(ctx, RenderOptions{FFmpegPath: ffmpegPath, Project: project, AssetPaths: assetPaths, Output: output, Progress: progress})
}

// RenderWithOptions renders in segments: each video segment is encoded on its own, audio is mixed into
// sample-exact PCM per segment, and one final pass concatenates the video losslessly, limits the mixed audio
// once and encodes AAC. A failed hardware video segment restarts the render once with libx264.
func RenderWithOptions(ctx context.Context, opts RenderOptions) error {
	opts.Progress = monotonicProgress(opts.Progress)
	err := renderOnce(ctx, opts)
	var segment *SegmentError
	if err == nil || !opts.Encoder.Hardware() || ctx.Err() != nil || !errors.As(err, &segment) || segment.Audio {
		return err
	}
	if opts.OnEncoderFallback != nil {
		opts.OnEncoderFallback(err)
	}
	opts.Encoder = Encoder{Quality: opts.Encoder.Quality}
	return renderOnce(ctx, opts)
}

func renderOnce(ctx context.Context, opts RenderOptions) error {
	project := opts.Project
	if err := Validate(project); err != nil {
		return fmt.Errorf("invalid Video Studio project: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	ffmpeg, err := resolveBinary(opts.FFmpegPath, "ffmpeg")
	if err != nil {
		return err
	}
	// A project that needs a filter or xfade transition this FFmpeg lacks fails here, before any staging. A probe that
	// cannot run leaves the check to FFmpeg itself, except for a feature FFmpeg gets wrong without an error: a fadegrays
	// transition is refused then, because its self-test could not run (index §4.3).
	if missing, probeErr := ProbeCapabilities(ctx, ffmpeg); probeErr == nil {
		if err := CheckProjectCapabilities(project, missing); err != nil {
			return err
		}
	} else if err := checkProjectUnverified(project, probeErr); err != nil {
		return err
	}
	outputPath, err := validateRenderOutput(opts.Output)
	if err != nil {
		return err
	}
	duration := EnabledDuration(project)
	if duration <= 0 {
		return fmt.Errorf("project has no clips to render")
	}
	start, end := opts.Start, opts.End
	if end == 0 {
		end = duration
	}
	if start < 0 || end <= start || end > duration {
		return fmt.Errorf("render range %d–%d is outside the timeline", start, end)
	}
	// A video asset that is too wide or too tall (imported before the import rule) would make gigabyte cover-fit
	// frames: refuse it from the project's own asset sizes, before any source is probed or FFmpeg renders.
	if err := checkSourceAspects(project, start, end); err != nil {
		return err
	}
	width, height := opts.Width, opts.Height
	if width == 0 && height == 0 {
		width, height = project.Width, project.Height
	}
	if width < 2 || height < 2 || width%2 != 0 || height%2 != 0 || width > MaxSourceDimension || height > MaxSourceDimension {
		return fmt.Errorf("render size %dx%d is not supported", width, height)
	}
	maxOutput := opts.MaxOutputBytes
	if maxOutput <= 0 {
		maxOutput = defaultMaxRenderOutputBytes
	}
	rate := ProjectRate(project)
	target := opts.SegmentFrames
	if target <= 0 {
		target = int(int64(defaultSegmentSeconds) * rate.Num / rate.Den)
	}

	renderCtx, cancel := context.WithTimeout(ctx, renderTimeout)
	defer cancel()
	sources, err := collectRenderSources(renderCtx, ffmpeg, project, opts.AssetPaths)
	if err != nil {
		return err
	}
	for _, source := range sources {
		if filepath.Clean(source.path) == filepath.Clean(outputPath) {
			return fmt.Errorf("output must not overwrite an input asset")
		}
	}
	videoSegments, err := PlanVideoSegments(project, start, end, target)
	if err != nil {
		return err
	}
	hasAudio := rangeHasAudio(project, sources, start, end)
	var audioSegments []Segment
	if hasAudio {
		if audioSegments, err = PlanAudioSegments(project, start, end, target); err != nil {
			return err
		}
	}

	workDir, err := os.MkdirTemp(filepath.Dir(outputPath), "segments-")
	if err != nil {
		return fmt.Errorf("create render segment directory: %w", err)
	}
	defer os.RemoveAll(workDir)
	if err := checkRenderSpace(workDir, rate, end-start, width, height, hasAudio); err != nil {
		return err
	}

	report := monotonicProgress(opts.Progress)
	total := float64(end - start)
	videoNames := make([]string, len(videoSegments))
	for i, seg := range videoSegments {
		videoNames[i] = fmt.Sprintf("v%04d.ts", i)
		base, span := float64(seg.Start-start), float64(seg.End-seg.Start)
		err := renderVideoSegment(renderCtx, ffmpeg, project, seg, sources, rate, width, height, opts.Encoder, filepath.Join(workDir, videoNames[i]), func(p float64) {
			report(0.85 * (base + p*span) / total)
		})
		if err != nil {
			return segmentFailure(ctx, renderCtx, err, i, seg, rate, false)
		}
	}
	audioNames := make([]string, len(audioSegments))
	for i, seg := range audioSegments {
		audioNames[i] = fmt.Sprintf("a%04d.wav", i)
		base, span := float64(seg.Start-start), float64(seg.End-seg.Start)
		err := renderAudioSegment(renderCtx, ffmpeg, project, seg, sources, rate, filepath.Join(workDir, audioNames[i]), func(p float64) {
			report(0.85 + 0.10*(base+p*span)/total)
		})
		if err != nil {
			return segmentFailure(ctx, renderCtx, err, i, seg, rate, true)
		}
	}
	if err := muxSegments(renderCtx, ffmpeg, workDir, videoNames, videoSegments, audioNames, audioSegments, rate, outputPath, maxOutput, func(p float64) { report(0.95 + 0.05*p) }); err != nil {
		_ = os.Remove(outputPath)
		if ctxErr := renderCtx.Err(); ctxErr != nil {
			return ctxErr
		}
		return err
	}
	// -fs stops writing once the file reaches the limit and FFmpeg still exits successfully (some versions only
	// check it between larger writes), so a file at or above the limit is a truncated or oversized export.
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
		return fmt.Errorf("verify rendered output: %w", err)
	}
	expectedUs := rate.Seconds(end-start) * 1e6
	tolerance := math.Max(2*1e6*float64(rate.Den)/float64(rate.Num), 50_000)
	if verified.Kind != AssetVideo || verified.Width != width || verified.Height != height || math.Abs(float64(verified.DurationUs)-expectedUs) > tolerance || verified.HasAudio != hasAudio {
		_ = os.Remove(outputPath)
		return fmt.Errorf("rendered output metadata does not match the project timeline")
	}
	report(1)
	return nil
}

// collectRenderSources stages what a render reads, by asset id:
//   - the assets of enabled clips on visible visual tracks, and of audible clips;
//   - the LUT assets that on lut effects of those visual clips reference (clipLUTRefs).
//
// Videos are probed for their decodable end and their decoded (autorotated) size. LUT files are parsed with
// LoadLUTFile, so the graph only ever reads Go-written copies (stageVideoGraph).
func collectRenderSources(ctx context.Context, ffmpeg string, project Project, assetPaths map[string]string) (map[string]renderSource, error) {
	assets := assetMap(project)
	used := make(map[string]struct{})
	for _, track := range project.Tracks {
		for _, clip := range track.Clips {
			if clip.Disabled {
				continue
			}
			if visualClip(track) || audibleClip(track, clip, assets[clip.AssetID]) {
				used[clip.AssetID] = struct{}{}
			}
			if !visualClip(track) {
				continue
			}
			for _, ref := range clipLUTRefs(clip) {
				if _, assetID, ok := ParseLUTRef(ref); ok && assetID != "" {
					used[assetID] = struct{}{}
				}
			}
		}
	}
	sources := make(map[string]renderSource, len(used))
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
		source := renderSource{asset: asset, path: path}
		switch asset.Kind {
		case AssetVideo:
			probed, rotation, err := probeVideo(ctx, ffmpeg, path)
			if err != nil {
				return nil, fmt.Errorf("inspect asset %q: %w", asset.ID, err)
			}
			source.videoEnd = float64(probed.VideoDurationUs) / 1e6
			source.decodedWidth, source.decodedHeight = rotatedSize(probed.Width, probed.Height, rotation)
		case AssetImage:
			source.decodedWidth, source.decodedHeight = asset.Width, asset.Height
		case AssetLUT:
			table, err := LoadLUTFile(path)
			if err != nil {
				return nil, fmt.Errorf("load LUT asset %q: %w", asset.ID, err)
			}
			source.lut = table
		}
		sources[asset.ID] = source
	}
	return sources, nil
}

func rangeHasAudio(project Project, sources map[string]renderSource, start, end int) bool {
	for _, track := range project.Tracks {
		for _, clip := range track.Clips {
			source, ok := sources[clip.AssetID]
			if ok && audibleClip(track, clip, source.asset) && clip.Start < end && clip.Start+clip.Duration > start {
				return true
			}
		}
	}
	return false
}

func renderVideoSegment(ctx context.Context, ffmpeg string, project Project, seg Segment, sources map[string]renderSource, rate Rate, width, height int, encoder Encoder, output string, progress func(float64)) error {
	inputs, graph, files, err := buildVideoSegment(project, seg, sources, rate, width, height, encoder.filterTail())
	if err != nil {
		return err
	}
	workDir, graphPath, err := stageVideoGraph(filepath.Dir(output), graph, rate, files)
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)
	// -copyts -start_at_zero keeps every seeked input on the source's own timeline, so the graph can pick source
	// frames by absolute position instead of restarting the frame-rate cadence at the first frame after the seek.
	// Hardware encoders initialise their device (globalArgs) before the inputs.
	args := append([]string{"-hide_banner", "-nostdin", "-loglevel", "error", "-copyts", "-start_at_zero"}, encoder.globalArgs()...)
	args = append(args, segmentInputArgs(inputs, rate)...)
	args = append(args,
		"-filter_threads", strconv.Itoa(threadLimit), "-filter_complex_threads", strconv.Itoa(threadLimit),
		"-filter_complex_script", graphPath, "-map", "[vs_video_out]", "-map_metadata", "-1",
		"-frames:v", strconv.Itoa(seg.End-seg.Start), "-r", rate.String())
	args = append(args, encoder.videoArgs(rate)...)
	// The concat demuxer subtracts each segment's start time, so the default MPEG-TS mux delay is harmless and
	// keeps B-frame timestamps valid.
	args = append(args, "-threads", strconv.Itoa(threadLimit), "-an",
		"-progress", "pipe:1", "-nostats", "-f", "mpegts", output)
	return runFFmpegIn(ctx, workDir, ffmpeg, args, rate.Seconds(seg.End-seg.Start), progress, inputPaths(inputs), output)
}

func renderAudioSegment(ctx context.Context, ffmpeg string, project Project, seg Segment, sources map[string]renderSource, rate Rate, output string, progress func(float64)) error {
	inputs, graph, err := buildAudioSegment(project, seg, sources, rate)
	if err != nil {
		return err
	}
	graphPath, err := writeGraph(filepath.Dir(output), graph)
	if err != nil {
		return err
	}
	defer os.Remove(graphPath)
	args := append([]string{"-hide_banner", "-nostdin", "-loglevel", "error"}, segmentInputArgs(inputs, rate)...)
	args = append(args,
		"-filter_complex_threads", strconv.Itoa(threadLimit), "-filter_complex_script", graphPath,
		"-map", "[vs_audio_out]", "-map_metadata", "-1", "-c:a", "pcm_f32le", "-ar", strconv.Itoa(AudioSampleRate), "-ac", "2",
		"-progress", "pipe:1", "-nostats", "-f", "wav", output)
	return runFFmpeg(ctx, ffmpeg, args, rate.Seconds(seg.End-seg.Start), progress, inputPaths(inputs), output)
}

func muxSegments(ctx context.Context, ffmpeg, dir string, videoNames []string, videoSegments []Segment, audioNames []string, audioSegments []Segment, rate Rate, output string, maxOutput int64, progress func(float64)) error {
	videoList := filepath.Join(dir, "video.ffconcat")
	if err := writeConcatList(videoList, videoNames, videoSegments, rate); err != nil {
		return err
	}
	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error",
		"-protocol_whitelist", allowedProtocols, "-format_whitelist", intermediateDemuxers, "-f", "concat", "-safe", "1", "-i", videoList}
	if len(audioNames) > 0 {
		audioList := filepath.Join(dir, "audio.ffconcat")
		if err := writeConcatList(audioList, audioNames, audioSegments, rate); err != nil {
			return err
		}
		args = append(args, "-protocol_whitelist", allowedProtocols, "-format_whitelist", intermediateDemuxers, "-f", "concat", "-safe", "1", "-i", audioList)
	}
	args = append(args, "-map", "0:v:0", "-c:v", "copy")
	if len(audioNames) > 0 {
		args = append(args, "-map", "1:a:0", "-af", "alimiter=limit=0.95", "-c:a", "aac", "-b:a", "192k", "-ar", strconv.Itoa(AudioSampleRate), "-ac", "2")
	} else {
		args = append(args, "-an")
	}
	frames := 0
	for _, seg := range videoSegments {
		frames += seg.End - seg.Start
	}
	args = append(args, "-map_metadata", "-1", "-movflags", "+faststart", "-max_muxing_queue_size", "512",
		"-fs", strconv.FormatInt(maxOutput, 10), "-progress", "pipe:1", "-nostats", "-f", "mp4", output)
	return runFFmpeg(ctx, ffmpeg, args, rate.Seconds(frames), progress, nil, output)
}

// writeConcatList writes an ffconcat list with explicit durations, so segment offsets never depend on
// container timestamp estimates.
func writeConcatList(path string, names []string, segments []Segment, rate Rate) error {
	return os.WriteFile(path, []byte(concatList(names, segments, rate)), 0o600)
}

// concatList measures every duration between microsecond-rounded segment starts relative to the first segment,
// so the rounding of NTSC durations never accumulates over an hour of segments.
func concatList(names []string, segments []Segment, rate Rate) string {
	var list strings.Builder
	list.WriteString("ffconcat version 1.0\n")
	micros := func(frame int) int64 { return int64(math.Round(rate.Seconds(frame-segments[0].Start) * 1e6)) }
	for i, name := range names {
		duration := micros(segments[i].End) - micros(segments[i].Start)
		fmt.Fprintf(&list, "file '%s'\nduration %d.%06d\n", name, duration/1_000_000, duration%1_000_000)
	}
	return list.String()
}

func writeGraph(dir, graph string) (string, error) {
	file, err := os.CreateTemp(dir, "graph-*.txt")
	if err != nil {
		return "", fmt.Errorf("create FFmpeg filter graph: %w", err)
	}
	if _, err := file.WriteString(graph); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return "", fmt.Errorf("write FFmpeg filter graph: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(file.Name())
		return "", fmt.Errorf("close FFmpeg filter graph: %w", err)
	}
	return file.Name(), nil
}

// graphFiles are the files a video FFmpeg process reads next to graph.txt, by bare name (index §4.10, §4.11):
//   - the sendcmd scripts of every clip's command groups;
//   - the LUT tables of its lut effects.
type graphFiles struct {
	commands []sendCmdFile
	luts     []lutFile
}

// lutFile is one LUT table that lut3d filters read. name is vs_lut_<n>.cube: n numbers the distinct tables of one FFmpeg
// process in the order the graph first uses them (lutNames), so every lut3d of that process that uses the table reads
// the same file.
type lutFile struct {
	name string
	lut  *LUT3D
}

// lutNames gives each distinct LUT table of one FFmpeg process one file name, vs_lut_0.cube, vs_lut_1.cube, … in the
// order of first use. Tables are told apart by identity: a built-in look and a project lut asset are each one shared
// *LUT3D, so any number of lut effects of one table stage one file (index §4.11). The zero value is ready.
type lutNames struct {
	byTable map[*LUT3D]string
}

// name returns the file name of table and numbers it on first use.
func (n *lutNames) name(table *LUT3D) string {
	if name, ok := n.byTable[table]; ok {
		return name
	}
	if n.byTable == nil {
		n.byTable = make(map[*LUT3D]string)
	}
	name := fmt.Sprintf("vs_lut_%d.cube", len(n.byTable))
	n.byTable[table] = name
	return name
}

// addLUTs appends the LUT files of one clip or layer to the process's files. A file whose name is present already is
// the same table (lutNames), so it is kept once and is written once.
func (f *graphFiles) addLUTs(luts []lutFile) {
	for _, file := range luts {
		if !slices.ContainsFunc(f.luts, func(have lutFile) bool { return have.name == file.name }) {
			f.luts = append(f.luts, file)
		}
	}
}

// sendCmdFileName and lutFileName are the only names a staged command or LUT file may have. A command file is a clip's
// group file (vs_cmd_<L>_<C>.txt, vs_cmd_<L>_<C>_<g>.txt) or one of its blur_in mask files (vs_cmd_<L>_<C>_m1.txt and
// _m2.txt, 2e).
var (
	sendCmdFileName = regexp.MustCompile(`^vs_cmd_[0-9]+_[0-9]+(_[0-9]+|_m[1-2])?\.txt$`)
	lutFileName     = regexp.MustCompile(`^vs_lut_[0-9]+\.cube$`)
)

// stageVideoGraph creates a fresh vsg-* directory in dir for one video FFmpeg process (renderVideoSegment,
// RenderFrame). It writes the filter graph (graph.txt), every command file and every LUT file into it, all with mode
// 0o600 (index §4.11). Each LUT file is one distinct table (lutNames), written once however many lut3d filters read it;
// a process stages at most MaxSegmentLUTInstances of them.
//
// FFmpeg runs with that directory as its working directory (runFFmpegIn), so the graph names files by bare name and no
// host path reaches filter-graph escaping. The caller removes workDir; on error nothing is left behind.
func stageVideoGraph(dir, graph string, rate Rate, files graphFiles) (workDir, graphPath string, err error) {
	created, err := os.MkdirTemp(dir, "vsg-")
	if err != nil {
		return "", "", fmt.Errorf("create FFmpeg graph directory: %w", err)
	}
	fail := func(err error) (string, string, error) {
		_ = os.RemoveAll(created)
		return "", "", err
	}
	if workDir, err = filepath.Abs(created); err != nil {
		return fail(fmt.Errorf("resolve FFmpeg graph directory: %w", err))
	}
	graphPath = filepath.Join(workDir, "graph.txt")
	if err := os.WriteFile(graphPath, []byte(graph), 0o600); err != nil {
		return fail(fmt.Errorf("write FFmpeg filter graph: %w", err))
	}
	if len(files.luts) > MaxSegmentLUTInstances {
		return fail(fmt.Errorf("%w: %d LUT tables for one FFmpeg process, at most %d", ErrTooManyLUTs, len(files.luts), MaxSegmentLUTInstances))
	}
	seen := make(map[string]bool, len(files.commands)+len(files.luts))
	for _, file := range files.commands {
		if !sendCmdFileName.MatchString(file.name) || seen[file.name] {
			return fail(fmt.Errorf("invalid sendcmd file name %q", file.name))
		}
		seen[file.name] = true
		if err := WriteSendCmd(filepath.Join(workDir, file.name), rate, file.tracks); err != nil {
			return fail(err)
		}
	}
	for _, file := range files.luts {
		if !lutFileName.MatchString(file.name) || seen[file.name] || file.lut == nil {
			return fail(fmt.Errorf("invalid LUT file %q", file.name))
		}
		seen[file.name] = true
		if err := writeStagedLUT(filepath.Join(workDir, file.name), file.lut); err != nil {
			return fail(err)
		}
	}
	return workDir, graphPath, nil
}

// writeStagedLUT writes a LUT for FFmpeg's lut3d as a new 0o600 file. It writes the table resampled onto the unit
// domain (unitDomainLUT), without TITLE or DOMAIN lines, through WriteCube: FFmpeg reads only Go-formatted numbers
// from parsed floats (index §4.20) and never depends on its own DOMAIN handling.
func writeStagedLUT(path string, l *LUT3D) error {
	if err := checkLUT(l); err != nil {
		return fmt.Errorf("write LUT file: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("write LUT file: %w", err)
	}
	writer := bufio.NewWriterSize(file, 64<<10)
	err = WriteCube(writer, unitDomainLUT(l))
	if err == nil {
		err = writer.Flush()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write LUT file: %w", err)
	}
	return nil
}

// runFFmpeg runs FFmpeg in the current working directory (see runFFmpegIn).
func runFFmpeg(ctx context.Context, ffmpeg string, args []string, seconds float64, progress func(float64), inputs []string, output string) error {
	return runFFmpegIn(ctx, "", ffmpeg, args, seconds, progress, inputs, output)
}

// runFFmpegIn runs FFmpeg with dir as its working directory ("" keeps the current one), reports progress from
// -progress pipe:1 and removes output on failure. Video processes run in their vsg-* staging directory, where the
// graph names command files by bare name; every path in args and the binary must then be absolute (resolveBinary
// returns absolute paths).
func runFFmpegIn(ctx context.Context, dir, ffmpeg string, args []string, seconds float64, progress func(float64), inputs []string, output string) error {
	if dir != "" && !filepath.IsAbs(ffmpeg) {
		return fmt.Errorf("FFmpeg binary %q must be an absolute path when it runs in a staging directory", ffmpeg)
	}
	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	cmd.Dir = dir
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("open FFmpeg progress stream: %w", err)
	}
	var stderr cappedBuffer
	stderr.limit = 1 << 20
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start FFmpeg: %w", err)
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 4096), 64<<10)
	for scanner.Scan() {
		if progress == nil {
			continue
		}
		if value, ok := renderProgress(scanner.Text(), seconds); ok {
			progress(value)
		}
	}
	scanErr := scanner.Err()
	if scanErr != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	waitErr := cmd.Wait()
	if err := ctx.Err(); err != nil {
		_ = os.Remove(output)
		return err
	}
	if scanErr != nil {
		_ = os.Remove(output)
		return fmt.Errorf("read FFmpeg progress: %w", scanErr)
	}
	if waitErr != nil {
		_ = os.Remove(output)
		message := scrubPaths(strings.TrimSpace(stderr.String()), output, inputs)
		if message == "" {
			return fmt.Errorf("FFmpeg failed: %w", waitErr)
		}
		return fmt.Errorf("FFmpeg failed: %s", message)
	}
	return nil
}

func segmentFailure(parent, renderCtx context.Context, err error, index int, seg Segment, rate Rate, audio bool) error {
	if ctxErr := parent.Err(); ctxErr != nil {
		return ctxErr
	}
	if ctxErr := renderCtx.Err(); ctxErr != nil {
		return ctxErr
	}
	return &SegmentError{Index: index, Start: rate.Seconds(seg.Start), End: rate.Seconds(seg.End), Audio: audio, Err: err}
}

// checkRenderSpace fails early when the staging volume cannot hold the video segments, the final copy and the
// float PCM audio. An unknown amount of free space never blocks a render.
func checkRenderSpace(dir string, rate Rate, frames, width, height int, audio bool) error {
	free, err := fileutil.FreeDiskBytes(dir)
	if err != nil {
		return nil
	}
	seconds := rate.Seconds(frames)
	pixelsPerSecond := float64(width) * float64(height) * float64(rate.Num) / float64(rate.Den)
	need := 2 * seconds * pixelsPerSecond * 0.15 / 8
	if audio {
		need += float64(rate.SampleAt(frames)) * 8
	}
	need *= 1.2
	if float64(free) < need {
		return fmt.Errorf("%w: about %d MiB needed, %d MiB free", ErrInsufficientDiskSpace, int64(need)>>20, free>>20)
	}
	return nil
}

func monotonicProgress(report func(float64)) func(float64) {
	last := 0.0
	return func(value float64) {
		if report == nil {
			return
		}
		value = math.Min(1, math.Max(0, value))
		if value > last {
			last = value
			report(value)
		}
	}
}

func validateRenderOutput(output string) (string, error) {
	return validateNewOutput(output, "")
}

func renderProgress(line string, totalSeconds float64) (float64, bool) {
	key, value, ok := strings.Cut(line, "=")
	if !ok || totalSeconds <= 0 {
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
	return math.Min(1, math.Max(0, elapsed/totalSeconds)), true
}

func scrubPaths(message, output string, inputs []string) string {
	if output != "" {
		message = strings.ReplaceAll(message, output, "<output>")
	}
	for _, input := range inputs {
		message = strings.ReplaceAll(message, input, "<staged-asset>")
	}
	return message
}
