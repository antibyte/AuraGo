/* Aura Drift - 3D actors: car rigs (livery, wheels, suspension, lamps, funny-car parts), ghost, crates.
 *
 * Space: three space is [x, up, y_sim]. A car group uses rotation order 'YZX' with
 * y = -psi, z = pitch (positive nose up), x = roll (positive right side down), exactly as the
 * sim publishes them (section 2.2). Wheels use order 'YXZ': y = -steer on the front axle,
 * z = -spin. The group pivots around the body centre so flips do not swing through the ground. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;
    const CRATE_RESPAWN = 8;

    function spring(s, key, target, k, c, dt) {
        const n = Math.max(1, Math.ceil(dt / 0.012));
        const h = dt / n;
        for (let i = 0; i < n; i++) {
            s[key + 'V'] += (k * (target - s[key]) - c * s[key + 'V']) * h;
            s[key] += s[key + 'V'] * h;
        }
    }

    /* A car in the scene. opts: { livery, ghost, gl, night }. */
    AD.createCarRig = function (T, carDef, opts) {
        opts = opts || {};
        const template = AD.assets.car(carDef.id);
        const root = template.clone(true);
        const group = new T.Group();
        group.rotation.order = 'YZX';
        const pivot = new T.Group();
        group.add(pivot);
        pivot.add(root);
        const scale = carDef.visualScale || 1;
        group.scale.setScalar(scale);

        const owned = [];
        const own = m => { owned.push(m); return m; };
        const pick = name => root.getObjectByName(name);
        const rig = {
            id: carDef.id, group: group, template: template, ghost: !!opts.ghost, gl: opts.gl, night: !!opts.night, scale: scale,
            st: { spin: 0, spinRate: 0, pitch: 0, pitchV: 0, roll: 0, rollV: 0, dip: 0, dipV: 0, ax: 0, axV: 0, lat: 0, latV: 0, gw: 1, brake: 0, lastVx: 0, wheelSpeed: 0, blade: 0 }
        };

        const livery = carDef.liveries[(opts.livery | 0) % carDef.liveries.length] || carDef.colour;
        const paintHex = rig.ghost ? '#9be7ff' : livery;
        const accentHex = rig.ghost ? '#e6fbff' : AD.accentColour(carDef);
        const paintMeshes = [], accentMeshes = [];
        let brakeMat = null, headMat = null;
        const wheels = [];
        const sharedDone = new Map();

        root.traverse(o => {
            if (!o.isMesh) return;
            const src = o.material;
            if (!src || Array.isArray(src)) return;
            o.userData.srcName = src.name;
            if (src.name === 'paint') paintMeshes.push(o);
            else if (src.name === 'paint_accent') accentMeshes.push(o);
            else if (src.name === 'brake_light') { brakeMat = brakeMat || own(src.clone()); o.material = brakeMat; }
            else if (src.name === 'head_light') { headMat = headMat || own(src.clone()); o.material = headMat; }
            else {
                AD.carTune(src);
                if (rig.ghost) {
                    if (!sharedDone.has(src)) {
                        const c = own(src.clone());
                        c.transparent = true; c.opacity = 0.35; c.depthWrite = false;
                        sharedDone.set(src, c);
                    }
                    o.material = sharedDone.get(src);
                }
            }
        });
        if (rig.ghost) for (const m of [brakeMat, headMat]) if (m) { m.transparent = true; m.opacity = 0.5; m.depthWrite = false; }

        let lookKey = '';
        function applyPaint(gl) {
            lookKey = '';
            for (const [list, hex] of [[paintMeshes, paintHex], [accentMeshes, accentHex]]) {
                if (!list.length) continue;
                const old = list[0].material;
                const mat = own(AD.carPaint(T, hex, gl, !!list[0].geometry.attributes.color, rig.ghost, opts.rim));
                for (const o of list) o.material = mat;
                if (old && owned.indexOf(old) >= 0) { old.dispose(); owned.splice(owned.indexOf(old), 1); }
            }
            rig.gl = gl; rig.castNear = undefined;
            root.traverse(o => {
                if (!o.isMesh) return;
                const n = o.userData.srcName;
                o.castShadow = !rig.ghost && !!gl.shadows && n !== 'glass' && n !== 'head_light' && n !== 'brake_light';
                o.receiveShadow = !rig.ghost && !!gl.shadows && n !== 'glass';
            });
        }

        for (const name of ['wheel_fl', 'wheel_fr', 'wheel_rl', 'wheel_rr']) {
            const node = pick(name);
            if (!node) continue;
            node.rotation.order = 'YXZ';
            let radius = 0.4;
            const mesh = node.isMesh ? node : null;
            if (mesh && mesh.geometry) {
                mesh.geometry.computeBoundingBox();
                radius = (mesh.geometry.boundingBox.max.y - mesh.geometry.boundingBox.min.y) / 2 || radius;
            }
            wheels.push({ node: node, front: name.indexOf('_f') > 0, radius: radius });
        }
        rig.wheels = wheels;
        const wInst = AD.carWheelInst ? AD.carWheelInst(T, wheels, root) : null;
        if (wInst) owned.push(wInst.mesh);
        rig.parts = { cone: pick('cone'), blade: pick('blade'), gnome: pick('gnome'), lamp: pick('lamp'), cushion: pick('cushion') };
        if (rig.parts.cushion) rig.parts.cushionY = rig.parts.cushion.position.y;

        /* Body-centre pivot and the true size, measured once. */
        root.updateMatrixWorld(true);
        const box = new T.Box3().setFromObject(root);
        const cy = clamp((box.max.y - 0) * 0.36, 0.3, 1.2);
        root.position.y = -cy;
        rig.cy = cy;
        rig.size = { l: (box.max.x - box.min.x) * scale, w: (box.max.z - box.min.z) * scale, h: box.max.y * scale };
        applyPaint(opts.gl || AD.QUALITY.high.gl);

        rig.setLights = function (night) {
            rig.night = !!night;
            if (headMat) headMat.emissiveIntensity = night ? 3.6 : 1.1;
        };
        rig.setLights(rig.night);
        rig.setGl = applyPaint;
        rig.fxRoot = pivot;

        /* Status looks: soot tint, emissive glow and cloak transparency. o = { soot 0..1, glow [r,g,b], alpha 0..1 }.
           The shared template materials are cloned once, the first time a car turns see-through. */
        const sootColour = new T.Color('#17120e');
        let cloakMats = null;
        function ensureCloakMats() {
            if (cloakMats) return;
            cloakMats = new Map();
            root.traverse(o => {
                if (!o.isMesh || !o.material || Array.isArray(o.material) || paintMeshes.indexOf(o) >= 0 || accentMeshes.indexOf(o) >= 0) return;
                const src = o.material;
                if (owned.indexOf(src) < 0) {
                    if (!cloakMats.has(src)) cloakMats.set(src, own(src.clone()));
                    o.material = cloakMats.get(src);
                }
                cloakMats.set(o.material, o.material);
            });
        }
        function setAlpha(m, a) {
            const clear = a < 0.995;
            if (m.transparent !== clear) { m.transparent = clear; m.needsUpdate = true; }
            m.opacity = a;
            m.depthWrite = !clear;
        }
        rig.look = function (o) {
            const key = o.soot.toFixed(2) + '|' + o.glow[0].toFixed(2) + '|' + o.glow[1].toFixed(2) + '|' + o.glow[2].toFixed(2) + '|' + o.alpha.toFixed(2);
            if (key === lookKey || rig.ghost) return;
            lookKey = key;
            for (const [list, hex] of [[paintMeshes, paintHex], [accentMeshes, accentHex]]) {
                const m = list.length ? list[0].material : null;
                if (!m) continue;
                m.color.set(hex).lerp(sootColour, o.soot * 0.85);
                m.emissive.setRGB(o.glow[0], o.glow[1], o.glow[2]);
                setAlpha(m, o.alpha);
            }
            if (o.alpha < 0.995) ensureCloakMats();
            if (cloakMats) for (const m of new Set(cloakMats.values())) setAlpha(m, o.alpha);
        };

        /* Presentation impulses from sim events. */
        rig.kick = function (kind, power) {
            const st = rig.st;
            if (kind === 'landing') { st.dipV -= 2.2 * power; st.pitchV -= 1.2 * power; }
            else if (kind === 'hit') { st.rollV += (power > 0 ? 1 : -1) * 2.2; st.dipV -= 0.8; }
        };

        rig.update = function (v, dt, ctx) {
            const st = rig.st;
            group.visible = !v.out;
            if (!group.visible) return;
            const air = !!v.airborne;
            if (dt > 0) {
                const rawAx = (v.vx - st.lastVx) / dt;
                const ax = clamp(rawAx, -30, 30), lat = clamp(v.vx * v.r, -26, 26);
                st.lastVx = v.vx;
                st.gw += ((air ? 0 : 1) - st.gw) * (1 - Math.exp(-14 * dt));
                spring(st, 'pitch', air ? 0 : clamp(ax * 0.0042, -0.06, 0.06), 130, 14, dt);
                /* The body rolls to the outside of the corner: right turn (r > 0) = left side down = negative roll. */
                spring(st, 'roll', air ? 0 : clamp(-lat * 0.0028, -0.06, 0.06), 120, 13, dt);
                spring(st, 'dip', 0, 150, 9, dt);
                /* Slosh of tall parts: underdamped, lags the acceleration. */
                spring(st, 'ax', ax, 55, 2.6, dt);
                spring(st, 'lat', lat, 55, 2.6, dt);
                const wheelTarget = v.vx;
                st.wheelSpeed += (wheelTarget - st.wheelSpeed) * (1 - Math.exp(-(air ? 0.6 : 18) * dt));
                st.blade += dt * (22 + Math.abs(v.vx) * 0.6);
                for (const w of wheels) {
                    if (v.handbrake && !w.front && !air) continue;
                    w.spin = (w.spin || 0) + st.wheelSpeed / (w.radius * rig.scale) * dt;
                }
            }
            group.position.set(v.x, v.z + rig.cy * rig.scale + st.dip * st.gw, v.y);
            group.rotation.set(v.roll + st.roll * st.gw, -v.psi, v.pitch + st.pitch * st.gw);
            for (const w of wheels) {
                w.node.rotation.y = w.front ? -v.steer : 0;
                w.node.rotation.z = -(w.spin || 0);
            }
            if (wInst) wInst.sync();
            const p = rig.parts;
            if (p.blade) p.blade.rotation.y = st.blade;
            if (p.cone) { p.cone.rotation.z = clamp(st.ax * 0.011, -0.4, 0.4); p.cone.rotation.x = clamp(-st.lat * 0.011, -0.4, 0.4); }
            if (p.lamp) { p.lamp.rotation.z = clamp(st.ax * 0.014, -0.5, 0.5); p.lamp.rotation.x = clamp(-st.lat * 0.014, -0.5, 0.5); }
            if (p.gnome) { p.gnome.rotation.z = clamp(st.ax * 0.009, -0.3, 0.3); p.gnome.rotation.x = clamp(-st.lat * 0.012, -0.4, 0.4); }
            if (p.cushion) p.cushion.position.y = p.cushionY + st.dip * 0.5;
            if (brakeMat) {
                const on = (v.brake > 0.1 || v.handbrake || v.vx < -0.5) ? 1 : 0;
                st.brake += (on - st.brake) * (1 - Math.exp(-(on ? 30 : 9) * Math.max(dt, 0.016)));
                brakeMat.emissiveIntensity = 0.5 + 2.4 * st.brake;
            }
            if (ctx && ctx.night !== rig.night) rig.setLights(ctx.night);
        };

        rig.dispose = function () {
            group.removeFromParent();
            for (const m of owned) m.dispose();
            owned.length = 0;
        };
        return rig;
    };

    /* Template meshes as instanced meshes. `pulse` lists material names that get a private clone per
       call (their emissive is animated). Returns { group, mats, box (template space), set, flush, dispose }. */
    function instancedFrom(T, template, count, pulse) {
        template.updateMatrixWorld(true);
        const inv = new T.Matrix4().copy(template.matrixWorld).invert();
        const parts = [], mats = [];
        template.traverse(o => {
            if (!o.isMesh) return;
            const local = new T.Matrix4().multiplyMatrices(inv, o.matrixWorld);
            let material = o.material;
            if (pulse) {
                const swap = m => { if (pulse.indexOf(m.name) < 0) return m; const c = m.clone(); mats.push(c); return c; };
                material = Array.isArray(material) ? material.map(swap) : swap(material);
            }
            const im = new T.InstancedMesh(o.geometry, material, Math.max(1, count));
            im.count = count;
            im.frustumCulled = false;
            parts.push({ im: im, local: local });
        });
        const group = new T.Group();
        const tmp = new T.Matrix4();
        for (const p of parts) group.add(p.im);
        const box = new T.Box3().setFromObject(template).applyMatrix4(inv);
        return {
            group: group, mats: mats, box: box, parts: parts,
            set(i, matrix) { for (const p of parts) p.im.setMatrixAt(i, tmp.multiplyMatrices(matrix, p.local)); },
            flush() { for (const p of parts) p.im.instanceMatrix.needsUpdate = true; },
            shadows(on) { for (const p of parts) p.im.castShadow = !!on; },
            dispose() { group.removeFromParent(); for (const p of parts) p.im.dispose(); for (const m of mats) m.dispose(); }
        };
    }

    /* Soft black radial gradient for blob shadows (the caller owns and disposes it). */
    AD.makeBlobTexture = function (T) {
        const c = document.createElement('canvas');
        c.width = c.height = 128;
        const g = c.getContext('2d');
        const grad = g.createRadialGradient(64, 64, 4, 64, 64, 62);
        grad.addColorStop(0, 'rgba(0,0,0,1)');
        grad.addColorStop(0.55, 'rgba(0,0,0,0.55)');
        grad.addColorStop(1, 'rgba(0,0,0,0)');
        g.fillStyle = grad;
        g.fillRect(0, 0, 128, 128);
        const tex = new T.CanvasTexture(c);
        tex.colorSpace = T.SRGBColorSpace;
        return tex;
    };

    const TAU = Math.PI * 2;
    const ease = k => k * k * (3 - 2 * k);
    const backOut = k => 1 + 2.7 * Math.pow(k - 1, 3) + 1.7 * Math.pow(k - 1, 2);
    const DESTRUCT_PIECE = { cone: 'cone', hay: 'hay_bale', barrel: 'barrel', tyres: 'tyre_stack' };
    const DESTRUCT_HALF = { cone: 0.41, hay: 0.95, barrel: 0.68, tyres: 0.75 };
    const SHIELD_VERT = 'varying vec3 vN; varying vec3 vV; varying float vY;\nvoid main() { vec4 mv = modelViewMatrix * vec4(position, 1.0); vN = normalize(normalMatrix * normal); vV = normalize(-mv.xyz); vY = position.y; gl_Position = projectionMatrix * mv; }';
    const SHIELD_FRAG = [
        'uniform vec3 uColor; uniform float uA; uniform float uT; varying vec3 vN; varying vec3 vV; varying float vY;',
        'void main() {',
        '  float f = 1.0 - abs(dot(normalize(vN), normalize(vV)));',
        '  float rim = pow(f, 2.0);',
        '  float band = 0.5 + 0.5 * sin(vY * 11.0 - uT * 4.0);',
        '  gl_FragColor = vec4(uColor * (0.2 + 1.5 * rim + 0.35 * band), (0.1 + 0.75 * rim + 0.08 * band) * uA);',
        '#include <colorspace_fragment>',
        '}'
    ].join('\n');

    AD.createActors = function (T, scene, q) {
        const root = new T.Group();
        root.name = 'actors';
        scene.add(root);
        const blobTex = AD.makeBlobTexture(T);
        const blobGeo = new T.PlaneGeometry(1, 1).rotateX(-Math.PI / 2);
        const blobMat = new T.MeshBasicMaterial({
            map: blobTex, color: 0x000000, transparent: true, depthWrite: false, opacity: 1,
            polygonOffset: true, polygonOffsetFactor: -2, polygonOffsetUnits: -2, fog: false
        });
        let gl = q || AD.QUALITY.high.gl;
        let slots = [], ghostSlot = null, race = null, night = false, carRim = null, checkAt = 0, time = 0, standIns = -1;
        let reduced = !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
        const viewer = { x: 0, y: 0 };
        const tmp = { m: new T.Matrix4(), m2: new T.Matrix4(), q: new T.Quaternion(), p: new T.Vector3(), s: new T.Vector3(), e: new T.Euler(), a: new T.Vector3(), v: new T.Vector3(), c: new T.Color() };

        /* ---- shared resources: `shared` lives as long as the actors, `per` is rebuilt with the pools ---- */
        const shared = [], per = [];
        const S = x => { shared.push(x); return x; }, P = x => { per.push(x); return x; };
        const canvasTex = (size, draw) => {
            const c = document.createElement('canvas'), g = c.getContext('2d');
            c.width = c.height = size; draw(g, size);
            const t = S(new T.CanvasTexture(c)); t.colorSpace = T.SRGBColorSpace; return t;
        };
        const glowTex = canvasTex(64, (g, n) => {
            const grad = g.createRadialGradient(n / 2, n / 2, 0, n / 2, n / 2, n / 2 - 1);
            grad.addColorStop(0, 'rgba(255,255,255,1)'); grad.addColorStop(0.45, 'rgba(255,255,255,0.45)'); grad.addColorStop(1, 'rgba(255,255,255,0)');
            g.fillStyle = grad; g.fillRect(0, 0, n, n);
        });
        const swirlTex = canvasTex(128, g => {
            for (let i = 0; i < 780; i++) {
                const u = (i % 260) / 260, a = Math.floor(i / 260) * TAU / 3 + u * 5.2, rr = 10 + u * 52;
                g.fillStyle = 'rgba(255,' + Math.round(210 - 150 * u) + ',' + Math.round(120 + 100 * u) + ',' + (0.85 * (1 - u * 0.8)) + ')';
                g.beginPath(); g.arc(64 + Math.cos(a) * rr, 64 + Math.sin(a) * rr, 2.2 + u * 3.4, 0, TAU); g.fill();
            }
        });
        const flat = g => S(g.rotateX(-Math.PI / 2));
        const discGeo = flat(new T.CircleGeometry(1, 28)), quadGeo = flat(new T.PlaneGeometry(1, 1)), ringGeo = flat(new T.RingGeometry(0.88, 1, 48));
        const sphereGeo = S(new T.SphereGeometry(1, 18, 12)), iceGeo = S(new T.IcosahedronGeometry(1, 0));
        const crystalGeo = S(new T.ConeGeometry(0.3, 1, 5).translate(0, 0.5, 0)), coneGeo = S(new T.ConeGeometry(1, 1, 24, 1, true).translate(0, -0.5, 0));
        const oilGeo = S((() => {
            const g = new T.CircleGeometry(1, 36).rotateX(-Math.PI / 2), p = g.attributes.position;
            for (let i = 1; i < p.count; i++) {
                const a = Math.atan2(p.getZ(i), p.getX(i)), k = 1 + 0.13 * Math.sin(3 * a + 0.7) + 0.07 * Math.sin(5 * a + 2.1);
                p.setX(i, p.getX(i) * k); p.setZ(i, p.getZ(i) * k);
            }
            return g;
        })());
        const FLAGS = { transparent: true, depthWrite: false, fog: false }, ADD = { blending: T.AdditiveBlending };
        const basic = o => P(new T.MeshBasicMaterial(Object.assign({}, FLAGS, o)));
        const glowMat = o => basic(Object.assign({ map: glowTex }, ADD, o));
        const sprite = o => new T.Sprite(P(new T.SpriteMaterial(Object.assign({}, FLAGS, ADD, o))));
        const road = o => Object.assign({ polygonOffset: true, polygonOffsetFactor: -3, polygonOffsetUnits: -3, transparent: true, depthWrite: false }, o);

        const stage = new T.Group();
        stage.name = 'actors-stage';
        root.add(stage);
        const grp = (...kids) => { const g = new T.Group(); g.add(...kids); stage.add(g); return g; };

        /* ---- pools ------------------------------------------------------------------------------ */
        const makers = {}, pools = {};
        function get(name) {
            if (pools[name]) return pools[name];
            const items = [];
            let n = 0;
            return (pools[name] = {
                items: items,
                begin() { n = 0; },
                next() {
                    const o = items[n] || (items[n] = makers[name]());
                    o.group.visible = true;
                    n++;
                    return o;
                },
                end() { for (let i = n; i < items.length; i++) items[i].group.visible = false; },
                used() { return n; }
            });
        }
        function clearPools() {
            for (const name of Object.keys(pools)) for (const o of pools[name].items) o.group.removeFromParent();
            for (const m of per) m.dispose();
            per.length = 0;
            for (const k of Object.keys(pools)) delete pools[k];
        }
        function shadowFlags(obj) { obj.traverse(o => { if (o.isMesh) o.castShadow = gl.casters === 2; }); }
        function cloneNamed(obj, names) {
            const found = {};
            obj.traverse(o => {
                if (!o.isMesh || !o.material) return;
                const arr = Array.isArray(o.material), list = arr ? o.material.slice() : [o.material];
                list.forEach((m, i) => { if (m && names.indexOf(m.name) >= 0) list[i] = found[m.name] || (found[m.name] = P(m.clone())); });
                o.material = arr ? list : list[0];
            });
            return found;
        }
        /* A cloned template: g (placed, yawed) > spin (animated) > inner (re-centred when `centre`). */
        function itemMaker(name, opts) {
            opts = opts || {};
            return () => {
                const inner = AD.assets.item(name).clone(true), spin = new T.Group(), mats = opts.blink ? cloneNamed(inner, opts.blink) : {};
                spin.add(inner);
                const g = grp(spin);
                if (opts.centre) {
                    inner.updateMatrixWorld(true);
                    inner.position.copy(new T.Box3().setFromObject(inner).getCenter(new T.Vector3()).negate());
                }
                shadowFlags(inner);
                return { group: g, spin: spin, mats: mats };
            };
        }
        const centred = { centre: true };
        makers.rocket = itemMaker('rocket', centred);
        makers.bratwurst = itemMaker('bratwurst', centred);
        makers.duck = itemMaker('rubber_duck');
        makers.grenade = itemMaker('frost_grenade', centred);
        makers.discoShot = itemMaker('disco_ball', centred);
        makers.harpoon = itemMaker('harpoon', centred);
        makers.glove = itemMaker('boxing_glove', centred);
        makers.mine = itemMaker('gnome_mine', { blink: ['glow_red'] });
        makers.holeShot = () => {
            const core = new T.Mesh(sphereGeo, basic({ color: 0x07030f, transparent: false })), halo = sprite({ map: glowTex, color: 0xb46bff });
            core.scale.setScalar(0.8); halo.scale.setScalar(4.5);
            return { group: grp(core, halo) };
        };
        /* A slick is 14 m wide and fills the lower screen behind the car, so it must read as glossy oil on the road, not as a
           black hole: a translucent dark pool with a soft edge, plus an additive rainbow film on the rim. */
        const oilPool = canvasTex(128, (g, n) => {
            const gr = g.createRadialGradient(n / 2, n / 2, 0, n / 2, n / 2, n / 2);
            gr.addColorStop(0, 'rgba(14,12,24,0.78)'); gr.addColorStop(0.62, 'rgba(18,14,30,0.66)'); gr.addColorStop(0.9, 'rgba(24,18,40,0.3)'); gr.addColorStop(1, 'rgba(24,18,40,0)');
            g.fillStyle = gr; g.fillRect(0, 0, n, n);
        });
        const oilFilm = canvasTex(128, (g, n) => {
            const cg = g.createConicGradient(0.6, n / 2, n / 2);
            ['#ff4fa0', '#ffb347', '#7dff8a', '#4fc3ff', '#9a6bff', '#ff4fa0'].forEach((c, i, a) => cg.addColorStop(i / (a.length - 1), c));
            g.fillStyle = cg; g.fillRect(0, 0, n, n);
            g.globalCompositeOperation = 'destination-in';
            const m = g.createRadialGradient(n / 2, n / 2, 0, n / 2, n / 2, n / 2);
            m.addColorStop(0, 'rgba(0,0,0,0)'); m.addColorStop(0.42, 'rgba(0,0,0,0)'); m.addColorStop(0.7, 'rgba(0,0,0,0.85)'); m.addColorStop(0.92, 'rgba(0,0,0,0.3)'); m.addColorStop(1, 'rgba(0,0,0,0)');
            g.fillStyle = m; g.fillRect(0, 0, n, n);
        });
        makers.oil = () => {
            const body = new T.Mesh(oilGeo, basic(road({ map: oilPool, color: 0xffffff, opacity: 0.9 })));
            const sheen = new T.Mesh(oilGeo, basic(road(Object.assign({ map: oilFilm, color: 0xffffff, opacity: 0.3 }, ADD))));
            sheen.position.y = 0.01;
            return { group: grp(body, sheen), body: body, sheen: sheen };
        };
        makers.ice = () => {
            const disc = new T.Mesh(discGeo, P(new T.MeshStandardMaterial(road({ color: 0x8fd8f5, roughness: 0.15, envMapIntensity: 0.5, opacity: 0.78, emissive: 0x2a90d0, emissiveIntensity: 0.35 }))));
            const cm = P(new T.MeshStandardMaterial({ color: 0xe6fbff, roughness: 0.1, transparent: true, opacity: 0.85, emissive: 0x6cc8f0, emissiveIntensity: 0.4, flatShading: true }));
            const crystals = [];
            for (let i = 0; i < 9; i++) {
                const c = new T.Mesh(crystalGeo, cm);
                c.rotation.set(Math.sin(i * 3.1) * 0.35, 0, Math.cos(i * 1.7) * 0.35);
                c.userData.h = 0.9 + 0.9 * ((i * 53 % 7) / 7);
                crystals.push(c);
            }
            return { group: grp(disc, ...crystals), disc: disc, crystals: crystals };
        };
        makers.hole = () => {
            const core = new T.Mesh(sphereGeo, basic({ color: 0x030108, transparent: false })), halo = sprite({ map: glowTex, color: 0x7b3cff, opacity: 0.7 });
            const a = sprite({ map: swirlTex, opacity: 0.95 }), b = sprite({ map: swirlTex, color: 0xa070ff, opacity: 0.8 });
            a.scale.setScalar(15); b.scale.setScalar(24); halo.scale.setScalar(36); core.scale.setScalar(2.6);
            for (const o of [halo, b, a, core]) o.position.y = 2.6;
            return { group: grp(halo, b, a, core), a: a.material, b: b.material, core: core };
        };
        makers.disco = () => {
            const ball = itemMaker('disco_ball', centred)(), g = grp(ball.group), spots = [];
            const beam = new T.Mesh(coneGeo, basic(Object.assign({ color: 0xffffff, opacity: 0.07, side: T.DoubleSide }, ADD)));
            for (let i = 0; i < 10; i++) { const s = new T.Mesh(quadGeo, glowMat({ opacity: 0.8 })); s.position.y = 0.09; spots.push(s); }
            g.add(beam, ...spots);
            return { group: g, ball: ball, beam: beam, spots: spots };
        };
        makers.shadow = () => {
            const ring = new T.Mesh(ringGeo, basic(road({ color: 0xff3030, opacity: 0.9 }))), disc = new T.Mesh(discGeo, basic(road({ color: 0x000000, opacity: 0.4 })));
            ring.position.y = 0.01;
            return { group: grp(disc, ring), ring: ring, disc: disc };
        };
        function animalMaker(kind, seed) {
            return () => {
                const man = AD.assets.manifest(), meta = Object.assign({ height: 1.6, stride: 0.5, bob: 0.05 }, (man && man.animals && man.animals[kind]) || {});
                const model = AD.assets.animal(kind).clone(true), tumble = new T.Group(), parts = {}, base = {};
                tumble.position.y = meta.height / 2; model.position.y = -meta.height / 2;
                tumble.add(model);
                for (const n of ['body', 'head', 'tail', 'leg_fl', 'leg_fr', 'leg_rl', 'leg_rr']) {
                    parts[n] = model.getObjectByName(n) || null;
                    base[n] = parts[n] ? parts[n].position.y : 0;
                }
                shadowFlags(model);
                return { group: grp(tumble), tumble: tumble, model: model, parts: parts, base: base, meta: meta, kind: kind, phase: seed * 1.7, seed: seed };
            };
        }
        makers.cowA = animalMaker('cow', 3);
        makers.cowB = animalMaker('cow', 9);

        /* Leg swing, head bob and tail wag; modes 'idle' 'walk' 'trot' 'fly'. */
        function animate(a, mode, dt) {
            const P = a.parts, m = a.meta, kind = a.kind, sd = a.seed, fly = mode === 'fly', moving = mode === 'walk' || mode === 'trot';
            a.phase += dt * (mode === 'trot' ? 14 : mode === 'walk' ? 8 : 0);
            const ph = a.phase, sw = moving ? (m.stride || 0.5) * (mode === 'trot' ? 2.1 : 1.5) : 0, amp = reduced ? 0 : 1;
            const set = (n, ax, v) => { if (P[n]) P[n].rotation[ax] = v; };
            const legs = ['leg_fl', 'leg_rr', 'leg_fr', 'leg_rl'];
            if (fly) {
                legs.forEach((n, i) => { set(n, 'z', Math.sin(time * 26 + i * 1.9) * 1.15); set(n, 'x', (i % 2 ? 1 : -1) * 0.4 * Math.sin(time * 19 + i)); });
            } else if (kind === 'penguin') {
                const flap = moving ? 0.35 + 0.55 * Math.sin(ph) : 0.08 * Math.sin(time * 2.2 + sd), step = moving ? Math.sin(ph) * 0.9 : 0;
                set('leg_fl', 'x', flap); set('leg_fr', 'x', -flap); set('leg_rl', 'z', step); set('leg_rr', 'z', -step);
                a.model.rotation.x = moving ? Math.sin(ph) * 0.2 : 0;
            } else if (kind === 'crab') {
                [['leg_fl', 1, 0], ['leg_rl', 1, 2], ['leg_fr', -1, 1], ['leg_rr', -1, 3]].forEach(e => set(e[0], 'x', moving ? e[1] * Math.sin(ph * 1.3 + e[2]) * 0.6 : 0));
            } else {
                legs.forEach((n, i) => set(n, 'z', Math.sin(ph + (i < 2 ? 0 : Math.PI)) * sw));
            }
            set('head', 'z', (fly ? Math.sin(time * 15) * 0.4 : moving ? Math.sin(ph * 2) * 0.14 : Math.sin(time * 1.4 + sd) * 0.06) * (kind === 'llama' ? 1.5 : 1));
            set('tail', 'y', Math.sin(time * (moving ? 9 : 3) + sd) * 0.5);
            const bob = moving ? Math.sin(ph * 2) * (m.bob || 0.05) * amp : 0;
            if (P.body) P.body.position.y = a.base.body + bob;
            if (P.head) P.head.position.y = a.base.head + bob;
            a.tumble.position.y = m.height / 2 + (moving ? Math.abs(Math.sin(ph)) * (mode === 'trot' ? 0.2 : 0.07) * amp : 0);
        }
        function tumbleAnimal(a, k, squash) {
            a.tumble.rotation.z = -k * TAU;
            a.tumble.rotation.x = Math.sin(k * Math.PI) * 0.5;
            const sq = squash || 0;
            a.tumble.scale.set(1 + 0.18 * sq, 1 - 0.32 * sq, 1 + 0.18 * sq);
        }

        /* ---- per-race instanced features ----------------------------------------------------- */
        let feat = null;
        function buildFeatures() {
            if (feat) { for (const i of feat.insts) i.dispose(); for (const a of feat.animals) a.group.removeFromParent(); }
            feat = null;
            const fs = race && race.features;
            if (!fs) return;
            const insts = [], f = { insts: insts, animals: [], crates: null, ramps: null, pads: null, bars: null, destr: {} };
            const add = (list, name, getter, pulse, kind) => {
                if (!list || !list.length) return null;
                const tpl = getter(name), inst = instancedFrom(T, tpl, list.length, pulse);
                inst.tpl = tpl; inst.list = list; inst.shadows(gl.casters === 2 && kind !== 'flat');
                stage.add(inst.group); insts.push(inst);
                return inst;
            };
            const itm = n => AD.assets.item(n), trk = n => AD.assets.trackPiece(n);
            f.crates = add(fs.crates, 'crate', itm, null, 'solid');
            f.ramps = add(fs.ramps, 'ramp', trk, null, 'solid');
            f.pads = add(fs.pads, 'boost_pad', trk, ['glow_cyan', 'glow_lime'], 'flat');
            const bars = [];
            for (const c of fs.crossings || []) bars.push(c);
            f.bars = add(bars, 'rumble_bars', trk, null, 'flat');
            const byKind = {};
            for (const d of fs.destructibles || []) (byKind[d.kind] = byKind[d.kind] || []).push(d);
            for (const kind of Object.keys(byKind)) f.destr[kind] = add(byKind[kind], DESTRUCT_PIECE[kind], trk, null, 'solid');
            const fit = (m, inst, x, y, psi, len, wid, hgt, centred, lift) => {
                const b = inst.box, sx = len / (b.max.x - b.min.x || 1), sz = wid / (b.max.z - b.min.z || 1), sy = hgt ? hgt / (b.max.y - b.min.y || 1) : 1;
                tmp.p.set(x, lift || 0, y); tmp.e.set(0, -psi, 0); tmp.q.setFromEuler(tmp.e); tmp.s.set(sx, sy, sz);
                m.compose(tmp.p, tmp.q, tmp.s);
                tmp.m2.makeTranslation(-(centred ? (b.min.x + b.max.x) / 2 : b.min.x), 0, -(b.min.z + b.max.z) / 2);
                m.multiply(tmp.m2);
            };
            if (f.ramps) { fs.ramps.forEach((r, i) => { fit(tmp.m, f.ramps, r.x, r.y, r.psi, r.length, r.width, r.height, false); f.ramps.set(i, tmp.m); }); f.ramps.flush(); }
            if (f.pads) { fs.pads.forEach((p, i) => { fit(tmp.m, f.pads, p.x, p.y, p.psi, p.length, p.width, 0, true, 0.03); f.pads.set(i, tmp.m); }); f.pads.flush(); }
            if (f.bars) {
                bars.forEach((c, i) => {
                    const p = AD.pointAt(race.track, c.barsS);
                    fit(tmp.m, f.bars, p.x, p.y, Math.atan2(p.ty, p.tx), 8.3, Math.max(8, (p.halfWidth || 7) * 1.9), 0, true, 0.03);
                    f.bars.set(i, tmp.m);
                });
                f.bars.flush();
            }
            (fs.animals || []).forEach((a, i) => { f.animals.push(animalMaker(a.kind, i)()); });
            feat = f;
            standIns = AD.assets.stats().standIns;
        }

        function drawFeatures(r, dt) {
            const f = feat, fs = r.features;
            if (!f || !fs) return;
            const m = tmp.m;
            if (f.crates) {
                for (let i = 0; i < fs.crates.length; i++) {
                    const c = fs.crates[i], since = r.time - c.taken;
                    const grow = clamp((since - (CRATE_RESPAWN - 0.5)) / 0.5, 0, 1), pop = ease(grow);
                    tmp.e.set(0, time * 1.4 + i, 0);
                    tmp.q.setFromEuler(tmp.e);
                    const bob = reduced ? 0 : Math.sin(time * 2.2 + i * 1.7) * 0.18;
                    tmp.p.set(c.x, 0.9 + bob, c.y);
                    const sc = Math.max(pop, 0.0001) * 1.1 * (1 + (reduced ? 0 : 0.07 * Math.sin(time * 6 + i)));
                    tmp.s.set(sc, sc, sc);
                    m.compose(tmp.p, tmp.q, tmp.s);
                    f.crates.set(i, m);
                }
                f.crates.flush();
            }
            if (f.pads) {
                const k = 1.1 + 0.9 * (0.5 + 0.5 * Math.sin(time * 6));
                for (const mm of f.pads.mats) mm.emissiveIntensity = k;
            }
            for (const kind of Object.keys(f.destr)) {
                const inst = f.destr[kind], half = DESTRUCT_HALF[kind];
                if (!inst) continue;
                inst.list.forEach((d, i) => {
                    /* Tumble about the body centre; a landed piece lies on its side. */
                    const fly = d.state === 'flying', dl = Math.hypot(d.x - d.hx, d.y - d.hy) || 1;
                    tmp.a.set(fly ? (d.y - d.hy) / dl : 0, 0, fly ? -(d.x - d.hx) / dl : 1);
                    tmp.q.setFromAxisAngle(tmp.a, !fly ? 0 : d.z > 0.02 ? d.spin * (r.time - d.t) : (d.id % 2 ? 1 : -1) * 1.5);
                    tmp.v.set(0, half, 0).applyQuaternion(tmp.q);
                    tmp.p.set(d.x - tmp.v.x, d.z + half - tmp.v.y, d.y - tmp.v.z);
                    tmp.s.set(1, 1, 1);
                    m.compose(tmp.p, tmp.q, tmp.s);
                    inst.set(i, m);
                });
                inst.flush();
            }
            /* animals */
            const list = fs.animals || [], near = list.map((a, i) => ({ i: i, d: Math.hypot(a.x - viewer.x, a.y - viewer.y) })).sort((p, q) => p.d - q.d);
            const cap = gl.casters === 2 ? 99 : 3;
            for (let i = 0; i < list.length; i++) {
                const a = list[i], rig = f.animals[i];
                if (!rig) continue;
                const far = near.findIndex(e => e.i === i) >= cap || near.find(e => e.i === i).d > (gl.casters === 2 ? 420 : 190);
                rig.group.visible = !far;
                if (far) continue;
                rig.group.position.set(a.x, a.z || 0, a.y);
                rig.group.rotation.y = -a.psi;
                const mode = a.state === 'flying' ? 'fly' : a.state === 'trot' ? 'trot' : a.state === 'walk' ? 'walk' : 'idle';
                animate(rig, mode, dt);
                if (mode === 'fly') tumbleAnimal(rig, clamp((r.time - a.t) / 0.78, 0, 1), 0);
                else tumbleAnimal(rig, 0, mode === 'trot' ? Math.max(0, 1 - (r.time - a.t) / 0.35) : 0);
            }
        }

        /* ---- hazards, projectiles, tethers ------------------------------------------------------ */
        const POOLS = ['oil', 'ice', 'mine', 'hole', 'disco', 'shadow', 'rocket', 'bratwurst', 'duck', 'grenade', 'discoShot',
            'harpoon', 'glove', 'holeShot', 'cowA', 'cowB'];
        const place = (o, x, y, z, psi) => { o.group.position.set(x, y, z); o.group.rotation.y = -(psi || 0); };
        const hsl = (mat, h, s, l) => { mat.color.setHSL(((h % 1) + 1) % 1, s, l); };

        function drawHazards(r) {
            for (const h of r.hazards) {
                if (Math.abs(h.x - viewer.x) > 520 || Math.abs(h.y - viewer.y) > 520) continue;
                const grow = ease(clamp((h.age || 0) / 0.3, 0, 1)), fade = clamp(h.ttl / 0.5, 0, 1);
                if (h.type === 'oil') {
                    const o = get('oil').next(), s = h.r * grow;
                    place(o, h.x, 0.05, h.y, h.x * 0.37);
                    o.group.scale.set(s, 1, s * 0.88);
                    o.body.material.opacity = 0.9 * fade; o.sheen.material.opacity = 0.3 * fade;
                } else if (h.type === 'ice') {
                    const o = get('ice').next(), s = h.r * grow, k = grow * fade + 0.001;
                    place(o, h.x, 0.05, h.y, 0);
                    o.disc.scale.set(s, 1, s); o.disc.material.opacity = 0.8 * fade;
                    o.crystals.forEach((c, i) => {
                        const a = i * 2.4 + 0.5, d = (0.18 + 0.7 * ((i * 37 % 9) / 9)) * s * 0.8;
                        c.position.set(Math.cos(a) * d, 0, Math.sin(a) * d); c.scale.set(0.9 * k, c.userData.h * k, 0.9 * k);
                    });
                } else if (h.type === 'gnome_mine') {
                    const o = get('mine').next(), armed = !(h.arm > 0), drop = clamp((h.age || 0) / 0.3, 0, 1);
                    place(o, h.x, (1 - drop * drop) * 3.2, h.y, h.x * 0.9);
                    o.group.scale.setScalar(1.8 * (armed || reduced ? 1 : 1 + 0.06 * Math.sin(time * 8)) * fade);
                    o.spin.rotation.z = reduced ? 0 : Math.sin(time * 4 + h.x) * 0.07;
                    o.spin.scale.y = 1 + (armed && !reduced ? 0.05 * Math.sin(time * 10) : 0);
                    if (o.mats.glow_red) o.mats.glow_red.emissiveIntensity = armed ? (Math.sin(time * 9 + h.y) > 0 ? 3.4 : 0.6) : 0.3 + 0.5 * Math.pow(Math.sin(time * 5), 2);
                } else if (h.type === 'black_hole') {
                    const o = get('hole').next(), g = Math.max(0.01, ease(clamp((h.age || 0) / 0.4, 0, 1)) * fade);
                    place(o, h.x, 0, h.y, 0);
                    o.group.scale.setScalar(g);
                    o.a.rotation = -time * 2.6; o.b.rotation = time * 1.5;
                    o.core.scale.setScalar(2.6 * (1 + (reduced ? 0 : 0.05 * Math.sin(time * 9))));
                } else if (h.type === 'disco') {
                    const o = get('disco').next(), g = ease(clamp((h.age || 0) / 0.3, 0, 1)) * fade, hz = h.z || 6;
                    place(o, h.x, 0, h.y, 0);
                    o.ball.group.position.y = hz;
                    o.ball.group.scale.setScalar(2.6 * Math.max(g, 0.01));
                    o.ball.spin.rotation.y = time * 2.2;
                    o.beam.position.y = hz; o.beam.scale.set(h.r * 0.85, hz, h.r * 0.85);
                    hsl(o.beam.material, time * 0.25, 1, 0.6); o.beam.material.opacity = 0.045 * g;
                    o.spots.forEach((s, i) => {
                        const ang = time * 0.7 * (i % 2 ? -1 : 1) + i * TAU / 10, rad = (5 + (i % 5) * 3.8 + 1.5 * Math.sin(time * 1.9 + i)) * g;
                        s.position.set(Math.cos(ang) * rad, 0.09, Math.sin(ang) * rad);
                        const sz = (4.6 + 1.8 * Math.sin(time * 3 + i)) * g + 0.01;
                        s.scale.set(sz, 1, sz);
                        hsl(s.material, time * 0.3 + i / 10, 1, 0.55);
                        s.material.opacity = g;
                    });
                } else if (h.type === 'cow_shadow') {
                    const o = get('shadow').next(), k = 1 - clamp((h.land || 0) / 1.4, 0, 1), disc = h.r * (0.35 + 0.65 * k), ring = h.r * (1.5 - 0.5 * k);
                    place(o, h.x, 0.06, h.y, 0);
                    o.disc.scale.set(disc, 1, disc); o.disc.material.opacity = 0.12 + 0.5 * k;
                    o.ring.scale.set(ring, 1, ring); o.ring.material.opacity = Math.sin(time * (8 + 14 * k)) > -0.2 ? 0.95 : 0.35;
                }
            }
        }

        function gloveAt(o, v, ext, squash) {
            const c = Math.cos(v.psi), s = Math.sin(v.psi), d = 2.4 + ext * 4.4;
            place(o, v.x + c * d, 1.0 + v.z, v.y + s * d, v.psi);
            o.group.scale.set(1.3 * (1 - 0.3 * squash), 1.3 * (1 + 0.25 * squash), 1.3 * (1 + 0.25 * squash));
        }

        function drawProjectiles(r, dt) {
            for (const p of r.projectiles) {
                if (Math.abs(p.x - viewer.x) > 520 || Math.abs(p.y - viewer.y) > 520) continue;
                switch (p.type) {
                    case 'rocket': { const o = get('rocket').next(); place(o, p.x, 0.75 + p.z, p.y, p.psi); o.group.scale.setScalar(1.7); o.spin.rotation.x = time * 9; break; }
                    case 'bullet': { const o = get('bratwurst').next(); place(o, p.x, 0.8 + p.z, p.y, p.psi); o.group.scale.setScalar(1.9); o.spin.rotation.set(time * 26 + p.x, time * 7, time * 13); break; }
                    case 'duck': {      /* waddle: roll and squash with the hop phase */
                        const o = get('duck').next(), w = p.wob || 0;
                        place(o, p.x, p.z, p.y, p.psi);
                        o.group.scale.setScalar(1.45);
                        o.spin.rotation.set(Math.sin(w * 0.5) * 0.35, 0, Math.sin(w) * 0.22); o.spin.scale.y = 1 + 0.14 * Math.cos(w);
                        break;
                    }
                    case 'grenade': { const o = get('grenade').next(); place(o, p.x, p.z + 0.4, p.y, p.psi); o.group.scale.setScalar(1.8); o.spin.rotation.set(time * 7, 0, time * 11); break; }
                    case 'hole_shot': { const o = get('holeShot').next(); place(o, p.x, p.z + 0.6, p.y, p.psi); const k = 1 + 0.1 * Math.sin(time * 18); o.group.scale.setScalar(1.5 * k); break; }
                    case 'disco_shot': { const o = get('discoShot').next(); place(o, p.x, p.z + 0.6, p.y, p.psi); o.group.scale.setScalar(1.1); o.spin.rotation.y = time * 9; break; }
                    case 'harpoon': { const o = get('harpoon').next(); place(o, p.x, 0.95 + p.z, p.y, p.psi); o.group.scale.setScalar(1.4); break; }
                    case 'glove': if (r.cars[p.owner]) gloveAt(get('glove').next(), r.cars[p.owner].v, backOut(clamp(1 - p.ttl / 0.12, 0, 1)), 0); break;
                    case 'cow': {   /* one somersault over the flight, legs flailing */
                        const o = get('cowA').next();
                        place(o, p.x, Math.max(0, p.z - 0.6), p.y, p.psi);
                        o.group.scale.setScalar(1.6);
                        animate(o, 'fly', dt); tumbleAnimal(o, clamp((p.t || 0) / (p.flight || 1.4), 0, 1), 0);
                        break;
                    }
                }
            }
            for (const b of r.beams) {      /* the rope is an fx beam; the harpoon head sits on the towed car */
                const A = b.type === 'tether' && r.cars[b.a], B = A && r.cars[b.b];
                if (!B) continue;
                const head = get('harpoon').next();
                place(head, B.v.x, 0.95, B.v.y, Math.atan2(B.v.y - A.v.y, B.v.x - A.v.x));
                head.group.scale.setScalar(1.1);
            }
        }

        /* ---- one-shot presentation: cow trot-off, glove retract ---------------------------- */
        const trots = [], gloves = [];
        function drawTrots(r, dt) {
            for (let i = trots.length - 1; i >= 0; i--) {
                const T0 = trots[i], age = time - T0.t0, go = age - 0.6, dist = clamp(go * 7, 0, T0.len);
                if (age > 0.6 + T0.len / 7 + 2.4) { trots.splice(i, 1); continue; }
                const o = get('cowB').next(), moving = go > 0 && dist < T0.len;
                const yaw = go > 0 ? T0.psi + AD.wrapAngle(T0.face - T0.psi) * ease(clamp(go / 0.3, 0, 1)) : T0.psi;
                place(o, T0.x + T0.dx * dist, 0, T0.y + T0.dy * dist, yaw);
                o.group.scale.setScalar(Math.max(0.01, 1 - ease(clamp((age - (T0.len / 7 + 2.5)) / 0.5, 0, 1))));
                animate(o, moving ? 'trot' : 'idle', dt);
                tumbleAnimal(o, 0, Math.max(0, 1 - age / 0.4));
                if (age < 0.6 && !reduced) o.tumble.rotation.x = Math.sin(time * 17) * 0.12;
            }
            for (let i = gloves.length - 1; i >= 0; i--) {
                const g = gloves[i], age = time - g.t0, car = r.cars[g.car];
                if (age > 0.3 || !car) { gloves.splice(i, 1); continue; }
                gloveAt(get('glove').next(), car.v, age < 0.08 ? 1 : 1 - ease((age - 0.08) / 0.22), g.hit && age < 0.14 ? 1 : 0);
            }
        }

        /* ---- cars: status looks and accessories ------------------------------------------------- */
        const fuseGeo = S(new T.CylinderGeometry(0.08, 0.1, 0.55, 6));
        function makeCarFx(s) {
            const rig = s.rig, k = 1 / rig.scale, sz = rig.size, rb = clamp(sz.w * 0.3, 0.55, 0.8) * k, mats = [];
            const M = m => { mats.push(m); return m; };
            const fx = { mats: mats, rl: sz.l * 0.5 * k, rw: sz.w * 0.5 * k, rh: sz.h * k, rb: rb };
            fx.shieldMat = M(new T.ShaderMaterial({
                uniforms: { uColor: { value: new T.Color('#6fe3ff') }, uA: { value: 1 }, uT: { value: 0 } },
                vertexShader: SHIELD_VERT, fragmentShader: SHIELD_FRAG, transparent: true, depthWrite: false, blending: T.AdditiveBlending, side: T.DoubleSide, fog: false
            }));
            fx.iceMat = M(new T.MeshStandardMaterial({ color: 0xb6ecff, roughness: 0.08, transparent: true, opacity: 0.6, emissive: 0x4aa8e0, emissiveIntensity: 0.35, flatShading: true, depthWrite: false }));
            fx.bombMat = M(new T.MeshStandardMaterial({ color: 0x1b1b22, roughness: 0.4, metalness: 0.3, emissive: 0xff2a10, emissiveIntensity: 0, envMapIntensity: 0.6 }));
            fx.glowMat = M(new T.MeshBasicMaterial({ map: glowTex, color: 0xff8a1a, transparent: true, blending: T.AdditiveBlending, depthWrite: false, fog: false, opacity: 0 }));
            fx.shield = new T.Mesh(sphereGeo, fx.shieldMat);
            fx.ice = new T.Mesh(iceGeo, fx.iceMat);
            fx.glow = new T.Mesh(quadGeo, fx.glowMat);
            fx.glow.position.y = -rig.cy + 0.08; fx.glow.scale.set(sz.l * k * 2.0, 1, sz.w * k * 2.8);
            const ball = new T.Mesh(sphereGeo, fx.bombMat), fuse = new T.Mesh(fuseGeo, fx.bombMat);
            fx.spark = new T.Sprite(M(new T.SpriteMaterial({ map: glowTex, color: 0xffa020, blending: T.AdditiveBlending, depthWrite: false, fog: false })));
            ball.scale.setScalar(rb); fuse.position.y = rb; fuse.scale.setScalar(rb * 1.4); fx.spark.position.y = rb * 1.56 + 0.15;
            fx.bomb = new T.Group();
            fx.bomb.add(ball, fuse, fx.spark);
            /* On the car's roof mount (anchor_weapon), or the top of its bounding box. */
            rig.group.updateMatrixWorld(true);
            const anchor = rig.group.getObjectByName('anchor_weapon'), top = new T.Vector3(0, fx.rh - rig.cy, 0);
            if (anchor) { anchor.getWorldPosition(top); rig.fxRoot.worldToLocal(top); }
            fx.bomb.position.set(top.x, top.y + rb * 0.8, top.z);
            fx.shield.position.y = fx.ice.position.y = fx.rh / 2 - rig.cy;
            for (const o of [fx.shield, fx.ice, fx.bomb, fx.glow]) { o.visible = false; rig.fxRoot.add(o); }
            return fx;
        }
        function dropFx(s) {
            if (!s || !s.fx) return;
            for (const o of [s.fx.shield, s.fx.ice, s.fx.bomb, s.fx.glow]) o.removeFromParent();
            for (const m of s.fx.mats) m.dispose();
            s.fx = null;
        }
        function updateCarFx(s, r, dt) {
            const v = s.actor.v, rig = s.rig, st = s.st, idx = s.actor.index | 0;
            const fx = s.fx || (s.fx = makeCarFx(s));
            const sm = (key, on, rate) => { st[key] += ((on ? 1 : 0) - st[key]) * (1 - Math.exp(-rate * dt)); return st[key]; };
            const soot = sm('soot', v.toast > 0, v.toast > 0 ? 12 : 1.6), cloak = sm('cloak', v.cloak > 0, 9), over = sm('over', v.overdrive > 0, 10);
            const shield = sm('shield', v.shield > 0, 14), ice = sm('ice', v.frozen > 0, 16);
            st.flash = Math.max(0, st.flash - dt * 3);
            const pulse = 0.5 + 0.5 * Math.sin(time * 18), heat = clamp(v.heat || 0, 0, 1);
            const cloakAlpha = idx === 0 ? 0.42 : 0.15 + 0.1 * Math.sin(time * 11 + idx * 2);
            rig.look({
                soot: soot,
                alpha: 1 - cloak * (1 - cloakAlpha),
                glow: [heat * 0.55 + over * 0.5 * (0.7 + 0.3 * pulse) + (v.frozen > 0 ? 0.1 : 0),
                    heat * 0.08 + over * 0.22 * (0.7 + 0.3 * pulse) + (v.frozen > 0 ? 0.35 : 0) + cloak * 0.14,
                    (v.frozen > 0 ? 0.5 : 0) + cloak * 0.2 + over * 0.03]
            });
            /* shield bubble */
            fx.shield.visible = shield > 0.02;
            if (fx.shield.visible) {
                const pop = 0.6 + 0.4 * backOut(clamp(shield, 0, 1)), flick = v.shield < 1 && v.shield > 0 ? (Math.sin(time * 36) > 0 ? 1 : 0.25) : 1;
                fx.shield.scale.set(fx.rl * 1.3 * pop, Math.max(fx.rh * 0.8, fx.rw * 1.1) * pop, fx.rw * 1.45 * pop);
                fx.shieldMat.uniforms.uA.value = flick * shield + st.flash * 0.7;
                fx.shieldMat.uniforms.uT.value = time;
            }
            /* ice shell */
            fx.ice.visible = ice > 0.02;
            if (fx.ice.visible) {
                const end = v.frozen > 0 && v.frozen < 0.2 ? 1 + (0.2 - v.frozen) * 2 : 1;
                const k = 0.7 + 0.3 * backOut(clamp(ice, 0, 1));
                fx.ice.scale.set(fx.rl * 1.1 * k * end, Math.max(fx.rh * 0.78, fx.rw) * k * end, fx.rw * 1.18 * k * end);
                fx.iceMat.opacity = 0.6 * (v.frozen > 0 && v.frozen < 0.2 ? v.frozen / 0.2 : 1);
            }
            /* overdrive glow on the road */
            fx.glow.visible = over > 0.02;
            fx.glowMat.opacity = over * (0.6 + 0.25 * pulse);
            /* the hot-potato bomb on the roof, fuse blinking faster as it burns down */
            const bomb = !!v.hasBomb && !v.out;
            fx.bomb.visible = bomb;
            if (bomb) {
                const fuse = r.bomb ? r.bomb.fuse : 5, rate = 2.5 + 16 * clamp(1 - fuse / 8, 0, 1);
                const on = Math.sin(time * rate * Math.PI) > 0;
                fx.bombMat.emissiveIntensity = on ? 0.8 : 0;
                fx.spark.scale.setScalar(fx.rb * (on ? 2.2 : 0.9));
                fx.bomb.rotation.z = reduced ? 0 : Math.sin(time * 9) * 0.12;
            }
        }
        function applyLaunch(s, r) {
            const L = s.launch, g = s.rig.group, age = time - L.t0;
            if (age > 1.7) { s.launch = null; g.scale.setScalar(s.rig.scale); return; }
            const gone = ease(clamp((age - 1.1) / 0.6, 0, 1));
            g.visible = true;
            g.position.set(L.x + L.vx * age, L.y0 + L.vz * age - 9 * age * age, L.z + L.vy * age);
            g.rotation.set(age * L.spin, -L.psi + age * L.spin * 0.6, age * L.spin * 0.8);
            g.scale.setScalar(s.rig.scale * (1 - gone));
        }

        /* ---- slots -------------------------------------------------------------------------------- */
        function makeSlot(actor, isGhost) {
            const carDef = actor.v.car;
            const rig = AD.createCarRig(T, carDef, { livery: actor.livery | 0, ghost: isGhost, gl: gl, night: night, rim: carRim });
            root.add(rig.group);
            let blob = null;
            if (!isGhost) {
                blob = new T.Mesh(blobGeo, blobMat.clone());
                blob.material.map = blobTex;
                blob.renderOrder = 1;
                blob.scale.set(rig.size.l * 1.3, 1, rig.size.w * 1.45);
                root.add(blob);
            }
            return { actor: actor, rig: rig, blob: blob, template: rig.template, fx: null, launch: null,
                st: { soot: 0, cloak: 0, over: 0, shield: 0, ice: 0, flash: 0 } };
        }
        function dropSlot(s) {
            if (!s) return;
            dropFx(s);
            s.rig.dispose();
            if (s.blob) { s.blob.removeFromParent(); s.blob.material.dispose(); }
        }
        function rebuildStage() {
            clearPools();
            buildFeatures();
        }
        function ghostActor() { return race && race.ghost && race.ghost.active !== false ? race.ghost : null; }

        const actors = {
            group: root,
            setRace(next) {
                for (const s of slots) dropSlot(s);
                dropSlot(ghostSlot);
                slots = []; ghostSlot = null; race = next;
                trots.length = 0; gloves.length = 0;
                rebuildStage();
                if (!race) return;
                night = !!(race.track && race.track.biome && race.track.biome.night);
                carRim = AD.carRimFor ? AD.carRimFor(AD.LOOK[race.track.biomeId]) : null;
                for (const car of race.cars) slots.push(makeSlot(car, false));
                if (race.ghost) ghostSlot = makeSlot(race.ghost, true);
            },
            setNight(value) { night = !!value; },
            setReduced(on) { reduced = !!on; },
            setQuality(next) {
                gl = next;
                for (const s of slots.concat(ghostSlot ? [ghostSlot] : [])) s.rig.setGl(gl);
                for (const s of slots) s.blob.material.opacity = gl.shadows ? 0.5 : 0.8;
                if (feat) for (const i of feat.insts) i.shadows(gl.casters === 2);
                for (const name of Object.keys(pools)) for (const o of pools[name].items) shadowFlags(o.group);
            },
            update(r, dt, pose, reducedNow) {
                if (r !== race) actors.setRace(r);
                if (reducedNow !== undefined) reduced = !!reducedNow;
                time += dt;
                /* Late GLBs replace stand-ins without a restart. */
                checkAt -= 1 / 60;
                if (checkAt <= 0) {
                    checkAt = 0.5;
                    for (let i = 0; i < slots.length; i++) {
                        if (AD.assets.car(slots[i].actor.v.car.id) !== slots[i].template) {
                            dropSlot(slots[i]);
                            slots[i] = makeSlot(slots[i].actor, false);
                        }
                    }
                    if (AD.assets.stats().standIns !== standIns) rebuildStage();
                }
                const lead = r.cars[0] && r.cars[0].v;
                if (lead) { viewer.x = lead.x; viewer.y = lead.y; }
                const ctx = { night: night };
                for (const name of POOLS) get(name).begin();
                drawFeatures(r, dt);
                drawHazards(r);
                drawProjectiles(r, dt);
                drawTrots(r, dt);
                for (const name of POOLS) get(name).end();
                for (const s of slots) {
                    const v = s.actor.v;
                    s.rig.update(v, dt, ctx);
                    AD.carCast(s.rig, Math.hypot(v.x - viewer.x, v.y - viewer.y) < 42, gl);
                    updateCarFx(s, r, dt);
                    if (v.steerWobble > 0 && !reduced) s.rig.group.rotation.x += Math.sin(time * 28) * 0.06;
                    if (s.launch) applyLaunch(s, r);
                    const b = s.blob;
                    b.visible = s.rig.group.visible && !s.launch;
                    b.position.set(v.x, 0.07, v.y);
                    b.rotation.y = -v.psi;
                    const fade = clamp(1 - v.z / 7, 0, 1);
                    b.material.opacity = (gl.shadows ? 0.5 : 0.8) * fade;
                    const grow = 1 + v.z * 0.12;
                    b.scale.set(s.rig.size.l * 1.3 * grow, 1, s.rig.size.w * 1.45 * grow);
                }
                const ghost = ghostActor();
                if (ghostSlot) {
                    ghostSlot.rig.group.visible = !!ghost;
                    if (ghost) ghostSlot.rig.update(ghost.v, dt, ctx);
                }
            },
            event(evt) {
                if (!race) return;
                const s = slots[evt.car], kick = (slot, kind, power) => { if (slot) slot.rig.kick(kind, power); };
                switch (evt.type) {
                    case 'landing': kick(s, 'landing', evt.quality === 'crash' ? 1.8 : evt.quality === 'perfect' ? 0.5 : 1); break;
                    case 'wall_hit': kick(s, 'hit', evt.side || (evt.car % 2 ? 1 : -1)); break;
                    case 'car_hit': kick(slots[evt.a], 'hit', 1); kick(slots[evt.b], 'hit', -1); break;
                    case 'hit': kick(s, 'hit', s && s.actor.v.spinKick || 1); break;
                    case 'shield_block': if (s) s.st.flash = 1; break;
                    case 'glove': gloves.push({ car: evt.car, t0: time, hit: !!evt.hit }); break;
                    case 'cow_land': {
                        /* The cow trots off to the nearer road edge. */
                        const sample = AD.sampleTrack(race.track, evt.x, evt.y), p = AD.pointAt(race.track, sample.s), side = sample.offset < 0 ? -1 : 1;
                        const dx = p.nx * side, dy = p.ny * side, len = Math.max(2, (sample.halfWidth || 8) - Math.abs(sample.offset) + 5);
                        trots.push({ x: evt.x, y: evt.y, dx: dx, dy: dy, len: len, psi: evt.psi || 0, face: Math.atan2(dy, dx), t0: time });
                        break;
                    }
                    case 'eliminated': if (s) {
                        /* Cartoon launch: the car is shot into the sky, tumbling, and shrinks away. */
                        const v = s.actor.v, sg = evt.car % 2 ? 1 : -1, fwd = Math.max(0, v.vx) * 0.35;
                        s.launch = { t0: time, x: evt.x, z: evt.y, y0: 0.6, psi: v.psi, vz: 17, spin: sg * 9,
                            vx: Math.cos(v.psi) * fwd - Math.sin(v.psi) * sg * 6, vy: Math.sin(v.psi) * fwd + Math.cos(v.psi) * sg * 6 };
                    } break;
                }
            },
            /* Points in three space for projection checks: centre, nose, tail of car `slot`. */
            carPoints(slot) {
                const s = slots[slot || 0];
                if (!s) return null;
                const v = s.actor.v, half = s.rig.size.l / 2;
                const c = Math.cos(v.psi), n = Math.sin(v.psi), y = v.z + 0.7;
                return { car: [v.x, y, v.y], nose: [v.x + c * half, y, v.y + n * half], tail: [v.x - c * half, y, v.y - n * half] };
            },
            stats() {
                let stand = 0, pooled = 0;
                for (const s of slots) if (AD.assets.isStandIn(s.template)) stand++;
                for (const name of Object.keys(pools)) pooled += pools[name].used();
                return { cars: slots.length, standIns: stand, crates: feat && feat.crates ? feat.crates.list.length : 0, animals: feat ? feat.animals.length : 0,
                    pooled: pooled, hazards: race ? race.hazards.length : 0, projectiles: race ? race.projectiles.length : 0 };
            },
            dispose() {
                for (const s of slots) dropSlot(s);
                dropSlot(ghostSlot);
                slots = []; ghostSlot = null;
                clearPools();
                if (feat) { for (const i of feat.insts) i.dispose(); for (const a of feat.animals) a.group.removeFromParent(); }
                feat = null;
                for (const x of shared) x.dispose();
                blobGeo.dispose(); blobMat.dispose(); blobTex.dispose();
                root.removeFromParent();
            }
        };
        return actors;
    };
})();
