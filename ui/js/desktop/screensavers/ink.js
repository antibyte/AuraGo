(function () {
    'use strict';

    const QUALITY = {
        high: { sim: 160, dye: 1024, iterations: 24, bloomLevels: 6 },
        medium: { sim: 128, dye: 768, iterations: 18, bloomLevels: 5 },
        low: { sim: 96, dye: 512, iterations: 14, bloomLevels: 4 }
    };
    const MAX_SPLATS = 12;
    const BRUSHES = 4;
    const PALETTES = [
        [[1.0, 0.05, 0.55], [0.0, 0.7, 1.0], [0.5, 0.1, 1.0], [1.0, 0.15, 0.25]],
        [[1.0, 0.35, 0.05], [1.0, 0.08, 0.05], [1.0, 0.18, 0.45], [0.95, 0.45, 0.08]],
        [[0.0, 0.85, 0.75], [0.05, 0.3, 1.0], [0.3, 0.6, 1.0], [0.0, 1.0, 0.5]],
        [[0.1, 1.0, 0.35], [0.0, 0.8, 1.0], [0.55, 0.15, 1.0], [1.0, 0.1, 0.6]]
    ];

    const CURL_FS = `
uniform sampler2D uVelocity;
uniform vec2 uTexel;
void main() {
    float L = texture(uVelocity, vUv - vec2(uTexel.x, 0.0)).y;
    float R = texture(uVelocity, vUv + vec2(uTexel.x, 0.0)).y;
    float T = texture(uVelocity, vUv + vec2(0.0, uTexel.y)).x;
    float B = texture(uVelocity, vUv - vec2(0.0, uTexel.y)).x;
    fragColor = vec4(0.5 * (R - L - T + B), 0.0, 0.0, 1.0);
}`;

    const VORTICITY_FS = `
uniform sampler2D uVelocity;
uniform sampler2D uCurl;
uniform vec2 uTexel;
uniform float uCurlStrength;
uniform float uDt;
void main() {
    float L = texture(uCurl, vUv - vec2(uTexel.x, 0.0)).x;
    float R = texture(uCurl, vUv + vec2(uTexel.x, 0.0)).x;
    float T = texture(uCurl, vUv + vec2(0.0, uTexel.y)).x;
    float B = texture(uCurl, vUv - vec2(0.0, uTexel.y)).x;
    float C = texture(uCurl, vUv).x;
    vec2 force = 0.5 * vec2(abs(T) - abs(B), abs(R) - abs(L));
    force /= length(force) + 1e-4;
    force *= uCurlStrength * C;
    force.y *= -1.0;
    vec2 vel = texture(uVelocity, vUv).xy + force * uDt;
    fragColor = vec4(clamp(vel, -1200.0, 1200.0), 0.0, 1.0);
}`;

    const DIVERGENCE_FS = `
uniform sampler2D uVelocity;
uniform vec2 uTexel;
void main() {
    vec2 uvL = vUv - vec2(uTexel.x, 0.0);
    vec2 uvR = vUv + vec2(uTexel.x, 0.0);
    vec2 uvT = vUv + vec2(0.0, uTexel.y);
    vec2 uvB = vUv - vec2(0.0, uTexel.y);
    vec2 C = texture(uVelocity, vUv).xy;
    float L = uvL.x < 0.0 ? -C.x : texture(uVelocity, uvL).x;
    float R = uvR.x > 1.0 ? -C.x : texture(uVelocity, uvR).x;
    float T = uvT.y > 1.0 ? -C.y : texture(uVelocity, uvT).y;
    float B = uvB.y < 0.0 ? -C.y : texture(uVelocity, uvB).y;
    fragColor = vec4(0.5 * (R - L + T - B), 0.0, 0.0, 1.0);
}`;

    const SCALE_FS = `
uniform sampler2D uSource;
uniform float uScale;
void main() {
    fragColor = texture(uSource, vUv) * uScale;
}`;

    const PRESSURE_FS = `
uniform sampler2D uPressure;
uniform sampler2D uDivergence;
uniform vec2 uTexel;
void main() {
    float L = texture(uPressure, vUv - vec2(uTexel.x, 0.0)).x;
    float R = texture(uPressure, vUv + vec2(uTexel.x, 0.0)).x;
    float T = texture(uPressure, vUv + vec2(0.0, uTexel.y)).x;
    float B = texture(uPressure, vUv - vec2(0.0, uTexel.y)).x;
    float div = texture(uDivergence, vUv).x;
    fragColor = vec4((L + R + B + T - div) * 0.25, 0.0, 0.0, 1.0);
}`;

    const GRADIENT_FS = `
uniform sampler2D uPressure;
uniform sampler2D uVelocity;
uniform vec2 uTexel;
void main() {
    float L = texture(uPressure, vUv - vec2(uTexel.x, 0.0)).x;
    float R = texture(uPressure, vUv + vec2(uTexel.x, 0.0)).x;
    float T = texture(uPressure, vUv + vec2(0.0, uTexel.y)).x;
    float B = texture(uPressure, vUv - vec2(0.0, uTexel.y)).x;
    vec2 vel = texture(uVelocity, vUv).xy - vec2(R - L, T - B);
    fragColor = vec4(vel, 0.0, 1.0);
}`;

    const ADVECT_FS = `
uniform sampler2D uVelocity;
uniform sampler2D uSource;
uniform vec2 uSimTexel;
uniform float uDt;
uniform float uDissipation;
uniform float uFloor;
void main() {
    vec2 coord = vUv - uDt * texture(uVelocity, vUv).xy * uSimTexel;
    vec4 result = texture(uSource, coord) / (1.0 + uDissipation * uDt);
    fragColor = sign(result) * max(abs(result) - uFloor * uDt, vec4(0.0));
}`;

    const SPLAT_FS = `
uniform sampler2D uTarget;
uniform float uAspect;
uniform int uCount;
uniform vec4 uSplats[${MAX_SPLATS}];
uniform vec4 uValues[${MAX_SPLATS}];
void main() {
    vec4 base = texture(uTarget, vUv);
    vec3 add = vec3(0.0);
    for (int i = 0; i < ${MAX_SPLATS}; i++) {
        if (i >= uCount) break;
        vec2 p = vUv - uSplats[i].xy;
        p.x *= uAspect;
        add += exp(-dot(p, p) / uSplats[i].z) * uValues[i].xyz;
    }
    fragColor = vec4(base.xyz + add, 1.0);
}`;

    const DISPLAY_FS = `
uniform sampler2D uDye;
uniform sampler2D uBloom;
uniform sampler2D uRays;
uniform vec2 uDyeTexel;
uniform vec2 uResolution;
uniform float uTime;
uniform float uBloomStrength;
uniform float uExposure;

float luma(vec3 c) { return dot(c, vec3(0.2126, 0.7152, 0.0722)); }

vec3 hueTonemap(vec3 col) {
    float L = luma(col);
    float Lt = acesTonemap(vec3(L)).x;
    vec3 tinted = col * (Lt / max(L, 1e-4));
    tinted = mix(tinted, vec3(Lt), smoothstep(2.5, 10.0, L) * 0.3);
    return clamp(tinted, 0.0, 1.0);
}

void main() {
    vec3 c = texture(uDye, vUv).rgb;
    float l = luma(texture(uDye, vUv - vec2(uDyeTexel.x, 0.0)).rgb);
    float r = luma(texture(uDye, vUv + vec2(uDyeTexel.x, 0.0)).rgb);
    float t = luma(texture(uDye, vUv + vec2(0.0, uDyeTexel.y)).rgb);
    float b = luma(texture(uDye, vUv - vec2(0.0, uDyeTexel.y)).rgb);
    float edge = clamp(length(vec2(r - l, t - b)) * 5.0, 0.0, 1.0);
    float density = smoothstep(0.012, 0.32, luma(c));
    vec3 col = c * mix(0.18, 1.0, density) * (0.85 + 1.6 * edge);
    col = max(mix(vec3(luma(col)), col, 1.25), vec3(0.0));
    col += texture(uBloom, vUv).rgb * uBloomStrength;
    col += texture(uRays, vUv).rgb * 0.3;
    col *= uExposure;
    col = hueTonemap(col);
    float aspect = uResolution.x / uResolution.y;
    float vig = smoothstep(1.3, 0.3, length((vUv - 0.5) * vec2(aspect * 0.8, 1.0)));
    col *= mix(0.7, 1.0, vig);
    col = linearToSrgb(col);
    float grain = (hash12(gl_FragCoord.xy + fract(uTime) * 71.3) - 0.5) * 0.01;
    fragColor = vec4(col + grain + screenDither(gl_FragCoord.xy), 1.0);
}`;

    const RAYS_FS = `
uniform sampler2D uSource;
uniform vec2 uCenter;
uniform float uDecay;
void main() {
    vec2 dir = (vUv - uCenter) * (1.0 / 40.0);
    vec2 uv = vUv;
    vec3 acc = vec3(0.0);
    float weight = 1.0;
    for (int i = 0; i < 40; i++) {
        uv -= dir;
        vec3 s = texture(uSource, uv).rgb;
        acc += max(s - 0.05, vec3(0.0)) * weight;
        weight *= uDecay;
    }
    fragColor = vec4(acc * (1.0 / 40.0), 1.0);
}`;

    function mixColor(a, b, f) {
        return [a[0] + (b[0] - a[0]) * f, a[1] + (b[1] - a[1]) * f, a[2] + (b[2] - a[2]) * f];
    }

    function createInk(env) {
        const GL = window.AuraScreensaverGL;
        const kit = GL.create(env.canvas);
        if (!kit.caps.halfRender) {
            kit.dispose();
            throw new Error('float-render-unavailable');
        }
        const seed = (Math.random() * 0xffffffff) >>> 0;
        const rng = GL.createRng(seed);
        const programs = {
            curl: kit.program(CURL_FS, { name: 'ink-curl', common: false }),
            vorticity: kit.program(VORTICITY_FS, { name: 'ink-vorticity', common: false }),
            divergence: kit.program(DIVERGENCE_FS, { name: 'ink-divergence', common: false }),
            scale: kit.program(SCALE_FS, { name: 'ink-scale', common: false }),
            pressure: kit.program(PRESSURE_FS, { name: 'ink-pressure', common: false }),
            gradient: kit.program(GRADIENT_FS, { name: 'ink-gradient', common: false }),
            advect: kit.program(ADVECT_FS, { name: 'ink-advect', common: false }),
            splat: kit.program(SPLAT_FS, { name: 'ink-splat', common: false }),
            rays: kit.program(RAYS_FS, { name: 'ink-rays', common: false }),
            display: kit.program(DISPLAY_FS, { name: 'ink-display' })
        };
        let quality = QUALITY[env.quality] ? env.quality : 'high';
        let width = env.width;
        let height = env.height;
        let velocity;
        let dye;
        let pressure;
        let divergence;
        let curl;
        let rays;
        let bloom = null;
        let paletteIndex = Math.floor(rng() * PALETTES.length);
        let paletteNext = (paletteIndex + 1 + Math.floor(rng() * (PALETTES.length - 1))) % PALETTES.length;
        let paletteMix = 0;
        let nextBurst = 4 + rng() * 4;
        const brushes = [];
        for (let i = 0; i < BRUSHES; i++) {
            brushes.push({ slot: i, active: false, until: rng() * 2.5, pos: [0.5, 0.5], angle: 0, speed: 0, turn: 0, end: 0 });
        }

        function gridSize(resolution) {
            const aspect = width / height;
            return aspect >= 1
                ? [Math.round(resolution * aspect), resolution]
                : [resolution, Math.round(resolution / aspect)];
        }

        function layout() {
            const q = QUALITY[quality];
            const sim = gridSize(q.sim);
            const dyeSize = gridSize(q.dye);
            if (!velocity) {
                velocity = kit.doubleTarget(sim[0], sim[1], { format: 'rgba16f' });
                pressure = kit.doubleTarget(sim[0], sim[1], { format: 'rgba16f', filter: 'nearest' });
                divergence = kit.target(sim[0], sim[1], { format: 'rgba16f', filter: 'nearest' });
                curl = kit.target(sim[0], sim[1], { format: 'rgba16f', filter: 'nearest' });
                dye = kit.doubleTarget(dyeSize[0], dyeSize[1], { format: 'rgba16f' });
                rays = kit.target(Math.round(dyeSize[0] / 2), Math.round(dyeSize[1] / 2), { format: 'rgba16f' });
                bloom = kit.bloom(dyeSize[0], dyeSize[1], { levels: q.bloomLevels });
            } else {
                velocity.resize(sim[0], sim[1]);
                pressure.resize(sim[0], sim[1]);
                divergence.resize(sim[0], sim[1]);
                curl.resize(sim[0], sim[1]);
                dye.resize(dyeSize[0], dyeSize[1]);
                rays.resize(Math.round(dyeSize[0] / 2), Math.round(dyeSize[1] / 2));
                bloom.resize(dyeSize[0], dyeSize[1]);
            }
            velocity.clear();
            pressure.clear();
            dye.clear();
        }
        layout();

        function paletteColor(slot, t) {
            const a = PALETTES[paletteIndex][slot % 4];
            const b = PALETTES[paletteNext][slot % 4];
            const base = mixColor(a, b, paletteMix);
            const shimmer = 0.85 + 0.15 * Math.sin(t * 0.7 + slot * 1.9);
            return base.map(c => c * shimmer);
        }

        function collectSplats(t, dt) {
            const splats = [];
            const aspect = width / height;
            brushes.forEach(b => {
                if (!b.active) {
                    if (t < b.until) return;
                    b.active = true;
                    b.pos = [0.15 + rng() * 0.7, 0.15 + rng() * 0.7];
                    b.angle = rng() * Math.PI * 2;
                    b.speed = 0.3 + rng() * 0.4;
                    b.turn = (rng() - 0.5) * 3.2;
                    b.end = t + 0.8 + rng() * 1.1;
                }
                b.turn += (rng() - 0.5) * dt * 5;
                b.angle += b.turn * dt;
                const vx = Math.cos(b.angle) * b.speed;
                const vy = Math.sin(b.angle) * b.speed;
                b.pos[0] += vx * dt / aspect;
                b.pos[1] += vy * dt;
                if (b.pos[0] < 0.06 || b.pos[0] > 0.94) b.angle = Math.PI - b.angle;
                if (b.pos[1] < 0.06 || b.pos[1] > 0.94) b.angle = -b.angle;
                b.pos[0] = Math.min(0.94, Math.max(0.06, b.pos[0]));
                b.pos[1] = Math.min(0.94, Math.max(0.06, b.pos[1]));
                const life = Math.min(1, (b.end - t) * 3);
                if (t > b.end) {
                    b.active = false;
                    b.until = t + 0.4 + rng() * 1.8;
                    return;
                }
                const color = paletteColor(b.slot, t).map(c => c * 0.3 * life);
                splats.push({ x: b.pos[0], y: b.pos[1], radius: 0.00024, force: [vx * 1300, vy * 1300], color });
            });
            if (t > nextBurst) {
                nextBurst = t + 9 + rng() * 9;
                const jets = 1 + Math.floor(rng() * 3);
                const slot = Math.floor(rng() * 4);
                for (let i = 0; i < jets && splats.length < MAX_SPLATS; i++) {
                    const side = Math.floor(rng() * 4);
                    const along = 0.2 + rng() * 0.6;
                    const origin = [[along, 0.04], [along, 0.96], [0.04, along], [0.96, along]][side];
                    const target = [0.35 + rng() * 0.3, 0.35 + rng() * 0.3];
                    const len = Math.hypot(target[0] - origin[0], target[1] - origin[1]) || 1;
                    const speed = 2600 + rng() * 1400;
                    const color = paletteColor(slot + i, t).map(c => c * (1.6 + rng() * 0.8));
                    splats.push({
                        x: origin[0],
                        y: origin[1],
                        radius: 0.00045 + rng() * 0.00025,
                        force: [(target[0] - origin[0]) / len * speed * aspect, (target[1] - origin[1]) / len * speed],
                        color
                    });
                }
            }
            return splats.slice(0, MAX_SPLATS);
        }

        function applySplats(splats) {
            if (!splats.length) return;
            const points = new Float32Array(MAX_SPLATS * 4);
            const forces = new Float32Array(MAX_SPLATS * 4);
            const colors = new Float32Array(MAX_SPLATS * 4);
            splats.forEach((s, i) => {
                points.set([s.x, s.y, s.radius, 0], i * 4);
                forces.set([s.force[0], s.force[1], 0, 0], i * 4);
                colors.set([s.color[0], s.color[1], s.color[2], 0], i * 4);
            });
            const aspect = width / height;
            kit.draw(programs.splat, { uTarget: velocity.read, uAspect: aspect, uCount: splats.length, uSplats: points, uValues: forces }, velocity.write);
            velocity.swap();
            kit.draw(programs.splat, { uTarget: dye.read, uAspect: aspect, uCount: splats.length, uSplats: points, uValues: colors }, dye.write);
            dye.swap();
        }

        function step(dt) {
            const q = QUALITY[quality];
            const texel = velocity.texel;
            kit.draw(programs.curl, { uVelocity: velocity.read, uTexel: texel }, curl);
            kit.draw(programs.vorticity, { uVelocity: velocity.read, uCurl: curl, uTexel: texel, uCurlStrength: 14, uDt: dt }, velocity.write);
            velocity.swap();
            kit.draw(programs.divergence, { uVelocity: velocity.read, uTexel: texel }, divergence);
            kit.draw(programs.scale, { uSource: pressure.read, uScale: 0.8 }, pressure.write);
            pressure.swap();
            for (let i = 0; i < q.iterations; i++) {
                kit.draw(programs.pressure, { uPressure: pressure.read, uDivergence: divergence, uTexel: texel }, pressure.write);
                pressure.swap();
            }
            kit.draw(programs.gradient, { uPressure: pressure.read, uVelocity: velocity.read, uTexel: texel }, velocity.write);
            velocity.swap();
            kit.draw(programs.advect, { uVelocity: velocity.read, uSource: velocity.read, uSimTexel: texel, uDt: dt, uDissipation: 0.18, uFloor: 0.0 }, velocity.write);
            velocity.swap();
            kit.draw(programs.advect, { uVelocity: velocity.read, uSource: dye.read, uSimTexel: texel, uDt: dt, uDissipation: 0.6, uFloor: 0.01 }, dye.write);
            dye.swap();
        }

        function frame(t, dt) {
            const gl = kit.gl;
            const simDt = Math.min(dt, 1 / 30);
            paletteMix += simDt / 45;
            if (paletteMix >= 1) {
                paletteMix = 0;
                paletteIndex = paletteNext;
                paletteNext = (paletteIndex + 1 + Math.floor(rng() * (PALETTES.length - 1))) % PALETTES.length;
            }
            applySplats(collectSplats(t, simDt));
            step(simDt);
            const bloomTex = bloom.render(dye.read, { threshold: 0.42, knee: 0.3, scatter: 0.8 });
            kit.draw(programs.rays, { uSource: bloomTex, uCenter: [0.5, 0.5], uDecay: 0.955 }, rays);
            kit.draw(programs.display, {
                uDye: dye.read,
                uBloom: bloomTex,
                uRays: rays,
                uDyeTexel: dye.texel,
                uResolution: [gl.drawingBufferWidth, gl.drawingBufferHeight],
                uTime: t,
                uBloomStrength: 0.8,
                uExposure: 1.5
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
                return { quality, sim: [velocity.width, velocity.height], dye: [dye.width, dye.height], seed, palette: paletteIndex };
            },
            dispose() {
                kit.dispose();
            }
        };
    }

    window.AuraScreensavers.register('ink', createInk);
})();
