package render

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// meterFilter runs both meters in one graph. I comes from ebur128, because loudnorm under-reads
// short films. TP comes from loudnorm.
//
// framelog=quiet is refused by FFmpeg 5.1. framelog=verbose prints nothing at -loglevel info.
const meterFilter = "ebur128=framelog=verbose,loudnorm=I=-16:TP=-1:LRA=11:print_format=json"

// meterErrorBytes is how much of FFmpeg's closing text a failed meter keeps.
const meterErrorBytes = 400

// meterSummaryI matches the I: line of ebur128's summary. A per-frame line starts with the filter
// name, so it never matches.
var meterSummaryI = regexp.MustCompile(`(?m)^\s*I:\s+(\S+)\s+LUFS\s*$`)

// MeterReading is one measurement. A silent reading holds -Inf in both levels.
type MeterReading struct {
	I, TP  float64
	Silent bool // loudnorm reported -inf
}

// Meter is a running FFmpeg process that measures float samples written to its stdin.
type Meter struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stderr *tailBuffer
	buf    []byte
	closed bool
	result MeterReading
	err    error
}

// MeterArgs returns the FFmpeg arguments of a meter. -loglevel info is needed, because both reports
// vanish at error.
func MeterArgs() []string {
	return []string{
		"-hide_banner", "-nostdin", "-nostats", "-loglevel", "info",
		"-protocol_whitelist", "file,pipe", "-format_whitelist", "f32le",
		"-f", "f32le", "-ar", "48000", "-ac", "2", "-i", "pipe:0",
		"-af", meterFilter, "-f", "null", "-",
	}
}

// StartMeter starts the process. ctx ends it.
func StartMeter(ctx context.Context, ffmpegPath string) (*Meter, error) {
	cmd := exec.CommandContext(ctx, ffmpegPath, MeterArgs()...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("storymaker: open the meter input: %w", err)
	}
	m := &Meter{cmd: cmd, stdin: stdin, stderr: &tailBuffer{limit: ffmpegStderrKeep}}
	cmd.Stderr = m.stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("storymaker: start the FFmpeg meter: %w", err)
	}
	return m, nil
}

// Write sends interleaved stereo samples to the meter.
func (m *Meter) Write(interleaved []float32) error {
	need := len(interleaved) * 4
	if cap(m.buf) < need {
		m.buf = make([]byte, need)
	}
	b := m.buf[:need]
	for i, v := range interleaved {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(v))
	}
	if _, err := m.stdin.Write(b); err != nil {
		return fmt.Errorf("storymaker: write to the FFmpeg meter: %w", err)
	}
	return nil
}

// Close ends the input, waits for FFmpeg and parses its reports. A second call returns the first answer.
func (m *Meter) Close() (MeterReading, error) {
	if m.closed {
		return m.result, m.err
	}
	m.closed = true
	_ = m.stdin.Close()
	waitErr := m.cmd.Wait()
	text := m.stderr.String()
	if waitErr != nil {
		tail := strings.TrimSpace(text)
		if len(tail) > meterErrorBytes {
			tail = tail[len(tail)-meterErrorBytes:]
		}
		m.err = fmt.Errorf("storymaker: the FFmpeg meter failed: %w: %s", waitErr, tail)
		return m.result, m.err
	}
	m.result, m.err = parseMeterOutput(text)
	return m.result, m.err
}

// MeterStored measures a stored WAV or MP3 with the graph of Meter. FFmpeg reads the file from src on its stdin, so it sees
// only these bytes. ctx ends it.
func MeterStored(ctx context.Context, ffmpegPath string, src io.Reader) (MeterReading, error) {
	cmd := exec.CommandContext(ctx, ffmpegPath,
		"-hide_banner", "-nostdin", "-nostats", "-loglevel", "info",
		"-protocol_whitelist", "file,pipe", "-format_whitelist", "wav,mp3", "-i", "pipe:0",
		"-map", "0:a:0", "-ar", "48000", "-ac", "2", "-t", "3600",
		"-af", meterFilter, "-f", "null", "-")
	cmd.Stdin = src
	stderr := &tailBuffer{limit: ffmpegStderrKeep}
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return MeterReading{}, fmt.Errorf("storymaker: the FFmpeg meter failed: %w", err)
	}
	return parseMeterOutput(stderr.String())
}

// parseMeterOutput reads I from the last I: line of ebur128's summary and TP from the loudnorm JSON
// between the last { and the last }. The JSON values are strings.
func parseMeterOutput(stderr string) (MeterReading, error) {
	lines := meterSummaryI.FindAllStringSubmatch(stderr, -1)
	if len(lines) == 0 {
		return MeterReading{}, errors.New("storymaker: the meter printed no loudness summary")
	}
	integrated, err := strconv.ParseFloat(lines[len(lines)-1][1], 64)
	if err != nil || math.IsNaN(integrated) {
		return MeterReading{}, errors.New("storymaker: the meter printed an unreadable loudness")
	}
	open, end := strings.LastIndex(stderr, "{"), strings.LastIndex(stderr, "}")
	if open < 0 || end < open {
		return MeterReading{}, errors.New("storymaker: the meter printed no peak report")
	}
	var report struct {
		InputI  string `json:"input_i"`
		InputTP string `json:"input_tp"`
	}
	if err := json.Unmarshal([]byte(stderr[open:end+1]), &report); err != nil {
		return MeterReading{}, fmt.Errorf("storymaker: read the meter's peak report: %w", err)
	}
	inputI, err := strconv.ParseFloat(report.InputI, 64)
	if err != nil {
		return MeterReading{}, fmt.Errorf("storymaker: read the meter's input level: %w", err)
	}
	truePeak, err := strconv.ParseFloat(report.InputTP, 64)
	if err != nil || math.IsNaN(truePeak) {
		return MeterReading{}, errors.New("storymaker: the meter printed an unreadable peak")
	}
	if math.IsInf(inputI, -1) || math.IsInf(truePeak, -1) {
		return MeterReading{I: math.Inf(-1), TP: math.Inf(-1), Silent: true}, nil
	}
	return MeterReading{I: integrated, TP: truePeak}, nil
}
