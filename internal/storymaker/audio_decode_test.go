package storymaker

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"math/rand"
	"os"
	"strings"
	"testing"
)

// audioTestFormat describes a WAV file for audioTestWAV.
type audioTestFormat struct {
	tag, channels, bits uint16
	rate                uint32
	align               uint16  // 0 means channels * bits / 8
	sub                 uint16  // first two bytes of the sub-format GUID when tag is 0xFFFE
	dataField           *uint32 // nil means the real data length
	listChunk           bool    // an odd-sized LIST chunk ahead of the data
}

func audioTestWAV(f audioTestFormat, data []byte) []byte {
	le := binary.LittleEndian
	align := f.align
	if align == 0 {
		align = f.channels * f.bits / 8
	}
	var format bytes.Buffer
	binary.Write(&format, le, f.tag)
	binary.Write(&format, le, f.channels)
	binary.Write(&format, le, f.rate)
	binary.Write(&format, le, f.rate*uint32(align))
	binary.Write(&format, le, align)
	binary.Write(&format, le, f.bits)
	if f.tag == 0xFFFE {
		binary.Write(&format, le, uint16(22)) // extension size
		binary.Write(&format, le, f.bits)     // valid bits
		binary.Write(&format, le, uint32(0))  // channel mask
		binary.Write(&format, le, f.sub)      // start of the sub-format GUID
		format.Write(make([]byte, 14))        // rest of the GUID
	}
	var out bytes.Buffer
	out.WriteString("RIFF")
	binary.Write(&out, le, uint32(0)) // the decoder never reads the RIFF size
	out.WriteString("WAVEfmt ")
	binary.Write(&out, le, uint32(format.Len()))
	out.Write(format.Bytes())
	if f.listChunk {
		out.WriteString("LIST")
		binary.Write(&out, le, uint32(3))
		out.Write([]byte{1, 2, 3, 0}) // 3 bytes plus the pad byte
	}
	out.WriteString("data")
	field := uint32(len(data))
	if f.dataField != nil {
		field = *f.dataField
	}
	binary.Write(&out, le, field)
	out.Write(data)
	return out.Bytes()
}

func audioTestS16(values ...int16) []byte {
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, values)
	return b.Bytes()
}

func audioTestS24(values ...int32) []byte {
	var b bytes.Buffer
	for _, v := range values {
		b.Write([]byte{byte(v), byte(v >> 8), byte(v >> 16)})
	}
	return b.Bytes()
}

func audioTestF32(values ...float32) []byte {
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, values)
	return b.Bytes()
}

func audioTestOneMP3(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/narration/one.mp3")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDecodeAudioWAVFormats(t *testing.T) {
	// Rate 4000: two frames last 0.5 ms, which rounds to 1 ms, the shortest audio the decoder keeps.
	s16 := audioTestFormat{tag: 1, channels: 1, bits: 16, rate: 4000}
	withField := func(f audioTestFormat, v uint32) audioTestFormat {
		f.dataField = &v
		return f
	}
	withList := s16
	withList.listChunk = true
	half := audioTestS16(16384, 16384) // 16384 / 32768 = 0.5
	cases := []struct {
		name     string
		format   audioTestFormat
		data     []byte
		channels int
		want     []float32
	}{
		{"s16", s16, audioTestS16(0, 16384, -32768), 1, []float32{0, 0.5, -1}},
		{"s24", audioTestFormat{tag: 1, channels: 1, bits: 24, rate: 4000}, audioTestS24(0x400000, -0x800000), 1, []float32{0.5, -1}},
		{"f32 clamps and drops NaN", audioTestFormat{tag: 3, channels: 1, bits: 32, rate: 4000}, audioTestF32(0.25, -0.5, 2, float32(math.NaN())), 1, []float32{0.25, -0.5, 1, 0}},
		{"stereo", audioTestFormat{tag: 1, channels: 2, bits: 16, rate: 4000}, audioTestS16(16384, -16384, 16384, -16384), 2, []float32{0.5, -0.5, 0.5, -0.5}},
		{"extensible s16", audioTestFormat{tag: 0xFFFE, sub: 1, channels: 1, bits: 16, rate: 4000}, half, 1, []float32{0.5, 0.5}},
		{"extensible f32", audioTestFormat{tag: 0xFFFE, sub: 3, channels: 1, bits: 32, rate: 4000}, audioTestF32(0.25, 0.25), 1, []float32{0.25, 0.25}},
		{"data size 0", withField(s16, 0), half, 1, []float32{0.5, 0.5}},
		{"data size 0xFFFFFFFF", withField(s16, 0xFFFFFFFF), half, 1, []float32{0.5, 0.5}},
		{"data size above the file", withField(s16, 1000), half, 1, []float32{0.5, 0.5}},
		{"a chunk ahead of the data", withList, half, 1, []float32{0.5, 0.5}},
	}
	for _, c := range cases {
		a, ext, err := DecodeAudio(audioTestWAV(c.format, c.data))
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if ext != "wav" || a.Rate != 4000 || a.Channels != c.channels || len(a.Samples) != len(c.want) {
			t.Errorf("%s: ext %q rate %d channels %d samples %v", c.name, ext, a.Rate, a.Channels, a.Samples)
			continue
		}
		for i, want := range c.want {
			if math.Abs(float64(a.Samples[i]-want)) > 1e-6 {
				t.Errorf("%s: sample %d is %v, want %v", c.name, i, a.Samples[i], want)
			}
		}
	}
}

func TestDecodeAudioWAVRefusals(t *testing.T) {
	for name, format := range map[string]audioTestFormat{
		"rate 3000":   {tag: 1, channels: 1, bits: 16, rate: 3000},
		"3 channels":  {tag: 1, channels: 3, bits: 16, rate: 16000},
		"wrong align": {tag: 1, channels: 1, bits: 16, rate: 16000, align: 4},
	} {
		if _, _, err := DecodeAudio(audioTestWAV(format, make([]byte, 1200))); err == nil {
			t.Errorf("%s was accepted", name)
		} else if errors.Is(err, ErrUnknownAudio) {
			t.Errorf("%s: the format is known, got %v", name, err)
		}
	}
	empty := audioTestWAV(audioTestFormat{tag: 1, channels: 1, bits: 16, rate: 16000}, nil)
	if _, _, err := DecodeAudio(empty); err == nil {
		t.Error("a WAV without samples was accepted")
	}
}

func TestDecodeAudioCapsTheDuration(t *testing.T) {
	// 4000 Hz mono s16 is 8000 bytes per second.
	format := audioTestFormat{tag: 1, channels: 1, bits: 16, rate: 4000}
	if _, _, err := DecodeAudio(audioTestWAV(format, make([]byte, 8000*maxDecodedSeconds))); err != nil {
		t.Errorf("600 s: %v", err)
	}
	if _, _, err := DecodeAudio(audioTestWAV(format, make([]byte, 8000*(maxDecodedSeconds+1)))); err == nil {
		t.Error("601 s of WAV was accepted")
	}
	// one.mp3 lasts about 1 s, so a cap of 0 s trips and a cap of 2 s does not. DecodeAudio passes maxDecodedSeconds.
	one := audioTestOneMP3(t)
	if _, err := decodeMP3(one, 0); err == nil {
		t.Error("an MP3 above the cap was accepted")
	}
	if _, err := decodeMP3(one, 2); err != nil {
		t.Errorf("an MP3 under the cap: %v", err)
	}
	if maxDecodedSeconds != 600 {
		t.Errorf("maxDecodedSeconds is %d", maxDecodedSeconds)
	}
}

func TestDecodeAudioMP3(t *testing.T) {
	a, ext, err := DecodeAudio(audioTestOneMP3(t))
	if err != nil {
		t.Fatal(err)
	}
	if ext != "mp3" || a.Rate != 24000 || a.Channels != 2 || len(a.Samples)%2 != 0 {
		t.Fatalf("ext %q rate %d channels %d samples %d", ext, a.Rate, a.Channels, len(a.Samples))
	}
	// The source lasts 1000 ms. Encoder delay, the Xing frame and padding add under 200 ms.
	// A 24 kHz frame is 24 ms, so one frame below the source length is the lowest allowed value.
	ms := len(a.Samples) / 2 * 1000 / a.Rate
	if ms < 976 || ms > 1200 {
		t.Errorf("decoded %d ms", ms)
	}
	// FFmpeg's sine source has amplitude 1/8. The lossy peak stays well above 0.05.
	peak := 0.0
	for _, s := range a.Samples {
		peak = math.Max(peak, math.Abs(float64(s)))
	}
	if peak < 0.05 || peak > 1 {
		t.Errorf("peak %v", peak)
	}
}

func TestDecodeAudioGarbageMP3IsAnError(t *testing.T) {
	// A valid MPEG-1 Layer III header (128 kbit/s, 44.1 kHz, frame size 417 bytes) with only 64 bytes behind it.
	truncated := append([]byte{0xFF, 0xFB, 0x90, 0x00}, make([]byte, 64)...)
	// An ID3v2 header that claims a tag of about 256 MiB. go-mp3 would allocate it.
	hugeTag := []byte("ID3\x04\x00\x00\x7f\x7f\x7f\x7f")
	for name, data := range map[string][]byte{"truncated": truncated, "huge tag": hugeTag} {
		if _, _, err := DecodeAudio(data); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	// Valid headers around noise may decode to noise or fail, but they must return.
	rng := rand.New(rand.NewSource(1))
	var noise bytes.Buffer
	for i := 0; i < 200; i++ {
		frame := make([]byte, 417)
		rng.Read(frame)
		copy(frame, []byte{0xFF, 0xFB, 0x90, 0x00})
		noise.Write(frame)
	}
	_, _, _ = DecodeAudio(noise.Bytes())
}

func TestRecoverMP3TurnsAPanicIntoAnError(t *testing.T) {
	boom := func() (err error) {
		defer recoverMP3(&err)
		panic("bad frame")
	}
	if err := boom(); err == nil || !strings.Contains(err.Error(), "bad frame") {
		t.Fatalf("got %v", err)
	}
}

func TestSniffAudioReadsTheBytes(t *testing.T) {
	wav := audioTestWAV(audioTestFormat{tag: 1, channels: 1, bits: 16, rate: 4000}, audioTestS16(1, 2, 3))
	for name, c := range map[string]struct {
		head []byte
		want string
	}{
		"wav":      {wav[:12], "wav"},
		"id3":      {[]byte("ID3"), "mp3"},
		"frame":    {[]byte{0xFF, 0xFB}, "mp3"},
		"adts aac": {[]byte{0xFF, 0xF1}, ""}, // layer bits 00 are reserved in MPEG audio
		"ogg":      {[]byte("OggS"), ""},
		"empty":    {nil, ""},
	} {
		if got := SniffAudio(c.head); got != c.want {
			t.Errorf("%s: got %q, want %q", name, got, c.want)
		}
	}
	if _, ext, err := DecodeAudio(wav); err != nil || ext != "wav" {
		t.Errorf("wav bytes: ext %q, err %v", ext, err)
	}
	if _, ext, err := DecodeAudio([]byte("OggS and more bytes")); !errors.Is(err, ErrUnknownAudio) || ext != "" {
		t.Errorf("ogg bytes: ext %q, err %v", ext, err)
	}
}

type audioTestCountingReader struct {
	*bytes.Reader
	read int64
}

func (c *audioTestCountingReader) Read(p []byte) (int, error) {
	n, err := c.Reader.Read(p)
	c.read += int64(n)
	return n, err
}

func TestProbeAudio(t *testing.T) {
	// 48 kHz mono s16 is 96000 bytes per second, so 5760000 bytes last 60 s.
	file := audioTestWAV(audioTestFormat{tag: 1, channels: 1, bits: 16, rate: 48000}, make([]byte, 5760000))
	counting := &audioTestCountingReader{Reader: bytes.NewReader(file)}
	ext, ms, err := ProbeAudio(counting)
	if err != nil || ext != "wav" || ms != 60000 {
		t.Errorf("wav: ext %q, %d ms, err %v", ext, ms, err)
	}
	if counting.read > 4096 {
		t.Errorf("the probe read %d bytes, so it read samples", counting.read)
	}

	// A streamed size takes its length from the file. Music runs up to 3600 s, so the 600 s decode cap does not apply.
	// 4000 Hz mono s16 is 8000 bytes per second, so 601 s is 4808000 bytes.
	zero := uint32(0)
	long := audioTestWAV(audioTestFormat{tag: 1, channels: 1, bits: 16, rate: 4000, dataField: &zero}, make([]byte, 4808000))
	if ext, ms, err = ProbeAudio(bytes.NewReader(long)); err != nil || ext != "wav" || ms != 601000 {
		t.Errorf("long wav: ext %q, %d ms, err %v", ext, ms, err)
	}

	// The same bounds as TestDecodeAudioMP3.
	if ext, ms, err = ProbeAudio(bytes.NewReader(audioTestOneMP3(t))); err != nil || ext != "mp3" || ms < 976 || ms > 1200 {
		t.Errorf("mp3: ext %q, %d ms, err %v", ext, ms, err)
	}

	if _, _, err = ProbeAudio(bytes.NewReader([]byte("OggS"))); !errors.Is(err, ErrUnknownAudio) {
		t.Errorf("ogg: %v", err)
	}
}
