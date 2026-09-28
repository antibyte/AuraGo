(function () {
    'use strict';

    const QUALITY = {
        high: { scale: 0.72, steps: 48, reflectSteps: 24, bloomLevels: 6 },
        medium: { scale: 0.58, steps: 34, reflectSteps: 16, bloomLevels: 5 },
        low: { scale: 0.45, steps: 22, reflectSteps: 10, bloomLevels: 4 }
    };

    const PALETTES = [
        { weight: 5, edge: [1.0, 0.22, 0.62], main: [0.1, 1.0, 0.38], top: [0.46, 0.12, 0.95] },
        { weight: 3, edge: [0.8, 0.26, 1.0], main: [0.06, 0.95, 0.66], top: [0.3, 0.24, 1.0] },
        { weight: 1, edge: [1.0, 0.3, 0.55], main: [0.25, 1.0, 0.34], top: [1.0, 0.12, 0.18] }
    ];

    const CAMERA_HEIGHT = 0.12;
    const TAN_HALF_FOV = 0.58;
    const TEMPORAL_BLEND = 0.2;

    const SHARED_GLSL = `
uniform sampler2D uNoise;
uniform float uPhase;
uniform float uIntensity;
uniform vec3 uColEdge;
uniform vec3 uColMain;
uniform vec3 uColTop;
uniform vec3 uAmbient;

float tnoise(vec2 p) {
    vec2 i = floor(p);
    vec2 f = fract(p);
    f = f * f * (3.0 - 2.0 * f);
    return textureLod(uNoise, (i + f + 0.5) * (1.0 / 256.0), 0.0).r;
}

float ridgeProfile(float x, float seed) {
    float h = 0.0;
    float amp = 1.0;
    float f = 1.0;
    float norm = 0.0;
    for (int i = 0; i < 6; i++) {
        float n = tnoise(vec2(x * f, seed + float(i) * 7.3));
        n = 1.0 - abs(n * 2.0 - 1.0);
        h += n * n * n * amp;
        norm += amp;
        amp *= 0.5;
        f *= 2.17;
    }
    return h / norm;
}

float treeProfile(float a) {
    float x = a * 46.0;
    float best = 0.0;
    for (int k = 0; k < 3; k++) {
        float xs = x + float(k) * 0.37;
        float cell = floor(xs);
        float f = fract(xs);
        float h = tnoise(vec2(cell * 3.7 + float(k) * 9.1, 11.0 + float(k) * 5.0));
        float gap = step(0.16, tnoise(vec2(cell * 0.61 + float(k), 91.0)));
        float tall = (0.35 + 0.9 * h * h) * gap;
        float spire = max(0.0, 1.0 - abs(f - 0.5) * 2.6);
        float tiers = 0.86 + 0.14 * abs(sin(f * 38.0 + cell * 1.7));
        best = max(best, spire * spire * tall * tiers);
    }
    return best;
}

vec3 skyGradient(vec3 rd) {
    float up = clamp(rd.y, 0.0, 1.0);
    vec3 zenith = vec3(0.0005, 0.0009, 0.0028);
    vec3 horizon = vec3(0.0035, 0.0065, 0.012);
    vec3 col = mix(horizon, zenith, pow(up, 0.4));
    col += uAmbient * (0.045 * exp(-up * 6.0) + 0.006);
    vec3 bandNormal = normalize(vec3(0.42, 0.55, -0.72));
    float band = exp(-pow(dot(rd, bandNormal), 2.0) * 30.0);
    float dust = tnoise(rd.xz * 26.0 + rd.y * 9.0) * tnoise(rd.zy * 47.0 + 3.0);
    float lanes = smoothstep(0.35, 0.75, tnoise(rd.xy * 18.0 + 40.0));
    col += vec3(0.0065, 0.0068, 0.009) * band * (0.25 + 2.2 * dust) * (1.0 - 0.6 * lanes);
    return col;
}

float curtainCenter(float x, float k) {
    float ph = uPhase * (0.75 + 0.2 * k);
    return 4.0 + k * 4.0
        + 2.6 * sin(x * 0.11 + ph * 0.5 + k * 2.1)
        + 1.1 * sin(x * 0.29 - ph * 0.8 + k * 1.3)
        + 2.4 * (tnoise(vec2(x * 0.12 + k * 31.0, ph * 0.2)) - 0.5)
        + 0.22 * sin(x * 1.15 + ph * 2.0 + k);
}

vec3 auroraColor(float h) {
    vec3 c = mix(uColEdge, uColMain, smoothstep(0.0, 0.12, h));
    return mix(c, uColTop, smoothstep(0.9, 2.8, h));
}

vec3 auroraVolume(vec3 ro, vec3 rd, int steps, float jitter) {
    if (rd.y < 0.015) return vec3(0.0);
    float t0 = (1.9 - ro.y) / rd.y;
    float t1 = min((8.2 - ro.y) / rd.y, t0 + 44.0);
    float dt = (t1 - t0) / float(steps);
    float t = t0 + dt * jitter;
    vec3 acc = vec3(0.0);
    for (int i = 0; i < 96; i++) {
        if (i >= steps) break;
        vec3 p = ro + rd * t;
        float fade = exp(-length(p.xz) * 0.028);
        for (int kk = 0; kk < 3; kk++) {
            float k = float(kk);
            float d = abs(p.z - curtainCenter(p.x, k));
            float w = 0.2 + 0.1 * k;
            if (d > w * 3.0) continue;
            float wLocal = w * (0.7 + 0.6 * tnoise(vec2(p.x * 0.9 + k * 11.0, uPhase * 0.4)));
            float sheet = exp(-d * d / (wLocal * wLocal));
            float base = 2.3 + 0.35 * k + 0.4 * (tnoise(vec2(p.x * 0.25 + k * 5.0, uPhase * 0.3)) - 0.5) + 0.07 * sin(p.x * 2.3 + uPhase * 3.0 + k);
            float h = p.y - base;
            float edge = smoothstep(-0.05, 0.05, h);
            float hp = max(h, 0.0);
            float fall = exp(-hp * (0.55 + 0.15 * k)) * (1.0 - smoothstep(3.2, 5.5, hp));
            float r = tnoise(vec2(p.x * 4.0 + k * 17.0, uPhase * 0.9));
            float rays = 0.3 + 1.7 * r * r * r + 0.35 * tnoise(vec2(p.x * 13.0 - uPhase * 1.5, k * 3.0));
            float patchiness = 0.25 + 1.35 * pow(tnoise(vec2(p.x * 0.05 + k * 7.0, uPhase * 0.08)), 1.5);
            float brightness = 1.0 - 0.25 * k;
            float glowEdge = 1.0 + 1.0 * exp(-hp * 7.0);
            acc += auroraColor(hp) * (sheet * edge * fall * rays * patchiness * brightness * glowEdge * fade * dt);
        }
        t += dt;
    }
    return acc * uIntensity;
}

// Returns rgb plus star visibility in alpha.
vec4 landscape(vec3 ro, vec3 rd, int steps, float jitter, float pixelAngle) {
    float horiz = length(rd.xz);
    float tanE = rd.y / max(horiz, 1e-4);
    float az = atan(rd.x, rd.z);
    float aa = pixelAngle * 1.25;

    vec3 sky = skyGradient(rd) + auroraVolume(ro, rd, steps, jitter);
    vec4 col = vec4(sky, 1.0);

    float layers[3] = float[3](38.0, 23.0, 13.5);
    float heights[3] = float[3](6.5, 5.2, 2.7);
    for (int i = 0; i < 3; i++) {
        float D = layers[i];
        float fi = float(i);
        float coord = az * (3.4 - fi * 0.6) + fi * 41.0;
        float r = ridgeProfile(coord, fi * 19.0 + 3.0);
        float H = heights[i] * (0.12 + 1.05 * r);
        float ridgeTan = (H - ro.y) / D;
        float cover = 1.0 - smoothstep(ridgeTan - aa, ridgeTan + aa, tanE);
        if (cover <= 0.0) continue;
        float baseTan = -ro.y / D;
        float hf = clamp((tanE - baseTan) / max(ridgeTan - baseTan, 1e-4), 0.0, 1.0);
        vec2 face = vec2(az * D, tanE * D);
        float tex = tnoise(face * 0.8 + fi * 13.0) * 0.5
                  + tnoise(face * vec2(2.4, 1.3) + fi * 7.0) * 0.33
                  + tnoise(face * vec2(6.5, 3.6) + fi * 3.0) * 0.17;
        float snow = smoothstep(0.46, 0.6, hf * 0.7 + tex * 0.6);
        float rim = exp(-max(ridgeTan - tanE, 0.0) / (pixelAngle * (2.5 + 2.5 * (2.0 - fi))));
        vec3 rock = vec3(0.0009, 0.0013, 0.0024);
        vec3 snowCol = vec3(0.006, 0.0085, 0.013) + uAmbient * (0.1 + 0.03 * fi);
        vec3 surface = mix(rock, snowCol, snow);
        surface += uAmbient * 0.05 * rim * snow;
        float haze = (2.0 - fi) * 0.22;
        surface = mix(surface, vec3(0.003, 0.0055, 0.0105), haze);
        col = mix(col, vec4(surface, 0.0), cover);
    }

    float trees = 0.05 + treeProfile(az) * 0.5 + 0.08 * ridgeProfile(az * 4.0, 77.0);
    float treeTan = (trees - ro.y) / 6.5;
    float treeCover = 1.0 - smoothstep(treeTan - aa, treeTan + aa, tanE);
    vec3 treeCol = vec3(0.0004, 0.0007, 0.0011) + uAmbient * 0.004;
    col = mix(col, vec4(treeCol, 0.0), treeCover);
    return col;
}
`;

    const SCENE_FS = SHARED_GLSL + `
uniform vec2 uResolution;
uniform vec3 uCamPos;
uniform mat3 uCamRot;
uniform float uTanHalfFov;
uniform float uTime;
uniform int uSteps;
uniform int uReflectSteps;
uniform sampler2D uPrev;
uniform float uBlend;
uniform float uFrame;

vec4 shade(vec2 frag) {
    vec2 ndc = (frag / uResolution) * 2.0 - 1.0;
    ndc.x *= uResolution.x / uResolution.y;
    vec3 rd = normalize(uCamRot * vec3(ndc * uTanHalfFov, 1.0));
    float pixelAngle = 2.0 * uTanHalfFov / uResolution.y;
    float jitter = fract(52.9829189 * fract(dot(frag + 5.588238 * mod(uFrame, 64.0), vec2(0.06711056, 0.00583715))));

    float horiz = length(rd.xz);
    float tanE = rd.y / max(horiz, 1e-4);
    float shoreTan = -uCamPos.y / 6.5;
    if (tanE >= shoreTan) {
        vec4 land = landscape(uCamPos, rd, uSteps, jitter, pixelAngle);
        float shoreMist = exp(-abs(tanE - shoreTan) * 70.0);
        land.rgb += uAmbient * 0.025 * shoreMist;
        return land;
    }

    float tWater = -uCamPos.y / rd.y;
    vec3 wp = uCamPos + rd * tWater;
    float dist = length(wp.xz);
    vec2 q = wp.xz * vec2(0.8, 4.2);
    vec2 flow = vec2(uTime * 0.11, uTime * 0.37);
    float e = 0.05;
    float n0 = tnoise(q + flow) + 0.45 * tnoise(q * 2.7 - flow * 1.3);
    float nx = tnoise(q + vec2(e, 0.0) + flow) + 0.45 * tnoise((q + vec2(e, 0.0)) * 2.7 - flow * 1.3);
    float nz = tnoise(q + vec2(0.0, e) + flow) + 0.45 * tnoise((q + vec2(0.0, e)) * 2.7 - flow * 1.3);
    float ripple = 0.006 * exp(-dist * 0.2) + 0.0012;
    vec2 slope = vec2(nx - n0, nz - n0) / e * ripple;
    vec3 mirroredDir = normalize(vec3(rd.x + slope.x * 0.3, max(-rd.y + slope.y, 0.002), rd.z));
    vec3 mirroredPos = vec3(uCamPos.x, -uCamPos.y, uCamPos.z);
    vec4 refl = landscape(mirroredPos, mirroredDir, uReflectSteps, jitter, pixelAngle * 1.5);

    float cosTheta = clamp(-rd.y, 0.0, 1.0);
    float fresnel = 0.02 + 0.98 * pow(1.0 - cosTheta, 5.0);
    vec3 deep = vec3(0.0003, 0.0008, 0.0014) + uAmbient * 0.006;
    vec3 water = mix(deep, refl.rgb * 0.9, fresnel);
    float mist = (1.0 - exp(-dist * 0.08)) * (0.4 + 0.8 * tnoise(wp.xz * 0.35 + vec2(uTime * 0.04, 0.0)));
    water += uAmbient * 0.02 * mist;
    return vec4(water, -max(refl.a, 0.0) * fresnel * 0.7);
}

void main() {
    vec4 current = shade(gl_FragCoord.xy);
    vec4 prev = texture(uPrev, gl_FragCoord.xy / uResolution);
    fragColor = mix(prev, current, uBlend);
}
`;

    const COMPOSITE_FS = `
uniform sampler2D uScene;
uniform sampler2D uBloom;
uniform vec2 uResolution;
uniform mat3 uCamRot;
uniform float uTanHalfFov;
uniform float uTime;
uniform float uExposure;
uniform float uBloomStrength;
uniform vec4 uMeteor;
uniform float uMeteorProgress;
uniform float uMeteorAlpha;

vec3 starLayer(vec3 d, float scale, float threshold, float seed, float pixelCells, float gain) {
    vec3 p = d * scale;
    vec3 cell = floor(p);
    vec3 h = hash33(cell + seed);
    if (h.x < threshold) return vec3(0.0);
    vec3 center = cell + 0.5 + (h - 0.5) * 0.45;
    float dist = length(p - center);
    float size = max(pixelCells * 0.7, 0.035);
    float twinkle = 0.72 + 0.28 * sin(uTime * (1.3 + h.z * 3.5) + h.x * 80.0);
    float mag = pow((h.x - threshold) / (1.0 - threshold), 4.0);
    vec3 tint = mix(vec3(0.72, 0.83, 1.0), vec3(1.0, 0.86, 0.7), h.y);
    return tint * exp(-dist * dist / (size * size)) * twinkle * (0.06 + 2.4 * mag) * gain;
}

vec3 starField(vec3 d, float pixelAngle) {
    return starLayer(d, 55.0, 0.955, 1.0, pixelAngle * 55.0, 1.0)
         + starLayer(d, 130.0, 0.9, 17.0, pixelAngle * 130.0, 0.35);
}

float segmentDistance(vec2 p, vec2 a, vec2 b, out float along) {
    vec2 pa = p - a;
    vec2 ba = b - a;
    along = clamp(dot(pa, ba) / max(dot(ba, ba), 1e-6), 0.0, 1.0);
    return length(pa - ba * along);
}

void main() {
    vec4 scene = texture(uScene, vUv);
    vec3 col = scene.rgb;
    vec2 ndc = vUv * 2.0 - 1.0;
    float aspect = uResolution.x / uResolution.y;
    ndc.x *= aspect;
    vec3 rd = normalize(uCamRot * vec3(ndc * uTanHalfFov, 1.0));
    float pixelAngle = 2.0 * uTanHalfFov / uResolution.y;
    if (scene.a > 0.002) {
        col += starField(rd, pixelAngle) * scene.a;
    } else if (scene.a < -0.002) {
        vec3 mirrored = normalize(vec3(rd.x, -rd.y, rd.z));
        col += starField(mirrored, pixelAngle * 1.6) * (-scene.a) * 0.7;
    }

    if (uMeteorAlpha > 0.0) {
        vec2 p = vec2(vUv.x * aspect, vUv.y);
        vec2 a = vec2(uMeteor.x * aspect, uMeteor.y);
        vec2 b = vec2(uMeteor.z * aspect, uMeteor.w);
        vec2 head = mix(a, b, uMeteorProgress);
        vec2 tail = mix(a, b, max(0.0, uMeteorProgress - 0.22));
        float along;
        float d = segmentDistance(p, tail, head, along);
        float width = 0.0008 + 0.0011 * along;
        float glow = exp(-d * d / (width * width)) * along * along;
        col += vec3(0.85, 0.95, 1.0) * glow * uMeteorAlpha * 4.0 * max(scene.a, 0.0);
    }

    col += texture(uBloom, vUv).rgb * uBloomStrength;
    col *= uExposure;
    col = acesTonemap(col);
    float vig = smoothstep(1.3, 0.3, length((vUv - 0.5) * vec2(aspect * 0.8, 1.0)));
    col *= mix(0.68, 1.0, vig);
    col = linearToSrgb(col);
    float grain = (hash12(gl_FragCoord.xy + fract(uTime) * 91.7) - 0.5) * 0.014;
    fragColor = vec4(col + grain + screenDither(gl_FragCoord.xy), 1.0);
}
`;

    function pickPalette(rng) {
        const total = PALETTES.reduce((sum, p) => sum + p.weight, 0);
        let roll = rng() * total;
        for (const palette of PALETTES) {
            roll -= palette.weight;
            if (roll <= 0) return palette;
        }
        return PALETTES[0];
    }

    function cameraMatrix(yaw, pitch) {
        const cy = Math.cos(yaw);
        const sy = Math.sin(yaw);
        const cp = Math.cos(pitch);
        const sp = Math.sin(pitch);
        // Columns: right, up, forward (column-major for uniformMatrix3fv).
        return new Float32Array([
            cy, 0, -sy,
            sy * sp, cp, cy * sp,
            sy * cp, -sp, cy * cp
        ]);
    }

    function noiseData(rng) {
        const data = new Uint8Array(256 * 256 * 4);
        for (let i = 0; i < data.length; i++) data[i] = Math.floor(rng() * 256);
        return data;
    }

    function createAurora(env) {
        const GL = window.AuraScreensaverGL;
        const kit = GL.create(env.canvas);
        const seed = (Math.random() * 0xffffffff) >>> 0;
        const rng = GL.createRng(seed);
        const palette = pickPalette(rng);
        const noise = kit.dataTexture(256, 256, 'rgba8', noiseData(rng), { wrap: 'repeat' });
        const sceneProgram = kit.program(SCENE_FS, { name: 'aurora-scene' });
        const compositeProgram = kit.program(COMPOSITE_FS, { name: 'aurora-composite' });
        let quality = QUALITY[env.quality] ? env.quality : 'high';
        let width = env.width;
        let height = env.height;
        const scene = kit.doubleTarget(1, 1, { format: 'rgba16f' });
        let bloom = null;
        let frameIndex = 0;
        let blend = 1;
        const yawBase = (rng() - 0.5) * 1.2;
        let phase = rng() * 50;
        let meteor = null;
        let nextMeteor = 6 + rng() * 10;
        let surge = { start: 18 + rng() * 25, length: 14 };
        const ambient = palette.main.map((c, i) => c * 0.12 + palette.top[i] * 0.03);

        function layout() {
            const q = QUALITY[quality];
            scene.resize(width * q.scale, height * q.scale);
            blend = 1;
            if (!bloom) bloom = kit.bloom(scene.width, scene.height, { levels: q.bloomLevels });
            else bloom.resize(scene.width, scene.height);
        }
        layout();

        function surgeFactor(t) {
            if (t > surge.start + surge.length) {
                surge = { start: t + 25 + rng() * 45, length: 10 + rng() * 12 };
            }
            if (t < surge.start) return 0;
            const x = (t - surge.start) / surge.length;
            return Math.sin(Math.PI * Math.min(1, Math.max(0, x)));
        }

        function updateMeteor(t, dt) {
            if (!meteor && t > nextMeteor) {
                const x0 = 0.1 + rng() * 0.8;
                const y0 = 0.72 + rng() * 0.24;
                const angle = (rng() < 0.5 ? -1 : 1) * (0.35 + rng() * 0.5);
                const len = 0.18 + rng() * 0.22;
                meteor = { a: [x0, y0], b: [x0 + Math.sin(angle) * len, y0 - Math.cos(angle) * len * 0.8], age: 0, life: 0.6 + rng() * 0.5 };
            }
            if (!meteor) return;
            meteor.age += dt;
            if (meteor.age > meteor.life) {
                meteor = null;
                nextMeteor = t + 8 + rng() * 18;
            }
        }

        function frame(t, dt) {
            const gl = kit.gl;
            const q = QUALITY[quality];
            const s = surgeFactor(t);
            phase += dt * (0.1 + 0.3 * s);
            updateMeteor(t, dt);
            const intensity = (0.52 + 0.1 * Math.sin(t * 0.21) + 0.07 * Math.sin(t * 0.67)) * (1 + 0.8 * s);
            const yaw = yawBase + 0.2 * Math.sin(t * 0.017) + 0.05 * Math.sin(t * 0.043);
            const pitch = -0.2 - 0.03 * Math.sin(t * 0.029);
            const rot = cameraMatrix(yaw, pitch);
            const amb = ambient.map(c => c * intensity);
            kit.draw(sceneProgram, {
                uNoise: noise,
                uPhase: phase,
                uIntensity: intensity,
                uColEdge: palette.edge,
                uColMain: palette.main,
                uColTop: palette.top,
                uAmbient: amb,
                uResolution: [scene.width, scene.height],
                uCamPos: [0, CAMERA_HEIGHT, 0],
                uCamRot: rot,
                uTanHalfFov: TAN_HALF_FOV,
                uTime: t,
                uSteps: q.steps,
                uReflectSteps: q.reflectSteps,
                uPrev: scene.read,
                uBlend: blend,
                uFrame: frameIndex
            }, scene.write);
            scene.swap();
            blend = TEMPORAL_BLEND;
            frameIndex++;
            const bloomTex = bloom.render(scene.read, { threshold: 0.18, knee: 0.2, scatter: 0.82 });
            kit.draw(compositeProgram, {
                uScene: scene.read,
                uBloom: bloomTex,
                uResolution: [gl.drawingBufferWidth, gl.drawingBufferHeight],
                uCamRot: rot,
                uTanHalfFov: TAN_HALF_FOV,
                uTime: t,
                uExposure: 1.3,
                uBloomStrength: 0.5,
                uMeteor: meteor ? [meteor.a[0], meteor.a[1], meteor.b[0], meteor.b[1]] : [0, 0, 0, 0],
                uMeteorProgress: meteor ? Math.min(1, meteor.age / meteor.life * 1.2) : 0,
                uMeteorAlpha: meteor ? Math.sin(Math.PI * Math.min(1, meteor.age / meteor.life)) : 0
            }, null);
        }

        return {
            frame,
            resize(w, h) {
                width = w;
                height = h;
                layout();
            },
            setQuality(level) {
                if (!QUALITY[level] || level === quality) return;
                quality = level;
                layout();
            },
            stats() {
                return { quality, sceneWidth: scene.width, sceneHeight: scene.height, steps: QUALITY[quality].steps, seed };
            },
            dispose() {
                kit.dispose();
            }
        };
    }

    window.AuraScreensavers.register('aurora', createAurora);
})();
