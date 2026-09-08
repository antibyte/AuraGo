# Animated personas: integration payload

The reviewed assets are ready for integration. This import does not change the
current avatar renderer or connect live voice events.

## Files

- `/img/personas/animated/catalog.json`: AuraGo personality IDs, relative Rive
  filenames, artboard names, revisions, hashes, controls and robot speech styles.
- `/img/personas/animated/<key>.riv`: RIVE 7.0 with embedded images. No external
  PNG atlas is required at runtime.
- `/js/vendor/rive/rive.js` and `/js/vendor/rive/rive.wasm`: local
  `@rive-app/canvas@2.42.0`, with `manifest.json` and `LICENSE.txt`.

The existing `ui/embed.go` directory patterns include these files. Rebuild
AuraGo to serve them from the embedded UI. They are not yet part of a running
installed binary or an activated voice UI.

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

Load the local script lazily when opening the future voice view and set the
WASM URL before constructing a player. Apply AuraGo's build-version query key
to the script, WASM, catalog and Rive URLs. The following initialization is a
reference, not a new auto-starting UI module:

```javascript
// `rive` is provided by the local script; `entry` comes from the catalog.
// `versioned()` appends the current AuraGo BuildVersion as ?v=.
rive.RuntimeLoader.setWasmUrl(versioned('/js/vendor/rive/rive.wasm'));
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
    player.resizeDrawingSurfaceToCanvas();
    inputs.mode.value = 0;
  }
});
```

- `mode`: idle 0, listening 1, thinking 2, speaking 3, happy 4.
- `viseme`: rest 0, AA 1, EE 2, OO 3, FV 4.
- `mouthOpen`: 0–1; values at or below 0.08 close the mouth. Use the outgoing
  assistant audio and its phoneme/viseme timing when available.
- `headTilt`: −1 to +1, a small 2D neck tilt rather than a 3D side view.
- MCP uses display patterns; Terminator uses mechanical jaw poses.
- Pause when hidden or reduced motion is requested; resize with the view and
  call `player.cleanup()` when disposing it. Retain the PNG fallback on errors.

Speech poses are discrete. Cross-expression blinking and continuous lip morphs
remain prototype limitations. Silver revision 6 preserves the neutral face
contour while thinking; all characters use localized blink regions.

## Authoring project

The complete project has moved to the sibling directory `../personas/`.
Its current dataset collection is `../personas/personas/`, including the
switchable test page, all twelve source folders, builders, checks and ZIP.
The outer directory also retains the original Silver/Green prototypes,
research and diagnostic versions. See that project's `START-HERE.md`.

Only runtime payloads belong in AuraGo; update the catalog hashes and the local
runtime manifest when importing a newly reviewed revision.
