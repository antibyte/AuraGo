package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
)

const rate = 48000

type wavWriter struct {
	f      *os.File
	bw     *bufio.Writer
	frames int64
	float  bool
	buf    []byte
}

func newWav(path string, float bool) (*wavWriter, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	w := &wavWriter{f: f, bw: bufio.NewWriterSize(f, 1<<20), float: float}
	_, err = w.bw.Write(make([]byte, 44))
	return w, err
}

func (w *wavWriter) writeS16(s []int16) error {
	if cap(w.buf) < len(s)*2 {
		w.buf = make([]byte, len(s)*2)
	}
	b := w.buf[:len(s)*2]
	for i, v := range s {
		binary.LittleEndian.PutUint16(b[i*2:], uint16(v))
	}
	w.frames += int64(len(s) / 2)
	_, err := w.bw.Write(b)
	return err
}

func (w *wavWriter) writeF32(s []float32) error {
	if cap(w.buf) < len(s)*4 {
		w.buf = make([]byte, len(s)*4)
	}
	b := w.buf[:len(s)*4]
	for i, v := range s {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(v))
	}
	w.frames += int64(len(s) / 2)
	_, err := w.bw.Write(b)
	return err
}

func (w *wavWriter) Close() error {
	if err := w.bw.Flush(); err != nil {
		return err
	}
	bits, code := 16, 1
	if w.float {
		bits, code = 32, 3
	}
	dataLen := uint32(w.frames * 2 * int64(bits/8))
	h := make([]byte, 44)
	copy(h[0:], "RIFF")
	binary.LittleEndian.PutUint32(h[4:], 36+dataLen)
	copy(h[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(h[16:], 16)
	binary.LittleEndian.PutUint16(h[20:], uint16(code))
	binary.LittleEndian.PutUint16(h[22:], 2)
	binary.LittleEndian.PutUint32(h[24:], rate)
	binary.LittleEndian.PutUint32(h[28:], uint32(rate*2*bits/8))
	binary.LittleEndian.PutUint16(h[32:], uint16(2*bits/8))
	binary.LittleEndian.PutUint16(h[34:], uint16(bits))
	copy(h[36:], "data")
	binary.LittleEndian.PutUint32(h[40:], dataLen)
	if _, err := w.f.WriteAt(h, 0); err != nil {
		return err
	}
	return w.f.Close()
}

// wavInfo locates the data chunk. Formats: s16 stereo or f32 stereo.
type wavInfo struct {
	off    int64
	frames int64
	bits   int
}

func parseWAV(f *os.File) (wavInfo, error) {
	h := make([]byte, 12)
	if _, err := f.ReadAt(h, 0); err != nil {
		return wavInfo{}, err
	}
	if string(h[0:4]) != "RIFF" || string(h[8:12]) != "WAVE" {
		return wavInfo{}, errors.New("not a wav")
	}
	pos := int64(12)
	var bits, ch int
	for {
		ch8 := make([]byte, 8)
		if _, err := f.ReadAt(ch8, pos); err != nil {
			return wavInfo{}, err
		}
		id, size := string(ch8[0:4]), int64(binary.LittleEndian.Uint32(ch8[4:]))
		switch id {
		case "fmt ":
			fm := make([]byte, 16)
			f.ReadAt(fm, pos+8)
			ch = int(binary.LittleEndian.Uint16(fm[2:]))
			bits = int(binary.LittleEndian.Uint16(fm[14:]))
		case "data":
			if ch != 2 || (bits != 16 && bits != 32) {
				return wavInfo{}, errors.New("need stereo s16/f32")
			}
			if size == 0xFFFFFFFF || pos+8+size > fileSize(f) {
				size = fileSize(f) - pos - 8
			}
			return wavInfo{off: pos + 8, frames: size / int64(ch*bits/8), bits: bits}, nil
		}
		pos += 8 + size + size&1
	}
}

func fileSize(f *os.File) int64 { st, _ := f.Stat(); return st.Size() }

func readAllS16(path string) ([]int16, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := parseWAV(f)
	if err != nil {
		return nil, err
	}
	if info.bits != 16 {
		return nil, errors.New("clip must be s16")
	}
	raw := make([]byte, info.frames*4)
	if _, err := f.ReadAt(raw, info.off); err != nil && err != io.EOF {
		return nil, err
	}
	out := make([]int16, info.frames*2)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(raw[i*2:]))
	}
	return out, nil
}

// srcWindow streams frames from an s16 wav with a sequential read-ahead window.
type srcWindow struct {
	f      *os.File
	info   wavInfo
	base   int64
	n      int64
	buf    []int16
	raw    []byte
	winLen int64
}

func openSrc(path string) (*srcWindow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := parseWAV(f)
	if err != nil {
		return nil, err
	}
	if info.bits != 16 {
		return nil, errors.New("bed must be s16")
	}
	w := &srcWindow{f: f, info: info, winLen: 65536}
	w.buf = make([]int16, w.winLen*2)
	w.raw = make([]byte, w.winLen*4)
	w.base, w.n = -1, 0
	return w, nil
}

func (w *srcWindow) at(frame int64) (float32, float32) {
	if frame < w.base || frame >= w.base+w.n {
		cnt := w.winLen
		if frame+cnt > w.info.frames {
			cnt = w.info.frames - frame
		}
		if cnt <= 0 {
			return 0, 0
		}
		w.f.ReadAt(w.raw[:cnt*4], w.info.off+frame*4)
		for i := int64(0); i < cnt*2; i++ {
			w.buf[i] = int16(binary.LittleEndian.Uint16(w.raw[i*2:]))
		}
		w.base, w.n = frame, cnt
	}
	i := (frame - w.base) * 2
	return float32(w.buf[i]) / 32768, float32(w.buf[i+1]) / 32768
}
