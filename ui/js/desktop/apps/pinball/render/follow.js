// AuraPinball ball camera maths (mode `follow`). Pure and DOM/three-free so Node tests can import it.
// Table coordinates: x across [-0.2575, 0.2575], y up the table [0, 1.07], h above the playfield (metres).
// followGoal() picks what to frame, createFollow()/stepFollow() chase it with critically damped springs.
export const FOLLOW = Object.freeze({
    elevation: 43,        // degrees above the playfield (play uses 49)
    depth: 0.56,          // metres of playfield depth in view while following one ball
    maxDepth: 1.1,        // multiball spread zooms out to the whole table (the play framing)
    tau: 0.26,            // s, time constant of the position spring
    tauDepth: 0.55,       // s, time constant of the zoom spring
    lookahead: 0.14,      // s of ball velocity added to the target
    lookMax: 0.09,        // m, largest lookahead shift
    yMin: -0.03,          // lowest visible playfield y (lockdown bar edge)
    yMax: 1.07,           // highest visible playfield y (top arc)
    restY: 0.25,          // framing centre without a ball in play (clamped to the flippers view)
    xGain: 0.7,           // fraction of the ball x followed when the width does not fit
    pad: 0.17,            // m of margin around the two lowest balls before zooming out
    maxWiden: 1.22,       // the camera may back off this much to fit the full width before cropping it
    blend: 0.5            // s, smooth switch between camera modes
});

const num = (v, d) => typeof v === 'number' && Number.isFinite(v) ? v : d;
const clamp = (v, lo, hi) => Math.max(lo, Math.min(hi, v));
// Only real play balls count: table toys such as the leviathan's captive pearl ball are not the ball to follow.
const PLAY_MODES = ['field', 'path', 'held', 'grab', 'shooter'];
const live = b => b && !b.hidden && PLAY_MODES.includes(b.mode) && Number.isFinite(b.x) && Number.isFinite(b.y);

// Screen-vertical position of a point equals y + h / tan(elevation): a raised ball needs the target shifted up the table.
const lift = (b, p) => num(b.h, 0) / Math.tan(p.elevation * Math.PI / 180);

// What to frame: the most urgent (lowest) in-play ball; several far apart widen the depth so the lowest two stay in view.
export function followGoal(balls, opts = {}) {
    const p = { ...FOLLOW, ...opts }, xRange = Math.max(0, num(opts.xRange, 0));
    const list = (Array.isArray(balls) ? balls : []).filter(live).sort((a, b) => a.y - b.y);
    if (!list.length) return { y: clamp(p.restY, p.yMin + p.depth / 2, p.yMax - p.depth / 2), x: 0, depth: p.depth, balls: 0 };
    const eff = b => {
        const still = b.mode === 'held' || b.mode === 'grab' || b.mode === 'shooter';
        const look = still ? 0 : clamp(num(b.vy, 0) * p.lookahead, -p.lookMax, p.lookMax);
        return b.y + look + lift(b, p);
    };
    const first = list[0], y1 = eff(first);
    let center = y1, depth = p.depth;
    if (list.length > 1) {
        const y2 = eff(list[1]), span = Math.abs(y2 - y1);
        depth = clamp(Math.max(p.depth, span + 2 * p.pad), p.depth, p.maxDepth);
        center = y1 + (y2 - y1) * 0.5 * clamp((span - 0.1) / 0.25, 0, 1);
    }
    const lookX = first.mode === 'held' || first.mode === 'grab' || first.mode === 'shooter' ? 0 : clamp(num(first.vx, 0) * p.lookahead, -p.lookMax, p.lookMax);
    const x = clamp((first.x + lookX) * p.xGain, -xRange, xRange);
    return { y: clamp(center, p.yMin + depth / 2, p.yMax - depth / 2), x, depth, balls: list.length };
}

export const createFollow = () => ({ ready: false, y: 0, vy: 0, x: 0, vx: 0, depth: FOLLOW.depth, vd: 0 });

// Exact critically damped step for a constant goal: frame-rate independent.
function spring(x, v, goal, tau, dt) {
    const w = 1 / tau, e = Math.exp(-w * dt), d = x - goal, j = v + w * d;
    return [goal + (d + j * dt) * e, (v - w * j * dt) * e];
}

// Advances the state towards the goal by dt seconds. The first call (or snap) jumps to the goal. The result always stays inside the table.
export function stepFollow(s, goal, dt, opts = {}) {
    const p = { ...FOLLOW, ...opts };
    if (!s.ready || opts.snap) { s.ready = true; s.y = goal.y; s.x = goal.x; s.depth = goal.depth; s.vy = s.vx = s.vd = 0; }
    else if (dt > 0) {
        const h = Math.min(dt, 0.25);
        [s.y, s.vy] = spring(s.y, s.vy, goal.y, p.tau, h);
        [s.x, s.vx] = spring(s.x, s.vx, goal.x, p.tau, h);
        [s.depth, s.vd] = spring(s.depth, s.vd, goal.depth, p.tauDepth, h);
    }
    s.depth = clamp(num(s.depth, p.depth), p.depth, p.maxDepth);
    s.y = clamp(num(s.y, p.restY), p.yMin + s.depth / 2, p.yMax - s.depth / 2);
    s.x = num(s.x, 0);
    return s;
}
