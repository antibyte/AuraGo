# RB-338 mode for Synth Studio: design and implementation plan

The design keeps the OpenRB-338 engine exact, and the file layout follows existing repo conventions. Two of these conventions back the main choices:
- **Rust crate:** `tools/aurago-tui/` is already a standalone Rust crate (own `Cargo.toml`, `.gitignore` with `/target/`).
- **GPL module:** the Strudel integration already ships a copyleft browser module this way (`ui/js/vendor/strudel/{LICENSES.md,sources.zip,manifest.json}`, `scripts/build-strudel-vendor.mjs`, notice in `licences/THIRD_PARTY_NOTICES.md:16-28`).

## 0. Decisions at a glance

| Topic | Decision |
|---|---|
| Wrapper crate | `tools/rb338-wasm/` (package `aurago-rb338`, GPL-3.0-only, `cdylib` + `rlib`). It depends on `orb-engine`, `rbs` and `rbm` as **pinned git dependencies** (rev `0bebfcdfd544b3cdea14536f19fb28a8de314df5`). Raw `extern "C"` ABI, no wasm-bindgen. |
| ABI style | One **wire-message batch** entry point for all panel edits. The bytes are exactly the original panel's messages as OpenRB encodes them (`orb-ui/src/link.rs:126-157`, `Msg::encode`). A few lifecycle exports cover create, load, save, render, snapshot and export. |
| Engine timing | 1024-frame engine calls. Queued commands are drained at the start of each call, as in orb-player (`orb-player/src/lib.rs:21-29, 682-698`). The output is identical to OpenRB live, and to the original. |
| Audio path | A dedicated `AudioContext({sampleRate: 44100})`. The wasm instance runs inside an `AudioWorkletProcessor`; the Module is compiled on the main thread and posted, with a bytes fallback. Commands go in by postMessage and snapshots come back after every call. A linear resampler is used only if the browser refuses 44.1 kHz. |
| Offline render | A dedicated Worker with its own wasm instance handles WAV export and Arrange clips. It runs compose → save bytes → parse → fresh engine → `frame::export`. This is the same path OpenRB's "Export Loop" uses (`docs/ui_spec.md` §18 "Export"), so song-mode WAVs are byte-identical to `orb render`. |
| Source of truth | The engine in the worklet. JS holds a **pattern mirror** (patterns never change by themselves), as OpenRB's `Link` does (`link.rs:26-33`). Knob, transport and mixer values follow the snapshots. |
| Audio clips | The Synth Studio model has **no audio clips**. Add project **v4** with `track.instrument:'audio'` and `clip.audio = {kind:'rb338', song:<base64 .rbs>, mode, start, end, frames}`. The PCM is **re-rendered deterministically** by the Worker and cached as an AudioBuffer, so no binary is stored. |
| Licensing | Under GPL-3.0-only: the wasm, the crate, and the JS host, worklet, worker, model and panel (the panel ports OpenRB behaviour). The CSS skin stays MIT. Strudel precedent wording: "the browser combination containing it follows GPL terms". |

---

## A. Design

### A1. Module map and licences

| Path | What | Licence |
|---|---|---|
| `tools/rb338-wasm/` (`Cargo.toml`, `Cargo.lock`, `rust-toolchain.toml`, `.gitignore`, `README.md`, `LICENSE`, `src/*.rs`, `tests/engine.rs`, `tests/data/{909_bass,303_slide}.rbs`) | wasm wrapper crate | GPL-3.0-only |
| `scripts/build-rb338-vendor.mjs` | builds the wasm and copies the artefacts, source zip and notices | GPL-3.0-only (part of the corresponding source) |
| `ui/js/vendor/rb338/rb338-engine.wasm`, `Standard.rbm`, `LICENSE.txt` (copy of `licences/texts/GPL-3.0-only.txt`), `LICENSES.md`, `manifest.json`, `sources.zip` | shipped artefacts, committed | GPL-3.0-only; `Standard.rbm` is Propellerhead material |
| `ui/js/desktop/apps/synth-studio-rb338-model.js` | pure logic (UMD, node-testable) | GPL-3.0-only |
| `ui/js/desktop/apps/synth-studio-rb338-host.js` | asset cache, live engine, renderer, RB-clip cache | GPL-3.0-only |
| `ui/js/desktop/apps/synth-studio-rb338-worklet.js` | AudioWorkletProcessor `aurago-rb338` | GPL-3.0-only |
| `ui/js/desktop/apps/synth-studio-rb338-worker.js` | offline renderer Worker | GPL-3.0-only |
| `ui/js/desktop/apps/synth-studio-rb338.js` | the panel (DOM, interaction, follow, document, autosave) | GPL-3.0-only |
| `ui/css/desktop-app-synth-studio-rb338.css` | rack layout and skin (`ss-rb-*` classes, never `cc-`) | MIT |
| Synth Studio edits (workspace, model v4, audio, editor, export, synth-studio.js) | integration | MIT |

Every GPL file gets the header `// SPDX-License-Identifier: GPL-3.0-only` plus "Derived from OpenRB-338 (commit 0bebfcd), Copyright (C) 2026 the OpenRB-338 authors" (`OpenRB README.md:157-160`).

### A2. The Rust wrapper crate

**Dependency approach (verified against the clone)**
- `orb-engine` depends only on `rbs` and `libm =0.2.16` (`crates/orb-engine/Cargo.toml:8-11`).
- `rbm` depends on `orb-engine`, `toml =0.9.12` and `sha2 0.10` (`crates/rbm/Cargo.toml:8-13`). `rbs` has no dependencies.
- The dev-dependency cycles (orb-engine ↔ rbm, orb-testkit) are irrelevant to dependents.
- `version.workspace = true` and friends resolve against the git checkout's own workspace root. Cargo supports this for git deps.
- The upstream `[patch.crates-io] raw-window-metal` and `[profile.*]` (`Cargo.toml:12-41`) are ignored for dependencies (patches and profiles only apply from the top-level workspace). The upstream `rust-toolchain.toml` is ignored too.
- So the crate needs its own `rust-toolchain.toml` (`channel = "1.99.1"`, `targets = ["wasm32-unknown-unknown"]`, `profile = "minimal"`) and its own profiles.
- None of the three crates declares features. Library code uses no `std::time`, threads or fs (grep: only tests do).

`Cargo.toml` shape:
```toml
[package]  name="aurago-rb338" edition="2024" license="GPL-3.0-only" publish=false
[lib]      crate-type=["cdylib","rlib"]
[dependencies]
orb-engine = { git="https://github.com/pingw33n/OpenRB-338", rev="0bebfcdfd544b3cdea14536f19fb28a8de314df5" }
rbs        = { git=…, rev=… }
rbm        = { git=…, rev=… }
[dev-dependencies] sha2="0.10"
[profile.release] opt-level=3  lto="fat"  codegen-units=1  panic="abort"  strip=true
[profile.test]    opt-level=3
```
Use opt-level 3, not "z": DSP speed matters in the worklet. Rust has no fast-math, so the numerics are unaffected.

**Keep the 2.5 MB kit out of the wasm.** `rbm::standard::EMBEDDED` uses `include_bytes!` (`rbm/src/standard.rs:136`). The cdylib must not reference `EMBEDDED`, `builtin_samples` or `builtin_file`; only the Rust tests may. The linker then drops it. The build script asserts the wasm stays under about 1.5 MB.

**Internal modules** (each names what it ports):
- `kit.rs`
  - `Rbm::parse(bytes)` → `KeyTable::embedded(&r).filter(|t| r.all_covered(t))` → `r.load_all_samples(&t)` → `Arc<SampleSet>`.
  - Refs: `rbm/src/lib.rs:294, 468, 554`; `standard.rs:139-160, 179-189`.
- `session.rs`: `Ctx { tables: Arc<Tables>, process: ProcessState, kit, engine: Box<Engine>, doc: Doc{base: rbs::Song, form, mod_name}, queue, buffers, preview, snap, io, applied_seq, frames, generation }`.
  - **create:** `Tables::new()` (`tables/mod.rs:75`) and `ProcessState::new()` (`process.rs:144`). Then `Engine::new(tables, Devices::implemented_with(Some(kit), process.clone()))`, the same as `Player::with_call_frames` (`orb-player/src/lib.rs:498-545`).
  - **new song:** like `Link::load_new` (`link.rs:807-824`). Drain the queue, stop, build a new engine, then activate it.
  - **load:** `open_bytes` semantics (`orb-ui/src/session.rs:31-56`).
    - A 1.x file: `rbs::rb1x::is_1x` → `Song1x::parse` → `orb_engine::load::convert_1x`. The doc base is the `source_of_1x` stand-in (`document.rs:82-93`); port `Usri1x::of` (`orb-cli/src/info.rs:99-118`, about 15 lines).
    - Otherwise `rbs::Song::parse` → `load::convert` (`load.rs:479`).
    - Build like `Player::build`: `Engine::new` + `Engine::load` (`lib.rs:558-569`; `frame/mod.rs:1315-1355`).
    - **Activate like `Audio::activate`:** the old song is stopped, `silence_all()` runs on the old song's devices (this refills the shared delay line), then swap (`lib.rs:915-925, 939-943`). Do **not** call `Engine::activate`; that is the rbhost/export path.
    - First drain the queue into the old song, as `Link::queue_song` forwards the pending events and pushes Stop then Load (`link.rs:827-851`).
  - **render call:** port `Audio::drain` + `render_call` (`lib.rs:690-698, 751-773`).
    - Apply the queued messages, zero the i16 buffer, then `engine.render(buf, 1024)` (`frame/mod.rs:1438`). If the fault count rose, zero the buffer again.
    - Mix the preview (startup sound) after the song (`lib.rs:294-304, 759-768`).
    - Convert to planar f32 as `x/32768` (`lib.rs:657-663`). Then write the snapshot.
- `wire.rs`: decodes one panel message and applies it to the engine. It ports `Link::command` (`link.rs:1074-1138`) plus `Audio::apply` (`lib.rs:796-909`); the table is below. Typed helpers such as `Ctx::device_param` are kept for the Rust tests.
- `snapshot.rs`: a fixed-layout writer, a port of `Audio::publish` (`lib.rs:945-1013`) reduced to what the panel needs. Controller blocks are written **in RBS controller numbering**; the TR-909 is mapped through `tr909::internal_index` (`tr909.rs:164-171`), so JS never sees internal indices. Step lights follow the rule in `link.rs:1267-1274`.
- `compose.rs`: Save. It ports `document::compose` (`orb-ui/src/document.rs:119-279`) and `set_engine_tracks` (`:101-116`), but reads the **engine** instead of panel and link:
  - Device values: `dev.saved_block(song_mode)` (`frame/mod.rs:247`; this is the live block in pattern mode and the pattern-mode copy in song mode, matching what compose takes from `panel`/`song_mode_blocks`).
  - Mixer: `devices.mixer.saved(song_mode)` (`frame/mod.rs:681-691`).
  - Patterns: `channels[c].patterns_mut()` (`:268`).
  - Tracks: `seq.events(t)` (`seq.rs:773`). Globals: `seq.globals`.
  - Keep compose's rules intact:
    - the "unchanged-since-load byte stays as read" `put` rule for current 4.2.0 files, with `convert(base)` computed on the fly;
    - the 303 flag bits 5..7 stay as read;
    - the 808 `raw != 0` rule;
    - the 909 `internal_index`;
    - the version is always written as 4.2.0;
    - the vintage byte is written as `(vintage != 0) as u8`;
    - the mod name comes from the doc (for new songs "Standard ReBirth", `rbm::standard::NAME`).
  - Then `rbs::Song::write()` (`rbs/src/lib.rs:1059`).
- `export.rs`: the offline WAV.
  - Song mode: `parse → convert → Engine::new(tables, Devices::implemented(Some(kit)))` → `frame::export::export_song(&mut e, &song, 9088)` → `wav_bytes` (`frame/export.rs:129-143, 165-172`; `orb-cli/src/render.rs:20, 172-187`). The corpus tests always pass the kit (`orb-engine/tests/common/mod.rs:53-68`), so always pass it.
  - Pattern mode (our extension; the original disables export there, `ui_spec.md` §11): `load` + `activate(ReplacesCurrent{current_playing:false})` + `export::export(e, ExportRange{tempo, loop_start:0, loop_end: bars*768}, 9088)` (`export.rs:103-123`).

**C ABI** (all `#[no_mangle] pub extern "C"`; handle = `*mut Ctx` from `Box::into_raw`):

| Export | Purpose |
|---|---|
| `orb_abi_version() -> u32` | guards against stale cached assets (=1) |
| `orb_alloc(len) -> *mut u8`, `orb_free(ptr, len)` | linear-memory buffers |
| `orb_create(kit_ptr, kit_len) -> *mut Ctx` (null on error) / `orb_last_error() -> u32` | context with kit and new empty song |
| `orb_destroy(h)` | |
| `orb_new_song(h) -> i32` | File > New |
| `orb_load_song(h, ptr, len) -> i32` | 0 ok, 1 not a song, 2 cannot convert, 3 `LoadRefused` (`frame/mod.rs:139-152`, ALERT text) |
| `orb_save_song(h) -> i32` | drains the queue, composes; RBS bytes go to `io` |
| `orb_song_info(h) -> i32` | small JSON in `io`: `{version, form:"rbs"|"1x", mod, title, text}` |
| `orb_patterns(h) -> *const u8` | 18688 bytes: 303#1 32×98, 303#2 32×98, 808 32×194, 909 32×194 (`link.rs:186-192`) |
| `orb_defaults(h) -> *const u8` | new-song controller values (same layout as snapshot bytes 64..183), a port of `new_song_knobs` (`link.rs:382-419`); used for double-click reset |
| `orb_push(h, ptr, len, seq) -> i32` | appends a wire batch (repeated `u16 LE len` + message bytes); applied at the next call start |
| `orb_apply(h)` | drains the queue now |
| `orb_render(h) -> *const f32` | one 1024-frame call; planar L[1024] then R[1024] |
| `orb_snapshot(h) -> *const u8` | 192-byte snapshot (below) |
| `orb_startup_sound(h)` | preview player with `samples::STARTUP_ID` (`lib.rs:596-600, 900-906`); the logo click |
| `orb_io_ptr(h)`, `orb_io_len(h)` | result bytes of save, info and export |
| `orb_export_wav(h, song_ptr, song_len, mode(0 song, 1 pattern), bars, idle_blocks) -> i32` | WAV in `io`; does not touch `h.engine` |

**Wire messages** (the original panel's, `link.rs:123-157, 1005-1071`; values are BE u32 for ids 4, 8, 12, 16):

| Edit | Bytes | Engine call |
|---|---|---|
| mode (0 pattern, 1 song) / play-stop / record / loop | `[35,0,m]` `[35,1,1|0]` `[35,2,v]` `[35,3,v]` | `set_song_mode` / `play`/`stop` / `set_record` / `set_loop_on` (`frame/mod.rs:1638,1605,1508,1645,1656`) |
| tempo (BPM×1000) / locate (ticks) / loop start / loop end | `[35,4,u32]` `[35,8,u32]` `[35,12,u32]` `[35,16,u32]` | `set_tempo`/`locate`/`set_loop_start`/`set_loop_end` |
| shuffle / vintage | `[35,20,v]` `[35,21,v]` | `set_shuffle`/`set_vintage` |
| knob, switch, on/off (ctrl 0), pattern select (ctrl 1) | `[41,dev,sub,ctrl,v]`, sub 303=40, 808=57, 909=74, delay=84, dist=92, comp=102, PCF=114 | `Engine::device_param` (`frame/edit.rs:122`) |
| master / comp id / PCF id | `[41,0,145,id,v]` | `mixer_global` (`edit.rs:135`) |
| channel mute(0)/level(1)/pan(2)/delay(3)/dist(4) | `[41,0,147,c,k,v]` | `mixer_channel` (`edit.rs:153`) |
| pattern shuffle(0)/length(1) | `[41,dev,42|59|76,p,field,v]` | `pattern_field` (`edit.rs:174`) |
| whole pattern (Clear, Cut, Paste, Duplicate, Undo) | `[41,dev,44|61|78,p,98|194 bytes]` | `pattern` (`edit.rs:198`) |
| 303 step field (0 note 0..12, 1 slide, 2 accent, 3 up, 4 down, 5 rest) / drum step (field = instrument, 808 0/1, 909 0..3) | `[41,dev,45|62|79,p,s,field,v]` | `step_field` (`edit.rs:216`) |

Notes on the user's list:
- "Gate/tie" does not exist in ReBirth: a step is note, rest, slide, accent, up or down.
- "Solo" does not exist either (only per-channel mute and pattern-section on/off).
- Song-mode "track editing" in OpenRB is recording only (Record in song mode → record hook, `edit.rs:8-12`).

**Snapshot v1** (192 bytes, little-endian; defined once in `snapshot.rs` and mirrored as constants in the JS model):

| Offset | Content |
|---|---|
| 0 | version=1 |
| 1 | flags: bit0 playing, 1 recording, 2 song_mode, 3 loop_on, 4 vintage, 5 faulted since last, 6 preview playing |
| 2 | shuffle |
| 4 | `u32` applied_seq |
| 8 | `f64` frames |
| 16 | `i32` tick |
| 20 / 24 / 28 | `u32` loop_start / loop_end / tempo |
| 32 / 36 / 40 | `u32` generation / tracks revision (`seq.revision`) / merges (`seq.merges`, `seq.rs:721-723`) |
| 44 | `u8[4]` play_step (255 = none) |
| 48 | `u8[4]` playing_pattern |
| 52 | `u8[4]` channel meters |
| 56 | `u8[2]` master meters |
| 58 | comp gain byte (comp live 3) |
| 59 | `u8[4]` effect meters (delay live 5, dist 3, PCF 7, comp 4; `link.rs:1256-1265`) |
| 64 | mixer[23]: master, comp_id, pcf_id, 4×(enabled, level, pan, send, dist) |
| 88 | 303#1 ctrl 0..8 |
| 97 | 303#2 ctrl 0..8 |
| 106 | 808 ctrl 0..28 |
| 135 | 909 ctrl 0..29 by RBS number |
| 165 | delay 0..4 |
| 170 | dist 0..2 |
| 173 | PCF 0..6 |
| 180 | comp 0..2 |

**wasm notes**
- No imports are expected; the build script asserts `WebAssembly.Module.imports(m).length === 0`.
- `panic=abort` means a panic traps. The worklet catches it, outputs silence, posts `crash`, and the host offers a restart from the last autosave.
- Memory growth detaches typed-array views: re-create views after every call that can allocate (create, load, save, push, export).

### A3. Build script and artefacts

`scripts/build-rb338-vendor.mjs` is modelled on `scripts/build-strudel-vendor.mjs:1-42`, using `fflate.zipSync` with fixed mtimes:
1. `cargo +1.99.1 build --release --locked --target wasm32-unknown-unknown --manifest-path tools/rb338-wasm/Cargo.toml`
2. Load `target/wasm32-unknown-unknown/release/aurago_rb338.wasm`. Assert there are no imports, the required exports exist, and the size is under about 1.5 MB. Copy it to `ui/js/vendor/rb338/rb338-engine.wasm`.
3. `cargo metadata --format-version 1` gives the `rbm` package `manifest_path`. The repo root is three levels up; take `assets/Standard.rbm` from there. Assert SHA-256 `6355b5f14fc2ebfe7ed9e6e42208f97817f96c5e30ed42fdca4205d3cdab5fbe` and 2,572,792 bytes (`OpenRB assets/README.md:43-47`). Copy it.
4. `cargo vendor --versioned-dirs <tmp>` collects the exact sources of every crate (orb-engine, rbs, rbm, libm, toml…, sha2…). Build `sources.zip` from:
   - `aurago-rb338/` (crate, `Cargo.lock`, toolchain file);
   - `vendor/`;
   - `aurago/` (the GPL JS files and this build script);
   - a `README.md` with rebuild steps and the upstream URL and commit.
5. `LICENSE.txt` is a copy of `licences/texts/GPL-3.0-only.txt`. Write `LICENSES.md` (the notice) and `manifest.json` (upstream repo and rev, toolchain, and bytes plus SHA-256 per file, as Strudel's `manifest.json` does).

The built artefacts are committed, so AuraGo's normal build needs no Rust. Packaging and serving:
- `assets/web-assets.json:5`: add `".rbm"` to the ui extensions. `.wasm`, `.zip` and `.txt` are already allowed; `LICENSES.md` ships through the LICENSE-prefix rule in `cmd/assetpack/main.go:74`.
- MIME: `http.FileServer` → Go's `mime` maps `.wasm` to `application/wasm`; `.rbm` is sniffed as octet-stream. The JS uses fetch → `arrayBuffer` → `WebAssembly.compile`, so it does not depend on MIME.
- Fetch through `AuraLazyAssets.versionedURL(...)` (`ui/js/shared/lazy-assets.js:7-16`), falling back to `'?v='+(window.BUILD_VERSION||'dev')`. Without `?v`, `versionedUIHandler` sends `no-store` (`internal/server/assets_handlers.go:151-155`).
- CSP already allows `'wasm-unsafe-eval'` and `worker-src 'self' blob:` (`internal/server/security_headers_test.go:28,32`).
- `.gitattributes`: after line 54 (`ui/js/vendor/** text=auto …`) add `ui/js/vendor/rb338/*.wasm binary`, `*.rbm binary`, `*.zip binary`, and `tools/rb338-wasm/tests/data/*.rbs binary`.

### A4. Audio path

- **Context:** create `new AudioContext({sampleRate: 44100, latencyHint: 'interactive'})` **synchronously inside the tab-click or Play gesture**; the asset load can follow asynchronously. If construction throws or `sampleRate !== 44100`, use the default context and pass `processorOptions.ratio = 44100/ctx.sampleRate`. The worklet then runs a linear resampler on the 44.1 kHz call output. Live playback is no longer bit-exact in that case; exports stay exact because the Worker never touches WebAudio. Chrome, Firefox and Safari 14.1+ accept 44100.
- **Worklet** (`registerProcessor('aurago-rb338', …)`, `outputChannelCount:[2]`):
  - `process()` fills the 128-frame quanta from a 1024-frame call buffer. When the buffer is used up it calls `orb_render(h)`, copies L and R, and posts the snapshot as a transferred buffer.
  - This mirrors `Audio::pull` (`lib.rs:665-679`). 1024 = 8 quanta, so each call starts exactly at a quantum boundary.
  - Port messages arrive between quanta. Each `msgs` batch is `orb_push`ed at once and applied at the next call start, which is orb-player's exact semantics.
  - Command latency is at most one call (23 ms) plus output latency.
- **CPU:** each 8th quantum renders 1024 frames, a peak cost. T3 measures `orb_render` in node; target under 1.5 ms per call. If too slow, see C1.
- **Protocol**

  | Direction | Messages |
  |---|---|
  | main → worklet | `init{module|bytes, kit, song?}` · `msgs{buf, seq}` · `load{id, bytes}` · `new{id}` · `save{id}` · `startup` |
  | worklet → main | `ready{defaults, patterns, info}` · `loaded{id, ok, code, patterns, info}` · `saved{id, bytes}` · `snap{buf}` · `error{code}` · `crash{message}` |

  For `init`, if posting the Module throws `DataCloneError`, post the bytes; the worklet then calls `new WebAssembly.Module(bytes)` (sync compile is allowed off the main thread).
- **Suspend and dispose:** `context.suspend()` when the RB tab is left while stopped; `resume()` on return and on Play. `close()` the context and terminate the Worker on app dispose. RB keeps playing when the tab is left while it plays.
- **Transport exclusivity:** RB Play stops the Synth Studio transport and Strudel. Synth Studio Play and Strudel Play stop RB (hook into the existing `beforePlay`, `synth-studio.js:51`).

### A5. Offline renderer (Worker)

The Worker instantiates the same module with its own `orb_create(kit)`.

| Direction | Messages |
|---|---|
| main → worker | `init{module|bytes, kit}` · `render{id, song: ArrayBuffer, mode, bars}` |
| worker → main | `rendered{id, wav, frames}` · `error{id, code}` |

The song bytes always come from the live engine's `save`, which reproduces OpenRB's "song as saved" export. Pattern mode renders bars × 768 ticks from tick 0, with bars chosen from 1, 2, 4 (default), 8 or 16. Song mode renders the loop range.

### A6. UI: the rack in Synth Studio's skin

**Structure** follows `docs/ui_spec.md` §1-9. Layout: one row per channel (pattern section | device | mixer channel) for 303#1, 303#2, 808 and 909, plus a right column with Master, PCF, Delay, Dist and Comp, and the transport on top.

Each device is a plate using `--vd-mat-card*` and `--ss-edge`. The section headers use the label font (uppercase, tracking). Components:
- **Knobs:** `<input type=range min=0 max=127 data-ss-kind="knob" data-rb="tb303_1.cutoff" data-ss-default="…">`. The existing `SynthStudioControls.observe(root)` (`synth-studio-controls.js:44-49, 140-179`) already watches `.ss-app`, so the inputs are upgraded automatically; `data-ss-default` drives double-click reset (`:128-137`).
- **Faders:** the same input with `data-ss-kind="vfader"` for mixer level, master, and PCF Freq, Q, Amt and Dec.
- **Toggles and radios:** LED keys, i.e. buttons with `aria-pressed`, `--ss-key` and an LED dot (`--ss-phosphor`; amber for accent; `--ss-red` for record).
- **Two-way switches:** a segmented pair. The drawn position comes from `behavior::switch_position` (`orb-ui/src/behavior.rs:106-112`): the PCF shows LP only when the value is 1, everything else any non-zero value.
- **Displays:** `<output class="ss-rb-lcd">` with `--ss-screen` and `--ss-mono-font` phosphor digits, plus up/down repeat buttons (first step on press, then `repeat_extra_steps`, `behavior.rs:79-87`).
- **Meters:** `.ss-led-meter`/`.ss-led-seg` (`desktop-app-synth-studio-modern.css:596-604`) with 11 states via `meter_state` (`model.rs:312`). The effect headers show 4 segments via `header_meter_segments` (`:347`); the comp GR row has 9 LEDs via `gr_lit` (`:362`).
- **Tints:** the 808 level knobs get an `is-level` tint (the original's red level knobs).
- **Mod window:** OpenRB shows the kit's splash here; instead show a "Standard ReBirth" label and the song's mod name, with a notice when the song names another mod (it plays with the standard kit).

**Controls per device** (controller numbers from `link.rs:236-343` and `docs/ui_spec.md` §3-9):
- **Transport** (msg 35): Shuffle knob (20); tempo display 3 digits, ±1000 clamped 20000..500000; mode switch (0); Play, Stop, Rewind, FF (8, ±10 bars); Record (2, song mode only); bar display with up/down (8); Loop switch (3); loop-start display with up/down (12 then 16); loop-length display with up/down (16); logo (startup sound). The bar and loop displays are disabled in pattern mode.
- **Pattern section** (per row, device = 303#1/303#2/808/909):
  - on/off LED = ctrl 0;
  - 8 pattern buttons × banks A-D = ctrl 1 = (button−1)+8×bank; the bank is UI-only;
  - Shuffle button = header field 0 of the current pattern; Steps display ± = header field 1, clamped 1..16.
- **TB-303** (dev 1/2):
  - Waveform 8 (0 saw, 1 square); Tune 2, Cutoff 3, Reso 4, Env Mod 5, Decay 6, Accent 7.
  - Step editor: edit-step display; 13 keys C..C' (note 0..12) with LEDs; Down, Up, Accent and Slide with LEDs; Note/Pause with Note and Rest LEDs; Back, Step, Pitch mode with LED; Clear.
- **TR-808** (dev 3):
  - Knobs: AC level 2; BD level 3, tone 4, decay 5; SD level 6, tone 7, snappy 8; LT level 9, tuning 10; MT level 12, tuning 13; HT level 15, tuning 16; RS level 18; CP level 20; CB level 22; CY level 23, tone 24, decay 25; OH level 26, decay 27; CH level 28.
  - Switches: LT/LC 11, MT/MC 14, HT/HC 17, RS/CL 19, CP/MA 21 (0 = upper label).
  - Instrument legend (0 AC, 1 BD, 2 SD, 3 LT/LC, 4 MT/MC, 5 HT/HC, 6 RS/CL, 7 CP/MA, 8 CB, 9 CY, 10 OH, 11 CH); the upper labels select 3..7.
  - 16 step buttons with LED colour per group of four.
- **TR-909** (dev 4):
  - AC level 2; BD level 3, tune 4, attack 5, decay 6; SD level 7, tune 8, tone 9, snappy 10; LT level 11, tune 12, decay 13; MT level 14, tune 15, decay 16; HT level 17, tune 18, decay 19; RS level 20; CP level 21; HH level 22; CH decay 23; OH decay 24; CC level 25, tune 26; RC level 27, tune 28; Flam 29.
  - The Flam button is UI-only.
  - Legend: 0 AC, 1 BD, 2 SD, 3 LT, 4 MT, 5 HT, 6 RS, 7 CP, 8 CH, 9 OH, 10 CC, 11 RC.
  - 16 steps; values 1 orange, 2 red, 3 or more flam green.
- **Mixer channel** c=0..3: mute (147 k0), level fader (k1), pan (k2), delay send (k3), Dist (k4). PCF radio sets `pcf_id = c+1 | 0` (145 id2); Comp radio sets `comp_id = c+2 | 0` (145 id1). Meter.
- **Master:** two meters, fader (145 id0), Comp radio (`comp_id` 1).
- **PCF** (dev 7): on 0, Freq 1, Q 2, Amt 3, wave display ± 4 (0..54), Dec 5, BP/LP 6, meter.
- **Delay** (dev 5): on 0, steps display ± 1 (1..32), triplet(0)/16th(1) 2, F.Back 3, Pan 4, meter.
- **Dist** (dev 6): on 0, Amount 1, Shape 2, meter.
- **Comp** (dev 8): on 0, Ratio 1, Thres 2, meter, GR LEDs.

**Behaviour to port** (JS, GPL):
- Transport arithmetic and rules: `model.rs:77-148` (Transport), `:761-805` (stop when stopped locates to loop start or 0; record only in song mode), `:892-968` (arrows do nothing in pattern mode; rewind/FF/bar-down turn record off; loop start keeps the length), `:602-617` (switching to pattern mode also sends `{2,0}`), `:1032-1037` (locate loop).
- Pattern section: `:807-829`; adopt the engine's pattern and let the bank follow it (`:667-673`).
- 303 editor:
  - The edit step wraps modulo 16 and keys set the note (`:831-886`, `:235-252`).
  - Pitch mode resets the edit step to 1, and a key then advances it.
  - The note LED flashes for 0.4 s in pitch mode.
  - Clear sends the whole pattern with cleared steps: note 0, flags off, rest on; length and shuffle kept (`link.rs:195, 211-222`).
- Drums:
  - 808: a click toggles the selected instrument's step, and a drag paints the crossed steps with that new state (`ui_spec.md` §6, `model.rs:973-986`).
  - 909: values follow `tr909_step_value` (`behavior.rs:124-138`); Ctrl, Alt and Flam come from the event modifiers. A drag paints the crossed steps. Setting CH or OH clears the other hi-hat on that step (`model.rs:993-1014`).
  - Running light: the playing step is inverted.
- Taps: `tap_303` and `tap_drum` (`model.rs:1042-1071`).
- Mixer radios: `model.rs:721-746`. `adopt` skips the control being held (`:634-664`).

**Keyboard** (`ui_spec.md` §10). Use `KeyboardEvent.code` for physical positions; browsers distinguish numpad keys, so the original's table works without egui's compromises.
- Always active:
  - Space play/stop; NumpadEnter play; Numpad0 stop; Numpad4 rewind, Numpad5 FF; Numpad7/8 bar down/up; Numpad1/2 locate to loop start/end.
  - NumpadMultiply or Insert record; NumpadAdd/Subtract tempo; NumpadDivide loop.
  - ArrowUp/Down move the row focus (wraps); PageUp/Down scroll the rack.
- "Program synth", with a 303 focused:
  - C F V G B N J M K , L . / play notes 0..12; Enter is Step and Backspace is Back.
  - P accent, [ slide, ; down, ' up; Minus or Equal is Note/Pause.
  - Tab / Shift+Tab tap a note or rest on the playing step, only while playing; otherwise Tab moves focus normally.
- "Program synth", with a drum machine focused: Minus/Equal = AC; A S D F G H J K L ; ' = instruments 1..11 set on the playing step.
- "Select patterns": Digit1-8 → 303#1, Q-I → 303#2, A-K → 808, Z-, → 909 (shown bank).
- The keyboard mode is a toolbar toggle; Ctrl+F/G are not hijacked. Ignore `e.repeat` (repeat buttons repeat via keydown/keyup hold).
- Ctrl+X/C/V/D/Z run the RB pattern commands and Ctrl+S saves the RB song. All handled keys call `preventDefault()`, so Synth Studio's root handler skips them (`synth-studio.js:478-485` returns on `e.defaultPrevented`).

**Responsive rack:** `container-type: inline-size` on `.ss-rb338-workbench`, with vertical scroll.
- 1180px or wider: the original's 4-column grid, scaled.
- 760-1180px: the effects column moves to a bottom row.
- Under 760px: the pattern section stacks above its device and the mixer channel becomes a horizontal strip.

**Rendering:** a requestAnimationFrame loop runs only while the RB tab is visible. Only changed DOM values are written.

### A7. State ownership, refresh, documents

- **Engine** (worklet) holds all song state.
- **JS mirror:** the pattern arrays, refreshed from `orb_patterns` on ready, load, new and the undo restore. Local edits update the mirror and send the same message, a port of `Link::messages`/`drum_step`/`set_messages` (`link.rs:933-1071`).
- **Adopt rule:** knob, mixer, transport and row-pattern values are taken from a snapshot only when `snap.applied_seq >= lastSentSeq` and the control is not held. This replaces Link's frame mark (`link.rs:1198-1206, 1276-1279`); a sequence number is robust against postMessage delay. Meters and step lights update on every snapshot.
- **Coalescing:** consecutive same-control knob values are coalesced per animation frame. Steps, patterns and transport messages are sent immediately in order.
- **Modified:** any edit message except play, stop and locate, or a change of `merges`, marks the RB song as modified.
- **Open:** `<input type=file accept=".rbs,.RBS">` or drag-drop onto the rack → `load`. On error, show the mapped message.
- **Save:** `save` → download `<name>.rbs`.
- **Confirm** before New or Open replaces a modified song.
- **Autosave:** debounced 1.5 s after edits, and on tab leave and dispose. `save` bytes go to `OfficeSession.draft('put', location.origin+':synth-studio:rb338:current', {id, bytes, name, modified, updated}, 'synth-studio')`; the namespace exists in `writer-session.js:51`, and `drafts()` strips `bytes`. On init, read it back with `draft('get', …)` and pass it as `init.song`. UI prefs (key mode, focus, shown banks, instruments, export bars) go to `localStorage['aurago:synth-studio:rb338:1']`.
- **Pattern Edit commands:** Cut, Copy, Paste, Duplicate and one-level Undo, a port of `orb-ui/src/menu.rs:1124-1200` plus Link's empty-pattern helpers (`link.rs:199-231`). Undo of a pattern no longer selected waits until the engine stops playing it (`set_pattern_unplayed` + `follow`, `link.rs:682-700, 1233-1239`). Status lines follow `ui_spec.md` §18.
- **Export WAV:** Worker render → download `<name>.wav` (16-bit stereo 44.1 kHz).

### A8. Audio clips in Arrange

**Finding:**
- Clip keys are fixed (`synth-studio-model.js:17`, `normalizedClip` `:261-290`), and tracks need a preset `instrument` (`:459`).
- The audio engine schedules only notes, controllers, automation and metronome (`synth-studio-audio.js:101-195, 564-787`); the offline render does the same (`:969-1022`).
- Stems skip tracks without notes (`synth-studio-export.js:141`).
- There is no sample playback anywhere; only noise buffers exist (`synth-studio-voices.js:337,360`).
- The library "patterns" are note patterns.

So no existing mechanism can carry audio.

**Choice:** the smallest real addition, a v4 audio clip whose PCM source is the RB song itself, re-rendered by the deterministic Worker.

Why not alternatives:
- Inline PCM: 1.2 MB raw (1.6 MB base64) per 4-bar loop at 140 BPM would hit the 5 MiB project limit (`model.js:11`) after about three clips.
- IndexedDB blobs would not travel with saved `.aurasynth` files.
- Sidecar files via `/api/desktop/upload` need file-lifecycle plumbing and do not work for unsaved drafts.
- The RB song is about 20-40 KB of base64, bit-exact and re-editable.

**Schema (v4):**
- `validate` accepts `version` 4 (`model.js:415`).
- Allow `instrument:'audio'` only in v4, and add it to `assertInstrument`.
- On v4 clips, allow an optional `audio`: `{kind:'rb338', song: base64 (≤512 KiB decoded), mode:'song'|'pattern', start, end (RB ticks), frames}`.
- An audio clip must have empty `notes` and `controllers`, and an `'audio'` track must hold only audio clips.
- `upgrade` from v3 to v4 is a no-op.
- Projects stay v3 until the first audio clip is inserted; insertion sets `version=4`. Existing tests asserting `"version":3` keep passing.

**Playback** (`synth-studio-audio.js`):
- `create(options)` gets `options.audioBuffer(clip) → AudioBuffer|null`, and `renderWav(project, {audioBuffer})` gets the same.
- `normalizeProject` carries `clip.audio` through. `allEvents` emits `{type:'audio', tick:clip.start, track, clip}`.
- Start seeds: a clip that straddles the start tick starts with an offset. Loop cycles schedule the audio events in range, plus a straddling clip at `loopStart`.
- `process('audio')` creates an `AudioBufferSourceNode` → `busFor(track).input`, with `start(at, offset)` and `stop(at + min(remaining buffer, remaining clip length, loop end))`.
- `rangeEnd`, `end` and `stop()` stop the active sources.
- Audio events do not count toward the voice limits.
- For `instrument==='audio'`, open the bus filter fully and disable the drive in `createTrackBus`/`updateTrackBus` (`synth-studio-voices.js:446-530`) so the clip is unprocessed apart from volume, pan, inserts and sends.

**Elsewhere:**
- `renderStems` includes tracks with audio clips.
- MIDI export skips audio tracks.
- The editor draws audio clips as `ss-clip-audio` plates labelled "RB-338 · …".
- Selecting an audio clip does not open the piano roll; it shows a small panel with "Edit in RB-338" (loads the clip's song into the RB tab, with confirm).
- The inspector hides instrument and synth params for audio tracks.

**Send to Arrange** (RB toolbar):
1. Live `save` → Worker `render` → AudioBuffer (int16/32768) is cached by key `mode|start|end|song`.
2. If the RB tempo differs from the project tempo and lies within 40..240, offer to set the project tempo.
3. One `mutate` step adds a track named "RB-338" (or reuses an existing audio track of that name) with the clip at the playhead snapped to the bar. The clip length is `round(frames/44100 × tempo/60 × 480)` ticks, and the project version is set to 4.
4. Refuse when 16 tracks are already used or the app is read-only.

On project load and before WAV/stem export, `clips.prepare(project)` renders any missing buffers; export awaits it, and the editor refreshes when done. If the module is unavailable, show a notice and leave the clips silent.

### A9. Synth Studio integration points

- `synth-studio-workspace.js`:
  - MODES/MODE_NAME (`:3-4`): add `'rb338'` → `'rb338'`.
  - Create `<section class="ss-rb338-workbench" data-rb338-panel hidden>` inserted **into `root`** before `.ss-notice` (as the master panel is; not inside `[data-editor-host]`, which sits inside the hidden `.ss-workspace`).
  - `render()` (`:51-71`) toggles it, and on change calls `options.onRb338?.(active)` (like `onMix`, `:65-68`).
  - Return it as `rb338` (`:115-120`).
- CSS: `.ss-app[data-mode=rb338] :is(.ss-workspace,.ss-transport,.ss-master-panel,.ss-midi-panel){display:none!important}`; the workbench gets `flex:1; overflow:auto`. Same pattern as `[data-mode=mix]` in `desktop-app-synth-studio-modern.css:189-190`.
- `synth-studio.js`:
  - Create `rb338 = SynthStudioRB338.create(workspace.rb338, {tr, esc, signal, notice, blocked, readonly, confirm, beforePlay, sendToArrange})` after `workspace` (`:49`).
  - `onRb338` → `rb338.activate(active)` (lazy init inside the click gesture).
  - Pass `audio = SynthStudioAudio.create({audioBuffer})` (`:16`).
  - `loadProject` (`:74`) and `exportFile` (`:366-384`) call `clips.prepare`.
  - `stop()` and `play()` stop RB.
  - Dispose closes the RB host.
- Registration:
  - `module-loader.js:318-319`: add the stylesheet `/css/desktop-app-synth-studio-rb338.css`, and insert `synth-studio-rb338-model.js`, `-host.js` and `synth-studio-rb338.js` before `synth-studio.js`. The worklet and worker are loaded by URL, not registered.
  - Then `npm run build:ui` (module-loader is bundled into `ui/js/desktop/bundles/main.bundle.js`).
  - Update `scripts/test-synth-studio-registration.mjs:29-44` and the Go test's `scripts` slice and stylesheet link (`ui/desktop_synth_studio_browser_test.go:22-25`).

### A10. Licensing and notices

- `licences/THIRD_PARTY_NOTICES.md`: a new section after Strudel (`:16-28`), "Synth Studio — OpenRB-338 (RB-338 mode)". It covers:
  - the engine crates (orb-engine, rbs, rbm), GPL-3.0-only, Copyright (C) 2026 the OpenRB-338 authors, upstream https://github.com/pingw33n/OpenRB-338 at commit 0bebfcd…, compiled unchanged to WebAssembly;
  - AuraGo's RB-338 module (`tools/rb338-wasm`, the five JS files, the build script), added 2026-11-xx under GPL-3.0-only;
  - the browser combination containing it follows GPL terms; MIT sources keep their licence; nothing is linked into the Go binaries;
  - the full corresponding source in `ui/js/vendor/rb338/sources.zip`, plus `LICENSE.txt`, `LICENSES.md` and `manifest.json`;
  - **`Standard.rbm`**: Propellerhead's ReBirth 2.01 material (46 drum samples plus 58 panel JPEGs and the splash, recrypted standard mod, SHA-256 6355b5f1…), shipped by the user's decision of 2026-11-20 with provenance from `OpenRB assets/README.md`. The panel images are not displayed.
- `licences/THIRD_PARTY_LIBRARIES.md`: a paragraph after the Strudel one (`:9-14`).
- `licences/UEBERSICHT.md`: extend the Synth Studio row (`:153`), or add a row "Synth Studio RB-338 | OpenRB-338 (Rust→WebAssembly), Standard.rbm (ReBirth-2.01-Samples, Propellerhead) | GPL-3.0-only; Samples: Propellerhead-Material, auf Nutzerentscheid mitgeliefert".
- In the UI, the RB toolbar footer reads "Engine: OpenRB-338 · GPL-3.0 · Source", linking `LICENSES.md` and downloading `sources.zip` (like `synth-studio-strudel.js:26`).

### A11. i18n keys

All in `ui/lang/desktop/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json` with the `synthStudio.` prefix; the registration test enforces identical key sets (`test-synth-studio-registration.mjs:66-74`).

Device silkscreen (TUNE, CUTOFF, BD, SD, F.BACK…) and brand names stay fixed English like the hardware; their aria-labels are composed from device + silkscreen. Proposed keys:

| Group | Keys |
|---|---|
| Mode and loading | `mode_rb338`, `rb_loading`, `rb_load_failed`, `rb_audio_blocked`, `rb_crashed` |
| Song files | `rb_new`, `rb_open`, `rb_save`, `rb_export_wav`, `rb_send_arrange` |
| Options | `rb_vintage`, `rb_keys`, `rb_keys_synth`, `rb_keys_patterns`, `rb_bars` |
| Pattern menu | `rb_pattern_menu`, `rb_cut`, `rb_copy`, `rb_paste`, `rb_duplicate`, `rb_undo` |
| Transport labels | `rb_play`, `rb_stop`, `rb_rewind`, `rb_forward`, `rb_record`, `rb_pattern_mode`, `rb_song_mode`, `rb_loop`, `rb_tempo`, `rb_shuffle`, `rb_bar`, `rb_loop_start`, `rb_loop_length`, `rb_row_on`, `rb_bank` |
| Errors | `rb_err_not_song`, `rb_err_convert`, `rb_err_memory` (the original's alert text), `rb_err_export` |
| Document state | `rb_replace_confirm`, `rb_unsaved`, `rb_autosaved`, `rb_mod_missing` ("{mod}" plays with the standard kit) |
| Status lines | `rb_dup_done`, `rb_paste_done`, `rb_no_empty`, `rb_clip_kind`, `rb_clip_empty` |
| Send to Arrange | `rb_sent`, `rb_tempo_match`, `audio_clip`, `rb_clip_edit`, `rb_clip_rendering`, `rb_clip_failed` |
| Licence | `rb_license`, `rb_source` |

---

## B. Implementation plan

Work in a git worktree, copying the real `node_modules` (see the worktree memory). Before editing any existing function, run GitNexus `impact` from the main checkout (worktrees cannot be indexed): `validate`, `upgrade`, `normalizedClip`, `normalizeProject`, `play`, `renderWav`, `renderStems`, `createTrackBus`, `updateTrackBus`, workspace `create`. Run `detect_changes` before each commit. Commit after each task.

**T1 — Wrapper crate: session, wire, snapshot, ABI.**
- Files: `tools/rb338-wasm/{Cargo.toml, rust-toolchain.toml, .gitignore, README.md, LICENSE, src/lib.rs, src/abi.rs, src/kit.rs, src/session.rs, src/wire.rs, src/snapshot.rs}`, `tests/engine.rs`, `tests/data/909_bass.rbs` and `303_slide.rbs`, copied from `C:\disposable\openrb\src\tests\songs\diag\songs\` with a `tests/data/README.md` holding their manifest `wav_sha256`:
  - 909_bass `62b37761b09a6cc439a2dd8432a78a0c3444dc33e196138c0b8792086878c80c`;
  - 303_slide `558cc7b9d788e07e2784ade269401051271ff8026a966fc89c9bc071a203d59a` (both 176400 frames, idle 9088).
- Implements A2: create, new, load, push, apply, render, snapshot, startup, defaults, patterns.
- Tests (kit = `rbm::standard::EMBEDDED`, tests only):
  - a `[41,4,79,p,0,1,1]` step message changes the 909 pattern byte (via `orb_patterns`);
  - after `[35,1,1]` the rendered output of 909_bass is non-silent and `play_step[3] < 16`;
  - snapshot offsets are pinned by a `const` assert.
- Verify: `cargo +1.99.1 test --release --manifest-path tools/rb338-wasm/Cargo.toml` and `cargo +1.99.1 build --release --target wasm32-unknown-unknown --manifest-path tools/rb338-wasm/Cargo.toml` (commit `Cargo.lock`; use `--locked` from then on).

**T2 — Save (compose), song info, export.**
- Files: `src/compose.rs`, `src/export.rs`, ABI additions.
- Tests:
  - `orb_export_wav(909_bass, song)` and `(303_slide)` have SHA-256 equal to the manifest values above (the same check as `orb-engine/tests/corpus_golden.rs:68-69`);
  - load → save → parse → `convert` equals `convert` of the original 4.2 file;
  - new → save parses.
- Verify: the same `cargo +1.99.1 test` command.

**T3 — Build script, artefacts, serving, notices, wasm-in-node test.**
- Files: `scripts/build-rb338-vendor.mjs`; `ui/js/vendor/rb338/*` (generated, committed); `assets/web-assets.json` (`.rbm`); `.gitattributes`; `licences/THIRD_PARTY_NOTICES.md`, `THIRD_PARTY_LIBRARIES.md`, `UEBERSICHT.md`; `scripts/test-synth-studio-rb338.mjs` (wasm part).
- The node test:
  - instantiates `rb338-engine.wasm` with no imports and `Standard.rbm`;
  - asserts `orb_export_wav(909_bass)` SHA-256 equals the manifest, which **proves wasm equals native, bit for bit**;
  - checks that `orb_render` after play is non-silent;
  - prints the ms per 1024-frame call (fails if over 5 ms, warns if over 1.5 ms).
- Check `node -e "require.resolve('fflate')"`; if it fails, add a devDependency.
- Verify: `node scripts/build-rb338-vendor.mjs`, `node scripts/test-synth-studio-rb338.mjs`, `go test ./cmd/assetpack -count=1`, `go test ./ui -run '(?i)(TestShippedWebAssetsLoadNothingExternal)' -count=1` (GOCACHE/GOTMPDIR per the D: memory note).

**T4 — Pure JS model.**
- File: `ui/js/desktop/apps/synth-studio-rb338-model.js` (UMD like `synth-studio-model.js:1-5`). Contents:
  - `DEV`, `PARAM_SUB {1:40,2:40,3:57,4:74,5:84,6:92,7:114,8:102}`, `HEADER_SUB {1:42,2:42,3:59,4:76}`;
  - the `CONTROLS` table (id, dev, ctrl, kind, silkscreen, snapshot offset), `ROW_LAYOUT`;
  - `encodeGlobal`, `encodeDevice`, `batch`, `decodeSnapshot`;
  - transport helpers (ports of `model.rs:77-148`), `isEmptyPattern`, `emptiedPattern`, `nextEmptyPattern`;
  - `tr909StepValue`, `repeatExtraSteps`, `meterState`, `headerMeterSegments`, `grLit`, `switchPosition`;
  - `keyAction(code, {shift, mode, focusRow, playing})`.
- Add a model section to `scripts/test-synth-studio-rb338.mjs`:
  - encodings: `[35,4,0,2,34,224]` for tempo 140000, and `[41,0,147,2,1,100]`;
  - stop-stop-stop reaches 0; loop start keeps the length;
  - 909 values: 0→1→2→0, Ctrl+Alt gives 3, accent row 0↔2;
  - `nextEmptyPattern` wraps.
- Add the file to `package.json:12` `test:synth-studio`.
- Verify: `npm run test:synth-studio`.

**T5 — Host, worklet, worker.**
- Files: `synth-studio-rb338-host.js`, `-worklet.js`, `-worker.js`.
- Shape:
  - `SynthStudioRB338Host = { assets(), live({onSnapshot,onCrash}) → {context, send(buf,seq), load(bytes), newSong(), save(), startup(), resume(), suspend(), close()}, renderer() → {render({song,mode,bars}) → {wav,frames}, close()}, clips: {prepare(project,onReady), buffer(clip)} }`.
  - `assets()` is a module-level Promise cache: fetch the wasm and rbm with `versionedURL`, then `WebAssembly.compile`.
  - A crash leads to a re-init from the last autosave.
- Verify: covered by T12's smoke. Optionally a throwaway harness page in the disposable folder.

**T6 — Sixth tab, wiring, i18n, registration.**
- Files: `synth-studio-workspace.js`, `synth-studio.js`, `desktop-app-synth-studio-modern.css` (mode rules) or the new rb338 CSS, `ui/lang/desktop/*.json` (16 locales), `module-loader.js`, `ui/js/desktop/bundles/*` (rebuilt), `scripts/test-synth-studio-registration.mjs`, `ui/desktop_synth_studio_browser_test.go` (scripts and stylesheet only).
- Verify: `npm run build:ui && npm run check:ui`, `node scripts/test-synth-studio-registration.mjs`.

**T7 — Panel part 1: rack DOM, skin, follow.**
- Files: `synth-studio-rb338.js`, `ui/css/desktop-app-synth-studio-rb338.css`.
- All sections and controls from A6, with knob and fader inputs, the snapshot follow using the seq adopt rule, meters, step lights, displays, and the responsive grid.

**T8 — Panel part 2: behaviour and keyboard.**
- File: `synth-studio-rb338.js`.
- All press, arrow, step-paint and 909 rules, mixer radios, repeat and wind buttons, the row focus, and the A6 key map.

**T9 — Song document.**
- File: `synth-studio-rb338.js`.
- New, Open (picker and drop), Save (download), the modified flag with confirm, the IndexedDB autosave and restore, prefs, Edit pattern commands with deferred undo, Vintage, the logo startup sound, WAV export, and the licence and source links.

**T10 — Audio clips in Synth Studio.**
- Files: `synth-studio-model.js` (v4, `'audio'`, `clip.audio`), `synth-studio-audio.js` (live and offline scheduling, resolver), `synth-studio-voices.js` (neutral bus for audio tracks), `synth-studio-export.js` (stems), `synth-studio-midi.js` (skip), `synth-studio-editor.js` (draw and select), inspector guard in `synth-studio.js`.
- Add a model section to `scripts/test-synth-studio-rb338.mjs`: v4 accepts a valid audio clip, rejects notes on an audio clip, and rejects `instrument:'audio'` in v3.
- Verify: `npm run test:synth-studio`, `node scripts/test-synth-studio-storage.mjs`.

**T11 — Send to Arrange and the RB clip cache.**
- Files: `synth-studio-rb338.js` (button, tempo match), `synth-studio-rb338-host.js` (`clips`), `synth-studio.js` (insert track and clip, prepare on load and export, Edit in RB-338).

**T12 — One browser smoke and final checks.**
- In `TestDesktopSynthStudioBrowser`, add one block:
  1. click `[data-work-tab=rb338]`;
  2. wait for `.ss-rb338-workbench [data-rb-ready]`; this needs only the Worker and wasm, since a headless AudioContext may stay suspended;
  3. check the rack (14 303 knob and switch inputs, 27 808 controls, 28 909 knobs, 4 mixer faders);
  4. click 909 BD step 1;
  5. click Send to Arrange;
  6. wait until the project has a track with `instrument==='audio'` and `clips[0].audio.kind==='rb338'`, and the cached buffer has peak > 0;
  7. `fixtureErrors.length===0`.
- Verify:
  - `$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./ui -run '(?i)(TestDesktopSynthStudioBrowser|TestShippedWebAssetsLoadNothingExternal)' -count=1 -timeout 20m -v`;
  - `npm run test:synth-studio`;
  - `node scripts/test-synth-studio-registration.mjs`;
  - `cargo +1.99.1 test --release --manifest-path tools/rb338-wasm/Cargo.toml`;
  - GitNexus `detect_changes --scope all`.

---

## C. Risks and open questions

1. **Worklet peak load.** Exact OpenRB timing renders 1024 frames in one quantum every 8th quantum. If T3 measures over about 1.5 ms per call, there are two fallbacks:
   - `latencyHint:'playback'`, which adds latency;
   - 128-frame engine calls. These are output-identical except in fault edge cases, because `Engine::render` emulates position-dependent stack words (`frame/mod.rs:1461-1462`), and they shorten command latency. **This needs your OK, since it is a tiny deviation from "exact".**
2. **Wasm bit-exactness is unproven upstream** (OpenRB task-169 is still "To Do"). The T3 node test checks it against the corpus hashes; a mismatch blocks the feature.
3. **Git dependency at build time.** Cargo clones the whole OpenRB repo, and the rev must stay reachable upstream. `sources.zip` (cargo vendor) is the fallback for rebuilding without GitHub.
4. **`Standard.rbm` redistribution.** It is Propellerhead material (samples plus panel art) shipped in every AuraGo release (2.5 MB). This is your decision, recorded in the notices; it remains a legal risk.
5. **Project v4 compatibility.** Projects with RB clips will not open in older AuraGo builds. Their playback needs the RB module, since clips are re-rendered.
6. **GPL scope.** The panel ports OpenRB's model and link logic, so it is labelled GPL and the Synth Studio browser combination follows GPL terms (Strudel precedent). An MIT panel would require a clean-room rewrite.
7. **Open questions for you:**
   - Should device silkscreen labels stay English (proposed)?
   - Default pattern-mode export length of 4 bars, and should "Send to Arrange" offer a seamless pre-roll (render two cycles, keep the second) so delay tails loop cleanly?
   - Is local picker plus download enough for `.rbs`, or should Save also write to Desktop Documents via `/api/desktop/upload`?

---

## D. Decisions on the open questions (main session, 2026-11-20)

User decisions (2026-11-20): original OpenRB-338 engine as WebAssembly (GPL-3.0 module), `Standard.rbm` shipped,
sixth mode with WAV export and send-to-Arrange.

Defaults chosen by the main session for C7 and C1:

1. Device silkscreen labels (TUNE, CUTOFF, BD, SD, ...) stay English like the hardware; aria-labels are localised.
2. Pattern-mode export length defaults to 4 bars (choice 1, 2, 4, 8, 16). **WAV export** stays exactly OpenRB's
   path (no pre-roll). **Send to Arrange** in pattern mode renders one pre-roll cycle and keeps the second cycle, so
   delay tails loop seamlessly; in song mode it uses the loop range as exported.
3. `.rbs` open/save uses the local file picker and a download; no Desktop Documents integration in this round.
4. C1: keep 1024-frame engine calls. Only if the T3 node measurement exceeds 1.5 ms per call does the main session
   ask the user about 128-frame calls.
5. Execution: tasks run in the worktree `.claude/worktrees/rb338` on branch `claude/synth-studio-rb338`.
   T1+T2, T4 and T10 are independent and may run in parallel (disjoint files); T10's node tests go to their own
   file `scripts/test-synth-studio-audio-clips.mjs`. Implementers do not commit; the main session commits per batch.
