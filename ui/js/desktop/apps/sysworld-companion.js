import * as THREE from 'three';
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';
import { MeshoptDecoder } from 'three/addons/libs/meshopt_decoder.module.js';
// A decorative quadruped patrols the quiet waterfront beside the quay lanterns.
// The GLB is a meshopt-compressed derivative of the pinned BSD-3-Clause
// MuJoCo Menagerie Spot (see assets/system-world/spot-source.json). It is
// scenery, not an actor: it never joins traffic, telemetry or the robot foley.
// One bounded loop, one accumulated clock, no per-frame allocations.

const LOOP = { cx: 20, cz: 69.8, rx: 9, rz: 2.0, seconds: 24, gaitHz: 1.7 };
const JOINTS = ['body', 'fl_hip', 'fr_hip', 'hl_hip', 'hr_hip',
  'fl_uleg', 'fr_uleg', 'hl_uleg', 'hr_uleg',
  'fl_lleg', 'fr_lleg', 'hl_lleg', 'hr_lleg'];
// Diagonal pairs move together: a quiet trot, never a gallop or a spin.
const TROT = { fl: 0, hr: 0, fr: Math.PI, hl: Math.PI };

// Pure and deterministic: the acceptance check can replay any point in time.
export function companionPose(time) {
  const a = (time / LOOP.seconds) * Math.PI * 2;
  const x = LOOP.cx + LOOP.rx * Math.cos(a), z = LOOP.cz + LOOP.rz * Math.sin(a);
  const dx = -LOOP.rx * Math.sin(a), dz = LOOP.rz * Math.cos(a);
  // The kit's forward axis is +X; a Y rotation maps +X to (cos, 0, -sin).
  return { x, z, heading: Math.atan2(-dz, dx), phase: time * LOOP.gaitHz * Math.PI * 2 };
}

// Diagonal pairs share a phase, opposite pairs are half a cycle apart. Pure so
// the trot can be asserted without a renderer.
export function companionGait(phase) {
  const out = {};
  for (const [key, offset] of Object.entries(TROT))
    out[key] = { hip: .26 * Math.sin(phase + offset), uleg: .13 * Math.sin(phase + offset + .9) };
  return out;
}

export function createCompanion({ scene, resourceURL, signal, groundY = 0, onError }) {
  const group = new THREE.Group();
  group.visible = false;
  scene.add(group);
  const joints = Object.create(null);
  const box = new THREE.Box3();
  let model = null, baseY = 0, time = 0, failed = false, disposed = false;
  const loader = new GLTFLoader();
  loader.setMeshoptDecoder(MeshoptDecoder);

  const ready = (async () => {
    try {
      await MeshoptDecoder.ready;
      const response = await fetch(resourceURL('/3d/system-world/spot.glb'), { signal });
      if (!response.ok) throw Error('Companion asset unavailable: ' + response.status);
      const buffer = await response.arrayBuffer();
      if (disposed) return false;
      const gltf = await loader.parseAsync(buffer, '');
      if (disposed) return false;
      model = gltf.scene;
      model.traverse(n => { if (n.isMesh) { n.castShadow = true; n.receiveShadow = true; } });
      for (const name of JOINTS) joints[name] = model.getObjectByName(name) || null;
      if (!joints.body) throw Error('Companion joint hierarchy missing');
      // Place the feet on the quay regardless of how the exporter baked pivots.
      box.setFromObject(model);
      baseY = groundY - box.min.y;
      group.add(model);
      group.visible = true;
      return true;
    } catch (e) {
      failed = true;
      onError?.(e);
      return false;
    }
  })();

  function update(dt, animated) {
    if (!model || disposed) return;
    if (animated) time += Math.min(.1, Math.max(0, dt));
    const pose = companionPose(time);
    group.position.set(pose.x, baseY + Math.abs(Math.sin(pose.phase)) * .012, pose.z);
    group.rotation.y = pose.heading;
    for (const [key, gait] of Object.entries(companionGait(pose.phase))) {
      if (joints[key + '_hip']) joints[key + '_hip'].rotation.y = gait.hip;
      if (joints[key + '_uleg']) joints[key + '_uleg'].rotation.y = gait.uleg;
    }
  }

  return {
    ready, update,
    stats: () => ({ loaded: !!model, failed, time: +time.toFixed(2), joints: Object.keys(joints).filter(k => joints[k]).length,
      position: [+group.position.x.toFixed(2), +group.position.z.toFixed(2)] }),
    dispose() {
      if (disposed) return;
      disposed = true;
      if (model) model.traverse(n => { if (n.isMesh) { n.geometry.dispose(); for (const m of (Array.isArray(n.material) ? n.material : [n.material])) m.dispose(); } });
      group.removeFromParent();
      group.clear();
    },
  };
}
