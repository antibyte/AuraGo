package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/mediabins"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// mediaConversionLookPath is the executable lookup seam. Tests replace it to
// simulate a missing or present converter binary.
var mediaConversionLookPath = mediabins.LookPath

// mediaConversionRunCommand is the converter execution seam. Tests replace it
// to capture the argument vector without starting a real converter.
var mediaConversionRunCommand = func(ctx context.Context, name string, args []string) ([]byte, error) {
	return mediabins.Run(ctx, name, args, mediabins.DefaultOutputLimit)
}

const (
	// mediaConversionProtocols restricts ffmpeg and ffprobe to local sources.
	// A converter must never follow a remote reference that a staged playlist
	// or concat list points at.
	mediaConversionProtocols = "file,pipe"
	// mediaConversionOutputLimit caps the retained converter output per stream.
	mediaConversionOutputLimit = 4 << 20
)

// mediaConversionDemuxers maps a known input extension to the ffmpeg demuxer
// that reads it. The format whitelist is only applied for known extensions so
// that unusual but legitimate local inputs keep working.
var mediaConversionDemuxers = map[string]string{
	"mp3": "mp3", "wav": "wav", "aac": "aac", "m4a": "mov", "m4b": "mov",
	"ogg": "ogg", "oga": "ogg", "opus": "ogg", "flac": "flac", "aiff": "aiff",
	"aif": "aiff", "wma": "asf", "amr": "amr", "au": "au", "ape": "ape",
	"mp4": "mov", "m4v": "mov", "mov": "mov", "mkv": "matroska", "webm": "matroska,webm",
	"avi": "avi", "flv": "flv", "mpeg": "mpeg", "mpg": "mpeg", "ts": "mpegts",
	"mxf": "mxf", "3gp": "3gp", "asf": "asf", "rm": "rm", "rv": "rv",
	"png": "png_pipe,image2", "jpg": "jpeg_pipe,image2", "jpeg": "jpeg_pipe,image2",
	"webp": "webp_pipe,image2", "gif": "gif", "tiff": "tiff_pipe,image2",
	"tif": "tiff_pipe,image2", "bmp": "bmp_pipe,image2",
}

// conversionFormatAllowlist lists the target formats per media type. A request
// outside the list is rejected with the supported formats in the message.
var conversionFormatAllowlist = map[string][]string{
	"audio": {"mp3", "wav", "aac", "m4a", "ogg", "flac", "opus", "aiff", "wma"},
	"video": {"mp4", "webm", "mkv", "mov", "avi", "m4v", "mpeg", "ts"},
	"image": {"png", "jpg", "jpeg", "webp", "gif", "tiff", "tif", "bmp"},
}

type MediaConversionRequest struct {
	Operation    string
	FilePath     string
	OutputFile   string
	OutputFormat string
	VideoCodec   string
	AudioCodec   string
	VideoBitrate string
	AudioBitrate string
	Width        int
	Height       int
	FPS          int
	SampleRate   int
	QualityPct   int
}

// conversionTarget is the single resolved description of one conversion. The
// staged file name always derives from Format, never from the destination file
// name, so the converter always receives an input it can interpret.
type conversionTarget struct {
	Destination string // absolute path the result is written to
	StagedPath  string // private staging file the converter writes
	Format      string // normalized target format
}

type mediaConversionResult struct {
	Status          string  `json:"status"`
	Message         string  `json:"message"`
	Operation       string  `json:"operation,omitempty"`
	File            string  `json:"file,omitempty"`
	Tool            string  `json:"tool,omitempty"`
	MediaType       string  `json:"media_type,omitempty"`
	InputFormat     string  `json:"input_format,omitempty"`
	OutputFormat    string  `json:"output_format,omitempty"`
	Width           int     `json:"width,omitempty"`
	Height          int     `json:"height,omitempty"`
	Channels        int     `json:"channels,omitempty"`
	SampleRate      int     `json:"sample_rate,omitempty"`
	DurationSeconds float64 `json:"duration_seconds,omitempty"`
	OutputSizeBytes int64   `json:"output_size_bytes,omitempty"`
	VideoCodec      string  `json:"video_codec,omitempty"`
	AudioCodec      string  `json:"audio_codec,omitempty"`
}

type mediaConversionHealthBinary struct {
	Available bool   `json:"available"`
	Path      string `json:"path,omitempty"`
	Version   string `json:"version,omitempty"`
	Message   string `json:"message,omitempty"`
}

type mediaConversionHealthResult struct {
	Status      string                      `json:"status"`
	Message     string                      `json:"message"`
	FFmpeg      mediaConversionHealthBinary `json:"ffmpeg"`
	ImageMagick mediaConversionHealthBinary `json:"imagemagick"`
}

type ffprobeOutput struct {
	Streams []struct {
		CodecType  string `json:"codec_type"`
		CodecName  string `json:"codec_name"`
		Width      int    `json:"width"`
		Height     int    `json:"height"`
		Channels   int    `json:"channels"`
		SampleRate string `json:"sample_rate"`
	} `json:"streams"`
	Format struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
	} `json:"format"`
}

func mediaConversionJSON(r mediaConversionResult) string {
	b, err := json.Marshal(r)
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":%q}`, "failed to encode result: "+err.Error())
	}
	return string(b)
}

// MediaConversionDeps carries the runtime dependencies of a conversion.
type MediaConversionDeps struct {
	// DataDir is the media data directory. A converted file is copied there so
	// that the media registry entry points at a file the server can serve.
	DataDir string
	// RegistryDB is the media registry. A nil database skips the registration.
	RegistryDB *sql.DB
}

// ExecuteMediaConversion runs one media conversion. The caller context is the
// parent of the conversion deadline: cancelling the agent turn stops the
// converter instead of leaving it running until the timeout.
//
// The converted file is registered in the media registry when a registry
// database is supplied, so the media view and the send_* tools can find it.
func ExecuteMediaConversion(ctx context.Context, workspaceDir string, cfg *config.MediaConversionConfig, deps MediaConversionDeps, req MediaConversionRequest) string {
	if ctx == nil {
		ctx = context.Background()
	}
	if cfg == nil || !cfg.Enabled {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: "media conversion is disabled"})
	}

	op := strings.ToLower(strings.TrimSpace(req.Operation))
	if op == "" {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: "operation is required"})
	}
	if op != "info" && cfg.ReadOnly {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: "media conversion is in read-only mode"})
	}
	if strings.TrimSpace(workspaceDir) == "" {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: "workspace directory is required", Operation: op})
	}
	if err := ctx.Err(); err != nil {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: "media conversion was cancelled", Operation: op})
	}

	if workspaceDir != "" && req.FilePath != "" {
		resolved, err := secureResolve(workspaceDir, req.FilePath)
		if err != nil {
			return mediaConversionJSON(mediaConversionResult{Status: "error", Message: fmt.Sprintf("invalid input path: %v", err), Operation: op})
		}
		req.FilePath = resolved
	}
	if workspaceDir != "" && req.OutputFile != "" {
		resolved, err := secureResolve(workspaceDir, req.OutputFile)
		if err != nil {
			return mediaConversionJSON(mediaConversionResult{Status: "error", Message: fmt.Sprintf("invalid output path: %v", err), Operation: op})
		}
		req.OutputFile = resolved
	}
	if strings.TrimSpace(req.FilePath) == "" {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: "file_path is required", Operation: op})
	}
	// Keep the source name: the request path is replaced by the staged copy below.
	sourceName := filepath.Base(req.FilePath)
	// External converters receive only private staged files. A workspace path can
	// otherwise be swapped for a symlink after validation but before they open it.
	input, err := rootedToolOpen(req.FilePath)
	if err != nil {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: fmt.Sprintf("cannot access input file: %v", err), Operation: op})
	}
	defer input.Close()
	if err := checkMediaFileSize(cfg, req.FilePath); err != nil {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: err.Error(), Operation: op})
	}
	stagingDir, err := os.MkdirTemp("", "aurago-media-*")
	if err != nil {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: fmt.Sprintf("cannot stage input: %v", err), Operation: op})
	}
	defer os.RemoveAll(stagingDir)
	stagedInput := filepath.Join(stagingDir, "input"+filepath.Ext(req.FilePath))
	staged, err := os.OpenFile(stagedInput, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: fmt.Sprintf("cannot stage input: %v", err), Operation: op})
	}
	_, copyErr := io.Copy(staged, mediaConversionContextReader{ctx: ctx, r: input})
	closeErr := staged.Close()
	if copyErr != nil || closeErr != nil {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: fmt.Sprintf("cannot stage input: %v", errors.Join(copyErr, closeErr)), Operation: op})
	}
	var target conversionTarget
	if op != "info" {
		target, err = resolveConversionTarget(stagingDir, req.FilePath, req.OutputFile, req.OutputFormat, mediaTypeForOperation(op))
		if err != nil {
			return mediaConversionJSON(mediaConversionResult{Status: "error", Message: err.Error(), Operation: op})
		}
		if target.Destination, err = secureResolve(workspaceDir, target.Destination); err != nil {
			return mediaConversionJSON(mediaConversionResult{Status: "error", Message: fmt.Sprintf("invalid output path: %v", err), Operation: op})
		}
		if err = requireUnprotectedNotesPath(target.Destination, true); err != nil {
			return mediaConversionJSON(mediaConversionResult{Status: "error", Message: err.Error(), Operation: op})
		}
	}
	req.FilePath = stagedInput

	var result string
	switch op {
	case "audio_convert":
		result = executeFFmpegConversion(ctx, cfg, req, target, "audio")
	case "video_convert":
		result = executeFFmpegConversion(ctx, cfg, req, target, "video")
	case "image_convert":
		result = executeImageMagickConversion(ctx, cfg, req, target)
	case "info":
		return executeMediaInfo(ctx, cfg, req)
	default:
		return mediaConversionJSON(mediaConversionResult{
			Status:    "error",
			Message:   "unknown operation: use audio_convert, video_convert, image_convert, or info",
			Operation: op,
		})
	}
	var converted mediaConversionResult
	if err := json.Unmarshal([]byte(result), &converted); err != nil || converted.Status != "success" {
		return result
	}
	output, err := os.Open(target.StagedPath)
	if err != nil {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: fmt.Sprintf("cannot open converted output: %v", err), Operation: op})
	}
	defer output.Close()
	if err := rootedToolWriteFromReaderAtomic(target.Destination, output); err != nil {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: fmt.Sprintf("cannot save converted output: %v", err), Operation: op})
	}
	converted.File = target.Destination
	if info, err := os.Stat(target.Destination); err == nil {
		converted.OutputSizeBytes = info.Size()
	}
	if meta := mediaConversionOutputMetadata(ctx, cfg, target.Destination); meta != nil {
		if meta.Width > 0 {
			converted.Width = meta.Width
		}
		if meta.Height > 0 {
			converted.Height = meta.Height
		}
		if meta.DurationSeconds > 0 {
			converted.DurationSeconds = meta.DurationSeconds
		}
	}
	if deps.RegistryDB != nil {
		if err := registerConvertedMedia(deps, target, converted, sourceName); err != nil {
			// The file is already written; a registry failure must not lose it.
			converted.Message += fmt.Sprintf(" (media registry update failed: %v)", err)
		}
	}
	return mediaConversionJSON(converted)
}

// mediaConversionOutputMetadata reads the metadata of the produced file. It is
// best effort: a missing probe never fails the conversion.
func mediaConversionOutputMetadata(ctx context.Context, cfg *config.MediaConversionConfig, path string) *mediaConversionResult {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".tif", ".tiff", ".webp":
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		cfgImage, _, err := image.DecodeConfig(f)
		if err != nil {
			return nil
		}
		return &mediaConversionResult{MediaType: "image", Width: cfgImage.Width, Height: cfgImage.Height}
	default:
		meta, err := probeMediaMetadata(ctx, cfg, path)
		if err != nil {
			return nil
		}
		return &meta
	}
}

// registerConvertedMedia copies the converted file into the media data directory
// and registers that copy. The registry only serves files that live in the media
// data directory, so registering the workspace file directly would create an
// invisible entry without a usable media URL.
func registerConvertedMedia(deps MediaConversionDeps, target conversionTarget, result mediaConversionResult, sourceName string) error {
	mediaType := result.MediaType
	if mediaType == "" {
		mediaType = mediaTypeForOperation(result.Operation)
	}
	subdir, prefix, ok := MediaDataLocation(mediaType)
	if !ok {
		return fmt.Errorf("media type %q has no media data location", mediaType)
	}
	if strings.TrimSpace(deps.DataDir) == "" {
		return fmt.Errorf("media data directory is not configured")
	}
	hash, hashErr := ComputeMediaFileHash(target.Destination)
	if hashErr != nil {
		hash = ""
	}
	filename := mediaRegistryFilename(filepath.Base(target.Destination), hash)
	mediaPath := filepath.Join(deps.DataDir, subdir, filename)
	if err := copyMediaFile(target.Destination, mediaPath); err != nil {
		return fmt.Errorf("cannot place the converted file in the media directory: %w", err)
	}
	_, _, err := RegisterMedia(deps.RegistryDB, MediaItem{
		MediaType:   mediaType,
		SourceTool:  "media_conversion",
		Filename:    filename,
		FilePath:    mediaPath,
		WebPath:     prefix + filename,
		FileSize:    result.OutputSizeBytes,
		Format:      target.Format,
		Hash:        hash,
		DurationMs:  int64(result.DurationSeconds * 1000),
		Description: fmt.Sprintf("%s converted to %s", sourceName, target.Format),
	})
	return err
}

// mediaRegistryFilename builds a collision-free media file name from the result
// name and the content hash.
func mediaRegistryFilename(base, hash string) string {
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	var cleaned strings.Builder
	for _, r := range stem {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			cleaned.WriteRune(r)
		default:
			cleaned.WriteRune('_')
		}
	}
	name := cleaned.String()
	if name == "" {
		name = "media"
	}
	if len(name) > 64 {
		name = name[:64]
	}
	if len(hash) >= 8 {
		name += "_" + hash[:8]
	}
	return name + ext
}

// copyMediaFile copies a file into the media data directory. The copy is
// written through a temporary file so that a reader never sees a partial file.
func copyMediaFile(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	dir := filepath.Dir(destination)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".media-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o640); err != nil {
		return err
	}
	// os.Rename replaces an existing regular file on every supported platform,
	// so the destination never disappears in between.
	return os.Rename(tmp.Name(), destination)
}

func MediaConversionHealth(ctx context.Context, cfg *config.MediaConversionConfig) map[string]interface{} {
	if cfg == nil {
		return map[string]interface{}{"status": "error", "message": "media conversion config is not available"}
	}

	ffmpeg := inspectMediaBinary(ctx, cfg.FFmpegPath, []string{"ffmpeg"}, []string{"-version"})
	imageMagick := inspectImageMagickBinary(ctx, cfg.ImageMagickPath)

	status := "success"
	message := "FFmpeg and ImageMagick are available"
	switch {
	case !ffmpeg.Available && !imageMagick.Available:
		status = "error"
		message = "Neither FFmpeg nor ImageMagick is available"
	case !ffmpeg.Available:
		status = "warning"
		message = "ImageMagick is available, but FFmpeg is missing"
	case !imageMagick.Available:
		status = "warning"
		message = "FFmpeg is available, but ImageMagick is missing"
	}

	result := mediaConversionHealthResult{
		Status:      status,
		Message:     message,
		FFmpeg:      ffmpeg,
		ImageMagick: imageMagick,
	}
	var out map[string]interface{}
	raw, _ := json.Marshal(result)
	_ = json.Unmarshal(raw, &out)
	return out
}

func executeFFmpegConversion(ctx context.Context, cfg *config.MediaConversionConfig, req MediaConversionRequest, target conversionTarget, mediaType string) string {
	if strings.TrimSpace(req.FilePath) == "" {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: "file_path is required", Operation: req.Operation})
	}
	if _, err := os.Stat(req.FilePath); err != nil {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: fmt.Sprintf("cannot access input file: %v", err), Operation: req.Operation})
	}
	if err := checkMediaFileSize(cfg, req.FilePath); err != nil {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: err.Error(), Operation: req.Operation})
	}

	ffmpegPath, err := resolveConfiguredBinary(cfg.FFmpegPath, []string{"ffmpeg"})
	if err != nil {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: err.Error(), Operation: req.Operation})
	}

	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error", "-protocol_whitelist", mediaConversionProtocols}
	if demuxer := mediaConversionDemuxerForPath(req.FilePath); demuxer != "" {
		args = append(args, "-format_whitelist", demuxer)
	}
	args = append(args, "-y", "-i", req.FilePath)
	switch mediaType {
	case "audio":
		// The result of an audio conversion is an audio file, never a container
		// that still carries the source video track.
		args = append(args, "-vn")
		if req.AudioCodec != "" {
			args = append(args, "-c:a", req.AudioCodec)
		}
		if req.AudioBitrate != "" {
			args = append(args, "-b:a", req.AudioBitrate)
		}
		if req.SampleRate > 0 {
			args = append(args, "-ar", strconv.Itoa(req.SampleRate))
		}
	case "video":
		if req.VideoCodec != "" {
			args = append(args, "-c:v", req.VideoCodec)
		}
		if req.AudioCodec != "" {
			args = append(args, "-c:a", req.AudioCodec)
		}
		if req.VideoBitrate != "" {
			args = append(args, "-b:v", req.VideoBitrate)
		}
		if req.AudioBitrate != "" {
			args = append(args, "-b:a", req.AudioBitrate)
		}
		if scale := ffmpegScaleArg(req.Width, req.Height); scale != "" {
			args = append(args, "-vf", scale)
		}
		if req.FPS > 0 {
			args = append(args, "-r", strconv.Itoa(req.FPS))
		}
	}
	args = append(args, target.StagedPath)

	if err := ensureParentDir(target.StagedPath); err != nil {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: err.Error(), Operation: req.Operation})
	}

	ctx, cancel := context.WithTimeout(ctx, mediaConversionTimeout(cfg))
	defer cancel()

	output, runErr := mediaConversionRunCommand(ctx, ffmpegPath, args)
	if runErr != nil {
		return mediaConversionRunError(ctx, cfg, runErr, output, req.Operation, "ffmpeg")
	}

	return mediaConversionJSON(mediaConversionResult{
		Status:       "success",
		Message:      fmt.Sprintf("%s converted successfully", mediaType),
		Operation:    req.Operation,
		File:         target.StagedPath,
		Tool:         "ffmpeg",
		MediaType:    mediaType,
		OutputFormat: target.Format,
	})
}

func executeImageMagickConversion(ctx context.Context, cfg *config.MediaConversionConfig, req MediaConversionRequest, target conversionTarget) string {
	if strings.TrimSpace(req.FilePath) == "" {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: "file_path is required", Operation: req.Operation})
	}
	if _, err := os.Stat(req.FilePath); err != nil {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: fmt.Sprintf("cannot access input file: %v", err), Operation: req.Operation})
	}
	if err := checkMediaFileSize(cfg, req.FilePath); err != nil {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: err.Error(), Operation: req.Operation})
	}

	imageMagickPath, err := resolveConfiguredImageMagick(cfg.ImageMagickPath)
	if err != nil {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: err.Error(), Operation: req.Operation})
	}

	args := []string{req.FilePath}
	if geometry := imageMagickResizeArg(req.Width, req.Height); geometry != "" {
		args = append(args, "-resize", geometry)
	}
	if req.QualityPct != 0 {
		if req.QualityPct < 1 || req.QualityPct > 100 {
			return mediaConversionJSON(mediaConversionResult{Status: "error", Message: "quality_pct must be between 1 and 100", Operation: req.Operation})
		}
		args = append(args, "-quality", strconv.Itoa(req.QualityPct))
	}
	args = append(args, target.StagedPath)

	ctx, cancel := context.WithTimeout(ctx, mediaConversionTimeout(cfg))
	defer cancel()
	output, runErr := mediaConversionRunCommand(ctx, imageMagickPath, args)
	if runErr != nil {
		return mediaConversionRunError(ctx, cfg, runErr, output, req.Operation, imageMagickToolName(imageMagickPath))
	}

	return mediaConversionJSON(mediaConversionResult{
		Status:       "success",
		Message:      "image converted successfully",
		Operation:    req.Operation,
		File:         target.StagedPath,
		Tool:         imageMagickToolName(imageMagickPath),
		MediaType:    "image",
		OutputFormat: target.Format,
	})
}

func executeMediaInfo(ctx context.Context, cfg *config.MediaConversionConfig, req MediaConversionRequest) string {
	if strings.TrimSpace(req.FilePath) == "" {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: "file_path is required", Operation: req.Operation})
	}
	if _, err := os.Stat(req.FilePath); err != nil {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: fmt.Sprintf("cannot access input file: %v", err), Operation: req.Operation})
	}

	ext := strings.ToLower(filepath.Ext(req.FilePath))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".tif", ".tiff", ".webp":
		return imageInfoResult(req)
	default:
		return ffprobeInfoResult(ctx, cfg, req)
	}
}

func imageInfoResult(req MediaConversionRequest) string {
	f, err := os.Open(req.FilePath)
	if err != nil {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: fmt.Sprintf("cannot open input file: %v", err), Operation: req.Operation})
	}
	defer f.Close()

	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: fmt.Sprintf("cannot read image info: %v", err), Operation: req.Operation})
	}

	return mediaConversionJSON(mediaConversionResult{
		Status:      "success",
		Message:     "image info collected",
		Operation:   req.Operation,
		Tool:        "builtin",
		MediaType:   "image",
		InputFormat: strings.ToLower(format),
		Width:       cfg.Width,
		Height:      cfg.Height,
	})
}

func ffprobeInfoResult(ctx context.Context, cfg *config.MediaConversionConfig, req MediaConversionRequest) string {
	probe, err := probeMediaMetadata(ctx, cfg, req.FilePath)
	if err != nil {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: err.Error(), Operation: req.Operation, Tool: "ffprobe"})
	}
	probe.Status = "success"
	probe.Message = "media info collected"
	probe.Operation = req.Operation
	probe.Tool = "ffprobe"
	return mediaConversionJSON(probe)
}

// probeMediaMetadata reads the stream metadata of a media file with ffprobe.
func probeMediaMetadata(ctx context.Context, cfg *config.MediaConversionConfig, path string) (mediaConversionResult, error) {
	ffprobePath, err := resolveFFprobeBinary(cfg)
	if err != nil {
		return mediaConversionResult{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, mediaConversionTimeout(cfg))
	defer cancel()
	args := []string{"-hide_banner", "-nostdin", "-v", "error", "-protocol_whitelist", mediaConversionProtocols}
	if demuxer := mediaConversionDemuxerForPath(path); demuxer != "" {
		args = append(args, "-format_whitelist", demuxer)
	}
	args = append(args, "-print_format", "json", "-show_format", "-show_streams", path)
	output, runErr := mediaConversionRunCommand(ctx, ffprobePath, args)
	if runErr != nil {
		return mediaConversionResult{}, errors.New(trimCommandOutput(runErr, output))
	}

	var probe ffprobeOutput
	if err := json.Unmarshal(output, &probe); err != nil {
		return mediaConversionResult{}, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	result := mediaConversionResult{
		InputFormat: firstCSVItem(probe.Format.FormatName),
	}
	if dur, err := strconv.ParseFloat(strings.TrimSpace(probe.Format.Duration), 64); err == nil {
		result.DurationSeconds = dur
	}
	for _, stream := range probe.Streams {
		switch stream.CodecType {
		case "video":
			result.MediaType = "video"
			result.VideoCodec = stream.CodecName
			if stream.Width > 0 {
				result.Width = stream.Width
			}
			if stream.Height > 0 {
				result.Height = stream.Height
			}
		case "audio":
			if result.MediaType == "" {
				result.MediaType = "audio"
			}
			result.AudioCodec = stream.CodecName
			if stream.Channels > 0 {
				result.Channels = stream.Channels
			}
			if rate, err := strconv.Atoi(strings.TrimSpace(stream.SampleRate)); err == nil {
				result.SampleRate = rate
			}
		}
	}

	return result, nil
}

func inspectMediaBinary(ctx context.Context, configuredPath string, candidates []string, versionArgs []string) mediaConversionHealthBinary {
	path, err := resolveConfiguredBinary(configuredPath, candidates)
	if err != nil {
		return mediaConversionHealthBinary{Available: false, Message: err.Error()}
	}
	output, runErr := mediaConversionRunCommand(ctx, path, versionArgs)
	if runErr != nil {
		return mediaConversionHealthBinary{
			Available: false,
			Path:      path,
			Message:   trimCommandOutput(runErr, output),
		}
	}
	return mediaConversionHealthBinary{
		Available: true,
		Path:      path,
		Version:   firstOutputLine(output),
	}
}

func inspectImageMagickBinary(ctx context.Context, configuredPath string) mediaConversionHealthBinary {
	path, err := resolveConfiguredImageMagick(configuredPath)
	if err != nil {
		return mediaConversionHealthBinary{Available: false, Message: err.Error()}
	}
	output, runErr := mediaConversionRunCommand(ctx, path, []string{"-version"})
	if runErr != nil {
		return mediaConversionHealthBinary{
			Available: false,
			Path:      path,
			Message:   trimCommandOutput(runErr, output),
		}
	}
	return mediaConversionHealthBinary{
		Available: true,
		Path:      path,
		Version:   firstOutputLine(output),
	}
}

func resolveConfiguredBinary(configuredPath string, candidates []string) (string, error) {
	return mediabins.ResolveWith(mediaConversionLookPath, configuredPath, candidates)
}

func resolveConfiguredImageMagick(configuredPath string) (string, error) {
	return mediabins.ResolveImageMagickWith(mediaConversionLookPath, configuredPath)
}

func resolveFFprobeBinary(cfg *config.MediaConversionConfig) (string, error) {
	if cfg != nil && strings.TrimSpace(cfg.FFmpegPath) != "" {
		ffmpegPath := strings.TrimSpace(cfg.FFmpegPath)
		base := filepath.Base(ffmpegPath)
		dir := filepath.Dir(ffmpegPath)
		ffprobeName := strings.Replace(strings.ToLower(base), "ffmpeg", "ffprobe", 1)
		if ffprobeName != base {
			if candidate := filepath.Join(dir, ffprobeName); mediaConversionFileExists(candidate) {
				return candidate, nil
			}
		}
		ext := filepath.Ext(base)
		if ext == "" && runtime.GOOS == "windows" {
			if candidate := filepath.Join(dir, "ffprobe.exe"); mediaConversionFileExists(candidate) {
				return candidate, nil
			}
		}
	}
	candidates := []string{"ffprobe"}
	if runtime.GOOS == "windows" {
		candidates = []string{"ffprobe", "ffprobe.exe"}
	}
	return resolveConfiguredBinary("", candidates)
}

// mediaTypeForOperation maps a conversion operation to its media type. An empty
// result means the operation has no format allowlist.
func mediaTypeForOperation(op string) string {
	switch strings.ToLower(strings.TrimSpace(op)) {
	case "audio_convert":
		return "audio"
	case "video_convert":
		return "video"
	case "image_convert":
		return "image"
	default:
		return ""
	}
}

// resolveConversionTarget resolves one conversion into a destination, a staged
// file and a target format. It is the only place that decides which file the
// converter writes and which format is produced.
func resolveConversionTarget(stagingDir, inputFile, outputFile, outputFormat, mediaType string) (conversionTarget, error) {
	format := normalizedOutputFormat(outputFormat)
	outputFile = strings.TrimSpace(outputFile)

	if outputFile != "" {
		ext := normalizedOutputFormat(strings.TrimPrefix(filepath.Ext(outputFile), "."))
		switch {
		case format == "":
			format = ext
		case ext != "" && ext != format:
			return conversionTarget{}, fmt.Errorf("output_file %q conflicts with output_format %q: use one consistent target format", outputFile, format)
		}
	}
	if format == "" {
		return conversionTarget{}, fmt.Errorf("output_file or output_format is required")
	}
	if mediaType != "" {
		supported := conversionFormatAllowlist[mediaType]
		if !slices.Contains(supported, format) {
			return conversionTarget{}, fmt.Errorf("output format %q is not supported for %s conversion (supported: %s)", format, mediaType, strings.Join(supported, ", "))
		}
	}

	destination := outputFile
	if destination == "" {
		destination = strings.TrimSuffix(inputFile, filepath.Ext(inputFile)) + "." + preferredExtension(format)
	} else if filepath.Ext(destination) == "" {
		destination += "." + preferredExtension(format)
	}
	if err := validateMediaConversionOutputFile(destination); err != nil {
		return conversionTarget{}, err
	}
	// Prevent accidental in-place overwrite which would destroy the source file
	if mediaConversionSameFile(inputFile, destination) {
		return conversionTarget{}, fmt.Errorf("output file must be different from input file")
	}
	return conversionTarget{
		Destination: destination,
		StagedPath:  filepath.Join(stagingDir, "output."+preferredExtension(format)),
		Format:      format,
	}, nil
}

// mediaConversionSameFile reports whether two paths name the same file. On a
// case-insensitive filesystem "Input.mp3" and "input.mp3" are the same file, so
// the comparison ignores the case there and keeps it everywhere else.
func mediaConversionSameFile(left, right string) bool {
	cleanedLeft := filepath.Clean(left)
	cleanedRight := filepath.Clean(right)
	if cleanedLeft == cleanedRight {
		return true
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(cleanedLeft, cleanedRight)
	}
	return false
}

// mediaConversionDemuxerForPath returns the ffmpeg demuxer for a known input
// extension. An unknown extension yields no format whitelist, so that unusual
// but legitimate local inputs keep working.
func mediaConversionDemuxerForPath(path string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	return mediaConversionDemuxers[ext]
}

func validateMediaConversionOutputFile(outputFile string) error {
	trimmed := strings.TrimSpace(outputFile)
	if trimmed == "" {
		return fmt.Errorf("output_file is required")
	}
	lower := strings.ToLower(trimmed)
	for _, prefix := range []string{
		"|", "@", "-",
		"msl:", "ephemeral:", "https:", "http:", "caption:", "label:", "vid:", "json:", "fd:",
	} {
		if strings.HasPrefix(lower, prefix) {
			return fmt.Errorf("unsafe output_file prefix %q is not allowed", prefix)
		}
	}
	return nil
}

func normalizedOutputFormat(raw string) string {
	format := strings.ToLower(strings.TrimSpace(raw))
	switch format {
	case "jpg":
		return "jpeg"
	case "tif":
		return "tiff"
	default:
		return format
	}
}

func preferredExtension(format string) string {
	switch format {
	case "jpeg":
		return "jpg"
	case "tiff":
		return "tiff"
	default:
		return format
	}
}

func ffmpegScaleArg(width, height int) string {
	switch {
	case width > 0 && height > 0:
		return fmt.Sprintf("scale=%d:%d", width, height)
	case width > 0:
		return fmt.Sprintf("scale=%d:-2", width)
	case height > 0:
		return fmt.Sprintf("scale=-2:%d", height)
	default:
		return ""
	}
}

func imageMagickResizeArg(width, height int) string {
	switch {
	case width > 0 && height > 0:
		return fmt.Sprintf("%dx%d", width, height)
	case width > 0:
		return fmt.Sprintf("%dx", width)
	case height > 0:
		return fmt.Sprintf("x%d", height)
	default:
		return ""
	}
}

func checkMediaFileSize(cfg *config.MediaConversionConfig, path string) error {
	if cfg == nil || cfg.MaxFileSizeMB <= 0 {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	maxBytes := int64(cfg.MaxFileSizeMB) * 1024 * 1024
	if info.Size() > maxBytes {
		return fmt.Errorf("file size %s exceeds maximum allowed %d MB", formatFileSize(info.Size()), cfg.MaxFileSizeMB)
	}
	return nil
}

func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := "KMGTPE"
	if exp >= len(units) {
		return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), units[len(units)-1])
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), units[exp])
}

func mediaConversionTimeout(cfg *config.MediaConversionConfig) time.Duration {
	if cfg == nil || cfg.TimeoutSeconds <= 0 {
		return 120 * time.Second
	}
	return time.Duration(cfg.TimeoutSeconds) * time.Second
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("cannot create output directory: %w", err)
	}
	return nil
}

// mediaConversionContextReader stops a staging copy as soon as the caller
// context ends, so a cancelled agent turn does not keep copying.
type mediaConversionContextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r mediaConversionContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

// mediaConversionRunError renders a failed converter run. An expired deadline
// is reported as a timeout so that the agent can tell it apart from a codec
// error, and a cancelled run is reported as a cancellation.
func mediaConversionRunError(ctx context.Context, cfg *config.MediaConversionConfig, runErr error, output []byte, operation, tool string) string {
	message := trimCommandOutput(runErr, output)
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		message = fmt.Sprintf("conversion timed out after %s", mediaConversionTimeout(cfg))
	case errors.Is(ctx.Err(), context.Canceled):
		message = "conversion was cancelled"
	}
	return mediaConversionJSON(mediaConversionResult{
		Status:    "error",
		Message:   message,
		Operation: operation,
		Tool:      tool,
	})
}

func trimCommandOutput(runErr error, output []byte) string {
	msg := strings.TrimSpace(string(output))
	if msg == "" {
		return runErr.Error()
	}
	lines := strings.Split(msg, "\n")
	if len(lines) > 10 {
		lines = lines[len(lines)-10:]
	}
	return strings.Join(lines, "\n")
}

func firstOutputLine(output []byte) string {
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(lines[0])
}

func imageMagickToolName(path string) string {
	base := strings.ToLower(filepath.Base(path))
	if strings.Contains(base, "magick") {
		return "imagemagick"
	}
	return base
}

func mediaConversionFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func firstCSVItem(raw string) string {
	part := strings.TrimSpace(strings.Split(raw, ",")[0])
	return strings.ToLower(part)
}
