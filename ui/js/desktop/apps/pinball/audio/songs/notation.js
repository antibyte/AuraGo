// Tiny score-writing helpers for the pinball songs. DOM-free, deterministic, pure data out.
// Event = [step16, midi, lenSteps, vel]. A bar is 16 sixteenth-note steps.
const SEMI = { C: 0, D: 2, E: 4, F: 5, G: 7, A: 9, B: 11 };
const DRUM_VEL = { X: 1, x: 0.75, o: 0.45, g: 0.28 };

export function midi(name) {
    if (typeof name === 'number') return name;
    const m = /^([A-G])([#b]?)(-?\d)$/.exec(name);
    if (!m) throw new Error('bad note name: ' + name);
    return 12 * (Number(m[3]) + 1) + SEMI[m[1]] + (m[2] === '#' ? 1 : m[2] === 'b' ? -1 : 0);
}
const r2 = (v) => Math.round(v * 100) / 100;

// Drum grid, one char per step: X loud, x medium, o soft, g ghost, '.' rest (any length, bars concatenate).
export function drum(grid, { vel = 1, note = 60, len = 1 } = {}) {
    const out = [];
    for (let i = 0; i < grid.length; i++) {
        const v = DRUM_VEL[grid[i]];
        if (v) out.push([i, note, len, r2(Math.min(1, v * vel))]);
    }
    return out;
}

// Melody string: tokens 'A4:2' (note:len), '.:3' rest, suffix ^ accent, ~ soft. Default len 1.
export function melody(str, { vel = 0.8, start = 0 } = {}) {
    const out = [];
    let step = start;
    for (const tok of String(str).trim().split(/\s+/)) {
        if (!tok) continue;
        const m = /^(\.|[A-G][#b]?-?\d)(?::(\d+))?([\^~])?$/.exec(tok);
        if (!m) throw new Error('bad melody token: ' + tok);
        const len = m[2] ? Number(m[2]) : 1;
        if (m[1] !== '.') out.push([step, midi(m[1]), len, r2(Math.min(1, vel * (m[3] === '^' ? 1.2 : m[3] === '~' ? 0.7 : 1)))]);
        step += len;
    }
    return out;
}

// Arpeggio: order is a string of indexes into notes ('.' rests), cycled over `steps` 16ths.
export function arp(notes, order, { start = 0, len = 1, vel = 0.7, steps = 16 } = {}) {
    const ns = notes.map(midi);
    const out = [];
    for (let i = 0; i < steps; i++) {
        const ch = order[i % order.length];
        if (ch === '.') continue;
        out.push([start + i, ns[Number(ch)], len, r2(vel * (i % 4 === 0 ? 1 : 0.8))]);
    }
    return out;
}

// Sustained chord (one event per note).
export function chord(notes, { step = 0, len = 16, vel = 0.5 } = {}) {
    return notes.map((n) => [step, midi(n), len, vel]);
}

// Chord stabs on the 'x' positions of a grid.
export function stabs(notes, grid, { len = 1, vel = 0.7 } = {}) {
    const out = [];
    for (let i = 0; i < grid.length; i++) if (grid[i] !== '.') for (const n of notes) out.push([i, midi(n), len, vel]);
    return out;
}

export function shift(events, steps) { return events.map((e) => [e[0] + steps, e[1], e[2], e[3]]); }

// One entry per bar -> concatenated with a 16 step offset each. items may be arrays of events or any value fn maps.
export function over(items, fn = (x) => x) {
    const out = [];
    items.forEach((item, i) => { for (const e of shift(fn(item, i), 16 * i)) out.push(e); });
    return out;
}

export function merge(...lists) { return [].concat(...lists); }
export function pat(bars, tracks) { return { bars, tracks }; }

export function transpose(events, semis) { return events.map((e) => [e[0], e[1] + semis, e[2], e[3]]); }

// Repeat one bar's events `n` times.
export function rep(events, n) { return over(new Array(n).fill(0), () => events); }

// Stinger table -> { patterns: {st_<cue>: pattern}, map: {cue: 'st_<cue>'} }. Each def is { track: events }.
export function stingerSet(defs) {
    const patterns = {}, map = {};
    for (const cue of Object.keys(defs)) {
        patterns['st_' + cue] = pat(1, defs[cue]);
        map[cue] = 'st_' + cue;
    }
    return { patterns, map };
}
