// Score arranger: turns a declarative biome config into three exactly aligned, seamlessly looping stems (bed, drive, peak).
// Everything wraps round the loop: notes and effect tails fold back onto the start, LFOs complete whole cycles.
import { SR, mk, rng, seedOf, filter, loopProcess, reverb, pingPong, chorus, duckEnv, loopHz, mtof, softClip, compress, TAU } from './dsp.mjs';
import * as I from './music-inst.mjs';
import { CONFIGS, MENU, RESULTS } from './music-biomes.mjs';

export const BIOMES = ['alpine', 'desert', 'harbour', 'salt', 'forest', 'industry', 'glacier', 'metro'];
const VEL = { X: 1, x: 0.78, o: 0.36 };
const degNote = (scale, d) => { const L = scale.length; return scale[((d % L) + L) % L] + 12 * Math.floor(d / L); };
const pickPat = (p, bar) => {
    if (!p) return '';
    if (typeof p === 'string') return p;
    const q = bar % 4;
    if (q === 3 && p.fill && (bar === 15 || bar === 7 || p.fillAll)) return p.fill;
    if (q === 3 && p.pre) return p.pre;
    if (bar >= 8 && p.B) return p.B;
    return p.A ?? '';
};
const hits = str => [...str].map((c, i) => VEL[c] ? [i, VEL[c]] : null).filter(Boolean);
const parsePhrase = text => text.trim().split(/\s+/).map(token => token.split(':').map(Number));

function newStem(N) { return { L: mk(N), R: mk(N), rev: mk(N), dly: mk(N) }; }

export function compose(cfg) {
    const bars = cfg.bars ?? 16, N = Math.round(bars * 4 * 60 / cfg.tempo * SR), stepLen = N / (bars * 16), beatSec = 60 / cfg.tempo, barSec = beatSec * 4;
    const r = rng(seedOf('music:' + cfg.id)), human = () => (r() - 0.5) * 2;
    const stems = { bed: newStem(N), drive: newStem(N), peak: newStem(N) };
    const sw = step => step + (step % 2 === 1 ? (cfg.swing ?? 0) : 0);
    // sidechain from the kick pattern, used as a ghost on the harmonic layers in every stem
    const kickTimes = [];
    for (let b = 0; b < bars; b++) for (const [s] of hits(pickPat(cfg.drums.kick, b))) kickTimes.push((b * 16 + s) * stepLen / SR);
    const duck = cfg.sc ? duckEnv(N, kickTimes, { depth: cfg.sc.depth, release: cfg.sc.release ?? 0.2 }) : null;
    const ctx = { N, stepLen, bars, r, beatSec, barSec, cfg, stems, duck };
    ctx.put = (stem, buf, step, o = {}) => {
        const s = stems[stem], gain = (o.gain ?? 1) * (o.vel ?? 1), a = ((o.pan ?? 0) + 1) * Math.PI / 4, gl = Math.cos(a) * gain, gr = Math.sin(a) * gain;
        const start = Math.round(step * stepLen + (o.human ? human() * o.human * SR : 0)), rv = o.rev ?? 0, dl = o.dly ?? 0, useDuck = o.duck && duck;
        for (let i = 0; i < buf.length; i++) {
            const idx = ((start + i) % N + N) % N, v = buf[i] * (useDuck ? duck[idx] : 1);
            s.L[idx] += v * gl; s.R[idx] += v * gr;
            if (rv) s.rev[idx] += v * rv * gain; if (dl) s.dly[idx] += v * dl * gain;
        }
    };
    const chordDegs = bar => { const d = cfg.prog[bar % cfg.prog.length]; return [d, d + 2, d + 4]; };
    const midiOf = (base, deg) => base + degNote(cfg.scale, deg);
    const pad = cfg.pad;

    // ---- pads: bed (always), plus a thin octave layer in peak ----
    if (pad) {
        const lfo = loopHz(N, 1 / (barSec * (bars / 2)));
        for (let b = 0; b < bars; b++) {
            const [d0, d1, d2] = chordDegs(b), freqs = [midiOf(cfg.root + (pad.oct ?? 0), d0), midiOf(cfg.root + (pad.oct ?? 0), d1), midiOf(cfg.root + (pad.oct ?? 0), d2), midiOf(cfg.root + (pad.oct ?? 0), d0 + 5)].map(mtof);
            const open = (pad.openBars ? pad.openBars[b % pad.openBars.length] : 1) * (1 + 0.12 * (b >= 8 ? 1 : 0));
            const buf = pad.voice === 'choir' ? I.choir(freqs.slice(0, 3), barSec + 0.4, r, { vowel: pad.vowel ?? ['a', 'o'], level: pad.level ?? 0.5 })
                : I.padChord(freqs, barSec + 0.35, { ...pad.opts, cutoff: (pad.opts?.cutoff ?? 1400) * open, lfoHz: lfo, lfoPhase: TAU * lfo * (b * barSec) }, r);
            if (pad.wide) {
                const buf2 = I.padChord(freqs, barSec + 0.35, { ...pad.opts, cutoff: (pad.opts?.cutoff ?? 1400) * open, lfoHz: lfo, lfoPhase: TAU * lfo * (b * barSec) + 1.1 }, r);
                ctx.put('bed', buf, b * 16, { gain: (pad.gain ?? 0.8) * 0.75, rev: pad.rev ?? 0.35, pan: -0.55, duck: true });
                ctx.put('bed', buf2, b * 16, { gain: (pad.gain ?? 0.8) * 0.75, rev: pad.rev ?? 0.35, pan: 0.55, duck: true });
            } else ctx.put('bed', buf, b * 16, { gain: pad.gain ?? 0.8, rev: pad.rev ?? 0.35, pan: 0, duck: true });
            if (cfg.peakPad) ctx.put('peak', I.padChord(freqs.map(f => f * 2), barSec + 0.3, { ...(pad.opts ?? {}), cutoff: 3200, voices: 3, attack: 0.3 }, r), b * 16, { gain: cfg.peakPad, rev: 0.4, duck: true });
        }
    }
    // ---- soft sub (bed) ----
    if (cfg.sub) {
        for (let b = 0; b < bars; b++) {
            const f = mtof(midiOf(cfg.bassRoot, chordDegs(b)[0]));
            ctx.put('bed', I.bass('sine', f, barSec * (cfg.sub.len ?? 0.9), r), b * 16, { gain: cfg.sub.gain ?? 0.5, duck: true });
        }
    }
    // ---- texture hook: custom ambience in the bed ----
    // ---- drums ----
    const D = cfg.drums, kit = cfg.kit;
    for (let b = 0; b < bars; b++) {
        const base = b * 16;
        for (const [s, v] of hits(pickPat(D.kick, b))) ctx.put('drive', I.kick(kit.kick, r), base + s, { gain: 0.95 * v });
        for (const [s, v] of hits(pickPat(D.snare, b))) {
            ctx.put('drive', I.snare(kit.snare, r), sw(base + s), { gain: 0.6 * v, rev: kit.snareRev ?? 0.18, human: 0.0015 });
            if (kit.clap) ctx.put('drive', I.snare('clap', r), sw(base + s), { gain: 0.35 * v, rev: 0.25 });
        }
        for (const [s, v] of hits(pickPat(D.hat, b))) ctx.put('drive', kit.shaker ? I.shaker(r) : I.hat(false, r, kit.hatBright ?? 1), sw(base + s), { gain: 0.26 * v * (0.85 + 0.3 * r()), pan: (s % 4 === 0 ? -0.25 : 0.25), human: 0.001 });
        for (const [s, v] of hits(pickPat(D.open, b))) ctx.put('drive', kit.shaker ? I.shaker(r, true) : I.hat(true, r, kit.hatBright ?? 1), sw(base + s), { gain: 0.27 * v, pan: 0.2, rev: 0.12 });
        if (D.perc) D.perc(ctx, b, base, I, hits, pickPat);
        // fills at the end of each four-bar group
        if (b % 4 === 3 && D.fill !== false) {
            const big = b === 7 || b === 15, f = kit.fillTom ?? 150;
            for (let s = (big ? 8 : 12); s < 16; s++) {
                const rise = (s - (big ? 8 : 12)) / (big ? 8 : 4);
                if (kit.fill === 'tom') ctx.put('drive', I.tom(f * (1.5 - 0.5 * rise) * (s % 2 ? 1.2 : 1), r), base + s, { gain: 0.55 + 0.35 * rise, pan: -0.4 + 0.8 * rise, rev: 0.12 });
                else ctx.put('drive', I.snare(kit.snare === 'brush' ? 'brush' : 'tight', r), base + s, { gain: 0.2 + 0.4 * rise, rev: 0.12 });
            }
        }
    }
    // ---- bass (drive) ----
    if (cfg.bass) {
        const map = { R: 0, '3': 2, '5': 4, r: 5, l: -5, '7': 3 };
        for (let b = 0; b < bars; b++) {
            const pat = pickPat(cfg.bass.pats, b), d = chordDegs(b)[0], onsets = [...pat].map((c, i) => (c in map) ? [i, map[c]] : null).filter(Boolean);
            onsets.forEach(([s, off], k) => {
                const nextS = k + 1 < onsets.length ? onsets[k + 1][0] : 16, len = Math.max(1, nextS - s) * stepLen / SR * (cfg.bass.gate ?? 0.9);
                const f = mtof(midiOf(cfg.bassRoot, d + off));
                ctx.put('drive', I.bass(cfg.bass.kind, f, len, r), sw(b * 16 + s), { gain: (cfg.bass.gain ?? 0.7) * (0.9 + 0.2 * r()), duck: !!cfg.bass.duck, human: 0.0008 });
            });
        }
    }
    // ---- rhythm stabs / strums (drive) ----
    if (cfg.stab) {
        for (let b = 0; b < bars; b++) {
            const pat = pickPat(cfg.stab.pats, b), [d0, d1, d2] = chordDegs(b);
            for (const [s, v] of hits(pat)) {
                const len = (cfg.stab.len ?? 1.5) * stepLen / SR;
                const chordNotes = cfg.stab.voicing ? cfg.stab.voicing.map(o => d0 + o) : [d0, d1, d2];
                chordNotes.forEach((deg, k) => ctx.put('drive', cfg.stab.voice(mtof(midiOf(cfg.root + (cfg.stab.oct ?? 0), deg)), len, r), sw(b * 16 + s) + k * (cfg.stab.strum ?? 0), { gain: (cfg.stab.gain ?? 0.3) * v / Math.sqrt(chordNotes.length), pan: (k - 1) * 0.25, rev: cfg.stab.rev ?? 0.2, dly: cfg.stab.dly ?? 0, duck: true, human: 0.001 }));
            }
        }
    }
    // ---- arps: soft in the bed, full in the peak ----
    if (cfg.arp) {
        const A = cfg.arp;
        for (let b = 0; b < bars; b++) {
            const pat = pickPat(A.pats, b), d = chordDegs(b)[0], tones = [d, d + 2, d + 4, d + 5, d + 7, d + 9];
            [...pat].forEach((c, s) => {
                if (c === '.' || c === ' ') return;
                const deg = tones[Number(c)], f = mtof(midiOf(cfg.root + (A.oct ?? 12), deg)), vel = (s % 4 === 0 ? 1 : 0.72) * (0.85 + 0.3 * r());
                const buf = A.voice(f, A.len ?? 0.25, r);
                if (A.bed && s % (A.bedEvery ?? 2) === 0) ctx.put('bed', buf, sw(b * 16 + s), { gain: A.bed * vel, pan: ((s >> 1) % 2 ? 0.4 : -0.4), rev: A.rev ?? 0.4, dly: A.dly ?? 0.3, duck: true });
                if (A.peak) ctx.put('peak', buf, sw(b * 16 + s), { gain: A.peak * vel * (b >= 8 ? 1.1 : 0.9), pan: ((s >> 1) % 2 ? -0.35 : 0.35), rev: A.rev ?? 0.4, dly: A.dly ?? 0.3, duck: true, human: 0.001 });
            });
        }
    }
    // ---- lead hooks (peak) ----
    if (cfg.lead) {
        const L = cfg.lead, ph = Object.fromEntries(Object.entries(L.phrases).map(([k, v]) => [k, parsePhrase(v)]));
        const form = L.form;
        form.forEach((key, slot) => {
            const bar0 = slot * 2;
            if (bar0 >= bars) return;
            for (const [s, len, deg] of ph[key]) {
                const f = mtof(midiOf(cfg.root + (L.oct ?? 12), deg + (L.shift?.[slot] ?? 0))), lenSec = len * stepLen / SR * (L.gate ?? 0.92);
                const buf = L.voice(f, lenSec, r, { glideFrom: L.glide ? mtof(midiOf(cfg.root + (L.oct ?? 12), deg - 1)) : 0 });
                ctx.put('peak', buf, sw(bar0 * 16 + s), { gain: (L.gain ?? 0.5) * (0.92 + 0.16 * r()), rev: L.rev ?? 0.3, dly: L.dly ?? 0.25, duck: !!L.duck, human: 0.002 });
                if (L.octave && bar0 >= 8) ctx.put('peak', L.voice(f * 2, lenSec, r, {}), sw(bar0 * 16 + s), { gain: (L.gain ?? 0.5) * L.octave, rev: L.rev ?? 0.3, dly: L.dly ?? 0.25, pan: 0.2 });
            }
        });
    }
    // ---- biome specials (textures, ambience, extra voices) ----
    if (cfg.special) cfg.special(ctx, I);
    // ---- section markers: crashes and risers in the peak stem ----
    if (cfg.marks !== false && bars >= 8) {
        for (const b of [0, 8].filter(x => x < bars)) ctx.put('peak', I.crash(r, 2.4), b * 16, { gain: cfg.crash ?? 0.22, rev: 0.3 });
        for (const b of bars >= 16 ? [6, 14] : [bars - 2]) ctx.put('peak', I.riser(r, barSec * 2, 400, 9000, 1), b * 16, { gain: cfg.riser ?? 0.2, rev: 0.25, pan: 0 });
        ctx.put('drive', I.reverseCrash(r, barSec * 0.9), (bars - 1) * 16, { gain: 0.18, rev: 0.2 });
    }

    // ---- effects and master per stem ----
    const out = {};
    for (const key of Object.keys(stems)) {
        const s = stems[key], g = cfg.fx;
        let [L, R] = [s.L, s.R];
        const [rl, rr] = loopProcess([s.rev, s.rev], ch => reverb(ch, { size: g.size, damp: g.damp, wet: 1, width: 1, predelay: g.predelay ?? 0.012 }));
        const [dl, dr] = loopProcess([s.dly, s.dly], ch => pingPong(ch, { time: beatSec * (g.delayBeats ?? 0.75), feedback: g.fb ?? 0.38, mix: 1, damp: g.delayDamp ?? 3200 }));
        const mixL = mk(N), mixR = mk(N);
        for (let i = 0; i < N; i++) { mixL[i] = L[i] + (rl[i] - s.rev[i]) * (g.wet ?? 1) + (dl[i] - s.dly[i] * 1) * 0.8 + 0; mixR[i] = R[i] + (rr[i] - s.rev[i]) * (g.wet ?? 1) + (dr[i] - s.dly[i] * 1) * 0.8; }
        let pair = [filter(mixL, 'hp', 28, 0.7), filter(mixR, 'hp', 28, 0.7)];
        pair = loopProcess(pair, ch => compress(ch, { threshold: -20, ratio: 2.2, attack: 0.012, release: 0.14, makeup: 2, warm: false }));
        out[key] = [softClip(pair[0], 1.05, 0.5), softClip(pair[1], 1.05, 0.5)];
    }
    return out;
}

export const renderBiome = id => compose(CONFIGS[id]);
const mixdown = stems => { const keys = Object.keys(stems), n = stems[keys[0]][0].length, out = [mk(n), mk(n)]; for (const k of keys) for (let c = 0; c < 2; c++) for (let i = 0; i < n; i++) out[c][i] += stems[k][c][i]; return out; };
export const renderMenu = () => mixdown(compose(MENU));
export const renderResults = () => mixdown(compose(RESULTS));
void chorus;
