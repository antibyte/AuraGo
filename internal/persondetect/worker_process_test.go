package persondetect

import (
	"fmt"
	"strings"
	"testing"
)

// writeInChunks writes text to a fresh workerLogBuffer in chunks of size
// bytes (a pipe hands stderr over in arbitrary pieces).
func writeInChunks(text string, size int) *workerLogBuffer {
	log := newWorkerLogBuffer()
	for start := 0; start < len(text); start += size {
		_, _ = log.Write([]byte(text[start:min(start+size, len(text))]))
	}
	return log
}

func TestWorkerLogBufferReportsNativeCrashHeader(t *testing.T) {
	crash := syntheticNativeCrash()
	if len(crash) < workerLogHeadBytes+workerLogBytes || strings.Contains(crash[len(crash)-workerLogBytes:], "Exception") {
		t.Fatalf("sample has %d bytes; the header must lie before the last %d", len(crash), workerLogBytes)
	}
	for _, size := range []int{1, 7, 64, 4096, len(crash)} {
		log := writeInChunks(crash, size)
		if summary := log.summary(); summary != "crash report: Exception 0xc0000005 0x8 0x10 0x10" {
			t.Fatalf("chunks of %d: summary = %q", size, summary)
		}
		kept := log.String()
		skipped := len(crash) - workerLogHeadBytes - workerLogBytes
		if !strings.HasPrefix(kept, "[W:onnxruntime:aurago-persondetect] Some nodes") || !strings.Contains(kept, "Exception 0xc0000005 0x8 0x10 0x10\nPC=0x10") ||
			!strings.Contains(kept, fmt.Sprintf("\n[… %d bytes skipped …]\n", skipped)) || !strings.HasSuffix(kept, "gs      0x2b") {
			t.Fatalf("chunks of %d: kept log (%d bytes) = %q", size, len(kept), kept)
		}
	}
}

func TestWorkerLogBufferSummary(t *testing.T) {
	longHeader := "panic: " + strings.Repeat("y", 600)
	cases := []struct {
		name, log, want string
	}{
		{"empty", "", ""},
		{"plain lines", "loading model\r\nready\r\n", "last log line: ready"},
		{"unix signal in native code", "SIGSEGV: segmentation violation\nPC=0x7f3a m=0 sigcode=1 addr=0x10\nsignal arrived during cgo execution\n\ngoroutine 1 gp=0xc000002380 m=0 mp=0x5c [syscall]:\nruntime.cgocall(0x4b2c40, 0xc00005ff10)\n\t/usr/local/go/src/runtime/cgocall.go:167 +0x4b\nrax    0x10\n", "crash report: SIGSEGV: segmentation violation"},
		{"go panic", "panic: runtime error: invalid memory address or nil pointer dereference\n[signal 0xc0000005 code=0x0 addr=0x0 pc=0x5f1a2b]\n\ngoroutine 1 [running]:\nmain.main()\n\tC:/x/main.go:9 +0x1b\n", "crash report: panic: runtime error: invalid memory address or nil pointer dereference"},
		{"re-panic after a recovered panic", "panic: first [recovered]\n\tpanic: second\n\ngoroutine 1 [running]:\nmain.main()\n", "crash report: panic: second"},
		{"panic answered as a frame error", "goroutine 8 [running]:\nruntime/debug.Stack()\n\tC:/go/src/runtime/debug/stack.go:26 +0x5e\npanic: boom [recovered: request 3 answered with an error]\n", "last log line: panic: boom [recovered: request 3 answered with an error]"},
		{"crash after a recovered panic", "panic: boom [recovered: request 3 answered with an error]\nruntime: out of memory: cannot allocate 4194304-byte block (3976200192 in use)\nfatal error: out of memory\n\ngoroutine 1 [running]:\nruntime.throw()\n", "crash report: fatal error: out of memory"},
		{"unterminated header", "loading\nException 0xc0000005 0x0 0x0 0x7ffd", "crash report: Exception 0xc0000005 0x0 0x0 0x7ffd"},
		{"long header", longHeader + "\n\ngoroutine 1 [running]:\n", "crash report: " + longHeader[:maxWorkerErrorBytes] + "…"},
		{"long last line", strings.Repeat("z", 600), "last log line: " + strings.Repeat("z", maxWorkerErrorBytes) + "…"},
	}
	for _, tc := range cases {
		if got := writeInChunks(tc.log, 5).summary(); got != tc.want {
			t.Errorf("%s: summary = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestWorkerLogBufferKeepsSmallLogsWhole(t *testing.T) {
	for _, size := range []int{100, workerLogBytes, workerLogBytes + 10, workerLogBytes + workerLogHeadBytes} {
		text := strings.Repeat("abcdefghi\n", size/10+1)[:size]
		if kept := writeInChunks(text, 33).String(); kept != strings.TrimSpace(text) {
			t.Fatalf("%d bytes: kept %d bytes", size, len(kept))
		}
	}
}
