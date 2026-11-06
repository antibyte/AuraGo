/* Aura Drift - procedural Web Audio engine, effects, spatial rivals and score. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;

    AD.createAudio = function (settings) {
        const Ctor = window.AudioContext || window.webkitAudioContext;
        let ctx = null, master = null, comp = null;
        let engineGain = null, sfxGain = null, musicGain = null;
        let engineNodes = [], noiseBuffer = null, rivalNodes = [];
        let rollGain = null, windGain = null, engineFilter = null;
        let squealOsc = null, squealGain = null, squealFilter = null;
        let surfaceGain = null, surfaceFilter = null, scrapeGain = null;
        let sprayGain = null, sprayFilter = null;
        let turboOsc = null, turboGain = null, whineOsc = null, whineGain = null;
        let reverb = null, reverbSend = null;
        let voices = 0, music = null;
        let lastTick = 0;
        let volume = settings.volume, muted = settings.muted;
        let running = false;
        let lastGear = 1, lastLoad = 0, lastRpm = 0;
        let limiterPhase = 0;
        const rivalDistance = [70, 70, 70];

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

        /* A generated impulse response gives the venues their size: a long,
           bright tail in the harbour, a tight one in the salt flats. */
        function makeImpulse(seconds, decay) {
            const length = Math.max(1, Math.floor(ctx.sampleRate * seconds));
            const buffer = ctx.createBuffer(2, length, ctx.sampleRate);
            for (let ch = 0; ch < 2; ch++) {
                const data = buffer.getChannelData(ch);
                for (let i = 0; i < length; i++) {
                    const t = i / length;
                    data[i] = (Math.random() * 2 - 1) * Math.pow(1 - t, decay);
                }
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

            /* Reverb bus: everything that needs room tone feeds it. */
            reverb = ctx.createConvolver();
            reverb.buffer = makeImpulse(1.9, 2.6);
            reverbSend = ctx.createGain();
            reverbSend.gain.value = 0.0;
            reverbSend.connect(reverb);
            reverb.connect(master);

            noiseBuffer = makeNoise();
            engineFilter = ctx.createBiquadFilter();
            engineFilter.type = 'lowpass'; engineFilter.frequency.value = 900;
            engineFilter.Q.value = 0.9;
            engineFilter.connect(engineGain);

            /* Engine stack: two saws a few cents apart plus a square an octave
               down for body, a quiet upper harmonic for rasp and a sub growl
               that only speaks under load. */
            for (const spec of [
                { type: 'sawtooth', detune: 0, gain: 0.30, ratio: 1 },
                { type: 'sawtooth', detune: -9, gain: 0.18, ratio: 1 },
                { type: 'square', detune: 7, gain: 0.14, ratio: 0.5 },
                { type: 'sawtooth', detune: 3, gain: 0.07, ratio: 2 },
                { type: 'sawtooth', detune: -22, gain: 0.12, ratio: 0.5 }
            ]) {
                const osc = ctx.createOscillator();
                osc.type = spec.type; osc.frequency.value = 60; osc.detune.value = spec.detune;
                const g = ctx.createGain(); g.gain.value = spec.gain;
                osc.connect(g).connect(engineFilter);
                osc.start();
                engineNodes.push({ osc, gain: g, base: spec.gain, ratio: spec.ratio });
            }
            const intake = loopNoise('bandpass', 420, 0.12);
            intake.gain.connect(engineGain);
            engineNodes.push({ source: intake.source, filter: intake.filter, gain: intake.gain, noise: true });
            const exhaust = loopNoise('bandpass', 260, 0.05);
            exhaust.gain.connect(engineGain);
            engineNodes.push({ source: exhaust.source, filter: exhaust.filter, gain: exhaust.gain, noise: true });

            /* Turbo whistle rides on load and revs. */
            turboOsc = ctx.createOscillator();
            turboOsc.type = 'sine'; turboOsc.frequency.value = 2400;
            turboGain = ctx.createGain(); turboGain.gain.value = 0;
            turboOsc.connect(turboGain).connect(master);
            turboOsc.start();

            /* Gear whine: the thin top-end that makes a gearbox sound fast. */
            whineOsc = ctx.createOscillator();
            whineOsc.type = 'triangle'; whineOsc.frequency.value = 900;
            whineGain = ctx.createGain(); whineGain.gain.value = 0;
            whineOsc.connect(whineGain).connect(master);
            whineOsc.start();

            /* Rolling noise under the tyres, separate from the slip squeal. */
            const tyre = loopNoise('bandpass', 1900, 0);
            tyre.gain.connect(master); rollGain = tyre.gain;
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
            const squealAir = loopNoise('bandpass', 3200, 0);
            squealAir.gain.connect(master);
            squealAir.gain.gain.value = 0;

            /* Surface grit: gravel, sand, mud and snow spray under the tyres. */
            const surface = loopNoise('highpass', 1400, 0);
            surface.gain.connect(master);
            surfaceGain = surface.gain; surfaceFilter = surface.filter;

            /* Water spray on wet asphalt. */
            const spray = loopNoise('highpass', 2600, 0);
            spray.gain.connect(master);
            sprayGain = spray.gain; sprayFilter = spray.filter;

            /* Wall scrape layer fed by the impact envelope. */
            const scrape = loopNoise('bandpass', 2600, 0);
            scrape.gain.connect(master); scrapeGain = scrape.gain;

            /* Rival engines: one lean saw voice per opponent with distance
               gain, distance low-pass, Doppler shift and a reverb send, so a
               duel is heard the way it looks. */
            for (let i = 0; i < 3; i++) {
                const osc = ctx.createOscillator();
                osc.type = 'sawtooth'; osc.frequency.value = 70;
                const filter = ctx.createBiquadFilter();
                filter.type = 'lowpass'; filter.frequency.value = 1100;
                const gain = ctx.createGain(); gain.gain.value = 0;
                let pan = null;
                try {
                    if (typeof ctx.createStereoPanner === 'function') {
                        pan = ctx.createStereoPanner(); pan.pan.value = 0;
                    }
                } catch (_) { pan = null; }
                if (pan) osc.connect(filter).connect(gain).connect(pan).connect(master);
                else osc.connect(filter).connect(gain).connect(master);
                gain.connect(reverbSend);
                osc.start();
                rivalNodes.push({ osc, filter, gain, pan });
            }

            if (typeof AD.createMusicEngine === 'function') {
                music = AD.createMusicEngine(ctx, noiseBuffer, musicGain);
            } else {
                music = {
                    setBiome() {}, setAmbience() {}, update() {}, event() {}, stop() {}
                };
            }

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

        function noiseHit(level, duration, freq, type) {
            if (!ctx || muted || voices >= 32) return;
            voices++;
            const now = ctx.currentTime;
            const source = ctx.createBufferSource(); source.buffer = noiseBuffer;
            const filter = ctx.createBiquadFilter();
            filter.type = type || 'bandpass'; filter.frequency.value = freq || 900;
            const gain = ctx.createGain();
            gain.gain.setValueAtTime(level, now);
            gain.gain.exponentialRampToValueAtTime(0.0001, now + duration);
            source.connect(filter).connect(gain).connect(sfxGain);
            source.start(now); source.stop(now + duration + 0.02);
            source.onended = () => { voices = Math.max(0, voices - 1); source.disconnect(); filter.disconnect(); gain.disconnect(); };
        }

        /* Layered impact: low body thump plus a noise crack and, on heavy
           hits, a ringing piece of metal. */
        function impact(heavy) {
            tone(heavy ? 88 : 120, heavy ? 0.26 : 0.14, 'sine', heavy ? 0.22 : 0.12, heavy ? 42 : 70);
            noiseHit(heavy ? 0.3 : 0.15, heavy ? 0.34 : 0.16, heavy ? 320 : 520);
            if (heavy) {
                noiseHit(0.12, 0.5, 1800);
                tone(1480, 0.5, 'triangle', 0.05, 900);
            }
        }

        function gearShift() {
            tone(210, 0.05, 'square', 0.07, 150);
            noiseHit(0.05, 0.05, 2400);
        }

        function backfire() {
            noiseHit(0.16, 0.12, 380);
            tone(140, 0.09, 'triangle', 0.1, 80);
        }

        /* Blow-off valve: the turbo dumps its pressure when the throttle
           closes. It is the sound of a car that was working hard. */
        function blowOff() {
            if (!ctx || muted) return;
            const now = ctx.currentTime;
            const source = ctx.createBufferSource(); source.buffer = noiseBuffer;
            const filter = ctx.createBiquadFilter();
            filter.type = 'bandpass'; filter.frequency.setValueAtTime(4200, now);
            filter.frequency.exponentialRampToValueAtTime(1400, now + 0.22);
            filter.Q.value = 1.4;
            const gain = ctx.createGain();
            gain.gain.setValueAtTime(0.11, now);
            gain.gain.exponentialRampToValueAtTime(0.0001, now + 0.26);
            source.connect(filter).connect(gain).connect(sfxGain);
            source.start(now); source.stop(now + 0.3);
            source.onended = () => { source.disconnect(); filter.disconnect(); gain.disconnect(); };
        }

        function ui(kind) {
            if (!ctx || muted) return;
            if (kind === 'confirm') { tone(560, 0.09, 'triangle', 0.10, 780); tone(840, 0.12, 'sine', 0.05, 1120); }
            else if (kind === 'back') { tone(420, 0.10, 'triangle', 0.08, 260); }
            else if (kind === 'move') { tone(700, 0.04, 'sine', 0.05); }
            else if (kind === 'start') { tone(330, 0.16, 'sawtooth', 0.09, 660); noiseHit(0.06, 0.2, 1800); }
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
                case 'sector': tone(evt.index === 0 ? 520 : 740, 0.09, 'sine', 0.09, evt.index === 0 ? 640 : 880); break;
                case 'respawn': tone(280, 0.18, 'triangle', 0.12, 520); break;
                case 'drift_start':
                    /* Only the player's own slides get a cue; the AI drifts
                       silently so the mix stays readable. */
                    if (evt.car === 0) noiseHit(0.05, 0.18, 2600, 'bandpass');
                    break;
                case 'near_miss':
                    if (evt.car !== 0) break;
                    noiseHit(0.09, 0.26, 1400, 'bandpass');
                    tone(1180, 0.16, 'sine', 0.05, 700);
                    break;
                case 'style_bank': {
                    if (evt.car !== 0) break;
                    const base = 660 + Math.min(8, evt.multiplier || 1) * 60;
                    tone(base, 0.12, 'triangle', 0.07, base * 1.5);
                    tone(base * 1.5, 0.16, 'sine', 0.05, base * 2);
                    break;
                }
                case 'finish':
                    [523, 659, 784, 1046].forEach((f, i) => setTimeout(() => {
                        tone(f, 0.32, 'triangle', 0.15);
                        tone(f * 1.5, 0.3, 'sine', 0.07);
                    }, i * 130));
                    break;
            }
            if (music) music.event(evt);
        }

        /* Per-surface tyre character: asphalt hums, gravel rattles, snow
           crunches, wet asphalt sprays and ice is almost silent. */
        function surfaceProfile(surface) {
            switch (surface) {
                case 'gravel': return { freq: 1500, type: 'bandpass', level: 1.0, q: 1.2 };
                case 'sand': return { freq: 420, type: 'lowpass', level: 1.15, q: 0.8 };
                case 'mud': return { freq: 700, type: 'bandpass', level: 0.85, q: 2.2 };
                case 'snow': return { freq: 950, type: 'bandpass', level: 0.7, q: 1.4 };
                case 'ice': return { freq: 2600, type: 'highpass', level: 0.25, q: 0.7 };
                case 'wet': return { freq: 2200, type: 'highpass', level: 0.8, q: 0.7 };
                case 'salt': return { freq: 1800, type: 'bandpass', level: 0.55, q: 1.0 };
                default: return { freq: 420, type: 'bandpass', level: 0.5, q: 0.9 };
            }
        }

        return {
            unlock,
            available() { return !!Ctor; },
            ui,
            setSettings(next) {
                volume = clamp(next.volume, 0, 1); muted = !!next.muted;
                if (master && ctx) master.gain.setTargetAtTime(muted ? 0 : volume * 0.72, ctx.currentTime, 0.03);
            },
            setBiome(id) {
                if (music) music.setBiome(id);
                if (reverbSend && ctx) {
                    /* Big open venues ring longer than a tight street circuit. */
                    const sizes = { harbour: 0.34, industry: 0.30, metro: 0.26, glacier: 0.22, alpine: 0.16, forest: 0.12, salt: 0.06, desert: 0.06 };
                    reverbSend.gain.setTargetAtTime(sizes[id] !== undefined ? sizes[id] : 0.15, ctx.currentTime, 0.4);
                }
            },
            update(race, active) {
                if (!ctx || !running) return;
                const v = race.cars[0].v;
                const rpm = clamp(v.rpm / 7200, 0, 1);
                const load = clamp(v.throttle, 0, 1);
                const speed = Math.max(0, v.vx);
                const now = ctx.currentTime;
                /* The real frame time drives the Doppler shift and the score
                   scheduler, so they stay correct at any frame rate. */
                const tick = clamp(now - lastTick, 0.004, 0.1);
                lastTick = now;

                /* Gear shift clicks and throttle-lift backfires. */
                if (v.gear !== lastGear && speed > 2) { gearShift(); lastGear = v.gear; }
                if (lastLoad > 0.55 && load < 0.1 && rpm > 0.55 && speed > 12) {
                    backfire();
                    if (rpm > 0.7) blowOff();
                }
                lastLoad = load;

                /* Rev limiter: the engine bounces off the cut instead of
                   running flat out. */
                let limiter = 1;
                if (rpm > 0.965) {
                    limiterPhase += 0.35;
                    limiter = (limiterPhase % 1) > 0.5 ? 0.45 : 1;
                }

                for (const node of engineNodes) {
                    if (node.noise) {
                        node.filter.frequency.setTargetAtTime(300 + rpm * 1500, now, 0.05);
                        node.gain.gain.setTargetAtTime(0.03 + load * 0.12, now, 0.06);
                    } else {
                        node.osc.frequency.setTargetAtTime((48 + rpm * 145) * node.ratio, now, 0.04);
                        node.gain.gain.setTargetAtTime(node.base * (0.55 + load * 0.6) * limiter, now, 0.05);
                    }
                }
                engineFilter.frequency.setTargetAtTime(420 + rpm * 2600 + load * 500, now, 0.05);
                engineGain.gain.setTargetAtTime(active ? (0.16 + load * 0.10) * limiter : 0.02, now, 0.08);
                turboGain.gain.setTargetAtTime(
                    active ? clamp((rpm - 0.42) * 0.5, 0, 1) * load * 0.028 : 0, now, 0.08);
                turboOsc.frequency.setTargetAtTime(1900 + rpm * 2400, now, 0.09);
                whineGain.gain.setTargetAtTime(
                    active ? clamp(rpm - 0.45, 0, 1) * 0.014 * (v.gear || 1) : 0, now, 0.06);
                whineOsc.frequency.setTargetAtTime(600 + rpm * 2600, now, 0.06);
                lastRpm = rpm;

                /* Tyre layers: rolling noise, slip squeal and surface grit. */
                const surface = v.surface || 'asphalt';
                const profile = surfaceProfile(surface);
                rollGain.gain.setTargetAtTime(
                    clamp(speed / 90, 0, 1) * 0.05 * profile.level * (active ? 1 : 0), now, 0.06);
                const squeal = clamp((v.slip - 1.6) * 0.05, 0, 0.22) * (active ? 1 : 0);
                squealGain.gain.setTargetAtTime(squeal * (surface === 'ice' ? 1.4 : 1), now, 0.04);
                squealOsc.frequency.setTargetAtTime(520 + v.slip * 60, now, 0.06);
                squealFilter.frequency.setTargetAtTime(1500 + v.slip * 40, now, 0.05);
                surfaceGain.gain.setTargetAtTime(
                    clamp(speed / 80, 0, 1) * 0.09 * profile.level * (active ? 1 : 0), now, 0.08);
                surfaceFilter.type = profile.type;
                surfaceFilter.frequency.setTargetAtTime(profile.freq, now, 0.1);
                if (surfaceFilter.Q) surfaceFilter.Q.setTargetAtTime(profile.q, now, 0.1);
                const wet = surface === 'wet';
                sprayGain.gain.setTargetAtTime(wet ? clamp(speed / 70, 0, 1) * 0.05 * (active ? 1 : 0) : 0, now, 0.1);
                sprayFilter.frequency.setTargetAtTime(1800 + speed * 20, now, 0.1);
                scrapeGain.gain.setTargetAtTime(clamp(v.impact * 0.16, 0, 0.16), now, 0.03);
                const draftWind = clamp((v.slipstream || 0), 0, 1) * 0.03;
                windGain.gain.setTargetAtTime((clamp(speed / 90, 0, 1) * 0.055 + draftWind) * (active ? 1 : 0), now, 0.1);

                /* Rival engines: distance gain, distance low-pass, Doppler
                   shift and stereo pan. */
                const cos = Math.cos(v.psi), sin = Math.sin(v.psi);
                for (let i = 0; i < rivalNodes.length; i++) {
                    const node = rivalNodes[i];
                    const rival = race.cars[i + 1];
                    if (!rival || !active) {
                        node.gain.gain.setTargetAtTime(0, now, 0.1);
                        rivalDistance[i] = 70;
                        continue;
                    }
                    const dx = rival.v.x - v.x, dy = rival.v.y - v.y;
                    const dist = Math.hypot(dx, dy);
                    const lateral = (-dx * sin + dy * cos) / Math.max(1, dist);
                    const rrpm = clamp(rival.v.rpm / 7200, 0, 1);
                    const level = clamp(1 - dist / 70, 0, 1);
                    /* Doppler: the rate at which the distance changes bends
                       the pitch exactly like a real engine passing by. */
                    const rate = (dist - rivalDistance[i]) / tick;
                    rivalDistance[i] = dist;
                    const doppler = clamp(1 / (1 + rate / 343), 0.82, 1.22);
                    node.osc.frequency.setTargetAtTime((48 + rrpm * 145) * doppler, now, 0.05);
                    node.filter.frequency.setTargetAtTime(
                        380 + level * 2200 + rrpm * 600, now, 0.08);
                    node.gain.gain.setTargetAtTime(level * level * 0.13, now, 0.1);
                    if (node.pan) node.pan.pan.setTargetAtTime(clamp(lateral, -1, 1) * 0.8, now, 0.1);
                }

                const conditions = race.conditions ? race.conditions.state : null;
                const intensity = clamp(speed / 70, 0, 1);
                if (music) {
                    music.update(tick, intensity, conditions);
                    if (conditions && conditions.ambience) music.setAmbience(conditions.ambience);
                }
            },
            event,
            suspend() { if (ctx && ctx.state === 'running') ctx.suspend().catch(() => {}); running = false; },
            resume() { if (ctx && ctx.state === 'suspended') ctx.resume().catch(() => {}); running = true; },
            dispose() {
                running = false;
                if (music) music.stop();
                for (const node of engineNodes) {
                    try { if (node.osc) node.osc.stop(); if (node.source) node.source.stop(); } catch (_) { /* already stopped */ }
                }
                engineNodes = [];
                for (const node of rivalNodes) {
                    try { if (node.osc) node.osc.stop(); } catch (_) { /* already stopped */ }
                    try { if (node.gain) node.gain.disconnect(); } catch (_) { /* already gone */ }
                    try { if (node.pan) node.pan.disconnect(); } catch (_) { /* already gone */ }
                }
                rivalNodes = [];
                for (const osc of [squealOsc, turboOsc, whineOsc]) {
                    try { if (osc) osc.stop(); } catch (_) { /* already stopped */ }
                }
                if (ctx) { try { ctx.close(); } catch (_) { /* already closed */ } }
                ctx = null; master = null; comp = null; reverb = null; reverbSend = null;
                squealOsc = null; turboOsc = null; whineOsc = null;
                lastGear = 1; lastLoad = 0; lastRpm = 0;
            }
        };
    };
})();
