/* Aura Drift - post processing for the three.js renderer (section 4, T4.2).
 *
 * Pipeline (tier gl.post): RenderPass into a HalfFloat target (MSAA from gl.msaa) -> GTAO (ultra)
 * -> bloom -> final grade pass -> OutputPass (tone mapping + sRGB) -> SMAA or FXAA.
 * The grade pass does biome colour grading, vignette, nitro radial blur, impact aberration,
 * black hole lens warps and low health desaturation, all in linear HDR space with texture lookups
 * only (no hash noise). Low tier reports available:true, enabled:false and renders directly.
 * ShaderPass clones its uniforms, so every write goes to pass.uniforms. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;

    const FINAL_VERT = 'varying vec2 vUv;\nvoid main() { vUv = uv; gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }';
    const FINAL_FRAG = [
        'uniform sampler2D tDiffuse;',
        'uniform float uAspect; uniform float uVig; uniform float uBlur; uniform vec2 uBlurC; uniform float uAb;',
        'uniform float uSat; uniform float uCon; uniform vec3 uLift; uniform vec3 uGain; uniform vec3 uShadow; uniform vec3 uHigh;',
        'uniform float uSplit; uniform float uPink; uniform float uDesat; uniform float uWob; uniform float uTime;',
        'uniform vec4 uH0; uniform vec4 uH1;',
        'varying vec2 vUv;',
        'const vec3 LUMA = vec3(0.2126, 0.7152, 0.0722);',
        'vec3 grab(vec2 p) {',
        '  if (uAb > 0.0005) {',
        '    vec2 r = (p - 0.5) * uAb;',
        '    return vec3(texture2D(tDiffuse, p + r).r, texture2D(tDiffuse, p).g, texture2D(tDiffuse, p - r).b);',
        '  }',
        '  return texture2D(tDiffuse, p).rgb;',
        '}',
        /* h = (x, y, radius, strength); radius is in aspect corrected units of screen height. */
        'vec2 lens(vec2 p, vec4 h, inout float core, inout float ring) {',
        '  if (h.w <= 0.0) return p;',
        '  vec2 d = p - h.xy; d.x *= uAspect;',
        '  float t = length(d) / h.z;',
        '  if (t >= 1.0) return p;',
        '  float k = h.w * (1.0 - t) * (1.0 - t);',
        '  float a = k * (1.15 + uWob * sin(t * 9.0 - uTime * 5.0));',
        '  float cs = cos(a), sn = sin(a);',
        '  d = vec2(cs * d.x - sn * d.y, sn * d.x + cs * d.y) * (1.0 + k * 1.0);',
        '  core = max(core, h.w * smoothstep(0.3, 0.1, t));',
        '  ring = max(ring, h.w * exp(-pow((t - 0.34) / 0.08, 2.0)));',
        '  d.x /= uAspect;',
        '  return h.xy + d;',
        '}',
        'void main() {',
        '  vec2 uv = vUv;',
        '  float core = 0.0; float ring = 0.0;',
        '  uv = lens(uv, uH0, core, ring);',
        '  uv = lens(uv, uH1, core, ring);',
        '  vec3 c;',
        '  if (uBlur > 0.002) {',
        '    vec2 dir = (uv - uBlurC) * uBlur;',
        '    c = grab(uv) * 0.22;',
        '    c += grab(uv - dir * 0.2) * 0.2; c += grab(uv - dir * 0.4) * 0.17; c += grab(uv - dir * 0.6) * 0.15;',
        '    c += grab(uv - dir * 0.8) * 0.14; c += grab(uv - dir) * 0.12;',
        '  } else { c = grab(uv); }',
        '  c = min(max(c, vec3(0.0)), vec3(64.0));',
        /* Grade: contrast around mid grey, lift in the shadows, gain, split toning, vibrance. */
        '  float l = dot(c, LUMA);',
        '  c = 0.18 * pow(max(c, vec3(0.0001)) / 0.18, vec3(uCon));',
        '  c = c * uGain + uLift * (1.0 - clamp(l * 3.0, 0.0, 1.0));',
        '  float lt = l / (1.0 + l);',
        '  float sw = 1.0 - smoothstep(0.1, 0.5, lt);',
        '  float hw = smoothstep(0.3, 0.75, lt);',
        '  c *= mix(vec3(1.0), uShadow, sw * uSplit) * mix(vec3(1.0), uHigh, hw * uSplit);',
        /* Mid and dark pink in the lower screen (a horizon reflected by the road) is pulled toward teal; neon and sky are kept. */
        '  float pm = smoothstep(0.0, 0.3, (c.r - c.g) / (l + 0.05)) * (1.0 - hw) * (1.0 - smoothstep(0.5, 0.62, vUv.y));',
        '  c.r *= 1.0 - uPink * pm * 0.55; c.g *= 1.0 + uPink * pm * 0.1; c.b *= 1.0 - uPink * pm * 0.08;',
        '  float l2 = dot(c, LUMA);',
        '  float sat = uSat + (1.0 - clamp(length(c - l2) / (l2 + 0.1), 0.0, 1.0)) * (uSat - 1.0) * 0.6;',
        '  c = mix(vec3(l2), c, sat);',
        '  c = mix(c, vec3(dot(c, LUMA)), uDesat);',
        '  vec2 q = (vUv - 0.5) * vec2(uAspect, 1.0) * 0.9;',
        '  c *= 1.0 - uVig * smoothstep(0.25, 1.0, dot(q, q) * 1.6);',
        '  c = c * (1.0 - core) + vec3(0.62, 0.33, 1.0) * ring * 0.22;',
        '  gl_FragColor = vec4(c, 1.0);',
        '}'
    ].join('\n');

    /* Per biome grade. shadow/high tint the dark and bright halves, gain is the white balance,
       thresh the bloom threshold (day biomes sit higher so snow and sand do not wash out), bloom a
       strength factor on LOOK.bloom, pink pulls dark pink reflections toward teal. */
    const NEUTRAL = { sat: 1.1, con: 1.06, gain: [1, 1, 1], lift: [0, 0, 0], shadow: [1, 1, 1], high: [1, 1, 1], split: 0, pink: 0, thresh: 1.2, bloom: 0.8, vig: 0.25 };
    function grade(o) { return Object.assign({}, NEUTRAL, o); }
    const GRADES = {
        alpine: grade({ sat: 1.22, con: 1.1, gain: [1.04, 1.0, 0.93], shadow: [0.93, 0.98, 1.08], high: [1.08, 1.0, 0.86], split: 0.5, thresh: 1.3, bloom: 0.7 }),
        desert: grade({ sat: 1.2, con: 1.1, gain: [1.06, 0.99, 0.88], shadow: [1.0, 0.9, 1.08], high: [1.1, 1.0, 0.8], split: 0.55, thresh: 1.35, bloom: 0.7 }),
        harbour: grade({ sat: 1.2, con: 1.18, gain: [0.97, 1.02, 1.04], lift: [0, 0.006, 0.01], shadow: [0.68, 1.04, 1.1], high: [1.14, 0.92, 1.06], split: 0.9, pink: 0.85, thresh: 1.15, bloom: 0.55 }),
        salt: grade({ sat: 1.12, con: 1.05, gain: [1.0, 1.0, 1.02], shadow: [0.95, 0.98, 1.1], high: [1.04, 0.99, 0.94], split: 0.45, thresh: 1.5, bloom: 0.6 }),
        forest: grade({ sat: 1.24, con: 1.1, gain: [1.0, 1.04, 0.9], shadow: [0.86, 1.05, 1.0], high: [1.08, 1.02, 0.8], split: 0.65, thresh: 1.3, bloom: 0.7 }),
        industry: grade({ sat: 1.18, con: 1.14, gain: [1.06, 0.98, 0.86], shadow: [0.88, 0.95, 1.14], high: [1.14, 0.98, 0.76], split: 0.7, thresh: 1.15, bloom: 0.6 }),
        glacier: grade({ sat: 1.2, con: 1.12, gain: [0.94, 1.0, 1.1], shadow: [0.8, 0.98, 1.22], high: [0.92, 1.04, 1.14], split: 0.75, thresh: 1.15, bloom: 0.55 }),
        metro: grade({ sat: 1.26, con: 1.18, gain: [1.0, 0.96, 1.06], lift: [0.002, 0, 0.012], shadow: [0.78, 0.92, 1.24], high: [1.14, 0.9, 1.1], split: 0.85, pink: 0.35, thresh: 1.5, bloom: 0.32 })
    };

    function tierName(gl) {
        for (const k in AD.QUALITY) if (AD.QUALITY[k] && AD.QUALITY[k].gl === gl) return k;
        return 'custom';
    }

    AD.createPost = function (T, renderer, scene, camera, glInitial) {
        let q = glInitial || AD.QUALITY.high.gl;
        let biome = 'none', g = NEUTRAL, bloomBase = 0.4;
        let view = { w: 960, h: 540, dpr: 1 };
        let composer = null, passes = null, failed = false, disposed = false;
        let ab = 0, kick = 0, blur = 0, time = 0;
        let manual = null;
        const holes = [new T.Vector4(), new T.Vector4()];
        const tmp = new T.Vector3(), tmp2 = new T.Vector3(), right = new T.Vector3();
        const hasPipeline = !!(T.EffectComposer && T.RenderPass && T.ShaderPass && T.OutputPass && T.UnrealBloomPass);

        function bloomStrength() { return bloomBase * g.bloom * (q.bloom === 'half' ? 1.25 : 1); }

        function dropComposer() {
            if (!composer) return;
            try {
                for (const p of composer.passes) if (p.dispose) p.dispose();
                composer.dispose();
            } catch (_) { /* partially built */ }
            composer = null; passes = null;
        }

        function build() {
            dropComposer();
            const rt = new T.WebGLRenderTarget(4, 4, { type: T.HalfFloatType, samples: q.msaa | 0 });
            const cp = new T.EffectComposer(renderer, rt);
            cp.setPixelRatio(view.dpr);
            cp.setSize(view.w, view.h);
            const render = new T.RenderPass(scene, camera);
            cp.addPass(render);
            let gtao = null;
            if (q.gtao && T.GTAOPass) {
                try {
                    gtao = new T.GTAOPass(scene, camera, view.w, view.h);
                    gtao.output = T.GTAOPass.OUTPUT.Default;
                    if (gtao.updateGtaoMaterial) gtao.updateGtaoMaterial({ radius: 9, distanceExponent: 1.5, thickness: 1, scale: 0.8, samples: 16, distanceFallOff: 1, screenSpaceRadius: true });
                    gtao.blendIntensity = 0.3;
                    /* The normal pass uses an override material that ignores transparency and depthWrite, so
                       sky domes, blob shadows and fx quads would count as occluders. Hide them for that pass. */
                    const baseRender = gtao.render.bind(gtao);
                    gtao.render = (r, wb, rb, dt, mask) => {
                        const hidden = [];
                        const shadowAutoUpdate = r.shadowMap.autoUpdate;
                        scene.traverse(o => {
                            if (!o.visible || !(o.isMesh || o.isPoints || o.isLine || o.isSprite)) return;
                            const m = Array.isArray(o.material) ? o.material[0] : o.material;
                            if (m && (m.transparent || m.depthWrite === false)) { o.visible = false; hidden.push(o); }
                        });
                        // The preceding beauty pass already rendered this frame's shadows.
                        r.shadowMap.autoUpdate = false;
                        try { baseRender(r, wb, rb, dt, mask); } finally {
                            r.shadowMap.autoUpdate = shadowAutoUpdate;
                            for (const o of hidden) o.visible = true;
                        }
                    };
                    cp.addPass(gtao);
                } catch (_) { gtao = null; }
            }
            let bloom = null;
            if (q.bloom !== 'off') {
                bloom = new T.UnrealBloomPass(new T.Vector2(view.w, view.h), bloomStrength(), 0.55, g.thresh);
                if (q.bloom === 'half') {
                    const base = bloom.setSize.bind(bloom);
                    bloom.setSize = (w, h) => base(Math.max(2, w >> 1), Math.max(2, h >> 1));
                }
                cp.addPass(bloom);
            }
            const fin = new T.ShaderPass({
                uniforms: {
                    tDiffuse: { value: null }, uAspect: { value: 1.7 }, uVig: { value: 0.25 }, uBlur: { value: 0 }, uBlurC: { value: new T.Vector2(0.5, 0.5) },
                    uAb: { value: 0 }, uSat: { value: 1 }, uCon: { value: 1 }, uLift: { value: new T.Vector3() }, uGain: { value: new T.Vector3(1, 1, 1) },
                    uShadow: { value: new T.Vector3(1, 1, 1) }, uHigh: { value: new T.Vector3(1, 1, 1) }, uSplit: { value: 0 }, uPink: { value: 0 },
                    uDesat: { value: 0 }, uWob: { value: 0 }, uTime: { value: 0 }, uH0: { value: new T.Vector4() }, uH1: { value: new T.Vector4() }
                },
                vertexShader: FINAL_VERT, fragmentShader: FINAL_FRAG
            });
            cp.addPass(fin);
            cp.addPass(new T.OutputPass());
            let aa = null;
            if (q.aa === 'smaa' && T.SMAAPass) aa = new T.SMAAPass();
            else if (q.aa !== 'none' && T.FXAAPass) aa = new T.FXAAPass();
            if (aa) cp.addPass(aa);
            composer = cp;
            passes = { bloom: bloom, fin: fin, gtao: gtao };
            cp.setSize(view.w, view.h);
            applyGrade();
        }

        function applyGrade() {
            if (!passes) return;
            const u = passes.fin.uniforms;
            u.uSat.value = g.sat; u.uCon.value = g.con; u.uSplit.value = g.split; u.uPink.value = g.pink; u.uVig.value = g.vig;
            u.uGain.value.set(g.gain[0], g.gain[1], g.gain[2]);
            u.uLift.value.set(g.lift[0], g.lift[1], g.lift[2]);
            u.uShadow.value.set(g.shadow[0], g.shadow[1], g.shadow[2]);
            u.uHigh.value.set(g.high[0], g.high[1], g.high[2]);
            if (passes.bloom) { passes.bloom.strength = bloomStrength(); passes.bloom.threshold = g.thresh; }
        }

        /* Projects a black hole to screen: uv centre, radius in aspect corrected screen heights. */
        function holeTo(out, h, aspect) {
            tmp.set(h.x, 1.2, h.y);
            tmp2.copy(tmp).applyMatrix4(camera.matrixWorldInverse);
            if (tmp2.z > -0.5) return false;
            const dist = -tmp2.z;
            tmp.project(camera);
            if (Math.abs(tmp.x) > 1.8 || Math.abs(tmp.y) > 1.8) return false;
            right.setFromMatrixColumn(camera.matrixWorld, 0);
            tmp2.set(h.x, 1.2, h.y).addScaledVector(right, h.r * 0.45).project(camera);
            const R = Math.hypot((tmp2.x - tmp.x) * 0.5 * aspect, (tmp2.y - tmp.y) * 0.5);
            const age = h.age || 0, ttl = h.ttl === undefined ? 9 : h.ttl;
            const s = clamp(age / 0.35, 0, 1) * clamp(ttl / 0.5, 0, 1);
            if (s <= 0.01 || R < 0.01) return false;
            out.set(tmp.x * 0.5 + 0.5, tmp.y * 0.5 + 0.5, Math.min(R, 1.2), s);
            return dist > 0;
        }

        function updateUniforms(dt, race, reduced) {
            const u = passes.fin.uniforms;
            const aspect = view.w / Math.max(1, view.h);
            time += dt;
            const car = race && race.cars && race.cars[0] ? race.cars[0].v : null;
            ab = ab > 0.01 ? ab * Math.exp(-4 * dt) : 0;
            kick = kick > 0.01 ? kick * Math.exp(-3 * dt) : 0;
            const nitro = car && (car.boost > 0 || car.burst > 0) ? 1 : 0;
            blur += (nitro - blur) * (1 - Math.exp(-(nitro ? 7 : 3.5) * dt));
            const calm = reduced ? 0 : 1;
            u.uAspect.value = aspect;
            u.uAb.value = ab * 0.05 * calm;
            u.uBlur.value = clamp(blur * 0.1 + kick * 0.05, 0, 0.14) * calm;
            u.uWob.value = 0.5 * calm;
            u.uTime.value = time % 100;
            u.uDesat.value = car ? clamp((car.damage - 55) / 45, 0, 1) * 0.35 : 0;
            let n = 0;
            if (manual) {
                for (const h of manual) if (n < 2) holes[n++].set(h.x, h.y, h.r, h.s === undefined ? 1 : h.s);
            } else if (race && race.hazards) {
                let best = null;
                for (const h of race.hazards) {
                    if (h.type !== 'black_hole') continue;
                    (best || (best = [])).push(h);
                }
                if (best) {
                    best.sort((a, b) => Math.hypot(a.x - camera.position.x, a.y - camera.position.z) - Math.hypot(b.x - camera.position.x, b.y - camera.position.z));
                    for (const h of best) if (n < 2 && holeTo(holes[n], h, aspect)) n++;
                }
            }
            for (let i = n; i < 2; i++) holes[i].set(0, 0, 1, 0);
            u.uH0.value.copy(holes[0]);
            u.uH1.value.copy(holes[1]);
        }

        const api = {
            available: hasPipeline,
            get enabled() { return hasPipeline && !!q.post && !failed && !disposed; },
            render(dt, race, pose, reduced) {
                if (!api.enabled) return false;
                try {
                    if (!composer) build();
                    updateUniforms(dt, race, !!reduced);
                    composer.render(dt);
                    return true;
                } catch (_) {
                    failed = true;
                    dropComposer();
                    return false;
                }
            },
            setQuality(next) {
                if (!next) return;
                const old = q;
                q = next;
                if (!q.post) { dropComposer(); return; }
                if (composer && (old.msaa !== q.msaa || old.aa !== q.aa || old.gtao !== q.gtao || old.bloom !== q.bloom)) dropComposer();
                else if (composer) { composer.setPixelRatio(view.dpr); composer.setSize(view.w, view.h); applyGrade(); }
            },
            setBiome(id) {
                biome = id || 'none';
                const look = AD.LOOK && AD.LOOK[biome];
                g = GRADES[biome] || NEUTRAL;
                bloomBase = look && typeof look.bloom === 'number' ? look.bloom : 0.4;
                applyGrade();
            },
            resize(w, h, dpr) {
                view = { w: Math.max(2, Math.round(w)), h: Math.max(2, Math.round(h)), dpr: dpr || 1 };
                if (composer) { composer.setPixelRatio(view.dpr); composer.setSize(view.w, view.h); }
            },
            /* kind: 'impact' (alias 'hit') drives the aberration, 'boost' a short blur kick. */
            impulse(kind, amount) {
                const a = clamp(amount === undefined ? 1 : amount, 0, 1);
                if (kind === 'impact' || kind === 'hit') ab = Math.max(ab, a);
                else if (kind === 'boost') kick = Math.max(kick, a);
            },
            /* Manual lens override: [{x, y (uv), r (screen heights), s}] or null to follow race.hazards. */
            setHoles(list) { manual = list && list.length ? list : null; },
            restore() { dropComposer(); failed = false; },
            stats() { return { available: hasPipeline, enabled: api.enabled, degraded: false, quality: tierName(q), biome: biome }; },
            dispose() { disposed = true; dropComposer(); }
        };
        return api;
    };
})();
