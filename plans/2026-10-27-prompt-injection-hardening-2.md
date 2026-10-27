# Prompt-injection hardening, round 2 (2026-10-27)

## Goal

Close the remaining gaps in AuraGo's prompt-injection defenses and extend the existing layers (scanner,
isolation, quarantine, action policy, memory persistence, ingress channels) without replacing the
Guardian-centric architecture. Round 1 is `docs/superpowers/plans/2026-06-28-prompt-injection-hardening.md`.

## Baseline (verified 2026-10-27 at 13de9d026)

- Regex/fuzzy Guardian on promptsec v0.1.0 (sanitizer: zero-width strip, confusables, padded base64 and
  `\xNN` decode), threat levels None..Critical. External tool output is withheld at ThreatHigh
  (`QuarantineNotice`), isolated below that; source output stays copyable until ThreatCritical.
- `IsolateExternalData`: Cf strip, angle-bracket lookalike folding, escape neutralisation, `[INST]` break,
  HTML escape; fixed `<external_data>` wrapper that a body cannot emit.
- Default-deny trust classification (`tool_output_trust.go`): unknown tools are external.
- Strict content scanner (4 KiB windows, 512 B overlap, 8 windows, fail closed, complete-only cache) and
  fail-closed PromptSec judge; both opt-in.
- Ingress gates block at ThreatHigh before isolation (Telegram, Discord, Rocket.Chat, A2A, MeshCore, SMS,
  AgentMail, webhooks, MCP server arguments).
- Action policy (on by default): HighRisk calls need approval interactively and an admin allow rule
  unattended; copied-span gate refuses verbatim copies of isolated output into HighRisk calls.
- Core memory fenced as context, instruction-like core facts rejected by keyword list, learnings grounded
  by verbatim human quotes.

## Method

Six security-agent auditors mapped one area each (scanners, tool-output pipeline, prompt and memory,
ingress, content tools, actions and exfiltration); three adversarial verifiers re-read the code and ran
scratch harnesses. 47 candidates: 39 confirmed or partial, 2 duplicates, none fully refuted; several were
narrowed (severity or scope). Raw results: workflow `wf_faf67dd4-6eb` (session transcript).

## Batches

Batches are cut by package ownership so wave-1 batches run in parallel worktrees. Wave 2 starts from the
merged wave-1 tip because batch D reuses the directive-shape classifier from batch A.

Shared rules for every batch: keep existing contracts unless the item says otherwise, update the owning
`AGENTS.md` line in the same commit when a contract changes, write the listed tests (plus a test for each
real fix), no extra property/mutation suites. Do not edit files another session has uncommitted in the
main checkout (see "Landing").

### Wave 1

**A — Scan engine and LLM guardian (`internal/security` only)**

| Id | Finding | Fix contract |
| --- | --- | --- |
| A1 | scanners-1, -3, -2 (dehomoglyph hides Cyrillic rules) | Build a scan view before `protector.Analyze` in `scanWithOptions`: every Cf rune and zero-width separator becomes one ASCII space (runs collapsed), then NFKC (`golang.org/x/text/unicode/norm`). Scan the raw text and the view; the result is the maximum. The isolated body is unchanged. A scan view never deletes separators. |
| A2 | content-1 | For external (and source) output that is valid JSON, also scan the decoded string leaves joined by newlines; withhold on the maximum. Role-marker stripping applies to decoded leaves too. |
| A3 | scanners-4 | Extra decode shadows in the scan view: unpadded and URL-safe base64 runs (>= 16 chars, decoded text must be mostly printable), percent-decoding when `%` is present, HTML numeric entities. Each shadow is scanned as an extra window at decode severity 0.7. |
| A4 | scanners-2 | AuraGo-owned `CustomPatterns` passed from `buildPromptSecGuards`: override and prompt-disclosure phrases for it, pt, pl, nl, tr, ru, zh, ja, ko (>= 0.8) and the chat-template families `<|start_header_id|>`, `<|eot_id|>`, `<start_of_turn>`, `<|endoftext|>`, `<tool_call>`. Correct the "multilingual" comment to the real coverage. |
| A5 | prompt_memory-6 | Exported deterministic `DirectiveShape(text)` classifier: an imperative combining a tool, secret or egress term with a durability/scope term (English and German). `SanitizeToolOutput` adds its sanitized category to the warning suffix for external output (no withholding). Batch D reuses it for memory writes. |
| A6 | content-4, scanners-10c | External output scanned at ThreatMedium (or with a DirectiveShape hit) gets the `[SECURITY WARNING: ...]` suffix with a sanitized category, still exactly equal to `ToolOutputWarning` for that output; Info log with category only. |
| A7 | pipeline-5, scanners-10a | `sourceBoundaryRisk` falls back to the escaped form on special-token shapes (`<|word|>`, `<start_of_turn>`, `<end_of_turn>`); bare `<|` stays copyable. `hardenExternalBoundary` breaks `[TOOL_CALL]`/`[/TOOL_CALL]`. |
| A8 | scanners-7 | Tool-guardian prompt isolates every PARAMS value and CONTEXT with `IsolateExternalData`, states they are untrusted data, and tool verdicts use a strict one-line grammar; anything else is fail-safe, never Allow. |
| A9 | scanners-8 | `LLMGuardian.Judge` floors severity by decision: block >= 0.8, quarantine >= 0.5. |
| A10 | scanners-9 | `GenerateCacheKey` is injective over operation, context snippet and parameters (length-prefixed fields). |
| A11 | scanners-5 | Routine fast path parses argv and accepts only read-only shapes (no `find -delete/-exec*/-ok*/-fprint*/-fls`, no `rg --pre*`, `ip route|link|addr` only with show/list/get, `hostname` without arguments). |
| A12 | scanners-10d | Bound the regex scan work per call. Measure first; only text that was scanned may reach the model (if the model-facing part is already bounded, cap to it; otherwise incomplete quarantine beyond the cap). Avoid the duplicate full scan of `web_scraper` output. |

**B — Action policy and egress (`internal/agent` policy files, `agent_parse.go`)**

| Id | Finding | Fix contract |
| --- | --- | --- |
| B1 | actions_exfil-3, ingress-2 | `DescribeAction`: `manage_missions` create/add/update/edit/run/run_now/execute and `cron_scheduler`/`schedule_cron`/`manage_schedule` add/enable/update are HighRisk with operation `schedule`. |
| B2 | ingress-3 | `home_assistant` call_service/service on lock, alarm_control_panel, cover, script, automation, shell_command, siren, valve domains is HighRisk; docker stop/pause is HighRisk. |
| B3 | actions_exfil-6 | Per-tool table of external-reach operations that are HighRisk (here_now update/duplicate/update_access/set_password, onedrive share, github create_or_update_file/create_issue/comment-like ops, and equivalents found in the other publishing integrations); a test walks the operation enums. No global `update` keyword. |
| B4 | actions_exfil-2 | `dns_lookup`, `network_ping`, `port_scanner` are HighRisk when a host label is longer than 24 characters, looks high-entropy, or the call copies an isolated span. |
| B5 | pipeline-2, actions_exfil-5 | Copied-span index holds the full isolated payload under a per-session byte budget (256 KiB, oldest evicted) instead of 3 bodies x 1500 runes; both sides are normalised (HTML unescape, JSON string decode, whitespace runs to one space) before the 48-rune window test. |
| B6 | ingress-1 | For autonomous run types, the copied-span check also runs on the URL of `web_scraper`, `web_capture`, `api_request` (any method), `browser_automation` navigate and other fetch tools, and on DNS host labels; a hit returns `action_rule_required`. |
| B7 | actions_exfil-4 | Per-session record of external tools whose (non-quarantined) output reached the model recently; shown in the approval preview and appended to `GuardianCheck.Context`. Not a gate. |
| B8 | pipeline-3 | LLM Guardian BLOCK message to the model uses fixed category text; the judge reason goes to the log only. |
| B9 | pipeline-6, ingress-8 | Dispatch never skips `SanitizeToolOutput`: a nil Guardian falls back to `security.NewGuardian(nil)`. |
| B10 | actions_exfil-9 | Egress-class calls whose arguments contain a registered secret are refused with a fixed message (hash logged); arguments matching only the redaction regexes make the call HighRisk. |
| B11 | actions_exfil-8 | Copied-span approval previews include the body byte length and SHA-256. |

**C — Discovery and MCP metadata (`internal/agent` discovery files)**

| Id | Finding | Fix contract |
| --- | --- | --- |
| C1 | pipeline-1, content-5 | `get_tool_info` fences every annotation string leaf of `Parameters` (description, title, examples, default, string enums) with `IsolateExternalData`; keys, types, required and properties stay raw JSON. `discover_tools_fence_test` and the `internal/agent/AGENTS.md` "schema JSON stays raw" sentence change together. |
| C2 | content-5 | MCP catalog ingestion (`discovery_run.go`) scans the description and every schema string leaf locally (scan view + DirectiveShape once A lands; plain local scan is enough here); ThreatHigh hides the entry with fixed `HiddenReason` `suspicious_description`. Over-long descriptions are capped. |

### Wave 2 (from the merged wave-1 tip)

**D — Memory persistence**

| Id | Finding | Fix contract |
| --- | --- | --- |
| D1 | prompt_memory-1 | `isCoreMemoryInstructionLike` also rejects `security.DirectiveShape` hits (covers `remember` and `manage_memory add`). |
| D2 | prompt_memory-1 | `handleRememberWritable` runs `Guardian.ScanForInjectionLocal` on the content regardless of the LLM Guardian toggle; durable targets (core, fact) are denied at ThreatMedium, journal/task at ThreatHigh. |
| D3 | prompt_memory-2 | Memory analysis requires a verbatim human quote per fact, preference, correction and pending action (normalised substring of the current human message), like learnings; pending-action triggers must be grounded. KG excerpt marks assistant lines as non-source context. DirectiveShape hits are dropped. |
| D4 | prompt_memory-4 | `manage_memory add` goes through `AddModelCoreFact` (origin row, `[trust:agent-recorded]`); the RETRIEVED MEMORIES header explains the trust labels once. |
| D5 | prompt_memory-5 | Memory-reflection issues get their own advisory kind: no "REQUIRED USER NOTICE", no supervisor wording, never prepended to the answer. |

**E1 — Ingress channels**

| Id | Finding | Fix contract |
| --- | --- | --- |
| E1a | ingress-5 | MQTT relay scans topic + payload locally; ThreatHigh delivers only a `QuarantineNotice` (webhook pattern). |
| E1b | ingress-6 | `agentmail.allowed_senders` (address or `@domain`); relay with an empty list delivers only the quarantine notice; config validation warns when relay is on and `scan_emails` is off. Config UI field with all locales. |
| E1c | ingress-4 | A2A `UserIntent` is the isolated text; the co-agent denylist also covers `manage_webhooks`, `save_tool`, `create_skill_from_template`, `set_skill_documentation`, filesystem writes and scheduling tools. |
| E1d | ingress-7, ingress-8 | SMS ingress passes no `HumanInput` (channel evidence instead); `prepareIncomingSMSAgentInput` never returns raw text when quarantined. |
| E1e | ingress-7 | Telegram forwarded messages and Discord referenced messages get a fixed "forwarded/quoted from <origin>" marker inside the isolated block. |
| E1f | actions_exfil-1 | Agent answers sent to Telegram disable web page previews; Discord answers use suppress-embeds. |
| E1g | prompt_memory-3 | The compression summary is scanned before it is stored (keep the previous summary at ThreatHigh or a DirectiveShape hit). The recap moves out of the system role where the provider layer allows it (check strict chat-template adaptation first); otherwise it stays system-role but scanned. |

**E2 — Content tools and flows**

| Id | Finding | Fix contract |
| --- | --- | --- |
| E2a | content-3 | `SummariseContent` scans the raw source before the helper call (ThreatHigh: quarantine notice, no helper call) and isolates the model-written search query. |
| E2b | content-6 | `fallbackVisionAnalysis` isolates and scans the vision text; ThreatHigh drops it. |
| E2c | content-2 | `flowLLM.Step` scans the rendered ai.step prompt (ThreatHigh: FLOW error, no provider call) and re-isolates untrusted leaves spliced into it. |
| E2d | actions_exfil-7 | Publish lint: untrusted templated data in the scheme or host position of a URL parameter is an error; query/body positions stay warnings. |

## Decisions taken (secure defaults)

- Mission/cron creation and edits now need approval in interactive chats (B1): an agent-authored
  persistent prompt is the main persistence vector.
- Home Assistant HighRisk is limited to security-relevant domains (B2) so lights, switches and media keep
  working without prompts.
- Link previews are disabled for agent answers on Telegram and Discord (E1f).
- The flows S2 decision (2026-10-25: untrusted data into command/code sinks stays a warning) is unchanged;
  E2d adds only the URL host/scheme position.

## Deferred (follow-ups, need a product call or larger UX)

- scanners-6: chunked judging of long execute/send parameters (opt-in guardian; cost and UX design).
- pipeline-4: persisted MCP description/schema pinning with an admin re-approve flow (C2 re-scans every refresh meanwhile).
- A2A API key in the query string (the A2A agent card allows `in: query`; removal breaks peers).
- Explicit tool allowlist for mission runs (B1 closes the creation path).
- prompt_memory-4 remainder: `source_channel` provenance column and an `external-derived` label.
- prompt_memory-7: digest pinning of operator-owned disk prompt files.
- content-7: LLM judge stays off by default (documented in `internal/security/AGENTS.md`).
- An outbound host allowlist (`egress.allowed_hosts`).

## Landing

Work happens in `.worktrees/pi-hardening` (branch `claude/prompt-injection-hardening-2`) and per-batch
worktrees. Another session has uncommitted TTS/memory/chat edits in the main checkout; batches must not
edit these files: `internal/agent/agent_loop.go`, `agent_loop_helpers.go`, `bluetooth_tool.go`,
`dispatch_platform.go`, `internal/memory/{chroma,corpus_embedding_pass,document_corpus,embedding_blocked_retry,embedding_retry,graph_semantic,index_batches,long_term}.go`,
`internal/server/{agodesk_handlers,chat_voice_output,cyd_glass_speech,cyd_handlers,sip_voice,tts_routes}.go`,
`internal/tools/{chromecast,notification,tts,tts_memory}.go`, `internal/tools/AGENTS.md`, `internal/cyd/*`,
root `AGENTS.md`, `ui/AGENTS.md`, `ui/cfg/tts.js`, `ui/js/chat/*`, `ui/css/chat*`. Land with rebase +
`merge --ff-only` once those paths are clear.

## Verification

Per batch: the listed tests plus `go test` of each touched package. After each wave: `go build ./...`,
`go vet` of touched packages, and the package suites of `internal/security`, `internal/agent`,
`internal/server`, `internal/flows`, `internal/memory`, `internal/tools` as touched.
