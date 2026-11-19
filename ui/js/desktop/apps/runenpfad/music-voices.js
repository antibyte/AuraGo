// Runenpfad orchestral & world instrument synthesis engine, stereo delay/reverb master bus,
// and dramatic stingers. Consumes RunenpfadAudioDsp physical models & PeriodicWaves (zero DOM/RNG randomness).
(function (root) {
    'use strict';

    function hz(midi) { return 440 * Math.pow(2, (midi - 69) / 12); }

    function env(g, t, peak, a, end, rel) {
        const p = Math.max(0.00015, peak);
        const rStart = Math.max(t + a + 0.005, end - (rel || 0.05));
        g.gain.setValueAtTime(0.0001, t);
        g.gain.linearRampToValueAtTime(p, t + Math.max(0.002, a));
        if (rStart > t + a + 0.01) g.gain.exponentialRampToValueAtTime(Math.max(0.00015, p * 0.72), rStart);
        g.gain.exponentialRampToValueAtTime(0.0001, Math.max(rStart + 0.01, end));
    }

    // Studio master bus for Runenpfad music: HPF sub cleanup -> warmth/presence EQ ->
    // tape saturation -> ducking stage -> gentle bus compressor -> port.bus + reverb + stereo delay.
    function createMasterChain(ctx, port) {
        const nodes = [];
        const keep = (n) => { nodes.push(n); return n; };
        const disc = (n) => { try { n.disconnect(); } catch (_) {} };
        const dsp = root.RunenpfadAudioDsp;

        const input = keep(ctx.createGain());
        input.gain.value = 1.0;

        const hpf = keep(ctx.createBiquadFilter());
        hpf.type = 'highpass';
        hpf.frequency.value = 40;
        hpf.Q.value = 0.7;

        const lowWarm = keep(ctx.createBiquadFilter());
        lowWarm.type = 'peaking';
        lowWarm.frequency.value = 185;
        lowWarm.Q.value = 0.9;
        lowWarm.gain.value = -1.8;

        const presence = keep(ctx.createBiquadFilter());
        presence.type = 'peaking';
        presence.frequency.value = 3400;
        presence.Q.value = 0.85;
        presence.gain.value = 2.0;

        const air = keep(ctx.createBiquadFilter());
        air.type = 'highshelf';
        air.frequency.value = 7200;
        air.gain.value = 2.0;

        const duckGain = keep(ctx.createGain());
        duckGain.gain.value = 1.0;

        const outGain = keep(ctx.createGain());
        outGain.gain.value = 0.92;

        input.connect(hpf);
        hpf.connect(lowWarm);
        lowWarm.connect(presence);
        presence.connect(air);

        let preSat = null;
        let lastStage = air;
        if (typeof ctx.createWaveShaper === 'function' && dsp) {
            preSat = keep(ctx.createGain());
            preSat.gain.value = 0.50;
            const shaper = keep(ctx.createWaveShaper());
            shaper.curve = dsp.getCurve(ctx, 'tape');
            shaper.oversample = '2x';
            const postSat = keep(ctx.createGain());
            postSat.gain.value = 1.24;
            lastStage.connect(preSat);
            preSat.connect(shaper);
            shaper.connect(postSat);
            lastStage = postSat;
        }
        lastStage.connect(duckGain);

        if (typeof ctx.createDynamicsCompressor === 'function') {
            const comp = keep(ctx.createDynamicsCompressor());
            comp.threshold.value = -8;
            comp.knee.value = 12;
            comp.ratio.value = 1.6;
            comp.attack.value = 0.018;
            comp.release.value = 0.20;
            duckGain.connect(comp);
            comp.connect(outGain);
        } else {
            duckGain.connect(outGain);
        }

        const busDest = (port && port.bus) || ctx.destination;
        if (busDest) outGain.connect(busDest);

        // Reverb send + warm stereo dotted-8th delay return.
        const revSend = keep(ctx.createGain());
        revSend.gain.value = 0.34;
        duckGain.connect(revSend);
        if (port && port.reverb) revSend.connect(port.reverb);

        let dlyL = null, dlyR = null;
        const wetSend = keep(ctx.createGain());
        wetSend.gain.value = 0.19;
        if (typeof ctx.createDelay === 'function') {
            dlyL = keep(ctx.createDelay(1.5));
            dlyR = keep(ctx.createDelay(1.5));
            dlyL.delayTime.value = 0.34;
            dlyR.delayTime.value = 0.51;
            const dlyFilt = keep(ctx.createBiquadFilter());
            dlyFilt.type = 'lowpass';
            dlyFilt.frequency.value = 2600;
            const fbL = keep(ctx.createGain());
            const fbR = keep(ctx.createGain());
            fbL.gain.value = 0.28;
            fbR.gain.value = 0.24;
            const dlyRet = keep(ctx.createGain());
            dlyRet.gain.value = 0.52;

            wetSend.connect(dlyFilt);
            dlyFilt.connect(dlyL);
            dlyFilt.connect(dlyR);
            dlyL.connect(fbL); fbL.connect(dlyR);
            dlyR.connect(fbR); fbR.connect(dlyL);
            if (typeof ctx.createStereoPanner === 'function') {
                const pL = keep(ctx.createStereoPanner()); pL.pan.value = -0.62;
                const pR = keep(ctx.createStereoPanner()); pR.pan.value = 0.62;
                dlyL.connect(pL); pL.connect(dlyRet);
                dlyR.connect(pR); pR.connect(dlyRet);
            } else {
                dlyL.connect(dlyRet);
                dlyR.connect(dlyRet);
            }
            dlyRet.connect(duckGain);
            if (port && port.reverb) dlyRet.connect(revSend);
        }

        function setTempo(bpm) {
            if (!dlyL || !dlyR || !bpm) return;
            const spb = 60 / bpm;
            const t = ctx.currentTime;
            dlyL.delayTime.setValueAtTime(Math.min(1.4, spb * 0.75), t);
            dlyR.delayTime.setValueAtTime(Math.min(1.4, spb * 1.125), t);
        }

        function duck(depth, hold, rel) {
            const now = ctx.currentTime;
            const target = Math.max(0.18, 1 - (depth || 0.28));
            const h = hold || 0.14;
            const r = rel || 0.55;
            duckGain.gain.cancelScheduledValues(now);
            duckGain.gain.setTargetAtTime(target, now, 0.025);
            duckGain.gain.setTargetAtTime(1.0, now + h, Math.max(0.08, r * 0.35));
        }

        let baseWet = 0.34;
        let lastDrama = 0;

        function setAmbientWet(level) {
            baseWet = Math.max(0.14, Math.min(0.6, level));
            revSend.gain.setTargetAtTime(Math.max(0.14, Math.min(0.56, baseWet * (1.12 - 0.36 * lastDrama))), ctx.currentTime, 0.4);
        }

        function setDrama(level, urgency) {
            const d = Math.max(0, Math.min(1, level || 0));
            const u = Math.max(0, Math.min(1, urgency || 0));
            lastDrama = d;
            const now = ctx.currentTime;
            presence.gain.setTargetAtTime(2.0 + 1.8 * d, now, 0.35);
            air.gain.setTargetAtTime(2.0 + 1.6 * d, now, 0.35);
            revSend.gain.setTargetAtTime(Math.max(0.14, Math.min(0.56, baseWet * (1.12 - 0.36 * d))), now, 0.35);
            if (preSat) preSat.gain.setTargetAtTime(0.50 + 0.24 * d + 0.06 * u, now, 0.35);
        }

        function dispose() {
            nodes.forEach(disc);
            nodes.length = 0;
        }

        return { input, wetInput: wetSend, setTempo, setDrama, duck, setAmbientWet, dispose };
    }

    // Play a pre-baked physical-model sample pitched cleanly by f / refHz.
    function playModel(h, id, f, t, dur, vel, dest, opts) {
        const dsp = h.dsp;
        const sample = dsp && dsp.getSample(h.ctx, id);
        if (!sample) return null;
        const src = h.keep(h.ctx.createBufferSource());
        src.buffer = sample.buf;
        if (f > 0 && sample.refHz) {
            src.playbackRate.value = Math.max(0.25, Math.min(4.0, f / sample.refHz));
        }
        const g = h.keep(h.ctx.createGain());
        let node = src;
        if (opts && opts.lp) {
            const lp = h.keep(h.filt('lowpass', opts.lp * (0.72 + 0.45 * vel), opts.q || 0.9));
            node.connect(lp);
            node = lp;
        }
        node.connect(g);
        g.connect(dest);
        const playDur = opts && opts.maxDur ? Math.min(dur, opts.maxDur) : dur;
        const end = t + Math.max(0.06, playDur);
        env(g, t, vel * ((opts && opts.gain) || 0.32), (opts && opts.att) || 0.002, end, (opts && opts.rel) || 0.08);
        src.start(t);
        src.stop(end + 0.03);
        return { main: src, end };
    }

    // Play a sustained harmonic voice using PeriodicWave + optional vibrato, breath noise, & filter sweep.
    function playWaveVoice(h, waveKind, fbWave, f, t, dur, vel, dest, opts) {
        const o = h.keep(h.ctx.createOscillator());
        const pw = h.dsp && h.dsp.getWave(h.ctx, waveKind);
        if (pw && typeof o.setPeriodicWave === 'function') o.setPeriodicWave(pw);
        else o.type = fbWave || 'triangle';
        o.frequency.value = f;

        let tail = o;
        if (opts && opts.detune) {
            const o2 = h.keep(h.ctx.createOscillator());
            if (pw && typeof o2.setPeriodicWave === 'function') o2.setPeriodicWave(pw);
            else o2.type = fbWave || 'triangle';
            o2.frequency.value = f * (1 + opts.detune);
            o2.start(t);
            o2.stop(t + dur + 0.04);
            const sum = h.keep(h.ctx.createGain());
            o.connect(sum);
            o2.connect(sum);
            tail = sum;
        }

        if (opts && opts.vibHz && opts.vibDepth && dur > 0.25) {
            const lfo = h.keep(h.ctx.createOscillator());
            const lg = h.keep(h.ctx.createGain());
            lfo.frequency.value = opts.vibHz;
            lg.gain.setValueAtTime(0.0001, t);
            lg.gain.linearRampToValueAtTime(opts.vibDepth, t + Math.min(0.28, dur * 0.45));
            lfo.connect(lg);
            lg.connect(o.frequency);
            lfo.start(t);
            lfo.stop(t + dur + 0.04);
        }

        const g = h.keep(h.ctx.createGain());
        if (opts && opts.lp) {
            const lp = h.keep(h.filt('lowpass', opts.lp, opts.q || 1.1));
            if (opts.lpPeak && opts.lpPeak > opts.lp) {
                lp.frequency.setValueAtTime(opts.lp, t);
                lp.frequency.exponentialRampToValueAtTime(opts.lpPeak, t + Math.min(0.14, dur * 0.35));
                lp.frequency.exponentialRampToValueAtTime(Math.max(80, opts.lp * 0.8), t + dur);
            }
            tail.connect(lp);
            lp.connect(g);
        } else {
            tail.connect(g);
        }

        if (opts && opts.breath) {
            const n = h.keep(h.noiseSrc(t, opts.breath, 'pink'));
            const bp = h.keep(h.filt('bandpass', Math.min(3400, f * 2.2), 1.6));
            const bg = h.keep(h.ctx.createGain());
            const bEnd = t + Math.min(dur, 0.14);
            bg.gain.setValueAtTime(Math.max(0.0002, opts.breath * 0.32), t);
            bg.gain.exponentialRampToValueAtTime(0.0001, Math.max(t + 0.02, bEnd));
            n.connect(bp);
            bp.connect(bg);
            bg.connect(g);
            n.start(t);
            n.stop(Math.max(t + 0.03, bEnd + 0.02));
        }

        if (opts && opts.drive && typeof h.ctx.createWaveShaper === 'function' && h.dsp) {
            const ws = h.keep(h.ctx.createWaveShaper());
            ws.curve = h.dsp.getCurve(h.ctx, opts.drive);
            g.connect(ws);
            ws.connect(dest);
        } else {
            g.connect(dest);
        }

        const end = t + dur;
        env(g, t, vel * ((opts && opts.gain) || 0.22), (opts && opts.att) || 0.03, end, (opts && opts.rel) || Math.min(0.25, dur * 0.35));
        o.start(t);
        o.stop(end + 0.04);
        return { main: o, end };
    }

    const MODEL_MAP = {
        harp: { id: 'harp', lp: 8400, gain: 0.34, rel: 0.14 },
        lute: { id: 'lute', lp: 6800, gain: 0.34, rel: 0.12 },
        oud: { id: 'oud', lp: 6200, gain: 0.34, rel: 0.12 },
        pluck: { id: 'lute', lp: 6600, gain: 0.32, rel: 0.12 },
        pizz: { id: 'pizz', lp: 3800, gain: 0.34, rel: 0.08 },
        marimba: { id: 'marimba', lp: 6400, gain: 0.32, rel: 0.10 },
        celesta: { id: 'celesta', lp: 9200, gain: 0.31, rel: 0.18 },
        plink: { id: 'celesta', lp: 8400, gain: 0.28, rel: 0.12 },
        darkbell: { id: 'darkbell', lp: 5400, gain: 0.31, rel: 0.25 },
        glass: { id: 'glass', lp: 9600, gain: 0.28, rel: 0.22 },
        bell: { id: 'celesta', lp: 8800, gain: 0.29, rel: 0.16 },
        anvil: { id: 'anvil', lp: 7200, gain: 0.28, rel: 0.16 },
        clank: { id: 'anvil', lp: 5800, gain: 0.25, rel: 0.12 },
        metal: { id: 'anvil', lp: 6400, gain: 0.26, rel: 0.14 },
        kick: { id: 'kick', lp: 3200, gain: 0.40, maxDur: 0.34, rel: 0.06 },
        snare: { id: 'snare', lp: 9200, gain: 0.40, maxDur: 0.24, rel: 0.04 },
        hat: { id: 'hat', lp: 12000, gain: 0.28, maxDur: 0.14, rel: 0.025 },
        taiko: { id: 'taiko', lp: 3800, gain: 0.38, maxDur: 0.82, rel: 0.14 },
        framedrum: { id: 'framedrum', lp: 3600, gain: 0.38, maxDur: 0.42, rel: 0.08 },
        darbuka: { id: 'darbuka_doum', lp: 4200, gain: 0.38, maxDur: 0.36, rel: 0.07 },
        darbuka_doum: { id: 'darbuka_doum', lp: 3800, gain: 0.40, maxDur: 0.38, rel: 0.07 },
        darbuka_tek: { id: 'darbuka_tek', lp: 8600, gain: 0.35, maxDur: 0.14, rel: 0.03 },
        tom: { id: 'tom', lp: 3600, gain: 0.38, maxDur: 0.40, rel: 0.08 },
        tamb: { id: 'tamb', lp: 11500, gain: 0.30, maxDur: 0.24, rel: 0.05 }
    };

    const WAVE_MAP = {
        pad: { wave: 'strings', fb: 'sawtooth', lp: 1850, lpPeak: 3100, detune: 0.0038, att: 0.16, gain: 0.12, rel: 0.32 },
        string: { wave: 'strings', fb: 'sawtooth', lp: 2800, lpPeak: 4600, detune: 0.0032, vibHz: 5.1, vibDepth: 2.2, att: 0.08, gain: 0.19, rel: 0.20 },
        choir: { wave: 'choir', fb: 'triangle', lp: 1950, lpPeak: 2800, detune: 0.0042, vibHz: 4.8, vibDepth: 1.8, breath: 0.09, att: 0.18, gain: 0.15, rel: 0.35 },
        cello: { wave: 'cello', fb: 'sawtooth', lp: 1750, lpPeak: 2800, vibHz: 4.9, vibDepth: 1.6, att: 0.06, gain: 0.21, rel: 0.16 },
        fiddle: { wave: 'strings', fb: 'sawtooth', lp: 3800, lpPeak: 5400, vibHz: 5.8, vibDepth: 4.0, att: 0.025, gain: 0.22, rel: 0.08 },
        flute: { wave: 'flute', fb: 'sine', lp: 5200, vibHz: 5.2, vibDepth: 3.4, breath: 0.18, att: 0.04, gain: 0.24, rel: 0.10 },
        ney: { wave: 'ney', fb: 'triangle', lp: 4200, lpPeak: 5600, vibHz: 4.7, vibDepth: 4.5, breath: 0.25, att: 0.045, gain: 0.25, rel: 0.11 },
        reed: { wave: 'reed', fb: 'sawtooth', lp: 2900, lpPeak: 4400, vibHz: 4.6, vibDepth: 2.8, att: 0.03, gain: 0.21, rel: 0.09 },
        horn: { wave: 'horn', fb: 'sawtooth', lp: 1550, lpPeak: 4400, att: 0.04, gain: 0.25, rel: 0.16 },
        brass: { wave: 'brass', fb: 'sawtooth', lp: 1850, lpPeak: 5200, drive: 'warm', att: 0.022, gain: 0.25, rel: 0.14 },
        organ: { wave: 'organ', fb: 'sawtooth', lp: 3400, detune: 0.0024, att: 0.018, gain: 0.19, rel: 0.14 },
        lead: { wave: 'brass', fb: 'sawtooth', lp: 2400, lpPeak: 4600, vibHz: 5.2, vibDepth: 2.5, att: 0.014, gain: 0.22, rel: 0.10 },
        bass: { wave: 'bass', fb: 'triangle', lp: 680, lpPeak: 1200, att: 0.006, gain: 0.28, rel: 0.06 },
        fuzzbass: { wave: 'bass', fb: 'sawtooth', lp: 1100, lpPeak: 2100, drive: 'fuzz', att: 0.005, gain: 0.27, rel: 0.06 },
        ostinato: { wave: 'cello', fb: 'sawtooth', lp: 1950, lpPeak: 4200, drive: 'warm', att: 0.005, gain: 0.24, rel: 0.05 },
        stab: { wave: 'brass', fb: 'sawtooth', lp: 1800, lpPeak: 4200, drive: 'warm', att: 0.006, gain: 0.21, rel: 0.09 },
        drone: { wave: 'cello', fb: 'sawtooth', lp: 640, detune: 0.0045, att: 0.20, gain: 0.15, rel: 0.35 }
    };

    // Builds any Runenpfad voice returning { nodes, src, end } within strict node budget (3..7 nodes).
    function buildVoice(h, ev, t, f, dur, dest, pan) {
        const vel = Math.max(0.05, ev.vel || 0.7);
        const type = ev.type || 'harp';
        let target = dest;
        if (Math.abs(pan) > 0.04 && typeof h.ctx.createStereoPanner === 'function') {
            const p = h.keep(h.ctx.createStereoPanner());
            p.pan.value = Math.max(-0.92, Math.min(0.92, pan));
            p.connect(dest);
            target = p;
        }
        // Unpitched noise & special percussion effects.
        if (type === 'growl') {
            return playWaveVoice(h, 'bass', 'sawtooth', Math.max(48, f * 0.5), t, dur, vel, target, { lp: 420, lpPeak: 780, drive: 'fuzz', att: 0.03, gain: 0.34, rel: 0.15 });
        }
        if (type === 'clock') {
            const m = h.keep(h.noiseSrc(t, 0.25, 'crackle'));
            const bp = h.keep(h.filt('bandpass', 2400, 3.2));
            const g = h.keep(h.ctx.createGain());
            m.connect(bp); bp.connect(g); g.connect(target);
            const end = t + 0.055;
            env(g, t, 0.22 * vel, 0.001, end, 0.04);
            m.start(t); m.stop(end + 0.02);
            return { main: m, end };
        }
        if (type === 'tick') {
            const m = h.keep(h.noiseSrc(t, 0.25, 'white'));
            const bp = h.keep(h.filt('bandpass', 4600, 3.8));
            const g = h.keep(h.ctx.createGain());
            m.connect(bp); bp.connect(g); g.connect(target);
            const end = t + 0.042;
            env(g, t, 0.16 * vel, 0.001, end, 0.025);
            m.start(t); m.stop(end + 0.02);
            return { main: m, end };
        }
        if (type === 'shaker') {
            const m = h.keep(h.noiseSrc(t, 0.35, 'pink'));
            const bp = h.keep(h.filt('bandpass', 6800, 1.4));
            const g = h.keep(h.ctx.createGain());
            m.connect(bp); bp.connect(g); g.connect(target);
            const end = t + Math.max(0.05, Math.min(dur, 0.09));
            env(g, t, 0.16 * vel, 0.006, end, 0.04);
            m.start(t); m.stop(end + 0.02);
            return { main: m, end };
        }
        if (type === 'swell') {
            const m = h.keep(h.noiseSrc(t, 0.24, 'pink'));
            const bp = h.keep(h.filt('bandpass', 360, 1.4));
            bp.frequency.setValueAtTime(360, t);
            bp.frequency.exponentialRampToValueAtTime(3600, t + Math.max(0.05, dur));
            const g = h.keep(h.ctx.createGain());
            m.connect(bp); bp.connect(g); g.connect(target);
            const end = t + dur + 0.05;
            env(g, t, 0.18 * vel, Math.max(0.04, dur * 0.78), end, Math.max(0.04, dur * 0.22));
            m.start(t); m.stop(end + 0.02);
            return { main: m, end };
        }

        const modSpec = MODEL_MAP[type];
        if (modSpec) {
            const res = playModel(h, modSpec.id, f, t, dur, vel, target, modSpec);
            if (res) return res;
        }
        const waveSpec = WAVE_MAP[type] || WAVE_MAP.string;
        return playWaveVoice(h, (waveSpec && waveSpec.wave) || 'strings', (waveSpec && waveSpec.fb) || 'triangle', f || 220, t, dur, vel, target, waveSpec);
    }

    function playStinger(ctx, out, kind, theme) {
        if (!ctx || !out) return;
        const dsp = root.RunenpfadAudioDsp;
        const emit = (ev, at, fq, d, pan) => {
            const vNodes = [];
            const vh = {
                ctx,
                keep: (n) => { vNodes.push(n); return n; },
                dsp,
                filt: (t0, f0, q) => { const fl = ctx.createBiquadFilter(); fl.type = t0; fl.frequency.value = f0; fl.Q.value = q; return fl; },
                noiseSrc: (t0, amp, k) => {
                    const s = ctx.createBufferSource();
                    s.buffer = (dsp && dsp.getNoise(ctx, k || 'pink')) || ctx.createBuffer(1, 2048, 44100);
                    s.loop = true;
                    return s;
                }
            };
            const res = buildVoice(vh, ev, at, fq, d, out, pan || 0);
            if (res && res.main) res.main.onended = () => vNodes.forEach((n) => { try { n.disconnect(); } catch (_) {} });
        };
        const t = ctx.currentTime + 0.025;
        const r = (theme && theme.root ? theme.root : 62) + 12;
        const scale = (theme && theme.scale) || [0, 2, 3, 5, 7, 8, 10];
        const step = (d) => r + scale[((d % scale.length) + scale.length) % scale.length] + Math.floor(d / scale.length) * 12;

        if (kind === 'victory') {
            [0, 2, 4, 7].forEach((d, i) => {
                emit({ type: 'horn', vel: 0.78 }, t + i * 0.13, hz(step(d)), i === 3 ? 1.5 : 0.45, (i - 1.5) * 0.22);
                emit({ type: 'harp', vel: 0.65 }, t + i * 0.13, hz(step(d) + 12), 1.35, (i % 2 ? 0.35 : -0.35));
            });
            emit({ type: 'taiko', vel: 0.8 }, t, 55, 0.7, 0);
        } else if (kind === 'defeat') {
            [4, 2, 1, 0].forEach((d, i) => {
                emit({ type: 'cello', vel: 0.72 }, t + i * 0.28, hz(step(d) - 12), i === 3 ? 1.7 : 0.65, -0.2);
            });
            emit({ type: 'darkbell', vel: 0.55 }, t + 0.02, hz(r - 12), 2.1, 0.18);
        } else if (kind === 'rune') {
            [0, 2, 4, 7, 9].forEach((d, i) => {
                emit({ type: 'celesta', vel: 0.65 }, t + i * 0.055, hz(step(d) + 12), 0.9, (i - 2) * 0.18);
            });
        } else if (kind === 'checkpoint') {
            emit({ type: 'horn', vel: 0.65 }, t, hz(r - 12), 1.1, -0.15);
            [0, 4, 7].forEach((d, i) => {
                emit({ type: 'harp', vel: 0.68 }, t + i * 0.09, hz(step(d)), 1.25, (i - 1) * 0.28);
            });
        } else if (kind === 'boss_intro') {
            emit({ type: 'taiko', vel: 0.92 }, t, 52, 0.85, -0.12);
            emit({ type: 'taiko', vel: 0.95 }, t + 0.42, 48, 0.9, 0.12);
            emit({ type: 'anvil', vel: 0.55 }, t + 0.84, 220, 1.0, 0.28);
            [0, 1, 4].forEach((d, i) => {
                emit({ type: 'brass', vel: 0.78 }, t + 0.84, hz(step(d) - 12), 1.7, (i - 1) * 0.3);
            });
        } else if (kind === 'phase_up') {
            emit({ type: 'taiko', vel: 0.95 }, t, 50, 0.8, -0.18);
            emit({ type: 'taiko', vel: 0.98 }, t + 0.22, 54, 0.85, 0.18);
            emit({ type: 'anvil', vel: 0.65 }, t + 0.44, 240, 1.0, 0.25);
            [0, 1, 4, 7].forEach((d, i) => {
                emit({ type: i % 2 ? 'brass' : 'horn', vel: 0.82 }, t + 0.44 + i * 0.04, hz(step(d) - 12), 1.55, (i - 1.5) * 0.24);
            });
        } else if (kind === 'dragon_warn') {
            emit({ type: 'taiko', vel: 0.95 }, t, 46, 0.9, -0.2);
            emit({ type: 'swell', vel: 0.9 }, t, 0, 0.85, 0);
            [0, 4].forEach((d, i) => {
                emit({ type: 'horn', vel: 0.84 }, t + 0.12, hz(step(d) - 12), 1.45, i ? 0.25 : -0.25);
            });
        } else if (kind === 'key') {
            [0, 2, 4, 7].forEach((d, i) => {
                emit({ type: 'horn', vel: 0.74 }, t + i * 0.08, hz(step(d) - 12), i === 3 ? 1.2 : 0.45, (i - 1.5) * 0.2);
                emit({ type: 'harp', vel: 0.66 }, t + i * 0.08, hz(step(d) + 12), 1.1, (i % 2 ? 0.3 : -0.3));
            });
        } else if (kind === 'solved') {
            [0, 2, 4, 7, 9].forEach((d, i) => {
                emit({ type: i === 4 ? 'celesta' : 'harp', vel: 0.7 }, t + i * 0.07, hz(step(d) + (i === 4 ? 12 : 0)), 1.15, (i - 2) * 0.18);
            });
        } else if (kind === 'secret') {
            [4, 2, 5, 7].forEach((d, i) => {
                emit({ type: i % 2 ? 'glass' : 'harp', vel: 0.62 }, t + i * 0.09, hz(step(d) + 12), 1.05, (i - 1.5) * 0.25);
            });
        } else if (kind === 'tension') {
            emit({ type: 'taiko', vel: 0.86 }, t, 56, 0.65, -0.15);
            emit({ type: 'pizz', vel: 0.74 }, t + 0.04, hz(step(0)), 0.45, 0.22);
            emit({ type: 'horn', vel: 0.7 }, t + 0.12, hz(step(4) - 12), 0.9, -0.2);
        } else if (kind === 'danger') {
            emit({ type: 'cello', vel: 0.78 }, t, hz(step(0) - 12), 0.95, -0.22);
            emit({ type: 'darkbell', vel: 0.56 }, t + 0.08, hz(step(1)), 1.35, 0.22);
        }
    }

    root.RunenpfadMusicVoices = {
        createMasterChain,
        buildVoice,
        playStinger,
        MODEL_NAMES: Object.keys(MODEL_MAP),
        WAVE_NAMES: Object.keys(WAVE_MAP)
    };
})(typeof window !== 'undefined' ? window : globalThis);
