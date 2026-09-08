# Codex handoff: animated personas in Live Speech

Date: 2026-09-09. Audience: Codex in VS Code, working in the AuraGo repository.

## Task to continue

Integrate the twelve prepared Rive personas into the existing live voice panel.
Start with webchat and reuse the shared panel for Desktop Live Speech. The avatar
should follow the selected personality, listen, think and move its mouth with
the assistant's audible speech. Preserve the existing voice providers, session
handling, controls, captions and static chat-message avatars.

This is an implementation handoff. The asset import is complete; the live UI
integration has not been implemented or tested yet.

## Workspace and completed work

- AuraGo: `C:\Users\Andi\Documents\repo\AuraGo`.
- Complete authoring project: `C:\Users\Andi\Documents\repo\personas`.
- Current twelve-persona collection:
  `C:\Users\Andi\Documents\repo\personas\personas`.
- Asset import commit: `47b795b1a` (`feat(ui): add reviewed Rive persona integration assets`).
- Runtime assets: `ui/img/personas/animated/`, approximately 28.7 MB total.
- Local runtime: `ui/js/vendor/rive/`, pinned `@rive-app/canvas@2.42.0`, MIT.
- Read [the asset contract](animated-personas.md) for the exact initialization,
  personality mapping and controls. `catalog.json` is the machine-readable
  source of truth for filenames, artboards, revisions, sizes and SHA-256 values.

The `.riv` files embed their images; no external atlas or PNG sequence is needed.
Existing `ui/embed.go` patterns already include the assets and runtime. Rebuild
AuraGo before expecting a running binary to serve them. Keep source PNGs,
builders, diagnostics, demo audio and ZIP archives in the sibling project.

The former authoring location under `D:\auragollm\training` is obsolete. All
278 original files were moved and individually verified by SHA-256.

## Read before editing

Read root `AGENTS.md`, `ui/AGENTS.md` and
`ui/img/personas/animated/AGENTS.md`; read `ui/js/desktop/apps/AGENTS.md` if
touching Desktop app files. Inspect current Git status and preserve unrelated
edits. Follow the repository's impact checks, bundle generation, translations,
DOX and local-commit requirements. Do not push without a user request.

## Verified integration points

These names were checked against the working source at handoff time; re-read
the files before editing rather than relying on line numbers.

| Source | Existing responsibility and integration point |
|---|---|
| `ui/js/realtime-speech/panel.js` | `AuraRealtimeSpeechUI.mount(root, options)` builds the shared panel and returns an unmount callback. Attach one avatar to each visible panel and dispose it through this lifecycle. |
| `ui/js/realtime-speech/webchat.js` | Mounts the panel during page initialization; `setOpen()` only toggles the overlay. Mounting does **not** mean it is visible. Defer Rive loading/animation until opened and pause on close without changing voice-session semantics. |
| `ui/index.html` | `#realtime-speech-webchat-panel`, overlay and versioned realtime scripts. |
| `ui/js/desktop/apps/live-speech.js` | Uses the same panel; `dispose(windowId, options)` calls its unmount callback and handles `keepSession`. |
| `ui/js/realtime-speech/core.js` | `window.AuraRealtimeSpeech` is an EventTarget. Read `state`, `adapter`, `userSpeaking`, `providerSpeaking`, `actionActive`; subscribe to `state`, `action`, `mute`, `config` as needed. `bindAdapter()` maps provider audio events; `handleSpeechStart()` interrupts assistant output. |
| `ui/js/chat/chat-history.js` | `setActivePersonaIconKey()` sets `window._activePersonaIconKey` and emits `aurago:persona-icon-change` with `event.detail.key`. |
| `ui/js/desktop/apps/agent-chat.js` | `applyDesktopPersonaIconKey()` emits the same event. `ensureDesktopChatPersona()` resolves `/api/personalities` response `active` and the matching entry's `core` flag. Live Speech can open before Agent Chat, so do not assume its initial key has been loaded. |
| `ui/js/shared/template-data.js` | Exposes `window.BUILD_VERSION` and `window.AURAGO_BUILD_VERSION` for asset cache keys. |
| `ui/css/realtime-speech.css`, `ui/css/desktop-realtime-speech.css` | Existing panel/overlay styling; keep responsive controls and captions usable around the avatar. |
| `scripts/build-ui-bundles.js` | Source of truth for generated bundles. Inspect current script loading before adding a module; never edit generated bundles directly. |

Resolve the existing active personality, then look it up in the imported catalog.
`custom` and unsupported personalities retain the static fallback. Use catalog
membership, not raw server/user values, to construct `.riv` paths. Listen for
persona changes and reject stale async loads after switching or unmounting.

## Reuse the existing assistant-output analysers

`AuraRealtimeSpeech`'s `level` event is **microphone input** from `bindAudioGate()`.
It must not drive the assistant's mouth. The output sampling already exists:

- OpenAI: `adapter.getOutputLevel()` in `provider-openai.js`; its analyser tap
  observes the remote stream while the existing audio element owns playback.
- Speech Lab: `adapter.getOutputLevel()` in `provider-speech-lab.js` observes
  its existing media-element playback path.
- xAI and Gemini: `adapter.player.getOutputLevel()` on `PCMPlayer` in
  `provider-common.js`; this player also has `playing`/`idle` events.
- `ui/js/desktop/apps/live-speech-fx.js` contains `readOutputLevel()`, the
  existing adapter-first/player-second lookup to follow.

Use these taps without creating another microphone stream, AudioContext or
audible output connection. Re-resolve the current adapter after reconnects.
An unavailable analyser should leave a closed mouth and functioning voice chat.

Provider `audio: {active: true}` and runtime `speaking` are not reliable evidence
of audible samples: OpenAI/xAI can emit `pending: true` before playback, and
OpenAI response completion need not coincide with the final audible sample.
Use actual output energy for mouth movement; reset promptly on interruption,
stop, error and disposal. Test queued audio tails and pending/silent responses.

## Animation mapping and deliberately small first implementation

Use the existing `VoicePersona` state machine; obtain inputs after `onLoad`.

| Signal | Rive input |
|---|---|
| Idle, closed, parked, connecting, reconnecting or error | `mode = 0`, closed mouth; retain real textual status. |
| Listening to the user | `mode = 1`, closed mouth. |
| Executing a tool/action, with no audible assistant output | `mode = 2`, closed mouth. |
| Audible assistant output | `mode = 3`, output-driven mouth; this can overlap an active action. |
| Explicit happy preview / a real application cue | `mode = 4`; do not infer emotion from arbitrary transcript text. |

- `mouthOpen`: 0..1, closed at or below 0.08. Clamp finite output values, apply
  modest attack/release smoothing and a silence gate to avoid rapid pose chatter.
- `viseme`: rest 0, AA 1, EE 2, OO 3, FV 4. Current live adapters do not expose
  timed phonemes/visemes. Start with conservative amplitude-driven opening
  (for example AA/rest); RMS alone cannot identify phonemes. Do not advertise
  phoneme-accurate lip sync or copy the demo's timed cues onto arbitrary speech.
- `headTilt`: -1..1 gives approximately +/-3 degrees of **2D** neck tilt.
  Keep motion subtle; it is not a 3D left/right head-turn rig.
- Blink/idle animation is in the Rive asset. Do not add full-frame blink swaps.
- MCP uses display patterns and Terminator mechanical jaw poses; respect the
  catalog's `speechStyle` instead of assuming human lips.

One small shared avatar module and existing panel lifecycle should suffice.
No provider rewrite, new server API, framework, remote CDN, character generation
or authoring-source import is needed for this first integration.

## Loading, quality and lifecycle

- Load the pinned local script/WASM once, and only the selected `.riv` lazily.
  Version the script, WASM, catalog and `.riv` requests using BuildVersion.
  Keep `enableRiveAssetCDN: false` and the PNG fallback until loading succeeds.
- Use the catalog's artboard, `Fit.Contain`, a bounded device-pixel ratio and
  resize handling. Preserve the full head/hair and avoid stretching the face.
- Pause when the view or tab is hidden. Honor `prefers-reduced-motion` with a
  static fallback/paused presentation; this must not stop a voice session.
- On unmount, cancel animation work, observers and listeners; call
  `player.cleanup()`. Rapid open/close or persona switching must not revive an
  old player. Preserve Desktop's `keepSession` behavior.
- Keep focus handling, text status, captions and controls accessible. Translate
  any new visible labels through all supported locales.

User-reviewed fixes to preserve: localized blinking keeps hair/clothing stable;
Silver thinking revision 6 keeps the neutral facial contour; Punk/Hoodie head
alignment and Professor/Terminator border artifacts were corrected. Mouth poses
remain discrete, and cross-expression blinking/continuous lip morphs are still
prototype limitations. Compare against the reviewed player before blaming an
integration issue on the source assets.

## Acceptance and hand-back

1. All twelve catalog entries load, switch correctly and preserve the PNG
   fallback for custom, missing files, runtime failures and reduced motion.
2. Webchat and Desktop display the selected persona, including opening Desktop
   Live Speech before Agent Chat. Opening the view does not start recording.
3. Check idle/listen/think/speak/happy with controlled inputs and real playback.
   Microphone-only speech leaves the mouth closed. Silent/pending output, queued
   audio tails, barge-in, stop, errors and reconnects leave no stuck mouth.
4. Compare blink, hair, clothes, nose/mouth separation, face scale and head
   alignment against the source player, including Silver, Punk, Hoodie,
   Professor and Terminator. Test narrow screens, high DPI and both surfaces.
5. Rapid switching, overlay reopen, Desktop disposal, hidden tabs and reduced
   motion leave no orphaned Rive instances, listeners or animation loops.

Existing checks to run after implementation, from the AuraGo root:

```powershell
node scripts/test-realtime-speech.mjs
go test -count=1 ./ui -run 'TestChatIndexLoadsAllRealtimeSpeechProviders|TestDesktopLiveSpeech'
node scripts/build-ui-bundles.js --check
```

Run `node --check` on changed JS. If a bundle input changed, regenerate with
`node scripts/build-ui-bundles.js`, then repeat its read-only `--check`. Add a
focused runnable check for the new avatar's output/state and disposal behavior;
existing checks do not establish animation acceptance. Inspect the actual
rendered browser and confirm local asset/WASM loading without CDN requests.

For the source reference, run `Start-Demo.ps1` in the outer `repo\personas`
directory and open `http://127.0.0.1:8793/personas/player.html`. Its current
collection also includes `check.html`, `alignment-check.html`, `player.js` and
`verify_collection.py`. The preview server may need restarting in a new session.

Already verified during import: 12 datasets/108 poses and checksums, all runtime
payload hashes, Go embed inclusion, vendored JS syntax and bundle consistency.
These checks are **asset-import evidence**, not live integration acceptance.
Report which providers were actually exercised with audible output; separate
mocked coverage from live verification. Commit the focused implementation and
report its hash, checks and any remaining visual/audio limitations.
