/* Aura Drift - adaptive score and biome ambience, all synthesised offline. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;

    const SCALES = {
        alpine: [0, 3, 5, 7, 10], desert: [0, 2, 5, 7, 9],
        harbour: [0, 3, 7, 10, 12], salt: [0, 2, 4, 7, 9],
        forest: [0, 2, 5, 7, 11], industry: [0, 1, 5, 7, 8],
        glacier: [0, 2, 3, 7, 10], metro: [0, 2, 4, 7, 9]
    };
    const TEMPI = {
        alpine: 100, desert: 118, harbour: 92,
        salt: 106, forest: 96, industry: 128,
        glacier: 84, metro: 122
    };
    const PROGRESSION = [0, 3, 1, 4];

    /* Every biome plays a different arrangement of the same sixteen-step grid:
       the drums, the bass rhythm and the lead voice all change. */
    const PATTERNS = {
        alpine: { kick: [0, 8], snare: [4, 12], hat: [1, 3, 5, 7, 9, 11, 13, 15], bass: [0, 6, 8, 14], lead: [2, 10], pad: true },
        desert: { kick: [0, 6, 8, 14], snare: [4, 12], hat: [2, 6, 10, 14], bass: [0, 3, 8, 11], lead: [5, 13], pad: true },
        harbour: { kick: [0, 3, 8, 11], snare: [4, 12], hat: [2, 6, 10, 14], bass: [0, 8], lead: [7, 15], pad: true },
        salt: { kick: [0, 4, 8, 12], snare: [4, 12], hat: [2, 6, 10, 14], bass: [0, 4, 8, 12], lead: [3, 11], pad: true },
        forest: { kick: [0, 6, 10], snare: [4, 12], hat: [1, 5, 9, 13], bass: [0, 5, 10], lead: [2, 7, 12], pad: false },
        industry: { kick: [0, 4, 8, 12], snare: [4, 12], hat: [2, 6, 10, 14], bass: [0, 2, 8, 10], lead: [6, 14], pad: true },
        glacier: { kick: [0, 10], snare: [6], hat: [4, 12], bass: [0, 8], lead: [7], pad: true },
        metro: { kick: [0, 4, 8, 12], snare: [4, 12], hat: [2, 6, 10, 14], bass: [0, 3, 8, 11], lead: [5, 13], pad: true }
    };

    AD.createMusicEngine = function (ctx, noiseBuffer, destination) {
        let musicKey = 'alpine';
        let nextNote = ctx.currentTime + 0.1;
        let noteIndex = 0;
        let running = true;
        let ambience = { wind: 0.4, birds: 0, machinery: 0, water: 0, crowd: 0, reverb: 0.1 };
        let bedGain = null;
        const beds = {};
        let birdTimer = 2;
        let ambienceClock = 0;

        function noiseSource(buffer) {
            const source = ctx.createBufferSource();
            source.buffer = buffer;
            source.loop = true;
            source.start();
            return source;
        }

        function bed(filterType, freq, q) {
            const source = noiseSource(noiseBuffer);
            const filter = ctx.createBiquadFilter();
            filter.type = filterType; filter.frequency.value = freq;
            if (q) filter.Q.value = q;
            const gain = ctx.createGain(); gain.gain.value = 0;
            source.connect(filter).connect(gain).connect(destination);
            beds[filterType + ':' + freq] = { source, filter, gain };
            return beds[filterType + ':' + freq];
        }

        /* Ambience beds: wind, water, machinery and crowd. Birds and metal
           clanks are one-shots fired from the update loop. */
        function buildBeds() {
            bed('highpass', 700, 0.6);       /* wind */
            bed('lowpass', 420, 0.8);        /* water */
            bed('bandpass', 180, 1.6);       /* machinery */
            bed('bandpass', 900, 0.7);       /* crowd */
        }
        buildBeds();

        function noteFreq(scale, degree, root) {
            const steps = scale.length;
            const octave = Math.floor(degree / steps);
            const semi = scale[((degree % steps) + steps) % steps];
            return root * Math.pow(2, (semi + octave * 12) / 12);
        }

        function kick(time, level) {
            const osc = ctx.createOscillator(), gain = ctx.createGain();
            osc.type = 'sine';
            osc.frequency.setValueAtTime(128, time);
            osc.frequency.exponentialRampToValueAtTime(44, time + 0.11);
            gain.gain.setValueAtTime(level, time);
            gain.gain.exponentialRampToValueAtTime(0.0001, time + 0.16);
            osc.connect(gain).connect(destination);
            osc.start(time); osc.stop(time + 0.18);
            osc.onended = () => { osc.disconnect(); gain.disconnect(); };
        }

        function hat(time, open) {
            const source = ctx.createBufferSource(); source.buffer = noiseBuffer;
            const filter = ctx.createBiquadFilter(); filter.type = 'highpass'; filter.frequency.value = 6800;
            const gain = ctx.createGain();
            gain.gain.setValueAtTime(open ? 0.12 : 0.07, time);
            gain.gain.exponentialRampToValueAtTime(0.0001, time + (open ? 0.12 : 0.045));
            source.connect(filter).connect(gain).connect(destination);
            source.start(time); source.stop(time + 0.16);
            source.onended = () => { source.disconnect(); filter.disconnect(); gain.disconnect(); };
        }

        function snare(time, level) {
            const source = ctx.createBufferSource(); source.buffer = noiseBuffer;
            const filter = ctx.createBiquadFilter(); filter.type = 'bandpass'; filter.frequency.value = 2100; filter.Q.value = 0.8;
            const gain = ctx.createGain();
            gain.gain.setValueAtTime(level, time);
            gain.gain.exponentialRampToValueAtTime(0.0001, time + 0.11);
            source.connect(filter).connect(gain).connect(destination);
            source.start(time); source.stop(time + 0.13);
            source.onended = () => { source.disconnect(); filter.disconnect(); gain.disconnect(); };
            const osc = ctx.createOscillator(), og = ctx.createGain();
            osc.type = 'triangle'; osc.frequency.value = 190;
            og.gain.setValueAtTime(level * 0.7, time);
            og.gain.exponentialRampToValueAtTime(0.0001, time + 0.08);
            osc.connect(og).connect(destination);
            osc.start(time); osc.stop(time + 0.1);
            osc.onended = () => { osc.disconnect(); og.disconnect(); };
        }

        function bass(time, freq, dur, level) {
            const osc = ctx.createOscillator(), gain = ctx.createGain(), filter = ctx.createBiquadFilter();
            osc.type = musicKey === 'forest' ? 'triangle' : 'sawtooth';
            osc.frequency.value = freq;
            filter.type = 'lowpass'; filter.frequency.value = 420;
            gain.gain.setValueAtTime(0.0001, time);
            gain.gain.exponentialRampToValueAtTime(level, time + 0.012);
            gain.gain.exponentialRampToValueAtTime(0.0001, time + dur);
            osc.connect(filter).connect(gain).connect(destination);
            osc.start(time); osc.stop(time + dur + 0.03);
            osc.onended = () => { osc.disconnect(); gain.disconnect(); filter.disconnect(); };
        }

        function pluck(time, freq, level) {
            const osc = ctx.createOscillator(), gain = ctx.createGain();
            osc.type = 'triangle'; osc.frequency.value = freq;
            gain.gain.setValueAtTime(0.0001, time);
            gain.gain.exponentialRampToValueAtTime(level, time + 0.01);
            gain.gain.exponentialRampToValueAtTime(0.0001, time + 0.22);
            osc.connect(gain).connect(destination);
            osc.start(time); osc.stop(time + 0.25);
            osc.onended = () => { osc.disconnect(); gain.disconnect(); };
        }

        function bell(time, freq, level) {
            const osc = ctx.createOscillator(), gain = ctx.createGain();
            osc.type = 'sine'; osc.frequency.value = freq;
            gain.gain.setValueAtTime(0.0001, time);
            gain.gain.exponentialRampToValueAtTime(level, time + 0.006);
            gain.gain.exponentialRampToValueAtTime(0.0001, time + 1.1);
            osc.connect(gain).connect(destination);
            osc.start(time); osc.stop(time + 1.2);
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
                    osc.connect(filter).connect(gain).connect(destination);
                    osc.start(time); osc.stop(time + dur + 0.05);
                    osc.onended = () => { osc.disconnect(); gain.disconnect(); filter.disconnect(); };
                }
            }
        }

        /* --- ambience one-shots ------------------------------------------- */
        function birdCall(time) {
            const osc = ctx.createOscillator(), gain = ctx.createGain();
            osc.type = 'sine';
            const base = 2400 + Math.random() * 900;
            osc.frequency.setValueAtTime(base, time);
            osc.frequency.exponentialRampToValueAtTime(base * 1.5, time + 0.05);
            osc.frequency.exponentialRampToValueAtTime(base * 0.8, time + 0.12);
            gain.gain.setValueAtTime(0.0001, time);
            gain.gain.exponentialRampToValueAtTime(0.05, time + 0.01);
            gain.gain.exponentialRampToValueAtTime(0.0001, time + 0.16);
            osc.connect(gain).connect(destination);
            osc.start(time); osc.stop(time + 0.2);
            osc.onended = () => { osc.disconnect(); gain.disconnect(); };
        }

        function clank(time) {
            const source = ctx.createBufferSource(); source.buffer = noiseBuffer;
            const filter = ctx.createBiquadFilter(); filter.type = 'bandpass';
            filter.frequency.value = 2600 + Math.random() * 900; filter.Q.value = 5;
            const gain = ctx.createGain();
            gain.gain.setValueAtTime(0.07, time);
            gain.gain.exponentialRampToValueAtTime(0.0001, time + 0.26);
            source.connect(filter).connect(gain).connect(destination);
            source.start(time); source.stop(time + 0.3);
            source.onended = () => { source.disconnect(); filter.disconnect(); gain.disconnect(); };
        }

        function gull(time) {
            const osc = ctx.createOscillator(), gain = ctx.createGain();
            osc.type = 'sawtooth';
            osc.frequency.setValueAtTime(900, time);
            osc.frequency.exponentialRampToValueAtTime(1500, time + 0.07);
            osc.frequency.exponentialRampToValueAtTime(800, time + 0.22);
            const filter = ctx.createBiquadFilter(); filter.type = 'bandpass';
            filter.frequency.value = 1600; filter.Q.value = 3;
            gain.gain.setValueAtTime(0.0001, time);
            gain.gain.exponentialRampToValueAtTime(0.045, time + 0.02);
            gain.gain.exponentialRampToValueAtTime(0.0001, time + 0.26);
            osc.connect(filter).connect(gain).connect(destination);
            osc.start(time); osc.stop(time + 0.3);
            osc.onended = () => { osc.disconnect(); gain.disconnect(); filter.disconnect(); };
        }

        function scheduleMusic(intensity) {
            if (!running) return;
            const tempo = TEMPI[musicKey] || 104;
            const beat = 60 / tempo / 2;
            const scale = SCALES[musicKey] || SCALES.alpine;
            const pattern = PATTERNS[musicKey] || PATTERNS.alpine;
            const root = musicKey === 'industry' ? 49 : 55;
            while (nextNote < ctx.currentTime + 0.12) {
                const step = noteIndex % 16;
                const bar = Math.floor(noteIndex / 16);
                const degree = PROGRESSION[bar % PROGRESSION.length];

                if (pattern.kick.indexOf(step) >= 0) kick(nextNote, 0.5);
                if (pattern.snare.indexOf(step) >= 0 && intensity > 0.3) {
                    snare(nextNote, 0.10 + intensity * 0.09);
                }
                if (pattern.hat.indexOf(step) >= 0 && intensity > 0.25) {
                    hat(nextNote, step % 8 === 7);
                }
                if (pattern.bass.indexOf(step) >= 0) {
                    bass(nextNote, noteFreq(scale, degree, root), beat * 1.7, 0.15 + intensity * 0.08);
                    if (intensity > 0.55 && step % 4 === 2) {
                        bass(nextNote, noteFreq(scale, degree + 2, root * 2), beat * 0.8, 0.06);
                    }
                }
                if (pattern.lead.indexOf(step) >= 0 && intensity > 0.42) {
                    if (musicKey === 'glacier') {
                        bell(nextNote, noteFreq(scale, degree + 4, root * 8), 0.05 + intensity * 0.05);
                    } else {
                        pluck(nextNote, noteFreq(scale, degree + 2 + (step % 3), root * 8), 0.045 + intensity * 0.05);
                    }
                }
                if (intensity > 0.68 && pattern.pad && step === 0) {
                    pad(nextNote, [
                        noteFreq(scale, degree, root * 4),
                        noteFreq(scale, degree + 2, root * 4),
                        noteFreq(scale, degree + 4, root * 4)
                    ], beat * 8);
                }
                nextNote += beat;
                noteIndex++;
            }
        }

        function updateAmbience(dt, conditions) {
            const mix = ambience;
            /* The rare one-shots (birds, gulls, distant clanks) run on their
               own clock, independent of the bed level throttle. */
            birdTimer -= dt;
            if (birdTimer <= 0) {
                birdTimer = 2.5 + Math.random() * 6;
                const at = ctx.currentTime;
                if ((mix.birds || 0) > 0.2) birdCall(at + Math.random() * 0.2);
                else if ((mix.water || 0) > 0.4 && Math.random() > 0.5) gull(at);
                else if ((mix.machinery || 0) > 0.4 && Math.random() > 0.6) clank(at);
            }
            /* The bed levels move slowly, so they are re-targeted a few times
               per second instead of every frame. */
            ambienceClock -= dt;
            if (ambienceClock > 0) return;
            ambienceClock = 0.25;
            const now = ctx.currentTime;
            const windLevel = clamp((mix.wind || 0) * 0.05 + (conditions && conditions.wind ? conditions.wind * 0.03 : 0), 0, 0.12);
            const windBed = beds['highpass:700'];
            const waterBed = beds['lowpass:420'];
            const machineBed = beds['bandpass:180'];
            const crowdBed = beds['bandpass:900'];
            if (windBed) windBed.gain.gain.setTargetAtTime(windLevel, now, 0.4);
            if (windBed) windBed.filter.frequency.setTargetAtTime(600 + (conditions ? conditions.intensity * 500 : 0), now, 0.6);
            if (waterBed) waterBed.gain.gain.setTargetAtTime((mix.water || 0) * 0.035, now, 0.6);
            if (machineBed) machineBed.gain.gain.setTargetAtTime((mix.machinery || 0) * 0.045, now, 0.6);
            if (crowdBed) crowdBed.gain.gain.setTargetAtTime((mix.crowd || 0) * 0.03, now, 0.8);
        }

        return {
            setBiome(id) {
                musicKey = Object.prototype.hasOwnProperty.call(SCALES, id) ? id : 'alpine';
                noteIndex = 0;
                nextNote = ctx.currentTime + 0.1;
                const profile = AD.conditionProfile ? AD.conditionProfile(musicKey) : null;
                if (profile && profile.ambience) ambience = profile.ambience;
            },
            setAmbience(next) { if (next) ambience = next; },
            update(dt, intensity, conditions) {
                if (!running) return;
                scheduleMusic(clamp(intensity, 0, 1));
                updateAmbience(dt, conditions);
            },
            /* The score reacts to the race: pickups and laps get their sting. */
            event(evt) {
                if (!running || !evt) return;
                const now = ctx.currentTime;
                if (evt.type === 'pickup') {
                    pluck(now, 720, 0.05); pluck(now + 0.07, 1080, 0.04);
                } else if (evt.type === 'lap') {
                    bell(now, 660, 0.06); bell(now + 0.12, 990, 0.05);
                } else if (evt.type === 'finish') {
                    [523, 659, 784, 1046].forEach((f, i) => bell(now + i * 0.13, f, 0.07));
                }
            },
            stop() {
                running = false;
                for (const key of Object.keys(beds)) {
                    const item = beds[key];
                    try { item.source.stop(); } catch (_) { /* already stopped */ }
                    try { item.gain.disconnect(); } catch (_) { /* already gone */ }
                    delete beds[key];
                }
            }
        };
    };
})();
