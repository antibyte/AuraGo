# Tiefsee creature kit

Original Blender assets for the Tiefsee (deep sea) screensaver of the AuraGo
Virtual Desktop: a moon jelly, a lion's mane jelly, a comb jelly and a manta ray.

`build_abyss.py` builds every creature from parametric surfaces (lobed bells,
horseshoe gonads, ruffled oral arms, tapered tentacle tubes, meridional comb rows
and a swept manta planform), saves `production/aurago-abyss.blend` and exports one
self-contained GLB per creature plus a manifest into `ui/3d/screensaver/abyss/v1/`.

The runtime animates the creatures in its shaders. Two UV sets carry the data:

| Set | Meaning |
| --- | --- |
| `TEXCOORD_0` | `along` (0 at the attachment, 1 at the tip) and a stable per-strand `phase` |
| `TEXCOORD_1` | `angle` around the bell / span position and a part-specific `side` flag (belly for the manta) |

```powershell
& 'D:/Blender 5.2/blender.exe' --background --factory-startup --python assets/screensaver-abyss/build_abyss.py
python assets/screensaver-abyss/check_assets.py
node scripts/validate-screensaver-glbs.mjs
node scripts/build-screensaver-abyss.js
```

The kit contains no textures and stays well below its 1.5 MiB budget
(about 0.7 MiB, 2k–10k triangles per creature).
