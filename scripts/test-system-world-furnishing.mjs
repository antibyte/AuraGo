// Phase 4 furnishing: deterministic placements, every clearance of design decision 3 (checked independently of
// violations()), the purpose areas, SIZES and RESERVED against the manifests, and the life contract once shipped.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import * as F from '../ui/js/desktop/apps/sysworld-furnishing.js';
import {streets,stations,hoverPads,tramWaypoints,interiors,towers} from '../ui/js/desktop/apps/sysworld-layout.js';
import {LOBBIES} from '../ui/js/desktop/apps/sysworld-lobbies.js';
import {kerb,zebras,pedestrianGraph,plotSlabs,streetProfile,onCarriageway} from '../ui/js/desktop/apps/sysworld-ground.js';
import {PODIUMS,DOORS,FLIGHT,lobbyGround,lobbyGraph} from '../ui/js/desktop/apps/sysworld-lobbies.js';
import {streetCurve} from '../ui/js/desktop/apps/sysworld-navigation.js';

const read=f=>fs.readFile(f,'utf8').then(JSON.parse),v1=await read('ui/3d/system-world/v1/manifest.json'),v2=await read('ui/3d/system-world/v2/manifest.json'),v3=await read('ui/3d/system-world/v3/manifest.json');
const v3ids=new Map(v3.assets.map(a=>[a.id,a])),{gap,footprint}=F,items=F.furnishing(),ground=items.filter(i=>F.SIZES[i.asset]);
assert.equal(F.furnishing(),items,'cached');assert.ok(Object.isFrozen(items));
for(const i of items)assert.ok([i.x,i.y,i.z,i.angle].every(Number.isFinite)&&typeof i.asset==='string'&&i.area,'bad item '+JSON.stringify(i));
const count=(area,asset)=>items.filter(i=>i.area===area&&(!asset||i.asset===asset)).length;
// Purpose areas (decision 5): every explicit piece fits.
assert.deepEqual([count('park','tree-round')+count('park','tree-column'),count('park','flower-bed'),count('park','street-bench')],[11,3,4],'park');
assert.deepEqual([count('market','kiosk'),count('market','cafe-set'),count('market','banner-mast')],[3,4,4],'market');
assert.equal(count('grove'),12,'groves');assert.deepEqual([count('plaza','charge-post'),count('plaza','info-pylon'),count('plaza')],[4,1,11],'plaza');
assert.deepEqual([count('cafe','cafe-set'),count('cafe')],[3,5],'cafe');assert.equal(count('quay','mooring-post'),8);assert.equal(count('station','info-pylon'),7);
// Rule-made pieces: lower bounds only (the hand-back reports the real numbers).
// Street rows are evenly spaced at a target of 8 m; the floor leaves room for reserved places that move later (Task 10 moves the parcel sorter).
assert.ok(count('street')>=100,'street trees '+count('street'));assert.ok(count('pavement','zebra-sign')>=52,'signs '+count('pavement','zebra-sign'));
assert.ok(count('pavement','bollard')>=100,'bollards '+count('pavement','bollard'));assert.ok(count('pavement','litter-bin')>=20,'bins '+count('pavement','litter-bin'));
assert.ok(count('district','flower-bed')>=6&&count('district','street-bench')>=6&&count('district','utility-cabinet')>=6,'district furnishing');
// Clearances, each computed here from the ground, graph and registry modules.
const P=kerb.pavementLine,pt=(x,z)=>[x,z,x,z];
const lines=[...streets.zs.flatMap(z=>[-P,P].map(o=>[streets.minX,z+o,streets.maxX,z+o])),...streets.xs.flatMap(x=>[-P,P].map(o=>[x+o,streets.minZ,x+o,streets.quayZ]))];
const zebraLinks=zebras().map(zb=>zb.axis==='x'?[zb.at,zb.line-P,zb.at,zb.line+P]:[zb.line-P,zb.at,zb.line+P,zb.at]);
const nodes=[...pedestrianGraph().nodes,...lobbyGraph().nodes.filter(n=>n.kind==='foot'||n.kind==='porch')];
const curve=streetCurve(tramWaypoints),rail=[];
for(let i=0;i<2000;i++){const p=curve.getPointAt(i/2000),t=curve.getTangentAt(i/2000);for(const s of[-.9,.9])rail.push(pt(p.x-t.z*s,p.z+t.x*s));}
const platforms=stations.map(s=>F.turnedBox([-4.5,-2,4.5,2],s.platformX,s.platformZ,s.angle));
const stairs=PODIUMS.flatMap(p=>p.stairs.map(f=>[f.x-f.width/2,p.z1,f.x+f.width/2,p.z1+FLIGHT])),slabs=plotSlabs();
for(const it of ground){
  const f=footprint(it),name=JSON.stringify(it),pave=F.PAVEMENT.has(it.asset),corners=[[f[0],f[1]],[f[2],f[1]],[f[0],f[3]],[f[2],f[3]]];
  if(pave){
    for(const [x,z]of corners){const p=streetProfile(x,z);assert.ok(p&&p.kind==='pavement',name+' off the pavement at '+x+','+z);}
    const p=streetProfile(it.x,it.z);assert.ok(p.d-p.run<=.5+1e-9,name+' more than 0.5 m behind the kerb');
    for(const zb of zebras()){const across=zb.axis==='x'?Math.abs(it.z-zb.line):Math.abs(it.x-zb.line),along=zb.axis==='x'?Math.max(f[0]-zb.at,zb.at-f[2],0):Math.max(f[1]-zb.at,zb.at-f[3],0);
      assert.ok(across>streets.halfWidth||along>=2.1-1e-9,name+' on a zebra, dropped kerb or tactile strip');}
  }else assert.ok(slabs.some(s=>f[0]>=s.x0+.4-1e-9&&f[2]<=s.x1-.4+1e-9&&f[1]>=s.z0+.4-1e-9&&f[3]<=s.z1-.4+1e-9),name+' not 0.4 m inside a plot');
  for(const [x,z]of[...corners,[it.x,it.z]]){assert.equal(onCarriageway(x,z),false,name+' on a carriageway');assert.equal(lobbyGround(x,z),null,name+' on a podium or stair');}
  for(const n of nodes)assert.ok(gap(pt(n.x,n.z),f)>=1.2-1e-9,name+' within 1.2 m of node '+n.x+','+n.z);
  for(const l of lines)assert.ok(gap(l,f)>=(pave?.6:1.2)-1e-9,name+' too close to a pavement line');
  for(const l of zebraLinks)assert.ok(gap(l,f)>=1.2-1e-9,name+' within 1.2 m of a zebra link');
  for(const d of hoverPads)assert.ok(gap(pt(d.x,d.z),f)>=3-1e-9,name+' within 3 m of dock '+d.id);
  assert.ok(rail.every(r=>gap(r,f)>=2.5-1e-9),name+' within 2.5 m of a rail');
  for(const b of platforms)assert.ok(gap(b,f)>=2.5-1e-9,name+' within 2.5 m of a platform');
  for(const s of stairs)assert.ok(gap(s,f)>=2-1e-9,name+' within 2 m of stairs');
  for(const d of DOORS)assert.ok(gap([d.x-d.width/2,d.z,d.x+d.width/2,d.z],f)>=2-1e-9,name+' within 2 m of a door');
  // Entrances stay visible from the street: no tree in the strip 1.5 m beyond each flight's width, from the podium edge 18 m out.
  if(/^tree-/.test(it.asset))for(const p of PODIUMS)for(const s of p.stairs)
    assert.ok(!(it.x>=s.x-s.width/2-1.5&&it.x<=s.x+s.width/2+1.5&&it.z>=p.z1&&it.z<=p.z1+18),name+' stands in front of a stair flight');
  for(const r of F.RESERVED)assert.ok(gap(r.box,f)>=(r.kind==='living'?3:1)-1e-9,name+' too close to '+r.id);
}
const fps=ground.map(footprint);
for(let i=0;i<fps.length;i++)for(let j=i+1;j<fps.length;j++)assert.ok(gap(fps[i],fps[j])>=1-1e-9,'colliders closer than 1 m: '+JSON.stringify([ground[i],ground[j]]));
// SIZES mirror the manifest colliders; RESERVED boxes cover the real footprints (colliders, else LOD 0 bounds).
const union=c=>[Math.min(...c.map(x=>x[0])),Math.min(...c.map(x=>x[2])),Math.max(...c.map(x=>x[3])),Math.max(...c.map(x=>x[5]))];
for(const [id,size]of Object.entries(F.SIZES)){const a=v3ids.get(id);if(!a){assert.ok(F.LIFE_DESIGNS.includes(id),id+' is not shipped');continue;}
  union(a.navigation.colliders).forEach((v,k)=>assert.ok(Math.abs(v-size[k])<.011,id+' SIZES differ from its colliders'));}
const lookup=id=>v3ids.get(id)||v2.assets.find(a=>a.id===id)||v1.assets.find(a=>a.id===id);
for(const r of F.RESERVED){if(!r.asset)continue;const a=lookup(r.asset);assert.ok(a,r.id+': no asset '+r.asset);
  const c=a.navigation?.colliders?.length?a.navigation.colliders:null,b=a.lods[0].bounds,local=c?union(c):[b.min[0],b.min[2],b.max[0],b.max[2]];
  if(r.asset==='bridge'){local[0]*=1.75;local[2]*=1.75;}
  const need=F.turnedBox(local,r.x,r.z,r.angle);assert.ok(need[0]>=r.box[0]-.01&&need[1]>=r.box[1]-.01&&need[2]<=r.box[2]+.01&&need[3]<=r.box[3]+.01,`${r.id} box ${r.box} must cover ${need.map(v=>v.toFixed(2))}`);}
// Life designs (Task 0): ship together; parts; the charge post's pylon stands behind its pad; places.
const life=F.LIFE_DESIGNS.filter(id=>v3ids.has(id));
assert.ok(life.length===0||life.length===F.LIFE_DESIGNS.length,'Life designs ship together: '+life);
if(life.length){
  for(const [id,parts]of[['sail-skiff',['rig']],['harbour-buoy',['lamp']],['banner-mast',['banner']],['gull',['wing_l','wing_r']]])for(const p of parts)assert.ok(v3ids.get(id).components.includes(p),id+' part '+p);
  const post=v3ids.get('charge-post');assert.deepEqual(post.navigation.workpoints.map(w=>w.id),['charge']);
  assert.ok(post.navigation.colliders.every(b=>b[5]<=0),'charge-post collider must stand behind the pad (glTF -Z): '+JSON.stringify(post.navigation.colliders));
  const places=F.furnishingPlaces(v3ids);assert.deepEqual(places.map(p=>p.place.id),['charge-plaza','harbour-cafe']);
  assert.equal(places[0].entry.navigation.workpoints.length,4);assert.ok(places.every(p=>p.place.kit));
}
console.log(`System World furnishing: ${items.length} pieces (${count('street')} street trees, ${count('pavement')} pavement, ${count('district')+count('station')} district/station, `
  +`${items.length-count('street')-count('pavement')-count('district')-count('station')} in purpose areas); clearances, sizes and reserved boxes verified; life designs `+(life.length?'shipped':'not shipped yet')+'.');
// Real colliders: installCityColliders reads every kit design's navigation.colliders; nothing else within 1 m.
{
  const {createTraffic}=await import('../ui/js/desktop/apps/sysworld-traffic.js');
  const {installCityColliders,kitCatalog}=await import('../ui/js/desktop/apps/sysworld-colliders.js');
  const {districts,placements,kitPlacements,obstacles}=await import('../ui/js/desktop/apps/sysworld-scene.js');
  const traffic=createTraffic();installCityColliders(traffic,kitCatalog(v1,v3),districts,placements,kitPlacements);
  let checked=0;
  kitPlacements.forEach((p,i)=>{
    if(!F.SIZES[p.asset]||!(v3ids.get(p.asset)?.navigation?.colliders||[]).length)return;
    const f=footprint(p),probe={id:'probe',ignore:'kit:'+i,circles:[{x:0,z:0,r:.9}],minY:0,maxY:2.4,reach:.9},clearAt=(x,z)=>traffic.obstruction(probe,{x,y:p.y,z,heading:0},{x,y:p.y,z,heading:0},false);
    const mid={x:(f[0]+f[2])/2,y:p.y+.5,z:(f[1]+f[3])/2,heading:0};
    assert.equal(traffic.clear({id:'in',circles:[{x:0,z:0,r:.05}],minY:0,maxY:.1,reach:.05},mid,mid,false),false,JSON.stringify(p)+' has no solid');
    for(let x=f[0];x<=f[2]+1e-9;x+=Math.max(.2,(f[2]-f[0])/4))for(const z of[f[1],f[3]])assert.equal(clearAt(x,z),null,JSON.stringify(p)+' has '+clearAt(x,z)+' within 1 m');
    for(let z=f[1];z<=f[3]+1e-9;z+=Math.max(.2,(f[3]-f[1])/4))for(const x of[f[0],f[2]])assert.equal(clearAt(x,z),null,JSON.stringify(p)+' has '+clearAt(x,z)+' within 1 m');
    checked++;});
  assert.ok(checked>=300,'placed furniture with solids: '+checked);
  assert.ok(obstacles.some(o=>o.asset==='litter-bin')&&obstacles.some(o=>o.asset==='bollard'),'street robots steer around pavement pieces');
  assert.ok(!obstacles.some(o=>/^tree-/.test(o.asset)),'plot pieces stay out of the robot obstacles');
  console.log(`System World furniture colliders: ${checked} pieces solid and clear of every other solid by 1 m.`);
}
// Neon (decision 9): wide signs on the six rectangular halls' top beams, round signs beside the pavilion doors,
// blade signs on the data-tower fronts. Signs have no colliders.
{
  const neon=items.filter(i=>i.area==='neon');
  assert.deepEqual(['neon-wave','neon-ring','neon-blade'].map(a=>neon.filter(i=>i.asset===a).length),[6,3,6]);
  for(const i of neon.filter(i=>i.asset==='neon-wave')){const l=LOBBIES.find(l=>l.rect&&Math.abs((l.rect[0]+l.rect[2])/2-i.x)<1e-9&&Math.abs(i.z-l.front)<=.5);
    assert.ok(l&&Math.abs(i.z-l.front)<=.5&&i.y>=l.ceiling+.9&&i.y+1.1<=l.ceiling+3&&i.angle===0,'wave off its beam '+JSON.stringify(i));}
  for(const i of neon.filter(i=>i.asset==='neon-ring'))assert.ok(interiors.some(r=>Math.abs(i.z-r.doorZ)<=.5&&Math.abs(i.x-r.x)<=6)&&i.angle===Math.PI&&i.y>=2,'ring off a pavilion front '+JSON.stringify(i));
  for(const i of neon.filter(i=>i.asset==='neon-blade'))assert.ok(towers.some(t=>Math.abs(i.z-t.z-6.5)<1e-9&&Math.abs(Math.abs(i.x-t.x)-6.6)<1e-9)&&i.y===6,'blade off a tower front '+JSON.stringify(i));
  assert.ok(neon.every(i=>!F.SIZES[i.asset]),'signs carry no colliders');
  console.log(`System World neon: ${neon.length} signs on registry mount points, no colliders.`);
}
