package render

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/videostudio"
)

const (
	ffmpegWriteTimeout = 30 * time.Second
	ffmpegWaitAfterEnd = 120 * time.Second
	ffmpegStderrKeep   = 8 << 10
	// hardwareSettle is how long a hardware encoder gets after the first frame to fail: FFmpeg opens the
	// encoder only once it has seen a frame, and a failure must surface before the page renders on.
	hardwareSettle      = 1500 * time.Millisecond
	audioRate           = 48000
	audioBitrate        = "192k"
	silentAudioLavfiSrc = "anullsrc=r=48000:cl=stereo"
)

// FFmpegConfig describes one frame-input encode.
type FFmpegConfig struct {
	Path      string
	Encoder   videostudio.Encoder
	Transport string // png, webp, raw or screenshot (PNG frames)
	Width     int
	Height    int
	FPS       int
	Frames    int
	Output    string
	AudioPath string // "" keeps anullsrc with -t; else the mix WAV as input 1
}

// BuildFFmpegArgs returns the FFmpeg arguments of a render: frames on stdin (video), the mix WAV or
// a silent track from anullsrc as the AAC audio, BT.709
// limited-range YUV 4:2:0 through an explicit scale (FFmpeg before 7.1 would otherwise convert with
// BT.601 while the stream is tagged BT.709), the encoder's filter tail, and +faststart. Both inputs
// carry protocol and format whitelists.
func BuildFFmpegArgs(cfg FFmpegConfig) ([]string, error) {
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.FPS <= 0 || cfg.Frames <= 0 || cfg.Output == "" {
		return nil, errors.New("storymaker: incomplete ffmpeg configuration")
	}
	fps := strconv.Itoa(cfg.FPS)
	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error", "-nostats"}
	args = append(args, cfg.Encoder.GlobalArgs()...)
	switch cfg.Transport {
	case TransportPNG, TransportScreenshot:
		args = append(args, "-protocol_whitelist", "file,pipe", "-format_whitelist", "image2pipe,png_pipe",
			"-f", "image2pipe", "-c:v", "png", "-framerate", fps, "-i", "pipe:0")
	case TransportWebP:
		args = append(args, "-protocol_whitelist", "file,pipe", "-format_whitelist", "image2pipe,webp_pipe",
			"-f", "image2pipe", "-c:v", "webp", "-framerate", fps, "-i", "pipe:0")
	case TransportRaw:
		args = append(args, "-protocol_whitelist", "file,pipe", "-format_whitelist", "rawvideo",
			"-f", "rawvideo", "-pixel_format", "rgba", "-video_size", fmt.Sprintf("%dx%d", cfg.Width, cfg.Height),
			"-framerate", fps, "-i", "pipe:0")
	default:
		return nil, fmt.Errorf("storymaker: unknown transport %q", cfg.Transport)
	}
	if cfg.AudioPath != "" {
		args = append(args, "-protocol_whitelist", "file,pipe", "-format_whitelist", "wav", "-i", cfg.AudioPath)
	} else {
		seconds := strconv.FormatFloat(float64(cfg.Frames)/float64(cfg.FPS), 'f', 6, 64)
		args = append(args, "-protocol_whitelist", "file,pipe", "-format_whitelist", "lavfi",
			"-f", "lavfi", "-t", seconds, "-i", silentAudioLavfiSrc)
	}
	args = append(args, "-map", "0:v:0", "-map", "1:a:0",
		"-vf", "scale=out_color_matrix=bt709:out_range=tv,format=yuv420p"+cfg.Encoder.FilterTail())
	args = append(args, cfg.Encoder.VideoArgs(videostudio.Rate{Num: int64(cfg.FPS), Den: 1})...)
	args = append(args, "-c:a", "aac", "-b:a", audioBitrate, "-ar", strconv.Itoa(audioRate), "-ac", "2",
		"-movflags", "+faststart", "-f", "mp4", "-y", cfg.Output)
	return args, nil
}

// FFmpeg is a running frame-input encode.
type FFmpeg struct {
	cmd   *exec.Cmd
	stdin interface {
		Write([]byte) (int, error)
		Close() error
	}
	output string
	settle time.Duration
	frames int
	stderr *tailBuffer
	done   chan struct{}

	mu        sync.Mutex
	waitErr   error
	stdinOnce sync.Once
}

// StartFFmpeg starts the process. ctx ends it.
func StartFFmpeg(ctx context.Context, cfg FFmpegConfig) (*FFmpeg, error) {
	args, err := BuildFFmpegArgs(cfg)
	if err != nil {
		return nil, err
	}
	_ = os.Remove(cfg.Output)
	cmd := exec.CommandContext(ctx, cfg.Path, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	f := &FFmpeg{cmd: cmd, stdin: stdin, output: cfg.Output, stderr: &tailBuffer{limit: ffmpegStderrKeep}, done: make(chan struct{})}
	if cfg.Encoder.Hardware() {
		f.settle = hardwareSettle
	}
	cmd.Stderr = f.stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start FFmpeg: %w", err)
	}
	go func() {
		err := cmd.Wait()
		f.mu.Lock()
		f.waitErr = err
		f.mu.Unlock()
		close(f.done)
	}()
	return f, nil
}

// Done is closed when the process has exited.
func (f *FFmpeg) Done() <-chan struct{} { return f.done }

// Err is the exit error once Done is closed.
func (f *FFmpeg) Err() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.waitErr
}

// Message is the tail of FFmpeg's error output with the output path and the staging folder scrubbed.
func (f *FFmpeg) Message() string {
	text := strings.TrimSpace(f.stderr.String())
	if f.output != "" {
		text = strings.ReplaceAll(text, f.output, "<output>")
		text = scrubStaging(text, filepath.Dir(f.output))
	}
	return clip(text, 600)
}

// scrubStaging removes the staging folder from a tool's message, in both slash forms, so no
// absolute path reaches a job error.
func scrubStaging(text, dir string) string {
	if dir == "" || dir == "." {
		return text
	}
	text = strings.ReplaceAll(text, dir, "<staging>")
	return strings.ReplaceAll(text, filepath.ToSlash(dir), "<staging>")
}

// WriteFrame implements FrameSink. A write that does not complete within 30 seconds kills FFmpeg.
func (f *FFmpeg) WriteFrame(data []byte) error {
	select {
	case <-f.done:
		return f.exitError()
	default:
	}
	timer := time.AfterFunc(ffmpegWriteTimeout, f.Abort)
	defer timer.Stop()
	if _, err := f.stdin.Write(data); err != nil {
		// Give the process a moment to exit so its message explains the broken pipe.
		select {
		case <-f.done:
		case <-time.After(2 * time.Second):
		}
		if exit := f.exitError(); exit != nil {
			return exit
		}
		return fmt.Errorf("write to FFmpeg: %w", err)
	}
	f.frames++
	if f.frames == 1 && f.settle > 0 {
		select {
		case <-f.done:
			return f.exitError()
		case <-time.After(f.settle):
		}
	}
	return nil
}

func (f *FFmpeg) exitError() error {
	if msg := f.Message(); msg != "" {
		return fmt.Errorf("FFmpeg stopped: %s", msg)
	}
	if err := f.Err(); err != nil {
		return fmt.Errorf("FFmpeg stopped: %w", err)
	}
	return errors.New("FFmpeg stopped")
}

// Finish implements FrameSink: it closes the frame input so FFmpeg writes the trailer and exits.
func (f *FFmpeg) Finish() error {
	f.closeStdin()
	return nil
}

func (f *FFmpeg) closeStdin() { f.stdinOnce.Do(func() { _ = f.stdin.Close() }) }

// Wait waits for the process to end after Finish and returns its error with FFmpeg's message.
func (f *FFmpeg) Wait(ctx context.Context) error {
	timer := time.NewTimer(ffmpegWaitAfterEnd)
	defer timer.Stop()
	select {
	case <-f.done:
	case <-ctx.Done():
		f.Abort()
		return ctx.Err()
	case <-timer.C:
		f.Abort()
		return errors.New("FFmpeg did not finish after the last frame")
	}
	if f.Err() != nil {
		return f.exitError()
	}
	return nil
}

// Abort kills the process and removes its partial output. It is safe to call more than once.
func (f *FFmpeg) Abort() {
	f.closeStdin()
	if f.cmd.Process != nil {
		_ = f.cmd.Process.Kill()
	}
}

// Discard aborts the process, waits for it and deletes the partial output file.
func (f *FFmpeg) Discard() {
	f.Abort()
	<-f.done
	_ = os.Remove(f.output)
}

// tailBuffer keeps the last limit bytes written to it.
type tailBuffer struct {
	mu    sync.Mutex
	limit int
	buf   []byte
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.limit {
		t.buf = append([]byte(nil), t.buf[len(t.buf)-t.limit:]...)
	}
	return len(p), nil
}

func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}
