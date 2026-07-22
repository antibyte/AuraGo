(function () {
    'use strict';

    // Fleet module for the system-world desktop app: five visually distinct
    // sub-districts arranged around the core.
    //   1. Mission ring  - one beacon per mission on a flat XZ ring
    //   2. Cron dial     - tick marks on a wider thin ring plus a rotating hand
    //   3. Co-agent drones - small ships on lissajous orbits with trails
    //   4. Tool belt     - sparse asteroid-like ring of glyph plates
    //   5. Infrastructure field - container cubes and daemon pylons below
    // Every district is diff-driven by id so polling updates never tear down
    // the whole scene. Augments the shared window.SysWorld namespace.

    const NS = window.SysWorld = window.SysWorld || {};

    // Per-district visual caps.
    const CAP_MISSIONS = 60;
    const CAP_CRON = 60;
    const CAP_DRONES = 24;
    const CAP_TOOLS = 18;
    const CAP_CONTAINERS = 80;
    const CAP_DAEMONS = 40;

    const ARC_PTS = 26;           // points per running-mission arc
    const MAX_ARCS = 12;          // simultaneous core-bound arcs
    const RING_PTS = 128;         // guide circle resolution
    const DRONE_LEAVE_SECS = 1.4; // outward-fly despawn duration
    const HAND_SPEED = 0.1;       // cron dial hand rotation (rad/s)

    // FNV-1a style string hash -> uint32, used for deterministic seeding.
    function hashString(str) {
        let h = 2166136261 >>> 0;
        for (let i = 0; i < str.length; i++) {
            h ^= str.charCodeAt(i);
            h = Math.imul(h, 16777619);
        }
        return h >>> 0;
    }

    // mulberry32 PRNG: tiny seedable generator for stable per-entity params.
    function mulberry32(seed) {
        let a = seed >>> 0;
        return function () {
            a |= 0; a = (a + 0x6D2B79F5) | 0;
            let t = Math.imul(a ^ (a >>> 15), 1 | a);
            t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
            return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
        };
    }

    NS.createFleet = function (inst) {
        const THREE = inst.THREE || window.THREE;
        if (!THREE) { return null; }
        const fx = inst.fx || {};
        const P = NS.PALETTE || {};
        const LAY = NS.LAYOUT || {};
        const MISSION_R = LAY.missionRingRadius || 84;
        const CRON_R = MISSION_R + 14;
        const BELT_R = LAY.beltRadius || 130;
        const INFRA_Y = LAY.infraY != null ? LAY.infraY : -34;
        const quality = Math.max(0.4, inst.qualityScale || 1);

        // Resolved palette with defensive fallbacks.
        const C = {
            ok: P.ok != null ? P.ok : 0x66bb6a,
            warn: P.warn != null ? P.warn : 0xffca28,
            error: P.error != null ? P.error : 0xef5350,
            dim: P.dim != null ? P.dim : 0x455a64,
            cron: P.cron != null ? P.cron : 0xaed581,
            agent: P.agent != null ? P.agent : 0x80deea,
            tool: P.tool != null ? P.tool : 0xfff176,
            mission: P.mission != null ? P.mission : 0xffd54f,
            storage: P.storage != null ? P.storage : 0x8ea3b0,
            infra: P.infrastructure != null ? P.infrastructure : 0xffb74d
        };

        // One root group for all five districts (sits at the scene origin,
        // so group-local coordinates double as world coordinates).
        const group = new THREE.Group();
        group.name = 'sysworld-fleet';
        if (inst.stage && inst.stage.scene) { inst.stage.scene.add(group); }

        // Module-level temps: no THREE allocations inside update().
        const _v1 = new THREE.Vector3();
        const _v2 = new THREE.Vector3();
        const _v3 = new THREE.Vector3();
        const _c1 = new THREE.Color();
        const WHITE = new THREE.Color(0xffffff);
        const UP = new THREE.Vector3(0, 1, 0);

        // Shared unit geometries; every mesh scales them to size.
        const boxGeo = new THREE.BoxGeometry(1, 1, 1);
        const octaGeo = new THREE.OctahedronGeometry(1, 0);
        const coneGeo = new THREE.ConeGeometry(0.55, 1.8, 8);
        const planeGeo = new THREE.PlaneGeometry(1, 1);
        const finGeo = new THREE.BoxGeometry(0.7, 0.08, 0.3);      // drone wing fins
        const gearGeo = new THREE.TorusGeometry(1.1, 0.12, 6, 20); // daemon gear ring
        const plateEdgeGeo = new THREE.EdgesGeometry(boxGeo);      // tool plate accent frame
        // 12 radial tick marks (radius ~1.6) forming one mission halo ring.
        const TICK_COUNT = 12;
        const tickPos = new Float32Array(TICK_COUNT * 6);
        for (let i = 0; i < TICK_COUNT; i++) {
            const ta = (i / TICK_COUNT) * Math.PI * 2;
            tickPos[i * 6] = Math.cos(ta) * 1.3;
            tickPos[i * 6 + 1] = Math.sin(ta) * 1.3;
            tickPos[i * 6 + 2] = 0;
            tickPos[i * 6 + 3] = Math.cos(ta) * 1.6;
            tickPos[i * 6 + 4] = Math.sin(ta) * 1.6;
            tickPos[i * 6 + 5] = 0;
        }
        const tickGeo = new THREE.BufferGeometry();
        tickGeo.setAttribute('position', new THREE.BufferAttribute(tickPos, 3));

        // Material caches keyed by "hex|opacity" to avoid per-entity clones.
        const solidMats = new Map();
        const lineMats = new Map();
        function solidMat(hex, opacity) {
            const key = hex + '|' + opacity;
            let m = solidMats.get(key);
            if (!m) {
                m = new THREE.MeshBasicMaterial({ color: hex, transparent: true, opacity: opacity });
                solidMats.set(key, m);
            }
            return m;
        }
        function lineMat(hex, opacity) {
            const key = hex + '|' + opacity;
            let m = lineMats.get(key);
            if (!m) {
                m = new THREE.LineBasicMaterial({
                    color: hex, transparent: true, opacity: opacity,
                    blending: THREE.AdditiveBlending, depthWrite: false
                });
                lineMats.set(key, m);
            }
            return m;
        }

        // Guarded call into the shared effects module (may be absent).
        function fxCall(name, args) {
            const f = fx[name];
            if (typeof f !== 'function') { return undefined; }
            try { return f.apply(fx, args); } catch (err) { return undefined; }
        }

        // Spawn-in scale tween (0.001 -> 1) for newly added records. Degrades
        // to a no-op leaving full scale when fx.tween is unavailable.
        function spawnIn(apply) {
            const handle = fxCall('tween', [{
                duration: 0.45, ease: 'outCubic',
                update: function (e) { apply(0.001 + e * 0.999); }
            }]);
            if (handle) { apply(0.001); }
        }

        // Stable pickables array for the entry's raycaster.
        const pickables = [];
        function setPick(mesh, kind, id, label, payload) {
            mesh.userData = { kind: kind, id: id, label: label, payload: payload };
            pickables.push(mesh);
        }
        function dropPick(mesh) {
            const i = pickables.indexOf(mesh);
            if (i >= 0) { pickables.splice(i, 1); }
        }
        function disposeGlow(sprite) {
            if (!sprite) { return; }
            if (sprite.parent) { sprite.parent.remove(sprite); }
            if (sprite.material) { sprite.material.dispose(); }
        }

        // Faint guide circles marking the ring districts.
        const guides = [];
        function makeRing(radius, hex, opacity, y) {
            const pos = new Float32Array((RING_PTS + 1) * 3);
            for (let i = 0; i <= RING_PTS; i++) {
                const a = (i / RING_PTS) * Math.PI * 2;
                pos[i * 3] = Math.cos(a) * radius;
                pos[i * 3 + 1] = y || 0;
                pos[i * 3 + 2] = Math.sin(a) * radius;
            }
            const geo = new THREE.BufferGeometry();
            geo.setAttribute('position', new THREE.BufferAttribute(pos, 3));
            const line = new THREE.Line(geo, lineMat(hex, opacity));
            line.frustumCulled = false;
            group.add(line);
            guides.push(line);
            return line;
        }
        makeRing(MISSION_R, C.mission, 0.14, 0);
        makeRing(CRON_R, C.cron, 0.12, 0);
        makeRing(BELT_R, C.tool, 0.07, 0);
        makeRing(46, C.storage, 0.10, INFRA_Y);
        makeRing(30, C.storage, 0.08, INFRA_Y);

        // ==================================================================
        // 1. Mission ring
        // ==================================================================
        const missions = new Map(); // id -> rec
        let missionOrder = [];      // stable slot order of mission ids

        function missionColor(status) {
            const s = String(status || '').toLowerCase();
            if (s === 'running' || s === 'active' || s === 'in_progress') { return C.ok; }
            if (s === 'queued' || s === 'waiting' || s === 'pending' || s === 'scheduled') { return C.warn; }
            if (s === 'error' || s === 'failed' || s === 'failure') { return C.error; }
            return C.dim;
        }
        function isRunning(status) {
            const s = String(status || '').toLowerCase();
            return s === 'running' || s === 'active' || s === 'in_progress';
        }
        function countArcs() {
            let n = 0;
            for (const rec of missions.values()) { if (rec.arc) { n++; } }
            return n;
        }

        // Shimmering arc from a running beacon toward the core crown.
        function makeArc(colorHex) {
            const geo = new THREE.BufferGeometry();
            const arr = new Float32Array(ARC_PTS * 3);
            geo.setAttribute('position', new THREE.BufferAttribute(arr, 3));
            const mat = new THREE.LineBasicMaterial({
                color: colorHex, transparent: true, opacity: 0.3,
                blending: THREE.AdditiveBlending, depthWrite: false
            });
            const line = new THREE.Line(geo, mat);
            line.frustumCulled = false;
            group.add(line);
            return { line: line, arr: arr, phase: Math.random() * Math.PI * 2 };
        }
        function disposeArc(rec) {
            if (!rec.arc) { return; }
            group.remove(rec.arc.line);
            rec.arc.line.geometry.dispose();
            rec.arc.line.material.dispose();
            rec.arc = null;
        }
        // Rewrite the arc's curve in place (quadratic bezier, no allocation).
        function updateArc(rec, arc, elapsed) {
            const p = rec.mesh.position;
            const mx = p.x * 0.5;
            const mz = p.z * 0.5;
            const my = 26 + Math.sin(elapsed * 0.7 + arc.phase) * 4;
            for (let i = 0; i < ARC_PTS; i++) {
                const t = i / (ARC_PTS - 1);
                const u = 1 - t;
                arc.arr[i * 3] = u * u * p.x + 2 * u * t * mx;
                arc.arr[i * 3 + 1] = u * u * p.y + 2 * u * t * my + t * t * 8;
                arc.arr[i * 3 + 2] = u * u * p.z + 2 * u * t * mz;
            }
            arc.line.geometry.attributes.position.needsUpdate = true;
            arc.line.material.opacity = 0.16 + 0.18 * (0.5 + 0.5 * Math.sin(elapsed * 2.6 + arc.phase));
        }

        function addMissionRec(m, id) {
            const color = missionColor(m.status);
            const rec = {
                id: id,
                label: m.name || id,
                payload: m,
                status: m.status,
                color: color,
                angle: 0, targetAngle: 0,
                phase: (hashString(id) % 628) / 100,
                isNew: true,
                mesh: null, glow: null, arc: null,
                name: null, ticks: null, tickMat: null, trail: null
            };
            const mesh = new THREE.Mesh(octaGeo, solidMat(color, 0.95));
            mesh.scale.set(1.7, 2.4, 1.7);
            rec.mesh = mesh;
            setPick(mesh, 'mission', id, rec.label, m);
            group.add(mesh);
            rec.glow = fxCall('makeGlowSprite', [color, 7]) || null;
            if (rec.glow) { group.add(rec.glow); }
            // Floating name tag under the beacon (texture stays fx-cached).
            rec.name = fxCall('textSprite', [rec.label, color, { scale: 0.6 }]) || null;
            if (rec.name) { group.add(rec.name); }
            // Rotating halo of 12 tick marks; the material is per-mission.
            rec.tickMat = new THREE.LineBasicMaterial({
                color: color, transparent: true, opacity: 0.55,
                blending: THREE.AdditiveBlending, depthWrite: false
            });
            rec.ticks = new THREE.LineSegments(tickGeo, rec.tickMat);
            rec.ticks.scale.setScalar(1.5);
            rec.ticks.frustumCulled = false;
            group.add(rec.ticks);
            missions.set(id, rec);
            spawnIn(function (k) { mesh.scale.set(1.7 * k, 2.4 * k, 1.7 * k); });
            return rec;
        }
        function disposeMissionTrail(rec) {
            if (rec.trail && typeof rec.trail.dispose === 'function') {
                try { rec.trail.dispose(); } catch (err) { /* effect module owns it */ }
            }
            rec.trail = null;
        }
        function removeMission(rec) {
            dropPick(rec.mesh);
            group.remove(rec.mesh); // shared geo / cached material survive
            disposeGlow(rec.glow);
            disposeGlow(rec.name); // sprite material only; texture stays cached
            if (rec.ticks) { group.remove(rec.ticks); }
            if (rec.tickMat) { rec.tickMat.dispose(); }
            disposeMissionTrail(rec);
            disposeArc(rec);
            missions.delete(rec.id);
        }
        function positionMission(rec, elapsed) {
            const y = Math.sin(elapsed * 0.9 + rec.phase) * 0.5;
            rec.mesh.position.set(Math.cos(rec.angle) * MISSION_R, y, Math.sin(rec.angle) * MISSION_R);
            if (rec.glow) { rec.glow.position.copy(rec.mesh.position); }
            if (rec.name) {
                rec.name.position.set(rec.mesh.position.x, rec.mesh.position.y - 2.4, rec.mesh.position.z);
            }
            if (rec.ticks) { rec.ticks.position.copy(rec.mesh.position); }
        }

        // Diff-driven update from GET /api/missions/v2 -> { missions: [...] }.
        function setMissions(list) {
            const arr = Array.isArray(list) ? list.slice(0, CAP_MISSIONS) : [];
            const seen = new Set();
            for (let i = 0; i < arr.length; i++) {
                const m = arr[i];
                if (!m) { continue; }
                const id = String(m.id != null ? m.id : (m.name || ''));
                if (!id || seen.has(id)) { continue; }
                seen.add(id);
                let rec = missions.get(id);
                if (!rec) { rec = addMissionRec(m, id); }
                rec.label = m.name || id;
                rec.payload = m;
                rec.status = m.status;
                rec.mesh.userData.label = rec.label;
                rec.mesh.userData.payload = m;
                const col = missionColor(m.status);
                if (col !== rec.color) {
                    rec.color = col;
                    rec.mesh.material = solidMat(col, 0.95);
                    if (rec.glow && rec.glow.material) { rec.glow.material.color.setHex(col); }
                    if (rec.tickMat) { rec.tickMat.color.setHex(col); }
                }
                const run = isRunning(m.status);
                if (run && !rec.arc && countArcs() < MAX_ARCS) { rec.arc = makeArc(col); }
                if (!run && rec.arc) { disposeArc(rec); }
                // Running beacons drag a comet trail; it dies with the state.
                if (run && !rec.trail) { rec.trail = fxCall('trailFor', [rec.mesh, col, 20]) || null; }
                if (!run && rec.trail) { disposeMissionTrail(rec); }
            }
            for (const entry of missions) {
                if (!seen.has(entry[0])) { removeMission(entry[1]); }
            }
            // Stable slot order: keep previous relative order, append new ids.
            missionOrder = missionOrder.filter(function (id) { return seen.has(id); });
            seen.forEach(function (id) { if (missionOrder.indexOf(id) < 0) { missionOrder.push(id); } });
            const n = missionOrder.length || 1;
            for (let i = 0; i < missionOrder.length; i++) {
                const rec = missions.get(missionOrder[i]);
                if (!rec) { continue; }
                rec.targetAngle = (i / n) * Math.PI * 2;
                if (rec.isNew) {
                    rec.angle = rec.targetAngle;
                    positionMission(rec, 0);
                    fxCall('burst', [rec.mesh.position, rec.color, 12]);
                    rec.isNew = false;
                }
            }
        }

        function updateMissions(dt, elapsed) {
            for (const rec of missions.values()) {
                // Shortest-arc angular easing toward the assigned slot.
                let d = rec.targetAngle - rec.angle;
                d = ((d + Math.PI) % (Math.PI * 2) + Math.PI * 2) % (Math.PI * 2) - Math.PI;
                rec.angle += d * Math.min(1, dt * 2.5);
                positionMission(rec, elapsed);
                rec.mesh.rotation.y += dt * 0.6;
                const running = isRunning(rec.status);
                if (rec.glow) {
                    const pulse = running ? 1 + 0.35 * Math.sin(elapsed * 4 + rec.phase) : 1;
                    const s = 7 * pulse;
                    rec.glow.scale.set(s, s, 1);
                }
                if (rec.arc) { updateArc(rec, rec.arc, elapsed); }
                if (rec.ticks) { rec.ticks.rotation.z += dt * 0.3; }
                if (rec.trail && typeof rec.trail.update === 'function') { rec.trail.update(dt); }
            }
        }

        // ==================================================================
        // 2. Cron dial
        // ==================================================================
        const cronJobs = new Map(); // id -> rec
        const handMat = new THREE.MeshBasicMaterial({
            color: C.cron, transparent: true, opacity: 0.35,
            blending: THREE.AdditiveBlending, depthWrite: false
        });
        const handPivot = new THREE.Group();
        const hand = new THREE.Mesh(boxGeo, handMat);
        hand.scale.set(0.35, 0.35, CRON_R - 4);
        hand.position.set(0, 0, (CRON_R - 4) / 2 + 2);
        hand.frustumCulled = false;
        handPivot.add(hand);
        group.add(handPivot);

        // Diff-driven update from dashboard activity -> cron_jobs: [...].
        function setCron(list) {
            const arr = Array.isArray(list) ? list.slice(0, CAP_CRON) : [];
            const seen = new Set();
            for (let i = 0; i < arr.length; i++) {
                const j = arr[i];
                if (!j) { continue; }
                const id = String(j.id != null ? j.id : (j.cron_expr || i));
                if (seen.has(id)) { continue; }
                seen.add(id);
                let rec = cronJobs.get(id);
                if (!rec) {
                    const mesh = new THREE.Mesh(boxGeo, solidMat(C.cron, 0.9));
                    mesh.scale.set(0.3, 1.0, 2.2);
                    group.add(mesh);
                    rec = { id: id, mesh: mesh, payload: j };
                    cronJobs.set(id, rec);
                    setPick(mesh, 'cron', id, j.cron_expr || id, j);
                }
                rec.payload = j;
                rec.mesh.userData.payload = j;
                // Disabled jobs are dimmed, enabled jobs glow in cron green.
                rec.mesh.material = j.disabled ? solidMat(C.dim, 0.35) : solidMat(C.cron, 0.9);
            }
            for (const entry of cronJobs) {
                if (!seen.has(entry[0])) {
                    dropPick(entry[1].mesh);
                    group.remove(entry[1].mesh);
                    cronJobs.delete(entry[0]);
                }
            }
            // Even angular slots, sorted by id for deterministic placement.
            const ids = Array.from(cronJobs.keys()).sort();
            const n = ids.length || 1;
            for (let i = 0; i < ids.length; i++) {
                const rec = cronJobs.get(ids[i]);
                const a = (i / n) * Math.PI * 2;
                rec.mesh.position.set(Math.cos(a) * CRON_R, 0, Math.sin(a) * CRON_R);
                rec.mesh.rotation.y = -a; // radial orientation
            }
        }

        // ==================================================================
        // 3. Co-agent drones
        // ==================================================================
        const drones = new Map(); // id -> rec

        function droneColor(state) {
            const s = String(state || '').toLowerCase();
            if (s === 'running' || s === 'active' || s === 'working') { return C.agent; }
            if (s === 'queued' || s === 'pending' || s === 'waiting') { return C.warn; }
            if (s === 'done' || s === 'completed' || s === 'finished' || s === 'success') { return C.ok; }
            if (s === 'error' || s === 'failed' || s === 'failure') { return C.error; }
            return C.dim;
        }
        function isDroneRunning(state) {
            const s = String(state || '').toLowerCase();
            return s === 'running' || s === 'active' || s === 'working';
        }
        // Gentle lissajous orbit around the core, evaluated into `out`.
        function dronePosition(rec, t, out) {
            out.set(
                Math.cos(t * rec.w1 + rec.p1) * rec.r,
                rec.yBase + Math.sin(t * rec.w3 + rec.p3) * rec.yAmp,
                Math.sin(t * rec.w2 + rec.p2) * rec.r * 0.92
            );
        }

        function addDrone(a, id) {
            const rand = mulberry32(hashString(id));
            const color = droneColor(a.state);
            // Own material per drone so the despawn fade stays local.
            const mat = new THREE.MeshBasicMaterial({ color: color, transparent: true, opacity: 0.95 });
            const mesh = new THREE.Mesh(coneGeo, mat);
            const rec = {
                id: id,
                label: a.task || a.specialist || id,
                payload: a,
                color: color,
                state: a.state,
                mesh: mesh, glow: null, trail: null,
                engine: null, blink: null,
                t: 0, bornT: 0,
                leaving: false, leaveT: 0,
                r: 16 + rand() * 16,
                w1: 0.18 + rand() * 0.22, w2: 0.16 + rand() * 0.2, w3: 0.3 + rand() * 0.4,
                p1: rand() * Math.PI * 2, p2: rand() * Math.PI * 2, p3: rand() * Math.PI * 2,
                yBase: 7 + rand() * 9, yAmp: 2.5 + rand() * 4
            };
            // Wing fins share the body material and ride the cone transform.
            const finL = new THREE.Mesh(finGeo, mat);
            finL.position.set(-0.58, -0.35, 0);
            finL.rotation.z = 0.4;
            const finR = new THREE.Mesh(finGeo, mat);
            finR.position.set(0.58, -0.35, 0);
            finR.rotation.z = -0.4;
            mesh.add(finL);
            mesh.add(finR);
            setPick(mesh, 'coagent', id, rec.label, a);
            group.add(mesh);
            rec.glow = fxCall('makeGlowSprite', [color, 5]) || null;
            if (rec.glow) { group.add(rec.glow); }
            // Engine exhaust glow at the tail + state blink light at the nose.
            rec.engine = fxCall('makeGlowSprite', [color, 1.2]) || null;
            if (rec.engine) { group.add(rec.engine); }
            rec.blink = fxCall('makeGlowSprite', [color, 0.55]) || null;
            if (rec.blink) { group.add(rec.blink); }
            rec.trail = fxCall('trailFor', [mesh, color, Math.round(26 * quality)]) || null;
            drones.set(id, rec);
            // Spawn burst at the drone's initial orbit position.
            dronePosition(rec, 0, _v1);
            mesh.position.copy(_v1);
            if (rec.glow) { rec.glow.position.copy(_v1); }
            if (rec.engine) { rec.engine.position.copy(_v1); }
            if (rec.blink) { rec.blink.position.copy(_v1); }
            fxCall('burst', [_v1, color, 12]);
            return rec;
        }
        function removeDrone(rec) {
            dropPick(rec.mesh);
            group.remove(rec.mesh);
            rec.mesh.material.dispose(); // per-drone clone (fins share it)
            disposeGlow(rec.glow);
            disposeGlow(rec.engine);
            disposeGlow(rec.blink);
            if (rec.trail && typeof rec.trail.dispose === 'function') {
                try { rec.trail.dispose(); } catch (err) { /* effect module owns it */ }
            }
            drones.delete(rec.id);
        }

        // Diff-driven update from dashboard activity -> coagents: [...].
        function setCoAgents(list) {
            const arr = Array.isArray(list) ? list.slice(0, CAP_DRONES) : [];
            const seen = new Set();
            for (let i = 0; i < arr.length; i++) {
                const a = arr[i];
                if (!a) { continue; }
                const id = String(a.id != null ? a.id : (a.task || ''));
                if (!id || seen.has(id)) { continue; }
                seen.add(id);
                let rec = drones.get(id);
                if (!rec) { rec = addDrone(a, id); }
                rec.label = a.task || a.specialist || id;
                rec.payload = a;
                rec.state = a.state;
                rec.mesh.userData.label = rec.label;
                rec.mesh.userData.payload = a;
                const col = droneColor(a.state);
                if (col !== rec.color) {
                    rec.color = col;
                    rec.mesh.material.color.setHex(col);
                    if (rec.glow && rec.glow.material) { rec.glow.material.color.setHex(col); }
                    if (rec.engine && rec.engine.material) { rec.engine.material.color.setHex(col); }
                    if (rec.blink && rec.blink.material) { rec.blink.material.color.setHex(col); }
                }
            }
            // Missing ids despawn by flying outward with a fade.
            for (const entry of drones) {
                if (!seen.has(entry[0]) && !entry[1].leaving) {
                    entry[1].leaving = true;
                    entry[1].leaveT = 0;
                }
            }
        }

        function updateDrones(dt, elapsed) {
            for (const rec of drones.values()) {
                if (rec.leaving) {
                    rec.leaveT += dt;
                    const k = Math.min(1, rec.leaveT / DRONE_LEAVE_SECS);
                    // Accelerate radially outward and fade out.
                    rec.mesh.position.multiplyScalar(1 + dt * 0.9);
                    rec.mesh.position.y += dt * 6;
                    rec.mesh.material.opacity = 0.95 * (1 - k);
                    if (rec.glow) {
                        rec.glow.position.copy(rec.mesh.position);
                        if (rec.glow.material) { rec.glow.material.opacity = 1 - k; }
                    }
                    if (rec.engine) {
                        rec.engine.position.copy(rec.mesh.position);
                        if (rec.engine.material) { rec.engine.material.opacity = 0.85 * (1 - k); }
                    }
                    if (rec.blink) {
                        rec.blink.position.copy(rec.mesh.position);
                        if (rec.blink.material) { rec.blink.material.opacity = 0; }
                    }
                    if (k >= 1) { removeDrone(rec); }
                    continue;
                }
                rec.t += dt;
                rec.bornT = Math.min(1, rec.bornT + dt / 0.7);
                dronePosition(rec, rec.t, _v1);
                // Orient the cone tip along the velocity vector.
                dronePosition(rec, rec.t + 0.08, _v2);
                _v3.subVectors(_v2, _v1);
                if (_v3.lengthSq() > 0.000001) {
                    _v3.normalize();
                    rec.mesh.quaternion.setFromUnitVectors(UP, _v3);
                }
                rec.mesh.position.copy(_v1);
                const s = 1.4 * (0.2 + 0.8 * rec.bornT); // scale-in on spawn
                rec.mesh.scale.set(s, s, s);
                if (rec.glow) {
                    rec.glow.position.copy(_v1);
                    const g = 5 * (0.3 + 0.7 * rec.bornT);
                    rec.glow.scale.set(g, g, 1);
                }
                if (rec.engine) {
                    // Exhaust sits behind the tail and flickers while running.
                    rec.engine.position.copy(_v1).addScaledVector(_v3, -(0.9 * s + 0.4));
                    const e = 1.2 * (0.3 + 0.7 * rec.bornT);
                    rec.engine.scale.set(e, e, 1);
                    if (rec.engine.material) {
                        rec.engine.material.opacity = isDroneRunning(rec.state)
                            ? 0.85 + 0.15 * Math.sin(elapsed * 9 + rec.p1)
                            : 0.25;
                    }
                }
                if (rec.blink) {
                    // Nose beacon: ~2 Hz square-wave blink, running only.
                    rec.blink.position.copy(_v1).addScaledVector(_v3, 0.9 * s + 0.25);
                    const b = 0.55 * (0.3 + 0.7 * rec.bornT);
                    rec.blink.scale.set(b, b, 1);
                    if (rec.blink.material) {
                        rec.blink.material.opacity =
                            isDroneRunning(rec.state) && Math.sin(elapsed * 12.566 + rec.p2) > 0 ? 0.9 : 0;
                    }
                }
                if (rec.trail && typeof rec.trail.update === 'function') { rec.trail.update(dt); }
            }
        }

        // ==================================================================
        // 4. Tool belt
        // ==================================================================
        const tools = new Map(); // tool name -> rec

        // Canvas glyph label for a plate (created on diff, never per frame).
        function makeLabelTexture(text) {
            if (typeof document === 'undefined') { return null; }
            const canvas = document.createElement('canvas');
            canvas.width = 256; canvas.height = 96;
            const ctx = canvas.getContext('2d');
            if (!ctx) { return null; }
            ctx.clearRect(0, 0, 256, 96);
            ctx.font = 'bold 40px Consolas, Menlo, monospace';
            ctx.textAlign = 'center';
            ctx.textBaseline = 'middle';
            ctx.fillStyle = 'rgba(255,255,255,0.92)';
            ctx.fillText(text, 128, 50);
            const tex = new THREE.CanvasTexture(canvas);
            tex.minFilter = THREE.LinearFilter;
            tex.generateMipmaps = false;
            return tex;
        }

        function addTool(item, name) {
            const count = Math.max(0, item.count | 0);
            const rand = mulberry32(hashString(name));
            const sizeK = 1 + Math.log(1 + count) * 0.3;
            // Plate = thin box body + glyph face, grouped so scaling stays
            // uniform and the label never distorts.
            const plate = new THREE.Group();
            const mat = new THREE.MeshBasicMaterial({
                color: C.tool, transparent: true,
                opacity: Math.min(0.95, 0.5 + Math.log(1 + count) * 0.1)
            });
            const body = new THREE.Mesh(boxGeo, mat);
            body.scale.set(5.4, 3.1, 0.35);
            plate.add(body);
            // Accent frame around the plate body; shared edge geometry,
            // per-plate material in the tool color.
            const frameMat = new THREE.LineBasicMaterial({
                color: C.tool, transparent: true, opacity: 0.65,
                blending: THREE.AdditiveBlending, depthWrite: false
            });
            body.add(new THREE.LineSegments(plateEdgeGeo, frameMat));
            let face = null, faceMat = null, faceTex = null;
            const label = name.toUpperCase();
            faceTex = makeLabelTexture(label.length > 12 ? label.slice(0, 11) + '…' : label);
            if (faceTex) {
                faceMat = new THREE.MeshBasicMaterial({ map: faceTex, transparent: true, opacity: 0.95, depthWrite: false });
                face = new THREE.Mesh(planeGeo, faceMat);
                face.scale.set(5.0, 1.9, 1);
                face.position.z = 0.22;
                plate.add(face);
            }
            plate.scale.setScalar(sizeK);
            const rec = {
                name: name,
                label: name,
                payload: item,
                count: count,
                plate: plate, body: body,
                face: face, faceMat: faceMat, faceTex: faceTex,
                frameMat: frameMat,
                glow: null,
                glowBaseOpacity: 0.6,
                sizeK: sizeK,
                angle: 0, targetAngle: 0,
                radius: BELT_R + (rand() - 0.5) * 14,
                yBase: (rand() - 0.5) * 8,
                bobPh: rand() * Math.PI * 2,
                flashT: 0,
                isNew: true
            };
            setPick(body, 'tool', name, name, item);
            group.add(plate);
            rec.glow = fxCall('makeGlowSprite', [C.tool, 6 * sizeK]) || null;
            if (rec.glow) {
                group.add(rec.glow);
                if (rec.glow.material) { rec.glowBaseOpacity = rec.glow.material.opacity; }
            }
            tools.set(name, rec);
            spawnIn(function (k) { rec.plate.scale.setScalar(rec.sizeK * k); });
            return rec;
        }
        function removeTool(rec) {
            dropPick(rec.body);
            group.remove(rec.plate);
            rec.body.material.dispose();
            if (rec.faceMat) { rec.faceMat.dispose(); }
            if (rec.faceTex) { rec.faceTex.dispose(); }
            if (rec.frameMat) { rec.frameMat.dispose(); }
            disposeGlow(rec.glow);
            tools.delete(rec.name);
        }

        // Diff-driven update from /api/dashboard/tool-stats -> top_tools.
        function setTools(list) {
            const arr = Array.isArray(list) ? list.slice(0, CAP_TOOLS) : [];
            const seen = new Set();
            for (let i = 0; i < arr.length; i++) {
                const item = arr[i];
                if (!item) { continue; }
                const name = String(item.tool || item.name || '');
                if (!name || seen.has(name)) { continue; }
                seen.add(name);
                let rec = tools.get(name);
                if (!rec) { rec = addTool(item, name); }
                rec.payload = item;
                rec.count = Math.max(0, item.count | 0);
                rec.body.userData.payload = item;
            }
            for (const entry of tools) {
                if (!seen.has(entry[0])) { removeTool(entry[1]); }
            }
            // Even angular slots in list order (top tools first).
            const names = [];
            seen.forEach(function (n) { names.push(n); });
            const n = names.length || 1;
            for (let i = 0; i < names.length; i++) {
                const rec = tools.get(names[i]);
                if (!rec) { continue; }
                rec.targetAngle = (i / n) * Math.PI * 2;
                if (rec.isNew) {
                    rec.angle = rec.targetAngle;
                    rec.isNew = false;
                }
            }
        }

        // Flash the matching plate and hand its world position back so the
        // entry can fire a comet from the core. Returns null when unknown.
        function flashTool(name) {
            const rec = tools.get(String(name));
            if (!rec) { return null; }
            rec.flashT = 1;
            fxCall('pulseRing', [rec.plate.position, C.tool, 10]);
            // Heat soak: glow opacity spikes, then cools back to baseline.
            if (rec.glow && rec.glow.material) {
                const gm = rec.glow.material;
                const base = rec.glowBaseOpacity;
                const tw = fxCall('tween', [{
                    duration: 2, ease: 'outCubic',
                    update: function (e) { gm.opacity = 0.95 + (base - 0.95) * e; }
                }]);
                if (tw) { gm.opacity = 0.95; }
            }
            return rec.plate.getWorldPosition(new THREE.Vector3());
        }

        function updateTools(dt, elapsed) {
            for (const rec of tools.values()) {
                // Shortest-arc angular easing toward the assigned slot.
                let d = rec.targetAngle - rec.angle;
                d = ((d + Math.PI) % (Math.PI * 2) + Math.PI * 2) % (Math.PI * 2) - Math.PI;
                rec.angle += d * Math.min(1, dt * 2.5);
                const y = rec.yBase + Math.sin(elapsed * 0.6 + rec.bobPh) * 1.2;
                rec.plate.position.set(Math.cos(rec.angle) * rec.radius, y, Math.sin(rec.angle) * rec.radius);
                // Face outward radially with a slight asteroid-like wobble.
                rec.plate.rotation.y = Math.PI / 2 - rec.angle + Math.sin(elapsed * 0.3 + rec.bobPh) * 0.3;
                if (rec.glow) {
                    rec.glow.position.copy(rec.plate.position);
                    const g = 6 * rec.sizeK * (1 + rec.flashT * 1.6);
                    rec.glow.scale.set(g, g, 1);
                }
                if (rec.flashT > 0) {
                    rec.flashT = Math.max(0, rec.flashT - dt * 1.4);
                    _c1.setHex(C.tool).lerp(WHITE, rec.flashT);
                    rec.body.material.color.copy(_c1);
                    rec.body.material.opacity = 0.7 + 0.3 * rec.flashT;
                }
            }
        }

        // ==================================================================
        // 5. Infrastructure field
        // ==================================================================
        const containers = new Map(); // id -> rec
        const daemons = new Map();    // skill_name -> rec
        let infraHalfDepth = 24;      // half the container grid depth

        function containerColor(c) {
            const s = String(c.state || c.status || '').toLowerCase();
            if (s.indexOf('run') >= 0 || s === 'up' || s === 'healthy') { return C.ok; }
            if (s.indexOf('exit') >= 0 || s.indexOf('dead') >= 0 || s.indexOf('stop') >= 0) { return C.error; }
            if (s.indexOf('pause') >= 0 || s.indexOf('restart') >= 0) { return C.warn; }
            return C.dim;
        }
        function daemonColor(d) {
            if (d.auto_disabled) { return C.dim; }
            const s = String(d.status || '').toLowerCase();
            if (s === 'running' || s === 'active' || s === 'ok' || s === 'healthy') { return C.ok; }
            if (s === 'error' || s === 'failed' || s === 'crashed') { return C.error; }
            if (s === 'stopped' || s === 'disabled' || s === 'paused') { return C.warn; }
            return C.dim;
        }

        function addContainer(c, id) {
            const color = containerColor(c);
            const mesh = new THREE.Mesh(boxGeo, solidMat(color, 0.9));
            mesh.scale.set(3.4, 3.4, 3.4);
            // Wireframe overlay in the status color; geometry is per-container.
            const edgeGeo = new THREE.EdgesGeometry(boxGeo);
            const edgeMat = new THREE.LineBasicMaterial({
                color: color, transparent: true, opacity: 0.5,
                blending: THREE.AdditiveBlending, depthWrite: false
            });
            mesh.add(new THREE.LineSegments(edgeGeo, edgeMat));
            const rec = {
                id: id,
                label: c.name || id,
                payload: c,
                color: color,
                state: c.state || c.status,
                restarting: String(c.state || c.status || '').toLowerCase().indexOf('restart') >= 0,
                mesh: mesh, glow: null,
                edgeGeo: edgeGeo, edgeMat: edgeMat,
                baseX: 0, baseZ: 0,
                bobPh: (hashString(id) % 628) / 100
            };
            setPick(mesh, 'container', id, rec.label, c);
            group.add(mesh);
            containers.set(id, rec);
            updateContainerGlow(rec);
            spawnIn(function (k) { mesh.scale.set(3.4 * k, 3.4 * k, 3.4 * k); });
            return rec;
        }
        // Only running containers earn a glow sprite, to cap visual noise.
        function updateContainerGlow(rec) {
            if (rec.color === C.ok && !rec.glow) {
                rec.glow = fxCall('makeGlowSprite', [C.ok, 6]) || null;
                if (rec.glow) { group.add(rec.glow); }
            } else if (rec.color !== C.ok && rec.glow) {
                disposeGlow(rec.glow);
                rec.glow = null;
            }
        }
        function removeContainer(rec) {
            dropPick(rec.mesh);
            group.remove(rec.mesh);
            if (rec.edgeGeo) { rec.edgeGeo.dispose(); }
            if (rec.edgeMat) { rec.edgeMat.dispose(); }
            disposeGlow(rec.glow);
            containers.delete(rec.id);
        }
        // Loose square grid centered below the core.
        function layoutContainers() {
            const ids = Array.from(containers.keys());
            const n = ids.length;
            if (!n) { infraHalfDepth = 24; return; }
            const cols = Math.ceil(Math.sqrt(n));
            const rows = Math.ceil(n / cols);
            const spacing = 9;
            for (let i = 0; i < ids.length; i++) {
                const rec = containers.get(ids[i]);
                const col = i % cols;
                const row = Math.floor(i / cols);
                rec.baseX = (col - (cols - 1) / 2) * spacing;
                rec.baseZ = (row - (rows - 1) / 2) * spacing;
            }
            infraHalfDepth = ((rows - 1) / 2) * spacing;
        }

        function daemonRestarts(d) {
            if (!d) { return 0; }
            const r = d.restarts != null ? d.restarts : d.restart_count;
            return Math.max(0, r | 0);
        }
        // Restart-count tag; rebuilt only when the count actually changes.
        function updateDaemonLabel(rec) {
            if (rec.rcLabel) { disposeGlow(rec.rcLabel); rec.rcLabel = null; }
            if (rec.restarts > 0) {
                rec.rcLabel = fxCall('textSprite', ['×' + rec.restarts, rec.color, { scale: 0.45 }]) || null;
                if (rec.rcLabel) {
                    rec.rcLabel.position.set(rec.mesh.position.x, rec.mesh.position.y + 1.6, rec.mesh.position.z);
                    group.add(rec.rcLabel);
                }
            }
        }
        function addDaemon(d, id) {
            const color = daemonColor(d);
            const mesh = new THREE.Mesh(boxGeo, solidMat(color, 0.9));
            mesh.scale.set(0.9, 6.5, 0.9);
            const rec = {
                id: id,
                label: d.skill_name || id,
                payload: d,
                color: color,
                mesh: mesh,
                gear: null, rcLabel: null,
                restarts: daemonRestarts(d),
                baseX: 0, baseZ: 0,
                bobPh: (hashString(id) % 628) / 100
            };
            // Slowly rotating gear ring around the pylon shaft.
            rec.gear = new THREE.Mesh(gearGeo, solidMat(color, 0.75));
            group.add(rec.gear);
            setPick(mesh, 'daemon', id, rec.label, d);
            group.add(mesh);
            daemons.set(id, rec);
            updateDaemonLabel(rec);
            return rec;
        }
        function removeDaemon(rec) {
            dropPick(rec.mesh);
            group.remove(rec.mesh);
            if (rec.gear) { group.remove(rec.gear); } // shared geo / cached mat survive
            if (rec.rcLabel) { disposeGlow(rec.rcLabel); rec.rcLabel = null; }
            daemons.delete(rec.id);
        }
        // Slim pylons lined up along the back edge of the field.
        function layoutDaemons() {
            const ids = Array.from(daemons.keys()).sort();
            const n = ids.length;
            if (!n) { return; }
            const z = -(infraHalfDepth + 12);
            for (let i = 0; i < ids.length; i++) {
                const rec = daemons.get(ids[i]);
                rec.baseX = (i - (n - 1) / 2) * 4.5;
                rec.baseZ = z;
                rec.mesh.position.set(rec.baseX, INFRA_Y + 3.4, rec.baseZ);
            }
        }

        // Diff-driven update: setInfra({ containers, daemons }).
        function setInfra(data) {
            const clist = data && Array.isArray(data.containers) ? data.containers.slice(0, CAP_CONTAINERS) : [];
            const seenC = new Set();
            for (let i = 0; i < clist.length; i++) {
                const c = clist[i];
                if (!c) { continue; }
                const id = String(c.id || c.name || '');
                if (!id || seenC.has(id)) { continue; }
                seenC.add(id);
                let rec = containers.get(id);
                if (!rec) { rec = addContainer(c, id); }
                rec.label = c.name || id;
                rec.payload = c;
                rec.mesh.userData.label = rec.label;
                rec.mesh.userData.payload = c;
                const col = containerColor(c);
                if (col !== rec.color) {
                    rec.color = col;
                    rec.mesh.material = solidMat(col, 0.9);
                    if (rec.edgeMat) { rec.edgeMat.color.setHex(col); }
                    updateContainerGlow(rec);
                }
                rec.state = c.state || c.status;
                rec.restarting = String(rec.state || '').toLowerCase().indexOf('restart') >= 0;
            }
            for (const entry of containers) {
                if (!seenC.has(entry[0])) { removeContainer(entry[1]); }
            }
            layoutContainers();

            const dlist = data && Array.isArray(data.daemons) ? data.daemons.slice(0, CAP_DAEMONS) : [];
            const seenD = new Set();
            for (let i = 0; i < dlist.length; i++) {
                const d = dlist[i];
                if (!d) { continue; }
                const id = String(d.skill_name || d.name || '');
                if (!id || seenD.has(id)) { continue; }
                seenD.add(id);
                let rec = daemons.get(id);
                if (!rec) { rec = addDaemon(d, id); }
                rec.label = d.skill_name || id;
                rec.payload = d;
                rec.mesh.userData.label = rec.label;
                rec.mesh.userData.payload = d;
                const col = daemonColor(d);
                if (col !== rec.color) {
                    rec.color = col;
                    rec.mesh.material = solidMat(col, 0.9);
                    if (rec.gear) { rec.gear.material = solidMat(col, 0.75); }
                }
                const rc = daemonRestarts(d);
                if (rc !== rec.restarts) { rec.restarts = rc; updateDaemonLabel(rec); }
            }
            for (const entry of daemons) {
                if (!seenD.has(entry[0])) { removeDaemon(entry[1]); }
            }
            layoutDaemons();
        }

        // Gentle hover float for the whole field.
        function updateInfra(dt, elapsed) {
            for (const rec of containers.values()) {
                rec.mesh.position.set(
                    rec.baseX,
                    INFRA_Y + 2 + Math.sin(elapsed * 1.1 + rec.bobPh) * 0.55,
                    rec.baseZ
                );
                if (rec.glow) { rec.glow.position.copy(rec.mesh.position); }
                if (rec.edgeMat) {
                    // Restarting containers pulse their wireframe.
                    rec.edgeMat.opacity = rec.restarting
                        ? 0.3 + 0.5 * Math.abs(Math.sin(elapsed * 6))
                        : 0.5;
                }
            }
            for (const rec of daemons.values()) {
                rec.mesh.position.y = INFRA_Y + 3.4 + Math.sin(elapsed * 0.8 + rec.bobPh) * 0.18;
                if (rec.gear) {
                    rec.gear.position.copy(rec.mesh.position);
                    rec.gear.rotation.z += dt * 0.5;
                }
                if (rec.rcLabel) {
                    rec.rcLabel.position.set(rec.mesh.position.x, rec.mesh.position.y + 1.6, rec.mesh.position.z);
                }
            }
        }

        // ==================================================================
        // Frame update + lifecycle
        // ==================================================================
        function update(dt, elapsed) {
            updateMissions(dt, elapsed);
            // Slowly rotating cron dial hand with a soft shimmer.
            handPivot.rotation.y = elapsed * HAND_SPEED;
            handMat.opacity = 0.3 + 0.12 * (0.5 + 0.5 * Math.sin(elapsed * 1.4));
            updateDrones(dt, elapsed);
            updateTools(dt, elapsed);
            updateInfra(dt, elapsed);
        }

        // Free every GPU resource and detach the fleet group.
        function dispose() {
            for (const entry of Array.from(missions)) { removeMission(entry[1]); }
            for (const entry of Array.from(cronJobs)) {
                dropPick(entry[1].mesh);
                group.remove(entry[1].mesh);
                cronJobs.delete(entry[0]);
            }
            for (const entry of Array.from(drones)) { removeDrone(entry[1]); }
            for (const entry of Array.from(tools)) { removeTool(entry[1]); }
            for (const entry of Array.from(containers)) { removeContainer(entry[1]); }
            for (const entry of Array.from(daemons)) { removeDaemon(entry[1]); }
            missionOrder = [];
            pickables.length = 0;
            for (let i = 0; i < guides.length; i++) {
                group.remove(guides[i]);
                guides[i].geometry.dispose();
            }
            guides.length = 0;
            handPivot.remove(hand);
            group.remove(handPivot);
            handMat.dispose();
            boxGeo.dispose();
            octaGeo.dispose();
            coneGeo.dispose();
            planeGeo.dispose();
            finGeo.dispose();
            gearGeo.dispose();
            plateEdgeGeo.dispose();
            tickGeo.dispose();
            solidMats.forEach(function (m) { m.dispose(); });
            solidMats.clear();
            lineMats.forEach(function (m) { m.dispose(); });
            lineMats.clear();
            if (group.parent) { group.parent.remove(group); }
        }

        return {
            setMissions: setMissions,
            setCron: setCron,
            setCoAgents: setCoAgents,
            setTools: setTools,
            setInfra: setInfra,
            flashTool: flashTool,
            pickables: pickables,
            update: update,
            dispose: dispose
        };
    };
})();
