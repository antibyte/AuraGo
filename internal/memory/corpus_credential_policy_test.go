package memory

import (
	"bytes"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	chromem "github.com/philippgille/chromem-go"
	"gopkg.in/yaml.v3"

	"aurago/internal/promptsource"
)

// seedPendingCorpusIn writes pending documents (as left behind by a model change or
// a legacy migration) into one collection of the corpus. Documents without a
// fingerprint get a stale one.
func seedPendingCorpusIn(t *testing.T, cv *ChromemVectorDB, collection string, docs ...chromem.Document) {
	t.Helper()
	tx, err := cv.corpus.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for _, doc := range docs {
		meta := make(map[string]string, len(doc.Metadata)+1)
		for key, value := range doc.Metadata {
			meta[key] = value
		}
		if _, ok := meta["embedding_fingerprint"]; !ok {
			meta["embedding_fingerprint"] = "old-model"
		}
		doc.Metadata = meta
		if doc.Embedding == nil {
			doc.Embedding = []float32{0, 1, 0}
		}
		if err := writeCorpusDocument(t.Context(), tx, collection, doc); err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func corpusRowState(t *testing.T, cv *ChromemVectorDB, collection, id string) (pending int, blocked string) {
	t.Helper()
	if err := cv.corpus.db.QueryRow(`SELECT pending,blocked_reason FROM documents WHERE collection=? AND id=?`, collection, id).Scan(&pending, &blocked); err != nil {
		t.Fatalf("read state of %s/%s: %v", collection, id, err)
	}
	return pending, blocked
}

func requireCorpusRowState(t *testing.T, cv *ChromemVectorDB, collection, id string, wantPending int, wantBlocked string) {
	t.Helper()
	if pending, blocked := corpusRowState(t, cv, collection, id); pending != wantPending || blocked != wantBlocked {
		t.Fatalf("%s/%s: pending=%d blocked=%q, want pending=%d blocked=%q", collection, id, pending, blocked, wantPending, wantBlocked)
	}
}

// bundledGuideCredentialExample returns a shipped tool-guide chunk that contains a
// reviewed credential demonstration.
func bundledGuideCredentialExample(t *testing.T) chromem.Document {
	t.Helper()
	guides, err := loadToolGuideFiles(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatal(err)
	}
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
		for _, doc := range buildToolGuideDocuments(name, guide.Path, buildToolGuideEmbeddingContent(meta.Description, body)) {
			if HasDurableCredential(doc.Content) {
				return doc
			}
		}
	}
	t.Fatal("no bundled guide chunk carries a credential demonstration")
	return chromem.Document{}
}

// Stored content is re-embedded after a provider, model or URL change and for
// documents written before the credential policy existed. It must pass the same
// policy as a fresh write before any of it reaches the provider; a rejected
// document is neither embedded nor loaded into the index.
func TestCorpusEmbeddingPassesNeverSendCredentialsToTheProvider(t *testing.T) {
	leaky := "deploy note " + batchTestSecret

	t.Run("reconcile", func(t *testing.T) {
		embedder := &recordingEmbedder{}
		cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "vectors"))
		cv.embeddingFunc = embedder.embed
		var logs bytes.Buffer
		cv.logger = slog.New(slog.NewTextHandler(&logs, nil))
		// Fingerprints written before they were redacted carry the endpoint's query
		// credential; that must not block the document, the pass replaces it.
		staleFingerprint := "external|https://api.example.com/v1?key=" + batchTestSecret + "|m"
		seedPendingCorpusDocuments(t, cv,
			chromem.Document{ID: "clean", Content: "ordinary note about gardening", Metadata: map[string]string{"embedding_fingerprint": staleFingerprint}},
			chromem.Document{ID: "leaky", Content: leaky},
			chromem.Document{ID: "leaky-metadata", Content: "harmless text", Metadata: map[string]string{"source": "password=hunter2hunter2xyz"}},
		)

		if err := cv.reconcileCorpus(t.Context()); err != nil {
			t.Fatalf("reconcile reported an error for policy rejections: %v", err)
		}
		if got := embedder.callCount(); got != 1 || embedder.sawText(batchTestSecret) {
			t.Fatalf("provider calls = %d (secret seen: %v), want only the clean document", got, embedder.sawText(batchTestSecret))
		}
		requireCorpusRowState(t, cv, "aurago_memories", "clean", 0, "")
		requireCorpusRowState(t, cv, "aurago_memories", "leaky", 1, "credential_policy")
		requireCorpusRowState(t, cv, "aurago_memories", "leaky-metadata", 1, "credential_policy")
		doc, err := cv.collection.GetByID(t.Context(), "clean")
		if err != nil || doc.Metadata["embedding_fingerprint"] != cv.EmbeddingFingerprint() {
			t.Fatalf("clean document is not published with the current fingerprint: %v %+v", err, doc)
		}
		for _, id := range []string{"leaky", "leaky-metadata"} {
			if _, err := cv.collection.GetByID(t.Context(), id); err == nil {
				t.Fatalf("%s was loaded into the index", id)
			}
		}
		if strings.Contains(logs.String(), batchTestSecret) || strings.Contains(logs.String(), "hunter2") {
			t.Fatalf("logs contain document content: %s", logs.String())
		}
		if got := strings.Count(logs.String(), "Corpus embedding pass finished"); got != 1 {
			t.Fatalf("pass summary logged %d times, want once: %s", got, logs.String())
		}

		// The runtime retry loop does not pick up policy rejections.
		embedder.reset(0)
		if err := cv.runBlockedEmbeddingRetryPass(t.Context()); err != nil || embedder.callCount() != 0 {
			t.Fatalf("retry pass: err=%v provider calls=%d, want an idle pass", err, embedder.callCount())
		}
		requireCorpusRowState(t, cv, "aurago_memories", "leaky", 1, "credential_policy")
	})

	t.Run("retry pass", func(t *testing.T) {
		embedder := &recordingEmbedder{}
		cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "vectors"))
		cv.embeddingFunc = embedder.embed
		seedPendingCorpusDocuments(t, cv,
			chromem.Document{ID: "clean", Content: "ordinary note about gardening"},
			chromem.Document{ID: "leaky", Content: leaky},
		)
		// Blocked by an earlier reconcile that did not check the policy.
		if err := cv.setCorpusBlocked(t.Context(), "aurago_memories", []string{"clean", "leaky"}, "embedding_failed"); err != nil {
			t.Fatal(err)
		}

		if err := cv.runBlockedEmbeddingRetryPass(t.Context()); err != nil {
			t.Fatalf("retry pass: %v", err)
		}
		if got := embedder.callCount(); got != 1 || embedder.sawText(batchTestSecret) {
			t.Fatalf("provider calls = %d (secret seen: %v), want only the clean document", got, embedder.sawText(batchTestSecret))
		}
		requireCorpusRowState(t, cv, "aurago_memories", "clean", 0, "")
		requireCorpusRowState(t, cv, "aurago_memories", "leaky", 1, "credential_policy")
		if _, err := cv.collection.GetByID(t.Context(), "leaky"); err == nil {
			t.Fatal("leaky document was loaded into the index")
		}
	})

	// Documents written by the legacy migration are pending, so the reconcile that
	// follows is what keeps their credentials from the provider.
	t.Run("legacy migration", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "vectors")
		legacy, err := chromem.NewPersistentDB(dir, false)
		if err != nil {
			t.Fatal(err)
		}
		col, err := legacy.GetOrCreateCollection("aurago_memories", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, doc := range []chromem.Document{
			{ID: "legacy-clean", Content: "ordinary legacy note", Metadata: map[string]string{"embedding_fingerprint": "old-model"}, Embedding: []float32{1, 0, 0}},
			{ID: "legacy-leaky", Content: leaky, Metadata: map[string]string{"embedding_fingerprint": "old-model"}, Embedding: []float32{1, 0, 0}},
		} {
			if err := col.AddDocument(t.Context(), doc); err != nil {
				t.Fatal(err)
			}
		}
		embedder := &recordingEmbedder{}
		cv := newCorpusTestVectorDB(t, dir)
		cv.embeddingFunc = embedder.embed

		if err := cv.reconcileCorpus(t.Context()); err != nil {
			t.Fatalf("reconcile: %v", err)
		}
		if got := embedder.callCount(); got != 1 || embedder.sawText(batchTestSecret) {
			t.Fatalf("provider calls = %d (secret seen: %v), want only the clean legacy document", got, embedder.sawText(batchTestSecret))
		}
		requireCorpusRowState(t, cv, "aurago_memories", "legacy-clean", 0, "")
		requireCorpusRowState(t, cv, "aurago_memories", "legacy-leaky", 1, "credential_policy")
		if source, err := cv.corpusDocument(t.Context(), "aurago_memories", "legacy-leaky"); err != nil || source.Content != leaky {
			t.Fatalf("legacy source was changed: %v", err)
		}
	})

	// Byte-identical shipped guide chunks may carry reviewed credential
	// demonstrations; the corpus rows carry the metadata that grant needs.
	t.Run("bundled tool guide exception", func(t *testing.T) {
		example := bundledGuideCredentialExample(t)
		embedder := &recordingEmbedder{}
		cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "vectors"))
		cv.embeddingFunc = embedder.embed
		forged := example
		forged.ID += "_forged"
		forged.Content += "\npassword=hunter2hunter2xyz"
		seedPendingCorpusIn(t, cv, "tool_guides", example, forged)

		if err := cv.reconcileCorpus(t.Context()); err != nil {
			t.Fatalf("reconcile: %v", err)
		}
		requireCorpusRowState(t, cv, "tool_guides", example.ID, 0, "")
		requireCorpusRowState(t, cv, "tool_guides", forged.ID, 1, "credential_policy")
		if got := embedder.callCount(); got != 1 || embedder.sawText("hunter2") {
			t.Fatalf("provider calls = %d (forged text seen: %v), want only the shipped chunk", got, embedder.sawText("hunter2"))
		}
	})
}
