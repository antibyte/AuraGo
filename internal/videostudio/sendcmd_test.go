package videostudio

import (
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestSendCmdScript(t *testing.T) {
	tracks := []SendCmdTrack{
		{Target: "colorchannelmixer@vs_opacity_0_1", Option: "aa", Values: []float64{0.5, 0.5, 0.75}},
		{Target: "crop@vs_crop_0_1", Option: "w", Values: []float64{640, 642, 642}},
	}
	got, err := sendCmdScript(Rate{Num: 25, Den: 1}, tracks)
	if err != nil {
		t.Fatal(err)
	}
	// Frame 0 at 0, frame k half a frame early, one interval per frame, unchanged values skipped.
	want := "0.000000 colorchannelmixer@vs_opacity_0_1 aa 0.5, crop@vs_crop_0_1 w 640;\n" +
		"0.020000 crop@vs_crop_0_1 w 642;\n" +
		"0.060000 colorchannelmixer@vs_opacity_0_1 aa 0.75;\n"
	if got != want {
		t.Fatalf("script:\n%s\nwant:\n%s", got, want)
	}
	// NTSC: each command time lies after the previous frame's timestamp and at or before its own frame's, in the
	// microseconds sendcmd compares (av_rescale_q of the frame pts to AV_TIME_BASE_Q, rounded to nearest).
	rate := Rate{Num: 30000, Den: 1001}
	values := make([]float64, 3000)
	for k := range values {
		values[k] = float64(k%2) / 4
	}
	script, err := sendCmdScript(rate, []SendCmdTrack{{Target: "colorchannelmixer@vs_opacity_2_0", Option: "aa", Values: values}})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(script, "\n"), "\n")
	if len(lines) != len(values) {
		t.Fatalf("%d lines, want one per frame (%d)", len(lines), len(values))
	}
	micros := func(frame int) int64 { return (int64(frame)*rate.Den*1_000_000 + rate.Num/2) / rate.Num }
	for k, line := range lines {
		seconds, _, _ := strings.Cut(line, " ")
		value, err := strconv.ParseFloat(seconds, 64)
		if err != nil {
			t.Fatalf("line %d: %q", k, line)
		}
		start := int64(math.Round(value * 1e6))
		if (k == 0 && start != 0) || (k > 0 && (start <= micros(k-1) || start > micros(k))) {
			t.Fatalf("frame %d command at %d µs, frame timestamps %d and %d µs", k, start, micros(k-1), micros(k))
		}
	}
	if !strings.HasPrefix(lines[1], "0.016683 ") || !strings.HasPrefix(lines[2], "0.050050 ") {
		t.Fatalf("NTSC times %q, %q", lines[1], lines[2])
	}
}

func TestWriteSendCmdRejects(t *testing.T) {
	dir := t.TempDir()
	rate := Rate{Num: 25, Den: 1}
	good := SendCmdTrack{Target: "colorchannelmixer@vs_opacity_0_1", Option: "aa", Values: []float64{1, 0.5}}
	path := filepath.Join(dir, "vs_cmd_0_1.txt")
	if err := WriteSendCmd(path, rate, []SendCmdTrack{good}); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0o600) {
		t.Fatalf("command file %v, %v; want mode 0600", info, err)
	}
	for name, tracks := range map[string][]SendCmdTrack{
		"no tracks":              nil,
		"no values":              {{Target: good.Target, Option: "aa"}},
		"two-input overlay":      {{Target: "overlay@vs_place_0_1", Option: "x", Values: []float64{1, 2}}},
		"option of another role": {{Target: good.Target, Option: "w", Values: []float64{1, 2}}},
		"unknown role":           {{Target: "eq@vs_color_0_1", Option: "contrast", Values: []float64{1, 2}}},
		"injected target":        {{Target: "colorchannelmixer@vs_opacity_0_1;x", Option: "aa", Values: []float64{1, 2}}},
		"injected option":        {{Target: good.Target, Option: "aa 1;", Values: []float64{1, 2}}},
		"NaN":                    {{Target: good.Target, Option: "aa", Values: []float64{1, math.NaN()}}},
		"infinite":               {{Target: good.Target, Option: "aa", Values: []float64{math.Inf(1), 1}}},
		"length mismatch":        {good, {Target: "crop@vs_crop_0_1", Option: "w", Values: []float64{640}}},
		"duplicate track":        {good, good},
	} {
		bad := filepath.Join(dir, "bad.txt")
		if err := WriteSendCmd(bad, rate, tracks); err == nil {
			t.Errorf("%s: accepted", name)
		}
		if _, err := os.Stat(bad); !os.IsNotExist(err) {
			t.Errorf("%s: a refused script was written", name)
		}
	}
	if err := WriteSendCmd(filepath.Join(dir, "rate.txt"), Rate{}, []SendCmdTrack{good}); err == nil {
		t.Error("a zero rate was accepted")
	}
}
