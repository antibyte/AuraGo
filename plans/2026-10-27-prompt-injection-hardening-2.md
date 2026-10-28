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

## Status (2026-10-28)

- Branch `claude/prompt-injection-hardening-2`, HEAD `55dff579d`, on top of `13de9d026`. The plan commit is
  `a2339dd31`. Not merged to `main`, not pushed.
- Every row below carries its status as built: `done`, `partial` (what is open is named in the row) or
  `skipped` (blocked by a file that another session has dirty in the main checkout). Open items are also
  listed under "Deferred" and, for the four dirty files, under "Pending lines for files dirty in main".
- Commit hashes are the branch's own. Cherry-picked batch commits have new hashes; the batch worktree
  hashes are not used.
- Landing waits for the `ui/lang/backend/*.json` overlap with the main checkout. See "Landing" and "Verification".

## Batches (as implemented)

### Wave 1

#### A: scan engine and LLM guardian (`internal/security`)

Commits: `bf680e932`, `e0fd87e18`, `c6b316610` (scan 1); `7abd9ee24`, `e4680449f`, `1d61ac637`,
`fc666065a` (scan 2); `8d17a8da4`, `c79ed29f5`, `06d85be38` (LLM); `052534185` (batch A merged into this branch by
fast-forward, with review fixes `3d1e99c5f`, `119fcd247`, `1bd6261c0` and the verdict-steering floor).

| Id | Finding | Status | As implemented |
| --- | --- | --- | --- |
| A1 | scanners-1, -3, -2 | done | `scan_view.go`: every Cf rune, the listed fillers and other Unicode spaces become one ASCII space (runs collapsed), then NFKC. The raw text and the view are both scanned and the higher verdict counts. The isolated body is unchanged. Review fix `3d1e99c5f` added U+FE00-FE0F, U+180B-180D, U+180F and U+E0100-E01EF. |
| A2 | content-1 | done | For external and source output that is a JSON object, array or a valid string, the decoded string values (joined by newlines) and the distinct object keys (a second view) are scanned; the higher verdict counts. Role markers are stripped from the values too. Review fix: a top-level JSON string is one leaf. |
| A3 | scanners-4 | done (deviation) | Decode shadows per window: unpadded and URL-safe base64 of 16+ characters (decoded text must be printable), percent-escape clusters and HTML numeric-entity clusters. One shadow per encoding per window, every cluster decoded (`3d1e99c5f`). Shadow budget is 2x the scanned bytes plus four windows; exhausting it marks the scan incomplete. Deviation: only injection findings (instruction_override, system_prompt_leak, role_manipulation, delimiter_injection) are lifted to ThreatHigh; normalisation notes such as `encoding_attack` are never lifted. |
| A4 | scanners-2 | done | `auraGoInjectionPatterns()` passed as `CustomPatterns` from `buildPromptSecGuards`. Two tiers (`119fcd247`): strong, 0.85, withholds, for an override that reaches for everything or the assistant's own instructions and for a request for the system prompt; warning, 0.45 (Medium), for "previous instructions" without all or your, and for "system message" requests. Languages: it, pt, pl, nl, tr, ru, zh (both scripts), ja, ko. Chat-template tokens (`<|start_header_id|>`, `<|eot_id|>`, `<start_of_turn>`, `<|endoftext|>`, `<tool_call>`) at 0.9. Coverage is stated in `internal/security/AGENTS.md`. |
| A5 | prompt_memory-6 | done | `DirectiveShape(text)` in `directive_shape.go`. Scope markers (from now on, going forward, before replying, standing instruction, and German forms) need a tool name, a send verb next to a secret or destination, or a read verb next to a secret. Repeat markers (always, whenever, immer, and so on) need a sentence that commands the tool or a send verb next to a secret or destination (`1bd6261c0`). Third-person recaps of tool use are not hits (`1a644ed16`). Used for the warning line, the compression screen and `GateScan`; the memory-write gate uses `memory.StandingOrder` instead (D1). |
| A6 | content-4, scanners-10c | partial | External output at ThreatMedium (detector categories, `unspecified` without one), with a DirectiveShape hit, or with relieved bare phrases gets the `[SECURITY WARNING: ...]` line and is not withheld. `ToolOutputWarning` is exact for source output and for clean external output. For flagged external output it is a prefix of the line, because only a scan of the original text can name the categories (decision below). CYD Creator's sizing still calls the cheap prefix. |
| A7 | pipeline-5, scanners-10a | done | `sourceBoundaryRisk` falls back to the escaped form for `<\|word\|>`, `<start_of_turn>` and `<end_of_turn>` (also as entity or escape spellings); a bare `<\|` stays copyable. `hardenExternalBoundary` breaks `[TOOL_CALL]` and `[/TOOL_CALL]` (`7abd9ee24`). |
| A8 | scanners-7 | done | Tool guardian: PARAMS values, CONTEXT and the AGENT JUSTIFICATION are sanitised and isolated, and both system prompts say they are data. The reply must be one line in the strict grammar (`c79ed29f5`); anything else is a fail-safe quarantine. A verdict steered by the call's own text is floored to quarantine (`052534185`, `verdict_steering.go`). |
| A9 | scanners-8 | done | `LLMGuardian.Judge`: block gives at least 0.8 (ThreatCritical), quarantine at least 0.5 (ThreatHigh); safe keeps its score (`8d17a8da4`). |
| A10 | scanners-9 | done | `GenerateCacheKey` and `generateToolCacheKey` length-prefix every field (`8d17a8da4`). |
| A11 | scanners-5 | done | `routine_command.go`: `splitRoutineArgv` reads argv without a shell and accepts exact read-only shapes only (`06d85be38`). |
| A12 | scanners-10d | partial | Measured first: 16 KiB 0.15 s, 256 KiB 4.4 s, 1 MiB 17 s, 5 MiB about 3.5 min on the development host. External output above 256 KiB is clipped to a UTF-8-safe prefix with a note, the prefix is scanned and delivered like ordinary output, and the rest is never delivered (`1d61ac637`; I1 `3bfe31d76` replaced the withholding; project source `55dff579d`). Duplicate scan: `ScanExternalContent` skips its advisory scan above the cap and when the Guardian has no logger; the scraper's own call stays because the scraper is outside this scope. |

#### B: action policy and egress (`internal/agent` policy files)

Commits: `0949bf508`, `cd44d9d7d`, `e3907c520`, `3c6bd2b2a`, `bb81ba364`, `88ca67f07`, `6acf65eda` (batch B and its review fixes).

| Id | Finding | Status | As implemented |
| --- | --- | --- | --- |
| B1 | actions_exfil-3, ingress-2 | done | `schedulingActionHighRisk` in `action_policy_risk.go`: manage_missions create, add, update, edit, run, run_now and execute; cron_scheduler and manage_schedule add, enable and update; every schedule_cron call. Operation `schedule` (`0949bf508`). follow_up and wait_for_event are HighRisk in non-interactive runs only (`bb81ba364`, pinned by `6acf65eda`; decision below). |
| B2 | ingress-3 | done | home_assistant call_service and service on lock, alarm_control_panel, cover, script, automation, shell_command, siren and valve, plus the generic domains homeassistant, hassio, rest_command, python_script and pyscript. docker stop and pause are HighRisk; docker start is not (decision below). |
| B3 | actions_exfil-6 | done | `externalReachOperations`: a per-tool table with no global `update` keyword. `bb81ba364` added s3_storage copy and move, homepage webserver_start, agentmail_inboxes create_inbox, virtual_workspace request_credential_grant and virtual_computers run_shell_task and run_desktop_task. `unattendedReachOperations` (virtual_workspace exec, start_job, job_input; virtual_computers exec) is HighRisk only unattended. `TestExternalReachOperationsAreClassified` denies by default. |
| B4 | actions_exfil-2 | done | `probeHostLooksLikeExfiltration`: a label over 24 characters; a label, or a run of several short labels left of the last two, that reads as encoded text (16 or more digits; hex with letters and digits of 12 or more; a segment of 10 or more characters whose letters and digits alternate at least four times; 16 or more characters with few vowels); a name over 64 characters. A host that copies a 48-character span of earlier isolated output is HighRisk in every run type (`copiedFetchSpan`, `bb81ba364`). The copy gate then asks interactively and refuses unattended, and an allow rule does not lift the refusal. The shape test is a heuristic; a host allowlist is deferred. |
| B5 | pipeline-2, actions_exfil-5 | done | The copied-span index keeps the full isolated payload per session (256 KiB, least recently used sessions evicted first, 16 MiB and 256 sessions overall; `3c6bd2b2a`). Both sides are normalised (HTML unescape, JSON string decode, whitespace runs). Percent-decoded argument strings (up to three rounds, plus as space) are matched too. |
| B6 | ingress-1 | done | `copied_span_fetch.go`: in unattended runs the URL or host of a fetch tool is checked against earlier output. Destinations come from the argument names (`fetchTargetsFromArguments`: url, *_url, host, domain, path, endpoint, target), not from the tool name (`88ca67f07`). `TestEveryDestinationParameterIsAFetchTarget` walks the builtin schemas. |
| B7 | actions_exfil-4 | done | `recordIsolatedModelOutput` notes eight external tools whose non-quarantined output reached the model, for one hour. The approval preview shows them (`_approval_untrusted_reads`) and so does the LLM Guardian context. Not a gate. |
| B8 | pipeline-3 | done | `classifyGuardianBlock`: the model gets a fixed category. The judge's reason goes to the log only (`cd44d9d7d`). |
| B9 | pipeline-6, ingress-8 | done | `fallbackToolOutputGuardian` (`security.NewGuardian(nil)`, shared) is used when the dispatch has no Guardian, so sanitising is never skipped (`cd44d9d7d`). |
| B10 | actions_exfil-9 | done | Registered secrets in egress arguments are refused with `egress_secret_refused` (hash logged); no rule or approval lifts it. Identities (`security.RegisterIdentity`) only redact: `egressArgumentSecrets` asks `Scrub` and then `ScrubSecrets`. The SMTP and IMAP user name is an identity (I3, `7aa4f7660`, through `registerLogin`), and so are the ONVIF camera user name, the go2rtc discovery user name and the Tuya account e-mail (`7d3d66b81`). This completes the partial `88ca67f07`. A credential that looks like an address and was registered with `RegisterSensitive` is still refused. |
| B11 | actions_exfil-8 | done | Copied-span approval previews start with `_approval_copied_span`: call bytes and SHA-256 of the whole call, and `long_text_fields` (up to four text fields of 1 KiB or more) (`e3907c520`). Review minor: only the four largest fields are listed (deferred). |

#### C: discovery and MCP metadata (`internal/agent` discovery files)

Commits: `bd9dd922c` (C1), `339fec811` (C2), `8543a0982` (decision by scan).

| Id | Finding | Status | As implemented |
| --- | --- | --- | --- |
| C1 | pipeline-1, content-5 | done (threshold changed) | `get_tool_info` deep-copies the Parameters schema and fences every annotation string leaf (default-deny walk, `bd9dd922c`). Review fix `8543a0982`: value keywords (enum, const, default, examples, pattern) are judged together by `schemaScan` (`discovery_schema_scan.go`) with the run's Guardian, and fenced at ThreatHigh. Benign values stay byte-exact. The token-shape rule was removed. |
| C2 | content-5 | partial | `mcpCatalogEntry` and `vetMCPMetadata` scan the description and the schema strings locally. `8543a0982`: ThreatHigh no longer hides a tool. An injection in a property name hides it with `suspicious_description`. An injection in the description or in values keeps the entry enabled with `external_tool_flagged`; the tool stays callable, its text is fenced, and the invoke instruction warns. Names above the scan budget hide the tool (`TestMCPNamesAboveTheScanBudgetHideTheTool`). Not covered: package__ and skill__ or tool__ entries (`discovery_run.go`), MCP tool names and aliases, and `mcp_call` with operation `call_tool` (`dispatch_platform.go`, outside this scope). |

#### Integration fixes (after wave 1; the integration run's head was `e21c957bb`)

| Id | Source | Status | As implemented |
| --- | --- | --- | --- |
| I1 | A12 review | done | `clipToScanLimit`: `SanitizeToolOutput` clips external output above 256 KiB instead of withholding it, scans the prefix, and appends the note. ThreatHigh in the prefix or an incomplete scan still withholds (`3bfe31d76`). |
| I2 | C13 test breakage after B9 | done | `flowOpenDispatchLayer` removes exactly the dispatch layer in the flow tool invoker (`7eb87b877`). The dispatch always wraps since B9, so the invoker's `dc.Guardian != nil` assumption broke the Ollama and SSRF refusal mapping, and web.read and doc.pdf_read stopped reading the tool's JSON. Four server tests pass again. |
| I3 | identities vs secrets | done | `security.RegisterIdentity` and the secrets-only `ScrubSecrets` (`7aa4f7660`); see B10. |
| I4 | identities for camera and Tuya | done | `7d3d66b81` registers the ONVIF camera user name, the go2rtc discovery user name and the Tuya account e-mail as identities. `38784be42` gives the execution isolation test a fixture the scanner does not withhold (a pre-existing failure). |

### Wave 2 (from the merged wave-1 tip)

#### D: memory persistence

Commits: `b79ee4a42`, `718d52284`, `245c99e95`, `c4e6dc5fa`, `cf7c43b22`, `b1c7d9ea9`, `b89682b26`, `8d713f7e6`.

| Id | Finding | Status | As implemented |
| --- | --- | --- | --- |
| D1 | prompt_memory-1 | done (mechanism changed) | `isCoreMemoryInstructionLike` and `ValidateCoreMemoryFact` ask `memory.StandingOrder` (`internal/memory/standing_order.go`, `b89682b26`). The first version used `security.DirectiveShape` (`b79ee4a42`), which did not cover recurrence wording. StandingOrder takes the strongest marker of a sentence, or of two neighbouring sentences: a recurrence per turn with a tool, secret, URL or address; a persistence marker with a tool, a send to a secret, URL or conversation, or a read of a secret; a bare always or immer with a secret sent or printed. It covers remember, manage_memory add and update, correction previews, learned rules and the prompt-side filter. |
| D2 | prompt_memory-1 | done | `rememberContentRefused` runs the local screen with the dispatch Guardian (`ScanForInjectionLocal`, the package fallback when there is none), whatever the LLM Guardian setting (`718d52284`). Durable targets (core, fact) refuse at ThreatMedium or on a StandingOrder hit; journal, task and note refuse at ThreatHigh; knowledge-graph edges are screened too (Source, Target, Relation, content, property keys and values, with length bounds of 200 and 80 runes). `dispatchExec` screens manage_memory add and update before `ManageCoreMemory` (`b89682b26`). Review fix `5c7f5d617` applies the bare-phrase relief, and the closed benign form `rememberReduceBenignRole` lets "always answer in German" through. |
| D3 | prompt_memory-2 | done | `groundMemoryAnalysisResult` (`memory_analysis_grounding.go`, `cf7c43b22`): every item needs a verbatim quote of at least three words and 16 runes (8 for Han and Kana text). `memory.DerivedFrom` requires three of five distinctive words of the item's content, title and summary to occur in the quote (`8d713f7e6`). A pending trigger of at least 3 runes must be verbatim, and an item that is a StandingOrder hit is dropped. Personality profile values must be derived from the human text. The knowledge-graph excerpt labels assistant lines source=false (prompt-level only). `helper_llm_manager.go` batched prompts ask for quotes too (a scope extension). |
| D4 | prompt_memory-4 | partial | `manage_memory add` goes through `AddModelCoreFact` with the session ID (origin row, `[trust:agent-recorded]`, `245c99e95`). `ReadCoreMemory` shows the label and the RETRIEVED MEMORIES note explains the trust labels once (`retrievedMemoryTrustNote`). Open: manage_memory update still invalidates the provenance label (an update path that rewrites the origin row is needed); the label has no explanation in the minimal tier; the `source_channel` column and the `external-derived` label stay deferred. |
| D5 | prompt_memory-5 | done (changed) | The advisory marker is `OperationalIssueNotice.Advisory`, derived from the memory_reflect source (`b1c7d9ea9`), not a new stored kind (the dashboard would need filter labels in all locales). The reminder carries `OperationalIssueAdvisoryPrefix` as a string marker in ContextFlags; a dedicated field is deferred. Shown only through typed chat delivery; Telegram, Discord and SMS keep them pending. |

#### E1: ingress channels

Commits: `134a3019d`, `dd1c17ccb`, `9973fc561`, `12f6eba06`, `752657f5b`, `4da4e67ae`, `bf3d98671`, `515426963` (batch E1); `a21a29d83`, `7fc97abac`, `1a644ed16` (review fixes).

| Id | Finding | Status | As implemented |
| --- | --- | --- | --- |
| E1a | ingress-5 | done | MQTT relay: `mqttRelayText` (topic and payload, cut to the 16 KiB prompt limit) is scanned locally with `ScanForInjectionLocal` after the debounce, and the LLM toggle does not matter. ThreatHigh delivers a QuarantineNotice (`9973fc561`, `752657f5b`). MQTT mission triggers (review fix `7fc97abac`): `missionMQTTAdapter.admitDelivery` scans the whole payload before the mission starts; ThreatHigh drops and logs the trigger, and the interval is not consumed. Decision: drop and log, not a notice payload. |
| E1b | ingress-6 | done (one open point) | `agentmail.allowed_senders`: a full address or `@domain`, matched case-insensitively on the parsed From address. With the relay on and an empty list, every message becomes a QuarantineNotice with no content, sender or label. With entries, other senders are ignored without a notice. CheckSecurity warns with `agentmail_relay_unscanned` (scan_emails off) and `agentmail_relay_no_senders`. The config UI field (`ui/cfg/agentmail.js`) and 16 locale files exist. Open: malformed entries are dropped silently in `normalizeAllowedSenders`, with no config warning (`12f6eba06`). |
| E1c | ingress-4 | partial | A2A `UserIntent` is the isolated peer text, and the A2A run config sets `allow_filesystem_write` to false (`dd1c17ccb`). The co-agent denylist adds manage_webhooks, save_tool, create_skill_from_template, set_skill_documentation and the scheduling tools. Review fallback (`a21a29d83`, instead of the allowlist): `a2aRestriction` for A2A runs only denies code execution (shell, Python, sandbox, sudo, remote shell, run_tool, execute_skill, run_agent_skill_script, and the git and remote_execution aliases), virtual_desktop writes except the read operations, and homepage operations that `HomepageOperationMutates` names. Not done: the default-deny A2A allowlist (`ToolScopeRestricted`, `a2a.server.allowed_tools`) and removal of the query-string API key (deferred). Filesystem writes are denied for A2A runs only; local co-agents keep their file tools. |
| E1d | ingress-7, ingress-8 | done (deviation) | SMS: a non-quarantined SMS gets no `HumanInput`. There is no channel-evidence kind, so none is recorded instead. `prepareIncomingSMSAgentInput` never returns raw text when quarantined, and the SMS helper refuses raw text itself (`134a3019d`). |
| E1e | ingress-7 | done | A forwarded Telegram message (forward_* fields) gets `[Forwarded from <origin>; third-party content]` before its text, inside the isolated block and before the scan. A referenced Discord reply gets the same marker (`bf3d98671`). The routing text of a forwarded message starts with the marker (F3, `ea164abc9`, `telegramForwardedRoutingText`). Telegram reply quotes are not delivered by the library, so they need no marker. |
| E1f | actions_exfil-1 | partial | Done: `sendTelegramMessage` sets DisableWebPagePreview on every chunk (`4da4e67ae`); Discord `agentTextMessage` and `agentFileMessage` suppress embeds with captions kept (F3); the newspaper digest and fallback chunks disable previews (`newspaperTelegramText`, F3). Open: `internal/tools/notification.go` `sendTelegramNotification` (send_notification, person-detection text alerts, the overflow text of SendTelegramFile) still unfurls links. Rocket.Chat previews are a platform setting and cannot be checked here. The file is dirty in the main checkout; see the pending lines. |
| E1g | prompt_memory-3 | partial | `compressPersistentHistory` runs `screenGeneratedSummary` before any mutation: local Guardian at ThreatHigh (an incomplete scan counts as High), DirectiveShape or StandingOrder rejects the summary. The rejection counts as `summary_rejected` with a one-minute cooldown, and the previous summary and every source message stay in place (`515426963`). Third-person recaps of tool use pass (`1a644ed16`). Open: the recap stays in the system role (moving it needs a change in `request_budget.go` and `prepared_prompt_history.go`, and a provider decision); the transient compression summary and summaries stored before the screen are not screened; there is no read-side check. |

#### E2: content tools and flows

Commits: `a62c9fb05`, `ee0b1af02`, `08620a09d`, `ad6fbcacd`, `3e564690c`, `bbefa8c1b`, `6ae3e8ab6`, `f83b723aa`, `6f24358d3` (batch E2 and its review fixes).

| Id | Finding | Status | As implemented |
| --- | --- | --- | --- |
| E2a | content-3 | done (deviation) | `SummariseContent` scans the part the helper would receive (`maxSummaryPromptContentChars`, 12000 bytes) with `security.GateScan` and the operator's Guardian (`GuardianFromConfig`). ThreatHigh or an incomplete scan returns a QuarantineNotice and makes no helper call. The model-written search query is isolated in `buildSummaryPrompt` (`a62c9fb05`). Deviation: the scan covers the helper's part of the source, not the whole raw source. |
| E2b | content-6 | partial | `fallbackVisionAnalysis` scans each analysis with `GateScan` and the operator's Guardian. ThreatHigh or an incomplete scan drops it and writes `visionFallbackWithheldNote`; anything else is isolated under a line that calls it untrusted (`ee0b1af02`). Open: the raw-pixel path (a multimodal main model receives image parts) is not scanned, and the attachment name is written raw into the fallback message (`multimodal_ingest.go`). |
| E2c | content-2 | partial | `checkFlow` keeps the lint's taint and passes it as `Env.Untrusted`. `ParamSpec.FenceUntrusted` is set on the ai.step prompt and instructions (`f83b723aa`), so untrusted values are fenced where they are spliced (`08620a09d`). `flowLLM.Step` scans the instructions and the rendered prompt with `GateScan` before any provider call; ThreatHigh or an incomplete scan fails the step with `FLOW_PARAM_INVALID` (`ad6fbcacd`). The layers of a tool result are described in `internal/flows/AGENTS.md`. Open: the decision state of ai.decision is not fenced (`decisions_media.go`). |
| E2d | actions_exfil-7 | partial | `ParamSpec.URLTarget` on http.request and web.read `url` and on generic url, uri, endpoint and `*_url` text parameters. The lint reports `UNTRUSTED_URL_TARGET` as an error at publish time when untrusted data reaches the scheme, user info, host or port (`3e564690c`). Path and query positions stay warnings. Deferred: the run-time refusal in `refusalsOf` (a product call, because the lint taints a node as a whole) and the classification of http.request under RunType flow in the internal/agent action policy. |

#### Review-driven batches F1 to F3 (cherry-picked on the branch)

| Id | Source | Status | As implemented |
| --- | --- | --- | --- |
| F1 | the bare-phrase finding of E2 (flow, summary and vision gates) and the tool-output jailbreak table | done | `GateScan`, `GuardianFromConfig` and `NewGuardianFromConfig` moved to `internal/security/scan_gate.go` (`2a0d487ad`); `internal/tools/scan_gate.go` was removed. `relieveBarePhrases` (`bare_phrase.go`) relieves each view of a scanned text separately; a text stays hostile only when the rescan still reaches the bar, the scan is incomplete, DirectiveShape hits, or the phrase hands over a role, mode, order or action (bareHandoff). Benign bare-phrase rows are delivered with the category warning; 18 jailbreak openers stay withheld. A QuarantineNotice omits an empty Reference line. The action vocabulary of the repair (`37594408e`) is part of this rule. |
| F2 | homepage and git alias gates (review of the alias path) | partial | `canonicalizeHomepageAliasCall` (`canonical_action.go`) rewrites a homepage alias to the focused tool that publishes the operation before any gate (`921a32771`). The late redirect in `dispatchInner` is removed, and the git and exec redirects there were dead code and are removed too. An alias-only grant denies; a scope that lists the focused tool admits the alias as a direct call would. Decision: list_files, read_file and write_file are no longer redirected; they end as unknown actions with a hint that names homepage_file and filesystem (open decision for review). Open: a direct call to the umbrella `homepage` action dispatches when the policy is off. |
| F3 | the E1e and E1f link-preview and forwarded-content paths | partial | Newspaper Telegram text disables previews (`newspaperTelegramText`). Discord SendDiscordImage and SendDiscordAudio suppress embeds through `agentFileMessage` (captions kept). The forwarded routing text (`telegramForwardedRoutingText`). No change needed: internal/persondetect (photo captions do not unfurl) and `tools/telegram_document.go` (captions). Skipped: `internal/tools/notification.go` (dirty in the main checkout, see the pending lines). |

### Review repairs (whole-branch review, six important issues)

| Id | Finding | Status | As implemented |
| --- | --- | --- | --- |
| R1 | flow invoker keeps the Guardian warning line | done | `flowOpenDispatchLayer` strips a trailing `[SECURITY WARNING: ...]` line before unwrapping the fence (`36c9175f7`). |
| R2 | benign "From now on" preferences refused by the remember screen | done | `rememberTextFindings` applies the relief at ThreatMedium for durable targets and ThreatHigh for the journal (`5c7f5d617`). |
| R3 | compression screen did not reject a standing order | done | `screenGeneratedSummary` also rejects a `memory.StandingOrder` hit; the logged reason is the gate's own category (`2b300c4dd`). |
| R4 | persona followed by an action was let through by the relief | done | `bareActionWords` (execute, shell, delete, upload, password, vault, without asking, and so on) keeps such a text withheld (`37594408e`). |
| R5 | oversize project source was delivered unscanned | done | Project source above the cap is clipped, scanned at the Critical bar and delivered in the escaped form (`55dff579d`). |
| R6 | Telegram link previews of notification.go | skipped | The file is dirty in the main checkout. The exact lines are under "Pending lines for files dirty in main". |

## Decisions taken (secure defaults and decisions made during implementation)

- Mission and cron creation and edits need approval in interactive chats (B1): an agent-authored persistent
  prompt is the main persistence vector. Unattended runs need an allow rule with operation `schedule`.
- Operation names changed with B1: an admin allow rule keyed on the old `cron_scheduler` add or
  `manage_missions` create no longer matches, and the upgrade note is in `documentation/action-policy.md`
  and the release notes.
- follow_up and wait_for_event are HighRisk only in non-interactive runs (follow-up, mqtt, heartbeat, mission
  and other background runs). An interactive chat keeps them ordinary, because a chain of follow-ups is how
  the agent works through a long task. A self-continuing unattended chain needs an allow rule for the tool,
  operation `schedule` and target `tool:follow_up` or `tool:wait_for_event`.
- Home Assistant HighRisk is limited to security-relevant domains (B2), so lights, switches and media keep
  working without prompts. The generic homeassistant, hassio, rest_command, python_script and pyscript domains
  are added because they reach any entity or run code. docker start is not HighRisk; stop and pause are.
- Link previews are disabled for agent answers on Telegram and Discord (E1f). Discord file and audio sends
  suppress embeds and keep their captions.
- The flows S2 decision (2026-10-25: untrusted data into command or code sinks stays a warning) is unchanged;
  E2d adds only the URL scheme, user info, host and port position, as a publish error.
- A6 warning: `ToolOutputWarning` heads the combined line and the categories of the original-text scan follow
  it. Exact equality holds only for source output and clean external output.
- A12 and I1: output above 256 KiB is clipped to a scanned prefix with a note, not withheld. Source output
  above the cap takes the escaped form. The text beyond the clip is never delivered.
- A4: the injection rules have two tiers. Manual-style phrases in these languages produce a warning line,
  not a withhold.
- A3: shadow decoding lifts only injection findings. A normalisation note (a Cyrillic or invisible-character
  finding) never withholds.
- B4: DNS and probe hosts that copy a span of earlier output are HighRisk in every run type. An unattended run
  refuses them even with an allow rule.
- B5: the copied-span store evicts the least recently used session first, and keeps 256 KiB per session.
- B10: identities and secrets are separate. Mailbox user names, camera user names, the go2rtc discovery user
  name and the Tuya account e-mail are identities: redacted, not refused. Values registered with
  `RegisterSensitive` are refused whatever they look like. The mailbox-address heuristic is removed.
- AgentMail: an empty `allowed_senders` with the relay on means quarantine notices only (E1b). The From header
  can be forged, so the list narrows who reaches the agent and does not authenticate the sender.
- MQTT mission triggers: a ThreatHigh trigger is dropped and logged, and its interval is not consumed (E1a).
- Memory gate: `memory.StandingOrder` is the durable-write gate and the profile gate. `security.DirectiveShape`
  warns on external tool output, screens compression summaries and feeds `GateScan`; it does not gate memory
  writes.
- Memory extraction needs quotes: three words and 16 runes (8 for Han and Kana), and `memory.DerivedFrom`
  (three of five distinctive words). The batched path takes the analyzer's user_request as the human text
  until agent_loop.go is free (pending line).
- Reflection advisories: the advisory flag comes from the memory_reflect source, not from a new stored kind.
  They are shown only through typed chat delivery (SSE), never as an answer prefix on other channels.
- A2A: the restrictions are a fallback set (`a2aRestriction`), not a default-deny allowlist. A2A runs have
  `allow_filesystem_write` off. The default set of an allowlist still needs a product decision.
- SMS: no HumanInput; there is no channel-evidence kind to record instead.
- Bare-phrase relief (F1): one rule, `relieveBarePhrases`, on every path that reads text a model is about to
  read (tool output, the flow ai.step, summary mode, the vision fallback and remember). A text is withheld only
  when it stays hostile after the relief. A relieved external output is delivered with the categories of the
  text as written. Source output is relieved at the Critical bar, which decides only the form, never delivery.
- The flow, summary and vision gates use the operator's guardian options (`GuardianFromConfig`); they scan
  locally and never call the judge.
- Homepage aliases are rewritten before any gate to the focused tool that publishes the operation. The alias
  name grants nothing. list_files, read_file and write_file are never guessed into Homepage (an unknown
  action with a hint).
- Core Memory: a standing order is refused, and an existing entry that reads like one is hidden from the
  prompt by the core-memory filter. Entries the agent recorded carry `[trust:agent-recorded]`.
- Compression summaries that hit the screen are rejected, with a one-minute cooldown and an operational issue
  on the second failure. The recap stays in the system role.
- Oversize project source output (Game Maker, CYD Creator) is clipped, scanned and delivered in the escaped form.
- The schema fence of MCP and tool schemas is decided by a scan at ThreatHigh, not by token shape (C1).
- Flagged MCP tools stay callable with `external_tool_flagged` (C2); only an injection in a tool or property
  name hides the tool (`suspicious_description`).

## Deferred (follow-ups, need a product call or larger UX)

From the original plan:

- scanners-6: chunked judging of long execute/send parameters (opt-in guardian; cost and UX design).
- pipeline-4: persisted MCP description/schema pinning with an admin re-approve flow (C2 re-scans every refresh meanwhile).
- A2A API key in the query string (the A2A agent card allows `in: query`; removal breaks peers).
- Explicit tool allowlist for mission runs (B1 closes the creation path).
- prompt_memory-4 remainder: `source_channel` provenance column and an `external-derived` label.
- prompt_memory-7: digest pinning of operator-owned disk prompt files.
- content-7: LLM judge stays off by default (documented in `internal/security/AGENTS.md`).
- An outbound host allowlist (`egress.allowed_hosts`).

Added during implementation and review:

- A2A default-deny allowlist (`a2a.server.allowed_tools`, `ToolScopeRestricted`) with a default set that needs a
  product decision, and removal of the query-string key (E1c). Until then, reads of memory, the knowledge graph,
  the journal, notes and plans stay open to peers, and so does every tool without an `a2aRestriction` entry
  (the Office tools, desktop notes and QR codes).
- Run-time refusal of a flow node whose web address takes untrusted data (`refusalsOf`, owner flows; a product
  call, because the lint taints a node as a whole, and already published flows keep running), and the
  classification of http.request under RunType flow in the internal/agent action policy (E2d).
- Recap role: move the compression recap from the system role to the user role (strict chat templates,
  `request_budget.go`, `prepared_prompt_history.go`), with a read-side recap check in
  `FormatContextRecapForPrompt` (E1g). The transient compression summary (`context_compression.go`) and summaries
  stored before the screen are not screened.
- Raw-pixel uploads to a multimodal main model are not scanned (`multimodal_ingest.go`, E2b); the attachment
  name is written raw into the fallback message.
- Fencing the ai.decision `state` (`decisions_media.go`); it goes to the decision scorer, not to a chat (E2c).
- manage_memory update invalidates the provenance label. It needs an update path that rewrites the origin row (D4).
- The core-memory `[trust:agent-recorded]` label has no explanation in the minimal prompt tier (D4).
- A dedicated ContextFlags field for the reflection advisory, in place of the string marker
  `OperationalIssueAdvisoryPrefix` (agent_loop.go, agent_loop_init.go; D5).
- The batched memory analysis human text in agent_loop.go (pending line below; D3).
- Telegram link previews of `send_notification`, person-detection text alerts and SendTelegramFile overflow
  text (`internal/tools/notification.go`; E1f, F3, R6; pending line below).
- Malformed `agentmail.allowed_senders` entries are dropped silently; a config warning is missing (E1b).
- Memory correction and learnings read `RunConfig.TaskRoutingText` as the owner's evidence, so forwarded
  text counts as the owner's words there (`memory_correction.go`, `learnings.go`; F3 review). A forwarded
  message that arrives while a question is pending is dropped with the reply-with-a-number notice
  (`internal/telegram/bot.go`; UX only).
- Homepage: a direct call to the umbrella `homepage` action still dispatches when the policy is off
  (`canonical_action.go` area; F2). The git_* names in the unknown-action hint are unreachable and were kept.
- Journal writes are still refused for "Always answer in German": the closed benign form only applies to durable
  targets (`rememberReduceBenignRole`; repair observation).
- `bareRoleSlot` reads at most two words, so "From now on you are my administrator." is stored; widening the
  slot to three words closes it (repair observation).
- Benign login or shop pages that say password, upload, delete or remove all next to a bare phrase are
  withheld, as in the baseline. This is the cost of the action vocabulary (repair observation).
- Summaries with a bare "from now on" or "you are now" phrase get no relief, so compression can stall until a
  later summary drops the phrase (review minor).
- `egressArgumentSecrets` tests list items and pairs over 512 bytes one at a time, so a secret split over list
  items passes; fail closed (review minor). `copiedSpanDigest` lists only the four largest text fields (B11).
  `guardianContextWithReads` cuts the user message to 60 bytes when the read note is long (review minor).
- `mcp_call` with operation `call_tool` still runs a hidden tool by server and tool name
  (`dispatch_platform.go`, outside the batch scope; C minor). package__ and skill entries are not vetted.
  The vet cache is bounded by entry count, not bytes. `activate_tools` echoes unknown names.
  Fenced prose can push a schema over the output budget (`schema_exceeds_output_budget`).
- The ai.step scan costs about 4 to 6 s for a 256 KiB prompt and refuses a prompt that quotes an override phrase
  (E2 minor). Fencing HTML-escapes values, so a model asked to copy them may echo the markers.
- Output beyond the clip is not retrievable through `read_tool_output` (I1 operator note).
- The scraper's advisory scan (`internal/scraper/scraper.go`, lines 286 to 288) still runs; the dispatch scan is the gate.
- Pre-existing, not caused by this round: `TestToolSearchTermsCoverEveryBuiltinTool` (qr_code has no search terms).

## Pending lines for files dirty in main

The main checkout has uncommitted work on these files, so this branch did not edit them. Apply the lines
below when each file is free, in the same commit as the code they describe. Other dirty files in the list
were not touched.

### internal/tools/AGENTS.md

The file has no Telegram, ManageCoreMemory or summary-mode line yet, so add three bullets.

Summary mode (E2a):

```
- Summary mode (`SummariseContent`, behind `summary_mode` of web_scraper, wikipedia_search, ddg_search and pdf_extractor) scans the source before the helper model: the part the helper would receive (`maxSummaryPromptContentChars`, 12000 bytes) goes through `security.GateScan` with `security.GuardianFromConfig(llmCfg.Config)`; ThreatHigh or an incomplete scan returns `security.QuarantineNotice` (`ContentScanQuarantine`) and makes no helper call. The model-written search query is isolated in `buildSummaryPrompt`. Verify `TestSummariseContentQuarantinesHighThreatSourceBeforeHelper`, `TestSummariseContentSummarisesPagesWithBarePhrases`, `TestSummariseContentScansWithTheConfiguredGuardian`, `TestBuildSummaryPromptIsolatesModelWrittenSearchQuery` and `TestGateScan*` (in `internal/security`).
```

Core memory (D2, D4):

```
- `ManageCoreMemory` add goes through `memory.AddModelCoreFact` with the dispatch session ID (last parameter), so a model-written core entry gets an origin row and `ReadCoreMemory` tags it `[trust:agent-recorded]`. `dispatchExec` screens manage_memory add and update with `rememberContentRefused` before the call; validation (including the `memory.StandingOrder` gate), the soft and hard entry cap and the "fact exists" answer are unchanged. Verify `TestManageMemoryAddWritesOriginRow` and `TestDispatchManageMemoryScreensAddAndUpdate`.
```

Telegram notification (E1f, F3 item 6): only with the notification.go change below.

```
- `sendTelegramNotification` (`send_notification`, the person-detection text alerts and the overflow text of `SendTelegramFile`) sends `disable_web_page_preview: true`, so Telegram does not fetch a link preview of a URL in the text. Verify `TestSendTelegramNotificationDisablesLinkPreviews`.
```

### ui/AGENTS.md

```
- The AgentMail config offers `agentmail.allowed_senders` as an `array-lines` textarea (`ui/cfg/agentmail.js`), with label and help in the 16 `ui/lang/config/agentmail/*.json` locales. Verify `TestConfigAgentMailAllowedSendersFieldAndTranslations` (`ui/config_agentmail_test.go`).
```

### internal/tools/notification.go

In `sendTelegramNotification` (about lines 329 to 333), the payload becomes:

```go
	// Telegram fetches a link preview from its own servers, so a URL in the text
	// would be requested from there. Previews stay off; the link stays clickable.
	payload, _ := json.Marshal(map[string]interface{}{
		"chat_id":                  chatID,
		"text":                     text,
		"parse_mode":               "MarkdownV2",
		"disable_web_page_preview": true,
	})
```

Run gofmt on the file (the value column moves to match the longest key). Test, appended to
`internal/tools/notification_test.go` (the file exists; add the imports `encoding/json`, `net/http` and
`net/http/httptest` if they are missing; `tg04Config` is defined in `telegram_document_hardening_test.go` and
returns `(*config.Config, string)`):

```go
func TestSendTelegramNotificationDisablesLinkPreviews(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer srv.Close()
	old := telegramAPIBase
	telegramAPIBase = srv.URL
	t.Cleanup(func() { telegramAPIBase = old })
	cfg, _ := tg04Config(t, "123:secret")
	if err := sendTelegramNotification(cfg, "t", "see https://evil.example/x"); err != nil {
		t.Fatal(err)
	}
	if got["disable_web_page_preview"] != true {
		t.Fatalf("disable_web_page_preview = %v, want true", got["disable_web_page_preview"])
	}
}
```

On the unfixed code the decoded body has only `chat_id`, `parse_mode` and `text` (measured in the F3 batch), so the test fails until the change lands.

### internal/agent/agent_loop.go

Batched memory analysis (D3). Immediately before the call at about line 2609,
`applyMemoryAnalysisResultWithContext(analysisCtx, cfg, s.currentLogger, shortTermMem, longTermMem, sid, batchResult.MemoryAnalysis)`,
copy the run's human text into the batched result, as `runMemoryAnalysis` does
(`memory_analysis.go`, lines 319 to 321):

```go
	if h := runCfg.learningAnalysisScope().HumanText; h != nil {
		batchResult.MemoryAnalysis.human = *h
	}
```

Not compiled. Check that `batchResult` is a local variable (not a map element) and that `human` is the
unexported field of `memoryAnalysisResult`, as in `helper_llm_manager.go`. The ContextFlags field for the
advisory marker stays deferred (it is in agent_loop.go too).

## Landing

Work happens in `.worktrees/pi-hardening` (branch `claude/prompt-injection-hardening-2`) and per-batch
worktrees. Another session has uncommitted TTS/memory/chat edits in the main checkout; batches must not
edit these files: `internal/agent/agent_loop.go`, `agent_loop_helpers.go`, `bluetooth_tool.go`,
`dispatch_platform.go`, `internal/memory/{chroma,corpus_embedding_pass,document_corpus,embedding_blocked_retry,embedding_retry,graph_semantic,index_batches,long_term}.go`,
`internal/server/{agodesk_handlers,chat_voice_output,cyd_glass_speech,cyd_handlers,sip_voice,tts_routes}.go`,
`internal/tools/{chromecast,notification,tts,tts_memory}.go`, `internal/tools/AGENTS.md`, `internal/cyd/*`,
root `AGENTS.md`, `ui/AGENTS.md`, `ui/cfg/tts.js`, `ui/js/chat/*`, `ui/css/chat*`. Land with rebase +
`merge --ff-only` once those paths are clear.

Status at 2026-10-28: the branch is not merged. The four files named in "Pending lines for files dirty in
main" (`internal/tools/AGENTS.md`, `ui/AGENTS.md`, `internal/tools/notification.go`, `internal/agent/agent_loop.go`)
get their lines in a follow-up commit once the other session has committed them; the branch is complete
without them, and the remaining Telegram preview gap closes with that follow-up. The batched memory analysis
grounds quotes in the `user_request` the helper saw until the `agent_loop.go` line lands. The only file overlap
with the main checkout's dirty state is `ui/lang/backend/*.json` (D5 keys at the top of each file, the other
session's keys about line 247), which makes `merge --ff-only` refuse until those files are committed or set
aside for the merge.

## Verification

Per batch: the listed tests plus `go test` of each touched package. After each wave: `go build ./...`,
`go vet` of touched packages, and the package suites of `internal/security`, `internal/agent`,
`internal/server`, `internal/flows`, `internal/memory`, `internal/tools` as touched.

What was run:

- Wave 1 and integration (head `e21c957bb`): `go build ./...` and `go vet` of security, agent and server are clean.
  Packages passing: security, flows, mailer, a2a, agentmail, telegram, discord, rocketchat, webhooks, config,
  audit, prompts, llm, scraper. `internal/server` passes except `TestGetDesktopServiceReusesCanonicalConfigWithRelativeDefaults`,
  a D:/C: path failure of the environment. `internal/agent`: `TestExecutionOutputIsolationSurvivesVaultAndCompression`
  was fixed after the run (`38784be42`); `TestToolSearchTermsCoverEveryBuiltinTool` (qr_code has no search
  terms) was not re-run. `internal/tools`: three Docker-daemon tests fail (no Docker daemon on this host; the
  same on the base), and `internal/memory` has one timing flake (`TestPersonalityManualProfileWinsAndDeletionRejectsOldAnalysis`,
  which passes alone).
- Batches D, E1, E2, F1 to F3 and the review repairs ran the targeted tests of each commit and the packages
  they touch (see the commit bodies).
- Waves 1 and 2 plus F1 to F3 (head `ea164abc9`): full suites of security, agent, server (`-skip Browser`),
  flows, tools, memory and 20 more packages, every failure attributed against a base export of `a2339dd31`.
  No new failure: `TestToolSearchTermsCoverEveryBuiltinTool` and two `ui` tests (chromecast help keys,
  dashboard card anatomy) fail identically on the base, three Docker volume-ownership tests in `internal/tools`
  need a Docker daemon, `TestMQTTMissionOwnershipFollowsEnableUpdateAndDelete` and the personality test flake on
  the base too, and the `go vet` lostcancel finding in `internal/llm/retry.go` is on the base.
- Review repairs (head `55dff579d`): full security, flows, server (`-skip Browser`) and agent packages pass
  apart from `TestToolSearchTermsCoverEveryBuiltinTool`.
- Documentation (this pass): names, thresholds (DNS rule, ThreatHigh for schema fences, 256 KiB clip,
  12000-byte summary part), test names, HiddenReason strings and the warning-line format were checked against
  the code by reading and grep.
