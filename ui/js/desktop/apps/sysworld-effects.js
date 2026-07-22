(function () {
    'use strict';

    // System World — visual effects module.
    // Provides the shared palette, easing helpers and the per-window fx
    // instance (glow textures, comets, particle bursts, pulse rings, trails,
    // tween runner). Everything transient is pooled and capped, all
    // canvas-generated textures are cached, and nothing is allocated inside
    // per-frame hot paths.

    const NS = window.SysWorld = window.SysWorld || {};

    // Shared color contract for every System World module.
    NS.PALETTE = { core: 0x59d4ff, coreHot: 0xffffff, communication: 0x4fc3f7, smarthome: 0x81c784, infrastructure: 0xffb74d, ai: 0xba68c8, storage: 0x8ea3b0, monitoring: 0xf06292, other: 0x9575cd, memory: 0x4dd0e1, journal: 0xff8a65, mission: 0xffd54f, cron: 0xaed581, agent: 0x80deea, tool: 0xfff176, error: 0xef5350, ok: 0x66bb6a, warn: 0xffca28, dim: 0x455a64, gold: 0xffd54f, bg: 0x020208 };

    // Easing functions shared by all modules (fx.tween also accepts names).
    NS.easing = {
        linear: function (t) { return t; },
        outCubic: function (t) { const f = t - 1; return f * f * f + 1; },
        inOutCubic: function (t) { return t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2; }
    };

    // Pool caps per quality tier. Comet caps are part of the shared contract.
    // `arcs` > 0 only in ultra: pooled electric arcs from the core to the
    // integration rings (the sensational tier-exclusive effect).
    const QUALITY = {
        ultra: { comets: 72, bursts: 24, rings: 12, particles: 1.6, arcs: 4 },
        high: { comets: 48, bursts: 16, rings: 12, particles: 1, arcs: 0 },
        medium: { comets: 28, bursts: 10, rings: 10, particles: 0.7, arcs: 0 },
        low: { comets: 14, bursts: 6, rings: 8, particles: 0.45, arcs: 0 }
    };

    const COMET_TRAIL_POINTS = 8;
    const BURST_MAX_PARTICLES = 64;
    const TRAIL_MAX_POINTS = 64;
    const RING_MAX = 12;
    const GLOW_TEXTURE_SIZE = 128;

    NS.createFx = function (inst) {
        const THREE = inst.THREE || window.THREE;

        // All pooled effect objects live under one group that is attached to
        // the stage scene lazily (fx may be created before the stage).
        const group = new THREE.Group();
        group.name = 'sysworld-fx';
        let groupAttached = false;

        function ensureGroup() {
            if (!groupAttached && inst.stage && inst.stage.scene) {
                inst.stage.scene.add(group);
                groupAttached = true;
            }
        }

        // Scratch objects reused across calls — never allocated per frame.
        const tmpVec = new THREE.Vector3();
        const tmpColor = new THREE.Color();

        // ------------------------------------------------------------------
        // Glow textures (cached — repo perf rules forbid per-frame canvases)
        // ------------------------------------------------------------------
        const textureCache = new Map();

        function glowTexture(hexColor) {
            const key = (Number(hexColor) >>> 0) || 0;
            const cached = textureCache.get(key);
            if (cached) return cached;
            const s = GLOW_TEXTURE_SIZE;
            const canvas = document.createElement('canvas');
            canvas.width = s;
            canvas.height = s;
            const ctx2d = canvas.getContext('2d');
            tmpColor.setHex(key);
            const r = Math.round(tmpColor.r * 255);
            const g = Math.round(tmpColor.g * 255);
            const b = Math.round(tmpColor.b * 255);
            const half = s / 2;
            // Radial gradient: white hot core → tinted body → transparent.
            const grad = ctx2d.createRadialGradient(half, half, 0, half, half, half);
            grad.addColorStop(0, 'rgba(255,255,255,1)');
            grad.addColorStop(0.22, 'rgba(255,255,255,0.9)');
            grad.addColorStop(0.45, 'rgba(' + r + ',' + g + ',' + b + ',0.55)');
            grad.addColorStop(1, 'rgba(' + r + ',' + g + ',' + b + ',0)');
            ctx2d.fillStyle = grad;
            ctx2d.fillRect(0, 0, s, s);
            const tex = new THREE.CanvasTexture(canvas);
            tex.needsUpdate = true;
            textureCache.set(key, tex);
            return tex;
        }

        function makeGlowSprite(hexColor, size) {
            const material = new THREE.SpriteMaterial({
                map: glowTexture(hexColor),
                color: hexColor,
                blending: THREE.AdditiveBlending,
                transparent: true,
                depthWrite: false
            });
            const sprite = new THREE.Sprite(material);
            const s = size || 1;
            sprite.scale.set(s, s, 1);
            return sprite;
        }

        // ------------------------------------------------------------------
        // Text sprites: crisp canvas labels with an accent underline, shared
        // by every module (satellites, missions, tools, daemons). Textures
        // are cached per text+color and disposed exactly once with the fx.
        // ------------------------------------------------------------------
        const textTextureCache = new Map();

        function textSprite(text, hexColor, opts) {
            const o = opts || {};
            const label = String(text == null ? '' : text).slice(0, 26);
            const key = label + '|' + ((Number(hexColor) >>> 0) & 0xffffff);
            let tex = textTextureCache.get(key);
            if (!tex) {
                const w = 256;
                const h = 64;
                const canvas = document.createElement('canvas');
                canvas.width = w;
                canvas.height = h;
                const c2 = canvas.getContext('2d');
                c2.clearRect(0, 0, w, h);
                c2.font = '600 28px "Segoe UI", system-ui, sans-serif';
                c2.textAlign = 'center';
                c2.textBaseline = 'middle';
                c2.shadowColor = 'rgba(0, 0, 0, 0.85)';
                c2.shadowBlur = 6;
                c2.fillStyle = 'rgba(235, 244, 255, 0.95)';
                c2.fillText(label, w / 2, h / 2 - 5, w - 16);
                c2.shadowBlur = 0;
                tmpColor.setHex((Number(hexColor) >>> 0) & 0xffffff);
                const r = Math.round(tmpColor.r * 255);
                const g = Math.round(tmpColor.g * 255);
                const b = Math.round(tmpColor.b * 255);
                const uw = Math.min(w * 0.62, 26 + label.length * 7.5);
                c2.fillStyle = 'rgba(' + r + ',' + g + ',' + b + ',0.85)';
                c2.fillRect(w / 2 - uw / 2, h - 13, uw, 3);
                tex = new THREE.CanvasTexture(canvas);
                tex.minFilter = THREE.LinearFilter;
                tex.generateMipmaps = false;
                textTextureCache.set(key, tex);
            }
            const mat = new THREE.SpriteMaterial({
                map: tex,
                transparent: true,
                opacity: o.opacity != null ? o.opacity : 0.9,
                depthWrite: false
            });
            const sprite = new THREE.Sprite(mat);
            const scale = o.scale || 1;
            sprite.scale.set(8 * scale, 2 * scale, 1);
            return sprite;
        }

        // ------------------------------------------------------------------
        // Quality state
        // ------------------------------------------------------------------
        let caps = QUALITY.high;
        let particleScale = 1;

        function setQuality(q) {
            caps = QUALITY[q] || QUALITY.high;
            particleScale = typeof inst.qualityScale === 'number' ? inst.qualityScale : caps.particles;
        }

        // ------------------------------------------------------------------
        // Comets: glowing head + short fading trail on a raised bezier arc
        // ------------------------------------------------------------------
        const comets = [];
        for (let i = 0; i < QUALITY.high.comets; i++) {
            const headMat = new THREE.SpriteMaterial({
                map: glowTexture(0xffffff),
                blending: THREE.AdditiveBlending,
                transparent: true,
                depthWrite: false
            });
            const head = new THREE.Sprite(headMat);
            head.visible = false;
            const hist = new Float32Array(COMET_TRAIL_POINTS * 3);
            const cols = new Float32Array(COMET_TRAIL_POINTS * 3);
            const geom = new THREE.BufferGeometry();
            const posAttr = new THREE.BufferAttribute(hist, 3);
            posAttr.setUsage(THREE.DynamicDrawUsage);
            geom.setAttribute('position', posAttr);
            geom.setAttribute('color', new THREE.BufferAttribute(cols, 3));
            const lineMat = new THREE.LineBasicMaterial({
                vertexColors: true,
                transparent: true,
                opacity: 0.85,
                blending: THREE.AdditiveBlending,
                depthWrite: false
            });
            const line = new THREE.Line(geom, lineMat);
            line.visible = false;
            line.frustumCulled = false;
            group.add(head);
            group.add(line);
            comets.push({
                active: false, head: head, headMat: headMat, line: line,
                geom: geom, lineMat: lineMat, posAttr: posAttr, hist: hist, cols: cols,
                from: new THREE.Vector3(), ctrl: new THREE.Vector3(), to: new THREE.Vector3(),
                color: 0xffffff, t: 0, duration: 1
            });
        }

        function comet(fromVec3, toVec3, hexColor, opts) {
            ensureGroup();
            let c = null;
            for (let i = 0; i < caps.comets; i++) {
                if (!comets[i].active) { c = comets[i]; break; }
            }
            if (!c) return null; // pool exhausted at the current cap: drop
            const o = opts || {};
            c.from.copy(fromVec3);
            c.to.copy(toVec3);
            c.color = hexColor;
            // Raised control point: midpoint lifted along +Y with a slight
            // sideways bend so consecutive arcs do not look identical.
            const dx = toVec3.x - fromVec3.x;
            const dy = toVec3.y - fromVec3.y;
            const dz = toVec3.z - fromVec3.z;
            const dist = Math.sqrt(dx * dx + dy * dy + dz * dz) || 1;
            const arc = typeof o.arc === 'number' ? o.arc : 0.25;
            const bend = dist * 0.06;
            c.ctrl.set(
                (fromVec3.x + toVec3.x) / 2 + (dz / dist) * bend * (Math.random() * 2 - 1),
                (fromVec3.y + toVec3.y) / 2 + dist * arc,
                (fromVec3.z + toVec3.z) / 2 - (dx / dist) * bend * (Math.random() * 2 - 1)
            );
            c.duration = Math.max(0.35, Math.min(2.6, typeof o.duration === 'number' ? o.duration : dist / 90));
            c.t = 0;
            c.active = true;
            c.headMat.map = glowTexture(hexColor);
            c.headMat.color.setHex(hexColor);
            const size = o.size || 2.2;
            c.head.scale.set(size, size, 1);
            c.head.position.copy(fromVec3);
            c.head.visible = true;
            // Reset trail history to the start point and pre-bake the fade.
            tmpColor.setHex(hexColor);
            for (let i = 0; i < COMET_TRAIL_POINTS; i++) {
                c.hist[i * 3] = fromVec3.x;
                c.hist[i * 3 + 1] = fromVec3.y;
                c.hist[i * 3 + 2] = fromVec3.z;
                const fade = Math.pow(i / (COMET_TRAIL_POINTS - 1), 1.6);
                c.cols[i * 3] = tmpColor.r * fade;
                c.cols[i * 3 + 1] = tmpColor.g * fade;
                c.cols[i * 3 + 2] = tmpColor.b * fade;
            }
            c.posAttr.needsUpdate = true;
            c.geom.attributes.color.needsUpdate = true;
            c.line.visible = true;
            return c;
        }

        function updateComets(dt) {
            for (let i = 0; i < comets.length; i++) {
                const c = comets[i];
                if (!c.active) continue;
                c.t += dt / c.duration;
                const t = Math.min(1, c.t);
                const e = 1 - t;
                // Quadratic bezier evaluated with scalars (no temp vectors).
                const px = e * e * c.from.x + 2 * e * t * c.ctrl.x + t * t * c.to.x;
                const py = e * e * c.from.y + 2 * e * t * c.ctrl.y + t * t * c.to.y;
                const pz = e * e * c.from.z + 2 * e * t * c.ctrl.z + t * t * c.to.z;
                c.head.position.set(px, py, pz);
                // Shift trail history left, append the newest head position.
                c.hist.copyWithin(0, 3);
                const last = (COMET_TRAIL_POINTS - 1) * 3;
                c.hist[last] = px;
                c.hist[last + 1] = py;
                c.hist[last + 2] = pz;
                c.posAttr.needsUpdate = true;
                if (c.t >= 1) {
                    c.active = false;
                    c.head.visible = false;
                    c.line.visible = false;
                    burst(c.to, c.color, 18);
                }
            }
        }

        // ------------------------------------------------------------------
        // Bursts: pooled particle explosions (drift + fade, ~0.8s life)
        // ------------------------------------------------------------------
        const bursts = [];
        for (let i = 0; i < QUALITY.high.bursts; i++) {
            const positions = new Float32Array(BURST_MAX_PARTICLES * 3);
            const velocities = new Float32Array(BURST_MAX_PARTICLES * 3);
            const geom = new THREE.BufferGeometry();
            const posAttr = new THREE.BufferAttribute(positions, 3);
            posAttr.setUsage(THREE.DynamicDrawUsage);
            geom.setAttribute('position', posAttr);
            geom.setDrawRange(0, 0);
            const mat = new THREE.PointsMaterial({
                size: 1.4,
                map: glowTexture(0xffffff),
                blending: THREE.AdditiveBlending,
                transparent: true,
                opacity: 0,
                depthWrite: false,
                sizeAttenuation: true
            });
            const points = new THREE.Points(geom, mat);
            points.visible = false;
            points.frustumCulled = false;
            group.add(points);
            bursts.push({
                active: false, points: points, geom: geom, mat: mat,
                posAttr: posAttr, positions: positions, velocities: velocities,
                age: 0, life: 0.8, count: 0
            });
        }

        function burst(posVec3, hexColor, count) {
            ensureGroup();
            let b = null;
            for (let i = 0; i < caps.bursts; i++) {
                if (!bursts[i].active) { b = bursts[i]; break; }
            }
            if (!b) return;
            const n = Math.max(3, Math.min(BURST_MAX_PARTICLES, Math.round((count || 16) * particleScale)));
            b.count = n;
            b.age = 0;
            b.life = 0.8;
            for (let i = 0; i < n; i++) {
                b.positions[i * 3] = posVec3.x;
                b.positions[i * 3 + 1] = posVec3.y;
                b.positions[i * 3 + 2] = posVec3.z;
                // Random direction on the unit sphere with random speed.
                let x = Math.random() * 2 - 1;
                let y = Math.random() * 2 - 1;
                let z = Math.random() * 2 - 1;
                const len = Math.sqrt(x * x + y * y + z * z) || 1;
                const speed = 5 + Math.random() * 17;
                b.velocities[i * 3] = (x / len) * speed;
                b.velocities[i * 3 + 1] = (y / len) * speed;
                b.velocities[i * 3 + 2] = (z / len) * speed;
            }
            b.mat.color.setHex(hexColor);
            b.mat.map = glowTexture(hexColor);
            b.mat.opacity = 0.95;
            b.mat.size = 1.2 + particleScale * 0.6;
            b.geom.setDrawRange(0, n);
            b.posAttr.needsUpdate = true;
            b.points.visible = true;
            b.active = true;
        }

        function updateBursts(dt) {
            const damp = Math.exp(-2.4 * dt);
            for (let i = 0; i < bursts.length; i++) {
                const b = bursts[i];
                if (!b.active) continue;
                b.age += dt;
                if (b.age >= b.life) {
                    b.active = false;
                    b.points.visible = false;
                    continue;
                }
                const k = b.age / b.life;
                for (let p = 0; p < b.count; p++) {
                    const ix = p * 3;
                    b.velocities[ix] *= damp;
                    b.velocities[ix + 1] *= damp;
                    b.velocities[ix + 2] *= damp;
                    b.positions[ix] += b.velocities[ix] * dt;
                    b.positions[ix + 1] += b.velocities[ix + 1] * dt;
                    b.positions[ix + 2] += b.velocities[ix + 2] * dt;
                }
                b.posAttr.needsUpdate = true;
                b.mat.opacity = 0.95 * (1 - k);
            }
        }

        // ------------------------------------------------------------------
        // Pulse rings: expanding transparent ring meshes (pooled, <= 12)
        // ------------------------------------------------------------------
        const ringGeom = new THREE.RingGeometry(0.92, 1, 48);
        const rings = [];
        for (let i = 0; i < RING_MAX; i++) {
            const mat = new THREE.MeshBasicMaterial({
                color: 0xffffff,
                transparent: true,
                opacity: 0,
                side: THREE.DoubleSide,
                blending: THREE.AdditiveBlending,
                depthWrite: false
            });
            const mesh = new THREE.Mesh(ringGeom, mat);
            mesh.visible = false;
            mesh.frustumCulled = false;
            group.add(mesh);
            rings.push({ active: false, mesh: mesh, mat: mat, age: 0, life: 0.9, maxRadius: 10 });
        }

        function pulseRing(posVec3, hexColor, maxRadius) {
            ensureGroup();
            let r = null;
            const cap = Math.min(caps.rings, RING_MAX);
            for (let i = 0; i < cap; i++) {
                if (!rings[i].active) { r = rings[i]; break; }
            }
            if (!r) return;
            r.age = 0;
            r.life = 0.9;
            r.maxRadius = Math.max(0.5, maxRadius || 10);
            r.mesh.position.copy(posVec3);
            r.mat.color.setHex(hexColor);
            r.mat.opacity = 0.85;
            r.mesh.scale.set(0.05, 0.05, 0.05);
            r.mesh.visible = true;
            r.active = true;
        }

        function updateRings(dt) {
            const cam = inst.stage && inst.stage.camera;
            for (let i = 0; i < rings.length; i++) {
                const r = rings[i];
                if (!r.active) continue;
                r.age += dt;
                if (r.age >= r.life) {
                    r.active = false;
                    r.mesh.visible = false;
                    continue;
                }
                const k = r.age / r.life;
                const e = NS.easing.outCubic(k);
                const s = 0.05 + e * r.maxRadius;
                r.mesh.scale.set(s, s, s);
                r.mat.opacity = 0.85 * (1 - k);
                // Billboard the ring so pulses read well from any angle.
                if (cam) r.mesh.quaternion.copy(cam.quaternion);
            }
        }

        // ------------------------------------------------------------------
        // Trails: fading position history line for moving objects (drones)
        // ------------------------------------------------------------------
        const trails = new Set();

        function trailFor(object3d, hexColor, maxPoints) {
            ensureGroup();
            const n = Math.max(2, Math.min(TRAIL_MAX_POINTS, maxPoints || 24));
            const positions = new Float32Array(n * 3);
            const colors = new Float32Array(n * 3);
            tmpColor.setHex(hexColor);
            // Pre-bake the fade gradient once (oldest point nearly invisible).
            for (let i = 0; i < n; i++) {
                const fade = Math.pow(i / (n - 1), 1.7);
                colors[i * 3] = tmpColor.r * fade;
                colors[i * 3 + 1] = tmpColor.g * fade;
                colors[i * 3 + 2] = tmpColor.b * fade;
            }
            const geom = new THREE.BufferGeometry();
            const posAttr = new THREE.BufferAttribute(positions, 3);
            posAttr.setUsage(THREE.DynamicDrawUsage);
            geom.setAttribute('position', posAttr);
            geom.setAttribute('color', new THREE.BufferAttribute(colors, 3));
            geom.setDrawRange(0, 0);
            const mat = new THREE.LineBasicMaterial({
                vertexColors: true,
                transparent: true,
                opacity: 0.8,
                blending: THREE.AdditiveBlending,
                depthWrite: false
            });
            const line = new THREE.Line(geom, mat);
            line.frustumCulled = false;
            group.add(line);

            let count = 0;
            const trail = {
                setVisible: function (v) { line.visible = !!v; },
                update: function () {
                    object3d.getWorldPosition(tmpVec);
                    // Skip recording when the object barely moved so the trail
                    // does not collapse into a dot while hovering.
                    if (count > 0) {
                        const li = (n - 1) * 3;
                        const dx = tmpVec.x - positions[li];
                        const dy = tmpVec.y - positions[li + 1];
                        const dz = tmpVec.z - positions[li + 2];
                        if (dx * dx + dy * dy + dz * dz < 0.02) return;
                    }
                    positions.copyWithin(0, 3);
                    const li = (n - 1) * 3;
                    positions[li] = tmpVec.x;
                    positions[li + 1] = tmpVec.y;
                    positions[li + 2] = tmpVec.z;
                    if (count < n) {
                        count++;
                        // Render only the newest `count` points so the baked
                        // fade gradient stays aligned (head = full color).
                        geom.setDrawRange(n - count, count);
                    }
                    posAttr.needsUpdate = true;
                },
                dispose: function () {
                    trails.delete(trail);
                    group.remove(line);
                    geom.dispose();
                    mat.dispose();
                }
            };
            trails.add(trail);
            return trail;
        }

        // ------------------------------------------------------------------
        // Selection beacon: persistent halo that tracks the focused object
        // (two counter-rotating tilted rings + soft glow). One per window.
        // ------------------------------------------------------------------
        const beaconGroup = new THREE.Group();
        beaconGroup.visible = false;
        beaconGroup.name = 'sysworld-beacon';
        const beaconRingGeom = new THREE.RingGeometry(0.94, 1, 72);
        const beaconMatA = new THREE.MeshBasicMaterial({
            color: 0xffffff,
            transparent: true,
            opacity: 0,
            side: THREE.DoubleSide,
            blending: THREE.AdditiveBlending,
            depthWrite: false
        });
        const beaconMatB = beaconMatA.clone();
        const beaconRingA = new THREE.Mesh(beaconRingGeom, beaconMatA);
        const beaconRingB = new THREE.Mesh(beaconRingGeom, beaconMatB);
        beaconRingA.frustumCulled = false;
        beaconRingB.frustumCulled = false;
        const beaconGlow = makeGlowSprite(0xffffff, 3);
        beaconGlow.material.opacity = 0;
        // Light pillar rising from the selected object.
        const beaconPillarGeom = new THREE.CylinderGeometry(0.32, 0.62, 30, 12, 1, true);
        const beaconPillarMat = new THREE.MeshBasicMaterial({
            color: 0xffffff,
            transparent: true,
            opacity: 0,
            side: THREE.DoubleSide,
            blending: THREE.AdditiveBlending,
            depthWrite: false
        });
        const beaconPillar = new THREE.Mesh(beaconPillarGeom, beaconPillarMat);
        beaconPillar.position.y = 15;
        beaconPillar.frustumCulled = false;
        // Orbiter sparks circling the halo (6 points, one shared buffer).
        const SPARK_COUNT = 6;
        const sparkPositions = new Float32Array(SPARK_COUNT * 3);
        const sparkGeom = new THREE.BufferGeometry();
        const sparkAttr = new THREE.BufferAttribute(sparkPositions, 3);
        sparkAttr.setUsage(THREE.DynamicDrawUsage);
        sparkGeom.setAttribute('position', sparkAttr);
        const sparkMat = new THREE.PointsMaterial({
            size: 0.85,
            map: glowTexture(0xffffff),
            color: 0xffffff,
            transparent: true,
            opacity: 0,
            blending: THREE.AdditiveBlending,
            depthWrite: false,
            sizeAttenuation: true
        });
        const sparkPoints = new THREE.Points(sparkGeom, sparkMat);
        sparkPoints.frustumCulled = false;
        beaconGroup.add(beaconRingA);
        beaconGroup.add(beaconRingB);
        beaconGroup.add(beaconGlow);
        beaconGroup.add(beaconPillar);
        beaconGroup.add(sparkPoints);
        group.add(beaconGroup);
        const beacon = {
            active: false,
            target: null,
            baseScale: 2,
            spin: 0,
            born: 0
        };
        let beaconElapsed = 0;

        // Attaches the halo to an object; radius is the object's world radius.
        function selectBeacon(object3d, hexColor, radius) {
            ensureGroup();
            if (!object3d) { clearBeacon(); return; }
            beacon.active = true;
            beacon.target = object3d;
            beacon.baseScale = Math.max(1.4, (radius || 1) * 1.7);
            beacon.spin = 0;
            beacon.born = beaconElapsed;
            beaconMatA.color.setHex(hexColor);
            beaconMatB.color.setHex(hexColor);
            beaconPillarMat.color.setHex(hexColor);
            sparkMat.color.setHex(hexColor);
            sparkGeom.setDrawRange(0, particleScale < 0.5 ? 3 : SPARK_COUNT);
            beaconGlow.material.color.setHex(hexColor);
            beaconGlow.material.map = glowTexture(hexColor);
            try {
                object3d.getWorldPosition(beaconGroup.position);
            } catch (_) {
                beaconGroup.position.set(0, 0, 0);
            }
            beaconGroup.visible = true;
            // Welcome ripple + particle pop at the selected object.
            pulseRing(beaconGroup.position, hexColor, beacon.baseScale * 2.4);
            burst(beaconGroup.position, hexColor, 14);
        }

        function clearBeacon() {
            beacon.active = false;
            beacon.target = null;
            beaconGroup.visible = false;
        }

        function updateBeacon(dt, elapsed) {
            beaconElapsed = elapsed;
            if (!beacon.active || !beacon.target) return;
            // The tracked mesh may have been rebuilt (graph refresh): drop.
            if (!beacon.target.parent) { clearBeacon(); return; }
            beacon.target.getWorldPosition(beaconGroup.position);
            beacon.spin += dt;
            const age = Math.min(1, (elapsed - beacon.born) / 0.5); // fade-in
            const breathe = 1 + 0.055 * Math.sin(elapsed * 2.3);
            const s = beacon.baseScale * breathe * (0.6 + 0.4 * age);
            beaconRingA.scale.set(s, s, s);
            beaconRingB.scale.set(s * 1.18, s * 1.18, s * 1.18);
            beaconRingA.rotation.set(Math.PI / 2.4, 0, beacon.spin * 0.9);
            beaconRingB.rotation.set(Math.PI / 1.7, beacon.spin * 0.7, -beacon.spin * 0.55);
            const flicker = 0.62 + 0.26 * Math.sin(elapsed * 3.1);
            beaconMatA.opacity = flicker * age;
            beaconMatB.opacity = flicker * 0.55 * age;
            const gs = beacon.baseScale * (2.6 + 0.3 * Math.sin(elapsed * 1.7));
            beaconGlow.scale.set(gs, gs, 1);
            beaconGlow.material.opacity = 0.22 * age;
            // Light pillar breathing; scales with the object so small tools
            // and big containers both read correctly.
            const ph = beacon.baseScale * 0.55;
            beaconPillar.scale.set(ph, Math.max(0.6, beacon.baseScale * 0.45), ph);
            beaconPillar.position.y = 15 * Math.max(0.6, beacon.baseScale * 0.45);
            beaconPillarMat.opacity = (0.085 + 0.045 * Math.sin(elapsed * 2.6)) * age;
            // Orbiter sparks on a tilted path around the halo.
            const sr = beacon.baseScale * 1.55;
            for (let i = 0; i < SPARK_COUNT; i++) {
                const a = beacon.spin * 1.35 + i * (Math.PI * 2 / SPARK_COUNT);
                sparkPositions[i * 3] = Math.cos(a) * sr;
                sparkPositions[i * 3 + 1] = Math.sin(elapsed * 2.1 + i * 1.7) * beacon.baseScale * 0.5;
                sparkPositions[i * 3 + 2] = Math.sin(a) * sr;
            }
            sparkAttr.needsUpdate = true;
            sparkMat.opacity = 0.85 * age;
        }

        // ------------------------------------------------------------------
        // Hover ring: a single pooled billboard ring that tracks the hovered
        // object (set null to hide). Shares the beacon's look, white-hot.
        // ------------------------------------------------------------------
        const hoverGeom = new THREE.RingGeometry(0.88, 1, 48);
        const hoverMat = new THREE.MeshBasicMaterial({
            color: 0xffffff,
            transparent: true,
            opacity: 0,
            side: THREE.DoubleSide,
            blending: THREE.AdditiveBlending,
            depthWrite: false
        });
        const hoverMesh = new THREE.Mesh(hoverGeom, hoverMat);
        hoverMesh.visible = false;
        hoverMesh.frustumCulled = false;
        group.add(hoverMesh);
        const hover = { target: null, scale: 1 };

        function hoverRing(object3d, radius, hexColor) {
            ensureGroup();
            if (!object3d) {
                hover.target = null;
                hoverMesh.visible = false;
                return;
            }
            hover.target = object3d;
            hover.scale = Math.max(0.7, (radius || 1) * 1.55);
            hoverMat.color.setHex(hexColor != null ? hexColor : 0xffffff);
            try { object3d.getWorldPosition(hoverMesh.position); } catch (_) {}
            hoverMesh.visible = true;
        }

        function updateHoverRing(dt, elapsed) {
            if (!hover.target) return;
            if (!hover.target.parent) {
                hover.target = null;
                hoverMesh.visible = false;
                return;
            }
            hover.target.getWorldPosition(hoverMesh.position);
            const s = hover.scale * (1 + 0.07 * Math.sin(elapsed * 5.1));
            hoverMesh.scale.set(s, s, s);
            hoverMat.opacity = 0.5 + 0.22 * Math.sin(elapsed * 4.3);
            const cam = inst.stage && inst.stage.camera;
            if (cam) hoverMesh.quaternion.copy(cam.quaternion);
        }

        // ------------------------------------------------------------------
        // Energy beams: short-lived additive lines between two points
        // ------------------------------------------------------------------
        const BEAM_MAX = 16;
        const beams = [];
        for (let i = 0; i < BEAM_MAX; i++) {
            const positions = new Float32Array(6);
            const geom = new THREE.BufferGeometry();
            const posAttr = new THREE.BufferAttribute(positions, 3);
            posAttr.setUsage(THREE.DynamicDrawUsage);
            geom.setAttribute('position', posAttr);
            const mat = new THREE.LineBasicMaterial({
                color: 0xffffff,
                transparent: true,
                opacity: 0,
                blending: THREE.AdditiveBlending,
                depthWrite: false
            });
            const line = new THREE.Line(geom, mat);
            line.visible = false;
            line.frustumCulled = false;
            group.add(line);
            beams.push({
                active: false, line: line, geom: geom, mat: mat, posAttr: posAttr,
                positions: positions, age: 0, life: 0.7,
                from: new THREE.Vector3(), to: new THREE.Vector3()
            });
        }

        function beam(fromVec3, toVec3, hexColor, opts) {
            ensureGroup();
            let b = null;
            for (let i = 0; i < BEAM_MAX; i++) {
                if (!beams[i].active) { b = beams[i]; break; }
            }
            if (!b) return null;
            const o = opts || {};
            b.from.copy(fromVec3);
            b.to.copy(toVec3);
            b.age = 0;
            b.life = Math.max(0.25, o.duration || 0.85);
            b.mat.color.setHex(hexColor);
            b.mat.opacity = 0.9;
            b.positions[0] = fromVec3.x; b.positions[1] = fromVec3.y; b.positions[2] = fromVec3.z;
            b.positions[3] = toVec3.x; b.positions[4] = toVec3.y; b.positions[5] = toVec3.z;
            b.posAttr.needsUpdate = true;
            b.line.visible = true;
            b.active = true;
            if (o.burst !== false) burst(toVec3, hexColor, o.burstCount || 10);
            return b;
        }

        function updateBeams(dt) {
            for (let i = 0; i < beams.length; i++) {
                const b = beams[i];
                if (!b.active) continue;
                b.age += dt;
                if (b.age >= b.life) {
                    b.active = false;
                    b.line.visible = false;
                    continue;
                }
                const k = b.age / b.life;
                b.positions[0] = b.from.x;
                b.positions[1] = b.from.y;
                b.positions[2] = b.from.z;
                b.positions[3] = b.to.x;
                b.positions[4] = b.to.y;
                b.positions[5] = b.to.z;
                b.posAttr.needsUpdate = true;
                b.mat.opacity = 0.95 * (1 - k) * (0.65 + 0.35 * Math.sin(b.age * 22));
            }
        }

        // ------------------------------------------------------------------
        // Ambient sparkles near a point (tiny short bursts)
        // ------------------------------------------------------------------
        function sparkle(posVec3, hexColor, count) {
            burst(posVec3 || tmpVec.set(0, 0, 0), hexColor || 0xffffff, count || 8);
        }

        // ------------------------------------------------------------------
        // Electric arcs (ultra only): jittered lightning lines from the core
        // to random points on the integration rings. Fully pooled — positions
        // are rewritten in place every frame, nothing is allocated.
        // ------------------------------------------------------------------
        const ARC_POINTS = 14;
        const ARC_MAX = 8;
        const arcs = [];
        for (let i = 0; i < ARC_MAX; i++) {
            const geom = new THREE.BufferGeometry();
            const posAttr = new THREE.BufferAttribute(new Float32Array(ARC_POINTS * 3), 3);
            posAttr.setUsage(THREE.DynamicDrawUsage);
            geom.setAttribute('position', posAttr);
            const mat = new THREE.LineBasicMaterial({
                color: 0x9fe8ff,
                transparent: true,
                opacity: 0,
                blending: THREE.AdditiveBlending,
                depthWrite: false
            });
            const line = new THREE.Line(geom, mat);
            line.visible = false;
            line.frustumCulled = false;
            group.add(line);
            arcs.push({
                geom: geom, posAttr: posAttr, positions: posAttr.array, mat: mat, line: line,
                tx: 0, ty: 0, tz: 0, px: 0, py: 0, pz: 0, qx: 0, qy: 0, qz: 0,
                age: 0, life: 0, active: false
            });
        }

        function respawnArc(a, initial) {
            // Random target on the inner or outer integration ring.
            const L = NS.LAYOUT || {};
            const ring = Math.random() < 0.55 ? (L.orbitInner || 26) : (L.orbitOuter || 64);
            const ang = Math.random() * Math.PI * 2;
            a.tx = Math.cos(ang) * ring;
            a.ty = (Math.random() * 2 - 1) * 6;
            a.tz = Math.sin(ang) * ring;
            // Two orthogonal jitter axes spanning the core→target direction.
            const len = Math.max(0.001, Math.sqrt(a.tx * a.tx + a.ty * a.ty + a.tz * a.tz));
            a.px = -a.tz / len; a.py = 0; a.pz = a.tx / len;
            a.qx = (a.ty * a.pz - a.tz * a.py) / 1; a.qy = (a.tz * a.px - a.tx * a.pz) / 1; a.qz = (a.tx * a.py - a.ty * a.px) / 1;
            // Lightning cadence: rare strikes with a long, soft fade-out.
            a.state = 'idle';
            a.age = 0;
            a.idle = initial ? (2.5 + Math.random() * 6) : (5 + Math.random() * 6);
            a.life = 0.6 + Math.random() * 0.4;
            a.amp = 1.6 + Math.random() * 1.4;
            a.active = true;
            a.line.visible = false;
            a.mat.opacity = 0;
        }

        function jitterArc(a, k) {
            const jitterAmp = a.amp * (1 - k * 0.5);
            for (let p = 0; p < ARC_POINTS; p++) {
                const t = p / (ARC_POINTS - 1);
                const env = Math.sin(t * Math.PI) * jitterAmp;
                const j1 = (Math.random() * 2 - 1) * env;
                const j2 = (Math.random() * 2 - 1) * env * 0.6;
                a.positions[p * 3] = a.tx * t + a.px * j1 + a.qx * j2;
                a.positions[p * 3 + 1] = a.ty * t + a.py * j1 + a.qy * j2;
                a.positions[p * 3 + 2] = a.tz * t + a.pz * j1 + a.qz * j2;
            }
            a.posAttr.needsUpdate = true;
        }

        function updateArcs(dt) {
            const want = caps.arcs || 0;
            for (let i = 0; i < arcs.length; i++) {
                const a = arcs[i];
                if (i >= want) {
                    if (a.line.visible) a.line.visible = false;
                    a.active = false;
                    continue;
                }
                if (!a.active) respawnArc(a, true);
                a.age += dt;
                if (a.state === 'idle') {
                    if (a.age < a.idle) continue;
                    a.state = 'strike';
                    a.age = 0;
                    jitterArc(a, 0);
                    a.line.visible = true;
                    continue;
                }
                const k = a.age / a.life;
                if (k >= 1) {
                    respawnArc(a, false);
                    continue;
                }
                jitterArc(a, k);
                // Violent attack, then a long smooth fade-out glow.
                const attack = k < 0.12 ? k / 0.12 : 1;
                const decay = k < 0.12 ? 1 : Math.pow(1 - (k - 0.12) / 0.88, 1.7);
                a.mat.opacity = 0.95 * attack * decay * (0.72 + 0.28 * Math.sin(a.age * 55 + i * 3));
            }
        }

        // ------------------------------------------------------------------
        // Tween runner (driven by fx.update)
        // ------------------------------------------------------------------
        const tweens = new Set();

        function tween(def) {
            const d = def || {};
            const easeFn = typeof d.ease === 'function' ? d.ease : (NS.easing[d.ease] || NS.easing.outCubic);
            const tw = {
                t: 0,
                duration: Math.max(0.0001, typeof d.duration === 'number' ? d.duration : 1),
                ease: easeFn,
                update: typeof d.update === 'function' ? d.update : null,
                done: typeof d.done === 'function' ? d.done : null
            };
            tweens.add(tw);
            return {
                cancel: function () { tweens.delete(tw); }
            };
        }

        function updateTweens(dt) {
            tweens.forEach(function (tw) {
                tw.t += dt;
                const k = Math.min(1, tw.t / tw.duration);
                if (tw.update) tw.update(tw.ease(k), k);
                if (k >= 1) {
                    tweens.delete(tw);
                    if (tw.done) tw.done();
                }
            });
        }

        // ------------------------------------------------------------------
        // Frame update / lifecycle
        // ------------------------------------------------------------------
        let disposed = false;

        function update(dt, elapsed) {
            if (disposed) return;
            ensureGroup();
            updateTweens(dt);
            updateComets(dt);
            updateBursts(dt);
            updateRings(dt);
            updateBeams(dt);
            updateArcs(dt);
            updateBeacon(dt, elapsed);
            updateHoverRing(dt, elapsed);
        }

        function dispose() {
            if (disposed) return;
            disposed = true;
            tweens.clear();
            // Trails own their geometry/material; dispose any still attached.
            trails.forEach(function (t) { t.dispose(); });
            trails.clear();
            comets.forEach(function (c) {
                c.geom.dispose();
                c.headMat.dispose();
                c.lineMat.dispose();
            });
            bursts.forEach(function (b) {
                b.geom.dispose();
                b.mat.dispose();
            });
            rings.forEach(function (r) { r.mat.dispose(); });
            ringGeom.dispose();
            beaconRingGeom.dispose();
            beaconMatA.dispose();
            beaconMatB.dispose();
            beaconGlow.material.dispose();
            beaconPillarGeom.dispose();
            beaconPillarMat.dispose();
            sparkGeom.dispose();
            sparkMat.dispose();
            hoverGeom.dispose();
            hoverMat.dispose();
            // Cached text label textures are shared — dispose them here.
            textTextureCache.forEach(function (tex) { tex.dispose(); });
            textTextureCache.clear();
            beams.forEach(function (b) {
                b.geom.dispose();
                b.mat.dispose();
            });
            arcs.forEach(function (a) {
                a.geom.dispose();
                a.mat.dispose();
            });
            // Cached canvas textures are shared — dispose them exactly here.
            textureCache.forEach(function (tex) { tex.dispose(); });
            textureCache.clear();
            if (group.parent) group.parent.remove(group);
        }

        setQuality(inst.quality || 'high');

        return {
            glowTexture: glowTexture,
            makeGlowSprite: makeGlowSprite,
            textSprite: textSprite,
            comet: comet,
            burst: burst,
            beam: beam,
            sparkle: sparkle,
            pulseRing: pulseRing,
            trailFor: trailFor,
            selectBeacon: selectBeacon,
            clearBeacon: clearBeacon,
            hoverRing: hoverRing,
            tween: tween,
            update: update,
            setQuality: setQuality,
            dispose: dispose,
            group: group
        };
    };
})();
