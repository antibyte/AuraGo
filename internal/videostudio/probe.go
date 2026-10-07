package videostudio

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	allowedProtocols = "file,pipe"
	allowedDemuxers  = "mov,matroska,webm,avi,asf,flv,mpeg,mpegts,wav,mp3,aac,flac,ogg,opus,ac3,ape,amr,au,rm,rv,mxf,ivf,image2,png_pipe,jpeg_pipe,webp_pipe,bmp_pipe,tiff_pipe,gif"
)

type ffprobeResult struct {
	Streams []struct {
		CodecType   string `json:"codec_type"`
		CodecName   string `json:"codec_name"`
		Width       int    `json:"width"`
		Height      int    `json:"height"`
		Duration    string `json:"duration"`
		Disposition struct {
			AttachedPicture int `json:"attached_pic"`
		} `json:"disposition"`
	} `json:"streams"`
	Format struct {
		Name     string `json:"format_name"`
		Duration string `json:"duration"`
	} `json:"format"`
}

// Probe inspects one private staged media file using the FFprobe paired with
// binaryPath, which may be either the configured FFmpeg or FFprobe path.
func Probe(ctx context.Context, binaryPath, input string) (Asset, error) {
	path, err := ensureRegularLocalFile(input)
	if err != nil {
		return Asset{}, fmt.Errorf("open staged media: %w", err)
	}
	probePath, err := resolveProbeFromConfiguredPath(binaryPath)
	if err != nil {
		return Asset{}, err
	}
	probeCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	args := []string{
		"-hide_banner", "-v", "error",
		"-protocol_whitelist", allowedProtocols,
		"-format_whitelist", allowedDemuxers,
		"-show_format", "-show_streams", "-of", "json", "-i", path,
	}
	output, err := runSmallCommand(probeCtx, probePath, args, 8<<20)
	if err != nil {
		return Asset{}, fmt.Errorf("probe staged media: %w", err)
	}
	var result ffprobeResult
	if err := json.Unmarshal(output, &result); err != nil {
		return Asset{}, fmt.Errorf("decode FFprobe output: %w", err)
	}

	asset := Asset{Name: filepath.Base(path)}
	hasVideo := false
	duration := parseSeconds(result.Format.Duration)
	for _, stream := range result.Streams {
		if stream.CodecType == "video" && stream.Disposition.AttachedPicture != 0 {
			continue
		}
		streamDuration := parseSeconds(stream.Duration)
		if streamDuration > duration {
			duration = streamDuration
		}
		switch stream.CodecType {
		case "audio":
			asset.HasAudio = true
		case "video":
			hasVideo = true
			if asset.Width == 0 && stream.Width > 0 && stream.Height > 0 {
				asset.Width, asset.Height = stream.Width, stream.Height
			}
		}
	}
	if hasVideo && isStillImageFormat(result.Format.Name) {
		asset.Kind = AssetImage
	} else if hasVideo {
		asset.Kind = AssetVideo
	} else if asset.HasAudio {
		asset.Kind = AssetAudio
	} else {
		return Asset{}, fmt.Errorf("staged media has no supported audio or video stream")
	}
	if asset.Kind != AssetImage {
		if duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) {
			return Asset{}, fmt.Errorf("staged media has no valid duration")
		}
		frames := math.Ceil(duration*FramesPerSecond - 1e-7)
		if frames > float64(int(^uint(0)>>1)) {
			return Asset{}, fmt.Errorf("staged media duration is too large")
		}
		asset.DurationFrames = int(frames)
		if asset.DurationFrames <= 0 {
			asset.DurationFrames = 1
		}
	}
	if asset.Kind != AssetAudio && !validSourceDimensions(asset.Width, asset.Height) {
		return Asset{}, fmt.Errorf("staged media dimensions exceed the supported source limit")
	}
	return asset, nil
}

// PreparePreview writes a complete bounded 540p, 30 fps H.264 proxy for a local video.
func PreparePreview(ctx context.Context, ffmpegPath, input, output string) error {
	inputPath, err := ensureRegularLocalFile(input)
	if err != nil {
		return fmt.Errorf("open staged preview source: %w", err)
	}
	ffmpeg, err := resolveBinary(ffmpegPath, "ffmpeg")
	if err != nil {
		return err
	}
	source, err := Probe(ctx, ffmpeg, inputPath)
	if err != nil {
		return fmt.Errorf("inspect preview source: %w", err)
	}
	if source.Kind != AssetVideo {
		return fmt.Errorf("preview source must contain video")
	}
	outputPath, err := validateNewOutput(output, inputPath)
	if err != nil {
		return err
	}
	previewCtx, cancel := context.WithTimeout(ctx, 4*time.Hour)
	defer cancel()
	args := []string{
		"-hide_banner", "-nostdin", "-loglevel", "error",
		"-protocol_whitelist", allowedProtocols,
		"-format_whitelist", allowedDemuxers,
		"-i", inputPath,
		"-map", "0:V:0", "-map", "0:a:0?",
		"-vf", "scale=960:540:force_original_aspect_ratio=decrease:force_divisible_by=2,fps=30",
		"-threads", "2", "-filter_threads", "2",
		"-c:v", "libx264", "-preset", "ultrafast", "-crf", "32",
		"-pix_fmt", "yuv420p", "-c:a", "aac", "-b:a", "64k", "-ar", "48000",
		"-movflags", "+faststart", "-fs", "536870912", "-f", "mp4", outputPath,
	}
	if _, err := runSmallCommand(previewCtx, ffmpeg, args, 1<<20); err != nil {
		_ = os.Remove(outputPath)
		return fmt.Errorf("create media preview: %w", err)
	}
	if err := ensureOutputSize(outputPath, 536870912); err != nil {
		_ = os.Remove(outputPath)
		return err
	}
	preview, err := Probe(ctx, ffmpeg, outputPath)
	if err != nil {
		_ = os.Remove(outputPath)
		return fmt.Errorf("verify media preview: %w", err)
	}
	if preview.Kind != AssetVideo || preview.DurationFrames < source.DurationFrames-2 || preview.DurationFrames > source.DurationFrames+2 || preview.HasAudio != source.HasAudio {
		_ = os.Remove(outputPath)
		return fmt.Errorf("media preview is incomplete")
	}
	return nil
}

func ensureRegularLocalFile(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", fmt.Errorf("path must be a regular staged file")
	}
	return abs, nil
}

func validateNewOutput(output, input string) (string, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return "", fmt.Errorf("output path is required")
	}
	path, err := filepath.Abs(output)
	if err != nil {
		return "", fmt.Errorf("resolve output path: %w", err)
	}
	if filepath.Clean(path) == filepath.Clean(input) {
		return "", fmt.Errorf("output must not overwrite its input")
	}
	if _, err := os.Lstat(path); err == nil {
		return "", fmt.Errorf("output already exists")
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect output path: %w", err)
	}
	parent, err := os.Stat(filepath.Dir(path))
	if err != nil || !parent.IsDir() {
		return "", fmt.Errorf("output directory does not exist")
	}
	return path, nil
}

func ensureOutputSize(path string, limit int64) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect generated media: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > limit {
		_ = os.Remove(path)
		return fmt.Errorf("generated media exceeds the output limit")
	}
	return nil
}

func validSourceDimensions(width, height int) bool {
	return width > 0 && height > 0 && width <= MaxSourceDimension && height <= MaxSourceDimension && int64(width)*int64(height) <= MaxSourcePixels
}

func parseSeconds(value string) float64 {
	seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || seconds < 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return 0
	}
	return seconds
}

func isStillImageFormat(format string) bool {
	for _, name := range strings.Split(format, ",") {
		switch strings.TrimSpace(name) {
		case "image2", "png_pipe", "jpeg_pipe", "webp_pipe", "bmp_pipe", "tiff_pipe", "gif":
			return true
		}
	}
	return false
}
