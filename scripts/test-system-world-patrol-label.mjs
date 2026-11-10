// A patrol robot shows the 'yield' label only while it is held for a tram or stepping aside; it clears when the
// robot moves on, also when the society is suspended while the robot waits.
import assert from 'node:assert/strict';
import * as THREE from 'three';
import {createTraffic,bodyShape} from '../ui/js/desktop/apps/sysworld-traffic.js';
import {createSociety} from '../ui/js/desktop/apps/sysworld-society.js';

const world=createTraffic(),scene=new THREE.Scene(),camera=new THREE.PerspectiveCamera();
const shape=bodyShape({min:[-.3,0,-.3],max:[.3,1.6,.3]},1.15),tramShape=bodyShape({min:[-.8,0,-4],max:[.8,3.5,4]});
const body=world.register('patrol-0',shape,{x:0,y:0,z:0,heading:0},1);
const tram=world.register('tram-0',tramShape,{x:2.2,y:0,z:0,heading:0},3);
assert.ok(body&&tram);
const society=createSociety(scene,{traffic:world,camera,floor:()=>0,active:()=>true});
society.patrol('patrol-0',body,new THREE.Group(),()=>{});
const state=()=>society.stats().residents.find(r=>r.id==='patrol-0')?.state;
const step=n=>{for(let i=0;i<n;i++){world.begin();society.update(1/60,true);world.solve(1/60);}};
step(5);
assert.equal(state(),'yield','A tram beside the patrol makes it yield');
society.suspend();
assert.equal(state(),'patrol','Suspending clears the yield label of a held patrol');
step(1);
world.remove('tram-0');
step(120);
assert.equal(state(),'patrol','The label clears once the tram is gone');
society.dispose();world.dispose();
console.log('System World patrol label: yield shows only while held, clears on suspend and when the tram is gone.');
