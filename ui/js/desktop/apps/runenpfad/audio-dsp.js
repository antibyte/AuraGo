// Runenpfad offline DSP core: deterministic PRNG, bandlimited PeriodicWave harmonics,
// Karplus-Strong / modal physical-model AudioBuffers, waveshaper curves, and stereo biome IRs.
// Pure Web Audio math only (zero DOM, zero ambient PRNG, zero wall-clock) so Node vm tests load it directly.
(function (root) {
    'use strict';

    const TWO_PI = Math.PI * 2;
    const banks = typeof WeakMap === 'function' ? new WeakMap() : null;

    function rng(seed) {
        let a = (seed >>> 0) || 1;
        return function () {
            a = (a + 0x6D2B79F5) >>> 0;
            let t = Math.imul(a ^ (a >>> 15), 1 | a);
            t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
            return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
        };
    }

    function softClip(x) {
        return Math.tanh(x);
    }

    function biquadCoeffs(type, fc, q, sr) {
        const w0 = TWO_PI * Math.min(0.45 * sr, Math.max(20, fc)) / sr;
        const cosW = Math.cos(w0);
        const alpha = Math.sin(w0) / (2 * Math.max(0.1, q));
        const a0 = 1 + alpha;
        let b0 = 0, b1 = 0, b2 = 0;
        if (type === 'lowpass') {
            b1 = 1 - cosW; b0 = b1 * 0.5; b2 = b0;
        } else if (type === 'bandpass') {
            b0 = alpha; b1 = 0; b2 = -alpha;
        } else if (type === 'highpass') {
            b1 = -(1 + cosW); b0 = (1 + cosW) * 0.5; b2 = b0;
        } else {
            b0 = 1;
        }
        return {
            b0: b0 / a0,
            b1: b1 / a0,
            b2: b2 / a0,
            a1: (-2 * cosW) / a0,
            a2: (1 - alpha) / a0
        };
    }

    function applyBiquad(buf, type, fc, q, sr, mix) {
        const c = biquadCoeffs(type, fc, q, sr);
        const m = mix === undefined ? 1 : mix;
        let x1 = 0, x2 = 0, y1 = 0, y2 = 0;
        for (let i = 0; i < buf.length; i++) {
            const x = buf[i];
            const y = c.b0 * x + c.b1 * x1 + c.b2 * x2 - c.a1 * y1 - c.a2 * y2;
            x2 = x1; x1 = x; y2 = y1; y1 = y;
            buf[i] = x * (1 - m) + y * m;
        }
    }

    function normalizePeak(arr, ceiling) {
        let peak = 0;
        for (let i = 0; i < arr.length; i++) {
            const a = Math.abs(arr[i]);
            if (a > peak) peak = a;
        }
        if (peak > 1e-6) {
            const g = (ceiling || 0.92) / peak;
            for (let i = 0; i < arr.length; i++) arr[i] *= g;
        }
        return arr;
    }

    // Write generated mono PCM frames into an AudioBuffer while respecting FakeContext (fixed short length).
    function fillBuffer(ac, targetFrames, sr, synthFn) {
        const frames = Math.max(64, Math.round(targetFrames));
        const buf = ac.createBuffer(1, frames, sr);
        const out = buf.getChannelData(0);
        const n = out.length;
        if (n < 256) {
            const r = rng(frames);
            for (let i = 0; i < n; i++) out[i] = (r() * 2 - 1) * 0.1 * Math.exp(-3 * i / n);
            return buf;
        }
        const tmp = new Float32Array(n);
        synthFn(tmp, sr);
        for (let i = 0; i < n; i++) out[i] = tmp[i];
        return buf;
    }

    // ---- Physical Modeling String & Membrane Generators ----

    function karplusString(out, sr, opts) {
        const f0 = opts.refHz || 261.6256;
        const exactPeriod = sr / f0;
        const delayLen = Math.max(4, Math.floor(exactPeriod - 0.5));
        const frac = Math.max(0.02, Math.min(0.98, exactPeriod - 0.5 - delayLen));
        const apC = (1 - frac) / (1 + frac);
        const line = new Float32Array(delayLen);
        const r = rng(opts.seed || 101);
        const pick = Math.max(0.05, Math.min(0.45, opts.pick || 0.16));
        const combLag = Math.max(1, Math.round(pick * delayLen));
        const raw = new Float32Array(delayLen);
        let lp = 0;
        for (let i = 0; i < delayLen; i++) {
            lp = lp * (opts.excSmooth || 0.35) + (r() * 2 - 1) * (1 - (opts.excSmooth || 0.35));
            raw[i] = lp;
        }
        for (let i = 0; i < delayLen; i++) {
            line[i] = raw[i] - raw[(i - combLag + delayLen) % delayLen] * (opts.combAmt !== undefined ? opts.combAmt : 0.72);
        }
        const n = out.length;
        const rho = opts.rho || 0.997;
        const bright = opts.bright !== undefined ? opts.bright : 0.52;
        const settleSamples = opts.slideMs ? Math.round(opts.slideMs * 0.001 * sr) : 0;
        const bodyRes = opts.bodyHz || 190;
        let idx = 0, prev = 0, apIn = 0, apOut = 0, bState = 0;
        const bAlpha = Math.exp(-TWO_PI * bodyRes / sr);
        for (let i = 0; i < n; i++) {
            let effLine = line[idx];
            if (settleSamples > 0 && i < settleSamples) {
                const k = 1 - i / settleSamples;
                effLine += Math.sin(TWO_PI * f0 * (1 - 0.045 * k) * (i / sr)) * 0.14 * k;
            }
            const nextIdx = (idx + 1) % delayLen;
            const damped = rho * (bright * effLine + (1 - bright) * prev);
            prev = line[idx];
            const y = apC * (damped - apOut) + apIn;
            apIn = damped;
            apOut = y;
            line[idx] = y;
            idx = nextIdx;
            bState = bState * bAlpha + y * (1 - bAlpha);
            let s = y + bState * (opts.bodyGain || 0.45);
            if (i < Math.round(0.004 * sr)) {
                s += (r() * 2 - 1) * (opts.pluckClick || 0.12) * (1 - i / (0.004 * sr));
            }
            out[i] = s;
        }
        if (opts.eqHz) applyBiquad(out, 'bandpass', opts.eqHz, opts.eqQ || 1.2, sr, opts.eqMix || 0.25);
        const fade = Math.min(Math.round(0.05 * sr), n >> 2);
        for (let i = 0; i < fade; i++) out[n - 1 - i] *= i / fade;
        normalizePeak(out, opts.peak || 0.88);
    }

    function modalBank(out, sr, opts) {
        const baseHz = opts.refHz || 440;
        const modes = opts.modes;
        const n = out.length;
        const r = rng(opts.seed || 777);
        const clickLen = Math.round((opts.clickMs || 5) * 0.001 * sr);
        for (let m = 0; m < modes.length; m++) {
            const ratio = modes[m][0];
            const amp = modes[m][1];
            const tau = modes[m][2];
            const drift = modes[m][3] || 0;
            const freq = baseHz * ratio;
            if (freq >= sr * 0.44) continue;
            const w = TWO_PI * freq / sr;
            const decay = Math.exp(-1 / (tau * sr));
            const phase = (r() * 0.2) * TWO_PI;
            let env = amp;
            for (let i = 0; i < n; i++) {
                const ph = i * w + (drift ? drift * Math.sin(TWO_PI * 3.1 * i / sr) : 0) + phase;
                out[i] += Math.sin(ph) * env;
                env *= decay;
                if (env < 1e-6) break;
            }
        }
        if (opts.clickGain) {
            for (let i = 0; i < Math.min(clickLen, n); i++) {
                const env = (1 - i / clickLen);
                out[i] += (r() * 2 - 1) * opts.clickGain * env * env;
            }
        }
        if (opts.doubleBounce && n > Math.round(0.03 * sr)) {
            const delayS = Math.round(0.022 * sr);
            for (let i = n - 1; i >= delayS; i--) out[i] += out[i - delayS] * 0.38;
        }
        if (opts.sat) {
            for (let i = 0; i < n; i++) out[i] = softClip(out[i] * opts.sat);
        }
        const att = Math.max(1, Math.round((opts.attackMs || 1.2) * 0.001 * sr));
        for (let i = 0; i < Math.min(att, n); i++) out[i] *= i / att;
        const tail = Math.min(Math.round(0.04 * sr), n >> 2);
        for (let i = 0; i < tail; i++) out[n - 1 - i] *= i / tail;
        normalizePeak(out, opts.peak || 0.88);
    }

    function synthMembrane(out, sr, opts) {
        const n = out.length;
        const r = rng(opts.seed || 404);
        let phase = 0;
        let lp = 0, hpPrev = 0, hpState = 0;
        const bodyDecay = opts.bodyTau || 0.14;
        const snapDecay = opts.snapTau || 0.028;
        const noiseTau = opts.noiseTau || 0.045;
        for (let i = 0; i < n; i++) {
            const t = i / sr;
            const fInst = opts.fEnd + (opts.fStart - opts.fEnd) * Math.exp(-t / (opts.pitchTau || 0.026));
            phase += TWO_PI * fInst / sr;
            const body = Math.sin(phase) * Math.exp(-t / bodyDecay)
                + (opts.overAmp || 0) * Math.sin(phase * (opts.overRatio || 1.62)) * Math.exp(-t / snapDecay);
            const raw = r() * 2 - 1;
            lp = lp * (opts.noiseLp || 0.45) + raw * (1 - (opts.noiseLp || 0.45));
            hpState = 0.82 * (hpState + lp - hpPrev);
            hpPrev = lp;
            const noisePart = (opts.noiseHp ? hpState : lp) * (opts.noiseGain || 0) * Math.exp(-t / noiseTau);
            const transient = t < (opts.clickDur || 0.004) ? Math.sin(TWO_PI * (opts.clickHz || 1400) * t) * (1 - t / (opts.clickDur || 0.004)) * (opts.clickGain || 0.25) : 0;
            out[i] = softClip((body + noisePart + transient) * (opts.drive || 1.35));
        }
        const fade = Math.min(Math.round(0.025 * sr), n >> 2);
        for (let i = 0; i < fade; i++) out[n - 1 - i] *= i / fade;
        normalizePeak(out, opts.peak || 0.9);
    }

    function synthMetallicPerc(out, sr, opts) {
        const n = out.length;
        const r = rng(opts.seed || 909);
        const freqs = opts.freqs;
        const phases = new Float32Array(freqs.length);
        let hp1 = 0, lp1 = 0;
        for (let i = 0; i < n; i++) {
            const t = i / sr;
            let oscSum = 0;
            for (let k = 0; k < freqs.length; k++) {
                phases[k] += TWO_PI * freqs[k] / sr;
                oscSum += ((phases[k] % TWO_PI) < Math.PI ? 1 : -1) * 0.16;
            }
            const raw = r() * 2 - 1;
            const nSig = opts.oscMix * oscSum + (1 - opts.oscMix) * raw;
            hp1 = nSig - lp1;
            lp1 = lp1 * opts.hpCoef + nSig * (1 - opts.hpCoef);
            let s = hp1 * Math.exp(-t / opts.tau);
            if (opts.rimHz && t < 0.028) {
                s += Math.sin(TWO_PI * opts.rimHz * t) * Math.exp(-t / 0.006) * 0.55;
            }
            out[i] = s;
        }
        const fade = Math.min(Math.round(0.018 * sr), n >> 2);
        for (let i = 0; i < fade; i++) out[n - 1 - i] *= i / fade;
        normalizePeak(out, opts.peak || 0.85);
    }

    // Nominal pitch metadata so callers compute accurate playbackRate = freq / refHz.
    const SAMPLE_SPECS = {
        harp: { dur: 2.1, refHz: 261.6256, make: (o, sr) => karplusString(o, sr, { refHz: 261.6256, rho: 0.9983, pick: 0.24, bright: 0.72, bodyHz: 185, bodyGain: 0.40, pluckClick: 0.14, seed: 11 }) },
        lute: { dur: 1.45, refHz: 261.6256, make: (o, sr) => karplusString(o, sr, { refHz: 261.6256, rho: 0.9968, pick: 0.18, bright: 0.65, bodyHz: 240, bodyGain: 0.42, eqHz: 620, eqMix: 0.2, pluckClick: 0.18, seed: 22 }) },
        oud: { dur: 1.55, refHz: 261.6256, make: (o, sr) => karplusString(o, sr, { refHz: 261.6256, rho: 0.9962, pick: 0.11, combAmt: 0.84, bright: 0.72, slideMs: 22, bodyHz: 155, bodyGain: 0.50, eqHz: 510, eqQ: 1.8, eqMix: 0.34, pluckClick: 0.20, seed: 33 }) },
        pizz: { dur: 0.62, refHz: 196.0, make: (o, sr) => karplusString(o, sr, { refHz: 196.0, rho: 0.9922, pick: 0.31, excSmooth: 0.55, bright: 0.52, bodyHz: 112, bodyGain: 0.52, pluckClick: 0.18, seed: 44 }) },
        marimba: { dur: 0.82, refHz: 261.6256, make: (o, sr) => modalBank(o, sr, { refHz: 261.6256, attackMs: 2.5, clickGain: 0.18, clickMs: 4, modes: [[1, 1.0, 0.22], [4.0, 0.28, 0.055], [9.95, 0.06, 0.016]], seed: 55 }) },
        celesta: { dur: 1.55, refHz: 523.251, make: (o, sr) => modalBank(o, sr, { refHz: 523.251, attackMs: 1.4, clickGain: 0.08, clickMs: 2.5, modes: [[1, 1.0, 0.56], [2.756, 0.22, 0.22], [5.404, 0.08, 0.09], [2.0, 0.16, 0.38]], seed: 66 }) },
        darkbell: { dur: 2.35, refHz: 220.0, make: (o, sr) => modalBank(o, sr, { refHz: 220.0, attackMs: 2.0, clickGain: 0.22, clickMs: 5, sat: 1.15, modes: [[0.5, 0.55, 1.1], [1.0, 1.0, 0.85], [1.188, 0.62, 0.52, 0.008], [1.503, 0.42, 0.45], [2.0, 0.48, 0.36], [2.514, 0.24, 0.22], [3.01, 0.16, 0.14]], seed: 77 }) },
        glass: { dur: 1.95, refHz: 523.251, make: (o, sr) => modalBank(o, sr, { refHz: 523.251, attackMs: 6.5, modes: [[1.0, 1.0, 0.72, 0.012], [2.32, 0.36, 0.34], [3.89, 0.16, 0.16], [5.76, 0.07, 0.08]], seed: 88 }) },
        anvil: { dur: 1.05, refHz: 440.0, make: (o, sr) => modalBank(o, sr, { refHz: 440.0, attackMs: 0.6, clickGain: 0.45, clickMs: 4, sat: 1.45, modes: [[1.0, 0.9, 0.35], [1.48, 0.85, 0.28], [2.13, 0.72, 0.2], [2.91, 0.5, 0.14], [3.84, 0.35, 0.09], [5.12, 0.24, 0.06]], seed: 99 }) },
        coin: { dur: 0.56, refHz: 1318.51, make: (o, sr) => modalBank(o, sr, { refHz: 1318.51, attackMs: 0.4, clickGain: 0.14, clickMs: 2, doubleBounce: true, modes: [[1.0, 1.0, 0.18], [1.63, 0.58, 0.12], [2.41, 0.38, 0.08], [3.29, 0.2, 0.05]], seed: 109 }) },
        sword_ring: { dur: 0.48, refHz: 440.0, make: (o, sr) => modalBank(o, sr, { refHz: 440.0, attackMs: 1.2, clickGain: 0.34, clickMs: 14, sat: 1.3, modes: [[2.0, 0.55, 0.16], [3.21, 0.7, 0.14], [5.09, 0.6, 0.11], [8.23, 0.35, 0.08], [11.77, 0.2, 0.05]], seed: 121 }) },
        shield_clang: { dur: 0.55, refHz: 260.0, make: (o, sr) => modalBank(o, sr, { refHz: 260.0, attackMs: 0.6, clickGain: 0.55, clickMs: 9, sat: 1.6, modes: [[0.44, 0.75, 0.08], [1.0, 1.0, 0.18], [1.6, 0.85, 0.15], [2.58, 0.65, 0.12], [4.17, 0.45, 0.08], [6.5, 0.3, 0.05]], seed: 131 }) },
        kick: { dur: 0.34, refHz: 60.0, make: (o, sr) => synthMembrane(o, sr, { fStart: 148, fEnd: 43, pitchTau: 0.024, bodyTau: 0.105, clickHz: 1850, clickGain: 0.28, noiseGain: 0.08, noiseTau: 0.015, drive: 1.45, seed: 201 }) },
        snare: { dur: 0.24, refHz: 200.0, make: (o, sr) => synthMembrane(o, sr, { fStart: 255, fEnd: 175, pitchTau: 0.016, bodyTau: 0.055, overAmp: 0.42, overRatio: 1.72, snapTau: 0.032, noiseGain: 0.82, noiseHp: true, noiseLp: 0.15, noiseTau: 0.068, clickHz: 2400, clickGain: 0.24, drive: 1.25, seed: 202 }) },
        hat: { dur: 0.14, refHz: 8000.0, make: (o, sr) => synthMetallicPerc(o, sr, { freqs: [318, 463, 681, 892, 1143, 1520], oscMix: 0.58, hpCoef: 0.86, tau: 0.028, seed: 203 }) },
        taiko: { dur: 0.85, refHz: 55.0, make: (o, sr) => synthMembrane(o, sr, { fStart: 118, fEnd: 42, pitchTau: 0.042, bodyTau: 0.19, overAmp: 0.28, overRatio: 1.56, snapTau: 0.075, noiseGain: 0.52, noiseLp: 0.35, noiseTau: 0.055, clickHz: 1450, clickGain: 0.42, drive: 1.5, seed: 204 }) },
        framedrum: { dur: 0.42, refHz: 75.0, make: (o, sr) => synthMembrane(o, sr, { fStart: 136, fEnd: 58, pitchTau: 0.032, bodyTau: 0.145, overAmp: 0.28, overRatio: 1.58, snapTau: 0.06, noiseGain: 0.24, noiseLp: 0.55, noiseTau: 0.04, clickHz: 650, clickGain: 0.18, drive: 1.25, seed: 205 }) },
        darbuka_doum: { dur: 0.38, refHz: 80.0, make: (o, sr) => synthMembrane(o, sr, { fStart: 122, fEnd: 64, pitchTau: 0.028, bodyTau: 0.13, overAmp: 0.34, overRatio: 2.02, snapTau: 0.065, noiseGain: 0.16, noiseLp: 0.48, noiseTau: 0.028, clickHz: 950, clickGain: 0.18, drive: 1.3, seed: 206 }) },
        darbuka_tek: { dur: 0.14, refHz: 440.0, make: (o, sr) => synthMembrane(o, sr, { fStart: 620, fEnd: 340, pitchTau: 0.01, bodyTau: 0.026, overAmp: 0.6, overRatio: 2.35, snapTau: 0.018, noiseGain: 0.48, noiseHp: true, noiseTau: 0.022, clickHz: 2900, clickGain: 0.32, drive: 1.2, seed: 207 }) },
        tom: { dur: 0.42, refHz: 95.0, make: (o, sr) => synthMembrane(o, sr, { fStart: 158, fEnd: 72, pitchTau: 0.03, bodyTau: 0.135, overAmp: 0.22, overRatio: 1.54, snapTau: 0.05, noiseGain: 0.18, noiseLp: 0.5, noiseTau: 0.025, clickHz: 1100, clickGain: 0.16, drive: 1.2, seed: 208 }) },
        tamb: { dur: 0.24, refHz: 6000.0, make: (o, sr) => synthMetallicPerc(o, sr, { freqs: [3420, 4710, 6190, 8110, 9640], oscMix: 0.48, hpCoef: 0.78, tau: 0.048, rimHz: 460, seed: 209 }) }
    };

    // Bandlimited additive PeriodicWave spectra (harmonics index 1..N with smooth HF rolloff).
    const WAVE_SPECTRA = {
        strings: [1.0, 0.72, 0.58, 0.42, 0.34, 0.25, 0.19, 0.14, 0.10, 0.07, 0.05, 0.035, 0.02, 0.012],
        cello: [1.0, 0.78, 0.62, 0.42, 0.32, 0.22, 0.15, 0.10, 0.06, 0.04, 0.02],
        horn: [1.0, 0.80, 0.60, 0.44, 0.28, 0.16, 0.08, 0.035],
        brass: [0.85, 1.0, 0.88, 0.72, 0.54, 0.40, 0.28, 0.19, 0.12, 0.07, 0.04],
        flute: [1.0, 0.24, 0.08, 0.025, 0.008],
        ney: [1.0, 0.28, 0.42, 0.14, 0.20, 0.07, 0.06, 0.02],
        organ: [1.0, 0.72, 0.38, 0.54, 0.20, 0.26, 0.08, 0.12],
        choir: [1.0, 0.56, 0.44, 0.22, 0.11, 0.05, 0.02],
        bass: [1.0, 0.64, 0.30, 0.13, 0.055, 0.02],
        reed: [1.0, 0.48, 0.78, 0.34, 0.48, 0.20, 0.25, 0.11, 0.10, 0.04]
    };

    function createBank(ac) {
        const sr = ac.sampleRate || 44100;
        const makeNoise = (lenSec, kind, seed) => {
            const frames = Math.max(64, Math.round(lenSec * sr));
            const buf = ac.createBuffer(1, frames, sr);
            const d = buf.getChannelData(0);
            const r = rng(seed);
            let b0 = 0, b1 = 0, b2 = 0, b3 = 0, b4 = 0, b5 = 0, b6 = 0, br = 0;
            for (let i = 0; i < d.length; i++) {
                const w = r() * 2 - 1;
                if (kind === 'white') {
                    d[i] = w;
                } else if (kind === 'pink') {
                    b0 = 0.99886 * b0 + w * 0.0555179;
                    b1 = 0.99332 * b1 + w * 0.0750759;
                    b2 = 0.96900 * b2 + w * 0.1538520;
                    b3 = 0.86650 * b3 + w * 0.3104856;
                    b4 = 0.55000 * b4 + w * 0.5329522;
                    b5 = -0.7616 * b5 - w * 0.0168980;
                    d[i] = (b0 + b1 + b2 + b3 + b4 + b5 + b6 + w * 0.5362) * 0.14;
                    b6 = w * 0.115926;
                } else if (kind === 'brown') {
                    br = (br + 0.03 * w) * 0.985;
                    d[i] = Math.max(-1, Math.min(1, br * 3.4));
                } else {
                    // crackle
                    d[i] = r() < 0.015 ? (r() * 2 - 1) * 0.95 : w * 0.06;
                }
            }
            return buf;
        };

        const makeCurve = (fn) => {
            const c = new Float32Array(513);
            for (let i = 0; i < 513; i++) c[i] = fn((i / 256) - 1);
            return c;
        };

        return {
            sr,
            noises: {
                white: makeNoise(2.0, 'white', 0x51a9c3),
                pink: makeNoise(2.4, 'pink', 0x71b3e7),
                brown: makeNoise(2.4, 'brown', 0x91c715),
                crackle: makeNoise(2.0, 'crackle', 0x33e19b)
            },
            curves: {
                warm: makeCurve((x) => Math.tanh(x * 1.45 + 0.08 * x * x)),
                tape: makeCurve((x) => Math.tanh(x * 1.85) / Math.tanh(1.85)),
                drive: makeCurve((x) => Math.tanh(x * 3.0)),
                fuzz: makeCurve((x) => Math.tanh(x * 4.2 + 0.32 * x * x))
            },
            waves: Object.create(null),
            samples: Object.create(null),
            getWave(kind) {
                if (this.waves[kind]) return this.waves[kind];
                if (typeof ac.createPeriodicWave !== 'function') return null;
                const spec = WAVE_SPECTRA[kind];
                if (!spec) return null;
                const n = spec.length + 1;
                const real = new Float32Array(n);
                const imag = new Float32Array(n);
                for (let k = 0; k < spec.length; k++) {
                    // Lanczos sigma factor avoids Gibbs ringing on sharp harmonic stacks.
                    const ang = Math.PI * (k + 1) / (spec.length + 1);
                    const sigma = ang > 0 ? Math.sin(ang) / ang : 1;
                    imag[k + 1] = spec[k] * sigma;
                }
                const pw = ac.createPeriodicWave(real, imag, { disableNormalization: false });
                this.waves[kind] = pw;
                return pw;
            },
            getSample(id) {
                if (this.samples[id]) return this.samples[id];
                const spec = SAMPLE_SPECS[id];
                if (!spec) return null;
                const buf = fillBuffer(ac, spec.dur * sr, sr, (out, rate) => spec.make(out, rate));
                this.samples[id] = { buf, refHz: spec.refHz };
                return this.samples[id];
            }
        };
    }

    function bankFor(ac) {
        if (!ac) return null;
        if (banks) {
            let b = banks.get(ac);
            if (!b) { b = createBank(ac); banks.set(ac, b); }
            return b;
        }
        if (!ac.__rpDspBank) ac.__rpDspBank = createBank(ac);
        return ac.__rpDspBank;
    }

    // Architectural stereo biome impulse response: pre-delay + early L/R reflections + HF-damped diffuse tail.
    const ROOM_PRESETS = {
        woodland: { sec: 1.5, decay: 2.9, preMs: 18, dampHz: 6600, erAmp: 0.42 },
        cavern: { sec: 2.7, decay: 2.1, preMs: 38, dampHz: 2600, erAmp: 0.56 },
        desert: { sec: 1.7, decay: 2.7, preMs: 24, dampHz: 6800, erAmp: 0.36 },
        glacier: { sec: 2.4, decay: 2.3, preMs: 30, dampHz: 7600, erAmp: 0.48 },
        citadel: { sec: 2.9, decay: 2.0, preMs: 42, dampHz: 5400, erAmp: 0.54 },
        interior: { sec: 0.9, decay: 3.4, preMs: 10, dampHz: 4200, erAmp: 0.38 }
    };

    function buildRoomImpulse(ac, biome, spec) {
        const sr = ac.sampleRate || 44100;
        const p = ROOM_PRESETS[biome] || spec || ROOM_PRESETS.woodland;
        const sec = (spec && spec.sec) || p.sec;
        const decay = (spec && spec.decay) || p.decay;
        const frames = Math.max(64, Math.round(sec * sr));
        const buf = ac.createBuffer(2, frames, sr);
        const preSamp = Math.round(((p.preMs || 20) * 0.001) * sr);
        const erTaps = [0.006, 0.013, 0.021, 0.031, 0.044, 0.059, 0.076];

        for (let ch = 0; ch < 2; ch++) {
            const d = buf.getChannelData(ch);
            const n = d.length;
            const r = rng((ch + 1) * 0x9E3779B9 ^ Math.round(sec * 1000));
            if (n < 256) {
                for (let i = 0; i < n; i++) d[i] = (r() * 2 - 1) * Math.pow(1 - i / n, decay);
                continue;
            }
            let lp = 0;
            const fadeLen = Math.max(1, Math.round(0.018 * sr));
            for (let i = preSamp; i < n; i++) {
                const u = (i - preSamp) / (n - preSamp);
                const fade = i - preSamp < fadeLen ? (i - preSamp) / fadeLen : 1;
                const w = r() * 2 - 1;
                // Progressive damping: earlier tail stays brighter, late tail warms naturally.
                const fc = (p.dampHz || 4500) * (1.25 - 0.8 * u);
                const dCoef = Math.exp(-TWO_PI * Math.max(350, fc) / sr);
                lp = lp * dCoef + w * (1 - dCoef);
                d[i] = lp * Math.pow(1 - u, decay) * fade * 0.42;
            }
            for (let k = 0; k < erTaps.length; k++) {
                const tapIdx = preSamp + Math.round((erTaps[k] + ch * 0.0023 * (k % 2 ? 1 : -1)) * sr);
                if (tapIdx > 0 && tapIdx < n) {
                    const sign = (k + ch) % 2 === 0 ? 1 : -1;
                    d[tapIdx] += sign * (p.erAmp || 0.4) * Math.pow(0.76, k);
                }
            }
        }
        return buf;
    }

    root.RunenpfadAudioDsp = {
        rng,
        softClip,
        bankFor,
        getSample(ac, id) {
            const b = bankFor(ac);
            return b ? b.getSample(id) : null;
        },
        getWave(ac, kind) {
            const b = bankFor(ac);
            return b ? b.getWave(kind) : null;
        },
        getNoise(ac, kind) {
            const b = bankFor(ac);
            return b ? (b.noises[kind || 'white'] || b.noises.white) : null;
        },
        getCurve(ac, kind) {
            const b = bankFor(ac);
            return b ? (b.curves[kind || 'warm'] || b.curves.warm) : null;
        },
        buildRoomImpulse,
        SAMPLE_NAMES: Object.keys(SAMPLE_SPECS),
        WAVE_NAMES: Object.keys(WAVE_SPECTRA)
    };
})(typeof window !== 'undefined' ? window : globalThis);
