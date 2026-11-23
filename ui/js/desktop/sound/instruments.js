// Instrument models for desktop UI sounds: fifteen parameterised voices built only on DesktopSoundDSP.
// Every model renders a stereo { L, R } of exactly ceil(note.dur * SR) samples with note.gain and note.pan applied.
// All randomness comes from the `rand` argument in a fixed order, so renders are deterministic.
// The constant tables at the top of each model (partials, presets, mix levels, `trim`) are meant to be retuned by ear;
// `trim` is the per-model output level that puts a default note between -12 and -3 dBFS (target about -7.5 dBFS).
(function (root) {
    'use strict';

    const D = root.DesktopSoundDSP;
    if (!D) throw new Error('DesktopSoundDSP must be loaded before DesktopSoundInstruments');
    const { SR, clamp, applyEnv, modal, fm, osc, pluck, bubble, noise, svf, highpass, lowpass, saturate, panMono, addMono } = D;
    const TAU = Math.PI * 2;
    const FADE_OUT = 0.005; // raised-cosine fade at the end of every render (s)

    // ---- shared helpers ------------------------------------------------------------------------
    const lengthOf = note => Math.max(1, Math.ceil(note.dur * SR));
    // Duration to hand to kernel sources so that ceil(dur * SR) is exactly n.
    const srcDur = n => (n - 0.5) / SR;
    const semis = st => Math.pow(2, st / 12);
    const cents = c => Math.pow(2, c / 1200);
    const zeros = n => ({ L: new Float32Array(n), R: new Float32Array(n) });
    // One random offset within +-detune/2 cents for models without per-partial spread.
    function offsetFreq(freq, note, rand) {
        const d = note.detune ?? 0;
        return d > 0 ? freq * cents((rand() * 2 - 1) * d / 2) : freq;
    }
    // Adds `src` (mono, optional start offset in seconds) to a stereo canvas with an equal-power pan.
    function addPan(st, src, gain, pan, at = 0) {
        const a = (clamp(pan ?? 0, -1, 1) + 1) * Math.PI / 4;
        const gl = Math.cos(a) * Math.SQRT2 * gain, gr = Math.sin(a) * Math.SQRT2 * gain;
        const off = Math.max(0, Math.round(at * SR));
        const m = Math.min(src.length, st.L.length - off);
        for (let i = 0; i < m; i++) { st.L[off + i] += src[i] * gl; st.R[off + i] += src[i] * gr; }
    }
    function addInto(dst, src, at, gain) {
        const off = Math.max(0, Math.round(at * SR));
        const m = Math.min(src.length, dst.length - off);
        for (let i = 0; i < m; i++) dst[off + i] += src[i] * gain;
    }
    function fadeOut(buf) {
        const n = buf.length, f = Math.min(n, Math.round(FADE_OUT * SR));
        for (let i = 0; i < f; i++) buf[n - 1 - i] *= 0.5 + 0.5 * Math.cos(Math.PI * (1 - i / f));
    }
    // Mono model result: gain, trim and equal-power pan, then the end fade.
    function finishMono(buf, note, trim) {
        const st = panMono(buf, (note.gain ?? 1) * trim, note.pan);
        fadeOut(st.L); fadeOut(st.R);
        return st;
    }
    // Stereo model result. With `balance` the note pan attenuates the far side; models that placed their
    // voices with the note pan already pass balance = false.
    function finishStereo(st, note, trim, balance) {
        const g = (note.gain ?? 1) * trim;
        const pan = balance ? clamp(note.pan ?? 0, -1, 1) : 0;
        const gl = g * (pan > 0 ? 1 - pan : 1), gr = g * (pan < 0 ? 1 + pan : 1);
        for (let i = 0; i < st.L.length; i++) { st.L[i] *= gl; st.R[i] *= gr; }
        fadeOut(st.L); fadeOut(st.R);
        return st;
    }
    // Enveloped noise burst: raised-cosine attack, exponential decay (-60 dB at `decay`).
    function burst(rand, color, len, attack, decay) {
        const b = noise(len, color, rand);
        return applyEnv(b, { attack: Math.min(attack, len / 2), decay, sustain: 0 });
    }
    // Air bed: a faint breath of band-passed pink noise (independent L and R, centred, not panned) that follows the
    // note. Its peak is `air` times the peak of the finished tonal part. Draws no random numbers when air is 0.
    const AIR = { lengthT60: 0.6, lengthExtra: 0.06, centerMul: 2.5, minHz: 600, maxHz: 8000, q: 0.5, attack: 0.010, decayT60: 0.5 };
    function addAir(res, note, air, t60, rand) {
        if (!(air > 0)) return res;
        const n = res.L.length;
        let peak = 0;
        for (let i = 0; i < n; i++) peak = Math.max(peak, Math.abs(res.L[i]), Math.abs(res.R[i]));
        const len = Math.min(n, Math.ceil((AIR.lengthT60 * t60 + AIR.lengthExtra) * SR));
        const fc = clamp(AIR.centerMul * note.freq, AIR.minHz, AIR.maxHz);
        const beds = [0, 1].map(() => {
            const b = noise((len - 0.5) / SR, 'pink', rand);
            bandpass(b, fc, AIR.q);
            return applyEnv(b, { attack: AIR.attack, decay: AIR.decayT60 * t60, sustain: 0 });
        });
        let bp = 0;
        for (const b of beds) for (let i = 0; i < b.length; i++) bp = Math.max(bp, Math.abs(b[i]));
        if (bp < 1e-12) return res;
        const k = air * peak / bp;
        for (let i = 0; i < len; i++) { res.L[i] += beds[0][i] * k; res.R[i] += beds[1][i] * k; }
        return res;
    }
    const bandpass = (buf, fc, q) => svf(buf, { type: 'bp', cutoff: fc, q });
    // Phase loop with a per-sample frequency function; returns the unit sine.
    function sineLoop(n, freqAt) {
        const out = new Float32Array(n);
        let ph = 0;
        for (let i = 0; i < n; i++) {
            out[i] = Math.sin(ph);
            ph += TAU * freqAt(i / SR) / SR;
            if (ph >= TAU) ph -= TAU;
        }
        return out;
    }

    // ---- glass ---------------------------------------------------------------------------------
    const GLASS = {
        partials: { r: [1, 2.0, 2.756, 5.404, 8.933], g: [1, 0.22, 0.16, 0.07, 0.025] },
        mainGain: 0.62, twinGain: 0.38,   // modal main and beat twin level
        mainPan: -0.3, twinPan: 0.35,     // x width
        strikeMs: 0.003, strikeCenter: 3, strikeMaxHz: 9000, strikeQ: 1.5, strikeGain: 0.9, strikeAttack: 0.0003,
        trim: 0.34
    };

    // ---- bar -----------------------------------------------------------------------------------
    const BAR = {
        kinds: {
            marimba: { r: [1, 3.93, 9.2], g: [1, 0.24, 0.08], t60: 0.5, damp: 1.25 },
            xylophone: { r: [1, 3, 6.5], g: [1, 0.3, 0.1], t60: 0.3, damp: 1.4 },
            glockenspiel: { r: [1, 2.76, 5.4, 8.93], g: [1, 0.45, 0.22, 0.1], t60: 1.2, damp: 0.5 },
            kalimba: { r: [1, 5.4, 14], g: [1, 0.16, 0.05], t60: 0.8, damp: 1 },
            woodblock: { r: [1, 2.32, 3.81], g: [1, 0.5, 0.3], t60: 0.08, damp: 0.6 }
        },
        malletMs: 0.012, malletQ: 1.2, malletLevel: 0.6, malletBase: 0.5, malletAttack: 0.0004,
        upperBase: 0.6, upperHard: 0.8,   // upper partials x (base + hard * hardness)
        bodyT60: 0.2, bodyGain: 0.3, bodyMinHz: 130,
        trim: 0.348
    };

    // ---- bell ----------------------------------------------------------------------------------
    const BELL = {
        kinds: {
            bell: { mods: [{ ratio: 3.5, index: 2.5, t60: 1.2 }, { ratio: 1.41, index: 0.5, t60: 0.6 }], t60: 1.4 },
            celesta: { mods: [{ ratio: 1, index: 1.2, t60: 0.35 }, { ratio: 4, index: 0.4, t60: 0.05 }], t60: 0.6 },
            tine: { mods: [{ ratio: 1, index: 1.1, t60: 0.5, floor: 0.15 }, { ratio: 14, index: 0.5, t60: 0.06 }], t60: 0.55 }
        },
        twinCents: 3, twinPan: 0.5,
        strikeMs: 0.004, strikeCenter: 5, strikeMaxHz: 9000, strikeQ: 1.2, strikeGain: 0.5, strikeAttack: 0.0003, strikeIndexMax: 1.5,
        trim: 0.42
    };

    // ---- string --------------------------------------------------------------------------------
    const STRING = {
        kinds: {
            nylon: { brightness: 0.35, pick: 0.22, t60: 0.8 },
            harp: { brightness: 0.55, pick: 0.3, t60: 1.2 },
            muted: { brightness: 0.2, pick: 0.12, t60: 0.2 }
        },
        maxFreq: 4000,
        loopT60: 3,                       // loop loss t60 = t60 x this, the envelope then ends the note at t60
        bodyHz: 280, bodyQ: 1.4, bodyGain: 0.5,
        pickMs: 0.002, pickBase: 1500, pickRange: 4500, pickQ: 1.5, pickGain: 0.25, pickBright: 0.4, pickAttack: 0.0002,
        trim: 0.394
    };

    // ---- analog --------------------------------------------------------------------------------
    const ANALOG = { subMix: 0.35, trim: 0.446 };

    // ---- chip ----------------------------------------------------------------------------------
    const CHIP = { lowpassHz: 9000, vibratoRamp: 0.05, trim: 0.381 };

    // ---- drop ----------------------------------------------------------------------------------
    const DROP = {
        lengthT60: 2.5, memberFade: 0.12, startJitter: 0.6,
        splashMs: 0.09, splashFrom: 3200, splashTo: 1200, splashQ: 1.2, splashDecay: 0.07, splashLevel: 0.3,
        trim: 0.463
    };

    // ---- blip ----------------------------------------------------------------------------------
    const BLIP = { boingDepth: 1.25, boingDecay: 0.15, harm2: 0.5, harm3: 0.25, trim: 0.43 };

    // ---- pad -----------------------------------------------------------------------------------
    const PAD = { shimmerRate: [0.5, 1.5], shimmerDepth: 0.3, trim: 0.104 };

    // ---- metal ---------------------------------------------------------------------------------
    const METAL = {
        kinds: {
            plate: { r: [1, 1.59, 2.14, 2.30, 2.65, 2.92, 3.16, 3.50], g: [1, 0.8, 0.6, 0.55, 0.45, 0.4, 0.3, 0.25] },
            pipe: { r: [1, 2.76, 5.40, 8.93, 13.34], g: [1, 0.6, 0.35, 0.2, 0.1] },
            ping: { r: [1, 2.32, 4.25, 6.63], g: [1, 0.5, 0.3, 0.15] }
        },
        mainGain: 0.62, twinGain: 0.38, mainPan: -0.3, twinPan: 0.35,
        strikeMs: 0.006, strikeHz: 2000, strikeLevel: 0.4, strikeFloor: 0.3, strikeAttack: 0.0003,
        trim: 0.195
    };

    // ---- click ---------------------------------------------------------------------------------
    const CLICK = {
        materials: {
            felt: { tone: 1200, ring: 0.012, noise: 0.8, q: 0.8, length: 0.006 },
            relay: { tone: 2600, ring: 0.02, noise: 0.6, q: 2.5, length: 0.003, double: 0.012 },
            plastic: { tone: 1800, ring: 0.025, noise: 0.5, q: 2, length: 0.004 },
            glass: { tone: 5200, ring: 0.04, noise: 0.3, q: 4, length: 0.002 },
            copper: { tone: 3400, ring: 0.05, noise: 0.4, q: 3, length: 0.003, second: 1.48 },
            stone: { tone: 1500, ring: 0.008, noise: 0.9, q: 1, length: 0.005 }
        },
        ringShare: 0.5,                   // resonance level x (1 - ringShare * noise)
        secondGain: 0.6, doubleLevel: 0.5, doubleTone: 1.06, burstGain: 1,
        trim: 0.68
    };

    // ---- thud ----------------------------------------------------------------------------------
    const THUD = { noiseMs: 0.04, noiseDecay: 0.03, noiseHighpass: 130, noiseLevel: 0.6, noiseAttack: 0.0004, trim: 0.441 };

    // ---- noise ---------------------------------------------------------------------------------
    const NOISE = { flutterHz: 27, trim: 1.35 };

    // ---- squelch -------------------------------------------------------------------------------
    const SQUELCH = { trim: 2.64 };

    // ---- debris --------------------------------------------------------------------------------
    const DEBRIS = {
        wood: { lo: 600, range: 1200, t60: 0.03, burstMs: 0.003, burstQ: 2, burstGain: 0.5, level: 1 },
        stone: { lo: 1500, range: 3500, burstMs: 0.002, q: 3, ringT60: 0.01, ringGain: 0.5, level: 1 },
        earth: { lo: 200, range: 500, ms: 0.006, highpass: 150, level: 4 },
        attack: 0.0003, trim: 0.876
    };

    // ---- parameter spec helpers ----------------------------------------------------------------
    const MODELS = {};
    const num = (a, b, c) => [a, b, c];
    const wave = (vals, def) => ({ values: vals, def });

    MODELS.glass = {
        params: {
            t60: num(0.1, 3, 0.9), damp: num(0, 2, 0.75), beat: num(0, 3, 0.55), attack: num(0.001, 0.05, 0.004),
            tilt: num(-1, 2, 0), width: num(0, 1, 0.6), strike: num(0, 1, 0.08),
            spread: num(0, 12, 3), air: num(0, 0.2, 0.04)
        },
        pitched: () => true,
        minFreq: () => 130,
        duration: p => p.t60 * 1.3 + p.attack,
        render(p, note, rand) {
            const n = lengthOf(note), w = p.width;
            const { a, b } = modal({
                freq: note.freq, dur: srcDur(n), t60: p.t60, damp: p.damp, beat: p.beat, attack: p.attack, tilt: p.tilt,
                spread: p.spread + (note.detune ?? 0), partials: GLASS.partials.r.map((r, i) => ({ r, g: GLASS.partials.g[i] }))
            }, rand);
            const st = zeros(n);
            addPan(st, a, b ? GLASS.mainGain : 1, note.pan + GLASS.mainPan * w);
            if (b) addPan(st, b, GLASS.twinGain, note.pan + GLASS.twinPan * w);
            if (p.strike > 0) { // tapped-glass transient: very short band-passed noise around 3x the note
                const s = burst(rand, 'white', GLASS.strikeMs, GLASS.strikeAttack, GLASS.strikeMs);
                bandpass(s, Math.min(GLASS.strikeMaxHz, GLASS.strikeCenter * note.freq), GLASS.strikeQ);
                addPan(st, s, p.strike * GLASS.strikeGain, note.pan);
            }
            return addAir(finishStereo(st, note, GLASS.trim, false), note, p.air, p.t60, rand);
        }
    };

    MODELS.bar = {
        params: {
            kind: wave(['marimba', 'xylophone', 'glockenspiel', 'kalimba', 'woodblock'], 'marimba'),
            t60: num(0.03, 2.5, 0.5), damp: num(0, 2.5, 1.25), hardness: num(0, 1, 0.5),
            attack: num(0.0005, 0.02, 0.0015), body: num(0, 1, 0.2), mallet: num(0, 1, 0.3),
            spread: num(0, 12, 3), air: num(0, 0.2, 0.04)
        },
        presets: { kind: BAR.kinds },
        pitched: () => true,
        minFreq: () => 130,
        duration: p => p.t60 * 1.2 + 0.02,
        render(p, note, rand) {
            const n = lengthOf(note), f = note.freq, h = p.hardness;
            const partials = p.r.map((r, i) => ({ r, g: p.g[i] * (i === 0 ? 1 : BAR.upperBase + BAR.upperHard * h) }));
            const { a } = modal({ freq: f, dur: srcDur(n), t60: p.t60, damp: p.damp, attack: p.attack, spread: p.spread + (note.detune ?? 0), partials }, rand);
            if (p.body > 0 && f / 2 >= BAR.bodyMinHz) {
                const body = modal({ freq: f, dur: srcDur(n), damp: 0, attack: p.attack, partials: [{ r: 0.5, g: p.body * BAR.bodyGain, t60: BAR.bodyT60 }] }, rand).a;
                addInto(a, body, 0, 1);
            }
            if (p.mallet > 0) {
                const m = burst(rand, 'white', BAR.malletMs, BAR.malletAttack, BAR.malletMs);
                bandpass(m, Math.min(8000, f * (2 + 4 * h)), BAR.malletQ);
                addInto(a, m, 0, p.mallet * (BAR.malletBase + h) * BAR.malletLevel / 0.3);
            }
            return addAir(finishMono(a, note, BAR.trim), note, p.air, p.t60, rand);
        }
    };

    MODELS.bell = {
        params: {
            kind: wave(['bell', 'celesta', 'tine'], 'bell'),
            t60: num(0.1, 3, 1.4), index: num(0, 4, 1), attack: num(0.001, 0.03, 0.002), spread: num(0, 1, 0), air: num(0, 0.2, 0.04)
        },
        presets: { kind: BELL.kinds },
        pitched: () => true,
        minFreq: () => 130,
        duration: p => p.t60 + p.attack + 0.02,
        render(p, note, rand) {
            const n = lengthOf(note), st = zeros(n);
            const voices = p.spread > 0 ? 2 : 1;
            const norm = voices === 2 ? 1 / Math.sqrt(1 + p.spread * p.spread) : 1;
            for (let v = 0; v < voices; v++) {
                const f = offsetFreq(note.freq, note, rand) * (v === 1 ? cents(BELL.twinCents) : 1);
                const mods = p.mods.map(m => ({ ratio: m.ratio, index: m.index * p.index, t60: m.t60, floor: (m.floor ?? 0) * p.index }));
                const out = fm({ freq: f, dur: srcDur(n), ratio: 1, mods });
                applyEnv(out, { attack: p.attack, decay: p.t60, sustain: 0 });
                const pan = voices === 1 ? note.pan : note.pan + (v === 0 ? -1 : 1) * BELL.twinPan * p.spread;
                addPan(st, out, v === 0 ? norm : norm * p.spread, pan);
            }
            const lvl = BELL.strikeGain * Math.min(p.index, BELL.strikeIndexMax) / BELL.strikeIndexMax;
            if (lvl > 0) {
                const s = burst(rand, 'white', BELL.strikeMs, BELL.strikeAttack, BELL.strikeMs);
                bandpass(s, Math.min(BELL.strikeMaxHz, BELL.strikeCenter * note.freq), BELL.strikeQ);
                addPan(st, s, lvl, note.pan);
            }
            return addAir(finishStereo(st, note, BELL.trim, false), note, p.air, p.t60, rand);
        }
    };

    MODELS.string = {
        params: {
            kind: wave(['nylon', 'harp', 'muted'], 'nylon'),
            t60: num(0.05, 3, 0.8), brightness: num(0.02, 0.98, 0.35), pick: num(0.05, 0.5, 0.22), body: num(0, 1, 0.2), air: num(0, 0.2, 0.04)
        },
        presets: { kind: STRING.kinds },
        pitched: () => true,
        minFreq: () => 130,
        duration: p => p.t60 * 1.1,
        render(p, note, rand) {
            const n = lengthOf(note), f = offsetFreq(note.freq, note, rand);
            if (f > STRING.maxFreq) throw new Error('string: frequency above ' + STRING.maxFreq + ' Hz');
            const out = pluck({ freq: f, dur: srcDur(n), t60: p.t60 * STRING.loopT60, brightness: p.brightness, pick: p.pick }, rand);
            if (p.body > 0) {
                const body = bandpass(out.slice(), STRING.bodyHz, STRING.bodyQ);
                addInto(out, body, 0, p.body * STRING.bodyGain);
            }
            applyEnv(out, { attack: 0.001, decay: p.t60, sustain: 0 });
            const c = burst(rand, 'white', STRING.pickMs, STRING.pickAttack, STRING.pickMs);
            bandpass(c, STRING.pickBase + STRING.pickRange * p.brightness, STRING.pickQ);
            addInto(out, c, 0, STRING.pickGain * (STRING.pickBright + p.brightness));
            return addAir(finishMono(out, note, STRING.trim), note, p.air, p.t60, rand);
        }
    };

    // Shared envelope option bag for the gated models (analog, chip).
    const gateEnv = p => ({
        attack: p.attack, hold: p.hold, decay: p.decay, sustain: p.sustain, release: p.release,
        gate: p.sustain > 0 ? p.attack + p.hold : undefined
    });
    const gatedDuration = p => p.sustain > 0 ? p.attack + p.hold + p.release * 1.05 : p.attack + p.hold + p.decay * 1.05;

    MODELS.analog = {
        params: {
            wave: wave(['saw', 'pulse', 'tri'], 'saw'), unison: num(1, 7, 3), detune: num(0, 50, 14), drift: num(0, 20, 3),
            pw: num(0.05, 0.95, 0.5), sub: num(0, 1, 0.35), cutoff: num(0.5, 20, 1.5), env: num(0, 24, 7),
            envDecay: num(0.005, 2, 0.055), q: num(0.5, 8, 1.6), drive: num(0, 4, 1.3), attack: num(0.001, 1, 0.004),
            hold: num(0, 2, 0), decay: num(0.02, 3, 0.42), sustain: num(0, 1, 0), release: num(0.01, 2, 0.15),
            glide: num(-24, 24, 0), glideTime: num(0.005, 1, 0.08), width: num(0, 1, 0.3)
        },
        pitched: () => true,
        minFreq: p => p.sub > 0 ? 260 : 130,
        duration: gatedDuration,
        render(p, note, rand) {
            const n = lengthOf(note), f = offsetFreq(note.freq, note, rand), dur = srcDur(n);
            const glide = p.glide !== 0, start = glide ? f * semis(p.glide) : f;
            const bank = () => {
                const o = { wave: p.wave, freq: start, dur, unison: Math.round(p.unison), detune: p.detune, drift: p.drift, pw: p.pw, glideTime: p.glideTime };
                if (glide) o.glideTo = f;
                return osc(o, rand);
            };
            const A = bank(), B = bank();
            if (p.sub > 0) {
                const o = { wave: 'tri', freq: start / 2, dur, unison: 1, glideTime: p.glideTime };
                if (glide) o.glideTo = f / 2;
                const s = osc(o, rand);
                addInto(A, s, 0, p.sub * ANALOG.subMix); addInto(B, s, 0, p.sub * ANALOG.subMix);
            }
            const cut = t => f * (p.cutoff + p.env * Math.exp(-t / p.envDecay));
            const env = gateEnv(p);
            for (const ch of [A, B]) {
                svf(ch, { type: 'lp', cutoff: cut, q: p.q });
                if (p.drive > 0) saturate(ch, p.drive);
                applyEnv(ch, env);
            }
            const st = zeros(n);
            for (let i = 0; i < n; i++) {
                const mid = (A[i] + B[i]) * 0.5, side = (A[i] - B[i]) * 0.5;
                st.L[i] = mid + p.width * side; st.R[i] = mid - p.width * side;
            }
            return finishStereo(st, note, ANALOG.trim, true);
        }
    };

    MODELS.chip = {
        params: {
            wave: wave(['pulse', 'tri'], 'pulse'), duty: num(0.125, 0.75, 0.5), arp: { list: [-24, 24], max: 4, def: [] },
            arpRate: num(8, 60, 30), vibrato: num(0, 100, 0), vibratoRate: num(2, 10, 6), vibratoDelay: num(0, 0.5, 0.08),
            bits: num(4, 16, 8), attack: num(0.001, 0.02, 0.002), hold: num(0, 1, 0), decay: num(0.02, 2, 0.25),
            sustain: num(0, 1, 0), release: num(0.01, 1, 0.08)
        },
        pitched: () => true,
        minFreq: () => 130,
        duration: gatedDuration,
        render(p, note, rand) {
            const n = lengthOf(note), f0 = offsetFreq(note.freq, note, rand);
            const steps = [0, ...(p.arp || []).slice(0, 4)];
            const out = new Float32Array(n);
            const tri = p.wave === 'tri', duty = p.duty, leak = 1 - TAU * 5 / SR;
            const dc = tri ? 0 : 2 * duty - 1;
            let ph = 0, tv = -1;
            for (let i = 0; i < n; i++) {
                const t = i / SR;
                let c = 0;
                if (p.vibrato > 0 && t > p.vibratoDelay) {
                    c = p.vibrato * Math.sin(TAU * p.vibratoRate * (t - p.vibratoDelay)) * Math.min(1, (t - p.vibratoDelay) / CHIP.vibratoRamp);
                }
                const st = steps[Math.floor(t * p.arpRate) % steps.length];
                const dt = f0 * semis(st) * cents(c) / SR;
                if (tri) {
                    let p2 = ph + 0.5; if (p2 >= 1) p2 -= 1;
                    const sq = (ph < 0.5 ? 1 : -1) + D.polyblep(ph, dt) - D.polyblep(p2, dt);
                    tv = 4 * dt * sq + leak * tv;
                    out[i] = tv;
                } else {
                    let p2 = ph + 1 - duty; if (p2 >= 1) p2 -= 1;
                    out[i] = (ph < duty ? 1 : -1) + D.polyblep(ph, dt) - D.polyblep(p2, dt) - dc;
                }
                ph += dt; if (ph >= 1) ph -= 1;
            }
            const levels = Math.pow(2, Math.round(p.bits) - 1);
            for (let i = 0; i < n; i++) out[i] = Math.round(out[i] * levels * 0.5) / (levels * 0.5);
            svf(out, { type: 'lp', cutoff: CHIP.lowpassHz, q: Math.SQRT1_2 });
            applyEnv(out, gateEnv(p));
            return finishMono(out, note, CHIP.trim);
        }
    };

    MODELS.drop = {
        params: {
            rise: num(0, 2, 0.6), t60: num(0.02, 0.5, 0.08), attack: num(0.0005, 0.01, 0.003), count: num(1, 8, 1),
            spread: num(0, 0.2, 0.04), pitchSpread: num(0, 12, 4), splash: num(0, 1, 0), thick: num(0, 1, 0), width: num(0, 1, 0.4)
        },
        pitched: () => true,
        minFreq: () => 130,
        duration: p => (Math.round(p.count) - 1) * p.spread * 1.3 + p.t60 * 2.5 + 0.02,
        render(p, note, rand) {
            const n = lengthOf(note), st = zeros(n), count = Math.round(p.count);
            const base = offsetFreq(note.freq, note, rand);
            for (let k = 0; k < count; k++) {
                const r1 = rand(), r2 = rand(), r3 = rand();
                const at = k * p.spread * (0.7 + DROP.startJitter * r1);
                const off = Math.round(at * SR);
                if (off >= n) continue;
                const f = k === 0 ? base : base * semis((2 * r2 - 1) * p.pitchSpread);
                const len = Math.min(n - off, Math.ceil(p.t60 * DROP.lengthT60 * SR));
                const b = bubble({ freq: f, dur: srcDur(len), t60: p.t60, rise: p.rise, attack: p.attack });
                addPan(st, b, 1 - DROP.memberFade * k, note.pan + (2 * r3 - 1) * p.width, at);
            }
            if (p.thick > 0) { saturate(st.L, 1 + 3 * p.thick); saturate(st.R, 1 + 3 * p.thick); }
            if (p.splash > 0) {
                const s = burst(rand, 'pink', DROP.splashMs, 0.0005, DROP.splashDecay);
                const k = Math.log(DROP.splashTo / DROP.splashFrom) / DROP.splashMs;
                svf(s, { type: 'bp', cutoff: t => DROP.splashFrom * Math.exp(k * t), q: DROP.splashQ });
                addPan(st, s, p.splash * DROP.splashLevel, note.pan);
            }
            return finishStereo(st, note, DROP.trim, false);
        }
    };

    MODELS.blip = {
        params: {
            glide: num(-24, 24, 0), glideTime: num(0.002, 1, 0.05), boing: num(0, 1, 0), boingRate: num(4, 16, 10),
            harm: num(0, 1, 0.1), attack: num(0.001, 0.05, 0.004), hold: num(0, 1, 0), decay: num(0.02, 2, 0.3)
        },
        pitched: () => true,
        minFreq: () => 130,
        duration: p => p.attack + p.hold + p.decay * 1.05,
        render(p, note, rand) {
            const n = lengthOf(note), f0 = offsetFreq(note.freq, note, rand);
            const out = new Float32Array(n);
            let ph = 0;
            for (let i = 0; i < n; i++) {
                const t = i / SR;
                const st = p.glide * Math.exp(-t / p.glideTime)
                    + p.boing * BLIP.boingDepth * Math.exp(-t / BLIP.boingDecay) * Math.sin(TAU * p.boingRate * t);
                out[i] = Math.sin(ph) + p.harm * (BLIP.harm2 * Math.sin(2 * ph) + BLIP.harm3 * Math.sin(3 * ph));
                ph += TAU * f0 * semis(st) / SR;
                if (ph >= TAU) ph -= TAU;
            }
            applyEnv(out, { attack: p.attack, hold: p.hold, decay: p.decay, sustain: 0 });
            return finishMono(out, note, BLIP.trim);
        }
    };

    MODELS.pad = {
        params: {
            partials: num(1, 8, 4), detune: num(0, 30, 8), attack: num(0.05, 1.5, 0.35), hold: num(0, 2, 0.2),
            release: num(0.05, 2.5, 0.8), cutoff: num(400, 8000, 2500), width: num(0, 1, 0.7), shimmer: num(0, 1, 0)
        },
        pitched: () => true,
        minFreq: () => 130,
        duration: p => p.attack + p.hold + p.release * 1.05,
        render(p, note, rand) {
            const n = lengthOf(note), st = zeros(n), count = Math.round(p.partials), spread = note.detune ?? 0;
            for (let k = 1; k <= count; k++) {
                for (let j = 0; j < 3; j++) {
                    const c = (j - 1) * p.detune;
                    const phase = rand() * TAU, v = rand();
                    const f = note.freq * k * cents(c + (spread > 0 ? (v * 2 - 1) * spread : 0));
                    if (f >= SR * 0.45) continue;
                    // the three copies alternate left / right, flipping from partial to partial
                    const pan = (j - 1) * p.width * (k & 1 ? 1 : -1);
                    const a = (clamp(pan, -1, 1) + 1) * Math.PI / 4, g = 1 / k;
                    const gl = Math.cos(a) * Math.SQRT2 * g, gr = Math.sin(a) * Math.SQRT2 * g;
                    const inc = TAU * f / SR;
                    let ph = phase;
                    for (let i = 0; i < n; i++) {
                        const s = Math.sin(ph);
                        st.L[i] += s * gl; st.R[i] += s * gr;
                        ph += inc; if (ph >= TAU) ph -= TAU;
                    }
                }
            }
            svf(st.L, { type: 'lp', cutoff: p.cutoff, q: Math.SQRT1_2 });
            svf(st.R, { type: 'lp', cutoff: p.cutoff, q: Math.SQRT1_2 });
            if (p.shimmer > 0) {
                const rate = PAD.shimmerRate[0] + (PAD.shimmerRate[1] - PAD.shimmerRate[0]) * rand(), ph0 = rand() * TAU;
                const depth = PAD.shimmerDepth * p.shimmer;
                for (let i = 0; i < n; i++) {
                    const t = i / SR;
                    st.L[i] *= 1 - depth * (0.5 + 0.5 * Math.sin(TAU * rate * t + ph0));
                    st.R[i] *= 1 - depth * (0.5 + 0.5 * Math.sin(TAU * rate * t + ph0 + Math.PI / 2));
                }
            }
            const env = { attack: p.attack, hold: p.hold, sustain: 1, release: p.release, gate: p.attack + p.hold };
            applyEnv(st.L, env); applyEnv(st.R, env);
            return finishStereo(st, note, PAD.trim, true);
        }
    };

    MODELS.metal = {
        params: {
            kind: wave(['plate', 'pipe', 'ping'], 'plate'),
            t60: num(0.05, 4, 0.8), damp: num(0, 2, 0.4), beat: num(0, 6, 1.2), hardness: num(0, 1, 0.5),
            attack: num(0.0005, 0.02, 0.001), width: num(0, 1, 0.6),
            spread: num(0, 12, 3), air: num(0, 0.2, 0.04)
        },
        presets: { kind: METAL.kinds },
        pitched: () => true,
        minFreq: () => 130,
        duration: p => p.t60 * 1.2,
        render(p, note, rand) {
            const n = lengthOf(note), w = p.width;
            const { a, b } = modal({
                freq: note.freq, dur: srcDur(n), t60: p.t60, damp: p.damp, beat: p.beat, attack: p.attack, tilt: 1 - p.hardness,
                spread: p.spread + (note.detune ?? 0), partials: p.r.map((r, i) => ({ r, g: p.g[i] }))
            }, rand);
            const st = zeros(n);
            addPan(st, a, b ? METAL.mainGain : 1, note.pan + METAL.mainPan * w);
            if (b) addPan(st, b, METAL.twinGain, note.pan + METAL.twinPan * w);
            const s = burst(rand, 'white', METAL.strikeMs, METAL.strikeAttack, METAL.strikeMs);
            highpass(s, METAL.strikeHz);
            addPan(st, s, METAL.strikeLevel * (METAL.strikeFloor + (1 - METAL.strikeFloor) * p.hardness), note.pan);
            return addAir(finishStereo(st, note, METAL.trim, false), note, p.air, p.t60, rand);
        }
    };

    MODELS.click = {
        params: {
            material: wave(['felt', 'relay', 'plastic', 'glass', 'copper', 'stone'], 'plastic'),
            tone: num(200, 9000, 1800), ring: num(0.004, 0.2, 0.025), noise: num(0, 1, 0.5), q: num(0.5, 10, 2),
            length: num(0.001, 0.02, 0.004), double: num(0, 0.03, 0), tuned: num(0, 1, 0), attack: num(0.0005, 0.004, 0.0008)
        },
        presets: { material: CLICK.materials },
        pitched: p => p.tuned === 1,
        minFreq: p => p.tuned === 1 ? 130 : null,
        duration: p => p.length + p.ring * 1.2 + p.double + 0.01,
        render(p, note, rand) {
            const n = lengthOf(note), out = new Float32Array(n);
            const tone0 = p.tuned === 1 ? offsetFreq(note.freq, note, rand) : p.tone;
            const one = (tone, level) => {
                const b = burst(rand, 'white', p.length, p.attack, p.length);
                bandpass(b, tone, p.q);
                const dur = Math.max(p.length, p.ring * 1.2 + 0.005);
                const partials = [{ r: 1, g: 1 }];
                if (p.second) partials.push({ r: p.second, g: CLICK.secondGain });
                const res = modal({ freq: tone, dur: srcDur(Math.ceil(dur * SR)), t60: p.ring, damp: 0.6, attack: p.attack, partials }, rand).a;
                return { b, res, level };
            };
            const put = (c, at) => {
                addInto(out, c.b, at, p.noise * CLICK.burstGain * c.level);
                addInto(out, c.res, at, (1 - CLICK.ringShare * p.noise) * c.level);
            };
            put(one(tone0, 1), 0);
            if (p.double > 0) put(one(tone0 * CLICK.doubleTone, CLICK.doubleLevel), p.double);
            return finishMono(out, note, CLICK.trim);
        }
    };

    MODELS.thud = {
        params: {
            drop: num(0, 24, 7), dropTime: num(0.005, 0.2, 0.03), t60: num(0.03, 1, 0.18), noise: num(0, 1, 0.4),
            noiseCut: num(200, 4000, 900), attack: num(0.0005, 0.01, 0.001)
        },
        pitched: () => true,
        minFreq: () => 130,
        duration: p => p.t60 * 1.2,
        render(p, note, rand) {
            const n = lengthOf(note), f0 = offsetFreq(note.freq, note, rand);
            const out = sineLoop(n, t => f0 * semis(p.drop * Math.exp(-t / p.dropTime)));
            applyEnv(out, { attack: p.attack, decay: p.t60, sustain: 0 });
            if (p.noise > 0) {
                const b = burst(rand, 'pink', THUD.noiseMs, THUD.noiseAttack, THUD.noiseDecay);
                lowpass(b, p.noiseCut); highpass(b, THUD.noiseHighpass);
                addInto(out, b, 0, p.noise * THUD.noiseLevel);
            }
            return finishMono(out, note, THUD.trim);
        }
    };

    MODELS.noise = {
        params: {
            color: wave(['white', 'pink', 'brown'], 'pink'), band: { pair: [120, 18000], def: [300, 6000] },
            sweep: num(-4, 4, 0), q: num(0.3, 6, 0.7), attack: num(0.001, 1.5, 0.06), hold: num(0, 2, 0),
            decay: num(0.01, 3, 0.2), flutter: num(0, 1, 0), width: num(0, 1, 0.8)
        },
        pitched: () => false,
        minFreq: () => null,
        duration: p => p.attack + p.hold + p.decay * 1.05,
        render(p, note, rand) {
            const n = lengthOf(note), dur = srcDur(n);
            const A = noise(dur, p.color, rand), B = noise(dur, p.color, rand);
            const [lo, hi] = p.band;
            const fHp = t => lo * Math.pow(2, p.sweep * t / dur), fLp = t => hi * Math.pow(2, p.sweep * t / dur);
            const env = { attack: p.attack, hold: p.hold, decay: p.decay, sustain: 0 };
            for (const ch of [A, B]) {
                svf(ch, { type: 'hp', cutoff: fHp, q: p.q });
                svf(ch, { type: 'lp', cutoff: fLp, q: p.q });
                if (p.flutter > 0) for (let i = 0; i < n; i++) ch[i] *= 1 - 0.5 * p.flutter * (1 + Math.sin(TAU * NOISE.flutterHz * i / SR));
                applyEnv(ch, env);
            }
            const st = zeros(n);
            for (let i = 0; i < n; i++) { st.L[i] = A[i]; st.R[i] = (1 - p.width) * A[i] + p.width * B[i]; }
            return finishStereo(st, note, NOISE.trim, true);
        }
    };

    MODELS.squelch = {
        params: {
            from: num(100, 4000, 300), to: num(100, 4000, 1200), sweepTime: num(0.01, 0.5, 0.08), q: num(1, 12, 6),
            saw: num(0, 1, 0.5), noise: num(0, 1, 0.5), drive: num(0, 4, 1), attack: num(0.001, 0.03, 0.003), decay: num(0.02, 1, 0.15)
        },
        pitched: () => true,
        minFreq: () => 80,
        duration: p => p.attack + p.decay * 1.05,
        render(p, note, rand) {
            const n = lengthOf(note), dur = srcDur(n), f0 = offsetFreq(note.freq, note, rand);
            const s = osc({ wave: 'saw', freq: f0, dur, unison: 1 }, rand);
            const w = noise(dur, 'white', rand);
            for (let i = 0; i < n; i++) s[i] = s[i] * p.saw + w[i] * p.noise;
            svf(s, { type: 'bp', cutoff: t => p.to + (p.from - p.to) * Math.exp(-t / p.sweepTime), q: p.q });
            if (p.drive > 0) saturate(s, p.drive);
            applyEnv(s, { attack: p.attack, decay: p.decay, sustain: 0 });
            return finishMono(s, note, SQUELCH.trim);
        }
    };

    MODELS.debris = {
        params: {
            grain: wave(['wood', 'stone', 'earth'], 'stone'), count: num(2, 40, 12), spread: num(0.005, 0.4, 0.12),
            decay: num(0.05, 1, 0.25), tone: num(0.5, 2, 1), size: num(0, 1, 0.5), width: num(0, 1, 0.6)
        },
        pitched: () => false,
        minFreq: () => null,
        duration: p => Math.min(0.7, p.spread + p.decay + 0.05),
        render(p, note, rand) {
            const n = lengthOf(note), st = zeros(n), count = Math.round(p.count);
            const sizeF = 1 - 0.4 * p.size, sizeG = 0.7 + 0.6 * p.size;
            for (let g = 0; g < count; g++) {
                const r1 = rand(), r2 = rand(), r3 = rand(), r4 = rand();
                const onset = g === 0 ? 0 : p.spread * (-Math.log(1 - 0.95 * r1)) / 3;
                const level = Math.pow(10, -3 * onset / p.decay) * (0.5 + 0.5 * r2) * sizeG;
                const pan = note.pan + (2 * r3 - 1) * p.width;
                let buf;
                if (p.grain === 'wood') {
                    const W = DEBRIS.wood, f = (W.lo + W.range * r4) * p.tone * sizeF;
                    buf = modal({ freq: f, dur: 0.12, t60: W.t60, damp: 0, attack: DEBRIS.attack, partials: [{ r: 1, g: 1 }] }, rand).a;
                    const b = bandpass(burst(rand, 'white', W.burstMs, DEBRIS.attack, W.burstMs), f, W.burstQ);
                    addInto(buf, b, 0, W.burstGain);
                    for (let i = 0; i < buf.length; i++) buf[i] *= W.level;
                } else if (p.grain === 'earth') {
                    const E = DEBRIS.earth, f = (E.lo + E.range * r4) * p.tone * sizeF;
                    buf = burst(rand, 'pink', E.ms, DEBRIS.attack, E.ms);
                    lowpass(buf, f); highpass(buf, E.highpass);
                    for (let i = 0; i < buf.length; i++) buf[i] *= E.level;
                } else {
                    const S = DEBRIS.stone, f = (S.lo + S.range * r4) * p.tone * sizeF;
                    buf = bandpass(burst(rand, 'white', S.burstMs, DEBRIS.attack, S.burstMs), f, S.q);
                    const r = modal({ freq: f, dur: 0.05, t60: S.ringT60, damp: 0, attack: DEBRIS.attack, partials: [{ r: 1, g: 1 }] }, rand).a;
                    const full = new Float32Array(Math.max(buf.length, r.length));
                    full.set(buf); addInto(full, r, 0, S.ringGain);
                    buf = full;
                    for (let i = 0; i < buf.length; i++) buf[i] *= S.level;
                }
                addPan(st, buf, level, pan, onset);
            }
            return finishStereo(st, note, DEBRIS.trim, false);
        }
    };

    // ---- public API ----------------------------------------------------------------------------
    function resolveParams(model, params) {
        const m = MODELS[model];
        if (!m) throw new Error('unknown instrument model ' + model);
        const given = params || {};
        const out = {};
        for (const [k, s] of Object.entries(m.params)) {
            if (Array.isArray(s)) out[k] = s[2];
            else if (s.values) out[k] = s.def;
            else out[k] = s.def.slice();
        }
        for (const [k, table] of Object.entries(m.presets || {})) Object.assign(out, table[given[k] ?? out[k]]);
        for (const [k, v] of Object.entries(given)) if (k in m.params) out[k] = Array.isArray(v) ? v.slice() : v;
        return out;
    }
    function render(model, params, note, rand) {
        const m = MODELS[model];
        if (!m) throw new Error('unknown instrument model ' + model);
        return m.render(resolveParams(model, params), note, rand);
    }

    const api = { MODELS, resolveParams, render };
    root.DesktopSoundInstruments = api;
    if (typeof module === 'object' && module && module.exports) module.exports = api;
})(typeof self !== 'undefined' ? self : globalThis);
