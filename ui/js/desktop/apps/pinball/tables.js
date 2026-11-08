export const STEP = 1 / 480;
export const BALL_RADIUS = .0135;
export const BALL_MASS = .08;
export const TABLES = Object.freeze([
    { id: 'neon', title: 'Cyber Neon Metropolis', accent: '#37d9ff', secondary: '#ff48c4', missions: ['hack', 'uplink', 'security'], finale: 'overload', goals: [{ type: 'target', count: 3 }, { type: 'ramp', count: 4, alternate: true }, { type: 'orbit', count: 5 }] },
    { id: 'leviathan', title: 'Deep Sea Leviathan', accent: '#43e3d5', secondary: '#dcad63', missions: ['ruins', 'relics', 'current'], finale: 'awakening', goals: [{ type: 'orbit', count: 3 }, { type: 'magnet', count: 2 }, { type: 'ramp', count: 3 }] },
    { id: 'temple', title: 'Lost Temple of the Sun', accent: '#ffcb65', secondary: '#8ad6a3', missions: ['expedition', 'sun_relics', 'chamber'], finale: 'sunrise', goals: [{ type: 'target', count: 3 }, { type: 'upper', count: 2 }, { type: 'ramp', count: 3 }] }
]);
export const tableOf = id => TABLES.find(table => table.id === id) || TABLES[0];
export const ASSET_ROOT = '/3d/pinball/v1/';
export const clamp = (v, low, high) => Math.max(low, Math.min(high, v));
export const finite = (v, fallback) => typeof v === 'number' && Number.isFinite(v) ? v : fallback;
export const copy = value => JSON.parse(JSON.stringify(value));
export const DEFAULT_KEYS = Object.freeze({ left: 'ShiftLeft', right: 'ShiftRight', plunger: 'Space', nudgeLeft: 'ArrowLeft', nudgeRight: 'ArrowRight', nudgeUp: 'ArrowUp', pause: 'Escape', camera: 'KeyC', reset: 'KeyR' });
export const DEFAULT_PAD = Object.freeze({ left: 4, right: 5, plunger: 7, pause: 9, camera: 3, reset: 2, nudgeLeft: 14, nudgeRight: 15, nudgeUp: 12 });

export function normalizeSettings(value) {
    const s = value && typeof value === 'object' ? value : {};
    const keys = { ...DEFAULT_KEYS }, pad = { ...DEFAULT_PAD };
    for (const key of Object.keys(keys)) if (typeof s.keys?.[key] === 'string' && /^[A-Za-z][A-Za-z0-9]{0,23}$/.test(s.keys[key])) keys[key] = s.keys[key];
    if (new Set(Object.values(keys)).size !== Object.keys(keys).length) Object.assign(keys, DEFAULT_KEYS);
    for (const key of Object.keys(pad)) if (Number.isInteger(s.pad?.[key]) && s.pad[key] >= 0 && s.pad[key] <= 31) pad[key] = s.pad[key];
    if (new Set(Object.values(pad)).size !== Object.keys(pad).length) Object.assign(pad, DEFAULT_PAD);
    return { quality: ['low', 'medium', 'high'].includes(s.quality) ? s.quality : 'high', effects: clamp(finite(s.effects, .65), 0, 1), ambience: clamp(finite(s.ambience, .3), 0, 1), reduced: s.reduced === true, keys, pad };
}

export function normalizeScores(value) {
    const out = {};
    for (const table of TABLES) out[table.id] = (Array.isArray(value?.[table.id]) ? value[table.id] : [])
        .filter(row => row && typeof row.name === 'string' && Number.isSafeInteger(row.score) && row.score >= 0 && Number.isFinite(row.date))
        .map(row => ({ name: Array.from(row.name).slice(0, 24).join(''), score: row.score, date: row.date }))
        .sort((a, b) => b.score - a.score).slice(0, 10);
    return out;
}
