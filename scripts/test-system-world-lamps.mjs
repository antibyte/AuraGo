// Street-lamp light cones are a night effect: invisible by day, fading in towards dusk, off on low.
import assert from 'node:assert/strict';
import * as THREE from 'three';
import {createAtmosphere} from '../ui/js/desktop/apps/sysworld-atmosphere.js';

const scene = new THREE.Scene(); scene.fog = new THREE.FogExp2(0x08121e, .004);
const atmosphere = createAtmosphere(scene, {lamps: [{x: 0, z: 0, y: 0, angle: 0}]});
const camera = new THREE.PerspectiveCamera(), up = new THREE.Vector3(0, 1, 0);
const cones = scene.getObjectByName('city-atmosphere').children.find(n => n.isInstancedMesh && n.geometry.type === 'ConeGeometry');
assert.ok(cones, 'lamp cones exist');
// The cones must be instanced (shader applies instanceMatrix) and stand under their lamp: apex at the luminaire underside, base on the ground.
assert.ok(cones.material.vertexShader.includes('instanceMatrix'), 'lamp vertex shader applies instanceMatrix');
{
  const lamp = {x: 12, z: -7, y: 2, angle: .7};
  const probeScene = new THREE.Scene(), probe = createAtmosphere(probeScene, {lamps: [lamp]});
  const mesh = probeScene.getObjectByName('city-atmosphere').children.find(n => n.isInstancedMesh && n.geometry.type === 'ConeGeometry');
  const m = new THREE.Matrix4(); mesh.getMatrixAt(0, m); mesh.geometry.computeBoundingBox();
  const box = mesh.geometry.boundingBox, apex = new THREE.Vector3(0, box.max.y, 0).applyMatrix4(m), base = new THREE.Vector3(2.7, box.min.y, 0).applyMatrix4(m);
  const head = new THREE.Vector3(lamp.x + 1.55 * Math.cos(lamp.angle), lamp.y + 6.5, lamp.z - 1.55 * Math.sin(lamp.angle));
  assert.ok(apex.distanceTo(head) < .1, 'cone apex touches the luminaire underside (off by ' + apex.distanceTo(head).toFixed(3) + ' m)');
  assert.ok(Math.abs(base.y - lamp.y) < .1, 'cone base lies on the ground at the lamp base height');
  probe.dispose();
}
atmosphere.setTier('medium');   // medium keeps the cones but skips the cinema pass
const at = (day, evening) => { atmosphere.setLighting(day, evening, up); atmosphere.update(.016, 1, camera, true); return atmosphere.stats().lampLevel; };
assert.equal(at(1, 0), 0); assert.equal(cones.visible, false, 'no cones by day');
const afternoon = at(.35, .1);
assert.ok(afternoon > 0 && afternoon < 1 && cones.visible, 'cones fade in towards dusk');
assert.equal(at(0, 1), 1); assert.equal(at(0, 0), 1); assert.equal(cones.visible, true, 'full cones in the evening and at night');
atmosphere.setTier('low'); assert.equal(cones.visible, false, 'low tier keeps cones off');
atmosphere.dispose();
console.log('System World lamps: cones off by day, ' + afternoon.toFixed(2) + ' in the afternoon, full at night.');
