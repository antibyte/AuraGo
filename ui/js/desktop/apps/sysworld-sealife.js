import * as THREE from 'three';
import {mergeGeometries} from 'three/addons/utils/BufferGeometryUtils.js';
import {SPECIES} from '../../chat/ocean-life-species.js';
import {createSchool,createRng} from '../../chat/ocean-life-boids.js';
import * as P from './sysworld-sealife-paths.js';
import {createSeaMaterials} from './sysworld-sealife-shaders.js';
import {MATERIAL_KIND, DOUBLE_SIDED} from '../../chat/ocean-life-shaders.js';
const {SEA,SCALE,LEAP,DRAW,BLOW,TAU,BOIDS_STEP}=P;
// Sea life (backlog: fish and the other creatures of the Ocean chat theme swim in the System World's water). Fish schools
// (boids in box habitats along the shores), leaping fish with splash rings, reef sharks, hammerheads, turtles, a whale with a
// blow, mantas and moon jellies. Models: the light fish built for the city (ui/3d/system-world/sea-life) and the chat's and the
// abyss kit's GLBs unchanged; the swim deformation and colours are the chat's GLSL (sysworld-sealife-shaders.js). Everything is a
// function of the atmosphere clock (sysworld-sealife-paths.js), so reduced motion freezes the sea life. Low tier: nothing loads.
// load(folder, file) resolves to a glTF scene; the folders are FOLDERS below (under /3d/).
export const FOLDERS=Object.freeze({fish:'system-world/sea-life/v1/',chat:'chat-ocean-life/v1/',abyss:'screensaver/abyss/v1/'});
export const MODELS=Object.freeze([
  {id:'fish_sardine',folder:'fish',file:'fish-sardine.lod1.glb'},{id:'fish_tang',folder:'fish',file:'fish-tang.lod1.glb'},
  {id:'fish_clown',folder:'fish',file:'fish-clown.lod1.glb'},{id:'fish_butterfly',folder:'fish',file:'fish-butterfly.lod1.glb'},
  {id:'shark_reef',folder:'chat',file:'shark-reef.glb'},{id:'shark_hammer',folder:'chat',file:'shark-hammer.glb'},{id:'turtle',folder:'chat',file:'turtle.glb'},{id:'whale',folder:'chat',file:'whale.glb'},
  {id:'manta',folder:'abyss',file:'manta.glb',length:2.1,fixup:g=>g.rotateY(Math.PI)},   // length: the model's depth in metres before SCALE; 2.1 gives a wing span of 4 m at scale 1.2 (the chat's 3.5 would give 6.7 m)   // the abyss kit's nose points to -Z; the chat frame wants +Z
  {id:'jelly_moon',folder:'fish',file:'jelly-moon.lod1.glb'},   // light copy (1378 triangles) of the abyss kit's moon jelly
]);
// Big animals: kind (see LOOPS), model, how many on a tier, scale. The stroke rate makes slow animals' strokes readable.
const BIG=[{kind:'shark',id:'shark_reef',count:'sharks',rate:1.6},{kind:'hammer',id:'shark_hammer',count:'hammers',rate:1.6},{kind:'turtle',id:'turtle',count:'turtles',rate:2},
  {kind:'whale',id:'whale',count:'whale',rate:3},{kind:'manta',id:'manta',count:'mantas',rate:1},{kind:'jelly',id:'jelly_moon',count:'jellies',rate:1}];
const IDENTITY=new THREE.Matrix4(),ZERO=new Float32Array(16);
// Bake a glTF scene like the chat's loadModel: world transforms applied, scaled so the depth equals the species length, TEXCOORD_1 as aAnim.
export function prepareModel(root,spec,fixup){
  root.updateMatrixWorld(true);const parts=[];
  root.traverse(o=>{if(!o.isMesh)return;const g=o.geometry;if(!o.matrixWorld.equals(IDENTITY))g.applyMatrix4(o.matrixWorld);parts.push({name:o.material?.name||'body',geometry:g});o.material?.dispose?.();});
  const drop=()=>parts.forEach(p=>p.geometry.dispose());
  if(!parts.length)throw Error(spec.id+' has no meshes');
  for(const p of parts)for(const a of ['position','normal','uv','uv1'])if(!p.geometry.getAttribute(a)){drop();throw Error(spec.id+' primitive "'+p.name+'" lacks '+a);}
  if(fixup)parts.forEach(p=>fixup(p.geometry));
  const box=new THREE.Box3();parts.forEach(p=>{p.geometry.computeBoundingBox();box.union(p.geometry.boundingBox);});
  const depth=box.max.z-box.min.z;if(!(depth>1e-6)){drop();throw Error(spec.id+' has no depth');}
  const scale=spec.length/depth;let radius=0;
  parts.forEach(p=>{p.geometry.scale(scale,scale,scale);p.geometry.computeBoundingSphere();radius=Math.max(radius,p.geometry.boundingSphere.radius);
    p.geometry.setAttribute('aAnim',p.geometry.getAttribute('uv1'));p.geometry.deleteAttribute('uv1');});
  return {spec,parts,scale,front:box.max.z*scale,radius};
}
// Column-major matrix at a[o..o+15]: +Z along (fx,fy,fz), +Y up (turned by roll about the nose), uniform scale s, position p.
export function place(a,o,px,py,pz,fx,fy,fz,s,roll){
  const l=Math.hypot(fx,fy,fz)||1;fx/=l;fy/=l;fz/=l;
  let xx=fz,xz=-fx;const xl=Math.hypot(xx,xz);if(xl<1e-4){xx=1;xz=0;}else{xx/=xl;xz/=xl;}
  let yx=fy*xz,yy=fz*xx-fx*xz,yz=-fy*xx,x0=xx,y0=0,z0=xz;   // X = up x forward (the left), Y = forward x X
  if(roll){const c=Math.cos(roll),n=Math.sin(roll);x0=xx*c+yx*n;y0=yy*n;z0=xz*c+yz*n;[yx,yy,yz]=[yx*c-xx*n,yy*c,yz*c-xz*n];}
  a[o]=x0*s;a[o+1]=y0*s;a[o+2]=z0*s;a[o+3]=0;a[o+4]=yx*s;a[o+5]=yy*s;a[o+6]=yz*s;a[o+7]=0;a[o+8]=fx*s;a[o+9]=fy*s;a[o+10]=fz*s;a[o+11]=0;a[o+12]=px;a[o+13]=py;a[o+14]=pz;a[o+15]=1;
}
const between=(rng,r)=>r[0]+rng()*(r[1]-r[0]);
// The boids start in loose clusters sized for the chat's 15 cm fish; the city's fish are SCALE.fish times larger, so each cluster opens up by that much.
function spreadClusters(school,h){
  const n=school.count,clusters=Math.max(1,Math.round(n/10)),p=school.pos,k=SCALE.fish;
  for(let c=0;c<clusters;c++){
    let x=0,z=0,y=0,m=0;for(let i=c;i<n;i+=clusters){x+=p[i*3];y+=p[i*3+1];z+=p[i*3+2];m++;}x/=m;y/=m;z/=m;
    for(let i=c;i<n;i+=clusters){const o=i*3;
      p[o]=Math.min(h.x[1],Math.max(h.x[0],x+(p[o]-x)*k));p[o+2]=Math.min(h.z[1],Math.max(h.z[0],z+(p[o+2]-z)*k));
      p[o+1]=Math.min(SEA-h.depth[0],Math.max(SEA-h.depth[1],y+(p[o+1]-y)*(k-1)));}
  }
}
export function createSealife({group,load,water}){
  const holder=new THREE.Group();holder.name='sealife';group.add(holder);
  const clock={value:0},models=new Map(),frustum=new THREE.Frustum(),proj=new THREE.Matrix4(),sphere=new THREE.Sphere(),camPos=new THREE.Vector3();
  let materials=null,pop=null,tier='high',started=false,disposed=false,loaded=false,loading=null,failures=0,leaps=0,now=0,last=null,builtTier=null;
  let prof=null;
  const offsets={shark:[],hammer:[],turtle:[],whale:[],manta:[]},debugState={leapAt:null,hidden:false,skip:{}},pins={};   // pins: survey checks put an animal at a given place
  const speciesOf=id=>SPECIES.find(s=>s.id===id);
  // ---- batches: one InstancedMesh per material part, sharing one matrix buffer and one per-instance buffer ----
  // merged: all parts of the model in ONE mesh and one draw call (fish): each vertex carries its material kind (aMat) and the double-sided
  // parts (fins) are added a second time with reversed winding and normals, so the single material can stay single-sided.
  function mergedParts(model){
    const list=model.parts.flatMap(part=>{
      const g=part.geometry.clone(),kind=MATERIAL_KIND[part.name]??0,count=g.getAttribute('position').count;
      g.setAttribute('aMat',new THREE.BufferAttribute(new Float32Array(count).fill(kind),1));
      if(!DOUBLE_SIDED[part.name])return [g];
      const back=g.clone(),index=back.getIndex().array.slice(),normal=back.getAttribute('normal');
      for(let i=0;i<index.length;i+=3){const t=index[i+1];index[i+1]=index[i+2];index[i+2]=t;}
      back.setIndex(new THREE.BufferAttribute(index,1));back.setAttribute('normal',new THREE.BufferAttribute(normal.array.map(v=>-v),3));
      return [g,back];
    });
    const merged=mergeGeometries(list);list.forEach(g=>g.dispose());merged.computeBoundingSphere();return [{name:'merged',geometry:merged}];
  }
  function makeBatch(model,count,{tint=false,material,merge=false}={}){
    const n=Math.max(1,count),inst=new THREE.InstancedBufferAttribute(new Float32Array(n*4),4);inst.setUsage(THREE.DynamicDrawUsage);
    const tintAttr=tint?new THREE.InstancedBufferAttribute(new Float32Array(n*3),3):null,geometries=[];let first=null;
    const meshes=(merge?mergedParts(model):model.parts).map(part=>{
      const g=merge?part.geometry:part.geometry.clone();g.setAttribute('aInst',inst);if(tintAttr)g.setAttribute('aTint',tintAttr);geometries.push(g);
      const mesh=new THREE.InstancedMesh(g,material(part.name),n);mesh.frustumCulled=false;mesh.castShadow=false;mesh.receiveShadow=false;mesh.visible=false;mesh.count=0;
      if(first)mesh.instanceMatrix=first.instanceMatrix;else{first=mesh;mesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage);}
      holder.add(mesh);return mesh;
    });
    return {meshes,inst,tint:tintAttr,matrix:first.instanceMatrix,geometries,
      // only a batch that is drawn uploads its buffers
      show(k){for(const m of meshes){m.count=k;m.visible=k>0;}if(k>0){this.matrix.needsUpdate=true;inst.needsUpdate=true;if(tintAttr)tintAttr.needsUpdate=true;}},
      visible:()=>meshes[0].visible,
      dispose(){meshes.forEach(m=>{m.removeFromParent();m.dispose();});geometries.forEach(g=>g.dispose());}};
  }
  const fishMaterial=model=>part=>materials.creature(model,part),mergedFish=model=>()=>materials.creatureMerged(model);
  // ---- population ----
  function clearPop(){
    if(!pop)return;
    for(const s of pop.schools)s.batch.dispose();
    for(const b of pop.big)b.batch.dispose();
    for(const l of pop.leapers.values())l.dispose();
    pop.rings.dispose();pop.puffs.dispose();pop=null;builtTier=null;
  }
  function build(){
    if(disposed||tier==='low'||!loaded){if(tier==='low')clearPop();return;}
    if(builtTier===tier)return;
    clearPop();materials||(materials=createSeaMaterials(THREE,water,clock));
    const counts=P.TIER_COUNTS[tier]||P.TIER_COUNTS.high,next={schools:[],big:[],leapers:new Map(),rings:null,puffs:null};
    P.habitatsFor(tier).forEach((h,idx)=>{
      const model=models.get(h.species);if(!model)return;
      const rng=createRng(0x5ea1+idx*7919),k=SCALE.fish,spec=model.spec,w=h.x[1]-h.x[0],d=h.z[1]-h.z[0],ix=Math.min(3,.22*w),iz=Math.min(3,.22*d);
      const bounds={x:[h.x[0]+ix,h.x[1]-ix],y:[SEA-h.depth[1],SEA-h.depth[0]],z:[h.z[0]+iz,h.z[1]-iz]};
      const school=createSchool({count:h.count,rng,bounds,speed:spec.speed.map(v=>v*k),size:[.85,1.15],
        params:{neighbor:1.2*k,maxTurn:2.4,bounce:1.6}});
      spreadClusters(school,h);
      const batch=makeBatch(model,h.count,{material:mergedFish(model),merge:true}),freq=new Float32Array(h.count),amp=spec.swim.amp*spec.length;
      for(let i=0;i<h.count;i++){freq[i]=between(rng,spec.swim.freq);batch.inst.setXYZW(i,school.phase[i],freq[i],amp,1);}
      const cx=(h.x[0]+h.x[1])/2,cz=(h.z[0]+h.z[1])/2,cy=SEA-(h.depth[0]+h.depth[1])/2;
      const instBase=Float32Array.from(batch.inst.array);
      next.schools.push({h,model,school,batch,bounds,instBase,stride:1,acc:(idx%4)*BOIDS_STEP/4,hidden:-1,visible:false,centre:new THREE.Vector3(cx,cy,cz),home:new THREE.Vector3(cx,cy,cz),radius:Math.hypot(w/2,1.2,d/2)+4,homeRadius:Math.hypot(w/2,1.2,d/2)+4,pin:null});
      writeSchool(next.schools[next.schools.length-1]);
    });
    for(const def of BIG){
      const model=models.get(def.id),n=counts[def.count]||0;if(!model||!n)continue;
      const spec=model.spec,rng=createRng(0xb16+def.kind.length*977+def.kind.charCodeAt(0)),scale=SCALE[def.kind],jelly=def.kind==='jelly';
      const mantaMaterial=def.kind==='manta'?materials.manta(model,1.2):null,material=jelly?part=>materials.jelly(part):mantaMaterial?()=>mantaMaterial:fishMaterial(model);
      const batch=makeBatch(model,n,{tint:jelly,material}),stroke=new Float32Array(n*4),tints=new Float32Array(n*3);
      for(let i=0;i<n;i++){
        const amp=def.kind==='whale'?spec.swim.amp:spec.swim.amp*spec.length;
        if(jelly){const J=P.JELLY_HOMES[i];stroke.set([J.pulse,J.freq,.4,1],i*4);tints.set([.8+.4*((i*37)%11)/11,.85+.3*((i*53)%7)/7,1],i*3);}
        else stroke.set([rng(),between(rng,spec.swim.freq)*def.rate,amp,1],i*4);
      }
      next.big.push({def,model,batch,n,stroke,tints,scale:jelly?scale*1:scale,poses:Array.from({length:n},()=>({x:0,y:SEA,z:0,fx:0,fy:0,fz:1,roll:0,size:1})),shown:0,posed:new Uint8Array(n),drawn:jelly?DRAW.jelly:DRAW.big});
    }
    // splash rings and blow puffs
    const ringGeometry=new THREE.RingGeometry(.6,1,40,1);ringGeometry.rotateX(-Math.PI/2);
    next.rings=makeEffect(ringGeometry,materials.ring(),16,'aRing');
    const puffGeometry=new THREE.PlaneGeometry(1,1);
    next.puffs=makeEffect(puffGeometry,materials.puff(),BLOW.count,'aPuff');
    // leapers: one instance per leaping species (sardine and tang)
    for(const id of ['fish_sardine','fish_tang']){const model=models.get(id);if(!model)continue;
      const b=makeBatch(model,1,{material:mergedFish(model),merge:true});b.inst.setXYZW(0,0,16,model.spec.swim.amp*model.spec.length*1.3,1);next.leapers.set(id,b);}
    pop=next;builtTier=tier;leap=null;last=null;
    for(const [kind,list] of Object.entries(offsets))list.length=0;
  }
  function makeEffect(geometry,material,count,name){
    const attr=new THREE.InstancedBufferAttribute(new Float32Array(count*2),2);attr.setUsage(THREE.DynamicDrawUsage);geometry.setAttribute(name,attr);
    const mesh=new THREE.InstancedMesh(geometry,material,count);mesh.frustumCulled=false;mesh.visible=false;mesh.count=0;mesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage);mesh.renderOrder=2;holder.add(mesh);
    return {mesh,attr,list:[],dispose(){mesh.removeFromParent();mesh.dispose();geometry.dispose();}};
  }
  // ---- schools ----
  const env={repulsors:[],speedMul:1,cohesionMul:1};
  // Every stride-th fish goes into slot i/stride; the pose is the last 20 Hz boids state carried on by S.acc seconds of its velocity.
  function writeSchool(S){
    const {school,batch}=S,a=batch.matrix.array,pos=school.pos,vel=school.vel,k=SCALE.fish,st=S.stride,e=S.pin?0:S.acc;
    for(let i=0,j=0;i<school.count;i+=st,j++){
      const o=i*3;
      if(i===S.hidden){a.set(ZERO,j*16);continue;}
      const l=Math.hypot(vel[o],vel[o+1],vel[o+2]);
      place(a,j*16,pos[o]+vel[o]*e,Math.min(SEA-.05,pos[o+1]+vel[o+1]*e),pos[o+2]+vel[o+2]*e,l>1e-6?vel[o]:1,l>1e-6?vel[o+1]:0,l>1e-6?vel[o+2]:0,school.size[i]*k,0);
    }
  }
  function setStride(S,st){
    if(S.stride===st)return;S.stride=st;const ia=S.batch.inst.array;
    for(let i=0,j=0;i<S.school.count;i+=st,j++)ia.set(S.instBase.subarray(i*4,i*4+4),j*4);
  }
  function stepSchool(S,dt){
    const {school,bounds,h}=S;
    env.repulsors.length=0;
    if(pop){for(const b of pop.big)if(b.def.kind==='shark'||b.def.kind==='hammer')for(const p of b.poses){
      if(Math.hypot(p.x-S.centre.x,p.z-S.centre.z)<S.radius+10)env.repulsors.push({x:p.x,y:p.y,z:p.z,radius:7,strength:4});}}
    school.step(dt,env);
    const pos=school.pos;
    for(let i=0;i<school.count;i++){const o=i*3;   // the habitat is a hard limit: never above the surface, never out of the box
      pos[o]=Math.min(h.x[1],Math.max(h.x[0],pos[o]));pos[o+1]=Math.min(SEA-h.depth[0],Math.max(SEA-h.depth[1],pos[o+1]));pos[o+2]=Math.min(h.z[1],Math.max(h.z[0],pos[o+2]));}
  }
  // a school held at a place (survey checks): the fish hang in a loose grid there, nose along +X
  function holdSchool(S){
    const p=S.school.pos,v=S.school.vel,pin=S.pin;
    for(let i=0;i<S.school.count;i++){p[i*3]=pin.x+((i%6)-2.5)*.9;p[i*3+1]=SEA-pin.depth;p[i*3+2]=pin.z+(Math.floor(i/6)%3-1)*.8;v[i*3]=.5;v[i*3+1]=0;v[i*3+2]=0;}
  }
  function hideAll(){
    if(!pop)return;endLeap();
    for(const S of pop.schools){S.visible=false;S.batch.show(0);}
    for(const B of pop.big){B.shown=0;B.batch.show(0);}
    for(const l of pop.leapers.values())l.show(0);
    pop.rings.mesh.count=pop.puffs.mesh.count=0;pop.rings.mesh.visible=pop.puffs.mesh.visible=false;
  }
  // ---- leaps and splashes ----
  let leap=null,cursor=0;
  const leapOut={x:0,z:0,h:0,fx:0,fy:0,fz:1},leapMath={apex:0,speed:0};
  function addRing(x,z,t0,size){
    const list=pop.rings.list;if(list.length>=16)list.shift();list.push({x,z,t0,size});
  }
  function startLeap(k,t0,cam){
    let best=null,bestD=Infinity;
    for(const S of pop.schools){if(!S.visible||!pop.leapers.has(S.h.species))continue;const d=S.centre.distanceTo(cam);if(d<bestD){best=S;bestD=d;}}
    if(!best)return false;
    const school=best.school,i=(Math.imul(k+7,2654435761)>>>0)%school.count,o=i*3,l=Math.hypot(school.vel[o],school.vel[o+2])||1;
    const p=P.leapParams(k<0?9000-k:k,leapMath),dx=school.vel[o]/l||1,dz=school.vel[o+2]/l;
    leap={k,t0,school:best,fish:i,x0:school.pos[o],z0:school.pos[o+2],dx,dz,apex:p.apex,speed:p.speed};
    best.hidden=i;leaps++;
    const dist=p.speed*LEAP.duration;addRing(leap.x0,leap.z0,t0,1);addRing(leap.x0+dx*dist,leap.z0+dz*dist,t0+LEAP.duration,1);
    return true;
  }
  function endLeap(){
    if(!leap)return;
    const S=leap.school,o=leap.fish*3,dist=leap.speed*LEAP.duration,h=S.h;
    S.school.pos[o]=Math.min(h.x[1],Math.max(h.x[0],leap.x0+leap.dx*dist));S.school.pos[o+2]=Math.min(h.z[1],Math.max(h.z[0],leap.z0+leap.dz*dist));S.school.pos[o+1]=SEA-.5;
    S.hidden=-1;pop.leapers.get(S.h.species)?.show(0);leap=null;
  }
  function updateLeaps(cam){
    if(leap){const s=(now-leap.t0)/LEAP.duration;if(s>=1||s<0)endLeap();else{
      P.leapPose(leap.apex,leap.speed,leap.dx,leap.dz,s,leapOut);
      const b=pop.leapers.get(leap.school.h.species);
      place(b.matrix.array,0,leap.x0+leapOut.x,SEA+leapOut.h,leap.z0+leapOut.z,leapOut.fx,leapOut.fy,leapOut.fz,leap.school.school.size[leap.fish]*SCALE.fish,0);b.show(1);}}
    if(leap)return;
    if(debugState.leapAt!==null&&now>=debugState.leapAt){if(startLeap(-1-leaps,now,cam)||now>debugState.leapAt+5)debugState.leapAt=null;return;}
    while(cursor<P.LEAP_STARTS.length&&P.LEAP_STARTS[cursor]<now-LEAP.duration)cursor++;
    if(cursor<P.LEAP_STARTS.length&&P.LEAP_STARTS[cursor]<=now){startLeap(cursor,P.LEAP_STARTS[cursor],cam);cursor++;}
  }
  // ---- big animals ----
  const tilt={x:0,y:0,z:0};
  function poseOf(B,i,t){
    const kind=B.def.kind,p=B.poses[i];
    if(kind==='jelly'){P.jellyPosition(i,t,tilt);const J=P.JELLY_HOMES[i],yaw=J.tilt+t*.03,pitch=.28*Math.sin(t*.07+J.tilt),c=Math.cos(pitch);
      p.x=tilt.x;p.y=tilt.y;p.z=tilt.z;p.fx=Math.cos(yaw)*c;p.fy=Math.sin(pitch);p.fz=Math.sin(yaw)*c;p.roll=.28*Math.cos(t*.05+J.tilt*1.3);p.size=SCALE.jelly*J.size;const pin=pins['jelly'+i];if(pin){p.x=pin.x;p.z=pin.z;p.y=SEA-pin.depth;}return;}
    P.seaPose(kind,i,t+(offsets[kind][i]||0),B.n,p);p.size=SCALE[kind];
    const pin=pins[kind+i];if(pin){p.x=pin.x;p.z=pin.z;p.y=SEA-pin.depth;}
  }
  function updateBig(B,active){
    const a=B.batch.matrix.array,ia=B.batch.inst.array;let n=0;
    const jelly=B.def.kind==='jelly';
    for(let i=0;i<B.n;i++){
      // a jelly well out of reach keeps its last pose (every jelly is posed once, so locate() always answers)
      if(jelly&&B.posed[i]&&Math.hypot(P.JELLY_HOMES[i].x-camPos.x,P.JELLY_HOMES[i].z-camPos.z)>B.drawn+14)continue;
      poseOf(B,i,now);B.posed[i]=1;
      if(!active)continue;const p=B.poses[i],r=B.model.radius*p.size*1.15;
      sphere.center.set(p.x,p.y,p.z);sphere.radius=r;
      if(camPos.distanceTo(sphere.center)-r>B.drawn||!frustum.intersectsSphere(sphere))continue;
      place(a,n*16,p.x,p.y,p.z,p.fx,p.fy,p.fz,p.size,p.roll);
      ia.set(B.stroke.subarray(i*4,i*4+4),n*4);if(B.batch.tint)B.batch.tint.array.set(B.tints.subarray(i*3,i*3+3),n*3);n++;
    }
    B.shown=n;B.batch.show(n);
  }
  const puffOut={x:0,y:0,z:0,size:1},ringHit={k:-1},evHit={k:0,u:0},whaleBase={x:0,y:0,z:0,fx:0,fy:0,fz:1,roll:0};
  function updateEffects(active){
    const R=pop.rings,Pf=pop.puffs;let n=0;
    // the whale's surge and dive throw water: two rings per surfacing, registered when the window is first seen
    const whale=pop.big.find(b=>b.def.kind==='whale');
    if(whale){const e=P.eventAt(P.WHALE_EVENTS,now+(offsets.whale[0]||0),P.SURFACING.whale.rise,P.SURFACING.whale.hold+P.SURFACING.whale.fall,evHit);
      if(e&&e.k!==ringHit.k){ringHit.k=e.k;const base=now+(offsets.whale[0]||0),p=whaleBase;
        for(const du of [-1,3.8]){P.seaPose('whale',0,base-e.u+du,1,p);addRing(p.x,p.z,now-e.u+du,4.5);}}}
    for(let i=R.list.length-1;i>=0;i--)if(now-R.list[i].t0>LEAP.ring*1.5)R.list.splice(i,1);
    const a=R.mesh.instanceMatrix.array,ra=R.attr.array;
    for(const r of R.list){const age=(now-r.t0)/LEAP.ring;if(age<0||age>1||n>=16)continue;
      place(a,n*16,r.x,SEA+.03,r.z,0,0,1,r.size*(.25+1.5*age),0);ra[n*2]=age;ra[n*2+1]=1;n++;}
    R.mesh.count=active?n:0;R.mesh.visible=active&&n>0;R.mesh.instanceMatrix.needsUpdate=true;R.attr.needsUpdate=true;
    let m=0;const pa=Pf.mesh.instanceMatrix.array,pr=Pf.attr.array,tw=now+(offsets.whale[0]||0);
    if(whale)for(let i=0;i<BLOW.count;i++){const age=P.blowPuff(tw,i,puffOut);if(age<0)continue;
      place(pa,m*16,puffOut.x,puffOut.y,puffOut.z,0,0,1,puffOut.size,0);pr[m*2]=age;pr[m*2+1]=i;m++;}
    Pf.mesh.count=active?m:0;Pf.mesh.visible=active&&m>0;Pf.mesh.instanceMatrix.needsUpdate=true;Pf.attr.needsUpdate=true;
  }
  // ---- loading ----
  function ensureLoaded(){
    if(loading)return loading;
    loading=Promise.allSettled(MODELS.map(async m=>prepareModel(await load(FOLDERS[m.folder],m.file),m.length?{...speciesOf(m.id),length:m.length}:speciesOf(m.id),m.fixup))).then(results=>{
      results.forEach((r,i)=>{if(r.status==='fulfilled'){if(disposed)r.value.parts.forEach(p=>p.geometry.dispose());else models.set(MODELS[i].id,r.value);}else failures++;});
      loaded=true;
    });
    return loading;
  }
  const rebuild=()=>{ensureLoaded().then(()=>{if(!disposed)build();});};
  return {
    start(){if(started)return;started=true;if(tier!=='low')rebuild();},
    setTier(value){tier=value;if(!started)return;if(tier==='low')clearPop();else rebuild();},
    // time: the atmosphere clock (frozen under reduced motion). Nothing is simulated or submitted when the camera cannot see water.
    update(time,camera,animated=true){
      if(!pop||disposed)return;
      const dt=last===null||!animated?0:Math.min(.1,Math.max(0,time-last));last=time;now=time;clock.value=time;
      if(debugState.hidden){hideAll();return;}
      camera.updateMatrixWorld();camera.getWorldPosition(camPos);proj.multiplyMatrices(camera.projectionMatrix,camera.matrixWorldInverse.copy(camera.matrixWorld).invert());frustum.setFromProjectionMatrix(proj);
      const active=P.waterInView(camPos.x,camPos.y,camPos.z),near=P.nearWater(camPos.x,camPos.y,camPos.z),lap=prof?(k=>{const t=performance.now();prof[k]=(prof[k]||0)+t-mark;mark=t;}):null;let mark=prof?performance.now():0;
      for(const S of pop.schools){
        const lod=active&&!debugState.skip.schools?P.schoolStride(camPos.distanceTo(S.centre),near):0;
        S.visible=lod>0&&(sphere.center.copy(S.centre),sphere.radius=S.radius,frustum.intersectsSphere(sphere));
        if(lap)lap('visibility');
        if(S.visible){
          setStride(S,lod);
          if(S.pin)holdSchool(S);
          else if(dt>0){S.acc+=dt;const st=BOIDS_STEP*lod;let n=0;while(S.acc>=st&&n<2){stepSchool(S,st);S.acc-=st;n++;}if(S.acc>=st)S.acc=0;}
          if(lap)lap('boids');writeSchool(S);if(lap)lap('write');}
        S.batch.show(S.visible?Math.ceil(S.school.count/S.stride):0);
        if(lap)lap('show');
      }
      for(const B of pop.big)updateBig(B,active&&!(debugState.skip[B.def.kind==='jelly'?'jellies':'big']));
      if(lap)lap('big');
      if(active)updateLeaps(camPos);else if(leap)endLeap();
      if(lap)lap('leaps');
      updateEffects(active);
      if(!active)for(const l of pop.leapers.values())l.show(0);
      if(lap){lap('effects');prof.frames=(prof.frames||0)+1;}
    },
    // Pictures: force events deterministically. leapNow/leapAt (seconds from now); at:{kind,index,time}: that animal's own clock; whaleSurfaceAt,
    // turtleSurfaceAt (turtle index in `turtle`), sharkFinOut (index): the event is at its most visible that many seconds from now; reset.
    debug(o={}){
      if(o.reset){for(const k of Object.keys(offsets))offsets[k].length=0;debugState.leapAt=null;debugState.hidden=false;for(const k of Object.keys(pins))delete pins[k];pop?.schools.forEach(S=>{if(S.pin){S.pin=null;S.centre.copy(S.home);S.radius=S.homeRadius;}});}
      if(o.skip)debugState.skip=o.skip;   // measuring aid: {schools,big,jellies:true} leaves that group out
      if(o.profile!==undefined)prof=o.profile?{}:null;
      if(o.hidden!==undefined){debugState.hidden=!!o.hidden;if(!o.hidden)last=null;}
      if(o.pin)pins[o.pin.kind+(o.pin.index||0)]={x:o.pin.x,z:o.pin.z,depth:o.pin.depth??.8};
      if(o.pinSchool){const S=pop?.schools[o.pinSchool.index];if(S){S.pin={x:o.pinSchool.x,z:o.pinSchool.z,depth:o.pinSchool.depth??.8};S.centre.set(S.pin.x,SEA-S.pin.depth,S.pin.z);S.radius=6;}}
      if(o.leapNow)debugState.leapAt=now;
      if(o.leapAt!==undefined)debugState.leapAt=now+o.leapAt;
      if(o.at)offsets[o.at.kind][o.at.index||0]=o.at.time-now;
      if(o.whaleSurfaceAt!==undefined)offsets.whale[0]=P.WHALE_EVENTS[1]+2-o.whaleSurfaceAt-now;
      if(o.turtleSurfaceAt!==undefined){const i=o.turtle||0,n=pop?.big.find(b=>b.def.kind==='turtle')?.n||4;offsets.turtle[i]=P.TURTLE_EVENTS[i+2*n]+1.5-o.turtleSurfaceAt-now;}
      if(o.sharkFinOut!==undefined){const i=o.sharkFinOut,L=P.LOOPS.shark[i],t=(((-Math.PI/2-L.ph)/TAU%1)+1)%1*L.period+L.period*10;offsets.shark[i]=t-(o.sharkFinOutAt||0)-now;}
      return {profile:prof,leapAt:debugState.leapAt,hidden:debugState.hidden,offsets:Object.fromEntries(Object.entries(offsets).map(([k,v])=>[k,v.slice()]))};
    },
    // World position (and nose direction where it has one) of school / shark / hammer / turtle / whale / manta / jelly / leap number index.
    locate(kind,index=0){
      if(!pop)return null;
      if(kind==='school'){const S=pop.schools[index];if(!S)return null;const p=S.school.pos;let x=0,y=0,z=0;for(let i=0;i<S.school.count;i++){x+=p[i*3];y+=p[i*3+1];z+=p[i*3+2];}const n=S.school.count||1;return {x:x/n,y:y/n,z:z/n,species:S.h.species,visible:S.visible};}
      if(kind==='leap'){if(!leap)return null;return {x:leap.x0+leapOut.x,y:SEA+leapOut.h,z:leap.z0+leapOut.z};}
      const B=pop.big.find(b=>b.def.kind===kind),p=B?.poses[index];
      return p?{x:p.x,y:p.y,z:p.z,fx:p.fx,fy:p.fy,fz:p.fz}:null;
    },
    stats(){
      const big=kind=>pop?.big.find(b=>b.def.kind===kind)?.n||0;
      let fish=0,visibleFish=0,visibleSchools=0,calls=0;
      if(pop){for(const S of pop.schools){fish+=S.school.count;if(S.visible){visibleFish+=Math.ceil(S.school.count/S.stride);visibleSchools++;}calls+=S.batch.visible()?S.batch.meshes.length:0;}
        for(const B of pop.big)if(B.batch.visible())calls+=B.batch.meshes.length;
        for(const l of pop.leapers.values())if(l.visible())calls+=l.meshes.length;
        calls+=(pop.rings.mesh.visible?1:0)+(pop.puffs.mesh.visible?1:0);}
      return {loaded:!!pop,failures,tier,schools:pop?.schools.length||0,fish,visibleFish,visibleSchools,sharks:big('shark'),hammerheads:big('hammer'),turtles:big('turtle'),whale:big('whale'),mantas:big('manta'),jellies:big('jelly'),
        visibleBig:pop?pop.big.reduce((s,b)=>s+b.shown,0):0,leaps,drawCalls:calls,splashes:pop?pop.rings.mesh.count:0,puffs:pop?pop.puffs.mesh.count:0,materials:materials?.count()||0};
    },
    dispose(){disposed=true;clearPop();materials?.dispose();for(const m of models.values())m.parts.forEach(p=>p.geometry.dispose());models.clear();holder.removeFromParent();},
  };
}
