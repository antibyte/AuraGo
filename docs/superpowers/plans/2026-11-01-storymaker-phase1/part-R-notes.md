# Part R notes: the player (C) against the real starter catalog (D), and A + C + D integrated

Worktree: `C:/Users/Andi/AppData/Local/Temp/claude/C--Users-Andi-Documents-repo-AuraGo/f2c97a7f-addd-4aa7-bd49-85d6c9b3f2a1/scratchpad/sm-wt-r` (base `0248bc133` plus the wave-1 outputs of A, C and D; nothing committed).
Plan part: `part-R.md` (9 tasks, R1 to R9, 1831 lines). It is generated from the verified files by `make-plan-r.mjs` from the edit data `plan-r-data.mjs`; `sim-r.mjs` replays every task in a scratch tree (`sm-tmp-r`, the wave-1 state of C and D): each new check fails first, each task ends green (53, 56, 57, 58, 59, 61, 64, 71 checks), and the final tree is **byte-identical** to the worktree (117 files compared, results in `sim-r-results.json`).

## Result

- `node scripts/test-storymaker-player.mjs`: **71 passed, 0 failed** (C's 53 plus 18 new checks in `scripts/storymaker-tests/11-real-catalog.mjs`), about 2 s.
- `npm run build:ui` and `npm run check:ui` green; `node scripts/build-storymaker-catalog.mjs --check` green (`storymaker catalog ab33e88a6b77 is up to date`); D's seven node test files green (6, 3, 5, 4, 3, 4, 6).
- `go test ./internal/storymaker/... -count=1` ok (new `catalog_real_test.go`: real catalog loads, every referenced file exists, `minimal.json`, `firewall.json` and the cast sheet validate with zero issues before and after `ApplyDefaults`). `go test ./ui -run 'TestStorymakerCatalog|TestShippedWebAssetsLoadNothingExternal|TestTranslations_'` ok, `go vet ./ui ./internal/storymaker/...` clean, my Go files are gofmt clean.
- Browser tests (installed Chrome 156, through `go test`, sandbox off, `GOTMPDIR=D:\tmp\go-build`): the four existing C tests PASS (firewall render still bit-identical twice, 56 frames/s at 640x360; screenshot transport 0 of 230400 pixels differ) and the new `TestStorymakerParityBrowser` PASS in 136 s (cast sheet 27 frames 79 s, firewall 16:9 21 s, 9:16 22 s, 1:1 14 s).
- Stability: with six instances of the suite at a time, C's own suite failed 6 of 18 runs (real 250 ms frame deadline); after R1 18 of 18 pass.

## Files

New: `scripts/storymaker-tests/11-real-catalog.mjs`, `internal/storymaker/testdata/player/castsheet.json` (27 scenes), `internal/storymaker/catalog_real_test.go`, `ui/storymaker_parity_browser_test.go`.
Changed (C): `scripts/storymaker-test-support.mjs` (`playerStory`, `REAL`, `realCatalog`, `realFiles`, `realAssets`, `renderAssets(extra, files)`, `TEST_LIMITS`), `scripts/storymaker-tests/02-stage-parts-text.mjs` (desk expectation), `08-host-app.mjs`, `09-host-render.mjs` (generous limits), `ui/js/storymaker/{rig,render,text,stage,beats}.js`, the regenerated `player.bundle.js` (`host.bundle.js` did not change).
Changed (D): `assets/storymaker/lib/environments.mjs` (light `text` token on dark variants) and the regenerated `ui/img/storymaker/environments/*.json` + `catalog.json`. New catalog version **`ab33e88a6b77`** (was `70d5cf51a9a9`). I also patched `part-C-notes.md` (the "Rig semantics" paragraph, five places marked "part R") and appended an update section to `part-D-notes.md`.
Not touched: A's Go code (only a new test file), the section 6, 7 and 8 interfaces, `package.json`, the catalog-mini stubs.

## Mismatches found and fixed (each with a failing Node test first)

1. **Seated root offset** (R2). D: `pose.offset_y` 215 (rig units, positive = down) and `rig.seat_drop` 215; C used the `lower_leg_l` length 205 (6 px too high on screen). Now `SM.rig.poseDrop(rig, pose)`: `pose.offset_y` when a number; else for a `seated` pose `rig.seat_drop`, then `rig.seated_offset`, then the lower leg, then 0.24 x units_height; the drop blends with the pose blend (two seated poses do not add up). Check: hip joint of the typing woman at the desk spot is within 15 px of D's pinned chair seat (774 px).
2. **Text box units** (R3). `box.pad` and `box.radius` are fractions of the canvas height (caption 0.012 gives 13 px at 1080); `layout` returns `radius`; a box without numbers keeps 0.4 em padding and 0.3 em corners. C notes text updated.
3. **Point poses when mirrored** (R4). `poseFor(cat, name, facing)` swaps `point-left` and `point-right` (their bones, hands and `view` hint) for a left-facing actor when both entries exist. Test: the hand tip of all four name x facing combinations lands on the named screen side, computed from the bone matrices.
4. **Gesture hands** (R5, found by reading D's data, not in the brief). D's `gesture.count` sets `hands: {r: "point"}` only at t = 0.2 and expects it to hold; C's "a key without hands clears" opened the hand at t = 0.35. Now hands hold per side until a later key sets that side; at `u >= 1` the pose hands return.
5. **Environment layer order** (R6). C sorted layers by `parallax`; D lists them back to front and parallax is not monotonic (office: wall 0.1, floor 1, furniture 0.7; abstract-tech: ... floor 1, hud 0.7), so furniture was painted under the floor. List order is the draw order now; parallax only scales the camera pan.
6. **Spots in 9:16 and 1:1** (R6, found in the parity run: the seated woman sat beside her chair). C squeezed every spot without an override into the slice; D overrides only spots that would fall outside and draws furniture (desk, chair) at the design position. A spot without an override keeps its design position and is squeezed only when outside the slice (3 percent margin), so the mini catalog (no overrides) still works; an override may carry `face`. The existing mini test of the desk spot was updated.
7. **Text colours on dark environments** (R7, found in the parity run: the title was invisible on every night variant and on abstract-tech, the label box stayed beige). Text colours and the label box fill now resolve from `env.tokens[variant]` first, then the palette (the same chain as the environment's own shapes). D's dark variants carry a light `text` token (plain-stage, office, home-office night `#f2f4ff`; abstract-tech day and night `#f4f7ff`, plus `bg2` `#0e1a47`/`#070b22`, none of its shapes uses `$bg2`). The test pins the contrast (WCAG >= 4.5) of the label ink on its box for every environment x variant x palette and that every dark variant has a light `text`.
8. **Camera reach** (R7, found in the parity run: a close framing cut the head off). `ACTOR_REACH` (camera target height above the feet) was 200 px, tuned to the 0.36 stub rig; the real rig is 0.55 of the height, so 297 px. The manifest carries no `rig.height`, so it stays a constant.
9. **Flaky host tests** (R1). C's 08/09 tests drive the host with real timers and the production deadlines; they fail by chance on a busy machine. `TEST_LIMITS` (generous deadlines) is the default of both helpers, tests that are about a deadline pass their own limits. Module 11 also awaits the pending async checks of earlier modules before its own long synchronous work.

## Checked against the real data, no change needed (guarded by module 11)

Limb sides per view (visual: near/far arm order, pointing to the named side in front and three-quarter), slot params incl. `hair_back` keyed by `hair`, eyes/brows `<value>/<state>` keys with the `/neutral` fallback (every character x 3 views x 10 expressions x 34 slots resolves to a variant or a deliberate `none`; every variant has all three views), expression mouths and hand shapes named by poses and gesture keys exist, character params and the outfit mapping to `top/bottom/shoes` (no `#ff00ff` anywhere), body types (rest overrides merge per coordinate, `child` hips y -375, `elder` scale 0.97), prop `size.h` and `anchor` (the padlock's bottom centre lands on its spot, exact transform asserted), environment `tokens[variant]` and `$tokens` in gradient stops, spot `depth` (synthetic spot), `aspects` focus/spot overrides (every spot of every environment inside the slice, override positions exact), cycle keys (`offset_y` positive = down: 21 at the walk contact, `walk.speed` 16 gives the right walk seconds), `pose.view` hints, text styles and `style` on `show` beats (A's validator accepts it: the Go guard has zero issues), fonts (name `default`, weights 400/500/600/700: the three text styles need `font:default/500`, `/600`, `/700`).

## Visual parity verdict

PNG folder (git-ignored): `C:/Users/Andi/AppData/Local/Temp/claude/C--Users-Andi-Documents-repo-AuraGo/f2c97a7f-addd-4aa7-bd49-85d6c9b3f2a1/scratchpad/sm-wt-r/disposable/storymaker-parity/` (70 files). D's sheets: `.../scratchpad/sm-wt-d/disposable/storymaker-review/{cast-sheet,env-prop-sheet,variety-sheet}.png`.

- `castsheet-NN-<scene>.png` (1920x1080, one frame 0.7 s into each of the 27 scenes) and `castsheet-sheet-01..05.png` (2 x 3 half-size tiles, the `.txt` lists the tiles); `firewall-{16x9,9x16,1x1}-<scene>-<pct>.png` (7 frames per aspect) and `firewall-<aspect>-sheet-0N.png`.
- **Verdict: matches D's intent.** Shapes, colours, proportions and draw order of the four characters are the same as D's sheets in front, three-quarter and profile; mirrored actors are exact mirror images; the ten poses, nine gesture peaks, ten expressions and the walk cycle in profile look like D's cells. The seated woman sits on the office chair and on the home-office chair with hips at the seat, thighs forward and feet on the floor, in 16:9 and (after fix 6) in 9:16 and 1:1; the desk is in front of her. `point-left` and `point-right` reach the named screen side whichever way the actor faces. Props stand on the floor line (bottom centre on the spot), scaled by the floor depth; layers draw back to front (sky, hills, floor / wall, floor, furniture; network, cards, floor, hud) and the parallax layers move less than the floor when the camera pans (`castsheet-27-cam-pan.png`). Text boxes have D's padding and radius, Geist renders with ä ö ü ß, a long caption wraps to two lines inside the safe area. The fade into the code scene composites correctly.
- **Differences that remain and why:** (a) the player adds a soft ground shadow under every actor (C's choice, D's sheets have none); (b) the idle cycle moves the torso and arms by a pixel or two, D's sheets are static; (c) gestures are eased (ease-in-out per segment), D's evaluator is linear: same peaks, different in-betweens; (d) `enter`/`exit` default to 0.6 s as the spec says, so a long entrance (about 800 px) runs far faster than the walk cycle's stride (16 % of the width per second): it looks like gliding; the spec's own words are "enter takes 0.6 s", so I left it and list it below; (e) the labels in the cast sheet scenes are my own, D's sheets use SVG text.

## Art weaknesses and follow-ups for later (none blocks phase 1)

- D (already in its notes): long mannequin-like arms (pointing arms reach about 270 px), profile hair of short styles reads as a cap, `balding` profile is a wedge, `child` body is a small adult.
- `thumbs-up` reads as a raised fist: the `fist` hand variant has no visible thumb.
- No blink variants in the rig (`eyes` keys `<value>/blink`): the player supports a blink but never triggers it.
- In `gesture celebrate` the it-tech character shows a thin horizontal grey line next to the hoodie hem (see `castsheet-13-gest-2.png` around x 1275, y 711). Not analysed whether D's SVG renderer shows it too (D's sheets render only the woman for gestures).
- `prop.coffee-cup` has `hold: true` in the catalog; the player ignores `hold` (no prop in hand yet).
- `enter` and `exit` default durations: a distance-based default (the walk time at `cycle.walk.speed`) is a one-function change in `beats.js` (`kinds.enter.dur`, `kinds.exit.dur`) and would fit the walk cycle; it needs a spec decision because the spec says 0.6 s.
- A dark text token is the only environment-driven colour; a `title` over a light prop or over a mid-tone sky can still have low contrast. A catalog-wide rule (a `tone: dark|light` per environment) would be the next step; for now the dark variants carry `text`.
- The camera reach constant (297 px) fits the 0.55 rig; a catalog with a different `rig.height` should publish it in the manifest (`rig.height`), then `ACTOR_REACH` derives from it.

## For B and E

- **No change at the section 6, 7 and 8 interfaces** (host, render page, protocol messages, compile report shape, asset keys).
- **The catalog version changed to `ab33e88a6b77`** (D's environment JSON files and `catalog.json`). Any test that hard-codes `70d5cf51a9a9` (B's `status.catalog_version`, E's fixtures; both briefs quote it) must read the version from the catalog or use the new one.
- Text colours now depend on the scene's environment variant; a thumbnail or a render of a night scene shows light titles. Nothing to do on the app side.
- The opt-in parity harness (`ui/storymaker_parity_browser_test.go`) is a working example of the render page plus ingest against the **real** catalog and fonts (boot.json with four Geist faces taken from `catalog.json`, `{stop:true}` after the last wanted frame, strict frame order); B can reuse `runStorymakerParity` for its parity and determinism tests. The cast sheet story is `internal/storymaker/testdata/player/castsheet.json` (zero issues in Go).
- Preview widths: in 9:16 and 1:1 the stage shows a slice of the design frame; a spot keeps its design position (the app inserting an actor at `desk` gets the woman on the chair).

## Contract deviations and additions

1. Section 3a colour tokens: texts (and the label box fill) resolve from `env.tokens[variant]`, then the palette; props still from the palette only.
2. Section 3 `aspects.<ratio>.spots`: a spot without an override keeps its design position and is squeezed into the slice only when it would be outside (3 percent margin); an override may carry `face`.
3. Section 3a pose `offset_y`: the root drop of a pose (rig units, positive = down); seated poses without it use `rig.seat_drop`, then `rig.seated_offset`, then the old fallback.
4. D's catalog data: light `text` tokens in the dark environment variants, `bg2` in abstract-tech, new version `ab33e88a6b77`.
5. New files outside the ownership table: `internal/storymaker/testdata/player/castsheet.json` (A's testdata folder, new subfolder `player/`), `internal/storymaker/catalog_real_test.go`, `ui/storymaker_parity_browser_test.go`.

## Open issues

- GitNexus: `impact` for `rigState`, `layout` and the other edited functions returned nothing (the files exist only in the uncommitted wave-1 outputs, the index of the main checkout does not know them); callers were confirmed by grep (all inside `ui/js/storymaker` and its tests). `detect_changes` belongs to the commit step of the plan (CLAUDE.md), not run here because nothing is committed.
- The parity PNGs are reviewed by eye by me only; the machine-checked part is frame size, no compile error, no `#ff00ff` pixel and not flat.
- Chrome is the only browser tried. The parity test needs about 2 minutes and is opt-in (`AURAGO_RUN_BROWSER_SMOKE=1` and `AURAGO_STORYMAKER_PARITY_DIR=<absolute dir>`; `AURAGO_STORYMAKER_PARITY_ONLY=castsheet|firewall` filters the jobs).
- Part C's host tests are now independent of the machine speed, but the playback test (`seek, play with frame dropping`) is inherently time based; it passed in all 18 loaded runs after R1.
