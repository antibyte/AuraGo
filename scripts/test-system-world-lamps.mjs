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
