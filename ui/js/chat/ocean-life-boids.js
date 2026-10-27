// Ocean chat theme: DOM-free school simulation (boids) and a seeded RNG.
// The module has no side effects and no three.js dependency so it can be unit-tested in Node
// (scripts/test-chat-ocean-life.mjs) and is bundled into the sea life renderer.

const DEFAULT_PARAMS = {
    neighbor: 1.2,
    separation: 0.35,
    align: 0.6,
    cohesion: 0.35,
    maxTurn: 2.4,
    bounce: 1.6
};

const LANE_GAIN = 3.2;
const BOUNCE_MARGIN = 0.35;
const BURST_DECAY_SECONDS = 1.2;
const BURST_BOOST = 1.2;
const SPEED_RESPONSE = 3.0;

// mulberry32: tiny, fast, good enough for placement and jitter.
export function createRng(seed) {
    let a = seed >>> 0;
    return function rng() {
        a = (a + 0x6d2b79f5) >>> 0;
        let t = a;
        t = Math.imul(t ^ (t >>> 15), t | 1);
        t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
        return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
}

function clamp01(x) {
    return x < 0 ? 0 : x > 1 ? 1 : x;
}

// GLSL-style smoothstep that also accepts edge0 > edge1 (falling edge).
function smoothstep(edge0, edge1, x) {
    const t = clamp01((x - edge0) / (edge1 - edge0));
    return t * t * (3 - 2 * t);
}

function between(rng, range) {
    return range[0] + rng() * (range[1] - range[0]);
}

export function createSchool({ count, rng, bounds, speed, size, params }) {
    const n = Math.max(0, count | 0);
    const cfg = Object.assign({}, DEFAULT_PARAMS, params || {});
    const pos = new Float32Array(n * 3);
    const vel = new Float32Array(n * 3);
    const sizes = new Float32Array(n);
    const phase = new Float32Array(n);
    const cruise = new Float32Array(n);
    const burst = new Float32Array(n);
    const nextVel = new Float32Array(n * 3);
    let clock = 0;

    // Fish start in a few loose clusters with a shared heading so the school reads as a school
    // from the first frame; the boid rules keep it together afterwards.
    const clusters = Math.max(1, Math.round(n / 10));
    const centres = [];
    for (let c = 0; c < clusters; c++) {
        const yaw = rng() * Math.PI * 2;
        centres.push({
            x: bounds.x[0] + (bounds.x[1] - bounds.x[0]) * (0.1 + 0.8 * rng()),
            y: bounds.y[0] + (bounds.y[1] - bounds.y[0]) * (0.15 + 0.7 * rng()),
            z: bounds.z[0] + (bounds.z[1] - bounds.z[0]) * (0.2 + 0.6 * rng()),
            hx: Math.cos(yaw),
            hz: Math.sin(yaw) * 0.35
        });
    }
    for (let i = 0; i < n; i++) {
        const c = centres[i % clusters];
        const o = i * 3;
        pos[o] = c.x + (rng() - 0.5) * 1.6;
        pos[o + 1] = c.y + (rng() - 0.5) * 0.8;
        pos[o + 2] = c.z + (rng() - 0.5) * 0.8;
        cruise[i] = between(rng, speed);
        sizes[i] = between(rng, size);
        phase[i] = rng();
        let hx = c.hx + (rng() - 0.5) * 0.5;
        let hy = (rng() - 0.5) * 0.2;
        let hz = c.hz + (rng() - 0.5) * 0.3;
        const len = Math.hypot(hx, hy, hz) || 1;
        hx /= len; hy /= len; hz /= len;
        vel[o] = hx * cruise[i];
        vel[o + 1] = hy * cruise[i];
        vel[o + 2] = hz * cruise[i];
    }

    function step(dt, env) {
        if (!(dt > 0) || n === 0) return;
        const e = env || {};
        const laneHalfWidth = typeof e.laneHalfWidth === 'function' ? e.laneHalfWidth : null;
        const laneStrength = e.laneStrength || 0;
        const repulsors = e.repulsors || [];
        const speedMul = e.speedMul == null ? 1 : e.speedMul;
        const cohesionMul = e.cohesionMul == null ? 1 : e.cohesionMul;
        const neighborR = cfg.neighbor;
        const neighborR2 = neighborR * neighborR;
        const sepR = neighborR * 0.4;
        const sepR2 = sepR * sepR;
        const burstDecay = Math.exp(-dt / BURST_DECAY_SECONDS);
        const maxStep = cfg.maxTurn * dt;
        clock += dt;

        for (let i = 0; i < n; i++) {
            const o = i * 3;
            const px = pos[o], py = pos[o + 1], pz = pos[o + 2];
            const speedNow = Math.hypot(vel[o], vel[o + 1], vel[o + 2]);
            let hx, hy, hz;
            if (speedNow > 1e-6) {
                hx = vel[o] / speedNow; hy = vel[o + 1] / speedNow; hz = vel[o + 2] / speedNow;
            } else {
                hx = 1; hy = 0; hz = 0;
            }

            // Boid rules over the neighbours.
            let sepX = 0, sepY = 0, sepZ = 0;
            let aliX = 0, aliY = 0, aliZ = 0;
            let cohX = 0, cohY = 0, cohZ = 0;
            let near = 0;
            for (let j = 0; j < n; j++) {
                if (j === i) continue;
                const q = j * 3;
                const dx = pos[q] - px, dy = pos[q + 1] - py, dz = pos[q + 2] - pz;
                const d2 = dx * dx + dy * dy + dz * dz;
                if (d2 > neighborR2) continue;
                near++;
                cohX += dx; cohY += dy; cohZ += dz;
                const sj = Math.hypot(vel[q], vel[q + 1], vel[q + 2]) || 1;
                aliX += vel[q] / sj; aliY += vel[q + 1] / sj; aliZ += vel[q + 2] / sj;
                if (d2 < sepR2) {
                    const d = Math.sqrt(d2) || 1e-3;
                    const push = (1 - d / sepR) / d;
                    sepX -= dx * push; sepY -= dy * push; sepZ -= dz * push;
                }
            }
            let wx = hx, wy = hy, wz = hz;
            if (near > 0) {
                const inv = 1 / near;
                // cohesion: toward the neighbour centroid, normalised by the neighbour radius
                wx += cfg.cohesion * cohesionMul * Math.min(1, cohX * inv / neighborR);
                wy += cfg.cohesion * cohesionMul * Math.min(1, cohY * inv / neighborR) * 0.6;
                wz += cfg.cohesion * cohesionMul * Math.min(1, cohZ * inv / neighborR);
                // alignment: toward the mean neighbour heading
                wx += cfg.align * (aliX * inv - hx);
                wy += cfg.align * (aliY * inv - hy) * 0.6;
                wz += cfg.align * (aliZ * inv - hz);
                // separation
                wx += cfg.separation * 4 * sepX;
                wy += cfg.separation * 4 * sepY * 0.6;
                wz += cfg.separation * 4 * sepZ;
            }

            // Fish prefer level swimming and wander a little so the school never looks mechanical.
            wy -= hy * 0.35;
            const wander = clock * 0.6 + phase[i] * 6.2831;
            wx += Math.sin(wander) * 0.12;
            wy += Math.sin(wander * 1.3 + 1.7) * 0.07;
            wz += Math.cos(wander * 0.9) * 0.05;

            // Reading lane: keep the centre of the chat free.
            if (laneHalfWidth && laneStrength > 0) {
                const half = laneHalfWidth(pz);
                if (half > 0) {
                    const edge = smoothstep(half + 0.8, half - 0.8, Math.abs(px));
                    if (edge > 0) {
                        const side = Math.abs(px) < 1e-3 ? (i % 2 ? 1 : -1) : Math.sign(px);
                        wx += side * laneStrength * LANE_GAIN * edge;
                    }
                }
            }

            // Repulsors (pointer, sharks): push away and startle.
            let inside = false;
            for (let r = 0; r < repulsors.length; r++) {
                const rep = repulsors[r];
                const dx = px - rep.x, dy = py - rep.y, dz = pz - rep.z;
                const d = Math.hypot(dx, dy, dz);
                if (d >= rep.radius) continue;
                inside = true;
                const k = rep.strength * (1 - d / rep.radius);
                if (d > 1e-4) {
                    wx += dx / d * k; wy += dy / d * k * 0.6; wz += dz / d * k;
                } else {
                    wx += hx * k; wy += hy * k; wz += hz * k;
                }
            }
            burst[i] *= burstDecay;
            if (inside) burst[i] = 1;

            // Soft bounds: steer back toward the volume once a fish leaves it (or nears the edge).
            const bx = overshoot(px, bounds.x);
            const by = overshoot(py, bounds.y);
            const bz = overshoot(pz, bounds.z);
            wx += bx * cfg.bounce;
            wy += by * cfg.bounce;
            wz += bz * cfg.bounce;

            // Turn the heading toward the wish at a limited rate.
            const wl = Math.hypot(wx, wy, wz);
            let nx = hx, ny = hy, nz = hz;
            if (wl > 1e-6) {
                const tx = wx / wl, ty = wy / wl, tz = wz / wl;
                const cosA = Math.max(-1, Math.min(1, hx * tx + hy * ty + hz * tz));
                const angle = Math.acos(cosA);
                if (angle <= maxStep) {
                    nx = tx; ny = ty; nz = tz;
                } else if (angle > 1e-5) {
                    // rotate by maxStep in the plane spanned by heading and target
                    let px2 = tx - cosA * hx, py2 = ty - cosA * hy, pz2 = tz - cosA * hz;
                    let pl = Math.hypot(px2, py2, pz2);
                    if (pl < 1e-5) {
                        // anti-parallel: pick any perpendicular, deterministic
                        px2 = -hz; py2 = 0; pz2 = hx;
                        pl = Math.hypot(px2, pz2);
                        if (pl < 1e-5) { px2 = 0; py2 = 1; pz2 = 0; pl = 1; }
                    }
                    px2 /= pl; py2 /= pl; pz2 /= pl;
                    const c = Math.cos(maxStep), s = Math.sin(maxStep);
                    nx = c * hx + s * px2; ny = c * hy + s * py2; nz = c * hz + s * pz2;
                }
            }
            const target = cruise[i] * speedMul * (1 + BURST_BOOST * burst[i]);
            const spd = speedNow + (target - speedNow) * Math.min(1, dt * SPEED_RESPONSE);
            nextVel[o] = nx * spd;
            nextVel[o + 1] = ny * spd;
            nextVel[o + 2] = nz * spd;
        }

        for (let i = 0; i < n * 3; i++) {
            vel[i] = nextVel[i];
            pos[i] += vel[i] * dt;
        }
    }

    // 0 inside the volume (minus a small margin), ramps to a unit pull toward the inside outside it
    function overshoot(p, range) {
        const lo = range[0] + BOUNCE_MARGIN, hi = range[1] - BOUNCE_MARGIN;
        if (p < lo) return Math.min(1.5, (lo - p) / 0.8);
        if (p > hi) return -Math.min(1.5, (p - hi) / 0.8);
        return 0;
    }

    return { count: n, pos, vel, size: sizes, phase, cruise, burst, step };
}
