# Desktop sound system v2 — design

Date: 2026-11-23 · Status: design written after brainstorming (scope, character and new events chosen by the user);
written spec awaiting user review. Replaces the synthesizer and all sound themes introduced by `4d677b76f`
(opt-in synthesized UI sound system) and its follow-ups. Runtime rules that stay in force are described in
`documentation/desktop-themes.md` ("Matching cursors and sounds", "Effect sounds").

## 1. Goal

Rework the desktop sound system so that every sound theme consists of finely tuned, high-quality synthetic sounds:

- a new synthesis core that can produce convincing materials (glass, wood, metal, strings, water, analogue and FM
  synths) with real reverb tails, instead of a few oscillator layers with a gated noise convolver;
- all 16 sound themes composed anew on that core, each with its own instruments, key and room, all following one
  shared sound language so that the meaning of a sound is the same in every theme;
- consistent loudness: every event has a loudness target by role, so a menu tick is never as loud as a notification
  and switching themes does not change the perceived volume;
- seven new events: a start sound, success and warning notices, and quiet click feedback;
- sounds that can be rendered, measured and tested outside the browser.

Character chosen by the user: **subtle and refined**. Frequent events are short and quiet, notices and dialogs are a
little more melodic, the theme's material stays audible without pushing itself forward.

## 2. Non-goals

- Recorded samples or shipped audio files. Everything stays synthesized at run time.
- New sound theme ids, or a different pairing of desktop themes and sound themes.
- Spatial placement of window sounds by window position (the shell API stays `desktopSound(eventId)`).
- A logout sound, per-app sounds, or sounds inside apps.
- A sound editor in the UI.
- Changes to the theme effect gates and rate limits (`THEME_FX` in `sound-runtime.js`).

## 3. Decisions taken with the user

| Question | Decision |
| --- | --- |
| Scope | All 16 sound themes are composed anew; the old synthesizer is removed completely. |
| Character | Subtle and refined (see §1). |
| New events | Start sound, success and warning notices, click feedback for switches, sliders and dock icons. |

## 4. What is wrong today

Findings from reading `ui/js/desktop/sound/synth-core.js`, `ui/js/desktop/core/sound-runtime.js` and the theme files:

1. **Reverb tails are cut off.** `wireReverb` puts the convolver *before* the amplitude envelope, so the envelope
   gates the reverb. The impulse is `Math.random()` noise of 0.18–0.55 s, different on every render.
2. **No loudness management.** `normalizeBuffer` peak-normalizes every event to −3 dBFS. A menu tick and a
   notification end at the same peak; the `gain` values in the themes only balance layers within one sound.
3. **Detuned playback.** `playBuffer` varies `playbackRate` by ±3 %, which is ±51 cents: chimes and melodic notices
   sound randomly out of tune.
4. **Weak sources.** `pluck` is a decaying noise burst, not a string. "Pink" noise is white noise with a linear
   fade. FM index is a fixed frequency deviation in Hz. Square and saw waves are not shaped for short UI sounds.
5. **Generic themes.** Eight of the 16 themes (`amiga`, `cyberpunk`, `black_gloss`, `crazy`, `lollipop`, `organic`
   in `theme-desktop.js`, plus `electronic_dreams` and `windows_xp`) and the shell events of `green_slime` and
   `voxel_world` are generated from a small profile: the same voice plays different note lists for every event.
6. **Fixed 1.2 s buffers** for every event, including 80 ms clicks, and all 19 events are rendered before the first
   one plays.
7. **Master compressor** at −18 dB / 3:1 with default timing works on every single sound.
8. **Only testable in a browser.** The synth builds Web Audio node graphs in an `OfflineAudioContext`, so sounds
   cannot be rendered, measured or compared in Node, and the render is not deterministic.

## 5. Approach

Three approaches were considered:

- **A. New sample-level core in plain JavaScript (chosen).** Sounds are computed directly into `Float32Array`s.
  The same code runs in the browser (in a Web Worker) and in Node. Deterministic through a seeded random
  generator. Allows modal synthesis, a real string model, proper filters, a feedback-delay-network reverb and
  loudness metering. Cost: a new core and all themes written anew.
- **B. Keep the Web Audio node graph and repair it** (reverb routing, envelopes, noise, loudness). Cheaper, but the
  building blocks stay limited (no modal or physical models without an AudioWorklet), renders differ between
  browsers, and offline measurement needs a headless browser.
- **C. Pre-rendered audio files.** Full control, but binary assets in the repository and in the server binary, no
  variation at run time, and harder to retune.

A feasibility spike of approach A (outside the repository, `C:\disposable\desktop-sounds\spike`) rendered nine demo
sounds in Node:

- 11–56 ms per sound (0.2–1.5 s long, including the reverb tail), unoptimized;
- every sound hits its loudness target exactly and stays below the peak ceiling;
- the listening models of `earshot` read the materials as intended: the crystal notice as "a success chime"
  (z 4.5) and "a notification ding", the wood sound as "a kalimba note" / "a marimba" / "a wooden block hit", the
  water sound as "a water drop" / "a liquid bloop" (AudioSet "Plop" 0.87).

The spike is a proof of the method, not tuned sound design; the spike code is a starting point, not the final core.

## 6. Architecture

### 6.1 Files

All under `ui/js/desktop/sound/`, shipped in the lazy bundle `desktop-sounds.bundle.js` in this order:

| File | Responsibility |
| --- | --- |
| `dsp-kernel.js` | Primitives on `Float32Array`: seeded random generator, envelopes, oscillators, resonators, noise, filters, reverb, chorus, echo, saturation, loudness and true-peak meters. No knowledge of events or themes. |
| `instruments.js` | Named instrument models built from kernel primitives (§6.3). Each is `render(params, note, rand) → mono or stereo buffer`. |
| `sound-engine.js` | Event catalogue and roles (§7), theme registry, validation of theme data, the render pipeline (§6.4), variants (§6.6). Public API `DesktopSoundEngine`. |
| `theme-<id>.js` × 16 | One theme each: pure data (§6.5). `theme-desktop.js` is split into six files. |
| `render-host.js` | Last part of the bundle. In a Worker it answers render requests; in a window it does nothing. |

Removed: `synth-core.js` (`DesktopSoundSynth`, `composeDesktopEvents`, the layer kinds `tone`/`fm`/`noise`/`pluck`/
`sweep`/`arp`).

All files attach to `self`, not `window`, so the bundle loads unchanged in a window and in a Worker. Kernel,
instruments and engine also export through `module.exports` when present, for the Node tools (§12).

`ui/js/desktop/core/sound-runtime.js` stays the only part in the main bundle. It keeps playback, settings, unlock,
rate limits, theme effect gates and the public shell API, and gains the Worker client, the variant choice, the new
events and the feedback listeners (§6.7, §6.8).

### 6.2 DSP kernel

- **Sources:** damped-sine resonator bank (modal synthesis, two-pole recurrence); FM operator stack with index
  envelopes and feedback; band-limited saw, pulse and triangle (PolyBLEP) with unison detune and drift;
  Karplus-Strong string with tuned fractional delay and loss filter; bubble model (decaying sine with rising
  pitch); white, pink (filtered) and brown noise.
- **Shaping:** envelopes with raised-cosine attack and exponential decay and release (times given as −60 dB
  times); zero-delay-feedback state variable filter with a cutoff trajectory; shelving filters; second-order low
  cut; `tanh` saturation.
- **Space:** stereo feedback delay network reverb (8 lines, Householder feedback, per-line damping, input
  diffusion, pre-delay, low cut on the wet signal). The tail is part of the rendered sound and extends its length.
  Stereo chorus; filtered feedback echo.
- **Meters:** maximum momentary loudness after ITU-R BS.1770 (K-weighting, 400 ms window, 10 ms hop); true peak by
  4× oversampling.
- **Determinism:** no `Math.random()`; every random value comes from a generator seeded per theme, event and
  variant. The same request always gives the same samples.

Sample rate is fixed at 48 kHz. Buffers are played through an `AudioBuffer` of 48 kHz, which the browser resamples
if the output device runs at another rate.

### 6.3 Instruments

Instrument models are code; themes only choose a model and set its parameters.

| Model | Sounds like | Used for |
| --- | --- | --- |
| `glass` | Glass and crystal chimes; partials with a slow beating twin that sits on the other stereo side | Crystal, Black Gloss |
| `bar` | Tuned bars: marimba, xylophone, glockenspiel, kalimba, wood block (partial set, damping, mallet noise) | Wood, Organic, Lollipop, Voxel World |
| `bell` | FM bell, celesta, electric-piano tine | Lollipop, Windows XP, Electronic Dreams |
| `string` | Plucked string (nylon, harp, muted) | Organic, Wood |
| `analog` | Subtractive synth: detuned saws or pulse, sub oscillator, low-pass with envelope, drift, soft drive | Analog, Electronic Dreams, Cyberpunk |
| `chip` | Band-limited pulse and triangle with duty cycle, fast arpeggio and vibrato, mild bit reduction | Amiga |
| `drop` | Water drop or bubble, single or as a small cluster | Water, Crazy, Green Slime |
| `blip` | Pure tone with a pitch envelope (sonar ping, sweep, boing) | Orbit, Crazy |
| `pad` | Soft swell of a detuned partial cluster, slow attack | Start sounds, Orbit, Windows XP |
| `metal` | Inharmonic ring of plates and pipes | Workshop, Cyberpunk |
| `click` | Shaped transient: a few milliseconds of filtered noise plus a damped resonance (felt, relay, plastic, glass) | Touch feedback, ticks in every theme |
| `thud` | Low knock: sine with a fast pitch drop plus filtered noise | Deny, drop, block impacts |
| `noise` | Filtered noise burst or sweep (air, hiss, whoosh, static, wash) | Space switch, Workshop, Orbit, Water |
| `squelch` | Resonant filter sweep over noise and a low saw (gel) | Green Slime |
| `debris` | Burst of many tiny impacts | Voxel World effects |

### 6.4 Render pipeline

`DesktopSoundEngine.render(themeId, eventId, variant)` returns `{ L, R, sampleRate, seconds, lufs, peakDb }`:

1. Look up the sound definition (shell event, or theme effect by event and variant name).
2. Render every note and layer with its instrument onto a stereo mix (time, level, pan), with the variant's small
   deviations applied (§6.6).
3. Apply the sound's bus effects in a fixed order: drive, tone, chorus, echo, space.
4. Master: low cut at 110 Hz; trim the tail where it stays below −66 dB of the peak; 15 ms fade-out and 0.5 ms
   fade-in guard so a buffer always starts and ends at zero; gain to the role's loudness target, limited by the
   role's true-peak ceiling (−3 dBTP).

Definitions are validated when a theme registers: unknown model, voice or event, a missing event, or a value
outside its range is an error (thrown in tests, logged once and the theme skipped in the browser).

### 6.5 Theme data

A theme is one `DesktopSoundEngine.defineTheme({...})` call with pure data:

```js
DesktopSoundEngine.defineTheme({
    id: 'crystal',
    key: 81,                                   // tonal centre as a MIDI note
    space: { size: 0.9, rt60: 0.85, damping: 7000, predelay: 0.012, mix: 0.24 },
    voices: {
        chime: { model: 'glass', t60: 0.9, beat: 0.55, attack: 0.004 },
        tick: { model: 'click', tone: 5200, ring: 0.03 },
        air: { model: 'noise', color: 'pink', band: [2500, 7500], attack: 0.06, decay: 0.2 }
    },
    events: {
        'window.open': {
            notes: [{ v: 'chime', n: 0, t: 0, g: 0.9, pan: -0.15 }, { v: 'chime', n: 5, t: 0.085, g: 1, pan: 0.2 }],
            layers: [{ v: 'air', t: 0, g: 0.05 }]
        }
        // ... all 26 events
    },
    fx: {}                                     // theme effects: event -> variant -> definition
});
```

`n` is in semitones relative to `key`. A sound may override the theme's `space` and add `chorus`, `echo`, `drive`
or `tone`. A note may override parameters of its voice. Every theme defines all 26 events explicitly; there is no
generator that derives events from a profile.

### 6.6 Variants

Frequent events are rendered in two or three variants (§7) and played round-robin, never the same one twice in a
row. A variant differs only slightly: partial tuning within ±4 cents, mallet or strike hardness, level within
±0.7 dB, timing within ±2 ms, a different noise seed. Notices, dialogs and the start sound have exactly one
variant, so signature sounds are always identical.

This replaces the ±3 % playback-rate and ±1.5 dB gain jitter in `playBuffer` for shell events. Theme effects
(blocks, bubbles, pings) are physical sounds with their own variant banks; they keep a small rate jitter of ±2 %.

### 6.7 Rendering in a Worker, caching

- The runtime creates `new Worker('/js/desktop/bundles/desktop-sounds.bundle.js')` on first need (sounds enabled
  and unlocked, or a preview). Requests are `{ id, theme, event, variant }`; replies carry the two channel buffers
  as transferables plus the measured values.
- **Fallback:** if the Worker cannot be created, reports an error, or does not answer the first request within
  4 s, the runtime loads the bundle into the window through the existing module loader and renders on the main
  thread, one sound per task. `DesktopSounds.inspect().renderer` reports `worker` or `main`.
- **Prewarm:** after unlock the runtime requests variant 0 of every event of the active theme in priority order
  (windows and menus first, start sound and notices last), then the remaining variants. An event that is needed
  before its buffer exists jumps the queue.
- **Late sounds are dropped:** if a buffer arrives more than 150 ms after its event, the event is not played (the
  buffer is kept). A sound never plays out of sync with what the user saw. Previews wait for their buffers and are
  not dropped.
- Only the active theme is cached; a theme or settings change drops the cache as today. Buffers have their real
  length (0.05–2.8 s). Worst case for a full theme is about 10 MB of audio memory.
- The Worker is terminated when sounds are switched off and on `pagehide`, together with the audio context.

Plan task: confirm in a browser test that the desktop page's content security policy allows a same-origin Worker;
the fallback covers the case that it does not.

### 6.8 Runtime changes

- **Playback chain:** source → per-play gain → optional panner (theme effects only) → category gain → master gain
  (volume squared, as today) → limiter → output. The compressor becomes a safety limiter (threshold −2 dB, ratio
  12, attack 3 ms, release 120 ms) that leaves single sounds untouched and only catches overlaps.
- **Categories:** the four existing ones plus `interface` (touch feedback) and `startup` (start sound).
- **Notices:** `showDesktopNotification` maps the payload type: `error` → `notify.error`, `warning` →
  `notify.warning`, `success` → `notify.success`, MeshCore messages → `notify.message`, everything else →
  `notify.info`. Plan task: confirm the type names the toast code uses.
- **Start sound:** `session.start` is requested once per page load when the shell has finished booting and the
  session restore is over. It plays only if the audio context runs without a user gesture at that moment (the
  browser allows this typically right after logging in) and its buffer is ready within 1.5 s. Otherwise it is
  skipped; it is never played later on the first click.
- **Touch feedback:** one set of delegated listeners in the sound runtime: `change` on shell switches →
  `ui.toggle.on` / `ui.toggle.off`; `input` on shell range sliders → `ui.tick` (at most one per 45 ms, pitch
  follows the slider value within ±1 semitone through the playback rate); activation of a dock icon →
  `ui.select`. Covered surfaces: Settings, Quick Settings, dock. Apps are not touched. An element with
  `data-sound="off"` is skipped. Plan task: take the exact selectors from the shell markup.
- **Rate limits:** unchanged for existing events. `ui.*` events have their own gap (40 ms per event) and do not
  count against the budget of six voices per 250 ms, so they cannot starve window sounds.
- **Public API:** `desktopSound(eventId)`, `previewDesktopSound`, `setDesktopSoundVolume`,
  `applySoundSettingsChange`, `syncDesktopSoundSettings` and `DesktopSounds.{play, preview, inspect, renderTheme,
  close}` keep their names. New: `DesktopSounds.renderEvent(themeId, eventId, variant)` resolves to the measured
  values of one sound (used by browser tests), and `DesktopSounds.previewCategory(category, themeId)` (§11).
- **Unchanged:** sounds are off by default; preview works without enabling sounds; nothing plays while the page is
  hidden or a session is being restored; `THEME_FX` gates, merge windows and rates.

## 7. Events and roles

26 shell events: the 19 existing ones and seven new (`session.start`, `notify.success`, `notify.warning`,
`ui.toggle.on`, `ui.toggle.off`, `ui.tick`, `ui.select`). The table lives in `sound-engine.js` and is the single
source for category, loudness target (maximum momentary loudness in LUFS before the volume setting), maximum
length including the reverb tail, and number of variants. The targets below are the starting values; tuning may
move a value by a few dB, the order of the roles stays.

| Event | Category | Target | Max length | Variants |
| --- | --- | --- | --- | --- |
| `session.start` | startup | −18 | 2.8 s | 1 |
| `notify.message` | notifications | −18 | 1.4 s | 1 |
| `notify.info` | notifications | −19 | 1.2 s | 1 |
| `notify.success` | notifications | −19 | 1.2 s | 1 |
| `notify.warning` | notifications | −18 | 1.2 s | 1 |
| `notify.error` | notifications | −18 | 1.2 s | 1 |
| `dialog.open` | files | −22 | 0.6 s | 1 |
| `dialog.confirm` | files | −21 | 0.7 s | 1 |
| `dialog.cancel` | files | −23 | 0.5 s | 1 |
| `window.open` | windows | −23 | 0.6 s | 3 |
| `window.close` | windows | −24 | 0.5 s | 3 |
| `window.maximize` | windows | −24 | 0.6 s | 3 |
| `window.minimize` | windows | −25 | 0.4 s | 3 |
| `window.restore` | windows | −25 | 0.4 s | 3 |
| `window.snap` | windows | −26 | 0.25 s | 3 |
| `window.deny` | windows | −23 | 0.35 s | 2 |
| `file.drop` | files | −23 | 0.5 s | 3 |
| `file.trash` | files | −23 | 0.5 s | 2 |
| `file.delete` | files | −22 | 0.6 s | 2 |
| `space.switch` | navigation | −25 | 0.6 s | 2 |
| `menu.open` | navigation | −28 | 0.2 s | 3 |
| `menu.close` | navigation | −30 | 0.15 s | 3 |
| `ui.select` | interface | −31 | 0.12 s | 3 |
| `ui.toggle.on` | interface | −31 | 0.12 s | 3 |
| `ui.toggle.off` | interface | −32 | 0.12 s | 3 |
| `ui.tick` | interface | −34 | 0.06 s | 3 |

Very short sounds may be limited by the peak ceiling before they reach their loudness target; that is intended
(a tick cannot and should not carry the energy of a chime).

Theme effects are mastered to the window level (−23) and keep their run-time gains (`0.52`, `0.4`, `0.42`, `0.38`
times strength), so at full strength they sit about 8 dB below a window sound, as today.

## 8. Sound language

The same grammar holds in every theme; only instruments, key and room change.

- **Direction.** Opening, arriving and affirming gestures rise; closing, leaving and declining gestures fall.
  `window.open` / `window.close`, `window.restore` / `window.minimize`, `menu.open` / `menu.close`,
  `dialog.confirm` / `dialog.cancel` and `ui.toggle.on` / `ui.toggle.off` are mirrored pairs built from the same
  notes.
- **Size.** `window.maximize` is the open gesture made wider (larger interval, more room). `window.snap` is one dry
  tick, the shortest window sound.
- **Notices.** Info: two rising notes, neutral. Message: the theme's signature motif of three notes, its most
  recognizable sound. Success: rising and resolved on the tonal centre. Warning: the same note twice, unresolved.
  Error: two low falling notes a semitone or a tritone apart, with a soft attack; never a buzzer.
- **Deny.** One muted low knock.
- **Files.** Drop: a soft landing, low and damped. Trash: a short falling flick with a little noise. Delete: lower
  and more final than trash.
- **Space switch.** A short swell of the theme's texture.
- **Touch.** Toggle: a two-tone tick, up for on and down for off. Tick: the shortest click. Select: a soft press.
- **Start.** Four or five notes that state the message motif in the theme's fullest room.
- **Harmony.** All tonal content of a theme uses its key and scale, so overlapping sounds stay consonant.
- **Spectrum.** Main energy between 250 Hz and 5 kHz; nothing below 110 Hz; highs restrained (air only as a faint
  layer). Tonal notes have attacks of 2–8 ms; ticks are shaped transients, never single-sample clicks.

## 9. The 16 themes

Each identity follows the theme's existing description in Settings, so the description texts stay valid. Keys and
rooms are starting points for tuning.

| Theme | Identity | Lead instruments | Tonal centre and scale | Room |
| --- | --- | --- | --- | --- |
| `crystal` | Glass bells, airy shimmer, hall | `glass` chime, `noise` air, glass `click` | A5, major pentatonic | Bright hall, 0.85 s |
| `wood` | Marimba taps, felt clicks, soft room | `bar` (rosewood), felt `click`, `thud` | F4, major pentatonic | Small dry room, 0.3 s |
| `analog` | Detuned synth, chorus, plate | `analog` pluck and sweep, sub | D4, Dorian | Plate 0.6 s, chorus |
| `workshop` | Servos, relay clicks, pneumatic hiss, metal ring | relay `click`, `metal` ping, `noise` hiss, `blip` servo glide | E4, fifths and fourths only | Tight workshop, 0.35 s |
| `water` | Bubbles, droplets, underwater wash | `drop`, drop clusters, low-passed `noise` wash | G4, major pentatonic | Wet cave, 0.8 s, damped |
| `amiga` | Square-wave notes, retro arpeggios | `chip` pulse and triangle | C5, major | Dry, tiny slap echo |
| `cyberpunk` | Deep electronic pulses, metallic signals, static clicks | `analog` pulse bass, `metal`, static `noise` | A3, Phrygian | Dark short plate, echo |
| `black_gloss` | Polished glass tones, soft spacious tails | `glass` (darker, slower attack), soft `bell` | D♭5, major with added ninth | Smooth hall, 1.0 s |
| `crazy` | Bouncing bubbles, surprising colourful sequences | `drop` and `blip` with boing glides, toy `bar` | E4, whole-tone and Lydian | Short, springy |
| `lollipop` | Celesta chimes, candy pops | `bell` (celesta), high `drop` pop, glockenspiel `bar` | B♭5, major | Small bright room |
| `organic` | Warm wooden notes, soft air, water-like tones | `bar` (kalimba, log drum), `string`, breath `noise`, soft `drop` | G3, minor pentatonic | Soft room, 0.45 s |
| `electronic_dreams` | Copper clicks, mellow synth pulses, amber signal arpeggio | copper `click`, mellow `analog` pulse, `bell` | A3, minor seventh | Tape echo, plate |
| `windows_xp` | Bright synthesized chimes, soft clicks, classic digital tones | `bell` electric piano, `pad` swell, soft `click` | E♭4, major | Medium hall, 0.8 s |
| `green_slime` | Gel pops, bubbly drops, low squelches | `squelch`, thick low `drop`, wet `thud` | F2–F4, minor third steps | Damp and short |
| `voxel_world` | Wooden taps, stone ticks, blocky percussive notes | wood-block `bar`, stone `click`, earth `thud` | C4, major pentatonic in plain steps | Dry |
| `orbit` | Soft synth whooshes, sonar pings, airlock hiss | `blip` sonar with echo, `noise` whoosh and hiss, `pad` | E4, Lydian | Deep space, long soft tail, echo |

`windows_xp` uses instruments typical of early-2000s system sounds with motifs of its own; it does not recreate
any existing system sound.

## 10. Theme effects

The three effect families are composed anew with the new instruments; the runtime contract does not change.

| Family | Events and variants | Instruments | Length |
| --- | --- | --- | --- |
| Voxel World | `window.fracture`, `window.impact` × `wood`, `stone`, `earth` | `debris`, wood-block `bar`, stone `click`, `thud` | ≤ 0.7 s |
| Green Slime | `slime.pop`, `slime.snap` × `small`, `medium`, `large` | `drop`, `squelch` | ≤ 0.3 s |
| Orbit | `orbit.ping` × `small`, `medium`, `large` | `blip` sonar, own short echo | ≤ 1.0 s |

Effect sounds use no room reverb (they repeat quickly and must not smear); an effect may carry its own short echo.

## 11. Settings and translations

- New settings in `internal/desktop/types.go` and the JS defaults: `sound.interface` (default `true`) and
  `sound.startup` (default `true`), both `true`/`false`. They appear as two more switches in the Sound category.
  All other keys, values and defaults stay as they are; saved choices remain valid.
- **Listening per category:** every category switch row in Settings gets a small preview button that plays that
  category's events of the selected theme in sequence (`DesktopSounds.previewCategory`). Together with the six
  rows this makes all 26 events audible from Settings. The theme card's Preview button keeps playing three sounds
  (`window.open`, `notify.message`, `window.close`).
- New translation keys, translated into all 16 locales: `desktop.settings_sound_interface`, `…_interface_desc`,
  `desktop.settings_sound_startup`, `…_startup_desc`, `desktop.settings_sound_preview_category` (with a `{name}`
  placeholder, used as the accessible name of the row buttons).
- Theme names and descriptions are not changed.

## 12. Tools and tests

- **`scripts/render-desktop-sounds.mjs`** loads the sound sources in bundle order and renders in Node:
  one WAV per sound, a contact sheet per theme (all events in sequence), and a table with loudness, true peak,
  length, spectral centroid and attack per sound (`--theme`, `--event`, `--out`, `--json`). This is the authoring
  and listening tool.
- **`scripts/test-desktop-sounds.mjs`** (`npm run test:desktop-sounds`, added to the web-assets checks) renders
  every theme, event and variant and every effect variant and asserts:
  - all 16 themes register and define all 26 events; validation rejects broken definitions;
  - loudness within ±0.5 LU of the role target, or below it when the peak ceiling binds; true peak ≤ −3 dBTP;
  - length within the role's maximum; first and last sample at zero; no DC offset; energy below 90 Hz at least
    30 dB under the total;
  - rendering is deterministic (same request, same samples);
  - variants of one event differ from each other, within 1 LU;
  - mirrored pairs are different sounds; no two themes give identical audio for the same event;
  - a whole theme renders within a time budget.
- **Go tests** in `ui/` that pin the old internals are rewritten to the new contract:
  `ui/desktop_sound_settings_test.go` (bundle markers, the 26 event keys in each of the 16 theme files, new
  settings and translation keys; the `THEME_FX` literals stay), `ui/desktop_sound_browser_test.go` (per-theme
  checks through `DesktopSounds.renderEvent`, numbers re-derived for 26 events and the new levels; the runtime
  scenarios for rate limits, effect bursts, preview and "no late audio after mute" keep their intent; new checks
  for the Worker path, the fallback and the start sound), and the sound parts of the sensory, Green Slime, Orbit
  and settings layout browser tests where they touch changed names. `internal/desktop/service_test.go` covers the
  two new settings.
- `scripts/build-ui-bundles.js` gets the new part list; `npm run build:ui` and `npm run check:ui` as usual.

## 13. Tuning and acceptance

Tests can prove levels, lengths and cleanliness, not beauty. Tuning therefore has three steps per theme:

1. **Measured gates:** the Node test of §12 passes.
2. **Listening by tool:** the theme's contact sheet and its key sounds are checked with `earshot`: the material
   must read as the theme's identity (water as drops, wood as marimba or wood block, and so on), no sound may read
   as harsh or as a cheap beep more strongly than as its intended material, and the five most used events are
   compared with the old render. `earshot` is a set of listening models, not a human ear.
3. **The user's ears:** one contact-sheet WAV per theme is handed over in the conversation, and all events can be
   heard in Settings. Feedback changes theme data only, not the core.

The work is accepted when all gates pass and the user has heard and accepted the themes.

## 14. Documentation

- `documentation/desktop-themes.md`: rewrite "Matching cursors and sounds" and "Effect sounds" for the new core,
  the event list, roles, the theme data format and the tools.
- `ui/js/desktop/apps/AGENTS.md` ("Desktop UI sound contract"): new events, categories, tests and commands; remove
  the stale "five themes" wording.

## 15. Risks

- **Amount of authored sound:** 16 themes × 26 events plus 15 effect variants. Mitigation: the shared language, a
  small set of strong instruments, themes authored in groups, measured gates before any listening.
- **Taste cannot be tested.** Mitigation: listening rounds with the user, cheap retuning because themes are data.
- **Worker blocked or slow on weak devices.** Mitigation: main-thread fallback, prewarm order, dropping late sounds.
- **Start sound depends on the browser's autoplay rule** and will not play everywhere. It is skipped silently.
- **Other sessions work in the same checkout.** The implementation runs on its own branch and touches only the
  files named here.

## 16. Later, not in this design

- Window sounds placed in the stereo field by window position.
- A logout sound; sounds for app-specific actions.
- Own sound themes for Standard, Fruity and Minimal (they keep using Crystal).
