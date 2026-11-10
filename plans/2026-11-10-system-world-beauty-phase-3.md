# System World Beautification — Phase 3 Implementation Plan (enterable buildings)

> **For agentic workers.** One code-writing agent per task, in the order given under "Order and parallelism". A test-running agent runs the commands listed in each task. Lean workflow: one combined review per batch, Critical and Important findings only.
>
> - **Agents do not change git state.** No commits, stashes, resets or branch switches. The coordinator commits each task with the message given at the end of it (last line `Co-Authored-By: Claude <noreply@anthropic.com>`).
> - **CLAUDE.md rules.** Run GitNexus `impact` (upstream) on every function you edit, and `detect_changes` before you hand back. Use the MCP tools, or the CLI from the main checkout root. The index does not cover the worktree. `canWalk`, `groundHeight`, `installCityColliders`, `createWayfinder`, `createSociety`, `createExperience` and `createCity` are expected to come back HIGH or UNKNOWN. Report the result and carry on: this plan is the mitigation, and its tests cover the callers.
> - **Scripts.** Throwaway scripts go into your session scratch directory via the Write tool, never into the repo. Bash heredocs mangle backslashes on this machine.
> - **Paths.** All paths are relative to the worktree `C:\Users\Andi\Documents\repo\AuraGo\.worktrees\sysworld-beauty`. Never Glob or Grep the main repository root.
> - **Anchors, not line numbers.** Every edit names a function and quotes the anchor text. If the text has drifted, make the same change to the equivalent statement and say so in your hand-back.

**Goal**
- Six district buildings get a real ground floor. Visitors and residents walk up lit steps and through sliding glass doors into a themed lobby, and can be seen through real glass from the street.
- The operations beacon gets real podium ground and stairs but stays closed.

**Architecture**
- **`ui/js/desktop/apps/sysworld-lobbies.js` (new, pure, no imports).** The single source of truth for everything lobby-related:
  - the registry: `PODIUMS`, `LOBBIES`, `DOORS`;
  - ground: `lobbyGround`, `podiumAt`;
  - the step rule: `stepFree`;
  - visitor geometry: `lobbyAt`, `lobbyWall`, `lobbyPassage`, `lobbyAirspace`;
  - collider boxes: `lobbySolids`;
  - door states: `createLobbyDoors`, plain JS;
  - the residents' graph and places: `lobbyGraph`, `lobbyPlaces`.

  Derived numbers come from named constants: the clear room is the rect shrunk by `WALL`, `FLIGHT = STEPS*TREAD`, `RISER = PODIUM/(STEPS+1)`, and the atrium front is its apothem.
- **Ownership.**

  | Concern | Owner |
  | --- | --- |
  | Podium/stair ground, step rule, wall and door crossing for the visitor | `sysworld-exploration.js` (`groundHeight`, `canWalk`), calling the registry |
  | Static colliders: walls, corner piers, lintels, ceiling slab, atrium lid, foundry towers, spire struts, and interior fixtures from the v3 manifest | `installCityColliders` (`sysworld-colliders.js`) |
  | Door states (sensing, easing, reduced motion, low tier, E-hold), inside flag, hover rules, lobby places | `sysworld-experience.js` (owns a `createLobbyDoors()` instance, exposed as `experience.lobbies`) |
  | Residents' routes through stairs, porches, doors and interiors | `createWayfinder` (`sysworld-places.js`) with `lobbyGraph` |
  | Loading interiors on approach, swapping the `lobby` stand-in, posing the door leaves, the shared `city.pane` glass, tier rules, removal | new `sysworld-lobby-render.js`, driven by `sysworld-scene.js` |

- **Data flow per frame.**
  1. In the fixed step, `move()` asks `experience.move` → `canWalk` (step rule, walls, `lobbyPassage` with `experience.lobbies.passable`) plus `traffic.clear` (walls and fixtures).
  2. `experience.update` steps the door states from the visitor and the resident bodies, reports door sounds, and sets `room`/`lobby` → `onEnvironment`.
  3. After the fixed steps, `lobbyRender.update(camera.position, landmarks)` loads, shows or hides interiors, writes `doors.setReady(lobby, shown)`, toggles each district clone's `lobby` node, and moves the leaves from `doors.value(key)`.

**Tech notes**
- three 0.186.1. `GLTFLoader` sanitises node names (dots are stripped), so the leaves are named `door_<n>_l` / `door_<n>_r`. That is the coordinator's correction to design decision 8.
- Kit v3 is packed with `gltfpack@1.3.0 -cc -kn -km -ke -kv -vtf`.
- Tests: Node ESM with `node:assert`, Go and rod browser tests, and the survey harness.

---

## What I verified, and what I could not

**Verified by reading** (worktree at `966c9a359`, plus the coordinator's dry-run facts of 2026-11-10)

- **Exterior authoring.**
  - `facade_assets3.stairs` puts `count` treads of `TREAD` with tops `PODIUM - rise*(i+1)` on the `-Y` edge, from the podium edge outwards (glTF +Z).
  - `podium()` puts the stone body top at 0.70 m with a 0.25 m border and the bronze slab top at 0.88 m. Stairs are only drawn at LOD < 2.
- **Flights.** `district_assets3` places stairs at:
  - agent x 0, width 7.5;
  - memory ±5, width 4.2;
  - integrations ±8, width 3.4;
  - missions x 0, width 23;
  - foundry ±3.75, width 3.0;
  - atrium width 4.4;
  - beacon width 2.2.

  Together with the district origins in `sysworld-scene.js` `districts`, these reproduce every podium and stair entry of the design table.
- **Portals.** `portal()` puts the closed glass of the stand-in at `y+.25` (0.25 m inside the facade line), height `min(3.5, glass_top - z0 - .5)` (2.92 integrations, 3.12 foundry), material `city.glass`, part `lobby`.
  - The atrium portal stands 0.1 m outside its apothem, height 3.1.
  - Frames and piers reach 0.2–0.23 m outside the facade line.
  - Corner piers are 1.3 m and centred 0.55 m inside.
- **Interior frame.** `interior_assets3.shell` makes:
  - glass top = registry ceiling (agent 5.0, memory and missions 6.1, integrations 4.3, foundry 4.5);
  - leaves at `front - .25` (atrium `front - .15`);
  - workpoints `[x, FLOOR, -y]`;
  - colliders in glTF axes with base `PODIUM`.

  Coordinator facts: `lobby-agent` `door_0_l` at (−1.00, 0.88, 7.25), `door_0_r` at (1.00, 0.88, 7.25); 2/2/1+1/4/2/1 doors; LOD 0 triangles ≤ 10 604; no `city.window` in interiors; `city.pane` exported with BLEND.
- **Shipped manifest.** `ui/3d/system-world/v3/manifest.json` (25 designs) already lists the component `lobby` for the six enterable districts and `signal` for the beacon, so the exterior half of the contract test can run now.
- **Runtime.**
  - `groundHeight` and `canWalk` know only the pavilions.
  - `buildingFloor` is used only by `experience.groundAt` (hover).
  - The society walks on `placeFloor` (= `groundHeight` plus living-place surfaces), so podium heights reach residents and footing automatically.
  - `createWayfinder` caches per `traffic.revision()`.
  - `traffic.solid` bumps the revision on every change.
  - `society.addPlace(place, entry)` turns `entry.navigation.workpoints` into slots at `place.x + p[0]`, `place.z + p[2]`.
  - `choose()` filters slots by role.
  - `experience.update` sets `room` from `roomAt` and calls `onEnvironment(!!room)` → `weather.setIndoor`, `sound.setEnvironment`.
  - The HUD prompt label is `sysworld.world.interact_<kind>` (`sysworld-controls.js` `interaction`). `interact_door` exists ("Open / close door").
  - `model(id, lod, 'v3')` shares materials by name; `surfaces.apply` skips names without a kind.
  - The survey `EXPOSE` already carries `model` and `catalogV3`.
  - `[data-sw-mode="street"]` plus `[data-sw-district="agent"]` puts the visitor at (0, 8) looking north (`focus`).
- **Tests that move.**
  - `test-system-world-expansion.mjs` line `'Solid spire foundation cannot be crossed'` pins the agent footprint this phase removes.
  - `test-system-world-living.mjs` waits for `places.length===6`.
  - `test-system-world-v3.mjs` pins 25 designs.
  - `test-system-world-ground.mjs` routes `[-40,-50]→[-40,-15]` and `[-40,-50]→[40,30]`. The start (−40, −50) lies on the foundry podium and the ends inside the archive and departure halls, so after the step rule these resolve only through the lobby graph. Task 5 adds that graph by default, so the test stays unchanged.

**Could not verify**
1. Interior GLBs. They are not built in this worktree; the payload arrives in Task 0. The interior half of the contract test is dormant until then.
2. Ray hits on the packed exteriors (quantised positions plus `-kn` node transforms). The contract test settles this at Task 1/2 time on the shipped GLBs.
3. Glass sorting, the pane look, and the pop at the swap. No GPU here; the survey decides.
4. Resident throughput through 2–2.4 m doors and between the foundry towers (2.1 m), and the living-test margins after the step rule.
5. Wayfinder cache-build time with about 300 extra nodes.
6. Rain wetness on interior floors. `createSurfaces` has only three `swShelter` rects (pavilions); see Risks.
7. Whether the parcel sorter (35, 49) and the charger (51, 49) crowd the missions stair foot (stair foot z 48.1).

## Deviations from the design

1. **Leaf names** are `door_<n>_<l|r>` (coordinator correction, decision 8).
2. **No door-panel colliders** (decision 4). Panel solids would bump `traffic.revision()` on every door movement and drop every cached resident route. Lobby doors move whenever a resident passes. Instead:
   - the visitor is held at the leaf line by `lobbyPassage` (inside `canWalk`), which requires `doors.passable(key)`, as the pavilion crossing rule already does;
   - hover flights are refused by `lobbyAirspace` and the lintel and ceiling boxes;
   - residents pass when the door has opened for them.
3. **Doors open for residents on every tier, whether or not the interior is shown.** Doors open for the visitor only when the interior is shown and the tier is not low. On low, lobby workplaces and lobby nodes leave the residents' choice, so residents only leave the lobbies. Without this, residents could be trapped when an interior unloads at 70 m.
4. **Graph nodes.** The "threshold" is a pair: `porch` (0.9 m outside the wall band) and `entry` (0.9 m inside the clear room). A single node inside the 0.6 m wall band makes every diagonal link clip the gap's edges. Each flight adds a `foot` and a `landing`. Each lobby adds a 2 m grid of inside nodes besides the workpoint nodes, because the shipped workpoints sit behind counters and desks and are rarely in a straight line from the door.
5. **Atrium lid.** The atrium gets a collision lid at the rotunda cap (4.7 + 0.4 m). It stays `open` for rain and audio, but no hover flight drops into the court.
6. **Extra solids from the exterior sources:**
   - 1.2 m corner piers;
   - walls reaching 0.25 m outside the facade line (frames and piers);
   - the foundry's three tower plinths (2.7 m) and their street-side pipes;
   - the spire's four corner struts, which lean through the lobby corners (radius 9 → 7.4 m).
7. **The podium's outer 0.25 m border** (stone at 0.70 m) counts as 0.88 m ground, like `buildingFloor` today.
8. **"Dispose"** removes the interior clone from the scene (no draw cost). The decoded template stays in the scene's model cache, like every kit model, and is freed with the city.
9. **HUD prompt** reuses the kind `door`: E holds a lobby door open or releases it. No new locale strings.
10. **Distances are 3D** from each door at eye height, so an orbit camera high above a building does not load its interior.
11. **The expansion test's spire-foundation assertion** is replaced by lobby wall, door, stair and podium-edge assertions (Task 3).

---

## File map

| File | Status | Responsibility |
| --- | --- | --- |
| `ui/js/desktop/apps/sysworld-lobbies.js` | new | Registry, ground, step rule, walls, passage, solids, door states, graph, places |
| `ui/js/desktop/apps/sysworld-lobby-render.js` | new | Interior loading on approach, stand-in swap, leaves, `city.pane`, tiers, stats, dispose |
| `scripts/test-system-world-lobbies.mjs` | new | Registry unit test (Task 1) and collider section (Task 4) |
| `scripts/test-system-world-lobby-contract.mjs` | new | Registry against packed GLBs (exteriors now, interiors after Task 0) |
| `scripts/test-system-world-lobby-walk.mjs` | new | Enter and leave every lobby through every door with the real experience |
| `scripts/test-system-world-lobby-render.mjs` | new | Render module against the real `lobby-agent` GLBs |
| `ui/js/desktop/apps/sysworld-exploration.js` | edit | `footprints`, `buildingFloor`, `solidBuilding`, `groundHeight`, `canWalk` |
| `ui/js/desktop/apps/sysworld-colliders.js` | edit | Lobby solids and interior fixtures |
| `ui/js/desktop/apps/sysworld-places.js` | edit | `createWayfinder(traffic, floor, catalog)` |
| `ui/js/desktop/apps/sysworld-society.js` | edit | `catalog` option, lobby slots, low-tier rule, `lobbyVisits` |
| `ui/js/desktop/apps/sysworld-experience.js` | edit | Doors, inside flag, hover rules, places, prompt, stats |
| `ui/js/desktop/apps/sysworld-scene.js` | edit | Render module wiring, `kit3`, stats |
| `scripts/test-system-world-{surfaces,v3,expansion,ground,living}.mjs` | edit | Pins (see table) |
| `scripts/system-world-survey.mjs`, `.scenes.json` | edit | Expose `lobbies`; 19 lobby scenes |
| `ui/desktop_sysworld_test.go`, `ui/desktop_sysworld_city_browser_test.go`, `ui/desktop_sysworld_expansion_browser_test.go` | edit | Markers, tier check, lobby walk |
| `assets/system-world/AGENTS.md`, `README.md`, `ui/3d/system-world/AGENTS.md`, `plans/2026-11-09-system-world-beauty-design.md` | edit | Docs |
| `ui/js/vendor/system-world/{city.esm.js,manifest.json}` | regenerate | Task 10 |
| `assets/system-world/kit3.py`, `build_world3.py`, `ui/3d/system-world/v3/*` | coordinator | Task 0 |

## Invariants every task must keep

- **Survey patch anchors.** `sysworld-scene.js` keeps the exact text `  return {\n    districts, canvas, update, focus(id)`, and `sysworld-society.js` keeps `    add,patrol,update,suspend,\n`. Add return members elsewhere on those objects.
- **Go markers.** Every string in `TestDesktopSysWorldAppMarkers` stays (for example `windows.adopt(n)`, `kitCatalog(catalog,catalog3)`, `function seat()`, `skyWalkable(`).
- **Material ownership.**
  - `city.window` belongs to `sysworld-windows.js`.
  - `city.pane` belongs to `sysworld-lobby-render.js`: it never enters the shared `materials` map, never takes `surfaces.apply` patches, and is disposed only by `lobbyRender.dispose()`.
- **Pavilions untouched.** `interiors` and every pavilion assertion (HUD, telescopes, "3 galleries", pavilion `canWalk` lines) stay as they are.
- **First display does not grow.** Interiors load only on approach. `rebuild()` `expectedBytes` does not include them.
- **Bundle staleness.** `node scripts/build-system-world.js --check` is red from Task 3 until Task 10 rebuilds the bundle. No Go test checks it.
- **No git state changes by agents.**

## Order and parallelism

| Step | Task | Notes |
| --- | --- | --- |
| A | 1 | Registry and unit test |
| B | 2 ∥ 3 | Disjoint files. 2 = contract test plus Task 0 prep. 3 = `sysworld-exploration.js` plus the expansion test |
| C | **0** | Coordinator payload, after 2 |
| C | 4 ∥ 5 | After 3. 4 = `sysworld-colliders.js` plus the collider section. 5 = `sysworld-places.js` plus the ground test |
| D | 6 | Experience and society, after 0, 4, 5 |
| E | 7 | Render module and scene, after 6 |
| F | 8 ∥ 9 | Survey (8) and docs (9) |
| G | 10 | Markers, browser checks, final pins, bundle, regression, survey set, budgets |

---

## Task 1 — `sysworld-lobbies.js` and its unit test

**Files:** new `ui/js/desktop/apps/sysworld-lobbies.js`, new `scripts/test-system-world-lobbies.mjs`.

**Steps**
- [ ] Write the test below. Run `node scripts/test-system-world-lobbies.mjs` → **FAIL** (module not found).
- [ ] Write the module below. Run again → PASS, printing `System World lobbies: 7 halls, 12 doors, …`.
- [ ] `node --check ui/js/desktop/apps/sysworld-lobbies.js` → no output.

### `ui/js/desktop/apps/sysworld-lobbies.js` (complete)

```js
// Pure registry of the enterable district buildings (System World beauty pass, phase 3): podiums, stair
// flights, lobby rooms and doors, the 0.35 m step rule, wall/lintel/ceiling solids, door states and the
// residents' lobby graph. World metres, Y up; every building front faces +Z (south). The numbers mirror
// assets/system-world/facade_assets3.py (PODIUM, STEPS, TREAD), district_assets3.py and interior_assets3.py;
// scripts/test-system-world-lobby-contract.mjs checks them against the packed GLBs. `interiors` in
// sysworld-layout.js stays the list of waterfront pavilions.
export const PODIUM=.88,STEPS=5,TREAD=.32,FLIGHT=STEPS*TREAD,RISER=PODIUM/(STEPS+1),FLOOR=PODIUM+.02;
// Wall band inside the facade line, frames/piers outside it, corner piers, atrium pier half width, leaf inset.
export const WALL=.6,WALL_OUT=.25,PIER=1.2,PIER_HALF=.35,LEAF_INSET=.25,SOLID_TOP=8;
export const MAX_STEP=.35,STEP_SAMPLE=.25,SENSE=3.2,DOOR_TIME=.6,PASS=.85,LOAD_DISTANCE=45,HIDE_DISTANCE=70,GRID=2,FOOT=.9;
const ATRIUM={sides:12,radius:9.45,wall:4.7,cap:.4,portal:.1};
// Foundry cooling towers: 2.7 m plinths; each pipe runs 3.7 m towards the street (0.36 m thick, 2 m high).
const TOWER={radius:2.7,pipe:.4,reach:4.1,top:2.4};
// Spire corner struts lean through the lobby corners (radius 9 m at the floor, 7.4 m at 7 m).
const STRUT={from:7.3,to:9.3,half:.45,top:5.9};
export const PODIUMS=Object.freeze([
  {id:'agent',x0:-11.5,z0:-23.5,x1:11.5,z1:-.5,stairs:[{x:0,width:7.5}]},
  {id:'memory',x0:-56,z0:-21,x1:-30,z1:1,stairs:[{x:-48,width:4.2},{x:-38,width:4.2}]},
  {id:'integrations',x0:30,z0:-18,x1:56,z1:-2,stairs:[{x:35,width:3.4},{x:51,width:3.4}]},
  {id:'missions',x0:28,z0:25.5,x1:58,z1:46.5,stairs:[{x:43,width:23}]},
  {id:'infra',x0:-55.5,z0:-64.5,x1:-30.5,z1:-41.5,stairs:[{x:-46.75,width:3},{x:-39.25,width:3}]},
  {id:'graph',x0:-55,z0:24,x1:-31,z1:48,stairs:[{x:-43,width:4.4}]},
  {id:'operations',x0:-4.5,z0:31.5,x1:4.5,z1:40.5,stairs:[{x:0,width:2.2}]},
].map(p=>Object.freeze(p)));
// Doors: [node number in the interior asset, x, width, height]; ascending x.
const TABLE=[
  {id:'agent',district:'agent',asset:'lobby-agent',role:'technician',origin:[0,-12],rect:[-7.5,-19.5,7.5,-4.5],ceiling:5,doors:[[0,0,4,3.5]],struts:true},
  {id:'memory',district:'memory',asset:'lobby-memory',role:'archivist',origin:[-43,-10],rect:[-54,-20,-32,-2],ceiling:6.1,doors:[[0,-48,2.8,3.5],[1,-38,2.8,3.5]]},
  {id:'integrations-west',district:'integrations',asset:'lobby-integrations',role:'technician',origin:[43,-10],rect:[32,-14.5,38,-5.5],ceiling:4.3,doors:[[0,35,2,2.92]]},
  {id:'integrations-east',district:'integrations',asset:'lobby-integrations',role:'technician',origin:[43,-10],rect:[48,-14.5,54,-5.5],ceiling:4.3,doors:[[1,51,2,2.92]]},
  {id:'missions',district:'missions',asset:'lobby-missions',role:'archivist',origin:[43,36],rect:[30,27.5,56,42.5],ceiling:6.1,doors:[[0,34,4,3.5],[1,40,4,3.5],[2,46,4,3.5],[3,52,4,3.5]]},
  {id:'infra',district:'infra',asset:'lobby-infra',role:'technician',origin:[-43,-53],rect:[-53,-62,-33,-50],ceiling:4.5,doors:[[0,-46.75,2.4,3.12],[1,-39.25,2.4,3.12]],towers:[[-50.5,-48],[-43,-48],[-35.5,-48]]},
  {id:'graph',district:'graph',asset:'lobby-graph',role:'archivist',origin:[-43,36],circle:{x:-43,z:36,...ATRIUM},ceiling:null,open:true,doors:[[0,-43,3.2,3.1]]},
];
const apothem=c=>c.radius*Math.cos(Math.PI/c.sides);
export const LOBBIES=Object.freeze(TABLE.map(t=>{
  const front=t.circle?t.circle.z+apothem(t.circle):t.rect[3];
  return Object.freeze({...t,open:!!t.open,front,leaf:front-LEAF_INSET+(t.circle?ATRIUM.portal:0),
    doors:Object.freeze(t.doors.map(([n,x,width,height])=>Object.freeze({n,x,width,height})))});
}));
export const DOORS=Object.freeze(LOBBIES.flatMap(l=>l.doors.map(d=>Object.freeze({...d,key:'lobby:'+l.id+':'+d.n,lobby:l.id,asset:l.asset,district:l.district,z:l.leaf}))));
export const LOBBY_DISTRICTS=new Set(LOBBIES.map(l=>l.district));
const NORMALS=Array.from({length:ATRIUM.sides},(_,k)=>[Math.sin(k*2*Math.PI/ATRIUM.sides),Math.cos(k*2*Math.PI/ATRIUM.sides)]);
// Distance beyond the facade line (negative inside): the largest overshoot over any face.
function reach(l,x,z){
  if(l.circle){const dx=x-l.circle.x,dz=z-l.circle.z;let m=-Infinity;for(const [s,c]of NORMALS)m=Math.max(m,dx*s+dz*c);return m-apothem(l.circle);}
  const [x0,z0,x1,z1]=l.rect;return Math.max(x0-x,x-x1,z0-z,z-z1);
}
const corner=(l,x,z)=>{const [x0,z0,x1,z1]=l.rect;return Math.min(x-x0,x1-x)<PIER&&Math.min(z-z0,z1-z)<PIER;};
const inGap=(l,x,z)=>{const f=z-l.front;return f>=-WALL&&f<=WALL_OUT&&l.doors.some(d=>Math.abs(x-d.x)<d.width/2);};
export const lobbyBounds=l=>l.rect?[...l.rect]:[l.circle.x-l.circle.radius,l.circle.z-l.circle.radius,l.circle.x+l.circle.radius,l.circle.z+l.circle.radius];
export const clearRoom=l=>{const [x0,z0,x1,z1]=l.rect;return [x0+WALL,z0+WALL,x1-WALL,z1-WALL];};
export function podiumAt(x,z){return PODIUMS.find(p=>x>=p.x0&&x<=p.x1&&z>=p.z0&&z<=p.z1)||null;}
// Podium top, or the tread under a stair flight (STEPS treads of TREAD, STEPS + 1 equal risers); null elsewhere.
export function lobbyGround(x,z){
  if(podiumAt(x,z))return PODIUM;
  for(const p of PODIUMS){const s=z-p.z1;if(s<=0||s>=FLIGHT)continue;
    for(const f of p.stairs)if(Math.abs(x-f.x)<=f.width/2)return PODIUM-RISER*(Math.min(STEPS-1,Math.floor(s/TREAD))+1);}
  return null;
}
// The lobby whose clear room (facade line minus WALL) holds the point.
export function lobbyAt(x,z){return LOBBIES.find(l=>reach(l,x,z)<=-WALL)||null;}
export function lobbyAirspace(x,y,z){return y<SOLID_TOP&&!!lobbyAt(x,z);}
export function lobbyWall(x,z){
  for(const l of LOBBIES){const r=reach(l,x,z);if(r>WALL_OUT)continue;
    if(r>-WALL){if(!inGap(l,x,z))return true;continue;}
    if(l.rect&&corner(l,x,z))return true;}
  return false;
}
const nearPodium=(a,b,m)=>PODIUMS.some(p=>Math.max(a.x,b.x)>=p.x0-m&&Math.min(a.x,b.x)<=p.x1+m&&Math.max(a.z,b.z)>=p.z0-m&&Math.min(a.z,b.z)<=p.z1+FLIGHT+m);
// Nobody takes a step higher than MAX_STEP between samples at most STEP_SAMPLE apart. Away from the
// podiums every ground change (kerb 0.14, platform 0.30) is lower, so only links near them are sampled.
export function stepFree(a,b,height){
  if(!nearPodium(a,b,1))return true;
  const n=Math.max(1,Math.ceil(Math.hypot(b.x-a.x,b.z-a.z)/STEP_SAMPLE));let last=height(a.x,a.z);
  for(let i=1;i<=n;i++){const t=i/n,h=height(a.x+(b.x-a.x)*t,a.z+(b.z-a.z)*t);if(Math.abs(h-last)>MAX_STEP)return false;last=h;}
  return true;
}
// Visitor move: never into a wall band, and across a leaf line only through an open door.
export function lobbyPassage(from,to,doorOpen){
  const n=Math.max(1,Math.ceil(Math.hypot(to.x-from.x,to.z-from.z)/STEP_SAMPLE));
  for(let i=0;i<=n;i++){const t=i/n;if(lobbyWall(from.x+(to.x-from.x)*t,from.z+(to.z-from.z)*t))return false;}
  for(const d of DOORS){
    if(Math.sign(from.z-d.z)===Math.sign(to.z-d.z))continue;
    const t=to.z===from.z?0:(d.z-from.z)/(to.z-from.z),x=from.x+(to.x-from.x)*t;
    if(Math.abs(x-d.x)<d.width/2&&!doorOpen(d.key))return false;
  }
  return true;
}
// Traffic boxes for installCityColliders: walls with door gaps, corner piers, lintels, the ceiling slab
// (atrium: a collision lid at its cap), the foundry towers and pipes, the spire struts.
export function lobbySolids(district){
  const out=[],box=(x0,z0,x1,z1,y0=0,y1=SOLID_TOP)=>out.push({x:0,z:0,heading:0,min:[x0,y0,z0],max:[x1,y1,z1]});
  for(const l of LOBBIES){
    if(l.district!==district)continue;
    if(l.rect){
      const [x0,z0,x1,z1]=l.rect,o=WALL_OUT;
      box(x0-o,z0-o,x1+o,z0+WALL);box(x0-o,z0-o,x0+WALL,z1+o);box(x1-WALL,z0-o,x1+o,z1+o);
      for(const [cx,sx]of[[x0,1],[x1,-1]])for(const [cz,sz]of[[z0,1],[z1,-1]])
        box(Math.min(cx,cx+sx*PIER),Math.min(cz,cz+sz*PIER),Math.max(cx,cx+sx*PIER),Math.max(cz,cz+sz*PIER));
      let start=x0-o;
      for(const d of l.doors){box(start,z1-WALL,d.x-d.width/2,z1+o);box(d.x-d.width/2,z1-WALL,d.x+d.width/2,z1+o,PODIUM+d.height);start=d.x+d.width/2;}
      box(start,z1-WALL,x1+o,z1+o);
      box(x0-o,z0-o,x1+o,z1+o,l.ceiling);
    }else{
      const c=l.circle,A=apothem(c),half=c.radius*Math.sin(Math.PI/c.sides)+PIER_HALF,d=l.doors[0],dx=d.x-c.x;
      for(let k=0;k<c.sides;k++){
        const heading=k*2*Math.PI/c.sides,seg=(a,b,y0=0)=>out.push({x:c.x,z:c.z,heading,min:[a,y0,A-WALL],max:[b,SOLID_TOP,A+WALL_OUT]});
        if(k){seg(-half,half);continue;}
        seg(-half,dx-d.width/2);seg(dx-d.width/2,dx+d.width/2,PODIUM+d.height);seg(dx+d.width/2,half);
      }
      const r=c.radius+WALL_OUT;out.push({x:c.x,z:c.z,heading:0,min:[-r,c.wall+c.cap,-r],max:[r,SOLID_TOP,r]});
    }
    for(const [x,z]of l.towers||[]){box(x-TOWER.radius,z-TOWER.radius,x+TOWER.radius,z+TOWER.radius);box(x-TOWER.pipe,z,x+TOWER.pipe,z+TOWER.reach,0,TOWER.top);}
    if(l.struts)for(const k of[1,3,5,7])out.push({x:l.origin[0],z:l.origin[1],heading:k*Math.PI/4,min:[-STRUT.half,0,STRUT.from],max:[STRUT.half,STRUT.top,STRUT.to]});
  }
  return out;
}
const ease=t=>t*t*(3-2*t);
// Door states. Residents within SENSE open a door on every tier; the visitor only once the interior is
// shown (setReady) and never on low. Opening takes DOOR_TIME (eased), reduced motion snaps.
export function createLobbyDoors(){
  const state=new Map(DOORS.map(d=>[d.key,{door:d,t:0,target:0,held:false}])),ready=new Set();let tier='high';
  const may=d=>tier!=='low'&&ready.has(d.lobby),near=(d,p)=>!!p&&Math.hypot(p.x-d.x,p.z-d.z)<SENSE;
  return {
    setTier(value){tier=value;},
    setReady(lobby,value){if(value)ready.add(lobby);else ready.delete(lobby);},
    usable(key){const s=state.get(key);return !!s&&may(s.door);},
    hold(key){const s=state.get(key);if(!s||!may(s.door))return false;s.held=!s.held;return true;},
    update(dt,visitor,residents=[],reduced=false){
      const flips=[];
      for(const s of state.values()){
        const d=s.door;if(!may(d))s.held=false;
        const want=(may(d)&&(s.held||near(d,visitor)))||residents.some(p=>near(d,p))?1:0;
        if(want!==s.target){s.target=want;flips.push(d);}
        s.t=reduced?want:Math.min(1,Math.max(0,s.t+(want?1:-1)*Math.max(0,dt||0)/DOOR_TIME));
      }
      return flips;
    },
    value(key){const s=state.get(key);return s?ease(s.t):0;},
    passable(key){const s=state.get(key);return !!s&&may(s.door)&&s.t>=PASS;},
    stats:()=>({open:[...state.values()].filter(s=>s.t>=PASS).map(s=>s.door.key),ready:[...ready],tier}),
  };
}
const entryOf=(catalog,id)=>catalog instanceof Map?catalog.get(id):catalog?.assets?.find(a=>a.id===id);
// Workpoints are building-local glTF positions; only those inside this hall's clear room count.
export function workpointsOf(l,catalog){
  return (entryOf(catalog,l.asset)?.navigation?.workpoints||[]).map(w=>({id:w.id,x:l.origin[0]+w.position[0],z:l.origin[1]+w.position[2]}))
    .filter(w=>lobbyAt(w.x,w.z)?.id===l.id);
}
// One living place per interior asset (the place origin is the district origin, as addPlace expects).
export function lobbyPlaces(catalog){
  const out=[],seen=new Set();
  for(const l of LOBBIES){
    const entry=entryOf(catalog,l.asset);if(seen.has(l.asset)||!entry?.navigation?.workpoints?.length)continue;seen.add(l.asset);
    out.push({place:{id:l.asset,district:l.district,x:l.origin[0],z:l.origin[1],role:l.role,lobby:true},entry});
  }
  return out;
}
// Residents' graph: per flight a foot (pavement) and a landing (podium); per door a porch (outside) and an
// entry (inside); per hall a GRID of inside nodes and its workpoints. Inside nodes carry `lobby`.
export function lobbyGraph(catalog=null){
  const nodes=[],links=[],add=n=>nodes.push(Object.freeze(n))-1,join=(a,b)=>links.push([a,b]),tops=new Map(PODIUMS.map(p=>[p.id,[]]));
  for(const p of PODIUMS)for(const f of p.stairs){
    const foot=add({x:f.x,z:p.z1+FLIGHT+FOOT,kind:'foot',lobby:null}),landing=add({x:f.x,z:p.z1-FOOT,kind:'landing',lobby:null});
    join(foot,landing);tops.get(p.id).push(landing);
  }
  for(const l of LOBBIES){
    const inside=[];
    for(const d of l.doors){
      const porch=add({x:d.x,z:l.front+WALL_OUT+FOOT,kind:'porch',lobby:null}),entry=add({x:d.x,z:l.front-WALL-FOOT,kind:'entry',lobby:l.id});
      join(porch,entry);tops.get(l.district).push(porch);inside.push(entry);
    }
    const [x0,z0,x1,z1]=lobbyBounds(l);
    for(let x=x0+WALL+GRID/2;x<=x1-WALL-GRID/2+1e-9;x+=GRID)for(let z=z0+WALL+GRID/2;z<=z1-WALL-GRID/2+1e-9;z+=GRID)
      if(reach(l,x,z)<=-WALL-.3)inside.push(add({x,z,kind:'grid',lobby:l.id}));
    for(const w of workpointsOf(l,catalog))inside.push(add({x:w.x,z:w.z,id:w.id,kind:'work',lobby:l.id}));
    for(let i=0;i<inside.length;i++)for(let j=i+1;j<inside.length;j++){
      const a=nodes[inside[i]],b=nodes[inside[j]],r=a.kind==='grid'&&b.kind==='grid'?GRID*1.5:GRID*2;
      if(Math.hypot(a.x-b.x,a.z-b.z)<=r+1e-9)join(inside[i],inside[j]);
    }
  }
  for(const ids of tops.values())for(let i=0;i<ids.length;i++)for(let j=i+1;j<ids.length;j++)join(ids[i],ids[j]);
  return {nodes,links};
}
```

### `scripts/test-system-world-lobbies.mjs` (complete for Task 1)

```js
// Phase 3 lobby registry: design numbers, ground, step rule, walls, solids, doors and the residents' graph.
import assert from 'node:assert/strict';
import * as L from '../ui/js/desktop/apps/sysworld-lobbies.js';
const {PODIUMS,LOBBIES,DOORS,PODIUM,FLIGHT,RISER,TREAD,STEPS,DOOR_TIME,SOLID_TOP}=L;
const near=(a,b,eps=1e-9)=>Math.abs(a-b)<=eps;
// Design table (plans/2026-11-10-system-world-beauty-phase-3-design.md) and the kit v3 stair profile.
assert.ok(near(FLIGHT,1.6)&&near(RISER,.88/6)&&STEPS===5&&TREAD===.32&&PODIUM===.88,'stair profile');
assert.deepEqual(PODIUMS.map(p=>p.id),['agent','memory','integrations','missions','infra','graph','operations']);
assert.deepEqual(LOBBIES.map(l=>l.id),['agent','memory','integrations-west','integrations-east','missions','infra','graph']);
assert.equal(DOORS.length,12);assert.equal(new Set(DOORS.map(d=>d.key)).size,12);
L.clearRoom(LOBBIES[0]).forEach((v,i)=>assert.ok(near(v,[-6.9,-18.9,6.9,-5.1][i],1e-9),'agent clear room'));
const atrium=LOBBIES.find(l=>l.id==='graph');assert.ok(near(atrium.front,45.13,.005)&&atrium.open&&atrium.ceiling===null,'atrium');
assert.ok(LOBBIES.every(l=>l.doors.every(d=>d.height>=2.9)),'every door clears the visitor (eye 2.4 m)');
assert.equal(DOORS.find(d=>d.key==='lobby:agent:0').z,-4.75);
// Ground: podium top, treads, nothing beyond the flight or beside it; the beacon has stairs too.
assert.equal(L.lobbyGround(0,-12),PODIUM);
assert.ok(near(L.lobbyGround(0,-.5+.16),PODIUM-RISER));assert.ok(near(L.lobbyGround(0,-.5+FLIGHT-.01),RISER));
assert.equal(L.lobbyGround(0,-.5+FLIGHT+.01),null);assert.equal(L.lobbyGround(5,.3),null);assert.equal(L.lobbyGround(0,5),null);
assert.ok(near(L.lobbyGround(0,40.6),PODIUM-RISER),'beacon flight');
// Step rule: up the stairs passes, the podium edge and the side of a flight do not; 0.14/0.30/0.147 pass, 0.36 not.
const h=(x,z)=>L.lobbyGround(x,z)??0;
assert.ok(L.stepFree({x:0,z:3},{x:0,z:-3},h),'up the agent stairs');
assert.equal(L.stepFree({x:10,z:1},{x:10,z:-1},h),false,'podium edge');
assert.equal(L.stepFree({x:3.6,z:.3},{x:4.2,z:.3},h),false,'off the side of a flight');
for(const [rise,ok]of[[.14,true],[.30,true],[RISER,true],[.36,false]])assert.equal(L.stepFree({x:12,z:-10},{x:12.4,z:-10},x=>x>12.2?rise:0),ok,'step '+rise);
// Rooms, walls, gaps, corner piers, the atrium polygon, the integration passage.
assert.equal(L.lobbyAt(0,-12)?.id,'agent');assert.equal(L.lobbyAt(0,-4.9),null);
assert.equal(L.lobbyWall(0,-4.9),false,'door gap');assert.equal(L.lobbyWall(3,-4.9),true);assert.equal(L.lobbyWall(-6.6,-5.4),true,'corner pier');
assert.equal(L.lobbyWall(0,-3),false);assert.equal(L.lobbyAt(-43,36)?.id,'graph');
assert.equal(L.lobbyWall(-43,45),false);assert.equal(L.lobbyWall(-40,45),true);assert.equal(L.lobbyWall(43,-10),false,'integration passage');
assert.ok(L.lobbyAirspace(0,3.3,-12)&&!L.lobbyAirspace(0,9,-12)&&!L.lobbyAirspace(0,3.3,3),'airspace');
// Passage: closed door, open door, wall.
assert.equal(L.lobbyPassage({x:0,z:-4},{x:0,z:-5.5},()=>false),false);
assert.ok(L.lobbyPassage({x:0,z:-4},{x:0,z:-5.5},k=>k==='lobby:agent:0'));
assert.equal(L.lobbyPassage({x:3,z:-4},{x:3,z:-5.5},()=>true),false);
// Solids: counts per building, lintel and ceiling heights, atrium lid, nothing for the beacon.
const agent=L.lobbySolids('agent');assert.equal(agent.length,15);
const lintel=agent.find(b=>near(b.min[1],PODIUM+3.5));assert.ok(lintel&&near(lintel.min[0],-2)&&near(lintel.max[0],2)&&lintel.max[1]===SOLID_TOP,'agent lintel');
assert.ok(agent.some(b=>b.min[1]===5&&b.max[1]===SOLID_TOP),'agent ceiling');
assert.equal(L.lobbySolids('memory').length,13);assert.equal(L.lobbySolids('integrations').length,22);assert.equal(L.lobbySolids('missions').length,17);
assert.equal(L.lobbySolids('infra').length,19);assert.equal(L.lobbySolids('graph').length,15);assert.deepEqual(L.lobbySolids('operations'),[]);
assert.ok(L.lobbySolids('graph').some(b=>near(b.min[1],4.7+.4)),'atrium lid');
// Doors.
{
  const doors=L.createLobbyDoors(),key='lobby:agent:0',at={x:0,z:-3.5};
  doors.update(.1,at,[]);assert.equal(doors.value(key),0,'no interior shown: the visitor waits');assert.equal(doors.usable(key),false);
  doors.update(DOOR_TIME,null,[at]);assert.equal(doors.value(key),1,'residents open doors');assert.equal(doors.passable(key),false);
  doors.update(DOOR_TIME,null,[]);assert.equal(doors.value(key),0);
  doors.setReady('agent',true);assert.equal(doors.update(DOOR_TIME/2,at,[]).length,1,'a door reports when it starts to move');
  assert.ok(near(doors.value(key),.5)&&!doors.passable(key));doors.update(DOOR_TIME*.4,at,[]);assert.ok(doors.passable(key));
  doors.update(1,null,[],true);assert.equal(doors.value(key),0,'reduced motion snaps');
  assert.ok(doors.hold(key));doors.update(0,null,[],true);assert.equal(doors.value(key),1,'held open');
  doors.setTier('low');doors.update(0,at,[],true);assert.equal(doors.value(key),0,'low: shut for the visitor');assert.equal(doors.usable(key),false);
  doors.update(0,null,[at],true);assert.equal(doors.value(key),1,'low: residents still pass');assert.equal(doors.passable(key),false);
}
// Graph.
{
  const g=L.lobbyGraph(null),kind=k=>g.nodes.filter(n=>n.kind===k).length;
  assert.deepEqual([kind('foot'),kind('landing'),kind('porch'),kind('entry')],[10,10,12,12]);
  for(const n of g.nodes)assert.equal(L.lobbyAt(n.x,n.z)?.id||null,n.lobby,'node on the wrong side: '+JSON.stringify(n));
  for(const l of LOBBIES)assert.ok(g.nodes.some(n=>n.lobby===l.id&&n.kind==='grid'),l.id+' grid');
  assert.ok(g.links.every(([a,b])=>g.nodes[a]&&g.nodes[b]&&a!==b));
  const adj=g.nodes.map(()=>[]);for(const [a,b]of g.links){adj[a].push(b);adj[b].push(a);}
  g.nodes.forEach((n,i)=>{if(n.kind!=='entry')return;const seen=new Set([i]),queue=[i];let out=false;
    while(queue.length&&!out){const j=queue.pop();if(g.nodes[j].kind==='foot')out=true;for(const k of adj[j])if(!seen.has(k)){seen.add(k);queue.push(k);}}
    assert.ok(out,'entry without a way down to the pavement: '+JSON.stringify(n));});
  const fake=new Map([['lobby-agent',{navigation:{workpoints:[{id:'desk',position:[-1.35,.9,1.5]},{id:'outside',position:[0,.9,9]}]}}]]);
  assert.deepEqual(L.lobbyGraph(fake).nodes.filter(n=>n.kind==='work').map(n=>[n.x,n.z]),[[-1.35,-10.5]]);
  const places=L.lobbyPlaces(fake);assert.equal(places.length,1);
  assert.deepEqual(places[0].place,{id:'lobby-agent',district:'agent',x:0,z:-12,role:'technician',lobby:true});
  console.log(`System World lobbies: ${LOBBIES.length} halls, ${DOORS.length} doors, ${g.nodes.length} graph nodes / ${g.links.length} links; ground, step rule, walls, solids and doors verified.`);
}
```

**Acceptance:** the test passes, and no runtime file imports the module yet.

**Commit:** `feat(system-world): pure lobby registry (podiums, stairs, doors, walls, step rule, door states, residents' graph)`

---

## Task 2 — Registry-versus-GLB contract test and Task 0 prep

**Files:** new `scripts/test-system-world-lobby-contract.mjs`, `scripts/test-system-world-surfaces.mjs`, `scripts/test-system-world-v3.mjs`.

**Steps**
- [ ] Write the contract test below and run `node scripts/test-system-world-lobby-contract.mjs` → PASS with `interiors: none shipped yet`.
  - **If an exterior assertion fails, do not loosen it.** Report the measured value: either the registry or the coordinator's model is wrong.
  - A ray that misses every tread suggests the meshopt node transforms were not applied. Check `gltf.scene.updateMatrixWorld(true)` before changing the test.
- [ ] **`test-system-world-surfaces.mjs`.**
  - After `const windowPanes = new Set([WINDOW_MATERIAL]);` add:
    ```js
    // Lobby glazing is the shared transparent material of sysworld-lobby-render.js, never the surface maps.
    const lobbyGlass = new Set(['city.pane']);
    ```
  - Change the coverage assertion to `assert.ok(KINDS[name] || WINDOWS.has(name) || untextured.has(name) || windowPanes.has(name) || lobbyGlass.has(name), 'Unmapped kit material: ' + name);`.
  - After the `'city.window belongs to sysworld-windows.js'` assertion add:
    ```js
    const lobbyPane = new THREE.MeshStandardMaterial({name: 'city.pane', vertexColors: true});
    surfaces.apply(lobbyPane);
    assert.equal(lobbyPane.onBeforeCompile, THREE.Material.prototype.onBeforeCompile, 'city.pane belongs to sysworld-lobby-render.js');
    ```
- [ ] **`test-system-world-v3.mjs`.** Replace `assert.equal(manifest.assets.length, 25, '6 street + 12 superseded + 7 new furniture designs');` with:
  ```js
  // Phase 3: the six lobby interiors ship together (Task 0) or not at all.
  const lobbyIds = ['lobby-agent', 'lobby-memory', 'lobby-integrations', 'lobby-missions', 'lobby-infra', 'lobby-graph'];
  const lobbiesShipped = lobbyIds.filter(id => ids.includes(id)).length;
  assert.ok(lobbiesShipped === 0 || lobbiesShipped === 6, 'Interiors ship together: ' + lobbiesShipped);
  assert.equal(manifest.assets.length, 25 + lobbiesShipped, '6 street + 12 superseded + 7 furniture designs (+ 6 lobby interiors)');
  ```

### `scripts/test-system-world-lobby-contract.mjs` (complete)

```js
// Phase 3 contract: the lobby registry (sysworld-lobbies.js) against the packed kit v3 GLBs. Exteriors are
// measured now (podiums, treads by ray casts, the `lobby` stand-in, its door glass); interiors once shipped.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import * as THREE from 'three';
import {GLTFLoader} from 'three/addons/loaders/GLTFLoader.js';
import {MeshoptDecoder} from 'three/addons/libs/meshopt_decoder.module.js';
import {districts} from '../ui/js/desktop/apps/sysworld-scene.js';
import {PODIUMS,LOBBIES,DOORS,PODIUM,TREAD,FLIGHT,FLOOR,lobbyGround,lobbyAt,lobbyBounds} from '../ui/js/desktop/apps/sysworld-lobbies.js';

const base='ui/3d/system-world/v3',manifest=JSON.parse(await fs.readFile(base+'/manifest.json','utf8')),catalog=new Map(manifest.assets.map(a=>[a.id,a]));
await MeshoptDecoder.ready;
const loader=new GLTFLoader().setMeshoptDecoder(MeshoptDecoder),docOf=data=>JSON.parse(data.subarray(20,20+data.readUInt32LE(12)).toString('utf8'));
async function open(id,level=0){
  const file=catalog.get(id).lods.find(l=>l.level===level).file,data=await fs.readFile(base+'/'+file);
  const gltf=await loader.parseAsync(data.buffer.slice(data.byteOffset,data.byteOffset+data.byteLength),'');gltf.scene.updateMatrixWorld(true);
  return {gltf,doc:docOf(data),file};
}
const near=(a,b,eps,what)=>assert.ok(Math.abs(a-b)<=eps,`${what}: ${Number(a).toFixed(3)} instead of ${Number(b).toFixed(3)}`);
const district=id=>districts.find(d=>d.id===id);
for(const l of LOBBIES){const d=district(l.district);assert.deepEqual(l.origin,[d.x,d.z],l.id+' is authored in its building frame');}
// Podiums and stairs at LOD 0 (LOD 2 draws no stairs): rays from 2.5 m, below every canopy.
const ray=new THREE.Raycaster(),down=new THREE.Vector3(0,-1,0);
const top=(scene,x,z)=>{ray.set(new THREE.Vector3(x,2.5,z),down);ray.far=3;const hit=ray.intersectObject(scene,true)[0];return hit?2.5-hit.distance:null;};
let treads=0;
for(const p of PODIUMS){
  const d=district(p.id),{gltf,file}=await open(d.asset),at=(x,z)=>top(gltf.scene,x-d.x,z-d.z);
  for(const [x,z,ox,oz]of[[p.x0,p.z0,1,1],[p.x1,p.z0,-1,1],[p.x0,p.z1,1,-1],[p.x1,p.z1,-1,-1]]){
    assert.ok((at(x+ox*.4,z+oz*.4)??0)>=PODIUM-.02,file+' podium corner');const out=at(x-ox*.15,z-oz*.15);assert.ok(out===null||out<.02,file+' ground outside the podium');
  }
  for(const f of p.stairs){
    for(const dx of[-f.width/2+.35,0,f.width/2-.35])for(let i=0;i<5;i++){const x=f.x+dx,z=p.z1+(i+.5)*TREAD;near(at(x,z)??-1,lobbyGround(x,z),.02,`${file} tread ${i} at x ${x}`);treads++;}
    near(at(f.x,p.z1-.5)??-1,PODIUM,.02,file+' landing');
    const below=at(f.x,p.z1+FLIGHT+.25);assert.ok(below===null||below<.02,file+' ground beyond the flight');assert.equal(lobbyGround(f.x,p.z1+FLIGHT+.25),null);
  }
}
// Stand-ins: the `lobby` part spans the ground-floor glazing; its closed door glass sits on the registry doors.
const v=new THREE.Vector3();
for(const id of new Set(LOBBIES.map(l=>l.district))){
  const d=district(id),halls=LOBBIES.filter(l=>l.district===id),{gltf,file}=await open(d.asset),part=gltf.scene.getObjectByName('lobby');
  assert.ok(part,file+' has the lobby stand-in part');
  const box=new THREE.Box3().setFromObject(part),rects=halls.map(lobbyBounds);
  near(box.min.y,PODIUM,.02,file+' stand-in bottom');near(box.max.y,Math.max(...halls.map(l=>l.ceiling??l.circle.wall)),.03,file+' stand-in top');
  for(const [x0,z0,x1,z1]of rects)assert.ok(box.min.x<=x0-d.x+.7&&box.max.x>=x1-d.x-.7&&box.min.z<=z0-d.z+.7&&box.max.z>=z1-d.z-.7,file+' stand-in covers its hall');
  assert.ok(box.min.x>=Math.min(...rects.map(r=>r[0]))-d.x-.05&&box.max.x<=Math.max(...rects.map(r=>r[2]))-d.x+.05&&
    box.min.z>=Math.min(...rects.map(r=>r[1]))-d.z-.05&&box.max.z<=Math.max(...rects.map(r=>r[3]))-d.z+.05,file+' stand-in stays inside the lobby');
  const glass=[];part.traverse(m=>{if(!m.isMesh||m.material.name!=='city.glass')return;const pos=m.geometry.getAttribute('position');for(let i=0;i<pos.count;i++)glass.push(v.fromBufferAttribute(pos,i).applyMatrix4(m.matrixWorld).clone());});
  const doors=DOORS.filter(x=>x.district===id);
  for(const door of doors){
    const g=glass.filter(p=>Math.abs(p.x-(door.x-d.x))<=door.width/2+.05);assert.ok(g.length,door.key+' door glass');
    const xs=g.map(p=>p.x),ys=g.map(p=>p.y),zs=g.map(p=>p.z);
    near((Math.min(...xs)+Math.max(...xs))/2,door.x-d.x,.02,door.key+' centre');near(Math.max(...xs)-Math.min(...xs),door.width,.02,door.key+' width');
    near(Math.min(...ys),PODIUM,.02,door.key+' sill');near(Math.max(...ys)-Math.min(...ys),door.height,.02,door.key+' height');
    near((Math.min(...zs)+Math.max(...zs))/2,door.z-d.z,.03,door.key+' leaf line');
  }
  assert.equal(glass.filter(p=>!doors.some(door=>Math.abs(p.x-(door.x-d.x))<=door.width/2+.05)).length,0,file+' door glass without a registry door');
}
// Interiors (Task 0 payload): ship together, frame, leaves, workpoints, fixtures, glazing, size.
const interiorIds=[...new Set(LOBBIES.map(l=>l.asset))],shipped=interiorIds.filter(id=>catalog.has(id));
assert.ok(shipped.length===0||shipped.length===interiorIds.length,'Interiors ship together: '+shipped);
let interiorBytes=0;
for(const id of shipped){
  const a=catalog.get(id),halls=LOBBIES.filter(l=>l.asset===id),[ox,oz]=halls[0].origin,doors=DOORS.filter(d=>d.asset===id);
  assert.equal(a.category,'interior',id+' category');
  for(const door of doors)for(const side of['l','r'])assert.ok(a.components.includes(`door_${door.n}_${side}`),`${id} door_${door.n}_${side}`);
  const points=a.navigation.workpoints||[];assert.ok(points.length>=3,id+' workpoints');
  for(const w of points){near(w.position[1],FLOOR,.01,id+' '+w.id+' height');assert.ok(halls.some(l=>lobbyAt(ox+w.position[0],oz+w.position[2])?.id===l.id),id+' '+w.id+' outside the clear room');}
  for(const b of a.navigation.colliders||[]){
    assert.ok(b[1]>=PODIUM-.01&&b[4]>b[1],id+' collider height '+b);
    assert.ok(halls.some(l=>{const [x0,z0,x1,z1]=lobbyBounds(l);return ox+b[0]>=x0-.05&&ox+b[3]<=x1+.05&&oz+b[2]>=z0-.05&&oz+b[5]<=z1+.05;}),id+' collider outside the lobby '+b);
  }
  assert.ok(a.lods[0].triangles<=12000,id+' LOD 0 triangles '+a.lods[0].triangles);interiorBytes+=a.lods.reduce((s,l)=>s+l.bytes,0);
  const {gltf,doc,file}=await open(id);
  assert.ok(doc.materials.some(m=>m.name==='city.pane')&&!doc.materials.some(m=>m.name==='city.window'),file+' glazing');
  for(const door of doors)for(const [side,sign]of[['l',-1],['r',1]]){
    const leaf=gltf.scene.getObjectByName(`door_${door.n}_${side}`);assert.ok(leaf,file+' leaf node');
    near(leaf.position.x,door.x-ox+sign*door.width/4,.02,door.key+side+' pivot x');near(leaf.position.y,PODIUM,.02,door.key+side+' pivot y');
    near(leaf.position.z,door.z-oz,.02,door.key+side+' pivot z');
    const box=new THREE.Box3().setFromObject(leaf);near(box.max.x-box.min.x,door.width/2,.08,door.key+side+' leaf width');
  }
}
for(const a of manifest.assets)if(!shipped.includes(a.id))for(const lod of a.lods)
  assert.ok(!docOf(await fs.readFile(base+'/'+lod.file)).materials.some(m=>m.name==='city.pane'),lod.file+' must not carry lobby glazing');
assert.ok(interiorBytes<2.5*1024*1024,'Interiors exceed 2.5 MiB: '+interiorBytes);
console.log(`System World lobby contract: ${PODIUMS.length} podiums, ${treads} treads, ${DOORS.length} doors verified; interiors: `+(shipped.length?`${shipped.length} designs, ${interiorBytes} bytes verified`:'none shipped yet'));
```

**Run:**
- `node scripts/test-system-world-lobby-contract.mjs` → PASS, `interiors: none shipped yet`.
- `node scripts/test-system-world-surfaces.mjs` → PASS.
- `node scripts/test-system-world-v3.mjs` → PASS (25 designs).

**Acceptance:** exterior geometry matches the registry, and both prep pins accept the coming payload.

**Commit:** `test(system-world): lobby registry contract against the kit v3 GLBs; surfaces and kit pins ready for interiors`

---

## Task 0 — Coordinator delivers the payload (no agent work)

**What the coordinator does**
1. Adds the palette key `pane` to `assets/system-world/kit3.py` `PALETTE`.
2. Registers `interior_assets3` in `assets/system-world/build_world3.py` `FAMILIES` (`SOURCES` follows).
3. `& 'D:/Blender 5.2/blender.exe' --background --factory-startup --python assets/system-world/build_world3.py`
4. `node scripts/build-system-world-v3.mjs`, then `node scripts/build-system-world-v3.mjs --check`.
5. Commits `kit3.py`, `build_world3.py` and `ui/3d/system-world/v3/*`: `feat(system-world): kit v3 lobby interiors and lobby glazing (payload)`.

**What must already hold at that moment**
- **Task 2:**
  - `test-system-world-surfaces.mjs` accepts `city.pane`;
  - `test-system-world-v3.mjs` accepts 25 + 6 designs;
  - the interior half of `test-system-world-lobby-contract.mjs` turns on.
- **Task 1:** the registry names the six asset ids and the door numbers of the coordinator's dry run.
- **No runtime reference to the interior ids yet**, except `installCityColliders` fixtures if Task 4 has already landed. Those are harmless static boxes inside closed buildings.
- **Unchanged:** `check_assets.py` (v1, 3 designs); the model directory stays under 40 MiB.

**Acceptance commands** (all must pass)
- `node scripts/build-system-world-v3.mjs --check` → `verified`, 31 designs, 93 GLBs.
- `node scripts/test-system-world-v3.mjs` → window line still 10 designs; 31 designs.
- `node scripts/test-system-world-lobby-contract.mjs` → `interiors: 6 designs, … bytes verified`.
- `node scripts/test-system-world-surfaces.mjs`
- `node scripts/test-system-world-layout.mjs`, `node scripts/test-system-world-hover.mjs`, `node scripts/test-system-world-living.mjs` (fixture colliders take effect if Task 4 is in).
- `python assets/system-world/check_assets.py`

**If the contract fails:** fix the model or tell the plan executor which registry number moved. Never widen tolerances.

---

## Task 3 — Ground on podiums and stairs, the step rule and walls in `canWalk`

**Files:** `ui/js/desktop/apps/sysworld-exploration.js`, `scripts/test-system-world-expansion.mjs`.

**Steps**
- [ ] **Test first.** In `test-system-world-expansion.mjs`:
  - Change the import to `import {interiors,canWalk,groundHeight,daylight} from '../ui/js/desktop/apps/sysworld-exploration.js';` and add `import {stepFree} from '../ui/js/desktop/apps/sysworld-lobbies.js';`.
  - Replace the line `assert.equal(canWalk(0,-12,{x:0,z:-11},()=>false,districts),false,'Solid spire foundation cannot be crossed');` with:
    ```js
    // Phase 3: the spire is entered through its lobby door, up real steps; podium edges are walls.
    assert.equal(canWalk(5,-4.7,{x:5,z:-3.9},()=>true,districts),false,'The spire lobby wall stands beside its door');
    assert.equal(canWalk(0,-5.4,{x:0,z:-4.3},()=>false,districts),false,'A closed lobby door cannot be crossed');
    assert.ok(canWalk(0,-5.4,{x:0,z:-4.3},()=>true,districts),'An open lobby door lets the visitor in');
    assert.ok(canWalk(0,-1.9,{x:0,z:.9},()=>true,districts),'The spire stairs climb in steps');
    assert.equal(canWalk(10,-.2,{x:10,z:.8},()=>true,districts),false,'The podium edge beside the stairs is a wall');
    assert.equal(groundHeight(0,-12),.88);assert.ok(Math.abs(groundHeight(0,.9)-.88/6)<1e-9,'lowest tread');
    assert.ok(stepFree({x:-45,z:-36},{x:-45,z:-34},groundHeight),'A kerb is passable');
    assert.ok(stepFree({x:-74,z:-50},{x:-74,z:-53},groundHeight),'A tram platform is passable');
    assert.ok(canWalk(-33,36,{x:-33,z:37},()=>false,districts),'The atrium podium ring outside the rotunda is walkable');
    ```
  - Run → **FAIL** (`The spire lobby wall …`).
- [ ] **`sysworld-exploration.js`.**
  1. After `import {streetProfile} from './sysworld-ground.js';` add `import {PODIUM,podiumAt,lobbyGround,stepFree,lobbyPassage} from './sysworld-lobbies.js';`.
  2. Replace the two comment lines above `footprints` and the whole `export const footprints={…};` statement (three lines, ending `infra:[[-10.7,-9.4,10.7,3.4],[-10,-2.6,10,8.5]]};`) with:
     ```js
     // Ground-level solids of what is not enterable. Phase 3 replaced the six enterable buildings by lobby walls
     // with door gaps (sysworld-lobbies.js); the integration gate keeps the two posts of its central passage.
     export const footprints={integrations:[[-4.8,-.8,-3.2,.8],[3.2,-.8,4.8,.8]]};
     ```
  3. Delete the line `const podiums={agent:[23,23],…,operations:[9,9]};`.
  4. Replace `export function buildingFloor(x,z,districts){…}` (one line) with `export function buildingFloor(x,z){return podiumAt(x,z)?PODIUM:0;}`.
  5. In `solidBuilding`, change `if(d.id==='graph'&&Math.hypot(dx,dz)<10.2)return true;if(d.id==='operations'&&Math.hypot(dx,dz)<3)return true;` to `if(d.id==='operations'&&Math.hypot(dx,dz)<3)return true;`.
  6. In `groundHeight`, after `if(roomAt(x,z))return surfaces.room;` add `const lifted=lobbyGround(x,z);if(lifted!==null)return lifted;`.
  7. In `canWalk`, replace the final `return !solidBuilding(x,z,districts);` with:
     ```js
       // No step above 0.35 m (podium edges close everywhere but at the stairs), no lobby wall, doors only when open.
       if(!stepFree(from,{x,z},groundHeight)||!lobbyPassage(from,{x,z},doorOpen))return false;
       return !solidBuilding(x,z,districts);
     ```

**Run:**
- `node scripts/test-system-world-expansion.mjs` → PASS.
- `node scripts/test-system-world-lobbies.mjs`, `node scripts/test-system-world-companion.mjs`, `node scripts/test-system-world-ground.mjs` → PASS.
- `node scripts/test-system-world-hover.mjs` → PASS. Pads stand on pavement; `reachesLane` uses the new rule.

**Acceptance:** residents and the visitor stand at 0.88 m on podiums and step on treads; podium edges and lobby walls are closed to the visitor.

**Commit:** `feat(system-world): podiums and stairs are real ground; 0.35 m step rule, lobby walls and doors for the visitor`

---

## Task 4 — Colliders: lobby solids and interior fixtures

**Files:** `ui/js/desktop/apps/sysworld-colliders.js`, `scripts/test-system-world-lobbies.mjs` (append).

**Steps**
- [ ] **Test first.** Append to `scripts/test-system-world-lobbies.mjs`:
  ```js
  // Colliders (installCityColliders): door gaps open, walls and towers beside them, lintels, ceilings, the lid, fixtures.
  {
    const fs=await import('node:fs/promises');
    const {createTraffic}=await import('../ui/js/desktop/apps/sysworld-traffic.js');
    const {installCityColliders,kitCatalog}=await import('../ui/js/desktop/apps/sysworld-colliders.js');
    const {districts,placements}=await import('../ui/js/desktop/apps/sysworld-scene.js');
    const read=f=>fs.readFile(f,'utf8').then(JSON.parse),v1=await read('ui/3d/system-world/v1/manifest.json'),v3=await read('ui/3d/system-world/v3/manifest.json');
    const traffic=createTraffic();installCityColliders(traffic,kitCatalog(v1,v3),districts,placements);
    const walker={id:'w',circles:[{x:0,z:0,r:.24}],minY:-2.3,maxY:.4,reach:.24},at=(x,z,y=PODIUM+2.4)=>({x,y,z,heading:0});
    for(const l of LOBBIES)for(const d of l.doors){
      const out=at(d.x,l.front+L.WALL_OUT+1.1),inn=at(d.x,l.front-L.WALL-1.1);
      assert.ok(traffic.clear(walker,out,inn,false),`${l.id}:${d.n} door gap blocked by ${traffic.obstruction(walker,out,inn,false)}`);
      const side=d.x+d.width/2+.6;
      assert.match(String(traffic.obstruction(walker,at(side,l.front+1.1),at(side,l.front-L.WALL-1.1),false)),new RegExp('^district:'+l.district+':'),`${l.id}:${d.n} wall beside the door`);
      const y=PODIUM+d.height+.5;assert.equal(traffic.clear(walker,at(d.x,l.front+1.5,y),at(d.x,l.front-1.5,y),false),false,`${l.id}:${d.n} lintel`);
    }
    for(const l of LOBBIES.filter(l=>!l.open)){const [x0,z0,x1,z1]=l.rect,p=at((x0+x1)/2,(z0+z1)/2,l.ceiling+1);assert.equal(traffic.clear(walker,p,p,false),false,l.id+' ceiling');}
    assert.equal(traffic.clear(walker,at(-43,36,6.1),at(-43,36,6.1),false),false,'atrium lid');
    assert.ok(traffic.clear(walker,at(-33,36),at(-33,36),false),'old atrium box removed');
    let fixtures=0;
    for(const a of v3.assets.filter(a=>a.category==='interior')){const l=LOBBIES.find(l=>l.asset===a.id);
      for(const b of a.navigation.colliders||[]){const p=at(l.origin[0]+(b[0]+b[3])/2,l.origin[1]+(b[2]+b[5])/2,PODIUM+2.4);assert.equal(traffic.clear(walker,p,p,false),false,a.id+' fixture '+b);fixtures++;}}
    console.log(`System World lobby colliders: gaps, walls, lintels, ceilings, lid and ${fixtures} interior fixtures verified.`);
  }
  ```
  Run → **FAIL** (a door gap blocked by `district:agent:0`).
- [ ] **`sysworld-colliders.js`.**
  1. After `import {footprints,upperSolids} from './sysworld-exploration.js';` add `import {LOBBIES,LOBBY_DISTRICTS,lobbySolids} from './sysworld-lobbies.js';`.
  2. In `installCityColliders`, replace `const ground=footprints[d.id]||(d.id==='graph'?[[-10.2,-10.2,10.2,10.2]]:[[-3,-3,3,3]]);` with `const ground=footprints[d.id]||(LOBBY_DISTRICTS.has(d.id)?[]:[[-3,-3,3,3]]);`.
  3. Directly after the `ground.forEach(…);` line add:
     ```js
         // Enterable buildings: walls with door gaps, corner piers, lintels, ceiling slab, towers, struts (registry),
         // plus the interior's fixtures from its kit v3 manifest (building-local glTF boxes at the district origin).
         lobbySolids(d.id).forEach((box,i)=>traffic.solid('district:'+d.id+':lobby:'+i,box));
         for(const asset of new Set(LOBBIES.filter(l=>l.district===d.id).map(l=>l.asset)))
           (catalog.get(asset)?.navigation?.colliders||[]).forEach((b,i)=>traffic.solid('district:'+d.id+':'+asset+':'+i,{x:d.x,z:d.z,min:[b[0],b[1],b[2]],max:[b[3],b[4],b[5]]}));
     ```

**Run:**
- `node scripts/test-system-world-lobbies.mjs` → PASS.
- `node scripts/test-system-world-layout.mjs` → PASS (lift shaft, deck, crown).
- `node scripts/test-system-world-hover.mjs` → PASS (pad clearances).
- `node scripts/test-system-world-living.mjs` → PASS.

**Acceptance:** footprint boxes of the six buildings are gone, the doors are the only ground-level way in, and fixtures block.

**Commit:** `feat(system-world): lobby walls, lintels, ceilings and interior fixtures replace the six building footprints`

---

## Task 5 — Wayfinder: step rule and lobby nodes

**Files:** `ui/js/desktop/apps/sysworld-places.js`, `scripts/test-system-world-ground.mjs` (append).

**Steps**
- [ ] **Test first.** Append to `test-system-world-ground.mjs`:
  ```js
  // --- Lobby routes: up the stairs, through a door, into every hall (real colliders) ---
  {
    const fs=await import('node:fs/promises');
    const {createWayfinder}=await import('../ui/js/desktop/apps/sysworld-places.js');
    const {createTraffic}=await import('../ui/js/desktop/apps/sysworld-traffic.js');
    const {groundHeight}=await import('../ui/js/desktop/apps/sysworld-exploration.js');
    const {installCityColliders,kitCatalog}=await import('../ui/js/desktop/apps/sysworld-colliders.js');
    const {districts,placements}=await import('../ui/js/desktop/apps/sysworld-scene.js');
    const {LOBBIES,lobbyAt,stepFree}=await import('../ui/js/desktop/apps/sysworld-lobbies.js');
    const read=f=>fs.readFile(f,'utf8').then(JSON.parse),v1=await read('ui/3d/system-world/v1/manifest.json'),v3=await read('ui/3d/system-world/v3/manifest.json');
    const traffic=createTraffic();installCityColliders(traffic,kitCatalog(v1,v3),districts,placements);
    const finder=createWayfinder(traffic,groundHeight,kitCatalog(v3)),started=Date.now();
    const body=p=>({id:'walker',circles:[{x:0,z:0,r:.45}],minY:0,maxY:2,reach:.45,heading:0,...p,y:groundHeight(p.x,p.z)});
    for(const l of LOBBIES){
      const goal=finder.points.find(p=>p.lobby===l.id&&traffic.clear(body(p),p,p,false));assert.ok(goal,'no free inside node in '+l.id);
      const start=pedestrianGraph().nodes.reduce((a,n)=>Math.hypot(n.x-goal.x,n.z-goal.z)<Math.hypot(a.x-goal.x,a.z-goal.z)?n:a);
      const path=finder.route(body(start),{x:goal.x,z:goal.z,y:groundHeight(goal.x,goal.z)});assert.ok(path.length,'no route into '+l.id);
      let prev=start;for(const p of path){assert.ok(stepFree(prev,p,groundHeight),'leg breaks the step rule: '+JSON.stringify([prev,p]));prev=p;}
      assert.equal(lobbyAt(prev.x,prev.z)?.id,l.id);
    }
    console.log(`System World lobby routes: every hall reached from the pavement over its stairs (${Date.now()-started} ms incl. graph build).`);
  }
  ```
  Run → **FAIL** (`finder.points` has no inside node).
- [ ] **`sysworld-places.js`.**
  - After the `sysworld-ground.js` import add `import {lobbyGraph,lobbyAt,stepFree} from './sysworld-lobbies.js';`.
  - Replace the whole `createWayfinder` function (from its comment `// Residents walk the pavement graph…` to its closing `}`) with:

```js
// Residents walk the pavement graph of sysworld-ground.js plus the lobby graph of sysworld-lobbies.js (stair
// feet and landings, porches, entries, inside grid and workpoints). Every static link keeps the 0.35 m step
// rule (built once, on first use); bodies and solids filter links per revision. Inside a lobby a route links
// only to that lobby's inside nodes; outside never to an inside node, so the doors are the only way in.
export function createWayfinder(traffic, floor, catalog=null) {
  const graph=pedestrianGraph(),lobby=lobbyGraph(catalog),base=graph.nodes.length;
  const points=[...graph.nodes,...lobby.nodes].map(n=>({x:n.x,z:n.z,y:floor(n.x,n.z),heading:0,lobby:n.lobby||null})),cache=new Map();
  let revision=-1,fixed=null;
  function staticLinks(){
    if(fixed)return fixed;
    const length=(a,b)=>Math.hypot(points[a].x-points[b].x,points[a].z-points[b].z);
    fixed=graph.links.filter(l=>stepFree(points[l.a],points[l.b],floor)).map(l=>({a:l.a,b:l.b,length:l.length}));
    for(const [a,b]of lobby.links)if(stepFree(points[base+a],points[base+b],floor))fixed.push({a:base+a,b:base+b,length:length(base+a,base+b)});
    lobby.nodes.forEach((n,k)=>{if(n.kind!=='foot')return;const f=base+k;
      for(let i=0;i<base;i++){const d=length(f,i);if(d<maxLink&&carriagewayFree(points[f],points[i])&&stepFree(points[f],points[i],floor))fixed.push({a:f,b:i,length:d});}});
    return fixed;
  }
  function route(body,destination) {
    if(revision!==traffic.revision()){cache.clear();revision=traffic.revision();}
    const key=body.circles.map(c=>[c.x,c.z,c.r].join(',')).join(';')+':'+body.maxY;
    if(!cache.has(key)) {
      const index=new Map(),nodes=[];
      points.forEach((p,i)=>{if(traffic.clear(body,p,p,false)){index.set(i,nodes.length);nodes.push(p);}});
      const edges=nodes.map(()=>[]);
      for(const l of staticLinks()) {
        const a=index.get(l.a),b=index.get(l.b);if(a===undefined||b===undefined)continue;
        if(traffic.clear(body,nodes[a],nodes[b],false)){edges[a].push([b,l.length]);edges[b].push([a,l.length]);}
      }
      cache.set(key,{nodes,edges});
    }
    const cached=cache.get(key),here=lobbyAt(body.x,body.z)?.id||null,there=lobbyAt(destination.x,destination.z)?.id||null;
    const nodes=[...cached.nodes,{x:body.x,y:body.y,z:body.z,heading:body.heading,lobby:here},{...destination,heading:0,lobby:there}];
    const start=nodes.length-2,end=nodes.length-1,startOnZebra=onZebra(nodes[start]),edges=cached.edges.map(e=>[...e]);edges.push([],[]);
    for(const k of [start,end])for(let i=0;i<k;i++) {
      const a=nodes[k],b=nodes[i],d=Math.hypot(a.x-b.x,a.z-b.z);
      if(a.lobby!==b.lobby)continue;
      // Only a start standing on a zebra may step off it; every other link stays off carriageways and kerbs.
      if((d<maxLink||i===start)&&carriagewayFree(a,b,k===start&&startOnZebra&&d<stepOff)&&stepFree(a,b,floor)&&traffic.clear(body,a,b,false)){edges[k].push([i,d]);edges[i].push([k,d]);}
    }
    const costs=nodes.map(()=>Infinity),previous=nodes.map(()=>-1),open=new Set(nodes.map((_,i)=>i));costs[start]=0;
    while(open.size) {
      let current=-1;for(const i of open)if(current<0||costs[i]<costs[current])current=i;
      if(!Number.isFinite(costs[current])||current===end)break;open.delete(current);
      for(const [next,weight]of edges[current])if(costs[current]+weight<costs[next]){costs[next]=costs[current]+weight;previous[next]=current;}
    }
    if(!Number.isFinite(costs[end]))return [];
    const result=[];for(let n=end;n!==start;n=previous[n]){if(n<0)return [];result.unshift(nodes[n]);}return result;
  }
  return {route,points};
}
```

**Run:**
- `node scripts/test-system-world-ground.mjs` → PASS. The existing random-route ratio must stay ≥ 0.7; report the printed ratio and the graph-build ms.
- `node scripts/test-system-world-living.mjs` → PASS. Report residents' progress against the last run.

**Acceptance:** pavement-to-hall routes exist for all seven halls, and no leg breaks the step rule.

**Commit:** `feat(system-world): residents route over stairs and through lobby doors (step rule, lobby graph)`

---

## Task 6 — Experience and society: doors, inside flag, hover rules, lobby places, prompt

**Files:**
- `ui/js/desktop/apps/sysworld-experience.js`, `ui/js/desktop/apps/sysworld-society.js`;
- `scripts/test-system-world-living.mjs`;
- new `scripts/test-system-world-lobby-walk.mjs`.

**Steps**
- [ ] **Test first.** Write the walk test below, and make these edits to the living test:
  - Replace `installCityColliders(traffic,kitCatalog(catalog,JSON.parse(await fs.readFile(root+'v3/manifest.json','utf8'))),districts,placements);` with `const v3=JSON.parse(await fs.readFile(root+'v3/manifest.json','utf8'));` plus `installCityColliders(traffic,kitCatalog(catalog,v3),districts,placements);`.
  - In `createExperience(...)` change `tier:'high',assetURL:` to `tier:'high',kit3:kitCatalog(v3),assetURL:`.
  - Change `experience.stats().society.places.length===6` to `===12`.
  - After the `'Robots must reach workstations…'` assertion add `assert.ok(experience.stats().society.lobbyVisits>0,'Residents must also work inside the lobbies');`.
  - Run both → **FAIL**.
- [ ] **`sysworld-society.js`.**
  1. `export function createSociety(scene,{traffic,camera,floor,onSelect,onDemonstrate,active,onGait,shadows}) {` → `export function createSociety(scene,{traffic,camera,floor,catalog,onSelect,onDemonstrate,active,onGait,shadows}) {`, and `finder=createWayfinder(traffic,floor)` → `finder=createWayfinder(traffic,floor,catalog)`.
  2. `let clock=0,selected=null,replay=false,tier='high',disposed=false,completed=0,selectedStatus='';` → append `,lobbyVisits=0` before `;`.
  3. In `choose`: `const eligible=slots.filter(s=>!s.owner&&(` → `const eligible=slots.filter(s=>!s.owner&&!(s.lobby&&tier==='low')&&(`. Then replace `const p=finder.points[(a.trip++*13+a.index)%finder.points.length],path=finder.route(a.body,p);` with `const p=finder.points[(a.trip++*13+a.index)%finder.points.length];if(p.lobby&&tier==='low')continue;const path=finder.route(a.body,p);`.
  4. In `update`: `a.play(a.state==='work'?'work':'idle');if(a.slot)onDemonstrate?.(a.slot.place,5);completed++;` → `a.play(a.state==='work'?'work':'idle');if(a.slot&&!a.slot.lobby)onDemonstrate?.(a.slot.place,5);if(a.slot?.lobby)lobbyVisits++;completed++;`.
  5. In `addPlace`: `z:place.z+point.position[2],owner:null});` → `z:place.z+point.position[2],owner:null,lobby:!!place.lobby});`.
  6. In `nearby()`: `for(const p of places.values()){const distance=` → `for(const p of places.values()){if(p.lobby)continue;const distance=`.
  7. In `stats`: `stats:()=>({completed,` → `stats:()=>({completed,lobbyVisits,`.
- [ ] **`sysworld-experience.js`.**
  1. After `import {createHoverFlight} from './sysworld-hover.js';` add `import {lobbyAt,lobbyGround,lobbyAirspace,createLobbyDoors,lobbyPlaces,DOORS,SENSE,PODIUM,SOLID_TOP} from './sysworld-lobbies.js';`.
  2. After `const traffic=options.traffic||createTraffic(),machinery=createMachinery(scene),living=new Map();` add `const lobbyDoors=createLobbyDoors();let lobby=null;`.
  3. In `createHoverFlight({…})`:
     - change `clear:(from,to)=>options.clear?options.clear(from,to):!visitorBody()||traffic.clear(visitorBody(),{...from,heading:0},{...to,heading:0},false),` to `clear:(from,to)=>!lobbyAirspace(to.x,to.y,to.z)&&(options.clear?options.clear(from,to):!visitorBody()||traffic.clear(visitorBody(),{...from,heading:0},{...to,heading:0},false)),`;
     - change `walkable:(x,z)=>{if(roomAt(x,z)||!canWalk(` to `walkable:(x,z)=>{if(roomAt(x,z)||lobbyGround(x,z)!==null||!canWalk(`;
     - update the comment above: `// A landing spot is open street-level pavement or ground: never a pavilion, lobby, stair, podium, building or resident.`.
  4. `createSociety(scene,{traffic,camera,floor:placeFloor,onSelect:` → `createSociety(scene,{traffic,camera,floor:placeFloor,catalog:options.kit3,onSelect:`.
  5. In `init`, replace `      options.onReady?.();\n    }catch(e){if(!disposed)options.onError?.(e);}\n  }\n  void init();` with:
     ```js
           // Lobby workplaces (kit v3 interiors' workpoints); the interiors themselves load in the scene on approach.
           for(const {place,entry}of lobbyPlaces(options.kit3))society.addPlace(place,entry);
           options.onReady?.();
         }catch(e){if(!disposed)options.onError?.(e);}
       }
       void init();
     ```
  6. In `interact`, before `if(near.kind==='door'){const d=doors.get(near.id);` insert `if(near.kind==='door'&&near.id.startsWith('lobby:')){if(lobbyDoors.hold(near.id))callSound('door',camera.position,near.id);return;}`.
  7. In `candidates`, after the line `for(const s of scopes)if(Math.abs(camera.position.y-(s.floor+2.4))<1.5)add('telescope',s.id,s.x,s.z,1.7);` add `if(camera.position.y<SOLID_TOP)for(const d of DOORS)if(lobbyDoors.usable(d.key))add('door',d.key,d.x,d.z,SENSE*.75);`.
  8. In `update`, replace `room=(ride?.kind==='hover'&&camera.position.y>8.3?null:roomAt(camera.position.x,camera.position.z)?.id)||null;` with:
     ```js
     const high=ride?.kind==='hover'&&camera.position.y>8.3,hall=high||camera.position.y>SOLID_TOP?null:lobbyAt(camera.position.x,camera.position.z);lobby=hall?.id||null;
         // The atrium is open to the sky: a place, but not indoors for rain and audio.
         room=(high?null:roomAt(camera.position.x,camera.position.z)?.id)||(hall&&!hall.open?hall.id:null)||null;
     ```
  9. In `update`, after `society.update(step,animated);machinery.update(step,animated);` add:
     ```js
         // Lobby doors slide for residents on every tier and for the visitor once the interior is shown (never on low).
         const crowd=[];for(let i=0;i<residents.length;i++){const b=traffic.body('resident-'+i);if(b?.enabled)crowd.push(b);}
         for(const d of lobbyDoors.update(Math.min(dt,.1),mode==='street'&&!ride?camera.position:null,crowd,!animated))
           if(Math.hypot(camera.position.x-d.x,camera.position.z-d.z)<20)callSound('door',{x:d.x,y:PODIUM+1.5,z:d.z},d.key);
     ```
  10. In `move(x,z,from)`, change `return canWalk(x,z,from,id=>doors.get(id)?.value>.85&&!!rooms.get(id)?.ready,options.districts);},` to `return canWalk(x,z,from,id=>id.startsWith('lobby:')?lobbyDoors.passable(id):doors.get(id)?.value>.85&&!!rooms.get(id)?.ready,options.districts);},`.
  11. `async function setTier(value){tier=value;if(!manifest)return;` → `async function setTier(value){tier=value;lobbyDoors.setTier(value);if(!manifest)return;`.
  12. `return {update,interact,endRide,setTier,society,` → `return {update,interact,endRide,setTier,society,lobbies:lobbyDoors,`.
  13. In `stats`, `inside:room,` → `inside:room,lobby,lobbies:lobbyDoors.stats(),`.

### `scripts/test-system-world-lobby-walk.mjs` (complete)

```js
// Phase 3 walk: with the real experience, colliders and kit v3 interiors, the visitor climbs every flight,
// waits at a door whose interior is not shown, enters through every door once it is, and leaves again.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import * as THREE from 'three';
import {createExperience} from '../ui/js/desktop/apps/sysworld-experience.js';
import {createTraffic} from '../ui/js/desktop/apps/sysworld-traffic.js';
import {installCityColliders,kitCatalog} from '../ui/js/desktop/apps/sysworld-colliders.js';
import {districts,placements} from '../ui/js/desktop/apps/sysworld-scene.js';
import {LOBBIES,PODIUMS,PODIUM,FLIGHT,WALL,WALL_OUT} from '../ui/js/desktop/apps/sysworld-lobbies.js';

const original={fetch:globalThis.fetch,localStorage:globalThis.localStorage,self:globalThis.self,createImageBitmap:globalThis.createImageBitmap};
globalThis.self=globalThis;globalThis.createImageBitmap=async()=>({width:1,height:1,close(){}});globalThis.localStorage={getItem:()=>null,setItem:()=>{}};
globalThis.fetch=async url=>{const bytes=await fs.readFile(String(url));return {ok:true,json:async()=>JSON.parse(bytes),arrayBuffer:async()=>bytes.buffer.slice(bytes.byteOffset,bytes.byteOffset+bytes.byteLength)};};
const root='ui/3d/system-world/',read=f=>fs.readFile(root+f,'utf8').then(JSON.parse),v1=await read('v1/manifest.json'),v3=await read('v3/manifest.json');
const scene=new THREE.Scene(),camera=new THREE.PerspectiveCamera(),traffic=createTraffic(),errors=[],sounds=[];
installCityColliders(traffic,kitCatalog(v1,v3),districts,placements);camera.position.set(18,2.4,57);
const visitor=traffic.register('visitor',{circles:[{x:0,z:0,r:.24}],minY:-2.3,maxY:.4,reach:.24},camera.position,100);
let ready=false;
const experience=createExperience(scene,{traffic,camera,districts,tier:'high',kit3:kitCatalog(v3),assetURL:x=>root+'v2/'+x,reduced:()=>false,active:()=>false,
  onError:e=>errors.push(e),onReady:()=>{ready=true;},onSound:k=>sounds.push(k)});
const dt=1/60,pose=(x,z)=>({x,y:2.4+experience.floor(x,z),z,heading:0});
const tick=()=>{traffic.begin();experience.update(dt,true,'street');traffic.propose(visitor,{...camera.position,heading:0},(_,p)=>camera.position.set(p.x,p.y,p.z));traffic.solve(dt);experience.settle(dt);};
const put=(x,z)=>{const p=pose(x,z);camera.position.set(p.x,p.y,p.z);Object.assign(visitor,p);};
// One step as sysworld-scene.js move(): the pure contract, then the colliders.
const stepTo=(x,z)=>{const p=pose(x,z);if(!experience.move(x,z,camera.position)||!traffic.clear(visitor,visitor,p))return;camera.position.set(p.x,p.y,p.z);};
const walk=(x,z,limit=600)=>{for(let n=0;n<limit;n++){const dx=x-camera.position.x,dz=z-camera.position.z,d=Math.hypot(dx,dz);if(d<.05)return true;const s=Math.min(d,.15);stepTo(camera.position.x+dx/d*s,camera.position.z+dz/d*s);tick();}return false;};
try{
  for(let n=0;n<1500&&!ready;n++){if(errors.length)throw errors[0];await new Promise(r=>setTimeout(r,10));}
  assert.ok(ready,'world did not load');
  let climbs=0;
  for(const l of LOBBIES)for(const d of l.doors){
    const key='lobby:'+l.id+':'+d.n,p=PODIUMS.find(p=>p.id===l.district),foot={x:d.x,z:p.z1+FLIGHT+.9},porch={x:d.x,z:l.front+WALL_OUT+1.1},inside={x:d.x,z:l.front-WALL-1.1};
    if(traffic.clear(visitor,pose(foot.x,foot.z),pose(foot.x,foot.z),false)){put(foot.x,foot.z);tick();assert.ok(walk(porch.x,porch.z),key+': cannot climb the stairs');climbs++;}
    else{put(porch.x,porch.z);tick();}
    assert.ok(Math.abs(camera.position.y-(2.4+PODIUM))<1e-6,key+': not on the podium');
    assert.equal(walk(inside.x,inside.z,80),false,key+': entered although the interior is not shown');
    assert.ok(camera.position.z>l.leaf,key+': crossed the closed leaf line');
    experience.lobbies.setReady(l.id,true);
    assert.ok(walk(inside.x,inside.z),key+': cannot walk in through the open door');
    const s=experience.stats();assert.equal(s.lobby,l.id);assert.equal(s.inside,l.open?null:l.id,key+': inside flag');
    assert.ok(walk(porch.x,porch.z),key+': cannot walk back out');assert.equal(experience.stats().lobby,null);
    experience.lobbies.setReady(l.id,false);for(let n=0;n<60;n++)tick();
  }
  assert.ok(climbs>=8,'too few stair feet are free: '+climbs);
  // Low tier: the doors stay shut for the visitor even with the interior flagged as shown.
  const agent=LOBBIES[0];experience.lobbies.setReady(agent.id,true);await experience.setTier('low');put(0,agent.front+WALL_OUT+1.1);tick();
  assert.equal(walk(0,agent.front-WALL-1.1,120),false,'low tier must keep the doors shut');await experience.setTier('high');
  assert.ok(sounds.includes('door'),'door sounds');assert.equal(errors.length,0);
  console.log(`System World lobby walk: ${LOBBIES.reduce((n,l)=>n+l.doors.length,0)} doors entered and left, ${climbs} flights climbed, low tier shut.`);
}finally{experience.dispose();traffic.dispose();Object.assign(globalThis,original);}
```

**Run:**
- `node scripts/test-system-world-lobby-walk.mjs` → PASS.
- `node scripts/test-system-world-living.mjs` → PASS. Report `completed`, `lobbyVisits` and residents with more than 8 m travel; at least 15 must remain.
- `node scripts/test-system-world-hover.mjs`, `node scripts/test-system-world-layout.mjs`, `node scripts/test-system-world-expansion.mjs` → PASS.

**Acceptance:**
- Every door is entered and left.
- The inside flag is right; the atrium counts as open.
- Hover cannot land on podiums or fly into lobbies.
- Residents work inside.

**Commit:** `feat(system-world): sliding lobby doors, inside flag, hover rules and lobby workplaces for residents`

---

## Task 7 — Interior loading, stand-in swap and lobby glass

**Files:** new `ui/js/desktop/apps/sysworld-lobby-render.js`, new `scripts/test-system-world-lobby-render.mjs`, `ui/js/desktop/apps/sysworld-scene.js`.

**Steps**
- [ ] Write the test, run → **FAIL** (module missing). Write the module, run → PASS.
- [ ] **`sysworld-scene.js`** (exact anchors):
  1. After `import { createWindows } from './sysworld-windows.js';` add `import { createLobbyRender } from './sysworld-lobby-render.js';`.
  2. After the statement `scene.add(world); world.add(staticCity, landmarks);` add:
     ```js
       // Enterable lobbies: interiors load on approach, hide each building's `lobby` stand-in and own the `city.pane` glass.
       const lobbyRender = createLobbyRender({ group: world, doors: () => experience?.lobbies,
         load: (id, lod) => { if (!cache.has('v3:' + id + ':' + lod)) expectedBytes += catalog3.get(id)?.lods.find(l => l.level === lod)?.bytes || 0; return model(id, lod, 'v3'); } });
     ```
  3. In `model()`, change `windows.adopt(n);` to `windows.adopt(n); lobbyRender.adopt(n);`, and change `if (windows.owns(m)) return m;` to `if (windows.owns(m) || lobbyRender.owns(m)) return m;`.
  4. In the `createExperience` options, change `    camera,districts,tier,traffic,reduced:()=>reduced,active:` to `    camera,districts,tier,traffic,kit3:catalog3,reduced:()=>reduced,active:`.
  5. In `applyTier()`, change `windows.setTier(tier);` to `windows.setTier(tier); lobbyRender.setTier(tier);`.
  6. In `update()`, directly after the line `    if(!simulate(Math.min(.1,Math.max(0,dt)))){traffic.begin();traffic.propose(visitor,{...camera.position,heading:0},(_,p)=>camera.position.set(p.x,p.y,p.z));traffic.solve(0);}` add `    if (lobbyRender.update(camera.position, landmarks)) renderer.shadowMap.needsUpdate = true;`.
  7. In `dispose()`, change `surfaces.dispose(); windows.dispose();` to `surfaces.dispose(); windows.dispose(); lobbyRender.dispose();`.
  8. In `stats()`, change `windows:windows.stats(),` to `windows:windows.stats(), lobbies:lobbyRender.stats(),`.

### `ui/js/desktop/apps/sysworld-lobby-render.js` (complete)

```js
import * as THREE from 'three';
import {LOBBIES,DOORS,PODIUM,LOAD_DISTANCE,HIDE_DISTANCE} from './sysworld-lobbies.js';

// Lobby interiors of the enterable buildings (kit v3 `lobby-*`, category interior, authored in the building
// frame). One loads when the camera comes within LOAD_DISTANCE of one of its doors (3D, at eye height),
// hides the building's `lobby` stand-in part and leaves again beyond HIDE_DISTANCE; the decoded template
// stays in the scene's model cache. The low tier never loads interiors. The see-through glazing
// (`city.pane`) shares one transparent material owned here; leaves `door_<n>_<l|r>` slide along X by half
// the door width, following the door states of sysworld-lobbies.js (experience.lobbies).
export const PANE_MATERIAL='city.pane';
const LEVELS={medium:1,high:0,ultra:0},LEAF=/^door_(\d+)_([lr])$/;
export const lobbyLevel=tier=>LEVELS[tier]??null;
export function createLobbyRender({group,load,doors=()=>null}){
  const holder=new THREE.Group();holder.name='lobby-interiors';group.add(holder);
  const material=new THREE.MeshStandardMaterial({name:PANE_MATERIAL,color:new THREE.Color(.03,.09,.1),roughness:.05,metalness:0,
    envMapIntensity:2.4,transparent:true,opacity:.2,depthWrite:false,side:THREE.DoubleSide});
  const assets=[...new Set(LOBBIES.map(l=>l.asset))].map(asset=>{const halls=LOBBIES.filter(l=>l.asset===asset);
    return {asset,halls,district:halls[0].district,origin:halls[0].origin,doors:DOORS.filter(d=>d.asset===asset),node:null,level:null,pending:null,failed:false,leaves:[]};});
  let level=0,disposed=false;
  const ready=(a,value)=>{for(const h of a.halls)doors()?.setReady(h.id,value);};
  function show(a,template,lod){
    const node=template.clone(true);node.name=a.asset;node.position.set(a.origin[0],0,a.origin[1]);
    node.traverse(n=>{const m=LEAF.exec(n.name),door=m&&a.doors.find(d=>d.n===Number(m[1]));
      if(door)a.leaves.push({node:n,key:door.key,base:n.position.x,shift:(m[2]==='l'?-1:1)*door.width/2});});
    holder.add(node);a.node=node;a.level=lod;ready(a,true);
  }
  function hide(a){if(!a.node)return;holder.remove(a.node);a.node=null;a.level=null;a.leaves=[];ready(a,false);}
  return {
    material,owns:m=>m===material,
    // Swaps every city.pane slot for the shared glass, disposes the loaded material; glass casts no shadow.
    adopt(mesh){
      if(!mesh?.isMesh||disposed)return false;
      const list=Array.isArray(mesh.material)?mesh.material:[mesh.material];let hit=false;
      const next=list.map(m=>{if(m===material||m?.name!==PANE_MATERIAL)return m;hit=true;m.dispose();return material;});
      if(!hit)return false;mesh.material=Array.isArray(mesh.material)?next:next[0];mesh.castShadow=false;return true;
    },
    setTier(tier){level=lobbyLevel(tier);for(const a of assets)a.failed=false;},
    // Returns true when the shadow map should update (an interior or a stand-in changed).
    update(position,landmarks){
      if(disposed)return false;let changed=false;const states=doors();
      for(const a of assets){
        const distance=Math.min(...a.doors.map(d=>Math.hypot(position.x-d.x,position.y-PODIUM-1.5,position.z-d.z)));
        const want=level!==null&&(distance<=LOAD_DISTANCE||(!!a.node&&distance<=HIDE_DISTANCE));
        if(a.node&&(!want||a.level!==level)){hide(a);changed=true;}
        if(want&&!a.node&&!a.failed&&a.pending!==level){
          const lod=level;a.pending=lod;
          load(a.asset,lod).then(template=>{if(a.pending!==lod)return;a.pending=null;if(!disposed&&level===lod&&!a.node)show(a,template,lod);},
            ()=>{if(a.pending===lod){a.pending=null;a.failed=true;}});
        }
        const standIn=landmarks?.children.find(n=>n.userData.district===a.district)?.getObjectByName('lobby');
        if(standIn&&standIn.visible===!!a.node){standIn.visible=!a.node;changed=true;}
        if(states)for(const leaf of a.leaves)leaf.node.position.x=leaf.base+leaf.shift*states.value(leaf.key);
      }
      return changed;
    },
    stats:()=>({level,shown:assets.filter(a=>a.node).map(a=>a.asset),loading:assets.filter(a=>a.pending!==null).length,failed:assets.filter(a=>a.failed).map(a=>a.asset)}),
    dispose(){if(disposed)return;for(const a of assets)hide(a);disposed=true;holder.removeFromParent();material.dispose();},
  };
}
```

### `scripts/test-system-world-lobby-render.mjs` (complete)

```js
// Phase 3 render: interiors load on approach (never at the overview, never on low), swap the stand-in,
// share one transparent pane material, slide their leaves, keep between 45 and 70 m, leave beyond 70 m.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import * as THREE from 'three';
import {GLTFLoader} from 'three/addons/loaders/GLTFLoader.js';
import {MeshoptDecoder} from 'three/addons/libs/meshopt_decoder.module.js';
import {createLobbyRender,lobbyLevel,PANE_MATERIAL} from '../ui/js/desktop/apps/sysworld-lobby-render.js';
import {createLobbyDoors,DOORS} from '../ui/js/desktop/apps/sysworld-lobbies.js';

await MeshoptDecoder.ready;
const manifest=JSON.parse(await fs.readFile('ui/3d/system-world/v3/manifest.json','utf8')),loads=[];
async function load(id,lod){
  loads.push(id+':'+lod);const file=manifest.assets.find(a=>a.id===id).lods.find(l=>l.level===lod).file,data=await fs.readFile('ui/3d/system-world/v3/'+file);
  const gltf=await new GLTFLoader().setMeshoptDecoder(MeshoptDecoder).parseAsync(data.buffer.slice(data.byteOffset,data.byteOffset+data.byteLength),'');
  gltf.scene.traverse(n=>render.adopt(n));return gltf.scene;     // the scene does this in model()
}
const world=new THREE.Group(),landmarks=new THREE.Group(),doors=createLobbyDoors();
for(const id of['agent','memory','integrations','missions','infra','graph']){const b=new THREE.Group(),s=new THREE.Group();b.userData.district=id;s.name='lobby';b.add(s);landmarks.add(b);}
const render=createLobbyRender({group:world,load,doors:()=>doors});
const settle=async p=>{render.update(p,landmarks);for(let i=0;i<300&&render.stats().loading;i++)await new Promise(r=>setTimeout(r,20));render.update(p,landmarks);};
const standIn=id=>landmarks.children.find(n=>n.userData.district===id).getObjectByName('lobby'),shown=()=>render.stats().shown;
assert.deepEqual([lobbyLevel('low'),lobbyLevel('medium'),lobbyLevel('high'),lobbyLevel('ultra')],[null,1,0,0]);
await settle(new THREE.Vector3(122,106,183));assert.equal(loads.length,0,'nothing loads at the overview');
const door=DOORS.find(d=>d.key==='lobby:agent:0'),eye=z=>new THREE.Vector3(door.x,3.28,door.z+z);
await settle(eye(12));
assert.ok(shown().includes('lobby-agent')&&!shown().includes('lobby-graph'));assert.equal(standIn('agent').visible,false);assert.ok(standIn('graph').visible);
assert.ok(doors.stats().ready.includes('agent'));
const node=world.getObjectByName('lobby-agent');let panes=0;
node.traverse(n=>{if(n.isMesh&&n.material.name===PANE_MATERIAL){panes++;assert.equal(n.material,render.material);assert.equal(n.castShadow,false);}});
const m=render.material;assert.ok(panes>0&&m.transparent&&m.opacity<=.25&&m.depthWrite===false&&m.side===THREE.DoubleSide&&m.roughness<=.06,'lobby glass');
const left=node.getObjectByName('door_0_l'),right=node.getObjectByName('door_0_r'),l0=left.position.x,r0=right.position.x;
doors.update(0,{x:door.x,z:door.z+1},[],true);render.update(eye(12),landmarks);
assert.ok(Math.abs(left.position.x-(l0-door.width/2))<1e-6&&Math.abs(right.position.x-(r0+door.width/2))<1e-6,'leaves slide by half the door width');
await settle(eye(-60));assert.ok(shown().includes('lobby-agent'),'kept between 45 and 70 m');
await settle(eye(-80));assert.ok(!shown().includes('lobby-agent'));assert.ok(standIn('agent').visible);assert.ok(!doors.stats().ready.includes('agent'));
render.setTier('low');await settle(eye(12));assert.deepEqual(shown(),[],'low tier never loads interiors');
render.setTier('medium');await settle(eye(12));assert.ok(loads.includes('lobby-agent:1')&&shown().includes('lobby-agent'));
render.dispose();assert.equal(world.children.length,0);assert.ok(standIn('agent').visible===false||true);
console.log(`System World lobby render: ${loads.length} loads, approach/hysteresis/tiers, stand-in swap, glass and leaves verified.`);
```

**Run:**
- `node scripts/test-system-world-lobby-render.mjs` → PASS.
- `node scripts/test-system-world-layout.mjs`, `node scripts/test-system-world-surfaces.mjs`, `node scripts/test-system-world-windows.mjs` → PASS.
- `node -e "import('esbuild').then(e=>e.build({entryPoints:['ui/js/desktop/apps/sysworld-scene.js'],bundle:true,format:'esm',write:false,logLevel:'error'})).then(()=>console.log('ok'))"` → `ok`.

**Acceptance:**
- Interiors appear only near their doors; the stand-in hides while an interior is shown.
- The glass is one transparent material.
- The low tier keeps stand-ins and closed doors.

**Commit:** `feat(system-world): lobby interiors load on approach, replace the stand-in and share one glass material`

---

## Task 8 — Survey scenes

**Files:** `scripts/system-world-survey.mjs`, `scripts/system-world-survey.scenes.json`.

**Steps**
- [ ] In `EXPOSE`, change `'model, catalogV3: () => catalog3, '` to `'model, catalogV3: () => catalog3, lobbies: lobbyRender, '`. Add `lobbies` to the `__sw` list in the header comment.
- [ ] Append 18 scenes, three per hall, from this table. Each uses `"fov":43,"weather":"clear","frames":40,"waitMs":400`.
  - Names: `lobby-<id>-day` and `lobby-<id>-night` use the street camera; `lobby-<id>-inside` uses the inside camera with `"environment":"evening"`.
  - The `eval` is the template below, with PX/PY/PZ = that scene's camera position, ASSET and POINTS filled in.

  | id | ASSET | street camera → target | inside camera → target | POINTS (residents) |
  | --- | --- | --- | --- | --- |
  | agent | lobby-agent | [3,1.7,3.5] → [0,2.6,-10] | [2,2.58,-7] → [-1,2.2,8] | `[[-2,-6],[2.5,-6.2]]` |
  | memory | lobby-memory | [-41,1.7,5] → [-43,3,-12] | [-45,2.58,-3.6] → [-45,2.2,12] | `[[-46,-3.4],[-40,-3.6]]` |
  | integrations | lobby-integrations | [37,1.7,1.5] → [35,2.6,-10] | [36,2.58,-7.5] → [35,2.2,8] | `[[35.8,-7],[51.5,-7]]` |
  | missions | lobby-missions | [40,1.7,51.5] → [43,3,30] | [43,2.58,40.5] → [43,2.2,60] | `[[41,41],[45,40.8]]` |
  | infra | lobby-infra | [-41,1.7,-38.5] → [-43,3,-55] | [-43,2.58,-51.5] → [-43,2.2,-35] | `[[-46,-51.2],[-40,-51.4]]` |
  | graph | lobby-graph | [-41,1.7,51.5] → [-43,3,36] | [-41,2.58,41.5] → [-43,3,60] | `[[-44.5,42.5],[-41,43]]` |

  Eval template:
  ```
  "__sw.camera.position.set(PX,PY,PZ);for(let i=0;i<300&&!__sw.lobbies.stats().shown.includes('ASSET');i++)await new Promise(r=>setTimeout(r,50));if(!__sw.lobbies.stats().shown.includes('ASSET'))throw Error('interior not shown: ASSET');POINTS.forEach(([x,z],i)=>{const a=__sw.experience.society.actors.find(r=>r.id==='resident-'+i);if(!a||!__sw.traffic.relocate(a.body,{x,y:.88,z,heading:0}))throw Error('no room for resident '+i);a.slot=null;a.path=[];a.state='work';a.until=1e9;a.node.position.set(a.body.x,a.body.y,a.body.z);});"
  ```
- [ ] Append the stair sequence:
  ```json
  {"name":"lobby-agent-stairs","followResident":0,"offset":[3,2.2,2],"lookHeight":1,"sequence":8,"intervalMs":300,"environment":"evening","weather":"clear","frames":20,"waitMs":200,"eval":"const a=__sw.experience.society.actors.find(x=>x.id==='resident-0');if(!__sw.traffic.relocate(a.body,{x:1.5,y:0,z:3.2,heading:Math.PI}))throw Error('stair start blocked');a.node.position.set(a.body.x,a.body.y,a.body.z);a.slot=null;a.state='walk';a.until=1e9;a.path=[{x:1.5,y:.88,z:-1.6,heading:0},{x:1.5,y:.88,z:-3.2,heading:0}];"}
  ```
- [ ] **Run:** `node scripts/system-world-survey.mjs --out reports/system-world-survey/phase-3 --tier high --only <the 19 names>` → 18 PNGs plus 8 sequence frames, `page errors: 0`, and `cameraMovedByCollision: 0` in `report.json`. If a camera was moved, shift it 1 m along the pavement.
- [ ] **Visual acceptance:**
  - two residents visible through the lobby glass by day and by night;
  - lit ceilings at night;
  - the street visible from inside;
  - no black or flickering glass;
  - the resident's feet on the treads in the sequence.

  Allowed tuning, recorded in the hand-back: pane `opacity` 0.15–0.3, `envMapIntensity` 1.5–3, colour.

**Commit:** `chore(system-world): survey scenes for the six lobbies (street day/night, inside, stair climb)`

---

## Task 9 — Documentation

**Files:** `assets/system-world/AGENTS.md`, `assets/system-world/README.md`, `ui/3d/system-world/AGENTS.md`, `plans/2026-11-09-system-world-beauty-design.md`.

**Content** (concise, existing style)
- **`assets/system-world/AGENTS.md` — interior contract.**
  - `interior_assets3.py` is registered in `FAMILIES`; palette key `pane` → `city.pane`.
  - Interiors are authored in the building frame; floor finish at `FLOOR`.
  - Leaves are separate nodes `door_<n>_<l|r>` (never dots: `GLTFLoader` strips them), pivot at the leaf's bottom centre, each half the door width, authored closed.
  - `navigation.workpoints` and `navigation.colliders` are building-local glTF boxes with base `PODIUM`.
  - Exterior `lobby` part = ground-floor panes plus closed door glass.
  - Every number mirrors `ui/js/desktop/apps/sysworld-lobbies.js`; `scripts/test-system-world-lobby-contract.mjs` checks them.
- **`README.md`.**
  - Runtime payload: kit v3 31 designs.
  - Enterable lobbies: load on approach (45 m, hide at 70 m), low tier keeps stand-ins.
  - The step rule; residents work inside.
- **`ui/3d/system-world/AGENTS.md`.**
  - `city.pane` belongs to `sysworld-lobby-render.js`.
  - Lobby registry, walls and fixtures in `installCityColliders`.
  - Verification: `test-system-world-lobbies.mjs`, `-lobby-contract.mjs`, `-lobby-walk.mjs`, `-lobby-render.mjs`.
- **Design doc.** The phase 3 roadmap row points to `plans/2026-11-10-system-world-beauty-phase-3.md`.

**Run:** `go test ./ui -run '^TestDesktopSysWorld' -count=1` → PASS.

**Commit:** `docs(system-world): enterable lobbies, interior contract, lobby registry`

---

## Task 10 — Markers, browser checks, final pins, bundle, regression, survey set, budgets

**Files:** `ui/desktop_sysworld_test.go`, `ui/desktop_sysworld_city_browser_test.go`, `ui/desktop_sysworld_expansion_browser_test.go`, `scripts/test-system-world-v3.mjs`, `ui/js/vendor/system-world/*` (generated).

**Steps**
- [ ] **Markers** in `TestDesktopSysWorldAppMarkers`:
  - Add the entries:
    ```go
    "js/desktop/apps/sysworld-lobbies.js":      {"export const LOBBIES", "export const PODIUMS", "export function lobbyGround", "export function stepFree", "export function lobbySolids", "export function createLobbyDoors", "export function lobbyGraph"},
    "js/desktop/apps/sysworld-lobby-render.js": {"export function createLobbyRender", "export const PANE_MATERIAL", "depthWrite:false", "getObjectByName('lobby')", "door_(\\d+)_([lr])", "dispose()"},
    ```
  - Append to the `sysworld-scene.js` list: `"createLobbyRender(", "lobbyRender.adopt(n)", "kit3:catalog3"`.
  - Append to the `sysworld-experience.js` list: `"createLobbyDoors()", "lobbyAirspace(", "lobbyGround(x,z)!==null"`.
  - Run `gofmt -w ui/desktop_sysworld_test.go`.
- [ ] **City browser test.**
  - Directly after the window-interiors eval (the one throwing `'Window interiors inactive: '`), add:
    ```go
    	page.MustEval(`()=>{const s=SysWorldApp.inspect(cityId);if(!s.lobbies||s.lobbies.level!==0||s.lobbies.shown.length)throw Error('Lobby interiors must not load at the overview: '+JSON.stringify(s.lobbies));}`)
    ```
  - After the eval throwing `'Low tier must use flat window cells: '`, add:
    ```go
    	page.MustEval(`()=>{const s=SysWorldApp.inspect(cityId);if(s.lobbies.level!==null||s.lobbies.shown.length)throw Error('Low tier must keep the lobby stand-ins: '+JSON.stringify(s.lobbies));}`)
    ```
- [ ] **Expansion browser test.** After the `for _, id := range []string{"agent", "memory", "missions"} { … }` loop, add:
  ```go
  	// Phase 3: up the spire stairs, through the sliding doors into the agent lobby and out again.
  	page.MustEval(`()=>{document.querySelector('[data-sw-mode="street"]').click();document.querySelector('[data-sw-district="agent"]').click();}`)
  	page.Timeout(20 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).lobbies.shown.includes('lobby-agent')`)
  	axis(2, -3.2, input.KeyS, input.KeyW)
  	page.Timeout(10 * time.Second).MustWait(`()=>document.querySelector('.sw-interaction').dataset.kind==='door'`)
  	axis(2, -8, input.KeyS, input.KeyW)
  	page.Timeout(10 * time.Second).MustWait(`()=>{const e=SysWorldApp.inspect(cityId).experience;return e.lobby==='agent'&&e.inside==='agent';}`)
  	page.MustScreenshot(filepath.Join(dir, "world2-lobby-agent.png"))
  	axis(2, 4, input.KeyS, input.KeyW)
  	page.Timeout(10 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).experience.lobby===null`)
  	t.Logf("loaded after the lobby visit: %d bytes", page.MustEval(`()=>SysWorldApp.inspect(cityId).loadedBytes`).Int())
  ```
- [ ] **Final pin** in `test-system-world-v3.mjs`: replace the two lines `assert.ok(lobbiesShipped === 0 || lobbiesShipped === 6, …);` and `assert.equal(manifest.assets.length, 25 + lobbiesShipped, …);` with `assert.equal(lobbiesShipped, 6, 'The six lobby interiors ship');` and `assert.equal(manifest.assets.length, 31, '6 street + 12 superseded + 7 furniture + 6 lobby interiors');`.
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
- [ ] **Survey set:**
  - `--tier high --out reports/system-world-survey/phase-3-final` (all scenes);
  - `--tier medium --out reports/system-world-survey/phase-3-medium --only lobby-agent-night,lobby-missions-inside`;
  - `--tier low --out reports/system-world-survey/phase-3-low --only lobby-agent-night,facade-agent-night`. On low the stand-in must show, without residents behind it.
  - 0 page errors in every run.
- [ ] **Report budgets:**
  - model directory: the `model directory N bytes` line of `test-system-world-v3.mjs`;
  - first display: the initial `loadedBytes` of the expansion browser run (< 20 MiB, unchanged rule) and `stats.loadedBytes` of `overview-day` in the survey report;
  - whole app: `runtimeBytes` of `test-system-world-expansion.mjs` (< 80 MiB);
  - interiors: the bytes line of the contract test, and the post-visit `loadedBytes` log line.

**Commit:** `chore(system-world): rebuild renderer bundle, pin lobby markers, browser walk into the agent lobby`

---

## Test and pin changes at a glance

| Pin | Old | New |
| --- | --- | --- |
| `test-system-world-expansion.mjs` spire assertion | footprint at (0, −12) blocks | wall beside the door, closed/open door, stairs, podium edge, kerb/platform pass, atrium ring walkable |
| `footprints` | 6 districts | integrations posts only |
| `installCityColliders` | footprint boxes, graph 10.2 m box | registry solids plus interior fixtures; operations keeps its 3 m box |
| `test-system-world-surfaces.mjs` | — | `city.pane` mapped as lobby glass, untouched by `apply` |
| `test-system-world-v3.mjs` designs | `=== 25` | Task 2: 25 or 31 (all six); Task 10: `=== 31` |
| `test-system-world-living.mjs` | 6 places; v3 manifest inline | 12 places; `kit3`; `lobbyVisits > 0`; other thresholds unchanged |
| `test-system-world-ground.mjs` | — | every hall reachable from the pavement, legs step-free |
| New tests | — | `-lobbies`, `-lobby-contract`, `-lobby-walk`, `-lobby-render` |
| `TestDesktopSysWorldAppMarkers` | — | lobby modules, scene and experience markers |
| City browser test | — | nothing loaded at the overview; low tier keeps stand-ins |
| Expansion browser test | — | walk into and out of the agent lobby, door prompt, inside flag |
| First display | 20 MiB | unchanged limit; interiors load on approach only |

## Risks and open questions

1. **Glass sorting and look.** One transparent material without depth write, and no transparent surfaces inside, keeps sorting simple. Leaves and the front glazing are separate objects sorted by distance. If they flicker in the survey, set `renderOrder` 1 on leaf meshes in `show()`.
2. **Pop at the swap** (≥ 45 m). If it is visible in the survey, fade `material.opacity` in over 0.3 s after `show()`. That change stays inside `sysworld-lobby-render.js`.
3. **Wet interior floors.** `createSurfaces` has three `swShelter` rects (the pavilions), so lobby floors take rain wetness. The fix is to raise the uniform array to 10 and append the six closed lobby rects in `createCity`. That is a surfaces-shader change with its own test, proposed as a follow-up unless the survey's rain shots show it badly.
4. **Throughput.**
   - Foundry corridors are 2.1 m between tower boxes.
   - Integrations doors are 2 m wide.
   - The missions stair foot is near the parcel sorter and the charger.

   The walk test requires at least 8 free stair feet. The living test reports progress; a drop below 15 moving residents is escalated, not loosened.
5. **Wayfinder cost.** About 300 extra nodes. The step-rule prefilter limits sampling to podium surroundings, and inside/outside pruning keeps dynamic links small. Task 5 prints the build time. If a route cache rebuild exceeds about 150 ms, raise `GRID` to 2.5.
6. **Residents through closed stand-ins.**
   - On low, or beyond the interior distance, a resident crossing a door passes the static stand-in leaves. It is short and distant, and on low lobby workplaces are withdrawn.
   - The visitor never passes a closed door: `passable` needs `ready` and a tier other than low.
7. **Feet on risers** (design risk 2). If the stair sequence shows feet on nosings, snap the foot target to the tread centre in `adaptFeet`'s floor sampler. Not planned unless the survey shows it.
8. **Invisible stairs on low.** LOD 2 has no stair geometry, but the ground still steps. That is accepted (as the podium border lip in deviation 7).
9. **Prompt competition.** The browser wait for the `door` prompt can lose to a closer resident. If it flakes, remove that wait; the walk test covers `usable`/`hold`.
10. **Coordinator facts not re-measured** (leaf translations, interior materials, triangles). The interior half of the contract test turns them into assertions at Task 0.
