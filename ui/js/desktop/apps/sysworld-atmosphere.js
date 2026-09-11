import * as THREE from 'three';
import { ShaderPass } from 'three/addons/postprocessing/ShaderPass.js';

// Scene atmosphere: sky dome, stars, moon, animated sea, sea mist, drifting dust, the spire
// beacon and lamp light cones. Every animation is driven by one shared time uniform, so
// reduced motion and hidden windows freeze the whole layer by not advancing it.
// Every pow() base is clamped: multisampled edges extrapolate UVs slightly past [0,1], and a
// negative base yields NaN on ANGLE/Direct3D, which bloom would smear over the entire frame.
const NOISE = `float hash2(vec2 p){return fract(sin(dot(p,vec2(127.1,311.7)))*43758.5453);}
float vnoise(vec2 p){vec2 i=floor(p),f=fract(p);vec2 u=f*f*(3.0-2.0*f);
return mix(mix(hash2(i),hash2(i+vec2(1.0,0.0)),u.x),mix(hash2(i+vec2(0.0,1.0)),hash2(i+vec2(1.0,1.0)),u.x),u.y);}`;
const WORLD_VERTEX = 'varying vec3 vWorld;varying vec2 vUv;void main(){vUv=uv;vec4 w=modelMatrix*vec4(position,1.0);vWorld=w.xyz;gl_Position=projectionMatrix*viewMatrix*w;}';
const SKY = `uniform float time;uniform vec3 zenith,horizon,haze,warm,sunDir;varying vec3 vWorld;
void main(){vec3 d=normalize(vWorld);float h=clamp(d.y,-0.08,1.0);
vec3 col=mix(horizon,zenith,pow(smoothstep(0.0,0.62,h),0.55));
vec3 level=normalize(vec3(d.x,0.0,d.z)+vec3(0.0,0.0,1e-4));
float az=max(0.0,dot(level,normalize(vec3(sunDir.x,0.0,sunDir.z))));
col+=warm*pow(az,5.0)*exp(-max(h,0.0)*8.0)*0.85;
float bend=atan(d.x,d.z);
float b1=(h-0.2+0.05*sin(bend*3.0+time*0.07))*13.0,b2=(h-0.33+0.04*sin(bend*2.5-time*0.05+1.7))*17.0;
float a1=exp(-b1*b1),a2=exp(-b2*b2);
float ripple=0.5+0.5*sin(bend*7.0+time*0.16+sin(bend*3.0)*1.3);
col+=vec3(0.08,0.32,0.34)*a1*ripple*0.5+vec3(0.26,0.12,0.4)*a2*(1.0-ripple)*0.42;
col=mix(haze,col,smoothstep(0.0,0.14,h));
col=mix(col,haze*0.6,smoothstep(0.0,0.08,-d.y));
vec3 s=normalize(sunDir);float mu=max(0.0,dot(d,s));
float disc=smoothstep(0.99955,0.99982,mu);
float halo=pow(mu,220.0)*0.55+pow(mu,48.0)*0.12;
col+=vec3(0.92,0.95,1.06)*disc*2.4+vec3(0.45,0.62,1.0)*halo;
gl_FragColor=vec4(col,1.0);}`;
const STARS = {
  vertex: `attribute float phase,speed,size;uniform float time,pointScale;varying float vAlpha;
void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
float tw=0.65+0.35*sin(time*speed+phase);gl_PointSize=size*tw*pointScale;vAlpha=tw*smoothstep(0.02,0.16,position.y/1450.0);}`,
  fragment: `varying float vAlpha;void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;float a=pow(1.0-d,1.6)*vAlpha;gl_FragColor=vec4(vec3(0.82,0.9,1.0)*a,1.0);}`,
};
const SEA = {
  vertex: `varying vec3 vWorld;
#include <fog_pars_vertex>
void main(){vec4 w=modelMatrix*vec4(position,1.0);vWorld=w.xyz;vec4 mvPosition=viewMatrix*w;gl_Position=projectionMatrix*mvPosition;
#include <fog_vertex>
}`,
  fragment: `uniform float time;uniform vec3 deep,shallow,sky,sunDir,sunColor;varying vec3 vWorld;
#include <fog_pars_fragment>
float height(vec2 p){return 0.36*sin(p.x*0.09+time*0.9+sin(p.y*0.07)*1.5)+0.26*sin(p.y*0.11-time*0.7+p.x*0.03)
+0.12*sin((p.x+p.y)*0.21+time*1.4)+0.07*sin(p.x*0.52-p.y*0.37+time*2.2);}
void main(){vec2 p=vWorld.xz;float e=0.6;
vec3 n=normalize(vec3(height(p-vec2(e,0.0))-height(p+vec2(e,0.0)),2.0*e*0.9,height(p-vec2(0.0,e))-height(p+vec2(0.0,e))));
vec3 view=normalize(cameraPosition-vWorld);float fresnel=pow(clamp(1.0-dot(n,view),0.0,1.0),4.0);
vec3 col=mix(deep,shallow,clamp(0.5+n.x*2.5,0.0,1.0));col=mix(col,sky,0.22+0.62*fresnel);
vec3 refl=reflect(-view,n);float spec=pow(max(dot(refl,sunDir),0.0),260.0)*2.6+pow(max(dot(refl,sunDir),0.0),28.0)*0.22;
float sparkle=pow(max(0.0,sin(p.x*3.7+time*3.0)*sin(p.y*4.1-time*2.3)),40.0)*0.35*max(0.0,dot(refl,sunDir));
col+=sunColor*(spec+sparkle);gl_FragColor=vec4(col,1.0);
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
const POST = `uniform sampler2D tDiffuse;uniform float time,vignette,grain;varying vec2 vUv;
float hash2(vec2 p){return fract(sin(dot(p,vec2(12.9898,78.233)))*43758.5453);}
void main(){vec4 c=texture2D(tDiffuse,vUv);vec2 d=vUv-0.5;float v=1.0-vignette*smoothstep(0.28,1.05,dot(d,d)*2.4);
float g=(hash2(vUv*vec2(1920.0,1080.0)+fract(time)*7.0)-0.5)*grain;gl_FragColor=vec4(c.rgb*v+g,c.a);}`;

export function createAtmosphere(scene, options = {}) {
  const geometries = [], materials = [], group = new THREE.Group(); group.name = 'city-atmosphere'; scene.add(group);
  const own = (g, m) => { geometries.push(g); materials.push(m); return new THREE.Mesh(g, m); };
  const time = { value: 0 }, pointScale = { value: 900 };
  const sunDir = (options.sunDirection || new THREE.Vector3(-70, 145, 85)).clone().normalize();
  const zenith = new THREE.Color(0x05091a), horizon = new THREE.Color(0x14324c), warm = new THREE.Color(0x7a3d2a);
  const haze = new THREE.Color(scene.fog?.color || 0x08121e);
  const shader = (vertex, fragment, uniforms, extra = {}) => new THREE.ShaderMaterial({ uniforms, vertexShader: vertex, fragmentShader: fragment, ...extra });
  const additive = { transparent: true, depthWrite: false, blending: THREE.AdditiveBlending };
  // Sky dome and stars are unlit backdrops outside the fog.
  const sky = own(new THREE.SphereGeometry(1500, 48, 24), shader(WORLD_VERTEX, SKY, { time, zenith: { value: zenith }, horizon: { value: horizon }, haze: { value: haze }, warm: { value: warm }, sunDir: { value: sunDir } }, { side: THREE.BackSide, depthWrite: false, fog: false }));
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
  const starMaterial = shader(STARS.vertex, STARS.fragment, { time, pointScale }, { ...additive, fog: false });
  const stars = new THREE.Points(starGeometry, starMaterial); stars.frustumCulled = false; stars.renderOrder = -9; group.add(stars);
  geometries.push(starGeometry); materials.push(starMaterial);
  // Animated water replaces the flat standard sea; fog still fades it to the horizon.
  const sea = own(new THREE.PlaneGeometry(1800, 1800), shader(SEA.vertex, SEA.fragment, {
    time, deep: { value: new THREE.Color(0x061420) }, shallow: { value: new THREE.Color(0x0e3350) }, sky: { value: horizon.clone().multiplyScalar(.7) },
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
  // Final grade: soft vignette and fine grain after the output pass.
  const post = new ShaderPass(new THREE.ShaderMaterial({ uniforms: { tDiffuse: { value: null }, time, vignette: { value: .42 }, grain: { value: .028 } }, vertexShader: PANEL_VERTEX(), fragmentShader: POST }));
  post.enabled = false; materials.push(post.material);
  let busy = false, tier = 'high';
  return {
    post, sea,
    setTier(value) {
      tier = value;
      const rich = tier !== 'low';
      mist.visible = rich; dust.visible = rich; lampCones.visible = rich;
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
    },
    stats: () => ({ tier, busy, stars: starCount, dust: dustCount, lamps: lamps.length, post: post.enabled }),
    dispose() {
      group.removeFromParent(); lampCones.dispose();
      geometries.forEach(g => g.dispose()); materials.forEach(m => m.dispose()); post.dispose?.();
    },
  };
}
function PANEL_VERTEX() { return 'varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}'; }
