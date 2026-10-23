# Detective

## Purpose
Own isolated Desktop research cases, durable budgets, evidence, report revisions,
exports and the embedded research skill. Server adapters own provider selection
and tool dispatch; Desktop adapters own UI state.

## Local Contracts
- A run has one cumulative budget across phases and continuation. Continue keeps
  the recorded profile and active time and ignores a posted effort. Only an
  explicit deepen request or a new scheduled update grants a new budget, and
  deepen on a draft returns `ErrConflict`. Closing the client never cancels a run.
- A finding quote needs at least 8 Unicode code points after trim and must be an
  exact substring of a source recorded as read.
- Live status and existence checks do not load the evidence body. `Live` and
  `ListSummaries` read `detective_summaries`, which `saveCaseTx` writes from
  `caseSummary`; a missing row (a case written before version 4) is projected from
  the body once. The summary has its own table and the case row keeps `body` as its
  last column, because SQLite walks the overflow chain of a large value for every
  column stored after it. The server prompt copy includes the bounded working
  findings, with text limited to 500 runes and quotes to 240. Update prompts include
  a bounded report preview and recent archived findings. Case-scoped `history`
  retrieves older evidence.
- `detective_cases.active_ms` wins over the JSON value when the column is greater.
  Restart selects running/queued cases from the `status` column (never by decoding
  every body) and loads each one with `getLocked` before saving `interrupted` /
  `server_restart`. Schema version 4 migrates the source/finding/report archives
  (version 3), rebuilds the case row with the body last and adds the summary table,
  in one transaction after a synced `.pre-v4-<timestamp>.bak` SQLite snapshot.
  Failed backup/migration aborts opening; repeat opens do not migrate or back up
  again.
- A run whose terminal state cannot be persisted leaves the case queued and returns
  the error; the worker waits for the next tick instead of retrying the same row in
  a loop, and warns every third failed attempt. `Options.Logger` is optional.
- Persist checkpoints privately; never serialize them in case API responses or
  exports. Publication validates references against server-recorded retrievals.
- Research output is untrusted data. Never render raw HTML or treat a cited URL
  as proof of retrieval. Partial results retain an explicit termination reason.
- Every export binds an immutable report revision and uses the same content.
- The server scheduler shares the single research queue. An enabled case starts
  only after a completed baseline report; each completion schedules from now,
  with at most one overdue catchup. Interrupted automatic runs keep their ID,
  profile, counters and private continuation; fresh updates clear continuation.
  Update effort and report style are snapshotted per run. Questions pause until
  answered. Stop/disable cancels automatic work and pauses future scheduling.
- A revoked permission never starts a queued case: the worker leaves it queued and
  retries on the next tick. A run that loses its rights mid-flight returns
  `ErrPermission` and is stored as `interrupted` / `permission_revoked`, so it stays
  resumable; an automatic run records `permission_blocked` and reschedules without
  publishing a revision.
- The working set keeps source metadata only; `Source.Excerpt` is empty in the body
  and lives in `detective_sources` (the tool receipt and `GET .../source?id=` still
  carry it). Findings keep their text in the body for the prompt, while every report
  revision snapshots them in full. A case response therefore carries no evidence
  text: `detectiveCaseView` blanks excerpts and drops the case-level findings.
- Automatic failures never publish partial replacement reports. `no_changes`
  requires a successful retrieval and no pending new finding; it changes neither
  revision nor report. Publication, result and next due time commit together.
  Deduplicate sources by retrieved content/locator and findings by normalized
  text. Previously unpublished findings need a new retrieval before publication.
- Append revisions contain immutable new blocks and a decreasing base revision;
  rewrites stand alone. Archives have no lifetime count cap. Keep bounded working
  sets, case-scoped pages (default 20, maximum 50), and report metadata in lists.
  Exports resolve at most 4096 segments/16 MiB and produce at most 32 MiB; preserve
  legacy cached exports exactly. Old archive content is never auto-deleted.
- Changes to research policy must cover direct and wrapped tool calls.

## Desktop invocation ownership

- Server research runners retain Desktop admission through provider work and
  report/evidence publication. Readonly revokes active provider requests; Stop
  remains available. Finish still produces a report and is a write operation.

## Verification
Run `go test ./internal/detective ./internal/agent ./internal/server ./ui` with
the focused Detective tests first. Browser tests use the production app modules.

## Child DOX Index
None.
