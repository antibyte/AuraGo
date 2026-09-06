# Sprite production

Original illustrations generated with OpenAI Imagegen for AuraGo on 2026-09-06.
No third-party asset pack or existing game character was used. The repository
MIT license applies; its complete notice travels in each exported JSON.

Only `../catalog.json` and ten `../*/sheet.{png,json}` pairs are embedded. This
directory retains original PNGs and the explicit production manifest. Source
hashes also travel in metadata. Preserve originals; change pose/crop choices in
`manifest.json`, then regenerate.
Bump the pack's `version` when changing already-released artwork or metadata;
existing projects keep their earlier versioned copies.

## Production brief

Shared direction: fantasy/arcade pixel art, dark outlines, emerald green,
sapphire blue, violet, warm gold/orange and natural skin tones. Isolated objects
have true transparent backgrounds, no labels, grid lines or ground shadows.
Terrain may fill its tile. Side-view characters face right. Top-down characters
use up/right/down/left: ranger (red hair, green cloak), knight (silver armor,
blue cape), slime, goblin, wolf and boar. Keep body size and ground line stable.

| Source | Requested artwork |
|---|---|
| humans-side.png | Ranger/knight idle, walk, run, jump, attack, hurt and death |
| humans-top.png | Both humans in four directions, idle/walk/attack and equipment |
| humans-top-attacks.png | Four phases per human up/down attack; no detached projectiles |
| monsters-side.png | Four creatures with idle/movement/attack/hurt/death poses |
| monsters-top.png | Six movement poses in four directions per creature, four loot icons |
| effects.png | Explosion, campfire, smoke, portal, coin, chest, water, torch, door, trap |
| platformer.png | 40 terrain, 20 platforms/props, 10 items, 10 hazards, 20 animated frames |
| adventure.png | 50 terrain, 20 plants/props, 10 items, 20 chest/gate frames |
| space.png | 10 ships, 20 enemies, 20 projectiles, 10 asteroids, 10 pickups, 10 station parts, 20 explosions |
| blocks.png | 40 blocks, 10 paddles, 20 balls, 20 power-ups, 10 symbols |
| cards.png | 52 cards, 2 jokers, 4 backs, 12 chess pieces, 12 die faces, 8 counters, 10 fields |

The manifest is authoritative for pose selection and rectangles. Imagegen does
not reliably obey exact grids: reviewed explicit rectangles replace guessed
uniform positions. Repeated poses are intentional holds, not claims of 1,000
distinct drawings. Card IDs/corner ranks define values; pip placement is decorative.

## Rebuild and review

Use Python 3 and Pillow 12.2.0:

```sh
python scripts/pack_game_sprites.py
python scripts/pack_game_sprites.py --check
go test ./internal/gamemaker ./internal/server -run 'TestSpritePack|TestGameMakerAssetPack'
```

The packer rejects sources without real alpha, thresholds near-transparent
halos, scales without smoothing, enforces bounds and packs exactly 100 frames.
It never guesses a background from RGB. Shared scale and explicit anchors keep
alignment. Freestanding objects retain transparent cell borders.

In Studio, review every animation on light and dark backgrounds for clipping,
neighbor fragments, direction, actual motion, ground line and transitions.
Idle uses 6 FPS, movement 10 FPS, actions/effects 12 FPS. Creature resting
animations reuse existing poses. The first complete production check was the
ranger side-view walk, with JSON and a rendered animation preview.

Use embedded Phaser 4.2.1 for browser checks; the curated gameplay skill includes
the shared loading example. Keep PNG and JSON together in project exports.
