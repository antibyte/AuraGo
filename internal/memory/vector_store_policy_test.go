package memory

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/chunking"
	"aurago/internal/promptsource"
	"aurago/internal/security"
	chromem "github.com/philippgille/chromem-go"
	"gopkg.in/yaml.v3"
)

func TestCanonicalDocumentDeletionDoesNotRequireEmbeddingsOrDerivedIndex(t *testing.T) {
	for _, state := range []string{"disabled", "provider_failed", "starting"} {
		t.Run(state, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "vectors")
			cv := newCorpusTestVectorDB(t, dir)
			for _, id := range []string{"single", "batch1", "batch2", " raw conditional ID ", "kept"} {
				doc := chromem.Document{ID: id, Content: "original " + id, Metadata: cv.addEmbeddingMetadata(nil), Embedding: []float32{1, 0, 0}}
				cv.mu.Lock()
				err := cv.persistDocumentsLocked(t.Context(), cv.collection, []chromem.Document{doc})
				cv.mu.Unlock()
				if err != nil {
					t.Fatal(err)
				}
			}
			col, err := cv.db.GetOrCreateCollection("custom", nil, cv.embeddingFunc)
			if err != nil {
				t.Fatal(err)
			}
			if err := cv.persistDocumentsLocked(t.Context(), col, []chromem.Document{{ID: "named", Content: "named source", Metadata: cv.addEmbeddingMetadata(nil), Embedding: []float32{1, 0, 0}}}); err != nil {
				t.Fatal(err)
			}
			cv.disabled.Store(state != "starting")
			cv.ready.Store(state != "starting")
			cv.db = nil
			cv.collection = nil
			cv.embeddingFunc = func(context.Context, string) ([]float32, error) {
				t.Fatal("deletion requested an embedding")
				return nil, nil
			}
			if err := cv.DeleteDocument("single"); err != nil {
				t.Fatal(err)
			}
			if err := cv.DeleteDocuments([]string{"batch1", "batch2"}); err != nil {
				t.Fatal(err)
			}
			if err := cv.DeleteDocumentFromCollection("named", "custom"); err != nil {
				t.Fatal(err)
			}
			if deleted, err := cv.DeleteDocumentIfContentMatches(" raw conditional ID ", contentSHA256("wrong")); err != nil || deleted {
				t.Fatalf("mismatched conditional delete=%v %v", deleted, err)
			}
			if deleted, err := cv.DeleteDocumentIfContentMatches(" raw conditional ID ", contentSHA256("original  raw conditional ID ")); err != nil || !deleted {
				t.Fatalf("conditional delete=%v %v", deleted, err)
			}
			if err := cv.Close(); err != nil {
				t.Fatal(err)
			}
			cv = newCorpusTestVectorDB(t, dir)
			if cv.Count() != 1 {
				t.Fatalf("deletion was not durable: %d", cv.Count())
			}
			if source, err := cv.GetByID("kept"); err != nil || source != "original kept" {
				t.Fatalf("unrelated source changed: %q %v", source, err)
			}
		})
	}
}

func TestCanonicalNamedCollectionRemainsReadableWhenEmbeddingsDisabled(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "vectors")
	cv := newCorpusTestVectorDB(t, dir)
	ids, err := cv.StoreDocumentInCollection("topic", "named collection source", "custom")
	if err != nil {
		t.Fatal(err)
	}
	if err := cv.Close(); err != nil {
		t.Fatal(err)
	}
	cv = newCorpusTestVectorDB(t, dir)
	cv.disabled.Store(true)
	cv.ready.Store(false)
	if source, err := cv.GetByIDFromCollection(ids[0], "custom"); err != nil || !strings.Contains(source, "named collection source") {
		t.Fatalf("disabled source read: %q %v", source, err)
	}
}

func TestUnavailableLegacyVectorStoreKeepsConservativeDeletionPolicy(t *testing.T) {
	cv := newTestChromemVectorDB(t, func(context.Context, string) ([]float32, error) { return nil, errors.New("must not embed") })
	defer cv.Close()
	cv.disabled.Store(true)
	if err := cv.DeleteDocument("unknown"); !errors.Is(err, ErrVectorDBDisabled) {
		t.Fatalf("legacy disabled delete=%v", err)
	}
	if deleted, err := cv.DeleteDocumentIfContentMatches("unknown", "hash"); deleted || !errors.Is(err, ErrVectorDBDisabled) {
		t.Fatalf("legacy conditional delete=%v %v", deleted, err)
	}
}

func TestNewMemoryWritesRejectOversizeWithoutTruncationEmbeddingOrMutation(t *testing.T) {
	cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "vectors"))
	seedVectorPolicySource(t, cv)
	var calls atomic.Int32
	cv.embeddingFunc = func(context.Context, string) ([]float32, error) { calls.Add(1); return []float32{1, 0, 0}, nil }
	oversize := strings.Repeat("界", maxBatchItemBytes/3+1)
	tests := []struct {
		name  string
		store func() error
	}{
		{"single", func() error { _, err := cv.StoreDocument("topic", oversize); return err }},
		{"owned", func() error { _, err := cv.StoreDocumentOwned("topic", oversize, VectorStoreForceCreate); return err }},
		{"context", func() error {
			_, err := cv.StoreDocumentOwnedContext(t.Context(), "topic", oversize, VectorStoreForceCreate)
			return err
		}},
		{"domain", func() error { _, err := cv.StoreDocumentWithDomain("topic", oversize, "home"); return err }},
		{"batch", func() error {
			items := []ArchiveItem{{Concept: "safe", Content: "safe preceding item"}, {Concept: "large", Content: oversize}}
			before := slices.Clone(items)
			ids, err := cv.StoreBatch(items)
			if len(ids) != 0 || !slices.Equal(items, before) {
				t.Fatal("batch partially wrote or changed caller input")
			}
			return err
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.store(); err == nil {
				t.Fatal("oversize write succeeded")
			}
			if calls.Load() != 0 || cv.Count() != 1 {
				t.Fatalf("rejected write had side effects: embeddings=%d sources=%d", calls.Load(), cv.Count())
			}
			if source, err := cv.corpusDocument(t.Context(), "aurago_memories", "existing"); err != nil || source.Content != "existing complete source" {
				t.Fatalf("rejected write changed existing source: %v", err)
			}
		})
	}
}

func TestDirectVectorWritesRejectCredentialsBeforeEmbeddingOrPersistence(t *testing.T) {
	secret := "fixture-durable-credential-26f2d8e9"
	security.RegisterSensitive(secret)
	cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "vectors"))
	seedVectorPolicySource(t, cv)
	var calls atomic.Int32
	cv.embeddingFunc = func(context.Context, string) ([]float32, error) { calls.Add(1); return []float32{1, 0, 0}, nil }
	tests := []struct {
		name  string
		store func() error
	}{
		{"single", func() error { _, err := cv.StoreDocument("topic", secret); return err }},
		{"owned", func() error { _, err := cv.StoreDocumentOwned("topic", secret, VectorStoreForceCreate); return err }},
		{"context", func() error {
			_, err := cv.StoreDocumentOwnedContext(t.Context(), "topic", secret, VectorStoreForceCreate)
			return err
		}},
		{"concept", func() error {
			_, err := cv.StoreDocumentOwned(secret, "ordinary fact", VectorStoreForceCreate)
			return err
		}},
		{"domain", func() error { _, err := cv.StoreDocumentWithDomain("topic", "ordinary fact", secret); return err }},
		{"collection", func() error { _, err := cv.StoreDocumentInCollection("topic", secret, "named"); return err }},
		{"chunking", func() error {
			_, err := cv.StoreDocumentInCollectionWithChunking("topic", secret, "named", chunking.DefaultOptions(), nil)
			return err
		}},
		{"metadata", func() error {
			_, err := cv.StoreDocumentInCollectionWithChunking("topic", "ordinary fact", "named", chunking.DefaultOptions(), map[string]string{"provenance": secret})
			return err
		}},
		{"batch", func() error {
			_, err := cv.StoreBatch([]ArchiveItem{{Concept: "safe", Content: "ordinary fact"}, {Concept: "secret", Content: secret}})
			return err
		}},
		{"multimodal", func() error { _, err := cv.StoreDocumentWithEmbedding("topic", secret, []float32{1, 0, 0}); return err }},
		{"multimodal_collection", func() error {
			_, err := cv.StoreDocumentWithEmbeddingInCollection("topic", secret, []float32{1, 0, 0}, "named")
			return err
		}},
		{"cheatsheet", func() error { return cv.StoreCheatsheet("test", "Guide", secret) }},
		{"attachment", func() error { return cv.StoreCheatsheet("test", "Guide", "ordinary fact", secret) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.store(); !errors.Is(err, ErrAutomaticMemorySkipped) {
				t.Fatalf("credential was accepted: %v", err)
			}
			if calls.Load() != 0 || cv.Count() != 1 {
				t.Fatalf("credential write had side effects: embeddings=%d sources=%d", calls.Load(), cv.Count())
			}
			if source, err := cv.corpusDocument(t.Context(), "aurago_memories", "existing"); err != nil || source.Content != "existing complete source" {
				t.Fatalf("credential rejection changed existing source: %v", err)
			}
		})
	}
}

func seedVectorPolicySource(t *testing.T, cv *ChromemVectorDB) {
	t.Helper()
	doc := chromem.Document{ID: "existing", Content: "existing complete source", Metadata: cv.addEmbeddingMetadata(nil), Embedding: []float32{1, 0, 0}}
	cv.mu.Lock()
	defer cv.mu.Unlock()
	if err := cv.persistDocumentsLocked(t.Context(), cv.collection, []chromem.Document{doc}); err != nil {
		t.Fatal(err)
	}
}

func TestCredentialPolicyDoesNotChangeLegacyMigrationBytes(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "vectors")
	legacy, err := chromem.NewPersistentDB(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	col, err := legacy.GetOrCreateCollection("aurago_memories", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	secret := "fixture-legacy-import-credential-8417bda0"
	security.RegisterSensitive(secret)
	doc := chromem.Document{ID: " raw legacy ID ", Content: "original " + secret, Metadata: map[string]string{"embedding_fingerprint": "old-model", "original_note": secret}, Embedding: []float32{1, 0, 0}}
	if err := col.AddDocument(t.Context(), doc); err != nil {
		t.Fatal(err)
	}
	cv := newCorpusTestVectorDB(t, dir)
	if err := cv.reconcileCorpus(t.Context()); err != nil {
		t.Fatal(err)
	}
	actual, err := cv.corpusDocument(t.Context(), "aurago_memories", doc.ID)
	if err != nil || actual.Content != doc.Content || actual.Metadata["original_note"] != secret {
		t.Fatalf("legacy source was rewritten: %v", err)
	}
}

func TestCredentialPolicyKeepsPersonalFactsCodeAndLegacySourceReindex(t *testing.T) {
	cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "vectors"))
	for _, body := range []string{"My birthday is 12 May 1985.", "I prefer Vim for Go development.", `apiKey := os.Getenv("SERVICE_API_KEY"); client.SetBearerToken(apiKey)`} {
		if _, err := cv.StoreDocumentOwned("preference", body, VectorStoreForceCreate); err != nil {
			t.Fatalf("ordinary source rejected: %v", err)
		}
	}
	secret := "fixture-legacy-credential-16cb578f"
	security.RegisterSensitive(secret)
	tx, err := cv.corpus.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	legacy := chromem.Document{ID: "legacy", Content: "original " + secret, Metadata: map[string]string{"embedding_fingerprint": "previous"}, Embedding: []float32{1, 0, 0}}
	if err := writeCorpusDocument(t.Context(), tx, "aurago_memories", legacy); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := cv.reconcileCorpus(t.Context()); err != nil {
		t.Fatal(err)
	}
	source, err := cv.corpusDocument(t.Context(), "aurago_memories", legacy.ID)
	if err != nil || source.Content != legacy.Content {
		t.Fatalf("legacy source changed or blocked: %v", err)
	}
}

func TestDurableCredentialPolicySeparatesDataNamesTemplatesAndValues(t *testing.T) {
	for _, text := range []string{`{"key":"value"}`, `{"key":"reports/annual.pdf"}`, `{"key":"HOME_ASSISTANT_KEY"}`, `inject_token=true`, `inject_token: false`, `api_key: "your-api-key"`, `OPENAI_API_KEY=${OPENAI_API_KEY}`, `api_key = os.Getenv("SERVICE_API_KEY")`, `My birthday is 12 May 1985.`} {
		if HasDurableCredential(text) {
			t.Fatal("ordinary data or explicit placeholder rejected")
		}
	}
	for _, text := range []string{`api_key=fixture-actual-api-credential-7485`, `password="fixture-password-7492"`, `token: abc12345`, `https://alice:fixture-password-7485@example.com/`, `key=sk-proj-fixture-material-849732174`, `sk-proj-fixture-material-849732174`} {
		if !HasDurableCredential(text) {
			t.Fatal("credential-bearing source accepted")
		}
	}
	const registered = "your_scoped_sensitive_value_7485"
	release := security.RegisterScopedSensitiveExact(registered)
	defer release()
	if !HasDurableCredential("api_key=" + registered) {
		t.Fatal("placeholder syntax bypassed registered secret")
	}
}

func TestBundledGuideExamplesRequireExactCompiledSource(t *testing.T) {
	guides, err := loadToolGuideFiles(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatal(err)
	}
	var credentialExample chromem.Document
	for _, guide := range guides {
		front, body, _, err := promptsource.Split(string(guide.Data))
		if err != nil {
			t.Fatal(err)
		}
		var meta ToolGuideMeta
		if front != "" {
			if err := yaml.Unmarshal([]byte(front), &meta); err != nil {
				t.Fatal(err)
			}
		}
		name := strings.TrimSuffix(guide.Name, ".md")
		docs := buildToolGuideDocuments(name, guide.Path, buildToolGuideEmbeddingContent(meta.Description, body))
		if err := validateNewVectorDocuments(docs); err != nil {
			t.Fatalf("bundled guide rejected: %s: %v", guide.Name, err)
		}
		for _, doc := range docs {
			if HasDurableCredential(doc.Content) {
				credentialExample = doc
			}
		}
	}
	if credentialExample.ID == "" {
		t.Fatal("fixture did not exercise credential demonstration")
	}
	forged := credentialExample
	forged.Content += "\npassword=fixture-unreviewed-credential-26f85"
	if !errors.Is(validateNewVectorDocuments([]chromem.Document{forged}), ErrAutomaticMemorySkipped) {
		t.Fatal("modified source inherited bundled guide grant")
	}
	forged.Content = "api_key=fixture-arbitrary-collection-credential-26f85"
	if !errors.Is(validateNewVectorDocuments([]chromem.Document{forged}), ErrAutomaticMemorySkipped) {
		t.Fatal("metadata alone granted a write exemption")
	}
	release := security.RegisterScopedSensitiveExact(credentialExample.Content)
	if !errors.Is(validateNewVectorDocuments([]chromem.Document{credentialExample}), ErrAutomaticMemorySkipped) {
		t.Fatal("registered secret bypassed compiled source check")
	}
	release()
	if err := validateDurableDocument("arbitrary collection", credentialExample.Content); !errors.Is(err, ErrAutomaticMemorySkipped) {
		t.Fatal("public writes gained guide exemption")
	}
}
