// AuraPinball lighting: moody room environment (PMREM), hemisphere + key + rim lights, fixed flasher point lights,
// flasher domes and GI bulbs (instanced emissive meshes). The light count never changes at runtime.
import * as THREE from 'three';
import { hexToLinear } from './playfield.js';

const BALL_R = 0.0135;

export function createEnvironment(renderer, palette) {
    const s = new THREE.Scene();
    const geo = new THREE.SphereGeometry(12, 32, 16), col = [], p = geo.attributes.position;
    for (let i = 0; i < p.count; i++) {
        const t = (p.getY(i) / 12 + 1) / 2;                                                // 0 bottom .. 1 top
        const k = t > 0.5 ? 0.012 + (t - 0.5) * 0.06 : 0.003 + t * 0.018;
        col.push(k * 0.8, k * 0.95, k * 1.5);
    }
    geo.setAttribute('color', new THREE.Float32BufferAttribute(col, 3));
    s.add(new THREE.Mesh(geo, new THREE.MeshBasicMaterial({ vertexColors: true, side: THREE.BackSide })));
    const panel = (w, h, pos, rot, color, k) => {
        const m = new THREE.Mesh(new THREE.PlaneGeometry(w, h), new THREE.MeshBasicMaterial({ color: new THREE.Color(color).multiplyScalar(k), side: THREE.DoubleSide }));
        m.position.set(...pos); m.rotation.set(...rot); s.add(m);
    };
    const acc = palette.accent || '#37d9ff', sec = palette.secondary || '#ff48c4';
    panel(9, 1.4, [0, 5.6, -4.6], [0, 0, 0], '#e8f1ff', 1.3);                   // back-high soft box (glossy sheen in the play view)
    panel(8, 1.1, [0, 5.4, 4.8], [0, 0, 0], '#fff0dc', 1.6);
    panel(0.5, 5, [-6, 2.6, 0], [0, Math.PI / 2, 0], acc, 3);                  // neon strips on both sides
    panel(0.5, 5, [6, 2.6, 0], [0, Math.PI / 2, 0], sec, 3);
    panel(6, 0.4, [0, 1.6, -7], [0, 0, 0], acc, 3);
    panel(4, 0.3, [0, 1.2, 6.8], [0, Math.PI, 0], sec, 2.2);
    s.children.forEach(o => { if (o.isMesh && o.geometry.type === 'PlaneGeometry') o.lookAt(0, 0.5, 0); });
    const pm = new THREE.PMREMGenerator(renderer);
    const target = pm.fromScene(s, 0.03);
    pm.dispose();
    s.traverse(o => { if (o.geometry) o.geometry.dispose(); if (o.material) o.material.dispose(); });
    return target;
}

function autoBulbs(layout, group) {
    const out = [];
    const near = (x, y) => out.some(b => Math.hypot(b[0] - x, b[1] - y) < 0.05);
    for (const w of layout.walls || []) {
        if (!/^(outer|divider|inlaneGuide)/.test(w.id) || !w.points) continue;
        for (let i = 0; i < w.points.length - 1; i++) {
            const a = w.points[i], b = w.points[i + 1], len = Math.hypot(b[0] - a[0], b[1] - a[1]), n = Math.floor(len / 0.075);
            for (let k = 0; k <= n; k++) {
                const t = n ? k / n : 0.5, x = a[0] + (b[0] - a[0]) * t, y = a[1] + (b[1] - a[1]) * t;
                const inward = [Math.sign(0 - x) * 0.016, 0];
                const bx = /^outer/.test(w.id) ? x + inward[0] : x, by = y;
                if (by < group.yMin - 0.01 || by > group.yMax + 0.01 || bx > 0.215 || near(bx, by)) continue;
                out.push([bx, by]);
            }
        }
    }
    return out.slice(0, 40);
}

export function createLights({ scene, tableRoot, layout, tier, palette, shadows }) {
    const accent = palette.accent || '#37d9ff', secondary = palette.secondary || '#ff48c4';
    const hemi = new THREE.HemisphereLight('#9db6dc', '#0b0d14', 0.5);
    const key = new THREE.DirectionalLight('#fff4e6', 1.1);
    key.position.set(-0.55, 1.9, 0.35);
    const target = new THREE.Object3D(); target.position.set(0, 0.05, -0.5); scene.add(target); key.target = target;
    const rim = new THREE.DirectionalLight(secondary, 0.55); rim.position.set(0.7, 0.6, -1.6); rim.target = target;
    const fill = new THREE.DirectionalLight(accent, 0.28); fill.position.set(0.9, 0.7, 1.2); fill.target = target;
    scene.add(hemi, key, rim, fill);
    const cam = key.shadow.camera;
    cam.left = -0.5; cam.right = 0.5; cam.top = 0.9; cam.bottom = -0.9; cam.near = 0.5; cam.far = 4.5;
    key.shadow.bias = -0.00015; key.shadow.normalBias = 0.0015;

    const flashers = layout.flashers || [], giDefs = layout.gi || [];
    const points = [];
    function setLightCount(n) {
        while (points.length > n) { const l = points.pop(); tableRoot.remove(l); l.dispose && l.dispose(); }
        while (points.length < n) { const l = new THREE.PointLight('#ffffff', 0, 0.6, 1.5); l.position.set(0, 0.06, -0.5); tableRoot.add(l); points.push(l); }
    }
    setLightCount(tier.lights);

    // Flasher domes (instanced half spheres).
    let domes = null;
    if (flashers.length) {
        const g = new THREE.SphereGeometry(1, 16, 8, 0, Math.PI * 2, 0, Math.PI / 2);
        domes = new THREE.InstancedMesh(g, new THREE.MeshBasicMaterial({ color: '#ffffff' }), flashers.length);
        domes.frustumCulled = false;
        flashers.forEach((f, i) => domes.setMatrixAt(i, new THREE.Matrix4().compose(new THREE.Vector3(f.x, 0.001, -f.y), new THREE.Quaternion(), new THREE.Vector3().setScalar(f.r || 0.012))));
        tableRoot.add(domes);
    }
    // GI bulbs.
    const bulbs = [];
    for (let k = 0; k < giDefs.length; k++) {
        const list = giDefs[k].bulbs && giDefs[k].bulbs.length ? giDefs[k].bulbs : [];
        for (const b of list) bulbs.push({ x: b[0], y: b[1], gi: k });
    }
    if (!bulbs.length) giDefs.forEach((gd, k) => { for (const b of autoBulbs(layout, gd)) bulbs.push({ x: b[0], y: b[1], gi: k }); });
    let bulbMesh = null;
    if (bulbs.length) {
        bulbMesh = new THREE.InstancedMesh(new THREE.SphereGeometry(0.0027, 10, 6, 0, Math.PI * 2, 0, Math.PI / 2), new THREE.MeshBasicMaterial({ color: '#ffffff' }), bulbs.length);
        bulbMesh.frustumCulled = false;
        bulbs.forEach((b, i) => bulbMesh.setMatrixAt(i, new THREE.Matrix4().makeTranslation(b.x, 0.001, -b.y)));
        tableRoot.add(bulbMesh);
    }
    const flashColor = flashers.map(f => hexToLinear(f.color || '#ffffff'));
    const giColor = giDefs.map(g => hexToLinear(g.color || '#ffffff'));
    const tmp = new THREE.Color();
    let lastGi = '';

    function update(lamps, reduced) {
        const fl = lamps && lamps.flashLevel, gl = lamps && lamps.giLevel;
        const giAvg = gl && gl.length ? gl.reduce((a, v) => a + v, 0) / gl.length : 1;
        hemi.intensity = 0.25 + 0.65 * giAvg; rim.intensity = 0.2 + 0.4 * giAvg; fill.intensity = 0.08 + 0.22 * giAvg;
        const n = points.length;
        for (let p = 0; p < n; p++) {
            let best = -1, level = 0;
            for (let j = p; j < flashers.length; j += n) { const v = fl ? fl[j] || 0 : 0; if (v >= level) { level = v; best = j; } }
            const light = points[p];
            if (best >= 0) { const f = flashers[best]; light.position.set(f.x, 0.06, -f.y); light.color.copy(flashColor[best]); }
            light.intensity = level * (reduced ? 0.7 : 1) * 0.55;
        }
        if (domes) for (let j = 0; j < flashers.length; j++) {
            const v = fl ? fl[j] || 0 : 0;
            tmp.copy(flashColor[j]).multiplyScalar(0.12 + 5 * v); domes.setColorAt(j, tmp);
        }
        if (domes) domes.instanceColor.needsUpdate = true;
        if (bulbMesh) {
            const sig = gl ? Array.from(gl, v => v.toFixed(2)).join() : '';
            if (sig !== lastGi || !bulbMesh.instanceColor) {
                lastGi = sig;
                bulbs.forEach((b, i) => { const lv = gl ? gl[b.gi] : 1; tmp.copy(giColor[b.gi]).multiplyScalar(0.04 + 0.5 * lv); bulbMesh.setColorAt(i, tmp); });
                bulbMesh.instanceColor.needsUpdate = true;
            }
        }
    }
    function setTier(next, shadowsOn) {
        setLightCount(next.lights);
        key.castShadow = !!shadowsOn && next.shadow > 0;
        if (key.castShadow) { key.shadow.mapSize.set(next.shadow, next.shadow); if (key.shadow.map) { key.shadow.map.dispose(); key.shadow.map = null; } }
    }
    setTier(tier, shadows);
    update(null, false);
    return {
        update, setTier, key, hemi, points, domes, bulbMesh, bulbCount: bulbs.length,
        dispose() {
            points.forEach(l => tableRoot.remove(l));
            if (key.shadow.map) key.shadow.map.dispose();
            for (const m of [domes, bulbMesh]) if (m) { tableRoot.remove(m); m.geometry.dispose(); m.material.dispose(); m.dispose(); }
            scene.remove(hemi, key, rim, fill, target);
        }
    };
}

export const BALL_RADIUS = BALL_R;
