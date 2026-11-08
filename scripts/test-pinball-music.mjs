// T10: pinball music. Pure Node, no AudioContext.
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { CUES, MUSIC_SECTIONS } from '../ui/js/desktop/apps/pinball/contracts.js';
import { expandBar, createConductor, createScheduler, stingerLength, STEPS_PER_BAR } from '../ui/js/desktop/apps/pinball/audio/music.js';
import { INSTRUMENTS } from '../ui/js/desktop/apps/pinball/audio/instruments.js';
import neon from '../ui/js/desktop/apps/pinball/audio/songs/neon.js';
import leviathan from '../ui/js/desktop/apps/pinball/audio/songs/leviathan.js';
import temple from '../ui/js/desktop/apps/pinball/audio/songs/temple.js';

const SONGS = { neon, leviathan, temple };
const BPM = { neon: 112, leviathan: 84, temple: 100 };
let passed = 0;
function test(name, fn) { fn(); passed++; console.log('ok   ' + name); }
const barsOf = (song, s) => song.sections[s].reduce((n, p) => n + song.patterns[p].bars, 0);

test('files stay under 700 lines', () => {
    for (const f of ['audio/music.js', 'audio/instruments.js', 'audio/songs/neon.js', 'audio/songs/leviathan.js', 'audio/songs/temple.js', 'audio/songs/notation.js']) {
        const n = readFileSync(new URL('../ui/js/desktop/apps/pinball/' + f, import.meta.url), 'utf8').split('\n').length;
        assert.ok(n <= 700, f + ' has ' + n + ' lines');
    }
});

test('expandBar is deterministic', () => {
    for (const [id, song] of Object.entries(SONGS)) {
        assert.equal(song.bpm, BPM[id]);
        for (const section of MUSIC_SECTIONS) {
            for (let bar = 0; bar < 40; bar++) {
                const a = JSON.stringify(expandBar(song, section, bar, { transition: bar % 3 === 0 }));
                const b = JSON.stringify(expandBar(song, section, bar, { transition: bar % 3 === 0 }));
                assert.equal(a, b, id + ' ' + section + ' bar ' + bar);
            }
        }
    }
});

test('every section and stinger reference resolves', () => {
    for (const [id, song] of Object.entries(SONGS)) {
        for (const section of MUSIC_SECTIONS) {
            assert.ok(Array.isArray(song.sections[section]), id + ' missing section ' + section);
            if (section === 'off') { assert.equal(song.sections.off.length, 0); continue; }
            assert.ok(song.sections[section].length > 0, id + ' empty section ' + section);
            for (const p of song.sections[section]) assert.ok(song.patterns[p], id + ' ' + section + ' -> ' + p);
            assert.ok(expandBar(song, section, 0).events.length > 0, id + ' ' + section + ' is silent');
        }
        for (const cue of CUES) {
            const name = song.stingers[cue];
            assert.ok(name && song.patterns[name], id + ' missing stinger ' + cue);
        }
        for (const f of Object.values(song.fills || {}).concat(Object.values(song.transitions || {}))) assert.ok(song.patterns[f], id + ' -> ' + f);
        for (const [name, p] of Object.entries(song.patterns)) {
            for (const track of Object.keys(p.tracks)) {
                const cfg = song.instruments[track];
                assert.ok(cfg, id + ' ' + name + ' uses undefined track ' + track);
                assert.equal(typeof INSTRUMENTS[cfg.type], 'function', id + ' unknown voice ' + cfg.type);
            }
        }
    }
});

test('notes are in range and sections are long enough', () => {
    for (const [id, song] of Object.entries(SONGS)) {
        const stepSec = 60 / song.bpm / 4;
        for (const [name, p] of Object.entries(song.patterns)) {
            for (const [track, evs] of Object.entries(p.tracks)) {
                for (const [step, midi, len, vel] of evs) {
                    const where = id + ' ' + name + '/' + track + ' @' + step;
                    assert.ok(Number.isInteger(step) && step >= 0 && step < p.bars * STEPS_PER_BAR, where + ' step');
                    assert.ok(Number.isInteger(midi) && midi >= 24 && midi <= 108, where + ' midi ' + midi);
                    assert.ok(Number.isInteger(len) && len >= 1 && len <= 64, where + ' len');
                    assert.ok(vel > 0 && vel <= 1, where + ' vel ' + vel);
                }
            }
        }
        for (const cue of CUES) {
            const secs = stingerLength(song, cue) * stepSec;
            assert.ok(secs >= 0.3 && secs <= 3, id + ' stinger ' + cue + ' lasts ' + secs.toFixed(2) + ' s');
        }
        for (const section of ['main', 'mode', 'multiball', 'wizard', 'attract']) {
            assert.ok(barsOf(song, section) >= 8, id + ' ' + section + ' has ' + barsOf(song, section) + ' bars');
        }
        const sig = (b) => JSON.stringify(expandBar(song, 'main', b).events.map((e) => [e.step, e.track, e.midi]));
        const mainBars = barsOf(song, 'main');
        assert.ok(new Set(Array.from({ length: mainBars }, (_, b) => sig(b))).size >= 8, id + ' main repeats too soon');
        const density = (s) => { let n = 0; const total = barsOf(song, s); for (let b = 0; b < total; b++) n += expandBar(song, s, b).events.length; return n / total; };
        assert.ok(density('multiball') > density('main') * 1.15, id + ' multiball is not clearly busier (' + density('multiball').toFixed(1) + ' vs ' + density('main').toFixed(1) + ')');
        assert.ok(density('wizard') >= density('multiball') * 0.9, id + ' wizard is thinner than multiball');
    }
});

test('a jittery clock over 10 s schedules each step exactly once', () => {
    let seed = 12345;
    const rnd = () => { seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0; return seed / 4294967296; };
    for (const song of Object.values(SONGS)) {
        const stepSec = 60 / song.bpm / 4;
        let now = 100, tickFn = null;
        const seen = [];
        const sched = createScheduler({ now: () => now }, { start: (fn) => { tickFn = fn; return 1; }, stop() { tickFn = null; } }, {
            stepSec, onStep: (s, t) => seen.push([s, t])
        });
        sched.start();
        const t0 = now + 0.06;
        while (now < 110) { now += 0.025 + rnd() * 0.07; tickFn(); } // 25 ms nominal, up to 95 ms late
        sched.stop();
        assert.ok(seen.length >= Math.floor(10 / stepSec), 'only ' + seen.length + ' steps');
        seen.forEach(([s, t], i) => {
            assert.equal(s, i, 'step order/duplicates at ' + i);
            assert.ok(Math.abs(t - (t0 + i * stepSec)) < 1e-9, 'step time drifted at ' + i);
        });
    }
});

test('section changes land on a bar boundary; stingers start on the next step', () => {
    for (const song of Object.values(SONGS)) {
        const c = createConductor(song);
        let prev = 'off', stingerSeen = 0;
        const changes = [];
        for (let g = 0; g < 16 * 14; g++) {
            if (g === 3) c.request('main');
            if (g === 16 * 3 + 5) c.request('multiball');
            if (g === 16 * 6 + 11) c.request('wizard');
            if (g === 16 * 8 + 2) { c.request('mode'); c.request('wizard'); } // second request cancels the change
            if (g === 16 * 10 + 9) c.request('bonus');
            if (g === 16 * 5 + 7) c.stinger('jackpot');
            const r = c.step(g);
            if (r.section !== prev) {
                changes.push([g, r.section]);
                assert.equal(g % 16, 0, 'section changed off the bar line at step ' + g);
                assert.ok(r.switched);
                prev = r.section;
            }
            if (r.stingerStarted) {
                stingerSeen++;
                assert.equal(g, 16 * 5 + 7);
                assert.ok(r.events.some((e) => e.tag === 'stinger'));
            }
        }
        assert.deepEqual(changes, [[16, 'main'], [16 * 4, 'multiball'], [16 * 7, 'wizard'], [16 * 11, 'bonus']]);
        assert.equal(stingerSeen, 1);
        assert.equal(c.request('nonsense'), false);
    }
});

console.log('pinball music ok (' + passed + ' tests)');
