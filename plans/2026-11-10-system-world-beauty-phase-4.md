# System World Beautification — Phase 4 Implementation Plan (city life)

> **For agentic workers.** One code-writing agent per task, in the order given under "Order and parallelism". A test-running agent runs the commands listed in each task. Lean workflow: one combined review per batch, Critical and Important findings only.
>
> - **Agents do not change git state.** No commits, stashes, resets or branch switches. The coordinator commits each task with the message given at the end of it (last line `Co-Authored-By: Claude <noreply@anthropic.com>`).
> - **GitNexus is the coordinator's job.** Before handing a task out, the coordinator runs `impact` (upstream) on the functions it edits; `detect_changes` before each commit. Expected HIGH or UNKNOWN: `createTraffic`, `installCityColliders`, `createNavigator`, `createSurfaces`, `createAtmosphere`, `createExperience`, `createSociety`, `createCity`. This plan is the mitigation; its tests cover the callers.
> - **Scripts.** Throwaway scripts go into the session scratch directory via the Write tool, never into the repo. Bash heredocs mangle backslashes on this machine.
> - **Paths.** All paths are relative to the worktree `C:\Users\Andi\Documents\repo\AuraGo\.worktrees\sysworld-beauty`. Never Glob or Grep the main repository root.
> - **Anchors, not line numbers.** Every edit names a function and quotes the anchor text. If the text has drifted, make the same change to the equivalent statement and say so in your hand-back.

**Goal**
- The city looks inhabited: street trees and beds that move in the wind, a park, a market, two groves, a charging plaza and a harbour cafe on the empty plots, benches, bins, bollards and crossing signs along the streets, neon on facades, boats and buoys riding the sea, gulls over the harbour, steam from the foundry and fireflies in the green at night.
- Every new static piece comes from one pure rule module, and every new solid is real for the visitor, the residents, the hover pack and the street robots.

**Architecture**
- **`sysworld-furnishing.js` (new, pure).** The single source of all new static placements, as `{asset,x,z,y,angle,area}`:
  - rules: `GAP`, `SIZES` (collider footprints), `PAVEMENT`, `NEAR`, `RESERVED` (existing ground placements as boxes), `footprint`, `turnedBox`, `violations`;
  - generators: purpose areas, district extras, pavement pieces, street trees, neon (Task 8); `furnishing()` runs them through `violations` and caches the result;
  - `SURFACES` (lawn and paving rects), `PLACES` and `furnishingPlaces(catalog)` (charging plaza and cafe as resident workplaces).
- **Kit placements become real solids.** `sysworld-scene.js` appends `furnishing()` to `kitPlacements` (instanced by the existing batch code). `installCityColliders(…, kit)` reads each design's `navigation.colliders`, turned with the placement. `obstaclesFrom(placements, sizes)` derives robot circles from the same boxes. `createTraffic` and `createNavigator` get spatial buckets (design risk 1).
- **Ownership.**

  | Concern | Owner |
  | --- | --- |
  | Wind value (direction, strength, gusts) | `sysworld-atmosphere.js` (`windAt`, `wind()`), on the atmosphere clock |
  | Leaf sway (vertex shader, `city.leaf`, instanced draws only) | `sysworld-surfaces.js` (`SW_SWAY`, `setWind`) |
  | Banner cloth, near-gate, lawn/paving slabs, wiring of every new module (update, tier, reduced motion, dispose, stats) | `sysworld-scene.js` |
  | Instanced moving parts (pivoted nodes) | `sysworld-parts.js` (new) |
  | Boats, buoys, wave ride, keep-out | `sysworld-harbour.js` (new) |
  | Gulls | `sysworld-gulls.js` (new) |
  | Steam and fireflies | `sysworld-particles.js` (new) |
  | Charging plaza and cafe as workplaces | `sysworld-experience.js` (adds places), `sysworld-society.js` (no demonstrate prompt for them) |

- **Per frame** (in `update()`, after `const mood = weather.mood();`):
  1. `atmosphere.wind()` → `surfaces.setWind`, `swingBanners`;
  2. `harbour.update(seaTime, day, wind, animated)`;
  3. `gulls.update(seaTime, harbour.barge())`;
  4. `particles.update(dt, animated, wind)`.

  All four are allocation-free and driven by the atmosphere clock, which reduced motion freezes.

**Tech notes**
- three 0.186.1; kit v3 packed with `gltfpack -kn` keeps the part nodes (`rig`, `lamp`, `banner`, `wing_l`, `wing_r`), whose origin is the authored pivot (`kit3.py` translates the mesh by `-pivot` and sets `obj.location = pivot`).
- `angle` convention everywhere: world = (x·cos a + z·sin a, −x·sin a + z·cos a). The design front (glTF +Z) faces (sin a, cos a); the boat bow (+X) heads (cos h, −sin h).

---

## What I verified, and what I could not

**Verified by reading** (worktree at `fe6410de5`)
- **Grid.**
  - `streets`, `plotSlabs()` (25 plots + quay; bands as in the design), `zebras()` (64), `pedestrianGraph()` (192 nodes; links join every node pair under 48 m that stays off carriageways, so diagonal links cross plot interiors).
  - `streetProfile` gives `kind:'pavement'` for `d ≥ run`; `run` = 0.14 beyond 2.0 m from a zebra centre.
  - `streetSegments()` is exported.
- **Kit v3 manifest (31 designs).** Shipped furniture colliders (glTF `[x0,y0,z0,x1,y1,z1]`):
  - trees `±0.3` trunk to 3.19/2.52 m;
  - bench `[-1,0,-0.32,1,0.9,0.3]`;
  - bollard `±0.12`; bin `±0.3`; sign `±0.1`;
  - kiosk `[-1.6,0,-1.1,1.6,3,1.6]`;
  - lamp `±0.3`, planter `±1.75 × ±0.8`;
  - `data-tower-a` bounds x ±7.5, z −6.5..8.11.

  Material counts per GLB (LOD 0, read from the JSON chunk): tree 4, bench 3, bollard 2, bin 5, sign 3, kiosk 10.
- **Life source** (`life_assets3.py`): `collider()` maps Blender y to glTF z = −y; pivots for `rig (.7,0,1.25)`, `lamp (0,0,2.05)`, `banner (.14,0,6.55)`, `wing_l/r (0,±.06,.03)` (Blender axes). Every life design uses only existing `city.*` materials (bronze, titanium, graphite, ceramic, warm, ivory, cyan, jade, leaf, road, stone, kerb, setts, iron, marking, tactile), so the surfaces coverage test needs no change.
- **Runtime.**
  - `installCityColliders` reads v1 bounds for `placements` only.
  - `kitPlacements` is `streetModules()` only, and `test-system-world-street-models.mjs` relies on that.
  - `rebuild()` batches by mesh node (`batches.set(n.uuid, …)`).
  - `model(id, lod, 'v3')` shares materials by name; `obstaclesFrom` knows trams, racks, lamps and planters.
  - `createNavigator.hitsObstacle` scans every obstacle (the robot test runs 108 000 frames).
  - `traffic.obstruction` scans every solid and returns the first in Map insertion order.
  - The experience places v2 outdoors (gardens, arcades, bridge, pad, chargers, coolers, lanterns, quay, stations) with colliders from the v2 manifest.
  - `society.addPlace(place, entry)` makes slots at `place.x + p[0]`, without rotation; `nearby()` turns every non-lobby place into a `demonstrate` prompt; `machinery.demonstrate` ignores unknown ids.
- **Sea and lamps.**
  - `SEA.fragment` `height(p)` has four sine terms (not three), uses the `time` uniform, and shades a flat plane at y −3.2: the waves are normals only.
  - Atmosphere `update` advances `time` only when animated.
  - `test-system-world-lamps.mjs` already builds an atmosphere in Node.
- **Surfaces.** `KINDS['city.leaf']` exists. The `VERTEX_MAIN` injection runs after `#include <begin_vertex>` and knows `USE_INSTANCING`. `customProgramCacheKey` keys by kind, windows, space and level.
- **Phase 3 registry.** `LOBBIES` (rect halls, `front`, `ceiling`, `doors`), `PODIUMS.stairs`, `DOORS.z`, `lobbyGraph()` kinds `foot`/`porch`, and `LOBBIES[infra].towers` at (−50.5/−43/−35.5, −48), matching `cooling_tower` tops at 9.6 m and `roof_unit` tops at 14.22 m, z −57.5.
- **Parcel sorter** (backlog 13): v2 bounds x ±3.15, z −1.8..1.125 at (27.6, 49.4); missions stairs x 31.5..54.5, z 46.5..48.1; plot edge x 24, z 53.
- **Survey and browser checks.**
  - The survey `EXPOSE` anchor is `'model, catalogV3: () => catalog3, lobbies: lobbyRender, '`; the `overview-day` camera is at y 70.
  - The city browser test has anchors `'Lobby interiors must not load at the overview: '` and `'Low tier must keep the lobby stand-ins: '`.

**Could not verify**
1. Life GLBs (not built here): node names after packing, primitive counts per design, LOD triangles. Task 0 and the contract section of the furnishing test settle names; Task 12 measures draw calls.
2. v2 bounds of `kinetic-fountain`, `glass-garden`, `meeting-charge`, `data-monument`, `neon-gate`, `sky-pod`. `RESERVED` uses conservative boxes, and the furnishing test checks them against the v2 manifest.
3. Facade planes for neon mounts: the data-tower wall (bounds suggest ±6.5 with an 8.11 m entrance projection), the pavilion wall beside the door, and the lobby beam's outer face. The survey decides, within the tuning ranges given in Task 8.
4. The quay piece's edge detail at z ≈ 80 (mooring posts sit at z 79.2).
5. Resident throughput after ~600 new solids. The living simulation is chaotic; Tasks 3 and 10 re-run it.

## Deviations from the design

1. **Link clearance for pavement pieces** (decision 3).
   - The 0.5 m band behind the kerb (offset 3.44–3.94 m) lies 0.8–1.3 m from the pavement centre line (4.75 m), so a 0.6 m bin cannot keep 1.2 m to it.
   - Pavement pieces keep **0.6 m collider-to-line** (resident radius 0.5 plus gap 0.06 still passes) and 1.2 m to every node and zebra link.
   - Diagonal links across plot interiors are exempt: they cross every plot, buildings already cut most of them, and the wayfinder drops blocked links per revision.
2. **Strip rule** (decision 2). The origin of a street piece lies in the 0.4–2.4 m strip, and its collider lies ≥ 0.4 m inside the plot (the kiosk is 2.7 m deep). Purpose areas use their whole plot, with the same inset.
3. **Heave is scaled** (decision 7). The sea is flat geometry with shaded waves, so a hull riding the full ±0.8 m would separate from the water. Hulls heave with `HEAVE = 0.25 ×` the shader height and tilt with its full slope.
4. **Draw-call target** (risk 4). Thirty-one new designs at 2–10 materials each cannot stay within 60 extra calls with per-design instancing. This plan does three things:
   - hides `NEAR` designs (benches, bins, bollards, signs, cabinets, posts, beds, cafe sets, pylons, chargers) above 40 m camera height;
   - estimates about 75–85 extra calls at `overview-day` and about 120 at street level;
   - measures both in Task 12.

   If the coordinator holds the target, the follow-up is a per-material merged static layer (risk 2).
5. **Sway applies to instanced draws only.** Kit planters, trees, beds and v2 gardens sway. District clones (atrium sculpture, facade planters) do not: their leaves sit 6–15 m above the model origin.
6. **Charging plaza and cafe are "kit places"** (`kit:true`), not `livingPlaces` entries (those load v2 models). They get no approach grid.
   - Plaza slots come from the charge-post workpoint, pushed along +Z until a 0.56 m body clears the post's own collider.
   - `cafe-set` has no workpoints, so the cafe spots are runtime-defined.
7. **Neon mounts.**
   - Wide signs stand on the top beam of the six rectangular halls (the atrium has no room above its canopy).
   - Round signs hang on the pavilion wall beside the door.
   - Blade signs sit on the data-tower fronts: a 4 m blade is taller than a 3.55 m kiosk.
8. **Gulls.** Even gulls circle the harbour mouth, odd ones circle the barge (100–150 m out).
9. **Steam vents.** The three cooling towers, the three foundry roof units and five street drains: the only vents the registry and the district sources locate.
10. **Backlog 7** is solved by two runtime instanced slabs (lawn, paving; 0 bytes, 2 calls). **Backlog 13** moves the parcel sorter 2 m south.

## Requests to the coordinator

1. **`charge-post` collider is mirrored.** `collider(-.26, -.84, .26, -.4, 1.9)` puts the box at glTF z +0.4..+0.84, in front of the pad, while the pylon stands at glTF z −0.62. Change it to `collider(-.26, .4, .26, .84, 1.9)` before Task 0's build. The furnishing test's contract section asserts every charge-post collider has `maxZ ≤ 0`.
2. The manifest `components` of `sail-skiff`, `harbour-buoy`, `banner-mast` and `gull` must list `rig`, `lamp`, `banner`, `wing_l`/`wing_r` (the v3 test then proves the nodes survive packing).
3. A third tree or a hedge (design, Blender list) needs a `SIZES` entry and area slots. It is not planned; tell the executor if you add one.

---

## File map

| File | Status | Responsibility |
| --- | --- | --- |
| `ui/js/desktop/apps/sysworld-furnishing.js` | new | Rules, generators, `SURFACES`, places, neon |
| `ui/js/desktop/apps/sysworld-parts.js` | new | Instanced pivoted parts |
| `ui/js/desktop/apps/sysworld-harbour.js` | new | Boats, buoys, wave ride |
| `ui/js/desktop/apps/sysworld-gulls.js` | new | Flock |
| `ui/js/desktop/apps/sysworld-particles.js` | new | Steam and fireflies |
| `ui/js/desktop/apps/sysworld-traffic.js` | edit | Solid buckets |
| `ui/js/desktop/apps/sysworld-colliders.js` | edit | `kit` argument |
| `ui/js/desktop/apps/sysworld-navigation.js` | edit | `obstaclesFrom(…, sizes)`, obstacle buckets |
| `ui/js/desktop/apps/sysworld-scene.js` | edit | `streetPlacements`, `kitPlacements`, obstacles, near-gate, slabs, banners, wiring |
| `ui/js/desktop/apps/sysworld-atmosphere.js` | edit | `windAt`, `wind()`, `seaHeight`, `seaTime()` |
| `ui/js/desktop/apps/sysworld-surfaces.js` | edit | `SW_SWAY`, `setWind` |
| `ui/js/desktop/apps/sysworld-experience.js`, `sysworld-society.js`, `sysworld-places.js` | edit | Kit places, prompt filter, sorter move |
| `scripts/test-system-world-furnishing.mjs`, `-wind.mjs`, `-harbour.mjs`, `-gulls.mjs`, `-particles.mjs` | new | Tests |
| `scripts/test-system-world-{traffic,v3,hover,living,ground,lobbies,lobby-walk,street-models,surfaces}.mjs` | edit | See the pin table |
| `scripts/system-world-survey.mjs`, `.scenes.json` | edit | Expose and 13 scenes |
| `ui/desktop_sysworld_test.go`, `ui/desktop_sysworld_city_browser_test.go` | edit | Markers, checks |
| docs (`assets/system-world/AGENTS.md`, `README.md`, `ui/3d/system-world/AGENTS.md`, design roadmap row) | edit | Task 11 |
| `ui/js/vendor/system-world/*` | regenerate | Task 12 |
| `assets/system-world/build_world3.py`, `life_assets3.py`, `ui/3d/system-world/v3/*` | coordinator | Task 0 |

## Invariants every task must keep

- **Survey patch anchors.** `sysworld-scene.js` keeps `  return {\n    districts, canvas, update, focus(id)`; `sysworld-society.js` keeps `    add,patrol,update,suspend,\n`.
- **Go markers.** Every string in `TestDesktopSysWorldAppMarkers` stays.
- **Material ownership.**
  - `city.window` belongs to `sysworld-windows.js`; `city.pane` belongs to `sysworld-lobby-render.js`.
  - The new `lawn` and `paving` materials live in the scene's `matSet`, never in the shared `materials` map.
  - Harbour, gull and banner instances reuse cached kit materials and never dispose them.
- **First display stays under 20 MiB and grows little.**
  - At first display (`rebuild()`, the tier's LOD): all `kitPlacements` designs, which are the 6 street modules, the 7 shipped furniture designs and 10 static life designs (`flower-bed`, `cafe-set`, `charge-post`, `info-pylon`, `utility-cabinet`, `mooring-post`, `banner-mast`, `neon-blade`, `neon-wave`, `neon-ring`). Estimate +0.26 MiB at LOD 0.
  - Later (after `rebuild()` resolves, not in `expectedBytes`): `harbour-launch`, `harbour-barge`, `sail-skiff`, `harbour-buoy` (LOD 1, low LOD 2) and `gull` (LOD 1). Estimate +0.1 MiB.
- **No per-frame allocation** in `harbour.update`, `gulls.update`, `particles.update`, `swingBanners`, `atmosphere.update`'s wind.
- **Bundle staleness.** `node scripts/build-system-world.js --check` is red from Task 3 (the scene imports new modules) until Task 12.
- **Kit v3 optional per design.** `rebuild()` skips kit placements whose design is not in `catalog3`.

## Order and parallelism

| Step | Task | Notes |
| --- | --- | --- |
| A | 1 ∥ 2 | Disjoint files. 1 = furnishing module, its test, v3 pin prep. 2 = traffic buckets plus the draw-call baseline |
| B | **0** | Coordinator payload, after 1 |
| C | 3 | After 0 and 2: colliders, navigator, `kitPlacements`, test call sites |
| D | 4 ∥ 8 ∥ 10 | 4 = scene (gate, slabs). 8 = furnishing (neon) plus its test section. 10 = experience, society, places, living test |
| E | 5 | After 4: wind, sway, parts, banners |
| F | 6 | After 5: harbour |
| G | 7 | After 6: gulls |
| H | 9 | After 7: steam and fireflies |
| I | 11 | After 8, 9, 10: survey scenes plus docs |
| J | 12 | Markers, browser checks, final pins, bundle, regression, survey set, budgets, frame cost |

---

## Task 1 — `sysworld-furnishing.js` and the rule test

**Files:** new `ui/js/desktop/apps/sysworld-furnishing.js`, new `scripts/test-system-world-furnishing.mjs`, `scripts/test-system-world-v3.mjs`.

**Steps**
- [ ] Write the test. `node scripts/test-system-world-furnishing.mjs` → **FAIL** (module not found).
- [ ] Write the module. Run again → PASS, printing the counts line. Report the printed counts and how long `furnishing()` takes (add `console.time` in your scratch copy only).
- [ ] **If a `RESERVED` cover assertion fails** (a v2 bounds box is larger than its reserved box), widen that entry's local box to the printed value. If an explicit area item is rejected, report its violation; do not drop it.
- [ ] **`test-system-world-v3.mjs`.** Replace `assert.equal(manifest.assets.length, 31, '6 street + 12 superseded + 7 furniture designs + 6 lobby interiors = 31, found ' + manifest.assets.length);` with:
  ```js
  // Phase 4: the fifteen city-life designs ship together (Task 0) or not at all.
  const lifeIds = ['harbour-launch', 'harbour-barge', 'sail-skiff', 'harbour-buoy', 'mooring-post', 'flower-bed', 'cafe-set', 'charge-post', 'info-pylon', 'utility-cabinet', 'neon-blade', 'neon-wave', 'neon-ring', 'banner-mast', 'gull'];
  const lifeShipped = lifeIds.filter(id => ids.includes(id)).length;
  assert.ok(lifeShipped === 0 || lifeShipped === 15, 'Life designs ship together: ' + lifeShipped);
  assert.equal(manifest.assets.length, 31 + lifeShipped, '6 street + 12 superseded + 7 furniture + 6 lobby interiors (+ 15 life), found ' + manifest.assets.length);
  ```

### `ui/js/desktop/apps/sysworld-furnishing.js` (complete for Task 1)

```js
// Pure placement rules for the furnished city (System World beauty pass, phase 4). Every new static placement is
// {asset,x,z,y,angle,area}; world metres, Y up; `angle` turns about +Y as in sysworld-scene.js, so the design's
// front (glTF +Z) faces (sin a, cos a). SIZES mirror each design's navigation.colliders footprint (glTF, model-local
// [x0,z0,x1,z1]); RESERVED mirrors the static ground placements of sysworld-experience.js init() and of
// sysworld-scene.js `placements`. scripts/test-system-world-furnishing.mjs checks both and every rule.
import {streets,surfaces,stations,hoverPads,towers,dronePad,interiors,tramWaypoints} from './sysworld-layout.js';
import {kerb,zebras,pedestrianGraph,plotSlabs,streetProfile,streetSegments} from './sysworld-ground.js';
import {PODIUMS,LOBBIES,DOORS,FLIGHT,lobbyGraph} from './sysworld-lobbies.js';
import {livingPlaces,landmarkPlaces} from './sysworld-places.js';

export const GAP=Object.freeze({node:1.2,line:1.2,pavementLine:.6,dock:3,rail:2.5,platform:2.5,stair:2,door:2,podium:.5,solid:1,living:3,crown:4.5,zebra:2.1,inset:.4,behindKerb:.5});
export const TREE_STEP=12,TREE_INSET=1.4,NEAR_HEIGHT=40,RAIL_HALF=.9;
export const SIZES=Object.freeze({'tree-round':[-.3,-.3,.3,.3],'tree-column':[-.3,-.3,.3,.3],'street-bench':[-1,-.32,1,.3],bollard:[-.12,-.12,.12,.12],
  'litter-bin':[-.3,-.3,.3,.3],'zebra-sign':[-.1,-.1,.1,.1],kiosk:[-1.6,-1.1,1.6,1.6],'flower-bed':[-1.6,-.7,1.6,.7],'cafe-set':[-1,-1,1,1],
  'charge-post':[-.26,-.84,.26,-.4],'info-pylon':[-.48,-.22,.48,.22],'utility-cabinet':[-.65,-.25,.65,.25],'mooring-post':[-.3,-.3,.3,.3],'banner-mast':[-.25,-.25,.25,.25]});
export const PAVEMENT=new Set(['bollard','litter-bin','zebra-sign']);
// Small pieces that sysworld-scene.js hides above NEAR_HEIGHT: sub-pixel from the orbit views, one draw call each.
export const NEAR=new Set([...PAVEMENT,'street-bench','utility-cabinet','mooring-post','cafe-set','charge-post','info-pylon','flower-bed']);
export const LIFE_DESIGNS=Object.freeze(['harbour-launch','harbour-barge','sail-skiff','harbour-buoy','mooring-post','flower-bed','cafe-set','charge-post','info-pylon','utility-cabinet','neon-blade','neon-wave','neon-ring','banner-mast','gull']);
const PI=Math.PI,face={north:PI,south:0,east:PI/2,west:-PI/2},{xs,zs,halfWidth}=streets,P=kerb.pavementLine;
export const gap=(a,b)=>Math.hypot(Math.max(0,b[0]-a[2],a[0]-b[2]),Math.max(0,b[1]-a[3],a[1]-b[3]));
// World AABB of a local [x0,z0,x1,z1] box turned by `angle` and placed at (x,z).
export function turnedBox(local,x,z,angle=0){
  const c=Math.cos(angle),s=Math.sin(angle);let x0=Infinity,z0=Infinity,x1=-Infinity,z1=-Infinity;
  for(const lx of[local[0],local[2]])for(const lz of[local[1],local[3]]){const wx=x+lx*c+lz*s,wz=z-lx*s+lz*c;x0=Math.min(x0,wx);x1=Math.max(x1,wx);z0=Math.min(z0,wz);z1=Math.max(z1,wz);}
  return [x0,z0,x1,z1];
}
export const footprint=item=>turnedBox(SIZES[item.asset]||[0,0,0,0],item.x,item.z,item.angle);
export const LAMPS=zs.flatMap(z=>[-55,-31,31,55].map(x=>({x,z:z-4.5})));
const planterX=x=>x+4*Math.sign(Math.sign(x)*43-x);
const res=(id,asset,x,z,angle,local,kind='solid')=>({id,asset,x,z,angle,kind,local,box:turnedBox(local,x,z,angle)});
export const RESERVED=Object.freeze([
  ...[[-79,68],[-79,-67],[55,4],[8,40],[45,-23]].flatMap(([x,z],i)=>[res('garden-'+i,'garden',x,z,0,[-2,-2,2,2]),res('garden-bench-'+i,'bench',x+3,z,PI/2,[-1.5,-.42,1.5,.4])]),
  ...[[-26,23],[30,23],[-28,72]].map(([x,z],i)=>res('arcade-'+i,'arcade',x,z,0,[-3.25,-1.2,3.25,1.2])),
  res('bridge','bridge',-78.5,40,0,[-3.61,-6,3.61,6]),res('ramp-north','ramp',-80,49,PI,[-1.54,-3.01,1.54,3.01]),
  res('stairs-north','stairs',-76.8,49,PI,[-1.54,-3.01,1.54,3.01]),res('ramp-south','ramp',-78.5,31,0,[-1.54,-3.01,1.54,3.01]),
  res('drone-pad','pad',dronePad.x,dronePad.z,0,[-4,-4,4,4]),res('service-cart','service-cart',55,51.2,PI/2,[-1.5,-1.5,1.5,1.63]),
  ...[[-60,-56],[-60,-46],[51,51]].map(([x,z],i)=>res('charger-'+i,'charger',x,z,0,[-.35,-.25,.65,.28])),
  ...[[-59,-61],[-59,-52]].map(([x,z],i)=>res('cooler-'+i,'cooler',x,z,0,[-1.5,-1.27,1.5,1.27])),
  ...[-68,-52,-28,28,52,68].map((x,i)=>res('quay-lantern-'+i,'quay-lantern',x,75.5,0,[-.34,-.34,.34,.34])),
  ...stations.map(s=>res('station-'+s.id,'station',s.platformX,s.platformZ,s.angle,[-4.5,-2,4.5,2],'platform')),
  ...interiors.map(r=>res('pavilion-'+r.id,null,r.x,r.z,0,[-6.2,-6.2,6.2,6.2])),
  ...livingPlaces.map(p=>res('living-'+p.id,p.id,p.x,p.z,0,[-4,-4,4,4],'living')),
  ...landmarkPlaces.filter(l=>l.y>=0).map(l=>res('landmark-'+l.id,l.id,l.x,l.z,l.angle||0,l.id==='neon-gate'?[-7,-3,7,3]:[-6,-6,6,6])),
  ...LAMPS.flatMap((p,i)=>[res('lamp-'+i,'street-lamp',p.x,p.z,0,[-.3,-.3,.3,.3]),res('planter-'+i,'planter',planterX(p.x),p.z,0,[-1.78,-.85,1.83,1])]),
  ...[-55,-49].map((x,i)=>res('rack-'+i,'server-rack',x,-36.7,0,[-.7,-.6,.7,.6])),
  ...towers.map((t,i)=>res('tower-'+i,'data-tower-a',t.x,t.z,0,[-7.5,-6.5,7.5,8.11])),
]);
const ZEBRAS=zebras();
const LINES=[...zs.flatMap(z=>[-P,P].map(o=>[streets.minX,z+o,streets.maxX,z+o])),...xs.flatMap(x=>[-P,P].map(o=>[x+o,streets.minZ,x+o,streets.quayZ]))];
const ZEBRA_LINKS=ZEBRAS.map(zb=>zb.axis==='x'?[zb.at,zb.line-P,zb.at,zb.line+P]:[zb.line-P,zb.at,zb.line+P,zb.at]);
const RAILS=tramWaypoints.map((p,i)=>{const q=tramWaypoints[(i+1)%tramWaypoints.length];return [Math.min(p[0],q[0]),Math.min(p[1],q[1]),Math.max(p[0],q[0]),Math.max(p[1],q[1])];});
const NODES=[...pedestrianGraph().nodes,...lobbyGraph().nodes.filter(n=>n.kind==='foot'||n.kind==='porch')].map(n=>[n.x,n.z,n.x,n.z]);
const STAIRS=PODIUMS.flatMap(p=>p.stairs.map(f=>[f.x-f.width/2,p.z1,f.x+f.width/2,p.z1+FLIGHT]));
const DOOR_BOXES=DOORS.map(d=>[d.x-d.width/2,d.z-.6,d.x+d.width/2,d.z+.6]);
const PODIUM_BOXES=PODIUMS.map(p=>[p.x0,p.z0,p.x1,p.z1]);
const SLABS=plotSlabs().map(s=>[s.x0+GAP.inset,s.z0+GAP.inset,s.x1-GAP.inset,s.z1-GAP.inset]);
const DOCKS=hoverPads.map(p=>[p.x,p.z,p.x,p.z]);
const inside=(f,s)=>f[0]>=s[0]-1e-9&&f[1]>=s[1]-1e-9&&f[2]<=s[2]+1e-9&&f[3]<=s[3]+1e-9;
function onPavement(item,f){
  for(const [x,z]of[[f[0],f[1]],[f[2],f[1]],[f[0],f[3]],[f[2],f[3]]]){const p=streetProfile(x,z);if(!p||p.kind!=='pavement')return false;}
  if(xs.some(x=>Math.abs(item.x-x)<halfWidth+1)&&zs.some(z=>Math.abs(item.z-z)<halfWidth+1))return false;
  for(const zb of ZEBRAS){const across=zb.axis==='x'?Math.abs(item.z-zb.line):Math.abs(item.x-zb.line),along=zb.axis==='x'?Math.max(f[0]-zb.at,zb.at-f[2],0):Math.max(f[1]-zb.at,zb.at-f[3],0);
    if(across<=halfWidth&&along<GAP.zebra)return false;}
  const p=streetProfile(item.x,item.z);return p.d-p.run<=GAP.behindKerb+1e-9;
}
// Rule names an item breaks (empty: it fits). `placed` holds the footprints of the pieces accepted before it.
export function violations(item,placed=[]){
  const f=footprint(item),pave=PAVEMENT.has(item.asset),out=[];
  if(pave?!onPavement(item,f):!SLABS.some(s=>inside(f,s)))out.push('ground');
  if(NODES.some(n=>gap(n,f)<GAP.node))out.push('node');
  if(LINES.some(l=>gap(l,f)<(pave?GAP.pavementLine:GAP.line))||ZEBRA_LINKS.some(l=>gap(l,f)<GAP.line))out.push('line');
  if(DOCKS.some(d=>gap(d,f)<GAP.dock))out.push('dock');
  if(RAILS.some(r=>gap(r,f)<GAP.rail+RAIL_HALF))out.push('rail');
  if(STAIRS.some(s=>gap(s,f)<GAP.stair)||DOOR_BOXES.some(d=>gap(d,f)<GAP.door))out.push('stair');
  if(PODIUM_BOXES.some(p=>gap(p,f)<GAP.podium))out.push('podium');
  for(const r of RESERVED)if(gap(r.box,f)<(r.kind==='platform'?GAP.platform:r.kind==='living'?GAP.living:GAP.solid))out.push(r.id);
  if(placed.some(g=>gap(g,f)<GAP.solid))out.push('solid');
  if(/^tree-/.test(item.asset)&&LAMPS.some(p=>Math.hypot(p.x-item.x,p.z-item.z)<GAP.crown))out.push('crown');
  return out;
}
const item=(asset,x,z,angle,area,y=0)=>({asset,x,z,y,angle,area});
// Empty plots with a purpose (decision 5), keyed by their (x0, z0); street trees and cabinets skip them.
export const PURPOSE_PLOTS=Object.freeze([[-61,-94],[24,-94],[-85,-94],[73,-94],[73,-71],[73,65]]);
const purpose=s=>PURPOSE_PLOTS.some(([x,z])=>x===s.x0&&z===s.z0);
function areaItems(){
  const out=[];
  // Linear park (x -61..-24, z -94..-83): two tree rows clear of the lamp crowns, three beds, benches facing them.
  for(const x of[-59,-50.5,-44,-37.5,-26.5])out.push(item('tree-round',x,-84.6,x*.7,'park'));
  [-58,-51.5,-45,-38.5,-32,-25.5].forEach((x,i)=>out.push(item(i%2?'tree-round':'tree-column',x,-92.4,x*.9,'park')));
  for(const x of[-53,-42.5,-32])out.push(item('flower-bed',x,-88.5,0,'park'));
  for(const x of[-48,-37.5])out.push(item('street-bench',x,-86.5,face.north,'park'),item('street-bench',x,-90.5,face.south,'park'));
  // Market (x 24..61, z -94..-83): three kiosks facing the street, cafe sets behind them, banner masts.
  for(const x of[29.5,42.5,55.5])out.push(item('kiosk',x,-85,face.south,'market'));
  [28,35,49,56].forEach((x,i)=>out.push(item('cafe-set',x,-89.6,i*PI/2,'market')));
  for(const [x,z]of[[36,-84.4],[49,-84.4],[25.5,-92.5],[59.5,-92.5]])out.push(item('banner-mast',x,z,0,'market'));
  // Groves in both north corners (x +-79, z -88.5).
  for(const s of[-1,1]){
    for(const [x,z,asset]of[[76.2,-84.6,'tree-round'],[81.8,-84.6,'tree-round'],[79,-88.5,'tree-column'],[76.2,-92.4,'tree-round'],[81.8,-92.4,'tree-column']])out.push(item(asset,s*x,z,x+z,'grove'));
    out.push(item('street-bench',s*79,-85.2,face.south,'grove'));
  }
  // Charging plaza (x 73..85, z -71..-38): four posts facing the street, a pylon, two beds, a tree row.
  for(const z of[-64,-58,-52,-46])out.push(item('charge-post',74.6,z,face.west,'plaza'));
  out.push(item('info-pylon',77.5,-40.5,0,'plaza'));
  for(const z of[-62,-54])out.push(item('flower-bed',79.5,z,PI/2,'plaza'));
  [-67,-59,-51,-43].forEach((z,i)=>out.push(item(i%2?'tree-column':'tree-round',82.2,z,z,'plaza')));
  // Harbour cafe terrace (x 73..85, z 65..74).
  for(const [x,z,a]of[[75.6,67.4,0],[81,67.4,PI/2],[78.3,71.4,PI]])out.push(item('cafe-set',x,z,a,'cafe'));
  out.push(item('flower-bed',83.9,70.6,PI/2,'cafe'),item('tree-column',84,66,1,'cafe'));
  // Mooring posts along the quay edge, clear of the pavilions, the lanterns and the harbour dock.
  for(const x of[-80,-62,-34,-14,14,34,60,80])out.push(item('mooring-post',x,79.2,0,'quay',surfaces.quay));
  return out;
}
// District plots: beds beside every stair flight, benches facing it, a cabinet per block, a pylon at every station.
const PYLONS=[[-75.5,-60.5,PI/2],[-75.5,-17.5,PI/2],[-34.8,67,0],[7.8,67,0],[34.8,67,0],[75.5,-2.5,-PI/2],[7.5,-85.5,PI]];
function districtExtras(){
  const out=[];
  for(const p of PODIUMS)for(const f of p.stairs)for(const s of[-1,1])
    out.push(item('flower-bed',f.x+s*(f.width/2+GAP.stair+1.7),p.z1+1.25,0,'district'),item('street-bench',f.x+s*(f.width/2+3.2),p.z1+FLIGHT+3.2,face.north,'district'));
  for(const s of plotSlabs()){
    if(s.kind!=='plot'||s.x1-s.x0<20||purpose(s))continue;
    const north=zs.some(z=>Math.abs(s.z0-(z+halfWidth))<1e-6);
    out.push(item('utility-cabinet',s.x1-4,north?s.z0+.9:s.z1-.9,north?face.north:face.south,'district'));
  }
  for(const [x,z,a]of PYLONS)out.push(item('info-pylon',x,z,a,'station'));
  return out;
}
// Pavement: two bollards beside each dropped kerb and a crossing sign (away from the junction), one bin per block side.
function pavementPieces(){
  const out=[];
  for(const zb of ZEBRAS){
    const crossing=(zb.axis==='x'?xs:zs).reduce((a,c)=>Math.abs(c-zb.at)<Math.abs(a-zb.at)?c:a),away=Math.sign(zb.at-crossing);
    const at=(s,side,o)=>zb.axis==='x'?{x:zb.at+away*s,z:zb.line+side*o}:{x:zb.line+side*o,z:zb.at+away*s};
    for(const side of[-1,1]){const p=at(2.4,side,3.65);out.push(item('bollard',p.x,p.z,0,'pavement'));}
    const sign=at(3.8,1,3.65);out.push(item('zebra-sign',sign.x,sign.z,zb.axis==='x'?PI/2:0,'pavement'));
  }
  for(const seg of streetSegments())if(seg.start==='crossing'&&seg.end==='crossing'){
    const mid=(seg.from+seg.to)/2;
    out.push([1,-1].map(side=>seg.axis==='x'?item('litter-bin',mid,seg.line+side*3.82,0,'pavement'):item('litter-bin',seg.line+side*3.82,mid,0,'pavement')));
  }
  return out;
}
// A tree every TREE_STEP metres, TREE_INSET inside every street-facing plot edge (corners included).
function streetTrees(){
  const out=[];let k=0;
  for(const s of plotSlabs()){
    if(s.kind!=='plot'||purpose(s))continue;
    const edges=[];
    if(zs.some(z=>Math.abs(s.z0-(z+halfWidth))<1e-6))edges.push([s.z0+TREE_INSET,true]);
    if(zs.some(z=>Math.abs(s.z1-(z-halfWidth))<1e-6))edges.push([s.z1-TREE_INSET,true]);
    if(xs.some(x=>Math.abs(s.x0-(x+halfWidth))<1e-6))edges.push([s.x0+TREE_INSET,false]);
    if(xs.some(x=>Math.abs(s.x1-(x-halfWidth))<1e-6))edges.push([s.x1-TREE_INSET,false]);
    for(const [at,alongX]of edges){const lo=alongX?s.x0:s.z0,hi=alongX?s.x1:s.z1;
      for(let t=lo+TREE_INSET;t<=hi-TREE_INSET+1e-9;t+=TREE_STEP){out.push(item(k%3===2?'tree-column':'tree-round',alongX?t:at,alongX?at:t,(k*2.39996)%(2*PI),'street'));k++;}}
  }
  return out;
}
let cache=null;
// Deterministic and cached: areas first (all must fit), then district extras, pavement pieces and street trees.
// An array candidate lists alternatives; the first that fits is taken.
export function furnishing(){
  if(cache)return cache;
  const accepted=[],boxes=[];
  for(const c of[...areaItems(),...districtExtras(),...pavementPieces(),...streetTrees()])
    for(const it of Array.isArray(c)?c:[c])if(!violations(it,boxes).length){accepted.push(Object.freeze(it));boxes.push(footprint(it));break;}
  return cache=Object.freeze(accepted);
}
// Lawn and paving slabs over the purpose plots (backlog item 7), 0.2 m inside the plot edges.
export const SURFACES=Object.freeze([
  {kind:'lawn',x0:-60.8,z0:-93.8,x1:-24.2,z1:-83.2},{kind:'lawn',x0:-84.8,z0:-93.8,x1:-73.2,z1:-83.2},{kind:'lawn',x0:73.2,z0:-93.8,x1:84.8,z1:-83.2},
  {kind:'paving',x0:24.2,z0:-93.8,x1:60.8,z1:-83.2},{kind:'paving',x0:73.2,z0:-70.8,x1:84.8,z1:-38.2},{kind:'paving',x0:73.2,z0:65.2,x1:84.8,z1:73.8},
]);
export const PLACES=Object.freeze([
  Object.freeze({id:'charge-plaza',district:'integrations',role:'technician',x:79,z:-55}),
  Object.freeze({id:'harbour-cafe',district:'missions',role:'archivist',x:79,z:69.5}),
]);
const CAFE_POINTS=[['serve-0',78.3,67.4],['serve-1',75.4,70.4]];
const entryOf=(catalog,id)=>catalog instanceof Map?catalog.get(id):catalog?.assets?.find(a=>a.id===id);
// A standing spot for a resident body (0.5 m plus the 0.06 m traffic gap) at a workpoint: pushed along the design's
// +Z (its front) until it clears the design's own colliders.
export function standing(entry,w,clear=.56){
  const boxes=entry.navigation?.colliders||[],x=w.position[0];let z=w.position[2];
  for(let k=0;k<40&&boxes.some(b=>x>b[0]-clear&&x<b[3]+clear&&z>b[2]-clear&&z<b[5]+clear);k++)z+=.05;
  return [x,z];
}
// Society places for the charging plaza and the harbour cafe (kit:true: no machinery, no demonstrate prompt).
export function furnishingPlaces(catalog){
  const out=[],[plaza,cafe]=PLACES,post=entryOf(catalog,'charge-post');
  if(post?.navigation?.workpoints?.length){
    const workpoints=[];
    furnishing().filter(f=>f.area==='plaza'&&f.asset==='charge-post').forEach((f,i)=>{const c=Math.cos(f.angle),s=Math.sin(f.angle);
      for(const w of post.navigation.workpoints){const [lx,lz]=standing(post,w);workpoints.push({id:w.id+'-'+i,position:[f.x+lx*c+lz*s-plaza.x,0,f.z-lx*s+lz*c-plaza.z]});}});
    out.push({place:{...plaza,kit:true},entry:{navigation:{workpoints}}});
  }
  if(entryOf(catalog,'cafe-set'))out.push({place:{...cafe,kit:true},entry:{navigation:{workpoints:CAFE_POINTS.map(([id,x,z])=>({id,position:[x-cafe.x,0,z-cafe.z]}))}}});
  return out;
}
```

### `scripts/test-system-world-furnishing.mjs` (complete for Task 1)

```js
// Phase 4 furnishing: deterministic placements, every clearance of design decision 3 (checked independently of
// violations()), the purpose areas, SIZES and RESERVED against the manifests, and the life contract once shipped.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import * as F from '../ui/js/desktop/apps/sysworld-furnishing.js';
import {streets,stations,hoverPads,tramWaypoints} from '../ui/js/desktop/apps/sysworld-layout.js';
import {kerb,zebras,pedestrianGraph,plotSlabs,streetProfile,onCarriageway} from '../ui/js/desktop/apps/sysworld-ground.js';
import {PODIUMS,DOORS,FLIGHT,lobbyGround,lobbyGraph} from '../ui/js/desktop/apps/sysworld-lobbies.js';
import {streetCurve} from '../ui/js/desktop/apps/sysworld-navigation.js';

const read=f=>fs.readFile(f,'utf8').then(JSON.parse),v1=await read('ui/3d/system-world/v1/manifest.json'),v2=await read('ui/3d/system-world/v2/manifest.json'),v3=await read('ui/3d/system-world/v3/manifest.json');
const v3ids=new Map(v3.assets.map(a=>[a.id,a])),{gap,footprint}=F,items=F.furnishing(),ground=items.filter(i=>F.SIZES[i.asset]);
assert.equal(F.furnishing(),items,'cached');assert.ok(Object.isFrozen(items));
for(const i of items)assert.ok([i.x,i.y,i.z,i.angle].every(Number.isFinite)&&typeof i.asset==='string'&&i.area,'bad item '+JSON.stringify(i));
const count=(area,asset)=>items.filter(i=>i.area===area&&(!asset||i.asset===asset)).length;
// Purpose areas (decision 5): every explicit piece fits.
assert.deepEqual([count('park','tree-round')+count('park','tree-column'),count('park','flower-bed'),count('park','street-bench')],[11,3,4],'park');
assert.deepEqual([count('market','kiosk'),count('market','cafe-set'),count('market','banner-mast')],[3,4,4],'market');
assert.equal(count('grove'),12,'groves');assert.deepEqual([count('plaza','charge-post'),count('plaza','info-pylon'),count('plaza')],[4,1,11],'plaza');
assert.deepEqual([count('cafe','cafe-set'),count('cafe')],[3,5],'cafe');assert.equal(count('quay','mooring-post'),8);assert.equal(count('station','info-pylon'),7);
// Rule-made pieces: lower bounds only (the hand-back reports the real numbers).
assert.ok(count('street')>=120,'street trees '+count('street'));assert.ok(count('pavement','zebra-sign')>=52,'signs '+count('pavement','zebra-sign'));
assert.ok(count('pavement','bollard')>=100,'bollards '+count('pavement','bollard'));assert.ok(count('pavement','litter-bin')>=20,'bins '+count('pavement','litter-bin'));
assert.ok(count('district','flower-bed')>=6&&count('district','street-bench')>=6&&count('district','utility-cabinet')>=6,'district furnishing');
// Clearances, each computed here from the ground, graph and registry modules.
const P=kerb.pavementLine,pt=(x,z)=>[x,z,x,z];
const lines=[...streets.zs.flatMap(z=>[-P,P].map(o=>[streets.minX,z+o,streets.maxX,z+o])),...streets.xs.flatMap(x=>[-P,P].map(o=>[x+o,streets.minZ,x+o,streets.quayZ]))];
const zebraLinks=zebras().map(zb=>zb.axis==='x'?[zb.at,zb.line-P,zb.at,zb.line+P]:[zb.line-P,zb.at,zb.line+P,zb.at]);
const nodes=[...pedestrianGraph().nodes,...lobbyGraph().nodes.filter(n=>n.kind==='foot'||n.kind==='porch')];
const curve=streetCurve(tramWaypoints),rail=[];
for(let i=0;i<2000;i++){const p=curve.getPointAt(i/2000),t=curve.getTangentAt(i/2000);for(const s of[-.9,.9])rail.push(pt(p.x-t.z*s,p.z+t.x*s));}
const platforms=stations.map(s=>F.turnedBox([-4.5,-2,4.5,2],s.platformX,s.platformZ,s.angle));
const stairs=PODIUMS.flatMap(p=>p.stairs.map(f=>[f.x-f.width/2,p.z1,f.x+f.width/2,p.z1+FLIGHT])),slabs=plotSlabs();
for(const it of ground){
  const f=footprint(it),name=JSON.stringify(it),pave=F.PAVEMENT.has(it.asset),corners=[[f[0],f[1]],[f[2],f[1]],[f[0],f[3]],[f[2],f[3]]];
  if(pave){
    for(const [x,z]of corners){const p=streetProfile(x,z);assert.ok(p&&p.kind==='pavement',name+' off the pavement at '+x+','+z);}
    const p=streetProfile(it.x,it.z);assert.ok(p.d-p.run<=.5+1e-9,name+' more than 0.5 m behind the kerb');
    for(const zb of zebras()){const across=zb.axis==='x'?Math.abs(it.z-zb.line):Math.abs(it.x-zb.line),along=zb.axis==='x'?Math.max(f[0]-zb.at,zb.at-f[2],0):Math.max(f[1]-zb.at,zb.at-f[3],0);
      assert.ok(across>streets.halfWidth||along>=2.1-1e-9,name+' on a zebra, dropped kerb or tactile strip');}
  }else assert.ok(slabs.some(s=>f[0]>=s.x0+.4-1e-9&&f[2]<=s.x1-.4+1e-9&&f[1]>=s.z0+.4-1e-9&&f[3]<=s.z1-.4+1e-9),name+' not 0.4 m inside a plot');
  for(const [x,z]of[...corners,[it.x,it.z]]){assert.equal(onCarriageway(x,z),false,name+' on a carriageway');assert.equal(lobbyGround(x,z),null,name+' on a podium or stair');}
  for(const n of nodes)assert.ok(gap(pt(n.x,n.z),f)>=1.2-1e-9,name+' within 1.2 m of node '+n.x+','+n.z);
  for(const l of lines)assert.ok(gap(l,f)>=(pave?.6:1.2)-1e-9,name+' too close to a pavement line');
  for(const l of zebraLinks)assert.ok(gap(l,f)>=1.2-1e-9,name+' within 1.2 m of a zebra link');
  for(const d of hoverPads)assert.ok(gap(pt(d.x,d.z),f)>=3-1e-9,name+' within 3 m of dock '+d.id);
  assert.ok(rail.every(r=>gap(r,f)>=2.5-1e-9),name+' within 2.5 m of a rail');
  for(const b of platforms)assert.ok(gap(b,f)>=2.5-1e-9,name+' within 2.5 m of a platform');
  for(const s of stairs)assert.ok(gap(s,f)>=2-1e-9,name+' within 2 m of stairs');
  for(const d of DOORS)assert.ok(gap([d.x-d.width/2,d.z,d.x+d.width/2,d.z],f)>=2-1e-9,name+' within 2 m of a door');
  for(const r of F.RESERVED)assert.ok(gap(r.box,f)>=(r.kind==='living'?3:1)-1e-9,name+' too close to '+r.id);
}
const fps=ground.map(footprint);
for(let i=0;i<fps.length;i++)for(let j=i+1;j<fps.length;j++)assert.ok(gap(fps[i],fps[j])>=1-1e-9,'colliders closer than 1 m: '+JSON.stringify([ground[i],ground[j]]));
// SIZES mirror the manifest colliders; RESERVED boxes cover the real footprints (colliders, else LOD 0 bounds).
const union=c=>[Math.min(...c.map(x=>x[0])),Math.min(...c.map(x=>x[2])),Math.max(...c.map(x=>x[3])),Math.max(...c.map(x=>x[5]))];
for(const [id,size]of Object.entries(F.SIZES)){const a=v3ids.get(id);if(!a){assert.ok(F.LIFE_DESIGNS.includes(id),id+' is not shipped');continue;}
  union(a.navigation.colliders).forEach((v,k)=>assert.ok(Math.abs(v-size[k])<.011,id+' SIZES differ from its colliders'));}
const lookup=id=>v3ids.get(id)||v2.assets.find(a=>a.id===id)||v1.assets.find(a=>a.id===id);
for(const r of F.RESERVED){if(!r.asset)continue;const a=lookup(r.asset);assert.ok(a,r.id+': no asset '+r.asset);
  const c=a.navigation?.colliders?.length?a.navigation.colliders:null,b=a.lods[0].bounds,local=c?union(c):[b.min[0],b.min[2],b.max[0],b.max[2]];
  if(r.asset==='bridge'){local[0]*=1.75;local[2]*=1.75;}
  const need=F.turnedBox(local,r.x,r.z,r.angle);assert.ok(need[0]>=r.box[0]-.01&&need[1]>=r.box[1]-.01&&need[2]<=r.box[2]+.01&&need[3]<=r.box[3]+.01,`${r.id} box ${r.box} must cover ${need.map(v=>v.toFixed(2))}`);}
// Life designs (Task 0): ship together; parts; the charge post's pylon stands behind its pad; places.
const life=F.LIFE_DESIGNS.filter(id=>v3ids.has(id));
assert.ok(life.length===0||life.length===F.LIFE_DESIGNS.length,'Life designs ship together: '+life);
if(life.length){
  for(const [id,parts]of[['sail-skiff',['rig']],['harbour-buoy',['lamp']],['banner-mast',['banner']],['gull',['wing_l','wing_r']]])for(const p of parts)assert.ok(v3ids.get(id).components.includes(p),id+' part '+p);
  const post=v3ids.get('charge-post');assert.deepEqual(post.navigation.workpoints.map(w=>w.id),['charge']);
  assert.ok(post.navigation.colliders.every(b=>b[5]<=0),'charge-post collider must stand behind the pad (glTF -Z): '+JSON.stringify(post.navigation.colliders));
  const places=F.furnishingPlaces(v3ids);assert.deepEqual(places.map(p=>p.place.id),['charge-plaza','harbour-cafe']);
  assert.equal(places[0].entry.navigation.workpoints.length,4);assert.ok(places.every(p=>p.place.kit));
}
console.log(`System World furnishing: ${items.length} pieces (${count('street')} street trees, ${count('pavement')} pavement, ${count('district')+count('station')} district/station, `
  +`${items.length-count('street')-count('pavement')-count('district')-count('station')} in purpose areas); clearances, sizes and reserved boxes verified; life designs `+(life.length?'shipped':'not shipped yet')+'.');
```

**Run**
- `node scripts/test-system-world-furnishing.mjs` → PASS (`life designs not shipped yet`).
- `node scripts/test-system-world-v3.mjs` → PASS (31 designs).
- `node --check ui/js/desktop/apps/sysworld-furnishing.js` → no output.

**Acceptance:** the rules hold on the result; no runtime file imports the module yet.

**Commit:** `feat(system-world): pure furnishing rules (street trees, pavement pieces, purpose areas, clearances) with their test`

---

## Task 2 — Spatial buckets for static solids, and the draw-call baseline

**Files:** `ui/js/desktop/apps/sysworld-traffic.js`, `scripts/test-system-world-traffic.mjs` (append).

**Steps**
- [ ] **Baseline first, before any visual change.** Run:
  ```
  node scripts/system-world-survey.mjs --out reports/system-world-survey/phase-4-baseline --tier high --only overview-day,facade-agent-night
  ```
  Report `stats.calls` and `stats.triangles` of both scenes from `report.json` (Task 12 compares).
- [ ] **Test first.** Append to `scripts/test-system-world-traffic.mjs`:
  ```js
  // Spatial buckets (phase 4): identical answers to the plain scan, first solid in insertion order, moved/removed/wide solids.
  {
    const {createTraffic}=await import('../ui/js/desktop/apps/sysworld-traffic.js');
    let seed=99;const rnd=()=>(seed=(Math.imul(seed,1664525)+1013904223)>>>0)/4294967296;
    const grid=createTraffic(),plain=createTraffic({grid:false});
    for(let i=0;i<900;i++){const box={x:(rnd()-.5)*180,z:(rnd()-.5)*180,heading:rnd()*6.28,min:[-rnd()*2,0,-rnd()*2],max:[rnd()*2,1+rnd()*5,rnd()*2]};
      if(i%7===0)box.owner='o'+(i%3);if(i%50===0){box.min[0]=-1200;box.max[0]=1200;}for(const t of[grid,plain])t.solid('s'+i,box);}
    for(const t of[grid,plain]){t.solid('s5',{x:10,z:10,min:[-1,0,-1],max:[1,2,1]});t.removeSolid('s6');t.removeOwner('o1');}
    const body={id:'b',circles:[{x:0,z:0,r:.4},{x:0,z:.6,r:.4}],minY:0,maxY:2,reach:1};let hits=0;const started=performance.now();
    for(let i=0;i<20000;i++){const a={x:(rnd()-.5)*200,y:rnd()*3,z:(rnd()-.5)*200,heading:rnd()*6},b={x:a.x+(rnd()-.5)*8,y:a.y,z:a.z+(rnd()-.5)*8,heading:a.heading+rnd()-.5};
      const g=grid.obstruction(body,a,b,false);assert.equal(g,plain.obstruction(body,a,b,false),'buckets and scan disagree at '+JSON.stringify([a,b]));if(g)hits++;
      if(i%10===0)assert.equal(grid.ceiling(body,a.x,a.z),plain.ceiling(body,a.x,a.z),'ceiling');}
    assert.ok(hits>500,'the queries must hit solids: '+hits);assert.equal(grid.stats().solids,plain.stats().solids);
    console.log(`Traffic buckets: 20000 swept queries agree with the plain scan (${hits} hits, ${(performance.now()-started).toFixed(0)} ms).`);
  }
  ```
  Run → **FAIL** (`createTraffic({grid:false})` still scans, so it passes trivially). Proceed: the assertion becomes meaningful once both paths exist; report the timing.
- [ ] **`sysworld-traffic.js`.**
  1. Replace `export function createTraffic() {` and its next two lines with:
     ```js
     // Static solids are bucketed in CELL-metre cells by their world box (design risk 1: hundreds of furniture
     // solids). A query visits only the cells its swept box touches and still reports the first blocking solid in
     // insertion order; solids spanning more than WIDE cells, and every solid with {grid:false}, are always visited.
     const CELL=16,WIDE=64;
     export function createTraffic({grid:useGrid=true}={}) {
       const bodies=new Map(),solids=new Map(),requests=new Map(),cells=new Map(),wide=new Set();
       let stops=0,steps=0,clock=0,revision=0,order=0,stamp=0;
       function index(s,add){
         const i0=Math.floor(s.world[0]/CELL),j0=Math.floor(s.world[1]/CELL),i1=Math.floor(s.world[2]/CELL),j1=Math.floor(s.world[3]/CELL);
         if(!useGrid||(i1-i0+1)*(j1-j0+1)>WIDE){if(add)wide.add(s);else wide.delete(s);return;}
         for(let i=i0;i<=i1;i++)for(let j=j0;j<=j1;j++){const k=i*65536+j;let list=cells.get(k);
           if(add){if(!list)cells.set(k,list=[]);list.push(s);}else if(list){const at=list.indexOf(s);if(at>=0)list.splice(at,1);if(!list.length)cells.delete(k);}}
       }
       function near(x0,z0,x1,z1,visit){
         const q=++stamp;for(const s of wide)if(s.stamp!==q){s.stamp=q;visit(s);}
         if(!useGrid)return;
         for(let i=Math.floor(x0/CELL),i1=Math.floor(x1/CELL);i<=i1;i++)for(let j=Math.floor(z0/CELL),j1=Math.floor(z1/CELL);j<=j1;j++){
           const list=cells.get(i*65536+j);if(list)for(const s of list)if(s.stamp!==q){s.stamp=q;visit(s);}}
       }
     ```
  2. In `obstruction`, replace `    for(const s of solids.values())if(s.enabled!==false&&!(s.owner&&(s.owner===body.ignore||s.owner===body.id))&&solidHit(body,from,to,s))return s.id;` with:
     ```js
         const r=body.reach+GAP;let best=null;
         near(Math.min(from.x,to.x)-r,Math.min(from.z,to.z)-r,Math.max(from.x,to.x)+r,Math.max(from.z,to.z)+r,s=>{
           if(s.enabled!==false&&!(s.owner&&(s.owner===body.ignore||s.owner===body.id))&&(!best||s.order<best.order)&&solidHit(body,from,to,s))best=s;});
         if(best)return best.id;
     ```
  3. In `solid(id,box)`, replace `solids.set(id,next);revision++;},` with `next.order=old?old.order:order++;if(old)index(old,false);index(next,true);solids.set(id,next);revision++;},`.
  4. Replace `removeSolid(id){if(solids.delete(id))revision++;},` with `removeSolid(id){const s=solids.get(id);if(s){index(s,false);solids.delete(id);revision++;}},`.
  5. Replace `removeOwner(owner){for(const [id,s]of solids)if(s.owner===owner){solids.delete(id);revision++;}},` with `removeOwner(owner){for(const [id,s]of solids)if(s.owner===owner){index(s,false);solids.delete(id);revision++;}},`.
  6. Replace the body of `ceiling(body,x,z)` with `{let y=0;const r=body.reach+GAP;near(x-r,z-r,x+r,z+r,s=>{if(s.enabled!==false&&solidHit(body,{x,y:s.min[1],z,heading:0},{x,y:s.min[1],z,heading:0},s))y=Math.max(y,s.max[1]-body.minY+1);});return y;}`.
  7. Replace `dispose(){bodies.clear();solids.clear();requests.clear();},` with `dispose(){bodies.clear();solids.clear();requests.clear();cells.clear();wide.clear();},`.

**Run**
- `node scripts/test-system-world-traffic.mjs` → PASS. Report the timing line.
- `node scripts/test-system-world-living.mjs`, `node scripts/test-system-world-hover.mjs`, `node scripts/test-system-world-lobbies.mjs`, `node scripts/test-system-world-ground.mjs` → PASS. Report the living JSON line and the ground graph-build ms (they should not grow).

**Acceptance:** identical obstruction ids and ceilings; existing suites unchanged.

**Commit:** `perf(system-world): bucket static solids in 16 m cells; same first-hit order as the plain scan`

---

## Task 0 — Coordinator delivers the payload (no agent work)

**What the coordinator does**
1. Fixes the `charge-post` collider (Request 1).
2. Registers `life_assets3` in `assets/system-world/build_world3.py` `FAMILIES` (`SOURCES` follows).
3. `& 'D:/Blender 5.2/blender.exe' --background --factory-startup --python assets/system-world/build_world3.py`
4. `node scripts/build-system-world-v3.mjs`, then `node scripts/build-system-world-v3.mjs --check`.
5. Commits `build_world3.py`, `life_assets3.py` and `ui/3d/system-world/v3/*`: `feat(system-world): kit v3 city-life designs (harbour craft, plaza furniture, neon, gull) (payload)`.

**What must already hold at that moment**
- **Task 1:**
  - `test-system-world-v3.mjs` accepts 31 or 46 designs (all fifteen life designs or none);
  - the life contract section of `test-system-world-furnishing.mjs` turns on: parts `rig`, `lamp`, `banner`, `wing_l/r`; charge-post collider behind the pad; both places.
- **No test change needed for materials.** `test-system-world-surfaces.mjs` maps every name the life designs use; the run proves it.
- **No runtime reference yet.** The scene does not import the furnishing module before Task 3.

**Acceptance commands** (all must pass)
- `node scripts/build-system-world-v3.mjs --check` → `verified`, 46 designs, 138 GLBs.
- `node scripts/test-system-world-v3.mjs` → 46 designs; window line unchanged (10 designs).
- `node scripts/test-system-world-furnishing.mjs` → `life designs shipped`.
- `node scripts/test-system-world-surfaces.mjs`, `node scripts/test-system-world-lobby-contract.mjs`
- `python assets/system-world/check_assets.py`

**If the contract fails:** fix the model (node names, collider). Never widen the test.

---

## Task 3 — Kit colliders, robot obstacles, furniture in `kitPlacements`

**Files:**
- `ui/js/desktop/apps/sysworld-colliders.js`, `sysworld-navigation.js`, `sysworld-scene.js`;
- `scripts/test-system-world-{furnishing,hover,living,ground,lobbies,lobby-walk,street-models}.mjs`.

**Steps**
- [ ] **Test first.** Append to `scripts/test-system-world-furnishing.mjs`:
  ```js
  // Real colliders: installCityColliders reads every kit design's navigation.colliders; nothing else within 1 m.
  {
    const {createTraffic}=await import('../ui/js/desktop/apps/sysworld-traffic.js');
    const {installCityColliders,kitCatalog}=await import('../ui/js/desktop/apps/sysworld-colliders.js');
    const {districts,placements,kitPlacements,obstacles}=await import('../ui/js/desktop/apps/sysworld-scene.js');
    const traffic=createTraffic();installCityColliders(traffic,kitCatalog(v1,v3),districts,placements,kitPlacements);
    let checked=0;
    kitPlacements.forEach((p,i)=>{
      if(!F.SIZES[p.asset]||!(v3ids.get(p.asset)?.navigation?.colliders||[]).length)return;
      const f=footprint(p),probe={id:'probe',ignore:'kit:'+i,circles:[{x:0,z:0,r:.9}],minY:0,maxY:2.4,reach:.9},clearAt=(x,z)=>traffic.obstruction(probe,{x,y:p.y,z,heading:0},{x,y:p.y,z,heading:0},false);
      const mid={x:(f[0]+f[2])/2,y:p.y+.5,z:(f[1]+f[3])/2,heading:0};
      assert.equal(traffic.clear({id:'in',circles:[{x:0,z:0,r:.05}],minY:0,maxY:.1,reach:.05},mid,mid,false),false,JSON.stringify(p)+' has no solid');
      for(let x=f[0];x<=f[2]+1e-9;x+=Math.max(.2,(f[2]-f[0])/4))for(const z of[f[1],f[3]])assert.equal(clearAt(x,z),null,JSON.stringify(p)+' has '+clearAt(x,z)+' within 1 m');
      for(let z=f[1];z<=f[3]+1e-9;z+=Math.max(.2,(f[3]-f[1])/4))for(const x of[f[0],f[2]])assert.equal(clearAt(x,z),null,JSON.stringify(p)+' has '+clearAt(x,z)+' within 1 m');
      checked++;});
    assert.ok(checked>=300,'placed furniture with solids: '+checked);
    assert.ok(obstacles.some(o=>o.asset==='litter-bin')&&obstacles.some(o=>o.asset==='bollard'),'street robots steer around pavement pieces');
    assert.ok(!obstacles.some(o=>/^tree-/.test(o.asset)),'plot pieces stay out of the robot obstacles');
    console.log(`System World furniture colliders: ${checked} pieces solid and clear of every other solid by 1 m.`);
  }
  ```
  Run → **FAIL** (`kitPlacements` has no furniture).
- [ ] **`sysworld-colliders.js`.**
  - `export function installCityColliders(traffic,catalog,districts,placements) {` → `export function installCityColliders(traffic,catalog,districts,placements,kit=[]) {`.
  - Before the function's closing `}` (after the `placements.forEach(…);` statement), add:
    ```js
      // Kit v3 placements (phase 4 furniture; street modules have none): every navigation collider of the design,
      // model-local glTF boxes turned with the placement. The owner lets a probe ignore one piece's own solids.
      kit.forEach((p,i)=>(catalog.get(p.asset)?.navigation?.colliders||[]).forEach((b,k)=>traffic.solid('kit:'+i+':'+k,
        {owner:'kit:'+i,x:p.x,z:p.z,heading:p.angle,min:[b[0],p.y+b[1],b[2]],max:[b[3],p.y+b[4],b[5]]})));
    ```
- [ ] **`sysworld-navigation.js`.**
  1. Replace the comment and signature `export function obstaclesFrom(placements) {` with:
     ```js
     // Kit pieces listed in `sizes` (collider footprints [x0,z0,x1,z1], see sysworld-furnishing.js SIZES): square
     // boxes are round pieces and take the inscribed radius; longer boxes a chain of circles along their long axis.
     export function obstaclesFrom(placements, sizes = {}) {
     ```
  2. After the line `    else if (p.asset === 'planter') { add(p, -.9, 0, .85); add(p, .9, 0, .85); }` add:
     ```js
         else if (sizes[p.asset]) {
           const [x0, z0, x1, z1] = sizes[p.asset], hx = (x1 - x0) / 2, hz = (z1 - z0) / 2, cx = (x0 + x1) / 2, cz = (z0 + z1) / 2;
           if (Math.abs(hx - hz) < .05) add(p, cx, cz, Math.max(hx, hz));
           else { const longX = hx > hz, major = Math.max(hx, hz), minor = Math.min(hx, hz), n = Math.ceil(major / Math.max(.3, minor)), half = major / n;
             for (let k = 0; k < n; k++) { const o = -major + half + 2 * half * k; add(p, cx + (longX ? o : 0), cz + (longX ? 0 : o), Math.hypot(minor, half)); } }
         }
     ```
  3. In `createNavigator`, replace the whole `function hitsObstacle(x, z) {…}` with:
     ```js
       // Obstacles are bucketed in 8 m cells; a probe visits the cells within the largest reach.
       const CELL = 8, buckets = new Map(), reachMax = Math.max(0, ...obstacles.map(o => o.r)) + robotRadius;
       for (const o of obstacles) { const k = Math.floor(o.x / CELL) * 4096 + Math.floor(o.z / CELL); if (!buckets.has(k)) buckets.set(k, []); buckets.get(k).push(o); }
       function hitsObstacle(x, z) {
         for (let i = Math.floor((x - reachMax) / CELL); i <= Math.floor((x + reachMax) / CELL); i++)
           for (let j = Math.floor((z - reachMax) / CELL); j <= Math.floor((z + reachMax) / CELL); j++) {
             const list = buckets.get(i * 4096 + j);
             if (list) for (const o of list) { const r = o.r + robotRadius; if ((x - o.x) ** 2 + (z - o.z) ** 2 < r * r) return true; }
           }
         return false;
       }
     ```
- [ ] **`sysworld-scene.js`.**
  1. After `import { streetModules, plotSlabs, island } from './sysworld-ground.js';` add `import { furnishing, SIZES as furnitureSizes } from './sysworld-furnishing.js';`.
  2. Replace the two comment lines above `export const kitPlacements` and that statement with:
     ```js
     // Kit v3 placements, instanced like `placements` but loaded from ui/3d/system-world/v3/: the street modules (fixed
     // length, never stretched; sysworld-ground.js) and the phase 4 furniture and signs (sysworld-furnishing.js).
     export const streetPlacements = streetModules().map(m => ({ asset: m.id, x: m.x, z: m.z, y: 0, angle: m.angle, scale: [1, 1, 1] }));
     export const kitPlacements = [...streetPlacements, ...furnishing().map(f => ({ asset: f.asset, x: f.x, z: f.z, y: f.y, angle: f.angle, scale: [1, 1, 1], area: f.area }))];
     ```
  3. Replace `export const obstacles = [...obstaclesFrom(placements), ` with:
     ```js
     // Street robots also steer around the kit pieces standing in a street band (the pavement furniture).
     const inStreet = p => streets.xs.some(x => Math.abs(p.x - x) <= streets.halfWidth) || streets.zs.some(z => Math.abs(p.z - z) <= streets.halfWidth);
     export const obstacles = [...obstaclesFrom([...placements, ...kitPlacements.filter(p => furnitureSizes[p.asset] && inStreet(p))], furnitureSizes), 
     ```
  4. `installCityColliders(traffic,kitCatalog(catalog,catalog3),districts,placements);` → `installCityColliders(traffic,kitCatalog(catalog,catalog3),districts,placements,kitPlacements);`.
  5. In `rebuild()`, `const kitIds = [...new Set(kitPlacements.map(p => p.asset))];` → `const kitIds = [...new Set(kitPlacements.map(p => p.asset))].filter(id => catalog3.has(id));`, and `for (const p of kitPlacements) add(p, kit.get(p.asset));` → `for (const p of kitPlacements) if (kit.has(p.asset)) add(p, kit.get(p.asset));`.
- [ ] **Test call sites** (furniture solids in every city test):
  - In `test-system-world-hover.mjs`, `-living.mjs`, `-lobby-walk.mjs`, `-lobbies.mjs`, `-ground.mjs` (three places): `{districts,placements` → `{districts,placements,kitPlacements` in the scene import, and `districts,placements);` → `districts,placements,kitPlacements);` in every `installCityColliders(` call.
  - In `test-system-world-street-models.mjs`: `import {kitPlacements} from '../ui/js/desktop/apps/sysworld-scene.js';` → `import {streetPlacements as kitPlacements} from '../ui/js/desktop/apps/sysworld-scene.js';`.

**Run**
- `node scripts/test-system-world-furnishing.mjs` → PASS.
- `node scripts/test-system-world-hover.mjs` → PASS (docks clear by 3 m, reachable).
- `node scripts/test-system-world-living.mjs` → PASS, **limits unchanged**. Report the JSON line (completed, lobbyVisits, byPlace, yield seconds) against Task 2's.
- `node scripts/test-system-world.mjs` → PASS (one simulated hour: no furniture hits, < 2 % stopped). Report the runtime.
- `node scripts/test-system-world-ground.mjs` (ratio ≥ 0.7, report it), `-lobbies.mjs`, `-lobby-walk.mjs` (≥ 8 climbs), `-street-models.mjs`, `-jitter.mjs`, `-surfaces.mjs` → PASS.

**If the living test fails:** read `SYSTEM_WORLD_DIAGNOSTICS=1` output; move or drop the specific piece through a rule (for example a larger `GAP.living`), never lower a limit.

**Acceptance:** about 400–600 furniture solids are real for the visitor, residents, hover and robots; all suites pass.

**Commit:** `feat(system-world): place the phase 4 furniture; kit v3 colliders and robot obstacles from the design boxes`

---

## Task 4 — Near-gate, lawn and paving

**Files:** `ui/js/desktop/apps/sysworld-scene.js`.

**Steps**
- [ ] Change the furnishing import to `import { furnishing, SIZES as furnitureSizes, NEAR, NEAR_HEIGHT, SURFACES } from './sysworld-furnishing.js';`.
- [ ] After `  world.add(plots);` add:
  ```js
    // Lawn and paving over the purpose plots (backlog item 7): 2 cm instanced slabs, one draw each, surface maps of
    // the leaf and setts kinds (the lawn is not `city.leaf`, so it never sways).
    const lawnMaterial = new THREE.MeshStandardMaterial({ name: 'lawn', color: 0x1f3a26, metalness: 0, roughness: .9 });
    const pavingMaterial = new THREE.MeshStandardMaterial({ name: 'paving', color: 0x3a4146, metalness: .05, roughness: .75 });
    matSet.add(lawnMaterial); matSet.add(pavingMaterial); surfaces.apply(lawnMaterial, 'world', 'city.leaf'); surfaces.apply(pavingMaterial, 'world', 'city.setts');
    const groundCover = [['lawn', lawnMaterial], ['paving', pavingMaterial]].map(([kind, material]) => {
      const rects = SURFACES.filter(s => s.kind === kind), mesh = new THREE.InstancedMesh(slabGeometry, material, Math.max(1, rects.length));
      rects.forEach((s, i) => mesh.setMatrixAt(i, new THREE.Matrix4().compose(new THREE.Vector3((s.x0 + s.x1) / 2, .01, (s.z0 + s.z1) / 2), new THREE.Quaternion(), new THREE.Vector3(s.x1 - s.x0, .02, s.z1 - s.z0))));
      mesh.count = rects.length; mesh.name = kind; mesh.receiveShadow = true; world.add(mesh); return mesh;
    });
  ```
- [ ] `  let moodKey = '';` → `  let moodKey = '', nearHidden = null;`.
- [ ] In `rebuild()`:
  - `if (!batches.has(n.uuid)) batches.set(n.uuid, { node: n, matrices: [] });` → `if (!batches.has(n.uuid)) batches.set(n.uuid, { node: n, matrices: [], near: NEAR.has(p.asset) });`;
  - `for (const { node, matrices } of batches.values()) {` → `for (const { node, matrices, near } of batches.values()) {`;
  - `mesh.castShadow = true; mesh.receiveShadow = true; staticCity.add(mesh);` → `mesh.castShadow = true; mesh.receiveShadow = true; mesh.userData.near = near; staticCity.add(mesh);`;
  - directly after that loop's closing `}` add `      nearHidden = null;`.
- [ ] In `update()`, after `    if (lobbyRender.update(camera.position, landmarks)) renderer.shadowMap.needsUpdate = true;` add:
  ```js
      // Small furniture (NEAR) is sub-pixel from the orbit views: hidden above NEAR_HEIGHT, fewer draw calls.
      const far = camera.position.y > NEAR_HEIGHT;
      if (far !== nearHidden) { nearHidden = far; staticCity.traverse(n => { if (n.userData.near) n.visible = !far; }); renderer.shadowMap.needsUpdate = true; }
  ```
- [ ] In `dispose()`, `beacons.dispose(); plots.dispose();` → `beacons.dispose(); plots.dispose(); groundCover.forEach(m => m.dispose());`.
- [ ] In `stats()`, after `lobbies:lobbyRender.stats(),` insert `furniture:kitPlacements.length-streetPlacements.length,nearHidden,`.

**Run**
- `node -e "import('esbuild').then(e=>e.build({entryPoints:['ui/js/desktop/apps/sysworld-scene.js'],bundle:true,format:'esm',write:false,logLevel:'error'})).then(()=>console.log('ok'))"` → `ok`.
- `node scripts/test-system-world-surfaces.mjs`, `node scripts/test-system-world-layout.mjs` → PASS.
- `node scripts/system-world-survey.mjs --out reports/system-world-survey/phase-4-t4 --tier high --only overview-day,facade-agent-night`
  - expected: two PNGs, page errors 0;
  - report `calls`/`triangles` and `furniture`/`nearHidden` from `report.json`; `nearHidden` must be `true` at `overview-day`.

**Commit:** `feat(system-world): lawn and paving on the purpose plots; small furniture hidden above 40 m`

---

## Task 5 — Wind, leaf sway, banners

**Files:**
- `ui/js/desktop/apps/sysworld-atmosphere.js`, `sysworld-surfaces.js`, new `sysworld-parts.js`, `sysworld-scene.js`;
- new `scripts/test-system-world-wind.mjs`, `scripts/test-system-world-surfaces.mjs`.

**Steps**
- [ ] **Test first.** Write the wind test below. In `test-system-world-surfaces.mjs`, before `surfaces.dispose();` add:
  ```js
  // Leaf sway (phase 4): only city.leaf, only on instanced draws and tiers above low; the lawn (leaf kind) never sways.
  {
    const leafHigh = compile('city.leaf', 'world', 'high'), leafLow = compile('city.leaf', 'world', 'low');
    assert.ok(leafHigh.text.includes('#define SW_SWAY ') && leafHigh.text.includes('defined(SW_SWAY)&&defined(USE_INSTANCING)') && leafHigh.text.includes('uniform vec4 swWind'), 'leaf sway injection');
    assert.ok(!leafLow.text.includes('#define SW_SWAY '), 'no sway on low');
    surfaces.setTier('high');const lawn = new THREE.MeshStandardMaterial({ name: 'lawn' });surfaces.apply(lawn, 'world', 'city.leaf');
    const shader = { uniforms: {}, vertexShader: THREE.ShaderLib.standard.vertexShader, fragmentShader: THREE.ShaderLib.standard.fragmentShader };lawn.onBeforeCompile(shader);
    assert.ok(!shader.vertexShader.includes('#define SW_SWAY '), 'the lawn never sways');assert.notEqual(lawn.customProgramCacheKey(), leafHigh.material.customProgramCacheKey(), 'sway changes the program key');
    surfaces.setWind({ x: 0, z: 1, strength: .4, time: 12 });assert.deepEqual(shader.uniforms.swWind.value.toArray(), [0, 1, .4, 12]);
  }
  ```
  Run both → **FAIL**.
- [ ] **`sysworld-atmosphere.js`.**
  1. Before `export function createAtmosphere(scene, options = {}) {` add:
     ```js
     // Wind (phase 4): one slowly turning direction with gusts, written into `out` (no allocation). Rain blows harder,
     // fog calms it. Leaves (sysworld-surfaces.js), banners, skiff rigs and steam read it; the leaf sound will (phase 5).
     export function windAt(t, weather = 'clear', out = {}) {
       const angle = .6 + .5 * Math.sin(t * .013), base = .32 + .12 * Math.sin(t * .021 + 1.1), g = Math.max(0, Math.sin(t * .37) * Math.sin(t * .113 + 1.3));
       out.x = Math.cos(angle); out.z = Math.sin(angle);
       out.strength = Math.min(1, (base + .45 * g * g) * (weather === 'rain' ? 1.5 : weather === 'fog' ? .6 : 1));
       return out;
     }
     ```
  2. `let busy = false, tier = 'high', sweep = 0, lampLevel = 1;` → `let busy = false, tier = 'high', sweep = 0, lampLevel = 1, weatherKind = 'clear';` and add on the next line `const wind = { x: 1, z: 0, strength: 0, time: 0 };`.
  3. `setWeather(value) { cloud.value = …; },` → prefix the body with `weatherKind = value; `.
  4. In `update`, after `if (post.enabled) cinema.update(step, camera, sunDir, { day: day.value, dusk: dusk.value, cloud: cloud.value, animated });` add `windAt(time.value, weatherKind, wind); if (tier === 'low' || !animated) wind.strength = 0; wind.time = time.value;`.
  5. After `    setPointScale(value) { pointScale.value = value; },` add `    wind: () => wind,`.
  6. In `stats`, after `clouds: +cloud.value.toFixed(2),` insert `wind: +wind.strength.toFixed(2),`.
- [ ] **`sysworld-surfaces.js`.**
  1. `'city.leaf': {top: 'foliage', side: 'foliage', albedo: .7, rough: .3, bump: .03, wet: .5},` → add `, sway: .035` before `}`.
  2. `const VERTEX_PARS = \`varying vec3 vSwPos;varying vec3 vSwNrm;` → `const VERTEX_PARS = \`varying vec3 vSwPos;varying vec3 vSwNrm;uniform vec4 swWind;`.
  3. At the start of the `VERTEX_MAIN` template (directly after the opening backtick and newline) insert:
     ```
     #if defined(SW_SWAY)&&defined(USE_INSTANCING)
     {float swHgt=max(transformed.y,0.0),swPh=dot(instanceMatrix[3].xz,vec2(.37,.23));
     vec3 swDir=transpose(mat3(instanceMatrix))*vec3(swWind.x,0.0,swWind.y);
     transformed+=swDir*(swWind.z*SW_SWAY*swHgt*sqrt(swHgt)*(.62+.38*sin(swWind.w*1.9+swPh)+.12*sin(swWind.w*5.3+swPh*2.1)));}
     #endif
     ```
  4. In `shared`, after `swShelter: { value: shelters },` add `swWind: { value: new THREE.Vector4(1, 0, 0, 0) },`.
  5. In `inject`, after `if (windows) defines.push('#define SW_WINDOWS');` add `if (config?.sway && material.name === 'city.leaf' && level > 0) defines.push('#define SW_SWAY ' + float(config.sway));`.
  6. In `apply`, `material.customProgramCacheKey = () => ['sw', kind, windows, space, kind ? level : 0].join('|');` → `material.customProgramCacheKey = () => ['sw', kind, windows, space, kind ? level : 0, !!(KINDS[kind]?.sway && material.name === 'city.leaf' && level > 0)].join('|');`.
  7. After `    setWeather(value) { … },` add:
     ```js
         // Wind for leaf sway (atmosphere.wind()): direction x/z, strength (0 = still), time.
         setWind(w) { shared.swWind.value.set(w.x, w.z, w.strength, w.time); },
     ```
- [ ] **`sysworld-parts.js`** (complete):
  ```js
  import * as THREE from 'three';
  // Instanced moving parts of kit v3 designs (phase 4): banner cloth, skiff rigs, buoy lanterns, gull wings and whole
  // hulls. One InstancedMesh per mesh of the template; a mesh under a `movable` part node turns about that node's origin
  // (the pivot kit3.py authors from `pivots`). `only` keeps just the meshes under those part nodes. Materials stay the
  // template's (shared kit materials): never dispose them here.
  export function partOf(node,names){for(let o=node;o;o=o.parent)if(names.includes(o.name))return o;return null;}
  export function createPartInstances(template,count,{only=null,movable=[]}={}){
    template.updateMatrixWorld(true);const rows=[];
    template.traverse(n=>{
      if(!n.isMesh||(only&&!partOf(n,only)))return;
      const part=partOf(n,movable),pivot=(part||template).matrixWorld.clone(),local=pivot.clone().invert().multiply(n.matrixWorld);
      const mesh=new THREE.InstancedMesh(n.geometry,n.material,count);mesh.frustumCulled=false;mesh.castShadow=true;mesh.receiveShadow=true;
      rows.push({mesh,part:part?.name||null,pivot,local});
    });
    const m=new THREE.Matrix4(),zero=new THREE.Matrix4().makeScale(0,0,0);
    return {
      meshes:rows.map(r=>r.mesh),
      // matrix places the whole design; turn(partName) returns a rotation Matrix4 for that part, or null.
      set(i,matrix,turn){for(const r of rows){m.multiplyMatrices(matrix,r.pivot);const t=r.part&&turn?turn(r.part):null;if(t)m.multiply(t);m.multiply(r.local);r.mesh.setMatrixAt(i,m);}},
      hide(i){for(const r of rows)r.mesh.setMatrixAt(i,zero);},
      commit(){for(const r of rows)r.mesh.instanceMatrix.needsUpdate=true;},
      dispose(){for(const r of rows)r.mesh.dispose();},
    };
  }
  ```
- [ ] **`sysworld-scene.js`.**
  1. After the furnishing import add `import { createPartInstances, partOf } from './sysworld-parts.js';`.
  2. `  let moodKey = '', nearHidden = null;` → append on the next line:
     ```js
       let banners = null, bannerAngle = 0;
       const bannerTurn = new THREE.Matrix4(), bannerRotation = () => bannerTurn.makeRotationY(bannerAngle);
       // Banner cloth swings about the mast with the wind (zero wind: at rest).
       function swingBanners(wind, time) {
         if (!banners) return;
         for (let i = 0; i < banners.bases.length; i++) { bannerAngle = wind.strength * (.22 + .16 * Math.sin(time * 1.7 + i * 2.1)); banners.parts.set(i, banners.bases[i], bannerRotation); }
         banners.parts.commit();
       }
     ```
  3. In `rebuild()`'s `add`, `if (!n.isMesh) return;` → `if (!n.isMesh || (p.asset === 'banner-mast' && partOf(n, ['banner']))) return;`.
  4. Directly before the `      nearHidden = null;` line added in Task 4, insert:
     ```js
           // The banner cloth gets its own instances (sysworld-parts.js); the mast stays in the static batch.
           const masts = kitPlacements.filter(p => p.asset === 'banner-mast' && kit.has(p.asset));
           banners = masts.length ? { parts: createPartInstances(kit.get('banner-mast'), masts.length, { only: ['banner'], movable: ['banner'] }),
             bases: masts.map(p => new THREE.Matrix4().compose(new THREE.Vector3(p.x, p.y, p.z), new THREE.Quaternion().setFromAxisAngle(new THREE.Vector3(0, 1, 0), p.angle), new THREE.Vector3(1, 1, 1))) } : null;
           if (banners) { banners.parts.meshes.forEach(m => staticCity.add(m)); swingBanners({ strength: 0 }, 0); }
     ```
  5. In `update()`, after `    const mood = weather.mood();` add:
     ```js
         // City life (phase 4): the atmosphere's wind moves leaves and banners; the atmosphere clock drives the harbour.
         const wind = atmosphere.wind(), seaTime = atmosphere.seaTime?.() ?? elapsed; surfaces.setWind(wind); swingBanners(wind, seaTime);
     ```

### `scripts/test-system-world-wind.mjs` (complete)

```js
// Phase 4 wind: bounded, turning, gusty, weather-dependent and deterministic; owned by the atmosphere, still on low and
// without motion; banner parts turn about their pivot.
import assert from 'node:assert/strict';
import * as THREE from 'three';
import {windAt,createAtmosphere} from '../ui/js/desktop/apps/sysworld-atmosphere.js';
import {createPartInstances} from '../ui/js/desktop/apps/sysworld-parts.js';
let lo=1,hi=0;const out={};
for(let t=0;t<3600;t+=.5){windAt(t,'clear',out);lo=Math.min(lo,out.strength);hi=Math.max(hi,out.strength);assert.ok(Math.abs(Math.hypot(out.x,out.z)-1)<1e-9,'unit direction');}
assert.ok(lo>=.15&&hi<=1&&hi-lo>.3,'wind range '+lo.toFixed(2)+'..'+hi.toFixed(2));
assert.ok(windAt(100,'rain').strength>windAt(100,'clear').strength&&windAt(100,'fog').strength<windAt(100,'clear').strength,'weather');
assert.deepEqual(windAt(42.5),windAt(42.5));assert.equal(windAt(1,'clear',out),out,'writes into out');
const atmosphere=createAtmosphere(new THREE.Scene(),{}),camera=new THREE.PerspectiveCamera();
atmosphere.setTier('high');atmosphere.update(1,1,camera,true);assert.ok(atmosphere.wind().strength>0,'wind blows');
atmosphere.update(1,2,camera,false);assert.equal(atmosphere.wind().strength,0,'reduced motion stills the wind');
atmosphere.setTier('low');atmosphere.update(1,3,camera,true);assert.equal(atmosphere.wind().strength,0,'no wind on low');atmosphere.dispose();
// Parts: a mesh under `banner` turns about the banner node's origin; the rest is skipped with `only`.
const root=new THREE.Group(),mast=new THREE.Mesh(new THREE.BoxGeometry(),new THREE.MeshBasicMaterial()),banner=new THREE.Group(),cloth=new THREE.Mesh(mast.geometry,mast.material);
banner.name='banner';banner.position.set(.14,6.55,0);cloth.position.set(.5,-1.6,0);banner.add(cloth);root.add(mast,banner);
const parts=createPartInstances(root,2,{only:['banner'],movable:['banner']});assert.equal(parts.meshes.length,1,'only the cloth');
const m=new THREE.Matrix4(),p=new THREE.Vector3(),turn=new THREE.Matrix4().makeRotationY(Math.PI/2);
parts.set(0,new THREE.Matrix4(),()=>null);parts.meshes[0].getMatrixAt(0,m);p.setFromMatrixPosition(m);assert.ok(p.distanceTo(new THREE.Vector3(.64,4.95,0))<1e-9,'rest pose');
parts.set(0,new THREE.Matrix4(),()=>turn);parts.meshes[0].getMatrixAt(0,m);p.setFromMatrixPosition(m);assert.ok(p.distanceTo(new THREE.Vector3(.14,4.95,-.5))<1e-9,'turns about the pivot');
parts.hide(1);parts.meshes[0].getMatrixAt(1,m);assert.equal(m.determinant(),0,'hidden instance');parts.dispose();
console.log(`System World wind: ${lo.toFixed(2)}..${hi.toFixed(2)} strength, weather, low/reduced stillness and pivoted parts verified.`);
```

**Run**
- `node scripts/test-system-world-wind.mjs`, `node scripts/test-system-world-surfaces.mjs`, `node scripts/test-system-world-lamps.mjs`, `node scripts/test-system-world-windows.mjs` → PASS.
- Esbuild bundle check (Task 4 command) → `ok`.

**Acceptance:**
- Trees, planters, beds and v2 gardens sway, with zero amplitude at their origin; trunks of other materials stay put.
- Banners swing; nothing moves on low or with reduced motion.

**Commit:** `feat(system-world): atmosphere wind; instanced leaves sway and banners swing (off on low and with reduced motion)`

---

## Task 6 — Harbour traffic

**Files:** `ui/js/desktop/apps/sysworld-atmosphere.js`, new `sysworld-harbour.js`, `sysworld-scene.js`, new `scripts/test-system-world-harbour.mjs`.

**Steps**
- [ ] Write the test. Run → **FAIL**.
- [ ] **`sysworld-atmosphere.js`.**
  - Before `export function windAt` add:
    ```js
    // The sea shader's own height function (SEA.fragment `height`), in JS: harbour hulls ride it. Keep both in step;
    // scripts/test-system-world-harbour.mjs evaluates the GLSL text against this function.
    export function seaHeight(x, z, t) { return .36 * Math.sin(x * .09 + t * .9 + Math.sin(z * .07) * 1.5) + .26 * Math.sin(z * .11 - t * .7 + x * .03) + .12 * Math.sin((x + z) * .21 + t * 1.4) + .07 * Math.sin(x * .52 - z * .37 + t * 2.2); }
    ```
  - After `    wind: () => wind,` add `    seaTime: () => time.value,`.
- [ ] Write `sysworld-harbour.js` below.
- [ ] **`sysworld-scene.js`.**
  1. After the parts import add `import { createHarbour } from './sysworld-harbour.js';`.
  2. Directly after the `kits` object (the line `  };` that closes `const kits = {`) add:
     ```js
       // Harbour craft and gulls load after the first display (not part of expectedBytes).
       const lifeLoad = (id, lod) => catalog3.has(id) ? model(id, lod, 'v3') : Promise.reject(Error('Kit v3 design missing: ' + id));
       const harbour = createHarbour({ group: world, load: lifeLoad });
     ```
  3. `try { applyTier(); await rebuild(); } catch(e) { dispose(); throw e; }` → append ` harbour.start();` after it on the same line.
  4. In `applyTier`, `lobbyRender.setTier(tier);` → `lobbyRender.setTier(tier); harbour.setTier(tier);`.
  5. In `update()`, after the line ending `swingBanners(wind, seaTime);` add `    harbour.update(seaTime, mood.day, wind, !reduced);`.
  6. In `dispose()`, `lobbyRender.dispose();` → `lobbyRender.dispose(); harbour.dispose();`.
  7. In `stats()`, after `lobbies:lobbyRender.stats(),` insert `harbour:harbour.stats(),`.

### `ui/js/desktop/apps/sysworld-harbour.js` (complete)

```js
import * as THREE from 'three';
import {seaHeight} from './sysworld-atmosphere.js';
import {createPartInstances} from './sysworld-parts.js';

// Harbour traffic (phase 4): a launch loops off the south quay, a barge crawls a long stadium 100-150 m out, three
// skiffs circle east of the island with the rig swinging to leeward, six buoys mark the channel and blink at night.
// Positions are closed-form functions of the atmosphere clock (craftPoses), so reduced motion freezes them and a test
// can replay ten minutes. The sea is flat geometry with shaded waves: hulls heave HEAVE x the shader height and tilt
// with its full slope. Boats keep KEEP_OUT from the island, the tidal turbine and each other. Low tier: LOD 2, travel stops.
export const SEA_LEVEL=-3.2,HEAVE=.25,KEEP_OUT=12;
export const CRAFT=Object.freeze([
  {id:'launch',asset:'harbour-launch',length:9,beam:2.9,speed:4.5,route:{kind:'ellipse',x:-45,z:100,rx:28,rz:6}},
  {id:'barge',asset:'harbour-barge',length:22,beam:6.2,speed:2.6,route:{kind:'stadium',x:0,z:205,half:200,radius:25}},
  {id:'skiff-0',asset:'sail-skiff',length:5.4,beam:1.9,speed:2.2,route:{kind:'circle',x:112,z:-40,r:9}},
  {id:'skiff-1',asset:'sail-skiff',length:5.4,beam:1.9,speed:-2.6,route:{kind:'circle',x:125,z:5,r:11}},
  {id:'skiff-2',asset:'sail-skiff',length:5.4,beam:1.9,speed:2,route:{kind:'circle',x:110,z:50,r:9}},
].map(c=>Object.freeze(c)));
export const BUOYS=Object.freeze([[25,100],[39,100],[25,122],[39,122],[25,144],[39,144]]);
const TAU=Math.PI*2,SLOT=CRAFT.map((c,i)=>CRAFT.slice(0,i).filter(o=>o.asset===c.asset).length);
const routeLength=r=>r.kind==='circle'?TAU*r.r:r.kind==='ellipse'?Math.PI*(3*(r.rx+r.rz)-Math.sqrt((3*r.rx+r.rz)*(r.rx+3*r.rz))):4*r.half+TAU*r.radius;
// Pose at arc length s on a closed route: x, z and heading (bow +X; heading h points the bow to (cos h, -sin h)).
export function routePose(r,s,out){
  const L=routeLength(r);let u=((s%L)+L)%L,a;
  if(r.kind!=='stadium'){
    const rx=r.kind==='circle'?r.r:r.rx,rz=r.kind==='circle'?r.r:r.rz;a=u/L*TAU;
    out.x=r.x+rx*Math.cos(a);out.z=r.z+rz*Math.sin(a);out.heading=Math.atan2(-rz*Math.cos(a),-rx*Math.sin(a));return out;
  }
  const leg=2*r.half,turn=Math.PI*r.radius;
  if(u<leg){out.x=r.x-r.half+u;out.z=r.z-r.radius;out.heading=0;return out;}
  u-=leg;
  if(u<turn){a=-Math.PI/2+u/r.radius;out.x=r.x+r.half+r.radius*Math.cos(a);}
  else{u-=turn;if(u<leg){out.x=r.x+r.half-u;out.z=r.z+r.radius;out.heading=Math.PI;return out;}u-=leg;a=Math.PI/2+u/r.radius;out.x=r.x-r.half+r.radius*Math.cos(a);}
  out.z=r.z+r.radius*Math.sin(a);out.heading=Math.atan2(-Math.cos(a),-Math.sin(a));return out;
}
export function craftPoses(t,out=CRAFT.map(()=>({x:0,z:0,heading:0}))){
  for(let i=0;i<CRAFT.length;i++){routePose(CRAFT[i].route,CRAFT[i].speed*t,out[i]);if(CRAFT[i].speed<0)out[i].heading+=Math.PI;}
  return out;
}
const e=new THREE.Euler(),q=new THREE.Quaternion(),v=new THREE.Vector3(),one=new THREE.Vector3(1,1,1);
// Hull matrix on the sea: heave from the height, pitch along the keel, roll across the beam (all from seaHeight).
function ride(m,x,z,heading,length,beam,t){
  const c=Math.cos(heading),s=Math.sin(heading),fx=c*length*.4,fz=-s*length*.4,sx=s*beam*.5,sz=c*beam*.5;
  const pitch=Math.atan2(seaHeight(x+fx,z+fz,t)-seaHeight(x-fx,z-fz,t),length*.8),roll=-Math.atan2(seaHeight(x+sx,z+sz,t)-seaHeight(x-sx,z-sz,t),beam);
  e.set(roll,heading,pitch,'YZX');q.setFromEuler(e);v.set(x,SEA_LEVEL+HEAVE*seaHeight(x,z,t),z);return m.compose(v,q,one);
}
export function createHarbour({group,load}){
  const holder=new THREE.Group();holder.name='harbour';group.add(holder);
  const poses=craftPoses(0),m=new THREE.Matrix4(),turn=new THREE.Matrix4(),zero=new THREE.Matrix4().makeScale(0,0,0);
  let tier='high',lod=null,parts=new Map(),travel=0,started=false,disposed=false,rigAngle=0,lampOn=false;
  const rigTurn=name=>name==='rig'?turn.makeRotationY(rigAngle):null,lampTurn=name=>name==='lamp'&&!lampOn?zero:null;
  async function build(level){
    lod=level;const ids=[...new Set([...CRAFT.map(c=>c.asset),'harbour-buoy'])],loaded=await Promise.allSettled(ids.map(id=>load(id,level)));
    if(disposed||lod!==level)return;
    for(const p of parts.values())p.dispose();holder.clear();parts=new Map();
    ids.forEach((id,i)=>{if(loaded[i].status!=='fulfilled')return;
      const p=createPartInstances(loaded[i].value,id==='sail-skiff'?3:id==='harbour-buoy'?BUOYS.length:1,{movable:['rig','lamp']});
      p.meshes.forEach(x=>{x.castShadow=false;holder.add(x);});parts.set(id,p);});
  }
  return {
    start(){if(started)return;started=true;void build(tier==='low'?2:1);},
    setTier(value){tier=value;const level=tier==='low'?2:1;if(started&&level!==lod)void build(level);},
    // time: the atmosphere clock (frozen under reduced motion); day: 1 day .. 0 night; wind: atmosphere.wind().
    update(time,day,wind,animated=true){
      if(tier!=='low')travel=time;craftPoses(travel,poses);
      for(let i=0;i<CRAFT.length;i++){const c=CRAFT[i],p=parts.get(c.asset);if(!p)continue;const pose=poses[i];
        // The boom swings to leeward: towards the side the wind blows to, further in gusts.
        const across=wind.x*Math.sin(pose.heading)+wind.z*Math.cos(pose.heading);rigAngle=(across>=0?1:-1)*(.35+.4*wind.strength)+.06*Math.sin(time*1.3+i);
        p.set(SLOT[i],ride(m,pose.x,pose.z,pose.heading,c.length,c.beam,time),rigTurn);}
      const buoys=parts.get('harbour-buoy');
      if(buoys)for(let i=0;i<BUOYS.length;i++){lampOn=day<.5&&(!animated||(time+i*.5)%3<.45);buoys.set(i,ride(m,BUOYS[i][0],BUOYS[i][1],i*.9,1.6,1.6,time),lampTurn);}
      for(const p of parts.values())p.commit();
    },
    barge:()=>poses[1],
    meshes:id=>parts.get(id)?.meshes||[],
    stats:()=>({craft:CRAFT.filter(c=>parts.has(c.asset)).length,buoys:parts.has('harbour-buoy')?BUOYS.length:0,lod,travel:+travel.toFixed(1)}),
    dispose(){disposed=true;for(const p of parts.values())p.dispose();parts.clear();holder.removeFromParent();},
  };
}
```

### `scripts/test-system-world-harbour.mjs` (complete)

```js
// Phase 4 harbour: the JS wave is the shader's, ten minutes inside the keep-out, wave ride, rigs, lanterns, tiers,
// no per-frame allocation, dispose.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import * as THREE from 'three';
import {seaHeight} from '../ui/js/desktop/apps/sysworld-atmosphere.js';
import {CRAFT,KEEP_OUT,SEA_LEVEL,HEAVE,craftPoses,createHarbour} from '../ui/js/desktop/apps/sysworld-harbour.js';
import {streets} from '../ui/js/desktop/apps/sysworld-layout.js';
import {landmarkPlaces} from '../ui/js/desktop/apps/sysworld-places.js';
const source=await fs.readFile('ui/js/desktop/apps/sysworld-atmosphere.js','utf8');
const glsl=/float height\(vec2 p\)\{return ([\s\S]*?);\}/.exec(source)[1].replace(/\s+/g,'').replace(/p\.x/g,'x').replace(/p\.y/g,'z').replace(/time/g,'t').replace(/sin\(/g,'Math.sin(');
const shader=new Function('x','z','t','return '+glsl);
for(let i=0;i<500;i++){const x=(Math.random()-.5)*600,z=(Math.random()-.5)*600,t=Math.random()*900;assert.ok(Math.abs(shader(x,z,t)-seaHeight(x,z,t))<1e-9,'seaHeight differs from the SEA shader');}
const turbine=landmarkPlaces.find(l=>l.id==='tidal-turbine'),poses=craftPoses(0);
const corners=(c,p)=>[[1,1],[1,-1],[-1,1],[-1,-1]].map(([a,b])=>{const lx=a*c.length/2,lz=b*c.beam/2,co=Math.cos(p.heading),si=Math.sin(p.heading);return [p.x+lx*co+lz*si,p.z-lx*si+lz*co];});
const island=([x,z])=>Math.hypot(Math.max(0,streets.minX-x,x-streets.maxX),Math.max(0,streets.minZ-z,z-streets.maxZ));
let worst={island:Infinity,turbine:Infinity,boats:Infinity},previous=null;
for(let t=0;t<=600;t+=.1){
  craftPoses(t,poses);const cs=CRAFT.map((c,i)=>corners(c,poses[i]));
  for(const p of cs.flat()){worst.island=Math.min(worst.island,island(p));worst.turbine=Math.min(worst.turbine,Math.hypot(p[0]-turbine.x,p[1]-turbine.z));}
  for(let i=0;i<cs.length;i++)for(let j=i+1;j<cs.length;j++)for(const a of cs[i])for(const b of cs[j])worst.boats=Math.min(worst.boats,Math.hypot(a[0]-b[0],a[1]-b[1]));
  if(previous)poses.forEach((p,i)=>{const dx=p.x-previous[i].x,dz=p.z-previous[i].z,d=Math.hypot(dx,dz);if(d>1e-6)assert.ok(Math.cos(p.heading)*dx/d-Math.sin(p.heading)*dz/d>.9,CRAFT[i].id+' sails bow first at '+t.toFixed(1));});
  previous=poses.map(p=>({...p}));
}
assert.ok(worst.island>=KEEP_OUT&&worst.turbine>=KEEP_OUT&&worst.boats>=KEEP_OUT,'keep-out over ten minutes: '+JSON.stringify(worst));
const box=new THREE.BoxGeometry(),mat=new THREE.MeshBasicMaterial(),loads=[];
const template=()=>{const g=new THREE.Group();g.add(new THREE.Mesh(box,mat));for(const name of['rig','lamp']){const p=new THREE.Group();p.name=name;p.position.set(.5,1,0);p.add(new THREE.Mesh(box,mat));g.add(p);}return g;};
const group=new THREE.Group(),harbour=createHarbour({group,load:(id,lod)=>{loads.push(id+':'+lod);return Promise.resolve(template());}});
harbour.setTier('high');harbour.start();await new Promise(r=>setTimeout(r,0));
assert.deepEqual(loads.sort(),['harbour-barge:1','harbour-buoy:1','harbour-launch:1','sail-skiff:1']);
const m=new THREE.Matrix4(),p=new THREE.Vector3(),at=(id,row,i=0)=>{harbour.meshes(id)[row].getMatrixAt(i,m);return m;};
harbour.update(100,1,{x:1,z:0,strength:.5},true);const pose=craftPoses(100)[0];p.setFromMatrixPosition(at('harbour-launch',0));
assert.ok(Math.abs(p.x-pose.x)<1e-6&&Math.abs(p.z-pose.z)<1e-6&&Math.abs(p.y-(SEA_LEVEL+HEAVE*seaHeight(pose.x,pose.z,100)))<1e-6,'the launch rides the wave');
const hull=at('sail-skiff',0).clone(),rigA=at('sail-skiff',1).clone();harbour.update(100,1,{x:-1,z:0,strength:.5},true);
assert.ok(at('sail-skiff',0).equals(hull)&&!at('sail-skiff',1).equals(rigA),'the rig follows the wind, the hull does not');
assert.equal(at('harbour-buoy',2).determinant(),0,'lanterns dark by day');harbour.update(100,0,{x:1,z:0,strength:.5},false);assert.notEqual(at('harbour-buoy',2).determinant(),0,'lanterns lit at night');
let lit=0;for(let t=0;t<3;t+=.05){harbour.update(t,0,{x:1,z:0,strength:.5},true);if(at('harbour-buoy',2).determinant()!==0)lit++;}assert.ok(lit>=3&&lit<=15,'lanterns blink: '+lit);
const clones=[THREE.Matrix4,THREE.Vector3,THREE.Quaternion].map(C=>{const o=C.prototype.clone;let n=0;C.prototype.clone=function(){n++;return o.call(this);};return {C,o,count:()=>n};});
const reuse=craftPoses(5);assert.equal(craftPoses(6,reuse),reuse);for(let i=0;i<1000;i++)harbour.update(200+i/60,.3,{x:1,z:0,strength:.4},true);
for(const c of clones){assert.equal(c.count(),0,'update clones '+c.C.name);c.C.prototype.clone=c.o;}
harbour.update(300,1,{x:1,z:0,strength:.5},true);harbour.setTier('low');await new Promise(r=>setTimeout(r,0));harbour.update(400,1,{x:1,z:0,strength:.5},true);
assert.ok(loads.includes('harbour-launch:2')&&harbour.stats().lod===2,'low tier loads LOD 2');p.setFromMatrixPosition(at('harbour-launch',0));
assert.ok(Math.abs(p.x-craftPoses(300)[0].x)<1e-6,'low tier: the boats keep their place');assert.equal(harbour.stats().craft,5);
harbour.dispose();assert.equal(group.children.length,0);
console.log(`System World harbour: keep-out ${Object.entries(worst).map(([k,v])=>k+' '+v.toFixed(1)).join(', ')} m; wave ride, rig, lanterns, tiers and allocation verified.`);
```

**Run**
- `node scripts/test-system-world-harbour.mjs` → PASS. Report the keep-out line.
- `node scripts/test-system-world-lamps.mjs`, `node scripts/test-system-world-wind.mjs` → PASS. Esbuild bundle check → `ok`.
- `node scripts/system-world-survey.mjs --out reports/system-world-survey/phase-4-t6 --tier high --only overview-day` → page errors 0.

**Commit:** `feat(system-world): harbour traffic (launch, barge, skiffs, buoys) riding the sea's own waves`

---

## Task 7 — Gulls

**Files:** new `ui/js/desktop/apps/sysworld-gulls.js`, new `scripts/test-system-world-gulls.mjs`, `ui/js/desktop/apps/sysworld-scene.js`.

**Steps**
- [ ] Write the test, run → **FAIL**. Write the module, run → PASS.
- [ ] **`sysworld-scene.js`.**
  - Import `import { createGulls } from './sysworld-gulls.js';` after the harbour import.
  - `  const harbour = createHarbour({ group: world, load: lifeLoad });` → append ` const gulls = createGulls({ group: world, load: lifeLoad });`.
  - ` harbour.start();` → ` harbour.start(); gulls.start();`.
  - `harbour.setTier(tier);` → `harbour.setTier(tier); gulls.setTier(tier);`.
  - After `    harbour.update(seaTime, mood.day, wind, !reduced);` add `    gulls.update(seaTime, harbour.barge());`.
  - `harbour.dispose();` → `harbour.dispose(); gulls.dispose();`.
  - In `stats()`, after `harbour:harbour.stats(),` insert `gulls:gulls.stats(),`.

### `ui/js/desktop/apps/sysworld-gulls.js` (complete)

```js
import * as THREE from 'three';
import {createPartInstances} from './sysworld-parts.js';
// A small flock (phase 4): even gulls circle the harbour mouth, odd ones circle the barge. Body and wings are instanced;
// matrices come from closed-form paths of the atmosphere clock (frozen under reduced motion). 12 on high/ultra, 6 on
// medium, none on low. The gull flies towards +X; wing_l (glTF -Z) and wing_r hinge about X at the shoulders.
export const FLOCK=Object.freeze({low:0,medium:6,high:12,ultra:12}),HARBOUR_SKY=Object.freeze({x:0,z:104});
export function gullPose(i,t,barge,out){
  const follow=i%2===1,cx=follow?barge.x:HARBOUR_SKY.x,cz=follow?barge.z:HARBOUR_SKY.z,dir=i%3===0?-1:1;
  const r=(follow?12:24)+(i*3.7)%9,w=dir*(follow?.42:.26)*(1+(i%4)*.06),a=w*t+i*1.9;
  out.x=cx+r*Math.cos(a);out.z=cz+r*Math.sin(a);out.y=(follow?13:17)+(i*2.3)%7+1.1*Math.sin(t*.55+i);
  out.heading=Math.atan2(-r*Math.cos(a)*w,-r*Math.sin(a)*w);out.bank=-Math.sign(w)*.32;
  // Three wing beats, then a glide with the wings raised a little.
  const cycle=(t*.21+i*.37)%1;out.flap=cycle<.3?.55*Math.sin(cycle/.3*Math.PI*6):.08;
  return out;
}
export function createGulls({group,load}){
  const holder=new THREE.Group();holder.name='gulls';group.add(holder);
  const pose={x:0,y:0,z:0,heading:0,bank:0,flap:0},m=new THREE.Matrix4(),wing=new THREE.Matrix4(),e=new THREE.Euler(),q=new THREE.Quaternion(),v=new THREE.Vector3(),one=new THREE.Vector3(1,1,1);
  let parts=null,tier='high',started=false,disposed=false,flap=0;
  const turn=name=>name==='wing_l'?wing.makeRotationX(flap):name==='wing_r'?wing.makeRotationX(-flap):null;
  return {
    start(){if(started)return;started=true;load('gull',1).then(t=>{if(disposed)return;parts=createPartInstances(t,FLOCK.ultra,{movable:['wing_l','wing_r']});parts.meshes.forEach(x=>{x.castShadow=false;holder.add(x);});},()=>{});},
    setTier(value){tier=value;},
    update(time,barge){
      if(!parts)return;const count=FLOCK[tier]??0;
      for(let i=0;i<FLOCK.ultra;i++){
        if(i>=count){parts.hide(i);continue;}
        gullPose(i,time,barge,pose);flap=pose.flap;e.set(pose.bank,pose.heading,0,'YZX');q.setFromEuler(e);v.set(pose.x,pose.y,pose.z);parts.set(i,m.compose(v,q,one),turn);
      }
      parts.commit();
    },
    stats:()=>({count:parts?FLOCK[tier]??0:0,loaded:!!parts}),
    dispose(){disposed=true;parts?.dispose();holder.removeFromParent();},
  };
}
```

### `scripts/test-system-world-gulls.mjs` (complete)

```js
// Phase 4 gulls: counts per tier, paths over the harbour and around the barge, flapping wings, hidden extras.
import assert from 'node:assert/strict';
import * as THREE from 'three';
import {FLOCK,HARBOUR_SKY,gullPose,createGulls} from '../ui/js/desktop/apps/sysworld-gulls.js';
const barge={x:60,z:180},pose={};
for(let i=0;i<12;i++)for(let t=0;t<120;t+=.5){gullPose(i,t,barge,pose);const c=i%2?barge:HARBOUR_SKY,d=Math.hypot(pose.x-c.x,pose.z-c.z);
  assert.ok(d>=11.9&&d<=33.1&&pose.y>=11.9&&pose.y<=25.2&&Math.abs(pose.flap)<=.56,'gull '+i+' off its circle: '+JSON.stringify(pose));}
const box=new THREE.BoxGeometry(),mat=new THREE.MeshBasicMaterial(),root=new THREE.Group();root.add(new THREE.Mesh(box,mat));
for(const [name,z]of[['wing_l',-.06],['wing_r',.06]]){const w=new THREE.Group();w.name=name;w.position.set(0,.03,z);const mesh=new THREE.Mesh(box,mat);mesh.position.z=z*6;w.add(mesh);root.add(w);}
const group=new THREE.Group(),gulls=createGulls({group,load:()=>Promise.resolve(root)});gulls.start();await new Promise(r=>setTimeout(r,0));
const meshes=group.getObjectByName('gulls').children,m=new THREE.Matrix4(),det=(mesh,i)=>{mesh.getMatrixAt(i,m);return m.determinant();};
for(const [tier,count]of Object.entries(FLOCK)){gulls.setTier(tier);gulls.update(10,barge);assert.equal(gulls.stats().count,count,tier);
  for(let i=0;i<12;i++)assert.equal(det(meshes[0],i)!==0,i<count,tier+' gull '+i);}
gulls.setTier('high');gulls.update(0,barge);const a=new THREE.Matrix4();meshes[1].getMatrixAt(0,a);gulls.update(.2,barge);meshes[1].getMatrixAt(0,m);assert.ok(!a.equals(m),'wings move');
gulls.dispose();assert.equal(group.children.length,0);
console.log('System World gulls: tiers 0/6/12/12, harbour and barge circles, wing beats verified.');
```

**Run**
- `node scripts/test-system-world-gulls.mjs` → PASS. `node scripts/test-system-world-harbour.mjs` → PASS. Esbuild bundle check → `ok`.

**Commit:** `feat(system-world): gulls over the harbour and around the barge (instanced body and wings)`

---

## Task 8 — Neon signs

**Files:** `ui/js/desktop/apps/sysworld-furnishing.js`, `scripts/test-system-world-furnishing.mjs`.

**Steps**
- [ ] **Test first.**
  - Add `import {LOBBIES} from '../ui/js/desktop/apps/sysworld-lobbies.js';` and change the layout import to `import {streets,stations,hoverPads,tramWaypoints,interiors,towers} from '../ui/js/desktop/apps/sysworld-layout.js';`.
  - Before the final `console.log`, add:
    ```js
    // Neon (decision 9): wide signs on the six rectangular halls' top beams, round signs beside the pavilion doors,
    // blade signs on the data-tower fronts. Signs have no colliders.
    {
      const neon=items.filter(i=>i.area==='neon');
      assert.deepEqual(['neon-wave','neon-ring','neon-blade'].map(a=>neon.filter(i=>i.asset===a).length),[6,3,6]);
      for(const i of neon.filter(i=>i.asset==='neon-wave')){const l=LOBBIES.find(l=>l.rect&&Math.abs((l.rect[0]+l.rect[2])/2-i.x)<1e-9);
        assert.ok(l&&Math.abs(i.z-l.front)<=.5&&i.y>=l.ceiling+.9&&i.y+1.1<=l.ceiling+3&&i.angle===0,'wave off its beam '+JSON.stringify(i));}
      for(const i of neon.filter(i=>i.asset==='neon-ring'))assert.ok(interiors.some(r=>Math.abs(i.z-r.doorZ)<=.5&&Math.abs(i.x-r.x)<=6)&&i.angle===Math.PI&&i.y>=2,'ring off a pavilion front '+JSON.stringify(i));
      for(const i of neon.filter(i=>i.asset==='neon-blade'))assert.ok(towers.some(t=>Math.abs(i.z-t.z-6.5)<1e-9&&Math.abs(Math.abs(i.x-t.x)-6.6)<1e-9)&&i.y===6,'blade off a tower front '+JSON.stringify(i));
      assert.ok(neon.every(i=>!F.SIZES[i.asset]),'signs carry no colliders');
    }
    ```
  - Run → **FAIL** (`[0,0,0]`).
- [ ] **`sysworld-furnishing.js`.**
  - Before `let cache=null;` add:
    ```js
    // Neon (decision 9), mount points from the facade registries: a wide sign standing on each rectangular hall's top beam
    // (ceiling + 0.9 m, 0.27 m in front of the facade line, centred on the hall); a round sign on the wall beside each
    // pavilion door (the pavilions face north); blade signs on the data towers' south fronts, 6 m up, projecting south.
    // Survey tuning ranges: wave y +0..+0.4, z +0.2..+0.5; ring y 2.4..3.4; blade TOWER_FRONT 6.3..8.1.
    export const TOWER_FRONT=6.5;
    export function neonSigns(){
      const out=[];
      for(const l of LOBBIES)if(l.rect)out.push(item('neon-wave',(l.rect[0]+l.rect[2])/2,l.front+.27,0,'neon',l.ceiling+.95));
      for(const r of interiors)out.push(item('neon-ring',r.x+4,r.doorZ-.2,PI,'neon',2.6));
      for(const t of towers)for(const s of[-1,1])out.push(item('neon-blade',t.x+s*6.6,t.z+TOWER_FRONT,0,'neon',6));
      return out;
    }
    ```
  - In `furnishing()`, `return cache=Object.freeze(accepted);` → `return cache=Object.freeze([...accepted,...neonSigns().map(i=>Object.freeze(i))]);`.

**Run**
- `node scripts/test-system-world-furnishing.mjs` → PASS. `node scripts/test-system-world-living.mjs`, `node scripts/test-system-world-hover.mjs` → PASS (signs add no solids).
- Night picture check:
  ```
  node scripts/system-world-survey.mjs --out reports/system-world-survey/phase-4-neon --tier high --only facade-agent-night
  ```
  Expected: page errors 0. Visual: thin glowing tubes with no bloom blob. If a sign floats or sinks into a wall, tune within the listed ranges and record the values.

**Commit:** `feat(system-world): neon wave, ring and blade signs at registry mount points`

---

## Task 9 — Steam and fireflies

**Files:** new `ui/js/desktop/apps/sysworld-particles.js`, new `scripts/test-system-world-particles.mjs`, `ui/js/desktop/apps/sysworld-scene.js`.

**Steps**
- [ ] Write the test, run → **FAIL**. Write the module, run → PASS.
- [ ] **`sysworld-scene.js`.**
  - Import `import { createParticles } from './sysworld-particles.js';`.
  - After the statement `  const weather = createWeather(scene,{…});` add `  const particles = createParticles(scene);`.
  - In `resize()`, `flow.setPointScale(height * dpr);` → `flow.setPointScale(height * dpr); particles.setPointScale(height * dpr);`.
  - `gulls.setTier(tier);` → `gulls.setTier(tier); particles.setTier(tier);`.
  - After `    gulls.update(seaTime, harbour.barge());` add `    particles.setMood(mood.day, mood.weather); particles.update(dt, !reduced, wind);`.
  - `gulls.dispose();` → `gulls.dispose(); particles.dispose();`.
  - In `stats()`, after `gulls:gulls.stats(),` insert `particles:particles.stats(),`.

### `ui/js/desktop/apps/sysworld-particles.js` (complete)

```js
import * as THREE from 'three';
import {LOBBIES} from './sysworld-lobbies.js';
import {streetModules} from './sysworld-ground.js';
import {SURFACES} from './sysworld-furnishing.js';
// Steam and fireflies (phase 4). Steam: soft camera-facing quads per vent, one instanced draw (at most 60), animated
// in the shader and drifting with the wind. Fireflies: twinkling points over the lawns and gardens on clear nights
// (at most 200). Both off on low and with reduced motion; medium shows half.
export const STEAM_MAX=60,FIREFLY_MAX=200,LIMITS=Object.freeze({low:[0,0],medium:[30,100],high:[60,200],ultra:[60,200]});
// Vents: the foundry's three cooling towers (top 9.6 m), its three roof units (14.2 m, 4.5 m north of the origin), five street drains.
export function steamVents(){
  const infra=LOBBIES.find(l=>l.id==='infra'),out=[],drains=streetModules().filter(m=>m.id==='street-drain');
  for(const [x,z]of infra.towers)out.push({x,y:9.6,z,size:2.4,count:10});
  for(const [x]of infra.towers)out.push({x,y:14.2,z:infra.origin[1]-4.5,size:1.3,count:5});
  for(let k=0;k<5;k++){const m=drains[Math.floor((k+.5)*drains.length/5)];out.push({x:m.x,y:.05,z:m.z,size:.8,count:3});}
  return out;
}
export function fireflyAreas(){return [...SURFACES.filter(s=>s.kind==='lawn'),...[[-79,68],[-79,-67],[55,4],[8,40],[45,-23]].map(([x,z])=>({x0:x-3,z0:z-3,x1:x+3,z1:z+3}))];}
const STEAM_VERTEX=`attribute vec4 aVent;attribute vec2 aSeed;uniform float time;uniform vec4 wind;varying float vA;varying vec2 vUv;
void main(){vUv=uv;float life=fract(time*aSeed.y+aSeed.x);
vec3 c=aVent.xyz+vec3(wind.x,0.0,wind.y)*life*(1.0+3.0*wind.z)+vec3(sin(aSeed.x*41.0)*0.35,life*aVent.w*2.4,cos(aSeed.x*29.0)*0.35);
vec4 mv=viewMatrix*vec4(c,1.0);mv.xy+=position.xy*aVent.w*(0.55+1.4*life);vA=smoothstep(0.0,0.12,life)*(1.0-life);gl_Position=projectionMatrix*mv;}`;
const STEAM_FRAGMENT=`uniform vec3 color;uniform float strength;varying float vA;varying vec2 vUv;
void main(){float d=length(vUv-0.5)*2.0;float a=(1.0-smoothstep(0.2,1.0,d))*vA*strength;if(a<0.004)discard;gl_FragColor=vec4(color,a);}`;
const FLY_VERTEX=`attribute vec4 aSeed;uniform float time,pointScale;varying float vA;
void main(){vec3 p=position+vec3(sin(time*0.7+aSeed.x*20.0)*0.6,sin(time*1.3+aSeed.y*9.0)*0.3,cos(time*0.6+aSeed.z*13.0)*0.6);
vec4 mv=modelViewMatrix*vec4(p,1.0);vA=pow(max(0.0,sin(time*(1.5+aSeed.w)+aSeed.x*50.0)),6.0);gl_PointSize=clamp(2.6*pointScale/max(1.0,-mv.z),1.5,5.0);gl_Position=projectionMatrix*mv;}`;
const FLY_FRAGMENT=`uniform vec3 color;uniform float strength;varying float vA;void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;gl_FragColor=vec4(color*pow(1.0-d,1.5)*vA*strength,1.0);}`;
export function createParticles(scene){
  const group=new THREE.Group();group.name='city-particles';scene.add(group);
  let seed=7;const rnd=()=>(seed=(Math.imul(seed,1664525)+1013904223)>>>0)/4294967296;
  const timeU={value:0},windU={value:new THREE.Vector4()},pointScale={value:900};
  const plane=new THREE.PlaneGeometry(1,1),steamGeometry=new THREE.InstancedBufferGeometry();
  steamGeometry.index=plane.index;steamGeometry.setAttribute('position',plane.getAttribute('position'));steamGeometry.setAttribute('uv',plane.getAttribute('uv'));
  const vent=new Float32Array(STEAM_MAX*4),phase=new Float32Array(STEAM_MAX*2);let quads=0;
  for(const v of steamVents())for(let k=0;k<v.count&&quads<STEAM_MAX;k++,quads++){vent.set([v.x,v.y,v.z,v.size],quads*4);phase.set([k/v.count+rnd()*.1,.07+rnd()*.06],quads*2);}
  steamGeometry.setAttribute('aVent',new THREE.InstancedBufferAttribute(vent,4));steamGeometry.setAttribute('aSeed',new THREE.InstancedBufferAttribute(phase,2));steamGeometry.instanceCount=quads;
  const steamMaterial=new THREE.ShaderMaterial({uniforms:{time:timeU,wind:windU,color:{value:new THREE.Color(.6,.62,.64)},strength:{value:.32}},vertexShader:STEAM_VERTEX,fragmentShader:STEAM_FRAGMENT,transparent:true,depthWrite:false});
  const steam=new THREE.Mesh(steamGeometry,steamMaterial);steam.frustumCulled=false;steam.name='steam';group.add(steam);
  const areas=fireflyAreas(),weights=areas.map(a=>(a.x1-a.x0)*(a.z1-a.z0)),total=weights.reduce((s,w)=>s+w,0),pos=new Float32Array(FIREFLY_MAX*3),fly=new Float32Array(FIREFLY_MAX*4);
  for(let i=0;i<FIREFLY_MAX;i++){let r=rnd()*total,k=0;while(r>weights[k]&&k<areas.length-1)r-=weights[k++];const a=areas[k];
    pos.set([a.x0+rnd()*(a.x1-a.x0),.5+rnd()*1.7,a.z0+rnd()*(a.z1-a.z0)],i*3);fly.set([rnd(),rnd(),rnd(),rnd()],i*4);}
  const flyGeometry=new THREE.BufferGeometry();flyGeometry.setAttribute('position',new THREE.BufferAttribute(pos,3));flyGeometry.setAttribute('aSeed',new THREE.BufferAttribute(fly,4));
  const flyMaterial=new THREE.ShaderMaterial({uniforms:{time:timeU,pointScale,color:{value:new THREE.Color(.75,1,.45)},strength:{value:0}},vertexShader:FLY_VERTEX,fragmentShader:FLY_FRAGMENT,transparent:true,depthWrite:false,blending:THREE.AdditiveBlending});
  const flies=new THREE.Points(flyGeometry,flyMaterial);flies.frustumCulled=false;flies.name='fireflies';group.add(flies);
  let tier='high',day=1,weather='clear',time=0;
  return {
    setTier(value){tier=value;},
    setPointScale(value){pointScale.value=value;},
    setMood(d,w){day=d;weather=w;},
    update(dt,animated,wind){
      const [steamCount,flyCount]=LIMITS[tier]??LIMITS.high,night=1-day;
      if(animated)time+=Math.min(.1,Math.max(0,dt));timeU.value=time;windU.value.set(wind.x,wind.z,wind.strength,0);
      steam.visible=animated&&steamCount>0;steamGeometry.instanceCount=Math.min(quads,steamCount);steamMaterial.uniforms.color.value.setScalar(.12+.5*day);
      flies.visible=animated&&flyCount>0&&weather==='clear'&&night>.7;flyGeometry.setDrawRange(0,flyCount);flyMaterial.uniforms.strength.value=.85*Math.min(1,(night-.7)/.2);
    },
    stats:()=>({steam:steam.visible?steamGeometry.instanceCount:0,fireflies:flies.visible?flyGeometry.drawRange.count:0}),
    dispose(){group.removeFromParent();plane.dispose();steamGeometry.dispose();steamMaterial.dispose();flyGeometry.dispose();flyMaterial.dispose();},
  };
}
```

### `scripts/test-system-world-particles.mjs` (complete)

```js
// Phase 4 steam and fireflies: vents and budgets, per tier, night and clear sky only for fireflies, off without motion.
import assert from 'node:assert/strict';
import * as THREE from 'three';
import {steamVents,fireflyAreas,createParticles,STEAM_MAX,FIREFLY_MAX} from '../ui/js/desktop/apps/sysworld-particles.js';
const vents=steamVents();assert.equal(vents.reduce((n,v)=>n+v.count,0),STEAM_MAX,'60 steam quads');
assert.deepEqual(vents.slice(0,3).map(v=>[v.x,v.y,v.z]),[[-50.5,9.6,-48],[-43,9.6,-48],[-35.5,9.6,-48]],'cooling tower tops');
assert.ok(fireflyAreas().length>=8);
const scene=new THREE.Scene(),p=createParticles(scene),wind={x:1,z:0,strength:.4},cases=[
  ['high',0,'clear',true,60,200],['ultra',0,'clear',true,60,200],['medium',0,'clear',true,30,100],['low',0,'clear',true,0,0],
  ['high',1,'clear',true,60,0],['high',0,'rain',true,60,0],['high',0,'clear',false,0,0]];
for(const [tier,day,weather,animated,steam,fireflies]of cases){p.setTier(tier);p.setMood(day,weather);p.update(1/60,animated,wind);assert.deepEqual(p.stats(),{steam,fireflies},JSON.stringify([tier,day,weather,animated]));}
const flies=scene.getObjectByName('fireflies'),pos=flies.geometry.getAttribute('position');assert.equal(pos.count,FIREFLY_MAX);
for(let i=0;i<pos.count;i++)assert.ok(fireflyAreas().some(a=>pos.getX(i)>=a.x0&&pos.getX(i)<=a.x1&&pos.getZ(i)>=a.z0&&pos.getZ(i)<=a.z1),'firefly outside the green');
p.dispose();assert.equal(scene.children.length,0);
console.log('System World particles: 60 steam quads at 11 vents, 200 fireflies over the green, tiers, night, weather and motion verified.');
```

**Run**
- `node scripts/test-system-world-particles.mjs` → PASS. Esbuild bundle check → `ok`.

**Commit:** `feat(system-world): steam from the foundry and drains, fireflies over the green on clear nights`

---

## Task 10 — Charging plaza and cafe as workplaces; parcel sorter

**Files:** `ui/js/desktop/apps/sysworld-experience.js`, `sysworld-society.js`, `sysworld-places.js`, `scripts/test-system-world-living.mjs`.

**Steps**
- [ ] **Test first** (`test-system-world-living.mjs`):
  - `experience.stats().society.places.length===12` → `===14`;
  - `for(const place of ['repair-bay','relay-mast','kinetic-fountain','glass-garden'])` → `for(const place of ['repair-bay','relay-mast','kinetic-fountain','glass-garden','charge-plaza','harbour-cafe'])`.

  Run → **FAIL** (12 places).
- [ ] **`sysworld-experience.js`.**
  - After the lobbies import add `import {furnishingPlaces} from './sysworld-furnishing.js';`.
  - After `      for(const {place,entry}of lobbyPlaces(options.kit3))society.addPlace(place,entry);` add:
    ```js
          // Kit workplaces (phase 4): the charging plaza's posts and the harbour cafe, after the residents spawned.
          for(const {place,entry}of furnishingPlaces(options.kit3))society.addPlace(place,entry);
    ```
- [ ] **`sysworld-society.js`.** In `nearby()`, `if(p.lobby)continue;` → `if(p.lobby||p.kit)continue;`.
- [ ] **`sysworld-places.js`** (backlog 13). `{id:'parcel-sorter',district:'missions',x:27.6,z:49.4,role:'courier'},` → `{id:'parcel-sorter',district:'missions',x:27.6,z:51.4,role:'courier'},` and above the array add `// The parcel sorter stands 1.5 m clear of the mission stairs (z 46.5..48.1) since phase 4.`.

**Run**
- `node scripts/test-system-world-living.mjs` → PASS with every limit unchanged. Report `byPlace` for `charge-plaza`, `harbour-cafe`, `parcel-sorter`.
- If a new place is not served:
  - check its slots are routable (`finder.route` from the nearest pavement node, in a scratch script);
  - move the spots within the plaza or terrace (`CAFE_POINTS`, post z values);
  - never drop the assertion; escalate if routing is fine and it still fails.
- `node scripts/test-system-world-furnishing.mjs` (the sorter is reserved; pieces near it may change), `node scripts/test-system-world-hover.mjs`, `node scripts/test-system-world-lobby-walk.mjs` → PASS.
- `go test ./ui -run '^TestDesktopSysWorld' -count=1` → PASS.

**Commit:** `feat(system-world): charging plaza and harbour cafe as resident workplaces; parcel sorter clear of the stairs`

---

## Task 11 — Survey scenes and documentation

**Files:** `scripts/system-world-survey.mjs`, `scripts/system-world-survey.scenes.json`, `assets/system-world/AGENTS.md`, `assets/system-world/README.md`, `ui/3d/system-world/AGENTS.md`, `plans/2026-11-09-system-world-beauty-design.md`.

**Steps**
- [ ] **Survey harness.** In `EXPOSE`, `'model, catalogV3: () => catalog3, lobbies: lobbyRender, '` → `'model, catalogV3: () => catalog3, lobbies: lobbyRender, harbour, gulls, particles, '`; add `harbour, gulls, particles` to the header comment's `__sw` list.
- [ ] **Scenes.** Append (each with `"fov":43,"weather":"clear","frames":40,"waitMs":400` unless noted):
  ```json
  {"name":"life-park-day","position":[-30,7,-74],"target":[-44,1,-89],"environment":"day"},
  {"name":"life-market-day","position":[52,7,-74],"target":[41,1.5,-88],"environment":"day"},
  {"name":"life-grove-day","position":[70,6,-78],"target":[79,2,-89],"environment":"day"},
  {"name":"life-plaza-day","position":[64,6,-48],"target":[78,1.2,-56],"environment":"day"},
  {"name":"life-terrace-evening","position":[66,5,60],"target":[79,1,69],"environment":"evening"},
  {"name":"life-street-trees-day","position":[-60,4,-32],"target":[-22,2.5,-32],"environment":"day"},
  {"name":"life-street-trees-night","position":[-60,4,-32],"target":[-22,2.5,-32],"environment":"night"},
  {"name":"life-harbour-dusk","position":[-10,16,86],"target":[-45,-3,100],"environment":"evening"},
  {"name":"life-skiffs-day","position":[92,9,-2],"target":[118,-3,4],"environment":"day"},
  {"name":"life-gulls-day","position":[5,9,84],"target":[0,18,104],"environment":"day"},
  {"name":"life-fireflies-night","position":[-30,5,-80],"target":[-44,1,-90],"environment":"night"},
  {"name":"life-neon-night","position":[40,6,-32],"target":[40,8,-47],"environment":"night"},
  {"name":"life-steam-day","position":[-43,9,-30],"target":[-43,11,-50],"environment":"day"}
  ```
  Write them as full objects in the file's existing one-line style.
- [ ] **Run:**
  ```
  node scripts/system-world-survey.mjs --out reports/system-world-survey/phase-4 --tier high --only <the 13 names>
  ```
  Expected: 13 PNGs, `page errors: 0`, `cameraMovedByCollision: 0`. If a camera was moved, raise it 1 m.
- [ ] **Visual acceptance:**
  - park, market, grove, plaza and terrace read as places;
  - trees line the street;
  - neon tubes are thin, with no bloom blobs at night;
  - boats sit on the water, not above it;
  - gulls are visible;
  - fireflies at night;
  - steam rises from the foundry towers.

  Allowed tuning, recorded in the hand-back:
  - steam `strength` 0.2–0.4;
  - firefly `strength` 0.6–0.9;
  - `HEAVE` 0.15–0.35;
  - lawn and paving colours;
  - the neon ranges of Task 8.
- [ ] **Docs** (concise, existing style):
  - **`assets/system-world/AGENTS.md`.**
    - `life_assets3.py` is registered; 46 designs.
    - Part nodes `rig`, `lamp`, `banner`, `wing_l`, `wing_r` with pivots; boats have their origin on the waterline, bow +X.
    - The charge-post workpoint sits on the pad and its collider behind it.
    - Furniture `navigation.colliders` mirror `SIZES` in `sysworld-furnishing.js` (checked by `test-system-world-furnishing.mjs`).
  - **`README.md`.**
    - Runtime payload: kit v3 46 designs.
    - City life: furniture by rules, wind, harbour, gulls, neon, steam, fireflies; which designs load at first display and which later; tier table (low: none of the motion, boats parked).
  - **`ui/3d/system-world/AGENTS.md`.**
    - Ownership table from this plan's Architecture.
    - Kit placements are solids.
    - The lawn and paving materials are scene-owned.
    - Verification scripts: `-furnishing`, `-wind`, `-harbour`, `-gulls`, `-particles`.
  - **Design doc.** The phase 4 roadmap row points to `plans/2026-11-10-system-world-beauty-phase-4.md`.

**Run:** `go test ./ui -run '^TestDesktopSysWorld' -count=1` → PASS.

**Commit:** `docs(system-world): city life (furnishing rules, wind, harbour, gulls, neon, particles); survey scenes`

---

## Task 12 — Markers, browser checks, final pins, bundle, regression, budgets, frame cost

**Files:** `ui/desktop_sysworld_test.go`, `ui/desktop_sysworld_city_browser_test.go`, `scripts/test-system-world-v3.mjs`, `ui/js/vendor/system-world/*` (generated).

**Steps**
- [ ] **Markers** in `TestDesktopSysWorldAppMarkers`. Add:
  ```go
  "js/desktop/apps/sysworld-furnishing.js": {"export function furnishing()", "export function violations(", "export const SIZES", "export function furnishingPlaces(", "export function neonSigns()", "export const SURFACES", "export const NEAR"},
  "js/desktop/apps/sysworld-parts.js":      {"export function createPartInstances("},
  "js/desktop/apps/sysworld-harbour.js":    {"export function createHarbour(", "export function craftPoses(", "HEAVE*seaHeight("},
  "js/desktop/apps/sysworld-gulls.js":      {"export function createGulls(", "'wing_l'"},
  "js/desktop/apps/sysworld-particles.js":  {"export function createParticles(", "export function steamVents()", "STEAM_MAX"},
  ```
  Then append:
  - to the `sysworld-scene.js` list: `"furnishing().map(", "districts,placements,kitPlacements)", "createHarbour(", "createGulls(", "createParticles(", "surfaces.setWind(", "NEAR_HEIGHT"`;
  - to `sysworld-atmosphere.js`: `"export function windAt(", "export function seaHeight("`;
  - to `sysworld-surfaces.js`: `"SW_SWAY"`;
  - to `sysworld-navigation.js`: `"sizes[p.asset]"`;
  - to `sysworld-experience.js`: `"furnishingPlaces(options.kit3)"`.

  Run `gofmt -w ui/desktop_sysworld_test.go`.
- [ ] **City browser test.**
  - After the eval throwing `'Lobby interiors must not load at the overview: '` add:
    ```go
    	page.Timeout(30 * time.Second).MustWait(`()=>{const s=SysWorldApp.inspect(cityId);return !!s.harbour&&s.harbour.craft===5&&s.gulls.loaded;}`)
    	page.MustEval(`()=>{const s=SysWorldApp.inspect(cityId);if(s.furniture<300||s.nearHidden!==true||s.gulls.count!==12||s.harbour.buoys!==6||typeof s.atmosphere.wind!=='number'||s.particles.steam>60||s.particles.fireflies>200)throw Error('City life incomplete: '+JSON.stringify({f:s.furniture,n:s.nearHidden,h:s.harbour,g:s.gulls,p:s.particles,w:s.atmosphere.wind}));}`)
    ```
  - After the eval throwing `'Low tier must keep the lobby stand-ins: '` add:
    ```go
    	page.MustEval(`()=>{const s=SysWorldApp.inspect(cityId);if(s.gulls.count!==0||s.particles.steam!==0||s.particles.fireflies!==0||s.atmosphere.wind!==0||s.harbour.lod!==2)throw Error('Low tier must drop gulls, steam, fireflies and wind and park the boats: '+JSON.stringify({g:s.gulls,p:s.particles,w:s.atmosphere.wind,h:s.harbour}));}`)
    ```
- [ ] **Final pin** in `test-system-world-v3.mjs`. Replace the two lines `assert.ok(lifeShipped === 0 || lifeShipped === 15, …);` and `assert.equal(manifest.assets.length, 31 + lifeShipped, …);` with:
  ```js
  assert.equal(lifeShipped, 15, 'The fifteen life designs ship');
  assert.equal(manifest.assets.length, 46, '6 street + 12 superseded + 7 furniture + 6 interiors + 15 life');
  ```
- [ ] **Bundle:** `node scripts/build-system-world.js`, then `node scripts/build-system-world.js --check` → `verified`.
- [ ] **Final regression** (PowerShell, all must pass):
  ```powershell
  Get-ChildItem scripts/test-system-world*.mjs | ForEach-Object { node $_.FullName; if ($LASTEXITCODE) { throw $_.Name } }
  python assets/system-world/check_assets.py
  node scripts/build-system-world-v3.mjs --check
  node scripts/build-system-world.js --check
  go test ./ui -run '^TestDesktopSysWorld' -count=1
  $env:AURAGO_RUN_BROWSER_SMOKE='1'; $env:AURAGO_SYSTEM_WORLD_MATRIX='1'; $env:AURAGO_SYSTEM_WORLD_EXPANSION='1'; go test ./ui -run '^TestDesktopAuroraBrowser$' -count=1 -timeout 30m
  ```
- [ ] **Survey set** (0 page errors in each):
  - `--tier high --out reports/system-world-survey/phase-4-final --timeout 2400` (all scenes);
  - `--tier medium --out reports/system-world-survey/phase-4-medium --only life-street-trees-day,life-gulls-day,life-fireflies-night`;
  - `--tier low --out reports/system-world-survey/phase-4-low --only life-harbour-dusk,life-park-day`. On low: boats present but parked, no gulls, steam or fireflies.
- [ ] **Report budgets:**
  - model directory: the `model directory N bytes` line of `test-system-world-v3.mjs` (< 40 MiB);
  - first display: the expansion browser run's initial `loadedBytes` (< 20 MiB) and `overview-day` `stats.loadedBytes`, against phase 3;
  - whole app: `runtimeBytes` of `test-system-world-expansion.mjs` (< 80 MiB).
- [ ] **Frame-cost report:**
  - a table of `calls` and `triangles` for `overview-day` and `facade-agent-night`, before (Task 2 baseline) and after (phase-4-final), plus `life-street-trees-day`;
  - target: at most 60 extra calls at `overview-day`;
  - if it is exceeded, state the number and leave the decision to the coordinator (risk 2). Do not merge geometry in this task.

**Commit:** `chore(system-world): rebuild renderer bundle, pin city-life markers and checks, 46 kit designs`

---

## Test and pin changes at a glance

| Pin | Old | New |
| --- | --- | --- |
| `test-system-world-v3.mjs` designs | `=== 31` | Task 1: 31 or 46 (all fifteen life designs); Task 12: `=== 46` |
| `installCityColliders` | v1 bounds of `placements` | plus `kit` argument: every kit design's `navigation.colliders`, turned |
| `kitPlacements` | street modules | `streetPlacements` plus furniture and neon; street-models test uses `streetPlacements` |
| `obstacles` / `obstaclesFrom` | trams, racks, lamps, planters | plus kit pieces in street bands (circles from `SIZES`); navigator buckets |
| `createTraffic` | plain scan | 16 m buckets, same first-hit order (`{grid:false}` for the reference) |
| City tests (hover, living, ground, lobbies, lobby-walk) | v1 solids | plus furniture solids (limits unchanged) |
| `test-system-world-living.mjs` | 12 places; 4 served outdoor places | 14 places; plus `charge-plaza`, `harbour-cafe` |
| `test-system-world-surfaces.mjs` | — | sway only for `city.leaf`, instanced, not low; lawn never; key and uniform |
| New tests | — | `-furnishing`, `-wind`, `-harbour`, `-gulls`, `-particles`; traffic buckets section |
| `TestDesktopSysWorldAppMarkers` | — | 5 new files, scene, atmosphere, surfaces, navigation, experience strings |
| City browser test | — | city life present at overview; low tier drops motion and parks boats |
| First display | 6.1 MiB (phase 1) | about +0.26 MiB (furniture and signs); craft and gulls load later |

## Risks and open questions

1. **Living simulation is chaotic.** About 500 new solids and two new workplaces shift it.
   - Tasks 3 and 10 re-run it and report against the previous JSON line.
   - Levers in order: `GAP.living`, moving single pieces, the cafe and plaza spots.
   - Never a lower limit.
2. **Draw calls over target** (deviation 4).
   - Estimate at `overview-day`: trees 8, kiosk 10, banner masts 6, neon about 15, craft about 25, buoys about 6, gulls 7, slabs 2, steam 1 → about 80. At street level all `NEAR` designs add about 45 more.
   - Follow-up if the coordinator holds 60: one merged static mesh per material for all non-swaying furniture (pre-transformed vertices, about 17 calls instead of about 85; about 10 MB of vertex data). Its own plan.
3. **Triangles.** About 200 street trees at 1780 triangles plus 120 bollards at 924 at LOD 0, all drawn every frame (one instanced bounding sphere). Task 12 reports them; levers are `TREE_STEP` 16 and bollards on one side only.
4. **Start-up cost.** `furnishing()` runs at scene import (pedestrian graph plus about 1000 candidates × rules). Task 1 reports the milliseconds; above about 300 ms, cache the candidates' static checks per area.
5. **Charge-post collider** (Request 1). Without the fix, the contract assertion stops Task 0, and `standing()` would push the slot 1.4 m forward.
6. **Mount points not measured** (could-not-verify 3). The survey decides within the listed ranges. A blade colliding with a tower canopy moves to `TOWER_FRONT` 8.1.
7. **Neon bloom.** Thin tubes stay under threshold by design. If a sign blooms, set its LOD-0 brightness via distance only (`neon-*` near the camera) — a follow-up, not tuned by changing `bloom.threshold`.
8. **Sliding detail.** Instanced craft and gulls use world-space surface maps, so detail slides over moving hulls. It fades beyond 35 m and is accepted. Object-space clones (the drones' pattern) are the fix if visible.
9. **Mooring posts versus quay edge** (could-not-verify 4). Tuning z 78.6–79.4.
10. **Barge visibility.** At 150–230 m, fog dims the barge. If the gull scene shows only specks, bring the south leg to z 165; the keep-out test guards it.
11. **Traffic buckets** touch a HIGH-risk shared module. The equivalence test proves identical answers, including insertion order; the coordinator's `impact` and `detect_changes` must show only the listed callers.
12. **Third tree or hedge** (design). Not planned; needs `SIZES` entries and area slots if the coordinator adds them.

## Execution notes (2026-11-10)

Recorded after the implementation. The code is the truth; the body above is unchanged.

- **Street trees.** Evenly spaced between the inset plot corners at a target of 8 m (`TREE_STEP = 8`, `TREE_INSET = 1.4`), not every 12 m. `scripts/test-system-world-furnishing.mjs` asserts at least 100 street trees; the plan's 120 was an estimate. Truth: `ui/js/desktop/apps/sysworld-furnishing.js`.
- **Furnishing rules.** `GAP.footLink` (0.55 m) keeps the straight links from each stair foot to the nearest zebra nodes open. `GAP.frontage` (1.5 m) and `FRONTAGE_DEPTH` (18 m) keep street trees out of the front of stair flights. Truth: `sysworld-furnishing.js`; the stair rule is asserted in `scripts/test-system-world-furnishing.mjs`.
- **Reserved places.** The station pylon moved to z 67.4 (`PYLONS`, was 67). Three `RESERVED` boxes were widened: `service-cart` (z to 1.65), `charger-*` (x and z) and `rack-*` (z to 0.66). Truth: `sysworld-furnishing.js`.
- **District benches** have alternative offsets along the flight (`[3.2, 1.8, 2.2]`). Truth: the district generator in `sysworld-furnishing.js`.
- **Ground test.** `scripts/test-system-world-ground.mjs` routes the living places with the society's resident body (`residentShape`, reach about 0.5 m) instead of the former 1.29 m stand-in.
- **Charge post.** Request 1 was applied as written: `collider(-.26, .4, .26, .84, 1.9)`, which puts the collider at glTF z -0.84 to -0.4. Truth: `assets/system-world/life_assets3.py` `charge_post`.
- **Task 10.**
  - The plaza and the cafe register before the residents spawn (`sysworld-experience.js`).
  - After an outdoor job a resident prefers a free lobby workpoint within 70 m (`sysworld-society.js`).
  - The cafe's first serving spot is at x 79.2 (`CAFE_POINTS`, was 78.3).
  - The parcel sorter was not moved: `sysworld-places.js` keeps (27.6, 49.4). Backlog item 13 stays open.
  - `scripts/test-system-world-lobby-walk.mjs` waits for a free flight foot before it sets the visitor down.
- **Task 5.** Leaf sway grows with the height above 2 m, and `sway` is `.045` (`KINDS['city.leaf']` in `sysworld-surfaces.js`).
- **Harbour test.** Tolerances are at float32 precision (1e-3) for instance matrices (`scripts/test-system-world-harbour.mjs`).
- **Survey harness.** It applies the near-gate from its own camera (`syncNear` in `scripts/system-world-survey.mjs`). The thirteen life scenes were added before Task 11 (`scripts/system-world-survey.scenes.json`).
- **Commits.** Tasks 6, 7 and 9 landed in one commit (e5d1a5168).
- **Registration.** `life_assets3.py` is in `build_world3.py` `FAMILIES`; kit version 3.3.0; the manifest holds 46 designs and 138 GLBs. The AGENTS and README notes that said it was not registered are corrected.
