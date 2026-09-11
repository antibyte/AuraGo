# Game Maker presentation production

- Own content/code is MIT; recordings must be individually verified CC0.
  `sources/sources.json` retains URLs, authors and original SHA-256 values.
  Source archives are excluded from runtime data.
- `catalog.py` owns IDs. `build.py` masters PCM16/48 kHz WAVs and catalogs;
  `build.py --check` verifies exact output. Rebuild JavaScript with
  `node scripts/build-game-maker-presentation.js`. Pinned Three Water has one
  documented build-time patch exposing its render target disposal.
- The game owns update/render timing. Helpers must never add their own timers
  or animation loops. Reset clears particles, decals and voices; dispose
  restores host materials and releases GPU/audio resources. Exclude HUD filters.
- Limits: 64 MiB additional uncompressed runtime, 4,000 3D particles, 1,500 2D
  particles, 96 decals, 32 voices and four ambience layers. Import only selected
  IDs/dependencies through the existing atomic publication transaction.
- Verify `TestPresentation*`, opt-in `GAMEMAKER_PRESENTATION_BROWSER=1`, and all
  guided templates with `GAMEMAKER_GUIDED_BROWSER=1 GAMEMAKER_PRESENTATION=1`.
  Review actual images/audio; code checks are not design acceptance.
