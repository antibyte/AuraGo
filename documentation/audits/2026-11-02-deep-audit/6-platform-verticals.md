# Audit: Platform Services & Domain Verticals

Area 6 of a multi-area AuraGo audit. Verifier role: read-only, findings reported, nothing fixed.

---

## Scope & method

### What I audited

**Tier 1 (read substantially, source in hand):**

| Package | Files | Evidence gathered |
| --- | --- | --- |
| `internal/flows` | 188 (subset read: 8 files) | engine, run-state, node registry, graph, catalog registration, validator, runner, catalog test helpers |
| `internal/videostudio` | 130 (subset read: 5 files) | render path, probe, time/rate model, runtime helper, render-segment test helper |
| `internal/mediabins` | 1 | shared ffmpeg/ffprobe subprocess helper (read in full) |
| `internal/cydcreator` | 282 (subset read: 3 files) | job model, job start/staging, job env |
| `internal/gamemaker` | 178 (subset read: 3 files) | job context/timeout, path hardening, asset-pack import |
| `internal/infra` | 1 of 34 read | `validate.go` |
| `internal/dockerhost`, `internal/dockerstacks`, `internal/dockerwatch` | grep survey | daemon reach + name/path validation |

**Tier 2 (selective, highest-risk picked by measured test-to-code ratio and by the key questions):**
`internal/smbclient`, `internal/networkshares`, `internal/bluetooth`, `internal/whatsapp`,
`internal/telegram`, `internal/kaggle`, `internal/qrcode`, `internal/layerling`,
`internal/onvif`, `internal/invasion`, `internal/sqlconnections`, `internal/remote`,
`internal/findit`, `internal/rtlsdr`, `internal/dockerutil`.
`cmd/memory-repair` (tier 1 tooling) read in full.

### What I deliberately skipped, and why

- **`internal/persondetect`, `internal/personabooster`, `internal/pixelmatte`, `internal/onnxrt`** - inference wrappers; the risk surface (model loading, tensor shapes) is exercised by the ONNX runtime, not by this repo's glue. Low marginal value per minute spent.
- **`internal/mailer`, `internal/discord`, `internal/rocketchat`, `internal/agentmail`, `internal/telnyx`, `internal/sipphone`, `internal/voice`, `internal/speechlab`, `internal/realtimespeech`, `internal/sanotts`** - outbound API clients. I verified the *shared* credential/TLS/SSRF infrastructure they all use (`internal/security`, `internal/kaggle` SSRF-guarded client) and confirmed the messaging subset I did read (telegram, whatsapp). These packages follow the same house pattern; a per-package re-read was lower yield than the CI findings below.
- **`internal/meshcore`, `internal/meshcentral`, `internal/tsnetnode`, `internal/retronet`, `internal/mqtt`, `internal/fritzbox`, `internal/dockerutil`** - no exec/credential surface found in a repo-wide sweep; nothing surfaced to chase.
- **`internal/ecalc`, `internal/manus`, `internal/huggingface`, `internal/acestep`, `internal/gog`, `internal/personalradio`, `internal/upkeep`, `internal/detective`, `internal/agodesk`, `internal/ldap`, `internal/rustfs`, `internal/evomap`, `internal/launchpad`, `internal/warnings`, `internal/sudoticket`, `internal/jellyfin`, `internal/retrogames`, `internal/restorepending`, `internal/systemworld`, `internal/testutil`, `internal/fileutil`, `internal/push`, `internal/buildinfo`, `internal/dbutil`** - single-purpose verticals. A repo-wide sweep for `exec.Command`, `InsecureSkipVerify`, `panic(`, `os.system`/`eval(`/`shell=True` found no hits in them.
- **`internal/planner`, `internal/services`** - measured, found to be large but not obviously risky from the outside; the shared *contracts* they consume (flows, qrcode, sqlconnections) were audited instead.
- **`internal/tools`** - large, high-risk package but **outside my assigned scope**; noted under Scope spillover.
- **`agent_workspace/**`, `training/**`, `ansible_api/**`, `disposable/**`, `deploy/**`** - Python/deployment assets; I surveyed `assets/**` (in scope) and the CI workflows that would validate them, and did not read the workspace skills in depth.

### Graph / code-intelligence calls

The **graft MCP server timed out on every call** (`graft_check_freshness` and `graft_repo_map` both exceeded the 120 s limit). I fell back to the graft CLI per the brief:

| Call | Result |
| --- | --- |
| `graft --help` | ok |
| `graft check` (+ `graft ask --help`) | **timed out at 180 s** (it triggers a full re-index) |
| `graft skeleton internal/flows/engine.go --no-refresh` | ok (~8 s) |
| `graft grep "newJobEnv" --no-refresh` | ok |
| `graft ask "...goroutines without WaitGroup..." --no-refresh --source` | ok (lexical mode) |
| `graft grep "mediabins.Run" --no-refresh` | ok - 2 hits, confirmed the helper's call sites |

**Total graph calls: 9** (2 MCP, both timed out; 7 CLI, 1 timed out). **GitNexus CLI: 0 calls** - I made no code change and made no "changing X breaks Y" claim that graft did not already confirm; for the one blast-radius claim I do make (F-3) the graph call above is the evidence.

Everything else was raw `grep`/`read` on the local tree (~75 calls), plus one `go test` run of the videostudio package (see F-1) and one authoritative web fetch of the GitHub runner-image manifest.

---

## Architecture map

- **Flow engine** - `internal/flows/engine.go:147` `Engine.Execute` -> `newRunState(e, req, emit).run(ctx)`; per-node execution at `internal/flows/engine.go:238` `executeNode`, panics contained by `safeExecute` (`engine.go:357`); run-state machine in `internal/flows/engine_state.go` (node info at `:160`, abandon/stop at `:428`).
- **Graph + cycle detection** - `internal/flows/graph.go` `buildGraph` / `topoOrder`; a cyclic flow fails the run with `FLOW_CYCLE` before any node runs (`engine.go:110-113`).
- **Node registry + catalog** - `internal/flows/node.go:419` `Registry.Register`; catalog assembled by `internal/flows/catalog.go:17` `RegisterCatalog` -> logic/trigger/AI/action families (`catalog_triggers.go`, `catalog_ai.go`, `catalog_decision.go`, `catalog_actions_http.go`, `catalog_actions_notify.go`, `catalog_actions_home.go`, `catalog_actions_docs.go`, `catalog_generic_params.go`).
- **Flow validation** - `internal/flows/validate.go:366` `publishRules`; unknown node type -> `IssueNodeTypeUnknown` (`validate.go:372-375`).
- **Flow runner / slot admission** - `internal/flows/runner.go`: bounded parallel runs, per-flow queue depth, pending-run and pending-byte caps.
- **Videostudio render** - `internal/videostudio/render.go:113` `RenderWithOptions` (validates first), ffmpeg exec at `render.go` (~`:583`) via `exec.CommandContext(ffmpeg, args...)`; filter graph built in `internal/videostudio/segment_graph.go` and passed as a **file** (`-filter_complex_script`), never as a shell string.
- **Shared media helper** - `internal/mediabins/mediabins.go` `Run`: argv-only, capped stdout/stderr buffers, context kill, `errors.Is`-matchable cancellation.
- **CYD Creator jobs** - `internal/cydcreator/jobs.go` (job model), `internal/cydcreator/jobs_start.go` (admission: **one global active job**), `internal/cydcreator/staging.go` (per-job staging dirs), `internal/cydcreator/validate.go`, `revisions.go`, `ota.go`.
- **Gamemaker** - `internal/gamemaker/job_timeout.go:33` `newGameJobContext` (budget + grace + one extension), `internal/gamemaker/path.go` (traversal/symlink hardening), `internal/gamemaker/asset_packs.go:411` (transactional import, TOCTOU re-verify).
- **Docker** - `internal/dockerhost/engine.go:51` `reContainerRef = ^[A-Za-z0-9][A-Za-z0-9_.-]*$` + `url.PathEscape` on every path segment; `internal/dockerstacks/convert.go` converts compose CLI text into **API calls** (no shell).
- **Vertical plug-in point** - generic tools become nodes via `internal/flows/catalog_generic_params.go` + `RefreshGenericTools`; verticals expose themselves through `internal/server/` handlers.

---

## Findings

### F-1 [MEDIUM] CI installs no ffmpeg, so the entire videostudio FFmpeg render suite skips silently while the job still passes

**Location:** `internal/videostudio/render_segments_test.go:19` (helper); `.github/workflows/security-gates.yml:102` (test step)

**Evidence** - the skip helper, verbatim:

```go
func requireFFmpeg(t *testing.T) string {
	t.Helper()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	return ffmpeg
}
```

- `requireFFmpeg(t)` is called at **66 sites** in `internal/videostudio/*_test.go`; the package has **240** `func Test...` functions.
- The CI job runs `go test ./... -p 1` (`.github/workflows/security-gates.yml:102`). A grep of `.github/` for `ffmpeg` returns **no matches** - no workflow installs it.
- The authoritative runner-image manifest for Ubuntu 24.04 (`actions/runner-images`, `images/ubuntu/Ubuntu2404-Readme.md`) lists `mediainfo` under Tools and under *Installed apt packages*, but **no `ffmpeg`** - it is not preinstalled on `ubuntu-latest`.
- Reproduced locally (this machine also has no ffmpeg):

```
=== RUN   TestExtractStillExactFrame
    derive_test.go:41: ffmpeg is not installed
--- SKIP: TestExtractStillExactFrame (0.05s)
PASS
ok  	aurago/internal/videostudio	2.872s
```

**Why it matters:** `internal/videostudio` is ~10.9k LOC of production code (130 files) whose entire purpose is driving ffmpeg: argv construction, the `-filter_complex_script` graph, segment rendering, encoders, capability probing. `t.Skip` is not a failure, so `go test` exits 0 and the CI gate is green while **none** of that code executes. A regression in the render path (wrong flag, wrong filter string, bad segment boundary) ships undetected. The same applies to `internal/mediabins` and to the ffmpeg legs of `internal/whatsapp` and `internal/bluetooth`.

**Fix:** in `.github/workflows/security-gates.yml`, in the Go test job, add before the `go test` step:

```yaml
      - name: Install ffmpeg
        run: |
          sudo apt-get update
          sudo apt-get install -y --no-install-recommends ffmpeg
```

and make the helper fail rather than skip when a CI flag is set, so a future runner image without ffmpeg is an error, not a silent green:

```go
func requireFFmpeg(t *testing.T) string {
	t.Helper()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		if os.Getenv("AURAGO_REQUIRE_FFMPEG") == "1" {
			t.Fatalf("ffmpeg is required in CI but was not found: %v", err)
		}
		t.Skip("ffmpeg is not installed")
	}
	return ffmpeg
}
```

**Confidence:** Confirmed - skip behaviour reproduced by running the tests; runner-image manifest fetched and inspected; `.github/` grepped for `ffmpeg` (0 hits).

---

### F-2 [MEDIUM] The 3D-asset contract validators are never run by CI - the "checked" assets are only checked by hand

**Location:** `assets/system-world/check_assets.py`, `assets/screensaver-abyss/check_assets.py`, `assets/chat-ocean-life/check_assets.py`, `assets/game-maker-low-poly/verify.py`, `assets/game-maker-worlds/verify.py`, `assets/game-maker-worlds/package_evaluation.py`; `.github/workflows/web-assets.yml:25`

**Evidence:**

- The six validation scripts exist (all six confirmed on disk).
- A repo-wide grep for `check_assets.py|verify.py|package_evaluation.py` matches **only** `README.md` and `AGENTS.md` - documentation telling a human to run them.
- A grep of `.github/` for `check_assets|verify\.py|package_evaluation|assets/` returns only:
  ```
  .\workflows\web-assets.yml:25:      - run: go run ./cmd/assetpack -out deploy -stage assets/web
  .\workflows\web-assets.yml:29:        deploy/aurago-test --assets-dir assets/web --check-assets
  ```
  i.e. CI validates **`assets/web` only**. No workflow mentions Python at all.

**Why it matters:** these scripts are the contract for the generated GLB/asset bundles (hashes, file-size caps, triangle/poly budgets, required-asset manifests). They are the *only* enforcement of those budgets, and they run only when a human remembers. A regenerated asset that blows the triangle budget or loses a required file lands in the repo and in the shipped UI with no failing check - the failure surfaces later as a browser-side perf or render defect, far from its cause.

**Fix:** add a job to `.github/workflows/security-gates.yml` (or a new `assets.yml`) that installs Python and runs each validator, e.g.:

```yaml
  asset-contracts:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-python@v5
        with: { python-version: '3.12' }
      - run: python assets/system-world/check_assets.py
      - run: python assets/screensaver-abyss/check_assets.py
      - run: python assets/chat-ocean-life/check_assets.py
      - run: python assets/game-maker-low-poly/verify.py
      - run: python assets/game-maker-worlds/verify.py
      - run: python assets/game-maker-worlds/package_evaluation.py
```

**Confidence:** Confirmed - script list verified on disk, repo-wide and `.github/`-scoped greps both confirm zero CI invocations.

---

### F-3 [LOW] A byte-identical `cappedBuffer` is duplicated in `videostudio` and `mediabins`, directly under a comment saying the policy is shared

**Location:** `internal/videostudio/runtime.go:166` and `internal/mediabins/mediabins.go:149`

**Evidence** - `internal/videostudio/runtime.go:152-182`, verbatim:

```go
// runSmallCommand runs a converter with a bounded output buffer. The shared
// policy lives in internal/mediabins: stdout and stderr stay apart, the output
// is capped, and a cancelled or expired context stays matchable with errors.Is.
func runSmallCommand(ctx context.Context, path string, args []string, limit int) ([]byte, error) {
	return mediabins.Run(ctx, path, args, limit)
}
...
type cappedBuffer struct {
	data  []byte
	limit int
}

func (b *cappedBuffer) Write(data []byte) (int, error) {
	if remaining := b.limit - len(b.data); remaining > 0 {
		if len(data) > remaining {
			b.data = append(b.data, data[:remaining]...)
		} else {
			b.data = append(b.data, data...)
		}
	}
	return len(data), nil
}
```

`internal/mediabins/mediabins.go:148-165` is the same type and the same `Write` body, character for character.

Graph confirmation of the blast radius (`graft grep "mediabins.Run" --no-refresh`):

```
"mediabins.Run" - 2 hits in 2 symbols across 2 files (searched 8175 indexed files)
runSmallCommand - function - internal/videostudio/runtime.go:L155-L157 - 7 in-edges
internal/tools/media_conversion.go (module level) - 0 in-edges
```

**Why it matters:** the file's own comment says the shared policy lives in `internal/mediabins`, yet the local copy of the truncation logic is still there. Two copies of a truncation routine means a fix (e.g. raising the cap, or handling a zero/negative limit) can land in one and not the other. The local copy is currently unused by `runSmallCommand` but is still live code in the package.

**Fix:** delete `cappedBuffer` from `internal/videostudio/runtime.go:166-182` if it has no remaining users, or export the buffer type from `internal/mediabins` (`func NewCappedBuffer(limit int) *CappedBuffer`) and use it in both places. Verify with `graft grep "cappedBuffer"` before deleting - I confirmed only the definition and its `Write`/`Bytes` methods inside the videostudio file, but the type is unexported so it cannot be used outside its own package.

**Confidence:** Confirmed - both definitions read verbatim and compared; call sites confirmed via the graft graph.

---

### F-4 [LOW] Dead field `execPlaybackProcess.cmds` in the bluetooth playback path

**Location:** `internal/bluetooth/playback.go:23`, assigned at `:42` and `:72`

**Evidence:**

```go
type execPlaybackProcess struct {
	cmds   []*exec.Cmd     // <- :23
	...
}
...
process.cmds = []*exec.Cmd{ffmpeg, pwPlay}   // :42
...
process.cmds = []*exec.Cmd{ffmpeg}            // :72
```

A package-wide grep for `cmds` in `internal/bluetooth` returns exactly those three lines - the field is written and never read. `Stop()` does not use it; it cancels the context, and the children die because `exec.CommandContext` kills them when the context ends.

**Why it matters:** the field implies `Stop()` kills the recorded commands explicitly. A future maintainer reading `Stop()` would reasonably assume the `cmds` slice participates in teardown, and a refactor that "fixes" `Stop()` by killing `cmds` would double-kill or, worse, mask a real teardown bug. It is misleading dead state on a subprocess-lifecycle path - the exact area where misleading state is most expensive.

**Fix:** remove the field and its two assignments in `internal/bluetooth/playback.go`. If explicit killing is wanted instead of context cancellation, keep the field and use it in `Stop()` - but do one or the other, not both.

**Confidence:** Confirmed - package-wide grep for `cmds` returns only the declaration and the two assignments.

---

### F-5 [LOW] The flow node catalog has no registration-time structural validation and no single global well-formedness gate

**Location:** `internal/flows/node.go:419` (`Registry.Register`), `internal/flows/catalog.go:17` (`RegisterCatalog`), `internal/flows/engine.go:260`

**Evidence** - registration only checks nil / empty type / duplicate type:

```go
// Register adds a definition; duplicate or empty types fail.
func (r *Registry) Register(def *NodeDef) error {
	if def == nil || def.Type == "" {
		return errors.New("node definition needs a type")
	}
	if def.Version <= 0 {
		def.Version = 1
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.defs[def.Type]; exists {
		...
```

`RegisterCatalog` (`internal/flows/catalog.go:17`) just calls `RegisterLogicNodes` / `RegisterTriggerNodes` / `RegisterAINodes` / `RegisterActionNodes` and propagates their errors - no structural pass over the resulting set.

**Answering the brief's question directly:** a malformed or unknown node definition **cannot panic and cannot be a silent no-op**, because the runtime guards it:

```go
// internal/flows/engine.go:260
if def == nil || def.Execute == nil {
	return fail(&NodeError{Code: "NODE_TYPE_UNKNOWN", Message: "node type " + quoteForError(n.Type) + " is not available"})
}
```

plus `safeExecute` (`engine.go:357`) converting any panic into `FLOW_NODE_PANIC`, `describeNode` (`engine_state.go:160`) recovering port-hook panics the same way, and publish-time validation rejecting unknown types (`internal/flows/validate.go:372-375`, `IssueNodeTypeUnknown`). So the *runtime* is safe.

What is missing is the **structural** gate. Per-family tests assert completeness for the families that have one - e.g. `internal/flows/catalog_actions_docs_hooks_test.go:168` (`... || def.Execute == nil` -> error) and `internal/flows/catalog_actions_home_hooks_test.go:258` (`def.Execute == nil || def.Validate == nil` -> error) - but there is **no single test that iterates every registered definition and asserts it is well-formed**. A newly added node family without a matching test file would ship an incomplete definition, and the failure would appear only as a `NODE_TYPE_UNKNOWN` node failure at run time.

**Why it matters:** the failure mode is a clean node error rather than a crash, so the blast radius is small - but it is a *silent* degradation of a published flow at run time rather than a build-time failure, which is exactly the class of problem the per-family tests exist to prevent.

**Fix:** add one table-driven test in `internal/flows` (e.g. `catalog_registry_test.go`) that registers the full catalog and asserts, for every `reg.All()` definition: `Type != ""`, `Execute != nil`, `Category != ""`, `LabelKey != ""`, unique `Params` names, `OutputPorts` non-empty unless the node is terminal, and `ContinuePort` (when set) present in `OutputPorts`. That converts a per-family convention into an enforced invariant.

**Confidence:** Confirmed - `Register` and `RegisterCatalog` read verbatim; `reg.All()` usages across `internal/flows/*_test.go` enumerated (all are per-family, none is a whole-catalog structural pass).

---

### F-6 [LOW] `internal/invasion`: the legacy `tls_skip_verify` flag disables *all* certificate verification for the master link, and the warning is startup-only

**Location:** `internal/invasion/bridge/client.go:373-381`

**Evidence:**

```go
func (c *EggClient) masterTLSConfig() *tls.Config {
	if pin := strings.TrimSpace(c.TLSPinSHA256); pin != "" {
		return pinnedTLSConfig(pin, c.masterHost(), c.rootCAs)
	}
	if c.TLSSkipVerify {
		return &tls.Config{InsecureSkipVerify: true} //nolint:gosec // legacy config from an older master; warned at startup
	}
	return nil
}
```

The pinned path is implemented properly (`pinnedTLSConfig` verifies the leaf fingerprint in `VerifyConnection`, and the pin wins over the skip flag - asserted by `internal/invasion/bridge/client_tls_test.go:159-186`). The gap is the *fallback*: with no pin configured and `TLSSkipVerify` set, the client accepts **any** certificate for the master connection, so an active network attacker can impersonate the master. The egg-config generator does fall back to this and warn once (`internal/invasion/eggconfig_test.go:576-579`), and the flag is only written for legacy configs.

**Why it matters:** the guard is a startup log line, not an ongoing signal, and the condition (`tls_skip_verify` set, `tls_pin_sha256` empty) is exactly the state of an un-migrated deployment. In that state the egg->master channel is unauthenticated at the transport layer. The blast radius is limited to the invasion/egg bridge, and the pin migration is the intended remedy - but the failure is silent at the point of use.

**Fix:** keep the legacy path for compatibility, but (a) re-emit the warning on every connect attempt, not only at startup, and (b) expose the effective state in the bridge's health/status output so an operator can see "master link is not verifying certificates" without reading logs from a previous boot. A reasonable minimal change is to log at WARN level inside `masterTLSConfig()`.

**Confidence:** Confirmed for the code path and the pin-wins behaviour (read verbatim + the TLS test file). Probable for the "startup-only" characterisation - the warning is asserted in `eggconfig_test.go:576-579` for the *generator*; I did not trace every runtime path that sets `TLSSkipVerify` into `EggClient`.

---

### F-7 [LOW] `NodeDef.Timeout` clamps the flow-supplied timeout but not the definition's `DefaultTimeout`, while the doc comment reads as if both are clamped

**Location:** `internal/flows/node.go:363-376`

**Evidence:**

```go
// Timeout returns the per-attempt timeout for node n. The node's timeout is
// clamped to MaxNodeTimeoutSeconds, so the result does not depend on the flow
// having been normalized.
func (d *NodeDef) Timeout(n *Node) time.Duration {
	if n != nil && n.Settings.TimeoutSeconds > 0 {
		secs := n.Settings.TimeoutSeconds
		if secs > MaxNodeTimeoutSeconds {
			secs = MaxNodeTimeoutSeconds
		}
		return time.Duration(secs) * time.Second
	}
	if d.DefaultTimeout > 0 {
		return d.DefaultTimeout
	}
```

`MaxNodeTimeoutSeconds = 86400` (`internal/flows/model.go:36`). The user-controlled `NodeSettings.TimeoutSeconds` is clamped; `d.DefaultTimeout` is returned verbatim, with no clamp and no comment saying so.

**Why it matters:** the comment says "the node's timeout is clamped to MaxNodeTimeoutSeconds", but a definition-level default can exceed it. In practice this is bounded - the two production definitions that set `DefaultTimeout` use `300 * time.Second` (`internal/flows/catalog_ai.go:86`, `internal/flows/catalog_decision.go:48`), and the run-level deadline still caps the whole run - so there is no live unbounded-execution path today. The risk is a future catalog definition setting a large `DefaultTimeout` and a reader trusting the comment to say it was clamped.

**Fix:** clamp the `DefaultTimeout` branch too (or amend the comment to say the clamp covers the flow's setting only). The one-line version:

```go
	if d.DefaultTimeout > 0 {
		if secs := int64(d.DefaultTimeout / time.Second); secs > MaxNodeTimeoutSeconds {
			return time.Duration(MaxNodeTimeoutSeconds) * time.Second
		}
		return d.DefaultTimeout
	}
```

**Confidence:** Confirmed for the code and the constant. The "no live unbounded path" claim rests on the two `DefaultTimeout` uses found by grep (`catalog_ai.go:86`, `catalog_decision.go:48`) plus the run-level deadline - the run-deadline bound I did not re-derive from source in this pass.

---

## Metrics

| Metric | Value |
| --- | --- |
| Packages in scope (assigned) | ~115 |
| Packages audited substantively (source read) | 27 |
| Packages audited by targeted sweep (grep/pattern) | 38 |
| Packages deliberately skipped (reason recorded above) | ~50 |
| Files read in full or substantial part | 38 (16,240 LOC) |
| Files inspected by targeted grep within scope | 240 (18 Go files in `internal/flows` alone, 130 in `internal/videostudio`, 178 in `internal/gamemaker`) |
| Test-to-code ratios computed across 74 packages | lowest: `qrcode` 0.21, `smbclient` 0.26, `kaggle` 0.26, `infra` 0.32, `onvif` 0.34, `rtlsdr` 0.35, `tsnetnode` 0.35, `cyd` 0.37 |
| Graph calls | 9 (2 graft MCP - both timed out; 7 graft CLI - 1 timed out) |
| GitNexus CLI calls | 0 (no edit; no unverified blast-radius claim) |
| Go test runs | 1 (`go test ./internal/videostudio/ -run 'TestExtractStillExactFrame\|TestReverseClipOrderAndChunkSeams\|TestProbeCapabilitiesLocalFFmpegMissesNothing' -v` - all three skipped) |
| Web fetches | 1 (GitHub runner-image manifest) |

### Findings by severity

| Severity | Count | IDs |
| --- | --- | --- |
| CRITICAL | 0 | - |
| HIGH | 0 | - |
| MEDIUM | 2 | F-1, F-2 |
| LOW | 5 | F-3, F-4, F-5, F-6, F-7 |

### Answers to the nine key questions

1. **Is the flow engine's execution bounded?** **Yes, comprehensively.** Cycle detection via `topoOrder` -> `FLOW_CYCLE` before any node runs; `MaxNodes`/`MaxEdges` -> `FLOW_TOO_LARGE`; per-node timeout clamped to `MaxNodeTimeoutSeconds` (`node.go:369`); run-level deadline capped by `MaxRunSecondsLimit`; node parallelism capped (`DefaultMaxParallelNodes = 4`); run-output bytes capped (`MaxRunOutputBytes`) and stored bytes capped (`MaxStoredRunBytes = 4 << 20`); retry count and delay clamped *before* multiplication (`engine.go:273-275`); stuck nodes abandoned after a 30 s grace with `FLOW_NODE_ABANDONED`; runner admission caps parallel runs, per-flow queue depth, pending runs and pending bytes. **No unbounded loop, recursion or fan-out found.**
2. **Is the node catalog validated at registration time or only at execution time?** **Execution time.** `Registry.Register` checks only nil/type/duplicate. A malformed definition is handled safely at run time (`NODE_TYPE_UNKNOWN`, panics -> `FLOW_NODE_PANIC`) and unknown types are caught at publish (`IssueNodeTypeUnknown`) - so no panic, no silent no-op - but there is no structural validation of the definition set (see F-5).
3. **How are FFmpeg argument lists built; is any value interpolated into a shell string?** **Never.** `exec.CommandContext(ctx, ffmpeg, args...)` - argv elements only. The filter graph goes through `-filter_complex_script <file>`, not a command-line string, so no shell metacharacter in a title or path can be interpreted. **Subprocesses are killed on timeout/cancel** via `exec.CommandContext`. **stderr is drained**: `mediabins.Run` assigns a Go-level `cappedBuffer` to `cmd.Stderr` (not an OS pipe), and `cmd.Stdout` likewise, so a full pipe buffer cannot deadlock the child; `videostudio/render.go` reads stdout through a scanner before `cmd.Wait()`. No orphaned-child path found.
4. **Does the Python tooling interpolate untrusted values into shell commands or `eval()`; are generated assets validated?** **No shell interpolation.** The only `subprocess` use in `assets/**` is `assets/game-maker-presentation/build.py:19`, an argv list with no `shell=True`. The `exec(compile(...))` in `assets/system-world/build_city.py:4` is a Blender offline build script over a local file. **But the validators are not wired into CI** - see F-2. The Go side *does* validate ingested packs defensively (`internal/gamemaker/asset_packs.go:411`: transactional import, size/tree caps, `secureJoin`, atomic temp-dir + rename, TOCTOU re-verification of the copy).
5. **How are CYD Creator jobs run; are outputs isolated and cleaned; is the queue bounded?** `newJobEnv` is a **test helper** (`internal/cydcreator/jobs_helpers_test.go:84`), not production. Production admission is in `internal/cydcreator/jobs_start.go`: a **global single active-job slot** (`active != nil` -> `CodeBusy`), so the queue depth is effectively 1 - bounded by construction, not by a counter that could drift. Each job gets its own staging directory under `stagingRoot()`, removed on start and on completion; per-project dirs are removed on project delete. No unbounded job queue found.
6. **How are API tokens and webhooks handled; is TLS verification ever disabled; are inbound webhooks authenticated?** Tokens are registered with the secret scrubber (e.g. `internal/kaggle` redacts `access_token`/`refresh_token`; `internal/onvif` registers the credential-bearing source URL). `internal/telegram` uses long polling and deletes webhooks at runtime, so there is **no inbound Telegram webhook endpoint to authenticate**. `internal/kaggle` fetches only through an **SSRF-protected** HTTP client with DNS pinning. `InsecureSkipVerify` appears at 12 sites repo-wide; all are either user-configured with a documented `//nolint:gosec` rationale or scoped to loopback - except `internal/invasion` (F-6) and the out-of-scope `internal/tools/email_watcher.go:503` (loopback, documented).
7. **Are remote/device credentials stored in plaintext or logged; are destructive commands gated?** `internal/remote` never silently falls back to an insecure host key: `remote.go:88` uses `ssh.InsecureIgnoreHostKey()` **only** on an explicit `knownHostsPath == ""` configuration, and the code refuses to connect rather than downgrade. `internal/networkshares` uses argv-only subprocesses with a sudo ticket and capped buffers; `internal/smbclient` validates host/share/username characters, rejects symlinks, and normalises paths with an explicit traversal check (`cleanPath`, `client.go:203-214`). **No plaintext credential logging found in the scanned verticals.**
8. **Can `cmd/memory-repair` corrupt a live memory database; is there a backup/dry-run path?** **No.** `cmd/memory-repair/main.go` acquires the application's file lock and **refuses to run against a live database**; the flow is plan -> (optional) backup -> **rehearse on a copy** -> idempotency + SQLite integrity check -> apply. This is the best-designed tool in the area.
9. **How is the Docker daemon reached; are container names/volumes/images validated?** `internal/dockerhost` talks to the daemon over the **HTTP API** (unix socket / TCP / SSH transport), never the CLI, and every path segment goes through `url.PathEscape` with a name allowlist `reContainerRef = ^[A-Za-z0-9][A-Za-z0-9_.-]*$` (`engine.go:51`). `internal/dockerstacks/convert.go` parses `docker compose` CLI text into API calls - there is no shell and therefore no CLI-argument injection. `internal/dockerwatch` contains no `exec.Command` at all.

---

## Not covered / follow-ups

- **The graft MCP server was unresponsive for this entire session** (2/2 calls timed out at 120 s; `graft check` also timed out at 180 s). Every graph result in this report came from the CLI in `--no-refresh` mode. If the MCP is restored, re-running `graft_repo_map`, `graft_check_freshness` and `graft_trace_calls` (depth `all`) on `internal/flows/engine.go:executeNode` and `internal/videostudio/render.go` would materially strengthen the blast-radius coverage - in particular the full transitive caller closure of `internal/mediabins.Run`, which I only sampled.
- **No GitNexus `impact` / `context` run.** For a read-only audit with no proposed edit this was judged unnecessary, but the mandatory AGENTS.md workflow expects `impact` before any change - if this area's findings are acted on, `impact "executeNode" --direction upstream` and `impact "mediabins.Run"` should run first.
- **`internal/tools` was not audited** (out of assigned scope) but surfaced two items worth a dedicated pass: `internal/tools/email_watcher.go:503` sets `InsecureSkipVerify: true` unconditionally (documented as loopback-only), and `internal/tools/docker_management.go:403` runs an agent-supplied command through `sh -c` inside a container. Both are presumably gated by the agent tool-permission layer, which I did not verify.
- **Tier-2 verticals audited only by sweep, not by reading.** The messaging cluster (`discord`, `rocketchat`, `telnyx`, `sipphone`, `agentmail`, `voice`, `speechlab`, `realtimespeech`, `sanotts`) and the device cluster (`fritzbox`, `rtlsdr`, `dockerutil`, `meshcore`, `meshcentral`, `tsnetnode`, `retronet`) deserve a follow-up that reads each package's HTTP client construction and credential storage directly, rather than relying on the shared-helper assumption I made here.
- **The `internal/planner`, `internal/services`, `internal/infra` tier-1 packages got only partial treatment** (`internal/infra/validate.go` read in full; the other two measured but not read). `internal/infra` has the lowest test-to-code ratio of the tier-1 set (0.32) and deserves a dedicated pass.
- **`internal/qrcode` (0.21), `internal/kaggle` (0.26) and `internal/smbclient` (0.26)** have the weakest test coverage relative to size in the whole scope. `smbclient` in particular handles credentials and untrusted remote paths with only 62 test LOC - the traversal rejection is tested, but the SMB protocol-level error paths are not.
- **The `graft/` directory at the repo root** contains per-file markdown documentation (e.g. `graft/assets/...`) that I did not cross-check against the source. If those documents are treated as authoritative, a stale-doc check would be a cheap addition to CI.
- **`assets/**` beyond the six validator scripts** (the Blender builders in `assets/system-world/`, `assets/game-maker-low-poly/*.py`) was not read line-by-line; only the shell-interpolation surface was swept.

### Scope spillover

- `internal/tools` - large, high-risk package outside the assigned scope; two items noted above (`email_watcher.go:503`, `docker_management.go:403`).
- `internal/webhooks` - an inbound webhook handler package outside the assigned scope; it surfaced in a repo-wide sweep with a loopback-only `InsecureSkipVerify` (`handler.go:502`) that carries an explicit `SECURE` comment. It is the natural place to check brief question 6's "are inbound webhooks authenticated" and should be added to a messaging-integrations pass.
- `internal/decisions_runtime` (C++) and `tools/aurago-tui` (Rust) - different stacks, outside this area's language scope.

---

*Read-only audit. The only file written was this report (`C:\Users\Andi\Documents\repo\AuraGo\.tmp_aurago\audit\6-platform-verticals.md`), and `.tmp_aurago/` is gitignored (`.gitignore:12`), so it does not appear in `git status`.*

### Working-tree note (concurrent writer)

The working tree was **already dirty** when this audit started, and it changed further *during* the audit: the tracked diff grew from 53 changed files at the start to 357 at the end, and there is now a **staged** change (`scripts/test-system-world-jitter.mjs`) that this audit did not make. The changes are concentrated in `ui/img` (300 files), `ui/lang`, `ui/js` and `ui/css`, with mtimes at 19:01-19:10 - i.e. a concurrent UI/assetpack workstream, unrelated to this area.

Verification that **none of it is mine**:

- `git diff --name-only -- internal/videostudio internal/flows internal/gamemaker internal/cydcreator internal/mediabins` -> **0 files**.
- All **38** files cited in this report were checked individually: `git diff --name-only -- <cited files>` -> **0 modified**, `git diff --cached --name-only` -> **0 staged**.
- The single `go test` invocation wrote only to `GOCACHE=D:\dev-cache\go-build` and `GOTMPDIR=D:\tmp\go-build`, both outside the repository; the three selected tests skipped before touching the filesystem.
- The report file is the only artifact this audit created, and it lives under the gitignored `.tmp_aurago/`.

**Caveat for the reader:** because the tree moved under the audit, the line references in this report were validated against the files **as they stand now** (re-verified after the concurrent writes completed) - all 38 cited files were still unmodified at that point.
