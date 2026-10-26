# AuraGo chess piece kit

Original Blender source for the AuraGo 3D chess board. The kit is authored from
scratch for this repository and ships under the MIT license; no third-party
model, texture or scan is used.

## Contents

| File | Purpose |
| --- | --- |
| `generate.py` | Blender 5.2 LTS script that builds and exports the whole kit |
| `aura-chess-pieces.blend` | Editable source scene produced by the script |
| `AGENTS.md` | Local contract for this pack |

## Regenerating

```bash
blender --background --factory-startup --python assets/chess-3d/generate.py
```

The script writes `ui/3d/chess/v1/<piece>.glb`, `ui/3d/chess/v1/manifest.json`
and `ui/3d/chess/v1/LICENSE.txt`, then saves the source scene. Afterwards run:

```bash
node scripts/test-chess-assets.mjs
```

## Design

The set is a clean, modernised Staunton: turned bodies with crisp chamfers, a
carved knight head and a compact crown. Every piece is authored in metres on a
55 mm board square with its base centred on the origin, so the runtime places a
model on a square without a per-model offset.

| Piece | Height | Base radius |
| --- | --- | --- |
| pawn | 0.040 m | 0.0155 m |
| knight | 0.068 m | 0.0180 m |
| bishop | 0.080 m | 0.0180 m |
| rook | 0.070 m | 0.0175 m |
| queen | 0.089 m | 0.0190 m |
| king | 0.100 m | 0.0195 m |

Bodies are surfaces of revolution built from explicit profiles, the rook
merlons and the king cross are extruded prisms and the knight head is a
silhouette extruded along the width axis with a bevel that turns the hard
outline into a soft, carved edge. The knight faces the opponent: the model is
rotated so it looks along +Z in the exported Y-up glTF space.

## License

MIT, see `LICENSE` and the copy shipped next to the runtime assets.
