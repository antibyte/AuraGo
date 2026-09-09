# Leafy desktop plant

Enable **Leafy** in the desktop Widget Manager, open its pot or the leaf button in
the taskbar, then choose **Plant**. It starts as a seedling and grows across open
windows. Uncovered areas stay clickable. The shell and care palette stay above
the foliage. A healthy, uncut stem adds one visible segment and a leaf every
complete hour. Side branches appear every twelve growth hours; cut stems rest
for six hours before producing a new shoot.

Water every 24–36 hours and fertilize weekly. Neglect first stops growth, then
causes wilting and eventually death. Water and nutrients can restore a living
plant; a dead plant needs replanting. Frequent watering does not accelerate time.

Use **Prune** to select a stem on the desktop, inspect its highlighted growth and
confirm **Cut here**. The branch selector also supports keyboard operation.
**Trim to pot** cuts all vines after confirmation. A cut can be undone for
30 seconds; healthy cut stems produce new shoots after six biological hours.
Esc puts the scissors away. Drag the grip below the pot, or focus it and use
arrow keys to reposition the plant.

**Vacation** freezes growth and care, including while the desktop is closed.
End vacation to continue without catching up the paused time. Hiding the
foliage or disabling the widget only changes visibility: care continues.
Use vacation before a longer absence. All desktop Spaces and browsers share
one plant; pot placement is saved separately in each browser.

## Runtime and persistence

- Existing local Three.js r128, loaded only with the widget. Botanical leaf textures on eight curved
  mesh variations, tapered stems, ceramic pot, moss, tendrils and seasonal flowers.
  Fruity light uses ivory ceramic; dark themes use graphite.
- One transparent foreground canvas below shell/menu/modal layers. Ordinary
  foliage is click-through; scissors explicitly capture canvas input.
- A maximum of 64 branches, 36 nodes per branch, 960 sampled leaves and 80
  flowers. Stable sampling distributes leaves across the complete plant.
  Frame geometry is bounded below 320,000 triangles and 35 draw calls.
  This replaces the initial 150,000-triangle target to preserve curved leaves.
- Pixel ratio at most 1.5 and four million pixels. No continuous animation loop:
  occasional three-second breezes render at most 30 FPS, followed by 22 seconds
  of rest. Reduced motion, disabled animations, vacation, death and hidden
  documents suppress breezes. Geometry rebuilds happen on state/viewport
  changes, never per animation frame.
- Without WebGL, or after context loss, Canvas 2D uses the local atlas baked from
  the same botanical textures and procedural models. Care and pruning use the same state and geometry.
- Versioned JSON in existing SQLite `desktop_meta`, key `leafy.state.v1`.
  No new database, cron job, LLM request or external notification.
- `GET /api/desktop/plant` computes the current snapshot without storing a
  simulation update. `POST /api/desktop/plant/actions` advances time and commits
  one action atomically, respecting desktop write permissions and read-only mode.
- Actions: `replant`, `water`, `fertilize`, `prune`, `trim`, `undo_prune`,
  `vacation`. Requests include `action_id` and `revision`. The most recent
  32 action receipts prevent repeated writes; stale revisions return HTTP 409
  with a fresh snapshot. A pending failed action is retried with the same ID.
- Server UTC owns biological time. Pauses and partial hours survive reloads.
  Negative clock differences are ignored. Unattended simulation stops at
  death, so arbitrarily long absences require bounded work.
- Existing `plant_changed` desktop events refresh visible clients; reconnects
  and visibility restoration also fetch the current state. No plant state is
  written into the ordinary widget config.

## Artwork and checks

The original transparent, four-leaf texture sheet is
`ui/img/leafy/leaves-natural.png`, generated with the built-in Imagegen tool.
Its full authoring prompt is retained in `ui/img/leafy/leaves-prompt.txt`.
The renderer derives each blade's bounds and petiole attachment from its alpha
channel, locates the actual transparent gutters and repacks the blades with
16 pixels of padding before applying eight gently curled and twisted meshes. Seeded variation
changes each leaf's proportions, size, tilt and petiole length without flickering
between state updates. Leaf edges, small veins and mottling come from the local
texture; dry leaves retain their detail while losing green pigmentation.
Procedural models remain in `ui/js/desktop/leafy/renderer.js` and `geometry.js`.
Icons are original AuraGo SVGs. No remote assets are required.
The eight-cell, transparent 1024×512 fallback atlas contains four leaf poses,
graphite and ivory pots, and two flower colors.

Rebake the atlas explicitly after artwork changes, then run runtime tests in a
separate invocation against the new image:

```powershell
$env:AURAGO_RUN_BROWSER_SMOKE='1'
$env:AURAGO_LEAFY_BAKE='1'
go test ./ui -run '^TestDesktopLeafyGraphicsBrowser$' -count=1 -v
Remove-Item Env:AURAGO_LEAFY_BAKE
go test ./ui -run '^TestDesktopLeafyRuntimeBrowser$' -count=1 -v
go test ./internal/desktop ./internal/server -run Leafy -count=1
node scripts/build-ui-bundles.js --check
```

Browser screenshots and measurements go to ignored `reports/leafy/`.
The graphics test covers four ages, wilting, death and ivory ceramic. The real
desktop test covers three themes, 1920×1080, 1366×768, narrow touch layout, DPR 2,
pruning/undo, vacation, hiding, remounting, Spaces, context loss and idle rendering.
Wall-clock simulation tests cover a year of care, long neglect, partial-hour
vacations, recovery, persistence, retries and concurrent actions.

Build a matching resource set and binary as described in
[web-assets.md](web-assets.md), and validate with `--check-assets` before installing.
