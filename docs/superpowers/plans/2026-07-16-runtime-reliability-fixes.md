# Runtime Reliability Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eliminate virtual-computer SQLite lock contention, degrade Guardian timeouts without an error cascade, and accept valid German post-tool completion summaries.

**Architecture:** AuraGo will share one explicitly owned virtual-computers ledger between the HTTP server and task manager, with repository-standard SQLite pragmas and a migration fast path. Guardian requests will carry an internal expected-deadline marker that only changes transport log severity, while existing fail-safe decisions remain authoritative. Completion recovery will add narrowly bounded German evidence patterns without weakening action-promise detection.

**Tech Stack:** Go 1.26.1+, modernc.org/sqlite, `database/sql`, `log/slog`, standard `context`, existing promptsec and OpenAI-compatible client, table-driven Go tests.

## Global Constraints

- No database schema, API, configuration, Vault, or network protocol changes.
- `fail_safe: allow` and `fail_safe: quarantine` preserve availability; `fail_safe: block` remains blocking.
- Existing `OpenTaskManager` callers remain source-compatible.
- Machine-list responses and the five-second polling behavior remain backward compatible.
- Do not add UI assets or translation keys.
- Before editing every existing symbol, run GitNexus upstream impact analysis; warn and stop for user confirmation on HIGH or CRITICAL risk.
- Use test-driven development: observe each focused regression test fail before implementing its production change.
- Before each commit, run `detect_changes --scope compare --base-ref main` and stage only files belonging to that task.

## File Map

- `internal/virtualcomputers/ledger.go`: standard SQLite opening and current-schema migration fast path.
- `internal/virtualcomputers/ledger_test.go`: migration and concurrent-write regressions.
- `internal/virtualcomputers/task_manager.go`: shared-ledger constructor and explicit ownership.
- `internal/virtualcomputers/task_manager_test.go`: shared-ledger lifetime regression.
- `internal/server/server.go`: server dependency for the shared ledger.
- `internal/server/virtual_computers_handlers.go`: reuse the server ledger and close only fallback ledgers.
- `internal/server/virtual_computers_handlers_test.go`: server-ledger reuse regression.
- `cmd/aurago/main.go`: startup and shutdown ownership wiring.
- `internal/llm/transport_log.go`: expected-deadline context marker and debug-level transport completion.
- `internal/llm/transport_log_test.go`: deadline logging regression.
- `internal/security/llm_guardian.go`: mark bounded Guardian evaluations and preserve fail-safe mapping.
- `internal/security/llm_guardian_test.go`: promptsec judge timeout mapping regression.
- `internal/agent/agent_loop_announcements.go`: conservative German completion evidence.
- `internal/agent/agent_loop_announcements_test.go`: exact log-response and false-positive regressions.

---

### Task 1: Serialize Virtual-Computer Ledger Access

**Files:**
- Modify: `internal/virtualcomputers/ledger.go:1-210`
- Modify: `internal/virtualcomputers/ledger_test.go`
- Modify: `internal/virtualcomputers/task_manager.go:20-95,300-330`
- Modify: `internal/virtualcomputers/task_manager_test.go`
- Modify: `internal/server/server.go:110-230,250-285,1083-1190`
- Modify: `internal/server/virtual_computers_handlers.go:1262-1289`
- Modify: `internal/server/virtual_computers_handlers_test.go`
- Modify: `cmd/aurago/main.go:1079-1115`

**Interfaces:**
- Consumes: `dbutil.Open(path string, opts ...dbutil.Option) (*sql.DB, error)`.
- Produces: `virtualcomputers.NewTaskManager(ledger *Ledger, logger *slog.Logger, opts TaskManagerOptions) (*TaskManager, error)`.
- Produces: `server.StartOptions.VirtualComputersLedger *virtualcomputers.Ledger` and `server.Server.VirtualComputersLedger *virtualcomputers.Ledger`.
- Preserves: `virtualcomputers.OpenTaskManager(path string, logger *slog.Logger, opts TaskManagerOptions) (*TaskManager, error)`.

- [ ] **Step 1: Run GitNexus impact analysis for every production symbol in this task**

Run:

```powershell
node .gitnexus/run.cjs impact OpenLedger --direction upstream --repo AuraGo
node .gitnexus/run.cjs impact Migrate --direction upstream --repo AuraGo
node .gitnexus/run.cjs impact OpenTaskManager --direction upstream --repo AuraGo
node .gitnexus/run.cjs impact Close --direction upstream --repo AuraGo
node .gitnexus/run.cjs impact virtualComputersLedger --direction upstream --repo AuraGo
node .gitnexus/run.cjs impact virtualComputersWithLedger --direction upstream --repo AuraGo
node .gitnexus/run.cjs impact newServerFromOptions --direction upstream --repo AuraGo
node .gitnexus/run.cjs impact Start --direction upstream --repo AuraGo
```

Expected: impact reports are reviewed; no HIGH or CRITICAL result is modified without warning the user first.

- [ ] **Step 2: Add failing ledger and ownership regressions**

Add these tests, using the existing package imports and helpers:

```go
func TestOpenLedgerSkipsMigrationWriteForCurrentSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "virtual_computers.db")
	ledger, err := OpenLedger(path)
	if err != nil {
		t.Fatalf("OpenLedger: %v", err)
	}
	if _, err := ledger.db.Exec(`CREATE TRIGGER reject_schema_version_update
		BEFORE UPDATE ON schema_meta BEGIN SELECT RAISE(ABORT, 'schema version rewrite'); END`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatalf("close ledger: %v", err)
	}

	reopened, err := OpenLedger(path)
	if err != nil {
		t.Fatalf("reopen current ledger: %v", err)
	}
	defer reopened.Close()
}

func TestOpenLedgersWaitForConcurrentWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "virtual_computers.db")
	first, err := OpenLedger(path)
	if err != nil {
		t.Fatalf("OpenLedger first: %v", err)
	}
	defer first.Close()
	second, err := OpenLedger(path)
	if err != nil {
		t.Fatalf("OpenLedger second: %v", err)
	}
	defer second.Close()

	tx, err := first.db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	if _, err := tx.Exec(`INSERT INTO machines(id, updated_at) VALUES ('held-writer', '2026-07-16T00:00:00Z')`); err != nil {
		t.Fatalf("hold write lock: %v", err)
	}
	result := make(chan error, 1)
	go func() {
		result <- second.UpsertMachine(context.Background(), Machine{ID: "queued-writer"})
	}()
	select {
	case err := <-result:
		t.Fatalf("second writer returned before lock release: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := <-result; err != nil {
		t.Fatalf("queued writer: %v", err)
	}
}

func TestTaskManagerWithSharedLedgerDoesNotCloseLedger(t *testing.T) {
	ledger, err := OpenLedger(filepath.Join(t.TempDir(), "virtual_computers.db"))
	if err != nil {
		t.Fatalf("OpenLedger: %v", err)
	}
	defer ledger.Close()
	mgr, err := NewTaskManager(ledger, slog.Default(), TaskManagerOptions{})
	if err != nil {
		t.Fatalf("NewTaskManager: %v", err)
	}
	if err := mgr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := ledger.UpsertMachine(context.Background(), Machine{ID: "vm-after-close"}); err != nil {
		t.Fatalf("shared ledger was closed: %v", err)
	}
}
```

In `internal/server/virtual_computers_handlers_test.go`, add:

```go
func TestVirtualComputersLedgerReusesServerDependency(t *testing.T) {
	ledger, err := virtualcomputers.OpenLedger(filepath.Join(t.TempDir(), "virtual_computers.db"))
	if err != nil {
		t.Fatalf("OpenLedger: %v", err)
	}
	defer ledger.Close()
	s := &Server{Cfg: &config.Config{}, VirtualComputersLedger: ledger}
	got, owned, err := virtualComputersLedger(s)
	if err != nil {
		t.Fatalf("virtualComputersLedger: %v", err)
	}
	if got != ledger || owned {
		t.Fatalf("got=%p owned=%v, want shared %p owned=false", got, owned, ledger)
	}
}
```

- [ ] **Step 3: Run focused tests and verify they fail for the missing behavior**

Run:

```powershell
go test ./internal/virtualcomputers -run 'TestOpenLedgerSkipsMigrationWriteForCurrentSchema|TestOpenLedgersWaitForConcurrentWriter|TestTaskManagerWithSharedLedgerDoesNotCloseLedger' -count=1
go test ./internal/server -run TestVirtualComputersLedgerReusesServerDependency -count=1
```

Expected: FAIL because `NewTaskManager` and the server ledger ownership return do not exist; after compiling the isolated ledger tests, reopening fails with `schema version rewrite` and the second writer returns `SQLITE_BUSY` before the first transaction commits.

- [ ] **Step 4: Implement standard opening, migration fast path, and explicit task-manager ownership**

Replace the raw SQLite open with `dbutil.Open`, add a current-version check before the migration transaction, and split task-manager construction:

```go
func OpenLedger(path string) (*Ledger, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("virtual computers ledger path is empty")
	}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create virtual computers ledger dir: %w", err)
		}
	}
	db, err := dbutil.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open virtual computers ledger: %w", err)
	}
	ledger := &Ledger{db: db, path: path}
	if err := ledger.Migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return ledger, nil
}

func (l *Ledger) currentSchemaInstalled(ctx context.Context) (bool, error) {
	var tableCount int
	if err := l.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_meta'`).Scan(&tableCount); err != nil {
		return false, fmt.Errorf("inspect virtual computers schema metadata: %w", err)
	}
	if tableCount == 0 {
		return false, nil
	}
	var version string
	err := l.db.QueryRowContext(ctx, `SELECT value FROM schema_meta WHERE key = 'schema_version'`).Scan(&version)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read virtual computers schema version: %w", err)
	}
	return version == "2", nil
}
```

At the start of `Migrate`, return when `currentSchemaInstalled` is true. Add `ownsLedger bool` to `TaskManager`; make `OpenTaskManager` call `OpenLedger` and an internal constructor with ownership true; make exported `NewTaskManager` call the same constructor with ownership false. In `Close`, call `m.ledger.Close()` only when `ownsLedger` is true.

- [ ] **Step 5: Wire the shared ledger through server startup and request handlers**

Add the ledger field to `Server` and `StartOptions`, copy it in `newServerFromOptions`, and change the handler helper to return ownership:

```go
func virtualComputersLedger(s *Server) (*virtualcomputers.Ledger, bool, error) {
	if s == nil || s.Cfg == nil {
		return nil, false, nil
	}
	if s.VirtualComputersLedger != nil {
		return s.VirtualComputersLedger, false, nil
	}
	s.CfgMu.RLock()
	path := s.Cfg.SQLite.VirtualComputersPath
	s.CfgMu.RUnlock()
	if strings.TrimSpace(path) == "" {
		return nil, false, nil
	}
	ledger, err := virtualcomputers.OpenLedger(path)
	return ledger, ledger != nil, err
}
```

Update `virtualComputersWithLedger` to defer `Close` only when `owned` is true. In `cmd/aurago/main.go`, open one ledger, build the task manager with `NewTaskManager`, pass it as `StartOptions.VirtualComputersLedger`, and register defers so the manager closes before the ledger.

- [ ] **Step 6: Run focused virtual-computer tests**

Run:

```powershell
go test ./internal/virtualcomputers -count=1
go test ./internal/server -run TestVirtualComputers -count=1
```

Expected: PASS with no `SQLITE_BUSY`, no trigger failure, and no shared-ledger use-after-close.

- [ ] **Step 7: Review scope and commit the virtual-computer fix**

Run:

```powershell
node .gitnexus/run.cjs detect-changes --scope compare --base-ref main --repo AuraGo
git add internal/virtualcomputers/ledger.go internal/virtualcomputers/ledger_test.go internal/virtualcomputers/task_manager.go internal/virtualcomputers/task_manager_test.go internal/server/server.go internal/server/virtual_computers_handlers.go internal/server/virtual_computers_handlers_test.go cmd/aurago/main.go
git diff --cached --check
git commit -m "fix(virtual-computers): serialize ledger access"
```

Expected: only virtual-computer ledger lifecycle symbols and their startup/handler consumers are affected.

---

### Task 2: Degrade Guardian Deadlines Without an Error Cascade

**Files:**
- Modify: `internal/llm/transport_log.go`
- Create: `internal/llm/transport_log_test.go`
- Modify: `internal/security/llm_guardian.go:274-284,330-345,1133-1165`
- Modify: `internal/security/llm_guardian_test.go`

**Interfaces:**
- Produces: `llm.WithExpectedDeadline(ctx context.Context) context.Context`.
- Consumes: existing `LLMGuardian.failSafeResult` mapping for `allow`, `quarantine`, and `block`.

- [ ] **Step 1: Run GitNexus impact analysis for Guardian and transport symbols**

Run:

```powershell
node .gitnexus/run.cjs impact RoundTrip --direction upstream --repo AuraGo
node .gitnexus/run.cjs impact EvaluateWithFailSafe --direction upstream --repo AuraGo
node .gitnexus/run.cjs impact callLLM --direction upstream --repo AuraGo
node .gitnexus/run.cjs impact Judge --direction upstream --repo AuraGo
```

Expected: reports are reviewed; HIGH or CRITICAL risk pauses implementation for a user warning.

- [ ] **Step 2: Add failing expected-deadline logging and fail-safe mapping tests**

Create `internal/llm/transport_log_test.go` with a local `RoundTripper` function adapter and a debug-enabled text logger. Build a request whose marked context is already deadline-exceeded, return `req.Context().Err()` from the base transport, and assert the log contains `roundtrip_done (expected deadline)` but does not contain `level=ERROR`.

Add a table-driven security test that constructs an `LLMGuardian` with an HTTP test server that waits for request cancellation, invokes `Judge` with a short timeout, and asserts the verdict plus exactly one warning containing `LLM check timed out`, the `promptsec_judge` operation, and `latency_ms`:

```go
tests := []struct {
	failSafe string
	want     promptsec.LLMJudgeVerdict
}{
	{failSafe: "allow", want: promptsec.LLMJudgeVerdictSafe},
	{failSafe: "quarantine", want: promptsec.LLMJudgeVerdictUnknown},
	{failSafe: "block", want: promptsec.LLMJudgeVerdictUnsafe},
}
```

- [ ] **Step 3: Run focused tests and verify failure**

Run:

```powershell
go test ./internal/llm -run TestLoggingTransportExpectedDeadlineIsNotError -count=1
go test ./internal/security -run TestLLMGuardianJudgeDeadlineRespectsFailSafe -count=1
```

Expected: FAIL because the context marker does not exist and the transport currently emits an error-level completion.

- [ ] **Step 4: Implement the internal deadline marker and transport classification**

Add an unexported typed context key and exported constructor:

```go
type expectedDeadlineContextKey struct{}

func WithExpectedDeadline(ctx context.Context) context.Context {
	return context.WithValue(ctx, expectedDeadlineContextKey{}, true)
}

func hasExpectedDeadline(ctx context.Context) bool {
	expected, _ := ctx.Value(expectedDeadlineContextKey{}).(bool)
	return expected
}
```

In `loggingTransport.RoundTrip`, before the generic error log, handle only marked deadline errors:

```go
if err != nil && hasExpectedDeadline(req.Context()) && errors.Is(err, context.DeadlineExceeded) {
	t.logger.Debug("[LLM Transport] roundtrip_done (expected deadline)",
		"method", method, "url", url, "elapsed_ms", elapsed.Milliseconds())
	return nil, err
}
```

In `EvaluateWithFailSafe`, mark the context before applying the bounded timeout:

```go
ctx = llm.WithExpectedDeadline(ctx)
ctx, cancel := context.WithTimeout(ctx, timeout)
```

Classify the expected deadline in `callLLM` while leaving other failures on the existing path:

```go
if err != nil {
	if errors.Is(err, context.DeadlineExceeded) {
		g.logger.Warn("[Guardian] LLM check timed out; applying fail-safe",
			"operation", check.Operation,
			"latency_ms", time.Since(start).Milliseconds())
	} else {
		g.logger.Warn("[Guardian] LLM call failed", "error", err, "operation", check.Operation)
	}
	g.Metrics.RecordError()
	return g.failSafeResult(start, fmt.Sprintf("LLM error: %v", err))
}
```

Keep `Judge`'s existing safe/unknown/unsafe mapping unchanged so the configured fail-safe remains authoritative.

- [ ] **Step 5: Run focused Guardian and transport tests**

Run:

```powershell
go test ./internal/llm -run 'TestLoggingTransport' -count=1
go test ./internal/security -run 'TestLLMGuardianJudgeDeadlineRespectsFailSafe|TestFailSafeResult|TestGuardianLLMJudge' -count=1
```

Expected: PASS; expected deadlines have no error-level transport duplicate, and all three fail-safe modes map correctly.

- [ ] **Step 6: Review scope and commit the Guardian fix**

Run:

```powershell
node .gitnexus/run.cjs detect-changes --scope compare --base-ref main --repo AuraGo
git add internal/llm/transport_log.go internal/llm/transport_log_test.go internal/security/llm_guardian.go internal/security/llm_guardian_test.go
git diff --cached --check
git commit -m "fix(guardian): degrade expected judge timeouts"
```

Expected: only LLM transport logging and Guardian evaluation flows are affected.

---

### Task 3: Recognize German Post-Tool Completion Evidence

**Files:**
- Modify: `internal/agent/agent_loop_announcements.go:8-85`
- Modify: `internal/agent/agent_loop_announcements_test.go`

**Interfaces:**
- Preserves: `containsCompletionEvidence(content string) bool`.
- Consumes: existing `handleAgentLoopRecoveries` implicit-completion branch.

- [ ] **Step 1: Run GitNexus impact analysis for completion detection**

Run:

```powershell
node .gitnexus/run.cjs impact containsCompletionEvidence --direction upstream --repo AuraGo
node .gitnexus/run.cjs impact handleAgentLoopRecoveries --direction upstream --repo AuraGo
```

Expected: reports are reviewed; HIGH or CRITICAL risk pauses implementation for a user warning.

- [ ] **Step 2: Add exact failing log-response regressions and false-positive coverage**

Add a helper that creates the same post-tool loop state used by `TestMidTaskSubstantiveTextWithoutDoneSkipsAnnouncementRecovery`. Add this table-driven test:

```go
func TestMidTaskGermanCompletionEvidenceSkipsRecovery(t *testing.T) {
	responses := []string{
		"Status: stabil. Gefunden: 1 offener ToDo (`test`, Priorität medium, keine Dringlichkeitshinweise). Keine weiteren Aufgaben, Missionen oder Termine mit Handlungsbedarf. Keine Benachrichtigung nötig.",
		"Die Statusprüfung ist abgeschlossen. Der letzte Bericht war korrekt — keine weitere Aktion erforderlich.",
	}
	for _, content := range responses {
		t.Run(content[:min(24, len(content))], func(t *testing.T) {
			s := newPostToolRecoveryTestState(t)
			parsed := ParsedToolResponse{Content: content, SanitizedContent: content}
			got, _, shouldContinue, _ := handleAgentLoopRecoveries(s, content, ToolCall{}, parsed, true, emotionBehaviorPolicy{})
			if shouldContinue {
				t.Fatal("completed German status response requested recovery")
			}
			if got != content {
				t.Fatalf("content changed: got %q want %q", got, content)
			}
			if len(s.req.Messages) != 0 {
				t.Fatalf("unexpected recovery messages: %#v", s.req.Messages)
			}
		})
	}
}
```

Add negative cases asserting `containsCompletionEvidence` is false for `"Ich prüfe das jetzt."`, `"Die Prüfung läuft."`, and `"Status: unbekannt."`.

- [ ] **Step 3: Run the focused test and verify recovery still occurs**

Run:

```powershell
go test ./internal/agent -run 'TestMidTaskGermanCompletionEvidenceSkipsRecovery|TestGermanCompletionEvidenceRejectsUnfinishedText' -count=1
```

Expected: FAIL because the supplied German responses are not recognized and recovery messages are appended.

- [ ] **Step 4: Add bounded German completion evidence patterns**

Extend the status pattern with successful German values and add a separate phrase pattern:

```go
var statusEvidencePattern = regexp.MustCompile(`(?i)\b(?:status|exit code|http)\s*[:=]?\s*(?:ok|success|successful|stabil|erfolgreich|unauff[äa]llig|error|failed|200|201|204|400|401|403|404|409|422|429|500)\b`)
var completionPhrasePattern = regexp.MustCompile(`(?i)\b(?:ist\s+abgeschlossen|wurde\s+abgeschlossen|keine\s+(?:weitere\s+)?aktion\s+erforderlich|keine\s+benachrichtigung\s+n[öo]tig)\b`)

func containsCompletionEvidence(content string) bool {
	if strings.ContainsAny(content, "✅✓☑✔") {
		return true
	}
	return resultMetricPattern.MatchString(content) ||
		statusEvidencePattern.MatchString(content) ||
		completionPhrasePattern.MatchString(content)
}
```

Do not add generic matches for `fertig`, `erledigt`, `Prüfung`, or `Status` without a successful value.

- [ ] **Step 5: Run agent recovery regressions**

Run:

```powershell
go test ./internal/agent -run 'TestAnnouncement|TestActionPromise|TestMidTask|TestGermanCompletion' -count=1
```

Expected: PASS; both supplied responses complete immediately, while unfinished promises still recover.

- [ ] **Step 6: Review scope and commit completion detection**

Run:

```powershell
node .gitnexus/run.cjs detect-changes --scope compare --base-ref main --repo AuraGo
git add internal/agent/agent_loop_announcements.go internal/agent/agent_loop_announcements_test.go
git diff --cached --check
git commit -m "fix(agent): recognize German completion summaries"
```

Expected: only announcement/completion recovery flows are affected.

---

### Task 4: Full Verification

**Files:**
- Verify only; no planned production changes.

**Interfaces:**
- Consumes: all three completed task commits.
- Produces: verified repository state ready for handoff.

- [ ] **Step 1: Run focused packages together**

Run:

```powershell
go test ./internal/virtualcomputers ./internal/server ./internal/llm ./internal/security ./internal/agent -count=1
```

Expected: PASS.

- [ ] **Step 2: Run UI checks because server embedding and desktop routes share repository contracts**

Run:

```powershell
npm run check:ui
npm run test:ui-regressions
```

Expected: both commands exit successfully without Virtual Computers or Quick Connect regressions.

- [ ] **Step 3: Run the complete Go suite**

Run:

```powershell
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Run final GitNexus and repository checks**

Run:

```powershell
node .gitnexus/run.cjs detect-changes --scope compare --base-ref main --repo AuraGo
git diff --check
git status --short
```

Expected: GitNexus lists only the intended virtual-computer, Guardian transport, and agent completion flows; `git diff --check` is clean; no unrelated file is staged or modified.
