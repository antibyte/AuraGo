// System World ground contract: street profile, continuity, module assembly, slabs and pedestrian graph.
import assert from 'node:assert/strict';
import {streets,surfaces} from '../ui/js/desktop/apps/sysworld-layout.js';
import {kerb,moduleLength,maxLink,island,rampRun,streetLines,streetSegments,streetModules,zebras,streetProfile,plotSlabs,pedestrianGraph,carriagewayFree} from '../ui/js/desktop/apps/sysworld-ground.js';

const R=surfaces.road,P=surfaces.road+kerb.rise,near=(a,b,e=1e-9)=>Math.abs(a-b)<=e;
assert.equal(surfaces.pavement,surfaces.road+kerb.rise,'kerb rise joins road and pavement');
assert.equal(surfaces.pavement,surfaces.ground,'pavements are flush with the plots');

// --- Cross-section on a straight far from any zebra (z = -32 street at x = -45) ---
{
  const at=o=>streetProfile(-45,-32+o);
  for(const o of[0,1.5,3,-3.3])assert.ok(near(at(o).height,R,1e-6)&&at(o).kind==='road','carriageway at '+o);
  assert.ok(near(at(3.3).height,R,1e-6),'kerb line is at road height');   // kind may be 'kerb' by float noise (d ~ 1e-15)
  assert.ok(near(at(3.37).height,R+kerb.rise/2,1e-6)&&at(3.37).kind==='kerb','splayed kerb halfway');
  for(const o of[3.45,3.58,4.75,5.99,-4])assert.ok(near(at(o).height,P,1e-6)&&at(o).kind==='pavement','pavement at '+o);
  assert.equal(streetProfile(-45,-32+6.01),null,'plots are off-street');
}
// --- Dropped kerb and flares at the zebra 8 m west of the x = -18 crossing (x = -26) ---
{
  const at=(s,o)=>streetProfile(-26-s,-32+o);   // s grows away from the intersection
  assert.ok(near(at(0,kerb.line+.6).height,R+kerb.rise/2,1e-6),'1.2 m ramp, halfway up');
  assert.ok(near(at(1.2,kerb.line+1.2).height,P,1e-6),'ramp top at offset 4.5');
  assert.equal(at(0,4.6).kind,'pavement');assert.equal(at(.5,3.9).kind,'kerb');
  assert.ok(near(rampRun(1.6),(kerb.droppedRun+kerb.run)/2,1e-12),'flare midpoint');
  assert.ok(near(rampRun(2),kerb.run,1e-12)&&near(rampRun(5),kerb.run,1e-12)&&near(rampRun(1.2),kerb.droppedRun,1e-12));
  assert.ok(near(at(2.5,kerb.line+.07).height,R+kerb.rise/2,1e-6),'standard kerb beyond the flare');
  // Closed-form values at hand-computed points (offset from the street centre, s from the zebra centre).
  assert.ok(near(at(0,3.9).height,R+kerb.droppedRun*0+.14*.6/1.2,1e-9),'dropped kerb centre at 3.9: road + 0.07');
  // flare midpoint s = 1.6: run = 0.67, offset 3.3 + 0.335 -> half way up
  assert.ok(near(at(1.6,3.3+.335).height,R+kerb.rise/2,1e-9),'flare point at half the local run');
  assert.ok(near(at(1.6,3.3+.67).height,P,1e-9)&&at(1.6,3.3+.67).kind==='pavement','flare top');
}
// --- Rounded corner kerb at the (-18, -32) intersection ---
{
  const r=kerb.cornerRadius,corner=(qx,qz)=>streetProfile(-18+kerb.line+qx,-32+kerb.line+qz);
  assert.equal(corner(0,0).kind,'road');assert.equal(corner(-.2,2).kind,'road','either carriageway is road');
  assert.ok(near(corner(r,r).height,P,1e-6),'corner pavement');
  const k=r-(r-.07)/Math.SQRT2;   // d = .07 on the diagonal
  assert.ok(near(corner(k,k).height,R+kerb.rise/2,1e-6),'rounded kerb halfway');
  assert.ok(near(corner(2,.07).height,R+kerb.rise/2,1e-6),'straight kerb beside the arc');
}
// --- End caps at the island edge and at the quay edge ---
{
  for(const [x,z,dir]of[[-67,streets.minZ,1],[18,streets.quayZ,-1]]){
    const at=e=>streetProfile(x,z+dir*e);
    assert.ok(near(at(.2).height,P,1e-6)&&at(.2).kind==='kerb','end block top');
    assert.ok(near(at(kerb.endDepth+kerb.run/2).height,R+kerb.rise/2,1e-6),'end block inner splay');
    assert.ok(near(at(1).height,R,1e-6),'carriageway beyond the cap');
  }
  assert.equal(streetProfile(18,streets.quayZ+.01),null,'the quay strip is continuous across the street');
  assert.ok(near(streetProfile(streets.maxX-.2,13).height,P,1e-6),'east island edge');
  assert.equal(streetProfile(-62,streets.minZ+.2).kind,'pavement','pavement at the edge stays pavement');
}
// --- Continuity: no step above 2 cm per 1 cm of travel anywhere on the streets ---
{
  let worst=0,where=null;
  const walk=(x0,z0,x1,z1)=>{const n=Math.ceil(Math.hypot(x1-x0,z1-z0)/.01);let prev=null;
    for(let i=0;i<=n;i++){const x=x0+(x1-x0)*i/n,z=z0+(z1-z0)*i/n,p=streetProfile(x,z);
      if(p&&prev){const step=Math.abs(p.height-prev.height);if(step>worst){worst=step;where=[x,z];}}prev=p;}};
  for(const l of streetLines()){
    for(let a=l.from;a<=l.to;a+=.5)l.axis==='x'?walk(a,l.line-6,a,l.line+6):walk(l.line-6,a,l.line+6,a);
    for(const o of[0,3.3,3.37,3.44,3.9,4.5,4.75,5.5,-3.37,-4.5])l.axis==='x'?walk(l.from,l.line+o,l.to,l.line+o):walk(l.line+o,l.from,l.line+o,l.to);
  }
  let seed=1;const rnd=()=>(seed=(Math.imul(seed,1664525)+1013904223)>>>0)/4294967296;
  const centres=[...zebras().map(z=>[z.x,z.z]),...streets.xs.flatMap(x=>streets.zs.map(z=>[x,z]))];
  for(let i=0;i<6000;i++){const [cx,cz]=centres[i%centres.length],x=cx+(rnd()-.5)*12,z=cz+(rnd()-.5)*12,a=rnd()*Math.PI*2;walk(x,z,x+Math.cos(a),z+Math.sin(a));}
  assert.ok(worst<=.02,'profile step '+worst+' m per cm at '+where);
}
// --- Module assembly: every segment covered end to end, no gaps or overlaps ---
{
  const modules=streetModules(),segs=streetSegments();
  assert.equal(modules.filter(m=>m.id==='street-crossing').length,streets.xs.length*streets.zs.length);
  assert.equal(modules.filter(m=>m.id==='street-end').length,2*(streets.xs.length+streets.zs.length));
  assert.equal(zebras().length,2*2*streets.xs.length*streets.zs.length);
  assert.equal(zebras().length,64,'16 intersections x 4 arms');
  for(const s of segs){
    assert.ok(Number.isInteger(s.to-s.from)&&s.to>s.from,'whole-metre segment '+JSON.stringify(s));
    const own=modules.filter(m=>m.axis===s.axis&&m.line===s.line&&m.id!=='street-end'&&m.from>=s.from-1e-9&&m.to<=s.to+1e-9).sort((a,b)=>a.from-b.from);
    assert.ok(near(own[0].from,s.from)&&near(own.at(-1).to,s.to),'segment covered end to end: '+JSON.stringify(s));
    for(let i=1;i<own.length;i++)assert.ok(near(own[i].from,own[i-1].to),'gap or overlap in '+JSON.stringify(s));
    for(const m of own)assert.ok(near(m.to-m.from,m.id==='street-fill'?1:moduleLength),m.id+' length');
    if(s.start==='crossing')assert.equal(own[0].id,'street-zebra');
    if(s.end==='crossing')assert.equal(own.at(-1).id,'street-zebra');
    assert.ok(own.filter(m=>m.id==='street-fill').length<=3);
    const fours=own.filter(m=>m.id==='street-plain'||m.id==='street-drain'),ordered=s.start==='crossing'?fours:[...fours].reverse();
    ordered.forEach((m,k)=>assert.equal(m.id,k%3===2?'street-drain':'street-plain','every third 4 m module is a drain'));
    for(const m of own)if(m.id!=='street-crossing')assert.equal(m.angle,s.axis==='x'?0:Math.PI/2);
  }
  // Per street line: segments, intersection squares and modules tile [from, to] with no gap or overlap,
  // and every module boundary falls on a whole metre.
  for(const l of streetLines()){
    const spans=[...modules.filter(m=>m.axis===l.axis&&m.line===l.line&&m.id!=='street-end').map(m=>[m.from,m.to]),
      ...l.crossings.map(c=>[c-streets.halfWidth,c+streets.halfWidth])].sort((a,b)=>a[0]-b[0]);
    assert.ok(near(spans[0][0],l.from)&&near(spans.at(-1)[1],l.to),'line covered end to end: '+JSON.stringify([l.axis,l.line]));
    for(let i=1;i<spans.length;i++)assert.ok(near(spans[i][0],spans[i-1][1]),`gap or overlap on line ${l.axis}${l.line} at ${spans[i][0]}`);
    for(const [a,b]of spans)assert.ok(Number.isInteger(a)&&Number.isInteger(b),'whole-metre boundaries');
  }
  for(const z of zebras())assert.ok(modules.some(m=>m.id==='street-zebra'&&near(m.x,z.x)&&near(m.z,z.z)),'zebra module centred on every crossing');
  for(const m of modules.filter(m=>m.id==='street-end')){
    const ox=Math.cos(m.angle),oz=-Math.sin(m.angle);   // local +X in world
    assert.equal(streetProfile(m.x+ox*.3,m.z+oz*.3),null,'end cap points out of the street');
    assert.ok(streetProfile(m.x-ox*.3,m.z-oz*.3),'end cap sits on the street end');
  }
  const counts={};for(const m of modules)counts[m.id]=(counts[m.id]||0)+1;
  console.log('module counts',JSON.stringify(counts));
}
// --- Plot slabs and streets cover the island exactly once ---
{
  const slabs=plotSlabs();assert.equal(slabs.length,26);assert.equal(slabs.filter(s=>s.kind==='quay').length,1);
  for(const s of slabs)assert.ok(s.top===surfaces.ground&&s.bottom===island.top,'slab heights');
  for(let x=streets.minX+.25;x<streets.maxX;x+=.5)for(let z=streets.minZ+.25;z<streets.maxZ;z+=.5){
    const covered=slabs.filter(s=>x>s.x0&&x<s.x1&&z>s.z0&&z<s.z1).length+(streetProfile(x,z)?1:0);
    assert.equal(covered,1,`ground cover at ${x}, ${z}`);
  }
}
// --- Pedestrian graph: pavements and zebras only ---
{
  const t0=Date.now(),{nodes,links}=pedestrianGraph(),ms=Date.now()-t0;
  assert.equal(nodes.length,streets.xs.length*streets.zs.length*4+zebras().length*2);
  for(const n of nodes){const p=streetProfile(n.x,n.z);assert.ok(p&&p.kind==='pavement',`node off the pavement: ${n.x}, ${n.z}`);}
  const zebraLinks=links.filter(l=>l.zebra);assert.equal(zebraLinks.length,zebras().length);
  for(const l of zebraLinks){const a=nodes[l.a],b=nodes[l.b];assert.equal(streetProfile((a.x+b.x)/2,(a.z+b.z)/2).kind,'road','zebra links cross the carriageway');}
  for(const l of links){
    if(l.zebra)continue;assert.ok(l.length<maxLink);
    const a=nodes[l.a],b=nodes[l.b],n=Math.ceil(l.length/.02);
    for(let i=0;i<=n;i++){const p=streetProfile(a.x+(b.x-a.x)*i/n,a.z+(b.z-a.z)*i/n);assert.ok(!p||p.kind==='pavement',`link ${l.a}-${l.b} touches a ${p?.kind}`);}
  }
  const adjacency=nodes.map(()=>[]);for(const l of links){adjacency[l.a].push(l.b);adjacency[l.b].push(l.a);}
  const seen=new Set([0]),queue=[0];while(queue.length){const i=queue.pop();for(const j of adjacency[i])if(!seen.has(j)){seen.add(j);queue.push(j);}}
  assert.equal(seen.size,nodes.length,'every pavement node is reachable');
  const n0=nodes[0];assert.ok(carriagewayFree(n0,{x:n0.x,z:n0.z+14}),'node 0 starts a straight pavement run');
  assert.equal(carriagewayFree({x:-45,z:-37},{x:-45,z:-27}),false,'across a street');
  const zb=zebras()[0],side=zb.axis==='x'?{x:zb.at,z:zb.line+kerb.pavementLine}:{x:zb.line+kerb.pavementLine,z:zb.at};
  assert.equal(carriagewayFree({x:zb.x,z:zb.z},side),false);assert.ok(carriagewayFree({x:zb.x,z:zb.z},side,true),'a start on a zebra may step off it');
  console.log(`System World ground: profile, continuity, ${streetModules().length} modules, 26 slabs, ${nodes.length} nodes / ${links.length} links (graph ${ms} ms) verified.`);
}

// --- Resident routes cross carriageways only on zebras ---
{
  const {createWayfinder}=await import('../ui/js/desktop/apps/sysworld-places.js');
  const {createTraffic}=await import('../ui/js/desktop/apps/sysworld-traffic.js');
  const {groundHeight}=await import('../ui/js/desktop/apps/sysworld-exploration.js');
  const {nodes,links}=pedestrianGraph(),finder=createWayfinder(createTraffic(),groundHeight);
  const same=(n,p)=>near(n.x,p.x,1e-9)&&near(n.z,p.z,1e-9);
  const zebraLeg=(a,b)=>links.some(l=>l.zebra&&((same(nodes[l.a],a)&&same(nodes[l.b],b))||(same(nodes[l.a],b)&&same(nodes[l.b],a))));
  // A start standing on a zebra may step off it: every sample of that leg off the pavement must lie in a zebra corridor.
  const inCorridor=(x,z)=>zebras().some(zb=>zb.axis==='x'?Math.abs(x-zb.at)<=kerb.opening&&Math.abs(z-zb.line)<=streets.halfWidth:Math.abs(z-zb.at)<=kerb.opening&&Math.abs(x-zb.line)<=streets.halfWidth);
  const onlyOnZebra=(a,b)=>{const n=Math.ceil(Math.hypot(b.x-a.x,b.z-a.z)/.05);for(let i=0;i<=n;i++){const x=a.x+(b.x-a.x)*i/n,z=a.z+(b.z-a.z)*i/n,p=streetProfile(x,z);if(p&&p.kind!=='pavement'&&!inCorridor(x,z))return false;}return true;};
  // A start on a carriageway or kerb may leave it: leading samples that break the rule are ignored until the leg
  // has reached open pavement once (0.15 m beyond the kerb top); from then on only zebra corridors are exempt.
  const leaveOnly=(a,b)=>{let leaving=true;const n=Math.ceil(Math.hypot(b.x-a.x,b.z-a.z)/.05);for(let i=0;i<=n;i++){const x=a.x+(b.x-a.x)*i/n,z=a.z+(b.z-a.z)*i/n,p=streetProfile(x,z);if(!p||(p.kind==='pavement'&&p.d-p.run>=.15)){leaving=false;continue;}if(!leaving&&!inCorridor(x,z))return false;}return true;};
  const bodyAt=(p)=>({id:'walker',circles:[{x:0,z:0,r:.6}],minY:0,maxY:2,reach:.6,heading:0,...p,y:groundHeight(p.x,p.z)});
  const check=(from,to)=>{
    const path=finder.route(bodyAt(from),{...to,y:groundHeight(to.x,to.z)});
    let prev=from,length=0;
    for(const p of path){
      if(!carriagewayFree(prev,p))assert.ok(zebraLeg(prev,p)||(prev===from&&(onlyOnZebra(prev,p)||leaveOnly(prev,p))),'route crosses a carriageway away from a zebra: '+JSON.stringify([prev,p]));
      length+=Math.hypot(p.x-prev.x,p.z-prev.z);prev=p;
    }
    return {path,length};
  };
  for(const [from,to]of[[{x:-40,z:-50},{x:-40,z:-15}],[{x:-40,z:-50},{x:40,z:30}],[{x:5,z:45},{x:-50,z:70}]])
    assert.ok(check(from,to).path.length,'route found '+JSON.stringify([from,to]));

  // Seeded sample: random plot or pavement points (never on a carriageway).
  let seed=20261109;const rnd=()=>{seed=(seed*1664525+1013904223)>>>0;return seed/4294967296;};
  const walkable=()=>{for(;;){
    const x=streets.minX+rnd()*(streets.maxX-streets.minX),z=streets.minZ+rnd()*(streets.quayZ-streets.minZ),p=streetProfile(x,z);
    if(!p||p.kind==='pavement')return {x,z};
  }};
  let tried=0,resolved=0,detour=0;
  for(let i=0;i<200;i++){
    const from=walkable(),to=walkable(),straight=Math.hypot(to.x-from.x,to.z-from.z);if(straight<10)continue;
    tried++;const {path,length}=check(from,to);
    if(path.length){resolved++;detour+=length/straight;}
  }
  assert.ok(resolved/tried>=.7,`most random routes resolve (${resolved}/${tried})`);
  console.log(`System World routes: ${resolved}/${tried} resolve (${(100*resolved/tried).toFixed(1)}%), mean detour x${(detour/resolved).toFixed(2)}, no leg off a zebra touches a carriageway.`);
}

// --- Lobby routes: up the stairs, through a door, into every hall (real colliders) ---
{
  const fs=await import('node:fs/promises');
  const {createWayfinder}=await import('../ui/js/desktop/apps/sysworld-places.js');
  const {createTraffic}=await import('../ui/js/desktop/apps/sysworld-traffic.js');
  const {groundHeight}=await import('../ui/js/desktop/apps/sysworld-exploration.js');
  const {installCityColliders,kitCatalog}=await import('../ui/js/desktop/apps/sysworld-colliders.js');
  const {districts,placements}=await import('../ui/js/desktop/apps/sysworld-scene.js');
  const {LOBBIES,lobbyAt,stepFree}=await import('../ui/js/desktop/apps/sysworld-lobbies.js');
  const read=f=>fs.readFile(f,'utf8').then(JSON.parse),v1=await read('ui/3d/system-world/v1/manifest.json'),v3=await read('ui/3d/system-world/v3/manifest.json');
  const traffic=createTraffic();installCityColliders(traffic,kitCatalog(v1,v3),districts,placements);
  const finder=createWayfinder(traffic,groundHeight,kitCatalog(v3)),started=Date.now();
  const body=p=>({id:'walker',circles:[{x:0,z:0,r:.45}],minY:0,maxY:2,reach:.45,heading:0,...p,y:groundHeight(p.x,p.z)});
  for(const l of LOBBIES){
    const goal=finder.points.find(p=>p.lobby===l.id&&traffic.clear(body(p),p,p,false));assert.ok(goal,'no free inside node in '+l.id);
    const start=pedestrianGraph().nodes.reduce((a,n)=>Math.hypot(n.x-goal.x,n.z-goal.z)<Math.hypot(a.x-goal.x,a.z-goal.z)?n:a);
    const path=finder.route(body(start),{x:goal.x,z:goal.z,y:groundHeight(goal.x,goal.z)});assert.ok(path.length,'no route into '+l.id);
    let prev=start;for(const p of path){assert.ok(stepFree(prev,p,groundHeight),'leg breaks the step rule: '+JSON.stringify([prev,p]));prev=p;}
    assert.equal(lobbyAt(prev.x,prev.z)?.id,l.id);
  }
  console.log(`System World lobby routes: every hall reached from the pavement over its stairs (${Date.now()-started} ms incl. graph build).`);
}

// --- Living places: every workpoint reaches every other living place (resident-sized body, real colliders) ---
{
  const fs=await import('node:fs/promises');
  const {createWayfinder,livingPlaces}=await import('../ui/js/desktop/apps/sysworld-places.js');
  const {createTraffic}=await import('../ui/js/desktop/apps/sysworld-traffic.js');
  const {groundHeight}=await import('../ui/js/desktop/apps/sysworld-exploration.js');
  const {installCityColliders,kitCatalog}=await import('../ui/js/desktop/apps/sysworld-colliders.js');
  const {districts,placements}=await import('../ui/js/desktop/apps/sysworld-scene.js');
  const read=f=>fs.readFile(f,'utf8').then(JSON.parse),v1=await read('ui/3d/system-world/v1/manifest.json'),v2=await read('ui/3d/system-world/v2/manifest.json'),v3=await read('ui/3d/system-world/v3/manifest.json');
  const traffic=createTraffic();installCityColliders(traffic,kitCatalog(v1,v3),districts,placements);
  const entry=id=>v2.assets.find(a=>a.id===id).navigation;
  for(const p of livingPlaces)entry(p.id).colliders.forEach((b,i)=>traffic.solid('place:'+p.id+':'+i,{x:p.x,z:p.z,min:[b[0],b[1],b[2]],max:[b[3],b[4],b[5]]}));
  const finder=createWayfinder(traffic,groundHeight,kitCatalog(v3)),started=Date.now();
  const resident={id:'walker',circles:[{x:0,z:0,r:1.29}],minY:0,maxY:2,reach:1.29,heading:0};
  const points=p=>entry(p.id).workpoints.map(w=>({x:p.x+w.position[0],z:p.z+w.position[2]}));
  const built=Date.now();finder.route({...resident,x:0,z:0,y:0},{x:1,z:1,y:0});const buildMs=Date.now()-built;
  let routes=0;
  for(const a of livingPlaces)for(const from of points(a)) {
    for(const b of livingPlaces) {
      if(a===b)continue;
      for(const to of points(b)) {
        const start={...resident,...from,y:groundHeight(from.x,from.z)},path=finder.route(start,{...to,y:groundHeight(to.x,to.z)});
        assert.ok(path.length,`no route ${a.id} -> ${b.id}: ${JSON.stringify([from,to])}`);
        let prev=from,length=0;for(const q of path){length+=Math.hypot(q.x-prev.x,q.z-prev.z);prev=q;}
        const straight=Math.hypot(to.x-from.x,to.z-from.z);
        assert.ok(length<=2.5*straight,`route ${a.id} -> ${b.id} is ${length.toFixed(1)} m for ${straight.toFixed(1)} m`);
        routes++;
      }
    }
  }
  console.log(`System World living places: ${routes} routes between all workpoints exist and stay within 2.5x the straight distance (first route incl. street, approach and place nodes: ${buildMs} ms; all routes ${Date.now()-started} ms).`);
}
