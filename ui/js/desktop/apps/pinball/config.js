// AuraPinball shared configuration: storage keys, helpers, settings and score normalisation. DOM-free.
export const STORAGE = Object.freeze({ settings: 'aurago.desktop.pinball.settings.v1', scores: 'aurago.desktop.pinball.scores.v1' });
export const TABLE_IDS = Object.freeze(['neon', 'leviathan', 'temple']);
export const ASSET_ROOT = '/3d/pinball/v1/';
export const FONT_STACK = '"Segoe UI","Noto Sans","Helvetica Neue",Arial,"Microsoft YaHei","PingFang SC","Hiragino Sans","Nirmala UI","Noto Sans Devanagari",sans-serif';

export const clamp = (v, low, high) => Math.max(low, Math.min(high, v));
export const finite = (v, fallback) => typeof v === 'number' && Number.isFinite(v) ? v : fallback;
export const copy = value => JSON.parse(JSON.stringify(value));

// mulberry32 on holder.seed (uint32), so the generator state stays JSON-serialisable.
export function rngNext(holder) {
    holder.seed = (holder.seed + 0x6D2B79F5) >>> 0;
    let t = holder.seed;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
}

export const DEFAULT_KEYS = Object.freeze({ left: 'ShiftLeft', right: 'ShiftRight', plunger: 'Space', start: 'Digit1', nudgeLeft: 'ArrowLeft', nudgeRight: 'ArrowRight', nudgeUp: 'ArrowUp', pause: 'Escape', camera: 'KeyC', reset: 'KeyR' });
export const DEFAULT_PAD = Object.freeze({ left: 4, right: 5, plunger: 7, start: 8, pause: 9, camera: 3, reset: 2, nudgeLeft: 14, nudgeRight: 15, nudgeUp: 12 });
const START_KEY_CHOICES = Object.freeze(['Digit1', 'Digit2', 'KeyS', 'KeyT']);
const START_PAD_CHOICES = Object.freeze([8, 6, 10, 11]);
const KEY_RE = /^[A-Za-z][A-Za-z0-9]{0,23}$/;

function normalizeBindings(stored, defaults, valid, startChoices) {
    const out = { ...defaults };
    const given = {};
    for (const name of Object.keys(defaults)) if (valid(stored?.[name])) given[name] = stored[name];
    const others = Object.keys(defaults).filter(name => name !== 'start');
    for (const name of others) if (name in given) out[name] = given[name];
    if (new Set(others.map(name => out[name])).size !== others.length) return { ...defaults };
    if ('start' in given) out.start = given.start;
    else if (others.some(name => out[name] === defaults.start)) {
        const used = new Set(others.map(name => out[name]));
        const free = startChoices.find(choice => !used.has(choice));
        if (free === undefined) return { ...defaults };
        out.start = free;
    }
    if (new Set(Object.values(out)).size !== Object.keys(out).length) return { ...defaults };
    return out;
}

export function normalizeSettings(value) {
    const s = value && typeof value === 'object' ? value : {};
    const keys = normalizeBindings(s.keys, DEFAULT_KEYS, v => typeof v === 'string' && KEY_RE.test(v), START_KEY_CHOICES);
    const pad = normalizeBindings(s.pad, DEFAULT_PAD, v => Number.isInteger(v) && v >= 0 && v <= 31, START_PAD_CHOICES);
    return {
        quality: ['low', 'medium', 'high'].includes(s.quality) ? s.quality : 'high',
        effects: clamp(finite(s.effects, .65), 0, 1),
        music: clamp(finite(s.music, .5), 0, 1),
        ambience: clamp(finite(s.ambience, .3), 0, 1),
        callouts: s.callouts !== false,
        camera: s.camera === 'top' ? 'top' : 'play',
        reduced: s.reduced === true,
        keys, pad
    };
}

export function normalizeScores(value) {
    const out = {};
    for (const id of TABLE_IDS) out[id] = (Array.isArray(value?.[id]) ? value[id] : [])
        .filter(row => row && typeof row.name === 'string' && Number.isSafeInteger(row.score) && row.score >= 0 && Number.isFinite(row.date))
        .map(row => ({ name: Array.from(row.name).slice(0, 24).join(''), score: row.score, date: row.date }))
        .sort((a, b) => b.score - a.score).slice(0, 10);
    return out;
}
