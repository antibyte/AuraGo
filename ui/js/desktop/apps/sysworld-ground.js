// Pure street ground contract, shared by the renderer, the residents and the tests. Metres, Y up.
// One continuous height profile describes every street: carriageway, splayed kerb, dropped kerbs and
// flares at the zebra crossings, rounded corner kerbs and the end caps at the island and quay edges.
// The kit v3 street modules are authored to it (scripts/test-system-world-street-models.mjs).
import {streets,surfaces} from './sysworld-layout.js';

export const kerb={line:3.30,run:.14,top:.14,rise:.14,droppedRun:1.2,opening:1.2,flare:.8,zebraOffset:8,cornerRadius:1.2,endDepth:.5,pavementLine:4.75};
export const moduleLength=4,fillLength=1,maxLink=48;
// The island body under the plot slabs; its top only shows through hairline seams between modules.
export const island={top:-.2,bottom:-3};
// Pedestrian links keep MARGIN beyond every kerb top. d - run changes by at most 2.33 m per metre
// (flare 1.33 + offset 1), so with a sample every SAMPLE metres no link can touch a kerb or a
// carriageway between two samples (2.33 * SAMPLE / 2 < MARGIN).
const MARGIN=.15,SAMPLE=.1;
const clamp01=v=>v<0?0:v>1?1:v;

// `axis` is the world axis a street runs along: the zs lines run along X, the xs lines along Z.
// Streets along Z end at the quay edge; the quay strip stays continuous across them.
const LINES=[
  ...streets.zs.map(line=>({axis:'x',line,from:streets.minX,to:streets.maxX,crossings:streets.xs})),
  ...streets.xs.map(line=>({axis:'z',line,from:streets.minZ,to:streets.quayZ,crossings:streets.zs})),
];
export function streetLines(){return LINES.map(l=>({...l,crossings:[...l.crossings]}));}

// A zebra on every arm of every intersection, centred kerb.zebraOffset from the intersection centre.
const ZEBRAS=[];
for(const l of LINES)for(const c of l.crossings)for(const sign of[-1,1]){
  const at=c+sign*kerb.zebraOffset;
  if(at>=l.from&&at<=l.to)ZEBRAS.push({axis:l.axis,line:l.line,at,x:l.axis==='x'?at:l.line,z:l.axis==='x'?l.line:at});
}
export function zebras(){return ZEBRAS.map(z=>({...z}));}

// Kerb run at distance s along the street from the nearest zebra centre: dropped inside the
// opening, a linear flare outside it, the standard splay beyond.
export function rampRun(s){
  if(s<=kerb.opening)return kerb.droppedRun;
  if(s>=kerb.opening+kerb.flare)return kerb.run;
  return kerb.droppedRun+(kerb.run-kerb.droppedRun)*(s-kerb.opening)/kerb.flare;
}

// null off-street, else {height, kind: 'road'|'kerb'|'pavement', d, run}; d is the distance beyond
// the kerb line (negative on the carriageway), run the local kerb run.
export function streetProfile(x,z){
  const {xs,zs,halfWidth,minX,maxX,minZ,quayZ}=streets;
  let nx=Infinity,nz=Infinity;
  if(z>=minZ&&z<=quayZ)for(const v of xs)nx=Math.min(nx,Math.abs(x-v));
  if(x>=minX&&x<=maxX)for(const v of zs)nz=Math.min(nz,Math.abs(z-v));
  const onX=nx<=halfWidth,onZ=nz<=halfWidth;   // near an xs line (street along Z) / a zs line (street along X)
  if(!onX&&!onZ)return null;
  const road=surfaces.road;
  let d,run=kerb.run;
  if(onX&&onZ){
    if(nx<=kerb.line||nz<=kerb.line)d=-1;
    else{const qx=nx-kerb.line,qz=nz-kerb.line,r=kerb.cornerRadius;d=qx<r&&qz<r?r-Math.hypot(r-qx,r-qz):Math.min(qx,qz);}
  }else{
    const along=onZ?x:z,crossings=onZ?xs:zs;let s=Infinity;
    for(const c of crossings)s=Math.min(s,Math.abs(along-c-kerb.zebraOffset),Math.abs(along-c+kerb.zebraOffset));
    d=(onZ?nz:nx)-kerb.line;run=rampRun(s);
  }
  let height=road+kerb.rise*clamp01(d/run),kind=d<=0?'road':d>=run?'pavement':'kerb';
  // End caps: a kerb block across the carriageway, top at pavement level, inner face splayed.
  const e=Math.min(onX?z-minZ:Infinity,onX?quayZ-z:Infinity,onZ?x-minX:Infinity,onZ?maxX-x:Infinity),endRun=kerb.endDepth+kerb.run;
  if(e<endRun){const cap=road+kerb.rise*clamp01((endRun-e)/kerb.run);if(cap>height){height=cap;kind='kerb';}}
  return {height,kind,d,run};
}

// Segments between stops (an intersection square edge or a street end). All lengths are whole metres.
export function streetSegments(){
  const out=[];
  for(const l of LINES){
    const stops=[{at:l.from,kind:'end'},...l.crossings.map(at=>({at,kind:'crossing'})),{at:l.to,kind:'end'}];
    for(let i=0;i+1<stops.length;i++){
      const a=stops[i],b=stops[i+1];
      out.push({axis:l.axis,line:l.line,start:a.kind,end:b.kind,
        from:a.kind==='crossing'?a.at+streets.halfWidth:a.at,to:b.kind==='crossing'?b.at-streets.halfWidth:b.at});
    }
  }
  return out;
}

const ALONG={x:0,z:Math.PI/2};
function piece(id,axis,line,from,to){const at=(from+to)/2;return {id,axis,line,from,to,x:axis==='x'?at:line,z:axis==='x'?line:at,angle:ALONG[axis]};}
// Deterministic kit v3 placement list (y = 0; the GLBs carry the profile heights). Modules run along
// local +X and are rotated like the former v1 tiles; a street-end's local +X points out of the island.
export function streetModules(){
  const out=[];
  for(const x of streets.xs)for(const z of streets.zs)out.push({id:'street-crossing',axis:null,line:null,from:null,to:null,x,z,angle:0});
  for(const s of streetSegments()){
    let lo=s.from,hi=s.to;
    if(s.start==='crossing'){out.push(piece('street-zebra',s.axis,s.line,lo,lo+moduleLength));lo+=moduleLength;}
    if(s.end==='crossing'){out.push(piece('street-zebra',s.axis,s.line,hi-moduleLength,hi));hi-=moduleLength;}
    // 4 m modules from the intersection side, every third one a drain; whole-metre fills at the far end.
    const up=s.start==='crossing',count=Math.floor((hi-lo)/moduleLength+1e-9),fills=Math.round(hi-lo-count*moduleLength);
    for(let k=0;k<count;k++){
      const id=k%3===2?'street-drain':'street-plain';
      out.push(up?piece(id,s.axis,s.line,lo+k*moduleLength,lo+(k+1)*moduleLength):piece(id,s.axis,s.line,hi-(k+1)*moduleLength,hi-k*moduleLength));
    }
    for(let k=0;k<fills;k++){const at=up?hi-fills+k:lo+k;out.push(piece('street-fill',s.axis,s.line,at,at+fillLength));}
    // End caps overlay the outermost kerb.endDepth + kerb.run of the last module.
    for(const [kind,at,dir]of[[s.start,s.from,-1],[s.end,s.to,1]]){
      if(kind!=='end')continue;
      out.push({id:'street-end',axis:s.axis,line:s.line,from:at,to:at,x:s.axis==='x'?at:s.line,z:s.axis==='x'?s.line:at,
        angle:s.axis==='x'?(dir>0?0:Math.PI):(dir>0?-Math.PI/2:Math.PI/2)});
    }
  }
  return out;
}

// 25 plot slabs between the street bands plus the quay base, all with their top exactly at ground level.
export function plotSlabs(){
  const {xs,zs,halfWidth:w,minX,maxX,minZ,maxZ,quayZ}=streets;
  const cuts=(lines,lo,hi)=>{const out=[];let a=lo;for(const l of lines){out.push([a,l-w]);a=l+w;}out.push([a,hi]);return out;};
  const slabs=[];
  for(const [x0,x1]of cuts(xs,minX,maxX))for(const [z0,z1]of cuts(zs,minZ,quayZ))slabs.push({kind:'plot',x0,z0,x1,z1,top:surfaces.ground,bottom:island.top});
  slabs.push({kind:'quay',x0:minX,z0:quayZ,x1:maxX,z1:maxZ,top:surfaces.ground,bottom:island.top});
  return slabs;
}

const BAND=kerb.line+kerb.droppedRun+MARGIN;
function nearCarriageway(a,b){
  const x0=Math.min(a.x,b.x),x1=Math.max(a.x,b.x),z0=Math.min(a.z,b.z),z1=Math.max(a.z,b.z);
  return LINES.some(l=>l.axis==='x'
    ?z0<=l.line+BAND&&z1>=l.line-BAND&&x1>=l.from&&x0<=l.to
    :x0<=l.line+BAND&&x1>=l.line-BAND&&z1>=l.from&&z0<=l.to);
}
function onZebra(x,z){
  return ZEBRAS.some(zb=>zb.axis==='x'
    ?Math.abs(x-zb.at)<=kerb.opening&&Math.abs(z-zb.line)<=streets.halfWidth
    :Math.abs(z-zb.at)<=kerb.opening&&Math.abs(x-zb.line)<=streets.halfWidth);
}
// True when the straight line a-b never touches a carriageway or kerb face. `allowZebra` lets a start
// that stands on a zebra step off it (only that crossing corridor is exempt).
export function carriagewayFree(a,b,allowZebra=false){
  if(!nearCarriageway(a,b))return true;
  const n=Math.max(1,Math.ceil(Math.hypot(b.x-a.x,b.z-a.z)/SAMPLE));
  for(let i=0;i<=n;i++){
    const t=i/n,x=a.x+(b.x-a.x)*t,z=a.z+(b.z-a.z)*t,p=streetProfile(x,z);
    if(!p||(p.kind==='pavement'&&p.d-p.run>=MARGIN))continue;
    if(allowZebra&&onZebra(x,z))continue;
    return false;
  }
  return true;
}

// Pavement-centre nodes: one per intersection corner (listed first, (+,+) first, so node 0 starts a
// straight pavement run), two per zebra. Only zebra links cross a carriageway. Built once; frozen.
let graph=null;
export function pedestrianGraph(){
  if(graph)return graph;
  const p=kerb.pavementLine,nodes=[],links=[];
  for(const x of streets.xs)for(const z of streets.zs)for(const [sx,sz]of[[1,1],[1,-1],[-1,1],[-1,-1]])nodes.push({x:x+sx*p,z:z+sz*p,kind:'corner'});
  for(const zb of ZEBRAS){
    const a=nodes.length;
    for(const side of[-1,1])nodes.push(zb.axis==='x'?{x:zb.at,z:zb.line+side*p,kind:'zebra'}:{x:zb.line+side*p,z:zb.at,kind:'zebra'});
    nodes[a].pair=a+1;nodes[a+1].pair=a;
    links.push({a,b:a+1,zebra:true,length:2*p});
  }
  for(let i=0;i<nodes.length;i++)for(let j=i+1;j<nodes.length;j++){
    if(nodes[i].pair===j)continue;
    const length=Math.hypot(nodes[i].x-nodes[j].x,nodes[i].z-nodes[j].z);
    if(length<maxLink&&carriagewayFree(nodes[i],nodes[j]))links.push({a:i,b:j,zebra:false,length});
  }
  nodes.forEach(Object.freeze);links.forEach(Object.freeze);
  graph=Object.freeze({nodes:Object.freeze(nodes),links:Object.freeze(links)});
  return graph;
}
