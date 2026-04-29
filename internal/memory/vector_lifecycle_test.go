package memory

import (
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestChromemVectorDBCloseWaitsForAsyncIndexing(t *testing.T) {
	cv := &ChromemVectorDB{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}

	cv.indexingWg.Add(1)
	go func() {
		time.Sleep(50 * time.Millisecond)
		cv.indexingWg.Done()
	}()

	start := time.Now()
	if err := cv.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 45*time.Millisecond {
		t.Fatalf("Close returned before async indexing completed: elapsed=%s", elapsed)
	}
}
