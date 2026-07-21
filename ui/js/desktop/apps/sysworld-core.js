(function () {
    'use strict';

    // System World — Agent Core module.
    // Renders the living centerpiece of the system universe: a shader-driven
    // inner sun, a rotating lattice, gyroscopic energy rings, an orbiting
    // corona particle shell and the agent memory nebula. Exposes
    // NS.createCore(inst) which returns the per-window inst.core module.
    // Loaded after sysworld-effects.js / sysworld-scene.js; all shared
    // constants and fx helpers are resolved lazily and guarded so this file
    // can never crash the boot sequence when something is missing.

    const NS = window.SysWorld = window.SysWorld || {};

    // Fallback mirrors of the shared contract constants. The authoritative
    // values live on NS.PALETTE / NS.LAYOUT (defined by the foundation
    // modules); these only protect against a missing foundation.
    const FALLBACK_PALETTE = {
        core: 0x59d4ff,
        coreHot: 0xffffff,
        agent: 0x80deea,
        memory: 0x4dd0e1,
        gold: 0xffd54f,
        journal: 0xff8a65,
        warn: 0xffca28
    };
    const FALLBACK_LAYOUT = { coreRadius: 6, memoryRadius: 15 };

    const TWO_PI = Math.PI * 2;

    // Inner-sun shaders, written for the vendored Three.js r128 (manual
    // uniforms/varyings, no includes). Look: fresnel rim glow plus a slow
    // trigonometric noise swirl; the base color uniform is lerped outside
    // toward the current mood color.
    const SUN_VERTEX_SHADER = [
        'varying vec3 vNormal;',
        'varying vec3 vViewPosition;',
        'varying vec3 vPosition;',
        'void main() {',
        '    vNormal = normalize(normalMatrix * normal);',
        '    vPosition = position;',
        '    vec4 mvPosition = modelViewMatrix * vec4(position, 1.0);',
        '    vViewPosition = -mvPosition.xyz;',
        '    gl_Position = projectionMatrix * mvPosition;',
        '}'
    ].join('\n');

    const SUN_FRAGMENT_SHADER = [
        'uniform vec3 uColor;',
        'uniform float uTime;',
        'uniform float uHot;',
        'uniform float uPulse;',
        'varying vec3 vNormal;',
        'varying vec3 vViewPosition;',
        'varying vec3 vPosition;',
        'float swirl(vec3 p, float t) {',
        '    float v = sin(p.x * 2.1 + t * 0.70) * cos(p.y * 2.3 - t * 0.60);',
        '    v += sin(p.z * 3.1 + t * 0.90) * 0.50;',
        '    v += cos((p.x + p.y + p.z) * 1.7 + t * 0.40) * 0.35;',
        '    v += sin(p.y * 5.2 - t * 1.30) * 0.20;',
        '    return v * 0.5 + 0.5;',
        '}',
        'void main() {',
        '    vec3 n = normalize(vNormal);',
        '    vec3 v = normalize(vViewPosition);',
        '    float fres = pow(1.0 - abs(dot(n, v)), 2.2);',
        '    float s = swirl(vPosition * 1.4, uTime);',
        '    vec3 base = mix(uColor, vec3(1.0), uHot * (0.35 + 0.45 * s));',
        '    vec3 col = base * (0.55 + 0.75 * s) + base * fres * 1.6;',
        '    col += vec3(1.0) * uPulse * 0.12;',
        '    float alpha = clamp(0.82 + fres * 0.18, 0.0, 1.0);',
        '    gl_FragColor = vec4(col, alpha);',
        '}'
    ].join('\n');

    function noop() {}

    function pal() {
        return NS.PALETTE || FALLBACK_PALETTE;
    }

    function lay() {
        return NS.LAYOUT || FALLBACK_LAYOUT;
    }

    function clamp(v, lo, hi) {
        return v < lo ? lo : (v > hi ? hi : v);
    }

    function toFinite(v, fallback) {
        const n = Number(v);
        return isFinite(n) ? n : fallback;
    }

    // Maps an agent mood string to a palette color. Unknown moods fall back
    // to the signature core blue.
    function moodToHex(mood, P) {
        const m = String(mood || '').toLowerCase();
        if (m === 'happy' || m === 'excited') return P.gold;
        if (m === 'curious') return P.memory;
        if (m === 'calm' || m === 'focused') return P.core;
        if (m === 'alert' || m === 'stressed') return P.warn;
        return P.core;
    }

    // Local radial-gradient sprite texture, used only when the effects
    // module is unavailable. Marked as owned so dispose() may free it.
    function makeFallbackGlowTexture(THREE) {
        const canvas = document.createElement('canvas');
        canvas.width = 64;
        canvas.height = 64;
        const ctx = canvas.getContext('2d');
        const grad = ctx.createRadialGradient(32, 32, 0, 32, 32, 32);
        grad.addColorStop(0, 'rgba(255, 255, 255, 1)');
        grad.addColorStop(0.35, 'rgba(255, 255, 255, 0.55)');
        grad.addColorStop(1, 'rgba(255, 255, 255, 0)');
        ctx.fillStyle = grad;
        ctx.fillRect(0, 0, 64, 64);
        const tex = new THREE.CanvasTexture(canvas);
        tex.userData = tex.userData || {};
        tex.userData.__sysworldOwned = true;
        return tex;
    }

    // Disposes a single material (or array). Textures handed out by the fx
    // module are considered shared and are left alone; only textures this
    // module created itself (flagged __sysworldOwned) are freed.
    function disposeMaterialDeep(material, seenTextures) {
        if (!material) return;
        const mats = Array.isArray(material) ? material : [material];
        mats.forEach(function (mat) {
            if (!mat) return;
            const map = mat.map;
            if (map && map.userData && map.userData.__sysworldOwned && !seenTextures.has(map)) {
                seenTextures.add(map);
                if (typeof map.dispose === 'function') map.dispose();
            }
            if (typeof mat.dispose === 'function') mat.dispose();
        });
    }

    function disposeObjectTree(root) {
        if (!root) return;
        const seenGeometries = new Set();
        const seenTextures = new Set();
        root.traverse(function (node) {
            if (node.geometry && !seenGeometries.has(node.geometry)) {
                seenGeometries.add(node.geometry);
                if (typeof node.geometry.dispose === 'function') node.geometry.dispose();
            }
            disposeMaterialDeep(node.material, seenTextures);
        });
    }

    // Null-object module returned when THREE or the stage is missing so the
    // entry can still wire the app without defensive checks everywhere.
    function nullModule(inst) {
        const api = {
            group: null,
            setAgent: noop,
            setMood: noop,
            setMetrics: noop,
            setMemory: noop,
            memoryFlash: noop,
            punch: noop,
            update: noop,
            dispose: noop
        };
        if (inst) inst.core = api;
        return api;
    }

    NS.createCore = function (inst) {
        const THREE = (inst && inst.THREE) || window.THREE;
        if (!THREE || !inst || !inst.stage || !inst.stage.scene) {
            return nullModule(inst);
        }

        const P = pal();
        const L = lay();
        const fx = inst.fx || null;
        const qualityScale = clamp(toFinite(inst.qualityScale, 1), 0.05, 2);
        const coreRadius = Math.max(0.5, toFinite(L.coreRadius, 6));
        const memoryRadius = Math.max(2, toFinite(L.memoryRadius, 15));

        // All mutable per-window state lives here; nothing is cached in
        // module globals so multiple desktop windows stay independent.
        const state = {
            disposed: false,
            busy: false,
            moodHex: P.core,
            cpu: 0,
            memory: 0,
            hot: 0,
            hotTarget: 0,
            pulsePhase: 0,
            pulse: 0,
            pop: 0,
            memFlash: 0,
            memCounts: { nebula: -1, stars: -1, embers: -1 },
            nebula: null,
            stars: [],
            embers: null
        };

        // Scratch objects reused every frame (no per-frame allocations).
        const currentColor = new THREE.Color(P.core);
        const targetColor = new THREE.Color(P.core);
        const originVec = new THREE.Vector3(0, 0, 0);

        const group = new THREE.Group();
        group.name = 'sysworld-core';
        const coreGroup = new THREE.Group();
        coreGroup.name = 'sysworld-core-sun';
        const nebulaGroup = new THREE.Group();
        nebulaGroup.name = 'sysworld-core-nebula';
        group.add(coreGroup);
        group.add(nebulaGroup);

        let ownedGlowTex = null;

        // Resolves a soft radial glow texture, preferring the shared fx
        // helper and falling back to a locally owned canvas texture.
        function glowTexture() {
            if (fx && typeof fx.glowTexture === 'function') {
                const tex = fx.glowTexture(P.core);
                if (tex) return tex;
            }
            if (!ownedGlowTex) ownedGlowTex = makeFallbackGlowTexture(THREE);
            return ownedGlowTex;
        }

        function makeGlow(hex, size) {
            if (fx && typeof fx.makeGlowSprite === 'function') {
                const sprite = fx.makeGlowSprite(hex, size);
                if (sprite) return sprite;
            }
            const sprite = new THREE.Sprite(new THREE.SpriteMaterial({
                map: glowTexture(),
                color: hex,
                transparent: true,
                opacity: 0.8,
                blending: THREE.AdditiveBlending,
                depthWrite: false
            }));
            sprite.scale.set(size, size, 1);
            return sprite;
        }

        // ----------------------------------------------------------------
        // Inner sun — custom ShaderMaterial with fresnel rim + noise swirl.
        // ----------------------------------------------------------------
        const sunUniforms = {
            uColor: { value: new THREE.Color(P.core) },
            uTime: { value: 0 },
            uHot: { value: 0 },
            uPulse: { value: 0 }
        };
        const sun = new THREE.Mesh(
            new THREE.SphereGeometry(coreRadius * 0.55, 48, 32),
            new THREE.ShaderMaterial({
                uniforms: sunUniforms,
                vertexShader: SUN_VERTEX_SHADER,
                fragmentShader: SUN_FRAGMENT_SHADER,
                transparent: true,
                blending: THREE.AdditiveBlending,
                depthWrite: false
            })
        );
        sun.name = 'sysworld-core-inner-sun';
        coreGroup.add(sun);

        // Central bloom sprite hugging the sun.
        const coreGlow = makeGlow(P.core, coreRadius * 5.4);
        coreGlow.material.opacity = 0.5;
        coreGroup.add(coreGlow);
        const coreGlowBase = coreGlow.scale.x || coreRadius * 5.4;

        // ----------------------------------------------------------------
        // Lattice — slowly tumbling icosahedron wireframe around the sun.
        // ----------------------------------------------------------------
        const latticeMat = new THREE.MeshBasicMaterial({
            color: P.agent,
            wireframe: true,
            transparent: true,
            opacity: 0.16,
            blending: THREE.AdditiveBlending,
            depthWrite: false
        });
        const lattice = new THREE.Mesh(
            new THREE.IcosahedronGeometry(coreRadius * 0.98, 1),
            latticeMat
        );
        lattice.name = 'sysworld-core-lattice';
        coreGroup.add(lattice);

        // ----------------------------------------------------------------
        // Gyroscopic energy rings — three tilted tori spinning on
        // independent axes at independent speeds.
        // ----------------------------------------------------------------
        const rings = [];
        [
            { radius: 1.28, tilt: [0.90, 0.00, 0.35], spin: [0.35, 0.55], opacity: 0.55 },
            { radius: 1.55, tilt: [1.35, 0.50, 0.00], spin: [-0.28, 0.42], opacity: 0.42 },
            { radius: 1.85, tilt: [0.25, 1.10, 0.80], spin: [0.50, -0.30], opacity: 0.34 },
            { radius: 2.18, tilt: [0.60, 0.85, 1.20], spin: [-0.18, 0.62], opacity: 0.22 }
        ].forEach(function (def, index) {
            const mat = new THREE.MeshBasicMaterial({
                color: P.core,
                transparent: true,
                opacity: def.opacity,
                blending: THREE.AdditiveBlending,
                depthWrite: false
            });
            const mesh = new THREE.Mesh(
                new THREE.TorusGeometry(coreRadius * def.radius, coreRadius * 0.012, 8, 96),
                mat
            );
            mesh.name = 'sysworld-core-ring-' + index;
            mesh.rotation.set(def.tilt[0], def.tilt[1], def.tilt[2]);
            coreGroup.add(mesh);
            rings.push({ mesh: mesh, mat: mat, spinX: def.spin[0], spinY: def.spin[1] });
        });

        // Outer wireframe halo shell for extra depth around the core.
        const haloMat = new THREE.MeshBasicMaterial({
            color: P.coreHot,
            wireframe: true,
            transparent: true,
            opacity: 0.08,
            blending: THREE.AdditiveBlending,
            depthWrite: false
        });
        const halo = new THREE.Mesh(
            new THREE.IcosahedronGeometry(coreRadius * 2.35, 1),
            haloMat
        );
        halo.name = 'sysworld-core-halo';
        coreGroup.add(halo);

        // Occasional ambient flare timer (driven in update).
        let flareCooldown = 1.8 + Math.random() * 2.5;

        // ----------------------------------------------------------------
        // Corona — additive point shell orbiting the core with a gentle
        // radial breathing motion. Per-particle params are precomputed once;
        // update() only rewrites the position attribute.
        // ----------------------------------------------------------------
        const coronaCount = Math.max(60, Math.floor(600 * qualityScale));
        const coronaRadius = new Float32Array(coronaCount);
        const coronaAngle = new Float32Array(coronaCount);
        const coronaSpeed = new Float32Array(coronaCount);
        const coronaIncl = new Float32Array(coronaCount);
        const coronaPhase = new Float32Array(coronaCount);
        const coronaPositions = new Float32Array(coronaCount * 3);
        const coronaColors = new Float32Array(coronaCount * 3);
        const scratchColor = new THREE.Color(0xffffff);
        for (let i = 0; i < coronaCount; i++) {
            coronaRadius[i] = coreRadius * (1.15 + Math.random() * 1.15);
            coronaAngle[i] = Math.random() * TWO_PI;
            coronaSpeed[i] = (0.05 + Math.random() * 0.20) * (Math.random() < 0.5 ? 1 : -1);
            coronaIncl[i] = (Math.random() * 2 - 1);
            coronaPhase[i] = Math.random() * TWO_PI;
            const mix = Math.random();
            if (mix < 0.55) scratchColor.setHex(P.core);
            else if (mix < 0.8) scratchColor.setHex(P.coreHot);
            else scratchColor.setHex(P.agent);
            const dim = 0.55 + Math.random() * 0.45;
            coronaColors[i * 3] = scratchColor.r * dim;
            coronaColors[i * 3 + 1] = scratchColor.g * dim;
            coronaColors[i * 3 + 2] = scratchColor.b * dim;
        }
        const coronaGeo = new THREE.BufferGeometry();
        const coronaPosAttr = new THREE.BufferAttribute(coronaPositions, 3);
        coronaPosAttr.setUsage(THREE.DynamicDrawUsage);
        coronaGeo.setAttribute('position', coronaPosAttr);
        coronaGeo.setAttribute('color', new THREE.BufferAttribute(coronaColors, 3));
        const coronaMat = new THREE.PointsMaterial({
            size: coreRadius * 0.055,
            map: glowTexture(),
            vertexColors: true,
            transparent: true,
            opacity: 0.85,
            blending: THREE.AdditiveBlending,
            depthWrite: false,
            sizeAttenuation: true
        });
        const corona = new THREE.Points(coronaGeo, coronaMat);
        corona.name = 'sysworld-core-corona';
        corona.frustumCulled = false;
        coreGroup.add(corona);

        function updateCorona(elapsed) {
            const breatheGlobal = 1 + state.pop * 0.25;
            for (let i = 0; i < coronaCount; i++) {
                const a = coronaAngle[i] + elapsed * coronaSpeed[i];
                const breathe = breatheGlobal * (1 + 0.08 * Math.sin(elapsed * 0.7 + coronaPhase[i]));
                const r = coronaRadius[i] * breathe;
                coronaPositions[i * 3] = Math.cos(a) * r;
                coronaPositions[i * 3 + 1] = coronaIncl[i] * r * 0.38;
                coronaPositions[i * 3 + 2] = Math.sin(a) * r;
            }
            coronaPosAttr.needsUpdate = true;
        }

        // ----------------------------------------------------------------
        // Memory nebula — rebuilt on every (bucket-changing) setMemory()
        // call. Old geometry/materials are disposed before rebuilding.
        // ----------------------------------------------------------------

        function disposeMemoryVisuals() {
            if (state.nebula) {
                nebulaGroup.remove(state.nebula.points);
                state.nebula.geo.dispose();
                state.nebula.mat.dispose();
                state.nebula = null;
            }
            if (state.embers) {
                nebulaGroup.remove(state.embers.points);
                state.embers.geo.dispose();
                state.embers.mat.dispose();
                state.embers = null;
            }
            for (let i = 0; i < state.stars.length; i++) {
                const star = state.stars[i];
                nebulaGroup.remove(star.sprite);
                disposeMaterialDeep(star.sprite.material, new Set());
            }
            state.stars.length = 0;
        }

        function buildNebula(count) {
            const positions = new Float32Array(count * 3);
            const colors = new Float32Array(count * 3);
            const radius = new Float32Array(count);
            const angle = new Float32Array(count);
            const speed = new Float32Array(count);
            const incl = new Float32Array(count);
            const phase = new Float32Array(count);
            for (let i = 0; i < count; i++) {
                radius[i] = memoryRadius * (0.72 + Math.random() * 0.66);
                angle[i] = Math.random() * TWO_PI;
                speed[i] = (0.008 + Math.random() * 0.035) * (Math.random() < 0.5 ? 1 : -1);
                incl[i] = (Math.random() * 2 - 1);
                phase[i] = Math.random() * TWO_PI;
                const mix = Math.random();
                if (mix < 0.6) scratchColor.setHex(P.memory);
                else if (mix < 0.85) scratchColor.setHex(P.agent);
                else scratchColor.setHex(P.coreHot);
                const dim = 0.35 + Math.random() * 0.5;
                colors[i * 3] = scratchColor.r * dim;
                colors[i * 3 + 1] = scratchColor.g * dim;
                colors[i * 3 + 2] = scratchColor.b * dim;
            }
            const geo = new THREE.BufferGeometry();
            const posAttr = new THREE.BufferAttribute(positions, 3);
            posAttr.setUsage(THREE.DynamicDrawUsage);
            geo.setAttribute('position', posAttr);
            geo.setAttribute('color', new THREE.BufferAttribute(colors, 3));
            const mat = new THREE.PointsMaterial({
                size: 0.55,
                map: glowTexture(),
                vertexColors: true,
                transparent: true,
                opacity: 0.5,
                blending: THREE.AdditiveBlending,
                depthWrite: false,
                sizeAttenuation: true
            });
            const points = new THREE.Points(geo, mat);
            points.name = 'sysworld-nebula';
            points.frustumCulled = false;
            nebulaGroup.add(points);
            state.nebula = {
                points: points, geo: geo, mat: mat, posAttr: posAttr,
                positions: positions, radius: radius, angle: angle,
                speed: speed, incl: incl, phase: phase, count: count,
                baseOpacity: 0.5
            };
        }

        function buildStars(count) {
            for (let i = 0; i < count; i++) {
                const sprite = makeGlow(P.gold, 1.6 + Math.random() * 1.2);
                sprite.name = 'sysworld-core-star';
                nebulaGroup.add(sprite);
                state.stars.push({
                    sprite: sprite,
                    radius: memoryRadius * (0.35 + Math.random() * 0.27),
                    angle: Math.random() * TWO_PI,
                    speed: 0.02 + Math.random() * 0.05,
                    incl: (Math.random() * 2 - 1) * 0.55,
                    phase: Math.random() * TWO_PI,
                    baseScale: sprite.scale.x || 1.6
                });
            }
        }

        function buildEmbers(count) {
            const positions = new Float32Array(count * 3);
            const colors = new Float32Array(count * 3);
            const baseColors = new Float32Array(count * 3);
            const angle = new Float32Array(count);
            const spin = new Float32Array(count);
            const incl = new Float32Array(count);
            const phase = new Float32Array(count);
            const rise = new Float32Array(count);
            for (let i = 0; i < count; i++) {
                angle[i] = Math.random() * TWO_PI;
                spin[i] = (0.01 + Math.random() * 0.03) * (Math.random() < 0.5 ? 1 : -1);
                incl[i] = (Math.random() * 2 - 1);
                phase[i] = Math.random();
                // Outward drift period between ~35s and ~95s.
                rise[i] = 1 / (35 + Math.random() * 60);
                const mix = Math.random();
                if (mix < 0.7) scratchColor.setHex(P.journal);
                else scratchColor.setHex(P.gold);
                baseColors[i * 3] = scratchColor.r;
                baseColors[i * 3 + 1] = scratchColor.g;
                baseColors[i * 3 + 2] = scratchColor.b;
            }
            const geo = new THREE.BufferGeometry();
            const posAttr = new THREE.BufferAttribute(positions, 3);
            posAttr.setUsage(THREE.DynamicDrawUsage);
            geo.setAttribute('position', posAttr);
            const colAttr = new THREE.BufferAttribute(colors, 3);
            colAttr.setUsage(THREE.DynamicDrawUsage);
            geo.setAttribute('color', colAttr);
            const mat = new THREE.PointsMaterial({
                size: 0.45,
                map: glowTexture(),
                vertexColors: true,
                transparent: true,
                opacity: 0.55,
                blending: THREE.AdditiveBlending,
                depthWrite: false,
                sizeAttenuation: true
            });
            const points = new THREE.Points(geo, mat);
            points.name = 'sysworld-embers';
            points.frustumCulled = false;
            nebulaGroup.add(points);
            state.embers = {
                points: points, geo: geo, mat: mat, posAttr: posAttr, colAttr: colAttr,
                positions: positions, colors: colors, baseColors: baseColors,
                angle: angle, spin: spin, incl: incl, phase: phase, rise: rise,
                count: count, baseOpacity: 0.55,
                rMin: memoryRadius * 0.85, rMax: memoryRadius * 2.3
            };
        }

        // Translates raw memory counters into particle budgets and rebuilds
        // the nebula visuals only when a bucket actually changed.
        function buildMemory(mem) {
            const source = mem || {};
            const entries = clamp(toFinite(source.vectordb_entries, 0), 0, 1e9);
            const facts = clamp(toFinite(source.core_memory_facts, 0), 0, 1e6);
            const journal = clamp(toFinite(source.journal_entries, 0), 0, 1e9);

            const nebulaCap = Math.max(90, Math.floor(1400 * qualityScale));
            const nebulaTarget = clamp(Math.round(110 + Math.log10(entries + 1) * 330), 90, nebulaCap);
            const starTarget = clamp(Math.round(facts), 0, 40);
            const emberCap = Math.max(24, Math.floor(260 * qualityScale));
            const emberTarget = journal <= 0 ? 0 : clamp(Math.round(50 + Math.log10(journal + 1) * 75), 8, emberCap);

            const counts = state.memCounts;
            if (counts.nebula === nebulaTarget && counts.stars === starTarget && counts.embers === emberTarget) {
                return;
            }
            counts.nebula = nebulaTarget;
            counts.stars = starTarget;
            counts.embers = emberTarget;

            disposeMemoryVisuals();
            buildNebula(nebulaTarget);
            buildStars(starTarget);
            if (emberTarget > 0) buildEmbers(emberTarget);
        }

        function updateNebula(elapsed) {
            const neb = state.nebula;
            if (!neb) return;
            for (let i = 0; i < neb.count; i++) {
                const a = neb.angle[i] + elapsed * neb.speed[i];
                const breathe = 1 + 0.05 * Math.sin(elapsed * 0.5 + neb.phase[i]) + state.memFlash * 0.06;
                const r = neb.radius[i] * breathe;
                neb.positions[i * 3] = Math.cos(a) * r;
                neb.positions[i * 3 + 1] = neb.incl[i] * r * 0.42;
                neb.positions[i * 3 + 2] = Math.sin(a) * r;
            }
            neb.posAttr.needsUpdate = true;
            neb.mat.opacity = clamp(neb.baseOpacity * (1 + state.memFlash * 1.3), 0, 1);
        }

        function updateStars(elapsed) {
            const flashBoost = 1 + state.memFlash * 0.8;
            for (let i = 0; i < state.stars.length; i++) {
                const star = state.stars[i];
                const a = star.angle + elapsed * star.speed;
                star.sprite.position.set(
                    Math.cos(a) * star.radius,
                    star.incl * star.radius,
                    Math.sin(a) * star.radius
                );
                const twinkle = 0.8 + 0.35 * Math.sin(elapsed * 2.1 + star.phase);
                const s = star.baseScale * twinkle * flashBoost;
                star.sprite.scale.set(s, s, 1);
            }
        }

        function updateEmbers(dt, elapsed) {
            const emb = state.embers;
            if (!emb) return;
            const flash = clamp(0.55 + 0.45 * state.memFlash, 0, 1);
            for (let i = 0; i < emb.count; i++) {
                emb.angle[i] += dt * emb.spin[i];
                const cycle = (elapsed * emb.rise[i] + emb.phase[i]) % 1;
                const r = emb.rMin + cycle * (emb.rMax - emb.rMin);
                const a = emb.angle[i];
                emb.positions[i * 3] = Math.cos(a) * r;
                emb.positions[i * 3 + 1] = emb.incl[i] * r * 0.4;
                emb.positions[i * 3 + 2] = Math.sin(a) * r;
                // Fade in at birth, fade out while drifting away.
                const fadeIn = Math.min(1, cycle / 0.12);
                const fadeOut = Math.min(1, (1 - cycle) / 0.3);
                const f = Math.min(fadeIn, fadeOut) * flash;
                emb.colors[i * 3] = emb.baseColors[i * 3] * f;
                emb.colors[i * 3 + 1] = emb.baseColors[i * 3 + 1] * f;
                emb.colors[i * 3 + 2] = emb.baseColors[i * 3 + 2] * f;
            }
            emb.posAttr.needsUpdate = true;
            emb.colAttr.needsUpdate = true;
            emb.mat.opacity = clamp(emb.baseOpacity * (1 + state.memFlash), 0, 1);
        }

        // ----------------------------------------------------------------
        // Public API
        // ----------------------------------------------------------------

        function setAgent(agent) {
            state.busy = !!(agent && agent.busy);
            state.hotTarget = state.busy ? 1 : 0;
        }

        function setMood(mood) {
            state.moodHex = moodToHex(mood, pal());
            targetColor.setHex(state.moodHex);
        }

        function setMetrics(m) {
            // Accept flat percents or nested dashboard/SSE shapes
            // ({ usage_percent } / { used_percent }).
            const cpuRaw = m && (
                typeof m.cpu === 'number' ? m.cpu
                    : (m.cpu && (m.cpu.usage_percent != null ? m.cpu.usage_percent : m.cpu.percent))
            );
            const memRaw = m && (
                typeof m.memory === 'number' ? m.memory
                    : (m.memory && (m.memory.used_percent != null ? m.memory.used_percent : m.memory.percent))
            );
            state.cpu = clamp(toFinite(cpuRaw, 0), 0, 100);
            state.memory = clamp(toFinite(memRaw, 0), 0, 100);
        }

        function setMemory(mem) {
            buildMemory(mem);
        }

        // Brief nebula brightening, triggered on memory_update SSE events.
        function memoryFlash() {
            state.memFlash = 1;
        }

        // Energy shock: expanding pulse ring at the origin plus a short
        // scale pop of the whole core. Called on agent activity.
        function punch(strength) {
            const s = clamp(toFinite(strength, 1), 0.05, 4);
            state.pop = Math.min(1.6, state.pop + 0.55 * s);
            if (fx && typeof fx.pulseRing === 'function') {
                fx.pulseRing(originVec, state.moodHex, coreRadius * (5 + s * 2));
            }
        }

        function update(dt, elapsed) {
            if (state.disposed) return;
            dt = clamp(toFinite(dt, 0), 0, 0.1);
            elapsed = toFinite(elapsed, 0);

            // Smooth mood-color and heat transitions.
            currentColor.lerp(targetColor, 1 - Math.exp(-dt * 2.6));
            state.hot += (state.hotTarget - state.hot) * (1 - Math.exp(-dt * 3));

            // Heartbeat: cpu drives the base frequency, busy accelerates it.
            const freq = (0.7 + (state.cpu / 100) * 2.4) * (state.busy ? 1.8 : 1);
            state.pulsePhase += dt * freq * Math.PI;
            state.pulse = 0.5 + 0.5 * Math.sin(state.pulsePhase);

            // Exponential decays for transient effects.
            state.pop *= Math.exp(-dt * 3.2);
            state.memFlash *= Math.exp(-dt * 2.4);

            // Sun shader + heartbeat scale.
            sunUniforms.uTime.value = elapsed;
            sunUniforms.uColor.value.copy(currentColor);
            sunUniforms.uHot.value = state.hot;
            sunUniforms.uPulse.value = state.pulse;
            sun.scale.setScalar(1 + state.pulse * 0.05);
            coreGroup.scale.setScalar(1 + state.pop * 0.14);

            // Central glow follows the mood color and breathes with activity.
            if (coreGlow && coreGlow.material) {
                coreGlow.material.color.copy(currentColor);
                const gs = coreGlowBase * (1 + state.pulse * 0.08 + state.pop * 0.35 + state.memFlash * 0.1);
                coreGlow.scale.set(gs, gs, 1);
            }

            // Lattice tumble + mood cohesion.
            lattice.rotation.y += dt * 0.14;
            lattice.rotation.x += dt * 0.06;
            latticeMat.color.copy(currentColor);
            lattice.scale.setScalar(1 + state.pulse * 0.02);
            latticeMat.opacity = 0.14 + state.pulse * 0.08 + state.hot * 0.06;

            // Outer halo counter-rotates slowly and breathes with heat.
            halo.rotation.y -= dt * 0.08;
            halo.rotation.z += dt * 0.04;
            haloMat.color.copy(currentColor);
            haloMat.opacity = 0.06 + state.pulse * 0.05 + state.hot * 0.08 + state.pop * 0.1;
            halo.scale.setScalar(1 + state.pulse * 0.03 + state.pop * 0.08);

            // Gyroscopic rings.
            for (let i = 0; i < rings.length; i++) {
                const ring = rings[i];
                const boost = 1 + state.busy * 0.55 + state.hot * 0.4;
                ring.mesh.rotation.x += ring.spinX * dt * boost;
                ring.mesh.rotation.y += ring.spinY * dt * boost;
                ring.mat.color.copy(currentColor);
                ring.mat.opacity = Math.min(0.85, (0.22 + i * 0.08) + state.pulse * 0.12 + state.hot * 0.1);
            }

            // Ambient core flares so the center never feels static.
            flareCooldown -= dt;
            if (flareCooldown <= 0 && fx && inst.effectsEnabled !== false) {
                flareCooldown = state.busy ? (0.7 + Math.random() * 1.1) : (2.2 + Math.random() * 3.5);
                if (typeof fx.pulseRing === 'function') {
                    fx.pulseRing(originVec, state.moodHex, coreRadius * (3.5 + Math.random() * 3));
                }
                if (typeof fx.sparkle === 'function' && Math.random() < 0.55) {
                    fx.sparkle(originVec, state.moodHex, 10 + Math.floor(Math.random() * 10));
                }
            }

            updateCorona(elapsed);
            updateNebula(elapsed);
            updateStars(elapsed);
            updateEmbers(dt, elapsed);
        }

        function dispose() {
            if (state.disposed) return;
            state.disposed = true;
            disposeMemoryVisuals();
            if (group.parent) group.parent.remove(group);
            disposeObjectTree(group);
        }

        const api = {
            group: group,
            setAgent: setAgent,
            setMood: setMood,
            setMetrics: setMetrics,
            setMemory: setMemory,
            memoryFlash: memoryFlash,
            punch: punch,
            update: update,
            dispose: dispose
        };

        // Build the default (empty) nebula so the core never looks dead
        // before the first data push arrives.
        buildMemory(null);
        inst.stage.scene.add(group);
        inst.core = api;
        return api;
    };
})();
