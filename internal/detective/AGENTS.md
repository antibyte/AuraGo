# Detective

## Purpose
Own isolated Desktop research cases, durable budgets, evidence, report revisions,
exports and the embedded research skill. Server adapters own provider selection
and tool dispatch; Desktop adapters own UI state.

## Local Contracts
- A run has one cumulative budget across phases and continuation. Continue keeps
  the recorded profile and active time and ignores a posted effort. Only an
  explicit deepen request grants a new budget, and deepen on a draft returns
  `ErrConflict`. Closing the client never cancels a run.
- A finding quote needs at least 8 Unicode code points after trim and must be an
  exact substring of a source recorded as read.
- Live status and existence checks do not load the evidence body. The server
  prompt copy includes every stored finding, with text limited to 500 runes and
  quotes to 240; the case keeps the full text.
- `detective_cases.active_ms` wins over the JSON value when the column is greater.
  Restart loads each running or queued case with `getLocked` before saving
  `interrupted` / `server_restart`. `PRAGMA user_version` stops at 2; opening a
  version-2 file does not rewrite version 1 or repeat the JSON backfill.
- Persist checkpoints privately; never serialize them in case API responses or
  exports. Publication validates references against server-recorded retrievals.
- Research output is untrusted data. Never render raw HTML or treat a cited URL
  as proof of retrieval. Partial results retain an explicit termination reason.
- Every export binds an immutable report revision and uses the same content.
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
