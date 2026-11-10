// Pure registry of the enterable district buildings (System World beauty pass, phase 3): podiums, stair
// flights, lobby rooms and doors, the 0.35 m step rule, wall/lintel/ceiling solids, door states and the
// residents' lobby graph. World metres, Y up; every building front faces +Z (south). The numbers mirror
// assets/system-world/facade_assets3.py (PODIUM, STEPS, TREAD), district_assets3.py and interior_assets3.py;
// scripts/test-system-world-lobby-contract.mjs checks them against the packed GLBs. `interiors` in
// sysworld-layout.js stays the list of waterfront pavilions.
export const PODIUM=.88,STEPS=5,TREAD=.32,FLIGHT=STEPS*TREAD,RISER=PODIUM/(STEPS+1),FLOOR=PODIUM+.02;
// Wall band inside the facade line, frames/piers outside it, corner piers, atrium pier half width, leaf inset.
export const WALL=.6,WALL_OUT=.25,PIER=1.2,PIER_HALF=.35,LEAF_INSET=.25,SOLID_TOP=8;
export const MAX_STEP=.35,STEP_SAMPLE=.25,SENSE=3.2,DOOR_TIME=.6,PASS=.85,LOAD_DISTANCE=45,HIDE_DISTANCE=70,GRID=.5,FOOT=.9,TIE=1.2,SPUR=3;
const ATRIUM={sides:12,radius:9.45,wall:4.7,cap:.4,portal:.1};
// Foundry cooling towers: 2.7 m plinths; each pipe runs 3.7 m towards the street (0.36 m thick, 2 m high).
const TOWER={radius:2.7,pipe:.4,reach:4.1,top:2.4};
// Spire corner struts lean through the lobby corners (radius 9 m at the floor, 7.4 m at 7 m).
const STRUT={from:7.3,to:9.3,half:.45,top:5.9};
export const PODIUMS=Object.freeze([
  {id:'agent',x0:-11.5,z0:-23.5,x1:11.5,z1:-.5,stairs:[{x:0,width:7.5}]},
  {id:'memory',x0:-56,z0:-21,x1:-30,z1:1,stairs:[{x:-48,width:4.2},{x:-38,width:4.2}]},
  {id:'integrations',x0:30,z0:-18,x1:56,z1:-2,stairs:[{x:35,width:3.4},{x:51,width:3.4}]},
  {id:'missions',x0:28,z0:25.5,x1:58,z1:46.5,stairs:[{x:43,width:23}]},
  {id:'infra',x0:-55.5,z0:-64.5,x1:-30.5,z1:-41.5,stairs:[{x:-46.75,width:3},{x:-39.25,width:3}]},
  {id:'graph',x0:-55,z0:24,x1:-31,z1:48,stairs:[{x:-43,width:4.4}]},
  {id:'operations',x0:-4.5,z0:31.5,x1:4.5,z1:40.5,stairs:[{x:0,width:2.2}]},
].map(p=>Object.freeze(p)));
// Doors: [node number in the interior asset, x, width, height]; ascending x.
const TABLE=[
  {id:'agent',district:'agent',asset:'lobby-agent',role:'technician',origin:[0,-12],rect:[-7.5,-19.5,7.5,-4.5],ceiling:5,doors:[[0,0,4,3.5]],struts:true},
  {id:'memory',district:'memory',asset:'lobby-memory',role:'archivist',origin:[-43,-10],rect:[-54,-20,-32,-2],ceiling:6.1,doors:[[0,-48,2.8,3.5],[1,-38,2.8,3.5]]},
  {id:'integrations-west',district:'integrations',asset:'lobby-integrations',role:'technician',origin:[43,-10],rect:[32,-14.5,38,-5.5],ceiling:4.3,doors:[[0,35,2,2.92]]},
  {id:'integrations-east',district:'integrations',asset:'lobby-integrations',role:'technician',origin:[43,-10],rect:[48,-14.5,54,-5.5],ceiling:4.3,doors:[[1,51,2,2.92]]},
  {id:'missions',district:'missions',asset:'lobby-missions',role:'archivist',origin:[43,36],rect:[30,27.5,56,42.5],ceiling:6.1,doors:[[0,34,4,3.5],[1,40,4,3.5],[2,46,4,3.5],[3,52,4,3.5]]},
  {id:'infra',district:'infra',asset:'lobby-infra',role:'technician',origin:[-43,-53],rect:[-53,-62,-33,-50],ceiling:4.5,doors:[[0,-46.75,2.4,3.12],[1,-39.25,2.4,3.12]],towers:[[-50.5,-48],[-43,-48],[-35.5,-48]]},
  {id:'graph',district:'graph',asset:'lobby-graph',role:'archivist',origin:[-43,36],circle:{x:-43,z:36,...ATRIUM},ceiling:null,open:true,doors:[[0,-43,3.2,3.1]]},
];
const apothem=c=>c.radius*Math.cos(Math.PI/c.sides);
export const LOBBIES=Object.freeze(TABLE.map(t=>{
  const front=t.circle?t.circle.z+apothem(t.circle):t.rect[3];
  return Object.freeze({...t,open:!!t.open,front,leaf:front-LEAF_INSET+(t.circle?ATRIUM.portal:0),
    doors:Object.freeze(t.doors.map(([n,x,width,height])=>Object.freeze({n,x,width,height})))});
}));
export const DOORS=Object.freeze(LOBBIES.flatMap(l=>l.doors.map(d=>Object.freeze({...d,key:'lobby:'+l.id+':'+d.n,lobby:l.id,asset:l.asset,district:l.district,z:l.leaf}))));
export const LOBBY_DISTRICTS=new Set(LOBBIES.map(l=>l.district));
const NORMALS=Array.from({length:ATRIUM.sides},(_,k)=>[Math.sin(k*2*Math.PI/ATRIUM.sides),Math.cos(k*2*Math.PI/ATRIUM.sides)]);
// Distance beyond the facade line (negative inside): the largest overshoot over any face.
function reach(l,x,z){
  if(l.circle){const dx=x-l.circle.x,dz=z-l.circle.z;let m=-Infinity;for(const [s,c]of NORMALS)m=Math.max(m,dx*s+dz*c);return m-apothem(l.circle);}
  const [x0,z0,x1,z1]=l.rect;return Math.max(x0-x,x-x1,z0-z,z-z1);
}
const corner=(l,x,z)=>{const [x0,z0,x1,z1]=l.rect;return Math.min(x-x0,x1-x)<PIER&&Math.min(z-z0,z1-z)<PIER;};
const inGap=(l,x,z)=>{const f=z-l.front;return f>=-WALL&&f<=WALL_OUT&&l.doors.some(d=>Math.abs(x-d.x)<d.width/2);};
export const lobbyBounds=l=>l.rect?[...l.rect]:[l.circle.x-l.circle.radius,l.circle.z-l.circle.radius,l.circle.x+l.circle.radius,l.circle.z+l.circle.radius];
export const clearRoom=l=>{const [x0,z0,x1,z1]=l.rect;return [x0+WALL,z0+WALL,x1-WALL,z1-WALL];};
export function podiumAt(x,z){return PODIUMS.find(p=>x>=p.x0&&x<=p.x1&&z>=p.z0&&z<=p.z1)||null;}
// Podium top, or the tread under a stair flight (STEPS treads of TREAD, STEPS + 1 equal risers); null elsewhere.
export function lobbyGround(x,z){
  if(podiumAt(x,z))return PODIUM;
  for(const p of PODIUMS){const s=z-p.z1;if(s<=0||s>=FLIGHT)continue;
    for(const f of p.stairs)if(Math.abs(x-f.x)<=f.width/2)return PODIUM-RISER*(Math.min(STEPS-1,Math.floor(s/TREAD))+1);}
  return null;
}
// Feet on the flights (phase 6): the z of the tread centre under (x, z) on a stair flight; on the podium within one tread of
// a flight's top edge, the furthest z that keeps a resident sole (FOOT_HALF, half of the 0.41 m sole at scale 1.15)
// behind the edge; NaN elsewhere. Pure.
export const FOOT_HALF=.21;
export function treadCentre(x,z){
  for(const p of PODIUMS){const s=z-p.z1;if(s<=-TREAD||s>=FLIGHT)continue;
    for(const f of p.stairs)if(Math.abs(x-f.x)<=f.width/2)return s<=0?Math.min(z,p.z1-FOOT_HALF):p.z1+(Math.min(STEPS-1,Math.floor(s/TREAD))+.5)*TREAD;}
  return NaN;
}
// The lobby whose clear room (facade line minus WALL) holds the point.
export function lobbyAt(x,z){return LOBBIES.find(l=>reach(l,x,z)<=-WALL)||null;}
export function lobbyAirspace(x,y,z){return y<SOLID_TOP&&!!lobbyAt(x,z);}
export function lobbyWall(x,z){
  for(const l of LOBBIES){const r=reach(l,x,z);if(r>WALL_OUT)continue;
    if(r>-WALL){if(!inGap(l,x,z))return true;continue;}
    if(l.rect&&corner(l,x,z))return true;}
  return false;
}
const nearPodium=(a,b,m)=>PODIUMS.some(p=>Math.max(a.x,b.x)>=p.x0-m&&Math.min(a.x,b.x)<=p.x1+m&&Math.max(a.z,b.z)>=p.z0-m&&Math.min(a.z,b.z)<=p.z1+FLIGHT+m);
// Nobody takes a step higher than MAX_STEP between samples at most STEP_SAMPLE apart. Away from the
// podiums every ground change (kerb 0.14, platform 0.30) is lower, so only links near them are sampled.
export function stepFree(a,b,height){
  if(!nearPodium(a,b,1))return true;
  const n=Math.max(1,Math.ceil(Math.hypot(b.x-a.x,b.z-a.z)/STEP_SAMPLE));let last=height(a.x,a.z);
  for(let i=1;i<=n;i++){const t=i/n,h=height(a.x+(b.x-a.x)*t,a.z+(b.z-a.z)*t);if(Math.abs(h-last)>MAX_STEP)return false;last=h;}
  return true;
}
// Visitor move: never into a wall band, and across a leaf line only through an open door.
export function lobbyPassage(from,to,doorOpen){
  const n=Math.max(1,Math.ceil(Math.hypot(to.x-from.x,to.z-from.z)/STEP_SAMPLE));
  for(let i=0;i<=n;i++){const t=i/n;if(lobbyWall(from.x+(to.x-from.x)*t,from.z+(to.z-from.z)*t))return false;}
  for(const d of DOORS){
    if(Math.sign(from.z-d.z)===Math.sign(to.z-d.z))continue;
    const t=to.z===from.z?0:(d.z-from.z)/(to.z-from.z),x=from.x+(to.x-from.x)*t;
    if(Math.abs(x-d.x)<d.width/2&&!doorOpen(d.key))return false;
  }
  return true;
}
// Traffic boxes for installCityColliders: walls with door gaps, corner piers, lintels, the ceiling slab
// (atrium: a collision lid at its cap), the foundry towers and pipes, the spire struts.
export function lobbySolids(district){
  const out=[],box=(x0,z0,x1,z1,y0=0,y1=SOLID_TOP)=>out.push({x:0,z:0,heading:0,min:[x0,y0,z0],max:[x1,y1,z1]});
  for(const l of LOBBIES){
    if(l.district!==district)continue;
    if(l.rect){
      const [x0,z0,x1,z1]=l.rect,o=WALL_OUT;
      box(x0-o,z0-o,x1+o,z0+WALL);box(x0-o,z0-o,x0+WALL,z1+o);box(x1-WALL,z0-o,x1+o,z1+o);
      for(const [cx,sx]of[[x0,1],[x1,-1]])for(const [cz,sz]of[[z0,1],[z1,-1]])
        box(Math.min(cx,cx+sx*PIER),Math.min(cz,cz+sz*PIER),Math.max(cx,cx+sx*PIER),Math.max(cz,cz+sz*PIER));
      let start=x0-o;
      for(const d of l.doors){box(start,z1-WALL,d.x-d.width/2,z1+o);box(d.x-d.width/2,z1-WALL,d.x+d.width/2,z1+o,PODIUM+d.height);start=d.x+d.width/2;}
      box(start,z1-WALL,x1+o,z1+o);
      box(x0-o,z0-o,x1+o,z1+o,l.ceiling);
    }else{
      const c=l.circle,A=apothem(c),half=c.radius*Math.sin(Math.PI/c.sides)+PIER_HALF,d=l.doors[0],dx=d.x-c.x;
      for(let k=0;k<c.sides;k++){
        const heading=k*2*Math.PI/c.sides,seg=(a,b,y0=0)=>out.push({x:c.x,z:c.z,heading,min:[a,y0,A-WALL],max:[b,SOLID_TOP,A+WALL_OUT]});
        if(k){seg(-half,half);continue;}
        seg(-half,dx-d.width/2);seg(dx-d.width/2,dx+d.width/2,PODIUM+d.height);seg(dx+d.width/2,half);
      }
      const r=c.radius+WALL_OUT;out.push({x:c.x,z:c.z,heading:0,min:[-r,c.wall+c.cap,-r],max:[r,SOLID_TOP,r]});
    }
    for(const [x,z]of l.towers||[]){box(x-TOWER.radius,z-TOWER.radius,x+TOWER.radius,z+TOWER.radius);box(x-TOWER.pipe,z,x+TOWER.pipe,z+TOWER.reach,0,TOWER.top);}
    if(l.struts)for(const k of[1,3,5,7])out.push({x:l.origin[0],z:l.origin[1],heading:k*Math.PI/4,min:[-STRUT.half,0,STRUT.from],max:[STRUT.half,STRUT.top,STRUT.to]});
  }
  return out;
}
const ease=t=>t*t*(3-2*t);
// Door states. Residents within SENSE open a door on every tier; the visitor only once the interior is
// shown (setReady) and never on low. Opening takes DOOR_TIME (eased), reduced motion snaps.
export function createLobbyDoors(){
  const state=new Map(DOORS.map(d=>[d.key,{door:d,t:0,target:0,held:false}])),ready=new Set();let tier='high';
  const may=d=>tier!=='low'&&ready.has(d.lobby),near=(d,p)=>!!p&&Math.hypot(p.x-d.x,p.z-d.z)<SENSE;
  return {
    setTier(value){tier=value;},
    setReady(lobby,value){if(value)ready.add(lobby);else ready.delete(lobby);},
    usable(key){const s=state.get(key);return !!s&&may(s.door);},
    hold(key){const s=state.get(key);if(!s||!may(s.door))return false;s.held=!s.held;return true;},
    update(dt,visitor,residents=[],reduced=false){
      const flips=[];
      for(const s of state.values()){
        const d=s.door;if(!may(d))s.held=false;
        const want=(may(d)&&(s.held||near(d,visitor)))||residents.some(p=>near(d,p))?1:0;
        if(want!==s.target){s.target=want;flips.push(d);}
        s.t=reduced?want:Math.min(1,Math.max(0,s.t+(want?1:-1)*Math.max(0,dt||0)/DOOR_TIME));
      }
      return flips;
    },
    value(key){const s=state.get(key);return s?ease(s.t):0;},
    passable(key){const s=state.get(key);return !!s&&may(s.door)&&s.t>=PASS;},
    stats:()=>({open:[...state.values()].filter(s=>s.t>=PASS).map(s=>s.door.key),ready:[...ready],tier}),
  };
}
const entryOf=(catalog,id)=>catalog instanceof Map?catalog.get(id):catalog?.assets?.find(a=>a.id===id);
// Workpoints are building-local glTF positions; only those inside this hall's clear room count.
export function workpointsOf(l,catalog){
  return (entryOf(catalog,l.asset)?.navigation?.workpoints||[]).map(w=>({id:w.id,x:l.origin[0]+w.position[0],z:l.origin[1]+w.position[2]}))
    .filter(w=>lobbyAt(w.x,w.z)?.id===l.id);
}
// One living place per interior asset (the place origin is the district origin, as addPlace expects).
export function lobbyPlaces(catalog){
  const out=[],seen=new Set();
  for(const l of LOBBIES){
    const entry=entryOf(catalog,l.asset);if(seen.has(l.asset)||!entry?.navigation?.workpoints?.length)continue;seen.add(l.asset);
    out.push({place:{id:l.asset,district:l.district,x:l.origin[0],z:l.origin[1],role:l.role,lobby:true},entry});
  }
  return out;
}
// Residents' graph: per flight a foot (pavement) and a landing (podium); per door a porch (outside) and an
// entry (inside); per hall a GRID (.5 m) of inside cells with 8-neighbour links, and its workpoints. Entries and
// workpoints tie to every cell within TIE. Inside nodes carry `lobby`. Cells are only geometric (clear of the walls);
// the wayfinder drops what a body shape cannot stand on, walk through, or reach from an entry.
export function lobbyGraph(catalog=null){
  const nodes=[],links=[],add=n=>nodes.push(Object.freeze(n))-1,join=(a,b)=>links.push([a,b]),tops=new Map(PODIUMS.map(p=>[p.id,[]]));
  for(const p of PODIUMS)for(const f of p.stairs){
    const foot=add({x:f.x,z:p.z1+FLIGHT+FOOT,kind:'foot',lobby:null}),landing=add({x:f.x,z:p.z1-FOOT,kind:'landing',lobby:null});
    join(foot,landing);tops.get(p.id).push(landing);
  }
  for(const l of LOBBIES){
    const ties=[],cells=new Map();
    for(const d of l.doors){
      const porch=add({x:d.x,z:l.front+WALL_OUT+FOOT,kind:'porch',lobby:null}),entry=add({x:d.x,z:l.front-WALL-FOOT,kind:'entry',lobby:l.id});
      join(porch,entry);tops.get(l.district).push(porch);ties.push(entry);
    }
    const [x0,z0,x1,z1]=lobbyBounds(l),nx=Math.floor((x1-x0-2*WALL-GRID)/GRID+1e-9),nz=Math.floor((z1-z0-2*WALL-GRID)/GRID+1e-9);
    for(let i=0;i<=nx;i++)for(let k=0;k<=nz;k++){
      const x=x0+WALL+GRID/2+i*GRID,z=z0+WALL+GRID/2+k*GRID;
      if(reach(l,x,z)<=-WALL-.3)cells.set(i+','+k,add({x,z,kind:'grid',lobby:l.id}));
    }
    for(const [key,a]of cells){const [i,k]=key.split(',').map(Number);
      for(const [di,dk]of[[1,0],[0,1],[1,1],[1,-1]]){const b=cells.get((i+di)+','+(k+dk));if(b!==undefined)join(a,b);}}
    for(const w of workpointsOf(l,catalog))ties.push(add({x:w.x,z:w.z,id:w.id,kind:'work',lobby:l.id}));
    // Entries and workpoints rarely sit on the lattice, and an aisle can be narrower than one cell: each sends a
    // spur of cells GRID apart along both axes (SPUR long), tied to the lattice cells beside it.
    const cellList=[...cells.values()],spurs=[];
    for(const t of ties)for(const [dx,dz]of[[1,0],[-1,0],[0,1],[0,-1]]){
      let prev=t;
      for(let k=1;k*GRID<=SPUR+1e-9;k++){
        const x=nodes[t].x+dx*k*GRID,z=nodes[t].z+dz*k*GRID;if(reach(l,x,z)>-WALL-.3)break;
        const n=add({x,z,kind:'grid',lobby:l.id,spur:true});join(prev,n);spurs.push(n);prev=n;
      }
    }
    for(const t of ties)for(const c of cellList)if(Math.hypot(nodes[t].x-nodes[c].x,nodes[t].z-nodes[c].z)<=TIE+1e-9)join(t,c);
    for(const n of spurs)for(const c of cellList)if(Math.hypot(nodes[n].x-nodes[c].x,nodes[n].z-nodes[c].z)<=GRID*.75+1e-9)join(n,c);
    // Entries and workpoints that stand in the same hall see each other directly, too (short hops only).
    for(let a=0;a<ties.length;a++)for(let b=a+1;b<ties.length;b++)
      if(Math.hypot(nodes[ties[a]].x-nodes[ties[b]].x,nodes[ties[a]].z-nodes[ties[b]].z)<=TIE+1e-9)join(ties[a],ties[b]);
  }
  for(const ids of tops.values())for(let i=0;i<ids.length;i++)for(let j=i+1;j<ids.length;j++)join(ids[i],ids[j]);
  return {nodes,links};
}
