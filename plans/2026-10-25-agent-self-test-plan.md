# Prompt-driven agent self-tests

Status: original design, 2026-10-25. The bounded runner, admin Dashboard controls, private reports, reviewed read probes, local fixtures and exact targeted bindings are implemented; current behavior and remaining specialized-fixture limits are documented in the [operator guide](../documentation/agent-self-tests.md). The canonical runtime prompts live in [`prompts/self_tests/catalog.md`](../prompts/self_tests/catalog.md); the companion catalog is the original design snapshot. Statements below about proposed work describe that original scope, not a claim of live acceptance. Documentation is English; executable user-facing prompts are German.

## Goal and recommended approach

AuraGo should discover its actual capabilities, execute one bounded prompt per test, investigate failures with permitted tools, and produce an evidence-backed German report. Cover every enabled tool, integration instance, and supported operation in the inventory, including capabilities that cannot currently be tested. A skipped test is visible missing coverage, never a success.

Use the existing agent runtime, discovery, permission checks, execution hooks, mission lifecycle, and integration probes. Add a small trusted coordinator and a versioned prompt catalog. Each test remains a real prompt executed through AuraGo's normal tool path. The coordinator selects tests, enforces limits, records evidence, validates assertions, and renders the report. An LLM explains observations but cannot certify its own success without execution evidence.

This is preferable to one large “test everything” prompt: individual cases are repeatable, independently cancellable, comparable across versions, and do not lose early results when context fills.

## Current source anchors and gaps

Inspected working-tree source at HEAD `de9a7b42f`; unrelated local changes were present. This is a design-time inventory, not the installed system's enabled configuration. The GitNexus index was seven commits behind and its keyword search partial; current source was used to confirm the relevant contracts.

| Existing component | Reuse | Remaining work |
| --- | --- | --- |
| `internal/agent/native_tools_registry.go`: `builtinToolSchemas`, `ToolNamesFromConfig`; `execution_hooks.go`: `ConfiguredToolSchemas`, `RuntimeToolFeatureFlags` | Canonical schema builders and capability gates | Obtain a runtime-bound inventory with real dependencies, channel, role and scope; config-only schemas are insufficient. |
| `internal/agent/tool_catalog.go`: `BuildToolCatalog`, `ToolCatalogEntry`, `callMethodForEntry` | Native, skill, custom, MCP and Agent Skill entries; active/hidden/disabled/needs_setup | Freeze inventory identity for reporting; recheck live authorization before every call. |
| `prompts/tools_manuals/discover_tools.md` | Category/family pagination, `get_tool_info`, `get_manual`, exact `call_method` | Reconcile every page with the trusted inventory; do not mistake a top search result for completeness. |
| `internal/agent/execution_hooks.go`, `agent.go`: `dispatchInner` | Trusted hooks, canonical leaf calls, tool scope and existing policy enforcement | Add test scope, target/effect limits and evidence observation without replacing handlers or broadening permissions. Rejections before hooks also need records. |
| `internal/agent/tool_call_contract.go`, `tool_execution_policy.go`, `tool_outcome.go`, `action_ledger.go` | Typed outcomes, output references, execution/accounting evidence | Separate test assertions from tool success; capture async terminal receipts, skipped batch calls and unknown outcomes. |
| `internal/server/integration_test_handlers.go`, `server_routes_config.go`; `ui/js/config/integration_actions.js` | Existing saved-config connection probes, including Telegram, Discord, Rocket.Chat, Home Assistant, Proxmox, S3 and Ansible | Share selected probe functions with a narrowly scoped diagnostic entry point. A POST route or “test” label alone does not establish safety. |
| `internal/tools/missions_v2.go`, `internal/server/mission_runs.go`, `mission_history_handlers.go` | Manual/scheduled entry, cancellation, history and parent lifetime | A self-test run/case ledger; mission prose/history alone is not the evidence store. |
| `internal/agent/gog_tools.go`, `layerling_tool.go`, dynamic schema loaders | Account-specific and editor-specific capabilities | Instance expansion; no automatic account switching or guessed editor IDs. |
| `internal/agent/AGENTS.md`: CYD Creator and prepared profiles | Specialized phase/job scopes | Test specialized tools only in the corresponding prepared job, never add them to ordinary chat discovery. |

Static extraction finds 222 distinct literal `tool("...")` names in `native_tools*.go`. This includes compatibility/mega-tool definitions that may be replaced by focused tools, and excludes generated gog names and other registration paths. It is a useful catalog cross-check, **not** a runtime tool count or coverage denominator. The prompt catalog addresses these names plus additional registered, generated and integration-only surfaces. Current runtime discovery remains authoritative.

## Selection and coverage

1. The server takes a sanitized snapshot of enabled integrations and their instances, configured capabilities, actual runtime dependencies, effective agent access, role/channel, selected provider, schema/manual revisions, and build version. Do not read `config.yaml`, `.env`, Vault data or process environments into the model.
2. The agent runs the discovery prompt and follows all cursors. Resolve aliases and wrapper calls to canonical tools. Hidden enabled tools remain in scope; disabled tools are inventory-only and must not be activated.
3. Join the trusted inventory with catalog targets. Separate `enabled`, `callable_now`, `read_only`, allowed operation, required fixture, instance/account, and execution profile. Enabled integrations without an agent tool receive a connection-probe or specialized-profile case, or an explicit access gap.
4. Expand each matching row into concrete cases. One selected tool, valid operation contract, integration instance, approved target and execution profile per case. A lifecycle test may perform setup/readback/cleanup as part of that one behavior. Multiple operations verified by it receive individual assertion records.
5. Enumerate valid operation contracts from the canonical schemas and dispatcher/manual rules. `operation`, `action`, `mode`, `resource`, `sub_operation`, `metric` and `target` are not interchangeable; do not take their Cartesian product. `toolCatalogOperations` is a discovery hint, not a complete operation contract.
6. Every eligible operation gets a curated case or an explicit `NOT_COVERED` record. Generic templates may handle reviewed read operations. An LLM must not infer that an unknown operation is safe from its name. Unknown effects block execution until reviewed.
7. New capabilities, changed schemas, removed aliases, expired fixture grants and changed instance IDs invalidate affected bindings. A new tool can be inventoried immediately without automatically executing it.

Report these denominators separately:

- Enabled integration instances, enabled canonical tools, and enabled valid operation contracts.
- Runnable cases under the selected profile and grants, executed cases, verified passing cases, failing cases, blocked/skipped/uncovered cases.
- Inventory coverage = mapped or explicitly classified enabled units / all enabled units.
- Execution coverage = distinct enabled units with actual execution evidence / all enabled units.
- Runnable execution coverage = executed runnable cases / selected runnable cases; show this beside total coverage, never instead of it.
- Functional assertion coverage = verified operation assertions / applicable operation assertions. A connection check does not cover every operation of that integration. Duplicate wrappers and aliases do not inflate counts.

## Execution profiles and grants

| Profile/tag | Automatic scope after the user starts that profile | Examples |
| --- | --- | --- |
| `R` — inspect | Reviewed bounded reads, credential checks and local calculations; respect data-minimization and egress policy | Status, list, schema, synthetic search, `SELECT 1` on an allowed connection |
| `W` — fixture | Reversible operations on explicitly assigned disposable resources with verified cleanup support | Scratch file, test workbook, dedicated S3 prefix, isolated test database |
| `X` — explicit effect | Only the exact approved target/action, time window, count and budget | Sending a message, audio playback, device switching, telephony, deployments, purchases or billable generation |
| `H` — human observation | Interactive prompts, visual/audio judgments or hardware behavior without trusted telemetry | Confirmation dialog, physical light, audible quality, live desktop input |
| `L` — isolated lab | Fault injection, negative authorization, cancellation races and destructive operations in a disposable environment | Permission revocation, unavailable dependency, synthetic 429, denied path traversal |

These are selection profiles, not a permission hierarchy. Each bound case must additionally declare its actual effect class (`read`, `fixture_write` or `external_effect`). `H` and `L` add human/laboratory prerequisites and never waive the grant for that effect. Account identity, private reads, network discovery and paid reads still require the appropriate target/egress/budget grants. Existing capability, read-only, role, action-policy and operation gates always apply. Self-test mode can restrict them, never widen them.

Starting a read-only run is sufficient authorization for its reviewed reads; do not ask for every probe. Collect missing fixture/effect grants in one preparation screen. Unattended runs skip missing grants and report them. The server owns approvals and immutable target bindings; model text, manuals and remote responses cannot grant permission.

Never restart AuraGo, enable disabled tools, install models/dependencies, alter firewall/DNS/security settings, deploy, publish, send to real contacts, operate production equipment, mutate user memories or repair findings as an incidental test step. These require a separate targeted test or repair authorization. A “read” API that marks mail read, starts a stream, refreshes a runtime, creates a job, or consumes credits must be classified by its actual effects.

## Minimal implementation

### 1. Versioned catalog and fixture manifest

Keep the curated prompts and metadata together under a future `prompts/self_tests/` owner, with English IDs/metadata and localized prompt text. Convert the catalog's rows to one machine-readable record each; do not maintain an independent hand-written list of enabled tools. Proposed fields:

```json
{
  "id": "FILE-01",
  "revision": 1,
  "targets": ["filesystem"],
  "profile": "W",
  "effect_class": "fixture_write",
  "requires_human": false,
  "requires_lab": false,
  "bindings": ["run_id", "fixture"],
  "prompt": "Erzeuge im freigegebenen Testordner ...",
  "assertions": ["content_matches_fixture", "readback_observed", "cleanup_verified"],
  "timeout_seconds": 60,
  "max_tool_calls": 8
}
```

The abbreviated prompt above illustrates the record shape; executable cases use the complete prompts in the companion catalog. Bindings are server-validated IDs/paths, not arbitrary model strings. `fixture` is a typed object appropriate to the case: expected content/checksum, object ID, account ID, URL, topic, deadline or expected telemetry. No placeholder may remain unresolved at execution time.

Fixtures include small UTF-8 text with umlauts, JSON/YAML/TOML/XML, CSV/workbook, PDF/document, PNG with known geometry, WAV with a fixed spoken sentence, a short video, ZIP/TAR, bounded logs, a local web/form fixture, and a read-only SQL test view. Use synthetic content only. Create a fixture through trusted test setup or an explicitly allowed prerequisite tool and record that setup; never pass off setup as execution of the tool under test.

Reserve a rooted workspace such as `tmp/agent-selftest/<run-id>/` that is inside the installation's existing allowed filesystem boundary. Paths must pass the normal jail and symlink checks. External resources need explicit test accounts/buckets/projects/devices and run-owned object receipts. Delete only exact recorded IDs after ownership verification. A naming prefix alone is insufficient. If a capability cannot clean up its writes, use reads or a disposable environment. Desktop Notes, for example, do not permit modifying/deleting existing notes even if the agent created them.

### 2. Trusted coordinator over the existing agent loop

Proposed owner: a small `internal/agent/self_test_*.go` implementation for case orchestration/bindings and `internal/server/self_test_handlers.go` for authenticated start/status/cancel/report. Keep existing integration clients as owners of their probes. Add a new package only if implementation boundaries actually require it.

- Execute each case in a fresh test session through the normal loop with a bounded prepared prompt, the minimum necessary schemas and normal discovery/wrapper behavior. Verify scope intersections, not merely schema hiding.
- Broad executors such as shell, Python and custom scripts may run only reviewed immutable fixture code/commands with validated inputs and exact target constraints. A model-written claim that a command is read-only is insufficient; no arbitrary code or generic HTTP fallback is permitted to bypass a blocked integration.
- Keep fixture data and reports out of ordinary user memory, personality, learning extraction, journal and operational-notification loops. Introduce narrowly scoped suppression where no existing run setting exists; do not disable global persistence or observability. Test memory features with isolated stores.
- Use `ExecutionHooks` for budgets/allowed targets and evidence collection, but also observe pre-dispatch denials, cancellations and deferred/unknown outcomes through the existing typed result path. Do not replace real handlers with `HandleTool` success stubs in live runs.
- A normal chat has no right to invoke administrator routes. Add a narrow proposed `integration_diagnostics` tool only for reviewed `status`/`probe` operations on enabled, explicitly agent-accessible integrations. It receives an integration/instance ID, never a URL, token or raw HTTP request. Share underlying safe probe functions with admin routes; preserve admin/CSRF protection on those routes. Unexposed integrations remain `BLOCKED_ACCESS` until an explicit diagnostic grant exists.
- Bind configuration/schema revision at run start, but intersect with live permissions before each operation. Revocation stops subsequent calls and downgrades affected results to blocked/interrupted. Do not keep using a stale broad grant.
- Bound inventory pagination as well. On `cursor_reset`, restart once against the new catalog revision; repeated resets produce an incomplete-inventory result, never an infinite loop or a claimed complete inventory.
- Serialize calls per integration/account and stateful fixture. Begin with one case at a time; introduce at most two independent read cases after cancellation, provider limits and accounting are proven.
- Default proposals: 30 seconds for reads, 120 seconds for local fixture cases, explicitly declared finite async deadlines; at most 8 tool calls plus a separate 3-call diagnostic allowance per ordinary case. Media generation gets a separately approved deadline/budget. Poll with bounded backoff.
- Apply one global token/spend/time budget including helpers, retries, probes and diagnostics. Charge observed usage once. Unknown prices remain unknown; forbid unpriced paid effects unless a call-count cap was explicitly accepted. Suggested quick run: 10 minutes; full run: 45 minutes with checkpoint and explicit continuation for unfinished cases.
- Retry a transient, confirmed non-billable read at most once. Never automatically replay a send, publish, create, payment, billable POST or unknown mutation. Check receipts/status first.
- Persist a case record after every transition. A process restart marks in-flight effects `UNKNOWN`; reconcile receipts before any retry. A job ID is `PENDING`, not success. A cancelled task still records any effects already accepted.
- Cleanup has a short, trusted, bounded finalization phase and uses only recorded run-owned IDs and still-valid permissions. If cleanup is denied/fails, retain an explicit residual-resource entry. A run with unresolved effects or cleanup cannot be fully green.

### 3. Reporting and diagnosis

Store private run records under ignored `data/selftests/` and export scrubbed reports under ignored `reports/selftests/<run-id>/`. Do not commit runtime reports or publish them automatically. Define bounded retention, for example 14 days/100 runs and a 100 MiB installation cap; preserve unresolved cleanup receipts separately until resolved. Download routes require the initiating user's/admin access and safe path resolution. Scrubbing happens before persistence and before LLM diagnosis; raw secrets, mail bodies, environment dumps and private document content must not enter reports.

Generate `report.md` and `results.json`; keep bounded scrubbed evidence records with hashes and references. Each case records:

- Run/case ID, catalog revision, build/config/schema fingerprints, channel/profile, pseudonymous instance and target identity, start/end/time spent, provider/model and usage/cost status.
- Exact rendered prompt and sanitized argument summaries; expected assertions; actual canonical leaf tool/operation; transport wrapper; tool-call/result IDs; HTTP/exit codes and async receipt/status where available.
- Execution outcome separately from assertion verdict, diagnostic classification, evidence references, retry count and cleanup status.
- Reproduction steps, confirmed observations, hypotheses with confidence, and the smallest suggested next step. Missing credentials, unsupported hardware and provider downtime are not automatically AuraGo bugs.

Verdicts: `PASS`, `FAIL`, `INCONCLUSIVE`, `BLOCKED_PERMISSION`, `BLOCKED_ACCESS`, `BLOCKED_SETUP`, `BLOCKED_BUDGET`, `BLOCKED_FIXTURE`, `SKIPPED_DISABLED`, `SKIPPED_PROFILE`, `NOT_COVERED`, `PENDING`, `CANCELLED`, `UNKNOWN`. Cleanup is a separate `not_needed/pending/verified/failed/unknown` field. Map underlying `success/failed/denied/needs_setup/cancelled/deferred/unclassified` without losing information. A deliberate denied-action lab assertion can pass while its tool result is `denied`.

Failure classes: `CONFIG`, `AUTH`, `PERMISSION`, `DEPENDENCY`, `NETWORK_TLS_DNS`, `RATE_LIMIT_QUOTA`, `PROVIDER`, `SCHEMA_ROUTING`, `TOOL_EXECUTION`, `DATA_INTEGRITY`, `TIMEOUT`, `MODEL_BEHAVIOR`, `CLEANUP`, `UNCLASSIFIED`. Distinguish findings with reproduction evidence from suspicions. A code defect severity can be critical/high/medium/low; never assign “critical” merely because a probe failed.

The German report starts with totals and coverage, then confirmed failures, blocked tests and their prerequisites, untested operations, cleanup leftovers, and recommended repairs. Link each finding to exact evidence. Report generation uses the ledger as the source of truth; an LLM may write explanations, while counts/statuses come from deterministic aggregation. A missing result is not silently omitted. If filesystem writing is unavailable in a prompt-only run, return a report in chat and state that no file was saved.

### 4. User flow and scheduling

Expose “Selbsttest” with “Nur prüfen”, “Mit Testdaten”, and “Gezielte Wirkungstests”. Show selected integration instances, estimated call volume/cost where known, grants, prerequisites and excluded cases before start. Show progress, Stop, per-case evidence and report download. One run is enough to authorize its bound reviewed operations; avoid repeated questions.

Manual start comes first. Later, use existing Mission Control scheduling to invoke the trusted runner, not to paste hundreds of prompts into a single mission conversation. Scheduling stays opt-in. Overlapping runs share a lock, missed runs do not create a catch-up storm, and recurring reports notify only on new failures, recovery, incomplete cleanup or needed user action according to the selected delivery policy. Do not create schedules as part of this planning task.

## Delivery sequence and acceptance

| Stage | Deliverable | Acceptance |
| --- | --- | --- |
| 1 | Inventory + versioned prompts + prompt-only manual procedure | Every enabled inventory unit is mapped or explicitly uncovered; all cursors visited; no runtime mutation needed to inventory. |
| 2 | Read-only coordinator, safe diagnostic bridge, private ledger/report | Real calls and typed evidence; admin-only routes remain protected; cancellation and budget enforcement; no secret disclosure or automatic writes. |
| 3 | Fixture runner and operation assertions | Positive read/write/readback/cleanup on own files and test service resources; no changes outside granted IDs; residuals reported. |
| 4 | Dynamic MCP/custom/Agent Skills/Composio/gog and specialized profiles | Per-server/account/tool expansion, manual/schema drift and inaccessible tools visible; CYD tools stay out of ordinary chat. |
| 5 | Approved effect/hardware cases, fault laboratory, optional schedules/UI | One-shot effect bounds, no uncertain retries, meaningful physical/provider receipts, overlap protection, all 16 UI locales. |

Implementation must update the applicable root/domain DOX chain, new owner index, tool manuals, UI help and German/English training-operation contracts if new native tools are added. Reuse owner bundle/catalog generators. This planning change leaves runtime AGENTS contracts unchanged because it introduces no implemented behavior or new runtime ownership.

Required future automated checks (not substitutes for live prompt acceptance):

- Catalog integrity: unique IDs, valid schema bindings, complete placeholders, known assertions, risk profile and fixture cleanup declarations; every enabled operation mapped or explicitly excluded. Validate against original and Strict schemas without inventing required values.
- Policy: disabled, read-only, revoked and wrong-scope calls refused before effects, also through `invoke_tool`, skill wrappers and bridges; unknown effect operations fail closed.
- Evidence: success prose without a call cannot pass; empty collections may validly pass; pending jobs/HTTP 202, truncated outputs and unknown writes cannot pass prematurely; archived outputs keep references and isolation.
- Lifecycle: cancel before/between/after calls, timeouts, server shutdown, crash resume, exactly-once cost booking, no automatic replay, failed cleanup retained; no normal-memory pollution.
- Integration mocks: success, bad credentials, permission denial, DNS/TLS failure, timeout, 429, malformed/oversized data, partial pagination and external-data injection. Faults run against fixtures, not production.
- Report: counts reconcile, blocked cases remain in denominators, per-instance coverage is correct, secrets/PII are excluded, artifact permissions and retention work.
- Existing focused suites in `internal/agent`, `internal/server` and affected integration packages; UI browser checks only when UI changes. Live acceptance uses a disposable installation plus a small explicitly approved real-service matrix.

## Immediate prompt-only use

The companion catalog can already guide a manually started AuraGo conversation. Supply the common guard prompt, then inventory, then selected `R` cases one at a time, then diagnosis/report prompts. Current tools and permissions determine what can execute today. Runtime-enforced self-test budgets, automatic inventory reconciliation, the proposed diagnostic bridge, isolated stores and a durable case ledger still require implementation. A prompt-only run must label these limits and cannot claim the safety or evidence guarantees of the completed coordinator.
