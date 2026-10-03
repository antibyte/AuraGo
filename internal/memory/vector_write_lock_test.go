package memory

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/chunking"
)

// newBlockingEmbeddingVectorDB returns a store whose provider blocks for texts that
// contain marker until release is called.
func newBlockingEmbeddingVectorDB(t *testing.T, marker string) (*ChromemVectorDB, <-chan struct{}, func()) {
	t.Helper()
	entered := make(chan struct{})
	release := make(chan struct{})
	var enterOnce, releaseOnce sync.Once
	cv := newTestChromemVectorDB(t, func(ctx context.Context, text string) ([]float32, error) {
		if strings.Contains(text, marker) {
			enterOnce.Do(func() { close(entered) })
			select {
			case <-release:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return []float32{1, 0, 0}, nil
	})
	releaseFn := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(releaseFn)
	return cv, entered, releaseFn
}

func TestVectorStoresDoNotHoldWriteLockWhileEmbedding(t *testing.T) {
	const marker = "slow-embedding-marker"
	stores := map[string]func(cv *ChromemVectorDB) error{
		"memory document": func(cv *ChromemVectorDB) error {
			_, err := cv.StoreDocument("slow concept", marker)
			return err
		},
		"collection document": func(cv *ChromemVectorDB) error {
			_, err := cv.StoreDocumentInCollectionWithChunking("slow file", marker, "files", chunking.Options{}, nil)
			return err
		},
		"cheatsheet": func(cv *ChromemVectorDB) error {
			return cv.StoreCheatsheet("cs-slow", "Slow sheet", marker)
		},
	}
	for name, store := range stores {
		t.Run(name, func(t *testing.T) {
			cv, entered, release := newBlockingEmbeddingVectorDB(t, marker)
			storeDone := make(chan error, 1)
			go func() { storeDone <- store(cv) }()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("store never reached the embedding provider")
			}

			searchDone := make(chan error, 1)
			go func() {
				_, err := cv.SearchMemoriesOnlyScoredContext(context.Background(), "unrelated query", 3)
				searchDone <- err
			}()
			select {
			case err := <-searchDone:
				if err != nil {
					t.Fatalf("search: %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("search waited for a store's embedding call; cv.mu is held across the provider call")
			}

			release()
			if err := <-storeDone; err != nil {
				t.Fatalf("store: %v", err)
			}
		})
	}
}

func TestStoreCheatsheetKeepsPreviousVersionWhenEmbeddingFails(t *testing.T) {
	var mu sync.Mutex
	fail := false
	cv := newTestChromemVectorDB(t, func(context.Context, string) ([]float32, error) {
		mu.Lock()
		defer mu.Unlock()
		if fail {
			return nil, errors.New("provider down")
		}
		return []float32{1, 0, 0}, nil
	})
	if err := cv.StoreCheatsheet("cs-1", "Sheet", "first version"); err != nil {
		t.Fatalf("StoreCheatsheet first: %v", err)
	}
	mu.Lock()
	fail = true
	mu.Unlock()
	if err := cv.StoreCheatsheet("cs-1", "Sheet", "second version"); err == nil {
		t.Fatal("StoreCheatsheet succeeded although the provider failed")
	}
	body, err := cv.GetByID("cs_cs-1")
	if err != nil || !strings.Contains(body, "first version") {
		t.Fatalf("previous cheatsheet lost after a failed update: body=%q err=%v", body, err)
	}
}
