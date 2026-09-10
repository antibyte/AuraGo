import * as THREE from 'three';
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';

// Five decorative city residents. Their routes are streets, not claimed data flows.
const routes = [
  [[-67,59],[-18,59],[-18,13],[-67,13]],
  [[-18,13],[18,13],[18,59],[-18,59]],
  [[18,59],[67,59],[67,13],[18,13]],
  [[67,13],[67,-32],[18,-32],[18,13]],
  [[-67,-32],[-18,-32],[-18,-77],[-67,-77]],
];
function streetPath(corners) {
  const path = new THREE.CurvePath(), points = corners.map(([x,z]) => new THREE.Vector3(x,0,z));
  const arrivals = [], departures = [];
  for (let i=0;i<points.length;i++) {
    const p=points[i], before=points[(i+3)%4], after=points[(i+1)%4];
    arrivals.push(p.clone().addScaledVector(before.clone().sub(p).normalize(),3));
    departures.push(p.clone().addScaledVector(after.clone().sub(p).normalize(),3));
  }
  for(let i=0;i<4;i++) {
    path.add(new THREE.QuadraticBezierCurve3(arrivals[i],points[i],departures[i]));
    path.add(new THREE.LineCurve3(departures[i],arrivals[(i+1)%4]));
  }
  return { points:path.getSpacedPoints(512), length:path.getLength() };
}
export function createCityLife(scene, districts, options) {
  let disposed=false, time=0, robotsLoaded=false, robotError=false, latestEvent=0;
  const group=new THREE.Group();group.name='city-life';scene.add(group);
  const geometry=new Set(), materials=new Set(), textures=new Set(), signalMaterials=[];
  const abort=new AbortController(), residents=[], signals=[];
  const ownGeometry=g=>(geometry.add(g),g), ownMaterial=m=>(materials.add(m),m);
  const ringGeometry=ownGeometry(new THREE.TorusGeometry(1,.013,5,64));
  for(const d of districts) {
    const material=ownMaterial(new THREE.MeshBasicMaterial({color:0x71b8ce,transparent:true,opacity:.08,depthWrite:false,toneMapped:false}));
    const ring=new THREE.Mesh(ringGeometry,material);ring.rotation.x=Math.PI/2;
    ring.position.set(d.x,.6,d.z);ring.scale.setScalar(d.radius*1.12);group.add(ring);
    signals.push({id:d.id,ring,material,state:'unknown',eventUntil:0,color:new THREE.Color(0x71818d)});
  }
  // Soft ground light replaces five moving shadow maps/point lights.
  const glowGeometry=ownGeometry(new THREE.PlaneGeometry(7,7));
  const glowMaterial=ownMaterial(new THREE.ShaderMaterial({
    transparent:true,depthWrite:false,blending:THREE.AdditiveBlending,
    uniforms:{color:{value:new THREE.Color(0x53d9f4)}},
    vertexShader:'varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}',
    fragmentShader:'varying vec2 vUv;uniform vec3 color;void main(){float r=length(vUv-.5)*2.;float a=exp(-r*r*5.)*.23*(1.-smoothstep(.65,1.,r));gl_FragColor=vec4(color,a);}',
  }));
  const jetGeometry=ownGeometry(new THREE.ConeGeometry(.42,1.3,12,1,true));
  const jetMaterial=ownMaterial(new THREE.MeshBasicMaterial({color:0x92eeff,transparent:true,opacity:.22,depthWrite:false,toneMapped:false,blending:THREE.AdditiveBlending}));
  routes.forEach((corners,i)=>{
    const root=new THREE.Group();root.name='city-white-robot-'+(i+1);
    const body=new THREE.Group();root.add(body);group.add(root);
    const glow=new THREE.Mesh(glowGeometry,glowMaterial);glow.rotation.x=-Math.PI/2;glow.position.y=.58;group.add(glow);
    const jet=new THREE.Mesh(jetGeometry,jetMaterial);jet.rotation.z=Math.PI;jet.position.y=-.45;root.add(jet);
    root.visible=false;glow.visible=false;
    residents.push({root,body,glow,jet,path:streetPath(corners),phase:i*.173,speed:5.1+i*.43});
  });
  function releaseModel(model) {
    model.traverse(n=>{
      if(!n.isMesh)return;geometry.add(n.geometry);
      for(const m of Array.isArray(n.material)?n.material:[n.material]) {
        materials.add(m);for(const value of Object.values(m))if(value?.isTexture)textures.add(value);
      }
    });
  }
  function freeAssets() {
    geometry.forEach(g=>g.dispose());materials.forEach(m=>m.dispose());
    const images=new Set();textures.forEach(t=>{if(t.image)images.add(t.image);t.dispose();});
    images.forEach(i=>i.close?.());geometry.clear();materials.clear();textures.clear();
  }
  async function loadRobot() {
    const timeout=setTimeout(()=>abort.abort(),15000);
    try {
      const response=await fetch(options.robotURL,{signal:abort.signal});if(!response.ok)throw Error('Robot unavailable');
      const bytes=await response.arrayBuffer();if(disposed)return;
      const {scene:model}=await new GLTFLoader().parseAsync(bytes,'');
      releaseModel(model);
      if(disposed){freeAssets();return;}
      const box=new THREE.Box3().setFromObject(model), size=box.getSize(new THREE.Vector3());
      const extent=Math.max(size.x,size.y,size.z);if(!Number.isFinite(extent)||extent<=0)throw Error('Invalid robot bounds');
      model.scale.setScalar(6/extent);model.updateMatrixWorld(true);box.setFromObject(model);
      const center=box.getCenter(new THREE.Vector3());model.position.set(-center.x,-box.min.y,-center.z);
      // Same heading correction as the white ThreeDee robot.
      model.rotation.y=Math.PI;
      model.traverse(n=>{if(n.isMesh){n.castShadow=false;n.receiveShadow=true;}});
      residents.forEach(r=>{r.body.add(model.clone(true));r.root.visible=true;r.glow.visible=true;});
      robotsLoaded=true;update(0,false);
    } catch(error) {if(!disposed){robotError=true;options.onError?.(error);}}
    finally {clearTimeout(timeout);}
  }
  function attachLandmarks(landmarks) {
    signalMaterials.forEach(m=>{m.dispose();materials.delete(m);});signalMaterials.length=0;
    const ops=landmarks.children.find(n=>n.userData.district==='operations');
    ops?.getObjectByName('signal')?.traverse(n=>{
      if(!n.isMesh)return;
      const clone=m=>{const copy=ownMaterial(m.clone());signalMaterials.push(copy);return copy;};
      n.material=Array.isArray(n.material)?n.material.map(clone):clone(n.material);
    });
  }
  function setData(entities, events=[]) {
    for(const s of signals) {
      const e=entities.find(e=>e.id===s.id);
      s.state=!e||e.stale?'unknown':e.state;
      s.color.setHex(s.state==='error'?0xff493e:s.state==='running'?0x70ebd3:s.state==='unknown'?0x71818d:0x71b8ce);
      s.material.color.copy(s.color);
    }
    for(const e of events)if(e.id>latestEvent) {
      if(Date.now()-e.at<6000){const s=signals.find(s=>s.id===e.district);if(s)s.eventUntil=e.at+5000;}
    }
    latestEvent=Math.max(latestEvent,...events.map(e=>e.id));
  }
  function update(dt, animated) {
    if(disposed)return;if(animated)time+=Math.min(.1,Math.max(0,dt));
    for(const [i,r]of residents.entries()) {
      const samples=r.path.points, cursor=((time*r.speed/r.path.length+r.phase)%1)*512;
      const index=Math.floor(cursor), blend=cursor-index, a=samples[index], b=samples[index+1];
      r.root.position.lerpVectors(a,b,blend);
      r.root.position.y=1.6+Math.sin(time*1.6+i*1.9)*.18;
      const heading=Math.atan2(b.x-a.x,b.z-a.z);
      const turn=Math.atan2(Math.sin(heading-r.root.rotation.y),Math.cos(heading-r.root.rotation.y));
      r.root.rotation.y+=dt===0?turn:animated?turn*Math.min(1,dt*9):0;
      r.body.rotation.z=Math.sin(time*.85+i)*.035;
      r.body.rotation.x=-.035+Math.sin(time*1.1+i)*.018;
      r.glow.position.x=r.root.position.x;r.glow.position.z=r.root.position.z;
      r.jet.scale.y=1+Math.sin(time*4+i)*.12;
    }
    for(const s of signals) {
      const pulse=animated ? .5+.5*Math.sin(time*(s.state==='error'?3.2:1.8)) : .5;
      const active=s.state==='error'||s.state==='running'||s.eventUntil>Date.now();
      s.material.opacity=s.state==='unknown'?.03:active?.2+pulse*.46:.065;
      s.ring.scale.setScalar(districts.find(d=>d.id===s.id).radius*1.12*(1+(active&&animated?pulse*.035:0)));
      if(s.id==='operations')for(const m of signalMaterials) {
        m.color.copy(s.color);m.emissive?.copy(s.color);
        m.emissiveIntensity=s.state==='error'?1.1+pulse*2.2:s.state==='unknown'?.08:.55;
      }
    }
  }
  function dispose() {
    if(disposed)return;disposed=true;abort.abort();options.signal?.removeEventListener('abort',dispose);
    group.removeFromParent();freeAssets();residents.length=0;signalMaterials.length=0;
  }
  options.signal?.addEventListener('abort',dispose,{once:true});
  if(options.signal?.aborted)dispose();else void loadRobot();
  return {update,setData,attachLandmarks,dispose,
    stats:()=>({robots:robotsLoaded?residents.length:0,robotError,time,
      positions:residents.map(r=>r.root.position.toArray()),
      signals:signals.map(s=>({id:s.id,state:s.state,intensity:s.material.opacity})),
    })};
}
