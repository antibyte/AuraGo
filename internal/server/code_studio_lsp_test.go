package server

import (
	"bufio"
	"fmt"
	"strings"
	"testing"
)

func TestCodeStudioLSPRejectsOversizeMessages(t *testing.T) {
	read := func(raw string) ([]byte, error) {
		return readCodeStudioLSPMessage(bufio.NewReaderSize(strings.NewReader(raw), codeStudioLSPReadBuffer))
	}
	if body, err := read("Content-Length: 2\r\n\r\n{}"); err != nil || string(body) != "{}" {
		t.Fatalf("plain message = %q, %v", body, err)
	}
	// A header block over 8 KiB is refused, even when it ends properly.
	header := "X-Padding: " + strings.Repeat("a", codeStudioLSPHeaderLimit) + "\r\nContent-Length: 2\r\n\r\n{}"
	if _, err := read(header); err == nil || !strings.Contains(err.Error(), "header exceeds") {
		t.Fatalf("oversize header: err = %v", err)
	}
	// A body over 32 MiB is refused from its Content-Length, before any body byte is read.
	body := fmt.Sprintf("Content-Length: %d\r\n\r\n{}", codeStudioLSPMessageLimit+1)
	if _, err := read(body); err == nil || !strings.Contains(err.Error(), "message exceeds") {
		t.Fatalf("oversize body: err = %v", err)
	}
}
