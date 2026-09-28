import * as THREE from 'three';
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';
import { EffectComposer } from 'three/addons/postprocessing/EffectComposer.js';
import { RenderPass } from 'three/addons/postprocessing/RenderPass.js';
import { UnrealBloomPass } from 'three/addons/postprocessing/UnrealBloomPass.js';
import { ShaderPass } from 'three/addons/postprocessing/ShaderPass.js';
import { OutputPass } from 'three/addons/postprocessing/OutputPass.js';

// Tiefsee: a bioluminescent deep-sea scene. Creatures are Blender-authored GLBs whose
// animation is driven in the vertex shaders from baked UV sets (see assets/screensaver-abyss).

const MODEL_BASE = '/3d/screensaver/abyss/v1/';
const QUALITY = {
    high: { moon: 12, lion: 3, comb: 7, snow: 26000, sparkles: 1400, bloomScale: 0.5, rays: 11 },
    medium: { moon: 8, lion: 2, comb: 5, snow: 15000, sparkles: 900, bloomScale: 0.42, rays: 8 },
    low: { moon: 5, lion: 1, comb: 3, snow: 7000, sparkles: 500, bloomScale: 0.34, rays: 6 }
};
const FOG_COLOR = new THREE.Color(0.006, 0.03, 0.055);
const FOG_DENSITY = 0.034;
const SURFACE_Y = 22;
const SWARM_RADIUS = 11;
const SWARM_TOP = 11;
const SWARM_BOTTOM = -17;

const PART = { bell: 0, gonad: 1, arm: 2, tentacle: 3, comb_body: 4, comb_row: 5 };

const JELLY_VERTEX = /* glsl */`
attribute vec2 aAnim;
attribute vec4 aInst;   // phase, pulse frequency, swim, fade
attribute vec3 aTint;
uniform float uTime;
uniform float uPart;
varying vec3 vViewPos;
varying vec3 vNormal;
varying vec2 vUv;
varying vec2 vAnim;
varying vec3 vTint;
varying float vFade;
varying float vPulse;

float contraction(float c) {
    return c < 0.22 ? smoothstep(0.0, 0.22, c) : 1.0 - smoothstep(0.22, 1.0, c);
}

void main() {
    float cycle = fract(uTime * aInst.y + aInst.x);
    float k = contraction(cycle);
    float along = clamp(uv.x, 0.0, 1.0);
    float phase = uv.y;
    vec3 p = position;
    if (uPart < 0.5 || (uPart > 0.5 && uPart < 1.5)) {
        float a = uPart < 0.5 ? along : clamp(length(p.xz), 0.0, 1.0);
        p.xz *= 1.0 - 0.17 * k * pow(max(a, 0.0), 1.3);
        p.y += 0.07 * k * a;
    } else if (uPart < 3.5) {
        float lagK = contraction(fract(cycle - along * 0.35));
        p.xz *= 1.0 - 0.15 * lagK * (1.0 - along * 0.7);
        float sway = pow(max(along, 0.0), 1.5) * (uPart > 2.5 ? 0.22 : 0.16) * (1.0 - 0.5 * aInst.z);
        p.x += sin(uTime * 0.9 + phase * 6.2832 + along * 5.0) * sway;
        p.z += cos(uTime * 0.7 + phase * 6.2832 + along * 4.2) * sway;
        p.y -= aInst.z * pow(max(along, 0.0), 1.2) * 0.35;
    } else {
        p *= 1.0 + 0.025 * sin(uTime * 1.3 + aInst.x * 6.2832);
    }
    vec4 mv = modelViewMatrix * instanceMatrix * vec4(p, 1.0);
    vViewPos = mv.xyz;
    vNormal = normalize(normalMatrix * mat3(instanceMatrix) * normal);
    vUv = uv;
    vAnim = aAnim;
    vTint = aTint;
    vFade = aInst.w;
    vPulse = k;
    gl_Position = projectionMatrix * mv;
}`;

const JELLY_FRAGMENT = /* glsl */`
uniform float uTime;
uniform float uPart;
uniform vec3 uBase;
uniform vec3 uGlow;
uniform float uFogDensity;
varying vec3 vViewPos;
varying vec3 vNormal;
varying vec2 vUv;
varying vec2 vAnim;
varying vec3 vTint;
varying float vFade;
varying float vPulse;

vec3 hue(float h) {
    return clamp(abs(fract(h + vec3(0.0, 0.6667, 0.3333)) * 6.0 - 3.0) - 1.0, 0.0, 1.0);
}

void main() {
    vec3 V = normalize(-vViewPos);
    vec3 N = normalize(vNormal);
    float facing = abs(dot(N, V));
    float fresnel = pow(clamp(1.0 - facing, 0.0, 1.0), 2.2);
    float along = vUv.x;
    float phase = vUv.y;
    vec3 base = uBase * vTint;
    vec3 glow = uGlow * vTint;
    vec3 col;
    float alpha;
    if (uPart < 0.5) {
        float rim = exp(-(1.0 - along) * 14.0);
        float dome = exp(-along * 3.0) * 0.35;
        float veins = pow(clamp(0.5 + 0.5 * cos(vAnim.x * 6.2832 * 16.0), 0.0, 1.0), 12.0) * smoothstep(0.2, 0.9, along) * 0.35;
        col = base * (0.035 + 0.5 * fresnel + dome * 0.55 + veins) + glow * rim * (0.35 + 0.5 * vPulse);
        alpha = 1.0;
    } else if (uPart < 1.5) {
        col = glow * (0.32 + 0.25 * fresnel) * (0.8 + 0.4 * vPulse);
        alpha = 1.0;
    } else if (uPart < 2.5) {
        float wave = exp(-pow((fract(along * 1.4 - uTime * 0.22 + phase) - 0.5) * 5.0, 2.0));
        col = base * (0.05 + 0.32 * fresnel) + glow * wave * 0.3;
        alpha = 0.8 * (1.0 - along * 0.5);
    } else if (uPart < 3.5) {
        float beads = smoothstep(0.86, 1.0, sin(along * 34.0 - uTime * 2.6 + phase * 40.0));
        float travel = exp(-pow((fract(along * 0.8 - uTime * 0.35 + phase) - 0.5) * 7.0, 2.0));
        col = base * 0.1 + glow * (0.07 + beads * 0.45 + travel * 0.7);
        alpha = (1.0 - along * 0.6);
    } else if (uPart < 4.5) {
        col = base * (0.03 + 0.45 * fresnel);
        alpha = 1.0;
    } else {
        float wave = fract(along * 2.2 - uTime * 1.4 + phase * 3.0);
        float flicker = 0.55 + 0.45 * sin(uTime * 9.0 + along * 60.0 + phase * 20.0);
        col = hue(wave + phase * 0.5) * (1.2 + 1.6 * flicker) * smoothstep(0.0, 0.1, along) * smoothstep(1.0, 0.9, along);
        alpha = 1.0;
    }
    float fog = exp(-uFogDensity * uFogDensity * dot(vViewPos, vViewPos));
    float nearFade = smoothstep(1.5, 7.0, -vViewPos.z);
    gl_FragColor = vec4(col * alpha * fog * vFade * nearFade, 1.0);
}`;

const MANTA_VERTEX = /* glsl */`
attribute vec2 aAnim;
uniform float uTime;
uniform float uFlap;
varying vec3 vViewPos;
varying vec3 vNormal;
varying vec3 vWorldNormal;
varying float vBelly;
varying float vSpan;
varying vec3 vObj;
void main() {
    vec3 p = position;
    vObj = position;
    float span = clamp(uv.x, 0.0, 1.0);
    float wave = sin(uTime * uFlap - span * 1.4);
    p.y += wave * pow(span, 1.6) * 0.42;
    p.z += cos(uTime * uFlap - span * 1.4) * pow(span, 2.0) * 0.08;
    vec4 world = modelMatrix * vec4(p, 1.0);
    vec4 mv = viewMatrix * world;
    vViewPos = mv.xyz;
    vNormal = normalize(normalMatrix * normal);
    vWorldNormal = normalize(mat3(modelMatrix) * normal);
    vBelly = aAnim.y;
    vSpan = span;
    gl_Position = projectionMatrix * mv;
}`;

const MANTA_FRAGMENT = /* glsl */`
uniform vec3 uFogColor;
uniform float uFogDensity;
uniform vec3 uLightDir;
varying vec3 vViewPos;
varying vec3 vNormal;
varying vec3 vWorldNormal;
varying float vBelly;
varying float vSpan;
varying vec3 vObj;
void main() {
    vec3 N = normalize(vWorldNormal);
    vec3 V = normalize(-vViewPos);
    float light = clamp(dot(N, uLightDir), 0.0, 1.0);
    float sky = clamp(N.y * 0.5 + 0.5, 0.0, 1.0);
    vec3 top = mix(vec3(0.006, 0.012, 0.02), vec3(0.03, 0.05, 0.07), vSpan * 0.5);
    float spots = smoothstep(0.72, 0.9, sin(vObj.x * 13.1) * sin(vObj.z * 11.7 + vObj.x * 3.1)) * (1.0 - smoothstep(0.2, 0.6, vSpan));
    vec3 belly = mix(vec3(0.075, 0.09, 0.105), vec3(0.018, 0.028, 0.038), smoothstep(0.45, 1.0, vSpan));
    belly *= 1.0 - 0.7 * spots;
    vec3 albedo = mix(top, belly, step(0.5, vBelly));
    vec3 ambient = mix(vec3(0.02, 0.05, 0.07), vec3(0.18, 0.35, 0.45), sky);
    float rim = pow(clamp(1.0 - abs(dot(normalize(vNormal), V)), 0.0, 1.0), 4.0);
    vec3 col = albedo * (ambient * 0.9 + vec3(0.6, 0.85, 1.0) * light * 1.2) + vec3(0.12, 0.32, 0.45) * rim * 0.35;
    float fog = 1.0 - exp(-uFogDensity * uFogDensity * dot(vViewPos, vViewPos));
    gl_FragColor = vec4(mix(col, uFogColor, fog), 1.0);
}`;

const RAY_VERTEX = /* glsl */`
varying vec2 vUv;
varying vec3 vViewPos;
varying vec3 vNormal;
void main() {
    vUv = uv;
    vec4 mv = modelViewMatrix * vec4(position, 1.0);
    vViewPos = mv.xyz;
    vNormal = normalize(normalMatrix * normal);
    gl_Position = projectionMatrix * mv;
}`;

const RAY_FRAGMENT = /* glsl */`
uniform float uTime;
uniform float uSeed;
uniform float uStrength;
uniform float uFogDensity;
varying vec2 vUv;
varying vec3 vViewPos;
varying vec3 vNormal;
void main() {
    float facing = abs(dot(normalize(vNormal), normalize(-vViewPos)));
    float core = pow(clamp(facing, 0.0, 1.0), 2.5);
    float streak = 0.55 + 0.45 * sin(vUv.x * 6.2832 * 5.0 + uSeed * 10.0 + uTime * 0.25) * sin(vUv.x * 6.2832 * 3.0 - uTime * 0.17 + uSeed);
    float lengthFade = smoothstep(0.0, 0.7, vUv.y) * smoothstep(1.0, 0.92, vUv.y);
    float flicker = 0.75 + 0.25 * sin(uTime * 0.6 + uSeed * 7.0);
    float fog = exp(-uFogDensity * uFogDensity * dot(vViewPos, vViewPos) * 0.35);
    vec3 col = vec3(0.3, 0.62, 0.8) * core * streak * lengthFade * flicker * uStrength * fog;
    gl_FragColor = vec4(col, 1.0);
}`;

const SKY_FRAGMENT = /* glsl */`
uniform vec3 uFogColor;
uniform vec3 uCameraPos;
uniform float uSurfaceY;
uniform float uTime;
varying vec3 vWorldDir;
float ripple(vec2 p) {
    float v = sin(p.x * 0.9 + uTime * 0.6) * sin(p.y * 0.8 - uTime * 0.45);
    v += 0.6 * sin(p.x * 1.9 - p.y * 1.3 + uTime * 0.9);
    v += 0.35 * sin(p.x * 3.7 + p.y * 2.9 - uTime * 1.3);
    v += 0.2 * sin(p.x * 7.1 - p.y * 5.3 + uTime * 1.9);
    return v;
}
void main() {
    vec3 dir = normalize(vWorldDir);
    vec3 below = vec3(0.0004, 0.0016, 0.0035);
    vec3 col = mix(below, uFogColor, smoothstep(-0.7, 0.1, dir.y));
    if (dir.y > 0.02) {
        float dist = (uSurfaceY - uCameraPos.y) / dir.y;
        vec2 hit = uCameraPos.xz + dir.xz * dist;
        float r = ripple(hit * 0.32);
        float caust = pow(clamp(1.0 - abs(r) * 0.55, 0.0, 1.0), 7.0);
        float window = smoothstep(0.62, 0.8, dir.y);
        vec3 surfaceDeep = vec3(0.012, 0.06, 0.1) * (0.7 + 0.5 * caust);
        vec3 surfaceBright = vec3(0.3, 0.66, 0.86) * (0.45 + 1.0 * caust);
        vec3 surface = mix(surfaceDeep, surfaceBright, window);
        float fog = exp(-dist * 0.034);
        col = mix(col, surface, fog);
    }
    gl_FragColor = vec4(col, 1.0);
}`;

const SNOW_VERTEX = /* glsl */`
attribute vec4 aSeed;
uniform float uTime;
uniform vec3 uCameraPos;
uniform float uBox;
uniform float uPointScale;
uniform float uSparkle;
varying float vAlpha;
varying vec3 vColor;
void main() {
    vec3 p = position;
    p.y -= uTime * (0.08 + 0.2 * aSeed.x);
    p.x += sin(uTime * 0.21 + aSeed.y * 30.0) * 0.6;
    p.z += cos(uTime * 0.17 + aSeed.z * 30.0) * 0.6;
    vec3 rel = mod(p - uCameraPos + uBox * 0.5, uBox) - uBox * 0.5;
    vec4 mv = viewMatrix * vec4(uCameraPos + rel, 1.0);
    float depth = -mv.z;
    float size = uPointScale * (0.5 + aSeed.w * 1.4) * (1.0 + uSparkle * 1.2) / max(depth, 0.1);
    gl_PointSize = clamp(size, 1.0, 22.0);
    float near = smoothstep(0.4, 2.5, depth);
    float far = exp(-depth * 0.06);
    float twinkle = uSparkle > 0.5 ? (0.35 + 0.65 * pow(0.5 + 0.5 * sin(uTime * (1.5 + aSeed.x * 3.0) + aSeed.y * 40.0), 4.0)) : 1.0;
    vAlpha = near * far * twinkle / max(1.0, size * size / 16.0);
    vColor = uSparkle > 0.5 ? mix(vec3(0.2, 1.0, 0.85), vec3(0.4, 0.6, 1.0), aSeed.z) * 2.2 : vec3(0.45, 0.6, 0.7) * 0.35;
    gl_Position = projectionMatrix * mv;
}`;

const SNOW_FRAGMENT = /* glsl */`
varying float vAlpha;
varying vec3 vColor;
void main() {
    vec2 c = gl_PointCoord * 2.0 - 1.0;
    float r2 = dot(c, c);
    if (r2 > 1.0) discard;
    gl_FragColor = vec4(vColor * vAlpha * exp(-r2 * 2.5), 1.0);
}`;

const GRADE_SHADER = {
    uniforms: {
        tDiffuse: { value: null },
        uTime: { value: 0 },
        uAberration: { value: 0.0035 },
        uResolution: { value: new THREE.Vector2(1, 1) }
    },
    vertexShader: /* glsl */`
varying vec2 vUv;
void main() { vUv = uv; gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }`,
    fragmentShader: /* glsl */`
uniform sampler2D tDiffuse;
uniform float uTime;
uniform float uAberration;
uniform vec2 uResolution;
varying vec2 vUv;
float hash(vec2 p) { return fract(sin(dot(p, vec2(12.9898, 78.233))) * 43758.5453); }
void main() {
    vec2 c = vUv - 0.5;
    vec2 off = c * uAberration;
    vec3 col;
    col.r = texture2D(tDiffuse, vUv + off).r;
    col.g = texture2D(tDiffuse, vUv).g;
    col.b = texture2D(tDiffuse, vUv - off).b;
    col = mix(col, col * vec3(0.86, 1.0, 1.08), 0.5);
    float aspect = uResolution.x / uResolution.y;
    float vig = smoothstep(1.2, 0.25, length(c * vec2(aspect * 0.85, 1.0)));
    col *= mix(0.45, 1.0, vig);
    col += (hash(vUv * uResolution + fract(uTime) * 91.0) - 0.5) * 0.012;
    gl_FragColor = vec4(max(col, vec3(0.0)), 1.0);
}`
};

function rng(seed) {
    let a = seed >>> 0;
    return () => {
        a = (a + 0x6d2b79f5) >>> 0;
        let t = a;
        t = Math.imul(t ^ (t >>> 15), t | 1);
        t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
        return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
}

function contraction(c) {
    if (c < 0.22) {
        const x = c / 0.22;
        return x * x * (3 - 2 * x);
    }
    const x = (c - 0.22) / 0.78;
    return 1 - x * x * (3 - 2 * x);
}

function partMaterial(name, species, uniforms) {
    const colors = {
        moon: { base: [0.5, 0.72, 1.0], glow: [0.75, 0.55, 1.0] },
        lion: { base: [1.0, 0.55, 0.3], glow: [1.0, 0.35, 0.18] },
        comb: { base: [0.55, 0.8, 1.0], glow: [0.6, 0.9, 1.0] }
    }[species];
    const part = PART[name] == null ? PART.bell : PART[name];
    return new THREE.ShaderMaterial({
        uniforms: {
            uTime: uniforms.uTime,
            uFogDensity: uniforms.uFogDensity,
            uPart: { value: part },
            uBase: { value: new THREE.Color().fromArray(colors.base) },
            uGlow: { value: new THREE.Color().fromArray(name === 'gonad' ? [1.0, 0.45, 0.85] : colors.glow) }
        },
        vertexShader: JELLY_VERTEX,
        fragmentShader: JELLY_FRAGMENT,
        transparent: true,
        depthWrite: false,
        blending: THREE.AdditiveBlending,
        side: THREE.DoubleSide
    });
}

function meshesByMaterial(gltf) {
    const parts = [];
    gltf.scene.traverse(obj => {
        if (!obj.isMesh) return;
        const geometry = obj.geometry;
        if (geometry.getAttribute('uv1')) geometry.setAttribute('aAnim', geometry.getAttribute('uv1'));
        parts.push({ name: obj.material && obj.material.name || 'bell', geometry });
        if (obj.material && obj.material.dispose) obj.material.dispose();
    });
    return parts;
}

export async function createAbyss(env) {
    const quality = QUALITY[env.quality] ? env.quality : 'high';
    const q = QUALITY[quality];
    const random = rng((Math.random() * 0xffffffff) >>> 0);
    const url = path => (env.versionedURL ? env.versionedURL(path) : path);
    const loader = new GLTFLoader();
    const [moonGltf, lionGltf, combGltf, mantaGltf] = await Promise.all(
        ['jelly-moon.glb', 'jelly-lion.glb', 'jelly-comb.glb', 'manta.glb'].map(file => loader.loadAsync(url(MODEL_BASE + file)))
    );
    if (env.isCurrent && !env.isCurrent()) {
        [moonGltf, lionGltf, combGltf, mantaGltf].forEach(g => g.scene.traverse(o => o.geometry && o.geometry.dispose()));
        return { frame() {}, resize() {}, dispose() {} };
    }

    const renderer = new THREE.WebGLRenderer({ canvas: env.canvas, antialias: false, alpha: false, powerPreference: 'high-performance', stencil: false });
    renderer.setPixelRatio(1);
    renderer.setSize(env.width, env.height, false);
    renderer.toneMapping = THREE.ACESFilmicToneMapping;
    renderer.toneMappingExposure = 1.05;
    renderer.outputColorSpace = THREE.SRGBColorSpace;
    renderer.info.autoReset = false;

    const scene = new THREE.Scene();
    const camera = new THREE.PerspectiveCamera(52, env.width / env.height, 0.1, 400);
    const owned = [];
    const shared = { uTime: { value: 0 }, uFogDensity: { value: FOG_DENSITY } };

    const sky = new THREE.Mesh(new THREE.SphereGeometry(300, 32, 16), new THREE.ShaderMaterial({
        uniforms: { uFogColor: { value: FOG_COLOR }, uCameraPos: { value: new THREE.Vector3() }, uSurfaceY: { value: SURFACE_Y }, uTime: shared.uTime },
        vertexShader: 'varying vec3 vWorldDir; void main(){ vWorldDir = position; gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }',
        fragmentShader: SKY_FRAGMENT,
        side: THREE.BackSide,
        depthWrite: false
    }));
    sky.renderOrder = -10;
    scene.add(sky);
    owned.push(sky.geometry, sky.material);

    const lightDir = new THREE.Vector3(0.25, 1, 0.12).normalize();
    const rayGroup = new THREE.Group();
    for (let i = 0; i < q.rays; i++) {
        const height = 55 + random() * 15;
        const geometry = new THREE.CylinderGeometry(0.25 + random() * 0.35, 1.2 + random() * 1.8, height, 24, 1, true);
        const material = new THREE.ShaderMaterial({
            uniforms: { uTime: shared.uTime, uSeed: { value: random() * 10 }, uStrength: { value: 0.045 + random() * 0.06 }, uFogDensity: shared.uFogDensity },
            vertexShader: RAY_VERTEX,
            fragmentShader: RAY_FRAGMENT,
            transparent: true,
            depthWrite: false,
            blending: THREE.AdditiveBlending,
            side: THREE.DoubleSide
        });
        const ray = new THREE.Mesh(geometry, material);
        const angle = random() * Math.PI * 2;
        const radius = random() * 11;
        ray.position.set(Math.cos(angle) * radius, SURFACE_Y - height / 2, Math.sin(angle) * radius);
        ray.rotation.set(0.18 + random() * 0.06, 0, -0.22 + random() * 0.08);
        ray.userData.sway = random() * Math.PI * 2;
        rayGroup.add(ray);
        owned.push(geometry, material);
    }
    scene.add(rayGroup);

    function makeSnow(count, sparkle) {
        const positions = new Float32Array(count * 3);
        const seeds = new Float32Array(count * 4);
        const box = 46;
        for (let i = 0; i < count; i++) {
            positions[i * 3] = (random() - 0.5) * box;
            positions[i * 3 + 1] = (random() - 0.5) * box;
            positions[i * 3 + 2] = (random() - 0.5) * box;
            seeds.set([random(), random(), random(), random()], i * 4);
        }
        const geometry = new THREE.BufferGeometry();
        geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));
        geometry.setAttribute('aSeed', new THREE.BufferAttribute(seeds, 4));
        const material = new THREE.ShaderMaterial({
            uniforms: {
                uTime: shared.uTime,
                uCameraPos: { value: new THREE.Vector3() },
                uBox: { value: box },
                uPointScale: { value: 1 },
                uSparkle: { value: sparkle ? 1 : 0 }
            },
            vertexShader: SNOW_VERTEX,
            fragmentShader: SNOW_FRAGMENT,
            transparent: true,
            depthWrite: false,
            blending: THREE.AdditiveBlending
        });
        const points = new THREE.Points(geometry, material);
        points.frustumCulled = false;
        scene.add(points);
        owned.push(geometry, material);
        return points;
    }
    const snow = makeSnow(q.snow, false);
    const sparkles = makeSnow(q.sparkles, true);

    const species = [];
    function addSpecies(key, gltf, count, scaleRange, pulseRange, tint) {
        const parts = meshesByMaterial(gltf);
        const inst = new THREE.InstancedBufferAttribute(new Float32Array(count * 4), 4);
        const tints = new THREE.InstancedBufferAttribute(new Float32Array(count * 3), 3);
        inst.setUsage(THREE.DynamicDrawUsage);
        const meshes = parts.map(part => {
            part.geometry.setAttribute('aInst', inst);
            part.geometry.setAttribute('aTint', tints);
            const material = partMaterial(part.name, key, shared);
            const mesh = new THREE.InstancedMesh(part.geometry, material, count);
            mesh.frustumCulled = false;
            mesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
            scene.add(mesh);
            owned.push(part.geometry, material);
            return mesh;
        });
        const jellies = [];
        for (let i = 0; i < count; i++) {
            const angle = random() * Math.PI * 2;
            const radius = Math.sqrt(random()) * SWARM_RADIUS;
            const t = tint(random);
            tints.setXYZ(i, t[0], t[1], t[2]);
            jellies.push({
                pos: new THREE.Vector3(Math.cos(angle) * radius, SWARM_BOTTOM + random() * (SWARM_TOP - SWARM_BOTTOM), Math.sin(angle) * radius),
                scale: scaleRange[0] + random() * (scaleRange[1] - scaleRange[0]),
                phase: random(),
                freq: pulseRange[0] + random() * (pulseRange[1] - pulseRange[0]),
                tiltPhase: random() * Math.PI * 2,
                drift: random() * Math.PI * 2,
                speed: 0
            });
        }
        tints.needsUpdate = true;
        species.push({ key, meshes, jellies, inst });
    }
    addSpecies('moon', moonGltf, q.moon, [0.8, 1.5], [0.32, 0.5], r => [0.8 + r() * 0.4, 0.85 + r() * 0.3, 1.0]);
    addSpecies('lion', lionGltf, q.lion, [1.4, 2.1], [0.2, 0.3], r => [1.0, 0.8 + r() * 0.3, 0.7 + r() * 0.3]);
    addSpecies('comb', combGltf, q.comb, [0.6, 1.0], [0.25, 0.4], r => [0.8 + r() * 0.3, 0.9, 1.0]);

    const mantaMaterial = new THREE.ShaderMaterial({
        uniforms: { uTime: shared.uTime, uFlap: { value: 1.2 }, uFogColor: { value: FOG_COLOR }, uFogDensity: shared.uFogDensity, uLightDir: { value: lightDir } },
        vertexShader: MANTA_VERTEX,
        fragmentShader: MANTA_FRAGMENT,
        side: THREE.DoubleSide
    });
    const manta = new THREE.Group();
    meshesByMaterial(mantaGltf).forEach(part => {
        const mesh = new THREE.Mesh(part.geometry, mantaMaterial);
        mesh.frustumCulled = false;
        mesh.rotation.y = Math.PI;
        manta.add(mesh);
        owned.push(part.geometry);
    });
    owned.push(mantaMaterial);
    manta.scale.setScalar(3.0);
    scene.add(manta);
    const mantaPath = { phase: random() * Math.PI * 2, radius: 11 + random() * 3, height: 8 + random() * 2 };

    const composer = new EffectComposer(renderer);
    composer.setPixelRatio(1);
    const renderPass = new RenderPass(scene, camera);
    const bloom = new UnrealBloomPass(new THREE.Vector2(env.width * q.bloomScale, env.height * q.bloomScale), 0.75, 0.55, 0.32);
    const grade = new ShaderPass(GRADE_SHADER);
    const output = new OutputPass();
    composer.addPass(renderPass);
    composer.addPass(bloom);
    composer.addPass(grade);
    composer.addPass(output);

    let width = env.width;
    let height = env.height;
    function resize(w, h) {
        width = w;
        height = h;
        renderer.setSize(w, h, false);
        composer.setSize(w, h);
        bloom.resolution.set(w * q.bloomScale, h * q.bloomScale);
        camera.aspect = w / h;
        camera.updateProjectionMatrix();
        grade.uniforms.uResolution.value.set(w, h);
        const pointScale = h / 1080 * 42;
        snow.material.uniforms.uPointScale.value = pointScale;
        sparkles.material.uniforms.uPointScale.value = pointScale;
    }
    resize(width, height);

    const matrix = new THREE.Matrix4();
    const quaternion = new THREE.Quaternion();
    const euler = new THREE.Euler();
    const scaleVec = new THREE.Vector3();
    const lookTarget = new THREE.Vector3();
    const mantaPos = new THREE.Vector3();
    const mantaAhead = new THREE.Vector3();

    function updateJellies(t, dt) {
        species.forEach(group => {
            group.jellies.forEach((j, i) => {
                const k = contraction((t * j.freq + j.phase) % 1);
                j.speed += ((0.25 + k * 1.3) * (0.7 + j.scale * 0.3) - j.speed) * Math.min(1, dt * 2.5);
                const tiltX = Math.sin(t * 0.07 + j.tiltPhase) * 0.28;
                const tiltZ = Math.cos(t * 0.05 + j.tiltPhase * 1.3) * 0.28;
                euler.set(tiltX, j.tiltPhase + t * 0.03, tiltZ);
                quaternion.setFromEuler(euler);
                const upX = Math.sin(tiltZ) * -1;
                const upZ = Math.sin(tiltX);
                j.pos.x += (upX * j.speed + Math.sin(t * 0.05 + j.drift) * 0.15) * dt;
                j.pos.y += j.speed * dt * 0.8;
                j.pos.z += (upZ * j.speed + Math.cos(t * 0.04 + j.drift) * 0.15) * dt;
                if (j.pos.y > SWARM_TOP) {
                    j.pos.y = SWARM_BOTTOM;
                    const angle = random() * Math.PI * 2;
                    const radius = Math.sqrt(random()) * SWARM_RADIUS;
                    j.pos.x = Math.cos(angle) * radius;
                    j.pos.z = Math.sin(angle) * radius;
                }
                const horizontal = Math.hypot(j.pos.x, j.pos.z);
                if (horizontal > SWARM_RADIUS * 1.3) {
                    j.pos.x *= SWARM_RADIUS / horizontal;
                    j.pos.z *= SWARM_RADIUS / horizontal;
                }
                const fade = Math.min(1, (SWARM_TOP - j.pos.y) / 3) * Math.min(1, (j.pos.y - SWARM_BOTTOM) / 3);
                scaleVec.setScalar(j.scale);
                matrix.compose(j.pos, quaternion, scaleVec);
                group.meshes.forEach(mesh => mesh.setMatrixAt(i, matrix));
                group.inst.setXYZW(i, j.phase, j.freq, Math.min(1, Math.max(0, (j.speed - 0.3) / 1.2)), Math.max(0, fade));
            });
            group.meshes.forEach(mesh => { mesh.instanceMatrix.needsUpdate = true; });
            group.inst.needsUpdate = true;
        });
    }

    function updateManta(t) {
        const a = mantaPath.phase + t * 0.055;
        mantaPos.set(Math.cos(a) * mantaPath.radius, mantaPath.height + Math.sin(t * 0.08) * 2, Math.sin(a) * mantaPath.radius * 0.8);
        const b = a + 0.02;
        mantaAhead.set(Math.cos(b) * mantaPath.radius, mantaPath.height + Math.sin((t + 0.4) * 0.08) * 2, Math.sin(b) * mantaPath.radius * 0.8);
        manta.position.copy(mantaPos);
        manta.lookAt(mantaAhead);
        manta.rotateZ(-0.35);
        manta.rotateX(0.08 * Math.sin(t * 0.3));
    }

    function frame(t, dt) {
        shared.uTime.value = t;
        renderer.info.reset();
        const orbit = t * 0.014 + 0.6;
        const radius = 21 + 3 * Math.sin(t * 0.021);
        camera.position.set(Math.cos(orbit) * radius, -4 + 3 * Math.sin(t * 0.017), Math.sin(orbit) * radius);
        lookTarget.set(Math.sin(t * 0.031) * 2, 1.5 + 2.5 * Math.sin(t * 0.023), Math.cos(t * 0.027) * 2);
        camera.lookAt(lookTarget);
        camera.rotateZ(0.04 * Math.sin(t * 0.05));
        sky.material.uniforms.uCameraPos.value.copy(camera.position);
        snow.material.uniforms.uCameraPos.value.copy(camera.position);
        sparkles.material.uniforms.uCameraPos.value.copy(camera.position);
        sky.position.copy(camera.position);
        rayGroup.children.forEach(ray => {
            ray.rotation.y = Math.sin(t * 0.03 + ray.userData.sway) * 0.05;
        });
        updateJellies(t, Math.min(dt, 0.1));
        updateManta(t);
        grade.uniforms.uTime.value = t;
        composer.render(dt);
    }

    function dispose() {
        owned.forEach(item => item && item.dispose && item.dispose());
        bloom.dispose();
        grade.dispose();
        output.dispose();
        composer.dispose();
        renderer.dispose();
        renderer.forceContextLoss();
    }

    return {
        frame,
        resize,
        stats() {
            return { quality, jellies: species.reduce((n, s) => n + s.jellies.length, 0), triangles: renderer.info.render.triangles, calls: renderer.info.render.calls };
        },
        dispose
    };
}
