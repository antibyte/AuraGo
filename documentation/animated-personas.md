# Animated personas in Live Speech

Webchat and Desktop Live Speech share a compact animated persona panel. The
selected persona listens, thinks and moves its mouth with assistant playback.
Static chat-message avatars and existing voice-session controls stay separate.

The [original integration handoff](handoff-animated-personas-codex.md) records
the asset import. The current runtime contract is below.

## Files

- `/img/personas/animated/catalog.json`: AuraGo personality IDs, relative Rive
  filenames, artboard names, revisions, hashes, controls and robot speech styles.
- `/img/personas/animated/<key>.riv`: RIVE 7.0 with embedded images. No external
  PNG atlas is required at runtime.
- `/js/vendor/rive/rive.js` and `/js/vendor/rive/rive.wasm`: local
  `@rive-app/canvas@2.42.0`, with `manifest.json` and `LICENSE.txt`.

The existing `ui/embed.go` directory patterns include these files. Rebuild
AuraGo to serve the updated embedded UI; copying source files does not update
an already running binary.

| AuraGo key | Authoring persona |
|---|---|
| servant | 01-butler |
| neutral | 02-silver |
| professional | 03-manager |
| mistress | 04-kommandantin |
| thinker | 05-professor |
| evil | 06-vampir |
| secretary | 07-assistentin |
| psycho | 08-green |
| punk | 09-punk |
| friend | 10-hoodie |
| mcp | 11-mcp |
| terminator | 12-terminator |

`custom` has no generated animation; retain its existing static image.

## Runtime contract

Load the local script lazily when opening the voice view and set the
WASM URL before constructing a player. Apply AuraGo's build-version query key
to the script, WASM, catalog and Rive URLs. The following initialization is a
reference; production lifecycle handling lives in `ui/js/realtime-speech/avatar.js`:

```javascript
// `rive` is provided by the local script; `entry` comes from the catalog.
// `versioned()` appends the current AuraGo BuildVersion as ?v=.
rive.RuntimeLoader.setWasmUrl(versioned('/js/vendor/rive/rive.wasm'));
rive.RuntimeLoader.setWasmFallbackUrl(null);
const player = new rive.Rive({
  src: versioned('/img/personas/animated/' + entry.file),
  canvas,
  artboard: entry.artboard,
  stateMachine: 'VoicePersona',
  autoplay: true,
  enableRiveAssetCDN: false,
  layout: new rive.Layout({fit: rive.Fit.Contain}),
  onLoad() {
    const inputs = Object.fromEntries(
      player.stateMachineInputs('VoicePersona').map(input => [input.name, input])
    );
    player.resizeDrawingSurfaceToCanvas(Math.min(2, window.devicePixelRatio || 1));
    inputs.mode.value = 0;
  }
});
```

- `mode`: idle 0, listening 1, thinking 2, speaking 3, happy 4.
- `viseme`: rest 0, AA 1, EE 2, OO 3, FV 4.
- `mouthOpen`: 0–1; values at or below 0.08 close the mouth. Above that it only
  enables the selected discrete pose, not continuous lip morphing. Current
  adapters expose output energy, not timed phonemes. The controller follows
  a decaying speech peak, uses quick attack/release and closes at short energy
  gaps. Narrow, rounded and wide poses follow energy, with a minimum 60 ms
  dwell between open poses. These are visual approximations, not recognized
  AA/EE/OO/FV sounds. There is no random or transcript-timed mouth loop.
- `headTilt`: −1 to +1, a small 2D neck tilt rather than a 3D side view.
- MCP uses display patterns; Terminator uses mechanical jaw poses.
- Pause when hidden or reduced motion is requested; resize with the view and
  call `player.cleanup()` when disposing it. Retain the PNG fallback on errors.

Speech poses are discrete. Cross-expression blinking and continuous lip morphs
remain prototype limitations. Silver revision 6 preserves the neutral face
contour while thinking; all characters use localized blink regions.

`AuraRealtimeSpeechAvatar.mount(host, {runtime, visible})` returns
`{setVisible, dispose}`. The shared panel keeps its unmount callback and exposes
`AuraRealtimeSpeechUI.setVisible(root, visible)`. Webchat mounts hidden; opening
it starts neither a microphone nor a voice session. Closing its overlay pauses
animation while preserving the session. Desktop disposal retains its existing
`keepSession` semantics. Reduced motion keeps the selected PNG, and all loads
are generation-checked against switches/disposal. Rive load failure is bounded
to 15 seconds with a static fallback, without either CDN fallback.

The output lookup follows the current adapter, then its PCMPlayer. Microphone
`level` events never drive lips. Paused/ended/muted media and suspended output
contexts return zero. Response completion does not cut off queued playback;
interruption, stop and error close the mouth. Happy mode is reserved for an
explicit preview/application cue, never inferred from arbitrary transcripts.

## Verification

Run `node scripts/test-realtime-speech.mjs` (including the avatar state,
discrete-pose, playback and disposal checks),
`go test -count=1 ./ui -run 'TestChatIndexLoadsAllRealtimeSpeechProviders|TestDesktopLiveSpeech'`,
and `node scripts/build-ui-bundles.js --check`. Check changed JS syntax and
regenerate affected bundles before building the binary.

Browser acceptance must use real Rive files and audible output, cover all twelve
personas and both surfaces, short pauses, response tails, interruption, rapid
switching, hidden/reduced-motion views, narrow/light/dark layouts and local
WASM failure. Distinguish local audio fixtures through real playback adapters
from a live provider-service test; simulated state checks alone do not establish
visual or audio quality.

The integration acceptance used all twelve real Rive assets in the rebuilt
embedded-UI preview. Audible local SAPI-generated TTS was played through the
Speech Lab media-element path, OpenAI's output tap, and the xAI/Gemini PCM
players; multiple mouth poses and closed pauses were observed. These were
local playback checks, not live Speech Lab, OpenAI, xAI or Gemini service
sessions. Live provider-service acceptance remains outstanding.

## Authoring project

The complete project has moved to the sibling directory `../personas/`.
Its current dataset collection is `../personas/personas/`, including the
switchable test page, all twelve source folders, builders, checks and ZIP.
The outer directory also retains the original Silver/Green prototypes,
research and diagnostic versions. See that project's `START-HERE.md`.

Only runtime payloads belong in AuraGo; update the catalog hashes and the local
runtime manifest when importing a newly reviewed revision.
