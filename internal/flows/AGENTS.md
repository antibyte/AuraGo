# internal/flows — EasyDrag flow core

Spec: `docs/superpowers/specs/2026-10-03-easydrag-design.md` (local, git-ignored).

## Boundaries
- Never import `internal/agent`, `internal/server` or `internal/tools`. Tool calls go through
  `ToolInvoker`, LLM calls through `LLMStepper`, time through `Clock` (`services.go`).
- Node types live in a `Registry`. `RegisterLogicNodes` adds the built-in logic nodes; integration
  and trigger nodes are registered by the server wiring.

## Writing a node (NodeDef)
- `Execute` returns an `ExecResult` and a literal `nil`, or a `*NodeError` (`NewNodeError(code, format, …)`).
  Never return a nil `*NodeError` variable as `error`: it is a non-nil interface and fails the node with
  `FLOW_NODE_FAILED` ("the node returned a nil error value").
- Error codes are stable strings the UI translates (the message is the fallback). Reuse the engine's:
  `FLOW_TEMPLATE_ERROR`, `FLOW_NODE_FAILED`, `FLOW_NODE_TIMEOUT`, `FLOW_NODE_PANIC`, `FLOW_OUTPUT_INVALID`,
  `FLOW_OUTPUT_TOO_LARGE`, `FLOW_PORT_INVALID`. A plain error becomes `FLOW_NODE_FAILED`, one wrapping
  `context.DeadlineExceeded` becomes `FLOW_NODE_TIMEOUT`; a `NodeError` with an empty code gets `FLOW_NODE_FAILED`.
- Every error is retried per the node's `Retry` settings, so side effects must tolerate a re-run.
  Honour `ctx`; the timeout applies per attempt. Exception: a failure with a code in `nonRetryableCodes`
  (`engine.go`) is final at once, because a retry cannot fix it and would only pay for the same refusal
  again (an `ai.step` under Retry 5 could make 12 model calls). The set: `FLOW_PARAM_INVALID`,
  `FLOW_CONDITION_INVALID` (the node's own parameters), `FLOW_AI_UNAVAILABLE`, `FLOW_TOOLS_UNAVAILABLE`,
  `FLOW_NODE_UNAVAILABLE`, `FLOW_TOOL_DENIED`, `FLOW_SECRET_UNAVAILABLE` (missing, refused or unusable
  capability, a vault entry included), `FLOW_BUDGET_EXCEEDED`,
  `FLOW_OUTPUT_TOO_LARGE`. Everything else is retried, in particular `FLOW_NODE_FAILED`,
  `FLOW_NODE_TIMEOUT`, `FLOW_TOOL_ERROR` and `FLOW_AI_OUTPUT_INVALID`, where a new attempt can answer
  differently. Choose the code of a new error with that in mind.
- Output must be JSON-encodable (else `FLOW_OUTPUT_INVALID`). `Ports == nil` means the default port;
  every returned port must be declared.
- Read-only contract: resolved params and inputs may alias run data (a single-expression template
  returns the env value itself). Never modify them or anything inside them; clone first, `append`
  included. Engine outputs are normalised through JSON, so stored outputs never alias inputs. A param
  that resolves to null is present with a nil value (defaults do not replace it).
- User data in error and issue text goes through `quoteForError`, `truncateForError` or `echoKey`
  (40 runes), so a message stays short whatever the document holds.
- Hooks (`OutputsFunc`, `OutputFieldsFunc`, `EffectsFunc`, `Validate`, `AvailabilityFunc`) see the RAW
  node: params unresolved (a template is still a string), any type, nil or huge. Keep them pure, cheap,
  deterministic and panic-free; a panic is recovered but still fails the run or is reported as an
  issue. `OutputPorts(nil)` and `FieldsOf(nil)` ignore their hooks. Any new caller of a hook (for example
  a 1b catalog API calling `AvailabilityFunc`) must go through `catchPanic`.
- Set `UntrustedOutput` on a def whose output an attacker can influence (web, mail, webhook, chat) and
  `SensitiveSink` on a `ParamSpec` where such data is dangerous (command, code, script, path, url,
  recipient, device). The lint follows only template refs and `passesInputs`: no flag, no warning.
- Set `OutputIndependent` on a `ParamSpec` that reaches only the node's effect and is never copied into its
  output (the text of a message, the content of a document or file): a ref there does not taint the node's
  output, so "untrusted text goes into a PDF, the PDF's path goes on" is not warned about at every step.
  A param that is also a `SensitiveSink` still warns. Check `Execute` first: a param that picks a name, path
  or address ends up in the output (a file name, `to`) and must not carry the flag. The cap path (a node
  with more than 1000 refs) taints regardless. The flag is not part of the editor's JSON.
- List every outward effect. A new `Effect` constant needs an `effectOrder` entry and a deliberate
  `IsRisky` decision; unknown effects count as not risky.
- Tools: call them only through `callTool` (`catalog_tools.go`). It fails on a cancelled context, rejects raw
  output over 8 MiB (`FLOW_OUTPUT_TOO_LARGE`) before parsing, classifies by the response status first
  (`denied` gives `FLOW_TOOL_DENIED`, `needs_setup` gives `FLOW_NODE_UNAVAILABLE`) and cuts tool text echoed in
  errors to 300 runes. `Args` is read-only and may alias run data.
- `ParseToolOutput` limits: numbers decode as float64 engine-wide, so 64-bit ids must be strings.
- `ParseToolOutput` limits: JSON followed by trailing text is not JSON and falls back to `{"text": …}`.
- `ParseToolOutput` limits: nesting deeper than about 10000 levels falls back to text, and the engine fails
  a node whose output is nested beyond its own depth limit.

## Document and template invariants
- `SchemaVersion` is 1. Limits: `MaxNodes` 500, `MaxEdges` 2000, `MaxDocumentBytes` 2 MiB. `Normalize`
  clamps numbers and coerces unknown enums (kind to flow, concurrency to queue, on_error to stop).
- Node ids match `n_[a-z2-7]{8}`; keys match `[a-z][a-z0-9_]{0,39}` and are not reserved
  (`trigger run flow item index input env vars secrets`). Use `KeyFromLabel` for new keys.
- Templates may only reference upstream nodes (validator rule). A template that is exactly one `{{…}}`
  keeps its JSON type; otherwise it renders text (`Stringify`). Evaluated text is never evaluated again.
- Limits: assembled template text 8 MiB; params nest at most 32 levels; `CollectTemplateRefs` returns at
  most 1000 refs+problems and cuts display paths to 40 runes per segment; `join`, `replace` and `split`
  are capped (8 MiB, 8 MiB, 100000 parts); regex `matches` is capped at 200 bytes and 1000 compiled
  instructions (`maxPatternInsts`).

## Validation and lint
- Structural problems are always errors; publish rules are errors only in `ModePublish` (warnings in
  `ModeDraft`). `NODE_UNREACHABLE`, `TEMPLATE_UNKNOWN_FIELD` and the lint are always warnings. Callers
  save a draft only when `!HasErrors(Validate(…, ModeDraft))`. A cycle is only a draft warning
  (an error when publishing).
- Above `MaxNodes` nodes or `MaxEdges` edges `Validate` returns right after the document checks (one
  `FLOW_TOO_MANY_NODES` error per exceeded limit). Emit issues in document or topological order, never
  by ranging over a map, so the output stays deterministic.
- `finish()` clamps NodeID/EdgeID to 64 runes, Param to 200 and Message to 300, and caps the list at 500
  issues: the cap selects errors first, then the earliest warnings, and keeps the original order. If
  anything was dropped it adds `FLOW_TOO_MANY_ISSUES` on top of the 500, an error whenever a dropped
  issue was an error, so `HasErrors` gating holds.
- Output-port and declared-field lookups are memoized per node; ancestors are computed once per node.
- `Validate` calls definition hooks under a recover: a panic gives one `PARAM_INVALID` issue for the
  node ("the node definition crashed: …"), skips its other hooks and goes on. It is a publish rule (a
  draft warning, so the draft stays saveable). `Validate` also probes `EffectsFunc` (`CollectEffects`
  leaves out a node whose hook panics) and the output ports of every node, leaves and disabled nodes
  included (the engine needs them at run start). The lint calls no hooks.
- `LintUntrustedData`: taint flows through template refs and through `passesInputs` nodes (merge, and
  set with keep_input). Disabled nodes neither warn nor taint; a cyclic flow gets no lint issues. If the
  ref cap is hit while a taint source exists, the node counts as tainted and every sink param is warned.
- A new node whose `Execute` copies `in.Inputs` into its output must be added to `passesInputs`.

## Engine semantics
- One coordinator goroutine per run owns all state; nodes run on workers (at most `parallel`, default 4).
  A node holds its slot until it returns, so a `logic.wait` (up to 1 h) can starve parallel branches.
- Run-start checks: nil flow `FLOW_INVALID`; over the limits `FLOW_TOO_LARGE`; a loop `FLOW_CYCLE`; a
  panicking definition hook `FLOW_NODE_PANIC`. Test runs execute unpublished drafts, so these are the
  guard. Definitions, ports and default ports are computed once per run; a definition is pinned for it.
- Scheduling follows topological order. A node runs when all incoming edges are resolved and at least
  one delivered; otherwise it is skipped and its outgoing edges are skipped (this is how If/Switch and
  Merge work). Disabled nodes and nodes without inputs are skipped with a step record; non-fired
  triggers and nodes outside an `OnlyNode` scope are skipped without one.
- `on_error`: `stop` fails the run, `continue` delivers `{error}` on the first non-error port (`true` for
  if, `case_1` for switch, `default` for a switch without cases), `error_port` delivers `{error}` on
  `error` only. Retries re-run the whole node (not after a `nonRetryableCodes` error); timeouts are per
  attempt. Timeouts and retry delays are clamped (`MaxNodeTimeoutSeconds`, `MaxRetryDelaySeconds`,
  `MaxRunSecondsLimit`).
- Params are resolved on the worker before the node timeout starts and cannot be cancelled (known
  limit: a huge filter chain over a large value can take about a second).
- The first terminal outcome wins: a stop does not override an earlier failure, cancel or timeout. A run
  is `FLOW_CANCELLED` or `FLOW_RUN_TIMEOUT` only if that actually interrupted a node.
- A returned port that the node does not declare, or `error` on success, fails with `FLOW_PORT_INVALID`.
- Stuck nodes are abandoned 30 s after the run context ends (`FLOW_NODE_ABANDONED`); an abandoned worker
  can still cause side effects or read run data later, so treat `RunResult.Outputs` as read-only.
- Nodes still open at the end get a closing step (`closeOpenNodes`): `cancelled`, or `skipped` after a
  stop node. Step status `cancelled` exists in addition to the spec's list.
- Error messages are capped at 1000 runes, node labels in run errors at 80.
- Events: `run_started`, `step_started`, `step_finished`, `run_finished`; `Seq` starts at 1 per run.
- Sizes: a node output over `MaxOutputBytes` (5 MiB) fails (`FLOW_OUTPUT_TOO_LARGE`); all outputs of a
  run together are capped by `MaxRunOutputBytes` (32 MiB, `FLOW_RUN_OUTPUT_TOO_LARGE`). Stored outputs
  and params over `MaxStoredOutputBytes` (256 KiB) become `{"_preview": …}` (first 64 KiB, cut on a
  rune boundary) with `OutputTruncated`/`ParamsTruncated`. The trigger output is bounded as well.

## Runner and event bus
- Test runs ignore per-flow policies but use the global limit. `queue` keeps at most `MaxQueuedPerFlow`
  runs waiting (`RunnerConfig`, default 20), `skip` drops triggers while a live run is admitted or
  queued (no run id), `parallel` only obeys the global limit. The global queue is FIFO.
  `MaxQueuedPerFlow` also caps, separately, the flow's runs waiting for a global slot; `Start` then
  returns `ErrQueueFull`.
- Every run with an id has an event log from its creation (queued runs too), and the log always ends
  with `run_finished`: `endLog` publishes it for runs that end without the engine (cancelled while
  queued, `FLOW_SHUTDOWN`, runner panic). A runner panic gives `FLOW_RUNNER_PANIC` and frees the slot.
- `OnRunStarted` fires right before a run executes; `OnRunFinished` is called for every run, including
  runs cancelled before they started (those never get `OnRunStarted`). Hooks run outside all locks.
- `Start` writes the run row outside `mu`, serialized by `startMu`. A missing run row (flow deleted
  mid-run) is logged at Debug. `Cancel` does not know a run before `Start` returns; after `Shutdown`,
  `IsBusy` can stay true (stale `live` counts).
- `CancelFlow(flowID)` cancels all runs of a flow: running ones through their context, queued and
  waiting ones at once (`FLOW_CANCELLED`, `OnRunFinished` before it returns). It waits for a `Start` in
  progress; a Start that begins later is not affected, so deleting a flow calls it before and after the
  store delete (after it, Start can no longer record a run of the flow).
- A closed subscriber channel means the run finished, the subscriber was dropped for falling more than
  256 events behind, or it was cancelled. A consumer that did not see `run_finished` resubscribes with
  its last `Seq`. Events are shared by all subscribers and the run result: read-only.
- The bus has no goroutine: it sweeps lazily on `Open`. `Rearm` runs at launch. A log still open
  `MaxRunSecondsLimit` + 1 h + retain after the last `Open`/`Rearm` is a leak and is removed.

## Store
- Times are stored as fixed-width UTC text (`timeLayout`) so text order is time order. There is no
  corruption auto-recovery: a failed open is an error, so users' flows never vanish silently.
- Test runs store their document in `flow_runs.doc_json`; live runs reference `flow_versions` (last 50).
- Not-found and exists cases are typed (`ErrFlowExists`, `ErrNotFound`, `ErrRunNotFound`). Inserts and
  upserts of child rows are guarded by `WHERE EXISTS` on the parent row; plain updates by id
  (`SetMissionID`, `SetRunStatus`, `FinishRun`) use the affected row count. A stale draft revision is
  `ErrRevisionConflict`. The schema version is checked on write and the kind is normalised; `SaveDraft`
  needs `f.ID == id`.
- Any read-then-write transaction must start with the write: SQLite does not run the busy handler when a
  read upgrades to a write (see `Publish`, `ReplaceTimers`).
- List queries (`ListRuns`, `LastLiveRuns`) do not load trigger data; only `GetRun` does. Trigger data in
  the run header is bounded like step outputs.
- Timers are settled conditionally on the occurrence that fired (`DeleteTimerAt`, `MoveTimer`).
- Keep migrations idempotent (`CREATE … IF NOT EXISTS`); add columns with `dbutil.MigrateAddColumn`.

## Timers
- Callbacks run synchronously, are panic-recovered (the timer counts as handled), must return quickly
  and must not call `Stop`. Delivery is at least once. Timers over `MissedTimerGrace` (10 min) late at
  start-up go to the `missed` callback.
- A store error makes the loop wait `retryDelay` (30 s) before retrying; `Stop` takes effect between
  timers; `Start` is idempotent and fails after `Stop`.
- Yearly timers keep their anchor day (Feb 29 fires only in leap years). The arithmetic runs in UTC; a
  location is still to be wired in (plan 1b-12).

## Tests
- `go test ./internal/flows/ -count=1`; the race detector needs cgo, so `go test -race ./internal/flows/`
  runs on Linux (aurago-test), not locally on Windows.
- Run timing-sensitive tests alone in a fresh process. On Windows the first local-time format in a
  process costs about 48 ms; gate such tests on a `started` channel, not on a short run timeout.
- `fakeClock` keeps abandoned waiters until `Advance` passes their deadline; `WaitForWaiters` counts them.
