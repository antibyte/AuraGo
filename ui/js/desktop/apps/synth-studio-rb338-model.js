// SPDX-License-Identifier: GPL-3.0-only
// Derived from OpenRB-338 (commit 0bebfcd), Copyright (C) 2026 the OpenRB-338 authors.
// Pure logic of the RB-338 panel: wire messages, snapshot decoding, control table and the
// ported OpenRB UI rules (orb-ui model.rs, behavior.rs, link.rs). No DOM, no audio.
(function (root, factory) {
    const api = factory(root);
    if (typeof module === 'object' && module.exports) module.exports = api;
    else root.SynthStudioRB338Model = api;
})(typeof globalThis !== 'undefined' ? globalThis : window, function () {
    'use strict';

    // ---- devices and wire subs ---------------------------------------------------------------
    const DEV = Object.freeze({ MIXER: 0, TB303_1: 1, TB303_2: 2, TR808: 3, TR909: 4, DELAY: 5, DIST: 6, PCF: 7, COMP: 8 });
    const PARAM_SUB = Object.freeze({ 1: 40, 2: 40, 3: 57, 4: 74, 5: 84, 6: 92, 7: 114, 8: 102 });
    const HEADER_SUB = Object.freeze({ 1: 42, 2: 42, 3: 59, 4: 76 });
    const PATTERN_SUB = Object.freeze({ 1: 44, 2: 44, 3: 61, 4: 78 });
    const STEP_SUB = Object.freeze({ 1: 45, 2: 45, 3: 62, 4: 79 });
    const MIXER_GLOBAL_SUB = 145;
    const MIXER_CHANNEL_SUB = 147;

    // ---- transport constants -----------------------------------------------------------------
    const BAR = 768;
    const MAX_POS = 766464;
    const MAX_END = 767232;
    const TEMPO_MIN = 20000;
    const TEMPO_MAX = 500000;

    // ---- snapshot v1 layout ------------------------------------------------------------------
    const SNAPSHOT_SIZE = 192;
    const SNAP = Object.freeze({
        VERSION: 0, FLAGS: 1, SHUFFLE: 2, APPLIED_SEQ: 4, FRAMES: 8, TICK: 16,
        LOOP_START: 20, LOOP_END: 24, TEMPO: 28, GENERATION: 32, TRACKS_REV: 36, MERGES: 40,
        PLAY_STEP: 44, PLAYING_PATTERN: 48, CHANNEL_METERS: 52, MASTER_METERS: 56, COMP_GAIN: 58,
        EFFECT_METERS: 59, MIXER: 64,
        CTRL: Object.freeze({ 1: 88, 2: 97, 3: 106, 4: 135, 5: 165, 6: 170, 7: 173, 8: 180 })
    });
    const CTRL_COUNT = Object.freeze({ 1: 9, 2: 9, 3: 29, 4: 30, 5: 5, 6: 3, 7: 7, 8: 3 });
    const FLAG = Object.freeze({ PLAYING: 1, RECORDING: 2, SONG_MODE: 4, LOOP_ON: 8, VINTAGE: 16, FAULTED: 32, PREVIEW: 64 });
    const CTRL_KEYS = Object.freeze({ 1: 'tb303_1', 2: 'tb303_2', 3: 'tr808', 4: 'tr909', 5: 'delay', 6: 'dist', 7: 'pcf', 8: 'comp' });

    // ---- pattern byte layouts (orb_patterns buffer, link.rs layout()) ------------------------
    // Per row: 2 header bytes (shuffle, length) then 16 steps; a 303 step is 6 bytes
    // (note, slide, accent, up, down, rest), a drum step 12 bytes (one per instrument).
    const PATTERN_COUNT = 32;
    const ROW_PATTERN = Object.freeze([
        { plen: 98, slen: 6, base: 0 },
        { plen: 98, slen: 6, base: 3136 },
        { plen: 194, slen: 12, base: 6272 },
        { plen: 194, slen: 12, base: 12480 }
    ]);
    const PATTERNS_SIZE = 18688;
    const CLEAR_303_STEP = Object.freeze([0, 0, 0, 0, 0, 1]);

    function patternOffset(row, p) { return ROW_PATTERN[row].base + ROW_PATTERN[row].plen * p; }
    function patternBytes(buf, row, p) {
        const o = patternOffset(row, p);
        return Uint8Array.from(buf.subarray ? buf.subarray(o, o + ROW_PATTERN[row].plen) : buf.slice(o, o + ROW_PATTERN[row].plen));
    }
    function stepIndex(row, step, field) { return 2 + ROW_PATTERN[row].slen * step + field; }
    function stepField(buf, row, p, step, field) { return buf[patternOffset(row, p) + stepIndex(row, step, field)]; }
    function setStepField(buf, row, p, step, field, value) { buf[patternOffset(row, p) + stepIndex(row, step, field)] = value; }

    function isEmptyPattern(row, pattern) {
        const slen = ROW_PATTERN[row].slen;
        for (let s = 2; s < pattern.length; s += slen) {
            for (let i = 0; i < slen; i++) {
                if (pattern[s + i] !== (row < 2 ? CLEAR_303_STEP[i] : 0)) return false;
            }
        }
        return true;
    }
    // Steps emptied; length and shuffle stay (303: note 0, flags off, rest on).
    function emptiedPattern(row, pattern) {
        const p = Uint8Array.from(pattern);
        const slen = ROW_PATTERN[row].slen;
        for (let s = 2; s < p.length; s += slen) {
            for (let i = 0; i < slen; i++) p[s + i] = row < 2 ? CLEAR_303_STEP[i] : 0;
        }
        return p;
    }
    // First empty pattern after `from` (wrapping through 31 to 0) in the row's 32 patterns, or null.
    function nextEmptyPattern(row, patterns, from) {
        const plen = ROW_PATTERN[row].plen;
        for (let d = 1; d < PATTERN_COUNT; d++) {
            const p = (from + d) % PATTERN_COUNT;
            if (isEmptyPattern(row, patterns.slice(plen * p, plen * p + plen))) return p;
        }
        return null;
    }
    // Pattern button 1..8 of bank 0..3 -> pattern 0..31.
    function patternIndex(bank, button) { return (button - 1) + 8 * bank; }
    // Setting a hi-hat on a 909 step clears the other one (instruments 8 CH and 9 OH): the other, or -1.
    function otherHat909(inst, value) { return value !== 0 && (inst === 8 || inst === 9) ? 17 - inst : -1; }
    // Value a drum tap key sets on the playing step.
    function tapDrumValue(is808, inst) { return is808 ? 1 : (inst === 0 ? 2 : 1); }

    // ---- wire encoders -----------------------------------------------------------------------
    function u8(v) { return v & 255; }
    function encodeGlobal(id, value) {
        if (id === 4 || id === 8 || id === 12 || id === 16) {
            const v = value >>> 0;
            return [35, id, (v >>> 24) & 255, (v >>> 16) & 255, (v >>> 8) & 255, v & 255];
        }
        return [35, id, u8(value)];
    }
    // msg 41 variants. spec: {type:'param', dev, ctrl, value} | {type:'mixerGlobal', id, value}
    // | {type:'mixerChannel', ch, k, value} | {type:'patternField', dev, p, field, value}
    // | {type:'pattern', dev, p, bytes} | {type:'step', dev, p, step, field, value}
    function encodeDevice(spec) {
        switch (spec.type) {
        case 'param': return [41, spec.dev, PARAM_SUB[spec.dev], spec.ctrl, u8(spec.value)];
        case 'mixerGlobal': return [41, DEV.MIXER, MIXER_GLOBAL_SUB, spec.id, u8(spec.value)];
        case 'mixerChannel': return [41, DEV.MIXER, MIXER_CHANNEL_SUB, spec.ch, spec.k, u8(spec.value)];
        case 'patternField': return [41, spec.dev, HEADER_SUB[spec.dev], spec.p, spec.field, u8(spec.value)];
        case 'pattern': return [41, spec.dev, PATTERN_SUB[spec.dev], spec.p].concat(Array.from(spec.bytes));
        case 'step': return [41, spec.dev, STEP_SUB[spec.dev], spec.p, spec.step, spec.field, u8(spec.value)];
        default: throw new Error('unknown device message ' + spec.type);
        }
    }
    // Repeated u16 LE length + message bytes.
    function batch(messages) {
        let n = 0;
        for (const m of messages) n += 2 + m.length;
        const out = new Uint8Array(n);
        let o = 0;
        for (const m of messages) {
            out[o++] = m.length & 255;
            out[o++] = (m.length >> 8) & 255;
            out.set(m, o);
            o += m.length;
        }
        return out;
    }

    // ---- snapshot decoding -------------------------------------------------------------------
    function decodeSnapshot(source) {
        const bytes = source instanceof Uint8Array ? source : new Uint8Array(source);
        if (bytes.length < SNAPSHOT_SIZE) throw new Error('snapshot too short');
        const v = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
        const flags = bytes[SNAP.FLAGS];
        const arr = (o, n) => Array.from(bytes.subarray(o, o + n));
        const channels = [];
        for (let c = 0; c < 4; c++) {
            const o = SNAP.MIXER + 3 + 5 * c;
            channels.push({ mute: bytes[o], level: bytes[o + 1], pan: bytes[o + 2], send: bytes[o + 3], dist: bytes[o + 4] });
        }
        const ctrl = {};
        for (const d of Object.keys(SNAP.CTRL)) ctrl[CTRL_KEYS[d]] = arr(SNAP.CTRL[d], CTRL_COUNT[d]);
        return {
            version: bytes[SNAP.VERSION],
            flags,
            playing: !!(flags & FLAG.PLAYING),
            recording: !!(flags & FLAG.RECORDING),
            songMode: !!(flags & FLAG.SONG_MODE),
            loopOn: !!(flags & FLAG.LOOP_ON),
            vintage: !!(flags & FLAG.VINTAGE),
            faulted: !!(flags & FLAG.FAULTED),
            preview: !!(flags & FLAG.PREVIEW),
            shuffle: bytes[SNAP.SHUFFLE],
            appliedSeq: v.getUint32(SNAP.APPLIED_SEQ, true),
            frames: v.getFloat64(SNAP.FRAMES, true),
            tick: v.getInt32(SNAP.TICK, true),
            loopStart: v.getUint32(SNAP.LOOP_START, true),
            loopEnd: v.getUint32(SNAP.LOOP_END, true),
            tempo: v.getUint32(SNAP.TEMPO, true),
            generation: v.getUint32(SNAP.GENERATION, true),
            tracksRev: v.getUint32(SNAP.TRACKS_REV, true),
            merges: v.getUint32(SNAP.MERGES, true),
            playStep: arr(SNAP.PLAY_STEP, 4),
            playingPattern: arr(SNAP.PLAYING_PATTERN, 4),
            channelMeters: arr(SNAP.CHANNEL_METERS, 4),
            masterMeters: arr(SNAP.MASTER_METERS, 2),
            compGain: bytes[SNAP.COMP_GAIN],
            effectMeters: { delay: bytes[59], dist: bytes[60], pcf: bytes[61], comp: bytes[62] },
            mixer: { master: bytes[SNAP.MIXER], compId: bytes[SNAP.MIXER + 1], pcfId: bytes[SNAP.MIXER + 2], channels },
            ctrl
        };
    }

    // ---- controls table ----------------------------------------------------------------------
    // Entry: {id, dev, ctrl, kind, label, off, min, max, msg, ...}. `off` is the snapshot byte offset
    // (null when the value lives elsewhere). msg: param | mixerGlobal | mixerChannel | patternField | global.
    const CONTROLS = [];
    const BY_ID = Object.create(null);
    function add(id, kind, label, props) {
        const c = Object.assign({ id, kind, label, dev: null, ctrl: null, off: null, min: 0, max: kind === 'switch' || kind === 'toggle' ? 1 : 127 }, props);
        CONTROLS.push(c);
        BY_ID[id] = c;
        return c;
    }
    function param(dev, ctrl, name, kind, label, extra) {
        return add(CTRL_KEYS[dev] + '.' + name, kind, label, Object.assign({ dev, ctrl, msg: 'param', off: SNAP.CTRL[dev] + ctrl }, extra));
    }

    const ROW_LAYOUT = Object.freeze([
        { row: 0, dev: DEV.TB303_1, name: 'TB-303 #1', prefix: 'tb303_1', section: 'pattern1', channel: 0, ...ROW_PATTERN[0] },
        { row: 1, dev: DEV.TB303_2, name: 'TB-303 #2', prefix: 'tb303_2', section: 'pattern2', channel: 1, ...ROW_PATTERN[1] },
        { row: 2, dev: DEV.TR808, name: 'TR-808', prefix: 'tr808', section: 'pattern3', channel: 2, ...ROW_PATTERN[2] },
        { row: 3, dev: DEV.TR909, name: 'TR-909', prefix: 'tr909', section: 'pattern4', channel: 3, ...ROW_PATTERN[3] }
    ]);
    const COLUMN_LAYOUT = Object.freeze(['master', 'pcf', 'delay', 'dist', 'comp']);

    // transport (msg 35); value lives in the snapshot's flags/u32 fields, named by `field`
    add('transport.shuffle', 'knob', 'SHUFFLE', { msg: 'global', g: 20, field: 'shuffle', off: SNAP.SHUFFLE });
    add('transport.mode', 'switch', 'MODE', { msg: 'global', g: 0, field: 'songMode', off: SNAP.FLAGS });
    add('transport.record', 'toggle', 'RECORD', { msg: 'global', g: 2, field: 'recording', off: SNAP.FLAGS });
    add('transport.loop', 'switch', 'LOOP', { msg: 'global', g: 3, field: 'loopOn', off: SNAP.FLAGS });
    add('transport.tempo', 'display', 'TEMPO', { msg: 'global', g: 4, field: 'tempo', off: SNAP.TEMPO, min: TEMPO_MIN, max: TEMPO_MAX });
    add('transport.position', 'display', 'BAR', { msg: 'global', g: 8, field: 'tick', off: SNAP.TICK, min: 0, max: MAX_POS });
    add('transport.loop_start', 'display', 'LOOP START', { msg: 'global', g: 12, field: 'loopStart', off: SNAP.LOOP_START, min: 0, max: MAX_POS });
    add('transport.loop_end', 'display', 'LOOP LENGTH', { msg: 'global', g: 16, field: 'loopEnd', off: SNAP.LOOP_END, min: BAR, max: MAX_END });
    add('song.vintage', 'toggle', 'VINTAGE', { msg: 'global', g: 21, field: 'vintage', off: SNAP.FLAGS });

    // pattern sections (rows)
    for (const r of ROW_LAYOUT) {
        add(r.section + '.on', 'toggle', 'ON', { dev: r.dev, ctrl: 0, msg: 'param', off: SNAP.CTRL[r.dev] });
        add(r.section + '.pattern', 'radio', 'PATTERN', { dev: r.dev, ctrl: 1, msg: 'param', off: SNAP.CTRL[r.dev] + 1, max: 31 });
        add(r.section + '.shuffle', 'toggle', 'SHUFFLE', { dev: r.dev, msg: 'patternField', field: 0 });
        add(r.section + '.steps', 'display', 'STEPS', { dev: r.dev, msg: 'patternField', field: 1, min: 1, max: 16 });
    }

    // TB-303 x2
    for (const dev of [DEV.TB303_1, DEV.TB303_2]) {
        param(dev, 2, 'tune', 'knob', 'TUNE');
        param(dev, 3, 'cutoff', 'knob', 'CUTOFF');
        param(dev, 4, 'reso', 'knob', 'RESO');
        param(dev, 5, 'envmod', 'knob', 'ENV MOD');
        param(dev, 6, 'decay', 'knob', 'DECAY');
        param(dev, 7, 'accent', 'knob', 'ACCENT');
        param(dev, 8, 'wave', 'switch', 'WAVEFORM');
    }

    // TR-808
    const T808 = [
        ['ac_level', 2, 'AC'], ['bd_level', 3, 'BD LEVEL'], ['bd_tone', 4, 'BD TONE'], ['bd_decay', 5, 'BD DECAY'],
        ['sd_level', 6, 'SD LEVEL'], ['sd_tone', 7, 'SD TONE'], ['sd_snappy', 8, 'SD SNAPPY'],
        ['lt_level', 9, 'LT LEVEL'], ['lt_tuning', 10, 'LT TUNING'], ['slot3_switch', 11, 'LT/LC', 'switch'],
        ['mt_level', 12, 'MT LEVEL'], ['mt_tuning', 13, 'MT TUNING'], ['slot4_switch', 14, 'MT/MC', 'switch'],
        ['ht_level', 15, 'HT LEVEL'], ['ht_tuning', 16, 'HT TUNING'], ['slot5_switch', 17, 'HT/HC', 'switch'],
        ['rs_level', 18, 'RS LEVEL'], ['slot6_switch', 19, 'RS/CL', 'switch'], ['cp_level', 20, 'CP LEVEL'],
        ['slot7_switch', 21, 'CP/MA', 'switch'], ['cb_level', 22, 'CB LEVEL'], ['cy_level', 23, 'CY LEVEL'],
        ['cy_tone', 24, 'CY TONE'], ['cy_decay', 25, 'CY DECAY'], ['oh_level', 26, 'OH LEVEL'],
        ['oh_decay', 27, 'OH DECAY'], ['ch_level', 28, 'CH LEVEL']
    ];
    for (const [name, ctrl, label, kind] of T808) param(DEV.TR808, ctrl, name, kind || 'knob', label, { level: /_level$/.test(name) });

    // TR-909 (RBS controller numbers)
    const T909 = [
        ['ac_level', 2, 'AC'], ['bd_level', 3, 'BD LEVEL'], ['bd_tune', 4, 'BD TUNE'], ['bd_attack', 5, 'BD ATTACK'], ['bd_decay', 6, 'BD DECAY'],
        ['sd_level', 7, 'SD LEVEL'], ['sd_tune', 8, 'SD TUNE'], ['sd_tone', 9, 'SD TONE'], ['sd_snappy', 10, 'SD SNAPPY'],
        ['lt_level', 11, 'LT LEVEL'], ['lt_tune', 12, 'LT TUNE'], ['lt_decay', 13, 'LT DECAY'],
        ['mt_level', 14, 'MT LEVEL'], ['mt_tune', 15, 'MT TUNE'], ['mt_decay', 16, 'MT DECAY'],
        ['ht_level', 17, 'HT LEVEL'], ['ht_tune', 18, 'HT TUNE'], ['ht_decay', 19, 'HT DECAY'],
        ['rs_level', 20, 'RS LEVEL'], ['cp_level', 21, 'CP LEVEL'], ['hh_level', 22, 'HH LEVEL'],
        ['ch_decay', 23, 'CH DECAY'], ['oh_decay', 24, 'OH DECAY'], ['cc_level', 25, 'CC LEVEL'], ['cc_tune', 26, 'CC TUNE'],
        ['rc_level', 27, 'RC LEVEL'], ['rc_tune', 28, 'RC TUNE']
    ];
    for (const [name, ctrl, label] of T909) param(DEV.TR909, ctrl, name, 'knob', label);
    param(DEV.TR909, 29, 'flam', 'toggle', 'FLAM', { uiOnly: true });

    // effects
    param(DEV.DELAY, 0, 'on', 'toggle', 'ON');
    param(DEV.DELAY, 1, 'steps', 'display', 'STEPS', { min: 1, max: 32 });
    param(DEV.DELAY, 2, 'triplet', 'switch', 'TRIPLET/16TH');
    param(DEV.DELAY, 3, 'feedback', 'knob', 'F.BACK');
    param(DEV.DELAY, 4, 'pan', 'knob', 'PAN');
    param(DEV.DIST, 0, 'on', 'toggle', 'ON');
    param(DEV.DIST, 1, 'amount', 'knob', 'AMOUNT');
    param(DEV.DIST, 2, 'shape', 'knob', 'SHAPE');
    param(DEV.COMP, 0, 'on', 'toggle', 'ON');
    param(DEV.COMP, 1, 'ratio', 'knob', 'RATIO');
    param(DEV.COMP, 2, 'threshold', 'knob', 'THRES');
    param(DEV.PCF, 0, 'on', 'toggle', 'ON');
    param(DEV.PCF, 1, 'freq', 'vfader', 'FREQ');
    param(DEV.PCF, 2, 'q', 'vfader', 'Q');
    param(DEV.PCF, 3, 'amt', 'vfader', 'AMT');
    param(DEV.PCF, 4, 'pattern', 'display', 'WAVE', { min: 0, max: 54 });
    param(DEV.PCF, 5, 'dec', 'vfader', 'DEC');
    param(DEV.PCF, 6, 'mode', 'switch', 'BP/LP');

    // mixer
    add('master.fader', 'vfader', 'MASTER', { dev: DEV.MIXER, ctrl: 0, msg: 'mixerGlobal', off: SNAP.MIXER });
    add('mixer.comp_id', 'radio', 'COMP', { dev: DEV.MIXER, ctrl: 1, msg: 'mixerGlobal', off: SNAP.MIXER + 1, max: 5 });
    add('mixer.pcf_id', 'radio', 'PCF', { dev: DEV.MIXER, ctrl: 2, msg: 'mixerGlobal', off: SNAP.MIXER + 2, max: 4 });
    const MIX_K = [['mute', 'toggle', 'MUTE'], ['level', 'vfader', 'LEVEL'], ['pan', 'knob', 'PAN'], ['delay', 'knob', 'DELAY'], ['dist', 'toggle', 'DIST']];
    for (let c = 0; c < 4; c++) {
        MIX_K.forEach(([name, kind, label], k) => {
            add('mixer' + (c + 1) + '.' + name, kind, label, { dev: DEV.MIXER, ch: c, k, msg: 'mixerChannel', off: SNAP.MIXER + 3 + 5 * c + k });
        });
        add('mixer' + (c + 1) + '.meter', 'display', 'METER', { off: SNAP.CHANNEL_METERS + c, readOnly: true });
    }
    add('master.meter_l', 'display', 'L', { off: SNAP.MASTER_METERS, readOnly: true });
    add('master.meter_r', 'display', 'R', { off: SNAP.MASTER_METERS + 1, readOnly: true });
    add('delay.meter', 'display', 'METER', { off: SNAP.EFFECT_METERS, readOnly: true });
    add('dist.meter', 'display', 'METER', { off: SNAP.EFFECT_METERS + 1, readOnly: true });
    add('pcf.meter', 'display', 'METER', { off: SNAP.EFFECT_METERS + 2, readOnly: true });
    add('comp.meter', 'display', 'METER', { off: SNAP.EFFECT_METERS + 3, readOnly: true });
    add('comp.gr', 'display', 'GR', { off: SNAP.COMP_GAIN, readOnly: true });

    function control(id) { return BY_ID[id] || null; }
    // The wire message bytes for setting control `id` to `value`; `pattern` is the row's current
    // pattern for the pattern-header controls. Null for read-only or unknown controls.
    function encodeControl(id, value, pattern) {
        const c = BY_ID[id];
        if (!c || c.readOnly || c.uiOnly) return null;
        switch (c.msg) {
        case 'global': return encodeGlobal(c.g, value);
        case 'param': return encodeDevice({ type: 'param', dev: c.dev, ctrl: c.ctrl, value });
        case 'mixerGlobal': return encodeDevice({ type: 'mixerGlobal', id: c.ctrl, value });
        case 'mixerChannel': return encodeDevice({ type: 'mixerChannel', ch: c.ch, k: c.k, value });
        case 'patternField': return encodeDevice({ type: 'patternField', dev: c.dev, p: pattern || 0, field: c.field, value });
        default: return null;
        }
    }

    // ---- transport (model.rs Transport + press_transport/arrow) --------------------------------
    function createTransport() {
        return { tempo: 140000, position: 0, loopStart: 0, loopEnd: 4 * BAR, songMode: false, playing: false, recording: false, looping: false };
    }
    function clamp(x, lo, hi) { return Math.max(lo, Math.min(hi, x)); }
    function tempoStep(t, d) { return Object.assign({}, t, { tempo: clamp(t.tempo + d * 1000, TEMPO_MIN, TEMPO_MAX) }); }
    // Moves the position by d ticks from the start of the current bar.
    function locateBy(t, d) { return Object.assign({}, t, { position: clamp(d + BAR * Math.floor(t.position / BAR), 0, MAX_POS) }); }
    function wind(t, forward) { return locateBy(Object.assign({}, t, { recording: false }), forward ? 10 * BAR : -10 * BAR); }
    // Bar down turns record off, bar up does not.
    function barStep(t, up) { return locateBy(up ? t : Object.assign({}, t, { recording: false }), up ? BAR : -BAR); }
    // Loop start by d ticks; the length is kept.
    function loopStartStep(t, d) {
        const len = t.loopEnd - t.loopStart;
        const start = BAR * Math.floor(clamp(t.loopStart + d, 0, MAX_POS) / BAR);
        return Object.assign({}, t, { loopStart: start, loopEnd: clamp(len + start, start + BAR, MAX_POS) });
    }
    // Loop length by d ticks, at least one bar.
    function loopLengthStep(t, d) {
        const hi = Math.max(MAX_END - t.loopStart, BAR);
        const len = BAR * Math.floor(clamp(t.loopEnd - t.loopStart + d, BAR, hi) / BAR);
        return Object.assign({}, t, { loopEnd: t.loopStart + len });
    }
    // Stop: record off; stops when playing; when stopped locates to the loop start, or to 0 when
    // the position is at or before it.
    function stopStep(t) {
        const n = Object.assign({}, t, { recording: false });
        if (t.playing) n.playing = false;
        else n.position = t.loopStart >= t.position ? 0 : t.loopStart;
        return n;
    }
    function tempoDisplay(t) { return Math.floor(t.tempo / 1000); }
    function barDisplay(t) { return Math.floor(t.position / BAR) + 1; }
    function loopStartDisplay(t) { return Math.floor(t.loopStart / BAR) + 1; }
    function loopLengthDisplay(t) { return Math.floor((t.loopEnd - t.loopStart) / BAR); }

    const SONG_ONLY_ARROWS = /^(rewind|ff|bar_(up|down)|loop_start_(up|down)|loop_length_(up|down))$/;
    // One transport button or arrow. name: play stop record loop mode rewind ff bar_up bar_down
    // tempo_up tempo_down loop_start_up/down loop_length_up/down locate_start locate_end.
    // Returns {transport, messages} (messages are wire byte arrays, in send order).
    function transportPress(t, name) {
        const m = [];
        const g = (id, v) => m.push(encodeGlobal(id, v));
        let n = t;
        if (!t.songMode && SONG_ONLY_ARROWS.test(name)) return { transport: t, messages: m };
        switch (name) {
        case 'play': n = Object.assign({}, t, { playing: true }); g(1, 1); break;
        case 'stop':
            n = stopStep(t);
            if (t.recording) g(2, 0);
            if (t.playing) g(1, 0); else g(8, n.position);
            break;
        case 'record':
            if (!t.songMode) break;
            n = Object.assign({}, t, { recording: !t.recording }); g(2, +n.recording); break;
        case 'loop': n = Object.assign({}, t, { looping: !t.looping }); g(3, +n.looping); break;
        case 'mode':
            n = Object.assign({}, t, { songMode: !t.songMode });
            g(0, +n.songMode);
            if (!n.songMode) { n.recording = false; g(2, 0); }
            break;
        case 'rewind': case 'ff':
            if (t.recording) g(2, 0);
            n = wind(t, name === 'ff'); g(8, n.position); break;
        case 'bar_up': case 'bar_down':
            if (name === 'bar_down' && t.recording) g(2, 0);
            n = barStep(t, name === 'bar_up'); g(8, n.position); break;
        case 'tempo_up': case 'tempo_down':
            n = tempoStep(t, name === 'tempo_up' ? 1 : -1); g(4, n.tempo); break;
        case 'loop_start_up': case 'loop_start_down':
            n = loopStartStep(t, name === 'loop_start_up' ? BAR : -BAR); g(12, n.loopStart); g(16, n.loopEnd); break;
        case 'loop_length_up': case 'loop_length_down':
            n = loopLengthStep(t, name === 'loop_length_up' ? BAR : -BAR); g(16, n.loopEnd); break;
        case 'locate_start': case 'locate_end':
            n = Object.assign({}, t, { position: clamp(name === 'locate_end' ? t.loopEnd : t.loopStart, 0, MAX_POS) });
            g(8, n.position); break;
        default: break;
        }
        return { transport: n, messages: m };
    }

    // ---- behaviour.rs / model.rs display rules -----------------------------------------------
    // Extra steps of a held repeat button after t ms (the mouse down itself gives one).
    function repeatExtraSteps(tMs) {
        const a = tMs >= 400 ? Math.floor(Math.min(tMs - 400, 2399) / 100) : 0;
        const b = tMs >= 2400 ? Math.floor((tMs - 2400) / 20) : 0;
        return a + b;
    }
    // Drawn position (0/1) of a two-way switch: PCF mode is LP only at 1, all others any non-zero.
    function switchPosition(id, v) { return id === 'pcf.mode' ? +(v === 1) : +(v !== 0); }
    // New TR-909 step value on press from the current value c.
    function tr909StepValue(c, accentRow, flamOn, mods) {
        const m = mods || {};
        const toggle = (on) => (c === on ? 0 : on);
        if (accentRow) return c !== 0 ? 0 : 2;
        if ((m.ctrl && m.alt) || flamOn) return toggle(3);
        if (m.ctrl) return toggle(1);
        if (m.alt) return toggle(2);
        return (c + 1) % 3;
    }
    function peakMeterLevel(byte) {
        const b = Math.min(byte, 127);
        return b === 0 ? 0 : Math.floor(Math.log(b) / Math.log(127) * 127);
    }
    function peakMeterFrame(byte, frames) { return Math.floor(peakMeterLevel(byte) * (Math.max(frames, 1) - 1) / 127); }
    // Meter state 0..10 of a meter byte.
    function meterState(byte) { return peakMeterFrame(byte, 11); }
    const HEADER_SEGMENTS = [0, 1, 1, 1, 2, 2, 2, 3, 3, 4, 4];
    function headerMeterSegments(state) { return HEADER_SEGMENTS[Math.min(state, 10)]; }
    const GR_COLORS = Object.freeze(['g', 'g', 'g', 'y', 'y', 'y', 'r', 'r', 'r']);
    // Lit gain-reduction LEDs (0..9) for the comp gain byte.
    function grLit(byte) {
        const b = Math.min(byte, 127);
        if (b === 0) return 0;
        const low = Math.log(0.0625);
        const t = Math.trunc((Math.log(b / 16) - low) / (Math.log(7.9375) - low) * 127);
        return Math.floor(t * 9 / 127);
    }

    // ---- keyboard (ui_spec section 10), KeyboardEvent.code based -----------------------------
    const NOTE_KEYS = { KeyC: 0, KeyF: 1, KeyV: 2, KeyG: 3, KeyB: 4, KeyN: 5, KeyJ: 6, KeyM: 7, KeyK: 8, Comma: 9, KeyL: 10, Period: 11, Slash: 12 };
    const KEY_303 = { Enter: 'step', Backspace: 'back', KeyP: 'accent', BracketLeft: 'slide', Semicolon: 'down', Quote: 'up', Minus: 'note_pause', Equal: 'note_pause' };
    const DRUM_KEYS = { Minus: 0, Equal: 0, KeyA: 1, KeyS: 2, KeyD: 3, KeyF: 4, KeyG: 5, KeyH: 6, KeyJ: 7, KeyK: 8, KeyL: 9, Semicolon: 10, Quote: 11 };
    const PATTERN_KEYS = [
        ['Digit1', 'Digit2', 'Digit3', 'Digit4', 'Digit5', 'Digit6', 'Digit7', 'Digit8'],
        ['KeyQ', 'KeyW', 'KeyE', 'KeyR', 'KeyT', 'KeyY', 'KeyU', 'KeyI'],
        ['KeyA', 'KeyS', 'KeyD', 'KeyF', 'KeyG', 'KeyH', 'KeyJ', 'KeyK'],
        ['KeyZ', 'KeyX', 'KeyC', 'KeyV', 'KeyB', 'KeyN', 'KeyM', 'Comma']
    ];
    const ALWAYS = {
        NumpadEnter: 'play', Numpad0: 'stop', Numpad4: 'rewind', Numpad5: 'ff', Numpad7: 'bar_down', Numpad8: 'bar_up',
        Numpad1: 'locate_start', Numpad2: 'locate_end', NumpadMultiply: 'record', Insert: 'record',
        NumpadAdd: 'tempo_up', NumpadSubtract: 'tempo_down', NumpadDivide: 'loop'
    };
    const CTRL_KEYS_EDIT = { KeyX: 'cut', KeyC: 'copy', KeyV: 'paste', KeyD: 'duplicate', KeyZ: 'undo' };

    // Maps a key to an action object, or null (the caller then leaves the event alone).
    // opts: {shift, ctrl, mode: 'synth'|'patterns', focusRow: 0..3, playing, bare}; bare = the target keeps no key of its own
    // (Tab taps the playing step only then, otherwise it moves the focus as usual).
    // Actions: {type:'transport', name} | {type:'focus', delta} | {type:'scroll', delta}
    // | {type:'pattern', row, button} | {type:'note303', note} | {type:'step303', name}
    // | {type:'tap303', rest} | {type:'tapDrum', inst} | {type:'edit', cmd} | {type:'save'}.
    function keyAction(code, opts) {
        const o = opts || {};
        if (o.ctrl) {
            if (code === 'KeyS') return { type: 'save' };
            return CTRL_KEYS_EDIT[code] ? { type: 'edit', cmd: CTRL_KEYS_EDIT[code] } : null;
        }
        if (code === 'Space') return { type: 'transport', name: o.playing ? 'stop' : 'play' };
        if (ALWAYS[code]) return { type: 'transport', name: ALWAYS[code] };
        if (code === 'ArrowUp') return { type: 'focus', delta: -1 };
        if (code === 'ArrowDown') return { type: 'focus', delta: 1 };
        if (code === 'PageUp') return { type: 'scroll', delta: -1 };
        if (code === 'PageDown') return { type: 'scroll', delta: 1 };
        const row = o.focusRow;
        if (o.mode === 'patterns') {
            for (let r = 0; r < 4; r++) {
                const i = PATTERN_KEYS[r].indexOf(code);
                if (i >= 0) return { type: 'pattern', row: r, button: i + 1 };
            }
            return null;
        }
        if (row === 0 || row === 1) {
            if (code in NOTE_KEYS) return { type: 'note303', note: NOTE_KEYS[code] };
            if (KEY_303[code]) return { type: 'step303', name: KEY_303[code] };
            if (code === 'Tab' && o.playing && o.bare) return { type: 'tap303', rest: !!o.shift };
        } else if ((row === 2 || row === 3) && code in DRUM_KEYS) {
            return { type: 'tapDrum', inst: DRUM_KEYS[code] };
        }
        return null;
    }

    return {
        DEV, PARAM_SUB, HEADER_SUB, PATTERN_SUB, STEP_SUB, MIXER_GLOBAL_SUB, MIXER_CHANNEL_SUB,
        BAR, MAX_POS, MAX_END, TEMPO_MIN, TEMPO_MAX,
        SNAPSHOT_SIZE, SNAP, CTRL_COUNT, FLAG, CTRL_KEYS,
        PATTERN_COUNT, ROW_PATTERN, PATTERNS_SIZE, CLEAR_303_STEP,
        patternOffset, patternBytes, stepIndex, stepField, setStepField,
        isEmptyPattern, emptiedPattern, nextEmptyPattern, patternIndex, otherHat909, tapDrumValue,
        encodeGlobal, encodeDevice, encodeControl, batch, decodeSnapshot,
        CONTROLS, ROW_LAYOUT, COLUMN_LAYOUT, control,
        createTransport, tempoStep, locateBy, wind, barStep, loopStartStep, loopLengthStep, stopStep,
        tempoDisplay, barDisplay, loopStartDisplay, loopLengthDisplay, transportPress,
        repeatExtraSteps, switchPosition, tr909StepValue, peakMeterLevel, peakMeterFrame,
        meterState, headerMeterSegments, GR_COLORS, grLit, keyAction
    };
});
