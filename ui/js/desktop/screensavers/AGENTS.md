# Desktop screensavers

## Purpose
Lazy overlay host and the five screensaver scenes of the Virtual Desktop:
Tiefsee (`abyss`, 3D), Ereignishorizont (`event_horizon`), Nordlicht (`aurora`),
Flüssige Tinte (`ink`) and Sternenstaub-Uhr (`stardust`).

## Ownership
- Idle detection, suppression, wake and settings sync: `../core/screensaver-runtime.js`
  (main desktop bundle; exports `window.DesktopScreensaver` and
  `window.previewDesktopScreensaver`).
- This folder: `host.js` (overlay, clock, render lifecycle), `gl-kit.js` (WebGL2
  helpers, bloom, MRT), one file per scene, and `abyss-scene.js`, which is build
  input for `../../vendor/screensaver-abyss/abyss.esm.js`
  (`node scripts/build-screensaver-abyss.js`, `--check` must pass).
- Overlay CSS: `ui/css/desktop-screensaver.css` (lazy, never in the shell bundle).
- Posters/thumbnails: `ui/img/screensaver/` (see its `CREDITS.md`).
- Tiefsee models: `ui/3d/screensaver/abyss/v1/`, authored by
  `assets/screensaver-abyss/build_abyss.py`.

## Local Contracts
- Settings are installation-wide `screensaver.enabled` (default `false`),
  `screensaver.theme` (five scenes plus `random`), `screensaver.idle_minutes`
  (1, 2, 3, 5, 10, 15, 30, 60) and `screensaver.clock`. Backend allowlist,
  frontend defaults and the Settings pane stay in sync.
- The runtime never activates while the tab is hidden, a fullscreen element exists,
  an audible visible video plays, `#vd-sip-incoming` is shown, or an opaque iframe
  without SDK activity pings has focus. Same-origin frames get direct listeners;
  generated apps report `aurago.desktop.activity` through `aura-desktop-sdk.js`.
- Waking input (pointer, key, wheel, touch or >8 px movement) is swallowed in the
  capture phase, including the trailing click/keyup for 600 ms. Incoming calls stop
  the screensaver immediately. Preview ignores input for its first 800 ms.
- The overlay uses `--vd-z-screensaver` (20000): above the always-on-top pet (9999),
  below incoming calls (26000). It never locks the session.
- Scenes register with `AuraScreensavers.register(id, factory, meta)` and return
  `{ frame(t, dt), resize(w, h), setQuality?(level), stats?(), dispose() }`.
  Factories may be async; the host disposes results that arrive after a stop.
- The host caps DPR at 1.5 (1 on touch phones) and 3840x2160 pixels, calibrates the
  display cadence over 24 frames, caps at 60 fps (30 fps after 30 minutes), lowers
  resolution/quality after sustained slow frames, and releases GPU resources with
  `WEBGL_lose_context`/`forceContextLoss` on stop. Reduced motion, missing WebGL2,
  missing float render targets and context loss show the scene poster; the clock
  stays. Stardust shows only the date because the scene is the clock.
- Shaders are original code; do not paste Shadertoy or other CC-licensed shader
  sources. Clamp every `pow()` base. No `eval`/`new Function`, no remote assets;
  every URL goes through `AuraLazyAssets.versionedURL`.
- Tiefsee uses the isolated MIT Three.js 0.186.1 bundle; the separately bundled shared Three.js global and
  the System World bundle stay untouched. Bloom never blends into a multisampled
  buffer (the renderer runs without MSAA).
- Keep every JS file below the desktop line budget and all strings in the 16
  `ui/lang/desktop` locales (`desktop.screensaver_*`, `desktop.settings_screensaver_*`).

## Verification
- `go test -count=1 ./ui -run 'Screensaver|DesktopSettings'` and
  `go test -count=1 ./internal/desktop`.
- `$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test -count=1 ./ui -run 'TestDesktopScreensaver.*Browser'`
  (SwiftShader). Add `AURAGO_SCREENSAVER_GPU=1` for the local GPU,
  `AURAGO_BROWSER_ARTIFACT_DIR` for screenshots, `AURAGO_SCREENSAVER_LONG=1` for four
  frames per scene and `AURAGO_SCREENSAVER_FAKE_CLOCK=<ISO time>` to exercise
  Stardust's logo minute.
- `node scripts/build-screensaver-abyss.js --check`, `npm run build:ui -- --check`,
  `python assets/screensaver-abyss/check_assets.py` and
  `node scripts/validate-screensaver-glbs.mjs`.

## Child DOX Index
None.
