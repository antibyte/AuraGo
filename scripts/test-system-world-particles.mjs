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
