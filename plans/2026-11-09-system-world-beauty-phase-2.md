
# System World Beautification — Phase 2 Implementation Plan (windows with life, reworked buildings)

> **For agentic workers.** Subagents carry out this plan one task at a time, in the order given under "Order and parallelism". A code-writing agent (Sonnet class) implements each task and a test-running agent runs the commands listed under it. One combined review per batch, Critical and Important findings only (lean workflow).
>
> - **Commit after every task** with a conventional message (`feat(system-world): …`, `test(…)`, `chore(…)`, `docs(…)`). The last line is `Co-Authored-By: Claude <noreply@anthropic.com>`.
> - **CLAUDE.md rules.** Run GitNexus `impact` (upstream) on every function you edit, and `detect_changes` before every commit. Use the MCP tools, or the CLI from the main checkout root. The index does not cover the worktree. `createCity`, `createSurfaces` and `createAtmosphere` are expected to come back HIGH or UNKNOWN. Report the result and continue: this plan is the mitigation, and its tests cover the callers.
> - **Scripts.** Write throwaway scripts with the Write tool into your session scratch directory, never into the repo. Bash heredocs mangle backslashes on this machine.
> - **Paths.** All paths are relative to the worktree `C:\Users\Andi\Documents\repo\AuraGo\.worktrees\sysworld-beauty`. Never Glob or Grep the main repo root.
> - **Anchors, not line numbers.** Phase 1 is still landing in the files this plan touches. Every edit names a function and quotes the anchor text. If phase 1 changed the surrounding text, apply the same change to the equivalent statement and say so in the commit message.

**Goal**
- Kit v3 replaces the seven district buildings, both data towers, `skybridge`, `street-lamp` and `planter` everywhere they are placed.
- Window panes (`city.window`) show interior-mapped rooms behind real glossy glass: lit rooms by time of day, furniture, robots and screens, warm lobbies, and a procedural frame.
- The twelve superseded v1 designs leave the runtime payload.
- Street-lamp light cones disappear by day.

**Architecture**
- **New pure-render module `ui/js/desktop/apps/sysworld-windows.js`.** It owns ONE shared patched `MeshStandardMaterial` for every `city.window` mesh: static, instanced and every LOD.
  - The fragment injection ray-casts a box room per UV cell.
  - Tangent frame and metres per cell come from screen-space derivatives, with an analytic fallback.
  - Quality levels: 0 = flat cells, 1 = rooms with parallax, 2 = rooms with life.
- **Routing in `sysworld-scene.js` `model()`.** Every loaded mesh passes `windows.adopt(n)` before material sharing, so all paths (instanced `staticCity`, district clones, LOD rebuilds, skyline LOD 2) get the shared material.
- **v3 precedence.** `kitFor(id, catalog3)` picks v3 when the v3 manifest ships the id. Colliders use `kitCatalog(v1, v3)`.
- **Surfaces.** `sysworld-surfaces.js` never patches `city.window`. `city.glass` gains a surface kind. Vertex-coloured (v3) `city.ivory` and `city.warm` parts are light fixtures, not switching panes. v1 meshes keep the existing `WINDOWS`/`windowSeeds` pane lighting unchanged.

**Tech Stack**
- three 0.186.1 (`onBeforeCompile` injection, WebGL2/GLSL3), `GLTFLoader` and `MeshoptDecoder`.
- gltfpack 1.3.0 (`-cc -kn -km -ke -kv -vtf`).
- Node ESM tests with `node:assert`, Python `check_assets.py`, Go + rod browser tests.
- The survey harness `scripts/system-world-survey.mjs`.

---

## What I verified, and what I could not

**Verified by reading (worktree at `cc27b886f` plus uncommitted phase 1 work)**
- **`city.window` today.** It is in neither `KINDS` nor `WINDOWS`, so `surfaces.apply` already skips it. But `test-system-world-surfaces.mjs` asserts every kit material is mapped (`KINDS | WINDOWS | untextured`), so registering the payload turns it red.
- **Glass assertion.** The surfaces test asserts `city.glass` stays untouched ("Glass must stay untouched"). Decision 4 needs that assertion changed.
- **`windowSeeds` on v3 geometry.** It gives a random seed to every connected component of up to 6 vertices. Exported v3 boxes split per face at sharp edges, so v3 light strips would switch per face. Task 1 fixes this.
- **three 0.186.1 shader inputs.**
  - The vertex prefix always declares `attribute vec2 uv;` (`WebGLProgram.js` line 627).
  - The fragment prefix declares `uniform vec3 cameraPosition;`.
  - `ShaderLib.standard` contains `#include <common>`, `<begin_vertex>`, `<color_fragment>`, `<roughnessmap_fragment>` and `<emissivemap_fragment>` exactly once.
- **Authoring geometry.**
  - Pane winding in `facade_assets3.pane` gives outward normals.
  - `u` runs with the face direction (left to right seen from outside).
  - Fronts face Blender −Y, which is glTF +Z.
  - Room sizes: bays 1.4–4.2 m (beacon segments 1.40 m, atrium lobby segments ≈4.2 m), storeys 1.6–4.0 m (crown pavilion 1.6 m, tower lobby 4.0 m).
  - Rows reach 0..13 per tier, plus 20·k, 40 (crown pavilion) and 60 (integration bridge). `u` reaches ≈ 3·16 + bays.
- **Colliders.** `installCityColliders(traffic, catalog, …)` reads `catalog.get(id).lods[0].bounds` from the v1 catalog for districts, lamps, planters, towers and the skybridge. The layout, hover and living tests do the same from `v1/manifest.json`.
- **v1 byte figures** (from `v1/manifest.json`).
  - The twelve superseded designs total **3,998,504 bytes** (3.81 MiB).
  - After phase 1 has removed the streets, v1 keeps `service-drone`, `data-tram` and `server-rack`, total **280,116 bytes**.
  - LOD 0 of the twelve plus LOD 2 of the two towers is 2,259,912 bytes. That is the v1 share of today's first display that v3 replaces.
- **Time-of-day values** (`daylight`): day = (amount 1, evening 0), evening = (0, 1), night = (0, 0).
- **Lamp cones** (`createAtmosphere`): constant strength 0.3 regardless of daylight; `setTier` only hides them on low.
- **Diagnostics.** `SysWorldApp.inspect()` spreads `city.stats()`, so new stats fields reach the browser test.

**Could not verify**
1. **gltfpack and UVs on texture-free materials.** Settled by the main session on 2026-11-10 with `data-tower-a.lod0`: quantised texture coordinates (`-vt 14`) are normalised to 0..1 and lose the room coordinates, because the dequantisation is only written into `KHR_texture_transform` on texture slots.
2. **`-vtf`.** Settled as well: gltfpack 1.3.0 accepts `-vtf` and writes float TEXCOORD_0 with the room coordinates intact, at practically the same file size. The plan uses `-vtf` throughout; Task 1's decode check still guards it.
3. **Exporter V flip.** That the Blender 5.2 glTF exporter writes V as `1 − v` (standard exporter behaviour). The decode check fails loudly if not, with the fix described there.
4. **v3 bounds** against v1 bounds. Task 3 measures them.
5. **GPU compile** and the visual result. No GPU here; the survey and browser runs prove them.
6. **`createAtmosphere` in Node.** Whether it constructs (it runs `createCinema` → `ShaderPass`). Task 5 has a fallback.
7. **Real `loadedBytes`** and frame cost after the swap.

## Deviations from the brief

- **Low tier freezes switching.** On low, rooms do not re-roll their light over time (`winEpoch = 0`), to honour "low = no animation". The lit share still follows the time of day.
- **Sky sheen.** Glass reflects `scene.environment` (the RoomEnvironment PMREM). To make it "still reflect the sky", the material also adds a cheap Fresnel sheen of the current fog/horizon colour (`winSky`), passed in `setLighting`. A true sky-matched environment map is a phase 6 candidate.
- **Fixture seeds.** Vertex-coloured `city.ivory`/`city.warm` get the fixture seed (follows day/night brightness, never switches per face). The brief did not mention this; without it the v3 light strips flicker per face.

---

## File map

| File | Status | Responsibility |
| --- | --- | --- |
| `ui/js/desktop/apps/sysworld-windows.js` | new | `WINDOW_MATERIAL`, `WINDOW_SHADER`, `windowLighting`, `windowLevel`, `createWindows()` (shared material, `adopt`, `owns`, `setLighting`, `setTier`, `setReducedMotion`, `update`, `stats`, `dispose`) |
| `scripts/test-system-world-windows.mjs` | new | Lit share, levels, material flags, injection hooks, uniform plumbing, NaN guards, routing on real v3 GLBs |
| `scripts/test-system-world-lamps.mjs` | new | Lamp cones off by day, fade in towards dusk, off on low |
| `ui/js/desktop/apps/sysworld-surfaces.js` | edit | `city.glass` kind; `city.window` exemption; fixture seeds for vertex-coloured light materials |
| `ui/js/desktop/apps/sysworld-scene.js` | edit | Import and create windows; `adopt` in `model()`; `setTier`/`setLighting`/`update`/reduced motion/`dispose`/`stats`; `supersededV1`, `kitFor`, v3 precedence in `rebuild()`; `kitCatalog` colliders; `kitModels` stat |
| `ui/js/desktop/apps/sysworld-colliders.js` | edit | `kitCatalog(...catalogs)` |
| `ui/js/desktop/apps/sysworld-atmosphere.js` | edit | `lampLevel` from daylight; cone visibility and strength |
| `scripts/build-system-world-v3.mjs` | edit | `PACK_FLAGS` += `-vtf` |
| `scripts/test-system-world-v3.mjs` | edit | Pane UV decode check (Task 1); design pins, precedence and v1 retirement pins (Tasks 3–4) |
| `scripts/test-system-world-surfaces.mjs` | edit | Window exemption, glass kind, fixture seeds |
| `scripts/test-system-world-{layout,hover,living}.mjs` | edit | `kitCatalog(v1, v3)` for colliders |
| `scripts/system-world-survey.mjs` | edit | Expose `windows` |
| `scripts/system-world-survey.scenes.json` | edit | Facade, lobby, skyline and overview scenes (Task 1); proof scenes (Task 2) |
| `ui/3d/system-world/v1/manifest.json` + 36 GLBs | edit/delete | Retire 12 designs (Task 4) |
| `assets/system-world/check_assets.py` | edit | v1 count 15 → 3 |
| `ui/desktop_sysworld_test.go` | edit | Markers for windows, scene and atmosphere |
| `ui/desktop_sysworld_city_browser_test.go` | edit | Window interiors active on high, flat on low |
| `assets/system-world/AGENTS.md`, `README.md`, `ui/3d/system-world/AGENTS.md`, `plans/2026-11-09-system-world-beauty-design.md` | edit | Docs |
| `ui/js/vendor/system-world/{city.esm.js,manifest.json}` | regenerate | Task 7 |
| `assets/system-world/build_world3.py`, `ui/3d/system-world/v3/*` | main session | Task 0 |

## Invariants every task must keep

- **Survey patch anchors.** `sysworld-scene.js` keeps the exact text `  return {\n    districts, canvas, update, focus(id)`, and `sysworld-society.js` keeps `    add,patrol,update,suspend,\n`. Add new return members on their own lines.
- **Go markers.** Keep every marker string in `TestDesktopSysWorldAppMarkers` (for example `surfaces.prepare(n)`, `InstancedMesh`, `export function windowSeeds`, `#define SW_LEVEL `, `setTier(`).
- **Window material ownership.** `city.window` never enters the shared `materials` map, never gets `surfaces.apply` patches, and is disposed only by `windows.dispose()`.
- **Window GLSL safety.** No `normalize(` anywhere in it. Every `pow(` base starts with `clamp(`. Every division is by a guarded quantity (the test enforces all three).
- **No placement of new furniture.** The seven new v3 furniture ids (`tree-round`, `tree-column`, `street-bench`, `bollard`, `litter-bin`, `zebra-sign`, `kiosk`) are not placed in phase 2.
- **Bundle staleness.** `node scripts/build-system-world.js --check` is red from Task 1 until Task 7 rebuilds the bundle (as in phase 1). No Go test checks it.

## Order and parallelism

| Step | Task | Notes |
| --- | --- | --- |
| A | 1 | Runtime prep. Captures the before-survey first. **Precondition:** phase 1 Task 15 committed (`check_assets.py` says 15 designs; v3 street modules placed). |
| B | **0** | Main session delivers the payload. Blocked on Task 1. |
| C | 2 | Window module and proof. Blocked on Task 0. |
| D | 3 | Swap. After 2. |
| E | 4 | Retire v1. After 3. |
| — | 5 | Lamp cones. **Parallel** with any task (disjoint files: `sysworld-atmosphere.js` plus a new test). |
| F | 6 | Docs. After 4; may run parallel with 5. |
| G | 7 | Markers, bundle, regression, survey, budgets. Last. |

---

## Task 1 — Runtime prep: surfaces and tests know `city.window` and `city.glass`, `-vtf`, pane UV check (no payload)

**Files**
- `scripts/system-world-survey.scenes.json`
- `ui/js/desktop/apps/sysworld-surfaces.js` (`KINDS`, `apply`, `windowSeeds`, `prepare`)
- `scripts/test-system-world-surfaces.mjs`
- `scripts/build-system-world-v3.mjs` (`PACK_FLAGS`, header comment)
- `scripts/test-system-world-v3.mjs`

**Steps**

- [ ] Append these scenes to the JSON array in `scripts/system-world-survey.scenes.json`, one per line, same style as the existing ones.
  - Eye height is 1.7 m on the flush pavement.
  - Facade cameras stand on the far pavement of the street in front of each building, where there are no lamps or planters.
  ```json
  {"name":"facade-agent-day","position":[6,1.7,17.5],"target":[0,12,-12],"fov":43,"environment":"day","weather":"clear","frames":40,"waitMs":400},
  {"name":"facade-agent-evening","position":[6,1.7,17.5],"target":[0,12,-12],"fov":43,"environment":"evening","weather":"clear","frames":40,"waitMs":400},
  {"name":"facade-agent-night","position":[6,1.7,17.5],"target":[0,12,-12],"fov":43,"environment":"night","weather":"clear","frames":40,"waitMs":400},
  {"name":"facade-memory-day","position":[-37,1.7,17.5],"target":[-43,8,-10],"fov":43,"environment":"day","weather":"clear","frames":40,"waitMs":400},
  {"name":"facade-memory-evening","position":[-37,1.7,17.5],"target":[-43,8,-10],"fov":43,"environment":"evening","weather":"clear","frames":40,"waitMs":400},
  {"name":"facade-memory-night","position":[-37,1.7,17.5],"target":[-43,8,-10],"fov":43,"environment":"night","weather":"clear","frames":40,"waitMs":400},
  {"name":"facade-missions-day","position":[50,1.7,63.5],"target":[43,7,36],"fov":43,"environment":"day","weather":"clear","frames":40,"waitMs":400},
  {"name":"facade-missions-evening","position":[50,1.7,63.5],"target":[43,7,36],"fov":43,"environment":"evening","weather":"clear","frames":40,"waitMs":400},
  {"name":"facade-missions-night","position":[50,1.7,63.5],"target":[43,7,36],"fov":43,"environment":"night","weather":"clear","frames":40,"waitMs":400},
  {"name":"facade-integration-night","position":[37,1.7,17.5],"target":[43,11,-10],"fov":43,"environment":"night","weather":"clear","frames":40,"waitMs":400},
  {"name":"facade-atrium-night","position":[-36,1.7,63.5],"target":[-43,5,36],"fov":43,"environment":"night","weather":"clear","frames":40,"waitMs":400},
  {"name":"facade-beacon-night","position":[5,1.7,63.5],"target":[0,7,36],"fov":43,"environment":"night","weather":"clear","frames":40,"waitMs":400},
  {"name":"facade-foundry-night","position":[-37,1.7,-27.5],"target":[-43,6,-53],"fov":43,"environment":"night","weather":"clear","frames":40,"waitMs":400},
  {"name":"facade-tower-night","position":[36,1.7,-27.5],"target":[30.5,16,-55],"fov":43,"environment":"night","weather":"clear","frames":40,"waitMs":400},
  {"name":"lobby-agent-night","position":[2.5,2.0,3.0],"target":[0,3.0,-4.9],"fov":43,"environment":"night","weather":"clear","frames":40,"waitMs":400},
  {"name":"skyline-night","position":[10,45,-75],"target":[0,35,-220],"fov":43,"environment":"night","weather":"clear","frames":40,"waitMs":400},
  {"name":"overview-night","position":[95,70,120],"target":[0,10,-10],"fov":43,"environment":"night","weather":"clear","frames":40,"waitMs":400}
  ```
- [ ] **Before any code change**, capture the before-survey:
  - `node scripts/system-world-survey.mjs --out reports/system-world-survey/phase-2-before --tier high --only facade-agent-day,facade-agent-evening,facade-agent-night,facade-memory-day,facade-memory-evening,facade-memory-night,facade-missions-day,facade-missions-evening,facade-missions-night,facade-integration-night,facade-atrium-night,facade-beacon-night,facade-foundry-night,facade-tower-night,lobby-agent-night,skyline-night,overview-night`
  - Expected: 17 PNGs and `result: done … page errors: 0`.
  - In `report.json`, every shot should have `cameraMovedByCollision: 0`. If one is non-zero, move that camera 1 m along the pavement and recapture.
  - Do not commit pictures (`git check-ignore reports/system-world-survey/phase-2-before` prints the path).
- [ ] **Test first: `scripts/test-system-world-surfaces.mjs`.**
  - Replace
    `const untextured = new Set(['city.glass', 'city.garden-glass', 'city.cyan', 'city.jade', 'city.lantern-glow']);`
    with
    ```js
    const untextured = new Set(['city.garden-glass', 'city.cyan', 'city.jade', 'city.lantern-glow']);
    // Kit v3 window panes are interior-mapped by sysworld-windows.js, never by the surface maps.
    const windowPanes = new Set(['city.window']);
    ```
  - Change the loop assertion to `assert.ok(KINDS[name] || WINDOWS.has(name) || untextured.has(name) || windowPanes.has(name), 'Unmapped kit material: ' + name);`.
  - Replace the three lines starting at `const glass = new THREE.MeshStandardMaterial({name: 'city.glass'});` (through the `'Glass must stay untouched'` assertion) with:
    ```js
    // Kit glass (doors, small panes) takes the surface maps; window panes never do.
    const glass = compile('city.glass', 'world', 'high');
    assert.ok(glass.text.includes('diffuseColor.rgb*=swAlbedoF'), 'Kit glass takes the surface maps');
    const pane = new THREE.MeshStandardMaterial({name: 'city.window', vertexColors: true});
    surfaces.apply(pane);
    assert.equal(pane.onBeforeCompile, THREE.Material.prototype.onBeforeCompile, 'city.window belongs to sysworld-windows.js');
    // Vertex-coloured (kit v3) warm/ivory parts are light fixtures: one seed 2, never per-face switching.
    {
      const fixture = new THREE.BufferGeometry();
      fixture.setAttribute('position', new THREE.Float32BufferAttribute(vertices.flat(), 3));
      fixture.setIndex(indices);
      surfaces.prepare(new THREE.Mesh(fixture, new THREE.MeshStandardMaterial({name: 'city.ivory', vertexColors: true})));
      assert.ok(fixture.getAttribute('swSeed').array.every(s => s === 2), 'Kit v3 light parts must be fixtures');
    }
    ```
  - Run `node scripts/test-system-world-surfaces.mjs`. Expected: **FAIL** (`Kit glass takes the surface maps`).
- [ ] **`sysworld-surfaces.js`.**
  - In `KINDS`, after the `'city.tactile'` entry, add:
    ```js
      // Kit glass (doors and small panes, opaque by design): faint panel detail, streaks when wet.
      'city.glass': {top: 'panels', side: 'panels', albedo: .15, rough: .3, bump: .002, wet: .6},
    ```
  - Below `export const WINDOWS = …` add:
    ```js
    // Kit v3 window panes belong to sysworld-windows.js and never take the projected maps.
    const PANES = 'city.window';
    ```
  - In `apply`, change `if (!material?.isMeshStandardMaterial || patched.has(material)) return;` to `if (!material?.isMeshStandardMaterial || patched.has(material) || material.name === PANES) return;`.
  - In `windowSeeds`, change the signature to `export function windowSeeds(geometry, fixture = false) {`. Directly after `if (!position || geometry.getAttribute('swSeed')) return;` add:
    ```js
      // Kit v3 has no lit-window quads: its warm/ivory parts (light lines, diffusers) are fixtures
      // that only follow the day/night brightness, so every vertex gets the fixture seed.
      if (fixture) { geometry.setAttribute('swSeed', new THREE.BufferAttribute(new Float32Array(position.count).fill(2), 1)); return; }
    ```
  - In `prepare`, change `if (WINDOWS.has(material?.name)) windowSeeds(mesh.geometry);` to `if (WINDOWS.has(material?.name)) windowSeeds(mesh.geometry, !!material.vertexColors);`.
  - v1 materials have no vertex colours, so the remaining v1 models keep per-pane seeds unchanged.
- [ ] **`scripts/build-system-world-v3.mjs`.**
  - Change `const PACK_FLAGS = ['-cc', '-kn', '-km', '-ke', '-kv'];` to `const PACK_FLAGS = ['-cc', '-kn', '-km', '-ke', '-kv', '-vtf'];`.
  - Add to the header comment: ` * -vtf keeps window room coordinates (TEXCOORD_0, up to about 70) as floats; quantised texture`
    ` * coordinates would be normalised to 0..1 because the kit has no textures to carry the transform.`
  - The street payload was packed without `-vtf`, so `--check` reports a repack difference until Task 0 repacks everything. That is expected.
- [ ] **`scripts/test-system-world-v3.mjs`.**
  - Add imports at the top:
    ```js
    import * as THREE from 'three';
    import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';
    import { MeshoptDecoder } from 'three/addons/libs/meshopt_decoder.module.js';
    ```
  - Insert before the final `console.log(…)`:
    ```js
    // Window panes (city.window): TEXCOORD_0 holds room coordinates that must survive packing.
    // After undoing glTF's flipped V, u advances one unit per bay (0.8-6 m) left to right seen from
    // outside, v one unit per storey (1.2-6 m) upwards, and every pane corner sits on the room grid.
    await MeshoptDecoder.ready;
    const loader = new GLTFLoader().setMeshoptDecoder(MeshoptDecoder);
    const windowDesigns = new Set();
    let paneTriangles = 0;
    const a = new THREE.Vector3(), b = new THREE.Vector3(), c = new THREE.Vector3(), n = new THREE.Vector3();
    for (const asset of manifest.assets) for (const lod of asset.lods) {
      const data = await fs.readFile(path.join(base, lod.file));
      const doc = JSON.parse(data.subarray(20, 20 + data.readUInt32LE(12)).toString('utf8'));
      if (!(doc.materials || []).some(m => m.name === 'city.window')) continue;
      windowDesigns.add(asset.id);
      const gltf = await loader.parseAsync(data.buffer.slice(data.byteOffset, data.byteOffset + data.byteLength), '');
      gltf.scene.updateMatrixWorld(true);
      gltf.scene.traverse(mesh => {
        if (!mesh.isMesh || mesh.material.name !== 'city.window') return;
        const g = mesh.geometry, pos = g.getAttribute('position'), uv = g.getAttribute('uv'), nor = g.getAttribute('normal'), index = g.getIndex();
        assert.ok(uv && nor && g.getAttribute('color'), lod.file + ': window panes need TEXCOORD_0, NORMAL and COLOR_0');
        const normalMatrix = new THREE.Matrix3().getNormalMatrix(mesh.matrixWorld);
        const room = i => [uv.getX(i), 1 - uv.getY(i)];
        for (let i = 0; i < uv.count; i++) for (const value of room(i))
          assert.ok(Math.abs(value - Math.round(value)) < .01, lod.file + ': pane corner off the room grid ' + room(i));
        let checked = 0;
        const count = index ? index.count : pos.count;
        for (let t = 0; t < count; t += 3) {
          const [i0, i1, i2] = [t, t + 1, t + 2].map(k => index ? index.getX(k) : k);
          a.fromBufferAttribute(pos, i0).applyMatrix4(mesh.matrixWorld);
          b.fromBufferAttribute(pos, i1).applyMatrix4(mesh.matrixWorld);
          c.fromBufferAttribute(pos, i2).applyMatrix4(mesh.matrixWorld);
          const [r0, r1, r2] = [room(i0), room(i1), room(i2)];
          const d1 = [r1[0] - r0[0], r1[1] - r0[1]], d2 = [r2[0] - r0[0], r2[1] - r0[1]], det = d1[0] * d2[1] - d1[1] * d2[0];
          if (Math.abs(det) < 1e-6) continue;
          const e1 = b.clone().sub(a), e2 = c.clone().sub(a);
          const pu = e1.clone().multiplyScalar(d2[1]).addScaledVector(e2, -d1[1]).divideScalar(det);
          const pv = e2.clone().multiplyScalar(d1[0]).addScaledVector(e1, -d2[0]).divideScalar(det);
          n.fromBufferAttribute(nor, i0).applyMatrix3(normalMatrix).normalize();
          const where = lod.file + ' pane triangle ' + t / 3;
          assert.ok(pu.length() > .8 && pu.length() < 6, where + ': bay width ' + pu.length().toFixed(2) + ' m');
          assert.ok(pv.length() > 1.2 && pv.length() < 6, where + ': storey height ' + pv.length().toFixed(2) + ' m');
          assert.ok(Math.abs(pu.y) < .05 * pu.length() && pv.y > .98 * pv.length(), where + ': rooms must run along the facade and upwards');
          assert.ok(pu.clone().cross(pv).dot(n) > 0, where + ': u must run left to right seen from outside');
          checked++; paneTriangles++;
        }
        assert.ok(checked > 0, lod.file + ': no pane triangle carries room coordinates');
      });
    }
    if (windowDesigns.size) assert.match(manifest.pack ?? '', /-vtf\b/, 'Room coordinates need float texture coordinates');
    console.log('System World v3 window panes: ' + (windowDesigns.size ? windowDesigns.size + ' designs, ' + paneTriangles + ' pane triangles verified' : 'none shipped yet'));
    ```
- [ ] Run, each must pass:
  - `node scripts/test-system-world-surfaces.mjs` → PASS (15 kinds).
  - `node scripts/test-system-world-v3.mjs` → PASS, printing `window panes: none shipped yet`.
  - `node --check scripts/build-system-world-v3.mjs` → no output.
  - `node scripts/build-system-world-v3.mjs --check` → `verified`.
    - Without `disposable/system-world/v3-raw` the repack is skipped.
    - If the raw directory exists and the repack differs **only** because of the new flag, report it and continue. Task 0 repacks.
  - `go test ./ui -run '^TestDesktopSysWorld' -count=1` → PASS.
- [ ] Commit: `feat(system-world): surfaces and kit tests ready for kit v3 buildings (window panes, glass kind, fixtures, float UVs)`.

**Acceptance**
- `city.window` is exempt from the surface maps, and `city.glass` has a kind.
- v3 light parts are fixtures; v1 pane seeds are unchanged.
- The pack flags carry `-vtf`.
- The pane UV check exists and is dormant.
- The before-survey exists.

---

## Task 0 — Main session delivers the payload (no agent work)

**What the main session does**
1. In `assets/system-world/build_world3.py`, set `FAMILIES = [street_assets3, facade_assets3, district_assets3, furniture_assets3]` (import the three modules). `SOURCES` follows automatically.
2. `& 'D:/Blender 5.2/blender.exe' --background --factory-startup --python assets/system-world/build_world3.py`.
3. `node scripts/build-system-world-v3.mjs`, then `node scripts/build-system-world-v3.mjs --check` (packs with Task 1's `-vtf`).
4. Commit `assets/system-world/build_world3.py` and `ui/3d/system-world/v3/*`: `feat(system-world): kit v3 district buildings, data towers and street furniture (payload)`.

**What must already hold at that moment** (all provided by Task 1 or untouched)
- **Placements.** `catalog3` lists 25 designs, but only the street ids are placed (`kitPlacements`). No runtime code references the new ids, so the city looks exactly as before.
- **Materials.** The new GLB materials (`city.window`, `city.glass`, `city.cyan`, `city.jade`, `city.leaf`, `city.ivory`, `city.warm`, `city.ceramic`, `city.titanium`, `city.bronze`, `city.graphite`, `city.stone`, `city.iron`, `city.road`) are all accepted by the surfaces test.
- **Pane UV check.** The check in `test-system-world-v3.mjs` becomes active.
- **v1 untouched.** `check_assets.py` still says 15 designs.
- **Budget.** The model directory stays under 40 MiB with v1 still present.

**Acceptance commands** (all must pass)
- `node scripts/build-system-world-v3.mjs --check` → 25 designs, 75 files.
- `node scripts/test-system-world-v3.mjs`. The window line must report 10 designs:
  - 7 districts + 2 towers + `skybridge`;
  - `street-lamp`, `planter` and the furniture have no panes.
- `node scripts/test-system-world-surfaces.mjs`
- `node scripts/test-system-world-street-models.mjs` (phase 1)
- `node scripts/test-system-world-expansion.mjs`
- `python assets/system-world/check_assets.py`
- The manifest `pack` is `gltfpack@1.3.0 -cc -kn -km -ke -kv -vtf`.

**If the pane UV check fails**
- **Corners off the grid, or bay and storey lengths absurd.** gltfpack normalised the texture coordinates although `-vtf` is set (verified on 2026-11-10 that 1.3.0 keeps float room coordinates with `-vtf`). Check that `PACK_FLAGS` really carries `-vtf`; as a last resort use `'-noq'` and report the size growth.
- **`pv.y` negative everywhere.** The exporter did not flip V. Tell the plan executor to change `1.0-uv.y` to `uv.y` in `WINDOW_SHADER.vertexMain` (Task 2), and `1 - uv.getY(i)` to `uv.getY(i)` in the check.

---

## Task 2 — `sysworld-windows.js`, its test, routing in `model()`, survey proof (nothing placed yet)

**Files**
- New `ui/js/desktop/apps/sysworld-windows.js`
- New `scripts/test-system-world-windows.mjs`
- `ui/js/desktop/apps/sysworld-scene.js`: imports, `createCity`, `model()`, `applyTier()`, `update()`, `dispose()`, return object (`setReducedMotion`, `stats`)
- `scripts/test-system-world-surfaces.mjs`
- `scripts/system-world-survey.mjs` (`EXPOSE` and header)
- `scripts/system-world-survey.scenes.json`

**Steps**

- [ ] Write the test below. Run `node scripts/test-system-world-windows.mjs` → FAIL (module not found).
- [ ] Write the module below. Run again → PASS.
- [ ] In `scripts/test-system-world-surfaces.mjs`, add `import {WINDOW_MATERIAL} from '../ui/js/desktop/apps/sysworld-windows.js';` and change `const windowPanes = new Set(['city.window']);` to `const windowPanes = new Set([WINDOW_MATERIAL]);`.
- [ ] **`sysworld-scene.js` integration** (exact anchors):
  1. After `import { createSurfaces } from './sysworld-surfaces.js';` add `import { createWindows } from './sysworld-windows.js';`.
  2. Directly after the statement `const surfaces = createSurfaces(renderer, { … });` (it ends with `});`) add:
     ```js
       // Kit v3 window panes share one interior-mapped glass material (sysworld-windows.js).
       const windows = createWindows();
     ```
  3. In `model()`, inside `gltf.scene.traverse(n => {`:

     Before:
     ```js
               geoSet.add(n.geometry); n.castShadow = true; n.receiveShadow = true;
               const source = Array.isArray(n.material) ? n.material : [n.material];
               const shared = source.map(m => {
                 const name = sharedMaterialKey(m);
     ```
     After:
     ```js
               geoSet.add(n.geometry); n.castShadow = true; n.receiveShadow = true;
               // Window panes (city.window) take the shared interior material on every path that loads
               // a model (instanced city, district clones, LOD rebuilds, skyline). It never enters the
               // shared-material map and sysworld-surfaces.js leaves it alone.
               windows.adopt(n);
               const source = Array.isArray(n.material) ? n.material : [n.material];
               const shared = source.map(m => {
                 if (windows.owns(m)) return m;
                 const name = sharedMaterialKey(m);
     ```
     The following `surfaces.prepare(n);` stays. It skips the window material.
  4. In `applyTier()`, change `surfaces.setTier(tier);` to `surfaces.setTier(tier); windows.setTier(tier);`.
  5. In `update()`:

     Before:
     ```js
         const mood = weather.mood();
         flow.update(dt,!reduced,1-(mood.day||0));
     ```
     After:
     ```js
         const mood = weather.mood();
         flow.update(dt,!reduced,1-(mood.day||0));
         // Window interiors: lit share, lobbies and glass sheen follow the time of day; no motion freezes them.
         windows.setLighting(mood.day, mood.evening, mood.weather === 'clear' ? 0 : 1, scene.fog.color); windows.update(dt, !reduced);
     ```
  6. Change `hologram.setReducedMotion(!!reduced);` (before `try { applyTier(); await rebuild(); }`) to `hologram.setReducedMotion(!!reduced); windows.setReducedMotion(!!reduced);`.
  7. In the returned `setReducedMotion(value) {`, change `hologram.setReducedMotion(!!value);` to `hologram.setReducedMotion(!!value); windows.setReducedMotion(!!value);`.
  8. In `dispose()`, change `surfaces.dispose();` to `surfaces.dispose(); windows.dispose();`.
  9. In `stats()`, change `surfaces:surfaces.stats(),` to `surfaces:surfaces.stats(), windows:windows.stats(),`.
- [ ] **`scripts/system-world-survey.mjs`.**
  - In `EXPOSE`, change `atmosphere, surfaces, sun, bloom, drones, ` to `atmosphere, surfaces, windows, sun, bloom, drones, `.
  - Add `windows` to the list in the header comment.
- [ ] **Append proof scenes** to `scripts/system-world-survey.scenes.json`.
  - They place one v3 `data-tower-a` LOD 0 on the sea in front of the quay, front (+Z) towards the camera.
  - The `eval` is identical in all five scenes and idempotent; set it as the `"eval"` value of each:
  ```
  "if(!__sw.scene.getObjectByName('survey-window-proof')){const t=(await __sw.model('data-tower-a',0,'v3')).clone(true);t.name='survey-window-proof';t.position.set(30,0,105);__sw.scene.add(t);__sw.renderer.shadowMap.needsUpdate=true;}let n=0,bad=0;__sw.scene.getObjectByName('survey-window-proof').traverse(m=>{if(m.isMesh&&m.material.name==='city.window'){n++;if(m.material!==__sw.windows.material)bad++;}});if(!n||bad)throw Error('window panes not routed: '+n+' panes, '+bad+' unrouted');"
  ```
  | name | position | target | environment |
  | --- | --- | --- | --- |
  | `proof-window-day` | [42,8,142] | [30,16,105] | day |
  | `proof-window-evening` | [42,8,142] | [30,16,105] | evening |
  | `proof-window-night` | [42,8,142] | [30,16,105] | night |
  | `proof-window-lobby` | [33.5,2.6,119] | [30,4,109] | night |
  | `proof-window-close` | [31.5,12,114.5] | [30,12.5,109.5] | night |

  All five use `"fov":43,"weather":"clear","frames":40,"waitMs":400`.
- [ ] **Run:**
  - `node scripts/test-system-world-windows.mjs` → PASS.
  - `node scripts/test-system-world-surfaces.mjs` → PASS.
  - `node scripts/test-system-world-layout.mjs` → PASS (imports the scene).
  - `node -e "import('esbuild').then(e=>e.build({entryPoints:['ui/js/desktop/apps/sysworld-scene.js'],bundle:true,format:'esm',write:false,logLevel:'error'})).then(()=>console.log('ok'))"` → `ok`.
  - `node scripts/system-world-survey.mjs --out reports/system-world-survey/phase-2-proof --tier high --only proof-window-day,proof-window-evening,proof-window-night,proof-window-lobby,proof-window-close` → 5 PNGs, 0 page errors. A GLSL compile error appears as a `console.error` page error and fails the run.
  - The same with `--tier medium --out reports/system-world-survey/phase-2-proof-medium --only proof-window-night,proof-window-close`, and with `--tier low --out reports/system-world-survey/phase-2-proof-low --only proof-window-night,proof-window-close`.
- [ ] **Visual acceptance** (judge the PNGs):
  - **Day:** dark glossy glass with faint rooms behind, about one room in eight lit.
  - **Evening:** about 60 % lit, ceiling panels bloom.
  - **Night:** about half lit, lobby row warm and lit.
  - **Close (high):** parallax rooms with side walls, floor and ceiling, desk or shelf silhouettes, some robots, some cyan screens, blinds in some rooms.
  - **Medium:** rooms without robots or screens.
  - **Low:** flat cells with frame lines.
  - **Everywhere:** no black or NaN blocks, and no shimmering stripes at the proof distance.
  - **Allowed tuning** (no structural change; record the final values in the commit message): the constants in `windowLighting`, the panel factor `2.5`, the sheen `0.6`, the wall palette.
- [ ] Commit: `feat(system-world): interior-mapped window panes (shared material, routing, survey proof)`.

### `ui/js/desktop/apps/sysworld-windows.js` (complete)

```js
import * as THREE from 'three';

// Interior-mapped window panes for the kit v3 buildings. Decorative, never telemetry.
// Every `city.window` quad carries room coordinates in TEXCOORD_0: u is one unit per bay, left
// to right seen from outside, +16 per facade; v is one unit per storey, 0 at the first upper
// floor, -1 for the ground-floor lobby, +20 per tier. glTF stores V flipped, so rows are 1 - v.
// One shared MeshStandardMaterial (glossy glass that still reflects the environment) adds, per
// room cell, a ray-cast box room behind the glass as emissive light: walls, floor, a ceiling
// light, a furniture layer, and on high tiers robots and screens. Lit rooms follow the time of
// day and switch slowly. The tangent frame and the room size come from screen-space derivatives
// (the kit has no tangents); every normalisation and division is guarded, because one NaN pixel
// would spread through the bloom chain.
export const WINDOW_MATERIAL = 'city.window';
const LEVELS = {low: 0, medium: 1, high: 2, ultra: 2};
export const windowLevel = tier => LEVELS[tier] ?? 2;
const clamp01 = v => (v > 0 ? (v < 1 ? v : 1) : 0);
const smoothstep = (x, a, b) => { const t = clamp01((x - a) / (b - a)); return t * t * (3 - 2 * t); };

// Same inputs and dusk curve as the pane lighting in sysworld-surfaces.js (day amount and evening
// from sysworld-exploration.js daylight): about 12 % of rooms lit by day, 60 % in the evening and
// 50 % at night; overcast days light a few more. Lobbies are lit from dusk to dawn.
export function windowLighting(day, evening, overcast = 0) {
  const d = clamp01(day), dusk = smoothstep(1 - d, .1, .9);
  return {
    share: Math.min(.9, .12 + .38 * dusk + .1 * clamp01(evening) * dusk + .08 * clamp01(overcast)),
    ambient: .012 + .1 * d,
    lobby: dusk,
    glow: .6 + .9 * dusk,
  };
}

export const WINDOW_SHADER = {
  vertexPars: 'varying vec3 vWinPos;varying vec3 vWinNrm;varying vec2 vWinRoom;varying float vWinSeed;',
  vertexMain: `
#ifdef USE_INSTANCING
mat4 winObject=modelMatrix*instanceMatrix;
#else
mat4 winObject=modelMatrix;
#endif
vWinPos=(winObject*vec4(transformed,1.0)).xyz;
vWinNrm=mat3(winObject)*objectNormal;
// glTF stores V flipped: rows count upwards from the first upper floor again.
vWinRoom=vec2(uv.x,1.0-uv.y);
// One seed per placed building from its world translation, so instanced towers differ.
vec3 winKey=fract(floor(winObject[3].xyz*4.0)*0.1031);
winKey+=dot(winKey,winKey.zyx+31.32);
vWinSeed=fract((winKey.x+winKey.y)*winKey.z);`,
  fragmentPars: `uniform float winTime;uniform vec4 winState;uniform vec3 winSky;
varying vec3 vWinPos;varying vec3 vWinNrm;varying vec2 vWinRoom;varying float vWinSeed;
float winHash(vec2 p){vec3 q=fract(vec3(p.xyx)*0.1031);q+=dot(q,q.yzx+33.33);return fract((q.x+q.y)*q.z);}
float winBox(vec2 p,vec2 lo,vec2 hi){vec2 s=step(lo,p)*step(p,hi);return s.x*s.y;}
// Robot silhouette in metres around its feet: two legs, a torso and a round head.
float winRobot(vec2 r){return min(1.0,winBox(vec2(abs(r.x),r.y),vec2(0.05,0.0),vec2(0.13,0.78))+winBox(r,vec2(-0.2,0.78),vec2(0.2,1.36))+step(length(r-vec2(0.0,1.52)),0.13));}`,
  fragmentMain: `
// Room cell, per-room hashes and the light state (rooms re-roll their light every 20-60 s).
vec2 winCell=floor(vWinRoom),winF=vWinRoom-winCell;
vec2 winAa=max(fwidth(vWinRoom),vec2(1e-4));
vec3 winN=vWinNrm/max(length(vWinNrm),1e-6);
vec3 winV=vWinPos-cameraPosition;winV/=max(length(winV),1e-4);
float winFres=pow(clamp(1.0-abs(dot(winV,winN)),0.0,1.0),5.0);
vec2 winKey=winCell+vWinSeed*vec2(71.3,113.9);
float winH0=winHash(winKey),winH1=winHash(winKey*1.31+3.7),winH2=winHash(winKey*0.71+9.1);
float winLobby=step(winCell.y,-0.5);
#if WIN_LEVEL>0
float winEpoch=floor(winTime/(20.0+40.0*winH0)+winH0);
#else
float winEpoch=0.0;
#endif
float winOn=max(step(winHash(winKey+winEpoch*vec2(3.1,1.7)+vec2(5.3,2.9)),winState.x),winLobby*step(0.5,winState.z));
vec3 winWall=mix(vec3(0.62,0.56,0.48),vec3(0.40,0.48,0.55),step(0.55,winH1));
vec3 winLamp=mix(vec3(1.0,0.8,0.56),vec3(0.8,0.9,1.0),step(0.72,winH1))*mix(vec3(1.0),vec3(1.12,0.95,0.78),winLobby);
vec3 winLit=mix(vec3(winState.y),winLamp*winState.w,winOn);
vec2 winS=vec2(WIN_BAY,WIN_STOREY);
vec3 winRoomCol;
#if WIN_LEVEL>0
// Tangent frame and metres per room from screen-space derivatives; an analytic frame (horizontal
// tangent, rows upwards, default room) replaces them when they are degenerate or implausible.
vec3 winDpx=dFdx(vWinPos),winDpy=dFdy(vWinPos);
vec2 winDux=dFdx(vWinRoom),winDuy=dFdy(vWinRoom);
float winDet=winDux.x*winDuy.y-winDux.y*winDuy.x;
float winOk=step(1e-12,abs(winDet));
float winDiv=mix(1.0,winDet,winOk);
vec3 winPu=(winDuy.y*winDpx-winDux.y*winDpy)/winDiv,winPv=(winDux.x*winDpy-winDuy.x*winDpx)/winDiv;
float winLu=length(winPu),winLv=length(winPv);
winOk*=step(0.25,winLu)*step(winLu,40.0)*step(0.25,winLv)*step(winLv,40.0);
vec3 winC=cross(vec3(0.0,1.0,0.0),winN);
vec3 winT=mix(winC/max(length(winC),1e-4),winPu/max(winLu,1e-4),winOk);
vec3 winB=mix(vec3(0.0,1.0,0.0),winPv/max(winLv,1e-4),winOk);
winS=mix(winS,vec2(winLu,winLv),winOk);
// View ray into a box room one cell wide, one storey high and WIN_DEPTH deep.
vec3 winD=vec3(dot(winV,winT),dot(winV,winB),max(-dot(winV,winN),0.02));
vec3 winSize=vec3(winS,WIN_DEPTH),winP=vec3(winF*winS,0.0);
vec3 winT3=abs(step(0.0,winD)*winSize-winP)/max(abs(winD),vec3(1e-4));
float winHit=min(min(winT3.x,winT3.y),winT3.z);
vec3 winQ=clamp((winP+winD*winHit)/winSize,0.0,1.0);
float winBack=step(winT3.z,min(winT3.x,winT3.y));
float winSide=(1.0-winBack)*step(winT3.x,winT3.y);
float winFlr=(1.0-winBack-winSide)*step(winD.y,0.0);
float winCeil=1.0-winBack-winSide-winFlr;
vec3 winAlb=winWall*(winBack+0.72*winSide+0.9*winCeil)+vec3(0.16,0.15,0.17)*winFlr;
float winPanel=winCeil*winBox(winQ.xz,vec2(0.28,0.3),vec2(0.72,0.62));
winRoomCol=winAlb*winLit*mix(0.9,0.6,winQ.z)+winPanel*winLamp*winOn*2.5*winState.w;
// Furniture at 55 % depth: a desk or counter, sometimes a tall shelf.
float winTf=0.55*WIN_DEPTH/winD.z;
vec2 winPf=(winP+winD*winTf).xy;
float winX0=(0.12+0.3*winH1)*winS.x,winX1=winX0+(0.3+0.2*fract(winH1*7.0))*winS.x;
float winFurn=min(1.0,winBox(winPf,vec2(winX0,0.0),vec2(winX1,0.76))+step(0.5,fract(winH1*13.0))*winBox(winPf,vec2(winS.x-0.42,0.0),vec2(winS.x-0.1,1.9)))*step(winTf,winHit);
winRoomCol=mix(winRoomCol,winWall*0.16*winLit,winFurn);
#if WIN_LEVEL>1
// Life at 30 % depth in a share of the lit rooms: a robot walking across or standing and bobbing
// (two in lobbies), and a flickering screen on the back wall of offices.
float winTl=0.3*WIN_DEPTH/winD.z;
vec2 winPl=(winP+winD*winTl).xy;
float winWalk=step(0.5,fract(winH2*5.3));
float winX=mix(0.2+0.6*fract(winH2*29.0),0.5+0.36*sin(winTime*(0.25+0.25*fract(winH2*17.0))+winH2*40.0),winWalk)*winS.x;
float winBob=0.025*sin(winTime*2.6+winH2*30.0)*(1.0-winWalk);
vec2 winR1=vec2(winPl.x-winX,winPl.y-winBob),winR2=vec2(winPl.x-winS.x+winX,winPl.y+winBob);
float winBots=max(winRobot(winR1),winRobot(winR2)*winLobby)*winOn*step(winH2,mix(0.4,0.85,winLobby))*step(winTl,winHit);
float winVisor=max(winBox(winR1,vec2(-0.09,1.49),vec2(0.09,1.55)),winBox(winR2,vec2(-0.09,1.49),vec2(0.09,1.55))*winLobby)*winBots;
float winScreen=winOn*winBack*(1.0-winLobby)*(1.0-winFurn)*step(fract(winH2*3.1),0.35)*winBox(vec2((winQ.x-0.5)*winS.x,winQ.y*winS.y),vec2(-0.5,1.0),vec2(0.5,1.6));
winRoomCol+=winScreen*vec3(0.3,0.7,1.0)*1.8*(0.8+0.2*sin(winTime*9.0+winH2*50.0)*sin(winTime*2.3+winH2*20.0));
winRoomCol=mix(winRoomCol,vec3(0.02,0.025,0.03),winBots)+winVisor*vec3(0.2,0.9,1.0)*1.5;
#endif
// Blinds in about one room in five, drawn on the glass; slats fade out when they get sub-pixel.
float winBlind=step(fract(winH1*23.0),0.2)*step(0.7-0.6*fract(winH2*41.0),winF.y);
float winSlat=0.8+0.2*step(0.5,fract(winF.y*winS.y*12.0))*(1.0-smoothstep(0.2,0.6,winAa.y*winS.y*12.0));
winRoomCol=mix(winRoomCol,vec3(0.55,0.5,0.42)*winSlat*(winState.y*1.6+winLamp*winOn*0.45*winState.w),winBlind);
#else
// Low tier: a flat lit or unlit cell, brighter under the ceiling; no parallax, no motion.
winRoomCol=winWall*winLit*0.8+winLamp*winOn*winState.w*0.5*smoothstep(0.35,1.0,winF.y);
#endif
winRoomCol*=1.0-winFres;
// A faint frame (mullions on the cell's vertical edges, a spandrel at its foot) so the lowest LOD,
// which has no fins or slabs, still reads as a facade.
vec2 winM=vec2(0.06,0.3)/winS;
float winFrame=max(1.0-smoothstep(winM.x-winAa.x,winM.x+winAa.x,min(winF.x,1.0-winF.x)),1.0-smoothstep(winM.y-winAa.y,winM.y+winAa.y,winF.y));
vec3 winGlow=(max(winRoomCol,vec3(0.0))+winSky*(0.04+0.6*winFres))*(1.0-winFrame);
diffuseColor.rgb=mix(diffuseColor.rgb,vec3(0.035,0.042,0.05),winFrame);`,
  roughness: '\nroughnessFactor=mix(roughnessFactor,0.55,winFrame);',
  emissive: '\ntotalEmissiveRadiance+=winGlow;',
};

export function createWindows() {
  const uniforms = {
    winTime: {value: 0},
    winState: {value: new THREE.Vector4(.12, .112, 0, .6)},   // lit share, unlit ambient, lobby, lit glow
    winSky: {value: new THREE.Color(0x08121e)},
  };
  let level = 2, tier = 'high', reduced = false, meshes = 0, disposed = false;
  const material = new THREE.MeshStandardMaterial({
    name: WINDOW_MATERIAL, color: new THREE.Color(.018, .032, .045), roughness: .08, metalness: 0, vertexColors: true,
  });
  material.onBeforeCompile = shader => {
    Object.assign(shader.uniforms, uniforms);
    const head = `#define WIN_LEVEL ${level}\n#define WIN_DEPTH 3.0\n#define WIN_BAY 1.9\n#define WIN_STOREY 3.05\n`;
    shader.vertexShader = head + shader.vertexShader
      .replace('#include <common>', '#include <common>\n' + WINDOW_SHADER.vertexPars)
      .replace('#include <begin_vertex>', '#include <begin_vertex>' + WINDOW_SHADER.vertexMain);
    shader.fragmentShader = head + shader.fragmentShader
      .replace('#include <common>', '#include <common>\n' + WINDOW_SHADER.fragmentPars)
      .replace('#include <color_fragment>', '#include <color_fragment>' + WINDOW_SHADER.fragmentMain)
      .replace('#include <roughnessmap_fragment>', '#include <roughnessmap_fragment>' + WINDOW_SHADER.roughness)
      .replace('#include <emissivemap_fragment>', '#include <emissivemap_fragment>' + WINDOW_SHADER.emissive);
  };
  material.customProgramCacheKey = () => 'sw-windows|' + level;
  return {
    material, uniforms,
    owns: m => m === material,
    // Swaps every city.window slot of a mesh for the shared material and disposes the loaded one.
    adopt(mesh) {
      if (!mesh?.isMesh || disposed) return false;
      const list = Array.isArray(mesh.material) ? mesh.material : [mesh.material];
      let hit = false;
      const next = list.map(m => {
        if (m === material || m?.name !== WINDOW_MATERIAL) return m;
        hit = true; m.dispose(); return material;
      });
      if (!hit) return false;
      mesh.material = Array.isArray(mesh.material) ? next : next[0];
      meshes++;
      return true;
    },
    setLighting(day, evening, overcast = 0, sky = null) {
      const l = windowLighting(day, evening, overcast);
      uniforms.winState.value.set(l.share, l.ambient, l.lobby, l.glow);
      if (sky) uniforms.winSky.value.copy(sky);
    },
    setTier(value) {
      tier = value;
      const next = windowLevel(value);
      if (next !== level) { level = next; material.needsUpdate = true; }
    },
    setReducedMotion(value) { reduced = !!value; },
    // Time drives room switching and the life layer; it stands still without motion.
    update(dt, animated) {
      if (!animated || reduced || disposed) return;
      uniforms.winTime.value = (uniforms.winTime.value + Math.min(.1, Math.max(0, dt || 0))) % 7200;
    },
    stats: () => ({
      level, tier, reduced, meshes,
      share: +uniforms.winState.value.x.toFixed(2), lobby: +uniforms.winState.value.z.toFixed(2),
      glow: +uniforms.winState.value.w.toFixed(2), time: +uniforms.winTime.value.toFixed(1),
    }),
    dispose() { disposed = true; material.dispose(); },
  };
}
```

### `scripts/test-system-world-windows.mjs` (complete)

```js
// System World window interiors: lit-share curve, levels, material flags, shader injection
// against the pinned Three.js standard material, uniform plumbing, NaN guards, and routing of
// the real kit v3 panes. Needs no GPU.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import * as THREE from 'three';
import {GLTFLoader} from 'three/addons/loaders/GLTFLoader.js';
import {MeshoptDecoder} from 'three/addons/libs/meshopt_decoder.module.js';
import {WINDOW_MATERIAL, WINDOW_SHADER, windowLighting, windowLevel, createWindows} from '../ui/js/desktop/apps/sysworld-windows.js';
import {createSurfaces} from '../ui/js/desktop/apps/sysworld-surfaces.js';

const near = (a, b, e = .005) => Math.abs(a - b) <= e;

// 1. Lit share by time of day (daylight(): day 1/0, evening 0/1, night 0/0).
const day = windowLighting(1, 0), evening = windowLighting(0, 1), night = windowLighting(0, 0);
assert.ok(near(day.share, .12) && near(evening.share, .6) && near(night.share, .5), 'lit share ' + JSON.stringify([day.share, evening.share, night.share]));
assert.ok(windowLighting(1, 0, 1).share > day.share, 'overcast days light more rooms');
for (let d = 0, last = Infinity; d <= 1.0001; d += .05) { const s = windowLighting(d, 0).share; assert.ok(s <= last + 1e-9, 'share must fall as daylight grows'); last = s; }
assert.ok(day.lobby === 0 && night.lobby === 1 && evening.lobby === 1, 'lobbies are lit from dusk to dawn');
assert.ok(day.ambient > night.ambient * 5 && night.ambient < .02, 'unlit rooms: dim by day, near black at night');
assert.ok(night.glow > day.glow, 'lit rooms glow brighter at night');
for (const v of Object.values(windowLighting(NaN, -3, 7))) assert.ok(Number.isFinite(v), 'lighting stays finite on bad input');

// 2. Levels follow the scene tiers.
assert.deepEqual(['low', 'medium', 'high', 'ultra', 'bogus'].map(windowLevel), [0, 1, 2, 2, 2]);

// 3. One shared material: glossy, reflective, vertex colours on, opaque.
const windows = createWindows(), m = windows.material;
assert.ok(m.isMeshStandardMaterial && m.name === WINDOW_MATERIAL && m.vertexColors === true && m.transparent === false, 'material flags');
assert.ok(near(m.roughness, .08) && m.metalness === 0 && m.envMap === null, 'glass reflects the scene environment');

// 4. Injection: every hook matched once per tier, defines first, uniforms shared by reference.
const compile = tier => {
  windows.setTier(tier);
  const shader = {uniforms: {}, vertexShader: THREE.ShaderLib.standard.vertexShader, fragmentShader: THREE.ShaderLib.standard.fragmentShader};
  m.onBeforeCompile(shader);
  return shader;
};
const keys = new Set();
for (const [tier, level] of [['low', 0], ['medium', 1], ['high', 2], ['ultra', 2]]) {
  const s = compile(tier);
  assert.ok(s.vertexShader.startsWith('#define WIN_LEVEL ' + level) && s.fragmentShader.startsWith('#define WIN_LEVEL ' + level), tier + ': level define');
  for (const part of [WINDOW_SHADER.vertexPars, WINDOW_SHADER.vertexMain]) assert.equal(s.vertexShader.split(part).length, 2, tier + ': vertex hook');
  for (const part of [WINDOW_SHADER.fragmentPars, WINDOW_SHADER.fragmentMain, WINDOW_SHADER.roughness, WINDOW_SHADER.emissive])
    assert.equal(s.fragmentShader.split(part).length, 2, tier + ': fragment hook');
  assert.ok(s.fragmentShader.indexOf(WINDOW_SHADER.fragmentMain) > s.fragmentShader.indexOf('#include <color_fragment>'), 'interior after the colour chunk');
  assert.ok(s.fragmentShader.indexOf(WINDOW_SHADER.emissive) > s.fragmentShader.indexOf('#include <emissivemap_fragment>'), 'glow after the emissive chunk');
  for (const name of ['winTime', 'winState', 'winSky']) assert.equal(s.uniforms[name], windows.uniforms[name], name + ' shared by reference');
  keys.add(m.customProgramCacheKey());
}
assert.equal(keys.size, 3, 'one program per level');
assert.ok(WINDOW_SHADER.vertexMain.includes('#ifdef USE_INSTANCING') && WINDOW_SHADER.vertexMain.includes('modelMatrix*instanceMatrix'), 'instancing-safe world position and seed');

// 5. Tier switches recompile only when the level changes.
windows.setTier('high'); const version = m.version;
windows.setTier('ultra'); assert.equal(m.version, version, 'high and ultra share a program');
windows.setTier('medium'); assert.ok(m.version > version, 'a level change recompiles');

// 6. Lighting and time plumbing; reduced motion freezes the life layer.
windows.setLighting(0, 1, 0, new THREE.Color(0x2a2638));
assert.ok(near(windows.uniforms.winState.value.x, .6) && windows.uniforms.winState.value.z === 1, 'evening state');
assert.equal(windows.uniforms.winSky.value.getHex(), 0x2a2638, 'sky sheen follows the fog colour');
const t0 = windows.uniforms.winTime.value;
windows.update(.05, true); assert.ok(near(windows.uniforms.winTime.value - t0, .05, 1e-9), 'time advances');
windows.update(5, true); assert.ok(near(windows.uniforms.winTime.value - t0, .15, 1e-9), 'dt is clamped to 0.1 s');
windows.update(.05, false); windows.setReducedMotion(true); windows.update(.05, true); windows.update(NaN, true);
assert.ok(near(windows.uniforms.winTime.value - t0, .15, 1e-9), 'no motion: time frozen');
windows.setReducedMotion(false);
assert.deepEqual(Object.keys(windows.stats()).sort(), ['glow', 'level', 'lobby', 'meshes', 'reduced', 'share', 'tier', 'time']);

// 7. NaN guards: no normalize() at all, every pow() base clamped, every division guarded,
// balanced preprocessor blocks and parentheses.
const glsl = Object.values(WINDOW_SHADER).join('\n'), code = glsl.replace(/\/\/[^\n]*/g, '');
assert.ok(!code.includes('normalize('), 'use x/max(length(x),eps) instead of normalize()');
for (const hit of code.matchAll(/pow\(/g)) assert.ok(code.startsWith('pow(clamp(', hit.index), 'unclamped pow() base');
for (const [, token] of code.matchAll(/\/\s*([A-Za-z_][\w.]*|\()/g))
  assert.ok(['max', 'winDiv', 'winSize', 'winS', 'winD.z', '('].includes(token), 'unguarded division by ' + token);
for (const [name, part] of Object.entries(WINDOW_SHADER)) {
  const strip = part.replace(/\/\/[^\n]*/g, '');
  assert.equal((strip.match(/^#if/gm) || []).length, (strip.match(/^#endif/gm) || []).length, name + ': #if/#endif');
  assert.equal((strip.match(/\(/g) || []).length, (strip.match(/\)/g) || []).length, name + ': parentheses');
}
assert.ok(code.includes('mix(1.0,winDet,winOk)') && code.includes('max(-dot(winV,winN),0.02)') && code.includes('max(abs(winD),vec3(1e-4))'), 'derivative guards present');

// 8. Routing: adopt() swaps city.window for the shared material; surfaces never patch it.
const original = new THREE.MeshStandardMaterial({name: WINDOW_MATERIAL, vertexColors: true});
let disposals = 0; original.addEventListener('dispose', () => disposals++);
const pane = new THREE.Mesh(new THREE.PlaneGeometry(), original);
assert.equal(windows.adopt(pane), true); assert.equal(pane.material, m); assert.equal(disposals, 1);
assert.equal(windows.adopt(pane), false, 'adopting twice is a no-op');
const other = new THREE.Mesh(new THREE.PlaneGeometry(), new THREE.MeshStandardMaterial({name: 'city.graphite'}));
assert.equal(windows.adopt(other), false); assert.notEqual(other.material, m);
const multi = new THREE.Mesh(new THREE.PlaneGeometry(), [new THREE.MeshStandardMaterial({name: 'city.titanium'}), new THREE.MeshStandardMaterial({name: WINDOW_MATERIAL})]);
assert.equal(windows.adopt(multi), true); assert.equal(multi.material[1], m); assert.notEqual(multi.material[0], m);
assert.ok(windows.owns(m) && !windows.owns(original));
const surfaces = createSurfaces({capabilities: {getMaxAnisotropy: () => 8}}, {});
const hook = m.onBeforeCompile; surfaces.apply(m); surfaces.prepare(pane);
assert.equal(m.onBeforeCompile, hook, 'sysworld-surfaces.js must leave city.window alone');
assert.equal(pane.geometry.getAttribute('swSeed'), undefined, 'no pane seeds on window panes');
surfaces.dispose();

// 9. Real kit v3 buildings: every pane at every LOD routes and carries room coordinates and AO.
await MeshoptDecoder.ready;
const loader = new GLTFLoader().setMeshoptDecoder(MeshoptDecoder);
const kit = JSON.parse(await fs.readFile('ui/3d/system-world/v3/manifest.json', 'utf8'));
let routed = 0;
for (const id of ['data-tower-a', 'agent-spire', 'knowledge-atrium', 'operations-beacon']) {
  const entry = kit.assets.find(a => a.id === id); assert.ok(entry, 'kit v3 lacks ' + id);
  for (const lod of entry.lods) {
    const data = await fs.readFile('ui/3d/system-world/v3/' + lod.file);
    const gltf = await loader.parseAsync(data.buffer.slice(data.byteOffset, data.byteOffset + data.byteLength), '');
    let panes = 0;
    gltf.scene.traverse(n => {
      if (!n.isMesh || !windows.adopt(n)) return;
      panes++;
      assert.ok(n.geometry.getAttribute('uv') && n.geometry.getAttribute('color'), lod.file + ': pane lacks uv or colour');
    });
    gltf.scene.traverse(n => { if (n.isMesh) assert.ok(n.material === m || n.material.name !== WINDOW_MATERIAL, lod.file + ': unrouted pane'); });
    assert.ok(panes > 0, lod.file + ': no window panes'); routed += panes;
  }
}
assert.equal(windows.stats().meshes, routed + 2, 'stats count every routed mesh');
windows.dispose();
console.log(`System World windows: share day/evening/night ${day.share.toFixed(2)}/${evening.share.toFixed(2)}/${night.share.toFixed(2)}, 3 programs, guards verified, ${routed} kit v3 pane meshes routed.`);
```

**Acceptance**
- The test passes and the survey proof shows interiors on all three levels with 0 page errors.
- Nothing in the city itself changes yet (no v3 building is placed).

---

## Task 3 — Swap: kit v3 wins per id (districts, towers, skyline, skybridge, lamps, planters)

**Files**
- `ui/js/desktop/apps/sysworld-scene.js`: module-level exports, `createCity` collider call, `rebuild()`, `stats()`
- `ui/js/desktop/apps/sysworld-colliders.js`
- `scripts/test-system-world-v3.mjs`, `scripts/test-system-world-layout.mjs`, `scripts/test-system-world-hover.mjs`, `scripts/test-system-world-living.mjs`

**Steps**

- [ ] **One-off bounds check.** Write a throwaway Node script in scratch that reads both manifests and prints, for the twelve ids below, the LOD 0 `bounds` of v1 and v3 and the largest |Δ| in x/z and in max y.
  - Acceptance:
    - buildings |Δx|, |Δz| ≤ 1.0 m and Δ max y ≤ 2.5 m;
    - `street-lamp` and `planter` |Δ| ≤ 0.3 m.
  - Report the table in the commit message. On a breach, stop and report it to the main session (colliders and hover docks depend on it).
  - Ids: `agent-spire, memory-archive, integration-gate, mission-terminal, compute-foundry, knowledge-atrium, operations-beacon, data-tower-a, data-tower-b, skybridge, street-lamp, planter`.
- [ ] **Test first: `scripts/test-system-world-v3.mjs`.**
  - Add `import { supersededV1, kitFor } from '../ui/js/desktop/apps/sysworld-scene.js';`.
  - Before the final `console.log`, add:
    ```js
    // Phase 2: kit v3 takes over twelve v1 designs and ships street furniture that is not placed yet.
    const furniture = ['tree-round', 'tree-column', 'street-bench', 'bollard', 'litter-bin', 'zebra-sign', 'kiosk'];
    assert.equal(manifest.assets.length, 25, '6 street + 12 superseded + 7 new furniture designs');
    for (const id of [...supersededV1, ...furniture]) assert.ok(ids.includes(id), 'Missing v3 design ' + id);
    const catalogV3 = new Map(manifest.assets.map(a => [a.id, a]));
    for (const id of supersededV1) assert.equal(kitFor(id, catalogV3), 'v3', id + ' must load from v3');
    assert.equal(kitFor('service-drone', catalogV3), 'v1');
    for (const id of supersededV1.filter(id => id !== 'street-lamp' && id !== 'planter')) assert.ok(windowDesigns.has(id), id + ' has window panes');
    assert.ok(catalogV3.get('operations-beacon').components.includes('signal'), 'operations-beacon keeps its signal part');
    ```
  - Run → FAIL (exports missing).
- [ ] **`sysworld-colliders.js`.** Add above `installCityColliders`:
  ```js
  // One catalog for colliders and tests: entries of later catalogs (kit v3) replace earlier ones
  // (kit v1) with the same id. Accepts Maps or manifests ({assets: [...]}).
  export function kitCatalog(...catalogs){
    const merged=new Map();
    for(const c of catalogs)for(const [id,a] of c instanceof Map?c:(c?.assets||[]).map(a=>[a.id,a]))merged.set(id,a);
    return merged;
  }
  ```
- [ ] **`sysworld-scene.js`.**
  1. Change `import {installCityColliders} from './sysworld-colliders.js';` to `import {installCityColliders,kitCatalog} from './sysworld-colliders.js';`.
  2. Directly above `export const placements = [];` add:
     ```js
     // Designs kit v3 took over from v1 in phase 2. Wherever both kits ship an id, v3 wins: districts,
     // skyline, towers, lamps and planters load the v3 design (kitFor), colliders read its bounds.
     export const supersededV1 = ['agent-spire', 'memory-archive', 'integration-gate', 'mission-terminal', 'compute-foundry', 'knowledge-atrium',
       'operations-beacon', 'data-tower-a', 'data-tower-b', 'skybridge', 'street-lamp', 'planter'];
     export function kitFor(id, v3Catalog) { return v3Catalog?.has(id) ? 'v3' : 'v1'; }
     ```
  3. In `createCity`, change `installCityColliders(traffic,catalog,districts,placements);` to `installCityColliders(traffic,kitCatalog(catalog,catalog3),districts,placements);`.
  4. In `rebuild()`:

     Before:
     ```js
         const kitIds = [...new Set(kitPlacements.map(p => p.asset))];
         const size = (id, level) => catalog.get(id)?.lods.find(l => l.level === level)?.bytes || 0;
         expectedBytes = ids.reduce((sum, id) => sum + (cache.has('v1:' + id + ':' + lod) ? 0 : size(id, lod)), loadBytes) + (lod === 2 ? 0 : ['data-tower-a', 'data-tower-b'].reduce((sum, id) => sum + (cache.has('v1:' + id + ':2') ? 0 : size(id, 2)), 0))
           + kitIds.reduce((sum, id) => sum + (cache.has('v3:' + id + ':' + lod) ? 0 : (catalog3.get(id)?.lods.find(l => l.level === lod)?.bytes || 0)), 0);
         try {
           const loaded = new Map(await Promise.all(ids.map(async id => [id, await model(id, lod)])));
           // Skyline deliberately uses low LOD even on Ultra.
           const distant = new Map(await Promise.all(['data-tower-a','data-tower-b'].map(async id => [id, await model(id, 2)])));
     ```
     After:
     ```js
         const kitIds = [...new Set(kitPlacements.map(p => p.asset))];
         // A v3 design replaces the v1 design with the same id everywhere it is placed.
         const kitOf = id => kitFor(id, catalog3);
         const size = (id, level, kit = kitOf(id)) => (kit === 'v3' ? catalog3 : catalog).get(id)?.lods.find(l => l.level === level)?.bytes || 0;
         const pending = (id, level, kit = kitOf(id)) => cache.has(kit + ':' + id + ':' + level) ? 0 : size(id, level, kit);
         expectedBytes = ids.reduce((sum, id) => sum + pending(id, lod), loadBytes) + (lod === 2 ? 0 : ['data-tower-a', 'data-tower-b'].reduce((sum, id) => sum + pending(id, 2), 0))
           + kitIds.reduce((sum, id) => sum + pending(id, lod, 'v3'), 0);
         try {
           const loaded = new Map(await Promise.all(ids.map(async id => [id, await model(id, lod, kitOf(id))])));
           // Skyline deliberately uses low LOD even on Ultra.
           const distant = new Map(await Promise.all(['data-tower-a','data-tower-b'].map(async id => [id, await model(id, 2, kitOf(id))])));
     ```
     The rest of `rebuild()` stays: the batch `add`, the landmark clones, `life?.attachLandmarks`, drones, and roofs/aviation from `distant`.
  5. In `stats()`, after `cachedModels:cache.size,` add `kitModels:{v1:[...cache.keys()].filter(k=>k.startsWith('v1:')).length,v3:[...cache.keys()].filter(k=>k.startsWith('v3:')).length},`.
- [ ] **Collider tests use the merged catalog.** Change only the catalog argument.
  - `test-system-world-layout.mjs`:
    - change the import to `import {installCityColliders,kitCatalog} from '../ui/js/desktop/apps/sysworld-colliders.js';`;
    - `installCityColliders(city,new Map(v1.assets.map(a=>[a.id,a])),districts,[]);` → `installCityColliders(city,kitCatalog(v1,JSON.parse(await fs.readFile('ui/3d/system-world/v3/manifest.json','utf8'))),districts,[]);`.
  - `test-system-world-hover.mjs`:
    - same import change;
    - append `,v3=JSON.parse(await fs.readFile('ui/3d/system-world/v3/manifest.json','utf8'))` to the `const real=…,v1=…;` declaration;
    - `installCityColliders(traffic,new Map(v1.assets.map(a=>[a.id,a])),districts,placements);` → `installCityColliders(traffic,kitCatalog(v1,v3),districts,placements);`.
  - `test-system-world-living.mjs`:
    - same import change;
    - `installCityColliders(traffic,new Map(catalog.assets.map(a=>[a.id,a])),districts,placements);` → `installCityColliders(traffic,kitCatalog(catalog,JSON.parse(await fs.readFile(root+'v3/manifest.json','utf8'))),districts,placements);`.
- [ ] **Run**, each must pass:
  - `node scripts/test-system-world-v3.mjs`
  - `node scripts/test-system-world-windows.mjs`
  - `node scripts/test-system-world-surfaces.mjs`
  - `node scripts/test-system-world-layout.mjs`
  - `node scripts/test-system-world-hover.mjs`
  - `node scripts/test-system-world-living.mjs`
  - `node scripts/test-system-world-traffic.mjs`
  - `node scripts/test-system-world.mjs`
  - If the living or hover thresholds move because lamp or planter colliders changed, report the numbers. Do not loosen thresholds without the main session.
- [ ] **After-survey of the swap:**
  - `node scripts/system-world-survey.mjs --out reports/system-world-survey/phase-2-swap --tier high --only <the 17 Task 1 scenes>,overview-day,street-agent,memory-facade,foundry-facade,atrium-facade`.
  - Compare with `phase-2-before`.
  - Also report the `stats.calls`, `stats.triangles` and `stats.loadedBytes` of `overview-day` in both runs.
- [ ] **Visual acceptance:**
  - every district, both towers, the skybridge, lamps and planters show the v3 designs;
  - facades show rooms at street level by day, evening and night;
  - lobbies are lit at night;
  - `skyline-night` shows the instanced, stretched LOD 2 towers with lit rooms and frame lines and no shimmer bands;
  - aviation lights sit on the roofs or masts;
  - the hologram sits on the memory roof;
  - the beacon `signal` still pulses.
- [ ] Commit: `feat(system-world): kit v3 buildings, towers, lamps and planters replace their v1 designs`.

**Acceptance**
- `cachedModels` shows v3 keys for all twelve ids (`kitModels.v3 ≥ 14` on high).
- No v1 key for a superseded id is ever created.
- Colliders come from v3 bounds.
- The pictures are captured.

---

## Task 4 — Retire the superseded v1 designs

**Files:** `ui/3d/system-world/v1/manifest.json`, 36 GLBs, `assets/system-world/check_assets.py`, `scripts/test-system-world-v3.mjs`.

**Steps**

- [ ] **Test first.** In `scripts/test-system-world-v3.mjs`, before the final `console.log`, add:
  ```js
  // The twelve superseded v1 designs left the runtime payload; v1 keeps the drone, the tram and the rack.
  const v1 = JSON.parse(await fs.readFile('ui/3d/system-world/v1/manifest.json', 'utf8'));
  assert.deepEqual(v1.assets.map(a => a.id).sort(), ['data-tram', 'server-rack', 'service-drone']);
  for (const id of supersededV1) for (const level of [0, 1, 2])
    await assert.rejects(fs.access(`ui/3d/system-world/v1/${id}.lod${level}.glb`), 'retired v1 file still present: ' + id);
  ```
  Run → FAIL.
- [ ] **Edit the v1 manifest.** Write a throwaway Python script in scratch:
  ```python
  import json, pathlib
  p = pathlib.Path('ui/3d/system-world/v1/manifest.json')
  m = json.loads(p.read_text(encoding='utf-8'))
  retired = {'agent-spire', 'memory-archive', 'integration-gate', 'mission-terminal', 'compute-foundry', 'knowledge-atrium',
             'operations-beacon', 'data-tower-a', 'data-tower-b', 'skybridge', 'street-lamp', 'planter'}
  assert retired <= {a['id'] for a in m['assets']}
  m['assets'] = [a for a in m['assets'] if a['id'] not in retired]
  assert sorted(a['id'] for a in m['assets']) == ['data-tram', 'server-rack', 'service-drone'], [a['id'] for a in m['assets']]
  m['total_model_bytes'] = sum(l['bytes'] for a in m['assets'] for l in a['lods'])
  with p.open('w', encoding='utf-8', newline='\n') as f:
      f.write(json.dumps(m, indent=2) + '\n')
  print(m['total_model_bytes'])
  ```
  - Run it from the worktree root. Expected output: `280116`.
  - `git diff ui/3d/system-world/v1/manifest.json` must show only removed asset blocks and the new total. If other lines are reformatted, match the original formatting (as phase 1 Task 12 did) and rerun.
- [ ] **Delete the 36 GLBs** (PowerShell):
  ```powershell
  $ids='agent-spire','memory-archive','integration-gate','mission-terminal','compute-foundry','knowledge-atrium','operations-beacon','data-tower-a','data-tower-b','skybridge','street-lamp','planter'
  foreach($id in $ids){ foreach($l in 0,1,2){ git rm -q "ui/3d/system-world/v1/$id.lod$l.glb" } }
  ```
- [ ] **`check_assets.py`.** In `check()`, change `assert len(manifest['assets']) == 15, 'Incomplete asset catalog'` to `== 3`. If it still says 17, phase 1 Task 12 has not landed: stop.
- [ ] **Run**, each must pass:
  - `python assets/system-world/check_assets.py` → `"assets": 3`, `"models": 9`, `"model_bytes": 280116`, `"checks": "passed"`.
  - `node scripts/test-system-world-v3.mjs`
  - `node scripts/test-system-world-layout.mjs`
  - `node scripts/test-system-world-hover.mjs`
  - `node scripts/test-system-world-living.mjs`
  - `node scripts/test-system-world-expansion.mjs` (model directory down by 3,998,504 bytes)
  - `node scripts/test-system-world-surfaces.mjs`
  - `node scripts/build-system-world-v3.mjs --check`
  - `node scripts/system-world-survey.mjs --out reports/system-world-survey/phase-2-retired --tier high --only overview-day,facade-agent-night,skyline-night` → 0 page errors, and `report.json` `http404` empty.
- [ ] Commit: `chore(system-world): retire the twelve v1 designs superseded by kit v3`.

**Acceptance:** v1 holds 3 designs and 9 GLBs, the city loads with no 404s, and every listed test passes.

---

## Task 5 — Street-lamp cones only after dusk (parallel with any task)

**Files:** `ui/js/desktop/apps/sysworld-atmosphere.js` (`createAtmosphere`), new `scripts/test-system-world-lamps.mjs`.

**Steps**

- [ ] Write the test, run → FAIL (`lampLevel` missing):
  ```js
  // Street-lamp light cones are a night effect: invisible by day, fading in towards dusk, off on low.
  import assert from 'node:assert/strict';
  import * as THREE from 'three';
  import {createAtmosphere} from '../ui/js/desktop/apps/sysworld-atmosphere.js';

  const scene = new THREE.Scene(); scene.fog = new THREE.FogExp2(0x08121e, .004);
  const atmosphere = createAtmosphere(scene, {lamps: [{x: 0, z: 0, y: 0, angle: 0}]});
  const camera = new THREE.PerspectiveCamera(), up = new THREE.Vector3(0, 1, 0);
  const cones = scene.getObjectByName('city-atmosphere').children.find(n => n.isInstancedMesh && n.geometry.type === 'ConeGeometry');
  assert.ok(cones, 'lamp cones exist');
  atmosphere.setTier('medium');   // medium keeps the cones but skips the cinema pass
  const at = (day, evening) => { atmosphere.setLighting(day, evening, up); atmosphere.update(.016, 1, camera, true); return atmosphere.stats().lampLevel; };
  assert.equal(at(1, 0), 0); assert.equal(cones.visible, false, 'no cones by day');
  const afternoon = at(.35, .1);
  assert.ok(afternoon > 0 && afternoon < 1 && cones.visible, 'cones fade in towards dusk');
  assert.equal(at(0, 1), 1); assert.equal(at(0, 0), 1); assert.equal(cones.visible, true, 'full cones in the evening and at night');
  atmosphere.setTier('low'); assert.equal(cones.visible, false, 'low tier keeps cones off');
  atmosphere.dispose();
  console.log('System World lamps: cones off by day, ' + afternoon.toFixed(2) + ' in the afternoon, full at night.');
  ```
  If `createAtmosphere` cannot be constructed in Node (for example `createCinema` needs a DOM), do not mock it. Replace the test with source assertions on `sysworld-atmosphere.js` (`lampLevel`, `smoothstep(daylight, .12, .55)`, `.3 * lampLevel *`) and report it.
- [ ] **`sysworld-atmosphere.js`:**
  1. `let busy = false, tier = 'high', sweep = 0;` → `let busy = false, tier = 'high', sweep = 0, lampLevel = 1;`. On the next line add:
     ```js
       // Lamp cones only read at night: invisible by day, fading in towards dusk (never on low).
       const showLamps = () => { lampCones.visible = tier !== 'low' && lampLevel > .01; };
     ```
  2. At the end of `setLighting(daylight, evening, direction) {`, before its closing `},`, add `lampLevel = 1 - THREE.MathUtils.smoothstep(daylight, .12, .55); showLamps();`.
  3. In `setTier`, change `mist.visible = rich; dust.visible = rich; lampCones.visible = rich;` to `mist.visible = rich; dust.visible = rich; showLamps();`.
  4. In `update`, change `lampMaterial.uniforms.strength.value = .3 * (1 + ` to `lampMaterial.uniforms.strength.value = .3 * lampLevel * (1 + ` (rest of the line unchanged).
  5. In `stats`, add `lampLevel: +lampLevel.toFixed(2),` after `lamps: lamps.length,`.
- [ ] **Run:**
  - `node scripts/test-system-world-lamps.mjs` → PASS.
  - `go test ./ui -run '^TestDesktopSysWorld' -count=1` → PASS (atmosphere markers unchanged).
  - `node scripts/system-world-survey.mjs --out reports/system-world-survey/phase-2-lamps --tier high --only street-agent,night-street,evening-overview` → no cones in `street-agent` (day), cones in `night-street`.
- [ ] Commit: `fix(system-world): street-lamp light cones appear only towards dusk`.

---

## Task 6 — Documentation (after Task 4)

**Files:** `assets/system-world/AGENTS.md`, `assets/system-world/README.md`, `ui/3d/system-world/AGENTS.md`, `plans/2026-11-09-system-world-beauty-design.md`.

**Content** (concise, existing style)

- **`assets/system-world/AGENTS.md`**
  - **Kit v3 families:** `facade_assets3.py`, `district_assets3.py`, `furniture_assets3.py` (registered in `build_world3.py` `FAMILIES`; hashed in `sources`).
  - **Window contract:**
    - material `city.window`, one quad per facade and tier (flat per segment on curved buildings);
    - `TEXCOORD_0` = room coordinates: u one unit per bay, left to right from outside, +16 per face; v one unit per storey, 0 = first upper floor, −1 = lobby, +20 per tier;
    - bays 0.8–6 m, storeys 1.2–6 m;
    - corners on whole rooms;
    - checked by `scripts/test-system-world-v3.mjs`;
    - the exporter flips V and the renderer undoes it.
  - **No separate lit-window quads.** `city.ivory`/`city.warm` in v3 are fixtures.
  - **Pack flags:** `-vtf` (or the fallback actually used at Task 0).
  - **v1:** now only `service-drone`, `data-tram`, `server-rack`. `build_city.py` must drop the twelve retired designs before the next v1 rebuild, or `check_assets.py` (3 designs) fails. That is main-session work.
- **`README.md`**
  - Runtime payload: v1 3 designs / 9 GLBs.
  - Kit v3 lists buildings, towers and furniture.
  - The renderer's interior-mapped windows (`ui/js/desktop/apps/sysworld-windows.js`) with tiers low/medium/high.
  - The v3 precedence rule (`kitFor`).
- **`ui/3d/system-world/AGENTS.md`**
  - "Preserve the 15 v1 designs" → 3.
  - v3 wins per id.
  - `city.window` is owned by `sysworld-windows.js` (never `sysworld-surfaces.js`).
  - Colliders read `kitCatalog(v1, v3)`.
  - Verification commands: `node scripts/test-system-world-windows.mjs`, `node scripts/test-system-world-lamps.mjs`.
- **Design doc.** In the roadmap row for phase 2, replace "own spec and plan" with `plans/2026-11-09-system-world-beauty-phase-2.md`. Move "street-lamp light cones" from "noted for phase 6" to "fixed in phase 2".

**Steps**
- [ ] Edit the four files.
- [ ] `go test ./ui -run '^TestDesktopSysWorld' -count=1` → PASS.
- [ ] Commit: `docs(system-world): kit v3 buildings, window contract, retired v1 designs`.

---

## Task 7 — Markers, browser check, bundle, final regression, survey set, budgets

**Files:** `ui/desktop_sysworld_test.go` (`TestDesktopSysWorldAppMarkers`), `ui/desktop_sysworld_city_browser_test.go`, `ui/js/vendor/system-world/*` (generated).

**Steps**

- [ ] **Markers.** In `TestDesktopSysWorldAppMarkers`:
  - add the entry
    ```go
    "js/desktop/apps/sysworld-windows.js": {"export function createWindows", "export function windowLighting", "export const WINDOW_MATERIAL", "onBeforeCompile", "customProgramCacheKey", "#define WIN_LEVEL ", "USE_INSTANCING", "vWinRoom=vec2(uv.x,1.0-uv.y)", "dispose()"},
    ```
  - append `"createWindows()", "windows.adopt(n)", "kitFor(id, catalog3)", "kitCatalog(catalog,catalog3)"` to the `sysworld-scene.js` list;
  - append `"lampLevel"` to the `sysworld-atmosphere.js` list;
  - run `gofmt -w ui/desktop_sysworld_test.go`.
- [ ] **Browser check.** In `ui/desktop_sysworld_city_browser_test.go`:
  - directly after the surface-detail `page.MustEval` (the one throwing `'Surface detail inactive: '`), add:
    ```go
    	// Window interiors: kit v3 panes on the shared interior material, full level on high.
    	page.MustEval(`()=>{const s=SysWorldApp.inspect(cityId);if(!s.windows||s.windows.level!==2||s.windows.meshes<10||s.kitModels.v3<14)throw Error('Window interiors inactive: '+JSON.stringify({windows:s.windows,kits:s.kitModels}));}`)
    ```
  - after the low-tier eval that throws `'Low tier must disable surface detail'`, add:
    ```go
    	page.MustEval(`()=>{const s=SysWorldApp.inspect(cityId);if(s.windows.level!==0)throw Error('Low tier must use flat window cells: '+JSON.stringify(s.windows));}`)
    ```
- [ ] **Bundle:** `node scripts/build-system-world.js`, then `node scripts/build-system-world.js --check` → `verified`.
- [ ] **Final regression** (PowerShell, all must pass):
  ```powershell
  Get-ChildItem scripts/test-system-world*.mjs | ForEach-Object { node $_.FullName; if ($LASTEXITCODE) { throw $_.Name } }
  python assets/system-world/check_assets.py
  python assets/system-world/build_textures.py --check
  node scripts/build-system-world-v3.mjs --check
  node scripts/build-system-world.js --check
  go test ./ui -run '^TestDesktopSysWorld' -count=1
  $env:AURAGO_RUN_BROWSER_SMOKE='1'; $env:AURAGO_SYSTEM_WORLD_MATRIX='1'; $env:AURAGO_SYSTEM_WORLD_EXPANSION='1'; go test ./ui -run '^TestDesktopAuroraBrowser$' -count=1 -timeout 30m
  ```
  - The glob covers the new `windows` and `lamps` tests and phase 1's `ground`, `footing` and `street-models` tests.
  - The browser run enforces the **20 MiB first display** (`desktop_sysworld_expansion_browser_test.go`, `loadedBytes > 20*1024*1024` fails) and the new window checks.
- [ ] **Survey set:**
  - `node scripts/system-world-survey.mjs --out reports/system-world-survey/phase-2 --tier high` (all scenes);
  - `--tier medium --out reports/system-world-survey/phase-2-medium --only facade-agent-night,lobby-agent-night,proof-window-close`;
  - `--tier low --out reports/system-world-survey/phase-2-low --only facade-agent-night,skyline-night,proof-window-night`.
  - Report before/after pairs against `phase-2-before` for: every `facade-*` scene (day, evening, night), `lobby-agent-night`, `skyline-night`, `overview-night`, and `street-agent` (lamp cones by day).
  - 0 page errors in every run.
- [ ] **Report budgets:**
  - model directory: the `model directory N bytes` line of `build-system-world-v3.mjs --check`; expected about 1.5 MiB less than before phase 2 if the buildings add about 2.5 MiB;
  - first display: `loadedBytes` from the browser report `world2-exploration.json`, plus `stats.loadedBytes` of `overview-day` in the survey `report.json`; must be < 20 MiB;
  - whole app: `runtimeBytes` from `test-system-world-expansion.mjs`, < 80 MiB;
  - the v1 reduction: 3,998,504 bytes.
- [ ] Commit: `chore(system-world): rebuild renderer bundle, pin window markers and browser checks`.

**Acceptance:** every command passes, the survey sets are captured and compared, and the three budget figures are reported.

---

## Test and pin changes at a glance

| Pin | Old | New |
| --- | --- | --- |
| `test-system-world-surfaces.mjs` untextured set | includes `city.glass` | without `city.glass`; `windowPanes` = {`city.window`} |
| Same test, glass assertion | glass untouched | glass takes the maps; `city.window` untouched; v3 light parts fixtures |
| `KINDS` count (printed) | 14 | 15 |
| `test-system-world-v3.mjs` designs | `>= 6` | `=== 25`; ids = 6 street + 12 `supersededV1` + 7 furniture; 10 designs with panes; pane UV contract; `pack` has `-vtf` (or `-vtf`); v1 ids = drone, tram, rack |
| `build-system-world-v3.mjs` `PACK_FLAGS` | `-cc -kn -km -ke -kv` | `… -vtf` (manifest `pack` follows at Task 0) |
| `check_assets.py` v1 count | 15 | 3 (`model_bytes` 280,116, 9 GLBs) |
| `v1/manifest.json` | 15 designs | 3 designs, `total_model_bytes` 280116 |
| layout / hover / living colliders | v1 catalog | `kitCatalog(v1, v3)` |
| `TestDesktopSysWorldAppMarkers` | — | `sysworld-windows.js` entry; scene and atmosphere markers |
| City browser test | — | windows level 2 with ≥ 10 meshes and `kitModels.v3 ≥ 14` on high; level 0 on low |
| First display (`desktop_sysworld_expansion_browser_test.go`) | 20 MiB | unchanged limit, re-measured and reported |

---

## Risks and open questions

1. **gltfpack texture-coordinate quantisation without textures** (unverified).
   - gltfpack stores the UV dequantisation in `KHR_texture_transform` on texture slots, and `city.window` has none. Room coordinates may arrive normalised.
   - The pane check in `test-system-world-v3.mjs` catches it at Task 0. The fallbacks are `-vtf` (float UVs; existence in 1.3.0 unverified) or `-noq` (no quantisation, larger files).
   - Note also that every primitive of a part that holds panes carries `TEXCOORD_0` (zeros on non-pane faces). That is a small, well-compressing size cost.
2. **Derivative-based tangent frame.**
   - **Precision.** Room coordinates reach about 66, so fp32 varyings carry roughly 1–2 % per-quad jitter in cell size at under 1 m from the glass. This is invisible at the street distances the colliders allow.
   - **Degenerate cases.** Grazing views and implausible sizes already fall back to the analytic frame (horizontal tangent, rows up, 1.9 × 3.05 m rooms).
   - **If a GPU or driver mis-computes the derivatives** (blocky or swimming rooms in the survey on some machine), switch to plan B, contained in `sysworld-windows.js`:
     - `adopt()` computes per pane vertex an attribute `winAxisU` (object-space dP/du) and `winStorey` (|dP/dv|) from the triangles, using the same maths as the v3 test;
     - the vertex shader transforms them with `mat3(winObject)`;
     - the fragment stage drops `dFdx`/`dFdy` and keeps only `fwidth` for anti-aliasing.
     Cost: about 16 bytes per pane vertex (few vertices) and a cheaper fragment shader.
3. **Shader cost.**
   - At high, the fragment adds roughly 150 ALU operations and 4–5 hashes per pane pixel. At street level facades fill 40–60 % of the screen.
   - The auto tier drops to medium/low when frames exceed 28 ms. Medium removes the life layer and low is flat.
   - If the browser run or a manual check shows a regression on integrated GPUs, move the furniture layer to level 2 (one `#if` change) before touching the art.
4. **Brightness and bloom.** The room glow (0.6 → 1.5), the ceiling panel (×2.5) and the screens (×1.8) are tuned by eye in Task 2. The bloom threshold is 1.06, so ceiling panels and screens bloom and walls do not. The only allowed knobs are the constants in `windowLighting`, the panel and sheen factors, and the palette.
5. **Collider and threshold drift.** v3 bounds feed the hover-pack upper solids and the lamp/planter boxes. The living and hover tests could move their progress or clearance figures. Task 3 measures the bounds deltas first and escalates breaches instead of loosening tests.
6. **Aviation lights** take the height of the v3 tower LOD 2 bounding box, including the roof masts (up to about 7.9 m above the roof in variant 1). Judge this in `skyline-night`. If they float, compute roofs from `bounds.max.y` minus the mast, which needs a manifest field from the main session.
7. **v3 geometry against the v2 sky deck and lift** (spire crown at 72.2 m, lift on the east side). Verified only by the browser run, which rides the lift, and the survey. The main session states the tiers keep v1 heights.
8. **The generic precedence rule** makes any future v3 id equal to a v1 id override automatically. That is intended. The v3 `street-crossing` shares an id with the v1 design retired in phase 1, which is harmless.
9. **`build_city.py`** (currently modified in the main session's working tree) still generates the twelve retired v1 designs. The next v1 rebuild re-adds them and `check_assets.py` fails. This is main-session work, recorded in AGENTS.md (Task 6).
10. **The glass reflects `scene.environment`** (RoomEnvironment), not the animated sky. The sky sheen (`winSky`, Fresnel-weighted fog/horizon colour) approximates it. A sky-matched environment map is a phase 6 candidate.
11. **Low-tier switching** is frozen to honour "no animation". If you prefer rooms to keep switching slowly on low, delete the `#else float winEpoch=0.0;` branch (one line).
12. **Survey camera collisions.** Facade cameras sit on far pavements away from the lamp and planter rows. Tram stops and docks were not checked against each position. A non-zero `cameraMovedByCollision` in `report.json` means moving that camera by 1 m.
