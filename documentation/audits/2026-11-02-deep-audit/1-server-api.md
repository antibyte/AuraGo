# Audit: Server, HTTP-API & Service Layer

## Scope & method

**Area:** Area 1 - Server, HTTP-API & Service Layer.

**Packages covered (production `.go` files, excluding `_test.go`):**

| Package | Files | LOC |
| --- | --- | --- |
| `internal/server/**` | 555 | 149,689 |
| `cmd/aurago/**` | 9 | 1,785 |
| `internal/setup/**` | 3 | 924 |
| `internal/media/**` | 2 | 663 |
| `internal/webassets/**` | 3 | 467 |
| `internal/commands/**` | 6 | 469 |
| `internal/i18n/**` | 2 | 447 |
| `internal/heartbeat/**` | 3 | 358 |
| `internal/updater/**` | 1 | 338 |
| `internal/integrationstatus/**` | 2 | 323 |
| `internal/logger/**` | 3 | 276 |
| `internal/httpstream/**` | 1 | 104 |
| `internal/httporigin/**` | 1 | 50 |
| `internal/uid/**` | 1 | 22 |
| **Total** | **592** | **~155,900** |

**Tooling.** The `graft` **MCP server timed out on every call** (120 s timeout, 5 attempts: `graft_check_freshness`, `graft_find_code` x2, `graft_find_all`; only the very first `graft_repo_map` succeeded). All graft work therefore went through the **graft CLI** from the repo root. The working tree is dirty (97 changed paths), which forced graft to rebuild the graph on every invocation; `--no-refresh` was used for the sweeps afterwards to make them usable.

Graft/graph calls made (~19): `graft repo_map`; `graft check`; `graft skeleton` (`internal/server/server.go`, `internal/server/desktop_auth.go`, `cmd/aurago/main.go`, `internal/commands/commands.go`); `graft ask` (route registration); `graft grep --no-refresh` over `internal/server` for `mux.HandleFunc`, `http.Server{`, `http.Client{`, `MaxBytesReader|LimitReader`, `json.NewDecoder`, `jsonError(w, err.Error()...)`, `exec.Command`, `http.DefaultClient|http.Get|http.Post`, `io.ReadAll(resp.Body)`, `InsecureSkipVerify`, `filepath.Join(...r.URL...)`; `graft callers NewInternalHTTPClient --depth 2`.

Supplementary cross-checks used ripgrep (`grep` tool) and `read` for exact spans, plus three PowerShell sweeps over `internal/server` for duplicate route literals, shallow-config-copy counts and `jsonError`/`security.Scrub` counts.

**Key questions answered** (summary in *Architecture map* and *Findings*): route registration is spread across ~30 `register*Routes` functions with **no single route table**; a **global `authMiddleware` wraps the whole mux** and there is a second, deliberately-ungated auth tier for API Bearer tokens; **no handler is reachable without auth** except the documented bypass list and the separately-bound Chromecast media server; **response bodies are all closed**; **request bodies are all size-limited**; **outbound `http.Client`s are timeout-bounded**; `http.Server`s are timeout-bounded but the main one lacks `ReadHeaderTimeout`.

**What I could NOT cover (honest gaps):**

- **No GitNexus `impact` runs.** I never assert that a symbol is safe to change or that a change breaks something, so this was not required for the findings below, but it means I have **no graph-verified blast radius** for any finding.
- `internal/server` is 150k LOC; I read ~40 files in full or in large spans and swept the rest with regex. Files I did **not** open at all include `agodesk_handlers.go` (3,497 LOC), `virtual_computers_handlers.go` (2,008), `cyd_creator_builder.go` (1,507), `code_studio_handlers.go` (1,504), `video_studio_handlers.go` (1,480), `desktop_bookshelf_handlers.go` (1,234), `sip_handlers.go` (1,174), `invasion_hatch_handlers.go` (1,167), `realtime_speech_handlers.go` (1,119), `mailer_oauth.go` (1,081). These are **unreviewed** and are the most likely home of further defects.
- `internal/setup/**` and `internal/integrationstatus/**` were located but only skimmed; `internal/i18n`, `internal/uid`, `internal/logger` were confirmed present and in scope but not audited in depth.
- The UI (`ui/`), TUI (`tools/aurago-tui`), sidecar and `decisions_runtime/` were **out of scope** and not audited.
- No build, `go vet`, `go test`, or race-detector run was performed (read-only constraint; a `-race` run would be the cheapest way to confirm F-1).

---

## Architecture map

- **Entry point:** `cmd/aurago/main.go:152` - `main()` is ~1,234 lines and does config load -> vault init -> server construction -> signal handling -> shutdown drain in one function.
- **Server construction:** `internal/server/server.go:864` `(*Server).Start()` (~987 lines) builds the `*Server` struct (555-file package; field block at `internal/server/server.go:299-305`).
- **HTTP servers:** three distinct servers exist -
  1. main agent server `internal/server/server_internal_http.go:67` `newAgentHTTPServer` (ReadTimeout/WriteTimeout/IdleTimeout, **no `ReadHeaderTimeout`**),
  2. internal loopback server `internal/server/server_internal_http.go:77` `newInternalLoopbackServer`,
  3. Chromecast/TTS media server `internal/server/server_routes_ui.go:1277` `ttsServer` (own mux, **no auth middleware**).
- **Middleware chain (main server):** `internal/server/server.go:2070` - `trustedProxyMiddleware -> previewHostMiddleware -> desktopTicketMiddleware -> panicRecoveryMiddleware -> gzipMiddleware -> accessLogMiddleware -> securityHeadersMiddleware -> authMiddleware -> mux`.
  - `internal/server/trusted_proxy.go:16` strips client-supplied `X-Forwarded-*` unless the peer is in `TrustedProxyCIDRs`.
  - `internal/server/server.go:2248` `securityHeadersMiddleware` sets CSP/HSTS/frame/referrer/cache headers.
  - `internal/server/auth.go:924` `authMiddleware` is the single auth gate; `internal/server/auth.go:838` `isAuthBypassed` consults `authBypassExactPaths` / `authBypassSubtrees` (login, auth status, health, static UI assets, Telnyx webhook, Space-Agent bridge).
  - `internal/server/auth.go:1164` `validRouteBearer` is the Bearer-token tier: admin scope for generic paths (`:1203`), per-path scope for `/api/desktop/**` and `/api/go2rtc/**`.
  - `internal/server/desktop_auth.go:133` `requireDesktopPermission` is the per-route desktop-scope gate.
- **Routing:** ~787 literal `mux.Handle`/`mux.HandleFunc` registrations spread over ~30 `register*Routes` functions in `internal/server/server_routes.go` (1,115 LOC), `server_routes_ui.go` (1,231), `server_routes_config.go` (619+), `server_routes_infrastructure.go` (217) - **no single source of truth**.
- **Error boundary:** `internal/server/invasion_handlers.go:858` `writeJSON`, `:863` `jsonError`, `:869` `jsonErrorWithDetails` - the only JSON writers used across the package.
- **Config mutation path:** `internal/server/config_handlers_main.go:489` `handleUpdateConfig` (~1,306 lines) - validates a patch, writes `config.yaml`, writes the Vault, reloads, then fans out to subsystem restarts (`internal/server/config_handlers_main.go:1210-1214` restarts the heartbeat scheduler).
- **Config snapshot:** `internal/config/snapshot.go:9` `(*Config).Clone()` - reflection deep-copy; the canonical safe way to snapshot a config.
- **File serving:** `internal/server/fileserver.go` - root-bound filesystem (`os.Root`, symlink refusal, no FIFO blocking); `internal/server/server_routes_ui.go:1229` serves the workspace dir; `internal/server/server_routes_ui.go:1235-1255` serves `/tts/` and `/cast-media/`.
- **Streams:** `internal/server/sse.go` - bounded 128-buffer broadcaster with stale-client eviction; `internal/server/go2rtc_websocket.go` - per-connection read limits, deadlines, candidate cap, joined goroutine.
- **Asset pipeline:** `internal/webassets/assets.go` / `install.go` / `import.go` - SHA-256-pinned, size-capped tar/gzip install with manifest allow-list and `flock`.
- **Outbound HTTP:** `internal/server/server_internal_http.go:85` `NewInternalHTTPClient` (loopback-only, `InsecureSkipVerify` justified); `internal/updater/updater.go:145` `Download` (host-pinned + redirect allow-list).

---
## Findings

### F-1 [HIGH] Heartbeat scheduler: `sync.WaitGroup` misuse + uninterruptible `Wait()` - server can hang forever on shutdown

**Location**
- `internal/heartbeat/scheduler.go:43-92` (`Start`), `internal/heartbeat/scheduler.go:95-110` (`Stop`), `internal/heartbeat/scheduler.go:113-119` (`Restart`)
- Callers: `internal/server/config_handlers_main.go:1210-1214` (`Restart`), `internal/server/server.go:2173-2175` (`Stop`), `internal/server/server.go:1343` (`Start`)
- Hang path: `internal/server/server.go:2126` (`defer close(shutdownDone)`), `internal/server/server.go:2238` (`<-shutdownDone`)

**Evidence** - `internal/heartbeat/scheduler.go:43-110`:

```go
func (s *Scheduler) Start() {
	s.mu.Lock()

	// If a loop is already running, close its channel and wait for it to exit
	// before starting a new one.
	if s.stopCh != nil {
		ch := s.stopCh
		s.stopCh = nil
		s.mu.Unlock()

		select {
		case <-ch:
			// already closed
		default:
			close(ch)
		}
		s.wg.Wait()

		s.mu.Lock()
	}
	...
	ch := make(chan struct{})
	s.stopCh = ch
	s.wg.Add(1)
	go s.loop(ch)
	...
	s.mu.Unlock()
	...
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	ch := s.stopCh
	s.stopCh = nil
	s.mu.Unlock()

	if ch != nil {
		select {
		case <-ch:
			// already closed
		default:
			close(ch)
		}
		s.wg.Wait()
	}
}
```

`Stop()` clears `s.stopCh` **before** closing the channel and then blocks in `s.wg.Wait()` with **no timeout and no context**. `Start()` performs `s.wg.Add(1)` (line 76) *after* observing `s.stopCh == nil`. Go's `sync.WaitGroup` contract states that a positive `Add` must happen before a `Wait` when the counter is zero; here `Add(1)` can run **concurrently** with the other goroutine's `Wait()`.

**Why it matters.** Two independent triggers exist:

1. *Config save during shutdown.* `handleUpdateConfig` calls `s.HeartbeatScheduler.Restart(newCfg)` (`internal/server/config_handlers_main.go:1212`) -> `Stop()` then `Start()`. Concurrently, `serveWithShutdown` calls `s.HeartbeatScheduler.Stop()` (`internal/server/server.go:2174`). Interleaving: shutdown-`Stop()` reads `ch = c1`, sets `s.stopCh = nil`, releases the lock; the config-save `Restart()` then runs `Start()`, sees `s.stopCh == nil`, creates `c2`, does `s.wg.Add(1)`, and starts `loop(c2)`. The shutdown `Stop()` now calls `s.wg.Wait()`, whose counter is now `1` for `loop(c2)` - and `loop(c2)` only exits when `c2` is closed, which nothing will do. `s.wg.Wait()` **never returns**.
2. *Two concurrent `Restart()` calls.* `handleUpdateConfig` holds `s.CfgSaveMu` (`internal/server/config_handlers_main.go:574-575`), so two config saves are serialised and cannot race each other - but `Stop()` (shutdown) does **not** take `CfgSaveMu`, so trigger 1 stands.

Because `s.HeartbeatScheduler.Stop()` is called at `internal/server/server.go:2174`, *before* the `defer close(shutdownDone)` at `internal/server/server.go:2126` can run, a hang there means `shutdownDone` is never closed, `<-shutdownDone` at `internal/server/server.go:2238` blocks forever, `Start()` never returns, and the process never exits. The 45-second shutdown context created at `internal/server/server.go:2130` does **not** cover this call.

**Fix** - `internal/heartbeat/scheduler.go`:
- Hold `s.mu` for the whole `Start()`/`Stop()` transition, or replace the `stopCh` + `wg` pair with a per-generation `done` channel so a `Wait()` can never outlive the loop it was waiting for.
- At minimum, make `Stop()` idempotent and safe against a concurrent `Start()`: capture `ch` **and** `done` under the lock, and wait on that specific `done` channel instead of the shared `s.wg`. Replace `s.wg.Wait()` with `<-done`.
- Additionally give `Stop()` a context/timeout so the shutdown goroutine cannot be parked forever.

**Confidence:** Confirmed (code read end-to-end; the `WaitGroup` contract violation and the unsynchronised `stopCh`/`wg` pairing are visible in the source). The *interleaving* is Probable rather than demonstrated - I did not run it under `-race`. **Verify with:** a table test that calls `Restart` and `Stop` from two goroutines with a `go test -race -timeout 30s` bound, asserting `Stop()` returns within a deadline.

---

### F-2 [MEDIUM] Dedicated Chromecast/TTS media server binds `0.0.0.0` even when the main server is bound to `127.0.0.1`

**Location**
- `internal/server/server_routes_ui.go:65-73` (`chromecastMediaServerBindHost`)
- `internal/server/server_routes_ui.go:1257-1298` (dedicated server construction), `:1277-1285` (`ttsServer` literal)
- `internal/config/config.go:1914-1916` (default `Server.Host`)

**Evidence** - `internal/server/server_routes_ui.go:65-73`:

```go
func chromecastMediaServerBindHost(host string) string {
	normalized := strings.TrimSpace(strings.ToLower(host))
	switch normalized {
	case "", "0.0.0.0", "127.0.0.1", "localhost", "::1", "[::1]":
		return "0.0.0.0"
	default:
		return host
	}
}
```

`internal/config/config.go:1914-1916`:

```go
	if cfg.Server.Host == "" {
		cfg.Server.Host = "127.0.0.1"
	}
```

`internal/server/server_routes_ui.go:1277-1285`:

```go
		ttsServer = &http.Server{
			Addr:              fmt.Sprintf("%s:%d", ttsHost, s.Cfg.Chromecast.TTSPort),
			Handler:           s.trackHTTP(httpstream.WithWriteTimeout(ttsMux, 30*time.Second)),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			IdleTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			MaxHeaderBytes:    16 << 10,
		}
```

**Why it matters.** The default deployment binds the main API to `127.0.0.1` (loopback only). `chromecastMediaServerBindHost` maps that same `"127.0.0.1"` to **`0.0.0.0`**, so enabling `chromecast.enabled` silently publishes an additional HTTP listener on **every network interface** - strictly wider than the surface the operator configured for the main server. The dedicated server's handler is `s.trackHTTP(httpstream.WithWriteTimeout(ttsMux, ...))` only: it bypasses `authMiddleware`, `securityHeadersMiddleware`, `accessLogMiddleware` and `gzipMiddleware` that the main server uses. There is **no config key** for the TTS server's bind address, so the operator cannot restrict it.

The exposure is **not** unauthenticated file serving - `castMediaAssetHandler(ccTTSDir, "/tts/", true)` (`internal/server/server_routes_ui.go:1265`) requires a ticket, and `security.ValidCastMediaTicket` (`internal/security/cast_media_ticket.go:49`) verifies an HMAC-SHA256 signature over path+expiry with `hmac.Equal` against a per-process random key. The residual risks are: (a) the media surface is LAN-reachable even when the API is not; (b) **there is no access log** on that listener, so LAN access to it is untraceable; (c) signed URLs embedded in query strings are logged by Chromecast devices and any intermediary.

**Fix** - `internal/server/server_routes_ui.go:65-73`: stop widening the bind address. Return `"127.0.0.1"` for loopback inputs and let the operator opt into LAN exposure explicitly; add a `server.chromecast.tts_bind_host` config key defaulting to the effective `server.host`. Also route the TTS server through `accessLogMiddleware` (or a minimal equivalent) so LAN access is auditable.

**Confidence:** Confirmed (source read; default host confirmed at `internal/config/config.go:1914-1916`, ticket requirement confirmed at `internal/server/cast_media_handler.go:20-23` and `internal/security/cast_media_ticket.go:49-64`).

---
### F-3 [MEDIUM] HTTP boundary returns raw internal error strings - 437 call sites bypass the repo's own secret scrubber

**Location**
- `internal/server/invasion_handlers.go:863-876` (`jsonError`, `jsonErrorWithDetails`)
- Example call sites: `internal/server/config_handlers_main.go:531, 543, 547, 608, 621, 660, 665, 682, 686, 690, 694, 698`
- The scrubber that exists but is not used: `internal/security/scrubber.go:167-172`

**Evidence** - `internal/server/invasion_handlers.go:863-876`:

```go
func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func jsonErrorWithDetails(w http.ResponseWriter, msg string, details string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   msg,
		"details": details,
	})
}
```

`jsonError` writes `msg` verbatim. A sweep of `internal/server` production files counted **437 occurrences of `jsonError(w, err.Error())` across 71 files**, against **123 `security.Scrub` call sites** in the same package. One call site in the same file does redact (`internal/server/config_handlers_main.go:616` uses `security.Scrub(err.Error())`), which shows the policy exists but is applied inconsistently.

**Why it matters.** Every validator, filesystem, SQL and upstream-provider error that reaches a client is disclosed verbatim. Typical content: absolute server paths (`C:\Users\<u>\.aurago\config.yaml`), internal identifier names, YAML parse positions, and upstream provider error payloads - upstream auth errors frequently **echo the submitted credential**. Bearer-token API clients receive the same text, and it typically lands in their logs and any proxy log in between. The secret that `internal/security/scrubber.go` was built to redact is redacted in 123 places and leaked in 437.

**Fix** - `internal/server/invasion_handlers.go:863`: make redaction the default rather than opt-in, e.g. `jsonError` applies `security.Scrub(msg)` before encoding (the scrubber is a no-op on strings containing no registered secret, so this is cheap and safe), and keep the raw text in the server log only. Where the raw detail is genuinely needed by the owner, use `jsonErrorWithDetails` behind the existing admin/browser-session tiers.

**Confidence:** Confirmed for the counts and for the bypass mechanism (both numbers reproduced by sweep). **Needs-verification** for a *specific* secret-bearing error: I did not find a validator inside `internal/config` whose message embeds a credential value (a targeted `Errorf(...%[sv]...[Kk]ey|token|secret|password)` sweep over `internal/config` returned no matches), so the concrete leak is currently *internal-detail disclosure* rather than a proven credential leak. **Verify with:** register a fake secret with the scrubber, then drive a config-save validation error and assert the response body no longer contains it.

---

### F-4 [MEDIUM] Oversized functions violate the project's own "one responsibility per file" rule

**Location**
- `cmd/aurago/main.go:152` - `main()`
- `internal/server/config_handlers_main.go:489` - `handleUpdateConfig`
- `internal/server/server.go:864` - `(*Server).Start()`
- `internal/server/server_routes_ui.go:688` - `registerUIRoutes`
- `internal/server/backup_handlers.go:449` / `:1023` - `handleBackupCreate` / `handleBackupImport`

**Evidence.** Line spans taken from the graft graph (`graft skeleton` / `graft ask`), cross-checked against file lengths measured directly:

| Function | Span | Lines |
| --- | --- | --- |
| `cmd/aurago/main.go` -> `main()` | L152-L1386 | ~1,234 |
| `internal/server/config_handlers_main.go` -> `handleUpdateConfig` | L489-L1795 | ~1,306 |
| `internal/server/server.go` -> `(*Server).Start()` | L864-L1851 | ~987 |
| `internal/server/server_routes_ui.go` -> `registerUIRoutes` | L688-L1301 | ~613 |
| `internal/server/backup_handlers.go` -> `handleBackupCreate` | L449-L1018 | ~570 |

Largest files in `internal/server` (non-test): `agodesk_handlers.go` 3,497 LOC, `config_handlers_main.go` 2,846, `server.go` 2,349, `virtual_computers_handlers.go` 2,008, `cyd_creator_builder.go` 1,507, `code_studio_handlers.go` 1,504, `video_studio_handlers.go` 1,480, `backup_handlers.go` 1,388.

**Why it matters.** `AGENTS.md` states *"One responsibility per file; split oversized files"* - these functions are 10-40x that guidance and each mixes several concerns (validation + persistence + Vault writes + subsystem restart fan-out in `handleUpdateConfig`; server construction + route registration + goroutine spawning + signal handling in `Start()`). The concrete failure mode is regression risk in exactly the paths F-1 and F-5 live in: `handleUpdateConfig` holds `s.CfgSaveMu` across ~1,300 lines and fans out to subsystem restarts; `main()` is untestable end-to-end because everything is inline.

**Fix** - split along existing seams, lowest-risk first: `handleUpdateConfig` -> `validateConfigPatch` / `persistConfigPatch` / `applyConfigSideEffects`; `main()` -> `run()` / `setupSignalHandler()` / `drainOnShutdown()`; `Start()` -> `buildServer()` / `registerRoutes()` / `startSubsystems()`.

**Confidence:** Confirmed (line spans from the graft graph, file lengths measured directly).

---

### F-5 [MEDIUM] 41 shallow `*s.Cfg` copies bypass the existing `config.Config.Clone()` deep-copy helper

**Location**
- The helper that exists: `internal/config/snapshot.go:5-14` (`(*Config).Clone`)
- Shallow copies: 41 occurrences across 26 production files in `internal/server` - e.g. `internal/server/webhook_handlers.go:541-545`, `internal/server/config_handlers_main.go:851`, `internal/server/config_handlers_main.go:970`, and 38 more.

**Evidence** - `internal/config/snapshot.go:5-14`:

```go
// Clone detaches the mutable values of a configuration while preserving runtime
// callbacks and unexported value state (for example time.Time). Configurations
// are acyclic data; services and synchronization primitives must not be stored in
// them. A publisher must rebind AuthorizationSnapshots on the returned copy.
func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}
	return cloneConfigValue(reflect.ValueOf(c)).Interface().(*Config)
}
```

`internal/server/webhook_handlers.go:541-545`:

```go
	s.CfgMu.RLock()
	current := *s.Cfg
	current.Webhooks.Outgoing = append([]config.OutgoingWebhook(nil), s.Cfg.Webhooks.Outgoing...)
	configPath := current.ConfigPath
	s.CfgMu.RUnlock()
```

**Why it matters.** `Config` is a large struct of nested maps and slices (`internal/config/config_types.go:1137` onward). `current := *s.Cfg` copies the struct **shallowly**, so every map and slice field in `current` is *the same* backing storage as the live config's. The code above shows the author is aware for one field (`Outgoing` is re-copied explicitly) but nothing enforces that for the other ~40 sites. Any handler that later writes into `current.<SomeMap>[k] = v` mutates the live configuration **without** holding `s.CfgMu` - a data race and a config-integrity violation that no compiler will catch. `Clone()` exists precisely to prevent this and is simply not used at these sites.

**Fix** - replace the 41 shallow copies with `s.Cfg.Clone()` (`internal/config/snapshot.go:9`) and keep `s.CfgMu` held only long enough to clone. Where a snapshot is compared later (e.g. `reflect.DeepEqual` on `oldCfg` vs `newCfg` at `internal/server/config_handlers_main.go:970`), a deep clone is also the correct semantics - a shallow snapshot can compare equal while sharing mutated storage.

**Confidence:** Confirmed for the count (41 / 26 files, swept) and for the existence of `Clone()`. **Probable** that at least one of the 41 sites mutates a shared map/slice - I did not read all 41 call sites individually. **Verify with:** `go test -race` over `internal/server` config-save tests, or a targeted read of the 41 sites for a write into `current.*` after the lock is released.

---

### F-6 [MEDIUM] `/debug/pprof` is unauthenticated whenever `debug_pprof` is enabled and `auth.enabled` is false

**Location**
- `internal/server/pprof_routes.go:8-16` (`registerPProfRoutes`)
- `internal/server/server_routes.go:515` (conditional registration)
- `internal/config/config_types.go:1165` (`DebugPProf bool` with `yaml:"debug_pprof"`)

**Evidence** - `internal/server/pprof_routes.go:8-16`:

```go
// registerPProfRoutes wires the standard net/http/pprof handlers into mux.
// Callers must ensure this is only exposed in trusted/debug environments.
func registerPProfRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
}
```

`internal/config/config_types.go:1165`:

```go
			DebugPProf           bool   `yaml:"debug_pprof"`             // expose /debug/pprof endpoints (default false)
```

**Why it matters.** The handlers are registered directly on the shared `mux`, so they *do* pass through `authMiddleware` - but `authMiddleware` short-circuits and allows everything when `auth.enabled` is false (the default for a local install). The result is a remote, unauthenticated `/debug/pprof/profile?seconds=30` (sustained 100% CPU for 30 s, repeatable), `/debug/pprof/heap` and `/debug/pprof/goroutine` (full memory and stack disclosure of every goroutine, including file paths and config state). `pprof.Profile` is a trivially-scripted CPU-exhaustion vector. The config flag defaults to `false`, and `internal/config/config.go:1917` even carries the comment *"DebugPProf defaults to false for security; must be explicitly enabled"* - but nothing stops an operator who enables it on a LAN-exposed instance, and nothing gates it behind auth when `auth.enabled` is false.

**Fix** - `internal/server/server_routes.go:515`: gate the registration on **both** `DebugPProf` **and** an explicit `debug_pprof_allow_unauthenticated` (default `false`), or wrap the pprof handlers in a middleware that requires an admin-scoped Bearer token / loopback peer (`r.RemoteAddr` in `127.0.0.0/8`) regardless of `auth.enabled`. Consider keeping `pprof.Index`/`Symbol` while `Profile`/`Trace` stay disabled by default.

**Confidence:** Confirmed (source read; `authMiddleware`'s allow-all-when-disabled behaviour confirmed at `internal/server/auth.go:924`+). The *reachability* claim depends on the operator enabling `debug_pprof` with `auth.enabled: false` - the default is off, so this is a latent, not an active, exposure.

---
### F-7 [LOW] The main HTTP server omits `ReadHeaderTimeout`; the auxiliary server sets it

**Location**
- `internal/server/server_internal_http.go:67-75` (`newAgentHTTPServer`)
- `internal/server/server_routes_ui.go:1277-1285` (`ttsServer`)

**Evidence** - `internal/server/server_internal_http.go:67-75`:

```go
func newAgentHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:         addr,
		Handler:      httpstream.WithWriteTimeout(handler, agentHTTPWriteTimeout),
		ReadTimeout:  agentHTTPReadTimeout,
		WriteTimeout: agentHTTPWriteTimeout,
		IdleTimeout:  agentHTTPIdleTimeout,
	}
}
```

The auxiliary TTS server at `internal/server/server_routes_ui.go:1280` sets `ReadHeaderTimeout: 5 * time.Second`; the main server - the one actually exposed to the network - does not. A sweep of `internal/server` for `ReadHeaderTimeout|WriteTimeout|IdleTimeout|ReadTimeout` found no `ReadHeaderTimeout` on the main or TLS servers.

**Why it matters.** `ReadTimeout` (30 s) does bound the header read, so this is *not* an unbounded Slowloris. But `ReadHeaderTimeout` gives a much sharper bound specifically on header parsing, and Go's own documentation recommends it precisely for this. The inconsistency with the auxiliary server shows the knob is understood in this codebase. Severity is LOW because `ReadTimeout` is a backstop.

**Fix** - `internal/server/server_internal_http.go:68`: add `ReadHeaderTimeout: 10 * time.Second` (and the same on the TLS servers in `internal/server/tls.go`).

**Confidence:** Confirmed (both literals read; absence verified by sweep).

---

### F-8 [LOW] `internal/updater` reads the whole release-API response with no size cap

**Location** - `internal/updater/updater.go:248` (`fetchLatestRelease`)

**Evidence** - `internal/updater/updater.go:248`:

```go
	raw, err := io.ReadAll(resp.Body)
```

**Why it matters.** The response is unbounded. The repo's established pattern elsewhere is `io.LimitReader(resp.Body, 1<<20)` (e.g. `internal/server/config_handlers_providers.go:486`) or `io.LimitReader(resp.Body, (64<<10)+1)` (`internal/server/oauth_handlers.go:431`). The 10 s client timeout bounds *time*, not *bytes*: a hostile or compromised endpoint can push tens of MB in 10 s and the process will buffer all of it. The URL is `opts.ReleaseAPIURL` or the GitHub API (`internal/updater/updater.go`), i.e. server-side config rather than request input, so exploitation requires control of that endpoint - which is exactly the scenario the limit should cover.

**Fix** - `internal/updater/updater.go:248`: `raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))` and reject over-length bodies.

**Confidence:** Confirmed (line read; the `io.ReadAll` is unguarded).

---

### F-9 [LOW] `internal/updater` builds a shell command with Go `%q` quoting instead of shell quoting

**Location** - `internal/updater/updater.go:344-346` (`DefaultStartScript`)

**Evidence** - `internal/updater/updater.go:344-346`:

```go
	wrapper := fmt.Sprintf("cd %s && nohup %s %s >> %s 2>&1 &",
		quoteShellPath(launch.ScriptDir),
		quoteShellPath(launch.BashPath),
		quoteShellPath(launch.ScriptPath),
		quoteShellPath(launch.LogPath))
```

**Why it matters.** `quoteShellPath` is Go's `%q` string-literal quoting, not POSIX shell quoting. Go escapes `"` and `\` but leaves `$`, backtick, `!` and `&` untouched, so a path containing `$(...)` or a backtick is expanded by the shell that runs the script. The paths derive from `opts.InstallDir` -> `filepath.Dir(cfg.ConfigPath)`, and `ConfigPath` is resolved at load time (`cmd/aurago`) rather than from request input, so this is **not currently remotely reachable**. It is a latent injection primitive: any future path that becomes user-influenced (setup wizard, a `--config` flag on a shared host, an installer directory containing `$`) turns into command injection in a script that receives `AURAGO_MASTER_KEY` in its environment (`internal/updater/updater.go`, `updateEnv`).

**Fix** - `internal/updater/updater.go`: replace `quoteShellPath` with single-quote POSIX quoting (`'` -> `'\''`) or, better, drop the shell entirely and exec the script directly (`exec.Command(launch.BashPath, launch.ScriptPath)` with `Dir` set to `launch.ScriptDir`), removing the need for `nohup`/redirection.

**Confidence:** Confirmed (code read). **Needs-verification** on reachability: I confirmed `ConfigPath` is not request-controlled, so this is a hardening item rather than a live vulnerability.

---

### F-10 [LOW] `jsonError` / `writeJSON` discard the encoder error, hiding truncated responses

**Location** - `internal/server/invasion_handlers.go:858-867`

**Evidence:**

```go
func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
```

`json.NewEncoder(w).Encode(v)` returns an error that is discarded; the same pattern appears in `jsonError` and `jsonErrorWithDetails` (`:866`, `:874`).

**Why it matters.** When the client disconnects mid-write (or the response writer fails), `Encode` returns an error that is silently dropped - the server logs nothing, the metrics show a "successful" 200, and the failure is invisible to both operator and client. On a 150k-LOC package where `writeJSON` is the standard response helper, this is a systematic blind spot in error reporting.

**Fix** - `internal/server/invasion_handlers.go:858`: check the error and log it at debug/warn level (`if err := json.NewEncoder(w).Encode(v); err != nil { logWarn(...) }`). Keep writing the status code first, since the response is already committed.

**Confidence:** Confirmed (code read).

---

### F-11 [LOW] `handleMediaList` loads and filters the entire media registry before paginating

**Location** - `internal/server/media_handlers.go` (`handleMediaList`, `filterDisplayableMediaItems`)

**Evidence.** `handleMediaList` calls `searchAllMediaForServerWithTags` (which pages the whole registry in 1,000-row batches into memory), then applies `filterDisplayableMediaItems`, and only then slices `filtered[offset:end]` for the requested page.

**Why it matters.** The `limit`/`offset` parameters do not bound the work: every request scans and materialises all matching media rows before returning at most 200. As the media registry grows (this is a media-serving product with an image/video library), list latency and peak memory scale with total library size, not page size, and concurrent list requests multiply the cost. The offset/end arithmetic itself is safe - `offset` is clamped to `total` and `end` is clamped to `total` - so this is a performance/resource issue, not a panic.

**Fix** - push the displayability filter and the pagination into the query (or at least stop accumulating all rows before slicing), so `limit`/`offset` bound the work.

**Confidence:** Confirmed for the load-then-slice ordering (read). The precise scaling characteristics are **Needs-verification** - I did not benchmark it or measure the registry size on a real instance.

---

### F-12 [LOW] Four route patterns share one handler that dispatches on the raw request path, straddling two authorisation tiers

**Location**
- `internal/server/server_routes_config.go:476-479` (registration)
- `internal/server/dashboard_memory_curation.go:21-47` (`handleDashboardMemoryCuration`)

**Evidence** - `internal/server/server_routes_config.go:476-479`:

```go
	mux.HandleFunc("/api/dashboard/memory/curation", handleDashboardMemoryCuration(s))
	mux.HandleFunc("/api/dashboard/memory/curation/dry-run", handleDashboardMemoryCuration(s))
	mux.Handle("/api/dashboard/memory/curation/apply", requireAdmin(s, handleDashboardMemoryCuration(s)))
	mux.Handle("/api/dashboard/memory/curation/", requireAdmin(s, handleDashboardMemoryCuration(s)))
```

The handler then branches on `r.URL.Path` (`internal/server/dashboard_memory_curation.go:21-47`) to pick List / DryRun / Apply / DocAction.

**Why it matters.** I checked this specifically for an authz bypass and **did not find one**: the exact-path `apply` pattern is more specific than the `/curation/` subtree, so `apply` always lands on the `requireAdmin` wrapper, and the un-wrapped `/curation` and `/curation/dry-run` paths can only reach the read-only List/DryRun branches. The defect is structural, not an exploitable bypass: the privilege tier of a request is decided by *which pattern matched*, while the behaviour is decided by *string comparison inside the handler*. The two must be kept in sync by hand. The same shape repeats at `internal/server/server_routes_config.go:481-482` (`/api/dashboard/memory/hygiene` and `/api/dashboard/memory/hygiene/dry-run` registered without `requireAdmin`, while `:480` uses it).

A secondary observation: `requireAdmin` on these generic paths is effectively a **no-op**. `validRouteBearer` (`internal/server/auth.go:1164`) requires the `"admin"` scope for *any* path that is not admin-protected, not desktop-scoped and not `/api/go2rtc/**` (`:1203-1204`), and `requireAdmin` (`internal/server/auth.go:453`) treats browser sessions as the built-in admin identity. So the wrappers add no protection beyond the bearer fallback - the actual privilege boundary is session-vs-bearer, and the route table's appearance of tiering is misleading to a reader.

**Fix** - `internal/server/server_routes_config.go:476-479`: register four distinct handler funcs (one per path) instead of one path-dispatching handler, so the route table and the handler body cannot drift; and either drop the no-op `requireAdmin` wrappers here or reduce `validRouteBearer`'s generic fallback to a non-admin scope so the wrapper actually means something.

**Confidence:** Confirmed (registration and dispatch both read; `validRouteBearer` fallback confirmed at `internal/server/auth.go:1203-1204`).

---

### F-13 [LOW] `DoInternalRequestWithStartupRetry` falls back to `http.DefaultClient` (no timeout)

**Location** - `internal/server/server_internal_http.go:102-105`

**Evidence:**

```go
func DoInternalRequestWithStartupRetry(ctx context.Context, client *http.Client, method, rawURL string, body []byte, headers http.Header, maxWait time.Duration) (*http.Response, error) {
	if client == nil {
		client = http.DefaultClient
	}
```

**Why it matters.** `http.DefaultClient` has **no** `Timeout` and no `ResponseHeaderTimeout`. All current call sites pass an explicit client (`internal/server/server.go:1386`, `internal/server/agentmail_handlers.go:178`, `internal/server/fritzbox_runtime.go:57`, `cmd/aurago/main.go:905,1112,1390`), so this is currently unreachable in practice - but the fallback silently disables the timeout guarantee the rest of the code relies on, and it is exactly the kind of default that bites when a new caller is added.

**Fix** - `internal/server/server_internal_http.go:103`: replace the fallback with `&http.Client{Timeout: maxWait}` (or return an error) rather than `http.DefaultClient`.

**Confidence:** Confirmed (code read; all current callers pass a client).

---
## Areas that are in good shape (stated explicitly, no findings manufactured)

These were examined specifically because the audit brief asks about them, and they held up:

- **Route -> auth wiring.** Every route lives on the single main `mux`, and that mux is wrapped once by `authMiddleware` (`internal/server/server.go:2070`). I enumerated **787 literal `mux.Handle`/`mux.HandleFunc` registrations** across `internal/server` and checked every duplicate: the only repeats are `/api/auth/status`, `/auth/login`, `/auth/logout`, `/` (in `assets_handlers.go`, which belongs to the separate asset-recovery server, `internal/server/assets_handlers.go:35,67,75,82`) and `/api/webhooks` (`internal/server/server_routes_config.go:431` vs `:465`), which sit in **mutually exclusive `if/else` branches** of the same `if s.WebhookManager != nil` test. **`mux.HandleFunc` panics on a duplicate pattern**, so this was worth checking - no live duplicate exists.
- **The auth model is coherent.** `internal/server/auth.go:924` `authMiddleware` distinguishes session auth, Bearer auth and the bypass list; `internal/server/auth.go:1164` `validRouteBearer` fails **closed** for unknown paths (`admin` scope required at `:1203`); `internal/server/auth.go:838` `isAuthBypassed` uses exact paths plus a small subtree list, and the Telnyx webhook bypass is bound to the *mounted* handler rather than live config (`internal/server/auth.go:853-855`). Desktop-scoped paths get granular scopes (`internal/server/desktop_auth.go:133`). I looked hard for a route reachable without auth and found none on the main mux.
- **Request-body limits.** Every JSON entry point I checked is bounded: `internal/server/json_request.go:32-51` (`decodeJSONRequest`, the shared helper, 79 in-edges), `internal/server/flows_handlers.go:318-331` (`flowsDecode` -> `http.MaxBytesReader`), `internal/server/webhook_handlers.go:532` (1 MB), `internal/server/handlers.go:266` (chat completions, 1 MB), `internal/server/desktop_handlers.go:37-45` (`decodeDesktopJSON`). I initially suspected `flowsDecode`, `handlePutOutgoingWebhooks` and `handleChatCompletions` were unbounded and verified each - **all three are limited**.
- **Response-body hygiene.** I checked every `io.ReadAll(resp.Body)` / `io.Copy(w, resp.Body)` site in `internal/server` (`internal/server/onedrive_handlers.go:61,63,156,158,330,332`, `internal/server/oauth_handlers.go:381,429`, `internal/server/radiobrowser_handlers.go:77,91`, `internal/server/three_d_printer_handlers.go:224,245`, `internal/server/config_handlers_providers.go:468,547,560`) - **every one has a matching `defer resp.Body.Close()`**.
- **Path traversal.** `internal/server/fileserver.go` uses an `os.Root`-bound filesystem, refuses symlinks, `Lstat`s before opening and avoids opening FIFOs/devices (which would block). `internal/server/media_handlers.go` validates media IDs and re-checks containment before delete. `internal/webassets/assets.go:63-72` rejects `..` and Windows-reserved names in manifest paths.
- **SSE fan-out and websockets.** `internal/server/sse.go` uses a bounded 128-buffer per client with slow-client eviction and context cancellation - **no unbounded growth**. `internal/server/go2rtc_websocket.go` caps read size, sets deadlines, caps candidates, propagates context and joins its goroutine before returning.
- **Outbound TLS.** `InsecureSkipVerify` appears in 56 places repo-wide, but the one inside `internal/server` (`internal/server/server_internal_http.go:90`) is confined to loopback calls to `InternalAPIURL` (127.0.0.1) and its comment says so. `internal/updater/updater.go:150-162` pins the scheme/host and allow-lists redirect targets. **No remote TLS verification bypass found.**
- **Asset install.** `internal/webassets/install.go` verifies a pinned SHA-256 and exact byte count before extraction, allow-lists tar entries against the manifest, forbids non-regular files and setuid bits, and consumes the gzip trailer.

---

## Metrics

| Metric | Value |
| --- | --- |
| Production `.go` files in scope | 592 (555 in `internal/server`) |
| Production LOC in scope | ~155,900 |
| Test files in `internal/server` | 700 |
| Files read in full or in large spans | ~40 |
| Files swept by regex (`graft grep` + ripgrep) | 555 |
| graft MCP calls | 6 (1 succeeded, 5 timed out at 120 s) |
| graft CLI calls | 13 (`repo_map` x1, `check` x1, `skeleton` x4, `ask` x1, `grep` x6, `callers` x1) |
| GitNexus `impact` / `detect_changes` calls | 0 (read-only audit; no change proposed) |
| Raw `read`/`grep`/shell calls | ~41 |
| **Total tool calls** | **~60** |
| Route registrations enumerated | 787 literal + subtree patterns |
| Duplicate route patterns found (live) | 0 |

**Findings by severity**

| Severity | Count | IDs |
| --- | --- | --- |
| CRITICAL | 0 | - |
| HIGH | 1 | F-1 |
| MEDIUM | 5 | F-2, F-3, F-4, F-5, F-6 |
| LOW | 7 | F-7, F-8, F-9, F-10, F-11, F-12, F-13 |
| **Total** | **13** | |

**Confidence distribution:** Confirmed 10 | Probable 1 (F-5) | Needs-verification 2 (F-3, F-9).

---

## Not covered / follow-ups

### Honest gaps in this pass

1. **The graph was never consulted for blast radius.** I used graft's `callers` for `NewInternalHTTPClient` and ripgrep for the call sites cited, but I did **not** run GitNexus `impact` on any symbol. Any statement in this report about "who calls X" is backed by a literal call-site sweep, not a resolved call graph.
2. **Ten large `internal/server` files were never opened:** `agodesk_handlers.go` (3,497 LOC), `virtual_computers_handlers.go` (2,008), `cyd_creator_builder.go` (1,507), `code_studio_handlers.go` (1,504), `video_studio_handlers.go` (1,480), `agodesk_knowledge_upload.go` (1,295), `desktop_bookshelf_handlers.go` (1,234), `video_studio_service.go` (1,234), `sip_handlers.go` (1,174), `invasion_hatch_handlers.go` (1,167), `realtime_speech_handlers.go` (1,119), `desktop_infra_mutations.go` (1,118), `mailer_oauth.go` (1,081). Given that the files I *did* read produced one HIGH finding, these are the highest-yield next target.
3. **`internal/setup`, `internal/integrationstatus`, `internal/i18n`, `internal/logger`, `internal/uid` were located but not audited in depth** - the setup/bootstrap flow and the Telegram/WhatsApp status probes were not exercised.
4. **No dynamic verification.** No test, build, `go vet`, or `-race` run was performed (read-only constraint). F-1 in particular would be *proven* by a 20-line race test.
5. **`graft grep` on `filepath.Join(...r.URL...)` returned "no hits"** because the pattern was too specific - that sweep is therefore **inconclusive**, not clean. Path-joining was instead audited by reading the file-serving layer directly.

### Recommended next passes

1. **Prove F-1** with a `go test -race` test in `internal/heartbeat` that races `Restart` against `Stop`; then fix the `stopCh`/`WaitGroup` pairing.
2. **Sweep the 41 shallow `*s.Cfg` copies** (`internal/server`, 26 files) for a write into a shared map/slice after the lock is released - that converts F-5 from Probable to a concrete data race.
3. **Second pass over the unreviewed large handler files**, with a focus on: outbound request targets (SSRF), subprocess invocation, and file writes.
4. **Token/secret-response audit**: drive every `jsonError(w, err.Error())` site with a registered secret in the scrubber and assert it is absent from the response - this quantifies F-3's real credential-exposure rate.
5. **Establish a route-table single source of truth.** Today the route table is implicit across ~30 `register*Routes` functions. A generated/checked route manifest would have caught the `requireAdmin` asymmetry in F-12 and would make the "which routes are unauthenticated" question answerable by inspection.

### Scope spillover

- `internal/security` (scrubber, cast-media ticket) and `internal/config` (`Clone`, `Validate*`, `Config` types) are **dependencies** of this area and were read where the server calls into them, but they are assigned to another audit area and were not audited on their own.
- `internal/agent`, `internal/tools`, `internal/remote`, `internal/flow`, `internal/vault` are out of scope; several are reached from server handlers (e.g. `agent.RunSessionRetro` at `internal/commands/commands.go:127`) and were not reviewed.
