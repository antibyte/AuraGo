# Detective: agent-driven research for the Virtual Desktop

Status: proposed implementation plan, 2026-09-19. Source review baseline:
`c1b1de0a4`. This document adds no runtime functionality.

## Purpose

Add a built-in Desktop app named **Detective** (`detective`). A user submits a
topic or question, selects **Quick**, **Normal**, or **Maximum effort**, and
receives a research report with traceable sources. The same report revision can
be downloaded as Markdown, PDF, and Word (`.docx`) without another LLM call.

Research should cover the relevant publicly accessible information within an
explicit budget. Never claim to have found every fact on the Internet. State the
research date, investigated questions, source coverage, contradictions, and gaps.
The effort setting controls breadth, depth, verification, time, and tool budget;
it does not merely make the final answer longer.

## Existing building blocks and required extensions

| Area | Reuse | Required work |
| --- | --- | --- |
| Agent execution | `internal/agent/agent.go`: `RunConfig`, `ToolCallLimit`, `AllowedTools`, `AllowedAgentSkills`, `Checkpoint`, `StableSystemPrompt`, `RunComplete` | Bind a Detective job and an authoritative cumulative budget to every dispatch and LLM request. |
| Agent loop | `internal/agent/agent_loop.go`, `agent_parse.go`, `recovery_policy.go` | Add a server-owned per-run iteration limit; the current loop has a fixed 100-iteration ceiling. Preserve the legacy default for other callers. |
| Job lifecycle | `internal/gamemaker/service.go`, server Game Maker continuation/broker, `internal/server/mission_runs.go` | Use these patterns for a separate research service, store, cancellation, checkpoints, and event replay. Do not depend on Game Maker project files or game validation. |
| Tools and integrations | Native tool catalog, `discover_tools`, `invoke_tool`, skill manager, MCP and Composio dispatch | Generate a job-specific capability snapshot and enforce operation/resource scope through every dispatch path. |
| Curated skill | `internal/tools/agent_skills_bundled.go` | Parameterize the currently hard-coded `system:game-maker` provenance for trusted built-in callers without weakening package hash verification. |
| PDF | Maroto in `internal/tools/document_creator.go`; optional configured Gotenberg backend | Reuse rendering components with report pagination, font coverage, tables, and source links; avoid the global document output directory for private case artifacts. |
| Word | `internal/office/office.go`: `EncodeDOCX`, `EncodeDocument` | Add a separate additive report encoder for real tables, hyperlinks, source references, and page structure. The current paragraph/run encoder is insufficient for the complete report contract. Preserve existing Office behavior. |
| Desktop | Built-in registrations, lazy `module-loader.js`, app lifecycle, theme bridge, SSE patterns | Add the app, icon, source/report views, all 16 locales, and window-local cleanup. |

Use the existing provider/model configuration and route-aware context fitting.
Detective owns an isolated session, not a message in the default Desktop chat.
The existing Researcher specialist supplies useful guidance but is not the sole
execution host: co-agent restrictions exclude Virtual Workspace control, and a
specialist prompt alone does not implement case persistence or evidence capture.
Version 1 uses one coordinating agent per running case, with bounded parallel
read operations where supported. Additional autonomous sub-agents are unnecessary
for the initial version.

## Desktop flow

1. **New research**: topic/question and intensity; optional timeframe, region,
   report language, source URLs/files, and specific questions. Default to Normal
   and the Desktop language. Advanced settings expose provider/model through the
   existing selector and any configured token/cost ceiling.
2. Show a concise capability summary before starting: available search engines,
   browser, document reading, and relevant connected services. Public Internet
   research is the default. Private connected data and local documents require
   an explicit source selection for that case; selecting an integration never
   grants new system permissions.
3. Start immediately when the request is sufficiently clear. Ask a compact
   clarification inside the case only when ambiguity would materially change the
   research. Save assumptions when reasonable defaults suffice.
4. Left pane: research cases and their status. Main pane: **Report**, **Sources**,
   and **Activity**. A running header shows the current phase, elapsed active
   time, remaining budget, and Stop. Use actual findings and tool activity for
   progress; never manufacture percentage completion or display private reasoning.
5. A source row shows title, publisher/domain, URL, retrieval date, publication
   date when known, retrieval status, and which findings use it. Distinguish a
   search hit from a document actually read. A user can inspect the supporting
   excerpt and open the source.
6. **Finish with current findings** starts bounded synthesis. **Stop** cancels
   running work immediately and retains the checkpoint. **Continue** restores
   the unfinished case. **Deepen research** creates a new revision with an
   explicitly selected additional budget and reuses previous evidence.
7. A completed report provides Markdown/PDF/Word downloads and **Open in Autor**
   for an explicit exported copy. Export remains available for partial reports,
   clearly labeled as partial. Export failures leave the completed research intact.

Closing the app detaches its event subscription; it does not cancel the job.
Reopening or refreshing shows persisted status and replays missed events.
Use native Desktop materials, accent colors, focus controls, and Standard/Fruity
light/dark themes. Collapse to a single pane on narrow windows. The app should
look like part of the Desktop, with a restrained detective/magnifier identity.

## Effort profiles and circuit breakers

Initial configurable defaults:

| Profile | Maximum active job time | Tool dispatches | Agent loop iterations | Typical evidence scope, not a quota |
| --- | ---: | ---: | ---: | --- |
| Quick | 5 minutes | 40 | 60 | 4–8 relevant read sources; direct answer and essential checks |
| Normal | 15 minutes | 100 | 140 | 10–25 sources; multiple aspects and independent corroboration |
| Maximum effort | 60 minutes | 250 | 320 | 25–60 sources where useful; primary documents, alternative explanations, contradiction and gap checks |

These are Detective-specific execution profiles, raising the ordinary default
budget without modifying global settings. Administrators can change each profile
within an explicit Detective ceiling. The UI displays the effective values;
never silently apply a lower generic tool/iteration limit halfway through a job.
Hard provider context limits, global spending limits, and tool security policy
remain authoritative. These numbers are product defaults to validate, not
measured completion-time promises.

Budget rules:

- Start active time when the execution slot is acquired; queue and user-input
  waiting time are shown separately. Count all running phases, retries, backoff,
  and timeouts. An app refresh must not reset the clock.
- Reserve the final 20% of time and any configured token allowance for synthesis
  and source checking. Stop new research calls at the research deadline. If the
  provider is unavailable, retain findings and produce an explicitly incomplete
  deterministic outline instead of inventing a finished report.
- Maintain one persistent ledger for the complete job: tool calls, LLM requests,
  iterations, tokens, estimated cost, fetched pages/bytes, and active duration.
  Changing phase, re-entering the loop, failover, or Continue cannot reset it.
- Reserve capacity atomically before dispatching a parallel batch. Nested calls
  via `invoke_tool`, skills/Tool Bridge, MCP, Composio, and crawlers are accounted
  for at their underlying operation boundary. Do not double-charge a wrapper,
  and do not treat a 100-page crawl as one page. Fetch size, page, concurrency,
  and tool-duration limits remain bounded independently of dispatch count.
- Keep duplicate-call, repeated-error, empty-output, and stream-inactivity
  breakers effective. Maximum effort grants more useful research, not more
  identical failed requests. Rate limits honor bounded Retry-After within the
  remaining deadline, then use another available source or record a gap.
- End early once the requested questions have supported answers and further
  targeted searches add no material evidence. No padding to hit a source quota.
- Exhausted jobs become `partial` with a named reason. Continue uses remaining
  allowance after interruption; an exhausted allowance needs the explicit
  Deepen action. Cancellation must also stop in-flight tools and free browser
  sessions, then save a final checkpoint with a short bounded cleanup context.

## Research workflow and evidence

1. Convert the request into a small set of research questions, terms, scope, and
   completion criteria. Save the plan as structured case data.
2. Search broadly, including relevant language variants. Prefer primary sources,
   original datasets, official documents, and direct reporting where suitable.
3. Read promising pages/documents and follow relevant references. Use targeted
   crawling for documentation or archives; do not crawl whole domains blindly.
4. Record evidence continuously. Compare independent sources and track
   syndicated copies as one origin. Important contradictory results trigger
   another targeted check within the selected budget.
5. Synthesize a report with a brief answer first, findings organized by the
   research questions, supporting references, disagreements, and open questions.
6. Validate citation integrity and coverage before final publication. A citation
   must reference a source that was actually retrieved and the relevant excerpt
   or document page. Structural validation is not proof that a claim is true;
   clearly distinguish sourced facts, analysis, and uncertainty.

Persist `Case`, `Run`, `ResearchQuestion`, `Source`, `Evidence`, `Finding`,
`ReportRevision`, `Artifact`, and bounded `Event` records. A source has a stable
ID, canonical/original URL, title, publisher, optional author/publication date,
retrieval time, retrieval method, content hash, and status. Evidence stores a
bounded excerpt, locator/page, and retrieval reference. Findings link evidence
IDs; they do not obtain credibility by supplying a plausible URL.

The server records successful retrievals from real tool results and mints their
source IDs. The agent can organize sources and attach findings, but cannot mark
an unseen page as retrieved. Search snippets, generated tool summaries, and
visual inference remain distinguishable from original source text. Preserve
original supporting excerpts even when a helper summarizes a long document.
Use bounded case-local downloads and caches; never put all source text into
every LLM request. Apply retention and explicit case deletion to artifacts,
cached extracts, browser sessions, and private continuations.

## Tools, integrations, and the Detective skill

Add `internal/detective/skills/aurago-detective/SKILL.md`, embedded and verified as
a bundled system package. Load it once automatically for Detective. It explains
question decomposition, query selection, source evaluation, evidence recording,
contradictions, stopping criteria, budget use, report structure, and recovery.

Attach a compact **server-generated capability snapshot** to the case context:
exact callable tool/skill names, operations, parameter examples, prerequisites,
limits, availability, and the catalog's binding `call_method`. It must reflect
this installation, not a hand-maintained promise that every integration exists.

| Research need | Existing capability to consider when enabled |
| --- | --- |
| General search | `ddg_search`, configured `brave_search`; `wikipedia_search` for orientation |
| Read pages and feeds | `web_scraper` with static/auto/RSS modes; `site_crawler` for a bounded site section |
| Interactive or rendered sites | `virtual_workspace` / `virtual_browser`; configured background `browser_automation` where appropriate |
| Papers, PDFs, screenshots | Registered `pdf_extractor`, existing file/media readers and `analyze_image` where needed |
| Structured/public data | `api_request`, applicable native data integrations, permitted `mcp_call` and `composio_call` operations |
| Selected user material | `workspace_search`, file/Office readers, permitted connected-service read operations within the case scope |
| Additional capabilities | `discover_tools` plus its advertised direct / `invoke_tool` / `execute_skill` / `run_tool` path |

Show common research tools and exact examples up front so the agent does not
spend calls rediscovering them. Keep the full configured catalog discoverable for
unusual subjects. Use relevant tools, rather than calling every integration for
every task. Unavailable providers or services cause transparent fallbacks.

A dedicated job policy enforces operation- and resource-level scope through
direct, bridged, skill, MCP, and Composio execution. A tool-name allowlist alone
is insufficient for mixed read/write tools. Read-only annotations are hints,
not authorization. Unknown external operations need an administrator-approved
research policy. Computation may use existing enabled sandboxed processing;
only case-local output is writable. No installation/config changes, external
messages, purchases, unrelated file writes, or credential exposure are implied
by starting research. Existing network/redirect/SSRF and content-safety rules
continue to apply. Web content stays untrusted data, never a skill instruction.

Interactive challenges enter `waiting_for_user` with a visible workspace link
when available; otherwise record the access gap. Do not claim that a headless
session is the user's visible browser, repeatedly retry CAPTCHAs, or let one
blocked source stall the entire investigation.

Use a stable system prompt, skill revision, and deterministically ordered tool
schema set per phase. Topic, remaining budgets, retrieved evidence, and progress
belong in appended data messages, not constantly rewritten prefix instructions.
Resolve additional tools through discovery; change the schema snapshot only at
an explicit capability/phase boundary. Re-check live permissions at dispatch and
route context limits on every call. Resuming restores complete tool/result
groups and the private provider-native continuation where compatible; UI and
exports receive findings and concise progress summaries only.

## Reports and exports

Use a versioned structured `ResearchReport` as the single source for display and
all exporters. Supported blocks: headings, paragraphs with citation IDs, lists,
tables, short quotations, and optional locally stored figures with attribution.
Each report includes title, question, scope/date, summary, findings, limitations,
and bibliography; long reports add a contents section. Findings may be concise
in Quick mode, but citations and uncertainty rules apply to every mode.

- **Markdown:** UTF-8, ordinary links/numbered references, portable tables and
  headings. Text-only reports are one `.md` file. If referenced local figures
  exist, offer a companion ZIP with the same Markdown and relative `assets/`
  files; never silently produce broken image paths.
- **PDF:** server-generated paginated output with embedded licensed fonts,
  selectable text, page numbers, repeated table headers, and clickable sources.
  Reuse Maroto for the built-in path, with an optional already configured
  Gotenberg renderer. A new Docker service or an open Desktop window must not be
  required for baseline export. Font fallback and complex-script rendering must
  be proven with fixtures, not inferred from UTF-8 input support.
- **Word:** a genuine editable OOXML `.docx`, with semantic headings, native
  tables, link relationships, numbered source references, page numbers, and
  embedded approved figures. Extend the Office backend additively for report
  blocks; do not flatten existing Autor documents through the legacy encoder.

Export from a pinned immutable report revision, with a safe filename, correct
MIME type, checksum, and atomic publication. Queue export work separately from
the research budget, with its own bounded timeout. Re-export does not call the
LLM or overwrite an edited Autor copy. No source credentials, raw HTML/scripts,
private agent conversation, or provider reasoning enters the deliverable.

## Backend and integration outline

- `internal/detective/`: SQLite store/migrations, service, job state machine,
  budgets, evidence, report schema, exports, and embedded skill. Add its owning
  `AGENTS.md` when the implementation creates this runtime boundary.
- `internal/server/detective_*.go`: authenticated Desktop handlers, runner,
  checkpoint/broker, download delivery, and capability snapshot. Reuse existing
  Desktop auth/CSRF/read-only gates and provider selection.
- `internal/agent/`: minimal generic additions for server-owned iteration limits,
  cumulative usage callbacks/dispatch scope, and nested-call propagation. Keep
  other callers' existing defaults and prompt behavior unchanged.
- `internal/config/`: Detective enablement, profiles, concurrency, retention,
  case storage, and optional token/cost ceilings; synchronize template and UI.
- Desktop: `detective.js` with separate API, case list, activity, sources and
  report modules; `desktop-app-detective.css`, built-in app registration,
  module loader, icon allowlist/manifests, and sixteen locale dictionaries.

Proposed API prefix `/api/desktop/detective`: capabilities; cases list/create;
case read/update/delete; run start/stop/finish/continue; events with cursor replay;
sources/evidence; report revisions; export creation/status/download. Start and
Continue accept idempotency keys and permit at most one active run per case.
Version 1 runs one research case per server by default and queues others through
the existing global agent concurrency guard; configuration may increase it.

Persist states `queued`, `running`, `waiting_for_user`, `completed`, `partial`,
`cancelled`, `interrupted`, and `failed`, plus a separate research phase. On
server restart, stale running jobs become interrupted and are resumable. Never
silently restart from zero or claim completion from model prose alone.

## Delivery sequence and acceptance

1. **End-to-end slice:** case storage, isolated job, Quick profile, search/read,
   persisted evidence, cited Markdown report, stop/reopen/continue, basic Desktop
   UI. Prove the loop with deterministic mock tools/provider first.
2. **Research quality and budgets:** all profiles, cumulative nested accounting,
   operation scopes, live capability snapshot, verified skill, contradiction
   checks, meaningful progress, bounded failures, and stable prompt prefixes.
3. **Complete reports:** canonical report revisions, built-in PDF, real DOCX,
   tables/citations/fonts, source inspection, export retry and Autor handoff.
4. **Integration and acceptance:** all enabled research routes, provider failover,
   browser handover, sixteen translations, themes, lifecycle, live comparative
   runs, resource packaging, and documentation.

Required verification:

- Fake-clock tests for each profile, reserve transition, early completion,
  remaining budget after resume, iteration limits above 100 only for configured
  jobs, concurrent batches and nested dispatches. Prove no budget reset or global
  config mutation, and that provider/global spending caps remain effective.
- Cancellation reaches a blocked LLM, crawl, and browser operation. Closing a
  window does not cancel a job. Refresh/event replay, duplicate Start/Continue,
  provider failure, process restart, and incomplete checkpoint recovery retain
  findings without launching duplicate research.
- Evidence fixtures cover duplicates/syndication, conflicting dates/numbers,
  redirects, search-only snippets, inaccessible sources, PDF page references,
  prompt injection, fabricated source IDs, irrelevant citations, and scripts in
  report content. Structural checks must not overstate factual certainty.
- Capability tests cover missing credentials, disabled integrations, revoked
  permissions mid-run, read/write tools, MCP/Composio and Tool Bridge paths;
  blocked paths cannot be regained through a wrapper or a skill.
- Exact report content/source parity across Markdown, PDF and DOCX. Open DOCX in
  Autor and Word/LibreOffice; render PDF and inspect pagination, links, tables,
  figures, long URLs, umlauts and representative scripts from all 16 locales.
- Browser tests for real start, stop, continue, source inspection and download,
  Standard/Fruity light/dark, 1080p and narrow windows, keyboard/focus, and repeated
  open/close without leaked listeners, requests or browser sessions.
- Run four fixed live topics (current events, technical comparison, historical
  question, conflicting-source investigation) at all three effort levels with
  the configured primary model: **12 documented runs**. Repeat selected cases
  with an available weaker/failover model. Record latency, tool/LLM usage, cached
  tokens where exposed, retrieved/cited sources, citation defects, gaps and
  export results. Keep failures visible; more effort is not accepted merely for
  producing more words or calls.
- Run focused agent/tools/server/Office/UI regression checks, UI bundle check,
  the appropriate broader project suites, resource package build and a matching
  binary's `--check-assets`. No deployment claim without checking the running
  build ID.

The feature is complete when all three effort levels honor their budgets, useful
evidence survives interruption, citations refer to inspected sources, and each
report can be exported in all three formats without repeating the research.
