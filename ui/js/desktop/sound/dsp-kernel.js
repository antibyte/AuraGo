// Sample-level synthesis kernel for desktop UI sounds.
// Pure JavaScript on Float32Arrays, so the same code renders in the browser (Worker or main thread) and in Node
// (offline WAV renders, loudness measurement, tests). Deterministic: all randomness comes from makeRng(seed).
(function (root) {
    'use strict';

    const SR = 48000;
    const TAU = Math.PI * 2;
    const T60 = 6.907755; // ln(1000): exp(-T60 * t / t60) reaches -60 dB at t = t60

    function makeRng(seed) {
        let a = seed >>> 0;
        return function () {
            a = (a + 0x6D2B79F5) >>> 0;
            let t = a;
            t = Math.imul(t ^ (t >>> 15), t | 1);
            t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
            return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
        };
    }
    // FNV-1a, 32 bit, over the parts joined with "|". Code units above 255 contribute both bytes.
    function hashSeed(...parts) {
        const s = parts.join('|');
        let h = 2166136261;
        for (let i = 0; i < s.length; i++) {
            const c = s.charCodeAt(i);
            h = Math.imul(h ^ (c & 255), 16777619);
            if (c > 255) h = Math.imul(h ^ (c >>> 8), 16777619);
        }
        return h >>> 0;
    }
    const midiToHz = n => 440 * Math.pow(2, (n - 69) / 12);
    const dbToGain = db => Math.pow(10, db / 20);
    const gainToDb = g => 20 * Math.log10(Math.max(g, 1e-12));
    const clamp = (v, lo, hi) => Math.min(hi, Math.max(lo, v));

    // ---- stereo canvas -------------------------------------------------------------------------
    function createMix(seconds) {
        const n = Math.ceil(seconds * SR);
        return { L: new Float32Array(n), R: new Float32Array(n), n };
    }
    function addMono(mix, buf, at, gain, pan) {
        const off = Math.max(0, Math.round((at ?? 0) * SR));
        const a = (clamp(pan ?? 0, -1, 1) + 1) * Math.PI / 4;
        const g = gain ?? 1;
        const gl = Math.cos(a) * Math.SQRT2 * g;
        const gr = Math.sin(a) * Math.SQRT2 * g;
        const n = Math.min(buf.length, mix.n - off);
        const L = mix.L, R = mix.R;
        for (let i = 0; i < n; i++) {
            L[off + i] += buf[i] * gl;
            R[off + i] += buf[i] * gr;
        }
    }
    // Adds a stereo buffer {L, R} at an offset. balance -1..1 attenuates the opposite side linearly.
    function addStereo(mix, st, at = 0, gain = 1, balance = 0) {
        const off = Math.max(0, Math.round(at * SR));
        const b = clamp(balance, -1, 1);
        const gl = gain * (b > 0 ? 1 - b : 1);
        const gr = gain * (b < 0 ? 1 + b : 1);
        const n = Math.min(st.L.length, mix.n - off);
        const L = mix.L, R = mix.R;
        for (let i = 0; i < n; i++) {
            L[off + i] += st.L[i] * gl;
            R[off + i] += st.R[i] * gr;
        }
    }
    // Equal-power pan of a mono buffer into a new stereo buffer (same law as addMono).
    function panMono(buf, gain, pan) {
        const a = (clamp(pan ?? 0, -1, 1) + 1) * Math.PI / 4;
        const g = gain ?? 1;
        const gl = Math.cos(a) * Math.SQRT2 * g;
        const gr = Math.sin(a) * Math.SQRT2 * g;
        const L = new Float32Array(buf.length), R = new Float32Array(buf.length);
        for (let i = 0; i < buf.length; i++) { L[i] = buf[i] * gl; R[i] = buf[i] * gr; }
        return { L, R };
    }

    // ---- envelopes -----------------------------------------------------------------------------
    // Raised-cosine attack, optional hold, exponential decay to a sustain level, exponential release
    // starting at `gate`. All times in seconds; decay and release are -60 dB times.
    function applyEnv(buf, o) {
        const a = Math.max(0.0002, o.attack ?? 0.002);
        const h = o.hold ?? 0;
        const s = o.sustain ?? 0;
        const kd = Math.exp(-T60 / (Math.max(0.005, o.decay ?? 0.3) * SR));
        const kr = Math.exp(-T60 / (Math.max(0.005, o.release ?? 0.1) * SR));
        const gate = o.gate ?? Infinity;
        const na = Math.round(a * SR), nh = na + Math.round(h * SR), ng = gate * SR;
        let d = 1, r = 1;
        for (let i = 0; i < buf.length; i++) {
            let e;
            if (i < na) e = 0.5 - 0.5 * Math.cos(Math.PI * i / na);
            else if (i < nh) e = 1;
            else { e = s + (1 - s) * d; d *= kd; }
            if (i > ng) { r *= kr; e *= r; }
            buf[i] *= e;
        }
        return buf;
    }

    // ---- sources -------------------------------------------------------------------------------
    // Modal resonator bank: every partial is an exponentially decaying sine, computed with a two-pole
    // recurrence. Higher partials decay faster (damp). `beat` adds a slightly detuned twin per partial,
    // returned separately so it can sit on the other side of the stereo image. `spread` (cents) detunes
    // every partial and its twin randomly.
    function modal(o, rand) {
        const n = Math.ceil(o.dur * SR);
        const a = new Float32Array(n);
        const b = o.beat ? new Float32Array(n) : null;
        const damp = o.damp ?? 0.6;
        const tilt = o.tilt ?? 0;
        const spread = o.spread ?? 0;
        const detune = () => (spread > 0 && rand) ? Math.pow(2, (rand() * 2 - 1) * spread / 1200) : 1;
        for (const p of o.partials) {
            const f = o.freq * p.r * detune();
            const t60 = Math.max(0.002, (p.t60 ?? o.t60 ?? 0.5) * Math.pow(p.r, -damp));
            const g = (p.g ?? 1) * Math.pow(p.r, -tilt);
            if (f < SR * 0.45) resonate(a, f, t60, g);
            if (b) {
                const fb = (o.freq * p.r + o.beat * (0.6 + 0.4 * p.r)) * detune();
                if (fb < SR * 0.45) resonate(b, fb, t60, g);
            }
        }
        const env = { attack: o.attack ?? 0.002, decay: 60, sustain: 1 };
        applyEnv(a, env);
        if (b) applyEnv(b, env);
        return { a, b };
    }
    function resonate(out, f, t60, g) {
        const w = TAU * f / SR;
        const k = Math.exp(-T60 / (t60 * SR));
        const c1 = 2 * k * Math.cos(w), c2 = -k * k;
        // y[n] = g k^n sin(w n), so y[-1] = -g sin(w) / k and y[-2] = -g sin(2w) / k^2 (y[0] = 0: no click)
        let y1 = -g * Math.sin(w) / k, y2 = -g * Math.sin(2 * w) / (k * k);
        for (let i = 0; i < out.length; i++) {
            const y = c1 * y1 + c2 * y2;
            out[i] += y;
            y2 = y1; y1 = y;
        }
    }

    // FM: one carrier, any number of parallel modulators with their own ratio, peak index (radians) and
    // index decay. `fb` is modulator self-feedback. Phases wrap modulo 2 pi. A modulator whose Carson
    // bandwidth would pass 0.45 SR gets its index scaled down, or is dropped when its frequency alone does.
    function fm(o) {
        const n = Math.ceil(o.dur * SR);
        const out = new Float32Array(n);
        const lim = 0.45 * SR;
        const src = [];
        for (const m of (o.mods || [])) {
            const fmod = o.freq * m.ratio;
            if (fmod >= lim) continue;
            let idx = m.index;
            if (fmod * (idx + 1) > lim) idx = lim / fmod - 1;
            src.push({ m, idx });
        }
        const M = src.length;
        const inc = new Float64Array(M), mph = new Float64Array(M), idx = new Float64Array(M), floor = new Float64Array(M);
        const kd = new Float64Array(M), fbk = new Float64Array(M), prev = new Float64Array(M);
        for (let j = 0; j < M; j++) {
            const m = src[j].m;
            inc[j] = TAU * o.freq * m.ratio / SR;
            idx[j] = src[j].idx;
            floor[j] = Math.min(m.floor ?? 0, idx[j]);
            kd[j] = Math.exp(-T60 / ((m.t60 ?? 0.2) * SR));
            fbk[j] = m.fb ?? 0;
        }
        const cinc = TAU * o.freq * (o.ratio ?? 1) / SR;
        let ph = 0;
        for (let i = 0; i < n; i++) {
            let pm = 0;
            for (let j = 0; j < M; j++) {
                const v = Math.sin(mph[j] + fbk[j] * prev[j]);
                prev[j] = v;
                pm += idx[j] * v;
                mph[j] += inc[j]; if (mph[j] >= TAU) mph[j] -= TAU;
                idx[j] = floor[j] + (idx[j] - floor[j]) * kd[j];
            }
            out[i] = Math.sin(ph + pm);
            ph += cinc; if (ph >= TAU) ph -= TAU;
        }
        return out;
    }

    // Band-limited oscillator bank (PolyBLEP saw / pulse / triangle, sine) with unison detune, slow drift
    // (cents) and an exponential (log-frequency) glide.
    function polyblep(t, dt) {
        if (t < dt) { t /= dt; return t + t - t * t - 1; }
        if (t > 1 - dt) { t = (t - 1) / dt; return t * t + t + t + 1; }
        return 0;
    }
    function osc(o, rand) {
        const n = Math.ceil(o.dur * SR);
        const out = new Float32Array(n);
        const voices = o.unison ?? 1;
        const spread = o.detune ?? 0; // cents, total spread
        const drift = o.drift ?? 0;   // cents
        const pw = o.pw ?? 0.5;
        const wave = o.wave;
        const glide = o.glideTo != null;
        const kg = glide && o.glideTime > 0 ? Math.exp(-4.6 / (o.glideTime * SR)) : 0;
        const norm = 1 / Math.sqrt(voices);
        const leak = 1 - TAU * 5 / SR;
        for (let v = 0; v < voices; v++) {
            const cents = voices === 1 ? 0 : (v / (voices - 1) - 0.5) * spread;
            const lf0 = Math.log(o.freq * Math.pow(2, cents / 1200));
            const lf1 = glide ? Math.log(o.glideTo * Math.pow(2, cents / 1200)) : lf0;
            let ph = rand ? rand() : 0;
            let dRate = 0, dPh = 0;
            if (drift > 0) {
                dRate = TAU * (0.3 + 0.6 * (rand ? rand() : 0.5)) / SR;
                dPh = TAU * (rand ? rand() : 0);
            }
            const moving = glide || drift > 0;
            let lf = lf0, f = Math.exp(lf0), dt = f / SR;
            let tri = 1 - 4 * Math.abs(ph - 0.5);
            for (let i = 0; i < n; i++) {
                if (moving) {
                    if (glide) lf = lf1 + (lf - lf1) * kg;
                    f = Math.exp(drift > 0 ? lf + drift * Math.sin(dPh + dRate * i) * 0.000577623 : lf);
                    dt = f / SR;
                }
                let s;
                switch (wave) {
                    case 'saw': s = 2 * ph - 1 - polyblep(ph, dt); break;
                    case 'pulse': {
                        let p2 = ph + 1 - pw; if (p2 >= 1) p2 -= 1;
                        s = (ph < pw ? 1 : -1) + polyblep(ph, dt) - polyblep(p2, dt);
                        break;
                    }
                    case 'tri': {
                        let p2 = ph + 0.5; if (p2 >= 1) p2 -= 1;
                        const sq = (ph < 0.5 ? 1 : -1) + polyblep(ph, dt) - polyblep(p2, dt);
                        tri = 4 * dt * sq + leak * tri;
                        s = tri;
                        break;
                    }
                    default: s = Math.sin(TAU * ph);
                }
                out[i] += s * norm;
                ph += dt; if (ph >= 1) ph -= 1;
            }
        }
        return out;
    }

    // Karplus-Strong string with a tuned first-order allpass (fractional delay) and a one-pole loss filter.
    function pluck(o, rand) {
        if (!(o.freq > 0) || o.freq > SR / 8) throw new Error('pluck freq out of range');
        const n = Math.ceil(o.dur * SR);
        const out = new Float32Array(n);
        const bright = clamp(o.brightness ?? 0.5, 0.02, 0.98);
        const lp = 1 - bright; // one-pole coefficient: y = (1 - lp) x + lp y
        // phase delay of the one-pole at f, in samples
        const w = TAU * o.freq / SR;
        const lpDelay = Math.atan2(lp * Math.sin(w), 1 - lp * Math.cos(w)) / w;
        const total = SR / o.freq - lpDelay;
        const N = Math.floor(total - 0.5);
        const frac = total - N;
        const apc = (1 - frac) / (1 + frac);
        const line = new Float32Array(N);
        // excitation: low-passed noise, comb-filtered by the pick position, without DC
        const pick = Math.max(1, Math.round(N * (o.pick ?? 0.18)));
        let e = 0;
        const tmp = new Float32Array(N);
        for (let i = 0; i < N; i++) { e += ((rand() * 2 - 1) - e) * (0.25 + 0.7 * bright); tmp[i] = e; }
        let mean = 0;
        for (let i = 0; i < N; i++) { line[i] = tmp[i] - tmp[(i + pick) % N] * 0.9; mean += line[i]; }
        mean /= N;
        for (let i = 0; i < N; i++) line[i] -= mean;
        const g = Math.exp(-T60 / (Math.max(0.01, o.t60 ?? 0.8) * o.freq)); // loss per round trip
        let idx = 0, lpState = 0, apX = 0, apY = 0;
        for (let i = 0; i < n; i++) {
            const x = line[idx];
            lpState = (1 - lp) * x + lp * lpState;
            const y = apc * lpState + apX - apc * apY;
            apX = lpState; apY = y;
            line[idx] = y * g;
            out[i] = x;
            idx++; if (idx >= N) idx = 0;
        }
        return out;
    }

    // Bubble / drop: a sine whose pitch rises while it decays (Minnaert resonance of a shrinking cavity).
    // The rising frequency is capped at 4x the start frequency.
    function bubble(o) {
        const n = Math.ceil(o.dur * SR);
        const out = new Float32Array(n);
        const t60 = o.t60 ?? 0.08;
        const rise = o.rise ?? 0.6;
        const k = Math.exp(-T60 / (t60 * SR));
        const fmax = Math.min(4 * o.freq, SR * 0.4);
        let amp = 1, ph = 0;
        for (let i = 0; i < n; i++) {
            const f = Math.min(o.freq * (1 + rise * (i / SR) / t60), fmax);
            ph += TAU * f / SR;
            if (ph >= TAU) ph -= TAU;
            out[i] = Math.sin(ph) * amp;
            amp *= k;
        }
        return applyEnv(out, { attack: o.attack ?? 0.003, decay: 60, sustain: 1 });
    }

    function noise(seconds, color, rand) {
        const n = Math.ceil(seconds * SR);
        const out = new Float32Array(n);
        if (color === 'pink') {
            let b0 = 0, b1 = 0, b2 = 0, b3 = 0, b4 = 0, b5 = 0, b6 = 0;
            for (let i = 0; i < n; i++) {
                const w = rand() * 2 - 1;
                b0 = 0.99886 * b0 + w * 0.0555179; b1 = 0.99332 * b1 + w * 0.0750759;
                b2 = 0.96900 * b2 + w * 0.1538520; b3 = 0.86650 * b3 + w * 0.3104856;
                b4 = 0.55000 * b4 + w * 0.5329522; b5 = -0.7616 * b5 - w * 0.0168980;
                out[i] = (b0 + b1 + b2 + b3 + b4 + b5 + b6 + w * 0.5362) * 0.11;
                b6 = w * 0.115926;
            }
        } else if (color === 'brown') {
            let b = 0;
            for (let i = 0; i < n; i++) {
                b = (b + 0.02 * (rand() * 2 - 1)) / 1.02;
                out[i] = 3.5 * b;
            }
        } else {
            for (let i = 0; i < n; i++) out[i] = rand() * 2 - 1;
        }
        return out;
    }

    // ---- filters -------------------------------------------------------------------------------
    // Zero-delay-feedback state variable filter. `cutoff` is a number or a function of time (s),
    // evaluated every 16 samples.
    function svf(buf, o) {
        const type = o.type ?? 'lp';
        const mode = type === 'lp' ? 0 : type === 'bp' ? 1 : type === 'hp' ? 2 : -1;
        if (mode < 0) throw new Error('unknown svf type');
        const kq = 1 / Math.max(0.3, o.q ?? 0.707);
        const fn = typeof o.cutoff === 'function' ? o.cutoff : null;
        let g = 0, a1 = 0, a2 = 0, a3 = 0, ic1 = 0, ic2 = 0;
        const set = fc => {
            g = Math.tan(Math.PI * clamp(fc, 20, SR * 0.45) / SR);
            a1 = 1 / (1 + g * (g + kq)); a2 = g * a1; a3 = g * a2;
        };
        set(fn ? fn(0) : o.cutoff);
        for (let i = 0; i < buf.length; i++) {
            if (fn && (i & 15) === 0) set(fn(i / SR));
            const v0 = buf[i];
            const v3 = v0 - ic2;
            const v1 = a1 * ic1 + a2 * v3;
            const v2 = ic2 + a2 * ic1 + a3 * v3;
            ic1 = 2 * v1 - ic1; ic2 = 2 * v2 - ic2;
            buf[i] = mode === 0 ? v2 : mode === 1 ? v1 * kq : v0 - kq * v1 - v2;
        }
        return buf;
    }
    function biquad(buf, b0, b1, b2, a1, a2) {
        let x1 = 0, x2 = 0, y1 = 0, y2 = 0;
        for (let i = 0; i < buf.length; i++) {
            const x = buf[i];
            const y = b0 * x + b1 * x1 + b2 * x2 - a1 * y1 - a2 * y2;
            x2 = x1; x1 = x; y2 = y1; y1 = y;
            buf[i] = y;
        }
        return buf;
    }
    function highpass(buf, fc, q = Math.SQRT1_2) { // RBJ 2nd order
        const w = TAU * clamp(fc, 5, SR * 0.45) / SR, c = Math.cos(w), al = Math.sin(w) / (2 * q), a0 = 1 + al;
        return biquad(buf, (1 + c) / 2 / a0, -(1 + c) / a0, (1 + c) / 2 / a0, -2 * c / a0, (1 - al) / a0);
    }
    function lowpass(buf, fc, q = Math.SQRT1_2) { // RBJ 2nd order
        const w = TAU * clamp(fc, 5, SR * 0.45) / SR, c = Math.cos(w), al = Math.sin(w) / (2 * q), a0 = 1 + al;
        return biquad(buf, (1 - c) / 2 / a0, (1 - c) / a0, (1 - c) / 2 / a0, -2 * c / a0, (1 - al) / a0);
    }
    function shelf(buf, fc, db, high) { // RBJ shelving
        const A = Math.pow(10, db / 40), w = TAU * fc / SR, c = Math.cos(w), s = Math.sin(w);
        const al = s / 2 * Math.SQRT2, q = 2 * Math.sqrt(A) * al, sg = high ? 1 : -1;
        const a0 = (A + 1) - sg * (A - 1) * c + q;
        return biquad(buf,
            A * ((A + 1) + sg * (A - 1) * c + q) / a0,
            -sg * 2 * A * ((A - 1) + sg * (A + 1) * c) / a0,
            A * ((A + 1) + sg * (A - 1) * c - q) / a0,
            sg * 2 * ((A - 1) - sg * (A + 1) * c) / a0,
            ((A + 1) - sg * (A - 1) * c - q) / a0);
    }

    // ---- effects -------------------------------------------------------------------------------
    function nextPrime(n) {
        for (n = Math.max(2, Math.round(n)); ; n++) {
            let prime = true;
            for (let d = 2; d * d <= n; d++) if (n % d === 0) { prime = false; break; }
            if (prime) return n;
        }
    }
    // Feedback delay network reverb: 8 lines (prime lengths), Householder feedback, per-line damping after
    // Jot (every line loses the same amount at the damping frequency), input diffusion.
    const FDN_BASE = [1123, 1291, 1489, 1699, 1931, 2137, 2377, 2609];
    const AP_BASE = [113, 337, 541, 127, 359, 571]; // L chain, then R chain
    function reverb(mix, o) {
        const size = o.size ?? 1;
        const rt = Math.max(0.05, o.rt60 ?? 0.6);
        const wet = o.mix ?? 0.2;
        const pre = Math.round((o.predelay ?? 0) * SR);
        const lowcut = o.lowcut ?? 220;
        const w = TAU * clamp(o.damping ?? 6000, 100, SR * 0.45) / SR;
        const tail = Math.ceil(rt * SR * 1.1) + pre;
        const n = mix.n + tail;
        const out = { L: new Float32Array(n), R: new Float32Array(n), n };
        out.L.set(mix.L); out.R.set(mix.R);
        const N = 8;
        const len = new Int32Array(N), off = new Int32Array(N);
        const gains = new Float64Array(N), damp = new Float64Array(N);
        let total = 0;
        for (let k = 0; k < N; k++) {
            len[k] = nextPrime(Math.max(31, Math.round(FDN_BASE[k] * size)));
            off[k] = total; total += len[k];
            const m = Math.pow(10, -3 * len[k] / (rt * SR));
            gains[k] = m;
            const m2 = m * m, one = 1 - m2, b = 1 - m2 * Math.cos(w);
            damp[k] = one < 1e-9 ? 0 : clamp((b - Math.sqrt(Math.max(0, b * b - one * one))) / one, 0, 0.9999);
        }
        const buf = new Float32Array(total);
        const pos = new Int32Array(N);
        const lp = new Float64Array(N), v = new Float64Array(N);
        const apLen = new Int32Array(6), apOff = new Int32Array(6), apPos = new Int32Array(6);
        let apTotal = 0;
        for (let s = 0; s < 6; s++) {
            apLen[s] = nextPrime(Math.max(7, Math.round(AP_BASE[s] * Math.min(1, size))));
            apOff[s] = apTotal; apTotal += apLen[s];
        }
        const apBuf = new Float32Array(apTotal);
        const apG = 0.6;
        const wl = new Float32Array(n), wr = new Float32Array(n);
        const mL = mix.L, mR = mix.R, mn = mix.n;
        for (let i = 0; i < n; i++) {
            const j = i - pre;
            let inL = 0, inR = 0;
            if (j >= 0 && j < mn) { inL = mL[j]; inR = mR[j]; }
            for (let s = 0; s < 3; s++) {
                let p = apPos[s], q = apOff[s] + p;
                let d = apBuf[q], y = -apG * inL + d;
                apBuf[q] = inL + apG * y;
                if (++p >= apLen[s]) p = 0;
                apPos[s] = p; inL = y;
                const s2 = s + 3;
                p = apPos[s2]; q = apOff[s2] + p;
                d = apBuf[q]; y = -apG * inR + d;
                apBuf[q] = inR + apG * y;
                if (++p >= apLen[s2]) p = 0;
                apPos[s2] = p; inR = y;
            }
            let sum = 0;
            for (let k = 0; k < N; k++) {
                const y = buf[off[k] + pos[k]];
                lp[k] = (1 - damp[k]) * y + damp[k] * lp[k];
                const vk = lp[k] * gains[k];
                v[k] = vk; sum += vk;
            }
            sum *= 2 / N;
            wl[i] = (v[0] - v[1] + v[2] - v[3] + v[4] - v[5] + v[6] - v[7]) * 0.35;
            wr[i] = (v[0] + v[1] - v[2] - v[3] + v[4] + v[5] - v[6] - v[7]) * 0.35;
            for (let k = 0; k < N; k++) {
                buf[off[k] + pos[k]] = ((k & 1) ? inR : inL) * 0.5 + v[k] - sum;
                if (++pos[k] >= len[k]) pos[k] = 0;
            }
        }
        if (lowcut > 0) { highpass(wl, lowcut); highpass(wr, lowcut); }
        const oL = out.L, oR = out.R;
        for (let i = 0; i < n; i++) { oL[i] += wl[i] * wet; oR[i] += wr[i] * wet; }
        return out;
    }

    // Stereo feedback delay: L delay `time`, R delay time*(1+0.25*spread), one-pole low-pass at `tone`
    // inside the loop. Output is dry + mix * wet.
    function echo(mix, o) {
        const time = Math.max(0.005, o.time ?? 0.25);
        const fb = clamp(o.feedback ?? 0.4, 0, 0.95);
        const wet = o.mix ?? 0.3;
        const spread = Math.max(0, o.spread ?? 0);
        const tone = clamp(o.tone ?? 4000, 100, SR * 0.45);
        const dL = Math.max(1, Math.round(time * SR));
        const dR = Math.max(1, Math.round(time * (1 + 0.25 * spread) * SR));
        const repeats = Math.ceil(Math.log(0.001) / Math.log(Math.max(fb, 0.05)));
        const tail = Math.min(2, time * (1 + 0.25 * spread) * repeats);
        const n = mix.n + Math.ceil(tail * SR);
        const out = { L: new Float32Array(n), R: new Float32Array(n), n };
        const bL = new Float32Array(dL), bR = new Float32Array(dR);
        const a = Math.exp(-TAU * tone / SR);
        let pL = 0, pR = 0, lpL = 0, lpR = 0;
        const mL = mix.L, mR = mix.R, mn = mix.n, oL = out.L, oR = out.R;
        for (let i = 0; i < n; i++) {
            const xL = i < mn ? mL[i] : 0, xR = i < mn ? mR[i] : 0;
            lpL = (1 - a) * bL[pL] + a * lpL;
            lpR = (1 - a) * bR[pR] + a * lpR;
            bL[pL] = xL + fb * lpL;
            bR[pR] = xR + fb * lpR;
            if (++pL >= dL) pL = 0;
            if (++pR >= dR) pR = 0;
            oL[i] = xL + wet * lpL;
            oR[i] = xR + wet * lpR;
        }
        return out;
    }

    // Stereo chorus: two modulated taps per channel. Needs depth < delay (the engine enforces this).
    function tapAt(src, i, d) {
        const p = i - d; if (p < 0) return 0;
        const i0 = Math.floor(p), fr = p - i0;
        return src[i0] * (1 - fr) + (i0 + 1 < src.length ? src[i0 + 1] : 0) * fr;
    }
    function chorus(mix, o) {
        const depthS = o.depth ?? 0.0025, delayS = o.delay ?? 0.012;
        if (!(depthS < delayS)) throw new Error('chorus depth must be smaller than delay');
        const depth = depthS * SR, base = delayS * SR, wet = o.mix ?? 0.35, rate = o.rate ?? 0.7;
        const out = { L: new Float32Array(mix.n), R: new Float32Array(mix.n), n: mix.n };
        for (let i = 0; i < mix.n; i++) {
            const t = i / SR;
            const l1 = Math.sin(TAU * rate * t), l2 = Math.sin(TAU * rate * 1.31 * t + 1.7);
            out.L[i] = mix.L[i] + wet * tapAt(mix.L, i, base + depth * l1);
            out.R[i] = mix.R[i] + wet * tapAt(mix.R, i, base * 1.13 + depth * l2);
        }
        return out;
    }

    function saturate(buf, drive) {
        const d = Math.max(0.01, drive), norm = Math.tanh(d);
        for (let i = 0; i < buf.length; i++) buf[i] = Math.tanh(buf[i] * d) / norm;
        return buf;
    }

    // ---- metering and mastering ----------------------------------------------------------------
    // Maximum momentary loudness (ITU-R BS.1770 K-weighting, 400 ms window, 10 ms hop), in LUFS.
    // The signal is padded with 400 ms of silence, so the window always fits.
    function loudness(mix) {
        const pad = Math.ceil(0.4 * SR);
        const kw = src => {
            const b = new Float32Array(src.length + pad);
            b.set(src);
            biquad(b, 1.53512485958697, -2.69169618940638, 1.19839281085285, -1.69065929318241, 0.73248077421585);
            biquad(b, 1, -2, 1, -1.99004745483398, 0.99007225036621);
            return b;
        };
        const l = kw(mix.L), r = kw(mix.R);
        const n = l.length, win = pad, hop = Math.round(0.01 * SR);
        const cum = new Float64Array(n + 1);
        for (let i = 0; i < n; i++) cum[i + 1] = cum[i] + l[i] * l[i] + r[i] * r[i];
        let best = 0;
        for (let s = 0; s + win <= n; s += hop) best = Math.max(best, (cum[s + win] - cum[s]) / win);
        return -0.691 + 10 * Math.log10(Math.max(best, 1e-12));
    }
    // True peak estimate: 4x oversampling with a 32-tap Hann-windowed sinc interpolator (3 phases between samples).
    const TP_TAPS = 32, TP_LEFT = 15;
    const TP_PHASES = (function () {
        const h = new Float64Array(3 * TP_TAPS);
        for (let p = 1; p < 4; p++) {
            let sum = 0;
            for (let k = 0; k < TP_TAPS; k++) {
                const x = k - TP_LEFT - p / 4;
                const sinc = x === 0 ? 1 : Math.sin(Math.PI * x) / (Math.PI * x);
                const wdw = 0.5 + 0.5 * Math.cos(Math.PI * x / (TP_TAPS / 2));
                h[(p - 1) * TP_TAPS + k] = sinc * wdw; sum += sinc * wdw;
            }
            for (let k = 0; k < TP_TAPS; k++) h[(p - 1) * TP_TAPS + k] /= sum;
        }
        return h;
    })();
    function truePeak(mix) {
        let peak = 0;
        const chans = [mix.L, mix.R];
        for (const ch of chans) for (let i = 0; i < ch.length; i++) { const a = Math.abs(ch[i]); if (a > peak) peak = a; }
        for (const ch of chans) {
            const len = ch.length;
            for (let i = 0; i < len - 1; i++) {
                // an inter-sample peak between i and i+1 needs one of them to be near the peak
                if (Math.max(Math.abs(ch[i]), Math.abs(ch[i + 1])) < 0.5 * peak) continue;
                const j0 = i - TP_LEFT;
                const inside = j0 >= 0 && j0 + TP_TAPS <= len;
                for (let p = 0; p < 3; p++) {
                    const base = p * TP_TAPS;
                    let acc = 0;
                    if (inside) for (let k = 0; k < TP_TAPS; k++) acc += ch[j0 + k] * TP_PHASES[base + k];
                    else for (let k = 0; k < TP_TAPS; k++) { const j = j0 + k; if (j >= 0 && j < len) acc += ch[j] * TP_PHASES[base + k]; }
                    if (Math.abs(acc) > peak) peak = Math.abs(acc);
                }
            }
        }
        return peak;
    }

    // Final stage of every sound: 4th-order low cut, tail trim with fade, optional length cap, loudness
    // target and true-peak ceiling. Returns { mix, lufs, peakDb, seconds, naturalSeconds, limited }.
    function master(mix, o) {
        const lowcut = o.lowcut ?? 110;
        if (lowcut > 0) {
            for (const ch of [mix.L, mix.R]) { highpass(ch, lowcut, 0.5412); highpass(ch, lowcut, 1.3066); }
        }
        let peak = 0;
        for (let i = 0; i < mix.n; i++) peak = Math.max(peak, Math.abs(mix.L[i]), Math.abs(mix.R[i]));
        if (peak < 1e-9) {
            return { mix: { L: new Float32Array(0), R: new Float32Array(0), n: 0 }, lufs: -120, peakDb: -120, seconds: 0, naturalSeconds: 0, limited: false };
        }
        const floor = peak * dbToGain(-66);
        let end = mix.n - 1;
        while (end > 0 && Math.abs(mix.L[end]) < floor && Math.abs(mix.R[end]) < floor) end--;
        const fade = Math.round(0.015 * SR);
        let n = Math.min(mix.n, end + fade);
        const naturalSeconds = n / SR;
        if (o.maxSeconds != null) n = Math.min(n, Math.max(1, Math.round(o.maxSeconds * SR)));
        const out = { L: mix.L.slice(0, n), R: mix.R.slice(0, n), n };
        for (let i = 0; i < fade && i < n; i++) {
            const g = 0.5 + 0.5 * Math.cos(Math.PI * (1 - i / fade));
            out.L[n - 1 - i] *= g; out.R[n - 1 - i] *= g;
        }
        const head = Math.round(0.0005 * SR);
        for (let i = 0; i < head && i < n; i++) { out.L[i] *= i / head; out.R[i] *= i / head; }
        const lufs = loudness(out), tp = truePeak(out);
        const gLoud = dbToGain((o.lufs ?? -20) - lufs), gTp = dbToGain(o.ceiling ?? -3) / tp;
        const gain = Math.min(gLoud, gTp);
        for (let i = 0; i < n; i++) { out.L[i] *= gain; out.R[i] *= gain; }
        out.L[0] = 0; out.R[0] = 0; out.L[n - 1] = 0; out.R[n - 1] = 0; // exact zero ends (no -0, no residue)
        return { mix: out, lufs: lufs + gainToDb(gain), peakDb: gainToDb(tp * gain), seconds: n / SR, naturalSeconds, limited: gTp < gLoud };
    }

    const api = {
        SR, makeRng, hashSeed, midiToHz, dbToGain, gainToDb, clamp, createMix, addMono, addStereo, panMono, applyEnv,
        modal, fm, osc, pluck, bubble, noise, svf, biquad, polyblep, lowpass, highpass, shelf,
        reverb, echo, chorus, saturate, loudness, truePeak, master
    };
    root.DesktopSoundDSP = api;
    if (typeof module === 'object' && module && module.exports) module.exports = api;
})(typeof self !== 'undefined' ? self : globalThis);
