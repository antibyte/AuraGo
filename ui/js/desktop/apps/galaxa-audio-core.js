(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    GC.createAudioCore = function (ctx) {
                function audio() {
            if (!ctx.actx) try { ctx.actx = new (window.AudioContext || window.webkitAudioContext)(); } catch (e) { return null; }
            if (ctx.actx && ctx.actx.state === 'suspended') ctx.actx.resume();
            if (ctx.actx && !ctx.masterCompressor) {
                ctx.masterCompressor = ctx.actx.createDynamicsCompressor();
                ctx.masterCompressor.threshold.value = -12;
                ctx.masterCompressor.knee.value = 10;
                ctx.masterCompressor.ratio.value = 4;
                ctx.masterCompressor.attack.value = 0.003;
                ctx.masterCompressor.release.value = 0.15;
                ctx.masterCompressor.connect(ctx.actx.destination);
                try {
                    ctx.reverbNode = ctx.actx.createConvolver();
                    const rate = ctx.actx.sampleRate, length = Math.floor(rate * 0.4);
                    const impulse = ctx.actx.createBuffer(2, length, rate);
                    for (let ch = 0; ch < 2; ch++) { const d = impulse.getChannelData(ch); for (let i = 0; i < length; i++) d[i] = (Math.random() * 2 - 1) * Math.pow(1 - i / length, 3); }
                    ctx.reverbNode.buffer = impulse;
                    ctx.reverbGain = ctx.actx.createGain(); ctx.reverbGain.gain.value = 0.12;
                    ctx.reverbNode.connect(ctx.reverbGain); ctx.reverbGain.connect(ctx.masterCompressor);
                } catch (_) { ctx.reverbNode = null; ctx.reverbGain = null; }
            }
            return ctx.actx;
        }

        function beep(type, f0, f1, dur, vol, panX) {
            const a = audio(); if (!a || ctx.G.muted) return;
            const o = a.createOscillator(), g = a.createGain();
            o.type = type; o.frequency.setValueAtTime(f0, a.currentTime);
            if (f1 !== f0) o.frequency.linearRampToValueAtTime(f1, a.currentTime + dur);
            g.gain.setValueAtTime(ctx.G.vol * vol, a.currentTime);
            g.gain.exponentialRampToValueAtTime(0.001, a.currentTime + dur + 0.02);
            if (panX !== undefined && a.createStereoPanner) {
                const p = a.createStereoPanner();
                p.pan.value = Math.max(-1, Math.min(1, (panX / (ctx.W / 2)) - 1));
                o.connect(g).connect(p).connect(a.destination);
            } else {
                o.connect(g).connect(a.destination);
            }
            o.start(); o.stop(a.currentTime + dur + 0.02);
        }

        function noise(dur, vol, freq, panX) {
            const a = audio(); if (!a || ctx.G.muted) return;
            const buf = a.createBuffer(1, a.sampleRate * dur, a.sampleRate), d = buf.getChannelData(0);
            for (let i = 0; i < d.length; i++) d[i] = (Math.random() * 2 - 1) * (1 - i / d.length);
            const s = a.createBufferSource(), f = a.createBiquadFilter(), g = a.createGain();
            s.buffer = buf; f.type = 'lowpass'; f.frequency.value = freq || 2000;
            g.gain.setValueAtTime(ctx.G.vol * vol, a.currentTime);
            g.gain.exponentialRampToValueAtTime(0.001, a.currentTime + dur);
            if (panX !== undefined && a.createStereoPanner) {
                const p = a.createStereoPanner();
                p.pan.value = Math.max(-1, Math.min(1, (panX / (ctx.W / 2)) - 1));
                s.connect(f).connect(g).connect(p).connect(a.destination);
            } else {
                s.connect(f).connect(g).connect(a.destination);
            }
            s.start();
        }

        function schedNoise(startTime, dur, vol, freq, dest, panX) {
            const a = audio(); if (!a || ctx.G.muted) return null;
            const buf = a.createBuffer(1, Math.max(1, Math.floor(a.sampleRate * dur)), a.sampleRate), d = buf.getChannelData(0);
            for (let i = 0; i < d.length; i++) d[i] = (Math.random() * 2 - 1) * (1 - i / d.length);
            const s = a.createBufferSource(), f = a.createBiquadFilter(), g = a.createGain();
            s.buffer = buf; f.type = freq > 4000 ? 'highpass' : 'lowpass'; f.frequency.value = freq || 2000;
            g.gain.setValueAtTime(ctx.G.vol * vol, startTime);
            g.gain.exponentialRampToValueAtTime(0.001, startTime + dur);
            const target = dest || a.destination;
            if (panX !== undefined && a.createStereoPanner) {
                const p = a.createStereoPanner();
                p.pan.value = Math.max(-1, Math.min(1, (panX / (ctx.W / 2)) - 1));
                s.connect(f).connect(g).connect(p).connect(target);
            } else {
                s.connect(f).connect(g).connect(target);
            }
            s.start(startTime); s.stop(startTime + dur + 0.01);
            return s;
        }

        function pv() { return 0.95 + Math.random() * 0.1; }
        function vv() { return 0.9 + Math.random() * 0.2; }

        // NEW: Audio ducking — temporarily lower music master gain on loud SFX
        let duckTimer = 0, duckTarget = 1;
        function duckMusic(amount, durMs) {
            if (!ctx.MusicEngine.masterGain) return;
            const a = audio(); if (!a) return;
            duckTarget = Math.max(0.2, 1 - amount);
            ctx.MusicEngine.masterGain.gain.linearRampToValueAtTime(ctx.G.muted ? 0 : ctx.G.vol * 0.35 * duckTarget, a.currentTime + 0.04);
            duckTimer = durMs;
        }
        function updateDuck(dtMs) {
            if (duckTimer > 0) { duckTimer -= dtMs; if (duckTimer <= 0) { duckTimer = 0; const a = audio(); if (a && ctx.MusicEngine.masterGain) ctx.MusicEngine.masterGain.gain.linearRampToValueAtTime(ctx.G.muted ? 0 : ctx.G.vol * 0.35, a.currentTime + 0.2); } }
        }
        ctx.duckMusic = duckMusic;
        ctx.updateDuck = updateDuck;
        ctx.audio = audio;
        ctx.beep = beep;
        ctx.noise = noise;
        ctx.schedNoise = schedNoise;
        ctx.pv = pv;
        ctx.vv = vv;
    };
})();
