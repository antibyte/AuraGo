import * as THREE from 'three';
import {GLTFLoader} from 'three/addons/loaders/GLTFLoader.js';
import {interiors,stations,tramWaypoints,dronePad,roomAt,canWalk,groundHeight,buildingFloor} from './sysworld-exploration.js';
import {surfaces,liftContains,upperWalkable,roomFloorTiles,skyDeck,skyWalkable} from './sysworld-layout.js';
import {streetCurve} from './sysworld-navigation.js';
import {bodyShape,createTraffic} from './sysworld-traffic.js';
import {livingPlaces} from './sysworld-places.js';
import {createSociety} from './sysworld-society.js';
import {createMachinery} from './sysworld-machinery.js';

// All art is Blender-authored. This layer owns poses, navigation, interactions and
// animation mixers; it is stepped by the existing city RAF, never its own timer.
export function createExperience(scene,options){
  const group=new THREE.Group();group.name='world-2';scene.add(group);
  const cache=new Map(),geometries=new Set(),materials=new Set(),mixers=[],residents=[],trams=[],rooms=new Map(),doors=new Map();
  const staticGroups=new Map(),shells=new Map(),loaded=new Set(),bindings=[],textures=new Set(),freight=[],missionStates=new Map(),controller=new AbortController();
  let memoryTexts=[],freightEvents=0,missionSeeded=false;
  let disposed=false,tier=options.tier||'high',manifest=null,bytes=0,clock=0,ride=null,near=null,footClock=0,visited=new Set(),room=null,loading=0;
  const tmp=new THREE.Vector3(),ahead=new THREE.Vector3(),lastPosition=new THREE.Vector3(),camera=options.camera;
  // Sky deck lift cab (value = metres above the podium) and the telescopes on galleries and deck.
  const sky={ready:false,lift:null,cab:null,value:0,motion:null,idle:0},scopes=[],euler=new THREE.Euler(0,0,0,'YXZ');
  const wrapAngle=a=>Math.atan2(Math.sin(a),Math.cos(a)),smooth=t=>t<=0?0:t>=1?1:t*t*(3-2*t);
  const traffic=options.traffic||createTraffic(),machinery=createMachinery(scene),living=new Map();
  function placeFloor(x,z){
    let height=groundHeight(x,z);
    for(const place of livingPlaces)for(const s of manifest?.assets.find(a=>a.id===place.id)?.navigation.surfaces||[]){const [x0,z0,x1,z1]=s.rect;if(x>=place.x+x0&&x<=place.x+x1&&z>=place.z+z0&&z<=place.z+z1)height=Math.max(height,groundHeight(place.x,place.z)+s.height);}
    return height;
  }
  const society=createSociety(scene,{traffic,camera,floor:placeFloor,onSelect:options.onSociety,onDemonstrate:machinery.demonstrate,active:options.active});
  let placementSequence=0,lodClock=0;
  try{visited=new Set(JSON.parse(localStorage.getItem('aurago.desktop.sysworld.discoveries')||'[]').filter(id=>stations.some(s=>s.id===id)));}catch(_){}
  const route=streetCurve(tramWaypoints),length=route.getLength();
  const stops=stations.map(s=>{let best=Infinity,u=0;for(let i=0;i<3000;i++){route.getPointAt(i/3000,tmp);const d=Math.hypot(tmp.x-s.x,tmp.z-s.z);if(d<best){best=d;u=i/3000;}}return {...s,u};});
  const callSound=(kind,position)=>options.onSound?.(kind,position?.x||0,position?.y||0,position?.z||0,!!room);
  // The visible rails use the exact bounded curve followed by both trams.
  const railMaterial=new THREE.MeshStandardMaterial({color:0xb49360,metalness:.8,roughness:.35});materials.add(railMaterial);
  for(const side of[-1,1]){
    const points=[],indices=[],segments=512;
    for(let i=0;i<=segments;i++){
      route.getPointAt(i/segments,tmp);route.getTangentAt(i/segments,ahead);
      for(const width of[-.045,.045]){const offset=side*.85+width;points.push(tmp.x-ahead.z*offset,.21,tmp.z+ahead.x*offset);}
      if(i<segments){const n=i*2;indices.push(n,n+1,n+2,n+1,n+3,n+2);}
    }
    const geometry=new THREE.BufferGeometry();geometry.setAttribute('position',new THREE.Float32BufferAttribute(points,3));geometry.setIndex(indices);geometry.computeVertexNormals();geometries.add(geometry);group.add(new THREE.Mesh(geometry,railMaterial));
  }
  async function template(id,level=tier==='low'?2:tier==='medium'?1:0){
    const key=id+':'+level;
    if(!cache.has(key))cache.set(key,(async()=>{
      const entry=manifest.assets.find(a=>a.id===id),lod=entry?.lods.find(l=>l.level===level);
      if(!lod||!/^[\w-]+\.lod[0-2]\.glb$/.test(lod.file))throw Error('Invalid world asset');
      const response=await fetch(options.assetURL(lod.file),{signal:controller.signal});if(!response.ok)throw Error('World asset unavailable');
      const data=await response.arrayBuffer();if(disposed)throw Error('Disposed');bytes+=data.byteLength;
      const gltf=await new GLTFLoader().parseAsync(data,'');
      // Moving or articulated assets use object-space surface detail so it never slides over them.
      const moving=gltf.animations.length>0||/^(tram|service-cart|robot-|door|lift|sky-lift|telescope)/.test(id);
      gltf.scene.traverse(n=>{if(n.isMesh){geometries.add(n.geometry);for(const m of(Array.isArray(n.material)?n.material:[n.material]))materials.add(m);n.castShadow=true;n.receiveShadow=true;options.surfaces?.prepare(n,moving);}});
      if(disposed){geometries.forEach(g=>g.dispose());materials.forEach(m=>m.dispose());throw Error('Disposed');}
      loaded.add(id);return gltf;
    })());
    return cache.get(key);
  }
  async function place(id,x,z,y=0,angle=0,parent=group,scale=[1,1,1],level){
    const gltf=await template(id,level);if(disposed)return null;
    const node=gltf.scene.clone(true);node.position.set(x,y,z);node.rotation.y=angle;node.scale.set(...scale);parent.add(node);
    node.traverse(n=>{if(n.isMesh){n.userData.worldAsset=id;n.userData.worldPart=n.name;bindings.push(n);}});
    const entry=manifest.assets.find(a=>a.id===id),owner='furnishing:'+placementSequence++;
    for(const [index,b]of(entry.navigation.colliders||[]).entries())traffic.solid(owner+':'+index,{owner,x,z,heading:angle,min:[b[0]*scale[0],y+b[1]*scale[1],b[2]*scale[2]],max:[b[3]*scale[0],y+b[4]*scale[1],b[5]*scale[2]]});
    const mixer=gltf.animations.length?new THREE.AnimationMixer(node):null;
    if(mixer)mixers.push(mixer);
    let current=null;
    return {node,mixer,owner,bounds:entry.motion_bounds||entry.lods[0].bounds,clips:gltf.animations,play(name,once=false){
      if(!mixer)return;const clip=gltf.animations.find(c=>c.name===name);if(!clip)return;
      const action=mixer.clipAction(clip);if(current===action)return action;
      action.reset();if(once){action.setLoop(THREE.LoopOnce,1);action.clampWhenFinished=true;}
      action.play();if(current)action.crossFadeFrom(current,.25,false);current=action;return action;
    }};
  }
  // Batch static furnishings by shared primitive and material, retaining no per-instance draws.
  function batch(parent){
    parent.updateMatrixWorld(true);const primitives=new Map();
    parent.traverse(n=>{if(!n.isMesh)return;const key=n.geometry.uuid+':'+(Array.isArray(n.material)?n.material.map(m=>m.uuid).join(','):n.material.uuid);if(!primitives.has(key))primitives.set(key,{n,matrices:[]});primitives.get(key).matrices.push(n.matrixWorld.clone());});
    const out=new THREE.Group();group.add(out);
    for(const{n,matrices}of primitives.values()){const mesh=new THREE.InstancedMesh(n.geometry,n.material,matrices.length);matrices.forEach((m,i)=>mesh.setMatrixAt(i,m));mesh.castShadow=true;mesh.receiveShadow=true;mesh.userData={...n.userData};bindings.push(mesh);out.add(mesh);}
    parent.traverse(n=>{const index=bindings.indexOf(n);if(index>=0)bindings.splice(index,1);});
    parent.removeFromParent();staticGroups.set(out.uuid,out);return out;
  }
  function loadShell(r){
    if(shells.has(r.id))return shells.get(r.id);
    const pending=(async()=>{
      const shell=new THREE.Group();group.add(shell);
      const tasks=[];
      for(const tile of roomFloorTiles())tasks.push(place('floor',r.x+tile.x,r.z+tile.z,surfaces.room,0,shell,[tile.sx,1,tile.sz]));
      for(let x=-4;x<=4;x+=4)for(let z=-4;z<=4;z+=4)tasks.push(place('ceiling',r.x+x,r.z+z,8.15+surfaces.room,0,shell));
      for(let x=-4;x<=4;x+=4)tasks.push(place('wall',r.x+x,r.z+6,surfaces.room,0,shell));
      for(let z=-4;z<=4;z+=4)for(const side of[-1,1])for(const level of[0,4])tasks.push(place('window',r.x+side*6,r.z+z,surfaces.room+level,Math.PI/2,shell));
      for(let x=-4;x<=4;x+=4)for(const side of[-1,1])tasks.push(place('window',r.x+x,r.z+side*6,surfaces.gallery,0,shell));
      for(const x of[-4,4])tasks.push(place('wall',r.x+x,r.doorZ,surfaces.room,0,shell));
      // A six-metre-deep gallery, with a real shaft opening and guards. The
      // floor ends exactly at the lift's left edge, never beneath its platform.
      for(const [x,sx]of[[-4,1],[.2,1.1]]){
        tasks.push(place('floor',r.x+x,r.z+3,surfaces.gallery,0,shell,[sx,1,1.5]));
        tasks.push(place('railing',r.x+x,r.z,surfaces.gallery,0,shell,[sx,1,1]));
      }
      for(const z of[.7,5.3])tasks.push(place('railing',r.x+2.4,r.z+z,surfaces.gallery,Math.PI/2,shell,[.35,1,1]));
      for(const x of[-5.6,1.9])tasks.push(place('wall',r.x+x,r.z+.15,surfaces.room,0,shell,[.05,1,.6]));
      await Promise.all(tasks);if(!disposed)return batch(shell);
    })();shells.set(r.id,pending);return pending;
  }
  async function loadRoom(r){
    if(rooms.has(r.id)||disposed)return;rooms.set(r.id,{ready:false,lift:null,liftValue:0,liftTarget:0});loading++;
    try{
      await loadShell(r);const furniture=new THREE.Group();group.add(furniture);const tasks=[];
      for(const x of[-3,0])tasks.push(place(r.id==='missions'?'cargo':r.id==='memory'?'archive-shelf':'console',r.x+x,r.z+3,surfaces.room,Math.PI,furniture));
      tasks.push(place('bench',r.x-4,r.z-1,surfaces.room,Math.PI/2,furniture));
      tasks.push(place('bench',r.x-4,r.z+4.8,surfaces.gallery,0,furniture));
      tasks.push(place('console',r.x,r.z+4.8,surfaces.gallery,Math.PI,furniture));
      await Promise.all(tasks);if(disposed)return;batch(furniture);
      const info=rooms.get(r.id);
      info.hologram=await place('hologram',r.x-1,r.z,surfaces.room);info.hologram?.play('operate');
      if(r.id==='memory')drawMemory();
      info.lift=await place('lift',r.liftX,r.liftZ,surfaces.room);info.liftAction=info.lift?.play('operate',true);if(info.liftAction)info.liftAction.paused=true;
      // Gallery viewer at the atrium railing, looking north over the city through the upper windows.
      await addScope(r.id,r.x-2.2,r.z+.9,surfaces.gallery,Math.PI,{skip:null,limit:.9,pitch:[-.14,.18],eye:0});
      info.ready=true;
    }catch(e){if(!disposed)options.onError?.(e);}finally{loading--;}
  }
  async function init(){
    try{
      const response=await fetch(options.assetURL('manifest.json'),{signal:controller.signal});if(!response.ok)throw Error('World manifest unavailable');manifest=await response.json();
      const outdoors=new THREE.Group();group.add(outdoors);const tasks=[];
      for(const s of stations)tasks.push(place('station',s.platformX,s.platformZ,surfaces.pavement,s.angle,outdoors));
      for(const r of interiors){tasks.push(loadShell(r));const d=await place('door',r.x,r.doorZ,surfaces.room,Math.PI);if(disposed)return;const a=d.play('open',true);a.paused=true;doors.set(r.id,{...d,action:a,value:0,open:false});}
      for(let x=-76;x<=76;x+=8)tasks.push(place('quay',x,77,surfaces.quay,0,outdoors));
      for(const [x,z]of[[-79,68],[-79,-67],[55,2],[8,40],[55,-24]]){tasks.push(place('garden',x,z,0,0,outdoors));tasks.push(place('bench',x+3,z,0,Math.PI/2,outdoors));}
      for(const [x,z]of[[-30,23],[30,23],[-28,72]])tasks.push(place('arcade',x,z,0,0,outdoors));
      tasks.push(place('bridge',-78.5,40,2,0,outdoors,[1.75,1,1]),place('ramp',-80,49,0,Math.PI,outdoors),place('stairs',-76.8,49,0,Math.PI,outdoors),place('ramp',-78.5,31,0,0,outdoors));
      tasks.push(place('pad',dronePad.x,dronePad.z,0,0,outdoors));
      tasks.push(place('sky-deck',skyDeck.x,skyDeck.z,skyDeck.floor,0,outdoors));
      for(const b of skyDeck.benches)tasks.push(place('bench',b.x,b.z,skyDeck.floor,b.angle,outdoors));
      for(const [x,z]of[[-60,-56],[-60,-46],[51,49]])tasks.push(place('charger',x,z,0,0,outdoors));
      await Promise.all(tasks);if(disposed)return;batch(outdoors);
      sky.lift=await place('sky-lift',skyDeck.lift.x,skyDeck.lift.z,skyDeck.lift.base);if(disposed)return;
      sky.cab=sky.lift?.node.getObjectByName('cab')||null;sky.ready=!!sky.cab;
      // Deck viewers look from a virtual objective just beyond the balustrade, so steep views
      // down onto the districts are never blocked by the terrace floor.
      for(const t of skyDeck.telescopes)await addScope(t.id,t.x,t.z,skyDeck.floor,t.angle,{skip:'agent',limit:1.45,pitch:[-1.1,.3],eye:1.25});
      for(const [x,z]of[[-57,-61],[-57,-52]]){const cooler=await place('cooler',x,z);cooler?.play('operate');}
      for(const p of livingPlaces) {
        const model=await place(p.id,p.x,p.z,groundHeight(p.x,p.z),0,group,[1,1,1],2);if(disposed)return;
        if(model){model.level=2;living.set(p.id,model);machinery.attach(p.id,model);society.addPlace(p,manifest.assets.find(a=>a.id===p.id));model.play('operate');}
      }
      const cart=await place('service-cart',55,49,surfaces.ground,Math.PI/2);if(cart)cart.play('open',true);
      for(let i=0;i<19;i++){
        const type=['courier','technician','archivist'][i%3],body=await place('robot-'+type,0,0);if(!body)return;
        body.node.scale.setScalar(1.15);residents.push(body);society.add(body,i,body.bounds);body.play('idle');
      }
      for(let i=0;i<2;i++){const body=await place('tram',0,0);if(!body)return;const door=body.play('open',true);door.paused=true;
        const u=stops[i?3:0].u;route.getPointAt(u,tmp);route.getTangentAt(u,ahead);
        const mover=traffic.register('tram-'+i,bodyShape(body.bounds),{x:tmp.x,y:surfaces.road+.03,z:tmp.z,heading:Math.atan2(ahead.x,ahead.z)},3);
        trams.push({...body,body:mover,u,speed:0,dwell:5,stop:i?3:0,door});}
      for(let i=0;i<3;i++){const box=await place('cargo',43,70);if(box){traffic.removeOwner(box.owner);box.node.visible=false;freight.push({...box,body:null,elapsed:9,direction:1});}}
      options.onReady?.();
    }catch(e){if(!disposed)options.onError?.(e);}
  }
  void init();
  async function livingLOD(id,model,level){
    if(model.level===level||model.requested===level)return;model.requested=level;
    try{const gltf=await template(id,level);if(disposed||model.requested!==level)return;
      const parts=new Map();gltf.scene.traverse(n=>{if(n.isMesh)parts.set(n.name,n);});
      model.node.traverse(n=>{const replacement=parts.get(n.name);if(n.isMesh&&replacement){n.geometry=replacement.geometry;n.material=replacement.material;}});
      model.level=level;model.requested=null;options.onReady?.();
    }catch(e){model.requested=null;if(!disposed)options.onError?.(e);}
  }
  function drawMemory(){
    const info=rooms.get('memory');if(!info?.hologram||(!info.text&&!memoryTexts.length))return;
    if(!info.text){const canvas=document.createElement('canvas');canvas.width=1024;canvas.height=512;const texture=new THREE.CanvasTexture(canvas);texture.colorSpace=THREE.SRGBColorSpace;textures.add(texture);
      const geometry=new THREE.PlaneGeometry(4.8,2.4),material=new THREE.MeshBasicMaterial({map:texture,transparent:true,depthWrite:false,side:THREE.DoubleSide,toneMapped:false});geometries.add(geometry);materials.add(material);
      const mesh=new THREE.Mesh(geometry,material),r=interiors.find(r=>r.id==='memory');mesh.position.set(r.x-1,3.6+surfaces.room,r.z+.3);mesh.rotation.y=Math.PI;group.add(mesh);info.text={canvas,texture,mesh};}
    const {canvas,texture,mesh}=info.text,ctx=canvas.getContext('2d');ctx.clearRect(0,0,1024,512);mesh.visible=memoryTexts.length>0;
    ctx.fillStyle='rgba(5,24,32,.87)';ctx.fillRect(0,0,1024,512);ctx.fillStyle='#b9f4ef';ctx.font='28px sans-serif';
    // The existing protected sampler owns these excerpts. Canvas only, no diagnostics or history.
    memoryTexts.slice(0,4).forEach((text,i)=>{const chars=Array.from(text).slice(0,96);ctx.fillText(chars.slice(0,48).join(''),30,55+i*118);ctx.fillText(chars.slice(48).join(''),30,94+i*118);});texture.needsUpdate=true;
  }
  async function addScope(id,x,z,floor,angle,view){
    const model=await place('telescope',x,z,floor,angle);if(!model||disposed)return;
    scopes.push({id,model,tube:model.node.getObjectByName('tube'),x,z,floor,angle,...view,ex:x+Math.sin(angle)*view.eye,ez:z+Math.cos(angle)*view.eye});
  }
  // Looking through a viewer: the camera sits at its pivot, aims at the district nearest to
  // the viewer's resting direction and zooms in; the viewer itself hides meanwhile.
  function startScope(id){
    const s=scopes.find(s=>s.id===id);if(!s)return;
    const yaw0=wrapAngle(s.angle+Math.PI),eyeY=s.floor+2.05;let yaw=yaw0,pitch=-.08,best=Infinity;
    for(const d of options.districts||[]){
      if(d.id===s.skip)continue;const dx=d.x-s.ex,dz=d.z-s.ez,distance=Math.hypot(dx,dz),aim=Math.atan2(-dx,-dz),off=Math.abs(wrapAngle(aim-yaw0));
      if(distance>12&&off<s.limit&&off<best){best=off;yaw=aim;pitch=THREE.MathUtils.clamp(Math.atan2(d.height*.45-eyeY,distance),...s.pitch);}
    }
    ride={kind:'scope',id,scope:s,yaw0,fov:14,sent:'',target:null,back:{position:camera.position.clone(),quaternion:camera.quaternion.clone(),fov:camera.fov}};
    s.model.node.visible=false;camera.position.set(s.ex,eyeY,s.ez);camera.quaternion.setFromEuler(euler.set(pitch,yaw,0,'YXZ'));
    if(options.reduced()){camera.fov=ride.fov;camera.updateProjectionMatrix();}
    options.onRide?.('scope');callSound('door',camera.position);
  }
  function hitDistrict(origin,dir,d){
    const r=d.radius*.6,ox=origin.x-d.x,oz=origin.z-d.z,a=dir.x*dir.x+dir.z*dir.z;if(a<1e-9)return Infinity;
    const b=2*(ox*dir.x+oz*dir.z),disc=b*b-4*a*(ox*ox+oz*oz-r*r);if(disc<0)return Infinity;
    let t=(-b-Math.sqrt(disc))/(2*a);if(t<0)t=(-b+Math.sqrt(disc))/(2*a);if(t<0)return Infinity;
    const y=origin.y+dir.y*t;return y>=0&&y<=d.height?t:Infinity;
  }
  function updateScope(dt){
    const s=ride.scope;euler.setFromQuaternion(camera.quaternion,'YXZ');
    euler.y=ride.yaw0+THREE.MathUtils.clamp(wrapAngle(euler.y-ride.yaw0),-s.limit,s.limit);euler.x=THREE.MathUtils.clamp(euler.x,...s.pitch);euler.z=0;
    camera.quaternion.setFromEuler(euler);camera.position.set(s.ex,s.floor+2.05,s.ez);
    let fov=options.reduced()?ride.fov:THREE.MathUtils.damp(camera.fov,ride.fov,9,Math.min(dt,.1));if(Math.abs(fov-ride.fov)<.02)fov=ride.fov;
    if(fov!==camera.fov){camera.fov=fov;camera.updateProjectionMatrix();}
    camera.getWorldDirection(tmp);let target=null,distance=Infinity;
    for(const d of options.districts||[]){if(d.id===s.skip)continue;const t=hitDistrict(camera.position,tmp,d);if(t>12&&t<distance){distance=t;target=d.id;}}
    ride.target=target;const zoom=ride.back.fov/camera.fov,bearing=(Math.atan2(tmp.x,-tmp.z)*180/Math.PI+360)%360;
    const key=[target,target?Math.round(distance):0,zoom.toFixed(1),Math.round(bearing)].join();
    if(key!==ride.sent){ride.sent=key;options.onScope?.({id:ride.id,target,distance:target?Math.round(distance):null,zoom:+zoom.toFixed(1),bearing:Math.round(bearing)%360});}
  }
  // The open east lift of the agent tower: rides take 15 s, an empty cab answers a call
  // in 7 s and, while nobody is near, makes an occasional trip so the tower looks alive.
  function moveCab(to,duration,sound){
    sky.motion={from:sky.value,to,t:0,duration:options.reduced()?0:duration};sky.idle=0;
    if(sound)callSound('lift',sky.lift.node.position);
  }
  function cabCamera(){const lift=skyDeck.lift;camera.position.set(lift.x,lift.base+lift.cab+sky.value+2.4,lift.z);}
  function rideSky(to){
    moveCab(to,15,true);ride={kind:'sky',to};cabCamera();
    // Face south through the unbraced shaft side, over the plaza and the bay; free look stays.
    camera.quaternion.setFromEuler(euler.set(-.42,Math.PI,0,'YXZ'));options.onRide?.('sky');
  }
  function updateSky(dt,step){
    const lift=skyDeck.lift;
    if(!sky.motion&&ride?.kind!=='sky'&&step>0&&Math.hypot(camera.position.x-lift.x,camera.position.z-lift.z)>30){sky.idle+=step;if(sky.idle>28)moveCab(sky.value>1?0:lift.travel,15,false);}
    if(sky.motion){const m=sky.motion;m.t+=Math.min(dt,.1);const p=m.duration?m.t/m.duration:1;sky.value=m.from+(m.to-m.from)*smooth(p);if(p>=1){sky.value=m.to;sky.motion=null;}}
    sky.cab.position.y=sky.value;
    if(ride?.kind==='sky'){cabCamera();if(!sky.motion){ride=null;options.onRide?.(null);callSound('lift',camera.position);}}
  }
  function interact(){
    if(ride){if(ride.kind==='tram'&&trams[ride.index].dwell<=1){ride.exitRequested=true;return;}endRide();return;}
    if(!near)return;
    if(near.kind==='telescope'){startScope(near.id);return;}
    if(near.kind==='skycall'){moveCab(camera.position.y>40?skyDeck.lift.travel:0,7,true);return;}
    if(near.kind==='skyup'||near.kind==='skydown'){rideSky(near.kind==='skyup'?skyDeck.lift.travel:0);return;}
    if(near.kind==='resident'){society.inspect(near.id);return;}
    if(near.kind==='demonstrate'){society.demonstrate(near.id,options.reduced());options.onSociety?.({id:near.id,role:'installation',state:options.reduced()?'idle':'work',source:'ambient'});return;}
    if(near.kind==='door'){const d=doors.get(near.id);if(d){d.open=!d.open;callSound('door',d.node.position);const r=interiors.find(r=>r.id===near.id);void loadRoom(r);}}
    if(near.kind==='discover'){visited.add(near.id);try{localStorage.setItem('aurago.desktop.sysworld.discoveries',JSON.stringify([...visited]));}catch(_){}options.onDiscover?.(near.id);callSound('discover',camera.position);}
    if(near.kind==='tram'&&!options.reduced()){ride={kind:'waiting',station:near.id};options.onRide?.('waiting');}
    if(near.kind==='terminal')options.onTerminal?.(near.id);
    if(near.kind==='drone'&&!options.reduced()){ride={kind:'drone',elapsed:0,origin:camera.position.clone()};options.onRide?.('drone');callSound('tram',camera.position);}
    if(near.kind==='lift'){const r=rooms.get(near.id);if(r){
      const level=camera.position.y>5?4:0;
      if(Math.abs(r.liftValue-level)>.02){r.liftTarget=level;return;}
      r.liftTarget=level?0:4;ride={kind:'lift',id:near.id};options.onRide?.('lift');callSound('lift',camera.position);
    }}
  }
  function endRide(){
    if(!ride)return;
    if(ride.kind==='lift'){const r=rooms.get(ride.id),plot=interiors.find(p=>p.id===ride.id);r.liftTarget=r.liftValue<2?0:4;camera.position.set(plot.x+1.5,2.4+surfaces.room+r.liftTarget,plot.liftZ);}
    if(ride.kind==='tram'){const s=stations.find(s=>s.id===ride.station)||stations[0];camera.position.set(s.platformX,2.4+surfaces.pavement+.3,s.platformZ);}
    if(ride.kind==='drone')camera.position.copy(ride.origin);
    if(ride.kind==='sky'){sky.motion=null;sky.value=ride.to;sky.cab.position.y=sky.value;cabCamera();}
    if(ride.kind==='scope'){
      // Step back from the viewer: it keeps the aimed direction, the visitor keeps the heading.
      const s=ride.scope;euler.setFromQuaternion(camera.quaternion,'YXZ');s.tube?.rotation.set(-euler.x,wrapAngle(euler.y+Math.PI-s.angle),0,'YXZ');
      const heading=euler.y;s.model.node.visible=true;camera.position.copy(ride.back.position);euler.setFromQuaternion(ride.back.quaternion,'YXZ');euler.y=heading;
      camera.quaternion.setFromEuler(euler);camera.fov=ride.back.fov;camera.updateProjectionMatrix();options.onScope?.(null);
    }
    ride=null;options.onRide?.(null);
  }
  function candidates(){
    if(ride)return [{kind:ride.kind==='scope'?'unscope':'exit',id:ride.kind,distance:0}];
    camera.getWorldDirection(tmp);
    const result=society.nearby().filter(p=>(p.x-camera.position.x)*tmp.x+(p.z-camera.position.z)*tmp.z>p.distance*.3);
    const add=(kind,id,x,z,radius)=>{const distance=Math.hypot(camera.position.x-x,camera.position.z-z);if(distance<radius)result.push({kind,id,distance});};
    for(const r of interiors){if(camera.position.y<5)add('door',r.id,r.x,r.doorZ,3);if(rooms.get(r.id)?.ready){add('lift',r.id,r.liftX,r.liftZ,2.3);if(camera.position.y<5)add('terminal',r.id,r.x-2,r.z+3,2.5);}}
    if(camera.position.y<5)for(const s of stations){add('tram',s.id,s.platformX,s.platformZ,4);if(!visited.has(s.id))add('discover',s.id,s.platformX+(s.angle===0?5:0),s.platformZ+(s.angle===0?0:5),3);}
    add('drone','drone',dronePad.x,dronePad.z,5);
    if(sky.ready){
      const lift=skyDeck.lift,top=camera.position.y>40,distance=Math.hypot(camera.position.x-lift.x,camera.position.z-lift.z),here=top?lift.travel:0;
      if(distance<(top?3.6:2.4)&&(top||camera.position.y<10))result.push({kind:sky.motion||Math.abs(sky.value-here)>.05?'skycall':top?'skydown':'skyup',id:'sky',distance});
    }
    for(const s of scopes)if(Math.abs(camera.position.y-(s.floor+2.4))<1.5)add('telescope',s.id,s.x,s.z,1.7);
    return result.sort((a,b)=>a.distance-b.distance);
  }
  function update(dt,animated,mode){
    if(disposed||!manifest)return;const step=animated?Math.min(dt,.05):0;clock+=step;room=roomAt(camera.position.x,camera.position.z)?.id||null;
    lodClock+=dt;if(lodClock>.5){lodClock=0;for(const [id,model]of living){const distance=camera.position.distanceTo(model.node.position),level=tier==='low'||distance>55?2:tier==='medium'||distance>32?1:0;void livingLOD(id,model,level);}}
    if(!options.traffic)traffic.begin();
    for(const r of interiors)if(Math.hypot(camera.position.x-r.x,camera.position.z-r.z)<32)void loadRoom(r);
    mixers.forEach(m=>{if(m.getRoot().visible)m.update(step);});
    for(const d of doors.values()) {
      let next=THREE.MathUtils.damp(d.value,d.open?1:0,5,Math.min(dt,.1));if(Math.abs(next-(d.open?1:0))<.001)next=d.open?1:0;
      const panels=value=>[-1,1].map(side=>({owner:d.owner,x:d.node.position.x,z:d.node.position.z,min:[side*.8+side*1.6*value-.8,surfaces.room,-.13],max:[side*.8+side*1.6*value+.8,surfaces.room+3.6,.13]}));
      if(d.open||panels(next).every(p=>traffic.solidClear(p)))d.value=next;
      panels(d.value).forEach((p,i)=>traffic.solid(d.owner+':panel:'+i,p));
      d.action.time=d.value*d.action.getClip().duration;d.mixer.update(0);
    }
    if(sky.ready)updateSky(dt,step);
    if(ride?.kind==='scope')updateScope(dt);
    for(const [id,r]of rooms){if(!r.liftAction)continue;r.liftValue=THREE.MathUtils.damp(r.liftValue,r.liftTarget,1.8,Math.min(dt,.1));if(Math.abs(r.liftValue-r.liftTarget)<.02)r.liftValue=r.liftTarget;r.liftAction.time=r.liftValue/4*r.liftAction.getClip().duration;r.lift.mixer.update(0);if(ride?.kind==='lift'&&ride.id===id){const pos=interiors.find(i=>i.id===id);camera.position.set(pos.liftX,2.4+surfaces.room+r.liftValue,pos.liftZ);if(r.liftValue===r.liftTarget){ride=null;options.onRide?.(null);}}}
    society.update(step,animated);machinery.update(step,animated);
    trams.forEach((t,i)=>{
      t.node.visible=!!t.body&&i<(tier==='low'?1:2);
      if(t.body){if(t.node.visible&&!t.body.enabled&&!traffic.clear({...t.body,enabled:true},t.body,t.body))t.node.visible=false;t.body.enabled=t.node.visible;}
      if(!t.node.visible)return;
      const previous={u:t.u,dwell:t.dwell,stop:t.stop};let arrived=false;
      if(t.dwell>0){t.dwell-=step;t.speed=0;}else{
        const gap=s=>((s.u-t.u+1)%1)*length,station=stops.reduce((best,s)=>gap(s)>.00001&&gap(s)<gap(best)?s:best,stops.find(s=>gap(s)>.00001)||stops[0]);
        let desired=Math.min(11,Math.sqrt(gap(station)*8));
        for(let n=1;n<=3;n++){
          const u=(t.u+(1.2+t.speed*.7)*n/3/length)%1;route.getPointAt(u,tmp);route.getTangentAt(u,ahead);
          const probe={x:tmp.x,y:surfaces.road+.03,z:tmp.z,heading:Math.atan2(ahead.x,ahead.z)};
          if(!traffic.clear(t.body,probe,probe)){desired=0;break;}
        }
        t.speed+=THREE.MathUtils.clamp(desired-t.speed,-step*8,step*3.5);
        const advance=step*t.speed/length,previous=t.u;t.u=(t.u+advance)%1;
        const next=stops.findIndex(s=>((s.u-previous+1)%1)>0&&((s.u-previous+1)%1)<=advance+.00001);
        if(next>=0){t.stop=next;t.u=stops[next].u;t.dwell=6;t.speed=0;arrived=true;}
      }
      route.getPointAt(t.u,t.node.position);t.node.position.y=surfaces.road+.03;route.getTangentAt(t.u,ahead);t.node.rotation.y=Math.atan2(ahead.x,ahead.z);
      if(t.body){t.body.enabled=t.node.visible;traffic.propose(t.body,{...t.node.position,heading:t.node.rotation.y},(accepted,b)=>{if(!accepted){Object.assign(t,previous);t.speed=0;}t.node.position.set(b.x,b.y,b.z);t.node.rotation.y=b.heading;if(accepted&&arrived)callSound('tram',t.node.position);});}
      t.door.time=(t.dwell>1?1:0)*t.door.getClip().duration;t.mixer.update(0);
      if(ride?.kind==='waiting'&&stops[t.stop].id===ride.station&&t.dwell>1){ride={kind:'tram',index:i,station:ride.station,offset:0};options.onRide?.('tram');}
      if(ride?.kind==='tram'&&ride.index===i){if(t.dwell>1){ride.station=stops[t.stop].id;if(ride.exitRequested){endRide();return;}}camera.position.copy(t.node.position).addScaledVector(ahead,ride.offset);camera.position.y+=.9+1.4;camera.lookAt(t.node.position.x+ahead.x*16,camera.position.y,t.node.position.z+ahead.z*16);}
    });
    if(ride?.kind==='drone'){
      ride.elapsed+=Math.min(dt,.1);const a=ride.elapsed/40*Math.PI*2;
      camera.position.set(Math.cos(a)*100,45+Math.sin(a*2)*12,Math.sin(a)*95-10);camera.lookAt(0,22,-12);if(ride.elapsed>=40)endRide();
    }
    near=mode==='street'?candidates()[0]||null:null;
    if(mode==='street'&&!ride){const distance=camera.position.distanceTo(lastPosition);if(distance<3)footClock+=distance;if(footClock>1.8){callSound(room?'step_inside':'step',camera.position);footClock=0;}}
    lastPosition.copy(camera.position);options.onEnvironment?.(!!room);
    for(const [i,box]of freight.entries()) {
      box.elapsed+=step;box.node.visible=animated&&box.elapsed<8;
      if(!box.node.visible){if(box.body)traffic.remove(box.body.id);box.body=null;continue;}
      const t=box.direction>0?box.elapsed/8:1-box.elapsed/8,next={x:40+t*3,y:surfaces.room,z:70,heading:0};
      if(!box.body)box.body=traffic.register('freight-'+i,bodyShape(box.bounds),next,0);
      if(!box.body){box.elapsed=9;box.node.visible=false;continue;}
      traffic.propose(box.body,next,(accepted,b)=>{if(!accepted)box.elapsed-=step;box.node.position.set(b.x,b.y,b.z);});
    }
    if(!options.traffic)traffic.solve(step);
    options.onInteraction?.(near,visited.size,room);
  }
  async function setTier(value){tier=value;if(!manifest)return;const level=value==='low'?2:value==='medium'?1:0;
    society.setTier(value);machinery.setTier(value);
    try{const replacements=new Map(await Promise.all([...loaded].filter(id=>!living.has(id)).map(async id=>{const gltf=await template(id,level),parts=new Map();gltf.scene.traverse(n=>{if(n.isMesh)parts.set(n.name,n);});return [id,parts];})));
      if(disposed||tier!==value)return;for(const mesh of bindings){const n=replacements.get(mesh.userData.worldAsset)?.get(mesh.userData.worldPart);if(n){mesh.geometry=n.geometry;mesh.material=n.material;}}options.onReady?.();
    }catch(e){if(!disposed)options.onError?.(e);}
  }
  return {update,interact,endRide,setTier,society,
    suspend(){society.suspend();machinery.clear();missionSeeded=false;for(const box of freight){box.elapsed=9;box.node.visible=false;if(box.body)traffic.remove(box.body.id);box.body=null;}},
    syncRide(){if(ride?.kind==='tram'){const t=trams[ride.index];route.getTangentAt(t.u,ahead);camera.position.copy(t.node.position).addScaledVector(ahead,ride.offset);camera.position.y+=2.3;camera.lookAt(t.node.position.x+ahead.x*16,camera.position.y,t.node.position.z+ahead.z*16);}},
    socialAction(verb,id){const s=stations.find(s=>s.id===id);return society.action(options.reduced()&&verb==='guide'?'cancel':verb,s?{x:s.platformX,z:s.platformZ}:null);},
    setMemory(texts){memoryTexts=(Array.isArray(texts)?texts:[]).filter(t=>typeof t==='string').slice(0,8);drawMemory();},
    setWorld(snapshot,replay=false){
      if(replay){society.setReplay(true);machinery.clear();}else society.setReplay(false);
      const rows=snapshot?.entities?.filter(e=>e.kind==='mission')||[],seen=new Set();
      for(const e of rows){seen.add(e.id);const before=missionStates.get(e.id);if(!replay&&options.active()&&missionSeeded&&before!=null&&before!==e.state&&['running','completed','failed','cancelled'].includes(e.state)&&Date.now()-e.at<30000){const box=freight.find(b=>b.elapsed>=8);if(box){box.elapsed=0;box.direction=e.state==='running'?1:-1;freightEvents++;}}missionStates.set(e.id,e.state);}
      for(const id of missionStates.keys())if(!seen.has(id))missionStates.delete(id);missionSeeded=!replay;
      if(replay)for(const box of freight){box.elapsed=9;box.node.visible=false;}
    },
    walkRide(forward,dt,right=0){if(!ride)return false;if(ride.kind==='tram')ride.offset=THREE.MathUtils.clamp(ride.offset+forward*dt*3,-2.5,2.5);
      // Keys pan a viewer, slower when zoomed in; updateScope clamps to its field.
      if(ride.kind==='scope'){euler.setFromQuaternion(camera.quaternion,'YXZ');const k=dt*.9*camera.fov/ride.back.fov;euler.x+=forward*k;euler.y-=right*k;camera.quaternion.setFromEuler(euler);}
      return true;},
    zoom(delta){if(ride?.kind!=='scope')return false;ride.fov=THREE.MathUtils.clamp(ride.fov*Math.exp(delta*.0012),5,26);return true;},
    scoping:()=>ride?.kind==='scope',
    move(x,z,from){if(ride)return false;if(from.y>40)return skyWalkable(x,z,!sky.motion&&sky.value===skyDeck.lift.travel);const r=roomAt(from.x,from.z);if(r&&from.y>5){
        if(Math.abs(z-r.z-4.8)<.85&&(Math.abs(x-r.x)<1.1||Math.abs(x-r.x+4)<1.75))return false;
        return upperWalkable(r,x,z,rooms.get(r.id)?.liftValue===4);
      }
      if(r&&from.y<5){
        if([-5.6,1.9].some(dx=>Math.abs(x-r.x-dx)<.25&&Math.abs(z-r.z-.15)<.25))return false;
        if(liftContains(r,x,z)&&!(x<r.liftX+1.25&&Math.abs(z-r.liftZ)<1.25&&rooms.get(r.id)?.liftValue===0))return false;
        if(Math.hypot(x-(r.x-1),z-r.z)<1.55||[-3,0].some(dx=>Math.abs(x-r.x-dx)<1.1&&Math.abs(z-r.z-3)<.8))return false;
      }
      return canWalk(x,z,from,id=>doors.get(id)?.value>.85&&!!rooms.get(id)?.ready,options.districts);},
    floor(x,z){if(camera.position.y>40)return skyDeck.floor;const r=roomAt(x,z);if(r&&camera.position.y>5&&upperWalkable(r,x,z,rooms.get(r.id)?.liftValue===4))return surfaces.gallery;return Math.max(placeFloor(x,z),buildingFloor(x,z,options.districts));},
    visit(id){endRide();ride=null;
      const place=livingPlaces.find(p=>p.id===id);if(place){const point=manifest?.assets.find(a=>a.id===id)?.navigation.interaction[0]?.position||[0,1.5,4];camera.position.set(place.x,2.4+groundHeight(place.x,place.z),place.z+point[2]+1);camera.lookAt(place.x,2,place.z);return;}
      if(id==='drone'){camera.position.set(dronePad.x,2.7,dronePad.z+3);camera.lookAt(dronePad.x,2,dronePad.z);return;}
      // Sky deck arrival beside the north-east viewer, looking north over the skyline and coast.
      if(id==='skydeck'){const b=57*Math.PI/180,x=skyDeck.x+Math.sin(b)*7.2,z=skyDeck.z-Math.cos(b)*7.2;camera.position.set(x,skyDeck.floor+2.4,z);camera.lookAt(x,skyDeck.floor+.4,z-10);return;}const s=stations.find(s=>s.id===id);if(s){const offset=visited.has(id)?0:5,x=s.platformX+(s.angle===0?offset:0),z=s.platformZ+(s.angle===0?0:offset);camera.position.set(x,2.4+groundHeight(x,z),z);camera.lookAt(s.platformX,2,s.platformZ===z?s.platformZ+1:s.platformZ);}},
    destination(id){const r=interiors.find(r=>r.id===id);if(r){endRide();ride=null;const z=r.doorZ+r.front*4;camera.position.set(r.x,2.4+groundHeight(r.x,z),z);camera.lookAt(r.x,2.4+groundHeight(r.x,z),r.z);}},
    isRiding:()=>!!ride,interaction:()=>near,
    rideBody:()=>ride?.kind==='tram'?'tram-'+ride.index:ride?.kind==='scope'?ride.scope.model.owner:ride?.kind==='sky'?sky.lift?.owner||null:null,
    stats:()=>({society:society.stats(),machinery:machinery.stats(),details:[...living].map(([id,m])=>({id,level:m.level})),loaded:[...loaded],bytes,freightEvents,residents:residents.filter(r=>r.node.visible).length,trams:trams.filter(t=>t.node.visible).length,rooms:[...rooms].filter(([,r])=>r.ready).map(([id])=>id),inside:room,ride:ride?.kind||null,station:ride?.station||null,discovered:[...visited],loading,interactions:near?{kind:near.kind,id:near.id}:null,
      sky:{ready:sky.ready,value:+sky.value.toFixed(2),moving:!!sky.motion},telescopes:scopes.length,
      scope:ride?.kind==='scope'?{id:ride.id,target:ride.target,fov:+camera.fov.toFixed(1)}:null}),
    dispose(){if(disposed)return;disposed=true;controller.abort();society.dispose();machinery.dispose();if(!options.traffic)traffic.dispose();memoryTexts=[];missionStates.clear();group.removeFromParent();mixers.forEach(m=>{m.stopAllAction();m.uncacheRoot(m.getRoot());});group.traverse(n=>{if(n.isInstancedMesh)n.dispose();});geometries.forEach(g=>g.dispose());materials.forEach(m=>m.dispose());textures.forEach(t=>t.dispose());cache.clear();},
  };
}
