# internal/flows — EasyDrag flow core

Spec: `docs/superpowers/specs/2026-10-03-easydrag-design.md` (local, git-ignored).

## Boundaries
- Never import `internal/agent`, `internal/server` or `internal/tools`. Tool calls go through
  `ToolInvoker`, LLM calls through `LLMStepper`, time through `Clock` (`services.go`).
- Node types live in a `Registry`. `RegisterCatalog(reg, env)` registers the 35 curated types (see Catalog);
  the server wiring (`internal/server`, see Integration) supplies the `CatalogEnv` and the `GenericTool` list for
  `RefreshGenericTools`.

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
- `CancelFlowMode(flowID, mode)` is `CancelFlow` for one mode (same queue, slot and `startMu` handling, counts
  only new cancels). Runs of other modes keep their places; a cancelled waiting run frees its flow slot, which
  admits the flow's next queued run of another mode. Mission Control's cancel uses it with `ModeLive`.
- `CancelRun(runID)` is `Cancel` that also reports whether this call cancelled the run first (`activeRun.cancelled`);
  a caller that records a cancel (the API's audit entry) does so only then. `Store.GetRunHeader` /
  `Service.RunHeader` read a run's header without trigger data and steps.
- A closed subscriber channel means the run finished, the subscriber was dropped for falling more than
  256 events behind, or it was cancelled. A consumer that did not see `run_finished` resubscribes with
  its last `Seq`. Events are shared by all subscribers and the run result: read-only.
- The bus has no goroutine: it sweeps lazily on `Open`. `Rearm` runs at launch. A log still open
  `MaxRunSecondsLimit` + 1 h + retain after the last `Open`/`Rearm` is a leak and is removed.

## Store
- Times are stored as fixed-width UTC text (`timeLayout`) so text order is time order. There is no
  corruption auto-recovery: a failed open is an error, so users' flows never vanish silently.
- Schema version: `flows_meta.schema_version` holds `storeSchemaVersion` (1). `OpenStore` refuses a database
  with a greater version (or one that is not a number) before any migration runs and leaves the file untouched,
  so an older AuraGo never writes into a schema a newer one migrated. Raise `storeSchemaVersion` with every
  migration a previous release cannot work with.
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
- `GetFlowByMission` reads at most two rows (`LIMIT 2`, index `idx_flows_mission`): none, or an empty id, gives
  `ErrNotFound`; two owners give `ErrMissionAmbiguous` (wrapped, bounded id) and no flow, because Mission Control
  would run the wrong one; a ctx error is passed through. `Store.SetMissionID` and `Store.CreateFlow` have no
  uniqueness guard on the mission id. That is a decision: `Service.CreateFlow` creates a new mission for every
  flow, `SetMissionID` has no production caller and the lookup fails closed. A re-link path would need a
  conditional `UPDATE … AND NOT EXISTS(…)` and a matching guard in the insert (or a partial unique index), with
  an `ErrMissionTaken` sentinel.
- Timers are settled conditionally on the occurrence that fired (`DeleteTimerAt`, `MoveTimer`).
- `NextTimerAt(flowID)` is one `MIN(fire_at)` over the flow's rows (primary key `flow_id` prefix, pinned by
  `TestStoreNextTimerAtUsesThePrimaryKey`); rows whose `fire_at` is not in `timeLayout` are left out.
- Keep migrations idempotent (`CREATE … IF NOT EXISTS`); add columns with `dbutil.MigrateAddColumn`.

## Timers
- Callbacks run synchronously, are panic-recovered (the timer counts as handled), must return quickly
  and must not call `Stop`. Delivery is at least once. Timers over `MissedTimerGrace` (10 min) late at
  start-up go to the `missed` callback.
- A store error makes the loop wait `retryDelay` (30 s) before retrying; `Stop` takes effect between
  timers; `Start` is idempotent and fails after `Stop`.
- Yearly timers keep their anchor day (Feb 29 fires only in leap years). The arithmetic runs in the
  zone set by `SetLocation` (UTC when unset; the Service sets `Services.Loc()`); fire times are stored
  as UTC. In the zone a yearly timer keeps its local date and wall-clock time across daylight saving
  changes, a local Feb 29 included, with one limit: a time in the spring-forward gap (02:30 on the
  switch day) becomes 03:30 and stays 03:30 in later years, because the stored time is all the timer
  remembers, until `armTimers` rebinds the flow from its document (Publish, SetEnabled). A time in the autumn
  overlap keeps its wall-clock time; which of the two instants is not defined.

## Catalog
- Counts: `RegisterCatalog(reg, env)` registers 35 curated types: logic 6 (`logic.if`, `switch`, `merge`, `wait`,
  `set`, `stop`), triggers 13, AI 1 (`ai.step`) and actions 15 (web 3, documents 4, notify 4, smart home 2,
  planner 2); `TestRegisterCatalogCounts` pins the total and the trigger count. Generic `tool.<name>` nodes are
  separate: `RefreshGenericTools(reg, tools, env)` replaces all of them in one atomic `Registry.ReplaceWhere`
  step (a concurrent `Lookup` never sees a half-built set) and returns how many it registered. Call it again
  when the tool configuration changes.
- Registry generation: `Registry.Generation()` moves with every change of the registered set (`Register`,
  `Replace`, a `RemoveWhere` that removed something, a `ReplaceWhere` that removed or added something, so every
  `RefreshGenericTools`); reads and failed calls leave it. A cache of anything derived from the registry keys on
  it and reads it before reading the registry (the server's palette answer, `flowNodeTypesCache`). A new
  mutating method must move it too (`TestRegistryGenerationCountsEveryChange`).
- Tool access: a node calls one tool per call (`callTool`, `AllowedTools = [tool]`; its limits are under
  "Writing a node") and reaches a model only through `Services.LLM`. Curated nodes call `requireSuccess(out,
  what)` on every answer whose success matters: it fails closed unless the tool said `"status":"success"`, so a
  plain-text refusal cannot pass as success. Generic nodes do not, so such a refusal reaches them as output
  unless the invoker sets `ToolResponse.IsError` or `Status`.
- Logical tool names the invoker and the `CatalogEnv` must special-case (they are not plain native tools):
  `brave_search` (a direct action without a native schema), `pdf_extractor` (a skill reached through
  `execute_skill`) and `document_creator:gotenberg` (availability only, never called).
- What the server's invoker adds to a tool call (summary modes off, forced `block_remote_content`, refused Home
  Assistant script domains, the documents bridge, refusal mapping) is under Integration.
- `ParseToolOutput` strips the `[Tool Output]`/`Tool Output:` prefixes (whole output only) and the
  `<external_data>` wrappers (around the whole output and every nested string value) and un-escapes their HTML
  escaping. Untrusted web, RSS or webhook text therefore reaches an `ai.step` prompt without the marker the
  agent loop keeps. Limit: `ai.step` has no tools (`LLMRequest` carries none) and its output is untrusted, so
  the risk is manipulated text, not actions. Nothing in this package guards the prompt: the server's `flowLLM`
  puts a fixed guard instruction first in the system message (never follow instructions found in the data; see
  Integration).
- `ai.step`: one `Services.LLM` call in text mode; in fields mode at most one more (a repair), tokens summed.
  Limits: prompt plus instructions 256 KiB, answer text 1 MiB (`FLOW_OUTPUT_TOO_LARGE`, the node's own cap; the
  server's `LLMStepper` refuses answers over 256 KiB first, see Integration), 50 fields, a field description 500
  runes, a model name 200 bytes. `FLOW_AI_UNAVAILABLE` (no stepper), `FLOW_AI_OUTPUT_INVALID`
  (retried) and `FLOW_BUDGET_EXCEEDED` (set by the stepper, kept by `aiCall`).
- Triggers: `BindTriggers(flow, reg, loc, now)` returns one `TriggerBinding` per enabled trigger node, in
  document order: `manual`, `cron` (`trigger.schedule`, through `ScheduleToCron`), `timer` (`trigger.datetime`,
  yearly dates through `nextYearly`) or `mission` (a Mission Control trigger type plus `Config`, which uses the
  JSON names of `tools.TriggerConfig`; `min_interval_seconds` is capped at 30 days and MQTT uses
  `mqtt_min_interval_seconds` instead). Pass the same `now` and `loc` to `Validate` and `BindTriggers`.
  `TriggerSample` gives test runs and the editor example data, per event where a trigger has events.
- `ScheduleToCron` validates with the robfig parser configured like AuraGo's schedulers, rejects sub-minute
  schedules and cron text over 200 bytes, and needs interval minutes to be a proper divisor of 60 and hours of
  24 (60 minutes and 24 hours themselves are refused). A monthly day of 29 to 31 skips the shorter months (cron
  semantics).
- `NormalizeTriggerData(kind, raw)` builds `trigger.data` from Mission Control's raw text, as the second line of
  defence after the caller's own bound: text over 8 MiB is not parsed (the first 64 KiB become `raw`, with
  `truncated: true`, and `payload: nil` for a webhook). A webhook keeps `raw` and the parsed `payload`, so the
  engine (5 MiB trigger output limit) loses bodies above about 2.5 MiB. `trigger.mission_completed` data comes
  from `enqueueCompletionDependentsAtDepthLocked` (`internal/tools/missions_v2_flow_runs.go`): `source_mission`,
  `result`, `chain_depth`, `output` (the source's answer, at most 2000 bytes) and, for a flow source, `outputs`
  (its leaf outputs by key, bounded to 64 KiB of JSON, else `{"_truncated": true, "_preview": <first 4 KiB>}`),
  as the sample promises.
- Taint: untrusted outputs are the triggers webhook, email, mqtt, fritzbox_call, planner and mission_completed;
  the actions web.search, web.read, http.request, file.read, doc.pdf_read and home.assistant; `ai.step` (a
  model that reads untrusted data can be prompt-injected); and every generic node. The other triggers (manual,
  schedule, datetime, ha_state, device, startup, budget) are trusted. Asymmetry: `home.assistant` output is
  untrusted (an entity state can be shaped by mail, RSS or MQTT) while `trigger.ha_state` is pinned trusted by a
  plan test (`catalog_triggers_test.go`), although the same states reach the flow there.
- Sinks are search queries, paths and file names, URLs, headers and bodies, recipients, attachments, accounts,
  channels, MQTT topic and payload, Home Assistant entity and service data, the planner title and the `ai.step`
  instructions (they become the system message; untrusted data belongs in the prompt, which is no sink).
  `OutputIndependent` are the message, the pdf and notify titles (not the planner title, a sink), subject, body,
  document and file content, the MQTT payload and the planner description.
- Secrets: a `secret_ref` param (`http.request` `auth_secret`) holds only the vault key. The node reads the value
  through `Services.Secrets`; the vault value wins over a user header of the same name; `secretScrubber`
  redacts the value (and its JSON, HTML and URL escapes) from outputs and errors. A `SecretReader` must register
  the value with the global output scrubber too. A missing or unusable secret is `FLOW_SECRET_UNAVAILABLE`.
- Generic nodes (`tool.<name>`, category `tool:<category>`):
  - Curated tools are excluded, so a generic node cannot bypass a curated node's strict readers and taint
    flags: `IsGenericToolExcluded` (the `genericExcluded` list and the prefixes `skill__`, `tool__`,
    `game_maker_`, `mcp__`, `package__`) and, at refresh time, the `Tool` of every registered curated def.
    `genericCuratedAllowed` (`filesystem`, `manage_appointments`, `manage_todos`) is the allowed list;
    `TestGenericExclusionCoversCuratedTools` fails for a curated tool in neither.
  - Dropped params: `genericDroppedParams` (`_todo`, `vault_keys`, `credential_ids`, the tool-bridge params,
    `inject_token`, `agent_instruction`, `wake_agent`) and credential params, by exact name (`password`,
    `token`, `api_key`, `secret`, …) or suffix (`_password`, `_token`, `_secret`, `_api_key`, …); there is
    deliberately no `_key` suffix. A value typed into a node sits in clear in the document, its versions and
    the run's step params.
  - Dropped operations (`genericDroppedOperations`): the secret ones (`send_secret` of `invasion_tasks` and
    `set_env` of `netlify` and `vercel`, with their value params) and the ones that spend tokens or money outside
    the flow budget (`smart_file_read` `summarize` with `query`, `go2rtc` `analyze_snapshot` and `three_d_printer`
    `analyze_camera` with `prompt`, `video_download` `transcribe`, `fritzbox_telephony` `transcribe_tam_message`,
    `rtl_sdr` `transcribe` with the `transcribe` switch of record and schedule, `virtual_computers` `run_shell_task`
    and `run_desktop_task` with `instruction`, `knowledge_graph` `optimize` and `optimize_graph`, `invasion_tasks`
    `send_task` (an agent turn on the egg) with `task` and `egg_name`, and `sip_phone` `dial` (the telephone agent)
    with `target`). A tool left without an operation
    gets no node. Tools that spend on every call never reach `RefreshGenericTools`: the server leaves them out
    (`flowSpendingTools` in `internal/server/flows_catalog_env.go`).
  - Sinks: by name for every generic tool (`genericSinkNames` and suffixes: command, code, path, url, to,
    entity_id, topic, headers, title, …), nested (a JSON param whose `properties` or `items.properties` one
    level down hold a sink name), file content names for file tools (`content`, `new_text`, `patches`, …),
    page input of browser and form tools, per-tool content and exposure params (`genericToolContentParams`,
    `genericExposureParams`: port forwards, tunnel port, tailscale routes, network shares), and every text
    param of memory tools and code-running tools.
  - Effects come from the tool, its category and the literal operation (`genericEffects`). An operation that is
    not a literal name (a template, another type) gets the worst case (`genericWorstEffects`): the union over
    the operations the schema lists, or, when it lists none, every effect the tool could have
    (`genericAllEffects`: `runs_code` and `deletes` always, `system_change`, `sends_message`, `writes_files` and
    `controls_devices` by tool or category). `Execute` refuses an operation the schema does not list
    (`FLOW_PARAM_INVALID`), so the effects shown before publishing hold at run time.
  - Labels are literal (no i18n keys), and `CatalogI18nKeys` skips these nodes.
  - Known limit: plaintext secrets nested in JSON params are not dropped (`manage_outgoing_webhooks` `headers`,
    the attribute maps of `ldap`).
- Catalog description: `DescribeNodeTypes(reg, tr)` orders by `CategoryOrder` (generic categories last, then
  alphabetically), then by label. It runs every hook on a fresh sample node (params = defaults) through
  `describeHook` (`catchPanic` plus a Warn log). The fallback per hook: `AvailabilityFunc` panics → blocked;
  `EffectsFunc` panics → risky with no effects; `OutputsFunc` panics → the static ports; `OutputFieldsFunc` and
  the trigger sample panic → none. Availability is normalised to `available`, `needs_setup` or `blocked`
  (any other state is blocked). A `NodeTypeInfo` shares no memory with its def, and its `Effects` and `Risky`
  describe the default params (a generic node shows the worst case): never present "not risky" as a safety
  claim, the publish dialog (`CollectEffects` over the real params) is authoritative; the test dialog adds those effects from `publish-preview` to the catalog's.
- A def with an `OutputsFunc` or `OutputFieldsFunc` needs a `dynamicOutputs` or `dynamicFields` entry that names
  the driving param (`logic.switch` `cases`; `ai.step` `fields`; `logic.merge` `mode`);
  `TestDynamicMarkersCoverTheHooks` enforces it. Merge fields: mode `append` (exact match) gives `items` (list,
  primary) and `count`; any other mode gives none, because the output is then keyed by upstream node key.
- i18n: `CatalogI18nKeys(reg)` lists every key of the catalog and the starter templates: node label, description
  and summary (`easydrag.node.<type, dots as underscores>.label|description|summary`), param label and help,
  option labels, output-field descriptions (collected like `DescribeNodeTypes` does, from a sample node under a
  recover), `easydrag.category.<c>` and the template name, description and texts. Never hard-code display text
  in Go; give the def or param a key (generic nodes are the exception, see above). Error and issue codes
  (`FLOW_*`, `PARAM_*`, `TEMPLATE_*`) are not in the list and need their own UI strings;
  `Availability.Reason` is raw text (English when a hook failed).
- Codes that come with the catalog (each needs a UI string, with the three `ai.step` codes above):
  `FLOW_TOOL_ERROR`, `FLOW_TOOL_DENIED`, `FLOW_NODE_UNAVAILABLE`, `FLOW_TOOLS_UNAVAILABLE`,
  `FLOW_SECRET_UNAVAILABLE`, `FLOW_FILE_EXISTS`, `FLOW_HTTP_STATUS` and `FLOW_NOTIFY_FAILED`. The last two are
  retried: they are not in the non-retryable set under "Writing a node".
- Files travel as `FileRef` objects (`{"$type":"file","path","name","mime","size","web_path"}`); `FilePath`
  accepts such an object or a plain path and reads only a text `path`. `$type` is a path hint, not proof: anyone
  who shapes flow data can write one, so every path-taking param is a `SensitiveSink` and `filePathParam`
  bounds the path (4096 bytes, no NUL, valid UTF-8).
- Retries run non-idempotent effects again:
  - `FLOW_FILE_EXISTS` is retryable. A `file.write` that succeeded but timed out in transit makes the retry hit
    `FLOW_FILE_EXISTS` (`if_exists` `fail`) or write a second copy under the next free name (`unique`, at most
    100 names); `overwrite` repeats safely.
  - `send_email` can fail after the server accepted the mail, so a retry may send it twice. The same holds for a
    Home Assistant service (a toggle, a script), an MQTT delivery (at any QoS; the tool does not deduplicate) and
    a planner add.
- Planner: the title is a sink (one line, at most 500 runes), because `planner.BuildPromptContextText` puts open
  titles into the agent's system prompt. Times go to the tool in UTC and the node output keeps the zone the date
  was written in; the planner itself compares times as text (`GetDueNotifications`,
  `AutoExpireAppointments`), a root cause a separate task has to fix.
- Templates (`TemplateFlow(id, tr)`, listed by `Templates()`): six, all free of lint warnings. Their texts are
  trusted repo translations and may hold expressions (plan 1c's reminder and budget texts read trigger fields).
  The webhook body and the feed titles go through `boundedExpr`, which truncates to `templateUntrustedRunes`
  (40000 runes) so a worst case of four bytes per rune stays under the 256 KiB prompt cap; the search snippets
  of `ai_news_pdf_telegram` are not truncated. `newTemplateInfo(id, texts, categories…)` must declare as many
  `text_N` keys as the builder calls `text(n)` (`TestTemplateKeysMatchTheCatalogList` enforces it). An unknown
  id is `ErrUnknownTemplate` (wrapped, bounded id). Known limit: the PDF title date format `DD.MM.YYYY` is
  hard-coded in Go and ambiguous for English readers.

## Service
- `Service` owns the store, engine, runner and timers and reaches Mission Control only through `MissionBridge`
  (`internal/server` implements it, see Integration). `SaveDraft` validates with draft rules; the store's
  revision check is its only guard (no flow lock).
- `Publish` validates with publish rules (plus the self-trigger rule below), binds the triggers with the same
  `now`, stores the revision, syncs the mission and calls `armTimers`. Timers are armed only while the flow's
  mission is enabled; `SetEnabled(false)` clears them and `SetEnabled(true)` needs a published flow
  (`ErrNotPublished`).
- Lock order: the per-flow lock (`flowLocks`) is the outermost lock. `Publish`, `SetEnabled`, `DeleteFlow`,
  `DeleteFlowForMission`, `MissionEnabledChanged` and `ReconcileMissions` (one flow at a time) hold it. Under
  it the Service calls only the store, the bridge, `TimerService.Replace` and `Runner.CancelFlow`, and none of
  those may take a flow lock. Run paths (starting runs, runner hooks, timer callbacks) never take it;
  `armTimers` requires it to be held.
- Bridge rule: a `MissionBridge` must not synchronously call a Service method that takes a flow lock, for any
  flow (today `Publish`, `SetEnabled`, `DeleteFlow`, `DeleteFlowForMission`, `MissionEnabledChanged` and
  `ReconcileMissions`; the same holds for the optional `MissionReconciler` methods). The Service calls the
  bridge under a flow lock, and runner hooks can run inside such an operation. Lock-free reads (`GetFlow`,
  `ListFlows`, `NextTimer`) and `TriggerFromMission` (also from inside `FlowRunFinished`) may be called
  synchronously and must stay lock-free.
- Any new Service method that reads a flow and then changes its mission or timers takes the flow lock
  (`s.locks.lock`) before it reads the flow and holds it through the timers; a method that must first look
  the flow up (by mission) reads it again once it holds the lock. It switches to `context.WithoutCancel(ctx)`
  right after its first step that cannot be undone, so a caller that goes away cannot leave the store, the
  mission and the timers apart.
- Mission Control helpers (`service_missions.go`): `MissionEnabledChanged` follows that rule and detaches as
  soon as it holds the lock (Mission Control switched the mission before calling it); it runs from
  `go FlowHooks.FlowEnabledChanged`, ignores a mission no flow holds and a flow deleted while it waited.
  `CancelMissionRuns` takes no flow lock: it calls `Runner.CancelFlowMode(flow, ModeLive)` (`ErrNotFound`
  for an unknown mission) and reports the runs that never started to `FlowRunFinished` on the caller's
  goroutine. `NextTimer` is lock-free and reads no document (`flowIDByMission`, `LIMIT 2` on
  `idx_flows_mission`, then `Store.NextTimerAt`; not exactly one flow gives false); the bridge's
  `broadcastMissionState` calls it synchronously.
- Delete order: mission → timers off → `Runner.CancelFlow` → store delete → `CancelFlow` again (it catches a
  `Start` that raced the delete). A failed step leaves the earlier ones done and `DeleteFlow` can be called again
  (deleting a gone mission is not an error). `DeleteFlowForMission` ignores a mission no flow holds and, when
  several flows hold it, returns `ErrMissionAmbiguous` and deletes nothing.
- Heal path: when `Publish` returns a record together with an error, the new revision is live but the mission or
  the timers were not updated. Publishing the same draft revision again repeats the update (the store treats an
  already-live revision as a no-op, then the Service re-syncs and re-arms); any later successful publish heals it
  too. The retry fails validation once a one-off date/time in the draft has passed, until the draft is edited.
- Startup heal: `ReconcileMissions` (`service_reconcile.go`; the server runs it once on a goroutine after `Start`)
  takes each flow's lock in turn (at most `reconcileLockWait` per flow, a busy flow is skipped) and, for a published
  flow, re-syncs the mission from the live revision bound at its `PublishedAt` (skipped when the optional
  `MissionReconciler` bridge extension reports it in sync) and makes the timers follow Mission Control's enabled
  switch and the live revision (clear when disabled; when enabled re-arm only stale timers, `staleTimers`: a
  node or repeat the live revision lacks, or a trigger without its timer; matching timers keep their fire time,
  a due one-off too). It deletes nothing: it warns about a flow without its mission and, with the extension, a
  flow mission without its flow that Mission Control still holds at report time.
  `Publish` never recreates a missing mission (the sync fails after the store published). `Shutdown` waits for a
  reconciliation in progress and ends its lock waits. Limit: `staleTimers` compares node ids and repeat kinds
  only, so a Date/Time node whose date or time changed while its repeat kind stayed the same is not healed;
  after a crash between `store.Publish` and `armTimers` its stale timer fires once at the old time (a yearly
  node: every year) until the next `Publish` or `SetEnabled` of the flow re-arms it.
- Known limits: `Start` itself repairs nothing that a crash cut short, and `Shutdown` does not wait for operations
  in flight (stop the API first, close the store after `Shutdown`). DST: the Service sets
  `TimerService.SetLocation(Services.Loc())`; the spring-gap, Feb 29 and overlap rules are under Timers.
- Self-trigger: a `trigger.mission_completed` on the flow's own mission is refused at publish and in the
  preview (`PARAM_INVALID`, param `source`), because every run would start the next one. Longer loops (flow A ↔
  flow B, a flow and a prompt mission) are stopped at run time by Mission Control: `mission_completed` trigger
  data carries `chain_depth`, and no dependent fires past depth 10 (`maxCompletionChainDepth`,
  `internal/tools`), so such a loop ends after 11 runs and the last mission's `LastOutput` says why. The bridge
  reads the depth from the run record (`tools.CompletionChainDepth`), which keeps trigger data whole up to
  `MaxStoredOutputBytes`; `mission_completed` data stays below 80 KiB (`TestC19bChainDepthSurvivesTheRunRecord`
  in `internal/server` pins that), so lowering that bound below it would reset every chain to depth 1.
- Test runs (`StartTestRun`) execute the draft and are never reported to Mission Control (both run hooks return
  early for `ModeTest`). They validate with draft rules first (errors give a `*ValidationError`, an unknown
  `OnlyNode` gives `IssueNodeNotFound`); remembered sample data over `MaxStoredOutputBytes` is refused with
  `ErrTestDataTooLarge` before anything is stored. Without data the remembered or built-in sample is used.
  `SaveTriggerSample` and `TriggerSampleData` accept only an enabled trigger of the draft (else `ErrNoTrigger`,
  wrapped with the bounded node id), so made-up node ids never get a stored row.
- Live runs (`RunNow`, `TriggerFromMission`, timer callbacks, all through `startLive`) execute the published
  revision (`ErrNotPublished` without one, `ErrNoTrigger` without a matching enabled trigger) and report start and
  finish: `FlowRunStarted` returns the history id, and `FlowRunFinished(RunFinishedInfo)` carries the leaf
  outputs (nodes without successors, by key) of the document the run executed, for dependent
  `mission_completed` triggers. A run is reported even when its flow was deleted meanwhile (the hook remembers
  mission and name), so no history entry stays "running"; a run that never started (cancelled while queued,
  shutdown) is reported with `Started` false. `ErrQueueFull` and `ErrRunnerClosed` from the runner pass through.
- Trigger input from Mission Control is untrusted and bounded: the trigger type is kept only as 1 to 40 characters
  of `[a-z0-9_]` (else `unknown`), the run header and the bridge record keep at most `MaxStoredOutputBytes` of
  trigger data (`{"_preview": …}` beyond), and the engine replaces trigger data that would exceed
  `MaxOutputBytes` with `{}`. The caller bounds the raw text at the source (`NormalizeTriggerData`).

## Tests
- `go test ./internal/flows/ -count=1`; the race detector needs cgo, so `go test -race ./internal/flows/`
  runs on Linux (aurago-test), not locally on Windows.
- Run timing-sensitive tests alone in a fresh process. On Windows the first local-time format in a
  process costs about 48 ms; gate such tests on a `started` channel, not on a short run timeout.
- `fakeClock` keeps abandoned waiters until `Advance` passes their deadline; `WaitForWaiters` counts them.
- Catalog and service tests share fakes (`catalog_helpers_test.go`, `service_helpers_test.go`): extend them only
  additively and give new helpers a task prefix. Locking and cancel tests to repeat under `-race -count=20` on
  aurago-test: `TestServiceSerializesOperationsPerFlow`, `TestServiceConcurrentPublishAndEnable`,
  `TestServiceDeleteCancelsTheFlowsRuns`, `TestRunnerCancelFlow*`, `TestFlowLocksAreKeyedAndCancellable`,
  `TestServiceLifecycle`, `TestServiceMissionEnabledChanged*`, `TestServiceCancelMissionRuns*` and
  `TestServiceNextTimerPerFlow`.

## Integration (plan 1c)
- `internal/server` implements the interfaces of this package; the wiring, the API, the bridge and the
  notifications are in `internal/server/AGENTS.md` ("EasyDrag flows").
  - `ToolInvoker` (`flows_tool_invoker*.go`): one `agent.DispatchToolCallResult` per call with
    `ToolScopeRestricted` and `AllowedTools = {action}` (`execute_skill` for `pdf_extractor`), session
    `flow-<id>`, message source `flow` and the server's regex Guardian, but no LLM Guardian (nodes are written by
    the user, not by a model). A tool outside the node's `AllowedTools` is `denied`, one that is not set up
    `needs_setup`, both without a dispatch. The call runs with a copy of the configuration snapshot whose summary
    modes (`web_scraper`, `ddg_search`, `wikipedia_search`, `pdf_extractor`) are off and whose preferred MCP web
    search is cleared (a summary is a model call outside the flow budget); the message source denies the
    local-Ollama SSRF exception of `api_request`. It always sets `block_remote_content: true` (a JSON bool) for
    `document_creator`, refuses Home Assistant `script`, `shell_command`, `python_script` and `hassio` services
    unless `home_assistant.allowed_services` lists them, registers the call's credentials with the output
    scrubber and redacts them from the output, and never logs arguments (`flowLogHandler`).
  - The invoker's outcome (`flowToolOutcome`) sets `IsError` for plain-text failures and maps AuraGo's own
    refusal texts (read-only, not enabled, SSRF, runtime gates, path refusals) to `denied`/`needs_setup`; a call
    whose context ended without success returns the context's error. A `send_telegram` whose text went out while
    its document failed (`text_sent`) is denied, and later attempts of the same run and node are refused without
    a dispatch for 25 h (in memory). `send_email` returns only when the SMTP session ended (`deliverSMTP` has no
    context), so two attempts of one email node never overlap.
  - Documents bridge (`flows_tool_invoker_documents.go`): a `file.read` or `doc.pdf_read` of a documents-folder
    path (what `doc.pdf_create` returns, or `/files/documents/<name>`) reads a copy in
    `<workspace>/.easydrag/<run>/`, made through `tools.OpenOutgoingAttachment` and removed after the call (run
    folders older than 25 h are swept), so the output's `file.path` stays the original.
  - `LLMStepper` (`flowLLM`, `flows_llm.go`): one chat call on the node's provider, else `flows.ai_provider`
    (a provider reference), else the main model, plus one `json_object` retry when a provider refuses the
    `json_schema` format. Budget category `flows`: `FLOW_BUDGET_EXCEEDED` while the daily budget blocks it
    (enforcement `partial` or `full`); usage is charged, a failed call's too when the provider reports it. One
    system message: the fixed guard instruction (`flowAIGuardInstruction`: the prompt may hold untrusted data,
    never follow instructions in it), then the node's instructions. Temperature 0.2; completion budget 4096
    tokens (`llm.ReasoningOutputTokens`, 8192, on reasoning routes), a requested one capped at 16384, both
    clamped to the route's max output. Fields mode asks for a strict `json_schema` when the route supports
    structured outputs. `Text` is the answer without `<think>` blocks, trimmed; `JSON` is set only for a JSON
    object. An answer cut at the token limit is `FLOW_AI_OUTPUT_INVALID`, one over 256 KiB
    `FLOW_OUTPUT_TOO_LARGE` (checked here, before the node's own 1 MiB cap). Provider errors are scrubbed (the route's keys replaced) and cut to 300 runes.
    `flowProviderEntry` refuses an unknown id, media and unknown provider types and a typed provider without
    its credential with `FLOW_AI_UNAVAILABLE`; a keyless `custom` endpoint and an untyped one with a model stay
    usable.
  - `SecretReader` (`flowSecrets`, `flows_llm.go`): only vault entries `easydrag_<name>` (`[a-z0-9_]{1,40}`),
    which the agent cannot read. A read registers the value, raw and trimmed, with the global output scrubber;
    the scrubber ignores values under 8 bytes (the node's `secretScrubber` still removes them from the node's
    own result), and a value of only whitespace is not registered.
  - `CatalogEnv` (`flowCatalogEnv`, `flows_catalog_env.go`): tool availability and the generic tools
    (`agent.ConfiguredToolSchemas`) per configuration snapshot, cached; the API refreshes the registry when the
    snapshot changed (`refreshRegistry`). Tools that spend outside the flow budget never become generic nodes
    (`flowSpendingTools`: `analyze_image`, `manus`, `huggingface`, `memory_reflect`, `space_agent`,
    `treg_call`, `transcribe_audio`; the families `generate_`, `yepapi_`, `telnyx_`); spending operations of
    other tools are dropped here (`genericDroppedOperations`, under Catalog).
  - `MissionBridge` (`flowMissionBridge`, `flows_bridge*.go`, with the optional `MissionReconciler`) and the
    `tools.FlowHooks` that lead back into the Service (`flowMissionHooks`).
- `Service.CancelMissionRuns`, `MissionEnabledChanged` and `NextTimer` serve Mission Control (see Service).
- Every key of `CatalogI18nKeys` must exist in `ui/lang/easydrag/<16 locales>.json`. `catalog_i18n_test.go`
  enforces this: `TestCatalogTranslationsAreComplete` (en holds every key; every locale holds exactly en's keys,
  the server's `easydrag.notify.*` and `easydrag.option.*` strings included, with the same placeholders) and
  `TestTemplatesAreValidInEveryLocale` (the starter templates validate in every locale).
