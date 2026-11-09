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

export function createCompanion({ scene, resourceURL, signal, ground = groundHeight, shadows = null, onError }) {
  const group = new THREE.Group();
  group.visible = false;
  scene.add(group);
  const joints = Object.create(null);
  const box = new THREE.Box3();
  const height = { y: NaN, v: 0 }, pitch = { y: 0, v: 0 }, shadow = shadows?.claim() || null;
  let model = null, footOffset = 0, time = 0, failed = false, disposed = false;
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
      // Feet on the ground regardless of how the exporter baked pivots.
      box.setFromObject(model);
      footOffset = -box.min.y;
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

  return {
    ready, update,
    stats: () => ({ loaded: !!model, failed, time: +time.toFixed(2), joints: Object.keys(joints).filter(k => joints[k]).length,
      position: [+group.position.x.toFixed(2), +group.position.z.toFixed(2)] }),
    dispose() {
      if (disposed) return;
      disposed = true;
      shadow?.hide();
      if (model) model.traverse(n => { if (n.isMesh) { n.geometry.dispose(); for (const m of (Array.isArray(n.material) ? n.material : [n.material])) m.dispose(); } });
      group.removeFromParent();
      group.clear();
    },
  };
}
