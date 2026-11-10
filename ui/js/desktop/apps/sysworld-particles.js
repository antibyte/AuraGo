import * as THREE from 'three';
import {LOBBIES} from './sysworld-lobbies.js';
import {streetModules} from './sysworld-ground.js';
import {SURFACES} from './sysworld-furnishing.js';
// Steam and fireflies (phase 4). Steam: soft camera-facing quads per vent, one instanced draw (at most 60), animated
// in the shader and drifting with the wind. Fireflies: twinkling points over the lawns and gardens on clear nights
// (at most 200). Both off on low and with reduced motion; medium shows half.
export const STEAM_MAX=60,FIREFLY_MAX=200,LIMITS=Object.freeze({low:[0,0],medium:[30,100],high:[60,200],ultra:[60,200]});
// Vents: the foundry's three cooling towers (top 9.6 m), its three roof units (14.2 m, 4.5 m north of the origin), five street drains.
export function steamVents(){
  const infra=LOBBIES.find(l=>l.id==='infra'),out=[],drains=streetModules().filter(m=>m.id==='street-drain');
  for(const [x,z]of infra.towers)out.push({x,y:9.6,z,size:2.4,count:10});
  for(const [x]of infra.towers)out.push({x,y:14.2,z:infra.origin[1]-4.5,size:1.3,count:5});
  for(let k=0;k<5;k++){const m=drains[Math.floor((k+.5)*drains.length/5)];out.push({x:m.x,y:.05,z:m.z,size:.8,count:3});}
  return out;
}
export function fireflyAreas(){return [...SURFACES.filter(s=>s.kind==='lawn'),...[[-79,68],[-79,-67],[55,4],[8,40],[45,-23]].map(([x,z])=>({x0:x-3,z0:z-3,x1:x+3,z1:z+3}))];}
const STEAM_VERTEX=`attribute vec4 aVent;attribute vec2 aSeed;uniform float time;uniform vec4 wind;varying float vA;varying vec2 vUv;
void main(){vUv=uv;float life=fract(time*aSeed.y+aSeed.x);
vec3 c=aVent.xyz+vec3(wind.x,0.0,wind.y)*life*(1.0+3.0*wind.z)+vec3(sin(aSeed.x*41.0)*0.35,life*aVent.w*2.4,cos(aSeed.x*29.0)*0.35);
vec4 mv=viewMatrix*vec4(c,1.0);mv.xy+=position.xy*aVent.w*(0.55+1.4*life);vA=smoothstep(0.0,0.12,life)*(1.0-life);gl_Position=projectionMatrix*mv;}`;
const STEAM_FRAGMENT=`uniform vec3 color;uniform float strength;varying float vA;varying vec2 vUv;
void main(){float d=length(vUv-0.5)*2.0;float a=(1.0-smoothstep(0.2,1.0,d))*vA*strength;if(a<0.004)discard;gl_FragColor=vec4(color,a);}`;
const FLY_VERTEX=`attribute vec4 aSeed;uniform float time,pointScale;varying float vA;
void main(){vec3 p=position+vec3(sin(time*0.7+aSeed.x*20.0)*0.6,sin(time*1.3+aSeed.y*9.0)*0.3,cos(time*0.6+aSeed.z*13.0)*0.6);
vec4 mv=modelViewMatrix*vec4(p,1.0);vA=pow(max(0.0,sin(time*(1.5+aSeed.w)+aSeed.x*50.0)),6.0);gl_PointSize=clamp(2.6*pointScale/max(1.0,-mv.z),1.5,5.0);gl_Position=projectionMatrix*mv;}`;
const FLY_FRAGMENT=`uniform vec3 color;uniform float strength;varying float vA;void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;gl_FragColor=vec4(color*pow(1.0-d,1.5)*vA*strength,1.0);}`;
export function createParticles(scene){
  const group=new THREE.Group();group.name='city-particles';scene.add(group);
  let seed=7;const rnd=()=>(seed=(Math.imul(seed,1664525)+1013904223)>>>0)/4294967296;
  const timeU={value:0},windU={value:new THREE.Vector4()},pointScale={value:900};
  const plane=new THREE.PlaneGeometry(1,1),steamGeometry=new THREE.InstancedBufferGeometry();
  steamGeometry.index=plane.index;steamGeometry.setAttribute('position',plane.getAttribute('position'));steamGeometry.setAttribute('uv',plane.getAttribute('uv'));
  const vent=new Float32Array(STEAM_MAX*4),phase=new Float32Array(STEAM_MAX*2);let quads=0;
  for(const v of steamVents())for(let k=0;k<v.count&&quads<STEAM_MAX;k++,quads++){vent.set([v.x,v.y,v.z,v.size],quads*4);phase.set([k/v.count+rnd()*.1,.07+rnd()*.06],quads*2);}
  steamGeometry.setAttribute('aVent',new THREE.InstancedBufferAttribute(vent,4));steamGeometry.setAttribute('aSeed',new THREE.InstancedBufferAttribute(phase,2));steamGeometry.instanceCount=quads;
  const steamMaterial=new THREE.ShaderMaterial({uniforms:{time:timeU,wind:windU,color:{value:new THREE.Color(.6,.62,.64)},strength:{value:.32}},vertexShader:STEAM_VERTEX,fragmentShader:STEAM_FRAGMENT,transparent:true,depthWrite:false});
  const steam=new THREE.Mesh(steamGeometry,steamMaterial);steam.frustumCulled=false;steam.name='steam';group.add(steam);
  const areas=fireflyAreas(),weights=areas.map(a=>(a.x1-a.x0)*(a.z1-a.z0)),total=weights.reduce((s,w)=>s+w,0),pos=new Float32Array(FIREFLY_MAX*3),fly=new Float32Array(FIREFLY_MAX*4);
  for(let i=0;i<FIREFLY_MAX;i++){let r=rnd()*total,k=0;while(r>weights[k]&&k<areas.length-1)r-=weights[k++];const a=areas[k];
    pos.set([a.x0+rnd()*(a.x1-a.x0),.5+rnd()*1.7,a.z0+rnd()*(a.z1-a.z0)],i*3);fly.set([rnd(),rnd(),rnd(),rnd()],i*4);}
  const flyGeometry=new THREE.BufferGeometry();flyGeometry.setAttribute('position',new THREE.BufferAttribute(pos,3));flyGeometry.setAttribute('aSeed',new THREE.BufferAttribute(fly,4));
  const flyMaterial=new THREE.ShaderMaterial({uniforms:{time:timeU,pointScale,color:{value:new THREE.Color(.75,1,.45)},strength:{value:0}},vertexShader:FLY_VERTEX,fragmentShader:FLY_FRAGMENT,transparent:true,depthWrite:false,blending:THREE.AdditiveBlending});
  const flies=new THREE.Points(flyGeometry,flyMaterial);flies.frustumCulled=false;flies.name='fireflies';group.add(flies);
  let tier='high',day=1,weather='clear',time=0;
  return {
    setTier(value){tier=value;},
    setPointScale(value){pointScale.value=value;},
    setMood(d,w){day=d;weather=w;},
    update(dt,animated,wind){
      const [steamCount,flyCount]=LIMITS[tier]??LIMITS.high,night=1-day;
      if(animated)time+=Math.min(.1,Math.max(0,dt));timeU.value=time;windU.value.set(wind.x,wind.z,wind.strength,0);
      steam.visible=animated&&steamCount>0;steamGeometry.instanceCount=Math.min(quads,steamCount);steamMaterial.uniforms.color.value.setScalar(.12+.5*day);
      flies.visible=animated&&flyCount>0&&weather==='clear'&&night>.7;flyGeometry.setDrawRange(0,flyCount);flyMaterial.uniforms.strength.value=.85*Math.min(1,(night-.7)/.2);
    },
    stats:()=>({steam:steam.visible?steamGeometry.instanceCount:0,fireflies:flies.visible?flyGeometry.drawRange.count:0}),
    dispose(){group.removeFromParent();plane.dispose();steamGeometry.dispose();steamMaterial.dispose();flyGeometry.dispose();flyMaterial.dispose();},
  };
}
