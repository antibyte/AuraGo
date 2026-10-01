package services

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
)

func TestIndexerRetryPreservesPartialWriteReceipts(t *testing.T) {
	fi := &FileIndexer{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	writeErr := errors.New("rate limit exceeded after partial write")
	calls := 0
	ids, err := fi.indexStoreWithRetry(context.Background(), func() ([]string, error) {
		calls++
		return []string{"created"}, writeErr
	}, "file")
	if !errors.Is(err, writeErr) || len(ids) != 1 || ids[0] != "created" || calls != 1 {
		t.Fatalf("ids = %v, err = %v, calls = %d", ids, err, calls)
	}
	calls = 0
	id, err := fi.indexStoreDocWithRetry(context.Background(), func() (string, error) {
		calls++
		return "created", writeErr
	}, "image")
	if !errors.Is(err, writeErr) || id != "created" || calls != 1 {
		t.Fatalf("id = %q, err = %v, calls = %d", id, err, calls)
	}
}
