package tools

import (
	"strings"
	"testing"
)

func TestBoundedBuffer(t *testing.T) {
	t.Run("UnderLimit", func(t *testing.T) {
		buf := NewBoundedBuffer(10)
		n, err := buf.Write([]byte("hello"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 5 {
			t.Errorf("expected 5 bytes written, got %d", n)
		}
		if buf.String() != "hello" {
			t.Errorf("expected 'hello', got '%s'", buf.String())
		}
	})

	t.Run("AtLimit", func(t *testing.T) {
		buf := NewBoundedBuffer(5)
		n, err := buf.Write([]byte("hello"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 5 {
			t.Errorf("expected 5 bytes written, got %d", n)
		}
		if buf.String() != "hello" {
			t.Errorf("expected 'hello', got '%s'", buf.String())
		}
	})

	t.Run("OverLimitSingleWrite", func(t *testing.T) {
		buf := NewBoundedBuffer(5)
		n, err := buf.Write([]byte("helloworld"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 10 { // Should pretend to write all
			t.Errorf("expected 10 bytes written, got %d", n)
		}

		str := buf.String()
		if !strings.HasPrefix(str, "hello") {
			t.Errorf("expected string to start with 'hello', got '%s'", str)
		}
		if !strings.Contains(str, "TRUNCATED") {
			t.Errorf("expected truncation warning out, got '%s'", str)
		}
	})

	t.Run("OverLimitMultipleWrites", func(t *testing.T) {
		buf := NewBoundedBuffer(5)
		buf.Write([]byte("hel"))
		n, err := buf.Write([]byte("loworld"))

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 7 {
			t.Errorf("expected 7 bytes written mock, got %d", n)
		}

		str := buf.String()
		if !strings.HasPrefix(str, "hello") {
			t.Errorf("expected string to start with 'hello', got '%s'", str)
		}
		if !strings.Contains(str, "TRUNCATED") {
			t.Errorf("expected truncation warning out, got '%s'", str)
		}
	})

	t.Run("WriteAfterLimit", func(t *testing.T) {
		buf := NewBoundedBuffer(5)
		buf.Write([]byte("hello"))
		n, err := buf.Write([]byte("world"))

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 5 {
			t.Errorf("expected 5 bytes written mock, got %d", n)
		}

		str := buf.String()
		if !strings.HasPrefix(str, "hello") {
			t.Errorf("expected string to start with 'hello', got '%s'", str)
		}
		if !strings.Contains(str, "TRUNCATED") {
			t.Errorf("expected truncation warning out, got '%s'", str)
		}
	})
}
