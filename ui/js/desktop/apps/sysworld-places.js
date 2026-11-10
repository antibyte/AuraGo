import {surfaces,streets} from './sysworld-layout.js';
import {pedestrianGraph,carriagewayFree,maxLink,kerb,zebras} from './sysworld-ground.js';
import {lobbyGraph,lobbyAt,stepFree} from './sysworld-lobbies.js';
// Fixed places belong to the city, never to a changing telemetry inventory.
export const livingPlaces = [
  {id:'repair-bay',district:'infra',x:-28,z:-62,role:'technician'},
  {id:'parcel-sorter',district:'missions',x:35,z:49,role:'courier'},
  {id:'relay-mast',district:'integrations',x:57,z:-20,role:'archivist'},
  {id:'kinetic-fountain',district:'graph',x:-28,z:47,role:'archivist'},
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

// Residents walk the pavement graph of sysworld-ground.js plus the lobby graph of sysworld-lobbies.js (stair
// feet and landings, porches, entries, inside grid and workpoints). Every static link keeps the 0.35 m step
// rule (built once, on first use); bodies and solids filter links per revision. Inside a lobby a route links
// only to that lobby's inside nodes; outside never to an inside node, so the doors are the only way in.
export function createWayfinder(traffic, floor, catalog=null) {
  const graph=pedestrianGraph(),lobby=lobbyGraph(catalog),base=graph.nodes.length;
  const points=[...graph.nodes,...lobby.nodes].map(n=>({x:n.x,z:n.z,y:floor(n.x,n.z),heading:0,lobby:n.lobby||null})),cache=new Map();
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
      cache.set(key,{nodes,edges});
    }
    const cached=cache.get(key),here=lobbyAt(body.x,body.z)?.id||null,there=lobbyAt(destination.x,destination.z)?.id||null;
    const nodes=[...cached.nodes,{x:body.x,y:body.y,z:body.z,heading:body.heading,lobby:here},{...destination,heading:0,lobby:there}];
    const start=nodes.length-2,end=nodes.length-1,startOnZebra=onZebra(nodes[start]),edges=cached.edges.map(e=>[...e]);edges.push([],[]);
    for(const k of [start,end])for(let i=0;i<k;i++) {
      const a=nodes[k],b=nodes[i],d=Math.hypot(a.x-b.x,a.z-b.z);
      if(a.lobby!==b.lobby)continue;
      // Only a start standing on a zebra may step off it; every other link stays off carriageways and kerbs.
      if((d<maxLink||i===start)&&carriagewayFree(a,b,k===start&&startOnZebra&&d<stepOff)&&stepFree(a,b,floor)&&traffic.clear(body,a,b,false)){edges[k].push([i,d]);edges[i].push([k,d]);}
    }
    const costs=nodes.map(()=>Infinity),previous=nodes.map(()=>-1),open=new Set(nodes.map((_,i)=>i));costs[start]=0;
    while(open.size) {
      let current=-1;for(const i of open)if(current<0||costs[i]<costs[current])current=i;
      if(!Number.isFinite(costs[current])||current===end)break;open.delete(current);
      for(const [next,weight]of edges[current])if(costs[current]+weight<costs[next]){costs[next]=costs[current]+weight;previous[next]=current;}
    }
    if(!Number.isFinite(costs[end]))return [];
    const result=[];for(let n=end;n!==start;n=previous[n]){if(n<0)return [];result.unshift(nodes[n]);}return result;
  }
  return {route,points};
}
