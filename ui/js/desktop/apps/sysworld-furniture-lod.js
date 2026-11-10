// Furniture near and far instance sets (System World beauty pass, phase 6). Each furniture design keeps two instanced
// sets: pieces within FURNITURE_LOD.near metres of the camera (they leave again beyond .far) draw at the tier's level of
// detail, the rest at LOD .level. sync() re-sorts at most every .interval seconds and only after the camera moved .move
// metres; it copies precomputed matrices into fixed-size instance buffers and allocates nothing. A re-sort does not
// refresh the shadow map: a piece keeps its place and footprint whichever level draws it. Visibility belongs to the
// scene's near gate (an empty set has count 0). Geometries and materials belong to the scene's model cache; dispose()
// releases only the instanced meshes.
import * as THREE from 'three';

export const FURNITURE_LOD = Object.freeze({ near: 45, far: 60, interval: .5, move: 4, level: 2, worth: 8000 });
export function nearFlag(distance, wasNear, lod = FURNITURE_LOD) { return wasNear ? distance <= lod.far : distance < lod.near; }

// A design gets near and far sets only when drawing it at LOD .level saves at least .worth triangles city-wide; every
// set doubles that design's draw calls, which cost more than a small saving gives back.
export function worthSets(pieces, tierTriangles, farTriangles, lod = FURNITURE_LOD) { return pieces * (tierTriangles - farTriangles) >= lod.worth; }
// counts: Map design -> piece count; catalog: Map design -> {lods:[{level,triangles}]}. Returns the Set of designs with sets.
export function setDesigns(counts, catalog, level, lod = FURNITURE_LOD) {
  const out = new Set();
  if (level >= lod.level) return out;
  for (const [id, n] of counts) {
    const lods = catalog.get(id)?.lods, tier = lods?.find(l => l.level === level)?.triangles, far = lods?.find(l => l.level === lod.level)?.triangles;
    if (tier !== undefined && far !== undefined && worthSets(n, tier, far, lod)) out.add(id);
  }
  return out;
}

export function createFurnitureSets({ pieces, near, far, isNear = () => false, lod = FURNITURE_LOD }) {
  const groups = [], meshes = [], placement = new THREE.Matrix4(), part = new THREE.Matrix4(), position = new THREE.Vector3(),
    turn = new THREE.Quaternion(), scale = new THREE.Vector3(), up = new THREE.Vector3(0, 1, 0), byAsset = new Map();
  for (const p of pieces) if (near.has(p.asset) && far.has(p.asset)) { if (!byAsset.has(p.asset)) byAsset.set(p.asset, []); byAsset.get(p.asset).push(p); }
  function build(template, list, kind, asset) {
    template.updateMatrixWorld(true);
    const out = [];
    template.traverse(node => {
      if (!node.isMesh) return;
      const mesh = new THREE.InstancedMesh(node.geometry, node.material, list.length), source = new Float32Array(list.length * 16);
      list.forEach((p, i) => {
        placement.compose(position.set(p.x, p.y || 0, p.z), turn.setFromAxisAngle(up, p.angle || 0), scale.fromArray(p.scale || [1, 1, 1]));
        part.multiplyMatrices(placement, node.matrixWorld).toArray(source, i * 16);
      });
      mesh.count = 0; mesh.castShadow = true; mesh.receiveShadow = true;
      Object.assign(mesh.userData, { near: !!isNear(asset), furniture: kind, asset });
      meshes.push(mesh); out.push({ mesh, source });
    });
    return out;
  }
  for (const [asset, list] of byAsset) groups.push({ list, flags: new Uint8Array(list.length), near: build(near.get(asset), list, 'near', asset), far: build(far.get(asset), list, 'far', asset) });
  const total = groups.reduce((n, g) => n + g.list.length, 0);
  let lastTime = -Infinity, lastX = NaN, lastZ = NaN, sorts = 0, nearCount = 0;
  function write(entries, list, flags, want) {
    for (const { mesh, source } of entries) {
      const target = mesh.instanceMatrix.array; let k = 0;
      for (let i = 0; i < list.length; i++) if (flags[i] === want) { for (let j = 0; j < 16; j++) target[k * 16 + j] = source[i * 16 + j]; k++; }
      mesh.count = k; mesh.instanceMatrix.needsUpdate = true;
      if (k) mesh.computeBoundingSphere();
    }
  }
  function sync(camera, time, force = false) {
    if (!force && (time - lastTime < lod.interval || Math.hypot(camera.x - lastX, camera.z - lastZ) < lod.move)) return false;
    lastTime = time; lastX = camera.x; lastZ = camera.z;
    let changed = force; nearCount = 0;
    for (const g of groups) for (let i = 0; i < g.list.length; i++) {
      const p = g.list[i], flag = nearFlag(Math.hypot(p.x - camera.x, p.z - camera.z), g.flags[i] === 1, lod) ? 1 : 0;
      if (flag !== g.flags[i]) { g.flags[i] = flag; changed = true; }
      nearCount += flag;
    }
    if (!changed) return false;
    for (const g of groups) { write(g.near, g.list, g.flags, 1); write(g.far, g.list, g.flags, 0); }
    sorts++; return true;
  }
  return {
    meshes, sync,
    stats: () => ({ designs: groups.length, pieces: total, near: nearCount, far: total - nearCount, sorts, meshes: meshes.length }),
    dispose() { for (const m of meshes) { m.removeFromParent(); m.dispose(); } meshes.length = 0; groups.length = 0; },
  };
}
