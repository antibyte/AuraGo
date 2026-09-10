package cyd

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
	"unicode"

	"aurago/internal/sanotts"
)

const (
	SpeakRate       = 8000
	SpeakMaxSeconds = 4
	speakMaxBytes   = SpeakRate * SpeakMaxSeconds
)

// Speaker renders English notification text with sanoTTS (heart-nano) and
// stores 8 kHz unsigned PCM for the glass to fetch.
type Speaker struct {
	mu    sync.Mutex
	clips map[string][]byte
	order []string
	bin   string
	voice string
}

// NewSpeaker looks up the sanotts CLI. If it is missing, Available is false
// and the glass keeps using beep cues only.
func NewSpeaker() *Speaker {
	s := &Speaker{
		clips: make(map[string][]byte),
		voice: "heart-nano",
	}
	if p, err := exec.LookPath("sanotts"); err == nil {
		s.bin = p
	}
	return s
}

func (s *Speaker) Available() bool {
	return s != nil && s.bin != ""
}

func (s *Speaker) Queue(id, title, body string) {
	if !s.Available() || id == "" || inGoTest() {
		return
	}
	text := SpeakLine(title, body)
	go s.render(id, text)
}

func (s *Speaker) Get(id string) []byte {
	if s == nil || id == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.clips[id]
}

func (s *Speaker) render(id, text string) {
	pcm, err := synthesizeU8(s.bin, s.voice, text)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.clips[id]; !ok {
		s.order = append(s.order, id)
	}
	s.clips[id] = pcm
	for len(s.order) > 8 {
		old := s.order[0]
		s.order = s.order[1:]
		delete(s.clips, old)
	}
}

// SpeakLine is the English sentence the model hears.
func SpeakLine(title, body string) string {
	t := asciiEnglish(strings.TrimSpace(title))
	b := asciiEnglish(strings.TrimSpace(body))
	if t == "" {
		t = "Notification"
	}
	if b == "" {
		return t
	}
	return t + ". " + b
}

func asciiEnglish(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r > 127 {
			if unicode.IsSpace(r) {
				b.WriteByte(' ')
			}
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune(" .,!?'-", r) {
			b.WriteRune(r)
		} else if unicode.IsSpace(r) {
			b.WriteByte(' ')
		}
	}
	out := strings.Join(strings.Fields(b.String()), " ")
	if len(out) > 140 {
		out = out[:140]
	}
	return out
}

func synthesizeU8(bin, voice, text string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	wav, err := sanotts.Synthesize(ctx, bin, voice, text, "")
	if err != nil {
		return nil, err
	}
	return PCM8kFromWAV(wav)
}

// PCM8kFromWAV converts a PCM WAV (typically 24 kHz s16 from sanoTTS) to
// unsigned 8-bit mono at 8 kHz for the CYD speaker.
func PCM8kFromWAV(wav []byte) ([]byte, error) {
	if len(wav) < 44 || string(wav[0:4]) != "RIFF" || string(wav[8:12]) != "WAVE" {
		return nil, fmt.Errorf("not a wav")
	}
	off := 12
	var ch, bits int
	var rate uint32
	var data []byte
	for off+8 <= len(wav) {
		id := string(wav[off : off+4])
		sz := int(binary.LittleEndian.Uint32(wav[off+4 : off+8]))
		off += 8
		if sz < 0 || off+sz > len(wav) {
			return nil, fmt.Errorf("wav chunk overflow")
		}
		chunk := wav[off : off+sz]
		switch id {
		case "fmt ":
			if sz < 16 {
				return nil, fmt.Errorf("wav fmt")
			}
			if binary.LittleEndian.Uint16(chunk[0:2]) != 1 {
				return nil, fmt.Errorf("wav not pcm")
			}
			ch = int(binary.LittleEndian.Uint16(chunk[2:4]))
			rate = binary.LittleEndian.Uint32(chunk[4:8])
			bits = int(binary.LittleEndian.Uint16(chunk[14:16]))
		case "data":
			data = chunk
		}
		off += sz
		if sz%2 == 1 {
			off++
		}
	}
	if data == nil || ch != 1 || bits != 16 || rate == 0 {
		return nil, fmt.Errorf("need mono 16-bit pcm, got ch=%d bits=%d rate=%d", ch, bits, rate)
	}
	n := len(data) / 2
	samples := make([]int16, n)
	for i := 0; i < n; i++ {
		samples[i] = int16(binary.LittleEndian.Uint16(data[i*2:]))
	}
	out := downsampleU8(samples, int(rate), SpeakRate)
	if len(out) > speakMaxBytes {
		out = out[:speakMaxBytes]
	}
	return out, nil
}

func downsampleU8(in []int16, from, to int) []uint8 {
	if from <= 0 || to <= 0 || len(in) == 0 {
		return nil
	}
	step := from / to
	if step < 1 {
		step = 1
	}
	n := len(in) / step
	out := make([]uint8, n)
	for i := 0; i < n; i++ {
		sum := 0
		base := i * step
		for j := 0; j < step; j++ {
			sum += int(in[base+j])
		}
		v := (sum / step) / 256
		u := v + 128
		if u < 0 {
			u = 0
		}
		if u > 255 {
			u = 255
		}
		out[i] = uint8(u)
	}
	return out
}

func inGoTest() bool {
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.") {
			return true
		}
	}
	return false
}
