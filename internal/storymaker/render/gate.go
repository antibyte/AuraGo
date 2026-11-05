package render

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aurago/internal/mediabins"
)

const (
	postEncodeMinLimit       = 120 * time.Second
	postEncodeBytesPerSecond = 10 << 20
	gateProbeOutputCap       = 8 << 20
	gateAudioDelta           = 0.025 // seconds
)

// postEncodeLimit is the time FFmpeg's last step and the frame gate each get for a file of this size: 120 s,
// or the size at 10 MiB/s when that is longer. The +faststart rewrite and -count_packets go through the whole
// file, so a fixed limit would throw away a long film on a slow disk after hours of rendering. A file over
// maxOutputBytes is refused anyway, so it gets no more time, and the single worker stays bounded.
func postEncodeLimit(size int64) time.Duration {
	if size > maxOutputBytes {
		size = maxOutputBytes
	}
	limit := time.Duration(size) * time.Second / postEncodeBytesPerSecond
	if limit < postEncodeMinLimit {
		return postEncodeMinLimit
	}
	return limit
}

// runGateProbe runs ffprobe; a variable so a test can read the bounds and force an answer.
var runGateProbe = mediabins.Run

// ProbeResult is what the frame gate reads from a finished MP4.
type ProbeResult struct {
	VideoPackets               int
	VideoSeconds, AudioSeconds float64
}

// ProbeOutput reads the packet count and the stream durations of a finished MP4 within timeout, which covers
// all three runs. finish runs after wait with no overall timer, so the timeout and the output cap keep ffprobe
// from holding the single worker.
func ProbeOutput(ctx context.Context, ffprobe, file string, timeout time.Duration) (ProbeResult, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	run := func(args ...string) (string, error) {
		full := append([]string{"-v", "error", "-protocol_whitelist", "file", "-format_whitelist", "mov"}, args...)
		full = append(full, "-of", "csv=p=0", "-i", file)
		out, err := runGateProbe(ctx, ffprobe, full, gateProbeOutputCap)
		if err != nil {
			msg := strings.ReplaceAll(err.Error(), ffprobe, "ffprobe")
			return "", fmt.Errorf("ffprobe: %s", scrubStaging(msg, filepath.Dir(file)))
		}
		return firstCSVField(out), nil
	}
	var result ProbeResult
	packets, err := run("-count_packets", "-select_streams", "v:0", "-show_entries", "stream=nb_read_packets")
	if err != nil {
		return ProbeResult{}, err
	}
	if result.VideoPackets, err = strconv.Atoi(packets); err != nil {
		return ProbeResult{}, fmt.Errorf("ffprobe reported the packet count %q", clip(packets, 40))
	}
	if result.VideoSeconds, err = probeStreamSeconds(run, "v:0"); err != nil {
		return ProbeResult{}, err
	}
	if result.AudioSeconds, err = probeStreamSeconds(run, "a:0"); err != nil {
		return ProbeResult{}, err
	}
	return result, nil
}

func probeStreamSeconds(run func(args ...string) (string, error), stream string) (float64, error) {
	text, err := run("-select_streams", stream, "-show_entries", "stream=duration")
	if err != nil {
		return 0, err
	}
	seconds, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0 {
		return 0, fmt.Errorf("ffprobe reported the duration %q for stream %s", clip(text, 40), stream)
	}
	return seconds, nil
}

// firstCSVField returns the first field of the first line of ffprobe's csv output.
func firstCSVField(out []byte) string {
	line := strings.TrimSpace(string(out))
	if i := strings.IndexAny(line, "\r\n"); i >= 0 {
		line = line[:i]
	}
	if i := strings.IndexByte(line, ','); i >= 0 {
		line = line[:i]
	}
	return strings.TrimSpace(line)
}

// CheckOutput is the frame gate: the video packets equal the frames, and the audio lies within 25 ms
// of the video, because the packet count does not notice a wrong audio length.
func CheckOutput(r ProbeResult, frames int) error {
	if r.VideoPackets != frames {
		return fmt.Errorf("the file has %d video packets, the compile report has %d frames", r.VideoPackets, frames)
	}
	if delta := math.Abs(r.AudioSeconds - r.VideoSeconds); delta >= gateAudioDelta {
		return fmt.Errorf("the audio and the video differ by %.0f ms", delta*1000)
	}
	return nil
}
