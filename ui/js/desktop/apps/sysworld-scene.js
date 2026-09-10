import * as THREE from 'three';
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { RoomEnvironment } from 'three/addons/environments/RoomEnvironment.js';
import { EffectComposer } from 'three/addons/postprocessing/EffectComposer.js';
import { RenderPass } from 'three/addons/postprocessing/RenderPass.js';
import { UnrealBloomPass } from 'three/addons/postprocessing/UnrealBloomPass.js';
import { OutputPass } from 'three/addons/postprocessing/OutputPass.js';
import { createCityLife } from './sysworld-life.js';
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
const homeTarget = new THREE.Vector3(0, 22, -12);

export async function createCity(host, options) {
  let disposed = false, visible = true, mode = 'orbit', quality = options.quality || 'auto';
  let tier = quality === 'auto' ? 'high' : quality, generation = 0, flight = null;
  let frames = 0, measured = 0, sampleFrames = 0, lastQualityChange = 0, loadBytes = 0;
  let tourTime = 0, tourIndex = 0, selected = null, reduced = options.reducedMotion;
  let width = 1, height = 1, objects = [], failed = false, observer = null, life = null;
  const abort = new AbortController(), requests = new Set(), cache = new Map(), materials = new Map();
  const geoSet = new Set(), matSet = new Set(), keys = new Set(), cleanup = [];
  const renderer = new THREE.WebGLRenderer({ antialias: true, alpha: false });
  renderer.toneMapping = THREE.ACESFilmicToneMapping; renderer.toneMappingExposure = .98;
  renderer.shadowMap.type = THREE.PCFSoftShadowMap;
  renderer.shadowMap.autoUpdate = false; renderer.info.autoReset = false;
  const canvas = renderer.domElement;
  canvas.className = 'sysworld-gl'; canvas.tabIndex = 0;
  canvas.setAttribute('aria-label', options.label);
  host.append(canvas);
  const scene = new THREE.Scene();
  scene.background = new THREE.Color(0x08121e); scene.fog = new THREE.FogExp2(0x08121e, .004);
  const camera = new THREE.PerspectiveCamera(43, 1, .15, 1800);
  camera.position.copy(home);
  const controls = new OrbitControls(camera, canvas);
  controls.target.copy(homeTarget); controls.enableDamping = true; controls.dampingFactor = .085;
  controls.minDistance = 12; controls.maxDistance = 410; controls.maxPolarAngle = Math.PI * .485;
  controls.update();
  const pmrem = new THREE.PMREMGenerator(renderer), room = new RoomEnvironment();
  const environment = pmrem.fromScene(room, .05);
  scene.environment = environment.texture; scene.environmentIntensity = .48;
  room.dispose(); pmrem.dispose();
  scene.add(new THREE.HemisphereLight(0xc0dcff, 0x10202d, .8));
  const sun = new THREE.DirectionalLight(0xc8deff, 3.2);
  sun.position.set(-70, 145, 85); sun.castShadow = true;
  Object.assign(sun.shadow.camera, { left: -120, right: 120, top: 120, bottom: -120, near: 1, far: 360 });
  sun.shadow.normalBias = .09; sun.shadow.bias = -.00015; scene.add(sun);
  const rim = new THREE.DirectionalLight(0xffb572, 2.4); rim.position.set(90, 80, -120); scene.add(rim);
  const reactorLight = new THREE.PointLight(0x62dfff, 0, 75, 2);
  reactorLight.position.set(0, 34, -12); scene.add(reactorLight);
  const renderTarget = new THREE.WebGLRenderTarget(1, 1, { type: THREE.HalfFloatType, samples: Math.min(4, renderer.capabilities.maxSamples) });
  const composer = new EffectComposer(renderer, renderTarget);
  composer.addPass(new RenderPass(scene, camera));
  // HDR threshold selects luminous windows/signals; dark work surfaces never bloom.
  const bloom = new UnrealBloomPass(new THREE.Vector2(1, 1), .24, .5, 1.25);
  composer.addPass(bloom); composer.addPass(new OutputPass());
  const world = new THREE.Group(), staticCity = new THREE.Group(), landmarks = new THREE.Group();
  scene.add(world); world.add(staticCity, landmarks);
  function ownMesh(geometry, material) {
    geoSet.add(geometry); matSet.add(material); return new THREE.Mesh(geometry, material);
  }
  const ground = ownMesh(new THREE.BoxGeometry(170, 3, 174),
    new THREE.MeshStandardMaterial({ color: 0x16242b, metalness: .55, roughness: .38 }));
  ground.position.set(0, -1.5, -7); ground.receiveShadow = true; world.add(ground);
  const sea = ownMesh(new THREE.PlaneGeometry(1800, 1800),
    new THREE.MeshStandardMaterial({ color: 0x07111c, metalness: .75, roughness: .3 }));
  sea.rotation.x = -Math.PI / 2; sea.position.y = -3.2; sea.receiveShadow = true; scene.add(sea);
  // Architectural scale reference: skyline uses the same compact building kit.
  const placements = [], place = (asset, x, z, y = 0, angle = 0, scale = [1, 1, 1]) =>
    placements.push({ asset, x, z, y, angle, scale });
  const xs = [-67, -18, 18, 67], zs = [-77, -32, 13, 59];
  for (const z of zs) {
    for (const x of xs) place('street-crossing', x, z);
    const edges = [-85, ...xs, 85];
    for (let i = 0; i < edges.length - 1; i++) {
      const a = edges[i] + (i ? 6 : 0), b = edges[i + 1] - (i < edges.length - 2 ? 6 : 0);
      place('street-tile', (a + b) / 2, z, 0, 0, [(b - a) / 16, 1, 1]);
    }
  }
  for (const x of xs) {
    const edges = [-94, ...zs, 80];
    for (let i = 0; i < edges.length - 1; i++) {
      const a = edges[i] + (i ? 6 : 0), b = edges[i + 1] - (i < edges.length - 2 ? 6 : 0);
      place('street-tile', x, (a + b) / 2, 0, Math.PI / 2, [(b - a) / 16, 1, 1]);
    }
  }
  for (const z of zs) for (const x of [-55, -31, 31, 55]) {
    place('street-lamp', x, z - 4.5, .45); place('planter', x + 4, z - 4.5, .45);
  }
  for (const [x, z, s] of [[29,-55,1],[49,-55,1.25],[-3,-57,1.2],[4,-73,.75]]) {
    place('data-tower-a', x, z, 0, 0, [1,s,1]);
  }
  place('skybridge', 39, -55, 15);
  place('server-rack', -55, -34, .5); place('server-rack', -49, -34, .5);
  place('data-tram', 33, 59, .5); place('data-tram', -44, -77, .5);
  // Distant buildings are scenery, never presented as additional real entities.
  for (let i = 0; i < 48; i++) {
    const x = (i % 12 - 5.5) * 22, z = -143 - Math.floor(i / 12) * 29;
    const s = .7 + ((i * 17) % 13) / 12;
    place(i % 2 ? 'data-tower-a' : 'data-tower-b', x, z, -3, 0, [.8, s, .8]);
  }
  const selection = ownMesh(new THREE.RingGeometry(1, 1.035, 64),
    new THREE.MeshBasicMaterial({ color: 0x8ee8ee, transparent: true, opacity: .85, depthWrite: false, side: THREE.DoubleSide }));
  selection.rotation.x = -Math.PI / 2; selection.visible = false; world.add(selection);
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
  async function model(id, lod) {
    const key = id + ':' + lod;
    if (!cache.has(key)) cache.set(key, (async () => {
      const item = catalog.get(id)?.lods.find(l => l.level === lod);
      if (!item || !/^[a-z0-9-]+\.lod[0-2]\.glb$/.test(item.file)) throw Error('Invalid city asset');
      const controller = new AbortController(); requests.add(controller);
      const timer = setTimeout(() => controller.abort(), 15000);
      try {
        const response = await fetch(options.assetURL(item.file), { signal: controller.signal });
        if (!response.ok) throw Error('City asset unavailable');
        const bytes = await response.arrayBuffer();
        if (disposed) throw Error('Disposed');
        const gltf = await new GLTFLoader().parseAsync(bytes, '');
        if (disposed) { gltf.scene.traverse(n => { if(n.isMesh) { n.geometry.dispose(); n.material.dispose(); } }); throw Error('Disposed'); }
        loadBytes += bytes.byteLength;
        gltf.scene.traverse(n => {
          if (!n.isMesh) return;
          geoSet.add(n.geometry); n.castShadow = true; n.receiveShadow = true;
          const source = Array.isArray(n.material) ? n.material : [n.material];
          const shared = source.map(m => {
            if (materials.has(m.name)) { const original = m; m = materials.get(m.name); original.dispose(); }
            else { materials.set(m.name, m); matSet.add(m); }
            return m;
          });
          n.material = Array.isArray(n.material) ? shared : shared[0];
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
    const ids = [...new Set([...placements.map(p => p.asset), ...districts.map(d => d.asset)])];
    try {
      const loaded = new Map(await Promise.all(ids.map(async id => [id, await model(id, lod)])));
      // Skyline deliberately uses low LOD even on Ultra.
      const distant = new Map(await Promise.all(['data-tower-a','data-tower-b'].map(async id => [id, await model(id, 2)])));
      if (disposed || token !== generation) return;
      clearGroup(staticCity); clearGroup(landmarks); objects = [];
      const batches = new Map();
      for (const p of placements) {
        const template = (p.z < -100 ? distant : loaded).get(p.asset); template.updateMatrixWorld(true);
        const matrix = new THREE.Matrix4().compose(new THREE.Vector3(p.x, p.y, p.z),
          new THREE.Quaternion().setFromAxisAngle(new THREE.Vector3(0, 1, 0), p.angle), new THREE.Vector3(...p.scale));
        template.traverse(n => {
          if (!n.isMesh) return;
          const key = n.uuid;
          if (!batches.has(key)) batches.set(key, { node: n, matrices: [] });
          batches.get(key).matrices.push(new THREE.Matrix4().multiplyMatrices(matrix, n.matrixWorld));
        });
      }
      for (const { node, matrices } of batches.values()) {
        const mesh = new THREE.InstancedMesh(node.geometry, node.material, matrices.length);
        matrices.forEach((m, i) => mesh.setMatrixAt(i, m));
        mesh.castShadow = true; mesh.receiveShadow = true; staticCity.add(mesh);
      }
      for (const d of districts) {
        const mesh = loaded.get(d.asset).clone(true); mesh.position.set(d.x, 0, d.z);
        mesh.userData.district = d.id; landmarks.add(mesh); objects.push(mesh);
      }
      life?.attachLandmarks(landmarks);
      renderer.shadowMap.needsUpdate = true; options.onReady?.();
    } catch (e) { if (!disposed && token === generation) options.onError?.(e); }
  }
  function resize() {
    if (disposed) return;
    width = Math.max(1, host.clientWidth); height = Math.max(1, host.clientHeight);
    const dpr = Math.min(devicePixelRatio || 1, tiers[tier].dpr, Math.sqrt(3840 * 2160 / (width * height)));
    renderer.setPixelRatio(dpr); renderer.setSize(width, height, false); composer.setPixelRatio(dpr); composer.setSize(width, height);
    camera.aspect = width / height; camera.updateProjectionMatrix();
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
    renderer.shadowMap.needsUpdate = true; bloom.enabled = config.bloom;
    resize(); options.onQuality?.(quality, tier);
  }
  function flyTo(position, target) {
    if (reduced) { camera.position.copy(position); controls.target.copy(target); controls.update(); flight = null; return; }
    flight = { start: camera.position.clone(), targetStart: controls.target.clone(), end: position.clone(), target: target.clone(), time: 0 };
  }
  function focus(id) {
    const d = districts.find(d => d.id === id); if (!d) return;
    selected = id; selection.position.set(d.x, .55, d.z); selection.scale.setScalar(d.radius * 1.3); selection.visible = true;
    if (mode === 'street') {
      camera.position.set(d.x, 2.4, d.z + d.radius + 7); camera.lookAt(d.x, d.height * .4, d.z);
    } else {
      flyTo(new THREE.Vector3(d.x + d.radius * 2.7, d.height * .7 + 18, d.z + d.radius * 4),
        new THREE.Vector3(d.x, d.height * .4, d.z));
    }
  }
  function setMode(value) {
    keys.clear(); flight = null; mode = value; controls.enabled = mode === 'orbit' || mode === 'tour';
    if (document.pointerLockElement === canvas) document.exitPointerLock();
    if (mode === 'street') {
      camera.position.set(18, 2.4, 57); camera.lookAt(0, 26, -12);
    } else if (mode !== 'map') {
      flyTo(home.clone().multiplyScalar(camera.aspect < 1 ? 1.3 : 1), homeTarget);
    }
    tourTime = 0; tourIndex = 0; options.onMode?.(mode);
  }
  const cancelTour = () => { if (mode === 'tour') { mode = 'orbit'; flight = null; options.onMode?.(mode); } };
  controls.addEventListener('start', cancelTour);
  function listen(node, event, fn, config) { node.addEventListener(event, fn, config); cleanup.push(() => node.removeEventListener(event, fn, config)); }
  let down = null, dragging = false;
  const euler = new THREE.Euler(0, 0, 0, 'YXZ');
  function look(dx, dy) {
    euler.setFromQuaternion(camera.quaternion); euler.y -= dx * .0025; euler.x = THREE.MathUtils.clamp(euler.x - dy * .0025, -1.35, 1.35);
    camera.quaternion.setFromEuler(euler);
  }
  listen(canvas, 'pointerdown', e => { canvas.focus({ preventScroll: true }); cancelTour(); down = [e.clientX, e.clientY]; dragging = false; if (mode === 'street') canvas.setPointerCapture(e.pointerId); });
  listen(canvas, 'pointermove', e => {
    if (mode === 'street' && (document.pointerLockElement === canvas || down)) look(e.movementX, e.movementY);
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
    if (e.key === 'Escape') { setMode('orbit'); e.preventDefault(); return; }
    if (mode === 'street' && ['KeyW','KeyA','KeyS','KeyD','ArrowUp','ArrowDown','ArrowLeft','ArrowRight','ShiftLeft'].includes(e.code)) {
      keys.add(e.code); e.preventDefault(); e.stopPropagation();
    }
  });
  listen(canvas, 'keyup', e => keys.delete(e.code));
  listen(canvas, 'blur', () => keys.clear());
  listen(window, 'blur', () => { keys.clear(); down = null; });
  listen(canvas, 'webglcontextlost', e => { e.preventDefault(); failed = true; keys.clear(); options.onContextLost?.(); });
  observer = new ResizeObserver(resize); observer.observe(host);
  function move(dt) {
    const speed = (keys.has('ShiftLeft') ? 25 : 11) * dt;
    let forward = Number(keys.has('KeyW') || keys.has('ArrowUp')) - Number(keys.has('KeyS') || keys.has('ArrowDown'));
    let right = Number(keys.has('KeyD') || keys.has('ArrowRight')) - Number(keys.has('KeyA') || keys.has('ArrowLeft'));
    if (!forward && !right) return;
    const scale = speed / Math.hypot(forward, right);
    camera.getWorldDirection(v); v.y = 0; v.normalize();
    const dx = (v.x * forward - v.z * right) * scale, dz = (v.z * forward + v.x * right) * scale;
    const allowed = (x, z) => Math.abs(x) < 82 && z > -88 && z < 76 &&
      !districts.some(d => Math.abs(x-d.x) < d.radius+1 && Math.abs(z-d.z) < d.radius+1) &&
      !(z < -42 && z > -73 && x > -10 && x < 60);
    if (allowed(camera.position.x + dx, camera.position.z)) camera.position.x += dx;
    if (allowed(camera.position.x, camera.position.z + dz)) camera.position.z += dz;
    camera.position.y = 2.4;
  }
  function update(dt, elapsed) {
    if (disposed || !visible || failed || mode === 'map') return;
    if (mode === 'street') move(Math.min(dt, .05));
    else {
      if (mode === 'tour') {
        tourTime -= dt;
        if (tourTime <= 0) { const id = districts[tourIndex++ % districts.length].id; focus(id); options.onTourFocus?.(id); tourTime = 7; }
      }
      if (flight) {
        flight.time += dt; const p = Math.min(1, flight.time / 1.1), eased = p*p*(3-2*p);
        camera.position.lerpVectors(flight.start, flight.end, eased); controls.target.lerpVectors(flight.targetStart, flight.target, eased);
        if (p === 1) flight = null;
      }
      controls.update();
    }
    camera.getWorldDirection(v);
    options.onListener?.(camera.position.x,camera.position.y,camera.position.z,v.x,v.z);
    reactorLight.intensity = reduced ? 0 : (options.busy?.() ? 180 + Math.sin(elapsed*2)*35 : 0);
    life?.update(dt, !reduced);
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
      beacons.setColorAt(i, new THREE.Color(!e || e.stale ? 0x7b8b9b : e.state === 'error' ? 0xff7868 : e.state === 'running' ? 0x75f4d1 : 0x7cbfe3));
    }); beacons.instanceColor.needsUpdate = true;
  }
  function dispose() {
    if (disposed) return; disposed = true; generation++; abort.abort(); requests.forEach(c => c.abort());
    keys.clear(); if(document.pointerLockElement === canvas) document.exitPointerLock();
    options.signal?.removeEventListener('abort', dispose); cleanup.forEach(fn => fn()); observer?.disconnect(); controls.dispose();
    life?.dispose(); clearGroup(staticCity); clearGroup(landmarks); beacons.dispose();
    geoSet.forEach(g => g.dispose()); matSet.forEach(m => m.dispose());
    composer.passes.forEach(p => p.dispose?.()); composer.dispose(); environment.dispose(); sun.shadow.dispose();
    renderer.dispose(); renderer.forceContextLoss(); canvas.remove(); cache.clear(); materials.clear();
  }
  life = createCityLife(scene, districts, {robotURL:options.resourceURL('/3d/system-world/white-robot.glb'), signal:options.signal, onError:options.onRobotError, active:()=>visible&&!failed&&mode!=='map'});
  try { applyTier(); await rebuild(); } catch(e) { dispose(); throw e; }
  return {
    districts, canvas, update, focus(id) { cancelTour(); focus(id); }, setMode, setQuality, setData, dispose,
    setVisible(value) { visible = value; if(!value) { keys.clear(); down = null; if(document.pointerLockElement===canvas) document.exitPointerLock(); } },
    setReducedMotion(value) { reduced = value; if(value && mode === 'tour') setMode('orbit'); },
    moveKey(key, pressed) { if(pressed) keys.add(key); else keys.delete(key); },
    lockPointer() { if(mode === 'street') return canvas.requestPointerLock(); },
    project(id) { const d = districts.find(d => d.id === id); if(!d) return null; v.set(d.x,d.height+4,d.z).project(camera); return { x:(v.x+1)*width/2,y:(1-v.y)*height/2,visible:v.z<1 && v.z>-1 }; },
    stats() { return { life:life?.stats(), frames, tier, mode, focusedDistrict: selected, loadedBytes:loadBytes, cachedModels:cache.size, calls:renderer.info.render.calls, triangles:renderer.info.render.triangles, geometries:renderer.info.memory.geometries, position:camera.position.toArray(), renderer:renderer.getContext().getParameter(renderer.getContext().getExtension('WEBGL_debug_renderer_info')?.UNMASKED_RENDERER_WEBGL || renderer.getContext().RENDERER), disposed }; },
  };
}
