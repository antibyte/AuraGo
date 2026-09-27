import assert from 'node:assert/strict';
import {createTraffic,bodyShape,fixedSteps} from '../ui/js/desktop/apps/sysworld-traffic.js';
import * as THREE from 'three';
import {createSociety} from '../ui/js/desktop/apps/sysworld-society.js';
import {createSocialSignals} from '../ui/js/desktop/apps/sysworld-signals.js';

const pedestrian=bodyShape({min:[-.4,0,-.35],max:[.4,2.1,.35]});
const tram=bodyShape({min:[-1.5,0,-4],max:[1.5,3.5,4]});
const drone=bodyShape({min:[-2,-.5,-1],max:[2,.5,1]});
const p=(x,z,y=0,heading=0)=>({x,y,z,heading});
{
  const world=createTraffic();
  world.solid('thin-wall',{x:0,z:0,min:[-.01,0,-5],max:[.01,3,5]});
  const body=world.register('fast',pedestrian,p(-5,0));assert.ok(body);
  world.begin();world.propose(body,p(5,0));world.solve(.25);
  assert.equal(body.x,-5,'Swept motion cannot tunnel through a thin wall');
  world.begin();world.propose(body,p(-5,3));world.solve(.1);assert.equal(body.z,3,'Sliding parallel to a wall remains possible');
  const flying=world.register('flying',drone,p(-5,0,8));assert.ok(flying);
  world.begin();world.propose(flying,p(5,0,8));world.solve(.25);assert.equal(flying.x,5,'Height intervals separate airborne traffic');
}
{
  const world=createTraffic(),a=world.register('a',pedestrian,p(-3,0)),b=world.register('b',pedestrian,p(3,0));
  world.begin();world.propose(a,p(3,0));world.propose(b,p(-3,0));world.solve(.2);
  assert.equal(a.x,-3);assert.equal(b.x,3,'Swapping sides may not pass through another body');
  world.begin();world.propose(a,p(-3,2));world.propose(b,p(3,-2));world.solve(.2);
  assert.equal(a.z,2);assert.equal(b.z,-2,'Separated detours may both proceed');
}
{
  const world=createTraffic(),car=world.register('tram',tram,p(0,0),3),visitor=world.register('visitor',pedestrian,p(0,7),100);
  assert.ok(car&&visitor);
  world.begin();world.propose(car,p(0,5));world.solve(.1);
  assert.equal(car.z,0,'A priority vehicle must stop for a standing visitor');
  world.begin();world.propose(visitor,p(5,7));world.solve(.1);
  world.begin();world.propose(car,p(0,5));world.solve(.1);assert.equal(car.z,5,'Traffic resumes after the obstruction leaves');
  assert.ok(world.register('parallel',pedestrian,p(3,5)),'The full-length tram should not block the adjacent sidewalk');
  const passenger={...pedestrian,id:'passenger',...p(0,5),ignore:'tram'};
  assert.ok(world.clear(passenger,passenger,passenger),'Only the passenger own vehicle is excluded');
  passenger.ignore=null;assert.equal(world.clear(passenger,passenger,passenger),false);
}
{
  const world=createTraffic(),lead=world.register('lead',pedestrian,p(0,0)),middle=world.register('middle',pedestrian,p(-2,0)),rear=world.register('rear',pedestrian,p(-4,0));
  world.solid('stop',{min:[1,0,-2],max:[2,3,2]});
  world.begin();world.propose(lead,p(2,0));world.propose(middle,p(0,0));world.propose(rear,p(-2,0));world.solve(.2);
  assert.deepEqual([lead.x,middle.x,rear.x],[0,-2,-4],'A rejected leader must also stop following intentions');
}
{
  const world=createTraffic(),rear=world.register('tram-0',tram,p(0,0),3),front=world.register('tram-1',tram,p(0,9.53),3);
  assert.ok(rear&&front);
  world.begin();world.propose(rear,p(0,.5));world.propose(front,p(0,9.63));world.solve(.1);
  assert.equal(front.z,9.63,'A follower must not win priority over a slowing leader');
}
{
  const world=createTraffic(),car=world.register('tram',tram,p(0,0),3),walker=world.register('patrol',pedestrian,p(1.8,5.1),1);
  assert.ok(car&&walker);
  world.begin();world.propose(car,p(0,.8));world.propose(walker,p(2.1,5.1));world.solve(.1);
  assert.equal(walker.x,2.1,'A pedestrian escaping the rails must not deadlock with a higher-priority tram');
}
{
  const world=createTraffic(),moving=world.register('door-visitor',pedestrian,p(0,0));
  assert.equal(world.solidClear({min:[-.5,0,-.1],max:[.5,3,.1]}),false,'A closing panel must not sweep through the visitor');
  assert.equal(world.solidClear({min:[2,0,-.1],max:[3,3,.1]}),true);
  let simulated=0;const step=fixedSteps(dt=>{simulated+=dt;});
  for(const dt of [1/120,1/120,1/30,.05])step(dt);
  assert.ok(Math.abs(simulated-.1)<1e-8);step(90);assert.ok(simulated<.21,'Resuming a hidden window cannot replay a long backlog');
  step(0);assert.equal(world.register('overlap',pedestrian,moving),null,'Spawn must be checked, too');
}
{
  // Reproduce a rail bend where another resident blocks a straight sidestep.
  const world=createTraffic(),scene=new THREE.Scene(),camera=new THREE.PerspectiveCamera();
  const shape={circles:[{x:-1.222,z:0,r:2.451},{x:1.218,z:0,r:2.451}],minY:-.4,maxY:6.4,reach:3.673};
  const vehicle=world.register('tram-1',tram,p(-64.2724,-75.8183,.15,-.98866),3);
  const walker=world.register('patrol-4',shape,p(-62.2673,-70.3926,1.6,1.85416));
  const blocker=world.register('resident-6',{circles:[{x:0,z:0,r:1.5}],minY:0,maxY:2.5,reach:1.5},p(-58.2369,-66.3072));
  assert.ok(vehicle&&walker&&blocker);
  const society=createSociety(scene,{traffic:world,camera,floor:()=>0,active:()=>true}),node=new THREE.Group(),start={...walker};
  society.patrol(walker.id,walker,node,()=>{});
  const startTram={...vehicle};let progress=0;
  for(let i=0;i<1200&&progress<1;i++){
    world.begin();society.update(1/60,true);const t=Math.min(1,progress+.003);
    world.propose(vehicle,{...vehicle,x:startTram.x+(-66.5-startTram.x)*t,z:startTram.z+(-72-startTram.z)*t,heading:startTram.heading+(-.25-startTram.heading)*t},accepted=>{if(accepted)progress=t;});
    world.solve(1/60);assert.ok(world.clear(walker,walker,walker));
  }
  assert.ok(Math.hypot(walker.x-start.x,walker.z-start.z)>2,'A blocked rail bend needs a diagonal escape: '+JSON.stringify(walker));
  assert.equal(progress,1,'The waiting tram can continue around the bend');
  society.dispose();world.dispose();
}
{
  const world=createTraffic(),scene=new THREE.Scene(),camera=new THREE.PerspectiveCamera();
  const society=createSociety(scene,{traffic:world,camera,floor:()=>0,active:()=>true});
  const a=society.add({node:new THREE.Group(),play(){}},0,{min:[-.3,0,-.3],max:[.3,2,.3]}),start={...a.body},goal={x:start.x,z:start.z+12};
  camera.position.set(start.x,2,start.z);society.inspect(a.id);assert.ok(society.action('guide',goal).guided);
  for(const [i,box]of [[-2,0,-2,2,3,-1.5],[-2,0,1.5,2,3,2],[-2,0,-2,-1.5,3,2],[1.5,0,-2,2,3,2]].entries())world.solid('gate-'+i,{owner:'gates',x:start.x,z:start.z,min:box.slice(0,3),max:box.slice(3)});
  for(let i=0;i<420;i++){world.begin();society.update(1/60,true);world.solve(1/60);}
  assert.ok(society.stats().residents[0].guided,'A blocked guide waits without abandoning the visitor');
  world.removeOwner('gates');
  for(let i=0;i<1200&&society.stats().residents[0].guided;i++){camera.position.set(a.body.x,2,a.body.z);world.begin();society.update(1/60,true);world.solve(1/60);}
  assert.ok(Math.hypot(a.body.x-goal.x,a.body.z-goal.z)<.5,'The guide resumes and reaches its destination');
  society.dispose();world.dispose();
}
{
  const scene=new THREE.Scene(),signals=createSocialSignals(scene),root=scene.getObjectByName('city-conversations');
  const from={x:0,y:0,z:0,maxY:2,enabled:true},to={x:10,y:0,z:0,maxY:2,enabled:true};
  assert.ok(signals.send(from,to));signals.update(.8);
  const wave=root.children[0],normal=new THREE.Vector3(0,0,1).applyQuaternion(wave.quaternion);
  assert.ok(wave.visible&&wave.position.x>0&&wave.position.x<10&&normal.x>.99,'Wave fronts point towards the receiver');
  to.x=12;signals.update(.85);
  const halo=root.children[3];assert.ok(halo.visible&&halo.position.x===12,'Reception follows the moving receiver');
  const scale=halo.scale.x;signals.update(.2);assert.ok(halo.scale.x<scale,'Reception converges at the receiver');
  signals.clear();assert.ok(signals.send(to,from));signals.update(.8);
  assert.ok(new THREE.Vector3(0,0,1).applyQuaternion(wave.quaternion).x<-.99,'The reply travels in the opposite direction');
  from.enabled=false;signals.update(.1);assert.equal(signals.stats().active,0,'Interrupted encounters release their effects');
  from.enabled=true;for(let i=0;i<8;i++)assert.ok(signals.send(from,to));assert.equal(signals.send(from,to),false,'The effect pool stays bounded');
  signals.update(3);assert.equal(signals.stats().active,0);
  signals.send(from,to);signals.update(0,false);assert.equal(signals.stats().active,0,'Reduced motion clears pending waves');
  signals.dispose();assert.equal(scene.children.length,0);
}
console.log('System World shared traffic: swept walls, opposing actors, height, camera, tram, yielding, doorway, guide resumption, directed exchanges and bounded fixed steps passed.');
