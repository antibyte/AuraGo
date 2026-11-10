import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import * as THREE from 'three';
import {createHoverFlight,hoverLimits} from '../ui/js/desktop/apps/sysworld-hover.js';
import {createExperience} from '../ui/js/desktop/apps/sysworld-experience.js';
import {districts,placements,kitPlacements} from '../ui/js/desktop/apps/sysworld-scene.js';
import {streetCurve} from '../ui/js/desktop/apps/sysworld-navigation.js';
import {canWalk,groundHeight,roomAt,tramWaypoints,dronePad} from '../ui/js/desktop/apps/sysworld-exploration.js';
import {streets,streetAt,hoverPads} from '../ui/js/desktop/apps/sysworld-layout.js';
import {installCityColliders,kitCatalog} from '../ui/js/desktop/apps/sysworld-colliders.js';
import {createTraffic} from '../ui/js/desktop/apps/sysworld-traffic.js';
import {pedestrianGraph} from '../ui/js/desktop/apps/sysworld-ground.js';

// The street-mode visitor: a slim body from the soles (2.3 m below the eye) to just above the head.
const walker={circles:[{x:0,z:0,r:.24}],minY:-2.3,maxY:.4,reach:.24};
const pose=p=>({x:p.x,y:p.y,z:p.z,heading:0});
const dt=1/60;
function world(solids=[],{ground=()=>.5,walkable=()=>true,reduced=false}={}){
  const traffic=createTraffic();solids.forEach((s,i)=>traffic.solid('s'+i,s));
  const body={id:'visitor',...walker};
  const flight=createHoverFlight({clear:(a,b)=>traffic.clear(body,pose(a),pose(b),false),ground,walkable:(x,z)=>walkable(x,z)&&traffic.clear(body,{x,y:ground(x,z)+2.4,z,heading:0},{x,y:ground(x,z)+2.4,z,heading:0},false),reduced:()=>reduced});
  return {traffic,body,flight};
}
const run=(flight,seconds,input,yaw=0,onStep)=>{let p=flight.state();for(let t=0;t<seconds-1e-9;t+=dt){p=flight.step(dt,input,yaw);onStep?.(p,t);}return p;};
const takeoff=(flight,at)=>{flight.start(at);return run(flight,hoverLimits.takeoffTime+.05,{});};

// Takeoff lifts the visitor 1.5 m in the first 0.7 s and ignores the controls meanwhile.
{
  const {flight}=world(),start={x:5,y:2.9,z:7};
  flight.start(start);const mid=run(flight,.35,{forward:1,up:1});
  assert.ok(mid.y>start.y+.3&&mid.y<start.y+1.2,'Takeoff is under way after 0.35 s: '+mid.y);
  assert.equal(mid.x,5);assert.equal(mid.z,7,'Controls wait for the takeoff');
  const top=run(flight,.4,{});
  assert.ok(Math.abs(top.y-(start.y+hoverLimits.takeoff))<=hoverLimits.bob+1e-9,'Takeoff ends 1.5 m up (within the bob): '+top.y);assert.equal(flight.state().phase,'flying');
  assert.equal(flight.state().altitude.toFixed(2),'1.50');
}
// WASD follows the camera heading; velocity eases in and out; Shift boosts.
{
  const {flight}=world();takeoff(flight,{x:0,y:2.9,z:0});
  const a=flight.step(dt,{forward:1},0),speedOne=flight.state().speed;
  assert.ok(a.z<0&&Math.abs(a.x)<1e-9,'Forward at yaw 0 flies towards -z');assert.ok(speedOne>0&&speedOne<hoverLimits.cruise*.2,'Velocity eases in: '+speedOne);
  run(flight,2,{forward:1});assert.ok(Math.abs(flight.state().speed-hoverLimits.cruise)<.2,'Cruise is 12 m/s: '+flight.state().speed);
  run(flight,1.5,{forward:1,boost:true});assert.ok(Math.abs(flight.state().speed-hoverLimits.boost)<.5,'Boost is 26 m/s: '+flight.state().speed);
  flight.step(dt,{},0);assert.ok(flight.state().speed>hoverLimits.boost*.9,'No instant stop');
  run(flight,3,{});assert.equal(flight.state().speed,0,'The visitor comes to rest');
  const before=flight.state();
  const left=run(flight,1,{forward:1},Math.PI/2);assert.ok(left.x<before.x-5&&Math.abs(left.z-before.z)<.5,'Yaw 90 degrees turns forward towards -x');
  run(flight,3,{});const here=flight.state(),right=run(flight,1,{right:1},0);assert.ok(right.x>here.x+4&&Math.abs(right.z-here.z)<.5,'Right at yaw 0 flies towards +x');
  run(flight,3,{});run(flight,3,{forward:1,right:1},0);assert.ok(flight.state().speed<=hoverLimits.cruise+.01,'Diagonals are not faster: '+flight.state().speed);
  const climb=run(flight,2,{up:1}).y;run(flight,2,{up:-1});assert.ok(flight.state().y<climb,'C descends');assert.ok(Math.abs(flight.state().climb)<=hoverLimits.sink+.01);
}
// A solid blocks the flight, which slides along it and loses the velocity of the blocked axis.
{
  const {flight}=world([{x:0,z:-30,min:[-15,0,-.5],max:[15,60,.5]}]);takeoff(flight,{x:0,y:2.9,z:0});
  const end=run(flight,5,{forward:1});
  assert.ok(end.z>-30+.24,'The wall stops the flight: '+end.z);assert.ok(end.z<-27,'Flight reaches the wall: '+end.z);assert.equal(flight.state().speed,0,'A blocked axis loses its velocity');
  const x0=end.x,slide=run(flight,1.5,{forward:1,right:1});
  assert.ok(slide.x>x0+8,'The flight slides along the wall: '+(slide.x-x0));assert.ok(Math.abs(slide.z-end.z)<.01,'...without entering it');
  const over=run(flight,10,{up:1});assert.ok(over.y>60+2.3,'Climbing clears the wall: '+over.y);
  const across=run(flight,3,{forward:1});assert.ok(across.z<-31,'...and the flight crosses it: '+across.z);
}
// Floor (street eye height, never below), ceiling and horizontal limits.
{
  const ground=(x,z)=>x>20?3:.5,{flight}=world([],{ground});takeoff(flight,{x:0,y:2.9,z:0});
  const low=run(flight,4,{up:-1});assert.equal(low.y,.5+hoverLimits.eye,'The floor is the street eye height');
  run(flight,1,{up:-1},0,p=>assert.ok(p.y>=ground(p.x,p.z)+hoverLimits.eye-1e-9));
  run(flight,3,{right:1},0,p=>assert.ok(p.y>=ground(p.x,p.z)+hoverLimits.eye-1e-9,'Never below the floor'));
  assert.equal(flight.state().y,3+hoverLimits.eye,'Rising ground lifts the flight with it');
  const sky=run(flight,40,{up:1});assert.equal(sky.y,hoverLimits.ceiling,'The ceiling is 150 m');assert.ok(flight.state().altitude>140);
  const east=run(flight,30,{right:1,boost:true});assert.ok(east.x<=hoverLimits.maxX&&east.x>hoverLimits.maxX-.5,'Eastern limit: '+east.x);
  const north=run(flight,30,{forward:1,boost:true});assert.equal(north.z,hoverLimits.minZ);
  const west=run(flight,30,{right:-1,boost:true});assert.equal(west.x,hoverLimits.minX);
  const south=run(flight,30,{forward:-1,boost:true});assert.equal(south.z,hoverLimits.maxZ);
}
// The hover bob is subtle, only while flying with motion allowed, and never dips below the floor.
{
  const {flight}=world();takeoff(flight,{x:0,y:2.9,z:0});run(flight,.5,{up:1});run(flight,2,{});
  let min=Infinity,max=-Infinity;run(flight,3,{},0,p=>{min=Math.min(min,p.y);max=Math.max(max,p.y);});
  assert.ok(max-min>.02&&max-min<=.06+1e-9,'Bob of about +-3 cm: '+(max-min));
  const quiet=world([],{reduced:true});quiet.flight.start({x:0,y:2.9,z:0});quiet.flight.step(dt,{});
  let still=new Set();run(quiet.flight,2,{up:1});run(quiet.flight,4,{});run(quiet.flight,2,{},0,p=>still.add(p.y));assert.equal(still.size,1,'No bob under reduced motion');
  const grounded=world();takeoff(grounded.flight,{x:0,y:2.9,z:0});run(grounded.flight,3,{up:-1},0,p=>assert.ok(p.y>=2.9-1e-9,'The bob never sinks below the floor'));
}
// Landing: nearest walkable spot, glides within its speed limits around a building and finishes at eye height.
{
  const block={x:0,z:0,min:[-10,0,-10],max:[10,30,10]},standable=(x,z)=>Math.abs(x)>11||Math.abs(z)>11;
  const {flight,traffic,body}=world([block],{walkable:standable});
  takeoff(flight,{x:20,y:2.9,z:0});run(flight,4,{up:1});run(flight,2,{right:-1});
  const start=flight.state();assert.ok(start.altitude>20,'Airborne first: '+start.altitude);
  assert.equal(flight.land(),true);assert.equal(flight.state().landing,true);
  let last=flight.state(),worstH=0,worstV=0,steps=0;
  while(!flight.state().done&&steps++<60*60){
    const p=flight.step(dt,{forward:1,up:1},1);// controls are ignored while landing
    worstH=Math.max(worstH,Math.hypot(p.x-last.x,p.z-last.z)/dt);worstV=Math.max(worstV,Math.abs(p.y-last.y)/dt);
    assert.ok(traffic.clear(body,pose(p),pose(p),false),'The glide never enters a solid: '+JSON.stringify(p));last=p;
  }
  const end=flight.state();
  assert.ok(end.done&&end.phase==='idle'&&!end.flying,'Landing reports done');assert.ok(worstH<=hoverLimits.glide+1e-6,'Glide <= 8 m/s: '+worstH);assert.ok(worstV<=hoverLimits.descend+1e-6,'Descent <= 6 m/s: '+worstV);
  assert.equal(end.y,.5+hoverLimits.eye,'Touchdown at street eye height');assert.ok(standable(end.x,end.z),'Touchdown on a walkable spot');
  assert.ok(Math.hypot(end.x-start.x,end.z-start.z)<20,'The nearest spot was chosen: '+Math.hypot(end.x-start.x,end.z-start.z));
  const spot=world([],{walkable:(x,z)=>x>50});spot.flight.start({x:30,y:20,z:0});run(spot.flight,1,{});spot.flight.place({x:30,y:20,z:0});
  assert.ok(spot.flight.land());run(spot.flight,40,{});const there=spot.flight.state();assert.ok(there.done&&there.x>50&&there.x<52,'Spiral search finds the nearest spot: '+there.x);
  const none=world([],{walkable:()=>false});takeoff(none.flight,{x:0,y:2.9,z:0});assert.equal(none.flight.land(),false,'No spot: the visitor keeps flying');assert.equal(none.flight.state().phase,'flying');
  assert.equal(none.flight.land(true),false);assert.equal(flight.land(),false,'Nothing to land when grounded');
}
// Rising ground lifts the flight only through free space: a refused rise undoes the step instead of forcing the visitor into a solid.
{
  const ramp=x=>x>10?2.5:.5,{flight,traffic,body}=world([{x:15,z:0,min:[-5,5,-20],max:[5,9,20]}],{ground:ramp}),xs=[];
  takeoff(flight,{x:0,y:2.9,z:0});
  run(flight,4,{right:1},0,p=>{assert.ok(traffic.clear(body,pose(p),pose(p),false),'The refused rise never pushes the visitor into the ceiling: '+JSON.stringify(p));xs.push(p.x);});
  assert.ok(Math.max(...xs)<=10+1e-9,'The step onto the low ramp is refused: '+Math.max(...xs));
  run(flight,2,{right:-1});run(flight,3,{up:1});run(flight,3,{right:1});assert.ok(flight.state().x>12,'High enough, the flight rides up the ramp');assert.ok(flight.state().y>=ramp(flight.state().x)+hoverLimits.eye);
}
// A landing hands the controls back on fresh input or a second E; a walled-in glide re-plans, then gives up; a slow one ends at the nearest spot.
{
  const {flight}=world([],{walkable:x=>Math.abs(x)>30});takeoff(flight,{x:0,y:2.9,z:0});run(flight,2,{up:1});
  assert.ok(flight.land());run(flight,.3,{forward:1});assert.equal(flight.state().landing,true,'Keys already held when the landing began do not cancel it');
  run(flight,.1,{});assert.equal(flight.state().landing,true);const z0=flight.step(dt,{forward:1},0).z;
  assert.equal(flight.state().phase,'flying','Fresh input hands the flight back');assert.ok(run(flight,1,{forward:1}).z<z0-3,'...and the visitor steers again');
  assert.ok(flight.land());run(flight,.2,{});assert.equal(flight.cancel(),true);assert.equal(flight.state().phase,'flying','A second E cancels the landing');assert.equal(flight.cancel(),false);
  const walled={on:false},stuck=createHoverFlight({clear:()=>!walled.on,ground:()=>.5,walkable:x=>x>5});
  stuck.start({x:0,y:2.9,z:0});run(stuck,1,{});run(stuck,1,{up:1});assert.ok(stuck.land());walled.on=true;run(stuck,2,{});
  assert.equal(stuck.state().phase,'flying','A landing that cannot move gives the controls back');walled.on=false;assert.ok(run(stuck,1,{forward:1}).z<-3);
  const {flight:detour,traffic,body}=world([],{walkable:x=>x>8});takeoff(detour,{x:0,y:2.9,z:0});run(detour,3,{up:1});
  assert.ok(detour.land());const first=detour.state().target;traffic.solid('door',{x:9,z:0,min:[-1,0,-1],max:[1,30,1]});
  let steps=0;while(!detour.state().done&&steps++<60*30){const p=detour.step(dt,{});assert.ok(traffic.clear(body,pose(p),pose(p),false),'The glide never enters the solid: '+JSON.stringify(p));}
  const end=detour.state();assert.ok(end.done,'A walled-in glide re-plans and finishes: '+steps*dt+' s');assert.ok(Math.hypot(end.x-first.x,end.z-first.z)>=1&&end.x>8,'...on another spot');
  const slack=hoverLimits.slack,tall={x:0,z:0,min:[-10,0,-10],max:[10,30,10]},standable=(x,z)=>Math.abs(x)>11||Math.abs(z)>11,landFrom=()=>{
    const w=world([tall],{walkable:standable});w.flight.start({x:0,y:40,z:0});run(w.flight,1,{});w.flight.place({x:0,y:40,z:0});assert.ok(w.flight.land());
    const t=w.flight.state().target;let n=0;while(!w.flight.state().done&&n++<60*40)w.flight.step(dt,{});return {seconds:n*dt,end:w.flight.state(),estimate:Math.max(Math.hypot(t.x,t.z)/hoverLimits.glide,(40-.5-hoverLimits.eye)/hoverLimits.descend)};};
  try{
    hoverLimits.slack=15;const free=landFrom();assert.ok(free.end.done&&free.seconds>free.estimate,'A real glide is slower than its estimate: '+free.seconds+' vs '+free.estimate);
    hoverLimits.slack=0;const capped=landFrom();assert.ok(capped.end.done&&capped.seconds<=capped.estimate+.1,'Past the estimate plus slack the landing ends at the nearest spot: '+capped.seconds+' vs '+capped.estimate);
    assert.equal(capped.end.y,.5+hoverLimits.eye);assert.ok(standable(capped.end.x,capped.end.z));
  }finally{hoverLimits.slack=slack;}
}
// Reduced motion: takeoff and landing are instant.
{
  const {flight}=world([],{reduced:true,walkable:(x,z)=>Math.abs(z)>8});
  flight.start({x:0,y:2.9,z:0});const lifted=flight.step(dt,{});assert.equal(lifted.y,2.9+hoverLimits.takeoff,'Takeoff completes in one step');
  run(flight,1,{up:1});assert.equal(flight.land(),true);const end=flight.state();
  assert.ok(end.done&&end.y===.5+hoverLimits.eye&&Math.abs(end.z)>8,'Landing is instant on a walkable spot');
  const forced=world();takeoff(forced.flight,{x:0,y:2.9,z:0});run(forced.flight,2,{up:1});assert.ok(forced.flight.land(true));assert.ok(forced.flight.state().done,'An instant landing needs no glide');
}

// ---- The five docks in the real city ----
const original={fetch:globalThis.fetch,localStorage:globalThis.localStorage};
globalThis.localStorage={getItem:()=>null,setItem:()=>{}};
const real=JSON.parse(await fs.readFile('ui/3d/system-world/v2/manifest.json','utf8')),v1=JSON.parse(await fs.readFile('ui/3d/system-world/v1/manifest.json','utf8')),v3=JSON.parse(await fs.readFile('ui/3d/system-world/v3/manifest.json','utf8'));
const realAsset=real.assets.find(a=>a.id==='hover-pack');
// Until the Blender asset is exported the contract is served from a synthetic one: a dock holding a
// removable 'pack' (fan_l, fan_r), one 'idle' clip, a 1.4 x 2.1 x 1.0 m collider and an interaction spot.
function syntheticGLB(){
  const bin=Buffer.alloc(40);bin.writeFloatLE(0,0);bin.writeFloatLE(2,4);[0,0,0,1,0,.866,0,.5].forEach((v,i)=>bin.writeFloatLE(v,8+i*4));
  const doc={asset:{version:'2.0'},scene:0,scenes:[{nodes:[0]}],nodes:[{name:'hover-pack',children:[1,2]},{name:'dock'},{name:'pack',children:[3,4]},{name:'fan_l'},{name:'fan_r'}],
    animations:[{name:'idle',samplers:[{input:0,output:1,interpolation:'LINEAR'}],channels:[{sampler:0,target:{node:3,path:'rotation'}}]}],
    accessors:[{bufferView:0,componentType:5126,count:2,type:'SCALAR',min:[0],max:[2]},{bufferView:1,componentType:5126,count:2,type:'VEC4'}],
    bufferViews:[{buffer:0,byteOffset:0,byteLength:8},{buffer:0,byteOffset:8,byteLength:32}],buffers:[{byteLength:40}]};
  let json=Buffer.from(JSON.stringify(doc));json=Buffer.concat([json,Buffer.alloc((4-json.length%4)%4,32)]);
  const head=Buffer.alloc(12),jh=Buffer.alloc(8),bh=Buffer.alloc(8);head.write('glTF',0);head.writeUInt32LE(2,4);head.writeUInt32LE(12+8+json.length+8+bin.length,8);
  jh.writeUInt32LE(json.length,0);jh.write('JSON',4);bh.writeUInt32LE(bin.length,0);bh.write('BIN\0',4);return Buffer.concat([head,jh,json,bh,bin]);
}
const synthetic=syntheticGLB(),syntheticEntry={id:'hover-pack',lods:[0,1,2].map(level=>({level,file:'hover-pack.lod'+level+'.glb',bytes:synthetic.length,sha256:'',bounds:{min:[-.7,0,-.5],max:[.7,2.1,.5]},triangles:0})),
  navigation:{colliders:[[-.7,0,-.5,.7,2.1,.5]],surfaces:[],portals:[],interaction:[{id:'use',position:[0,1.5,1.2]}]},animations:['idle'],components:[]};
async function city({withPack,reduced=false}){
  globalThis.fetch=async name=>{
    const file=String(name).split('/').at(-1);assert.match(file,/^[\w.-]+$/);
    let bytes;
    if(file==='manifest.json'){const m=JSON.parse(JSON.stringify(real));m.assets=m.assets.filter(a=>a.id!=='hover-pack');if(withPack)m.assets.push(realAsset||syntheticEntry);bytes=Buffer.from(JSON.stringify(m));}
    else if(/^hover-pack\.lod\d\.glb$/.test(file)&&!realAsset)bytes=synthetic;else bytes=await fs.readFile('ui/3d/system-world/v2/'+file);
    return {ok:true,json:async()=>JSON.parse(bytes),arrayBuffer:async()=>bytes.buffer.slice(bytes.byteOffset,bytes.byteOffset+bytes.byteLength)};
  };
  const traffic=createTraffic(),scene=new THREE.Scene(),camera=new THREE.PerspectiveCamera(),errors=[],flights=[],sounds=[],state={reduced,ready:false};
  installCityColliders(traffic,kitCatalog(v1,v3),districts,placements,kitPlacements);
  const pad=hoverPads[0],stand={x:pad.x+1.2*Math.sin(pad.angle),z:pad.z+1.2*Math.cos(pad.angle)};camera.position.set(stand.x,2.9,stand.z);
  const visitor=traffic.register('visitor',walker,camera.position,100);assert.ok(visitor,'The visitor fits at the first dock');
  const experience=createExperience(scene,{camera,districts,tier:'high',traffic,assetURL:x=>x,reduced:()=>state.reduced,active:()=>true,onError:e=>errors.push(e),onReady:()=>{state.ready=true;},
    clear:(a,b)=>traffic.clear(visitor,pose(a),pose(b),false),onFlight:s=>flights.push(s),onSound:(kind)=>sounds.push(kind)});
  const wait=async check=>{for(let n=0;n<1500;n++){if(errors.length)throw errors[0];if(check())return;await new Promise(r=>setTimeout(r,10));}throw Error('World did not load');};
  // The scene's fixed step: move first, then update, then the visitor's own proposal.
  const tick=(seconds=dt,input={},mode='street')=>{for(let t=0;t<seconds-1e-9;t+=dt){traffic.begin();if(experience.hovering())experience.fly(dt,input);experience.update(dt,true,mode);traffic.propose(visitor,{...camera.position,heading:0},(_,p)=>camera.position.set(p.x,p.y,p.z));traffic.solve(dt);}};
  const put=(x,y,z)=>{camera.position.set(x,y,z);Object.assign(visitor,{x,y,z,heading:0});};
  return {traffic,scene,camera,visitor,experience,errors,flights,sounds,state,wait,tick,put,stand:p=>({x:p.x+1.2*Math.sin(p.angle),z:p.z+1.2*Math.cos(p.angle)})};
}
// Clearance of a point: the widest probe circle that touches no solid (buildings, lamps, racks, stations, furnishings...).
const clearance=(traffic,x,z)=>{
  const probe=r=>traffic.clear({id:'probe',circles:[{x:0,z:0,r}],minY:0,maxY:2.5,reach:r},{x,y:.6,z,heading:0},{x,y:.6,z,heading:0},false);
  let lo=0,hi=12;if(!probe(.05))return 0;for(let i=0;i<10;i++){const m=(lo+hi)/2;if(probe(m))lo=m;else hi=m;}return lo;
};
const route=streetCurve(tramWaypoints),railPoints=Array.from({length:3000},(_,i)=>route.getPointAt(i/3000));
const lane=(x,z)=>streetAt(x,z,streets.laneHalfWidth);
// A visitor reaches a carriageway from the standing spot on open pavement (8-neighbour flood fill, 0.5 m cells).
function reachesLane(traffic,x0,z0){
  const seen=new Set([x0+','+z0]),queue=[[x0,z0]];
  while(queue.length&&seen.size<1500){
    const [x,z]=queue.shift();if(lane(x,z))return true;
    for(const [dx,dz]of[[.5,0],[-.5,0],[0,.5],[0,-.5],[.5,.5],[-.5,.5],[.5,-.5],[-.5,-.5]]){
      const nx=x+dx,nz=z+dz,key=nx+','+nz;if(seen.has(key)||Math.hypot(nx-x0,nz-z0)>14)continue;seen.add(key);
      if(canWalk(nx,nz,{x,z},()=>false,districts)&&traffic.clear({id:'v',...walker},{x,y:2.9,z,heading:0},{x:nx,y:2.9,z:nz,heading:0},false))queue.push([nx,nz]);
    }
  }
  return false;
}
try{
  // Without the asset the city places no pads and offers nothing, without errors.
  {
    const c=await city({withPack:false});await c.wait(()=>c.state.ready);
    assert.equal(c.experience.stats().hover.pads,0);assert.equal(c.experience.stats().hover.docked,0);assert.equal(c.errors.length,0);
    for(const p of hoverPads){const s=c.stand(p);c.put(s.x,2.9,s.z);c.tick(dt);assert.notEqual(c.experience.interaction()?.kind,'hoverpack','No pack, no prompt: '+p.id);}
    // The five docks stand on open pavement, well apart from each other and from every other placement.
    assert.equal(hoverPads.length,5);assert.equal(new Set(hoverPads.map(p=>p.id)).size,5);
    for(const [i,p] of hoverPads.entries()){
      const s=Math.sin(p.angle),k=Math.cos(p.angle),stand=c.stand(p);
      const corners=[[-.7,-.5],[.7,-.5],[-.7,.5],[.7,.5],[0,0]].map(([dx,dz])=>[p.x+dx*k+dz*s,p.z-dx*s+dz*k]);
      for(const [x,z]of[...corners,[stand.x,stand.z]]){
        assert.ok(canWalk(x,z,{x,z},()=>false,districts)&&!roomAt(x,z),`${p.id}: inside a building footprint at ${x},${z}`);
        assert.ok(!lane(x,z),`${p.id}: on a road lane at ${x},${z}`);
        assert.ok(Math.abs(groundHeight(x,z)-groundHeight(p.x,p.z))<1e-9,`${p.id}: uneven ground at ${x},${z}`);
      }
      assert.ok(railPoints.every(r=>Math.hypot(r.x-stand.x,r.z-stand.z)>2.5&&Math.hypot(r.x-p.x,r.z-p.z)>3),`${p.id}: too close to the tram rails`);
      assert.ok(clearance(c.traffic,p.x,p.z)>=3,`${p.id}: another placement within 3 m (${clearance(c.traffic,p.x,p.z).toFixed(2)} m)`);
      assert.ok(clearance(c.traffic,stand.x,stand.z)>=1.5,`${p.id}: the standing spot is crowded`);
      assert.ok(Math.hypot(p.x-dronePad.x,p.z-dronePad.z)>=6,`${p.id}: on the drone pad`);
      // Docks stay off the zebra crossings: no pedestrian node within 2 m of a dock centre.
      for(const n of pedestrianGraph().nodes)assert.ok(Math.hypot(n.x-p.x,n.z-p.z)>=2,`${p.id}: blocks the pavement node at ${n.x}, ${n.z}`);
      assert.ok(c.traffic.clear({id:'v',...walker},{x:stand.x,y:2.9,z:stand.z,heading:0},{x:stand.x,y:2.9,z:stand.z,heading:0},false),`${p.id}: the visitor cannot stand in front of the dock`);
      assert.ok([1.5,2.5,3.5,4.5,5.5].some(d=>lane(p.x+d*s,p.z+d*k)),`${p.id}: does not face the street`);
      assert.ok(reachesLane(c.traffic,stand.x,stand.z),`${p.id}: not reachable on foot from a street`);
      for(const q of hoverPads.slice(i+1))assert.ok(Math.hypot(p.x-q.x,p.z-q.z)>=30,`${p.id} and ${q.id} are too close`);
    }
    c.experience.dispose();
  }
  // With the asset: five docked packs, the prompt, takeoff, flight, the landing prompt, landing and re-docking.
  {
    const c=await city({withPack:true});await c.wait(()=>c.experience.stats().hover.pads===5);await c.wait(()=>c.state.ready);
    const packs=()=>{const list=[];c.scene.traverse(n=>{if(n.name==='pack')list.push(n);});return list;},mixers=c.experience.stats();
    assert.equal(mixers.hover.docked,5);assert.equal(packs().length,5,'Every dock holds a removable pack');assert.ok(packs().every(n=>n.visible));
    for(const p of hoverPads){const s=c.stand(p);c.put(s.x,2.9,s.z);c.tick(dt);const near=c.experience.interaction();assert.equal(near?.kind,'hoverpack',p.id+' offers its pack');assert.equal(near.id,p.id);assert.ok(reachesLane(c.traffic,s.x,s.z),p.id+': the dock leaves the way to the street open');}
    // A reduced-motion visitor is offered no pack.
    {const s=c.stand(hoverPads[1]);c.put(s.x,2.9,s.z);c.state.reduced=true;c.tick(dt);assert.notEqual(c.experience.interaction()?.kind,'hoverpack','No hover pack under reduced motion');c.state.reduced=false;}
    // Strap it on at the harbour dock.
    const pad=hoverPads[1],s=c.stand(pad);c.put(s.x,2.9,s.z);c.camera.lookAt(s.x,2.9,s.z-10);c.tick(dt);
    assert.equal(c.experience.interaction().id,pad.id);c.experience.interact();
    assert.ok(c.experience.hovering()&&c.experience.isRiding()&&c.experience.rideBody()===null);
    let hover=c.experience.stats();assert.equal(hover.ride,'hover');assert.equal(hover.hover.docked,4);assert.equal(hover.hover.flying,true);assert.equal(packs().filter(n=>!n.visible).length,1,'The pad\'s pack leaves its dock');
    assert.ok(c.sounds.includes('hover'),'Takeoff sound');assert.deepEqual(c.flights.at(-1),{altitude:0,speed:0,landing:false});
    c.tick(.9);assert.ok(Math.abs(c.camera.position.y-(2.9+hoverLimits.takeoff))<.05,'Takeoff rises 1.5 m: '+c.camera.position.y);
    assert.ok(c.experience.interaction()?.kind==='land','While hovering the prompt is Land');assert.equal(c.experience.walkRide(1,.1),true,'The walking code stays out of a ride');assert.equal(c.experience.move(0,0,c.camera.position),false);
    const z0=c.camera.position.z;c.tick(1.5,{forward:1});assert.ok(c.camera.position.z<z0-5,'Forward flies where the visitor looks: '+(c.camera.position.z-z0));
    c.tick(3,{up:1});assert.ok(c.camera.position.y>15,'Climbing: '+c.camera.position.y);
    const last=c.flights.at(-1);assert.ok(last.altitude>12&&last.speed>=0&&last.landing===false);assert.ok(c.experience.stats().hover.altitude>12);
    const counted=c.flights.length;c.tick(.5,{});c.tick(.5,{});assert.ok(c.flights.length-counted<=60,'Reports only when the rounded values change');
    c.experience.interact();assert.equal(c.experience.stats().hover.landing,true);assert.equal(c.flights.at(-1).landing,true);
    c.tick(dt);assert.equal(c.experience.interaction(),null,'No prompt while landing');
    c.tick(.3);c.experience.interact();assert.equal(c.experience.stats().hover.landing,false,'A second E cancels the landing');assert.equal(c.flights.at(-1).landing,false);
    c.tick(dt);assert.equal(c.experience.interaction()?.kind,'land','...and the Land prompt is back');c.experience.interact();assert.equal(c.experience.stats().hover.landing,true);
    for(let i=0;i<4000&&c.experience.stats().ride;i++)c.tick(.05);
    hover=c.experience.stats();assert.equal(hover.ride,null,'Landing ends the ride');assert.equal(hover.hover.docked,5,'The pack is back on its own dock');assert.ok(packs().every(n=>n.visible));
    assert.equal(c.flights.at(-1),null,'The HUD is told when the flight ends');
    const end=c.camera.position;assert.ok(Math.abs(end.y-(groundHeight(end.x,end.z)+hoverLimits.eye))<1e-6,'Standing on the street again: '+end.y);
    assert.ok(c.experience.move(end.x,end.z,end),'The landing spot is walkable');assert.equal(c.sounds.filter(k=>k==='hover').length,2,'Takeoff and landing sounds');
    // Another ride ends cleanly from the outside (Escape, a visit, reduced motion): instant landing, pack docked.
    c.put(c.stand(hoverPads[2]).x,2.9,c.stand(hoverPads[2]).z);c.tick(dt);c.experience.interact();c.tick(2.5,{forward:1,up:1});
    // A teleporting landing writes its pose into the visitor body, so the next traffic step does not sweep from the air.
    const seated=what=>{const p=c.camera.position,b=c.visitor;assert.ok(Math.abs(b.x-p.x)+Math.abs(b.y-p.y)+Math.abs(b.z-p.z)<1e-6,what+': the body stays in the air: '+JSON.stringify([b.x,b.y,b.z,p.toArray()]));c.tick(.1);assert.ok(Math.abs(p.y-(groundHeight(p.x,p.z)+hoverLimits.eye))<1e-6,what+': the camera snapped back up: '+p.y);};
    assert.ok(c.camera.position.y>8,'Airborne before the ride ends: '+c.camera.position.y);assert.ok(c.visitor.y>8,'The body followed the flight');c.experience.endRide();seated('endRide');
    hover=c.experience.stats();assert.equal(hover.ride,null);assert.equal(hover.hover.docked,5,'endRide re-docks the pack');assert.equal(c.flights.at(-1),null);
    const after=c.camera.position;assert.ok(Math.abs(after.y-(groundHeight(after.x,after.z)+hoverLimits.eye))<1e-6&&c.experience.move(after.x,after.z,after),'endRide puts the visitor on walkable ground');
    // A visit from the air and the reduced-motion switch behave the same.
    c.put(c.stand(hoverPads[3]).x,2.9,c.stand(hoverPads[3]).z);c.tick(dt);c.experience.interact();c.tick(1,{up:1});c.experience.visit('infra');assert.equal(c.experience.stats().hover.docked,5);assert.equal(c.experience.stats().ride,null);
    c.put(c.stand(hoverPads[4]).x,2.9,c.stand(hoverPads[4]).z);c.tick(dt);c.experience.interact();c.tick(1,{up:1});c.state.reduced=true;c.experience.endRide();assert.equal(c.experience.stats().hover.docked,5);seated('reduced-motion switch');
    // E under reduced motion (the preference changed in flight) lands at once, with the body seated too.
    c.put(c.stand(hoverPads[0]).x,2.9,c.stand(hoverPads[0]).z);c.state.reduced=false;c.tick(dt);c.experience.interact();c.tick(2,{up:1});c.state.reduced=true;c.experience.interact();
    assert.equal(c.experience.stats().ride,null,'E under reduced motion lands at once');seated('instant E');c.state.reduced=false;
    assert.equal(c.errors.length,0);c.experience.dispose();
  }
  console.log('Hover pack: takeoff, easing, boost, collision sliding, floor/ceiling/limits, landing glide, reduced motion, 5 docks on open pavement (rules checked against the real colliders), prompt, ride, re-docking passed'+(realAsset?' with the exported asset.':' with the synthetic asset contract.'));
}finally{globalThis.fetch=original.fetch;globalThis.localStorage=original.localStorage;}
