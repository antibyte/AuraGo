# Ablaufende Fakten und Salience-Vorfilter – Umsetzungsplan

> **Für ausführende Agenten:** PFLICHT-SKILL: superpowers:subagent-driven-development (empfohlen) oder superpowers:executing-plans, Aufgabe für Aufgabe. Schritte sind Checkboxen (`- [ ]`).

**Ziel:** AuraGo merkt sich zeitlich begrenzte Fakten mit einem Ablaufdatum. Abgelaufene Fakten verschwinden sofort aus dem Abruf und werden nachts archiviert. Ein günstiger Vorfilter erspart dem Extraktions-LLM Züge, die nur Füllwörter oder reine Aufträge enthalten.

**Herkunft:** Vergleich mit divagr18/memlayer am 2026-10-29. Memlayer vergibt bei der Extraktion `expiration_date` und filtert vor der Extraktion per Prototyp-Embeddings. Beides fehlt AuraGo. Heute filtert AuraGo kurzlebige Aussagen weg ([memory_analysis.go:83](../internal/agent/memory_analysis.go)) oder speichert sie für immer.

**Architektur:**
- **Ablaufdatum:**
  - Das Extraktions-LLM liefert pro Fakt optional `expires_on` als Tag (`YYYY-MM-DD`).
  - Go macht daraus das Ende dieses lokalen Tages und speichert es in der neuen Tabelle `memory_expiry(doc_id, expires_at)`.
  - Die Suche (`visibleCandidates`) blendet abgelaufene Dokumente aus.
  - Die Nachtwartung archiviert sie und löscht ihre Vektoren. Dabei bleibt keine Archiv-Identität stehen, damit derselbe kurzlebige Fakt später wieder gelernt werden kann.
- **Vorfilter:**
  - Vor dem LLM-Aufruf in `runMemoryAnalysis` prüft erst eine Wortliste, dann ein Embedding-Vergleich gegen deutsche und englische Prototypsätze.
  - Übersprungen wird nur bei klarem Abstand. Bei jedem Fehler lässt der Filter den Zug durch.
  - Das Embedding der Nutzernachricht kommt meist aus dem Query-Cache, weil RAG dieselbe Nachricht vorher schon eingebettet hat.

**Tech-Stack:** Go, SQLite (modernc), chromem-go, vorhandene Memory-Publikations- und Archiv-Mechanik.

**Bewusst nicht enthalten:**
- **Der gebündelte Helper-Pfad (`AnalyzeTurn`) bekommt keinen Vorfilter.** Er erzeugt im selben Aufruf auch Activity-Digest und Persönlichkeitsanalyse, ein Überspringen würde nichts sparen. Er bekommt aber `expires_on`.
- **Pending Actions bekommen kein Ablaufdatum.** Sie haben einen eigenen Lebenszyklus.
- **Keine Anzeige des Ablaufdatums im Prompt oder im Dashboard.**
- **Kein Konfigurationsschalter.** Die Abruf-Feinsteuerung ist adaptiv (siehe [memory_analysis_rollout.go](../internal/agent/memory_analysis_rollout.go)), und der Filter ist konservativ und fail-open.

**Sicherheitsnetz:** Fakten, die der Vorfilter fälschlich überspringt, erreicht weiterhin die nächtliche Konsolidierung des archivierten Gesprächsverlaufs ([maintenance.go](../internal/agent/maintenance.go), Konsolidierungsphase).

---

## Dateien

| Datei | Änderung |
| --- | --- |
| `internal/memory/memory_expiry.go` | **neu:** Schema-Schritt, `SetMemoryExpiry`, `MemoryExpiry`, `ArchiveExpiredMemoriesContext` |
| `internal/memory/memory_expiry_test.go` | **neu:** Speicher-, Abruf- und Archivtests |
| `internal/memory/schema_migrations.go:103-116` | Schema-Version 17 |
| `internal/memory/file_index_replacement.go:623` | abgelaufene Dokumente im Abruf ausblenden |
| `internal/memory/short_term.go:953-1005` | `memory_expiry` beim Löschen von Vektoren mit aufräumen |
| `internal/memory/long_term.go:315` | `EmbedQueries` über den Query-Cache |
| `internal/memory/long_term_test.go` | Test für `EmbedQueries` |
| `internal/agent/memory_expiry_extraction.go` | **neu:** `memoryFactExpiry`, `recordMemoryExpiry`, `memoryPromptDate` |
| `internal/agent/memory_expiry_extraction_test.go` | **neu** |
| `internal/agent/memory_analysis.go:42-103, 278, 333-453` | Feld `ExpiresOn`, Prompt, Anwendung in drei Schleifen |
| `internal/agent/helper_llm_manager.go:28-33, 74-86, 743-749` | Helper-Prompt und Datum |
| `internal/agent/maintenance.go:1541-1568` | Nachtlauf ruft das Archivieren abgelaufener Fakten auf |
| `internal/agent/memory_salience_gate.go` | **neu:** Vorfilter |
| `internal/agent/memory_salience_gate_test.go` | **neu** |
| `internal/agent/memory_analysis_completion_test.go` | Fall „Vorfilter überspringt Füllwörter“ und Datumsprüfung |
| `internal/memory/AGENTS.md` | Vertrag ergänzen |

**Commit-Regel (parallele Sessions im selben Checkout):** Neue Dateien mit `git add <neue Dateien>` vormerken. Dann `git commit -m "…" -- <alle Pfade der Aufgabe>` ausführen, das committet nur diese Pfade. Fremde geänderte Dateien (`AGENTS.md`, `internal/agent/AGENTS.md` usw.) nie mitcommitten. Vor jedem Commit `detect_changes` laufen lassen (Projektregel in `CLAUDE.md`).

**Testlauf-Hinweis:** `GOTMPDIR` muss in Backslash-Form gesetzt sein, sonst schlagen `t.TempDir`-Pfadtests fehl. Bei Plattenplatzfehlern den Go-Cache auf D: trimmen.

---

### Aufgabe 1: Tabelle `memory_expiry` mit Setzen und Lesen

**Dateien:**
- Neu: `internal/memory/memory_expiry.go`
- Ändern: `internal/memory/schema_migrations.go:103-116`
- Test: `internal/memory/memory_expiry_test.go`

- [ ] **Schritt 1: Failing Test schreiben**

```go
package memory

import (
	"testing"
	"time"
)

func TestMemoryExpiryRoundTripAndClear(t *testing.T) {
	s := newInMemorySQLiteMemory(t)
	ctx := t.Context()
	if _, ok, err := s.MemoryExpiry(ctx, "doc"); err != nil || ok {
		t.Fatalf("fresh expiry ok=%v err=%v", ok, err)
	}
	at := time.Date(2026, 11, 2, 23, 59, 59, 0, time.FixedZone("CET", 3600))
	if err := s.SetMemoryExpiry(ctx, []string{"doc", " ", "other"}, at); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.MemoryExpiry(ctx, "doc")
	if err != nil || !ok || !got.Equal(at) {
		t.Fatalf("expiry=%v ok=%v err=%v, want %v", got, ok, err, at)
	}
	if err := s.SetMemoryExpiry(ctx, []string{"doc"}, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := s.MemoryExpiry(ctx, "doc"); err != nil || ok {
		t.Fatalf("cleared expiry ok=%v err=%v", ok, err)
	}
	if _, ok, _ := s.MemoryExpiry(ctx, "other"); !ok {
		t.Fatal("clearing one document removed another")
	}
}
```

- [ ] **Schritt 2: Test laufen lassen, er muss scheitern**

Befehl: `go test ./internal/memory -run TestMemoryExpiryRoundTripAndClear -count=1`
Erwartet: Build-Fehler `s.MemoryExpiry undefined`.

- [ ] **Schritt 3: Implementieren**

`internal/memory/memory_expiry.go`:

```go
package memory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// memoryExpiryLayout matches SQLite CURRENT_TIMESTAMP, so expires_at compares
// as text against it. Values are always UTC.
const memoryExpiryLayout = "2006-01-02 15:04:05"

func migrateMemoryExpiry(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS memory_expiry (
			doc_id TEXT PRIMARY KEY, expires_at TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_memory_expiry_at ON memory_expiry(expires_at);`)
	if err != nil {
		return fmt.Errorf("create memory expiry schema: %w", err)
	}
	return nil
}

// SetMemoryExpiry records when the given memories stop being true. A zero time
// removes the expiry: the latest statement of a fact decides how long it holds.
func (s *SQLiteMemory) SetMemoryExpiry(ctx context.Context, docIDs []string, expiresAt time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range docIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if expiresAt.IsZero() {
			_, err = tx.ExecContext(ctx, `DELETE FROM memory_expiry WHERE doc_id=?`, id)
		} else {
			_, err = tx.ExecContext(ctx, `INSERT INTO memory_expiry(doc_id,expires_at) VALUES(?,?)
				ON CONFLICT(doc_id) DO UPDATE SET expires_at=excluded.expires_at`, id, expiresAt.UTC().Format(memoryExpiryLayout))
		}
		if err != nil {
			return fmt.Errorf("record memory expiry: %w", err)
		}
	}
	return tx.Commit()
}

// MemoryExpiry returns the recorded end of validity of one memory.
func (s *SQLiteMemory) MemoryExpiry(ctx context.Context, docID string) (time.Time, bool, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT expires_at FROM memory_expiry WHERE doc_id=?`, docID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	at, err := time.ParseInLocation(memoryExpiryLayout, raw, time.UTC)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("parse memory expiry: %w", err)
	}
	return at, true, nil
}
```

In `internal/memory/schema_migrations.go` hinter den Eintrag für Version 16 (Zeile 115) einfügen:

```go
		{version: 17, apply: migrateMemoryExpiry},
```

- [ ] **Schritt 4: Test laufen lassen, er muss bestehen**

Befehl: `go test ./internal/memory -run "TestMemoryExpiryRoundTripAndClear|TestMemorySchema" -count=1`
Erwartet: PASS.

- [ ] **Schritt 5: Commit**

```bash
git add internal/memory/memory_expiry.go internal/memory/memory_expiry_test.go
git commit -m "Record when a memory stops being true." -- internal/memory/memory_expiry.go internal/memory/memory_expiry_test.go internal/memory/schema_migrations.go
```

---

### Aufgabe 2: Abgelaufene Fakten aus dem Abruf ausblenden

**Dateien:**
- Ändern: `internal/memory/file_index_replacement.go:616-623`
- Test: `internal/memory/memory_expiry_test.go`

- [ ] **Schritt 1: Failing Test schreiben** (an `memory_expiry_test.go` anhängen, Importe um `path/filepath` und `github.com/philippgille/chromem-go` ergänzen)

```go
func TestExpiredMemoriesStayOutOfRetrieval(t *testing.T) {
	cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "vectors"))
	stm := cv.fileIndexMemory.Load()
	for _, id := range []string{"expired", "active"} {
		doc := chromem.Document{ID: id, Content: "topic\n\n" + id + " fact", Embedding: []float32{1, 0, 0}, Metadata: cv.addEmbeddingMetadata(nil)}
		cv.mu.Lock()
		err := cv.persistDocumentsLocked(t.Context(), cv.collection, []chromem.Document{doc})
		cv.mu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
		if err := stm.UpsertMemoryMeta(id); err != nil {
			t.Fatal(err)
		}
	}
	if err := stm.SetMemoryExpiry(t.Context(), []string{"expired"}, time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := stm.SetMemoryExpiry(t.Context(), []string{"active"}, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	results, err := cv.queryVisibleCollection(t.Context(), cv.collection, []float32{1, 0, 0}, 2)
	if err != nil || len(results) != 1 || results[0].ID != "active" {
		t.Fatalf("expired memory visible: %+v %v", results, err)
	}
}
```

Den Import-Pfad von chromem aus `document_corpus_test.go` übernehmen.

- [ ] **Schritt 2: Test laufen lassen, er muss scheitern**

Befehl: `go test ./internal/memory -run TestExpiredMemoriesStayOutOfRetrieval -count=1`
Erwartet: FAIL, `expired memory visible` mit zwei Treffern.

- [ ] **Schritt 3: Implementieren**

In `visibleCandidates` die Argumentliste verdoppeln und die Archiv-Abfrage um abgelaufene Dokumente erweitern. Zeilen 619-623 ersetzen durch:

```go
				args := make([]any, 0, len(chunk)*2)
				for _, index := range chunk {
					args = append(args, results[index].ID)
				}
				args = append(args, args...)
				// Expired memories leave retrieval at once; the nightly run archives them.
				rows, err := stm.db.QueryContext(ctx, `SELECT doc_id FROM memory_meta WHERE doc_id IN (`+corpusPlaceholders(len(chunk))+`) AND (verification_status='archived' OR COALESCE(archived_at,'')<>'')
					UNION SELECT doc_id FROM memory_expiry WHERE doc_id IN (`+corpusPlaceholders(len(chunk))+`) AND expires_at<=CURRENT_TIMESTAMP`, args...)
```

Der Rest der Schleife (Map `archived`, Deaktivierung) bleibt unverändert. Er behandelt abgelaufene IDs wie archivierte.

- [ ] **Schritt 4: Tests laufen lassen**

Befehl: `go test ./internal/memory -run "TestExpiredMemoriesStayOutOfRetrieval|TestDocumentCorpusArchivesDoNotConsumeTopKOrDeduplicate|TestCorpusWave6" -count=1`
Erwartet: PASS.

- [ ] **Schritt 5: Commit**

```bash
git commit -m "Hide expired memories from retrieval." -- internal/memory/file_index_replacement.go internal/memory/memory_expiry_test.go
```

---

### Aufgabe 3: Abgelaufene Fakten nachts archivieren

**Dateien:**
- Ändern: `internal/memory/memory_expiry.go`
- Ändern: `internal/memory/short_term.go:972` (in `cleanupDeletedVectorReferencesTx`)
- Test: `internal/memory/memory_expiry_test.go`

- [ ] **Schritt 1: Failing Test schreiben**

```go
func TestArchiveExpiredMemoriesRemovesVectorAndAllowsRelearning(t *testing.T) {
	s := newInMemorySQLiteMemory(t)
	ctx := t.Context()
	vector := &fakeRepairVectorDB{docs: map[string]string{
		"old":    "Maintenance window tonight",
		"later":  "Vacation until next week",
		"pinned": "Door code valid today",
	}}
	for id := range vector.docs {
		if err := s.UpsertMemoryMeta(id); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SetMemoryMetaProtection("pinned", true, false); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMemoryExpiry(ctx, []string{"old", "pinned"}, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMemoryExpiry(ctx, []string{"later"}, time.Now().Add(48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	archived, err := s.ArchiveExpiredMemoriesContext(ctx, vector)
	if err != nil || archived != 1 {
		t.Fatalf("archived=%d err=%v", archived, err)
	}
	meta, err := s.GetMemoryMeta("old")
	if err != nil || !IsMemoryArchived(meta) || meta.ArchivedReason != "expired" {
		t.Fatalf("old meta=%+v err=%v", meta, err)
	}
	if _, ok := vector.docs["old"]; ok {
		t.Fatal("expired vector kept")
	}
	var identities int
	if err := s.db.QueryRow(`SELECT count(*) FROM memory_archive_identities WHERE doc_id='old'`).Scan(&identities); err != nil || identities != 0 {
		t.Fatalf("expired fact blocks relearning: identities=%d err=%v", identities, err)
	}
	if _, ok, _ := s.MemoryExpiry(ctx, "old"); ok {
		t.Fatal("expiry row of archived memory kept")
	}
	if meta, _ := s.GetMemoryMeta("later"); IsMemoryArchived(meta) {
		t.Fatal("unexpired memory archived")
	}
	if _, ok, _ := s.MemoryExpiry(ctx, "later"); !ok {
		t.Fatal("unexpired expiry removed")
	}
	if meta, _ := s.GetMemoryMeta("pinned"); IsMemoryArchived(meta) {
		t.Fatal("protected memory archived")
	}
	if _, ok, _ := s.MemoryExpiry(ctx, "pinned"); ok {
		t.Fatal("protected memory kept its expiry")
	}
}
```

- [ ] **Schritt 2: Test laufen lassen, er muss scheitern**

Befehl: `go test ./internal/memory -run TestArchiveExpiredMemoriesRemovesVectorAndAllowsRelearning -count=1`
Erwartet: Build-Fehler `s.ArchiveExpiredMemoriesContext undefined`.

- [ ] **Schritt 3: Implementieren** (an `memory_expiry.go` anhängen)

```go
// ArchiveExpiredMemoriesContext archives memories whose stated validity ended
// and removes their vectors through the archive cleanup queue. Unlike budget
// archives, an expired fact keeps no archive identity: the user may state the
// same short-lived fact again later. Protected and keep-forever memories only
// lose their expiry.
func (s *SQLiteMemory) ArchiveExpiredMemoriesContext(ctx context.Context, vector VectorDB) (int, error) {
	if vector == nil {
		return 0, nil
	}
	if err := lockMemoryMutex(ctx, &s.automaticMemoryMu); err != nil {
		return 0, err
	}
	defer s.automaticMemoryMu.Unlock()
	s.bindArchiveVector(vector)
	ctx, release, err := s.BeginMemoryPublication(ctx)
	if err != nil {
		return 0, err
	}
	defer release()
	if _, err := s.db.ExecContext(ctx, `DELETE FROM memory_expiry WHERE doc_id NOT IN (SELECT doc_id FROM memory_meta)
		OR doc_id IN (SELECT doc_id FROM memory_meta WHERE protected=1 OR keep_forever=1
			OR verification_status='archived' OR COALESCE(archived_at,'')<>'')`); err != nil {
		return 0, fmt.Errorf("prune memory expiry: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT doc_id FROM memory_expiry WHERE expires_at<=CURRENT_TIMESTAMP ORDER BY expires_at, doc_id LIMIT 500`)
	if err != nil {
		return 0, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	candidates := make([]memoryArchiveCandidate, 0, len(ids))
	for _, id := range ids {
		var candidate memoryArchiveCandidate
		if err := scanMemoryMeta(s.db.QueryRowContext(ctx, `SELECT `+memoryMetaSelectColumns+` FROM memory_meta WHERE doc_id=?`, id), &candidate.meta); err != nil {
			return 0, err
		}
		candidate.identity, candidate.hash, err = s.archiveDocumentIdentity(ctx, vector, id)
		if err != nil {
			return 0, err
		}
		candidates = append(candidates, candidate)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	applied := 0
	for _, candidate := range candidates {
		var current MemoryMeta
		err := scanMemoryMeta(tx.QueryRowContext(ctx, `SELECT `+memoryMetaSelectColumns+` FROM memory_meta WHERE doc_id=?`, candidate.meta.DocID), &current)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return 0, err
		}
		if current.Protected || current.KeepForever || IsMemoryArchived(current) || !memoryMetaEqual(current, candidate.meta) {
			continue
		}
		// Without source content there is no hash that could authorize a deletion.
		if err := archiveMemoryCandidate(ctx, tx, candidate, "expired", "system", candidate.hash != ""); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM memory_archive_identities WHERE doc_id=?`, candidate.meta.DocID); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM memory_expiry WHERE doc_id=?`, candidate.meta.DocID); err != nil {
			return 0, err
		}
		applied++
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit memory expiry archive: %w", err)
	}
	if err := s.cleanupMemoryArchives(ctx, vector, 500); err != nil {
		return applied, err
	}
	return applied, nil
}
```

In `cleanupDeletedVectorReferencesTx` (`internal/memory/short_term.go`) direkt nach dem Block, der `file_embedding_docs` aufräumt, einfügen:

```go
	if err := deleteMemoryIDsContext(ctx, tx, "memory_expiry", "doc_id", normalizedIDs); err != nil {
		return fmt.Errorf("cleanup memory_expiry: %w", err)
	}
```

**Vor dem Edit:** `impact` auf `cleanupDeletedVectorReferencesTx` und `archiveMemoryCandidate` (upstream) laufen lassen, das Risiko notieren und bei HIGH/CRITICAL melden (Projektregel).

- [ ] **Schritt 4: Tests laufen lassen**

Befehl: `go test ./internal/memory -run "TestArchiveExpired|TestArchiveBudget|TestArchiveCleanup|TestMemoryExpiry|TestExpiredMemories" -count=1`
Erwartet: PASS.

- [ ] **Schritt 5: Commit**

```bash
git commit -m "Archive expired memories without blocking them from being learned again." -- internal/memory/memory_expiry.go internal/memory/memory_expiry_test.go internal/memory/short_term.go
```

---

### Aufgabe 4: `EmbedQueries` über den Query-Cache

**Dateien:**
- Ändern: `internal/memory/long_term.go` (hinter `TestEmbedding`, Zeile 328)
- Test: `internal/memory/long_term_test.go`

- [ ] **Schritt 1: Failing Test schreiben**

```go
func TestEmbedQueriesReusesTheQueryCache(t *testing.T) {
	calls := 0
	cv := newTestChromemVectorDB(t, func(_ context.Context, text string) ([]float32, error) {
		calls++
		return []float32{float32(len(text)), 0, 0}, nil
	})
	vectors, err := cv.EmbedQueries(t.Context(), []string{"a", "bb", "a"})
	if err != nil || len(vectors) != 3 || vectors[1][0] != 2 || vectors[2][0] != 1 {
		t.Fatalf("vectors=%v err=%v", vectors, err)
	}
	if calls != 2 {
		t.Fatalf("embedding calls=%d, want 2 (cache hit for the repeat)", calls)
	}
}
```

- [ ] **Schritt 2: Test laufen lassen, er muss scheitern**

Befehl: `go test ./internal/memory -run TestEmbedQueriesReusesTheQueryCache -count=1`
Erwartet: Build-Fehler `cv.EmbedQueries undefined`.

- [ ] **Schritt 3: Implementieren**

```go
// EmbedQueries embeds texts through the shared query cache without writing
// anything. The memory salience gate runs after a turn, when RAG has usually
// cached the same user message already.
func (cv *ChromemVectorDB) EmbedQueries(ctx context.Context, texts []string) ([][]float32, error) {
	if cv.IsDisabled() || !cv.IsReady() {
		return nil, fmt.Errorf("embedding runtime is unavailable")
	}
	vectors := make([][]float32, 0, len(texts))
	for _, text := range texts {
		vector, err := cv.getQueryEmbedding(ctx, text)
		if err != nil {
			return nil, err
		}
		vectors = append(vectors, vector)
	}
	return vectors, nil
}
```

- [ ] **Schritt 4: Test laufen lassen, er muss bestehen**

Befehl: `go test ./internal/memory -run TestEmbedQueriesReusesTheQueryCache -count=1 -race`
Erwartet: PASS. Ohne gcc lokal fällt `-race` weg; dann ohne `-race` laufen lassen und das Rennen später auf aurago-test prüfen.

- [ ] **Schritt 5: Commit**

```bash
git commit -m "Let callers embed queries through the vector store's query cache." -- internal/memory/long_term.go internal/memory/long_term_test.go
```

---

### Aufgabe 5: `expires_on` extrahieren und speichern

**Dateien:**
- Neu: `internal/agent/memory_expiry_extraction.go`
- Neu: `internal/agent/memory_expiry_extraction_test.go`
- Ändern: `internal/agent/memory_analysis.go`
- Ändern: `internal/agent/helper_llm_manager.go`
- Ändern: `internal/agent/memory_analysis_completion_test.go`

**Vor dem Edit:** `impact` auf `applyMemoryAnalysisResultWithContext`, `runMemoryAnalysis` und `AnalyzeTurn` (upstream). `applyMemoryAnalysisResultWithContext` hat mehrere Aufrufer (Helper-Pfad, Echtzeit-Pfad, Tests). Das Risiko notieren.

- [ ] **Schritt 1: Failing Tests schreiben**

`internal/agent/memory_expiry_extraction_test.go`:

```go
package agent

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestMemoryFactExpiry(t *testing.T) {
	loc := time.FixedZone("CET", 3600)
	now := time.Date(2026, 10, 29, 15, 0, 0, 0, loc)
	for _, test := range []struct {
		raw      string
		wantKeep bool
		want     time.Time
	}{
		{raw: "", wantKeep: true},
		{raw: "not a date", wantKeep: true},
		{raw: "2026-10-29", wantKeep: true, want: time.Date(2026, 10, 29, 23, 59, 59, 0, loc)},
		{raw: " 2026-11-02 ", wantKeep: true, want: time.Date(2026, 11, 2, 23, 59, 59, 0, loc)},
		{raw: "2026-10-28", wantKeep: false},
		{raw: "2028-01-01", wantKeep: true},
	} {
		got, keep := memoryFactExpiry(test.raw, now)
		if keep != test.wantKeep || !got.Equal(test.want) {
			t.Errorf("memoryFactExpiry(%q) = %v %v, want %v %v", test.raw, got, keep, test.want, test.wantKeep)
		}
	}
}

func TestMemoryAnalysisStoresFactExpiry(t *testing.T) {
	stm, logger := groundingTestStore(t)
	vdb := &fakeVectorDB{}
	tomorrow := time.Now().AddDate(0, 0, 1)
	result := memoryAnalysisResult{
		Facts: []extractedFact{{Content: "User is on vacation until tomorrow", Category: "personal", Confidence: 0.95, Quote: "I am on vacation until tomorrow", ExpiresOn: tomorrow.Format("2006-01-02")}},
		human: "I am on vacation until tomorrow, so keep answers short.",
	}
	if stored := applyMemoryAnalysisResult(enabledMemoryTestConfig(), logger, stm, vdb, "session-1", result); stored != 1 {
		t.Fatalf("stored=%d concepts=%v", stored, vdb.storedConcepts)
	}
	got, ok, err := stm.MemoryExpiry(context.Background(), "stored-doc")
	want := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 23, 59, 59, 0, time.Local)
	if err != nil || !ok || !got.Equal(want) {
		t.Fatalf("expiry=%v ok=%v err=%v, want %v", got, ok, err, want)
	}
}

func TestMemoryAnalysisDropsFactThatAlreadyExpired(t *testing.T) {
	stm, logger := groundingTestStore(t)
	vdb := &fakeVectorDB{}
	result := memoryAnalysisResult{
		Facts: []extractedFact{{Content: "User was on vacation until yesterday", Category: "personal", Confidence: 0.95, Quote: "I was on vacation until yesterday", ExpiresOn: time.Now().AddDate(0, 0, -1).Format("2006-01-02")}},
		human: "I was on vacation until yesterday, what did I miss?",
	}
	if stored := applyMemoryAnalysisResult(enabledMemoryTestConfig(), logger, stm, vdb, "session-1", result); stored != 0 || len(vdb.storedConcepts) != 0 {
		t.Fatalf("stored=%d concepts=%v", stored, vdb.storedConcepts)
	}
}

func TestMemoryAnalysisPromptsAskForExpiry(t *testing.T) {
	for _, prompt := range []string{memoryAnalysisPrompt, helperTurnBatchPrompt} {
		if !strings.Contains(prompt, "expires_on") {
			t.Fatalf("prompt does not ask for expires_on:\n%s", prompt)
		}
	}
}
```

Hinweis: Falls die Grounding-Prüfung den Inhalt „User is on vacation until tomorrow“ verwirft (Log „Dropped items“), Inhalt und Zitat wörtlich angleichen. Vorbild ist `TestAnalyzeTurnBindsExtractionToTheHumanRequest` mit „User runs Proxmox“ und dem Zitat „I run Proxmox at home“. Am Grounding selbst nichts ändern.

In `checkRealtimeExtraction` (`memory_analysis_completion_test.go`) in der Schleife über die Requests, direkt nach `if i == 0 { firstPrompt = … }`, ergänzen:

```go
		if !strings.Contains(req.Messages[0].Content, "Current date: "+time.Now().Format("2006-01-02")) {
			t.Fatal("extraction prompt carries no current date")
		}
```

Dazu `"time"` importieren.

- [ ] **Schritt 2: Tests laufen lassen, sie müssen scheitern**

Befehl: `go test ./internal/agent -run "TestMemoryFactExpiry|TestMemoryAnalysisStoresFactExpiry|TestMemoryAnalysisDropsFactThatAlreadyExpired|TestMemoryAnalysisPromptsAskForExpiry" -count=1`
Erwartet: Build-Fehler `unknown field ExpiresOn` und `undefined: memoryFactExpiry`.

- [ ] **Schritt 3: Helfer implementieren**

`internal/agent/memory_expiry_extraction.go`:

```go
package agent

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/memory"
)

// memoryPromptDate is the date line the extractors resolve "until Friday" against.
// Day granularity keeps the helper response cache valid for the whole day.
func memoryPromptDate(now time.Time) string {
	return now.Format("2006-01-02 (Monday)")
}

// memoryFactExpiry turns the analyzer's expires_on day into the end of that
// local day. keep is false when the stated day is already over: the item is
// stale before it is stored. Unparseable or implausibly distant days are
// ignored, so the item is kept without an expiry.
func memoryFactExpiry(expiresOn string, now time.Time) (expiry time.Time, keep bool) {
	expiresOn = strings.TrimSpace(expiresOn)
	if expiresOn == "" {
		return time.Time{}, true
	}
	day, err := time.ParseInLocation("2006-01-02", expiresOn, now.Location())
	if err != nil {
		return time.Time{}, true
	}
	end := day.AddDate(0, 0, 1).Add(-time.Second)
	if end.Before(now) {
		return time.Time{}, false
	}
	if end.After(now.AddDate(1, 0, 1)) {
		return time.Time{}, true
	}
	return end, true
}

// recordMemoryExpiry also clears an older expiry when the restated fact has none.
func recordMemoryExpiry(ctx context.Context, logger *slog.Logger, stm *memory.SQLiteMemory, ids []string, expiry time.Time) {
	if stm == nil || len(ids) == 0 {
		return
	}
	if err := stm.SetMemoryExpiry(ctx, ids, expiry); err != nil && logger != nil {
		logger.Warn("[Memory Analysis] Failed to record memory expiry", "error", err)
	}
}
```

- [ ] **Schritt 4: `extractedFact` und Echtzeit-Prompt erweitern** (`memory_analysis.go`)

Im Struct `extractedFact` (Zeilen 42-48) nach `Quote` einfügen:

```go
	// ExpiresOn is the last day (YYYY-MM-DD) the item holds, or empty.
	ExpiresOn string `json:"expires_on,omitempty"`
```

`parseMemoryAnalysisResult` bleibt unverändert. Ein ungültiges `expires_on` darf die Extraktion nicht verwerfen, `memoryFactExpiry` ignoriert es.

In `memoryAnalysisPrompt`:
- Unter „For each extracted item, provide:“ nach der `quote`-Zeile ergänzen:
  `- expires_on: The last day (YYYY-MM-DD) on which the item is still true, when the User message states or clearly implies an end (e.g. "until Friday", "for the next 24 hours", "bis Freitag", "heute Nacht"). Resolve it against the current date below. Use "" when the item holds without a known end.`
- Die Regel `- Only extract genuinely useful, long-term information. Skip transient requests like "show me the logs".` ersetzen durch:
  `- Only extract genuinely useful information: long-term facts, or time-bounded facts that matter in the coming days (give those an expires_on). Skip transient requests like "show me the logs".`
- In der Formatzeile `Each fact, preference and correction is {"content":"","category":"","confidence":0.0,"quote":""}.` das Objekt erweitern zu `{"content":"","category":"","confidence":0.0,"quote":"","expires_on":""}`.
- Vor `User message:` eine Zeile `Current date: %s` und eine Leerzeile einfügen.

Den Aufruf in Zeile 278 anpassen:

```go
		prompt := fmt.Sprintf(memoryAnalysisPrompt, memoryPromptDate(time.Now()), security.IsolateExternalData(userMsg), security.IsolateExternalData(sourceResponse))
```

- [ ] **Schritt 5: Helper-Prompt erweitern** (`helper_llm_manager.go`)

- In der JSON-Form (Zeilen 29-31) bei `facts`, `preferences` und `corrections` jeweils `, "expires_on": ""` hinter `"quote": ""` ergänzen.
- Unter „Rules for memory_analysis:“ nach der Zeile `- Store only durable or near-term useful information.` einfügen:
  `- expires_on: the last day (YYYY-MM-DD) an item is still true when user_request states or clearly implies an end ("until Friday", "bis Freitag", "for the next 24 hours"); resolve it against the current date in the request; "" otherwise.`
- In `buildPrompt` (Zeilen 743-749) die Datumszeile vor den User-Request setzen:

```go
		userPrompt := fmt.Sprintf(
			"Current date: %s\n\nUser request:\n%s\n\nAssistant reply:\n%s\n\nTools used:\n%s\n\nTool summaries:\n%s",
			memoryPromptDate(time.Now()),
			userRequestBlock,
			assistantReplyBlock,
			strings.Join(uniqueActivityStrings(toolNames, 12), ", "),
			toolSummaryBlock,
		)
```

- [ ] **Schritt 6: Ablaufdatum beim Speichern anwenden** (`applyMemoryAnalysisResultWithContext`, Zeilen 354-453)

Vor der Fakten-Schleife (nach `stored := 0`):

```go
	now := time.Now()
```

In **jeder** der drei Schleifen (Facts mit Variable `f`, Preferences mit `p`, Corrections mit `c`) direkt nach dem `ctx.Err()`-Block einfügen, hier am Beispiel Facts:

```go
		expiry, current := memoryFactExpiry(f.ExpiresOn, now)
		if !current {
			continue
		}
```

In jeder Schleife den Fehlerzweig des Speicherns ändern von

```go
						if ctx.Err() == nil {
							queuePendingMemoryAnalysisWrite(ctx, logger, stm, concept, content, details, err)
						}
```

zu

```go
						// A queued retry carries no expiry; a short-lived item is not worth one.
						if ctx.Err() == nil && expiry.IsZero() {
							queuePendingMemoryAnalysisWrite(ctx, logger, stm, concept, content, details, err)
						}
```

Im Erfolgszweig (`} else {`) jeder Schleife als erste Anweisung einfügen:

```go
						recordMemoryExpiry(ctx, logger, stm, ids, expiry)
```

- [ ] **Schritt 7: Tests laufen lassen**

Befehl: `go test ./internal/agent -run "TestMemoryFactExpiry|TestMemoryAnalysis|TestRealtimeMemoryAnalysis|TestAuditMemoryAnalysis|TestHelperLLMManagerAnalyzeTurn|TestParseHelperTurnBatchResult|TestAnalyzeTurnBindsExtraction|TestShouldStoreExtractedMemory" -count=1`
Erwartet: PASS.

- [ ] **Schritt 8: Commit**

```bash
git add internal/agent/memory_expiry_extraction.go internal/agent/memory_expiry_extraction_test.go
git commit -m "Extract when a remembered fact stops being true and store it with the fact." -- internal/agent/memory_expiry_extraction.go internal/agent/memory_expiry_extraction_test.go internal/agent/memory_analysis.go internal/agent/helper_llm_manager.go internal/agent/memory_analysis_completion_test.go
```

---

### Aufgabe 6: Nachtlauf archiviert abgelaufene Fakten

**Dateien:**
- Ändern: `internal/agent/maintenance.go:1541-1546` (`loadNightlyMemoryMeta`)

- [ ] **Schritt 1: Implementieren**

In `loadNightlyMemoryMeta` direkt vor `metas, err := stm.GetAllMemoryMeta(...)` einfügen:

```go
	if ltm != nil {
		if expired, err := stm.ArchiveExpiredMemoriesContext(ctx, ltm); err != nil {
			resultErr = errors.Join(resultErr, err)
			logger.Warn("[Maintenance] Archiving expired memories failed", "error", err)
		} else if expired > 0 {
			logger.Info("[Maintenance] Archived expired memories", "archived", expired)
		}
	}
```

So sieht die anschließende Metadaten-Abfrage den Stand nach dem Archivieren. Ein eigener Test ist nicht nötig: Die Logik steckt in `ArchiveExpiredMemoriesContext` (Aufgabe 3), hier wird nur aufgerufen.

- [ ] **Schritt 2: Bauen und vorhandene Wartungstests laufen lassen**

Befehl: `go build ./... && go test ./internal/agent -run "(?i)(nightly|maintenance)" -count=1`
Erwartet: Build ok, PASS.

- [ ] **Schritt 3: Commit**

```bash
git commit -m "Archive expired memories in the nightly memory run." -- internal/agent/maintenance.go
```

---

### Aufgabe 7: Salience-Vorfilter

**Dateien:**
- Neu: `internal/agent/memory_salience_gate.go`
- Neu: `internal/agent/memory_salience_gate_test.go`

- [ ] **Schritt 1: Failing Tests schreiben**

```go
package agent

import (
	"context"
	"errors"
	"slices"
	"testing"
)

type fakeSalienceEmbedder struct {
	messages map[string][]float32
	err      error
	calls    int
}

func (f *fakeSalienceEmbedder) EmbedQueries(_ context.Context, texts []string) ([][]float32, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	vectors := make([][]float32, 0, len(texts))
	for _, text := range texts {
		switch {
		case f.messages[text] != nil:
			vectors = append(vectors, f.messages[text])
		case slices.Contains(salientMemoryPrototypes, text):
			vectors = append(vectors, []float32{1, 0})
		default:
			vectors = append(vectors, []float32{0, 1})
		}
	}
	return vectors, nil
}

func (f *fakeSalienceEmbedder) EmbeddingFingerprint() string { return "fake-salience" }

func TestMemoryExtractionWorthwhile(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct {
		name       string
		msg        string
		vector     []float32
		err        error
		noEmbedder bool
		want       bool
		reason     string
	}{
		{name: "filler words skip without embedding", msg: "Super, vielen Dank dafür!", want: false, reason: "filler_words"},
		{name: "close to salient", msg: "Mein NAS heißt atlas und läuft mit TrueNAS.", vector: []float32{1, 0}, want: true},
		{name: "close to filler", msg: "Zeig mir bitte die Logs vom Container.", vector: []float32{0, 1}, want: false, reason: "filler_prototype"},
		{name: "borderline stays", msg: "Kannst du das kurz prüfen und mir sagen?", vector: []float32{0.7, 0.71}, want: true},
		{name: "embedder failure fails open", msg: "Zeig mir bitte die Logs vom Container.", err: errors.New("down"), want: true},
		{name: "no embedder keeps", msg: "Zeig mir bitte die Logs vom Container.", noEmbedder: true, want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeSalienceEmbedder{messages: map[string][]float32{test.msg: test.vector}, err: test.err}
			var embedder memoryQueryEmbedder = fake
			if test.noEmbedder {
				embedder = nil
			}
			got, reason := memoryExtractionWorthwhile(ctx, embedder, test.msg)
			if got != test.want || reason != test.reason {
				t.Fatalf("got %v %q, want %v %q", got, reason, test.want, test.reason)
			}
			if test.reason == "filler_words" && fake.calls != 0 {
				t.Fatal("filler words paid for an embedding")
			}
		})
	}
}

func TestOnlySalienceFillerWords(t *testing.T) {
	for msg, want := range map[string]bool{
		"Super, vielen Dank dafür!": true,
		"ok danke 👍":                true,
		"Thanks a lot, perfect.":    true,
		"Ja, mach das morgen":       false,
		"Danke, mein Server heißt atlas": false,
		"": false,
	} {
		if got := onlySalienceFillerWords(msg); got != want {
			t.Errorf("onlySalienceFillerWords(%q) = %v, want %v", msg, got, want)
		}
	}
}
```

- [ ] **Schritt 2: Tests laufen lassen, sie müssen scheitern**

Befehl: `go test ./internal/agent -run "TestMemoryExtractionWorthwhile|TestOnlySalienceFillerWords" -count=1`
Erwartet: Build-Fehler `undefined: memoryExtractionWorthwhile`.

- [ ] **Schritt 3: Implementieren**

`internal/agent/memory_salience_gate.go`:

```go
package agent

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// memoryQueryEmbedder is the part of the vector store the salience gate needs.
type memoryQueryEmbedder interface {
	EmbedQueries(ctx context.Context, texts []string) ([][]float32, error)
	EmbeddingFingerprint() string
}

const (
	// The gate skips only when a message is clearly closer to filler than to
	// anything worth remembering. Missed facts still reach the nightly
	// consolidation of the archived transcript.
	salienceSkipMargin   = 0.05
	salienceMaxGateRunes = 300
	salienceGateTimeout  = 5 * time.Second
	salienceMaxFillerLen = 8
)

var salienceFillerWords = map[string]struct{}{}

func init() {
	for _, word := range strings.Fields(`
		danke dankeschön vielen dank dafür super prima perfekt passt ok okay okey alles klar gut sehr toll cool top
		ja nein jo jep gerne bitte hallo hi hey moin servus tschüss ciao das war es ist
		thanks thank you a lot cheers awesome perfect fine good great nice got it yes no yep sure bye hello for that`) {
		salienceFillerWords[word] = struct{}{}
	}
}

// Prototype sentences in German and English; the embedder is multilingual.
var salientMemoryPrototypes = []string{
	"Ich heiße Maria und wohne in Hamburg.",
	"My name is Alex and I live in Berlin.",
	"Mein Homeserver läuft mit Proxmox auf einem Dell R730.",
	"The NAS uses ZFS and is reachable at 192.168.1.20.",
	"Ich bevorzuge Go gegenüber Python.",
	"I prefer short answers without emojis.",
	"Merk dir bitte, dass ich montags keine Meetings habe.",
	"Remember that the backup runs every night at 3 am.",
	"Nein, das stimmt nicht, ich nutze Podman statt Docker.",
	"That is wrong, I moved from Berlin to Munich.",
	"Mach das in Zukunft bitte immer auf Deutsch.",
	"From now on always ask before deleting files.",
	"Ich bin bis Freitag im Urlaub.",
	"The maintenance window is tonight from 10 pm.",
	"Meine Frau heißt Julia und arbeitet als Ärztin.",
	"Our project deadline is the 15th of November.",
}

var fillerMemoryPrototypes = []string{
	"Danke, das hat geholfen.",
	"Thanks, that works now.",
	"Okay, passt so.",
	"Alright, sounds good.",
	"Zeig mir die Logs vom Container.",
	"Show me the logs of the web server.",
	"Starte den Dienst neu.",
	"Restart the container please.",
	"Fasse diesen Artikel zusammen.",
	"Summarize this page for me.",
	"Übersetze den Text ins Englische.",
	"Translate this into German.",
	"Wie wird das Wetter morgen?",
	"What time is it in Tokyo?",
	"Erklär mir, wie DNS funktioniert.",
	"Explain how a reverse proxy works.",
}

var salienceCache struct {
	mu          sync.Mutex
	fingerprint string
	salient     [][]float32
	filler      [][]float32
}

// memoryExtractionWorthwhile decides before the extraction LLM call whether a
// turn can hold anything worth remembering. It fails open: without an
// embedder, or when embedding fails, the turn goes to the extractor.
func memoryExtractionWorthwhile(ctx context.Context, embedder memoryQueryEmbedder, userMsg string) (bool, string) {
	if onlySalienceFillerWords(userMsg) {
		return false, "filler_words"
	}
	userMsg = strings.TrimSpace(userMsg)
	if embedder == nil || utf8.RuneCountInString(userMsg) > salienceMaxGateRunes {
		return true, ""
	}
	ctx, cancel := context.WithTimeout(ctx, salienceGateTimeout)
	defer cancel()
	salient, filler, err := salienceProtoVectors(ctx, embedder)
	if err != nil {
		return true, ""
	}
	vectors, err := embedder.EmbedQueries(ctx, []string{userMsg})
	if err != nil || len(vectors) != 1 {
		return true, ""
	}
	if maxCosine(vectors[0], filler)-maxCosine(vectors[0], salient) >= salienceSkipMargin {
		return false, "filler_prototype"
	}
	return true, ""
}

func onlySalienceFillerWords(msg string) bool {
	words := strings.FieldsFunc(strings.ToLower(msg), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	if len(words) == 0 || len(words) > salienceMaxFillerLen {
		return false
	}
	for _, word := range words {
		if _, ok := salienceFillerWords[word]; !ok {
			return false
		}
	}
	return true
}

// salienceProtoVectors embeds the prototypes once per embedding model.
func salienceProtoVectors(ctx context.Context, embedder memoryQueryEmbedder) (salient, filler [][]float32, err error) {
	fingerprint := embedder.EmbeddingFingerprint()
	salienceCache.mu.Lock()
	defer salienceCache.mu.Unlock()
	if salienceCache.fingerprint == fingerprint && salienceCache.salient != nil {
		return salienceCache.salient, salienceCache.filler, nil
	}
	texts := append(append([]string(nil), salientMemoryPrototypes...), fillerMemoryPrototypes...)
	vectors, err := embedder.EmbedQueries(ctx, texts)
	if err != nil {
		return nil, nil, err
	}
	if len(vectors) != len(texts) {
		return nil, nil, fmt.Errorf("embedder returned %d prototype vectors for %d texts", len(vectors), len(texts))
	}
	salienceCache.fingerprint = fingerprint
	salienceCache.salient = vectors[:len(salientMemoryPrototypes)]
	salienceCache.filler = vectors[len(salientMemoryPrototypes):]
	return salienceCache.salient, salienceCache.filler, nil
}

func maxCosine(vector []float32, prototypes [][]float32) float64 {
	best := -1.0
	for _, prototype := range prototypes {
		if score := cosine32(vector, prototype); score > best {
			best = score
		}
	}
	return best
}

func cosine32(left, right []float32) float64 {
	if len(left) == 0 || len(left) != len(right) {
		return 0
	}
	var dot, leftNorm, rightNorm float64
	for i := range left {
		dot += float64(left[i]) * float64(right[i])
		leftNorm += float64(left[i]) * float64(left[i])
		rightNorm += float64(right[i]) * float64(right[i])
	}
	if leftNorm == 0 || rightNorm == 0 {
		return 0
	}
	return dot / math.Sqrt(leftNorm*rightNorm)
}
```

**Achtung:** Die Wortliste darf keine Wörter enthalten, die Fakten oder Termine tragen. „morgen“ fehlt deshalb absichtlich: „Ja, morgen“ soll den Extraktor erreichen.

- [ ] **Schritt 4: Tests laufen lassen, sie müssen bestehen**

Befehl: `go test ./internal/agent -run "TestMemoryExtractionWorthwhile|TestOnlySalienceFillerWords" -count=1`
Erwartet: PASS.

- [ ] **Schritt 5: Commit**

```bash
git add internal/agent/memory_salience_gate.go internal/agent/memory_salience_gate_test.go
git commit -m "Add a fail-open salience gate for turn memory extraction." -- internal/agent/memory_salience_gate.go internal/agent/memory_salience_gate_test.go
```

---

### Aufgabe 8: Vorfilter in `runMemoryAnalysis` einhängen

**Dateien:**
- Ändern: `internal/agent/memory_analysis.go:237-239`
- Ändern: `internal/agent/memory_analysis_completion_test.go`

- [ ] **Schritt 1: Failing Testfall ergänzen**

Im Struct `realtimeExtractionCase` ein Feld `user string` ergänzen. In `checkRealtimeExtraction` direkt nach der Zuweisung von `userSource, assistantSource` einfügen:

```go
	if test.user != "" {
		userSource = test.user
	}
```

In der Tabelle von `TestRealtimeMemoryAnalysisCompletionBoundaries` den Fall ergänzen:

```go
		{name: "salience gate skips filler", content: validRealtimeExtraction, user: "Super, vielen Dank dafür!", wantCalls: 0},
```

- [ ] **Schritt 2: Test laufen lassen, er muss scheitern**

Befehl: `go test ./internal/agent -run "TestRealtimeMemoryAnalysisCompletionBoundaries" -count=1`
Erwartet: FAIL im Fall `salience gate skips filler` mit `calls=1`.

- [ ] **Schritt 3: Implementieren**

In `runMemoryAnalysis` nach dem Block `if isAmbiguousShortCommand(userMsg) { return }` einfügen:

```go
	// The batched helper path is not gated: its one call also feeds the
	// activity digest and the personality analysis.
	embedder, _ := ltm.(memoryQueryEmbedder)
	if worth, reason := memoryExtractionWorthwhile(ctx, embedder, userMsg); !worth {
		logger.Info("[Memory Analysis] Salience gate skipped extraction", "reason", reason, "session", sessionID)
		return
	}
```

- [ ] **Schritt 4: Tests laufen lassen**

Befehl: `go test ./internal/agent -run "TestRealtimeMemoryAnalysis|TestAuditMemoryAnalysis|TestMemoryExtractionWorthwhile|TestMemoryAnalysis" -count=1`
Erwartet: PASS. `fakeVectorDB` implementiert `EmbedQueries` nicht, die übrigen Fälle laufen also nur durch die Wortliste und bleiben unverändert.

- [ ] **Schritt 5: Commit**

```bash
git commit -m "Skip the memory extraction call for turns that only hold filler." -- internal/agent/memory_analysis.go internal/agent/memory_analysis_completion_test.go
```

---

### Aufgabe 9: Vertrag dokumentieren, Gesamtprüfung

**Dateien:**
- Ändern: `internal/memory/AGENTS.md` (im Abschnitt mit „Archive identity and cleanup“, ca. Zeile 136)

- [ ] **Schritt 1: Vertrag ergänzen** (Stil der Nachbarzeilen, Englisch)

```markdown
- **Memory expiry**: `memory_expiry(doc_id, expires_at)` holds the UTC end of validity of time-bounded facts (`expires_on` from turn extraction, end of that local day). `visibleCandidates` hides expired documents at once; `ArchiveExpiredMemoriesContext` archives them in the nightly run with reason `expired`, queues vector cleanup and, unlike budget archives, removes the archive identity so the same short-lived fact can be learned again. A restatement without expiry clears it. Protected and keep-forever memories only lose the expiry. Verify `TestMemoryExpiry*`, `TestExpiredMemoriesStayOutOfRetrieval` and `TestArchiveExpiredMemories*`.
```

`internal/agent/AGENTS.md` hat zurzeit fremde Änderungen. Den Vorfilter dort nur eintragen, wenn die Datei beim Landen sauber ist. Sonst eine Zeile in den Commit-Text schreiben und im Abschlussbericht als offenen Punkt nennen.

- [ ] **Schritt 2: Gesamtprüfung**

```bash
go vet ./internal/memory ./internal/agent
```

```bash
go test ./internal/memory -count=1
```

```bash
go test ./internal/agent -count=1 -timeout 30m
```

Erwartet: PASS. Bereits bekannte, unabhängige Fehlschläge (siehe Memory-Notizen zu Browser-Tests) mit Ausgabe nennen, nicht verschweigen.

- [ ] **Schritt 3: Graph-Prüfung und Commit**

`detect_changes({scope: "all"})` laufen lassen (Projektregel). Ein Ergebnis mit `partial` oder `truncated` erneut laufen lassen.

```bash
git commit -m "Document the memory expiry contract." -- internal/memory/AGENTS.md
```

---

## Abnahme auf aurago-test (nach dem Deploy, nicht Teil der Code-Aufgaben)

1. **Ablaufdatum:** Im Chat „Ich bin bis übermorgen im Urlaub“ schreiben. Danach muss es eine Zeile in `memory_expiry` mit dem Ende von übermorgen geben. Prüfen per `sqlite3` lesend auf die Memory-DB.
2. **Füllwörter:** „Super, danke!“ schreiben. Im Log muss `Salience gate skipped extraction reason=filler_words` stehen, ohne LLM-Aufruf für `memory_analysis`.
3. **Messung:** Nach einer Woche den Anteil `Salience gate skipped extraction` an allen Echtzeit-Analysen und die Zahl `Archived expired memories` aus den Nachtläufen auswerten. Das gilt nur, wenn der Helper-Pfad dort aus ist; sonst greift der Vorfilter nicht (siehe „Bewusst nicht enthalten“).
