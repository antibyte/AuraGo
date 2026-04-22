package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

var mediaConversionLookPath = exec.LookPath

var mediaConversionRunCommand = func(ctx context.Context, name string, args []string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
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
	Status       string                      `json:"status"`
	Message      string                      `json:"message"`
	FFmpeg       mediaConversionHealthBinary `json:"ffmpeg"`
	ImageMagick  mediaConversionHealthBinary `json:"imagemagick"`
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
	b, _ := json.Marshal(r)
	return string(b)
}

func ExecuteMediaConversion(workspaceDir string, cfg *config.MediaConversionConfig, req MediaConversionRequest) string {
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

	switch op {
	case "audio_convert":
		return executeFFmpegConversion(cfg, req, "audio")
	case "video_convert":
		return executeFFmpegConversion(cfg, req, "video")
	case "image_convert":
		return executeImageMagickConversion(cfg, req)
	case "info":
		return executeMediaInfo(cfg, req)
	default:
		return mediaConversionJSON(mediaConversionResult{
			Status:    "error",
			Message:   "unknown operation: use audio_convert, video_convert, image_convert, or info",
			Operation: op,
		})
	}
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

func executeFFmpegConversion(cfg *config.MediaConversionConfig, req MediaConversionRequest, mediaType string) string {
	if strings.TrimSpace(req.FilePath) == "" {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: "file_path is required", Operation: req.Operation})
	}
	if _, err := os.Stat(req.FilePath); err != nil {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: fmt.Sprintf("cannot access input file: %v", err), Operation: req.Operation})
	}

	ffmpegPath, err := resolveConfiguredBinary(cfg.FFmpegPath, []string{"ffmpeg"})
	if err != nil {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: err.Error(), Operation: req.Operation})
	}

	outputFile, outputFormat, err := resolveConversionOutput(req.FilePath, req.OutputFile, req.OutputFormat)
	if err != nil {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: err.Error(), Operation: req.Operation})
	}
	req.OutputFile = outputFile

	args := []string{"-y", "-i", req.FilePath}
	switch mediaType {
	case "audio":
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
	args = append(args, req.OutputFile)

	if err := ensureParentDir(req.OutputFile); err != nil {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: err.Error(), Operation: req.Operation})
	}

	ctx, cancel := context.WithTimeout(context.Background(), mediaConversionTimeout(cfg))
	defer cancel()

	output, runErr := mediaConversionRunCommand(ctx, ffmpegPath, args)
	if runErr != nil {
		return mediaConversionJSON(mediaConversionResult{
			Status:    "error",
			Message:   trimCommandOutput(runErr, output),
			Operation: req.Operation,
			Tool:      "ffmpeg",
		})
	}

	return mediaConversionJSON(mediaConversionResult{
		Status:       "success",
		Message:      fmt.Sprintf("%s converted successfully", mediaType),
		Operation:    req.Operation,
		File:         req.OutputFile,
		Tool:         "ffmpeg",
		MediaType:    mediaType,
		OutputFormat: outputFormat,
	})
}

func executeImageMagickConversion(cfg *config.MediaConversionConfig, req MediaConversionRequest) string {
	if strings.TrimSpace(req.FilePath) == "" {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: "file_path is required", Operation: req.Operation})
	}
	if _, err := os.Stat(req.FilePath); err != nil {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: fmt.Sprintf("cannot access input file: %v", err), Operation: req.Operation})
	}

	imageMagickPath, err := resolveConfiguredImageMagick(cfg.ImageMagickPath)
	if err != nil {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: err.Error(), Operation: req.Operation})
	}

	outputFile, outputFormat, err := resolveConversionOutput(req.FilePath, req.OutputFile, req.OutputFormat)
	if err != nil {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: err.Error(), Operation: req.Operation})
	}
	req.OutputFile = outputFile

	if err := ensureParentDir(req.OutputFile); err != nil {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: err.Error(), Operation: req.Operation})
	}

	args := []string{req.FilePath}
	if geometry := imageMagickResizeArg(req.Width, req.Height); geometry != "" {
		args = append(args, "-resize", geometry)
	}
	if req.QualityPct > 0 {
		args = append(args, "-quality", strconv.Itoa(req.QualityPct))
	}
	args = append(args, req.OutputFile)

	ctx, cancel := context.WithTimeout(context.Background(), mediaConversionTimeout(cfg))
	defer cancel()
	output, runErr := mediaConversionRunCommand(ctx, imageMagickPath, args)
	if runErr != nil {
		return mediaConversionJSON(mediaConversionResult{
			Status:    "error",
			Message:   trimCommandOutput(runErr, output),
			Operation: req.Operation,
			Tool:      imageMagickToolName(imageMagickPath),
		})
	}

	return mediaConversionJSON(mediaConversionResult{
		Status:       "success",
		Message:      "image converted successfully",
		Operation:    req.Operation,
		File:         req.OutputFile,
		Tool:         imageMagickToolName(imageMagickPath),
		MediaType:    "image",
		OutputFormat: outputFormat,
	})
}

func executeMediaInfo(cfg *config.MediaConversionConfig, req MediaConversionRequest) string {
	if strings.TrimSpace(req.FilePath) == "" {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: "file_path is required", Operation: req.Operation})
	}

	ext := strings.ToLower(filepath.Ext(req.FilePath))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".tif", ".tiff", ".webp":
		return imageInfoResult(req)
	default:
		return ffprobeInfoResult(cfg, req)
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

func ffprobeInfoResult(cfg *config.MediaConversionConfig, req MediaConversionRequest) string {
	ffprobePath, err := resolveFFprobeBinary(cfg)
	if err != nil {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: err.Error(), Operation: req.Operation})
	}

	ctx, cancel := context.WithTimeout(context.Background(), mediaConversionTimeout(cfg))
	defer cancel()
	output, runErr := mediaConversionRunCommand(ctx, ffprobePath, []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		req.FilePath,
	})
	if runErr != nil {
		return mediaConversionJSON(mediaConversionResult{
			Status:    "error",
			Message:   trimCommandOutput(runErr, output),
			Operation: req.Operation,
			Tool:      "ffprobe",
		})
	}

	var probe ffprobeOutput
	if err := json.Unmarshal(output, &probe); err != nil {
		return mediaConversionJSON(mediaConversionResult{Status: "error", Message: fmt.Sprintf("failed to parse ffprobe output: %v", err), Operation: req.Operation})
	}

	result := mediaConversionResult{
		Status:      "success",
		Message:     "media info collected",
		Operation:   req.Operation,
		Tool:        "ffprobe",
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

	return mediaConversionJSON(result)
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
	if strings.TrimSpace(configuredPath) != "" {
		path := strings.TrimSpace(configuredPath)
		if filepath.IsAbs(path) {
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
		if found, err := mediaConversionLookPath(path); err == nil {
			return found, nil
		}
		return "", fmt.Errorf("binary %q was not found", path)
	}
	for _, candidate := range candidates {
		if found, err := mediaConversionLookPath(candidate); err == nil {
			return found, nil
		}
	}
	return "", fmt.Errorf("required binary was not found: %s", strings.Join(candidates, ", "))
}

func resolveConfiguredImageMagick(configuredPath string) (string, error) {
	candidates := []string{"magick", "convert"}
	if runtime.GOOS == "windows" {
		candidates = []string{"magick", "magick.exe", "convert"}
	}
	return resolveConfiguredBinary(configuredPath, candidates)
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

func resolveConversionOutput(inputFile, outputFile, outputFormat string) (string, string, error) {
	format := normalizedOutputFormat(outputFormat)
	if outputFile == "" {
		if format == "" {
			return "", "", fmt.Errorf("output_file or output_format is required")
		}
		outputFile = strings.TrimSuffix(inputFile, filepath.Ext(inputFile)) + "." + preferredExtension(format)
	}
	if format == "" {
		format = normalizedOutputFormat(strings.TrimPrefix(filepath.Ext(outputFile), "."))
	}
	if format == "" {
		return "", "", fmt.Errorf("could not determine output format")
	}
	return outputFile, format, nil
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
