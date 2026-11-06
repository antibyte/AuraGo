// Biome score data: tempi and scales follow aura-drift-music.js; the arranger (music.mjs) turns each config into bed, drive and peak stems.
import { SR, TAU, mk, secs, pink, white, svf, filter, loopify, loopHz, mtof, add } from './dsp.mjs';
import { birdCall } from './sfx-world.mjs';

// Seamless noise bed: render N + 1 s, then crossfade the overhang onto the start.
function texture(ctx, stem, kind, gain, rev = 0.2) {
    const { N, r } = ctx, n = N + secs(1), lfo = loopHz(N, 0.1), lfo2 = loopHz(N, 0.23);
    let x;
    if (kind === 'wind') x = svf(pink(n, r), 'bp', i => 600 + 350 * Math.sin(TAU * lfo * i / SR) + 120 * Math.sin(TAU * lfo2 * i / SR), 0.6);
    else if (kind === 'air') x = svf(white(n, r), 'bp', i => 5200 + 1500 * Math.sin(TAU * lfo * i / SR), 0.7);
    else if (kind === 'sand') x = filter(svf(pink(n, r), 'bp', 380, 0.5), 'peak', 2000, 0.7, 2);
    else x = svf(pink(n, r), 'lp', 1500, 0.6);
    const loop = loopify(x, 1);
    const g = mk(N); for (let i = 0; i < N; i++) g[i] = loop[i] * (0.65 + 0.35 * Math.sin(TAU * lfo2 * i / SR + 1));
    ctx.put(stem, g, 0, { gain, rev });
}
const birds = (ctx, count, gain) => { const { r, stepLen } = ctx; for (let k = 0; k < count; k++) ctx.put('bed', birdCall(r, k % 4), Math.floor(r() * (ctx.bars * 16 - 12)), { gain: gain * (0.6 + 0.4 * r()), pan: r() * 1.6 - 0.8, rev: 0.45 }); void stepLen; };
const chordFreqs = (ctx, bar, mult = 1) => { const d = ctx.cfg.prog[bar]; const L = ctx.cfg.scale; const note = deg => L[((deg % L.length) + L.length) % L.length] + 12 * Math.floor(deg / L.length); return [d, d + 2, d + 4].map(x => mtof(ctx.cfg.root + note(x)) * mult); };

const std = (extra = {}) => ({ swing: 0, bars: 16, ...extra });

export const CONFIGS = {
    // alpine: folk waltz-less "oom-pah", accordion hook, zither arpeggios, cowbells, a yodel at the end of each half
    alpine: std({
        id: 'alpine', tempo: 100, root: 50, bassRoot: 38, scale: [0, 3, 5, 7, 10], prog: [0, 3, 1, 4, 0, 3, 1, 4, 2, 3, 1, 4, 0, 3, 2, 4],
        sc: { depth: 0.16, release: 0.25 }, kit: { kick: 'soft', snare: 'brush', shaker: true, fill: 'snare' },
        drums: { kick: { A: 'X.......x.......', B: 'X.......x......o' }, snare: { A: '....x.......x...', B: '....x...o...x.o.' }, hat: { A: 'o.x.o.x.o.x.o.x.', B: 'x.x.x.x.x.x.x.xo' } },
        pad: { gain: 0.3, rev: 0.4, oct: 0, opts: { voices: 3, spreadCents: 9, cutoff: 1000, movement: 0.3, attack: 0.5, release: 0.9, wave: 'tri' } },
        sub: { gain: 0.25, len: 0.85 },
        bass: { kind: 'pluck', gain: 0.8, pats: { A: 'R.......5.......', B: 'R.......5.....r.', pre: 'R...R...5...3...' } },
        stab: { voice: (f, len, r) => I_.accordion(f, len, r), pats: { A: '....x.......x...', B: '....x...x...x...' }, len: 2.2, gain: 0.5, oct: 12, rev: 0.15 },
        arp: { voice: (f, len, r) => I_.zither(f, len, r), pats: { A: '0.1.2.1.0.1.2.3.', B: '0.1.2.3.4.3.2.1.' }, bed: 0.22, peak: 0.2, oct: 12, rev: 0.4, dly: 0.25 },
        lead: {
            voice: (f, len, r) => I_.accordion(f, len, r), gain: 0.55, rev: 0.3, dly: 0.15, octave: 0.3, oct: 12, form: ['A', 'B', 'A', 'C', 'A', 'B', 'D', 'C'],
            phrases: {
                A: '0:3:5 3:1:6 4:2:7 6:2:6 8:4:5 12:2:3 14:2:4 16:3:5 19:1:6 20:2:7 22:2:8 24:6:7 30:2:6', B: '0:3:7 3:1:8 4:2:7 6:2:6 8:4:5 12:4:3 16:3:4 19:1:5 20:2:6 22:2:5 24:8:3',
                C: '0:3:5 3:1:6 4:2:7 6:2:9 8:6:8 14:2:7 16:3:6 19:1:7 20:4:8 24:3:7 27:1:6 28:4:5', D: '0:2:3 2:2:5 4:2:7 6:2:5 8:2:3 10:2:5 12:4:7 16:2:8 18:2:7 20:2:5 22:2:7 24:8:5'
            }
        },
        fx: { size: 0.55, damp: 0.5, wet: 0.7, delayBeats: 0.75, fb: 0.3 },
        special(ctx, I) {
            texture(ctx, 'bed', 'wind', 0.07, 0.3);
            [[6, 622], [22, 523], [41, 698], [58, 587], [87, 622], [102, 523], [121, 698], [150, 587], [183, 523], [214, 622]].forEach(([s, f], k) => ctx.put('bed', I.cowbell(f * (k % 2 ? 1 : 0.75), ctx.r), s, { gain: 0.1, pan: k % 2 ? 0.6 : -0.6, rev: 0.5 }));
            const t = ctx.beatSec;
            [7, 15].forEach(b => ctx.put('peak', I.yodel(ctx.r, [[62, 0.5], [74, 0.5], [66, 0.5], [78, 0.5], [67, 0.75], [79, 0.25], [74, 0.5], [62, 1.5]], t), b * 16 + 2, { gain: 0.5, rev: 0.4, dly: 0.2 }));
        }
    }),
    // desert: twang guitar over frame-drum and darbuka, dry reed drone, spring reverb
    desert: std({
        id: 'desert', tempo: 118, root: 52, bassRoot: 40, scale: [0, 2, 5, 7, 9], prog: [0, 3, 1, 4, 0, 3, 1, 4, 3, 2, 1, 4, 0, 3, 1, 0],
        sc: { depth: 0.2, release: 0.22 }, kit: { kick: 'punch', snare: 'rim', shaker: true, fill: 'tom', fillTom: 170 },
        drums: {
            kick: { A: 'X.....x.X.......', B: 'X.....x.X.x...x.' }, snare: { A: '....x.......x...', B: '....x.o.....x.x.' }, hat: { A: 'x.oxx.oxx.oxx.ox', B: 'x.xxx.xxx.xxx.xx' },
            perc(ctx, bar, base, I) { for (const s of [3, 7, 11, 15]) ctx.put('drive', I.darbuka('tek', ctx.r), base + s, { gain: 0.2 + 0.1 * (s === 15), pan: 0.3, human: 0.001 }); if (bar % 2) ctx.put('drive', I.darbuka('doum', ctx.r), base + 14, { gain: 0.35 }); }
        },
        pad: { gain: 0.16, rev: 0.25, oct: 0, opts: { voices: 3, spreadCents: 7, cutoff: 1500, movement: 0.5, attack: 0.4, release: 0.7, wave: 'square' } },
        sub: { gain: 0.35, len: 0.8 },
        bass: { kind: 'saw', gain: 0.75, pats: { A: 'R..R....R..R..5.', B: 'R..R..l.R..R..5r', pre: 'R..R....R.R.R.5.' } },
        stab: { voice: (f, len, r) => I_.guitarTwang(f, 0.14, r, { decay: 0.99 }), pats: { A: '.x..x..x.x..x...', B: '.x.xx..x.x.xx..x' }, len: 1, gain: 0.5, oct: 12, rev: 0.12, strum: 40 },
        arp: { voice: (f, len, r) => I_.harp(f, len, r), pats: { A: '0...2...1...3...', B: '0.2.1.3.2.4.3.1.' }, bed: 0.2, peak: 0.2, oct: 24, rev: 0.25, dly: 0.2 },
        lead: {
            voice: (f, len, r, o) => I_.guitarTwang(f, len, r, { slideFrom: o?.glideFrom || 0 }), gain: 0.8, rev: 0.22, dly: 0.28, octave: 0.2, oct: 12, glide: true, form: ['A', 'B', 'A', 'C', 'A', 'B', 'D', 'C'],
            phrases: {
                A: '0:2:4 2:2:4 4:2:5 6:2:4 8:3:3 11:1:2 12:4:3 16:2:4 18:2:4 20:2:6 22:2:5 24:4:4 28:4:2', B: '0:2:6 2:2:6 4:2:7 6:2:6 8:4:5 12:2:4 14:2:5 16:6:6 22:2:4 24:8:3',
                C: '0:1:3 1:1:4 2:2:5 4:4:6 8:2:7 10:2:6 12:2:5 14:2:4 16:4:3 20:4:5 24:4:4 28:2:3 30:2:2', D: '0:4:7 4:2:6 6:2:5 8:4:6 12:4:4 16:2:5 18:2:6 20:2:7 22:2:8 24:8:7'
            }
        },
        fx: { size: 0.28, damp: 0.2, predelay: 0.004, wet: 0.8, delayBeats: 0.75, fb: 0.32 },
        special(ctx, I) {
            texture(ctx, 'bed', 'sand', 0.09, 0.2);
            for (let b = 0; b < 16; b += 2) { ctx.put('peak', I.snare('clap', ctx.r), b * 16 + 4, { gain: 0.2, rev: 0.2 }); ctx.put('peak', I.snare('clap', ctx.r), b * 16 + 12, { gain: 0.2, rev: 0.2 }); }
            for (const b of [3, 11]) ctx.put('bed', I.padChord([mtof(40), mtof(47)], ctx.barSec * 2, { voices: 3, cutoff: 700, attack: 0.6, release: 1, wave: 'square' }, ctx.r), b * 16, { gain: 0.18, rev: 0.3 });
        }
    }),
    // harbour: synthwave. gated snare, pumping saw bass, analogue arp, ship horn
    harbour: std({
        id: 'harbour', tempo: 92, root: 54, bassRoot: 42, scale: [0, 3, 7, 10, 12], prog: [0, 3, 1, 4, 0, 3, 1, 4, 2, 3, 1, 4, 0, 3, 2, 4],
        sc: { depth: 0.5, release: 0.22 }, kit: { kick: 'punch', snare: 'gated', fill: 'tom', fillTom: 140, snareRev: 0.1 },
        drums: { kick: { A: 'X..x....X..x....', B: 'X..x....X..x..x.' }, snare: { A: '....X.......X...' }, hat: { A: '..x...x...x...x.', B: 'o.x.o.x.o.x.o.x.' }, open: { A: '......x.......x.' } },
        pad: { gain: 0.7, rev: 0.4, oct: 0, wide: true, opts: { voices: 5, spreadCents: 13, cutoff: 1700, movement: 0.45, attack: 0.5, release: 0.8 } },
        sub: { gain: 0.4, len: 0.95 },
        bass: { kind: 'saw', gain: 0.65, duck: true, gate: 0.85, pats: { A: 'R.R.R.R.R.R.R.R.', B: 'R.R.r.R.R.R.r.R.', pre: 'R.R.R.R.R.5.3.R.' } },
        arp: { voice: (f, len, r) => I_.sawStab(f, 0.1, r), pats: { A: '0102101201021012', B: '0123210301232103' }, bed: 0.1, peak: 0.2, oct: 12, rev: 0.3, dly: 0.45, bedEvery: 4 },
        lead: {
            voice: (f, len, r, o) => I_.squareLead(f, len, r, { glideFrom: o?.glideFrom || 0 }), gain: 0.42, rev: 0.35, dly: 0.4, glide: true, octave: 0.25, oct: 12, form: ['A', 'B', 'A', 'C', 'A', 'B', 'D', 'C'],
            phrases: {
                A: '0:6:7 6:2:6 8:4:5 12:4:6 16:6:7 22:2:8 24:8:6', B: '0:4:5 4:4:6 8:6:7 14:2:6 16:4:5 20:4:3 24:8:4',
                C: '0:2:7 2:2:8 4:4:9 8:4:8 12:4:7 16:6:6 22:2:7 24:8:8', D: '0:8:6 8:4:5 12:4:6 16:8:7 24:8:5'
            }
        },
        fx: { size: 0.72, damp: 0.35, wet: 0.9, delayBeats: 0.75, fb: 0.4 },
        special(ctx, I) {
            texture(ctx, 'bed', 'air', 0.025, 0.4);
            for (const b of [3, 11]) ctx.put('bed', I.padChord([mtof(42), mtof(49)], 1.8, { voices: 3, spreadCents: 6, cutoff: 600, attack: 0.18, release: 0.9 }, ctx.r), b * 16 + 8, { gain: 0.35, rev: 0.5 });
        }
    }),
    // salt: spacey. huge ping-pong delays, bell arps, choir wash, theremin hook, sweeps
    salt: std({
        id: 'salt', tempo: 106, root: 48, bassRoot: 36, scale: [0, 2, 4, 7, 9], prog: [0, 3, 1, 4, 0, 3, 1, 4, 3, 4, 1, 0, 0, 3, 4, 2],
        sc: { depth: 0.28, release: 0.3 }, kit: { kick: 'soft', snare: 'clap', hatBright: 0.85, snareRev: 0.5, fill: 'snare' },
        drums: { kick: { A: 'X...X...X...X...' }, snare: { A: '....x.......x...' }, hat: { A: '..x...x...x...x.', B: '..x..xx...x..xx.' }, open: { A: '..o...o...o...o.' } },
        pad: { gain: 0.6, rev: 0.6, oct: 0, wide: true, opts: { voices: 5, spreadCents: 16, cutoff: 1000, movement: 0.7, attack: 0.9, release: 1.2, q: 1.3 } },
        sub: { gain: 0.35, len: 1 },
        bass: { kind: 'fm', gain: 0.55, duck: true, pats: { A: 'R...R...R...R...', B: 'R...R.r.R...5...' } },
        arp: { voice: (f, len, r) => I_.fmBell(f, len, r, { tau: 0.9, index: 2.4 }), pats: { A: '0.1.2.3.2.1.0.1.', B: '0.1.2.4.3.2.1.0.' }, bed: 0.16, peak: 0.2, oct: 24, rev: 0.6, dly: 0.8 },
        lead: {
            voice: (f, len, r) => I_.theremin(f, len, r), gain: 0.4, rev: 0.55, dly: 0.45, oct: 12, form: ['A', 'B', 'A', 'C', 'A', 'B', 'D', 'C'],
            phrases: { A: '0:4:5 6:2:6 8:6:7 16:4:8 20:2:7 22:2:6 24:8:5', B: '0:6:7 8:4:8 12:4:9 16:8:7 24:8:5', C: '2:2:5 4:2:6 6:2:7 8:8:9 16:4:8 20:4:7 24:8:6', D: '0:8:9 8:4:8 12:4:7 16:8:6 24:8:4' }
        },
        fx: { size: 0.88, damp: 0.28, predelay: 0.02, wet: 1.2, delayBeats: 0.75, fb: 0.5, delayDamp: 2600 },
        special(ctx, I) {
            for (const b of [0, 4, 8, 12]) ctx.put('bed', I.riser(ctx.r, ctx.barSec * 1.8, 500, 7000, 1), b * 16 + 24, { gain: 0.07, rev: 0.6, pan: 0 });
            for (const b of [0, 8]) ctx.put('peak', I.bellTree(ctx.r, 12), b * 16, { gain: 0.25, rev: 0.6, dly: 0.4 });
            for (const b of [1, 5, 9, 13]) ctx.put('bed', I.choir([mtof(72), mtof(76), mtof(79)], ctx.barSec * 2, ctx.r, { vowel: ['a', 'o'], level: 0.5 }), b * 16, { gain: 0.28, rev: 0.7 });
        }
    }),
    // forest: organic. marimba and kalimba, wooden flute hook, frame drum, birds, acoustic bass
    forest: std({
        id: 'forest', tempo: 96, root: 55, bassRoot: 43, scale: [0, 2, 5, 7, 11], prog: [0, 3, 1, 4, 0, 3, 1, 4, 2, 4, 1, 3, 0, 3, 2, 4],
        kit: { kick: 'soft', snare: 'rim', shaker: true, fill: 'tom', fillTom: 120 },
        drums: {
            kick: { A: 'X.....x...x.....', B: 'X.....x...x...x.' }, snare: { A: '....x.......x...' }, hat: { A: '.x...x...x...x..', B: '.x.x.x...x.x.x..' },
            perc(ctx, bar, base, I) { for (const s of [2, 9, 15]) ctx.put('drive', I.woodblock(s === 9 ? 900 : 660, ctx.r), base + s, { gain: 0.3, pan: s === 9 ? 0.4 : -0.4 }); if (bar % 2 === 1) ctx.put('drive', I.frameDrum(ctx.r), base + 12, { gain: 0.5 }); }
        },
        pad: { gain: 0.5, rev: 0.5, oct: 0, wide: true, opts: { voices: 3, spreadCents: 8, cutoff: 1200, movement: 0.4, attack: 0.7, release: 1, wave: 'tri' } },
        sub: { gain: 0.35, len: 0.8 },
        bass: { kind: 'pluck', gain: 0.8, pats: { A: 'R....5...r......', B: 'R....5...r...3..' } },
        arp: { voice: (f, len, r) => I_.marimba(f, len, r), pats: { A: '0.1.2.1.0.1.2.3.', B: '0.1.2.3.4.3.2.1.' }, bed: 0.3, peak: 0.28, oct: 12, rev: 0.4, dly: 0.2 },
        lead: {
            voice: (f, len, r) => I_.flute(f, len, r), gain: 0.55, rev: 0.4, dly: 0.12, octave: 0.2, oct: 12, form: ['A', 'B', 'A', 'C', 'A', 'B', 'D', 'C'],
            phrases: {
                A: '0:3:5 3:3:6 6:2:7 8:3:6 11:3:5 14:2:3 16:3:5 19:3:6 22:2:8 24:8:7', B: '0:3:7 3:3:8 6:2:7 8:3:6 11:3:5 14:2:6 16:4:7 20:4:5 24:8:4',
                C: '0:3:5 3:3:7 6:2:8 8:6:9 14:2:8 16:3:7 19:3:6 22:2:5 24:8:6', D: '0:2:3 2:2:5 4:2:6 6:2:5 8:3:3 11:3:5 14:2:6 16:8:7 24:8:5'
            }
        },
        fx: { size: 0.6, damp: 0.45, wet: 0.85, delayBeats: 0.5, fb: 0.28 },
        special(ctx, I) {
            texture(ctx, 'bed', 'wind', 0.05, 0.4); birds(ctx, 9, 0.22);
            for (let b = 0; b < 16; b += 2) ctx.put('bed', I.kalimba(mtof(ctx.cfg.root + 24 + [0, 7, 9, 4][(b >> 1) % 4]), 0.6, ctx.r), b * 16 + 8, { gain: 0.16, pan: 0.5, rev: 0.4, dly: 0.3 });
        }
    }),
    // industry: hard. distorted reese, four-on-the-floor, anvil hits, saw stabs and a riff lead
    industry: std({
        id: 'industry', tempo: 128, root: 43, bassRoot: 31, scale: [0, 1, 5, 7, 8], prog: [0, 0, 3, 4, 0, 0, 3, 4, 2, 2, 3, 4, 0, 3, 4, 0],
        sc: { depth: 0.62, release: 0.16 }, kit: { kick: 'hard', snare: 'hard', clap: true, hatBright: 1.25, fill: 'snare', snareRev: 0.1 },
        drums: {
            kick: { A: 'X...X...X...X...', B: 'X...X...X...X..x' }, snare: { A: '....X.......X...', B: '....X.......X.x.' }, hat: { A: '..x...x...x...x.', B: 'x.xxx.xxx.xxx.xx' }, open: { A: '..o...o...o...o.' },
            perc(ctx, bar, base, I) { if (bar % 2 === 0) ctx.put('drive', I.anvil(430 + 50 * (bar % 4), ctx.r), base + 2, { gain: 0.22, pan: -0.3, rev: 0.2 }); if (bar % 2 === 1) ctx.put('drive', I.anvil(640, ctx.r), base + 10, { gain: 0.18, pan: 0.3, rev: 0.2 }); }
        },
        pad: { gain: 0.45, rev: 0.25, oct: 0, opts: { voices: 5, spreadCents: 18, cutoff: 900, movement: 0.8, attack: 0.25, release: 0.5, q: 1.4 } },
        sub: { gain: 0.45, len: 0.95 },
        bass: { kind: 'reese', gain: 0.75, duck: true, gate: 0.8, pats: { A: 'R.R.....R.R.....', B: 'R.R...r.R.R...5.', pre: 'R.R.R.R.R.R.5.5.' } },
        stab: { voice: (f, len, r) => I_.sawStab(f, len, r, { hard: true }), pats: { A: 'x.......x.......', B: 'x.....x.x.......' }, len: 1.4, gain: 0.34, oct: 12, rev: 0.15 },
        arp: { voice: (f, len, r) => I_.sawStab(f, 0.09, r, { hard: true }), pats: { A: '0202020202020202', B: '0202030203020302' }, bed: 0.07, peak: 0.16, oct: 12, rev: 0.15, dly: 0.2, bedEvery: 4 },
        lead: {
            voice: (f, len, r) => I_.sawStab(f, len, r, { hard: true }), gain: 0.42, rev: 0.2, dly: 0.2, oct: 12, form: ['A', 'B', 'A', 'B', 'A', 'B', 'A', 'B'], octave: 0.35,
            phrases: {
                A: '0:2:5 2:1:5 3:1:6 4:2:5 6:2:7 8:2:5 10:1:5 11:1:6 12:2:5 14:2:8 16:2:5 18:1:5 19:1:6 20:2:5 22:2:7 24:4:6 28:4:5',
                B: '0:2:7 2:2:7 4:2:8 6:2:7 8:4:6 12:4:5 16:2:7 18:2:7 20:2:9 22:2:8 24:8:7'
            }
        },
        fx: { size: 0.35, damp: 0.6, wet: 0.5, delayBeats: 0.5, fb: 0.3 },
        special(ctx, I) {
            texture(ctx, 'bed', 'air', 0.02, 0.1);
            for (let b = 0; b < 16; b += 4) ctx.put('bed', I.anvil(mtof(55), ctx.r), b * 16, { gain: 0.15, rev: 0.4 });
            for (let b = 3; b < 16; b += 4) ctx.put('peak', I.riser(ctx.r, ctx.barSec, 300, 8000, 1), b * 16, { gain: 0.14 });
        }
    }),
    // glacier: glassy. FM glass bells, airy pad, heartbeat kick, long delays
    glacier: std({
        id: 'glacier', tempo: 84, root: 57, bassRoot: 45, scale: [0, 2, 3, 7, 10], prog: [0, 3, 1, 4, 0, 3, 1, 4, 2, 3, 1, 4, 0, 3, 2, 4],
        sc: { depth: 0.12, release: 0.4 }, kit: { kick: 'heart', snare: 'rim', hatBright: 1.6, snareRev: 0.5 }, peakPad: 0.3,
        drums: { fill: false, kick: { A: 'X.........x.....' }, snare: { A: '......x.........', B: '......x.....o...' }, hat: { A: '....x.......x...' } },
        pad: { gain: 0.55, rev: 0.6, oct: 0, wide: true, opts: { voices: 4, spreadCents: 10, cutoff: 2300, movement: 0.5, attack: 1.0, release: 1.4, wave: 'tri', noise: 0.02 } },
        sub: { gain: 0.4, len: 1 },
        bass: { kind: 'sine', gain: 0.6, duck: true, gate: 1.0, pats: { A: 'R.......R.......', B: 'R.......R.....r.' } },
        arp: { voice: (f, len, r) => I_.fmBell(f, len, r, { glass: true, tau: 1.2, index: 2.2 }), pats: { A: '0.1.2.3.2.1.0.1.', B: '0.1.2.4.3.2.1.0.' }, bed: 0.2, peak: 0.2, oct: 24, rev: 0.6, dly: 0.7 },
        lead: {
            voice: (f, len, r) => I_.fmBell(f, len, r, { ratio: 3.5, index: 2.0, tau: 1.6 }), gain: 0.45, rev: 0.55, dly: 0.4, gate: 1.4, oct: 12, form: ['A', 'B', 'C', 'D', 'A', 'B', 'C', 'D'],
            phrases: { A: '0:6:5 8:4:6 12:4:7 16:6:8 24:8:7', B: '0:4:7 6:2:6 8:8:5 16:4:6 20:4:7 24:8:9', C: '2:4:8 8:4:7 12:4:6 16:8:5 24:8:3', D: '0:8:9 8:8:7 16:4:8 20:4:7 24:8:5' }
        },
        fx: { size: 0.9, damp: 0.22, predelay: 0.025, wet: 1.2, delayBeats: 0.75, fb: 0.5, delayDamp: 3600 },
        special(ctx, I) {
            texture(ctx, 'bed', 'air', 0.03, 0.6);
            for (const b of [0, 8]) ctx.put('peak', I.bellTree(ctx.r, 14), b * 16, { gain: 0.3, rev: 0.7, dly: 0.5 });
            for (const b of [2, 6, 10, 14]) ctx.put('bed', I.choir([mtof(81), mtof(84)], ctx.barSec * 2, ctx.r, { vowel: ['o', 'u'], level: 0.35 }), b * 16, { gain: 0.2, rev: 0.8 });
        }
    }),
    // metro: neon. pumping four-on-the-floor, galloping bass, supersaw stabs and a syncopated hook
    metro: std({
        id: 'metro', tempo: 122, root: 50, bassRoot: 38, scale: [0, 2, 4, 7, 9], prog: [0, 3, 1, 4, 0, 3, 1, 4, 2, 3, 1, 4, 0, 3, 2, 4],
        sc: { depth: 0.68, release: 0.17 }, kit: { kick: 'punch', snare: 'tight', clap: true, hatBright: 1.1, fill: 'snare' },
        drums: { kick: { A: 'X...X...X...X...' }, snare: { A: '....X.......X...' }, hat: { A: 'x.x.x.x.x.x.x.x.', B: 'xoxoxoxoxoxoxoxo' }, open: { A: '..x...x...x...x.' } },
        pad: { gain: 0.6, rev: 0.35, oct: 0, wide: true, opts: { voices: 5, spreadCents: 14, cutoff: 2000, movement: 0.5, attack: 0.2, release: 0.5 } },
        sub: { gain: 0.35, len: 0.9 },
        bass: { kind: 'saw', gain: 0.65, duck: true, gate: 0.8, pats: { A: 'R..R..R.R..R..R.', B: '.R.R.R.R.R.R.R.r', pre: 'R..R..R.R.R.R.5.' } },
        stab: { voice: (f, len, r) => I_.superLead(f, len, r, { voices: 5, cents: 12, cutoffHi: 5000 }), pats: { A: '..x...x...x...x.', B: '..x..xx...x..xx.' }, len: 1.4, gain: 0.34, oct: 12, rev: 0.25, dly: 0.2 },
        arp: { voice: (f, len, r) => I_.sawStab(f, 0.1, r), pats: { A: '0.1.2.1.0.1.2.1.', B: '0.1.2.3.2.1.0.1.' }, bed: 0.1, peak: 0.2, oct: 24, rev: 0.3, dly: 0.45, bedEvery: 4 },
        lead: {
            voice: (f, len, r) => I_.superLead(f, len, r), gain: 0.42, rev: 0.3, dly: 0.35, octave: 0.3, oct: 12, form: ['A', 'B', 'A', 'C', 'A', 'B', 'A', 'C'],
            phrases: {
                A: '0:3:5 3:3:7 6:2:6 8:3:5 11:3:7 14:2:8 16:3:9 19:3:7 22:2:6 24:6:7 30:2:5', B: '0:3:5 3:3:7 6:2:6 8:3:5 11:3:7 14:2:9 16:3:8 19:3:7 22:2:5 24:8:6',
                C: '0:2:7 2:2:9 4:4:8 8:2:7 10:2:9 12:4:10 16:3:9 19:3:8 22:2:7 24:8:9'
            }
        },
        fx: { size: 0.55, damp: 0.4, wet: 0.8, delayBeats: 0.75, fb: 0.35 },
        special(ctx, I) {
            texture(ctx, 'bed', 'air', 0.02, 0.3);
            for (const b of [0, 4, 8, 12]) ctx.put('peak', I.reverseCrash(ctx.r, ctx.barSec * 0.5), b * 16 + 8, { gain: 0.12 });
        }
    })
};

const base = { swing: 0, marks: false, crash: 0.2, riser: 0.2 };
export const MENU = {
    ...base, id: 'menu', bars: 8, tempo: 104, root: 48, bassRoot: 36, scale: [0, 2, 4, 7, 9], prog: [0, 3, 1, 4, 0, 3, 2, 4], sc: { depth: 0.3, release: 0.25 },
    kit: { kick: 'soft', snare: 'clap', shaker: true, fill: 'snare' }, drums: { fill: false, kick: { A: 'X.......X...x...' }, snare: { A: '....x.......x...' }, hat: { A: '..x...x...x...x.' } },
    pad: { gain: 0.6, rev: 0.4, oct: 0, wide: true, opts: { voices: 5, spreadCents: 12, cutoff: 1500, movement: 0.4, attack: 0.5, release: 0.8 } }, sub: { gain: 0.35, len: 0.9 },
    bass: { kind: 'fm', gain: 0.5, duck: true, pats: { A: 'R.....R.R...5...' } },
    arp: { voice: (f, len, r) => I_.marimba(f, len, r), pats: { A: '0.1.2.1.0.1.2.3.' }, bed: 0.25, peak: 0.25, oct: 12, rev: 0.4, dly: 0.4 },
    lead: { voice: (f, len, r) => I_.fmBell(f, len, r, { tau: 0.9 }), gain: 0.4, rev: 0.4, dly: 0.35, oct: 24, form: ['A', 'B', 'A', 'C'], phrases: { A: '0:3:5 3:1:6 4:4:7 8:4:6 12:4:5 16:3:5 19:1:6 20:4:7 24:8:9', B: '0:4:7 4:4:8 8:8:9 16:4:8 20:4:7 24:8:5', C: '0:3:9 3:1:8 4:4:7 8:4:8 12:4:7 16:16:5' } },
    fx: { size: 0.6, damp: 0.4, wet: 0.8, delayBeats: 0.75, fb: 0.35 }, special() { }
};
export const RESULTS = {
    ...base, id: 'results', bars: 8, tempo: 90, root: 53, bassRoot: 41, scale: [0, 2, 4, 7, 9], prog: [0, 1, 3, 4, 0, 3, 1, 0],
    kit: { kick: 'soft', snare: 'brush', shaker: true, fill: 'snare' }, drums: { fill: false, kick: { A: 'X.......x.......' }, snare: { A: '....x.......x...' }, hat: { A: 'o.x.o.x.o.x.o.x.' } },
    pad: { gain: 0.6, rev: 0.45, oct: 0, wide: true, opts: { voices: 3, spreadCents: 9, cutoff: 1300, movement: 0.35, attack: 0.7, release: 1, wave: 'tri' } }, sub: { gain: 0.35, len: 0.9 },
    bass: { kind: 'pluck', gain: 0.75, pats: { A: 'R.......5.......' } },
    arp: { voice: (f, len, r) => I_.harp(f, len, r), pats: { A: '0.1.2.3.2.1.0.1.' }, bed: 0.3, peak: 0.25, oct: 12, rev: 0.45, dly: 0.25 },
    lead: { voice: (f, len, r) => I_.kalimba(f, len, r), gain: 0.5, rev: 0.4, dly: 0.25, oct: 24, form: ['A', 'B', 'A', 'C'], phrases: { A: '0:4:5 4:2:6 6:2:7 8:8:6 16:4:5 20:4:3 24:8:4', B: '0:4:7 4:4:8 8:6:7 14:2:6 16:8:5 24:8:3', C: '0:6:7 8:4:6 12:4:5 16:16:4' } },
    fx: { size: 0.65, damp: 0.4, wet: 0.8, delayBeats: 0.75, fb: 0.3 }, special() { }
};

import * as I_ from './music-inst.mjs';
void chordFreqs;
