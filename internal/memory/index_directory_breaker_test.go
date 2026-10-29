package memory

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// writeIndexDirectoryFiles creates count small markdown files, one chunk each, and
// returns their paths in the order IndexDirectory visits them.
func writeIndexDirectoryFiles(t *testing.T, count int) (string, []string) {
	t.Helper()
	dir := t.TempDir()
	var paths []string
	for i := range count {
		path := filepath.Join(dir, fmt.Sprintf("note-%02d.md", i))
		if err := os.WriteFile(path, []byte(fmt.Sprintf("# Note %d\n\nContent of file number %d.\n", i, i)), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	return dir, paths
}

func newIndexDirectoryMemory(t *testing.T) *SQLiteMemory {
	t.Helper()
	stm, err := NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stm.Close() })
	return stm
}

// requireFileWriteLockFree fails when something still holds fileWriteMu, the lock
// that ReplaceIndexedFile (the services file indexer) waits on.
func requireFileWriteLockFree(t *testing.T, cv *ChromemVectorDB) {
	t.Helper()
	if !cv.fileWriteMu.TryLock() {
		t.Fatal("fileWriteMu is still held after IndexDirectory returned")
	}
	cv.fileWriteMu.Unlock()
}

// With a provider that answers every file with an error, each file used to spend
// its whole deadline while fileWriteMu blocked every other file writer. The run now
// gives up after a few files in a row and leaves the others for the next run.
func TestIndexDirectoryStopsAfterRepeatedProviderFailures(t *testing.T) {
	for _, tc := range []struct {
		name      string
		failure   error
		wantCalls int64
	}{
		{"server error", &embeddingHTTPStatusError{statusCode: 500}, 3},
		{"unclassified failure", fmt.Errorf("embedding provider unreachable"), 3},
		// The provider answers, so the run goes on with the other files.
		{"document rejected by the provider", &embeddingHTTPStatusError{statusCode: 400}, 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int64
			cv := newTestChromemVectorDB(t, func(context.Context, string) ([]float32, error) {
				calls.Add(1)
				return nil, tc.failure
			})
			dir, paths := writeIndexDirectoryFiles(t, 8)
			stm := newIndexDirectoryMemory(t)

			if err := cv.IndexDirectory(dir, "docs", stm, true); err == nil {
				t.Fatal("IndexDirectory reported success although every file failed")
			}
			if got := calls.Load(); got != tc.wantCalls {
				t.Fatalf("provider calls = %d, want %d", got, tc.wantCalls)
			}
			requireFileWriteLockFree(t, cv)
			for _, path := range paths {
				if indexed, err := stm.GetFileIndex(path, "docs"); err != nil || !indexed.IsZero() {
					t.Fatalf("%s tracked as indexed (%v, %v) although its embedding failed", filepath.Base(path), indexed, err)
				}
			}
		})
	}
}

// A successful file ends the streak, so scattered failures never stop a run.
func TestIndexDirectoryContinuesWhenFailuresAreNotConsecutive(t *testing.T) {
	var calls atomic.Int64
	cv := newTestChromemVectorDB(t, func(context.Context, string) ([]float32, error) {
		if n := calls.Add(1); n%2 == 1 {
			return nil, &embeddingHTTPStatusError{statusCode: 500}
		}
		return []float32{1, 0, 0}, nil
	})
	dir, _ := writeIndexDirectoryFiles(t, 8)
	stm := newIndexDirectoryMemory(t)

	if err := cv.IndexDirectory(dir, "docs", stm, true); err == nil {
		t.Fatal("IndexDirectory reported success although half of the files failed")
	}
	if got := calls.Load(); got != 8 {
		t.Fatalf("provider calls = %d, want all 8 files attempted", got)
	}
}

// Startup validation can disable the vector DB while a directory run is going.
func TestIndexDirectoryStopsWhenTheVectorDBGetsDisabled(t *testing.T) {
	var calls atomic.Int64
	var cv *ChromemVectorDB
	cv = newTestChromemVectorDB(t, func(context.Context, string) ([]float32, error) {
		if calls.Add(1) == 1 {
			cv.disabled.Store(true)
		}
		return []float32{1, 0, 0}, nil
	})
	dir, _ := writeIndexDirectoryFiles(t, 5)
	stm := newIndexDirectoryMemory(t)

	_ = cv.IndexDirectory(dir, "docs", stm, true)
	if got := calls.Load(); got != 1 {
		t.Fatalf("provider calls = %d, want the run to stop once the vector DB is disabled", got)
	}
	requireFileWriteLockFree(t, cv)
}
