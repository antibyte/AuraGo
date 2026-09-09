(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};
    GC.createAudioCore = function (ctx) {
        const voices = new Set(), musicVoices = new Set();
        let duckUntil = 0, duck = 1;
        ctx.sfxPriority = 1;
        function audio() {
            if (ctx.state.disposed) return null;
            if (!ctx.actx) try { ctx.actx = new (window.AudioContext || window.webkitAudioContext)(); } catch (_) { return null; }
            const a = ctx.actx;
            if (!ctx.masterCompressor) {
                ctx.masterCompressor = a.createDynamicsCompressor();
                Object.assign(ctx, { masterBus: a.createGain(), musicBus: a.createGain(), sfxBus: a.createGain() });
                ctx.masterCompressor.threshold.value = -10; ctx.masterCompressor.knee.value = 6;
                ctx.masterCompressor.ratio.value = 12; ctx.masterCompressor.attack.value = 0.002; ctx.masterCompressor.release.value = 0.15;
                ctx.musicBus.connect(ctx.masterCompressor); ctx.sfxBus.connect(ctx.masterCompressor);
                ctx.masterCompressor.connect(ctx.masterBus); ctx.masterBus.connect(a.destination);
                ctx.masterBus.gain.value = ctx.G.muted ? 0 : (ctx.settings.vol ?? 30) / 100 * 0.8;
                ctx.musicBus.gain.value = (ctx.settings.musicVol ?? 70) / 100;
                ctx.sfxBus.gain.value = (ctx.settings.sfxVol ?? 85) / 100;
                ctx.noiseBuffer = a.createBuffer(1, a.sampleRate * 2, a.sampleRate);
                const samples = ctx.noiseBuffer.getChannelData(0);
                for (let i = 0; i < samples.length; i++) samples[i] = Math.random() * 2 - 1;
                applyAudioSettings();
            }
            return a;
        }
        function applyAudioSettings() {
            if (ctx.masterBus) {
                const a = ctx.actx, now = a.currentTime, s = ctx.settings;
                ctx.masterBus.gain.setTargetAtTime(ctx.G.muted ? 0 : (s.vol ?? 30) / 100 * 0.8, now, 0.015);
                ctx.musicBus.gain.setTargetAtTime((s.musicVol ?? 70) / 100 * duck, now, 0.03);
                ctx.sfxBus.gain.setTargetAtTime((s.sfxVol ?? 85) / 100, now, 0.015);
            }
            if (ctx.GalagaMusic && ctx.GalagaMusic.el) ctx.GalagaMusic.setMuted(ctx.G.muted);
        }
        function release(voice) {
            voice.set.delete(voice);
            for (const node of voice.nodes) try { node.disconnect(); } catch (_) {}
        }
        function claim(nodes, source, end, music) {
            const set = music ? musicVoices : voices, priority = ctx.sfxPriority || 1;
            if (!music && set.size >= 32) {
                const victim = [...set].sort((a, b) => a.priority - b.priority || a.end - b.end)[0];
                if (victim.priority > priority) { nodes.forEach(n => n.disconnect()); return false; }
                victim.sources.forEach(n => { try { n.stop(); } catch (_) {} }); release(victim);
            }
            const voice = { nodes, sources: [source], end, priority, set };
            set.add(voice); source.onended = () => release(voice);
            ctx.audioPeakVoices = Math.max(ctx.audioPeakVoices || 0, voices.size);
            return voice;
        }
        function output(g, panX, nodes, destination) {
            if (Number.isFinite(panX) && ctx.actx.createStereoPanner) {
                const pan = ctx.actx.createStereoPanner(); pan.pan.value = Math.max(-0.8, Math.min(0.8, panX / ctx.W * 1.6 - 0.8));
                nodes.push(pan); g.connect(pan).connect(destination);
            } else g.connect(destination);
        }
        function tone(type, f0, f1, dur, vol, panX, time, destination, fm) {
            const a = audio(); if (!a || ctx.G.muted) return null;
            const start = Math.max(a.currentTime, time ?? ctx.sfxTime ?? a.currentTime), end = start + dur + 0.025;
            const o = a.createOscillator(), g = a.createGain(), filter = a.createBiquadFilter(), nodes = [o, g, filter];
            o.type = type; o.frequency.setValueAtTime(Math.max(20, f0), start); o.frequency.exponentialRampToValueAtTime(Math.max(20, f1), start + dur);
            filter.type = 'lowpass'; filter.frequency.value = Math.min(12000, Math.max(f0 * 5, 1800));
            g.gain.setValueAtTime(0, start); g.gain.linearRampToValueAtTime(Math.min(0.4, vol) * 0.42, start + 0.003);
            g.gain.exponentialRampToValueAtTime(0.0001, end);
            o.connect(filter).connect(g); output(g, panX, nodes, destination || ctx.sfxBus);
            const voice = claim(nodes, o, end, destination === ctx.musicBus); if (!voice) return null;
            if (fm) {
                const mod = a.createOscillator(), depth = a.createGain(); mod.frequency.value = f0 * fm;
                depth.gain.setValueAtTime(f0 * 1.8, start); depth.gain.exponentialRampToValueAtTime(1, end);
                mod.connect(depth).connect(o.frequency); nodes.push(mod, depth); voice.sources.push(mod); mod.start(start); mod.stop(end);
            }
            o.start(start); o.stop(end); return o;
        }
        function schedNoise(time, dur, vol, freq, dest, panX) {
            const a = audio(); if (!a || ctx.G.muted) return null;
            const start = Math.max(a.currentTime, time ?? ctx.sfxTime ?? a.currentTime), end = start + dur + 0.005;
            const source = a.createBufferSource(), filter = a.createBiquadFilter(), gain = a.createGain(), nodes = [source, filter, gain];
            source.buffer = ctx.noiseBuffer; source.loop = true;
            filter.type = freq > 5000 ? 'highpass' : 'lowpass'; filter.frequency.value = freq || 1500;
            gain.gain.setValueAtTime(0, start); gain.gain.linearRampToValueAtTime(Math.min(0.5, vol) * 0.35, start + 0.002);
            gain.gain.exponentialRampToValueAtTime(0.0001, end);
            source.connect(filter).connect(gain); output(gain, panX, nodes, dest || ctx.sfxBus);
            if (!claim(nodes, source, end, dest === ctx.musicBus)) return null;
            source.start(start); source.stop(end); return source;
        }
        function stopVoices(set) { for (const voice of [...set]) { voice.sources.forEach(n => { try { n.stop(); } catch (_) {} }); release(voice); } }
        ctx.audio = audio; ctx.applyAudioSettings = applyAudioSettings; ctx.synthTone = tone; ctx.schedNoise = schedNoise;
        ctx.beep = (type, f0, f1, dur, vol, pan) => tone(type, f0, f1, dur, vol, pan);
        ctx.noise = (dur, vol, freq, pan) => schedNoise(undefined, dur, vol, freq, undefined, pan);
        ctx.fm = (f0, f1, dur, vol, pan, ratio) => tone('sine', f0, f1, dur, vol, pan, undefined, undefined, ratio || 2);
        // Schedule envelopes immediately on the audio clock; no delayed JS callbacks.
        ctx.scheduleSfx = (fn, delay) => {
            const a = audio(); if (!a || ctx.G.muted) return;
            const previous = ctx.sfxTime; ctx.sfxTime = (previous ?? a.currentTime) + delay / 1000;
            try { fn(); } finally { ctx.sfxTime = previous; }
        };
        ctx.resumeAudio = () => { const a = audio(); if (a && a.state === 'suspended') a.resume().catch(() => {}); };
        ctx.stopMusicVoices = () => stopVoices(musicVoices);
        ctx.disposeAudio = () => { stopVoices(voices); stopVoices(musicVoices); if (ctx.MusicEngine) ctx.MusicEngine.stop(); };
        ctx.audioStats = () => ({ effects: voices.size, music: musicVoices.size, peak: ctx.audioPeakVoices || 0 });
        ctx.pv = () => 0.97 + Math.random() * 0.06; ctx.vv = () => 0.95 + Math.random() * 0.1;
        ctx.duckMusic = (amount, ms) => { duck = Math.max(0.2, 1 - amount); duckUntil = ms; applyAudioSettings(); };
        ctx.updateDuck = ms => { if (duckUntil > 0 && (duckUntil -= ms) <= 0) { duck = 1; applyAudioSettings(); } };
    };
})();
