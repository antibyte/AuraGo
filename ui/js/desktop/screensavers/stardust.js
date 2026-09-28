(function () {
    'use strict';

    const QUALITY = {
        high: { size: 512, accum: 1.0, bloomLevels: 6 },
        medium: { size: 384, accum: 0.8, bloomLevels: 5 },
        low: { size: 256, accum: 0.62, bloomLevels: 4 }
    };
    const PALETTES = [
        { cool: [0.45, 0.66, 1.0], cool2: [0.86, 0.93, 1.0], hot: [1.0, 0.38, 0.78] },
        { cool: [1.0, 0.7, 0.4], cool2: [1.0, 0.92, 0.78], hot: [1.0, 0.32, 0.08] },
        { cool: [0.3, 1.0, 0.72], cool2: [0.62, 0.9, 1.0], hot: [0.72, 0.3, 1.0] }
    ];
    const TEXT_W = 1400;
    const TEXT_H = 420;
    const WORLD_W = 4.2;
    const LOGO_URL = '/aurago_logo_dark.png';
    const LOGO_EVERY_MINUTES = 3;

    const SIM_FS = `
uniform sampler2D uPos;
uniform sampler2D uVel;
uniform sampler2D uTarget;
uniform sampler2D uSeed;
uniform float uTime;
uniform float uDt;
uniform float uForm;
uniform float uGalaxy;
uniform float uBurst;
uniform float uHold;
uniform float uInit;

vec3 potential(vec3 p) {
    return vec3(gnoise(p), gnoise(p + vec3(31.3, -12.7, 4.1)), gnoise(p + vec3(-7.9, 19.2, -23.5)));
}

vec3 curlNoise(vec3 p) {
    const float e = 0.08;
    vec3 dx = vec3(e, 0.0, 0.0);
    vec3 dy = vec3(0.0, e, 0.0);
    vec3 dz = vec3(0.0, 0.0, e);
    vec3 px0 = potential(p - dx), px1 = potential(p + dx);
    vec3 py0 = potential(p - dy), py1 = potential(p + dy);
    vec3 pz0 = potential(p - dz), pz1 = potential(p + dz);
    return vec3((py1.z - py0.z) - (pz1.y - pz0.y),
                (pz1.x - pz0.x) - (px1.z - px0.z),
                (px1.y - px0.y) - (py1.x - py0.x)) / (2.0 * e);
}

void main() {
    ivec2 ij = ivec2(gl_FragCoord.xy);
    vec4 sd = texelFetch(uSeed, ij, 0);
    if (uInit > 0.5) {
        float a = sd.x * TAU * 3.0 + sd.y * 9.0;
        float r = 0.35 + pow(sd.y, 0.6) * 2.4;
        vec3 axis = normalize(vec3(0.15, 0.9, 0.4));
        vec3 u = normalize(cross(axis, vec3(0.0, 0.0, 1.0)));
        vec3 w = cross(axis, u);
        vec3 pos = (u * cos(a) + w * sin(a)) * r + axis * (sd.z - 0.5) * 0.25;
        fragColor = vec4(pos, 1.0);
        fragData1 = vec4(0.0);
        return;
    }
    vec3 pos = texelFetch(uPos, ij, 0).xyz;
    vec3 vel = texelFetch(uVel, ij, 0).xyz;
    vec4 tgt = texelFetch(uTarget, ij, 0);

    float form = clamp((uForm - sd.x * 0.45) / 0.55, 0.0, 1.0);
    form = form * form * (3.0 - 2.0 * form);
    float attract = form * tgt.w;

    vec3 axis = normalize(vec3(0.15, 0.9, 0.4));
    vec3 rel = pos - axis * dot(pos, axis);
    float rr = length(rel) + 0.06;
    vec3 tangential = cross(axis, rel) / rr;
    vec3 swirl = tangential * (0.95 / sqrt(rr)) - (rel / rr) * (rr - 1.5) * 0.4 - axis * dot(pos, axis) * 0.9;
    vec3 flow = curlNoise(pos * 0.55 + vec3(0.0, 0.0, uTime * 0.04)) * 0.42;
    vec3 freeVel = flow + swirl * uGalaxy;
    freeVel -= pos * max(length(pos) - 3.2, 0.0) * 0.6;

    vec3 springAcc = (tgt.xyz - pos) * 16.0 - vel * 6.2;
    vec3 flowAcc = (freeVel - vel) * 1.5;
    vec3 acc = mix(flowAcc, springAcc, attract);
    acc += curlNoise(pos * 2.6 + vec3(uTime * 0.25)) * 0.35 * attract * uHold;
    vec3 outward = normalize(pos + (sd.xyz - 0.5) * 0.4 + vec3(0.0, 0.0, 0.3));
    acc += outward * uBurst * (5.0 + 7.0 * sd.y) * tgt.w;

    vel += acc * uDt;
    pos += vel * uDt;
    fragColor = vec4(pos, 1.0);
    fragData1 = vec4(vel, 1.0);
}
`;

    const POINTS_VS = `#version 300 es
precision highp float;
precision highp sampler2D;
uniform sampler2D uPos;
uniform sampler2D uVel;
uniform sampler2D uSeed;
uniform sampler2D uTarget;
uniform sampler2D uTargetColor;
uniform mat4 uViewProj;
uniform int uSize;
uniform float uPointScale;
uniform float uForm;
uniform float uBrightness;
uniform vec3 uCool;
uniform vec3 uCool2;
uniform vec3 uHot;
out vec3 vColor;
void main() {
    ivec2 ij = ivec2(gl_VertexID % uSize, gl_VertexID / uSize);
    vec3 p = texelFetch(uPos, ij, 0).xyz;
    vec3 v = texelFetch(uVel, ij, 0).xyz;
    vec4 sd = texelFetch(uSeed, ij, 0);
    vec4 tgt = texelFetch(uTarget, ij, 0);
    vec4 tc = texelFetch(uTargetColor, ij, 0);
    vec4 clip = uViewProj * vec4(p, 1.0);
    gl_Position = clip;
    float sparkle = step(0.993, sd.w);
    gl_PointSize = clamp(uPointScale * (0.75 + sd.y * 1.1 + sparkle * 2.2) / max(clip.w, 0.1), 1.0, 14.0);
    float speed = length(v);
    vec3 cool = mix(uCool, uCool2, sd.z);
    vec3 col = mix(cool, uHot, smoothstep(0.6, 3.2, speed));
    float form = clamp((uForm - sd.x * 0.45) / 0.55, 0.0, 1.0);
    col = mix(col, tc.rgb * 1.1, tc.a * form * tgt.w);
    float depthFade = clamp(1.25 - clip.w * 0.08, 0.35, 1.2) * smoothstep(0.9, 2.6, clip.w);
    vColor = col * uBrightness * (0.55 + 0.9 * sd.y + sparkle * 5.0) * depthFade;
}`;

    const POINTS_FS = `#version 300 es
precision highp float;
in vec3 vColor;
out vec4 fragColor;
void main() {
    vec2 c = gl_PointCoord * 2.0 - 1.0;
    float r2 = dot(c, c);
    if (r2 > 1.0) discard;
    fragColor = vec4(vColor * exp(-r2 * 3.2), 1.0);
}`;

    const FADE_FS = `
uniform sampler2D uSource;
uniform float uFade;
void main() {
    fragColor = vec4(texture(uSource, vUv).rgb * uFade, 1.0);
}`;

    const COMPOSITE_FS = `
uniform sampler2D uAccum;
uniform sampler2D uBloom;
uniform vec2 uResolution;
uniform float uTime;
uniform float uExposure;
uniform float uBloomStrength;
uniform vec3 uTint;
void main() {
    vec3 col = texture(uAccum, vUv).rgb + texture(uBloom, vUv).rgb * uBloomStrength;
    float aspect = uResolution.x / uResolution.y;
    vec2 q = (vUv - 0.5) * vec2(aspect, 1.0);
    col += uTint * 0.012 * exp(-dot(q, q) * 2.5);
    col *= uExposure;
    col = acesTonemap(col);
    float vig = smoothstep(1.35, 0.3, length(q * vec2(0.8, 1.0)));
    col *= mix(0.65, 1.0, vig);
    col = linearToSrgb(col);
    float grain = (hash12(gl_FragCoord.xy + fract(uTime) * 63.1) - 0.5) * 0.012;
    fragColor = vec4(col + grain + screenDither(gl_FragCoord.xy), 1.0);
}`;

    function perspective(fovY, aspect, near, far) {
        const f = 1 / Math.tan(fovY / 2);
        const nf = 1 / (near - far);
        return [f / aspect, 0, 0, 0, 0, f, 0, 0, 0, 0, (far + near) * nf, -1, 0, 0, 2 * far * near * nf, 0];
    }

    function lookAt(eye, target) {
        const fz = [eye[0] - target[0], eye[1] - target[1], eye[2] - target[2]];
        const lz = Math.hypot(fz[0], fz[1], fz[2]);
        const z = fz.map(v => v / lz);
        const xRaw = [z[2], 0, -z[0]];
        const lx = Math.hypot(xRaw[0], xRaw[1], xRaw[2]) || 1;
        const x = xRaw.map(v => v / lx);
        const y = [z[1] * x[2] - z[2] * x[1], z[2] * x[0] - z[0] * x[2], z[0] * x[1] - z[1] * x[0]];
        return [
            x[0], y[0], z[0], 0,
            x[1], y[1], z[1], 0,
            x[2], y[2], z[2], 0,
            -(x[0] * eye[0] + x[1] * eye[1] + x[2] * eye[2]),
            -(y[0] * eye[0] + y[1] * eye[1] + y[2] * eye[2]),
            -(z[0] * eye[0] + z[1] * eye[1] + z[2] * eye[2]),
            1
        ];
    }

    function multiply(a, b) {
        const out = new Float32Array(16);
        for (let c = 0; c < 4; c++) {
            for (let r = 0; r < 4; r++) {
                out[c * 4 + r] = a[r] * b[c * 4] + a[4 + r] * b[c * 4 + 1] + a[8 + r] * b[c * 4 + 2] + a[12 + r] * b[c * 4 + 3];
            }
        }
        return out;
    }

    function textCandidates(text) {
        const canvas = document.createElement('canvas');
        canvas.width = TEXT_W;
        canvas.height = TEXT_H;
        const g = canvas.getContext('2d', { willReadFrequently: true });
        let size = 330;
        const family = 'Geist, "Segoe UI", system-ui, sans-serif';
        g.font = '600 ' + size + 'px ' + family;
        const measured = g.measureText(text).width;
        if (measured > TEXT_W * 0.92) {
            size *= (TEXT_W * 0.92) / measured;
            g.font = '600 ' + size + 'px ' + family;
        }
        g.fillStyle = '#fff';
        g.textAlign = 'center';
        g.textBaseline = 'middle';
        g.fillText(text, TEXT_W / 2, TEXT_H / 2 + size * 0.04);
        const data = g.getImageData(0, 0, TEXT_W, TEXT_H).data;
        const picks = [];
        for (let y = 1; y < TEXT_H - 1; y++) {
            for (let x = 1; x < TEXT_W - 1; x++) {
                const a = data[(y * TEXT_W + x) * 4 + 3];
                if (a < 40) continue;
                const idx = y * TEXT_W + x;
                picks.push(idx);
                if (a < 220 || data[((y - 2 < 0 ? y : y - 2) * TEXT_W + x) * 4 + 3] < 40 || data[(y * TEXT_W + Math.min(TEXT_W - 1, x + 2)) * 4 + 3] < 40) {
                    picks.push(idx, idx);
                }
            }
        }
        return { picks: Int32Array.from(picks), width: TEXT_W, height: TEXT_H, colors: null };
    }

    function logoCandidates(image) {
        const w = 640;
        const h = Math.round(w * image.naturalHeight / image.naturalWidth);
        const canvas = document.createElement('canvas');
        canvas.width = w;
        canvas.height = h;
        const g = canvas.getContext('2d', { willReadFrequently: true });
        g.drawImage(image, 0, 0, w, h);
        const data = g.getImageData(0, 0, w, h).data;
        const picks = [];
        for (let y = 0; y < h; y++) {
            for (let x = 0; x < w; x++) {
                if (x > w * 0.9 && y > h * 0.84) continue;
                const o = (y * w + x) * 4;
                const lum = (0.2126 * data[o] + 0.7152 * data[o + 1] + 0.0722 * data[o + 2]) / 255;
                const weight = Math.max(0, lum - 0.16);
                const copies = Math.floor(Math.sqrt(weight) * 4.5 + Math.random());
                for (let k = 0; k < copies; k++) picks.push(y * w + x);
            }
        }
        return { picks: Int32Array.from(picks), width: w, height: h, colors: data };
    }

    function loadImage(url) {
        return new Promise((resolve, reject) => {
            const img = new Image();
            img.onload = () => resolve(img);
            img.onerror = () => reject(new Error('stardust logo unavailable'));
            img.src = url;
        });
    }

    function createStardust(env) {
        const GL = window.AuraScreensaverGL;
        const kit = GL.create(env.canvas);
        const gl = kit.gl;
        if (!kit.caps.halfRender) {
            kit.dispose();
            throw new Error('float-render-unavailable');
        }
        const seed = (Math.random() * 0xffffffff) >>> 0;
        const rng = GL.createRng(seed);
        const palette = PALETTES[Math.floor(rng() * PALETTES.length)];
        const quality = QUALITY[env.quality] ? env.quality : 'high';
        const N = QUALITY[quality].size;
        const count = N * N;
        const stateFormat = kit.caps.floatRender ? 'rgba32f' : 'rgba16f';
        let state = kit.multiTarget(N, N, [stateFormat, stateFormat], { filter: 'nearest' });
        let next = kit.multiTarget(N, N, [stateFormat, stateFormat], { filter: 'nearest' });
        const seedData = new Float32Array(count * 4);
        for (let i = 0; i < seedData.length; i++) seedData[i] = rng();
        const seedTex = kit.dataTexture(N, N, 'rgba32f', seedData, { filter: 'nearest' });
        const targetData = new Float32Array(count * 4);
        const targetTex = kit.dataTexture(N, N, 'rgba32f', targetData, { filter: 'nearest' });
        const colorData = new Uint8Array(count * 4);
        const colorTex = kit.dataTexture(N, N, 'rgba8', colorData, { filter: 'nearest' });
        const simProgram = kit.program(SIM_FS, { name: 'stardust-sim', outputs: 2 });
        const pointsProgram = kit.program(POINTS_FS, { name: 'stardust-points', vertex: POINTS_VS, rawFragment: true });
        const fadeProgram = kit.program(FADE_FS, { name: 'stardust-fade', common: false });
        const compositeProgram = kit.program(COMPOSITE_FS, { name: 'stardust-composite' });
        const pointsVao = kit.vertexArray();
        let width = env.width;
        let height = env.height;
        let accum = null;
        let bloom = null;
        let logoShape = null;
        let initialized = false;
        let shapeKey = '';
        let phase = { name: 'drift', start: 0, pendingKey: '' };
        let logoLoading = null;
        let fontReady = !document.fonts;
        if (document.fonts && document.fonts.load) {
            Promise.race([document.fonts.load('600 330px Geist'), new Promise(resolve => setTimeout(resolve, 2500))])
                .catch(() => {})
                .then(() => { fontReady = true; });
        }
        const timeFormat = new Intl.DateTimeFormat(env.lang || 'en', { hour: '2-digit', minute: '2-digit' });

        function layout() {
            const q = QUALITY[quality];
            const aw = width * q.accum;
            const ah = height * q.accum;
            if (!accum) {
                accum = kit.doubleTarget(aw, ah, { format: 'rgba16f' });
                bloom = kit.bloom(accum.width, accum.height, { levels: q.bloomLevels });
            } else {
                accum.resize(aw, ah);
                bloom.resize(accum.width, accum.height);
            }
            accum.clear();
        }
        layout();

        function applyShape(shape, share) {
            const aspect = shape.height / shape.width;
            const worldW = shape.colors ? 5.5 : WORLD_W;
            for (let i = 0; i < count; i++) {
                const o = i * 4;
                if (!shape.picks.length || rng() > share) {
                    targetData[o + 3] = 0;
                    colorData[o + 3] = 0;
                    continue;
                }
                const idx = shape.picks[Math.floor(rng() * shape.picks.length)];
                const px = (idx % shape.width) + rng();
                const py = Math.floor(idx / shape.width) + rng();
                targetData[o] = (px / shape.width - 0.5) * worldW;
                targetData[o + 1] = (0.5 - py / shape.height) * worldW * aspect + (shape.colors ? 0.02 : 0.08);
                targetData[o + 2] = (rng() - 0.5) * (shape.colors ? 0.22 : 0.16);
                targetData[o + 3] = 1;
                if (shape.colors) {
                    const c = (Math.floor(py) * shape.width + Math.floor(px)) * 4;
                    colorData[o] = shape.colors[c];
                    colorData[o + 1] = shape.colors[c + 1];
                    colorData[o + 2] = shape.colors[c + 2];
                    colorData[o + 3] = 255;
                } else {
                    colorData[o + 3] = 0;
                }
            }
            targetTex.update(targetData);
            colorTex.update(colorData);
        }

        function wantedKey(now) {
            const seconds = now.getSeconds();
            const logoMinute = now.getMinutes() % LOGO_EVERY_MINUTES === 1;
            if (logoMinute && seconds >= 18 && seconds < 44 && logoShape) return 'logo';
            return 'time:' + timeFormat.format(now);
        }

        function ensureLogo() {
            if (logoShape || logoLoading) return;
            logoLoading = loadImage(env.versionedURL ? env.versionedURL(LOGO_URL) : LOGO_URL)
                .then(img => { logoShape = logoCandidates(img); })
                .catch(() => { logoShape = null; });
        }
        ensureLogo();

        function updatePhase(t) {
            const key = wantedKey(new Date());
            if (key !== shapeKey && phase.name !== 'burst' && phase.name !== 'drift') {
                phase = { name: shapeKey ? 'burst' : 'drift', start: t, pendingKey: key };
            }
            if (!shapeKey && phase.name === 'drift' && !phase.pendingKey) phase.pendingKey = key;
            const elapsed = t - phase.start;
            if (phase.name === 'burst' && elapsed > 0.45) {
                phase = { name: 'drift', start: t, pendingKey: phase.pendingKey };
            } else if (phase.name === 'drift') {
                const driftTime = !shapeKey ? 3.2 : (phase.pendingKey === 'logo' || shapeKey === 'logo' ? 3.4 : 1.1);
                if (elapsed > driftTime && (fontReady || elapsed > driftTime + 2.5)) {
                    shapeKey = phase.pendingKey || wantedKey(new Date());
                    if (shapeKey === 'logo') applyShape(logoShape, 0.94);
                    else applyShape(textCandidates(shapeKey.slice(5)), 0.76);
                    phase = { name: 'form', start: t, pendingKey: '' };
                }
            } else if (phase.name === 'form' && elapsed > 3.4) {
                phase = { name: 'hold', start: t, pendingKey: '' };
            }
        }

        function frame(t, dt) {
            const simDt = Math.min(dt, 1 / 30);
            updatePhase(t);
            const elapsed = t - phase.start;
            let form = 0;
            if (phase.name === 'form') form = Math.min(1, elapsed / 3.4);
            else if (phase.name === 'hold') form = 1;
            const galaxy = phase.name === 'hold' ? 0.22 : 1;
            const burst = phase.name === 'burst' ? 1 : 0;
            kit.draw(simProgram, {
                uPos: state.textures[0],
                uVel: state.textures[1],
                uTarget: targetTex,
                uSeed: seedTex,
                uTime: t,
                uDt: simDt,
                uForm: form,
                uGalaxy: galaxy,
                uBurst: burst,
                uHold: phase.name === 'hold' ? 1 : 0,
                uInit: initialized ? 0 : 1
            }, next);
            initialized = true;
            const swap = state;
            state = next;
            next = swap;

            const yaw = 0.32 * Math.sin(t * 0.045) + 0.08 * Math.sin(t * 0.13);
            const pitch = 0.1 * Math.sin(t * 0.037);
            const dist = 4.7 + 0.25 * Math.sin(t * 0.05);
            const eye = [Math.sin(yaw) * Math.cos(pitch) * dist, Math.sin(pitch) * dist, Math.cos(yaw) * Math.cos(pitch) * dist];
            const viewProj = multiply(perspective(0.72, width / height, 0.1, 40), lookAt(eye, [0, 0.05, 0]));

            kit.draw(fadeProgram, { uSource: accum.read, uFade: Math.pow(0.8, simDt * 60) }, accum.write);
            gl.enable(gl.BLEND);
            gl.blendFunc(gl.ONE, gl.ONE);
            gl.useProgram(pointsProgram.program);
            kit.applyUniforms(pointsProgram, {
                uPos: state.textures[0],
                uVel: state.textures[1],
                uSeed: seedTex,
                uTarget: targetTex,
                uTargetColor: colorTex,
                uViewProj: viewProj,
                uSize: N,
                uPointScale: (accum.height / 1080) * 1.45 * dist,
                uForm: form,
                uBrightness: 0.055 * Math.sqrt(262144 / count),
                uCool: palette.cool,
                uCool2: palette.cool2,
                uHot: palette.hot
            });
            kit.bindTarget(accum.write);
            gl.bindVertexArray(pointsVao);
            gl.drawArrays(gl.POINTS, 0, count);
            gl.disable(gl.BLEND);
            accum.swap();

            const bloomTex = bloom.render(accum.read, { threshold: 0.55, knee: 0.4, scatter: 0.85 });
            kit.draw(compositeProgram, {
                uAccum: accum.read,
                uBloom: bloomTex,
                uResolution: [gl.drawingBufferWidth, gl.drawingBufferHeight],
                uTime: t,
                uExposure: 1.1,
                uBloomStrength: 0.8,
                uTint: palette.cool
            }, null);
        }

        return {
            frame,
            resize(w, h) {
                width = w;
                height = h;
                layout();
            },
            stats() {
                return { quality, particles: count, phase: phase.name, shape: shapeKey, seed };
            },
            dispose() {
                kit.dispose();
            }
        };
    }

    window.AuraScreensavers.register('stardust', createStardust, { clock: 'date' });
})();
