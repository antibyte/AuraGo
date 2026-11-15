# Purpose

Leuchtfeuer is an offline harbour walk on the virtual desktop. The player collects glass, wick and oil in any order and lights the tower.

# Ownership

This folder owns the world, the pure simulation, the Three.js view, the presentation binding and the window. The catalog record belongs to `internal/desktop`. The stylesheet, icon and sixteen locale files belong to `ui/css/desktop-app-leuchtfeuer.css`, `ui/img/leuchtfeuer/icon.svg` and `ui/lang/leuchtfeuer/`. Game Maker packs stay in `internal/gamemaker/asset_packs`; this game does not copy them.

# Local Contracts

- Load only the sixteen allowlisted `aurago-pirates-3d` models, and only `lods[0]`. Paths come from the manifest. The view imports the published runtime modules under `/api/game-maker/asset-packs/runtime/`.
- Water is the coast presentation (`water-ocean`) plus the simulation. There is no second ocean mesh.
- One camera frame: orbit yaw places the camera, and movement reads `camera.getWorldDirection` through `wishFromCamera`. Models with manifest forward `+Z` mount with `rotation.y = π` on parent −Z.
- Session key `aurago.leuchtfeuer.v1` in `sessionStorage`. Each window owns its renderer, loads and `AbortController`.
- Sound stays muted until a user gesture and the pause-menu toggle. A hidden, minimized or inactive window pauses the simulation, the mixer and the audio.
- Visible copy is `leuchtfeuer.*` in all sixteen locales, plus `desktop.app_leuchtfeuer`. The HUD uses `textContent`. CSS prefix `lf-`; the sheet is exempt from `--vd-mat-*`.
- No server, account, score list or network beyond the asset route.

# Work Guidance

Keep the simulation free of DOM and Three.js. Do not change Studio jobs, prompt profiles or the asset route. Leave `aurago-low-poly`, `aurago-isometric` and the sprite atlases untouched.

# Verification

- `node scripts/test-leuchtfeuer-sim.mjs`
- `go test ./ui -run TestLeuchtfeuerRegistration -count=1`
- `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestLeuchtfeuerBrowser -count=1`
- `node --check` on `world.js`, `sim.js`, `view.js`, `audio.js` and `app.js`
- Regenerate `ui/js/desktop/bundles/main.bundle.js` with `npm run build:ui` after a shell registration change.

# Child DOX Index

None.
