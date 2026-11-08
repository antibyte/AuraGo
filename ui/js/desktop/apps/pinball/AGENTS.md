# AuraPinball runtime

## Purpose

Three local 3D pinball tables, shared Rapier simulation and per-window Desktop game lifecycle.

## Ownership

- The sibling `pinball.js` owns `window.PinballApp` and lazy loading. This directory owns simulation, rules, input, scene, audio and views.
- `assets/pinball/AGENTS.md` owns Blender production and retained artwork. Payloads are under `ui/3d/pinball/v1/`.
- Registration is in `internal/desktop/types.go` and the Desktop loader/router; `pinball.*` belongs to all sixteen Desktop locale bundles.

## Local Contracts

- Use pinned Three.js 0.186.1 and Rapier 0.21.0 in an isolated local bundle. No game singleton, remote resources, backend routes or provider keys.
- Fixed physics at 480 Hz, SI units, 27 mm / 80 g balls, gravity aligned to a 6.5 degree playfield. Render interpolation never changes rules or contacts.
- Timestamp keyboard edges and consume them at simulation ticks so a short tap between render frames is preserved. Menu Space/Enter activation stays native.
- Motors and physical contacts drive flippers; all ramp flight is simulated. Tilt disables coils and bonus, never passive collision.
- Retry returned shooter balls through the physical launcher. Search each resting ball independently; bounded search failure replaces it through the trough without consuming a turn.
- One to four alternating players have independent mission/lock/score state. Training uses identical physics and never writes highscores.
- Pause and clear input on focus loss, hidden/minimized window or controller disconnect; resume explicitly. Dispose world, graphics, fetches, audio and listeners on close or stale load.
- Persist only validated settings and ten highscores per table. Never persist active physics worlds.
- Use `pb-*` classes, semantic menus, remappable controls and reduced-motion support. Translate visible copy into all sixteen locales.

## Work Guidance

- Keep DOM-free physics/rules modules testable from Node. Scene snapshots are read-only observations.
- Register local translation helpers in the existing i18n lint map.
- Build the runtime and shell bundles together after registration edits.

## Verification

- `npm run test:pinball`, `node scripts/build-pinball.mjs --check`
- `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run DesktopPinball -count=1`
- Existing app registry, disposal, locale and UI bundle checks.
- Hardware FPS and human flipper/catch acceptance are separate from software-rendered browser tests.

## Child DOX Index

None.
