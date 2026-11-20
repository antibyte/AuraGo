import {
    CREATURE_SWIM,
    CREATURE_COLORS,
    CREATURE_ALBEDO,
    JELLY_FN,
    JELLY_SWIM,
    JELLY_COLOR,
    MANTA_SWIM,
    MANTA_ALBEDO,
    SHADER_ID,
    MATERIAL_KIND,
    DOUBLE_SIDED,
    JELLY_PART
} from '../../chat/ocean-life-shaders.js';
import { SEA } from './minigolf-sealife-paths.js';

// Pirate Bay sea life shaders. Reuses the Ocean chat GLSL swim deformation and procedural colours,
// remaps underwater vertices onto the sea plane just toward the camera so the opaque lagoon disc
// does not hide them, clips anything inside the superellipse island rim, and matches AuraGolf's
// fog, NeutralToneMapping and sRGB output pipeline.
export const REMAP = /* glsl */`uniform float uSea;
vec4 seaClip(vec3 P) {
  float below = uSea - P.y;
  vec3 S = P;
  if (below > 0.0) {
    vec3 C = cameraPosition;
    float h = max(C.y - uSea, 0.02);
    S = C + (P - C) * (h / max(C.y - P.y, h));
    vec3 toC = C - S;
    float len = max(length(toC), 0.001);
    S += toC / len * ((0.03 + 0.02 * clamp(6.0 - below, 0.0, 6.0)) * (1.0 + 0.01 * len));
  }
  return projectionMatrix * viewMatrix * vec4(S, 1.0);
}`;

const ISLE_MASK = /* glsl */`uniform float uIslePow;
uniform vec4 uIsleBox;
bool insideIsle(vec3 wp) {
  vec2 d = max(abs((wp.xz - uIsleBox.xy) / max(uIsleBox.zw, vec2(0.1))), vec2(1e-5));
  return pow(d.x, uIslePow) + pow(d.y, uIslePow) < 1.0;
}`;

export const WATER = /* glsl */`uniform float uSea, uTime, uInner, uOuter;
uniform vec3 uSunDir, uSunColor, uShallow, uDeep, uSky;
${ISLE_MASK}
const vec3 ABSORB = vec3(0.45, 0.16, 0.11);
vec3 litSurface(vec3 albedo, vec3 N, vec3 V, float depth, float gloss, float shimmer) {
  vec3 L = normalize(uSunDir);
  float ndl = dot(N, L), diffuse = max(ndl, 0.0), wrap = clamp(ndl * 0.5 + 0.5, 0.0, 1.0);
  vec3 att = exp(-depth * ABSORB);
  vec3 lit = albedo * (uSky * (0.9 + 0.45 * N.y) * sqrt(att) + uSunColor * (diffuse * 0.95 + wrap * 0.12) * att * 0.95);
  lit += albedo * uSunColor * att * shimmer * 0.85;
  lit += albedo * vec3(0.03, 0.05, 0.07) * sqrt(att);
  lit += uSunColor * att * pow(max(dot(reflect(-L, N), V), 0.0), 40.0) * gloss * 0.35;
  return lit;
}
vec4 waterLook(vec3 lit, vec3 wp, vec3 V) {
  float d = max(uSea - wp.y, 0.0), cosV = abs(V.y), path = d / max(cosV, 0.12);
  vec3 T = exp(-path * ABSORB);
  float F = 0.02 + 0.98 * pow(clamp(1.0 - cosV, 0.0, 1.0), 5.0);
  float radial = smoothstep(uInner, max(uOuter, uInner + 0.1), length(wp.xz));
  vec3 scatter = mix(uShallow, uDeep, radial);
  return vec4(lit * T + scatter * (1.0 - T), (1.0 - F) * pow(T.g, 0.65));
}
vec4 seaFinish(vec3 lit, float alpha, vec3 wp, vec3 V) {
  float wet = smoothstep(-0.01, 0.05, uSea - wp.y);
  vec4 wl = waterLook(lit, wp, V);
  return vec4(min(mix(lit, wl.rgb, wet), vec3(0.92)), alpha * mix(1.0, wl.a, wet));
}`;

const CREATURE_VERTEX = /* glsl */`attribute vec2 aAnim;
attribute vec4 aInst;
#ifdef MERGED
attribute float aMat;
varying float vMat;
#endif
uniform float uTime, uShader, uSwimK, uLength, uFront;
varying vec3 vWorldPos, vWorldNormal;
varying vec2 vUv, vAnim;
varying float vFade;
${REMAP}
#include <fog_pars_vertex>
void main() {
  vec3 p = position;
${CREATURE_SWIM}
  vec4 world = modelMatrix * instanceMatrix * vec4(p, 1.0);
  vec4 mvPosition = viewMatrix * world;
  vWorldPos = world.xyz;
  vWorldNormal = normalize(mat3(modelMatrix) * mat3(instanceMatrix) * normal);
  vUv = uv;
  vAnim = aAnim;
  vFade = aInst.w;
#ifdef MERGED
  vMat = aMat;
#endif
  gl_Position = seaClip(world.xyz);
#include <fog_vertex>
}`;

const CREATURE_FRAGMENT = /* glsl */`uniform float uShader, uVariant;
#ifdef MERGED
varying float vMat;
#define uMat vMat
#else
uniform float uMat;
#endif
varying vec3 vWorldPos, vWorldNormal;
varying vec2 vUv, vAnim;
varying float vFade;
${WATER}
#include <fog_pars_fragment>
${CREATURE_COLORS}
void main() {
  vec3 N = normalize(vWorldNormal);
  if (!gl_FrontFacing) N = -N;
  vec3 V = normalize(cameraPosition - vWorldPos);
${CREATURE_ALBEDO}
  albedo = uShader > 2.5 ? albedo * 1.15 + 0.02 : pow(max(albedo, vec3(0.0)), vec3(2.2));
  float up = smoothstep(0.05, 0.75, N.y);
  vec2 cp = vWorldPos.xz * 0.9;
  float web = clamp(1.0 - abs(vnoise(cp * 1.3 + vec2(uTime * 0.16, uTime * 0.11)) + vnoise(cp * 2.1 - vec2(uTime * 0.12, -uTime * 0.17) + 7.3) - 1.0), 0.0, 1.0);
  vec3 lit = litSurface(albedo, N, V, max(uSea - vWorldPos.y, 0.0), gloss, pow(web, 6.0) * up);
  vec4 o = seaFinish(lit, alpha * vFade, vWorldPos, V);
  if (o.a < 0.01 || insideIsle(vWorldPos)) discard;
  gl_FragColor = o;
#include <fog_fragment>
#include <tonemapping_fragment>
#include <colorspace_fragment>
}`;

const JELLY_VERTEX = /* glsl */`attribute vec2 aAnim;
attribute vec4 aInst;
attribute vec3 aTint;
uniform float uTime, uPart;
varying vec3 vViewPos, vNormal, vWorldPos, vTint;
varying vec2 vUv, vAnim;
varying float vFade, vPulse;
${REMAP}
#include <fog_pars_vertex>
${JELLY_FN}
void main() {
${JELLY_SWIM}
  vec4 world = modelMatrix * instanceMatrix * vec4(p, 1.0);
  vec4 mvPosition = viewMatrix * world;
  vViewPos = mvPosition.xyz;
  vWorldPos = world.xyz;
  vNormal = normalize(mat3(viewMatrix) * mat3(modelMatrix) * mat3(instanceMatrix) * normal);
  vUv = uv;
  vAnim = aAnim;
  vTint = aTint;
  vFade = aInst.w;
  vPulse = k;
  gl_Position = seaClip(world.xyz);
#include <fog_vertex>
}`;

const JELLY_FRAGMENT = /* glsl */`uniform float uPart, uBusy;
uniform vec3 uBase, uGlow;
varying vec3 vViewPos, vNormal, vWorldPos, vTint;
varying vec2 vUv, vAnim;
varying float vFade, vPulse;
${WATER}
#include <fog_pars_fragment>
void main() {
${JELLY_COLOR}
  vec3 Vw = normalize(cameraPosition - vWorldPos);
  col = pow(max(col, vec3(0.0)), vec3(2.2));
  vec3 lit = col * (uSky * 1.15 + uSunColor * 0.5);
  vec4 o = seaFinish(lit, alpha * vFade * 0.72, vWorldPos, Vw);
  if (o.a < 0.01 || insideIsle(vWorldPos)) discard;
  gl_FragColor = o;
#include <fog_fragment>
#include <tonemapping_fragment>
#include <colorspace_fragment>
}`;

const MANTA_VERTEX = /* glsl */`attribute vec2 aAnim;
attribute vec4 aInst;
uniform float uTime, uFlap, uUnit;
varying vec3 vWorldPos, vWorldNormal, vObj;
varying float vBelly, vSpan, vFade;
${REMAP}
#include <fog_pars_vertex>
void main() {
${MANTA_SWIM}
  vec4 world = modelMatrix * instanceMatrix * vec4(p, 1.0);
  vec4 mvPosition = viewMatrix * world;
  vWorldPos = world.xyz;
  vWorldNormal = normalize(mat3(modelMatrix) * mat3(instanceMatrix) * normal);
  vBelly = aAnim.y;
  vSpan = span;
  vFade = aInst.w;
  gl_Position = seaClip(world.xyz);
#include <fog_vertex>
}`;

const MANTA_FRAGMENT = /* glsl */`varying vec3 vWorldPos, vWorldNormal, vObj;
varying float vBelly, vSpan, vFade;
${WATER}
#include <fog_pars_fragment>
void main() {
  vec3 N = normalize(vWorldNormal);
  if (!gl_FrontFacing) N = -N;
  vec3 V = normalize(cameraPosition - vWorldPos);
${MANTA_ALBEDO}
  albedo = albedo * 0.5;
  vec3 lit = litSurface(albedo, N, V, max(uSea - vWorldPos.y, 0.0), 0.1, 0.0);
  vec4 o = seaFinish(lit, vFade, vWorldPos, V);
  if (o.a < 0.01 || insideIsle(vWorldPos)) discard;
  gl_FragColor = o;
#include <fog_fragment>
#include <tonemapping_fragment>
#include <colorspace_fragment>
}`;

const RING_VERTEX = /* glsl */`attribute vec2 aRing;
varying float vAge, vEdge, vOpacity;
varying vec3 vWorldPos;
#include <fog_pars_vertex>
void main() {
  vec4 world = modelMatrix * instanceMatrix * vec4(position, 1.0);
  vec4 mvPosition = viewMatrix * world;
  vWorldPos = world.xyz;
  vAge = aRing.x;
  vOpacity = aRing.y;
  vEdge = (length(position.xz) - 0.58) / 0.42;
  gl_Position = projectionMatrix * mvPosition;
#include <fog_vertex>
}`;

const RING_FRAGMENT = /* glsl */`varying float vAge, vEdge, vOpacity;
varying vec3 vWorldPos;
${ISLE_MASK}
#include <fog_pars_fragment>
void main() {
  float a = pow(clamp(1.0 - vAge, 0.0, 1.0), 1.5) * sin(clamp(vEdge, 0.0, 1.0) * 3.14159) * 0.72 * vOpacity;
  if (a < 0.01 || insideIsle(vWorldPos)) discard;
  gl_FragColor = vec4(vec3(0.84, 0.96, 0.98), a);
#include <fog_fragment>
#include <tonemapping_fragment>
#include <colorspace_fragment>
}`;

const PUFF_VERTEX = /* glsl */`attribute vec2 aPuff;
varying vec2 vP;
varying float vAge;
#include <fog_pars_vertex>
void main() {
  vec3 centre = (modelMatrix * instanceMatrix * vec4(0.0, 0.0, 0.0, 1.0)).xyz;
  float size = length(instanceMatrix[0].xyz);
  vec3 right = vec3(viewMatrix[0][0], viewMatrix[1][0], viewMatrix[2][0]);
  vec3 up = vec3(viewMatrix[0][1], viewMatrix[1][1], viewMatrix[2][1]);
  vec4 mvPosition = viewMatrix * vec4(centre + (right * position.x + up * position.y) * size, 1.0);
  vP = position.xy;
  vAge = aPuff.x;
  gl_Position = projectionMatrix * mvPosition;
#include <fog_vertex>
}`;

const PUFF_FRAGMENT = /* glsl */`varying vec2 vP;
varying float vAge;
#include <fog_pars_fragment>
void main() {
  float r = length(vP) * 2.0;
  float a = (1.0 - smoothstep(0.15, 1.0, r)) * pow(clamp(1.0 - vAge, 0.0, 1.0), 1.3) * smoothstep(0.0, 0.08, vAge) * 0.75;
  if (a < 0.01) discard;
  gl_FragColor = vec4(vec3(0.84, 0.94, 0.97), a);
#include <fog_fragment>
#include <tonemapping_fragment>
#include <colorspace_fragment>
}`;

export { CREATURE_VERTEX, CREATURE_FRAGMENT, JELLY_VERTEX, JELLY_FRAGMENT, MANTA_VERTEX, MANTA_FRAGMENT, RING_VERTEX, RING_FRAGMENT, PUFF_VERTEX, PUFF_FRAGMENT };

export function createPirateSeaMaterials(THREE, clock) {
    const shared = {
        uSea: { value: SEA },
        uTime: clock,
        uInner: { value: 4.8 },
        uOuter: { value: 24 },
        uIslePow: { value: 3 },
        uIsleBox: { value: new THREE.Vector4(0, 0, 10, 10) },
        uSunDir: { value: new THREE.Vector3(-0.42, 0.84, 0.34).normalize() },
        uSunColor: { value: new THREE.Color(0xfff1d6) },
        uShallow: { value: new THREE.Color(0x62e3d6) },
        uDeep: { value: new THREE.Color(0x0f6f93) },
        uSky: { value: new THREE.Color(0x68bdd6) }
    };
    const all = [];
    const make = (vertexShader, fragmentShader, extra, options) => {
        const uniforms = Object.assign(THREE.UniformsUtils.clone(THREE.UniformsLib.fog), shared, extra);
        const m = new THREE.ShaderMaterial({
            uniforms,
            vertexShader,
            fragmentShader,
            fog: true,
            transparent: true,
            ...options
        });
        all.push(m);
        return m;
    };
    return {
        setEnvironment({ look, waterRadius = 60, isle }) {
            const reach = Math.max(1, waterRadius / 60);
            shared.uInner.value = 4.8 * reach;
            shared.uOuter.value = 24 * reach;
            if (look?.water) {
                shared.uSea.value = look.water.level ?? SEA;
                if (look.water.shallow !== undefined) shared.uShallow.value.set(look.water.shallow);
                if (look.water.deep !== undefined) shared.uDeep.value.set(look.water.deep);
            }
            if (look?.sun) {
                shared.uSunDir.value.fromArray(look.sun.direction).normalize();
                shared.uSunColor.value.set(look.sun.color);
            }
            shared.uSky.value.set(0x68bdd6);
            if (isle) {
                shared.uIsleBox.value.set(isle.cx, isle.cz, isle.solidHx, isle.solidHz);
                shared.uIslePow.value = isle.power;
            }
        },
        creatureMerged(species) {
            const { spec } = species;
            return make(CREATURE_VERTEX, CREATURE_FRAGMENT, {
                uShader: { value: SHADER_ID[spec.shader] ?? 0 },
                uVariant: { value: spec.variant },
                uSwimK: { value: spec.swim.k },
                uLength: { value: spec.length },
                uFront: { value: species.front }
            }, { depthWrite: true, side: THREE.FrontSide, defines: { MERGED: '' } });
        },
        jelly(part) {
            const kind = JELLY_PART[part] ?? 0;
            return make(JELLY_VERTEX, JELLY_FRAGMENT, {
                uPart: { value: kind },
                uBusy: { value: 0 },
                uBase: { value: new THREE.Vector3(0.45, 0.78, 1.0) },
                uGlow: { value: kind === 1 ? new THREE.Vector3(1.0, 0.45, 0.85) : new THREE.Vector3(0.65, 0.72, 1.0) }
            }, { depthWrite: false, side: THREE.DoubleSide });
        },
        manta(species, flap = 1.2) {
            return make(MANTA_VERTEX, MANTA_FRAGMENT, {
                uFlap: { value: flap },
                uUnit: { value: species.scale }
            }, { depthWrite: true, side: THREE.DoubleSide });
        },
        ring() {
            return make(RING_VERTEX, RING_FRAGMENT, {}, { depthWrite: false, side: THREE.DoubleSide });
        },
        puff() {
            return make(PUFF_VERTEX, PUFF_FRAGMENT, {}, { depthWrite: false, side: THREE.DoubleSide });
        },
        count: () => all.length,
        dispose() {
            for (const m of all) m.dispose();
            all.length = 0;
        }
    };
}
