package tools

import (
	"bytes"
	"strings"
	"testing"
)

func TestReadHTTPResponseBodyHonorsLimit(t *testing.T) {
	data, err := readHTTPResponseBody(strings.NewReader("hello"), 5)
	if err != nil {
		t.Fatalf("readHTTPResponseBody returned error: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("body = %q, want %q", string(data), "hello")
	}
}

func TestReadHTTPResponseBodyRejectsOversizePayload(t *testing.T) {
	_, err := readHTTPResponseBody(bytes.NewBufferString("abcdef"), 5)
	if err == nil {
		t.Fatal("expected oversize response error")
	}
	if !strings.Contains(err.Error(), "exceeds limit") {
		t.Fatalf("unexpected error: %v", err)
	}
}
