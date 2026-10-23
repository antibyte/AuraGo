package videostudio

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"aurago/internal/mediabins"
)

const commandOutputLimit = 4 << 20

type RuntimeStatus struct {
	Ready       bool   `json:"ready"`
	FFmpegPath  string `json:"ffmpeg_path,omitempty"`
	FFprobePath string `json:"ffprobe_path,omitempty"`
	Version     string `json:"version,omitempty"`
	Error       string `json:"error,omitempty"`
	// Missing lists the optional phase 2 capabilities a ready binary lacks (ProbeCapabilities entries). When the probe
	// failed it holds "xfade:fadegrays" alone (see ProbeError).
	Missing []string `json:"missing,omitempty"`
	// ProbeError is the error of a capability probe that did not finish (a ready binary only); empty when it did.
	ProbeError string `json:"probe_error,omitempty"`
}

// CheckRuntime resolves FFmpeg and its adjacent FFprobe binary and checks that
// both can start. For a ready binary it also lists the optional phase 2
// capabilities it lacks (Missing, memoized per binary). It does not change host
// configuration.
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
	// Phase 2 filters are optional: a missing one makes the features that need it unavailable, not the runtime. A
	// probe that fails (a context that ends inside the fadegrays self-test, a listing that does not run) is kept in
	// ProbeError and reports only what FFmpeg would get wrong without an error as missing: an unverifiable fadegrays
	// stays greyed. Renders report FFmpeg's own error for everything else.
	missing, err := ProbeCapabilities(ctx, ffmpeg)
	if err != nil {
		status.ProbeError = err.Error()
		missing = unverifiedMissing()
	}
	if len(missing) > 0 {
		status.Missing = missing
	}
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

// resolveBinary resolves an external converter binary. The shared policy lives
// in internal/mediabins: a configured path must be an existing regular file,
// and a configured bare name is looked up on PATH.
func resolveBinary(configuredPath, name string) (string, error) {
	candidates := []string{name}
	if runtime.GOOS == "windows" {
		candidates = []string{name + ".exe", name}
	}
	resolved, err := mediabins.Resolve(configuredPath, candidates)
	if err != nil {
		return "", fmt.Errorf("%s %w", name, err)
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

// runSmallCommand runs a converter with a bounded output buffer. The shared
// policy lives in internal/mediabins: stdout and stderr stay apart, the output
// is capped, and a cancelled or expired context stays matchable with errors.Is.
func runSmallCommand(ctx context.Context, path string, args []string, limit int) ([]byte, error) {
	return mediabins.Run(ctx, path, args, limit)
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
