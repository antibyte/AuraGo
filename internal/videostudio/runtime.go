package videostudio

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const commandOutputLimit = 4 << 20

type RuntimeStatus struct {
	Ready       bool   `json:"ready"`
	FFmpegPath  string `json:"ffmpeg_path,omitempty"`
	FFprobePath string `json:"ffprobe_path,omitempty"`
	Version     string `json:"version,omitempty"`
	Error       string `json:"error,omitempty"`
}

// CheckRuntime resolves FFmpeg and its adjacent FFprobe binary and checks that
// both can start. It does not change host configuration.
func CheckRuntime(ctx context.Context, path string) RuntimeStatus {
	status := RuntimeStatus{}
	ffmpeg, err := resolveBinary(path, "ffmpeg")
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.FFmpegPath = ffmpeg
	ffprobe, err := resolveProbeBinary(ffmpeg)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.FFprobePath = ffprobe

	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	version, err := runSmallCommand(checkCtx, ffmpeg, []string{"-hide_banner", "-version"}, 32<<10)
	if err != nil {
		status.Error = fmt.Sprintf("FFmpeg is unavailable: %v", err)
		return status
	}
	status.Version = firstLine(string(version))
	filters, err := runSmallCommand(checkCtx, ffmpeg, []string{"-hide_banner", "-filters"}, commandOutputLimit)
	if err != nil {
		status.Error = fmt.Sprintf("FFmpeg filters are unavailable: %v", err)
		return status
	}
	if !hasRequiredRenderFilters(string(filters)) {
		status.Error = "FFmpeg is missing a required video or audio filter"
		return status
	}
	encoders, err := runSmallCommand(checkCtx, ffmpeg, []string{"-hide_banner", "-encoders"}, commandOutputLimit)
	if err != nil || !hasListedName(string(encoders), "libx264") {
		status.Error = "FFmpeg is missing the libx264 encoder"
		return status
	}
	if _, err := runSmallCommand(checkCtx, ffprobe, []string{"-hide_banner", "-version"}, 32<<10); err != nil {
		status.Error = fmt.Sprintf("FFprobe is unavailable: %v", err)
		return status
	}
	status.Ready = true
	return status
}

func hasListedName(output, name string) bool {
	for _, line := range strings.Split(output, "\n") {
		for _, field := range strings.Fields(line) {
			if field == name {
				return true
			}
		}
	}
	return false
}

func hasRequiredRenderFilters(output string) bool {
	for _, name := range [...]string{"xfade", "amix", "afade", "tpad", "premultiply", "unpremultiply"} {
		if !hasListedName(output, name) {
			return false
		}
	}
	return true
}

func resolveBinary(configuredPath, name string) (string, error) {
	configuredPath = strings.TrimSpace(configuredPath)
	if configuredPath != "" {
		if filepath.IsAbs(configuredPath) || strings.ContainsAny(configuredPath, `/\`) {
			resolved, err := filepath.Abs(configuredPath)
			if err != nil {
				return "", fmt.Errorf("resolve %s path: %w", name, err)
			}
			info, err := os.Stat(resolved)
			if err != nil || !info.Mode().IsRegular() {
				return "", fmt.Errorf("%s binary %q is unavailable", name, configuredPath)
			}
			return resolved, nil
		}
		resolved, err := exec.LookPath(configuredPath)
		if err != nil {
			return "", fmt.Errorf("%s binary %q was not found", name, configuredPath)
		}
		return resolved, nil
	}
	if runtime.GOOS == "windows" {
		if resolved, err := exec.LookPath(name + ".exe"); err == nil {
			return resolved, nil
		}
	}
	resolved, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s binary was not found on PATH", name)
	}
	return resolved, nil
}

func resolveProbeBinary(ffmpegPath string) (string, error) {
	base := filepath.Base(ffmpegPath)
	probeBase := strings.Replace(strings.ToLower(base), "ffmpeg", "ffprobe", 1)
	if probeBase != strings.ToLower(base) {
		candidate := filepath.Join(filepath.Dir(ffmpegPath), probeBase)
		if runtime.GOOS == "windows" {
			candidate = filepath.Join(filepath.Dir(ffmpegPath), probeBase)
		}
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			return candidate, nil
		}
	}
	return resolveBinary("", "ffprobe")
}

func resolveProbeFromConfiguredPath(path string) (string, error) {
	if strings.Contains(strings.ToLower(filepath.Base(path)), "ffprobe") {
		return resolveBinary(path, "ffprobe")
	}
	ffmpeg, err := resolveBinary(path, "ffmpeg")
	if err != nil {
		return "", err
	}
	return resolveProbeBinary(ffmpeg)
}

func runSmallCommand(ctx context.Context, path string, args []string, limit int) ([]byte, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	var stdout, stderr cappedBuffer
	stdout.limit = limit
	stderr.limit = limit
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return stdout.Bytes(), ctx.Err()
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		return stdout.Bytes(), fmt.Errorf("%w: %s", err, message)
	}
	return stdout.Bytes(), nil
}

func firstLine(value string) string {
	if index := strings.IndexAny(value, "\r\n"); index >= 0 {
		return strings.TrimSpace(value[:index])
	}
	return strings.TrimSpace(value)
}

type cappedBuffer struct {
	data  []byte
	limit int
}

func (b *cappedBuffer) Write(data []byte) (int, error) {
	if remaining := b.limit - len(b.data); remaining > 0 {
		if len(data) > remaining {
			b.data = append(b.data, data[:remaining]...)
		} else {
			b.data = append(b.data, data...)
		}
	}
	return len(data), nil
}

func (b *cappedBuffer) Bytes() []byte  { return b.data }
func (b *cappedBuffer) String() string { return string(b.data) }
