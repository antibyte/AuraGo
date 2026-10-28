# Ocean chat sea life kit

Eight Blender-generated animals for the Ocean chat theme: four reef fish (blue tang, clownfish,
butterflyfish, sardine), two sharks (blacktip reef shark, hammerhead), a green sea turtle and a humpback
whale. The runtime (`ui/js/chat/ocean-life-scene.js`) loads them next to the Tiefsee manta and moon jelly
and swims them behind the chat messages.

The kit is original geometry under the MIT license: no textures, no images, no external buffers. Colours,
markings and all motion live in the runtime shaders; the GLBs only carry the shapes and two UV sets that
tell the shaders where on the animal each vertex sits.

## Files

| Path | Role |
| --- | --- |
| `build_ocean_life.py` | Deterministic Blender generator (parametric surfaces, export, manifest, previews) |
| `check_assets.py` | Manifest, hash, budget and contract check for the runtime GLBs |
| `production/aurago-ocean-life.blend` | Editable source scene written by the generator (animals laid out in a row) |
| `../../ui/3d/chat-ocean-life/v1/` | Runtime payload: eight GLBs, `manifest.json`, `LICENSE.txt` |

## Run

```powershell
& 'D:/Blender 5.2/blender.exe' --background --factory-startup --python assets/chat-ocean-life/build_ocean_life.py
python assets/chat-ocean-life/check_assets.py
```

Options after `--`:

- `--preview <dir>` renders `<id>-side.png`, `<id>-quarter.png` and `<id>-front.png` per animal with EEVEE
  (flat grey preview colours so shapes read). Keep the directory outside the repository. The front view looks
  along -Z at the nose, so a spike or a flipper edge poking out beside the head shows up there.
- `--views side,quarter,top,front` chooses the preview views.
- `--only fish_tang,whale` exports just those animals and skips manifest, license and `.blend`, which
  makes iterating on one shape quick. A full run (no `--only`) is required before committing.

Re-running must reproduce byte-identical GLBs and `manifest.json`. The manifest pins the LF-normalised
generator hash, so `check_assets.py` fails after any generator edit until the generator has been rerun.

## Contract

Frame: glTF, +Y up, **+Z is the nose**, +X is the animal's left, origin near the centre of mass, metres.
The generator authors points in this frame and converts each vertex with `(x, -z, y)` before it enters
Blender; the exporter converts back exactly.

| Set | x | y |
| --- | --- | --- |
| `TEXCOORD_0` | `along`: 0 at the nose, 1 at the tail tip. On fins, flippers and flukes: 0 at the root, 1 at the tip | `part`: 0 body or shell, 0.25 fin, 0.5 eye, 0.75 flipper, 1 fluke (whale) or caudal fin (fish, sharks) |
| `TEXCOORD_1` | `height`: 0 belly, 1 back. On plates: chord or width coordinate 0..1 | `side`: 0 at the +X flank, 0.5 on the midline, 1 at the -X flank. A ramp across the body section (usable as a coordinate for gill slits or throat grooves); plates and eyes carry one value from the sign of their x |

Material names (the runtime assigns shaders by name): `body`, `fin`, `eye`, `shell`, `flipper`, `fluke`.
`fin`, `flipper` and `fluke` are thin double-sided surfaces: the fish and shark fins are flat plates, while the
whale flukes and pectorals and the turtle flippers are thin wedges (`Builder.foil`: a top and a bottom surface
that meet at the leading edge, the trailing edge and the tip) so they keep a visible profile edge-on. `body`,
`shell` and `eye` are closed surfaces with outward normals.

Every primitive carries `POSITION`, `NORMAL`, `TEXCOORD_0` and `TEXCOORD_1`.

| Asset id | File | Length (m) | Notes |
| --- | --- | --- | --- |
| `fish_tang` | `fish-tang.glb` | 0.30 | tall disc, lunate caudal |
| `fish_clown` | `fish-clown.glb` | 0.12 | stubby ovoid, round fins |
| `fish_butterfly` | `fish-butterfly.glb` | 0.20 | disc with a thin snout, spiny dorsal and anal |
| `fish_sardine` | `fish-sardine.glb` | 0.15 | slim, deeply forked caudal |
| `shark_reef` | `shark-reef.glb` | 1.8 | torpedo body, tall first dorsal, long upper caudal lobe |
| `shark_hammer` | `shark-hammer.glb` | 2.6 | flat cephalofoil with the eyes on the lobe ends |
| `turtle` | `turtle.glb` | 1.0 | `length` is the whole animal (head tip to tail tip); the shell is about 0.76 m |
| `whale` | `whale.glb` | 12.0 | scalloped pectorals, notched flukes, dorsal hump |

`length` in `manifest.json` is the nose-to-tail extent of the model; the runtime
scales each GLB so that its bounding-box depth equals the species length it is configured with.

## Budgets

Enforced by `check_assets.py`: kit total at most 1.5 MiB; fish at most 3 000 triangles each; sharks,
turtle and whale at most 8 000 each; no textures or images; manifest bytes, SHA-256 and triangle counts
match the files; the nose points along +Z; the z extent matches `length` within 3 % for every animal.

## Animation

Nothing is baked. The runtime vertex shaders animate from the UVs: lateral undulation scaled by `along`,
fin flutter on `part == 0.25`, flipper strokes on `part == 0.75`, fluke strokes on `part == 1`.
Because `along` restarts at each fin root, the caudal fin and the whale flukes do not continue the body's
`along`. A shader that must keep the tail attached to a bending body should derive its body coordinate
from `position.z` and the model's bounding box, or the generator can pass `root_along` to
`Builder.fan`/`Builder.plate` for those plates.

## Builder helpers

`Builder.loft` (closed body of revolution from elliptic stations, poles or flat caps at the ends),
`Builder.plate` (thin strip fin from a lower and an upper edge), `Builder.fan` (thin fin from a
star-shaped perimeter, used for caudal fins and triangular fins), `Builder.foil` (wedge-shaped wing with a
thickness profile: whale flukes and pectorals, turtle flippers), `Builder.sphere` (eyes) and
`Builder.tube` (turtle tail). Coincident vertices are welded, so poles and collapsed fin tips become
triangles instead of degenerate quads. Randomness is limited to one seeded `random.Random` per species
(only the whale's pectoral scallops use it).
