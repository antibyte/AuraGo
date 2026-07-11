package tools

import (
	"bytes"
	"strings"
	"sync"
	"testing"
)

func TestProcessInfoOutputRingStartsSmallAndGrowsByPowersOfTwo(t *testing.T) {
	info := &ProcessInfo{}
	if n, err := info.Write([]byte("x")); err != nil || n != 1 {
		t.Fatalf("first Write = (%d, %v), want (1, nil)", n, err)
	}
	if got := len(info.outputRing); got != 4<<10 {
		t.Fatalf("initial capacity = %d, want 4096", got)
	}

	chunk := bytes.Repeat([]byte("a"), 5000)
	if n, err := info.Write(chunk); err != nil || n != len(chunk) {
		t.Fatalf("growth Write = (%d, %v), want (%d, nil)", n, err, len(chunk))
	}
	if got := len(info.outputRing); got != 8<<10 {
		t.Fatalf("grown capacity = %d, want 8192", got)
	}
	want := "x" + string(chunk)
	if got := info.ReadOutput(); got != want {
		t.Fatalf("grown output did not preserve order: len=%d want=%d", len(got), len(want))
	}
}

func TestProcessInfoOutputRingWrapsAndKeepsNewestBytes(t *testing.T) {
	info := &ProcessInfo{}
	first := bytes.Repeat([]byte("a"), maxOutputSize)
	if _, err := info.Write(first); err != nil {
		t.Fatalf("fill Write: %v", err)
	}
	if _, err := info.Write([]byte("TAIL")); err != nil {
		t.Fatalf("wrap Write: %v", err)
	}
	got := info.ReadOutput()
	if len(got) != maxOutputSize || !strings.HasSuffix(got, "TAIL") || strings.HasPrefix(got, "aaaa") == false {
		t.Fatalf("wrapped output invalid: len=%d prefix=%q suffix=%q", len(got), got[:4], got[len(got)-4:])
	}
}

func TestProcessInfoOversizeWriteReportsOriginalLengthAndKeepsTail(t *testing.T) {
	info := &ProcessInfo{}
	data := make([]byte, maxOutputSize+257)
	for i := range data {
		data[i] = byte(i % 251)
	}
	n, err := info.Write(data)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(data) {
		t.Fatalf("Write reported %d, want original length %d", n, len(data))
	}
	if got := []byte(info.ReadOutput()); !bytes.Equal(got, data[len(data)-maxOutputSize:]) {
		t.Fatalf("oversize output did not retain exact last MiB")
	}
	if len(info.outputRing) != maxOutputSize {
		t.Fatalf("oversize capacity = %d, want %d", len(info.outputRing), maxOutputSize)
	}
}

func TestProcessInfoConcurrentGrowthAndReadsRemainBounded(t *testing.T) {
	info := &ProcessInfo{}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func(value byte) {
			defer wg.Done()
			for j := 0; j < 64; j++ {
				_, _ = info.Write(bytes.Repeat([]byte{value}, 4097))
			}
		}(byte(i))
		go func() {
			defer wg.Done()
			for j := 0; j < 64; j++ {
				_ = info.ReadOutput()
			}
		}()
	}
	wg.Wait()
	if info.OutputLen() > maxOutputSize || len(info.outputRing) > maxOutputSize {
		t.Fatalf("buffer exceeded bound: len=%d capacity=%d", info.OutputLen(), len(info.outputRing))
	}
}
