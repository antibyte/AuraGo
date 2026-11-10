// Pure placement rules for the furnished city (System World beauty pass, phase 4). Every new static placement is
// {asset,x,z,y,angle,area}; world metres, Y up; `angle` turns about +Y as in sysworld-scene.js, so the design's
// front (glTF +Z) faces (sin a, cos a). SIZES mirror each design's navigation.colliders footprint (glTF, model-local
// [x0,z0,x1,z1]); RESERVED mirrors the static ground placements of sysworld-experience.js init() and of
// sysworld-scene.js `placements`. scripts/test-system-world-furnishing.mjs checks both and every rule.
import {streets,surfaces,stations,hoverPads,towers,dronePad,interiors,tramWaypoints} from './sysworld-layout.js';
import {kerb,zebras,pedestrianGraph,plotSlabs,streetProfile,streetSegments} from './sysworld-ground.js';
import {PODIUMS,LOBBIES,DOORS,FLIGHT,lobbyGraph} from './sysworld-lobbies.js';
import {livingPlaces,landmarkPlaces} from './sysworld-places.js';

export const GAP=Object.freeze({node:1.2,line:1.2,pavementLine:.6,dock:3,rail:2.5,platform:2.5,stair:2,door:2,podium:.5,solid:1,living:3,crown:4.5,zebra:2.1,inset:.4,behindKerb:.5});
export const TREE_STEP=8,TREE_INSET=1.4,NEAR_HEIGHT=40,RAIL_HALF=.9;
export const SIZES=Object.freeze({'tree-round':[-.3,-.3,.3,.3],'tree-column':[-.3,-.3,.3,.3],'street-bench':[-1,-.32,1,.3],bollard:[-.12,-.12,.12,.12],
  'litter-bin':[-.3,-.3,.3,.3],'zebra-sign':[-.1,-.1,.1,.1],kiosk:[-1.6,-1.1,1.6,1.6],'flower-bed':[-1.6,-.7,1.6,.7],'cafe-set':[-1,-1,1,1],
  'charge-post':[-.26,-.84,.26,-.4],'info-pylon':[-.48,-.22,.48,.22],'utility-cabinet':[-.65,-.25,.65,.25],'mooring-post':[-.3,-.3,.3,.3],'banner-mast':[-.25,-.25,.25,.25]});
export const PAVEMENT=new Set(['bollard','litter-bin','zebra-sign']);
// Small pieces that sysworld-scene.js hides above NEAR_HEIGHT: sub-pixel from the orbit views, one draw call each.
export const NEAR=new Set([...PAVEMENT,'street-bench','utility-cabinet','mooring-post','cafe-set','charge-post','info-pylon','flower-bed']);
export const LIFE_DESIGNS=Object.freeze(['harbour-launch','harbour-barge','sail-skiff','harbour-buoy','mooring-post','flower-bed','cafe-set','charge-post','info-pylon','utility-cabinet','neon-blade','neon-wave','neon-ring','banner-mast','gull']);
const PI=Math.PI,face={north:PI,south:0,east:PI/2,west:-PI/2},{xs,zs,halfWidth}=streets,P=kerb.pavementLine;
export const gap=(a,b)=>Math.hypot(Math.max(0,b[0]-a[2],a[0]-b[2]),Math.max(0,b[1]-a[3],a[1]-b[3]));
// World AABB of a local [x0,z0,x1,z1] box turned by `angle` and placed at (x,z).
export function turnedBox(local,x,z,angle=0){
  const c=Math.cos(angle),s=Math.sin(angle);let x0=Infinity,z0=Infinity,x1=-Infinity,z1=-Infinity;
  for(const lx of[local[0],local[2]])for(const lz of[local[1],local[3]]){const wx=x+lx*c+lz*s,wz=z-lx*s+lz*c;x0=Math.min(x0,wx);x1=Math.max(x1,wx);z0=Math.min(z0,wz);z1=Math.max(z1,wz);}
  return [x0,z0,x1,z1];
}
export const footprint=item=>turnedBox(SIZES[item.asset]||[0,0,0,0],item.x,item.z,item.angle);
export const LAMPS=zs.flatMap(z=>[-55,-31,31,55].map(x=>({x,z:z-4.5})));
const planterX=x=>x+4*Math.sign(Math.sign(x)*43-x);
const res=(id,asset,x,z,angle,local,kind='solid')=>({id,asset,x,z,angle,kind,local,box:turnedBox(local,x,z,angle)});
export const RESERVED=Object.freeze([
  ...[[-79,68],[-79,-67],[55,4],[8,40],[45,-23]].flatMap(([x,z],i)=>[res('garden-'+i,'garden',x,z,0,[-2,-2,2,2]),res('garden-bench-'+i,'bench',x+3,z,PI/2,[-1.5,-.42,1.5,.4])]),
  ...[[-26,23],[30,23],[-28,72]].map(([x,z],i)=>res('arcade-'+i,'arcade',x,z,0,[-3.25,-1.2,3.25,1.2])),
  res('bridge','bridge',-78.5,40,0,[-3.61,-6,3.61,6]),res('ramp-north','ramp',-80,49,PI,[-1.54,-3.01,1.54,3.01]),
  res('stairs-north','stairs',-76.8,49,PI,[-1.54,-3.01,1.54,3.01]),res('ramp-south','ramp',-78.5,31,0,[-1.54,-3.01,1.54,3.01]),
  res('drone-pad','pad',dronePad.x,dronePad.z,0,[-4,-4,4,4]),res('service-cart','service-cart',55,51.2,PI/2,[-1.5,-1.65,1.5,1.65]),
  ...[[-60,-56],[-60,-46],[51,51]].map(([x,z],i)=>res('charger-'+i,'charger',x,z,0,[-.4,-.3,.65,.3])),
  ...[[-59,-61],[-59,-52]].map(([x,z],i)=>res('cooler-'+i,'cooler',x,z,0,[-1.5,-1.27,1.5,1.27])),
  ...[-68,-52,-28,28,52,68].map((x,i)=>res('quay-lantern-'+i,'quay-lantern',x,75.5,0,[-.34,-.34,.34,.34])),
  ...stations.map(s=>res('station-'+s.id,'station',s.platformX,s.platformZ,s.angle,[-4.5,-2,4.5,2],'platform')),
  ...interiors.map(r=>res('pavilion-'+r.id,null,r.x,r.z,0,[-6.2,-6.2,6.2,6.2])),
  ...livingPlaces.map(p=>res('living-'+p.id,p.id,p.x,p.z,0,[-4,-4,4,4],'living')),
  ...landmarkPlaces.filter(l=>l.y>=0).map(l=>res('landmark-'+l.id,l.id,l.x,l.z,l.angle||0,l.id==='neon-gate'?[-7,-3,7,3]:[-6,-6,6,6])),
  ...LAMPS.flatMap((p,i)=>[res('lamp-'+i,'street-lamp',p.x,p.z,0,[-.3,-.3,.3,.3]),res('planter-'+i,'planter',planterX(p.x),p.z,0,[-1.78,-.85,1.83,1])]),
  ...[-55,-49].map((x,i)=>res('rack-'+i,'server-rack',x,-36.7,0,[-.7,-.6,.7,.66])),
  ...towers.map((t,i)=>res('tower-'+i,'data-tower-a',t.x,t.z,0,[-7.5,-6.5,7.5,8.11])),
]);
const ZEBRAS=zebras();
const LINES=[...zs.flatMap(z=>[-P,P].map(o=>[streets.minX,z+o,streets.maxX,z+o])),...xs.flatMap(x=>[-P,P].map(o=>[x+o,streets.minZ,x+o,streets.quayZ]))];
const ZEBRA_LINKS=ZEBRAS.map(zb=>zb.axis==='x'?[zb.at,zb.line-P,zb.at,zb.line+P]:[zb.line-P,zb.at,zb.line+P,zb.at]);
const RAILS=tramWaypoints.map((p,i)=>{const q=tramWaypoints[(i+1)%tramWaypoints.length];return [Math.min(p[0],q[0]),Math.min(p[1],q[1]),Math.max(p[0],q[0]),Math.max(p[1],q[1])];});
const NODES=[...pedestrianGraph().nodes,...lobbyGraph().nodes.filter(n=>n.kind==='foot'||n.kind==='porch')].map(n=>[n.x,n.z,n.x,n.z]);
const STAIRS=PODIUMS.flatMap(p=>p.stairs.map(f=>[f.x-f.width/2,p.z1,f.x+f.width/2,p.z1+FLIGHT]));
const DOOR_BOXES=DOORS.map(d=>[d.x-d.width/2,d.z-.6,d.x+d.width/2,d.z+.6]);
const PODIUM_BOXES=PODIUMS.map(p=>[p.x0,p.z0,p.x1,p.z1]);
const SLABS=plotSlabs().map(s=>[s.x0+GAP.inset,s.z0+GAP.inset,s.x1-GAP.inset,s.z1-GAP.inset]);
const DOCKS=hoverPads.map(p=>[p.x,p.z,p.x,p.z]);
const inside=(f,s)=>f[0]>=s[0]-1e-9&&f[1]>=s[1]-1e-9&&f[2]<=s[2]+1e-9&&f[3]<=s[3]+1e-9;
function onPavement(item,f){
  for(const [x,z]of[[f[0],f[1]],[f[2],f[1]],[f[0],f[3]],[f[2],f[3]]]){const p=streetProfile(x,z);if(!p||p.kind!=='pavement')return false;}
  if(xs.some(x=>Math.abs(item.x-x)<halfWidth+1)&&zs.some(z=>Math.abs(item.z-z)<halfWidth+1))return false;
  for(const zb of ZEBRAS){const across=zb.axis==='x'?Math.abs(item.z-zb.line):Math.abs(item.x-zb.line),along=zb.axis==='x'?Math.max(f[0]-zb.at,zb.at-f[2],0):Math.max(f[1]-zb.at,zb.at-f[3],0);
    if(across<=halfWidth&&along<GAP.zebra)return false;}
  const p=streetProfile(item.x,item.z);return p.d-p.run<=GAP.behindKerb+1e-9;
}
// Rule names an item breaks (empty: it fits). `placed` holds the footprints of the pieces accepted before it.
export function violations(item,placed=[]){
  const f=footprint(item),pave=PAVEMENT.has(item.asset),out=[];
  if(pave?!onPavement(item,f):!SLABS.some(s=>inside(f,s)))out.push('ground');
  if(NODES.some(n=>gap(n,f)<GAP.node))out.push('node');
  if(LINES.some(l=>gap(l,f)<(pave?GAP.pavementLine:GAP.line))||ZEBRA_LINKS.some(l=>gap(l,f)<GAP.line))out.push('line');
  if(DOCKS.some(d=>gap(d,f)<GAP.dock))out.push('dock');
  if(RAILS.some(r=>gap(r,f)<GAP.rail+RAIL_HALF))out.push('rail');
  if(STAIRS.some(s=>gap(s,f)<GAP.stair)||DOOR_BOXES.some(d=>gap(d,f)<GAP.door))out.push('stair');
  if(PODIUM_BOXES.some(p=>gap(p,f)<GAP.podium))out.push('podium');
  for(const r of RESERVED)if(gap(r.box,f)<(r.kind==='platform'?GAP.platform:r.kind==='living'?GAP.living:GAP.solid))out.push(r.id);
  if(placed.some(g=>gap(g,f)<GAP.solid))out.push('solid');
  if(/^tree-/.test(item.asset)&&LAMPS.some(p=>Math.hypot(p.x-item.x,p.z-item.z)<GAP.crown))out.push('crown');
  return out;
}
const item=(asset,x,z,angle,area,y=0)=>({asset,x,z,y,angle,area});
// Empty plots with a purpose (decision 5), keyed by their (x0, z0); street trees and cabinets skip them.
export const PURPOSE_PLOTS=Object.freeze([[-61,-94],[24,-94],[-85,-94],[73,-94],[73,-71],[73,65]]);
const purpose=s=>PURPOSE_PLOTS.some(([x,z])=>x===s.x0&&z===s.z0);
function areaItems(){
  const out=[];
  // Linear park (x -61..-24, z -94..-83): two tree rows clear of the lamp crowns, three beds, benches facing them.
  for(const x of[-59,-50.5,-44,-37.5,-26.5])out.push(item('tree-round',x,-84.6,x*.7,'park'));
  [-58,-51.5,-45,-38.5,-32,-25.5].forEach((x,i)=>out.push(item(i%2?'tree-round':'tree-column',x,-92.4,x*.9,'park')));
  for(const x of[-53,-42.5,-32])out.push(item('flower-bed',x,-88.5,0,'park'));
  for(const x of[-48,-37.5])out.push(item('street-bench',x,-86.5,face.north,'park'),item('street-bench',x,-90.5,face.south,'park'));
  // Market (x 24..61, z -94..-83): three kiosks facing the street, cafe sets behind them, banner masts.
  for(const x of[29.5,42.5,55.5])out.push(item('kiosk',x,-85,face.south,'market'));
  [28,35,49,56].forEach((x,i)=>out.push(item('cafe-set',x,-89.6,i*PI/2,'market')));
  for(const [x,z]of[[36,-84.4],[49,-84.4],[25.5,-92.5],[59.5,-92.5]])out.push(item('banner-mast',x,z,0,'market'));
  // Groves in both north corners (x +-79, z -88.5).
  for(const s of[-1,1]){
    for(const [x,z,asset]of[[76.2,-84.6,'tree-round'],[81.8,-84.6,'tree-round'],[79,-88.5,'tree-column'],[76.2,-92.4,'tree-round'],[81.8,-92.4,'tree-column']])out.push(item(asset,s*x,z,x+z,'grove'));
    out.push(item('street-bench',s*79,-85.2,face.south,'grove'));
  }
  // Charging plaza (x 73..85, z -71..-38): four posts facing the street, a pylon, two beds, a tree row.
  for(const z of[-64,-58,-52,-46])out.push(item('charge-post',74.6,z,face.west,'plaza'));
  out.push(item('info-pylon',77.5,-40.5,0,'plaza'));
  for(const z of[-62,-54])out.push(item('flower-bed',79.5,z,PI/2,'plaza'));
  [-67,-59,-51,-43].forEach((z,i)=>out.push(item(i%2?'tree-column':'tree-round',82.2,z,z,'plaza')));
  // Harbour cafe terrace (x 73..85, z 65..74).
  for(const [x,z,a]of[[75.6,67.4,0],[81,67.4,PI/2],[78.3,71.4,PI]])out.push(item('cafe-set',x,z,a,'cafe'));
  out.push(item('flower-bed',83.9,70.6,PI/2,'cafe'),item('tree-column',84,66,1,'cafe'));
  // Mooring posts along the quay edge, clear of the pavilions, the lanterns and the harbour dock.
  for(const x of[-80,-62,-34,-14,14,34,60,80])out.push(item('mooring-post',x,79.2,0,'quay',surfaces.quay));
  return out;
}
// District plots: beds beside every stair flight, benches facing it, a cabinet per block, a pylon at every station.
const PYLONS=[[-75.5,-60.5,PI/2],[-75.5,-17.5,PI/2],[-34.8,67,0],[7.8,67.4,0],[34.8,67,0],[75.5,-2.5,-PI/2],[7.5,-85.5,PI]];
function districtExtras(){
  const out=[];
  for(const p of PODIUMS)for(const f of p.stairs)for(const s of[-1,1])
    out.push(item('flower-bed',f.x+s*(f.width/2+GAP.stair+1.7),p.z1+1.25,0,'district'),item('street-bench',f.x+s*(f.width/2+3.2),p.z1+FLIGHT+3.2,face.north,'district'));
  for(const s of plotSlabs()){
    if(s.kind!=='plot'||s.x1-s.x0<20||purpose(s))continue;
    const north=zs.some(z=>Math.abs(s.z0-(z+halfWidth))<1e-6);
    out.push(item('utility-cabinet',s.x1-4,north?s.z0+.9:s.z1-.9,north?face.north:face.south,'district'));
  }
  for(const [x,z,a]of PYLONS)out.push(item('info-pylon',x,z,a,'station'));
  return out;
}
// Pavement: two bollards beside each dropped kerb and a crossing sign (away from the junction), one bin per block side.
function pavementPieces(){
  const out=[];
  for(const zb of ZEBRAS){
    const crossing=(zb.axis==='x'?xs:zs).reduce((a,c)=>Math.abs(c-zb.at)<Math.abs(a-zb.at)?c:a),away=Math.sign(zb.at-crossing);
    const at=(s,side,o)=>zb.axis==='x'?{x:zb.at+away*s,z:zb.line+side*o}:{x:zb.line+side*o,z:zb.at+away*s};
    for(const side of[-1,1]){const p=at(2.4,side,3.65);out.push(item('bollard',p.x,p.z,0,'pavement'));}
    const sign=at(3.8,1,3.65);out.push(item('zebra-sign',sign.x,sign.z,zb.axis==='x'?PI/2:0,'pavement'));
  }
  for(const seg of streetSegments())if(seg.start==='crossing'&&seg.end==='crossing'){
    const mid=(seg.from+seg.to)/2;
    out.push([1,-1].map(side=>seg.axis==='x'?item('litter-bin',mid,seg.line+side*3.82,0,'pavement'):item('litter-bin',seg.line+side*3.82,mid,0,'pavement')));
  }
  return out;
}
// Trees spaced evenly between the two inset corners of every street-facing plot edge; TREE_STEP is the target spacing
// (the real one comes out between about 7.5 and 8.6 m). Corner trees reached from two edges: the first edge in the
// fixed order z0, z1, x0, x1 keeps it, the duplicate is dropped by the solid rule.
function streetTrees(){
  const out=[];let k=0;
  for(const s of plotSlabs()){
    if(s.kind!=='plot'||purpose(s))continue;
    const edges=[];
    if(zs.some(z=>Math.abs(s.z0-(z+halfWidth))<1e-6))edges.push([s.z0+TREE_INSET,true]);
    if(zs.some(z=>Math.abs(s.z1-(z-halfWidth))<1e-6))edges.push([s.z1-TREE_INSET,true]);
    if(xs.some(x=>Math.abs(s.x0-(x+halfWidth))<1e-6))edges.push([s.x0+TREE_INSET,false]);
    if(xs.some(x=>Math.abs(s.x1-(x-halfWidth))<1e-6))edges.push([s.x1-TREE_INSET,false]);
    for(const [at,alongX]of edges){const lo=alongX?s.x0:s.z0,hi=alongX?s.x1:s.z1;
      const span=hi-lo-2*TREE_INSET,n=Math.max(1,Math.round(span/TREE_STEP));
      for(let i=0;i<=n;i++){const t=lo+TREE_INSET+i*span/n;out.push(item(k%3===2?'tree-column':'tree-round',alongX?t:at,alongX?at:t,(k*2.39996)%(2*PI),'street'));k++;}}
  }
  return out;
}
let cache=null;
// Deterministic and cached: areas first (all must fit), then district extras, pavement pieces and street trees.
// An array candidate lists alternatives; the first that fits is taken.
export function furnishing(){
  if(cache)return cache;
  const accepted=[],boxes=[];
  for(const c of[...areaItems(),...districtExtras(),...pavementPieces(),...streetTrees()])
    for(const it of Array.isArray(c)?c:[c])if(!violations(it,boxes).length){accepted.push(Object.freeze(it));boxes.push(footprint(it));break;}
  return cache=Object.freeze(accepted);
}
// Lawn and paving slabs over the purpose plots (backlog item 7), 0.2 m inside the plot edges.
export const SURFACES=Object.freeze([
  {kind:'lawn',x0:-60.8,z0:-93.8,x1:-24.2,z1:-83.2},{kind:'lawn',x0:-84.8,z0:-93.8,x1:-73.2,z1:-83.2},{kind:'lawn',x0:73.2,z0:-93.8,x1:84.8,z1:-83.2},
  {kind:'paving',x0:24.2,z0:-93.8,x1:60.8,z1:-83.2},{kind:'paving',x0:73.2,z0:-70.8,x1:84.8,z1:-38.2},{kind:'paving',x0:73.2,z0:65.2,x1:84.8,z1:73.8},
]);
export const PLACES=Object.freeze([
  Object.freeze({id:'charge-plaza',district:'integrations',role:'technician',x:79,z:-55}),
  Object.freeze({id:'harbour-cafe',district:'missions',role:'archivist',x:79,z:69.5}),
]);
const CAFE_POINTS=[['serve-0',78.3,67.4],['serve-1',75.4,70.4]];
const entryOf=(catalog,id)=>catalog instanceof Map?catalog.get(id):catalog?.assets?.find(a=>a.id===id);
// A standing spot for a resident body (0.5 m plus the 0.06 m traffic gap) at a workpoint: pushed along the design's
// +Z (its front) until it clears the design's own colliders.
export function standing(entry,w,clear=.56){
  const boxes=entry.navigation?.colliders||[],x=w.position[0];let z=w.position[2];
  for(let k=0;k<40&&boxes.some(b=>x>b[0]-clear&&x<b[3]+clear&&z>b[2]-clear&&z<b[5]+clear);k++)z+=.05;
  return [x,z];
}
// Society places for the charging plaza and the harbour cafe (kit:true: no machinery, no demonstrate prompt).
export function furnishingPlaces(catalog){
  const out=[],[plaza,cafe]=PLACES,post=entryOf(catalog,'charge-post');
  if(post?.navigation?.workpoints?.length){
    const workpoints=[];
    furnishing().filter(f=>f.area==='plaza'&&f.asset==='charge-post').forEach((f,i)=>{const c=Math.cos(f.angle),s=Math.sin(f.angle);
      for(const w of post.navigation.workpoints){const [lx,lz]=standing(post,w);workpoints.push({id:w.id+'-'+i,position:[f.x+lx*c+lz*s-plaza.x,0,f.z-lx*s+lz*c-plaza.z]});}});
    out.push({place:{...plaza,kit:true},entry:{navigation:{workpoints}}});
  }
  if(entryOf(catalog,'cafe-set'))out.push({place:{...cafe,kit:true},entry:{navigation:{workpoints:CAFE_POINTS.map(([id,x,z])=>({id,position:[x-cafe.x,0,z-cafe.z]}))}}});
  return out;
}
