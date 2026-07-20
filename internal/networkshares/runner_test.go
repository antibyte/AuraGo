package networkshares

import (
	"bytes"
	"testing"
)

func TestCappedBufferLimitsCapturedOutput(t *testing.T) {
	var buffer cappedBuffer
	input := bytes.Repeat([]byte("x"), maxCommandOutputBytes+4096)
	written, err := buffer.Write(input)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if written != len(input) || len(buffer.Bytes()) != maxCommandOutputBytes {
		t.Fatalf("written=%d captured=%d", written, len(buffer.Bytes()))
	}
}
