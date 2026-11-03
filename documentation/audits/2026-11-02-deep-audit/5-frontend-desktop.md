# Audit: Web UI, Desktop App & TUI

## Scope & method

**Area 5** of the AuraGo deep-audit plan: `ui/**` (web UI JS/CSS/HTML/partials + the ~684 Go browser/regression drivers), `scripts/**` (UI regression tooling), `internal/desktop/**` (native desktop service), `internal/desktopstore/**` (desktop app store), `tools/aurago-tui/**` (Rust terminal UI), `internal/ruview/**` (3D/precision viewer).

**Read-only.** No repository file was created, modified or deleted by this audit. The only artefact written is this report at `.tmp_aurago/audit/5-frontend-desktop.md`; the directory `/.tmp_aurago/` is already in `.gitignore`, and `git status --porcelain -- .tmp_aurago` returns empty.

*Integrity note:* `git status --porcelain` measured 670 changed lines (SHA256 `4a2ffbbf...`) at the start of the audit and 664 (SHA256 `8d56598f...`) at the end. The working tree was modified **by another writer** during the pass — the differing entries are files such as `ui/lang/desktop/sv.json` and `ui/lang/desktop/zh.json`, which this audit never wrote to (every command run here was `read`/`grep`/`glob` or a read-only shell command; the only write was this file). The report directory is excluded by `.gitignore`, so it does not appear in the status.

**Tooling.** The `graft` MCP server timed out on the very first call (`graft_check_freshness` → 120 s timeout) and `graft ask` later failed with a "Parent-death guardian target registration timed out". Per `AGENTS.md` I fell back to the **graft CLI** (`graft grep|skeleton|callers|map`, v0.28.0), which worked reliably. `graft` does **not** index Rust, so `tools/aurago-tui/**` and `scripts/**` were read with the raw `grep`/`read` tools.

**Graph calls made (≈50 graft/CLI + 3 GitNexus-adjacent greps, 3 failed/timed out):**
`graft map` · `graft skeleton` on `internal/desktop/service.go`, `internal/desktop/types.go`, `internal/desktopstore/service.go` · `graft grep` for `innerHTML|outerHTML|insertAdjacentHTML|document.write` (ui/js), `\beval\(|new Function\(|setTimeout\(\s*['"]` (ui/js), `exec\.Command|exec\.CommandContext` (internal/desktopstore, internal/ruview), `DOMPurify` (ui/js), `new EventSource|new WebSocket` (ui/js), `removeEventListener` (ui/js/desktop), `setInterval\(` (ui/js), `localStorage\.setItem|sessionStorage\.setItem` (ui/js), `allow_agent_control` (ui/js), `requireDesktopPermission` (internal/server), `isDesktopScopedAPIPath` (repo-wide), `AllowAgentControl` (internal/desktop), `sanitizeRenderedHTML|CHAT_SANITIZER_ALLOWED_TAGS`, `renderMarkdown|markdownToHtml|sanitizeHtml|DOMPurify`, `nasLog`, `desktop\.Config\{`, `type Config struct`, `X-Content-Type-Options` (internal/server), `isDesktopScopedAPIPath`, `desktopScope*` · `graft callers` on `Config` · `graft ask` (failed).

**What I could NOT cover (honest gaps):**
- **136 raw-variable `innerHTML` assignments across 74 `ui/js` files** were enumerated but only the chat/markdown path and two drawer renderers were read line-by-line. The remaining sites are *Needs-verification* (F-10).
- `internal/desktop/**` is ~127 k LOC across ~128 files. I read `service.go` (path resolution, config, bootstrap), `types.go`, `config.go` (via graft) and `nasscad_logs.js` in full, plus skeletons of the rest. `openscad_container.go` (51 KB), `looper.go` (44 KB), `code_container.go`, `sftp_handlers.go`, `vnc_proxy.go`, `ssh_proxy.go`, `serial_proxy.go` were **not** read.
- `internal/desktopstore/service.go` (3 085 LOC) was reviewed via `graft skeleton` only; the install/update/uninstall paths were not traced line-by-line.
- `internal/ruview/**` was sampled (`runtime.go`, `proxy.go`); `manager.go` (629 LOC) and `lifecycle.go` were not read.
- `ui/partials/**`, `ui/css/**` and the 684 Go driver files under `ui/` were not read individually — only via greps and skeletons.
- `scripts/*.mjs` other than `test-ui-regressions.mjs` were listed but not audited.
- No build, no test run, no `go vet` was executed (read-only constraint), so all findings are source-derived, not runtime-derived.

**Honest headline:** this area is *not* in bad shape. The chat Markdown sanitizer, the desktop path-traversal defence, the server-side desktop authorisation, the DOMPurify/i18n lint suites and the event-listener hygiene are all genuinely good. The defects I could confirm cluster in three places: (a) the Rust TUI's SSE transport, (b) browser smoke tests that never run in CI, and (c) source-text-based regression tests that give false confidence.

---

## Architecture map

**Web UI (server-rendered shell + vanilla JS)**
- Shared page runtime + i18n + modal stack: `ui/js/shared/shared-core.js:18` (`t()`), `:37` (`tOr()`), `:54` (modal queue), `:1884` (`_connect` EventSource).
- Locale dictionaries **are** shipped as files: `ui/lang/<section>/<lang>.json` — 2 049 JSON files, 30 sections x 17 languages (`cs da de el en es fr hi it ja nl no pl pt sv zh`). They are loaded and merged by `internal/i18n/i18n.go` (English first, then the selected locale, so a key missing from a locale still resolves to the English string) and injected into the page at `ui/js/shared/template-data.js:24` (`window.I18N = data.i18n || {}`) and `ui/js/shared/shared-core.js:1553`.
- Chat page: `ui/js/chat/chat-messages.js:150` (`appendMessage`) → `ui/js/chat/bundles/chat-runtime.bundle.js:7170`; sanitizer at `ui/js/chat/bundles/chat-runtime.bundle.js:433`, allowlists at `:357`/`:368`, URL guards `isSafeHref` `:417`, `isSafeMediaSource` `:484`.
- Desktop SPA (in-browser desktop): loader `ui/js/desktop/core/module-loader.js:685` (`loadAppI18nSections`), per-app modules under `ui/js/desktop/apps/**`, icons `ui/js/chat/ui-icons.js:182`.
- Bundled app JS is generated and drift-checked in CI: `scripts/build-ui-bundles.js --check` (`.github/workflows/security-gates.yml:108`).
- Standalone pages ship inline `onclick=` handlers (`ui/cheatsheets.html:108`, `ui/containers.html:115`, `ui/knowledge.html:310`, `ui/login.html:96`).

**Server ↔ UI boundary**
- Security headers + CSP: `internal/server/server.go:2248` (`securityHeadersMiddleware`).
- Desktop API authorisation: `internal/server/desktop_auth.go:23` (`requireDesktopPermission`, 146 call sites repo-wide), scopes at `:9-11`.
- Bearer-scope routing with a secure default: `internal/server/auth.go:1164` (`validRouteBearer`, falls through to `admin`).

**Desktop app (Go, in-process)**
- Config: `internal/desktop/types.go:25` (`Config`, **no credentials**), wiring `internal/desktop/config.go:11` (`ConfigFromAuraConfig`).
- Service: `internal/desktop/service.go:110` (`NewService`), `:123` (`normalizeConfig`), `:675` (`Bootstrap`), `:809` (`ResolvePath`), `:906` (`validatePathComponentsWithinRoot`).
- Bundled mini-apps: `internal/desktop/bundled_apps/nasscad/nasscad_logs.js:43` (`nasLog`).
- Store: `internal/desktopstore/service.go:93` (`NewService`), 3 085 LOC in one file.

**RuView (3D/precision viewer)**
- Native launch: `internal/ruview/runtime.go:313` (`launchNative`), hash verification `:295` (`verifyNative`).
- Upstream proxy hardening: `internal/ruview/proxy.go:133` (redirects disabled, `Set-Cookie`/`Location`/CSP stripped).

**Rust TUI**
- HTTP client: `tools/aurago-tui/src/api/mod.rs:21` (`ApiClient::new`), `tools/aurago-tui/src/api/auth.rs:278` (`save_session_cookie`).
- SSE: `tools/aurago-tui/src/api/sse.rs:68` (`connect_sse_once`), wired at `tools/aurago-tui/src/main.rs:877` (`start_chat_session`).
- Task supervision: `tools/aurago-tui/src/app.rs:667` (`spawn_tracked`), `:673` (`abort_all_background`), `:680` (`prune_finished_tasks`).

**Test/regression tooling**
- Go UI drivers: `ui/desktop_window_menu_test.go:404` (`readDesktopAssetText`), `ui/modal_regression_test.go:61` (`mustReadUIFile`), `ui/chat_regression_test.go:27` (`normalizeAssetText`), `ui/precision_workspace_browser_test.go:13` (`requirePrecisionBrowserSmoke`), `ui/destructive_flow_browser_test.go:47` (`newSmokeBrowser`).
- Node regression suite: `scripts/test-ui-regressions.mjs:11` (`read`), `:15` (`sourceBetween`).
- Lint suites: `ui/security_lint_test.go:266` (`TestMarkdownSinksUseDOMPurify`), `ui/i18n_lint_test.go:60` (`TestFrontend_JS_HardcodedUIToasts`), `:669` (`TestTranslations_AllKeysPresentInAllLanguages`).

---
## Findings

### F-1 [HIGH] Rust TUI: the agent SSE stream inherits a 30-second *total* request timeout and is severed every 30 s

**Location**
- `tools/aurago-tui/src/api/mod.rs:22-24`
- `tools/aurago-tui/src/main.rs:892`, `tools/aurago-tui/src/main.rs:906`
- `tools/aurago-tui/src/api/sse.rs:75`, `tools/aurago-tui/src/api/sse.rs:91`

**Evidence** — `tools/aurago-tui/src/api/mod.rs:21-28`:
```rust
pub fn new(base_url: &str, insecure: bool) -> Result<Self> {
    let mut builder = ClientBuilder::new()
        .cookie_store(false)
        .timeout(Duration::from_secs(30));
```
`tools/aurago-tui/src/main.rs:891-906`:
```rust
let url = client.sse_url("/events");
let c = client.client.clone();          // <- the same 30 s-total client
...
sse::connect_sse(c, url, origin, cookie, sse_tx).await;
```
`tools/aurago-tui/src/api/sse.rs:75` and `:91`:
```rust
let mut req = client.get(url).header("Origin", origin);   // no .timeout(None) override
...
let mut stream = resp.bytes_stream().eventsource();
```
reqwest documents `ClientBuilder::timeout` as: *"Enables a total request timeout. The timeout is applied from when the request starts connecting until the response body has finished. Also considered a total deadline."* (<https://docs.rs/reqwest/0.13.5/reqwest/struct.ClientBuilder.html>). The TUI pins `reqwest 0.13.5` (`tools/aurago-tui/Cargo.lock:2005-2007`).

**Why it matters**
`/events` is the AuraGo agent event stream — it carries `llm_stream_delta`, tool-call previews and status events for the *whole duration of an agent turn*. A whole-request deadline that covers the response body means the body is aborted 30 seconds after the connection opens, even while events are actively arriving. `connect_sse_once` then returns and the reconnect loop re-establishes the subscription with a backoff. Consequences:
- any agent turn longer than 30 s loses the tail of its event stream;
- the server-side subscriber is dropped and re-created, so state that was streamed once (and not replayed) is lost for good;
- the TUI shows the stream as "connected" (`sse.rs:89` sends `AgentStatus("connected")` *before* the body is consumed), so the failure is invisible to the user.

This is the classic "total deadline applied to a streaming endpoint" bug. The identical defect is described and fixed upstream in spiceai/spiceai#12589 ("`spice chat` truncates any streamed response longer than 30s").

**Fix** — build a dedicated streaming client in `tools/aurago-tui/src/api/mod.rs` instead of reusing the control-plane client, e.g. add `ApiClient::stream_client()` that is constructed with `.connect_timeout(Duration::from_secs(30))` and `.read_timeout(Duration::from_secs(60))` and **no** `.timeout(...)`; then use it at `tools/aurago-tui/src/main.rs:892`. As a minimum, call `.timeout(None)` on the request at `tools/aurago-tui/src/api/sse.rs:75`.

**Confidence** — **Confirmed** (code path read end-to-end + upstream documented semantics + pinned version). Runtime verification: hold `/events` open for >30 s against a live AuraGo and watch the TUI reconnect; or add a unit test that asserts `ApiClient`'s SSE request carries no total deadline.

---

### F-2 [HIGH] ~490 browser smoke tests never run in CI, and silently SKIP when the browser is absent

**Location**
- `ui/precision_workspace_browser_test.go:13-18` (`requirePrecisionBrowserSmoke`, ~294 callers)
- `ui/destructive_flow_browser_test.go:47-50` (`newSmokeBrowser`), `:54-60` (`newSmokeBrowserWith`), `:76-79` (launch failure)
- `.github/workflows/security-gates.yml:102`

**Evidence** — `ui/precision_workspace_browser_test.go:13-18`:
```go
func requirePrecisionBrowserSmoke(t *testing.T) {
	t.Helper()
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 to run the headless browser smoke test")
	}
}
```
`ui/destructive_flow_browser_test.go:54-60` and `:76-79`:
```go
	bin, ok := browserExecutable()
	if !ok {
		t.Skip("headless browser smoke test requires Chrome or Edge")
	}
	...
	u, err := l.Launch()
	if err != nil {
		t.Skipf("headless browser smoke test skipped; browser launch failed: %v", err)
	}
```
CI — `.github/workflows/security-gates.yml:101-102`:
```yaml
      - name: Run Go tests
        run: go test ./... -p 1
```
A repository-wide grep for `AURAGO_RUN_BROWSER_SMOKE` outside `ui/` returns **no match in `.github/`** — no workflow, job or step sets it.

**Why it matters**
There are two independent skip gates in series:
1. the env-var gate — every browser test is skipped in *every* CI run because CI never sets `AURAGO_RUN_BROWSER_SMOKE=1`;
2. the missing-browser gate — even when opted in locally, the whole suite skips if Chrome/Edge is not on the box.

The consequence is that `go test ./... -p 1` reports **`ok aurago/ui`** for a suite in which the entire browser-driven half never executed. In non-verbose Go output a `SKIP` is invisible; only `-v` or `-json` reveals it. A real browser regression (a broken modal, a dead button, a JS runtime error on load) ships green because the thing that would have caught it was absent. The guard is *nominally* present and *practically* absent — the exact "test that passes when the thing it guards is absent" pattern.

**Fix** — in `.github/workflows/security-gates.yml`, add a job that (a) installs Chrome (e.g. `browser-actions/setup-chrome` or `apt-get install chromium`) and (b) runs `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'Browser|Smoke' -v`. Additionally, make the skip loud: in `ui/destructive_flow_browser_test.go:54`, replace the bare `t.Skip` with a `t.Skipf` that also writes to stderr, and add a CI step that fails the build if the browser tests reported zero non-skipped runs (e.g. parse `go test -json` output and assert `>0` executed browser tests).

**Confidence** — **Confirmed** (both skip sites read verbatim; CI command read verbatim; absence of the env var established by grep over `.github/` with no matches).

---

### F-3 [MEDIUM] Rust TUI persists the session bearer cookie to disk with default (world-readable) permissions

**Location** — `tools/aurago-tui/src/api/auth.rs:278-284`, `tools/aurago-tui/src/config.rs`

**Evidence** — `tools/aurago-tui/src/api/auth.rs:278-284`:
```rust
pub fn save_session_cookie(cookie: &str, path: &std::path::Path) -> Result<()> {
    if let Some(parent) = path.parent() {
        std::fs::create_dir_all(parent)?;
    }
    std::fs::write(path, cookie)?;
    Ok(())
}
```
The path is `session.txt` under the platform config dir (`tools/aurago-tui/src/config.rs`, project-dirs based). The cookie is the AuraGo session credential and is replayed on every request (`tools/aurago-tui/src/main.rs:894`, `cookie = client.get_session_cookie()` → `Cookie:` header at `tools/aurago-tui/src/api/sse.rs:76-78`).

**Why it matters**
`std::fs::write` → `File::create` → mode `0o666 & ~umask`. On a typical Unix host (`umask 022`) the session file is created **0644 — readable by every local user**. On a shared or multi-tenant machine any other account can read the session token and drive the AuraGo API with the user's identity (chat history, files, desktop control). The desktop directory that holds it is also the one the desktop code treats as "as trusted as the vault" (`internal/ruview/runtime.go:318-321`), so the credential sits next to the protected material without matching its protection level.

**Fix** — in `tools/aurago-tui/src/api/auth.rs:278`, create the file with owner-only permissions instead of `std::fs::write`:
```rust
use std::os::unix::fs::OpenOptionsExt;
let mut f = std::fs::OpenOptions::new().write(true).create(true).truncate(true).mode(0o600).open(path)?;
f.write_all(cookie.as_bytes())?;
```
plus a `set_permissions(0o600)` after write for an already-existing file, and the Windows equivalent (`icacls`-style ACL restriction) or an explicit note that Windows relies on `%APPDATA%` ACLs.

**Confidence** — **Confirmed** (code read verbatim). Not runtime-verified on a Unix host — verify with `ls -l ~/.config/aurago-tui/session.txt` after logging in via the TUI.

---

### F-4 [MEDIUM] The primary web UI ships `script-src 'unsafe-inline'`, so there is no second layer behind any HTML injection

**Location** — `internal/server/server.go:2274-2282` (and the inline handlers across `ui/*.html`)

**Evidence** — `internal/server/server.go:2274-2282`:
```go
		// Content Security Policy
		// NOTE: unsafe-inline is still required by legacy inline SPA handlers/styles.
		// unsafe-eval is intentionally omitted; CodeMirror and UI bundles are served
		// as prebuilt static assets and must not require runtime eval.
		// TODO: Replace unsafe-inline with nonce-based CSP after moving inline UI
		// handlers/styles into bundled JS/CSS.
		connectSrc := "connect-src 'self' blob: ws: wss: https://api.open-meteo.com ... ; "
		csp := "default-src 'self'; " +
			"script-src 'self' 'unsafe-inline' 'wasm-unsafe-eval'; " +
			"style-src 'self' 'unsafe-inline'; " +
```
The UI relies on inline handlers: `ui/cheatsheets.html:108` (`onclick="closeModal('edit-modal')"`), `ui/containers.html:115`, `ui/knowledge.html:121` (`onclick="document.getElementById('file-upload-input').click()"`), `ui/login.html:96` (`onkeydown="if(event.key==='Enter') submitLogin()"`), `ui/media.html:144` — 100+ occurrences of `on(click|load|error|mouseover|submit)=` in `ui/*.html`.

**Why it matters**
With `'unsafe-inline'` in `script-src`, an HTML injection anywhere in the main origin is *immediately* executable: `<img src=x onerror=...>` runs. The CSP therefore provides no containment for the one class of bug the chat sanitizer is defending against, and it blocks the obvious hardening step (nonce-based CSP). The code comments say so explicitly, which makes this **acknowledged debt**, not a surprise — but the debt is load-bearing: the entire XSS defence of the chat surface currently rests on one hand-written sanitizer (see F-5).

**Fix** — execute the existing TODO: move the inline `on*=` handlers in `ui/*.html` into the bundled JS (the desktop SPA already has a module loader at `ui/js/desktop/core/module-loader.js:685` that could carry them), then switch `internal/server/server.go:2282` to a per-request nonce (`script-src 'self' 'nonce-<random>'`) and drop `'unsafe-inline'`. `style-src 'unsafe-inline'` can be tightened in the same pass.

**Confidence** — **Confirmed** (middleware and CSP string read verbatim; inline handlers confirmed by grep).

---

### F-5 [MEDIUM] The chat's hand-written HTML sanitizer is the highest-risk sink and is the one Markdown sink *not* covered by the DOMPurify lint

**Location**
- `ui/js/chat/bundles/chat-runtime.bundle.js:433-481` (`sanitizeRenderedHTML`), allowlists `:357-375`, `isSafeHref` `:417-431`, `isSafeMediaSource` `:484-496`
- `ui/security_lint_test.go:266-298` (`TestMarkdownSinksUseDOMPurify`)

**Evidence** — `ui/security_lint_test.go:269-273`:
```go
	files := []string{
		filepath.Join("js", "desktop", "apps", "cheater.js"),
		filepath.Join("js", "desktop", "apps", "viewer.js"),
		filepath.Join("js", "skills", "main.js"),
	}
```
The chat renderer is not in that list, and it is the only Markdown sink that handles **model output**. The chat sanitizer is a hand-rolled allowlist (`chat-runtime.bundle.js:357-375`) rather than DOMPurify, even though `DOMPurify` is already vendored (`ui/js/vendor/purify.min.js`, lazily loaded by `ui/js/desktop/core/module-loader.js` per `ui/security_lint_test.go:289-292`) and used in 11 other files.

To be fair and accurate: **the sanitizer itself is sound.** I read it end-to-end —
- `on*` attributes are removed (`chat-runtime.bundle.js:448`);
- `iframe` gets `sandbox="allow-scripts"` **overwritten** via `setAttribute` and `allow` removed (`:474-477`), so an author-supplied `sandbox="allow-scripts allow-same-origin"` cannot be smuggled in;
- `href` is protocol-checked (`:452` → `isSafeHref` `:417-431`, which rejects `javascript:`/`data:` and `//host`);
- `src`/`poster` are checked by `isSafeMediaSource` (`:455`, `:484-496`), which requires **same-origin AND** one of a fixed path prefixes (`/files/`, `/tts/`, `/img/`, …);
- parsing happens inside a `<template>` (`:436`), so no script executes during parse.

**Why it matters**
The gap is **coverage, not correctness**: a bespoke sanitizer protecting the single most attacker-reachable surface (LLM output, which can be steered by prompt injection from fetched web content) is the one sink the security lint does not police. If someone later relaxes a tag/attribute in the allowlist, or if a new sink is added to the chat pipeline, nothing fails. The project's own stated pattern is "Markdown sinks use DOMPurify"; the chat is the exception, and the exception is silent.

**Fix** — two complementary changes: (a) extend `ui/security_lint_test.go:266` to also assert that `ui/js/chat/chat-messages.js` (and its bundle) route through a sanitizer, and (b) add a behavioural test (not a marker test) that feeds `sanitizeRenderedHTML` a corpus — `<img src=x onerror=alert(1)>`, `<a href="javascript:alert(1)">`, `<iframe src="https://evil.tld" sandbox="allow-scripts allow-same-origin">`, `<svg onload=…>`, `<img src="https://evil.tld/p.gif">` — and asserts the output contains no `on*` attribute, no `javascript:` URL and no off-origin `src`. `scripts/test-ui-regressions.mjs` already demonstrates the `node:vm` sandbox technique for exactly this kind of behavioural JS test.

**Confidence** — **Confirmed** (both the sanitizer and the lint's file list were read verbatim).

---
### F-6 [MEDIUM] The three top coupling hubs assert *source text*, not behaviour

**Location**
- `ui/chat_regression_test.go:47-90` (`TestChatFrontend_ToolLeakSanitizerPatternsRemainPresent`)
- `ui/desktop_window_menu_test.go:404-464` (`readDesktopAssetText`)
- `ui/modal_regression_test.go:61-68` (`mustReadUIFile`)

**Evidence** — `ui/chat_regression_test.go:65-79`:
```go
	requiredMessageMarkers := []string{
		"function stripLeakedToolMarkup",
		"minimax:tool_call",
		"<invoke\\b",
		"<parameter\\b",
		"(action|tool|tool_call|tool_name)",
		"\"parameters\"",
		"\\[Suggested next step\\]",
		"containsLeakedToolMarkup(text)",
	}
	for _, marker := range requiredMessageMarkers {
		if !strings.Contains(msg, marker) {
			t.Fatalf("%s is missing expected regression marker %q", messagesPath, marker)
		}
	}
```
`ui/desktop_window_menu_test.go:404-437` — the helper reads a UI asset from the embedded FS and, if the asset contains `loadBundle('/js/desktop/bundles/…')`, **follows the reference and returns the generated bundle's text instead of the loader's text**:
```go
func readDesktopAssetText(t *testing.T, path string) string {
	...
	text := string(data)
	if strings.Contains(text, "loadBundle(") {
		for _, line := range strings.Split(text, "\n") {
			...
			bundleData, err := Content.ReadFile(filepath.ToSlash(strings.TrimPrefix(bundle, "/")))
			...
			return string(bundleData)
```
`ui/modal_regression_test.go:61-68` is a plain `os.ReadFile` wrapper, and its callers assert CSS substrings, e.g. `ui/modal_regression_test.go:52-58`:
```go
	openBlock := cssBlock(t, css, ".modal-overlay.open,")
	if !strings.Contains(openBlock, ".modal-overlay.active") {
		t.Fatal("precision-pages.css missing .modal-overlay.active companion rule")
	}
```

**Why it matters**
These ~1 200 assertions are *string presence* checks over JavaScript and CSS source. That is a real, if limited, technique: it survives the bundling step (which is why `readDesktopAssetText` follows `loadBundle`), and it catches accidental deletions. But it provides **false confidence** in both directions:
- *False negatives*: renaming `stripLeakedToolMarkup` → `stripToolMarkup` (a pure refactor) fails ~1 200 tests.
- *False positives*: a **behaviourally broken** `stripLeakedToolMarkup` that still contains the literal marker strings — e.g. one whose regex was widened to `/<invoke.*?>/s` — still passes every test. The test named "ToolLeakSanitizerPatternsRemainPresent" asserts only that the *patterns remain present*, never that leaked tool markup is actually stripped.

The repo already contains the better pattern: `scripts/test-ui-regressions.mjs` has **341 `node:vm` call sites** — it extracts functions from source and *executes* them in a sandbox, asserting on the transformed output. The Go-side hubs do not use that technique.

**Fix** — pick the highest-value few and convert them: in `ui/chat_regression_test.go`, replace the marker loop with a behavioural test that imports/executes `stripLeakedToolMarkup` (via a `node:vm` harness like `scripts/test-ui-regressions.mjs`, or a tiny `go:generate`-style node test) and asserts that a sample leaking payload (`minimax:tool_call` with `<invoke name=…>`) comes back stripped. Keep the cheap marker assertions as a smoke net, but do not count them as coverage of the sanitiser's behaviour.

**Confidence** — **Confirmed** (all three helpers and one representative marker test read verbatim).

---

### F-7 [LOW] A key missing from the *English* bundle renders the raw key path to the user, and the JS-side safe fallback is barely used

**Location** — `ui/js/shared/shared-core.js:18-27` (`t`), `:37-48` (`tOr`), `ui/js/chat/modules/integrations-drawer.js:19-21`

**Evidence** — `ui/js/shared/shared-core.js:18-27`:
```js
function t(k, p) {
    const dict = typeof I18N !== 'undefined' ? I18N : null;
    let s = (dict && dict[k]) || k;
    // A replacer function inserts the value literally; a replacement string would expand $&, $1 and $$.
    if (p) Object.entries(p).forEach(([a, b]) => {
        s = s.replaceAll('{{' + a + '}}', () => b);
        s = s.replaceAll('{' + a + '}', () => b);
    });
    return s;
}
```
`ui/js/chat/modules/integrations-drawer.js:19-21` repeats the pattern locally:
```js
    function t(key) {
        return typeof I18N !== 'undefined' && I18N[key] ? I18N[key] : key;
    }
```
The safe alternative exists in the same file, `ui/js/shared/shared-core.js:37-48`:
```js
function tOr(k, fallback, p) {
    const dict = typeof I18N !== 'undefined' ? I18N : null;
    if (!dict || !Object.prototype.hasOwnProperty.call(dict, k)) {
        return typeof fallback === 'string' ? fallback : (fallback == null ? k : String(fallback));
    }
    ...
```
`graft grep 'tOr(' --in ui/js` → **15 hits in 6 files only**; the overwhelming majority of call sites use `t()`.

**Why it matters**
There are **two** server-side fallbacks before a raw key can reach the screen, so the exposure is much narrower than the JS alone suggests. `internal/i18n/i18n.go:284-297` merges the English bundle first and then overlays the selected locale ("Merge English first so missing locale keys fall back individually, then overlay the selected locale"), and `internal/i18n/i18n.go:372-374` documents that `T()` falls back to English and only returns the key itself when English lacks it too. The loader also *reports* gaps: `internal/i18n/i18n.go:166-168` logs `missing_count` plus a sample of the missing keys.

The raw key therefore only reaches the user when the key is missing from the **English** bundle as well. That case is largely covered by `ui/i18n_lint_test.go:446` (`TestFrontend_StaticI18nKeysExistInEnglishBundle`), `:669` (`TestTranslations_AllKeysPresentInAllLanguages`) and `:1179` (`TestDesktopDynamicI18nPrefixKeysExistInAllLanguages`). The residual exposure is narrow and confined to **dynamically composed** keys (e.g. `ui/js/desktop/apps/kaggle.js:14` — ``const t = key => ctx.t('kaggle.' + key);``) and keys composed from server-supplied strings (`ui/js/chat/modules/integrations-drawer.js:108` — `t(item.message_key)`, where `message_key` arrives from the server), because neither the lint nor the loader's missing-key log can enumerate those ahead of time. When it does happen, the user sees an implementation path such as `chat.sessions_empty` instead of prose — ugly, not broken.

**Fix** — (a) in the surfaces where a raw key would be visible to a user (empty states, error banners, status labels), switch `t()` to `tOr()` with a human-readable fallback; (b) for `t(item.message_key)` at `ui/js/chat/modules/integrations-drawer.js:108`, validate `message_key` against the dictionary before use and fall back to `item.description`; (c) extend `ui/i18n_lint_test.go:1179` to cover every server-supplied `message_key` value, not just static prefixes.

**Confidence** — **Confirmed** for the code behaviour (both JS helpers, `internal/i18n/i18n.go:166-168` / `:284-297` / `:372-374`, `internal/i18n/section_fallback_test.go:8` and the `ui/lang/**` layout all read verbatim) and for the `tOr` usage count (graft grep over 724 indexed `ui/js` files). **Needs-verification** only for the concrete key-set gap: enumerate the `message_key` values the server can emit (grep the webhost registry under `internal/server`), check each against `ui/lang/chat/en.json`, and run `go test ./ui -run 'TestTranslations|TestFrontend_StaticI18nKeys'`.

---

### F-8 [MEDIUM] The desktop service layer performs no authorisation of its own; `AllowAgentControl` is advisory UI state only

**Location**
- `internal/desktop/types.go:25-44` (`Config`), `internal/desktop/service.go:740` (`Bootstrap` → `BootstrapPayload.AllowAgentControl`)
- `ui/js/desktop/apps/settings.js:112`
- enforcement: `internal/server/desktop_auth.go:23-28`

**Evidence** — `graft grep 'requireDesktopPermission|authorizeDesktop|checkPermission' --in internal/desktop` → **"no hits … in 122 indexed files"**. The desktop package contains no scope or permission check of any kind.

`internal/desktop/service.go:740` (inside `Bootstrap`, `:675-773`) publishes the flag to the browser:
```go
		AllowAgentControl:  cfg.AllowAgentControl,
```
and `ui/js/desktop/apps/settings.js:112` consumes it for **display only**:
```js
		settingInfo('desktop.setting_agent_control', boot.allow_agent_control ? ctx.t('desktop.on') : ctx.t('desktop.off'))
```
`graft grep 'allow_agent_control' --in ui/js` → exactly **one** hit, the display line above. No UI control is *gated* on the flag.

Enforcement does exist, but one layer up, in `internal/server/desktop_auth.go:23-28`:
```go
func requireDesktopPermission(s *Server, w http.ResponseWriter, r *http.Request, requiredScope string) bool {
	if !authenticateDesktopPermission(s, w, r, requiredScope) {
		return false
	}
	return checkDesktopOperation(s, w, r, desktopRequestOperation(r))
}
```
with 146 call sites across 57 files (`graft grep 'requireDesktopPermission' --in internal/server`), and a secure default in `internal/server/auth.go:1203`: any route that is not admin-protected, desktop-scoped or go2rtc-scoped requires the `admin` scope.

**Why it matters**
Today this is *not* exploitable — the HTTP surface is properly gated. It is a **defence-in-depth gap**: `internal/desktop.Service` is a trusted in-process object whose methods (`ReadFile`, `WriteFile`, `InstallApp`, `OpenURL`, `ConfigureBeszelAgent`, …) perform no scoping of their own, so every future caller inside the process inherits full authority. The flag that *looks* like a permission (`AllowAgentControl`) is actually a cosmetic toggle: a user with `desktop:read` sees "Agent control: Off" while the underlying service would happily execute an agent-control operation if a handler called it.

**Fix** — either (a) carry a scope/role on `internal/desktop.Config`/`Service` and assert it inside the service methods that mutate state (the desktop package already has `internal/desktop/AGENTS.md` documenting its contract — extend it), or (b) at minimum, make `AllowAgentControl` semantics explicit in `internal/desktop/types.go:28` and `internal/desktop/AGENTS.md`: "advisory display flag; enforcement lives in `internal/server/desktop_auth.go`". Option (b) is cheap and removes the misleading impression that the desktop layer enforces it.

**Confidence** — **Confirmed** for the code facts (no hits in 122 desktop files; flag published at `service.go:740`; single UI consumer; enforcement at `desktop_auth.go:23` and `auth.go:1164-1205`). The *exploitability* conclusion relies on `internal/server`, which is out of scope — I read only the two gate functions, not all 146 call sites.

---

### F-9 [LOW] `internal/desktopstore/service.go` is a single 3 085-line file; `internal/desktop/types.go` mixes schema and policy

**Location** — `internal/desktopstore/service.go` (3 085 LOC), `internal/desktop/types.go` (45.6 KB), `internal/desktop/openscad_container.go` (51 KB), `internal/desktop/looper.go` (44 KB)

**Evidence** — `graft skeleton internal/desktopstore/service.go` shows one file carrying, among others: schema migration (`migrateLocked` L280-346, `ensureColumn` L426-435), backup (`backupBeforeMigrationLocked` L352-380), interrupted-install recovery (`recoverInterruptedOperationsLocked` L462-516), install (`install` L1147-1300), update (`update` L1331-1498), uninstall (`uninstall` L1650-1729), port allocation (`allocatePortBindings` L1883-1905) and credential handling (`ExposedCredentials` L1001-1028, `ConfigureBeszelAgent` L1059-1145).

**Why it matters**
Installation state transitions, migration, recovery and credential projection live in one unit that no single reviewer can hold in their head. `ExposedCredentials`/`ConfigureBeszelAgent` in particular is credential-adjacent code that would benefit from its own unit and its own focused tests. It is a maintainability finding, not a defect — the code is at least structured into small, well-named methods.

**Fix** — split `internal/desktopstore/service.go` into `migrations.go`, `install.go`, `update.go`, `uninstall.go`, `ports.go` and `credentials.go`, keeping `Service` as the shared receiver. No behaviour change; the existing tests reference exported methods only, so the split is mechanical.

**Confidence** — **Confirmed** (file size and method inventory measured directly).

---

### F-10 [MEDIUM] 136 raw-variable `innerHTML` assignments across 74 `ui/js` files, with per-file (not repo-wide) lint coverage

**Location** — `ui/js/**` (74 files), `ui/security_lint_test.go:266-298`

**Evidence** — `graft grep '\.innerHTML\s*=\s*[A-Za-z_$][A-Za-z0-9_$.]*\s*[;+]' --in ui/js` → **"136 hits in 108 symbols across 74 files"**. Representative sinks:
- `ui/js/config/components.js:107` — `el.innerHTML = html;`
- `ui/js/desktop/apps/findit.js:439` — `template.innerHTML = app;`
- `ui/js/desktop/apps/bookshelf.js:432` — `by('[data-dialog-body]').innerHTML = body;`
- `ui/js/desktop/apps/synth-studio-stage.js:158` — `q('.ss-scene-pads').innerHTML=markup;`
- `ui/js/desktop/apps/easydrag-home.js:252` — `grid.innerHTML = banner + list.map(f => …`

The security lint polices a fixed file list only (`ui/security_lint_test.go:269-273` — `cheater.js`, `viewer.js`, `skills/main.js`).

**Why it matters**
Each of these is a potential HTML-injection sink. I verified the ones I read: the chat markdown path is safe (F-5), `ui/js/chat/modules/session-drawer.js:173-191` escapes every interpolation via `escapeHtml`/`escapeAttr`, and `ui/js/chat/modules/integrations-drawer.js:23-31` uses a correct `div.textContent` → `div.innerHTML` escaper. But `findit.js:439`, `bookshelf.js:432`, `synth-studio-stage.js:158` and `easydrag-home.js:252` were **not** verified line-by-line, and the lint would not catch a regression in any of them.

**Fix** — extend `ui/security_lint_test.go` from a fixed file list to a **repo-wide rule** over `ui/js/**`: flag any `innerHTML` assignment whose right-hand side is not (a) a literal without interpolation, or (b) a call to a whitelisted safe-builder (`escapeHtml`, `escapeAttr`, `DOMPurify.sanitize`, `sanitizeRenderedHTML`, `textContent`-based helpers). Turn the current 136 hits into a review list, then migrate the survivors to a single `setHtml(el, trustedMarkup)` helper so the rule has one exception to allow.

**Confidence** — **Needs-verification** for the individual sites (I did not read all 136; the four quoted above are unverified). **Confirmed** for the count and for the coverage shape of the lint.

---

### F-11 [LOW] Four independent Markdown renderers with three different escaping strategies coexist in `ui/js`

**Location** — `ui/js/cheatsheets/main.js:434`, `ui/js/desktop/chat-renderer.js:71`, `ui/js/desktop/apps/code-studio/agent.js:98`, `ui/js/desktop/apps/viewer.js:234`

**Evidence** — `graft grep 'renderMarkdown|markdownToHtml|sanitizeHtml|DOMPurify' --in ui/js` locates four separate `renderMarkdown` implementations. They differ materially in strategy:
- `ui/js/cheatsheets/main.js:434` — **escape-first**: `esc(md)` and only then apply the block/inline regexes;
- `ui/js/desktop/chat-renderer.js:71` — **render-then-sanitise** via `window.AuraChatCore.sanitizeRenderedHTML`;
- `ui/js/desktop/apps/code-studio/agent.js:98` — **render-then-sanitise** with its own local `sanitizeMarkdownHref`;
- `ui/js/desktop/apps/viewer.js:234` — render-then-`sanitizeMarkdownHTML` (DOMPurify).

**Why it matters**
Four implementations of "turn Markdown into safe HTML" means four places a future change can introduce a hole, and four different behaviours for the same user-visible content. The escape-first variant is safe for the same reason the others are, but only by construction — a later edit that moves the `esc()` call after a regex that reintroduces `<` would silently break it, and no shared test covers it.

**Fix** — extract a single `ui/js/shared/markdown.js` exporting `renderSafeMarkdown(src, {sanitiser})`, have the four call sites delegate, and add one behavioural test that runs the shared helper over the XSS corpus described in F-5.

**Confidence** — **Confirmed** (all four definitions located; the cheatsheets escape-first and code-studio `sanitizeMarkdownHref` implementations read verbatim).

---

### F-12 [LOW] `ui/js` leaks the shared `escapeHtml`/`escapeAttr` contract: many near-duplicate local copies

**Location** — `ui/js/chat/bundles/chat-runtime.bundle.js:400` & `:408`, `ui/js/chat/modules/integrations-drawer.js:23` & `:29`, `ui/js/chat/ui-icons.js:157`, and further copies returned by `graft grep 'function escapeHtml|const escapeHtml' --in ui/js`.

**Evidence** — two different escaping contracts coexist:
```js
// chat-runtime.bundle.js:400
function escapeHtml(value) {
    return String(value)
        .replace(/&/g, '&amp;').replace(/</g, '&lt;')
        .replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}
// integrations-drawer.js:29
function escapeAttr(str) {
    return String(str || '').replace(/&/g, '&amp;').replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}
```
`escapeHtml` in the chat bundle does **not** escape `'`; `escapeAttr` does. A copy used in an attribute context without `'` escaping would allow attribute breakout.

**Why it matters**
The escaping contract is the last line of defence for many sinks (F-10). Duplicated, subtly-different copies mean a caller can pick the wrong one. The differences are small today but they are exactly the kind that matter.

**Fix** — move `escapeHtml`/`escapeAttr` into `ui/js/shared/shared-core.js` (which already exports `t`/`tOr` at `:18`/`:37`), make the escaping identical everywhere (escape `& < > " '` in both), and delete the local copies. `ui/security_lint_test.go` already has `TestI18NHTMLUsesSafeTextNodeRendering` (`:300`) to extend.

**Confidence** — **Confirmed** for the two quoted copies and for the existence of many more via graft grep; **Needs-verification** for the complete list of copies (the grep was capped).

---
### F-13 [LOW] Rust TUI: `Action::Quit` does not cancel the SSE handle or the tracked background tasks

**Location** — `tools/aurago-tui/src/actions.rs:27-29`, `tools/aurago-tui/src/main.rs:734-736`, `tools/aurago-tui/src/app.rs:667-682`

**Evidence** — `tools/aurago-tui/src/actions.rs:27-29`:
```rust
		Action::Quit => {
			app.should_quit = true;
		}
```
`tools/aurago-tui/src/main.rs:734-736`:
```rust
		if app_lock.should_quit {
			break;
		}
```
`tools/aurago-tui/src/app.rs:667-682`:
```rust
    pub fn spawn_tracked(&mut self, handle: tokio::task::JoinHandle<()>) {
        self.background_tasks.push(handle);
    }
    pub fn abort_all_background(&mut self) {
        for handle in self.background_tasks.drain(..) {
            handle.abort();
        }
    }
    pub fn prune_finished_tasks(&mut self) {
        self.background_tasks.retain(|h| !h.is_finished());
    }
```
`abort_all_background` is called on screen navigation (`app.rs:617`), and `prune_finished_tasks` is called periodically (`app.rs:608`, every 10 ticks) and on navigation (`app.rs:617`) — so the `Vec` does not grow without bound. **But** the quit path calls neither: `Action::Quit` only sets the flag, and the main loop just `break`s.

**Why it matters**
No zombie *threads* result — `main` is `#[tokio::main]` (`tools/aurago-tui/src/main.rs:53`), and dropping the runtime cancels spawned async tasks. The residue is that the SSE reconnect loop (`tools/aurago-tui/src/api/sse.rs`, `loop { connect_sse_once(...) }`) and any in-flight background request are cancelled by runtime teardown rather than by an orderly shutdown: the server observes an abruptly dropped TCP connection instead of a clean close, and any server-side work still streaming for that client is abandoned. On a long-running agent turn, quitting the TUI leaves the server-side subscriber alive until it notices the disconnect.

**Fix** — in `tools/aurago-tui/src/actions.rs:27`, before setting the flag, abort the SSE handle and the tracked tasks:
```rust
		Action::Quit => {
			if let Some(h) = sse_handle.take() { h.abort(); }
			app.abort_all_background();
			app.should_quit = true;
		}
```
(`sse_handle` is already threaded through `handle_action` — `tools/aurago-tui/src/actions.rs:23` — so this is a one-line change.)

**Confidence** — **Confirmed** (all four sites read verbatim; `#[tokio::main]` confirmed at `main.rs:53`). Runtime behaviour of the teardown was not observed.

---

## Areas verified in good shape (reported honestly — no findings)

These were checked specifically because the brief asked about them; they hold up.

1. **No dynamic-code sinks in the UI.** `graft grep '\beval\(|new Function\(|setTimeout\(\s*['"]' --in ui/js` → *"no hits … in 724 indexed files"*. No `eval`, no `new Function`, no string-bodied timers.
2. **The chat Markdown sanitizer is correct.** Read end-to-end (`ui/js/chat/bundles/chat-runtime.bundle.js:417-496`): `on*` attributes stripped, `iframe` sandbox overwritten via `setAttribute` (so `allow-same-origin` cannot be injected), `href` protocol-checked (`javascript:`/`data:` rejected), `src`/`poster` restricted to same-origin path prefixes, parsing confined to an inert `<template>`. The gap is lint coverage (F-5), not correctness.
3. **Desktop path-traversal defence is thorough.** `internal/desktop/service.go:809` (`ResolvePath`) → `isWithinPath` (`:898`) → `validatePathComponentsWithinRoot` (`:906`), which walks every path component with `os.Lstat`, rejects symlinks whose target escapes the root, and double-checks with `filepath.EvalSymlinks` (`:955-961`). `resolveRenamePath` (`:835`) correctly validates the *parent* of the target.
4. **RuView native launch is safe.** `internal/ruview/runtime.go:313` launches a **hard-coded** binary name (`sensing-server`) from a package whose files are hash-verified first (`verifyNative`, `:295`), with a scrubbed environment (`:326-330`). No command injection surface. The upstream proxy disables redirects and strips `Set-Cookie`/`Location`/CSP (`internal/ruview/proxy.go:133-138`).
5. **Server-side desktop authorisation is comprehensive.** 146 `requireDesktopPermission` call sites across 57 files, three scopes (`internal/server/desktop_auth.go:9-11`), and a **secure default** in `validRouteBearer` (`internal/server/auth.go:1203`) — any route not explicitly scoped requires the `admin` scope. `isDesktopScopedAPIPath` (`:14-21`) failing to match a path therefore fails *closed*.
6. **Event-listener / timer hygiene in the desktop SPA is strong.** 300 `removeEventListener` call sites across 114 `ui/js/desktop` files; `ui/js/desktop/apps/log-viewer.js:404-414` (`closeStream`) closes the `EventSource` and clears the timer, and `dispose` (`:726-732`) calls it; `ui/js/desktop/apps/ruview.js:305-320` clears its 1 s ticker and aborts its `AbortController` on dispose.
7. **NASSCAD log output is sanitised before rendering.** `internal/desktop/bundled_apps/nasscad/nasscad_logs.js` escapes via `_escHtml` before every `innerHTML` write, including the MEDUSA engine lines pulled from the local HTTP endpoint (`nasscad_logs.js:131`). The only unescaped interpolation is the locally generated timestamp `s.when` (`:126`), which is not attacker-controlled.
8. **Security and i18n lint suites exist and are wired into CI.** `ui/security_lint_test.go` (11 tests) and `ui/i18n_lint_test.go` (20 tests) enforce DOMPurify on Markdown sinks, `innerHTML`-free error rendering, hardcoded-toast detection, and locale key parity. `npm run test:ui-regressions` runs in CI (`.github/workflows/security-gates.yml:46`).
9. **The i18n layer has a real per-key English fallback.** `internal/i18n/i18n.go:284-297` merges the English bundle before overlaying the selected locale; `internal/i18n/i18n.go:372-374` documents `T()`'s key-level fallback; `internal/i18n/i18n.go:166-168` logs missing keys with a count and a sample; `internal/i18n/section_fallback_test.go:8` pins the behaviour with a test. The dictionaries are 2 049 JSON files across 30 sections x 17 languages (`ui/lang/**`).
10. **`scripts/test-ui-regressions.mjs` is not merely string-matching.** It has **341 `node:vm` call sites** — it extracts functions from source and *executes* them in a sandbox, asserting on behaviour. This is the pattern the Go-side hubs should adopt (F-6).
11. **`internal/desktop.Config` carries no secrets.** `internal/desktop/types.go:25-44` holds only booleans, paths, and limits. Credentials live in the server layer and in `internal/desktopstore`, not in the desktop service configuration.

---

## Metrics

| Metric | Value |
|---|---|
| Area | Web UI, Desktop App & TUI (`ui/**`, `scripts/**`, `internal/desktop/**`, `internal/desktopstore/**`, `internal/ruview/**`, `tools/aurago-tui/**`) |
| Total LOC in scope (measured) | ~829 000 (ui/js 550 634 incl. `ui/js/vendor` · ui/css 144 499 · scripts 51 314 · internal/desktop 148 016 · internal/desktopstore 13 500 · internal/ruview 6 032 · tools/aurago-tui 9 652 · ui/*.html 4 850) plus `ui/lang` (2 049 locale JSON files, not LOC-counted) |
| Files read verbatim (source in context) | ~42 |
| Verbatim source lines read | ~1 400 (Go / JS / Rust) |
| Files covered by graph queries | 8 175 indexed repo-wide; 724 in `ui/js`; 576 in `ui/js/desktop`; 122 in `internal/desktop`; 41 in `internal/desktopstore`; 26 in `internal/ruview`; 1 256 in `internal/server` |
| graft / graph calls | ≈ 70 (≈50 graft CLI + ≈20 raw `grep`/`read` on Rust, YAML, HTML, JSON, `.mjs`); 3 failed (graft MCP `check_freshness` timeout, `graft ask` timeout, one `graft map` bad subcommand) |
| GitNexus CLI calls | 0 (graft CLI provided equivalent coverage; GitNexus was not needed) |
| **Findings** | **13** |
| — CRITICAL | **0** |
| — HIGH | **2** (F-1, F-2) |
| — MEDIUM | **6** (F-3, F-4, F-5, F-6, F-8, F-10) |
| — LOW | **5** (F-7, F-9, F-11, F-12, F-13) |
| Confirmed / Probable / Needs-verification | 10 Confirmed · 0 Probable · 3 Needs-verification (F-7 server-supplied `message_key` set, F-10 individual sinks, F-12 full copy list) |
| Areas verified clean | 10 (listed above) |

*Note on the severity counts:* totals are 2 HIGH + 6 MEDIUM + 5 LOW = 13, 0 CRITICAL. F-7 was drafted as MEDIUM and **downgraded to LOW** after reading `internal/i18n/i18n.go` and `ui/lang/**`: the server merges English first and falls back per key, so the raw-key path is only reachable for keys missing from the English bundle.

---

## Not covered / follow-ups

**Honest gaps in this pass**
- `internal/desktop/**` is ~148 k LOC across ~128 files; only `service.go`, `types.go`, `config.go` (via graft) and `nasscad_logs.js` were read in depth. `openscad_container.go` (51 KB), `looper.go` (44 KB), `code_container.go`, `sftp_handlers.go`, `vnc_proxy.go`, `ssh_proxy.go`, `serial_proxy.go` were **not** read — the proxy handlers and the container lifecycle are the highest-value next targets.
- `internal/desktopstore/service.go` (3 085 LOC) was reviewed via skeleton only. The install/update/uninstall state machine and `ExposedCredentials`/`ConfigureBeszelAgent` deserve a dedicated credential-handling pass.
- `internal/ruview/manager.go` (629 LOC) and `lifecycle.go` were not read.
- 136 `innerHTML` sinks across 74 `ui/js` files: only the chat path and two drawer renderers were verified line-by-line.
- `ui/lang/**` (2 049 locale JSON files) was surveyed for layout and fallback behaviour but **not** key-by-key diffed; `internal/i18n` was read only as evidence for F-7. Key-coverage conclusions for dynamic keys are therefore **Needs-verification**.
- `ui/partials/**` (13 measured LOC — the directory is effectively empty; my extension filter may have missed it) and `ui/css/**` were not audited.
- `scripts/*.mjs` other than `test-ui-regressions.mjs` (18 more files, ~48 k LOC) were listed but not read.
- No build, no `go vet`, no test execution — all findings are source-derived.

**Recommended next passes (in priority order)**
1. **Make the browser smoke tests real.** Install Chrome in CI and set `AURAGO_RUN_BROWSER_SMOKE=1`; add a CI assertion that a non-zero number of browser tests actually executed (F-2). This is the single highest-value change in this area — it converts ~490 nominal tests into real coverage.
2. **Fix the TUI SSE deadline** (F-1) and add a regression test that holds a stream open past the timeout.
3. **Add a behavioural sanitizer test** for the chat pipeline and extend `ui/security_lint_test.go` from a fixed file list to a repo-wide `innerHTML` rule (F-5, F-10).
4. **Convert the top Go-side marker tests to `node:vm` behavioural tests**, starting with `TestChatFrontend_ToolLeakSanitizerPatternsRemainPresent` (F-6) — the technique already exists in `scripts/test-ui-regressions.mjs`.
5. **Execute the CSP TODO**: move `ui/*.html` inline handlers into the bundled JS and switch to a nonce-based `script-src` (F-4).
6. **Document the desktop trust boundary** in `internal/desktop/AGENTS.md`: the desktop service performs no authorisation; enforcement lives in `internal/server/desktop_auth.go` (F-8). Cheap, removes a real source of future mistakes.
7. **Tighten the TUI session file to `0600`** (F-3).
8. **Split `internal/desktopstore/service.go`** and deduplicate the `escapeHtml`/`escapeAttr`/`renderMarkdown` helpers (F-9, F-11, F-12).

**Scope spillover (out of scope, noted once as required)**
- `internal/server/**` (~1 256 indexed files) is the layer that owns the desktop authorisation, the CSP and the bearer-scope routing; it was read only as *evidence* for F-4/F-8 and deserves its own audit.
- `internal/i18n/**` (the loader that merges `ui/lang/**` and serves it to `ui/js/shared/shared-core.js:18`) was read only as evidence for F-7; the dictionary files themselves (`ui/lang/**`, 2 049 files) are in scope and were surveyed, not key-by-key diffed.
