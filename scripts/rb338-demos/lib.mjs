// SPDX-License-Identifier: GPL-3.0-only
// Shared tools of the RB-338 demo songs: a node host for the vendored OpenRB-338 wasm, the
// pattern notation, the arrangement/automation compiler and an RBS container patcher.
//
// Authoring path: the engine writes the song file (devices, patterns, mixer, globals) from the
// messages the panel would send; the song-mode TRAK tracks (pattern changes and automation) are
// then written straight into the RBS container (the format of docs/RBS42.txt: a position is a
// tick / 24 delta in MIDI style) and the result is loaded and saved once more by the engine, so
// the shipped bytes are the engine's canonical form of the song. Nothing is recorded in real time.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

export const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const VENDOR = path.join(ROOT, 'ui/js/vendor/rb338');

// The UMD model is loaded as a CommonJS function body (the package is "type": "module").
function loadUmd(file) {
    const mod = { exports: {} };
    new Function('module', 'exports', fs.readFileSync(file, 'utf8'))(mod, mod.exports);
    return mod.exports;
}
export const M = loadUmd(path.join(ROOT, 'ui/js/desktop/apps/synth-studio-rb338-model.js'));

export const BAR = M.BAR; // 768 ticks
export const TICK_UNIT = 24; // a track position is tick / 24 (the 32nd note)

// ---- the engine in node -----------------------------------------------------------------------
export async function openEngine() {
    const module = await WebAssembly.compile(fs.readFileSync(path.join(VENDOR, 'rb338-engine.wasm')));
    const x = (await WebAssembly.instantiate(module, {})).exports;
    if (x.orb_abi_version() !== 1) throw new Error('unexpected RB-338 ABI version');
    const u8 = () => new Uint8Array(x.memory.buffer);
    const put = (bytes) => { const p = x.orb_alloc(bytes.length); u8().set(bytes, p); return p; };
    const kit = fs.readFileSync(path.join(VENDOR, 'Standard.rbm'));
    const kp = put(kit);
    const h = x.orb_create(kp, kit.length);
    x.orb_free(kp, kit.length);
    if (!h) throw new Error('orb_create failed, error ' + x.orb_last_error());
    let seq = 0;
    const io = () => Buffer.from(u8().slice(x.orb_io_ptr(h), x.orb_io_ptr(h) + x.orb_io_len(h)));
    return {
        x, h,
        newSong() { x.orb_new_song(h); },
        // messages: arrays of wire bytes
        push(messages) {
            const b = M.batch(messages);
            const p = put(b);
            const r = x.orb_push(h, p, b.length, ++seq);
            x.orb_free(p, b.length);
            if (r !== 0) throw new Error('orb_push refused');
            x.orb_apply(h);
        },
        load(bytes) {
            const p = put(bytes);
            const r = x.orb_load_song(h, p, bytes.length);
            x.orb_free(p, bytes.length);
            return r;
        },
        save() {
            const r = x.orb_save_song(h);
            if (r !== 0) throw new Error('orb_save_song failed: ' + r);
            return io();
        },
        info() { x.orb_song_info(h); return JSON.parse(io().toString('utf8')); },
        // song-mode WAV of a song file; returns { wav, pcm (Int16Array interleaved) }
        exportWav(bytes, idleBlocks = 9088) {
            const p = put(bytes);
            const r = x.orb_export_wav(h, p, bytes.length, 0, 0, idleBlocks);
            x.orb_free(p, bytes.length);
            if (r < 0 || r > 0) throw new Error('orb_export_wav failed: ' + r);
            const wav = io();
            const pcm = new Int16Array(wav.buffer.slice(wav.byteOffset + 44, wav.byteOffset + wav.length));
            return { wav, pcm };
        },
        destroy() { x.orb_destroy(h); }
    };
}

// ---- names ------------------------------------------------------------------------------------
const DEVICE_ALIAS = { t1: 'tb303_1', t2: 'tb303_2', d808: 'tr808', d909: 'tr909', delay: 'delay', dist: 'dist', pcf: 'pcf', comp: 'comp' };
export const ROWS = ['t1', 't2', 'd808', 'd909']; // the pattern devices, in orb_patterns order
const ROW_DEV = { t1: M.DEV.TB303_1, t2: M.DEV.TB303_2, d808: M.DEV.TR808, d909: M.DEV.TR909 };
const DEV_ALIAS = { t1: M.DEV.TB303_1, t2: M.DEV.TB303_2, d808: M.DEV.TR808, d909: M.DEV.TR909, delay: M.DEV.DELAY, dist: M.DEV.DIST, pcf: M.DEV.PCF, comp: M.DEV.COMP };
const MIX_CH = { ch1: 0, ch2: 1, ch3: 2, ch4: 3 };
const MIX_K = { on: 0, level: 1, pan: 2, send: 3, dist: 4 };

// A control address of the spec ('t1.cutoff', 'd909.bd_decay', 'mix.ch2.level', 'mix.master') as
// { id (model CONTROLS id), track (TRAK index), ctrl (TRAK controller), lo, hi, auto (false when
// the engine has no track controller for it) }.
export function resolve(address) {
    const parts = address.split('.');
    if (parts[0] === 'mix') {
        if (parts[1] === 'master') return { id: 'master.fader', track: 0, ctrl: null, lo: 0, hi: 127, auto: false };
        if (parts[1] === 'comp') return { id: 'mixer.comp_id', track: 0, ctrl: 1, lo: 0, hi: 5, auto: true };
        if (parts[1] === 'pcf') return { id: 'mixer.pcf_id', track: 0, ctrl: 2, lo: 0, hi: 4, auto: true };
        const c = MIX_CH[parts[1]];
        const k = MIX_K[parts[2]];
        if (c === undefined || k === undefined) throw new Error('unknown mixer address ' + address);
        const name = parts[2] === 'on' ? 'mute' : parts[2] === 'send' ? 'delay' : parts[2];
        return { id: 'mixer' + (c + 1) + '.' + name, track: 0, ctrl: k === 0 ? null : 5 + 6 * c + k, lo: 0, hi: k === 4 ? 1 : 127, auto: k !== 0 };
    }
    const prefix = DEVICE_ALIAS[parts[0]];
    if (!prefix) throw new Error('unknown device in ' + address);
    const c = M.control(prefix + '.' + parts[1]);
    if (!c || c.msg !== 'param') throw new Error('unknown control ' + address);
    return { id: c.id, track: c.dev, ctrl: c.ctrl, lo: c.min, hi: c.max, auto: true };
}

// ---- pattern notation -------------------------------------------------------------------------
const NOTE_NAMES = { 'C': 0, 'C#': 1, 'D': 2, 'D#': 3, 'E': 4, 'F': 5, 'F#': 6, 'G': 7, 'G#': 8, 'A': 9, 'A#': 10, 'B': 11, "C'": 12 };
// A 303 line: 16 whitespace separated steps. '.' is a rest. A step is a note (C C# D D# E F F# G
// G# A A# B, C' is the high C) followed by any of: '^' octave up, 'v' octave down, '>' accent,
// '~' slide (the step glides into the next one). Returns the 98 pattern bytes.
export function parse303(line, opts = {}) {
    const toks = line.trim().split(/\s+/);
    const len = opts.length || 16;
    if (toks.length !== len) throw new Error('303 line needs ' + len + ' steps, got ' + toks.length + ': ' + line);
    const out = new Uint8Array(98);
    out[0] = opts.shuffle ? 1 : 0;
    out[1] = len;
    for (let s = 0; s < 16; s++) {
        const o = 2 + 6 * s;
        out.set(M.CLEAR_303_STEP, o); // an unused step is an empty rest
        if (s >= len) continue;
        const t = toks[s];
        if (t === '.') continue;
        const m = /^([A-G]#?|C')([\^v>~]*)$/.exec(t);
        if (!m) throw new Error('bad 303 step "' + t + '" in: ' + line);
        const flags = m[2];
        out[o] = NOTE_NAMES[m[1]];
        out[o + 1] = flags.includes('~') ? 1 : 0;
        out[o + 2] = flags.includes('>') ? 1 : 0;
        out[o + 3] = flags.includes('^') ? 1 : 0;
        out[o + 4] = flags.includes('v') ? 1 : 0;
        out[o + 5] = 0;
    }
    return out;
}

const INST = {
    d808: ['AC', 'BD', 'SD', 'LT', 'MT', 'HT', 'RS', 'CP', 'CB', 'CY', 'OH', 'CH'],
    d909: ['AC', 'BD', 'SD', 'LT', 'MT', 'HT', 'RS', 'CP', 'CH', 'OH', 'CC', 'RC']
};
// A drum pattern: { BD: 'x...x...x...x...', ... } with one 16 character string per instrument.
// 'x' a hit, 'X' an accented hit (909: value 2), 'f' a flam (909 only), '.' or '-' nothing.
// Returns the 194 pattern bytes.
export function parseDrums(row, lines, opts = {}) {
    const names = INST[row];
    const out = new Uint8Array(194);
    out[0] = opts.shuffle ? 1 : 0;
    out[1] = opts.length || 16;
    for (const [inst, str] of Object.entries(lines)) {
        const i = names.indexOf(inst);
        if (i < 0) throw new Error('unknown ' + row + ' instrument ' + inst);
        const chars = str.replace(/\s+/g, '');
        if (chars.length !== 16) throw new Error(row + ' ' + inst + ' needs 16 steps: ' + str);
        for (let s = 0; s < 16; s++) {
            const ch = chars[s];
            let v = 0;
            if (ch === 'x') v = 1;
            else if (ch === 'X') v = row === 'd909' ? 2 : 1;
            else if (ch === 'f') { if (row !== 'd909') throw new Error('flam is 909 only'); v = 3; }
            else if (ch !== '.' && ch !== '-') throw new Error('bad drum char ' + ch + ' in ' + str);
            out[2 + 12 * s + i] = v;
        }
    }
    if (row === 'd909') {
        for (let s = 0; s < 16; s++) {
            if (out[2 + 12 * s + 8] && out[2 + 12 * s + 9]) throw new Error('909 closed and open hat on step ' + s);
        }
    }
    return out;
}

// ---- the song compiler ------------------------------------------------------------------------
const SILENT = '-';

// Pattern names of a row in order of first use; slot 31 stays empty (the silent pattern).
function allocatePatterns(spec) {
    const slots = {};
    for (const row of ROWS) {
        const defs = (spec.patterns && spec.patterns[row]) || {};
        const names = Object.keys(defs);
        if (names.length > 31) throw new Error(row + ': at most 31 patterns');
        slots[row] = new Map(names.map((n, i) => [n, i]));
    }
    return slots;
}

function patternBytes(spec, row, def) {
    const text = typeof def === 'string' ? { steps: def } : def;
    if (row === 't1' || row === 't2') return parse303(text.steps, text);
    return text.steps ? parseDrums(row, text.steps, text) : parseDrums(row, text, {});
}

// 'a b*3 (c d)*2' -> a b b b c d c d; '-' is silent
function expandSeq(seq) {
    if (Array.isArray(seq)) seq = seq.join(' ');
    const toks = String(seq).replace(/\(/g, ' ( ').replace(/\)(\*\d+)?/g, (m, n) => ' )' + (n || '') + ' ').trim().split(/\s+/);
    let i = 0;
    const parse = () => {
        const out = [];
        while (i < toks.length) {
            const t = toks[i++];
            if (t === '(') {
                const inner = parse();
                const close = toks[i - 1];
                const m = /^\)(?:\*(\d+))?$/.exec(close);
                if (!m) throw new Error('unbalanced group in: ' + seq);
                for (let k = 0; k < (m[1] ? Number(m[1]) : 1); k++) out.push(...inner);
            } else if (t.startsWith(')')) {
                return out;
            } else {
                const m = /^(.+?)(?:\*(\d+))?$/.exec(t);
                for (let k = 0; k < (m[2] ? Number(m[2]) : 1); k++) out.push(m[1]);
            }
        }
        return out;
    };
    return parse();
}

function clamp(v, lo, hi) { return Math.max(lo, Math.min(hi, v)); }

const CURVES = {
    lin: (t) => t,
    exp: (t) => t * t,
    log: (t) => Math.sqrt(t),
    smooth: (t) => t * t * (3 - 2 * t),
    // the engine has no value smoothing worth speaking of: a step jump is what the knob does
    step: (t) => (t >= 1 ? 1 : 0)
};

// The track events of a spec: { track -> [{ tick, ctrl, value }] }, plus the song length in bars.
// A track position is a multiple of 24 ticks. Events of one tick of a track would be saved in the
// reverse order by every load and save of the engine, so no two events of a track share a tick
// (except at tick 0, where the engine's own start events are replaced in place): ramps that run
// together on one track interleave on a coarser grid and any other clash moves one 24 ticks on.
export function compileTracks(spec, slots) {
    const events = Array.from({ length: 9 }, () => []);
    const add = (track, tick, ctrl, value) => events[track].push({ tick: Math.round(tick), ctrl, value: Math.round(value) });
    const range = (r, v, what) => {
        if (!Number.isFinite(v) || v < r.lo || v > r.hi) throw new Error(what + ': value ' + v + ' out of ' + r.lo + '..' + r.hi);
    };
    let bar0 = 0;
    for (const sec of spec.arrangement) {
        if (!(sec.bars >= 1)) throw new Error('section ' + sec.name + ': bars');
        // pattern changes
        for (const row of ROWS) {
            const seq = sec[row] === undefined ? SILENT : sec[row];
            const names = expandSeq(seq);
            if (!names.length || sec.bars % names.length) throw new Error('section ' + sec.name + ' ' + row + ': ' + names.length + ' patterns do not fit ' + sec.bars + ' bars');
            for (let b = 0; b < sec.bars; b++) {
                const n = names[b % names.length];
                let slot;
                if (n === SILENT) slot = 31;
                else if (slots[row].has(n)) slot = slots[row].get(n);
                else throw new Error('section ' + sec.name + ': unknown ' + row + ' pattern ' + n);
                const key = row + ':last';
                if (compileTracks.last[key] !== slot) {
                    add(ROW_DEV[row], (bar0 + b) * BAR, 1, slot);
                    compileTracks.last[key] = slot;
                }
            }
        }
        // static settings at the start of the section, and at bar offsets
        const sets = [];
        if (sec.set) sets.push({ bar: 0, set: sec.set });
        for (const a of sec.at || []) sets.push(a);
        for (const s of sets) {
            for (const [addr, v] of Object.entries(s.set)) {
                const r = resolve(addr);
                if (!r.auto) throw new Error(addr + ' cannot be changed inside a song');
                range(r, v, addr);
                add(r.track, (bar0 + s.bar) * BAR, r.ctrl, v);
            }
        }
        // ramps: { to, points: [[bar, value], ...], curve } or { to, hold: [[bar, value], ...] }
        // or { to, lfo: { center, depth, period, from, bars, phase, wave } }. Ramps of one track
        // that run in the same section take turns on a grid of 24 ticks times their number.
        const ramps = (sec.auto || []).map((a) => ({ a, r: resolve(a.to) }));
        const perTrack = new Map();
        for (const x of ramps) {
            if (!x.r.auto) throw new Error(x.a.to + ' cannot be automated');
            const n = perTrack.get(x.r.track) || 0;
            x.slot = n;
            perTrack.set(x.r.track, n + 1);
        }
        for (const { a, r, slot } of ramps) {
            const res = TICK_UNIT * perTrack.get(r.track);
            const phase = TICK_UNIT * slot;
            let last = null;
            const emit = (tickRel, v) => {
                const val = clamp(Math.round(v), r.lo, r.hi);
                if (val !== last) { add(r.track, bar0 * BAR + tickRel, r.ctrl, val); last = val; }
            };
            // the grid ticks (relative to the section) in [t0, t1]
            const grid = (t0, t1) => {
                const out = [];
                for (let t = Math.ceil((t0 - phase) / res) * res + phase; t <= t1; t += res) out.push(t);
                return out;
            };
            if (a.points) {
                const pts = a.points;
                const curve = CURVES[a.curve || 'lin'];
                for (const p of pts) range(r, p[1], a.to);
                emit(Math.round(pts[0][0] * BAR), pts[0][1]);
                for (let i = 0; i + 1 < pts.length; i++) {
                    const [b0, v0] = pts[i];
                    const [b1, v1] = pts[i + 1];
                    const t0 = b0 * BAR, t1 = b1 * BAR;
                    for (const t of grid(t0 + 1, t1)) emit(t, v0 + (v1 - v0) * curve((t - t0) / (t1 - t0)));
                }
            } else if (a.hold) {
                for (const [bar, v] of a.hold) { range(r, v, a.to); emit(Math.round(bar * BAR), v); }
            } else if (a.lfo) {
                const l = a.lfo;
                const from = (l.from || 0) * BAR;
                for (const t of grid(from, from + l.bars * BAR)) {
                    const ph = ((t - from) / BAR / l.period + (l.phase || 0)) * Math.PI * 2;
                    const w = l.wave === 'tri' ? (2 / Math.PI) * Math.asin(Math.sin(ph)) : Math.sin(ph);
                    emit(t, l.center + l.depth * w);
                }
            } else {
                throw new Error('auto of ' + a.to + ' needs points, hold or lfo');
            }
        }
        bar0 += sec.bars;
    }
    // one event per tick and track
    for (let track = 0; track < events.length; track++) {
        const patternTrack = track >= 1 && track <= 4;
        const all = events[track].map((e, i) => ({ ...e, i }));
        const used = new Set(all.filter((e) => patternTrack && e.ctrl === 1).map((e) => e.tick));
        const out = [];
        // the last event wins at one tick of one controller
        const seen = new Map();
        for (const e of all) seen.set(e.tick + ':' + e.ctrl, e);
        const rest = [...seen.values()].sort((x, y) => x.tick - y.tick || x.i - y.i);
        for (const e of rest) {
            let t = e.tick;
            if (t > 0 && !(patternTrack && e.ctrl === 1)) {
                while (used.has(t)) t += TICK_UNIT;
                used.add(t);
            }
            out.push({ tick: t, ctrl: e.ctrl, value: e.value });
        }
        events[track] = out;
    }
    return { events, bars: bar0 };
}
compileTracks.last = {};

// ---- RBS container ----------------------------------------------------------------------------
function readChunks(buf, pos, end) {
    const out = [];
    while (pos < end) {
        const tag = buf.toString('latin1', pos, pos + 4);
        const size = buf.readUInt32BE(pos + 4);
        out.push({ tag, start: pos + 8, size, body: buf.subarray(pos + 8, pos + 8 + size) });
        pos += 8 + size + (size & 1);
    }
    return out;
}
function chunk(tag, body) {
    const pad = body.length & 1 ? 1 : 0;
    const out = Buffer.alloc(8 + body.length + pad);
    out.write(tag, 0, 'latin1');
    out.writeUInt32BE(body.length, 4);
    body.copy(out, 8);
    return out;
}
const cat = (kind, children) => chunk('CAT ', Buffer.concat([Buffer.from(kind, 'latin1'), ...children]));

function decodeTrak(body) {
    const n = body.readUInt32BE(0);
    const out = [];
    let p = 4, pos = 0;
    for (let i = 0; i < n; i++) {
        let v = 0;
        for (;;) { const c = body[p++]; v = (v << 7) | (c & 0x7f); if (!(c & 0x80)) break; }
        pos += v;
        out.push({ tick: pos * TICK_UNIT, ctrl: body[p], value: body[p + 1] });
        p += 2;
    }
    return out;
}
function encodeTrak(events) {
    const bytes = [];
    let prev = 0;
    for (const e of events) {
        const pos = e.tick / TICK_UNIT;
        let d = pos - prev;
        prev = pos;
        const groups = [d & 0x7f];
        d >>>= 7;
        while (d) { groups.push(0x80 | (d & 0x7f)); d >>>= 7; }
        bytes.push(...groups.reverse(), e.ctrl, e.value);
    }
    const head = Buffer.alloc(4);
    head.writeUInt32BE(events.length, 0);
    return Buffer.concat([head, Buffer.from(bytes)]);
}

function putZ(buf, off, size, text) {
    if (!/^[\x20-\x7e\r]*$/.test(text)) throw new Error('song info must be ASCII');
    if (text.length > size - 1) throw new Error('song info too long (' + text.length + ' > ' + (size - 1) + '): ' + text);
    buf.fill(0, off, off + size);
    buf.write(text, off, 'latin1');
}

// Rewrites the song-mode parts of an engine-saved file: the globals, the song information and the
// tracks. `trackEvents` are merged over the file's own events (the constructor's tick-0 events).
export function patchSong(file, { songMode, loopOn, loopStart, loopEnd, title, text, trackEvents }) {
    const top = readChunks(file, 0, file.length)[0];
    const items = readChunks(file, top.start + 4, top.start + top.size);
    const outItems = items.map((c) => {
        if (c.tag === 'GLOB') {
            const g = Buffer.from(c.body);
            g[0] = songMode ? 1 : 0;
            g[1] = loopOn ? 1 : 0;
            g.writeUInt32BE(loopStart, 6);
            g.writeUInt32BE(loopEnd, 10);
            return chunk('GLOB', g);
        }
        if (c.tag === 'USRI') {
            const u = Buffer.from(c.body);
            putZ(u, 0, 41, title || '');
            putZ(u, 41, 401, (text || '').replace(/\n/g, '\r'));
            return chunk('USRI', u);
        }
        if (c.tag === 'CAT ' && c.body.toString('latin1', 0, 4) === 'TRKL') {
            const traks = readChunks(c.body, 4, c.body.length);
            const rebuilt = traks.map((t, i) => {
                const have = decodeTrak(t.body);
                const byKey = new Map(have.map((e) => [e.tick + ':' + e.ctrl, e]));
                for (const e of trackEvents[i] || []) byKey.set(e.tick + ':' + e.ctrl, e);
                // events of one tick keep their insertion order, then the controller number
                const all = [...byKey.values()].sort((a, b) => a.tick - b.tick || a.ctrl - b.ctrl);
                return chunk('TRAK', encodeTrak(all));
            });
            return cat('TRKL', rebuilt);
        }
        return chunk(c.tag, c.body);
    });
    return cat('RB40', outItems);
}

// ---- building a song --------------------------------------------------------------------------
export async function buildSong(spec, engine) {
    compileTracks.last = {};
    const slots = allocatePatterns(spec);
    const { events, bars } = compileTracks(spec, slots);
    const msgs = [];
    const send = (m) => msgs.push(m);
    // the song-mode state at tick 0: a knob that a track can move takes its start value from a tick-0 event
    for (const [addr, v] of Object.entries(spec.start || {})) {
        const r = resolve(addr);
        if (r.auto) events[r.track].unshift({ tick: 0, ctrl: r.ctrl, value: v });
    }
    engine.newSong();
    send(M.encodeGlobal(4, Math.round(spec.bpm * 1000)));
    send(M.encodeGlobal(20, spec.shuffle || 0));
    // knob and mixer start values: the song-mode tick-0 state is taken from these
    for (const [addr, v] of Object.entries(spec.start || {})) {
        const r = resolve(addr);
        const m = M.encodeControl(r.id, v, 0);
        if (!m) throw new Error('cannot set ' + addr);
        if (v < r.lo || v > r.hi) throw new Error(addr + ' out of range');
        send(m);
    }
    // patterns
    for (const row of ROWS) {
        const defs = (spec.patterns && spec.patterns[row]) || {};
        for (const [name, def] of Object.entries(defs)) {
            const p = slots[row].get(name);
            send(M.encodeDevice({ type: 'pattern', dev: ROW_DEV[row], p, bytes: patternBytes(spec, row, def) }));
        }
        // the pattern the song opens with
    }
    engine.push(msgs);
    const base = engine.save();
    const patched = patchSong(base, {
        songMode: true, loopOn: true, loopStart: 0, loopEnd: bars * BAR,
        title: spec.title, text: spec.text, trackEvents: events
    });
    const code = engine.load(patched);
    if (code !== 0) throw new Error(spec.slug + ': engine refused the song, code ' + code);
    const canonical = Buffer.from(engine.save());
    return { bytes: canonical, bars, seconds: bars * 4 * 60 / spec.bpm };
}

// ---- analysis ---------------------------------------------------------------------------------
export function analysePcm(pcm) {
    let peak = 0, sum = 0;
    for (const v of pcm) { const a = Math.abs(v); if (a > peak) peak = a; sum += v * v; }
    const rms = Math.sqrt(sum / pcm.length);
    const db = (x) => (x > 0 ? 20 * Math.log10(x / 32768) : -Infinity);
    return { peak: peak / 32768, peakDb: db(peak), rmsDb: db(rms), frames: pcm.length / 2, seconds: pcm.length / 2 / 44100 };
}
// RMS dBFS per window of `seconds`
export function windowRms(pcm, seconds) {
    const n = Math.round(seconds * 44100) * 2;
    const out = [];
    for (let o = 0; o < pcm.length; o += n) {
        let s = 0;
        const e = Math.min(pcm.length, o + n);
        for (let i = o; i < e; i++) s += pcm[i] * pcm[i];
        const r = Math.sqrt(s / (e - o));
        out.push(r > 0 ? 20 * Math.log10(r / 32768) : -Infinity);
    }
    return out;
}

// ---- BS.1770 loudness and true peak -----------------------------------------------------------
function kWeight(x, fs = 44100) {
    // stage 1: high shelf, stage 2: high pass (BS.1770-4 filter parameters, bilinear transform)
    const shelf = (() => {
        const f0 = 1681.974450955533, G = 3.999843853973347, Q = 0.7071752369554196;
        const K = Math.tan(Math.PI * f0 / fs), Vh = 10 ** (G / 20), Vb = Vh ** 0.4996667741545416;
        const a0 = 1 + K / Q + K * K;
        return { b: [(Vh + Vb * K / Q + K * K) / a0, 2 * (K * K - Vh) / a0, (Vh - Vb * K / Q + K * K) / a0], a: [2 * (K * K - 1) / a0, (1 - K / Q + K * K) / a0] };
    })();
    const hp = (() => {
        const f0 = 38.13547087602444, Q = 0.5003270373238773;
        const K = Math.tan(Math.PI * f0 / fs), a0 = 1 + K / Q + K * K;
        return { b: [1, -2, 1], a: [2 * (K * K - 1) / a0, (1 - K / Q + K * K) / a0] };
    })();
    let y = x;
    for (const f of [shelf, hp]) {
        const o = new Float64Array(y.length);
        let x1 = 0, x2 = 0, y1 = 0, y2 = 0;
        for (let i = 0; i < y.length; i++) {
            const v = f.b[0] * y[i] + f.b[1] * x1 + f.b[2] * x2 - f.a[0] * y1 - f.a[1] * y2;
            x2 = x1; x1 = y[i]; y2 = y1; y1 = v; o[i] = v;
        }
        y = o;
    }
    return y;
}

// Integrated loudness in LUFS (BS.1770-4 with the -70 LUFS and -10 LU gates) of interleaved int16 stereo.
export function integratedLufs(pcm) {
    const n = pcm.length / 2;
    const ch = [new Float64Array(n), new Float64Array(n)];
    for (let i = 0; i < n; i++) { ch[0][i] = pcm[2 * i] / 32768; ch[1][i] = pcm[2 * i + 1] / 32768; }
    const k = ch.map((c) => kWeight(c));
    const block = Math.round(0.4 * 44100), hop = Math.round(0.1 * 44100);
    const energies = [];
    for (let s = 0; s + block <= n; s += hop) {
        let e = 0;
        for (const c of k) { let sum = 0; for (let i = s; i < s + block; i++) sum += c[i] * c[i]; e += sum / block; }
        energies.push(e);
    }
    const lufs = (e) => -0.691 + 10 * Math.log10(e);
    const abs = energies.filter((e) => e > 0 && lufs(e) > -70);
    if (!abs.length) return -Infinity;
    const rel = lufs(abs.reduce((a, b) => a + b, 0) / abs.length) - 10;
    const gated = abs.filter((e) => lufs(e) > rel);
    return lufs(gated.reduce((a, b) => a + b, 0) / gated.length);
}

// True peak in dBTP: 4x oversampling with a Hann-windowed sinc (24 taps per phase) on the
// samples that can matter (above half of the sample peak).
export function truePeakDb(pcm) {
    const L = 12; // taps each side
    const phases = [0.25, 0.5, 0.75].map((f) => {
        const h = new Float64Array(2 * L);
        for (let j = -L + 1; j <= L; j++) {
            const t = j - f;
            const sinc = Math.abs(t) < 1e-9 ? 1 : Math.sin(Math.PI * t) / (Math.PI * t);
            h[j + L - 1] = sinc * (0.5 + 0.5 * Math.cos(Math.PI * t / (L + 0.5)));
        }
        return h;
    });
    let sp = 0;
    for (const v of pcm) sp = Math.max(sp, Math.abs(v));
    let peak = sp;
    const n = pcm.length / 2;
    for (let c = 0; c < 2; c++) {
        for (let i = L; i < n - L; i++) {
            if (Math.abs(pcm[2 * i + c]) < sp * 0.5 && Math.abs(pcm[2 * (i + 1) + c]) < sp * 0.5) continue;
            for (const h of phases) {
                let s = 0;
                for (let j = -L + 1; j <= L; j++) s += pcm[2 * (i + j) + c] * h[j + L - 1];
                if (Math.abs(s) > peak) peak = Math.abs(s);
            }
        }
    }
    return 20 * Math.log10(peak / 32768);
}
