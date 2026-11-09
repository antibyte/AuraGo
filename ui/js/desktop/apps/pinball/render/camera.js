// AuraPinball cameras (plan section 7): play / top / showcase / attract, pure fit maths and the shake spring.
// fitDistance / fitCamera / projectCorners are pure (plain arrays) so Node tests can import them.
import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';

const TILT = 6.5 * Math.PI / 180;
const sub = (a, b) => [a[0] - b[0], a[1] - b[1], a[2] - b[2]];
const dot = (a, b) => a[0] * b[0] + a[1] * b[1] + a[2] * b[2];
const cross = (a, b) => [a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0]];
const norm = a => { const l = Math.hypot(a[0], a[1], a[2]) || 1; return [a[0] / l, a[1] / l, a[2] / l]; };

// cam = {fov (deg, vertical), target [x,y,z], dir [x,y,z] target->camera, up [x,y,z]}. Returns NDC extents of the corners
// when the camera sits `d` metres from the target, looking at it.
export function projectCorners(cam, corners, aspect, d) {
    const dir = norm(cam.dir), fwd = [-dir[0], -dir[1], -dir[2]], right = norm(cross(fwd, cam.up)), up = cross(right, fwd);
    const pos = [cam.target[0] + dir[0] * d, cam.target[1] + dir[1] * d, cam.target[2] + dir[2] * d];
    const t = Math.tan(cam.fov * Math.PI / 360);
    let minX = Infinity, maxX = -Infinity, minY = Infinity, maxY = -Infinity, ok = true;
    const ndc = [];
    for (const c of corners) {
        const v = sub(c, pos), z = dot(v, fwd);
        if (z < 0.02) { ok = false; continue; }
        const x = dot(v, right) / (z * t * aspect), y = dot(v, up) / (z * t);
        ndc.push([x, y]);
        minX = Math.min(minX, x); maxX = Math.max(maxX, x); minY = Math.min(minY, y); maxY = Math.max(maxY, y);
    }
    return { minX, maxX, minY, maxY, ok, ndc };
}

// Smallest distance that fits all corners inside the viewport minus margin and insets (all fractions of the viewport
// size), plus the NDC lens shift that centres the content inside the free window.
export function fitCamera(cam, corners, aspect, insets = {}, margin = 0.03) {
    const m = 2 * margin, lo = [-1 + m + 2 * (insets.left || 0), -1 + m + 2 * (insets.bottom || 0)];
    const hi = [1 - m - 2 * (insets.right || 0), 1 - m - 2 * (insets.top || 0)];
    const fits = d => {
        const p = projectCorners(cam, corners, aspect, d);
        return p.ok && p.maxX - p.minX <= hi[0] - lo[0] && p.maxY - p.minY <= hi[1] - lo[1];
    };
    let a = 0.05, b = 20;
    for (let i = 0; i < 48; i++) { const mid = (a + b) / 2; if (fits(mid)) b = mid; else a = mid; }
    const p = projectCorners(cam, corners, aspect, b);
    return { distance: b, shiftX: (lo[0] + hi[0]) / 2 - (p.minX + p.maxX) / 2, shiftY: (lo[1] + hi[1]) / 2 - (p.minY + p.maxY) / 2 };
}
export const fitDistance = (cam, corners, aspect, insets, margin) => fitCamera(cam, corners, aspect, insets, margin).distance;

export const FIELD_CORNERS_LOCAL = (() => {
    const out = [];
    for (const x of [-0.2975, 0.2975]) for (const y of [-0.12, 1.07]) for (const h of [0, 0.075]) out.push([x, h, -y]);
    return out;
})();

// Player view: the lockdown bar edge to the top-arc toys (no backbox), so the field fills the free height.
export const PLAY_CORNERS_LOCAL = (() => {
    const out = [];
    for (const x of [-0.2975, 0.2975]) for (const y of [-0.03, 1.07]) for (const h of [0, 0.09]) out.push([x, h, -y]);
    return out;
})();
const PLAY_ELEVATION = 49 * Math.PI / 180;

export function createCameraRig({ camera, domElement, tableRoot, reduced = false }) {
    tableRoot.updateMatrixWorld(true);
    const toWorld = p => { const v = new THREE.Vector3(p[0], p[1], p[2]).applyMatrix4(tableRoot.matrixWorld); return [v.x, v.y, v.z]; };
    const rot = new THREE.Matrix4().makeRotationX(TILT);
    const dirWorld = p => { const v = new THREE.Vector3(p[0], p[1], p[2]).applyMatrix4(rot); return [v.x, v.y, v.z]; };
    const corners = FIELD_CORNERS_LOCAL.map(toWorld), playCorners = PLAY_CORNERS_LOCAL.map(toWorld);
    // Showcase frames the whole machine: backbox top and the legs down to the floor.
    const machine = corners.concat([[-0.3, 0.64, -1.12], [0.3, 0.64, -1.12], [-0.28, -0.8, 0.1], [0.28, -0.8, 0.1], [-0.28, -0.8, -1.05], [0.28, -0.8, -1.05]].map(toWorld));
    const controls = new OrbitControls(camera, domElement);
    controls.enableDamping = true; controls.dampingFactor = 0.08; controls.minDistance = 0.55; controls.maxDistance = 3.2;
    controls.maxPolarAngle = Math.PI * 0.495; controls.enablePan = false; controls.enabled = false; controls.autoRotateSpeed = 0.9;

    const ATTRACT = { fov: 36, target: toWorld([0, 0.02, -0.46]), dir: dirWorld([0, Math.sin(52 * Math.PI / 180), Math.cos(52 * Math.PI / 180)]), up: [0, 1, 0] };
    const PLAY = { fov: 36, target: toWorld([0, 0.02, -0.5]), dir: dirWorld([0, Math.sin(PLAY_ELEVATION), Math.cos(PLAY_ELEVATION)]), up: [0, 1, 0] };
    const TOP = { fov: 30, target: toWorld([0, 0, -0.5]), dir: dirWorld([0, 1, 0]), up: dirWorld([0, 0, -1]) };
    const SHOW = { fov: 36, target: toWorld([0, -0.05, -0.55]), dir: norm([-0.55, 0.5, 0.85]), up: [0, 1, 0] };
    const state = { mode: 'play', width: 1, height: 1, insetTop: 0, idle: 0, base: PLAY, dist: 1, shiftX: 0, shiftY: 0, swing: 0 };
    const shake = { x: 0, y: 0, vx: 0, vy: 0 };
    let lastInteraction = 0, time = 0;
    controls.addEventListener('start', () => { lastInteraction = time; controls.autoRotate = false; });
    controls.addEventListener('end', () => { lastInteraction = time; });

    function place(yaw = 0) {
        const b = state.base, d = norm(b.dir), c = Math.cos(yaw), s = Math.sin(yaw);
        const dir = [d[0] * c + d[2] * s, d[1], -d[0] * s + d[2] * c];
        camera.up.set(b.up[0], b.up[1], b.up[2]);
        camera.position.set(b.target[0] + dir[0] * state.dist, b.target[1] + dir[1] * state.dist, b.target[2] + dir[2] * state.dist);
        camera.lookAt(b.target[0], b.target[1], b.target[2]);
    }
    function applyShift(on) {
        if (!on) { camera.clearViewOffset(); return; }
        camera.setViewOffset(state.width, state.height, -state.shiftX * state.width / 2, state.shiftY * state.height / 2, state.width, state.height);
    }
    function refit() {
        const aspect = state.width / state.height, top = state.insetTop / state.height;
        camera.aspect = aspect;
        const m = state.mode;
        if (m === 'closeup') { camera.fov = state.base.fov; camera.updateProjectionMatrix(); applyShift(false); return; }
        if (m === 'showcase') {
            state.base = SHOW; camera.fov = SHOW.fov; camera.updateProjectionMatrix();
            state.dist = fitDistance(SHOW, machine, aspect, {}, 0.06); applyShift(false);
            return;
        }
        state.base = m === 'top' ? TOP : m === 'attract' ? ATTRACT : PLAY; camera.fov = state.base.fov;
        const margin = m === 'attract' ? 0.07 : m === 'play' ? 0.015 : 0.03;
        let fit = fitCamera(state.base, m === 'play' ? playCorners : corners, aspect, { top }, margin);
        if (m === 'attract') for (const yaw of [-0.21, 0.21]) {
            const b = state.base, d = b.dir, c = Math.cos(yaw), s = Math.sin(yaw);
            const alt = { ...b, dir: [d[0] * c + d[2] * s, d[1], -d[0] * s + d[2] * c] };
            const f2 = fitCamera(alt, corners, aspect, { top }, margin);
            if (f2.distance > fit.distance) fit = f2;
        }
        state.dist = fit.distance; state.shiftX = fit.shiftX; state.shiftY = fit.shiftY;
        camera.updateProjectionMatrix(); applyShift(true);
    }
    function setMode(mode) {
        state.mode = ['play', 'top', 'showcase', 'attract'].includes(mode) ? mode : 'play';
        controls.enabled = state.mode === 'showcase'; controls.autoRotate = false; lastInteraction = time;
        refit();
        place(0);
        if (state.mode === 'showcase') { controls.target.set(...SHOW.target); controls.update(); }
    }
    // Dev/survey helper: low close-up of a playfield spot (metres, h above the field).
    function closeup(x, y, h = 0) {
        state.mode = 'closeup'; controls.enabled = false; controls.autoRotate = false;
        state.base = { fov: 30, target: toWorld([x, h, -y]), dir: dirWorld(norm([0, 0.75, 0.66])), up: [0, 1, 0] };
        state.dist = 0.42; refit(); place(0);
    }
    function resize(w, h) { state.width = Math.max(1, w); state.height = Math.max(1, h); refit(); if (state.mode !== 'showcase' && state.mode !== 'closeup') place(state.swing); }
    function setInsets(i) { state.insetTop = Math.max(0, Math.min(state.height * 0.45, (i && i.top) || 0)); refit(); if (state.mode !== 'showcase') place(state.swing); }

    // Spring k 900, damping 30; amplitude in metres along the camera's right/up axes.
    function addShake(dx, dy, strength) {
        if (reduced) return;
        const l = Math.hypot(dx, dy) || 1;
        shake.x += (dx / l) * strength; shake.y += (dy / l) * strength;
    }
    function update(dt, t, { reducedNow = false, interactive = true } = {}) {
        time = t;
        reduced = reducedNow;
        if (state.mode === 'showcase') {
            controls.enabled = interactive;
            controls.autoRotate = !reduced && t - lastInteraction > 4;
            controls.update();
            return;
        }
        state.swing = state.mode === 'attract' && !reduced ? 0.21 * Math.sin(t * 0.26) : 0;
        place(state.swing);
        if (reduced) { shake.x = shake.y = shake.vx = shake.vy = 0; return; }
        if (Math.abs(shake.x) + Math.abs(shake.y) + Math.abs(shake.vx) + Math.abs(shake.vy) > 1e-6) {
            const h = Math.min(dt, 0.03);
            shake.vx += (-900 * shake.x - 30 * shake.vx) * h; shake.vy += (-900 * shake.y - 30 * shake.vy) * h;
            shake.x += shake.vx * h; shake.y += shake.vy * h;
            const r = new THREE.Vector3().setFromMatrixColumn(camera.matrixWorld, 0), u = new THREE.Vector3().setFromMatrixColumn(camera.matrixWorld, 1);
            camera.position.addScaledVector(r, shake.x).addScaledVector(u, shake.y);
        }
    }
    return { setMode, closeup, resize, setInsets, shake: addShake, update, controls, get mode() { return state.mode; }, get state() { return state; }, dispose() { controls.dispose(); } };
}
