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

    const SHARED_TUNE = {
        glass: m => { m.envMapIntensity = 1.8; m.depthWrite = false; m.roughness = 0.05; },
        chrome: m => { m.metalness = 1; m.roughness = 0.1; m.envMapIntensity = 1.5; },
        tyre: m => { m.envMapIntensity = 0.35; },
        trim: m => { m.envMapIntensity = 0.7; }
    };

    /* A near-black accent (Big Bertha's #2a2a2a) would turn its flames and trim into holes in the
       paint, so dark accents are lifted to a warm orange. fx uses the same colour for flames. */
    AD.accentColour = function (carDef) {
        const hex = String(carDef.accent || '#ffffff');
        const n = parseInt(hex.slice(1), 16);
        const lum = (0.2126 * ((n >> 16) & 255) + 0.7152 * ((n >> 8) & 255) + 0.0722 * (n & 255)) / 255;
        return lum < 0.3 ? '#ffa01e' : hex;
    };

    function paintMaterial(T, hex, gl, vertexColors, ghost) {
        const base = { color: hex, vertexColors: vertexColors, roughness: gl.clearcoat ? 0.28 : 0.3, metalness: gl.clearcoat ? 0.35 : 0.25 };
        const m = gl.clearcoat
            ? new T.MeshPhysicalMaterial(Object.assign(base, { clearcoat: 1, clearcoatRoughness: 0.08 }))
            : new T.MeshStandardMaterial(base);
        m.envMapIntensity = gl.clearcoat ? 1.0 : 1.35;
        if (ghost) { m.transparent = true; m.opacity = 0.35; m.depthWrite = false; }
        return m;
    }

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
                if (SHARED_TUNE[src.name] && !src.userData.adTuned) { SHARED_TUNE[src.name](src); src.userData.adTuned = true; }
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

        function applyPaint(gl) {
            for (const [list, hex] of [[paintMeshes, paintHex], [accentMeshes, accentHex]]) {
                if (!list.length) continue;
                const old = list[0].material;
                const mat = own(paintMaterial(T, hex, gl, !!list[0].geometry.attributes.color, rig.ghost));
                for (const o of list) o.material = mat;
                if (old && owned.indexOf(old) >= 0) { old.dispose(); owned.splice(owned.indexOf(old), 1); }
            }
            rig.gl = gl;
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

    /* Template meshes as instanced meshes, for crates. Returns { group, set(i, matrix), dispose }. */
    function instancedFrom(T, template, count) {
        template.updateMatrixWorld(true);
        const inv = new T.Matrix4().copy(template.matrixWorld).invert();
        const parts = [];
        template.traverse(o => {
            if (!o.isMesh) return;
            const local = new T.Matrix4().multiplyMatrices(inv, o.matrixWorld);
            const im = new T.InstancedMesh(o.geometry, o.material, count);
            im.frustumCulled = false;
            im.castShadow = true;
            parts.push({ im: im, local: local });
        });
        const group = new T.Group();
        const tmp = new T.Matrix4();
        for (const p of parts) group.add(p.im);
        return {
            group: group,
            set(i, matrix) { for (const p of parts) p.im.setMatrixAt(i, tmp.multiplyMatrices(matrix, p.local)); },
            flush() { for (const p of parts) p.im.instanceMatrix.needsUpdate = true; },
            dispose() { group.removeFromParent(); for (const p of parts) p.im.dispose(); }
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
        let slots = [], ghostSlot = null, crates = null, race = null, night = false, checkAt = 0, time = 0;
        const crateTmp = { m: new T.Matrix4(), q: new T.Quaternion(), p: new T.Vector3(), s: new T.Vector3(), e: new T.Euler() };

        function makeSlot(actor, isGhost) {
            const carDef = actor.v.car;
            const rig = AD.createCarRig(T, carDef, { livery: actor.livery | 0, ghost: isGhost, gl: gl, night: night });
            root.add(rig.group);
            let blob = null;
            if (!isGhost) {
                blob = new T.Mesh(blobGeo, blobMat.clone());
                blob.material.map = blobTex;
                blob.renderOrder = 1;
                blob.scale.set(rig.size.l * 1.3, 1, rig.size.w * 1.45);
                root.add(blob);
            }
            return { actor: actor, rig: rig, blob: blob, template: rig.template };
        }
        function dropSlot(s) {
            if (!s) return;
            s.rig.dispose();
            if (s.blob) { s.blob.removeFromParent(); s.blob.material.dispose(); }
        }
        function buildCrates() {
            if (crates) { crates.inst.dispose(); crates = null; }
            const list = race && race.features && race.features.crates;
            if (!list || !list.length) return;
            const tpl = AD.assets.item('crate');
            crates = { inst: instancedFrom(T, tpl, list.length), list: list, tpl: tpl };
            root.add(crates.inst.group);
        }
        function ghostActor() { return race && race.ghost && race.ghost.active !== false ? race.ghost : null; }

        const actors = {
            group: root,
            setRace(next) {
                for (const s of slots) dropSlot(s);
                dropSlot(ghostSlot);
                slots = []; ghostSlot = null; race = next;
                if (!race) return;
                night = !!(race.track && race.track.biome && race.track.biome.night);
                for (const car of race.cars) slots.push(makeSlot(car, false));
                if (race.ghost) ghostSlot = makeSlot(race.ghost, true);
                buildCrates();
            },
            setNight(value) { night = !!value; },
            setQuality(next) {
                gl = next;
                for (const s of slots.concat(ghostSlot ? [ghostSlot] : [])) s.rig.setGl(gl);
                for (const s of slots) s.blob.material.opacity = gl.shadows ? 0.5 : 0.8;
            },
            update(r, dt) {
                if (r !== race) actors.setRace(r);
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
                    if (crates && AD.assets.item('crate') !== crates.tpl) buildCrates();
                }
                const ctx = { night: night };
                for (const s of slots) {
                    const v = s.actor.v;
                    s.rig.update(v, dt, ctx);
                    const b = s.blob;
                    b.visible = s.rig.group.visible;
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
                if (crates) {
                    const m = crateTmp;
                    for (let i = 0; i < crates.list.length; i++) {
                        const c = crates.list[i];
                        const since = r.time - c.taken;
                        /* The crate grows back during the last half second of its respawn time. */
                        const grow = clamp((since - (CRATE_RESPAWN - 0.5)) / 0.5, 0, 1);
                        const pop = grow * grow * (3 - 2 * grow);
                        m.e.set(0, time * 1.4 + i, 0);
                        m.q.setFromEuler(m.e);
                        m.p.set(c.x, 0.9 + Math.sin(time * 2.2 + i * 1.7) * 0.18, c.y);
                        const sc = Math.max(pop, 0.0001) * 1.05;
                        m.s.set(sc, sc, sc);
                        m.m.compose(m.p, m.q, m.s);
                        crates.inst.set(i, m.m);
                    }
                    crates.inst.flush();
                }
            },
            event(evt) {
                if (!race) return;
                if (evt.type === 'landing' && slots[evt.car]) {
                    slots[evt.car].rig.kick('landing', evt.quality === 'crash' ? 1.8 : evt.quality === 'perfect' ? 0.5 : 1);
                } else if (evt.type === 'wall_hit' && slots[evt.car]) {
                    slots[evt.car].rig.kick('hit', evt.side || (evt.car % 2 ? 1 : -1));
                } else if (evt.type === 'car_hit') {
                    for (const i of [evt.a, evt.b]) if (slots[i]) slots[i].rig.kick('hit', i === evt.a ? 1 : -1);
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
                let standIns = 0;
                for (const s of slots) if (AD.assets.isStandIn(s.template)) standIns++;
                return { cars: slots.length, standIns: standIns, crates: crates ? crates.list.length : 0 };
            },
            dispose() {
                for (const s of slots) dropSlot(s);
                dropSlot(ghostSlot);
                slots = []; ghostSlot = null;
                if (crates) crates.inst.dispose();
                crates = null;
                blobGeo.dispose(); blobMat.dispose(); blobTex.dispose();
                root.removeFromParent();
            }
        };
        return actors;
    };
})();
