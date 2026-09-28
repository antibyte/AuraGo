(function () {
    'use strict';

    // Units: Schwarzschild radius = 1. Photon paths follow the exact orbit equation
    // u'' + u = 1.5 u^2 expressed as a central acceleration -1.5 h^2 r / |r|^5.
    const QUALITY = {
        high: { scale: 0.6, steps: 320, stepScale: 0.75, bloomLevels: 7 },
        medium: { scale: 0.5, steps: 230, stepScale: 1.0, bloomLevels: 6 },
        low: { scale: 0.4, steps: 150, stepScale: 1.4, bloomLevels: 5 }
    };
    const DISK_INNER = 3.0;
    const DISK_OUTER = 13.0;
    const TAN_HALF_FOV = 0.42;

    const SCENE_FS = `
uniform vec2 uResolution;
uniform vec3 uCamPos;
uniform mat3 uCamRot;
uniform float uTanHalfFov;
uniform float uTime;
uniform int uSteps;
uniform float uStepScale;
uniform float uEscape;
uniform float uDiskInner;
uniform float uDiskOuter;
uniform float uSpin;
uniform float uDiskGain;
uniform float uSeed;
uniform sampler2D uPrev;
uniform float uBlend;
uniform vec2 uJitter;

vec3 blackbody(float t) {
    vec3 c = vec3(0.9, 0.12, 0.02);
    c = mix(c, vec3(1.0, 0.36, 0.06), smoothstep(0.15, 0.4, t));
    c = mix(c, vec3(1.0, 0.68, 0.32), smoothstep(0.4, 0.72, t));
    c = mix(c, vec3(1.0, 0.93, 0.82), smoothstep(0.72, 1.05, t));
    c = mix(c, vec3(0.78, 0.86, 1.0), smoothstep(1.05, 1.7, t));
    return c;
}

float diskPattern(float r, float ang, float seed) {
    vec3 q = vec3(cos(ang) * 1.5, sin(ang) * 1.5, r * 3.2 + seed);
    float n = fbm(q, 5);
    float fine = gnoise(vec3(cos(ang) * 6.0, sin(ang) * 6.0, r * 11.0 + seed * 1.7));
    return clamp(0.5 + n * 1.1 + fine * 0.18, 0.0, 1.0);
}

vec4 diskSample(vec3 p, vec3 rayDir) {
    float r = length(p.xz);
    float x = (r - uDiskInner) / (uDiskOuter - uDiskInner);
    float ang = atan(p.z, p.x);
    float omega = uSpin * pow(max(r, 1.0), -1.5);
    const float period = 18.0;
    float ph1 = fract(uTime / period);
    float ph2 = fract(uTime / period + 0.5);
    float w1 = 1.0 - abs(2.0 * ph1 - 1.0);
    float n = diskPattern(r, ang - omega * ph1 * period, uSeed) * w1
            + diskPattern(r, ang - omega * ph2 * period, uSeed + 5.3) * (1.0 - w1);

    float profile = smoothstep(0.0, 0.035, x) * (1.0 - smoothstep(0.45, 1.0, x));
    float ratio = uDiskInner / r;
    float temp = pow(max(ratio, 0.0), 0.75) * pow(max(1.0 - sqrt(ratio) * 0.97, 0.0), 0.25) * 1.75;

    vec3 flow = normalize(vec3(-p.z, 0.0, p.x));
    float beta = min(sqrt(0.5 / max(r - 1.0, 0.2)), 0.72);
    float gamma = inversesqrt(1.0 - beta * beta);
    float cosT = dot(flow, -rayDir);
    float delta = 1.0 / (gamma * (1.0 - beta * cosT));
    float grav = sqrt(max(1.0 - 1.0 / r, 0.0));
    float shift = delta * grav;
    float tObs = temp * shift;
    float lanes = smoothstep(0.18, 0.62, n);
    float clumps = 0.55 + 0.9 * smoothstep(-0.3, 0.5, gnoise(vec3(cos(ang - omega * uTime * 0.35) * 2.6, sin(ang - omega * uTime * 0.35) * 2.6, r * 0.9 + uSeed)));
    float brightness = pow(max(shift, 0.0), 2.5) * pow(max(tObs, 0.0), 1.6) * uDiskGain * (0.25 + 1.35 * n * n) * clumps;
    float alpha = clamp(profile * (0.2 + 0.8 * lanes), 0.0, 0.96);
    return vec4(blackbody(tObs) * brightness * profile, alpha);
}

vec3 nebula(vec3 d) {
    vec3 bandAxis = normalize(vec3(0.25, 0.93, 0.28));
    float band = exp(-pow(dot(d, bandAxis), 2.0) * 6.0);
    float n1 = fbm(d * 2.3 + 11.0, 4);
    float n2 = fbm(d * 6.0 - 3.0, 3);
    vec3 c = vec3(0.0012, 0.0016, 0.0035);
    c += vec3(0.022, 0.016, 0.034) * band * smoothstep(-0.25, 0.55, n1) * (0.5 + 0.9 * max(n2 + 0.3, 0.0));
    c += vec3(0.045, 0.018, 0.01) * band * pow(max(n2, 0.0), 2.0);
    c += vec3(0.004, 0.009, 0.02) * smoothstep(0.05, 0.6, n1);
    float dark = smoothstep(0.1, 0.45, fbm(d * 9.0 + 7.0, 3));
    return c * (1.0 - 0.55 * dark * band);
}

void main() {
    vec2 frag = gl_FragCoord.xy + uJitter;
    vec2 ndc = (frag / uResolution) * 2.0 - 1.0;
    ndc.x *= uResolution.x / uResolution.y;
    vec3 rd = normalize(uCamRot * vec3(ndc * uTanHalfFov, 1.0));
    vec3 pos = uCamPos;
    vec3 vel = rd;
    vec3 L = cross(pos, vel);
    float h2 = dot(L, L);
    vec3 emission = vec3(0.0);
    float trans = 1.0;
    bool captured = false;
    for (int i = 0; i < 480; i++) {
        if (i >= uSteps) break;
        float r2 = dot(pos, pos);
        float r = sqrt(r2);
        float dt = clamp(0.06 * r * uStepScale * clamp(r / 4.5, 0.3, 1.0), 0.008, 1.8);
        vec3 acc = -1.5 * h2 * pos / (r2 * r2 * r);
        vec3 vHalf = vel + acc * (0.5 * dt);
        vec3 np = pos + vHalf * dt;
        float nr2 = dot(np, np);
        float nr = sqrt(nr2);
        vec3 nacc = -1.5 * h2 * np / (nr2 * nr2 * nr);
        vel = vHalf + nacc * (0.5 * dt);
        if (pos.y * np.y < 0.0) {
            float f = pos.y / (pos.y - np.y);
            vec3 hit = mix(pos, np, f);
            float hr = length(hit.xz);
            if (hr > uDiskInner && hr < uDiskOuter) {
                vec4 d = diskSample(hit, normalize(vel));
                emission += trans * d.rgb;
                trans *= 1.0 - d.a;
            }
        }
        pos = np;
        if (nr < 1.0) { captured = true; break; }
        if (nr > uEscape && dot(pos, vel) > 0.0) break;
        if (trans < 0.01) break;
    }
    if (captured) trans = 0.0;
    vec3 dir = normalize(vel);
    vec3 current = emission + nebula(dir) * trans;
    vec3 previous = texture(uPrev, gl_FragCoord.xy / uResolution).rgb;
    fragColor = vec4(mix(previous, current, uBlend), 1.0);
    fragData1 = vec4(dir - rd, trans);
}
`;

    const COMPOSITE_FS = `
uniform sampler2D uScene;
uniform sampler2D uDir;
uniform sampler2D uBloom;
uniform vec2 uResolution;
uniform float uTime;
uniform float uExposure;
uniform float uBloomStrength;
uniform float uAberration;
uniform float uPixelAngle;
uniform mat3 uCamRot;
uniform float uTanHalfFov;
uniform vec3 uCamPos;

vec3 starLayer(vec3 d, float scale, float threshold, float seed, float pixelCells, float gain) {
    vec3 p = d * scale;
    vec3 cell = floor(p);
    vec3 h = hash33(cell + seed);
    if (h.x < threshold) return vec3(0.0);
    vec3 center = cell + 0.5 + (h - 0.5) * 0.45;
    float dist = length(p - center);
    float size = max(pixelCells * 0.7, 0.035);
    float twinkle = 0.8 + 0.2 * sin(uTime * (0.8 + h.z * 2.5) + h.x * 80.0);
    float mag = pow((h.x - threshold) / (1.0 - threshold), 4.0);
    vec3 tint = mix(vec3(0.68, 0.8, 1.0), vec3(1.0, 0.82, 0.62), h.y);
    return tint * exp(-dist * dist / (size * size)) * twinkle * (0.05 + 2.8 * mag) * gain;
}

void main() {
    vec2 center = vUv - 0.5;
    vec2 off = center * uAberration;
    vec3 col;
    col.r = texture(uScene, vUv + off).r;
    col.g = texture(uScene, vUv).g;
    col.b = texture(uScene, vUv - off).b;
    vec4 dirData = texture(uDir, vUv);
    if (dirData.w > 0.003) {
        vec2 ndc = vUv * 2.0 - 1.0;
        ndc.x *= uResolution.x / uResolution.y;
        vec3 primary = normalize(uCamRot * vec3(ndc * uTanHalfFov, 1.0));
        vec3 d = normalize(primary + dirData.xyz);
        float s0 = dot(uCamPos, primary);
        vec3 closest = uCamPos - primary * s0;
        float b = length(closest);
        if (b > 4.5) {
            float alpha = (1.0 / b + 1.4726 / (b * b) + 2.6667 / (b * b * b)) * (1.0 - s0 / length(uCamPos));
            vec3 weak = normalize(primary * cos(alpha) - (closest / b) * sin(alpha));
            d = normalize(mix(d, weak, smoothstep(4.5, 6.5, b)));
        }
        vec3 stars = starLayer(d, 60.0, 0.94, 3.0, uPixelAngle * 60.0, 1.0)
                   + starLayer(d, 140.0, 0.88, 29.0, uPixelAngle * 140.0, 0.4);
        col += stars * dirData.w;
    }
    vec3 bloom;
    bloom.r = texture(uBloom, vUv + off * 1.6).r;
    bloom.g = texture(uBloom, vUv).g;
    bloom.b = texture(uBloom, vUv - off * 1.6).b;
    col += bloom * uBloomStrength;
    col *= uExposure;
    col = acesTonemap(col);
    float aspect = uResolution.x / uResolution.y;
    float vig = smoothstep(1.25, 0.25, length(center * vec2(aspect * 0.85, 1.0)));
    col *= mix(0.6, 1.0, vig);
    col = linearToSrgb(col);
    float grain = (hash12(gl_FragCoord.xy + fract(uTime) * 57.3) - 0.5) * 0.016;
    fragColor = vec4(col + grain + screenDither(gl_FragCoord.xy), 1.0);
}
`;

    function cross(a, b) {
        return [a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0]];
    }

    function normalize(v) {
        const l = Math.hypot(v[0], v[1], v[2]) || 1;
        return [v[0] / l, v[1] / l, v[2] / l];
    }

    function lookAtBasis(pos, target, roll) {
        const forward = normalize([target[0] - pos[0], target[1] - pos[1], target[2] - pos[2]]);
        const right = normalize(cross([0, 1, 0], forward));
        const up = cross(forward, right);
        const c = Math.cos(roll);
        const s = Math.sin(roll);
        const r2 = [right[0] * c + up[0] * s, right[1] * c + up[1] * s, right[2] * c + up[2] * s];
        const u2 = [up[0] * c - right[0] * s, up[1] * c - right[1] * s, up[2] * c - right[2] * s];
        return new Float32Array([r2[0], r2[1], r2[2], u2[0], u2[1], u2[2], forward[0], forward[1], forward[2]]);
    }

    function halton(index, base) {
        let result = 0;
        let f = 1 / base;
        let i = index;
        while (i > 0) {
            result += f * (i % base);
            i = Math.floor(i / base);
            f /= base;
        }
        return result;
    }

    function createEventHorizon(env) {
        const GL = window.AuraScreensaverGL;
        const kit = GL.create(env.canvas);
        const seed = (Math.random() * 0xffffffff) >>> 0;
        const rng = GL.createRng(seed);
        const sceneProgram = kit.program(SCENE_FS, { name: 'event-horizon-scene', outputs: 2 });
        const compositeProgram = kit.program(COMPOSITE_FS, { name: 'event-horizon-composite' });
        let quality = QUALITY[env.quality] ? env.quality : 'high';
        let width = env.width;
        let height = env.height;
        const dirFormat = kit.caps.floatRender && kit.caps.floatLinear ? 'rgba32f' : 'rgba16f';
        let scene = kit.multiTarget(1, 1, ['rgba16f', dirFormat]);
        let history = kit.multiTarget(1, 1, ['rgba16f', dirFormat]);
        let blend = 1;
        let frameIndex = 0;
        let bloom = null;
        const azimuth0 = rng() * Math.PI * 2;
        const phaseA = rng() * 10;
        const phaseB = rng() * 10;
        const noiseSeed = rng() * 40;
        const direction = rng() < 0.5 ? 1 : -1;

        function layout() {
            const q = QUALITY[quality];
            scene.resize(width * q.scale, height * q.scale);
            history.resize(width * q.scale, height * q.scale);
            blend = 1;
            if (!bloom) bloom = kit.bloom(scene.width, scene.height, { levels: q.bloomLevels });
            else bloom.resize(scene.width, scene.height);
        }
        layout();

        function frame(t) {
            const gl = kit.gl;
            const q = QUALITY[quality];
            const radius = 26 + 6 * Math.sin(t * 0.021 + phaseA);
            const az = azimuth0 + direction * t * 0.02;
            const el = 0.07 + 0.13 * (0.5 + 0.5 * Math.sin(t * 0.026 + phaseB));
            const pos = [radius * Math.cos(el) * Math.cos(az), radius * Math.sin(el), radius * Math.cos(el) * Math.sin(az)];
            const roll = 0.14 + 0.08 * Math.sin(t * 0.017 + phaseA);
            const rot = lookAtBasis(pos, [0, 0.35, 0], roll);
            kit.draw(sceneProgram, {
                uResolution: [scene.width, scene.height],
                uCamPos: pos,
                uCamRot: rot,
                uTanHalfFov: TAN_HALF_FOV,
                uTime: t,
                uSteps: q.steps,
                uStepScale: q.stepScale,
                uEscape: Math.max(42, radius * 1.6),
                uDiskInner: DISK_INNER,
                uDiskOuter: DISK_OUTER,
                uSpin: 2.6,
                uDiskGain: 2.0,
                uSeed: noiseSeed,
                uPrev: history.textures[0],
                uBlend: blend,
                uJitter: [halton(frameIndex % 16 + 1, 2) - 0.5, halton(frameIndex % 16 + 1, 3) - 0.5]
            }, scene);
            frameIndex++;
            blend = 0.3;
            const bloomTex = bloom.render(scene.textures[0], { threshold: 0.8, knee: 0.6, scatter: 0.88 });
            kit.draw(compositeProgram, {
                uScene: scene.textures[0],
                uDir: scene.textures[1],
                uBloom: bloomTex,
                uResolution: [gl.drawingBufferWidth, gl.drawingBufferHeight],
                uTime: t,
                uExposure: 1.0,
                uBloomStrength: 0.55,
                uAberration: 0.004,
                uPixelAngle: 2 * TAN_HALF_FOV / gl.drawingBufferHeight,
                uCamRot: rot,
                uTanHalfFov: TAN_HALF_FOV,
                uCamPos: pos
            }, null);
            const swap = history;
            history = scene;
            scene = swap;
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

    window.AuraScreensavers.register('event_horizon', createEventHorizon);
})();
