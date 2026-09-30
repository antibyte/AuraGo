package memory

import (
	"context"
	"math"
	"strings"
	"sync"
	"testing"
)

func nearIdenticalMemoryEmbeddings(_ context.Context, text string) ([]float32, error) {
	angle := 0.0
	if strings.Contains(text, "8080") {
		angle = .01
	} else if strings.Contains(text, "9090") {
		angle = .02
	}
	return []float32{float32(math.Cos(angle)), float32(math.Sin(angle)), 0}, nil
}

func TestMemoryDedupStoresChangedFactsAcrossWritePaths(t *testing.T) {
	for _, path := range []string{"owned", "single", "batch"} {
		t.Run(path, func(t *testing.T) {
			vdb := newTestChromemVectorDB(t, nearIdenticalMemoryEmbeddings)
			store := func(content string) ([]string, error) {
				switch path {
				case "owned":
					result, err := vdb.StoreDocumentOwned("Service port", content, VectorStoreDeduplicate)
					return append(result.CreatedIDs, result.ReusedIDs...), err
				case "batch":
					return vdb.StoreBatch([]ArchiveItem{{Concept: "Service port", Content: content}})
				default:
					return vdb.StoreDocument("Service port", content)
				}
			}
			first, err := store("API listens on port 8080")
			if err != nil || len(first) != 1 {
				t.Fatalf("first: %v, %v", first, err)
			}
			second, err := store("API listens on port 9090")
			if err != nil || len(second) != 1 || first[0] == second[0] || vdb.Count() != 2 {
				t.Fatalf("changed fact lost: first=%v second=%v count=%d err=%v", first, second, vdb.Count(), err)
			}
			content, err := vdb.GetByID(second[0])
			if err != nil || !strings.Contains(content, "9090") {
				t.Fatalf("changed content=%q err=%v", content, err)
			}
		})
	}
}

func TestMemoryDedupRequiresIdenticalCompleteDocumentAndDomain(t *testing.T) {
	vdb := newTestChromemVectorDB(t, nearIdenticalMemoryEmbeddings)
	first, err := vdb.StoreDocumentWithDomain("Service", "line one\r\nline two", "ops")
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := vdb.StoreDocumentWithDomain(" Service ", " line one\nline two ", " ops ")
	if err != nil || len(duplicate) != 1 || duplicate[0] != first[0] {
		t.Fatalf("normalized duplicate=%v first=%v err=%v", duplicate, first, err)
	}
	other, err := vdb.StoreBatch([]ArchiveItem{{Concept: "Service", Content: "line one\nline two", Domain: "personal"}})
	if err != nil || len(other) != 1 || other[0] == first[0] {
		t.Fatalf("different domain=%v err=%v", other, err)
	}
	long := strings.Repeat("A long stable fact. ", 300)
	a, err := vdb.StoreDocumentOwned("Long", long, VectorStoreDeduplicate)
	if err != nil || len(a.CreatedIDs) < 2 {
		t.Fatalf("chunked seed=%+v err=%v", a, err)
	}
	b, err := vdb.StoreDocumentOwned("Long", long, VectorStoreDeduplicate)
	if err != nil || len(b.ReusedIDs) != 0 || len(b.CreatedIDs) != len(a.CreatedIDs) {
		t.Fatalf("chunked duplicate=%+v err=%v", b, err)
	}
	// A chunk can equal a short incoming document but cannot prove full identity.
	doc, err := vdb.collection.GetByID(context.Background(), b.CreatedIDs[0])
	if err != nil {
		t.Fatal(err)
	}
	_, body, _ := strings.Cut(doc.Content, "\n\n")
	partial := newTestChromemVectorDB(t, nearIdenticalMemoryEmbeddings)
	if err := partial.collection.AddDocument(context.Background(), doc); err != nil {
		t.Fatal(err)
	}
	c, err := partial.StoreDocumentOwned("Long", body, VectorStoreDeduplicate)
	if err != nil || len(c.ReusedIDs) != 0 || len(c.CreatedIDs) != 1 {
		t.Fatalf("partial chunk reused=%+v err=%v", c, err)
	}
}

func TestMemoryDedupConcurrentNormalizedConcepts(t *testing.T) {
	vdb := newTestChromemVectorDB(t, nearIdenticalMemoryEmbeddings)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			concept := "Service"
			if i%2 == 0 {
				concept = " Service "
			}
			if _, err := vdb.StoreDocument(concept, "same fact"); err != nil {
				t.Errorf("store: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if vdb.Count() != 1 {
		t.Fatalf("concurrent duplicate count=%d", vdb.Count())
	}
}
