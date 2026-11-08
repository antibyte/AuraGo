// Shared AuraGolf constants and pure helpers. No three.js, Rapier or DOM, so the
// model, HUD, map, scene, physics and the Node tests can all import this module.
export const MAX_NAME_LENGTH = 24;
export const WORLD_ID = /^[a-z][a-z0-9-]{0,23}$/;
export const PALETTE = Object.freeze([
    { id: 'cream', hex: '#fff0c5' }, { id: 'coral', hex: '#f6a378' },
    { id: 'sky', hex: '#89c9ef' }, { id: 'lilac', hex: '#c8a6ed' },
    { id: 'lime', hex: '#a6d86f' }, { id: 'gold', hex: '#f4c542' },
    { id: 'rose', hex: '#f28cb1' }, { id: 'teal', hex: '#4cc2b4' }
].map(Object.freeze));
export const COLOR_IDS = Object.freeze(PALETTE.map(color => color.id));
export const DEFAULT_COLORS = Object.freeze(COLOR_IDS.slice(0, 4));
// Colour stops of the 3D power ring and the HUD power meter.
export const POWER_STOPS = Object.freeze([[0, '#7fe0a6'], [0.5, '#ffd789'], [1, '#ff8f77']].map(Object.freeze));
// Floors roll; physics treats every other surface as a bouncing wall and the map draws it as a rail.
export const FLOOR_SURFACES = new Set(['grass', 'sand']);
const CONTROL = /[\x00-\x1f\x7f-\x9f]/g;
const MINUS = String.fromCharCode(0x2212);

export const colorValue = id => (PALETTE.find(color => color.id === id) || PALETTE[0]).hex;

// Keeps valid, unused requested colours; other slots get their default colour
// when free, otherwise the first free palette colour.
export function assignColors(requested) {
    const taken = new Set(), result = new Array(requested.length);
    requested.forEach((id, index) => { if (COLOR_IDS.includes(id) && !taken.has(id)) { taken.add(id); result[index] = id; } });
    for (let index = 0; index < result.length; index++) {
        if (result[index]) continue;
        const preferred = DEFAULT_COLORS[index];
        result[index] = preferred && !taken.has(preferred) ? preferred : COLOR_IDS.find(id => !taken.has(id));
        taken.add(result[index]);
    }
    return result;
}

export function cleanName(value, fallback = '') {
    const cut = text => Array.from(String(text ?? '').replace(CONTROL, '').trim()).slice(0, MAX_NAME_LENGTH).join('').trim();
    return cut(value) || cut(fallback);
}

// Case-insensitive duplicates get " 2", " 3", ...; the stem shrinks to stay within 24 code points.
export function uniqueNames(names) {
    const used = new Set();
    return names.map(name => {
        let candidate = name, suffix = 1;
        while (used.has(candidate.toLocaleLowerCase())) {
            const tail = ' ' + ++suffix;
            candidate = Array.from(name).slice(0, MAX_NAME_LENGTH - tail.length).join('').trimEnd() + tail;
        }
        used.add(candidate.toLocaleLowerCase());
        return candidate;
    });
}

export const worldIds = courses => [...new Set((courses || []).map(course => course?.world).filter(id => typeof id === 'string' && WORLD_ID.test(id)))];

export const formatToPar = (diff, parLabel) => diff === 0 ? parLabel : diff < 0 ? MINUS + Math.abs(diff) : '+' + diff;

// Fills {name} placeholders; function replacements keep "$" in player names literal.
export const fillText = (text, values) => Object.entries(values).reduce((out, [name, value]) => out.replace('{' + name + '}', () => String(value)), text);

export function powerHex(value) {
    const t = Math.max(0, Math.min(1, Number(value) || 0));
    const index = t <= POWER_STOPS[1][0] ? 0 : 1;
    const [from, a] = POWER_STOPS[index], [to, b] = POWER_STOPS[index + 1];
    const k = (t - from) / (to - from), x = parseInt(a.slice(1), 16), y = parseInt(b.slice(1), 16);
    return '#' + [16, 8, 0].map(shift => Math.round(((x >> shift) & 255) * (1 - k) + ((y >> shift) & 255) * k).toString(16).padStart(2, '0')).join('');
}

// Cloth this close to solid geometry of the same prop (metres) does not sway, so a
// flag's hoist stays on the pole. The wind reaches full strength at PIN_FREE.
const PIN_HOLD = 0.035;
const PIN_FREE = 0.22;

// Pivot of a hole flag authored in world space: the midpoint of its only vertical
// edge (the hoist). A cloth with two vertical edges (hoist and fly) returns null,
// because either edge would be a guess. The scene then leaves that cloth still.
export function flagPolePivot(positions) {
    if (!Array.isArray(positions) || positions.length < 2) return null;
    const edges = [];
    for (let a = 0; a < positions.length; a++) {
        const pa = positions[a];
        if (!pa || pa.length < 3 || !pa.every(Number.isFinite)) return null;
        for (let b = a + 1; b < positions.length; b++) {
            const pb = positions[b];
            const vertical = Math.abs(pa[1] - pb[1]);
            if (vertical < 0.05) continue;
            if (Math.hypot(pa[0] - pb[0], pa[2] - pb[2]) > vertical * 0.02) continue;
            edges.push([(pa[0] + pb[0]) / 2, (pa[1] + pb[1]) / 2, (pa[2] + pb[2]) / 2]);
        }
    }
    if (!edges.length) return null;
    const [x, , z] = edges[0];
    let y = 0;
    for (const edge of edges) {
        if (Math.hypot(edge[0] - x, edge[2] - z) > 0.008) return null;
        y += edge[1];
    }
    return [x, y / edges.length, z];
}

// Pole meshes keep long edges, so a hoist vertex can sit on the wood and still be
// many centimetres from the nearest vertex. Samples along each edge close that gap.
export function edgeSamples(points, indices, step = 0.02) {
    if (!points?.length) return [];
    const samples = points.slice();
    const seen = new Set();
    const count = indices ? indices.length : points.length;
    const at = i => indices ? indices[i] : i;
    for (let i = 0; i + 2 < count; i += 3) {
        for (let corner = 0; corner < 3; corner++) {
            const ia = at(i + corner), ib = at(i + (corner + 1) % 3);
            const key = ia < ib ? ia + ':' + ib : ib + ':' + ia;
            if (seen.has(key)) continue;
            seen.add(key);
            const pa = points[ia], pb = points[ib];
            if (!pa || !pb) continue;
            const length = Math.hypot(pb[0] - pa[0], pb[1] - pa[1], pb[2] - pa[2]);
            const n = Math.floor(length / step);
            for (let k = 1; k < n; k++) {
                const t = k / n;
                samples.push([pa[0] + (pb[0] - pa[0]) * t, pa[1] + (pb[1] - pa[1]) * t, pa[2] + (pb[2] - pa[2]) * t]);
            }
        }
    }
    return samples;
}

// One wind weight per cloth vertex: 0 on the pole, 1 once PIN_FREE away from the
// nearest solid sample. An empty anchor list leaves the cloth free.
export function clothPinFactors(points, anchors) {
    if (!points?.length) return [];
    if (!anchors?.length) return points.map(() => 1);
    const buckets = new Map();
    const key = (x, y, z) => `${Math.floor(x / PIN_FREE)},${Math.floor(y / PIN_FREE)},${Math.floor(z / PIN_FREE)}`;
    for (const anchor of anchors) {
        const id = key(anchor[0], anchor[1], anchor[2]);
        const list = buckets.get(id);
        if (list) list.push(anchor);
        else buckets.set(id, [anchor]);
    }
    const hold2 = PIN_HOLD * PIN_HOLD, free2 = PIN_FREE * PIN_FREE;
    return points.map(point => {
        const ix = Math.floor(point[0] / PIN_FREE), iy = Math.floor(point[1] / PIN_FREE), iz = Math.floor(point[2] / PIN_FREE);
        let best = Infinity;
        for (let dx = -1; dx <= 1; dx++) for (let dy = -1; dy <= 1; dy++) for (let dz = -1; dz <= 1; dz++) {
            const list = buckets.get(`${ix + dx},${iy + dy},${iz + dz}`);
            if (!list) continue;
            for (const anchor of list) {
                const ex = point[0] - anchor[0], ey = point[1] - anchor[1], ez = point[2] - anchor[2];
                const distance = ex * ex + ey * ey + ez * ez;
                if (distance < best) best = distance;
            }
        }
        if (best <= hold2) return 0;
        if (best >= free2) return 1;
        const t = (Math.sqrt(best) - PIN_HOLD) / (PIN_FREE - PIN_HOLD);
        return t * t * (3 - 2 * t);
    });
}

// Automatic graphics tier: starts at medium, drops to low below 38 FPS (at most
// every 6 s), returns to medium after ~10 s at >= 55 FPS and 12 s after a change.
export function createQualityGovernor() {
    let tier = 'medium', changedAt = 0, goodFor = 0, sum = 0, count = 0, fps = 0;
    return {
        get tier() { return tier; },
        get fps() { return fps; },
        reset(now = 0) { tier = 'medium'; changedAt = now; goodFor = 0; },
        frame(dt, now) {
            if (!(dt > 0 && dt < 0.5)) return null;
            sum += dt; count++;
            if (sum <= 2) return null;
            const span = sum;
            fps = count / sum; sum = 0; count = 0;
            if (tier === 'medium') {
                if (fps >= 38 || now - changedAt <= 6) return null;
                tier = 'low'; changedAt = now; goodFor = 0;
                return 'down';
            }
            goodFor = fps >= 55 ? goodFor + span : 0;
            if (goodFor < 10 || now - changedAt < 12) return null;
            tier = 'medium'; changedAt = now; goodFor = 0;
            return 'up';
        }
    };
}
