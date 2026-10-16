package videostudio

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPreviewRateCapsAndFallsBack(t *testing.T) {
	cases := map[string]Rate{
		"25/1":       {Num: 25, Den: 1},
		"24000/1001": {Num: 24000, Den: 1001},
		"30000/1001": {Num: 30000, Den: 1001},
		"50/2":       {Num: 25, Den: 1},
		"60/1":       {Num: 60, Den: 1},
		"60000/1001": {Num: 60000, Den: 1001},
		"120/1":      {Num: 60, Den: 1},
		"90000/1":    {Num: 60, Den: 1},
		"":           {Num: 30, Den: 1},
		"0/0":        {Num: 30, Den: 1},
		"25":         {Num: 30, Den: 1},
		"abc":        {Num: 30, Den: 1},
		"1/2":        {Num: 30, Den: 1},
	}
	for source, want := range cases {
		if got := PreviewRate(Asset{Kind: AssetVideo, SourceFPS: source}); got != want {
			t.Errorf("PreviewRate(%q) = %s, want %s", source, got, want)
		}
	}
	if MaxPreviewFPS != 60 {
		t.Fatalf("MaxPreviewFPS = %d, want 60", MaxPreviewFPS)
	}
}

func TestPreviewRateInProxyArgs(t *testing.T) {
	software := strings.Join(previewArgs("in.mp4", "out.mp4", Encoder{}, Rate{Num: 25, Den: 1}), " ")
	if !strings.Contains(software, "force_divisible_by=2,fps=25 -threads") || !strings.Contains(software, "-c:v libx264") || strings.Contains(software, "fps=30") {
		t.Fatalf("software proxy args = %s", software)
	}
	vaapi := strings.Join(previewArgs("in.mp4", "out.mp4", Encoder{Codec: CodecVAAPI, Device: "/dev/dri/renderD128"}, Rate{Num: 30000, Den: 1001}), " ")
	for _, want := range []string{"-vaapi_device /dev/dri/renderD128", "scale=960:540:", "fps=30000/1001,format=nv12,hwupload", "-c:v h264_vaapi", "-qp 33", "-g 60 "} {
		if !strings.Contains(vaapi, want) {
			t.Fatalf("VAAPI proxy args lack %q: %s", want, vaapi)
		}
	}
}

func TestPreviewRateProxyEncodesAtSourceRate(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	root := t.TempDir()
	for _, tc := range []struct{ rate, want string }{{"25", "25/1"}, {"100", "60/1"}} {
		source := filepath.Join(root, "source-"+tc.rate+".mp4")
		output := filepath.Join(root, "proxy-"+tc.rate+".mp4")
		makeLavfiVideoFixture(t, ctx, ffmpeg, source, "testsrc2=s=320x240:r="+tc.rate+":d=2")
		if err := PreparePreview(ctx, ffmpeg, source, output); err != nil {
			t.Fatalf("%s fps source: PreparePreview() error = %v", tc.rate, err)
		}
		proxy, err := Probe(ctx, ffmpeg, output)
		if err != nil || proxy.SourceFPS != tc.want {
			t.Fatalf("%s fps source: proxy = %+v, %v; want source_fps %s", tc.rate, proxy, err, tc.want)
		}
	}
}
