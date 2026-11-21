// Clockwork Observatory 3D toys (plan R5 step 5, AM2): the orrery on the disc hub, the telescope barrel along the tube, the gear train
// (left back corner), the dome with its clock (right back corner) and a flush brass clock bezel round the disc.
// buildToys(THREE, kit, layout, { tier, low, models, reduced, isReduced }) -> { group, update(dt, t, snap, toyState, lamps, events), dispose() }.
// Frame: X = x, Y = h, Z = -y. Each toy clones its root from models.get('toys') (nodes by name, AM2) or builds a procedural stand-in with the
// same animated node names, so one driver serves both. Reduced motion freezes decorative motion; the disc angle is physics and stays honest
// except that the armillary arms follow it at 0.2 rad/s at most.
import { mergeGeometries } from 'three/addons/utils/BufferGeometryUtils.js';
import { buildAnimIndex } from '../../physics/snapshot.js';
import { createPaths } from '../../physics/paths.js';
import { BALL_R } from '../../physics/constants.js';

const TAU = Math.PI * 2;
const ARM_RATIO = [1.6, 1.0, 0.62, 0.4], GEAR_RATIO = [1, -1.76, 1.48, -2.37, -1.28], STEP = 0.105;
const BARREL_LEN = 0.12;
const num = (v, d) => (Number.isFinite(v) ? v : d);
const wrapDelta = d => { while (d > Math.PI) d -= TAU; while (d < -Math.PI) d += TAU; return d; };

export function buildToys(THREE, kit, layout, opts = {}) {
    const low = opts.low === true, models = opts.models, tier = opts.tier || (low ? 'low' : 'high');
    const reducedNow = () => (typeof opts.isReduced === 'function' ? !!opts.isReduced() : opts.reduced === true);
    const group = new THREE.Group(); group.name = 'observatoryToys';
    const own = { geo: [], mat: [] }, drivers = [];
    const AX = { x: new THREE.Vector3(1, 0, 0), y: new THREE.Vector3(0, 1, 0), z: new THREE.Vector3(0, 0, 1) };
    const geo = g => { own.geo.push(g); return g; };
    const mat = m => { own.mat.push(m); return m; };
    const std = o => mat(new THREE.MeshStandardMaterial({ envMapIntensity: 0.8, ...o }));
    const M = {
        brass: std({ color: '#d9a650', metalness: 0.9, roughness: 0.32 }),
        brassDark: std({ color: '#8a6420', metalness: 0.85, roughness: 0.42 }),
        copper: std({ color: '#c8743a', metalness: 0.85, roughness: 0.38 }),
        mahogany: std({ color: '#4a1a0e', metalness: 0.1, roughness: 0.5 }),
        enamel: std({ color: '#12296b', metalness: 0.2, roughness: 0.3 }),
        steel: std({ color: '#b9c0cc', metalness: 0.95, roughness: 0.3 })
    };
    const toyOf = kind => (layout.toys || []).find(t => t.kind === kind);
    const mesh = (g, m, name, shadow = true) => { const o = new THREE.Mesh(geo(g), m); if (name) o.name = name; o.castShadow = shadow && !low; return o; };
    const place = (root, t) => { root.position.set(t.x, num(t.h, 0), -t.y); root.rotation.y = num(t.rot, 0); const s = num(t.scale, 1); root.scale.setScalar(s); return root; };

    // ---- GLB roots: clone, then merge the static meshes of every unit (root or animated node) by material to save draw calls ----
    const entry = models && models.get && models.get('toys');
    function cloneModel(name, animated) {
        const src = entry && entry.nodes.get(name);
        if (!src) return null;
        const root = src.clone(true);
        root.position.set(0, 0, 0); root.rotation.set(0, 0, 0); root.scale.set(1, 1, 1);
        const named = new Set(animated.map(n => root.getObjectByName(n)).filter(Boolean));
        for (const unit of [root, ...named]) bake(unit, named);
        return root;
    }
    function bake(unit, animated) {
        const list = [], I = new THREE.Matrix4();
        const walk = (node, parentM) => {
            for (const c of node.children) {
                if (animated.has(c)) continue;
                c.updateMatrix();
                const m = new THREE.Matrix4().multiplyMatrices(parentM, c.matrix);
                if (c.isMesh && c.geometry && !Array.isArray(c.material)) list.push({ c, m });
                walk(c, m);
            }
        };
        walk(unit, I);
        const byMat = new Map();
        for (const it of list) { if (!byMat.has(it.c.material)) byMat.set(it.c.material, []); byMat.get(it.c.material).push(it); }
        for (const [material, items] of byMat) {
            const parts = items.map(({ c, m }) => {
                const g = c.geometry.index ? c.geometry.toNonIndexed() : c.geometry.clone();
                for (const k of Object.keys(g.attributes)) if (k !== 'position' && k !== 'normal') g.deleteAttribute(k);
                if (!g.attributes.normal) g.computeVertexNormals();
                return g.applyMatrix4(m);
            });
            const merged = geo(mergeGeometries(parts, false));
            parts.forEach(p => p.dispose());
            for (const { c } of items) if (c.parent) c.parent.remove(c);
            const o = new THREE.Mesh(merged, material); o.name = 'bake_' + (material.name || 'mat'); o.castShadow = !low && material.transparent !== true; o.frustumCulled = false;
            if (material.transparent) o.renderOrder = 2;
            unit.add(o);
        }
    }
    const findMat = (root, name) => { let found = null; root.traverse(o => { if (!found && o.isMesh && o.material && o.material.name === name) found = o.material; }); return found; };
    const base = node => { if (!node.userData.q0) node.userData.q0 = node.quaternion.clone(); return node.userData.q0; };
    const spin = (node, axis, a) => { node.quaternion.copy(base(node)).multiply(new THREE.Quaternion().setFromAxisAngle(AX[axis], a)); };

    // ---- procedural parts ----
    const cyl = (rt, rb, h, seg = 20) => new THREE.CylinderGeometry(rt, rb, h, low ? Math.max(8, seg >> 1) : seg);
    function gearGeometry(r, teeth, thick) {
        const s = new THREE.Shape(), ri = r * 0.84, step = TAU / teeth;
        for (let i = 0; i < teeth; i++) {
            const a = i * step;
            for (const [da, rad] of [[0, ri], [0.12, r], [0.38, r], [0.5, ri]]) { const x = Math.cos(a + da * step) * rad, y = Math.sin(a + da * step) * rad; if (!i && !da) s.moveTo(x, y); else s.lineTo(x, y); }
        }
        s.closePath();
        const hole = new THREE.Path(); hole.absarc(0, 0, r * 0.22, 0, TAU, true); s.holes.push(hole);
        return new THREE.ExtrudeGeometry(s, { depth: thick, bevelEnabled: false }).rotateX(-Math.PI / 2);
    }
    const nodeGroup = (name, parent) => { const g = new THREE.Group(); g.name = name; parent.add(g); return g; };
    const glowMat = (name, color, emissive) => mat(Object.assign(new THREE.MeshStandardMaterial({ color, emissive, emissiveIntensity: 1, roughness: 0.3, toneMapped: false }), { name }));

    function procOrrery() {
        const root = new THREE.Group();
        root.add(mesh(cyl(0.0035, 0.0045, 0.07), M.steel, 'orrery_spindle'));
        const sun = mesh(new THREE.SphereGeometry(0.0075, low ? 10 : 20, low ? 8 : 12).translate(0, 0.07, 0), glowMat('glow_sun', '#ffb84a', '#ff9d2e'), 'orrery_sun', false); root.add(sun);
        const rings = nodeGroup('orrery_rings', root); rings.position.y = 0.07;
        [[0.034, 0], [0.027, 0.9], [0.02, -0.7]].forEach(([r, tilt], i) => {
            const t = mesh(new THREE.TorusGeometry(r, 0.0008, 6, low ? 28 : 56), M.brass); t.rotation.set(Math.PI / 2 + tilt * 0.5, i * 1.1, tilt * 0.4); rings.add(t);
        });
        const cols = ['#d7a85c', '#1e5aa0', '#b8401e', '#c9a060'];
        ARM_RATIO.forEach((_, i) => {
            const arm = nodeGroup('orrery_arm_' + (i + 1), root); arm.position.y = 0.04 + i * 0.0038; arm.rotation.y = i * 1.45;
            const len = [0.014, 0.021, 0.028, 0.034][i];
            arm.add(mesh(cyl(0.0006, 0.0006, len, 6).rotateZ(Math.PI / 2).translate(len / 2, 0, 0), M.brass));
            arm.add(mesh(new THREE.SphereGeometry(0.0055 + i * 0.0004, 14, 10).translate(len, 0.006, 0), std({ color: cols[i], metalness: 0.3, roughness: 0.4 }), 'orrery_planet_' + (i + 1)));
        });
        return root;
    }

    function procTelescope() {
        const root = new THREE.Group();
        root.add(mesh(cyl(0.017, 0.021, 0.03), M.mahogany, 'telescope_pier'), mesh(cyl(0.0215, 0.0215, 0.003).translate(0, 0.0315, 0), M.brass, 'telescope_pier_band'));
        const barrel = nodeGroup('telescope_barrel', root);
        barrel.add(mesh(cyl(0.0215, 0.0215, BARREL_LEN, 24).rotateX(-Math.PI / 2).translate(0, 0, -BARREL_LEN / 2), M.brass, 'telescope_tube'));
        for (const z of [0.012, 0.05, 0.085]) barrel.add(mesh(new THREE.TorusGeometry(0.0222, 0.0016, 6, low ? 20 : 36).translate(0, 0, -z), M.brassDark));
        barrel.add(mesh(new THREE.CylinderGeometry(0.0155, 0.0155, 0.0016, low ? 14 : 28).rotateX(Math.PI / 2).translate(0, 0, -BARREL_LEN - 0.0004), glowMat('glow_star', '#8fe6ff', '#38bfff'), 'telescope_lens_glow', false));
        barrel.add(mesh(cyl(0.005, 0.005, 0.07, 10).rotateX(-Math.PI / 2).translate(0, 0.0265, -0.045), M.steel, 'telescope_finder'));
        return root;
    }

    function procGears() {
        const root = new THREE.Group();
        root.add(mesh(cyl(0.03, 0.033, 0.006), M.mahogany, 'gears_plinth').translateY(0.003));
        const at = [[-0.0112, -0.0028, 0.0098, 0.0085, 18], [0.0108, -0.0094, 0.0098, 0.0075, 22], [0.0126, 0.0101, 0.0133, 0.0075, 16], [-0.001, 0.0168, 0.0133, 0.007, 20], [-0.0206, 0.0151, 0.0171, 0.0055, 14]];
        at.forEach(([x, z, h, r, n], i) => {
            const g = nodeGroup('gear_' + (i + 1), root); g.position.set(x, h, z);
            g.add(mesh(gearGeometry(r, n, 0.0016), i % 2 ? M.copper : M.brass));
            root.add(mesh(cyl(0.0008, 0.0008, h, 6).translate(x, h / 2, z), M.steel));
        });
        return root;
    }

    function procDome() {
        const root = new THREE.Group();
        root.add(mesh(cyl(0.0225, 0.0245, 0.014), M.mahogany, 'dome_drum').translateY(0.007));
        root.add(mesh(new THREE.TorusGeometry(0.0232, 0.0014, 6, low ? 20 : 40).rotateX(Math.PI / 2).translate(0, 0.0144, 0), M.brass, 'dome_cornice'));
        root.add(mesh(new THREE.SphereGeometry(0.0215, low ? 16 : 32, low ? 8 : 16, 0, TAU, 0, Math.PI / 2).translate(0, 0.0144, 0), M.enamel, 'dome_shell'));
        const shutter = nodeGroup('dome_shutter', root);
        shutter.add(mesh(new THREE.SphereGeometry(0.0222, 12, 12, -0.32, 0.64, 0.12, 1.2).translate(0, 0.0144, 0), M.brass, 'dome_shutter_panel'));
        root.add(mesh(new THREE.SphereGeometry(0.0024, 10, 8).translate(0, 0.0372, 0), M.brass, 'dome_finial'));
        const clock = nodeGroup('clock', root); clock.position.set(0, 0.007, 0.0236);
        clock.add(mesh(new THREE.CylinderGeometry(0.0095, 0.0095, 0.0016, 24).rotateX(Math.PI / 2), std({ color: '#efe3c3', roughness: 0.5, metalness: 0.1 }), 'clock_face'));
        clock.add(mesh(new THREE.TorusGeometry(0.0098, 0.0011, 6, 24), M.brass, 'clock_bezel'));
        for (const [n, len] of [['clock_hand_hour', 0.0055], ['clock_hand_minute', 0.0082]]) {
            const hand = nodeGroup(n, clock); hand.position.z = n.endsWith('hour') ? 0.0017 : 0.002;
            hand.add(mesh(new THREE.BoxGeometry(0.0009, len, 0.0006).translate(0, len / 2 - 0.0008, 0), M.mahogany, n + '_blade', false));
        }
        return root;
    }

    // ---- toys ----
    const idx = buildAnimIndex(layout), discSlot = idx.has('orrery') ? idx.get('orrery') : -1;
    let tubeSamples = null;
    try { const p = createPaths(layout.paths || [], { layout }).debug().find(d => d.id === 'telescopeTube'); if (p) tubeSamples = p.samples; } catch (_) { /* no tube: the barrel keeps its model pose */ }
    const sampleAt = f => {
        const s = tubeSamples, cum = [0];
        for (let i = 1; i < s.length; i++) cum.push(cum[i - 1] + Math.hypot(s[i][0] - s[i - 1][0], s[i][1] - s[i - 1][1], s[i][2] - s[i - 1][2]));
        const want = Math.max(0, Math.min(1, f)) * cum[cum.length - 1];
        let i = 1; while (i < s.length - 1 && cum[i] < want) i++;
        const k = (want - cum[i - 1]) / Math.max(1e-9, cum[i] - cum[i - 1]);
        return new THREE.Vector3(s[i - 1][0] + (s[i][0] - s[i - 1][0]) * k, s[i - 1][2] + (s[i][2] - s[i - 1][2]) * k + BALL_R, -(s[i - 1][1] + (s[i][1] - s[i - 1][1]) * k));
    };

    // armillary on the disc hub
    const tOrr = toyOf('orrery');
    if (tOrr) {
        const names = ['orrery_rings', 'orrery_arm_1', 'orrery_arm_2', 'orrery_arm_3', 'orrery_arm_4'];
        const root = place(cloneModel('orrery', names) || procOrrery(), tOrr); root.name = 'orrery'; group.add(root);
        const rings = root.getObjectByName('orrery_rings'), arms = ARM_RATIO.map((_, i) => root.getObjectByName('orrery_arm_' + (i + 1))), sun = findMat(root, 'glow_sun');
        let last = null, shown = 0, ringAngle = 0;
        drivers.push((dt, snap, st) => {
            const reduced = reducedNow(), ts = (st && st.orrery) || {}, speed = num(st && st.disc && st.disc.speed, num(ts.speed, 0));
            if (snap && snap.anim && discSlot >= 0 && discSlot < snap.anim.length) {
                const a = snap.anim[discSlot];
                if (last !== null) { let d = wrapDelta(a - last); if (reduced) d = Math.max(-0.2 * dt, Math.min(0.2 * dt, d)); shown += d; }
                last = a;
            }
            if (!reduced) ringAngle += 0.3 * Math.sign(speed) * dt;
            if (rings) spin(rings, 'y', ringAngle);
            arms.forEach((arm, i) => arm && spin(arm, 'y', shown * ARM_RATIO[i]));
            if (sun) sun.emissiveIntensity = 0.3 + 1.0 * num(ts.glow, 0.3);
        });
    }

    // telescope barrel along the tube
    const tScope = toyOf('telescope');
    if (tScope) {
        const root = place(cloneModel('telescope', ['telescope_barrel']) || procTelescope(), tScope); root.name = 'telescope'; group.add(root);
        const barrel = root.getObjectByName('telescope_barrel'), lens = findMat(root, 'glow_star');
        if (barrel && tubeSamples) {
            const p0 = sampleAt(num(tScope.from, 0.10)), p1 = sampleAt(num(tScope.to, 0.66)), inv = -num(tScope.rot, 0);
            const local = v => v.clone().sub(root.position).applyAxisAngle(AX.y, inv).divideScalar(root.scale.x || 1);
            const a = local(p0), d = local(p1).sub(a).normalize(), m = new THREE.Matrix4().lookAt(new THREE.Vector3(), d, AX.y);
            barrel.position.copy(a); barrel.quaternion.setFromRotationMatrix(m); barrel.userData.q0 = barrel.quaternion.clone();
        }
        let level = 0;
        drivers.push((dt, snap, st) => {
            const ts = (st && st.telescope) || {}, target = num(ts.glow, 0.25) + 0.25 * num(ts.cap, 0);
            level = reducedNow() ? target : level + (target - level) * (1 - Math.exp(-6 * dt));
            if (lens) lens.emissiveIntensity = 0.5 + 2.2 * level;
        });
    }

    // gear train: an escapement ticks the gears in 0.105 rad steps
    const tGears = toyOf('gears');
    if (tGears) {
        const names = GEAR_RATIO.map((_, i) => 'gear_' + (i + 1));
        const root = place(cloneModel('gears', names) || procGears(), tGears); root.name = 'gears'; group.add(root);
        const gears = names.map(n => root.getObjectByName(n)), anchor = root.getObjectByName('gear_anchor');
        let steps = 0, shownSteps = 0, clock = 0, lastHour = null, rock = 0;
        drivers.push((dt, snap, st) => {
            const gs = (st && st.gears) || {}, run = num(gs.run, 0.25), hour = num(gs.hour, 0);
            clock += dt;
            const period = 2.4 - 1.8 * Math.max(0, Math.min(1, run));
            if (clock >= period) { clock = 0; steps++; }
            if (lastHour !== null && hour !== lastHour) steps += 3;
            lastHour = hour;
            shownSteps = reducedNow() ? steps : shownSteps + (steps - shownSteps) * (1 - Math.exp(-dt / 0.04));
            gears.forEach((g, i) => g && spin(g, 'y', shownSteps * STEP * GEAR_RATIO[i]));
            if (anchor) { rock = (steps % 2 ? 0.25 : -0.25); spin(anchor, 'y', rock * Math.min(1, 1 - Math.abs(steps - shownSteps))); }
        });
    }

    // dome: shutter opens in multiball and wizard, clock hands show the hour
    const tDome = toyOf('dome');
    if (tDome) {
        const names = ['dome_shutter', 'clock_hand_hour', 'clock_hand_minute'];
        const root = place(cloneModel('dome', names) || procDome(), tDome); root.name = 'dome'; group.add(root);
        const shutter = root.getObjectByName('dome_shutter'), hh = root.getObjectByName('clock_hand_hour'), mh = root.getObjectByName('clock_hand_minute');
        let open = 0, ah = 0, am = 0;
        drivers.push((dt, snap, st) => {
            const ds = (st && st.dome) || {}, hour = num(st && st.clock && st.clock.hour, 0), reduced = reducedNow();
            const ease = (v, tgt, rate) => (reduced ? tgt : v + (tgt - v) * (1 - Math.exp(-rate * dt)));
            open = ease(open, 0.5 * Math.max(0, Math.min(1, num(ds.open, 0))), 3);
            ah = ease(ah, -(hour % 12) / 12 * TAU, 5); am = ease(am, -hour * TAU, 5);
            if (shutter) spin(shutter, 'y', open);
            if (hh) spin(hh, 'z', ah);
            if (mh) spin(mh, 'z', am);
        });
    }

    // clock bezel: a brass ring flush with the playfield round the disc (inside the clock lamps at r 0.066)
    const tClock = toyOf('clockDial');
    if (tClock) {
        const ring = mesh(new THREE.TorusGeometry(0.0545, 0.0013, 8, low ? 48 : 96).rotateX(Math.PI / 2).translate(0, 0.0012, 0), M.brass, 'clockBezel', false);
        ring.position.set(tClock.x, num(tClock.h, 0), -tClock.y); group.add(ring);
        const tick = new THREE.Group();
        const mark = new THREE.BoxGeometry(0.0012, 0.0006, 0.0034), markMat = M.brassDark;
        for (let i = 0; i < 12; i++) {
            const a = i * TAU / 12, o = mesh(mark.clone(), markMat, null, false);
            o.position.set(Math.sin(a) * 0.0585, 0.0008, -Math.cos(a) * 0.0585); o.rotation.y = -a; tick.add(o);
        }
        mark.dispose(); tick.position.set(tClock.x, num(tClock.h, 0), -tClock.y); group.add(tick);
    }

    let t0 = null;
    return {
        group,
        update(dt, t, snap, toyState) {
            const step = Math.min(0.1, Math.max(0, num(dt, 0)));
            t0 = t;
            for (const d of drivers) d(step, snap, toyState || null);
        },
        dispose() {
            if (group.parent) group.parent.remove(group);
            own.geo.forEach(g => g.dispose()); own.mat.forEach(m => m.dispose());
            own.geo.length = own.mat.length = 0; drivers.length = 0; void t0; void tier;
        }
    };
}
export default buildToys;
