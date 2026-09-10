package cyd

import (
	"encoding/binary"
	"testing"
)

func TestSpeakLineEnglish(t *testing.T) {
	got := SpeakLine("Backup failed", "disk /data 98%")
	if got != "Backup failed. disk data 98" {
		t.Fatalf("got %q", got)
	}
}

func TestPCM8kFromWAVDownsamples(t *testing.T) {
	const from = 24000
	const n = 240 // 10 ms
	data := make([]byte, n*2)
	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint16(data[i*2:], uint16(int16(i*20)))
	}
	wav := makeWav(1, 16, from, data)
	pcm, err := PCM8kFromWAV(wav)
	if err != nil {
		t.Fatal(err)
	}
	want := n / (from / SpeakRate)
	if len(pcm) != want {
		t.Fatalf("len=%d want %d", len(pcm), want)
	}
}

func makeWav(ch, bits, rate int, pcm []byte) []byte {
	buf := make([]byte, 44+len(pcm))
	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(36+len(pcm)))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16)
	binary.LittleEndian.PutUint16(buf[20:22], 1)
	binary.LittleEndian.PutUint16(buf[22:24], uint16(ch))
	binary.LittleEndian.PutUint32(buf[24:28], uint32(rate))
	byteRate := rate * ch * bits / 8
	binary.LittleEndian.PutUint32(buf[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(buf[32:34], uint16(ch*bits/8))
	binary.LittleEndian.PutUint16(buf[34:36], uint16(bits))
	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(len(pcm)))
	copy(buf[44:], pcm)
	return buf
}
