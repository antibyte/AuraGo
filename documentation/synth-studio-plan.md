# Synth Studio expansion plan

Status: implemented. Baseline reviewed against the working tree on 2026-10-31;
the work below landed in the same cycle. This document plans four improvement
tracks — new sounds, a better demo, new effects and a better UI — for the Synth
Studio desktop app. The behavioural contract in `documentation/synth-studio.md`
is authoritative.

## What shipped

| Area | Delivered |
| --- | --- |
| Format | v3 with a per-track `fx` chain, a project `master` strip and an upgraded v1/v2 migration path |
| Effects | New module `synth-studio-fx.js` with 12 insert processors, a master strip, tempo-synced echo and post-fader sends |
| Voice engine | Per-voice LFO (pitch/amp/filter/pulse), continuous pulse width, selectable filter types, bipolar filter envelope, pink/brown noise, third FM modulator |
| Library | 80 → 120 factory presets with character tags; the drum kit now covers the 47 percussion notes from 35 to 81, each with its own patch |
| Demo | 32-bar arrangement with two scenes and two automation curves; three genuinely different templates |
| UI | Effect rack, tag/favourite/recent library, master analyser, adjustable master strip in the transport, guided tour, wide workbench |

Decisions taken while implementing: four insert slots per track, a real format v3
instead of widening v2, the sound designer stays deferred, and the showcase keeps
the "Neon Sketch" identity.

## Goal and scope

Synth Studio already ships as a complete browser-rendered instrument: 80 factory
presets, 128 General MIDI programs, a 16-track arrangement editor, a note editor,
MIDI input, scenes, tone/reverb automation, an arpeggiator, a Euclidean drum
generator, WAV/MIDI export and a validated versioned project format. The gap is
not missing features — it is **depth and first impression**. The instrument
engine is narrow, the demo is flat, the effect bus exposes only mix amounts of
hardcoded processors, and the UI does not show what the synth is doing.

The plan therefore spends effort on four axes and explicitly avoids re-inventing
what already works.

| Track | Outcome | Depends on |
| --- | --- | --- |
| A — New sounds | Wider, deeper, user-creatable sound library | — |
| B — Better demo | A 32-bar showcase arrangement, distinct templates, guided tour | — |
| C — New effects | Parameterised, orderable FX chain plus master strip | Project format v3 |
| D — Better UI | Analyser, FX rack, library v2, transport v2, responsive 3-column | A, C |

**Out of scope.** Audio recording, audio clips, plug-ins, external MIDI output,
sample playback and microphone input are explicitly excluded by the current
project contract and stay excluded. Multiplayer and cloud sync are out of scope.
Wavetable file import is *deferred*, not forbidden — see A6.

## Baseline findings

Each finding is anchored to a file and line so it can be re-verified.

### Sound engine

| # | Finding | Anchor |
| --- | --- | --- |
| S1 | The patch model is a closed operator list: `wave`, `level`, `second`, `unison` (1/3/5), `detune`, `attack`, `decay`, `sustain`, `release`, `pitchEnv`, `glide`, `sub`, `mod`/`mod2` + `ratio`/`depth`, `noise`, `cutoff`, `filterEnv`, `filterDecay`, `filterQ`, `fx{}`. | `ui/js/desktop/apps/synth-studio-voices.js:122-197` |
| S2 | The oscillator vocabulary is fixed: sine, triangle, sawtooth, square, six custom periodic waves (`w9999`, `supersaw`, `pulse25`, `pulse12`, `formant`, `bell`) and two noise buffers. | `ui/js/desktop/apps/synth-studio-voices.js:438-445`, `:210-225` |
| S3 | There is **no per-voice LFO**. Vibrato exists only as a response to the MIDI modulation wheel. | `ui/js/desktop/apps/synth-studio-voices.js:473-487` |
| S4 | Pulse width is baked into the harmonic table (`pulse25` = 25 %, `pulse12` = 12 %). There is no continuous width or PWM. | `ui/js/desktop/apps/synth-studio-voices.js:441-442` |
| S5 | Every filter is lowpass — the per-voice filter and the track bus filter alike. No highpass, bandpass, notch or morph. | `ui/js/desktop/apps/synth-studio-voices.js:497`, `:276` |
| S6 | The filter envelope can only open the filter, never close it. | `ui/js/desktop/apps/synth-studio-voices.js:501-507` |
| S7 | Modulator routing is index-based and limited to two modulators (`mod`, `mod2`). | `ui/js/desktop/apps/synth-studio-voices.js:180-194`, `:559-575` |
| S8 | The bespoke drum kit covers 16 GM notes; the remaining 31 percussion notes fall back to the vendored GM samples, so one kit mixes two timbres. | `ui/js/desktop/apps/synth-studio-voices.js:33-109`, `:202` |
| S9 | The library holds 16 presets in each of 5 categories = 80, plus 128 GM programs and 2 kits. | `ui/js/desktop/apps/synth-studio-presets.js:12-90`, `:100-101` |

### Effects

| # | Finding | Anchor |
| --- | --- | --- |
| E1 | The per-track bus is `input → lowpass → waveshaper drive → postFx → pan → gain → master`, with phaser, chorus, reverb and delay tapped off `postFx`. | `ui/js/desktop/apps/synth-studio-voices.js:274-381` |
| E2 | **Only the mix amount of each processor is exposed.** Chorus rate (0.68 Hz) and depth (2.6 ms), phaser rate (0.42 Hz) and depth (650 ¢), delay time (0.24 s) and feedback (0.28) and reverb impulse length (2.2 s) are hardcoded constants. | `ui/js/desktop/apps/synth-studio-voices.js:321-372`, `:227-252` |
| E3 | Delay is **not tempo-synced** and its fixed 0.24 s does not correspond to a musical subdivision. | `ui/js/desktop/apps/synth-studio-voices.js:359-360` |
| E4 | Reverb and delay connect to `mix.master` directly, bypassing the track's pan and volume. The track fader therefore does not affect its own tails. | `ui/js/desktop/apps/synth-studio-voices.js:355-370` |
| E5 | There is no EQ. "Tone" is a single lowpass. | `ui/js/desktop/apps/synth-studio-voices.js:276`, `ui/js/desktop/apps/synth-studio.js:88` |
| E6 | The master chain is a fixed gain (0.78) into a fixed compressor (threshold −8, knee 8, ratio 10, attack 3 ms, release 180 ms). No master EQ, no user-adjustable limiter. | `ui/js/desktop/apps/synth-studio-voices.js:254-271` |
| E7 | The project format closes the parameter vocabulary to 9 flat keys, and unknown fields are rejected — so **new effect parameters require a format change**. | `ui/js/desktop/apps/synth-studio-model.js:20`, `:265`, `:227` |
| E8 | Automation is closed to `tone` and `reverb`. | `ui/js/desktop/apps/synth-studio-model.js:307`, `ui/js/desktop/apps/synth-studio.js:111-112` |

### Demo

| # | Finding | Anchor |
| --- | --- | --- |
| D1 | The demo is "Neon Sketch" at 112 BPM with 5 tracks, and **every track repeats its single-bar pattern four times verbatim** — four identical bars. | `ui/js/desktop/apps/synth-studio-model.js:427-445` |
| D2 | The three templates are derived from that same demo and only change tempo and a couple of instrument swaps, so they are near-identical. | `ui/js/desktop/apps/synth-studio-model.js:448-456` |
| D3 | The demo never surfaces scenes, automation, the XY pad, the arpeggiator, variations or the Euclidean generator — the app's most capable features are invisible on first launch. | `documentation/synth-studio.md:81-101` |
| D4 | The pattern library holds 10 one-bar patterns; there is no multi-bar arrangement-level pattern such as an intro or a fill. | `ui/js/desktop/apps/synth-studio-model.js:403-425` |

### UI

| # | Finding | Anchor |
| --- | --- | --- |
| U1 | The footer hardcodes `80 SYNTHS · 6 FX · 128 GM`; it becomes wrong as soon as the catalogue or the FX list changes. | `ui/js/desktop/apps/synth-studio.js:31` |
| U2 | The base workspace rule declares three grid columns but is overridden by a later two-column rule; the third column is dead CSS. | `ui/css/desktop-app-synth-studio.css:41`, `:142` |
| U3 | The inspector is a flat grid of 10 range inputs with no grouping, no FX ordering and no per-effect parameters. | `ui/js/desktop/apps/synth-studio.js:88` |
| U4 | Library search matches the name substring only, and each row shows a meaningless two-digit index. | `ui/js/desktop/apps/synth-studio.js:80-81` |
| U5 | There is no spectrum, oscilloscope or level display anywhere; the meter is "visual feedback only". | `documentation/synth-studio.md:126-129` |
| U6 | The workbench is strictly one-of-three tabs, so notes and sound never appear together. | `ui/js/desktop/apps/synth-studio-workspace.js:11-22` |
| U7 | Every new UI string must land in 16 desktop language files. | `ui/lang/desktop/*.json` (16 files) |

## Track A — New sounds

### A1. Add a per-voice LFO to the patch model
Extend the patch schema with an explicit `lfo` block rather than overloading the
operator list, because a modulator needs a continuous target rather than a gain
envelope:

```json
"lfo": { "wave": "sine", "rate": 4.5, "depth": 0.3, "delay": 0.2,
          "target": "pitch" | "amp" | "filter" | "pulse", "sync": "1/4" | null }
```

Implement in `createVoice` (`synth-studio-voices.js:489`): build one oscillator
plus one depth gain per voice, start it at `at + delay`, and connect it to the
carrier frequencies, the voice gain or the voice filter cutoff according to
`target`. `pitch` and `pulse` targets need a dedicated detune/pitch-modulation
gain because `AudioParam` modulation of `frequency` conflicts with the existing
`setValueAtTime` pitch-envelope ramps — insert a `pitchMod` gain node between the
operator gain and the carrier and sum the LFO there.

This unlocks tape wobble, tremolo, auto-filter sweeps and PWM — character that no
current preset can express.

### A2. Continuous pulse width
Keep `pulse25`/`pulse12` for compatibility but add `pulseWidth` (0.05–0.95) to the
patch. Build the harmonic table on demand: for a duty cycle `d`, the Fourier
coefficients of a pulse train are `A_n = (2/(n·π))·sin(n·π·d)`. Cache the table by
duty cycle quantised to 1/64 to bound memory, and expose it through `waveFor`
(`synth-studio-voices.js:447`).

True PWM additionally needs the width to move over time, so A1's `pulse` target
must modulate the table's duty cycle. Implement by regenerating the
`PeriodicWave` on a coarse time grid is not possible for a running oscillator —
instead implement PWM with the classic two-saw method: two sawtooth oscillators
at the carrier frequency, one phase-delayed by the width, summed. That keeps the
width continuously modulatable and avoids rebuilding waves mid-note.

### A3. Filter type and bipolar filter envelope
Add `filterType` to the patch (`lowpass` | `highpass` | `bandpass` | `notch`) and
apply it to both the voice filter and the track bus filter. Allow `filterEnv` to
be negative so the envelope closes the filter, which produces the classic
"filter-down" sweep. Adjust `envAmt` in `synth-studio-voices.js:501-507` to
compute a signed offset instead of a multiplicative factor above 1.

Add a `filterMode` on the track that morphs continuously between lowpass and
highpass (Web Audio has no morph, so implement by crossfading two filters).

### A4. Richer noise
Add two more noise buffers: `n2` (pink, −3 dB/octave) and `n3` (brown, −6 dB/octave)
alongside the existing `n0` (white) and `n1` (pulsed). Pink noise is the standard
ingredient for wind, rain and air textures; brown noise is the base of sub-bass
and rumble. Add an optional per-operator bandpass in front of the noise source so
`n0` can become a hat, a snare rattle or a vinyl crackle without a new buffer.

### A5. Third modulator and general operator routing
Generalise the `g` routing flag so any operator can modulate any other
operator's frequency or gain, and add `mod3`/`mod2` depth and ratio fields with
the same shape as `mod`/`mod2`. The current routing is a positional convention
(`g: 1` means "modulate operator index 0's frequency"), which becomes ambiguous
as soon as a modulator is optional — a patch with `mod2` but no `mod` silently
shifts its targets.

### A6. Extend the factory library
Grow each category from 16 to roughly 24 presets (80 → ~120) and add tags to
every preset so the library can be filtered by character, not just name:

- **Drums** — complete the bespoke kit to all 47 GM percussion notes (currently
  16) plus 4 non-GM sounds: 808 clap, industrial hit, vinyl crack, metal bell.
- **Bass** — sub-bass with drive, reese, FM slap, acid, and a tuned bass with a
  plucked transient.
- **Leads** — supersaw, PWM, sync, formant, crystal, plus a portamento lead and a
  brass-like additive lead.
- **Pads** — warm, air, choir, shimmer, plus a tape-saturated pad and a
  granular-style evolving pad built from detuned operators.
- **Keys** — electric piano, FM piano, bell, clav, plus a Wurli, a koto and a
  electric "Rhodes-with-tremolo".

Tags use a fixed vocabulary (`acid`, `warm`, `dark`, `bright`, `pluck`,
`sustained`, `airy`, `noisy`, `percussive`, `retro`, `modern`) so the filter chips
stay bounded.

### A7. Deferred: user sound design
A preset editor over the operator model, saving browser-local user patches, is
the highest-value long-term sound feature. It is deferred because it needs the
patch schema from A1–A5 to be stable first, and because it requires a new
persistence surface that must not collide with the project draft rules.

## Track B — Better demo

### B1. Rebuild "Neon Sketch" as a 32-bar arrangement
Replace the four-times-repeated bar with a real arrangement:

| Bars | Section | Content |
| --- | --- | --- |
| 1–4 | Intro | Pad and filtered arpeggio only, no drums, reverb up |
| 5–12 | Groove A | Drums, bass, keys; lead enters at bar 9 |
| 13–16 | Build | Snare roll, filter opens, hats to 16ths, bass doubles |
| 17–24 | Groove B | Chord change, walking bass, lead variation |
| 25–28 | Break | Drums out, pad and vocal-style lead, large reverb |
| 29–32 | Outro | Drums drop out, final hit, tail |

The arrangement is expressed as data in the model module so it stays
version-controlled and testable, not as UI state.

### B2. Wire the features the demo currently hides
- **Two scenes** — "Main" (bars 5–24) and "Break" (bars 25–28) — so the scene
  pads are meaningful on first launch. Scenes already exist in format v2.
- **Two automation curves** — a tone curve that opens the lead filter across
  bars 13–16, and a reverb curve that swells over the break. Automation already
  exists for `tone` and `reverb`.
- **One XY gesture** recorded into the performance take path, so the XY pad has
  an audible result the moment the user touches it.

### B3. Three genuinely different templates
Rewrite `template(style, name)` (`synth-studio-model.js:448`) so Electronic,
Chill and Retro are separate compositions rather than one demo with a tempo
change: different instrument selection, groove, harmony and mix profile. Chill
should sit around 84 BPM with a brushed groove and wide pads; Retro around 108
BPM with a pulse-width lead and a gated snare feel; Electronic around 124 BPM
with the acid bass and a supersaw.

### B4. Guided tour
The welcome screen (`synth-studio-workspace.js:41-45`) currently offers three
template buttons plus New and Open. Replace it with a **Tour** entry that steps
through five beats — arrange, sound, performance, record, export — switching the
workbench panel and highlighting the relevant control at each step. The workspace
already exposes `panel(name)` and `refresh()`, so the tour is mostly a highlight
overlay on top of existing affordances.

### B5. Sound on the first click
The tour's first beat and the demo button should both start audio immediately.
AudioContext unlock happens on a user gesture (`audio.unlock()`), so a demo
preview that plays from bar 1 on click is the cheapest possible demonstration
that the app works. Keep transport playback and preview separate so the preview
never mutates the project.

## Track C — New effects

### C1. Project format v3 with a structured FX chain
The flat, closed parameter list (E7) is the blocker for everything else in this
track. Introduce an optional per-track `fx` object:

```json
"fx": {
  "inserts": [
    { "type": "chorus", "mix": 0.3, "rate": 0.7, "depth": 0.4, "sync": "1/8" }
  ],
  "sends": { "reverb": 0.25, "delay": 0.15 }
}
```

- Keep v1 and v2 readable. `upgrade()` (`synth-studio-model.js:314`) already
  performs the v1 → v2 migration; add v2 → v3 alongside it.
- The first v3 save requires a new filename in the regular picker, exactly as
  the v1 → v2 flow already does, so the original v1/v2 file is protected.
- When v3 is absent, synthesise an equivalent insert chain from the flat params
  so the audio path has a single code path.

### C2. New insert processors
Each is a factory in a new lazily loaded module `synth-studio-fx.js`, following
the existing `synth-studio-*` module convention:

| Insert | Parameters | Notes |
| --- | --- | --- |
| `tremolo` | rate, depth, shape | Cheapest high-impact addition |
| `flanger` | rate, depth, feedback, offset | Modulated comb delay |
| `autowah` | centre, range, Q, direction | Envelope-driven filter sweep |
| `bitcrush` | bits, sample-rate factor | WaveShaper quantisation |
| `softclip` | drive, character | tanh saturation, distinct from `drive` |
| `eq3` | low, low-mid, high gains + mid frequency | The missing mixing tool |
| `width` | width | Mid/side stereo width |
| `cabinet` | type, presence | Speaker-ish colour for bass and lead |
| `gate` | threshold, attack, release | Plucked and drum gating |

### C3. Give the existing processors real parameters
Replace the hardcoded constants (E2) with exposed values, including
**tempo-synced delay** (E3) — `1/4`, `1/8`, `1/8T`, `1/16` resolved against the
project tempo, which is the single largest perceived-quality improvement
available in this track.

Also expose chorus rate/depth/feedback, phaser rate/depth/feedback/stages and
reverb size/decay/pre-delay/tone, each with a `sync` option where a musical
division makes sense.

### C4. Route sends through the track bus
Move the reverb and delay returns behind the track's pan and gain (E4) so the
fader controls the tails. Add a genuine send bus with its own level, and keep
the option of a global plate on the master.

### C5. Master strip
Turn the fixed master chain (E6) into an adjustable strip: three-band EQ,
compressor with threshold/ratio/knee/attack/release, a true brick-wall limiter
with lookahead, and a stereo width control. Expose it in the transport area so
it is visible without opening a panel.

### C6. Widen automation
Once `fx` is structured, automation can address any insert mix plus the master
filter, replacing the closed `['tone','reverb']` set (E8). Keep the existing
parameter names working so v2 projects keep their curves.

## Track D — Better UI

### D1. Make the footer honest
Compute `SYNTHS · FX · GM` from the actual catalogue and FX registry instead of
the hardcoded string (U1), and keep the FX count honest when C2 lands.

### D2. Library v2
Replace the meaningless row index with a category glyph, a tag chip and a
favourite star (U4). Extend search to match tags (U4), and add **Favourites**
and **Recent** filters persisted in `localStorage` alongside the existing view
preferences. Show a small waveform thumbnail per preset — the harmonic table is
already known at build time, so the thumbnail is a static SVG data URI rather
than a rendered audio buffer.

### D3. Analyser and level display
Add a shared `AnalyserNode` on the master chain (U5) and render a spectrum and
an oscilloscope in the transport area, with the existing clip indicator kept.
Draw on a canvas at a throttled rate so the meter refresh cannot influence
musical timing — the current contract already states the meter is feedback only.

### D4. FX rack
Replace the flat inspector grid (U3) with an ordered rack of insert slots,
drag-reorderable, each showing its type, mix and a compact knob set. Reuse the
existing five-column parameter styling (`desktop-app-synth-studio.css:158`) so
the rack matches the rest of the workbench.

### D5. Transport v2
Enlarge the bar/beat readout, add a zoomable overview strip above the
arrangement with a draggable viewport rectangle, and show live voice count and
CPU headroom next to the transport.

### D6. Arrangement and note editor polish
Velocity-shaded notes, a velocity lane under the piano roll, snap-aware grid
shading, and a minimap of the whole arrangement. Clip drag, crop and the XY pad
also need keyboard equivalents — the XY pad already has labelled sliders, but
clip moving and cropping are pointer-only today.

### D7. Responsive 3-column layout
Above ~1400 px, show library, arrangement and sound workbench side by side
instead of the one-of-three tab set (U6). Below that, keep the current tabs. The
workbench already persists the selected panel and height, so the layout change
extends the same view-preference record.

Delivered as a wide workbench: the library keeps its column, the arrangement
takes the full width, and the sound workbench shares the row with the piano
roll. A resize observer moves the layout in and out of the wide mode, so no tab
switch is needed when the window crosses the breakpoint.

### D8. Housekeeping
Remove the dead three-column grid rule (U2). For every new user-facing string,
add all 16 locale files (U7) — use the existing i18n validation to catch gaps.

The narrow container query keeps its own two-column rule, because the inspector
moved into the sound workbench and the third column would stay empty.

## Sequencing

| Wave | Contents | Parallelism | Risk |
| --- | --- | --- | --- |
| **W0** | Foundations: dynamic footer counts, dead CSS removal, library index → tags/glyphs, `synth-studio-fx.js` scaffold | — | Low |
| **W1** | Sound engine: A1–A5, factory library expansion A6 | Independent of W2, W3 | Medium — voice graph changes |
| **W2** | Demo and templates: B1–B5 | Runs alongside W1 | Low — model data only |
| **W3** | Effects: C1–C6, project format v3 | Depends on W1 lessons in the engine | High — file format change |
| **W4** | UI v2: D1–D8 | Depends on W3 for the FX rack, W1 for the library | Medium |

W1 and W2 are independent and can run in parallel. W3 is the critical path and
the only wave that changes the on-disk format.

## Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| v3 breaks existing `.aurasynth` files | Keep v1 and v2 read paths; require a new filename on the first v3 save, mirroring the existing v1 → v2 flow. |
| Node count explodes. The bus already builds ~25 nodes per track, and 16 tracks with 5 inserts each adds hundreds. | Create insert nodes lazily, only once an insert has a non-zero mix, and retire idle buses — `retireBusesIfIdle` already exists (`synth-studio-audio.js:395`). |
| Voice polyphony pressure. Richer patches use more operators per note. | Keep the 256-voice render budget, cap operators per patch, and re-measure the render budget check before shipping A6. |
| Audio glitching on parameter commit | Follow the existing "queued at the next bar" rule for structural changes and smooth parameter ramps via `AudioParam` automation, as the current contract requires. |
| 16 locale files drift | Validate every locale file in CI; treat a missing key as a build failure. |
| Demo sounds thin because it is still synthesised | Re-render the demo WAV with the improved engine before shipping B1, and compare against the current demo for audible regression. |

## Verification

Per wave, in addition to the checks below:

```
node scripts/test-synth-studio-model.js
node scripts/test-synth-studio-audio.mjs
node scripts/test-synth-studio-recording.mjs
node scripts/test-synth-studio-storage.mjs
node scripts/test-synth-studio-registration.mjs
node scripts/test-synth-studio-voices-presets.mjs
node scripts/test-synth-studio-scheduler.mjs
```

Plus UI bundle generation and checks, the focused Desktop and server tests, and
browser smoke runs with `AURAGO_RUN_BROWSER_SMOKE=1` (collecting visual evidence
through `AURAGO_BROWSER_ARTIFACT_DIR`).

Wave-specific acceptance:

- **W1** — a new preset is audible in the library preview and inside a project;
  a patch with an LFO modulates its target; the drum kit renders all 47
  percussion notes without falling back to the GM timbres.
- **W2** — the demo plays 32 distinct bars; both scenes trigger at their bar
  ranges; the automation curves move the audible parameters; the three templates
  are distinguishable by ear.
- **W3** — a v2 project opens, upgrades and saves as v3 under a new filename; an
  inserted effect is audible and its mix is automatable; a v3 file reopened in an
  older build fails with a clear version error rather than a silent change.
- **W4** — the analyser reflects the master output; the master strip is adjustable
  from the transport; the library filters by tag and favourite; the wide workbench
  appears only above the breakpoint and follows the window width; every locale file
  contains every new key.

Real MIDI hardware latency, operating-system permission prompts and audio output
on target devices still require separate acceptance testing, as the current
documentation states.

## Open decisions

These need a decision before the relevant wave starts. Each one changes the
delivered result, so none of them should be resolved by assumption.

1. **Effect chain depth.** How many inserts per track should be allowed? Four is
   generous for a browser instrument; eight risks the node budget above.
2. **Format v3 versus extending v2 in place.** A v3 bump is the honest option
   given the closed-field validation, but it costs users a re-save. The
   alternative — adding optional fields to v2 — weakens the "unknown fields are
   rejected" guarantee.
3. **User sound designer.** Should the preset editor ship in this cycle, or stay
   deferred as in A7?
4. **Demo identity.** "Neon Sketch" keeps its name and genre, or does the new
   arrangement get a new identity that matches its new structure?
