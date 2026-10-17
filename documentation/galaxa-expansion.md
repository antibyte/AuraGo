# Galaxa Deluxe: animated expansion

The expansion adds 50 original animated motifs to the existing arcade game:
12 asteroids, 12 aliens, six weapon appearances, ten detailed planets and ten
combat effects. The 70 transparent source PNGs contain 684 frames; pickup aliases
and existing Mega upgrades reuse these frames. Existing ships, enemy artwork,
scores, unlocks and saved settings remain compatible.

## Content and encounter rules

| Motifs | Content | Runtime use |
| --- | --- | --- |
| 01–12 | Crater boulder, basalt monolith, iron meteor, ore geode, ice boulder, frost comet, crystal cluster, prism shard, lava boulder, slag core, wreck fragment, hollow asteroid | Separate asteroid encounters from stage 6; three sizes, material-specific destruction |
| 13–14 | Nebula ray, spore hunter | Nebula: `butterfly`, `stalker` |
| 15–16 | Ore cracker, armor crab | Asteroid: `bomber`, `shield_bee` |
| 17–18 | Prism falcon, splinter polyp | Crystal: `sniper`, `splitter` |
| 19–20 | Lightning jelly, storm wasp | Storm: `spinner`, `hunter` |
| 21–22 | Gravity spider, phase isopod | Blackhole: `weaver`, `teleporter` |
| 23–24 | Void runner, swarm carrier | Void: `kamikaze`, `carrier` |
| 25–30 | Proximity mine, EMP mine, cluster mine, seeker rocket, swarm rocket, seeker torpedo | Existing power-up collection and controls |
| 31–32 | Ocean, terran | Nebula planets |
| 33–34 | Desert, crater | Asteroid planets |
| 35–36 | Ice, crystal | Crystal planets |
| 37–38 | Storm gas giant, ringed | Storm planets |
| 39–40 | Lava, bioluminescent night | Blackhole and Void planets |
| 41–45 | Rock shatter, metal sparks, ice shatter, crystal shatter, lava ejecta | Asteroid material effects |
| 46–50 | Alien plasma, target lock, rocket exhaust, EMP wave, torpedo shock | Alien destruction, seeker targeting/flight, mine/torpedo impacts |

Art identity (`artId`) is separate from the existing enemy combat role. Every
third regular formation slot uses the sector pair from the sector's second
normal wave onward. Boss and bonus slots are excluded. Classic, Mirror and
Daily retain their 30 stages and six sector bosses. Endless/Hyperdrive use the
same sector pools; Gauntlet assigns each pair to two of its twelve waves.
Boss Rush keeps its six boss encounters, with planets and the weapon drops.

Asteroids have 1/3/6 HP. Large rocks split once into two small fragments, with
one score reward per original rock. The pool reserves fragmentation capacity
within its 20-rock limit. Rocks enter from above the field after a 900 ms
warning, leaving a 160-pixel corridor around the ship's stage-start lane.
They intercept enemy shots and use the existing ship shield/invulnerability
rules. They never count as enemies or hold up wave completion, and do not
appear in boss or bonus encounters.

## Weapons and controls

Collect weapon pickups normally. Fire launches rockets; mines are placed
automatically. New variants are rare drops. Mine power-ups last ten seconds,
rocket power-ups twelve. Collecting the same new variant refreshes its timer;
the original proximity mine and seeker retain their existing Mega upgrades.
HUD names, control hints and loading feedback are translated in all 16 locales.

| Weapon | Interval | Behavior |
| --- | --- | --- |
| Proximity mine | Existing 780 ms; Mega 520 ms | 1 damage within radius 52; existing Mega double placement |
| EMP mine | 1,000 ms | Clears enemy shots within radius 72; slows normal enemies by 50% for 1.5 seconds; no damage or boss slowdown |
| Cluster mine | 1,000 ms | 1 damage within radius 40; six short-lived fragments |
| Seeker rocket | Existing 380 ms; Mega 280 ms | Existing homing/splash behavior and Mega salvo, with animated targeting and exhaust |
| Swarm rocket | 850 ms | Three homing shots, preferably distinct targets; 1 damage each, no splash |
| Seeker torpedo | 1,200 ms | Slow homing projectile; 4 direct damage and 1 splash damage within radius 70 |

Mines arm after 450 ms, expire within nine seconds and have a twelve-mine cap.
Seekers reacquire living, visible, vulnerable enemies; without a target they
continue forward and expire within six seconds. A blast hits each eligible
target once. Damage uses the shared boss phase rules, and terminal enemy
rewards are centralized in `galaxa-entities-rewards.js`, including smoldering
crash dives. Mirror projectiles retain their existing ghost-damage rule.

## Planets, loading and memory

The draw order is sector scenery, distant planets, stars, gameplay, then HUD.
Planets drift at 0.8–1.6 logical pixels/second with at most two pixels of lateral
parallax. Their 180–340-pixel appearance uses source frames of at least 512×512.
Twelve frames form 24–48-second atmosphere cycles, blending adjacent frames.
Warp and camera shake do not affect them. Focus loss and pause stop movement;
Reduced Motion displays a still frame.

Gameplay sheets retain the existing preload before the title screen. Planet
sheets are marked `optional: "planet"` in `ui/img/galaxa/atlas.json` and loaded
on demand. Each instance retains at most two planet sheets, including a sector
crossfade; disposal/reset fences late loads and releases their references.
Planets draw directly from the source image with smoothing and never occupy
the 768-entry gameplay sprite cache. Only planets use smoothed scaling.
Failed optional loads retain the sector scenery and expose a localized Retry.

Each planet uses three four-frame sheets, avoiding one large twelve-frame
texture. The original PNGs increase download and decoded-image memory. The
resource archive limit is 768 MiB; expanded resources remain limited to 1 GiB
and 20,000 files. See [external web resources](web-assets.md) for the versioned
package and installation contract. No library, HTTP API or database is added.

## Source files and reproducible checks

- `assets/galaxa-expansion/catalog.json`: exactly 50 motifs and their production brief.
- `assets/galaxa-expansion/sources/*.json`: original `image_gen` output names, prompts and references.
- `ui/img/galaxa/exp-*.png`: unchanged original transparent PNG outputs.
- `ui/img/galaxa/atlas.json`: frame rectangles, anchors, timing, loops, optional sheet policy and the explicit expansion list.
- `ui/img/galaxa/provenance.json`: source names, prompts, byte sizes and SHA-256 hashes.

Regenerate metadata with `node scripts/build-galaxa-expansion.mjs --write`.
The Go helper only reads PNG pixels to find transparent grid cuts and stable
source anchors; it never resizes or re-encodes the artwork. Repair artwork with
`image_gen`, replace its source record, then regenerate the metadata.

Run these focused checks from the repository root:

```text
node scripts/build-galaxa-expansion.mjs --check
node scripts/test-galaxa-assets.mjs
node scripts/test-galaxa-expansion.mjs
node scripts/build-ui-bundles.js --check
go test ./internal/webassets ./cmd/assetpack
go test ./ui -run 'Galaxa|AdaptiveMusic' -count=1
```

Set `AURAGO_RUN_BROWSER_SMOKE=1` for the browser suite. Set
`AURAGO_BROWSER_ARTIFACT_DIR` to an ignored `reports/` directory for the five
motif galleries, actual Desktop screenshots, audio and performance records.
`TestGalaxaExpansionBrowser` checks all seven modes, six weapons and identical
Daily simulation at 30/60/144 render FPS. `TestGalaxaPlanetLifecycleBrowser`
injects a real HTTP load failure and checks retry, pause/focus, Reduced Motion,
frame-sheet boundaries, transitions and disposal. PNG checks validate crops,
alpha, anchors, distinct frames, native planet resolution and provenance.

For the real-time acceptance run, set `AURAGO_GALAXA_SOAK_SECONDS=1800` and run
`go test ./ui -run '^TestGalaxaArcadeSoak$' -count=1 -v -timeout=35m`.
It cycles all six weapons during play and checks resource caps, browser errors,
cleanup and a median frame rate of at least 55 FPS. Local browser acceptance
does not deploy or activate a resource set on another device.
