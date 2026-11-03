# Full-Codebase Deep Audit — AuraGo

**Date:** 2026-11-02
**Scope:** entire repository, split into 6 disjoint areas, audited in parallel by 6 independent sub-agents
**HEAD at audit time:** `97420240e`
**Total findings:** 74 — **0 CRITICAL · 6 HIGH · 39 MEDIUM · 28 LOW**

| # | Area | Report | Files / LOC audited | C | H | M | L |
|---|---|---|---|---:|---:|---:|---:|
| 1 | Server & HTTP-API | [`2026-11-02-deep-audit/1-server-api.md`](2026-11-02-deep-audit/1-server-api.md) | 592 files / ~156k LOC | 0 | 1 | 5 | 7 |
| 2 | Agent Core, LLM & Config | [`2026-11-02-deep-audit/2-agent-llm.md`](2026-11-02-deep-audit/2-agent-llm.md) | ~1 000 files / ~129k LOC | 0 | 2 | 10 | 4 |
| 3 | Tools, Sandbox & Security | [`2026-11-02-deep-audit/3-tools-security.md`](2026-11-02-deep-audit/3-tools-security.md) | 14 dirs, ~50 files read in full | 0 | 1 | 8 | 3 |
| 4 | Memory, Knowledge & Data | [`2026-11-02-deep-audit/4-memory-knowledge.md`](2026-11-02-deep-audit/4-memory-knowledge.md) | 632 files / ~152k LOC | 0 | 1 | 8 | 4 |
| 5 | Web UI, Desktop & TUI | [`2026-11-02-deep-audit/5-frontend-desktop.md`](2026-11-02-deep-audit/5-frontend-desktop.md) | ui/ + internal/desktop + Rust TUI | 0 | 2 | 6 | 5 |
| 6 | Platform & Domain Verticals | [`2026-11-02-deep-audit/6-platform-verticals.md`](2026-11-02-deep-audit/6-platform-verticals.md) | ~100 packages, tier-1 read in source | 0 | 0 | 2 | 5 |

**The 133 `internal/` packages are partitioned disjointly across the 6 areas — no package is audited twice, none is orphaned.** The full area→package assignment map is at the end of this document.

---

## Verdict

**No CRITICAL findings. The codebase is defensively built, and the six areas that matter most hold up under scrutiny:**

- **Authentication and authorisation on the HTTP boundary are sound.** The middleware chain is layered (`trustedProxy → previewHost → desktopTicket → panicRecovery → gzip → accessLog → securityHeaders → auth → mux`), the bearer-tier falls through to `admin` rather than open, the desktop scope gate has 146 call sites, and no handler was found reachable without auth.
- **The tool-execution path has 19 discrete control points** between the model's tool call and the host effect — authorisation intersection, action-policy risk gate, LLM guardian (fail-closed), re-authorisation after the async guardian wait, run-scoped permission narrowing, external-content classification, per-tool permission gate, shell policy, sandbox preparation, environment filtering, and output scrubbing.
- **No SQL injection, no zip-slip, no XXE, no SSRF, no command injection via shell interpolation, and no unbounded retrieval** were found in any area.
- **The flow engine is genuinely bounded** — cycle detection (`FLOW_CYCLE`), node/edge caps, clamped timeouts, run/output/pending-byte budgets, 30 s abandon grace.
- **FFmpeg is invoked argv-only**, the filter graph is passed as a *file* (`-filter_complex_script`), children are killed on cancel, and stderr is drained through a capped buffer — no pipe deadlock.
- **`cmd/memory-repair` refuses to run against a live database** (flock + backup + rehearsal).

The real damage is concentrated in **six HIGH findings** and, more importantly, in a repeated pattern of **green-but-blind CI**: several hundred tests that assert source text rather than behaviour, and entire subsystems that skip silently and still report `PASS`.

---

## The 6 HIGH findings

### H1 — Shell denylist is trivially bypassable (Area 3)
`internal/shellpolicy/policy.go:11`

The policy that decides what commands the agent may not run anchors its patterns to the **start of the command string**. `env rm -rf …`, `/usr/bin/rm …`, or any absolute-path invocation does not match any pattern. The agent picks the command string, so the control that decides "may the agent destroy things" is defeated by a path prefix.

*Proof:* the repository's own `BlockedReason()` was copied byte-for-byte outside the repo, wrapped in a throwaway module, and executed. The bypass is empirically confirmed, not inferred from reading the regexes.

### H2 — `toolArgInt` silently discards numeric arguments arriving as JSON strings (Area 2)
`internal/agent/tool_args_builtin.go:87`

Three of the four agent→tool transports deliver numeric parameters as strings. `toolArgInt` accepts only numeric Go types, so the value arrives, is validated, and is then thrown away — the tool runs with a zero value instead of the number the model asked for. The layer has **942 + 681 + 163** call sites depending on it.

### H3 — `ToolCall.UnmarshalJSON` contradicts its own raw-value contract (Area 2)
`internal/agent/agent.go:997`

The method documents that it preserves raw values, but it builds `Params` from **coerced** data. Since `Params` is the only channel the decoders read, the raw values are structurally unreachable — which is the mechanism behind H2. Two hand-rolled per-tool workarounds exist to patch the resulting gap.

### H4 — Heartbeat scheduler hangs the server on shutdown (Area 1)
`internal/heartbeat/scheduler.go:43-92`

`sync.WaitGroup` misuse plus an uninterruptible `Wait()`. `Stop()` can block forever, which turns a graceful shutdown into a hang — and the race sits exactly on the `CfgSaveMu`-held config-mutation path.

### H5 — SQLite corruption silently replaces the entire memory store with an empty database (Area 4)
`internal/dbutil/open.go:188`

Corruption is quarantined and replaced with a fresh empty store, and the app continues as if nothing happened. The user sees an empty memory rather than an error. The recovery story is also **inverted** against the sibling stores: the corpus and knowledge-graph stores fail loudly.

### H6 — Rust TUI: the agent SSE stream inherits a 30-second *total* request timeout (Area 5)
`tools/aurago-tui/src/api/mod.rs:22`

The streaming path shares one HTTP client with the control plane, so the agent stream is severed every 30 seconds and never resumes.

---

## Cross-cutting themes

These patterns recur in independent areas — they are structural, not local defects.

### 1. Green-but-blind CI is the single largest systemic risk
- **~490 browser smoke tests never run in CI** and silently SKIP when Chrome/Edge is absent — while the package still reports `ok aurago/ui` (Area 5, H2).
- **66 `requireFFmpeg(t)` call sites skip**, and `ffmpeg` is absent from the official `ubuntu-latest` runner image (manifest fetched and checked). The highest-risk subsystem — 130 files / ~11k LOC of FFmpeg rendering — is never executed (Area 6).
- **6 asset validators** (`check_assets.py` ×3, `verify.py` ×2, `package_evaluation.py`) exist on disk but are invoked by no workflow (Area 6).
- **The three top coupling hubs assert source text, not behaviour** — `readDesktopAssetText` (~693 callers), `mustReadUIFile` (~288), `normalizeAssetText` (~220). A behaviourally broken function that keeps the marker still passes (Area 5).

Together these produce **~1 200 green assertions that can all pass while the UI is broken.**

### 2. Denylist-by-regex is the wrong shape for a shell control
The shell policy, the injection guard's 29-name tool allowlist, and the egress allowlist all work by *matching strings that should not appear*. Each is defeated by changing the spelling: a path prefix (H1), a tool that is not on the list, or a differently-formatted IP.

### 3. Two sources of truth for permissions, and the privileged path loses the stricter one
The permission gates read the **process-wide server snapshot**; the agent dispatch layer applies **run-scoped narrowing** on the `context`. `ExecuteShellBackground` and `ExecuteSudo` take no `ctx` and therefore lose the narrowing — and they are exactly the two most privileged entry points (Area 3).

### 4. The audit trail is a log, not evidence
No hash chain exists anywhere in the repository (verified repo-wide, zero matches). `internal/audit` is a static manifest plus contract tests with **no runtime writer at all**. The real trail lives in `internal/memory/audit.go` and `internal/agent/action_ledger.go`, and failure paths are not consistently recorded (Area 3).

### 5. Schema governance is split three ways in one package
`internal/memory` runs a versioned `user_version` chain, an unversioned table-existence gate, and a ~320-line non-transactional legacy path — all writing to the same database. Three sibling databases, three different recovery contracts (Area 4).

### 6. Error scrubbing happens at exactly one layer, and most callers bypass it
- **437 `jsonError(w, err.Error())` call sites** bypass the repository's own secret scrubber (only 123 are scrubbed) (Area 1).
- **Tool-layer error envelopes carry raw error text**, and server HTTP handlers write them straight to clients (Area 3).

### 7. Shallow copies of live, shared state
**41 shallow `*s.Cfg` copies** bypass the existing `config.Config.Clone()` deep-copy helper (Area 1) — a latent data race on the live config.

### 8. Copy-paste drift between siblings
- A byte-identical `cappedBuffer` duplicated under a comment saying the policy is shared (Area 6).
- Four independent Markdown renderers with three different escaping strategies, and dozens of near-duplicate `escapeHtml`/`escapeAttr` copies with differing contracts (Area 5).
- Type coercion split across three unsynchronised helpers plus two per-tool workarounds (Area 2).

### 9. CSP and trust boundaries are weaker than they look
- The primary web UI ships **`script-src 'unsafe-inline'`** with 100+ inline `onclick=` handlers — there is no second layer behind any HTML injection (Area 5).
- `internal/desktop` (148k LOC) performs **no authorisation of its own**; `AllowAgentControl` is display-only state. Everything rests on `internal/server/desktop_auth.go` (Area 5).
- A dedicated Chromecast/TTS server binds **`0.0.0.0`** even when the main server is on `127.0.0.1` — and it bypasses the entire middleware chain (Area 1).

### 10. The same key questions asked of every area produce the same answer
"Does X have a bound / a timeout / a closed handle?" — the answer is yes in the flow engine, FFmpeg, the CYD job model, the memory migrations, the SSE broadcaster and the Go filesystem jail; and no in the heartbeat scheduler, the model-limit cache, `SQLiteMemory.Close()`, the Rust TUI's SSE handle, and the Docker daemon dialer.

---

## Test-coverage gaps, consolidated

| Gap | Area | Why it matters |
|---|---|---|
| ~490 browser smoke tests never run in CI | 5 | The browser half of the UI suite never executes; CI reports `ok` |
| 66 `requireFFmpeg` sites skip; no ffmpeg in the runner image | 6 | The highest-risk subsystem is untested in CI |
| 6 asset validators never invoked by any workflow | 6 | 3D-asset contracts are checked by hand only |
| Top 3 UI hubs assert source text, not behaviour (~1 200 assertions) | 5 | False confidence; behaviour breaks pass |
| Chat's hand-written HTML sanitizer has **no** behavioural XSS-corpus test | 5 | It is the sole barrier for model output in the UI |
| 136 raw-variable `innerHTML` sinks policed only by a fixed file list | 5 | Not a repo-wide rule; new files escape it |
| No direct unit tests for `toolArgInt`/`toolArgString`/`firstNonEmptyToolString`/`firstNonEmptyInt` | 2 | The layer 942+681+163 call sites depend on |
| `tool_args_*_test.go` hand-builds `Params` with correct Go types | 2 | The JSON→map coercion layer is invisible to CI |
| Only 1 of 7 `numericStringToolFields` is exercised | 2 | The other 6 are untested |
| No concurrent-migration test (two stores, one file, each step once) | 4 | The HIGH corruption/migration findings are unproven but unexcluded |
| No decompression-cap test on `readLegacyGob` | 4 | A bomb-sized `.gob.gz` is never rejected |
| Permission gates tested almost exclusively **in the open** | 3 | `TestMain` opens every gate for the whole package |
| Shell-policy deny table has no absolute-path case | 3 | The H1 bypass is untested |
| Sidecar: 9 tests for 1 055 source lines; nothing covers `proxyClientAllowed` or `cleanHeaders` | 3 | The egress boundary is untested |
| `internal/audit/tool_error_json_contract_test.go` ratchets JSON validity only | 3 | Nothing ratchets secret scrubbing |
| No test asserts run-scoped narrowing reaches the shell gate | 3 | The `context.Background()` drop is untested |
| No route-manifest test | 1 | "Which routes are unauthenticated" cannot be asserted automatically |
| No test asserts the Chromecast/TTS bind address matches `server.host` | 1 | The `0.0.0.0` widening is invisible to CI |
| No test asserts `handleUpdateConfig` error responses are redacted | 1 | The real credential-exposure rate is unmeasured |
| No concurrency test for `heartbeat.Scheduler` `Restart` vs `Stop` | 1 | The HIGH finding is unproven but unexcluded |
| Lowest test-to-code ratios in scope: `qrcode` 0.21, `smbclient` 0.26, `kaggle` 0.26, `infra` 0.32, `onvif` 0.34 | 6 | Measured risk concentrations |

---

## Recommended order of work

1. **H1 — shell-policy bypass.** Replace the denylist with an argv-aware check (or an allowlist of binaries resolved to absolute paths) and add the absolute-path case to the deny table. This is the one finding with an agent-controlled exploit path.
2. **H2/H3 — the tool-argument coercion contract.** Fix `ToolCall.UnmarshalJSON` to preserve raw values as documented, then make `toolArgInt` accept numeric strings. One change, two findings, ~1 800 call sites unblocked. Add the missing unit tests in the same pass.
3. **H4 — heartbeat shutdown hang.** A `-race` test for `Restart` vs `Stop` settles it.
4. **H5 — memory corruption recovery.** Decide the contract (quarantine-and-refuse vs. quarantine-and-replace), then make the three sibling stores agree.
5. **H6 — Rust TUI SSE timeout.** Split the streaming timeout from the control-plane timeout.
6. **The CI blind spots.** Install `ffmpeg` in the runner, set the browser-smoke env var, and wire the 6 asset validators. This is configuration work with outsized effect.
7. **Cross-cutting:** route the HTTP and tool error boundaries through the existing scrubber (`jsonError` → scrubbed variant); make the permission gates read the run-scoped context; add a hash chain to the audit trail; replace the `*s.Cfg` shallow copies with `Clone()`.

---

## Method, and what it does *not* cover

**How it ran.** The repository was partitioned into 6 disjoint areas and 6 independent sub-agents audited them in parallel, each with a self-contained brief: scope map, seven audit dimensions, a severity rubric, an evidence rule (no finding without a verbatim source read), and a required report structure. Every report carries per-finding **Location / Evidence / Why it matters / Fix / Confidence**, plus an architecture map, metrics and an explicit "not covered" section.

**Tooling.** The `graft` **MCP server timed out on essentially every call across all six areas** (120 s timeout). All graph work fell back to the `graft` CLI (`graft ask|grep|callers|skeleton`, with `--no-refresh` where the refresh was the slow part). The graph was mid-rebuild for much of the run, so every graph-derived claim was re-verified against raw source. GitNexus `impact` was executed in Area 1 only; Areas 3, 4 and 5 explicitly make **no graph-backed blast-radius claims** as a result.

**What this audit deliberately did not do:** no builds, no test execution, no formatters, no codegen, no `git` mutations. All findings are static source reads, except two behavioural experiments (Area 4's SQLite URI check, Area 3's `BlockedReason()` execution).

**Honest gaps, stated by the auditors themselves:**
- Area 1: ~13 large handler files (~20k LOC) were never opened — `agodesk_handlers.go` alone is 3 497 LOC. **This is the highest-yield next pass.**
- Area 2: `long_term.go` (~1 900 LOC), the privacy-deletion engine, and `internal/decisions` were not audited.
- Area 3: `internal/tools` (804 files / 211k LOC) was audited depth-first along the high-risk execution paths, **not exhaustively**; ~30 provider integrations using raw `&http.Client{}` were inventoried only.
- Area 5: `openscad_container.go` (51 KB), `looper.go` (44 KB), `code_container.go`, `sftp_handlers.go`, `vnc_proxy.go`, `ssh_proxy.go`, `serial_proxy.go` were not read; `internal/desktopstore/service.go` (3 085 LOC) was reviewed via skeleton only.
- Area 6: messaging and device clusters were swept, not read; the "shared-helper" assumption for their HTTP/TLS/credential handling is unproven.

**Two open items to verify before acting on them** (both flagged by their auditors):
- Area 2, F-1: the downstream consumer of `webScraperArgs.Limit` in `internal/tools` — determines H2's blast radius.
- Area 2, F-16: whether `ApplyVaultSecrets` writes resolved slots that `applyProviderCredentialBindings` does not cover.

**One methodological correction worth recording.** Area 3's first attempt to reproduce the shell-policy bypass re-implemented the regexes in JavaScript and produced empty capture groups — an extraction bug. That attempt was **discarded rather than reported**, and the finding was re-proven by executing the repository's own Go implementation. Two other auditors independently tested and **ruled out** suspected bugs (Area 4's SQLite URI percent-decoding; Area 3's IPv4-mapped-IPv6 handling).

---

## Area → package assignment map

Complete and disjoint. Every one of the 133 `internal/` packages plus the non-`internal/` code roots appears exactly once.

**Area 1 — Server & HTTP-API:** `internal/server`, `cmd/aurago`, `internal/commands`, `internal/webassets`, `internal/httporigin`, `internal/httpstream`, `internal/media`, `internal/setup`, `internal/heartbeat`, `internal/updater`, `internal/integrationstatus`, `internal/logger`, `internal/i18n`, `internal/uid`

**Area 2 — Agent Core, LLM & Config:** `internal/agent`, `internal/llm`, `internal/localllm`, `internal/config`, `internal/prompts` (+ `prompts/`), `internal/budget`, `internal/providerutil`, `internal/a2a`, `internal/vaultprompt`, `internal/promptsource`

**Area 3 — Tools, Sandbox & Security:** `internal/tools`, `internal/security`, `internal/audit`, `internal/sandbox`, `internal/injectionguard`, `internal/shellpolicy`, `internal/tresor`, `internal/credentials`, `internal/rules`, `internal/toolmeta`, `internal/virtualcomputers`, `internal/proxy`, `browser_automation_sidecar/`, `mcps/`

**Area 4 — Memory, Knowledge & Data:** `internal/memory`, `internal/embeddings`, `internal/chunking`, `internal/kgextraction`, `internal/kgquality`, `internal/codegraph`, `internal/localwiki`, `internal/zim`, `internal/office`, `internal/decisions` (+ `decisions_runtime/`), `internal/bookshelf`, `internal/bookshelfconvert`, `internal/obsidian`, `internal/notebooks`, `internal/scraper`, `internal/newspaper`, `internal/mediabins`, `internal/contacts`, `internal/inventory`

**Area 5 — Web UI, Desktop & TUI:** `ui/`, `scripts/`, `internal/desktop`, `internal/desktopstore`, `tools/aurago-tui/`, `internal/ruview`

**Area 6 — Platform & Domain Verticals:** `internal/flows`, `internal/gamemaker` (+ `assets/`), `internal/cydcreator`, `internal/cyd`, `internal/videostudio`, `internal/planner`, `internal/services`, `internal/infra`, `internal/persondetect`, `internal/personabooster`, `internal/pixelmatte`, `internal/onnxrt`, `internal/mailer`, `internal/whatsapp`, `internal/telegram`, `internal/discord`, `internal/rocketchat`, `internal/agentmail`, `internal/telnyx`, `internal/sipphone`, `internal/voice`, `internal/speechlab`, `internal/realtimespeech`, `internal/sanotts`, `internal/remote`, `internal/fritzbox`, `internal/rtlsdr`, `internal/onvif`, `internal/networkshares`, `internal/smbclient`, `internal/dockerstacks`, `internal/dockerhost`, `internal/dockerwatch`, `internal/dockerutil`, `internal/bluetooth`, `internal/serialutil`, `internal/mqtt`, `internal/tsnetnode`, `internal/meshcore`, `internal/meshcentral`, `internal/retronet`, `internal/ecalc`, `internal/findit`, `internal/qrcode`, `internal/manus`, `internal/kaggle`, `internal/huggingface`, `internal/acestep`, `internal/gog`, `internal/personalradio`, `internal/upkeep`, `internal/detective`, `internal/agodesk`, `internal/sqlconnections`, `internal/dbutil`, `internal/ldap`, `internal/rustfs`, `internal/layerling`, `internal/evomap`, `internal/launchpad`, `internal/warnings`, `internal/sudoticket`, `internal/jellyfin`, `internal/invasion`, `internal/retrogames`, `internal/restorepending`, `internal/systemworld`, `internal/testutil`, `internal/fileutil`, `internal/push`, `internal/buildinfo`, `internal/releaseverify`, `cmd/assetpack`, `cmd/config-merger`, `cmd/memory-repair`, `cmd/release-verifier`, `cmd/remote`, `agent_workspace/`, `training/`, `ansible_api/`, `disposable/`, `deploy/`

---

## Provenance

Six independent sub-agents, one per area, run in parallel under a uniform brief. Read-only on the repository: the only files written are this document and the six area reports under `2026-11-02-deep-audit/`. The working tree carried unrelated modifications throughout the run (a concurrent writer in `ui/img`, `ui/js`, `ui/css` plus seven commits landing mid-audit); every area auditor re-verified their cited line references against the post-commit tree before finishing.
