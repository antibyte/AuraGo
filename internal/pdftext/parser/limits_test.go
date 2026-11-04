package pdf

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestLimitsReachTokenizerAndAggregateStreams(t *testing.T) {
	assertLimit := func(t *testing.T, want error, run func()) {
		t.Helper()
		defer func() {
			cause, _ := recover().(error)
			if !errors.Is(cause, want) {
				t.Fatalf("panic=%v, want %v", cause, want)
			}
		}()
		run()
	}
	t.Run("cancel inside token", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		safety := newSafety(ctx)
		input := &cancelReader{Reader: strings.NewReader(strings.Repeat(" ", 8192)), cancel: cancel}
		assertLimit(t, context.Canceled, func() { newBuffer(input, 0, safety).readToken() })
	})
	t.Run("deadline", func(t *testing.T) {
		safety := newSafety(context.Background())
		safety.deadline = time.Now().Add(-time.Second)
		assertLimit(t, context.DeadlineExceeded, func() { newBuffer(bytes.NewReader(nil), 0, safety).readToken() })
	})
	t.Run("aggregate decoding", func(t *testing.T) {
		safety := newSafety(context.Background())
		safety.decoded = MaxDecodedBytes - 3
		io.ReadAll(&boundedStream{reader: strings.NewReader("ab"), safety: safety})
		assertLimit(t, ErrLimit, func() { io.ReadAll(&boundedStream{reader: strings.NewReader("ab"), safety: safety}) })
	})
	t.Run("xref allocation", func(t *testing.T) {
		safety := newSafety(context.Background())
		assertLimit(t, ErrLimit, func() { ensureXrefLen(nil, int(maxObjectEntries), safety) })
	})
	t.Run("xref cycle", func(t *testing.T) {
		r := &Reader{f: bytes.NewReader(nil), safety: newSafety(context.Background())}
		assertLimit(t, ErrLimit, func() { readPrevXrefs(r, int64(0), func(*buffer) (object, error) { return int64(0), nil }) })
	})
}

type cancelReader struct {
	io.Reader
	cancel context.CancelFunc
}

func (r *cancelReader) Read(p []byte) (int, error) { r.cancel(); return r.Reader.Read(p) }
