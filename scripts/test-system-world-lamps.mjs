// Street-lamp light cones are a night effect: invisible by day, fading in towards dusk, off on low.
import assert from 'node:assert/strict';
import * as THREE from 'three';
import {createAtmosphere,coneFade} from '../ui/js/desktop/apps/sysworld-atmosphere.js';
import {surfaces} from '../ui/js/desktop/apps/sysworld-layout.js';

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
  assert.ok(Math.abs(base.y - surfaces.pavement) < .1, 'cone base lies on the pavement');
  probe.dispose();
}
// One soft ground pool per lamp, under the luminaire, 2 cm above the lamp base.
const pools = scene.getObjectByName('city-atmosphere').children.find(n => n.name === 'lamp-pools');
assert.ok(pools && pools.isInstancedMesh, 'lamp pools exist'); assert.equal(pools.count, 1, 'one pool per lamp');
{
  const lamp = {x: 12, z: -7, y: 2, angle: .7};
  const probeScene = new THREE.Scene(), probe = createAtmosphere(probeScene, {lamps: [lamp, {x: 1, z: 2, y: 0, angle: 0}]});
  const mesh = probeScene.getObjectByName('city-atmosphere').children.find(n => n.name === 'lamp-pools');
  assert.equal(mesh.count, 2, 'pool count matches lamps');
  const m = new THREE.Matrix4(), c = new THREE.Vector3(); mesh.getMatrixAt(0, m); c.setFromMatrixPosition(m);
  assert.ok(Math.hypot(c.x - (lamp.x + 1.55 * Math.cos(lamp.angle)), c.z - (lamp.z - 1.55 * Math.sin(lamp.angle))) < .1, 'pool sits under the luminaire');
  assert.ok(Math.abs(c.y - (surfaces.pavement + .02)) < .05, 'pool lies 2 cm above the pavement');
  probe.dispose();
}
atmosphere.setTier('medium');   // medium keeps the cones but skips the cinema pass
const at = (day, evening) => { atmosphere.setLighting(day, evening, up); atmosphere.update(.016, 1, camera, true); return atmosphere.stats().lampLevel; };
assert.equal(at(1, 0), 0); assert.equal(cones.visible, false, 'no cones by day'); assert.equal(pools.visible, false, 'no pools by day');
const afternoon = at(.35, .1);
assert.ok(afternoon > 0 && afternoon < 1 && cones.visible && pools.visible, 'cones and pools fade in towards dusk');
assert.equal(at(0, 1), 1); assert.equal(at(0, 0), 1); assert.equal(cones.visible, true, 'full cones in the evening and at night'); assert.equal(pools.visible, true, 'pools visible at night');
atmosphere.setTier('low'); assert.equal(cones.visible, false, 'low tier keeps cones off'); assert.equal(pools.visible, false, 'low tier keeps pools off');
// Phase 6: cones fade out near the camera (0 inside, 1 beyond 3 m of the cone volume), per instance.
{ const c={x:0,z:0,bottom:0,top:6.5,radius:2.7};
  assert.equal(coneFade(0,1,0,c),0,'inside the cone');assert.equal(coneFade(10,1,0,c),1,'far away');
  const mid=coneFade(2.7+1.5,0,0,c);assert.ok(mid>0&&mid<1,'fading band');
  let last=-1;for(let d=0;d<6;d+=.25){const f=coneFade(2.7+d,0,0,c);assert.ok(f>=last,'monotonic');last=f;}
  assert.equal(coneFade(0,9.6,0,c),1,'3.1 m above the apex'); }
{ const s=new THREE.Scene();s.fog=new THREE.FogExp2(0x08121e,.004);const a=createAtmosphere(s,{lamps:[{x:0,z:0,y:0,angle:0}]}),cam=new THREE.PerspectiveCamera();a.setTier('medium');a.setLighting(0,0,up);
  const mesh=s.getObjectByName('city-atmosphere').children.find(n=>n.isInstancedMesh&&n.geometry.type==='ConeGeometry'),fade=mesh.geometry.getAttribute('fade');
  assert.ok(fade&&fade.isInstancedBufferAttribute,'per-instance fade attribute');
  cam.position.set(1.55,2,0);a.update(.016,1,cam,true);assert.equal(fade.array[0],0,'camera inside: cone faded out');
  cam.position.set(30,2,0);a.update(.016,1,cam,true);assert.ok(fade.array[0]>.99,'camera far: full cone');
  assert.ok(mesh.material.fragmentShader.includes('vFade'),'the fragment shader applies the fade');a.dispose(); }
atmosphere.dispose();
console.log('System World lamps: cones off by day, ' + afternoon.toFixed(2) + ' in the afternoon, full at night.');
