// SPDX-License-Identifier: GPL-3.0-only
// RB-338 mode tests. Sections are separate async functions called from main().
import assert from 'node:assert/strict';
import fs from 'node:fs';

// The package is "type": "module", so load the UMD file as a CommonJS function body.
const loadUmd = (file) => {
    const mod = { exports: {} };
    new Function('module', 'exports', fs.readFileSync(file, 'utf8'))(mod, mod.exports);
    return mod.exports;
};
const M = loadUmd('ui/js/desktop/apps/synth-studio-rb338-model.js');

async function modelSection() {
    // wire encodings
    assert.deepEqual(M.encodeGlobal(4, 140000), [35, 4, 0, 2, 34, 224]);
    assert.deepEqual(M.encodeGlobal(1, 1), [35, 1, 1]);
    assert.deepEqual(M.encodeDevice({ type: 'mixerChannel', ch: 2, k: 1, value: 100 }), [41, 0, 147, 2, 1, 100]);
    assert.deepEqual(M.encodeDevice({ type: 'mixerGlobal', id: 0, value: 90 }), [41, 0, 145, 0, 90]);
    assert.deepEqual(M.encodeDevice({ type: 'param', dev: M.DEV.TR808, ctrl: 3, value: 64 }), [41, 3, 57, 3, 64]);
    assert.deepEqual(M.encodeDevice({ type: 'patternField', dev: M.DEV.TR909, p: 5, field: 1, value: 12 }), [41, 4, 76, 5, 1, 12]);
    assert.deepEqual(M.encodeDevice({ type: 'step', dev: M.DEV.TR909, p: 0, step: 3, field: 1, value: 2 }), [41, 4, 79, 0, 3, 1, 2]);
    assert.equal(M.encodeDevice({ type: 'pattern', dev: M.DEV.TB303_2, p: 7, bytes: new Uint8Array(98) }).length, 4 + 98);
    assert.deepEqual(M.encodeControl('tb303_1.cutoff', 77), [41, 1, 40, 3, 77]);
    assert.deepEqual(M.encodeControl('pattern3.steps', 9, 4), [41, 3, 59, 4, 1, 9]);
    assert.deepEqual(Array.from(M.batch([[35, 1, 1], [41, 0, 147, 2, 1, 100]])), [3, 0, 35, 1, 1, 6, 0, 41, 0, 147, 2, 1, 100]);

    // controls table: unique ids, snapshot offsets inside the 192-byte block
    const ids = new Set(M.CONTROLS.map((c) => c.id));
    assert.equal(ids.size, M.CONTROLS.length);
    for (const c of M.CONTROLS) if (c.off !== null) assert.ok(c.off >= 0 && c.off < M.SNAPSHOT_SIZE, c.id);
    assert.equal(M.control('tr909.flam').off, 135 + 29);
    assert.equal(M.control('mixer3.level').off, 64 + 3 + 5 * 2 + 1);
    assert.equal(M.CONTROLS.filter((c) => /^tb303_[12]\.(tune|cutoff|reso|envmod|decay|accent|wave)$/.test(c.id)).length, 14);
    assert.equal(M.CONTROLS.filter((c) => /^tr808\./.test(c.id)).length, 27);
    assert.equal(M.ROW_LAYOUT.length, 4);

    // transport: stop-stop-stop reaches 0, loop start keeps the length
    let t = Object.assign(M.createTransport(), { playing: true, position: 3000, loopStart: 1536, loopEnd: 1536 + 4 * M.BAR });
    t = M.stopStep(t); assert.equal(t.playing, false); assert.equal(t.position, 3000);
    t = M.stopStep(t); assert.equal(t.position, 1536);
    t = M.stopStep(t); assert.equal(t.position, 0);
    const base = M.createTransport();
    const moved = M.loopStartStep(base, M.BAR);
    assert.equal(moved.loopStart, M.BAR);
    assert.equal(moved.loopEnd - moved.loopStart, base.loopEnd - base.loopStart);
    assert.equal(M.loopLengthStep(base, -10 * M.BAR).loopEnd, M.BAR);
    assert.equal(M.tempoStep(base, -10000000).tempo, M.TEMPO_MIN);
    // press rules: arrows idle in pattern mode, mode switch to pattern turns record off
    assert.deepEqual(M.transportPress(base, 'ff').messages, []);
    const song = Object.assign({}, base, { songMode: true, recording: true });
    assert.deepEqual(M.transportPress(song, 'mode').messages, [[35, 0, 0], [35, 2, 0]]);
    assert.deepEqual(M.transportPress(song, 'bar_up').messages, [[35, 8, 0, 0, 3, 0]]);
    assert.equal(M.transportPress(song, 'bar_up').transport.recording, true);
    assert.equal(M.transportPress(song, 'bar_down').transport.recording, false);
    assert.deepEqual(M.transportPress(base, 'record').messages, []);

    // 909 step values
    assert.deepEqual([0, 1, 2].map((c) => M.tr909StepValue(c, false, false, {})), [1, 2, 0]);
    assert.equal(M.tr909StepValue(0, false, false, { ctrl: true, alt: true }), 3);
    assert.equal(M.tr909StepValue(3, false, false, { ctrl: true, alt: true }), 0);
    assert.equal(M.tr909StepValue(0, false, true, {}), 3);
    assert.equal(M.tr909StepValue(0, true, false, {}), 2);
    assert.equal(M.tr909StepValue(2, true, false, {}), 0);
    assert.equal(M.tr909StepValue(0, false, false, { ctrl: true }), 1);
    assert.equal(M.tr909StepValue(0, false, false, { alt: true }), 2);
    assert.equal(M.otherHat909(8, 1), 9);
    assert.equal(M.otherHat909(8, 0), -1);

    // repeat, switch, meters
    assert.deepEqual([0, 399, 400, 499, 500, 2399, 2400, 2420].map(M.repeatExtraSteps), [0, 0, 0, 0, 1, 19, 20, 21]);
    assert.equal(M.switchPosition('pcf.mode', 2), 0);
    assert.equal(M.switchPosition('pcf.mode', 1), 1);
    assert.equal(M.switchPosition('tb303_1.wave', 127), 1);
    const states = [0, 1, 2, 3, 5, 7, 12, 19, 30, 49, 81, 127].map(M.meterState);
    assert.deepEqual(states, [0, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10]);
    assert.equal(M.headerMeterSegments(1), 1);
    assert.equal(M.headerMeterSegments(10), 4);
    assert.equal(M.grLit(0), 0);
    assert.equal(M.grLit(1), 0);
    assert.equal(M.grLit(18), 5);
    assert.equal(M.grLit(127), 9);

    // patterns: empty / emptied / next empty with wrap
    const plen = M.ROW_PATTERN[0].plen;
    const rowBuf = new Uint8Array(plen * 32);
    for (let p = 0; p < 32; p++) {
        rowBuf[plen * p + 1] = 16;
        M.emptiedPattern(0, rowBuf.subarray(plen * p, plen * p + plen)).forEach((b, i) => { rowBuf[plen * p + i] = b; });
    }
    assert.ok(M.isEmptyPattern(0, rowBuf.subarray(0, plen)));
    rowBuf[plen * 3 + 2] = 5; // a note in pattern 3
    assert.ok(!M.isEmptyPattern(0, rowBuf.subarray(plen * 3, plen * 4)));
    assert.equal(M.nextEmptyPattern(0, rowBuf, 2), 4);
    assert.equal(M.nextEmptyPattern(0, rowBuf, 31), 0);
    assert.equal(M.nextEmptyPattern(0, rowBuf, 3), 4);
    for (let p = 0; p < 32; p++) rowBuf[plen * p + 2] = 1;
    assert.equal(M.nextEmptyPattern(0, rowBuf, 0), null);
    const cleared = M.emptiedPattern(0, rowBuf.subarray(0, plen));
    assert.equal(cleared[1], 16);
    assert.deepEqual(Array.from(cleared.subarray(2, 8)), [0, 0, 0, 0, 0, 1]);
    assert.equal(M.PATTERNS_SIZE, 32 * 98 * 2 + 32 * 194 * 2);
    assert.equal(M.patternOffset(3, 31) + 194, M.PATTERNS_SIZE);
    assert.equal(M.patternIndex(2, 3), 18);

    // keyboard
    const k = M.keyAction;
    assert.deepEqual(k('Space', { playing: false }), { type: 'transport', name: 'play' });
    assert.deepEqual(k('Space', { playing: true }), { type: 'transport', name: 'stop' });
    assert.deepEqual(k('Numpad7', {}), { type: 'transport', name: 'bar_down' });
    assert.deepEqual(k('KeyC', { mode: 'synth', focusRow: 0 }), { type: 'note303', note: 0 });
    assert.deepEqual(k('Slash', { mode: 'synth', focusRow: 1 }), { type: 'note303', note: 12 });
    assert.deepEqual(k('KeyA', { mode: 'synth', focusRow: 3 }), { type: 'tapDrum', inst: 1 });
    assert.deepEqual(k('Minus', { mode: 'synth', focusRow: 2 }), { type: 'tapDrum', inst: 0 });
    assert.deepEqual(k('Minus', { mode: 'synth', focusRow: 0 }), { type: 'step303', name: 'note_pause' });
    assert.equal(k('Tab', { mode: 'synth', focusRow: 0, playing: false }), null);
    assert.deepEqual(k('Tab', { mode: 'synth', focusRow: 0, playing: true, shift: true }), { type: 'tap303', rest: true });
    assert.equal(k('Tab', { mode: 'synth', focusRow: 2, playing: true }), null);
    assert.deepEqual(k('KeyQ', { mode: 'patterns', focusRow: 0 }), { type: 'pattern', row: 1, button: 1 });
    assert.deepEqual(k('Comma', { mode: 'patterns', focusRow: 0 }), { type: 'pattern', row: 3, button: 8 });
    assert.equal(k('KeyP', { mode: 'patterns', focusRow: 0 }), null);
    assert.deepEqual(k('ArrowDown', {}), { type: 'focus', delta: 1 });
    assert.deepEqual(k('KeyV', { ctrl: true }), { type: 'edit', cmd: 'paste' });
    assert.equal(k('KeyF', { ctrl: true }), null);

    // snapshot decode of a hand-built buffer
    const buf = new ArrayBuffer(M.SNAPSHOT_SIZE);
    const b = new Uint8Array(buf);
    const dv = new DataView(buf);
    b[0] = 1; b[1] = 1 | 4 | 8; b[2] = 64;
    dv.setUint32(4, 77, true); dv.setFloat64(8, 123456, true); dv.setInt32(16, -5, true);
    dv.setUint32(20, 768, true); dv.setUint32(24, 3840, true); dv.setUint32(28, 140000, true);
    dv.setUint32(32, 2, true); dv.setUint32(36, 9, true); dv.setUint32(40, 1, true);
    b.set([255, 255, 255, 7], 44); b.set([0, 1, 2, 3], 48); b.set([10, 20, 30, 40], 52);
    b.set([50, 60], 56); b[58] = 18; b.set([1, 2, 3, 4], 59);
    b[64] = 100; b[65] = 3; b[66] = 2; b.set([1, 90, 64, 10, 0], 67 + 5);
    b[88 + 3] = 55; b[106 + 28] = 28; b[135 + 29] = 29; b[165 + 4] = 44; b[170 + 2] = 33; b[173 + 6] = 66; b[180 + 2] = 22;
    const s = M.decodeSnapshot(buf);
    assert.equal(s.version, 1);
    assert.deepEqual([s.playing, s.recording, s.songMode, s.loopOn, s.vintage], [true, false, true, true, false]);
    assert.equal(s.shuffle, 64);
    assert.deepEqual([s.appliedSeq, s.frames, s.tick, s.loopStart, s.loopEnd, s.tempo], [77, 123456, -5, 768, 3840, 140000]);
    assert.deepEqual([s.generation, s.tracksRev, s.merges], [2, 9, 1]);
    assert.deepEqual(s.playStep, [255, 255, 255, 7]);
    assert.deepEqual(s.playingPattern, [0, 1, 2, 3]);
    assert.deepEqual(s.channelMeters, [10, 20, 30, 40]);
    assert.deepEqual(s.masterMeters, [50, 60]);
    assert.equal(s.compGain, 18);
    assert.deepEqual(s.effectMeters, { delay: 1, dist: 2, pcf: 3, comp: 4 });
    assert.deepEqual([s.mixer.master, s.mixer.compId, s.mixer.pcfId], [100, 3, 2]);
    assert.deepEqual(s.mixer.channels[1], { mute: 1, level: 90, pan: 64, send: 10, dist: 0 });
    assert.equal(s.ctrl.tb303_1[3], 55);
    assert.equal(s.ctrl.tr808.length, 29);
    assert.equal(s.ctrl.tr808[28], 28);
    assert.equal(s.ctrl.tr909[29], 29);
    assert.equal(s.ctrl.delay[4], 44);
    assert.equal(s.ctrl.dist[2], 33);
    assert.equal(s.ctrl.pcf[6], 66);
    assert.equal(s.ctrl.comp[2], 22);
    // control offsets agree with the decoder
    assert.equal(b[M.control('tb303_1.cutoff').off], 55);
    assert.equal(b[M.control('mixer2.level').off], 90);
    assert.throws(() => M.decodeSnapshot(new ArrayBuffer(10)));
    console.log('rb338 model: ok');
}

async function main() {
    await modelSection();
}

main().catch((e) => { console.error(e); process.exit(1); });
