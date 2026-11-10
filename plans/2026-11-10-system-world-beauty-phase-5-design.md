# System World beauty pass — Phase 5: soundscape (design)

Date: 2026-11-10 · Status: design by the main session, based on the approved roadmap
(`plans/2026-11-09-system-world-beauty-design.md`, phase 5 row). The implementation plan is written when
phases 2–4 have landed (zones need the lobbies of phase 3 and the trees and water life of phase 4).

## Goal

The city should sound like a place: the harbour laps and calls, streets hum between the towers, a park
rustles, every lobby has its own room tone, steps sound like the surface under them, and nothing clips.

## What exists today (verified by reading, 2026-11-10)

- **Graph** (`sysworld-audio.js`). `master` gain → analyser → destination. Three buses into the master:
  ambience, effects, voice. One synthetic convolver (3.4 s) returns into the ambience bus. There is no
  compressor or limiter, no `PannerNode`; panning is `StereoPanner`, distance is hand-made gain.
- **Everything is synthesised.** Beds: sub drone, pad (chord every 16 s), air noise, harbour water, murmur,
  lamp hum. Timed events with weights: chime, swoosh, gust, birds, gull, pigeon, cricket, horn, bell, hammer,
  drip, windchime, siren, thunder. Robot foley (`sysworld-foley.js`): step, carry, glide, greet, work, idle,
  turn; 10 voices, 48 m. Effects (`sysworld-effects.js`): visitor step and step inside, door, lift, tram
  chime, discover, hover; plus four fixed spatial hum beds.
- **Not there:** continuous traffic, a moving tram or hover sound, companion steps, any zone except the
  harbour distance in `updateBeds`, per-space reverb.
- **Inside flag.** `setEnvironment(interior, condition)` gets a boolean. Indoors the water and lamp beds stop,
  murmur and foley drop, the effects filter closes to 280 Hz. Gap: timed events ignore the flag, so birds
  and horns play indoors at full level. Robot foley never reaches the convolver.
- **Steps.** Visitor: every 1.8 m, `step` or `step_inside`. Residents: `onGait(id, x, y, z, moving, carrying,
  style)` through `sysworld.js` → `sysworld-scene.js` → `sysworld-audio.js` → `foley.gait`. No surface
  information travels along. `streetProfile(x, z).kind` (`sysworld-ground.js`) knows road, kerb, pavement.
- **Samples.** No System World audio file ships. The only decoded audio is the tower voice from the TTS
  endpoint (`sysworld-voice.js`, `decodeAudioData`). Code comments and the design state that nothing is
  sampled; no test enforces it. `TestShippedWebAssetsLoadNothingExternal` scans text assets for external
  hosts only.
- **Controls.** Sound on/off is stored in `localStorage` (`aurago.desktop.sysworld.sound`); volume
  (default 0.18, max 0.35) and the three channel sliders are not stored. Audio starts only after a user
  gesture and suspends when the window is hidden.
- **Tests to keep green.** Go markers in `ui/desktop_sysworld_test.go` for `sysworld-audio.js`
  (`export function createCityAmbience`, `createConvolver`, `clearTimeout(eventTimer)`, `thunder(`,
  `setMood(`). `scripts/test-system-world-foley.mjs` uses a fake `AudioContext` without
  `createDynamicsCompressor` and `createPanner`: new node types need the fake extended.
  `scripts/test-system-world-voice.mjs`. The browser test's 20 MiB first-display limit.

## Decisions

1. **Limiter.** A `DynamicsCompressor` (threshold −10 dB, knee 6, ratio 12, attack 3 ms, release 250 ms)
   sits between the master gain and the analyser.
2. **Zones as one pure module** (`sysworld-zones.js`): from the listener position it returns smoothed
   weights for `harbour` (distance to the quay and water), `canyon` (between tall buildings), `green`
   (trees, beds, the garden court), `plaza` (open ground), `interior:<id>` (pavilions and lobbies, from
   the phase 3 registry) and `height` (above the roofs). Every consumer reads these weights; no module
   computes its own distances any more.
3. **Three spaces of reverb.** The existing long tail stays as the open-air space. A short slap space
   with discrete early echoes serves the street canyon. One convolver with a set of four small impulse
   responses (lobby, hall, machine hall, small room) serves interiors, chosen by the room the listener is
   in. Sends cross-fade by zone weight. Robot foley and the visitor's steps get a send as well.
4. **Glass damps, it does not mute.** Indoors the outdoor beds and events pass a low-pass (about 900 Hz)
   at −12 dB; timed outdoor events are gated by zone (no birds inside a machine hall). Standing outside
   near an open door lets the room tone leak out.
5. **Moving sources.** The tram gets a continuous rail hum that follows its position, with a soft squeal in
   curves and the existing chime at stops. The nearest four hover robots get a whir. A traffic bed follows
   `busy` and the canyon weight.
6. **Steps know their surface.** The emitter (society for residents, experience for the visitor, the
   companion for itself) passes a surface: road, pavement, kerb, stairs, quay, lobby stone, grating, garden
   path. Each surface has its own synthesised voice. The companion gets a light four-beat step.
7. **Room tones for the lobbies** (synthesised): agent core (reactor hum, soft interface tones), archive
   (near silence, page-like ticks), gate halls (relay clicks, patch tones), departure hall (board flaps,
   belt rumble, a chime before announcements), machine hall (fans, coolant flow), garden court (leaves,
   water trickle; open to the sky, so outdoor sound stays).
8. **Optional real recordings (needs the user's go-ahead before any download).** Recommended set, CC0 only:
   harbour water lapping, wind in leaves, a songbird chorus, gulls, a distant city hum, light rain on
   pavement. Mono Ogg, loops of 20–40 s, at most 1.5 MiB in total, loaded after the first display, stored
   under `ui/3d/system-world/audio/` with a provenance file (source URL, author, licence, hash) like
   `spot-source.json`. The synthesis stays as the fallback and as the layer that reacts to mood; the
   recordings add realism underneath. Without the go-ahead this item is dropped and the phase stays fully
   synthesised.
9. **Controls.** Volume and the three channel levels are stored like the on/off switch.

## Work packages (for the implementation plan)

1. Limiter, extended test fake, stored levels.
2. `sysworld-zones.js` with unit tests; `setEnvironment` carries the room id; events gated by zone.
3. Reverb spaces and sends; glass damping.
4. Tram, hover and traffic sources.
5. Surface-aware steps (callback chain through `sysworld.js`, `sysworld-scene.js`, `sysworld-society.js`,
   `sysworld-audio.js`, `foley.gait`), companion steps.
6. Lobby room tones.
7. Optional: recordings loader, provenance, budget check, licence notes in the docs.
8. Listening pass with the `earshot` tools on rendered captures (loudness, clipping, masking), markers,
   bundle, docs.

## Testing and acceptance

- Unit tests: zone weights at fixed points (quay, mid-street, garden, each lobby, roof height); the
  limiter is in the chain; no outdoor event fires indoors; the surface table covers every ground kind;
  voice counts stay bounded.
- Measured: no sample above −1 dBTP in a two-minute busy rain capture; integrated loudness within 2 LU
  between day-clear and night-rain.
- Existing foley, voice and marker tests stay green.

## Risks

1. A compressor on the master changes the perceived mix; levels of the beds need a re-balance.
2. Several convolvers cost CPU on weak machines: the low tier keeps the single existing reverb.
3. Recordings must loop without clicks and must not dominate; they are trimmed and checked by listening tools.
