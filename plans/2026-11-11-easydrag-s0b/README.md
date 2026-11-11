# EasyDrag S0b "Egress and credential hygiene" Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement S0b, the server-side half of Wave 0. Every headless browser that opens an untrusted URL goes through its own egress proxy. A credential typed into a flow can be moved into the vault, and a guided purge removes every plaintext copy.

**Architecture:**
- **Part A.** A new helper, `security.LaunchEgressBrowser`, starts a fresh Chrome behind a per-call loopback `CaptureProxy`. The proxy has per-call credentials, a path check and fail-closed behaviour. The five browser sites move to the helper, and the shared browser and the page hook are deleted.
- **Part B.** One detection rule in `internal/flows/credentials.go` drives validation, redaction and client masking. `http.request` gains query auth, and an admin endpoint moves a credential into the vault. The guided purge does six things:
  - takes a master-key encrypted 7-day backup;
  - marks old versions for purge in schema 9 and deletes them;
  - redacts runs in place;
  - compacts the database;
  - deletes the chosen snapshots;
  - verifies byte for byte.

**Tech Stack:** Go 1.27 (`internal/security`, `internal/tools`, `internal/scraper`, `internal/flows`, `internal/server`, `cmd/aurago`); go-rod v0.116.2; SQLite (WAL); vanilla JS classic scripts with `window.EasyDrag`; Node test harnesses; go-rod browser tests.

**Spec:** [`plans/2026-11-11-easydrag-s0b-spec.md`](../2026-11-11-easydrag-s0b-spec.md). **Roadmap:** [`plans/2026-11-09-easydrag-next-roadmap.md`](../2026-11-09-easydrag-next-roadmap.md) E1 (FD-13, FD-18) and §6 Wave 0.

---

## Files of this plan

| File | Spec | Tasks |
|---|---|---|
| [part-a-egress.md](part-a-egress.md) | §3, §7 Part A | A.1 spike on proxy auth · A.2 proxy credentials, `Done()`, check host · A.3 `LaunchEgressBrowser` · A.4 five call sites, deletions · A.5 S8 fixture and browser tests · A.6 live run on aurago-test |
| [part-b1-move.md](part-b1-move.md) | §4, §5 | B.1 the rule, vectors, validation, redaction · B.2 `http.request` query auth · B.3 move endpoint · B.5 client masking · B.4 UI action and `scrubHistory` |
| [part-b2-purge.md](part-b2-purge.md) | §6, §7 Part B | B.6 schema 9, marks, rollback guard · B.7 master-key file encryption, sweeper, restore flag · B.8 scan and hint card · B.9 run hold, purge job, dialog · B.10 acceptance fixture |

There are 16 tasks. Each commits once, except A.1, which commits only the spec amendment.

**Two worktrees, run in parallel.** Part A and Part B touch separate files.
- **Part A:** `.worktrees/easydrag-s0b-egress`, branch `feat/easydrag-s0b-egress`. Order A.1 → A.6.
- **Part B:** `.worktrees/easydrag-s0b-credentials`, branch `feat/easydrag-s0b-credentials`. Order B.1, B.2, B.3, **B.5, B.4** (amendment 7), then B.6 → B.10.

Each part's preamble has its own worktree setup and baseline check.

**Review batches.** One combined review per batch, reporting only Critical and Important findings, with no re-review rounds.

| Batch | Covers | Review point | Reviewer |
|---|---|---|---|
| A | Part A | after A.5, before the live run A.6 | security-agent |
| B1 | Part B1 | after B.4 | security-agent |
| B2 | Part B2 | after B.10 | security-agent, plus a code reviewer for the UI |

---

## Conventions that override the parts

The three parts were planned in parallel. Where they differ, this section decides.

- **Shell.** Git Bash. Every command starts by changing to the part's worktree, because the working directory resets between calls. `MSYS_NO_PATHCONV=1` for arguments that start with `//`.
- **IMPACT.** Before changing or moving a function, run `node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<name>" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo`.
  - Warn on HIGH or CRITICAL.
  - UNKNOWN is unresolved: confirm the callers with grep.
- **DETECT.** GitNexus cannot index a worktree on this machine. Every commit body therefore ends with the line `GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.`, placed before the Co-Authored-By line.
- **Commits.**
  - Stage exact paths, then `git commit -F <message file>`. Write message files into `C:/disposable/easydrag-s0b/`.
  - The last line is the Co-Authored-By line for the model the implementer actually runs as.
  - Never amend, rebase or push, except in "Finishing S0b".
- **Go temporary directory.**
  - `GOTMPDIR='D:\tmp\go-build'` for `internal/security`, `internal/tools`, `internal/scraper` and `internal/flows`.
  - No `GOTMPDIR` for `internal/server`, because a temp directory on D: breaks the cross-drive `filepath.Rel`.
- **Browser tests.**
  - Gated by `AURAGO_RUN_BROWSER_SMOKE=1`, with `AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0b-shots`. Every one sets reduced motion explicitly.
  - Re-run a single failing test alone before treating it as a regression.
  - Chrome launch problems: see the memory note `reference_chrome_launch_from_agent`.
- **Escapes.** The Write and Edit tools turn backslash-u sequences into characters.
  - Go: `"\x5c" + "u003c"` or numeric runes.
  - JS: `String.fromCodePoint(...)`.
- **Locales and codes.**
  - Every new key goes into all 16 files of its `ui/lang/<area>/` folder, with a real translation. German uses lowercase "du".
  - A new `FLOW_*` code is registered by its locale key `easydrag.ui.error_<code>` and by the member map in `ui/desktop_easydrag_test.go`. There is no Go registry.
- **New EasyDrag JS modules.**
  - Register each one in `module-loader.js`, in the Go list `easyDragModules`, and in the Node harness lists (grep `easydrag-run-stream.js` in `scripts/`).
  - The line ratchet warns above 900 and fails above 1,100.
- **Gates before each commit.**
  - Go changes: `go vet` and `gofmt -l` on the touched packages and files.
  - JS changes: `node scripts/test-easydrag.mjs` and `node scripts/test-easydrag-render-baseline.mjs` (`PASS render baseline (unchanged)`).
- **Tests.** Essential tests only (the owner's standing preference). No external assets.
- **aurago-test.** Any test is allowed, but never create, delete, recreate or update containers, apps or data there.

---

## Amendments found while planning

Each amendment is applied in the named task. The spec has been updated where it was wrong.

**Part A**
1. **Options (A.3).** `EgressBrowserOptions` gets `Bin` and `Sandbox` beside `AllowedOrigin` and `Lifetime`. The sites differ on sandbox: web capture, web performance and form automation use `NoSandbox(false)`, while the scraper and the homepage capture use rod's default. The homepage capture already has a test-only `BrowserBin`.
2. **Proxy log (A.2, A.3).** Main's proxy keeps no log. A.2 adds `Dialed()` and `Refused()`, capped at 256 entries. A.3 adds `ProxyDialed()` / `ProxyRefused()` on the browser and a tests-only `SetEgressLaunchObserver`. The two site tests need the observer, because the tool creates the proxy itself.
3. **Path check (A.2, A.3).** A 204 navigation does not commit, so the helper reads the nonce with `fetch` from a blank page, and the proxy adds the CORS headers for it. The proxy holds the nonce itself instead of echoing the path, so an intercepting proxy cannot fake it.
4. **Slot wait (A.3).** A caller without a deadline waits at most 2 minutes, then gets `ErrEgressBusy`.
5. **Spike rule (A.1).** A speculation-rules prefetch that headless Chrome never issues counts as a pass, because a request that is never sent cannot leak; A.5 exercises it again. Any other kind that does not show up after one rerun counts as a failure.
6. **Auth handling (A.1, A.3).** rod's `HandleAuth` answers one challenge per call. The helper uses its own `EachEvent` loop on `Fetch.authRequired` and answers only challenges whose source is the proxy and whose origin is the proxy address.

**Part B1**

7. **Order.** B.5 (client masking, `ED.credentials`) runs before B.4, because the UI action reads `ED.credentials.findings`. The headings keep the spec's numbers.
8. **Remembered test trigger.** `aurago.easydrag.test-trigger.<id>` holds only a node id, so it needs no masking. Test data lives on the server and is redacted by the purge. Spec §5.4 is amended.
9. **Rule limits (B.1, B.3).**
   - Top-level params of non-tool nodes are not scanned by name; `auth_secret` holds a vault name.
   - A header value that mixes a template with a literal is a finding, but it is not movable.
   - One `auth_secret` slot per node means one movable credential per node; further ones are not movable.
   - "Identical value elsewhere" also requires the same location.
   - Nested paths use `<pointer>?<key>`.
10. **Where the action sits (B.4).** It lives in the detail view's issue box, not on the card; a click on any card issue opens that view. Only the first credential issue per param is shown, and the next one appears after a move. Spec §5.3 is amended.
11. **Short values in `scrubHistory` (B.4).** A value under 6 characters is replaced only where it is a whole text.
12. **Vault write before the draft save (B.3).** If another save slips in between, the vault entry stays unused and the client reloads on 409. That is accepted, because the opposite failure, a draft pointing at a secret that was never written, is worse.
13. **Routes.** EasyDrag routes live under `/api/desktop/flows/...`. `FLOW_REVISION_CONFLICT` already exists and is reused. Spec routes are amended.

**Part B2**

14. **Pinned versions (B.6, B.10).** A pinned marked version keeps its plaintext until the prune deletes it. Redacting it in place could break a durable run that resumes from it. The acceptance byte search therefore runs after the pinned run has ended. Spec §7 is amended.
15. **Prune trigger (B.6).** Today's prune runs only inside `Publish`. B.6 also prunes marked versions in `FinishRun` and in the hourly `PruneNow`.
16. **The body token in the acceptance fixture (B.10).** It is not movable, so B.10 removes it from the draft before the republish, as "Wert entfernen" does. The byte search then expects **none** of the three tokens anywhere. This overrides the part text, which tolerated the body token in flows.db. Spec §7 is amended.
17. **The marker is never a finding.** B1's `literalCredentialText` excludes `CredentialRedactionMarker`, and B.8 pins this with a test, so the purge converges.
18. **`mountHint(container, app)` (B.8).** It takes the app as a second argument. `offer(flowId)` uses the app from the last `mountHint`.
19. **Admin on the start page (B.8).** There is no admin flag there. The hint calls the scan endpoint and stays hidden on 401 or 403. `easydrag-home.js` grows by 3 lines, to 889.
20. **Run hold (B.9).** `Runner.HoldDispatch()` refuses with `ErrRunsActive` while `slots > 0`. While held, new runs are recorded as queued and start on release.
21. **`doc_hash` (B.9).** Nothing recomputes it against `doc_json`; it is only a join key into `flow_test_docs`. Redacting in place is safe.
22. **Restore lock (B.7).** The flag takes the same `flock` on `<installDir>/aurago.lock` that `main.go` takes. While AuraGo runs, it exits 1 with "AuraGo is running".
23. **Clone.** Main has no "clone from a version" path. Only `RestoreVersion` and `RestoreBlockVersion` read old versions, and both go through the new `Store.GetVersionForCopy` guard.

## Not covered by S0b (follow-ups)
- **Secrets in other nodes.** A general vault reference for body fields and for nodes other than `http.request` (owner decision 4).
- **Out-of-scope browsers.** Storymaker rendering, the browser_automation sidecar, Gotenberg and guest-VM browsers keep their own egress (spec §2).
- **The CLAUDE.md rule on detect-changes.** GitNexus cannot index a worktree on this machine, so worktree commits are judged by symbols and tests. Fixing that is a GitNexus memory issue.

---

## Finishing S0b

Part A usually finishes first and lands first.

1. **Rebase.** Rebase the part's branch onto the current main inside its worktree, then run the part's full checks again:
   - Part A: `go test ./internal/security/ ./internal/scraper/ ./internal/tools/ -count=1`, plus the gated browser tests;
   - Part B: `go test ./internal/flows/ ./internal/server/ ./cmd/aurago/ -count=1`, `node scripts/test-easydrag.mjs`, the render baseline and one full EasyDrag browser run.
2. **Check the main checkout.** Before landing, check `git -C C:/Users/Andi/Documents/repo/AuraGo branch --show-current` and compare the uncommitted files with the branch's files.
3. **Land.** `git -C C:/Users/Andi/Documents/repo/AuraGo merge --ff-only <branch>`. Do not push; the owner decides when to push and deploy.
4. **Record.** Update the roadmap §6 Wave 0 status line and the EasyDrag project memory with the landed SHA. Remove the worktree and delete the merged branch.
