// Render-facing physics snapshot (plan 3.6): write, anim slot index, interpolation. DOM-free.
//   { tick, time, balls: [{id, x, y, h, vx, vy, mode, hidden}], flippers: Float64Array(2n) [angle, omega],
//     plunger, anim: Float32Array, spin: Int32Array }
// `spin` lists the anim slots that hold spinner angles (rad), so lerpSnapshot can take the shortest delta.
import { STEP } from './constants.js';

// One slot per device (layout order) plus one per drop target, keyed by target id. The returned Map
// carries `spin` (slot indices of spinners).
export function buildAnimIndex(layout) {
    const index = new Map(), spin = [];
    const slot = id => { if (!index.has(id)) index.set(id, index.size); return index.get(id); };
    for (const d of layout.devices || []) {
        const i = slot(d.id);
        if (d.type === 'spinner') spin.push(i);
        if (d.type === 'dropBank') for (const t of d.targets || []) slot(t.id);
    }
    index.spin = spin;
    return index;
}

export function createSnapshot(flipperCount, animCount, spin) {
    return {
        tick: 0, time: 0, balls: [], flippers: new Float64Array(flipperCount * 2), plunger: 0,
        anim: new Float32Array(animCount), spin: Int32Array.from(spin || [])
    };
}

function ballEntry() { return { id: 0, x: 0, y: 0, h: 0, vx: 0, vy: 0, mode: 'field', hidden: false }; }

// Fills `out` (or a fresh snapshot) from live state. `anim` is cleared and then written by `writeAnim`.
export function writeSnapshot(out, { tick, balls, flippers, plunger, animIndex, writeAnim }) {
    if (!out || out.flippers.length !== flippers.length * 2 || out.anim.length !== animIndex.size) {
        out = createSnapshot(flippers.length, animIndex.size, animIndex.spin);
    }
    out.tick = tick; out.time = tick * STEP; out.plunger = plunger;
    while (out.balls.length > balls.length) out.balls.pop();
    while (out.balls.length < balls.length) out.balls.push(ballEntry());
    for (let i = 0; i < balls.length; i++) {
        const b = balls[i], e = out.balls[i];
        e.id = b.id; e.x = b.x; e.y = b.y; e.h = b.h; e.vx = b.vx; e.vy = b.vy; e.mode = b.mode; e.hidden = !!b.hidden;
    }
    for (let i = 0; i < flippers.length; i++) { out.flippers[2 * i] = flippers[i].a; out.flippers[2 * i + 1] = flippers[i].w; }
    out.anim.fill(0);
    if (writeAnim) writeAnim(out.anim);
    return out;
}

const TWO_PI = Math.PI * 2;
const SNAP_DISTANCE = 0.08; // a ball further than this between two snapshots was teleported: do not blend

// Blend two snapshots (t = 0 -> a, 1 -> b) into `out`. Balls are matched by id; spinner angles take the shortest delta.
export function lerpSnapshot(a, b, t, out) {
    if (!out || out.flippers.length !== b.flippers.length || out.anim.length !== b.anim.length) {
        out = createSnapshot(b.flippers.length / 2, b.anim.length, b.spin);
    }
    out.tick = b.tick; out.time = a.time + (b.time - a.time) * t;
    out.plunger = a.plunger + (b.plunger - a.plunger) * t;
    while (out.balls.length > b.balls.length) out.balls.pop();
    while (out.balls.length < b.balls.length) out.balls.push(ballEntry());
    for (let i = 0; i < b.balls.length; i++) {
        const nb = b.balls[i], e = out.balls[i];
        let pb = null;
        for (let j = 0; j < a.balls.length; j++) if (a.balls[j].id === nb.id) { pb = a.balls[j]; break; }
        const p = pb || nb;
        const k = pb && Math.hypot(nb.x - pb.x, nb.y - pb.y) > SNAP_DISTANCE ? 1 : t;
        e.id = nb.id; e.mode = nb.mode; e.hidden = nb.hidden;
        e.x = p.x + (nb.x - p.x) * k; e.y = p.y + (nb.y - p.y) * k; e.h = p.h + (nb.h - p.h) * k;
        e.vx = p.vx + (nb.vx - p.vx) * k; e.vy = p.vy + (nb.vy - p.vy) * k;
    }
    for (let i = 0; i < b.flippers.length; i++) out.flippers[i] = a.flippers[i] + (b.flippers[i] - a.flippers[i]) * t;
    for (let i = 0; i < b.anim.length; i++) out.anim[i] = a.anim[i] + (b.anim[i] - a.anim[i]) * t;
    const spin = b.spin || [];
    for (let s = 0; s < spin.length; s++) {
        const i = spin[s];
        let d = (b.anim[i] - a.anim[i]) % TWO_PI;
        if (d > Math.PI) d -= TWO_PI; else if (d < -Math.PI) d += TWO_PI;
        out.anim[i] = a.anim[i] + d * t;
    }
    out.spin = b.spin;
    return out;
}
