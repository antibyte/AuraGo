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
