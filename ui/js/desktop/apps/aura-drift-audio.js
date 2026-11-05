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
    const TEMPI = {
        alpine: 100, desert: 118, harbour: 92,
        salt: 106, forest: 96, industry: 128
    };
    /* One chord walk per bar; degrees index into the biome scale. */
    const PROGRESSION = [0, 3, 1, 4];

    AD.createAudio = function (settings) {
        const Ctor = window.AudioContext || window.webkitAudioContext;
        let ctx = null, master = null, comp = null;
        let engineGain = null, sfxGain = null, musicGain = null;
        let engineNodes = [], noiseBuffer = null;
        let tyreGain = null, windGain = null, engineFilter = null;
        let squealOsc = null, squealGain = null, squealFilter = null;
        let surfaceGain = null, surfaceFilter = null, scrapeGain = null;
        let turboOsc = null, turboGain = null;
        let voices = 0, nextNote = 0, noteIndex = 0, musicKey = 'alpine';
        let volume = settings.volume, muted = settings.muted;
        let running = false;
        let lastGear = 1, lastLoad = 0;

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
            musicGain = ctx.createGain(); musicGain.gain.value = 0.32; musicGain.connect(master);

            noiseBuffer = makeNoise();
            engineFilter = ctx.createBiquadFilter();
            engineFilter.type = 'lowpass'; engineFilter.frequency.value = 900;
            engineFilter.Q.value = 0.9;
            engineFilter.connect(engineGain);

            /* Engine stack: two saws a few cents apart plus a square an octave
               down for body, and a quiet upper harmonic for rasp. */
            for (const spec of [
                { type: 'sawtooth', detune: 0, gain: 0.30, ratio: 1 },
                { type: 'sawtooth', detune: -9, gain: 0.18, ratio: 1 },
                { type: 'square', detune: 7, gain: 0.14, ratio: 0.5 },
                { type: 'sawtooth', detune: 3, gain: 0.07, ratio: 2 }
            ]) {
                const osc = ctx.createOscillator();
                osc.type = spec.type; osc.frequency.value = 60; osc.detune.value = spec.detune;
                const g = ctx.createGain(); g.gain.value = spec.gain;
                osc.connect(g).connect(engineFilter);
                osc.start();
                engineNodes.push({ osc, g, base: spec.gain, ratio: spec.ratio });
            }
            const intake = loopNoise('bandpass', 420, 0.12);
            intake.gain.connect(engineGain);
            engineNodes.push({ source: intake.source, filter: intake.filter, gain: intake.gain, noise: true });

            /* Turbo whistle rides on load and revs. */
            turboOsc = ctx.createOscillator();
            turboOsc.type = 'sine'; turboOsc.frequency.value = 2400;
            turboGain = ctx.createGain(); turboGain.gain.value = 0;
            turboOsc.connect(turboGain).connect(master);
            turboOsc.start();

            const tyre = loopNoise('bandpass', 1900, 0);
            tyre.gain.connect(master); tyreGain = tyre.gain;
            const wind = loopNoise('highpass', 900, 0);
            wind.gain.connect(master); windGain = wind.gain;

            /* Slip squeal: narrow resonant band driven by the slip angle. */
            squealOsc = ctx.createOscillator();
            squealOsc.type = 'sawtooth'; squealOsc.frequency.value = 620;
            squealFilter = ctx.createBiquadFilter();
            squealFilter.type = 'bandpass'; squealFilter.frequency.value = 1650; squealFilter.Q.value = 6;
            squealGain = ctx.createGain(); squealGain.gain.value = 0;
            squealOsc.connect(squealFilter).connect(squealGain).connect(master);
            squealOsc.start();

            /* Surface grit: gravel, sand and mud spray under the tyres. */
            const surface = loopNoise('highpass', 1400, 0);
            surface.gain.connect(master);
            surfaceGain = surface.gain; surfaceFilter = surface.filter;

            /* Wall scrape layer fed by the impact envelope. */
            const scrape = loopNoise('bandpass', 2600, 0);
            scrape.gain.connect(master); scrapeGain = scrape.gain;

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

        /* Layered impact: low body thump plus a noise crack. */
        function impact(heavy) {
            tone(heavy ? 88 : 120, heavy ? 0.26 : 0.14, 'sine', heavy ? 0.22 : 0.12, heavy ? 42 : 70);
            noiseHit(heavy ? 0.3 : 0.15, heavy ? 0.34 : 0.16, heavy ? 320 : 520);
            if (heavy) noiseHit(0.12, 0.5, 1800);
        }

        function gearShift() {
            tone(210, 0.05, 'square', 0.07, 150);
            noiseHit(0.05, 0.05, 2400);
        }

        function backfire() {
            noiseHit(0.16, 0.12, 380);
            tone(140, 0.09, 'triangle', 0.1, 80);
        }

        function event(evt) {
            if (!ctx || muted) return;
            switch (evt.type) {
                case 'count': tone(evt.value > 0 ? 440 : 660, 0.14, 'sine', 0.16); break;
                case 'go': tone(880, 0.32, 'sine', 0.22, 1180); tone(1320, 0.28, 'sine', 0.1); break;
                case 'pickup': tone(720, 0.12, 'sine', 0.14, 1180); tone(1080, 0.16, 'triangle', 0.08, 1560); break;
                case 'weapon':
                    if (evt.id === 'mg') tone(180, 0.08, 'square', 0.10, 90);
                    else if (evt.id === 'shield') { tone(520, 0.3, 'sine', 0.14, 900); tone(780, 0.34, 'triangle', 0.08, 1170); }
                    else noiseHit(0.16, 0.22, evt.id === 'emp' ? 2200 : 620);
                    break;
                case 'hit': impact(!!evt.heavy); break;
                case 'emp_pulse': tone(120, 0.4, 'sawtooth', 0.13, 60); tone(240, 0.32, 'sine', 0.08, 90); break;
                case 'lap': tone(660, 0.16, 'sine', 0.16, 990); tone(990, 0.2, 'triangle', 0.08, 1320); break;
                case 'respawn': tone(280, 0.18, 'triangle', 0.12, 520); break;
                case 'finish':
                    [523, 659, 784, 1046].forEach((f, i) => setTimeout(() => {
                        tone(f, 0.32, 'triangle', 0.15);
                        tone(f * 1.5, 0.3, 'sine', 0.07);
                    }, i * 130));
                    break;
            }
        }

        /* --- adaptive music ------------------------------------------------ */
        function noteFreq(scale, degree, root) {
            const steps = scale.length;
            const octave = Math.floor(degree / steps);
            const semi = scale[((degree % steps) + steps) % steps];
            return root * Math.pow(2, (semi + octave * 12) / 12);
        }

        function kick(time) {
            const osc = ctx.createOscillator(), gain = ctx.createGain();
            osc.type = 'sine';
            osc.frequency.setValueAtTime(128, time);
            osc.frequency.exponentialRampToValueAtTime(44, time + 0.11);
            gain.gain.setValueAtTime(0.5, time);
            gain.gain.exponentialRampToValueAtTime(0.0001, time + 0.16);
            osc.connect(gain).connect(musicGain);
            osc.start(time); osc.stop(time + 0.18);
            osc.onended = () => { osc.disconnect(); gain.disconnect(); };
        }

        function hat(time, open) {
            const source = ctx.createBufferSource(); source.buffer = noiseBuffer;
            const filter = ctx.createBiquadFilter(); filter.type = 'highpass'; filter.frequency.value = 6800;
            const gain = ctx.createGain();
            gain.gain.setValueAtTime(open ? 0.12 : 0.07, time);
            gain.gain.exponentialRampToValueAtTime(0.0001, time + (open ? 0.12 : 0.045));
            source.connect(filter).connect(gain).connect(musicGain);
            source.start(time); source.stop(time + 0.16);
            source.onended = () => { source.disconnect(); filter.disconnect(); gain.disconnect(); };
        }

        function bass(time, freq, dur, level) {
            const osc = ctx.createOscillator(), gain = ctx.createGain(), filter = ctx.createBiquadFilter();
            osc.type = 'sawtooth'; osc.frequency.value = freq;
            filter.type = 'lowpass'; filter.frequency.value = 420;
            gain.gain.setValueAtTime(0.0001, time);
            gain.gain.exponentialRampToValueAtTime(level, time + 0.012);
            gain.gain.exponentialRampToValueAtTime(0.0001, time + dur);
            osc.connect(filter).connect(gain).connect(musicGain);
            osc.start(time); osc.stop(time + dur + 0.03);
            osc.onended = () => { osc.disconnect(); filter.disconnect(); gain.disconnect(); };
        }

        function pluck(time, freq, level) {
            const osc = ctx.createOscillator(), gain = ctx.createGain();
            osc.type = 'triangle'; osc.frequency.value = freq;
            gain.gain.setValueAtTime(0.0001, time);
            gain.gain.exponentialRampToValueAtTime(level, time + 0.01);
            gain.gain.exponentialRampToValueAtTime(0.0001, time + 0.22);
            osc.connect(gain).connect(musicGain);
            osc.start(time); osc.stop(time + 0.25);
            osc.onended = () => { osc.disconnect(); gain.disconnect(); };
        }

        function pad(time, freqs, dur) {
            for (const freq of freqs) {
                for (const detune of [-6, 6]) {
                    const osc = ctx.createOscillator(), gain = ctx.createGain(), filter = ctx.createBiquadFilter();
                    osc.type = 'sawtooth'; osc.frequency.value = freq; osc.detune.value = detune;
                    filter.type = 'lowpass'; filter.frequency.value = 900;
                    gain.gain.setValueAtTime(0.0001, time);
                    gain.gain.exponentialRampToValueAtTime(0.028, time + dur * 0.3);
                    gain.gain.exponentialRampToValueAtTime(0.0001, time + dur);
                    osc.connect(filter).connect(gain).connect(musicGain);
                    osc.start(time); osc.stop(time + dur + 0.05);
                    osc.onended = () => { osc.disconnect(); filter.disconnect(); gain.disconnect(); };
                }
            }
        }

        function scheduleMusic(intensity) {
            if (!ctx || muted || !running) return;
            const tempo = TEMPI[musicKey] || 104;
            const beat = 60 / tempo / 2;
            const scale = SCALES[musicKey] || SCALES.alpine;
            const root = 55;
            while (nextNote < ctx.currentTime + 0.12) {
                const step = noteIndex % 16;
                const bar = Math.floor(noteIndex / 16);
                const degree = PROGRESSION[bar % PROGRESSION.length];
                if (step === 0) {
                    kick(nextNote);
                    pad(nextNote, [
                        noteFreq(scale, degree, root * 4),
                        noteFreq(scale, degree + 2, root * 4),
                        noteFreq(scale, degree + 4, root * 4)
                    ], beat * 8);
                }
                if (step === 8 || (intensity > 0.72 && step === 6)) kick(nextNote);
                if (step % 2 === 1 && intensity > 0.3) hat(nextNote, step % 8 === 7);
                if (step % 4 === 0 || step === 14) {
                    bass(nextNote, noteFreq(scale, degree, root), beat * 1.7, 0.16 + intensity * 0.08);
                }
                if (intensity > 0.45 && (step === 2 || step === 5 || step === 10 || step === 13)) {
                    pluck(nextNote, noteFreq(scale, degree + 2 + (step % 3), root * 8), 0.05 + intensity * 0.05);
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
            setBiome(id) {
                musicKey = Object.prototype.hasOwnProperty.call(SCALES, id) ? id : 'alpine';
                noteIndex = 0;
                if (ctx) nextNote = ctx.currentTime + 0.1;
            },
            update(race, active) {
                if (!ctx || !running) return;
                const v = race.cars[0].v;
                const rpm = clamp(v.rpm / 7200, 0, 1);
                const load = clamp(v.throttle, 0, 1);
                const speed = Math.max(0, v.vx);
                const now = ctx.currentTime;

                /* Gear shift clicks and throttle-lift backfires. */
                if (v.gear !== lastGear && speed > 2) { gearShift(); lastGear = v.gear; }
                if (lastLoad > 0.55 && load < 0.1 && rpm > 0.55 && speed > 12) backfire();
                lastLoad = load;

                for (const node of engineNodes) {
                    if (node.noise) {
                        node.filter.frequency.setTargetAtTime(300 + rpm * 1500, now, 0.05);
                        node.gain.gain.setTargetAtTime(0.03 + load * 0.12, now, 0.06);
                    } else {
                        node.osc.frequency.setTargetAtTime((48 + rpm * 145) * node.ratio, now, 0.04);
                        node.g.gain.setTargetAtTime(node.base * (0.55 + load * 0.6), now, 0.05);
                    }
                }
                engineFilter.frequency.setTargetAtTime(420 + rpm * 2600 + load * 500, now, 0.05);
                engineGain.gain.setTargetAtTime(active ? 0.16 + load * 0.10 : 0.02, now, 0.08);
                turboGain.gain.setTargetAtTime(
                    active ? clamp((rpm - 0.42) * 0.5, 0, 1) * load * 0.028 : 0, now, 0.08);
                turboOsc.frequency.setTargetAtTime(1900 + rpm * 2400, now, 0.09);

                /* Tyre layers: rolling noise, slip squeal and surface grit. */
                tyreGain.gain.setTargetAtTime(clamp(v.slip * 0.014, 0, 0.16) * (active ? 1 : 0), now, 0.05);
                const squeal = clamp((v.slip - 1.6) * 0.05, 0, 0.22) * (active ? 1 : 0);
                squealGain.gain.setTargetAtTime(squeal, now, 0.04);
                squealOsc.frequency.setTargetAtTime(520 + v.slip * 60, now, 0.06);
                const gritty = v.surface === 'gravel' || v.surface === 'sand' || v.surface === 'mud';
                surfaceGain.gain.setTargetAtTime(
                    gritty ? clamp(speed / 80, 0, 1) * 0.09 * (active ? 1 : 0) : 0, now, 0.08);
                surfaceFilter.frequency.setTargetAtTime(
                    v.surface === 'mud' ? 900 : (v.surface === 'sand' ? 2600 : 1900), now, 0.1);
                scrapeGain.gain.setTargetAtTime(clamp(v.impact * 0.16, 0, 0.16), now, 0.03);
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
                for (const osc of [squealOsc, turboOsc]) {
                    try { if (osc) osc.stop(); } catch (_) { /* already stopped */ }
                }
                if (ctx) { try { ctx.close(); } catch (_) { /* already closed */ } }
                ctx = null; master = null; comp = null;
                squealOsc = null; turboOsc = null; lastGear = 1; lastLoad = 0;
            }
        };
    };
})();
