// Course intro flight (P2): pure math on [x, y, z] arrays so Node tests can run it.
const BEHIND = 2.2, ABOVE = 3.2, AHEAD = 2.5, PHASE_A = .6;
const smooth = t => t * t * (3 - 2 * t);
const distance = (a, b) => Math.hypot(b[0] - a[0], b[1] - a[1], b[2] - a[2]);

export function pathLength(path) {
    let total = 0;
    for (let i = 1; i < (path?.length || 0); i++) total += distance(path[i - 1], path[i]);
    return total;
}

export const introDuration = path => Math.min(4.5, Math.max(2.5, .35 * pathLength(path)));

// Overview pose that frames the whole island. Islands up to 5.5 m keep the fixed P2 view
// (midpoint of tee and cup, offset (10, 13, 14)); larger ones look at the island centre from
// farther away. shadowHalf is the half-width of the sun shadow camera, waterRadius the open water disc.
const VIEW = [10, 13, 14], VIEW_LENGTH = Math.hypot(...VIEW);
export function overviewFor(course) {
    const { tee, cup } = course, island = course.island;
    const half = Array.isArray(island?.half) ? island.half : null;
    const span = half ? Math.max(Number(half[0]) || 0, Number(half[1]) || 0) : 0;
    if (span <= 5.5) {
        const target = [0, 1, 2].map(a => (tee[a] + cup[a]) / 2);
        return { position: target.map((value, a) => value + VIEW[a]), target, shadowHalf: 9, waterRadius: 60 };
    }
    const center = Array.isArray(island.center) ? island.center : [0, 0];
    const target = [center[0], (tee[1] + cup[1]) / 2, center[1]];
    const distance = Math.max(21.6, Math.min(60, span * 3.4));
    return {
        position: target.map((value, a) => value + VIEW[a] / VIEW_LENGTH * distance), target,
        shadowHalf: Math.max(9, span + 1.5), waterRadius: Math.max(60, span * 5)
    };
}

export function createIntro(path, endPosition, endTarget) {
    if (!Array.isArray(path) || path.length < 2) return null;
    const cumulative = [0];
    for (let i = 1; i < path.length; i++) cumulative.push(cumulative[i - 1] + distance(path[i - 1], path[i]));
    const total = cumulative[cumulative.length - 1];
    if (!(total > .5)) return null;
    const duration = introDuration(path), here = [0, 0, 0], ahead = [0, 0, 0];
    function along(d, out) {
        let i = 1;
        while (i < path.length - 1 && cumulative[i] < d) i++;
        const span = cumulative[i] - cumulative[i - 1] || 1, k = Math.max(0, Math.min(1, (d - cumulative[i - 1]) / span));
        for (let a = 0; a < 3; a++) out[a] = path[i - 1][a] + (path[i][a] - path[i - 1][a]) * k;
        return out;
    }
    function poseA(u, position, target) {
        const d = smooth(Math.max(0, Math.min(1, u))) * total;
        along(d, here); along(Math.min(total, d + AHEAD), ahead);
        let hx = ahead[0] - here[0], hz = ahead[2] - here[2], length = Math.hypot(hx, hz);
        if (length < 1e-4) {
            const a = path[path.length - 2], b = path[path.length - 1];
            hx = b[0] - a[0]; hz = b[2] - a[2]; length = Math.hypot(hx, hz) || 1;
        }
        hx /= length; hz /= length;
        position[0] = here[0] - hx * BEHIND; position[1] = here[1] + ABOVE; position[2] = here[2] - hz * BEHIND;
        target[0] = ahead[0]; target[1] = ahead[1]; target[2] = ahead[2];
    }
    const endA = [[0, 0, 0], [0, 0, 0]];
    poseA(1, endA[0], endA[1]);
    return {
        duration,
        // Writes the camera pose for `time` seconds into position/target; returns true when finished.
        sample(time, position, target) {
            const t = Math.max(0, Math.min(1, time / duration));
            if (t <= PHASE_A) { poseA(t / PHASE_A, position, target); return false; }
            const k = smooth((t - PHASE_A) / (1 - PHASE_A));
            for (let a = 0; a < 3; a++) {
                position[a] = endA[0][a] + (endPosition[a] - endA[0][a]) * k;
                target[a] = endA[1][a] + (endTarget[a] - endA[1][a]) * k;
            }
            return t >= 1;
        }
    };
}
