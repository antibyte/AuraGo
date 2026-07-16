# Runtime Reliability Fixes Design

**Date:** 2026-07-16

## Summary

AuraGo currently shows three independent reliability problems in the supplied runtime log:

1. Virtual-computer machine polling opens a new SQLite ledger for every refresh. Each open runs the full migration transaction, which competes with the persistent virtual-computer task manager and can produce `SQLITE_BUSY` while synchronizing machines or finishing a task.
2. The promptsec LLM judge can reach its deadline when the configured Guardian provider is slow or unavailable. The timeout is an expected degraded state for availability-oriented fail-safe modes, but it currently produces a noisy transport-error and Guardian-warning cascade.
3. Short German post-tool completion messages are not recognized as completion evidence. The agent therefore asks the model twice for an explicit `<done/>`, discards the useful answer, and eventually skips history persistence for the empty completion marker.

The fix will address all three causes without changing user-facing configuration, relaxing an explicitly configured blocking fail-safe, or weakening the detector for unfinished action promises.

## Goals

- Keep machine polling and agent-task persistence reliable under concurrent access.
- Run the virtual-computers migration once for the shared runtime ledger and avoid write transactions when the schema is already current.
- Preserve availability when the promptsec LLM judge times out under `allow` or `quarantine` fail-safe behavior.
- Preserve `block` behavior when the operator explicitly configured it.
- Treat expected Guardian deadlines as one controlled degraded outcome instead of an error cascade.
- Recognize conservative German completion evidence after a tool call and persist the first useful final response.
- Add regression tests that reproduce the exact failures from the supplied log.

## Non-goals

- Changing the machine polling interval or removing automatic machine refresh.
- Introducing a new database, schema version, background write queue, or configuration field.
- Making every short post-tool text response an implicit completion.
- Suppressing arbitrary Guardian provider failures or changing the main LLM retry policy.
- Changing promptsec's general `FailClosed` policy independently of AuraGo's configured LLM Guardian fail-safe.
- Adding UI elements or translation keys.

## Considered Approaches

### 1. SQLite timeout only

Apply WAL and `busy_timeout` to every independently opened ledger while otherwise preserving the per-request lifecycle.

This is a small change and would reduce transient lock failures. It does not remove repeated connection creation or the unnecessary migration write transaction on every five-second poll, so it treats contention rather than its cause.

### 2. Shared ledger with bounded SQLite waiting

Open one virtual-computers ledger during application startup, share it between the HTTP server and task manager, and close it after the task manager shuts down. Use the repository's standard SQLite pragmas and return early from migration when the current schema is already installed.

This directly removes the competing in-process database handles from the polling path. WAL and a bounded busy timeout remain useful defensive measures for external access and short write overlap. This is the selected approach.

### 3. Serialized asynchronous write queue

Route all machine synchronization and task updates through a dedicated writer goroutine.

This would serialize writes but introduces queue lifecycle, delivery, shutdown, and error-reporting semantics. It would also delay status visibility. The added complexity is not justified for one process and one small ledger.

## Design

### Shared virtual-computers ledger

The main process will open the virtual-computers ledger once. That ledger will be passed to both the virtual-computers task manager and the HTTP server through explicit constructors/startup dependencies.

Ownership will remain unambiguous:

- `OpenTaskManager(path, ...)` remains available for isolated callers and tests and owns the ledger it opens.
- A new constructor accepts an existing ledger and does not close it.
- The main process owns the shared ledger.
- Shutdown first stops and waits for the task manager, then closes the shared ledger.

The server stores the shared ledger in its runtime dependencies. Virtual-computer handlers use that instance and no longer open and close the database during each request. Tests that construct a server directly will provide a temporary ledger when ledger-backed behavior is under test.

### SQLite initialization and migration

The virtual-computers ledger will use the existing `internal/dbutil` opening policy so its connection has the repository-standard settings, including:

- WAL journal mode;
- `synchronous=NORMAL`;
- foreign-key enforcement;
- a bounded busy timeout;
- one open connection per shared database handle.

Migration will inspect the installed schema version before beginning a write transaction. When version 2 is already present, migration returns immediately. New databases and version-1 databases retain the current creation, backup, and migration behavior.

The busy timeout is defensive rather than the primary synchronization mechanism. Normal production access is serialized through the one shared handle.

### Guardian timeout degradation

The promptsec judge keeps its existing configured timeout. When its context deadline expires:

- AuraGo records the Guardian check as a degraded/failed evaluation;
- `fail_safe: allow` maps to an allowed judge result;
- `fail_safe: quarantine` maps to an unknown judge result and does not block the promptsec pipeline;
- `fail_safe: block` maps to an unsafe result and remains blocking;
- expected deadline cancellation is logged once at warning level with the Guardian operation and elapsed time;
- the underlying LLM transport does not emit an additional error-level entry for that explicitly tagged, expected Guardian deadline.

Non-deadline transport failures remain visible as errors or warnings according to the existing transport and Guardian behavior. The context marker used for log classification is internal and carries no prompt or credential data.

### Post-tool completion detection

Completion evidence will be extended with conservative German result phrases used by actual AuraGo responses. The matcher will recognize bounded phrases such as:

- `Status: stabil` and equivalent successful status values;
- `... ist abgeschlossen`;
- `keine weitere Aktion erforderlich`;
- `keine Benachrichtigung nötig`;
- explicit German result counts where they unambiguously describe completed work.

The phrases are evidence only in the existing post-tool completion path. They do not bypass these protections:

- responses containing a tool call continue through tool execution;
- questions still return control to the user;
- action promises without results continue to trigger recovery;
- announcement-only structures continue to be rejected;
- generic short text without completion evidence still requires a tool call or `<done/>`.

When implicit completion is accepted, the useful response text from that same model turn is returned and persisted. AuraGo will not request a standalone `<done/>`, so history persistence will not receive an empty final response.

## Data and Control Flow

### Machine refresh

1. The desktop polls `GET /api/virtual-computers/machines`.
2. The handler fetches the current machine list from boringd.
3. The handler synchronizes the list through the server's shared ledger.
4. If an agent task writes an event at the same time, both operations queue on the shared connection instead of competing through independent SQLite handles.
5. The handler returns the boringd machine list even if the optional ledger synchronization fails, preserving the current API behavior.

### Guardian evaluation

1. promptsec invokes the configured AuraGo LLM judge with its bounded context.
2. The LLM Guardian calls the configured security provider.
3. A deadline produces the configured AuraGo fail-safe result.
4. The result is mapped to safe, unknown, or unsafe without a second error cascade.
5. promptsec continues or blocks according to that mapped result.

### Agent completion

1. A tool returns successfully.
2. The model sends a short German status summary without `<done/>`.
3. Existing structural and action-promise checks run.
4. The completion-evidence matcher recognizes the result phrase.
5. The response is treated as implicit completion and persisted unchanged.

## Error Handling

- Ledger initialization failure keeps the current startup warning and disables ledger-backed task history rather than crashing unrelated AuraGo features.
- A runtime ledger synchronization failure remains a warning, but migration-related lock failures should no longer occur during normal polling.
- Task terminal-state persistence continues to report a failure if SQLite remains unavailable beyond the bounded busy timeout.
- Guardian timeouts are observable through Guardian metrics and one concise degraded-evaluation log entry.
- Explicit Guardian blocking configuration is never downgraded to availability behavior.
- Completion phrases remain deliberately narrow to avoid silently accepting unfinished work.

## Testing

### Virtual computers

- Verify a current version-2 ledger can be reopened without executing a schema-version write.
- Run machine synchronization concurrently with task event and terminal-status writes and assert that neither returns `SQLITE_BUSY`.
- Verify a task manager created with a shared ledger does not close it.
- Verify an independently opened task manager still owns and closes its ledger.
- Verify server machine polling reuses the supplied ledger.

### Guardian

- Use a blocking fake Guardian transport and assert the deadline maps to `allow`, `unknown`, or `unsafe` according to each fail-safe mode.
- Assert an expected Guardian deadline generates one degraded log path and no transport error-level duplicate.
- Assert non-timeout transport errors retain their existing visibility.
- Assert no credentials, headers, or prompt bodies are added to the new context marker or logs.

### Completion recovery

- Use the exact 199-character `Status: stabil` response from the supplied log and assert immediate implicit completion with preserved text.
- Use the exact `Die Statusprüfung ist abgeschlossen` response and assert the same behavior.
- Verify German action promises without results still request recovery.
- Verify unrelated short German text still requires completion evidence.
- Verify history persistence receives the useful response rather than an empty `<done/>` turn.

### Regression verification

Run focused tests for `internal/virtualcomputers`, `internal/server`, `internal/security`, and `internal/agent`, followed by `go test ./...`. Run the repository's applicable UI checks only if shared assets are touched; this design does not require UI changes.

Before editing each existing symbol, run GitNexus upstream impact analysis and warn before proceeding if any change is HIGH or CRITICAL risk. Before the implementation commit, run `detect_changes` against `main` and stage only the intended feature files.

## Security and Compatibility

- No database schema, API, configuration, Vault, or network protocol changes are introduced.
- The boringd token and all provider credentials remain server-side.
- Availability behavior applies only when `allow` or `quarantine` is configured; `block` remains fail-closed.
- Existing `OpenTaskManager` callers remain source-compatible.
- Machine-list responses and polling behavior remain backward compatible.
