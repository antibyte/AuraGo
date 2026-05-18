package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRotatingFileWriterRotatesBySize(t *testing.T) {
	t.Parallel()

	logPath := filepath.Join(t.TempDir(), "remote.log")
	writer, err := newRotatingFileWriter(logPath, 32, 2)
	if err != nil {
		t.Fatalf("newRotatingFileWriter() error = %v", err)
	}
	defer writer.Close()

	chunk := append(bytes.Repeat([]byte("x"), 24), '\n')
	for i := 0; i < 3; i++ {
		if _, err := writer.Write(chunk); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}

	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("current log stat error = %v", err)
	}
	if _, err := os.Stat(logPath + ".1"); err != nil {
		t.Fatalf("first backup stat error = %v", err)
	}
	if _, err := os.Stat(logPath + ".2"); err != nil {
		t.Fatalf("second backup stat error = %v", err)
	}
}
