// Phase 5 zones: the registry against the scene and kit, weights at known points, rooms, doors, surfaces, the tracker.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import * as Z from '../ui/js/desktop/apps/sysworld-zones.js';
import {districts} from '../ui/js/desktop/apps/sysworld-scene.js';
import {LOBBIES} from '../ui/js/desktop/apps/sysworld-lobbies.js';
import {interiors} from '../ui/js/desktop/apps/sysworld-layout.js';
import {streetProfile} from '../ui/js/desktop/apps/sysworld-ground.js';

districts.forEach((d,i)=>{const b=Z.BUILDINGS[i];assert.deepEqual([b.x,b.z,b.height,b.radius],[d.x,d.z,d.height,d.radius],'building '+d.id);});
const v3=JSON.parse(await fs.readFile('ui/3d/system-world/v3/manifest.json','utf8'));
assert.equal(Z.TOWER_HEIGHT,v3.assets.find(a=>a.id==='data-tower-a').lods[0].bounds.max[1],'TOWER_HEIGHT mirrors data-tower-a');
for(const l of LOBBIES)assert.ok(Z.ROOMS['lobby:'+l.id],'room for lobby '+l.id);
for(const r of interiors)assert.ok(Z.ROOMS['pavilion:'+r.id],'room for pavilion '+r.id);
assert.equal(Object.keys(Z.ROOMS).length,LOBBIES.length+interiors.length);assert.equal(Object.values(Z.ROOMS).filter(r=>r.open).length,1,'only the garden court is open');

const t0=performance.now(),w=(x,y,z,ox,oz)=>Z.zoneWeights(x,y,z,ox,oz,{}),first=w(0,2.4,0),build=performance.now()-t0;
const KEYS=['harbour','canyon','green','plaza','height'],top=o=>KEYS.reduce((a,k)=>o[k]>o[a]?k:a,'harbour'),sum=o=>KEYS.reduce((s,k)=>s+o[k],0),fmt=o=>KEYS.map(k=>k+' '+o[k].toFixed(2)).join(', ');
const quay=w(-20,2.48,77),street=w(-13.5,2.4,-12),park=w(-44,2.4,-88),plaza=w(79,2.4,-55),roof=w(0,74.6,-12),overview=w(120,110,183,0,-12);
assert.equal(top(quay),'harbour',fmt(quay));assert.ok(quay.harbour>=.6&&quay.quay===1&&quay.room===null,fmt(quay));
assert.equal(top(street),'canyon',fmt(street));assert.ok(street.canyon>=.5&&street.harbour<.05,fmt(street));
assert.equal(top(park),'green',fmt(park));assert.ok(park.green>=.8,fmt(park));
assert.equal(top(plaza),'plaza',fmt(plaza));assert.ok(plaza.plaza>=.5,fmt(plaza));
assert.ok(roof.height>=.95,'sky deck '+fmt(roof));assert.equal(overview.height,1);assert.equal(first.room,null);
const halls={agent:[0,-12],memory:[-43,-11],'integrations-west':[35,-10],'integrations-east':[51,-10],missions:[43,35],infra:[-43,-56],graph:[-43,36]};
for(const [id,[x,z]]of Object.entries(halls)){const o=w(x,3.28,z);assert.equal(o.room,'lobby:'+id,id);assert.equal(o.interior,1);assert.equal(o.open,id==='graph');assert.equal(o.space,Z.ROOMS['lobby:'+id].space);}
for(const r of interiors)assert.equal(w(r.x,2.56,r.z).room,'pavilion:'+r.id);
assert.equal(w(0,12,-12).room,null,'above the lobby band is outdoors');
let seed=7;const rnd=()=>(seed=(Math.imul(seed,1664525)+1013904223)>>>0)/4294967296;
for(let i=0;i<3000;i++){const o=w((rnd()-.5)*260,rnd()*120,(rnd()-.5)*260);assert.ok(Math.abs(sum(o)-1)<1e-9,'shares sum to 1: '+fmt(o));for(const k of KEYS)assert.ok(o[k]>=-1e-12&&o[k]<=1+1e-12);}
// Doors.
const value=key=>key==='lobby:agent:0'?.8:key==='pavilion:memory'?.5:0;
const inside=Z.doorState(0,3.28,-12,value,{});assert.deepEqual([inside.room,inside.open,inside.leakRoom],['lobby:agent',.8,null]);
const porch=Z.doorState(0,2.4,-1,value,{});assert.deepEqual([porch.room,porch.leakRoom,porch.open],[null,'lobby:agent',.8]);assert.ok(porch.leakDistance<5);
assert.equal(Z.doorState(0,2.4,30,value,{}).leakRoom,null);const pav=Z.doorState(-43,2.4,64,value,{});assert.deepEqual([pav.leakRoom,pav.open],['pavilion:memory',.5]);
// Surfaces.
const S=Z.surfaceAt,seen=new Set();const at=(x,y,z,want)=>{assert.equal(S(x,y,z),want,[x,y,z]+' should be '+want);seen.add(want);};
at(-18,-.14,-50,'road');at(-14.63,0,-50,'kerb');at(-13.5,0,-50,'pavement');at(0,.5,.3,'stairs');at(0,.88,-1,'lobby');at(0,.88,-12,'lobby');
at(-43,.88,-56,'grating');at(-43,.88,36,'garden');at(-44,0,-88,'garden');at(-20,.08,77,'quay');at(0,.16,73,'lobby');at(0,72.2,-16,'grating');at(-78.5,2,40,'grating');
assert.deepEqual([...seen].sort(),[...Z.SURFACE_KINDS].sort(),'every surface kind is reachable');
for(const z of[-50,-24])for(let x=-24.5;x<=-11.5;x+=.05){const p=streetProfile(x,z);if(!p)continue;assert.equal(S(x,0,z),p.kind==='road'?'road':p.kind==='kerb'?'kerb':'pavement','street profile at '+x.toFixed(2)+','+z);}
// Tracker.
const tr=Z.createZoneTracker(),listener={x:-20,z:77,fx:0,fz:-1};let raw=w(-20,2.48,77);
assert.equal(tr.update(raw,null,0,listener),tr.weights);assert.ok(Math.abs(tr.weights.harbour-raw.harbour)<1e-12,'the first update snaps');
const raw2=w(-13.5,2.4,-12);let prev=tr.weights.canyon;
for(let i=0;i<180;i++){tr.update(raw2,null,1/60,listener);assert.ok(tr.weights.canyon>=prev-1e-12);prev=tr.weights.canyon;assert.ok(Math.abs(sum(tr.weights)-1)<1e-9);}
assert.ok(Math.abs(tr.weights.canyon-raw2.canyon)<=.05*Math.abs(raw2.canyon-raw.canyon)+1e-9,'95 % after three seconds');
const hall=w(0,3.28,-12);for(let i=0;i<60;i++)tr.update(hall,Z.doorState(0,3.28,-12,()=>0,{}),1/60,listener);assert.ok(tr.weights.interior>.95&&tr.weights.room==='lobby:agent');
for(let i=0;i<30;i++)tr.update(raw2,null,1/60,listener);assert.equal(tr.weights.room,'lobby:agent','the room id stays while the interior fades');
for(let i=0;i<120;i++)tr.update(raw2,null,1/60,listener);assert.equal(tr.weights.room,null);
const out={x:3,z:-1,fx:0,fz:-1};tr.update(w(3,2.4,-1),Z.doorState(3,2.4,-1,()=>1,{}),1/60,out);
assert.ok(tr.weights.leak>.5&&tr.weights.leakTone==='agent'&&tr.weights.leakSpace==='lobby'&&tr.weights.leakPan<0,'an open door on the left leaks its room tone');
console.log(`System World zones: fields built in ${build.toFixed(0)} ms; quay ${fmt(quay)}; street ${fmt(street)}; park ${fmt(park)}; plaza ${fmt(plaza)}; rooms, doors, surfaces and tracker verified.`);
