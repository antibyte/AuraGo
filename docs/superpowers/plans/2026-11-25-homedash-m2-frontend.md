# HomeDash M2 Frontend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make HomeDash a registered desktop app (F1 + F2 of decision D1): it opens from the start menu (category `comms`, capability `homedash`), lists dashboards, offers "Create starter dashboard" and "Empty dashboard" on first run, and shows a dashboard live and view-only: values over SSE, sparklines from history, alert tones from the host, stale and offline looks, and themed banners when the stream drops. No designer (M4), no commands (M3), no kiosk (M6).

**Architecture:** Nine new engine modules extend `window.HomeDashEngine`: `hd-tone` (pure alert tone, D5), `hd-api` (the only file that knows routes), `hd-stream` (one SSE connection per JS realm, owners with an active flag), `hd-store` (entity views, staleness, linger, history series, widget-catalog cache), `hd-catalog` (sizes, constraints, option defaults), `hd-render` (one page of widget hosts placed by custom properties), `hd-sheet` (in-root modal), `hd-view` (one `.hd-root`: attributes, tabs, stage, banner list) and `hd-app` (`E.mount`, the composition root). The desktop host is `homedash-home.js` (list and first-run state) plus `homedash.js` (`window.HomeDashApp`: chrome, visibility signal, policy, menus, window context). The host chrome is painted from `--vd-mat-*` (`desktop-app-homedash.css`); the canvas keeps the frozen M1 tokens and gets a tenth bundle part `homedash-page.css` for page geometry. Until the backend lands, `TestDesktopHomeDashBrowser` runs against a Go fixture that implements the frozen contract.

**Tech Stack:** Vanilla ES2020 classic IIFE scripts (no framework), CSS with custom properties and container queries, Node 22 tests with the existing vm harness and strict fake DOM (`npm run test:homedash`), Go 1.27 tests in `./ui` and `./internal/desktop` (rod browser test, `AURAGO_RUN_BROWSER_SMOKE=1`).

The code in this plan was written without running it. Implementers may fix errors (typos, small API slips, a wrong selector in a test), but must not change the server contract (backend plan section "API contract (frozen for M2)") or the engine contracts in `ui/js/homedash/AGENTS.md` without reporting it to the lead.

---

## Lead decisions on this plan (2026-11-25)

- **SCR-M2-1 and SCR-M2-2 are approved** as written in Task 7 (duration tile with production geometry; live tail of the
  sparkline after its history arrived). They change frozen structure; the token freeze stays unchanged. Their look is
  judged at the Lead review gate in all four families.
- Delete with confirmation (self-review item 13) stays in M2.
- The server contract is the section "API contract (frozen for M2)" of `2026-11-25-homedash-m2-backend.md`
  including that plan's "Lead amendments": `system` history answers `origin:"source"` from the server start, so the
  fixture's starter history is `origin:"source"` with points from the first request.

Amendments from the contract review (binding; every dispatch of the named task includes them):

- **L1 (Task 11, custom palette).** Before `E.colour.applyPalette`, drop the names that `applyLook` owns (`accent`,
  `light-on`, `focus`, `ink-on-accent`, `hud`) from the solved `tokens`; `applyLook` then writes them. Add a view test
  with a custom palette that asserts those properties come from `applyLook`.
- **L2 (Task 4, view `readonly`).** Approved as planned: the view's `readonly` means "a control that could act is
  blocked". Entities without any command capability (all `system` sensors) are never shown read-only and get no lock
  badge; a read-only desktop still locks entities that have command capabilities. The raw server value stays
  available as `serverReadonly` on the view for later milestones.
- **L3 (Task 4, staleness).** The server's per-entity `stale` flag is the truth for value age. A source status of
  `degraded` does not mark entities stale by itself (so `system:uptime`, which the server never marks stale, stays
  fresh). Connection loss (stream down past the 5 s grace) marks every entity stale; a source status of `unavailable`
  marks that source's entities offline. Update the store tests accordingly.
- **L4 (Task 18, fixture fidelity).** `entity` and `history` validate the `<source>:<native>` grammar first and answer
  `400 bad_request` for a malformed id, `404 entity_not_found` only for a well-formed unknown id. Every write route
  checks the read-only desktop (403) before validating its query. The first template creation keeps the contract's
  example id `d_m3x7q2k5a4bc`. Add fixture controls for `revoked` with `shutdown` and with `disabled`, and for
  `503 homedash_disabled` on all routes, and cover them in the browser test (reconnect after `shutdown`; disabled
  state after `disabled`). The other contract paths the fixture does not emit (401, `schema_too_new`,
  `limit_exceeded`, `slug_taken`, PUT rejections `limit`/`source_disabled`/`forbidden`) are listed as uncovered in the
  Task 19 hand-over.
- **L5 (Task 6, translations).** All 16 locales get real translations of every new key; no English filler. Add a Node
  test that fails when a non-English locale's value equals the English value, with an explicit allowlist for brand
  names, units and symbols (for example `HomeDash`, `CPU`, `Mbit/s`). German uses "Du" and real umlauts.

## File structure

Created:
- `ui/js/homedash/hd-tone.js` (`E.tone`), `hd-api.js` (`E.api`), `hd-stream.js` (`E.stream`), `hd-store.js` (`E.store`), `hd-catalog.js` (`E.catalog`), `hd-render.js` (`E.render`), `hd-sheet.js` (`E.sheet`), `hd-view.js` (`E.view`), `hd-app.js` (`E.mount`)
- `ui/js/desktop/apps/homedash-home.js` (`window.HomeDashHome`), `ui/js/desktop/apps/homedash.js` (`window.HomeDashApp`)
- `ui/css/homedash-page.css` (engine bundle part 10), `ui/css/desktop-app-homedash.css` (host chrome)
- `ui/img/desktop-icons/homedash.svg`
- `ui/testdata/homedash-m2/starter-dashboard.json`, `ui/testdata/homedash-m2/system-entities.json` (shared by Node tests and the Go fixture; testdata is never shipped)
- `scripts/homedash-tests/live-fakes.mjs` (fake clock, fake EventSource), and the tests `api`, `tone`, `stream`, `store`, `catalog`, `scr`, `render`, `sheet`, `view`, `app`, `home`, `host`, `wiring` (`*.test.mjs`)
- `ui/desktop_homedash_wiring_test.go`, `ui/desktop_homedash_fixture_test.go`, `ui/desktop_homedash_browser_test.go`, `ui/desktop_homedash_live_browser_test.go` (Task 20, only if the backend landed)
- `internal/desktop/homedash_app_test.go`
- `docs/superpowers/specs/2026-11-23-homedash/m2-handover-frontend.md` (force-add)

Modified:
- `ui/js/homedash/hd-icons.js` (six glyphs), `ui/js/homedash/widgets/hd-w-misc.js` and `widgets/hd-w-sensor.js` (SCR-M2-1/2, behaviour only), `ui/css/homedash-widgets.css` (SCR-M2-1 tile geometry)
- `scripts/build-ui-bundles.js` (`cssBundles` homedash parts), `ui/css/homedash.bundle.css` (generated), `scripts/homedash-tests/bundle.test.mjs`, `scripts/homedash-tests/harness.mjs`
- `ui/lang/homedash/*.json` (16), `ui/lang/desktop/*.json` (16, one key)
- `ui/js/desktop/core/module-loader.js`, `desktop-foundation.js`, `menus-and-routing.js`, `window-shell-runtime.js`, `ui/js/desktop/bundles/main.bundle.js` (generated)
- `internal/desktop/types.go` (`BuiltinApps` row)
- `ui/js/homedash/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md`, `documentation/homedash-visual-system.md`, root `AGENTS.md` (HomeDash row)

## Contracts used

Contract = section "API contract (frozen for M2)" of `docs/superpowers/plans/2026-11-25-homedash-m2-backend.md`.

| Contract item | Used by |
|---|---|
| Base path, envelope, "status + `code`, fallback `error`" rule, `Cache-Control` | `hd-api` `create`, `normalizeError` |
| GET `bootstrap` (`catalog_hash`, `desktop_readonly`, `sources`) | `hd-api.bootstrap` ← `hd-app` `boot`, `recheck`; stream `diagnose` (realm in `hd-app`) |
| GET `widget-catalog` + `ETag` / `If-None-Match` / 304 | `hd-api.widgetCatalog` ← `hd-store.widgetCatalog` ← `hd-app` `boot` |
| GET `history` → `{origin, points:[{t,v}]}` | `hd-api.history` ← `hd-store.history` (`convert`) ← `hd-render` api `history` |
| GET `stream` | `hd-api.openStream` ← `hd-stream` `connect` |
| PUT `stream/subscriptions` (accepted / rejected, 404 `stream_not_found`) | `hd-api.putSubscriptions` ← `hd-stream` `put`; rejected → `hd-store` `subscribed` handler |
| GET `dashboards` | `hd-api.dashboards` ← `hd-app` realm `refresh` |
| POST `dashboards` `{template:"system", name}` / `{document}` | `hd-api.createFromTemplate` / `createDocument` ← `hd-app` `create` |
| GET `dashboard?id=` (+ `schema_too_new`) | `hd-api.dashboard` ← `hd-app` `open` |
| DELETE `dashboard?id=&revision=` (409 `revision_conflict`) | `hd-api.deleteDashboard` ← `hd-app` `remove` |
| Frames `hello`, `hb`, `revoked` | `hd-stream` `hello`, watchdog, `revoked` handler |
| Frames `snapshot`, `entity`, `removed`, `state`, `source` | `hd-store` handlers (`setEntity`, `setSources`, `appendLive`) |
| Frame `dashboard_changed` | `hd-store` → `'dashboards'` → `hd-app` realm `refresh` → `reconcile` |
| Reconnect rules (45 s, backoff, ±20 %, 60 s reset, PUT 404, 429, revoked reasons) | `hd-stream` `failure`, `armWatchdog`, `revoked`, `diagnose` |
| Entity / State / SourceStatus shapes | `hd-store.view` |
| Codes acted on: `homedash_disabled`, `storage_unavailable`, `desktop_readonly`, `dashboard_not_found`, `limit_exceeded`, `revision_conflict`, `stream_not_found`, `unauthorized` | `hd-app` `fail`, `hd-stream` `put` |
| `system` entities (D7) and the starter template (D3) | `ui/testdata/homedash-m2/*.json`, Go fixture |

Not called by the M2 client: `entities`, `entity`, `snapshot`, `commands`, `dashboards/validate`, PUT `dashboard`, `duplicate`, `revisions`, `restore`, `approve` (the fixture still serves `entities` and `entity`).

---

## Task 0: Workspace

Rules every task follows (each dispatch quotes the ones it needs):
- **R1 Worktree.** The session runs in the worktree the desktop app created. Use the branch the app gave you; do not run `git worktree add`. Run every command in that worktree.
- **R2 GitNexus.** Before editing an EXISTING symbol run impact (MCP `impact({target:"<symbol>", direction:"upstream", repo:"AuraGo"})` or CLI `node C:/Users/Andi/.gitnexus/maintenance/AuraGo/graph-alternate/run.cjs impact "<symbol>" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo`); report callers and risk, warn on HIGH/CRITICAL, treat `UNKNOWN` as unresolved and confirm callers with Grep. Before each commit run detect-changes (MCP `detect_changes({scope:"all", repo:"AuraGo", worktree:"<worktree>"})` or CLI `detect-changes --scope all`). When GitNexus cannot answer, grep the callers and add `GitNexus unavailable; callers checked with grep: <symbols>` to the commit body.
- **R3 Commits.** One commit per task with only its paths: `git add <paths>` then `git commit -m "<message>"`. Never `git add -A`. New `AGENTS.md` files and anything under `docs/` need `git add -f`; confirm with `git ls-files <path>`.
- **R4 Files.** LF, no BOM. Every JS file under 1100 lines (aim under 700). No `innerHTML`, `outerHTML`, `insertAdjacentHTML`, `setAttribute('style'`, `.style =`, `cssText`; styles only through `style.setProperty` / `E.core.setVars`. Untrusted text only through `textContent`. Full literal i18n keys (`t('homedash.…')`). Non-ASCII characters in JS through `String.fromCodePoint`. In Go source never write a literal backslash-u escape (use `\x5c`). No external assets.
- **R5 Builds.** After editing `desktop-foundation.js`, `menus-and-routing.js` or `window-shell-runtime.js`, or any `ui/css/homedash-*.css`, run `npm run build:ui` and commit the generated bundle with the task.
- **R6 Go environment.** `GOTMPDIR` in backslash form on a short path (`C:\tmp\go-build`); on `no such tool "vet"` prefix `GOTOOLCHAIN=go1.27.4`; if D: is full, trim `D:\dev-cache\go-build` entries older than 3 days (1 day if that frees nothing). Repository-wide checks (`go build ./...`, `./internal/audit/...`) only in this worktree.
- **R7 Tone rule (D5, quoted in Tasks 2, 9 and 19).** Numeric: `options.warn` and `options.alert` are thresholds. Direction is "falling" only when both are numbers and `warn > alert`; otherwise "rising" (one threshold alone, or `warn <= alert`). Rising: value ≥ alert → `crit`, else value ≥ warn → `warn`. Falling: value ≤ alert → `crit`, else value ≤ warn → `warn`. Boundaries count as crossed; `min`/`max` never change the direction. Binary: a `binary_sensor` with device class `smoke`, `gas`, `moisture`, `problem`, `safety` or `carbon_monoxide` is `crit` while on. Unavailable or unknown entities, non-numeric values and values inside the thresholds have no tone (`null`). The host puts the tone on the entity view (`tone`); widgets paint it (they call `api.setTone(e.tone …)` themselves).

Steps:
- [ ] Confirm the branch: `git branch --show-current` (the app's branch) and `git merge-base --is-ancestor be4f4cf9e HEAD; echo $LASTEXITCODE` → `0` (M0 + M1 are in the base).
- [ ] Copy a real `node_modules` (a junction breaks `check:ui`): PowerShell `robocopy C:\Users\Andi\Documents\repo\AuraGo\node_modules <worktree>\node_modules /E /MT:16 /NFL /NDL /NJH /NP`.
- [ ] Baseline (record results in the session scratchpad, not the repo):
  - `npm run test:homedash` → `homedash: 89/89 passed` (or the current count, all PASS).
  - `npm run check:ui` → exit 0.
  - `go test ./ui -run '(?i)^(TestTranslations_|TestFrontend_StaticI18nKeysExistInEnglishBundle|TestDesktopAppMaterialsAdoptedByApps|TestDockerManagerWiring)' -count=1` → ok.
  - `go test ./ui -run '^TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget$' -count=1` → record the known failures (other apps' files over 1100 lines); HomeDash files must never appear in that list.
  - `go test ./internal/desktop -count=1` → ok; `go vet ./ui` → ok.
  - One GitNexus call (`impact` for `appGlobalName`) to learn whether MCP or CLI answers; note the outcome.
- [ ] Check that the backend branch has not landed yet: `git log --oneline -1 -- internal/server/homedash_routes.go` (empty output = not landed; matters for Task 20).

**Done when:** baseline numbers are recorded, `node_modules` is a real directory, and the GitNexus route (MCP, CLI or grep fallback) is known.

---

### Task 1: hd-api.js, the only file that knows routes

**Files:** create `ui/js/homedash/hd-api.js`, `scripts/homedash-tests/api.test.mjs`.
**Read first:** contract sections "General rules", "Routes", "Error codes"; `ui/js/desktop/core/desktop-foundation.js:1091-1120` (`api()` throws an Error with `status`, `body`, `retryAfter`); `scripts/homedash-tests/harness.mjs`.

- [ ] Write the failing test `scripts/homedash-tests/api.test.mjs`:

```js
import assert from 'node:assert/strict'; import { loadEngine, eq } from './harness.mjs';
const load = () => loadEngine(['hd-core.js', 'hd-api.js']).E;
function fakeTransport(answers) {
  const calls = [];
  return { calls, request(url, init) { calls.push({ url, init }); const a = answers.shift(); return a && a.reject ? Promise.reject(a.reject) : Promise.resolve(a); },
    eventSource(url) { return { url }; } };
}
const httpError = (status, body) => Object.assign(new Error('HTTP ' + status), { status, body });
export const tests = [
  ['hd-api builds every M2 route under the base path with encoded query values', async () => {
    const E = load(), tr = fakeTransport(Array.from({ length: 7 }, () => ({ status: 'ok' })));
    const api = E.api.create({ base: '/api/desktop/homedash/', transport: tr });
    await api.bootstrap(); await api.dashboards(); await api.dashboard('d_m3x7q2k5a4bc');
    await api.history('system:net_rx', { from: Date.UTC(2026, 10, 25, 9), to: Date.UTC(2026, 10, 25, 10), points: 60 });
    await api.createFromTemplate('system', 'System'); await api.createDocument({ name: 'Mein Dashboard', pages: [{ id: 'main', name: 'Hauptseite' }] });
    await api.putSubscriptions('9f2c', ['system:cpu']);
    assert.deepEqual(tr.calls.map(c => c.init.method + ' ' + c.url), [
      'GET /api/desktop/homedash/bootstrap', 'GET /api/desktop/homedash/dashboards', 'GET /api/desktop/homedash/dashboard?id=d_m3x7q2k5a4bc',
      'GET /api/desktop/homedash/history?id=system%3Anet_rx&from=2026-11-25T09%3A00%3A00.000Z&to=2026-11-25T10%3A00%3A00.000Z&points=60',
      'POST /api/desktop/homedash/dashboards', 'POST /api/desktop/homedash/dashboards', 'PUT /api/desktop/homedash/stream/subscriptions']);
    eq(JSON.parse(tr.calls[4].init.body), { template: 'system', name: 'System' });
    eq(JSON.parse(tr.calls[5].init.body), { document: { name: 'Mein Dashboard', pages: [{ id: 'main', name: 'Hauptseite' }] } });
    eq(JSON.parse(tr.calls[6].init.body), { stream_id: '9f2c', entities: ['system:cpu'] });
    assert.equal(tr.calls[4].init.headers['Content-Type'], 'application/json'); assert.equal(tr.calls[0].init.body, undefined);
    assert.equal(api.openStream().url, '/api/desktop/homedash/stream'); }],
  ['hd-api keys errors on status plus code, falling back to error and then to the transport', async () => {
    const E = load();
    const cases = [
      [httpError(409, { status: 'error', code: 'revision_conflict', message: 'stale', current_revision: 9 }), 409, 'revision_conflict'],
      [httpError(403, { error: 'csrf_check_failed', message: 'origin' }), 403, 'csrf_check_failed'],
      [httpError(403, { error: 'desktop_readonly', code: 'desktop_readonly', message: 'read-only' }), 403, 'desktop_readonly'],
      [httpError(401, { error: 'unauthorized', redirect: '/auth/login' }), 401, 'unauthorized'],
      [httpError(502, {}), 502, 'http_502'], [new TypeError('Failed to fetch'), 0, 'network'],
      [Object.assign(new Error('aborted'), { name: 'AbortError' }), 0, 'aborted']];
    for (const [err, status, code] of cases) {
      const api = E.api.create({ transport: fakeTransport([{ reject: err }]) });
      await assert.rejects(api.dashboards(), e => e.hdApiError === true && e.status === status && e.code === code, code);
    }
    const conflict = E.api.normalizeError(cases[0][0]); assert.equal(conflict.data.current_revision, 9); assert.equal(conflict.message, 'stale');
    assert.equal(E.api.normalizeError(httpError(429, { code: 'rate_limited', retry_after_ms: 1500 })).retryAfter, 1.5);
    assert.equal(E.api.normalizeError(conflict), conflict); }],
  ['hd-api treats 304 on the widget catalog as not modified and refuses malformed dashboard ids before any request', async () => {
    const E = load(), tr = fakeTransport([{ reject: httpError(304, {}) }]), api = E.api.create({ transport: tr });
    eq(await api.widgetCatalog('abc'), { status: 'ok', not_modified: true });
    assert.equal(tr.calls[0].init.headers['If-None-Match'], '"abc"');
    await assert.rejects(api.dashboard('../etc'), e => e.status === 400 && e.code === 'bad_request');
    await assert.rejects(api.deleteDashboard('d_ABC', 1), e => e.code === 'bad_request');
    assert.equal(tr.calls.length, 1); }]
];
```

- [ ] Run `npm run test:homedash` → expected `FAIL api.test.mjs: …` (ENOENT reading `ui/js/homedash/hd-api.js`) for all three cases.
- [ ] Implement `ui/js/homedash/hd-api.js`:

```js
// HomeDash API client (F1). The only engine file that knows HomeDash routes (contract "API contract (frozen for M2)" in
// docs/superpowers/plans/2026-11-25-homedash-m2-backend.md). create({base, transport}) wraps the host transport: transport.request(url, init)
// resolves the JSON body or throws an Error with status and body (desktop api(), desktop-foundation.js); transport.eventSource(url) opens SSE.
// Every failure becomes an Error with {hdApiError, status, code, message, data, retryAfter}: code is the body's `code`, else its `error`
// (shared auth, CSRF and desktop layers answer without `status`), else a transport word (aborted, not_modified, network, http_<status>).
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core) throw new Error('hd-api.js: load hd-core.js first');
    const DASHBOARD_ID = /^d_[a-z2-7]{12}$/;
    function normalizeError(err) {
        const e = err || {};
        if (e.hdApiError === true) return e;
        const body = e.body && typeof e.body === 'object' ? e.body : {}, status = Number.isInteger(e.status) ? e.status : 0;
        const code = (typeof body.code === 'string' && body.code) || (typeof body.error === 'string' && body.error)
            || (e.name === 'AbortError' ? 'aborted' : status === 304 ? 'not_modified' : status ? 'http_' + status : 'network');
        const out = new Error((typeof body.message === 'string' && body.message) || (e.message ? String(e.message) : code));
        out.hdApiError = true; out.status = status; out.code = code; out.data = body;
        out.retryAfter = Number.isFinite(e.retryAfter) ? e.retryAfter : Number.isFinite(body.retry_after_ms) ? body.retry_after_ms / 1000 : null;
        return out;
    }
    function query(params) {
        const parts = [];
        for (const [key, value] of Object.entries(params || {})) if (value !== undefined && value !== null && value !== '') parts.push(encodeURIComponent(key) + '=' + encodeURIComponent(String(value)));
        return parts.length ? '?' + parts.join('&') : '';
    }
    const iso = v => (typeof v === 'number' && Number.isFinite(v) ? new Date(v).toISOString() : v);
    function create(options) {
        const o = options || {}, transport = o.transport || {}, base = String(o.base || '/api/desktop/homedash').replace(/\/+$/, '');
        if (typeof transport.request !== 'function') throw new Error('hd-api: transport.request is required');
        const url = (path, params) => base + '/' + path + query(params);
        async function call(method, path, params, body, extra) {
            const x = extra || {}, init = { method, headers: Object.assign({}, x.headers) };
            if (body !== undefined) { init.headers['Content-Type'] = 'application/json'; init.body = JSON.stringify(body); }
            if (x.signal) init.signal = x.signal;
            try { return await transport.request(url(path, params), init); } catch (err) { throw normalizeError(err); }
        }
        const dashboardId = id => { if (!DASHBOARD_ID.test(String(id))) throw normalizeError({ status: 400, body: { code: 'bad_request', message: 'invalid dashboard id' } }); return String(id); };
        return Object.freeze({
            base, url,
            bootstrap: extra => call('GET', 'bootstrap', null, undefined, extra),
            async widgetCatalog(etag, extra) {
                const headers = etag ? { 'If-None-Match': '"' + etag + '"' } : {};
                try { return await call('GET', 'widget-catalog', null, undefined, Object.assign({}, extra, { headers })); }
                catch (err) { if (err.status === 304) return { status: 'ok', not_modified: true }; throw err; }
            },
            history: (id, range, extra) => call('GET', 'history', { id, from: iso(range && range.from), to: iso(range && range.to), points: range && range.points }, undefined, extra),
            dashboards: extra => call('GET', 'dashboards', null, undefined, extra),
            dashboard: async (id, extra) => call('GET', 'dashboard', { id: dashboardId(id) }, undefined, extra),
            createFromTemplate: (template, name, extra) => call('POST', 'dashboards', null, name ? { template, name } : { template }, extra),
            createDocument: (document, extra) => call('POST', 'dashboards', null, { document }, extra),
            deleteDashboard: async (id, revision, extra) => call('DELETE', 'dashboard', { id: dashboardId(id), revision }, undefined, extra),
            putSubscriptions: (streamId, entities, extra) => call('PUT', 'stream/subscriptions', null, { stream_id: streamId, entities: Array.from(entities || []) }, extra),
            openStream() { if (typeof transport.eventSource !== 'function') throw new Error('hd-api: transport.eventSource is required'); return transport.eventSource(url('stream')); }
        });
    }
    E.api = Object.freeze({ create, normalizeError, DASHBOARD_ID });
})();
```

- [ ] Run `npm run test:homedash` → all `api.test.mjs` cases PASS, total count +3, no FAIL.
- [ ] GitNexus: new file only; run detect-changes.
- [ ] Commit `ui/js/homedash/hd-api.js scripts/homedash-tests/api.test.mjs` — `feat(homedash): hd-api client for the M2 contract`.

**Done when:** three new PASS lines, guard test (`engine JS is clean`) still PASS.

---

### Task 2: hd-tone.js (D5) and host icons

**Files:** create `ui/js/homedash/hd-tone.js`, `scripts/homedash-tests/tone.test.mjs`; modify `ui/js/homedash/hd-icons.js` (`PATHS`).
**Read first:** R7 (Task 0); `ui/js/homedash/hd-format.js` (`isOn`); `ui/js/homedash/widgets/hd-w-sensor.js:41-66` (gauge zones and tone); `ui/js/homedash/hd-icons.js`.

- [ ] Write the failing test `scripts/homedash-tests/tone.test.mjs`:

```js
import assert from 'node:assert/strict'; import { loadEngine, eq } from './harness.mjs';
const T = () => loadEngine(['hd-core.js', 'hd-format.js', 'hd-tone.js']).E.tone;
const view = (value, extra) => Object.assign({ id: 'system:cpu', kind: 'sensor', value, available: true, status: 'ok' }, extra);
export const tests = [
  ['tone: rising thresholds give warn and crit, boundaries included', () => { const t = T(), o = { warn: 80, alert: 95 };
    assert.equal(t.of(view(79.9), o), null); assert.equal(t.of(view(80), o), 'warn'); assert.equal(t.of(view(94.9), o), 'warn'); assert.equal(t.of(view(95), o), 'crit');
    assert.equal(t.of(view('96.5'), o), 'crit'); assert.equal(t.of(view(99), { alert: 95 }), 'crit'); assert.equal(t.of(view(90), { alert: 95 }), null);
    assert.equal(t.of(view(81), { warn: 80 }), 'warn'); assert.equal(t.of(view(90), { warn: 90, alert: 90 }), 'crit'); assert.equal(t.direction({ warn: 90, alert: 90 }), 'rising'); }],
  ['tone: warn above alert means falling (low battery)', () => { const t = T(), o = { warn: 30, alert: 15 };
    assert.equal(t.direction(o), 'falling'); assert.equal(t.of(view(31), o), null); assert.equal(t.of(view(30), o), 'warn'); assert.equal(t.of(view(15), o), 'crit');
    assert.equal(t.of(view(2), o), 'crit'); assert.equal(t.of(view(5), { warn: 30, alert: 15, min: 100, max: 0 }), 'crit'); }],
  ['tone: no tone without thresholds, for text, unavailable and unknown entities', () => { const t = T(), o = { warn: 80, alert: 95 };
    assert.equal(t.of(view(99), {}), null); assert.equal(t.of(view('on'), o), null); assert.equal(t.of(view(null), o), null);
    assert.equal(t.of(view(99, { available: false }), o), null); assert.equal(t.of(view(99, { status: 'unknown' }), o), null); assert.equal(t.of(null, o), null); }],
  ['tone: safety binary sensors are crit while on, other classes never', () => { const t = T();
    for (const dc of ['smoke', 'gas', 'moisture', 'problem', 'safety', 'carbon_monoxide']) {
      assert.equal(t.of(view('on', { kind: 'binary_sensor', device_class: dc })), 'crit', dc); assert.equal(t.of(view(true, { kind: 'binary_sensor', device_class: dc })), 'crit', dc);
      assert.equal(t.of(view('off', { kind: 'binary_sensor', device_class: dc })), null, dc); }
    assert.equal(t.of(view('on', { kind: 'binary_sensor', device_class: 'door' })), null);
    assert.equal(t.of(view('on', { kind: 'binary_sensor', device_class: 'smoke', available: false })), null); }],
  ['tone: gauge zones follow the thresholds inside the range', () => { const t = T();
    eq(t.zones({ min: 0, max: 100, warn: 80, alert: 95 }), [{ from: 80, to: 95, tone: 'warn' }, { from: 95, to: 100, tone: 'crit' }]);
    eq(t.zones({ min: 0, max: 100, warn: 30, alert: 15 }), [{ from: 0, to: 15, tone: 'crit' }, { from: 15, to: 30, tone: 'warn' }]);
    eq(t.zones({ min: 0, max: 100, warn: 80 }), [{ from: 80, to: 100, tone: 'warn' }]); eq(t.zones({ min: 0, max: 50, alert: 95 }), []); eq(t.zones({}), []); }],
  ['icons: the M2 host glyphs exist', () => { const E = loadEngine(['hd-core.js', 'hd-icons.js'], { dom: true }).E;
    for (const name of ['arrow-left', 'trash', 'dashboard', 'signal-off', 'info', 'refresh']) { assert.ok(E.icons.has(name), name); assert.ok(E.icons.icon(name).querySelectorAll('path').length > 0, name); } }]
];
```

- [ ] Run `npm run test:homedash` → expected FAIL for the five tone cases (missing file) and the icons case (`arrow-left`).
- [ ] Implement `ui/js/homedash/hd-tone.js`:

```js
// HomeDash alert tone (decision D5; pure). The host computes a widget's tone and hands it to the widget as the entity view's `tone`;
// widgets only paint it. Rule (also in ui/js/homedash/AGENTS.md): warn and alert are thresholds; the direction is "falling" only when both
// are numbers and warn > alert, otherwise "rising". Rising: value >= alert is crit, else value >= warn is warn. Falling: value <= alert is
// crit, else value <= warn is warn. Boundaries count as crossed; min/max never change the direction. A binary_sensor of class smoke, gas,
// moisture, problem, safety or carbon_monoxide is crit while on. No tone (null) for unavailable or unknown entities and non-numeric values.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.format) throw new Error('hd-tone.js: load hd-core.js and hd-format.js first');
    const SAFETY = Object.freeze(['smoke', 'gas', 'moisture', 'problem', 'safety', 'carbon_monoxide']);
    function num(v) {
        if (typeof v === 'number') return Number.isFinite(v) ? v : null;
        if (typeof v === 'string' && v.trim() !== '') { const n = Number(v); return Number.isFinite(n) ? n : null; }
        return null;
    }
    function direction(options) { const o = options || {}, w = num(o.warn), a = num(o.alert); return w !== null && a !== null && w > a ? 'falling' : 'rising'; }
    function numeric(value, options) {
        const o = options || {}, v = num(value), w = num(o.warn), a = num(o.alert);
        if (v === null || (w === null && a === null)) return null;
        if (direction(o) === 'falling') return a !== null && v <= a ? 'crit' : w !== null && v <= w ? 'warn' : null;
        return a !== null && v >= a ? 'crit' : w !== null && v >= w ? 'warn' : null;
    }
    function of(view, options) {
        if (!view || view.available === false || view.status === 'unavailable' || view.status === 'unknown') return null;
        if (view.kind === 'binary_sensor') return SAFETY.includes(view.device_class) && E.format.isOn(view) === true ? 'crit' : null;
        return numeric(view.value, options);
    }
    // Gauge zones for the same thresholds: ascending, clamped to [min, max], empty zones dropped.
    function zones(options) {
        const o = options || {}, w = num(o.warn), a = num(o.alert), lo0 = num(o.min) ?? 0, hi0 = num(o.max) ?? 100;
        const lo = Math.min(lo0, hi0), hi = Math.max(lo0, hi0), c = v => Math.min(hi, Math.max(lo, v)), out = [];
        if (direction(o) === 'falling') out.push({ from: lo, to: c(a), tone: 'crit' }, { from: c(a), to: c(w), tone: 'warn' });
        else { if (w !== null) out.push({ from: c(w), to: a !== null ? c(a) : hi, tone: 'warn' }); if (a !== null) out.push({ from: c(a), to: hi, tone: 'crit' }); }
        return out.filter(z => z.to > z.from);
    }
    E.tone = Object.freeze({ of, numeric, zones, direction, SAFETY });
})();
```

- [ ] GitNexus impact for `PATHS` / `icon` in `hd-icons.js` (expect callers in primitives and widgets; additive change, LOW).
- [ ] In `ui/js/homedash/hd-icons.js` add these entries to `PATHS` (24 px grid, stroked by `.hd-ico`), after `'trend-flat'`:

```js
        'arrow-left': ['M19 12H5', 'M11 18l-6-6 6-6'], trash: ['M4 7h16', 'M9 7V4h6v3', 'M6 7l1 13h10l1-13', 'M10 11v6', 'M14 11v6'],
        dashboard: ['M4 4h7v9H4z', 'M13 4h7v5h-7z', 'M13 11h7v9h-7z', 'M4 15h7v5H4z'],
        'signal-off': ['M3 3l18 18', 'M8.5 16.4a5 5 0 0 1 7 0', 'M5.2 12.9a10 10 0 0 1 4-2.3', 'M14.6 10.5a10 10 0 0 1 4.2 2.4', 'M2 8.8a15 15 0 0 1 4.3-2.7', 'M10.4 5.1A15 15 0 0 1 22 8.8', 'M12 20h.01'],
        info: ['M12 3a9 9 0 1 0 0 18a9 9 0 1 0 0-18z', 'M12 11v5', 'M12 8h.01'], refresh: ['M20 11a8 8 0 1 0-2.3 5.7', 'M20 4v7h-7']
```

- [ ] Run `npm run test:homedash` → six new PASS, no FAIL (the M1 token freeze is untouched).
- [ ] Detect-changes; commit `ui/js/homedash/hd-tone.js ui/js/homedash/hd-icons.js scripts/homedash-tests/tone.test.mjs` — `feat(homedash): host alert tone helper and app glyphs`.

**Done when:** six PASS lines; the rule text in the file header equals R7.

---

### Task 3: hd-stream.js, one SSE stream per realm

**Files:** create `ui/js/homedash/hd-stream.js`, `scripts/homedash-tests/live-fakes.mjs`, `scripts/homedash-tests/stream.test.mjs`.
**Read first:** contract "Stream protocol" and "Reconnect rules"; `ui/js/desktop/apps/docker-manager-live.js` (pattern: generation counter, single PUT in flight, close on error).

- [ ] Write `scripts/homedash-tests/live-fakes.mjs` (test support, not a test):

```js
// Fakes for the HomeDash live-data tests: a manual clock with timers, and EventSource objects the test drives.
export function fakeClock(start = Date.UTC(2026, 10, 25, 10)) {
  let now = start, seq = 0; const timers = new Map();
  const flush = async () => { for (let i = 0; i < 25; i++) await Promise.resolve(); };
  return {
    now: () => now, flush,
    setTimeout(fn, ms) { const id = ++seq; timers.set(id, { at: now + Math.max(0, Number(ms) || 0), fn }); return id; },
    clearTimeout(id) { timers.delete(id); },
    delays: () => [...timers.values()].map(x => x.at - now).sort((a, b) => a - b),
    async advance(ms) {
      const end = now + ms; await flush();
      for (;;) {
        let pick = null;
        for (const [id, x] of timers) if (x.at <= end && (!pick || x.at < pick[1].at || (x.at === pick[1].at && id < pick[0]))) pick = [id, x];
        if (!pick) break;
        timers.delete(pick[0]); now = pick[1].at; pick[1].fn(); await flush();
      }
      now = end; await flush();
    } };
}
export function fakeSources() {
  const list = [];
  class FakeEventSource {
    constructor(url) { this.url = url; this.closed = false; this.listeners = new Map(); list.push(this); }
    addEventListener(type, fn) { if (!this.listeners.has(type)) this.listeners.set(type, []); this.listeners.get(type).push(fn); }
    emit(type, data) { for (const fn of this.listeners.get(type) || []) fn({ type, data: typeof data === 'string' ? data : JSON.stringify(data) }); }
    fail() { for (const fn of this.listeners.get('error') || []) fn({ type: 'error' }); }
    close() { this.closed = true; }
  }
  return { list, open: url => new FakeEventSource(url), last: () => list[list.length - 1] };
}
export function fakeStreamApi(sources) {
  const puts = []; let answer = ids => ({ status: 'ok', accepted: ids, rejected: [] });
  return { base: '/api/desktop/homedash', puts, openStream: () => sources.open('/api/desktop/homedash/stream'),
    putSubscriptions(streamId, ids) { const list = Array.from(ids); puts.push({ streamId, ids: list }); const a = answer(list); return a instanceof Error ? Promise.reject(a) : Promise.resolve(a); },
    setAnswer(fn) { answer = fn; } };
}
export const HELLO = { stream_id: 's1', protocol: 1, heartbeat_ms: 15000, limits: { entities: 300 }, server_time: '2026-11-25T10:00:00Z', principal: 'session' };
```

- [ ] Write the failing test `scripts/homedash-tests/stream.test.mjs`:

```js
import assert from 'node:assert/strict'; import { loadEngine } from './harness.mjs'; import { fakeClock, fakeSources, fakeStreamApi, HELLO } from './live-fakes.mjs';
function setup(extra) {
  const { E } = loadEngine(['hd-core.js', 'hd-api.js', 'hd-stream.js']), clock = fakeClock(), es = fakeSources(), api = fakeStreamApi(es), random = { v: 0.5 };
  const s = E.stream.create(Object.assign({ api, setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now, random: () => random.v }, extra));
  const phases = []; s.on('connection', st => phases.push(st.phase + (st.reason ? ':' + st.reason : '')));
  return { E, s, clock, es, api, random, phases };
}
const gone = () => Object.assign(new Error('gone'), { hdApiError: true, status: 404, code: 'stream_not_found', data: {} });
export const tests = [
  ['stream: hello, then PUT of the active owners union; frames pass through', async () => {
    const { s, clock, es, api, phases } = setup(), seen = [];
    for (const type of ['snapshot', 'state', 'entity', 'removed', 'source', 'dashboard_changed']) s.on(type, d => seen.push(type + ':' + JSON.stringify(d)));
    s.attach('win-a', { entities: ['system:cpu', 'system:memory', 'system:cpu', 'bad'], active: true });
    assert.equal(es.list.length, 1); assert.equal(es.last().url, '/api/desktop/homedash/stream'); assert.deepEqual(phases, ['connecting']);
    es.last().emit('hello', HELLO); await clock.flush();
    assert.deepEqual(api.puts, [{ streamId: 's1', ids: ['system:cpu', 'system:memory'] }]); assert.deepEqual(phases, ['connecting', 'live']);
    es.last().emit('snapshot', { entities: [{ id: 'system:cpu' }], sources: [] }); es.last().emit('state', { changes: { 'system:cpu': { value: 1 } } });
    assert.deepEqual(seen, ['snapshot:{"entities":[{"id":"system:cpu"}],"sources":[]}', 'state:{"changes":{"system:cpu":{"value":1}}}']); s.dispose(); }],
  ['stream: the union is sent 250 ms after the last change; inactive owners drop out; hello limits cut it', async () => {
    const { s, clock, es, api } = setup();
    s.attach('a', { entities: ['system:cpu'], active: true }); es.last().emit('hello', HELLO); await clock.flush();
    const b = s.attach('b', { entities: ['system:disk'], active: true }); await clock.advance(249); assert.equal(api.puts.length, 1);
    b.set(['system:disk', 'system:uptime']); await clock.advance(249); assert.equal(api.puts.length, 1); await clock.advance(1);
    assert.deepEqual(api.puts[1].ids, ['system:cpu', 'system:disk', 'system:uptime']);
    b.setActive(false); await clock.advance(250); assert.deepEqual(api.puts[2].ids, ['system:cpu']);
    b.setActive(true); es.last().fail(); await clock.advance(1000); es.last().emit('hello', Object.assign({}, HELLO, { stream_id: 's2', limits: { entities: 2 } })); await clock.flush();
    assert.deepEqual(api.puts[3], { streamId: 's2', ids: ['system:cpu', 'system:disk'] }); s.dispose(); }],
  ['stream: errors close the source and back off; failures before hello wait 5 s; 60 s of life resets the backoff', async () => {
    const { s, clock, es, random } = setup();
    s.attach('a', { entities: ['system:cpu'], active: true }); es.last().emit('hello', HELLO); await clock.flush();
    const first = es.last(); first.fail(); assert.equal(first.closed, true); assert.equal(s.status().phase, 'down'); assert.deepEqual(clock.delays(), [1000]);
    await clock.advance(1000); assert.equal(es.list.length, 2);
    es.last().fail(); assert.deepEqual(clock.delays(), [5000]);
    await clock.advance(5000); assert.equal(es.list.length, 3); es.last().emit('hello', HELLO); await clock.flush();
    for (let i = 0; i < 4; i++) { await clock.advance(15000); es.last().emit('hb', { server_time: 'x' }); }
    random.v = 0; es.last().fail(); assert.deepEqual(clock.delays(), [800]);
    random.v = 0.999999; await clock.advance(800); es.last().emit('hello', HELLO); await clock.flush(); es.last().fail(); assert.deepEqual(clock.delays(), [2400]); s.dispose(); }],
  ['stream: 45 s without any frame is a dead stream; hb keeps it alive', async () => {
    const { s, clock, es } = setup();
    s.attach('a', { entities: ['system:cpu'], active: true }); es.last().emit('hello', HELLO); await clock.flush();
    await clock.advance(44000); es.last().emit('hb', {}); await clock.advance(44000); assert.equal(es.list[0].closed, false);
    await clock.advance(1000); assert.equal(es.list[0].closed, true); assert.equal(s.status().reason, 'watchdog'); s.dispose(); }],
  ['stream: PUT 404 reconnects; revoked shutdown reconnects; disabled and auth_lost stop until resume', async () => {
    const { s, clock, es, api, phases } = setup();
    api.setAnswer(gone); s.attach('a', { entities: ['system:cpu'], active: true }); es.last().emit('hello', HELLO); await clock.flush();
    assert.equal(es.list[0].closed, true); assert.equal(s.status().reason, 'stream_not_found');
    api.setAnswer(ids => ({ status: 'ok', accepted: ids, rejected: [] })); await clock.advance(1000); es.last().emit('hello', HELLO); await clock.flush();
    es.last().emit('revoked', { reason: 'shutdown' }); assert.equal(s.status().phase, 'down');
    await clock.advance(5000); es.last().emit('hello', HELLO); await clock.flush(); es.last().emit('revoked', { reason: 'disabled' });
    assert.equal(s.status().phase, 'stopped'); assert.equal(s.status().reason, 'disabled'); const count = es.list.length; await clock.advance(60000); assert.equal(es.list.length, count);
    s.resume(); assert.equal(es.list.length, count + 1); es.last().emit('hello', HELLO); await clock.flush(); es.last().emit('revoked', { reason: 'auth_lost' });
    assert.ok(phases.includes('stopped:auth_lost')); s.dispose(); }],
  ['stream: a refusal before hello asks diagnose; a disabled answer stops', async () => {
    let answer = 'ok', asked = 0;
    const { s, clock, es } = setup({ diagnose: () => { asked += 1; return Promise.resolve(answer); } });
    s.attach('a', { entities: ['system:cpu'], active: true }); es.last().fail(); await clock.flush();
    assert.equal(asked, 1); assert.deepEqual(clock.delays(), [5000]);
    answer = 'disabled'; await clock.advance(5000); es.last().fail(); await clock.flush();
    assert.equal(asked, 2); assert.equal(s.status().phase, 'stopped'); assert.equal(s.status().reason, 'disabled'); assert.deepEqual(clock.delays(), []); s.dispose(); }],
  ['stream: no active owner for 10 s closes the stream; an active owner reconnects; shared() is one per base', async () => {
    const { E, s, clock, es, api } = setup();
    const a = s.attach('a', { entities: ['system:cpu'], active: true }); es.last().emit('hello', HELLO); await clock.flush();
    a.setActive(false); await clock.advance(250); assert.deepEqual(api.puts[1].ids, []);
    await clock.advance(9749); assert.equal(es.list[0].closed, false); await clock.advance(1); assert.equal(es.list[0].closed, true); assert.equal(s.status().phase, 'idle');
    a.setActive(true); assert.equal(es.list.length, 2); s.dispose();
    const x = E.stream.shared({ api }), y = E.stream.shared({ api }); assert.equal(x, y); x.dispose(); assert.notEqual(E.stream.shared({ api }), x); }]
];
```

- [ ] Run `npm run test:homedash` → the seven stream cases FAIL (missing file).
- [ ] Implement `ui/js/homedash/hd-stream.js`:

```js
// HomeDash stream client (F1; contract "Stream protocol"). One SSE connection per JS realm (shared(), keyed by the API base path), used by
// every HomeDash window. Owners attach with the entity ids they show and an active flag (the host's visibility signal). The union of the
// active owners' ids, cut to hello.limits.entities, goes out with PUT stream/subscriptions 250 ms after the last change and right after every
// hello (a new stream starts empty). Silence for max(45 s, 3 x heartbeat_ms) or an EventSource error closes the source (never the browser's
// own retry: the stream id changes) and reconnects after 1, 2, 4, 8, 16, 30 s with +-20 % jitter. A connection that fails before hello waits
// at least 5 s (429 too_many_streams cannot be told apart from other refusals) and asks diagnose(). The backoff resets after a stream lived
// 60 s. revoked shutdown reconnects; disabled and auth_lost stop until resume(). No active owner for 10 s closes the stream. Timers, clock,
// random and the api are injected so scripts/homedash-tests/stream.test.mjs drives everything.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.api) throw new Error('hd-stream.js: load hd-core.js and hd-api.js first');
    const BACKOFF_MS = Object.freeze([1000, 2000, 4000, 8000, 16000, 30000]);
    const JITTER = 0.2, DEBOUNCE_MS = 250, WATCH_MIN_MS = 45000, WATCH_FACTOR = 3, RESET_AFTER_MS = 60000, OPEN_FAIL_MIN_MS = 5000;
    const IDLE_CLOSE_MS = 10000, PUT_RETRY_MS = 3000, PUT_RETRIES = 3, DEFAULT_LIMIT = 300;
    const PASS = Object.freeze(['snapshot', 'entity', 'removed', 'state', 'source', 'dashboard_changed']);
    const parse = data => { try { const v = JSON.parse(data); return v && typeof v === 'object' && !Array.isArray(v) ? v : null; } catch (err) { return null; } };
    const idList = list => [...new Set((list == null ? [] : Array.from(list)).map(String).filter(id => id.indexOf(':') > 0))];

    function create(options) {
        const o = options || {}, api = o.api;
        if (!api || typeof api.openStream !== 'function' || typeof api.putSubscriptions !== 'function') throw new Error('hd-stream: an api with openStream and putSubscriptions is required');
        const setT = o.setTimeout || ((fn, ms) => setTimeout(fn, ms)), clearT = o.clearTimeout || (id => clearTimeout(id));
        const now = o.now || (() => Date.now()), random = o.random || Math.random, bus = E.core.emitter(), owners = new Map();
        let source = null, streamId = '', phase = 'idle', reason = '', phaseAt = now(), generation = 0, failures = 0, helloSeen = false, helloAt = 0;
        let watchMs = WATCH_MIN_MS, limit = DEFAULT_LIMIT, retryTimer = 0, putTimer = 0, watchTimer = 0, idleTimer = 0, nextRetryAt = 0;
        let wantIds = [], wantSig = '', sentSig = '', sending = false, dirty = false, putRetries = 0, stopped = false, disposed = false;

        const status = () => Object.freeze({ phase, reason, streamId, attempt: failures, nextRetryAt, since: phaseAt });
        function setPhase(next, why) { const r = why || ''; if (phase === next && reason === r) return; phase = next; reason = r; phaseAt = now(); bus.emit('connection', status()); }
        const anyActive = () => { for (const ow of owners.values()) if (ow.active) return true; return false; };
        function union() {
            const seen = new Set(), out = [];
            for (const ow of owners.values()) if (ow.active) for (const id of ow.ids) if (!seen.has(id)) { seen.add(id); out.push(id); }
            return out.slice(0, limit);
        }
        const signature = list => list.slice().sort().join('\n');

        function refreshWant() {
            if (disposed) return;
            const next = union(), sig = signature(next);
            if (sig !== wantSig) { wantIds = next; wantSig = sig; putRetries = 0; if (phase === 'live') schedulePut(DEBOUNCE_MS); }
            if (stopped) return;
            if (anyActive()) { if (idleTimer) { clearT(idleTimer); idleTimer = 0; } if (phase === 'idle') connect(); }
            else if (!idleTimer && phase !== 'idle') idleTimer = setT(() => { idleTimer = 0; if (!anyActive()) { teardown(); failures = 0; setPhase('idle', 'inactive'); } }, IDLE_CLOSE_MS);
        }
        function schedulePut(delay) { clearT(putTimer); putTimer = setT(() => { putTimer = 0; void put(); }, delay); }
        async function put() {
            if (phase !== 'live' || !streamId || wantSig === sentSig) return;
            if (sending) { dirty = true; return; }
            sending = true; dirty = false;
            const mine = generation, sig = wantSig, list = wantIds.slice();
            try {
                const answer = await api.putSubscriptions(streamId, list);
                if (mine !== generation) return;
                sentSig = sig; putRetries = 0;
                bus.emit('subscribed', { accepted: Array.isArray(answer && answer.accepted) ? answer.accepted : [], rejected: Array.isArray(answer && answer.rejected) ? answer.rejected : [] });
            } catch (err) {
                if (mine !== generation) return;
                const e = E.api.normalizeError(err);
                if (e.code === 'stream_not_found') { failure('stream_not_found'); return; }
                if (e.code === 'homedash_disabled') { stop('disabled'); return; }
                if (e.status === 401) { stop('auth_lost'); return; }
                if (e.code !== 'aborted' && putRetries < PUT_RETRIES) { putRetries += 1; dirty = true; }
            } finally {
                if (mine === generation) { sending = false; if (dirty && phase === 'live') schedulePut(putRetries ? PUT_RETRY_MS : 0); }
            }
        }
        function armWatchdog() { clearT(watchTimer); watchTimer = setT(() => { watchTimer = 0; failure('watchdog'); }, watchMs); }
        function teardown() {
            generation += 1;
            if (source) { const old = source; source = null; try { old.close(); } catch (err) { /* already closed */ } }
            clearT(putTimer); clearT(watchTimer); clearT(retryTimer); putTimer = watchTimer = retryTimer = 0;
            streamId = ''; sentSig = ''; sending = false; dirty = false; helloSeen = false; nextRetryAt = 0;
        }
        function connect() {
            if (source || stopped || disposed) return;
            clearT(retryTimer); retryTimer = 0; nextRetryAt = 0;
            generation += 1; const mine = generation;
            let next;
            try { next = api.openStream(); } catch (err) { failure('open'); return; }
            source = next; streamId = ''; sentSig = ''; helloSeen = false;
            const live = () => source === next && mine === generation;
            const on = (type, fn) => next.addEventListener(type, event => { if (!live()) return; armWatchdog(); const data = parse(event && event.data); if (data) fn(data); });
            on('hello', hello);
            for (const type of PASS) on(type, data => bus.emit(type, data));
            on('hb', () => {});
            on('revoked', data => { const why = String(data.reason || ''); if (why === 'disabled' || why === 'auth_lost') stop(why); else failure('shutdown'); });
            next.addEventListener('error', () => { if (live()) failure(helloSeen ? 'error' : 'open'); });
            setPhase('connecting');
            armWatchdog();
        }
        function hello(data) {
            const id = typeof data.stream_id === 'string' ? data.stream_id : '';
            if (!id) { failure('protocol'); return; }
            const hb = Number(data.heartbeat_ms), lim = Number(data.limits && data.limits.entities);
            streamId = id; watchMs = Math.max(WATCH_MIN_MS, WATCH_FACTOR * (hb > 0 ? hb : 0)); limit = Number.isInteger(lim) && lim > 0 ? lim : DEFAULT_LIMIT;
            helloSeen = true; helloAt = now(); sentSig = ''; sending = false; dirty = false; putRetries = 0;
            armWatchdog();
            wantIds = union(); wantSig = signature(wantIds);
            setPhase('live');
            bus.emit('hello', { streamId, limits: { entities: limit } });
            if (wantIds.length) void put(); else sentSig = wantSig;
        }
        function failure(why) {
            const lived = helloSeen && now() - helloAt >= RESET_AFTER_MS, beforeHello = !helloSeen;
            teardown();
            if (stopped || disposed) return;
            if (!anyActive()) { failures = 0; setPhase('idle', why); return; }
            if (lived) failures = 0;
            let delay = Math.round(BACKOFF_MS[Math.min(failures, BACKOFF_MS.length - 1)] * (1 - JITTER + 2 * JITTER * random()));
            if (beforeHello) delay = Math.max(delay, OPEN_FAIL_MIN_MS);
            failures += 1; nextRetryAt = now() + delay;
            retryTimer = setT(() => { retryTimer = 0; nextRetryAt = 0; if (anyActive()) connect(); else setPhase('idle', 'inactive'); }, delay);
            setPhase('down', why);
            if (beforeHello && typeof o.diagnose === 'function') Promise.resolve().then(() => o.diagnose())
                .then(result => { if (!disposed && !stopped && (result === 'disabled' || result === 'auth_lost')) stop(result); }, () => {});
        }
        function stop(why) { teardown(); if (idleTimer) { clearT(idleTimer); idleTimer = 0; } stopped = true; failures = 0; setPhase('stopped', why); }
        function resume() { if (disposed || !stopped) return; stopped = false; failures = 0; setPhase('idle', 'resume'); refreshWant(); }
        function retryNow() { if (stopped) { resume(); return; } if (phase === 'down' && retryTimer) { clearT(retryTimer); retryTimer = 0; nextRetryAt = 0; connect(); } }
        function dispose() { if (disposed) return; teardown(); if (idleTimer) clearT(idleTimer); idleTimer = 0; disposed = true; owners.clear(); setPhase('disposed'); }
        function attach(ownerId, init) {
            const key = String(ownerId || E.core.uid('hd-owner')), i = init || {};
            owners.set(key, { ids: idList(i.entities), active: i.active !== false });
            refreshWant();
            return Object.freeze({
                id: key,
                set(list) { const ow = owners.get(key); if (!ow) return; ow.ids = idList(list); refreshWant(); },
                setActive(active) { const ow = owners.get(key); if (!ow || ow.active === !!active) return; ow.active = !!active; refreshWant(); },
                detach() { if (owners.delete(key)) refreshWant(); }
            });
        }
        return Object.freeze({ attach, status, resume, retryNow, dispose, on: bus.on, off: bus.off, wanted: () => wantIds.slice(), limit: () => limit });
    }
    const instances = new Map();
    function shared(options) {
        const key = options && options.api ? options.api.base : '', have = instances.get(key);
        if (have && have.status().phase !== 'disposed') return have;
        const made = create(options); instances.set(key, made); return made;
    }
    E.stream = Object.freeze({ create, shared, BACKOFF_MS, DEBOUNCE_MS, WATCH_MIN_MS, OPEN_FAIL_MIN_MS, RESET_AFTER_MS, IDLE_CLOSE_MS });
})();
```

- [ ] Run `npm run test:homedash` → seven stream cases PASS, no FAIL.
- [ ] Detect-changes; commit `ui/js/homedash/hd-stream.js scripts/homedash-tests/live-fakes.mjs scripts/homedash-tests/stream.test.mjs` — `feat(homedash): realm-shared SSE stream client`.

**Done when:** all reconnect rules of the contract have a passing case (watchdog, backoff with jitter, 5 s pre-hello floor, 60 s reset, PUT 404, revoked reasons, idle close).

---

### Task 4: hd-store.js (states, staleness, linger, history, catalog cache)

**Files:** create `ui/js/homedash/hd-store.js`, `scripts/homedash-tests/store.test.mjs`.
**Read first:** contract "Shared JSON types" (Entity, State, SourceStatus) and the `history` example; `ui/js/homedash/hd-widgets.js` (host reads `available`, `status`, `pending`, `stale`, `readonly`, `tone`, `name`; `heroStatus.staleText` reads `attributes.updated_at`; `history()` resolves `{points: [[t_ms, value|null]], source}`); `ui/testdata/homedash-lab/hero-fixtures.js:28-31` (view shape used by the lab).

Store rules (header comment of the file says the same):
1. A snapshot is the truth for the ids it lists (contract: added ids only); nothing else turns unavailable.
2. `stale` = server `stale:true`, or the stream was down (connecting/down) for 5 s or stopped (set on every entry, cleared per id by its next snapshot or state), or the entity's source state is `degraded`/`unavailable`. Only available entities are stale.
3. `readonly` only for an entity with a command capability (any name except `stream`, `snapshot`, `history`, `numeric`) that the server marks read-only or that sits on a read-only desktop.
4. `removed` ids and PUT-rejected ids get status `unknown`.
5. Linger: ids a window stops showing stay subscribed 30 s.
6. History: cache `id|span|points`, TTL 60 s for spans under 6 h, else 5 min; LRU 100; 4 requests at a time; live numeric states append to every cached series of the id, trimmed to the span and 1000 points.

- [ ] Write the failing test `scripts/homedash-tests/store.test.mjs`:

```js
import assert from 'node:assert/strict'; import { loadEngine, eq } from './harness.mjs'; import { fakeClock } from './live-fakes.mjs';
const CPU = { id: 'system:cpu', source: 'system', kind: 'sensor', name: 'CPU', unit: '%', icon_hint: 'cpu', capabilities: [{ name: 'numeric', allowed: true }, { name: 'history', allowed: true }],
  readonly: true, state: { value: 12.3, available: true, updated_at: '2026-11-25T10:00:00Z' } };
const UPTIME = Object.assign({}, CPU, { id: 'system:uptime', name: 'Uptime', unit: 's', device_class: 'duration', state: { value: 7215, available: true, updated_at: '2026-11-25T10:00:00Z' } });
function setup(apiExtra) {
  const { E } = loadEngine(['hd-core.js', 'hd-api.js', 'hd-store.js']), clock = fakeClock(), frames = [], bus = E.core.emitter(), handles = [];
  const stream = { on: bus.on, off: bus.off, status: () => ({ phase: 'idle', reason: '' }),
    attach(owner, init) { const h = { id: owner, sets: [], active: init.active, detached: false, set(ids) { h.sets.push(Array.from(ids)); }, setActive(a) { h.active = a; }, detach() { h.detached = true; } }; handles.push(h); return h; } };
  const api = Object.assign({ base: '/api/desktop/homedash', history: () => Promise.resolve({ origin: 'source', points: [] }), widgetCatalog: () => Promise.resolve({ status: 'ok', catalog: { hash: 'h1' } }) }, apiExtra);
  const store = E.store.create({ api, stream, raf: fn => { frames.push(fn); return frames.length; }, caf: () => {}, setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now });
  const changes = []; store.on('change', ids => changes.push(ids === null ? null : [...ids].sort()));
  return { E, store, bus, handles, clock, api, changes, run: () => { while (frames.length) frames.shift()(); } };
}
export const tests = [
  ['store: a snapshot of watched ids gives widget views; unwatched ids stay out', () => {
    const { store, bus, handles, changes, run } = setup();
    store.watch('win', { ids: ['system:cpu', 'system:uptime'] }); eq(handles[0].sets[0], ['system:cpu', 'system:uptime']);
    bus.emit('snapshot', { entities: [CPU, UPTIME, Object.assign({}, CPU, { id: 'system:memory' })], sources: [{ source: 'system', state: 'polling', at: 'x' }] }); run();
    eq(changes, [['system:cpu', 'system:uptime']]);
    const v = store.view('system:cpu');
    assert.equal(v.status, 'ok'); assert.equal(v.value, 12.3); assert.equal(v.unit, '%'); assert.equal(v.icon_hint, 'cpu'); assert.equal(v.readonly, false); assert.equal(v.stale, false);
    assert.equal(v.attributes.updated_at, '2026-11-25T10:00:00Z'); assert.equal(v.tone, null); assert.equal(store.view('system:memory'), null); assert.equal(store.view('system:cpu'), v); }],
  ['store: state frames coalesce into one notice; readonly needs a command capability', () => {
    const { store, bus, changes, run } = setup(), w = store.watch('win', { ids: ['system:cpu', 'system:uptime'] });
    bus.emit('snapshot', { entities: [CPU, UPTIME], sources: [] }); run();
    bus.emit('state', { changes: { 'system:cpu': { value: 50, available: true, updated_at: '2026-11-25T10:00:10Z' } } }); bus.emit('state', { changes: { 'system:uptime': { value: 7225, available: true, updated_at: '2026-11-25T10:00:10Z' } } });
    assert.equal(changes.length, 1); run(); eq(changes[1], ['system:cpu', 'system:uptime']); assert.equal(store.view('system:cpu').value, 50);
    w.set(['system:cpu', 'ha:switch.lamp']);
    bus.emit('entity', { entities: [{ id: 'ha:switch.lamp', source: 'ha', kind: 'switch', name: 'Lamp', capabilities: [{ name: 'toggle', allowed: false, blocked_by: 'homedash.readonly' }], readonly: true, state: { value: 'on', available: true, updated_at: 'x' } }] });
    assert.equal(store.view('ha:switch.lamp').readonly, true);
    store.setDesktopReadonly(true); assert.equal(store.view('system:cpu').readonly, false); }],
  ['store: removed and rejected ids are unknown entities', () => {
    const { store, bus } = setup(); store.watch('win', { ids: ['system:cpu', 'nope:x'] });
    bus.emit('snapshot', { entities: [CPU], sources: [] }); bus.emit('removed', { ids: ['system:cpu'] }); bus.emit('subscribed', { accepted: [], rejected: [{ id: 'nope:x', reason: 'unknown_source' }] });
    assert.equal(store.view('system:cpu').status, 'unknown'); assert.equal(store.view('system:cpu').value, null); assert.equal(store.view('nope:x').status, 'unknown'); assert.equal(store.view('nope:x').name, 'x'); }],
  ['store: 5 s down marks every value stale; a snapshot clears it per id; degraded sources are stale', async () => {
    const { store, bus, clock, run } = setup(); store.watch('win', { ids: ['system:cpu', 'system:uptime'] });
    bus.emit('snapshot', { entities: [CPU, UPTIME], sources: [] }); run();
    bus.emit('connection', { phase: 'down', reason: 'error' }); await clock.advance(4999);
    assert.equal(store.connection().offline, false); assert.equal(store.view('system:cpu').stale, false);
    await clock.advance(1); assert.equal(store.connection().offline, true); assert.equal(store.view('system:cpu').stale, true);
    bus.emit('connection', { phase: 'live', reason: '' }); assert.equal(store.connection().offline, false); assert.equal(store.view('system:cpu').stale, true);
    bus.emit('snapshot', { entities: [CPU], sources: [] }); assert.equal(store.view('system:cpu').stale, false); assert.equal(store.view('system:uptime').stale, true);
    bus.emit('source', { source: 'system', state: 'degraded', reason: 'stale_metrics', at: 'x' }); assert.equal(store.view('system:cpu').stale, true); eq(store.sources().map(s => s.state), ['degraded']); }],
  ['store: history becomes host series, is cached, grows with live states and expires', async () => {
    const calls = [];
    const { store, bus, clock } = setup({ history(id, r) { calls.push({ id, span: r.to - r.from, points: r.points });
      return Promise.resolve({ status: 'ok', id, origin: 'source', points: [{ t: '2026-11-25T09:58:40Z', v: 11.8 }, { t: '2026-11-25T09:59:50Z', v: 12.3 }, { t: 'bad', v: 1 }, { t: '2026-11-25T09:59:00Z', v: null }] }); } });
    store.watch('win', { ids: ['system:cpu'] }); bus.emit('snapshot', { entities: [CPU], sources: [] });
    const s1 = await store.history('system:cpu', '1h', 60);
    eq(s1, { points: [[Date.parse('2026-11-25T09:58:40Z'), 11.8], [Date.parse('2026-11-25T09:59:00Z'), null], [Date.parse('2026-11-25T09:59:50Z'), 12.3]], source: 'source' });
    eq(calls, [{ id: 'system:cpu', span: 3600000, points: 60 }]);
    await store.history('system:cpu', 3600000, 60); assert.equal(calls.length, 1);
    bus.emit('state', { changes: { 'system:cpu': { value: 14, available: true, updated_at: '2026-11-25T10:00:10Z' } } });
    bus.emit('state', { changes: { 'system:cpu': { value: 99, available: true, updated_at: '2026-11-25T09:00:00Z' } } });
    const s2 = await store.history('system:cpu', '1h', 60); assert.equal(s2.points.length, 4); eq(s2.points[3], [Date.parse('2026-11-25T10:00:10Z'), 14]);
    await clock.advance(60000); await store.history('system:cpu', '1h', 60); assert.equal(calls.length, 2); }],
  ['store: at most four history requests run at once', async () => {
    const pending = []; const { store, clock } = setup({ history: () => new Promise(resolve => pending.push(() => resolve({ origin: 'source', points: [] }))) });
    const all = ['a:1', 'a:2', 'a:3', 'a:4', 'a:5', 'a:6'].map(id => store.history(id, '24h', 120)); await clock.flush(); assert.equal(pending.length, 4);
    pending.shift()(); await clock.flush(); assert.equal(pending.length, 4); while (pending.length) { pending.shift()(); await clock.flush(); } await Promise.all(all); }],
  ['store: ids of a left page linger 30 s; release detaches the owner', async () => {
    const { store, handles, clock } = setup(), w = store.watch('win', { ids: ['system:cpu', 'system:disk'] }), h = handles[0];
    w.set(['system:uptime']); eq(h.sets[h.sets.length - 1], ['system:uptime', 'system:cpu', 'system:disk']);
    await clock.advance(29999); assert.equal(h.sets.length, 2); await clock.advance(1); eq(h.sets[h.sets.length - 1], ['system:uptime']);
    w.setActive(false); assert.equal(h.active, false); w.release(); assert.equal(h.detached, true); }],
  ['store: the widget catalog is cached by hash and refreshed with If-None-Match', async () => {
    const asked = []; let answer = { status: 'ok', catalog: { hash: 'h1', widgets: [] } };
    const { store } = setup({ widgetCatalog: etag => { asked.push(etag); return Promise.resolve(answer); } });
    assert.equal((await store.widgetCatalog('h1')).hash, 'h1'); assert.equal((await store.widgetCatalog('h1')).hash, 'h1'); eq(asked, ['']);
    answer = { status: 'ok', not_modified: true }; assert.equal((await store.widgetCatalog('h2')).hash, 'h1'); eq(asked, ['', 'h1']); }]
];
```

- [ ] Run `npm run test:homedash` → the eight store cases FAIL (missing file).
- [ ] Implement `ui/js/homedash/hd-store.js`:

```js
// HomeDash store (F1; design-frontend 3.3 under the M2 contract). One per JS realm (shared(), keyed by the API base path). It turns stream
// frames into entity views for the widget host, batches change notices into one animation frame, marks values stale, keeps the ids of a
// left page subscribed for 30 s, and caches history series and the widget catalog. Rules:
// 1 A snapshot is the truth for the ids it lists (the contract's snapshot covers newly added ids only).
// 2 stale = server stale:true, or the stream was connecting/down for 5 s or stopped (set on every entry, cleared per id by its next snapshot
//   or state), or the entity's source is degraded/unavailable. Only available entities are stale.
// 3 readonly only for an entity with a command capability (not stream/snapshot/history/numeric) that the server marks read-only or that sits
//   on a read-only desktop: a pure display is never drawn locked.
// 4 removed and PUT-rejected ids are status unknown.
// 5 History {origin, points:[{t, v}]} becomes the widget host's series {points: [[t_ms, value|null]], source: origin}; cache key id|span|points,
//   TTL 60 s under 6 h spans else 5 min, LRU 100, 4 requests at a time; live numeric states append to every cached series of the id.
// View: {id, source, kind, name, area, unit, device_class, icon_hint, value, text, attributes (+updated_at), available, status, readonly,
// capabilities, pending: false, stale, error: null, tone: null}.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.api) throw new Error('hd-store.js: load hd-core.js and hd-api.js first');
    const GRACE_MS = 5000, LINGER_MS = 30000, SERIES_MAX = 100, SERIES_POINTS = 1000, PARALLEL = 4, SHORT_TTL = 60000, LONG_TTL = 300000, SIX_HOURS = 21600000;
    const READ_CAPS = Object.freeze(['stream', 'snapshot', 'history', 'numeric']), STALE_STATES = Object.freeze(['degraded', 'unavailable']);
    const isObj = v => v !== null && typeof v === 'object' && !Array.isArray(v);
    const strings = v => (v == null ? [] : Array.from(v)).map(String);
    function rangeMs(range) {
        if (typeof range === 'number' && Number.isFinite(range)) return range;
        const m = /^(\d+)(m|h|d)$/.exec(String(range || '24h'));
        return m ? Number(m[1]) * { m: 60000, h: 3600000, d: 86400000 }[m[2]] : 86400000;
    }
    function convert(answer) {
        const out = [];
        for (const p of answer && Array.isArray(answer.points) ? answer.points : []) {
            if (!isObj(p)) continue;
            const t = Date.parse(p.t), v = p.v === null ? null : Number(p.v);
            if (Number.isFinite(t) && (v === null || Number.isFinite(v))) out.push([t, v]);
        }
        return out.sort((a, b) => a[0] - b[0]);
    }
    function create(options) {
        const o = options || {}, api = o.api, stream = o.stream;
        if (!api || !stream) throw new Error('hd-store: api and stream are required');
        const setT = o.setTimeout || ((fn, ms) => setTimeout(fn, ms)), clearT = o.clearTimeout || (id => clearTimeout(id)), now = o.now || (() => Date.now());
        const bus = E.core.emitter(), entries = new Map(), sources = new Map(), owners = new Map(), series = new Map(), queue = [], unhook = [];
        let changed = new Set(), all = false, desktopReadonly = false, globalRev = 0, conn = stream.status() || {}, offline = false, graceTimer = 0, lingerTimer = 0, running = 0, catalog = null, catalogLoad = null;
        const flush = E.core.frame(() => { const ids = all ? null : changed; changed = new Set(); all = false; bus.emit('change', ids); }, { raf: o.raf, caf: o.caf });
        const touch = id => { if (!all) changed.add(id); flush(); };
        const touchAll = () => { all = true; changed = new Set(); globalRev += 1; flush(); };
        const watched = id => { for (const rec of owners.values()) if (rec.ids.has(id) || rec.linger.has(id)) return true; return false; };
        const sourceOf = e => (e.meta && typeof e.meta.source === 'string' ? e.meta.source : String(e.id).split(':')[0]);
        const sourceStale = src => { const s = sources.get(src); return !!(s && STALE_STATES.includes(s.state)); };
        function entry(id) {
            let e = entries.get(id);
            if (!e && watched(id)) { e = { id, meta: null, state: null, status: 'ok', reason: '', connStale: false, rev: 0, cache: null }; entries.set(id, e); }
            return e || null;
        }
        const bump = e => { e.rev += 1; touch(e.id); };
        function setEntity(ent) {
            if (!isObj(ent) || typeof ent.id !== 'string') return;
            const e = entry(ent.id); if (!e) return;
            e.meta = ent; if (isObj(ent.state)) e.state = ent.state; e.status = 'ok'; e.reason = ''; e.connStale = false; bump(e);
        }
        function setSources(list) {
            const flipped = [];
            for (const s of Array.isArray(list) ? list : []) {
                if (!isObj(s) || typeof s.source !== 'string') continue;
                const before = sourceStale(s.source); sources.set(s.source, s); if (before !== sourceStale(s.source)) flipped.push(s.source);
            }
            for (const e of entries.values()) if (flipped.includes(sourceOf(e))) bump(e);
            if (Array.isArray(list) && list.length) bus.emit('sources', [...sources.values()]);
        }
        function appendLive(id, st) {
            const t = Date.parse(st.updated_at), v = typeof st.value === 'number' && Number.isFinite(st.value) ? st.value : null;
            if (v === null || st.available === false || !Number.isFinite(t)) return;
            for (const s of series.values()) {
                if (s.id !== id || !s.result) continue;
                const pts = s.result.points, last = pts[pts.length - 1];
                if (last && t <= last[0]) continue;
                pts.push([t, v]);
                let cut = 0; while (cut < pts.length - 1 && pts[cut][0] < t - s.span) cut += 1;
                if (pts.length - cut > SERIES_POINTS) cut = pts.length - SERIES_POINTS;
                if (cut > 0) pts.splice(0, cut);
            }
        }
        const connection = () => Object.freeze({ phase: conn.phase || 'idle', reason: conn.reason || '', offline, nextRetryAt: conn.nextRetryAt || 0, attempt: conn.attempt || 0 });
        function goOffline() {
            if (offline) return;
            offline = true; for (const e of entries.values()) { e.connStale = true; e.rev += 1; }
            touchAll(); bus.emit('connection', connection());
        }
        function onConnection(st) {
            conn = st || {};
            if (conn.phase === 'stopped') { clearT(graceTimer); graceTimer = 0; goOffline(); }
            else if (conn.phase === 'connecting' || conn.phase === 'down') { if (!offline && !graceTimer) graceTimer = setT(() => { graceTimer = 0; goOffline(); }, GRACE_MS); }
            else { clearT(graceTimer); graceTimer = 0; offline = false; }
            bus.emit('connection', connection());
        }
        const listen = (type, fn) => { stream.on(type, fn); unhook.push(() => stream.off(type, fn)); };
        listen('snapshot', d => { for (const ent of Array.isArray(d.entities) ? d.entities : []) setEntity(ent); setSources(d.sources); });
        listen('entity', d => { for (const ent of Array.isArray(d.entities) ? d.entities : []) setEntity(ent); });
        listen('removed', d => { for (const id of strings(d.ids)) { const e = entry(id); if (e) { e.status = 'unknown'; e.reason = 'removed'; bump(e); } } });
        listen('state', d => {
            if (!isObj(d.changes)) return;
            for (const [id, st] of Object.entries(d.changes)) { if (!isObj(st)) continue; const e = entry(id); if (!e) continue; e.state = st; e.connStale = false; appendLive(id, st); bump(e); }
        });
        listen('source', d => setSources([d]));
        listen('subscribed', d => { for (const r of Array.isArray(d.rejected) ? d.rejected : []) { if (!isObj(r)) continue; const e = entry(String(r.id)); if (e) { e.status = 'unknown'; e.reason = String(r.reason || 'rejected'); bump(e); } } });
        listen('dashboard_changed', d => bus.emit('dashboards', d));
        listen('connection', onConnection);

        const hasCommand = caps => Array.isArray(caps) && caps.some(c => isObj(c) && typeof c.name === 'string' && !READ_CAPS.includes(c.name));
        function view(id) {
            const e = entries.get(id);
            if (!e || (!e.meta && !e.state && e.status !== 'unknown')) return null;
            const key = e.rev + ':' + globalRev;
            if (e.cache && e.cache.key === key) return e.cache.view;
            const m = e.meta || {}, s = e.state || {}, src = sourceOf(e), caps = Array.isArray(m.capabilities) ? m.capabilities : [];
            const unknown = e.status === 'unknown', available = !unknown && !!e.state && s.available !== false;
            const v = Object.freeze({
                id, source: src, kind: typeof m.kind === 'string' ? m.kind : 'generic', name: typeof m.name === 'string' && m.name ? m.name : String(id).slice(src.length + 1),
                area: typeof m.area === 'string' ? m.area : '', unit: typeof m.unit === 'string' ? m.unit : '', device_class: typeof m.device_class === 'string' ? m.device_class : '',
                icon_hint: typeof m.icon_hint === 'string' ? m.icon_hint : '', value: unknown || s.value === undefined ? null : s.value, text: typeof s.text === 'string' ? s.text : '',
                attributes: Object.freeze(Object.assign({}, isObj(s.attributes) ? s.attributes : {}, { updated_at: typeof s.updated_at === 'string' ? s.updated_at : null })),
                available, status: unknown ? 'unknown' : available ? 'ok' : 'unavailable', readonly: hasCommand(caps) && (m.readonly === true || desktopReadonly),
                capabilities: caps, pending: false, stale: available && (s.stale === true || e.connStale || sourceStale(src)), error: null, tone: null });
            e.cache = { key, view: v };
            return v;
        }
        function prune() { for (const id of [...entries.keys()]) if (!watched(id)) entries.delete(id); }
        function armLinger() {
            clearT(lingerTimer); lingerTimer = 0;
            let next = Infinity; for (const rec of owners.values()) for (const at of rec.linger.values()) next = Math.min(next, at);
            if (next === Infinity) return;
            lingerTimer = setT(() => {
                lingerTimer = 0; const t = now();
                for (const rec of owners.values()) { let cut = false; for (const [id, at] of [...rec.linger]) if (at <= t) { rec.linger.delete(id); cut = true; } if (cut) rec.push(); }
                prune(); armLinger();
            }, Math.max(0, next - now()));
        }
        function watch(ownerId, init) {
            const i = init || {}, rec = { ids: new Set(strings(i.ids)), linger: new Map(), push: null };
            const handle = stream.attach(ownerId, { entities: [], active: i.active !== false });
            rec.push = () => handle.set([...rec.ids, ...rec.linger.keys()]);
            owners.set(handle.id, rec); rec.push();
            return Object.freeze({
                set(list) {
                    const next = new Set(strings(list)), until = now() + LINGER_MS;
                    for (const id of rec.ids) if (!next.has(id)) rec.linger.set(id, until);
                    for (const id of next) rec.linger.delete(id);
                    rec.ids = next; rec.push(); armLinger(); prune();
                },
                setActive(active) { handle.setActive(!!active); },
                release() { if (!owners.delete(handle.id)) return; handle.detach(); prune(); armLinger(); }
            });
        }
        function limit(task) { return new Promise((resolve, reject) => { queue.push({ task, resolve, reject }); pump(); }); }
        function pump() {
            while (running < PARALLEL && queue.length) {
                const job = queue.shift(); running += 1;
                Promise.resolve().then(job.task).then(job.resolve, job.reject).then(() => { running -= 1; pump(); });
            }
        }
        function history(id, range, points) {
            const span = E.core.clamp(rangeMs(range), 60000, 7 * 86400000), n = E.core.clamp(Math.round(Number(points) || 120), 1, 500), key = id + '|' + span + '|' + n;
            const hit = series.get(key);
            if (hit && now() - hit.at < (span < SIX_HOURS ? SHORT_TTL : LONG_TTL)) { series.delete(key); series.set(key, hit); return hit.promise; }
            const s = { id, span, at: now(), result: null, promise: null };
            s.promise = limit(() => api.history(id, { from: now() - span, to: now(), points: n })).then(answer => {
                s.result = { points: convert(answer), source: answer && typeof answer.origin === 'string' ? answer.origin : 'none' };
                return s.result;
            }, err => { if (series.get(key) === s) series.delete(key); throw E.api.normalizeError(err); });
            series.set(key, s);
            while (series.size > SERIES_MAX) series.delete(series.keys().next().value);
            return s.promise;
        }
        function widgetCatalog(hash) {
            if (catalog && (!hash || catalog.hash === hash)) return Promise.resolve(catalog);
            if (catalogLoad) return catalogLoad;
            catalogLoad = api.widgetCatalog(catalog ? catalog.hash : '').then(answer => {
                if (answer && answer.not_modified && catalog) return catalog;
                if (!answer || !isObj(answer.catalog)) throw E.api.normalizeError({ status: 0, body: { code: 'bad_catalog', message: 'widget catalog missing' } });
                catalog = answer.catalog; return catalog;
            }).finally(() => { catalogLoad = null; });
            return catalogLoad;
        }
        return Object.freeze({
            view, watch, history, widgetCatalog, connection, setSources, on: bus.on, off: bus.off, sources: () => [...sources.values()],
            setDesktopReadonly(flag) { if (desktopReadonly === !!flag) return; desktopReadonly = !!flag; touchAll(); },
            dispose() { for (const fn of unhook.splice(0)) fn(); clearT(graceTimer); clearT(lingerTimer); flush.cancel(); entries.clear(); series.clear(); owners.clear(); }
        });
    }
    const instances = new Map();
    function shared(options) { const key = options && options.api ? options.api.base : ''; if (!instances.has(key)) instances.set(key, create(options)); return instances.get(key); }
    E.store = Object.freeze({ create, shared, convert, rangeMs, GRACE_MS, LINGER_MS });
})();
```

- [ ] Run `npm run test:homedash` → eight store cases PASS.
- [ ] Detect-changes; commit `ui/js/homedash/hd-store.js scripts/homedash-tests/store.test.mjs` — `feat(homedash): realm store with staleness, linger and history series`.

**Done when:** eight PASS lines; `hd-store.js` under 700 lines.

---

### Task 5: hd-catalog.js

**Files:** create `ui/js/homedash/hd-catalog.js`, `scripts/homedash-tests/catalog.test.mjs`.
**Read first:** `internal/homedash/testdata/widget_catalog.json` (widgets: `variants[].min_size/default_size`, `max_size`, `bindings[].required`, `options[].default`).

- [ ] Write the failing test:

```js
import assert from 'node:assert/strict'; import { loadEngine, ENGINE_ORDER, read, eq } from './harness.mjs';
export const tests = [['catalog: sizes, constraints and option defaults come from the golden catalog; the registry matches it', () => {
  const { E } = loadEngine(ENGINE_ORDER.concat(['hd-catalog.js'])), raw = JSON.parse(read('internal/homedash/testdata/widget_catalog.json')), c = E.catalog.create(raw);
  assert.equal(c.hash, raw.hash); eq(c.defaultSize({ type: 'gauge', variant: 'arc' }), { w: 2, h: 2 }); eq(c.defaultSize({ type: 'sensor', variant: 'sparkline' }), { w: 2, h: 2 });
  eq(c.defaultSize({ type: 'nope' }), { w: 2, h: 1 }); eq(c.constraints({ type: 'sensor', variant: 'sparkline' }), { minW: 2, minH: 1, maxW: 6, maxH: 4 });
  eq(c.options({ type: 'gauge', options: { warn: 80 } }), { min: 0, max: 100, decimals: 0, warn: 80 }); eq(c.options({ type: 'entity' }), {});
  eq(c.registryMismatches(E.widgets), []); }]];
```

- [ ] Run `npm run test:homedash` → FAIL (missing file).
- [ ] Implement `ui/js/homedash/hd-catalog.js`:

```js
// HomeDash widget catalog access (F1). create(json) wraps the server catalog (GET widget-catalog, golden file
// internal/homedash/testdata/widget_catalog.json): default size and size constraints per type and variant, option defaults merged under a
// widget's own options, and registryMismatches(E.widgets): every registered type exists, its variants equal the catalog's, and it implements
// every required binding. The client never copies catalog numbers.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core) throw new Error('hd-catalog.js: load hd-core.js first');
    const size = s => (s && Number.isFinite(s.w) && Number.isFinite(s.h) ? { w: s.w, h: s.h } : null);
    function create(raw) {
        const cat = raw && typeof raw === 'object' ? raw : {};
        const byType = new Map((Array.isArray(cat.widgets) ? cat.widgets : []).filter(w => w && typeof w.type === 'string').map(w => [w.type, w]));
        function variant(type, id) { const w = byType.get(type), list = w && Array.isArray(w.variants) ? w.variants : []; return list.find(v => v && v.id === id) || list[0] || null; }
        const defaultSize = widget => size((variant(widget.type, widget.variant) || {}).default_size) || { w: 2, h: 1 };
        function constraints(widget) {
            const w = byType.get(widget.type) || {}, v = variant(widget.type, widget.variant) || {}, min = size(v.min_size), max = size(v.max_size) || size(w.max_size);
            return { minW: min ? min.w : 1, minH: min ? min.h : 1, maxW: max ? max.w : 12, maxH: max ? max.h : 24 };
        }
        function optionDefaults(type) {
            const w = byType.get(type), out = {};
            for (const opt of w && Array.isArray(w.options) ? w.options : []) if (opt && typeof opt.key === 'string' && opt.default !== undefined && opt.default !== null) out[opt.key] = opt.default;
            return out;
        }
        const options = widget => Object.assign(optionDefaults(widget.type), widget.options && typeof widget.options === 'object' ? widget.options : {});
        function registryMismatches(registry) {
            const out = [];
            for (const type of registry.types()) {
                const def = registry.get(type), w = byType.get(type);
                if (!w) { out.push(type + ': not in the catalog'); continue; }
                const want = (w.variants || []).map(v => v.id).sort().join(','), have = def.variants.slice().sort().join(',');
                if (want !== have) out.push(type + ': variants ' + have + ' vs catalog ' + want);
                for (const b of w.bindings || []) if (b && b.required && !def.bindings.includes(b.name)) out.push(type + ': missing required binding ' + b.name);
            }
            return out;
        }
        return Object.freeze({ hash: String(cat.hash || ''), has: type => byType.has(type), widget: type => byType.get(type) || null, variant, defaultSize, constraints, optionDefaults, options, registryMismatches });
    }
    E.catalog = Object.freeze({ create });
})();
```

- [ ] Run `npm run test:homedash` → PASS. If `registryMismatches` reports an M0 registry gap, do not change the widget modules: report the list to the lead (engine contract).
- [ ] Detect-changes; commit `ui/js/homedash/hd-catalog.js scripts/homedash-tests/catalog.test.mjs` — `feat(homedash): widget catalog access`.

**Done when:** PASS and the registry matches the golden catalog.

---

### Task 6: Translations for every M2 key

**Files:** modify `ui/lang/homedash/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json` and `ui/lang/desktop/{same 16}.json`.
**Read first:** `ui/js/homedash/AGENTS.md` "Translations"; root `AGENTS.md` (16 locales, no English filler, German "Du" with capital D and real umlauts).

- [ ] Add these keys to all 16 `ui/lang/homedash/*.json` (English and German given; translate the other 14 locales for real, placeholders `{{…}}` unchanged, no English filler; keep the files sorted as they are now, LF, no BOM):

| Key | en | de |
|---|---|---|
| homedash.app.loading | Loading HomeDash… | HomeDash wird geladen… |
| homedash.home.title | Dashboards | Dashboards |
| homedash.home.empty_title | Create your first dashboard | Erstelle Dein erstes Dashboard |
| homedash.home.empty_text | A dashboard shows your devices and the values of this server at a glance. Start with a ready-made system dashboard or with an empty page. | Ein Dashboard zeigt Deine Geräte und die Werte dieses Servers auf einen Blick. Beginne mit einem fertigen System-Dashboard oder mit einer leeren Seite. |
| homedash.home.create_starter | Create starter dashboard | Start-Dashboard erstellen |
| homedash.home.create_starter_hint | CPU, memory, disk, network and uptime of this server, live. | CPU, Arbeitsspeicher, Festplatte, Netzwerk und Laufzeit dieses Servers, live. |
| homedash.home.create_empty | Empty dashboard | Leeres Dashboard |
| homedash.home.create_empty_hint | A blank page for your own layout. | Eine leere Seite für Dein eigenes Layout. |
| homedash.home.starter_name | System | System |
| homedash.home.empty_name | My dashboard | Mein Dashboard |
| homedash.home.page_main | Main | Hauptseite |
| homedash.home.open | Open {{name}} | {{name}} öffnen |
| homedash.home.delete | Delete {{name}} | {{name}} löschen |
| homedash.home.updated | Updated {{when}} | Aktualisiert {{when}} |
| homedash.home.by_agent | saved by the agent | vom Agenten gespeichert |
| homedash.home.readonly_note | This desktop is read-only. You can view dashboards but not create or delete them. | Dieser Desktop ist schreibgeschützt. Du kannst Dashboards ansehen, aber nicht erstellen oder löschen. |
| homedash.home.notice_deleted | The dashboard was deleted. | Das Dashboard wurde gelöscht. |
| homedash.home.notice_missing | This dashboard no longer exists. | Dieses Dashboard gibt es nicht mehr. |
| homedash.home.notice_open_failed | The dashboard could not be opened. | Das Dashboard konnte nicht geöffnet werden. |
| homedash.home.notice_list_failed | The dashboards could not be loaded. | Die Dashboards konnten nicht geladen werden. |
| homedash.home.notice_limit | The dashboard limit is reached. Delete a dashboard first. | Die Höchstzahl an Dashboards ist erreicht. Lösche zuerst ein Dashboard. |
| homedash.home.notice_create_failed | The dashboard could not be created. | Das Dashboard konnte nicht erstellt werden. |
| homedash.home.notice_readonly | The desktop is read-only; nothing was changed. | Der Desktop ist schreibgeschützt; es wurde nichts geändert. |
| homedash.home.notice_delete_conflict | The dashboard was changed in the meantime. Check the list and try again. | Das Dashboard wurde inzwischen geändert. Prüfe die Liste und versuche es noch einmal. |
| homedash.home.notice_delete_failed | The dashboard could not be deleted. | Das Dashboard konnte nicht gelöscht werden. |
| homedash.home.retry | Try again | Erneut versuchen |
| homedash.delete.title | Delete dashboard? | Dashboard löschen? |
| homedash.delete.body | “{{name}}” and its saved versions will be removed. This cannot be undone. | „{{name}}“ und seine gespeicherten Versionen werden entfernt. Das lässt sich nicht rückgängig machen. |
| homedash.delete.confirm | Delete | Löschen |
| homedash.delete.cancel | Cancel | Abbrechen |
| homedash.view.back | Dashboards | Dashboards |
| homedash.view.pages | Pages | Seiten |
| homedash.view.empty_title | This dashboard is empty | Dieses Dashboard ist leer |
| homedash.view.empty_text | Widgets are added with the dashboard designer, which arrives in a later update. | Widgets fügst Du mit dem Dashboard-Designer hinzu, der mit einem späteren Update kommt. |
| homedash.view.unsupported | Not available in this version | In dieser Version nicht verfügbar |
| homedash.conn.live | Live | Live |
| homedash.conn.connecting | Connecting… | Verbinde… |
| homedash.conn.offline | Offline | Offline |
| homedash.banner.offline | Connection lost. The values shown may be out of date. Reconnecting… | Verbindung unterbrochen. Die angezeigten Werte sind eventuell veraltet. Verbinde neu… |
| homedash.banner.retry | Retry now | Jetzt neu verbinden |
| homedash.banner.auth_lost | Your session has ended. Sign in again to see live values. | Deine Sitzung ist abgelaufen. Melde Dich erneut an, um Live-Werte zu sehen. |
| homedash.banner.source_degraded | {{source}} reports old values; they may be out of date. | {{source}} meldet alte Werte; sie sind eventuell veraltet. |
| homedash.banner.source_unavailable | {{source}} is not reachable. | {{source}} ist nicht erreichbar. |
| homedash.banner.schema_too_new | This dashboard was saved by a newer HomeDash version and is shown as it is. | Dieses Dashboard wurde von einer neueren HomeDash-Version gespeichert und wird unverändert angezeigt. |
| homedash.banner.readonly | Read-only desktop: dashboards can be viewed, not changed. | Schreibgeschützter Desktop: Dashboards lassen sich ansehen, aber nicht ändern. |
| homedash.source.system | System | System |
| homedash.disabled.title | HomeDash is switched off | HomeDash ist ausgeschaltet |
| homedash.disabled.text | HomeDash is switched off in the AuraGo settings. Switch it on there to use your dashboards. | HomeDash ist in den AuraGo-Einstellungen ausgeschaltet. Schalte es dort ein, um Deine Dashboards zu nutzen. |
| homedash.disabled.retry | Check again | Erneut prüfen |
| homedash.error.title | HomeDash could not start | HomeDash konnte nicht starten |
| homedash.error.storage | The dashboard storage could not be opened. The server log has the details. | Der Dashboard-Speicher konnte nicht geöffnet werden. Details stehen im Server-Log. |
| homedash.error.generic | The server did not answer as expected ({{code}}). | Der Server hat nicht wie erwartet geantwortet ({{code}}). |
| homedash.menu.all | All dashboards | Alle Dashboards |
| homedash.menu.new_starter | New starter dashboard | Neues Start-Dashboard |
| homedash.menu.new_empty | New empty dashboard | Neues leeres Dashboard |
| homedash.menu.reload | Reload | Neu laden |

- [ ] Add `"desktop.app_homedash": "HomeDash"` (brand name, identical in all locales) to all 16 `ui/lang/desktop/*.json`, next to `desktop.app_ha_switchboard`.
- [ ] Run `go test ./ui -run '(?i)^Test(Translations_|Frontend_StaticI18nKeysExistInEnglishBundle|DesktopDynamicI18nPrefixKeysExistInAllLanguages)' -count=1` → ok (no failing case; this proves 16 files parse and carry every key).
- [ ] Detect-changes; commit the 32 locale files — `i18n(homedash): M2 app, home, view and banner texts in 16 locales`.

**Done when:** the translation tests pass and `de.json` uses "Du/Dein" forms only.

---

### Task 7: Engine behaviour for live data (SCR-M2-1 tile, SCR-M2-2 sparkline tail)

**Structure change request.** `widgets/hd-w-*.js` and `homedash-widgets.css` are frozen structure. This task changes behaviour and adds tile geometry; it renames nothing and changes no existing markup or token. **The lead approves SCR-M2-1 and SCR-M2-2 before dispatch.** If the lead declines one, skip its steps here and delete the blocks marked `// SCR-M2-1` or `// SCR-M2-2` in Task 18.

**Files:** modify `ui/js/homedash/widgets/hd-w-misc.js`, `ui/js/homedash/widgets/hd-w-sensor.js`, `ui/css/homedash-widgets.css`, `ui/css/homedash.bundle.css` (generated); create `scripts/homedash-tests/scr.test.mjs`.
**Read first:** `widgets/hd-w-misc.js` (whole), `widgets/hd-w-sensor.js:113-160`, `ui/css/homedash-widgets.css:1-40,82-91`, `documentation/homedash-visual-system.md` "Structure and paint".

Why: the starter's uptime is an `entity/tile` (D3) whose `.hd-tile` has no production CSS (only lab CSS) and shows raw seconds (contract: "format as a duration in the UI"); the sparkline module reads history once (`hd-w-sensor.js:146`), so a cached-series append alone never moves a visible line.

- [ ] Write the failing test `scripts/homedash-tests/scr.test.mjs`:

```js
import assert from 'node:assert/strict'; import { loadEngine, ENGINE_ORDER } from './harness.mjs';
function hostWidget(widget, ref, history) {
  const { E, document } = loadEngine(ENGINE_ORDER, { dom: true }), parent = document.createElement('div'); parent.setAttribute('data-hd-family', 'glass'); document.body.appendChild(parent);
  const api = () => ({ entity: () => ref.current, can: () => false, command: () => Promise.reject(new Error('view only')), moreInfo() {}, history: () => Promise.resolve(history || { points: [], source: 'source' }), announce() {} });
  return { E, parent, inst: E.widgets.host({ widget, mode: 'view', size: { w: 2, h: 2 }, t: k => k, lang: 'en', api, parent }) };
}
const view = (value, extra) => Object.assign({ id: 'system:uptime', kind: 'sensor', name: 'Uptime', unit: 's', device_class: 'duration', icon_hint: 'clock', value, available: true, status: 'ok',
  readonly: false, stale: false, pending: false, attributes: { updated_at: '2026-11-25T10:00:00Z' } }, extra);
const flush = async () => { for (let i = 0; i < 10; i++) await Promise.resolve(); };
export const tests = [
  ['SCR-M2-1: the entity tile writes a duration for duration sensors and shows a known icon hint', () => {
    const ref = { current: view(7215) }, { E, parent, inst } = hostWidget({ id: 'uptime', type: 'entity', variant: 'tile', bindings: { primary: 'system:uptime' } }, ref);
    assert.equal(parent.querySelector('.hd-num-int').textContent, E.format.duration(7215, 'en')); assert.equal(parent.querySelector('.hd-num-unit').textContent, '');
    assert.ok(parent.querySelector('.hd-iconbadge [data-hd-icon="clock"]'));
    ref.current = view(12.5, { unit: '%', device_class: '', icon_hint: 'nope' }); inst.update();
    assert.equal(parent.querySelector('.hd-num-int').textContent + parent.querySelector('.hd-num-frac').textContent, '12.5');
    assert.equal(parent.querySelector('.hd-num-unit').textContent, '%'); assert.ok(parent.querySelector('.hd-iconbadge [data-hd-icon="entity"]')); }],
  ['SCR-M2-2: the sparkline extends with fresh live values after its history arrived', async () => {
    const T = Date.parse('2026-11-25T10:00:00Z'), hist = { points: [[T - 120000, 10], [T - 60000, 12], [T, 11]], source: 'source' };
    const ref = { current: view(11, { id: 'system:net_rx', unit: 'Mbit/s', device_class: 'data_rate' }) };
    const { inst, parent } = hostWidget({ id: 'net', type: 'sensor', variant: 'sparkline', bindings: { primary: 'system:net_rx' } }, ref, hist);
    await flush(); const line = parent.querySelector('.hd-spark-line'), before = line.getAttribute('d');
    ref.current = Object.assign({}, ref.current, { value: 30, attributes: { updated_at: '2026-11-25T10:00:10Z' } }); inst.update();
    const after = line.getAttribute('d'); assert.notEqual(after, before);
    ref.current = Object.assign({}, ref.current, { value: 99, stale: true, attributes: { updated_at: '2026-11-25T10:00:20Z' } }); inst.update(); assert.equal(line.getAttribute('d'), after);
    ref.current = Object.assign({}, ref.current, { value: 5, stale: false, attributes: { updated_at: '2026-11-25T09:59:00Z' } }); inst.update(); assert.equal(line.getAttribute('d'), after); }]
];
```

- [ ] Run `npm run test:homedash` → both SCR cases FAIL (raw `7215`; unchanged `d`).
- [ ] GitNexus impact for the `entity` registration in `hd-w-misc.js` and `sparkline` in `hd-w-sensor.js` (callers: `E.widgets.host` via the registry, the lab). Expect LOW; report.
- [ ] `hd-w-misc.js`: add `const DURATION_UNITS = Object.freeze({ s: 1, min: 60, h: 3600, d: 86400 });` after the `const { el, setAttr } …` line, and replace the start of `update(v)` up to and including the numeric branch with:

```js
                update(v) {
                    const e = v.entities.primary;
                    // SCR-M2-1: a known icon hint names the badge glyph; duration sensors read as a duration, not as seconds.
                    icon.set({ icon: e && E.icons.has(e.icon_hint) ? e.icon_hint : (api.widget.icon || 'entity') });
                    if (!e) { num.set({ value: null }); show(true); return; }
                    if (typeof e.value === 'number') {
                        const k = e.device_class === 'duration' ? DURATION_UNITS[e.unit || 's'] : 0;
                        if (k) num.set({ text: F.duration(e.value * k, api.lang) }); else num.set({ value: e.value, unit: e.unit });
                        show(true); return;
                    }
```
(the rest of `update` stays as it is).
- [ ] `hd-w-sensor.js` in `sparkline(api)`: change `let lastView = null, hist = [], dims = null, lastShown;` to `let lastView = null, hist = [], dims = null, lastShown, loaded = false;`, add after it:

```js
        // SCR-M2-2: after the history arrived, a fresh numeric value newer than the last point extends the line; the window keeps the span of
        // the loaded series (at most 720 points). Stale, unavailable and older values never append.
        function tail(e) {
            if (!loaded || !e || hostWord(e) || e.stale) return;
            const v = numberOf(e.value), t = Date.parse(e.attributes && e.attributes.updated_at), last = hist[hist.length - 1];
            if (v === null || !Number.isFinite(t) || (last && t <= last[0])) return;
            const span = hist.length > 1 ? last[0] - hist[0][0] : 0;
            hist.push([t, v]);
            if (span > 0) while (hist.length > 2 && hist[0][0] < t - span) hist.shift();
            if (hist.length > 720) hist.splice(0, hist.length - 720);
        }
```
change the history callback to `history(api, s => { hist = Array.isArray(s.points) ? s.points.filter(Array.isArray) : []; loaded = true; if (lastView) render(); });` and `update(view) { lastView = view; tail(view.entities.primary); render(); },`.
- [ ] Append to `ui/css/homedash-widgets.css`:

```css
/* M2 entity tile (SCR-M2-1): icon badge on top, the numeral (or the lamp) below, start-aligned, numeral sized from the cell like the sensor numeral. */
.hd-widget[data-hd-type="entity"] .hd-body { place-items: stretch; }
.hd-widget[data-hd-type="entity"] .hd-tile { display: grid; grid-template-rows: auto minmax(0, 1fr); justify-items: start; align-items: end; gap: var(--hd-space-1); min-inline-size: 0; min-block-size: 0; }
.hd-widget[data-hd-type="entity"] .hd-tile > .hd-iconbadge { display: inline-grid; place-items: center; inline-size: 28px; block-size: 28px; color: var(--hd-ink-muted); }
.hd-widget[data-hd-type="entity"] .hd-tile > .hd-iconbadge .hd-ico { inline-size: 20px; block-size: 20px; }
.hd-widget[data-hd-type="entity"] .hd-tile > .hd-numeral { max-inline-size: 100%; overflow: hidden; font-size: min(20cqmin, var(--hd-num-hero-max)); font-weight: var(--hd-num-weight); letter-spacing: var(--hd-num-tracking); line-height: 1.05; white-space: nowrap; text-overflow: ellipsis; color: var(--hd-ink); }
.hd-widget[data-hd-type="entity"] .hd-tile > .hd-lamp { align-self: center; }
@container hd-widget (max-width: 139px) and (max-height: 139px) { .hd-widget[data-hd-type="entity"] .hd-tile > .hd-iconbadge { display: none; } }
```
- [ ] `npm run build:ui`, then `npm run test:homedash` → both SCR cases PASS, `m1.test.mjs` (token freeze) PASS, guard PASS. `npm run check:ui` → exit 0.
- [ ] Detect-changes; commit `ui/js/homedash/widgets/hd-w-misc.js ui/js/homedash/widgets/hd-w-sensor.js ui/css/homedash-widgets.css ui/css/homedash.bundle.css scripts/homedash-tests/scr.test.mjs` — `feat(homedash): duration tile and live sparkline tail (SCR-M2-1, SCR-M2-2)`.

**Done when:** both PASS, token freeze unchanged, lab hero history (ends at NOW, entity updated 2 min earlier) is not altered by the tail rule.

---

### Task 8: homedash-page.css, the tenth bundle part

**Files:** create `ui/css/homedash-page.css`; modify `scripts/build-ui-bundles.js` (`cssBundles` homedash entry), `scripts/homedash-tests/bundle.test.mjs`, `ui/css/homedash.bundle.css` (generated).
**Read first:** `scripts/homedash-tests/css-lint.mjs`, `scripts/homedash-tests/bundle.test.mjs`, `ui/js/homedash/hd-layout.js` (`metrics`, `cssVars`).

- [ ] GitNexus impact for `cssBundles` (`scripts/build-ui-bundles.js`) and `PARTS` (`bundle.test.mjs`, used by `guard.test.mjs`).
- [ ] Update `bundle.test.mjs` first (failing): `PARTS` gets `'page'` appended after `'widgets'`; the first test's name becomes `the bundle entry lists the ten parts in order and the parts are LF without BOM`; the regex in the second test becomes `/Checked ui\/css\/homedash\.bundle\.css \(10 parts\)/`; append this case:

```js
  ['page CSS places widgets from the render variables and declares no token', () => { const page = read('ui/css/homedash-page.css');
    for (const s of ['.hd-page > .hd-widget', 'var(--hd-x', 'var(--hd-colw', '.hd-tab[aria-selected="true"]', '.hd-page-empty', '.hd-widget.hd-unsupported']) assert.ok(page.includes(s), s);
    assert.ok(!/^\s*--hd-[\w-]+\s*:/m.test(page), 'the page part reads tokens only'); }]
```
- [ ] Run `npm run test:homedash` → FAIL (entry lists nine parts; file missing).
- [ ] Create `ui/css/homedash-page.css`:

```css
/* HomeDash page chrome (M2, part 10 of the bundle): the scroll stage, the page that places every widget from --hd-x/--hd-y/--hd-w/--hd-h
   (written by hd-render.js) and the metrics --hd-cols/--hd-rows/--hd-colw/--hd-rowh/--hd-gap/--hd-offx (E.layout.cssVars), the page tabs,
   the empty-page note and the unsupported cell. Reads --hd-* tokens only and declares none; no motion. */
.hd-root[data-hd-host] { position: relative; box-sizing: border-box; display: flex; flex-direction: column; inline-size: 100%; block-size: 100%; min-block-size: 0; overflow: hidden; }
.hd-stage { position: relative; flex: 1 1 auto; min-block-size: 0; overflow: auto; overscroll-behavior: contain; scrollbar-gutter: stable; }
.hd-root[data-hd-fit="fit"] .hd-stage { overflow: hidden; }
.hd-page { position: relative; box-sizing: border-box; min-block-size: 100%; block-size: calc(var(--hd-rows, 1) * var(--hd-rowh, 96px) + (var(--hd-rows, 1) + 1) * var(--hd-gap, 12px)); }
.hd-page > .hd-widget { position: absolute; box-sizing: border-box;
  inset-inline-start: calc(var(--hd-offx, 0px) + var(--hd-gap, 12px) + var(--hd-x, 0) * (var(--hd-colw, 96px) + var(--hd-gap, 12px)));
  inset-block-start: calc(var(--hd-gap, 12px) + var(--hd-y, 0) * (var(--hd-rowh, 96px) + var(--hd-gap, 12px)));
  inline-size: calc(var(--hd-w, 1) * var(--hd-colw, 96px) + (var(--hd-w, 1) - 1) * var(--hd-gap, 12px));
  block-size: calc(var(--hd-h, 1) * var(--hd-rowh, 96px) + (var(--hd-h, 1) - 1) * var(--hd-gap, 12px)); }
.hd-tabs { display: flex; flex: none; gap: var(--hd-space-1); padding: var(--hd-space-3) var(--hd-space-4) 0; overflow-x: auto; scrollbar-width: none; }
.hd-tab { flex: none; min-block-size: 36px; padding: 0 var(--hd-space-4); border: 0; border-radius: var(--hd-radius-pill); background: transparent; color: var(--hd-ink-muted); font: var(--hd-label-weight) var(--hd-text-m) / 1 var(--hd-font-ui); cursor: pointer; }
.hd-tab:hover { color: var(--hd-ink); background: color-mix(in srgb, var(--hd-ink) 6%, transparent); }
.hd-tab[aria-selected="true"] { background: var(--hd-selected); color: var(--hd-selected-ink); }
.hd-page-empty { position: absolute; inset: 0; display: grid; place-content: center; justify-items: center; gap: var(--hd-space-2); padding: var(--hd-space-6); text-align: center; color: var(--hd-ink-muted); }
.hd-page-empty > .hd-ico { inline-size: 48px; block-size: 48px; color: var(--hd-ink-faint); stroke-width: 1.5; }
.hd-page-empty-title { margin: var(--hd-space-2) 0 0; color: var(--hd-ink); font: 600 var(--hd-text-l) / 1.3 var(--hd-font-heading); }
.hd-page-empty-text { max-inline-size: 42ch; margin: 0; font-size: var(--hd-text-m); line-height: 1.5; }
.hd-widget.hd-unsupported .hd-body::before { content: ""; inline-size: 28px; block-size: 28px; background: var(--hd-ink-faint); -webkit-mask: var(--hd-mask-error) center / contain no-repeat; mask: var(--hd-mask-error) center / contain no-repeat; }
```
- [ ] In `scripts/build-ui-bundles.js` append `'ui/css/homedash-page.css'` after `'ui/css/homedash-widgets.css'` in the homedash entry; `npm run build:ui`.
- [ ] Run `npm run test:homedash` → all bundle and guard cases PASS; `npm run check:ui` → exit 0.
- [ ] Detect-changes; commit `ui/css/homedash-page.css scripts/build-ui-bundles.js scripts/homedash-tests/bundle.test.mjs ui/css/homedash.bundle.css` — `feat(homedash): page geometry as the tenth bundle part`.

**Done when:** bundle reports 10 parts, guard green, token freeze green.

---

### Task 9: hd-render.js, one page of widget hosts

**Files:** create `ui/js/homedash/hd-render.js`, `ui/testdata/homedash-m2/starter-dashboard.json`, `ui/testdata/homedash-m2/system-entities.json`, `scripts/homedash-tests/render.test.mjs`.
**Read first:** `hd-widgets.js` (`host`, api factory, `resize` needs `px`), `hd-layout.js`, `hd-doc.js` `normalise`, R7, `documentation/homedash-visual-system.md` "Host duties" 4–5.

Behaviour rules (code below implements them):
1. Each widget is hosted with an effective copy: options = catalog defaults ∪ document options; when `warn` or `alert` is set and `zones` is not, `zones = E.tone.zones(options)`.
2. `entity(binding)` = store view plus `tone: E.tone.of(view, options)`; `can()` false and `command()` rejects (view only); `history(binding, range)` uses `options.hours` when it is a positive number, else the widget's range, 120 points.
3. A type the engine does not register becomes `div.hd-widget.hd-unsupported` with a frame titled by `widget.title || type` and state text `homedash.view.unsupported`.
4. Layout runs once synchronously at creation and then in one animation frame per stage resize: breakpoint with hysteresis, effective layout with catalog constraints, metrics, page vars, `--hd-x/y/w/h` per widget, `resize({w, h, px})` when cell or pixel size changed.
5. Store change notices update only bound widgets; widgets off screen (IntersectionObserver, 200 px) are marked and updated when they reappear.
6. Every wrapper carries `data-hd-wid="<widget id>"`.

- [ ] Create `ui/testdata/homedash-m2/starter-dashboard.json` = the `dashboard` object of the contract's "Starter template answer" verbatim (id `d_m3x7q2k5a4bc`, six widgets, layouts, kiosk), pretty-printed with two spaces.
- [ ] Create `ui/testdata/homedash-m2/system-entities.json`: an array of the six Entity objects of the contract table "`system` entities" (`source: "system"`, `kind: "sensor"`, capabilities `[{"name":"numeric","allowed":true},{"name":"history","allowed":true}]`, `readonly: true`) with names CPU, Memory, Disk, Network receive, Network send, Uptime; units `%`, `%`, `%`, `Mbit/s`, `Mbit/s`, `s`; `device_class` `data_rate` for both network ids and `duration` for uptime; `icon_hint` `cpu`, `memory`, `disk`, `download`, `upload`, `clock`; states `{"value": V, "available": true, "updated_at": "2026-11-25T10:00:00Z"}` with V = 23.5, 50, 61, 12.4, 3.2, 7215.
- [ ] Write the failing test `scripts/homedash-tests/render.test.mjs`:

```js
import assert from 'node:assert/strict'; import { loadEngine, ENGINE_ORDER, read } from './harness.mjs';
const FILES = ENGINE_ORDER.concat(['hd-tone.js', 'hd-api.js', 'hd-catalog.js', 'hd-render.js']);
const STARTER = JSON.parse(read('ui/testdata/homedash-m2/starter-dashboard.json')), CATALOG = JSON.parse(read('internal/homedash/testdata/widget_catalog.json'));
const V = (id, value) => ({ id, kind: 'sensor', name: id, unit: '%', value, available: true, status: 'ok', readonly: false, stale: false, pending: false, attributes: { updated_at: '2026-11-25T10:00:00Z' } });
function setup(rawDoc) {
  const { E, document } = loadEngine(FILES, { dom: true }), catalog = E.catalog.create(CATALOG), doc = E.doc.normalise(rawDoc || STARTER, { sizeOf: catalog.defaultSize });
  const root = E.el('div', { class: 'hd-root', 'data-hd-family': 'glass' }), stage = E.el('div', { class: 'hd-stage' }); root.appendChild(stage); document.body.appendChild(root);
  const views = new Map(), bus = E.core.emitter(), histories = [], frames = [], layouts = []; let width = 1180;
  const store = { view: id => views.get(id) || null, on: bus.on, off: bus.off, history(id, range, points) { histories.push([id, range, points]); return Promise.resolve({ points: [[1, 2], [2, 3]], source: 'source' }); } };
  const page = E.render.page({ doc, page: doc.pages[0], stage, store, catalog, t: k => k, lang: 'en', raf: fn => { frames.push(fn); return frames.length; }, caf: () => {},
    measure: () => ({ width, height: 700 }), onLayout: m => layouts.push(m) });
  return { E, stage, views, bus, histories, page, layouts, run: () => { while (frames.length) frames.shift()(); }, setWidth: w => { width = w; } };
}
const vars = (el, ...names) => names.map(n => el.style.getPropertyValue(n)).join(',');
export const tests = [
  ['render: one host per widget, placed in cells, sized in pixels, history in the widget hours', () => {
    const { stage, histories, layouts } = setup();
    assert.equal(stage.querySelectorAll('.hd-page > .hd-widget').length, 6);
    const rx = stage.querySelector('[data-hd-wid="net-rx"]'); assert.equal(vars(rx, '--hd-x', '--hd-y', '--hd-w', '--hd-h'), '0,4,5,2');
    assert.equal(rx.querySelector('svg.hd-spark').getAttribute('width'), '473');
    assert.ok(histories.some(h => h[0] === 'system:net_rx' && h[1] === 3600000 && h[2] === 120)); assert.equal(layouts[0].bp, 'lg'); assert.equal(layouts[0].fit, 'scroll'); }],
  ['render: tone from warn/alert reaches the wrapper and zones are drawn; only bound widgets update', () => {
    const { stage, views, bus } = setup(), cpu = stage.querySelector('[data-hd-wid="cpu"]');
    assert.ok(cpu.querySelector('path.hd-gauge-zone[data-tone="crit"]')); assert.ok(cpu.querySelector('path.hd-gauge-zone[data-tone="warn"]'));
    views.set('system:cpu', V('system:cpu', 97)); views.set('system:memory', V('system:memory', 99)); bus.emit('change', new Set(['system:cpu']));
    assert.equal(cpu.getAttribute('data-hd-tone'), 'crit'); assert.equal(stage.querySelector('[data-hd-wid="memory"]').getAttribute('data-hd-tone'), null);
    views.set('system:cpu', V('system:cpu', 85)); bus.emit('change', null); assert.equal(cpu.getAttribute('data-hd-tone'), 'warn'); assert.equal(stage.querySelector('[data-hd-wid="memory"]').getAttribute('data-hd-tone'), 'crit'); }],
  ['render: a type the engine does not register gets a labelled cell; an empty page gets its note', () => {
    const doc = { name: 'X', pages: [{ id: 'main', widgets: [{ id: 'clock', type: 'clock', variant: 'digital', title: 'Clock' }], layouts: { lg: [{ id: 'clock', x: 0, y: 0, w: 2, h: 1 }] } }] };
    const { stage } = setup(doc), cell = stage.querySelector('.hd-unsupported');
    assert.equal(cell.querySelector('.hd-title').textContent, 'Clock'); assert.equal(cell.querySelector('.hd-state-text').textContent, 'homedash.view.unsupported');
    const empty = setup({ name: 'E', pages: [{ id: 'main', widgets: [], layouts: { lg: [] } }] });
    assert.equal(empty.stage.querySelector('.hd-page-empty .hd-page-empty-title').textContent, 'homedash.view.empty_title'); }],
  ['render: breakpoints follow the stage width with derived, constrained layouts; destroy cleans up', () => {
    const { stage, page, run, setWidth, layouts, bus } = setup();
    setWidth(500); page.layout(); run(); assert.equal(layouts[layouts.length - 1].bp, 'sm');
    assert.equal(stage.querySelector('[data-hd-wid="cpu"]').style.getPropertyValue('--hd-w'), '2');
    page.destroy(); assert.equal(stage.querySelectorAll('.hd-widget').length, 0); bus.emit('change', null); }]
];
```

- [ ] Run `npm run test:homedash` → four render cases FAIL.
- [ ] Implement `ui/js/homedash/hd-render.js`:

```js
// HomeDash page renderer (F2; design-frontend 2.3, 4.4, 9.1, 10). page(opts) renders one page of a normalised dashboard into the stage:
// div.hd-page with one widget host per widget, placed by --hd-x/--hd-y/--hd-w/--hd-h (homedash-page.css). Rules: options = catalog defaults
// under the document's; warn/alert without zones get E.tone.zones; entity() adds the host tone (E.tone.of); can() is false and command()
// rejects (view only in M2); history() honours options.hours. Unregistered types become a labelled .hd-unsupported cell. Layout runs once at
// creation and then in one animation frame per stage resize; store notices update only bound widgets; off-screen widgets update when shown.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.widgets || !E.layout || !E.tone || !E.api || !E.prim) throw new Error('hd-render.js: load the engine, hd-tone.js and hd-api.js first');
    const { el, setVars, setAttr } = E.core, ENTITY_ID = /^[a-z][a-z0-9_]*:\S+$/;
    const isEntityId = v => typeof v === 'string' && ENTITY_ID.test(v);
    const viewOnly = () => Promise.reject(E.api.normalizeError({ status: 0, body: { code: 'view_only', message: 'HomeDash M2 sends no commands' } }));
    function page(opts) {
        const o = opts, doc = o.doc, pg = o.page, store = o.store, catalog = o.catalog, t = o.t, lang = o.lang || 'en', family = doc.look && doc.look.family;
        const measure = o.measure || (node => ({ width: node.clientWidth, height: node.clientHeight }));
        const pageEl = el('div', { class: 'hd-page', id: o.id || null, role: o.role || 'region', 'aria-label': o.label || null, 'aria-labelledby': o.labelledBy || null });
        o.stage.appendChild(pageEl);
        const recs = new Map(), byEntity = new Map(), byEl = new Map(), limits = new Map();
        let bp = null, destroyed = false;
        const lgItem = id => (pg.layouts.lg || []).find(i => i.id === id) || { w: 1, h: 1 };
        function api(rec) {
            const idOf = binding => { const id = rec.widget.bindings && rec.widget.bindings[binding]; return isEntityId(id) ? id : null; };
            return () => ({
                entity(binding) { const id = idOf(binding), v = id ? store.view(id) : null; return v ? Object.assign({}, v, { tone: E.tone.of(v, rec.widget.options) }) : null; },
                can: () => false, command: viewOnly, moreInfo() {},
                history(binding, range) {
                    const id = idOf(binding); if (!id) return Promise.resolve({ points: [], source: 'none' });
                    const hours = Number(rec.widget.options.hours);
                    return store.history(id, Number.isFinite(hours) && hours > 0 ? hours * 3600000 : range, 120);
                },
                announce(text) { if (typeof o.announce === 'function') o.announce(String(text)); }
            });
        }
        function unsupported(widget) {
            const frame = E.prim.frame({ title: widget.title || widget.type });
            frame.set({ stateText: t('homedash.view.unsupported') });
            return el('div', { class: 'hd-widget hd-unsupported', 'data-hd-type': widget.type, 'data-hd-variant': widget.variant || null, 'data-hd-family': widget.family || family || null,
                'data-hd-status': 'unknown', role: 'group', 'aria-labelledby': frame.titleId }, frame.el);
        }
        function mount(widget) {
            const options = Object.assign({}, catalog.options(widget));
            if ((options.warn != null || options.alert != null) && !Array.isArray(options.zones)) options.zones = E.tone.zones(options);
            const rec = { widget: Object.assign({}, widget, { options }), ids: Object.values(widget.bindings || {}).filter(isEntityId), inst: null, el: null, cell: '', px: '', visible: true, dirty: false };
            limits.set(widget.id, catalog.constraints(widget));
            const item = lgItem(widget.id);
            try { rec.inst = E.widgets.host({ widget: rec.widget, mode: 'view', size: { w: item.w, h: item.h }, t, lang, parent: pageEl, api: api(rec) }); rec.el = rec.inst.el; }
            catch (err) { console.warn('HomeDash: widget ' + widget.id + ' is not rendered', err); rec.inst = null; rec.el = unsupported(widget); pageEl.appendChild(rec.el); }
            setAttr(rec.el, 'data-hd-wid', widget.id);
            recs.set(widget.id, rec); byEl.set(rec.el, rec);
            for (const id of rec.ids) { if (!byEntity.has(id)) byEntity.set(id, new Set()); byEntity.get(id).add(rec); }
        }
        for (const w of pg.widgets) mount(w);
        if (!pg.widgets.length) pageEl.appendChild(el('div', { class: 'hd-page-empty', role: 'note' }, [E.icons.icon('dashboard'),
            el('p', { class: 'hd-page-empty-title', text: t('homedash.view.empty_title') }), el('p', { class: 'hd-page-empty-text', text: t('homedash.view.empty_text') })]));
        function layout() {
            if (destroyed) return;
            const box = measure(o.stage);
            if (!box || !(box.width > 0)) return;
            bp = E.layout.breakpoint(box.width, bp);
            const cols = E.layout.COLS[bp], items = E.layout.effective(pg.layouts, bp, id => limits.get(id)), rows = Math.max(1, E.layout.rows(items));
            const gap = E.layout.gapFor(doc.look.density, doc.grid.gap), m = E.layout.metrics(box.width, box.height || 0, cols, rows, gap, doc.grid.row_unit, doc.grid.fit);
            setVars(pageEl, E.layout.cssVars(m, cols, rows, gap));
            if (typeof o.onLayout === 'function') o.onLayout({ bp, fit: m.mode });
            for (const it of items) {
                const rec = recs.get(it.id); if (!rec) continue;
                const cell = [it.x, it.y, it.w, it.h].join(',');
                if (cell !== rec.cell) { rec.cell = cell; setVars(rec.el, { '--hd-x': it.x, '--hd-y': it.y, '--hd-w': it.w, '--hd-h': it.h }); }
                const px = { width: Math.round(it.w * m.colW + (it.w - 1) * gap), height: Math.round(it.h * m.rowH + (it.h - 1) * gap) }, key = cell + ':' + px.width + 'x' + px.height;
                if (rec.inst && key !== rec.px) { rec.px = key; rec.inst.resize({ w: it.w, h: it.h, px }); }
            }
        }
        const schedule = E.core.frame(layout, { raf: o.raf, caf: o.caf });
        function onChange(ids) {
            if (destroyed) return;
            const targets = new Set();
            if (ids === null) for (const r of recs.values()) targets.add(r); else for (const id of ids) for (const r of byEntity.get(id) || []) targets.add(r);
            for (const r of targets) { if (!r.inst) continue; if (r.visible) r.inst.update(); else r.dirty = true; }
        }
        store.on('change', onChange);
        const IO = o.IntersectionObserver || (typeof IntersectionObserver === 'function' ? IntersectionObserver : null);
        const io = IO ? new IO(entries => { for (const en of entries) { const r = byEl.get(en.target); if (!r) continue; r.visible = !!en.isIntersecting; if (r.visible && r.dirty && r.inst) { r.dirty = false; r.inst.update(); } } }, { root: o.stage, rootMargin: '200px' }) : null;
        if (io) for (const r of recs.values()) io.observe(r.el);
        const RO = o.ResizeObserver || (typeof ResizeObserver === 'function' ? ResizeObserver : null);
        const ro = RO ? new RO(() => schedule()) : null;
        if (ro) ro.observe(o.stage);
        layout();
        function destroy() {
            if (destroyed) return; destroyed = true; schedule.cancel(); store.off('change', onChange);
            if (ro) ro.disconnect(); if (io) io.disconnect();
            for (const r of recs.values()) if (r.inst) r.inst.destroy();
            pageEl.remove(); recs.clear(); byEntity.clear(); byEl.clear();
        }
        return Object.freeze({ el: pageEl, entityIds: () => [...byEntity.keys()], layout: () => schedule(), refreshAll: () => onChange(null), destroy });
    }
    E.render = Object.freeze({ page, isEntityId });
})();
```

- [ ] Run `npm run test:homedash` → four render cases PASS.
- [ ] Detect-changes; commit `ui/js/homedash/hd-render.js ui/testdata/homedash-m2/starter-dashboard.json ui/testdata/homedash-m2/system-entities.json scripts/homedash-tests/render.test.mjs` — `feat(homedash): page renderer with host tones and catalog layout`.

**Done when:** four PASS; starter renders six hosts with zones and correct pixel sizes.

---

### Task 10: hd-sheet.js, the in-root modal

**Files:** create `ui/js/homedash/hd-sheet.js`, `scripts/homedash-tests/sheet.test.mjs`.
**Read first:** `hd-prim-displays.js:271-282` (`E.prim.sheet` markup: `.hd-sheet-layer[hidden] > .hd-sheet[role=dialog][aria-modal] > .hd-sheet-head > h2.hd-sheet-title`, `.hd-sheet-body`, `.hd-sheet-foot`); `css-lint.mjs` `JS_RULES` (no bare `confirm(`).

- [ ] Write the failing test `scripts/homedash-tests/sheet.test.mjs`:

```js
import assert from 'node:assert/strict'; import { loadEngine, ENGINE_ORDER } from './harness.mjs';
function setup() {
  const { E, document } = loadEngine(ENGINE_ORDER.concat(['hd-sheet.js']), { dom: true });
  const box = document.createElement('div'), other = document.createElement('main'), opener = document.createElement('button');
  box.appendChild(other); other.appendChild(opener); document.body.appendChild(box); opener.focus();
  return { E, document, box, other, opener, sheet: E.sheet.create(box) };
}
const key = (k, extra) => Object.assign({ type: 'keydown', key: k, shiftKey: false, preventDefault() {}, stopPropagation() {} }, extra);
export const tests = [
  ['sheet: open shows a labelled dialog, makes the rest inert, Escape resolves null and restores the focus', async () => {
    const { box, other, opener, sheet, document } = setup();
    const done = sheet.open({ title: 'Delete dashboard?', text: ['<b>System</b> goes away.'], actions: [{ id: 'cancel', label: 'Cancel' }, { id: 'ok', label: 'Delete', tone: 'danger' }] });
    const layer = box.querySelector('.hd-sheet-layer'), dlg = layer.querySelector('.hd-sheet');
    assert.equal(layer.hasAttribute('hidden'), false); assert.equal(dlg.getAttribute('role'), 'dialog'); assert.equal(dlg.getAttribute('aria-modal'), 'true');
    assert.equal(layer.querySelector('.hd-sheet-title').textContent, 'Delete dashboard?'); assert.equal(layer.querySelector('.hd-sheet-text').textContent, '<b>System</b> goes away.');
    assert.equal(layer.querySelector('b'), null); assert.equal(other.getAttribute('inert'), ''); assert.equal(document.activeElement.getAttribute('data-action'), 'cancel');
    layer.dispatchEvent(key('Escape')); assert.equal(await done, null);
    assert.equal(layer.hasAttribute('hidden'), true); assert.equal(other.hasAttribute('inert'), false); assert.equal(document.activeElement, opener); }],
  ['sheet: a button resolves its id, confirm() gives a boolean, Tab stays inside', async () => {
    const { sheet, box, document } = setup();
    const yes = sheet.confirm({ title: 'T', text: 'x', confirmLabel: 'Delete', cancelLabel: 'Cancel', tone: 'danger' });
    const layer = box.querySelector('.hd-sheet-layer'), [cancel, ok] = layer.querySelectorAll('.hd-sheet-btn');
    assert.equal(document.activeElement, cancel); assert.equal(ok.getAttribute('data-tone'), 'danger');
    layer.dispatchEvent(key('Tab')); assert.equal(document.activeElement, ok); layer.dispatchEvent(key('Tab')); assert.equal(document.activeElement, cancel);
    layer.dispatchEvent(key('Tab', { shiftKey: true })); assert.equal(document.activeElement, ok);
    layer.dispatchEvent({ type: 'click', target: ok }); assert.equal(await yes, true); }]
];
```

- [ ] Run → FAIL. Implement `ui/js/homedash/hd-sheet.js`:

```js
// HomeDash in-root modal sheet (F2; design-frontend 6). create(container) builds one E.prim.sheet layer inside `container`.
// open({title, text: [paragraphs], actions: [{id, label, tone, autofocus}], kind}) resolves the chosen action id, or null for Escape, a click
// on the backdrop or close(). While open, the container's other children are inert, Tab and Shift+Tab stay in the dialog, and focus returns to
// the element that had it. confirm({title, text, confirmLabel, cancelLabel, tone}) resolves true or false; a danger confirm starts on Cancel.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.prim || !E.prim.sheet) throw new Error('hd-sheet.js: load hd-core.js and hd-prim-displays.js first');
    const { el, setAttr } = E.core, FOCUSABLE = 'button, [href], input, select, textarea, [tabindex]';
    function create(container) {
        const doc = container.ownerDocument, sheet = E.prim.sheet({ kind: 'confirm' }), life = E.core.bag(), dialog = sheet.el.querySelector('.hd-sheet');
        container.appendChild(sheet.el);
        let resolveOpen = null, opener = null, inerted = [];
        const focusables = () => Array.from(sheet.el.querySelectorAll(FOCUSABLE)).filter(n => !n.hasAttribute('disabled') && n.getAttribute('tabindex') !== '-1');
        function close(value) {
            if (!resolveOpen) return;
            const done = resolveOpen; resolveOpen = null;
            sheet.set({ open: false });
            for (const n of inerted) n.removeAttribute('inert'); inerted = [];
            sheet.foot.replaceChildren(); sheet.body.replaceChildren();
            if (opener && typeof opener.focus === 'function' && opener.isConnected !== false) opener.focus();
            opener = null; done(value === undefined ? null : value);
        }
        function open(spec) {
            close(null);
            const s = spec || {};
            sheet.set({ open: true, title: s.title || '' }); setAttr(dialog, 'data-kind', s.kind || 'confirm');
            for (const line of [].concat(s.text || [])) sheet.body.appendChild(el('p', { class: 'hd-sheet-text', text: line }));
            let first = null;
            for (const a of s.actions || []) {
                const b = el('button', { type: 'button', class: 'hd-sheet-btn', 'data-action': a.id, 'data-tone': a.tone || 'default', text: a.label });
                sheet.foot.appendChild(b); if (a.autofocus || !first) first = b;
            }
            opener = doc.activeElement || null;
            inerted = Array.from(container.children).filter(n => n !== sheet.el && !n.hasAttribute('inert'));
            for (const n of inerted) n.setAttribute('inert', '');
            return new Promise(resolve => { resolveOpen = resolve; if (first) first.focus(); });
        }
        life.on(sheet.el, 'click', ev => {
            const b = ev.target && ev.target.closest ? ev.target.closest('.hd-sheet-btn') : null;
            if (b) close(b.getAttribute('data-action')); else if (ev.target === sheet.el) close(null);
        });
        life.on(sheet.el, 'keydown', ev => {
            if (!resolveOpen) return;
            if (ev.key === 'Escape') { ev.preventDefault(); ev.stopPropagation(); close(null); return; }
            if (ev.key !== 'Tab') return;
            const list = focusables(); if (!list.length) return;
            const i = list.indexOf(doc.activeElement), next = ev.shiftKey ? (i <= 0 ? list.length - 1 : i - 1) : (i === list.length - 1 ? 0 : i + 1);
            ev.preventDefault(); list[next].focus();
        });
        return Object.freeze({
            el: sheet.el, open, close, isOpen: () => !!resolveOpen,
            confirm: spec => open({ title: spec.title, text: spec.text, kind: 'confirm', actions: [{ id: 'cancel', label: spec.cancelLabel, tone: 'default' },
                { id: 'confirm', label: spec.confirmLabel, tone: spec.tone || 'primary', autofocus: spec.tone !== 'danger' }] }).then(v => v === 'confirm'),
            destroy() { close(null); life.dispose(); sheet.destroy(); }
        });
    }
    E.sheet = Object.freeze({ create });
})();
```

- [ ] Run → two sheet cases PASS. Detect-changes; commit `ui/js/homedash/hd-sheet.js scripts/homedash-tests/sheet.test.mjs` — `feat(homedash): in-root modal sheet with focus trap`.

**Done when:** both PASS; no bare `confirm(`/`alert(` in the file (guard green).

---

### Task 11: hd-view.js, one dashboard root

**Files:** create `ui/js/homedash/hd-view.js`, `scripts/homedash-tests/view.test.mjs`.
**Read first:** design-frontend 6 (tabs, banners), 9.1 (root attributes); `documentation/homedash-visual-system.md` "Host duties" 2, 3, 6, 7; `hd-colour.js` `solvePalette`, `applyPalette`, `clearPalette`.

Markup inside the given `.hd-root` (in this order): `div.hd-tabs[role=tablist][aria-label=t('homedash.view.pages')]` (hidden unless the document has more than one page) with `button.hd-tab[role=tab][id=<view>-tab-<page>][data-page][aria-selected][aria-controls=<view>-page][tabindex=0|-1]`; `div.hd-stage` holding the hd-render page (`role=tabpanel` + `aria-labelledby` with tabs, else `role=region` + `aria-label` = dashboard name); `div.hd-live.hd-sr-only[aria-live=polite]`.

Rules: 1. `show(doc, pageId)` normalises with `E.doc.normalise(doc, {sizeOf: catalog.defaultSize})`, keeps the current page when `pageId` is absent and the page still exists. 2. Root attributes before rendering: `data-hd-family`, `data-hd-palette`, `data-hd-scheme` (palette `light`/`dark` win; `custom` uses `palette.scheme`; `theme` uses the host's `scheme()`), `data-hd-density`, `data-hd-backdrop`, `data-hd-motion` (host `motion()`), `--hd-radius-scale` inline only when ≠ 1; custom palettes through `E.colour.applyPalette(root, solvePalette(palette).tokens)`, else `clearPalette`. Never write `--hd-accent`, `--hd-light-on`, `--hd-focus`, `--hd-ink-on-accent`, `--hd-hud` or `data-hd-corners`. 3. After every render and every `refreshLook()`: `E.widgets.applyLook(root)`. 4. Tabs: click selects; ArrowLeft/ArrowRight wrap, Home/End jump, each moves focus and activates; `onPage(pageId)` after every switch. 5. `setConnection(c)` writes `data-hd-conn` = `live` | `offline` | `connecting`; `setReadonly(f)` writes `data-hd-readonly`. 6. `banners(state)` (pure): at most two, in priority order: `auth_lost` (danger, alert, any mode); then in dashboard mode only `offline` (warn, status, action `retry`), one `source_unavailable`/`source_degraded` per affected source (warn), `schema_too_new` (info), `readonly` (info).

- [ ] Write the failing test `scripts/homedash-tests/view.test.mjs`:

```js
import assert from 'node:assert/strict'; import { loadEngine, ENGINE_ORDER, read, eq } from './harness.mjs';
const FILES = ENGINE_ORDER.concat(['hd-tone.js', 'hd-api.js', 'hd-catalog.js', 'hd-render.js', 'hd-view.js']), CATALOG = JSON.parse(read('internal/homedash/testdata/widget_catalog.json'));
const TWO = { id: 'd_aaaaaaaaaaab', name: 'Two', look: { family: 'minimal', palette: { mode: 'theme' } }, pages: [
  { id: 'main', name: 'One', widgets: [{ id: 'w1', type: 'entity', variant: 'tile', bindings: { primary: 'system:uptime' } }], layouts: { lg: [{ id: 'w1', x: 0, y: 0, w: 2, h: 1 }] } },
  { id: 'p2', name: 'Two', widgets: [{ id: 'w2', type: 'entity', variant: 'tile', bindings: { primary: 'system:cpu' } }], layouts: { lg: [] } }] };
const key = k => ({ type: 'keydown', key: k, preventDefault() {} });
export const tests = [
  ['view: banners pick at most two by priority', () => { const b = loadEngine(FILES).E.view.banners;
    eq(b({ mode: 'dashboard', connection: { phase: 'down', offline: true }, sources: [{ source: 'system', state: 'degraded' }], desktopReadonly: true }).map(x => x.kind), ['offline', 'source_degraded']);
    eq(b({ mode: 'dashboard', connection: { phase: 'live' }, sources: [], desktopReadonly: true, schemaTooNew: true }).map(x => x.kind), ['schema_too_new', 'readonly']);
    eq(b({ mode: 'home', connection: { phase: 'down', offline: true }, sources: [{ source: 'system', state: 'unavailable' }], desktopReadonly: true }).map(x => x.kind), []);
    eq(b({ mode: 'home', connection: { phase: 'stopped', reason: 'auth_lost' } }).map(x => x.kind), ['auth_lost']);
    assert.equal(b({ mode: 'dashboard', connection: { offline: true } })[0].action, 'retry'); }],
  ['view: root attributes follow the look and the host; tabs switch pages; connection and read-only attributes', () => {
    const { E, document } = loadEngine(FILES, { dom: true }), root = E.el('div', { class: 'hd-root' }); document.body.appendChild(root);
    const bus = E.core.emitter(), pages = [], store = { view: () => null, on: bus.on, off: bus.off, history: () => Promise.resolve({ points: [], source: 'none' }) };
    const view = E.view.create({ root, store, catalog: E.catalog.create(CATALOG), t: k => k, lang: 'en', host: 'desktop', scheme: () => 'light', motion: () => 'reduced',
      raf: fn => 1, caf: () => {}, measure: () => ({ width: 1180, height: 700 }), onPage: p => pages.push(p) });
    view.show(TWO);
    for (const [k, v] of [['data-hd-family', 'minimal'], ['data-hd-palette', 'theme'], ['data-hd-scheme', 'light'], ['data-hd-motion', 'reduced'], ['data-hd-density', 'comfortable'], ['data-hd-backdrop', 'theme'], ['data-hd-bp', 'lg']]) assert.equal(root.getAttribute(k), v, k);
    const tabs = root.querySelectorAll('.hd-tab'); assert.equal(tabs.length, 2); assert.equal(tabs[0].getAttribute('aria-selected'), 'true');
    const panel = root.querySelector('.hd-page'); assert.equal(panel.getAttribute('role'), 'tabpanel'); assert.equal(panel.getAttribute('aria-labelledby'), tabs[0].getAttribute('id'));
    root.querySelector('.hd-tabs').dispatchEvent(key('ArrowRight')); eq(pages, ['p2']); assert.equal(view.pageId(), 'p2');
    assert.ok(root.querySelector('[data-hd-wid="w2"]')); assert.equal(root.querySelector('[data-hd-wid="w1"]'), null); eq(view.entityIds(), ['system:cpu']);
    view.setConnection({ phase: 'down', offline: true }); assert.equal(root.getAttribute('data-hd-conn'), 'offline');
    view.setConnection({ phase: 'connecting', offline: false }); assert.equal(root.getAttribute('data-hd-conn'), 'connecting');
    view.setReadonly(true); assert.equal(root.getAttribute('data-hd-readonly'), 'true');
    view.show(Object.assign({}, TWO, { look: { family: 'glass', palette: { mode: 'dark' } } })); assert.equal(root.getAttribute('data-hd-scheme'), 'dark'); assert.equal(view.pageId(), 'p2');
    view.destroy(); assert.equal(root.children.length, 0); }]
];
```

- [ ] Run → FAIL. Implement `ui/js/homedash/hd-view.js`:

```js
// HomeDash dashboard view (F2; design-frontend 6, 9.1). create(opts) owns the content of one .hd-root: root attributes from the document look
// and the host (scheme, motion), page tabs, the scroll stage with one hd-render page, the live region, and E.widgets.applyLook after every
// render and look change. banners(state) is the pure banner list the hosts draw (at most two, by priority). View mode only in M2.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.render || !E.doc || !E.colour || !E.widgets) throw new Error('hd-view.js: load hd-render.js and the engine first');
    const { el, setAttr, setVars, uid } = E.core;
    function banners(s) {
        const x = s || {}, c = x.connection || {}, dash = x.mode === 'dashboard', out = [];
        if (c.phase === 'stopped' && c.reason === 'auth_lost') out.push({ id: 'auth_lost', kind: 'auth_lost', tone: 'danger', role: 'alert' });
        else if (dash && c.offline) out.push({ id: 'offline', kind: 'offline', tone: 'warn', role: 'status', action: 'retry' });
        if (dash) for (const src of Array.isArray(x.sources) ? x.sources : []) {
            if (!src || typeof src.source !== 'string') continue;
            if (src.state === 'unavailable') out.push({ id: 'source:' + src.source, kind: 'source_unavailable', tone: 'warn', role: 'status', source: src.source });
            else if (src.state === 'degraded') out.push({ id: 'source:' + src.source, kind: 'source_degraded', tone: 'warn', role: 'status', source: src.source });
        }
        if (dash && x.schemaTooNew) out.push({ id: 'schema_too_new', kind: 'schema_too_new', tone: 'info', role: 'status' });
        if (dash && x.desktopReadonly) out.push({ id: 'readonly', kind: 'readonly', tone: 'info', role: 'status' });
        return out.slice(0, 2);
    }
    function create(opts) {
        const o = opts, root = o.root, t = o.t, id = uid('hd-view'), life = E.core.bag();
        const tabs = el('div', { class: 'hd-tabs', role: 'tablist', 'aria-label': t('homedash.view.pages'), hidden: 'true' });
        const stage = el('div', { class: 'hd-stage' }), live = el('div', { class: 'hd-live hd-sr-only', 'aria-live': 'polite' });
        root.replaceChildren(tabs, stage, live);
        let doc = null, page = null, pageId = null;
        const scheme = () => (typeof o.scheme === 'function' && o.scheme() === 'light' ? 'light' : 'dark');
        const motion = () => (typeof o.motion === 'function' && o.motion() === 'reduced' ? 'reduced' : 'full');
        function applyRoot() {
            const look = doc.look, mode = look.palette.mode;
            const sch = mode === 'light' || mode === 'dark' ? mode : mode === 'custom' ? (look.palette.scheme === 'light' ? 'light' : 'dark') : scheme();
            const attrs = { 'data-hd-family': look.family, 'data-hd-palette': mode, 'data-hd-scheme': sch, 'data-hd-density': look.density, 'data-hd-backdrop': (look.backdrop && look.backdrop.kind) || 'theme', 'data-hd-motion': motion() };
            for (const [k, v] of Object.entries(attrs)) setAttr(root, k, v);
            setVars(root, { '--hd-radius-scale': look.radius_scale === 1 ? null : look.radius_scale });
            if (mode === 'custom') E.colour.applyPalette(root, E.colour.solvePalette(look.palette).tokens); else E.colour.clearPalette(root);
        }
        function renderTabs() {
            const many = doc.pages.length > 1; setAttr(tabs, 'hidden', !many); tabs.replaceChildren();
            if (!many) return;
            for (const p of doc.pages) { const on = p.id === pageId;
                tabs.appendChild(el('button', { type: 'button', class: 'hd-tab', role: 'tab', id: id + '-tab-' + p.id, 'data-page': p.id, 'aria-selected': on ? 'true' : 'false',
                    'aria-controls': id + '-page', tabindex: on ? '0' : '-1', text: p.name || t('homedash.home.page_main') })); }
        }
        function renderPage() {
            if (page) page.destroy();
            const pg = doc.pages.find(p => p.id === pageId), many = doc.pages.length > 1;
            page = E.render.page({ doc, page: pg, stage, store: o.store, catalog: o.catalog, t, lang: o.lang, id: id + '-page', role: many ? 'tabpanel' : 'region',
                labelledBy: many ? id + '-tab-' + pg.id : null, label: many ? null : doc.name, raf: o.raf, caf: o.caf, measure: o.measure,
                announce: text => { live.textContent = text; }, onLayout: m => { setAttr(root, 'data-hd-bp', m.bp); setAttr(root, 'data-hd-fit', m.fit); } });
        }
        function select(next, focus) {
            if (!doc || next === pageId || !doc.pages.some(p => p.id === next)) return;
            pageId = next; renderTabs(); renderPage(); E.widgets.applyLook(root);
            if (focus) { const tab = Array.from(tabs.children).find(b => b.getAttribute('data-page') === next); if (tab) tab.focus(); }
            if (typeof o.onPage === 'function') o.onPage(pageId);
        }
        life.on(tabs, 'click', ev => { const b = ev.target && ev.target.closest ? ev.target.closest('.hd-tab') : null; if (b) select(b.getAttribute('data-page'), false); });
        life.on(tabs, 'keydown', ev => {
            if (!doc) return;
            const ids = doc.pages.map(p => p.id), i = ids.indexOf(pageId);
            const next = ev.key === 'ArrowRight' ? ids[(i + 1) % ids.length] : ev.key === 'ArrowLeft' ? ids[(i - 1 + ids.length) % ids.length] : ev.key === 'Home' ? ids[0] : ev.key === 'End' ? ids[ids.length - 1] : null;
            if (next === null) return; ev.preventDefault(); select(next, true);
        });
        return Object.freeze({
            show(raw, wantPage) {
                doc = E.doc.normalise(raw, { sizeOf: o.catalog.defaultSize });
                pageId = doc.pages.some(p => p.id === wantPage) ? wantPage : doc.pages.some(p => p.id === pageId) ? pageId : doc.pages[0].id;
                applyRoot(); renderTabs(); renderPage(); E.widgets.applyLook(root);
            },
            pageId: () => pageId, entityIds: () => (page ? page.entityIds() : []),
            setConnection(c) { setAttr(root, 'data-hd-conn', c && c.phase === 'live' ? 'live' : c && c.offline ? 'offline' : 'connecting'); },
            setReadonly(flag) { setAttr(root, 'data-hd-readonly', !!flag); },
            refreshLook() { if (!doc) return; applyRoot(); E.widgets.applyLook(root); if (page) page.layout(); },
            destroy() { life.dispose(); if (page) page.destroy(); page = null; doc = null; root.replaceChildren(); }
        });
    }
    E.view = Object.freeze({ create, banners });
})();
```

- [ ] Run → two view cases PASS. Detect-changes; commit `ui/js/homedash/hd-view.js scripts/homedash-tests/view.test.mjs` — `feat(homedash): dashboard view root, tabs and banner list`.

**Done when:** both PASS; the view never writes the five `applyLook`-owned tokens.

---

### Task 12: hd-app.js (`E.mount`) and the test harness additions

**Files:** create `ui/js/homedash/hd-app.js`, `scripts/homedash-tests/app.test.mjs`; modify `scripts/homedash-tests/harness.mjs` (add `APP_ORDER`, `loadFiles`).
**Read first:** design-frontend 1.4 (mount contract), 8.4 (multiple windows), contract `dashboard_changed` and POST `dashboards` forms.

State object (frozen, emitted as `'state'`): `{mode: loading|home|dashboard|disabled|error, error: {code, status}|null, notice: dashboard_deleted|dashboard_missing|open_failed|list_failed|limit|create_failed|readonly|delete_conflict|delete_failed|null, busy, dashboards: [summary]|null, dashboard: {id, name, revision, savedBy, schemaTooNew}|null, pageId, connection, desktopReadonly, banners}`.

- [ ] GitNexus: `harness.mjs` exports are used by every test file; additive change.
- [ ] Append to `scripts/homedash-tests/harness.mjs`:

```js
export const APP_ORDER = ENGINE_ORDER.concat(['hd-tone.js', 'hd-api.js', 'hd-stream.js', 'hd-store.js', 'hd-catalog.js', 'hd-render.js', 'hd-sheet.js', 'hd-view.js', 'hd-app.js']);
// Like loadEngine, but for any repository-relative files (desktop host scripts after the engine).
export function loadFiles(relPaths, { dom = false } = {}) {
  const window = {};
  const context = { window, console, setTimeout, clearTimeout, AbortController, Intl, Map, Set, JSON, Math, Date, URL };
  if (dom) { context.document = createDocument(); window.document = context.document; }
  vm.createContext(context);
  for (const f of relPaths) vm.runInContext(read(f), context, { filename: f });
  return { E: window.HomeDashEngine, window, document: context.document };
}
```
- [ ] Write the failing test `scripts/homedash-tests/app.test.mjs`:

```js
import assert from 'node:assert/strict'; import { loadEngine, APP_ORDER, read, eq } from './harness.mjs'; import { fakeClock, fakeSources, HELLO } from './live-fakes.mjs';
const CATALOG = JSON.parse(read('internal/homedash/testdata/widget_catalog.json')), STARTER = JSON.parse(read('ui/testdata/homedash-m2/starter-dashboard.json'));
const ENTITIES = JSON.parse(read('ui/testdata/homedash-m2/system-entities.json'));
function contractServer(options) {
  const o = options || {}, docs = new Map(), calls = [];
  const reply = body => Promise.resolve(JSON.parse(JSON.stringify(body))), fail = (status, body) => Promise.reject(Object.assign(new Error('HTTP ' + status), { status, body }));
  function request(url, init) {
    const u = new URL(url, 'http://desk'), path = u.pathname.replace('/api/desktop/homedash/', ''), method = (init && init.method) || 'GET', body = init && init.body ? JSON.parse(init.body) : null;
    calls.push({ method, path, body });
    if (o.disabled) return fail(503, { status: 'error', code: 'homedash_disabled', message: 'off' });
    if (method === 'GET' && path === 'bootstrap') return reply({ status: 'ok', catalog_hash: CATALOG.hash, desktop_readonly: false, sources: [{ source: 'system', state: 'idle', at: 'x' }], templates: ['system'] });
    if (method === 'GET' && path === 'widget-catalog') return reply({ status: 'ok', catalog: CATALOG });
    if (method === 'GET' && path === 'dashboards') return reply({ status: 'ok', dashboards: [...docs.values()].map(d => ({ id: d.id, slug: d.slug, name: d.name, revision: d.revision, approved_revision: d.revision, updated_at: d.updated_at, saved_by: 'user' })) });
    if (method === 'POST' && path === 'dashboards') { const d = Object.assign(JSON.parse(JSON.stringify(STARTER)), { name: body.name }); docs.set(d.id, d); return reply({ status: 'ok', dashboard: d, warnings: [] }); }
    if (method === 'GET' && path === 'dashboard') { const d = docs.get(u.searchParams.get('id')); return d ? reply({ status: 'ok', dashboard: d, schema_too_new: false }) : fail(404, { status: 'error', code: 'dashboard_not_found', message: 'gone' }); }
    if (method === 'PUT' && path === 'stream/subscriptions') return reply({ status: 'ok', accepted: body.entities, rejected: [] });
    if (method === 'GET' && path === 'history') return reply({ status: 'ok', id: u.searchParams.get('id'), origin: 'source', points: [] });
    return fail(404, { status: 'error', code: 'not_found', message: path });
  }
  return { request, calls, docs };
}
function mountApp(options) {
  const o = options || {}, { E, document } = loadEngine(APP_ORDER, { dom: true }), clock = fakeClock(), es = fakeSources(), server = contractServer(o);
  const host = document.createElement('div'); document.body.appendChild(host);
  const inst = E.mount(host, { transport: { request: server.request, eventSource: es.open }, t: k => k, lang: 'en', measure: () => ({ width: 1180, height: 700 }), visibility: o.visibility,
    timers: { setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now, random: () => 0.5, raf: fn => clock.setTimeout(fn, 16), caf: id => clock.clearTimeout(id) } });
  return { inst, host, clock, es, server };
}
export const tests = [
  ['app: first run shows the empty home; the starter opens live with six widgets on the shared stream', async () => {
    const { inst, host, clock, es, server } = mountApp(); await clock.advance(20);
    assert.equal(inst.state().mode, 'home'); eq(inst.state().dashboards, []); assert.equal(es.list.length, 1);
    inst.createStarter(); await clock.advance(20);
    eq(server.calls.find(c => c.method === 'POST').body, { template: 'system', name: 'homedash.home.starter_name' });
    assert.equal(inst.state().mode, 'dashboard'); assert.equal(inst.state().dashboard.name, 'homedash.home.starter_name');
    assert.equal(host.querySelectorAll('.hd-page > .hd-widget').length, 6); assert.equal(host.querySelector('.hd-root').getAttribute('data-hd-family'), 'glass');
    es.last().emit('hello', HELLO); await clock.advance(20);
    const puts = server.calls.filter(c => c.method === 'PUT'); assert.equal(puts.length, 1);
    eq(puts[0].body.entities.slice().sort(), ['system:cpu', 'system:disk', 'system:memory', 'system:net_rx', 'system:net_tx', 'system:uptime']);
    es.last().emit('snapshot', { entities: ENTITIES, sources: [] }); await clock.advance(40);
    assert.equal(host.querySelector('[data-hd-wid="memory"] [aria-valuenow]').getAttribute('aria-valuenow'), '50');
    es.last().emit('state', { changes: { 'system:cpu': { value: 97, available: true, updated_at: '2026-11-25T10:00:10Z' } } }); await clock.advance(40);
    assert.equal(host.querySelector('[data-hd-wid="cpu"]').getAttribute('data-hd-tone'), 'crit'); inst.dispose(); }],
  ['app: dashboard_changed reloads a newer revision and goes home when the dashboard is gone', async () => {
    const { inst, host, clock, es, server } = mountApp(); await clock.advance(20); inst.createStarter(); await clock.advance(20); es.last().emit('hello', HELLO); await clock.advance(20);
    const d = server.docs.get('d_m3x7q2k5a4bc'); d.revision = 2; d.look.family = 'tactile';
    es.last().emit('dashboard_changed', { id: d.id, revision: 2, approved_revision: 2 }); await clock.advance(300);
    assert.equal(host.querySelector('.hd-root').getAttribute('data-hd-family'), 'tactile'); assert.equal(inst.state().dashboard.revision, 2);
    server.docs.delete(d.id); es.last().emit('dashboard_changed', { id: d.id, revision: 0, approved_revision: 0 }); await clock.advance(300);
    assert.equal(inst.state().mode, 'home'); assert.equal(inst.state().notice, 'dashboard_deleted'); assert.equal(host.querySelectorAll('.hd-widget').length, 0); inst.dispose(); }],
  ['app: a disabled HomeDash shows the disabled state; a hidden window leaves the stream union', async () => {
    const off = mountApp({ disabled: true }); await off.clock.advance(20); assert.equal(off.inst.state().mode, 'disabled'); off.inst.dispose();
    const subs = [], vis = { visible: () => true, subscribe(fn) { subs.push(fn); return () => {}; } };
    const { inst, clock, es, server } = mountApp({ visibility: vis }); await clock.advance(20); inst.createStarter(); await clock.advance(20); es.last().emit('hello', HELLO); await clock.advance(20);
    for (const fn of subs) fn(false); await clock.advance(300);
    const puts = server.calls.filter(c => c.method === 'PUT'); eq(puts[puts.length - 1].body.entities, []); inst.dispose(); }]
];
```

- [ ] Run → FAIL. Implement `ui/js/homedash/hd-app.js`:

```js
// HomeDash mount (F2; design-frontend 1.4). mount(host, options) is the composition root of one HomeDash window. Windows of one JS realm
// share one API client, stream and store per base path (realmFor). It loads bootstrap and the widget catalog, lists the dashboards, creates
// the starter (template "system") or an empty dashboard, deletes one, opens one dashboard in an hd-view and keeps its entities subscribed while
// the host's visibility signal says visible, refetches the list 250 ms after dashboard_changed (once per realm) and reconciles the open
// dashboard by revision. The host draws its chrome from the frozen 'state' object. View only in M2.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.api || !E.stream || !E.store || !E.catalog || !E.view) throw new Error('hd-app.js: load the HomeDash engine first');
    const realms = new Map(), LIST_COALESCE_MS = 250;
    function realmFor(base, transport, timers) {
        const have = realms.get(base);
        if (have) return have;
        const tm = timers || {}, api = E.api.create({ base, transport }), clock = { setTimeout: tm.setTimeout, clearTimeout: tm.clearTimeout, now: tm.now };
        const diagnose = () => api.bootstrap().then(() => 'ok', err => (err.code === 'homedash_disabled' ? 'disabled' : err.status === 401 ? 'auth_lost' : 'error'));
        const stream = E.stream.shared(Object.assign({ api, random: tm.random, diagnose }, clock));
        const store = E.store.shared(Object.assign({ api, stream, raf: tm.raf, caf: tm.caf }, clock));
        const setT = tm.setTimeout || ((fn, ms) => setTimeout(fn, ms)), clearT = tm.clearTimeout || (id => clearTimeout(id));
        const realm = { api, stream, store, list: null, load: null, timer: 0, listeners: new Set(), refresh: null };
        realm.refresh = () => {
            if (realm.load) return realm.load;
            realm.load = api.dashboards().then(answer => {
                realm.list = Object.freeze((answer && Array.isArray(answer.dashboards) ? answer.dashboards : []).filter(d => d && typeof d.id === 'string'));
                for (const fn of [...realm.listeners]) fn(realm.list);
                return realm.list;
            }).finally(() => { realm.load = null; });
            return realm.load;
        };
        store.on('dashboards', () => { clearT(realm.timer); realm.timer = setT(() => { realm.timer = 0; realm.refresh().catch(() => {}); }, LIST_COALESCE_MS); });
        realms.set(base, realm);
        return realm;
    }
    function mount(hostEl, options) {
        const o = options || {}, t = typeof o.t === 'function' ? o.t : (k => k), lang = o.lang || 'en';
        const realm = realmFor(o.base || '/api/desktop/homedash', o.transport, o.timers), api = realm.api, store = realm.store;
        const bus = E.core.emitter(), life = E.core.bag(), vis = o.visibility || { visible: () => true, subscribe: () => () => {} };
        const root = E.el('div', { class: 'hd-root hd-backdrop', 'data-hd-host': o.host || 'desktop', 'data-hd-mode': 'view' });
        hostEl.appendChild(root);
        const watch = store.watch(E.core.uid('hd-win'), { ids: [], active: vis.visible() });
        life.add(vis.subscribe(on => watch.setActive(on)));
        let state = Object.freeze({ mode: 'loading', error: null, notice: null, busy: false, dashboards: null, dashboard: null, pageId: null, connection: store.connection(), desktopReadonly: !!o.readonly, banners: [] });
        let catalog = null, view = null, doc = null, disposed = false, openSeq = 0;
        function set(patch) {
            if (disposed) return;
            const next = Object.assign({}, state, patch);
            next.banners = E.view.banners({ mode: next.mode, connection: next.connection, sources: store.sources(), desktopReadonly: next.desktopReadonly, schemaTooNew: !!(next.dashboard && next.dashboard.schemaTooNew) });
            state = Object.freeze(next); bus.emit('state', state);
        }
        const context = () => { if (typeof o.onContext === 'function') o.onContext({ dashboard_id: doc ? doc.id : null, page_id: view && doc ? view.pageId() : null }); };
        const hook = (type, fn) => { store.on(type, fn); life.add(() => store.off(type, fn)); };
        hook('connection', c => { if (view) view.setConnection(c); set({ connection: c }); if (c.phase === 'stopped' && c.reason === 'disabled') recheck(); });
        hook('sources', () => set({}));
        const onList = list => reconcile(list);
        realm.listeners.add(onList); life.add(() => realm.listeners.delete(onList)); life.add(() => watch.release());
        function fail(err, fallback) {
            const e = E.api.normalizeError(err);
            if (e.code === 'aborted') return;
            if (e.code === 'homedash_disabled') { closeView(); set({ mode: 'disabled', busy: false }); return; }
            if (e.code === 'desktop_readonly') setReadonly(true);
            fallback(e);
        }
        async function boot() {
            set({ mode: 'loading', error: null, notice: null });
            try {
                const b = await api.bootstrap();
                store.setSources(b.sources);
                const ro = !!o.readonly || b.desktop_readonly === true;
                store.setDesktopReadonly(ro);
                catalog = E.catalog.create(await store.widgetCatalog(b.catalog_hash));
                set({ desktopReadonly: ro });
                if (o.dashboardId) await open(o.dashboardId, o.pageId); else await home(null);
            } catch (err) { fail(err, e => set({ mode: 'error', error: Object.freeze({ code: e.code, status: e.status }) })); }
        }
        function recheck() { api.bootstrap().then(() => realm.stream.resume(), err => fail(err, () => {})); }
        async function home(notice) {
            openSeq += 1; closeView();
            set({ mode: 'home', dashboard: null, pageId: null, notice: notice || null, dashboards: realm.list });
            context();
            try { set({ dashboards: await realm.refresh() }); } catch (err) { fail(err, () => set({ notice: 'list_failed', dashboards: realm.list || [] })); }
        }
        async function open(id, pageId) {
            const mine = ++openSeq;
            if (state.mode !== 'dashboard') set({ mode: 'loading', notice: null });
            let answer;
            try { answer = await api.dashboard(id); }
            catch (err) { if (mine !== openSeq) return; fail(err, e => home(e.code === 'dashboard_not_found' || e.code === 'bad_request' ? 'dashboard_missing' : 'open_failed')); return; }
            if (mine !== openSeq || disposed) return;
            show(answer && answer.dashboard, pageId, !!(answer && answer.schema_too_new === true));
        }
        function show(d, pageId, schemaTooNew) {
            if (!d || typeof d.id !== 'string' || !catalog) { home('open_failed'); return; }
            if (!view) view = E.view.create({ root, store, catalog, t, lang, host: o.host || 'desktop', scheme: o.scheme, motion: o.motion, raf: o.timers && o.timers.raf, caf: o.timers && o.timers.caf,
                measure: o.measure, onPage: p => { watch.set(view.entityIds()); set({ pageId: p }); context(); } });
            doc = d; view.show(d, pageId); view.setConnection(store.connection()); view.setReadonly(state.desktopReadonly);
            watch.set(view.entityIds());
            set({ mode: 'dashboard', notice: null, busy: false, pageId: view.pageId(),
                dashboard: Object.freeze({ id: d.id, name: typeof d.name === 'string' && d.name ? d.name : d.id, revision: d.revision, savedBy: d.saved_by, schemaTooNew }) });
            context();
        }
        function closeView() { if (view) { view.destroy(); view = null; } doc = null; watch.set([]); }
        function reconcile(list) {
            if (disposed) return;
            if (state.mode === 'home') { set({ dashboards: list }); return; }
            if (state.mode !== 'dashboard' || !doc) return;
            const s = list.find(x => x.id === doc.id);
            if (!s) home('dashboard_deleted');
            else if (Number(s.revision) > Number(doc.revision)) open(doc.id, view ? view.pageId() : null);
        }
        async function create(kind) {
            if (state.busy || state.desktopReadonly || !catalog) return;
            set({ busy: true, notice: null });
            try {
                const answer = kind === 'starter' ? await api.createFromTemplate('system', t('homedash.home.starter_name'))
                    : await api.createDocument({ name: t('homedash.home.empty_name'), pages: [{ id: 'main', name: t('homedash.home.page_main') }] });
                if (disposed) return;
                openSeq += 1; show(answer && answer.dashboard, null, false);
                realm.refresh().catch(() => {});
            } catch (err) { fail(err, e => set({ busy: false, notice: e.code === 'limit_exceeded' ? 'limit' : e.code === 'desktop_readonly' ? 'readonly' : 'create_failed' })); }
        }
        async function remove(summary) {
            if (state.busy || state.desktopReadonly || !summary || typeof summary.id !== 'string') return false;
            set({ busy: true, notice: null });
            try { await api.deleteDashboard(summary.id, summary.revision); set({ busy: false }); await realm.refresh().catch(() => {}); return true; }
            catch (err) {
                fail(err, e => set({ busy: false, notice: e.code === 'revision_conflict' ? 'delete_conflict' : e.code === 'desktop_readonly' ? 'readonly' : 'delete_failed' }));
                realm.refresh().catch(() => {}); return false;
            }
        }
        function setReadonly(flag) { const ro = !!flag; store.setDesktopReadonly(ro); if (view) view.setReadonly(ro); if (ro !== state.desktopReadonly) set({ desktopReadonly: ro }); }
        boot();
        return Object.freeze({
            on: bus.on, off: bus.off, state: () => state, open, home: () => home(null), createStarter: () => create('starter'), createEmpty: () => create('empty'), remove, setReadonly,
            refreshLook() { if (view) view.refreshLook(); }, retry: () => boot(),
            reconnect() { const s = realm.stream.status(); if (s.phase === 'stopped') realm.stream.resume(); else realm.stream.retryNow(); },
            dispose() { if (disposed) return; disposed = true; openSeq += 1; life.dispose(); if (view) view.destroy(); view = null; root.remove(); }
        });
    }
    E.mount = mount;
})();
```

- [ ] Run `npm run test:homedash` → three app cases PASS, all earlier cases PASS.
- [ ] Detect-changes; commit `ui/js/homedash/hd-app.js scripts/homedash-tests/app.test.mjs scripts/homedash-tests/harness.mjs` — `feat(homedash): mount composition root with shared realm`.

**Done when:** an integration run in the fake DOM goes from empty home to a live, toned starter dashboard on one stream.

---

### Task 13: homedash-home.js, the list and first-run controller

**Files:** create `ui/js/desktop/apps/homedash-home.js`, `scripts/homedash-tests/home.test.mjs`.
**Read first:** Task 12 state object; `hd-format.js` `relTime`.

- [ ] Write the failing test `scripts/homedash-tests/home.test.mjs`:

```js
import assert from 'node:assert/strict'; import { ENGINE_ORDER, loadFiles } from './harness.mjs';
const FILES = ENGINE_ORDER.map(f => 'ui/js/homedash/' + f).concat(['ui/js/desktop/apps/homedash-home.js']);
function setup() {
  const { window, document } = loadFiles(FILES, { dom: true }), calls = [];
  const home = window.HomeDashHome.create({ t: (k, v) => (v && v.name ? k + ':' + v.name : k), lang: 'en', onOpen: id => calls.push('open:' + id), onStarter: () => calls.push('starter'),
    onEmpty: () => calls.push('empty'), onDelete: s => calls.push('delete:' + s.id) });
  document.body.appendChild(home.el);
  return { home, calls, click: node => home.el.dispatchEvent({ type: 'click', target: node }) };
}
const D = { id: 'd_aaaaaaaaaaab', name: '<b>System</b>', revision: 1, updated_at: new Date().toISOString(), saved_by: 'agent' };
export const tests = [
  ['home: first run offers starter and empty dashboard; busy marks only the pressed choice', () => {
    const { home, calls, click } = setup(); home.render({ dashboards: [], desktopReadonly: false, busy: false });
    assert.equal(home.el.getAttribute('data-hdx-mode'), 'empty'); assert.equal(home.el.querySelector('.hdx-empty').hasAttribute('hidden'), false);
    assert.ok(home.el.querySelector('.hdx-empty svg.hdx-art')); const starter = home.el.querySelector('.hdx-empty [data-action="create-starter"]');
    assert.equal(home.el.querySelector('.hdx-empty-title').textContent, 'homedash.home.empty_title'); click(starter); assert.deepEqual(calls, ['starter']);
    home.render({ dashboards: [], desktopReadonly: false, busy: true });
    assert.equal(starter.getAttribute('aria-busy'), 'true'); assert.equal(home.el.querySelector('.hdx-empty [data-action="create-empty"]').getAttribute('aria-busy'), null); assert.ok(starter.hasAttribute('disabled')); }],
  ['home: a read-only desktop hides creation and shows the note', () => {
    const { home } = setup(); home.render({ dashboards: [], desktopReadonly: true });
    for (const b of home.el.querySelectorAll('[data-action="create-starter"], [data-action="create-empty"]')) assert.ok(b.hasAttribute('hidden'));
    assert.equal(home.el.querySelector('.hdx-readonly-note').hasAttribute('hidden'), false); }],
  ['home: the list shows server names as text, opens and deletes, and keeps its nodes for the same list', () => {
    const { home, calls, click } = setup(); home.render({ dashboards: [D], desktopReadonly: false });
    assert.equal(home.el.getAttribute('data-hdx-mode'), 'list'); const item = home.el.querySelector('.hdx-item');
    assert.equal(item.querySelector('.hdx-card-name').textContent, '<b>System</b>'); assert.equal(item.querySelector('b'), null);
    assert.ok(item.querySelector('.hdx-card-meta').textContent.includes('homedash.home.by_agent'));
    click(item.querySelector('[data-action="open"]')); click(item.querySelector('[data-action="delete"]')); assert.deepEqual(calls, ['open:' + D.id, 'delete:' + D.id]);
    home.render({ dashboards: [D], desktopReadonly: false }); assert.equal(home.el.querySelector('.hdx-item'), item);
    home.render({ dashboards: [D], desktopReadonly: true }); assert.equal(home.el.querySelector('.hdx-card-delete'), null); }],
  ['home: notices are translated status lines', () => {
    const { home } = setup(); home.render({ dashboards: [], notice: 'limit' }); const n = home.el.querySelector('.hdx-notice');
    assert.equal(n.textContent, 'homedash.home.notice_limit'); assert.equal(n.getAttribute('role'), 'status'); assert.equal(n.getAttribute('data-tone'), 'warn');
    home.render({ dashboards: [], notice: null }); assert.ok(n.hasAttribute('hidden')); }]
];
```

- [ ] Run → FAIL. Implement `ui/js/desktop/apps/homedash-home.js`:

```js
/* HomeDash desktop home view (F2): the dashboard list and the first-run empty state, painted from the desktop theme (desktop-app-homedash.css).
   window.HomeDashHome.create({t, lang, onOpen(id), onStarter(), onEmpty(), onDelete(summary)}) returns {el, render(state), destroy()}.
   render(state) reads state.dashboards (null while loading), desktopReadonly, busy and notice (Task 12 state). Names and dates come from the
   server and reach the page only through textContent; the same list renders no new nodes and keeps the focus. */
(function () {
    'use strict';
    const DOT = ' ' + String.fromCodePoint(0xb7) + ' ';
    const NOTICE_TONES = { limit: 'warn', create_failed: 'warn', list_failed: 'warn', open_failed: 'warn', delete_failed: 'warn', delete_conflict: 'warn', readonly: 'info', dashboard_deleted: 'info', dashboard_missing: 'info' };
    function create(options) {
        const o = options || {}, E = window.HomeDashEngine, F = E.format, el = E.el, svg = E.svg, setAttr = E.core.setAttr, t = o.t, lang = o.lang || 'en', life = E.core.bag();
        let current = [], lastSig = '', pending = '';
        function noticeText(code) {
            switch (code) {
                case 'dashboard_deleted': return t('homedash.home.notice_deleted');
                case 'dashboard_missing': return t('homedash.home.notice_missing');
                case 'open_failed': return t('homedash.home.notice_open_failed');
                case 'list_failed': return t('homedash.home.notice_list_failed');
                case 'limit': return t('homedash.home.notice_limit');
                case 'create_failed': return t('homedash.home.notice_create_failed');
                case 'readonly': return t('homedash.home.notice_readonly');
                case 'delete_conflict': return t('homedash.home.notice_delete_conflict');
                case 'delete_failed': return t('homedash.home.notice_delete_failed');
                default: return '';
            }
        }
        function art() {
            const s = (tag, attrs) => svg(tag, attrs);
            return svg('svg', { class: 'hdx-art', viewBox: '0 0 160 112', 'aria-hidden': 'true', focusable: 'false' }, [
                s('rect', { class: 'hdx-art-card', x: 4, y: 4, width: 72, height: 60, rx: 10 }), s('rect', { class: 'hdx-art-card', x: 84, y: 4, width: 72, height: 28, rx: 10 }),
                s('rect', { class: 'hdx-art-card', x: 84, y: 36, width: 72, height: 28, rx: 10 }), s('rect', { class: 'hdx-art-card', x: 4, y: 72, width: 152, height: 36, rx: 10 }),
                s('path', { class: 'hdx-art-track', d: 'M22 50a18 18 0 1 1 36 0' }), s('path', { class: 'hdx-art-value', d: 'M22 50a18 18 0 0 1 30.7-12.7' }),
                s('rect', { class: 'hdx-art-pill', x: 96, y: 12, width: 26, height: 12, rx: 6 }), s('circle', { class: 'hdx-art-knob', cx: 116, cy: 18, r: 4 }),
                s('rect', { class: 'hdx-art-bar', x: 96, y: 46, width: 48, height: 6, rx: 3 }),
                s('polyline', { class: 'hdx-art-line', points: '16,98 40,88 62,94 86,80 108,86 132,78 146,82' }), s('circle', { class: 'hdx-art-dot', cx: 146, cy: 82, r: 3.5 })]);
        }
        const button = (action, icon, label, primary) => el('button', { type: 'button', class: 'hdx-btn' + (primary ? ' is-primary' : ''), 'data-action': action }, [E.icons.icon(icon), el('span', { text: label })]);
        const choice = (action, icon, label, hint, primary) => el('button', { type: 'button', class: 'hdx-choice' + (primary ? ' is-primary' : ''), 'data-action': action }, [
            el('span', { class: 'hdx-choice-icon', 'aria-hidden': 'true' }, E.icons.icon(icon)), el('span', { class: 'hdx-choice-text' }, [el('strong', { text: label }), el('small', { text: hint })])]);
        const head = el('div', { class: 'hdx-home-head', hidden: 'true' }, [el('h2', { class: 'hdx-home-title', text: t('homedash.home.title') }), el('div', { class: 'hdx-home-actions' },
            [button('create-starter', 'gauge', t('homedash.home.create_starter'), true), button('create-empty', 'plus', t('homedash.home.create_empty'), false)])]);
        const notice = el('p', { class: 'hdx-note hdx-notice', role: 'status', 'data-tone': 'info', hidden: 'true' });
        const roNote = el('p', { class: 'hdx-note hdx-readonly-note', 'data-tone': 'info', hidden: 'true' }, [E.icons.icon('lock'), el('span', { text: t('homedash.home.readonly_note') })]);
        const skeleton = el('div', { class: 'hdx-skeleton', 'aria-hidden': 'true' }, [el('div', { class: 'hdx-skel-card' }), el('div', { class: 'hdx-skel-card' }), el('div', { class: 'hdx-skel-card' })]);
        const list = el('ul', { class: 'hdx-list', 'aria-label': t('homedash.home.title'), hidden: 'true' });
        const empty = el('div', { class: 'hdx-empty', hidden: 'true' }, [art(), el('h2', { class: 'hdx-empty-title', text: t('homedash.home.empty_title') }),
            el('p', { class: 'hdx-empty-text', text: t('homedash.home.empty_text') }), el('div', { class: 'hdx-choices' }, [
                choice('create-starter', 'gauge', t('homedash.home.create_starter'), t('homedash.home.create_starter_hint'), true),
                choice('create-empty', 'dashboard', t('homedash.home.create_empty'), t('homedash.home.create_empty_hint'), false)])]);
        const root = el('section', { class: 'hdx-home', 'data-hdx-mode': 'loading', 'aria-label': t('homedash.home.title') }, el('div', { class: 'hdx-home-inner' }, [head, notice, roNote, skeleton, list, empty]));
        function item(d, ro) {
            const name = typeof d.name === 'string' && d.name ? d.name : d.id, when = Date.parse(d.updated_at);
            const meta = [Number.isFinite(when) ? t('homedash.home.updated', { when: F.relTime(when, Date.now(), lang) }) : '', d.saved_by === 'agent' ? t('homedash.home.by_agent') : ''].filter(Boolean).join(DOT);
            return el('li', { class: 'hdx-item' }, el('div', { class: 'hdx-card' }, [
                el('button', { type: 'button', class: 'hdx-card-open', 'data-action': 'open', 'data-id': d.id, 'aria-label': t('homedash.home.open', { name }) }, [
                    el('span', { class: 'hdx-card-icon', 'aria-hidden': 'true' }, E.icons.icon('dashboard')),
                    el('span', { class: 'hdx-card-text' }, [el('span', { class: 'hdx-card-name', text: name }), el('span', { class: 'hdx-card-meta', text: meta })])]),
                ro ? null : el('button', { type: 'button', class: 'hdx-card-delete', 'data-action': 'delete', 'data-id': d.id, 'aria-label': t('homedash.home.delete', { name }), title: t('homedash.home.delete', { name }) }, E.icons.icon('trash'))]));
        }
        function paintList(dashboards, ro) {
            const active = root.ownerDocument && root.ownerDocument.activeElement;
            const keep = active && active.closest && active.closest('.hdx-list') === list ? [active.getAttribute('data-action'), active.getAttribute('data-id')] : null;
            list.replaceChildren(...dashboards.map(d => item(d, ro)));
            if (keep) { const again = Array.from(list.querySelectorAll('button')).find(b => b.getAttribute('data-action') === keep[0] && b.getAttribute('data-id') === keep[1]); if (again) again.focus(); }
        }
        function render(s) {
            const st = s || {}, dashboards = Array.isArray(st.dashboards) ? st.dashboards : null, ro = !!st.desktopReadonly, busy = !!st.busy;
            const mode = !dashboards ? 'loading' : dashboards.length ? 'list' : 'empty';
            if (!busy) pending = '';
            setAttr(root, 'data-hdx-mode', mode);
            setAttr(head, 'hidden', mode !== 'list'); setAttr(list, 'hidden', mode !== 'list'); setAttr(empty, 'hidden', mode !== 'empty'); setAttr(skeleton, 'hidden', mode !== 'loading');
            for (const b of root.querySelectorAll('[data-action="create-starter"], [data-action="create-empty"]')) {
                setAttr(b, 'hidden', ro); setAttr(b, 'disabled', busy); setAttr(b, 'aria-busy', busy && b.getAttribute('data-action') === pending ? 'true' : null);
            }
            setAttr(roNote, 'hidden', !ro);
            const text = noticeText(st.notice); notice.textContent = text; setAttr(notice, 'hidden', !text); setAttr(notice, 'data-tone', NOTICE_TONES[st.notice] || 'info');
            if (dashboards) {
                current = dashboards;
                const sig = JSON.stringify([ro, dashboards.map(d => [d.id, d.name, d.revision, d.updated_at, d.saved_by])]);
                if (sig !== lastSig) { lastSig = sig; paintList(dashboards, ro); }
            }
        }
        life.on(root, 'click', ev => {
            const b = ev.target && ev.target.closest ? ev.target.closest('button') : null;
            if (!b || b.hasAttribute('disabled')) return;
            const action = b.getAttribute('data-action'), id = b.getAttribute('data-id');
            if (action === 'create-starter') { pending = action; o.onStarter(); }
            else if (action === 'create-empty') { pending = action; o.onEmpty(); }
            else if (action === 'open' && id) o.onOpen(id);
            else if (action === 'delete' && id) { const s = current.find(d => d.id === id); if (s) o.onDelete(s); }
        });
        return Object.freeze({ el: root, render, destroy() { life.dispose(); root.remove(); } });
    }
    window.HomeDashHome = Object.freeze({ create });
})();
```

- [ ] Run → four home cases PASS. Detect-changes; commit `ui/js/desktop/apps/homedash-home.js scripts/homedash-tests/home.test.mjs` — `feat(homedash): desktop list and first-run state`.

**Done when:** four PASS; server text never reaches markup parsing.

---

### Task 14: homedash.js, the desktop host

**Files:** create `ui/js/desktop/apps/homedash.js`, `scripts/homedash-tests/host.test.mjs`.
**Read first:** `docker-manager.js:1-60,466-480,1050-1086` (render, menus, dispose); `docker-manager-store.js:279-316` (visibility rule F11, policy event); Task 12 state; design-frontend 8.

Skeleton (order): `div.hdx-app[data-hdx-view]` > `header.hdx-bar` (`button.hdx-back[data-action=home][hidden]` with `arrow-left` icon + text `homedash.view.back`; `div.hdx-heading > h1.hdx-title`; `span.hdx-conn[data-conn][hidden] > span.hdx-conn-dot[aria-hidden] + span.hdx-conn-text`), `div.hdx-banners`, `main.hdx-main` (home `section.hdx-home`, `div.hdx-view[hidden]` = mount target, `div.hdx-state[hidden]`), then the `hd-sheet` layer (appended by `E.sheet.create(root)`).

- [ ] Write the failing test `scripts/homedash-tests/host.test.mjs` (Task 15 appends the CSS case):

```js
import assert from 'node:assert/strict'; import { read, loadFiles } from './harness.mjs'; import { lintJs } from './css-lint.mjs';
export const tests = [
  ['host: the desktop files are clean and within the line budget', () => {
    for (const f of ['ui/js/desktop/apps/homedash.js', 'ui/js/desktop/apps/homedash-home.js']) { const text = read(f); assert.deepEqual(lintJs(text, f), [], f); assert.ok(text.split('\n').length < 1100, f); } }],
  ['host: the visibility signal follows the tab, the window and its space (Docker Manager rule)', () => {
    const { window, document } = loadFiles(['ui/js/desktop/apps/homedash.js'], { dom: true }), listeners = [];
    document.hidden = false; document.addEventListener = (type, fn) => listeners.push([type, fn]);
    const cls = new Set(), win = { hidden: false, style: { display: '' }, classList: { contains: c => cls.has(c) }, getClientRects: () => [1] };
    const life = new AbortController(), seen = [], sig = window.HomeDashApp.visibilitySignal(document.createElement('div'), win, life.signal);
    assert.equal(sig.visible(), true); sig.subscribe(v => seen.push(v));
    cls.add('vd-space-hidden'); listeners.find(x => x[0] === 'visibilitychange')[1](); assert.equal(sig.visible(), false);
    cls.delete('vd-space-hidden'); document.hidden = true; listeners[0][1](); assert.equal(sig.visible(), false);
    document.hidden = false; listeners[0][1](); assert.deepEqual(seen, [false, true]); life.abort(); }]
];
```

- [ ] Run → FAIL. Implement `ui/js/desktop/apps/homedash.js` (full code; keep the numbered rules in comments short):

```js
/* HomeDash desktop host (F2; design-frontend 8). window.HomeDashApp = {render, dispose, visibilitySignal}. render(host, id, ctx) builds the
   window chrome (bar, banners, home view, dashboard view, state panel, confirm sheet), mounts the engine (HomeDashEngine.mount) with the
   desktop transport (ctx.api and EventSource), the visibility signal, the scheme and motion of the desktop body, the read-only policy event,
   window menus and the window context (dashboard_id, page_id) for session restore. Chrome is painted from the desktop theme
   (desktop-app-homedash.css); the dashboard canvas is the engine's .hd-root. Server text reaches the page only through textContent. */
(function () {
    'use strict';
    const instances = new Map();
    function dispose(id) { const fn = instances.get(id); instances.delete(id); if (fn) fn(); }
    // Visible while the tab is shown, the .vd-window is not hidden, minimised (no client rects) or on another space, and the app root
    // intersects the viewport (docker-manager-store.js attach()). Returns {visible(), subscribe(fn) -> unsubscribe}.
    function visibilitySignal(root, win, signal) {
        const listeners = new Set();
        let inView = typeof IntersectionObserver !== 'function', shown = false;
        const compute = () => !document.hidden && inView && (!win || (!win.hidden && !win.classList.contains('vd-space-hidden') && win.style.display !== 'none' && win.getClientRects().length > 0));
        const check = () => { const next = compute(); if (next === shown) return; shown = next; for (const fn of [...listeners]) { try { fn(shown); } catch (err) { console.error(err); } } };
        document.addEventListener('visibilitychange', check, { signal });
        const mo = typeof MutationObserver === 'function' && win ? new MutationObserver(check) : null;
        if (mo) mo.observe(win, { attributes: true, attributeFilter: ['style', 'class', 'hidden', 'data-space-hidden'] });
        const io = typeof IntersectionObserver === 'function' ? new IntersectionObserver(entries => { inView = !!(entries.length && entries[entries.length - 1].isIntersecting); check(); }, { threshold: 0.01 }) : null;
        if (io) io.observe(root);
        signal.addEventListener('abort', () => { if (mo) mo.disconnect(); if (io) io.disconnect(); listeners.clear(); }, { once: true });
        shown = compute();
        return { visible: () => shown, subscribe(fn) { listeners.add(fn); return () => listeners.delete(fn); } };
    }
    function render(host, id, ctx) {
        dispose(id);
        const E = window.HomeDashEngine, Home = window.HomeDashHome, el = E.el, setAttr = E.core.setAttr, icon = name => E.icons.icon(name);
        const t = (key, vars) => ctx.t(key, vars), lifetime = new AbortController(), signal = lifetime.signal, lang = document.documentElement.lang || 'en';
        const BANNER_ICONS = { auth_lost: 'lock', offline: 'signal-off', source_unavailable: 'alert', source_degraded: 'clock', schema_too_new: 'info', readonly: 'lock' };
        let inst = null, lastBanners = '', lastState = '', lastMenus = '', lookFrame = 0;
        const title = el('h1', { class: 'hdx-title', text: t('desktop.app_homedash') });
        const back = el('button', { type: 'button', class: 'hdx-back', 'data-action': 'home', hidden: 'true' }, [icon('arrow-left'), el('span', { text: t('homedash.view.back') })]);
        const connText = el('span', { class: 'hdx-conn-text' });
        const conn = el('span', { class: 'hdx-conn', 'data-conn': 'connecting', hidden: 'true' }, [el('span', { class: 'hdx-conn-dot', 'aria-hidden': 'true' }), connText]);
        const banners = el('div', { class: 'hdx-banners' });
        const viewHost = el('div', { class: 'hdx-view', hidden: 'true' }), statePanel = el('div', { class: 'hdx-state', hidden: 'true' });
        const home = Home.create({ t, lang, onOpen: dashboardId => inst.open(dashboardId), onStarter: () => inst.createStarter(), onEmpty: () => inst.createEmpty(), onDelete: summary => askDelete(summary) });
        setAttr(home.el, 'hidden', true);
        const root = el('div', { class: 'hdx-app', 'data-hdx-view': 'loading' }, [el('header', { class: 'hdx-bar' }, [back, el('div', { class: 'hdx-heading' }, title), conn]), banners,
            el('main', { class: 'hdx-main' }, [home.el, viewHost, statePanel])]);
        host.replaceChildren(root);
        const sheet = E.sheet.create(root), win = host.closest('.vd-window'), subtitleEl = win ? win.querySelector('.vd-window-subtitle') : null;
        const subtitle = text => { if (subtitleEl) subtitleEl.textContent = text; };
        const motion = () => (document.body.dataset.animations === 'false' || (typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches) ? 'reduced' : 'full');
        const sourceName = src => (src === 'system' ? t('homedash.source.system') : String(src));
        function bannerText(b) {
            switch (b.kind) {
                case 'auth_lost': return t('homedash.banner.auth_lost');
                case 'offline': return t('homedash.banner.offline');
                case 'source_unavailable': return t('homedash.banner.source_unavailable', { source: sourceName(b.source) });
                case 'source_degraded': return t('homedash.banner.source_degraded', { source: sourceName(b.source) });
                case 'schema_too_new': return t('homedash.banner.schema_too_new');
                default: return t('homedash.banner.readonly');
            }
        }
        function paintBanners(list) {
            const sig = JSON.stringify(list.map(b => [b.id, b.kind])); if (sig === lastBanners) return; lastBanners = sig;
            banners.replaceChildren(...list.map(b => el('div', { class: 'hdx-banner', 'data-banner': b.kind, 'data-tone': b.tone, role: b.role }, [icon(BANNER_ICONS[b.kind] || 'info'),
                el('span', { class: 'hdx-banner-text', text: bannerText(b) }),
                b.action === 'retry' ? el('button', { type: 'button', class: 'hdx-btn hdx-banner-action', 'data-action': 'reconnect', text: t('homedash.banner.retry') }) : null])));
        }
        function paintState(s) {
            const kind = ['loading', 'disabled', 'error'].includes(s.mode) ? s.mode : '', key = kind + ':' + (s.error ? s.error.code : '');
            if (key === lastState) return; lastState = key; statePanel.replaceChildren(); setAttr(statePanel, 'data-kind', kind || null);
            if (kind === 'loading') statePanel.append(el('div', { class: 'hdx-spinner', role: 'status', 'aria-label': t('homedash.app.loading') }));
            if (kind === 'disabled') statePanel.append(icon('power'), el('h2', { class: 'hdx-state-title', text: t('homedash.disabled.title') }),
                el('p', { class: 'hdx-state-text', text: t('homedash.disabled.text') }), el('button', { type: 'button', class: 'hdx-btn', 'data-action': 'retry', text: t('homedash.disabled.retry') }));
            if (kind === 'error') statePanel.append(icon('alert'), el('h2', { class: 'hdx-state-title', text: t('homedash.error.title') }),
                el('p', { class: 'hdx-state-text', text: s.error && s.error.code === 'storage_unavailable' ? t('homedash.error.storage') : t('homedash.error.generic', { code: (s.error && s.error.code) || '?' }) }),
                el('button', { type: 'button', class: 'hdx-btn', 'data-action': 'retry', text: t('homedash.home.retry') }));
        }
        function menus(s) {
            const sig = s.mode + '|' + !!s.desktopReadonly + '|' + !!s.busy; if (sig === lastMenus) return; lastMenus = sig;
            const blocked = !!s.desktopReadonly || !!s.busy || s.mode === 'disabled' || s.mode === 'error';
            ctx.setWindowMenus?.(id, [{ id: 'homedash', label: t('desktop.app_homedash'), items: [
                { label: t('homedash.menu.all'), disabled: s.mode === 'disabled', action: () => inst.home() },
                { type: 'separator' },
                { label: t('homedash.menu.new_starter'), icon: 'plus', disabled: blocked, action: () => inst.createStarter() },
                { label: t('homedash.menu.new_empty'), disabled: blocked, action: () => inst.createEmpty() },
                { type: 'separator' },
                { label: t('homedash.menu.reload'), icon: 'refresh', action: () => { const st = inst.state(); if (st.mode === 'dashboard' && st.dashboard) inst.open(st.dashboard.id, st.pageId); else inst.retry(); } }] }]);
        }
        function paint(s) {
            setAttr(root, 'data-hdx-view', s.mode);
            const dash = s.mode === 'dashboard' && s.dashboard, c = s.connection || {}, cs = c.phase === 'live' ? 'live' : c.offline ? 'offline' : 'connecting';
            title.textContent = dash ? s.dashboard.name : t('desktop.app_homedash');
            setAttr(back, 'hidden', s.mode !== 'dashboard'); setAttr(conn, 'hidden', s.mode !== 'dashboard'); setAttr(conn, 'data-conn', cs);
            connText.textContent = cs === 'live' ? t('homedash.conn.live') : cs === 'offline' ? t('homedash.conn.offline') : t('homedash.conn.connecting');
            paintBanners(s.banners || []);
            setAttr(home.el, 'hidden', s.mode !== 'home'); setAttr(viewHost, 'hidden', s.mode !== 'dashboard'); setAttr(statePanel, 'hidden', !['loading', 'disabled', 'error'].includes(s.mode));
            if (s.mode === 'home') home.render(s);
            paintState(s); subtitle(dash ? s.dashboard.name : ''); menus(s);
        }
        function askDelete(summary) {
            sheet.confirm({ title: t('homedash.delete.title'), text: [t('homedash.delete.body', { name: summary.name || summary.id })], confirmLabel: t('homedash.delete.confirm'), cancelLabel: t('homedash.delete.cancel'), tone: 'danger' })
                .then(yes => { if (yes) inst.remove(summary); });
        }
        inst = E.mount(viewHost, { host: 'desktop', base: '/api/desktop/homedash', t, lang, readonly: !!ctx.readonly,
            transport: { request: (url, init) => ctx.api(url, init), eventSource: url => new EventSource(url) },
            dashboardId: typeof ctx.dashboard_id === 'string' && ctx.dashboard_id ? ctx.dashboard_id : null, pageId: typeof ctx.page_id === 'string' ? ctx.page_id : null,
            visibility: visibilitySignal(root, win, signal), scheme: () => (document.body.dataset.scheme === 'light' ? 'light' : 'dark'), motion,
            onContext: patch => { if (typeof ctx.updateWindowContext === 'function') ctx.updateWindowContext(id, patch); } });
        root.addEventListener('click', event => {
            const b = event.target && event.target.closest ? event.target.closest('[data-action]') : null;
            if (!b || b.closest('.hdx-home') || b.closest('.hd-sheet-layer')) return;
            const action = b.getAttribute('data-action');
            if (action === 'home') inst.home(); else if (action === 'reconnect') inst.reconnect(); else if (action === 'retry') inst.retry();
        }, { signal });
        document.addEventListener('aurago:desktop-policy', event => {
            const d = (event && event.detail) || {};
            const ro = d.readonly === true || d.read_only === true ? true : d.readonly === false || d.read_only === false ? false : null;
            if (ro !== null) inst.setReadonly(ro);
        }, { signal });
        const themeObserver = typeof MutationObserver === 'function' ? new MutationObserver(() => {
            if (lookFrame) return; lookFrame = requestAnimationFrame(() => { lookFrame = 0; inst.refreshLook(); });
        }) : null;
        if (themeObserver) themeObserver.observe(document.body, { attributes: true, attributeFilter: ['data-theme', 'data-scheme', 'data-fruity-mode', 'data-accent', 'data-animations', 'style', 'class'] });
        inst.on('state', paint); paint(inst.state());
        instances.set(id, () => {
            lifetime.abort(); if (themeObserver) themeObserver.disconnect(); if (lookFrame) cancelAnimationFrame(lookFrame);
            inst.dispose(); sheet.destroy(); home.destroy(); ctx.setWindowMenus?.(id, []); subtitle(''); root.replaceChildren(); root.remove();
        });
    }
    window.HomeDashApp = { render, dispose, visibilitySignal };
})();
```

- [ ] Run `npm run test:homedash` → two host cases PASS.
- [ ] Detect-changes; commit `ui/js/desktop/apps/homedash.js scripts/homedash-tests/host.test.mjs` — `feat(homedash): desktop host with visibility, policy and menus`.

**Done when:** both PASS; `homedash.js` under 700 lines.

---

### Task 15: desktop-app-homedash.css, the host chrome

**Files:** create `ui/css/desktop-app-homedash.css`; modify `scripts/homedash-tests/host.test.mjs` (append the CSS case).
**Read first:** `documentation/desktop-themes.md` "App materials" (vocabulary, rules 1–10); `ui/css/desktop-app-docker-manager.css:1-70`; `ui/js/desktop/apps/AGENTS.md` "App theme bridge contract".

- [ ] Append the failing case to `host.test.mjs` `tests`:

```js
  ['host: the chrome CSS paints from the theme materials and keeps the app CSS rules', () => {
    const css = read('ui/css/desktop-app-homedash.css'), plain = css.replace(/\/\*[\s\S]*?\*\//g, '');
    assert.ok(plain.includes('var(--vd-mat-'), 'reads no --vd-mat-* token'); assert.ok(!/#[0-9a-fA-F]{3,8}\b/.test(plain), 'hex colour');
    assert.ok(!/\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch)\(/i.test(plain), 'literal colour function'); assert.ok(!/\[data-theme/.test(plain), '[data-theme]');
    assert.ok(!/uppercase/i.test(plain), 'uppercase'); assert.ok(!/light-dark\(/.test(plain), 'light-dark()'); assert.ok(css.split('\n').length <= 900);
    for (const m of plain.matchAll(/([^{}]+)\{([^{}]*)\}/g)) { const sel = m[1].trim(), body = m[2];
      if (/!important/.test(body)) assert.ok(/\[hidden\]$/.test(sel), '!important outside [hidden]: ' + sel);
      if (/\b(transition|animation)\s*:/.test(body)) assert.ok(sel.split(',').every(s => s.trim().startsWith('body:not([data-animations="false"])')), 'ungated motion: ' + sel); }
    assert.ok(plain.includes('@media (prefers-reduced-motion: no-preference)')); }]
```
- [ ] Run → FAIL (file missing). Create `ui/css/desktop-app-homedash.css` with exactly these rules (the class names are the ones Tasks 13–14 build):

```css
/* HomeDash desktop host: window bar, banners, home view (list and first run), state panels and the confirm sheet. Painted from the desktop
   theme's app materials (--vd-mat-*, --vd-*, --ds-*): no hex or literal colour functions, no theme ids, no uppercase labels; motion only
   inside @media (prefers-reduced-motion: no-preference) on selectors that start with body:not([data-animations="false"]). The dashboard
   canvas (.hd-root) is the engine's and is not styled here. scripts/homedash-tests/host.test.mjs guards these rules. */
.hdx-app { --hdx-line: var(--vd-mat-divider, var(--vd-theme-border)); --hdx-card: var(--vd-mat-card-color, var(--vd-theme-panel-bg)); --hdx-muted: var(--vd-mat-label-color, var(--vd-theme-muted)); --hdx-radius: var(--vd-mat-card-radius, 12px);
  position: relative; display: grid; grid-template-rows: auto auto minmax(0, 1fr); inline-size: 100%; block-size: 100%; min-block-size: 0; overflow: hidden; isolation: isolate;
  color: var(--vd-text); background: var(--vd-mat-canvas, var(--vd-theme-app-bg)); font: 14px/1.45 var(--vd-font-ui, system-ui, sans-serif); container: hdx / inline-size; }
.hdx-app * { box-sizing: border-box; }
.hdx-app button { font: inherit; color: inherit; }
.hdx-app [hidden] { display: none !important; }
.hdx-app :is(.hdx-bar, .hdx-banners, .hdx-home, .hdx-state, .hd-sheet-layer) :is(button, [tabindex="0"]):focus-visible { outline: var(--ds-focus-ring-width, 2px) solid var(--ds-color-border-focus, var(--vd-accent)); outline-offset: var(--ds-focus-ring-offset, 2px); }
.hdx-app button:disabled { opacity: .55; cursor: not-allowed; }
.hdx-app :is(.hdx-bar, .hdx-banners, .hdx-home, .hdx-state) .hd-ico { inline-size: 18px; block-size: 18px; stroke-width: 1.75; }
/* Bar */
.hdx-bar { display: flex; align-items: center; gap: 10px; min-block-size: 48px; padding: 6px 14px; background: var(--vd-mat-bar, var(--vd-theme-chrome-bg)); box-shadow: var(--vd-mat-bar-edge, none); border-block-end: 1px solid var(--hdx-line); }
.hdx-heading { flex: 1 1 auto; min-inline-size: 0; }
.hdx-title { margin: 0; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; color: var(--vd-mat-heading-color, var(--vd-text)); font: var(--vd-mat-heading-weight, 650) 15px/1.25 var(--vd-mat-heading-font, var(--vd-font-ui, system-ui, sans-serif)); letter-spacing: var(--vd-mat-heading-tracking, normal); }
.hdx-back { display: inline-flex; align-items: center; gap: 6px; min-block-size: 34px; padding: 0 12px 0 8px; border: 1px solid transparent; border-radius: var(--vd-mat-btn-radius, 8px); background: transparent; font-weight: 600; cursor: pointer; }
.hdx-back:hover { background: var(--vd-mat-hover, var(--vd-theme-control-hover)); }
.hdx-conn { --hdx-dot: var(--ds-color-warning-fg); display: inline-flex; flex: none; align-items: center; gap: 7px; padding: 3px 11px 3px 9px; border-radius: 999px; background: color-mix(in srgb, var(--vd-text) 7%, transparent); color: var(--hdx-muted); font-size: 12.5px; font-weight: 600; }
.hdx-conn[data-conn="live"] { --hdx-dot: var(--ds-color-success-fg); }
.hdx-conn[data-conn="offline"] { --hdx-dot: var(--ds-color-danger-fg); }
.hdx-conn-dot { inline-size: 8px; block-size: 8px; border-radius: 50%; background: var(--hdx-dot); box-shadow: 0 0 0 3px color-mix(in srgb, var(--hdx-dot) 22%, transparent); }
/* Buttons */
.hdx-btn { display: inline-flex; align-items: center; justify-content: center; gap: 8px; min-block-size: 36px; padding: 0 14px; border: 1px solid var(--vd-mat-btn-border, var(--hdx-line)); border-radius: var(--vd-mat-btn-radius, 8px); background: var(--vd-mat-btn, var(--vd-theme-control-bg)); box-shadow: var(--vd-mat-btn-shadow, none); font-weight: 600; white-space: nowrap; cursor: pointer; }
.hdx-app .hdx-btn { color: var(--vd-mat-btn-text, var(--vd-text)); }
.hdx-btn:hover:not(:disabled) { background: var(--vd-mat-btn-hover, var(--vd-theme-control-hover)); }
.hdx-btn.is-primary { border-color: var(--vd-mat-primary-border, transparent); background: var(--vd-mat-primary, var(--vd-accent)); box-shadow: var(--vd-mat-primary-shadow, none); }
.hdx-app .hdx-btn.is-primary { color: var(--vd-mat-primary-text, var(--vd-accent-contrast)); }
.hdx-btn.is-primary:hover:not(:disabled) { background: var(--vd-mat-primary-hover, var(--vd-mat-primary, var(--vd-accent))); }
/* Banners */
.hdx-banners { display: grid; gap: 8px; padding: 10px 14px 0; }
.hdx-banners:empty { display: none; }
.hdx-banner { --hdx-tone: var(--ds-color-info-fg); display: flex; align-items: center; gap: 10px; min-block-size: 42px; padding: 7px 8px 7px 12px; border: 1px solid color-mix(in srgb, var(--hdx-tone) 42%, var(--hdx-line)); border-radius: var(--hdx-radius); background: color-mix(in srgb, var(--hdx-tone) 12%, var(--hdx-card)); color: var(--vd-text); }
.hdx-banner[data-tone="warn"] { --hdx-tone: var(--ds-color-warning-fg); }
.hdx-banner[data-tone="danger"] { --hdx-tone: var(--ds-color-danger-fg); }
.hdx-banner > .hd-ico { flex: none; color: color-mix(in srgb, var(--hdx-tone) 78%, var(--vd-text)); }
.hdx-banner-text { flex: 1 1 auto; min-inline-size: 0; font-size: 13.5px; }
.hdx-app .hdx-banner-action { flex: none; min-block-size: 30px; padding: 0 12px; }
/* Main area and home */
.hdx-main { position: relative; display: grid; min-block-size: 0; overflow: hidden; }
.hdx-main > * { grid-area: 1 / 1; min-block-size: 0; min-inline-size: 0; }
.hdx-view { display: flex; }
.hdx-view > .hd-root { flex: 1 1 auto; min-inline-size: 0; }
.hdx-home { overflow: auto; padding: 26px clamp(16px, 4cqi, 40px) 36px; }
.hdx-home-inner { max-inline-size: 1080px; margin: 0 auto; }
.hdx-home-head { display: flex; flex-wrap: wrap; align-items: center; gap: 12px 16px; margin-block-end: 18px; }
.hdx-home-title { flex: 1 1 auto; margin: 0; color: var(--vd-mat-heading-color, var(--vd-text)); font: var(--vd-mat-heading-weight, 650) 22px/1.2 var(--vd-mat-heading-font, var(--vd-font-ui, system-ui, sans-serif)); letter-spacing: var(--vd-mat-heading-tracking, normal); }
.hdx-home-actions { display: flex; flex-wrap: wrap; gap: 8px; }
.hdx-note { display: flex; align-items: center; gap: 10px; margin: 0 0 16px; padding: 10px 14px; border: 1px solid color-mix(in srgb, var(--ds-color-info-fg) 35%, var(--hdx-line)); border-radius: var(--hdx-radius); background: color-mix(in srgb, var(--ds-color-info-fg) 10%, var(--hdx-card)); font-size: 13.5px; }
.hdx-note[data-tone="warn"] { border-color: color-mix(in srgb, var(--ds-color-warning-fg) 40%, var(--hdx-line)); background: color-mix(in srgb, var(--ds-color-warning-fg) 12%, var(--hdx-card)); }
.hdx-list, .hdx-skeleton { display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 14px; margin: 0; padding: 0; list-style: none; }
.hdx-card { position: relative; display: flex; min-block-size: 92px; border: 1px solid var(--vd-mat-card-border, var(--hdx-line)); border-radius: var(--hdx-radius); background: var(--vd-mat-card, var(--hdx-card)); box-shadow: var(--vd-mat-card-shadow, none); overflow: hidden; }
.hdx-card:hover { border-color: color-mix(in srgb, var(--vd-accent) 45%, var(--hdx-line)); }
.hdx-card-open { display: grid; flex: 1 1 auto; grid-template-columns: 44px minmax(0, 1fr); align-items: center; gap: 14px; padding: 16px 56px 16px 16px; border: 0; background: transparent; text-align: start; cursor: pointer; }
.hdx-card-icon, .hdx-choice-icon { position: relative; display: grid; place-items: center; inline-size: 44px; block-size: 44px; border-radius: 12px; background: color-mix(in srgb, var(--vd-accent) 16%, var(--hdx-card)); color: color-mix(in srgb, var(--vd-accent) 78%, var(--vd-text)); }
.hdx-app :is(.hdx-card-icon, .hdx-choice-icon) .hd-ico { inline-size: 22px; block-size: 22px; }
.hdx-card-text { display: grid; gap: 2px; min-inline-size: 0; }
.hdx-card-name { overflow: hidden; white-space: nowrap; text-overflow: ellipsis; font-size: 15px; font-weight: 650; }
.hdx-card-meta { overflow: hidden; white-space: nowrap; text-overflow: ellipsis; color: var(--hdx-muted); font-size: 12.5px; }
.hdx-card-delete { position: absolute; inset-block-start: 10px; inset-inline-end: 10px; display: grid; place-items: center; inline-size: 34px; block-size: 34px; padding: 0; border: 1px solid transparent; border-radius: 8px; background: transparent; color: var(--hdx-muted); cursor: pointer; }
.hdx-card-delete:hover { border-color: color-mix(in srgb, var(--ds-color-danger-fg) 40%, var(--hdx-line)); color: color-mix(in srgb, var(--ds-color-danger-fg) 80%, var(--vd-text)); }
.hdx-skel-card { block-size: 92px; border-radius: var(--hdx-radius); background: color-mix(in srgb, var(--vd-text) 6%, var(--hdx-card)); }
/* First run */
.hdx-empty { display: grid; justify-items: center; gap: 10px; max-inline-size: 680px; margin: clamp(12px, 6vh, 64px) auto 0; text-align: center; }
.hdx-art { inline-size: 200px; block-size: 140px; margin-block-end: 6px; }
.hdx-art-card { fill: var(--hdx-card); stroke: var(--hdx-line); stroke-width: 1.5; }
.hdx-art-track { fill: none; stroke: color-mix(in srgb, var(--vd-text) 14%, transparent); stroke-width: 6; stroke-linecap: round; }
.hdx-art-value { fill: none; stroke: var(--vd-accent); stroke-width: 6; stroke-linecap: round; }
.hdx-art-pill { fill: var(--vd-accent); }
.hdx-art-knob { fill: var(--vd-accent-contrast, var(--hdx-card)); }
.hdx-art-bar { fill: color-mix(in srgb, var(--ds-color-warning-fg) 70%, var(--hdx-card)); }
.hdx-art-line { fill: none; stroke: var(--ds-color-success-fg); stroke-width: 2.5; stroke-linecap: round; stroke-linejoin: round; }
.hdx-art-dot { fill: var(--ds-color-success-fg); }
.hdx-empty-title { margin: 0; color: var(--vd-mat-heading-color, var(--vd-text)); font: var(--vd-mat-heading-weight, 650) 24px/1.25 var(--vd-mat-heading-font, var(--vd-font-ui, system-ui, sans-serif)); letter-spacing: var(--vd-mat-heading-tracking, normal); }
.hdx-empty-text { max-inline-size: 54ch; margin: 0; color: var(--hdx-muted); font-size: 14.5px; }
.hdx-choices { display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 14px; inline-size: 100%; margin-block-start: 16px; }
.hdx-choice { display: grid; grid-template-columns: 44px minmax(0, 1fr); align-items: center; gap: 14px; min-block-size: 88px; padding: 16px 18px; border: 1px solid var(--vd-mat-card-border, var(--hdx-line)); border-radius: var(--hdx-radius); background: var(--vd-mat-card, var(--hdx-card)); box-shadow: var(--vd-mat-card-shadow, none); text-align: start; cursor: pointer; }
.hdx-choice:hover:not(:disabled) { border-color: color-mix(in srgb, var(--vd-accent) 55%, var(--hdx-line)); }
.hdx-choice.is-primary { border-color: color-mix(in srgb, var(--vd-mat-primary-color, var(--vd-accent)) 70%, var(--hdx-line)); background: color-mix(in srgb, var(--vd-mat-primary-color, var(--vd-accent)) 9%, var(--hdx-card)); }
.hdx-choice.is-primary .hdx-choice-icon { background: var(--vd-mat-primary, var(--vd-accent)); color: var(--vd-mat-primary-text, var(--vd-accent-contrast)); }
.hdx-choice-text { display: grid; gap: 3px; min-inline-size: 0; }
.hdx-choice-text strong { font-size: 15px; font-weight: 650; }
.hdx-choice-text small { color: var(--hdx-muted); font-size: 12.5px; line-height: 1.4; }
.hdx-app :is(.hdx-choice, .hdx-btn)[aria-busy="true"] .hd-ico { opacity: 0; }
.hdx-choice[aria-busy="true"] .hdx-choice-icon::after { content: ""; position: absolute; inset: 12px; border: 2.5px solid currentColor; border-inline-end-color: transparent; border-radius: 50%; }
/* State panels */
.hdx-state { display: grid; place-content: center; justify-items: center; gap: 10px; padding: 32px; text-align: center; }
.hdx-app .hdx-state > .hd-ico { inline-size: 44px; block-size: 44px; color: var(--hdx-muted); stroke-width: 1.5; }
.hdx-state-title { margin: 6px 0 0; color: var(--vd-mat-heading-color, var(--vd-text)); font: var(--vd-mat-heading-weight, 650) 20px/1.3 var(--vd-mat-heading-font, var(--vd-font-ui, system-ui, sans-serif)); }
.hdx-state-text { max-inline-size: 56ch; margin: 0 0 6px; color: var(--hdx-muted); }
.hdx-spinner { inline-size: 32px; block-size: 32px; border: 3px solid color-mix(in srgb, var(--vd-text) 14%, transparent); border-block-start-color: var(--vd-accent); border-radius: 50%; }
/* Confirm sheet (hd-sheet inside the app root) */
.hdx-app > .hd-sheet-layer { position: absolute; inset: 0; z-index: 20; display: grid; place-items: center; padding: 24px; background: var(--vd-theme-overlay-bg, color-mix(in srgb, var(--vd-text) 28%, transparent)); }
.hdx-app .hd-sheet { inline-size: min(440px, 100%); padding: 20px 22px 18px; border: 1px solid var(--vd-mat-card-border, var(--hdx-line)); border-radius: var(--hdx-radius); background: var(--vd-mat-card, var(--hdx-card)); box-shadow: var(--vd-mat-card-shadow, 0 18px 48px color-mix(in srgb, var(--vd-text) 22%, transparent)); color: var(--vd-text); }
.hdx-app .hd-sheet-title { margin: 0 0 8px; color: var(--vd-mat-heading-color, var(--vd-text)); font: var(--vd-mat-heading-weight, 650) 17px/1.3 var(--vd-mat-heading-font, var(--vd-font-ui, system-ui, sans-serif)); }
.hdx-app .hd-sheet-text { margin: 0 0 6px; color: var(--hdx-muted); }
.hdx-app .hd-sheet-foot { display: flex; justify-content: flex-end; gap: 8px; margin-block-start: 18px; }
.hdx-app .hd-sheet-btn { min-block-size: 36px; padding: 0 16px; border: 1px solid var(--vd-mat-btn-border, var(--hdx-line)); border-radius: var(--vd-mat-btn-radius, 8px); background: var(--vd-mat-btn, var(--vd-theme-control-bg)); color: var(--vd-mat-btn-text, var(--vd-text)); font-weight: 600; cursor: pointer; }
.hdx-app .hd-sheet-btn[data-tone="primary"] { border-color: transparent; background: var(--vd-mat-primary, var(--vd-accent)); color: var(--vd-mat-primary-text, var(--vd-accent-contrast)); }
.hdx-app .hd-sheet-btn[data-tone="danger"] { border-color: transparent; background: color-mix(in srgb, var(--ds-color-danger-fg) 84%, var(--vd-text)); color: var(--vd-theme-app-bg); }
@media (pointer: coarse) { .hdx-btn, .hdx-back, .hdx-app .hd-sheet-btn { min-block-size: 44px; } .hdx-card-delete { inline-size: 44px; block-size: 44px; } }
@container hdx (max-width: 520px) { .hdx-home-actions { inline-size: 100%; } .hdx-home-actions .hdx-btn { flex: 1 1 0; } .hdx-conn-text { display: none; } }
@media (prefers-reduced-motion: no-preference) {
  body:not([data-animations="false"]) .hdx-choice { transition: transform 140ms ease, border-color 140ms ease; }
  body:not([data-animations="false"]) .hdx-choice:hover:not(:disabled) { transform: translateY(-1px); }
  body:not([data-animations="false"]) .hdx-conn[data-conn="connecting"] .hdx-conn-dot, body:not([data-animations="false"]) .hdx-skel-card { animation: hdx-pulse 1.3s ease-in-out infinite; }
  body:not([data-animations="false"]) .hdx-spinner, body:not([data-animations="false"]) .hdx-choice[aria-busy="true"] .hdx-choice-icon::after { animation: hdx-spin .9s linear infinite; }
}
@keyframes hdx-pulse { 50% { opacity: .4; } }
@keyframes hdx-spin { to { transform: rotate(360deg); } }
```
- [ ] Run `npm run test:homedash` → host CSS case PASS.
- [ ] Detect-changes; commit `ui/css/desktop-app-homedash.css scripts/homedash-tests/host.test.mjs` — `feat(homedash): desktop chrome on the theme materials`.

**Done when:** guard PASS; no literal colour; every motion rule gated.

---

### Task 16: Desktop wiring and the wiring tests

**Files:** modify `ui/js/desktop/core/module-loader.js`, `desktop-foundation.js`, `menus-and-routing.js`, `window-shell-runtime.js`, `ui/js/desktop/bundles/main.bundle.js` (generated); create `ui/desktop_homedash_wiring_test.go`, `scripts/homedash-tests/wiring.test.mjs`.
**Read first:** `ui/desktop_docker_manager_wiring_test.go`; the four core files at the lines named below.

- [ ] Write the failing Go test `ui/desktop_homedash_wiring_test.go`:

```go
package ui

import (
	"regexp"
	"strings"
	"testing"
)

// The loader, the shell routing, the window sizing and the generated main bundle know HomeDash. The engine scripts load in the order of
// APP_ORDER in scripts/homedash-tests/harness.mjs, then the two desktop host files. Behaviour: npm run test:homedash and TestDesktopHomeDashBrowser.
func TestHomeDashWiring(t *testing.T) {
	t.Parallel()
	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	entry := regexp.MustCompile(`homedash: \{\s*styles: appStyles\('/css/homedash\.bundle\.css', '/css/desktop-app-homedash\.css'\),\s*scripts: \[([^\]]*)\]\s*\}`).FindStringSubmatch(loader)
	if entry == nil {
		t.Fatal("module-loader.js must register the homedash assets")
	}
	var scripts []string
	for _, match := range regexp.MustCompile(`'/js/([a-z/-]+\.js)'`).FindAllStringSubmatch(entry[1], -1) {
		scripts = append(scripts, match[1])
	}
	engine := []string{"hd-core", "hd-format", "hd-colour", "hd-grid", "hd-layout", "hd-doc", "hd-history", "hd-icons", "hd-geometry", "hd-prim-displays", "hd-prim-controls", "hd-widgets",
		"widgets/hd-w-misc", "widgets/hd-w-climate", "widgets/hd-w-light", "widgets/hd-w-sensor", "widgets/hd-w-switch",
		"hd-tone", "hd-api", "hd-stream", "hd-store", "hd-catalog", "hd-render", "hd-sheet", "hd-view", "hd-app"}
	var want []string
	for _, name := range engine {
		want = append(want, "homedash/"+name+".js")
	}
	want = append(want, "desktop/apps/homedash-home.js", "desktop/apps/homedash.js")
	if strings.Join(scripts, ",") != strings.Join(want, ",") {
		t.Fatalf("homedash scripts = %v, want %v", scripts, want)
	}
	if !strings.Contains(loader, "homedash: ['homedash']") {
		t.Fatal("module-loader.js must load the homedash translation section for the app")
	}
	for file, markers := range map[string][]string{
		"js/desktop/core/menus-and-routing.js":    {"appId === 'homedash' && window.HomeDashApp", "window.HomeDashApp.render(contentEl(id), id,"},
		"js/desktop/core/desktop-foundation.js":   {"homedash: 'HomeDashApp'"},
		"js/desktop/core/window-shell-runtime.js": {"homedash: { width: 1180, height: 780 }", "appId === 'homedash') return { width: 420, height: 360 }", "if (appId === 'homedash') return !(context && context.dashboard_id)"},
		"js/desktop/bundles/main.bundle.js":       {"appId === 'homedash' && window.HomeDashApp", "homedash: 'HomeDashApp'", "homedash: { width: 1180, height: 780 }", "if (appId === 'homedash') return !(context && context.dashboard_id)"},
	} {
		source := readDesktopAssetText(t, file)
		for _, marker := range markers {
			if !strings.Contains(source, marker) {
				t.Errorf("%s is missing %q (run npm run build:ui after editing the core files)", file, marker)
			}
		}
	}
}
```
- [ ] Write the failing Node test `scripts/homedash-tests/wiring.test.mjs`:

```js
import assert from 'node:assert/strict'; import { read, loadEngine, APP_ORDER } from './harness.mjs';
export const tests = [['wiring: the desktop loader runs the engine in APP_ORDER, then the two host files; the whole engine loads without a DOM', () => {
  const m = /homedash: \{\s*styles: appStyles\('\/css\/homedash\.bundle\.css', '\/css\/desktop-app-homedash\.css'\),\s*scripts: \[([^\]]*)\]\s*\}/.exec(read('ui/js/desktop/core/module-loader.js'));
  assert.ok(m, 'module-loader entry'); assert.deepEqual([...m[1].matchAll(/'([^']+)'/g)].map(x => x[1]), APP_ORDER.map(f => '/js/homedash/' + f).concat(['/js/desktop/apps/homedash-home.js', '/js/desktop/apps/homedash.js']));
  const { E } = loadEngine(APP_ORDER); for (const part of ['tone', 'api', 'stream', 'store', 'catalog', 'render', 'sheet', 'view']) assert.ok(E[part], part); assert.equal(typeof E.mount, 'function'); }]];
```
- [ ] Run `go test ./ui -run '^TestHomeDashWiring$' -count=1 -v` → FAIL ("must register the homedash assets"); `npm run test:homedash` → wiring FAIL.
- [ ] GitNexus impact for `appGlobalName`, `renderAppContent`, `appWindowSize`, `appWindowMinSize`, `matchesExistingAppWindow` (expect hub symbols with many callers; additive branches; report risk).
- [ ] `module-loader.js`: after the `ecalc` entry (the block ending at the line after `ecalc.js']`), add exactly:

```js
        homedash: {
            styles: appStyles('/css/homedash.bundle.css', '/css/desktop-app-homedash.css'),
            scripts: ['/js/homedash/hd-core.js', '/js/homedash/hd-format.js', '/js/homedash/hd-colour.js', '/js/homedash/hd-grid.js', '/js/homedash/hd-layout.js', '/js/homedash/hd-doc.js', '/js/homedash/hd-history.js', '/js/homedash/hd-icons.js', '/js/homedash/hd-geometry.js', '/js/homedash/hd-prim-displays.js', '/js/homedash/hd-prim-controls.js', '/js/homedash/hd-widgets.js', '/js/homedash/widgets/hd-w-misc.js', '/js/homedash/widgets/hd-w-climate.js', '/js/homedash/widgets/hd-w-light.js', '/js/homedash/widgets/hd-w-sensor.js', '/js/homedash/widgets/hd-w-switch.js', '/js/homedash/hd-tone.js', '/js/homedash/hd-api.js', '/js/homedash/hd-stream.js', '/js/homedash/hd-store.js', '/js/homedash/hd-catalog.js', '/js/homedash/hd-render.js', '/js/homedash/hd-sheet.js', '/js/homedash/hd-view.js', '/js/homedash/hd-app.js', '/js/desktop/apps/homedash-home.js', '/js/desktop/apps/homedash.js']
        },
```
and in `APP_I18N_SECTIONS` after `'docker-manager': ['docker_manager'],` add `homedash: ['homedash'],`.
- [ ] `desktop-foundation.js` `appGlobalName`: after `'docker-manager': 'DockerManagerApp',` add `homedash: 'HomeDashApp',`.
- [ ] `menus-and-routing.js` `renderAppContent`: directly after the `docker-manager` block (ends at the line after `readonly: desktopReadonly()` / `}));` / `}`), add:

```js
        if (appId === 'homedash' && window.HomeDashApp) {
            return window.HomeDashApp.render(contentEl(id), id, Object.assign({}, context || {}, {
                esc, t, api, setWindowMenus, clearWindowMenus, readonly: desktopReadonly(),
                updateWindowContext: (windowId, patch) => { updateWindowContext(windowId, patch); scheduleSessionPersist(); }
            }));
        }
```
- [ ] `window-shell-runtime.js`: in `appWindowSize` presets after `'docker-manager': { width: 1180, height: 760 },` add `homedash: { width: 1180, height: 780 },`; in `appWindowMinSize` after the docker-manager line add `if (appId === 'homedash') return { width: 420, height: 360 };`; in `matchesExistingAppWindow`, directly before the final `return !['editor', …` add `if (appId === 'homedash') return !(context && context.dashboard_id) || !!(win.context && win.context.dashboard_id === context.dashboard_id);`.
- [ ] `npm run build:ui`; `npm run check:ui` → exit 0.
- [ ] Run `go test ./ui -run '(?i)^(TestHomeDashWiring|TestDockerManagerWiring|TestDesktopAppMaterialsAdoptedByApps|TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget)$' -count=1 -v` → `TestHomeDashWiring`, `TestDockerManagerWiring`, `TestDesktopAppMaterialsAdoptedByApps` PASS; the line-budget test shows only the baseline failures from Task 0 (no HomeDash file). `npm run test:homedash` → all PASS. `go vet ./ui` → ok.
- [ ] Detect-changes; commit the four core files, `main.bundle.js`, `ui/desktop_homedash_wiring_test.go`, `scripts/homedash-tests/wiring.test.mjs` — `feat(homedash): register the desktop app in loader, routing and window shell`.

**Done when:** both wiring tests PASS and the bundle check is clean.

---

### Task 17: BuiltinApps row and the app icon

**Files:** modify `internal/desktop/types.go` (`BuiltinApps`); create `ui/img/desktop-icons/homedash.svg`, `internal/desktop/homedash_app_test.go`.
**Read first:** `internal/desktop/ecalc_app_test.go` (app-owned artwork through `logo_path` needs no themed icon entry), `docker_manager_app_test.go` (capability gating); `ui/img/desktop-icons/ecalc.svg`, `synth-studio.svg`.

- [ ] Write the failing test `internal/desktop/homedash_app_test.go`:

```go
package desktop

import (
	"os"
	"path/filepath"
	"testing"
)

// HomeDash is a comms app shown only while the server reports the "homedash" capability. Its icon is app-owned artwork (logo_path), like
// ECalc's, so it needs no entry in the themed icon sets.
func TestBuiltinHomeDashAppIsACommsAppGatedByItsCapability(t *testing.T) {
	app := testFindApp(t, BuiltinApps(), "homedash")
	if app.Entry != "builtin://homedash" || app.Name != "HomeDash" || app.Category != "comms" || app.Runtime != BuiltinRuntime {
		t.Fatalf("homedash manifest = %+v", app)
	}
	if app.Icon != "homedash" || app.Metadata["logo_path"] != "/img/desktop-icons/homedash.svg" {
		t.Fatalf("homedash icon = %q, metadata = %v", app.Icon, app.Metadata)
	}
	if len(app.Requires) != 1 || app.Requires[0] != "homedash" {
		t.Fatalf("homedash requires = %v, want [homedash]", app.Requires)
	}
	if !app.Builtin || !app.StartVisible || !app.DockVisible || app.Deletable || app.Internal {
		t.Fatalf("homedash must be a visible, permanent builtin: %+v", app)
	}
	if got := FilterAvailableApps([]AppManifest{app}, staticCapabilities{}); len(got) != 0 {
		t.Fatalf("without the capability the app must stay hidden: %+v", got)
	}
	if got := FilterAvailableApps([]AppManifest{app}, staticCapabilities{"homedash": true}); len(got) != 1 {
		t.Fatalf("with the capability the app must show: %+v", got)
	}
	if _, err := os.Stat(filepath.Join("..", "..", "ui", "img", "desktop-icons", "homedash.svg")); err != nil {
		t.Fatalf("icon artwork missing: %v", err)
	}
}
```
- [ ] Run `go test ./internal/desktop -run '^TestBuiltinHomeDashAppIsACommsAppGatedByItsCapability$' -count=1 -v` → FAIL (`app "homedash" not found` from `testFindApp`).
- [ ] GitNexus impact for `BuiltinApps` (hub symbol; HIGH/CRITICAL likely; the change only appends one row; warn and proceed).
- [ ] In `BuiltinApps()` after the `network-cameras` row add:

```go
		{ID: "homedash", Name: "HomeDash", Version: "1.0.0", Icon: "homedash", Entry: "builtin://homedash", Runtime: BuiltinRuntime, Category: "comms", Description: "Live dashboards for your home and this server: sensors, switches and system values at a glance.", Requires: []string{"homedash"}, Metadata: map[string]string{"logo_path": "/img/desktop-icons/homedash.svg"}},
```
- [ ] Create `ui/img/desktop-icons/homedash.svg` (visual language of `ecalc.svg`/`synth-studio.svg`: 128 px canvas, 112 px rounded square with radius 27 and a diagonal two-stop gradient, a white glyph, two accent colours, one signal dot). Shapes: a dusk-blue tile (`#4f7cff` → `#1d2766`); a white roof line over a deep-navy dashboard panel with a pale-blue rim; inside the panel an amber gauge arc on a dim track, a teal toggle pill with a white knob and a teal sparkline; a green "live" dot with a white ring top right.

```svg
<svg xmlns="http://www.w3.org/2000/svg" width="128" height="128" viewBox="0 0 128 128" role="img" aria-label="HomeDash">
  <defs>
    <linearGradient id="g" x1="18" y1="15" x2="108" y2="115" gradientUnits="userSpaceOnUse">
      <stop stop-color="#4f7cff"/>
      <stop offset="1" stop-color="#1d2766"/>
    </linearGradient>
  </defs>
  <rect x="8" y="8" width="112" height="112" rx="27" fill="url(#g)"/>
  <path d="M24 58 64 27l40 31" fill="none" stroke="#ffffff" stroke-width="7" stroke-linecap="round" stroke-linejoin="round"/>
  <rect x="32" y="55" width="64" height="47" rx="9" fill="#0f1733" stroke="#9db5ff" stroke-width="3"/>
  <path d="M42 82a11 11 0 0 1 22 0" fill="none" stroke="#33427a" stroke-width="5" stroke-linecap="round"/>
  <path d="M42 82a11 11 0 0 1 17.5-8.9" fill="none" stroke="#ffcb73" stroke-width="5" stroke-linecap="round"/>
  <rect x="70" y="65" width="18" height="10" rx="5" fill="#64f0df"/>
  <circle cx="83" cy="70" r="3.5" fill="#ffffff"/>
  <path d="M40 95l7-3.5 6 2.5 7-5 6 3.5 7-4.5 7 3 6-2" fill="none" stroke="#64f0df" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"/>
  <circle cx="98" cy="28" r="7" fill="#4ad29c" stroke="#ffffff" stroke-width="2.5"/>
</svg>
```
- [ ] Run `go test ./internal/desktop -count=1` → ok (includes `TestBuiltinAppsCarryStartMenuCategories`). Run `go test ./ui -run '(?i)^(TestDesktopStartMenuCategoriesStayInSync|TestHomeDashWiring)$' -count=1` → ok.
- [ ] Detect-changes; commit `internal/desktop/types.go internal/desktop/homedash_app_test.go ui/img/desktop-icons/homedash.svg` — `feat(homedash): builtin app row gated by the homedash capability, app icon`.

**Done when:** the app test passes; no themed manifest or `desktopPreferredIconNames` change (ECalc precedent).

---

### Task 18: TestDesktopHomeDashBrowser and the contract fixture

**Files:** create `ui/desktop_homedash_fixture_test.go`, `ui/desktop_homedash_browser_test.go`.
**Read first:** `ui/desktop_audit_browser_test.go:21-79` (`desktopAuditBrowserWithAPI`, `fixtureWords`, `auditTest`); `ui/desktop_docker_manager_browser_test.go:168-240` (SSE fixture); contract (whole); memory notes on Chrome launch (binary outside the repo, PowerShell, sandbox off).

- [ ] Create `ui/desktop_homedash_fixture_test.go`:

```go
package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

var homeDashIDPattern = regexp.MustCompile(`^d_[a-z2-7]{12}$`)

// homeDashFixture answers /api/desktop/homedash/ like the M2 contract (docs/superpowers/plans/2026-11-25-homedash-m2-backend.md, "API contract
// (frozen for M2)"): bootstrap, widget-catalog (golden JSON, ETag, 304), entities, entity, history, dashboards with the system template and
// the empty document, dashboard GET and DELETE, the SSE stream (hello, snapshot on every PUT that adds ids, state, dashboard_changed, hb) and
// PUT stream/subscriptions. Controls let a test push values, change a dashboard, drop the streams, refuse reconnects and go read-only.
type homeDashFixture struct {
	mu         sync.Mutex
	catalog    []byte
	hash       string
	starter    map[string]any
	entities   []map[string]any
	values     map[string]float64
	dashboards map[string]map[string]any
	seq        int
	streams    map[string]*homeDashStream
	opened     int
	refuse     int
	refused    int
	readonly   bool
	requests   []string
	bodies     map[string]string
}

type homeDashStream struct {
	id     string
	frames chan string
	done   chan struct{}
	subs   map[string]bool
}

func (s *homeDashStream) send(frame string) {
	select {
	case s.frames <- frame:
	default:
	}
}

func newHomeDashFixture(t *testing.T) *homeDashFixture {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "internal", "homedash", "testdata", "widget_catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	var head struct {
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal(raw, &head); err != nil || head.Hash == "" {
		t.Fatalf("golden catalog: %v", err)
	}
	f := &homeDashFixture{catalog: raw, hash: head.Hash, values: map[string]float64{}, dashboards: map[string]map[string]any{}, streams: map[string]*homeDashStream{}, bodies: map[string]string{}}
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "testdata/homedash-m2/starter-dashboard.json")), &f.starter); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "testdata/homedash-m2/system-entities.json")), &f.entities); err != nil {
		t.Fatal(err)
	}
	for _, e := range f.entities {
		if st, ok := e["state"].(map[string]any); ok {
			if v, ok := st["value"].(float64); ok {
				f.values[e["id"].(string)] = v
			}
		}
	}
	return f
}

func homeDashJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func homeDashError(w http.ResponseWriter, status int, code, message string) {
	homeDashJSON(w, status, map[string]any{"status": "error", "code": code, "message": message})
}

func homeDashReadonly(w http.ResponseWriter) {
	homeDashJSON(w, http.StatusForbidden, map[string]any{"error": "desktop_readonly", "code": "desktop_readonly", "message": "the desktop is read-only"})
}

func homeDashFrame(event string, data any) string {
	raw, _ := json.Marshal(data)
	return "event: " + event + "\ndata: " + string(raw) + "\n\n"
}

func homeDashNow() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func homeDashSource(state string) map[string]any {
	return map[string]any{"source": "system", "state": state, "at": homeDashNow()}
}

func homeDashID(n int) string {
	const abc = "abcdefghijklmnopqrstuvwxyz234567"
	b := []byte("d_aaaaaaaaaaaa")
	for i := len(b) - 1; n > 0 && i >= 2; i-- {
		b[i] = abc[n%32]
		n /= 32
	}
	return string(b)
}

func homeDashSlug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	if s := strings.TrimRight(b.String(), "-"); s != "" {
		return s
	}
	return "dashboard"
}

func homeDashClone(v map[string]any) map[string]any {
	raw, _ := json.Marshal(v)
	out := map[string]any{}
	_ = json.Unmarshal(raw, &out)
	return out
}

func homeDashInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	}
	return 0
}

func (f *homeDashFixture) uniqueSlugLocked(base string) string {
	used := map[string]bool{}
	for _, d := range f.dashboards {
		if s, ok := d["slug"].(string); ok {
			used[s] = true
		}
	}
	if !used[base] {
		return base
	}
	for i := 2; ; i++ {
		if s := fmt.Sprintf("%s-%d", base, i); !used[s] {
			return s
		}
	}
}

func (f *homeDashFixture) entityLocked(id string) map[string]any {
	for _, e := range f.entities {
		if e["id"] == id {
			out := map[string]any{}
			for k, v := range e {
				out[k] = v
			}
			out["state"] = map[string]any{"value": f.values[id], "available": true, "updated_at": homeDashNow()}
			return out
		}
	}
	return map[string]any{"id": id, "source": strings.SplitN(id, ":", 2)[0], "kind": "generic", "name": id, "capabilities": []any{}, "readonly": true,
		"state": map[string]any{"value": nil, "available": false, "updated_at": homeDashNow()}}
}

func (f *homeDashFixture) broadcastLocked(frame string) {
	for _, s := range f.streams {
		s.send(frame)
	}
}

func (f *homeDashFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	path := strings.TrimPrefix(r.URL.Path, "/api/desktop/homedash/")
	f.mu.Lock()
	f.requests = append(f.requests, r.Method+" "+r.URL.RequestURI())
	f.bodies[r.Method+" "+path] = string(raw)
	f.mu.Unlock()
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method + " " + path {
	case "GET bootstrap":
		f.mu.Lock()
		ro := f.readonly
		f.mu.Unlock()
		homeDashJSON(w, 200, map[string]any{"status": "ok", "api_version": 1, "stream_protocol": 1, "schema_version": 1, "catalog_hash": f.hash, "readonly": false, "desktop_readonly": ro, "kiosk_enabled": false,
			"limits":    map[string]any{"dashboards": 50, "subscriptions": 300, "pages": 12, "widgets_per_page": 120, "widgets": 400, "document_bytes": 262144, "history_points": 500, "history_range_seconds": 604800},
			"templates": []string{"system"}, "sources": []any{homeDashSource("idle")}})
	case "GET widget-catalog":
		w.Header().Set("ETag", `"`+f.hash+`"`)
		if strings.Contains(r.Header.Get("If-None-Match"), f.hash) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = io.WriteString(w, `{"status":"ok","catalog":`+string(f.catalog)+`}`)
	case "GET entities":
		f.mu.Lock()
		list := []any{}
		for _, e := range f.entities {
			list = append(list, f.entityLocked(e["id"].(string)))
		}
		f.mu.Unlock()
		homeDashJSON(w, 200, map[string]any{"status": "ok", "entities": list, "total": len(list), "limit": 100, "offset": 0, "sources": []any{homeDashSource("polling")}})
	case "GET entity":
		id := r.URL.Query().Get("id")
		f.mu.Lock()
		_, known := f.values[id]
		e := f.entityLocked(id)
		f.mu.Unlock()
		if !known {
			homeDashError(w, 404, "entity_not_found", "unknown entity")
			return
		}
		homeDashJSON(w, 200, map[string]any{"status": "ok", "entity": e})
	case "GET history":
		f.history(w, r)
	case "GET stream":
		f.stream(w, r)
	case "PUT stream/subscriptions":
		f.subscribe(w, raw)
	case "GET dashboards":
		f.list(w)
	case "POST dashboards":
		f.create(w, raw)
	case "GET dashboard":
		f.get(w, r)
	case "DELETE dashboard":
		f.remove(w, r)
	default:
		switch path {
		case "bootstrap", "widget-catalog", "entities", "entity", "history", "stream", "stream/subscriptions", "dashboards", "dashboard":
			homeDashError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		default:
			homeDashError(w, http.StatusNotFound, "not_found", "no route "+path)
		}
	}
}

func (f *homeDashFixture) history(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	id, to, points := q.Get("id"), time.Now().UTC(), 120
	from := to.Add(-24 * time.Hour)
	var err error
	if v := q.Get("to"); v != "" {
		if to, err = time.Parse(time.RFC3339Nano, v); err != nil {
			homeDashError(w, 400, "bad_request", "bad to")
			return
		}
	}
	if v := q.Get("from"); v != "" {
		if from, err = time.Parse(time.RFC3339Nano, v); err != nil {
			homeDashError(w, 400, "bad_request", "bad from")
			return
		}
	}
	if v := q.Get("points"); v != "" {
		if points, err = strconv.Atoi(v); err != nil || points < 1 || points > 500 {
			homeDashError(w, 400, "bad_request", "bad points")
			return
		}
	}
	if !to.After(from) || to.Sub(from) > 7*24*time.Hour {
		homeDashError(w, 400, "bad_request", "bad range")
		return
	}
	f.mu.Lock()
	base, known := f.values[id]
	f.mu.Unlock()
	if !known {
		homeDashError(w, 404, "entity_not_found", "unknown entity")
		return
	}
	out := []map[string]any{}
	if id != "system:uptime" {
		step := to.Sub(from) / time.Duration(points)
		for i := 0; i < points; i++ {
			v := math.Round((base+base*0.35*math.Sin(float64(i)/7)+base*0.1*math.Sin(float64(i)*1.7))*10) / 10
			out = append(out, map[string]any{"t": from.Add(step*time.Duration(i) + step/2).Format(time.RFC3339Nano), "v": math.Max(0, v)})
		}
	}
	homeDashJSON(w, 200, map[string]any{"status": "ok", "id": id, "origin": "source", "points": out})
}

func (f *homeDashFixture) stream(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	if f.refuse > 0 {
		f.refuse--
		f.refused++
		f.mu.Unlock()
		homeDashError(w, http.StatusTooManyRequests, "too_many_streams", "too many streams")
		return
	}
	f.opened++
	s := &homeDashStream{id: fmt.Sprintf("%032x", f.opened), frames: make(chan string, 64), done: make(chan struct{}), subs: map[string]bool{}}
	f.streams[s.id] = s
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		if f.streams[s.id] == s {
			delete(f.streams, s.id)
		}
		f.mu.Unlock()
	}()
	flusher, ok := w.(http.Flusher)
	if !ok {
		homeDashError(w, 500, "internal_error", "no flusher")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	write := func(frame string) bool {
		if _, err := io.WriteString(w, frame); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	if !write(homeDashFrame("hello", map[string]any{"stream_id": s.id, "protocol": 1, "heartbeat_ms": 15000, "limits": map[string]any{"entities": 300}, "server_time": homeDashNow(), "principal": "session"})) {
		return
	}
	hb := time.NewTicker(15 * time.Second)
	defer hb.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.done:
			return
		case frame := <-s.frames:
			if !write(frame) {
				return
			}
		case <-hb.C:
			if !write(homeDashFrame("hb", map[string]any{"server_time": homeDashNow()})) {
				return
			}
		}
	}
}

func (f *homeDashFixture) subscribe(w http.ResponseWriter, raw []byte) {
	var body struct {
		StreamID string   `json:"stream_id"`
		Entities []string `json:"entities"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		homeDashError(w, 400, "bad_request", "bad json")
		return
	}
	f.mu.Lock()
	s := f.streams[body.StreamID]
	if s == nil {
		f.mu.Unlock()
		homeDashError(w, 404, "stream_not_found", "unknown stream")
		return
	}
	accepted, rejected, added, next := []string{}, []map[string]any{}, []any{}, map[string]bool{}
	for _, id := range body.Entities {
		if next[id] {
			continue
		}
		if !strings.Contains(id, ":") {
			rejected = append(rejected, map[string]any{"id": id, "reason": "invalid_id"})
			continue
		}
		if !strings.HasPrefix(id, "system:") {
			rejected = append(rejected, map[string]any{"id": id, "reason": "unknown_source"})
			continue
		}
		next[id] = true
		accepted = append(accepted, id)
		if !s.subs[id] {
			added = append(added, f.entityLocked(id))
		}
	}
	s.subs = next
	if len(added) > 0 {
		s.send(homeDashFrame("snapshot", map[string]any{"entities": added, "sources": []any{homeDashSource("polling")}}))
	}
	f.mu.Unlock()
	homeDashJSON(w, 200, map[string]any{"status": "ok", "accepted": accepted, "rejected": rejected})
}

func (f *homeDashFixture) list(w http.ResponseWriter) {
	f.mu.Lock()
	out := []map[string]any{}
	for _, d := range f.dashboards {
		out = append(out, map[string]any{"id": d["id"], "slug": d["slug"], "name": d["name"], "revision": d["revision"], "approved_revision": d["approved_revision"], "updated_at": d["updated_at"], "saved_by": d["saved_by"]})
	}
	f.mu.Unlock()
	sort.Slice(out, func(i, j int) bool {
		a, b := strings.ToLower(out[i]["name"].(string)), strings.ToLower(out[j]["name"].(string))
		if a != b {
			return a < b
		}
		return out[i]["id"].(string) < out[j]["id"].(string)
	})
	homeDashJSON(w, 200, map[string]any{"status": "ok", "dashboards": out})
}

func (f *homeDashFixture) create(w http.ResponseWriter, raw []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.readonly {
		homeDashReadonly(w)
		return
	}
	var body struct {
		Template string         `json:"template"`
		Name     string         `json:"name"`
		Document map[string]any `json:"document"`
	}
	if json.Unmarshal(raw, &body) != nil || (body.Template == "") == (body.Document == nil) {
		homeDashError(w, 400, "bad_request", "send a template or a document")
		return
	}
	var doc map[string]any
	if body.Template != "" {
		if body.Template != "system" {
			homeDashError(w, 400, "bad_request", "unknown template")
			return
		}
		doc = homeDashClone(f.starter)
		doc["name"] = "System"
		if body.Name != "" {
			doc["name"] = body.Name
		}
		doc["slug"] = f.uniqueSlugLocked("system")
	} else {
		name, _ := body.Document["name"].(string)
		pages, _ := body.Document["pages"].([]any)
		if name == "" || len(pages) == 0 {
			homeDashJSON(w, 422, map[string]any{"status": "error", "code": "invalid_document", "message": "invalid document", "issues": []any{map[string]any{"path": "/pages", "code": "required", "message": "at least one page"}}})
			return
		}
		page, _ := pages[0].(map[string]any)
		doc = map[string]any{"schema_version": 1, "name": name, "slug": f.uniqueSlugLocked(homeDashSlug(name)),
			"look":  map[string]any{"family": "glass", "palette": map[string]any{"mode": "theme"}, "backdrop": map[string]any{"kind": "theme"}, "density": "comfortable", "radius_scale": 1},
			"grid":  map[string]any{"row_unit": "auto", "gap": nil, "fit": "scroll"},
			"pages": []any{map[string]any{"id": page["id"], "name": page["name"], "compact": true, "widgets": []any{}, "layouts": map[string]any{"lg": []any{}}}},
			"kiosk": map[string]any{"start_page": page["id"], "idle_dim_seconds": 120, "idle_dim_level": 0.3, "page_rotation_seconds": 0, "screen_saver": "clock", "show_clock": true}}
	}
	f.seq++
	id := homeDashID(f.seq)
	doc["id"], doc["revision"], doc["approved_revision"], doc["updated_at"], doc["saved_by"] = id, 1, 1, homeDashNow(), "user"
	f.dashboards[id] = doc
	f.broadcastLocked(homeDashFrame("dashboard_changed", map[string]any{"id": id, "revision": 1, "approved_revision": 1}))
	homeDashJSON(w, 201, map[string]any{"status": "ok", "dashboard": doc, "warnings": []any{}})
}

func (f *homeDashFixture) get(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if !homeDashIDPattern.MatchString(id) {
		homeDashError(w, 400, "bad_request", "bad id")
		return
	}
	f.mu.Lock()
	var doc map[string]any
	if d := f.dashboards[id]; d != nil {
		doc = homeDashClone(d)
	}
	f.mu.Unlock()
	if doc == nil {
		homeDashError(w, 404, "dashboard_not_found", "no such dashboard")
		return
	}
	homeDashJSON(w, 200, map[string]any{"status": "ok", "dashboard": doc, "schema_too_new": false})
}

func (f *homeDashFixture) remove(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	id := q.Get("id")
	rev, err := strconv.Atoi(q.Get("revision"))
	if !homeDashIDPattern.MatchString(id) || err != nil || rev < 1 {
		homeDashError(w, 400, "bad_request", "bad id or revision")
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.readonly {
		homeDashReadonly(w)
		return
	}
	doc := f.dashboards[id]
	if doc == nil {
		homeDashError(w, 404, "dashboard_not_found", "no such dashboard")
		return
	}
	if cur := homeDashInt(doc["revision"]); cur != rev {
		homeDashJSON(w, 409, map[string]any{"status": "error", "code": "revision_conflict", "message": "stale revision", "current_revision": cur})
		return
	}
	delete(f.dashboards, id)
	f.broadcastLocked(homeDashFrame("dashboard_changed", map[string]any{"id": id, "revision": 0, "approved_revision": 0}))
	homeDashJSON(w, 200, map[string]any{"status": "ok", "id": id})
}

// Test controls.
func (f *homeDashFixture) pushState(id string, value float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.values[id] = value
	frame := homeDashFrame("state", map[string]any{"changes": map[string]any{id: map[string]any{"value": value, "available": true, "updated_at": homeDashNow()}}})
	for _, s := range f.streams {
		if s.subs[id] {
			s.send(frame)
		}
	}
}

func (f *homeDashFixture) setFamily(t *testing.T, id, family string) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	doc := f.dashboards[id]
	if doc == nil {
		t.Fatalf("no dashboard %s", id)
	}
	doc["look"].(map[string]any)["family"] = family
	rev := homeDashInt(doc["revision"]) + 1
	doc["revision"], doc["approved_revision"], doc["updated_at"] = rev, rev, homeDashNow()
	f.broadcastLocked(homeDashFrame("dashboard_changed", map[string]any{"id": id, "revision": rev, "approved_revision": rev}))
}

func (f *homeDashFixture) dropStreams(refuseNext int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refuse = refuseNext
	for id, s := range f.streams {
		close(s.done)
		delete(f.streams, id)
	}
}

func (f *homeDashFixture) setReadonly(on bool) { f.mu.Lock(); f.readonly = on; f.mu.Unlock() }

func (f *homeDashFixture) counts() (opened, refused int) { f.mu.Lock(); defer f.mu.Unlock(); return f.opened, f.refused }

func (f *homeDashFixture) body(route string) string { f.mu.Lock(); defer f.mu.Unlock(); return f.bodies[route] }

func (f *homeDashFixture) findRequest(match func(string) bool) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.requests {
		if match(r) {
			return r
		}
	}
	return ""
}

func (f *homeDashFixture) idByName(name string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id, d := range f.dashboards {
		if d["name"] == name {
			return id
		}
	}
	return ""
}

func (f *homeDashFixture) waitSubscribed(t *testing.T, n int, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		for _, s := range f.streams {
			if len(s.subs) == n {
				f.mu.Unlock()
				return
			}
		}
		f.mu.Unlock()
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("no stream holds %d subscribed ids", n)
}
```
- [ ] Create `ui/desktop_homedash_browser_test.go`:

```go
package ui

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDesktopHomeDashBrowser opens HomeDash from the start menu against the M2 contract fixture: empty state, starter dashboard, live values
// and tones, sparkline history, a family change through dashboard_changed, a dropped stream with the stale banner and the reconnect, an
// empty dashboard, deleting it through the confirm sheet, and the read-only desktop. HOMEDASH_M2_SHOTS=1 also writes the lead's review
// screenshots to reports/homedash-m2/.
func TestDesktopHomeDashBrowser(t *testing.T) {
	fixture := newHomeDashFixture(t)
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/desktop/homedash/": fixture}).CancelTimeout().Timeout(5 * time.Minute)
	page.MustEval(`(words)=>Object.assign(window.fixtureWords, JSON.parse(words))`, readDesktopAssetText(t, "lang/homedash/de.json"))
	word := func(key string) string { return page.MustEval(`(k)=>window.fixtureWords[k]||''`, key).Str() }
	shots := os.Getenv("HOMEDASH_M2_SHOTS") == "1"
	theme := func(name, mode string) {
		page.MustEval(`(theme,mode)=>{const a=auditTest;a.state.bootstrap.settings['appearance.theme']=theme;a.state.bootstrap.settings['appearance.fruity_mode']=mode;document.documentElement.dataset.theme=mode;a.applyDesktopSettings();}`, name, mode)
	}
	shot := func(name string) {
		if !shots {
			return
		}
		dir := filepath.Join("..", "reports", "homedash-m2")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		time.Sleep(600 * time.Millisecond)
		if err := os.WriteFile(filepath.Join(dir, name+".png"), page.MustElement(`.vd-window[data-app-id="homedash"]`).MustScreenshot(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	both := func(name string) {
		if !shots {
			return
		}
		theme("fruity", "light")
		shot(name + "-fruity-light")
		theme("standard", "dark")
		shot(name + "-standard-dark")
	}
	theme("standard", "dark")
	const root = `document.querySelector('.hdx-view .hd-root')`

	// 1. Start menu, category comms.
	page.MustEval(`()=>{if(document.getElementById('vd-start-menu').hidden)document.getElementById('vd-start-button').click();}`)
	page.MustWait(`()=>!document.getElementById('vd-start-menu').hidden&&!!document.querySelector('#vd-start-rail .vd-start-category[data-category="comms"]')`)
	page.MustEval(`()=>document.querySelector('#vd-start-rail .vd-start-category[data-category="comms"]').click()`)
	page.MustWait(`()=>!!document.querySelector('.vd-start-item[data-app-id="homedash"]')`)
	page.MustEval(`()=>document.querySelector('.vd-start-item[data-app-id="homedash"]').click()`)

	// 2. First-run empty state.
	page.MustWait(`()=>!!document.querySelector('.hdx-app[data-hdx-view="home"] .hdx-home[data-hdx-mode="empty"]')`)
	if got := page.MustEval(`()=>document.querySelector('.hdx-empty-title').textContent`).Str(); got != word("homedash.home.empty_title") {
		t.Fatalf("empty state title = %q", got)
	}
	both("empty")

	// 3. Starter dashboard: six widgets in Glass, subscribed on the stream.
	page.MustEval(`()=>document.querySelector('.hdx-empty [data-action="create-starter"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('.hdx-view .hd-root .hd-page > .hd-widget').length===6`)
	if body := fixture.body("POST dashboards"); !strings.Contains(body, `"template":"system"`) || !strings.Contains(body, `"name":"`+word("homedash.home.starter_name")+`"`) {
		t.Fatalf("starter request body = %s", body)
	}
	starterID := fixture.idByName(word("homedash.home.starter_name"))
	page.MustWait(`()=>` + root + `.dataset.hdFamily==='glass'&&` + root + `.dataset.hdConn==='live'`)
	fixture.waitSubscribed(t, 6, 10*time.Second)
	page.MustWait(`()=>!!document.querySelector('[data-hd-wid="memory"] .hd-gauge[aria-valuenow="50"]')`)
	// SCR-M2-1: the uptime tile reads as a duration.
	page.MustWait(`()=>{const n=document.querySelector('[data-hd-wid="uptime"] .hd-num-int');return !!n&&n.textContent!==''&&!/^\d+$/.test(n.textContent);}`)

	// 4. Live values and host tones.
	fixture.pushState("system:cpu", 97)
	page.MustWait(`()=>{const w=document.querySelector('[data-hd-wid="cpu"]');return w.dataset.hdTone==='crit'&&!!w.querySelector('.hd-gauge[aria-valuenow="97"]');}`)
	fixture.pushState("system:cpu", 42)
	page.MustWait(`()=>{const w=document.querySelector('[data-hd-wid="cpu"]');return !w.hasAttribute('data-hd-tone')&&!!w.querySelector('.hd-gauge[aria-valuenow="42"]');}`)

	// 5. Sparkline history over the widget's hours option (1 h).
	page.MustWait(`()=>{const s=document.querySelector('[data-hd-wid="net-rx"] svg.hd-spark'),l=s&&s.querySelector('.hd-spark-line');return !!l&&!s.hasAttribute('data-hd-empty')&&(l.getAttribute('d')||'').length>40;}`)
	req := fixture.findRequest(func(r string) bool { return strings.Contains(r, "/history?") && strings.Contains(r, "id=system%3Anet_rx") })
	u, err := url.ParseRequestURI(strings.TrimPrefix(req, "GET "))
	if err != nil {
		t.Fatalf("history request %q: %v", req, err)
	}
	from, _ := time.Parse(time.RFC3339Nano, u.Query().Get("from"))
	to, _ := time.Parse(time.RFC3339Nano, u.Query().Get("to"))
	if span := to.Sub(from); span < 3595*time.Second || span > 3605*time.Second {
		t.Fatalf("history span = %v, want the widget's 1 h", span)
	}
	// SCR-M2-2: a newer live value extends the line.
	before := page.MustEval(`()=>document.querySelector('[data-hd-wid="net-rx"] .hd-spark-line').getAttribute('d')`).Str()
	fixture.pushState("system:net_rx", 88.5)
	page.MustWait(`(before)=>document.querySelector('[data-hd-wid="net-rx"] .hd-spark-line').getAttribute('d')!==before`, before)
	both("starter-glass")

	// 6. dashboard_changed re-renders in another family; screenshots of every family.
	for _, family := range []string{"tactile", "minimal", "instrument", "glass"} {
		fixture.setFamily(t, starterID, family)
		page.MustWait(`(f)=>` + root + `.dataset.hdFamily===f&&document.querySelectorAll('.hdx-view .hd-page > .hd-widget').length===6`, family)
		if family != "glass" {
			both("starter-" + family)
		}
	}

	// 7. Stream drop: stale banner after the 5 s grace, then the reconnect clears it.
	fixture.dropStreams(2)
	page.Timeout(20*time.Second).MustWait(`()=>!!document.querySelector('.hdx-banner[data-banner="offline"]')&&` + root + `.dataset.hdConn==='offline'&&!!document.querySelector('[data-hd-wid="cpu"][data-hd-stale="true"]')`)
	shot("stale-standard-dark")
	page.Timeout(40*time.Second).MustWait(`()=>!document.querySelector('.hdx-banner[data-banner="offline"]')&&` + root + `.dataset.hdConn==='live'&&!document.querySelector('[data-hd-wid="cpu"][data-hd-stale="true"]')`)
	if opened, refused := fixture.counts(); opened < 2 || refused != 2 {
		t.Fatalf("streams opened %d, refused %d; want a reconnect after two refusals", opened, refused)
	}
	fixture.pushState("system:cpu", 55)
	page.MustWait(`()=>!!document.querySelector('[data-hd-wid="cpu"] .hd-gauge[aria-valuenow="55"]')`)

	// 8. Home list, an empty dashboard, delete through the confirm sheet.
	page.MustEval(`()=>document.querySelector('.hdx-back').click()`)
	page.MustWait(`()=>document.querySelectorAll('.hdx-home[data-hdx-mode="list"] .hdx-card').length===1`)
	page.MustEval(`()=>document.querySelector('.hdx-home-head [data-action="create-empty"]').click()`)
	page.MustWait(`()=>!!document.querySelector('.hdx-view .hd-page-empty')`)
	if body := fixture.body("POST dashboards"); !strings.Contains(body, `"document"`) || !strings.Contains(body, `"name":"`+word("homedash.home.empty_name")+`"`) || !strings.Contains(body, `"id":"main"`) {
		t.Fatalf("empty dashboard request body = %s", body)
	}
	shot("empty-dashboard-standard-dark")
	emptyID := fixture.idByName(word("homedash.home.empty_name"))
	page.MustEval(`()=>document.querySelector('.hdx-back').click()`)
	page.MustWait(`()=>document.querySelectorAll('.hdx-home[data-hdx-mode="list"] .hdx-card').length===2`)
	both("list")
	page.MustEval(`(id)=>document.querySelector('.hdx-card-delete[data-id="'+id+'"]').click()`, emptyID)
	page.MustWait(`()=>{const l=document.querySelector('.hdx-app > .hd-sheet-layer');return !!l&&!l.hidden&&document.activeElement===l.querySelector('[data-action="cancel"]');}`)
	shot("delete-confirm-standard-dark")
	page.MustEval(`()=>document.querySelector('.hdx-app > .hd-sheet-layer [data-action="confirm"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('.hdx-home[data-hdx-mode="list"] .hdx-card').length===1`)
	if req := fixture.findRequest(func(r string) bool { return strings.HasPrefix(r, "DELETE ") }); !strings.Contains(req, "id="+emptyID) || !strings.Contains(req, "revision=1") {
		t.Fatalf("delete request = %q", req)
	}

	// 9. Read-only desktop: no creation, no delete, a read-only banner on the dashboard.
	fixture.setReadonly(true)
	page.MustEval(`()=>document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{readonly:true}}))`)
	page.MustWait(`()=>!document.querySelector('.hdx-readonly-note').hidden&&document.querySelector('.hdx-home-head [data-action="create-starter"]').hidden&&!document.querySelector('.hdx-card-delete')`)
	both("readonly-home")
	page.MustEval(`()=>document.querySelector('.hdx-card-open').click()`)
	page.MustWait(`()=>!!document.querySelector('.hdx-banner[data-banner="readonly"]')&&document.querySelectorAll('.hdx-view .hd-page > .hd-widget').length===6`)
	both("readonly-dashboard")

	if errs := page.MustEval(`()=>window.fixtureErrors.filter(e=>/homedash|HomeDash|hd-/i.test(e)).join('\n')`).Str(); errs != "" {
		t.Fatalf("page errors:\n%s", errs)
	}
}
```
- [ ] Build and run (PowerShell, sandbox off, from the worktree root):

```powershell
go test -c -o D:\tmp\go-build\hd-m2-ui.test.exe ./ui
cd ui
$env:AURAGO_RUN_BROWSER_SMOKE = '1'
& D:\tmp\go-build\hd-m2-ui.test.exe '-test.run=^TestDesktopHomeDashBrowser$' '-test.v' '-test.count=1' '-test.timeout=30m'
cd ..
```
Expected: `--- PASS: TestDesktopHomeDashBrowser` (about 1.5 minutes). If it fails, fix the cause in the module the assertion names; never loosen an assertion.
- [ ] `go test ./internal/audit -run '^TestUIBrowserCIShardsCoverEveryTest$' -count=1` → ok (the name lands in "Desktop H–R"). `go vet ./ui` → ok.
- [ ] Detect-changes; commit `ui/desktop_homedash_fixture_test.go ui/desktop_homedash_browser_test.go` — `test(homedash): browser test of the live app against the M2 contract fixture`.

**Done when:** the browser test passes in one run; the shard test passes.

---

## Lead review gate

Run once after Task 18 (PowerShell, sandbox off):

```powershell
go test -c -o D:\tmp\go-build\hd-m2-ui.test.exe ./ui
cd ui
$env:AURAGO_RUN_BROWSER_SMOKE = '1'; $env:HOMEDASH_M2_SHOTS = '1'
& D:\tmp\go-build\hd-m2-ui.test.exe '-test.run=^TestDesktopHomeDashBrowser$' '-test.v' '-test.count=1' '-test.timeout=30m'
cd ..
```
The lead judges the fresh PNGs in `reports/homedash-m2/` at 100 % (git-ignored folder):
`empty-{fruity-light,standard-dark}`, `starter-{glass,tactile,minimal,instrument}-{fruity-light,standard-dark}`, `stale-standard-dark`, `empty-dashboard-standard-dark`, `list-{fruity-light,standard-dark}`, `delete-confirm-standard-dark`, `readonly-home-{…}`, `readonly-dashboard-{…}`.

Pass criteria (every item, every listed file):
1. **Empty state:** illustration, title, text and two choice cards centred; the cards have equal height, the starter card is visibly primary, labels and hints legible in both themes; the bar matches the theme's bar material; nothing clipped.
2. **List:** cards aligned in the grid with name and "Updated …" meta; delete buttons visible but secondary; header actions aligned right; spacing consistent.
3. **Starter dashboard (four families, two themes):** three gauges in the top row, network receive and send plus uptime in the bottom row, no overlap, no scroll bar at 1180×780; gauge zone arcs visible (warn 80/85–95, crit 95–100); both sparklines filled across their width; uptime shows a duration with a clock or entity glyph, not raw seconds; units shown; no lock badge on the sensors; each family matches the M1 sign-off pack.
4. **Stale:** warning banner with "Retry now" in theme materials, connection pill "Offline", widgets show the stale clock badge and muted numerals and keep their values.
5. **Empty dashboard:** canvas note centred, legible, no stray chrome.
6. **Delete confirm:** card centred over a dimmed window, danger button distinct, focus ring on Cancel.
7. **Read-only desktop:** info banner on the dashboard; home without create buttons and delete buttons, with the read-only note.

A failed item becomes a numbered finding (file, item, required change); fix tasks follow; the gate is re-run on fresh screenshots. SCR-M2-1/2 are accepted only with the four-family starter screenshots.

---

### Task 19: Documentation (docs-agent)

**Files:** modify `ui/js/homedash/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md`, `documentation/homedash-visual-system.md`, root `AGENTS.md` (HomeDash row); create `docs/superpowers/specs/2026-11-23-homedash/m2-handover-frontend.md` (`git add -f`).
**Read first:** this plan's Tasks 1–18, R7, Lead review gate outcome.

- [ ] `ui/js/homedash/AGENTS.md`: Purpose (the desktop app now loads the engine); load order = `APP_ORDER` (harness) mirrored by the module-loader entry, pinned by `wiring.test.mjs` and `TestHomeDashWiring`; parts `E.tone`, `E.api`, `E.stream`, `E.store`, `E.catalog`, `E.render`, `E.sheet`, `E.view`, `E.mount`; the tone rule R7 verbatim; store rules 1–5 (Task 4); realm sharing (one api/stream/store per base path, owners with visibility); view-only api (`can` false, `command` rejects, `history` honours `hours`); stylesheet bundle now ten parts (`page` last); SCR-M2-1/2 as recorded structure changes; new tests in Verification.
- [ ] `ui/js/desktop/apps/AGENTS.md`: new section "HomeDash" (files `homedash.js`, `homedash-home.js`, `desktop-app-homedash.css`; capability `homedash`, category `comms`; visibility rule F11; policy event counts only `readonly`; window context `dashboard_id`, `page_id`; menus; banners drawn by the host from `E.view.banners`; materials adoption; tests: `npm run test:homedash`, `TestHomeDashWiring`, `TestDesktopHomeDashBrowser`).
- [ ] `documentation/homedash-visual-system.md`: "M2 additions": page part, host chrome (desktop theme, not tokens), SCR-M2-1 tile geometry and duration text, SCR-M2-2 live tail; Host duties 1–7 marked as implemented by `hd-view`/`hd-render`.
- [ ] Root `AGENTS.md` HomeDash row: add the desktop app files and the M2 state, keep the existing pointers.
- [ ] Hand-over: state (commits, tests, gate result), contract readings (Self-review items 3–12), open items (Task 20 outcome, sheet styling moves to the engine in M3, banners for kiosk in M6).
- [ ] Run `go test ./internal/audit -count=1` in the worktree (doc link checks) → ok. `git ls-files` shows the hand-over file.
- [ ] Commit the five paths — `docs(homedash): M2 frontend guides, visual system additions and hand-over`.

**Done when:** the guides describe what is on the branch and the audit tests pass.

---

### Task 20: Backend switch (D9)

**Files:** if the backend landed: create `ui/desktop_homedash_live_browser_test.go`; else none (hand-over line only).

- [ ] Check: `git log --oneline -1 main -- internal/server/homedash_routes.go`. Empty → append "D9: the switch to the real server moves to M3" to the hand-over (Task 19 file, `git add -f`), commit `docs(homedash): D9 backend switch moves to M3`, stop.
- [ ] Otherwise rebase this branch on main (`npm run check:ui` after the rebase; rebuild bundles if needed) and create the opt-in test (never against aurago-test; a local scratch instance built from main, its own token with scope `desktop:admin`):

```go
package ui

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"testing"
	"time"
)

// TestDesktopHomeDashLiveServerBrowser (D9) runs the starter scenario against a real AuraGo server through a reverse proxy. Set HOMEDASH_LIVE_URL
// (a local scratch instance built from main) and HOMEDASH_LIVE_TOKEN (bearer token, scope desktop:admin). Without them it is skipped.
func TestDesktopHomeDashLiveServerBrowser(t *testing.T) {
	base, token := os.Getenv("HOMEDASH_LIVE_URL"), os.Getenv("HOMEDASH_LIVE_TOKEN")
	if base == "" || token == "" {
		t.Skip("set HOMEDASH_LIVE_URL and HOMEDASH_LIVE_TOKEN to run against a live server")
	}
	target, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.FlushInterval = -1
	director := proxy.Director
	proxy.Director = func(r *http.Request) {
		director(r)
		r.Host = target.Host
		r.Header.Del("Origin")
		r.Header.Del("Cookie")
		r.Header.Set("Authorization", "Bearer "+token)
	}
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/desktop/homedash/": proxy}).CancelTimeout().Timeout(3 * time.Minute)
	page.MustEval(`(words)=>Object.assign(window.fixtureWords, JSON.parse(words))`, readDesktopAssetText(t, "lang/homedash/de.json"))
	page.MustEval(`()=>auditTest.openApp('homedash')`)
	page.MustWait(`()=>{const h=document.querySelector('.hdx-home');return !!h&&['empty','list'].includes(h.dataset.hdxMode);}`)
	page.MustEval(`()=>document.querySelector('.hdx-home [data-action="create-starter"]:not([hidden])').click()`)
	page.MustWait(`()=>document.querySelectorAll('.hdx-view .hd-page > .hd-widget').length===6`)
	page.Timeout(40*time.Second).MustWait(`()=>document.querySelector('.hdx-view .hd-root').dataset.hdConn==='live'&&!!document.querySelector('[data-hd-wid="cpu"] .hd-gauge[aria-valuenow]')`)
	page.Timeout(60*time.Second).MustWait(`()=>{const s=document.querySelector('[data-hd-wid="net-rx"] svg.hd-spark');return !!s&&!s.hasAttribute('data-hd-empty');}`)
	id := page.MustEval(`()=>auditTest.state.windows.get(auditTest.state.activeWindowId).context.dashboard_id`).Str()
	req, _ := http.NewRequest(http.MethodDelete, base+"/api/desktop/homedash/dashboard?id="+url.QueryEscape(id)+"&revision=1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	if resp, err := http.DefaultClient.Do(req); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("cleanup delete of %s failed: %v %v", id, err, resp)
	}
}
```
- [ ] Run without env → `--- SKIP` (CI-safe); with env against the scratch instance → PASS. `go test ./internal/audit -run '^TestUIBrowserCIShardsCoverEveryTest$' -count=1` → ok.
- [ ] Detect-changes; commit `ui/desktop_homedash_live_browser_test.go` — `test(homedash): opt-in browser run against a live server (D9)`.

**Done when:** either the hand-over records the move to M3, or the live test passed once against a scratch server.

---

## Batches and reviews

| Batch | Tasks | test-agent runs | reader-agent review focus (Critical/Important only, once) |
|---|---|---|---|
| A — F1 data layer + texts | 1, 2, 3, 4, 5, 6 | `npm run test:homedash`; the i18n tests of Task 6 | contract fidelity of hd-api/hd-stream/hd-store (status+code, reconnect rules, snapshot semantics), timer leaks, cross-window sharing |
| B — engine view | 7 (after lead SCR approval), 8, 9, 10, 11, 12 | `npm run test:homedash`; `npm run check:ui` | host duties (look, owned tokens, resize px), tone flow, focus trap, state machine races (`openSeq`), DOM safety |
| C — desktop host + wiring | 13, 14, 15, 16, 17 | `npm run test:homedash`; `go test ./ui -run '(?i)^(TestHomeDashWiring\|TestDockerManagerWiring\|TestDesktopAppMaterialsAdoptedByApps\|TestTranslations_\|TestFrontend_StaticI18nKeysExistInEnglishBundle)' -count=1`; `go test ./internal/desktop -count=1`; `go vet ./ui` | materials adoption, read-only paths, disposal (observers, listeners, menus), bundle regenerated |
| D — browser | 18 | the browser command of Task 18 and the shard test | fixture matches the contract; test asserts every lead-required behaviour |
| Gate | Lead review gate | screenshot run | lead only |
| E — docs and D9 | 19, 20 | `go test ./internal/audit -count=1` (worktree) | guides match code; no secrets in Task 20 |

Lean workflow: one review per batch; findings are fixed in one follow-up commit per batch without a second review round; minor style notes are not raised.

---

## Self-review

**Coverage.**
- D1 F1 (`hd-api`, `hd-stream`, `hd-store`, `hd-catalog` with Node tests) → Tasks 1, 3, 4, 5. F2 (`homedash.js` with `HomeDashApp`, `hd-render`, `hd-sheet`, `hd-view`, `hd-app`, module-loader, i18n section, `appGlobalName`, dispatch, size presets, `matchesExistingAppWindow`, `BuiltinApps` row, icon, `Requires: ["homedash"]`, `desktop-app-homedash.css`, `npm run build:ui`) → Tasks 9–17.
- D2 (only the frozen contract) → "Contracts used"; every route/frame use maps to one function. D3 (starter via `{"template":"system"}`, empty via `{"document":…}` with localized name) → Tasks 12, 13, 18. D4 is backend-only. D5 (tone helper, pure, Node-tested, documented) → Task 2, R7, Task 19. D6 (SSE + PUT, one stream per realm, 15 s hb, snapshot on add) → Tasks 3, 4, 12. D7 entities → testdata in Task 9, duration display in Task 7. D8 (Node tests, one wiring test, one browser test in H–R) → every task, Tasks 16, 18. D9 (fixture first, last task switches) → Tasks 18, 20.
- Lead decisions: history series conversion and live append → Task 4 (rule 5, tested); alert tone direction rule → R7; one stream per realm + host visibility → Tasks 3, 12, 14; themed host chrome with concrete CSS → Task 15; frozen canvas tokens untouched (`m1.test.mjs` runs in every task) → Tasks 7, 8; icon described and drawn → Task 17; browser test coverage list (start menu, empty state, starter, live change, sparkline history, drop → stale banner → reconnect) → Task 18 steps 1–7; file sizes, no innerHTML, CSSOM only, literal keys, 16 locales → R4, Tasks 6, 14–16.

**Contradictions found and the resolution chosen.**
1. **Starter `entity/tile` has no production look.** `.hd-tile` is styled only in `ui/testdata/homedash-lab.css` (grep finds no `.hd-tile` in `ui/css/homedash-*.css`), and the tile writes raw numbers (`hd-w-misc.js:21`), while the contract says uptime is formatted as a duration. Resolution: SCR-M2-1 (Task 7), lead approval before dispatch.
2. **Sparklines read history once** (`hd-w-sensor.js:146`), so the lead's cached-series append alone never moves a visible line. Resolution: the store appends (Task 4) and SCR-M2-2 adds a local tail (Task 7).
3. **D5 says the host passes the tone "through `api.setTone`".** The engine's channel is the view's `tone` (visual system "Host duties" 4; `hd-w-sensor.js:60` calls `api.setTone(e.tone …)`). Resolution: host sets `tone` on the view; widgets keep calling `setTone`.
4. **Watchdog:** design-frontend 3.2 probes with a PUT after silence; the contract says 45 s without any frame (hb is an event) is dead. Contract wins (Task 3).
5. **Jitter:** design 3.2 "0–750 ms" vs contract "±20 %". Contract wins.
6. **Backoff reset:** Docker Manager resets at hello; contract after 60 s of life. Contract wins.
7. **Snapshot semantics:** design 3.3 "ids missing from the snapshot become unavailable" vs contract "snapshot covers newly added ids". Contract wins (store rule 1).
8. **429 `too_many_streams` is invisible to EventSource.** Resolution: any failure before hello waits at least 5 s and calls `diagnose()` (bootstrap) to detect `homedash_disabled`/401.
9. **Stale banner timing:** the contract lists "show the stale banner" among the drop actions; design 3.3/6 show it after 5 s down. Chosen: 5 s grace (a 1 s reconnect does not flash); the browser test refuses two reconnects so the banner appears.
10. **Read-only lock on sensors:** every `system` entity is `readonly:true` (contract), and the M1 hero rule would lock-badge all six sensors. Chosen: view `readonly` only for entities with a command capability (store rule 3); a desktop read-only state still locks controllable entities.
11. **Banners in `.hd-root`** (design 9.1, engine) vs "banners use the desktop theme" (lead). Chosen: `E.view.banners()` computes them in the engine; the desktop host draws them in its chrome; kiosk (M6) draws its own.
12. **`.hd-more` and `.hd-sheet` have no production CSS** (only the lab styles them). Chosen: no `moreLabel` in M2 (more-info is M3); `hd-sheet` is used for the delete confirm and styled by the host CSS under `.hdx-app`.
13. **Delete is not in the lead's deliverable list,** but "Empty dashboard" without a designer would leave undeletable empty dashboards; it is also the only M2 use of `hd-sheet` (D1). Kept as part of Tasks 12–14; dropping it means removing `remove`, the card delete button and the confirm block.
14. **Icon allowlist and theme manifests:** `internal/desktop/ecalc_app_test.go:5-6` shows app-owned artwork via `logo_path` needs neither; `desktopPreferredIconNames` is not changed.
15. **Sensor history range:** the module always asks `'24h'` (`hd-w-sensor.js:36`); the starter sets `hours: 1`. The host maps `options.hours` (Task 9).
16. **Base CSS is frozen,** but page geometry was missing. A tenth bundle part `homedash-page.css` holds it (Task 8).
17. **History example `origin:"memory"` vs amendment A1 (`origin:"source"` for system).** The client does not depend on `origin`; the fixture uses `"source"`.
18. **Registry vs catalog:** temperature's optional `humidity` binding is not registered; the catalog check requires only required bindings (Task 5).

**What I shortened.** Translations for the 14 locales other than English and German are delegated to the maker (exact keys and both reference texts given). `hd-view` and the desktop host are written as full code but with compact comments; CSS for the canvas tabs and the host chrome is complete, but no separate kiosk styles exist (M6).
