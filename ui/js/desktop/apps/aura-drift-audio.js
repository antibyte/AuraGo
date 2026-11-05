/* Aura Drift - procedural Web Audio engine, effects and adaptive music. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;

    const SCALES = {
        alpine: [0, 3, 5, 7, 10], desert: [0, 2, 5, 7, 9],
        harbour: [0, 3, 7, 10, 12], salt: [0, 2, 4, 7, 9],
        forest: [0, 2, 5, 7, 11], industry: [0, 1, 5, 7, 8]
    };

    AD.createAudio = function (settings) {
        const Ctor = window.AudioContext || window.webkitAudioContext;
        let ctx = null, master = null, comp = null;
        let engineGain = null, sfxGain = null, musicGain = null;
        let engineNodes = [], noiseBuffer = null;
        let tyreGain = null, windGain = null, engineFilter = null;
        let voices = 0, nextNote = 0, noteIndex = 0, musicKey = 'alpine';
        let volume = settings.volume, muted = settings.muted;
        let running = false;

        function makeNoise() {
            const buffer = ctx.createBuffer(1, ctx.sampleRate, ctx.sampleRate);
            const data = buffer.getChannelData(0);
            let seed = 123456789;
            for (let i = 0; i < data.length; i++) {
                seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0;
                data[i] = (seed / 2147483648) - 1;
            }
            return buffer;
        }

        function loopNoise(filterType, freq, gainValue) {
            const source = ctx.createBufferSource();
            source.buffer = noiseBuffer; source.loop = true;
            const filter = ctx.createBiquadFilter();
            filter.type = filterType; filter.frequency.value = freq;
            const gain = ctx.createGain(); gain.gain.value = gainValue;
            source.connect(filter).connect(gain);
            source.start();
            return { source, filter, gain };
        }

        function unlock() {
            if (!Ctor) return false;
            if (ctx) { if (ctx.state === 'suspended') ctx.resume().catch(() => {}); return true; }
            try { ctx = new Ctor(); } catch (_) { return false; }
            comp = ctx.createDynamicsCompressor();
            comp.threshold.value = -14; comp.knee.value = 12;
            comp.ratio.value = 8; comp.attack.value = 0.004; comp.release.value = 0.16;
            master = ctx.createGain();
            master.gain.value = muted ? 0 : volume * 0.72;
            master.connect(comp).connect(ctx.destination);

            engineGain = ctx.createGain(); engineGain.gain.value = 0.0; engineGain.connect(master);
            sfxGain = ctx.createGain(); sfxGain.gain.value = 0.9; sfxGain.connect(master);
            musicGain = ctx.createGain(); musicGain.gain.value = 0.34; musicGain.connect(master);

            noiseBuffer = makeNoise();
            engineFilter = ctx.createBiquadFilter();
            engineFilter.type = 'lowpass'; engineFilter.frequency.value = 900;
            engineFilter.Q.value = 0.9;
            engineFilter.connect(engineGain);

            for (const spec of [
                { type: 'sawtooth', detune: 0, gain: 0.34 },
                { type: 'square', detune: 7, gain: 0.16 },
                { type: 'sawtooth', detune: -11, gain: 0.12 }
            ]) {
                const osc = ctx.createOscillator();
                osc.type = spec.type; osc.frequency.value = 60; osc.detune.value = spec.detune;
                const g = ctx.createGain(); g.gain.value = spec.gain;
                osc.connect(g).connect(engineFilter);
                osc.start();
                engineNodes.push({ osc, g, base: spec.gain });
            }
            const intake = loopNoise('bandpass', 420, 0.12);
            intake.gain.connect(engineGain);
            engineNodes.push({ source: intake.source, filter: intake.filter, gain: intake.gain, noise: true });

            const tyre = loopNoise('bandpass', 1900, 0);
            tyre.gain.connect(master); tyreGain = tyre.gain;
            const wind = loopNoise('highpass', 900, 0);
            wind.gain.connect(master); windGain = wind.gain;

            nextNote = ctx.currentTime + 0.1;
            running = true;
            return true;
        }

        function tone(freq, duration, type, level, sweep) {
            if (!ctx || muted || voices >= 32) return;
            voices++;
            const now = ctx.currentTime;
            const osc = ctx.createOscillator(), gain = ctx.createGain();
            osc.type = type || 'triangle';
            osc.frequency.setValueAtTime(freq, now);
            if (sweep) osc.frequency.exponentialRampToValueAtTime(Math.max(30, sweep), now + duration);
            gain.gain.setValueAtTime(0.0001, now);
            gain.gain.exponentialRampToValueAtTime(Math.max(0.001, level), now + 0.008);
            gain.gain.exponentialRampToValueAtTime(0.0001, now + duration);
            osc.connect(gain).connect(sfxGain);
            osc.start(now); osc.stop(now + duration + 0.02);
            osc.onended = () => { voices = Math.max(0, voices - 1); osc.disconnect(); gain.disconnect(); };
        }

        function noiseHit(level, duration, freq) {
            if (!ctx || muted || voices >= 32) return;
            voices++;
            const now = ctx.currentTime;
            const source = ctx.createBufferSource(); source.buffer = noiseBuffer;
            const filter = ctx.createBiquadFilter(); filter.type = 'bandpass'; filter.frequency.value = freq || 900;
            const gain = ctx.createGain();
            gain.gain.setValueAtTime(level, now);
            gain.gain.exponentialRampToValueAtTime(0.0001, now + duration);
            source.connect(filter).connect(gain).connect(sfxGain);
            source.start(now); source.stop(now + duration + 0.02);
            source.onended = () => { voices = Math.max(0, voices - 1); source.disconnect(); filter.disconnect(); gain.disconnect(); };
        }

        function event(evt) {
            if (!ctx || muted) return;
            switch (evt.type) {
                case 'count': tone(evt.value > 0 ? 440 : 660, 0.14, 'sine', 0.16); break;
                case 'go': tone(880, 0.32, 'sine', 0.22, 1180); break;
                case 'pickup': tone(720, 0.12, 'sine', 0.14, 1180); break;
                case 'weapon':
                    if (evt.id === 'mg') tone(180, 0.08, 'square', 0.10, 90);
                    else if (evt.id === 'shield') tone(520, 0.3, 'sine', 0.14, 900);
                    else noiseHit(0.16, 0.22, evt.id === 'emp' ? 2200 : 620);
                    break;
                case 'hit': noiseHit(evt.heavy ? 0.32 : 0.16, evt.heavy ? 0.36 : 0.18, 350); break;
                case 'emp_pulse': tone(120, 0.4, 'sawtooth', 0.13, 60); break;
                case 'lap': tone(660, 0.16, 'sine', 0.16, 990); break;
                case 'respawn': tone(280, 0.18, 'triangle', 0.12, 520); break;
                case 'finish': [523, 659, 784, 1046].forEach((f, i) => setTimeout(() => tone(f, 0.3, 'triangle', 0.16), i * 120)); break;
            }
        }

        function scheduleMusic(intensity) {
            if (!ctx || muted || !running) return;
            const beat = 60 / 112 / 2;
            const scale = SCALES[musicKey] || SCALES.alpine;
            while (nextNote < ctx.currentTime + 0.12) {
                const step = noteIndex % 16;
                const root = 110;
                if (step % 4 === 0 || (intensity > 0.6 && step % 2 === 0)) {
                    const semi = scale[(noteIndex + Math.floor(step / 4)) % scale.length];
                    const freq = root * Math.pow(2, semi / 12);
                    const osc = ctx.createOscillator(), gain = ctx.createGain();
                    osc.type = step % 8 === 0 ? 'triangle' : 'sine';
                    osc.frequency.value = freq;
                    gain.gain.setValueAtTime(0.0001, nextNote);
                    gain.gain.exponentialRampToValueAtTime(0.09 + intensity * 0.05, nextNote + 0.02);
                    gain.gain.exponentialRampToValueAtTime(0.0001, nextNote + beat * 1.6);
                    osc.connect(gain).connect(musicGain);
                    osc.start(nextNote); osc.stop(nextNote + beat * 1.8);
                    osc.onended = () => { osc.disconnect(); gain.disconnect(); };
                }
                nextNote += beat;
                noteIndex++;
            }
        }

        return {
            unlock,
            available() { return !!Ctor; },
            setSettings(next) {
                volume = clamp(next.volume, 0, 1); muted = !!next.muted;
                if (master && ctx) master.gain.setTargetAtTime(muted ? 0 : volume * 0.72, ctx.currentTime, 0.03);
            },
            setBiome(id) { musicKey = Object.prototype.hasOwnProperty.call(SCALES, id) ? id : 'alpine'; noteIndex = 0; if (ctx) nextNote = ctx.currentTime + 0.1; },
            update(race, active) {
                if (!ctx || !running) return;
                const v = race.cars[0].v;
                const rpm = clamp(v.rpm / 7200, 0, 1);
                const load = clamp(v.throttle, 0, 1);
                const speed = Math.max(0, v.vx);
                const now = ctx.currentTime;
                for (const node of engineNodes) {
                    if (node.noise) {
                        node.filter.frequency.setTargetAtTime(300 + rpm * 1500, now, 0.05);
                        node.gain.gain.setTargetAtTime(0.03 + load * 0.12, now, 0.06);
                    } else {
                        node.osc.frequency.setTargetAtTime(48 + rpm * 145, now, 0.04);
                        node.g.gain.setTargetAtTime(node.base * (0.55 + load * 0.6), now, 0.05);
                    }
                }
                engineFilter.frequency.setTargetAtTime(420 + rpm * 2600 + load * 500, now, 0.05);
                engineGain.gain.setTargetAtTime(active ? 0.16 + load * 0.10 : 0.02, now, 0.08);
                tyreGain.gain.setTargetAtTime(clamp(v.slip * 0.014, 0, 0.16) * (active ? 1 : 0), now, 0.05);
                windGain.gain.setTargetAtTime(clamp(speed / 90, 0, 1) * 0.055 * (active ? 1 : 0), now, 0.1);
                scheduleMusic(clamp(speed / 70, 0, 1));
            },
            event,
            suspend() { if (ctx && ctx.state === 'running') ctx.suspend().catch(() => {}); running = false; },
            resume() { if (ctx && ctx.state === 'suspended') ctx.resume().catch(() => {}); running = true; if (ctx) nextNote = ctx.currentTime + 0.1; },
            dispose() {
                running = false;
                for (const node of engineNodes) {
                    try { if (node.osc) node.osc.stop(); if (node.source) node.source.stop(); } catch (_) { /* already stopped */ }
                }
                engineNodes = [];
                if (ctx) { try { ctx.close(); } catch (_) { /* already closed */ } }
                ctx = null; master = null; comp = null;
            }
        };
    };
})();
