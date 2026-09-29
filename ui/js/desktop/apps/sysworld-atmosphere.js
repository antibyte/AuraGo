import * as THREE from 'three';
import { createCinema } from './sysworld-cinema.js';

// Scene atmosphere: sky dome with dusk, clouds, Milky Way and meteors, stars, moon, animated sea
// with shore foam and city-light reflections, sea mist, drifting dust, the spire beacon, lamp
// light cones, sweeping searchlights and blinking aviation lights on the skyline. Every animation is driven by one shared time uniform, so
// reduced motion and hidden windows freeze the whole layer by not advancing it.
// Every pow() base is clamped: multisampled edges extrapolate UVs slightly past [0,1], and a
// negative base yields NaN on ANGLE/Direct3D, which bloom would smear over the entire frame.
const NOISE = `float hash2(vec2 p){return fract(sin(dot(p,vec2(127.1,311.7)))*43758.5453);}
float vnoise(vec2 p){vec2 i=floor(p),f=fract(p);vec2 u=f*f*(3.0-2.0*f);
return mix(mix(hash2(i),hash2(i+vec2(1.0,0.0)),u.x),mix(hash2(i+vec2(0.0,1.0)),hash2(i+vec2(1.0,1.0)),u.x),u.y);}`;
const WORLD_VERTEX = 'varying vec3 vWorld;varying vec2 vUv;void main(){vUv=uv;vec4 w=modelMatrix*vec4(position,1.0);vWorld=w.xyz;gl_Position=projectionMatrix*viewMatrix*w;}';
const FBM = `float fbm(vec2 p){float v=0.0,a=0.5;for(int i=0;i<5;i++){if(float(i)>=octaves)break;v+=a*vnoise(p);p=p*2.03+vec2(1.7,9.2);a*=0.5;}return v;}`;
const SKY = `uniform float time,day,dusk,cloud,flash,octaves;uniform vec3 zenith,horizon,haze,warm,sunDir,glow;varying vec3 vWorld;${NOISE}${FBM}
void main(){vec3 d=normalize(vWorld);float h=clamp(d.y,-0.08,1.0),hp=max(h,0.0),night=1.0-day;
vec3 col=mix(horizon,zenith,pow(smoothstep(0.0,0.62,hp),0.55));
vec3 s=normalize(sunDir);
vec3 level=normalize(vec3(d.x,0.0,d.z)+vec3(0.0,0.0,1e-4));
float az=max(0.0,dot(level,normalize(vec3(s.x,0.0,s.z)+vec3(0.0,0.0,1e-4))));
col+=warm*pow(az,4.0)*exp(-hp*7.0)*(0.55+dusk*1.5);
col+=vec3(0.42,0.13,0.3)*dusk*pow(az,1.2)*exp(-hp*2.4)*0.45;
col+=glow*exp(-hp*11.0)*night*(1.0-dusk*0.5);
float bend=atan(d.x,d.z);
float b1=(h-0.2+0.05*sin(bend*3.0+time*0.07))*13.0,b2=(h-0.33+0.04*sin(bend*2.5-time*0.05+1.7))*17.0;
float a1=exp(-b1*b1),a2=exp(-b2*b2);
float ripple=0.5+0.5*sin(bend*7.0+time*0.16+sin(bend*3.0)*1.3);
col+=(vec3(0.08,0.32,0.34)*a1*ripple*0.5+vec3(0.26,0.12,0.4)*a2*(1.0-ripple)*0.42)*night*(1.0-cloud*0.8);
vec3 bn=normalize(vec3(0.42,0.3,-0.86));float band=dot(d,bn);
float mw=exp(-band*band*22.0)*(0.35+0.65*fbm(vec2(bend*4.0,hp*9.0)+3.1))*smoothstep(0.04,0.3,hp);
col+=vec3(0.13,0.15,0.24)*mw*night*(1.0-cloud)*0.9;
float period=9.0,idx=floor(time/period),lt=time-idx*period;
float r1=hash2(vec2(idx,1.3)),r2=hash2(vec2(idx,7.1)),r3=hash2(vec2(idx,3.7));
if(night>0.5&&r3>0.35&&lt<1.1){vec2 q=d.xz/(hp+0.35);vec2 st=vec2(r1*2.4-1.2,-r2*1.4-0.2),dir=normalize(vec2(r3-0.65,0.55));
float prog=lt/1.1,len=prog*0.9;vec2 pa=q-st;float t=clamp(dot(pa,dir),0.0,len),dist=length(pa-dir*t);
col+=vec3(0.75,0.85,1.0)*exp(-dist*dist*40000.0)*smoothstep(len-0.4,len,t)*sin(prog*3.14159)*night*(1.0-cloud)*1.6*smoothstep(0.08,0.2,hp);}
float mu=max(0.0,dot(d,s));
float disc=smoothstep(0.99955,0.99982,mu);
float crater=1.0-0.22*night*(1.0-dusk)*vnoise(d.xy*180.0+d.z*97.0);
vec3 discCol=mix(vec3(0.92,0.95,1.06),vec3(1.9,0.85,0.4),dusk);
vec3 haloCol=mix(vec3(0.45,0.62,1.0),vec3(1.0,0.55,0.25),dusk);
float halo=pow(mu,220.0)*0.55+pow(mu,48.0)*0.12+pow(mu,6.0)*dusk*0.22;
vec2 cp=d.xz/(hp+0.1)*0.55+vec2(time*0.004,time*0.0025);
float n=fbm(cp)+0.35*vnoise(cp*5.0-vec2(time*0.01,0.0))-0.15;
float th=mix(0.66,0.28,cloud);
float dens=smoothstep(th,th+0.3,n)*smoothstep(0.0,0.1,hp)*(0.55+0.45*cloud);
float lit=pow(clamp(dot(d,s)*0.5+0.5,0.0,1.0),3.0);
vec3 cNight=vec3(0.035,0.045,0.07)+glow*0.55*exp(-hp*3.0);
vec3 cDay=mix(vec3(0.5,0.56,0.64),vec3(0.98,0.98,1.0),lit);
vec3 cDusk=mix(vec3(0.2,0.11,0.2),vec3(1.1,0.52,0.26),lit*az);
vec3 cc=mix(mix(cNight,cDay,day),cDusk,dusk*0.9)+haloCol*pow(mu,16.0)*(0.12+0.3*day+0.3*dusk)+vec3(0.55,0.6,0.85)*flash;
col+=discCol*disc*2.4*crater*(1.0-dens*0.9)+haloCol*halo*(1.0-dens*0.6);
col=mix(col,cc,dens);
col+=vec3(0.35,0.38,0.55)*flash*0.35;
col=mix(haze,col,smoothstep(0.0,0.14,h));
col=mix(col,haze*0.6,smoothstep(0.0,0.08,-d.y));
gl_FragColor=vec4(col,1.0);}`;
const STARS = {
  vertex: `attribute float phase,speed,size;uniform float time,pointScale,cloud;varying float vAlpha;varying vec3 vTint;
void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
float tw=0.65+0.35*sin(time*speed+phase);gl_PointSize=size*tw*pointScale;vAlpha=tw*smoothstep(0.02,0.16,position.y/1450.0)*(1.0-cloud*0.85);
float k=fract(phase*3.1);vTint=k>0.86?vec3(1.0,0.84,0.66):k>0.7?vec3(0.7,0.82,1.0):vec3(0.82,0.9,1.0);}`,
  fragment: `varying float vAlpha;varying vec3 vTint;void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;float a=pow(1.0-d,1.6)*vAlpha;gl_FragColor=vec4(vTint*a,1.0);}`,
};
const SEA = {
  vertex: `varying vec3 vWorld;
#include <fog_pars_vertex>
void main(){vec4 w=modelMatrix*vec4(position,1.0);vWorld=w.xyz;vec4 mvPosition=viewMatrix*w;gl_Position=projectionMatrix*mvPosition;
#include <fog_vertex>
}`,
  fragment: `uniform float time,day;uniform vec3 deep,shallow,sky,sunDir,sunColor,glow;uniform vec4 island;varying vec3 vWorld;
#include <fog_pars_fragment>
${NOISE}
float height(vec2 p){return 0.36*sin(p.x*0.09+time*0.9+sin(p.y*0.07)*1.5)+0.26*sin(p.y*0.11-time*0.7+p.x*0.03)
+0.12*sin((p.x+p.y)*0.21+time*1.4)+0.07*sin(p.x*0.52-p.y*0.37+time*2.2);}
void main(){vec2 p=vWorld.xz;float e=0.6;
vec3 n=normalize(vec3(height(p-vec2(e,0.0))-height(p+vec2(e,0.0)),2.0*e*0.9,height(p-vec2(0.0,e))-height(p+vec2(0.0,e))));
vec3 view=normalize(cameraPosition-vWorld);float fresnel=pow(clamp(1.0-dot(n,view),0.0,1.0),4.0);
vec3 col=mix(deep,shallow,clamp(0.5+n.x*2.5,0.0,1.0));col=mix(col,sky,0.22+0.62*fresnel);
vec3 refl=reflect(-view,n);float spec=pow(max(dot(refl,sunDir),0.0),260.0)*2.6+pow(max(dot(refl,sunDir),0.0),28.0)*0.22;
float sparkle=pow(max(0.0,sin(p.x*3.7+time*3.0)*sin(p.y*4.1-time*2.3)),40.0)*0.35*max(0.0,dot(refl,sunDir));
float path=pow(max(dot(refl,sunDir),0.0),60.0)*smoothstep(0.55,0.95,vnoise(p*1.7+vec2(time*0.8,-time*0.5)))*0.9;
col+=sunColor*(spec+sparkle+path);
vec2 q=max(abs(p-island.xy)-island.zw,0.0);float edge=length(q);
float wobble=(vnoise(p*0.35+vec2(time*0.4,time*0.23))-0.5)*2.6;
float foam=smoothstep(3.2,0.0,edge+wobble)*(0.55+0.45*sin(edge*1.7-time*2.1))*smoothstep(0.35,0.7,vnoise(p*1.3-time*0.3));
col=mix(col,vec3(0.62,0.74,0.8)*(0.28+0.72*day),clamp(foam,0.0,1.0)*0.7);
float night=1.0-day;
col+=glow*exp(-edge*0.045)*night*(0.3+0.7*smoothstep(0.3,0.85,vnoise(p*0.45+vec2(time*0.35,-time*0.2))*vnoise(p*0.17-vec2(time*0.12,0.0))*1.8))*(0.25+fresnel*0.9);
gl_FragColor=vec4(col,1.0);
#include <fog_fragment>
}`,
};
const MIST = `uniform float time,strength;uniform vec3 color;uniform vec4 island;varying vec3 vWorld;${NOISE}
void main(){vec2 p=vWorld.xz;float n=vnoise(p*0.012+vec2(time*0.017,-time*0.011))*0.6+vnoise(p*0.031-vec2(time*0.02,time*0.013))*0.4;
float mist=smoothstep(0.32,0.82,n);vec2 d2=max(abs(p-island.xy)-island.zw,0.0);float outside=smoothstep(0.0,70.0,length(d2));
float far=1.0-smoothstep(500.0,850.0,length(p));gl_FragColor=vec4(color,mist*outside*far*strength);}`;
const DUST = {
  vertex: `attribute vec3 seed;uniform float time,pointScale;varying float vAlpha;
void main(){vec3 p=position;p.x+=sin(time*0.11*seed.x+seed.y*6.28)*9.0+time*0.35*(seed.z-0.5);p.y+=sin(time*0.13+seed.z*6.28)*3.0;
p.z+=cos(time*0.09*seed.y+seed.x*6.28)*9.0;p.x=mod(p.x+90.0,180.0)-90.0;vec4 mv=modelViewMatrix*vec4(p,1.0);gl_Position=projectionMatrix*mv;
gl_PointSize=(0.06+0.11*seed.x)*pointScale/max(1.0,-mv.z);vAlpha=0.55+0.45*sin(time*(1.0+seed.y)+seed.z*6.28);}`,
  fragment: `uniform vec3 color;uniform float strength;varying float vAlpha;
void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;gl_FragColor=vec4(color*pow(1.0-d,2.2)*vAlpha*strength,1.0);}`,
};
const BEAM = `uniform float strength;uniform vec3 color;varying vec2 vUv;
void main(){float along=pow(clamp(1.0-vUv.x,0.0,1.0),2.4);float across=pow(clamp(1.0-abs(vUv.y-0.5)*2.0,0.0,1.0),1.7);gl_FragColor=vec4(color*along*across*strength,1.0);}`;
const LAMP = `uniform vec3 color;uniform float strength;varying vec2 vUv;varying vec3 vNormal,vView;
void main(){float rim=clamp(1.0-abs(dot(normalize(vNormal),normalize(vView))),0.0,1.0);float a=pow(clamp(vUv.y,0.0,1.0),1.6)*(0.35+0.65*rim)*strength;gl_FragColor=vec4(color*a,1.0);}`;
// Distant coastline: a fixed aerial-perspective tint of the haze, darker towards the ridge.
const COAST = {
  vertex: 'attribute float lift;varying float vLift;void main(){vLift=lift;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}',
  fragment: 'uniform vec3 haze;uniform float shade;varying float vLift;void main(){gl_FragColor=vec4(haze*mix(1.0,shade,smoothstep(0.0,0.6,vLift)),1.0);}',
};
const SHORE_LIGHTS = {
  vertex: `attribute float phase;uniform float time,pointScale,strength;varying float vA;void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
vA=strength*(0.75+0.25*sin(time*(0.7+fract(phase)*1.3)+phase));gl_PointSize=clamp(2.2*pointScale/max(1.0,-mv.z),1.5,4.0);}`,
  fragment: 'varying float vA;void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;gl_FragColor=vec4(vec3(1.4,0.95,0.55)*pow(1.0-d,1.5)*vA,1.0);}',
};
const SEARCH = {
  vertex: 'varying vec2 vUv;varying float vRim,vNear;void main(){vUv=uv;vec3 n=normalize(normalMatrix*normal);vec4 mv=modelViewMatrix*vec4(position,1.0);vRim=abs(dot(n,normalize(-mv.xyz)));vNear=smoothstep(40.0,160.0,-mv.z);gl_Position=projectionMatrix*mv;}',
  fragment: 'uniform vec3 color;uniform float strength;varying vec2 vUv;varying float vRim,vNear;void main(){float along=pow(clamp(1.0-vUv.y,0.0,1.0),1.6)*smoothstep(0.0,0.03,vUv.y);gl_FragColor=vec4(color*along*pow(clamp(vRim,0.0,1.0),2.5)*vNear*strength,1.0);}',
};
const AVIATION = {
  vertex: `attribute float phase;uniform float time,pointScale;varying float vA;void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
vA=0.22+0.78*pow(max(0.0,sin(time*1.9+phase)),10.0);gl_PointSize=clamp(1.5*pointScale/max(1.0,-mv.z),2.0,14.0);}`,
  fragment: 'varying float vA;void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;gl_FragColor=vec4(vec3(3.2,0.35,0.25)*pow(1.0-d,2.0)*vA,1.0);}',
};
export function createAtmosphere(scene, options = {}) {
  const geometries = [], materials = [], group = new THREE.Group(); group.name = 'city-atmosphere'; scene.add(group);
  const own = (g, m) => { geometries.push(g); materials.push(m); return new THREE.Mesh(g, m); };
  const time = { value: 0 }, pointScale = { value: 900 };
  const day = { value: 0 }, dusk = { value: 0 }, cloud = { value: .25 }, flash = { value: 0 }, octaves = { value: 5 };
  const glow = new THREE.Color(0x4a2a18);
  const sunDir = (options.sunDirection || new THREE.Vector3(-70, 145, 85)).clone().normalize();
  const zenith = new THREE.Color(0x05091a), horizon = new THREE.Color(0x14324c), warm = new THREE.Color(0x7a3d2a);
  const haze = new THREE.Color(scene.fog?.color || 0x08121e);
  const shader = (vertex, fragment, uniforms, extra = {}) => new THREE.ShaderMaterial({ uniforms, vertexShader: vertex, fragmentShader: fragment, ...extra });
  const additive = { transparent: true, depthWrite: false, blending: THREE.AdditiveBlending };
  // Sky dome and stars are unlit backdrops outside the fog.
  const sky = own(new THREE.SphereGeometry(1500, 48, 24), shader(WORLD_VERTEX, SKY, { time, day, dusk, cloud, flash, octaves, glow: { value: glow }, zenith: { value: zenith }, horizon: { value: horizon }, haze: { value: haze }, warm: { value: warm }, sunDir: { value: sunDir } }, { side: THREE.BackSide, depthWrite: false, fog: false }));
  sky.frustumCulled = false; sky.renderOrder = -10; group.add(sky);
  const starCount = 1400, starPositions = new Float32Array(starCount * 3), phase = new Float32Array(starCount), speed = new Float32Array(starCount), size = new Float32Array(starCount);
  let seed = 12345; const random = () => (seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0) / 4294967296;
  for (let i = 0; i < starCount; i++) {
    const u = random() * Math.PI * 2, v = .03 + random() * .97, r = Math.sqrt(1 - v * v);
    starPositions.set([Math.cos(u) * r * 1450, v * 1450, Math.sin(u) * r * 1450], i * 3);
    phase[i] = random() * Math.PI * 2; speed[i] = .4 + random() * 1.6; size[i] = .0008 + random() * random() * .0022;
  }
  const starGeometry = new THREE.BufferGeometry();
  starGeometry.setAttribute('position', new THREE.BufferAttribute(starPositions, 3));
  starGeometry.setAttribute('phase', new THREE.BufferAttribute(phase, 1));
  starGeometry.setAttribute('speed', new THREE.BufferAttribute(speed, 1));
  starGeometry.setAttribute('size', new THREE.BufferAttribute(size, 1));
  const starMaterial = shader(STARS.vertex, STARS.fragment, { time, pointScale, cloud }, { ...additive, fog: false });
  const stars = new THREE.Points(starGeometry, starMaterial); stars.frustumCulled = false; stars.renderOrder = -9; group.add(stars);
  geometries.push(starGeometry); materials.push(starMaterial);
  // Animated water replaces the flat standard sea; fog still fades it to the horizon.
  const sea = own(new THREE.PlaneGeometry(1800, 1800), shader(SEA.vertex, SEA.fragment, {
    time, day, glow: { value: glow }, island: { value: new THREE.Vector4(0, -7, 85, 87) }, deep: { value: new THREE.Color(0x061420) }, shallow: { value: new THREE.Color(0x0e3350) }, sky: { value: horizon.clone().multiplyScalar(.7) },
    sunDir: { value: sunDir }, sunColor: { value: new THREE.Color(0xdff0ff) }, fogColor: { value: new THREE.Color() }, fogDensity: { value: 0 }, fogNear: { value: 1 }, fogFar: { value: 1000 },
  }, { fog: true }));
  sea.rotation.x = -Math.PI / 2; sea.position.y = -3.2; group.add(sea);
  const mist = own(new THREE.PlaneGeometry(1900, 1900), shader(WORLD_VERTEX, MIST, { time, strength: { value: .55 }, color: { value: new THREE.Color(0x1a3448) }, island: { value: new THREE.Vector4(0, -7, 85, 87) } }, { transparent: true, depthWrite: false, fog: false }));
  mist.rotation.x = -Math.PI / 2; mist.position.y = -2.4; group.add(mist);
  // Slow data dust over the city, positions animated on the GPU.
  const dustCount = 500, dustPositions = new Float32Array(dustCount * 3), dustSeeds = new Float32Array(dustCount * 3);
  for (let i = 0; i < dustCount; i++) {
    dustPositions.set([(random() - .5) * 180, 2 + random() * random() * 75, -95 + random() * 170], i * 3);
    dustSeeds.set([random(), random(), random()], i * 3);
  }
  const dustGeometry = new THREE.BufferGeometry();
  dustGeometry.setAttribute('position', new THREE.BufferAttribute(dustPositions, 3));
  dustGeometry.setAttribute('seed', new THREE.BufferAttribute(dustSeeds, 3));
  const dustMaterial = shader(DUST.vertex, DUST.fragment, { time, pointScale, color: { value: new THREE.Color(0x9fd8ff) }, strength: { value: .32 } }, additive);
  const dust = new THREE.Points(dustGeometry, dustMaterial); dust.frustumCulled = false; group.add(dust);
  geometries.push(dustGeometry); materials.push(dustMaterial);
  // Rotating spire beacon: two crossed additive blades plus a pulsing crown light.
  const beacon = new THREE.Group(); beacon.position.set(0, 86.5, -12); group.add(beacon);
  const beamMaterial = shader(WORLD_VERTEX, BEAM, { strength: { value: .45 }, color: { value: new THREE.Color(0x8fe6ff) } }, { ...additive, side: THREE.DoubleSide });
  const beamGeometry = new THREE.PlaneGeometry(100, 5.2); beamGeometry.translate(50, 0, 0); geometries.push(beamGeometry); materials.push(beamMaterial);
  const bladeA = new THREE.Mesh(beamGeometry, beamMaterial), bladeB = new THREE.Mesh(beamGeometry, beamMaterial);
  bladeB.rotation.x = Math.PI / 2; const blades = new THREE.Group(); blades.add(bladeA, bladeB); blades.rotation.z = -.07; beacon.add(blades);
  const crownMaterial = new THREE.MeshBasicMaterial({ color: 0xbff2ff, toneMapped: false });
  const crown = own(new THREE.SphereGeometry(.9, 16, 12), crownMaterial); beacon.add(crown);
  // Warm lamp cones from the kit's cantilever lamps.
  const lamps = (options.lamps || []).slice(0, 64);
  const lampMaterial = shader('varying vec2 vUv;varying vec3 vNormal,vView;void main(){vUv=uv;vNormal=normalize(normalMatrix*normal);vec4 mv=modelViewMatrix*vec4(position,1.0);vView=-mv.xyz;gl_Position=projectionMatrix*mv;}',
    LAMP, { color: { value: new THREE.Color(0xffd9a0) }, strength: { value: .3 } }, { ...additive, side: THREE.DoubleSide });
  const lampGeometry = new THREE.ConeGeometry(2.7, 6.1, 18, 1, true);
  const lampCones = new THREE.InstancedMesh(lampGeometry, lampMaterial, Math.max(1, lamps.length));
  geometries.push(lampGeometry); materials.push(lampMaterial);
  const scratch = new THREE.Object3D();
  lamps.forEach((lamp, i) => {
    scratch.position.set(lamp.x + 1.9 * Math.cos(lamp.angle || 0), 3.35, lamp.z - 1.9 * Math.sin(lamp.angle || 0)); scratch.updateMatrix(); lampCones.setMatrixAt(i, scratch.matrix);
  });
  lampCones.count = lamps.length; lampCones.instanceMatrix.needsUpdate = true; group.add(lampCones);
  // A faint coastline behind and beside the city gives the horizon depth; settlement lights
  // twinkle along it at night. The camera side stays open sea.
  const coastPositions = [], coastLift = [], coastIndex = [], shorePositions = [], shorePhase = [], segments = 360;
  const ridge = a => Math.max(0, .45 + .3 * Math.sin(a * 5 + 1.3) + .2 * Math.sin(a * 13 + .4) + .12 * Math.sin(a * 31 + 2.1)) * (.35 + .65 * Math.max(0, Math.sin(a * 2 + .6)));
  for (let i = 0; i <= segments; i++) {
    const a = Math.PI * .8 + Math.PI * 1.45 * i / segments, r = 1180 + 40 * Math.sin(a * 7), h = 6 + 74 * ridge(a);
    coastPositions.push(Math.cos(a) * r, -3.5, Math.sin(a) * r, Math.cos(a) * r, h, Math.sin(a) * r); coastLift.push(0, h / 80);
    if (i < segments) { const n = i * 2; coastIndex.push(n, n + 1, n + 2, n + 1, n + 3, n + 2); }
  }
  for (let i = 0; i < 90; i++) {
    const a = Math.PI * .82 + Math.PI * 1.41 * random(), r = 1170 + 40 * Math.sin(a * 7);
    if (ridge(a) < .08) continue;
    shorePositions.push(Math.cos(a) * r, 1 + random() * random() * 28, Math.sin(a) * r); shorePhase.push(random() * 20);
  }
  const coastGeometry = new THREE.BufferGeometry();
  coastGeometry.setAttribute('position', new THREE.Float32BufferAttribute(coastPositions, 3));
  coastGeometry.setAttribute('lift', new THREE.Float32BufferAttribute(coastLift, 1)); coastGeometry.setIndex(coastIndex);
  const coastShade = { value: .8 };
  const coast = own(coastGeometry, shader(COAST.vertex, COAST.fragment, { haze: { value: haze }, shade: coastShade }, { fog: false, side: THREE.DoubleSide }));
  coast.frustumCulled = false; coast.renderOrder = -8; group.add(coast);
  const shoreGeometry = new THREE.BufferGeometry();
  shoreGeometry.setAttribute('position', new THREE.Float32BufferAttribute(shorePositions, 3));
  shoreGeometry.setAttribute('phase', new THREE.Float32BufferAttribute(shorePhase, 1));
  const shoreStrength = { value: 0 };
  const shoreMaterial = shader(SHORE_LIGHTS.vertex, SHORE_LIGHTS.fragment, { time, pointScale, strength: shoreStrength }, { ...additive, fog: false });
  const shore = new THREE.Points(shoreGeometry, shoreMaterial); shore.frustumCulled = false; shore.renderOrder = -7; group.add(shore);
  geometries.push(shoreGeometry); materials.push(shoreMaterial);
  // Searchlights sweep the night sky from the skyline behind the city.
  const searchMaterial = shader(SEARCH.vertex, SEARCH.fragment, { color: { value: new THREE.Color(0xa9ccff) }, strength: { value: 0 } }, { ...additive, side: THREE.DoubleSide, fog: false });
  const searchGeometry = new THREE.CylinderGeometry(11, .7, 280, 20, 1, true); searchGeometry.translate(0, 140, 0);
  geometries.push(searchGeometry); materials.push(searchMaterial);
  const searchlights = [[-72, -168, 0], [64, -186, 2.1], [4, -232, 4.2]].map(([x, z, offset]) => {
    const beam = new THREE.Mesh(searchGeometry, searchMaterial); beam.position.set(x, -3, z); beam.rotation.order = 'YXZ';
    beam.userData.offset = offset; beam.frustumCulled = false; group.add(beam); return beam;
  });
  // Blinking red aviation lights on skyline tower roofs; positions arrive after the kit loads.
  const aviationMax = 96, aviationPositions = new Float32Array(aviationMax * 3), aviationPhase = new Float32Array(aviationMax);
  for (let i = 0; i < aviationMax; i++) aviationPhase[i] = random() * Math.PI * 2;
  const aviationGeometry = new THREE.BufferGeometry();
  aviationGeometry.setAttribute('position', new THREE.BufferAttribute(aviationPositions, 3));
  aviationGeometry.setAttribute('phase', new THREE.BufferAttribute(aviationPhase, 1));
  aviationGeometry.setDrawRange(0, 0);
  const aviationMaterial = shader(AVIATION.vertex, AVIATION.fragment, { time, pointScale }, { ...additive, fog: false });
  const aviation = new THREE.Points(aviationGeometry, aviationMaterial); aviation.frustumCulled = false; group.add(aviation);
  geometries.push(aviationGeometry); materials.push(aviationMaterial);
  // Cinematic grade after the output pass (high/ultra only).
  const cinema = createCinema(time), post = cinema.pass;
  let busy = false, tier = 'high', sweep = 0;
  return {
    post, sea,
    setLighting(daylight, evening, direction) {
      sunDir.copy(direction).normalize();
      day.value = daylight; dusk.value = Math.min(1, evening) * (1 - daylight);
      zenith.set(0x05091a).lerp(new THREE.Color(0x2f6fa8),daylight).lerp(new THREE.Color(0x1d2146),dusk.value*.8);
      horizon.set(0x14324c).lerp(new THREE.Color(0xa9c6d4),daylight).lerp(new THREE.Color(0xc0683e),dusk.value*.85);
      warm.set(evening>.1?0xff8a45:0x7a3d2a);
      glow.set(0x4a2a18).lerp(new THREE.Color(0x6b3a22),dusk.value);
      haze.copy(scene.fog.color);stars.visible=daylight<.35;
      sea.material.uniforms.sky.value.copy(horizon).lerp(zenith,dusk.value*.65).multiplyScalar(.7);
      sea.material.uniforms.deep.value.set(0x061420).lerp(new THREE.Color(0x0a3656),daylight);
      sea.material.uniforms.shallow.value.set(0x0e3350).lerp(new THREE.Color(0x1d6f98),daylight);
      mist.material.uniforms.strength.value=.55*(1-daylight*.85);
      coastShade.value=.82-.22*(1-daylight)-.2*dusk.value;shoreStrength.value=Math.max(0,1-daylight*1.6)*(1-dusk.value*.5);
      sea.material.uniforms.sunColor.value.set(0xdff0ff).lerp(new THREE.Color(0xffa060),dusk.value);
    },
    setWeather(value) { cloud.value = value === 'rain' ? .95 : value === 'fog' ? .7 : .25; },
    setFlash(value) { flash.value = Math.max(0, Math.min(1, value)); },
    setCinematic(value) { cinema.setCinematic(value); },
    // Roof-top points for the aviation lights; bounded to the preallocated buffer.
    setAviation(points) {
      const count = Math.min(aviationMax, points.length);
      for (let i = 0; i < count; i++) aviationPositions.set([points[i].x, points[i].y, points[i].z], i * 3);
      aviationGeometry.attributes.position.needsUpdate = true; aviationGeometry.setDrawRange(0, count);
    },
    setTier(value) {
      tier = value;
      const rich = tier !== 'low';
      mist.visible = rich; dust.visible = rich; lampCones.visible = rich;
      searchlights.forEach(beam => { beam.visible = rich; });
      octaves.value = tier === 'low' ? 3 : tier === 'medium' ? 4 : 5;
      post.enabled = tier === 'high' || tier === 'ultra';
    },
    setBusy(value) { busy = !!value; },
    setPointScale(value) { pointScale.value = value; },
    update(dt, elapsed, camera, animated) {
      const step = animated ? Math.min(.1, Math.max(0, dt)) : 0;
      time.value += step;
      if (scene.fog) {
        sea.material.uniforms.fogColor.value.copy(scene.fog.color);
        if (scene.fog.isFogExp2) sea.material.uniforms.fogDensity.value = scene.fog.density;
      }
      sea.position.x = camera.position.x; sea.position.z = camera.position.z;
      const target = busy ? 1.15 : .32, rate = beacon.userData.rate ?? target;
      beacon.userData.rate = rate + (target - rate) * Math.min(1, step * 1.5);
      blades.rotation.y += step * beacon.userData.rate;
      beamMaterial.uniforms.strength.value = busy ? .85 : .45;
      crown.scale.setScalar(1 + Math.sin(time.value * (busy ? 6 : 2.2)) * .18);
      sweep += step;
      const night = 1 - day.value;
      searchMaterial.uniforms.strength.value = night * night * .075 * (1 - dusk.value * .7) * (1 - cloud.value * .45);
      searchlights.forEach((beam, i) => {
        const o = beam.userData.offset;
        // Lean away from the city (-z) and search side to side, never across the island.
        beam.rotation.y = Math.PI + Math.sin(sweep * (.11 + i * .025) + o) * .95;
        beam.rotation.x = .22 + .14 * Math.sin(sweep * .13 + o * 1.7);
      });
      if (post.enabled) cinema.update(step, camera, sunDir, { day: day.value, dusk: dusk.value, cloud: cloud.value, animated });
    },
    stats: () => ({ tier, busy, stars: starCount, dust: dustCount, lamps: lamps.length, post: post.enabled, coast: coastIndex.length / 6, shoreLights: shorePhase.length,
      clouds: +cloud.value.toFixed(2), searchlights: searchlights.length, aviation: aviationGeometry.drawRange.count, cinema: cinema.stats() }),
    dispose() {
      group.removeFromParent(); lampCones.dispose();
      geometries.forEach(g => g.dispose()); materials.forEach(m => m.dispose()); cinema.dispose();
    },
  };
}

