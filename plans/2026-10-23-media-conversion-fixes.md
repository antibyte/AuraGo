# Media conversion — fixes and optimizations plan

Status: **executed** on 2026-10-23. All nine tasks are implemented and committed; see
"Execution record" at the end for the commit per task. The result was reviewed afterwards and the
seven review findings are fixed as well — see "Review record" at the end.
Source baseline: `reports/media-conversion-audit-2026-10-23.md` (findings A–L), inspected on 2026-10-23.

> **For agentic workers:** execute the tasks in wave order. Every task follows the same
> shape: write the failing test first, watch it fail, implement, watch it pass, commit.
> Do not skip the "Confirm callers" step — the `media_conversion` tool is reachable from the
> agent dispatcher, the MCP bridge, the native-tool registry and the config UI, and the
> `ExecuteMediaConversion` signature changes in MC5/MC7.

## Goal

Close all twelve audit findings (A–L) in the media-conversion integration without changing
the parts that already work: the staging design, the workspace path confinement, the
pseudo-protocol blocklist, the atomic write-back, the read-only mode and the Danger-Zone gate.

After this plan the integration must:

1. resolve the conversion target **once** and derive the staged file name from the resolved
   format, never from the user-supplied file name (A, F, G);
2. never let ffmpeg reach a remote protocol, and never let it read a format it has no business
   reading (C);
3. resolve converter binaries with the same rules the video studio already uses, including the
   Windows `convert.exe` trap (B, D);
4. honour the caller context so an aborted agent turn stops the converter (E, D);
5. validate `tools.media_conversion` server-side and expose every field the config struct owns
   in the UI (H);
6. restrict the health endpoint to approved converter binaries and admin callers (I);
7. report output metadata and register the converted file in the media registry (J, K);
8. carry a test suite that actually exercises every branch it claims to (L).

## Scope

In scope: `internal/tools/media_conversion.go`, `internal/server/media_conversion_handlers.go`,
the agent dispatch path for the `media_conversion` tool, `internal/config` validation for
`tools.media_conversion`, `ui/cfg/media_conversion.js` and its locale files, the tool manual in
`prompts/tools_manuals/media_conversion.md`, and a new `internal/mediabins` package.

Out of scope: the video studio's own rendering pipeline (only a mechanical extraction of two
helpers is allowed, see MC9); ffmpeg/ImageMagick installation guidance; a browser-side preview
of converted media; re-encoding presets beyond what the tool schema already exposes.

## Existing integration points

| Concern | Current source and behavior |
| --- | --- |
| Tool schema | `internal/agent/native_tools_content.go:171` — `media_conversion` with `operation`, `file_path`, `output_file`, `output_format`, `quality_pct`, `width`, `height`. |
| Argument decoding | `internal/agent/tool_args_execution.go:812` — `decodeMediaConversionArgs`, resolves the `path`/`dest`/`format` aliases. |
| Dispatch | `internal/agent/agent_dispatch_exec.go:1273` → `internal/agent/dispatch_filesystem.go:42`; Danger-Zone gate at `dispatch_filesystem.go:45`. |
| Tool entry point | `internal/tools/media_conversion.go:105` — `ExecuteMediaConversion(workspaceDir, cfg, req)`. |
| Staging | `internal/tools/media_conversion.go:138-177` — private temp dir, 0600 copies, `secureResolve` confinement. |
| Converters | `media_conversion.go:246` (ffmpeg), `:330` (ImageMagick), `:391`/`:432` (info), `:499`/`:519` (health). |
| Output resolution | `media_conversion.go:593` `resolveConversionOutput` — called **twice** (`:163` and inside both executors), which is the root cause of A. |
| Binary resolution | `media_conversion.go:539` `resolveConfiguredBinary`, `:560` `resolveConfiguredImageMagick`, `:568` `resolveFFprobeBinary`. |
| HTTP surface | `internal/server/media_conversion_handlers.go:14` — `POST /api/media-conversion/test`; route registered at `internal/server/server_routes_config.go:273`. |
| Admin protection | `internal/server/auth.go:453` `requireAdmin`; `internal/server/auth.go:495` `isAdminProtectedPath`. |
| Config | `internal/config/config_types.go:260` `MediaConversionConfig`; defaults `internal/config/config.go:474-476`; patch merge `internal/server/config_patch.go:66`. |
| UI | `ui/cfg/media_conversion.js:36-78`; locales `ui/lang/config/media_conversion/*.json` (18 files, 22 keys each). |
| Model-facing docs | `prompts/tools_manuals/media_conversion.md` — must document the new format allowlist. |
| Reusable reference | `internal/videostudio/runtime.go:110` `resolveBinary`, `:168` `runSmallCommand`; `internal/videostudio/probe.go:15` `allowedProtocols` / `allowedDemuxers`. |

## Design decisions

**DD1 — One resolved target, computed once.** Introduce `conversionTarget{Destination, StagedPath,
Format}` produced by a single `resolveConversionTarget`. The executors receive that struct instead
of re-deriving paths. This removes the duplicated `resolveConversionOutput` call, which is what
lets the reported format and the produced file diverge.

**DD2 — The staged file name derives from the format, not from the destination.** `StagedPath` is
always `<staging>/output.<ext(format)>`. The destination file name only decides *where* the result
lands in the workspace, never *what* the converter writes.

**DD3 — Formats are allowlisted per media type; conflicts are errors.** `audio`, `video` and
`image` each get an explicit list. `output_file: "out.mp4"` combined with `output_format: "mp3"`
fails with a message naming both values instead of silently producing an mp4 while reporting mp3.
A destination without an extension gets the format's extension appended.

**DD4 — Protocol whitelist always, format whitelist only when provable.** `-protocol_whitelist
file,pipe` is unconditional: it is the control that actually stops SSRF. `-format_whitelist` is
added only when the input extension maps to a known demuxer, so exotic-but-legitimate inputs keep
working.

**DD5 — A shared `internal/mediabins` package, but a stable test seam in `internal/tools`.**
`mediabins.Resolve` requires a regular file and refuses the Windows `convert` trap;
`mediabins.Run` bounds the output and reports context errors. `internal/tools` keeps its
package-level `mediaConversionLookPath` / `mediaConversionRunCommand` variables as thin
delegates, so the existing test doubles continue to work unchanged.

**DD6 — `convert` is never an ImageMagick candidate on Windows.** `C:\Windows\System32\convert.exe`
is the FAT converter and sits in the default PATH.

**DD7 — The caller context is the parent of the conversion deadline.** `context.Background()` is
replaced by the dispatcher context; the per-conversion timeout is derived from it with
`context.WithTimeout`.

**DD8 — The health endpoint executes allowlisted converter names only, for admin callers.**
A requested path must be an existing regular file whose base name is `ffmpeg`, `ffprobe`,
`magick` (plus `convert` on non-Windows). Anything else is rejected before execution.

**DD9 — Server-side validation mirrors the UI clamp.** The UI already clamps the timeout to
5–1800 s; the API must not accept 999999 or a negative size cap.

**DD10 — The result carries output metadata and is registered.** The staged output is probed
once; the result reports its size, duration and dimensions, and the file is registered in the
media registry so `send_audio` / `send_video` and the media UI can find it.

### Behavior changes to announce

These are intentional and must be reflected in `prompts/tools_manuals/media_conversion.md`:

- formats outside the allowlist are rejected with a message that lists the supported ones;
- a destination extension that contradicts `output_format` is an error, not a silent override;
- `audio_convert` now drops the video stream;
- a conversion that exceeds the timeout reports a timeout, not a generic process error.

## Task overview

| Task | Findings | Area | Risk |
| --- | --- | --- | --- |
| MC1 | A, F, G | `internal/tools` | medium — changes the resolved target for every conversion |
| MC2 | C | `internal/tools` | low — adds ffmpeg flags |
| MC3 | B, D | new `internal/mediabins` | medium — new shared package |
| MC4 | H | `internal/config`, `internal/server`, `ui` | low |
| MC5 | E, D | `internal/tools`, `internal/agent` | medium — signature change |
| MC6 | I | `internal/server` | low |
| MC7 | J, K | `internal/tools`, `internal/agent` | medium — touches the registry |
| MC8 | L | tests | low |
| MC9 | B, D (debt) | `internal/videostudio` | medium — optional, separate PR |

Wave 1 = MC1–MC4 (correctness and security), wave 2 = MC5–MC6 (lifecycle and hardening),
wave 3 = MC7–MC8 (observability and tests). MC9 is optional and may ship separately.

---

## Wave 1 — correctness and security

### Task MC1: single conversion-target resolution

Fixes A, F, G.

**Files:**
- Modify: `internal/tools/media_conversion.go:593` (`resolveConversionOutput` — replaced),
  `:634` (`normalizedOutputFormat`), `:646` (`preferredExtension`), `:105` (`ExecuteMediaConversion`),
  `:163-177` (staging block), `:246` and `:330` (both executors)
- Test: `internal/tools/media_conversion_test.go`

**Confirm callers before editing:**

```bash
node .gitnexus/run.cjs impact "ExecuteMediaConversion" --direction upstream --repo .
grep -rn "resolveConversionOutput\|executeFFmpegConversion\|executeImageMagickConversion" internal/
```

Expected: `ExecuteMediaConversion` is called from `internal/agent/dispatch_filesystem.go` and from
`internal/tools/media_conversion_test.go`. Note that the graph did **not** resolve the
package-qualified call `tools.ExecuteMediaConversion` — verify by text search, not by the graph.

**Steps:**

- [ ] **Step 1 — failing test.** Add to `internal/tools/media_conversion_test.go`:

  ```go
  func TestResolveConversionTargetDerivesStagedExtensionFromFormat(t *testing.T) {
      staging := t.TempDir()
      target, err := resolveConversionTarget(staging, "/ws/input.wav", "out", "mp3", "audio")
      if err != nil { t.Fatalf("resolveConversionTarget: %v", err) }
      if filepath.Ext(target.StagedPath) != ".mp3" {
          t.Fatalf("staged path = %q, want .mp3 suffix", target.StagedPath)
      }
      if filepath.Base(target.Destination) != "out.mp3" {
          t.Fatalf("destination = %q, want out.mp3", target.Destination)
      }
  }

  func TestResolveConversionTargetRejectsFormatConflict(t *testing.T) {
      _, err := resolveConversionTarget(t.TempDir(), "/ws/in.mp4", "out.mp4", "mp3", "audio")
      if err == nil { t.Fatal("expected a conflict error") }
      if !strings.Contains(err.Error(), "out.mp4") || !strings.Contains(err.Error(), "mp3") {
          t.Fatalf("error must name both values, got %q", err)
      }
  }

  func TestResolveConversionTargetRejectsUnsupportedFormat(t *testing.T) {
      _, err := resolveConversionTarget(t.TempDir(), "/ws/in.wav", "", "txt", "audio")
      if err == nil { t.Fatal("expected an allowlist error") }
      if !strings.Contains(err.Error(), "mp3") { t.Fatalf("error must list supported formats, got %q", err) }
  }
  ```

- [ ] **Step 2 — verify it fails.**

  ```bash
  go test ./internal/tools/ -run TestResolveConversionTarget -v
  ```

  Expected: compile error, `resolveConversionTarget` undefined.

- [ ] **Step 3 — implement.** In `internal/tools/media_conversion.go` add:

  ```go
  // conversionTarget is the single resolved description of one conversion.
  type conversionTarget struct {
      Destination string // absolute path inside the workspace
      StagedPath  string // private staging file the converter writes
      Format      string // normalized target format
  }

  var conversionFormatAllowlist = map[string][]string{
      "audio": {"mp3", "wav", "aac", "m4a", "ogg", "flac", "opus", "aiff", "wma"},
      "video": {"mp4", "webm", "mkv", "mov", "avi", "m4v", "mpeg", "ts"},
      "image": {"png", "jpg", "jpeg", "webp", "gif", "tiff", "tif", "bmp"},
  }

  func mediaTypeForOperation(op string) string {
      switch op {
      case "audio_convert": return "audio"
      case "video_convert": return "video"
      case "image_convert": return "image"
      }
      return ""
  }
  ```

  `resolveConversionTarget(stagingDir, inputFile, outputFile, outputFormat, mediaType string)
  (conversionTarget, error)` applies, in order:

  1. `format := normalizedOutputFormat(outputFormat)`;
  2. when `outputFile` is set, `ext := normalizedOutputFormat(strings.TrimPrefix(filepath.Ext(outputFile), "."))`;
     if `format` is empty it is taken from `ext`; if both are set and differ → error naming both;
  3. when neither `outputFile` nor `outputFormat` is set → error "output_file or output_format is required";
  4. allowlist check against `conversionFormatAllowlist[mediaType]` (skipped when `mediaType` is empty);
  5. `destination`: `outputFile` as given, otherwise `strings.TrimSuffix(inputFile, filepath.Ext(inputFile)) + "." + preferredExtension(format)`;
  6. `validateMediaConversionOutputFile(destination)` — keep the existing pseudo-protocol blocklist;
  7. in-place guard: `destination` must differ from `inputFile`;
  8. `StagedPath = filepath.Join(stagingDir, "output."+preferredExtension(format))`.

  Then in `ExecuteMediaConversion` replace the `:163-177` block with a single
  `resolveConversionTarget(stagingDir, req.FilePath, req.OutputFile, req.OutputFormat,
  mediaTypeForOperation(req.Operation))` call, set `req.OutputFile = target.StagedPath`, and copy
  `target.StagedPath` back to `target.Destination` after the conversion. Remove the
  `resolveConversionOutput` call inside `executeFFmpegConversion` and
  `executeImageMagickConversion`; give both executors the resolved `conversionTarget` instead.

- [ ] **Step 4 — drop the video stream for audio conversions.** In the audio branch of the ffmpeg
  argument builder add `-vn` before the output path.

- [ ] **Step 5 — verify it passes.**

  ```bash
  go test ./internal/tools/ -run "TestResolveConversionTarget|TestExecuteMediaConversion" -v
  go test ./internal/tools/
  ```

- [ ] **Step 6 — document the allowlist.** Update `prompts/tools_manuals/media_conversion.md` with
  the supported format list per operation and the conflict rule, so the model stops guessing
  formats outside the list.

- [ ] **Step 7 — commit.**

  ```bash
  git add internal/tools/media_conversion.go internal/tools/media_conversion_test.go prompts/tools_manuals/media_conversion.md
  git commit -m "fix(tools): resolve media conversion target once and allowlist output formats" -m "Audit findings A, F, G. The staged output name now derives from the resolved format instead of the destination file name, so an output_file without an extension no longer produces an extension-less converter input. Format allowlists per media type, an explicit conflict error, and -vn for audio conversions."
  ```

**Acceptance:** `output_file: "out"` + `output_format: "mp3"` writes `out.mp3`; a conflicting
extension errors; `txt` is rejected for audio; `audio_convert` from a video source produces a
file without a video stream; every pre-existing `internal/tools` test still passes.

---

### Task MC2: ffmpeg protocol and format hardening

Fixes C.

**Files:**
- Modify: `internal/tools/media_conversion.go:268` (ffmpeg args), `:432-445` (ffprobe args)
- Test: `internal/tools/media_conversion_test.go`

**Steps:**

- [ ] **Step 1 — failing test.** Capture the ffmpeg argument vector through the existing
  `mediaConversionRunCommand` seam and assert the whitelist flags:

  ```go
  func TestExecuteMediaConversionAddsProtocolWhitelist(t *testing.T) {
      // arrange a workspace input, stub mediaConversionLookPath/mediaConversionRunCommand
      // assert the captured args contain "-protocol_whitelist" with "file,pipe"
      // assert the captured args contain "-nostdin" and "-loglevel error"
  }
  ```

- [ ] **Step 2 — verify it fails.**

  ```bash
  go test ./internal/tools/ -run TestExecuteMediaConversionAddsProtocolWhitelist -v
  ```

- [ ] **Step 3 — implement.** Add to `media_conversion.go`:

  ```go
  const mediaConversionProtocols = "file,pipe"

  // mediaConversionDemuxers maps a known input extension to its ffmpeg demuxer.
  var mediaConversionDemuxers = map[string]string{
      "mp3": "mp3", "wav": "wav", "aac": "aac", "m4a": "mov", "m4b": "mov",
      "ogg": "ogg", "oga": "ogg", "opus": "ogg", "flac": "flac", "aiff": "aiff",
      "aif": "aiff", "wma": "asf", "mp4": "mov", "m4v": "mov", "mov": "mov",
      "mkv": "matroska", "webm": "matroska,webm", "avi": "avi", "flv": "flv",
      "mpeg": "mpeg", "mpg": "mpeg", "ts": "mpegts", "mxf": "mxf", "3gp": "3gp",
      "png": "png_pipe,image2", "jpg": "jpeg_pipe,image2", "jpeg": "jpeg_pipe,image2",
      "webp": "webp_pipe,image2", "gif": "gif", "tiff": "tiff_pipe,image2",
      "tif": "tiff_pipe,image2", "bmp": "bmp_pipe,image2",
  }
  ```

  Build the ffmpeg arguments as:

  ```go
  args := []string{"-hide_banner", "-nostdin", "-loglevel", "error",
      "-protocol_whitelist", mediaConversionProtocols}
  if demuxer := mediaConversionDemuxers[strings.ToLower(strings.TrimPrefix(filepath.Ext(req.FilePath), "."))]; demuxer != "" {
      args = append(args, "-format_whitelist", demuxer)
  }
  args = append(args, "-y", "-i", req.FilePath)
  ```

  Apply the same `-protocol_whitelist` (and the same optional `-format_whitelist`) to the ffprobe
  argument vector in `ffprobeInfoResult`.

- [ ] **Step 4 — verify it passes.**

  ```bash
  go test ./internal/tools/ -run "TestExecuteMediaConversion|TestMediaConversion" -v
  go test ./internal/tools/
  ```

- [ ] **Step 5 — commit.**

  ```bash
  git commit -m "fix(tools): restrict ffmpeg protocols for media conversion" -m "Audit finding C. ffmpeg and ffprobe now run with -nostdin and -protocol_whitelist file,pipe; -format_whitelist is added only for input extensions with a known demuxer so exotic inputs keep working."
  ```

**Acceptance:** no converter invocation can reach a remote protocol; a conversion of a local
`.wav`/`.mp4` still succeeds; an input extension outside the demuxer map still converts, without
a format whitelist.

---

### Task MC3: shared media-binary runtime

Fixes B, D.

**Files:**
- Create: `internal/mediabins/mediabins.go`, `internal/mediabins/mediabins_test.go`
- Modify: `internal/tools/media_conversion.go:30` (`mediaConversionRunCommand`),
  `:499` (`inspectMediaBinary`), `:519` (`inspectImageMagickBinary`),
  `:539` (`resolveConfiguredBinary`), `:560` (`resolveConfiguredImageMagick`),
  `:568` (`resolveFFprobeBinary`)
- Test: `internal/tools/media_conversion_test.go`

**Steps:**

- [ ] **Step 1 — failing test in the new package.**

  ```go
  func TestResolveRejectsWindowsSystemConvert(t *testing.T) {
      // a directory named "convert.exe" in a temp dir, temporarily prepended to PATH
      // ResolveImageMagick must not return it on Windows
  }

  func TestResolveRejectsNonRegularFile(t *testing.T) {
      // a directory must be rejected, not returned as an executable
  }

  func TestRunReportsDeadlineExceeded(t *testing.T) {
      // a command that sleeps longer than the deadline must surface context.DeadlineExceeded
  }
  ```

- [ ] **Step 2 — verify it fails.** `go test ./internal/mediabins/ -v` → package does not exist yet.

- [ ] **Step 3 — implement `internal/mediabins`.**

  ```go
  // Package mediabins resolves and runs external media converters.
  package mediabins

  // LookPath is the exported seam so callers can stub discovery in tests.
  var LookPath = exec.LookPath

  // Resolve returns an executable path for name, preferring an explicit configured path.
  func Resolve(configuredPath, name string) (string, error)

  // ResolveImageMagick resolves ImageMagick. On Windows the bare name "convert" is never a
  // candidate: C:\Windows\System32\convert.exe is the FAT converter, not ImageMagick.
  func ResolveImageMagick(configuredPath string) (string, error)

  // Run executes path with args, collecting at most limit bytes of stdout and stderr.
  // The returned error satisfies errors.Is(err, context.DeadlineExceeded) when the context
  // deadline fired, and prefers stderr over stdout in the message.
  func Run(ctx context.Context, path string, args []string, limit int) ([]byte, error)
  ```

  `Resolve` rules: an explicit configured path that contains a separator or is absolute must
  exist **and be a regular file** (`info.Mode().IsRegular()`); a bare configured name is looked up
  with `LookPath`; otherwise `name` is looked up, plus `name+".exe"` on Windows.

  `Run` uses capped buffers (see `internal/videostudio/runtime.go:168` for the proven pattern),
  sets `cmd.Stdin = nil`, and classifies the context error before the process error.

- [ ] **Step 4 — wire `internal/tools` to the new package, keeping the test seam.**

  ```go
  var mediaConversionLookPath = mediabins.LookPath
  var mediaConversionRunCommand = mediabins.Run
  ```

  Keep the variable names so the existing tests keep compiling and their doubles keep working.
  Replace the bodies of `resolveConfiguredBinary`, `resolveConfiguredImageMagick` and
  `resolveFFprobeBinary` with calls into `mediabins`; delete the duplicated `exec.LookPath`
  handling.

- [ ] **Step 5 — verify it passes.**

  ```bash
  go test ./internal/mediabins/ -v
  go test ./internal/tools/ -run "TestExecuteMediaConversion" -v
  go test ./internal/tools/
  ```

- [ ] **Step 6 — commit.**

  ```bash
  git add internal/mediabins internal/tools/media_conversion.go internal/tools/media_conversion_test.go
  git commit -m "feat(tools): add shared mediabins runtime for converter discovery" -m "Audit findings B, D. Binary discovery now requires a regular file and never selects the Windows system convert.exe as ImageMagick; command output is bounded and deadline errors are distinguishable."
  ```

**Acceptance:** a Windows PATH containing `C:\Windows\System32` does not yield
`convert.exe` as ImageMagick; a configured path pointing at a directory is rejected; a timed-out
conversion reports a deadline error; all existing `internal/tools` tests pass unchanged.

---

### Task MC4: server-side config validation and the missing UI field

Fixes H.

**Files:**
- Create: `internal/config/media_conversion.go`, `internal/config/media_conversion_test.go`
- Modify: `internal/config/config.go` (validator call next to `ValidateLocalMusicConfig` at `:1239`),
  `internal/server/config_patch.go:74` (validator call after the patch unmarshal),
  `internal/config/config_types.go:260` (doc comments),
  `ui/cfg/media_conversion.js:36-78` (add the `max_file_size_mb` field),
  `ui/lang/config/media_conversion/*.json` (18 locales — new label and help keys)
- Test: `internal/server/config_handlers_test.go`

**Steps:**

- [ ] **Step 1 — failing test.**

  ```go
  func TestValidateMediaConversionConfig(t *testing.T) {
      cases := []struct{ name string; cfg MediaConversionConfig; wantErr string }{
          {"timeout too large", MediaConversionConfig{TimeoutSeconds: 999999}, "timeout_seconds"},
          {"timeout too small", MediaConversionConfig{TimeoutSeconds: 1}, "timeout_seconds"},
          {"size cap negative", MediaConversionConfig{MaxFileSizeMB: -1}, "max_file_size_mb"},
          {"path with newline", MediaConversionConfig{FFmpegPath: "C:\\ffmpeg\n.exe"}, "ffmpeg_path"},
          {"defaults", MediaConversionConfig{}, ""},
      }
      // ...
  }
  ```

- [ ] **Step 2 — verify it fails.** `go test ./internal/config/ -run TestValidateMediaConversionConfig -v`.

- [ ] **Step 3 — implement.**

  ```go
  // ValidateMediaConversionConfig rejects values that the tool cannot honor safely.
  // TimeoutSeconds 0 means "use the default"; otherwise 5..1800 seconds.
  // MaxFileSizeMB 0 means "unlimited"; otherwise 1..102400 megabytes.
  func ValidateMediaConversionConfig(cfg *MediaConversionConfig) error
  ```

  Also reject a binary path longer than 1024 characters or containing control characters.

  Call it from `config.Load` next to `ValidateLocalMusicConfig` and from `applyConfigPatch` after
  the patch is unmarshalled into the config tree, so a direct API write cannot bypass it.

- [ ] **Step 4 — add the UI field.** In `ui/cfg/media_conversion.js` add a number input bound to
  `tools.media_conversion.max_file_size_mb` with `min="0"` and `max="102400"`, a label key
  `config.media_conversion.max_file_size_mb_label` and a help key
  `help.media_conversion.max_file_size_mb`, where 0 means unlimited. Follow the exact
  `data-path` / `t()` pattern already used by the neighbouring fields in the same file.

- [ ] **Step 5 — translate the new copy.** Add both keys to every file under
  `ui/lang/config/media_conversion/` (18 locales). Do not leave a locale without the key — the
  UI contract requires complete translations across the locale families.

- [ ] **Step 6 — verify it passes.**

  ```bash
  go test ./internal/config/ -run TestValidateMediaConversionConfig -v
  go test ./internal/server/ -run "TestConfig" -v
  go run scripts/validate_i18n_json.go
  ```

- [ ] **Step 7 — commit.**

  ```bash
  git commit -m "feat(config): validate media conversion settings and expose the size cap" -m "Audit finding H. tools.media_conversion is validated server-side with the same ranges the UI enforces, and max_file_size_mb is no longer YAML-only."
  ```

**Acceptance:** an API write of `timeout_seconds: 999999` is rejected with a message naming the
field; the UI shows a working size-cap field; every locale carries the new keys; the saved
config still round-trips.

---

## Wave 2 — lifecycle and hardening

### Task MC5: propagate the caller context and report timeouts

Fixes E, D.

**Files:**
- Modify: `internal/tools/media_conversion.go:105` (`ExecuteMediaConversion` signature),
  `:246`, `:330`, `:432` (executors), `:138-177` (staging copy)
- Modify: `internal/agent/dispatch_filesystem.go:49` (pass the dispatcher context)
- Test: `internal/tools/media_conversion_test.go`, `internal/agent/` dispatch tests

**Steps:**

- [ ] **Step 1 — failing test.**

  ```go
  func TestExecuteMediaConversionStopsWhenContextIsCancelled(t *testing.T) {
      // cancel the context before the call and assert the tool returns an error payload
      // and that no converter process was started
  }

  func TestExecuteMediaConversionReportsTimeout(t *testing.T) {
      // stub the runner so it returns context.DeadlineExceeded
      // assert the message contains "timed out" and the configured duration
  }
  ```

- [ ] **Step 2 — verify it fails.**

- [ ] **Step 3 — implement.**

  ```go
  func ExecuteMediaConversion(ctx context.Context, workspaceDir string, cfg *config.MediaConversionConfig, req MediaConversionRequest) string
  ```

  Replace the three `context.WithTimeout(context.Background(), …)` sites with
  `context.WithTimeout(ctx, mediaConversionTimeout(cfg))`. In `dispatch_filesystem.go` pass the
  dispatcher `ctx` (it is already in scope at `:11`).

  Make the staging copy interruptible: replace the plain `io.Copy` with a context-aware reader
  so a cancelled turn stops a multi-hundred-megabyte copy.

  Classify the error before the generic process error:

  ```go
  if errors.Is(ctx.Err(), context.DeadlineExceeded) {
      return mediaConversionJSON(mediaConversionResult{
          Status: "error", Operation: req.Operation, Tool: tool,
          Message: fmt.Sprintf("conversion timed out after %s", mediaConversionTimeout(cfg)),
      })
  }
  ```

- [ ] **Step 4 — update the callers and the test file.** The existing tests call
  `ExecuteMediaConversion(workspaceDir, cfg, req)`; add `context.Background()` as the first
  argument.

- [ ] **Step 5 — verify it passes.**

  ```bash
  go test ./internal/tools/ -v
  go test ./internal/agent/ -run "TestDispatch" -v
  ```

- [ ] **Step 6 — commit.**

  ```bash
  git commit -m "fix(tools): honor the caller context in media conversion" -m "Audit findings E, D. The conversion deadline now derives from the agent dispatcher context, the staging copy is interruptible, and a timeout is reported as a timeout instead of a generic process error."
  ```

**Acceptance:** cancelling the agent turn stops the converter; a timeout produces a message that
names the duration; no `context.Background()` remains in `media_conversion.go`.

---

### Task MC6: restrict the health endpoint

Fixes I.

**Files:**
- Modify: `internal/server/server_routes_config.go:273` (wrap the route),
  `internal/server/media_conversion_handlers.go:47-58` (validate the requested path),
  `internal/server/auth.go:495-542` (`isAdminProtectedPath` — add the route for an explicit
  admin scope)
- Create: `internal/server/media_conversion_handlers_test.go`

**Steps:**

- [ ] **Step 1 — failing test.**

  ```go
  func TestHandleMediaConversionTestRejectsForeignBinary(t *testing.T) {
      // write a marker file, POST its path as ffmpeg_path
      // assert 400, that the marker file was not executed, and that the response names the path
  }

  func TestHandleMediaConversionTestRequiresAdmin(t *testing.T) {
      // unauthenticated request -> 401
  }
  ```

- [ ] **Step 2 — verify it fails.**

- [ ] **Step 3 — implement the path validation.**

  ```go
  // validateMediaConversionBinaryPath accepts only an existing regular file whose base name is
  // an approved converter. "convert" is approved on non-Windows only, because
  // C:\Windows\System32\convert.exe is the FAT converter.
  func validateMediaConversionBinaryPath(path string) error
  ```

  The handler answers `400` with the reason instead of executing anything.

- [ ] **Step 4 — require admin.** Register the route as
  `mux.Handle("/api/media-conversion/test", requireAdmin(s, handleMediaConversionTest(s)))` and
  add `"/api/media-conversion/test"` to `isAdminProtectedPath` so the bearer scope requirement is
  explicit and testable.

- [ ] **Step 5 — verify it passes.**

  ```bash
  go test ./internal/server/ -run "TestHandleMediaConversionTest" -v
  go test ./internal/server/
  ```

- [ ] **Step 6 — commit.**

  ```bash
  git commit -m "fix(server): restrict the media conversion health probe" -m "Audit finding I. The health endpoint now requires admin and only executes an existing regular file whose base name is an approved converter."
  ```

**Acceptance:** a foreign binary path is rejected without execution; an unauthenticated call is
refused; a valid ffmpeg path still returns the health payload.

---

## Wave 3 — observability and tests

### Task MC7: output metadata and media-registry registration

Fixes J, K.

**Files:**
- Modify: `internal/tools/media_conversion.go:50-66` (`mediaConversionResult` — new fields),
  `:105` (signature: add the registry DB), `:246`/`:330` (probe the staged output),
  `internal/agent/dispatch_filesystem.go:49` (pass `dc.MediaRegistryDB`)
- Modify: `prompts/tools_manuals/media_conversion.md` (document the new result fields)
- Test: `internal/tools/media_conversion_test.go`

**Steps:**

- [ ] **Step 1 — failing test.** Assert that a successful conversion reports `output_size_bytes`
  and that a `media_items` row exists for the produced file with
  `source_tool = "media_conversion"`.

- [ ] **Step 2 — verify it fails.**

- [ ] **Step 3 — implement.**

  Add `OutputSizeBytes int64 \`json:"output_size_bytes,omitempty"\`` to `mediaConversionResult`.
  After the conversion, stat the staged output and report its size. For images, reuse the
  existing `image.DecodeConfig` path; for audio and video, reuse the ffprobe result that
  `ffprobeInfoResult` already produces instead of running a second probe.

  Change the signature to
  `ExecuteMediaConversion(ctx context.Context, workspaceDir string, cfg *config.MediaConversionConfig, registryDB *sql.DB, req MediaConversionRequest) string`
  and register the result with `RegisterMedia(registryDB, MediaItem{MediaType: …, SourceTool:
  "media_conversion", Filename: …, FilePath: destination, FileSize: …, Format: …, Hash: sha256 of
  the file, DurationMs: …})`. A registration failure must not fail the conversion — the file is
  already written; report it as a warning inside the result instead.

- [ ] **Step 4 — verify it passes.**

  ```bash
  go test ./internal/tools/ -run "TestExecuteMediaConversion" -v
  go test ./internal/agent/ -run "TestDispatch" -v
  ```

- [ ] **Step 5 — commit.**

  ```bash
  git commit -m "feat(tools): report and register media conversion output" -m "Audit findings J, K. A successful conversion now reports the output size and registers the produced file in the media registry so the media UI and send_audio/send_video can find it."
  ```

**Acceptance:** the result reports the output size; the file appears in the media registry; a
registry failure does not lose the converted file.

---

### Task MC8: close the test gaps

Fixes L.

**Files:**
- Modify: `internal/tools/media_conversion_test.go`
- Create: `internal/server/media_conversion_handlers_test.go` (created in MC6)

**Steps:**

- [ ] **Step 1 — replace the misleading oversized-file test.** The current
  `TestExecuteMediaConversionRejectsOversizedFile` never reaches the rejection branch. Rewrite it
  so the file is genuinely larger than `MaxFileSizeMB`, and assert the error message contains
  the size limit.

- [ ] **Step 2 — add the missing cases.**

  | Test | Asserts |
  | --- | --- |
  | `TestExecuteMediaConversionRejectsOversizedFile` | a file above the cap is rejected with the limit in the message |
  | `TestExecuteMediaConversionOutputWithoutExtension` | `output_file: "out"` + `output_format: "mp3"` writes `out.mp3` |
  | `TestExecuteMediaConversionFormatConflict` | `out.mp4` + `mp3` errors and names both values |
  | `TestExecuteMediaConversionTimesOut` | a deadline produces a timeout message |
  | `TestExecuteMediaConversionImageConvert` | the ImageMagick path round-trips through staging |
  | `TestResolveConversionTargetRejectsUnsupportedFormat` | the allowlist error lists the supported formats |
  | `TestHandleMediaConversionTestRejectsForeignBinary` | a foreign path is not executed |
  | `TestResolveConfiguredImageMagickSkipsSystemConvert` | the Windows system `convert` is not selected |

- [ ] **Step 3 — record the contract.** Add a `media_conversion` entry to the local contracts in
  `internal/tools/AGENTS.md` covering: the single resolved target, the format allowlist, the
  ffmpeg protocol whitelist, the caller-context deadline, the health-endpoint binary allowlist
  and the media-registry registration. Name the verifying tests, as the neighbouring entries do.

- [ ] **Step 4 — run the full package suites.**

  ```bash
  go test ./internal/tools/ ./internal/config/ ./internal/server/ ./internal/agent/ ./internal/mediabins/
  ```

- [ ] **Step 4 — commit.**

  ```bash
  git commit -m "test(tools): cover media conversion target resolution and limits" -m "Audit finding L. The oversized-file test now reaches the rejection branch, and the target resolution, timeout, image conversion and health-endpoint paths have their own cases."
  ```

**Acceptance:** every row of the table above has a passing test; the suite fails if the
staged-extension derivation regresses.

---

### Task MC9 (optional, separate PR): migrate the video studio to `mediabins`

Fixes the remaining duplication behind B and D.

**Files:**
- Modify: `internal/videostudio/runtime.go:110` (`resolveBinary`), `:168` (`runSmallCommand`),
  and every call site of both helpers.

**Steps:**

- [ ] **Step 1 — confirm the blast radius.**

  ```bash
  node .gitnexus/run.cjs impact "resolveBinary" --direction upstream --repo .
  node .gitnexus/run.cjs impact "runSmallCommand" --direction upstream --repo .
  ```

- [ ] **Step 2 — migrate the helpers** so that `videostudio` delegates to `mediabins` with
  identical behavior. Do not change any argument vector in the same commit.

- [ ] **Step 3 — verify.**

  ```bash
  go test ./internal/videostudio/
  ```

- [ ] **Step 4 — commit** as a standalone pure-refactor commit, separate from the media-conversion
  work, so a regression there can be reverted independently.

---

## Risk and rollback

| Risk | Mitigation |
| --- | --- |
| The format allowlist rejects a format that used to work | The allowlist is explicit per media type; MC4/MC1 document it in the tool manual. If a legitimate format is missing, add it in one line plus the demuxer map entry. |
| `-protocol_whitelist` blocks a local-but-unusual input | `file,pipe` covers every local path and pipe. The format whitelist is applied only for known extensions. |
| The new `mediabins` package changes error text | Error messages are internal to the tool; the tool returns its own fixed strings. |
| The `ExecuteMediaConversion` signature change breaks a caller the graph missed | Step "Confirm callers" in MC1 and MC5 plus `grep -rn "ExecuteMediaConversion" --include=*.go .` before each signature change. The known callers are `internal/agent/dispatch_filesystem.go` and `internal/tools/media_conversion_test.go`. |
| The media-registry write inside a conversion touches an unexpected table | The registration uses the existing `RegisterMedia` helper and the same `media_items` schema as every other media tool; it runs only after a successful conversion. |

Every task is a single commit and can be reverted independently. MC1–MC4 must land before MC5
and MC7, because those change the function signature that MC1 establishes.

## Verification

After the last task:

```bash
go build ./...
go test ./internal/tools/ ./internal/config/ ./internal/server/ ./internal/agent/ ./internal/mediabins/ ./internal/videostudio/
.\scripts\go-test-with-cache.ps1 -Packages @('internal/tools','internal/config','internal/server','internal/agent','internal/mediabins','internal/videostudio')
go vet ./...
```

Manual acceptance on a machine with ffmpeg and ImageMagick installed:

1. `media_conversion` with `operation: "audio_convert"`, `file_path: "in.wav"`,
   `output_file: "out"`, `output_format: "mp3"` → produces `out.mp3`, reports
   `output_format: "mp3"` and `output_size_bytes`.
2. The same call with `output_file: "out.mp4"` → an error naming both `out.mp4` and `mp3`.
3. `operation: "info"` on a video returns duration and dimensions.
4. A conversion of a file larger than `max_file_size_mb` is rejected with the limit in the message.
5. The config UI shows the size cap, and saving with `timeout_seconds: 999999` is refused by the
   API even though the field is clamped client-side.
6. `POST /api/media-conversion/test` with a foreign path answers `400` and executes nothing;
   without a session it answers `401`.

## Out of scope

- Installing or bundling ffmpeg / ImageMagick; the health probe only reports availability.
- A browser preview of converted media.
- Preset selection beyond the existing `quality_pct`, `width` and `height` parameters.
- The video studio's rendering pipeline beyond the optional MC9 extraction.

## Execution record

Implemented on 2026-10-23 on branch `main`. One commit per task:

| Task | Commit | Findings closed |
| --- | --- | --- |
| MC3 | `480e76dcd` feat(tools): add shared mediabins runtime for converter discovery | B, D |
| MC1 + MC2 | `bcae432f6` fix(tools): resolve media conversion target once and harden ffmpeg | A, F, G, C |
| MC6 | `c8ee59ce4` fix(server): restrict the media conversion health probe | I |
| MC4 (config) | `ccdaebad5` feat(config): validate media conversion settings | H |
| MC5 | `468fc5f66` fix(tools): honor the caller context in media conversion | E, D |
| MC4 (ui) | `de2cca3b6` feat(ui): expose the media conversion input size cap | H |
| MC7 | `df79b56c9` feat(tools): report and register media conversion output | J, K |
| MC8 | `2dc628fd0` test(tools): cover media conversion target resolution and limits | L |
| MC9 | `2b93b0138` refactor(videostudio): share converter discovery and execution with mediabins | B, D (debt) |

Deviation from the plan: the config validator normalizes instead of rejecting while the
configuration is loaded, so an out-of-range value in an existing `config.yaml` cannot block the
startup. The configuration API still rejects such a value.

Verification: `go build ./...`, `go vet` over all touched packages, `go run
scripts/validate_i18n_json.go` (1825 files, 0 errors) and the test suites of `internal/tools`,
`internal/mediabins`, `internal/config`, `internal/server`, `internal/agent` and
`internal/videostudio` are green. The only failing test in the repository is
`TestPythonQualityValidationIgnoresAgentVenvCWDAndPythonPath`, which needs a trusted system
Python that this machine does not have; it is unrelated to the media conversion work.

## Review record

The executed plan was reviewed on 2026-10-23 (code review plus an independent verifier). Seven
findings came out of that review; all of them are fixed and committed in one follow-up series.

| Finding | Problem | Fix | Files |
| --- | --- | --- | --- |
| R1 | The media conversion settings were only validated on the setup path, so `PUT /api/config` accepted an out-of-range `timeout_seconds` or `max_file_size_mb`. | `validateMediaConversionPatch` runs on every configuration save path. | `internal/server/config_media_conversion.go`, `internal/server/config_handlers_main.go`, `internal/server/config_patch.go` |
| R2 | The health endpoint refused a bare converter name, because it checked the raw configured string with `os.Stat` instead of resolving it. | The configured value is resolved through `internal/mediabins` exactly like the tool does; the base name of the resolved file decides. | `internal/server/media_conversion_handlers.go` |
| R3 | The registry entry pointed at the workspace file, which the media UI filters out, so a converted file was never visible. | The result is copied into the media data directory and registered there with its web path. | `internal/tools/media_conversion.go` |
| R4 | A pre-existing out-of-range value in `config.yaml` blocked every configuration save. | The validation is patch-scoped: only the values the patch itself changes are checked. `config.Load` still clamps the rest at startup. | `internal/server/config_media_conversion.go` |
| R5 | The registry description named the staging file instead of the source. | The source name is captured before staging and used in the description. | `internal/tools/media_conversion.go` |
| R6 | The in-place guard compared paths case-sensitively, so on a case-insensitive filesystem `Input.mp3` and `input.mp3` passed as two different files. | The comparison ignores the case on Windows and macOS. | `internal/tools/media_conversion.go` |
| R7 | The tool schema did not name the supported formats, so the model had to guess. | The description lists the allowlist per operation. | `internal/agent/native_tools_content.go` |

Deliberate scope decisions:

- The health endpoint does not pin the location of the converter: an administrator may test their
  own ffmpeg build. The residual risk is that an administrator can execute a renamed payload; the
  endpoint stays administrator-only.
- `DeleteMedia` remains a soft delete, so a registry entry can never delete a workspace file.
- The file name in the media data directory is hash based (`<stem>_<hash8>.<ext>`), so two
  conversions with the same name cannot overwrite each other.
- The workspace output stays exactly where the user asked for it; only the registry copy moves.

| Finding | Commit |
| --- | --- |
| R1, R4 | `5ba5776d7` fix(server): validate media conversion settings on every configuration save |
| R2 | `eff91d4b7` fix(server): resolve the probed converter before approving it |
| R3, R5, R6 | `b75a296ac` fix(tools): register the converted file in the media directory |
| R7 | `cc2957afb` fix(agent): name the supported output formats in the tool schema |

Verification of the review fixes: `go build ./...`, `go vet` over the touched packages,
`go run scripts/validate_i18n_json.go` (1825 files, 0 errors) and the test suites of
`internal/tools`, `internal/mediabins`, `internal/config`, `internal/videostudio`, `internal/agent`
and `internal/server` are green apart from failures outside the media conversion scope:

- `TestPythonQualityValidationIgnoresAgentVenvCWDAndPythonPath` (`internal/tools`) and
  `TestSkillQualityMaintenanceImprovementRestoresRunningDaemonWithNewApprovedHash`
  (`internal/agent`) need a trusted system Python, which this machine does not have;
- `TestReleaseScriptsPreserveCodeStudioDockerfile` reads a release script that an unrelated
  commit changed;
- `TestGetDesktopServiceReusesCanonicalConfigWithRelativeDefaults` computes a relative path from
  the package directory to a temporary directory, which fails when the temporary directory sits on
  another drive;
- `TestMQTTMissionOwnershipFollowsEnableUpdateAndDelete` is timing dependent and passes on its
  own.

