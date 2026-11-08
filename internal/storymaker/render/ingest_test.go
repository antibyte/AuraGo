package render

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image/color"
	"path/filepath"
	"strconv"
	"testing"

	"aurago/internal/videostudio"
)

type fakeSink struct {
	frames   [][]byte
	finished bool
	failAt   int // 1-based write number that fails; 0 never
	writes   int
}

func (s *fakeSink) WriteFrame(data []byte) error {
	s.writes++
	if s.failAt > 0 && s.writes == s.failAt {
		return errors.New("pipe closed")
	}
	s.frames = append(s.frames, append([]byte(nil), data...))
	return nil
}

func (s *fakeSink) Finish() error { s.finished = true; return nil }

func startedIngest(t *testing.T, cfg IngestConfig, total int) (*Ingest, *fakeSink) {
	t.Helper()
	in := NewIngest(cfg)
	sink := &fakeSink{}
	in.Start(total, sink, nil, nil)
	return in, sink
}

func ingestCode(err error) string {
	var ie *IngestError
	if errors.As(err, &ie) {
		return ie.Code
	}
	if err == nil {
		return ""
	}
	return "other:" + err.Error()
}

func TestIngestRefusesFramesUntilTheRenderRuns(t *testing.T) {
	cfg := IngestConfig{Width: 64, Height: 36, Transport: TransportPNG, FrameLimit: 10}
	in := NewIngest(cfg)
	frame := solidPNG(t, 64, 36, color.NRGBA{A: 255})
	if _, err := in.Accept(0, "image/png", frame); ingestCode(err) != "not_running" {
		t.Fatalf("before Start: %v", err)
	}
	sink := &fakeSink{}
	in.Start(2, sink, nil, nil)
	if _, err := in.Accept(0, "image/png", frame); err != nil {
		t.Fatal(err)
	}
	in.Stop()
	if _, err := in.Accept(1, "image/png", frame); ingestCode(err) != "not_running" {
		t.Fatalf("after Stop: %v", err)
	}
	if len(sink.frames) != 1 {
		t.Fatalf("the sink got %d frames", len(sink.frames))
	}
}

func TestIngestEnforcesOrderLimitAndCompletion(t *testing.T) {
	cfg := IngestConfig{Width: 64, Height: 36, Transport: TransportPNG, FrameLimit: 3}
	frame := solidPNG(t, 64, 36, color.NRGBA{A: 255})
	in, sink := startedIngest(t, cfg, 5) // the report announced 5 frames, the Go limit allows 3
	for _, tc := range []struct {
		n    int
		code string
	}{{1, "frame_order"}, {0, ""}, {0, "frame_order"}, {2, "frame_order"}, {1, ""}, {2, ""}, {3, "frame_limit"}} {
		_, err := in.Accept(tc.n, "image/png", frame)
		if ingestCode(err) != tc.code {
			t.Fatalf("frame %d: got %v, want %q", tc.n, err, tc.code)
		}
	}
	if len(sink.frames) != 3 || in.Bytes() != int64(3*len(frame)) {
		t.Fatalf("sink frames %d, bytes %d", len(sink.frames), in.Bytes())
	}

	in2, sink2 := startedIngest(t, IngestConfig{Width: 64, Height: 36, Transport: TransportPNG, FrameLimit: 100}, 2)
	if out, err := in2.Accept(0, "image/png", frame); err != nil || out != FrameOK {
		t.Fatalf("first: %v %v", out, err)
	}
	if out, err := in2.Accept(1, "image/png", frame); err != nil || out != FrameDone || !sink2.finished {
		t.Fatalf("last: %v %v finished=%v", out, err, sink2.finished)
	}
	if _, err := in2.Accept(2, "image/png", frame); ingestCode(err) != "not_running" {
		t.Fatalf("after the last frame: %v", err)
	}
}

func TestIngestChecksContentTypeAndHeaderAgainstTheCanvas(t *testing.T) {
	good := solidPNG(t, 64, 36, color.NRGBA{A: 255})
	wrongSize := solidPNG(t, 65, 36, color.NRGBA{A: 255})
	truncated := good[:len(good)-5]
	cases := []struct {
		name        string
		cfg         IngestConfig
		contentType string
		data        []byte
		code        string
	}{
		{"ok png", IngestConfig{Width: 64, Height: 36, Transport: TransportPNG}, "image/png", good, ""},
		{"ok png with charset parameter", IngestConfig{Width: 64, Height: 36, Transport: TransportPNG}, "image/png; x=y", good, ""},
		{"png wrong dimensions", IngestConfig{Width: 64, Height: 36, Transport: TransportPNG}, "image/png", wrongSize, "frame_size"},
		{"png truncated", IngestConfig{Width: 64, Height: 36, Transport: TransportPNG}, "image/png", truncated, "frame_format"},
		{"png garbage", IngestConfig{Width: 64, Height: 36, Transport: TransportPNG}, "image/png", bytes.Repeat([]byte{1}, 100), "frame_format"},
		{"png sent as raw", IngestConfig{Width: 64, Height: 36, Transport: TransportRaw}, "image/png", good, "content_type"},
		{"webp sent as png", IngestConfig{Width: 64, Height: 36, Transport: TransportPNG}, "image/webp", good, "content_type"},
		{"no content type", IngestConfig{Width: 64, Height: 36, Transport: TransportPNG}, "", good, "content_type"},
		{"ok raw", IngestConfig{Width: 4, Height: 2, Transport: TransportRaw}, "application/octet-stream", make([]byte, 32), ""},
		{"raw too short", IngestConfig{Width: 4, Height: 2, Transport: TransportRaw}, "application/octet-stream", make([]byte, 31), "frame_size"},
		{"raw too long", IngestConfig{Width: 4, Height: 2, Transport: TransportRaw}, "application/octet-stream", make([]byte, 33), "frame_size"},
		{"ok webp lossless", IngestConfig{Width: 640, Height: 360, Transport: TransportWebP}, "image/webp", fakeWebP("VP8L", 640, 360), ""},
		{"ok webp extended", IngestConfig{Width: 640, Height: 360, Transport: TransportWebP}, "image/webp", fakeWebP("VP8X", 640, 360), ""},
		{"ok webp lossy", IngestConfig{Width: 640, Height: 360, Transport: TransportWebP}, "image/webp", fakeWebP("VP8 ", 640, 360), ""},
		{"webp wrong dimensions", IngestConfig{Width: 640, Height: 360, Transport: TransportWebP}, "image/webp", fakeWebP("VP8L", 641, 360), "frame_size"},
		{"webp truncated", IngestConfig{Width: 640, Height: 360, Transport: TransportWebP}, "image/webp", fakeWebP("VP8L", 640, 360)[:40], "frame_format"},
		{"not a webp", IngestConfig{Width: 640, Height: 360, Transport: TransportWebP}, "image/webp", good, "frame_format"},
	}
	for _, tc := range cases {
		tc.cfg.FrameLimit = 10
		in, sink := startedIngest(t, tc.cfg, 5)
		_, err := in.Accept(0, tc.contentType, tc.data)
		if ingestCode(err) != tc.code {
			t.Errorf("%s: got %v, want %q", tc.name, err, tc.code)
		}
		if tc.code != "" && len(sink.frames) != 0 {
			t.Errorf("%s: a refused frame reached the sink", tc.name)
		}
	}
}

// fakeWebP builds the header bytes of a WebP of the given chunk type and size, padded to a plausible body.
func fakeWebP(chunk string, w, h int) []byte {
	body := make([]byte, 40)
	// body[i] sits at file offset 20+i.
	switch chunk {
	case "VP8L":
		body[0] = 0x2f
		binary.LittleEndian.PutUint32(body[1:5], uint32(w-1)|uint32(h-1)<<14)
	case "VP8X":
		body[4], body[5], body[6] = byte(w-1), byte((w-1)>>8), byte((w-1)>>16)
		body[7], body[8], body[9] = byte(h-1), byte((h-1)>>8), byte((h-1)>>16)
	case "VP8 ":
		body[3], body[4], body[5] = 0x9d, 0x01, 0x2a
		binary.LittleEndian.PutUint16(body[6:8], uint16(w))
		binary.LittleEndian.PutUint16(body[8:10], uint16(h))
	}
	var out bytes.Buffer
	out.WriteString("RIFF")
	_ = binary.Write(&out, binary.LittleEndian, uint32(4+8+len(body)))
	out.WriteString("WEBP")
	out.WriteString(chunk)
	_ = binary.Write(&out, binary.LittleEndian, uint32(len(body)))
	out.Write(body)
	return out.Bytes()
}

func TestIngestSinkFailureWithoutRecoveryIsFatalAndWithRecoveryRestarts(t *testing.T) {
	cfg := IngestConfig{Width: 64, Height: 36, Transport: TransportPNG, FrameLimit: 10}
	frame := solidPNG(t, 64, 36, color.NRGBA{A: 255})

	in := NewIngest(cfg)
	in.Start(3, &fakeSink{failAt: 1}, nil, nil)
	if _, err := in.Accept(0, "image/png", frame); ingestCode(err) != "encoder_failed" {
		t.Fatalf("no recovery: %v", err)
	}

	in = NewIngest(cfg)
	replacement := &fakeSink{}
	calls := 0
	in.Start(3, &fakeSink{failAt: 2}, func(error) (FrameSink, bool) {
		calls++
		return replacement, calls == 1
	}, nil)
	if _, err := in.Accept(0, "image/png", frame); err != nil {
		t.Fatal(err)
	}
	out, err := in.Accept(1, "image/png", frame)
	if err != nil || out != FrameRestart || in.Next() != 0 {
		t.Fatalf("restart: %v %v next=%d", out, err, in.Next())
	}
	for i := 0; i < 3; i++ {
		if _, err := in.Accept(i, "image/png", frame); err != nil {
			t.Fatalf("after the restart, frame %d: %v", i, err)
		}
	}
	if len(replacement.frames) != 3 || !replacement.finished {
		t.Fatalf("replacement sink got %d frames, finished=%v", len(replacement.frames), replacement.finished)
	}
}

func TestMaxFrameBytes(t *testing.T) {
	if got := MaxFrameBytes(IngestConfig{Width: 1920, Height: 1080, Transport: TransportPNG}); got != 1920*1080*4+1920*1080*4/100+(64<<10) {
		t.Errorf("1080p png = %d", got)
	}
	if got := MaxFrameBytes(IngestConfig{Width: 1920, Height: 1080, Transport: TransportRaw}); got != 1920*1080*4 {
		t.Errorf("1080p raw = %d", got)
	}
	if got := MaxFrameBytes(IngestConfig{Width: 320, Height: 180, Transport: TransportPNG}); got != 4<<20 {
		t.Errorf("small png = %d, want the 4 MiB floor", got)
	}
	if got := MaxFrameBytes(IngestConfig{Width: 3840, Height: 2160, Transport: TransportWebP}); got != 16<<20 {
		t.Errorf("4K webp = %d", got)
	}
	for _, transport := range []string{TransportPNG, TransportScreenshot} {
		if got := MaxFrameBytes(IngestConfig{Width: 1080, Height: 1920, Transport: transport}); got < 1080*1920*4+1920 {
			t.Errorf("%s cannot hold an incompressible portrait PNG: %d", transport, got)
		}
	}
}

func TestIngestRestartsWithSoftwareWhenTheHardwareEncoderFails(t *testing.T) {
	ffmpeg, ffprobe := testBinary(t, "ffmpeg"), testBinary(t, "ffprobe")
	const w, h, fps, n = 160, 90, 30, 12
	dir := t.TempDir()
	base := FFmpegConfig{Path: ffmpeg, Transport: TransportPNG, Width: w, Height: h, FPS: fps, Frames: n, Output: filepath.Join(dir, "hw.mp4")}
	hwCfg := base
	// VAAPI is missing or has no device on this host, so FFmpeg exits during start-up.
	hwCfg.Encoder = videostudio.Encoder{Codec: videostudio.CodecVAAPI, Device: "/dev/dri/does-not-exist"}
	hw, err := StartFFmpeg(context.Background(), hwCfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(hw.Discard)
	ing := NewIngest(IngestConfig{Width: w, Height: h, Transport: TransportPNG, FrameLimit: 100})
	var soft *FFmpeg
	recoverFn := func(err error) (FrameSink, bool) {
		var e error
		soft, e = StartFFmpeg(context.Background(), base)
		if e != nil {
			t.Fatal(e)
		}
		return soft, true
	}
	ing.Start(n, hw, recoverFn, nil)
	t.Cleanup(func() {
		if soft != nil {
			soft.Discard()
		}
	})
	frame := solidPNG(t, w, h, color.NRGBA{B: 255, A: 255})
	outcome, err := ing.Accept(0, "image/png", frame)
	if err != nil || outcome != FrameRestart {
		t.Fatalf("first frame = %v %v, want a restart", outcome, err)
	}
	if ing.Next() != 0 {
		t.Fatalf("next = %d after a restart", ing.Next())
	}
	for i := 0; i < n; i++ {
		outcome, err = ing.Accept(i, "image/png", frame)
		if err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
	}
	if outcome != FrameDone {
		t.Fatalf("last outcome = %v", outcome)
	}
	if err := soft.Wait(context.Background(), postEncodeMinLimit); err != nil {
		t.Fatalf("software encode: %v", err)
	}
	doc := ffprobeFile(t, ffprobe, base.Output)
	ok := false
	for _, s := range doc.Streams {
		ok = ok || (s.CodecType == "video" && s.NbFrames == strconv.Itoa(n))
	}
	if !ok {
		t.Fatalf("streams = %+v", doc.Streams)
	}
}
