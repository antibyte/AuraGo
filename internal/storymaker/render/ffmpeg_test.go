package render

import (
	"bytes"
	"context"
	"image/color"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/videostudio"
)

func TestBuildFFmpegArgs(t *testing.T) {
	args, err := BuildFFmpegArgs(FFmpegConfig{Path: "ffmpeg", Transport: TransportPNG, Width: 1920, Height: 1080, FPS: 30, Frames: 90, Output: "out.mp4"})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"-protocol_whitelist file,pipe -format_whitelist image2pipe,png_pipe -f image2pipe -c:v png -framerate 30 -i pipe:0",
		"-protocol_whitelist file,pipe -format_whitelist lavfi -f lavfi -t 3.000000 -i anullsrc=r=48000:cl=stereo",
		"scale=out_color_matrix=bt709:out_range=tv,format=yuv420p",
		"-c:a aac", "-movflags +faststart", "-f mp4",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("args lack %q:\n%s", want, joined)
		}
	}
	raw, _ := BuildFFmpegArgs(FFmpegConfig{Path: "ffmpeg", Transport: TransportRaw, Width: 1280, Height: 720, FPS: 25, Frames: 25, Output: "o.mp4"})
	if !strings.Contains(strings.Join(raw, " "), "-format_whitelist rawvideo -f rawvideo -pixel_format rgba -video_size 1280x720 -framerate 25") {
		t.Errorf("raw args: %v", raw)
	}
	hw, _ := BuildFFmpegArgs(FFmpegConfig{Path: "ffmpeg", Encoder: videostudio.Encoder{Codec: videostudio.CodecVAAPI, Device: "/dev/dri/renderD128"}, Transport: TransportPNG, Width: 64, Height: 64, FPS: 30, Frames: 3, Output: "o.mp4"})
	hj := strings.Join(hw, " ")
	if !strings.Contains(hj, "-vaapi_device /dev/dri/renderD128") || !strings.Contains(hj, "format=yuv420p,format=nv12,hwupload") {
		t.Errorf("hardware args: %s", hj)
	}
	if _, err := BuildFFmpegArgs(FFmpegConfig{Transport: "bmp", Width: 1, Height: 1, FPS: 1, Frames: 1, Output: "o"}); err == nil {
		t.Error("unknown transport accepted")
	}
}

func encodeFrames(t *testing.T, cfg FFmpegConfig, frames [][]byte) (*FFmpeg, error) {
	t.Helper()
	ff, err := StartFFmpeg(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ff.Discard)
	for _, f := range frames {
		if err := ff.WriteFrame(f); err != nil {
			return ff, err
		}
	}
	_ = ff.Finish()
	return ff, ff.Wait(context.Background(), postEncodeMinLimit)
}

func TestFFmpegEncodesPNGFramesToMP4WithBT709AndSilentAAC(t *testing.T) {
	ffmpeg, ffprobe := testBinary(t, "ffmpeg"), testBinary(t, "ffprobe")
	const w, h, fps, n = 320, 180, 30, 45
	out := filepath.Join(t.TempDir(), "out.mp4")
	var frames [][]byte
	red := solidPNG(t, w, h, color.NRGBA{R: 255, A: 255})
	for i := 0; i < n; i++ {
		frames = append(frames, red)
	}
	ff, err := encodeFrames(t, FFmpegConfig{Path: ffmpeg, Transport: TransportPNG, Width: w, Height: h, FPS: fps, Frames: n, Output: out}, frames)
	if err != nil {
		t.Fatalf("encode: %v (%s)", err, ff.Message())
	}
	doc := ffprobeFile(t, ffprobe, out)
	var video, audio = -1, -1
	for i, s := range doc.Streams {
		switch s.CodecType {
		case "video":
			video = i
		case "audio":
			audio = i
		}
	}
	if video < 0 || audio < 0 {
		t.Fatalf("streams = %+v", doc.Streams)
	}
	v, a := doc.Streams[video], doc.Streams[audio]
	if v.CodecName != "h264" || v.Width != w || v.Height != h || v.PixFmt != "yuv420p" || v.AvgFrameRate != "30/1" || v.NbFrames != strconv.Itoa(n) {
		t.Errorf("video = %+v", v)
	}
	if v.ColorSpace != "bt709" || v.ColorRange != "tv" {
		t.Errorf("colour tags = %q %q", v.ColorSpace, v.ColorRange)
	}
	if a.CodecName != "aac" || a.SampleRate != "48000" {
		t.Errorf("audio = %+v", a)
	}
	if d, _ := strconv.ParseFloat(doc.Format.Duration, 64); math.Abs(d-float64(n)/fps) > 0.12 {
		t.Errorf("duration = %v, want about %v", d, float64(n)/fps)
	}
	// +faststart: the moov atom comes before the media data.
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if moov, mdat := bytes.Index(data, []byte("moov")), bytes.Index(data, []byte("mdat")); moov < 0 || mdat < 0 || moov > mdat {
		t.Errorf("moov at %d, mdat at %d: not faststart", moov, mdat)
	}
	// Pure red must decode to the BT.709 limited-range luma (about 63), not the BT.601 one (about 82).
	raw, err := exec.Command(ffmpeg, "-v", "error", "-i", out, "-frames:v", "1", "-f", "rawvideo", "-pix_fmt", "yuv420p", "-").Output()
	if err != nil || len(raw) < w*h {
		t.Fatalf("decode: %v", err)
	}
	if y := int(raw[w*h/2+w/2]); y < 60 || y > 66 {
		t.Errorf("luma of pure red = %d, want about 63 (BT.709 limited range)", y)
	}
}

func TestFFmpegAcceptsRawFramesToo(t *testing.T) {
	ffmpeg, ffprobe := testBinary(t, "ffmpeg"), testBinary(t, "ffprobe")
	const w, h, fps, n = 160, 90, 25, 10
	out := filepath.Join(t.TempDir(), "raw.mp4")
	frame := bytes.Repeat([]byte{0, 128, 255, 255}, w*h)
	frames := make([][]byte, n)
	for i := range frames {
		frames[i] = frame
	}
	ff, err := encodeFrames(t, FFmpegConfig{Path: ffmpeg, Transport: TransportRaw, Width: w, Height: h, FPS: fps, Frames: n, Output: out}, frames)
	if err != nil {
		t.Fatalf("encode: %v (%s)", err, ff.Message())
	}
	doc := ffprobeFile(t, ffprobe, out)
	found := false
	for _, s := range doc.Streams {
		if s.CodecType == "video" && s.Width == w && s.Height == h && s.NbFrames == strconv.Itoa(n) {
			found = true
		}
	}
	if !found {
		t.Fatalf("streams = %+v", doc.Streams)
	}
}

func TestFFmpegWebPFrames(t *testing.T) {
	ffmpeg, ffprobe := testBinary(t, "ffmpeg"), testBinary(t, "ffprobe")
	const w, h, fps, n = 128, 72, 30, 6
	dir := t.TempDir()
	src := filepath.Join(dir, "frame.png")
	if err := os.WriteFile(src, solidPNG(t, w, h, color.NRGBA{G: 200, A: 255}), 0o600); err != nil {
		t.Fatal(err)
	}
	webpFile := filepath.Join(dir, "frame.webp")
	if out, err := exec.Command(ffmpeg, "-v", "error", "-y", "-i", src, "-c:v", "libwebp", "-lossless", "1", webpFile).CombinedOutput(); err != nil {
		t.Skipf("this FFmpeg cannot write WebP: %v %s", err, out)
	}
	webpBytes, _ := os.ReadFile(webpFile)
	if gw, gh, err := webpSize(webpBytes); err != nil || gw != w || gh != h {
		t.Fatalf("webpSize = %d %d %v", gw, gh, err)
	}
	frames := make([][]byte, n)
	for i := range frames {
		frames[i] = webpBytes
	}
	out := filepath.Join(dir, "webp.mp4")
	ff, err := encodeFrames(t, FFmpegConfig{Path: ffmpeg, Transport: TransportWebP, Width: w, Height: h, FPS: fps, Frames: n, Output: out}, frames)
	if err != nil {
		t.Fatalf("encode: %v (%s)", err, ff.Message())
	}
	doc := ffprobeFile(t, ffprobe, out)
	ok := false
	for _, s := range doc.Streams {
		ok = ok || (s.CodecType == "video" && s.NbFrames == strconv.Itoa(n))
	}
	if !ok {
		t.Fatalf("streams = %+v", doc.Streams)
	}
}

func TestFFmpegWriteFailsWithAMessageWhenTheProcessDies(t *testing.T) {
	ffmpeg := testBinary(t, "ffmpeg")
	// A 16:9 frame on a 4x4 declared canvas still starts, so kill the process instead.
	ff, err := StartFFmpeg(context.Background(), FFmpegConfig{Path: ffmpeg, Transport: TransportPNG, Width: 64, Height: 64, FPS: 30, Frames: 5, Output: filepath.Join(t.TempDir(), "x.mp4")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ff.Discard)
	ff.Abort()
	select {
	case <-ff.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("FFmpeg did not stop")
	}
	if err := ff.WriteFrame(solidPNG(t, 64, 64, color.NRGBA{A: 255})); err == nil {
		t.Fatal("a write to a dead FFmpeg succeeded")
	}
}

// TestFFmpegSwatchAndGate is the colour round-trip of spec line 836: a known colour goes in as a PNG (image2pipe) and as raw
// RGBA (rawvideo), and the decoded file holds its BT.709 limited-range value within 2 levels. The frame gate runs on the same
// file. Run it on 8.1.1 and in one debian:12 container, which has 5.1.9.
func TestFFmpegSwatchAndGate(t *testing.T) {
	ffmpeg, ffprobe := testBinary(t, "ffmpeg"), testBinary(t, "ffprobe")
	const w, h, fps, n = 320, 180, 30, 30
	// The limited-range values from the matrix definition, with R, G and B in 0..1:
	//   luma = kr*R + (1-kr-kb)*G + kb*B
	//   Y = 16 + 219*luma, U = 128 + 224*(B-luma)/(2*(1-kb)), V = 128 + 224*(R-luma)/(2*(1-kr))
	// BT.709 has kr 0.2126 and kb 0.0722. BT.601 has kr 0.299 and kb 0.114.
	levels := func(kr, kb float64, c color.NRGBA) [3]float64 {
		r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
		luma := kr*r + (1-kr-kb)*g + kb*b
		return [3]float64{16 + 219*luma, 128 + 224*(b-luma)/(2*(1-kb)), 128 + 224*(r-luma)/(2*(1-kr))}
	}
	// By the rules above: red is Y 62.6, U 102.3, V 240.0 in BT.709 and Y 81.5 in BT.601. The second swatch is Y 117.0, U 96.0,
	// V 173.9 in BT.709 and Y 122.7 in BT.601.
	for _, c := range []color.NRGBA{{R: 255, A: 255}, {R: 200, G: 100, B: 50, A: 255}} {
		want := levels(0.2126, 0.0722, c)
		// A swatch that cannot tell the matrices apart would let a BT.601 file pass.
		if bt601 := levels(0.299, 0.114, c); math.Abs(want[0]-bt601[0]) <= 2 {
			t.Fatalf("swatch %v has BT.709 Y %.1f and BT.601 Y %.1f, closer than the tolerance", c, want[0], bt601[0])
		}
		for _, transport := range []string{TransportPNG, TransportRaw} {
			frame := solidPNG(t, w, h, c)
			if transport == TransportRaw {
				frame = bytes.Repeat([]byte{c.R, c.G, c.B, c.A}, w*h)
			}
			frames := make([][]byte, n)
			for i := range frames {
				frames[i] = frame
			}
			out := filepath.Join(t.TempDir(), "swatch.mp4")
			ff, err := encodeFrames(t, FFmpegConfig{Path: ffmpeg, Transport: transport, Width: w, Height: h, FPS: fps, Frames: n, Output: out}, frames)
			if err != nil {
				t.Fatalf("%s: encode: %v (%s)", transport, err, ff.Message())
			}
			probe, err := ProbeOutput(context.Background(), ffprobe, out, postEncodeMinLimit)
			if err != nil {
				t.Fatalf("%s: frame gate probe: %v", transport, err)
			}
			if err := CheckOutput(probe, n); err != nil {
				t.Fatalf("%s: frame gate: %v (%+v)", transport, err, probe)
			}
			raw, err := exec.Command(ffmpeg, "-v", "error", "-i", out, "-frames:v", "1", "-f", "rawvideo", "-pix_fmt", "yuv420p", "-").Output()
			if err != nil || len(raw) != w*h*3/2 {
				t.Fatalf("%s: decode: %v, %d bytes", transport, err, len(raw))
			}
			// yuv420p is a w*h luma plane, then two (w/2)*(h/2) chroma planes. Each index is the middle of its plane.
			mid := (w/2)*(h/2)/2 + w/4
			got := [3]byte{raw[w*h/2+w/2], raw[w*h+mid], raw[w*h*5/4+mid]}
			for i, name := range []string{"Y", "U", "V"} {
				if math.Abs(float64(got[i])-want[i]) > 2 {
					t.Errorf("%s swatch %v: %s = %d, want %.1f within 2 levels", transport, c, name, got[i], want[i])
				}
			}
		}
	}
}

func TestBuildFFmpegArgsAudioInputs(t *testing.T) {
	base := FFmpegConfig{Path: "ffmpeg", Transport: TransportPNG, Width: 1920, Height: 1080, FPS: 30, Frames: 90, Output: "out.mp4"}
	silent, err := BuildFFmpegArgs(base)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(silent, " ")
	// 90 frames at 30 fps is 3 s.
	for _, want := range []string{
		"-protocol_whitelist file,pipe -format_whitelist lavfi -f lavfi -t 3.000000 -i anullsrc=r=48000:cl=stereo",
		"-c:a aac -b:a 192k -ar 48000 -ac 2",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("silent args lack %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "-shortest") {
		t.Errorf("silent args keep -shortest:\n%s", joined)
	}

	withMix := base
	withMix.AudioPath = "staging/mix.wav"
	mixed, err := BuildFFmpegArgs(withMix)
	if err != nil {
		t.Fatal(err)
	}
	joined = strings.Join(mixed, " ")
	for _, want := range []string{
		"-protocol_whitelist file,pipe -format_whitelist wav -i staging/mix.wav",
		"-map 0:v:0 -map 1:a:0",
		"-c:a aac -b:a 192k -ar 48000 -ac 2",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("mix args lack %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "anullsrc") || strings.Contains(joined, "-shortest") {
		t.Errorf("mix args:\n%s", joined)
	}
}

func TestFFmpegMessageHoldsNoStagingPath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "job")
	out := filepath.Join(dir, "out.mp4")
	mix := filepath.Join(dir, "mix.wav")
	f := &FFmpeg{output: out, stderr: &tailBuffer{limit: ffmpegStderrKeep}}
	_, _ = f.stderr.Write([]byte(mix + ": Invalid data\n" + out + ": No such file\n" + filepath.ToSlash(mix) + ": again\n"))
	msg := f.Message()
	if strings.Contains(msg, dir) || strings.Contains(msg, filepath.ToSlash(dir)) {
		t.Fatalf("message holds the staging path: %s", msg)
	}
	if !strings.Contains(msg, "mix.wav") || !strings.Contains(msg, "<output>") {
		t.Fatalf("message lost its content: %s", msg)
	}
}
