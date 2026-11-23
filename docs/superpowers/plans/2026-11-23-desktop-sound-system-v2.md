# Desktop Sound System v2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the Web Audio layer synthesizer and all 16 desktop sound themes with a deterministic sample-level synthesis core, loudness-managed roles, 26 events (7 new), Worker rendering, and Node render/gate tools, as approved in `docs/superpowers/specs/2026-11-23-desktop-sound-system-design.md`.
**Architecture:** A plain-JavaScript kernel (`dsp-kernel.js`), instrument models (`instruments.js`) and an engine (`sound-engine.js`) render every sound into `Float32Array`s. The same bundle runs in a Web Worker (via `render-host.js`), on the main thread as a fallback, and in Node for measurement and listening. `sound-runtime.js` stays the only main-bundle part. It adds the Worker client, variants, the new categories, a safety limiter, the start sound and touch feedback.
**Tech Stack:** Vanilla JS (ES2020, attaches to `self`), Web Audio `AudioBuffer` playback, Web Workers, Node 22 (`node:vm`, `node:assert`), Go 1.x tests with rod browser smoke, `scripts/build-ui-bundles.js`.

---

## Conventions for every task

- Work in the worktree on the feature branch. Edit only the files listed in the task. Commit only those paths (`git add <paths>`, never `git add -A`); other sessions share the repository.
- **Impact analysis (root CLAUDE.md).** Before editing an existing function, run this from the **main checkout** `C:\Users\Andi\Documents\repo\AuraGo` (worktrees cannot be indexed):
  `node ../../../.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<symbol>" --direction upstream --repo .`
  Report the callers and the risk. If the risk is HIGH or CRITICAL, stop and report to the coordinator. If it is `UNKNOWN`, confirm the callers with Grep before you go on.
- Sample rate is 48000 everywhere. No `Math.random()` in `ui/js/desktop/sound/*` (the runtime may use it for theme-effect rate jitter only).
- Scratch output goes to `C:\disposable\desktop-sounds\render`, never into the repository.
- Do **not** run `npm run build:ui` before Task 13. Bundles are committed build output.

## Resolved plan questions (spec "plan task" items)

a. **CSP allows the Worker.** `internal/server/server.go:2356-2369` (`securityHeadersMiddleware`) sends this header for every path, including `/desktop`:
   `default-src 'self'; script-src 'self' 'unsafe-inline' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https:; font-src 'self' data:; connect-src 'self' blob: ws: wss: …; media-src 'self' data: blob: http: https:; worker-src 'self' blob:; object-src 'none'; form-action 'self'; frame-src 'self' http: https:; frame-ancestors 'none'; base-uri 'self';`
   `worker-src 'self'` explicitly allows `new Worker('/js/desktop/bundles/desktop-sounds.bundle.js')`. `TestSecurityHeadersAllowEmbedsForYouTubeAndDesktopStoreApps` (`internal/server/security_headers_test.go:32`) already pins `worker-src 'self' blob:`. The browser test serves this CSP on its fixture (Task 15).
b. **Notification types.** `showDesktopNotification` (`core/sdk-events-bootstrap.js:496`) receives `payload.type` from the server and from shell callers. The values in use are `"error"` (`flows_bridge_notify.go:89`, `storymaker_agent_facade.go:72`, `shell-chrome-runtime.js:412`, `sdk-events-bootstrap.js:531`) and `"info"` (`person_detect_alerts.go:349`, storymaker, detective). Most shell callers pass no type. Nothing sends `success` or `warning` today. `showDesktopToast` (`:511`) ignores `type`. The mapping accepts `success`, `warning` and the alias `warn`, compared case-insensitively.
c. **Selectors** (all listeners are delegated on `document`, capture phase):
   - Shell switches, event `change`: Settings toggles `.vd-switch > input[type="checkbox"][data-setting-key]` (markup `apps/settings.js:438`); Quick Settings `#vd-quick-settings input[type="checkbox"][data-quick-setting]` and `#vd-quick-settings input[type="checkbox"][data-quick-dnd]` (`core/quick-settings-runtime.js:54,59,63`; panel id set at `:23`).
   - Shell range sliders, event `input`: `input[type="range"].vd-setting-range[data-setting-key]` (`apps/settings.js:429`) and `#vd-quick-settings input[type="range"][data-quick-setting]` (`quick-settings-runtime.js:55`, range 0–100).
   - Dock icon activation, event `click` (keyboard Enter/Space also fires `click` on buttons): `#vd-taskbar-apps .vd-dock-button` (Fruity dock, `core/window-shell-runtime.js:1266,1288`) and `#vd-taskbar-apps .vd-task-button` (standard taskbar, `:1160,1196`).
d. **Boot end.** `init()` in `core/sdk-events-bootstrap.js:968-1032` runs `await restoreDesktopSession();` (`:1019`; `state.sessionRestoring` is reset in its `finally`, `core/session-runtime.js:260`) and then `openInitialDesktopApp();` (`:1020`). The single place for `desktopSound('session.start')` is directly after `:1020`.
e. **CI.** No workflow runs any `scripts/test-*.mjs`. `.github/workflows/web-assets.yml` (`asset-contract`, ubuntu + windows, Node 22) runs `node scripts/build-ui-bundles.js --check`. Add `- run: npm run test:desktop-sounds` right after that step (Task 13).

## File structure

| Path | Action | Responsibility |
| --- | --- | --- |
| `scripts/desktop-sound-bundle-parts.js` | Create | Single list of the bundle's source files, shared by the builder and the Node tools |
| `scripts/desktop-sound-loader.mjs` | Create | Loads the sound sources in bundle order into Node (`self = globalThis`) |
| `ui/js/desktop/sound/dsp-kernel.js` | Create | `self.DesktopSoundDSP`: primitives, effects, meters, master (port of the spike) |
| `ui/js/desktop/sound/instruments.js` | Create | `self.DesktopSoundInstruments`: 15 models with parameter specs |
| `ui/js/desktop/sound/sound-engine.js` | Create | `self.DesktopSoundEngine`: EVENTS/ROLES/FX_ROLES, validation, registry, render pipeline |
| `ui/js/desktop/sound/render-host.js` | Create | Worker message handler (no-op in a window) |
| `ui/js/desktop/sound/theme-crystal.js` | Replace | Reference theme (Task 3) |
| `ui/js/desktop/sound/theme-{wood,analog,workshop,water,electronic-dreams,windows-xp,green-slime,voxel-world,orbit}.js` | Replace | Themes rewritten in the new format (Tasks 9–12) |
| `ui/js/desktop/sound/theme-{amiga,cyberpunk,black-gloss,crazy,lollipop,organic}.js` | Create | Split from `theme-desktop.js` (Tasks 9–11) |
| `ui/js/desktop/sound/synth-core.js`, `theme-desktop.js` | Delete | Old synthesizer and profile themes (Task 13) |
| `scripts/test-desktop-sound-dsp.mjs` | Create | Kernel unit tests |
| `scripts/test-desktop-sound-instruments.mjs` | Create | Instrument contract tests |
| `scripts/test-desktop-sounds.mjs` | Create | Measured gates of spec §12 |
| `scripts/render-desktop-sounds.mjs` | Create | Authoring tool: WAVs, contact sheets, level table |
| `package.json` | Modify | `test:desktop-sounds`, `render:desktop-sounds` scripts |
| `scripts/build-ui-bundles.js` | Modify :1-5, :166-182 | Import the shared part list |
| `.github/workflows/web-assets.yml` | Modify :27 | Run `npm run test:desktop-sounds` |
| `ui/js/desktop/core/sound-runtime.js` | Modify (whole file) | Worker client, variants, categories, limiter, start sound, feedback, new API |
| `ui/js/desktop/core/sdk-events-bootstrap.js` | Modify :496-509, :1019-1021 | Notification mapping; start-sound request |
| `internal/desktop/types.go` | Modify :288-291 | `sound.interface`, `sound.startup` |
| `internal/desktop/service_test.go` | Modify :1284-1292 | Defaults and validation of the new keys |
| `ui/js/desktop/core/desktop-foundation.js` | Modify :343-346 | JS defaults |
| `ui/js/desktop/core/settings-catalog-runtime.js` | Modify :13-15, :155-158 | Two new switches; preview category per row |
| `ui/js/desktop/apps/settings.js` | Modify :266-285, :434-449 | Row preview buttons and their wiring |
| `ui/css/desktop-app-settings.css` | Modify after :682 | Row action layout |
| `ui/lang/desktop/*.json` (16) | Modify | Five new keys |
| `ui/js/desktop/bundles/main.bundle.js`, `desktop-sounds.bundle.js` | Regenerate | Build output (Task 13) |
| `ui/desktop_sound_settings_test.go` | Modify | New contract markers |
| `ui/desktop_sound_browser_test.go` | Modify | Re-derived browser checks, Worker/fallback/start sound |
| `ui/desktop_theme_sensory_browser_test.go` | Modify :99-121 | Render gate instead of `DesktopSoundSynth` |
| `documentation/desktop-themes.md` | Modify :52-137, :448, :504 | New sound documentation |
| `ui/js/desktop/apps/AGENTS.md` | Modify :2400-2423 | Sound contract |

---

## Stage 1 — Core (strictly sequential)

### Task 1: Part list, Node loader, DSP kernel with unit tests

**Files:**
- Create: `scripts/desktop-sound-bundle-parts.js`, `scripts/desktop-sound-loader.mjs`, `ui/js/desktop/sound/dsp-kernel.js`
- Test: `scripts/test-desktop-sound-dsp.mjs`

- [ ] Create `scripts/desktop-sound-bundle-parts.js`:

```js
// Source files of desktop-sounds.bundle.js in load order. scripts/build-ui-bundles.js concatenates them and
// scripts/desktop-sound-loader.mjs loads the same list into Node, so the bundle and the tests cannot drift apart.
export const desktopSoundCoreParts = [
  'ui/js/desktop/sound/dsp-kernel.js',
  'ui/js/desktop/sound/instruments.js',
  'ui/js/desktop/sound/sound-engine.js'
];
// Theme id -> file, in the runtime's THEME_IDS order.
export const desktopSoundThemeParts = {
  crystal: 'ui/js/desktop/sound/theme-crystal.js',
  wood: 'ui/js/desktop/sound/theme-wood.js',
  analog: 'ui/js/desktop/sound/theme-analog.js',
  workshop: 'ui/js/desktop/sound/theme-workshop.js',
  water: 'ui/js/desktop/sound/theme-water.js',
  amiga: 'ui/js/desktop/sound/theme-amiga.js',
  cyberpunk: 'ui/js/desktop/sound/theme-cyberpunk.js',
  black_gloss: 'ui/js/desktop/sound/theme-black-gloss.js',
  crazy: 'ui/js/desktop/sound/theme-crazy.js',
  lollipop: 'ui/js/desktop/sound/theme-lollipop.js',
  organic: 'ui/js/desktop/sound/theme-organic.js',
  electronic_dreams: 'ui/js/desktop/sound/theme-electronic-dreams.js',
  windows_xp: 'ui/js/desktop/sound/theme-windows-xp.js',
  green_slime: 'ui/js/desktop/sound/theme-green-slime.js',
  voxel_world: 'ui/js/desktop/sound/theme-voxel-world.js',
  orbit: 'ui/js/desktop/sound/theme-orbit.js'
};
export const desktopSoundHostPart = 'ui/js/desktop/sound/render-host.js';
export const desktopSoundParts = [...desktopSoundCoreParts, ...Object.values(desktopSoundThemeParts), desktopSoundHostPart];
```

- [ ] Create `scripts/desktop-sound-loader.mjs`. It runs the files in the Node realm, so typed arrays are ordinary Node objects. It skips files that do not exist yet; the full gate passes `requireAll`.

```js
// Loads the desktop sound sources in bundle order into this Node process, exactly as the browser concatenates them:
// every file attaches to `self`, which is globalThis here. Runs once per process (the files define globals).
import { existsSync, readFileSync } from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';
import { desktopSoundCoreParts, desktopSoundThemeParts, desktopSoundHostPart } from './desktop-sound-bundle-parts.js';

export const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
let loaded = null;

export function loadDesktopSounds({ themes = 'all', strict = true, requireAll = false } = {}) {
  if (loaded) throw new Error('loadDesktopSounds runs once per process');
  globalThis.self = globalThis;
  globalThis.DesktopSoundEngineStrict = strict; // invalid themes throw instead of being skipped
  const missing = [];
  const run = rel => {
    const file = path.join(repoRoot, rel);
    if (!existsSync(file)) { missing.push(rel); return false; }
    vm.runInThisContext(readFileSync(file, 'utf8').replace(/^\uFEFF/, ''), { filename: file });
    return true;
  };
  desktopSoundCoreParts.forEach(run);
  const ids = themes === 'all' ? Object.keys(desktopSoundThemeParts) : themes === 'none' ? [] : themes;
  for (const id of ids) {
    if (!desktopSoundThemeParts[id]) throw new Error('unknown sound theme ' + id);
    run(desktopSoundThemeParts[id]);
  }
  run(desktopSoundHostPart);
  if (requireAll && missing.length) throw new Error('missing sound sources: ' + missing.join(', '));
  loaded = { DSP: globalThis.DesktopSoundDSP, Instruments: globalThis.DesktopSoundInstruments, Engine: globalThis.DesktopSoundEngine, missing };
  return loaded;
}
```

- [ ] Write the failing kernel test `scripts/test-desktop-sound-dsp.mjs`:

```js
import assert from 'node:assert/strict';
import { loadDesktopSounds } from './desktop-sound-loader.mjs';
const { DSP: D } = loadDesktopSounds({ themes: 'none' });
const SR = D.SR;
const sine = (f, sec, amp, phase = 0) => Float32Array.from({ length: Math.round(sec * SR) }, (_, i) => amp * Math.sin(2 * Math.PI * f * i / SR + phase));
const rms = a => Math.sqrt(a.reduce((s, x) => s + x * x, 0) / a.length);
const peakOf = a => a.reduce((m, x) => Math.max(m, Math.abs(x)), 0);
const cents = (f, ref) => 1200 * Math.log2(f / ref);
function pitch(buf, expect) { // autocorrelation around the expected period, parabolic refinement
  const start = Math.round(0.02 * SR), len = Math.round(0.06 * SR);
  const lo = Math.floor(SR / (expect * 1.25)), hi = Math.ceil(SR / (expect / 1.25));
  const r = l => { let s = 0; for (let i = start; i < start + len; i++) s += buf[i] * buf[i + l]; return s; };
  let best = lo; for (let l = lo; l <= hi; l++) if (r(l) > r(best)) best = l;
  const a = r(best - 1), b = r(best), c = r(best + 1);
  return SR / (best + 0.5 * (a - c) / (a - 2 * b + c));
}
// random generator and seeds
{ const a = D.makeRng(7), b = D.makeRng(7); for (let i = 0; i < 1000; i++) { const x = a(); assert.equal(x, b()); assert.ok(x >= 0 && x < 1); } }
assert.equal(D.hashSeed('crystal', 'window.open', '0'), D.hashSeed('crystal', 'window.open', '0'));
assert.notEqual(D.hashSeed('crystal', 'window.open', '0'), D.hashSeed('crystal', 'window.open', '1'));
// envelope starts at zero
{ const a = new Float32Array(480).fill(1); D.applyEnv(a, { attack: 0.004, decay: 0.2 }); assert.equal(a[0], 0); }
// modal partial: pitch, and -60 dB at t60
{ const { a } = D.modal({ freq: 1000, dur: 0.6, t60: 0.5, damp: 0, attack: 0.001, partials: [{ r: 1, g: 1 }] }, D.makeRng(1));
  assert.ok(Math.abs(cents(pitch(a, 1000), 1000)) < 3, 'modal pitch');
  const drop = 20 * Math.log10(peakOf(a.subarray(Math.round(0.49 * SR), Math.round(0.51 * SR))) / peakOf(a.subarray(0, Math.round(0.02 * SR))));
  assert.ok(drop > -61.5 && drop < -58.5, 'modal decay ' + drop); }
// plucked string: tuned within 5 cents, no DC
for (const f of [220, 440, 1000]) { const s = D.pluck({ freq: f, dur: 0.4, t60: 0.8, brightness: 0.5 }, D.makeRng(3));
  assert.ok(Math.abs(cents(pitch(s, f), f)) < 5, 'pluck pitch ' + f);
  assert.ok(Math.abs(s.reduce((x, y) => x + y, 0) / s.length) < 1e-3, 'pluck DC'); }
// filters
{ const x = sine(1000, 0.5, 0.5); D.svf(x, { type: 'lp', cutoff: 1000, q: Math.SQRT1_2 });
  const g = 20 * Math.log10(rms(x.subarray(4800)) / (0.5 / Math.SQRT2)); assert.ok(Math.abs(g + 3.01) < 0.5, 'svf -3 dB ' + g);
  assert.throws(() => D.svf(new Float32Array(8), { type: 'notch?', cutoff: 1000 })); }
{ const lo = sine(55, 1, 0.5), hi = sine(1000, 1, 0.5);
  for (const b of [lo, hi]) { D.highpass(b, 110, 0.5412); D.highpass(b, 110, 1.3066); }
  assert.ok(20 * Math.log10(rms(lo.subarray(SR / 2)) / (0.5 / Math.SQRT2)) < -20, '4th order low cut at 55 Hz');
  assert.ok(Math.abs(20 * Math.log10(rms(hi.subarray(SR / 2)) / (0.5 / Math.SQRT2))) < 0.1, 'low cut passes 1 kHz'); }
// BS.1770: a stereo 1 kHz sine of amplitude 0.1 reads -20 LUFS
{ const s = sine(1000, 1, 0.1); assert.ok(Math.abs(D.loudness({ L: s, R: s, n: s.length }) + 20) < 0.2); }
// true peak finds the inter-sample peak of a 12 kHz sine sampled at +-45 degrees
{ const s = sine(12000, 0.1, 0.5, Math.PI / 4); assert.ok(peakOf(s) < 0.36);
  assert.ok(Math.abs(20 * Math.log10(D.truePeak({ L: s, R: s, n: s.length }) / 0.5)) < 0.25, 'true peak'); }
// master: loudness target, zero ends, length cap
{ const make = () => { const m = D.createMix(1.2); const { a } = D.modal({ freq: 660, dur: 1.2, t60: 1, partials: [{ r: 1, g: 1 }] }, D.makeRng(2)); D.addMono(m, a, 0, 1, 0); return m; };
  const out = D.master(make(), { lufs: -23, ceiling: -3, maxSeconds: 2 });
  assert.ok(out.limited || Math.abs(D.loudness(out.mix) + 23) < 0.05);
  assert.equal(out.mix.L[0], 0); assert.equal(out.mix.R[out.mix.n - 1], 0);
  const capped = D.master(make(), { lufs: -23, ceiling: -3, maxSeconds: 0.2 });
  assert.ok(capped.seconds <= 0.2 + 1e-9 && capped.naturalSeconds > 0.2); }
// reverb and echo extend the sound, deterministically
{ const m = D.createMix(0.1); m.L[10] = m.R[10] = 1;
  const r1 = D.reverb(m, { size: 1, rt60: 0.5, damping: 6000, mix: 0.3 }), r2 = D.reverb(m, { size: 1, rt60: 0.5, damping: 6000, mix: 0.3 });
  assert.ok(r1.n >= m.n + 0.5 * SR); assert.deepEqual(r1.L, r2.L);
  const e = D.echo(m, { time: 0.1, feedback: 0.5, tone: 4000, mix: 0.3 }); assert.ok(e.n > m.n + 0.3 * SR); }
// noise colours are finite and seeded
for (const c of ['white', 'pink', 'brown']) { const a = D.noise(0.1, c, D.makeRng(5)), b = D.noise(0.1, c, D.makeRng(5));
  assert.deepEqual(a, b); assert.ok(a.every(Number.isFinite) && rms(a) > 0.01); }
console.log('desktop sound DSP kernel: ok');
```

- [ ] Run `node scripts/test-desktop-sound-dsp.mjs`. Expected: it fails because `DesktopSoundDSP` is undefined.
- [ ] Create `ui/js/desktop/sound/dsp-kernel.js` by **porting `C:\disposable\desktop-sounds\spike\dsp.js`**, with exactly these changes:
  1. **Wrapper:** `(function (root) { 'use strict'; … root.DesktopSoundDSP = api; if (typeof module === 'object' && module && module.exports) module.exports = api; })(typeof self !== 'undefined' ? self : globalThis);`. Always attach to `self`; the spike attached either to `self` or to `module.exports`, never both.
  2. **Defaults:** replace every `x || default` with `x ?? default`. Today `0` cannot be expressed, for example `rise: 0`, `lowcut: 0`, `depth: 0`, `t60`, `damping`, `rate`, `decay`, `release`, `ratio`.
  3. **Additions:** `hashSeed(...parts)` (FNV-1a 32-bit over `parts.join('|')`, returns uint32). `addStereo(mix, st, at = 0, gain = 1, balance = 0)` adds `{L,R}` at an offset; balance −1..1 attenuates the opposite side linearly. `panMono(buf, gain, pan) → {L, R}` uses the same equal-power law as `addMono`. `echo(mix, { time, feedback, tone, mix, spread })`: stereo feedback delay with L delay `time` and R delay `time·(1+0.25·spread)`, a one-pole low-pass at `tone` inside the loop, output dry + `mix`·wet. Its tail is `time·ceil(ln 0.001 / ln max(feedback, 0.05))`, capped at 2 s. `lowpass(buf, fc, q = 0.7071)` (RBJ). Export `biquad` and `polyblep`.
  4. **`highpass(buf, fc, q = 0.7071)`:** add the `q` argument (the spike fixed Q at 1/√2).
  5. **`noise`:** add `'brown'`: leaky integrator `b = (b + 0.02·w) / 1.02; out = 3.5·b`.
  6. **`osc`:** band-limited `'tri'`. Integrate the PolyBLEP square: `tri = 4·dt·sq + (1 − 2π·5/SR)·tri`. The naive triangle aliases. Add `drift` in cents: per voice a sine LFO at 0.3–0.9 Hz with the rate and phase taken from `rand`. Make the glide exponential in log-frequency: `lf = lf1 + (lf − lf1)·kg`. The spike glided linearly in Hz.
  7. **`modal(o, rand)`:** clamp every partial t60 to ≥ 0.002 s. The initial-condition formula divides by `k` and `k²`, which explode for tiny t60. Add `o.spread` (cents): each partial and its twin get a random detune within ±spread from `rand`. Partials at or above 0.45·SR are still skipped.
  8. **`fm`:** wrap phases modulo 2π. Alias guard: a modulator whose `freq·ratio·(index+1) > 0.45·SR` (Carson bandwidth) has its index scaled down until it fits, or is dropped if `freq·ratio ≥ 0.45·SR`.
  9. **`pluck`:** subtract the mean of the excitation after the pick comb. The DC component otherwise rings in the loop at DC gain ≈ 1 and leaves a slow offset tail. Throw if `freq > SR/8`, which keeps the delay line ≥ 6 samples.
  10. **`bubble`:** cap the rising frequency at `4·freq`. Today it rises without bound after t60.
  11. **`svf`:** throw `Error('unknown svf type')` for a type other than `lp`, `bp`, `hp`. Today an unknown type silently becomes a low-pass.
  12. **`reverb`:**
      - (a) Snap each scaled delay length `round(d·size)` up to the next prime. `|1` loses mutual primeness at small sizes, which gives coinciding modes and flutter.
      - (b) Per-line damping after Jot, so all lines decay at the same rate at the damping frequency. `damping` becomes the frequency at which RT60 is halved. For line k with length d_k, set `m = 10^(−3·d_k/(SR·rt60))` and `w = 2π·damping/SR`, and solve for the one-pole coefficient: `a = ((1 − m² cos w) − sqrt((1 − m² cos w)² − (1 − m²)²)) / (1 − m²)`, filter `y = (1 − a)·x + a·y`. The spike used one coefficient for all line lengths.
      - (c) Default `lowcut` 220 stays.
  13. **`loudness`:** remove the dead branch `if (n < win)`. `n` always includes the 400 ms pad.
  14. **`truePeak`:** use 32 taps per phase (Hann window, half-width 16). Evaluate the 3 interpolated phases between i and i+1 whenever `max(|x[i]|, |x[i+1]|) ≥ 0.5·peak`. The spike tested only `|x[i]|`, so an inter-sample peak next to a large x[i+1] was skipped, and 12 taps underestimate near-Nyquist peaks.
  15. **`master(mix, o)`:**
      - 4th-order low cut: `highpass(·, lowcut, 0.5412)` then `highpass(·, lowcut, 1.3066)`, default `lowcut` 110. 2nd order removes only about 6 dB at 90 Hz.
      - After the −66 dB tail trim, record `naturalSeconds`. If the length exceeds `o.maxSeconds`, cut it to `maxSeconds` and put the 15 ms fade-out at the new end.
      - Return `{ mix, lufs, peakDb, seconds, naturalSeconds, limited }`. `limited` is true when the true-peak gain is below the loudness gain.
      - Silent input (`peak < 1e-9`) returns `seconds: 0` and a zero-length mix.
  16. **Removal:** `encodeWav` moves to `scripts/render-desktop-sounds.mjs` (Task 3). The browser never writes WAV.
  17. Keep `SR, makeRng, midiToHz, dbToGain, gainToDb, clamp, createMix, addMono, applyEnv, modal, fm, osc, pluck, bubble, noise, svf, shelf, reverb, chorus, saturate, loudness, truePeak` with the spike's semantics. Validate `chorus` with `depth < delay`; the engine enforces this.
- [ ] Run `node scripts/test-desktop-sound-dsp.mjs`. Expected output: `desktop sound DSP kernel: ok`.
- [ ] Commit: `git add scripts/desktop-sound-bundle-parts.js scripts/desktop-sound-loader.mjs scripts/test-desktop-sound-dsp.mjs ui/js/desktop/sound/dsp-kernel.js` then `git commit -m "feat(desktop-sounds): sample-level DSP kernel with Node loader and unit tests"`.

### Task 2: Instrument models

**Files:**
- Create: `ui/js/desktop/sound/instruments.js`
- Test: `scripts/test-desktop-sound-instruments.mjs`

**Contract.** `self.DesktopSoundInstruments = { MODELS, resolveParams(model, params), render(model, params, note, rand) }`, with the same wrapper as the kernel.
- `MODELS[name]` has these members:
  - `params`: each parameter spec is `[min, max, default]`, `{ values: [...], def }`, `{ pair: [min, max], def: [lo, hi] }`, or `{ list: [min, max], max: n, def: [] }`.
  - `presets` (optional): `{ <param>: { <value>: { …defaults } } }`.
  - `pitched(p)`, `minFreq(p)`, `duration(p)`, `render(p, note, rand)`.
- `resolveParams` merges in this order: spec defaults, then the preset chosen by the enum parameter (`kind`, `material`, `grain`), then explicit values.
- `note = { freq, dur, gain, pan, at, detune }`. `render` returns `{ L, R }` of length `ceil(note.dur·SR)`, with gain and pan already applied. The first sample is 0 and the attack is raised-cosine, never a single-sample step. The engine places the buffer at `note.at`.
- `note.detune` (cents, 4 for variants and 0 otherwise): models with partials (glass, bar, bell, metal, pad) pass it as the per-partial `spread`. The other pitched models apply one random offset within ±detune/2.
- Integers (`unison`, `count`, `partials`) are rounded in `render`.
- Every model uses `rand` for all randomness.

| Model | Recipe (kernel primitives) | Parameters `[min, max, default]` | Stereo out · duration · pitched / minFreq |
| --- | --- | --- | --- |
| `glass` | `modal` with beat twin. Partials r [1, 2.0, 2.756, 5.404, 8.933], g [1, .22, .16, .07, .025] | t60 [0.1,3,0.9], damp [0,2,0.75], beat [0,3,0.55] Hz, attack [0.001,0.05,0.004], tilt [−1,2,0], width [0,1,0.6] | main at pan−0.3·width (×0.62), twin at pan+0.35·width (×0.38) · t60·1.3+attack · yes / 130 |
| `bar` | `modal` + mallet noise. Mallet: white 12 ms → svf bp at min(8000, f·(2+4·hardness)), q 1.2, decay 12 ms, level mallet·(0.5+hardness). Upper partials × (0.6+0.8·hardness). body: partial r 0.5, t60 0.2, gain body·0.3, skipped if f/2 < 130 | kind {marimba, xylophone, glockenspiel, kalimba, woodblock}. Presets: marimba r[1,3.93,9.2] g[1,.24,.08] t60 .5 damp 1.25; xylophone r[1,3,6.5] g[1,.3,.1] t60 .3 damp 1.4; glockenspiel r[1,2.76,5.4,8.93] g[1,.45,.22,.1] t60 1.2 damp .5; kalimba r[1,5.4,14] g[1,.16,.05] t60 .8 damp 1; woodblock r[1,2.32,3.81] g[1,.5,.3] t60 .08 damp .6. t60 [0.03,2.5], damp [0,2.5], hardness [0,1,0.5], attack [0.0005,0.02,0.0015], body [0,1,0.2], mallet [0,1,0.3] | mono panned · t60·1.2+0.02 · yes / 130 |
| `bell` | `fm` carrier ratio 1 + preset modulators (index × `index`), amplitude `applyEnv` decay t60 | kind {bell, celesta, tine}. Presets: bell mods [{3.5, 2.5, t60 1.2}, {1.41, 0.5, 0.6}] t60 1.4; celesta [{1, 1.2, .35}, {4, .4, .05}] t60 .6; tine [{1, 1.1, .5, floor .15}, {14, .5, .06}] t60 .55. t60 [0.1,3], index [0,4,1], attack [0.001,0.03,0.002], spread [0,1,0] (2nd voice +3 cents on the opposite side) | mono or 2-voice · t60+attack+0.02 · yes / 130 |
| `string` | `pluck` + parallel svf bp 280 Hz q 1.4 at body·0.5, env attack 1 ms, decay t60 | kind {nylon, harp, muted}. Presets: nylon brightness .35 pick .22 t60 .8; harp .55/.3/1.2; muted .2/.12/.2. t60 [0.05,3], brightness [0.02,0.98], pick [0.05,0.5], body [0,1,0.2] | mono panned · t60·1.1 · yes / 130 (render throws above 4000 Hz) |
| `analog` | two independent unison banks A, B (`osc`), sub `tri` at f/2 mixed at sub·0.35. svf lp with cutoff(t) = f·(cutoff + env·e^(−t/envDecay)), q. `saturate(drive)` if drive > 0. `applyEnv` (gate = attack+hold when sustain > 0). Width: L = mid + width·side, R = mid − width·side | wave {saw, pulse, tri} saw, unison [1,7,3], detune [0,50,14] cents, drift [0,20,3], pw [0.05,0.95,0.5], sub [0,1,0.35], cutoff [0.5,20,1.5], env [0,24,7], envDecay [0.005,2,0.055], q [0.5,8,1.6], drive [0,4,1.3], attack [0.001,1,0.004], hold [0,2,0], decay [0.02,3,0.42], sustain [0,1,0], release [0.01,2,0.15], glide [−24,24,0] semitones start offset, glideTime [0.005,1,0.08], width [0,1,0.3] | stereo · sustain>0 ? attack+hold+release·1.05 : attack+hold+decay·1.05 · yes / sub>0 ? 260 : 130 |
| `chip` | own phase loop using `polyblep` (pulse) or the band-limited triangle. Pitch steps through [0, …arp] semitones at arpRate. Vibrato starts after vibratoDelay. Quantize to 2^(bits−1) levels, then svf lp 9 kHz, then env | wave {pulse, tri} pulse, duty [0.125,0.75,0.5], arp {list [−24,24], max 4, def []}, arpRate [8,60,30], vibrato [0,100,0] cents, vibratoRate [2,10,6], vibratoDelay [0,0.5,0.08], bits [4,16,8], attack [0.001,0.02,0.002], hold [0,1,0], decay [0.02,2,0.25], sustain [0,1,0], release [0.01,1,0.08] | mono panned · as analog · yes / 130 |
| `drop` | `count` × `bubble`. Member k starts at k·spread·(0.7+0.6·rand), freq f·2^((2rand−1)·pitchSpread/12) (member 0 exact), pan ±width·rand, gain ×(1−0.12k). thick → `saturate(1+3·thick)`. splash: pink 90 ms, bp 3200→1200 Hz, q 1.2, decay 70 ms, level splash·0.3 | rise [0,2,0.6], t60 [0.02,0.5,0.08], attack [0.0005,0.01,0.003], count [1,8,1], spread [0,0.2,0.04], pitchSpread [0,12,4], splash [0,1,0], thick [0,1,0], width [0,1,0.4] | stereo · (count−1)·spread·1.3+t60·2.5+0.02 · yes / 130 |
| `blip` | phase loop. pitch(t) = f·2^(glide·e^(−t/glideTime)/12) · 2^(boing·1.25·e^(−t/0.15)·sin(2π·boingRate·t)/12). Out = sin φ + harm·(0.5 sin 2φ + 0.25 sin 3φ), then env | glide [−24,24,0], glideTime [0.002,1,0.05], boing [0,1,0], boingRate [4,16,10], harm [0,1,0.1], attack [0.001,0.05,0.004], hold [0,1,0], decay [0.02,2,0.3] | mono panned · attack+hold+decay·1.05 · yes / 130 |
| `pad` | k = 1..partials, three sines per partial at k·f·2^(c/1200), c ∈ {−detune, 0, +detune}, random phase, gain 1/k. Copies alternate L/R by width. svf lp cutoff. Shimmer = amplitude LFO 0.5–1.5 Hz, depth 0.3·shimmer. Env sustain 1, gate attack+hold | partials [1,8,4], detune [0,30,8], attack [0.05,1.5,0.35], hold [0,2,0.2], release [0.05,2.5,0.8], cutoff [400,8000,2500], width [0,1,0.7], shimmer [0,1,0] | stereo · attack+hold+release·1.05 · yes / 130 |
| `metal` | `modal` with beat twin + strike noise (white 6 ms, highpass 2 kHz, level 0.4·hardness). tilt = 1 − hardness | kind {plate, pipe, ping}. Presets: plate r[1,1.59,2.14,2.30,2.65,2.92,3.16,3.50] g[1,.8,.6,.55,.45,.4,.3,.25]; pipe r[1,2.76,5.40,8.93,13.34] g[1,.6,.35,.2,.1]; ping r[1,2.32,4.25,6.63] g[1,.5,.3,.15]. t60 [0.05,4,0.8], damp [0,2,0.4], beat [0,6,1.2], hardness [0,1,0.5], attack [0.0005,0.02,0.001], width [0,1,0.6] | like glass · t60·1.2 · yes / 130 |
| `click` | white burst of `length` (raised-cosine attack ≥ attack, exp decay) → svf bp at tone, q, mixed at `noise`. Single-partial `modal` resonance at tone, t60 ring, ×(1−0.5·noise). double: repeat after `double` s at 0.5 level, tone ×1.06. copper adds a 2nd resonance ×1.48. tuned=1 → tone = note.freq | material {felt, relay, plastic, glass, copper, stone} plastic. Presets (tone, ring, noise, q, length): felt 1200/.012/.8/.8/.006; relay 2600/.02/.6/2.5/.003 + double .012; plastic 1800/.025/.5/2/.004; glass 5200/.04/.3/4/.002; copper 3400/.05/.4/3/.003; stone 1500/.008/.9/1/.005. tone [200,9000], ring [0.004,0.2], noise [0,1], q [0.5,10], length [0.001,0.02], double [0,0.03,0], tuned [0,1,0], attack [0.0005,0.004,0.0008] | mono panned · length+ring·1.2+double+0.01 · tuned=1 / 130 |
| `thud` | sine phase loop, pitch f·2^(drop·e^(−t/dropTime)/12), amplitude decay t60. Noise: pink 40 ms → lowpass noiseCut → highpass 130, decay 30 ms, level noise·0.6 | drop [0,24,7], dropTime [0.005,0.2,0.03], t60 [0.03,1,0.18], noise [0,1,0.4], noiseCut [200,4000,900], attack [0.0005,0.01,0.001] | mono panned · t60·1.2 · yes / 130 |
| `noise` | two noise channels A, B (`noise` color). L = A, R = (1−width)·A + width·B. svf hp at band[0]·2^(sweep·t/dur) and svf lp at band[1]·2^(sweep·t/dur), q. flutter: ×(1 − 0.5·flutter·(1+sin 2π·27t)). Env attack, hold, decay | color {white, pink, brown} pink, band {pair [120,18000], def [300,6000]}, sweep [−4,4,0] octaves, q [0.3,6,0.7], attack [0.001,1.5,0.06], hold [0,2,0], decay [0.01,3,0.2], flutter [0,1,0], width [0,1,0.8] | stereo · attack+hold+decay·1.05 · no / — |
| `squelch` | saw (`osc`, unison 1) ×saw + white ×noise → svf bp with cutoff(t) = to + (from−to)·e^(−t/sweepTime), q → `saturate(drive)` → env | from [100,4000,300], to [100,4000,1200], sweepTime [0.01,0.5,0.08], q [1,12,6], saw [0,1,0.5], noise [0,1,0.5], drive [0,4,1], attack [0.001,0.03,0.003], decay [0.02,1,0.15] | mono panned · attack+decay·1.05 · yes / 80 (fundamental removed by bp and master low cut) |
| `debris` | `count` grains. Onsets spread·(−ln(1−0.95·rand))/3 (front-loaded, first at 0). Level 10^(−3·onset/decay)·(0.5+0.5rand). Grain: wood = single-partial modal (600–1800 Hz)·tone, t60 30 ms + white burst bp; stone = white 2 ms bp (1500–5000 Hz)·tone, q 3 + modal t60 10 ms; earth = pink 6 ms, lowpass (200–700 Hz)·tone, highpass 150. size scales frequency ×(1−0.4·size) and level ×(0.7+0.6·size). Pan ±width·rand | grain {wood, stone, earth} stone, count [2,40,12], spread [0.005,0.4,0.12], decay [0.05,1,0.25], tone [0.5,2,1], size [0,1,0.5], width [0,1,0.6] | stereo · min(0.7, spread+decay+0.05) · no / — |

- [ ] Write the failing test `scripts/test-desktop-sound-instruments.mjs`:

```js
import assert from 'node:assert/strict';
import { loadDesktopSounds } from './desktop-sound-loader.mjs';
const { DSP: D, Instruments: I } = loadDesktopSounds({ themes: 'none' });
const NAMES = ['glass', 'bar', 'bell', 'string', 'analog', 'chip', 'drop', 'blip', 'pad', 'metal', 'click', 'thud', 'noise', 'squelch', 'debris'];
assert.deepEqual(Object.keys(I.MODELS).sort(), NAMES.slice().sort());
const TONAL = ['glass', 'bar', 'bell', 'string', 'analog', 'chip', 'blip'];
function pitch(buf, expect) { /* same autocorrelation helper as test-desktop-sound-dsp.mjs */ }
const fp = o => { let h = 0x811c9dc5; for (const ch of [o.L, o.R]) { const b = new Uint8Array(ch.buffer, ch.byteOffset, ch.byteLength); for (let i = 0; i < b.length; i++) h = Math.imul(h ^ b[i], 0x01000193) >>> 0; } return h; };
for (const name of NAMES) {
  const m = I.MODELS[name];
  for (const [k, s] of Object.entries(m.params)) { // every default lies inside its own range
    if (Array.isArray(s)) assert.ok(s[2] >= s[0] && s[2] <= s[1], `${name}.${k} default`);
    else if (s.values) assert.ok(s.values.includes(s.def), `${name}.${k} default`);
  }
  const p = I.resolveParams(name, {});
  const note = { freq: 440, dur: Math.min(m.duration(p), 2), gain: 1, pan: 0, at: 0, detune: 0 };
  const a = I.render(name, {}, note, D.makeRng(11)), b = I.render(name, {}, note, D.makeRng(11));
  assert.equal(a.L.length, Math.ceil(note.dur * D.SR), `${name} length`);
  assert.ok(a.L.every(Number.isFinite) && a.R.every(Number.isFinite), `${name} finite`);
  assert.ok(Math.abs(a.L[0]) < 1e-4 && Math.abs(a.R[0]) < 1e-4, `${name} starts at zero`);
  assert.ok(a.L.some(x => Math.abs(x) > 1e-3), `${name} not silent`);
  assert.equal(fp(a), fp(b), `${name} deterministic`);
  if (TONAL.includes(name)) { const mono = a.L.map((x, i) => x + a.R[i]); assert.ok(Math.abs(1200 * Math.log2(pitch(mono, 440) / 440)) < 10, `${name} pitch`); }
}
assert.notEqual(fp(I.render('glass', {}, { freq: 880, dur: 0.5, gain: 1, pan: 0, at: 0, detune: 4 }, D.makeRng(1))),
  fp(I.render('glass', {}, { freq: 880, dur: 0.5, gain: 1, pan: 0, at: 0, detune: 0 }, D.makeRng(1))), 'variant detune changes partials');
assert.throws(() => I.render('kazoo', {}, { freq: 440, dur: 0.1, gain: 1, pan: 0, at: 0 }, D.makeRng(1)));
console.log('desktop sound instruments: ok');
```

- [ ] Run `node scripts/test-desktop-sound-instruments.mjs`. Expected: it fails (no instruments file).
- [ ] Implement `ui/js/desktop/sound/instruments.js` to the contract above. Use kernel primitives only; no Web Audio.
- [ ] Run `node scripts/test-desktop-sound-dsp.mjs && node scripts/test-desktop-sound-instruments.mjs`. Expected output: both print `ok`.
- [ ] Render each model once at 440 Hz into `C:\disposable\desktop-sounds\render\models\<model>.wav` with a throw-away script in `C:\disposable` that uses the loader and the WAV writer from Task 3, or postpone this to Task 3. Report the file list to the coordinator for listening.
- [ ] Commit: `git add ui/js/desktop/sound/instruments.js scripts/test-desktop-sound-instruments.mjs` then `git commit -m "feat(desktop-sounds): fifteen instrument models with parameter specs"`.

### Task 3: Engine, render host, Node tools, gate test, crystal reference theme

**Files:**
- Create: `ui/js/desktop/sound/sound-engine.js`, `ui/js/desktop/sound/render-host.js`, `scripts/render-desktop-sounds.mjs`, `scripts/test-desktop-sounds.mjs`
- Replace: `ui/js/desktop/sound/theme-crystal.js`
- Modify: `package.json` (scripts block)

- [ ] Create `sound-engine.js` with the kernel's wrapper. Catalogue, verbatim:

```js
    const THEME_IDS = ['crystal', 'wood', 'analog', 'workshop', 'water', 'amiga', 'cyberpunk', 'black_gloss', 'crazy', 'lollipop', 'organic', 'electronic_dreams', 'windows_xp', 'green_slime', 'voxel_world', 'orbit'];
    const CATEGORIES = ['windows', 'notifications', 'navigation', 'files', 'interface', 'startup'];
    const EVENTS = ['session.start', 'notify.message', 'notify.info', 'notify.success', 'notify.warning', 'notify.error',
        'dialog.open', 'dialog.confirm', 'dialog.cancel', 'window.open', 'window.close', 'window.maximize', 'window.minimize',
        'window.restore', 'window.snap', 'window.deny', 'file.drop', 'file.trash', 'file.delete', 'space.switch',
        'menu.open', 'menu.close', 'ui.select', 'ui.toggle.on', 'ui.toggle.off', 'ui.tick'];
    const role = (category, lufs, maxSeconds, variants) => Object.freeze({ category, lufs, maxSeconds, variants });
    // Loudness target = maximum momentary loudness (LUFS) before the volume setting; maxSeconds includes the tail.
    const ROLES = Object.freeze({
        'session.start': role('startup', -18, 2.8, 1),
        'notify.message': role('notifications', -18, 1.4, 1),
        'notify.info': role('notifications', -19, 1.2, 1),
        'notify.success': role('notifications', -19, 1.2, 1),
        'notify.warning': role('notifications', -18, 1.2, 1),
        'notify.error': role('notifications', -18, 1.2, 1),
        'dialog.open': role('files', -22, 0.6, 1),
        'dialog.confirm': role('files', -21, 0.7, 1),
        'dialog.cancel': role('files', -23, 0.5, 1),
        'window.open': role('windows', -23, 0.6, 3),
        'window.close': role('windows', -24, 0.5, 3),
        'window.maximize': role('windows', -24, 0.6, 3),
        'window.minimize': role('windows', -25, 0.4, 3),
        'window.restore': role('windows', -25, 0.4, 3),
        'window.snap': role('windows', -26, 0.25, 3),
        'window.deny': role('windows', -23, 0.35, 2),
        'file.drop': role('files', -23, 0.5, 3),
        'file.trash': role('files', -23, 0.5, 2),
        'file.delete': role('files', -22, 0.6, 2),
        'space.switch': role('navigation', -25, 0.6, 2),
        'menu.open': role('navigation', -28, 0.2, 3),
        'menu.close': role('navigation', -30, 0.15, 3),
        'ui.select': role('interface', -31, 0.12, 3),
        'ui.toggle.on': role('interface', -31, 0.12, 3),
        'ui.toggle.off': role('interface', -32, 0.12, 3),
        'ui.tick': role('interface', -34, 0.06, 3)
    });
    const fxRole = (theme, category, maxSeconds, names) => Object.freeze({ theme, category, lufs: -23, maxSeconds, names: Object.freeze(names) });
    const FX_ROLES = Object.freeze({
        'window.fracture': fxRole('voxel_world', 'windows', 0.7, ['wood', 'stone', 'earth']),
        'window.impact': fxRole('voxel_world', 'windows', 0.7, ['wood', 'stone', 'earth']),
        'slime.pop': fxRole('green_slime', 'windows', 0.3, ['small', 'medium', 'large']),
        'slime.snap': fxRole('green_slime', 'windows', 0.3, ['small', 'medium', 'large']),
        'orbit.ping': fxRole('orbit', 'navigation', 1.0, ['small', 'medium', 'large'])
    });
```

- [ ] Add the validation rules, verbatim (`D = root.DesktopSoundDSP`, `I = root.DesktopSoundInstruments`):

```js
    const isObj = v => !!v && typeof v === 'object' && !Array.isArray(v);
    const BUS = {
        space: { size: [0.2, 2], rt60: [0.05, 3], damping: [1000, 16000], predelay: [0, 0.08], mix: [0, 0.6], lowcut: [80, 1000] },
        chorus: { rate: [0.1, 5], depth: [0.0005, 0.008], delay: [0.004, 0.03], mix: [0, 0.8] },
        echo: { time: [0.02, 0.8], feedback: [0, 0.85], tone: [500, 12000], mix: [0, 0.7], spread: [0, 1] },
        tone: { low: [-12, 12], high: [-12, 12] }
    };
    const THEME_FIELDS = ['id', 'key', 'space', 'voices', 'events', 'fx'];
    const SOUND_FIELDS = ['notes', 'layers', 'space', 'chorus', 'echo', 'drive', 'tone'];
    const ITEM_FIELDS = { v: null, n: [-36, 36], t: [0, 3], g: [0, 2], pan: [-1, 1], dur: [0.005, 3], detune: [-50, 50] };

    function validateTheme(def) {
        const errors = [];
        const fail = (path, message) => { errors.push(path + ': ' + message); };
        const number = (path, value, range) => {
            if (typeof value !== 'number' || !Number.isFinite(value)) fail(path, 'must be a finite number');
            else if (value < range[0] || value > range[1]) fail(path, value + ' is outside ' + range[0] + '..' + range[1]);
        };
        const busBlock = (path, kind, value) => {
            if (!isObj(value)) return fail(path, 'must be an object');
            Object.keys(value).forEach(k => { if (!BUS[kind][k]) fail(path + '.' + k, 'unknown ' + kind + ' setting'); else number(path + '.' + k, value[k], BUS[kind][k]); });
            if (kind === 'chorus' && value.depth != null && value.delay != null && value.depth >= value.delay) fail(path, 'depth must be smaller than delay');
        };
        const params = (path, modelName, values, skip) => {
            const spec = I.MODELS[modelName].params;
            Object.keys(values).forEach(k => {
                if (skip.includes(k)) return;
                const s = spec[k], p = path + '.' + k, v = values[k];
                if (!s) return fail(p, 'unknown parameter of model ' + modelName);
                if (Array.isArray(s)) return number(p, v, s);
                if (s.values) { if (!s.values.includes(v)) fail(p, 'must be one of ' + s.values.join(', ')); return; }
                if (s.pair) {
                    if (!Array.isArray(v) || v.length !== 2) return fail(p, 'must be [low, high]');
                    number(p + '[0]', v[0], s.pair); number(p + '[1]', v[1], s.pair);
                    if (v[0] >= v[1]) fail(p, 'low must be below high');
                    return;
                }
                if (s.list) {
                    if (!Array.isArray(v) || v.length > s.max) return fail(p, 'must be a list of at most ' + s.max + ' values');
                    v.forEach((x, i) => { if (!Number.isInteger(x)) fail(p + '[' + i + ']', 'must be an integer'); else number(p + '[' + i + ']', x, s.list); });
                }
            });
        };
        if (!isObj(def)) return ['theme: must be an object'];
        if (!THEME_IDS.includes(def.id)) fail('id', 'unknown theme id ' + JSON.stringify(def.id));
        Object.keys(def).forEach(k => { if (!THEME_FIELDS.includes(k)) fail(k, 'unknown theme field'); });
        if (!Number.isInteger(def.key)) fail('key', 'must be an integer MIDI note'); else number('key', def.key, [36, 96]);
        if (def.space == null) fail('space', 'missing'); else busBlock('space', 'space', def.space);
        const voices = isObj(def.voices) ? def.voices : {};
        if (!Object.keys(voices).length) fail('voices', 'must be a non-empty object');
        Object.keys(voices).forEach(name => {
            const voice = voices[name];
            if (!isObj(voice) || !I.MODELS[voice.model]) return fail('voices.' + name, 'unknown model ' + JSON.stringify(voice && voice.model));
            params('voices.' + name, voice.model, voice, ['model']);
        });
        const sound = (path, s, soundRole, isFx) => {
            if (!isObj(s)) return fail(path, 'must be an object');
            Object.keys(s).forEach(k => { if (!SOUND_FIELDS.includes(k)) fail(path + '.' + k, 'unknown sound field'); });
            const notes = s.notes || [], layers = s.layers || [];
            if (!Array.isArray(notes) || !Array.isArray(layers)) return fail(path, 'notes and layers must be arrays');
            if (!notes.length && !layers.length) fail(path, 'needs at least one note or layer');
            [[notes, 'notes', true], [layers, 'layers', false]].forEach(([list, field, asNote]) => list.forEach((item, i) => {
                const p = path + '.' + field + '[' + i + ']';
                if (!isObj(item)) return fail(p, 'must be an object');
                const voice = voices[item.v];
                if (!voice || !I.MODELS[voice.model]) return fail(p, 'unknown voice ' + JSON.stringify(item.v));
                const model = I.MODELS[voice.model];
                Object.keys(ITEM_FIELDS).forEach(k => { if (k !== 'v' && k !== 'n' && item[k] != null) number(p + '.' + k, item[k], ITEM_FIELDS[k]); });
                if (item.t != null && item.t > soundRole.maxSeconds) fail(p + '.t', 'starts after the role maximum of ' + soundRole.maxSeconds + ' s');
                const overrides = {};
                Object.keys(item).forEach(k => { if (!(k in ITEM_FIELDS)) overrides[k] = item[k]; });
                params(p, voice.model, overrides, []);
                const merged = I.resolveParams(voice.model, Object.assign({}, voice, overrides));
                const pitched = model.pitched(merged);
                if (asNote && !pitched) return fail(p, 'voice ' + item.v + ' is unpitched: put it in layers');
                if (!asNote && pitched) return fail(p, 'voice ' + item.v + ' is pitched: put it in notes with n');
                if (asNote) {
                    if (!Number.isInteger(item.n)) return fail(p + '.n', 'must be an integer semitone offset');
                    number(p + '.n', item.n, ITEM_FIELDS.n);
                    const freq = D.midiToHz(def.key + item.n), floor = model.minFreq(merged);
                    if (freq < floor) fail(p + '.n', freq.toFixed(1) + ' Hz is below the ' + floor + ' Hz floor of ' + voice.model);
                    if (freq > 12000) fail(p + '.n', 'above 12 kHz');
                } else if (item.n != null) fail(p + '.n', 'layers take no n');
            }));
            if (s.space != null) { if (isFx) fail(path + '.space', 'effects use no room reverb'); else busBlock(path + '.space', 'space', s.space); }
            ['chorus', 'echo', 'tone'].forEach(k => { if (s[k] != null) busBlock(path + '.' + k, k, s[k]); });
            if (s.drive != null) number(path + '.drive', s.drive, [0, 6]);
        };
        const events = isObj(def.events) ? def.events : {};
        if (!isObj(def.events)) fail('events', 'must be an object');
        EVENTS.forEach(id => { if (!events[id]) fail('events.' + id, 'missing'); else sound('events.' + id, events[id], ROLES[id], false); });
        Object.keys(events).forEach(id => { if (!ROLES[id]) fail('events.' + id, 'unknown event'); });
        const fx = def.fx == null ? {} : def.fx;
        if (!isObj(fx)) fail('fx', 'must be an object');
        else {
            Object.keys(fx).forEach(id => {
                const r = FX_ROLES[id];
                if (!r) return fail('fx.' + id, 'unknown effect');
                if (r.theme !== def.id) return fail('fx.' + id, 'belongs to theme ' + r.theme);
                if (!isObj(fx[id])) return fail('fx.' + id, 'must map variant names to sounds');
                Object.keys(fx[id]).forEach(name => { if (!r.names.includes(name)) fail('fx.' + id + '.' + name, 'unknown variant'); });
                r.names.forEach(name => { if (!fx[id][name]) fail('fx.' + id + '.' + name, 'missing'); else sound('fx.' + id + '.' + name, fx[id][name], r, true); });
            });
            Object.keys(FX_ROLES).forEach(id => { if (FX_ROLES[id].theme === def.id && !fx[id]) fail('fx.' + id, 'missing'); });
        }
        return errors;
    }
```

- [ ] Add the registry. `const registry = {}; root.DesktopSoundThemes = root.DesktopSoundThemes || {}; const warned = new Set();`
  - `defineTheme(def)` runs `validateTheme`. On errors it builds `'desktop sound theme ' + id + ' is invalid:\n  ' + errors.join('\n  ')`. If `root.DesktopSoundEngineStrict` is true it throws; otherwise it `console.warn`s once per id and returns `false` (the theme is skipped).
  - On success it deep-freezes the def, stores it in `registry[def.id]` and in `root.DesktopSoundThemes[def.id]`, and returns `true`.
  - `themes()` returns `Object.keys(registry)`.
- [ ] Add `render(themeId, eventId, variant)` (spec §6.4) and the bus. A shell `variant` is an index 0..variants−1; an effect `variant` is a name. The engine keeps no cache and returns fresh arrays, because the host transfers their buffers.

```js
    function render(themeId, eventId, variant) {
        const theme = registry[themeId];
        if (!theme) throw new Error('unknown sound theme: ' + themeId);
        const fxR = FX_ROLES[eventId], r = fxR || ROLES[eventId];
        if (!r) throw new Error('unknown sound event: ' + eventId);
        let def, index = 0;
        if (fxR) {
            def = theme.fx && theme.fx[eventId] && theme.fx[eventId][variant];
            if (!def) throw new Error('theme ' + themeId + ' has no effect ' + eventId + ':' + variant);
        } else {
            index = Number(variant || 0);
            if (!Number.isInteger(index) || index < 0 || index >= r.variants) throw new Error('variant ' + variant + ' out of range for ' + eventId);
            def = theme.events[eventId];
        }
        const seed = D.hashSeed(themeId, eventId, String(variant ?? 0));
        const vary = !fxR && index > 0 ? D.makeRng(D.hashSeed(seed, 'vary')) : null; // variant 0 is the canonical sound
        const items = (def.notes || []).map(x => [x, true]).concat((def.layers || []).map(x => [x, false]));
        const placed = items.map(([item, asNote], i) => {
            const voice = theme.voices[item.v];
            const params = {};
            Object.keys(voice).forEach(k => { if (k !== 'model') params[k] = voice[k]; });
            Object.keys(item).forEach(k => { if (!(k in ITEM_FIELDS)) params[k] = item[k]; });
            const model = I.MODELS[voice.model];
            if (vary && model.params.hardness) params.hardness = D.clamp(I.resolveParams(voice.model, params).hardness + (vary() * 2 - 1) * 0.08, 0, 1);
            const dt = vary ? (vary() * 2 - 1) * 0.002 : 0;
            const dur = Math.min(item.dur ?? model.duration(I.resolveParams(voice.model, params)), r.maxSeconds);
            const note = { freq: D.midiToHz(theme.key + (asNote ? item.n : 0) + (item.detune || 0) / 100), dur,
                gain: item.g ?? 1, pan: item.pan ?? 0, at: Math.max(0, (item.t || 0) + dt), detune: vary ? 4 : 0 };
            return { model: voice.model, params, note, rand: D.makeRng(D.hashSeed(seed, 'note', i)) };
        });
        let mix = D.createMix(Math.max(0.05, ...placed.map(p => p.note.at + p.note.dur)) + 0.02);
        placed.forEach(p => D.addStereo(mix, I.render(p.model, p.params, p.note, p.rand), p.note.at));
        mix = bus(mix, def, theme, !!fxR);
        const m = D.master(mix, { lufs: r.lufs, ceiling: -3, lowcut: 110, maxSeconds: r.maxSeconds });
        if (!m.seconds) throw new Error(themeId + '/' + eventId + ': silent sound');
        let lufs = m.lufs, peakDb = m.peakDb;
        if (vary) { // level deviation of a variant, after mastering so the loudness gate still holds (±0.4 dB)
            const offset = m.limited ? Math.min(0, (vary() * 2 - 1) * 0.4) : (vary() * 2 - 1) * 0.4;
            const g = D.dbToGain(offset);
            for (let i = 0; i < m.mix.n; i++) { m.mix.L[i] *= g; m.mix.R[i] *= g; }
            lufs += offset; peakDb += offset;
        }
        return { L: m.mix.L, R: m.mix.R, sampleRate: D.SR, seconds: m.seconds, naturalSeconds: m.naturalSeconds, lufs, peakDb, limited: m.limited };
    }
    function bus(mix, def, theme, isFx) { // fixed order: drive, tone, chorus, echo, space
        if (def.drive) { D.saturate(mix.L, def.drive); D.saturate(mix.R, def.drive); }
        if (def.tone && def.tone.low) { D.shelf(mix.L, 250, def.tone.low, false); D.shelf(mix.R, 250, def.tone.low, false); }
        if (def.tone && def.tone.high) { D.shelf(mix.L, 5000, def.tone.high, true); D.shelf(mix.R, 5000, def.tone.high, true); }
        if (def.chorus) mix = D.chorus(mix, def.chorus);
        if (def.echo) mix = D.echo(mix, def.echo);
        if (!isFx && def.space !== null) mix = D.reverb(mix, Object.assign({}, theme.space, def.space || {}));
        return mix;
    }
```

  Export `{ THEME_IDS, EVENTS, ROLES, CATEGORIES, FX_ROLES, defineTheme, validateTheme, themes, render }` as `root.DesktopSoundEngine` and via `module.exports` when present.
- [ ] Create `render-host.js`, verbatim:

```js
// Last part of desktop-sounds.bundle.js. In a Worker it answers render requests; in a window it does nothing.
(function () {
    'use strict';
    if (typeof WorkerGlobalScope === 'undefined' || !(self instanceof WorkerGlobalScope)) return;
    self.onmessage = function (event) {
        const msg = event.data || {};
        try {
            const out = self.DesktopSoundEngine.render(String(msg.theme || ''), String(msg.event || ''), msg.variant == null ? 0 : msg.variant);
            self.postMessage({ id: msg.id, ok: true, L: out.L, R: out.R, sampleRate: out.sampleRate, seconds: out.seconds, lufs: out.lufs, peakDb: out.peakDb }, [out.L.buffer, out.R.buffer]);
        } catch (err) {
            self.postMessage({ id: msg.id, ok: false, error: String((err && err.message) || err) });
        }
    };
})();
```

- [ ] Create `theme-crystal.js` (`(function () { 'use strict'; self.DesktopSoundEngine.defineTheme({ … }); })();`). Key 81 (A5, major pentatonic: n ∈ {0,2,4,7,9} + 12k; `notify.error` may use the A major scale), space `{ size: 0.9, rt60: 0.85, damping: 7000, predelay: 0.012, mix: 0.24 }`. Voices: `chime: { model: 'glass', t60: 0.9, beat: 0.55, attack: 0.004 }`, `tick: { model: 'click', material: 'glass', tone: 5200, ring: 0.03 }`, `air: { model: 'noise', color: 'pink', band: [2500, 7500], attack: 0.06, decay: 0.2 }`, `knock: { model: 'noise', color: 'pink', band: [120, 900], attack: 0.001, decay: 0.03 }`. Three reference events (format reference for all theme tasks):

```js
        'window.open': {
            notes: [{ v: 'chime', n: -5, t: 0, g: 0.9, pan: -0.15 }, { v: 'chime', n: 0, t: 0.085, g: 1, pan: 0.2 }],
            layers: [{ v: 'air', t: 0, g: 0.05 }]
        },
        'notify.message': {
            notes: [{ v: 'chime', n: -5, t: 0, g: 0.8, pan: -0.2, t60: 0.7 }, { v: 'chime', n: 0, t: 0.11, g: 0.9, pan: 0.15, t60: 0.8 },
                { v: 'chime', n: 4, t: 0.22, g: 1, pan: 0, t60: 1.25 }],
            space: { size: 1.05, rt60: 1.2, damping: 6500, predelay: 0.018, mix: 0.3 }
        },
        'notify.error': {
            notes: [{ v: 'chime', n: -19, t: 0, g: 1, pan: -0.1, t60: 0.42, damp: 0.9, beat: 1.4, attack: 0.008 },
                { v: 'chime', n: -20, t: 0.14, g: 0.9, pan: 0.1, t60: 0.42, damp: 0.9, beat: 1.4, attack: 0.008 }],
            layers: [{ v: 'knock', t: 0, g: 0.5 }, { v: 'knock', t: 0.14, g: 0.45 }],
            space: { size: 0.7, rt60: 0.5, damping: 4500, predelay: 0.006, mix: 0.2 }
        },
```

  Author the other 23 events with the grammar of the theme contract (Stage 3), `fx: {}`.
- [ ] Create `scripts/test-desktop-sounds.mjs` (spec §12 gates), verbatim:

```js
// Measured gates of the desktop sound system (spec 2026-11-23 §12).
//   node scripts/test-desktop-sounds.mjs                 all 16 themes, every event, variant and effect
//   node scripts/test-desktop-sounds.mjs --theme water   authoring run for one or more themes (no cross-theme checks)
import assert from 'node:assert/strict';
import { performance } from 'node:perf_hooks';
import { loadDesktopSounds } from './desktop-sound-loader.mjs';

const args = process.argv.slice(2), only = [];
for (let i = 0; i < args.length; i++) if (args[i] === '--theme') only.push(...String(args[++i] || '').split(',').filter(Boolean));
const full = only.length === 0;
const budgetMs = Number(process.env.DESKTOP_SOUNDS_THEME_BUDGET_MS || 6000);
const { DSP, Engine } = loadDesktopSounds({ themes: full ? 'all' : only, requireAll: full });
const FX_OWNERS = { voxel_world: ['window.fracture', 'window.impact'], green_slime: ['slime.pop', 'slime.snap'], orbit: ['orbit.ping'] };
const MIRRORED = [['window.open', 'window.close'], ['window.restore', 'window.minimize'], ['menu.open', 'menu.close'], ['dialog.confirm', 'dialog.cancel'], ['ui.toggle.on', 'ui.toggle.off']];
const failures = [], warnings = [], rows = [];
const check = (ok, message) => { if (!ok) failures.push(message); };

// Catalogue and registration
assert.equal(Engine.EVENTS.length, 26);
assert.deepEqual(Object.keys(Engine.ROLES).sort(), Engine.EVENTS.slice().sort());
for (const id of Engine.EVENTS) { const r = Engine.ROLES[id]; assert.ok(Engine.CATEGORIES.includes(r.category) && r.variants >= 1 && r.variants <= 3, id); }
const themes = Engine.themes();
assert.deepEqual(themes.slice().sort(), (full ? Engine.THEME_IDS : only).slice().sort(), 'registered themes');
for (const t of themes) assert.deepEqual(Object.keys(self.DesktopSoundThemes[t].fx || {}).sort(), (FX_OWNERS[t] || []).slice().sort(), t + ' effects');

// Validation rejects broken definitions
const base = self.DesktopSoundThemes[themes[0]];
const clone = () => JSON.parse(JSON.stringify(base));
const voice0 = Object.keys(base.voices)[0];
assert.deepEqual(Engine.validateTheme(clone()), [], 'a registered theme validates');
for (const [name, mutate] of [
  ['unknown model', d => { d.voices[voice0].model = 'kazoo'; }],
  ['unknown parameter', d => { d.voices[voice0].sparkle = 1; }],
  ['unknown voice', d => { d.events['window.open'].notes = [{ v: 'nope', n: 0, t: 0, g: 1 }]; }],
  ['unknown event', d => { d.events['window.wobble'] = d.events['window.open']; }],
  ['missing event', d => { delete d.events['ui.tick']; }],
  ['value out of range', d => { d.space.rt60 = 99; }],
  ['effect of another theme', d => { const s = d.events['window.snap']; d.id = d.id === 'orbit' ? 'crystal' : d.id; d.fx = { 'orbit.ping': { small: s, medium: s, large: s } }; }]
]) { const d = clone(); mutate(d); check(Engine.validateTheme(d).length > 0, 'validation accepted a broken theme: ' + name); }

const fingerprint = (L, R) => { let h = 0x811c9dc5; for (const ch of [L, R]) { const b = new Uint8Array(ch.buffer, ch.byteOffset, ch.byteLength); for (let i = 0; i < b.length; i++) h = Math.imul(h ^ b[i], 0x01000193) >>> 0; } return h.toString(16) + ':' + L.length; };
const energy = a => { let e = 0; for (let i = 0; i < a.length; i++) e += a[i] * a[i]; return e; };
const below90 = ch => { const b = Float32Array.from(ch); DSP.lowpass(b, 90, 0.5412); DSP.lowpass(b, 90, 1.3066); return energy(b); };
const mean = ch => { let s = 0; for (let i = 0; i < ch.length; i++) s += ch[i]; return s / ch.length; };

function gate(label, out, r) {
  const { L, R } = out, n = L.length;
  check(n > 0 && R.length === n && out.sampleRate === 48000 && Math.abs(out.seconds - n / 48000) < 1e-6, `${label}: bad buffer shape`);
  for (let i = 0; i < n; i++) if (!Number.isFinite(L[i]) || !Number.isFinite(R[i])) { failures.push(`${label}: NaN or Infinity`); return; }
  if (out.limited) {
    check(out.lufs <= r.lufs + 0.5, `${label}: ${out.lufs.toFixed(2)} LUFS above target ${r.lufs} while peak-limited`);
    if (out.lufs < r.lufs - 10) warnings.push(`${label}: peak-limited ${(r.lufs - out.lufs).toFixed(1)} LU under target (transient too sharp?)`);
  } else check(Math.abs(out.lufs - r.lufs) <= 0.5, `${label}: ${out.lufs.toFixed(2)} LUFS, target ${r.lufs} ±0.5`);
  const measured = DSP.loudness({ L, R, n });
  check(Math.abs(measured - out.lufs) <= 0.05, `${label}: reports ${out.lufs.toFixed(2)} LUFS, measures ${measured.toFixed(2)}`);
  const tp = DSP.gainToDb(DSP.truePeak({ L, R, n }));
  check(tp <= -3 + 0.05, `${label}: true peak ${tp.toFixed(2)} dBTP above -3`);
  check(out.naturalSeconds <= r.maxSeconds + 1e-9, `${label}: natural length ${out.naturalSeconds.toFixed(3)} s over ${r.maxSeconds} s`);
  check(L[0] === 0 && R[0] === 0 && L[n - 1] === 0 && R[n - 1] === 0, `${label}: first or last sample not zero`);
  check(Math.abs(mean(L)) < 1e-3 && Math.abs(mean(R)) < 1e-3, `${label}: DC offset`);
  const low = 10 * Math.log10(Math.max(below90(L) + below90(R), 1e-20) / Math.max(energy(L) + energy(R), 1e-20));
  check(low <= -30, `${label}: energy below 90 Hz only ${(-low).toFixed(1)} dB under total`);
}

const prints = {};
for (const theme of themes) {
  const t0 = performance.now(); let renders = 0, worst = 0;
  prints[theme] = {};
  for (const eventId of Engine.EVENTS) {
    const r = Engine.ROLES[eventId], outs = [];
    for (let v = 0; v < r.variants; v++) { const out = Engine.render(theme, eventId, v); renders++; gate(`${theme}/${eventId}#${v}`, out, r); outs.push(out); if (!out.limited) worst = Math.max(worst, Math.abs(out.lufs - r.lufs)); }
    prints[theme][eventId] = fingerprint(outs[0].L, outs[0].R);
    for (let a = 0; a < outs.length; a++) for (let b = a + 1; b < outs.length; b++) {
      check(fingerprint(outs[a].L, outs[a].R) !== fingerprint(outs[b].L, outs[b].R), `${theme}/${eventId}: variants ${a} and ${b} are identical`);
      check(Math.abs(outs[a].lufs - outs[b].lufs) <= 1, `${theme}/${eventId}: variants ${a} and ${b} differ by more than 1 LU`);
    }
  }
  for (const eventId of FX_OWNERS[theme] || []) for (const name of Engine.FX_ROLES[eventId].names) { gate(`${theme}/${eventId}:${name}`, Engine.render(theme, eventId, name), Engine.FX_ROLES[eventId]); renders++; }
  const ms = performance.now() - t0;
  check(ms <= budgetMs, `${theme}: full render took ${ms.toFixed(0)} ms, budget ${budgetMs} ms`);
  for (const eventId of ['window.open', 'notify.message', 'ui.tick']) {
    const a = Engine.render(theme, eventId, 0), b = Engine.render(theme, eventId, 0);
    check(fingerprint(a.L, a.R) === fingerprint(b.L, b.R), `${theme}/${eventId}: render is not deterministic`);
  }
  for (const [x, y] of MIRRORED) check(prints[theme][x] !== prints[theme][y], `${theme}: mirrored pair ${x}/${y} renders identical audio`);
  rows.push({ theme, renders, ms: Math.round(ms), worstLU: worst.toFixed(2) });
}
if (full) for (const eventId of Engine.EVENTS) {
  const seen = new Set(themes.map(t => prints[t][eventId]));
  check(seen.size === themes.length, `${eventId}: two themes render identical audio`);
}
console.table(rows);
warnings.forEach(w => console.warn('warning: ' + w));
if (failures.length) { console.error(failures.length + ' gate failure(s):\n  ' + failures.join('\n  ')); process.exit(1); }
console.log('desktop sound gates: ok (' + themes.length + ' theme(s))');
```

- [ ] Create `scripts/render-desktop-sounds.mjs` (authoring tool; the parent agent and theme authors use it):
  - Args: `--theme <id[,id]|all>` (default: every theme file that exists), `--event <id>` (shell or effect), `--variant <n|name|all>` (default `all`), `--out <dir>` (required), `--json` (print the rows as JSON instead of `console.table`).
  - Loads with `loadDesktopSounds({ themes })`.
  - Writes `<out>/<theme>/<event>.v<n>.wav` (effects: `<event>.<name>.wav`). Also writes `<out>/<theme>/<theme>-contact-sheet.wav`: variant 0 of the 26 events in `EVENTS` order, then every effect variant, each followed by 0.5 s silence, at their true levels with no renormalization. Plus `<out>/<theme>/levels.json`.
  - Row columns: `theme, event, variant, seconds, natural, lufs, target, deltaLU, peakDb, limited, centroidHz, attackMs, renderMs`.
  - `centroidHz`: energy-weighted mean over 2048-sample Hann frames (hop 1024) of Σf·|X| / Σ|X|, using an in-file radix-2 FFT on (L+R)/2.
  - `attackMs`: time from the first sample above −40 dB of the peak to the first 1 ms RMS frame within 3 dB of the loudest 1 ms frame.
  - Contains the WAV writer moved from the spike (`encodeWav`, 24-bit stereo 48 kHz).
- [ ] Add to `package.json` `scripts`: `"test:desktop-sounds": "node scripts/test-desktop-sound-dsp.mjs && node scripts/test-desktop-sound-instruments.mjs && node scripts/test-desktop-sounds.mjs"` and `"render:desktop-sounds": "node scripts/render-desktop-sounds.mjs"`.
- [ ] Run `node scripts/test-desktop-sounds.mjs --theme crystal`. Expected: table row for crystal, `desktop sound gates: ok (1 theme(s))`, exit 0. Fix crystal data or engine bugs until it passes. Do not loosen the gates.
- [ ] Run `node scripts/render-desktop-sounds.mjs --theme crystal --out C:\disposable\desktop-sounds\render`. Expected: 56 WAVs + contact sheet + levels.json.
- [ ] Commit: `git add ui/js/desktop/sound/sound-engine.js ui/js/desktop/sound/render-host.js ui/js/desktop/sound/theme-crystal.js scripts/test-desktop-sounds.mjs scripts/render-desktop-sounds.mjs package.json` then `git commit -m "feat(desktop-sounds): engine, worker host, Node render and gate tools, crystal reference theme"`.

**Stage 1 review (coordinator):** one combined review of Tasks 1–3 (Critical/Important only). Listening check with `earshot` on the crystal contact sheet and on `window.open`, `notify.message`, `notify.error` (crystal must read as glass chimes or a notification ding). Hand the crystal contact sheet to the user.

---

## Stage 2 — Runtime integration, settings, translations (parallel to Stage 3)

### Task 4: Settings keys

**Files:**
- Modify: `internal/desktop/types.go:288-291`, `ui/js/desktop/core/desktop-foundation.js:343-346`
- Test: `internal/desktop/service_test.go:1284-1292`

- [ ] Run impact for: `DesktopSettingDefinitions`, `TestServiceSettingsUseDefaultsAndValidateWrites` (the test only needs a caller check).
- [ ] In `TestServiceSettingsUseDefaultsAndValidateWrites`, after the `sound.volume` check (`:1292`), add:

```go
	for _, key := range []string{"sound.interface", "sound.startup"} {
		if bootstrap.Settings[key] != "true" {
			t.Fatalf("default %s = %q", key, bootstrap.Settings[key])
		}
		if err := svc.SetSetting(ctx, key, "false", SourceUser); err != nil {
			t.Fatalf("SetSetting %s false: %v", key, err)
		}
		if err := svc.SetSetting(ctx, key, "maybe", SourceUser); err == nil {
			t.Fatalf("SetSetting %s accepted an invalid value", key)
		}
	}
```

- [ ] Run `go test ./internal/desktop -run '(?i)(TestServiceSettingsUseDefaultsAndValidateWrites)'`. Expected: FAIL (`default sound.interface = ""`).
- [ ] In `types.go` after `sound.files` (`:291`) add:

```go
		{Key: "sound.interface", Default: "true", Values: []string{"true", "false"}},
		{Key: "sound.startup", Default: "true", Values: []string{"true", "false"}},
```

- [ ] In `desktopSettingDefaults` (`desktop-foundation.js`) after `'sound.files': 'true',` add `'sound.interface': 'true',` and `'sound.startup': 'true',`.
- [ ] Run the same `go test`. Expected: PASS (count the `PASS` line).
- [ ] Commit: `git add internal/desktop/types.go internal/desktop/service_test.go ui/js/desktop/core/desktop-foundation.js` then `git commit -m "feat(desktop-sounds): interface and startup sound settings"`.

### Task 5: Translations (16 locales)

**Files:**
- Modify: `ui/lang/desktop/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json`

- [ ] Insert the five keys in alphabetical position inside the `desktop.settings_sound_*` block (about lines 3604–3627): `…_interface` and `…_interface_desc` after `…_files_desc`; `…_preview_category` after `…_preview`; `…_startup` and `…_startup_desc` right after it. Keep 2-space indent, UTF-8 without BOM and valid JSON. The placeholder is `{{name}}`, the desktop `t()` syntax (see `desktop.window_group_count`).

| Locale | `settings_sound_interface` | `…_interface_desc` | `settings_sound_startup` | `…_startup_desc` | `settings_sound_preview_category` |
| --- | --- | --- | --- | --- | --- |
| cs | Odezva kliknutí | Tichá cvaknutí u přepínačů, posuvníků a ikon v docku. | Úvodní zvuk | Krátká melodie po načtení plochy, pokud to prohlížeč povolí. | Přehrát ukázku: {{name}} |
| da | Klikfeedback | Diskrete klik for kontakter, skydere og dock-ikoner. | Startlyd | En kort melodi, når skrivebordet er indlæst, hvis browseren tillader det. | Forhåndslyt {{name}} |
| de | Klick-Feedback | Leise Klicks für Schalter, Schieberegler und Dock-Symbole. | Startklang | Eine kurze Melodie, sobald der Desktop geladen ist – sofern der Browser es erlaubt. | {{name}} anhören |
| el | Ηχητική απόκριση κλικ | Διακριτικά κλικ για διακόπτες, ρυθμιστικά και εικονίδια του dock. | Ήχος εκκίνησης | Μια σύντομη μελωδία μόλις φορτωθεί η επιφάνεια εργασίας, αν το επιτρέπει το πρόγραμμα περιήγησης. | Προεπισκόπηση: {{name}} |
| en | Click feedback | Quiet clicks for switches, sliders and dock icons. | Start sound | A short melody when the desktop has loaded, if the browser allows it. | Preview {{name}} |
| es | Respuesta al clic | Clics discretos para interruptores, controles deslizantes e iconos del dock. | Sonido de inicio | Una breve melodía al terminar de cargar el escritorio, si el navegador lo permite. | Escuchar {{name}} |
| fr | Retour sonore des clics | Clics discrets pour les interrupteurs, les curseurs et les icônes du dock. | Son de démarrage | Une courte mélodie une fois le bureau chargé, si le navigateur l’autorise. | Écouter {{name}} |
| hi | क्लिक प्रतिक्रिया | स्विच, स्लाइडर और डॉक आइकन के लिए हल्की क्लिक ध्वनियाँ। | आरंभ ध्वनि | डेस्कटॉप लोड होने पर एक छोटी धुन, यदि ब्राउज़र इसकी अनुमति दे। | {{name}} सुनें |
| it | Feedback dei clic | Clic discreti per interruttori, cursori e icone del dock. | Suono di avvio | Una breve melodia al termine del caricamento del desktop, se il browser lo consente. | Ascolta {{name}} |
| ja | クリックのフィードバック | スイッチ、スライダー、Dock アイコンの控えめなクリック音。 | 起動音 | デスクトップの読み込みが終わると短いメロディを鳴らします（ブラウザーが許可する場合）。 | {{name}}を試聴 |
| nl | Klikfeedback | Zachte klikjes voor schakelaars, schuifregelaars en dock-pictogrammen. | Opstartgeluid | Een korte melodie zodra het bureaublad is geladen, als de browser dat toestaat. | {{name}} beluisteren |
| no | Klikkelyd | Diskré klikk for brytere, skyveknapper og dock-ikoner. | Oppstartslyd | En kort melodi når skrivebordet er lastet inn, hvis nettleseren tillater det. | Forhåndslytt {{name}} |
| pl | Dźwięk kliknięć | Ciche kliknięcia przełączników, suwaków i ikon w docku. | Dźwięk startowy | Krótka melodia po wczytaniu pulpitu, jeśli przeglądarka na to pozwala. | Odsłuchaj: {{name}} |
| pt | Resposta ao clique | Cliques discretos para interruptores, controlos deslizantes e ícones da dock. | Som de arranque | Uma melodia curta quando o ambiente de trabalho acaba de carregar, se o navegador o permitir. | Ouvir {{name}} |
| sv | Klickljud | Diskreta klick för brytare, skjutreglage och dockikoner. | Startljud | En kort melodi när skrivbordet har laddats, om webbläsaren tillåter det. | Förhandslyssna på {{name}} |
| zh | 点击反馈 | 开关、滑块和程序坞图标的轻柔点击声。 | 启动音效 | 桌面加载完成后播放一段简短旋律（需浏览器允许）。 | 试听{{name}} |

- [ ] Run `node -e "for (const l of 'cs da de el en es fr hi it ja nl no pl pt sv zh'.split(' ')) JSON.parse(require('fs').readFileSync('ui/lang/desktop/'+l+'.json','utf8'))"`. Expected: no output.
- [ ] Commit: `git add ui/lang/desktop/*.json` then `git commit -m "i18n(desktop-sounds): click feedback, start sound and category preview in 16 locales"`.

### Task 6: Settings UI (two switches, per-category preview)

**Files:**
- Modify: `ui/js/desktop/core/settings-catalog-runtime.js:13-15, :155-158`; `ui/js/desktop/apps/settings.js:266-285, :434-449`; `ui/css/desktop-app-settings.css` (after `:682`)

- [ ] Run impact for: `settingToggle`, `sections`, `renderSettingItem`, `wireSettings`.
- [ ] `settingToggle(key, label, desc, previewCategory)` returns `previewCategory ? { type: 'toggle', key, label, desc, previewCategory } : { type: 'toggle', key, label, desc }`.
- [ ] Sound section items become (the literal prefixes `settingToggle('sound.windows'` and so on stay):

```js
                    settingToggle('sound.windows', 'desktop.settings_sound_windows', 'desktop.settings_sound_windows_desc', 'windows'),
                    settingToggle('sound.notifications', 'desktop.settings_sound_notifications', 'desktop.settings_sound_notifications_desc', 'notifications'),
                    settingToggle('sound.navigation', 'desktop.settings_sound_navigation', 'desktop.settings_sound_navigation_desc', 'navigation'),
                    settingToggle('sound.files', 'desktop.settings_sound_files', 'desktop.settings_sound_files_desc', 'files'),
                    settingToggle('sound.interface', 'desktop.settings_sound_interface', 'desktop.settings_sound_interface_desc', 'interface'),
                    settingToggle('sound.startup', 'desktop.settings_sound_startup', 'desktop.settings_sound_startup_desc', 'startup')
```

- [ ] In `renderSettingItem` replace the toggle branch of `control` (`:437-438`):

```js
        const toggle = `<label class="vd-switch"><input type="checkbox" data-setting-key="${esc(item.key)}" aria-label="${esc(t(item.label))}" ${settingBool(item.key) ? 'checked' : ''}><span></span></label>`;
        const control = item.type === 'toggle'
            ? (item.previewCategory
                ? `<div class="vd-setting-actions vd-sound-category-actions"><button type="button" class="vd-button vd-sound-category-preview" data-sound-category-preview="${esc(item.previewCategory)}" aria-label="${esc(t('desktop.settings_sound_preview_category', { name: t(item.label) }))}">${esc(t('desktop.settings_sound_preview'))}</button>${toggle}</div>`
                : toggle)
            : /* existing <select> markup unchanged */;
```

- [ ] In `wireSettings`, after the `[data-sound-preview]` block (`:267-285`), add:

```js
        host.querySelectorAll('[data-sound-category-preview]').forEach(btn => {
            btn.addEventListener('click', async event => {
                event.preventDefault();
                event.stopPropagation();
                const sounds = window.DesktopSounds;
                if (!sounds || typeof sounds.previewCategory !== 'function' || btn.disabled) return;
                btn.disabled = true;
                btn.setAttribute('aria-busy', 'true');
                btn.textContent = ctx.t('desktop.settings_sound_preview_playing');
                try { await sounds.previewCategory(btn.dataset.soundCategoryPreview, ctx.settingValue('sound.theme') || 'desktop'); }
                catch (err) { ctx.showDesktopNotification({ title: ctx.t('desktop.notification'), message: ctx.t('desktop.settings_sound_preview_failed') }); }
                finally {
                    if (btn.isConnected) {
                        btn.disabled = false;
                        btn.removeAttribute('aria-busy');
                        btn.textContent = ctx.t('desktop.settings_sound_preview');
                    }
                }
            });
        });
```

- [ ] CSS after `.vd-setting-actions` (`:682`): `.vd-sound-category-actions { align-items: center; justify-content: flex-end; flex-wrap: nowrap; } .vd-sound-category-preview { white-space: nowrap; }`.
- [ ] Run `node scripts/test-desktop-search.mjs` and `go test ./ui -run '(?i)(DesktopSoundSettingsUIMarkers|DesktopSettings|SettingsIcons)'`. Expected: PASS. `TestDesktopSoundSettingsUIMarkers` still passes because the literals are kept.
- [ ] Commit: `git add ui/js/desktop/core/settings-catalog-runtime.js ui/js/desktop/apps/settings.js ui/css/desktop-app-settings.css` then `git commit -m "feat(desktop-sounds): interface and startup switches, preview per sound category"`.

### Task 7: Sound runtime, notification mapping, start-sound request

**Files:**
- Modify: `ui/js/desktop/core/sound-runtime.js` (1-530), `ui/js/desktop/core/sdk-events-bootstrap.js:496-509, :1019-1021`

- [ ] Run impact for: `desktopSound`, `playBuffer`, `ensureContext`, `syncDesktopSoundSettings`, `closeSoundContext`, `wireUnlock`, `renderTheme`, `ensureBuffers`, `ensureThemeFxBuffer`, `rateLimited`, `previewDesktopSound`, `inspect`, `applySoundSettingsChange`, `showDesktopNotification`, `init` (sdk-events-bootstrap).
- [ ] Keep the IIFE shape (the file starts at `(function () {` and ends at `})();`, ui/AGENTS.md:1157-1160). Keep `THEME_FX`, `THEME_FX_OF`, `themeFxState`, `queueThemeFx`, `cleanThemeFxOptions`, `cancelThemeFx`, `invalidateThemeFxBuffers`, `loadSoundBundle` (with `loader('desktop-sounds')`), `actx = new Ctor()`, and the visibility, pagehide and auth-ended handlers.
- [ ] Tables. Extend `EVENT_CATEGORY` (keep every existing entry literally) with `'session.start': 'startup'`, `'notify.success': 'notifications'`, `'notify.warning': 'notifications'`, `'ui.select': 'interface'`, `'ui.toggle.on': 'interface'`, `'ui.toggle.off': 'interface'`, `'ui.tick': 'interface'`. Add:

```js
    const CATEGORIES = ['windows', 'notifications', 'navigation', 'files', 'interface', 'startup'];
    // Variants per shell event, in spec order (mirrors DesktopSoundEngine.ROLES; ui/desktop_sound_settings_test.go checks both).
    const EVENT_VARIANTS = {
        'session.start': 1, 'notify.message': 1, 'notify.info': 1, 'notify.success': 1, 'notify.warning': 1, 'notify.error': 1,
        'dialog.open': 1, 'dialog.confirm': 1, 'dialog.cancel': 1, 'window.open': 3, 'window.close': 3, 'window.maximize': 3,
        'window.minimize': 3, 'window.restore': 3, 'window.snap': 3, 'window.deny': 2, 'file.drop': 3, 'file.trash': 2,
        'file.delete': 2, 'space.switch': 2, 'menu.open': 3, 'menu.close': 3, 'ui.select': 3, 'ui.toggle.on': 3, 'ui.toggle.off': 3, 'ui.tick': 3
    };
    const PREWARM_ORDER = ['window.open', 'window.close', 'menu.open', 'menu.close', 'window.minimize', 'window.restore',
        'window.maximize', 'window.snap', 'window.deny', 'ui.select', 'ui.toggle.on', 'ui.toggle.off', 'ui.tick', 'space.switch',
        'dialog.open', 'dialog.confirm', 'dialog.cancel', 'file.drop', 'file.trash', 'file.delete',
        'notify.info', 'notify.message', 'notify.success', 'notify.warning', 'notify.error', 'session.start'];
    const WORKER_PATH = '/js/desktop/bundles/desktop-sounds.bundle.js';
    const FIRST_REPLY_MS = 4000, LATE_MS = 150, START_WAIT_MS = 1500, FEEDBACK_GAP_MS = 45;
```

  Replace every hard-coded `['windows', 'notifications', 'navigation', 'files']` (in `ensureContext`, `syncDesktopSoundSettings`, `inspect`) with `CATEGORIES`. Stats become `{ plays, renders, previewPlays, dropped, lastEvent, lastLufs, lastPeakDb }`; drop `lastPeak` and `lastRms`.
- [ ] Limiter in `ensureContext` (replaces `threshold -18 / ratio 3`):

```js
            compressor = actx.createDynamicsCompressor();
            // Safety limiter: mastered sounds peak at or below -3 dBTP and pass untouched; only overlaps are caught.
            compressor.threshold.value = -2;
            compressor.knee.value = 0;
            compressor.ratio.value = 12;
            compressor.attack.value = 0.003;
            compressor.release.value = 0.12;
```

- [ ] Worker client and cache. Delete `renderTheme`'s old body and `ensureBuffers`; add, verbatim:

```js
    let renderer = '';             // '' until a sound renders, then 'worker' or 'main'
    let worker = null, workerAnswered = false, workerBroken = false, firstReplyTimer = null, requestSeq = 0;
    const renderQueue = [];        // waiting jobs; urgent ones are put in front
    const renderJobs = new Map();  // key -> job (queued or running), so one sound renders once
    let running = null;            // at most one job in flight: queue order and queue jumping stay ours
    let cache = { theme: '', entries: {} };   // entries[event + '#' + variant] = { result, buffer }
    let prewarmedTheme = '';
    const warned = new Set();
    function warnOnce(key, ...args) { if (!warned.has(key)) { warned.add(key); console.warn(...args); } }

    function requestRender(theme, event, variant, urgent) {
        const key = theme + '|' + event + '|' + variant;
        const existing = renderJobs.get(key);
        if (existing) {
            const at = renderQueue.indexOf(existing);
            if (urgent && at > 0) { renderQueue.splice(at, 1); renderQueue.unshift(existing); }
            return existing.promise;
        }
        const job = { key, theme, event, variant, id: 0 };
        job.promise = new Promise((resolve, reject) => { job.resolve = resolve; job.reject = reject; });
        renderJobs.set(key, job);
        if (urgent) renderQueue.unshift(job); else renderQueue.push(job);
        pumpRenders();
        return job.promise;
    }

    function finishJob(job, error, result) {
        renderJobs.delete(job.key);
        if (running === job) running = null;
        // Test seam: a promise in window.DesktopSoundsRenderGate holds every reply until it settles.
        Promise.resolve(window.DesktopSoundsRenderGate).catch(() => {}).then(() => { if (error) job.reject(error); else job.resolve(result); });
        setTimeout(pumpRenders, 0);   // one sound per task
    }

    function pumpRenders() {
        if (running || !renderQueue.length) return;
        const job = renderQueue.shift();
        running = job;
        if (!workerBroken && ensureWorker()) {
            job.id = ++requestSeq;
            if (!workerAnswered && firstReplyTimer === null) firstReplyTimer = setTimeout(() => workerFailed(new Error('sound worker did not answer within 4 s')), FIRST_REPLY_MS);
            worker.postMessage({ id: job.id, theme: job.theme, event: job.event, variant: job.variant });
        } else renderOnMainThread(job);
    }

    function ensureWorker() {
        if (worker) return true;
        if (typeof Worker !== 'function') { workerBroken = true; return false; }
        try {
            const lazy = window.AuraLazyAssets;
            worker = new Worker(lazy && typeof lazy.versionedURL === 'function' ? lazy.versionedURL(WORKER_PATH) : WORKER_PATH);
        } catch (e) { worker = null; workerBroken = true; return false; }
        worker.onmessage = onWorkerMessage;
        worker.onerror = event => { if (event && event.preventDefault) event.preventDefault(); workerFailed(new Error('sound worker error')); };
        worker.onmessageerror = () => workerFailed(new Error('sound worker message error'));
        return true;
    }

    function onWorkerMessage(event) {
        const msg = event.data || {}, job = running;
        if (!job || msg.id !== job.id) return;   // reply for a job dropped by stopRenders
        if (firstReplyTimer !== null) { clearTimeout(firstReplyTimer); firstReplyTimer = null; }
        workerAnswered = true;
        renderer = 'worker';
        if (msg.ok) finishJob(job, null, { L: msg.L, R: msg.R, sampleRate: msg.sampleRate, seconds: msg.seconds, lufs: msg.lufs, peakDb: msg.peakDb });
        else finishJob(job, new Error(msg.error || 'sound render failed'));
    }

    function workerFailed(error) {
        if (firstReplyTimer !== null) { clearTimeout(firstReplyTimer); firstReplyTimer = null; }
        warnOnce('worker', 'Desktop sound worker unavailable; rendering on the main thread', error);
        workerBroken = true;   // stays on the main thread for this page load
        if (worker) { try { worker.terminate(); } catch (e) {} worker = null; }
        const job = running;
        running = null;
        if (job) renderQueue.unshift(job);
        pumpRenders();
    }

    function renderOnMainThread(job) {
        loadSoundBundle().then(() => new Promise(resolve => setTimeout(resolve, 0))).then(() => {
            if (running !== job) return;
            const engine = window.DesktopSoundEngine;
            if (!engine) throw new Error('DesktopSoundEngine unavailable');
            const out = engine.render(job.theme, job.event, job.variant);
            renderer = 'main';
            finishJob(job, null, out);
        }).catch(err => { if (running === job) finishJob(job, err); });
    }

    function stopRenders() {   // sounds off, pagehide, auth end: drop the queue and terminate the worker
        if (firstReplyTimer !== null) { clearTimeout(firstReplyTimer); firstReplyTimer = null; }
        const dropped = renderQueue.splice(0);
        if (running) dropped.push(running);
        running = null;
        renderJobs.clear();
        dropped.forEach(job => job.resolve(null));
        if (worker) { try { worker.terminate(); } catch (e) {} worker = null; }
        workerAnswered = false;
    }

    function ensureEventBuffer(themeId, eventId, variant, urgent) {
        const entryKey = eventId + '#' + variant;
        if (cache.theme === themeId && cache.entries[entryKey]) return Promise.resolve(cache.entries[entryKey]);
        return requestRender(themeId, eventId, variant, urgent).then(result => {
            if (!result) return null;
            stats.renders++;
            const entry = { result, buffer: null };
            if (cache.theme !== themeId) cache = { theme: themeId, entries: {} };   // only one theme is cached
            cache.entries[entryKey] = entry;
            return entry;
        });
    }

    function audioBufferFor(entry) {
        if (entry.buffer) return entry.buffer;
        if (!actx || !entry.result || !entry.result.L) return null;
        const r = entry.result;
        const buffer = actx.createBuffer(2, r.L.length, r.sampleRate);
        buffer.copyToChannel(r.L, 0);
        buffer.copyToChannel(r.R, 1);
        entry.buffer = buffer;
        entry.result = { sampleRate: r.sampleRate, seconds: r.seconds, lufs: r.lufs, peakDb: r.peakDb, length: r.L.length };
        return buffer;
    }

    function prewarm(themeId) {   // variant 0 of every event in priority order, then the other variants
        if (prewarmedTheme === themeId) return;
        prewarmedTheme = themeId;
        for (let i = renderQueue.length - 1; i >= 0; i--) {
            const job = renderQueue[i];
            if (job.theme !== themeId) { renderQueue.splice(i, 1); renderJobs.delete(job.key); job.resolve(null); }
        }
        const quiet = p => p.catch(err => warnOnce('prewarm:' + themeId, 'Desktop sound prewarm failed', err));
        PREWARM_ORDER.forEach(eventId => quiet(ensureEventBuffer(themeId, eventId, 0, false)));
        PREWARM_ORDER.forEach(eventId => { for (let v = 1; v < EVENT_VARIANTS[eventId]; v++) quiet(ensureEventBuffer(themeId, eventId, v, false)); });
    }
```

- [ ] Variant round-robin, verbatim:

```js
    const lastVariant = {};
    // Round-robin without repeating the previous variant; prefers a variant that is already rendered.
    function chooseVariant(themeId, eventId) {
        const count = EVENT_VARIANTS[eventId] || 1;
        if (count <= 1) return 0;
        const prev = lastVariant[eventId];
        const start = prev == null ? 0 : (prev + 1) % count;
        for (let k = 0; k < count; k++) {
            const v = (start + k) % count;
            if (v !== prev && cache.theme === themeId && cache.entries[eventId + '#' + v]) { lastVariant[eventId] = v; return v; }
        }
        lastVariant[eventId] = start;
        return start;
    }
```

- [ ] `rateLimited`: insert at the top `if (eventId.startsWith('ui.')) { if (now - (lastEventAt[eventId] || 0) < 40) return true; lastEventAt[eventId] = now; return false; }` (after `const now = performance.now();`). `ui.*` events never touch `recentVoices`.
- [ ] `playBuffer(buffer, category, options)`:
  - Destination: `options.preview ? master : (categoryGains[category] || master)`.
  - `src.playbackRate.value = options.fx ? 1 + (Math.random() * 0.04 - 0.02) : (Number.isFinite(options.rate) ? options.rate : 1);`.
  - `g.gain.value = options.gain != null ? options.gain : 1;`. Remove the gain jitter.
  - Panner unchanged (`options.pan`).
- [ ] `ensureThemeFxBuffer(fx, themeId, eventId, variant, epoch)`: keep the name, the `themeFxBuffers` cache and the pending map. Render with `requestRender(themeId, eventId, variant, true)`, store `{ result, buffer: null }` and return the entry. Discard the result when the epoch, `document.hidden` or `currentThemeId()` changed (as today).
- [ ] `desktopSound(eventId, options)`, new body:

```js
    async function desktopSound(eventId, options) {
        options = options || {};
        const family = THEME_FX_OF[eventId];
        if (family && !options.fxCoalesced) return queueThemeFx(eventId, family, options);
        if (eventId === 'session.start' && !options.preview) return playSessionStart();
        if (options.silent || document.hidden) return false;
        if (options.sessionRestore || (typeof state !== 'undefined' && state.sessionRestoring)) return false;
        if (!soundsEnabled() && !options.preview) return false;
        const category = EVENT_CATEGORY[eventId];
        if (!category) return false;
        if (!options.preview && !categoryEnabled(category)) return false;   // previews may audition a switched-off category
        if (!options.preview && !unlocked) { wireUnlock(); return false; }
        if (!options.preview && rateLimited(eventId)) return false;
        const epoch = soundEpoch, requestedAt = performance.now();
        try {
            const themeId = options.theme ? resolveThemeId(options.theme) : currentThemeId();
            const fx = family ? THEME_FX[family] : null;
            const fxState = family ? themeFxState[family] : null;
            if (fx && (options.preview || themeId !== fx.theme)) return false;
            const fxOptions = fx ? cleanThemeFxOptions(fx, options) : null;
            if (fx && fxOptions.strength <= 0) return false;
            const variant = fx ? fxOptions.variant : (Number.isInteger(options.variant) ? options.variant : chooseVariant(themeId, eventId));
            const entry = fx ? await ensureThemeFxBuffer(fx, themeId, eventId, variant, epoch) : await ensureEventBuffer(themeId, eventId, variant, true);
            if (!entry || epoch !== soundEpoch || document.hidden || (typeof state !== 'undefined' && state.sessionRestoring)) return false;
            if (options.preview) {
                if (options.previewToken !== previewEpoch || !window.DesktopSoundsPreviewActive) return false;
            } else {
                if (!soundsEnabled() || !unlocked || themeId !== currentThemeId() || !categoryEnabled(category)) return false;
                // A sound never plays out of sync with what the user saw; the buffer stays cached for next time.
                if (!fx && performance.now() - requestedAt > LATE_MS) { stats.dropped++; return false; }
            }
            if (fx) { /* existing expiresAt / windowMs / gapMs / eventGap block, unchanged */ }
            const buffer = audioBufferFor(entry);
            if (!buffer) return false;
            stats.lastLufs = entry.result.lufs;
            stats.lastPeakDb = entry.result.peakDb;
            const gain = fx ? Math.pow(fxOptions.strength, fx.curve) * fx.events[eventId].gain : options.gain;
            const played = playBuffer(buffer, category, { event: eventId, gain, pan: fx ? fxOptions.pan : undefined, fx: !!fx, rate: options.rate, preview: !!options.preview });
            if (played && fx) fxState.played++;
            return played;
        } catch (e) {
            warnOnce('play:' + eventId, 'Desktop sound failed', eventId, e);
            return false;
        }
    }
```

- [ ] Start sound, verbatim:

```js
    let startRequested = false, startSound = '';   // '', 'played', 'skipped-off', 'skipped-autoplay', 'skipped-late'
    async function playSessionStart() {
        if (startRequested) return false;   // once per page load, never later on the first click
        startRequested = true;
        if (document.hidden || !soundsEnabled() || !categoryEnabled('startup') || (typeof state !== 'undefined' && state.sessionRestoring)) { startSound = 'skipped-off'; return false; }
        const ctx = ensureContext(true);
        if (!ctx) { startSound = 'skipped-off'; return false; }
        if (ctx.state !== 'running') {
            try { await Promise.race([ctx.resume(), new Promise(resolve => setTimeout(resolve, 200))]); } catch (e) {}
        }
        if (ctx.state !== 'running') { startSound = 'skipped-autoplay'; return false; }
        unlocked = true;   // the browser lets this page play without a gesture
        const themeId = currentThemeId(), epoch = soundEpoch;
        const entry = await Promise.race([ensureEventBuffer(themeId, 'session.start', 0, true).catch(() => null), new Promise(resolve => setTimeout(() => resolve(null), START_WAIT_MS))]);
        if (!entry || epoch !== soundEpoch || document.hidden || !soundsEnabled() || currentThemeId() !== themeId) { startSound = 'skipped-late'; prewarm(themeId); return false; }
        const buffer = audioBufferFor(entry);
        const played = !!buffer && playBuffer(buffer, 'startup', { event: 'session.start' });
        startSound = played ? 'played' : 'skipped-late';
        prewarm(themeId);
        return played;
    }
```

- [ ] Wire prewarm. In `wireUnlock`'s `unlock`, after `unlocked = true;` add `prewarm(currentThemeId());`. In `syncDesktopSoundSettings`, after `observedSettings = signature;` add `if (soundsEnabled() && unlocked) prewarm(currentThemeId());`. In `closeSoundContext`, add `stopRenders(); prewarmedTheme = ''; cache = { theme: '', entries: {} }; Object.keys(lastVariant).forEach(k => delete lastVariant[k]);`.
- [ ] Previews. Refactor `previewDesktopSound(themeId)` into `runPreview(themeId, ['window.open', 'notify.message', 'window.close'], 'fixed')` and add `previewSoundCategory`:

```js
    async function runPreview(themeId, events, spacing) {
        syncDesktopSoundSettings();
        const token = ++previewEpoch;
        soundEpoch++;
        cancelThemeFx();
        invalidateThemeFxBuffers();
        stopVoices();
        window.DesktopSoundsPreviewActive = true;
        try {
            ensureContext(true);
            if (actx && actx.state === 'suspended') actx.resume().catch(function () {});
            if (!actx) throw new Error('Web Audio unavailable');
            unlocked = true;
            const resolved = resolveThemeId(themeId || currentThemeId());
            const pending = {};
            events.slice().reverse().forEach(id => { pending[id] = ensureEventBuffer(resolved, id, 0, true); });   // first event ends up in front
            const entries = await Promise.all(events.map(id => pending[id]));
            for (let i = 0; i < events.length; i++) {
                if (token !== previewEpoch || document.hidden) return;
                await desktopSound(events[i], { preview: true, theme: resolved, previewToken: token, variant: 0 });
                const seconds = entries[i] && entries[i].result ? entries[i].result.seconds : 0.4;
                await new Promise(r => setTimeout(r, spacing === 'fixed' ? 420 : Math.min(1600, Math.max(300, seconds * 1000 + 120))));
            }
            if (token === previewEpoch) stats.previewPlays++;
        } finally {
            if (token === previewEpoch) {
                window.DesktopSoundsPreviewActive = false;
                if (!soundsEnabled()) await closeSoundContext();
            }
        }
    }
    function previewDesktopSound(themeId) { return runPreview(themeId, ['window.open', 'notify.message', 'window.close'], 'fixed'); }
    function previewSoundCategory(category, themeId) {
        if (CATEGORIES.indexOf(category) < 0) return Promise.reject(new Error('unknown sound category: ' + category));
        return runPreview(themeId, Object.keys(EVENT_VARIANTS).filter(id => EVENT_CATEGORY[id] === category), 'measured');
    }
```

- [ ] `renderTheme`, `renderEvent`, `inspect`:

```js
    function measured(result) { return { seconds: result.seconds, lufs: result.lufs, peakDb: result.peakDb, sampleRate: result.sampleRate, length: result.length != null ? result.length : result.L.length }; }
    async function renderTheme(themeId) {
        const id = resolveThemeId(themeId), events = {};
        for (const eventId of Object.keys(EVENT_VARIANTS)) {
            events[eventId] = [];
            for (let v = 0; v < EVENT_VARIANTS[eventId]; v++) {
                const entry = await ensureEventBuffer(id, eventId, v, true);
                if (!entry) throw new Error('sound render cancelled: ' + id + '/' + eventId);
                events[eventId].push(measured(entry.result));
            }
        }
        return { theme: id, renderer, events };
    }
    async function renderEvent(themeId, eventId, variant) {   // measured values of one sound; used by browser tests
        const id = resolveThemeId(themeId);
        const result = THEME_FX_OF[eventId] ? await requestRender(id, eventId, String(variant), true)
            : ((await ensureEventBuffer(id, eventId, Number(variant) || 0, true)) || {}).result;
        if (!result) throw new Error('sound render cancelled: ' + id + '/' + eventId);
        return Object.assign(measured(result), { theme: id, event: eventId, variant, renderer });
    }
```

  `inspect()`:
  - `cachedTheme: cache.theme`, `renderedEvents: Object.keys(cache.entries).length`.
  - Add `renderer`, `startSound`, `dropped: stats.dropped`, `lastLufs`, `lastPeakDb`.
  - `categories` from `CATEGORIES`.
  - Keep `themeFx`, `plays`, `renders`, `previewPlays`, `lastEvent`, `themes`, `enabled`, `theme`, `volume`, `unlocked`, `bundleLoaded`.
  - `bundleLoaded` only reports the main-thread fallback.
- [ ] Feedback listeners, verbatim (top level of the IIFE):

```js
    const SWITCH_SELECTOR = '.vd-switch > input[type="checkbox"][data-setting-key], #vd-quick-settings input[type="checkbox"][data-quick-setting], #vd-quick-settings input[type="checkbox"][data-quick-dnd]';
    const RANGE_SELECTOR = 'input[type="range"].vd-setting-range[data-setting-key], #vd-quick-settings input[type="range"][data-quick-setting]';
    const DOCK_SELECTOR = '#vd-taskbar-apps .vd-dock-button, #vd-taskbar-apps .vd-task-button';
    let lastTickAt = 0;
    const feedbackSkipped = el => !el || el.disabled || !!el.closest('[data-sound="off"]');
    document.addEventListener('change', event => {
        const el = event.target;
        if (!el || typeof el.matches !== 'function' || !el.matches(SWITCH_SELECTOR) || feedbackSkipped(el)) return;
        desktopSound(el.checked ? 'ui.toggle.on' : 'ui.toggle.off');
    }, true);
    document.addEventListener('input', event => {
        const el = event.target;
        if (!el || typeof el.matches !== 'function' || !el.matches(RANGE_SELECTOR) || feedbackSkipped(el)) return;
        const now = performance.now();
        if (now - lastTickAt < FEEDBACK_GAP_MS) return;
        lastTickAt = now;
        const min = parseFloat(el.min || '0'), max = parseFloat(el.max || '100'), value = parseFloat(el.value);
        const pos = max > min && Number.isFinite(value) ? Math.min(1, Math.max(0, (value - min) / (max - min))) : 0.5;
        desktopSound('ui.tick', { rate: Math.pow(2, (pos * 2 - 1) / 12) });   // pitch follows the value within ±1 semitone
    }, true);
    document.addEventListener('click', event => {
        const el = event.target && typeof event.target.closest === 'function' ? event.target.closest(DOCK_SELECTOR) : null;
        if (el && !feedbackSkipped(el)) desktopSound('ui.select');
    }, true);
```

- [ ] Export: `window.DesktopSounds = { play: desktopSound, preview: previewDesktopSound, previewCategory: previewSoundCategory, renderEvent, inspect, renderTheme, close: closeSoundContext };`. Keep the other `window.*` exports and the `applyDesktopSettings` wrapper.
- [ ] `sdk-events-bootstrap.js`. Replace the three `desktopSound` lines in `showDesktopNotification` with `if (delivery.sound) desktopSound(notificationSoundEvent(payload));` and add, directly above it:

```js
    // MeshCore messages are conversations; otherwise the payload type picks the notice (unknown types are info).
    function notificationSoundEvent(payload) {
        if (payload.appId === 'meshcore') return 'notify.message';
        const type = String(payload.type || '').toLowerCase();
        if (type === 'error') return 'notify.error';
        if (type === 'warning' || type === 'warn') return 'notify.warning';
        if (type === 'success') return 'notify.success';
        return 'notify.info';
    }
```

  In `init()` add a new line `desktopSound('session.start');` directly after `openInitialDesktopApp();` (`:1020`).
- [ ] Run `node --check ui/js/desktop/core/sound-runtime.js` and `node --check ui/js/desktop/core/sdk-events-bootstrap.js`. Expected: no output. Browser verification follows in Task 15, after the rebuild in Task 13.
- [ ] Commit: `git add ui/js/desktop/core/sound-runtime.js ui/js/desktop/core/sdk-events-bootstrap.js` then `git commit -m "feat(desktop-sounds): worker rendering, variants, limiter, start sound and touch feedback in the runtime"`.

### Task 8: Bundle builder wiring

**Files:**
- Modify: `scripts/build-ui-bundles.js:1-5, :166-182`

- [ ] Add `import { desktopSoundParts } from './desktop-sound-bundle-parts.js';` after the storymaker import (`:4`). Replace the desktop-sounds entry with `{ out: 'ui/js/desktop/bundles/desktop-sounds.bundle.js', parts: desktopSoundParts },`.
- [ ] Run `node --check scripts/build-ui-bundles.js`. Expected: no output. Do **not** build (theme files are incomplete until Stage 3).
- [ ] Commit: `git add scripts/build-ui-bundles.js` then `git commit -m "build(desktop-sounds): bundle parts from the shared list"`.

**Stage 2 review (coordinator):** one combined review of Tasks 4–8 (Critical/Important only).

---

## Stage 3 — Themes (four parallel groups)

### Theme contract (applies to Tasks 9–12)

- **Ownership:** a theme task edits only its own `theme-<id>.js` files. Do not edit the kernel, instruments, engine or tools. If you need an instrument feature that does not exist, write it into the report; do not patch it.
- **Format:** `(function () { 'use strict'; self.DesktopSoundEngine.defineTheme({ id, key, space, voices, events, fx }); })();`, pure data as in spec §6.5 and the crystal reference (Task 3).
  - Notes use pitched voices and an integer `n` relative to `key`. Layers use unpitched voices (`noise`, `debris`, untuned `click`).
  - A note may override voice parameters. `space: null` makes a sound dry. A sound may add `chorus`, `echo`, `drive`, `tone`.
  - All 26 events are written out; nothing is generated. `fx: {}` unless the theme owns effects.
- **Grammar (spec §8):**
  - `window.open`/`close`, `window.restore`/`minimize`, `menu.open`/`close`, `dialog.confirm`/`cancel` and `ui.toggle.on`/`off` are mirrored pairs built from the same notes (rise vs fall).
  - `window.maximize` is the open gesture with a larger interval and more room.
  - `window.snap` is one dry tick (`space: null`), the shortest window sound.
  - `window.deny` is one muted low knock.
  - Notices: info = two rising notes; message = the theme's three-note signature motif; success = rising, ending on the tonal centre; warning = the same note twice, unresolved; error = two low falling notes a semitone or tritone apart, soft attack (6–12 ms), never a buzzer.
  - Files: drop = soft low damped landing; trash = short falling flick with a little noise; delete = lower and more final than trash.
  - `space.switch` = a short swell of the theme's texture.
  - Touch: toggle = two-tone tick (up for on, down for off); tick = the shortest click; select = a soft press.
  - `session.start` = 4–5 notes stating the message motif in the theme's fullest room.
  - Harmony: all tonal content in the theme's key and scale. Only `notify.error` may leave the scale (semitone or tritone).
  - Spectrum: main energy 250 Hz–5 kHz, nothing below 110 Hz, air only as a faint layer. Tonal attacks 2–8 ms; ticks are shaped transients.
- **Identity (spec §9), with pitch sets as semitones above `key`:**

| Theme | key (MIDI) | Pitch set | Lead instruments · room |
| --- | --- | --- | --- |
| crystal | 81 (A5) | 0,2,4,7,9 | glass chime, noise air, glass click · bright hall 0.85 s |
| wood | 65 (F4) | 0,2,4,7,9 | bar marimba (rosewood), felt click, thud · small dry room 0.3 s |
| analog | 62 (D4) | 0,2,3,5,7,9,10 | analog pluck and sweep, sub · plate 0.6 s, chorus |
| workshop | 64 (E4) | 0,5,7 | relay click, metal ping, noise hiss, blip servo glide · tight 0.35 s |
| water | 67 (G4) | 0,2,4,7,9 | drop, drop clusters, low-passed noise wash · wet cave 0.8 s, damped |
| amiga | 72 (C5) | 0,2,4,5,7,9,11 | chip pulse and triangle · dry, tiny slap echo |
| cyberpunk | 57 (A3) | 0,1,3,5,7,8,10 | analog pulse bass, metal, static noise · dark short plate, echo |
| black_gloss | 73 (D♭5) | 0,2,4,5,7,9,11 (sustained/overlapping notes on 0,2,4,7) | glass (darker tilt, slower attack), soft bell · smooth hall 1.0 s |
| crazy | 64 (E4) | whole-tone 0,2,4,6,8,10 and Lydian 0,2,4,6,7,9,11 | drop and blip with boing glides, toy bar · short, springy |
| lollipop | 82 (B♭5) | 0,2,4,5,7,9,11 | bell celesta, high drop pop, glockenspiel bar · small bright room |
| organic | 55 (G3) | 0,3,5,7,10 | bar kalimba and log drum (woodblock), string, breath noise, soft drop · soft room 0.45 s |
| electronic_dreams | 57 (A3) | 0,3,7,10 (passing 2,5,8) | copper click, mellow analog pulse, bell · tape echo, plate |
| windows_xp | 63 (E♭4) | 0,2,4,5,7,9,11 | bell tine (electric piano), pad swell, soft click · medium hall 0.8 s (own motifs, no recreated system sound) |
| green_slime | 53 (F3) | 0,3,6,9 | squelch, thick low drop, wet thud · damp and short (see Deviations) |
| voxel_world | 60 (C4) | 0,2,4,7,9 in plain steps | woodblock bar, stone click, earth thud · dry |
| orbit | 64 (E4) | 0,2,4,6,7,9,11 | blip sonar with echo, noise whoosh and hiss, pad · deep space, long soft tail, echo |

- **Theme effects (spec §10).** No `space`; an effect may carry its own `echo`.
  - voxel_world: `fx['window.fracture']` and `fx['window.impact']`, each with `wood`, `stone`, `earth`, ≤ 0.7 s. Fracture is debris-led; impact is one heavier hit plus a few grains.
  - green_slime: `fx['slime.pop']` and `fx['slime.snap']`, each with `small`, `medium`, `large`. Pop ≤ 0.18 s, snap ≤ 0.26 s (browser test). Larger = lower and longer.
  - orbit: `fx['orbit.ping']` with `small`, `medium`, `large`, each 0.35–1.0 s, sonar blip with its own echo.
- **Commands** (per theme):
  - `node scripts/test-desktop-sounds.mjs --theme <id>`: must exit 0.
  - `node scripts/render-desktop-sounds.mjs --theme <id> --out C:\disposable\desktop-sounds\render --json > C:\disposable\desktop-sounds\render\<id>\rows.json`
- **Measured gates:** all of spec §12, enforced by the test.
- **Report back per theme:** a levels table (event, variants, seconds, lufs, deltaLU, peakDb, centroidHz, attackMs, from the JSON rows), the contact-sheet path `C:\disposable\desktop-sounds\render\<id>\<id>-contact-sheet.wav`, the voices used, open questions, and any missing instrument feature.
- **Commit per theme:** `git add ui/js/desktop/sound/theme-<file>.js` then `git commit -m "feat(desktop-sounds): <id> theme"`.

### Task 9: Theme group A — black_gloss, wood, lollipop, organic

**Files:**
- Create: `ui/js/desktop/sound/theme-black-gloss.js`, `ui/js/desktop/sound/theme-lollipop.js`, `ui/js/desktop/sound/theme-organic.js`
- Replace: `ui/js/desktop/sound/theme-wood.js`

- [ ] black_gloss: write the theme to the contract, run the test and the render command, commit.
- [ ] wood: same.
- [ ] lollipop: same.
- [ ] organic: same.
- [ ] Run `node scripts/test-desktop-sounds.mjs --theme black_gloss,wood,lollipop,organic`. Expected: `ok (4 theme(s))`. Send the report.

### Task 10: Theme group B — analog, electronic_dreams, cyberpunk, amiga, windows_xp

**Files:**
- Create: `ui/js/desktop/sound/theme-cyberpunk.js`, `ui/js/desktop/sound/theme-amiga.js`
- Replace: `ui/js/desktop/sound/theme-analog.js`, `ui/js/desktop/sound/theme-electronic-dreams.js`, `ui/js/desktop/sound/theme-windows-xp.js`

- [ ] analog: write, test, render, commit.
- [ ] electronic_dreams: same.
- [ ] cyberpunk: same.
- [ ] amiga: same.
- [ ] windows_xp: same.
- [ ] Run `node scripts/test-desktop-sounds.mjs --theme analog,electronic_dreams,cyberpunk,amiga,windows_xp`. Expected: `ok (5 theme(s))`. Send the report.

### Task 11: Theme group C — water, crazy, green_slime (+ effects)

**Files:**
- Create: `ui/js/desktop/sound/theme-crazy.js`
- Replace: `ui/js/desktop/sound/theme-water.js`, `ui/js/desktop/sound/theme-green-slime.js`

- [ ] water: write, test, render, commit.
- [ ] crazy: same.
- [ ] green_slime: 26 events plus `fx` `slime.pop` and `slime.snap` (small, medium, large); write, test, render, commit.
- [ ] Run `node scripts/test-desktop-sounds.mjs --theme water,crazy,green_slime`. Expected: `ok (3 theme(s))`. Send the report, including effect lengths.

### Task 12: Theme group D — workshop, voxel_world (+ effects), orbit (+ effects)

**Files:**
- Replace: `ui/js/desktop/sound/theme-workshop.js`, `ui/js/desktop/sound/theme-voxel-world.js`, `ui/js/desktop/sound/theme-orbit.js`

- [ ] workshop: write, test, render, commit.
- [ ] voxel_world: 26 events plus `fx` `window.fracture` and `window.impact` (wood, stone, earth); write, test, render, commit.
- [ ] orbit: 26 events plus `fx` `orbit.ping` (small, medium, large, 0.35–1.0 s each); write, test, render, commit.
- [ ] Run `node scripts/test-desktop-sounds.mjs --theme workshop,voxel_world,orbit`. Expected: `ok (3 theme(s))`. Send the report.

**Stage 3 review (coordinator):** one combined review of the four groups. Run `earshot` on every contact sheet and the five most used events (`window.open`, `window.close`, `menu.open`, `notify.info`, `ui.toggle.on`): material identity, nothing harsh or cheap-beep. Hand one contact sheet per theme to the user. Feedback changes theme data only.

---

## Stage 4 — Integration, tests, docs

### Task 13: Remove the old synthesizer, rebuild bundles, CI step

**Files:**
- Delete: `ui/js/desktop/sound/synth-core.js`, `ui/js/desktop/sound/theme-desktop.js`
- Regenerate: `ui/js/desktop/bundles/desktop-sounds.bundle.js`, `ui/js/desktop/bundles/main.bundle.js`
- Modify: `.github/workflows/web-assets.yml:27`

- [ ] `git rm ui/js/desktop/sound/synth-core.js ui/js/desktop/sound/theme-desktop.js`.
- [ ] Run `npm run test:desktop-sounds`. Expected: three `ok` lines, `desktop sound gates: ok (16 theme(s))`.
- [ ] Run `npm run build:ui`, then `npm run check:ui`. Expected: `Checked …desktop-sounds.bundle.js (20 parts)` and no `Outdated`.
- [ ] Run `node -e "const s=require('fs').readFileSync('ui/js/desktop/bundles/desktop-sounds.bundle.js','utf8'); if(/Math\.random\(|DesktopSoundSynth/.test(s)) process.exit(1)"`. Expected: exit 0.
- [ ] In `web-assets.yml`, after `- run: node scripts/build-ui-bundles.js --check`, add `- run: npm run test:desktop-sounds`.
- [ ] Commit: `git add -u ui/js/desktop/sound ui/js/desktop/bundles/desktop-sounds.bundle.js ui/js/desktop/bundles/main.bundle.js .github/workflows/web-assets.yml` then `git commit -m "build(desktop-sounds): remove the old synthesizer, rebuild bundles, run sound gates in CI"`.

### Task 14: Go contract test rewrite (`ui/desktop_sound_settings_test.go`)

**Files:**
- Modify: `ui/desktop_sound_settings_test.go`

- [ ] `desktopSoundEvents`: **change** to the 26 ids in spec order (same list as `EVENTS`).
- [ ] `desktopSoundLocaleKeys`: **change**; append `desktop.settings_sound_interface`, `desktop.settings_sound_interface_desc`, `desktop.settings_sound_startup`, `desktop.settings_sound_startup_desc`, `desktop.settings_sound_preview_category`.
- [ ] `TestDesktopSoundRuntimeMarkers`: **change**.
  - Main-bundle markers: keep all 7 and add `"function previewSoundCategory("`, `"renderEvent"`, `"new Worker("`, `"DesktopSoundsRenderGate"`.
  - Runtime checks: keep.
  - Add: for each of the 26 events, the runtime contains `'<id>': '<category>'` (from a Go map of the spec §7 categories) and `'<id>': <variants>`.
  - Bundle markers: replace `"window.DesktopSoundSynth"`, `"window.DesktopSoundThemes"` and the 10 `"DesktopSoundThemes.<id>"` markers with `"root.DesktopSoundDSP"`, `"root.DesktopSoundInstruments"`, `"root.DesktopSoundEngine"`, `"/* ui/js/desktop/sound/render-host.js */"` and `"id: '<id>'"` for all 16 ids. Assert the bundle contains neither `"DesktopSoundSynth"` nor `"Math.random("`.
- [ ] `TestDesktopSoundHookMarkers`: **change** the `sdk-events-bootstrap.js` entry from `desktopSound('notify.message')`, `desktopSound('notify.error')`, `desktopSound('notify.info')` to `"function notificationSoundEvent("`, `"'notify.message'"`, `"'notify.error'"`, `"'notify.warning'"`, `"'notify.success'"`, `"'notify.info'"`, `"desktopSound('session.start')"`. All other files: keep.
- [ ] `TestDesktopSoundThemeEvents`: **change**.
  - Loop over the 16 files (`crystal, wood, analog, workshop, water, amiga, cyberpunk, black-gloss, crazy, lollipop, organic, electronic-dreams, windows-xp, green-slime, voxel-world, orbit`). Each must contain `"self.DesktopSoundEngine.defineTheme({"`, `"id: '<id>'"` and every one of the 26 event keys (same regex).
  - Delete the `composeDesktopEvents` checks and the orbit `"reverb:"` check.
  - Voxel markers: `"fx:"`, `"'window.fracture'"`, `"'window.impact'"`, `"wood:"`, `"stone:"`, `"earth:"`.
  - Orbit markers: `"fx:"`, `"'orbit.ping'"`, `"small:"`, `"medium:"`, `"large:"` (drop `"window.DesktopSoundThemes.orbit"` and `"specialFx"`).
- [ ] `TestDesktopThemeFxSoundRuntimeContract`: **keep** unchanged (every literal still exists).
- [ ] `TestGreenSlimeSoundFxContract`: **change** the bank marker `"specialFx"` to `"fx:"`; delete the `"reverb:"` negative check (the engine rejects `space` in effects); keep the bridge and simulation markers.
- [ ] `TestDesktopSoundSettingsUIMarkers`: **change**. Add catalog markers `"settingToggle('sound.interface'"` and `"settingToggle('sound.startup'"`, settings marker `"data-sound-category-preview"` and CSS marker `".vd-sound-category-actions"`. Keep the rest.
- [ ] `TestDesktopSoundTranslations`: **keep** (it uses the extended key list).
- [ ] Run `go test ./ui -run '(?i)(TestDesktopSound|TestDesktopThemeFxSoundRuntimeContract|TestGreenSlimeSoundFxContract)'`. Expected: PASS for all 7 (count the PASS lines).
- [ ] Commit: `git add ui/desktop_sound_settings_test.go` then `git commit -m "test(desktop-sounds): contract markers for the new sound system"`.

### Task 15: Browser test rewrites

**Files:**
- Modify: `ui/desktop_sound_browser_test.go`, `ui/desktop_theme_sensory_browser_test.go:99-121`
- Test (run only): `ui/desktop_theme_green_slime_interaction_browser_test.go`, `ui/desktop_theme_orbit_browser_test.go`, `ui/desktop_settings_layout_browser_test.go`

`TestDesktopSoundBrowser`, step by step:
- [ ] **Change** `Timeout(60 * time.Second)` → `Timeout(240 * time.Second)`.
- [ ] **Change** the `/fixture` handler to set `Content-Security-Policy` to the production main-UI policy without the external connect origins: `default-src 'self'; script-src 'self' 'unsafe-inline' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https:; font-src 'self' data:; connect-src 'self' blob: ws: wss:; media-src 'self' data: blob: http: https:; worker-src 'self' blob:; object-src 'none'; form-action 'self'; frame-src 'self' http: https:; frame-ancestors 'none'; base-uri 'self';`. Add `renderEvent:(t,e,v)=>DesktopSounds.renderEvent(t,e,v)` to `window.soundTest`.
- [ ] **Change** the quality block (`:129-173`): for each of the 16 themes and 26 events, `renderEvent(theme, event, 0)`. Assert:
  - 16 themes × 26 events;
  - `peakDb <= -2.9`;
  - `seconds <=` the Go map of `maxSeconds`;
  - `lufs` within ±0.6 of the Go map of targets, or below it;
  - the first result's `renderer == "worker"` (this is the CSP confirmation).
  Delete `rmsMin` (0.0005), `peakMax` (0.891) and `durMax` (1.5).
- [ ] **Keep** the play-counter, category-off and `renderTheme('water')`/`cachedTheme == "water"` checks (`:175-207`).
- [ ] **Change** the voxel block:
  - delete the `cachedTheme != "water"` assertion in the loop (`:234-236`); prewarm now caches the active theme;
  - keep the burst literals (`plays` 1..4, `late` 0);
  - at `:245` replace the `DesktopSoundSynth.renderLayers` wrapper with `window.DesktopSoundsRenderGate=new Promise(r=>setTimeout(r,400));` … `window.DesktopSoundsRenderGate=null;`, keeping the "stale impact" intent.
- [ ] **Change** the slime block:
  - delete the `cachedTheme != "water"` assertion (`:319-321`); keep `themeFx.slime.rendered == 1` and the rate-limit literals (burst 1, stream 2–3, snaps 1);
  - rewrite the bank measurement (`:370-415`) with `renderEvent('green_slime','window.open',0)` and `renderEvent('green_slime',event,variant)`. Assert per item: `peakDb <= -2.9`, `lufs + 20·log10(gain) − open.lufs` in [−10, −4] dB, `seconds` in [0.03, 0.18] for pop and [0.03, 0.26] for snap. Delete the 0.3–0.5 peak-ratio and 0.1–0.45 loudness-ratio checks.
- [ ] **Change** the orbit block: keep the rejection, acceptance and rate-limit literals. Rewrite the bank (`:504-541`) the same way: relative loudness in [−10, −4] dB, `seconds` in [0.35, 1.0], `peakDb <= -2.9`. Delete the 0.3–0.5 and 0.1–0.5 ratio checks and the 0.35–0.85 audible window.
- [ ] **Keep** the sounds-off, Settings screenshots and preview checks (`previewDesktopSound('crystal')` plays exactly 3; `previewPlays >= 1`). **Add** a check that `document.querySelectorAll('[data-sound-category-preview]').length == 6`, and that `DesktopSounds.previewCategory('interface','crystal')` raises `plays` by 4.
- [ ] **Add** a fallback check. Navigate again (`waitBoot`). Run `window.Worker=class{constructor(){throw new Error('blocked')}}`, then `await DesktopSounds.renderEvent('wood','window.open',0)`. Assert `renderer == "main"` and `lufs` finite.
- [ ] **Add** a silent-worker check. Navigate again. Set `window.Worker=class{postMessage(){} terminate(){}}` and time `renderEvent('wood','menu.open',0)`. Assert it resolves in 3.9–10 s with `renderer == "main"`.
- [ ] **Add** a start-sound check. Set `settings["sound.enabled"]="true"` in the Go map and navigate again. Wait up to 3 s for `inspect().startSound != ""`. Assert it is `played` or `skipped-autoplay`; if `played`, `lastEvent == "session.start"`. Call `desktopSound('session.start')` again and assert `plays` is unchanged. Dispatch a `pointerdown`, wait 300 ms and assert `lastEvent` is not `session.start`.

Other tests:
- [ ] `desktop_theme_sensory_browser_test.go:103-110`: **change** `const original=DesktopSoundSynth.renderLayers; … DesktopSoundSynth.renderLayers=async(...args)=>{await gate; return original(...args);};` to `window.DesktopSoundsRenderGate=gate;`, and `DesktopSoundSynth.renderLayers=original;` to `window.DesktopSoundsRenderGate=null;`. Everything else stays.
- [ ] `TestDesktopGreenSlimeInteractionBrowser`, `TestDesktopOrbitBrowser`, `TestDesktopSettingsLayoutBrowser`: **keep**. Run them to confirm the unchanged `DesktopSounds.play/inspect` contract and that the new row buttons fit.
- [ ] Run `$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./ui -run '(?i)(TestDesktopSoundBrowser|TestDesktopThemeSensoryBrowser|TestDesktopGreenSlimeInteractionBrowser|TestDesktopOrbitBrowser|TestDesktopSettingsLayoutBrowser)' -timeout 30m`. Expected: 5 PASS lines. No new test functions are added, so `TestUIBrowserCIShardsCoverEveryTest` is unaffected.
- [ ] Commit: `git add ui/desktop_sound_browser_test.go ui/desktop_theme_sensory_browser_test.go` then `git commit -m "test(desktop-sounds): browser checks for worker rendering, fallback, start sound and new levels"`.

### Task 16: Documentation (docs-agent)

**Files:**
- Modify: `documentation/desktop-themes.md:52-137` (also check the references at `:448` and `:504`), `ui/js/desktop/apps/AGENTS.md:2400-2423`

- [ ] `desktop-themes.md`, rewrite "Matching cursors and sounds" and "Effect sounds":
  - the layers (kernel, instruments, engine, themes, render host, runtime) and the Worker with main-thread fallback;
  - the 26 events with the role table of spec §7;
  - the categories, including the `sound.interface` and `sound.startup` switches and the per-category preview;
  - the theme data format (`defineTheme`, notes vs layers, overrides, `space: null`), variants and round-robin, the late-drop and start-sound rules;
  - effect families and that their runtime gates are unchanged;
  - the tools (`npm run test:desktop-sounds`, `node scripts/render-desktop-sounds.mjs --theme <id> --out <dir>`).
  Replace any `specialFx`/`DesktopSoundSynth` wording at `:448`/`:504`.
- [ ] `apps/AGENTS.md` "Desktop UI sound contract":
  - persisted keys now include `sound.interface`, `sound.startup`;
  - 26 events and 6 categories;
  - Worker rendering and the `window.DesktopSoundsRenderGate` test seam;
  - touch feedback selectors and `data-sound="off"`;
  - remove "all five themes";
  - verification list: `npm run test:desktop-sounds`, the Go tests of Task 14, the browser tests of Task 15.
- [ ] Commit: `git add documentation/desktop-themes.md ui/js/desktop/apps/AGENTS.md` then `git commit -m "docs(desktop-sounds): sound system v2 architecture, events, themes and tools"`.

### Task 17: Final verification (test-agent)

**Files:** none (verification only)

- [ ] `npm run test:desktop-sounds`: expect 3× ok and 16 themes.
- [ ] `npm run check:ui`: expect no `Outdated`.
- [ ] `go test ./internal/desktop -run '(?i)(TestServiceSettings)'` and `go test ./internal/server -run '(?i)(TestSecurityHeaders)'`: expect PASS.
- [ ] `go test ./ui -run '(?i)(TestDesktopSound|TestDesktopThemeFxSoundRuntimeContract|TestGreenSlimeSoundFxContract|TestDesktopSettings|TestDesktopFruity|TestDesktopScreensaver)'`: expect PASS.
- [ ] `go test ./internal/webassets ./cmd/assetpack`: expect PASS.
- [ ] Browser run of Task 15: expect 5 PASS.
- [ ] `node scripts/render-desktop-sounds.mjs --theme all --out C:\disposable\desktop-sounds\render\final`: report the 16 contact-sheet paths for the user.
- [ ] Coordinator: run GitNexus `detect-changes --scope compare --base-ref main` from the main checkout before merging (root CLAUDE.md).

---

## Deviations from the spec

1. **Placeholder syntax.** Spec §11 says `{name}`; the desktop `t()` interpolates `{{name}}` (`ui/js/shared/shared-core.js:13-23`, for example `desktop.window_group_count`). All locales use `{{name}}`.
2. **Variant level deviation ±0.7 dB → ±0.4 dB, applied after mastering.** Before mastering, the loudness normalization would erase it. ±0.7 dB after mastering would break the ±0.5 LU gate of §12. Peak-limited sounds only get negative offsets.
3. **green_slime "F2–F4".** F2 (87 Hz) breaks "nothing below 110 Hz" and the 90 Hz gate. The key is F3 (53). Sounding fundamentals stay ≥ 130 Hz (`squelch` ≥ 80 Hz, whose fundamental the band-pass and the 110 Hz low cut remove), so the F2 register is heard through the squelch's harmonics.
4. **Note floor 130 Hz** (validation, per model `minFreq`) instead of a bare 110 Hz rule. A fundamental at 110 Hz cannot pass the "30 dB under the total below 90 Hz" gate after pitch drops and bubble rises.
5. **Crystal example.** The spec's `window.open` uses `n: 5` (D6), which is outside A major pentatonic. The reference theme uses `n: -5 → 0` (E5 → A5), still a rising fourth.
6. **Theme-card preview.** The code plays `notify.info`, while spec §11 says it "keeps" `notify.message`. The runtime now plays `notify.message` (the signature motif); still three sounds.
7. **Previews bypass category switches** and route to the master gain, so a switched-off category can be auditioned from its own row. The spec is silent on this.
8. **Master length cap.** The master hard-caps at the role maximum and reports `naturalSeconds`. The gate asserts the natural length, so authors fix tails instead of relying on the cut.
9. **Late drop applies to shell events only.** Theme effects keep their existing 240 ms expiry (`THEME_FX`, unchanged per §2).
10. **"Dock icon" includes the standard taskbar buttons** (`.vd-task-button`). Outside Fruity the taskbar is the dock.
11. **Reverb `damping`** now means the frequency at which RT60 is halved (Jot per-line damping), no longer a one-pole cutoff. The spec's values (4500–7000 Hz) keep their character.
12. **Theme render time budget** is 6000 ms per theme including all variants and effects (env `DESKTOP_SOUNDS_THEME_BUDGET_MS`). The spec gives no number.
13. **Notification precedence** keeps the current order: MeshCore before `type`.
