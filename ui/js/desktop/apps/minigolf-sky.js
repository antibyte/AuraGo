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
uniform vec3 cloudColor; uniform vec3 silhouetteColor; uniform vec3 bandPole; uniform vec3 galaxyCore;
uniform float clouds; uniform float cloudOn; uniform float haze; uniform float silhouette; uniform float band; uniform float bandDetail; uniform float golfTime;
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
    // Milky Way (moon only): a broad galactic plane with a warm bulge, dark dust lanes and a faint nebula.
    // The noise and the nebula are medium/high only; low keeps a cheap smooth band.
    if (band > 0.0) {
        float lat = dot(d, bandPole);
        float broad = exp(-pow(lat * 2.4, 2.0));
        float coreBand = exp(-pow(lat * 5.4, 2.0));
        vec3 bandColor = vec3(0.10, 0.12, 0.23);
        float density = 0.22 * broad + 0.34 * coreBand;
        if (bandDetail > 0.5) {
            float dust = smoothstep(0.46, 0.80, golfFbm3(d * 8.2 + vec3(2.7, -1.3, 4.1)));
            float mist = golfFbm3(d * 3.4 + vec3(-3.1, 5.2, 1.4));
            bandColor = mix(vec3(0.025, 0.035, 0.10), vec3(0.17, 0.11, 0.28), mist);
            bandColor = mix(bandColor, vec3(0.33, 0.22, 0.16), smoothstep(0.62, 0.96, mist) * 0.3);
            density *= (0.35 + 0.65 * mist) * (1.0 - 0.55 * dust);
            float nebula = smoothstep(0.72, 0.98, golfFbm3(d * 2.1 + vec3(8.0, -4.0, 3.0))) * broad * smoothstep(-0.1, 0.45, h);
            color += mix(vec3(0.04, 0.10, 0.22), vec3(0.12, 0.06, 0.18), mist) * nebula * band * 0.5;
        }
        color += bandColor * density * band;
        color += vec3(0.30, 0.19, 0.12) * pow(max(dot(d, galaxyCore), 0.0), 14.0) * broad * 0.22 * band;
    }
    gl_FragColor = vec4(color, 1.0);
    #include <tonemapping_fragment>
    #include <colorspace_fragment>
}`;

const STAR_VERT = `attribute float golfSize; attribute float golfPhase; attribute float golfSpark; attribute vec3 golfColor;
uniform float golfTime; uniform float twinkle; uniform float sparkle;
varying float vGolfFade; varying vec3 vGolfColor; varying float vGolfSpark;
void main() {
    float wave = 0.5 + 0.5 * sin(golfTime * (1.3 + golfPhase) + golfPhase * 7.0);
    float flare = pow(0.5 + 0.5 * sin(golfTime * (0.21 + golfPhase * 0.17) + golfPhase * 11.0), 26.0);
    vGolfFade = (1.0 - twinkle * 0.45 * wave) * (1.0 + sparkle * flare * 1.7);
    vGolfColor = golfColor;
    vGolfSpark = golfSpark * sparkle;
    gl_PointSize = golfSize * (1.0 + sparkle * flare * 0.45);
    gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
}`;
// `#include` must start its own line: three.js resolves includes with a line-anchored pattern.
const STAR_FRAG = `varying float vGolfFade; varying vec3 vGolfColor; varying float vGolfSpark;
void main() {
    vec2 c = gl_PointCoord - 0.5; float d = dot(c, c);
    vec2 a = abs(c);
    float core = smoothstep(0.235, 0.0, d);
    float halo = smoothstep(0.25, 0.08, d) * 0.35;
    float horizontal = max(0.0, 1.0 - a.y * 26.0) * max(0.0, 1.0 - a.x * 2.5);
    float vertical = max(0.0, 1.0 - a.x * 26.0) * max(0.0, 1.0 - a.y * 2.5);
    float spark = (horizontal + vertical) * vGolfSpark * 0.85;
    float alpha = clamp(core + halo + spark, 0.0, 1.0) * vGolfFade;
    if (alpha < 0.004) discard;
    gl_FragColor = vec4(vGolfColor, alpha);
    #include <colorspace_fragment>
}`;

const METEOR_VERT = `attribute vec3 aStart; attribute vec3 aEnd;
attribute float aTrail; attribute float aPhase; attribute float aPeriod; attribute float aFlight; attribute float aBright;
uniform float golfTime; uniform float meteorOn;
varying float vFade; varying vec3 vColor;
void main() {
    float local = mod(golfTime + aPhase, aPeriod);
    float head = clamp(local / aFlight, 0.0, 1.0);
    float live = step(local, aFlight) * meteorOn;
    float onPath = step(aTrail * 0.10, head);
    vec3 dir = normalize(mix(aStart, aEnd, clamp(head - aTrail * 0.10, 0.0, 1.0)));
    if (live < 0.5) {
        gl_Position = vec4(2.0, 2.0, 2.0, 1.0);
        gl_PointSize = 1.0;
    } else {
        gl_Position = projectionMatrix * modelViewMatrix * vec4(dir * 138.0, 1.0);
        gl_PointSize = 4.6 - 2.6 * aTrail;
    }
    float fade = smoothstep(0.0, 0.12, head) * (1.0 - smoothstep(0.86, 1.0, head));
    vFade = live * onPath * aBright * fade * pow(1.0 - aTrail, 1.4);
    vColor = mix(vec3(0.66, 0.82, 1.0), vec3(1.0, 0.82, 0.55), aTrail);
}`;
const METEOR_FRAG = `varying float vFade; varying vec3 vColor;
void main() {
    vec2 c = gl_PointCoord - 0.5; float d = dot(c, c);
    if (d > 0.25) discard;
    float core = smoothstep(0.22, 0.0, d);
    float glow = smoothstep(0.25, 0.05, d) * 0.45;
    float alpha = (core + glow) * vFade;
    if (alpha < 0.008) discard;
    gl_FragColor = vec4(vColor, alpha);
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
    color = mix(color, vec3(0.82, 0.90, 1.0), smoothstep(0.74, 0.92, abs(n.y)) * 0.65);
    float lit = clamp(dot(n, sunDir) * 1.2 + 0.08, 0.0, 1.0);
    float night = pow(1.0 - lit, 4.0);
    float city = smoothstep(0.60, 0.74, golfFbm3(vGolfObject * 2.6 + 19.0)) * smoothstep(0.58, 0.78, golfNoise3(vGolfObject * 16.0));
    color += vec3(1.0, 0.74, 0.42) * city * land * night * 1.7;
    color *= lit + 0.03 * (1.0 - lit);
    vec3 viewDir = normalize(vGolfView);
    color += vec3(0.32, 0.55, 0.9) * pow(max(dot(n, normalize(sunDir + viewDir)), 0.0), 42.0) * (1.0 - land) * (0.25 + 0.75 * lit);
    color += vec3(0.25, 0.5, 1.0) * pow(1.0 - max(dot(n, viewDir), 0.0), 3.0) * (0.35 + lit);
    gl_FragColor = vec4(color, 1.0);
    #include <colorspace_fragment>
}`;

function seeded(seed) {
    return () => { seed |= 0; seed = seed + 0x6D2B79F5 | 0; let t = Math.imul(seed ^ seed >>> 15, 1 | seed); t = t + Math.imul(t ^ t >>> 7, 61 | t) ^ t; return ((t ^ t >>> 14) >>> 0) / 4294967296; };
}

const STAR_TINTS = Object.freeze([
    [0.86, 0.92, 1.00], [1.00, 0.97, 0.90], [0.70, 0.82, 1.00], [1.00, 0.85, 0.62], [0.88, 0.80, 1.00]
]);

// Star colours are seeded so every visit of a course shows the same sky.
function starField(look) {
    const random = seeded(4242), count = 1250;
    const pole = new THREE.Vector3().fromArray(look.sky.bandPole || [0, 1, 0]).normalize();
    const helper = Math.abs(pole.y) > 0.92 ? new THREE.Vector3(1, 0, 0) : new THREE.Vector3(0, 1, 0);
    const b1 = new THREE.Vector3().crossVectors(helper, pole).normalize();
    const b2 = new THREE.Vector3().crossVectors(pole, b1).normalize();
    const positions = new Float32Array(count * 3), sizes = new Float32Array(count);
    const phases = new Float32Array(count), sparks = new Float32Array(count), colors = new Float32Array(count * 3);
    let sparkCount = 0;
    for (let i = 0; i < count; i++) {
        let x, y, z;
        if (random() < 0.44) {
            // Two fifths of the stars gather along the galactic plane, so the band is dense instead of painted.
            const phi = random() * Math.PI * 2, lat = (random() + random() + random() - 1.5) * 0.26;
            const cl = Math.cos(lat), sl = Math.sin(lat), cp = Math.cos(phi), sp = Math.sin(phi);
            x = cl * (cp * b1.x + sp * b2.x) + sl * pole.x; y = cl * (cp * b1.y + sp * b2.y) + sl * pole.y; z = cl * (cp * b1.z + sp * b2.z) + sl * pole.z;
        } else {
            y = random() * 2 - 1;
            const theta = random() * Math.PI * 2, radius = Math.sqrt(1 - y * y);
            x = Math.cos(theta) * radius; z = Math.sin(theta) * radius;
        }
        const distance = 136 + random() * 8;
        positions.set([x * distance, y * distance, z * distance], i * 3);
        const spark = random() < 0.16;
        if (spark) sparkCount++;
        sizes[i] = spark ? 2.9 + random() * 2.4 : 0.9 + random() * random() * 2.1;
        phases[i] = random() * 3;
        sparks[i] = spark ? 0.55 + random() * 0.45 : 0;
        const tint = STAR_TINTS[Math.min(STAR_TINTS.length - 1, Math.floor(random() * STAR_TINTS.length))], brightness = 0.55 + random() * 0.45;
        colors.set([tint[0] * brightness, tint[1] * brightness, tint[2] * brightness], i * 3);
    }
    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));
    geometry.setAttribute('golfSize', new THREE.BufferAttribute(sizes, 1));
    geometry.setAttribute('golfPhase', new THREE.BufferAttribute(phases, 1));
    geometry.setAttribute('golfSpark', new THREE.BufferAttribute(sparks, 1));
    geometry.setAttribute('golfColor', new THREE.BufferAttribute(colors, 3));
    return { geometry, count, sparkCount };
}

// Shooting stars: one Points draw call, each trail is animated entirely on the GPU from the shared sky clock.
function meteorField(look, state) {
    const count = Math.max(0, Math.round(look.sky.meteors || 0)), trail = 48, total = count * trail;
    if (!count) return null;
    const random = seeded(90210);
    const positions = new Float32Array(total * 3), starts = new Float32Array(total * 3), ends = new Float32Array(total * 3);
    const trails = new Float32Array(total), phases = new Float32Array(total), periods = new Float32Array(total), flights = new Float32Array(total), brights = new Float32Array(total);
    const timing = [];
    let n = 0;
    for (let m = 0; m < count; m++) {
        const startY = .28 + random() * .48, startTheta = random() * Math.PI * 2;
        const startRadius = Math.sqrt(Math.max(0, 1 - startY * startY));
        const start = [Math.cos(startTheta) * startRadius, startY, Math.sin(startTheta) * startRadius];
        const endTheta = startTheta + (random() < .5 ? -1 : 1) * (.55 + random() * .85);
        const endY = Math.max(.04, startY - (.26 + random() * .5));
        const endRadius = Math.sqrt(Math.max(0, 1 - endY * endY));
        const end = [Math.cos(endTheta) * endRadius, endY, Math.sin(endTheta) * endRadius];
        const phase = random() * 20, period = 9 + random() * 10, flight = 1.0 + random() * 1.1, bright = .65 + random() * .35;
        timing.push({ phase, period, flight });
        for (let t = 0; t < trail; t++, n++) {
            positions.set(start, n * 3); starts.set(start, n * 3); ends.set(end, n * 3);
            trails[n] = t / (trail - 1); phases[n] = phase; periods[n] = period; flights[n] = flight; brights[n] = bright;
        }
    }
    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));
    geometry.setAttribute('aStart', new THREE.BufferAttribute(starts, 3));
    geometry.setAttribute('aEnd', new THREE.BufferAttribute(ends, 3));
    geometry.setAttribute('aTrail', new THREE.BufferAttribute(trails, 1));
    geometry.setAttribute('aPhase', new THREE.BufferAttribute(phases, 1));
    geometry.setAttribute('aPeriod', new THREE.BufferAttribute(periods, 1));
    geometry.setAttribute('aFlight', new THREE.BufferAttribute(flights, 1));
    geometry.setAttribute('aBright', new THREE.BufferAttribute(brights, 1));
    const uniforms = { golfTime: state.golfTime, meteorOn: { value: 1 } };
    const material = new THREE.ShaderMaterial({ uniforms, vertexShader: METEOR_VERT, fragmentShader: METEOR_FRAG, transparent: true, depthWrite: false, blending: THREE.AdditiveBlending });
    return { geometry, material, uniforms, count, timing };
}

// Sky dome (and for moon: stars, shooting stars and the Earth) for one look; `state` shares golfTime with the scene.
export function createSky(look, cfg, state = { golfTime: { value: 0 } }, backdrop = true) {
    const group = new THREE.Group();
    group.name = 'aurago-sky';
    const sunDir = new THREE.Vector3().fromArray(look.sun.direction).normalize();
    const bandPole = new THREE.Vector3().fromArray(look.sky.bandPole || [0, 1, 0]).normalize();
    const galaxyCore = new THREE.Vector3().fromArray(look.sky.galaxyCore || [0, 1, 0]).normalize();
    const uniforms = {
        zenith: { value: new THREE.Color(look.sky.zenith) }, horizon: { value: new THREE.Color(look.sky.horizon) }, below: { value: new THREE.Color(look.sky.below) },
        sunColor: { value: new THREE.Color(look.sky.sun) }, sunDir: { value: sunDir }, cloudColor: { value: new THREE.Color(look.sky.cloudColor) },
        silhouetteColor: { value: new THREE.Color(look.sky.silhouetteColor) }, clouds: { value: look.sky.clouds }, cloudOn: { value: cfg.clouds ? 1 : 0 },
        haze: { value: look.sky.haze }, silhouette: { value: look.sky.silhouette }, band: { value: look.sky.stars ? (look.sky.galaxy ?? .8) : 0 },
        bandDetail: { value: cfg.motion ? 1 : 0 }, bandPole: { value: bandPole }, galaxyCore: { value: galaxyCore }, golfTime: state.golfTime
    };
    const dome = new THREE.Mesh(new THREE.SphereGeometry(150, 48, 24), new THREE.ShaderMaterial({ uniforms, vertexShader: DOME_VERT, fragmentShader: DOME_FRAG, side: THREE.BackSide, depthWrite: false }));
    dome.renderOrder = -10; dome.frustumCulled = false;
    group.add(dome);
    const disposables = [dome.geometry, dome.material];
    let tierCfg = cfg, reducedMotion = false, starUniforms = null, field = null, meteor = null;
    if (backdrop && look.sky.stars) {
        field = starField(look);
        starUniforms = { golfTime: state.golfTime, twinkle: { value: cfg.twinkle ? 1 : 0 }, sparkle: { value: 0 } };
        const stars = new THREE.Points(field.geometry, new THREE.ShaderMaterial({ uniforms: starUniforms, vertexShader: STAR_VERT, fragmentShader: STAR_FRAG, transparent: true, depthWrite: false, blending: THREE.AdditiveBlending }));
        stars.renderOrder = -9; stars.frustumCulled = false;
        group.add(stars); disposables.push(field.geometry, stars.material);
        meteor = meteorField(look, state);
        if (meteor) {
            const shooting = new THREE.Points(meteor.geometry, meteor.material);
            shooting.renderOrder = -7; shooting.frustumCulled = false;
            group.add(shooting); disposables.push(meteor.geometry, meteor.material);
        }
    }
    if (backdrop && look.sky.earth) {
        const earth = new THREE.Mesh(new THREE.SphereGeometry(10, 48, 32), new THREE.ShaderMaterial({ uniforms: { sunDir: { value: sunDir }, golfTime: state.golfTime }, vertexShader: EARTH_VERT, fragmentShader: EARTH_FRAG }));
        earth.position.set(-.55, .32, -.77).normalize().multiplyScalar(120);
        earth.renderOrder = -8;
        group.add(earth); disposables.push(earth.geometry, earth.material);
    }
    function sync() {
        uniforms.cloudOn.value = tierCfg.clouds ? 1 : 0;
        uniforms.bandDetail.value = tierCfg.motion ? 1 : 0;
        if (starUniforms) {
            starUniforms.twinkle.value = tierCfg.twinkle ? 1 : 0;
            starUniforms.sparkle.value = tierCfg.twinkle ? 1 : 0;
        }
        if (meteor) meteor.uniforms.meteorOn.value = tierCfg.motion && !reducedMotion ? 1 : 0;
    }
    sync();
    return {
        group,
        setTier(next) { tierCfg = next; sync(); },
        setReduced(value) { reducedMotion = !!value; sync(); },
        stats() {
            const time = state.golfTime.value;
            return {
                stars: field?.count || 0, sparks: field?.sparkCount || 0,
                meteors: meteor?.count || 0,
                activeMeteors: meteor ? meteor.timing.filter(m => (((time + m.phase) % m.period) + m.period) % m.period < m.flight).length : 0,
                meteorOn: !!meteor && meteor.uniforms.meteorOn.value > 0.5,
                reduced: reducedMotion
            };
        },
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
