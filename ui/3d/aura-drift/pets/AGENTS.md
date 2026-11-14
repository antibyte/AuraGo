# Aura Drift mascot motions

## Purpose

Stationary animation subsets for optional trackside 3D mascots.

## Ownership

- `scripts/build-aura-drift-pets.mjs` writes nine motion files and `manifest.json`.
- `ui/js/desktop/apps/aura-drift-pet-decor.js` owns the roster, biome pairs,
  presentation placements and animation/resource lifecycle.
- Original GLBs and complete motions stay under `ui/3d/pets/`; their source and
  rights contract is `assets/desktop-pets/AGENTS.md`. Never overwrite them here.

## Local Contracts

- Use active, reviewed pets only; MCP, Snoopy, WALL·E and Webbi remain excluded.
- Three clips per pet: quiet idle plus suitable greeting/cheering gestures.
  Pin horizontal root translation; retain vertical breathing and articulation.
- Retain source/model/motion hashes, clip IDs and original CC0/GPL licences.
  Motion licences do not relabel character artwork.
- Maximum 512 KiB per pack and 2 MiB total. Reuse original GLBs through fixed,
  version-bound local URLs. No duplicate models or cloud requests.
- Six spots per track, beyond the whole road corridor and existing furniture.
  Reserve them from scatter using final scaled prop bounds. Keep tight inside
  curves clear. Never enter race physics or change replay state.
- Resolve standing height against the rendered terrain triangles. Its coarse
  meshes can rise above the analytic height and otherwise bury a mascot's feet.
- Show/animate at most one nearby mascot on Low, two on other tiers, within 105 m.
  Mascots behind the camera must not consume that visibility budget.
  Pause/reduced motion stop skeletal advancement. Asset failure omits mascots;
  disposal releases shared resources once and ignores late model completions.

## Work Guidance

Rebuild after source model/motion or roster changes. Review ground contact,
road clearance and gestures at track scale.

## Verification

`node scripts/build-aura-drift-pets.mjs --check`,
`node scripts/test-aura-drift-pets.mjs`, and
`AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestDesktopAuraDriftPetDecorBrowser`.

## Child DOX Index

None.
