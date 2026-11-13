# Goldfish Visual Pointer References

## Purpose

Independent artwork coordinates for every Goldfish hotspot and exit, verified
against complete Desktop app renders rather than authored hit rectangles.

## Ownership

- `R01.json` through `R16.json` own target IDs, visible landmarks, normalized
  interior points, ending/route variants and genuinely empty background points.
- Runtime and browser harness ownership remains with
  `../../js/desktop/apps/goldfish-adventure/AGENTS.md`.

## Local Contracts

- Five targets per room; at least three visible interior points per target and
  points on every separate component. Exits use the visible direction signs.
- Normalize against the actual canvas bounds recorded beside the app capture.
  Do not copy actor positions from isolated render tests or derive reference
  points from hit rectangles. Hover success alone does not prove click success.
- `visualSignature` hashes room staging, actor frame/pivot metadata and referenced
  PNG bytes. Graphics changes require a fresh full-scene visual review before
  updating the hash; blindly regenerating hashes is not acceptance.
- Keep state fixtures in the reducer-built test helper; never fabricate puzzle
  flags or write to the user's installed game saves.

## Work Guidance

- Luna Extra High agents own disjoint scene groups and independent cross-review.
  Preserve contrary image evidence when a pointer test fails; fix the runtime
  unless a screenshot proves the reference itself was wrong.
- Reports and before/after screenshots stay in ignored `reports/`; retain only
  reviewed normalized reference data here, without machine-local paths.

## Verification

- `TestGoldfishAdventureHotspotReferenceIntegrity` checks signatures and points.
- `TestGoldfishAdventureHotspotTargetsBrowser` checks all points, Look reactions,
  negative space and exits in every reachable visual state and all four profiles.
- `TestGoldfishAdventureHotspotActionsBrowser` checks actual bar/inventory actions;
  `TestGoldfishAdventureHotspotCinematicRecoveryBrowser` checks finish/skip recovery.

## Child DOX Index

None.
