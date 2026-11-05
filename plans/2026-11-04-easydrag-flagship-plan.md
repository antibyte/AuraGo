# EasyDrag flagship product plan

Status: implementation in progress. This roadmap is not a statement that every phase has shipped.
Requested outcome: make EasyDrag the flagship AuraGo experience, with exceptional usability, visual quality and depth.
Baseline: checkout `5f28d1967` and the working tree inspected on 2026-11-04. Reconfirm affected source before implementation.
Owners: `internal/flows/AGENTS.md`, `internal/server/AGENTS.md`, `ui/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md`; agent changes additionally require `internal/agent/AGENTS.md` and the prompt/training contracts.

## Implementation checkpoints

| Area | Current implementation boundary |
| --- | --- |
| Daily editing | Resizable workspace, step outline, selected-path focus, explicit mapping target, local introductory recipe, readable results and semantic publication review implemented. Cards/compact list, device-local favorites and saved filters are implemented. Automated theme/phone journeys pass; human usability acceptance remains open. |
| Recipes and data | Twelve curated recipes include email, MQTT temperature, device, camera and PDF examples with zero-effect outcome/failure tests. Six pure collection filters have 34 shared Go/JavaScript cases. The visual helper supports the existing 22 filters, exact expressions, typed local examples, explicit insertion targets and one isolated undo action. Automated browser/regression gates pass; local preview budgets are deliberately conservative and legacy date/HTML/case/rounding differences are disclosed when applicable. |
| Testing | Isolated backend simulation, encrypted private named scenarios, structured input, exact output/branch assertions and bounded retained-run comparison implemented. Scenario replay uses exact scenario version and current draft revision; automated browser acceptance passes. |
| Lifecycle | Paginated run history and explicit disabled Mission Control repair implemented. Test evidence requires the exact preview draft revision. |
| Organization | Shared flat folders and tags for flows, plus independently revisioned notes for published flow/block versions, are implemented. Metadata changes preserve executable documents, timestamps, pins and exact scenario/test evidence. Schema v4 takes a verified v3 backup; conflict, read-only, redaction and permission-loss journeys pass. Favorites and saved filters remain device-local. |
| Visual groups | Named, colored groups with notes, membership editing, movement, copy/paste and atomic undo are implemented. Collapse and layout locks are local presentation choices; outline navigation reveals hidden members. Legacy document bytes and pins remain unchanged. Schema v5 takes a verified v4 backup and prevents downgrade; grouped exports use import-only wire schema 2. Ordinary saves preserve omitted groups; version restore replaces the full document. Native theme/320–390 px journeys, old-binary refusal and migration preservation pass. Shared maximized-window resizing now follows workspace bounds without changing restore geometry. |
| AI | Goal-only proposal API and review/apply UI implemented for new and existing flows. Apply creates a new draft or one undoable revision-bound edit. Automated policy/shape/revision cases pass; real-provider task-quality evaluation remains open. No automatic publish, activation or execution. |
| Durable execution | State-transition contract is in `2026-11-04-easydrag-durable-execution-contract.md`. Private pure-run checkpoint/coordinator primitives have migration, encryption, lease and crash tests, including uncertain commit handling. After automated review requested explicit authorization, the user authorized Runner integration and full testing on 2026-11-05, with the feature kept disabled by default. Scheduler/admission/cancellation integration remains incomplete; no restart/resume promise is exposed yet. |
| Composition | Pure typed reusable blocks, isolated block simulation and synchronous nested `component.call` are implemented locally with immutable transitive revision/hash bindings, invocation-local node lookup, shared parent/child budgets, bounded admission and version/run retention pins. Calls are limited to eight levels and expanded graph limits; cycles are rejected before execution. Schema v3 introduced the pin tables with a verified v2 backup; nesting reuses them. The editor supports named fields, explicit caller upgrades, dependency-aware deletion and root-call validation focus. Publishing a child never upgrades callers automatically. Child-step history and effectful or durable blocks remain planned. |
| Selection extraction | A revision-bound preview and one atomic transaction create a published pure block and replace a normal flow's selected linear Set steps with a pinned call. Typed outputs, literal/filter syntax, security-sensitive fences, node deadlines and group membership are preserved within explicit eligibility limits. External object bindings require the trigger or the guaranteed immediate predecessor; unsupported branches, distant references and implicit inputs are refused. Expanded graph budgets include existing calls. One Undo restores the parent while keeping the block. A lost response is reconciled from one read snapshot; later edits or missing data remain uncertain, with no automatic Apply retry. Native desktop/320–390 px journeys and transaction/editor regression tests pass; release acceptance remains separate. |
| Batches | `component.each` processes at most 1,000 items serially through a pinned pure block, with explicit typed item/shared inputs, stop-or-collect behavior and ordered outcomes. Parent cancellation, deadlines and output/log limits are shared; preflight caps worst-case work at 10,000 step attempts. Literal item contents remain data across execution, validation, previews, renaming and clipboard operations. No nested/effectful/durable batch or automatic whole-batch retry is exposed. |
| Release acceptance | Human usability sessions, assistive-technology acceptance, reference-hardware performance and production integration acceptance remain unmeasured. Local fixture tests cannot substitute for these gates. |

Later phases and their acceptance criteria below remain part of the authorized implementation scope.

## 1. Product direction

EasyDrag should let a person turn an intended outcome into an understandable, testable and dependable automation. The graph is the editing tool; the user's result is the product.

Keep one consistent document and execution model. Offer progressive disclosure inside the same workspace: a guided recipe for newcomers, direct canvas manipulation for regular users, and precise data/diagnostic controls for experts. Avoid separate beginner/pro editions with incompatible capabilities or documents.

The flagship qualities are:

- **Clarity:** the user always understands what will happen, with which data, on which target, and whether it is simulated or live.
- **Craft:** readable typography, intentional spacing, consistent controls, stable layout, responsive feedback, and useful motion.
- **Confidence:** edits survive interruptions; failures explain their impact; publication and external actions are deliberate.
- **Depth:** reusable components, structured data, bounded iteration, persistent waits, human decisions and rich AuraGo integrations.
- **Ownership:** local data and templates, existing provider choice, explicit sharing, exportable flows, bounded resource use.

Recommended alternative to indiscriminate feature expansion: build a small number of complete, exemplary journeys first, then extend the same interaction grammar across the catalog. Adding a node is not complete until setup, mapping, test data, result inspection, failure recovery and help all work.

## 2. Existing foundation to preserve

Do not schedule these as if they were missing:

| Area | Existing foundation | Source |
| --- | --- | --- |
| Canvas | Multi-selection, copy/paste, undo/redo, minimap, zoom, auto-layout, node search, quick-add and non-drag connection choices | `ui/js/desktop/apps/easydrag-{canvas,interact,geometry,model,palette,history,wires}.js`; desktop app guide, EasyDrag section |
| Data | Upstream data mapping, template chips, filter expressions, preview, tree/table/JSON output | `easydrag-{mapping,template,forms,fields,detail}.js` |
| Lifecycle | Autosave, emergency local copy, conflicts, draft/live separation, publish preview, versions and restore-as-draft | `easydrag-{saver,publish,editor}.js` |
| Runs | Step tests, trigger samples, previous-run input reuse, live events/reconnection, stop, historical document view | `easydrag-runs.js`; `internal/flows/service_runs.go` and runner/store modules |
| Catalog | 38 curated types: 14 triggers and 24 other types, plus filtered generic tools; six starter templates | `internal/flows/AGENTS.md`, Catalog; `catalog*.go`, `templates*.go` |
| Safety | Vault references, permissions, effect review, input provenance, URL restrictions, cancellation and uncertainty, budgets and bounded data | `internal/flows/AGENTS.md`; server bridge/invoker/LLM contracts |
| Platform | German default, 16 locales, responsive editor, Standard/Fruity theme contracts, Mission Control integration | `documentation/manuals/de/24-easydrag.md`; UI guides |

The current document is a bounded directed graph: 500 nodes, 2,000 edges, 2 MiB document. Drafts may contain cycles with warnings; publication and execution require an acyclic graph. Existing waits, retries, error paths, merge nodes and completion-trigger chaining are useful foundations, but they do not by themselves provide persistent workflow suspension, safe partial replay, typed subflows or transactional external actions.

## 3. Evidence and uncertainty

A six-step current-run inspection used the real desktop bundle and EasyDrag assets with the repository's in-memory browser fixture at 1280 × 720, German, Standard theme. The fixture's catalog is deliberately smaller than production. No production configuration, provider, device or delivery endpoint was exercised.

| Step | Observed quality | Improvement hypothesis |
| --- | --- | --- |
| 1. Flow library | Search, filters, sorting and templates are present | Lead newcomers from an outcome to a ready-to-test recipe; show setup requirements before cloning |
| 2. Editor | Existing minimap and readable cards; the fourth sample node extends outside the viewport with the palette open | Make panel allocation adaptive and retain readable zoom; add focused path navigation |
| 3. Inspector | Named tabs and fields, keyboard opening and visible focus; the side inspector reduces canvas space and the selected node is no longer fully visible | Preserve selected-step context; show source and target together where space permits. Below 760 px retain the existing graph/inspector turn-taking layout |
| 4. Test preparation | Clear real-effects warning; the main input is raw JSON | Add sample cases and form-based input; implement real simulation below the UI |
| 5. Result | Tree/table/JSON, timings and tokens, successful fixture updates | Put content previews and a causal timeline ahead of technical metadata |
| 6. Publish preview | Clear draft/live explanation and effects; unchanged sample identified | Add meaningful before/after review, trigger preview and evidence of what was tested |

The screenshots and local notes are in ignored `reports/easydrag-plan-20261104/`. They are review evidence, not required product assets. This plan remains usable without that folder.

Source review supplements the captured states. Screen-reader, touch, Fruity, production integration, contrast and performance acceptance remain work for the implementation phases. Do not describe this inspection as full accessibility certification or successful end-to-end production testing.

Source anchors for actionable gaps (line numbers refer to the inspected baseline):

- `easydrag-detail.js:263`: click-to-map uses the last field, or a fallback first templatable field; make the insertion target explicit.
- `easydrag-publish.js:102`: publication diff summarizes counts; add named steps and changed parameter values.
- `easydrag-runs.js:831`: the drawer requests 50 runs; add a way to reach older retained runs.
- `easydrag-home.js:267`: templates render as one list; introduce discovery controls as the catalog grows.
- `internal/flows/service.go:251`: startup handles interrupted work; design persistence before promising resumability.
- `internal/flows/service.go:381`: creation selects flow kind; a stored block kind alone is not a usable subflow feature.

The JavaScript paths above are under `ui/js/desktop/apps/`. Reconfirm these anchors before editing, because concurrent work can move them.

## 4. Target workspace and visual direction

### 4.1 Information architecture

Keep the home library and the editor as the two primary places. Within a flow, provide three clearly named working views: **Build**, **Test**, and **Runs**. These are views over the same draft/live/run identities, not three editors with copied state. Versions, dependencies and settings remain contextual panels.

- Home: continue recent work, flows needing attention, favorites, folders/tags, saved filters, and an optional compact list. Templates have their own searchable collection rather than taking permanent space from a large flow library.
- Editor header: name, save state and exact draft/live status together; stable Test/Publish/Run locations with a state-appropriate primary action. Secondary export/email/settings actions move into a consistent menu. Do not make critical Stop or unsaved-state indicators disappear.
- Canvas: maximum useful area; collapsible/resizable library and inspector; user-controlled focus mode, selected-path framing and retained viewport. Opening an inspector must keep the selected step reachable.
- Inspector: a clear parameter form; attach a field's data source and preview nearby. Wide-window mapping can show source and target simultaneously; compact windows use a dedicated data picker sheet with preserved insertion position.
- Run detail: synchronized graph, chronological timeline, outcome preview and step input/output. Keep the exact executed revision visible, especially when the draft has changed.

### 4.2 Visual specification to produce in phase 0

Create explicit designs for home, empty/new flow, a small linear flow, a branched flow, mapping, simulation, failed-run diagnosis and publication. Include Standard dark and Fruity light/dark, a 1366 × 768 laptop, a 1920 × 1080 display, tablet and 390 px phone. Review at actual app-window sizes rather than only full browser width.

- Retain AuraGo's local fonts, icon system and theme tokens. Standard gets precise surfaces and clear separators; Fruity gets lighter chrome and softer depth. Both share geometry and behavior.
- Use an app-scoped spacing/type scale: 4/8 px spacing rhythm; 14–15 px primary form/body text and 12 px minimum secondary text at normal zoom as initial design targets. Verify against real localized content.
- Nodes communicate purpose, key parameter and current state. Use semantic icon + text + color; type color must never be confused with execution status. Selected, focused, disabled, invalid, running, simulated and uncertain are distinct.
- Improve connection readability through branch labels, selected-path emphasis and quieter unrelated wires. Show data previews on request, not permanently on every edge.
- Introduce compact/comfortable density without tiny hit targets. Keep ports visually small if appropriate, with generous hit regions and an equivalent explicit connect action.
- Use 120–180 ms transitions for panel/state changes as a starting target. Motion must not move the user's target or shift fields while typing; reduced-motion disables travel and pulsing effects. No decorative looping animation while idle.
- Design every empty, loading, stale, disconnected, blocked, conflict, read-only and partial-success state. Loading should preserve layout; error recovery should retain the user's entered values.

This is proposed art direction. Final geometry and palette choices require screenshot-based review before implementation; these are not fabricated finished mockups.

## 5. Product workstreams

### A. Outcome-led onboarding and templates — first release, then catalog expansion

**User outcome:** finish a useful first automation without understanding graph terminology or JSON.

1. A short entry choice: start from a recipe, describe an outcome, or build directly. Always allow skipping guidance.
2. Recipe cards show the actual intended result, inputs, required integrations, possible external effects and an indicative setup effort. Readiness means checked configuration, not proof a service is reachable.
3. A setup checklist binds existing accounts/devices/providers and requests only missing values. Return from configuration to the same step and draft. Credentials use existing Vault flows.
4. A guided walkthrough uses deterministic local steps and sample data, highlights the next incomplete step and ends with a visible result. In phase 1 it uses the existing real test on a recipe containing only pure logic; phase 2 adds simulation. No account, external provider or paid model is required for this introductory recipe.
5. Expand the six current templates into a curated initial set of about 12–15 complete recipes; quality and test coverage decide the count. Domains: morning brief, document intake, calendar reminders, home presence, quiet-hours alerts, service health, backups, spending limits and inbox triage.

**Acceptance:** at least 8 of 10 representative new users complete the bundled introductory local test within five minutes without assistance. Integration setup time is measured separately. Each recipe has a valid sample, expected result, missing-setup path, failure example and all 16 translations.

### B. Canvas and editor craftsmanship — first release

**User outcome:** construct and understand both small and large graphs without fighting the workspace.

1. Resizable panels, sensible first-open layout, focus-selected-path and a persistent step outline. Extend existing minimap, auto-layout and search rather than replacing them.
2. Searchable semantic commands with synonyms, recent/favorite nodes and type-compatible suggestions at a connection. The current quick-add becomes more useful; it is not a new parallel search system.
3. After the first release, visual groups with title, color and note, collapse/expand and optional alignment/locking. Groups are layout metadata initially; they must not change execution semantics.
4. Preview insertion, reconnection and multi-node operations before committing one undoable command. Explain an invalid connection next to the attempted target.
5. Route keyboard, mouse and touch through equivalent commands. Add a visible sequential outline for users who prefer a list to a spatial graph.

**Acceptance:** all authoring actions work with keyboard and single-pointer clicks; switching panels retains selection/caret/viewport; the 4-step and 50-step reference flows remain understandable on a small laptop; layout commands and group changes have predictable undo/redo.

### C. Data mapping that teaches itself — first release, then richer transforms

**User outcome:** know exactly which value enters a field and what it will become.

1. Searchable upstream data with friendly field names, type, example, source step, timestamp and provenance: schema example, test fixture or real run.
2. Insert by click, keyboard or drag; show the same source token and path in all modes. Warn clearly when a saved sample no longer matches the current node/schema.
3. Optional/missing/null/list states are distinct. A field receiving a list offers an explicit choice: one item, joined text, whole list, or bounded iteration once supported.
4. A visual transform builder for common operations: select/rename fields, defaults, format dates/numbers, split/join, filter, sort, deduplicate, aggregate. Keep the exact expression inspectable. Reuse the existing expression language and Go/JS parity vectors.
5. Evaluate previews against chosen local sample data with bounded work. Explain errors at the field with a concrete correction, preserving incomplete input.

**Acceptance:** users build the document-to-summary mapping without typing an expression; date/list/null cases round-trip identically through UI and Go; bad or huge data never freezes typing; no preview makes an external call.

### D. A safe and repeatable test workspace — core differentiator

**User outcome:** try an idea repeatedly without sending duplicate messages, altering devices or paying for repeated model calls.

1. Name three modes precisely: **Simulation**, **Real test**, **Live run**. Existing tests are real execution; a new label alone cannot turn them into simulation.
2. Simulation evaluates pure logic and uses validated sample outputs for external/tool/model steps. Unknown or unsupported steps stop with a request for sample data. Enforce this in the engine/invoker boundary; do not rely on frontend flags or incomplete effect labels.
3. Named test scenarios, including happy path, empty data, malformed input, unavailable service, timeout and failure branch. Use a form where a schema exists, with JSON as an advanced view.
4. Pin redacted outputs for a test, edit them locally, and invalidate incompatible fixtures. Production never reads test pins. File/media fixtures use authorized references and retention controls.
5. Run to a selected step, inspect intermediate values and compare expected outputs/branches. Initial stepping/breakpoints are simulation-only; production suspension comes later with persistence.
6. Reuse previous-run data only after checking truncation, redaction and schema compatibility. Do not execute `[redacted]` or omitted-data markers as if they were real values.

**Acceptance:** a simulation of a flow containing HTTP, file writes, notifications, device control and AI makes zero external calls and no product-state writes except bounded test records; each attempted bypass has a server test. Repeated fixtures produce the same branches; UI distinguishes simulated success from a verified real action.

### E. Run diagnosis and recovery — first release grows into operations

**User outcome:** answer “What happened, why, and what can I safely do next?” in one place.

1. Timeline shows queue time, active steps, retry attempts, waits, failure/cancellation and external-effect uncertainty. Use existing event sequencing and run documents.
2. Results render as readable text, tables, authorized file/document/media previews and delivery outcomes, with JSON available on demand. A tool success is not automatically verified third-party delivery.
3. Errors use a consistent structure: problem, affected data/target, already completed effects, actionable remedy, and optional technical detail. Link to the offending node/field or existing configuration panel.
4. Search/paginate retained runs by date, trigger, version, status and node. The current drawer asks for 50 records; retained older runs should be discoverable.
5. Compare runs and revisions, highlight changed input/output, and export a redacted diagnostic bundle only on explicit user action.
6. Offer “test this input in the current draft” and “inspect the original version” distinctly. Partial replay from a failed node requires the effect-safety work below; do not silently replay already completed actions.

**Acceptance:** at least 8 of 10 participants locate and explain a seeded missing-field or missing-connection failure within two minutes. Reconnect, late events and revision changes never show output against the wrong document. Old runs are reachable within configured retention.

### F. Publication, versions and library organization — first/second release

**User outcome:** safely evolve automations and find the one that needs attention.

1. Extend existing publish preview with node/field/connection before/after values, trigger schedule changes, targets, added effects and a link to the most relevant test of the exact draft revision.
2. Show next trigger times with timezone and DST/short-month explanation. Separate “publish this revision” from activation; preserve paused state unless the user explicitly changes it.
3. Named version notes, graph-aware comparison, restore as draft and clone from a version. Existing restore-as-draft remains the baseline.
4. Folders/tags/favorites, compact list, saved filters and bulk pause/archive with a preview. Flow deletion and a future archive/trash state must remain semantically distinct.
5. Dependency view: flows using a connection, secret reference, provider, reusable flow or integration. Show breakage before destructive configuration changes without exposing secret values.
6. Guided repair for missing Mission Control linkage and incomplete publication. Replace the documented export/delete/reimport workaround with a narrowly authorized, transactional repair path after ownership/conflict checks are designed.

**Acceptance:** every published version can be related to its review/test evidence; compare and restore never activate silently; concurrent editors keep their own revisions; repairing a linkage cannot attach a mission owned by another flow.

### G. Reusable and expressive automation — after testing foundations

**User outcome:** build sophisticated workflows without copying large blocks or relying on fragile conventions.

1. Typed subflows with explicit inputs/outputs, immutable published-version binding, extract-selection and parent/child run navigation. Updating a reusable flow shows affected callers before an explicit upgrade.
2. Bounded “for each” and batches as structured control nodes with a nested body, item identity, concurrency cap and clear stop/continue/collect-error behavior. Keep arbitrary graph cycles disallowed.
3. Explicit condition groups, collection transforms and merge/join semantics. A user can explain which branches must finish and what happens when one is skipped or fails.
4. Trigger controls for debounce, throttle, deduplication, quiet hours and event aggregation, with a clear distinction between dropping, coalescing and queuing an event.
5. Persistent variables only with a declared scope, lifetime, size bound and concurrency semantics. Prefer existing run data and subflow inputs before adding global mutable state.

**Acceptance:** recursion and cross-flow cycles are refused or bounded before execution; child work shares parent budgets/cancellation; 1,000 input items cannot create unbounded workers or outputs; retries never restart a whole completed batch accidentally.

### H. Durable waits, approvals and effect safety — foundational runtime work

**User outcome:** a long-running automation remains understandable through restarts and never claims an uncertain action was safely undone.

1. Persistent run checkpoints at defined boundaries, durable wait records and a resumable state machine. Current in-memory waiting and interruption-on-restart are not sufficient.
2. Wait for time, event or a human decision without holding a worker. Always expose timeout, cancellation and expiry behavior. Approval is bound to authenticated identity, exact revision, action summary and data digest.
3. Central action receipts/idempotency support where the integration permits it. A logical operation identity includes run, subflow invocation, node and iteration item; retry attempt numbers must not create a new logical action. Classify nodes as safe to retry, retry only with an idempotency key, or uncertain/manual reconciliation.
4. Avoid “exactly once” promises for arbitrary external services. A crash after an external effect and before its receipt may remain uncertain; display that state and require reconciliation rather than automatic resend.
5. Optional explicit compensation steps for reversible operations. Explain that compensation is another action and can fail; never call it universal Undo.
6. Approval inbox on Desktop/mobile, with deep links from notifications. Notifications are pointers to authenticated actions, not bearer authorization to execute them.

**Acceptance:** crash/fault injection before dispatch, after dispatch, before receipt, during wait and during approval produces a deterministic recoverable or uncertain state. Repeated approval/replay requests cannot duplicate a completed action. Live permissions are rechecked on resume.

### I. AI-assisted creation and maintenance — after schemas and review UI

**User outcome:** describe a goal, understand the proposed automation and improve it without trusting an opaque generated graph.

1. “Describe your automation” produces a reviewable draft using only available catalog nodes and their schemas. Ask targeted questions for genuinely missing bindings; otherwise use explicit, editable assumptions.
2. Explain a selected path in plain language, suggest the next compatible step, draft mappings and propose a fix from redacted diagnostics.
3. AI modifications are a previewable diff against one draft revision and apply as one undoable transaction. Show permissions, external destinations, changed triggers and estimated model cost when available.
4. Use AuraGo providers and existing budgets. Keep model unavailability graceful: manual authoring, templates, validation and simulation remain useful.
5. Treat captured data, descriptions and imported templates as untrusted. AI can propose; it cannot bypass tool policy, read secrets, publish, activate or run a flow as a side effect of explaining it.
6. Curated evaluation tasks measure valid drafts, successful sample outcomes, unsafe proposals rejected and user correction effort. Do not measure quality by attractive-looking graphs alone.

**Acceptance:** generated drafts validate and solve the reference scenarios; invalid schemas and fabricated integrations are caught before applying; stale proposals cannot overwrite a newer draft; no publication/activation/run occurs without the corresponding explicit user action.

### J. AuraGo integration and daily operation — incremental throughout

**User outcome:** EasyDrag feels like the organizing tool for the whole system.

1. Promote frequent generic tools into curated operations with precise forms, output schemas, test fixtures and honest effect metadata. Prioritize Home Assistant/MQTT, files/documents, notifications/calendar, then read-only infrastructure health and carefully gated remediation.
2. Deep-link from Files, Calendar, Mission Control, device/infrastructure views and relevant results to “create automation with this item”. Pass typed references, never arbitrary hidden authority.
3. Reusable connection selectors show saved configuration, missing permission and last explicit connectivity result separately. Reuse owning integration APIs instead of creating a competing credential subsystem.
4. Operational overview: active/paused/degraded flows, failed/skipped triggers, queue pressure, overdue waits, pending approvals, duration trends and known cost. Unknown provider prices display unknown, not zero.
5. Per-flow/run budgets, notification routing, deduplicated failure/recovery notices and maintenance pauses. New data-heavy metrics must respect existing retention and disk budgets.
6. A mobile task view prioritizes checking outcomes, approvals, pausing and guided parameter changes while preserving access to full editing through an outline and compact inspector.
7. Portable flow packs contain metadata, required versions/capabilities, fixtures and setup instructions. Import previews dependencies/effects; credentials and captured personal payloads are excluded by default. A public marketplace and live collaborative editing are later options, not prerequisites.

**Acceptance:** entry from another AuraGo app retains the originating resource and permission scope; denied/offline integrations have a next step; no integration is enabled implicitly; exports can be imported cleanly with explicit rebinding.

## 6. Reference journeys

These journeys define completeness across workstreams:

1. **First success:** choose a local text-processing recipe → supply sample text → test the pure local steps → inspect result → understand activation. No network required; add the distinct simulation path in phase 2.
2. **Daily briefing:** scheduled sources → selected-provider summary → PDF → notification; preview content and target, test with fixtures, publish, inspect a later failure.
3. **Home automation:** HA/MQTT event → debounce and quiet hours → condition → explicit device action → confirmation/readback where supported. Explain unavailable or uncertain devices.
4. **Document intake:** chosen folder/event source → extract fields → validate → human approval → move/save/send. Invalid documents follow a visible review branch.
5. **Infrastructure watch:** health signal → diagnosis → notification → explicit approval before a scoped restart. A restart timeout is reconciled, not blindly repeated.
6. **Repair:** open failure notification → see failed step and completed effects → fix draft → simulate with redacted input → compare → publish deliberately.
7. **Reuse:** extract a tested sequence → define typed inputs → use it in two parents → publish a new child revision → inspect and approve caller upgrades.
8. **Recovery:** restart AuraGo during a durable wait or ambiguous external action → reopen the exact run → resume safely or show the unresolved effect.

## 7. Architecture and compatibility

- Preserve the native vanilla-JS app, existing command/model/bus boundaries and Go flow service. No framework or workflow-engine replacement is justified by this plan.
- Add features behind the appropriate ownership boundary: editor state under EasyDrag modules, run semantics under `internal/flows`, integration authority in server/invoker adapters, credentials in Vault, AI proposal policy under agent/server owners.
- Extend catalog metadata before hand-written per-node UI: typed inputs/outputs, sample provenance, simulation capability, retry safety, effect previews and connection requirements. Treat absent metadata as unknown.
- Keep document, catalog, API, execution, persistence and UI schema versions distinct. Specify migration and compatibility matrices before adding groups, subflow references, test fixtures, run receipts or checkpoints.
- Existing v1 exports must continue to import; unsupported newer semantics must fail clearly without deleting fields. Snapshot/backup databases before migrations; test old data and downgrade refusal in staging.
- Model changes remain atomic undo operations. View-only choices never create draft changes. Concurrent window replies remain bound to their initiating document/revision.
- Simulation must not enter the ordinary invoker. Real test and live operations retain current permissions, provenance, resource limits, budget accounting and credential redaction.
- Durable execution and replay require a written state-transition specification and crash tests before the UI exposes “resume”. A new relational store schema must not silently reinterpret interrupted old runs as resumable.
- Keep subflow budgets/cancellation/authority inherited and bounded; do not implement them through an uncontrolled recursive agent/tool call.
- Batch/stream artifact previews use bounded pages and authorized media/file handles. Large binary payloads stay outside flow JSON and event logs.
- Library organization and visual groups must not require an external database, hosted account or paid provider.

## 8. Delivery sequence and exit gates

Effort bands are relative planning sizes (S/M/L/XL), not calendar commitments. Runtime semantics need a spike and fault-injection design before estimation.

| Phase | Scope | Size | Dependencies | Exit gate |
| --- | --- | --- | --- | --- |
| 0. Baseline and design contract | Reference journeys, usability sessions, metric baseline, actual-window designs, theme/state matrix, schema proposals and a retry/persistence risk spike | M | None | Approved interaction/visual target, measured baseline, prioritized backlog |
| 1. Flagship daily experience | A/B, first C/E/F slices: onboarding, panel layout, mapping clarity, result previews, actionable failures, publish diff | L | 0 | First-success and repair journeys meet usability targets; old editing contracts pass |
| 2. Test workspace | D, richer C and E: sample scenarios, mock/pin data, bounded preview, simulation, run comparison/search | L | 1 + simulation metadata/boundary | Zero-effect simulation verified at backend; reproducible scenario suite |
| 3. Reliable execution | H and relevant F/J: retry safety, receipts, durable waits, approval, reconciliation | XL | 2 + persistence design | Crash matrix, race checks, migration/retention acceptance |
| 4. Composable automation | G plus selected curated J actions: typed subflows, bounded batches, trigger controls | XL | 2; uses 3 for durable/replay features | Parent/child isolation, cycle/budget/cancel and batch failure acceptance |
| 5. AI and ecosystem | I, remaining J: proposal diffs, repair assistance, portable packs, richer operational views | L | 1/2; 3/4 where proposed actions require them | Evaluation corpus, permission review, all reference journeys pass |

Accessibility, localization, performance, design QA and documentation are part of every phase. Do not postpone them to a final polish pass. Early AI explanation/proposal experiments may run against fixtures after phase 1, but cannot block the core authoring experience.

**First-release boundary:** finish three slices: (1) the pure local first-success recipe, (2) editing the existing morning briefing with selected-step visibility, explicit mapping target and readable output, and (3) diagnosing a seeded failed run, correcting its draft and reviewing the publication diff. Improve three existing templates plus the local introductory recipe first. The 12–15-recipe collection, visual groups, general transform builder and durable library organization follow after these slices pass. Simulation-dependent repair/replay acceptance belongs to phase 2; the first release must not imply those capabilities exist.

## 9. Quality gates

All figures below are proposed release targets, not current measured performance.

| Dimension | Target and method |
| --- | --- |
| First successful use | At least 80% of 10 representative newcomers finish the local introductory test in 5 minutes, unaided; separately report expert results and integration setup |
| Error recovery | At least 80% locate/explain and correct a seeded mapping/setup error in 2 minutes without docs |
| Responsiveness | On a recorded midrange reference laptop, 500-node/2,000-edge fixture: p95 input feedback ≤100 ms; p95 pan/drag frame ≤33 ms; common 100-node path aims for 60 fps. Record browser, hardware and data shape |
| Loading | With local assets and a responsive local server: warm open of a 100-node flow ≤1 second, cold open ≤2 seconds, p95 over a reproducible sample. Network/remote-provider time is measured separately |
| Data preservation | Automated fault cases for save error, browser reload, concurrent edit, IME, import failure and interrupted migration lose no acknowledged save and preserve recoverable pending edits |
| Accessibility | WCAG 2.2 AA target for applicable UI; manual keyboard/single-pointer task journeys; NVDA + Chrome/Firefox and VoiceOver + Safari review; no color-only state; reduced-motion and 200% text/zoom coverage |
| Touch/reflow | 320/390/768 px widths and orientation changes, with real touch/pointer coverage. Form/header actions remain reachable; the spatial canvas has an equivalent usable outline |
| Target sizes | Product target: 44 × 44 CSS px for primary touch actions; desktop follows at least the applicable 24 × 24 minimum/spacing rule, with larger effective port hit regions |
| Themes/locales | Standard and Fruity light/dark full journey tests; representative screens across other supported skins; all 16 genuine translations, long-text and CJK/Hindi cases |
| Safety | Zero invoker/network/device/LLM calls from simulation; no secret leakage through preview/export/AI; receipt/replay and approval authorization tests |
| Reliability | Cancellation, queue fairness, duplicate events, reconnect, restart and uncertain-effect fault cases; resource/retention budgets remain enforced |
| Visual quality | Screenshot comparison at actual app-window sizes for every reference state; no inaccessible clipped controls, unstable fields, unreadable text or accidental global CSS changes |

W3C's dragging criterion requires a single-pointer alternative to dragging. Keyboard accessibility is a separate requirement, so keyboard shortcuts alone do not satisfy the dragging criterion. The 44 px touch goal is a product choice above the applicable WCAG 2.2 AA minimum. Sources: [Dragging Movements](https://www.w3.org/WAI/WCAG22/Understanding/dragging-movements.html), [Target Size (Minimum)](https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum.html).

## 10. Verification and DOX work for implementation

Use the existing checks, adding focused tests for new semantics rather than duplicating established coverage:

- Node checks: `scripts/test-easydrag.mjs`, `test-easydrag-extra.mjs` through `test-easydrag-extra7.mjs`, `test-easydrag-ux-batch5.mjs`.
- UI: `go test ./ui -run 'EasyDrag|MissionControlShowsFlow|StandaloneMissionsPage'`; browser smoke with `AURAGO_RUN_BROWSER_SMOKE=1` for `TestDesktopEasyDragBrowser`, `TestDesktopEasyDragDecisionBrowser`, `TestDesktopEasyDragUXBrowser` and new journey cases.
- Runtime: `go test ./internal/flows/ -count=1`, focused server bridge/invoker/handler and configuration cases, and Linux race coverage for changed runner/service/store paths.
- Integration: existing Mission Control, dashboard cron and missions-page checks; effective tool-schema/training validation when AI/tool exposure changes.
- UI packaging: syntax, locale parity, `npm run build:ui -- --check` and version-bound asset verification for implementation releases.
- Every phase updates the nearest owning guides, English/German user manuals and all changed UI locales. New permanent child owners are justified by a real boundary, not a feature-count target.

Existing runtime and UI guides remain authoritative for shipped behavior. The checkpoint table separates implemented slices from unfinished work; guides must change with implementation rather than preemptively advertise proposed features.

## 11. External pattern references

These references informed proposed interactions; they are not claims that a competitor's whole product should be copied or that EasyDrag lacks every listed feature.

- [n8n data mapping](https://docs.n8n.io/data/data-mapping/data-mapping-ui/): visible source-to-parameter mapping; extend EasyDrag's existing mapping with stronger type/provenance guidance.
- [n8n pin and mock data](https://docs.n8n.io/data/data-pinning/): repeatable development data kept separate from production; use explicit simulation boundaries in EasyDrag.
- [n8n execution debugging](https://docs.n8n.io/build/understand-workflows/understand-executions/debug-executions/) and [Make run replay](https://help.make.com/scenario-run-replay): prior-run data helps diagnosis, but replay may repeat external actions.
- [Node-RED subflows](https://nodered.org/docs/user-guide/editor/workspace/subflows): reuse with explicit boundaries and recursion restrictions; add version binding and inherited budgets for AuraGo.
- [Windmill suspend and approval](https://www.windmill.dev/docs/flows/flow_approval): suspended execution and timeout handling; use AuraGo authentication rather than treating a notification link as authorization.
- [n8n workflow history](https://docs.n8n.io/workflows/history/): inspect/compare/restore; build on EasyDrag's existing versions and restore-as-draft.

## 12. Immediate next implementation unit

Integrate and fully test the explicitly authorized pure-local durable Runner path, kept disabled by default, after the extraction clean-checkout gate. Reinspect current sources rather than applying the historical rejected patch; verify admission, restart/wake, original deadlines, cancellation, shutdown, key availability, Mission Control history and uncertain commit handling before exposing the capability. No deployment or activation is part of this slice.

Truthful child-step navigation follows with explicit invocation identity and bounded retained history. Distinguish repeated calls, nesting and batch items; never identify a child step by its document-local node ID alone. A completed-call trace can preserve the existing parent coordinator and events without pretending to stream child progress. Retain immutable block revisions, expose missing/truncated history and verify any store migration with backup and old-row handling. Simulation remains nonpersistent and effect-free. Human usability, assistive-technology and reference-hardware acceptance remain separate release gates.
