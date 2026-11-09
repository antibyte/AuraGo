# System World Beautification — Phases 0 and 1 Implementation Plan

> **For agentic workers.** Subagents carry out this plan one task at a time, in order. Every step is a checkbox. A code-writing agent (Sonnet class) implements each task. A test-running agent then runs the commands listed under the task. There is one combined review per batch, covering Critical and Important findings only.
>
> - **Commit after every task** with a conventional message such as `feat(system-world): …`. The last line of the message is `Co-Authored-By: Claude <noreply@anthropic.com>`.
> - **CLAUDE.md rules:** run GitNexus `impact` (upstream) on every function you edit, and `detect_changes` before every commit. Use the MCP tools, or the CLI from the main checkout root. `groundHeight`, `createCity`, `createSociety` and `createWayfinder` are expected to report HIGH risk. Report the result and continue: this plan *is* the mitigation.
> - **Scripts:** write throwaway scripts with the Write tool into your session scratch directory, never into the repo. Bash heredocs mangle backslashes on this machine.
> - **Paths:** all paths are relative to the worktree `C:\Users\Andi\Documents\repo\AuraGo\.worktrees\sysworld-beauty`. Never Glob or Grep the main repo root.

**Goal**
- **Phase 0:** a v3 asset pipeline, renderer support for meshopt + vertex-AO kits, raised budgets, and line-ending-safe provenance.
- **Phase 1:**
  - one continuous street height profile (flush pavements, 14 cm splayed kerbs, dropped kerbs at zebras, rounded corners, end caps), served by new v3 street modules;
  - pedestrian routes on pavements and zebras only;
  - residents whose height is damped and whose feet stay on the ground;
  - contact shadows;
  - a companion and a visitor camera that follow the ground.

**Architecture**
- A new pure module, `sysworld-ground.js`, owns the kerb constants, the street module and zebra layout, the analytic `streetProfile`, the plot slabs and the pedestrian graph. `groundHeight`, the wayfinder, the renderer and all tests read from it.
- A new pure module, `sysworld-footing.js`, owns the critically damped spring, the two-bone leg solve, the stance weight and the per-resident foot adaptation. It runs once per fixed step, after the mixer update and after traffic has committed the final pose.
- v3 street GLBs are loaded through a second, meshopt-enabled kit catalog in `sysworld-scene.js`. They are instanced exactly like the v1 placements.

**Tech Stack**
- Three.js 0.186.1 (isolated ESM bundle via esbuild 0.28.2) and `MeshoptDecoder` from `three/addons`.
- `gltfpack@1.3.0` through `npx`, and `gltf-validator` 2.0.0-dev.3.10.
- Node ESM contract tests with `node:assert`, the Python 3 `check_assets.py`, and Go + rod browser tests.

---

## What I verified, and what I could not

**Already present in the worktree**
- `.gitattributes` line 45 already has `assets/system-world/*.py text eol=lf`.
- The `assets/system-world/README.md` and `AGENTS.md` and `ui/3d/system-world/AGENTS.md` already state the 40/20/80 MiB budgets.
- `scripts/build-system-world-v3.mjs` and `scripts/test-system-world-v3.mjs` already exist, presumably written by the main session. Task 2 hardens the script and does not rewrite it.
- `kit3.py`, `street_assets3.py` and `build_world3.py` exist.

**Not done yet**
- LF hashing in `build_expansion.py` and `test-system-world-expansion.mjs`.
- Every numeric budget in code (12/48 MiB is still in five files).
- All renderer v3 support.
- Everything in Phase 1.

**Facts the plan relies on**
- Resident rig node names, verified in `v2/robot-courier.lod0.glb`: `structure`, `leg_l`, `shin_l`, `foot_l`, `leg_r`, `shin_r`, `foot_r`.
- The legs hang from **`structure`**, not from `torso` (`robot_assets.py` RIG: `leg_x ← structure`).
- The knee is a pure local-X hinge (`shin = RX(flex)`).
- Sole contacts: `HEEL=(-.085,-.13)` and `TOE=(.27,-.13)` (forward, up) in the foot frame. The ankle sits at 0.13 m.
- Residents are scaled 1.15. Clips: `idle`, `walk`, `turn`, `greet`, `work`, `carry`. Every clip keys every leg joint.
- The v2 `station` bounds are x ±4.5, z ±2. I used this as a proxy for its collider when re-siting the south dock.

**Could not verify by reading**
- Whether the new files above are committed.
- That `gltfpack@1.3.0` resolves on npm. The existing script pins it and contains a Windows exit-crash workaround, which suggests it has been run.
- That the current v2 `sources` hashes still pass. They do only if no authoring source changed since the v2 build.
- The real station collider extents.
- That robot part nodes have identity rest rotations. This is guarded at runtime and checked by a test (Task 8).
- The pedestrian-graph build time. My estimate is 0.1–0.3 s once per page.
- Whether resident connectivity on the new graph keeps `test-system-world-living.mjs` above its progress thresholds.

## Deviations from the brief

- **Continuity check.** The spec says "no step > 2 cm per 5 cm of travel". The standard kerb is a 45° splay (14 cm over 14 cm), so that is impossible as written. The test asserts **no step > 2 cm per 1 cm**. The steepest legitimate gradient is about 1.66, in the dropped-kerb flares. This proves there is no vertical step anywhere.
- **Two hover docks move.** `south` (−11, 64.3) and `east` (72.3, 21) sit on zebra crossings: their standing spot falls on a dropped-kerb ramp, which fails the hover test's "uneven ground" assertion and blocks the zebra's pavement node.
  - New positions: `south` → (−7.8, 64.3), `east` → (72.3, 42).
  - The browser test walk to the south dock changes with it (Task 6).
- **Two planters move.** The planters at x = −27 and x = 59 stand on the zebras at −26 and 59. Every planter now sits on the block-centre side of its lamp: −51, −35, 35, 51.
- **`streetAt`** (used by the HUD mirror and the tests) is left unchanged, so it still covers z up to 80 for the xs lines. Only the new profile ends those streets at the quay.

---

## File map

| File | Status | Responsibility |
| --- | --- | --- |
| `ui/js/desktop/apps/sysworld-ground.js` | new | Kerb constants, street lines, segments, module list, zebras, `streetProfile`, `plotSlabs`, `island`, `pedestrianGraph`, `carriagewayFree` (pure) |
| `ui/js/desktop/apps/sysworld-footing.js` | new | `springStep`, `stanceWeight`, `solveLeg`, `findRig`, `soleLift`, `adaptFeet` (pure maths on Object3Ds) |
| `ui/js/desktop/apps/sysworld-contact.js` | new | Instanced blob contact shadows (one draw call) |
| `scripts/test-system-world-ground.mjs` | new | Profile, continuity, module assembly, slabs, pedestrian graph, wayfinder routes |
| `scripts/test-system-world-footing.mjs` | new | Spring, stance weight, leg solve, kerb/step-down/ramp/random feet, rig detection, contact pool |
| `scripts/test-system-world-street-models.mjs` | new | Packed v3 street GLBs, rasterised from above, against `streetProfile` within 1.5 cm |
| `ui/js/desktop/apps/sysworld-layout.js` | edit | `streets.quayZ`; new `surfaces`; drop `streetHeight`; re-site 2 docks |
| `ui/js/desktop/apps/sysworld-exploration.js` | edit | `groundHeight` built on `streetProfile` |
| `ui/js/desktop/apps/sysworld-places.js` | edit | Neon gate y from `surfaces`; `createWayfinder` on the pedestrian graph |
| `ui/js/desktop/apps/sysworld-society.js` | edit | Rig, spring, pitch, feet in `settle`; contact shadows; `rigs` stat |
| `ui/js/desktop/apps/sysworld-experience.js` | edit | Rails/tram-exit heights derived; `settle`; shadows pass-through |
| `ui/js/desktop/apps/sysworld-scene.js` | edit | v3 kit catalog + meshopt loader + material key; `kitPlacements`; derived heights; ground meshes; contact shadows; settle call; eye spring |
| `ui/js/desktop/apps/sysworld-surfaces.js` | edit | 5 new surface kinds |
| `ui/js/desktop/apps/sysworld-atmosphere.js` | edit | Lamp cones follow the lamp height |
| `ui/js/desktop/apps/sysworld-life.js` | edit | Ground light + contact shadow on the real ground |
| `ui/js/desktop/apps/sysworld-companion.js` | edit | New one-level loop, ground height + pitch, shadow |
| `scripts/build-system-world-v3.mjs` | edit | `--check` validates committed files; triangle preservation |
| `assets/system-world/build_expansion.py` | edit | LF-normalised source hashes; budgets 40/80 MiB |
| `ui/3d/system-world/v2/manifest.json` | edit | Patched `build_expansion.py` hash and budget fields (textual) |
| `assets/system-world/check_assets.py` | edit | 40 MiB; v1 count 17 → 15 (Task 12) |
| `ui/3d/system-world/v1/manifest.json` + 6 GLBs | edit/delete | Remove `street-tile` and `street-crossing` (Task 12) |
| `scripts/test-system-world-{expansion,companion,surfaces,traffic,layout?,hover,living}.mjs` | edit | Budgets, LF hashes, new kinds, tram y, dock rules, settle/rigs |
| `scripts/build-system-world-spot.mjs` | edit | 40 MiB |
| `ui/desktop_sysworld_expansion_browser_test.go` | edit | 20 MiB first display; south-dock walk |
| `ui/desktop_sysworld_test.go` | edit | Markers for the 3 new modules (Task 15) |
| `assets/system-world/AGENTS.md`, `README.md`, `ui/3d/system-world/AGENTS.md` | edit | Kit v3, street contract, v1 removal |
| `ui/js/vendor/system-world/{city.esm.js,manifest.json}` | regenerate | Task 15 |

**Invariants every task must keep**
- `sysworld-scene.js` must keep the exact text `  return {\n    districts, canvas, update, focus(id)`, and `sysworld-society.js` must keep the line `    add,patrol,update,suspend,\n`. `scripts/system-world-survey.mjs` patches both and fails on any change. Add new return members on their own lines.
- Keep every marker string listed for `sysworld-scene.js`, `sysworld-experience.js` and `sysworld-layout.js` in `TestDesktopSysWorldAppMarkers` (`ui/desktop_sysworld_test.go`), for example `surfaces.prepare(n)`, `InstancedMesh`, `assetURL`, `Math.hypot(forward, right)` and `export const hoverPads`.

**Go test pins**
- No Go test enumerates `sysworld-*.js` files. `desktop_pixel_app_test.go` and `desktop_galaxa_modes_test.go` filter by other prefixes. New modules therefore need no registration. Task 15 adds marker entries for them in `ui/desktop_sysworld_test.go`.
- `ui/desktop_sysworld_expansion_browser_test.go` pins the first-display budget (line 22) and the south-dock walk (lines 152–155).

**Bundle**
- `node scripts/build-system-world.js --check` compares the shipped bundle with the sources. It stays red from Task 3 until Task 15 rebuilds the bundle; no Go test checks it.
- If a hook or CI requires `--check` per commit, run `node scripts/build-system-world.js` at the end of every runtime task instead.

## Order and parallelism

| Batch | Tasks | Notes |
| --- | --- | --- |
| A | 0 | Baseline survey. Must run before any visible change (Task 6 onwards) |
| B | 1, 2, 3, 4, 5 | **Parallel**, disjoint files |
| C | 6 | Needs 1 (Go file), 3 (scene), 4 (layout `quayZ`) |
| D | 7 ∥ 8 | 7 touches places.js + ground test; 8 touches society/experience/scene/living test |
| E | 9 → 10 → 11 | Sequential: all touch `sysworld-scene.js` |
| F | 12 → 13 | **Blocked until the main session delivers `ui/3d/system-world/v3` with the six street modules** |
| G | 14 → 15 | Docs, then markers, bundle, final regression, after-survey |

---

## Task 0 — Baseline survey (test agent, no code)

- [ ] Run `node scripts/system-world-survey.mjs --out reports/system-world-survey/baseline --tier high`.
  - Expected: one PNG per scene in `scripts/system-world-survey.scenes.json` (kerb-close, crossing, resident-feet-0/5/11 sequences, night-street, rain-crossing, and more).
- [ ] `git check-ignore reports/system-world-survey/baseline` must print the path. Do not commit pictures.

**Acceptance:** the baseline folder exists before Task 6 starts.

## Task 1 — LF-normalised provenance and raised budgets

**Files**
- `assets/system-world/build_expansion.py`: `build()` lines 421–430.
- `ui/3d/system-world/v2/manifest.json`: `sources`, `budget_bytes`, `model_budget_bytes`.
- `scripts/test-system-world-expansion.mjs`: lines 16, 71, 75.
- `assets/system-world/check_assets.py`: `check_robot()` line 109.
- `scripts/test-system-world-companion.mjs`: line 67.
- `scripts/build-system-world-spot.mjs`: line 95.
- `ui/desktop_sysworld_expansion_browser_test.go`: lines 22–23.
- `.gitattributes`: verify only.

**Steps**
- [ ] `.gitattributes` already contains `assets/system-world/*.py text eol=lf`. Run `git add --renormalize assets/system-world` and then `git status --short assets/system-world`. Expected: no content changes. If files appear, include them in this commit.
- [ ] Change `scripts/test-system-world-expansion.mjs` line 16 first. Expected: the test still passes on an LF checkout, so this is not a red step.

  Before:
  ```js
  for(const [name,hash]of Object.entries(manifest.sources))assert.equal(createHash('sha256').update(await fs.readFile('assets/system-world/'+name)).digest('hex'),hash,'Authoring provenance drift: '+name);
  ```
  After:
  ```js
  // Hashes cover LF-normalised bytes, so a CRLF checkout of the Python sources stays valid.
  const lf=data=>Buffer.from(data.toString('latin1').replaceAll('\r\n','\n'),'latin1');
  for(const [name,hash]of Object.entries(manifest.sources))assert.equal(createHash('sha256').update(lf(await fs.readFile('assets/system-world/'+name))).digest('hex'),hash,'Authoring provenance drift: '+name);
  ```
- [ ] Change the same file's lines 71 and 75:
  - `<12*1024*1024,'Complete model directory exceeds 12 MiB'` → `<40*1024*1024,'Complete model directory exceeds 40 MiB'`
  - `assert.ok(runtime<48*1024*1024);` → `assert.ok(runtime<80*1024*1024,'Whole app exceeds 80 MiB: '+runtime);`
- [ ] Change `build_expansion.py`.
  - Line 422: `'budget_bytes':48*1024*1024,'model_budget_bytes':12*1024*1024,` → `'budget_bytes':80*1024*1024,'model_budget_bytes':40*1024*1024,`
  - Line 423:

    Before:
    ```python
    'sources':{name:hashlib.sha256((ROOT/'assets/system-world'/name).read_bytes()).hexdigest() for name in [...]}}
    ```
    After (keep the list unchanged):
    ```python
    'sources':{name:hashlib.sha256((ROOT/'assets/system-world'/name).read_bytes().replace(b'\r\n',b'\n')).hexdigest() for name in [...]}}
    ```
  - Line 430: `assert total<=12*1024*1024,total` → `assert total<=40*1024*1024,total`.
- [ ] Patch the generated v2 manifest **textually**, because a JSON round trip in JS turns `0.0` into `0`. Write a throwaway Node script in your scratch directory that:
  1. reads `ui/3d/system-world/v2/manifest.json` as text;
  2. reads the old `sources["build_expansion.py"]` value via `JSON.parse`;
  3. computes the new value as sha256 over `assets/system-world/build_expansion.py` with `\r\n`→`\n` (latin1 round trip as above);
  4. `replace`s the old hash string with the new one (assert exactly one occurrence);
  5. replaces `"budget_bytes": 50331648` with `"budget_bytes": 83886080` and `"model_budget_bytes": 12582912` with `"model_budget_bytes": 41943040` (assert each occurs once);
  6. writes the file back with LF line endings.

  `git diff --stat` must show three changed lines in that manifest.
- [ ] Other budgets:
  - `check_assets.py:109`: `assert total < 12 * 1024 * 1024` → `assert total < 40 * 1024 * 1024`.
  - `test-system-world-companion.mjs:67`: `12 * 1024 * 1024, 'Complete model directory stays below 12 MiB'` → `40 * 1024 * 1024, 'Complete model directory stays below 40 MiB'`.
  - `build-system-world-spot.mjs:95`: `12 * 1024 * 1024` → `40 * 1024 * 1024`, and the message `'... exceeds the 40 MiB ceiling: '`.
- [ ] `ui/desktop_sysworld_expansion_browser_test.go:22-23`: `size > 12*1024*1024` → `size > 20*1024*1024`, and `"initial load exceeds 12 MiB"` → `"initial load exceeds 20 MiB"`.
- [ ] Run:
  - `node scripts/test-system-world-expansion.mjs` → JSON summary, exit 0.
  - `python assets/system-world/check_assets.py` → `"checks": "passed"` and `robot_checks: passed`.
  - `node scripts/test-system-world-companion.mjs` → exit 0.
  - `go test ./ui -run '^$' -count=1` → compiles.
  - If `disposable/system-world/boston_dynamics_spot-spot.glb` exists: `node scripts/build-system-world-spot.mjs --check`. Otherwise `node --check scripts/build-system-world-spot.mjs`.
- [ ] Commit: `chore(system-world): LF-normalised provenance and 40/20/80 MiB budgets`.

**Acceptance**
- Converting a source file to CRLF locally (do not commit it) still passes the expansion test. Optional to try.
- No remaining `12*1024*1024` or `48*1024*1024` in the listed files.

## Task 2 — Harden the existing v3 pack script

**Files**
- `scripts/build-system-world-v3.mjs`: the `--check` branch at lines 125–151, and `packOne` at lines 83–90.

**Steps**
- [ ] `npm view gltfpack@1.3.0 version` → prints `1.3.0`. If it fails, stop and report. Do not choose another version.
- [ ] Add a helper below `parseGlb`:
  ```js
  function triangleCount(doc) {
    let n = 0;
    for (const mesh of doc.meshes ?? []) for (const p of mesh.primitives)
      n += (p.indices !== undefined ? doc.accessors[p.indices].count : doc.accessors[p.attributes.POSITION].count) / 3;
    return n;
  }
  ```
- [ ] In `packOne`, after `checkContract(...)`:
  ```js
  const doc = parseGlb(bytes, lod.file), tris = triangleCount(doc);
  if (tris !== lod.triangles) fail(lod.file + ': gltfpack changed the triangle count (' + lod.triangles + ' -> ' + tris + '); no simplification is allowed');
  ```
  Replace the existing `checkContract(parseGlb(bytes, lod.file), asset, lod.file);` with `checkContract(doc, asset, lod.file);` placed before that check.
- [ ] In the `--check` loop, after the sha256 comparison:
  ```js
  const checked = await validator.validateBytes(new Uint8Array(data), { uri: lod.file, maxIssues: 50 });
  if (checked.issues.numErrors) fail(lod.file + ': Khronos validator: ' + JSON.stringify(checked.issues.messages));
  checkContract(parseGlb(data, lod.file), asset, lod.file);
  ```
- [ ] Run `node --check scripts/build-system-world-v3.mjs` → no output. If `disposable/system-world/v3-raw/manifest.raw.json` exists, also run `node scripts/build-system-world-v3.mjs` and then `--check`. Expected: `verified … reproduced N files`.
- [ ] If `git status` shows the script or `scripts/test-system-world-v3.mjs` as untracked, commit them together with this change.
- [ ] Commit: `build(system-world): v3 pack check validates committed GLBs and pins triangle counts`.

**Acceptance**
- `--check` alone (without the raw directory) validates every committed v3 GLB.
- A repack that changes triangles fails.

## Task 3 — Renderer support for kit v3 (works with an absent or empty v3 manifest)

**Files**
- `ui/js/desktop/apps/sysworld-scene.js`: imports; a new module-level `sharedMaterialKey` and `kitPlacements`; `createCity` after line 228 (the `catalog` line); `model()` lines 238–269; `rebuild()` lines 274–301.
- `ui/js/desktop/apps/sysworld-surfaces.js`: `KINDS` lines 12–23.
- `scripts/test-system-world-surfaces.mjs`.

**Steps**
- [ ] Extend the surfaces test first.
  - Change the import to also import `sharedMaterialKey, kitPlacements` from `../ui/js/desktop/apps/sysworld-scene.js`.
  - Change the kit loop to `for (const kit of ['v1', 'v2', 'v3'])` and skip a missing directory: `const files = await fs.readdir(...).catch(() => []);`.
  - Before `surfaces.dispose()` add:
    ```js
    // Kit v3 street materials: every new key is a projected surface, the marking wets like the road.
    for (const name of ['city.kerb', 'city.setts', 'city.iron', 'city.marking', 'city.tactile']) assert.ok(KINDS[name], 'Kit v3 kind missing: ' + name);
    assert.deepEqual(KINDS['city.marking'].puddle, KINDS['city.road'].puddle);
    assert.equal(KINDS['city.marking'].top, 'asphalt');
    // Vertex-coloured (baked AO) materials keep the surface injection and never share with plain ones.
    {
      const ao = new THREE.MeshStandardMaterial({ name: 'city.kerb', vertexColors: true });
      surfaces.apply(ao);
      const shader = { uniforms: {}, vertexShader: THREE.ShaderLib.standard.vertexShader, fragmentShader: THREE.ShaderLib.standard.fragmentShader };
      ao.onBeforeCompile(shader);
      assert.ok(shader.fragmentShader.includes('diffuseColor.rgb*=swAlbedoF'), 'AO material keeps the surface maps');
      assert.notEqual(sharedMaterialKey(ao), sharedMaterialKey(new THREE.MeshStandardMaterial({ name: 'city.kerb' })), 'AO and plain materials never share');
    }
    assert.ok(Array.isArray(kitPlacements));
    ```
- [ ] Run `node scripts/test-system-world-surfaces.mjs`. Expected: FAIL (export missing).
- [ ] `sysworld-surfaces.js`: add to `KINDS`, after `'city.leaf'`:
  ```js
  // Kit v3 street materials (baked AO in COLOR_0 multiplies the albedo).
  'city.kerb': {top: 'concrete', side: 'concrete', albedo: .45, rough: .55, bump: .01, wet: 1},
  'city.setts': {top: 'pavers', side: 'concrete', albedo: .5, rough: .6, bump: .014, wet: 1, topScale: 2, puddle: [.523, .516]},
  'city.iron': {top: 'panels', side: 'panels', albedo: .25, rough: .45, bump: .008, wet: .9},
  'city.marking': {top: 'asphalt', side: 'concrete', albedo: .25, rough: .5, bump: .006, wet: 1, puddle: [.447, .442]},
  'city.tactile': {top: 'concrete', side: 'concrete', albedo: .4, rough: .6, bump: .02, wet: 1},
  ```
- [ ] `sysworld-scene.js` imports: add `import { MeshoptDecoder } from 'three/addons/libs/meshopt_decoder.module.js';`.
- [ ] `sysworld-scene.js`, just above `export const placements = [];` (line 70):
  ```js
  // Shared kit materials are keyed by name plus vertex colours: a v3 material with baked AO
  // (COLOR_0) must never be reused by a v1/v2 mesh without that attribute.
  export function sharedMaterialKey(material) { return material.name + (material.vertexColors ? '|vertex-colours' : ''); }
  // Kit v3 placements, instanced like `placements` but loaded from ui/3d/system-world/v3/.
  export const kitPlacements = [];
  ```
- [ ] After `const catalog = new Map(manifest.assets.map(a => [a.id, a]));` (line 228):
  ```js
  // Kit v3 is optional until its designs ship: an absent or empty manifest leaves the catalog empty.
  const catalog3 = new Map();
  try {
    const response = await fetch(options.resourceURL('/3d/system-world/v3/manifest.json'), { signal: abort.signal });
    const kit = response.ok ? await response.json() : null;
    if (kit?.schema_version === 3 && Array.isArray(kit.assets)) for (const a of kit.assets) catalog3.set(a.id, a);
  } catch (error) { if (abort.signal.aborted) { dispose(); throw error; } }
  ```
- [ ] Replace `model()` (lines 238–269) completely:
  ```js
  const kits = {
    v1: { catalog, url: file => options.assetURL(file), loader: () => new GLTFLoader() },
    v3: { catalog: catalog3, url: file => options.resourceURL('/3d/system-world/v3/' + file), loader: () => new GLTFLoader().setMeshoptDecoder(MeshoptDecoder) },
  };
  async function model(id, lod, kit = 'v1') {
    const key = kit + ':' + id + ':' + lod;
    if (!cache.has(key)) cache.set(key, (async () => {
      const kitSource = kits[kit], item = kitSource.catalog.get(id)?.lods.find(l => l.level === lod);
      if (!item || !/^[a-z0-9-]+\.lod[0-2]\.glb$/.test(item.file)) throw Error('Invalid city asset');
      const controller = new AbortController(); requests.add(controller);
      const timer = setTimeout(() => controller.abort(), 15000);
      try {
        const response = await fetch(kitSource.url(item.file), { signal: controller.signal });
        if (!response.ok) throw Error('City asset unavailable');
        const bytes = await response.arrayBuffer();
        if (disposed) throw Error('Disposed');
        if (kit === 'v3') await MeshoptDecoder.ready;
        const gltf = await kitSource.loader().parseAsync(bytes, '');
        if (disposed) { gltf.scene.traverse(n => { if(n.isMesh) { n.geometry.dispose(); n.material.dispose(); } }); throw Error('Disposed'); }
        loadBytes += bytes.byteLength; reportProgress();
        gltf.scene.traverse(n => {
          if (!n.isMesh) return;
          geoSet.add(n.geometry); n.castShadow = true; n.receiveShadow = true;
          const source = Array.isArray(n.material) ? n.material : [n.material];
          const shared = source.map(m => {
            const name = sharedMaterialKey(m);
            if (materials.has(name)) { const original = m; m = materials.get(name); original.dispose(); }
            else { materials.set(name, m); matSet.add(m); }
            return m;
          });
          n.material = Array.isArray(n.material) ? shared : shared[0];
          surfaces.prepare(n);
        });
        return gltf.scene;
      } finally { clearTimeout(timer); requests.delete(controller); }
    })().catch(e => { cache.delete(key); throw e; }));
    return cache.get(key);
  }
  ```
- [ ] Change `rebuild()`.
  - In the `size` and `expectedBytes` lines, replace `cache.has(id + ':' + lod)` with `cache.has('v1:' + id + ':' + lod)` and `cache.has(id + ':2')` with `cache.has('v1:' + id + ':2')`.
  - After `const ids = …` add `const kitIds = [...new Set(kitPlacements.map(p => p.asset))];`.
  - Append to `expectedBytes`: `+ kitIds.reduce((sum, id) => sum + (cache.has('v3:' + id + ':' + lod) ? 0 : (catalog3.get(id)?.lods.find(l => l.level === lod)?.bytes || 0)), 0)`.
  - After the `distant` line: `const kit = new Map(await Promise.all(kitIds.map(async id => [id, await model(id, lod, 'v3')])));`.
  - Replace the `for (const p of placements) { … }` loop with a helper and two loops:
    ```js
    const add = (p, template) => {
      template.updateMatrixWorld(true);
      const matrix = new THREE.Matrix4().compose(new THREE.Vector3(p.x, p.y, p.z),
        new THREE.Quaternion().setFromAxisAngle(new THREE.Vector3(0, 1, 0), p.angle), new THREE.Vector3(...p.scale));
      template.traverse(n => {
        if (!n.isMesh) return;
        if (!batches.has(n.uuid)) batches.set(n.uuid, { node: n, matrices: [] });
        batches.get(n.uuid).matrices.push(new THREE.Matrix4().multiplyMatrices(matrix, n.matrixWorld));
      });
    };
    for (const p of placements) add(p, (p.z < -100 ? distant : loaded).get(p.asset));
    for (const p of kitPlacements) add(p, kit.get(p.asset));
    ```
- [ ] Run:
  - `node scripts/test-system-world-surfaces.mjs` → PASS, reporting 14 kinds.
  - `node scripts/test-system-world-layout.mjs` → PASS (it imports the scene module).
  - `node -e "import('esbuild').then(e=>e.build({entryPoints:['ui/js/desktop/apps/sysworld-scene.js'],bundle:true,format:'esm',write:false,logLevel:'error'})).then(()=>console.log('ok'))"` → `ok`.
- [ ] Commit: `feat(system-world): kit v3 loader with meshopt, AO-safe material sharing and street surface kinds`.

**Acceptance**
- The city loads unchanged when `ui/3d/system-world/v3/manifest.json` is absent (a 404 is ignored).
- v3 assets load through `MeshoptDecoder`.
- Material sharing distinguishes vertex colours.

## Task 4 — `sysworld-ground.js` and its test

**Files**
- `ui/js/desktop/apps/sysworld-layout.js` line 2: add `quayZ:74` to `streets`.
- New `ui/js/desktop/apps/sysworld-ground.js`.
- New `scripts/test-system-world-ground.mjs`.

**Steps**
- [ ] Layout line 2 → `export const streets = {xs:[-67,-18,18,67],zs:[-77,-32,13,59],halfWidth:6,laneHalfWidth:3.3,minX:-85,maxX:85,minZ:-94,maxZ:80,quayZ:74};`.
- [ ] Write the test below, run `node scripts/test-system-world-ground.mjs`, and see it FAIL on module-not-found.
- [ ] Write the module below.
- [ ] Run again → PASS. It prints counts and the graph build time. Total runtime should be under about 20 s; if it is far above that, report it.
- [ ] Run `node scripts/test-system-world-layout.mjs` → PASS. The HUD mirror is unaffected.
- [ ] Commit: `feat(system-world): pure street ground contract (profile, modules, zebras, slabs, pedestrian graph)`.

`ui/js/desktop/apps/sysworld-ground.js` (complete):
```js
// Pure street ground contract, shared by the renderer, the residents and the tests. Metres, Y up.
// One continuous height profile describes every street: carriageway, splayed kerb, dropped kerbs and
// flares at the zebra crossings, rounded corner kerbs and the end caps at the island and quay edges.
// The kit v3 street modules are authored to it (scripts/test-system-world-street-models.mjs).
import {streets,surfaces} from './sysworld-layout.js';

export const kerb={line:3.30,run:.14,top:.14,rise:.14,droppedRun:1.2,opening:1.2,flare:.8,zebraOffset:8,cornerRadius:1.2,endDepth:.5,pavementLine:4.75};
export const moduleLength=4,fillLength=1,maxLink=48;
// The island body under the plot slabs; its top only shows through hairline seams between modules.
export const island={top:-.2,bottom:-3};
// Pedestrian links keep MARGIN beyond every kerb top. d - run changes by at most 2.33 m per metre
// (flare 1.33 + offset 1), so with a sample every SAMPLE metres no link can touch a kerb or a
// carriageway between two samples (2.33 * SAMPLE / 2 < MARGIN).
const MARGIN=.15,SAMPLE=.1;
const clamp01=v=>v<0?0:v>1?1:v;

// `axis` is the world axis a street runs along: the zs lines run along X, the xs lines along Z.
// Streets along Z end at the quay edge; the quay strip stays continuous across them.
const LINES=[
  ...streets.zs.map(line=>({axis:'x',line,from:streets.minX,to:streets.maxX,crossings:streets.xs})),
  ...streets.xs.map(line=>({axis:'z',line,from:streets.minZ,to:streets.quayZ,crossings:streets.zs})),
];
export function streetLines(){return LINES.map(l=>({...l,crossings:[...l.crossings]}));}

// A zebra on every arm of every intersection, centred kerb.zebraOffset from the intersection centre.
const ZEBRAS=[];
for(const l of LINES)for(const c of l.crossings)for(const sign of[-1,1]){
  const at=c+sign*kerb.zebraOffset;
  if(at>=l.from&&at<=l.to)ZEBRAS.push({axis:l.axis,line:l.line,at,x:l.axis==='x'?at:l.line,z:l.axis==='x'?l.line:at});
}
export function zebras(){return ZEBRAS.map(z=>({...z}));}

// Kerb run at distance s along the street from the nearest zebra centre: dropped inside the
// opening, a linear flare outside it, the standard splay beyond.
export function rampRun(s){
  if(s<=kerb.opening)return kerb.droppedRun;
  if(s>=kerb.opening+kerb.flare)return kerb.run;
  return kerb.droppedRun+(kerb.run-kerb.droppedRun)*(s-kerb.opening)/kerb.flare;
}

// null off-street, else {height, kind: 'road'|'kerb'|'pavement', d, run}; d is the distance beyond
// the kerb line (negative on the carriageway), run the local kerb run.
export function streetProfile(x,z){
  const {xs,zs,halfWidth,minX,maxX,minZ,quayZ}=streets;
  let nx=Infinity,nz=Infinity;
  if(z>=minZ&&z<=quayZ)for(const v of xs)nx=Math.min(nx,Math.abs(x-v));
  if(x>=minX&&x<=maxX)for(const v of zs)nz=Math.min(nz,Math.abs(z-v));
  const onX=nx<=halfWidth,onZ=nz<=halfWidth;   // near an xs line (street along Z) / a zs line (street along X)
  if(!onX&&!onZ)return null;
  const road=surfaces.road;
  let d,run=kerb.run;
  if(onX&&onZ){
    if(nx<=kerb.line||nz<=kerb.line)d=-1;
    else{const qx=nx-kerb.line,qz=nz-kerb.line,r=kerb.cornerRadius;d=qx<r&&qz<r?r-Math.hypot(r-qx,r-qz):Math.min(qx,qz);}
  }else{
    const along=onZ?x:z,crossings=onZ?xs:zs;let s=Infinity;
    for(const c of crossings)s=Math.min(s,Math.abs(along-c-kerb.zebraOffset),Math.abs(along-c+kerb.zebraOffset));
    d=(onZ?nz:nx)-kerb.line;run=rampRun(s);
  }
  let height=road+kerb.rise*clamp01(d/run),kind=d<=0?'road':d>=run?'pavement':'kerb';
  // End caps: a kerb block across the carriageway, top at pavement level, inner face splayed.
  const e=Math.min(onX?z-minZ:Infinity,onX?quayZ-z:Infinity,onZ?x-minX:Infinity,onZ?maxX-x:Infinity),endRun=kerb.endDepth+kerb.run;
  if(e<endRun){const cap=road+kerb.rise*clamp01((endRun-e)/kerb.run);if(cap>height){height=cap;kind='kerb';}}
  return {height,kind,d,run};
}

// Segments between stops (an intersection square edge or a street end). All lengths are whole metres.
export function streetSegments(){
  const out=[];
  for(const l of LINES){
    const stops=[{at:l.from,kind:'end'},...l.crossings.map(at=>({at,kind:'crossing'})),{at:l.to,kind:'end'}];
    for(let i=0;i+1<stops.length;i++){
      const a=stops[i],b=stops[i+1];
      out.push({axis:l.axis,line:l.line,start:a.kind,end:b.kind,
        from:a.kind==='crossing'?a.at+streets.halfWidth:a.at,to:b.kind==='crossing'?b.at-streets.halfWidth:b.at});
    }
  }
  return out;
}

const ALONG={x:0,z:Math.PI/2};
function piece(id,axis,line,from,to){const at=(from+to)/2;return {id,axis,line,from,to,x:axis==='x'?at:line,z:axis==='x'?line:at,angle:ALONG[axis]};}
// Deterministic kit v3 placement list (y = 0; the GLBs carry the profile heights). Modules run along
// local +X and are rotated like the former v1 tiles; a street-end's local +X points out of the island.
export function streetModules(){
  const out=[];
  for(const x of streets.xs)for(const z of streets.zs)out.push({id:'street-crossing',axis:null,line:null,from:null,to:null,x,z,angle:0});
  for(const s of streetSegments()){
    let lo=s.from,hi=s.to;
    if(s.start==='crossing'){out.push(piece('street-zebra',s.axis,s.line,lo,lo+moduleLength));lo+=moduleLength;}
    if(s.end==='crossing'){out.push(piece('street-zebra',s.axis,s.line,hi-moduleLength,hi));hi-=moduleLength;}
    // 4 m modules from the intersection side, every third one a drain; whole-metre fills at the far end.
    const up=s.start==='crossing',count=Math.floor((hi-lo)/moduleLength+1e-9),fills=Math.round(hi-lo-count*moduleLength);
    for(let k=0;k<count;k++){
      const id=k%3===2?'street-drain':'street-plain';
      out.push(up?piece(id,s.axis,s.line,lo+k*moduleLength,lo+(k+1)*moduleLength):piece(id,s.axis,s.line,hi-(k+1)*moduleLength,hi-k*moduleLength));
    }
    for(let k=0;k<fills;k++){const at=up?hi-fills+k:lo+k;out.push(piece('street-fill',s.axis,s.line,at,at+fillLength));}
    // End caps overlay the outermost kerb.endDepth + kerb.run of the last module.
    for(const [kind,at,dir]of[[s.start,s.from,-1],[s.end,s.to,1]]){
      if(kind!=='end')continue;
      out.push({id:'street-end',axis:s.axis,line:s.line,from:at,to:at,x:s.axis==='x'?at:s.line,z:s.axis==='x'?s.line:at,
        angle:s.axis==='x'?(dir>0?0:Math.PI):(dir>0?-Math.PI/2:Math.PI/2)});
    }
  }
  return out;
}

// 25 plot slabs between the street bands plus the quay base, all with their top exactly at ground level.
export function plotSlabs(){
  const {xs,zs,halfWidth:w,minX,maxX,minZ,maxZ,quayZ}=streets;
  const cuts=(lines,lo,hi)=>{const out=[];let a=lo;for(const l of lines){out.push([a,l-w]);a=l+w;}out.push([a,hi]);return out;};
  const slabs=[];
  for(const [x0,x1]of cuts(xs,minX,maxX))for(const [z0,z1]of cuts(zs,minZ,quayZ))slabs.push({kind:'plot',x0,z0,x1,z1,top:surfaces.ground,bottom:island.top});
  slabs.push({kind:'quay',x0:minX,z0:quayZ,x1:maxX,z1:maxZ,top:surfaces.ground,bottom:island.top});
  return slabs;
}

const BAND=kerb.line+kerb.droppedRun+MARGIN;
function nearCarriageway(a,b){
  const x0=Math.min(a.x,b.x),x1=Math.max(a.x,b.x),z0=Math.min(a.z,b.z),z1=Math.max(a.z,b.z);
  return LINES.some(l=>l.axis==='x'
    ?z0<=l.line+BAND&&z1>=l.line-BAND&&x1>=l.from&&x0<=l.to
    :x0<=l.line+BAND&&x1>=l.line-BAND&&z1>=l.from&&z0<=l.to);
}
function onZebra(x,z){
  return ZEBRAS.some(zb=>zb.axis==='x'
    ?Math.abs(x-zb.at)<=kerb.opening&&Math.abs(z-zb.line)<=streets.halfWidth
    :Math.abs(z-zb.at)<=kerb.opening&&Math.abs(x-zb.line)<=streets.halfWidth);
}
// True when the straight line a-b never touches a carriageway or kerb face. `allowZebra` lets a start
// that stands on a zebra step off it (only that crossing corridor is exempt).
export function carriagewayFree(a,b,allowZebra=false){
  if(!nearCarriageway(a,b))return true;
  const n=Math.max(1,Math.ceil(Math.hypot(b.x-a.x,b.z-a.z)/SAMPLE));
  for(let i=0;i<=n;i++){
    const t=i/n,x=a.x+(b.x-a.x)*t,z=a.z+(b.z-a.z)*t,p=streetProfile(x,z);
    if(!p||(p.kind==='pavement'&&p.d-p.run>=MARGIN))continue;
    if(allowZebra&&onZebra(x,z))continue;
    return false;
  }
  return true;
}

// Pavement-centre nodes: one per intersection corner (listed first, (+,+) first, so node 0 starts a
// straight pavement run), two per zebra. Only zebra links cross a carriageway. Built once; frozen.
let graph=null;
export function pedestrianGraph(){
  if(graph)return graph;
  const p=kerb.pavementLine,nodes=[],links=[];
  for(const x of streets.xs)for(const z of streets.zs)for(const [sx,sz]of[[1,1],[1,-1],[-1,1],[-1,-1]])nodes.push({x:x+sx*p,z:z+sz*p,kind:'corner'});
  for(const zb of ZEBRAS){
    const a=nodes.length;
    for(const side of[-1,1])nodes.push(zb.axis==='x'?{x:zb.at,z:zb.line+side*p,kind:'zebra'}:{x:zb.line+side*p,z:zb.at,kind:'zebra'});
    nodes[a].pair=a+1;nodes[a+1].pair=a;
    links.push({a,b:a+1,zebra:true,length:2*p});
  }
  for(let i=0;i<nodes.length;i++)for(let j=i+1;j<nodes.length;j++){
    if(nodes[i].pair===j)continue;
    const length=Math.hypot(nodes[i].x-nodes[j].x,nodes[i].z-nodes[j].z);
    if(length<maxLink&&carriagewayFree(nodes[i],nodes[j]))links.push({a:i,b:j,zebra:false,length});
  }
  nodes.forEach(Object.freeze);links.forEach(Object.freeze);
  graph=Object.freeze({nodes:Object.freeze(nodes),links:Object.freeze(links)});
  return graph;
}
```

`scripts/test-system-world-ground.mjs` (complete; Task 7 appends one section and Task 12 adds two lines):
```js
// System World ground contract: street profile, continuity, module assembly, slabs and pedestrian graph.
import assert from 'node:assert/strict';
import {streets,surfaces} from '../ui/js/desktop/apps/sysworld-layout.js';
import {kerb,moduleLength,maxLink,island,rampRun,streetLines,streetSegments,streetModules,zebras,streetProfile,plotSlabs,pedestrianGraph,carriagewayFree} from '../ui/js/desktop/apps/sysworld-ground.js';

const R=surfaces.road,P=surfaces.road+kerb.rise,near=(a,b,e=1e-9)=>Math.abs(a-b)<=e;

// --- Cross-section on a straight far from any zebra (z = -32 street at x = -45) ---
{
  const at=o=>streetProfile(-45,-32+o);
  for(const o of[0,1.5,3,3.3,-3.3])assert.ok(near(at(o).height,R,1e-6)&&at(o).kind==='road','carriageway at '+o);
  assert.ok(near(at(3.37).height,R+kerb.rise/2,1e-6)&&at(3.37).kind==='kerb','splayed kerb halfway');
  for(const o of[3.45,3.58,4.75,5.99,-4])assert.ok(near(at(o).height,P,1e-6)&&at(o).kind==='pavement','pavement at '+o);
  assert.equal(streetProfile(-45,-32+6.01),null,'plots are off-street');
}
// --- Dropped kerb and flares at the zebra 8 m west of the x = -18 crossing (x = -26) ---
{
  const at=(s,o)=>streetProfile(-26-s,-32+o);   // s grows away from the intersection
  assert.ok(near(at(0,kerb.line+.6).height,R+kerb.rise/2,1e-6),'1.2 m ramp, halfway up');
  assert.ok(near(at(1.2,kerb.line+1.2).height,P,1e-6),'ramp top at offset 4.5');
  assert.equal(at(0,4.6).kind,'pavement');assert.equal(at(.5,3.9).kind,'kerb');
  assert.ok(near(rampRun(1.6),(kerb.droppedRun+kerb.run)/2,1e-12),'flare midpoint');
  assert.ok(near(rampRun(2),kerb.run,1e-12)&&near(rampRun(5),kerb.run,1e-12)&&near(rampRun(1.2),kerb.droppedRun,1e-12));
  assert.ok(near(at(2.5,kerb.line+.07).height,R+kerb.rise/2,1e-6),'standard kerb beyond the flare');
}
// --- Rounded corner kerb at the (-18, -32) intersection ---
{
  const r=kerb.cornerRadius,corner=(qx,qz)=>streetProfile(-18+kerb.line+qx,-32+kerb.line+qz);
  assert.equal(corner(0,0).kind,'road');assert.equal(corner(-.2,2).kind,'road','either carriageway is road');
  assert.ok(near(corner(r,r).height,P,1e-6),'corner pavement');
  const k=r-(r-.07)/Math.SQRT2;   // d = .07 on the diagonal
  assert.ok(near(corner(k,k).height,R+kerb.rise/2,1e-6),'rounded kerb halfway');
  assert.ok(near(corner(2,.07).height,R+kerb.rise/2,1e-6),'straight kerb beside the arc');
}
// --- End caps at the island edge and at the quay edge ---
{
  for(const [x,z,dir]of[[-67,streets.minZ,1],[18,streets.quayZ,-1]]){
    const at=e=>streetProfile(x,z+dir*e);
    assert.ok(near(at(.2).height,P,1e-6)&&at(.2).kind==='kerb','end block top');
    assert.ok(near(at(kerb.endDepth+kerb.run/2).height,R+kerb.rise/2,1e-6),'end block inner splay');
    assert.ok(near(at(1).height,R,1e-6),'carriageway beyond the cap');
  }
  assert.equal(streetProfile(18,streets.quayZ+.01),null,'the quay strip is continuous across the street');
  assert.ok(near(streetProfile(streets.maxX-.2,13).height,P,1e-6),'east island edge');
  assert.equal(streetProfile(-62,streets.minZ+.2).kind,'pavement','pavement at the edge stays pavement');
}
// --- Continuity: no step above 2 cm per 1 cm of travel anywhere on the streets ---
{
  let worst=0,where=null;
  const walk=(x0,z0,x1,z1)=>{const n=Math.ceil(Math.hypot(x1-x0,z1-z0)/.01);let prev=null;
    for(let i=0;i<=n;i++){const x=x0+(x1-x0)*i/n,z=z0+(z1-z0)*i/n,p=streetProfile(x,z);
      if(p&&prev){const step=Math.abs(p.height-prev.height);if(step>worst){worst=step;where=[x,z];}}prev=p;}};
  for(const l of streetLines()){
    for(let a=l.from;a<=l.to;a+=.5)l.axis==='x'?walk(a,l.line-6,a,l.line+6):walk(l.line-6,a,l.line+6,a);
    for(const o of[0,3.3,3.37,3.44,3.9,4.5,4.75,5.5,-3.37,-4.5])l.axis==='x'?walk(l.from,l.line+o,l.to,l.line+o):walk(l.line+o,l.from,l.line+o,l.to);
  }
  let seed=1;const rnd=()=>(seed=(Math.imul(seed,1664525)+1013904223)>>>0)/4294967296;
  const centres=[...zebras().map(z=>[z.x,z.z]),...streets.xs.flatMap(x=>streets.zs.map(z=>[x,z]))];
  for(let i=0;i<6000;i++){const [cx,cz]=centres[i%centres.length],x=cx+(rnd()-.5)*12,z=cz+(rnd()-.5)*12,a=rnd()*Math.PI*2;walk(x,z,x+Math.cos(a),z+Math.sin(a));}
  assert.ok(worst<=.02,'profile step '+worst+' m per cm at '+where);
}
// --- Module assembly: every segment covered end to end, no gaps or overlaps ---
{
  const modules=streetModules(),segs=streetSegments();
  assert.equal(modules.filter(m=>m.id==='street-crossing').length,streets.xs.length*streets.zs.length);
  assert.equal(modules.filter(m=>m.id==='street-end').length,2*(streets.xs.length+streets.zs.length));
  assert.equal(zebras().length,2*2*streets.xs.length*streets.zs.length);
  for(const s of segs){
    assert.ok(Number.isInteger(s.to-s.from)&&s.to>s.from,'whole-metre segment '+JSON.stringify(s));
    const own=modules.filter(m=>m.axis===s.axis&&m.line===s.line&&m.id!=='street-end'&&m.from>=s.from-1e-9&&m.to<=s.to+1e-9).sort((a,b)=>a.from-b.from);
    assert.ok(near(own[0].from,s.from)&&near(own.at(-1).to,s.to),'segment covered end to end: '+JSON.stringify(s));
    for(let i=1;i<own.length;i++)assert.ok(near(own[i].from,own[i-1].to),'gap or overlap in '+JSON.stringify(s));
    for(const m of own)assert.ok(near(m.to-m.from,m.id==='street-fill'?1:moduleLength),m.id+' length');
    if(s.start==='crossing')assert.equal(own[0].id,'street-zebra');
    if(s.end==='crossing')assert.equal(own.at(-1).id,'street-zebra');
    assert.ok(own.filter(m=>m.id==='street-fill').length<=3);
    const fours=own.filter(m=>m.id==='street-plain'||m.id==='street-drain'),ordered=s.start==='crossing'?fours:[...fours].reverse();
    ordered.forEach((m,k)=>assert.equal(m.id,k%3===2?'street-drain':'street-plain','every third 4 m module is a drain'));
    for(const m of own)if(m.id!=='street-crossing')assert.equal(m.angle,s.axis==='x'?0:Math.PI/2);
  }
  for(const z of zebras())assert.ok(modules.some(m=>m.id==='street-zebra'&&near(m.x,z.x)&&near(m.z,z.z)),'zebra module centred on every crossing');
  for(const m of modules.filter(m=>m.id==='street-end')){
    const ox=Math.cos(m.angle),oz=-Math.sin(m.angle);   // local +X in world
    assert.equal(streetProfile(m.x+ox*.3,m.z+oz*.3),null,'end cap points out of the street');
    assert.ok(streetProfile(m.x-ox*.3,m.z-oz*.3),'end cap sits on the street end');
  }
}
// --- Plot slabs and streets cover the island exactly once ---
{
  const slabs=plotSlabs();assert.equal(slabs.length,26);assert.equal(slabs.filter(s=>s.kind==='quay').length,1);
  for(const s of slabs)assert.ok(s.top===surfaces.ground&&s.bottom===island.top,'slab heights');
  for(let x=streets.minX+.25;x<streets.maxX;x+=.5)for(let z=streets.minZ+.25;z<streets.maxZ;z+=.5){
    const covered=slabs.filter(s=>x>s.x0&&x<s.x1&&z>s.z0&&z<s.z1).length+(streetProfile(x,z)?1:0);
    assert.equal(covered,1,`ground cover at ${x}, ${z}`);
  }
}
// --- Pedestrian graph: pavements and zebras only ---
{
  const t0=Date.now(),{nodes,links}=pedestrianGraph(),ms=Date.now()-t0;
  assert.equal(nodes.length,streets.xs.length*streets.zs.length*4+zebras().length*2);
  for(const n of nodes){const p=streetProfile(n.x,n.z);assert.ok(p&&p.kind==='pavement',`node off the pavement: ${n.x}, ${n.z}`);}
  const zebraLinks=links.filter(l=>l.zebra);assert.equal(zebraLinks.length,zebras().length);
  for(const l of zebraLinks){const a=nodes[l.a],b=nodes[l.b];assert.equal(streetProfile((a.x+b.x)/2,(a.z+b.z)/2).kind,'road','zebra links cross the carriageway');}
  for(const l of links){
    if(l.zebra)continue;assert.ok(l.length<maxLink);
    const a=nodes[l.a],b=nodes[l.b],n=Math.ceil(l.length/.02);
    for(let i=0;i<=n;i++){const p=streetProfile(a.x+(b.x-a.x)*i/n,a.z+(b.z-a.z)*i/n);assert.ok(!p||p.kind==='pavement',`link ${l.a}-${l.b} touches a ${p?.kind}`);}
  }
  const adjacency=nodes.map(()=>[]);for(const l of links){adjacency[l.a].push(l.b);adjacency[l.b].push(l.a);}
  const seen=new Set([0]),queue=[0];while(queue.length){const i=queue.pop();for(const j of adjacency[i])if(!seen.has(j)){seen.add(j);queue.push(j);}}
  assert.equal(seen.size,nodes.length,'every pavement node is reachable');
  const n0=nodes[0];assert.ok(carriagewayFree(n0,{x:n0.x,z:n0.z+14}),'node 0 starts a straight pavement run');
  assert.equal(carriagewayFree({x:-45,z:-37},{x:-45,z:-27}),false,'across a street');
  const zb=zebras()[0],side=zb.axis==='x'?{x:zb.at,z:zb.line+kerb.pavementLine}:{x:zb.line+kerb.pavementLine,z:zb.at};
  assert.equal(carriagewayFree({x:zb.x,z:zb.z},side),false);assert.ok(carriagewayFree({x:zb.x,z:zb.z},side,true),'a start on a zebra may step off it');
  console.log(`System World ground: profile, continuity, ${streetModules().length} modules, 26 slabs, ${nodes.length} nodes / ${links.length} links (graph ${ms} ms) verified.`);
}
```

**Acceptance:** all assertions pass; the module is pure (no `three` import); `streets.quayZ === 74`.

## Task 5 — `sysworld-footing.js` and its test

**Files:** new `ui/js/desktop/apps/sysworld-footing.js` and `scripts/test-system-world-footing.mjs`.

**Steps**
- [ ] Write the test, run `node scripts/test-system-world-footing.mjs` → FAIL (module missing).
- [ ] Write the module, run again → PASS.
- [ ] Commit: `feat(system-world): pure footing maths (damped spring, two-bone legs, stance weight, foot adaptation)`.

**How the rig is used** (it is practical; nothing had to be dropped)
- Each leg is a planar chain `leg_x` (hip) → `shin_x` (knee, local-X hinge) → `foot_x` (ankle).
- The solve bends the shin about the leg's world X axis, then swings the thigh about the hip so the ankle meets its target.
- The foot is set back to its **animated world orientation**, so a planted sole keeps the clip's roll. It is not tilted to the slope: the steepest slope a foot stands on is the dropped kerb at about 1:8.6, so the heel floats at most about 4 cm there. The 45° splay is only 14 cm wide.
- **Stance** is read from the animated sole height: the lower of heel and toe, relative to the body origin. Weight is 1 up to 2.5 cm, 0 from 7 cm, smoothstep between. Using the lower contact keeps the weight during the toe-roll.
- **Pelvis drop:** the whole node is lowered by `max(weight × float)` over both feet, at most 0.25 m. This is exactly a pelvis drop, because everything hangs under `structure`.
- **Lifting a foot:** every foot that would penetrate after the drop is lifted by knee bend, at most 0.35 m. Legs never stretch, so the solve can always reach.
- **Missing nodes:** `findRig` returns `null`, silently, when any of the seven nodes is missing, when the chain parenting is wrong, or when the rest-pose soles are not within 4 cm of the body origin. Then only the damped height and pitch apply.

`ui/js/desktop/apps/sysworld-footing.js` (complete):
```js
// Pure footing for the resident robots: a critically damped height spring, an analytic two-bone leg
// solve, a stance weight and the foot adaptation that keeps both soles on the ground profile.
// Three.js maths only; no scene state. A rig that is not complete disables the feet, never the walker.
import * as THREE from 'three';

// Rig of assets/system-world/robot_assets.py as glTF nodes: structure (pelvis) -> leg_x (hip) ->
// shin_x (knee, a pure local-X hinge) -> foot_x (ankle). Sole contact points in the foot frame from
// HEEL/TOE (forward, up) = (-.085, -.13) and (.27, -.13); glTF axes are (x, up, forward).
export const LEGS=[['leg_l','shin_l','foot_l'],['leg_r','shin_r','foot_r']];
export const SOLE=[[0,-.13,-.085],[0,-.13,.27]];

// Exact critically damped spring step; frame-rate independent. dt <= 0 (reduced motion), an unset
// state or a jump above `snap` (teleport, stairs) snaps.
export function springStep(state,target,dt,omega=14,snap=.6){
  if(!(dt>0)||!Number.isFinite(state.y)||Math.abs(target-state.y)>snap){state.y=target;state.v=0;return target;}
  const x=state.y-target,e=Math.exp(-omega*dt),t=(state.v+omega*x)*dt;
  state.v=(state.v-omega*t)*e;state.y=target+(x+t)*e;return state.y;
}

// 1 while the animated sole is on its ground plane (lift <= 2.5 cm), 0 in swing (lift >= 7 cm).
export function stanceWeight(lift,low=.025,high=.07){
  const t=Math.min(1,Math.max(0,(lift-low)/(high-low)));return 1-t*t*(3-2*t);
}

const _u=new THREE.Vector3(),_v=new THREE.Vector3(),_n=new THREE.Vector3(),_c=new THREE.Vector3(),_m=new THREE.Vector3(),_t=new THREE.Vector3();
const solved={hip:new THREE.Quaternion(),knee:new THREE.Quaternion()};
// World-space two-bone solve. Returns {hip, knee}: rotations to pre-multiply onto the thigh's and the
// shin's world orientation (knee first, then hip). The returned object is reused by the next call.
export function solveLeg(hip,knee,ankle,target,hinge){
  _u.subVectors(knee,hip);_v.subVectors(ankle,knee);
  const l1=_u.length(),l2=_v.length();
  if(l1<1e-6||l2<1e-6||hinge.lengthSq()<1e-12)return null;
  _n.copy(hinge).normalize();
  const reach=Math.min(l1+l2-1e-4,Math.max(Math.abs(l1-l2)+1e-4,target.distanceTo(hip)));
  const current=Math.atan2(_c.crossVectors(_u,_v).dot(_n),_u.dot(_v));   // signed knee bend about the hinge
  const inner=Math.acos(Math.max(-1,Math.min(1,(l1*l1+l2*l2-reach*reach)/(2*l1*l2))));
  const desired=(Math.PI-inner)*(current<0?-1:1);                          // straight legs bend forward
  solved.knee.setFromAxisAngle(_n,desired-current);
  _m.copy(_v).applyQuaternion(solved.knee).add(_u);_t.subVectors(target,hip);
  if(_m.lengthSq()<1e-12||_t.lengthSq()<1e-12)return null;
  solved.hip.setFromUnitVectors(_m.normalize(),_t.normalize());
  return solved;
}

const _a=new THREE.Vector3(),_b=new THREE.Vector3(),_hip=new THREE.Vector3(),_knee=new THREE.Vector3(),_ankle=new THREE.Vector3(),_target=new THREE.Vector3(),_hinge=new THREE.Vector3();
const _p=new THREE.Quaternion(),_q=new THREE.Quaternion(),_f=new THREE.Quaternion();
function contacts(foot){_a.fromArray(SOLE[0]).applyMatrix4(foot.matrixWorld);_b.fromArray(SOLE[1]).applyMatrix4(foot.matrixWorld);}
// Height of the lower sole contact above the body origin (matrices must be current).
export function soleLift(foot,root){contacts(foot);return Math.min(_a.y,_b.y)-root.matrixWorld.elements[13];}
function penetration(foot,ground){contacts(foot);return Math.max(ground(_a.x,_a.z)-_a.y,ground(_b.x,_b.z)-_b.y);}
function rotateWorld(node,q){node.parent.getWorldQuaternion(_p);node.quaternion.premultiply(_p).premultiply(q).premultiply(_p.invert());}

// {structure, legs:[{leg,shin,foot}]} or null when anything is missing or the rest soles do not
// stand on the body origin (then these contact points do not describe the model).
export function findRig(root){
  const structure=root.getObjectByName('structure');if(!structure)return null;
  const legs=[];
  for(const names of LEGS){
    const [leg,shin,foot]=names.map(n=>root.getObjectByName(n));
    if(!leg||!shin||!foot||shin.parent!==leg||foot.parent!==shin)return null;
    legs.push({leg,shin,foot});
  }
  root.updateMatrixWorld(true);
  for(const l of legs)if(Math.abs(soleLift(l.foot,root))>.04)return null;
  return {structure,legs};
}

// Runs after the mixer has posed the clip and the body has its final position for this step.
// Lowers the body until the lowest stance foot reaches the ground, then bends each knee so no
// sole stays below the ground. Writes root.position.y and the leg/shin/foot quaternions only.
export function adaptFeet(rig,root,ground,{maxDrop=.25,maxRaise=.35}={}){
  root.updateMatrixWorld(true);
  const feet=rig.legs.map(l=>({l,lift:soleLift(l.foot,root),pen:penetration(l.foot,ground)}));
  let drop=0;for(const f of feet)drop=Math.max(drop,stanceWeight(f.lift)*Math.max(0,-f.pen));
  drop=Math.min(drop,maxDrop);if(drop<1e-4)drop=0;
  if(drop){root.position.y-=drop;root.updateMatrixWorld(true);}
  const raised=[];
  for(const f of feet){
    const raise=Math.min(maxRaise,f.pen+drop);raised.push(Math.max(0,raise));
    if(raise<=5e-4)continue;
    const {leg,shin,foot}=f.l;
    leg.getWorldPosition(_hip);shin.getWorldPosition(_knee);foot.getWorldPosition(_ankle);
    _target.copy(_ankle);_target.y+=raise;_hinge.setFromMatrixColumn(leg.matrixWorld,0);
    const s=solveLeg(_hip,_knee,_ankle,_target,_hinge);if(!s)continue;
    foot.getWorldQuaternion(_f);
    rotateWorld(shin,s.knee);rotateWorld(leg,s.hip);leg.updateMatrixWorld(true);
    shin.getWorldQuaternion(_q);foot.quaternion.copy(_q.invert().multiply(_f));foot.updateMatrixWorld(true);
  }
  return {drop,raised};
}
```

`scripts/test-system-world-footing.mjs` (complete; Task 9 appends one block):
```js
// System World footing: spring response, stance weight, two-bone solve and foot adaptation on a
// synthetic rig with the resident proportions (robot_assets.py, scaled 1.15 like the runtime).
import assert from 'node:assert/strict';
import * as THREE from 'three';
import {springStep,stanceWeight,solveLeg,findRig,adaptFeet,soleLift,SOLE} from '../ui/js/desktop/apps/sysworld-footing.js';

// --- Spring ---
{
  const dt=1/60,s={y:0,v:0};let peak=0;
  for(let i=0;i<36;i++){springStep(s,.14,dt);peak=Math.max(peak,s.y);}
  assert.ok(peak<=.14+1e-12,'critically damped: no overshoot');assert.ok(Math.abs(s.y-.14)<.001,'settles within 0.6 s: '+s.y);
  const a={y:0,v:0},b={y:0,v:0};for(let i=0;i<6;i++)springStep(a,.14,dt);springStep(b,.14,.1);
  assert.ok(Math.abs(a.y-b.y)<1e-9&&Math.abs(a.v-b.v)<1e-9,'frame-rate independent');
  const mid={y:0,v:0};springStep(mid,.14,.05);assert.ok(mid.y>.01&&mid.y<.13,'eases instead of snapping: '+mid.y);
  const jump={y:0,v:0};springStep(jump,2,dt);assert.equal(jump.y,2,'teleports snap');
  const still={y:0,v:0};springStep(still,.14,0);assert.equal(still.y,.14,'reduced motion snaps');
  const unset={y:NaN,v:0};springStep(unset,.3,dt);assert.equal(unset.y,.3,'an unset spring starts at its target');
}
// --- Stance weight ---
assert.equal(stanceWeight(0),1);assert.equal(stanceWeight(.01),1);assert.equal(stanceWeight(.1),0);
assert.ok(stanceWeight(.04)>stanceWeight(.05)&&stanceWeight(.05)>stanceWeight(.06),'monotonic');
// --- Two-bone solve ---
{
  const hip=new THREE.Vector3(0,1,0),knee=new THREE.Vector3(0,.5,0),ankle=new THREE.Vector3(0,0,0),target=new THREE.Vector3(0,.14,0);
  const r=solveLeg(hip,knee,ankle,target,new THREE.Vector3(1,0,0));assert.ok(r);
  const u=knee.clone().sub(hip),v=ankle.clone().sub(knee);
  const k2=hip.clone().add(u.clone().applyQuaternion(r.hip)),a2=k2.clone().add(v.clone().applyQuaternion(r.knee).applyQuaternion(r.hip));
  assert.ok(a2.distanceTo(target)<1e-6,'ankle reaches the target');assert.ok(k2.z>.05,'the knee bends forward (+Z)');
  assert.ok(Math.abs(k2.distanceTo(hip)-.5)<1e-9&&Math.abs(a2.distanceTo(k2)-.5)<1e-9,'bone lengths kept');
  assert.equal(solveLeg(hip,hip.clone(),ankle,target,new THREE.Vector3(1,0,0)),null,'degenerate leg');
}
// --- Synthetic rig: structure at the hip, legs straight down, ankles 0.13 m above the soles ---
function rig(){
  const root=new THREE.Group();root.scale.setScalar(1.15);
  const structure=new THREE.Object3D();structure.name='structure';structure.position.set(0,1.07,0);root.add(structure);
  for(const [tag,s]of[['l',-1],['r',1]]){
    const leg=new THREE.Object3D();leg.name='leg_'+tag;leg.position.set(s*.165,0,0);structure.add(leg);
    const shin=new THREE.Object3D();shin.name='shin_'+tag;shin.position.set(0,-.47,0);leg.add(shin);
    const foot=new THREE.Object3D();foot.name='foot_'+tag;foot.position.set(0,-.47,0);shin.add(foot);
  }
  return root;
}
const soles=foot=>SOLE.map(p=>new THREE.Vector3(...p).applyMatrix4(foot.matrixWorld));
{
  const root=rig(),r=findRig(root);assert.ok(r&&r.legs.length===2,'complete rig');
  assert.ok(Math.abs(soleLift(r.legs[0].foot,root))<1e-9,'rest soles on the body origin');
  const before=r.legs.map(l=>l.leg.quaternion.clone()),res=adaptFeet(r,root,()=>0);
  assert.equal(res.drop,0);assert.ok(r.legs.every((l,i)=>l.leg.quaternion.equals(before[i])),'flat ground changes nothing');
}
{ // a kerb under the left foot only
  const root=rig(),r=findRig(root),ground=x=>x<0?.14:0;adaptFeet(r,root,ground);root.updateMatrixWorld(true);
  assert.ok(Math.abs(Math.min(...soles(r.legs[0].foot).map(p=>p.y))-.14)<.005,'left foot planted on the kerb');
  assert.ok(Math.abs(Math.min(...soles(r.legs[1].foot).map(p=>p.y)))<.005,'right foot stays on the road');
  assert.ok(r.legs[0].shin.getWorldPosition(new THREE.Vector3()).z>.02,'left knee bends forward');
}
{ // stepping down: the spring still holds the body 14 cm up
  const root=rig();root.position.y=.14;const r=findRig(root),res=adaptFeet(r,root,()=>0);root.updateMatrixWorld(true);
  assert.ok(Math.abs(res.drop-.14)<1e-6,'the pelvis lowers');
  for(const l of r.legs)assert.ok(Math.abs(Math.min(...soles(l.foot).map(p=>p.y)))<.005,'both soles reach the road');
}
{ // a 1:10 ramp rising forward
  const root=rig(),r=findRig(root),ground=(x,z)=>.1*z;adaptFeet(r,root,ground);root.updateMatrixWorld(true);
  for(const l of r.legs){const gap=soles(l.foot).map(p=>p.y-ground(p.x,p.z));
    assert.ok(Math.min(...gap)>-.003,'no contact below the ramp');assert.ok(Math.min(...gap)<.005,'the foot touches the ramp');}
}
{ // random walk-like poses over road and kerb: no sole ends below the ground
  let seed=7,clamped=0;const rnd=()=>(seed=(Math.imul(seed,1664525)+1013904223)>>>0)/4294967296;
  for(let n=0;n<300;n++){
    const root=rig(),r=findRig(root);root.position.y=-.05+rnd()*.25;
    for(const l of r.legs){const hip=(rnd()-.5)*.8,flex=rnd()*.9;l.leg.rotation.x=hip;l.shin.rotation.x=flex;l.foot.rotation.x=-(hip+flex);}
    const gl=rnd()<.5?0:.14,gr=rnd()<.5?0:.14,ground=x=>x<0?gl:gr,res=adaptFeet(r,root,ground);root.updateMatrixWorld(true);
    if(res.raised.some(v=>v>=.35-1e-9)||res.drop>=.25-1e-9){clamped++;continue;}
    for(const l of r.legs)for(const p of soles(l.foot))assert.ok(p.y>=ground(p.x,p.z)-.003,'a sole below the ground: '+JSON.stringify({n,p}));
  }
  assert.ok(clamped<15,'limits rarely reached: '+clamped);
}
{ // incomplete or foreign rigs disable the feet
  const missing=rig();missing.getObjectByName('foot_r').removeFromParent();assert.equal(findRig(missing),null);
  assert.equal(findRig(new THREE.Group()),null);
  const odd=rig();odd.getObjectByName('foot_l').position.y=-.3;assert.equal(findRig(odd),null,'soles away from the origin are rejected');
}
console.log('System World footing: spring, stance weight, two-bone solve, kerb, step-down, ramp, 300 random poses and rig detection passed.');
```

**Acceptance:** all checks pass. The module imports only `three`.

## Task 6 — Derive hard-coded heights; re-site two docks and two planters (values unchanged under the old surfaces)

**Files**
- `ui/js/desktop/apps/sysworld-places.js`: `landmarkPlaces` lines 11–19.
- `ui/js/desktop/apps/sysworld-scene.js`: the placements block lines 89–96.
- `ui/js/desktop/apps/sysworld-experience.js`: rails line 48; `endRide` line 326.
- `ui/js/desktop/apps/sysworld-atmosphere.js`: line 200.
- `ui/js/desktop/apps/sysworld-layout.js`: `hoverPads` lines 34–43.
- `scripts/test-system-world-hover.mjs`: the dock loop lines 211–227.
- `ui/desktop_sysworld_expansion_browser_test.go`: lines 152–155.

**Steps**
- [ ] Hover test first. Add `import {pedestrianGraph} from '../ui/js/desktop/apps/sysworld-ground.js';` and, inside `for(const [i,p] of hoverPads.entries()){`, after the `dronePad` assertion:
  ```js
  // Docks stay off the zebra crossings: no pedestrian node within 2 m of a dock centre.
  for(const n of pedestrianGraph().nodes)assert.ok(Math.hypot(n.x-p.x,n.z-p.z)>=2,`${p.id}: blocks the pavement node at ${n.x}, ${n.z}`);
  ```
  Run `node scripts/test-system-world-hover.mjs`. Expected: FAIL for `south`/`east`.
- [ ] `sysworld-layout.js` `hoverPads`: south → `{id:'south',x:-7.8,z:64.3,angle:Math.PI}`, east → `{id:'east',x:72.3,z:42,angle:-Math.PI/2}`. Extend the comment above with "and at least 2 m from every pedestrian node (no dock on a zebra or its dropped kerb)".
- [ ] `sysworld-places.js`: add `import {surfaces} from './sysworld-layout.js';` at the top.
  - Change the comment to "The gateway stands on the pavement so its arch clears the road".
  - `{id:'neon-gate',x:-18,z:0,y:.5,…}` → `y:surfaces.pavement`.
- [ ] `sysworld-scene.js` lines 89–96.

  Before:
  ```js
    for (const z of zs) for (const x of [-55, -31, 31, 55]) {
      place('street-lamp', x, z - 4.5, .45); place('planter', x + 4, z - 4.5, .45);
    }
  ...
    place('server-rack', -55, -36.7, .5); place('server-rack', -49, -36.7, .5);
  ```
  After:
  ```js
    // Lamps and planters stand 5 cm into the pavement. Each planter sits on the block-centre side of
    // its lamp (towards x = +-43), clear of every zebra crossing and its dropped kerb.
    const sunk = groundLevels.pavement - .05;
    for (const z of zs) for (const x of [-55, -31, 31, 55]) {
      place('street-lamp', x, z - 4.5, sunk); place('planter', x + 4 * Math.sign(Math.sign(x) * 43 - x), z - 4.5, sunk);
    }
  ...
    place('server-rack', -55, -36.7, groundLevels.pavement); place('server-rack', -49, -36.7, groundLevels.pavement);
  ```
- [ ] `sysworld-experience.js` line 48, inside the rail loop: `points.push(tmp.x-ahead.z*offset,.21,tmp.z+ahead.x*offset);` → `points.push(tmp.x-ahead.z*offset,surfaces.road+.09,tmp.z+ahead.x*offset);`. Comment: "Rail heads 9 cm above the carriageway".
- [ ] `sysworld-experience.js` line 326: `camera.position.set(s.platformX,2.4+surfaces.pavement+.3,s.platformZ);` → `camera.position.set(s.platformX,2.4+groundHeight(s.platformX,s.platformZ),s.platformZ);`.
- [ ] `sysworld-atmosphere.js` line 200: replace the `3.35` argument with `(lamp.y || 0) + 2.9` and add the comment `// cone apex 2.9 m above the lamp base`.
- [ ] Browser test lines 152–155. Before:
  ```go
  	// West along the platform, north to the dock's row, west again: the dock itself fills x -11.7..-10.3, z 63.8..64.8.
  	axis(0, -8, input.KeyA, input.KeyD)
  	axis(2, 62.9, input.KeyW, input.KeyS)
  	axis(0, -10.5, input.KeyA, input.KeyD)
  ```
  After:
  ```go
  	// West off the platform, north to the dock's row, west again: the dock itself fills x -8.5..-7.1, z 63.8..64.8.
  	axis(0, -5.3, input.KeyA, input.KeyD)
  	axis(2, 62.9, input.KeyW, input.KeyS)
  	axis(0, -7.8, input.KeyA, input.KeyD)
  ```
- [ ] Run:
  - `node scripts/test-system-world-hover.mjs` → PASS ("5 docks on open pavement…").
  - `node scripts/test-system-world-layout.mjs`, `node scripts/test-system-world-living.mjs` → PASS.
  - `go test ./ui -run '^$' -count=1` → compiles.
- [ ] Commit: `refactor(system-world): derive street-furniture heights from surfaces; keep docks and planters off zebras`.

**Acceptance**
- No literal `.45`, `.5` (rack/gate), `.21` or `3.35` height remains at these sites.
- The heights are numerically identical under the current `surfaces`.

## Task 7 — Pedestrian graph in the wayfinder (parallel with Task 8)

**Files**
- `ui/js/desktop/apps/sysworld-places.js`: `createWayfinder` lines 21–51.
- `scripts/test-system-world-ground.mjs`: append.

**Steps**
- [ ] Append this test section and run `node scripts/test-system-world-ground.mjs`. Expected: FAIL, because the old wayfinder links across carriageways.
  ```js
  // --- Resident routes cross carriageways only on zebras ---
  {
    const {createWayfinder}=await import('../ui/js/desktop/apps/sysworld-places.js');
    const {createTraffic}=await import('../ui/js/desktop/apps/sysworld-traffic.js');
    const {groundHeight}=await import('../ui/js/desktop/apps/sysworld-exploration.js');
    const {nodes,links}=pedestrianGraph(),finder=createWayfinder(createTraffic(),groundHeight);
    const same=(n,p)=>near(n.x,p.x,1e-9)&&near(n.z,p.z,1e-9);
    const zebraLeg=(a,b)=>links.some(l=>l.zebra&&((same(nodes[l.a],a)&&same(nodes[l.b],b))||(same(nodes[l.a],b)&&same(nodes[l.b],a))));
    for(const [from,to]of[[{x:-40,z:-50},{x:-40,z:-15}],[{x:-40,z:-50},{x:40,z:30}],[{x:5,z:45},{x:-50,z:70}]]){
      const body={id:'walker',circles:[{x:0,z:0,r:.6}],minY:0,maxY:2,reach:.6,heading:0,...from,y:groundHeight(from.x,from.z)};
      const path=finder.route(body,{...to,y:groundHeight(to.x,to.z)});assert.ok(path.length,'route found '+JSON.stringify([from,to]));
      let prev=from;
      for(const p of path){if(!carriagewayFree(prev,p))assert.ok(zebraLeg(prev,p),'route crosses a carriageway away from a zebra: '+JSON.stringify([prev,p]));prev=p;}
    }
  }
  ```
- [ ] Replace `createWayfinder` with:
  ```js
  // Residents walk the pavement graph of sysworld-ground.js: links along pavements and over plots,
  // carriageways crossed only on zebras. Static links are pure and built once; bodies and solids filter them.
  export function createWayfinder(traffic, floor) {
    const graph=pedestrianGraph(),points=graph.nodes.map(n=>({x:n.x,z:n.z,y:floor(n.x,n.z),heading:0})),cache=new Map();let revision=-1;
    function route(body,destination) {
      if(revision!==traffic.revision()){cache.clear();revision=traffic.revision();}
      const key=body.circles.map(c=>[c.x,c.z,c.r].join(',')).join(';')+':'+body.maxY;
      if(!cache.has(key)) {
        const index=new Map(),nodes=[];
        points.forEach((p,i)=>{if(traffic.clear(body,p,p,false)){index.set(i,nodes.length);nodes.push(p);}});
        const edges=nodes.map(()=>[]);
        for(const l of graph.links){const a=index.get(l.a),b=index.get(l.b);if(a===undefined||b===undefined)continue;
          if(traffic.clear(body,nodes[a],nodes[b],false)){edges[a].push([b,l.length]);edges[b].push([a,l.length]);}}
        cache.set(key,{nodes,edges});
      }
      const graphKey=cache.get(key),nodes=[...graphKey.nodes,{x:body.x,y:body.y,z:body.z,heading:body.heading},{...destination,heading:0}];
      const start=nodes.length-2,end=nodes.length-1,edges=graphKey.edges.map(e=>[...e]);edges.push([],[]);
      for(const k of [start,end])for(let i=0;i<k;i++) {
        const a=nodes[k],b=nodes[i],d=Math.hypot(a.x-b.x,a.z-b.z);
        // Only a start standing on a zebra may step off it; every other link stays off carriageways and kerbs.
        if((d<maxLink||i===start)&&carriagewayFree(a,b,k===start)&&traffic.clear(body,a,b,false)){edges[k].push([i,d]);edges[i].push([k,d]);}
      }
      // (Dijkstra and path reconstruction: keep the existing lines 41-48 unchanged.)
    }
    return {route,points};
  }
  ```
  Add the import `import {pedestrianGraph,carriagewayFree,maxLink} from './sysworld-ground.js';` at the top of the file. Keep the Dijkstra block (old lines 41–48) verbatim.
- [ ] Run:
  - `node scripts/test-system-world-ground.mjs` → PASS.
  - `node scripts/test-system-world-traffic.mjs` and `node scripts/test-system-world-jitter.mjs` → PASS. Node 0 starts a straight pavement run, so the guide tests keep their straight walks.
  - `node scripts/test-system-world-living.mjs` → PASS ("Most residents should make progress", "completed > 0").
- [ ] Commit: `feat(system-world): resident routes on pavements and zebra crossings only`.

**Acceptance:** no route leg crosses a carriageway except a zebra link; `points` is still exported, and spawns and fallbacks use it.

## Task 8 — Footing for residents: spring, feet, pitch, tier switch (parallel with Task 7)

**Files**
- `ui/js/desktop/apps/sysworld-society.js`: imports; `add()` line 27; the return object at line 179; `stats` line 213.
- `ui/js/desktop/apps/sysworld-experience.js`: line 410; the return object at line 419.
- `ui/js/desktop/apps/sysworld-scene.js`: `simulate` line 477.
- `scripts/test-system-world-living.mjs`.

**Where the correction runs**
1. `experience.update` calls `mixers.forEach(m=>m.update(step))` (line 359). This rewrites every leg/shin/foot quaternion and the `structure` translation, because every clip keys them. The `PropertyMixer` blends against bind-time originals, so corrections never accumulate.
2. `society.update` proposes moves.
3. `traffic.solve` commits x, z and heading through the commit callbacks.
4. Then `society.settle(dt)` places each resident: spring y, pitch, `adaptFeet`.

Invisible or disabled residents skip both the mixer and `settle`.

**Steps**
- [ ] Living test first.
  - In `simulate` (line 42), change `traffic.solve(dt);` to `traffic.solve(dt);experience.settle(dt);`.
  - After the `assert.equal(experience.stats().society.residents.length,24,…)` line add:
    ```js
    assert.equal(experience.stats().society.rigs,19,'Every resident exposes its leg rig (structure, leg/shin/foot per side) with soles at its origin');
    ```
  - Run `node scripts/test-system-world-living.mjs`. Expected: FAIL (`settle` is not a function).
- [ ] `sysworld-society.js`: add `import {springStep,findRig,adaptFeet} from './sysworld-footing.js';`.
- [ ] `add()`, line 27. Before:
  ```js
      actors.push(a);model.node.position.set(body.x,body.y,body.z);return a;
  ```
  After:
  ```js
      actors.push(a);model.node.position.set(body.x,body.y,body.z);
      // Damped height and slope pitch for every resident; feet only when the rig is complete.
      a.height={y:body.y,v:0};a.pitch={y:0,v:0};model.node.rotation.order='YXZ';a.rig=findRig(model.node);
      return a;
  ```
- [ ] Add a function before `return {` (line 179):
  ```js
  // One fixed step after the mixer has posed the clips and traffic has committed every move.
  function settle(dt) {
    if(disposed)return;
    const feet=tier!=='low';
    for(const a of actors) {
      if(!a.height)continue;                             // hovering patrols own their height
      if(!a.body.enabled||!a.node.visible)continue;
      const b=a.body,y=springStep(a.height,b.y,dt);
      const fx=Math.sin(b.heading),fz=Math.cos(b.heading),slope=floor(b.x+fx*.35,b.z+fz*.35)-floor(b.x-fx*.35,b.z-fz*.35);
      const pitch=springStep(a.pitch,Math.max(-.08,Math.min(.08,Math.atan2(slope,.7)*.5)),dt);
      a.node.position.set(b.x,y,b.z);a.node.rotation.y=b.heading;a.node.rotation.x=-pitch;
      if(feet&&a.rig)adaptFeet(a.rig,a.node,floor);
    }
  }
  ```
- [ ] In the return object, directly after the line `    add,patrol,update,suspend,` (unchanged), add a new line `    settle,`. In `stats`, add `rigs:actors.filter(a=>a.rig).length,` after `completed,`.
- [ ] `sysworld-experience.js` line 410: `if(!options.traffic)traffic.solve(step);` → `if(!options.traffic){traffic.solve(step);society.settle(step);}`. In the return object, after the line `return {update,interact,endRide,setTier,society,`, add `    settle(dt){if(!disposed&&manifest)society.settle(dt);},`.
- [ ] `sysworld-scene.js` line 477: `traffic.solve(step);experience.syncRide?.();` → `traffic.solve(step);experience.settle(reduced?0:step);experience.syncRide?.();`.
- [ ] Run:
  - `node scripts/test-system-world-living.mjs` → PASS with rigs 19. If it reports fewer, a rig check failed on the real GLB: report which, do not loosen it.
  - `node scripts/test-system-world-traffic.mjs`, `node scripts/test-system-world-jitter.mjs`, `node scripts/test-system-world-hover.mjs`, `node scripts/test-system-world-layout.mjs` → PASS.
- [ ] Commit: `feat(system-world): residents follow the ground with a damped spring, slope pitch and planted feet`.

**Acceptance**
- Foot adaptation runs on medium, high and ultra, and is off on low. Spring and pitch run on all tiers.
- Reduced motion snaps (dt 0).
- The survey harness patch strings are unchanged.

## Task 9 — Contact shadows

**Files**
- New `ui/js/desktop/apps/sysworld-contact.js`.
- `sysworld-scene.js`: imports; after line 173; `createExperience` options (230–237); `createCityLife` options (567); `dispose` (562).
- `sysworld-experience.js`: the `createSociety` call, line 36.
- `sysworld-society.js`: signature line 8; `add()`; `settle`.
- `sysworld-life.js`: imports; line 103; residents push line 106; `update` after line 201.
- `scripts/test-system-world-footing.mjs`: append.

**Steps**
- [ ] Append to the footing test (before the final `console.log`), with `import {createContactShadows} from '../ui/js/desktop/apps/sysworld-contact.js';` at the top:
  ```js
  {
    const scene=new THREE.Scene(),pool=createContactShadows(scene,{capacity:2}),a=pool.claim(),b=pool.claim();
    assert.ok(a&&b);assert.equal(pool.claim(),null,'bounded pool');
    a.set(1,0,2,.5,.4);const mesh=scene.getObjectByName('contact-shadows'),m=new THREE.Matrix4();mesh.getMatrixAt(0,m);
    const p=new THREE.Vector3().setFromMatrixPosition(m);assert.ok(p.x===1&&p.z===2&&p.y>0&&p.y<.05,'just above the ground');
    b.hide();mesh.getMatrixAt(1,m);assert.equal(new THREE.Vector3().setFromMatrixScale(m).x,0);
    pool.dispose();assert.equal(scene.children.length,0);
  }
  ```
  Run → FAIL.
- [ ] `sysworld-contact.js` (complete):
  ```js
  import * as THREE from 'three';
  // Soft blob shadows under residents, the companion and the ground projection of hover robots.
  // One instanced quad, one draw call; a slot is claimed once and moved every step.
  export function createContactShadows(scene,{capacity=40}={}){
    const geometry=new THREE.PlaneGeometry(1,1);geometry.rotateX(-Math.PI/2);
    const strength=new THREE.InstancedBufferAttribute(new Float32Array(capacity),1);geometry.setAttribute('swShadow',strength);
    const material=new THREE.ShaderMaterial({transparent:true,depthWrite:false,polygonOffset:true,polygonOffsetFactor:-2,polygonOffsetUnits:-2,
      vertexShader:'attribute float swShadow;varying vec2 vUv;varying float vA;void main(){vUv=uv;vA=swShadow;vec4 p=vec4(position,1.0);\n#ifdef USE_INSTANCING\np=instanceMatrix*p;\n#endif\ngl_Position=projectionMatrix*modelViewMatrix*p;}',
      fragmentShader:'varying vec2 vUv;varying float vA;void main(){float r=length(vUv-.5)*2.;gl_FragColor=vec4(0.,0.,0.,vA*(1.-smoothstep(.1,1.,r)));}'});
    const mesh=new THREE.InstancedMesh(geometry,material,capacity),matrix=new THREE.Matrix4();
    mesh.name='contact-shadows';mesh.frustumCulled=false;mesh.renderOrder=1;mesh.castShadow=false;mesh.receiveShadow=false;
    for(let i=0;i<capacity;i++)mesh.setMatrixAt(i,matrix.makeScale(0,0,0));scene.add(mesh);
    let used=0;
    return {
      claim(){
        if(used>=capacity)return null;const i=used++;
        return {
          set(x,y,z,radius,alpha){mesh.setMatrixAt(i,matrix.makeScale(radius*2,1,radius*2).setPosition(x,y+.015,z));strength.setX(i,alpha);mesh.instanceMatrix.needsUpdate=true;strength.needsUpdate=true;},
          hide(){mesh.setMatrixAt(i,matrix.makeScale(0,0,0));strength.setX(i,0);mesh.instanceMatrix.needsUpdate=true;strength.needsUpdate=true;},
        };
      },
      stats:()=>({used,capacity}),
      dispose(){mesh.removeFromParent();mesh.dispose();geometry.dispose();material.dispose();},
    };
  }
  ```
- [ ] `sysworld-scene.js`:
  - Add `import { createContactShadows } from './sysworld-contact.js';`.
  - After `scene.add(world); world.add(staticCity, landmarks);` add `const contact = createContactShadows(scene);`.
  - In the `createExperience` options add `shadows:contact,`.
  - In the `createCityLife` options object add `shadows:contact,`.
  - In `dispose()` add `contact.dispose();` right after `drones.dispose();`.
- [ ] `sysworld-experience.js` line 36: add `shadows:options.shadows` to the `createSociety` options.
- [ ] `sysworld-society.js`:
  - Signature: add `shadows` to the destructured options.
  - In `add()`, append `a.shadow=shadows?.claim()||null;` to the line that sets `a.rig`.
  - In `settle`, replace `if(!a.body.enabled||!a.node.visible)continue;` with `if(!a.body.enabled||!a.node.visible){a.shadow?.hide();continue;}`.
  - After the `adaptFeet` line add `a.shadow?.set(b.x,floor(b.x,b.z),b.z,.62,.5);`.
- [ ] `sysworld-life.js`:
  - Add `import {groundHeight} from './sysworld-exploration.js';`.
  - Line 103: `glow.position.y=.58` → `glow.position.y=.08`.
  - In the `residents.push({...})` at line 106 add `shadow:options.shadows?.claim()||null`.
  - In `update`, after `r.jet.scale.y=…` (line 201):
    ```js
          // Ground light and contact shadow lie on the actual street surface under the robot.
          const floor=groundHeight(r.glow.position.x,r.glow.position.z);r.glow.position.y=floor+.08;
          if(r.shadow){if(r.root.visible)r.shadow.set(r.glow.position.x,floor,r.glow.position.z,1.1,.3);else r.shadow.hide();}
    ```
- [ ] Run:
  - `node scripts/test-system-world-footing.mjs` → PASS.
  - `node scripts/test-system-world-living.mjs`, `node scripts/test-system-world-traffic.mjs` → PASS.
  - The esbuild compile one-liner from Task 3 → `ok`.
- [ ] Commit: `feat(system-world): contact shadows under residents and hover robots`.

**Acceptance:** one extra draw call. The hover-robot glow and the shadow sit on the real ground height.

## Task 10 — Companion follows the ground on one level

**Files**
- `ui/js/desktop/apps/sysworld-companion.js` (whole file).
- `sysworld-scene.js`: the `createCompanion` call at line 569.
- `scripts/test-system-world-companion.mjs`: lines 4–26.

**Steps**
- [ ] Companion test. Change the import to `import { companionPose, companionGait, LOOP } from '../ui/js/desktop/apps/sysworld-companion.js';`, add `import { groundHeight } from '../ui/js/desktop/apps/sysworld-exploration.js';` and `import { streetProfile } from '../ui/js/desktop/apps/sysworld-ground.js';`, and replace lines 7–26 with:
  ```js
  const first = companionPose(0), loop = companionPose(LOOP.seconds);
  assert.ok(Math.abs(first.x - loop.x) < 1e-9 && Math.abs(first.z - loop.z) < 1e-9, 'The patrol is a closed loop');
  assert.deepEqual(companionPose(7), companionPose(7), 'The pose is pure');
  let minX = Infinity, maxX = -Infinity, minZ = Infinity, maxZ = -Infinity, previous = -Infinity;
  for (let t = 0; t <= 2 * LOOP.seconds; t += .05) {
    const p = companionPose(t);
    assert.ok(Number.isFinite(p.x) && Number.isFinite(p.z) && Number.isFinite(p.heading));
    minX = Math.min(minX, p.x); maxX = Math.max(maxX, p.x); minZ = Math.min(minZ, p.z); maxZ = Math.max(maxZ, p.z);
    assert.ok(p.phase >= previous, 'Gait phase never runs backwards'); previous = p.phase;
  }
  // One level: the plot east of the x = 18 street, clear of its kerb, the southern boulevard (z <= 65),
  // the quay step (z >= 74), the missions pavilion (x >= 37) and the harbour hover dock.
  assert.ok(minX > 24 && maxX < 35.5, 'Patrol stays on the plot: ' + minX + '..' + maxX);
  assert.ok(minZ > 65 && maxZ < 74, 'Patrol stays off the boulevard and the quay: ' + minZ + '..' + maxZ);
  const level = groundHeight(LOOP.cx, LOOP.cz);
  for (let t = 0; t <= LOOP.seconds; t += .05) {
    const p = companionPose(t), s = streetProfile(p.x, p.z);
    assert.ok(!s || s.kind === 'pavement', 'The patrol never touches a kerb or a carriageway');
    for (const [dx, dz] of [[.6, 0], [-.6, 0], [0, .35], [0, -.35]]) assert.equal(groundHeight(p.x + dx, p.z + dz), level, 'The patrol stays on one level');
    assert.ok(Math.hypot(p.x - 23.3, p.z - 75) > 2.5, 'Patrol clears the hover dock');
  }
  ```
  Run → FAIL (`LOOP` not exported, old bounds).
- [ ] Replace `sysworld-companion.js` with the following. Keep the existing provenance comment lines 4–8, adjusting their first sentence as shown.
  ```js
  import * as THREE from 'three';
  import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';
  import { MeshoptDecoder } from 'three/addons/libs/meshopt_decoder.module.js';
  import { groundHeight } from './sysworld-exploration.js';
  import { springStep } from './sysworld-footing.js';
  // A decorative quadruped patrols the quiet plot between the x = 18 street and the missions pavilion,
  // on one level (no kerb, no carriageway); height and pitch still follow the ground.
  // The GLB is a meshopt-compressed derivative of the pinned BSD-3-Clause
  // MuJoCo Menagerie Spot (see assets/system-world/spot-source.json). It is
  // scenery, not an actor: it never joins traffic, telemetry or the robot foley.
  // One bounded loop, one accumulated clock, no per-frame allocations.

  export const LOOP = { cx: 30, cz: 69.5, rx: 4.5, rz: 2.5, seconds: 16, gaitHz: 1.7 };
  const JOINTS = ['body', 'fl_hip', 'fr_hip', 'hl_hip', 'hr_hip', 'fl_uleg', 'fr_uleg', 'hl_uleg', 'hr_uleg', 'fl_lleg', 'fr_lleg', 'hl_lleg', 'hr_lleg'];
  const TROT = { fl: 0, hr: 0, fr: Math.PI, hl: Math.PI };

  export function companionPose(time) { /* unchanged body */ }
  export function companionGait(phase) { /* unchanged body */ }

  export function createCompanion({ scene, resourceURL, signal, ground = groundHeight, shadows = null, onError }) {
    const group = new THREE.Group();
    group.visible = false;
    scene.add(group);
    const joints = Object.create(null), box = new THREE.Box3();
    const height = { y: NaN, v: 0 }, pitch = { y: 0, v: 0 }, shadow = shadows?.claim() || null;
    let model = null, footOffset = 0, time = 0, failed = false, disposed = false;
    const loader = new GLTFLoader();
    loader.setMeshoptDecoder(MeshoptDecoder);
    const ready = (async () => {
      /* unchanged, except: replace `baseY = groundY - box.min.y;` with `footOffset = -box.min.y;`
         and the comment with "Feet on the ground regardless of how the exporter baked pivots." */
    })();
    function update(dt, animated) {
      if (!model || disposed) return;
      const step = animated ? Math.min(.1, Math.max(0, dt)) : 0;
      time += step;
      const pose = companionPose(time);
      // The kit's forward axis is +X; rotation.y maps it to (cos, 0, -sin).
      const fx = Math.cos(pose.heading), fz = -Math.sin(pose.heading), half = .55;
      const floor = ground(pose.x, pose.z), y = springStep(height, floor, step);
      const slope = ground(pose.x + fx * half, pose.z + fz * half) - ground(pose.x - fx * half, pose.z - fz * half);
      const tilt = springStep(pitch, Math.atan2(slope, 2 * half), step);
      group.position.set(pose.x, y + footOffset + Math.abs(Math.sin(pose.phase)) * .012, pose.z);
      group.rotation.set(0, pose.heading, tilt, 'YZX');   // yaw, then nose up/down about the body's Z
      shadow?.set(pose.x, floor, pose.z, .6, .45);
      for (const [key, gait] of Object.entries(companionGait(pose.phase))) {
        if (joints[key + '_hip']) joints[key + '_hip'].rotation.y = gait.hip;
        if (joints[key + '_uleg']) joints[key + '_uleg'].rotation.y = gait.uleg;
      }
    }
    return { ready, update, stats: /* unchanged */, dispose() { if (disposed) return; disposed = true; shadow?.hide(); /* rest unchanged */ } };
  }
  ```
  Write the full bodies: copy the unchanged parts verbatim from the current file. Do not leave the `/* unchanged */` placeholders in the code.
- [ ] Scene line 569: `companion = createCompanion({scene, resourceURL:options.resourceURL, signal:abort.signal, groundY:groundLevels.quay, onError:options.onCompanionError});` → `companion = createCompanion({scene, resourceURL:options.resourceURL, signal:abort.signal, shadows:contact, onError:options.onCompanionError});`.
- [ ] Run `node scripts/test-system-world-companion.mjs` → PASS. Run the esbuild compile one-liner → `ok`.
- [ ] Commit: `feat(system-world): companion patrols one level east of the x=18 street and follows ground height and pitch`.

**Acceptance:** the loop is x 25.5–34.5, z 67–72, 16 s per lap. Spring and pitch come from the ground. The companion has a shadow.

## Task 11 — Visitor camera follows the ground with the walker's damping

**Files:** `ui/js/desktop/apps/sysworld-scene.js` — imports; `focus()` line 370; `setMode()` line 385; `move()` lines 453–468.

**Steps**
- [ ] Imports:
  - `import { interiors } from './sysworld-exploration.js';` → `import { interiors, groundHeight } from './sysworld-exploration.js';`
  - add `import { springStep } from './sysworld-footing.js';`
- [ ] `focus()` line 370: `camera.position.set(d.x, 2.4, d.z + d.radius + 7);` → `camera.position.set(d.x, 2.4 + groundHeight(d.x, d.z + d.radius + 7), d.z + d.radius + 7);`.
- [ ] `setMode()` line 385: `camera.position.set(18, 2.4, 57);` → `camera.position.set(18, 2.4 + groundHeight(18, 57), 57);`.
- [ ] Replace `move(dt)` with:
  ```js
  // The visitor's eye follows the ground with the residents' critically damped spring.
  const eye = { y: NaN, v: 0 };
  function move(dt) {
    const speed = (keys.has('ShiftLeft') ? 25 : 11) * dt;
    let forward = Number(keys.has('KeyW') || keys.has('ArrowUp')) - Number(keys.has('KeyS') || keys.has('ArrowDown'));
    let right = Number(keys.has('KeyD') || keys.has('ArrowRight')) - Number(keys.has('KeyA') || keys.has('ArrowLeft'));
    // A hover-pack flight runs even without a key held, so the velocity can ease out.
    if (experience.hovering()) { experience.fly(dt, { forward, right, up: Number(keys.has('Space')) - Number(keys.has('KeyC')), boost: keys.has('ShiftLeft') }); eye.y = NaN; return; }
    if (forward || right) {
      if (experience.walkRide(forward, dt, right)) return;
      const scale = speed / Math.hypot(forward, right);
      camera.getWorldDirection(v); v.y = 0; v.normalize();
      const dx = (v.x * forward - v.z * right) * scale, dz = (v.z * forward + v.x * right) * scale;
      const allowed = (x,z)=>experience.move(x,z,camera.position)&&traffic.clear(visitor,visitor,{x,y:2.4+experience.floor(x,z),z,heading:0});
      if (allowed(camera.position.x + dx, camera.position.z)) camera.position.x += dx;
      if (allowed(camera.position.x, camera.position.z + dz)) camera.position.z += dz;
    }
    if (experience.isRiding()) { eye.y = NaN; return; }
    // Anything else that placed the camera (a visit, a landing, a refused step) restarts the spring there.
    if (!(Math.abs(camera.position.y - eye.y) < 1e-6)) { eye.y = camera.position.y; eye.v = 0; }
    camera.position.y = springStep(eye, 2.4 + experience.floor(camera.position.x, camera.position.z), reduced ? 0 : dt);
  }
  ```
- [ ] Run the esbuild compile one-liner → `ok`. Run `go test ./ui -run '^TestDesktopSysWorldAppMarkers$' -count=1` → PASS (`Math.hypot(forward, right)` is kept).
- [ ] Commit: `feat(system-world): visitor eye height eases over kerbs and steps`.

**Acceptance:** the eye settles toward the ground even with no key held. Rides, flights and teleports are unaffected (the spring restarts). Reduced motion snaps.

## Task 12 — Switch the ground to the v3 street kit (**blocked until the main session delivers `ui/3d/system-world/v3` with `street-plain`, `street-drain`, `street-zebra`, `street-fill`, `street-crossing`, `street-end`**)

**Preconditions:** `node scripts/build-system-world-v3.mjs --check` and `node scripts/test-system-world-v3.mjs` pass on the delivered payload.

**Files**
- `sysworld-layout.js`: line 3; delete `streetHeight` (lines 49–52).
- `sysworld-exploration.js`: line 2 imports; `groundHeight` line 29.
- `sysworld-scene.js`: imports; the placement block lines 73–88; `kitPlacements`; the ground block lines 177–180; `dispose`.
- `ui/3d/system-world/v1/manifest.json` and six GLBs.
- `assets/system-world/check_assets.py`: line 18.
- `scripts/test-system-world-ground.mjs`, `scripts/test-system-world-traffic.mjs` line 71.

**Steps**
- [ ] Ground test: add at the top, after the `R`/`P` line:
  ```js
  assert.equal(surfaces.pavement,surfaces.road+kerb.rise,'kerb rise joins road and pavement');
  assert.equal(surfaces.pavement,surfaces.ground,'pavements are flush with the plots');
  ```
  Run → FAIL.
- [ ] Layout line 3 → `export const surfaces = {ground:0,road:-.14,pavement:0,quay:.08,room:.16,gallery:4.16};`. Delete `streetHeight`.
- [ ] Exploration:
  - Line 2 → `import {interiors,stations,towers,surfaces} from './sysworld-layout.js';` plus `import {streetProfile} from './sysworld-ground.js';`.
  - Line 29 before: `return Math.max(streetHeight(x,z),z>=74&&z<=80&&x>=-80&&x<=80?surfaces.quay:surfaces.ground);`
  - After:
    ```js
      const street=streetProfile(x,z);if(street)return street.height;
      return z>=74&&z<=80&&x>=-80&&x<=80?surfaces.quay:surfaces.ground;
    ```
  - Line 28 (`surfaces.pavement+.3`) stays: the platform stays 0.30 above the pavement.
- [ ] Scene:
  - Add `import { streetModules, plotSlabs, island } from './sysworld-ground.js';`.
  - Delete the two `street-crossing`/`street-tile` loops (lines 73–88, keeping `const {zs} = streets;`).
  - Replace `export const kitPlacements = [];` with:
    ```js
    // Kit v3 streets: fixed-length modules, never stretched (layout and order from sysworld-ground.js).
    export const kitPlacements = streetModules().map(m => ({ asset: m.id, x: m.x, z: m.z, y: 0, angle: m.angle, scale: [1, 1, 1] }));
    ```
- [ ] Scene ground block (lines 177–180) after:
  ```js
  // Island body: board-formed concrete quay walls on the sides. Its dark road-like top only shows
  // through hairline seams between the street modules. Plot and quay slabs sit flush at ground level.
  const groundMaterial = new THREE.MeshStandardMaterial({ color: 0x1d2a31, metalness: .05, roughness: .82 });
  const seamMaterial = new THREE.MeshStandardMaterial({ color: 0x15191d, metalness: .05, roughness: .9 });
  matSet.add(groundMaterial); matSet.add(seamMaterial);
  const bodyGeometry = new THREE.BoxGeometry(streets.maxX - streets.minX, island.top - island.bottom, streets.maxZ - streets.minZ); geoSet.add(bodyGeometry);
  const ground = new THREE.Mesh(bodyGeometry, [groundMaterial, groundMaterial, seamMaterial, groundMaterial, groundMaterial, groundMaterial]);
  ground.position.set((streets.minX + streets.maxX) / 2, (island.top + island.bottom) / 2, (streets.minZ + streets.maxZ) / 2);
  ground.receiveShadow = true; world.add(ground);
  surfaces.apply(groundMaterial, 'world', 'ground'); surfaces.apply(seamMaterial, 'world', 'city.road');
  const slabs = plotSlabs(), slabGeometry = new THREE.BoxGeometry(1, 1, 1); geoSet.add(slabGeometry);
  const plots = new THREE.InstancedMesh(slabGeometry, groundMaterial, slabs.length); plots.name = 'plots'; plots.receiveShadow = true;
  slabs.forEach((s, i) => plots.setMatrixAt(i, new THREE.Matrix4().compose(new THREE.Vector3((s.x0 + s.x1) / 2, (s.top + s.bottom) / 2, (s.z0 + s.z1) / 2),
    new THREE.Quaternion(), new THREE.Vector3(s.x1 - s.x0, s.top - s.bottom, s.z1 - s.z0))));
  world.add(plots);
  ```
  In `dispose()` add `plots.dispose();` next to `beacons.dispose();`.
- [ ] Remove the v1 street designs from the runtime payload.
  - Write a throwaway Python script in scratch that:
    1. loads `ui/3d/system-world/v1/manifest.json` with `json.load`;
    2. drops the assets with id `street-tile` and `street-crossing`;
    3. sets `total_model_bytes` to the sum of the remaining LOD `bytes`;
    4. writes `json.dumps(m, indent=2) + '\n'` with `newline='\n'`.
  - `git diff ui/3d/system-world/v1/manifest.json` must show only removed blocks and the new total.
  - `git rm ui/3d/system-world/v1/street-tile.lod{0,1,2}.glb ui/3d/system-world/v1/street-crossing.lod{0,1,2}.glb` (check the exact file names in the manifest before deleting).
  - `check_assets.py` line 18: `== 17, 'Incomplete asset catalog'` → `== 15, 'Incomplete asset catalog'`.
- [ ] `scripts/test-system-world-traffic.mjs` line 71: `p(-64.2724,-75.8183,.15,-.98866)` → `p(-64.2724,-75.8183,surfaces.road+.03,-.98866)`, and add `import {surfaces} from '../ui/js/desktop/apps/sysworld-layout.js';`.
- [ ] Run, each must PASS:
  - `node scripts/test-system-world-ground.mjs`
  - `node scripts/test-system-world-layout.mjs`
  - `node scripts/test-system-world-hover.mjs`
  - `node scripts/test-system-world-living.mjs`
  - `node scripts/test-system-world-traffic.mjs`
  - `node scripts/test-system-world-expansion.mjs`
  - `node scripts/test-system-world-companion.mjs`
  - `node scripts/test-system-world-surfaces.mjs`
  - `node scripts/test-system-world-v3.mjs`
  - `python assets/system-world/check_assets.py`
- [ ] Commit: `feat(system-world): flush pavements, 14 cm kerbs and v3 street modules replace the stretched v1 tiles`.

**Acceptance**
- No v1 street placement remains.
- `streetHeight` is gone.
- The island top is at −0.2 with the `city.road` kind; the slabs are flush at 0.
- Stations, quay, lamps, gate and racks derive their height. Tram stops keep the v2 station, and the 0.49 m step is accepted.

## Task 13 — Street-model contract test (blocked like Task 12)

**Files:** new `scripts/test-system-world-street-models.mjs`.

- [ ] Write the test below and run `node scripts/test-system-world-street-models.mjs`. Expected: PASS with `gaps 0, over tolerance 0`.
  - If it fails, report the worst cells per module id and LOD to the main session.
  - Likely causes are corner arcs with fewer than 8 segments per 90°, coarse flare triangulation, or a LOD that simplifies the walking surface.
  - Do not widen the tolerance.
  ```js
  // Kit v3 street modules against the analytic profile: every packed GLB, placed exactly like the renderer
  // places it, is rasterised from above (highest up-facing surface per 10 cm cell). Wherever streetProfile
  // defines a street the surface must exist and match within 1.5 cm, at every LOD.
  import assert from 'node:assert/strict';
  import fs from 'node:fs/promises';
  import * as THREE from 'three';
  import {GLTFLoader} from 'three/addons/loaders/GLTFLoader.js';
  import {MeshoptDecoder} from 'three/addons/libs/meshopt_decoder.module.js';
  import {streets} from '../ui/js/desktop/apps/sysworld-layout.js';
  import {streetProfile} from '../ui/js/desktop/apps/sysworld-ground.js';
  import {kitPlacements} from '../ui/js/desktop/apps/sysworld-scene.js';

  const dir='ui/3d/system-world/v3/',manifest=JSON.parse(await fs.readFile(dir+'manifest.json','utf8')),H=.1;
  const ids=[...new Set(kitPlacements.map(p=>p.asset))];
  await MeshoptDecoder.ready;const loader=new GLTFLoader().setMeshoptDecoder(MeshoptDecoder);
  const NX=Math.round((streets.maxX-streets.minX)/H),NZ=Math.round((streets.maxZ-streets.minZ)/H);
  for(const level of [0,1,2]){
    const templates=new Map();
    for(const id of ids){
      const entry=manifest.assets.find(a=>a.id===id);assert.ok(entry,'Kit v3 lacks '+id);
      const data=await fs.readFile(dir+entry.lods.find(l=>l.level===level).file);
      const gltf=await loader.parseAsync(data.buffer.slice(data.byteOffset,data.byteOffset+data.byteLength),'');
      gltf.scene.updateMatrixWorld(true);const tris=[],a=new THREE.Vector3(),b=new THREE.Vector3(),c=new THREE.Vector3(),n=new THREE.Vector3();
      let vc=0;gltf.scene.traverse(m=>{if(!m.isMesh)return;if(m.geometry.getAttribute('color'))vc++;
        assert.ok(m.material.vertexColors,id+': baked AO must load as vertex colours');
        const pos=m.geometry.getAttribute('position'),index=m.geometry.getIndex(),count=index?index.count:pos.count;
        for(let i=0;i<count;i+=3){
          a.fromBufferAttribute(pos,index?index.getX(i):i).applyMatrix4(m.matrixWorld);b.fromBufferAttribute(pos,index?index.getX(i+1):i+1).applyMatrix4(m.matrixWorld);c.fromBufferAttribute(pos,index?index.getX(i+2):i+2).applyMatrix4(m.matrixWorld);
          n.subVectors(b,a).cross(c.clone().sub(a)).normalize();if(Math.abs(n.y)>.2)tris.push([a.x,a.y,a.z,b.x,b.y,b.z,c.x,c.y,c.z]);
        }});
      assert.ok(vc>0,id+' carries COLOR_0');templates.set(id,tris);
    }
    const top=new Float32Array(NX*NZ).fill(-Infinity);
    for(const p of kitPlacements){
      const co=Math.cos(p.angle),si=Math.sin(p.angle),w=(x,z)=>[p.x+x*co+z*si,p.z-x*si+z*co];
      for(const t of templates.get(p.asset)){
        const [ax,az]=w(t[0],t[2]),[bx,bz]=w(t[3],t[5]),[cx,cz]=w(t[6],t[8]),ay=p.y+t[1],by=p.y+t[4],cy=p.y+t[7];
        const det=(bz-cz)*(ax-cx)+(cx-bx)*(az-cz);if(Math.abs(det)<1e-12)continue;
        const i0=Math.max(0,Math.ceil((Math.min(ax,bx,cx)-streets.minX)/H-.5)),i1=Math.min(NX-1,Math.floor((Math.max(ax,bx,cx)-streets.minX)/H-.5));
        const k0=Math.max(0,Math.ceil((Math.min(az,bz,cz)-streets.minZ)/H-.5)),k1=Math.min(NZ-1,Math.floor((Math.max(az,bz,cz)-streets.minZ)/H-.5));
        for(let i=i0;i<=i1;i++)for(let k=k0;k<=k1;k++){
          const x=streets.minX+(i+.5)*H,z=streets.minZ+(k+.5)*H;
          const l1=((bz-cz)*(x-cx)+(cx-bx)*(z-cz))/det,l2=((cz-az)*(x-cx)+(ax-cx)*(z-cz))/det,l3=1-l1-l2;
          if(l1<-1e-7||l2<-1e-7||l3<-1e-7)continue;
          const y=l1*ay+l2*by+l3*cy,idx=i*NZ+k;if(y>top[idx])top[idx]=y;
        }
      }
    }
    let gaps=0,bad=0,worst=0,examples=[];
    for(let i=0;i<NX;i++)for(let k=0;k<NZ;k++){
      const x=streets.minX+(i+.5)*H,z=streets.minZ+(k+.5)*H,p=streetProfile(x,z);if(!p)continue;
      const y=top[i*NZ+k];
      if(y===-Infinity){gaps++;if(examples.length<10)examples.push(['gap',x,z]);continue;}
      const d=Math.abs(y-p.height);worst=Math.max(worst,d);if(d>.015){bad++;if(examples.length<10)examples.push([d.toFixed(3),x,z,p.kind]);}
    }
    assert.ok(gaps===0&&bad===0,`LOD${level}: gaps ${gaps}, over tolerance ${bad}, worst ${worst.toFixed(4)} m, e.g. ${JSON.stringify(examples)}`);
    console.log(`Street models LOD${level}: ${kitPlacements.length} placements match the profile (worst ${(worst*1000).toFixed(1)} mm).`);
  }
  ```
- [ ] Commit: `test(system-world): packed v3 street modules match the analytic street profile`.

**Acceptance:** zero gaps and zero cells over 1.5 cm at LOD 0, 1 and 2. If LOD 2 cannot meet this by design, the main session decides; see open questions.

## Task 14 — Documentation

**Files:** `assets/system-world/AGENTS.md`, `assets/system-world/README.md`, `ui/3d/system-world/AGENTS.md`.

**Content required (concise, existing style)**
- **`assets/system-world/AGENTS.md`**
  - Ownership: add `ui/3d/system-world/v3/`, written only by `scripts/build-system-world-v3.mjs`, and the raw scratch `disposable/system-world/v3-raw/`.
  - Local Contracts, kit v3:
    - authored by `kit3.py`, `street_assets3.py` and `build_world3.py`;
    - every primitive carries `COLOR_0` baked AO (greyscale, linear); node names equal component names; materials are `city.*` (new: `city.kerb`, `city.setts`, `city.iron`, `city.marking`, `city.tactile`, each mapped in `sysworld-surfaces.js` `KINDS`);
    - gltfpack `1.3.0 -cc -kn -km -ke -kv`, no simplification; three LODs keep the walking surface;
    - street modules follow `ui/js/desktop/apps/sysworld-ground.js` (`kerb`, `streetProfile`, `streetModules`) within 1.5 cm; corner arcs with at least 8 segments; `street-end` overlays the outer 0.64 m and its wall reaches −0.62 m;
    - the island top is at −0.2.
  - Provenance: all source hashes are over LF-normalised bytes.
  - The v1 `street-tile`/`street-crossing` are retired: drop them from `build_city.py` before the next v1 rebuild, or `check_assets.py` fails (15 designs).
  - Verification: add `node scripts/build-system-world-v3.mjs --check`, `node scripts/test-system-world-v3.mjs`, `node scripts/test-system-world-street-models.mjs` and `node scripts/test-system-world-ground.mjs`.
- **`README.md`**
  - Add a "Kit v3" section: run command `& 'D:/Blender 5.2/blender.exe' --background --factory-startup --python assets/system-world/build_world3.py`, then `node scripts/build-system-world-v3.mjs` and `--check`; the manifest schema 3 fields; the meshopt decoder in the renderer.
  - Runtime payload: v1 now has 15 designs and 45 GLBs.
  - Assets list: remove "street segment, crossing".
  - Integration contract: replace the "16 by 12 metres … stretch" bullet with the v3 module assembly (4 m plain/drain/zebra modules, 1 m fills, 12 × 12 crossings, end caps, never stretched).
- **`ui/3d/system-world/AGENTS.md`**
  - Ownership adds `v3/`.
  - "Preserve the 17 v1 designs" → 15.
  - Add the v3 rules: meshopt + `KHR_mesh_quantization`, loaded through `GLTFLoader.setMeshoptDecoder`; vertex-colour materials are never shared with v1/v2 (`sharedMaterialKey`); counted in the 40 MiB directory and the 20 MiB first display.
  - Verification commands.

**Steps**
- [ ] Edit the three files.
- [ ] `go test ./ui -run '^TestDesktopSysWorld' -count=1` → PASS (no doc-pinned strings expected; check that nothing breaks).
- [ ] Commit: `docs(system-world): kit v3 pipeline, street ground contract and retired v1 street tiles`.

## Task 15 — Markers, bundle, final regression, after-survey

**Files:** `ui/desktop_sysworld_test.go` (the `TestDesktopSysWorldAppMarkers` map, lines 86–102); `ui/js/vendor/system-world/*` (generated).

**Steps**
- [ ] Add these marker entries to the map, then run `gofmt -w ui/desktop_sysworld_test.go`:
  ```go
  		"js/desktop/apps/sysworld-ground.js":  {"export function streetProfile", "export function streetModules", "export function pedestrianGraph", "export function carriagewayFree", "export const kerb"},
  		"js/desktop/apps/sysworld-footing.js": {"export function springStep", "export function solveLeg", "export function adaptFeet", "export function findRig"},
  		"js/desktop/apps/sysworld-contact.js": {"export function createContactShadows", "dispose()"},
  ```
- [ ] `node scripts/build-system-world.js`, then `node scripts/build-system-world.js --check` → `verified`. The vendor `manifest.json` (bytes, sha256) is regenerated by the build.
- [ ] Final regression. All must pass. From PowerShell, run each `scripts/test-system-world*.mjs`:
  ```powershell
  Get-ChildItem scripts/test-system-world*.mjs | ForEach-Object { node $_.FullName; if ($LASTEXITCODE) { throw $_.Name } }
  node scripts/test-system-world-footing.mjs; node scripts/test-system-world-ground.mjs; node scripts/test-system-world-street-models.mjs
  python assets/system-world/check_assets.py
  python assets/system-world/build_textures.py --check
  node scripts/build-system-world-v3.mjs --check
  node scripts/build-system-world.js --check
  go test ./ui -run '^TestDesktopSysWorld' -count=1
  $env:AURAGO_RUN_BROWSER_SMOKE='1'; $env:AURAGO_SYSTEM_WORLD_MATRIX='1'; $env:AURAGO_SYSTEM_WORLD_EXPANSION='1'; go test ./ui -run '^TestDesktopAuroraBrowser$' -count=1 -timeout 30m
  ```
  - Bash equivalent of the browser run: `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_SYSTEM_WORLD_MATRIX=1 AURAGO_SYSTEM_WORLD_EXPANSION=1 go test ./ui -run '^TestDesktopAuroraBrowser$' -count=1 -timeout 30m`.
  - `node scripts/build-system-world-spot.mjs --check` only if the pinned source exists in `disposable/`.
- [ ] After-survey: `node scripts/system-world-survey.mjs --out reports/system-world-survey/phase-1 --tier high`. Compare with `baseline` for kerb-close, crossing, resident-feet-0/5/11 (sequences), night-street and rain-crossing. Report the before/after pairs.
- [ ] Report actual budget use: the model directory (from `test-system-world-expansion` / `build-system-world-v3 --check`), first display (`loadedBytes` from the browser report `world2-exploration.json`), and the whole app (`runtimeBytes`).
- [ ] Commit: `chore(system-world): rebuild renderer bundle and pin new module markers`.

**Acceptance:** every command above passes, the survey pictures are captured, and the budget figures are reported.

---

## Risks and open questions

1. **Blocked dependency.** Tasks 12 and 13 need the six v3 street modules.
   - The contract test checks **all three LODs** at 1.5 cm. If LOD 2 is meant to simplify the kerb geometry, the main session must decide whether LOD 2 is exempt. Residents only adapt their feet on medium and above, which use LOD 1 and LOD 0.
   - Corner arcs need at least 8 segments per 90°, and the flares (run 1.2 → 0.14 over 0.8 m) need fine triangulation, or the 1.5 cm check fails.
2. **Rig assumptions.** These rest on reading only: identity rest rotations for the robot parts, and the sole points taken from `robot_assets.py`. They are guarded by `findRig`'s rest check (silently off when wrong) and pinned by the living test's `rigs === 19`. If that fails on the real GLBs, the feet need per-model sole offsets instead of constants.
3. **Feet are kept at the animated orientation.** On the dropped-kerb ramps (about 1:8.6) the heel can float about 4 cm. Tilting the foot to the slope is a possible follow-up.
4. **Graph build and connectivity.** The pedestrian graph has 192 nodes. The one-time pure build is estimated at 0.1–0.3 s. Lamps and planters on the north pavements cut some straight links, so routes detour over the opposite pavement through zebras. Watch the living test's progress thresholds after Task 7.
5. **Generated files edited by hand.** The v2 manifest gets a patched hash and budgets (Task 1), and the v1 manifest loses two entries (Task 12). The next Blender rebuild regenerates the same values only if `build_city.py` drops the retired street tiles. That is main-session work.
6. **Before the payload ships.** Until v3 is delivered, every page load requests `/3d/system-world/v3/manifest.json` and gets a 404. The renderer ignores it, but Chrome logs a network error. I found no browser test that fails on console errors. If one does, commit an empty schema-3 manifest (`assets: []`) as a stop-gap.
7. **Bundle staleness.** `node scripts/build-system-world.js --check` is red from Task 3 until Task 15 by design.
8. **Known, accepted mismatches.**
   - The tram floor sits 0.49 m above the platform (corrections list).
   - The HUD map mirror still draws the x-streets to z = 80. This is out of scope: no HUD changes.
   - The companion plot is also crossed by resident links. The companion is scenery and is not in traffic, as today.
9. **Docks and planters moved.** The south and east docks and two planters move. If a later design wants docks next to zebras, the hover test's new 2 m node rule must be revisited.
