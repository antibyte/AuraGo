import * as THREE from 'three';

const NOISE = `
float golfHash(vec2 p) { p = fract(p * vec2(123.34, 456.21)); p += dot(p, p + 45.32); return fract(p.x * p.y); }
float golfNoise(vec2 p) {
    vec2 i = floor(p), f = fract(p), u = f * f * (3.0 - 2.0 * f);
    return mix(mix(golfHash(i), golfHash(i + vec2(1.0, 0.0)), u.x), mix(golfHash(i + vec2(0.0, 1.0)), golfHash(i + vec2(1.0, 1.0)), u.x), u.y);
}
float golfFbm(vec2 p) { return golfNoise(p) * 0.62 + golfNoise(p * 2.03 + 7.1) * 0.38; }
float golfHash3(vec3 p) { p = fract(p * 0.3183099 + 0.1); p *= 17.0; return fract(p.x * p.y * p.z * (p.x + p.y + p.z)); }
float golfNoise3(vec3 x) {
    vec3 i = floor(x), f = fract(x); f = f * f * (3.0 - 2.0 * f);
    return mix(mix(mix(golfHash3(i), golfHash3(i + vec3(1, 0, 0)), f.x), mix(golfHash3(i + vec3(0, 1, 0)), golfHash3(i + vec3(1, 1, 0)), f.x), f.y),
               mix(mix(golfHash3(i + vec3(0, 0, 1)), golfHash3(i + vec3(1, 0, 1)), f.x), mix(golfHash3(i + vec3(0, 1, 1)), golfHash3(i + vec3(1, 1, 1)), f.x), f.y), f.z);
}
float golfFbm3(vec3 p) { return golfNoise3(p) * 0.55 + golfNoise3(p * 2.1 + 3.7) * 0.3 + golfNoise3(p * 4.3 + 9.2) * 0.15; }
`;

const DOME_VERT = `varying vec3 vGolfDir;
void main() { vGolfDir = position; gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }`;

const DOME_FRAG = `uniform vec3 zenith; uniform vec3 horizon; uniform vec3 below; uniform vec3 sunColor; uniform vec3 sunDir;
uniform vec3 cloudColor; uniform vec3 silhouetteColor;
uniform float clouds; uniform float cloudOn; uniform float haze; uniform float silhouette; uniform float band; uniform float golfTime;
varying vec3 vGolfDir;
${NOISE}
void main() {
    vec3 d = normalize(vGolfDir);
    float h = d.y;
    vec3 color = h >= 0.0 ? mix(horizon, zenith, smoothstep(0.0, 0.55, h)) : mix(horizon, below, smoothstep(0.0, 0.25, -h));
    float s = max(dot(d, sunDir), 0.0);
    color += sunColor * (pow(s, 48.0) * 0.35 + pow(s, 900.0) * 1.6);
    if (haze > 0.0) color = mix(color, horizon * 1.05, haze * exp(-abs(h) * 9.0));
    if (silhouette > 0.0) {
        vec2 around = normalize(d.xz + vec2(1e-4));
        float ridge = 0.03 + 0.035 * golfFbm(around * 2.5 + 3.0) + 0.012 * golfFbm(around * 9.0 + 1.3);
        color = mix(color, silhouetteColor, silhouette * smoothstep(ridge + 0.004, ridge - 0.004, h) * step(-0.02, h));
    }
    if (clouds > 0.0 && cloudOn > 0.5 && h > 0.0) {
        vec2 uv = d.xz / (h + 0.12) * 0.55 + vec2(golfTime * 0.006, golfTime * 0.002);
        color = mix(color, cloudColor, clouds * smoothstep(0.52, 0.78, golfFbm(uv)) * smoothstep(0.0, 0.18, h));
    }
    if (band > 0.0) color += vec3(0.10, 0.11, 0.2) * band * exp(-pow(dot(d, normalize(vec3(0.3, 0.55, 0.78))) * 5.0, 2.0)) * (0.6 + 0.4 * golfFbm(d.xy * 6.0));
    gl_FragColor = vec4(color, 1.0);
    #include <tonemapping_fragment>
    #include <colorspace_fragment>
}`;

const STAR_VERT = `attribute float golfSize; attribute float golfPhase; uniform float golfTime; uniform float twinkle; varying float vGolfFade;
void main() {
    vGolfFade = 1.0 - twinkle * 0.45 * (0.5 + 0.5 * sin(golfTime * (1.3 + golfPhase) + golfPhase * 7.0));
    gl_PointSize = golfSize;
    gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
}`;
// `#include` must start its own line: three.js resolves includes with a line-anchored pattern.
const STAR_FRAG = `varying float vGolfFade;
void main() {
    vec2 c = gl_PointCoord - 0.5; float d = dot(c, c);
    if (d > 0.25) discard;
    gl_FragColor = vec4(vec3(0.95, 0.96, 1.0) * vGolfFade, smoothstep(0.25, 0.0, d));
    #include <colorspace_fragment>
}`;

const EARTH_VERT = `varying vec3 vGolfNormal; varying vec3 vGolfObject; varying vec3 vGolfView;
void main() {
    vec4 world = modelMatrix * vec4(position, 1.0);
    vGolfNormal = normalize(mat3(modelMatrix) * normal); vGolfObject = position; vGolfView = cameraPosition - world.xyz;
    gl_Position = projectionMatrix * viewMatrix * world;
}`;
const EARTH_FRAG = `uniform vec3 sunDir; uniform float golfTime; varying vec3 vGolfNormal; varying vec3 vGolfObject; varying vec3 vGolfView;
${NOISE}
void main() {
    vec3 n = normalize(vGolfNormal);
    float land = smoothstep(0.50, 0.56, golfFbm3(vGolfObject * 0.28 + 3.0));
    vec3 ground = mix(vec3(0.16, 0.45, 0.16), vec3(0.55, 0.47, 0.30), smoothstep(0.6, 0.8, golfFbm3(vGolfObject * 0.6)));
    vec3 color = mix(vec3(0.05, 0.22, 0.55), ground, land);
    color = mix(color, vec3(1.0), 0.85 * smoothstep(0.55, 0.75, golfFbm3(vGolfObject * 0.45 + vec3(golfTime * 0.01))));
    float lit = clamp(dot(n, sunDir) * 1.2 + 0.08, 0.0, 1.0);
    color *= lit;
    color += vec3(0.25, 0.5, 1.0) * pow(1.0 - max(dot(n, normalize(vGolfView)), 0.0), 3.0) * (0.35 + lit);
    gl_FragColor = vec4(color, 1.0);
    #include <colorspace_fragment>
}`;

function seeded(seed) {
    return () => { seed |= 0; seed = seed + 0x6D2B79F5 | 0; let t = Math.imul(seed ^ seed >>> 15, 1 | seed); t = t + Math.imul(t ^ t >>> 7, 61 | t) ^ t; return ((t ^ t >>> 14) >>> 0) / 4294967296; };
}

// Sky dome (and for moon: stars and the Earth) for one look; `state` shares golfTime with the scene.
export function createSky(look, cfg, state = { golfTime: { value: 0 } }, backdrop = true) {
    const group = new THREE.Group();
    group.name = 'aurago-sky';
    const sunDir = new THREE.Vector3().fromArray(look.sun.direction).normalize();
    const uniforms = {
        zenith: { value: new THREE.Color(look.sky.zenith) }, horizon: { value: new THREE.Color(look.sky.horizon) }, below: { value: new THREE.Color(look.sky.below) },
        sunColor: { value: new THREE.Color(look.sky.sun) }, sunDir: { value: sunDir }, cloudColor: { value: new THREE.Color(look.sky.cloudColor) },
        silhouetteColor: { value: new THREE.Color(look.sky.silhouetteColor) }, clouds: { value: look.sky.clouds }, cloudOn: { value: cfg.clouds ? 1 : 0 },
        haze: { value: look.sky.haze }, silhouette: { value: look.sky.silhouette }, band: { value: look.sky.stars ? .8 : 0 }, golfTime: state.golfTime
    };
    const dome = new THREE.Mesh(new THREE.SphereGeometry(150, 48, 24), new THREE.ShaderMaterial({ uniforms, vertexShader: DOME_VERT, fragmentShader: DOME_FRAG, side: THREE.BackSide, depthWrite: false }));
    dome.renderOrder = -10; dome.frustumCulled = false;
    group.add(dome);
    const disposables = [dome.geometry, dome.material];
    let starUniforms = null;
    if (backdrop && look.sky.stars) {
        const random = seeded(4242), count = 900, positions = new Float32Array(count * 3), sizes = new Float32Array(count), phases = new Float32Array(count);
        for (let i = 0; i < count; i++) {
            const y = random() * 2 - 1, theta = random() * Math.PI * 2, r = Math.sqrt(1 - y * y);
            positions.set([Math.cos(theta) * r * 140, y * 140, Math.sin(theta) * r * 140], i * 3);
            sizes[i] = 1.2 + random() * random() * 2.6; phases[i] = random() * 3;
        }
        const geometry = new THREE.BufferGeometry();
        geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));
        geometry.setAttribute('golfSize', new THREE.BufferAttribute(sizes, 1));
        geometry.setAttribute('golfPhase', new THREE.BufferAttribute(phases, 1));
        starUniforms = { golfTime: state.golfTime, twinkle: { value: cfg.twinkle ? 1 : 0 } };
        const stars = new THREE.Points(geometry, new THREE.ShaderMaterial({ uniforms: starUniforms, vertexShader: STAR_VERT, fragmentShader: STAR_FRAG, transparent: true, depthWrite: false }));
        stars.renderOrder = -9; stars.frustumCulled = false;
        group.add(stars); disposables.push(geometry, stars.material);
    }
    if (backdrop && look.sky.earth) {
        const earth = new THREE.Mesh(new THREE.SphereGeometry(10, 48, 32), new THREE.ShaderMaterial({ uniforms: { sunDir: { value: sunDir }, golfTime: state.golfTime }, vertexShader: EARTH_VERT, fragmentShader: EARTH_FRAG }));
        earth.position.set(-.55, .32, -.77).normalize().multiplyScalar(120);
        earth.renderOrder = -8;
        group.add(earth); disposables.push(earth.geometry, earth.material);
    }
    return {
        group,
        setTier(next) { uniforms.cloudOn.value = next.clouds ? 1 : 0; if (starUniforms) starUniforms.twinkle.value = next.twinkle ? 1 : 0; },
        dispose() { group.removeFromParent(); for (const item of disposables) item.dispose(); }
    };
}

// Prefiltered environment from the look's sky and a bright sun disc; built once per look.
export function createEnvironment(renderer, look) {
    const pmrem = new THREE.PMREMGenerator(renderer);
    const envScene = new THREE.Scene();
    const sky = createSky(look, { clouds: false, twinkle: false }, undefined, false);
    envScene.add(sky.group);
    const sun = new THREE.Mesh(new THREE.SphereGeometry(8, 16, 8), new THREE.MeshBasicMaterial({ color: new THREE.Color(look.sun.color).multiplyScalar(look.sun.intensity * 4) }));
    sun.position.fromArray(look.sun.direction).normalize().multiplyScalar(100);
    envScene.add(sun);
    const target = pmrem.fromScene(envScene, .035, .1, 400);
    sky.dispose(); sun.geometry.dispose(); sun.material.dispose(); pmrem.dispose();
    return target;
}
