import {CREATURE_SWIM, CREATURE_COLORS, CREATURE_ALBEDO, JELLY_FN, JELLY_SWIM, JELLY_COLOR, MANTA_SWIM, MANTA_ALBEDO, SHADER_ID, MATERIAL_KIND, DOUBLE_SIDED, JELLY_PART} from '../../chat/ocean-life-shaders.js';
import {SEA, ISLAND} from './sysworld-sealife-paths.js';
// Sea life materials of the System World. The swim deformation and the procedural colours are the chat's own GLSL
// (ui/js/chat/ocean-life-shaders.js, pasted in as chunks); what is new is how the animals sit in the city's water:
//  - The sea is one opaque plane that never changes. A transparent animal below it is drawn where the view ray through each
//    vertex crosses the plane, a few centimetres towards the camera (seaClip; its JS twin is seaRemap in sysworld-sealife-paths.js):
//    same screen position, surface depth, so the sea does not hide it, while the quay, hulls and buoys in front still do.
//  - Below the surface the colour goes through the water: the red goes first (absorption), the rest scatters into the sea's
//    shallow colour, and the Fresnel reflection of the surface takes its share. Above it the animal is opaque and lit normally.
//  - Colour leaves as linear HDR like the sea's own shader (the composer's output pass tone maps); nothing exceeds 1.0, so
//    nothing reaches the bloom threshold. Fog is the scene's.
// Every division is guarded: the camera is always above the sea, but a reflected view must not produce NaN.
export const REMAP = `uniform float uSea;
vec4 seaClip(vec3 P){
  float below=uSea-P.y;
  vec3 S=P;
  if(below>0.0){
    vec3 C=cameraPosition;
    float h=max(C.y-uSea,0.02);
    S=C+(P-C)*(h/max(C.y-P.y,h));
    vec3 toC=C-S;
    float len=max(length(toC),0.001);
    S+=toC/len*((0.03+0.02*clamp(6.0-below,0.0,6.0))*(1.0+0.01*len));
  }
  return projectionMatrix*viewMatrix*vec4(S,1.0);
}`;
export const WATER = `uniform float uSea,uDay,uDusk,uTime;
uniform vec3 uSunDir,uSunColor,uShallow,uSky;
uniform vec4 uIsle;   // the island footprint (x0,x1,z0,z1): the island is solid, nothing of the sea life may show from inside it
bool insideIsle(vec3 wp){return wp.x>uIsle.x&&wp.x<uIsle.y&&wp.z>uIsle.z&&wp.z<uIsle.w;}
const vec3 ABSORB=vec3(0.45,0.16,0.11);
vec3 litSurface(vec3 albedo,vec3 N,vec3 V,float depth,float gloss,float shimmer){
  vec3 L=normalize(uSunDir);
  float ndl=dot(N,L),diffuse=max(ndl,0.0),wrap=clamp(ndl*0.5+0.5,0.0,1.0);
  vec3 sun=uSunColor*uDay,att=exp(-depth*ABSORB);
  vec3 lit=albedo*(uSky*(0.95+0.45*N.y)*sqrt(att)+sun*(diffuse*0.9+wrap*0.1)*att*0.9);
  lit+=albedo*sun*att*shimmer*0.8;
  lit+=albedo*vec3(0.025,0.04,0.06)*sqrt(att);   // never quite black: water scatters a little light everywhere
  lit+=sun*att*pow(max(dot(reflect(-L,N),V),0.0),40.0)*gloss*0.3;
  return lit;
}
vec4 waterLook(vec3 lit,vec3 wp,vec3 V){
  float d=max(uSea-wp.y,0.0),cosV=abs(V.y),path=d/max(cosV,0.12);
  vec3 T=exp(-path*ABSORB);
  float F=0.02+0.98*pow(clamp(1.0-cosV,0.0,1.0),5.0);
  vec3 scatter=uShallow*(0.2+0.8*uDay);
  return vec4(lit*T+scatter*(1.0-T),(1.0-F)*pow(T.g,0.7));
}
vec4 seaFinish(vec3 lit,float alpha,vec3 wp,vec3 V){
  float wet=smoothstep(-0.01,0.05,uSea-wp.y);
  vec4 wl=waterLook(lit,wp,V);
  return vec4(min(mix(lit,wl.rgb,wet),vec3(1.0)),alpha*mix(1.0,wl.a,wet));
}`;
const CREATURE_VERTEX = `attribute vec2 aAnim;
attribute vec4 aInst;
#ifdef MERGED
attribute float aMat;
varying float vMat;
#endif
uniform float uTime,uShader,uSwimK,uLength,uFront;
varying vec3 vWorldPos,vWorldNormal;
varying vec2 vUv,vAnim;
varying float vFade;
${REMAP}
#include <fog_pars_vertex>
void main(){
  vec3 p=position;
${CREATURE_SWIM}
  vec4 world=modelMatrix*instanceMatrix*vec4(p,1.0);
  vec4 mvPosition=viewMatrix*world;
  vWorldPos=world.xyz;
  vWorldNormal=normalize(mat3(modelMatrix)*mat3(instanceMatrix)*normal);
  vUv=uv;vAnim=aAnim;vFade=aInst.w;
#ifdef MERGED
  vMat=aMat;
#endif
  gl_Position=seaClip(world.xyz);
#include <fog_vertex>
}`;
const CREATURE_FRAGMENT = `uniform float uShader,uVariant;
#ifdef MERGED
varying float vMat;
#define uMat vMat
#else
uniform float uMat;
#endif
varying vec3 vWorldPos,vWorldNormal;
varying vec2 vUv,vAnim;
varying float vFade;
${WATER}
#include <fog_pars_fragment>
${CREATURE_COLORS}
void main(){
  vec3 N=normalize(vWorldNormal);
  if(!gl_FrontFacing)N=-N;
  vec3 V=normalize(cameraPosition-vWorldPos);
${CREATURE_ALBEDO}
  // The chat writes its colours straight to the screen; here they are lit in linear light. The whale is dark by design: kept as it is.
  albedo=uShader>2.5?albedo*1.15+0.02:pow(max(albedo,vec3(0.0)),vec3(2.2));
  float up=smoothstep(0.05,0.75,N.y);
  vec2 cp=vWorldPos.xz*0.9;
  float web=clamp(1.0-abs(vnoise(cp*1.3+vec2(uTime*0.16,uTime*0.11))+vnoise(cp*2.1-vec2(uTime*0.12,-uTime*0.17)+7.3)-1.0),0.0,1.0);
  vec3 lit=litSurface(albedo,N,V,max(uSea-vWorldPos.y,0.0),gloss,pow(web,6.0)*up);
  vec4 o=seaFinish(lit,alpha*vFade,vWorldPos,V);
  if(o.a<0.01||insideIsle(vWorldPos))discard;
  gl_FragColor=o;
#include <fog_fragment>
}`;
const JELLY_VERTEX = `attribute vec2 aAnim;
attribute vec4 aInst;
attribute vec3 aTint;
uniform float uTime,uPart;
varying vec3 vViewPos,vNormal,vWorldPos,vTint;
varying vec2 vUv,vAnim;
varying float vFade,vPulse;
${REMAP}
#include <fog_pars_vertex>
${JELLY_FN}
void main(){
${JELLY_SWIM}
  vec4 world=modelMatrix*instanceMatrix*vec4(p,1.0);
  vec4 mvPosition=viewMatrix*world;
  vViewPos=mvPosition.xyz;vWorldPos=world.xyz;
  vNormal=normalize(mat3(viewMatrix)*mat3(modelMatrix)*mat3(instanceMatrix)*normal);
  vUv=uv;vAnim=aAnim;vTint=aTint;vFade=aInst.w;vPulse=k;
  gl_Position=seaClip(world.xyz);
#include <fog_vertex>
}`;
// Moon jelly: the chat's colour and alpha, then the water. By day it is faint; at night it glows softly (capped at 1.0).
const JELLY_FRAGMENT = `uniform float uPart,uBusy;
uniform vec3 uBase,uGlow;
varying vec3 vViewPos,vNormal,vWorldPos,vTint;
varying vec2 vUv,vAnim;
varying float vFade,vPulse;
${WATER}
#include <fog_pars_fragment>
void main(){
${JELLY_COLOR}
  vec3 Vw=normalize(cameraPosition-vWorldPos);
  float night=1.0-uDay;
  col=pow(max(col,vec3(0.0)),vec3(2.2));
  vec3 lit=col*(uSky*1.4+uSunColor*uDay*0.45)+col*night*0.9;
  vec4 o=seaFinish(lit,alpha*vFade*(0.4+0.6*night),vWorldPos,Vw);
  if(o.a<0.01||insideIsle(vWorldPos))discard;
  gl_FragColor=o;
#include <fog_fragment>
}`;
const MANTA_VERTEX = `attribute vec2 aAnim;
attribute vec4 aInst;
uniform float uTime,uFlap,uUnit;
varying vec3 vWorldPos,vWorldNormal,vObj;
varying float vBelly,vSpan,vFade;
${REMAP}
#include <fog_pars_vertex>
void main(){
${MANTA_SWIM}
  vec4 world=modelMatrix*instanceMatrix*vec4(p,1.0);
  vec4 mvPosition=viewMatrix*world;
  vWorldPos=world.xyz;
  vWorldNormal=normalize(mat3(modelMatrix)*mat3(instanceMatrix)*normal);
  vBelly=aAnim.y;vSpan=span;vFade=aInst.w;
  gl_Position=seaClip(world.xyz);
#include <fog_vertex>
}`;
const MANTA_FRAGMENT = `varying vec3 vWorldPos,vWorldNormal,vObj;
varying float vBelly,vSpan,vFade;
${WATER}
#include <fog_pars_fragment>
void main(){
  vec3 N=normalize(vWorldNormal);
  if(!gl_FrontFacing)N=-N;
  vec3 V=normalize(cameraPosition-vWorldPos);
${MANTA_ALBEDO}
  albedo=albedo*0.5;   // dark by design: a dark back against the water, not gamma-lowered
  vec3 lit=litSurface(albedo,N,V,max(uSea-vWorldPos.y,0.0),0.1,0.0);
  vec4 o=seaFinish(lit,vFade,vWorldPos,V);
  if(o.a<0.01||insideIsle(vWorldPos))discard;
  gl_FragColor=o;
#include <fog_fragment>
}`;
// Splash ring: flat on the surface, grows and fades (age 0..1 per instance).
const RING_VERTEX = `attribute vec2 aRing;
varying float vAge,vEdge;
#include <fog_pars_vertex>
void main(){
  vec4 world=modelMatrix*instanceMatrix*vec4(position,1.0);
  vec4 mvPosition=viewMatrix*world;
  vAge=aRing.x;vEdge=(length(position.xz)-0.6)/0.4;
  gl_Position=projectionMatrix*mvPosition;
#include <fog_vertex>
}`;
const RING_FRAGMENT = `uniform float uDay;
varying float vAge,vEdge;
#include <fog_pars_fragment>
void main(){
  float a=pow(clamp(1.0-vAge,0.0,1.0),1.6)*sin(clamp(vEdge,0.0,1.0)*3.14159)*0.75;
  if(a<0.01)discard;
  gl_FragColor=vec4(vec3(0.82,0.93,1.0)*(0.3+0.7*uDay)*0.9,a);
#include <fog_fragment>
}`;
// Blow puff: a soft billboard (age 0..1 per instance, the size is the instance scale).
const PUFF_VERTEX = `attribute vec2 aPuff;
varying vec2 vP;
varying float vAge;
#include <fog_pars_vertex>
void main(){
  vec3 centre=(modelMatrix*instanceMatrix*vec4(0.0,0.0,0.0,1.0)).xyz;
  float size=length(instanceMatrix[0].xyz);
  vec3 right=vec3(viewMatrix[0][0],viewMatrix[1][0],viewMatrix[2][0]),up=vec3(viewMatrix[0][1],viewMatrix[1][1],viewMatrix[2][1]);
  vec4 mvPosition=viewMatrix*vec4(centre+(right*position.x+up*position.y)*size,1.0);
  vP=position.xy;vAge=aPuff.x;
  gl_Position=projectionMatrix*mvPosition;
#include <fog_vertex>
}`;
const PUFF_FRAGMENT = `uniform float uDay;
varying vec2 vP;
varying float vAge;
#include <fog_pars_fragment>
void main(){
  float r=length(vP)*2.0;
  float a=(1.0-smoothstep(0.15,1.0,r))*pow(clamp(1.0-vAge,0.0,1.0),1.3)*smoothstep(0.0,0.08,vAge)*0.8;
  if(a<0.01)discard;
  gl_FragColor=vec4(vec3(0.85,0.93,1.0)*(0.18+0.72*uDay),a);
#include <fog_fragment>
}`;
// water: the atmosphere's live uniform objects (day, dusk, sunDir, sunColor, shallow, sky); clock: {value} of the sea life.
export function createSeaMaterials(THREE, water, clock) {
  const fog = { fogColor: { value: new THREE.Color(0x08121e) }, fogDensity: { value: .004 }, fogNear: { value: 1 }, fogFar: { value: 1000 } };
  const common = { uSea: { value: SEA }, uIsle: { value: new THREE.Vector4(ISLAND.x0, ISLAND.x1, ISLAND.z0, ISLAND.z1) }, uTime: clock, uDay: water.day, uDusk: water.dusk, uSunDir: water.sunDir, uSunColor: water.sunColor, uShallow: water.shallow, uSky: water.sky, ...fog };
  const all = [];
  const make = (vertexShader, fragmentShader, extra, options) => {
    const m = new THREE.ShaderMaterial({ uniforms: { ...common, ...extra }, vertexShader, fragmentShader, fog: true, transparent: true, ...options }); all.push(m); return m;
  };
  return {
    // part: the glTF material name (body, fin, eye, shell, flipper, fluke); species: {spec, front}.
    creature(species, part) {
      const { spec } = species;
      return make(CREATURE_VERTEX, CREATURE_FRAGMENT, { uShader: { value: SHADER_ID[spec.shader] ?? 0 }, uVariant: { value: spec.variant }, uMat: { value: MATERIAL_KIND[part] ?? 0 },
        uSwimK: { value: spec.swim.k }, uLength: { value: spec.length }, uFront: { value: species.front } }, { depthWrite: true, side: DOUBLE_SIDED[part] ? THREE.DoubleSide : THREE.FrontSide });
    },
    // all parts of one model in one mesh: the material kind comes from the vertex attribute aMat
    creatureMerged(species) {
      const { spec } = species;
      return make(CREATURE_VERTEX, CREATURE_FRAGMENT, { uShader: { value: SHADER_ID[spec.shader] ?? 0 }, uVariant: { value: spec.variant }, uSwimK: { value: spec.swim.k }, uLength: { value: spec.length }, uFront: { value: species.front } },
        { depthWrite: true, side: THREE.FrontSide, defines: { MERGED: '' } });
    },
    jelly(part) {
      const kind = JELLY_PART[part] ?? 0;
      return make(JELLY_VERTEX, JELLY_FRAGMENT, { uPart: { value: kind }, uBusy: { value: 0 }, uBase: { value: new THREE.Vector3(.5, .72, 1) },
        uGlow: { value: kind === 1 ? new THREE.Vector3(1, .45, .85) : new THREE.Vector3(.75, .55, 1) } }, { depthWrite: false, side: THREE.DoubleSide });
    },
    manta(species, flap = 1.2) {
      return make(MANTA_VERTEX, MANTA_FRAGMENT, { uFlap: { value: flap }, uUnit: { value: species.scale } }, { depthWrite: true, side: THREE.DoubleSide });
    },
    ring() { return make(RING_VERTEX, RING_FRAGMENT, {}, { depthWrite: false, side: THREE.DoubleSide }); },
    puff() { return make(PUFF_VERTEX, PUFF_FRAGMENT, {}, { depthWrite: false, side: THREE.DoubleSide }); },
    count: () => all.length,
    dispose() { all.forEach(m => m.dispose()); all.length = 0; },
  };
}
