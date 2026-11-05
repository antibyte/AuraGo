package storymaker

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"

	mp3 "github.com/hajimehoshi/go-mp3"
)

// ErrUnknownAudio means the bytes are neither WAV nor MP3.
var ErrUnknownAudio = errors.New("storymaker: unknown audio format")

const maxDecodedSeconds = 600

const (
	minWAVRate        = 4000
	maxWAVRate        = 192000
	maxWAVChunks      = 128
	maxWAVFormatBytes = 4096
	wavTagPCM         = 1
	wavTagFloat       = 3
	wavTagExtensible  = 0xFFFE
	mp3FrameBytes     = 4 // go-mp3 always gives stereo s16
)

var errNoAudio = errors.New("storymaker: the file holds no audio")

// DecodedAudio is decoded PCM.
type DecodedAudio struct {
	Rate, Channels int
	Samples        []float32 // interleaved, -1..1
}

// SniffAudio detects the format from the first bytes: "wav", "mp3" or "".
func SniffAudio(head []byte) string {
	if len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WAVE" {
		return "wav"
	}
	if len(head) >= 3 && string(head[:3]) == "ID3" {
		return "mp3"
	}
	// A frame sync, without the reserved MPEG version (01) and the reserved layer (00). The second excludes ADTS AAC.
	if len(head) >= 2 && head[0] == 0xFF && head[1]&0xE0 == 0xE0 && head[1]&0x18 != 0x08 && head[1]&0x06 != 0 {
		return "mp3"
	}
	return ""
}

// DecodeAudio detects the format from the bytes, never from a claimed extension. ext is "wav" or "mp3".
func DecodeAudio(data []byte) (DecodedAudio, string, error) {
	switch SniffAudio(data) {
	case "wav":
		a, err := decodeWAV(data)
		if err != nil {
			return DecodedAudio{}, "", err
		}
		return a, "wav", nil
	case "mp3":
		a, err := decodeMP3(data, maxDecodedSeconds)
		if err != nil {
			return DecodedAudio{}, "", err
		}
		return a, "mp3", nil
	}
	return DecodedAudio{}, "", ErrUnknownAudio
}

// ProbeAudio reads the format and the duration without keeping any samples: the WAV header, or go-mp3's Length.
// Music tracks use it, so the 600 s decode cap does not apply.
func ProbeAudio(r io.ReadSeeker) (ext string, durationMS int, err error) {
	size, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return "", 0, err
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return "", 0, err
	}
	head := make([]byte, 12)
	n, err := io.ReadFull(r, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", 0, err
	}
	head = head[:n]
	switch SniffAudio(head) {
	case "wav":
		ms, err := probeWAV(r, size)
		if err != nil {
			return "", 0, err
		}
		return "wav", ms, nil
	case "mp3":
		ms, err := probeMP3(r, head, size)
		if err != nil {
			return "", 0, err
		}
		return "mp3", ms, nil
	}
	return "", 0, ErrUnknownAudio
}

// roundedMS is round(frames*1000/rate).
func roundedMS(frames int64, rate int) int {
	return int((frames*2000 + int64(rate)) / (2 * int64(rate)))
}

type wavHeader struct {
	rate, channels, bits, frameBytes int
	float                            bool
	dataOffset, dataLen              int64
}

// readWAVHeader walks the chunks after the 12-byte RIFF header. A data length of 0, 0xFFFFFFFF or above the file means
// the rest of the file.
func readWAVHeader(r io.ReadSeeker, size int64) (wavHeader, error) {
	var h wavHeader
	pos := int64(12)
	haveFormat := false
	for chunk := 0; chunk < maxWAVChunks; chunk++ {
		if _, err := r.Seek(pos, io.SeekStart); err != nil {
			return h, err
		}
		var head [8]byte
		if _, err := io.ReadFull(r, head[:]); err != nil {
			return h, errors.New("storymaker: the WAV has no data chunk")
		}
		length := int64(binary.LittleEndian.Uint32(head[4:]))
		body := pos + 8
		switch string(head[:4]) {
		case "fmt ":
			if err := readWAVFormat(r, length, &h); err != nil {
				return h, err
			}
			haveFormat = true
		case "data":
			if !haveFormat {
				return h, errors.New("storymaker: the WAV has its data ahead of its format")
			}
			rest := size - body
			if length == 0 || length == 0xFFFFFFFF || length > rest {
				length = rest
			}
			h.dataOffset = body
			h.dataLen = length - length%int64(h.frameBytes)
			return h, nil
		}
		pos = body + length + length%2
	}
	return h, errors.New("storymaker: the WAV has too many chunks")
}

func readWAVFormat(r io.Reader, length int64, h *wavHeader) error {
	if length < 16 || length > maxWAVFormatBytes {
		return fmt.Errorf("storymaker: the WAV format chunk has %d bytes", length)
	}
	b := make([]byte, length)
	if _, err := io.ReadFull(r, b); err != nil {
		return fmt.Errorf("storymaker: the WAV format chunk is cut off: %w", err)
	}
	tag := binary.LittleEndian.Uint16(b)
	h.channels = int(binary.LittleEndian.Uint16(b[2:]))
	h.rate = int(binary.LittleEndian.Uint32(b[4:]))
	align := int(binary.LittleEndian.Uint16(b[12:]))
	h.bits = int(binary.LittleEndian.Uint16(b[14:]))
	h.float = false
	if tag == wavTagExtensible {
		if length < 26 {
			return errors.New("storymaker: the WAV extensible format chunk is too short")
		}
		tag = binary.LittleEndian.Uint16(b[24:])
	}
	switch {
	case tag == wavTagPCM && (h.bits == 16 || h.bits == 24):
	case tag == wavTagFloat && h.bits == 32:
		h.float = true
	default:
		return fmt.Errorf("storymaker: WAV format %#x with %d bits is not supported", tag, h.bits)
	}
	if h.rate < minWAVRate || h.rate > maxWAVRate {
		return fmt.Errorf("storymaker: the WAV rate %d Hz is outside %d..%d", h.rate, minWAVRate, maxWAVRate)
	}
	if h.channels < 1 || h.channels > 2 {
		return fmt.Errorf("storymaker: the WAV has %d channels", h.channels)
	}
	h.frameBytes = h.channels * h.bits / 8
	if align != h.frameBytes {
		return fmt.Errorf("storymaker: the WAV block align is %d, want %d", align, h.frameBytes)
	}
	return nil
}

func decodeWAV(data []byte) (DecodedAudio, error) {
	h, err := readWAVHeader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return DecodedAudio{}, err
	}
	frames := h.dataLen / int64(h.frameBytes)
	if frames > int64(maxDecodedSeconds)*int64(h.rate) {
		return DecodedAudio{}, fmt.Errorf("storymaker: the WAV lasts more than %d s", maxDecodedSeconds)
	}
	if roundedMS(frames, h.rate) == 0 {
		return DecodedAudio{}, errNoAudio
	}
	raw := data[h.dataOffset : h.dataOffset+h.dataLen]
	sampleBytes := h.bits / 8
	samples := make([]float32, len(raw)/sampleBytes)
	for i := range samples {
		b := raw[i*sampleBytes:]
		switch {
		case h.float:
			samples[i] = cleanSample(math.Float32frombits(binary.LittleEndian.Uint32(b)))
		case h.bits == 24:
			v := int32(uint32(b[0])<<8|uint32(b[1])<<16|uint32(b[2])<<24) >> 8
			samples[i] = float32(v) / 8388608
		default:
			samples[i] = float32(int16(binary.LittleEndian.Uint16(b))) / 32768
		}
	}
	return DecodedAudio{Rate: h.rate, Channels: h.channels, Samples: samples}, nil
}

// cleanSample keeps a float sample inside -1..1 and turns NaN into silence.
func cleanSample(v float32) float32 {
	switch {
	case math.IsNaN(float64(v)):
		return 0
	case v > 1:
		return 1
	case v < -1:
		return -1
	}
	return v
}

func probeWAV(r io.ReadSeeker, size int64) (int, error) {
	h, err := readWAVHeader(r, size)
	if err != nil {
		return 0, err
	}
	ms := roundedMS(h.dataLen/int64(h.frameBytes), h.rate)
	if ms == 0 {
		return 0, errNoAudio
	}
	return ms, nil
}

// recoverMP3 turns a go-mp3 panic into an error. Call it directly with defer.
func recoverMP3(err *error) {
	if r := recover(); r != nil {
		*err = fmt.Errorf("storymaker: the MP3 decoder failed: %v", r)
	}
}

// checkID3Size refuses an ID3v2 tag that claims more bytes than the file holds, because go-mp3 allocates the claimed
// size. The size formula is go-mp3's own (source.go skipTags).
func checkID3Size(head []byte, total int64) error {
	if len(head) < 3 || string(head[:3]) != "ID3" {
		return nil
	}
	if len(head) < 10 {
		return errors.New("storymaker: the MP3 tag header is cut off")
	}
	size := int64(head[6])<<21 | int64(head[7])<<14 | int64(head[8])<<7 | int64(head[9])
	if size > total-10 {
		return errors.New("storymaker: the MP3 tag is larger than the file")
	}
	return nil
}

// decodeMP3 reads in chunks and stops with an error above maxSeconds, so the cap holds before the allocation.
func decodeMP3(data []byte, maxSeconds int) (a DecodedAudio, err error) {
	defer recoverMP3(&err)
	if err := checkID3Size(data, int64(len(data))); err != nil {
		return DecodedAudio{}, err
	}
	d, err := mp3.NewDecoder(bytes.NewReader(data))
	if err != nil {
		return DecodedAudio{}, fmt.Errorf("storymaker: decode MP3: %w", err)
	}
	rate := d.SampleRate()
	maxFrames := int64(maxSeconds) * int64(rate)
	tooLong := fmt.Errorf("storymaker: the MP3 lasts more than %d s", maxSeconds)
	if d.Length()/mp3FrameBytes > maxFrames {
		return DecodedAudio{}, tooLong
	}
	samples := make([]float32, 0, d.Length()/2)
	// go-mp3 keeps whole samples in its buffer and this size is a multiple of 4, so n stays aligned.
	buf := make([]byte, 64*1024)
	for {
		n, readErr := d.Read(buf)
		if int64(len(samples)/2)+int64(n/mp3FrameBytes) > maxFrames {
			return DecodedAudio{}, tooLong
		}
		for i := 0; i+1 < n; i += 2 {
			samples = append(samples, float32(int16(binary.LittleEndian.Uint16(buf[i:])))/32768)
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return DecodedAudio{}, fmt.Errorf("storymaker: decode MP3: %w", readErr)
		}
	}
	if roundedMS(int64(len(samples)/2), rate) == 0 {
		return DecodedAudio{}, errNoAudio
	}
	return DecodedAudio{Rate: rate, Channels: 2, Samples: samples}, nil
}

func probeMP3(r io.ReadSeeker, head []byte, size int64) (ms int, err error) {
	defer recoverMP3(&err)
	if err := checkID3Size(head, size); err != nil {
		return 0, err
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}
	d, err := mp3.NewDecoder(r)
	if err != nil {
		return 0, fmt.Errorf("storymaker: decode MP3: %w", err)
	}
	if d.Length() <= 0 {
		return 0, errNoAudio
	}
	return roundedMS(d.Length()/mp3FrameBytes, d.SampleRate()), nil
}
