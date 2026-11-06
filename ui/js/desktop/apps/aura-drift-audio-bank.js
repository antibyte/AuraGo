/* Aura Drift - audio sample bank.
 *
 * Loads the OGG pack lazily and by priority, decodes it, plays it with a 32-voice cap and falls back
 * to small live recipes (AD.SYNTH_FALLBACK, AD.SYNTH_LOOPS) when a group is missing, still loading or
 * failed. The game never waits on audio: play() answers immediately, either with the sample or with
 * the recipe, and a group that is not loaded yet is queued for next time. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const ROOT = '/3d/aura-drift/audio/';
    const MAX_VOICES = 32;
    const CONCURRENCY = 3;
    const clamp = AD.clamp;

    /* Per-group trim in dB. The pack is peak-normalised, so loudness differs by up to 20 dB between
       groups (measured as active RMS: explosions -7, tonal stings -5..-8 against impacts -19). The
       trims aim every group at a hierarchy of RMS levels at the bus: explosions -12..-14, impacts
       -15..-17, weapons and comic stingers -17..-18, props and animals -19..-21, tyres and loops
       -23..-27, ambience -32. Boosts are capped at +4 dB; the master limiter catches peaks. */
    AD.GROUP_GAIN_DB = {
        'impact.light': 2.6, 'impact.heavy': 4, 'impact.metal': 3.8, 'scrape.metal': -2.5, 'tyre.squeal': -6.2, 'explosion.small': 2.5, 'explosion.big': -2, 'crate.break': 3.4,
        'prop.hay': -1.7, 'prop.cone': 2.4, 'prop.barrel': -1.4, 'prop.tyres': -7.5, 'crowd.cheer': -5.2, 'crowd.aww': -9.5,
        'animal.cow': -8.7, 'animal.sheep': -7.4, 'animal.penguin': -5.5, 'animal.llama': -9, 'animal.crab': -1.5,
        'amb.wind': -17.5, 'amb.sea': -10, 'amb.forest': -14.1, 'amb.machinery': -8.1, 'amb.city': -18, 'amb.snow': -18, 'amb.desert': -16.4,
        'weapon.oil': -3.5, 'weapon.mine_drop': -1.5, 'weapon.emp_charge': -13.5, 'weapon.emp_pulse': -5.5, 'weapon.rocket_fire': -10.5, 'weapon.rocket_lock': -17.5,
        'weapon.mg': -3, 'weapon.shield_on': -4, 'weapon.shield_hit': 1, 'weapon.duck_launch': -0.5, 'weapon.duck_quack': -1.5, 'weapon.rail_charge': -14.5,
        'weapon.rail_fire': -5, 'weapon.flame_loop': -14, 'weapon.frost_throw': -3.5, 'weapon.frost_burst': 0.5, 'weapon.horn_blast': -8.5, 'weapon.hole_loop': -15,
        'weapon.disco_on': -4, 'weapon.glove': -6.5, 'weapon.harpoon_fire': 2.5, 'weapon.harpoon_clank': 3, 'weapon.cow_launch': -10, 'weapon.cow_land': -6,
        'weapon.cloak_on': -1.5, 'weapon.overdrive_on': -7.5,
        'sting.perfect': 0.5, 'sting.pancake': -1, 'sting.strike': -3.5, 'sting.slingshot': 1.5, 'sting.moo': -5.5, 'sting.trick': 0.5,
        'turbo.1': -6, 'turbo.2': -7.5, 'turbo.3': -8, 'turbo.tick': -9, 'pad.boost': -9.5, 'ramp.launch': -6.5, 'land.soft': -6.5, 'land.hard': -4.5,
        'roulette.tick': -6, 'roulette.done': -5.5, 'bomb.tick': -7.5, 'bomb.blast': -7, 'jingle.softeis': -7,
        'horn.vektor': -8, 'horn.kobalt': -10, 'horn.lumen': -9.5, 'horn.tundra': -10.5, 'horn.softeis': -3, 'horn.maehkoenig': -5, 'horn.couch': -10, 'horn.bertha': -8.5,
        'ufo.beam': -11.5, 'ui.count': -11, 'ui.go': -5.5, 'ui.lap': -6.5, 'ui.bank': -6.5, 'whoosh': -9,
        'stinger.final_lap': -3, 'stinger.disco': 0.5, 'fanfare.win': -4, 'fanfare.podium': -3.5, 'fanfare.lose': -7,
        'sting.near_miss': -7.5, 'sting.style_bank': -7, 'sting.drift_link': -9, 'sting.cloak': -8, 'sting.sector': -7.5
    };

    /* Live recipes. 't' = tone(freq, dur, type, level, sweep, delay); 'n' = noise(level, dur, freq, filter, delay, q). */
    const T = (f, d, ty, l, sw, dl) => ['t', f, d, ty || 'triangle', l, sw || 0, dl || 0];
    const N = (l, d, f, ty, dl, q) => ['n', l, d, f, ty || 'bandpass', dl || 0, q || 1];
    const run = (list, step) => list.map((f, i) => T(f, 0.3, 'triangle', 0.14, 0, i * step));
    const HORN = [T(440, 0.4, 'square', 0.08), T(550, 0.4, 'square', 0.07)];
    const BOOM = [N(0.4, 0.8, 350, 'lowpass'), T(70, 0.7, 'sine', 0.3, 30), N(0.15, 0.5, 1500)];
    AD.SYNTH_FALLBACK = {
        'impact.light': [T(120, 0.14, 'sine', 0.12, 70), N(0.15, 0.16, 520)],
        'impact.heavy': [T(88, 0.26, 'sine', 0.22, 42), N(0.3, 0.34, 320), N(0.12, 0.5, 1800), T(1480, 0.5, 'triangle', 0.05, 900)],
        'impact.metal': [N(0.2, 0.3, 2400, 'bandpass', 0, 4), T(1900, 0.4, 'triangle', 0.06, 1200)],
        'explosion.small': [N(0.3, 0.35, 500, 'lowpass'), T(100, 0.3, 'sine', 0.2, 45)],
        'explosion.big': BOOM,
        'crate.break': [N(0.2, 0.18, 1800), T(720, 0.1, 'square', 0.06, 300)],
        'prop.hay': [N(0.18, 0.25, 900, 'lowpass')],
        'prop.cone': [T(500, 0.08, 'triangle', 0.12, 300), N(0.08, 0.08, 2200)],
        'prop.barrel': [T(150, 0.2, 'sine', 0.16, 90), N(0.12, 0.15, 700)],
        'prop.tyres': [T(110, 0.18, 'sine', 0.14, 70), N(0.1, 0.2, 400, 'lowpass')],
        'crowd.cheer': [N(0.12, 1.0, 1600, 'bandpass', 0, 0.6)],
        'crowd.aww': [T(520, 0.6, 'triangle', 0.06, 300), N(0.05, 0.6, 900)],
        'animal.cow': [T(110, 0.7, 'sawtooth', 0.1, 85)],
        'animal.sheep': [T(420, 0.5, 'sawtooth', 0.07, 360)],
        'animal.penguin': [T(900, 0.2, 'square', 0.05, 700), T(800, 0.2, 'square', 0.05, 650, 0.25)],
        'animal.llama': [T(300, 0.5, 'sawtooth', 0.08, 450)],
        'animal.crab': [N(0.06, 0.08, 3200), N(0.06, 0.08, 3200, 'bandpass', 0.12)],
        'weapon.oil': [N(0.15, 0.3, 500, 'lowpass')],
        'weapon.mine_drop': [T(200, 0.12, 'triangle', 0.12, 120)],
        'weapon.emp_charge': [T(200, 0.8, 'sawtooth', 0.08, 1600)],
        'weapon.emp_pulse': [T(120, 0.4, 'sawtooth', 0.13, 60), T(240, 0.32, 'sine', 0.08, 90)],
        'weapon.rocket_fire': [N(0.25, 0.5, 900), T(150, 0.4, 'sawtooth', 0.1, 60)],
        'weapon.rocket_lock': [T(1400, 0.08, 'square', 0.06), T(1400, 0.08, 'square', 0.06, 0, 0.15)],
        'weapon.mg': [T(180, 0.08, 'square', 0.1, 90)],
        'weapon.shield_on': [T(520, 0.3, 'sine', 0.14, 900), T(780, 0.34, 'triangle', 0.08, 1170)],
        'weapon.shield_hit': [T(900, 0.15, 'triangle', 0.1, 500), N(0.1, 0.1, 2600)],
        'weapon.duck_launch': [T(500, 0.1, 'square', 0.08, 900)],
        'weapon.duck_quack': [T(520, 0.12, 'sawtooth', 0.08, 380), T(480, 0.12, 'sawtooth', 0.08, 350, 0.15)],
        'weapon.rail_charge': [T(200, 0.7, 'sawtooth', 0.1, 2400)],
        'weapon.rail_fire': [N(0.3, 0.35, 3000, 'highpass'), T(90, 0.3, 'sine', 0.2, 40)],
        'weapon.frost_throw': [N(0.12, 0.25, 2800, 'highpass')],
        'weapon.frost_burst': [N(0.2, 0.5, 3500, 'highpass'), T(1800, 0.4, 'triangle', 0.05, 900)],
        'weapon.horn_blast': [T(380, 0.6, 'sawtooth', 0.12), T(480, 0.6, 'sawtooth', 0.1)],
        'weapon.disco_on': [T(660, 0.15, 'square', 0.07), T(880, 0.15, 'square', 0.07, 0, 0.15), T(1100, 0.2, 'square', 0.07, 0, 0.3)],
        'weapon.glove': [T(300, 0.12, 'sine', 0.14, 150), N(0.1, 0.1, 600)],
        'weapon.harpoon_fire': [N(0.15, 0.2, 2000, 'highpass'), T(300, 0.15, 'sawtooth', 0.08, 120)],
        'weapon.harpoon_clank': [T(1100, 0.25, 'triangle', 0.1, 800), N(0.12, 0.1, 3000)],
        'weapon.cow_launch': [T(120, 0.9, 'sawtooth', 0.12, 80)],
        'weapon.cow_land': [T(70, 0.4, 'sine', 0.22, 35), N(0.2, 0.3, 300, 'lowpass')],
        'weapon.cloak_on': [N(0.08, 0.5, 3000, 'bandpass', 0, 0.6)],
        'weapon.overdrive_on': [T(220, 0.6, 'sawtooth', 0.1, 880), N(0.14, 0.5, 1500)],
        'sting.perfect': run([784, 1046], 0.12),
        'sting.pancake': [T(300, 0.25, 'sine', 0.12, 80)],
        'sting.strike': run([523, 659, 784], 0.12),
        'sting.slingshot': [T(300, 0.4, 'sawtooth', 0.08, 1200)],
        'sting.moo': [T(110, 0.7, 'sawtooth', 0.1, 85)],
        'sting.trick': [T(880, 0.12, 'sine', 0.1, 1320), T(1320, 0.14, 'sine', 0.07, 1760, 0.08)],
        'turbo.1': [N(0.1, 0.25, 2400), T(500, 0.2, 'sawtooth', 0.06, 900)],
        'turbo.2': [N(0.12, 0.3, 2600), T(600, 0.25, 'sawtooth', 0.07, 1200)],
        'turbo.3': [N(0.14, 0.4, 2800), T(700, 0.3, 'sawtooth', 0.08, 1600)],
        'turbo.tick': [T(1200, 0.04, 'sine', 0.06)],
        'pad.boost': [T(300, 0.35, 'sawtooth', 0.1, 1200)],
        'ramp.launch': [N(0.12, 0.35, 1400), T(200, 0.3, 'sine', 0.1, 500)],
        'land.soft': [T(110, 0.14, 'sine', 0.12, 70)],
        'land.hard': [T(88, 0.26, 'sine', 0.2, 42), N(0.2, 0.3, 400)],
        'roulette.tick': [T(1500, 0.03, 'square', 0.05)],
        'roulette.done': [T(880, 0.12, 'triangle', 0.1), T(1320, 0.2, 'triangle', 0.1, 0, 0.1)],
        'bomb.tick': [T(1800, 0.05, 'square', 0.08)],
        'bomb.blast': BOOM,
        'jingle.softeis': run([784, 659, 784, 523, 659, 587], 0.22),
        'ufo.beam': [T(300, 1.2, 'sine', 0.08, 900)],
        'ui.count': [T(440, 0.14, 'sine', 0.16)],
        'ui.go': [T(880, 0.32, 'sine', 0.22, 1180), T(1320, 0.28, 'sine', 0.1)],
        'ui.lap': [T(660, 0.16, 'sine', 0.16, 990), T(990, 0.2, 'triangle', 0.08, 1320)],
        'ui.bank': [T(700, 0.12, 'triangle', 0.07, 1050)],
        'whoosh': [N(0.09, 0.26, 1400)],
        'stinger.final_lap': run([523, 784, 1046], 0.18),
        'stinger.disco': run([660, 880, 1100, 1320], 0.14),
        'fanfare.win': run([523, 659, 784, 1046], 0.13),
        'fanfare.podium': run([523, 659, 784], 0.13),
        'fanfare.lose': [T(330, 0.3, 'triangle', 0.12, 260), T(247, 0.5, 'triangle', 0.12, 200, 0.3)],
        'sting.near_miss': [N(0.12, 0.3, 1400), T(900, 0.3, 'sine', 0.06, 400)],
        'sting.style_bank': run([784, 988, 1175, 1568], 0.07),
        'sting.drift_link': [T(640, 0.12, 'triangle', 0.08, 220), N(0.05, 0.05, 2400)],
        'sting.cloak': [N(0.08, 0.6, 2200, 'bandpass', 0, 0.8), T(520, 0.5, 'sine', 0.06, 700)],
        'sting.sector': run([659, 988], 0.1)
    };
    for (const id of ['vektor', 'kobalt', 'lumen', 'tundra', 'softeis', 'maehkoenig', 'couch', 'bertha']) AD.SYNTH_FALLBACK['horn.' + id] = HORN;

    /* Live loops: filtered noise and/or one oscillator. level is the gain at handle gain 1. */
    AD.SYNTH_LOOPS = {
        'tyre.squeal': { osc: ['sawtooth', 620], filter: ['bandpass', 1650, 6], level: 0.12 },
        'scrape.metal': { filter: ['bandpass', 2600, 1.5], level: 0.1 },
        'weapon.flame_loop': { filter: ['bandpass', 700, 0.8], level: 0.12 },
        'weapon.hole_loop': { osc: ['sine', 60], filter: ['lowpass', 300, 0.7], level: 0.12 },
        'amb.wind': { filter: ['bandpass', 500, 0.4], level: 0.05 },
        'amb.sea': { filter: ['lowpass', 400, 0.5], level: 0.06 },
        'amb.forest': { filter: ['bandpass', 1800, 0.5], level: 0.03 },
        'amb.machinery': { osc: ['sawtooth', 50], filter: ['lowpass', 180, 0.7], level: 0.05 },
        'amb.city': { filter: ['lowpass', 320, 0.5], level: 0.05 },
        'amb.snow': { filter: ['highpass', 900, 0.5], level: 0.025 },
        'amb.desert': { filter: ['bandpass', 380, 0.5], level: 0.05 }
    };

    AD.createSampleBank = function (ctx, bus, options) {
        const opts = options || {};
        const conn = typeof navigator !== 'undefined' ? navigator.connection : null;
        const lazy = !!(opts.lazy || (conn && conn.saveData));
        const buses = bus && typeof bus.connect === 'function' ? { sfx: bus } : (bus || {});
        const st = { asked: 0, loaded: 0, failed: 0, fallback: 0, dropped: 0, voices: 0, manifestFailed: false };
        const buffers = new Map();          // path -> AudioBuffer | null (failed)
        const jobs = new Map();             // path -> Promise
        const requested = new Set();        // groups already queued by play()
        const queue = [];
        const handles = new Set();
        let running = 0, manifest = null, manifestP = null, disposed = false;

        const noise = (function () {
            const buffer = ctx.createBuffer(1, ctx.sampleRate, ctx.sampleRate);
            const data = buffer.getChannelData(0);
            let seed = 123456789;
            for (let i = 0; i < data.length; i++) {
                seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0;
                data[i] = (seed / 2147483648) - 1;
            }
            return buffer;
        })();

        function decode(bytes) {
            return new Promise((resolve, reject) => {
                const p = ctx.decodeAudioData(bytes, resolve, reject);
                if (p && typeof p.then === 'function') p.then(resolve, reject);
            });
        }

        function pump() {
            while (running < CONCURRENCY && queue.length && !disposed) {
                const job = queue.shift();
                running++;
                job.run().then(() => { running--; pump(); }, () => { running--; pump(); });
            }
        }
        function enqueue(prio, run) {
            let i = queue.length;
            while (i > 0 && queue[i - 1].prio > prio) i--;
            queue.splice(i, 0, { prio: prio, run: run });
            pump();
        }

        function fetchBuffer(path, prio) {
            if (buffers.has(path)) return Promise.resolve(buffers.get(path));
            if (jobs.has(path)) return jobs.get(path);
            if (!AD.assets || typeof AD.assets.fetchBytes !== 'function') { buffers.set(path, null); st.failed++; return Promise.resolve(null); }
            st.asked++;
            const promise = new Promise(resolve => {
                enqueue(prio === undefined ? 50 : prio, () => AD.assets.fetchBytes(ROOT + path).then(decode).then(buffer => {
                    if (disposed) { resolve(null); return; }
                    buffers.set(path, buffer); st.loaded++; resolve(buffer);
                }).catch(() => { buffers.set(path, null); st.failed++; resolve(null); }));
            });
            jobs.set(path, promise);
            return promise;
        }

        function loadManifest() {
            if (manifest) return Promise.resolve(manifest);
            if (!manifestP) {
                if (!AD.assets || typeof AD.assets.fetchJson !== 'function') { st.manifestFailed = true; return Promise.resolve(null); }
                manifestP = AD.assets.fetchJson(ROOT + 'manifest.json').then(m => {
                    if (!disposed && m && m.version === 1) manifest = m; else st.manifestFailed = true;
                    return manifest;
                }).catch(() => { st.failed++; st.manifestFailed = true; return null; });
            }
            return manifestP;
        }

        function load(groups, prio) {
            return loadManifest().then(m => {
                if (!m) return 0;
                const all = [];
                for (const g of groups) for (const p of ((m.sfx && m.sfx[g]) || [])) all.push(fetchBuffer(p, prio));
                return Promise.all(all).then(list => list.filter(Boolean).length);
            });
        }
        function loadEngine(carId, layers, prio) {
            return loadManifest().then(m => {
                const set = m && m.engine && m.engine[carId];
                if (!set) return null;
                return Promise.all((layers || ['idle', 'low', 'mid', 'high']).map(l => set[l] ? fetchBuffer(set[l], prio) : Promise.resolve(null)));
            });
        }
        function engineLayers(carId, layers) {
            const set = manifest && manifest.engine && manifest.engine[carId];
            if (!set) return null;
            const out = {};
            for (const l of (layers || ['idle', 'low', 'mid', 'high'])) {
                const b = set[l] ? buffers.get(set[l]) : null;
                if (!b) return null;
                out[l] = b;
            }
            return out;
        }
        function members(group) {
            const list = (manifest && manifest.sfx && manifest.sfx[group]) || [];
            const out = [];
            for (const p of list) { const b = buffers.get(p); if (b) out.push(b); }
            return out;
        }
        function has(group) { return members(group).length > 0; }
        function request(group) {
            if (requested.has(group) || !manifest) { if (!manifest) loadManifest(); return; }
            requested.add(group);
            load([group], 40);
        }

        function destination(group, o) {
            const key = o.bus || (group.indexOf('amb.') === 0 ? 'amb' : 'sfx');
            return buses[key] || buses.sfx || null;
        }
        function gainNode(value, at) {
            const g = ctx.createGain();
            g.gain.value = value;
            return g;
        }

        /* Wires [src] -> optional lowpass -> gain -> optional pan -> bus (+ reverb send). */
        function chain(src, group, o, level, dest) {
            let tail = src, filter = null, pan = null;
            if (o.lowpass) {
                filter = ctx.createBiquadFilter();
                filter.type = 'lowpass'; filter.frequency.value = o.lowpass;
                tail.connect(filter); tail = filter;
            }
            const gain = gainNode(level);
            tail.connect(gain); tail = gain;
            if (typeof ctx.createStereoPanner === 'function' && o.pan) {
                pan = ctx.createStereoPanner(); pan.pan.value = clamp(o.pan, -1, 1);
                tail.connect(pan); tail = pan;
            }
            if (dest) tail.connect(dest);
            let send = null;
            if (o.send && buses.send) { send = gainNode(o.send); tail.connect(send); send.connect(buses.send); }
            return { gain: gain, filter: filter, pan: pan, send: send, tail: tail };
        }

        function trim(group) { return Math.pow(10, (AD.GROUP_GAIN_DB[group] || 0) / 20); }

        function handleFor(parts, level, stopNow, rateParam) {
            const now = () => ctx.currentTime;
            const h = {
                synth: false, ended: false,
                setGain(v, tc) { if (!h.ended) parts.gain.gain.setTargetAtTime(Math.max(0, v) * level, now(), tc || 0.05); },
                setRate(v, tc) { if (!h.ended && rateParam) rateParam(v, tc || 0.05); },
                setPan(v) { if (parts.pan && !h.ended) parts.pan.pan.setTargetAtTime(clamp(v, -1, 1), now(), 0.08); },
                setLowpass(v) { if (parts.filter && !h.ended) parts.filter.frequency.setTargetAtTime(v, now(), 0.08); },
                stop(fade) {
                    if (h.ended) return;
                    h.ended = true;
                    const t = now(), f = fade === undefined ? 0.05 : fade;
                    try { parts.gain.gain.cancelScheduledValues(t); parts.gain.gain.setTargetAtTime(0, t, Math.max(0.005, f / 3)); } catch (_) { /* gone */ }
                    stopNow(t + f + 0.03);
                }
            };
            return h;
        }

        function release(h, nodes) {
            handles.delete(h);
            st.voices = Math.max(0, st.voices - 1);
            for (const n of nodes) { try { n.disconnect(); } catch (_) { /* already gone */ } }
        }

        function playSample(group, buf, o) {
            const src = ctx.createBufferSource();
            src.buffer = buf; src.loop = !!o.loop;
            const rate = (o.rate || 1) * (o.vary ? 1 + (Math.random() - 0.5) * 2 * o.vary : 1);
            src.playbackRate.value = rate;
            const level = trim(group);
            const parts = chain(src, group, o, (o.gain === undefined ? 1 : o.gain) * level, destination(group, o));
            const h = handleFor(parts, level, t => { try { src.stop(t); } catch (_) { /* not started */ } }, (v, tc) => src.playbackRate.setTargetAtTime(v, ctx.currentTime, tc));
            src.onended = () => { h.ended = true; release(h, [src, parts.gain, parts.pan, parts.filter, parts.send].filter(Boolean)); };
            const at = ctx.currentTime + (o.delay || 0);
            src.start(at);
            if (o.dur && !o.loop) h.stopAt = at + o.dur;
            if (o.dur) { try { parts.gain.gain.setTargetAtTime(0, at + o.dur, 0.06); src.stop(at + o.dur + 0.3); } catch (_) { /* ignored */ } }
            st.voices++; handles.add(h);
            return h;
        }

        function oneShot(kind, level, dur, f, type, sweep, q, delay, o) {
            const at = ctx.currentTime + (delay || 0) + (o.delay || 0);
            let src;
            if (kind === 't') {
                src = ctx.createOscillator();
                src.type = type; src.frequency.setValueAtTime(f, at);
                if (sweep) src.frequency.exponentialRampToValueAtTime(Math.max(30, sweep), at + dur);
            } else {
                src = ctx.createBufferSource(); src.buffer = noise;
            }
            let tail = src;
            if (kind === 'n') {
                const filter = ctx.createBiquadFilter();
                filter.type = type; filter.frequency.value = f || 900; filter.Q.value = q || 1;
                src.connect(filter); tail = filter;
            }
            const g = ctx.createGain();
            const peak = Math.max(0.001, level * (o.gain === undefined ? 1 : o.gain));
            g.gain.setValueAtTime(0.0001, at);
            g.gain.exponentialRampToValueAtTime(peak, at + (kind === 't' ? 0.008 : 0.004));
            g.gain.exponentialRampToValueAtTime(0.0001, at + dur);
            tail.connect(g);
            const dest = o.dest || destination('', o);
            let last = g;
            if (typeof ctx.createStereoPanner === 'function' && o.pan) {
                const pan = ctx.createStereoPanner(); pan.pan.value = clamp(o.pan, -1, 1);
                g.connect(pan); last = pan;
            }
            if (dest) last.connect(dest);
            st.voices++;
            src.onended = () => { st.voices = Math.max(0, st.voices - 1); try { src.disconnect(); g.disconnect(); if (last !== g) last.disconnect(); } catch (_) { /* gone */ } };
            src.start(at); src.stop(at + dur + 0.03);
        }
        const tone = (f, d, type, level, sweep, o) => { if (!disposed && st.voices < MAX_VOICES) oneShot('t', level, d, f, type || 'triangle', sweep, 0, 0, o || {}); };
        const noiseHit = (level, d, f, type, o, q) => { if (!disposed && st.voices < MAX_VOICES) oneShot('n', level, d, f, type || 'bandpass', 0, q, 0, o || {}); };

        function playRecipe(group, o) {
            const recipe = AD.SYNTH_FALLBACK[group];
            if (recipe) {
                for (const s of recipe) {
                    if (st.voices >= MAX_VOICES) { st.dropped++; break; }
                    if (s[0] === 't') oneShot('t', s[4], s[2], s[1], s[3], s[5], 0, s[6], o);
                    else oneShot('n', s[1], s[2], s[3], s[4], 0, s[6], s[5], o);
                }
            }
            return { synth: true, ended: false, setGain() {}, setRate() {}, setPan() {}, setLowpass() {}, stop() {} };
        }

        function playLoopRecipe(group, o) {
            const spec = AD.SYNTH_LOOPS[group];
            if (!spec) return { synth: true, ended: false, setGain() {}, setRate() {}, setPan() {}, setLowpass() {}, stop() {} };
            const src = ctx.createBufferSource();
            src.buffer = noise; src.loop = true;
            let tail = src, filter = null, osc = null;
            if (spec.filter) {
                filter = ctx.createBiquadFilter();
                filter.type = spec.filter[0]; filter.frequency.value = spec.filter[1]; filter.Q.value = spec.filter[2];
                src.connect(filter); tail = filter;
            }
            const mix = ctx.createGain();
            tail.connect(mix);
            if (spec.osc) {
                osc = ctx.createOscillator();
                osc.type = spec.osc[0]; osc.frequency.value = spec.osc[1];
                if (spec.filter && src) { osc.connect(filter); mix.gain.value = 1; }
                else osc.connect(mix);
                osc.start();
            }
            const parts = chain(mix, group, o, (o.gain === undefined ? 1 : o.gain) * spec.level, destination(group, o));
            const level = spec.level;
            const h = handleFor(parts, level, t => { try { src.stop(t); if (osc) osc.stop(t); } catch (_) { /* gone */ } }, (v, tc) => {
                if (osc) osc.frequency.setTargetAtTime(spec.osc[1] * v, ctx.currentTime, tc);
                if (filter) filter.frequency.setTargetAtTime(spec.filter[1] * v, ctx.currentTime, tc);
            });
            h.synth = true;
            src.onended = () => { h.ended = true; release(h, [src, osc, mix, filter, parts.gain, parts.pan, parts.send].filter(Boolean)); };
            src.start();
            st.voices++; handles.add(h);
            return h;
        }

        function play(group, options) {
            const o = options || {};
            if (disposed || !ctx) return null;
            if (st.voices >= MAX_VOICES) { st.dropped++; return null; }
            const list = members(group);
            if (list.length) return playSample(group, list[Math.floor(Math.random() * list.length)], o);
            if (lazy || !requested.has(group)) request(group);
            st.fallback++;
            return o.loop ? playLoopRecipe(group, o) : playRecipe(group, o);
        }

        return {
            noise: noise,
            manifest() { return manifest; },
            ready() { return loadManifest(); },
            load: load, loadEngine: loadEngine, engineLayers: engineLayers,
            fetchBuffer: fetchBuffer, buffer(path) { return buffers.get(path) || null; },
            /* Shared decode cache for the music runtime: a pack path, or a random member of an sfx group. */
            musicBuffer(path) { return fetchBuffer(path, 30); },
            groupBuffer(group) {
                return load([group], 30).then(() => {
                    const list = members(group);
                    return list.length ? list[Math.floor(Math.random() * list.length)] : null;
                });
            },
            has: has, play: play, tone: tone, noiseHit: noiseHit,
            lazy: lazy,
            groups() { return manifest && manifest.sfx ? Object.keys(manifest.sfx) : []; },
            stats() { return { samples: st.loaded, failed: st.failed, fallback: st.fallback, dropped: st.dropped, voices: st.voices, queued: queue.length }; },
            dispose() {
                disposed = true;
                queue.length = 0;
                for (const h of Array.from(handles)) { try { h.stop(0); } catch (_) { /* gone */ } }
                handles.clear(); buffers.clear(); jobs.clear();
            }
        };
    };
})();
