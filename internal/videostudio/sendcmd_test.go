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

// colorchannelmixer@vs_animk takes rr, gg and bb: the constant blur_in masks of the mask branches (index §4.11). Each branch
// reads its own file vs_cmd_<L>_<C>_m<n>.txt, which sendCmdFileName admits for n = 1 and 2 only.
func TestSendCmdAnimMaskTargets(t *testing.T) {
	rate := Rate{Num: 25, Den: 1}
	target := "colorchannelmixer@vs_animk_1_2_1"
	values := []float64{0, 27.0 / 255, 27.0 / 255, 1}
	got, err := sendCmdScript(rate, []SendCmdTrack{
		{Target: target, Option: "rr", Values: values},
		{Target: target, Option: "gg", Values: values},
		{Target: target, Option: "bb", Values: values},
	})
	k := strconv.FormatFloat(27.0/255, 'f', -1, 64)
	want := "0.000000 " + target + " rr 0, " + target + " gg 0, " + target + " bb 0;\n" +
		"0.020000 " + target + " rr " + k + ", " + target + " gg " + k + ", " + target + " bb " + k + ";\n" +
		"0.100000 " + target + " rr 1, " + target + " gg 1, " + target + " bb 1;\n"
	if err != nil || got != want {
		t.Fatalf("mask script = %q, %v; want %q", got, err, want)
	}
	for name, track := range map[string]SendCmdTrack{
		"alpha":          {Target: target, Option: "aa", Values: values},
		"a cross term":   {Target: target, Option: "rg", Values: values},
		"another role":   {Target: "colorchannelmixer@vs_anim_1_2_1", Option: "rr", Values: values},
		"another filter": {Target: "geq@vs_animk_1_2_1", Option: "rr", Values: values},
	} {
		if _, err := sendCmdScript(rate, []SendCmdTrack{track}); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	for name, ok := range map[string]bool{
		"vs_cmd_1_2_m1.txt": true, "vs_cmd_1_2_m2.txt": true, "vs_cmd_1_2.txt": true, "vs_cmd_1_2_3.txt": true,
		"vs_cmd_1_2_m0.txt": false, "vs_cmd_1_2_m3.txt": false, "vs_cmd_1_2_m.txt": false, "vs_cmd_1_2_m1_1.txt": false, "vs_cmd_1_2_mm1.txt": false,
	} {
		if sendCmdFileName.MatchString(name) != ok {
			t.Errorf("sendCmdFileName(%q) = %v, want %v", name, !ok, ok)
		}
	}
}
