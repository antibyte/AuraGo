(function () {
    'use strict';

    // System World — Integrations Orbit module.
    // Renders the ~55 integration flags from GET /api/dashboard/overview
    // (`integrations`: name -> bool) as satellites traveling on three
    // inclined orbit rings around the Agent Core. Exposes
    // NS.createOrbit(inst) which returns the per-window inst.orbit module.
    // Loaded after sysworld-effects.js / sysworld-scene.js; all shared
    // constants and fx helpers are resolved lazily and guarded so this file
    // can never crash the boot sequence when something is missing.

    const NS = window.SysWorld = window.SysWorld || {};

    // Fallback mirrors of the shared contract constants. The authoritative
    // values live on NS.PALETTE / NS.LAYOUT (defined by the foundation
    // modules); these only protect against a missing foundation.
    const FALLBACK_PALETTE = {
        coreHot: 0xffffff,
        communication: 0x4fc3f7,
        smarthome: 0x81c784,
        infrastructure: 0xffb74d,
        ai: 0xba68c8,
        storage: 0x8ea3b0,
        monitoring: 0xf06292,
        other: 0x9575cd,
        dim: 0x455a64
    };
    const FALLBACK_LAYOUT = { orbitInner: 26, orbitOuter: 64 };

    const TWO_PI = Math.PI * 2;
    const DEG = Math.PI / 180;

    // Fixed integration -> category map. Keys are integration names exactly
    // as they appear in the dashboard payload, lowercased. Anything not
    // listed here falls into 'other'.
    const CATEGORY_MAP = {
        telegram: 'communication', discord: 'communication', email: 'communication',
        rocketchat: 'communication', a2a: 'communication', webhooks: 'communication',
        telnyx: 'communication', matrix: 'communication',
        home_assistant: 'smarthome', mqtt: 'smarthome', fritzbox: 'smarthome',
        evomap: 'smarthome',
        docker: 'infrastructure', proxmox: 'infrastructure', tailscale: 'infrastructure',
        cloudflare_tunnel: 'infrastructure', truenas: 'infrastructure', ansible: 'infrastructure',
        invasion: 'infrastructure', meshcentral: 'infrastructure', sandbox: 'infrastructure',
        virtual_computers: 'infrastructure',
        ollama: 'ai', helper_llm: 'ai', fallback_llm: 'ai', tts: 'ai', piper: 'ai',
        supertonic: 'ai', ai_gateway: 'ai', mcp: 'ai', mcp_server: 'ai',
        embeddings: 'ai', realtime_speech: 'ai',
        s3: 'storage', koofr: 'storage', webdav: 'storage', jellyfin: 'storage',
        obsidian: 'storage', netlify: 'storage', vercel: 'storage', huggingface: 'storage',
        adguard: 'monitoring', uptime_kuma: 'monitoring', grafana: 'monitoring',
        budget: 'monitoring'
    };

    // Fixed category -> ring assignment, chosen so the three rings end up
    // roughly balanced (ai+monitoring / communication+storage+other /
    // infrastructure+smarthome).
    const CATEGORY_RING = {
        ai: 0,
        monitoring: 0,
        communication: 1,
        storage: 1,
        other: 1,
        infrastructure: 2,
        smarthome: 2
    };

    // Gap left between neighboring category arc segments on a ring.
    const ARC_GAP = 0.12;

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

    function categorize(id) {
        return CATEGORY_MAP[id] || 'other';
    }

    function categoryColor(category) {
        const P = pal();
        return P[category] != null ? P[category] : P.other;
    }

    // Human-friendly label for hover/click panels: 'home_assistant' ->
    // 'Home Assistant'.
    function prettify(id) {
        return String(id).replace(/_/g, ' ').replace(/\b\w/g, function (ch) {
            return ch.toUpperCase();
        });
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

    // Disposes one material; fx-provided textures are treated as shared and
    // left alone, only locally created textures are freed.
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

    // Null-object module returned when THREE or the stage is missing so the
    // entry can still wire the app without defensive checks everywhere.
    function nullModule(inst) {
        const api = {
            group: null,
            pickables: [],
            setIntegrations: noop,
            satellitePosition: function () { return null; },
            update: noop,
            dispose: noop
        };
        if (inst) inst.orbit = api;
        return api;
    }

    NS.createOrbit = function (inst) {
        const THREE = (inst && inst.THREE) || window.THREE;
        if (!THREE || !inst || !inst.stage || !inst.stage.scene) {
            return nullModule(inst);
        }

        const P = pal();
        const L = lay();
        const fx = inst.fx || null;
        const inner = Math.max(1, toFinite(L.orbitInner, 26));
        const outer = Math.max(inner + 1, toFinite(L.orbitOuter, 64));

        // All mutable per-window state lives here; nothing is cached in
        // module globals so multiple desktop windows stay independent.
        const state = {
            disposed: false,
            sats: new Map(),
            signature: ''
        };

        const group = new THREE.Group();
        group.name = 'sysworld-orbit';

        let ownedGlowTex = null;

        function glowTexture() {
            if (fx && typeof fx.glowTexture === 'function') {
                const tex = fx.glowTexture(P.other);
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
                opacity: 0.9,
                blending: THREE.AdditiveBlending,
                depthWrite: false
            }));
            sprite.scale.set(size, size, 1);
            return sprite;
        }

        // ----------------------------------------------------------------
        // Ring definitions: radii spread within orbitInner..orbitOuter,
        // tilts +18 / 0 / -18 degrees, alternating travel directions.
        // Precomputed trig keeps the per-frame position math allocation-free.
        // ----------------------------------------------------------------
        const span = outer - inner;
        const rings = [
            { radius: inner + span * 0.16, tilt: 18 * DEG, yaw: 0.0, speed: 0.055 },
            { radius: inner + span * 0.50, tilt: 0.0, yaw: 0.8, speed: -0.042 },
            { radius: inner + span * 0.84, tilt: -18 * DEG, yaw: 1.7, speed: 0.034 }
        ].map(function (ring, index) {
            ring.index = index;
            ring.cosTilt = Math.cos(ring.tilt);
            ring.sinTilt = Math.sin(ring.tilt);
            ring.cosYaw = Math.cos(ring.yaw);
            ring.sinYaw = Math.sin(ring.yaw);
            // Organic irregularity: deterministic three-frequency radial
            // wobble per ring, shared by the guide loop and the satellites,
            // so orbits read as natural paths instead of perfect circles.
            ring.w1 = index * 1.7 + 0.4;
            ring.w2 = index * 2.3 + 1.9;
            ring.w3 = index * 3.1 + 3.7;
            ring.wobbleAmp = 0.045 + index * 0.012;
            return ring;
        });

        // Computes a point on a ring for a given orbit angle. Mirrors the
        // transform baked into the ring guide loops below: yaw around Y,
        // then tilt around X. Writes into `out` (x/y/z components only).
        function ringPoint(ring, angle, out) {
            const wob = 1 + ring.wobbleAmp * (
                Math.sin(angle * 3 + ring.w1) * 0.5 +
                Math.sin(angle * 7 + ring.w2) * 0.3 +
                Math.sin(angle * 13 + ring.w3) * 0.2);
            const cx = Math.cos(angle) * ring.radius * wob;
            const cz = Math.sin(angle) * ring.radius * wob;
            // Yaw around the Y axis.
            const x1 = cx * ring.cosYaw + cz * ring.sinYaw;
            const z1 = -cx * ring.sinYaw + cz * ring.cosYaw;
            // Tilt around the X axis (y stays 0 before this step).
            out.x = x1;
            out.y = -z1 * ring.sinTilt;
            out.z = z1 * ring.cosTilt;
            return out;
        }

        const tmpPos = { x: 0, y: 0, z: 0 };

        // Faint guide loop per ring so the orbit planes are visible.
        const ringLoops = [];
        rings.forEach(function (ring) {
            const segs = 160;
            const positions = new Float32Array(segs * 3);
            for (let i = 0; i < segs; i++) {
                ringPoint(ring, (i / segs) * TWO_PI, tmpPos);
                positions[i * 3] = tmpPos.x;
                positions[i * 3 + 1] = tmpPos.y;
                positions[i * 3 + 2] = tmpPos.z;
            }
            const geo = new THREE.BufferGeometry();
            geo.setAttribute('position', new THREE.BufferAttribute(positions, 3));
            const mat = new THREE.LineBasicMaterial({
                color: P.dim,
                transparent: true,
                opacity: 0.5,
                blending: THREE.AdditiveBlending,
                depthWrite: false
            });
            const loop = new THREE.LineLoop(geo, mat);
            loop.name = 'sysworld-orbit-ring-' + ring.index;
            group.add(loop);
            ringLoops.push(loop);
        });

        // Light mist hugging each ring: soft scattered points along the
        // wobbled orbit path, slowly drifting (built once, cheap per frame).
        const fogRng = (function () { let s = 1337; return function () { s = (s * 1664525 + 1013904223) >>> 0; return s / 4294967296; }; })();
        const fogClouds = [];
        rings.forEach(function (ring) {
            const count = 240;
            const positions = new Float32Array(count * 3);
            for (let i = 0; i < count; i++) {
                const angle = fogRng() * TWO_PI;
                ringPoint(ring, angle, tmpPos);
                const spread = 2.4 + fogRng() * 3.4;
                positions[i * 3] = tmpPos.x + (fogRng() * 2 - 1) * spread;
                positions[i * 3 + 1] = tmpPos.y + (fogRng() * 2 - 1) * spread * 0.55;
                positions[i * 3 + 2] = tmpPos.z + (fogRng() * 2 - 1) * spread;
            }
            const geo = new THREE.BufferGeometry();
            geo.setAttribute('position', new THREE.BufferAttribute(positions, 3));
            const mat = new THREE.PointsMaterial({
                size: 2.6,
                map: glowTexture(),
                color: 0x86aede,
                transparent: true,
                opacity: 0.11,
                blending: THREE.AdditiveBlending,
                depthWrite: false,
                sizeAttenuation: true
            });
            const cloud = new THREE.Points(geo, mat);
            cloud.frustumCulled = false;
            cloud.name = 'sysworld-orbit-fog-' + ring.index;
            group.add(cloud);
            fogClouds.push({ cloud: cloud, mat: mat, geo: geo, ring: ring });
        });

        // Shared geometries for every satellite (disposed once, with the
        // module — never per satellite). Each category gets its own low-poly
        // shape so the orbit reads as a taxonomy at a glance; the same
        // geometry is reused for the translucent wireframe shell.
        const categoryGeos = {
            communication: new THREE.IcosahedronGeometry(0.55, 0),
            smarthome: new THREE.BoxGeometry(0.95, 0.95, 0.95),
            infrastructure: new THREE.OctahedronGeometry(0.55, 0),
            ai: new THREE.DodecahedronGeometry(0.55, 0),
            storage: new THREE.CylinderGeometry(0.5, 0.5, 0.85, 10),
            monitoring: new THREE.ConeGeometry(0.52, 0.95, 10),
            other: new THREE.SphereGeometry(0.5, 14, 10)
        };
        function categoryGeo(category) {
            return categoryGeos[category] || categoryGeos.other;
        }
        // Per-category shared wireframe material for the outer shell.
        const wireMatCache = new Map();
        function wireMatFor(category) {
            let m = wireMatCache.get(category);
            if (!m) {
                m = new THREE.MeshBasicMaterial({
                    color: categoryColor(category),
                    wireframe: true,
                    transparent: true,
                    opacity: 0.25,
                    blending: THREE.AdditiveBlending,
                    depthWrite: false
                });
                wireMatCache.set(category, m);
            }
            return m;
        }
        // Shared bright inner core (identical for every satellite).
        const coreGeo = new THREE.SphereGeometry(0.22, 10, 8);
        const coreMat = new THREE.MeshBasicMaterial({
            color: P.coreHot != null ? P.coreHot : 0xffffff,
            transparent: true,
            opacity: 0.95,
            blending: THREE.AdditiveBlending,
            depthWrite: false
        });
        const sharedRingGeo = new THREE.TorusGeometry(1.05, 0.03, 6, 40);

        const pickables = [];

        // Applies the enabled/disabled look: color, glow, beam, wireframe
        // shell, accent ring and travel speed. Fires a small burst plus a
        // pulse ring when a satellite turns on.
        function applyEnabled(sat, enabled, celebrate) {
            const hex = categoryColor(sat.category);
            sat.enabled = enabled;
            sat.mesh.userData.enabled = enabled;
            sat.speed = rings[sat.ring].speed * (enabled ? 1 : 0.55);
            sat.mesh.material.color.setHex(enabled ? hex : P.dim);
            sat.glow.material.color.setHex(enabled ? hex : P.dim);
            sat.glow.material.opacity = enabled ? 0.9 : 0.1;
            sat.shell.visible = enabled;
            sat.beam.visible = enabled;
            sat.accent.visible = enabled;
            if (enabled && celebrate && fx) {
                if (typeof fx.burst === 'function') {
                    fx.burst(sat.node.position, hex, 12);
                }
                if (typeof fx.pulseRing === 'function') {
                    fx.pulseRing(sat.node.position, hex, 6);
                }
            }
        }

        function createSatellite(id, enabled) {
            const category = categorize(id);
            const hex = categoryColor(category);
            const ringIndex = CATEGORY_RING[category] != null ? CATEGORY_RING[category] : 1;

            const node = new THREE.Object3D();
            node.name = 'sysworld-sat-' + id;

            const mesh = new THREE.Mesh(categoryGeo(category), new THREE.MeshBasicMaterial({ color: hex }));
            mesh.userData = {
                kind: 'integration',
                id: id,
                label: prettify(id),
                enabled: enabled,
                category: category
            };
            node.add(mesh);

            // Bright inner core marking the "live" center of the solid.
            const core = new THREE.Mesh(coreGeo, coreMat);
            node.add(core);

            // Translucent wireframe shell echoing the category shape.
            const shell = new THREE.Mesh(categoryGeo(category), wireMatFor(category));
            shell.scale.set(1.35, 1.35, 1.35);
            node.add(shell);

            const glow = makeGlow(hex, 2.6);
            node.add(glow);

            // Name label under the satellite; opacity is distance-faded per
            // frame in update(). Skipped entirely without the effects module.
            let label = null;
            if (fx && typeof fx.textSprite === 'function') {
                label = fx.textSprite(prettify(id), hex, { scale: 0.55 });
                if (label) {
                    label.position.set(0, -2.0, 0);
                    node.add(label);
                }
            }

            // Thin accent ring circling the satellite (enabled state only).
            const accent = new THREE.Mesh(sharedRingGeo, new THREE.MeshBasicMaterial({
                color: hex,
                transparent: true,
                opacity: 0.55,
                blending: THREE.AdditiveBlending,
                depthWrite: false
            }));
            accent.rotation.x = 1.15;
            node.add(accent);

            // Beam line to the origin, built once and reused; only the far
            // endpoint is rewritten per frame.
            const beamGeo = new THREE.BufferGeometry();
            const beamAttr = new THREE.BufferAttribute(new Float32Array(6), 3);
            beamAttr.setUsage(THREE.DynamicDrawUsage);
            beamGeo.setAttribute('position', beamAttr);
            const beam = new THREE.Line(beamGeo, new THREE.LineBasicMaterial({
                color: hex,
                transparent: true,
                opacity: 0.22,
                blending: THREE.AdditiveBlending,
                depthWrite: false
            }));
            beam.frustumCulled = false;

            group.add(node);
            group.add(beam);

            const sat = {
                id: id,
                category: category,
                ring: ringIndex,
                enabled: enabled,
                node: node,
                mesh: mesh,
                core: core,
                shell: shell,
                glow: glow,
                label: label,
                labelBase: 0.9,
                accent: accent,
                beam: beam,
                beamAttr: beamAttr,
                baseAngle: 0,
                phase: 0,
                speed: 0,
                bobPhase: Math.random() * TWO_PI,
                spawnTimer: null,
                spawnTween: null
            };
            applyEnabled(sat, enabled, false);
            state.sats.set(id, sat);
            pickables.push(mesh);
            return sat;
        }

        // Scales a freshly created satellite from a pinpoint to full size,
        // delayed by its stagger slot. The timer/tween handles live on the
        // satellite record so removal and dispose() can cancel them safely.
        function spawnIn(sat, delayMs) {
            if (!fx || typeof fx.tween !== 'function') {
                sat.node.scale.set(1, 1, 1);
                return;
            }
            sat.node.scale.set(0.001, 0.001, 0.001);
            sat.spawnTimer = setTimeout(function () {
                sat.spawnTimer = null;
                if (state.disposed || !state.sats.has(sat.id)) return;
                sat.spawnTween = fx.tween({
                    duration: 0.5,
                    ease: 'outCubic',
                    update: function (e) {
                        const s = 0.001 + 0.999 * e;
                        sat.node.scale.set(s, s, s);
                    },
                    done: function () {
                        sat.spawnTween = null;
                        sat.node.scale.set(1, 1, 1);
                    }
                });
            }, delayMs);
        }

        function removeSatellite(sat) {
            const seenTextures = new Set();
            if (sat.spawnTimer) {
                clearTimeout(sat.spawnTimer);
                sat.spawnTimer = null;
            }
            if (sat.spawnTween) {
                sat.spawnTween.cancel();
                sat.spawnTween = null;
            }
            group.remove(sat.node);
            group.remove(sat.beam);
            // Shared category/core/ring geometries and shared core/wireframe
            // materials stay alive; only per-satellite materials and the beam
            // geometry are disposed here.
            disposeMaterialDeep(sat.mesh.material, seenTextures);
            disposeMaterialDeep(sat.glow.material, seenTextures);
            if (sat.label) disposeMaterialDeep(sat.label.material, seenTextures);
            disposeMaterialDeep(sat.accent.material, seenTextures);
            sat.beam.geometry.dispose();
            disposeMaterialDeep(sat.beam.material, seenTextures);
            const ix = pickables.indexOf(sat.mesh);
            if (ix >= 0) pickables.splice(ix, 1);
            state.sats.delete(sat.id);
        }

        // Deterministic slot assignment: within each ring, categories are
        // sorted and receive equal arc segments; satellites take a slot by
        // their sorted index inside the category so same-category
        // satellites cluster along a shared arc.
        function relayout() {
            const byRing = [[], [], []];
            state.sats.forEach(function (sat) {
                byRing[sat.ring].push(sat);
            });
            byRing.forEach(function (list) {
                if (!list.length) return;
                const cats = {};
                list.forEach(function (sat) {
                    (cats[sat.category] = cats[sat.category] || []).push(sat);
                });
                const catNames = Object.keys(cats).sort();
                const segSpan = TWO_PI / catNames.length;
                const usable = Math.max(0.05, segSpan - ARC_GAP);
                catNames.forEach(function (cat, ci) {
                    const members = cats[cat].sort(function (a, b) {
                        return a.id < b.id ? -1 : (a.id > b.id ? 1 : 0);
                    });
                    members.forEach(function (sat, si) {
                        sat.baseAngle = ci * segSpan + ARC_GAP * 0.5 + usable * ((si + 0.5) / members.length);
                        sat.phase = 0;
                    });
                });
            });
        }

        // Places a satellite on its ring for an absolute elapsed time.
        function positionSatellite(sat, elapsed) {
            const ring = rings[sat.ring];
            ringPoint(ring, sat.baseAngle + sat.phase, tmpPos);
            const bob = Math.sin(elapsed * 0.8 + sat.bobPhase) * 0.4;
            const x = tmpPos.x;
            const y = tmpPos.y + bob;
            const z = tmpPos.z;
            sat.node.position.set(x, y, z);
            const arr = sat.beamAttr.array;
            arr[3] = x;
            arr[4] = y;
            arr[5] = z;
            sat.beamAttr.needsUpdate = true;
        }

        // ----------------------------------------------------------------
        // Public API
        // ----------------------------------------------------------------

        // Diffs the incoming integration map against the previous state:
        // new keys are built, vanished keys are disposed, changed flags are
        // updated in place. A relayout only happens when membership changes.
        function setIntegrations(integrationsMap) {
            if (state.disposed) return;
            const source = integrationsMap || {};
            const next = {};
            Object.keys(source).forEach(function (key) {
                const id = String(key).toLowerCase().trim();
                if (id) next[id] = !!source[key];
            });

            const signature = Object.keys(next).sort().join('|');
            const membershipChanged = signature !== state.signature;

            if (membershipChanged) {
                // Remove vanished satellites.
                const doomed = [];
                state.sats.forEach(function (sat, id) {
                    if (!Object.prototype.hasOwnProperty.call(next, id)) doomed.push(sat);
                });
                doomed.forEach(removeSatellite);
                // Create newcomers.
                const newcomers = [];
                Object.keys(next).forEach(function (id) {
                    if (!state.sats.has(id)) {
                        const sat = createSatellite(id, next[id]);
                        positionSatellite(sat, 0);
                        newcomers.push(sat);
                    }
                });
                relayout();
                state.signature = signature;
                // Snap everyone onto their freshly assigned slots right
                // away instead of waiting for the next animation frame.
                state.sats.forEach(function (sat) {
                    positionSatellite(sat, 0);
                });
                // Spawn-in animation, staggered by relayout order.
                newcomers.forEach(function (sat, ix) {
                    spawnIn(sat, ix * 40);
                });
            }

            // Enabled-state transitions apply in place on every call.
            Object.keys(next).forEach(function (id) {
                const sat = state.sats.get(id);
                if (sat && sat.enabled !== next[id]) {
                    applyEnabled(sat, next[id], true);
                }
            });
        }

        // Live world position of a satellite (used as a comet target by
        // other modules). Returns null for unknown ids.
        function satellitePosition(id) {
            const sat = state.sats.get(String(id || '').toLowerCase());
            return sat ? sat.node.position : null;
        }

        function update(dt, elapsed) {
            if (state.disposed) return;
            dt = clamp(toFinite(dt, 0), 0, 0.1);
            elapsed = toFinite(elapsed, 0);
            const cam = inst.stage.camera;
            const camPos = cam ? cam.position : null;
            state.sats.forEach(function (sat) {
                sat.phase += dt * sat.speed;
                positionSatellite(sat, elapsed);
                // Gentle self-rotation; enabled satellites spin livelier.
                sat.mesh.rotation.y += dt * (sat.enabled ? 1.1 : 0.3);
                // Tumbling the tilted accent ring plane (a pure z spin would
                // be invisible on a uniform torus).
                sat.accent.rotation.y += dt * 0.6;
                // Distance-fade the name label: full base opacity up close,
                // linear fade-out to 120 units; disabled satellites keep a
                // faint remnant so the orbit stays readable.
                if (sat.label && camPos) {
                    const p = sat.node.position;
                    const dx = p.x - camPos.x;
                    const dy = p.y - camPos.y;
                    const dz = p.z - camPos.z;
                    const dist = Math.sqrt(dx * dx + dy * dy + dz * dz);
                    const fade = clamp((120 - dist) / 65, 0, 1);
                    let op = sat.labelBase * fade;
                    if (!sat.enabled) op *= 0.35;
                    sat.label.material.opacity = op;
                    sat.label.visible = op > 0.01;
                }
            });
            // Mist drifts slowly around each ring and gently breathes.
            for (let i = 0; i < fogClouds.length; i++) {
                const f = fogClouds[i];
                f.cloud.rotation.y += dt * 0.014 * (i % 2 ? -1 : 1);
                f.mat.opacity = 0.09 + 0.035 * Math.sin(elapsed * 0.4 + i * 2.1);
            }
        }

        function dispose() {
            if (state.disposed) return;
            state.disposed = true;
            const all = [];
            state.sats.forEach(function (sat) { all.push(sat); });
            all.forEach(removeSatellite);
            ringLoops.forEach(function (loop) {
                group.remove(loop);
                loop.geometry.dispose();
                disposeMaterialDeep(loop.material, new Set());
            });
            ringLoops.length = 0;
            fogClouds.forEach(function (f) {
                group.remove(f.cloud);
                f.geo.dispose();
                f.mat.dispose(); // material only — the glow texture is shared
            });
            fogClouds.length = 0;
            Object.keys(categoryGeos).forEach(function (key) {
                categoryGeos[key].dispose();
            });
            wireMatCache.forEach(function (m) { m.dispose(); });
            wireMatCache.clear();
            coreGeo.dispose();
            coreMat.dispose();
            sharedRingGeo.dispose();
            if (ownedGlowTex) {
                ownedGlowTex.dispose();
                ownedGlowTex = null;
            }
            if (group.parent) group.parent.remove(group);
        }

        const api = {
            group: group,
            pickables: pickables,
            setIntegrations: setIntegrations,
            satellitePosition: satellitePosition,
            update: update,
            dispose: dispose
        };

        inst.stage.scene.add(group);
        inst.orbit = api;
        return api;
    };
})();
