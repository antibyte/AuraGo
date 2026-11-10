import {surfaces,streets} from './sysworld-layout.js';
import {pedestrianGraph,carriagewayFree,maxLink,kerb,zebras} from './sysworld-ground.js';
import {lobbyGraph,lobbyAt,lobbyGround,stepFree} from './sysworld-lobbies.js';
// Fixed places belong to the city, never to a changing telemetry inventory.
export const livingPlaces = [
  {id:'repair-bay',district:'infra',x:-26,z:-62,role:'technician'},
  {id:'parcel-sorter',district:'missions',x:27.6,z:49.4,role:'courier'},
  {id:'relay-mast',district:'integrations',x:57.5,z:-23,role:'archivist'},
  {id:'kinetic-fountain',district:'graph',x:-26,z:47,role:'archivist'},
  {id:'glass-garden',district:'graph',x:40,z:-68,role:'technician'},
  {id:'meeting-charge',district:'operations',x:8,z:46,role:'courier'},
];

// Civic landmarks are authored in Blender and keep their own animated parts.
// The gateway stands on the pavement so its arch clears the road; the
// turbine stands on the harbour water below the quay.
export const landmarkPlaces = [
  {id:'data-monument',x:0,z:24,y:0,angle:0,clip:'spin'},
  {id:'neon-gate',x:-18,z:0,y:surfaces.pavement,angle:0,clip:'pulse'},
  {id:'sky-pod',x:56,z:47,y:0,angle:0,clip:'scan'},
  {id:'tidal-turbine',x:0,z:96,y:-3.2,angle:Math.PI,clip:'spin'},
];

// carriagewayFree exempts every zebra corridor along a leg; a start on one zebra may therefore only take
// legs short enough to cross that single street (street band 12 m), never a long diagonal over another crossing.
const stepOff=2*streets.halfWidth;
// True when a point stands in a zebra corridor (opening wide, across the whole street band).
function onZebra(p) {
  return zebras().some(zb=>zb.axis==='x'
    ?Math.abs(p.x-zb.at)<=kerb.opening&&Math.abs(p.z-zb.line)<=streets.halfWidth
    :Math.abs(p.z-zb.at)<=kerb.opening&&Math.abs(p.x-zb.line)<=streets.halfWidth);
}

// Wander targets (streetPoints) are pavement and zebra nodes only: never a stair foot, landing or door porch.
// Residents walk the pavement graph of sysworld-ground.js plus the lobby graph of sysworld-lobbies.js (stair
// feet and landings, porches, entries, inside grid and workpoints). Every static link keeps the 0.35 m step
// rule (built once, on first use); bodies and solids filter links per revision. Inside a lobby a route links
// only to that lobby's inside nodes; outside never to an inside node, so the doors are the only way in.
export function createWayfinder(traffic, floor, catalog=null) {
  const graph=pedestrianGraph(),lobby=lobbyGraph(catalog),base=graph.nodes.length;
  const points=[...graph.nodes,...lobby.nodes].map(n=>({x:n.x,z:n.z,y:floor(n.x,n.z),heading:0,lobby:n.lobby||null,kind:n.kind,cell:n.kind==='grid'||undefined})),cache=new Map();
  let revision=-1,fixed=null;
  function staticLinks(){
    if(fixed)return fixed;
    const length=(a,b)=>Math.hypot(points[a].x-points[b].x,points[a].z-points[b].z);
    fixed=graph.links.filter(l=>stepFree(points[l.a],points[l.b],floor)).map(l=>({a:l.a,b:l.b,length:l.length}));
    for(const [a,b]of lobby.links)if(stepFree(points[base+a],points[base+b],floor))fixed.push({a:base+a,b:base+b,length:length(base+a,base+b)});
    lobby.nodes.forEach((n,k)=>{if(n.kind!=='foot')return;const f=base+k;
      for(let i=0;i<base;i++){const d=length(f,i);if(d<maxLink&&carriagewayFree(points[f],points[i])&&stepFree(points[f],points[i],floor))fixed.push({a:f,b:i,length:d});}});
    return fixed;
  }
  // Approach nodes: around every living place a 2 m cell grid (workpoints included, one level, off podiums,
  // halls and carriageways), kept only where the cells connect to the pavement graph. Built once per body shape.
  const CELL=2,REACH=22;
  function approach(body,nodes,edges,index) {
    const street=[];index.forEach((n,k)=>{if(k<base)street.push(n);});
    const placed=[];
    for(const p of livingPlaces) {
      const cells=new Map();
      for(let i=-5;i<=5;i++)for(let j=-5;j<=8;j++) {
        const x=p.x+i*CELL,z=p.z+j*CELL,c={x,z,y:floor(x,z),heading:0,lobby:null,cell:true};
        if(lobbyGround(x,z)!==null||lobbyAt(x,z)||!carriagewayFree(c,c)||!traffic.clear(body,c,c,false))continue;
        cells.set(i+','+j,{c,i,j,links:[],exit:false});
      }
      const join=(u,v)=>{const d=Math.hypot(u.c.x-v.c.x,u.c.z-v.c.z);
        if(carriagewayFree(u.c,v.c)&&stepFree(u.c,v.c,floor)&&traffic.clear(body,u.c,v.c,false)){u.links.push([v,d]);v.links.push([u,d]);}};
      for(const u of cells.values())for(const [di,dj]of[[1,0],[0,1],[1,1],[1,-1]]){const v=cells.get((u.i+di)+','+(u.j+dj));if(v)join(u,v);}
      const exits=[];
      for(const u of cells.values())for(const n of street) {
        const d=Math.hypot(u.c.x-nodes[n].x,u.c.z-nodes[n].z);
        if(d<REACH&&carriagewayFree(u.c,nodes[n])&&stepFree(u.c,nodes[n],floor)&&traffic.clear(body,u.c,nodes[n],false)){u.exit=true;exits.push([u,n,d]);}
      }
      const kept=new Set(),queue=[...new Set(exits.map(e=>e[0]))];queue.forEach(u=>kept.add(u));
      while(queue.length){const u=queue.pop();for(const [v]of u.links)if(!kept.has(v)){kept.add(v);queue.push(v);}}
      for(const u of kept){u.place=p.id;placed.push(u);u.index=nodes.length;nodes.push(u.c);edges.push([]);}
      for(const u of kept)for(const [v,d]of u.links)if(u.index<v.index)edges[u.index].push([v.index,d]),edges[v.index].push([u.index,d]);
      for(const [u,n,d]of exits){edges[u.index].push([n,d]);edges[n].push([u.index,d]);}
    }
    // Neighbouring places share a path where their grids meet.
    for(const u of placed)for(const v of placed) {
      const d=Math.hypot(u.c.x-v.c.x,u.c.z-v.c.z);
      if(u.place<v.place&&d<3&&carriagewayFree(u.c,v.c)&&stepFree(u.c,v.c,floor)&&traffic.clear(body,u.c,v.c,false)){edges[u.index].push([v.index,d]);edges[v.index].push([u.index,d]);}
    }
  }
  // Inside nodes that the body cannot reach from any entry of their hall (a pocket between fixtures) lose their links.
  function pruneHalls(nodes,edges) {
    const seen=new Set(),queue=[];
    nodes.forEach((n,i)=>{if(n.kind==='entry'){seen.add(i);queue.push(i);}});
    while(queue.length){const u=queue.pop();for(const [v]of edges[u])if(!seen.has(v)){seen.add(v);queue.push(v);}}
    nodes.forEach((n,i)=>{if(n.lobby&&!seen.has(i)){for(const [v]of edges[i])edges[v]=edges[v].filter(e=>e[0]!==i);edges[i]=[];}});
  }
  function route(body,destination) {
    if(revision!==traffic.revision()){cache.clear();revision=traffic.revision();}
    const key=body.circles.map(c=>[c.x,c.z,c.r].join(',')).join(';')+':'+body.maxY;
    if(!cache.has(key)) {
      const index=new Map(),nodes=[];
      points.forEach((p,i)=>{if(traffic.clear(body,p,p,false)){index.set(i,nodes.length);nodes.push(p);}});
      const edges=nodes.map(()=>[]);
      for(const l of staticLinks()) {
        const a=index.get(l.a),b=index.get(l.b);if(a===undefined||b===undefined)continue;
        if(traffic.clear(body,nodes[a],nodes[b],false)){edges[a].push([b,l.length]);edges[b].push([a,l.length]);}
      }
      approach(body,nodes,edges,index);
      pruneHalls(nodes,edges);
      cache.set(key,{nodes,edges});
    }
    const cached=cache.get(key),here=lobbyAt(body.x,body.z)?.id||null,there=lobbyAt(destination.x,destination.z)?.id||null;
    const nodes=[...cached.nodes,{x:body.x,y:body.y,z:body.z,heading:body.heading,lobby:here},{...destination,heading:0,lobby:there}];
    const start=nodes.length-2,end=nodes.length-1,startOnZebra=onZebra(nodes[start]),extra=new Map();
    const link=(u,v,d)=>{for(const [p,q]of[[u,v],[v,u]]){const row=extra.get(p);if(row)row.push([q,d]);else extra.set(p,[[q,d]]);}};
    for(const k of [start,end])for(let i=0;i<k;i++) {
      const a=nodes[k],b=nodes[i],d=Math.hypot(a.x-b.x,a.z-b.z);
      if(a.lobby!==b.lobby||(b.cell&&d>=(b.lobby?4:10)))continue;
      // Only a start standing on a zebra may step off it; every other link stays off carriageways and kerbs.
      // A start standing on a carriageway may leave it; the leg is checked from the start outwards.
      const fromStart=k===start,toStart=i===start;
      const free=toStart?carriagewayFree(b,a,false,true):carriagewayFree(a,b,fromStart&&startOnZebra&&d<stepOff,fromStart);
      if((d<maxLink||i===start)&&free&&stepFree(a,b,floor)&&traffic.clear(body,a,b,false))link(k,i,d);
    }
    // Dijkstra over a binary heap: the approach cells make the graph a few hundred nodes larger.
    const costs=nodes.map(()=>Infinity),previous=nodes.map(()=>-1),heap=[[0,start]];costs[start]=0;
    const push=e=>{let n=heap.push(e)-1;while(n>0){const p=(n-1)>>1;if(heap[p][0]<=e[0])break;heap[n]=heap[p];n=p;}heap[n]=e;};
    const pop=()=>{const top=heap[0],last=heap.pop();if(heap.length){let n=0;for(;;){let c=2*n+1;if(c>=heap.length)break;if(c+1<heap.length&&heap[c+1][0]<heap[c][0])c++;if(heap[c][0]>=last[0])break;heap[n]=heap[c];n=c;}heap[n]=last;}return top;};
    while(heap.length) {
      const [cost,current]=pop();if(cost>costs[current])continue;if(current===end)break;
      const relax=([next,weight])=>{if(cost+weight<costs[next]){costs[next]=cost+weight;previous[next]=current;push([costs[next],next]);}};
      if(current<cached.edges.length)cached.edges[current].forEach(relax);
      extra.get(current)?.forEach(relax);
    }
    if(!Number.isFinite(costs[end]))return [];
    const result=[];for(let n=end;n!==start;n=previous[n]){if(n<0)return [];result.unshift(nodes[n]);}return result;
  }
  return {route,points,streetPoints:points.filter(p=>!p.lobby&&!['foot','landing','porch'].includes(p.kind))};
}
