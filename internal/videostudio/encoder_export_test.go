package videostudio

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestEncoderExportsMatchTheInternalViews(t *testing.T) {
	rate := Rate{Num: 30, Den: 1}
	for _, e := range []Encoder{{}, {Codec: CodecVAAPI, Device: "/dev/dri/renderD129"}, {Codec: CodecQSV}, {Codec: CodecNVENC, Quality: 20}, {Codec: CodecAMF}} {
		if !reflect.DeepEqual(e.GlobalArgs(), e.globalArgs()) {
			t.Errorf("%+v: GlobalArgs differs", e)
		}
		if e.FilterTail() != e.filterTail() {
			t.Errorf("%+v: FilterTail differs", e)
		}
		if !reflect.DeepEqual(e.VideoArgs(rate), e.videoArgs(rate)) {
			t.Errorf("%+v: VideoArgs differs", e)
		}
	}
	// The pieces a frame-input graph relies on.
	if got := (Encoder{Codec: CodecVAAPI}).FilterTail(); got != ",format=nv12,hwupload" {
		t.Errorf("VAAPI filter tail = %q", got)
	}
	if got := strings.Join((Encoder{}).VideoArgs(rate), " "); !strings.Contains(got, "-c:v libx264") || !strings.Contains(got, "-colorspace bt709") {
		t.Errorf("software video args = %q", got)
	}
}

func TestResolveFFmpegUsesTheSharedPolicy(t *testing.T) {
	if _, err := ResolveFFmpeg(filepath.Join(t.TempDir(), "no-ffmpeg")); err == nil {
		t.Error("a configured path that does not exist was accepted")
	}
	if _, err := ResolveFFmpeg(t.TempDir()); err == nil {
		t.Error("a directory was accepted as the FFmpeg binary")
	}
}

func TestResolveFFprobeFindsTheBinaryBesideFFmpeg(t *testing.T) {
	dir := t.TempDir()
	probe := filepath.Join(dir, "ffprobe.exe")
	if err := os.WriteFile(probe, []byte("stand-in"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveFFprobe(filepath.Join(dir, "ffmpeg.exe"))
	if err != nil || got != probe {
		t.Fatalf("ResolveFFprobe = %q, %v, want %q", got, err, probe)
	}
}
