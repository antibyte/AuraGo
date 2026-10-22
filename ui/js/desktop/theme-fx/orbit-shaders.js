(function () {
    'use strict';

    // Orbit shaders (global AuraOrbitShaders). The GLSL sources of the space scene as plain strings; this file holds no
    // WebGL calls and no JavaScript logic, orbit-scene.js compiles them. Colours leave the shaders linear: the baked
    // textures live in sRGB render targets, everything else lands in the half-float HDR buffer of the post chain.
    //  - NOISE: integer-hash gradient noise (pcg3d) and fbm, shared by every procedural shader,
    //  - bake: the gas giant surface (bands, eddies, storms, night lights) and the nebula plate, drawn once into textures,
    //  - planet / atmosphere / ring / dust / moon: the lit bodies, with the planet and ring shadowing each other,
    //  - sky / stars / sun / flare / comet / exhaust: the backdrop, the sun and the short-lived things,
    //  - grade: vignette, chromatic aberration, film grain, shooting stars and the scanner pulse.
    // window.AuraOrbitShaders = { NOISE, ...<name>Vertex / <name>Fragment pairs }.

    const NOISE = `
const mat3 ROT3 = mat3(0.00, 0.80, 0.60, -0.80, 0.36, -0.48, -0.60, -0.48, 0.64);
// Noise comes from one 256x256 RGBA texture of random numbers (r: value, g: r shifted by (37, 17), b and a: more
// random numbers): a lattice value is r at (x + 37 z, y + 17 z), so one bilinear fetch yields two z layers. This keeps
// the shaders small; Direct3D compiled the same noise written as integer hashes in many seconds.
uniform sampler2D uNoise;
vec3 hash3(vec3 p) { return texelFetch(uNoise, ivec2(mod(p.xy + vec2(37.0, 17.0) * p.z, 256.0)), 0).rba; }
float hash13(vec3 p) { return hash3(p).y; }
vec3 hash33(vec3 p) { return hash3(p) * 2.0 - 1.0; }
float gnoise(vec3 x) {
    vec3 p = floor(x);
    vec3 f = x - p;
    f = f * f * f * (f * (f * 6.0 - 15.0) + 10.0);
    vec2 rg = texture2D(uNoise, (p.xy + vec2(37.0, 17.0) * p.z + f.xy + 0.5) / 256.0).rg;
    return mix(rg.x, rg.y, f.z) * 2.0 - 1.0;
}
float fbm(vec3 p, int octaves) {
    float sum = 0.0;
    float amp = 0.5;
    for (int i = 0; i < 8; i++) {
        if (i >= octaves) break;
        sum += amp * gnoise(p);
        p = ROT3 * p * 2.03 + vec3(11.3, 7.1, 3.7);
        amp *= 0.5;
    }
    return sum * 1.5;
}
const float PI = 3.14159265359;
const float TAU = 6.28318530718;
vec3 srgb(float r, float g, float b) { return pow(vec3(r, g, b) / 255.0, vec3(2.2)); }
vec2 raySphere(vec3 ro, vec3 rd, float r) {
    float b = dot(ro, rd);
    float h = b * b - (dot(ro, ro) - r * r);
    if (h < 0.0) return vec2(-1.0);
    h = sqrt(h);
    return vec2(-b - h, -b + h);
}
// Differential rotation of the cloud decks in rad/s at a latitude; the scene mirrors it to place the storm.
float zonalSpeed(float lat) { return 0.0046 + 0.0014 * sin(lat * 5.0 + 0.4) + 0.0007 * sin(lat * 13.0 + 1.9); }
`;

    const FULLSCREEN_VERTEX = `
varying vec2 vUv;
void main() {
    vUv = uv;
    gl_Position = vec4(position.xy, 0.0, 1.0);
}`;

    // ---------------------------------------------------------------------------------------------
    // Gas giant surface, baked once as an equirectangular plate (u: longitude, v: latitude). rgb is the linear albedo,
    // alpha the mask of the night-side lights.

    const PLANET_BAKE_FRAGMENT = `
precision highp float;
varying vec2 vUv;
uniform vec2 uStorm;
uniform vec2 uOval;
${NOISE}
float wrapPi(float a) { return a - TAU * floor((a + PI) / TAU); }
// Rotates the sample point around a storm centre; returns the displacement, core is 1 at the centre.
vec2 vortex(vec2 q, vec2 radii, float amount, out float core) {
    vec2 s = q / radii;
    float r2 = dot(s, s);
    core = exp(-r2);
    float a = amount * exp(-r2 * 0.55);
    float c = cos(a);
    float sn = sin(a);
    return vec2(c * q.x - sn * q.y, sn * q.x + c * q.y) - q;
}
float zonal(float l) {
    float z = 0.0;
    z += 0.50 * sin(l * 11.0 + 0.8);
    z += 0.30 * sin(l * 23.0 + 2.1);
    z += 0.20 * sin(l * 41.0 + 4.0);
    z += 0.12 * sin(l * 79.0 + 1.0);
    return z / 1.12;
}
void main() {
    float phi = vUv.x * TAU;
    float lat = (vUv.y - 0.5) * PI;
    float coreA;
    float coreB;
    vec2 qA = vec2(wrapPi(phi - uStorm.x * TAU) * cos(uStorm.y), lat - uStorm.y);
    vec2 qB = vec2(wrapPi(phi - uOval.x * TAU) * cos(uOval.y), lat - uOval.y);
    vec2 off = vortex(qA, vec2(0.46, 0.17), 3.8, coreA) + vortex(qB, vec2(0.12, 0.06), 3.2, coreB);
    float lat2 = clamp(lat + off.y, -1.55, 1.55);
    float phi2 = phi + off.x / max(cos(lat), 0.2);
    vec3 d = vec3(-cos(phi2) * cos(lat2), sin(lat2), sin(phi2) * cos(lat2));

    float warpA = fbm(d * vec3(2.2, 3.4, 2.2) + vec3(3.1, 1.7, 9.2), 4);
    float warpB = fbm(d * 6.0 + vec3(8.3, 2.8, 5.5), 4);
    float latW = lat2 + 0.055 * warpA + 0.022 * warpB;
    float z = zonal(latW);
    float zc = sign(z) * pow(abs(z), 0.72);
    float streak = fbm(vec3(d.x * 2.6, d.y * 42.0, d.z * 2.6) + vec3(warpB, warpA * 0.5, warpB), 6);
    float edge = 1.0 - abs(zc);
    float eddy = fbm(d * vec3(11.0, 26.0, 11.0) + vec3(5.2, 1.3, 8.7) + vec3(warpA, 0.0, warpB) * 0.8, 5);
    float puffs = fbm(d * 58.0 + vec3(2.0, 6.0, 1.0), 4);
    float v = 0.5 + 0.42 * zc + 0.34 * streak + 0.30 * eddy * (0.3 + 0.7 * edge) + 0.05 * puffs;

    vec3 cream = srgb(233.0, 216.0, 181.0);
    vec3 rust = srgb(188.0, 98.0, 52.0);
    vec3 umber = srgb(64.0, 34.0, 22.0);
    vec3 slate = srgb(75.0, 93.0, 143.0);
    vec3 deep = srgb(26.0, 33.0, 64.0);
    vec3 salmon = srgb(212.0, 148.0, 108.0);
    vec3 col = mix(rust, cream, smoothstep(0.36, 0.86, v));
    col = mix(umber * 1.3, col, smoothstep(0.02, 0.38, v));
    col = mix(col, salmon, 0.30 * smoothstep(0.42, 0.58, v) * (1.0 - smoothstep(0.58, 0.78, v)));
    float blueBelt = smoothstep(0.05, 0.62, fbm(vec3(d.x, lat2 * 3.2, d.z) + 4.0, 3) + 0.28 * sin(lat * 9.0 + 2.0));
    col = mix(col, slate * 1.05 + cream * 0.10, blueBelt * 0.30 * (1.0 - smoothstep(0.18, 0.72, v)));
    float polar = smoothstep(0.62, 1.35, abs(lat2));
    col = mix(col, mix(slate, deep, 0.45) * (0.65 + 0.7 * v), polar * 0.88);

    // The large oval storm: a rust-salmon eye with a pale rim, the eye twists the bands around it.
    float eye = smoothstep(0.30, 0.62, coreA);
    float rim = smoothstep(0.08, 0.26, coreA) * (1.0 - smoothstep(0.26, 0.44, coreA));
    vec3 stormCol = mix(srgb(170.0, 70.0, 42.0), srgb(222.0, 142.0, 98.0), 0.5 + 0.5 * streak);
    col = mix(col, stormCol, eye * 0.82);
    col += cream * rim * 0.20;
    float ovalMask = smoothstep(0.35, 0.65, coreB);
    col = mix(col, srgb(246.0, 238.0, 222.0), ovalMask * 0.75);
    col = clamp(col, 0.0, 0.92);

    // Zones of floating cloud cities (alpha): the lights themselves are drawn as round dots by the planet shader.
    float zone = smoothstep(0.42, 0.70, fbm(d * 4.0 + 9.0, 4) + 0.20 * cos(lat * 3.0)) * (1.0 - polar);
    gl_FragColor = vec4(col, clamp(zone * 1.3, 0.0, 1.0));
}`;

    // ---------------------------------------------------------------------------------------------
    // Nebula plate: a window of the sky (longitude and latitude half extents in uExtent), forward = -Z.

    const SKY_BAKE_FRAGMENT = `
precision highp float;
varying vec2 vUv;
uniform vec2 uExtent;
uniform vec3 uBand;
${NOISE}
vec3 lin(float r, float g, float b) { return pow(vec3(r, g, b) / 255.0, vec3(2.2)); }
void main() {
    float lon = mix(-uExtent.x, uExtent.x, vUv.x);
    float lat = mix(-uExtent.y, uExtent.y, vUv.y);
    vec3 d = vec3(sin(lon) * cos(lat), sin(lat), -cos(lon) * cos(lat));
    vec3 p = d * 2.6;
    vec3 w = vec3(fbm(p + vec3(0.0, 0.0, 1.0), 4), fbm(p + vec3(5.2, 1.3, 2.0), 4), fbm(p + vec3(9.1, 4.7, 3.0), 4));
    float cloud = fbm(p * 1.5 + w * 1.9, 6) * 0.5 + 0.5;
    float fine = fbm(p * 4.6 + w * 2.6, 5) * 0.5 + 0.5;
    float b = dot(d, normalize(uBand));
    float band = exp(-b * b / 0.065);
    float lanes = smoothstep(0.22, 0.62, fbm(p * 2.8 + w * 2.2 + 7.0, 5) * 0.5 + 0.5);
    float dens = band * smoothstep(0.30, 0.86, cloud) * mix(0.10, 1.0, lanes);
    float dust = band * smoothstep(0.5, 0.9, fine) * mix(0.0, 1.0, lanes);
    vec3 violet = lin(59.0, 29.0, 110.0);
    vec3 cyan = lin(15.0, 95.0, 122.0);
    vec3 amber = lin(106.0, 58.0, 18.0);
    float mixA = smoothstep(0.25, 0.80, fbm(p * 1.1 + 13.0, 4) * 0.5 + 0.5);
    float mixB = smoothstep(0.45, 0.85, fbm(p * 0.9 + 21.0, 4) * 0.5 + 0.5);
    vec3 col = mix(violet, cyan, mixA) * dens + amber * mixB * dust * 0.9 + cyan * dust * 0.25;
    col *= 0.95;
    col += (violet * 0.045 + cyan * 0.026) * (0.5 + 0.5 * cloud);
    // dither before the sRGB store: the plate is dark and smooth
    col += (hash13(vec3(gl_FragCoord.xy, 5.0)) - 0.5) * 0.0006;
    gl_FragColor = vec4(max(col, 0.0), 1.0);
}`;

    const SKY_VERTEX = `
varying vec3 vLocal;
void main() {
    vLocal = position;
    gl_Position = projectionMatrix * viewMatrix * modelMatrix * vec4(position, 1.0);
}`;

    const SKY_FRAGMENT = `
precision highp float;
varying vec3 vLocal;
uniform sampler2D uPlate;
uniform vec2 uExtent;
uniform float uGain;
void main() {
    vec3 d = normalize(vLocal);
    float lon = atan(d.x, -d.z);
    float lat = asin(clamp(d.y, -1.0, 1.0));
    vec2 uv = vec2(lon / uExtent.x, lat / uExtent.y) * 0.5 + 0.5;
    vec3 col = texture2D(uPlate, uv).rgb * uGain;
    gl_FragColor = vec4(col, 1.0);
}`;

    // ---------------------------------------------------------------------------------------------
    // Planet

    const PLANET_VERTEX = `
varying vec2 vUv;
varying vec3 vWorld;
varying vec3 vNormalW;
void main() {
    vUv = uv;
    vec4 w = modelMatrix * vec4(position, 1.0);
    vWorld = w.xyz;
    vNormalW = normalize(mat3(modelMatrix) * normal);
    gl_Position = projectionMatrix * viewMatrix * w;
}`;

    const PLANET_FRAGMENT = `
precision highp float;
varying vec2 vUv;
varying vec3 vWorld;
varying vec3 vNormalW;
uniform sampler2D uAlbedo;
uniform sampler2D uRingTex;
uniform vec3 uSunDir;
uniform vec3 uSunColor;
uniform vec3 uRingN;
uniform vec3 uRimColor;
uniform vec4 uRingRange;
uniform vec4 uBolt[4];
uniform float uShift;
uniform float uTime;
uniform float uDetail;
uniform float uGain;
${NOISE}
void main() {
    vec3 N = normalize(vNormalW);
    vec3 V = normalize(cameraPosition - vWorld);
    float lat = (vUv.y - 0.5) * PI;
    float shift = uShift * zonalSpeed(lat) / TAU;
    vec2 uv = vec2(vUv.x + shift, vUv.y);
    vec4 tex = texture2D(uAlbedo, uv);
    vec3 albedo = tex.rgb;
    if (uDetail > 0.5) {
        float fine = gnoise(vec3(uv * vec2(420.0, 210.0), 3.0)) * 0.5 + gnoise(vec3(uv * vec2(900.0, 450.0), 8.0)) * 0.5;
        albedo *= 1.0 + 0.10 * fine;
    }

    float ndl = dot(N, uSunDir);
    float day = smoothstep(-0.16, 0.42, ndl);
    float diff = pow(clamp((ndl + 0.12) / 1.12, 0.0, 1.0), 1.35);
    float tq = (ndl - 0.03) / 0.16;
    float twilight = exp(-tq * tq);

    // shadow of the rings: follow the ray to the sun through the ring plane
    float shadow = 1.0;
    float s = -1.0;
    float denom = dot(uSunDir, uRingN);
    if (abs(denom) > 0.02) {
        float tt = -dot(vWorld, uRingN) / denom;
        if (tt > 0.0) {
            float r = length(vWorld + uSunDir * tt);
            s = (r - uRingRange.x) / (uRingRange.y - uRingRange.x);
        }
    }
    // sampled outside any branch: the derivatives stay defined
    float ringDens = texture2D(uRingTex, vec2(clamp(s, 0.0, 1.0), 0.5)).r * step(0.0, s) * step(s, 1.0);
    shadow = 1.0 - 0.88 * ringDens;

    vec3 col = albedo * uSunColor * diff * shadow * uGain;
    // the sky of the terminator turns the clouds red
    col += albedo * vec3(0.95, 0.38, 0.18) * twilight * 0.50 * uGain;
    // earthshine: the night side is never black
    col += albedo * vec3(0.50, 0.62, 1.0) * 0.024 * (1.0 - day);
    // soft blue haze toward the day-side limb
    float fres = pow(1.0 - max(dot(N, V), 0.0), 3.2);
    col += uRimColor * fres * day * 0.22;

    // night: lights of cloud cities
    float night = 1.0 - smoothstep(-0.02, 0.22, ndl);
    vec2 grid = uv * vec2(760.0, 380.0);
    vec3 hc = hash3(vec3(floor(grid), 4.0));
    float dd = length(fract(grid) - (0.25 + 0.5 * hc.xy));
    float dotMask = (1.0 - smoothstep(0.0, 0.30, dd)) * step(hc.z, tex.a * 0.32);
    float tw = 0.65 + 0.35 * sin(uTime * (1.5 + 3.5 * hc.x) + hc.y * 40.0);
    col += vec3(1.0, 0.74, 0.32) * dotMask * night * tw * 3.0;

    // lightning: a bright core and a soft flash that lights the cloud tops under it
    vec3 radial = normalize(vWorld);
    float cloudLuma = dot(albedo, vec3(0.3, 0.5, 0.2));
    for (int i = 0; i < 4; i++) {
        float k = uBolt[i].w;
        if (k <= 0.001) continue;
        float a = 1.0 - max(dot(radial, uBolt[i].xyz), 0.0);
        float core = exp(-a * 2600.0);
        float halo = exp(-a * 140.0) * (0.35 + 0.9 * cloudLuma);
        col += vec3(0.62, 0.78, 1.0) * (core * 3.2 + halo * 0.9) * k * (1.0 - 0.8 * day);
    }
    gl_FragColor = vec4(col, 1.0);
}`;

    const ATMOSPHERE_FRAGMENT = `
precision highp float;
varying vec3 vWorld;
uniform vec3 uSunDir;
uniform vec3 uAxis;
uniform vec3 uDay;
uniform vec3 uDusk;
uniform float uPlanetR;
uniform float uShellR;
uniform float uIntensity;
uniform float uAurora;
uniform float uTime;
uniform int uSteps;
${NOISE}
void main() {
    vec3 ro = cameraPosition;
    vec3 rd = normalize(vWorld - cameraPosition);
    vec2 sh = raySphere(ro, rd, uShellR);
    if (sh.y < 0.0) discard;
    float t0 = max(sh.x, 0.0);
    float t1 = sh.y;
    vec2 pl = raySphere(ro, rd, uPlanetR);
    if (pl.x > 0.0) t1 = min(t1, pl.x);
    float len = t1 - t0;
    if (len <= 0.0) discard;
    float dt = len / float(uSteps);
    float thick = uShellR - uPlanetR;
    vec3 acc = vec3(0.0);
    vec3 aur = vec3(0.0);
    for (int i = 0; i < 16; i++) {
        if (i >= uSteps) break;
        vec3 p = ro + rd * (t0 + (float(i) + 0.5) * dt);
        float rr = length(p);
        vec3 np = p / rr;
        float h = clamp((rr - uPlanetR) / thick, 0.0, 1.0);
        float ndl = dot(np, uSunDir);
        float lit = smoothstep(-0.20, 0.45, ndl);
        vec3 tint = mix(uDusk, uDay, smoothstep(-0.05, 0.55, ndl));
        acc += tint * exp(-h * 3.0) * lit * dt;
        float la = asin(clamp(abs(dot(np, uAxis)), 0.0, 1.0));
        float zq = (la - 1.20) / 0.085;
        float zone = exp(-zq * zq);
        float dark = 1.0 - smoothstep(-0.10, 0.14, ndl);
        if (zone * dark > 0.01) {
            float curtain = gnoise(np * 7.0 + vec3(0.0, 0.0, uTime * 0.12)) * 0.5 + 0.5;
            float rays = gnoise(np * 46.0 + vec3(0.0, uTime * 0.35, 0.0)) * 0.5 + 0.5;
            float alt = smoothstep(0.0, 0.16, h) * (1.0 - smoothstep(0.16, 0.8, h));
            vec3 ac = mix(vec3(0.12, 1.0, 0.55), vec3(0.50, 0.28, 1.0), smoothstep(0.12, 0.55, h));
            aur += ac * zone * dark * pow(curtain, 1.4) * (0.35 + 1.0 * rays) * alt * dt;
        }
    }
    vec3 col = acc * (uIntensity / thick) + aur * (uAurora / thick);
    col = col / (1.0 + col * 0.30);
    gl_FragColor = vec4(col, 1.0);
}`;

    // ---------------------------------------------------------------------------------------------
    // Rings: a 1D profile texture (r density, g tone, b sparkle) over the radius.

    const RING_VERTEX = `
varying vec3 vLocal;
varying vec3 vWorld;
void main() {
    vLocal = position;
    vec4 w = modelMatrix * vec4(position, 1.0);
    vWorld = w.xyz;
    gl_Position = projectionMatrix * viewMatrix * w;
}`;

    const RING_FRAGMENT = `
precision highp float;
varying vec3 vLocal;
varying vec3 vWorld;
uniform sampler2D uProfile;
uniform vec3 uSunDir;
uniform vec3 uSunColor;
uniform vec3 uNormal;
uniform vec4 uRange;
uniform float uPlanetR;
uniform float uTime;
void main() {
    float r = length(vLocal.xy);
    float s = (r - uRange.x) / (uRange.y - uRange.x);
    vec4 pr = texture2D(uProfile, vec2(s, 0.5));
    float dens = pr.r;
    vec3 inner = vec3(0.70, 0.58, 0.40);
    vec3 outer = vec3(0.28, 0.23, 0.18);
    vec3 col = mix(outer, inner, pr.g) * (0.80 + 0.45 * pr.b);

    vec3 V = normalize(cameraPosition - vWorld);
    vec3 L = uSunDir;
    float side = dot(uNormal, V) * dot(uNormal, L);
    float facing = smoothstep(-0.08, 0.08, side);
    float flux = 0.45 + 0.55 * sqrt(abs(dot(uNormal, L)));
    float lit = mix(0.14, 1.0, facing) * flux;
    // light scattered forward by the dust when the camera looks toward the sun
    float fwd = pow(max(dot(-V, L), 0.0), 5.0);
    lit += fwd * 1.1 * (1.0 - facing * 0.6);

    // the shadow of the planet
    float b = dot(vWorld, L);
    float shadow = 1.0;
    if (b < 0.0) {
        float d = sqrt(max(dot(vWorld, vWorld) - b * b, 0.0));
        shadow = smoothstep(uPlanetR * 0.965, uPlanetR * 1.035, d);
    }
    float edge = smoothstep(0.0, 0.014, s) * (1.0 - smoothstep(0.986, 1.0, s));
    gl_FragColor = vec4(col * uSunColor * lit * mix(0.04, 1.0, shadow), dens * edge * 0.94);
}`;

    const DUST_VERTEX = `
attribute vec4 aOrbit;
uniform sampler2D uProfile;
uniform vec4 uRange;
uniform vec3 uSunDir;
uniform float uTime;
uniform float uPx;
uniform float uPlanetR;
uniform float uRef;
varying float vAlpha;
void main() {
    float r = aOrbit.x;
    float ang = aOrbit.y + uTime * 0.035 * pow(uRange.x / r, 1.5);
    vec3 local = vec3(r * cos(ang), r * sin(ang), aOrbit.z);
    vec4 w = modelMatrix * vec4(local, 1.0);
    vec4 mv = viewMatrix * w;
    gl_Position = projectionMatrix * mv;
    float s = (r - uRange.x) / (uRange.y - uRange.x);
    float dens = texture2D(uProfile, vec2(s, 0.5)).r;
    float b = dot(w.xyz, uSunDir);
    float shadow = 1.0;
    if (b < 0.0) {
        float d = sqrt(max(dot(w.xyz, w.xyz) - b * b, 0.0));
        shadow = smoothstep(uPlanetR * 0.965, uPlanetR * 1.035, d);
    }
    vAlpha = (0.12 + 0.88 * dens) * mix(0.05, 1.0, shadow) * (0.45 + 0.55 * aOrbit.w);
    gl_PointSize = max(1.0, uPx * (0.9 + 1.6 * aOrbit.w) * uRef / max(-mv.z, 1.0));
}`;

    const DUST_FRAGMENT = `
precision highp float;
varying float vAlpha;
void main() {
    vec2 c = gl_PointCoord - 0.5;
    float f = exp(-dot(c, c) * 14.0);
    gl_FragColor = vec4(vec3(0.95, 0.86, 0.70) * f * vAlpha * 0.9, 1.0);
}`;

    // ---------------------------------------------------------------------------------------------
    // Moon: cratered grey sphere with the same sun.

    const MOON_VERTEX = `
varying vec3 vObj;
varying vec3 vWorld;
varying vec3 vNormalW;
void main() {
    vObj = position;
    vec4 w = modelMatrix * vec4(position, 1.0);
    vWorld = w.xyz;
    vNormalW = normalize(mat3(modelMatrix) * normal);
    gl_Position = projectionMatrix * viewMatrix * w;
}`;

    const MOON_FRAGMENT = `
precision highp float;
varying vec3 vObj;
varying vec3 vWorld;
varying vec3 vNormalW;
uniform vec3 uSunDir;
uniform vec3 uSunColor;
uniform vec3 uPlanetShine;
uniform vec3 uPlanetPos;
uniform float uPlanetR;
${NOISE}
// Distance to the nearest feature point of a jittered 3D grid.
float worley(vec3 p) {
    vec3 i = floor(p);
    vec3 f = p - i;
    float best = 8.0;
    for (int x = -1; x <= 1; x++) {
        for (int y = -1; y <= 1; y++) {
            for (int z = -1; z <= 1; z++) {
                vec3 g = vec3(float(x), float(y), float(z));
                vec3 o = hash33(i + g) * 0.5 + 0.5;
                best = min(best, length(g + o - f));
            }
        }
    }
    return best;
}
float crater(float f) {
    float bowl = 1.0 - smoothstep(0.14, 0.30, f);
    float rim = smoothstep(0.20, 0.30, f) * (1.0 - smoothstep(0.30, 0.44, f));
    return rim * 0.55 - bowl * 0.45;
}
void main() {
    vec3 N = normalize(vNormalW);
    vec3 p = normalize(vObj);
    float mare = smoothstep(0.02, 0.30, fbm(p * 1.9 + 3.0, 4));
    float c = crater(worley(p * 6.0)) + 0.7 * crater(worley(p * 14.0 + 5.0)) + 0.5 * crater(worley(p * 33.0 + 9.0));
    float grain = fbm(p * 22.0, 3);
    vec3 albedo = mix(vec3(0.36, 0.34, 0.31), vec3(0.17, 0.17, 0.19), mare) * (1.0 + 0.55 * c + 0.18 * grain);
    float ndl = dot(N, uSunDir);
    float diff = smoothstep(-0.05, 0.9, ndl);
    vec3 col = albedo * uSunColor * diff * 1.1;
    // the glow of the giant lights the night side of the moon
    float toPlanet = max(dot(N, normalize(uPlanetPos - vWorld)), 0.0);
    col += albedo * uPlanetShine * toPlanet * 0.30;
    col += albedo * 0.006;
    gl_FragColor = vec4(col, 1.0);
}`;

    // ---------------------------------------------------------------------------------------------
    // Stars: three point layers; the size, brightness, colour temperature and twinkle phase are per star.

    const STAR_VERTEX = `
attribute float aSize;
attribute float aBright;
attribute float aTemp;
attribute float aPhase;
uniform float uPx;
uniform float uTime;
uniform float uTwinkle;
varying vec3 vColor;
void main() {
    gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
    float rate = 1.3 + 2.7 * fract(aPhase * 7.31);
    float tw = 1.0 + uTwinkle * (0.22 * sin(uTime * rate + aPhase * 6.283) + 0.10 * sin(uTime * 4.1 + aPhase * 91.0));
    vec3 cool = vec3(1.0, 0.76, 0.52);
    vec3 white = vec3(1.0, 0.96, 0.90);
    vec3 hot = vec3(0.60, 0.74, 1.0);
    vec3 c = aTemp < 0.5 ? mix(cool, white, aTemp * 2.0) : mix(white, hot, (aTemp - 0.5) * 2.0);
    vColor = c * aBright * tw;
    gl_PointSize = max(1.0, aSize * uPx);
}`;

    const STAR_FRAGMENT = `
precision highp float;
varying vec3 vColor;
void main() {
    vec2 c = gl_PointCoord - 0.5;
    float d2 = dot(c, c) * 4.0;
    float f = exp(-d2 * 3.2) + 0.35 * exp(-d2 * 14.0);
    gl_FragColor = vec4(vColor * f, 1.0);
}`;

    // ---------------------------------------------------------------------------------------------
    // Sun and lens flare

    const BILLBOARD_VERTEX = `
varying vec2 vUv;
uniform float uSize;
void main() {
    vUv = uv * 2.0 - 1.0;
    vec4 mv = viewMatrix * modelMatrix * vec4(0.0, 0.0, 0.0, 1.0);
    mv.xy += position.xy * uSize;
    gl_Position = projectionMatrix * mv;
}`;

    const SUN_FRAGMENT = `
precision highp float;
varying vec2 vUv;
uniform vec3 uColor;
uniform float uGain;
void main() {
    float d = length(vUv);
    float ang = atan(vUv.y, vUv.x);
    float core = exp(-d * d * 1500.0) * 14.0;
    float glow = exp(-d * 8.5) * 0.85;
    float halo = 0.30 / (1.0 + d * d * 220.0);
    float rays = pow(abs(cos(ang * 3.0 + 0.3)), 18.0) * exp(-d * 5.0) * 0.30;
    float fade = 1.0 - smoothstep(0.82, 1.0, d);
    gl_FragColor = vec4(uColor * (core + glow + halo + rays) * fade * uGain, 1.0);
}`;

    const FLARE_VERTEX = `
varying vec2 vUv;
void main() {
    vUv = uv * 2.0 - 1.0;
    gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
}`;

    const FLARE_FRAGMENT = `
precision highp float;
varying vec2 vUv;
uniform vec3 uColor;
uniform float uAlpha;
uniform float uShape;
void main() {
    float d = length(vUv);
    float f = 0.0;
    if (uShape < 0.5) {
        f = (1.0 - smoothstep(0.86, 1.0, d)) * (0.30 + 0.70 * smoothstep(0.25, 1.0, d));
    } else if (uShape < 1.5) {
        vec2 q = abs(vUv);
        float hex = max(q.x * 0.866 + q.y * 0.5, q.y);
        f = (1.0 - smoothstep(0.84, 0.96, hex)) * (0.4 + 0.6 * smoothstep(0.2, 0.95, hex));
    } else if (uShape < 2.5) {
        float rq = (d - 0.78) / 0.09;
        f = exp(-rq * rq);
    } else {
        f = exp(-abs(vUv.y) * 70.0) * exp(-abs(vUv.x) * 2.2);
    }
    gl_FragColor = vec4(uColor * f * uAlpha, 1.0);
}`;

    // ---------------------------------------------------------------------------------------------
    // Comet and the exhaust trail of the shuttle (additive point sprites)

    const COMET_VERTEX = `
attribute float aT;
uniform vec3 uHead;
uniform vec3 uTail;
uniform float uLen;
uniform float uPx;
uniform float uAlpha;
varying float vFade;
varying float vK;
void main() {
    vec3 pos = uHead + uTail * (aT * uLen);
    gl_Position = projectionMatrix * viewMatrix * modelMatrix * vec4(pos, 1.0);
    vK = aT;
    vFade = uAlpha * (aT < 0.001 ? 1.0 : pow(1.0 - aT, 2.2) * 0.55);
    gl_PointSize = uPx * (aT < 0.001 ? 34.0 : mix(22.0, 5.0, aT));
}`;

    const COMET_FRAGMENT = `
precision highp float;
varying float vFade;
varying float vK;
void main() {
    vec2 c = gl_PointCoord - 0.5;
    float d2 = dot(c, c) * 4.0;
    float f = exp(-d2 * 4.0) + (vK < 0.001 ? 2.4 * exp(-d2 * 40.0) : 0.0);
    vec3 col = mix(vec3(0.85, 0.96, 1.0), vec3(0.45, 0.62, 1.0), clamp(vK * 1.6, 0.0, 1.0));
    gl_FragColor = vec4(col * f * vFade * (vK < 0.001 ? 3.0 : 0.9), 1.0);
}`;

    const EXHAUST_VERTEX = `
attribute float aAge;
uniform float uLife;
uniform float uPx;
uniform float uRef;
varying float vFade;
varying float vK;
void main() {
    vec4 mv = modelViewMatrix * vec4(position, 1.0);
    gl_Position = projectionMatrix * mv;
    float k = clamp(aAge / uLife, 0.0, 1.0);
    vK = k;
    vFade = (1.0 - k) * (1.0 - k) * (aAge < 0.0 || aAge > uLife ? 0.0 : 1.0);
    gl_PointSize = max(1.0, uPx * (1.0 - 0.45 * k) * 7.0 * uRef / max(-mv.z, 1.0));
}`;

    const EXHAUST_FRAGMENT = `
precision highp float;
varying float vFade;
varying float vK;
void main() {
    vec2 c = gl_PointCoord - 0.5;
    float f = exp(-dot(c, c) * 4.0 * 3.4);
    vec3 col = mix(vec3(0.36, 0.90, 1.0), vec3(0.54, 0.42, 1.0), vK);
    gl_FragColor = vec4(col * f * vFade * 2.6, 1.0);
}`;

    // ---------------------------------------------------------------------------------------------
    // Grade: the last pass before tone mapping.

    const GRADE_FRAGMENT = `
precision highp float;
varying vec2 vUv;
uniform sampler2D tDiffuse;
uniform vec2 uRes;
uniform float uAspect;
uniform float uTime;
uniform float uVignette;
uniform float uAberration;
uniform float uGrain;
uniform vec2 uPulseCenter;
uniform float uPulseTime;
uniform vec4 uMeteorA[3];
uniform vec4 uMeteorB[3];
float hash12(vec2 p) {
    vec3 p3 = fract(vec3(p.xyx) * 0.1031);
    p3 += dot(p3, p3.yzx + 33.33);
    return fract((p3.x + p3.y) * p3.z);
}
float segment(vec2 p, vec2 a, vec2 b, out float along) {
    vec2 pa = p - a;
    vec2 ba = b - a;
    along = clamp(dot(pa, ba) / max(dot(ba, ba), 1e-6), 0.0, 1.0);
    return length(pa - ba * along);
}
void main() {
    vec2 uv = vUv;
    vec2 asp = vec2(uAspect, 1.0);
    vec3 glow = vec3(0.0);

    // scanner pulse: a thin bright ring that expands over the screen and bends the picture inside its edge
    if (uPulseTime >= 0.0 && uPulseTime < 1.35) {
        vec2 delta = (uv - uPulseCenter) * asp;
        float dist = length(delta);
        float radius = 0.9 * uPulseTime;
        float fade = 1.0 - smoothstep(0.55, 1.35, uPulseTime);
        float w = 0.0065;
        float rq = (dist - radius) / w;
        float ring = exp(-rq * rq);
        float bq = (dist - radius) / (w * 4.5);
        float bump = exp(-bq * bq);
        uv -= normalize(delta + 1e-5) / asp * bump * 0.0055 * fade;
        glow += vec3(0.36, 0.88, 1.0) * ring * 1.7 * fade + vec3(0.16, 0.35, 0.8) * bump * 0.10 * fade;
    }

    vec2 c = vUv - 0.5;
    vec2 off = c * uAberration * (dot(c * asp, c * asp) * 2.0);
    vec3 col;
    col.r = texture2D(tDiffuse, uv + off).r;
    col.g = texture2D(tDiffuse, uv).g;
    col.b = texture2D(tDiffuse, uv - off).b;

    // shooting stars in screen space (y down, height-normalised)
    vec2 q = vec2(vUv.x * uAspect, 1.0 - vUv.y);
    for (int i = 0; i < 3; i++) {
        float vis = uMeteorB[i].y;
        if (vis <= 0.001) continue;
        float lenM = uMeteorA[i].w * min(uAspect, 1.0);
        vec2 dir = vec2(cos(uMeteorA[i].z), sin(uMeteorA[i].z));
        vec2 start = vec2(uMeteorA[i].x * uAspect, uMeteorA[i].y);
        float p = uMeteorB[i].x;
        vec2 head = start + dir * lenM * (0.1 + 2.0 * p);
        vec2 tail = head - dir * lenM * (0.30 + 0.35 * sin(3.14159 * p));
        float along;
        float dd = segment(q, tail, head, along);
        float line = exp(-pow(dd / 0.0011, 2.0)) * along * along;
        float soft = exp(-pow(dd / 0.0065, 2.0)) * along * along * 0.30;
        glow += vec3(0.86, 0.95, 1.0) * (line * 3.2 + soft) * vis;
    }
    col += glow;

    // vignette
    float vr = length(c * asp * vec2(1.0, 1.15));
    col *= 1.0 - uVignette * smoothstep(0.32, 1.22, vr);

    // film grain: multiplicative, a little stronger in the shadows, plus a small dither against banding
    float n1 = hash12(vUv * uRes + fract(uTime * 7.31) * 113.0);
    float n2 = hash12(vUv * uRes * 1.37 + fract(uTime * 3.17) * 71.0);
    float luma = dot(col, vec3(0.299, 0.587, 0.114));
    col *= 1.0 + (n1 - 0.5) * uGrain * 1.6 * (1.0 - 0.5 * smoothstep(0.0, 1.5, luma));
    col += (n2 - 0.5) * 0.0011 * (0.4 + sqrt(luma));
    gl_FragColor = vec4(max(col, 0.0), 1.0);
}`;

    window.AuraOrbitShaders = Object.freeze({
        NOISE,
        fullscreenVertex: FULLSCREEN_VERTEX,
        planetBakeFragment: PLANET_BAKE_FRAGMENT,
        skyBakeFragment: SKY_BAKE_FRAGMENT,
        skyVertex: SKY_VERTEX,
        skyFragment: SKY_FRAGMENT,
        planetVertex: PLANET_VERTEX,
        planetFragment: PLANET_FRAGMENT,
        atmosphereVertex: PLANET_VERTEX,
        atmosphereFragment: ATMOSPHERE_FRAGMENT,
        ringVertex: RING_VERTEX,
        ringFragment: RING_FRAGMENT,
        dustVertex: DUST_VERTEX,
        dustFragment: DUST_FRAGMENT,
        moonVertex: MOON_VERTEX,
        moonFragment: MOON_FRAGMENT,
        starVertex: STAR_VERTEX,
        starFragment: STAR_FRAGMENT,
        billboardVertex: BILLBOARD_VERTEX,
        sunFragment: SUN_FRAGMENT,
        flareVertex: FLARE_VERTEX,
        flareFragment: FLARE_FRAGMENT,
        cometVertex: COMET_VERTEX,
        cometFragment: COMET_FRAGMENT,
        exhaustVertex: EXHAUST_VERTEX,
        exhaustFragment: EXHAUST_FRAGMENT,
        gradeVertex: FULLSCREEN_VERTEX,
        gradeFragment: GRADE_FRAGMENT
    });
}());
