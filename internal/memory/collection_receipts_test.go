package memory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/chunking"
)

func TestCollectionStorePreservesFailedWriteReceipts(t *testing.T) {
	options := chunking.Options{Strategy: "recursive", MaxChars: 100, OverlapChars: 0, MaxChunks: 20}
	content := strings.Repeat("A complete sentence for the indexed document.\n\n", 8)
	chunks, err := chunking.ChunkText(content, chunking.NormalizeOptionsWithDefaults(options))
	if err != nil || len(chunks) < 3 {
		t.Fatalf("fixture chunks = %d, err = %v", len(chunks), err)
	}
	for _, failAt := range []int{1, 2, len(chunks)} {
		t.Run(fmt.Sprint(failAt), func(t *testing.T) {
			var calls atomic.Int32
			cv := newTestChromemVectorDB(t, func(context.Context, string) ([]float32, error) {
				if calls.Add(1) == int32(failAt) {
					return nil, errors.New("embedding failed")
				}
				return []float32{1, 0, 0}, nil
			})
			defer cv.Close()
			ids, err := cv.StoreDocumentInCollectionWithChunking("file", content, "files", options, nil)
			if err == nil || len(ids) != len(chunks) {
				t.Fatalf("receipts = %v, err = %v; want all %d allocated IDs", ids, err, len(chunks))
			}
			found := 0
			for _, id := range ids {
				if body, _ := cv.GetByIDFromCollection(id, "files"); body != "" {
					found++
				}
			}
			if found != cv.Count() {
				t.Fatalf("receipts cover %d documents, database contains %d", found, cv.Count())
			}
		})
	}
}
