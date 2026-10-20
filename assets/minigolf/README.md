# AuraGo Mini Golf courses

The 18 original low-poly courses and the Blender generator in this directory
were created for AuraGo and are licensed under the MIT License in this
directory. They were authored in Blender 5.2.2 using metres. Runtime GLBs and
manifest coordinates use glTF's Y-up coordinate system.

Regenerate the prop kits, runtime GLBs, and the collision manifest from the
repository root with Blender 5.2.2 (`-- --only kits` or `-- --only courses`
rebuilds just one half and keeps the other half of the manifest):

```powershell
& 'D:\Blender 5.2\blender.exe' --background --factory-startup --python assets/minigolf/generate.py
node scripts/test-minigolf-assets.mjs
```

The generator writes one self-contained GLB per course, one prop kit
`props-<look>.glb` per world (`pirate`, `temple`, `moon`) plus
`ui/3d/minigolf/manifest.json`. The Blender scenes are no longer versioned;
`-- --save-blend` writes them to the ignored `assets/minigolf/build/`. Fixed
collider vertices and triangle indices are read back from each exported GLB, so
physics follows Blender's exported topology and coordinates exactly. Moving
obstacles are named top-level nodes and remain separate from fixed colliders;
their local compound box list follows each visible gate component, with the
top-level half-extents retained as a conservative runtime fallback.
The runtime manifest is compact JSON to reduce download size; pretty-print it
for inspection with `python -m json.tool ui/3d/minigolf/manifest.json`.

Current baseline: the 18 dressed course GLBs total 10,065,848 bytes (at most
754,904 bytes and 21,149 triangles per course, 336,816 triangles in all), the
compact manifest is 3,312,219 bytes, the three prop kits total 5,029,080 bytes
and the whole runtime package including audio is 21,320,234 bytes. The asset
test enforces the budgets (course at most 768 KiB, 24,000 triangles and 24 mesh
nodes; kit at most 3 MiB and 60,000 triangles; prop at most 15,000 triangles,
hero 20,000; runtime package at most 32 MiB), the P1 collider and physics
fingerprints, numbered-name-free GLBs, props inside the island rim, a
lane corridor free of props, embedded local resources, valid GLBs,
geometry/collider agreement, named obstacle nodes, and open cup collision
geometry. Regenerated sizes can vary with edits; the test is the authoritative
budget check.

All art is original and generated locally from this source. The generator has
no network or third-party art dependencies.

Sound effects and music are not part of this Blender pack. Their sources,
provenance and rebuild steps are in [`audio/README.md`](audio/README.md).
