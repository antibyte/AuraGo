package videostudio

import (
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
