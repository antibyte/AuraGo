package memory

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCorpusStatusCountsMetadataWithoutDecodingVectorPayloads(t *testing.T) {
	cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "corpus"))
	// The status path only needs aggregate dimensions; decoding these opaque
	// payloads would fail and would unnecessarily allocate source content.
	_, err := cv.corpus.db.Exec(`INSERT OR IGNORE INTO collections(name) VALUES('aurago_memories'),('kg_embeddings');
		INSERT INTO documents(collection,id,content,metadata,vector,dimension,fingerprint)
		VALUES('aurago_memories','a','fixture','{}',X'FF',3,'fixture'),
		      ('aurago_memories','b','fixture','{}',X'FF',5,'fixture'),
		      ('kg_embeddings','legacy','fixture','{}',X'FF',7,'fixture')`)
	if err != nil {
		t.Fatal(err)
	}
	status := cv.CorpusStatus(t.Context())
	if !status.Available || status.Error != "" || status.Documents != 3 ||
		!reflect.DeepEqual(status.Dimensions, []int{3, 5, 7}) ||
		status.VectorPayloadBytes != 32 || status.DatabaseBytes <= 0 || status.WALBytes < 0 {
		t.Fatalf("status=%+v", status)
	}
	embedding := cv.EmbeddingStatus()
	if embedding.Corpus == nil || embedding.Corpus.Documents != 3 {
		t.Fatalf("embedding status omitted corpus: %+v", embedding)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if status := cv.CorpusStatus(ctx); status.Available || status.Error == "" {
		t.Fatalf("cancelled status looked healthy: %+v", status)
	}
}
