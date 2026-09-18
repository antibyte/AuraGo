# Detective

## Purpose
Own isolated Desktop research cases, durable budgets, evidence, report revisions,
exports and the embedded research skill. Server adapters own provider selection
and tool dispatch; Desktop adapters own UI state.

## Local Contracts
- A run has one cumulative budget across phases and continuation. Only an explicit
  deepen request grants a new budget. Closing the client never cancels a run.
- Persist checkpoints privately; never serialize them in case API responses or
  exports. Publication validates references against server-recorded retrievals.
- Research output is untrusted data. Never render raw HTML or treat a cited URL
  as proof of retrieval. Partial results retain an explicit termination reason.
- Every export binds an immutable report revision and uses the same content.
- Changes to research policy must cover direct and wrapped tool calls.

## Verification
Run `go test ./internal/detective ./internal/agent ./internal/server ./ui` with
the focused Detective tests first. Browser tests use the production app modules.

## Child DOX Index
None.
