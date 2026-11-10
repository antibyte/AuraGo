import * as THREE from 'three';
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { MeshoptDecoder } from 'three/addons/libs/meshopt_decoder.module.js';
import { RoomEnvironment } from 'three/addons/environments/RoomEnvironment.js';
import { EffectComposer } from 'three/addons/postprocessing/EffectComposer.js';
import { Pass, FullScreenQuad } from 'three/addons/postprocessing/Pass.js';
import { UnrealBloomPass } from 'three/addons/postprocessing/UnrealBloomPass.js';
import { OutputPass } from 'three/addons/postprocessing/OutputPass.js';
import { CopyShader } from 'three/addons/shaders/CopyShader.js';
import { createCityLife } from './sysworld-life.js';
import { obstaclesFrom } from './sysworld-navigation.js';
import { createMemoryHologram } from './sysworld-hologram.js';
import { createAtmosphere } from './sysworld-atmosphere.js';
import { createFlow } from './sysworld-flow.js';
import { createDrones } from './sysworld-drones.js';
import { createExperience } from './sysworld-experience.js';
import { createWeather } from './sysworld-weather.js';
import { interiors, groundHeight } from './sysworld-exploration.js';
import { springStep } from './sysworld-footing.js';
import { streets, towers, surfaces as groundLevels } from './sysworld-layout.js';
import { createCompanion } from './sysworld-companion.js';
import { createContactShadows } from './sysworld-contact.js';
import {createTraffic,fixedSteps} from './sysworld-traffic.js';
import {installCityColliders,kitCatalog} from './sysworld-colliders.js';
import { createSurfaces } from './sysworld-surfaces.js';
import { createWindows } from './sysworld-windows.js';
import { createLobbyRender } from './sysworld-lobby-render.js';
import { streetModules, plotSlabs, island } from './sysworld-ground.js';
import { furnishing, SIZES as furnitureSizes, NEAR, NEAR_HEIGHT, SURFACES } from './sysworld-furnishing.js';
export { createCityAmbience } from './sysworld-audio.js';

// Metres, Y up. Stable district anchors are shared with the accessible map.
export const districts = [
  { id: 'agent', asset: 'agent-spire', x: 0, z: -12, height: 87, radius: 13 },
  { id: 'infra', asset: 'compute-foundry', x: -43, z: -53, height: 24, radius: 18 },
  { id: 'integrations', asset: 'integration-gate', x: 43, z: -10, height: 34, radius: 14 },
  { id: 'missions', asset: 'mission-terminal', x: 43, z: 36, height: 23, radius: 15 },
  { id: 'memory', asset: 'memory-archive', x: -43, z: -10, height: 37, radius: 14 },
  { id: 'graph', asset: 'knowledge-atrium', x: -43, z: 36, height: 28, radius: 17 },
  { id: 'operations', asset: 'operations-beacon', x: 0, z: 36, height: 22, radius: 6 },
];
const tiers = {
  low: { lod: 2, dpr: 1, shadow: 0, bloom: false },
  medium: { lod: 1, dpr: 1.25, shadow: 1024, bloom: false },
  high: { lod: 0, dpr: 1.5, shadow: 2048, bloom: true },
  ultra: { lod: 0, dpr: 2, shadow: 4096, bloom: true },
};
const home = new THREE.Vector3(122, 106, 183);

// Renders the scene into a multisampled HDR target and resolves it into the composer's plain
// buffers, so bloom blends onto a resolved texture instead of a multisampled one. The copy also
// zeroes NaN/Inf pixels: multisampled edge extrapolation can push shader inputs out of range,
// and a single NaN would spread through the bloom mip chain and black out the whole frame.
class SceneCapturePass extends Pass {
  constructor(scene, camera, samples) {
    super(); this.scene = scene; this.camera = camera; this.needsSwap = false;
    this.target = new THREE.WebGLRenderTarget(1, 1, { type: THREE.HalfFloatType, samples });
    this.quad = new FullScreenQuad(new THREE.ShaderMaterial({
      uniforms: THREE.UniformsUtils.clone(CopyShader.uniforms), vertexShader: CopyShader.vertexShader, depthTest: false, depthWrite: false,
      fragmentShader: 'uniform sampler2D tDiffuse;varying vec2 vUv;void main(){vec4 c=texture2D(tDiffuse,vUv);if(any(isnan(c))||any(isinf(c)))c=vec4(0.0,0.0,0.0,1.0);gl_FragColor=c;}',
    }));
  }
  render(renderer, writeBuffer, readBuffer) {
    const autoClear = renderer.autoClear; renderer.autoClear = false;
    renderer.setRenderTarget(this.target); renderer.clear(); renderer.render(this.scene, this.camera);
    this.quad.material.uniforms.tDiffuse.value = this.target.texture;
    renderer.setRenderTarget(this.renderToScreen ? null : readBuffer); this.quad.render(renderer);
    renderer.autoClear = autoClear;
  }
  setSize(width, height) { this.target.setSize(width, height); }
  dispose() { this.target.dispose(); this.quad.material.dispose(); this.quad.dispose(); }
}
const homeTarget = new THREE.Vector3(0, 22, -12);
// Architectural scale reference: skyline uses the same compact building kit. The placement
// list is static so the robot navigator and tests derive the same street obstacles.
// Shared kit materials are keyed by name plus vertex colours: a v3 material with baked AO
// (COLOR_0) must never be reused by a v1/v2 mesh without that attribute.
export function sharedMaterialKey(material) { return material.name + (material.vertexColors ? '|vertex-colours' : ''); }
// Kit v3 placements, instanced like `placements` but loaded from ui/3d/system-world/v3/.
// Kit v3 streets: fixed-length modules, never stretched (layout and order from sysworld-ground.js).
// Kit v3 placements, instanced like `placements` but loaded from ui/3d/system-world/v3/: the street modules (fixed
// length, never stretched; sysworld-ground.js) and the phase 4 furniture and signs (sysworld-furnishing.js).
export const streetPlacements = streetModules().map(m => ({ asset: m.id, x: m.x, z: m.z, y: 0, angle: m.angle, scale: [1, 1, 1] }));
export const kitPlacements = [...streetPlacements, ...furnishing().map(f => ({ asset: f.asset, x: f.x, z: f.z, y: f.y, angle: f.angle, scale: [1, 1, 1], area: f.area }))];
// Designs kit v3 took over from v1 in phase 2. Wherever both kits ship an id, v3 wins: districts,
// skyline, towers, lamps and planters load the v3 design (kitFor), colliders read its bounds.
export const supersededV1 = ['agent-spire', 'memory-archive', 'integration-gate', 'mission-terminal', 'compute-foundry', 'knowledge-atrium',
  'operations-beacon', 'data-tower-a', 'data-tower-b', 'skybridge', 'street-lamp', 'planter'];
export function kitFor(id, v3Catalog) { return v3Catalog?.has(id) ? 'v3' : 'v1'; }
export const placements = [];
{
  const place = (asset, x, z, y = 0, angle = 0, scale = [1, 1, 1]) => placements.push({ asset, x, z, y, angle, scale });
  const {xs,zs} = streets;
  // Lamps and planters stand 5 cm into the pavement. Each planter sits on the block-centre side of
  // its lamp (towards x = +-43), clear of every zebra crossing and its dropped kerb.
  const sunk = groundLevels.pavement - .05;
  for (const z of zs) for (const x of [-55, -31, 31, 55]) {
    place('street-lamp', x, z - 4.5, sunk); place('planter', x + 4 * Math.sign(Math.sign(x) * 43 - x), z - 4.5, sunk);
  }
  for (const {x,z,scale:s} of towers) {
    place('data-tower-a', x, z, 0, 0, [1,s,1]);
  }
  place('skybridge', 39.75, -55, 15);
  place('server-rack', -55, -36.7, groundLevels.pavement); place('server-rack', -49, -36.7, groundLevels.pavement);
  // Distant buildings are scenery, never presented as additional real entities. The seeded
  // skyline is jittered, leaves gaps, turns some towers and rises towards the middle; a
  // lower far row behind it adds depth that the fog keeps faint.
  let seed = 7919; const random = () => (seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0) / 4294967296;
  for (let row = 0; row < 4; row++) for (let col = 0; col < 12; col++) {
    const jx = random(), jz = random(), tall = random(), gap = random(), wide = random(), turn = random();
    if (gap < .12) continue;
    const x = (col - 5.5) * 22 + (jx - .5) * 12, z = -143 - row * 29 + (jz - .5) * 14, centre = 1 - Math.min(1, Math.abs(x) / 150);
    const s = (.5 + tall * tall + .45 * centre) * (row ? 1 : .85), w = .7 + .34 * wide;
    place(jx > .5 ? 'data-tower-a' : 'data-tower-b', x, z, -3, turn < .3 ? Math.PI / 2 : 0, [w, s, w]);
  }
  for (let i = 0; i < 16; i++) {
    const x = (i - 7.5) * 34 + (random() - .5) * 16, z = -268 - random() * 46;
    place(random() < .5 ? 'data-tower-a' : 'data-tower-b', x, z, -3, 0, [1.1, .55 + random() * 1.1, 1.1]);
  }
}
// Street robots also steer around the kit pieces standing in a street band (the pavement furniture).
const inStreet = p => streets.xs.some(x => Math.abs(p.x - x) <= streets.halfWidth) || streets.zs.some(z => Math.abs(p.z - z) <= streets.halfWidth);
export const obstacles = [...obstaclesFrom([...placements, ...kitPlacements.filter(p => furnitureSizes[p.asset] && inStreet(p))], furnitureSizes), ...interiors.flatMap(r=>[-3,3].flatMap(dx=>[-3,3].map(dz=>({x:r.x+dx,z:r.z+dz,r:4.25,asset:'interior'}))))];

export async function createCity(host, options) {
  let disposed = false, visible = true, mode = 'orbit', quality = options.quality || 'auto';
  let tier = quality === 'auto' ? 'high' : quality, generation = 0, flight = null;
  let frames = 0, measured = 0, sampleFrames = 0, lastQualityChange = 0, loadBytes = 0;
  let tourTime = 0, tourIndex = 0, selected = null, reduced = options.reducedMotion;
  let width = 1, height = 1, objects = [], failed = false, observer = null, life = null, companion = null;
  const abort = new AbortController(), requests = new Set(), cache = new Map(), materials = new Map();
  const geoSet = new Set(), matSet = new Set(), keys = new Set(), cleanup = [];
  const renderer = new THREE.WebGLRenderer({ antialias: true, alpha: false });
  renderer.toneMapping = THREE.ACESFilmicToneMapping; renderer.toneMappingExposure = .98;
  // Shadows: three removed the soft PCF variant, so the city uses the plain
  // filtered map like the other 3D apps and keeps the console clean.
  renderer.shadowMap.type = THREE.PCFShadowMap;
  renderer.shadowMap.autoUpdate = false; renderer.info.autoReset = false;
  const canvas = renderer.domElement;
  canvas.className = 'sysworld-gl'; canvas.tabIndex = 0;
  canvas.setAttribute('aria-label', options.label);
  host.append(canvas);
  const scene = new THREE.Scene();
  scene.background = new THREE.Color(0x08121e); scene.fog = new THREE.FogExp2(0x08121e, .004);
  const camera = new THREE.PerspectiveCamera(43, 1, .3, 1800);
  camera.position.copy(home);
  const traffic=createTraffic(),visitor=traffic.register('visitor',{circles:[{x:0,z:0,r:.38}],minY:-.4,maxY:.4,reach:.38},camera.position,100);
  const controls = new OrbitControls(camera, canvas);
  controls.target.copy(homeTarget); controls.enableDamping = true; controls.dampingFactor = .085;
  controls.minDistance = 12; controls.maxDistance = 410; controls.maxPolarAngle = Math.PI * .485;
  controls.update();
  // Tileable surface detail, pane lighting and rain wetness for the UV-less kit; pavilion floors stay dry.
  let expectedBytes = 0;
  const reportProgress = () => { const s = surfaces?.progress(); options.onProgress?.(loadBytes + (s?.bytes || 0), expectedBytes + (s?.expected || 0)); };
  const surfaces = createSurfaces(renderer, { url: file => options.resourceURL('/3d/system-world/textures/v1/' + file), onProgress: () => reportProgress(),
    shelters: interiors.map(r => [r.x - r.width / 2, r.z - r.depth / 2, r.x + r.width / 2, r.z + r.depth / 2]) });
  // Kit v3 window panes share one interior-mapped glass material (sysworld-windows.js).
  const windows = createWindows();
  const pmrem = new THREE.PMREMGenerator(renderer), room = new RoomEnvironment();
  const environment = pmrem.fromScene(room, .05);
  scene.environment = environment.texture; scene.environmentIntensity = .48;
  room.dispose(); pmrem.dispose();
  const hemisphere = new THREE.HemisphereLight(0xc0dcff, 0x10202d, .8);scene.add(hemisphere);
  const sun = new THREE.DirectionalLight(0xc8deff, 3.2);
  sun.position.set(-70, 145, 85); sun.castShadow = true;
  // The frustum follows the walkable city, not the distant skyline, so the
  // shadow map spends its resolution where the visitor actually stands.
  Object.assign(sun.shadow.camera, { left: -108, right: 108, top: 108, bottom: -108, near: 1, far: 360 });
  sun.shadow.normalBias = .09; sun.shadow.bias = -.00015; sun.shadow.radius = 2.4; scene.add(sun);
  const rim = new THREE.DirectionalLight(0xffb572, 2.4); rim.position.set(90, 80, -120); scene.add(rim);
  // A dim upward bounce keeps the shadow sides of the districts readable
  // without flattening the key light.
  const bounce = new THREE.DirectionalLight(0xd89a6a, .42); bounce.position.set(-10, -40, 30); scene.add(bounce);
  const reactorLight = new THREE.PointLight(0x62dfff, 0, 75, 2);
  reactorLight.position.set(0, 34, -12); scene.add(reactorLight);
  const renderTarget = new THREE.WebGLRenderTarget(1, 1, { type: THREE.HalfFloatType });
  const composer = new EffectComposer(renderer, renderTarget);
  composer.addPass(new SceneCapturePass(scene, camera, Math.min(4, renderer.capabilities.maxSamples)));
  // HDR threshold selects luminous windows/signals; dark work surfaces never bloom.
  // A wider, slightly lower threshold gives the new emissive landmarks and the
  // night windows a softer halo instead of a hard edge.
  const bloom = new UnrealBloomPass(new THREE.Vector2(1, 1), .28, .68, 1.06);
  composer.addPass(bloom); composer.addPass(new OutputPass());
  const world = new THREE.Group(), staticCity = new THREE.Group(), landmarks = new THREE.Group();
  scene.add(world); world.add(staticCity, landmarks);
  // Enterable lobbies: interiors load on approach, hide each building's `lobby` stand-in and own the `city.pane` glass.
  const lobbyRender = createLobbyRender({ group: world, doors: () => experience?.lobbies,
    load: (id, lod) => { if (!cache.has('v3:' + id + ':' + lod)) expectedBytes += catalog3.get(id)?.lods.find(l => l.level === lod)?.bytes || 0; return model(id, lod, 'v3'); } });
  const contact = createContactShadows(scene);
  function ownMesh(geometry, material) {
    geoSet.add(geometry); matSet.add(material); return new THREE.Mesh(geometry, material);
  }
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
  // Sky, clouds, stars, moon, animated sea, mist, dust, spire beacon, lamp cones, searchlights,
  // aviation lights and the cinematic grade.
  const atmosphere = createAtmosphere(scene, { sunDirection: sun.position, lamps: placements.filter(p => p.asset === 'street-lamp') });
  composer.addPass(atmosphere.post);
  // Decorative data traffic between the districts. Fixed routes, bounded pool,
  // never a reading of the system.
  const flow = createFlow(scene, { anchors: districts.map(d => ({ id: d.id, x: d.x, z: d.z, height: d.height })) });
  const memoryDistrict = districts.find(d => d.id === 'memory');
  const hologram = createMemoryHologram(scene, memoryDistrict, { roof: 21, label: options.memoryLabel });
  const drones = createDrones(scene,{traffic,surfaces});
  const weather = createWeather(scene,{sun,rim,hemisphere,atmosphere,surfaces,onThunder:delay=>options.onThunder?.(delay)});
  let moodKey = '', nearHidden = null;
  let experience=null;
  // Selection: dashed, slowly turning, pulsing in the district's state colour. Hover: a quiet solid ring.
  const ringTime = { value: 0 }, ringGeometry = new THREE.RingGeometry(1, 1.045, 96); geoSet.add(ringGeometry);
  const ringMaterial = (opacity, dashes) => { const m = new THREE.ShaderMaterial({ transparent: true, depthWrite: false, side: THREE.DoubleSide,
    uniforms: { color: { value: new THREE.Color(0x8ee8ee) }, time: ringTime, opacity: { value: opacity }, dashes: { value: dashes } },
    vertexShader: 'varying vec2 vP;void main(){vP=position.xy;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}',
    fragmentShader: 'uniform vec3 color;uniform float time,opacity,dashes;varying vec2 vP;void main(){float a=atan(vP.y,vP.x)/6.2831853;float dash=dashes>0.5?step(0.3,fract(a*dashes-time*0.35)):1.0;gl_FragColor=vec4(color,opacity*dash*(0.8+0.2*sin(time*2.2)));}' });
    matSet.add(m); return m; };
  const selection = new THREE.Mesh(ringGeometry, ringMaterial(.9, 18)), hoverRing = new THREE.Mesh(ringGeometry, ringMaterial(.38, 0));
  for (const ring of [selection, hoverRing]) { ring.rotation.x = -Math.PI / 2; ring.visible = false; world.add(ring); }
  // A single expanding ripple marks a new selection. It is one reused mesh and
  // one draw call, and it never runs while reduced motion is active.
  const rippleMaterial = new THREE.ShaderMaterial({ transparent: true, depthWrite: false, side: THREE.DoubleSide,
    uniforms: { color: { value: new THREE.Color(0x8ee8ee) }, alpha: { value: 0 } },
    vertexShader: 'varying vec2 vP;void main(){vP=position.xy;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}',
    fragmentShader: 'uniform vec3 color;uniform float alpha;varying vec2 vP;void main(){float a=atan(vP.y,vP.x)/6.2831853;gl_FragColor=vec4(color,alpha*(0.55+0.45*sin(a*3.0)));}' });
  matSet.add(rippleMaterial);
  const ripple = new THREE.Mesh(new THREE.RingGeometry(.9, 1, 96), rippleMaterial);
  geoSet.add(ripple.geometry); ripple.rotation.x = -Math.PI / 2; ripple.visible = false; world.add(ripple);
  let rippleTime = 2;
  const stateColours = { error: 0xff7868, running: 0x75f4d1, stale: 0x7b8b9b, idle: 0x7cbfe3 }, districtState = {};
  const tintSelection = () => selection.material.uniforms.color.value.setHex(stateColours[districtState[selected]] ?? 0x8ee8ee);
  const beaconMaterial = new THREE.MeshBasicMaterial({ color: 0x66e0d6, toneMapped: false });
  const beacons = new THREE.InstancedMesh(new THREE.SphereGeometry(.65, 10, 6), beaconMaterial, districts.length);
  geoSet.add(beacons.geometry); matSet.add(beaconMaterial); world.add(beacons);
  const scratch = new THREE.Object3D(), v = new THREE.Vector3(), ray = new THREE.Raycaster(), pointer = new THREE.Vector2();
  districts.forEach((d, i) => { scratch.position.set(d.x, d.height + 2, d.z); scratch.updateMatrix(); beacons.setMatrixAt(i, scratch.matrix); beacons.setColorAt(i, new THREE.Color(0x80909e)); });
  options.signal?.addEventListener('abort', dispose, { once: true });
  let manifest;
  try {
    if (options.signal?.aborted) throw Error('Disposed');
    const manifestResponse = await fetch(options.assetURL('manifest.json'), { signal: abort.signal });
    if (!manifestResponse.ok) throw Error('City manifest unavailable');
    manifest = await manifestResponse.json();
  } catch (error) { dispose(); throw error; }
  const catalog = new Map(manifest.assets.map(a => [a.id, a]));
  // Kit v3 is optional until its designs ship: an absent or empty manifest leaves the catalog empty.
  const catalog3 = new Map();
  try {
    const response = await fetch(options.resourceURL('/3d/system-world/v3/manifest.json'), { signal: abort.signal });
    const kit = response.ok ? await response.json() : null;
    if (kit?.schema_version === 3 && Array.isArray(kit.assets)) for (const a of kit.assets) catalog3.set(a.id, a);
  } catch (error) { if (abort.signal.aborted) { dispose(); throw error; } }
  installCityColliders(traffic,kitCatalog(catalog,catalog3),districts,placements,kitPlacements);
  experience=createExperience(scene,{
    shadows:contact,
    camera,districts,tier,traffic,kit3:catalog3,reduced:()=>reduced,active:()=>visible&&!failed&&mode!=='map'&&!reduced&&!options.replaying?.(),
    assetURL:file=>options.resourceURL('/3d/system-world/v2/'+file),surfaces,
    clear:(from,to)=>traffic.clear(visitor,{...from,heading:0},{...to,heading:0},false),onFlight:state=>options.onFlight?.(state),
    onInteraction:options.onInteraction,onScope:state=>options.onScope?.(state),onDiscover:options.onDiscover,onSound:options.onSound,onGait:options.onGait,onTerminal:options.onTerminal,onSociety:options.onSociety,
    onEnvironment:indoor=>{weather.setIndoor(indoor);options.onEnvironment?.(indoor);},
    onError:options.onError,onReady:()=>{renderer.shadowMap.needsUpdate=true;},
  });
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
          // Window panes (city.window) take the shared interior material on every path that loads
          // a model (instanced city, district clones, LOD rebuilds, skyline). It never enters the
          // shared-material map and sysworld-surfaces.js leaves it alone.
          windows.adopt(n); lobbyRender.adopt(n);
          const source = Array.isArray(n.material) ? n.material : [n.material];
          const shared = source.map(m => {
            if (windows.owns(m) || lobbyRender.owns(m)) return m;
            const name = sharedMaterialKey(m);
            if (materials.has(name)) { const original = m; m = materials.get(name); original.dispose(); }
            else {
              // Smooth metal turns specular glints into night bloom flares: kit metals keep at least roughness .45.
              if (m.isMeshStandardMaterial && m.metalness > .5) m.roughness = Math.max(m.roughness, .45);
              materials.set(name, m); matSet.add(m);
            }
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
  function clearGroup(group) {
    group.traverse(n => { if (n.isInstancedMesh) n.dispose(); });
    group.clear();
  }
  async function rebuild() {
    const token = ++generation, config = tiers[tier], lod = config.lod;
    const ids = [...new Set([...placements.map(p => p.asset), ...districts.map(d => d.asset), 'service-drone'])];
    const kitIds = [...new Set(kitPlacements.map(p => p.asset))].filter(id => catalog3.has(id));
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
      const kit = new Map(await Promise.all(kitIds.map(async id => [id, await model(id, lod, 'v3')])));
      if (disposed || token !== generation) return;
      clearGroup(staticCity); clearGroup(landmarks); objects = [];
      const batches = new Map();
      const add = (p, template) => {
        template.updateMatrixWorld(true);
        const matrix = new THREE.Matrix4().compose(new THREE.Vector3(p.x, p.y, p.z),
          new THREE.Quaternion().setFromAxisAngle(new THREE.Vector3(0, 1, 0), p.angle), new THREE.Vector3(...p.scale));
        template.traverse(n => {
          if (!n.isMesh) return;
          if (!batches.has(n.uuid)) batches.set(n.uuid, { node: n, matrices: [], near: NEAR.has(p.asset) });
          batches.get(n.uuid).matrices.push(new THREE.Matrix4().multiplyMatrices(matrix, n.matrixWorld));
        });
      };
      for (const p of placements) add(p, (p.z < -100 ? distant : loaded).get(p.asset));
      for (const p of kitPlacements) if (kit.has(p.asset)) add(p, kit.get(p.asset));
      for (const { node, matrices, near } of batches.values()) {
        const mesh = new THREE.InstancedMesh(node.geometry, node.material, matrices.length);
        matrices.forEach((m, i) => mesh.setMatrixAt(i, m));
        mesh.castShadow = true; mesh.receiveShadow = true; mesh.userData.near = near; staticCity.add(mesh);
      }
      nearHidden = null;
      for (const d of districts) {
        const mesh = loaded.get(d.asset).clone(true); mesh.position.set(d.x, 0, d.z);
        mesh.userData.district = d.id; landmarks.add(mesh); objects.push(mesh);
      }
      life?.attachLandmarks(landmarks); drones.setTemplate(loaded.get('service-drone'));
      const box = new THREE.Box3(), roofs = new Map();
      for (const id of ['data-tower-a','data-tower-b']) roofs.set(id, box.setFromObject(distant.get(id)).max.y);
      atmosphere.setAviation(placements.filter(p => roofs.has(p.asset) && (p.z < -100 || p.scale[1] > 1.1)).map(p => ({ x: p.x, y: p.y + roofs.get(p.asset) * p.scale[1] + .6, z: p.z })));
      renderer.shadowMap.needsUpdate = true; options.onReady?.();
    } catch (e) { if (!disposed && token === generation) options.onError?.(e); }
  }
  function resize() {
    if (disposed) return;
    width = Math.max(1, host.clientWidth); height = Math.max(1, host.clientHeight);
    const dpr = Math.min(devicePixelRatio || 1, tiers[tier].dpr, Math.sqrt(3840 * 2160 / (width * height)));
    renderer.setPixelRatio(dpr); renderer.setSize(width, height, false); composer.setPixelRatio(dpr); composer.setSize(width, height);
    camera.aspect = width / height; camera.updateProjectionMatrix();
    hologram.setPointScale(height * dpr); atmosphere.setPointScale(height * dpr); flow.setPointScale(height * dpr);
  }
  function setQuality(value) {
    quality = value in tiers || value === 'auto' ? value : 'auto';
    tier = quality === 'auto' ? 'high' : quality;
    applyTier(); return rebuild();
  }
  function applyTier() {
    const config = tiers[tier];
    renderer.shadowMap.enabled = config.shadow > 0;
    if (config.shadow && sun.shadow.mapSize.x !== config.shadow) {
      sun.shadow.mapSize.set(config.shadow, config.shadow);
      sun.shadow.map?.dispose(); sun.shadow.map = null;
    }
    renderer.shadowMap.needsUpdate = true; bloom.enabled = config.bloom; surfaces.setTier(tier); windows.setTier(tier); lobbyRender.setTier(tier); atmosphere.setTier(tier);experience.setTier(tier);weather.setTier(tier);drones.setTier(tier);flow.setTier(tier);
    resize(); options.onQuality?.(quality, tier);
  }
  function flyTo(position, target) {
    flight=null;
    const safe=traffic.findFree(visitor,position);if(!safe)return;
    position=new THREE.Vector3(safe.x,safe.y,safe.z);
    if (reduced) { camera.position.copy(position);Object.assign(visitor,safe); controls.target.copy(target); controls.update(); flight = null; return; }
    const free=(a,b)=>traffic.clear(visitor,{...a,heading:0},{...b,heading:0},false),start=camera.position.clone(),altitude=Math.max(start.y,position.y,110);
    let waypoints=[start,position];
    if(!free(start,position)){
      const nodes=[start],aboveEnd=new THREE.Vector3(position.x,altitude,position.z),previous=[-1],queue=[0];let exit=-1;
      if(!free(aboveEnd,position))return;
      for(const radius of [3,8,16])for(let i=0;i<8;i++)nodes.push(new THREE.Vector3(start.x+Math.cos(i*Math.PI/4)*radius,start.y,start.z+Math.sin(i*Math.PI/4)*radius));
      for(const r of interiors)if(Math.hypot(start.x-r.x,start.z-r.z)<24)for(const z of [r.z,r.doorZ-r.front*2,r.doorZ+r.front*3])nodes.push(new THREE.Vector3(r.x,start.y,z));
      while(queue.length){
        const current=queue.shift(),p=nodes[current],above=new THREE.Vector3(p.x,altitude,p.z);
        if(free(p,above)&&free(above,aboveEnd)){exit=current;break;}
        for(let i=1;i<nodes.length;i++)if(previous[i]===undefined&&p.distanceTo(nodes[i])<=20&&free(p,nodes[i])){previous[i]=current;queue.push(i);}
      }
      if(exit<0)return;
      waypoints=[];for(let i=exit;i>=0;i=previous[i])waypoints.unshift(nodes[i]);
      waypoints.push(new THREE.Vector3(nodes[exit].x,altitude,nodes[exit].z),aboveEnd,position);
    }
    flight = { start: camera.position.clone(), targetStart: controls.target.clone(), end: position.clone(), target: target.clone(), time: 0,
      waypoints };
  }
  function settleCamera(){traffic.relocate(visitor,camera.position);camera.position.set(visitor.x,visitor.y,visitor.z);}
  function focus(id) {
    const d = districts.find(d => d.id === id); if (!d) return;
    selected = id; selection.position.set(d.x, .55, d.z); selection.scale.setScalar(d.radius * 1.3); selection.visible = true; tintSelection();
    // The ripple marks the change of selection without touching the camera.
    ripple.position.set(d.x, .62, d.z); ripple.scale.setScalar(d.radius * 1.3 * .35);
    rippleMaterial.uniforms.color.value.setHex(stateColours[districtState[id]] ?? 0x8ee8ee);
    rippleMaterial.uniforms.alpha.value = 0; rippleTime = 0; ripple.visible = !reduced;
    hoverRing.visible = !!hovered && hovered !== selected;
    if (mode === 'street') {
      experience.endRide(); options.onCut?.(); camera.position.set(d.x, 2.4 + groundHeight(d.x, d.z + d.radius + 7), d.z + d.radius + 7); camera.lookAt(d.x, d.height * .4, d.z);
      settleCamera();
    } else {
      flyTo(new THREE.Vector3(d.x + d.radius * 2.7, d.height * .7 + 18, d.z + d.radius * 4),
        new THREE.Vector3(d.x, d.height * .4, d.z));
    }
  }
  function setMode(value) {
    experience.endRide();
    experience.society.suspend();
    keys.clear(); flight = null; mode = value; controls.enabled = mode === 'orbit' || mode === 'tour';
    visitor.minY=mode==='street'?-2.3:-.4;visitor.ignore=visitor.follow=null;
    visitor.circles[0].r=visitor.reach=mode==='street'?.24:.38;
    if (document.pointerLockElement === canvas) document.exitPointerLock();
    if (mode === 'street') {
      options.onCut?.(); clearHover(); camera.position.set(18, 2.4 + groundHeight(18, 57), 57); camera.lookAt(0, 26, -12);
      settleCamera();
    } else if (mode !== 'map') {
      settleCamera();
      flyTo(home.clone().multiplyScalar(camera.aspect < 1 ? 1.3 : 1), homeTarget);
    }
    tourTime = 0; tourIndex = 0; options.onMode?.(mode);
  }
  const cancelTour = () => { flight=null;if (mode === 'tour') { mode = 'orbit'; options.onMode?.(mode); } };
  controls.addEventListener('start', cancelTour);
  function listen(node, event, fn, config) { node.addEventListener(event, fn, config); cleanup.push(() => node.removeEventListener(event, fn, config)); }
  let down = null, dragging = false;
  const euler = new THREE.Euler(0, 0, 0, 'YXZ');
  function look(dx, dy) {
    const k = .0025 * camera.fov / 43;
    euler.setFromQuaternion(camera.quaternion); euler.y -= dx * k; euler.x = THREE.MathUtils.clamp(euler.x - dy * k, -1.35, 1.35);
    camera.quaternion.setFromEuler(euler);
  }
  listen(canvas, 'pointerdown', e => { canvas.focus({ preventScroll: true }); cancelTour(); down = [e.clientX, e.clientY]; dragging = false; if (mode === 'street') canvas.setPointerCapture(e.pointerId); });
  let hovered = null, hoverAt = -1e9;
  function setHover(id) {
    if (id === hovered) return; hovered = id; canvas.style.cursor = id ? 'pointer' : '';
    const d = districts.find(d => d.id === id); hoverRing.visible = !!d && id !== selected;
    if (d) { hoverRing.position.set(d.x, .56, d.z); hoverRing.scale.setScalar(d.radius * 1.3); }
    options.onHover?.(id);
  }
  function clearHover() { setHover(null); }
  function hover(e) {
    if (mode === 'street' || mode === 'map' || down || e.timeStamp - hoverAt < 80) return;
    hoverAt = e.timeStamp; const rect = canvas.getBoundingClientRect();
    pointer.set((e.clientX - rect.left) / rect.width * 2 - 1, 1 - (e.clientY - rect.top) / rect.height * 2);
    ray.setFromCamera(pointer, camera); let obj = ray.intersectObjects(objects, true)[0]?.object;
    while (obj && !obj.userData.district) obj = obj.parent;
    setHover(obj?.userData.district || null);
  }
  listen(canvas, 'pointerleave', clearHover);
  listen(canvas, 'pointermove', e => {
    if (mode === 'street' && (document.pointerLockElement === canvas || down)) look(e.movementX, e.movementY);
    hover(e);
    if (down && Math.hypot(e.clientX-down[0],e.clientY-down[1]) > 5) dragging = true;
  });
  listen(canvas, 'pointerup', e => {
    if (down && !dragging && mode !== 'street') {
      const rect = canvas.getBoundingClientRect(); pointer.set((e.clientX-rect.left)/rect.width*2-1,1-(e.clientY-rect.top)/rect.height*2);
      ray.setFromCamera(pointer, camera); const hit = ray.intersectObjects(objects, true)[0];
      if (hit) { let obj = hit.object; while (obj && !obj.userData.district) obj = obj.parent; if(obj) options.onSelect?.(obj.userData.district); }
    }
    down = null;
  });
  listen(canvas, 'pointercancel', () => { down = null; keys.clear(); });
  listen(canvas, 'keydown', e => {
    if (e.key === 'Escape') { if (experience.scoping()) experience.endRide(); else setMode('orbit'); e.preventDefault(); return; }
    if (mode === 'street' && ['+', '=', '-'].includes(e.key) && experience.zoom(e.key === '-' ? 240 : -240)) { e.preventDefault(); return; }
    if(mode==='street'&&e.code==='KeyE'&&!e.repeat){experience.interact();e.preventDefault();return;}
    if (mode === 'street' && ['KeyW','KeyA','KeyS','KeyD','ArrowUp','ArrowDown','ArrowLeft','ArrowRight','ShiftLeft'].includes(e.code)) {
      keys.add(e.code); e.preventDefault(); e.stopPropagation();
    }
    // Hover pack: Space climbs and C descends; shortcuts such as Ctrl+C keep their meaning.
    if (mode === 'street' && ['Space','KeyC'].includes(e.code) && !e.ctrlKey && !e.metaKey && !e.altKey) {
      keys.add(e.code); e.preventDefault(); e.stopPropagation();
    }
  });
  listen(canvas, 'keyup', e => keys.delete(e.code));
  listen(canvas, 'wheel', e => { if (mode === 'street' && experience.zoom(e.deltaY)) e.preventDefault(); }, { passive: false });
  listen(canvas, 'blur', () => keys.clear());
  listen(window, 'blur', () => { keys.clear(); down = null; });
  listen(canvas, 'webglcontextlost', e => { e.preventDefault(); failed = true; keys.clear(); options.onContextLost?.(); });
  observer = new ResizeObserver(resize); observer.observe(host);
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
  let simulationTime=0;
  const simulate=fixedSteps(step=>{
    traffic.begin();simulationTime+=reduced?0:step;
    if(mode==='street')move(step);
    experience.update(step,!reduced,mode);
    const previousCarrier=visitor.follow;visitor.follow=experience.rideBody();visitor.ignore=visitor.follow||previousCarrier;
    life?.update(step,!reduced);drones.update(step,simulationTime,!reduced);
    traffic.propose(visitor,{...camera.position,heading:0},(_,p)=>camera.position.set(p.x,p.y,p.z));
    traffic.solve(step);experience.settle(reduced?0:step);experience.syncRide?.();
    if(visitor.follow)Object.assign(visitor,{x:camera.position.x,y:camera.position.y,z:camera.position.z});
    visitor.ignore=visitor.follow;
  });
  function update(dt, elapsed) {
    if (disposed || !visible || failed || mode === 'map') return;
    let travelling=null,flightTime=0,requested=null;
    if (mode !== 'street') {
      if (mode === 'tour') {
        tourTime -= dt;
        if (tourTime <= 0) { const id = districts[tourIndex++ % districts.length].id; focus(id); options.onTourFocus?.(id); tourTime = 7; }
      }
      if (flight) {
        travelling=flight;flightTime=flight.time;
        flight.time += dt; const p = Math.min(1, flight.time / (flight.duration || (flight.waypoints.length===2?1.1:3.2))), eased = p*p*(3-2*p);
        const cursor=Math.min(flight.waypoints.length-1-.000001,eased*(flight.waypoints.length-1)),index=Math.floor(cursor);
        camera.position.lerpVectors(flight.waypoints[index],flight.waypoints[index+1],cursor-index); controls.target.lerpVectors(flight.targetStart, flight.target, eased);
        camera.lookAt(controls.target);requested=camera.position.clone();
        if (p === 1) flight = null;
      }else controls.update();
    }
    if(!simulate(Math.min(.1,Math.max(0,dt)))){traffic.begin();traffic.propose(visitor,{...camera.position,heading:0},(_,p)=>camera.position.set(p.x,p.y,p.z));traffic.solve(0);}
    if (lobbyRender.update(camera.position, landmarks)) renderer.shadowMap.needsUpdate = true;
    // Small furniture (NEAR) is sub-pixel from the orbit views: hidden above NEAR_HEIGHT, fewer draw calls.
    const far = camera.position.y > NEAR_HEIGHT;
    if (far !== nearHidden) { nearHidden = far; staticCity.traverse(n => { if (n.userData.near) n.visible = !far; }); renderer.shadowMap.needsUpdate = true; }
    if(travelling){
      if(camera.position.distanceToSquared(requested)>.000001){travelling.wait=(travelling.wait||0)+dt;travelling.time=flightTime;flight=travelling.wait<6?travelling:null;}
      else travelling.wait=0;
    }
    camera.getWorldDirection(v);
    // The sound layer listens from where the view rests: on the street you stand
    // on, or on the district the overview is centred on. Measuring from the
    // camera itself would put every resident hundreds of metres away.
    const overhead=mode!=='street'&&camera.position.y>15;
    options.onListener?.(camera.position.x,camera.position.y,camera.position.z,v.x,v.z,
      overhead?controls.target.x:camera.position.x,overhead?controls.target.z:camera.position.z);
    const busy = !!options.busy?.();
    if(weather.update(dt,elapsed,!reduced))renderer.shadowMap.needsUpdate=true;
    // A dim idle glow keeps the spire core from going pitch black between jobs;
    // the busy pulse stays the readable signal, reduced motion keeps both off.
    reactorLight.intensity = reduced ? 0 : (busy ? 180 + Math.sin(elapsed*2)*35 : 12);
    atmosphere.setBusy(busy); atmosphere.setCinematic(mode === 'tour'); atmosphere.update(dt, elapsed, camera, !reduced); surfaces.update(dt, !reduced);
    companion?.update(dt, !reduced && visible && mode !== 'map' && !options.replaying?.());
    const mood = weather.mood();
    flow.update(dt,!reduced,1-(mood.day||0));
    // Window interiors: lit share, lobbies and glass sheen follow the time of day; no motion freezes them.
    windows.setLighting(mood.day, mood.evening, mood.weather === 'clear' ? 0 : 1, scene.fog.color); lobbyRender.setLighting(mood.day); windows.update(dt, !reduced);
    if(!reduced){
      // The selection ripple opens out from the district and fades away.
      if(rippleTime<1.4){rippleTime+=dt;const p=Math.min(1,rippleTime/1.4);
        const d=districts.find(d=>d.id===selected);
        if(d){ripple.scale.setScalar(d.radius*1.3*(.35+p*1.05));rippleMaterial.uniforms.alpha.value=(1-p)*.55;}
        if(p>=1)ripple.visible=false;
      }
      // The district beacons breathe so the skyline never looks frozen.
      for(let i=0;i<districts.length;i++){
        const d=districts[i];scratch.position.set(d.x,d.height+2,d.z);
        scratch.scale.setScalar(1+Math.sin(elapsed*1.7+i*1.13)*.13);
        scratch.updateMatrix();beacons.setMatrixAt(i,scratch.matrix);
      }
      beacons.instanceMatrix.needsUpdate=true;
    }
    if (!reduced) ringTime.value += Math.min(.1, Math.max(0, dt));
    const key = [busy, mood.day.toFixed(1), mood.evening.toFixed(1), mood.weather].join();
    bloom.strength = .2 + .15 * (1 - mood.day); bloom.threshold = 1.06 + .26 * THREE.MathUtils.smoothstep(mood.day, 0, 1); renderer.toneMappingExposure = .95 + .05 * (1 - mood.day);
    if (key !== moodKey) { moodKey = key; options.onMood?.({ busy, day: mood.day, evening: mood.evening, weather: mood.weather }); }
    hologram.update(dt, camera, !reduced);
    renderer.info.reset(); composer.render(); frames++;
    if (quality === 'auto' && dt > 0 && dt < .2 && elapsed - lastQualityChange > 12) {
      measured += dt; sampleFrames++;
      if (sampleFrames >= 180) {
        const avg = measured / sampleFrames, order = ['low','medium','high'], index = order.indexOf(tier);
        const next = avg > .028 && index > 0 ? order[index-1] : avg < .017 && index < 2 ? order[index+1] : tier;
        sampleFrames = 0; measured = 0;
        if(next !== tier) { tier = next; lastQualityChange = elapsed; applyTier(); void rebuild(); }
      }
    }
  }
  function setData(entities, events) {
    life?.setData(entities, events);
    districts.forEach((d, i) => {
      const e = entities.find(e => e.id === d.id);
      districtState[d.id] = !e || e.stale ? 'stale' : e.state === 'error' ? 'error' : e.state === 'running' ? 'running' : 'idle';
      beacons.setColorAt(i, new THREE.Color(stateColours[districtState[d.id]]));
    }); beacons.instanceColor.needsUpdate = true; tintSelection();
  }
  function dispose() {
    if (disposed) return; disposed = true; generation++; abort.abort(); requests.forEach(c => c.abort());
    keys.clear(); if(document.pointerLockElement === canvas) document.exitPointerLock();
    options.signal?.removeEventListener('abort', dispose); cleanup.forEach(fn => fn()); observer?.disconnect(); controls.dispose();
    life?.dispose();companion?.dispose();experience?.dispose();weather.dispose(); hologram.dispose(); drones.dispose();contact.dispose();traffic.dispose(); atmosphere.dispose(); surfaces.dispose(); windows.dispose(); lobbyRender.dispose(); flow.dispose(); clearGroup(staticCity); clearGroup(landmarks); beacons.dispose(); plots.dispose(); groundCover.forEach(m => m.dispose());
    geoSet.forEach(g => g.dispose()); matSet.forEach(m => m.dispose());
    composer.passes.forEach(p => p.dispose?.()); composer.dispose(); environment.dispose(); sun.shadow.dispose();
    renderer.dispose(); renderer.forceContextLoss(); canvas.remove(); cache.clear(); materials.clear();
  }
  life = createCityLife(scene, districts, {traffic,society:experience.society,robotURL:options.resourceURL('/3d/system-world/white-robot.glb'), signal:options.signal, onError:options.onRobotError, active:()=>visible&&!failed&&mode!=='map'&&!options.replaying?.(), obstacles, shadows:contact});
  // A missing or broken companion never stops the city from loading.
  companion = createCompanion({scene, resourceURL:options.resourceURL, signal:abort.signal, shadows:contact, onError:options.onCompanionError});
  hologram.setReducedMotion(!!reduced); windows.setReducedMotion(!!reduced);
  try { applyTier(); await rebuild(); } catch(e) { dispose(); throw e; }
  return {
    districts, canvas, update, focus(id) { cancelTour(); focus(id); }, setMode, setQuality, setData, dispose,
    interact(){experience.interact();},socialAction:experience.socialAction,
    visit(id){setMode('street');experience.visit(id);settleCamera();},enter(id){setMode('street');experience.destination(id);settleCamera();},
    setEnvironment(value){weather.set(value);renderer.shadowMap.needsUpdate=true;},
    setVisible(value) { visible = value; if(!value) { simulate(0);experience.suspend();life?.update(0,false);keys.clear(); down = null; if(document.pointerLockElement===canvas) document.exitPointerLock(); } },
    setReducedMotion(value) { reduced = value; hologram.setReducedMotion(!!value); windows.setReducedMotion(!!value);if(value){flight=null;experience.endRide();experience.society.suspend();} if(value && mode === 'tour') setMode('orbit'); },
    setHologram(texts, source) { hologram.setTexts(texts, source); experience.setMemory(texts); },
    setWorld(snapshot,replay) { experience.setWorld(snapshot,replay); },
    moveKey(key, pressed) { if(pressed) keys.add(key); else keys.delete(key); },
    lockPointer() { if(mode === 'street') return canvas.requestPointerLock(); },
    project(id) { const d = districts.find(d => d.id === id); if(!d) return null; v.set(d.x,d.height+4,d.z); const distance = camera.position.distanceTo(v); v.project(camera); return { x:(v.x+1)*width/2,y:(1-v.y)*height/2,visible:v.z<1 && v.z>-1,distance }; },
    // The frame is read in the same task as the render, so no preserved drawing buffer is needed.
    capture() { if (disposed || failed) return Promise.resolve(null); composer.render(); return new Promise(resolve => canvas.toBlob(resolve, 'image/png')); },
    // Opening shot on first display: a wide glide into the home view; any input cancels it.
    intro() {
      if (reduced || mode !== 'orbit' || disposed) return;
      const end = home.clone().multiplyScalar(camera.aspect < 1 ? 1.3 : 1);
      camera.position.copy(end).sub(homeTarget).applyAxisAngle(new THREE.Vector3(0, 1, 0), -.55).multiplyScalar(1.22).add(homeTarget); camera.position.y += 34;
      camera.lookAt(homeTarget); settleCamera(); flyTo(end, homeTarget); if (flight) flight.duration = 3.6;
    },
    stats() { return { flying:!!flight,traffic:traffic.stats(),experience:experience.stats(),weather:weather.stats(),life:life?.stats(), hologram:hologram.stats(), atmosphere:atmosphere.stats(), surfaces:surfaces.stats(), windows:windows.stats(), lobbies:lobbyRender.stats(), furniture:kitPlacements.length-streetPlacements.length,nearHidden,drones:drones.stats(), flow:flow.stats(), companion:companion?.stats(), frames, tier, mode, focusedDistrict: selected, hovered, loadedBytes:loadBytes+experience.stats().bytes+surfaces.stats().bytes, cachedModels:cache.size,kitModels:{v1:[...cache.keys()].filter(k=>k.startsWith('v1:')).length,v3:[...cache.keys()].filter(k=>k.startsWith('v3:')).length}, calls:renderer.info.render.calls, triangles:renderer.info.render.triangles, geometries:renderer.info.memory.geometries, position:camera.position.toArray(), renderer:renderer.getContext().getParameter(renderer.getContext().getExtension('WEBGL_debug_renderer_info')?.UNMASKED_RENDERER_WEBGL || renderer.getContext().RENDERER), disposed }; },
  };
}
