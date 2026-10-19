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

export function powerHex(value) {
    const t = Math.max(0, Math.min(1, Number(value) || 0));
    const index = t <= POWER_STOPS[1][0] ? 0 : 1;
    const [from, a] = POWER_STOPS[index], [to, b] = POWER_STOPS[index + 1];
    const k = (t - from) / (to - from), x = parseInt(a.slice(1), 16), y = parseInt(b.slice(1), 16);
    return '#' + [16, 8, 0].map(shift => Math.round(((x >> shift) & 255) * (1 - k) + ((y >> shift) & 255) * k).toString(16).padStart(2, '0')).join('');
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
