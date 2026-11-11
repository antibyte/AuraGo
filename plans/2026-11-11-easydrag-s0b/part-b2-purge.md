[← Plan index](README.md) · Spec: [plans/2026-11-11-easydrag-s0b-spec.md](../2026-11-11-easydrag-s0b-spec.md)

## Part B2: Schema 9, encrypted backups, the scan and the guided purge

This part continues in the Part B worktree `C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials` (branch `feat/easydrag-s0b-credentials`), after Task B.5. Every command starts with `cd <worktree> &&`. Shell is Git Bash. Tasks run in the order B.6 to B.10.

**Depends on Part B1.** These names exist when this part starts and are used as they are, never planned again: `CredentialRedactionMarker`, `CredentialFinding` (`NodeID, Param, Location, Path string; Movable bool; Value string`), `FindLiteralCredentials(doc *Flow) []CredentialFinding`, `literalCredentialsInValue(v any) []string`, `redactCredentialValue(v any) any`, and the move endpoint `POST /api/desktop/flows/{id}/credentials/move` of B.3.

**Facts found while reading the code (they shape the tasks):**
- **The real route prefix is `/api/desktop/flows/...`, not `/api/flows/...`.** `handleFlows` serves it (`internal/server/flows_handlers.go:433`). The new routes are `GET /api/desktop/flows/credentials/scan`, `GET` and `POST /api/desktop/flows/credentials/purge`. They are dispatched next to `parts[0] == "secrets"` (line 479) and need an admin scope for GET too (`flowsRequiredScope`, line 561).
- **Schema refusal today.** `checkStoreSchema` (`store.go:336`) refuses any `schema_version` greater than the `storeSchemaVersion` const. Bumping the const to 9 is the whole "older binary refuses schema 9" mechanism: an AuraGo built with const 8 refuses a version-9 file before it migrates anything. The test `TestC16OpenStoreRefusesANewerSchema` follows the const, so it keeps pinning this.
- **Migration pattern.** `OpenStore` runs `migrateFlowsV<n>` per version. Each one takes `backupFlowsSnapshot(db, logger, ".flows-v<n-1>-backup-*")` (verified `VACUUM INTO` plus `integrity_check`) when the database existed, then migrates in one transaction and verifies. V9 copies V8 (`store_migration_v8.go`).
- **Pins.** `pruneVersionsSQL` (`store.go:587`) keeps a version while a live run in `queued`, `running` or `waiting` has its revision, or a `flow_block_version_pins` / `flow_block_run_pins` row names it. Durable gates (`flow_durable_gates`, pending) are covered by their waiting run; the new predicate also names them. The prune only runs inside `Publish`, so a marked version whose run ends would stay forever if nothing else ran it. B.6 therefore also prunes marked versions when a run finishes and in the hourly retention loop.
- **Rollback and clone.** The only rollback sources are `Service.RestoreVersion` (`service.go:606`) and `Service.RestoreBlockVersion` (`blocks_service.go:111`); both read `Store.GetVersion`. No flow "clone from a version" path exists on main (the other `GetVersion` callers are `GetBlockVersion`, the run document, the durable scheduler and durable status, which must keep reading marked versions for pinned runs). B.6 adds `Store.GetVersionForCopy` and uses it in the two rollback paths; any future clone path must use it too.
- **`flow_runs.doc_hash` is never recomputed or compared against `doc_json`.** `grep -rn "doc_hash\|DocHash\|docHash" internal/flows internal/server` shows only: the column and index (`store.go:104,166`), `CreateRun` (computes sha256 of the test document once and inserts it, `store_runs.go:152-267`) and `GetRunDoc`'s join `d.hash = r.doc_hash` (`store_runs.go:439`). The hash is a lookup key into `flow_test_docs`, so redacting `flow_test_docs.json` and leaving the hash alone is safe. New runs write no `doc_json` at all; only runs from before E4 keep their own copy.
- **Run hold.** `Runner.dispatchLocked` (`runner.go:646`) is the only place that launches waiting runs (it is also called by `Release`, `Acquire`, the durable scheduler and `execute`'s end), and `Runner.slots` counts the runs that hold a slot, that is, the `running` ones (a parked `waiting` run gives its slot back, `runner_slots.go:38`). A hold flag checked in `dispatchLocked`, set only while `slots == 0`, is atomic under `Runner.mu`. B.9 adds it.
- **Restore lock.** `cmd/aurago/main.go:313` takes `flock.New(<installDir>/aurago.lock)` with `TryLock` and exits "BLOCKIERT: AuraGo laeuft bereits!" when it is held. The restore flag takes the same lock first, so it refuses while AuraGo runs. B.7 adds it.
- **Master key.** `main.go:451` calls `security.RetainProcessMasterKey`; `security.ProcessMasterKey()` is how server code reads it afterwards (`cfg.Server.MasterKey` is cleared at `main.go:477`). `go.mod` is `go 1.27.3`, and nothing in the repo uses `x/crypto/hkdf`, so B.7 uses the standard `crypto/hkdf`.
- **Route clash check.** B1 adds only a `credentials` entry (`move`) to `flowRouteSegments`, which `flowRoute` reads after the `{id}`. The collection routes here (`parts[0] == "credentials"`, listed in `flowsCollectionRoutes`) are matched before `flowRoute`, and flow ids are `flow_...`, so `GET /api/desktop/flows/credentials/scan` never reaches the per-flow route.
- **No UI "is admin" flag.** The start page has no such flag. Session users hold every scope; a bearer token needs `desktop:admin`. The hint therefore asks the scan endpoint, and shows nothing on 401 or 403.

**Gates before every commit** (a task states which apply): `GOTMPDIR='D:\tmp\go-build' go vet <package>` and `gofmt -l <files>` (no output) for Go; for any JS or locale change `node scripts/test-easydrag.mjs` (ends with `all EasyDrag checks passed`) and `node scripts/test-easydrag-render-baseline.mjs` (prints `PASS render baseline (unchanged)`). `internal/server` tests run without `GOTMPDIR`. If `gofmt -l` lists a file, run `gofmt -w` on it and check again (struct and map alignment is the usual cause).

**Locale helper.** Tasks B.6, B.8 and B.9 add UI keys to all 16 files. The script `C:/disposable/easydrag-s0b/add-locale-keys.mjs` (written in B.6) appends keys from a JSON file to every `ui/lang/easydrag/<lang>.json` and fails when one of the 16 languages is missing.

---

### Task B.6: Schema 9, version marks, and the rollback guard

**Files:**
- Create: `internal/flows/store_migration_v9.go`, `internal/flows/store_migration_v9_test.go`, `internal/flows/version_purge.go`, `internal/flows/version_purge_test.go`, `internal/server/flows_purge_test.go`, `C:/disposable/easydrag-s0b/add-locale-keys.mjs`, `C:/disposable/easydrag-s0b/keys-b6.json`
- Modify: `internal/flows/store.go:59` (const), `internal/flows/store.go:238-241` (verify call), `internal/flows/store.go:290-297` and `:330-333` (migration calls), `internal/flows/store_runs.go:354-363` (`FinishRun`), `internal/flows/service.go:371-375` (`PruneNow`) and `:610` (`RestoreVersion`), `internal/flows/blocks_service.go:115` (`RestoreBlockVersion`), `internal/server/flows_handlers.go:229-262` (`flowsErrorFrom`), the five migration tests that expect version "8", `ui/lang/easydrag/*.json` (16 files), `ui/desktop_easydrag_test.go` (code member list)
- Test: `internal/flows/store_migration_v9_test.go`, `internal/flows/version_purge_test.go`, `internal/server/flows_purge_test.go`

- [ ] **Step 1: Run the impact analysis and confirm the callers**

```bash
for s in OpenStore FinishRun RestoreVersion RestoreBlockVersion; do node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "$s" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo; done
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && grep -rn "GetVersion(" internal/flows internal/server --include=*.go | grep -v _test.go
```
Report the risk of each. `OpenStore` is HIGH (every flows test and the server use it); the change only adds a migration, say so. For `UNKNOWN`, trust the grep. The grep must list exactly: `blocks_service.go` (two), `durable_scheduler.go`, `durable_status.go`, `service.go`, `store_runs.go`. If another caller reads a version to copy it into a draft or a new flow, route it through `GetVersionForCopy` in Step 5 as well.

- [ ] **Step 2: Write the locale helper and the key for `FLOW_VERSION_PURGED`**

Write `C:/disposable/easydrag-s0b/add-locale-keys.mjs`:

```js
// usage: node add-locale-keys.mjs <worktree> <keys.json>
import fs from 'node:fs';
import path from 'node:path';

const [root, dataFile] = process.argv.slice(2);
const LANGS = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const data = JSON.parse(fs.readFileSync(dataFile, 'utf8'));
for (const [key, texts] of Object.entries(data)) {
    const missing = LANGS.filter(l => typeof texts[l] !== 'string' || !texts[l]);
    if (missing.length) throw new Error(key + ' lacks ' + missing.join(','));
}
for (const lang of LANGS) {
    const file = path.join(root, 'ui/lang/easydrag', lang + '.json');
    let text = fs.readFileSync(file, 'utf8');
    const existing = JSON.parse(text);
    const add = Object.keys(data).filter(k => !(k in existing));
    if (!add.length) continue;
    const head = text.slice(0, text.lastIndexOf('}')).replace(/\s+$/, '');
    text = head + ',\n' + add.map(k => '  ' + JSON.stringify(k) + ': ' + JSON.stringify(data[k][lang])).join(',\n') + '\n}\n';
    JSON.parse(text);
    fs.writeFileSync(file, text);
}
console.log('added up to ' + Object.keys(data).length + ' keys to ' + LANGS.length + ' languages');
```

Write `C:/disposable/easydrag-s0b/keys-b6.json`:

```json
{
  "easydrag.ui.error_flow_version_purged": {
    "cs": "Tuto verzi už nelze použít: obsahovala ručně zadané přihlašovací údaje a je označena k vyčištění.",
    "da": "Denne version kan ikke længere bruges: den indeholdt en indtastet adgangskode og er markeret til oprydning.",
    "de": "Diese Version kann nicht mehr verwendet werden: Sie enthielt ein getipptes Zugangsdatum und ist zur Bereinigung markiert.",
    "el": "Αυτή η έκδοση δεν μπορεί πλέον να χρησιμοποιηθεί: περιείχε πληκτρολογημένα διαπιστευτήρια και έχει επισημανθεί για εκκαθάριση.",
    "en": "This version can no longer be used: it held a typed credential and was marked for cleanup.",
    "es": "Esta versión ya no se puede usar: contenía una credencial escrita a mano y está marcada para su limpieza.",
    "fr": "Cette version ne peut plus être utilisée : elle contenait un identifiant saisi et est marquée pour nettoyage.",
    "hi": "यह संस्करण अब उपयोग नहीं किया जा सकता: इसमें टाइप किया गया क्रेडेंशियल था और इसे सफ़ाई के लिए चिह्नित किया गया है।",
    "it": "Questa versione non può più essere usata: conteneva una credenziale digitata ed è contrassegnata per la pulizia.",
    "ja": "このバージョンは使用できません。入力された認証情報が含まれていたため、クリーンアップ対象になっています。",
    "nl": "Deze versie kan niet meer worden gebruikt: ze bevatte ingetypte inloggegevens en is gemarkeerd voor opruiming.",
    "no": "Denne versjonen kan ikke lenger brukes: den inneholdt innskrevet legitimasjon og er merket for opprydding.",
    "pl": "Tej wersji nie można już używać: zawierała wpisane dane uwierzytelniające i została oznaczona do wyczyszczenia.",
    "pt": "Esta versão não pode mais ser usada: continha uma credencial digitada e está marcada para limpeza.",
    "sv": "Den här versionen kan inte längre användas: den innehöll en inskriven inloggningsuppgift och är markerad för rensning.",
    "zh": "此版本已无法使用：它包含手动输入的凭据，已被标记为待清理。"
  }
}
```

Run it: `cd <worktree> && node C:/disposable/easydrag-s0b/add-locale-keys.mjs . C:/disposable/easydrag-s0b/keys-b6.json`. Expected: `added up to 1 keys to 16 languages`.

There is no Go registry of FLOW codes. A code is registered by the locale key `easydrag.ui.error_<code lowercased>` in all 16 files (`core.errorText` looks it up, `easydrag-core.js:395`) plus the member lists in `ui/desktop_easydrag_test.go` (the map with `"easydrag.ui.error_flow_pack_": {...}`, about line 232). Add to that map the entry `"easydrag.ui.error_flow_version_": {"purged"},` now; B.9 adds `"easydrag.ui.error_flow_purge_": {"busy", "runs_active", "no_key", "no_space", "snapshot_invalid", "unavailable"},`. Add `ui/desktop_easydrag_test.go` to this task's commit.

- [ ] **Step 3: Write the failing tests**

Create `internal/flows/version_purge_test.go`:

```go
package flows

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// publishRevisions creates flow id and publishes it n times: revisions 1..n, revision n live.
func publishRevisions(t *testing.T, s *Store, id string, n int) {
	t.Helper()
	ctx := context.Background()
	f := sampleFlow(id)
	if _, err := s.CreateFlow(ctx, f, "", storeNow); err != nil {
		t.Fatalf("CreateFlow: %v", err)
	}
	draft := 1
	for i := 0; i < n; i++ {
		if i > 0 {
			f.Name = fmt.Sprintf("v%d", i)
			rev, err := s.SaveDraft(ctx, id, f, draft, storeNow)
			if err != nil {
				t.Fatalf("SaveDraft: %v", err)
			}
			draft = rev
		}
		if _, err := s.Publish(ctx, id, draft, storeNow); err != nil {
			t.Fatalf("Publish: %v", err)
		}
	}
}

func versionRevisions(t *testing.T, s *Store, id string) []int {
	t.Helper()
	list, err := s.ListVersions(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	out := []int{}
	for _, v := range list {
		out = append(out, v.Revision)
	}
	return out
}

func markCount(t *testing.T, s *Store) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM flow_version_purge`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestMarkedVersionsGoOnceNothingPinsThem(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	const id = "flow_bbbbbbbbbb"
	publishRevisions(t, s, id, 4)
	// A live run that is waiting still executes revision 2.
	if err := s.CreateRun(ctx, RunRecord{ID: "run_pinned", FlowID: id, Revision: 2, Mode: ModeLive, Status: RunWaiting,
		StartedAt: storeNow, TriggerNode: "n_aaaaaaaa", TriggerType: "manual"}, nil); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	// The live revision is never marked.
	if err := s.MarkVersionsForPurge(ctx, id, []int{1, 2, 3, 4}, storeNow); err != nil {
		t.Fatal(err)
	}
	if got := markCount(t, s); got != 3 {
		t.Fatalf("marks = %d, want 3 (revision 4 is live)", got)
	}
	n, err := s.PrunePurgedVersions(ctx, id)
	if err != nil || n != 2 {
		t.Fatalf("PrunePurgedVersions = %d, %v, want 2 (revisions 1 and 3)", n, err)
	}
	if got := fmt.Sprint(versionRevisions(t, s, id)); got != "[4 2]" {
		t.Fatalf("versions = %s, want [4 2]: the pinned one survives", got)
	}
	// Ending the run unpins revision 2 and deletes it at once.
	if err := s.FinishRun(ctx, "run_pinned", RunResult{Status: RunSuccess, FinishedAt: storeNow}); err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(versionRevisions(t, s, id)); got != "[4]" {
		t.Fatalf("versions after the run ended = %s, want [4]", got)
	}
	if got := markCount(t, s); got != 0 {
		t.Fatalf("marks left = %d, want 0 (they go with their version)", got)
	}
}

func TestRollbackRefusesAMarkedVersion(t *testing.T) {
	svc, _ := newServiceFixture(t, newFakeClock(storeNow))
	ctx := context.Background()
	rec := c16Publish(t, svc, simpleFlow("Gruss"))
	doc := simpleFlow("Gruss 2")
	doc.ID = rec.ID
	rev, _, err := svc.SaveDraft(ctx, rec.ID, doc, rec.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Publish(ctx, rec.ID, rev); err != nil { // revisions 1 and 2, 2 is live
		t.Fatal(err)
	}
	if err := svc.Store().MarkVersionsForPurge(ctx, rec.ID, []int{1}, storeNow); err != nil {
		t.Fatal(err)
	}
	cur, err := svc.GetFlow(ctx, rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.RestoreVersion(ctx, rec.ID, 1, cur.DraftRevision); !errors.Is(err, ErrVersionPurged) {
		t.Fatalf("RestoreVersion of a marked version = %v, want ErrVersionPurged", err)
	}
	if _, err := svc.Store().GetVersionForCopy(ctx, rec.ID, 2); err != nil {
		t.Fatalf("GetVersionForCopy of an unmarked version: %v", err)
	}
}
```

Create `internal/flows/store_migration_v9_test.go`:

```go
package flows

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestFlowsV9MigrationAddsPurgeMarksAfterAVerifiedBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "flows.db")
	s, err := OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	publishRevisions(t, s, "flow_aaaaaaaaaa", 2)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	// Turn it back into a schema 8 database.
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`DROP TABLE flow_version_purge; UPDATE flows_meta SET value='8' WHERE key='schema_version'`); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	s, err = OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	var version string
	if err := s.db.QueryRow(`SELECT value FROM flows_meta WHERE key='schema_version'`).Scan(&version); err != nil || version != "9" {
		t.Fatalf("schema = %q, %v, want 9", version, err)
	}
	if got := markCount(t, s); got != 0 {
		t.Fatalf("marks = %d", got)
	}
	if got := versionRevisions(t, s, "flow_aaaaaaaaaa"); len(got) != 2 {
		t.Fatalf("migration changed the versions: %v", got)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	backups, err := filepath.Glob(filepath.Join(dir, ".flows-v8-backup-*", "flows.db"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups = %v, %v, want one", backups, err)
	}
	backup, err := sql.Open("sqlite", backups[0])
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	if err := backup.QueryRow(`SELECT value FROM flows_meta WHERE key='schema_version'`).Scan(&version); err != nil || version != "8" {
		t.Fatalf("backup schema = %q, %v, want 8", version, err)
	}
	// A reopen at schema 9 only verifies; it takes no second backup.
	s, err = OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	if again, _ := filepath.Glob(filepath.Join(dir, ".flows-v8-backup-*")); len(again) != 1 {
		t.Fatalf("a reopen took another backup: %v", again)
	}
}
```

Create `internal/server/flows_purge_test.go`:

```go
package server

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func TestFlowsRollbackOfAMarkedVersionAnswersFlowVersionPurged(t *testing.T) {
	s, token := newFlowsTestServer(t)
	ctx := context.Background()
	rec := createTestFlow(t, s, greetFlowJSON)
	first, _, err := s.Flows.Publish(ctx, rec.ID, rec.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	first.Draft.Name = "Zweite"
	draft, _, err := s.Flows.SaveDraft(ctx, rec.ID, first.Draft, first.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Flows.Publish(ctx, rec.ID, draft); err != nil { // revisions 1 and 2
		t.Fatal(err)
	}
	if err := s.Flows.Store().MarkVersionsForPurge(ctx, rec.ID, []int{1}, time.Now()); err != nil {
		t.Fatal(err)
	}
	w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+rec.ID+"/versions", token,
		`{"revision":1,"base_revision":`+strconv.Itoa(draft)+`}`)
	if w.Code != http.StatusConflict || flowsBody(t, w)["code"] != "FLOW_VERSION_PURGED" {
		t.Fatalf("rollback of a marked version = %d %s, want 409 FLOW_VERSION_PURGED", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 4: Run the tests and see them fail for the right reason**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows -run 'TestFlowsV9|TestMarkedVersions|TestRollbackRefuses' -count=1 2>&1 | head -20
```
Expected: build failure `undefined: MarkVersionsForPurge` (and `ErrVersionPurged`, `PrunePurgedVersions`, `GetVersionForCopy`). The server test fails the same way.

- [ ] **Step 5: Implement the migration**

Create `internal/flows/store_migration_v9.go`:

```go
package flows

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// versionPurgeSchema marks published versions that the credential purge removes (or removes as soon
// as nothing pins them). A mark goes with its version (ON DELETE CASCADE), so the table never grows
// stale rows, and notes go with the version the same way.
const versionPurgeSchema = `CREATE TABLE flow_version_purge (
	flow_id TEXT NOT NULL,
	revision INTEGER NOT NULL,
	marked_at TEXT NOT NULL,
	PRIMARY KEY (flow_id, revision),
	FOREIGN KEY (flow_id, revision) REFERENCES flow_versions(flow_id, revision) ON DELETE CASCADE
)`

func verifyFlowsV9(db *sql.DB) error {
	version, err := flowSchemaVersion(db)
	if err != nil || version < 9 {
		return err
	}
	var table string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='flow_version_purge'`).Scan(&table); err != nil {
		return fmt.Errorf("verify version purge table: %w", err)
	}
	canonical := func(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), "")) }
	if canonical(table) != canonical(versionPurgeSchema) {
		return errors.New("invalid version purge table")
	}
	return nil
}

func migrateFlowsV9(db *sql.DB, logger *slog.Logger, existing bool) error {
	version, err := flowSchemaVersion(db)
	if err != nil {
		return err
	}
	if version >= 9 {
		return verifyFlowsV9(db)
	}
	if version != 8 {
		return errors.New("flows v9 migration requires schema v8")
	}
	if existing {
		if err := backupFlowsSnapshot(db, logger, ".flows-v8-backup-*"); err != nil {
			return err
		}
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(versionPurgeSchema); err != nil {
		return fmt.Errorf("create version purge table: %w", err)
	}
	result, err := tx.Exec(`UPDATE flows_meta SET value='9' WHERE key='schema_version' AND value='8'`)
	if err != nil {
		return fmt.Errorf("record flows v9 schema: %w", err)
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("record flows v9 schema: version changed")
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return verifyFlowsV9(db)
}
```

Edit `internal/flows/store.go`:
1. Line 59: `const storeSchemaVersion = 8` becomes `const storeSchemaVersion = 9`.
2. After the `verifyFlowsV8` block in `OpenStore` (the block ending `if err := verifyFlowsV8(db); err != nil { _ = db.Close(); return nil, err }`), add the same block for `verifyFlowsV9`.
3. After the `if versionBeforeWrites == 7 { ... migrateFlowsV8 ... }` block, add:
```go
	if versionBeforeWrites >= 6 && versionBeforeWrites <= 8 {
		if err := migrateFlowsV9(db, logger, existing > 0); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
```
4. After the final `if err := migrateFlowsV8(db, logger, existing > 0); err != nil { ... }` at the end of `OpenStore`, add the same call for `migrateFlowsV9`.

The five existing tests that open an older database through `OpenStore` now end at "9". Find them and change the expected final version:
```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && grep -n "version != \"8\"\|, \"8\")" internal/flows/store_migration_v*_test.go
```
In `store_migration_v2_test.go`, `store_migration_v3_test.go` (three places), `store_migration_v4_test.go` change `"8"` to `"9"` where the value is read after `OpenStore`. In `store_migration_v8_test.go` the commit case calls `assertData(s.db, "8")` after `OpenStore`: change it to `"9"` (the rollback case and the backup check stay `"7"`).

- [ ] **Step 6: Implement the marks, the prune and the guards**

Create `internal/flows/version_purge.go`:

```go
package flows

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ErrVersionPurged is returned when a rollback or a copy would read a version that the credential
// purge marked.
var ErrVersionPurged = errors.New("this version was marked for credential cleanup")

// versionPinnedSQL is the SQL predicate "a run, a block pin or a pending gate still needs this
// version". flowExpr and revExpr are SQL expressions for the version's flow id and revision. It is
// the pin rule of pruneVersionsSQL plus pending durable gates (their run is waiting anyway).
func versionPinnedSQL(flowExpr, revExpr string) string {
	return `(EXISTS (SELECT 1 FROM flow_runs r WHERE r.flow_id = ` + flowExpr + ` AND r.revision = ` + revExpr + ` AND r.mode = 'live' AND r.status IN ('queued', 'running', 'waiting'))
	OR EXISTS (SELECT 1 FROM flow_block_version_pins p WHERE p.block_flow_id = ` + flowExpr + ` AND p.block_revision = ` + revExpr + `)
	OR EXISTS (SELECT 1 FROM flow_block_run_pins p WHERE p.block_flow_id = ` + flowExpr + ` AND p.block_revision = ` + revExpr + `)
	OR EXISTS (SELECT 1 FROM flow_durable_gates g WHERE g.flow_id = ` + flowExpr + ` AND g.revision = ` + revExpr + ` AND g.state = 'pending'))`
}

// MarkVersionsForPurge marks published versions of one flow. The live revision is never marked.
func (s *Store) MarkVersionsForPurge(ctx context.Context, flowID string, revisions []int, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, revision := range revisions {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO flow_version_purge (flow_id, revision, marked_at)
			SELECT v.flow_id, v.revision, ? FROM flow_versions v JOIN flows f ON f.id = v.flow_id
			WHERE v.flow_id = ? AND v.revision = ? AND v.revision <> f.live_revision`, formatTime(now), flowID, revision); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// PrunePurgedVersions deletes the marked versions of flowID (every flow when it is empty) that
// nothing pins, with their notes (cascade) and marks (cascade). It deletes with secure_delete on
// and checkpoints the WAL afterwards, so a deleted version leaves no readable copy behind. It returns
// the number of deleted versions.
func (s *Store) PrunePurgedVersions(ctx context.Context, flowID string) (int64, error) {
	var one int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM flow_version_purge LIMIT 1`).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `PRAGMA secure_delete = ON`); err != nil {
		return 0, err
	}
	defer func() { _, _ = conn.ExecContext(context.WithoutCancel(ctx), `PRAGMA secure_delete = OFF`) }()
	res, err := conn.ExecContext(ctx, `DELETE FROM flow_versions
		WHERE (? = '' OR flow_id = ?)
		AND EXISTS (SELECT 1 FROM flow_version_purge m WHERE m.flow_id = flow_versions.flow_id AND m.revision = flow_versions.revision)
		AND revision < COALESCE((SELECT live_revision FROM flows f WHERE f.id = flow_versions.flow_id), 0)
		AND NOT `+versionPinnedSQL("flow_versions.flow_id", "flow_versions.revision"), flowID, flowID)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if n > 0 {
		// Best effort: a reader may hold the WAL; the next checkpoint finishes the job.
		_, _ = conn.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`)
	}
	return n, nil
}

// prunePurgedAfterRun is called when a run ends: the version it pinned may have been waiting for it.
func (s *Store) prunePurgedAfterRun(ctx context.Context, runID string) {
	var flowID string
	if err := s.db.QueryRowContext(ctx, `SELECT flow_id FROM flow_runs WHERE id = ?`, runID).Scan(&flowID); err != nil {
		return
	}
	if _, err := s.PrunePurgedVersions(ctx, flowID); err != nil {
		s.logger.Warn("marked flow versions could not be deleted", "flow_id", flowID, "error", err)
	}
}

// GetVersionForCopy is GetVersion for the callers that copy a version into a draft (rollback, clone).
// A version marked for purge is refused with ErrVersionPurged.
func (s *Store) GetVersionForCopy(ctx context.Context, id string, revision int) (*Flow, error) {
	var marked int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM flow_version_purge WHERE flow_id = ? AND revision = ?`, id, revision).Scan(&marked); err != nil {
		return nil, err
	}
	if marked > 0 {
		return nil, ErrVersionPurged
	}
	return s.GetVersion(ctx, id, revision)
}
```

Edit `internal/flows/store_runs.go` `FinishRun`: replace the last line `return requireRow(r, ErrRunNotFound)` of that function with
```go
	if err := requireRow(r, ErrRunNotFound); err != nil {
		return err
	}
	s.prunePurgedAfterRun(ctx, runID)
	return nil
```
Edit `internal/flows/service.go`: in `RestoreVersion` (`:610`) and in `internal/flows/blocks_service.go` `RestoreBlockVersion` (`:115`) change `s.store.GetVersion(ctx, id, revision)` to `s.store.GetVersionForCopy(ctx, id, revision)`. In `PruneNow` (`:371`) insert before `s.runner.Bus().Sweep()`:
```go
	if _, perr := s.store.PrunePurgedVersions(ctx, ""); perr != nil {
		s.logger.Warn("marked flow versions could not be deleted", "error", perr)
	}
```
Edit `internal/server/flows_handlers.go` `flowsErrorFrom`: add after the `ErrRevisionConflict` case
```go
	case errors.Is(err, flows.ErrVersionPurged):
		flowsError(w, http.StatusConflict, "FLOW_VERSION_PURGED", "this version was marked for credential cleanup")
```

- [ ] **Step 7: Run the tests and see them pass**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows -run 'TestFlowsV9|TestMarkedVersions|TestRollbackRefuses|Migration|TestC16|TestAudit18' -count=1 2>&1 | tail -15
go test ./internal/server -run 'TestFlowsRollbackOfAMarkedVersion' -count=1 2>&1 | tail -5
```
Expected: `ok` for both. A leftover `version != "8"` failure names the test file to fix (Step 5).

- [ ] **Step 8: Gates and commit**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && GOTMPDIR='D:\tmp\go-build' go vet ./internal/flows && go vet ./internal/server && gofmt -l internal/flows internal/server && node scripts/test-easydrag.mjs | tail -1 && node scripts/test-easydrag-render-baseline.mjs | tail -1
```
Expected: no vet or gofmt output, `all EasyDrag checks passed`, `PASS render baseline (unchanged)`. Also run the 16-locale parity guard: `go test ./ui -run 'EasyDrag' -count=1 2>&1 | tail -3` (browser tests skip without the env var) expected `ok`.

Write `C:/disposable/easydrag-s0b/msg-b6.txt` and commit:
```
feat(flows): schema 9, purge marks and the rollback guard

flows.db schema 9 adds flow_version_purge (marks cascade with their
version). The migration takes the usual verified snapshot; an AuraGo
with schema 8 refuses the file through checkStoreSchema. Marked versions
are deleted once no live run, block pin or pending gate needs them: when
a run ends, in the hourly retention pass and by the purge. The delete
runs with secure_delete and a WAL checkpoint. Rollback of a marked
version answers 409 FLOW_VERSION_PURGED.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <your model line>
```
```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && git add internal/flows/store_migration_v9.go internal/flows/store_migration_v9_test.go internal/flows/version_purge.go internal/flows/version_purge_test.go internal/flows/store.go internal/flows/store_runs.go internal/flows/service.go internal/flows/blocks_service.go internal/flows/store_migration_v2_test.go internal/flows/store_migration_v3_test.go internal/flows/store_migration_v4_test.go internal/flows/store_migration_v8_test.go internal/server/flows_handlers.go internal/server/flows_purge_test.go ui/desktop_easydrag_test.go ui/lang/easydrag && git commit -F C:/disposable/easydrag-s0b/msg-b6.txt
```

---
### Task B.7: Master-key file encryption, the backup sweeper and the restore flag

**Files:**
- Create: `internal/security/file_encrypt.go`, `internal/security/file_encrypt_test.go`, `internal/flows/purge_files.go`, `internal/flows/purge_files_test.go`, `cmd/aurago/restore_flows_purge_backup.go`, `cmd/aurago/restore_flows_purge_backup_test.go`
- Modify: `internal/flows/service.go:111-126` (`ServiceConfig`), `:158-188` (`Service` struct), `:355-369` (`retentionLoop`), `cmd/aurago/main.go:207` (flag) and `:288` (hook), `internal/flows/AGENTS.md` (one section)
- Test: the four `_test.go` files above

- [ ] **Step 1: Confirm the facts this task relies on**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && grep -n "^go " go.mod && grep -rn "x/crypto/hkdf\|crypto/hkdf" --include=*.go internal cmd | head -3; sed -n 296,316p cmd/aurago/main.go; grep -n "if healthcheck || printTsNetStateDirOnly" cmd/aurago/main.go
```
Expected: `go 1.27.3`; no hkdf import anywhere (so the standard `crypto/hkdf`, which exists since Go 1.24, is used and `go.mod` does not change); the `flock.New(absLockPath)` block; one `if healthcheck || printTsNetStateDirOnly {` line (the hook goes right before it). If a different count shows, anchor the hook on the line that follows the `loadDotEnv(filepath.Join(filepath.Dir(configFile), ".env"), appLog)` call and the password resolution instead.

- [ ] **Step 2: Write the failing security tests**

Create `internal/security/file_encrypt_test.go`:

```go
package security

import (
	"bytes"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var fileCryptTestKey = strings.Repeat("0123456789abcdef", 4)

const fileCryptTestLabel = "aurago/easydrag/purge-backup/v1"

func fileCryptFixture(t *testing.T, size int) (dir, src, enc string, plain []byte) {
	t.Helper()
	dir = t.TempDir()
	plain = make([]byte, size)
	if _, err := rand.Read(plain); err != nil {
		t.Fatal(err)
	}
	src = filepath.Join(dir, "plain.bin")
	enc = filepath.Join(dir, "backup.enc")
	if err := os.WriteFile(src, plain, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EncryptFileWithMasterKey(src, enc, fileCryptTestLabel, fileCryptTestKey); err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	return dir, src, enc, plain
}

func TestFileEncryptionRoundTrip(t *testing.T) {
	for _, size := range []int{0, 1, fileCryptChunkSize - 1, fileCryptChunkSize, fileCryptChunkSize + 1, 3*fileCryptChunkSize + 5} {
		dir, _, enc, plain := fileCryptFixture(t, size)
		data, err := os.ReadFile(enc)
		if err != nil {
			t.Fatal(err)
		}
		if size >= 32 && bytes.Contains(data, plain[:32]) {
			t.Fatalf("size %d: the encrypted file holds plaintext", size)
		}
		out := filepath.Join(dir, "out.bin")
		if err := DecryptFileWithMasterKey(enc, out, fileCryptTestLabel, fileCryptTestKey); err != nil {
			t.Fatalf("size %d: decrypt: %v", size, err)
		}
		got, err := os.ReadFile(out)
		if err != nil || !bytes.Equal(got, plain) {
			t.Fatalf("size %d: round trip differs (%v)", size, err)
		}
	}
}

func TestFileEncryptionRefusesWrongInputs(t *testing.T) {
	dir, _, enc, _ := fileCryptFixture(t, 3*fileCryptChunkSize+5)
	data, err := os.ReadFile(enc)
	if err != nil {
		t.Fatal(err)
	}
	sealedChunk := fileCryptChunkSize + fileCryptTagSize
	flip := func(at int) []byte {
		c := append([]byte(nil), data...)
		c[at] ^= 0x01
		return c
	}
	cases := map[string][]byte{
		"cut at a chunk boundary":    data[:fileCryptHeaderSize+sealedChunk],
		"cut one byte short":         data[:len(data)-1],
		"header only":                data[:fileCryptHeaderSize],
		"flipped bit in a chunk":     flip(fileCryptHeaderSize + 100),
		"flipped bit in the header":  flip(fileCryptHeaderSize - 3),
		"flipped bit in the last tag": flip(len(data) - 1),
	}
	for name, content := range cases {
		bad := filepath.Join(dir, "bad.enc")
		if err := os.WriteFile(bad, content, 0o600); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(dir, "out-"+strings.ReplaceAll(name, " ", "-"))
		if err := DecryptFileWithMasterKey(bad, out, fileCryptTestLabel, fileCryptTestKey); err == nil {
			t.Fatalf("%s: decrypt succeeded", name)
		}
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Fatalf("%s: a failed decrypt left %s", name, out)
		}
	}
	if err := DecryptFileWithMasterKey(enc, filepath.Join(dir, "x1"), "another/label", fileCryptTestKey); err == nil {
		t.Fatal("a wrong label decrypted")
	}
	if err := DecryptFileWithMasterKey(enc, filepath.Join(dir, "x2"), fileCryptTestLabel, strings.Repeat("ab", 32)); err == nil {
		t.Fatal("a wrong key decrypted")
	}
	if err := EncryptFileWithMasterKey(enc, filepath.Join(dir, "x3"), fileCryptTestLabel, "not hex"); err == nil {
		t.Fatal("a malformed master key encrypted")
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".enc-*.part")); len(left) != 0 {
		t.Fatalf("temporary files left: %v", left)
	}
}
```

Run: `cd <worktree> && GOTMPDIR='D:\tmp\go-build' go test ./internal/security -run TestFileEncryption -count=1 2>&1 | head`. Expected: build failure `undefined: EncryptFileWithMasterKey`.

- [ ] **Step 3: Implement the encryption**

Create `internal/security/file_encrypt.go`:

```go
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// File format: magic "AGFE", version byte, chunk size (uint32, big endian), 16-byte random file
// salt, then AES-256-GCM chunks. A chunk holds up to 64 KiB of plaintext plus a 16-byte tag. The
// key is HKDF-SHA256(master key, salt = file salt, info = label), so every file has its own key
// and a counter nonce is safe. The authenticated data of a chunk is the header plus a flag that
// is 1 for the last chunk: cutting the file after a chunk boundary, or reordering chunks, fails.
const (
	fileCryptMagic      = "AGFE"
	fileCryptVersion    = 1
	fileCryptChunkSize  = 64 * 1024
	fileCryptSaltSize   = 16
	fileCryptTagSize    = 16
	fileCryptHeaderSize = len(fileCryptMagic) + 1 + 4 + fileCryptSaltSize
)

// ErrFileDecrypt means the file cannot be read with this key and label, or it is damaged or cut short.
var ErrFileDecrypt = errors.New("the encrypted file cannot be read: wrong key or label, or the file is damaged or cut short")

func fileCryptAEAD(masterKeyHex, label string, salt []byte) (cipher.AEAD, error) {
	master, err := hex.DecodeString(masterKeyHex)
	if err != nil || len(master) != 32 {
		return nil, errors.New("the master key must be 64 hexadecimal characters")
	}
	defer clear(master)
	key, err := hkdf.Key(sha256.New, master, salt, label, 32)
	if err != nil {
		return nil, err
	}
	defer clear(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func fileCryptNonce(index uint64) []byte {
	nonce := make([]byte, 12)
	binary.BigEndian.PutUint64(nonce[4:], index)
	return nonce
}

func fileCryptAAD(header []byte, final bool) []byte {
	aad := append(append(make([]byte, 0, len(header)+1), header...), 0)
	if final {
		aad[len(aad)-1] = 1
	}
	return aad
}

// fileCryptOutput creates the temporary file next to dst (mode 0600) that is renamed over dst on success.
func fileCryptOutput(dst string) (*os.File, error) {
	return os.CreateTemp(filepath.Dir(dst), ".enc-*.part")
}

func fileCryptFinish(out *os.File, dst string, err error) error {
	if err == nil {
		err = out.Sync()
	}
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(out.Name(), dst)
	}
	if err != nil {
		_ = os.Remove(out.Name())
	}
	return err
}

// EncryptFileWithMasterKey writes src encrypted to dst (mode 0600). masterKeyHex is the 64-hex-character
// master key; label separates uses of the same key.
func EncryptFileWithMasterKey(src, dst, label, masterKeyHex string) error {
	salt := make([]byte, fileCryptSaltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return err
	}
	aead, err := fileCryptAEAD(masterKeyHex, label, salt)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open file to encrypt: %w", err)
	}
	defer in.Close()
	out, err := fileCryptOutput(dst)
	if err != nil {
		return fmt.Errorf("create encrypted file: %w", err)
	}
	header := append([]byte(fileCryptMagic), fileCryptVersion)
	header = binary.BigEndian.AppendUint32(header, fileCryptChunkSize)
	header = append(header, salt...)
	return fileCryptFinish(out, dst, encryptChunks(in, out, aead, header))
}

func encryptChunks(in io.Reader, out io.Writer, aead cipher.AEAD, header []byte) error {
	if _, err := out.Write(header); err != nil {
		return err
	}
	cur, next := make([]byte, fileCryptChunkSize), make([]byte, fileCryptChunkSize)
	n, rerr := io.ReadFull(in, cur)
	for index := uint64(0); ; index++ {
		if rerr != nil && rerr != io.EOF && rerr != io.ErrUnexpectedEOF {
			return rerr
		}
		final := rerr != nil // a short or empty read is the end of the file
		var nn int
		var nerr error
		if !final {
			nn, nerr = io.ReadFull(in, next)
			if nerr == io.EOF {
				final = true
			} else if nerr != nil && nerr != io.ErrUnexpectedEOF {
				return nerr
			}
		}
		if _, err := out.Write(aead.Seal(nil, fileCryptNonce(index), cur[:n], fileCryptAAD(header, final))); err != nil {
			return err
		}
		if final {
			return nil
		}
		cur, next = next, cur
		n, rerr = nn, nerr
	}
}

// DecryptFileWithMasterKey writes the plaintext of src to dst. Any failure (wrong key or label, a
// flipped bit, a truncated file) returns ErrFileDecrypt and leaves no dst behind.
func DecryptFileWithMasterKey(src, dst, label, masterKeyHex string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open encrypted file: %w", err)
	}
	defer in.Close()
	header := make([]byte, fileCryptHeaderSize)
	if _, err := io.ReadFull(in, header); err != nil ||
		string(header[:len(fileCryptMagic)]) != fileCryptMagic || header[len(fileCryptMagic)] != fileCryptVersion ||
		binary.BigEndian.Uint32(header[len(fileCryptMagic)+1:]) != fileCryptChunkSize {
		return ErrFileDecrypt
	}
	aead, err := fileCryptAEAD(masterKeyHex, label, header[len(header)-fileCryptSaltSize:])
	if err != nil {
		return err
	}
	out, err := fileCryptOutput(dst)
	if err != nil {
		return fmt.Errorf("create decrypted file: %w", err)
	}
	return fileCryptFinish(out, dst, decryptChunks(in, out, aead, header))
}

func decryptChunks(in io.Reader, out io.Writer, aead cipher.AEAD, header []byte) error {
	size := fileCryptChunkSize + fileCryptTagSize
	cur, next := make([]byte, size), make([]byte, size)
	n, rerr := io.ReadFull(in, cur)
	for index := uint64(0); ; index++ {
		if rerr != nil && rerr != io.ErrUnexpectedEOF {
			return ErrFileDecrypt // no chunk at all, or a read error
		}
		final := rerr != nil
		var nn int
		var nerr error
		if !final {
			nn, nerr = io.ReadFull(in, next)
			if nerr == io.EOF {
				final = true
			} else if nerr != nil && nerr != io.ErrUnexpectedEOF {
				return ErrFileDecrypt
			}
		}
		plain, err := aead.Open(nil, fileCryptNonce(index), cur[:n], fileCryptAAD(header, final))
		if err != nil {
			return ErrFileDecrypt
		}
		if _, err := out.Write(plain); err != nil {
			return err
		}
		if final {
			return nil
		}
		cur, next = next, cur
		n, rerr = nn, nerr
	}
}
```

- [ ] **Step 4: Run the security tests**

`cd <worktree> && GOTMPDIR='D:\tmp\go-build' go test ./internal/security -run TestFileEncryption -count=1 -v 2>&1 | tail -8`. Expected: both tests `PASS`, then `ok`. If `crypto/hkdf` is not found, the toolchain is older than 1.24: run `go version`; use `golang.org/x/crypto/hkdf` instead (`hkdf.New(sha256.New, master, salt, []byte(label))` read 32 bytes; `go.mod` already requires `x/crypto v0.58.0`).

- [ ] **Step 5: Write the failing sweeper and restore tests**

Create `internal/flows/purge_files_test.go`:

```go
package flows

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"aurago/internal/security"
)

var testPurgeKey = strings.Repeat("0123456789abcdef", 4)

func TestSweepPurgeBackupsDeletesAtSevenDaysAndKeepsSix(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	put := func(name string, age time.Duration) {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, now.Add(-age), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
	}
	put(".flows-purge-backup-a.enc", 7*24*time.Hour+time.Minute)
	put(".flows-pre-restore-b.enc", 8*24*time.Hour)
	put(".flows-purge-backup-c.enc", 6*24*time.Hour+23*time.Hour)
	put("other.enc", 30*24*time.Hour)
	put(".flows-purge-backup-tmp-plain", 30*24*time.Hour) // a plaintext leftover is the scan's business
	deleted, err := SweepPurgeBackups(dir, now)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(deleted)
	if strings.Join(deleted, ",") != ".flows-pre-restore-b.enc,.flows-purge-backup-a.enc" {
		t.Fatalf("deleted = %v", deleted)
	}
	for _, kept := range []string{".flows-purge-backup-c.enc", "other.enc", ".flows-purge-backup-tmp-plain"} {
		if _, err := os.Stat(filepath.Join(dir, kept)); err != nil {
			t.Fatalf("%s was swept: %v", kept, err)
		}
	}
}

func TestRestorePurgeBackupBringsBackTheOldDatabaseAndKeepsTheCurrentOne(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "flows.db")
	s, err := OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	publishRevisions(t, s, "flow_cccccccccc", 1)
	plain := filepath.Join(dir, "plain.db")
	if _, err := s.db.Exec(`VACUUM INTO ?`, plain); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(dir, PurgeBackupPrefix+"test"+PurgeBackupSuffix)
	if err := security.EncryptFileWithMasterKey(plain, backup, PurgeBackupLabel, testPurgeKey); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(plain)
	// The database moves on after the backup.
	if _, err := s.CreateFlow(ctx, sampleFlow("flow_dddddddddd"), "", storeNow); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	if err := RestorePurgeBackup(path, backup, testPurgeKey, storeNow); err != nil {
		t.Fatalf("RestorePurgeBackup: %v", err)
	}
	restored, err := OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if _, err := restored.GetFlow(ctx, "flow_cccccccccc"); err != nil {
		t.Fatalf("the backed-up flow is missing: %v", err)
	}
	if _, err := restored.GetFlow(ctx, "flow_dddddddddd"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("the later flow survived the restore: %v", err)
	}
	pre, _ := filepath.Glob(filepath.Join(dir, PreRestoreBackupPrefix+"*"+PurgeBackupSuffix))
	if len(pre) != 1 {
		t.Fatalf("pre-restore backups = %v, want one", pre)
	}
	prePlain := filepath.Join(dir, "pre.db")
	if err := security.DecryptFileWithMasterKey(pre[0], prePlain, PurgeBackupLabel, testPurgeKey); err != nil {
		t.Fatal(err)
	}
	old, err := sql.Open("sqlite", prePlain)
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	var n int
	if err := old.QueryRow(`SELECT COUNT(*) FROM flows WHERE id='flow_dddddddddd'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("the pre-restore copy lost the later flow: %d, %v", n, err)
	}
}
```

Run: `GOTMPDIR='D:\tmp\go-build' go test ./internal/flows -run 'TestSweepPurgeBackups|TestRestorePurgeBackup' -count=1 2>&1 | head -5`. Expected: `undefined: SweepPurgeBackups` (and `PurgeBackupPrefix`, `RestorePurgeBackup`).

- [ ] **Step 6: Implement the sweeper, the restore and the store helpers**

Create `internal/flows/purge_files.go`:

```go
package flows

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/security"
)

const (
	// PurgeBackupPrefix names the encrypted safety backup the purge takes first, and
	// PreRestoreBackupPrefix the encrypted copy the restore flag takes of the current database.
	PurgeBackupPrefix      = ".flows-purge-backup-"
	PreRestoreBackupPrefix = ".flows-pre-restore-"
	PurgeBackupSuffix      = ".enc"
	// PurgeBackupMaxAge is how long an encrypted backup lives (owner decision 2).
	PurgeBackupMaxAge = 7 * 24 * time.Hour
	// PurgeBackupLabel is the HKDF label of both kinds of backup.
	PurgeBackupLabel  = "aurago/easydrag/purge-backup/v1"
	backupTimeLayout  = "20060102T150405.000Z"
	purgeTempPrefix   = PurgeBackupPrefix + "tmp-"
	preRestoreTempExt = ".pre-restore.tmp"
)

func isEncryptedBackupName(name string) bool {
	return strings.HasSuffix(name, PurgeBackupSuffix) &&
		(strings.HasPrefix(name, PurgeBackupPrefix) || strings.HasPrefix(name, PreRestoreBackupPrefix))
}

// SweepPurgeBackups deletes the encrypted purge and pre-restore backups in dir that are older than
// PurgeBackupMaxAge, by modification time. It returns the deleted names.
func SweepPurgeBackups(dir string, now time.Time) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var deleted []string
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !isEncryptedBackupName(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil || now.Sub(info.ModTime()) <= PurgeBackupMaxAge {
			continue
		}
		if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil {
			return deleted, err
		}
		deleted = append(deleted, entry.Name())
	}
	return deleted, nil
}

// dbPath is the file of the open database ("" for an in-memory one).
func (s *Store) dbPath() (string, error) {
	var path string
	if err := s.db.QueryRow(`SELECT file FROM pragma_database_list WHERE name='main'`).Scan(&path); err != nil {
		return "", err
	}
	return path, nil
}

// dir is the directory of flows.db, where the snapshots and the purge backups live ("" in memory).
func (s *Store) dir() (string, error) {
	path, err := s.dbPath()
	if err != nil || path == "" {
		return "", err
	}
	return filepath.Dir(path), nil
}

// RestorePurgeBackup replaces the flows database at dbPath by the decrypted purge backup. It is
// meant for the --restore-flows-purge-backup flag and must run while AuraGo is stopped. It first
// keeps the current database as an encrypted .flows-pre-restore file, verifies the decrypted backup
// opens and passes integrity_check, and removes the stale WAL and SHM before the swap.
func RestorePurgeBackup(dbPath, backupFile, masterKeyHex string, now time.Time) error {
	if info, err := os.Lstat(backupFile); err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("the backup file %q cannot be read", backupFile)
	}
	if _, err := os.Stat(dbPath); err == nil {
		plain := dbPath + preRestoreTempExt
		_ = os.Remove(plain)
		current, err := sql.Open("sqlite", dbPath)
		if err != nil {
			return fmt.Errorf("open the current database: %w", err)
		}
		_, err = current.Exec(`VACUUM INTO ?`, plain)
		_ = current.Close()
		if err != nil {
			return fmt.Errorf("copy the current database: %w", err)
		}
		defer os.Remove(plain)
		pre := filepath.Join(filepath.Dir(dbPath), PreRestoreBackupPrefix+now.UTC().Format(backupTimeLayout)+PurgeBackupSuffix)
		if err := security.EncryptFileWithMasterKey(plain, pre, PurgeBackupLabel, masterKeyHex); err != nil {
			return fmt.Errorf("keep the current database: %w", err)
		}
	}
	restored := dbPath + ".restore.tmp"
	defer func() {
		for _, suffix := range []string{"", "-wal", "-shm"} {
			_ = os.Remove(restored + suffix)
		}
	}()
	if err := security.DecryptFileWithMasterKey(backupFile, restored, PurgeBackupLabel, masterKeyHex); err != nil {
		return err
	}
	check, err := sql.Open("sqlite", restored)
	if err != nil {
		return err
	}
	var integrity, version string
	err = check.QueryRow(`PRAGMA integrity_check`).Scan(&integrity)
	if err == nil {
		err = check.QueryRow(`SELECT value FROM flows_meta WHERE key='schema_version'`).Scan(&version)
	}
	_ = check.Close()
	if err != nil || integrity != "ok" {
		return fmt.Errorf("the decrypted backup is not a usable flows database (%q, %v)", integrity, err)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(dbPath + suffix); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return os.Rename(restored, dbPath)
}
```

Edit `internal/flows/service.go`:
1. In `ServiceConfig` (after `RetentionInterval  time.Duration`) add:
```go
	// PurgeKey returns the master key (64 hex characters) the credential purge encrypts its safety
	// backup with; nil or an empty result makes the purge refuse to start.
	PurgeKey func() string
```
2. In the `Service` struct (after `draftLocks flowLocks`) add: `lastSweep time.Time // retentionLoop only`.
3. Replace `retentionLoop` with:
```go
func (s *Service) retentionLoop() {
	defer close(s.done)
	s.sweepPurgeBackups()
	ticker := time.NewTicker(s.cfg.RetentionInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			if _, err := s.PruneNow(context.Background()); err != nil {
				s.logger.Warn("flow run retention failed", "error", err)
			}
			s.sweepPurgeBackups()
		}
	}
}

// sweepPurgeBackups deletes expired encrypted backups at start and then once a day.
func (s *Service) sweepPurgeBackups() {
	if !s.lastSweep.IsZero() && s.now().Sub(s.lastSweep) < 24*time.Hour {
		return
	}
	s.lastSweep = s.now()
	dir, err := s.store.dir()
	if err != nil || dir == "" {
		return
	}
	deleted, err := SweepPurgeBackups(dir, s.now())
	if err != nil {
		s.logger.Warn("expired flows backups could not be deleted", "error", err)
	}
	if len(deleted) > 0 {
		s.logger.Info("expired encrypted flows backups deleted", "count", len(deleted))
	}
}
```
(Keep the existing body of the `case <-ticker.C` exactly; read the function first with `sed -n 355,370p internal/flows/service.go` and replace only what differs.)

Edit `internal/server/flows_service.go` `initFlows`: in the `flows.ServiceConfig{...}` literal add `PurgeKey: security.ProcessMasterKey,` and add the import `"aurago/internal/security"`.

- [ ] **Step 7: Write the failing restore-flag test and implement the flag**

Create `cmd/aurago/restore_flows_purge_backup_test.go`:

```go
package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofrs/flock"
)

func TestRestoreFlowsPurgeBackupRefusesWhileAuraGoRuns(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "aurago.lock")
	running := flock.New(lockPath)
	if locked, err := running.TryLock(); err != nil || !locked {
		t.Fatalf("test lock: %v %v", locked, err)
	}
	defer running.Unlock()
	var stdout, stderr bytes.Buffer
	code := runRestoreFlowsPurgeBackup(lockPath, filepath.Join(t.TempDir(), "config.yaml"), "backup.enc", &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "AuraGo is running") || stdout.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q, want a refusal naming the running instance", code, stdout.String(), stderr.String())
	}
}
```
Run `GOTMPDIR='D:\tmp\go-build' go test ./cmd/aurago -run TestRestoreFlowsPurgeBackup -count=1 2>&1 | head -4`; expected `undefined: runRestoreFlowsPurgeBackup`.

Create `cmd/aurago/restore_flows_purge_backup.go`:

```go
package main

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"

	"aurago/internal/config"
	"aurago/internal/flows"
)

// instanceLockPath is the lock file main takes at startup ("Robust File Locking"): the binary's directory.
func instanceLockPath(installDir string) string {
	path := "aurago.lock"
	if installDir != "" {
		path = filepath.Join(installDir, "aurago.lock")
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

// runRestoreFlowsPurgeBackup implements --restore-flows-purge-backup and returns the exit code. It takes the
// instance lock first, so it refuses while AuraGo runs on this installation; it keeps the current flows.db
// as an encrypted .flows-pre-restore file, puts the decrypted backup in its place and exits.
func runRestoreFlowsPurgeBackup(lockPath, configFile, backupFile string, stdout, stderr io.Writer) int {
	lock := flock.New(lockPath)
	if locked, err := lock.TryLock(); err != nil || !locked {
		fmt.Fprintln(stderr, "AuraGo is running on this installation. Stop it first, then run the restore again.")
		return 1
	}
	defer lock.Unlock()
	cfg, err := config.Load(configFile)
	if err != nil {
		fmt.Fprintf(stderr, "CONFIG ERROR: %v\n", err)
		return 1
	}
	dbPath := config.FlowsDBPath(cfg)
	if dbPath == "" {
		fmt.Fprintln(stderr, "The flows database path is not configured (sqlite.game_maker_path).")
		return 1
	}
	key := strings.TrimSpace(cfg.Server.MasterKey)
	if key == "" {
		fmt.Fprintln(stderr, "The master key is not available: set AURAGO_MASTER_KEY or AURAGO_MASTER_KEY_FILE.")
		return 1
	}
	if err := flows.RestorePurgeBackup(dbPath, backupFile, key, time.Now()); err != nil {
		fmt.Fprintf(stderr, "RESTORE ERROR: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "flows.db restored from %s. The database it replaced was kept as an encrypted .flows-pre-restore file and is deleted after 7 days.\n", filepath.Base(backupFile))
	return 0
}
```

Edit `cmd/aurago/main.go`: after the `assetsImport := flag.String(...)` line (about 206) add
```go
	restoreFlowsBackup := flag.String("restore-flows-purge-backup", "", "Restore flows.db from an encrypted purge backup file, then exit (AuraGo must be stopped)")
```
and immediately before `if healthcheck || printTsNetStateDirOnly {` add
```go
	if *restoreFlowsBackup != "" {
		os.Exit(runRestoreFlowsPurgeBackup(instanceLockPath(installDir), configFile, *restoreFlowsBackup, os.Stdout, os.Stderr))
	}
```
Add `"aurago/internal/flows"` only to the new file (main.go needs no new import).

- [ ] **Step 8: Run the tests**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && GOTMPDIR='D:\tmp\go-build' go test ./internal/security -run TestFileEncryption -count=1 2>&1 | tail -2 && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows -run 'TestSweepPurgeBackups|TestRestorePurgeBackup|TestC16|Retention' -count=1 2>&1 | tail -3 && GOTMPDIR='D:\tmp\go-build' go test ./cmd/aurago -run TestRestoreFlowsPurgeBackup -count=1 2>&1 | tail -3 && go build ./cmd/aurago ./internal/server 2>&1 | head
```
Expected: `ok` four times and no build output. (`go build ./cmd/aurago` writes no binary when several packages are named; if it complains, use `go vet ./cmd/aurago`.)

- [ ] **Step 9: Document the restore and commit**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && grep -n "^## \|^### " internal/flows/AGENTS.md | tail -4
```
Append a section at the end of `internal/flows/AGENTS.md` with the Edit tool (anchor on the file's last line, which `tail -3 internal/flows/AGENTS.md` shows):
```
## Credential purge backups

The credential purge (B.9) first writes `.flows-purge-backup-<UTC>.enc` next to flows.db: AES-256-GCM in 64 KiB chunks, key = HKDF-SHA256(master key, file salt, label `aurago/easydrag/purge-backup/v1`). `security.EncryptFileWithMasterKey` and `DecryptFileWithMasterKey` are the only writers and readers. `Service.sweepPurgeBackups` deletes `.flows-purge-backup-*.enc` and `.flows-pre-restore-*.enc` after 7 days (at start, then daily). To restore, stop AuraGo and run `aurago --restore-flows-purge-backup <file>`: it takes the instance lock (`aurago.lock`), keeps the current flows.db as `.flows-pre-restore-<UTC>.enc`, verifies the decrypted backup, removes the stale `-wal` and `-shm`, swaps the file in and exits. There is no UI for it.
```
Gates: `GOTMPDIR='D:\tmp\go-build' go vet ./internal/security ./internal/flows ./cmd/aurago && go vet ./internal/server && gofmt -l internal/security internal/flows cmd/aurago internal/server` (no output).

Write `C:/disposable/easydrag-s0b/msg-b7.txt`:
```
feat(security): master-key file encryption, backup sweeper and restore flag

EncryptFileWithMasterKey and DecryptFileWithMasterKey encrypt a file with
AES-256-GCM in 64 KiB chunks under an HKDF-SHA256 key from the master key,
a label and a random file salt; the last chunk is flagged, so truncation
fails. The flows service deletes encrypted purge and pre-restore backups
after 7 days (at start, then daily). aurago --restore-flows-purge-backup
takes the instance lock, keeps the current flows.db encrypted, and swaps
the verified backup in.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <your model line>
```
```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && git add internal/security/file_encrypt.go internal/security/file_encrypt_test.go internal/flows/purge_files.go internal/flows/purge_files_test.go internal/flows/service.go internal/flows/AGENTS.md internal/server/flows_service.go cmd/aurago/restore_flows_purge_backup.go cmd/aurago/restore_flows_purge_backup_test.go cmd/aurago/main.go && git commit -F C:/disposable/easydrag-s0b/msg-b7.txt
```

---
### Task B.8: The credential scan, its endpoint and the start-page hint

**Files:**
- Create: `internal/flows/purge_scan.go`, `internal/flows/purge_scan_test.go`, `internal/server/flows_credentials.go`, `ui/js/desktop/apps/easydrag-credentials-purge.js`, `scripts/test-easydrag-credentials-purge.mjs`, `C:/disposable/easydrag-s0b/keys-b8.json`
- Modify: `internal/flows/service.go:158-188` (`Service` struct), `:355-369` (`retentionLoop`), `internal/server/flows_handlers.go:45` (`flowsCollectionRoutes`), `:479` (dispatch), `:561-573` (`flowsRequiredScope`), `internal/server/flows_bridge.go:454` (`broadcastFlowsChanged`), `internal/server/flows_purge_test.go`, `ui/js/desktop/apps/easydrag-core.js:377` (`credentialScan`), `ui/js/desktop/core/module-loader.js:319`, `ui/desktop_easydrag_test.go:14-15` (`easyDragModules`), `ui/js/desktop/apps/easydrag-home.js:131` and `:487` (the hook), `ui/lang/easydrag/*.json` (16 files)
- Test: `internal/flows/purge_scan_test.go`, `internal/server/flows_purge_test.go`, `scripts/test-easydrag-credentials-purge.mjs`

- [ ] **Step 1: Write the failing scan tests**

Create `internal/flows/purge_scan_test.go` (it also holds the seed helpers that B.9 reuses). `TestTheRedactionMarkerIsNeverAFinding` pins a contract with B1: if it fails, B1's rule lacks the same skip that `literalCredentialHeaders` has for `redactedText` (`credentials.go:66`); add it where B1 skips `{{` templates (a value or a query value equal to `CredentialRedactionMarker` is no finding), say so in the commit message, and rerun, because without it the purge could never converge.

```go
package flows

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	fixtureHeaderToken = "Bearer HDRTOKEN-0123456789abcdef"
	fixtureQueryToken  = "QRYTOKEN-0123456789abcdef"
	fixtureFlowID      = "flow_eeeeeeeeee"
)

// credentialDoc is a trigger and an http.request; typed puts a query token and an Authorization header in it.
func credentialDoc(id, name string, typed bool) *Flow {
	b := newFlow(name)
	tr := b.node("start", "test.trigger", nil)
	params := map[string]any{"method": "GET", "url": "https://api.example.com/v1/items"}
	if typed {
		params["url"] = "https://api.example.com/v1/items?api_key=" + fixtureQueryToken
		params["headers"] = map[string]any{"Authorization": fixtureHeaderToken}
	}
	req := b.node("req", "http.request", params)
	b.edge(tr, PortOut, req)
	f := b.build()
	f.ID = id
	return f
}

func newPurgeService(t *testing.T) (*Service, *Store) {
	t.Helper()
	store := openTestStore(t)
	svc := NewService(store, catalogRegistry(t, fullEnv()), &Services{Tools: &fakeTools{}, Clock: newFakeClock(storeNow), Location: time.UTC},
		newFakeBridge(), ServiceConfig{PurgeKey: func() string { return testPurgeKey }}, discardLogger())
	t.Cleanup(func() { _ = svc.Shutdown(context.Background()) })
	return svc, store
}

// seedCredentialStore models an owner who typed a token, published twice, then moved it into the vault
// and published again: revisions 1 and 2 hold the token, revision 3 (live) and the draft are clean. A
// waiting run pins revision 2, and old rows from before the redaction existed hold the token in clear.
func seedCredentialStore(t *testing.T, s *Store) {
	t.Helper()
	ctx := context.Background()
	f := credentialDoc(fixtureFlowID, "Alt", true)
	if _, err := s.CreateFlow(ctx, f, "", storeNow); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Publish(ctx, fixtureFlowID, 1, storeNow); err != nil {
		t.Fatal(err)
	}
	f.Name = "Alt 2"
	rev, err := s.SaveDraft(ctx, fixtureFlowID, f, 1, storeNow)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Publish(ctx, fixtureFlowID, rev, storeNow); err != nil {
		t.Fatal(err)
	}
	if rev, err = s.SaveDraft(ctx, fixtureFlowID, credentialDoc(fixtureFlowID, "Neu", false), rev, storeNow); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Publish(ctx, fixtureFlowID, rev, storeNow); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"run_old", "run_wait"} {
		revision, status := 1, RunSuccess
		if id == "run_wait" {
			revision, status = 2, RunWaiting
		}
		if err := s.CreateRun(ctx, RunRecord{ID: id, FlowID: fixtureFlowID, Revision: revision, Mode: ModeLive, Status: status,
			StartedAt: storeNow, TriggerNode: "n_aaaaaaaa", TriggerType: "manual"}, nil); err != nil {
			t.Fatal(err)
		}
	}
	steps := `{"url":"https://api.example.com/v1/items?api_key=` + fixtureQueryToken + `","headers":{"Authorization":"` + fixtureHeaderToken + `"}}`
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{`UPDATE flow_runs SET trigger_data_json = ? WHERE id = 'run_old'`, []any{`{"api_key":"` + fixtureQueryToken + `"}`}},
		{`INSERT INTO flow_run_steps (run_id, seq, node_id, status, params_json, output_json) VALUES ('run_old', 1, 'n_bbbbbbbb', 'success', ?, ?)`,
			[]any{steps, `{"body":"echo ` + fixtureQueryToken + `"}`}},
		{`INSERT INTO flow_test_data (flow_id, node_id, kind, json, updated_at) VALUES (?, 'n_aaaaaaaa', 'trigger', ?, 'x')`,
			[]any{fixtureFlowID, `{"token":"` + fixtureQueryToken + `"}`}},
	} {
		if _, err := s.db.Exec(stmt.sql, stmt.args...); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTheRedactionMarkerIsNeverAFinding(t *testing.T) {
	got := literalCredentialsInValue(map[string]any{
		"token":   CredentialRedactionMarker,
		"headers": map[string]any{"Authorization": CredentialRedactionMarker},
		"url":     "https://api.example.com/v1?api_key=" + CredentialRedactionMarker,
	})
	if len(got) != 0 {
		t.Fatalf("the redaction marker counts as a credential: %v (the purge could never converge)", got)
	}
}

func TestCredentialScanReportsEveryPlaceAPlaintextCredentialSits(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	seedCredentialStore(t, s)
	// A second flow whose owner did not move the credentials: draft and live both hold them.
	other := credentialDoc("flow_ffffffffff", "Unberuehrt", true)
	if _, err := s.CreateFlow(ctx, other, "", storeNow); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Publish(ctx, other.ID, 1, storeNow); err != nil {
		t.Fatal(err)
	}
	dir, err := s.dir()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".flows-v7-backup-abc123", ".flows-v8-backup-def456"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name, "flows.db"), []byte("SQLite "+fixtureQueryToken), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	enc := filepath.Join(dir, PurgeBackupPrefix+"20260101T000000.000Z"+PurgeBackupSuffix)
	if err := os.WriteFile(enc, []byte("cipher"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, purgeTempPrefix+"1"), []byte("plain"), 0o600); err != nil {
		t.Fatal(err)
	}

	sc, err := s.scanCredentials(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := listPurgeFiles(dir, &sc.report); err != nil {
		t.Fatal(err)
	}
	rep := sc.report
	if len(rep.Drafts) != 1 || rep.Drafts[0].FlowID != "flow_ffffffffff" || rep.Drafts[0].Findings < 2 || len(rep.Live) != 1 {
		t.Fatalf("drafts = %+v, live = %+v, want only the unmoved flow", rep.Drafts, rep.Live)
	}
	if rep.VersionsPurgeable != 1 || rep.VersionsPinned != 1 {
		t.Fatalf("versions purgeable %d pinned %d, want 1 and 1 (revision 1 free, revision 2 pinned by the waiting run)", rep.VersionsPurgeable, rep.VersionsPinned)
	}
	if rep.Runs != 1 || rep.Steps != 1 || rep.TestRows != 1 {
		t.Fatalf("runs %d steps %d test rows %d, want 1 each", rep.Runs, rep.Steps, rep.TestRows)
	}
	if len(rep.Snapshots) != 2 || rep.Snapshots[0].Name != ".flows-v7-backup-abc123" || rep.Snapshots[0].Bytes == 0 {
		t.Fatalf("snapshots = %+v", rep.Snapshots)
	}
	if len(rep.Backups) != 1 || !rep.Backups[0].ExpiresAt.After(rep.Backups[0].ModTime) || len(rep.Leftovers) != 1 {
		t.Fatalf("backups = %+v leftovers = %+v", rep.Backups, rep.Leftovers)
	}
	if _, ok := sc.rows[fixtureQueryToken]; !ok {
		t.Fatal("the token found in the run rows is not in the literal set")
	}
	if blob, _ := jsonForTest(rep); strings.Contains(blob, "HDRTOKEN") || strings.Contains(blob, "QRYTOKEN") {
		t.Fatal("the report carries a credential value")
	}
}

func jsonForTest(v any) (string, error) {
	data, err := json.Marshal(v)
	return string(data), err
}

func TestCredentialScanIsCachedAndFreshOnRequest(t *testing.T) {
	ctx := context.Background()
	svc, store := newPurgeService(t)
	seedCredentialStore(t, store)
	first, err := svc.CredentialScan(ctx)
	if err != nil || first.Runs != 1 {
		t.Fatalf("first scan = %+v, %v", first, err)
	}
	if _, err := store.db.Exec(`DELETE FROM flow_runs WHERE id = 'run_old'`); err != nil {
		t.Fatal(err)
	}
	if cached, _ := svc.CredentialScan(ctx); cached.Runs != 1 {
		t.Fatalf("the cached report changed: %+v", cached)
	}
	if fresh, err := svc.CredentialScanFresh(ctx); err != nil || fresh.Runs != 0 {
		t.Fatalf("fresh scan = %+v, %v", fresh, err)
	}
	if again, _ := svc.CredentialScan(ctx); again.Runs != 0 {
		t.Fatalf("a fresh scan must refill the cache: %+v", again)
	}
}
```

Run: `GOTMPDIR='D:\tmp\go-build' go test ./internal/flows -run 'TestCredentialScan' -count=1 2>&1 | head -5`. Expected: build failure `undefined: listPurgeFiles` (and `scanCredentials`, `CredentialScan`).

- [ ] **Step 2: Implement the scan**

Create `internal/flows/purge_scan.go`:

```go
package flows

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	credentialScanTTL    = 10 * time.Minute
	credentialScanMinGap = 30 * time.Second
	// minPurgeLiteral is the shortest literal the purge replaces by exact text; shorter findings are
	// still redacted by the rule, but a short string would also hit harmless text.
	minPurgeLiteral = 8
)

// snapshotNamePattern matches the migration snapshot directories backupFlowsSnapshot creates.
var snapshotNamePattern = regexp.MustCompile(`^\.flows-v[0-9]+-backup-[A-Za-z0-9]+$`)

type CredentialFlowRef struct {
	FlowID   string `json:"flow_id"`
	Name     string `json:"name"`
	Findings int    `json:"findings"`
}

type SnapshotInfo struct {
	Name    string    `json:"name"`
	Bytes   int64     `json:"bytes"`
	ModTime time.Time `json:"mod_time"`
}

type BackupInfo struct {
	Name      string    `json:"name"`
	Bytes     int64     `json:"bytes"`
	ModTime   time.Time `json:"mod_time"`
	ExpiresAt time.Time `json:"expires_at"`
}

// CredentialReport says where typed credentials still sit. It never holds a credential value.
type CredentialReport struct {
	GeneratedAt       time.Time           `json:"generated_at"`
	Drafts            []CredentialFlowRef `json:"drafts"`
	Live              []CredentialFlowRef `json:"live"`
	VersionsPurgeable int                 `json:"versions_purgeable"`
	VersionsPinned    int                 `json:"versions_pinned"`
	Runs              int                 `json:"runs"`
	Steps             int                 `json:"steps"`
	TestRows          int                 `json:"test_rows"`
	Snapshots         []SnapshotInfo      `json:"snapshots"`
	Leftovers         []SnapshotInfo      `json:"leftovers"`
	Backups           []BackupInfo        `json:"backups"`
}

// NeedsAttention is true while plaintext remains anywhere the purge can reach. The encrypted backups
// do not count: they are the purge's own safety net.
func (r CredentialReport) NeedsAttention() bool {
	return len(r.Drafts) > 0 || len(r.Live) > 0 || r.VersionsPurgeable > 0 || r.VersionsPinned > 0 ||
		r.Runs > 0 || r.Steps > 0 || r.TestRows > 0 || len(r.Snapshots) > 0 || len(r.Leftovers) > 0
}

func (r *CredentialReport) normalize() {
	if r.Drafts == nil {
		r.Drafts = []CredentialFlowRef{}
	}
	if r.Live == nil {
		r.Live = []CredentialFlowRef{}
	}
	if r.Snapshots == nil {
		r.Snapshots = []SnapshotInfo{}
	}
	if r.Leftovers == nil {
		r.Leftovers = []SnapshotInfo{}
	}
	if r.Backups == nil {
		r.Backups = []BackupInfo{}
	}
}

// credentialScan is a scan with the data the purge needs: the literal values found (kept in memory only).
type credentialScan struct {
	report    CredentialReport
	draftLive map[string]struct{} // literals in drafts and live documents, which the purge leaves alone
	versions  map[string]struct{} // literals in published versions
	rows      map[string]struct{} // literals found by the rule in runs, steps and test data
	purge     map[string][]int    // flow id -> revisions with findings that are not live
}

func (sc *credentialScan) addDoc(text string, into map[string]struct{}) int {
	doc, err := ParseFlow([]byte(text))
	if err != nil {
		return 0
	}
	n := 0
	for _, finding := range FindLiteralCredentials(doc) {
		n++
		if finding.Value != "" {
			into[finding.Value] = struct{}{}
		}
	}
	return n
}

// allLiterals is the set the purge replaces by exact text.
func (sc *credentialScan) allLiterals() []string {
	return expandLiterals(sc.draftLive, sc.versions, sc.rows)
}

// credentialLiteralsInText decodes JSON text and returns the literals the rule finds in it.
func credentialLiteralsInText(text string) []string {
	if text == "" {
		return nil
	}
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil
	}
	return literalCredentialsInValue(v)
}

// expandLiterals merges literal sets into the list the purge replaces: every value of at least
// minPurgeLiteral characters, plus the value without a leading "Bearer " or "Basic " (the vault
// stores the bare value), longest first so a prefixed value is replaced before its bare part.
func expandLiterals(sets ...map[string]struct{}) []string {
	seen := map[string]struct{}{}
	add := func(v string) {
		if len(v) >= minPurgeLiteral {
			seen[v] = struct{}{}
		}
	}
	for _, set := range sets {
		for v := range set {
			add(v)
			for _, prefix := range []string{"Bearer ", "Basic "} {
				if len(v) > len(prefix) && strings.EqualFold(v[:len(prefix)], prefix) {
					add(strings.TrimSpace(v[len(prefix):]))
				}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i]) != len(out[j]) {
			return len(out[i]) > len(out[j])
		}
		return out[i] < out[j]
	})
	return out
}

func containsAny(text string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(text, n) {
			return true
		}
	}
	return false
}

// credentialTextSources are the plaintext JSON columns the purge reads and rewrites (flows.db keeps
// scenarios and durable checkpoints encrypted, so they are not here). rowid is the row key of every table.
var credentialTextSources = []struct {
	kind, table string
	columns     []string
}{
	{"runs", "flow_runs", []string{"doc_json", "trigger_data_json", "checkpoint_json"}},
	{"steps", "flow_run_steps", []string{"params_json", "output_json"}},
	{"steps", "flow_child_traces", []string{"summary_json", "trace_json"}},
	{"test", "flow_test_data", []string{"json"}},
	{"test", "flow_test_docs", []string{"json"}},
}

func (s *Store) scanTexts(ctx context.Context, table string, columns []string, fn func(texts []string)) error {
	parts := make([]string, len(columns))
	for i, c := range columns {
		parts[i] = "COALESCE(" + c + ", '')"
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+strings.Join(parts, ", ")+` FROM `+table)
	if err != nil {
		return err
	}
	defer rows.Close()
	texts := make([]string, len(columns))
	ptrs := make([]any, len(columns))
	for i := range texts {
		ptrs[i] = &texts[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		fn(texts)
	}
	return rows.Err()
}

// scanRowCounts counts the rows that hold a finding of the rule or one of literals, per kind. The
// literals found by the rule are added to sc.rows.
func (s *Store) scanRowCounts(ctx context.Context, sc *credentialScan, literals []string) (map[string]int, error) {
	counts := map[string]int{}
	for _, src := range credentialTextSources {
		src := src
		err := s.scanTexts(ctx, src.table, src.columns, func(texts []string) {
			hit := false
			for _, text := range texts {
				if found := credentialLiteralsInText(text); len(found) > 0 {
					hit = true
					for _, v := range found {
						sc.rows[v] = struct{}{}
					}
				}
				if !hit && containsAny(text, literals) {
					hit = true
				}
			}
			if hit {
				counts[src.kind]++
			}
		})
		if err != nil {
			return nil, err
		}
	}
	return counts, nil
}

// scanCredentials reads every JSON column of flows.db: drafts, live documents, versions, runs, steps and test data.
func (s *Store) scanCredentials(ctx context.Context) (*credentialScan, error) {
	sc := &credentialScan{draftLive: map[string]struct{}{}, versions: map[string]struct{}{}, rows: map[string]struct{}{}, purge: map[string][]int{}}
	rep := &sc.report
	flowRows, err := s.db.QueryContext(ctx, `SELECT id, name, draft_json, live_json FROM flows ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	for flowRows.Next() {
		var id, name, draft, live string
		if err := flowRows.Scan(&id, &name, &draft, &live); err != nil {
			_ = flowRows.Close()
			return nil, err
		}
		if n := sc.addDoc(draft, sc.draftLive); n > 0 {
			rep.Drafts = append(rep.Drafts, CredentialFlowRef{FlowID: id, Name: name, Findings: n})
		}
		if live != "" {
			if n := sc.addDoc(live, sc.draftLive); n > 0 {
				rep.Live = append(rep.Live, CredentialFlowRef{FlowID: id, Name: name, Findings: n})
			}
		}
	}
	if err := flowRows.Err(); err != nil {
		_ = flowRows.Close()
		return nil, err
	}
	_ = flowRows.Close()

	versionRows, err := s.db.QueryContext(ctx, `SELECT v.flow_id, v.revision, v.json, f.live_revision, `+
		versionPinnedSQL("v.flow_id", "v.revision")+` FROM flow_versions v JOIN flows f ON f.id = v.flow_id ORDER BY v.flow_id, v.revision`)
	if err != nil {
		return nil, err
	}
	for versionRows.Next() {
		var flowID, text string
		var revision, live int
		var pinned bool
		if err := versionRows.Scan(&flowID, &revision, &text, &live, &pinned); err != nil {
			_ = versionRows.Close()
			return nil, err
		}
		if sc.addDoc(text, sc.versions) == 0 || revision == live { // the live version is the live document
			continue
		}
		if pinned {
			rep.VersionsPinned++
		} else {
			rep.VersionsPurgeable++
		}
		sc.purge[flowID] = append(sc.purge[flowID], revision)
	}
	if err := versionRows.Err(); err != nil {
		_ = versionRows.Close()
		return nil, err
	}
	_ = versionRows.Close()

	// Pass 1 finds the literals by the rule; pass 2 counts the rows again, now also the ones that only
	// repeat a literal (a moved token echoed in an output text).
	if _, err := s.scanRowCounts(ctx, sc, nil); err != nil {
		return nil, err
	}
	counts, err := s.scanRowCounts(ctx, sc, sc.allLiterals())
	if err != nil {
		return nil, err
	}
	rep.Runs, rep.Steps, rep.TestRows = counts["runs"], counts["steps"], counts["test"]
	return sc, nil
}

func dirBytes(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if info, ierr := d.Info(); ierr == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

// isPlaintextLeftover names the temporary files a failed purge or restore may leave in clear.
func isPlaintextLeftover(name string) bool {
	if strings.HasSuffix(name, preRestoreTempExt) || strings.HasSuffix(name, ".restore.tmp") {
		return true
	}
	return (strings.HasPrefix(name, PurgeBackupPrefix) || strings.HasPrefix(name, PreRestoreBackupPrefix)) && !strings.HasSuffix(name, PurgeBackupSuffix)
}

// listPurgeFiles fills the snapshot, leftover and encrypted-backup lists from the directory of flows.db.
func listPurgeFiles(dir string, rep *CredentialReport) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		info, err := entry.Info()
		if err != nil {
			continue
		}
		switch {
		case entry.IsDir() && snapshotNamePattern.MatchString(name):
			rep.Snapshots = append(rep.Snapshots, SnapshotInfo{Name: name, Bytes: dirBytes(filepath.Join(dir, name)), ModTime: info.ModTime()})
		case entry.Type().IsRegular() && isEncryptedBackupName(name):
			rep.Backups = append(rep.Backups, BackupInfo{Name: name, Bytes: info.Size(), ModTime: info.ModTime(), ExpiresAt: info.ModTime().Add(PurgeBackupMaxAge)})
		case entry.Type().IsRegular() && isPlaintextLeftover(name):
			rep.Leftovers = append(rep.Leftovers, SnapshotInfo{Name: name, Bytes: info.Size(), ModTime: info.ModTime()})
		}
	}
	return nil
}

// scanCache holds the last report. CredentialScan serves it and refreshes it in the background (at most
// every 30 seconds after a change, every 10 minutes otherwise); the dialog asks for a fresh one.
type scanCache struct {
	mu      sync.Mutex
	report  *CredentialReport
	at      time.Time
	dirty   bool
	running bool
}

// stopContext is a context that ends when the service shuts down.
func (s *Service) stopContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-s.stop:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

// InvalidateCredentialScan marks the cached report as out of date (a save, a move, a publish, a purge).
func (s *Service) InvalidateCredentialScan() {
	s.scan.mu.Lock()
	s.scan.dirty = true
	s.scan.mu.Unlock()
}

// CredentialScan returns the cached report. The first call computes it; later calls return the cache at
// once and start a background refresh when it is out of date.
func (s *Service) CredentialScan(ctx context.Context) (CredentialReport, error) {
	c := &s.scan
	c.mu.Lock()
	report, age := c.report, s.now().Sub(c.at)
	stale := report != nil && !c.running && ((c.dirty && age >= credentialScanMinGap) || age >= credentialScanTTL)
	if stale {
		c.running = true
	}
	c.mu.Unlock()
	if report == nil {
		return s.CredentialScanFresh(ctx)
	}
	if stale {
		go func() {
			bg, cancel := s.stopContext()
			defer cancel()
			_, _ = s.computeCredentialScan(bg)
		}()
	}
	return *report, nil
}

// CredentialScanFresh reads flows.db again and refills the cache.
func (s *Service) CredentialScanFresh(ctx context.Context) (CredentialReport, error) {
	return s.computeCredentialScan(ctx)
}

func (s *Service) computeCredentialScan(ctx context.Context) (CredentialReport, error) {
	c := &s.scan
	c.mu.Lock()
	c.dirty = false
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.running = false
		c.mu.Unlock()
	}()
	sc, err := s.store.scanCredentials(ctx)
	if err != nil {
		return CredentialReport{}, err
	}
	report := sc.report
	report.GeneratedAt = s.now()
	if dir, derr := s.store.dir(); derr == nil && dir != "" {
		if err := listPurgeFiles(dir, &report); err != nil {
			return CredentialReport{}, err
		}
	}
	report.normalize()
	c.mu.Lock()
	c.report, c.at = &report, s.now()
	c.mu.Unlock()
	return report, nil
}
```

Append to `internal/flows/purge_scan.go`:
```go
// initialCredentialScan fills the cache at start; it ends early when the service shuts down.
func (s *Service) initialCredentialScan() {
	ctx, cancel := s.stopContext()
	defer cancel()
	if _, err := s.computeCredentialScan(ctx); err != nil && ctx.Err() == nil {
		s.logger.Warn("the credential scan failed", "error", err)
	}
}
```
Edit `internal/flows/service.go`: add `scan scanCache` to the `Service` struct (after `lastSweep`), and in `retentionLoop`, right after `s.sweepPurgeBackups()` at the top, add `s.initialCredentialScan()`.

- [ ] **Step 3: Run the scan tests**

`cd <worktree> && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows -run 'TestCredentialScan|TestTheRedactionMarker' -count=1 -v 2>&1 | tail -10`. Expected: three `PASS`. If `rep.Drafts[0].Findings < 2` fails, `FindLiteralCredentials` finds fewer than the header plus the query token in the fixture node: B1's rule is incomplete for `http.request` params; stop and tell B1 (do not weaken the test).

- [ ] **Step 4: Write the failing server tests**

Append to `internal/server/flows_purge_test.go` (add `"strings"` and `"aurago/internal/flows"` to its imports):

```go
const tokenFlowJSON = `{"schema":1,"name":"Tokens","nodes":[
 {"id":"n_aaaaaaaa","key":"start","type":"trigger.manual","type_version":1,"label":"Start","position":{"x":0,"y":0},"params":{}},
 {"id":"n_bbbbbbbb","key":"req","type":"http.request","type_version":1,"label":"Anfrage","position":{"x":300,"y":0},
  "params":{"method":"GET","url":"https://api.example.com/v1?api_key=QRYTOKEN-0123456789abcdef","headers":{"Authorization":"Bearer HDRTOKEN-0123456789abcdef"}}}],
 "edges":[{"id":"e_aaaaaaaa","source":{"node":"n_aaaaaaaa","port":"out"},"target":{"node":"n_bbbbbbbb","port":"in"}}]}`

func TestCredentialScanEndpointNeedsAdminAndNeverReturnsAValue(t *testing.T) {
	s, token := newFlowsTestServer(t)
	doc, err := flows.ParseFlow([]byte(tokenFlowJSON))
	if err != nil {
		t.Fatal(err)
	}
	doc.ID = "flow_tokenflow1"
	if _, err := s.Flows.Store().CreateFlow(context.Background(), doc, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	readToken, _, err := s.TokenManager.Create("reader", []string{desktopScopeRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if w := flowsCall(t, s, http.MethodGet, "/api/desktop/flows/credentials/scan", readToken, ""); w.Code != http.StatusForbidden {
		t.Fatalf("a read token got %d %s, want 403", w.Code, w.Body.String())
	}
	w := flowsCall(t, s, http.MethodGet, "/api/desktop/flows/credentials/scan?fresh=1", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("scan = %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "QRYTOKEN") || strings.Contains(w.Body.String(), "HDRTOKEN") {
		t.Fatal("the scan answer carries a credential value")
	}
	report := flowsBody(t, w)["report"].(map[string]any)
	if drafts, _ := report["drafts"].([]any); len(drafts) != 1 {
		t.Fatalf("drafts = %v, want the one flow", report["drafts"])
	}
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/credentials/scan", token, "{}"); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST scan = %d, want 405", w.Code)
	}
}
```
Run `go test ./internal/server -run TestCredentialScanEndpoint -count=1 2>&1 | tail -6`. Expected failure: 404 or 403 mismatch (`a read token got 404`), because the route does not exist.

- [ ] **Step 5: Implement the endpoint**

Create `internal/server/flows_credentials.go`:

```go
package server

import (
	"net/http"
)

// flowsCredentials serves /api/desktop/flows/credentials/...: the scan and (B.9) the purge. Every
// method needs the desktop admin scope (flowsRequiredScope). No answer carries a credential value.
func (s *Server) flowsCredentials(w http.ResponseWriter, r *http.Request, rest []string) {
	switch {
	case len(rest) == 1 && rest[0] == "scan":
		if r.Method != http.MethodGet {
			flowsMethodNotAllowed(w)
			return
		}
		scan := s.Flows.CredentialScan
		if r.URL.Query().Get("fresh") == "1" {
			scan = s.Flows.CredentialScanFresh
		}
		report, err := scan(r.Context())
		if err != nil {
			s.flowsErrorFrom(w, r, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		flowsJSON(w, http.StatusOK, map[string]any{"report": report})
	default:
		flowsError(w, http.StatusNotFound, "FLOW_NOT_FOUND", "unknown flow route")
	}
}
```

Edit `internal/server/flows_handlers.go`:
1. `flowsCollectionRoutes` (line 45): add `"credentials": true`.
2. In the `switch` of `handleFlows`, after `case parts[0] == "secrets":` and its call, add
```go
		case parts[0] == "credentials":
			s.flowsCredentials(w, r, parts[1:])
```
3. In `flowsRequiredScope`, add as the first case
```go
	case len(parts) >= 1 && parts[0] == "credentials":
		return desktopScopeAdmin
```
Edit `internal/server/flows_bridge.go` `broadcastFlowsChanged`: first line of the body `if s.Flows != nil { s.Flows.InvalidateCredentialScan() }`.

Check the B.3 move handler broadcasts: `grep -n "broadcastFlowsChanged" internal/server/flows_credentials_move*.go internal/server/flows_handlers.go | head`. If the move handler (B.3) does not call `s.broadcastFlowsChanged(id, "saved")` after it rewrote the draft, add `s.Flows.InvalidateCredentialScan()` after its audit line.

Run: `go test ./internal/server -run 'TestCredentialScanEndpoint|TestFlowsRollbackOfAMarked' -count=1 2>&1 | tail -4`. Expected: `ok`.

- [ ] **Step 6: Write the failing Node test and the client module**

Create `scripts/test-easydrag-credentials-purge.mjs`:

```js
// Pure checks of the credential purge hint: what needs attention and what the card says.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

function load() {
    const here = path.dirname(fileURLToPath(import.meta.url));
    const source = path.join(here, '..', 'ui', 'js', 'desktop', 'apps', 'easydrag-credentials-purge.js');
    const esc = value => String(value).replace(/[&<>"']/g, c => '&#' + c.charCodeAt(0) + ';');
    const tr = (t, key, fallback) => { const value = t(key); return value && value !== key ? value : fallback; };
    const sandbox = vm.createContext({ window: { EasyDrag: { core: { esc, tr } } } });
    vm.runInContext(fs.readFileSync(source, 'utf8'), sandbox, { filename: source });
    return sandbox.window.EasyDrag.credentialsPurge;
}

export async function run() {
    const purge = load();
    const empty = { drafts: [], live: [], versions_purgeable: 0, versions_pinned: 0, runs: 0, steps: 0, test_rows: 0, snapshots: [], leftovers: [], backups: [{ name: 'x.enc' }] };
    assert.equal(purge.needsAttention(empty), false, 'encrypted backups alone need no attention');
    assert.equal(purge.needsAttention(null), false);
    assert.equal(purge.needsAttention('x'), false);
    for (const change of [{ drafts: [{ flow_id: 'f' }] }, { live: [{ flow_id: 'f' }] }, { versions_purgeable: 1 }, { versions_pinned: 2 },
        { runs: 1 }, { steps: 3 }, { test_rows: 1 }, { snapshots: [{ name: 's' }] }, { leftovers: [{ name: 'l' }] }]) {
        assert.equal(purge.needsAttention(Object.assign({}, empty, change)), true, JSON.stringify(change));
    }
    const t = key => ({ 'easydrag.ui.cred_hint_title': 'Title <b>', 'easydrag.ui.cred_hint_text': 'Text', 'easydrag.ui.cred_hint_action': 'Clean' }[key] || key);
    assert.equal(purge.hintMarkup(t, empty), '', 'a clean report shows no card');
    const markup = purge.hintMarkup(t, Object.assign({}, empty, { runs: 2 }));
    assert.ok(markup.includes('Title &#60;b&#62;') && !markup.includes('<b>'), 'the title is escaped');
    assert.ok(markup.includes('data-ed-credentials-open') && markup.includes('Clean') && markup.includes('Text'));
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) { await run(); console.log('credentials purge hint checks passed'); }
```
Run `node scripts/test-easydrag-credentials-purge.mjs 2>&1 | head -4`. Expected: failure `ENOENT ... easydrag-credentials-purge.js`.

Create `ui/js/desktop/apps/easydrag-credentials-purge.js`:

```js
// EasyDrag credential purge: the start-page hint (and, from the purge task on, the guided dialog).
// The scan answers only counts and names, never a credential value.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    let bound = null;            // the app whose api and t() the module uses; set by mountHint
    const hints = new Set();     // the mounted hint handles

    // needsAttention: plaintext remains anywhere the purge can reach (encrypted backups do not count).
    function needsAttention(report) {
        if (!report || typeof report !== 'object') return false;
        const list = value => Array.isArray(value) && value.length > 0;
        const count = value => Number.isFinite(value) && value > 0;
        return list(report.drafts) || list(report.live) || count(report.versions_purgeable) || count(report.versions_pinned) ||
            count(report.runs) || count(report.steps) || count(report.test_rows) || list(report.snapshots) || list(report.leftovers);
    }

    function hintMarkup(t, report) {
        if (!needsAttention(report)) return '';
        const esc = ED.core.esc;
        return '<h3>' + esc(t('easydrag.ui.cred_hint_title')) + '</h3><p>' + esc(t('easydrag.ui.cred_hint_text')) + '</p>' +
            '<button type="button" class="ed-btn" data-ed-credentials-open>' + esc(t('easydrag.ui.cred_hint_action')) + '</button>';
    }

    // mountHint puts the hint card into the start page. It asks the scan endpoint, which is admin only:
    // a 401, 403 or any failure shows nothing.
    function mountHint(container, app) {
        if (app) bound = app;
        const card = ED.core.el('<div class="ed-home-stale" role="status" data-ed-credentials-hint hidden></div>');
        const anchor = container.querySelector('.ed-home-toolbar');
        if (anchor) container.insertBefore(card, anchor); else container.appendChild(card);
        let disposed = false, sequence = 0;
        async function refresh() {
            const mine = ++sequence;
            let report = null;
            try { report = (await bound.api.credentialScan(false)).report; } catch (err) { report = null; }
            if (disposed || mine !== sequence) return;
            const markup = hintMarkup(bound.t, report);
            card.innerHTML = markup;
            card.hidden = !markup;
        }
        card.addEventListener('click', event => {
            if (event.target.closest('[data-ed-credentials-open]') && typeof ED.credentialsPurge.open === 'function') void ED.credentialsPurge.open();
        });
        const handle = { refresh, dispose() { disposed = true; hints.delete(handle); card.remove(); } };
        hints.add(handle);
        void refresh();
        return handle;
    }

    // offer is what the editor calls after a move and a publish: the hint refreshes at once.
    function offer(flowId) {
        hints.forEach(handle => { void handle.refresh(); });
        return flowId;
    }

    ED.credentialsPurge = { needsAttention, hintMarkup, mountHint, offer };
})();
```

Edit `ui/js/desktop/apps/easydrag-core.js` after the `secrets:` line (`:377`): add
```js
            credentialScan: (fresh, signal) => api(base + '/credentials/scan' + query({ fresh: fresh ? '1' : '' }), { signal, cache: 'no-store' }),
```
(the line before it, `secrets: () => api(base + '/secrets'),`, keeps its comma; place the new line right after `deleteSecret`'s line by adding a comma there. Read lines 376-381 first and keep the comma pattern.)

Edit `ui/js/desktop/core/module-loader.js` after the `'/js/desktop/apps/easydrag-pack.js',` entry add `'/js/desktop/apps/easydrag-credentials-purge.js',`. Edit `ui/desktop_easydrag_test.go` `easyDragModules`: insert `"easydrag-credentials-purge.js",` after `"easydrag-pack.js",`.

Edit `ui/js/desktop/apps/easydrag-home.js`: after line 131 (`if (runtime) { el.insertBefore(...) }`) add
```js
        const credentialHint = ED.credentialsPurge && ED.credentialsPurge.mountHint(el, app);
        if (credentialHint) bag.add(() => credentialHint.dispose());
```
and in `reload()`, after the line `focusLeftFlow();` that ends the success path (the one directly after `reportLoaded();`), add `if (credentialHint) void credentialHint.refresh();`. Check the size: `wc -l ui/js/desktop/apps/easydrag-home.js` must print 889 (the ratchet warns above 900).

- [ ] **Step 7: Add the locale keys**

Write `C:/disposable/easydrag-s0b/keys-b8.json`:

```json
{
  "easydrag.ui.cred_hint_title": {
    "cs": "Nalezeny ručně zadané přístupové údaje", "da": "Indtastede adgangsoplysninger fundet", "de": "Getippte Zugangsdaten gefunden",
    "el": "Βρέθηκαν πληκτρολογημένα διαπιστευτήρια", "en": "Typed credentials found", "es": "Credenciales escritas encontradas",
    "fr": "Identifiants saisis détectés", "hi": "टाइप किए गए क्रेडेंशियल मिले", "it": "Credenziali digitate trovate",
    "ja": "入力された認証情報が見つかりました", "nl": "Ingetypte inloggegevens gevonden", "no": "Innskrevet legitimasjon funnet",
    "pl": "Znaleziono wpisane dane uwierzytelniające", "pt": "Credenciais digitadas encontradas", "sv": "Inskrivna inloggningsuppgifter hittades",
    "zh": "发现手动输入的凭据"
  },
  "easydrag.ui.cred_hint_text": {
    "cs": "Staré kopie ručně zadaných tokenů mohou stále ležet ve tvých flow, během a zálohách. Vyčisti je v několika krocích.",
    "da": "Gamle kopier af indtastede tokens kan stadig ligge i dine flows, kørsler og sikkerhedskopier. Ryd op i få trin.",
    "de": "Alte Kopien getippter Tokens liegen vielleicht noch in deinen Flows, Läufen und Sicherungen. Räume sie in wenigen Schritten auf.",
    "el": "Παλιά αντίγραφα πληκτρολογημένων tokens μπορεί να βρίσκονται ακόμη στα flows, στις εκτελέσεις και στα αντίγραφα ασφαλείας σου. Καθάρισέ τα με λίγα βήματα.",
    "en": "Old copies of typed tokens may still sit in your flows, runs and backups. Clean them up in a few steps.",
    "es": "Puede que aún queden copias antiguas de tokens escritos en tus flujos, ejecuciones y copias de seguridad. Límpialas en unos pasos.",
    "fr": "D'anciennes copies de jetons saisis peuvent encore se trouver dans tes flows, exécutions et sauvegardes. Nettoie-les en quelques étapes.",
    "hi": "टाइप किए गए टोकन की पुरानी प्रतियाँ अब भी आपके फ़्लो, रन और बैकअप में हो सकती हैं। कुछ चरणों में उन्हें साफ़ करें।",
    "it": "Vecchie copie dei token digitati potrebbero essere ancora nei tuoi flow, nelle esecuzioni e nei backup. Puliscile in pochi passaggi.",
    "ja": "入力したトークンの古いコピーが、フロー・実行履歴・バックアップに残っている可能性があります。数ステップで消去できます。",
    "nl": "Oude kopieën van ingetypte tokens staan mogelijk nog in je flows, runs en back-ups. Ruim ze in een paar stappen op.",
    "no": "Gamle kopier av innskrevne tokens kan fortsatt ligge i flowene, kjøringene og sikkerhetskopiene dine. Rydd opp i noen få trinn.",
    "pl": "Stare kopie wpisanych tokenów mogą nadal znajdować się w twoich flow, uruchomieniach i kopiach zapasowych. Wyczyść je w kilku krokach.",
    "pt": "Cópias antigas de tokens digitados ainda podem estar nos teus fluxos, execuções e cópias de segurança. Limpa-as em poucos passos.",
    "sv": "Gamla kopior av inskrivna tokens kan fortfarande finnas i dina flöden, körningar och säkerhetskopior. Rensa dem i några steg.",
    "zh": "手动输入的令牌的旧副本可能仍保存在你的流程、运行记录和备份中。几步即可清理。"
  },
  "easydrag.ui.cred_hint_action": {
    "cs": "Zkontrolovat a vyčistit", "da": "Gennemgå og ryd op", "de": "Prüfen und bereinigen", "el": "Έλεγχος και καθαρισμός",
    "en": "Review and clean up", "es": "Revisar y limpiar", "fr": "Vérifier et nettoyer", "hi": "देखें और साफ़ करें",
    "it": "Controlla e ripulisci", "ja": "確認してクリーンアップ", "nl": "Bekijken en opruimen", "no": "Se gjennom og rydd opp",
    "pl": "Przejrzyj i wyczyść", "pt": "Rever e limpar", "sv": "Granska och rensa", "zh": "查看并清理"
  }
}
```
`cd <worktree> && node C:/disposable/easydrag-s0b/add-locale-keys.mjs . C:/disposable/easydrag-s0b/keys-b8.json`. Expected: `added up to 3 keys to 16 languages`.

- [ ] **Step 8: Run all gates, then commit**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && node scripts/test-easydrag-credentials-purge.mjs && node scripts/test-easydrag.mjs | tail -1 && node scripts/test-easydrag-render-baseline.mjs | tail -1 && go test ./ui -run 'EasyDrag|Locale|NoExternal|ShippedWebAssets' -count=1 2>&1 | tail -3 && wc -l ui/js/desktop/apps/easydrag-home.js && GOTMPDIR='D:\tmp\go-build' go vet ./internal/flows && go vet ./internal/server && gofmt -l internal/flows internal/server
```
Expected: `credentials purge hint checks passed`, `all EasyDrag checks passed`, `PASS render baseline (unchanged)`, `ok`, `889 ...`, nothing from vet or gofmt.

Write `C:/disposable/easydrag-s0b/msg-b8.txt`:
```
feat(flows): credential scan, its admin endpoint and the start-page hint

Service.CredentialScan reads every JSON column of flows.db (drafts, live
documents, versions, runs, steps, test data), the migration snapshot
directories, plaintext purge leftovers and the encrypted backups, and
answers counts and names only. The report is cached (10 minutes, 30
seconds after a change) and refreshed at start. GET
/api/desktop/flows/credentials/scan needs desktop:admin. The start page
shows a "Typed credentials found" card from the new module
easydrag-credentials-purge.js while plaintext remains; it stays hidden
when the scan answers 401 or 403.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <your model line>
```
```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && git add internal/flows/purge_scan.go internal/flows/purge_scan_test.go internal/flows/service.go internal/server/flows_credentials.go internal/server/flows_handlers.go internal/server/flows_bridge.go internal/server/flows_purge_test.go ui/js/desktop/apps/easydrag-credentials-purge.js ui/js/desktop/apps/easydrag-core.js ui/js/desktop/apps/easydrag-home.js ui/js/desktop/core/module-loader.js ui/desktop_easydrag_test.go scripts/test-easydrag-credentials-purge.mjs ui/lang/easydrag && git commit -F C:/disposable/easydrag-s0b/msg-b8.txt
```

---
### Task B.9: The purge job, the run hold and the dialog

**Files:**
- Create: `internal/flows/runner_hold.go`, `internal/flows/runner_hold_test.go`, `internal/flows/purge_job.go`, `internal/flows/purge_steps.go`, `internal/flows/purge_job_test.go`, `ui/testdata/easydrag-credentials-fixture.js`, `ui/desktop_easydrag_credentials_browser_test.go`, `C:/disposable/easydrag-s0b/keys-b9.json`
- Modify: `internal/flows/runner.go` (struct near `:184`, `:630`, `:647`, `:484`), `internal/server/flows_credentials.go`, `internal/server/flows_handlers.go:229-262` (`flowsErrorFrom`), `internal/server/flows_purge_test.go`, `ui/js/desktop/apps/easydrag-core.js` (two api entries), `ui/js/desktop/apps/easydrag-credentials-purge.js`, `scripts/test-easydrag-credentials-purge.mjs`, `ui/lang/easydrag/*.json` (16 files)
- Test: the `_test.go` files above, `scripts/test-easydrag-credentials-purge.mjs`

- [ ] **Step 1: Impact analysis and the facts this task needs**

```bash
for s in dispatchLocked admitLocked StartContext; do node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "$s" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo; done
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && grep -rn "launchLocked(" internal/flows --include=*.go | grep -v _test.go; grep -rn "WITHOUT ROWID" internal/flows --include=*.go | head -3; grep -n "^func FreeDiskBytes" internal/fileutil/free_space.go; grep -rn "aurago/internal/flows" internal/fileutil | head -1
```
`dispatchLocked` is the core of every run start (HIGH): report it. The change is one extra condition that is false except during a purge. Expected greps: `launchLocked(` is called only from `dispatchLocked` (`runner.go:661`), so a flag there holds every start path; no `WITHOUT ROWID` (the purge keys its updates by `rowid`; if a table shows up, key that table by its primary key instead); `FreeDiskBytes(path string) (int64, error)` exists; fileutil does not import flows. If `launchLocked` has another caller, hold that path too.

- [ ] **Step 2: Write the failing run-hold test**

Create `internal/flows/runner_hold_test.go`:

```go
package flows

import (
	"errors"
	"testing"
	"time"
)

func TestHoldDispatchRefusesWhileARunRunsAndQueuesNewRunsUntilReleased(t *testing.T) {
	fx := newRunnerFixture(t, RunnerConfig{})
	a := fx.flow(t, "flow_holdaaaaaa", ConcurrencyQueue)
	b := fx.flow(t, "flow_holdbbbbbb", ConcurrencyQueue)
	fx.start(t, a, ModeLive, "a")
	fx.gate.waitStarted(t, "a")
	if _, err := fx.r.HoldDispatch(); !errors.Is(err, ErrRunsActive) {
		t.Fatalf("HoldDispatch while a run runs = %v, want ErrRunsActive", err)
	}
	fx.gate.release("a")
	fx.waitFinished(t)
	var release func()
	for deadline := time.Now().Add(2 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		var err error
		if release, err = fx.r.HoldDispatch(); err == nil {
			break
		}
		if !errors.Is(err, ErrRunsActive) || time.Now().After(deadline) {
			t.Fatalf("HoldDispatch after the run ended = %v", err)
		}
	}
	if _, err := fx.r.HoldDispatch(); !errors.Is(err, ErrDispatchHeld) {
		t.Fatalf("a second hold = %v, want ErrDispatchHeld", err)
	}
	res := fx.start(t, b, ModeLive, "b")
	if res.Status != StartQueued {
		t.Fatalf("a run started during the hold has status %s, want queued", res.Status)
	}
	fx.gate.assertNotStarted(t)
	release()
	release() // idempotent
	fx.gate.waitStarted(t, "b")
	fx.gate.release("b")
	if rec := fx.waitFinished(t); rec.ID != res.RunID || rec.Status != RunSuccess {
		t.Fatalf("held run finished = %+v", rec)
	}
}
```
Run `GOTMPDIR='D:\tmp\go-build' go test ./internal/flows -run TestHoldDispatch -count=1 2>&1 | head -4`. Expected: `undefined: ErrRunsActive` (and `HoldDispatch`, `ErrDispatchHeld`).

- [ ] **Step 3: Implement the hold**

Create `internal/flows/runner_hold.go`:

```go
package flows

import (
	"errors"
	"sync"
)

var (
	// ErrRunsActive is returned by HoldDispatch while a run holds a slot, that is, while it is running (a
	// parked run that only waits gave its slot back).
	ErrRunsActive = errors.New("a flow run is running")
	// ErrDispatchHeld is returned when the dispatch is already on hold.
	ErrDispatchHeld = errors.New("flow runs are already on hold")
)

// HoldDispatch stops the runner from starting any run, new or queued or resuming, until release is
// called. It refuses while a run is running. The check and the hold happen under Runner.mu, so no run
// can start in between. Runs started during the hold are recorded as queued and start on release.
func (r *Runner) HoldDispatch() (release func(), err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch {
	case r.closed:
		return nil, ErrRunnerClosed
	case r.dispatchHeld:
		return nil, ErrDispatchHeld
	case r.slots > 0:
		return nil, ErrRunsActive
	}
	r.dispatchHeld = true
	var once sync.Once
	return func() {
		once.Do(func() {
			r.mu.Lock()
			r.dispatchHeld = false
			r.dispatchLocked()
			r.mu.Unlock()
		})
	}, nil
}
```
Edit `internal/flows/runner.go`:
1. In the `Runner` struct, after the line `nextTicket           uint64        // arrival order of waiting runs and resumers`, add `dispatchHeld         bool          // HoldDispatch: nothing starts`.
2. `dispatchLocked`: `for !r.closed && r.slots < r.cfg.MaxParallelRuns {` becomes `for !r.closed && !r.dispatchHeld && r.slots < r.cfg.MaxParallelRuns {`.
3. `admitLocked`: `if !(r.slots < r.cfg.MaxParallelRuns && r.fitsLocked(p.rec.FlowID, p.counted) && r.nextWaitingLocked() < 0) {` becomes `if r.dispatchHeld || !(r.slots < r.cfg.MaxParallelRuns && r.fitsLocked(p.rec.FlowID, p.counted) && r.nextWaitingLocked() < 0) {` (a run that waited for the hold asks the gate when it executes).
4. In `StartContext`: `slotless := r.slots >= r.cfg.MaxParallelRuns || !r.fitsLocked(flowID, counted)` becomes `slotless := r.dispatchHeld || r.slots >= r.cfg.MaxParallelRuns || !r.fitsLocked(flowID, counted)`.

Run: `GOTMPDIR='D:\tmp\go-build' go test ./internal/flows -run 'TestHoldDispatch|TestRunner' -count=1 2>&1 | tail -3`. Expected: `ok`.

- [ ] **Step 4: Write the failing purge-job tests**

Create `internal/flows/purge_job_test.go`:

```go
package flows

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/security"
)

func waitPurge(t *testing.T, svc *Service) PurgeState {
	t.Helper()
	for deadline := time.Now().Add(20 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		if st := svc.CredentialPurgeState(); st.Status != "running" {
			return st
		}
		if time.Now().After(deadline) {
			t.Fatalf("the purge did not finish: %+v", svc.CredentialPurgeState())
		}
	}
}

// tokenRows lists "table.column" once for every row whose column holds token.
func tokenRows(t *testing.T, s *Store, token string) []string {
	t.Helper()
	var hits []string
	for _, c := range [][2]string{{"flow_runs", "doc_json"}, {"flow_runs", "trigger_data_json"}, {"flow_run_steps", "params_json"},
		{"flow_run_steps", "output_json"}, {"flow_test_data", "json"}, {"flow_versions", "json"}, {"flows", "draft_json"}, {"flows", "live_json"}} {
		var n int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM `+c[0]+` WHERE instr(`+c[1]+`, ?) > 0`, token).Scan(&n); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < n; i++ {
			hits = append(hits, c[0]+"."+c[1])
		}
	}
	return hits
}

func TestPurgeJobBlacksOutDeletesAndVerifies(t *testing.T) {
	ctx := context.Background()
	svc, store := newPurgeService(t)
	seedCredentialStore(t, store)
	dir, _ := store.dir()
	const snap = ".flows-v7-backup-abc123"
	if err := os.MkdirAll(filepath.Join(dir, snap), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, snap, "flows.db"), []byte(fixtureQueryToken), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartCredentialPurge([]string{snap}); err != nil {
		t.Fatal(err)
	}
	st := waitPurge(t, svc)
	if st.Status != "done" || !st.Clean || len(st.Remaining) != 0 {
		t.Fatalf("state = %+v", st)
	}
	if st.VersionsDeleted != 1 || st.VersionsPinned != 1 || st.RowsRedacted < 3 {
		t.Fatalf("deleted %d pinned %d redacted %d, want 1, 1 and at least 3", st.VersionsDeleted, st.VersionsPinned, st.RowsRedacted)
	}
	if got := fmt.Sprint(versionRevisions(t, store, fixtureFlowID)); got != "[3 2]" {
		t.Fatalf("versions = %s, want [3 2]: revision 1 deleted, revision 2 pinned by the waiting run", got)
	}
	if _, err := os.Stat(filepath.Join(dir, snap)); !os.IsNotExist(err) {
		t.Fatalf("the snapshot is still there: %v", err)
	}
	for _, token := range []string{fixtureQueryToken, strings.TrimPrefix(fixtureHeaderToken, "Bearer ")} {
		if hits := tokenRows(t, store, token); fmt.Sprint(hits) != "[flow_versions.json]" {
			t.Fatalf("%s is still in %v, want only the pinned version", token, hits)
		}
	}
	var params string
	if err := store.db.QueryRow(`SELECT params_json FROM flow_run_steps WHERE run_id = 'run_old'`).Scan(&params); err != nil ||
		!strings.Contains(params, CredentialRedactionMarker) || !strings.Contains(params, "api.example.com") {
		t.Fatalf("step params = %q, %v: the token should be the marker and the host should stay", params, err)
	}

	// The encrypted backup holds no plaintext and restores to a database that still has the old versions.
	enc := filepath.Join(dir, st.Backup)
	data, err := os.ReadFile(enc)
	if err != nil || bytes.Contains(data, []byte(fixtureQueryToken)) {
		t.Fatalf("backup unreadable or holds plaintext: %v", err)
	}
	plain := filepath.Join(t.TempDir(), "restored.db")
	if err := security.DecryptFileWithMasterKey(enc, plain, PurgeBackupLabel, testPurgeKey); err != nil {
		t.Fatal(err)
	}
	restored, err := sql.Open("sqlite", plain)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var n int
	if err := restored.QueryRow(`SELECT COUNT(*) FROM flow_versions`).Scan(&n); err != nil || n != 3 {
		t.Fatalf("the backup holds %d versions, %v, want 3", n, err)
	}

	// The waiting run ends: revision 2 goes at once and no byte of the token is left in the files.
	if err := store.FinishRun(ctx, "run_wait", RunResult{Status: RunSuccess, FinishedAt: storeNow}); err != nil {
		t.Fatal(err)
	}
	if hits := tokenRows(t, store, fixtureQueryToken); len(hits) != 0 {
		t.Fatalf("the token is still in %v after the pinned run ended", hits)
	}
	for _, name := range []string{"flows.db", "flows.db-wal"} {
		if b, err := os.ReadFile(filepath.Join(dir, name)); err == nil && bytes.Contains(b, []byte(fixtureQueryToken)) {
			t.Fatalf("%s still holds the token in bytes", name)
		}
	}
}

func TestPurgeRefusesWhileARunIsRunningAndHoldsNewRunsWhileItWorks(t *testing.T) {
	fx := newRunnerFixture(t, RunnerConfig{})
	svc, _ := newPurgeService(t)
	svc.runner, svc.store = fx.r, fx.store // the purge works on the fixture's runner and store
	seedCredentialStore(t, fx.store)
	a := fx.flow(t, "flow_holdaaaaaa", ConcurrencyQueue)
	b := fx.flow(t, "flow_holdbbbbbb", ConcurrencyQueue)
	fx.start(t, a, ModeLive, "a")
	fx.gate.waitStarted(t, "a")
	if _, err := svc.StartCredentialPurge(nil); !errors.Is(err, ErrPurgeRunsActive) {
		t.Fatalf("purge while a run runs = %v, want ErrPurgeRunsActive", err)
	}
	fx.gate.release("a")
	fx.waitFinished(t)

	block := make(chan struct{})
	svc.purge.stepHook = func(step PurgeStep) {
		if step == PurgeStepBackup {
			<-block
		}
	}
	for deadline := time.Now().Add(2 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		_, err := svc.StartCredentialPurge(nil)
		if err == nil {
			break
		}
		if !errors.Is(err, ErrPurgeRunsActive) || time.Now().After(deadline) {
			t.Fatalf("StartCredentialPurge = %v", err)
		}
	}
	if _, err := svc.StartCredentialPurge(nil); !errors.Is(err, ErrPurgeBusy) {
		t.Fatalf("a second purge = %v, want ErrPurgeBusy", err)
	}
	if res := fx.start(t, b, ModeLive, "b"); res.Status != StartQueued {
		t.Fatalf("a run started during the purge has status %s, want queued", res.Status)
	}
	fx.gate.assertNotStarted(t)
	close(block)
	if st := waitPurge(t, svc); st.Status != "done" {
		t.Fatalf("state = %+v", st)
	}
	fx.gate.waitStarted(t, "b") // the hold ended with the job
	fx.gate.release("b")
	fx.waitFinished(t)
}

func TestPurgeStopsBeforeDestroyingAnythingWithoutAWorkingKeyAndIsIdempotent(t *testing.T) {
	svc, store := newPurgeService(t)
	seedCredentialStore(t, store)
	key := "not hex"
	svc.cfg.PurgeKey = func() string { return key }
	if _, err := svc.StartCredentialPurge(nil); err != nil {
		t.Fatal(err)
	}
	if st := waitPurge(t, svc); st.Status != "failed" || st.FailedStep != PurgeStepBackup || st.Error == "" {
		t.Fatalf("state with a key that cannot encrypt = %+v, want failed at backup", st)
	}
	if hits := tokenRows(t, store, fixtureQueryToken); len(hits) == 0 {
		t.Fatal("a failed backup must not have changed anything")
	}
	key = ""
	if _, err := svc.StartCredentialPurge(nil); !errors.Is(err, ErrPurgeNoKey) {
		t.Fatalf("no key = %v, want ErrPurgeNoKey", err)
	}
	key = testPurgeKey
	run := func() PurgeState {
		t.Helper()
		if _, err := svc.StartCredentialPurge(nil); err != nil {
			t.Fatal(err)
		}
		return waitPurge(t, svc)
	}
	if first := run(); first.Status != "done" || first.RowsRedacted == 0 {
		t.Fatalf("first run = %+v", first)
	}
	if second := run(); second.Status != "done" || second.VersionsDeleted != 0 || second.RowsRedacted != 0 || !second.Clean {
		t.Fatalf("a second run changed something: %+v", second)
	}
}
```
Run `GOTMPDIR='D:\tmp\go-build' go test ./internal/flows -run 'TestPurge' -count=1 2>&1 | head -5`. Expected: `undefined: PurgeState` (and `StartCredentialPurge`, `ErrPurgeRunsActive`, `svc.purge`).

- [ ] **Step 5: Implement the steps**

Create `internal/flows/purge_steps.go`:

```go
package flows

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"aurago/internal/security"
)

// writePurgeBackup copies flows.db with VACUUM INTO into a temporary file in dir, encrypts it into
// .flows-purge-backup-<UTC>.enc and removes the plaintext copy. It returns the file name.
func (s *Store) writePurgeBackup(ctx context.Context, dir, key string, now time.Time) (string, error) {
	suffix := make([]byte, 6)
	if _, err := rand.Read(suffix); err != nil {
		return "", err
	}
	tmp := filepath.Join(dir, purgeTempPrefix+hex.EncodeToString(suffix))
	defer os.Remove(tmp)
	if _, err := s.db.ExecContext(ctx, `VACUUM INTO ?`, tmp); err != nil {
		return "", fmt.Errorf("copy flows.db: %w", err)
	}
	_ = os.Chmod(tmp, 0o600)
	name := PurgeBackupPrefix + now.UTC().Format(backupTimeLayout) + PurgeBackupSuffix
	if err := security.EncryptFileWithMasterKey(tmp, filepath.Join(dir, name), PurgeBackupLabel, key); err != nil {
		return "", fmt.Errorf("encrypt the backup: %w", err)
	}
	return name, nil
}

func replaceText(text string, literals []string) string {
	for _, lit := range literals {
		text = strings.ReplaceAll(text, lit, CredentialRedactionMarker)
	}
	return text
}

// replaceLiterals replaces every exact occurrence of a literal inside the strings (and keys) of a decoded value.
func replaceLiterals(v any, literals []string) any {
	switch x := v.(type) {
	case string:
		return replaceText(x, literals)
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = replaceLiterals(x[i], literals)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, item := range x {
			out[replaceText(k, literals)] = replaceLiterals(item, literals)
		}
		return out
	}
	return v
}

// redactStoredText blacks out what the rule finds in JSON text and every exact occurrence of a literal.
// Text that is not JSON (a truncated output) only gets the exact replacement. It reports whether the
// text changed; running it on its own result changes nothing.
func redactStoredText(text string, literals []string) (string, bool) {
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	var v any
	valid := dec.Decode(&v) == nil
	if valid {
		_, err := dec.Token()
		valid = errors.Is(err, io.EOF)
	}
	if !valid {
		out := replaceText(text, literals)
		return out, out != text
	}
	redacted := replaceLiterals(redactCredentialValue(v), literals)
	if reflect.DeepEqual(v, redacted) {
		return text, false
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(redacted); err != nil {
		return text, false
	}
	return strings.TrimRight(buf.String(), "\n"), true
}

// redactStoredRows rewrites the plaintext JSON columns in batches and returns the number of rewritten cells.
func (s *Store) redactStoredRows(ctx context.Context, conn *sql.Conn, literals []string) (int, error) {
	total := 0
	for _, src := range credentialTextSources {
		for _, column := range src.columns {
			n, err := redactColumn(ctx, conn, src.table, column, literals)
			total += n
			if err != nil {
				return total, fmt.Errorf("%s.%s: %w", src.table, column, err)
			}
		}
	}
	return total, nil
}

func redactColumn(ctx context.Context, conn *sql.Conn, table, column string, literals []string) (int, error) {
	const batchSize = 200
	changed, last := 0, int64(0)
	type update struct {
		id   int64
		text string
	}
	for {
		rows, err := conn.QueryContext(ctx, `SELECT rowid, `+column+` FROM `+table+` WHERE rowid > ? AND `+column+` <> '' ORDER BY rowid LIMIT `+fmt.Sprint(batchSize), last)
		if err != nil {
			return changed, err
		}
		var batch []update
		n := 0
		for rows.Next() {
			var id int64
			var text string
			if err := rows.Scan(&id, &text); err != nil {
				_ = rows.Close()
				return changed, err
			}
			n++
			last = id
			if out, ok := redactStoredText(text, literals); ok {
				batch = append(batch, update{id, out})
			}
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return changed, err
		}
		if len(batch) > 0 {
			tx, err := conn.BeginTx(ctx, nil)
			if err != nil {
				return changed, err
			}
			for _, u := range batch {
				if _, err := tx.ExecContext(ctx, `UPDATE `+table+` SET `+column+` = ? WHERE rowid = ?`, u.text, u.id); err != nil {
					_ = tx.Rollback()
					return changed, err
				}
			}
			if err := tx.Commit(); err != nil {
				return changed, err
			}
			changed += len(batch)
		}
		if n < batchSize {
			return changed, nil
		}
	}
}

// compactDatabase runs VACUUM and truncates the WAL, on a connection that has secure_delete on.
func compactDatabase(ctx context.Context, conn *sql.Conn) error {
	if _, err := conn.ExecContext(ctx, `VACUUM`); err != nil {
		return fmt.Errorf("vacuum: %w", err)
	}
	for attempt := 0; attempt < 10; attempt++ {
		var busy, logFrames, checkpointed int
		if err := conn.QueryRowContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &logFrames, &checkpointed); err != nil {
			return fmt.Errorf("checkpoint: %w", err)
		}
		if busy == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}
	return errors.New("the WAL could not be truncated because the database stayed busy")
}

// deleteSnapshots removes the chosen snapshot directories (names are validated by validateSnapshotNames).
func deleteSnapshots(dir string, names []string) ([]string, error) {
	deleted := []string{}
	for _, name := range names {
		if !snapshotNamePattern.MatchString(name) {
			return deleted, ErrPurgeSnapshot
		}
		path := filepath.Join(dir, name)
		if info, err := os.Lstat(path); err != nil || !info.IsDir() {
			continue // already gone: running again is safe
		}
		if err := os.RemoveAll(path); err != nil {
			return deleted, err
		}
		deleted = append(deleted, name)
	}
	return deleted, nil
}

// fileHasAny reports whether the file contains one of the needles (a missing file does not).
func fileHasAny(path string, needles [][]byte) (bool, error) {
	if len(needles) == 0 {
		return false, nil
	}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	maxLen := 0
	for _, n := range needles {
		if len(n) > maxLen {
			maxLen = len(n)
		}
	}
	buf := make([]byte, 1<<20+maxLen)
	carry := 0
	for {
		n, err := f.Read(buf[carry:])
		window := buf[:carry+n]
		for _, needle := range needles {
			if bytes.Contains(window, needle) {
				return true, nil
			}
		}
		if errors.Is(err, io.EOF) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		keep := maxLen - 1
		if keep > len(window) {
			keep = len(window)
		}
		copy(buf, window[len(window)-keep:])
		carry = keep
	}
}

// verifyPurge scans again and searches the files byte for byte. A literal that a kept document still
// needs (a draft or live document that was not moved, a version a run still pins) is not searched for
// in the files; it is still counted in the run, step and test-data rows, which must be clean.
func (s *Store) verifyPurge(ctx context.Context, dbPath, dir string, literals []string) ([]PurgeRemaining, error) {
	post, err := s.scanCredentials(ctx)
	if err != nil {
		return nil, err
	}
	remaining := []PurgeRemaining{}
	for _, item := range []PurgeRemaining{{"runs", post.report.Runs}, {"steps", post.report.Steps}, {"test_data", post.report.TestRows}} {
		if item.Count > 0 {
			remaining = append(remaining, item)
		}
	}
	var needles [][]byte
	for _, lit := range literals {
		_, kept := post.draftLive[lit]
		if _, pinned := post.versions[lit]; !kept && !pinned {
			needles = append(needles, []byte(lit))
		}
	}
	files := []string{dbPath, dbPath + "-wal", dbPath + "-shm"}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.Type().IsRegular() && strings.HasPrefix(name, ".flows-") && !strings.HasSuffix(name, PurgeBackupSuffix) {
			files = append(files, filepath.Join(dir, name))
		}
	}
	inside, _ := filepath.Glob(filepath.Join(dir, ".flows-v*-backup-*", "*"))
	files = append(files, inside...)
	for _, file := range files {
		hit, err := fileHasAny(file, needles)
		if err != nil {
			return nil, err
		}
		if hit {
			rel, relErr := filepath.Rel(dir, file)
			if relErr != nil {
				rel = filepath.Base(file)
			}
			remaining = append(remaining, PurgeRemaining{Where: filepath.ToSlash(rel), Count: 1})
		}
	}
	return remaining, nil
}
```
`PurgeRemaining{"runs", n}` uses positional fields: declare `PurgeRemaining` with fields in the order `Where string; Count int` (Step 6).

- [ ] **Step 6: Implement the job**

Create `internal/flows/purge_job.go`:

```go
package flows

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aurago/internal/fileutil"
)

type PurgeStep string

const (
	PurgeStepBackup    PurgeStep = "backup"
	PurgeStepVersions  PurgeStep = "versions"
	PurgeStepRedact    PurgeStep = "redact"
	PurgeStepCompact   PurgeStep = "compact"
	PurgeStepSnapshots PurgeStep = "snapshots"
	PurgeStepVerify    PurgeStep = "verify"
)

var (
	ErrPurgeBusy        = errors.New("a credential cleanup is already running")
	ErrPurgeRunsActive  = errors.New("a flow run is running; try again when it has finished")
	ErrPurgeNoKey       = errors.New("the master key is not available, so the encrypted safety backup cannot be written")
	ErrPurgeNoSpace     = errors.New("there is not enough free disk space for the cleanup")
	ErrPurgeUnavailable = errors.New("the flows database is not stored in a file")
	ErrPurgeSnapshot    = errors.New("a chosen snapshot does not exist")
)

type PurgeRemaining struct {
	Where string `json:"where"`
	Count int    `json:"count"`
}

// PurgeState is what GET /credentials/purge answers. Status is idle, running, done or failed. It never
// holds a credential value.
type PurgeState struct {
	ID               string           `json:"id"`
	Status           string           `json:"status"`
	Step             PurgeStep        `json:"step,omitempty"`
	FailedStep       PurgeStep        `json:"failed_step,omitempty"`
	Error            string           `json:"error,omitempty"`
	StartedAt        time.Time        `json:"started_at"`
	FinishedAt       time.Time        `json:"finished_at"`
	Backup           string           `json:"backup,omitempty"`
	VersionsDeleted  int              `json:"versions_deleted"`
	VersionsPinned   int              `json:"versions_pinned"`
	RowsRedacted     int              `json:"rows_redacted"`
	SnapshotsDeleted []string         `json:"snapshots_deleted"`
	Clean            bool             `json:"clean"`
	Remaining        []PurgeRemaining `json:"remaining"`
}

type purgeState struct {
	mu    sync.Mutex
	state PurgeState
	// stepHook, set only by tests, runs when the job enters a step.
	stepHook func(PurgeStep)
}

func (s *Service) CredentialPurgeState() PurgeState {
	p := &s.purge
	p.mu.Lock()
	defer p.mu.Unlock()
	st := p.state
	if st.Status == "" {
		st.Status = "idle"
	}
	st.SnapshotsDeleted = append([]string{}, st.SnapshotsDeleted...)
	st.Remaining = append([]PurgeRemaining{}, st.Remaining...)
	return st
}

func (s *Service) updatePurge(change func(*PurgeState)) {
	s.purge.mu.Lock()
	change(&s.purge.state)
	s.purge.mu.Unlock()
}

// validateSnapshotNames accepts only names of existing migration snapshot directories.
func validateSnapshotNames(dir string, names []string) ([]string, error) {
	seen := map[string]bool{}
	out := []string{}
	for _, name := range names {
		if !snapshotNamePattern.MatchString(name) {
			return nil, ErrPurgeSnapshot
		}
		if info, err := os.Lstat(filepath.Join(dir, name)); err != nil || !info.IsDir() {
			return nil, ErrPurgeSnapshot
		}
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out, nil
}

// StartCredentialPurge checks everything that can be checked first, holds the runner and starts the job.
// It refuses while a run is running (ErrPurgeRunsActive), without a master key, without free disk space
// and while another job runs. New runs wait in the queue until the job ends; waiting runs do not block it.
func (s *Service) StartCredentialPurge(snapshots []string) (PurgeState, error) {
	key := ""
	if s.cfg.PurgeKey != nil {
		key = strings.TrimSpace(s.cfg.PurgeKey())
	}
	if key == "" {
		return PurgeState{}, ErrPurgeNoKey
	}
	dbPath, err := s.store.dbPath()
	if err != nil {
		return PurgeState{}, err
	}
	if dbPath == "" {
		return PurgeState{}, ErrPurgeUnavailable
	}
	dir := filepath.Dir(dbPath)
	names, err := validateSnapshotNames(dir, snapshots)
	if err != nil {
		return PurgeState{}, err
	}
	var size int64
	for _, suffix := range []string{"", "-wal"} {
		if info, err := os.Stat(dbPath + suffix); err == nil {
			size += info.Size()
		}
	}
	if free, err := fileutil.FreeDiskBytes(dir); err == nil && free < 2*size+16<<20 {
		return PurgeState{}, ErrPurgeNoSpace
	}
	p := &s.purge
	p.mu.Lock()
	if p.state.Status == "running" {
		p.mu.Unlock()
		return PurgeState{}, ErrPurgeBusy
	}
	release, err := s.runner.HoldDispatch()
	if err != nil {
		p.mu.Unlock()
		if errors.Is(err, ErrRunsActive) {
			return PurgeState{}, ErrPurgeRunsActive
		}
		if errors.Is(err, ErrDispatchHeld) {
			return PurgeState{}, ErrPurgeBusy
		}
		return PurgeState{}, err
	}
	id := make([]byte, 6)
	_, _ = rand.Read(id)
	p.state = PurgeState{ID: hex.EncodeToString(id), Status: "running", Step: PurgeStepBackup, StartedAt: s.now(),
		SnapshotsDeleted: []string{}, Remaining: []PurgeRemaining{}}
	state := p.state
	p.mu.Unlock()
	go s.runPurge(dbPath, dir, key, names, release)
	return state, nil
}

func (s *Service) runPurge(dbPath, dir, key string, snapshots []string, release func()) {
	defer release()
	ctx, cancel := s.stopContext()
	defer cancel()
	step := PurgeStepBackup
	fail := func(err error) {
		s.logger.Warn("credential cleanup stopped", "step", string(step), "error", err)
		s.updatePurge(func(st *PurgeState) {
			st.Status, st.FailedStep, st.Error, st.FinishedAt = "failed", step, err.Error(), s.now()
		})
	}
	defer func() {
		if r := recover(); r != nil {
			fail(fmt.Errorf("internal error: %v", r))
		}
	}()
	enter := func(next PurgeStep) {
		step = next
		s.updatePurge(func(st *PurgeState) { st.Step = next })
		if hook := s.purge.stepHook; hook != nil {
			hook(next)
		}
	}

	enter(PurgeStepBackup)
	backup, err := s.store.writePurgeBackup(ctx, dir, key, s.now())
	if err != nil {
		fail(err)
		return
	}
	s.updatePurge(func(st *PurgeState) { st.Backup = backup })

	enter(PurgeStepVersions)
	scan, err := s.store.scanCredentials(ctx) // before anything is deleted: the moved tokens are still in the old versions
	if err != nil {
		fail(err)
		return
	}
	literals := scan.allLiterals()
	for flowID, revisions := range scan.purge {
		if err := s.store.MarkVersionsForPurge(ctx, flowID, revisions, s.now()); err != nil {
			fail(err)
			return
		}
	}
	deleted, err := s.store.PrunePurgedVersions(ctx, "")
	if err != nil {
		fail(err)
		return
	}
	var pinned int
	if err := s.store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM flow_version_purge`).Scan(&pinned); err != nil {
		fail(err)
		return
	}
	s.updatePurge(func(st *PurgeState) { st.VersionsDeleted, st.VersionsPinned = int(deleted), pinned })

	enter(PurgeStepRedact)
	conn, err := s.store.db.Conn(ctx)
	if err != nil {
		fail(err)
		return
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `PRAGMA secure_delete = ON`); err != nil {
		fail(err)
		return
	}
	defer func() { _, _ = conn.ExecContext(context.WithoutCancel(ctx), `PRAGMA secure_delete = OFF`) }()
	redacted, err := s.store.redactStoredRows(ctx, conn, literals)
	if err != nil {
		fail(err)
		return
	}
	s.updatePurge(func(st *PurgeState) { st.RowsRedacted = redacted })

	enter(PurgeStepCompact)
	if err := compactDatabase(ctx, conn); err != nil {
		fail(err)
		return
	}

	enter(PurgeStepSnapshots)
	gone, err := deleteSnapshots(dir, snapshots)
	if err != nil {
		fail(err)
		return
	}
	s.updatePurge(func(st *PurgeState) { st.SnapshotsDeleted = gone })

	enter(PurgeStepVerify)
	remaining, err := s.store.verifyPurge(ctx, dbPath, dir, literals)
	if err != nil {
		fail(err)
		return
	}
	if _, err := s.computeCredentialScan(ctx); err != nil {
		s.logger.Warn("the credential scan after the cleanup failed", "error", err)
	}
	s.updatePurge(func(st *PurgeState) {
		st.Status, st.Step, st.Remaining, st.Clean, st.FinishedAt = "done", "", remaining, len(remaining) == 0, s.now()
	})
	s.logger.Info("credential cleanup finished", slog.Int("versions_deleted", int(deleted)), slog.Int("rows_redacted", redacted), slog.Int("remaining", len(remaining)))
}
```
Edit `internal/flows/service.go`: add `purge purgeState` to the `Service` struct (after `scan scanCache`).

Run: `GOTMPDIR='D:\tmp\go-build' go test ./internal/flows -run 'TestPurge|TestHoldDispatch|TestCredentialScan|TestMarked' -count=1 -v 2>&1 | grep -E "^(--- |ok|FAIL|panic)" | head`. Expected: every test `PASS`, then `ok`. Likely first failures and the fix:
- `redactCredentialValue` leaves the header value of `Authorization` alone: B1's rule must cover headers in any params map; stop and tell B1 if so.
- A byte of the token still in `flows.db-wal` right after the purge: `compactDatabase` returned before the checkpoint finished; the attempt loop above must reach `busy == 0`; check that no `rows` of this test are open (a leaked `*sql.Rows` keeps a read transaction).
- `database table is locked` from `VACUUM`: another connection holds a statement open; close every `rows` before the compact step.

- [ ] **Step 7: Server routes and the error codes**

Edit `internal/server/flows_credentials.go`: replace the `default:` case with the purge case and then the default:
```go
	case len(rest) == 1 && rest[0] == "purge":
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Cache-Control", "no-store")
			flowsJSON(w, http.StatusOK, map[string]any{"state": s.Flows.CredentialPurgeState()})
		case http.MethodPost:
			var body struct {
				Snapshots []string `json:"snapshots"`
			}
			if !flowsDecode(w, r, &body, flowsSmallBodyLimit, false) {
				return
			}
			state, err := s.Flows.StartCredentialPurge(body.Snapshots)
			if err != nil {
				s.flowsErrorFrom(w, r, err)
				return
			}
			s.recordFlowAudit("flow_credentials_purge", "", "", "Credential cleanup started")
			flowsJSON(w, http.StatusAccepted, map[string]any{"state": state})
		default:
			flowsMethodNotAllowed(w)
		}
	default:
```
Add the member entry `"easydrag.ui.error_flow_purge_": {"busy", "runs_active", "no_key", "no_space", "snapshot_invalid", "unavailable"},` next to the `error_flow_pack_` entry in `ui/desktop_easydrag_test.go` (and add that file to this commit's `git add`).

Edit `flowsErrorFrom` (`internal/server/flows_handlers.go`), after the `ErrVersionPurged` case from B.6:
```go
	case errors.Is(err, flows.ErrPurgeBusy):
		flowsError(w, http.StatusConflict, "FLOW_PURGE_BUSY", "a credential cleanup is already running")
	case errors.Is(err, flows.ErrPurgeRunsActive):
		flowsError(w, http.StatusConflict, "FLOW_PURGE_RUNS_ACTIVE", "a flow run is running; try again when it has finished")
	case errors.Is(err, flows.ErrPurgeNoKey):
		flowsError(w, http.StatusUnprocessableEntity, "FLOW_PURGE_NO_KEY", flowsErrorText(err))
	case errors.Is(err, flows.ErrPurgeNoSpace):
		flowsError(w, http.StatusUnprocessableEntity, "FLOW_PURGE_NO_SPACE", flowsErrorText(err))
	case errors.Is(err, flows.ErrPurgeUnavailable):
		flowsError(w, http.StatusUnprocessableEntity, "FLOW_PURGE_UNAVAILABLE", flowsErrorText(err))
	case errors.Is(err, flows.ErrPurgeSnapshot):
		flowsError(w, http.StatusBadRequest, "FLOW_PURGE_SNAPSHOT_INVALID", "a chosen snapshot does not exist")
```
Append to `internal/server/flows_purge_test.go` (imports `"aurago/internal/security"` and `"strings"` if missing):
```go
func TestCredentialPurgeEndpointWithAndWithoutAMasterKey(t *testing.T) {
	s, token := newFlowsTestServer(t)
	doc, err := flows.ParseFlow([]byte(tokenFlowJSON))
	if err != nil {
		t.Fatal(err)
	}
	doc.ID = "flow_tokenflow2"
	if _, err := s.Flows.Store().CreateFlow(context.Background(), doc, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	security.RetainProcessMasterKey("")
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/credentials/purge", token, `{"snapshots":[]}`); w.Code != http.StatusUnprocessableEntity || flowsBody(t, w)["code"] != "FLOW_PURGE_NO_KEY" {
		t.Fatalf("purge without a master key = %d %s, want 422 FLOW_PURGE_NO_KEY", w.Code, w.Body.String())
	}
	security.RetainProcessMasterKey(strings.Repeat("0123456789abcdef", 4))
	t.Cleanup(func() { security.RetainProcessMasterKey("") })
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/credentials/purge", token, `{"snapshots":[".flows-v7-backup-nothere"]}`); w.Code != http.StatusBadRequest || flowsBody(t, w)["code"] != "FLOW_PURGE_SNAPSHOT_INVALID" {
		t.Fatalf("purge of a missing snapshot = %d %s", w.Code, w.Body.String())
	}
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/credentials/purge", token, `{"snapshots":[]}`); w.Code != http.StatusAccepted {
		t.Fatalf("purge = %d %s, want 202", w.Code, w.Body.String())
	}
	deadline := time.Now().Add(20 * time.Second)
	for {
		w := flowsCall(t, s, http.MethodGet, "/api/desktop/flows/credentials/purge", token, "")
		state := flowsBody(t, w)["state"].(map[string]any)
		if state["status"] == "done" {
			break
		}
		if state["status"] == "failed" || time.Now().After(deadline) {
			t.Fatalf("purge state = %v", state)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
```
Run `go test ./internal/server -run 'TestCredentialPurgeEndpoint|TestCredentialScanEndpoint' -count=1 2>&1 | tail -4`. Expected: `ok`.

- [ ] **Step 8: The client dialog, the api and the Node test**

Edit `ui/js/desktop/apps/easydrag-core.js` `createApi`: after the `credentialScan` line from B.8 add (mind the commas)
```js
            credentialPurgeState: signal => api(base + '/credentials/purge', { signal, cache: 'no-store' }),
            credentialPurgeStart: snapshots => api(base + '/credentials/purge', json('POST', { snapshots })),
```
In `ui/js/desktop/apps/easydrag-credentials-purge.js` replace the line `ED.credentialsPurge = { needsAttention, hintMarkup, mountHint, offer };` and the `offer` function with the code below (everything above `offer` stays):

```js
    const STEP_KEYS = ['backup', 'versions', 'redact', 'compact', 'snapshots', 'verify'];
    let dialog = null;

    function sizeText(bytes) {
        const n = Number(bytes) || 0;
        return n < 1024 ? n + ' B' : n < 1048576 ? (n / 1024).toFixed(1) + ' KB' : (n / 1048576).toFixed(1) + ' MB';
    }
    function dateText(value) { const d = new Date(value); return Number.isFinite(d.getTime()) ? d.toLocaleDateString() : ''; }

    // dialogMarkup says in plain words what will be deleted, blacked out, and what stays and why.
    function dialogMarkup(t, report) {
        const esc = ED.core.esc;
        const flowItem = (ref, key) => '<li><button type="button" class="ed-btn" data-ed-credentials-flow="' + esc(ref.flow_id) + '">' + esc(ref.name) + '</button> ' + esc(t(key)) + '</li>';
        const drafts = report.drafts || [], live = (report.live || []).filter(ref => !drafts.some(d => d.flow_id === ref.flow_id));
        let html = '<p>' + esc(t('easydrag.ui.cred_purge_versions', { count: report.versions_purgeable || 0 })) + '</p>' +
            '<p>' + esc(t('easydrag.ui.cred_purge_rows', { runs: report.runs || 0, rows: (report.steps || 0) + (report.test_rows || 0) })) + '</p>';
        if (report.versions_pinned > 0) html += '<p class="ed-hint">' + esc(t('easydrag.ui.cred_purge_pinned', { count: report.versions_pinned })) + '</p>';
        const stays = drafts.map(ref => flowItem(ref, 'easydrag.ui.cred_purge_move_first')).concat(live.map(ref => flowItem(ref, 'easydrag.ui.cred_purge_republish')));
        if (stays.length) html += '<ul>' + stays.join('') + '</ul>';
        const snapshots = report.snapshots || [];
        if (snapshots.length) {
            html += '<h3>' + esc(t('easydrag.ui.cred_purge_snapshots')) + '</h3><ul>' + snapshots.map(s => '<li><label><input type="checkbox" checked data-ed-credentials-snapshot="' + esc(s.name) + '"> ' +
                esc(t('easydrag.ui.cred_purge_snapshot_line', { name: s.name, date: dateText(s.mod_time), size: sizeText(s.bytes) })) + '</label></li>').join('') + '</ul>';
        }
        return html + '<p class="ed-hint">' + esc(t('easydrag.ui.cred_purge_outside')) + '</p><div data-ed-credentials-status role="status" aria-live="polite"></div>';
    }

    function stateMarkup(t, state) {
        const esc = ED.core.esc;
        const step = key => ED.core.tr(t, 'easydrag.ui.cred_step_' + key, key);
        let html = '';
        if (state.status === 'running') html = '<p>' + esc(t('easydrag.ui.cred_purge_step', { step: step(state.step) })) + '</p>';
        else if (state.status === 'failed') html = '<p class="ed-error">' + esc(t('easydrag.ui.cred_purge_failed', { step: step(state.failed_step) })) + '</p>';
        else if (state.status === 'done' && state.clean) html = '<p>' + esc(t('easydrag.ui.cred_purge_clean')) + '</p>';
        else if (state.status === 'done') html = '<p>' + esc(t('easydrag.ui.cred_purge_left')) + '</p><ul>' + (state.remaining || []).map(r => '<li>' + esc(r.where) + '</li>').join('') + '</ul>';
        if (state.backup && state.status !== 'running') html += '<p class="ed-hint">' + esc(t('easydrag.ui.cred_purge_backup', { name: state.backup })) + '</p>';
        return html;
    }

    // open shows the dialog with a fresh scan. With onlyIfNeeded it stays closed when nothing needs cleaning
    // (the editor offers it after every publish that followed a move).
    async function open(options) {
        const app = bound;
        if (!app || dialog) return;
        const t = app.t, core = ED.core;
        let report;
        try { report = (await app.api.credentialScan(true)).report; } catch (err) { return; }
        if (options && options.onlyIfNeeded && !needsAttention(report)) return;
        if (dialog) return;
        const host = document.querySelector('.ed-app') || document.body;
        let closed = false;
        const owned = dialog = core.modal(host, {
            title: t('easydrag.ui.cred_purge_title'), closeLabel: t('easydrag.ui.close'), className: 'ed-modal--credentials', body: dialogMarkup(t, report),
            actions: [{ id: 'close', label: t('easydrag.ui.close') }, { id: 'purge', label: t('easydrag.ui.cred_purge_go'), primary: true }],
            onAction(id, instance) {
                if (id === 'close') { instance.close('close'); return false; }
                if (id === 'purge') void start();
                return false;
            }
        });
        owned.done.then(() => { closed = true; dialog = null; offer(); });
        const status = () => owned.body.querySelector('[data-ed-credentials-status]');
        owned.body.addEventListener('click', event => {
            const flow = event.target.closest('[data-ed-credentials-flow]');
            if (flow) { const id = flow.dataset.edCredentialsFlow; owned.close('flow'); app.openFlow(id); }
        });
        async function start() {
            const names = [...owned.body.querySelectorAll('[data-ed-credentials-snapshot]')].filter(box => box.checked).map(box => box.dataset.edCredentialsSnapshot);
            owned.setBusy(true);
            try { await app.api.credentialPurgeStart(names); } catch (err) {
                owned.setBusy(false);
                status().innerHTML = '<p class="ed-error">' + core.esc(core.errorText(t, err)) + '</p>';
                return;
            }
            await follow();
        }
        async function follow() {
            while (!closed) {
                let state;
                try { state = (await app.api.credentialPurgeState()).state; } catch (err) { state = null; }
                if (closed) return;
                if (state) {
                    status().innerHTML = stateMarkup(t, state);
                    if (state.status !== 'running') { owned.setBusy(false); return; }
                }
                await new Promise(resolve => setTimeout(resolve, 1000));
            }
        }
    }

    // offer is what the editor calls after a move and a publish: the hints refresh and, when something
    // can be cleaned, the dialog opens.
    function offer(flowId) {
        hints.forEach(handle => { void handle.refresh(); });
        if (flowId !== undefined && bound) void open({ onlyIfNeeded: true });
        return flowId;
    }

    ED.credentialsPurge = { needsAttention, hintMarkup, dialogMarkup, stateMarkup, mountHint, open, offer };
```
In `mountHint` the click handler already calls `ED.credentialsPurge.open()` when it is a function, so it needs no change.

Extend `scripts/test-easydrag-credentials-purge.mjs` `run()` (before its closing brace) with:
```js
    const tt = (key, params) => key + (params ? JSON.stringify(params) : '');
    const full = Object.assign({}, empty, { versions_purgeable: 2, versions_pinned: 1, runs: 3, steps: 4, test_rows: 5,
        drafts: [{ flow_id: 'flow_a', name: 'A <i>' }], live: [{ flow_id: 'flow_a', name: 'A <i>' }, { flow_id: 'flow_b', name: 'B' }],
        snapshots: [{ name: '.flows-v7-backup-abc', bytes: 2048, mod_time: '2026-11-01T10:00:00Z' }] });
    const dialog = purge.dialogMarkup(tt, full);
    assert.ok(dialog.includes('cred_purge_versions{"count":2}') && dialog.includes('cred_purge_rows{"runs":3,"rows":9}') && dialog.includes('cred_purge_pinned{"count":1}'));
    assert.equal((dialog.match(/data-ed-credentials-flow=/g) || []).length, 2, 'a flow with a draft finding is listed once, as move first');
    assert.ok(dialog.includes('cred_purge_move_first') && dialog.includes('cred_purge_republish') && dialog.includes('A &#60;i&#62;'), 'names are escaped');
    assert.ok(/data-ed-credentials-snapshot="\.flows-v7-backup-abc"/.test(dialog) && /checked/.test(dialog), 'snapshots are checked by default');
    assert.ok(dialog.includes('cred_purge_outside'));
    assert.ok(purge.stateMarkup(tt, { status: 'running', step: 'redact' }).includes('cred_purge_step'));
    assert.ok(purge.stateMarkup(tt, { status: 'done', clean: true }).includes('cred_purge_clean'));
    assert.ok(purge.stateMarkup(tt, { status: 'done', clean: false, remaining: [{ where: 'flows.db-wal', count: 1 }] }).includes('flows.db-wal'));
    assert.ok(purge.stateMarkup(tt, { status: 'failed', failed_step: 'compact', backup: 'x.enc' }).includes('cred_purge_backup'));
```
The harness core already has the real `tr` (Task B.8), so `tt('easydrag.ui.cred_step_redact')` returns its own key and `tr` falls back to the raw step name.

Run: `cd <worktree> && node scripts/test-easydrag-credentials-purge.mjs`. Expected: `credentials purge hint checks passed`.

- [ ] **Step 9: Browser fixture and gated browser test**

Create `ui/testdata/easydrag-credentials-fixture.js`:

```js
// Browser-only fixture for the credential hint and purge dialog: it answers the scan and purge routes.
(function () {
    'use strict';
    const previous = window.fetch.bind(window), base = '/api/desktop/flows/credentials/';
    const reply = (body, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' } });
    const dirty = {
        generated_at: '2026-11-11T10:00:00Z', drafts: [{ flow_id: 'flow_cred1', name: 'Wetter', findings: 2 }], live: [], versions_purgeable: 3, versions_pinned: 1,
        runs: 4, steps: 5, test_rows: 1, backups: [], leftovers: [],
        snapshots: [{ name: '.flows-v7-backup-abc123', bytes: 4096, mod_time: '2026-10-01T10:00:00Z' }, { name: '.flows-v8-backup-def456', bytes: 8192, mod_time: '2026-11-01T10:00:00Z' }]
    };
    const state = { requests: [], purged: false, polls: 0 };
    window.fetch = async (value, options = {}) => {
        const url = new URL(String(value), location.origin), method = String(options.method || 'GET').toUpperCase();
        if (!url.pathname.startsWith(base)) return previous(value, options);
        const route = url.pathname.slice(base.length);
        state.requests.push({ route, method, body: options.body ? JSON.parse(options.body) : null });
        if (route === 'scan') return reply({ report: state.purged ? Object.assign({}, dirty, { drafts: [], versions_purgeable: 0, versions_pinned: 0, runs: 0, steps: 0, test_rows: 0, snapshots: [] }) : dirty });
        if (route === 'purge' && method === 'POST') { state.purged = true; return reply({ state: { status: 'running', step: 'backup' } }, 202); }
        if (route === 'purge') {
            state.polls++;
            return reply({ state: state.polls < 2 ? { status: 'running', step: 'redact' } : { status: 'done', clean: true, backup: '.flows-purge-backup-x.enc', remaining: [] } });
        }
        return reply({ code: 'FLOW_NOT_FOUND', error: 'unknown' }, 404);
    };
    window.edCredFixture = { state };
})();
```
Create `ui/desktop_easydrag_credentials_browser_test.go`:

```go
package ui

import (
	"testing"

	"github.com/go-rod/rod/lib/proto"
)

// TestDesktopEasyDragCredentialsPurgeBrowser walks the hint card and the purge dialog against a stubbed backend.
func TestDesktopEasyDragCredentialsPurgeBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t, "/testdata/easydrag-credentials-fixture.js")
	p := s.page
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}}}).Call(p); err != nil {
		t.Fatal(err)
	}
	p.MustEval(`()=>fixtureTheme('standard')`)
	s.wait(`()=>matchMedia('(prefers-reduced-motion: reduce)').matches`)
	// The hint shows while plaintext remains.
	s.wait(`()=>{const c=document.querySelector('[data-ed-credentials-hint]');return !!c && !c.hidden && c.textContent.includes(t('easydrag.ui.cred_hint_title'))}`)
	s.shot("credentials-hint")
	p.MustElement(`[data-ed-credentials-hint] [data-ed-credentials-open]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-modal--credentials [data-ed-credentials-snapshot]')`)
	if got := p.MustEval(`()=>document.querySelectorAll('.ed-modal--credentials [data-ed-credentials-snapshot]:checked').length`).Int(); got != 2 {
		t.Fatalf("checked snapshots = %d, want both checked by default", got)
	}
	// One snapshot is unchecked: it must not be sent.
	p.MustElement(`.ed-modal--credentials [data-ed-credentials-snapshot=".flows-v8-backup-def456"]`).MustClick()
	s.shot("credentials-dialog")
	p.MustElement(`.ed-modal--credentials [data-ed-action="purge"]`).MustClick()
	s.wait(`()=>{const s=document.querySelector('.ed-modal--credentials [data-ed-credentials-status]');return !!s && s.textContent.includes(t('easydrag.ui.cred_purge_clean'))}`)
	s.shot("credentials-done")
	got := p.MustEval(`()=>JSON.stringify(edCredFixture.state.requests.filter(r=>r.route==='purge'&&r.method==='POST').map(r=>r.body.snapshots))`).Str()
	if got != `[[".flows-v7-backup-abc123"]]` {
		t.Fatalf("purge request snapshots = %s, want only the checked one", got)
	}
	p.MustElement(`.ed-modal--credentials [data-ed-action="close"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal--credentials')&&document.querySelector('[data-ed-credentials-hint]').hidden`)
	s.noRawKeys("credentials purge dialog")
}
```
Run (gated; Chrome must start, see the Chrome-launch memory note if it cannot):
```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0b-shots go test ./ui -run TestDesktopEasyDragCredentialsPurgeBrowser -count=1 -timeout 10m 2>&1 | tail -6
```
Expected: `ok`. Without the env var the test skips (`go test ./ui -run TestDesktopEasyDragCredentialsPurgeBrowser -v` prints `SKIP`). If `.ed-home` is not the first page the hint mounts into, check the screenshot `credentials-hint` in `D:/tmp/easydrag-s0b-shots`.

- [ ] **Step 10: Locale keys**

Write `C:/disposable/easydrag-s0b/keys-b9.json` (27 keys; every language on one line per key, placeholders `{count}`, `{runs}`, `{rows}`, `{name}`, `{date}`, `{size}`, `{step}` unchanged):

```json
{
  "easydrag.ui.cred_purge_title": {"cs": "Vyčistit ručně zadané přístupové údaje", "da": "Ryd op i indtastede adgangsoplysninger", "de": "Getippte Zugangsdaten bereinigen", "el": "Καθαρισμός πληκτρολογημένων διαπιστευτηρίων", "en": "Clean up typed credentials", "es": "Limpiar credenciales escritas", "fr": "Nettoyer les identifiants saisis", "hi": "टाइप किए गए क्रेडेंशियल साफ़ करें", "it": "Ripulisci le credenziali digitate", "ja": "入力された認証情報のクリーンアップ", "nl": "Ingetypte inloggegevens opruimen", "no": "Rydd opp i innskrevet legitimasjon", "pl": "Wyczyść wpisane dane uwierzytelniające", "pt": "Limpar credenciais digitadas", "sv": "Rensa inskrivna inloggningsuppgifter", "zh": "清理手动输入的凭据"},
  "easydrag.ui.cred_purge_versions": {"cs": "Bude smazáno starých verzí: {count}.", "da": "{count} gamle versioner slettes.", "de": "{count} alte Versionen werden gelöscht.", "el": "Θα διαγραφούν {count} παλιές εκδόσεις.", "en": "{count} old versions will be deleted.", "es": "Se eliminarán {count} versiones antiguas.", "fr": "{count} anciennes versions seront supprimées.", "hi": "{count} पुराने संस्करण हटाए जाएँगे।", "it": "{count} vecchie versioni verranno eliminate.", "ja": "古いバージョン {count} 件が削除されます。", "nl": "{count} oude versies worden verwijderd.", "no": "{count} gamle versjoner slettes.", "pl": "Zostanie usuniętych starych wersji: {count}.", "pt": "{count} versões antigas serão eliminadas.", "sv": "{count} gamla versioner raderas.", "zh": "将删除 {count} 个旧版本。"},
  "easydrag.ui.cred_purge_rows": {"cs": "Bude začerněno běhů: {runs} a řádků kroků či testů: {rows}.", "da": "{runs} kørsler og {rows} trin- eller testrækker overstreges.", "de": "{runs} Läufe und {rows} Schritt- oder Testzeilen werden geschwärzt.", "el": "Θα μαυριστούν {runs} εκτελέσεις και {rows} γραμμές βημάτων ή δοκιμών.", "en": "{runs} runs and {rows} step or test rows will be blacked out.", "es": "Se tacharán {runs} ejecuciones y {rows} filas de pasos o pruebas.", "fr": "{runs} exécutions et {rows} lignes d'étapes ou de test seront noircies.", "hi": "{runs} रन और {rows} चरण या परीक्षण पंक्तियाँ काली कर दी जाएँगी।", "it": "{runs} esecuzioni e {rows} righe di passaggi o test verranno oscurate.", "ja": "実行 {runs} 件とステップ・テスト行 {rows} 件が黒塗りされます。", "nl": "{runs} runs en {rows} stap- of testrijen worden zwartgemaakt.", "no": "{runs} kjøringer og {rows} trinn- eller testrader blir sladdet.", "pl": "Zostanie zaczernionych uruchomień: {runs} oraz wierszy kroków lub testów: {rows}.", "pt": "{runs} execuções e {rows} linhas de passos ou testes serão ocultadas.", "sv": "{runs} körningar och {rows} steg- eller testrader maskeras.", "zh": "将涂黑 {runs} 次运行和 {rows} 条步骤或测试记录。"},
  "easydrag.ui.cred_purge_pinned": {"cs": "{count} verzí stále používají běžící flow. Smažou se hned po skončení jejich běhu.", "da": "{count} versioner bruges stadig af kørende flows. De slettes, så snart deres kørsel slutter.", "de": "{count} Versionen werden noch von laufenden Flows benutzt. Sie werden gelöscht, sobald ihr Lauf endet.", "el": "{count} εκδόσεις χρησιμοποιούνται ακόμη από flows που εκτελούνται. Διαγράφονται μόλις τελειώσει η εκτέλεσή τους.", "en": "{count} versions are still used by running flows. They are deleted as soon as their run ends.", "es": "{count} versiones aún las usan flujos en ejecución. Se eliminan en cuanto termina su ejecución.", "fr": "{count} versions sont encore utilisées par des flows en cours. Elles sont supprimées dès la fin de leur exécution.", "hi": "{count} संस्करण अभी भी चल रहे फ़्लो उपयोग कर रहे हैं। उनका रन खत्म होते ही वे हटा दिए जाएँगे।", "it": "{count} versioni sono ancora usate da flow in esecuzione. Vengono eliminate non appena la loro esecuzione termina.", "ja": "{count} 件のバージョンは実行中のフローがまだ使用しています。実行が終わり次第削除されます。", "nl": "{count} versies worden nog gebruikt door lopende flows. Ze worden verwijderd zodra hun run eindigt.", "no": "{count} versjoner brukes fortsatt av kjørende flows. De slettes så snart kjøringen deres er ferdig.", "pl": "{count} wersji jest nadal używanych przez działające flow. Zostaną usunięte, gdy tylko ich uruchomienie się zakończy.", "pt": "{count} versões ainda são usadas por fluxos em execução. São eliminadas assim que a execução terminar.", "sv": "{count} versioner används fortfarande av körande flöden. De raderas så snart körningen är klar.", "zh": "{count} 个版本仍被运行中的流程使用，其运行结束后会立即删除。"},
  "easydrag.ui.cred_purge_move_first": {"cs": "Nejdřív přesuň přístupový údaj do trezoru", "da": "Flyt først oplysningen til boksen", "de": "Zuerst in den Tresor verschieben", "el": "Μετακίνησε πρώτα το διαπιστευτήριο στο θησαυροφυλάκιο", "en": "Move the credential into the vault first", "es": "Primero mueve la credencial a la caja fuerte", "fr": "Déplace d'abord l'identifiant dans le coffre", "hi": "पहले क्रेडेंशियल को वॉल्ट में ले जाएँ", "it": "Sposta prima la credenziale nel vault", "ja": "先に認証情報を保管庫へ移してください", "nl": "Verplaats de inloggegevens eerst naar de kluis", "no": "Flytt legitimasjonen til hvelvet først", "pl": "Najpierw przenieś dane do skarbca", "pt": "Move primeiro a credencial para o cofre", "sv": "Flytta först uppgiften till valvet", "zh": "请先把凭据移入保险库"},
  "easydrag.ui.cred_purge_republish": {"cs": "Publikovat znovu", "da": "Udgiv igen", "de": "Neu veröffentlichen", "el": "Δημοσίευση ξανά", "en": "Publish again", "es": "Publicar de nuevo", "fr": "Republier", "hi": "फिर से प्रकाशित करें", "it": "Pubblica di nuovo", "ja": "再公開する", "nl": "Opnieuw publiceren", "no": "Publiser på nytt", "pl": "Opublikuj ponownie", "pt": "Publicar novamente", "sv": "Publicera igen", "zh": "重新发布"},
  "easydrag.ui.cred_purge_snapshots": {"cs": "Staré migrační snímky", "da": "Gamle migreringsøjebliksbilleder", "de": "Alte Migrationssicherungen", "el": "Παλιά στιγμιότυπα μετάβασης", "en": "Old migration snapshots", "es": "Instantáneas de migración antiguas", "fr": "Anciens instantanés de migration", "hi": "पुराने माइग्रेशन स्नैपशॉट", "it": "Vecchi snapshot di migrazione", "ja": "古い移行スナップショット", "nl": "Oude migratiesnapshots", "no": "Gamle migreringsøyeblikksbilder", "pl": "Stare migawki migracji", "pt": "Instantâneos de migração antigos", "sv": "Gamla migreringsögonblicksbilder", "zh": "旧的迁移快照"},
  "easydrag.ui.cred_purge_snapshot_line": {"cs": "{name} · {date} · {size} · obsahuje tehdejší čistý text", "da": "{name} · {date} · {size} · indeholder datidens klartekst", "de": "{name} · {date} · {size} · enthält Klartext von damals", "el": "{name} · {date} · {size} · περιέχει απλό κείμενο της εποχής", "en": "{name} · {date} · {size} · contains plaintext from back then", "es": "{name} · {date} · {size} · contiene texto sin cifrar de entonces", "fr": "{name} · {date} · {size} · contient du texte en clair de l'époque", "hi": "{name} · {date} · {size} · उस समय का सादा पाठ मौजूद है", "it": "{name} · {date} · {size} · contiene testo in chiaro di allora", "ja": "{name} · {date} · {size} · 当時の平文を含みます", "nl": "{name} · {date} · {size} · bevat leesbare tekst van toen", "no": "{name} · {date} · {size} · inneholder klartekst fra den gangen", "pl": "{name} · {date} · {size} · zawiera ówczesny jawny tekst", "pt": "{name} · {date} · {size} · contém texto simples da época", "sv": "{name} · {date} · {size} · innehåller klartext från den tiden", "zh": "{name} · {date} · {size} · 含有当时的明文"},
  "easydrag.ui.cred_purge_outside": {"cs": "AuraGo se nedostane ke starším úplným zálohám mimo datovou složku.", "da": "AuraGo kan ikke nå ældre fulde sikkerhedskopier uden for datamappen.", "de": "Ältere AuraGo-Gesamtsicherungen außerhalb des Datenordners kann AuraGo nicht erreichen.", "el": "Το AuraGo δεν μπορεί να φτάσει σε παλαιότερα πλήρη αντίγραφα ασφαλείας εκτός του φακέλου δεδομένων.", "en": "AuraGo cannot reach older full backups outside the data folder.", "es": "AuraGo no puede acceder a copias de seguridad completas más antiguas fuera de la carpeta de datos.", "fr": "AuraGo ne peut pas atteindre les anciennes sauvegardes complètes situées hors du dossier de données.", "hi": "डेटा फ़ोल्डर के बाहर रखे पुराने पूर्ण बैकअप तक AuraGo नहीं पहुँच सकता।", "it": "AuraGo non può raggiungere i backup completi più vecchi fuori dalla cartella dei dati.", "ja": "データフォルダの外にある古い全体バックアップには AuraGo は触れられません。", "nl": "AuraGo kan oudere volledige back-ups buiten de gegevensmap niet bereiken.", "no": "AuraGo kan ikke nå eldre fullstendige sikkerhetskopier utenfor datamappen.", "pl": "AuraGo nie ma dostępu do starszych pełnych kopii zapasowych poza folderem danych.", "pt": "O AuraGo não consegue alcançar cópias de segurança completas mais antigas fora da pasta de dados.", "sv": "AuraGo kan inte nå äldre fullständiga säkerhetskopior utanför datamappen.", "zh": "AuraGo 无法处理数据文件夹之外的旧版完整备份。"},
  "easydrag.ui.cred_purge_go": {"cs": "Vyčistit hned", "da": "Ryd op nu", "de": "Jetzt bereinigen", "el": "Καθαρισμός τώρα", "en": "Clean up now", "es": "Limpiar ahora", "fr": "Nettoyer maintenant", "hi": "अभी साफ़ करें", "it": "Ripulisci ora", "ja": "今すぐクリーンアップ", "nl": "Nu opruimen", "no": "Rydd opp nå", "pl": "Wyczyść teraz", "pt": "Limpar agora", "sv": "Rensa nu", "zh": "立即清理"},
  "easydrag.ui.cred_purge_step": {"cs": "Probíhá: {step}", "da": "Arbejder: {step}", "de": "In Arbeit: {step}", "el": "Σε εξέλιξη: {step}", "en": "Working: {step}", "es": "En curso: {step}", "fr": "En cours : {step}", "hi": "जारी है: {step}", "it": "In corso: {step}", "ja": "実行中: {step}", "nl": "Bezig: {step}", "no": "Pågår: {step}", "pl": "W toku: {step}", "pt": "Em curso: {step}", "sv": "Pågår: {step}", "zh": "进行中：{step}"},
  "easydrag.ui.cred_purge_clean": {"cs": "Hotovo. Žádný ručně zadaný přístupový údaj nezůstal.", "da": "Færdig. Ingen indtastede adgangsoplysninger er tilbage.", "de": "Fertig. Es ist kein getipptes Zugangsdatum mehr übrig.", "el": "Έτοιμο. Δεν έμεινε κανένα πληκτρολογημένο διαπιστευτήριο.", "en": "Done. No typed credential is left.", "es": "Listo. No queda ninguna credencial escrita.", "fr": "Terminé. Il ne reste aucun identifiant saisi.", "hi": "हो गया। टाइप किया गया कोई क्रेडेंशियल नहीं बचा।", "it": "Fatto. Non è rimasta nessuna credenziale digitata.", "ja": "完了しました。入力された認証情報は残っていません。", "nl": "Klaar. Er zijn geen ingetypte inloggegevens meer over.", "no": "Ferdig. Ingen innskrevet legitimasjon er igjen.", "pl": "Gotowe. Nie pozostały żadne wpisane dane uwierzytelniające.", "pt": "Concluído. Não resta nenhuma credencial digitada.", "sv": "Klart. Ingen inskriven inloggningsuppgift finns kvar.", "zh": "完成。没有残留的手动输入凭据。"},
  "easydrag.ui.cred_purge_left": {"cs": "Hotovo, ale tato místa stále obsahují přístupový údaj:", "da": "Færdig, men disse steder indeholder stadig en adgangsoplysning:", "de": "Fertig, aber diese Stellen enthalten noch ein Zugangsdatum:", "el": "Έτοιμο, αλλά αυτά τα σημεία περιέχουν ακόμη διαπιστευτήριο:", "en": "Done, but these places still hold a credential:", "es": "Listo, pero estos lugares aún contienen una credencial:", "fr": "Terminé, mais ces emplacements contiennent encore un identifiant :", "hi": "हो गया, लेकिन इन जगहों में अब भी क्रेडेंशियल है:", "it": "Fatto, ma questi punti contengono ancora una credenziale:", "ja": "完了しましたが、次の場所に認証情報が残っています:", "nl": "Klaar, maar deze plekken bevatten nog inloggegevens:", "no": "Ferdig, men disse stedene inneholder fortsatt legitimasjon:", "pl": "Gotowe, ale te miejsca nadal zawierają dane uwierzytelniające:", "pt": "Concluído, mas estes locais ainda contêm uma credencial:", "sv": "Klart, men dessa platser innehåller fortfarande en inloggningsuppgift:", "zh": "完成，但以下位置仍含有凭据："},
  "easydrag.ui.cred_purge_failed": {"cs": "Čištění se zastavilo u kroku {step}. Šifrovaná záloha zůstala zachována, můžeš ho spustit znovu.", "da": "Oprydningen stoppede ved {step}. Den krypterede sikkerhedskopi blev beholdt, så du kan køre den igen.", "de": "Die Bereinigung ist bei {step} stehen geblieben. Die verschlüsselte Sicherung blieb erhalten, du kannst es erneut versuchen.", "el": "Ο καθαρισμός σταμάτησε στο βήμα {step}. Το κρυπτογραφημένο αντίγραφο ασφαλείας διατηρήθηκε, μπορείς να τον ξανατρέξεις.", "en": "The cleanup stopped at {step}. The encrypted safety backup was kept, so you can run it again.", "es": "La limpieza se detuvo en {step}. La copia de seguridad cifrada se conservó, así que puedes repetirla.", "fr": "Le nettoyage s'est arrêté à l'étape {step}. La sauvegarde chiffrée a été conservée, tu peux le relancer.", "hi": "सफ़ाई {step} पर रुक गई। एन्क्रिप्टेड सुरक्षा बैकअप रखा गया है, इसलिए आप इसे फिर चला सकते हैं।", "it": "La pulizia si è fermata a {step}. Il backup cifrato è stato conservato, puoi rilanciarla.", "ja": "クリーンアップは {step} で止まりました。暗号化した安全バックアップは残してあるので、もう一度実行できます。", "nl": "De opruiming stopte bij {step}. De versleutelde back-up is bewaard, je kunt het opnieuw proberen.", "no": "Oppryddingen stoppet ved {step}. Den krypterte sikkerhetskopien ble beholdt, så du kan kjøre den på nytt.", "pl": "Czyszczenie zatrzymało się na kroku {step}. Zaszyfrowana kopia zapasowa została zachowana, możesz uruchomić je ponownie.", "pt": "A limpeza parou em {step}. A cópia de segurança cifrada foi mantida, por isso podes executá-la de novo.", "sv": "Rensningen stannade vid {step}. Den krypterade säkerhetskopian sparades, så du kan köra den igen.", "zh": "清理在“{step}”处停止。加密的安全备份已保留，你可以重新运行。"},
  "easydrag.ui.cred_purge_backup": {"cs": "Záloha pro jistotu (šifrovaná, po 7 dnech smazána): {name}", "da": "Sikkerhedskopi (krypteret, slettes efter 7 dage): {name}", "de": "Sicherung (verschlüsselt, nach 7 Tagen gelöscht): {name}", "el": "Αντίγραφο ασφαλείας (κρυπτογραφημένο, διαγράφεται μετά από 7 ημέρες): {name}", "en": "Safety backup (encrypted, deleted after 7 days): {name}", "es": "Copia de seguridad (cifrada, se borra tras 7 días): {name}", "fr": "Sauvegarde (chiffrée, supprimée après 7 jours) : {name}", "hi": "सुरक्षा बैकअप (एन्क्रिप्टेड, 7 दिन बाद हटाया जाता है): {name}", "it": "Backup di sicurezza (cifrato, eliminato dopo 7 giorni): {name}", "ja": "安全バックアップ（暗号化・7日後に削除）: {name}", "nl": "Veiligheidsback-up (versleuteld, na 7 dagen verwijderd): {name}", "no": "Sikkerhetskopi (kryptert, slettes etter 7 dager): {name}", "pl": "Kopia bezpieczeństwa (zaszyfrowana, usuwana po 7 dniach): {name}", "pt": "Cópia de segurança (cifrada, eliminada após 7 dias): {name}", "sv": "Säkerhetskopia (krypterad, raderas efter 7 dagar): {name}", "zh": "安全备份（已加密，7 天后删除）：{name}"},
  "easydrag.ui.cred_step_backup": {"cs": "Záloha", "da": "Sikkerhedskopi", "de": "Sicherung", "el": "Αντίγραφο ασφαλείας", "en": "Safety backup", "es": "Copia de seguridad", "fr": "Sauvegarde", "hi": "सुरक्षा बैकअप", "it": "Backup", "ja": "安全バックアップ", "nl": "Veiligheidsback-up", "no": "Sikkerhetskopi", "pl": "Kopia bezpieczeństwa", "pt": "Cópia de segurança", "sv": "Säkerhetskopia", "zh": "安全备份"},
  "easydrag.ui.cred_step_versions": {"cs": "Staré verze", "da": "Gamle versioner", "de": "Alte Versionen", "el": "Παλιές εκδόσεις", "en": "Old versions", "es": "Versiones antiguas", "fr": "Anciennes versions", "hi": "पुराने संस्करण", "it": "Vecchie versioni", "ja": "古いバージョン", "nl": "Oude versies", "no": "Gamle versjoner", "pl": "Stare wersje", "pt": "Versões antigas", "sv": "Gamla versioner", "zh": "旧版本"},
  "easydrag.ui.cred_step_redact": {"cs": "Začerňování", "da": "Overstregning", "de": "Schwärzen", "el": "Μαύρισμα", "en": "Blacking out", "es": "Tachado", "fr": "Noircissement", "hi": "काला करना", "it": "Oscuramento", "ja": "黒塗り", "nl": "Zwartmaken", "no": "Sladding", "pl": "Zaczernianie", "pt": "Ocultação", "sv": "Maskering", "zh": "涂黑"},
  "easydrag.ui.cred_step_compact": {"cs": "Zhutňování", "da": "Komprimering", "de": "Verdichten", "el": "Συμπίεση", "en": "Compacting", "es": "Compactación", "fr": "Compactage", "hi": "संकुचन", "it": "Compattazione", "ja": "圧縮", "nl": "Comprimeren", "no": "Komprimering", "pl": "Kompaktowanie", "pt": "Compactação", "sv": "Komprimering", "zh": "压缩"},
  "easydrag.ui.cred_step_snapshots": {"cs": "Snímky", "da": "Øjebliksbilleder", "de": "Sicherungskopien", "el": "Στιγμιότυπα", "en": "Snapshots", "es": "Instantáneas", "fr": "Instantanés", "hi": "स्नैपशॉट", "it": "Snapshot", "ja": "スナップショット", "nl": "Snapshots", "no": "Øyeblikksbilder", "pl": "Migawki", "pt": "Instantâneos", "sv": "Ögonblicksbilder", "zh": "快照"},
  "easydrag.ui.cred_step_verify": {"cs": "Kontrola", "da": "Kontrol", "de": "Prüfen", "el": "Έλεγχος", "en": "Checking", "es": "Comprobación", "fr": "Vérification", "hi": "जाँच", "it": "Verifica", "ja": "確認", "nl": "Controleren", "no": "Kontroll", "pl": "Sprawdzanie", "pt": "Verificação", "sv": "Kontroll", "zh": "检查"},
  "easydrag.ui.error_flow_purge_busy": {"cs": "Čištění už běží.", "da": "En oprydning kører allerede.", "de": "Eine Bereinigung läuft bereits.", "el": "Ένας καθαρισμός εκτελείται ήδη.", "en": "A cleanup is already running.", "es": "Ya hay una limpieza en curso.", "fr": "Un nettoyage est déjà en cours.", "hi": "एक सफ़ाई पहले से चल रही है।", "it": "Una pulizia è già in corso.", "ja": "クリーンアップはすでに実行中です。", "nl": "Er loopt al een opruiming.", "no": "En opprydding pågår allerede.", "pl": "Czyszczenie już trwa.", "pt": "Já existe uma limpeza em curso.", "sv": "En rensning pågår redan.", "zh": "已有清理正在运行。"},
  "easydrag.ui.error_flow_purge_runs_active": {"cs": "Právě běží nějaký běh flow. Zkus to znovu, až skončí.", "da": "En flow-kørsel kører lige nu. Prøv igen, når den er færdig.", "de": "Gerade läuft ein Flow. Versuche es erneut, wenn er fertig ist.", "el": "Εκτελείται αυτή τη στιγμή ένα flow. Δοκίμασε ξανά όταν τελειώσει.", "en": "A flow run is running right now. Try again when it has finished.", "es": "Ahora mismo se está ejecutando un flujo. Inténtalo de nuevo cuando termine.", "fr": "Un flow est en cours d'exécution. Réessaie quand il sera terminé.", "hi": "अभी एक फ़्लो चल रहा है। उसके खत्म होने पर फिर कोशिश करें।", "it": "Un flow è in esecuzione in questo momento. Riprova quando ha finito.", "ja": "今、フローが実行中です。終わってからもう一度お試しください。", "nl": "Er draait nu een flow. Probeer het opnieuw als die klaar is.", "no": "Et flow kjører akkurat nå. Prøv igjen når det er ferdig.", "pl": "W tej chwili działa flow. Spróbuj ponownie, gdy się zakończy.", "pt": "Está a decorrer um fluxo neste momento. Tenta de novo quando terminar.", "sv": "Ett flöde körs just nu. Försök igen när det är klart.", "zh": "当前有流程正在运行。请在它结束后重试。"},
  "easydrag.ui.error_flow_purge_no_key": {"cs": "Hlavní klíč není k dispozici, proto nelze zapsat šifrovanou zálohu.", "da": "Hovednøglen er ikke tilgængelig, så den krypterede sikkerhedskopi kan ikke skrives.", "de": "Der Hauptschlüssel fehlt, deshalb kann die verschlüsselte Sicherung nicht geschrieben werden.", "el": "Το κύριο κλειδί δεν είναι διαθέσιμο, οπότε δεν μπορεί να γραφτεί το κρυπτογραφημένο αντίγραφο ασφαλείας.", "en": "The master key is not available, so the encrypted safety backup cannot be written.", "es": "La clave maestra no está disponible, por lo que no se puede escribir la copia de seguridad cifrada.", "fr": "La clé maître n'est pas disponible, la sauvegarde chiffrée ne peut donc pas être écrite.", "hi": "मास्टर कुंजी उपलब्ध नहीं है, इसलिए एन्क्रिप्टेड सुरक्षा बैकअप नहीं लिखा जा सकता।", "it": "La chiave principale non è disponibile, quindi non si può scrivere il backup cifrato.", "ja": "マスターキーが利用できないため、暗号化した安全バックアップを書き込めません。", "nl": "De hoofdsleutel is niet beschikbaar, dus de versleutelde back-up kan niet worden geschreven.", "no": "Hovednøkkelen er ikke tilgjengelig, så den krypterte sikkerhetskopien kan ikke skrives.", "pl": "Klucz główny jest niedostępny, więc nie można zapisać zaszyfrowanej kopii zapasowej.", "pt": "A chave mestra não está disponível, por isso não é possível escrever a cópia de segurança cifrada.", "sv": "Huvudnyckeln är inte tillgänglig, så den krypterade säkerhetskopian kan inte skrivas.", "zh": "主密钥不可用，因此无法写入加密的安全备份。"},
  "easydrag.ui.error_flow_purge_no_space": {"cs": "Na disku není dost volného místa pro čištění.", "da": "Der er ikke nok ledig diskplads til oprydningen.", "de": "Für die Bereinigung ist nicht genug freier Speicherplatz vorhanden.", "el": "Δεν υπάρχει αρκετός ελεύθερος χώρος στον δίσκο για τον καθαρισμό.", "en": "There is not enough free disk space for the cleanup.", "es": "No hay suficiente espacio libre en disco para la limpieza.", "fr": "Il n'y a pas assez d'espace disque libre pour le nettoyage.", "hi": "सफ़ाई के लिए डिस्क पर पर्याप्त खाली जगह नहीं है।", "it": "Non c'è abbastanza spazio libero su disco per la pulizia.", "ja": "クリーンアップに必要なディスクの空き容量が足りません。", "nl": "Er is niet genoeg vrije schijfruimte voor de opruiming.", "no": "Det er ikke nok ledig diskplass til oppryddingen.", "pl": "Brakuje wolnego miejsca na dysku do czyszczenia.", "pt": "Não há espaço livre suficiente em disco para a limpeza.", "sv": "Det finns inte tillräckligt med ledigt diskutrymme för rensningen.", "zh": "磁盘可用空间不足，无法进行清理。"},
  "easydrag.ui.error_flow_purge_snapshot_invalid": {"cs": "Jeden z vybraných snímků už neexistuje. Načti dialog znovu.", "da": "Et af de valgte øjebliksbilleder findes ikke længere. Genindlæs dialogen.", "de": "Eine der gewählten Sicherungskopien gibt es nicht mehr. Lade den Dialog neu.", "el": "Ένα από τα επιλεγμένα στιγμιότυπα δεν υπάρχει πια. Φόρτωσε ξανά το παράθυρο.", "en": "One of the chosen snapshots no longer exists. Reload the dialog.", "es": "Una de las instantáneas elegidas ya no existe. Vuelve a cargar el cuadro.", "fr": "Un des instantanés choisis n'existe plus. Recharge la fenêtre.", "hi": "चुने गए स्नैपशॉट में से एक अब मौजूद नहीं है। संवाद फिर से लोड करें।", "it": "Uno degli snapshot scelti non esiste più. Ricarica la finestra.", "ja": "選んだスナップショットの1つはもう存在しません。ダイアログを開き直してください。", "nl": "Een van de gekozen snapshots bestaat niet meer. Laad het venster opnieuw.", "no": "Et av de valgte øyeblikksbildene finnes ikke lenger. Last dialogen på nytt.", "pl": "Jedna z wybranych migawek już nie istnieje. Załaduj okno ponownie.", "pt": "Um dos instantâneos escolhidos já não existe. Recarrega a janela.", "sv": "En av de valda ögonblicksbilderna finns inte längre. Ladda om dialogrutan.", "zh": "所选快照之一已不存在。请重新打开对话框。"},
  "easydrag.ui.error_flow_purge_unavailable": {"cs": "Databáze flow není uložena v souboru, takže ji nelze vyčistit.", "da": "Flow-databasen er ikke gemt i en fil og kan derfor ikke ryddes.", "de": "Die Flow-Datenbank liegt nicht in einer Datei und kann daher nicht bereinigt werden.", "el": "Η βάση δεδομένων των flows δεν είναι αποθηκευμένη σε αρχείο και δεν μπορεί να καθαριστεί.", "en": "The flows database is not stored in a file, so it cannot be cleaned.", "es": "La base de datos de flujos no está en un archivo, por lo que no se puede limpiar.", "fr": "La base de données des flows n'est pas stockée dans un fichier, elle ne peut donc pas être nettoyée.", "hi": "फ़्लो डेटाबेस किसी फ़ाइल में संग्रहीत नहीं है, इसलिए उसे साफ़ नहीं किया जा सकता।", "it": "Il database dei flow non è salvato in un file, quindi non può essere ripulito.", "ja": "フローのデータベースがファイルに保存されていないため、クリーンアップできません。", "nl": "De flow-database staat niet in een bestand en kan dus niet worden opgeruimd.", "no": "Flow-databasen er ikke lagret i en fil og kan derfor ikke ryddes.", "pl": "Baza danych flow nie jest zapisana w pliku, więc nie można jej wyczyścić.", "pt": "A base de dados dos fluxos não está guardada num ficheiro, por isso não pode ser limpa.", "sv": "Flödesdatabasen ligger inte i en fil och kan därför inte rensas.", "zh": "流程数据库未保存在文件中，无法清理。"}
}
```
`cd <worktree> && node C:/disposable/easydrag-s0b/add-locale-keys.mjs . C:/disposable/easydrag-s0b/keys-b9.json`. Expected: `added up to 27 keys to 16 languages`.

- [ ] **Step 11: Gates and commit**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && GOTMPDIR='D:\tmp\go-build' go vet ./internal/flows ./internal/security && go vet ./internal/server ./ui && gofmt -l internal/flows internal/server ui && node scripts/test-easydrag-credentials-purge.mjs && node scripts/test-easydrag.mjs | tail -1 && node scripts/test-easydrag-render-baseline.mjs | tail -1 && go test ./ui -run 'EasyDrag|Locale|NoExternal|ShippedWebAssets' -count=1 2>&1 | tail -2 && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows -count=1 2>&1 | tail -3
```
Expected: clean vet and gofmt, `credentials purge hint checks passed`, `all EasyDrag checks passed`, `PASS render baseline (unchanged)`, `ok` twice. The full `internal/flows` run takes a few minutes; a failure in a runner or durable test means the hold broke the dispatch: re-read Step 3.

Write `C:/disposable/easydrag-s0b/msg-b9.txt`:
```
feat(flows): guided credential purge with a run hold and a dialog

POST /api/desktop/flows/credentials/purge starts one background job: free
space check, encrypted safety backup, mark and delete superseded versions
(pinned ones follow when their run ends), blacking out of runs, steps and
test data by the rule and by exact literal, secure_delete + VACUUM + WAL
truncate, the chosen migration snapshots, and a byte-for-byte verify.
GET reports the state. It refuses while a run is running; Runner
HoldDispatch queues new runs until the job ends. Every step is
idempotent. The dialog lives in easydrag-credentials-purge.js.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <your model line>
```
```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && git add internal/flows/runner.go internal/flows/runner_hold.go internal/flows/runner_hold_test.go internal/flows/purge_job.go internal/flows/purge_steps.go internal/flows/purge_job_test.go internal/flows/service.go internal/server/flows_credentials.go internal/server/flows_handlers.go internal/server/flows_purge_test.go ui/js/desktop/apps/easydrag-core.js ui/js/desktop/apps/easydrag-credentials-purge.js ui/testdata/easydrag-credentials-fixture.js ui/desktop_easydrag_credentials_browser_test.go ui/desktop_easydrag_test.go scripts/test-easydrag-credentials-purge.mjs ui/lang/easydrag && git commit -F C:/disposable/easydrag-s0b/msg-b9.txt
```

---
### Task B.10: The acceptance fixture

The whole Part B journey as one Go test on a real temporary data directory (spec §7, roadmap line 319). It adds no product code; if a step fails, the failing assertion names the task whose code is wrong.

**Files:**
- Create: `internal/server/flows_credentials_acceptance_test.go`
- Modify: `internal/flows/AGENTS.md` (the "Credential purge backups" section from B.7 gets the purge description)
- Test: `internal/server/flows_credentials_acceptance_test.go`

- [ ] **Step 1: Write the acceptance test**

Create `internal/server/flows_credentials_acceptance_test.go` with the file below. The fixture is an `http.request` with a token in an `Authorization` header, one in the `api_key` query parameter and one in a body field.

```go
package server

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"aurago/internal/flows"
	"aurago/internal/security"
)

const (
	accHeader = "HDRTOKEN-0123456789abcdef"
	accQuery  = "QRYTOKEN-0123456789abcdef"
	accBody   = "BODYTOKEN-0123456789abcdef"
	accKey    = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

const accFlowJSON = `{"schema":1,"name":"Wetter","nodes":[
 {"id":"n_aaaaaaaa","key":"start","type":"trigger.manual","type_version":1,"label":"Start","position":{"x":0,"y":0},"params":{}},
 {"id":"n_bbbbbbbb","key":"req","type":"http.request","type_version":1,"label":"Anfrage","position":{"x":300,"y":0},
  "params":{"method":"POST","url":"https://api.example.com/v1/items?api_key=QRYTOKEN-0123456789abcdef",
   "headers":{"Authorization":"Bearer HDRTOKEN-0123456789abcdef"},"body":"{\"client_secret\":\"BODYTOKEN-0123456789abcdef\"}"}}],
 "edges":[{"id":"e_aaaaaaaa","source":{"node":"n_aaaaaaaa","port":"out"},"target":{"node":"n_bbbbbbbb","port":"in"}}]}`

// filesHolding returns, for each needle, the files under root (relative, slash-separated, sorted) that hold it.
func filesHolding(t *testing.T, root string, needles ...string) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	skipped := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			skipped++
			return nil
		}
		for _, n := range needles {
			if bytes.Contains(data, []byte(n)) {
				rel, _ := filepath.Rel(root, path)
				out[n] = append(out[n], filepath.ToSlash(rel))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, files := range out {
		sort.Strings(files)
	}
	if skipped > 0 {
		t.Logf("%d unreadable files skipped in the byte search", skipped)
	}
	return out
}

func countWhere(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func TestCredentialHygieneAcceptance(t *testing.T) {
	security.RetainProcessMasterKey(accKey)
	t.Cleanup(func() { security.RetainProcessMasterKey("") })
	s, token := newFlowsTestServer(t)
	ctx := context.Background()
	dataDir := filepath.Dir(s.flowsDBPath())
	call := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		return flowsCall(t, s, method, "/api/desktop/flows"+path, token, body)
	}

	// 1. A flow with a token in a header, a query parameter and a body field, published twice.
	doc, err := flows.ParseFlow([]byte(accFlowJSON))
	if err != nil {
		t.Fatal(err)
	}
	rec, err := s.Flows.CreateFlow(ctx, flows.CreateRequest{Import: doc})
	if err != nil {
		t.Fatal(err)
	}
	id := rec.ID
	found := map[string]bool{}
	for _, f := range flows.FindLiteralCredentials(rec.Draft) {
		found[f.Location] = true
	}
	if !found["header"] || !found["url_query"] || !found["body_field"] {
		t.Fatalf("findings by location = %v, want header, url_query and body_field (B1's rule)", found)
	}
	first, issues, err := s.Flows.Publish(ctx, id, rec.DraftRevision) // revision 1
	if err != nil {
		t.Fatalf("Publish 1: %v, issues %+v", err, issues)
	}
	first.Draft.Name = "Wetter 2"
	draft, _, err := s.Flows.SaveDraft(ctx, id, first.Draft, first.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Flows.Publish(ctx, id, draft); err != nil { // revision 2, live for now
		t.Fatalf("Publish 2: %v", err)
	}

	// 2, 3 and 4. History from before the redaction existed (a finished run, its steps, test data), a
	// migration snapshot that holds all of it, and revision 2 pinned by a waiting run. The rows are written
	// the way an old AuraGo wrote them: in clear.
	raw, err := sql.Open("sqlite", s.flowsDBPath())
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	raw.SetMaxOpenConns(1)
	if _, err := raw.Exec(`PRAGMA busy_timeout = 10000`); err != nil {
		t.Fatal(err)
	}
	ts := time.Now().UTC().Format("2006-01-02T15:04:05.000000Z")
	params := `{"url":"https://api.example.com/v1/items?api_key=` + accQuery + `","headers":{"Authorization":"Bearer ` + accHeader + `"},"body":"{\"client_secret\":\"` + accBody + `\"}"}`
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO flow_runs (id, flow_id, revision, mode, status, started_at, trigger_data_json) VALUES ('run_old', ?, 1, 'live', 'success', ?, ?)`, []any{id, ts, `{"api_key":"` + accQuery + `"}`}},
		{`INSERT INTO flow_runs (id, flow_id, revision, mode, status, started_at) VALUES ('run_wait', ?, 2, 'live', 'waiting', ?)`, []any{id, ts}},
		{`INSERT INTO flow_run_steps (run_id, seq, node_id, status, params_json, output_json) VALUES ('run_old', 1, 'n_bbbbbbbb', 'success', ?, '{}')`, []any{params}},
		{`INSERT INTO flow_test_data (flow_id, node_id, kind, json, updated_at) VALUES (?, 'n_aaaaaaaa', 'trigger', ?, ?)`, []any{id, `{"token":"` + accQuery + `"}`, ts}},
	} {
		if _, err := raw.Exec(stmt.sql, stmt.args...); err != nil {
			t.Fatal(err)
		}
	}
	const snapName = ".flows-v7-backup-abc123"
	if err := os.MkdirAll(filepath.Join(dataDir, snapName), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`VACUUM INTO ?`, filepath.Join(dataDir, snapName, "flows.db")); err != nil {
		t.Fatal(err)
	}

	// 5. Move the header and the query token through the move endpoint (B.3); the body field cannot move.
	cur, err := s.Flows.GetFlow(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	nodeID := ""
	for _, n := range cur.Draft.Nodes {
		if n.Type == "http.request" {
			nodeID = n.ID
		}
	}
	move := func(location, path, name string) *httptest.ResponseRecorder {
		t.Helper()
		body := fmt.Sprintf(`{"revision":%d,"node_id":%q,"location":%q,"path":%q,"secret_name":%q,"replace":false}`, cur.DraftRevision, nodeID, location, path, name)
		return call(http.MethodPost, "/"+id+"/credentials/move", body)
	}
	for _, m := range [][3]string{{"header", "Authorization", "wetter_header"}, {"url_query", "api_key", "wetter_query"}} {
		w := move(m[0], m[1], m[2])
		if w.Code != http.StatusOK || strings.Contains(w.Body.String(), accHeader) || strings.Contains(w.Body.String(), accQuery) {
			t.Fatalf("move %s = %d %s", m[0], w.Code, w.Body.String())
		}
		if cur, err = s.Flows.GetFlow(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	if w := move("body_field", "client_secret", "wetter_body"); w.Code != http.StatusUnprocessableEntity || flowsBody(t, w)["code"] != "FLOW_CREDENTIAL_NOT_MOVABLE" {
		t.Fatalf("moving a body field = %d %s, want 422 FLOW_CREDENTIAL_NOT_MOVABLE", w.Code, w.Body.String())
	}
	if _, issues, err := s.Flows.Publish(ctx, id, cur.DraftRevision); err != nil { // revision 3, live
		t.Fatalf("Publish 3: %v, issues %+v", err, issues)
	}

	// 5. Purge: one job, the chosen snapshot, nothing running.
	if w := call(http.MethodPost, "/credentials/purge", `{"snapshots":["`+snapName+`"]}`); w.Code != http.StatusAccepted {
		t.Fatalf("purge = %d %s", w.Code, w.Body.String())
	}
	var state map[string]any
	for deadline := time.Now().Add(30 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		state = flowsBody(t, call(http.MethodGet, "/credentials/purge", ""))["state"].(map[string]any)
		if state["status"] != "running" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the purge did not finish: %v", state)
		}
	}
	if state["status"] != "done" || state["clean"] != true || state["versions_deleted"] != float64(1) || state["versions_pinned"] != float64(1) {
		t.Fatalf("purge state = %v, want done, clean, 1 deleted and 1 pinned", state)
	}
	backup := filepath.Join(dataDir, state["backup"].(string))

	// 7. The pinned revision survives and cannot be rolled back to; nothing else holds the header or the query token.
	versions, err := s.Flows.ListVersions(ctx, id)
	if err != nil || len(versions) != 2 || versions[0].Revision != 3 || versions[1].Revision != 2 {
		t.Fatalf("versions = %+v, %v, want revisions 3 and 2", versions, err)
	}
	if w := call(http.MethodPost, "/"+id+"/versions", fmt.Sprintf(`{"revision":2,"base_revision":%d}`, cur.DraftRevision)); w.Code != http.StatusConflict || flowsBody(t, w)["code"] != "FLOW_VERSION_PURGED" {
		t.Fatalf("rollback to the pinned marked version = %d %s", w.Code, w.Body.String())
	}
	for _, tok := range []string{accHeader, accQuery} {
		if n := countWhere(t, raw, `SELECT COUNT(*) FROM flow_versions WHERE instr(json, ?) > 0 AND revision <> 2`, tok); n != 0 {
			t.Fatalf("%s is in %d versions besides the pinned one", tok, n)
		}
		for _, col := range [][2]string{{"flow_runs", "trigger_data_json"}, {"flow_run_steps", "params_json"}, {"flow_test_data", "json"}, {"flows", "draft_json"}, {"flows", "live_json"}} {
			if n := countWhere(t, raw, `SELECT COUNT(*) FROM `+col[0]+` WHERE instr(`+col[1]+`, ?) > 0`, tok); n != 0 {
				t.Fatalf("%s is still in %s.%s", tok, col[0], col[1])
			}
		}
	}
	if n := countWhere(t, raw, `SELECT COUNT(*) FROM flow_runs r WHERE instr(r.trigger_data_json, ?) > 0`, accBody) +
		countWhere(t, raw, `SELECT COUNT(*) FROM flow_run_steps WHERE instr(params_json, ?) > 0`, accBody); n != 0 {
		t.Fatalf("the body token is still in the run history (%d rows)", n)
	}
	if _, err := os.Stat(filepath.Join(dataDir, snapName)); !os.IsNotExist(err) {
		t.Fatalf("the migration snapshot survived: %v", err)
	}

	// 8. The encrypted backup holds no plaintext, and 9. decrypts into a database that opens and holds the old versions.
	enc, err := os.ReadFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	for _, tok := range []string{accHeader, accQuery, accBody} {
		if bytes.Contains(enc, []byte(tok)) {
			t.Fatalf("the encrypted backup holds %s in clear", tok)
		}
	}
	plain := filepath.Join(t.TempDir(), "restored.db")
	if err := security.DecryptFileWithMasterKey(backup, plain, flows.PurgeBackupLabel, accKey); err != nil {
		t.Fatal(err)
	}
	restored, err := sql.Open("sqlite", plain)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var integrity string
	if err := restored.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("the decrypted backup does not open cleanly: %q, %v", integrity, err)
	}
	if n := countWhere(t, restored, `SELECT COUNT(*) FROM flow_versions`); n != 3 {
		t.Fatalf("the backup holds %d versions, want all 3", n)
	}
	if n := countWhere(t, restored, `SELECT COUNT(*) FROM flow_versions WHERE instr(json, ?) > 0`, accHeader); n == 0 {
		t.Fatal("the backup lost the old header token: it is not a real backup")
	}

	// 7. The waiting run ends: revision 2 is deleted at once.
	if err := s.Flows.Store().FinishRun(ctx, "run_wait", flows.RunResult{Status: flows.RunSuccess, FinishedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if versions, err = s.Flows.ListVersions(ctx, id); err != nil || len(versions) != 1 || versions[0].Revision != 3 {
		t.Fatalf("versions after the run ended = %+v, %v, want revision 3 only", versions, err)
	}

	// 6. A byte search of the data directory (flows.db, its WAL and SHM, every other file): the header and
	// query tokens are nowhere; the body token, which cannot move, is only in flows.db or its WAL.
	_ = raw.Close()
	held := filesHolding(t, dataDir, accHeader, accQuery, accBody)
	if len(held[accHeader]) != 0 || len(held[accQuery]) != 0 {
		t.Fatalf("plaintext tokens left in %v and %v", held[accHeader], held[accQuery])
	}
	for _, file := range held[accBody] {
		if file != "flows.db" && file != "flows.db-wal" {
			t.Fatalf("the body token is in %s, want only flows.db or its WAL (the flow was not moved)", file)
		}
	}
	if len(held[accBody]) == 0 {
		t.Fatal("the body token vanished from the live document: the purge must leave unmoved flows alone")
	}

	// The report agrees: only the unmoved flow is left to the owner.
	report := flowsBody(t, call(http.MethodGet, "/credentials/scan?fresh=1", ""))["report"].(map[string]any)
	if drafts, _ := report["drafts"].([]any); len(drafts) != 1 || report["runs"] != float64(0) || report["versions_purgeable"] != float64(0) || report["versions_pinned"] != float64(0) {
		t.Fatalf("report after the purge = %v", report)
	}
	if snaps, _ := report["snapshots"].([]any); len(snaps) != 0 {
		t.Fatalf("snapshots listed after the purge: %v", snaps)
	}
}
```
Run: `cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && go test ./internal/server -run TestCredentialHygieneAcceptance -count=1 -v 2>&1 | tail -12`. Expected: `--- PASS: TestCredentialHygieneAcceptance`, `ok`. No `GOTMPDIR`.

If it fails, the message says where; the usual causes:
- `move header = 404`: B.3's route or its flow-id pattern differs; read B.3's handler test for the exact request body.
- `purge state = ... failed`: the state's `error` and `failed_step` name the step; fix in B.9, not in the test.
- The byte search finds a token in `flows.db-wal` right after `FinishRun`: `PrunePurgedVersions`'s best-effort checkpoint did not run or the raw test connection still held a read lock; confirm `raw` is closed before the search (it is) and that `PRAGMA wal_checkpoint(TRUNCATE)` returned `busy = 0`.
- The body token missing from `flows.db`: the purge touched drafts or live documents; B.9's `credentialTextSources` must not include `flows` or `flow_versions`.
- A token found in another file (for example a mission file): read its name; if the file is a product write of the move or the purge (an audit line, a log), that is a real leak to fix in B.3 or B.9.

- [ ] **Step 2: Record the purge in the flows notes**

Append to the section `## Credential purge backups` of `internal/flows/AGENTS.md` (Edit, anchored on the sentence "There is no UI for it."):
```
The purge itself (`Service.StartCredentialPurge`, `purge_job.go`, `purge_steps.go`) runs the steps backup, versions, redact, compact, snapshots, verify under `Runner.HoldDispatch`: it refuses while a run is running, new runs queue until it ends, waiting runs do not block it. Versions that a live run, a block pin or a pending gate still needs are marked in `flow_version_purge` (schema 9) and deleted, with `secure_delete` and a WAL truncate, as soon as nothing pins them (`Store.PrunePurgedVersions`: when a run ends, in the hourly retention pass, and by the purge). The scan (`CredentialScan`) and the purge rewrite every plaintext JSON column listed in `credentialTextSources`; drafts and live documents are never touched. Rollback of a marked version answers `FLOW_VERSION_PURGED`; any new code that copies a version into a draft must read it with `Store.GetVersionForCopy`. `flow_runs.doc_hash` is a lookup key into `flow_test_docs` and is never recomputed, so rewriting a stored document leaves it valid.
```

- [ ] **Step 3: Gates, the full Part B test pass and commit**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && go vet ./internal/server && gofmt -l internal/server internal/flows && go test ./internal/server -run 'Flows|Credential' -count=1 2>&1 | tail -3 && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows ./internal/security -count=1 2>&1 | tail -3 && node scripts/test-easydrag-credentials-purge.mjs && node scripts/test-easydrag.mjs | tail -1 && node scripts/test-easydrag-render-baseline.mjs | tail -1
```
Expected: no vet or gofmt output, `ok` for server, flows and security, the three Node lines as before.

Write `C:/disposable/easydrag-s0b/msg-b10.txt`:
```
test(flows): acceptance fixture for the credential hygiene journey

One server test on a real data directory: a flow with a token in a
header, a query parameter and a body field, history rows and a migration
snapshot that hold them, one version pinned by a waiting run; move the
header and the query token, publish again, purge. Nothing but the pinned
version holds the moved tokens until its run ends; then a byte search of
the data directory finds only the unmovable body token in flows.db. The
encrypted backup holds no plaintext and decrypts into a database that
opens. Documents the purge in internal/flows/AGENTS.md.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <your model line>
```
```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-credentials && git add internal/server/flows_credentials_acceptance_test.go internal/flows/AGENTS.md && git commit -F C:/disposable/easydrag-s0b/msg-b10.txt
```

---

### Coverage: spec item to task

| Spec item | Task |
|---|---|
| §6.4 step 2: schema 9, `flow_version_purge`, verified backup, older binary refuses | B.6 |
| §6.4 step 2: marked versions without a pin deleted, pinned ones deleted once unpinned | B.6 (`PrunePurgedVersions`, hooks), B.9 (the step) |
| Rollback and clone refuse a marked version, `FLOW_VERSION_PURGED` | B.6 |
| §6.4 step 1 and §6.5: encrypted safety backup, `EncryptFileWithMasterKey`, `DecryptFileWithMasterKey` | B.7 (helpers), B.9 (the step) |
| §6.5: sweeper at start and daily, 7 days | B.7 |
| §6.5: `--restore-flows-purge-backup`, refuses while AuraGo runs, keeps the current database encrypted | B.7 |
| §6.2: the scan, the cache, `GET /credentials/scan` (admin) | B.8 |
| §6.2: the start-page hint, admin only | B.8 |
| §6.3: the dialog, its texts and the snapshot checkboxes | B.9 |
| §6.4: `POST` and `GET /credentials/purge`, one job, refuses while a run is running, new runs wait | B.9 |
| §6.4 steps 3 to 6: redact in place, compact, snapshots, verify | B.9 |
| §6.4 `doc_hash` check | B.6 preamble (finding), B.9 (`flow_test_docs` rewritten, hash untouched) |
| §7: purge tests, idempotent re-run, sweeper, restore refusal | B.6, B.7, B.9 |
| §7: the acceptance fixture | B.10 |
| §7 client: Node hint test, gated browser test of the purge dialog | B.8, B.9 |
| 16 locales, `FLOW_*` code entries | B.6 (`FLOW_VERSION_PURGED`), B.8, B.9 (`FLOW_PURGE_*`) |
