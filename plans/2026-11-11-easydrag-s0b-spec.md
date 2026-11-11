# EasyDrag S0b "Egress and credential hygiene": design spec

**Status:** design spec, 2026-11-11. The owner approved both parts in the brainstorming session the same day. Implementation plan: `plans/2026-11-11-easydrag-s0b/` (2026-11-11).
**Roadmap:** `plans/2026-11-09-easydrag-next-roadmap.md`, E1 items FD-13 and FD-18 (lines 292-293, acceptance 319-320, guardrails 330-340), Wave 0 (§6).
**Predecessor:** S0 landed on main at 79c7c601a (`plans/2026-11-10-easydrag-s0-spec.md`).

## 1. Goals and owner decisions

S0b closes the two server-side trust gaps of Wave 0:

- **Part A, browser egress (FD-18).** Every headless browser that opens an untrusted URL goes through a network-layer egress proxy, not a page-level hook.
- **Part B, credential hygiene (FD-13).** A credential typed into a flow can be moved into the vault with one action. A guided purge then removes every plaintext copy from flows.db, its WAL, the flows backups and the browser storage.

Owner decisions (2026-11-11):

| # | Question | Decision |
|---|---|---|
| 1 | Old runs whose stored step parameters hold a typed token | **Redact in place.** Runs stay in the history; the token becomes the redaction marker. |
| 2 | The safety backup the purge takes first | **Encrypted with a key from the master key, deleted automatically after 7 days.** |
| 3 | How the purge is reached | **Guided plus a hint.** It is offered after a move and a publish; the start page shows a hint while plaintext remains anywhere. |
| 4 | How far "move to vault" reaches | **Header and URL query parameter.** `http.request` gets a second place for its secret. Body fields and other nodes are detected, warned and purged, but not moved. |
| 5 | Part A as designed | **Approved.** One helper; a fresh browser and a proxy per call; proxy credentials over DevTools after a spike; a path check; the S8 fixture. The shared browser is dropped. |
| 6 | Part B as designed | **Approved,** including detection of known token formats. |

The old branch `claude/modest-northcutt-18107e` (9368239fb, c1d427ffc, base 476bad439 of 2026-10-27) is a reference only. Main has moved about 1,700 commits since then, and main already has the egress proxy that branch lacked. Part A is built new on main.

## 2. Scope

**In S0b**
- Part A: `security.LaunchEgressBrowser`, proxy credentials per call, the path check, failing closed, five call sites moved, the old hook and the shared browser deleted, the S8 fixture, one live run on aurago-test.
- Part B: one detection rule, `http.request` query auth, the move endpoint and action, client masking, the scan, the guided purge, the encrypted safety backup and its sweeper, the restore flag, schema 9.

**Not in S0b**
- Storymaker rendering: it already uses a strict proxy and loads only local pages (`internal/storymaker/render/browser.go:306-336`, `render.go:500`).
- `browser_automation`: a Docker sidecar with its own egress container (`internal/tools/browser_automation.go:36-40`).
- Gotenberg, which runs in its own container (`internal/tools/gotenberg_security.go:81`).
- Browsers inside guest VMs (`internal/virtualcomputers/guest_workspace_agent/`).
- A general vault placeholder in every text field (owner decision 4).
- A restore button in the UI; restore is a start flag (§6.5).
- AuraGo backup archives that already sit outside the data directory.

## 3. Part A: browser egress (FD-18)

### 3.1 Facts on main
- Four sites that open untrusted URLs use only the page hook `OpenRodPageWithSSRF` / `GuardRodPageSSRF` (`internal/security/rod_ssrf.go:14-76`). That hook covers one page target. It misses service-worker fetches, speculation-rules prefetch and prerender, and WebRTC.
  - The dynamic scraper: `internal/scraper/scraper.go:133` (`newScraperLauncher`), `:160-190` (`FetchDynamic`). It is reached by `web.read` with mode `dynamic`, or `auto` falling back to it (`internal/tools/scraper.go:80-82`, `:117`, `:391`).
  - Web capture: `internal/tools/web_capture.go:62-78`, on the shared browser.
  - Web performance: `internal/tools/web_performance.go:72-101`, on the shared browser.
  - Form automation: `internal/tools/form_automation.go:85-113`, a fresh browser with no proxy and no WebRTC flags.
- The shared browser `getSharedBrowser` (`internal/tools/utils.go:36-57`) is a `sync.Once` singleton. It keeps a failed launch cached for the life of the process.
- Flows reach the last three as generic `tool.*` nodes. They are not in `genericExcluded` (`internal/flows/catalog_generic.go:36-58`). The invoker sends them to `agent.DispatchToolCallResult` (`internal/server/flows_tool_invoker.go:81`, `:178-191`).
- Main already has the egress proxy: `security.StartCaptureProxy` and `StartStrictCaptureProxy` (`internal/security/capture_proxy.go:105`, `:119`). It binds `127.0.0.1:0` (`:146`). It has no authentication. After `Close` it answers 503 (`:203-214`, `:371-372`).
- The homepage annotation capture already uses the proxy, with both WebRTC flag spellings (`internal/tools/homepage_annotation_capture.go:116-132`). Chrome 156 ignores the `force-` spelling for STUN; the scraper sets only that one (`scraper.go:134`).
- No Go code handles a Chrome enterprise proxy policy that overrides `--proxy-server`.

### 3.2 The helper
New file `internal/security/egress_browser.go`:

```go
type EgressBrowserOptions struct {
    AllowedOrigin *url.URL      // nil: public addresses only (today's StartCaptureProxy(nil))
    Lifetime      time.Duration // 0 means 2 minutes
}

type EgressBrowser struct {
    Browser *rod.Browser
    // Done is closed when the browser, the proxy or the lifetime ends.
    Done <-chan struct{}
}

func LaunchEgressBrowser(ctx context.Context, opts EgressBrowserOptions) (*EgressBrowser, error)
func (b *EgressBrowser) Close() error
func (b *EgressBrowser) Err() error // why it ended: nil after Close, ErrEgressProxyDied, ErrEgressLifetime
```

- **Per call.** Each call starts its own proxy and a fresh Chrome with a fresh profile directory. Close stops both and removes the profile.
- **Chrome flags:**
  - `--proxy-server=http://127.0.0.1:<port>`;
  - `--proxy-bypass-list=<-loopback>`, so loopback also goes through the proxy;
  - `--force-webrtc-ip-handling-policy=disable_non_proxied_udp` and `--webrtc-ip-handling-policy=disable_non_proxied_udp`;
  - the existing headless and sandbox flags of each site, kept as they are.
- **Limits:**
  - at most 4 egress browsers at once, guarded by a semaphore; a fifth caller waits up to its context deadline;
  - a lifetime of 2 minutes, after which the browser and the proxy are stopped.
- **The slot is always released,** including when the launch or the DevTools dial fails. This is the lesson of c1d427ffc; a test pins it.
- **The browser binary.** It is resolved the way the sites resolve it today. `reference_chrome_launch_from_agent` applies to tests.

### 3.3 Proxy credentials per call (dropped after the A.1 spike)

**Owner decision 7 (2026-11-11, after the spike): no proxy credentials.**
- **Why there are none.** Chrome cannot answer the proxy challenge over DevTools for WebSockets and speculation-rules prefetch, and the service-worker fetch never reached the proxy (table below). With credentials, pages that need a WebSocket would break in the scraper and in web capture.
- **What credentials would have added.** The proxy is reachable only on `127.0.0.1` at a random port. It lives for at most 2 minutes, and it refuses every private, link-local, metadata and LAN target. A local program that used it could reach only the public internet, which it reaches without it. Credentials would have added nothing that is worth that breakage.
- **What changes.** The proxy keeps loopback, the random port, the lifetime, failing closed and the path check (§3.4), and has no credentials.
  - A.2 drops `WithProxyCredentials` and the 407 handling.
  - A.3 drops the DevTools auth loop.
- **Roadmap.** The roadmap guardrail at line 340 is amended to match.

The text below is the original design, kept for the record.

- `CaptureProxy` gets optional credentials, set by an option on both start functions. When they are set, the proxy answers a request without a matching `Proxy-Authorization: Basic …` with 407 and `Proxy-Authenticate: Basic realm="aurago-egress"`. That covers plain requests and CONNECT.
- `LaunchEgressBrowser` creates 16 random bytes for the user and 16 for the password, hex-encoded, for every call.
- Chrome receives them only over the DevTools protocol, through rod's auth handling, which uses `Fetch.authRequired` with `handleAuthRequests`. They never appear on a command line, in an environment variable or in a file.
- **Spike first (plan task A1).** Prove that Chrome answers the proxy challenge for all of these kinds of request:
  - a top-level navigation;
  - subresources;
  - `fetch` and XHR;
  - a WebSocket;
  - a fetch from a dedicated worker;
  - a fetch from a service worker;
  - a speculation-rules prefetch.

  If any of them cannot authenticate, the plan stops and asks the owner. A request that cannot authenticate still fails closed, so this is a functional question, not a security hole.

**A.1 spike result (2026-11-11, Chrome/157.0.8103.51, go-rod v0.116.2).** Each cell is one fresh Chrome and one fresh proxy; the proxy forces a 407 on the first request to every key.

| kind | browser-wide | browser-wide+attach |
|---|---|---|
| nav | PASS | PASS |
| subresource | PASS | PASS |
| fetch | PASS | PASS |
| xhr | PASS | PASS |
| websocket | FAIL | FAIL |
| worker | PASS | PASS |
| serviceworker | NOT-OBSERVED | NOT-OBSERVED |
| prefetch | FAIL | FAIL |

Verdict: STOP. No variant authenticates every kind (see the table). The owner then chose no proxy credentials (owner decision 7, at the top of this section).

Details. The Chrome challenge reads `source=Proxy origin=http://<proxy host:port> scheme=basic`; Task A.3 can filter on `Source == "Proxy"` and an origin equal to the proxy address. Per-target attach changed nothing: both variants give the same table. `websocket` and `prefetch` reached the proxy, got the 407, and Chrome never raised `Fetch.authRequired` for them, so the request stayed unauthenticated and failed closed (no leak). `serviceworker` was never seen at the proxy, even after a rerun with a 15 s settle (both variants), so by the spike rule it counts as a failure. `nav`, `subresource`, `fetch`, `xhr` and the dedicated `worker` authenticate with the plain browser-wide `Fetch.enable`.

### 3.4 Failing closed and the path check
- **Proxy death.** `CaptureProxy` gets a `Done()` channel that closes when its listener stops for any reason. When it closes, the helper cancels the browser context and records `ErrEgressProxyDied`. Callers return that error and never partial content.
- **No direct fallback.** Chrome with an explicit `--proxy-server` never falls back to a direct connection.
- **Path check.**
  1. After launch, the helper opens `http://egress-check.aurago.invalid/<nonce>` on a blank page.
  2. The proxy answers that host itself, with 204 and the header `X-AuraGo-Egress: <nonce>`. It never resolves or dials it.
  3. If the page does not see that header within 5 seconds, the helper closes everything and returns `ErrEgressBypassed`. The usual cause is a policy that overrides the proxy, because `.invalid` never resolves through DNS.

### 3.5 Call sites
| Site | Today | After |
|---|---|---|
| `scraper.FetchDynamic` (`scraper.go:133-190`) | own launcher, hook | helper, `AllowedOrigin` nil |
| `web_capture.go:62-78` | shared browser, hook | helper, `AllowedOrigin` nil |
| `web_performance.go:72-101` | shared browser, hook | helper, `AllowedOrigin` nil |
| `form_automation.go:85-113` | own browser, hook, no WebRTC flags | helper, `AllowedOrigin` nil |
| `homepage_annotation_capture.go:116-164` | own proxy and browser | helper, keeps its loopback-preview `AllowedOrigin` (`:97-101`) |

- `ValidateSSRF` stays in front of each launch, so a refused address gets its clear error before Chrome starts.
- Deleted:
  - `getSharedBrowser` (`internal/tools/utils.go:36-57`);
  - `OpenRodPageWithSSRF` and `GuardRodPageSSRF` (`internal/security/rod_ssrf.go`), after a text search confirms no other caller.
- `internal/security/AGENTS.md` and `internal/tools/AGENTS.md` record the rule. New headless-browser code that opens untrusted URLs uses `LaunchEgressBrowser`, never `launcher.New()` with a hook.
- **Speed.** Web capture and web performance lose the shared browser, which costs about one Chrome start (around 1 s) per call. Owner decision 5 accepts this.

### 3.6 The S8 fixture
New file `internal/testutil/browser_egress.go`:
- A loopback test server serves one page, reached with `AURAGO_SSRF_ALLOW_LOOPBACK=1` (`internal/security/ssrf.go:90-104`, loopback only).
- The page tries all of these:
  - `window.open('http://169.254.169.254/latest/meta-data/')`;
  - `new WebSocket('ws://10.255.255.1:80/')`;
  - a service worker that fetches `http://192.168.255.1/`;
  - `<link rel="prefetch">` and a speculation-rules prerender of `http://169.254.169.254/`;
  - an `RTCPeerConnection` with `stun:10.255.255.1:3478`.
- A fixture helper reads the proxy's refusal log and the page's own report.

Browser tests, gated by `AURAGO_RUN_BROWSER_SMOKE=1` and run per `reference_chrome_launch_from_agent`:
- `TestEgressBrowserRefusesTheS8Fixture`, in `internal/security`, drives the helper directly. Every probe is refused: the proxy logs each private target, and no request to any of them leaves the machine. The STUN probe produces no UDP packet to the target; the test checks that through the WebRTC stats, which show no candidate pair.
- `TestScraperDynamicGoesThroughTheEgressProxy` and `TestWebCaptureGoesThroughTheEgressProxy` show that the sites really use the helper: the proxy log has the page's own request.

Unit tests (not gated):
- the proxy without credentials gets 407; with them it passes;
- the slot is released after a failed launch;
- `ErrEgressProxyDied` after a forced proxy close;
- `ErrEgressBypassed` when the check host answers without the nonce.

**Live on aurago-test.** Build the gated security test binary for Linux (`go test -c`) and run it there once against its Chrome. The run is read-only: it creates no container, app or data and changes none (standing rule for aurago-test). The result goes into the plan's record.

**A.6 result (aurago-test, 2026-11-12).** PASS (`TestEgressBrowserRefusesTheS8Fixture`, 8.74 s). Chromium 128.0.6568.0 (the rod-downloaded build already in the SSH user's `~/.cache/rod/browser/chromium-1321438/`, found via `AURAGO_TEST_CHROME`; no install), Ubuntu 26.04.1 LTS, kernel 7.0.0-42, x86_64, run as the unprivileged SSH user with the sandbox on. The test sets `AURAGO_SSRF_ALLOW_LOOPBACK` itself.
- Refused: `172.31.255.1:80, 169.254.169.254:80, 172.31.255.1:80, 10.255.255.5:80, 10.255.255.1:80, 10.255.255.2:80, 192.168.255.1:80, 10.255.255.1:3478`.
- Dialed: `accounts.google.com:443, 127.0.0.1:40233, www.google.com:443, www.google.com:443` (the fixture page itself plus Chrome's own public traffic; no private target).
- Page report: every `Ran*` flag true; `CandidatePairs:0`; service worker activated, its fetch to `192.168.255.1` refused by the proxy.
- Not covered: prerender (no request reached the proxy and the page cannot see whether Chrome tried; the speculation rules API is present). Prefetch was issued (`RanPrefetch`) but the page event was `error`, and no prefetch request reached the proxy either, so it counts as refused-or-not-sent, not as an observed refusal.
- Difference from the local run: Chrome 128 here against a newer Chrome locally; Chrome's own accounts.google.com and www.google.com connections appear in Dialed on the server. No difference in the outcome.
- The run created nothing on the server beyond the copied binary and a scratch `/tmp/s0b-run`, both removed afterwards.

## 4. Part B: one detection rule

### 4.1 Facts on main
- Typed values sit in clear in the draft, the live document, every version, the run's document and its step parameters (`internal/flows/AGENTS.md:751-752`, `:776-777`).
- Detection exists only for credential-named headers of `http.request`: `credentialHeaderName` (`internal/flows/credentials.go:14-26`) and `literalCredentialHeaders` (`:55-71`). They raise the warning `PARAM_LITERAL_CREDENTIAL`.
- `RedactCredentials` (`credentials.go:106-136`) changes only header maps, plus group and note text. Stored step parameters use `redactCredentialHeaders` (`engine.go:313`, `durable_coordinator.go:82`).
- `security.ScrubSecrets` replaces only values that were registered (`internal/security/scrubber.go:94`, `:178`).

### 4.2 The rule
New function in `internal/flows/credentials.go`:

```go
type CredentialFinding struct {
    NodeID   string
    Param    string // the node parameter, for example "url", "headers", "body", or a generic tool param
    Location string // "header", "url_userinfo", "url_query", "body_field", "param_field", "format"
    Path     string // where inside the param: header name, query key, JSON pointer
    Movable  bool   // true for http.request header, url_query and url_userinfo
}

func FindLiteralCredentials(doc *Flow) []CredentialFinding
```

**What counts.** Only literal text. A value that is a template, or a query value or JSON leaf containing `{{`, is never a finding. The rule checks these places:
- **Headers.** The existing `credentialHeaderName` names.
- **URL userinfo.** `scheme://user:pass@host`, in any `URLTarget` param.
- **URL query parameters.** A credential name as the key, in any `URLTarget` param.
- **Body fields and parameter fields.** Credential-named keys at any depth, in JSON body params and in the params of generic `tool.*` nodes.
- **Known token formats.** Any literal string param of any node whose whole value, or one whitespace-separated word of it, matches a known format. It needs at least 16 characters.

**Credential names.** Matching ignores case, `-` and `_`. A key is a credential name when it matches any of these:
- one of these exact names: `token`, `accesstoken`, `refreshtoken`, `idtoken`, `apikey`, `key`, `secret`, `clientsecret`, `password`, `passwd`, `pwd`, `sig`, `signature`, `auth`, `authorization`, `xapikey`, `privatekey`, `secretkey`, `accesskey`, `sessiontoken`;
- a name that ends in `token`, `secret`, `password` or `apikey`.

`key` counts only as a URL query key, where it is a common API-key name. It does not count as a body field, where it is too often harmless.

**Token formats.**
- `sk-` and `sk_live_`, `rk_live_`, each followed by 20 or more characters;
- `ghp_`, `gho_`, `ghu_`, `ghs_`, `ghr_`, `github_pat_`;
- `glpat-`;
- `xoxb-`, `xoxp-`, `xoxa-`, `xoxr-`;
- `AKIA` followed by 16 characters from `[0-9A-Z]`;
- `AIza` followed by 35 characters from `[0-9A-Za-z_-]`;
- a JWT: three base64url parts separated by dots, where the first starts with `eyJ`.

**Shared test vectors.** The cases live in `internal/flows/testdata/credential_cases.json`. The Go test and the client test (§5.4) both read them, so the two rules cannot drift.

### 4.3 What the rule drives
- **Validation.** One `PARAM_LITERAL_CREDENTIAL` warning per finding, replacing today's header-only check.
  - The issue carries `location` and `path`, so the UI can name the place, for example "Adresse, Parameter api_key".
  - It stays a warning, so publishing is not blocked.
  - The issue message keys get location variants in all 16 locales.
- **Redaction.** `RedactCredentials` and the stored step parameters (`engine.go:313`, `durable_coordinator.go:82`) replace every finding with the existing redaction marker. That covers exports, packs, run views, child history and stored step params. This is the widening the roadmap guardrail asks for: URL query parameters and credential-named body fields.

## 5. Part B: moving a credential into the vault

### 5.1 `http.request` query auth
- Two new params next to `auth_secret` (`internal/flows/catalog_actions_http.go:427-432`):
  - `auth_in`: an enum, `header` (the default) or `query`;
  - `auth_query`: the name of the query parameter. It is required when `auth_in` is `query` and, like `auth_header`, must be written out (`authLiteralParams`, `:201`).
- In `withAuthSecret` (`:219`):
  - With `auth_in=query`, Execute sets that query parameter of the URL to the secret value, replacing any existing value, after templates are resolved.
  - The `SecretTarget` rules for the URL stay: untrusted data cannot reach the scheme, user info, host or port while `auth_secret` is set.
  - The secret goes only into the first request. A redirect follows the server's `Location`, and the parameter is never added to it again.
- The secret value is registered with the scrubber when it is read (`internal/server/flows_llm.go:565-572`). The URL in the node output and the stored step params therefore shows the marker.

### 5.2 The move endpoint
`POST /api/desktop/flows/{id}/credentials/move` is in admin scope, uses the same-origin CSRF check (`flowsRequiredScope` at `internal/server/flows_handlers.go:561-573`, `flowsOriginOK` at `:399-407`) and takes the secret-write rate limit (`:59-67`).

The request body is `{ "revision": n, "node_id": "...", "location": "...", "path": "...", "secret_name": "...", "replace": false }`.

Steps:
1. **Load the draft.** Load the stored draft at `revision`. If the draft has moved on, answer 409 `FLOW_REVISION_CONFLICT`. The client saves before it calls the endpoint.
2. **Read the literal.** Find the literal at (`node_id`, `location`, `path`). If it is no longer there, answer 409.
3. **Check the name.** `secret_name` must match `^[a-z0-9_]{1,40}$`. If `easydrag_<name>` exists and `replace` is false, answer 409 `FLOW_SECRET_EXISTS`.
4. **Write the vault entry,** with `WriteUserSecretContext` (`flows_handlers.go:970`).
5. **Rewrite the draft** in one save:
   - **header:** remove the header; set `auth_secret`, `auth_in=header` and `auth_header` to the header name. If the value starts with `Bearer ` or `Basic `, set `auth_prefix` to that word plus a space, and store the value without it.
   - **url_query:** remove the parameter from the URL; set `auth_secret`, `auth_in=query` and `auth_query` to the key.
   - **url_userinfo:** remove `user:pass@`; store the base64 of `user:pass` as the secret; set `auth_in=header`, `auth_header=Authorization` and `auth_prefix="Basic "`.
6. **Same value elsewhere.** Every other finding in the same flow with the identical literal gets the same rewrite when it is movable. The response lists the ones that are not movable.
7. **Audit.** Write the audit entry `flow_credential_moved` with the flow id, the node ids and the secret name, never the value.
8. **Answer** with the new revision and document.

Locations that are not movable answer 422 `FLOW_CREDENTIAL_NOT_MOVABLE`. The literal value never travels back to the browser, and the endpoint never returns it.

### 5.3 The UI action
- The warning in the detail view's issue box offers "In den Tresor verschieben" when `Movable` is true. Every issue click on a card already opens that view (amended while planning). Otherwise it offers "Wert entfernen", which clears the field in the client.
- A small dialog:
  - asks for the secret name, proposing one from the host and the key (`api_example_com_api_key`, cut to 40 characters);
  - says that the value is stored in the vault and replaced in this flow, in N places.
- On success the editor:
  - takes the returned document;
  - scrubs the undo history (§5.4);
  - shows: "Verschoben. Veröffentliche den Flow, dann kannst du alte Kopien bereinigen."
- After the next publish of a flow that had a move in this session, EasyDrag offers the purge dialog (§6.3).
- New locale keys go into all 16 `ui/lang/easydrag/*.json` files, with real translations. German uses lowercase "du". New FLOW codes get registry entries and translations.

### 5.4 Browser storage
New module `ui/js/desktop/apps/easydrag-credentials.js` (`ED.credentials`). It mirrors §4.2 for masking and is tested against the shared vectors. Masking means each finding's value becomes the marker before anything is written:
- the emergency copy (`easydrag-saver.js:113`, key `aurago.easydrag.draft.<id>`);
- the clipboard fragment (`easydrag-interact.js:568`, key `aurago.easydrag.clipboard`);
- not needed for the remembered test trigger (`easydrag-run-start.js:92`, key `aurago.easydrag.test-trigger.<id>`). It holds only a trigger node id. Test data lives on the server in `flow_test_data`, which the purge redacts (amended while planning).

Consequence: restoring an emergency copy brings back the marker, not the token. The restore notice says so.

**Undo.** After a successful move, the editor replaces the moved literal with the marker in every snapshot of the undo and redo stacks (`easydrag-model.js:186`, `:384-424`). Undo then cannot bring the token back. It needs a new model method, `scrubHistory(value)`.

## 6. Part B: the scan and the guided purge

### 6.1 Facts on main
- `flows.db` runs in WAL mode (`internal/dbutil/open.go:161-165`) and is at schema 8 (`internal/flows/store.go:59`).
- JSON text columns:
  - `flows.draft_json` and `flows.live_json`;
  - `flow_versions.json`;
  - `flow_runs.doc_json`, `trigger_data_json` and `checkpoint_json`;
  - `flow_run_steps.params_json` and `output_json`;
  - `flow_test_data` and `flow_test_docs`.

  Columns `store.go:61-154`. Scenarios and durable checkpoints are ciphertext.
- The version cap and the pins are in `pruneVersionsSQL` (`store.go:587-596`): the last 50 versions are kept, plus versions used by queued, running or waiting runs, and block pins. Test docs are pinned through `flow_runs.doc_hash`.
- Migration snapshots are made by `backupFlowsSnapshot` (`store_migration_v2.go:82-122`). It writes `VACUUM INTO` into a temporary directory next to flows.db, with the prefix `.flows-v2-backup-`, `.flows-v3-backup-` and so on. They are plaintext, and nothing ever deletes them.
- AuraGo's general backup includes flows.db as a `VACUUM INTO` copy (`internal/config/sqlite_paths.go:72`, `internal/server/backup_sqlite.go:36-39`).
- Nothing in `internal/flows` runs `VACUUM`, `secure_delete` or `wal_checkpoint`.
- The vault's AES-GCM key is private to `Vault` (`internal/security/vault.go:86-127`). The master key comes from `AURAGO_MASTER_KEY` (`internal/config/config.go:1793`).

### 6.2 The scan
`Service.CredentialScan(ctx) (CredentialReport, error)`, exposed as admin `GET /api/desktop/flows/credentials/scan`, reports:
- the flows whose draft holds findings, which the owner must move or remove;
- the flows whose live document holds findings, which must be published again after the move;
- the number of superseded versions with findings, split into purgeable and pinned;
- the number of runs, step rows and test-data rows with findings or with a known literal (§6.4 step 3);
- the migration snapshot directories `.flows-v*-backup-*` next to flows.db, with size and date;
- the plaintext purge leftovers, if a purge failed (§6.4);
- the encrypted purge backups with their expiry dates.

The service caches the report. It is computed in the background at start, after a move, a publish or a purge, and at most once every 10 minutes otherwise. The start page reads the cached report. A non-empty report shows a hint card, "Getippte Zugangsdaten gefunden", with the purge dialog behind it.

### 6.3 The dialog
The dialog is admin only and is reached from the hint card or after a publish (§5.3). It shows the scan report in plain words:
- what will be deleted: N old versions;
- what will be blacked out: M runs, K test rows;
- what stays and why: pinned versions are deleted as soon as their run ends; drafts and live documents that still hold a token show "erst verschieben" or "neu veröffentlichen", with a link that opens the flow;
- the migration snapshots, one checkbox each, all checked by default. Each shows its date and size and says "enthält Klartext von damals";
- one honest sentence: "Ältere AuraGo-Gesamtsicherungen außerhalb des Datenordners kann AuraGo nicht erreichen."

The button is "Jetzt bereinigen". Progress and the result are shown in the dialog.

### 6.4 The purge
`POST /api/desktop/flows/credentials/purge`, admin and CSRF, with `{ "snapshots": ["<dir name>", ...] }`. It starts a background job, at most one at a time. `GET /api/desktop/flows/credentials/purge` returns the job state. The purge refuses to start while a run is `running`, and new runs wait in the queue until it ends. Waiting runs do not block it.

1. **Safety backup.**
   - `VACUUM INTO` a temporary file in the data directory.
   - Encrypt it with AES-256-GCM in 64 KiB chunks, each with its own nonce. The key is derived by HKDF-SHA256 from the master key with the label `aurago/easydrag/purge-backup/v1`. The result is `data/.flows-purge-backup-<UTC>.enc`, mode 0600.
   - Remove the plaintext file.
   - The new helpers live in `internal/security`: `EncryptFileWithMasterKey(src, dst, label)` and `DecryptFileWithMasterKey(src, dst, label)`.
   - Without a master key the purge refuses to start. Its error says why.
2. **Mark and delete superseded versions.**
   - Schema 9 adds `flow_version_purge(flow_id TEXT, revision INTEGER, marked_at TEXT, PRIMARY KEY(flow_id, revision))`. The migration takes the usual verified backup and refuses older binaries.
   - Every version that holds a finding is marked, except the live revision.
   - Marked versions without a pin are deleted, together with their `flow_version_notes`.
   - Pinned ones stay marked. The existing prune path deletes them once nothing pins them any more.
   - Rollback and clone refuse a marked version with `FLOW_VERSION_PURGED` (roadmap line 550).
3. **Redact in place.**
   - Build the set of literal values from every finding in versions, runs and test data. A moved token still sits at its old place in those rows, so the rule finds it there. Drafts and live documents are left alone.
   - Rewrite these columns: `flow_runs.doc_json`, `trigger_data_json` and `checkpoint_json` (when it is plaintext JSON); `flow_run_steps.params_json` and `output_json`; `flow_test_data`; `flow_test_docs`; the child-history rows.
   - Replace each finding by the rule, and every exact occurrence of a known literal anywhere in the JSON text. Owner decision 1: runs stay.
   - `doc_hash` stays unchanged. The plan checks that no code recomputes and compares it against `doc_json`; if code does, the plan treats a redacted document as a match.
4. **Compact.** Turn on `PRAGMA secure_delete=ON` for the purge connection. Run `VACUUM`, then `PRAGMA wal_checkpoint(TRUNCATE)`.
5. **Snapshots.** Delete the chosen `.flows-v*-backup-*` directories.
6. **Verify.**
   - Search flows.db, flows.db-wal, flows.db-shm and every remaining plaintext `.flows-*` backup in the data directory, byte for byte, for each known literal. Also scan with the rule.
   - The result is clean, or a list of where something remains. The dialog shows it, and the scan cache refreshes.

If a step fails, the job stops, keeps the encrypted backup and reports the step. Running it again is safe, because every step is idempotent.

### 6.5 Backup lifetime and restore
- **Sweeper.** It deletes `.flows-purge-backup-*.enc` older than 7 days, at service start and once a day (owner decision 2).
- **Restore.** A start flag:
  - `aurago --restore-flows-purge-backup <file>` refuses to run while AuraGo is running on the same data directory.
  - It first encrypts the current `flows.db` as `.flows-pre-restore-<UTC>.enc`, with the same sweeper.
  - Then it decrypts the backup into `flows.db` and exits.
  - The documentation explains it. No UI.

### 6.6 General backups
Backups taken after a purge contain no plaintext token, because the general backup reads the purged flows.db. The dialog says that older archives are out of reach (§6.3).

## 7. Testing

Essential tests only (owner's standing preference).

**Part A**
- Unit tests: proxy authentication, slot release, proxy death, path check.
- Gated browser tests: the S8 fixture, scraper through the proxy, web capture through the proxy.
- One live run on aurago-test.

**Part B, Go**
- The rule against the shared vectors, including templates and mixed URLs.
- `http.request` with `auth_in=query`: the secret reaches only the first request, and the output URL shows the marker.
- The move endpoint for header, query and userinfo, plus 409 on a stale revision, 409 when the secret exists, and 422 for a body field.
- **The acceptance fixture** (roadmap line 319).
  1. A flow with a token in a header, in a query parameter and in a body field. It is published and run, it has test data and a migration snapshot, and one version is pinned by a waiting run.
  2. Move the header and the query token. Remove the body token with "Wert entfernen", because it is not movable. Publish again and purge.
  3. Let the pinned run end, so the prune deletes its marked version. Then a byte search of flows.db, WAL, SHM and the data directory finds none of the three tokens (amended while planning: a pinned version keeps its plaintext until it is deleted).
  4. The pinned version survives until its run ends, and is deleted then.
  5. The encrypted backup holds no plaintext.
  6. `DecryptFileWithMasterKey` restores a database that opens.
- The sweeper deletes after 7 days.

**Part B, client**
- Node: masking of the emergency copy and the clipboard, against the shared vectors; `scrubHistory`.
- Browser (gated): the guided path: the warning, the move dialog, publish, the purge offer, the purge result in the dialog.

Every browser test sets reduced motion explicitly.

## 8. Commit order

**Part A**
1. A1: the spike on proxy authentication. It decides §3.3, writes its findings into this spec and adds no product code.
2. A2: `CaptureProxy` credentials, `Done()` and the check host.
3. A3: `LaunchEgressBrowser` and its unit tests.
4. A4: the five call sites; `getSharedBrowser` and the hook deleted.
5. A5: the S8 fixture and the gated browser tests.
6. A6: the live run on aurago-test, recorded in this spec.

**Part B**
1. B1: the rule, the shared vectors, validation and redaction.
2. B2: `http.request` query auth.
3. B3: the move endpoint.
4. B4: the UI action, the locales and `scrubHistory`.
5. B5: client masking.
6. B6: schema 9, marks, and the rollback and clone guards.
7. B7: the master-key file encryption, the sweeper and the restore flag.
8. B8: the scan and the hint card.
9. B9: the purge job and the dialog.
10. B10: the acceptance fixture.

Parts A and B touch separate files and can run in parallel, in two worktrees. Part A is smaller and lands first.

## 9. Acceptance checklist
- [ ] Every site in §3.5 uses `LaunchEgressBrowser`. `getSharedBrowser` and `GuardRodPageSSRF` are gone.
- [ ] The proxy binds to loopback on a random port and has no credentials (owner decision 7). It fails the step closed when it dies, and the path check catches a bypass.
- [ ] The S8 fixture is refused locally, and once live on aurago-test (roadmap line 320).
- [ ] One rule drives validation, redaction and client masking. Go and JS pass the same vectors.
- [ ] "In den Tresor verschieben" works for a header, a query parameter and URL user info. The value never travels back to the browser.
- [ ] After a move, a republish and a purge, no plaintext token is found in the fixture's flows.db, its WAL or any flows backup in the data directory. Versions still pinned by a run survive until they are unpinned (roadmap line 319).
- [ ] The safety backup is encrypted, can be restored with the flag, and is gone after 7 days.
- [ ] Emergency copy, clipboard and undo carry no moved token.
- [ ] Every new key is in all 16 locales; every new FLOW code has a registry entry; `TestShippedWebAssetsLoadNothingExternal` stays green.

## 10. Risks
- **Proxy authentication in Chrome.** The spike (A1) decides. If Chrome cannot answer the challenge for service workers or WebSockets, the plan stops and asks.
- **False positives of the rule.** A warning is cheap, but redaction of a harmless field in exports would annoy. The name list is deliberately narrow (`key` counts only in queries), and the format list needs a fixed prefix.
- **VACUUM on a large flows.db** takes time and needs free disk space of about the size of the file, twice over, counting the backup. The purge checks free space first and refuses with a clear message.
- **Plaintext temporary file** during the safety backup. It lives seconds in the data directory and is then removed. The filesystem may keep its blocks; the dialog does not promise secure erase of the disk.
- **Other sessions commit in the main checkout.** Each part runs in its own worktree.
