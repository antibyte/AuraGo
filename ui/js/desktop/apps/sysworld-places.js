// Fixed places belong to the city, never to a changing telemetry inventory.
export const livingPlaces = [
  {id:'repair-bay',district:'infra',x:-28,z:-62,role:'technician'},
  {id:'parcel-sorter',district:'missions',x:35,z:49,role:'courier'},
  {id:'relay-mast',district:'integrations',x:57,z:-20,role:'archivist'},
  {id:'kinetic-fountain',district:'graph',x:-28,z:47,role:'archivist'},
  {id:'glass-garden',district:'graph',x:40,z:-68,role:'technician'},
  {id:'meeting-charge',district:'operations',x:8,z:46,role:'courier'},
];

export function createWayfinder(traffic, floor) {
  const points=[],cache=new Map();let revision=-1;
  for(const x of [-73,-61,-24,-12,12,24,61,73])for(const z of [-83,-71,-38,-26,7,19,53,65])points.push({x,z,y:floor(x,z),heading:0});
  function route(body,destination) {
    if(revision!==traffic.revision()){cache.clear();revision=traffic.revision();}
    const key=body.circles.map(c=>[c.x,c.z,c.r].join(',')).join(';')+':'+body.maxY;
    if(!cache.has(key)) {
      const nodes=points.filter(p=>traffic.clear(body,p,p,false)),edges=nodes.map(()=>[]);
      for(let i=0;i<nodes.length;i++)for(let j=i+1;j<nodes.length;j++) {
        const a=nodes[i],b=nodes[j],d=Math.hypot(a.x-b.x,a.z-b.z);
        if(d<48&&traffic.clear(body,a,b,false)){edges[i].push([j,d]);edges[j].push([i,d]);}
      }
      cache.set(key,{nodes,edges});
    }
    const graph=cache.get(key),nodes=[...graph.nodes,{x:body.x,y:body.y,z:body.z,heading:body.heading},{...destination,heading:0}];
    const start=nodes.length-2,end=nodes.length-1,edges=graph.edges.map(e=>[...e]);edges.push([],[]);
    for(const k of [start,end])for(let i=0;i<k;i++) {
      const a=nodes[k],b=nodes[i],d=Math.hypot(a.x-b.x,a.z-b.z);
      if((d<48||i===start)&&traffic.clear(body,a,b,false)){edges[k].push([i,d]);edges[i].push([k,d]);}
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
