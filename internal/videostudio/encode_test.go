package videostudio

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestEncoderCandidatesAndArgs(t *testing.T) {
	present := func(string) bool { return true }
	absent := func(string) bool { return false }
	codecs := func(list []Encoder) []string {
		names := []string{}
		for _, encoder := range list {
			names = append(names, encoder.Codec)
		}
		return names
	}
	for _, c := range []struct {
		goos, mode string
		exists     func(string) bool
		want       []string
	}{
		{"linux", "auto", present, []string{CodecVAAPI, CodecQSV, CodecNVENC}},
		{"linux", "", absent, []string{CodecNVENC}},
		{"windows", "auto", absent, []string{CodecNVENC, CodecQSV, CodecAMF}},
		{"darwin", "auto", present, []string{}},
		{"linux", "off", present, []string{}},
		{"windows", "amf", absent, []string{CodecAMF}},
	} {
		if got := codecs(encoderCandidates(c.goos, c.mode, "", c.exists)); !reflect.DeepEqual(got, c.want) {
			t.Fatalf("%s/%s candidates = %v, want %v", c.goos, c.mode, got, c.want)
		}
	}
	if got := encoderCandidates("linux", "vaapi", "/dev/dri/renderD129", absent); len(got) != 1 || got[0].Device != "/dev/dri/renderD129" {
		t.Fatalf("explicit VAAPI candidate = %+v", got)
	}

	vaapi := Encoder{Codec: CodecVAAPI}
	if !vaapi.Hardware() || !reflect.DeepEqual(vaapi.globalArgs(), []string{"-vaapi_device", "/dev/dri/renderD128"}) || vaapi.filterTail() != ",format=nv12,hwupload" {
		t.Fatalf("VAAPI setup = %v %q", vaapi.globalArgs(), vaapi.filterTail())
	}
	if args := strings.Join(vaapi.videoArgs(Rate{30000, 1001}), " "); !strings.Contains(args, "-c:v h264_vaapi -rc_mode CQP -qp 24") || !strings.Contains(args, "-g 60") {
		t.Fatalf("VAAPI args = %s", args)
	}
	if args := strings.Join(Encoder{Codec: CodecNVENC, Quality: 30}.videoArgs(Rate{60, 1}), " "); !strings.Contains(args, "-cq 30") || !strings.Contains(args, "-g 120") {
		t.Fatalf("NVENC args = %s", args)
	}
	software := Encoder{}
	if args := strings.Join(software.videoArgs(Rate{25, 1}), " "); software.Hardware() || !strings.Contains(args, "-c:v libx264 -preset veryfast -crf 23") || !strings.Contains(args, "-g 50") || software.filterTail() != "" {
		t.Fatalf("libx264 args = %s", args)
	}
}

func TestSelectEncoderFallsThroughCandidates(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	original := testEncode
	t.Cleanup(func() { testEncode = original })
	testEncode = func(_ context.Context, _ string, encoder Encoder) error {
		if encoder.Codec == CodecNVENC {
			return nil
		}
		return errors.New("no device\nsecond line")
	}
	if status := SelectEncoder(context.Background(), ffmpeg, "nvenc", ""); status.Name != CodecNVENC || !status.Hardware || status.Encoder.Codec != CodecNVENC {
		t.Fatalf("NVENC status = %+v", status)
	}
	testEncode = func(context.Context, string, Encoder) error { return errors.New("Unknown encoder 'h264_nvenc'") }
	status := SelectEncoder(context.Background(), ffmpeg, "nvenc", "")
	if status.Name != CodecX264 || status.Hardware || !strings.Contains(status.Note, "h264_nvenc: Unknown encoder") {
		t.Fatalf("fallback status = %+v", status)
	}
	if status := SelectEncoder(context.Background(), ffmpeg, "off", ""); status.Name != CodecX264 || status.Note != "" {
		t.Fatalf("off status = %+v", status)
	}
}
