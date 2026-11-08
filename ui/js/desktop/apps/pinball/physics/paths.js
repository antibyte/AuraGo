// Guided 3D paths (ramps, wire habitrails, VUK tubes). DOM-free, deterministic. Plan 3.3, 3.7, 4 "Paths".
//   createPaths(defs, ctx) -> PathSystem {substep(h), command(cmd) -> bool, reset(), position(ball) -> [x, y, h], debug()}
//
// Model: points [x, y, h] are sampled as a centripetal Catmull-Rom spline (200 segments, 201 samples) with a 3D
// arc-length table. A ball that crosses the entry segment forward becomes mode 'path' with ball.path = {id, s, v, ...}
// and moves in 1D: a = -G_SLOPE*t_y - G_NORMAL*t_h - decel*sign(v). The world skips non-field balls in integration and
// collisions, so a path ball is owned entirely by this module.
//
// ctx hooks (T03 side is optional at runtime, both are only called while stepping):
//   PROVIDED  ctx.launchPath(ball, pathId, speed) -> boolean   put `ball` at s=0 of that path with `speed` (m/s along the
//                                                              path), mode 'path', emits pathEnter. For VUK/scoop ejects.
//   EXPECTED  ctx.deliverToHole(ball, holeId) -> boolean        capture a ball that arrives from a path (the hole emits
//                                                              `hole {via:'path'}`); false = disabled/full -> the ball is
//                                                              dropped on the field moving along the final tangent.
//   EXPECTED  ctx.diverterOn(diverterId) -> boolean             read at arrival for 'switch' exits; absent -> path-level
//                                                              state set by the `diverter` command (defaults from layout).
// position(ball) returns a shared scratch array: copy it if you keep it.
import { BALL_R, G_SLOPE, G_NORMAL, MAX_SPEED, ROLL_DECEL } from './constants.js';

const SEGMENTS = 200;
const SEP = 2 * BALL_R;
const EPS = 1e-9;

function dist3(a, b) { return Math.hypot(a[0] - b[0], a[1] - b[1], a[2] - b[2]); }

// Barry-Goldman evaluation of the centripetal Catmull-Rom segment P1..P2 at u in [0,1].
function crPoint(p0, p1, p2, p3, u, out) {
    const t0 = 0;
    const t1 = t0 + Math.sqrt(Math.max(dist3(p0, p1), 1e-6));
    const t2 = t1 + Math.sqrt(Math.max(dist3(p1, p2), 1e-6));
    const t3 = t2 + Math.sqrt(Math.max(dist3(p2, p3), 1e-6));
    const t = t1 + (t2 - t1) * u;
    for (let k = 0; k < 3; k++) {
        const a1 = (t1 - t) / (t1 - t0) * p0[k] + (t - t0) / (t1 - t0) * p1[k];
        const a2 = (t2 - t) / (t2 - t1) * p1[k] + (t - t1) / (t2 - t1) * p2[k];
        const a3 = (t3 - t) / (t3 - t2) * p2[k] + (t - t2) / (t3 - t2) * p3[k];
        const b1 = (t2 - t) / (t2 - t0) * a1 + (t - t0) / (t2 - t0) * a2;
        const b2 = (t3 - t) / (t3 - t1) * a2 + (t - t1) / (t3 - t1) * a3;
        out[k] = (t2 - t) / (t2 - t1) * b1 + (t - t1) / (t2 - t1) * b2;
    }
}

function buildPath(def) {
    const raw = Array.isArray(def.points) ? def.points : [];
    const pts = raw.map(p => [+p[0] || 0, +p[1] || 0, +p[2] || 0]);
    while (pts.length < 2) pts.push(pts.length ? [pts[0][0], pts[0][1] + 0.01, pts[0][2]] : [0, 0, 0]);
    const n = pts.length - 1;
    const chord = [];
    let total = 0;
    for (let i = 0; i < n; i++) { chord.push(dist3(pts[i], pts[i + 1])); total += chord[i]; }
    const counts = chord.map(c => Math.max(1, Math.round(SEGMENTS * (total > 0 ? c / total : 1 / n))));
    let sum = counts.reduce((a, b) => a + b, 0), big = 0;
    for (let i = 1; i < n; i++) if (counts[i] > counts[big]) big = i;
    counts[big] = Math.max(1, counts[big] + SEGMENTS - sum);
    const count = counts.reduce((a, b) => a + b, 0);
    const xs = new Float64Array(count + 1), ys = new Float64Array(count + 1), hs = new Float64Array(count + 1), cum = new Float64Array(count + 1);
    const tmp = [0, 0, 0];
    let w = 0;
    for (let i = 0; i < n; i++) {
        const p0 = i > 0 ? pts[i - 1] : [2 * pts[0][0] - pts[1][0], 2 * pts[0][1] - pts[1][1], 2 * pts[0][2] - pts[1][2]];
        const p3 = i + 2 <= n ? pts[i + 2] : [2 * pts[n][0] - pts[n - 1][0], 2 * pts[n][1] - pts[n - 1][1], 2 * pts[n][2] - pts[n - 1][2]];
        for (let k = 0; k < counts[i]; k++, w++) {
            crPoint(p0, pts[i], pts[i + 1], p3, k / counts[i], tmp);
            xs[w] = tmp[0]; ys[w] = tmp[1]; hs[w] = tmp[2];
        }
    }
    xs[w] = pts[n][0]; ys[w] = pts[n][1]; hs[w] = pts[n][2];
    xs[0] = pts[0][0]; ys[0] = pts[0][1]; hs[0] = pts[0][2];
    for (let i = 1; i <= count; i++) cum[i] = cum[i - 1] + Math.hypot(xs[i] - xs[i - 1], ys[i] - ys[i - 1], hs[i] - hs[i - 1]);

    const a = def.entry && def.entry.a ? def.entry.a : [pts[0][0] - 0.02, pts[0][1]];
    const b = def.entry && def.entry.b ? def.entry.b : [pts[0][0] + 0.02, pts[0][1]];
    let ex = b[0] - a[0], ey = b[1] - a[1];
    const elen = Math.hypot(ex, ey) || 1;
    ex /= elen; ey /= elen;
    let dx = -ey, dy = ex;
    const mx = (a[0] + b[0]) / 2, my = (a[1] + b[1]) / 2;
    if (dx * (pts[1][0] - mx) + dy * (pts[1][1] - my) < 0) { dx = -dx; dy = -dy; }
    const kind = ROLL_DECEL[def.kind] !== undefined && def.kind !== 'field' ? def.kind : 'plastic';
    const sw = (def.switches || []).map(s => ({ id: s.id, at: Math.max(0, Math.min(1, +s.at || 0)) }));
    return {
        def, id: def.id, kind, decel: ROLL_DECEL[kind], xs, ys, hs, cum, count, len: cum[count],
        ax: a[0], ay: a[1], ex, ey, elen, dx, dy, minSpeed: +def.minSpeed || 0, switches: sw, exit: def.exit || null,
        balls: [], debugPts: pts
    };
}

export function createPaths(defs, ctx) {
    const list = (defs || []).filter(d => d && d.id !== undefined).map(buildPath);
    const byId = new Map(list.map(p => [p.id, p]));
    const divInit = new Map(), divState = new Map();
    const layoutDevices = (ctx && ctx.layout && ctx.layout.devices) || [];
    for (const d of layoutDevices) if (d && d.type === 'diverter') divInit.set(d.id, !!d.on);
    for (const [k, v] of divInit) divState.set(k, v);

    const S = { x: 0, y: 0, h: 0, tx: 0, ty: 0, th: 0 };      // scratch lookup result
    const POS = [0, 0, 0];

    // Fills S with the spline position and unit tangent at arc length s.
    function lookup(P, s) {
        if (s < 0) s = 0; else if (s > P.len) s = P.len;
        const cum = P.cum;
        let lo = 0, hi = P.count - 1;
        while (lo < hi) {
            const mid = (lo + hi + 1) >> 1;
            if (cum[mid] <= s) lo = mid; else hi = mid - 1;
        }
        const seg = cum[lo + 1] - cum[lo] || EPS, f = (s - cum[lo]) / seg;
        const ddx = P.xs[lo + 1] - P.xs[lo], ddy = P.ys[lo + 1] - P.ys[lo], ddh = P.hs[lo + 1] - P.hs[lo];
        S.x = P.xs[lo] + ddx * f; S.y = P.ys[lo] + ddy * f; S.h = P.hs[lo] + ddh * f;
        S.tx = ddx / seg; S.ty = ddy / seg; S.th = ddh / seg;
    }
    const diverterOn = id => (ctx && typeof ctx.diverterOn === 'function') ? !!ctx.diverterOn(id) : !!divState.get(id);

    function sync(ball, st, P) {
        lookup(P, st.s);
        ball.x = S.x; ball.y = S.y; ball.h = S.h;
        ball.vx = S.tx * st.v; ball.vy = S.ty * st.v;
    }
    function attach(P, ball, v, ex, ey) {
        const st = { id: P.id, s: 0, v: Math.min(MAX_SPEED, v), ps: 0, ex, ey };
        ball.path = st; ball.mode = 'path';
        P.balls.push(ball);
        sync(ball, st, P);
        ball.px = ball.x; ball.py = ball.y;
        return st;
    }
    function detach(P, i) { const last = P.balls.length - 1; P.balls[i] = P.balls[last]; P.balls.pop(); }

    function toField(ball, x, y, vx, vy) {
        ball.path = null; ball.mode = 'field'; ball.h = 0;
        ball.x = ball.px = x; ball.y = ball.py = y; ball.vx = vx; ball.vy = vy;
    }

    function launchPath(ball, pathId, speed) {
        const P = byId.get(pathId);
        if (!P || !ball) return false;
        if (ball.path) { const old = byId.get(ball.path.id); if (old) { const i = old.balls.indexOf(ball); if (i >= 0) detach(old, i); } }
        attach(P, ball, +speed || 0, P.xs[0], P.ys[0]);
        ctx.emit('pathEnter', P.id, ball, { speed: ball.path.v, via: 'launch' });
        return true;
    }
    if (ctx) ctx.launchPath = launchPath;

    // Resolve the exit spec at the end of path P (ball is detached and positioned at the end point).
    function applyExit(P, ball, exit, vEnd, depth) {
        let kind = exit ? exit.kind : 'field';
        if (kind === 'switch' && depth < 4) {
            const sub = diverterOn(exit.diverter) ? exit.on : exit.off;
            return applyExit(P, ball, sub || { kind: 'field' }, vEnd, depth + 1);
        }
        const t = lookupEnd(P);
        const tl = Math.hypot(t.tx, t.ty);
        const fx = tl > 1e-6 ? t.tx / tl : P.dx, fy = tl > 1e-6 ? t.ty / tl : P.dy;
        if (kind === 'path' && byId.has(exit.path)) {
            const Q = byId.get(exit.path);
            ctx.emit('pathMade', P.id, ball, { exit: 'path', path: Q.id, speed: vEnd });
            attach(Q, ball, vEnd, Q.xs[0], Q.ys[0]);
            return 'path';
        }
        if (kind === 'hole') {
            ctx.emit('pathMade', P.id, ball, { exit: 'hole', hole: exit.hole, speed: vEnd });
            toField(ball, ball.x, ball.y, fx * vEnd * 0.5, fy * vEnd * 0.5);
            if (typeof ctx.deliverToHole === 'function' && ctx.deliverToHole(ball, exit.hole)) return 'hole';
            return 'field';
        }
        let dx = fx, dy = fy, scale = 0.9;
        if (kind === 'field' && exit) {
            if (Array.isArray(exit.dir)) { const l = Math.hypot(exit.dir[0], exit.dir[1]); if (l > 1e-9) { dx = exit.dir[0] / l; dy = exit.dir[1] / l; } }
            if (Number.isFinite(exit.speedScale)) scale = exit.speedScale;
        }
        ctx.emit('pathMade', P.id, ball, { exit: 'field', speed: vEnd });
        toField(ball, ball.x, ball.y, dx * scale * vEnd, dy * scale * vEnd);
        return 'field';
    }
    function lookupEnd(P) { lookup(P, P.len); return S; }

    function fail(P, ball, st) {
        const x = st.ex - P.dx * 0.0005, y = st.ey - P.dy * 0.0005, v = Math.abs(st.v);
        toField(ball, x, y, -P.dx * v, -P.dy * v);
        ctx.emit('pathFail', P.id, ball, { speed: v });
    }

    function advance(P, h) {
        const bl = P.balls;
        for (let i = bl.length - 1; i >= 0; i--) {
            const ball = bl[i], st = ball.path;
            st.ps = st.s;
            lookup(P, st.s);
            let v = st.v + (-G_SLOPE * S.ty - G_NORMAL * S.th) * h;
            const fr = P.decel * h;
            if (v > fr) v -= fr; else if (v < -fr) v += fr; else v = 0;
            if (v > MAX_SPEED) v = MAX_SPEED; else if (v < -MAX_SPEED) v = -MAX_SPEED;
            st.v = v; st.s += v * h;
            if (st.s <= 0 && v < 0) { detach(P, i); st.s = 0; fail(P, ball, st); continue; }
            if (st.s < 0) st.s = 0;
            // intermediate switches (forward crossings only)
            for (let k = 0; k < P.switches.length; k++) {
                const at = P.switches[k].at * P.len;
                if (st.ps < at && st.s >= at && at < P.len) ctx.emit('pathSwitch', P.switches[k].id, ball, { path: P.id });
            }
            if (st.s >= P.len) {
                st.s = P.len; sync(ball, st, P); ball.px = ball.x; ball.py = ball.y;
                detach(P, i);
                ball.path = null;
                applyExit(P, ball, P.exit, v, 0);
                continue;
            }
            sync(ball, st, P);
            ball.px = ball.x; ball.py = ball.y;
        }
        // balls never pass each other: keep >= 2r of arc between neighbours
        for (let i = 0; i < bl.length; i++) {
            for (let j = i + 1; j < bl.length; j++) {
                const a = bl[i], b = bl[j], sa = a.path, sb = b.path;
                if (Math.abs(sa.s - sb.s) >= SEP) continue;
                const aLead = sa.ps !== sb.ps ? sa.ps > sb.ps : sa.s >= sb.s;
                const lead = aLead ? a : b, tail = aLead ? b : a, sl = lead.path, stl = tail.path;
                if (stl.v > sl.v) { const t = stl.v; stl.v = sl.v * 0.9; sl.v = t * 0.9; }
                if (sl.s - SEP >= 0) stl.s = sl.s - SEP; else { stl.s = 0 < stl.s ? stl.s : 0; sl.s = stl.s + SEP; }
                sync(lead, sl, P); sync(tail, stl, P);
                lead.px = lead.x; lead.py = lead.y; tail.px = tail.x; tail.py = tail.y;
            }
        }
    }

    // ---- entry detection ----------------------------------------------------------------------------------------
    let cur = null;
    function tryEnter(ball) {
        if (ball.mode !== 'field') return;
        const P = cur;
        const dp = (ball.px - P.ax) * P.dx + (ball.py - P.ay) * P.dy;
        const dq = (ball.x - P.ax) * P.dx + (ball.y - P.ay) * P.dy;
        if (!(dp < 0 && dq >= 0)) return;
        const t = dp / (dp - dq);
        const cx = ball.px + (ball.x - ball.px) * t, cy = ball.py + (ball.y - ball.py) * t;
        const u = (cx - P.ax) * P.ex + (cy - P.ay) * P.ey;
        if (u < 0 || u > P.elen) return;
        lookup(P, 0);
        const tl = Math.hypot(S.tx, S.ty);
        const tx = tl > 1e-6 ? S.tx / tl : P.dx, ty = tl > 1e-6 ? S.ty / tl : P.dy;
        const v0 = ball.vx * tx + ball.vy * ty;
        if (!(v0 > 0) || v0 < P.minSpeed) return;
        attach(P, ball, v0, cx, cy);
        ctx.emit('pathEnter', P.id, ball, { speed: v0, via: 'field' });
    }

    function substep(h) {
        if (list.length === 0) return;
        for (let i = 0; i < list.length; i++) { cur = list[i]; ctx.balls.forEach(tryEnter); }
        for (let i = 0; i < list.length; i++) if (list[i].balls.length) advance(list[i], h);
    }

    function dropRefs(id) {
        for (const P of list) for (let i = P.balls.length - 1; i >= 0; i--) {
            if (id === undefined || P.balls[i].id === id) { P.balls[i].path = null; detach(P, i); }
        }
    }
    function command(cmd) {
        if (!cmd) return false;
        if (cmd.type === 'diverter') {
            if (!divState.has(cmd.id)) { let used = false; for (const P of list) used = used || usesDiverter(P.exit, cmd.id); if (!used) return false; }
            divState.set(cmd.id, !!cmd.on);
            return true;
        }
        if (cmd.type === 'removeBall') dropRefs(cmd.id);
        else if (cmd.type === 'clearBalls') dropRefs(undefined);
        return false;
    }
    function usesDiverter(e, id) { return !!e && e.kind === 'switch' && (e.diverter === id || usesDiverter(e.on, id) || usesDiverter(e.off, id)); }

    function reset() {
        for (const P of list) P.balls.length = 0;
        divState.clear();
        for (const [k, v] of divInit) divState.set(k, v);
    }

    function position(ball) {
        const st = ball && ball.path, P = st ? byId.get(st.id) : null;
        if (!P) { POS[0] = ball.x; POS[1] = ball.y; POS[2] = ball.h; return POS; }
        lookup(P, st.s);
        POS[0] = S.x; POS[1] = S.y; POS[2] = S.h;
        return POS;
    }

    function debug() {
        return list.map(P => {
            const samples = [];
            for (let i = 0; i <= P.count; i++) samples.push([P.xs[i], P.ys[i], P.hs[i]]);
            return {
                id: P.id, kind: P.kind, color: P.def.color, width: P.def.width, length: P.len, samples,
                entry: { a: [P.ax, P.ay], b: [P.ax + P.ex * P.elen, P.ay + P.ey * P.elen], dir: [P.dx, P.dy] },
                balls: P.balls.map(b => ({ id: b.id, s: b.path.s, v: b.path.v }))
            };
        });
    }

    return { substep, command, reset, position, debug, launchPath, length: id => (byId.has(id) ? byId.get(id).len : 0) };
}

export default createPaths;
