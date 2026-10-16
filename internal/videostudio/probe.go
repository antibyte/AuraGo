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
		CodecType    string `json:"codec_type"`
		CodecName    string `json:"codec_name"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		Duration     string `json:"duration"`
		AvgFrameRate string `json:"avg_frame_rate"`
		RFrameRate   string `json:"r_frame_rate"`
		Disposition  struct {
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
	info, err := os.Stat(path)
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

	asset := Asset{Name: filepath.Base(path), SizeBytes: info.Size()}
	hasVideo := false
	audioCodec := ""
	videoDuration := 0.0
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
			if audioCodec == "" {
				audioCodec = safeCodecName(stream.CodecName)
			}
		case "video":
			if !hasVideo {
				asset.Codec = safeCodecName(stream.CodecName)
				asset.SourceFPS = normalizedFrameRate(stream.AvgFrameRate, stream.RFrameRate)
				videoDuration = streamDuration
			}
			hasVideo = true
			if asset.Width == 0 && stream.Width > 0 && stream.Height > 0 {
				asset.Width, asset.Height = stream.Width, stream.Height
			}
		}
	}
	if !hasVideo {
		asset.Codec = audioCodec
	}
	if hasVideo && isStillImageFormat(result.Format.Name) {
		asset.Kind = AssetImage
		asset.SourceFPS = ""
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
		if duration > 366*24*3600 {
			return Asset{}, fmt.Errorf("staged media duration is too large")
		}
		asset.DurationUs = int64(math.Round(duration * 1e6))
		asset.DurationFrames = Rate{Num: FramesPerSecond, Den: 1}.FramesAt(asset.DurationUs)
		if asset.Kind == AssetVideo {
			asset.VideoDurationUs = asset.DurationUs
			if videoDuration > 0 && videoDuration < duration {
				asset.VideoDurationUs = int64(math.Round(videoDuration * 1e6))
			}
		}
	}
	if asset.Kind != AssetAudio && !validSourceDimensions(asset.Width, asset.Height) {
		return Asset{}, fmt.Errorf("staged media dimensions exceed the supported source limit")
	}
	return asset, nil
}

// MaxPreviewBytes bounds one 540p preview proxy.
const MaxPreviewBytes = int64(2 << 30)

// PreparePreview writes a complete bounded 540p, 30 fps H.264 proxy for a local video with libx264.
func PreparePreview(ctx context.Context, ffmpegPath, input, output string) error {
	return PreparePreviewWith(ctx, ffmpegPath, input, output, Encoder{})
}

// PreparePreviewWith encodes on the GPU when encoder is a hardware encoder and falls back to software for this
// preview when that fails. Decoding and scaling stay on the CPU: reading VAAPI-decoded frames back for the
// scaler made 1080p previews about three times slower than software on an AMD iGPU, while CPU decoding with
// VAAPI encoding was faster than software and still used less CPU.
func PreparePreviewWith(ctx context.Context, ffmpegPath, input, output string, encoder Encoder) error {
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
	err = runPreview(ctx, ffmpeg, inputPath, outputPath, encoder)
	if err != nil && encoder.Hardware() && ctx.Err() == nil {
		_ = os.Remove(outputPath)
		err = runPreview(ctx, ffmpeg, inputPath, outputPath, Encoder{})
	}
	if err != nil {
		_ = os.Remove(outputPath)
		return fmt.Errorf("create media preview: %w", err)
	}
	if err := ensureOutputSize(outputPath, MaxPreviewBytes); err != nil {
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

func runPreview(ctx context.Context, ffmpeg, input, output string, encoder Encoder) error {
	previewCtx, cancel := context.WithTimeout(ctx, 4*time.Hour)
	defer cancel()
	args := append([]string{"-hide_banner", "-nostdin", "-loglevel", "error"}, encoder.globalArgs()...)
	args = append(args,
		"-protocol_whitelist", allowedProtocols, "-format_whitelist", allowedDemuxers, "-i", input,
		"-map", "0:V:0", "-map", "0:a:0?",
		"-vf", "scale=960:540:force_original_aspect_ratio=decrease:force_divisible_by=2,fps=30"+encoder.filterTail(),
		"-threads", "2", "-filter_threads", "2")
	if encoder.Hardware() {
		proxy := encoder
		proxy.Quality = 32
		args = append(args, proxy.videoArgs(Rate{Num: 30, Den: 1})...)
	} else {
		args = append(args, "-c:v", "libx264", "-preset", "ultrafast", "-crf", "32", "-pix_fmt", "yuv420p")
	}
	args = append(args, "-c:a", "aac", "-b:a", "64k", "-ar", "48000",
		"-movflags", "+faststart", "-fs", strconv.FormatInt(MaxPreviewBytes, 10), "-f", "mp4", output)
	_, err := runSmallCommand(previewCtx, ffmpeg, args, 1<<20)
	return err
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

// PosterLongestSide is the longest side of a media poster in pixels.
const PosterLongestSide = 320

// maxPosterBytes bounds one poster JPEG.
const maxPosterBytes = int64(1 << 20)

// WritePoster writes a JPEG poster: one frame of a video (one second in, or the first frame when the clip is shorter
// than two seconds) or the picture of a still image. The longest side is PosterLongestSide pixels. Audio has no
// picture and therefore no poster.
func WritePoster(ctx context.Context, ffmpegPath, input, output string, asset Asset) error {
	if asset.Kind != AssetVideo && asset.Kind != AssetImage {
		return fmt.Errorf("only video and image assets have posters")
	}
	inputPath, err := ensureRegularLocalFile(input)
	if err != nil {
		return fmt.Errorf("open poster source: %w", err)
	}
	ffmpeg, err := resolveBinary(ffmpegPath, "ffmpeg")
	if err != nil {
		return err
	}
	outputPath, err := validateNewOutput(output, inputPath)
	if err != nil {
		return err
	}
	seek := "1"
	if asset.Kind == AssetImage || (asset.DurationUs > 0 && asset.DurationUs < 2_000_000) {
		seek = "0"
	}
	err = runPoster(ctx, ffmpeg, inputPath, outputPath, seek)
	if err != nil && seek != "0" && ctx.Err() == nil {
		_ = os.Remove(outputPath)
		err = runPoster(ctx, ffmpeg, inputPath, outputPath, "0")
	}
	if err != nil {
		_ = os.Remove(outputPath)
		return fmt.Errorf("create media poster: %w", err)
	}
	if err := ensureOutputSize(outputPath, maxPosterBytes); err != nil {
		_ = os.Remove(outputPath)
		return err
	}
	return nil
}

func runPoster(ctx context.Context, ffmpeg, input, output, seek string) error {
	posterCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	args := []string{
		"-hide_banner", "-nostdin", "-loglevel", "error",
		"-protocol_whitelist", allowedProtocols, "-format_whitelist", allowedDemuxers,
		"-ss", seek, "-i", input,
		"-map", "0:V:0", "-frames:v", "1",
		"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease:force_divisible_by=2", PosterLongestSide, PosterLongestSide),
		"-q:v", "4", "-f", "image2", output,
	}
	if _, err := runSmallCommand(posterCtx, ffmpeg, args, 1<<20); err != nil {
		return err
	}
	info, err := os.Stat(output)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 {
		return fmt.Errorf("poster frame is missing")
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

// normalizedFrameRate returns the first valid rational frame rate in lowest terms ("30000/1001").
func normalizedFrameRate(values ...string) string {
	for _, value := range values {
		num, den, ok := parseRational(value)
		if !ok {
			continue
		}
		divisor := gcd(num, den)
		num, den = num/divisor, den/divisor
		if num > 9_999_999 || den > 9_999_999 {
			continue
		}
		return fmt.Sprintf("%d/%d", num, den)
	}
	return ""
}

func parseRational(value string) (int64, int64, bool) {
	left, right, found := strings.Cut(strings.TrimSpace(value), "/")
	if !found {
		right = "1"
	}
	num, numErr := strconv.ParseInt(strings.TrimSpace(left), 10, 64)
	den, denErr := strconv.ParseInt(strings.TrimSpace(right), 10, 64)
	if numErr != nil || denErr != nil || num <= 0 || den <= 0 {
		return 0, 0, false
	}
	return num, den, true
}

func gcd(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
