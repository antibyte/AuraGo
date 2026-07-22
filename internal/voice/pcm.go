package voice

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
)

var supportedRates = map[int]bool{8000: true, 16000: true, 24000: true}

// Resampler is a stateful fixed-rate mono PCM16 resampler. It intentionally
// accepts only the rates used by telephone, ASR and realtime voice providers.
type Resampler struct {
	from       int
	to         int
	phase      float64
	last       int16
	haveSample bool
}

func NewResampler(from, to int) (*Resampler, error) {
	if !supportedRates[from] || !supportedRates[to] {
		return nil, fmt.Errorf("unsupported PCM sample-rate conversion %d to %d", from, to)
	}
	return &Resampler{from: from, to: to}, nil
}

func (r *Resampler) Process(input []int16) []int16 {
	if len(input) == 0 {
		return nil
	}
	if r.from == r.to {
		return append([]int16(nil), input...)
	}

	samples := input
	if r.haveSample {
		samples = make([]int16, len(input)+1)
		samples[0] = r.last
		copy(samples[1:], input)
	}
	r.last = input[len(input)-1]
	r.haveSample = true

	step := float64(r.from) / float64(r.to)
	capacity := int(math.Ceil(float64(len(input))*float64(r.to)/float64(r.from))) + 1
	output := make([]int16, 0, capacity)
	for r.phase < float64(len(samples)-1) {
		left := int(r.phase)
		fraction := r.phase - float64(left)
		value := float64(samples[left])*(1-fraction) + float64(samples[left+1])*fraction
		output = append(output, int16(math.Round(value)))
		r.phase += step
	}
	r.phase -= float64(len(samples) - 1)
	return output
}

func EncodeWAVPCM16(samples []int16, sampleRate int) ([]byte, error) {
	if !supportedRates[sampleRate] {
		return nil, fmt.Errorf("unsupported WAV sample rate %d", sampleRate)
	}
	dataBytes := len(samples) * 2
	buffer := bytes.NewBuffer(make([]byte, 0, 44+dataBytes))
	buffer.WriteString("RIFF")
	_ = binary.Write(buffer, binary.LittleEndian, uint32(36+dataBytes))
	buffer.WriteString("WAVEfmt ")
	_ = binary.Write(buffer, binary.LittleEndian, uint32(16))
	_ = binary.Write(buffer, binary.LittleEndian, uint16(1))
	_ = binary.Write(buffer, binary.LittleEndian, uint16(1))
	_ = binary.Write(buffer, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(buffer, binary.LittleEndian, uint32(sampleRate*2))
	_ = binary.Write(buffer, binary.LittleEndian, uint16(2))
	_ = binary.Write(buffer, binary.LittleEndian, uint16(16))
	buffer.WriteString("data")
	_ = binary.Write(buffer, binary.LittleEndian, uint32(dataBytes))
	for _, sample := range samples {
		_ = binary.Write(buffer, binary.LittleEndian, sample)
	}
	return buffer.Bytes(), nil
}

func DecodeWAVPCM16(data []byte) ([]int16, int, error) {
	if len(data) < 44 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, 0, fmt.Errorf("invalid WAV container")
	}
	var sampleRate int
	var channels, bits uint16
	var pcm []byte
	for offset := 12; offset+8 <= len(data); {
		chunkID := string(data[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		offset += 8
		if size < 0 || offset+size > len(data) {
			return nil, 0, fmt.Errorf("invalid WAV chunk length")
		}
		switch chunkID {
		case "fmt ":
			if size < 16 || binary.LittleEndian.Uint16(data[offset:offset+2]) != 1 {
				return nil, 0, fmt.Errorf("WAV must contain PCM audio")
			}
			channels = binary.LittleEndian.Uint16(data[offset+2 : offset+4])
			sampleRate = int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
			bits = binary.LittleEndian.Uint16(data[offset+14 : offset+16])
		case "data":
			pcm = data[offset : offset+size]
		}
		offset += size
		if size%2 != 0 {
			offset++
		}
	}
	if channels != 1 || bits != 16 || !supportedRates[sampleRate] || len(pcm)%2 != 0 {
		return nil, 0, fmt.Errorf("WAV must be mono PCM16 at 8, 16 or 24 kHz")
	}
	samples := make([]int16, len(pcm)/2)
	for i := range samples {
		samples[i] = int16(binary.LittleEndian.Uint16(pcm[i*2 : i*2+2]))
	}
	return samples, sampleRate, nil
}
