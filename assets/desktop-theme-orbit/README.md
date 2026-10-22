# AuraGo Orbit model pack

Original Blender source for the 3D models behind the Orbit desktop theme: a ring
space station with a rotating habitat ring, a shuttle that docks and undocks, a
communications satellite and five cratered asteroid rocks. Everything is authored
from scratch for this repository and ships under the MIT license; no third-party
model, texture or scan is used.

## Contents

| File | Purpose |
| --- | --- |
| `generate.py` | Blender 5.2 LTS script that builds and exports the whole pack |
| `orbit-models.blend` | Editable source scene produced by the script (models laid out side by side) |
| `AGENTS.md` | Local contract for this pack |

## Regenerating

```bash
blender --background --factory-startup --python assets/desktop-theme-orbit/generate.py
```

The script writes `ui/3d/desktop-theme-orbit/v1/{station,shuttle,satellite,asteroids}.glb`,
`manifest.json` (SHA-256, bytes and triangle count per file) and `LICENSE.txt`, then
saves the source scene. Afterwards run:

```bash
node scripts/test-desktop-theme-orbit-assets.mjs
```

Add `-- --preview <dir>` to render EEVEE previews (1024 x 768, three-point lighting on a
dark background) of each model. Point it at a scratch directory outside the repository.
`-- --only station,shuttle` rebuilds just those GLBs without touching the manifest.

The build is deterministic: the same Blender version produces byte-identical GLBs.

## Models

| File | Bytes | Triangles | Notes |
| --- | --- | --- | --- |
| `station.glb` | about 930 KB | about 25,100 | spin axis Y, ring centreline radius 1.0 |
| `shuttle.glb` | about 145 KB | about 3,800 | 0.175 long, forward +Z, origin at the nose docking probe |
| `satellite.glb` | about 171 KB | about 3,800 | bus 0.12, solar wings span 0.72 |
| `asteroids.glb` | about 120 KB | 5 x 752 | vertex-coloured rocks, unit size |

The whole pack stays below 1.5 MiB. Exact numbers are in `manifest.json`.

### Station

A 96 x 18 habitat ring (rounded-square cross-section, 32 modules separated by dark
ribs) turns about the Y axis as the `RingRotor` node: it carries the ring hull, the
inner dark band, six spokes with hub collar and elevator carriages, three hab pods,
64 lit window panes on the outer equator (merged into one `WindowsEmissive` mesh) and
cyan running lights. The static part is a stepped spine (-0.85 .. +0.85) with rib
tori, four propellant tanks, two crinkled gold-foil service modules, louvre radiators,
two truss solar wings along +-X at y = 0.55 (four inset-cell panels each), a parabolic
dish with feed horn on top and a docking module at the bottom with eight cyan dock
lights. `DockPort` marks the docking face (docking axis -Y), `BeaconRed` sits at the
+X wing tip and `BeaconGreen` at the -X tip.

### Shuttle

A lofted lifting-body fuselage (8 cross-sections, smoothly interpolated), delta wings
with 4 degrees of dihedral and canted winglets, a vertical fin with rudder, a glass
canopy, a white upper hull with a tiled dark belly, an engine block with three bell
nozzles and three `EngineEmissive` disks (`EngineGlow`). `Exhaust` is an empty 0.02
behind the nozzle mouths. Cyan lights ring the nose docking probe; red (+X) and green
(-X) navigation lights sit on the winglets.

### Satellite

A crinkled gold-foil bus with a radiator plate, instrument deck, star trackers, apogee
motor, two three-panel solar wings, whip antennas and a sensor boom. The `Dish`
node is an empty at the gimbal with the reflector and feed as children; its boresight
is +Z at rest, so the runtime can aim it by rotating that one node.

### Asteroids

Five rocks (`Rock0` .. `Rock4`), each a different potato shape (fractal and creased
noise, 2-4 craters with floor, wall and rim, planar fracture cuts, non-uniform scale
0.8 - 1.3 per axis) at 752 triangles with vertex colours from grey-brown (#6e655c)
to dark (#3b3631) driven by cavity and noise. The nodes sit at x = 0, 3, 6, 9, 12;
the mesh data of every rock is centred on its own origin.

## Runtime notes

- Metals: `GoldFoil` is fully metallic and `HullDark` 0.6 metallic, as specified. They
  only look right with an environment or strong specular light in the renderer.
- Emissive materials carry colour in `emissiveFactor` and strength in
  `KHR_materials_emissive_strength`; the runtime may scale `emissiveIntensity` for bloom.

## License

MIT, see `ui/3d/desktop-theme-orbit/v1/LICENSE.txt`.
