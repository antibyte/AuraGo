/* Aura Drift - engine audio.
 *
 * Player: four looped rpm layers of the chosen car (idle 1000, low 2600, mid 4600, high 6600 rpm),
 * crossfaded with equal power, pitched by clamp(rpm / layerRpm, 0.7, 1.3) and shaped by a throttle
 * low-pass; the rev limiter bounces the level. Rivals: the mid layer of each rival's own car on five
 * voices with distance gain, distance low-pass, Doppler and stereo pan. When samples are missing or
 * still loading the original oscillator stack takes over, so the engine is never silent. The
 * drivetrain extras (turbo whistle, gear whine, gear click, backfire, blow-off) stay live synthesis. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;
    const ORDER = ['idle', 'low', 'mid', 'high'];
    const LAYER_RPM = { idle: 1000, low: 2600, mid: 4600, high: 6600 };
    const RIVAL_VOICES = 5;
    const RIVAL_RANGE = 80;

    AD.ENGINE_LAYER_RPM = LAYER_RPM;

    /* Equal-power weights over the four layers for an rpm. */
    AD.engineWeights = function (rpm) {
        const w = [0, 0, 0, 0];
        if (rpm <= LAYER_RPM.idle) { w[0] = 1; return w; }
        if (rpm >= LAYER_RPM.high) { w[3] = 1; return w; }
        for (let i = 0; i < 3; i++) {
            const a = LAYER_RPM[ORDER[i]], b = LAYER_RPM[ORDER[i + 1]];
            if (rpm < b) {
                const f = (rpm - a) / (b - a);
                w[i] = Math.cos(f * Math.PI / 2); w[i + 1] = Math.sin(f * Math.PI / 2);
                return w;
            }
        }
        return w;
    };

    AD.createEngineAudio = function (ctx, bus, bank, options) {
        const opts = options || {};
        const send = opts.send || null;
        const noise = bank.noise;
        let carId = null, disposed = false, checkAt = 0;
        let osc = null, layers = null;         // live stack / sample layers (each null while unused)
        let limiterPhase = 0, lastGear = 1, lastLoad = 0, wasBoost = false;
        const rivalDistance = new Array(RIVAL_VOICES).fill(RIVAL_RANGE);
        const rivals = [];
        for (let i = 0; i < RIVAL_VOICES; i++) rivals.push({ carId: null, node: null, mode: 'none', asked: false, checkAt: 0 });

        function safe(fn) { try { fn(); } catch (_) { /* node already gone */ } }
        function pannerFor(node, out) {
            let pan = null;
            if (typeof ctx.createStereoPanner === 'function') { pan = ctx.createStereoPanner(); pan.pan.value = 0; node.connect(pan); pan.connect(out); }
            else node.connect(out);
            return pan;
        }

        /* ---- player: live stack ---- */
        function buildOsc() {
            if (osc) return;
            const out = ctx.createGain(); out.gain.value = 0; out.connect(bus);
            const filter = ctx.createBiquadFilter();
            filter.type = 'lowpass'; filter.frequency.value = 900; filter.Q.value = 0.9;
            filter.connect(out);
            const nodes = [];
            for (const spec of [
                { type: 'sawtooth', detune: 0, gain: 0.30, ratio: 1 },
                { type: 'sawtooth', detune: -9, gain: 0.18, ratio: 1 },
                { type: 'square', detune: 7, gain: 0.14, ratio: 0.5 },
                { type: 'sawtooth', detune: 3, gain: 0.07, ratio: 2 },
                { type: 'sawtooth', detune: -22, gain: 0.12, ratio: 0.5 }
            ]) {
                const o = ctx.createOscillator();
                o.type = spec.type; o.frequency.value = 60; o.detune.value = spec.detune;
                const g = ctx.createGain(); g.gain.value = spec.gain;
                o.connect(g).connect(filter); o.start();
                nodes.push({ o: o, gain: g, base: spec.gain, ratio: spec.ratio });
            }
            const hiss = [];
            for (const [f, level] of [[420, 0.12], [260, 0.05]]) {
                const src = ctx.createBufferSource(); src.buffer = noise; src.loop = true;
                const bp = ctx.createBiquadFilter(); bp.type = 'bandpass'; bp.frequency.value = f;
                const g = ctx.createGain(); g.gain.value = level;
                src.connect(bp).connect(g).connect(out); src.start();
                hiss.push({ src: src, filter: bp, gain: g });
            }
            osc = { out: out, filter: filter, nodes: nodes, hiss: hiss };
        }
        function killOsc(fade) {
            if (!osc) return;
            const o = osc; osc = null;
            const now = ctx.currentTime;
            safe(() => o.out.gain.setTargetAtTime(0, now, fade / 3));
            setTimeout(() => {
                for (const n of o.nodes) safe(() => n.o.stop());
                for (const h of o.hiss) safe(() => h.src.stop());
                safe(() => o.out.disconnect());
            }, fade * 1000 + 60);
        }

        /* ---- player: sample layers ---- */
        function buildLayers(set, id) {
            killLayers(0.15);
            const out = ctx.createGain(); out.gain.value = 0; out.connect(bus);
            const shape = ctx.createBiquadFilter();
            shape.type = 'lowpass'; shape.frequency.value = 2400; shape.Q.value = 0.7;
            shape.connect(out);
            const list = ORDER.map(name => {
                const src = ctx.createBufferSource();
                src.buffer = set[name]; src.loop = true;
                const g = ctx.createGain(); g.gain.value = 0;
                src.connect(g).connect(shape);
                src.start(ctx.currentTime, Math.random() * 0.5);
                return { src: src, gain: g, rpm: LAYER_RPM[name] };
            });
            layers = { id: id, out: out, shape: shape, list: list };
        }
        function killLayers(fade) {
            if (!layers) return;
            const l = layers; layers = null;
            safe(() => l.out.gain.setTargetAtTime(0, ctx.currentTime, Math.max(0.01, fade / 3)));
            setTimeout(() => { for (const n of l.list) safe(() => n.src.stop()); safe(() => l.out.disconnect()); }, fade * 1000 + 60);
        }

        /* ---- drivetrain extras (live) ---- */
        const turbo = ctx.createOscillator(), turboGain = ctx.createGain();
        turbo.type = 'sine'; turbo.frequency.value = 2400; turboGain.gain.value = 0;
        turbo.connect(turboGain).connect(bus); turbo.start();
        const whine = ctx.createOscillator(), whineGain = ctx.createGain();
        whine.type = 'triangle'; whine.frequency.value = 900; whineGain.gain.value = 0;
        whine.connect(whineGain).connect(bus); whine.start();

        function gearShift() {
            bank.tone(210, 0.05, 'square', 0.07, 150);
            bank.noiseHit(0.05, 0.05, 2400);
        }
        function backfire() {
            bank.noiseHit(0.16, 0.12, 380);
            bank.tone(140, 0.09, 'triangle', 0.1, 80);
            setTimeout(() => { if (!disposed) bank.noiseHit(0.09, 0.08, 520); }, 55);
        }
        /* The turbo dumps its pressure when the throttle closes. */
        function blowOff() {
            const now = ctx.currentTime;
            const src = ctx.createBufferSource(); src.buffer = noise;
            const filter = ctx.createBiquadFilter();
            filter.type = 'bandpass'; filter.frequency.setValueAtTime(4200, now);
            filter.frequency.exponentialRampToValueAtTime(1400, now + 0.22); filter.Q.value = 1.4;
            const gain = ctx.createGain();
            gain.gain.setValueAtTime(0.11, now); gain.gain.exponentialRampToValueAtTime(0.0001, now + 0.26);
            src.connect(filter).connect(gain).connect(bus);
            src.start(now); src.stop(now + 0.3);
            src.onended = () => { safe(() => { src.disconnect(); filter.disconnect(); gain.disconnect(); }); };
        }

        /* ---- rivals ---- */
        function buildRival(r, id) {
            killRival(r);
            const mid = bank.engineLayers(id, ['mid']);
            const gain = ctx.createGain(); gain.gain.value = 0;
            const lp = ctx.createBiquadFilter(); lp.type = 'lowpass'; lp.frequency.value = 1100;
            let src;
            if (mid) {
                src = ctx.createBufferSource(); src.buffer = mid.mid; src.loop = true;
                src.start(ctx.currentTime, Math.random() * 0.5);
            } else {
                src = ctx.createOscillator(); src.type = 'sawtooth'; src.frequency.value = 70; src.start();
            }
            src.connect(lp).connect(gain);
            const pan = pannerFor(gain, bus);
            if (send) gain.connect(send);
            r.node = { src: src, lp: lp, gain: gain, pan: pan };
            r.mode = mid ? 'sample' : 'osc'; r.carId = id;
        }
        function killRival(r) {
            const n = r.node;
            r.node = null; r.mode = 'none';
            if (!n) return;
            safe(() => n.src.stop()); safe(() => n.gain.disconnect()); safe(() => n.lp.disconnect()); if (n.pan) safe(() => n.pan.disconnect());
        }

        function updatePlayer(v, active, now) {
            const rpmN = clamp(v.rpm / 7200, 0, 1);
            const load = clamp(v.throttle, 0, 1);
            const speed = Math.max(0, v.vx);
            if (v.gear !== lastGear && speed > 2) { gearShift(); lastGear = v.gear; }
            if (lastLoad > 0.55 && load < 0.1 && rpmN > 0.55 && speed > 12) { backfire(); if (rpmN > 0.7) blowOff(); }
            if (wasBoost && !v.boost && speed > 10) blowOff();
            lastLoad = load;
            wasBoost = !!(v.boost || v.burst > 0);
            let limiter = 1;
            if (rpmN > 0.965) { limiterPhase += 0.35; limiter = (limiterPhase % 1) > 0.5 ? 0.45 : 1; }

            /* Sample layers as soon as the four loops of this car are decoded; the live stack until then. */
            if (now >= checkAt && (!layers || layers.id !== carId)) {
                checkAt = now + 0.4;
                const set = carId && bank.engineLayers(carId);
                if (set) { buildLayers(set, carId); killOsc(0.25); }
            }
            if (!layers && !osc) buildOsc();
            const samples = !!layers;

            if (layers) {
                const w = AD.engineWeights(v.rpm);
                for (let i = 0; i < 4; i++) {
                    const l = layers.list[i];
                    l.gain.gain.setTargetAtTime(w[i], now, 0.04);
                    l.src.playbackRate.setTargetAtTime(clamp(v.rpm / l.rpm, 0.7, 1.3), now, 0.04);
                }
                layers.shape.frequency.setTargetAtTime(1400 + load * 5500 + rpmN * 3000, now, 0.05);
                layers.out.gain.setTargetAtTime(active ? (0.34 + load * 0.2) * limiter : 0.03, now, 0.08);
            }
            if (osc) {
                for (const n of osc.nodes) {
                    n.o.frequency.setTargetAtTime((48 + rpmN * 145) * n.ratio, now, 0.04);
                    n.gain.gain.setTargetAtTime(n.base * (0.55 + load * 0.6) * limiter, now, 0.05);
                }
                for (const h of osc.hiss) {
                    h.filter.frequency.setTargetAtTime(300 + rpmN * 1500, now, 0.05);
                    h.gain.gain.setTargetAtTime(0.03 + load * 0.12, now, 0.06);
                }
                osc.filter.frequency.setTargetAtTime(420 + rpmN * 2600 + load * 500, now, 0.05);
                osc.out.gain.setTargetAtTime(active ? (0.16 + load * 0.10) * limiter : 0.02, now, 0.08);
            }
            const share = samples ? 0.6 : 1;
            turboGain.gain.setTargetAtTime(active ? clamp((rpmN - 0.42) * 0.5, 0, 1) * load * 0.028 * share : 0, now, 0.08);
            turbo.frequency.setTargetAtTime(1900 + rpmN * 2400, now, 0.09);
            whineGain.gain.setTargetAtTime(active ? clamp(rpmN - 0.45, 0, 1) * 0.014 * (v.gear || 1) * share : 0, now, 0.06);
            whine.frequency.setTargetAtTime(600 + rpmN * 2600, now, 0.06);
        }

        function updateRivals(race, v, active, tick, heading, now) {
            const cos = Math.cos(heading), sin = Math.sin(heading);
            for (let i = 0; i < RIVAL_VOICES; i++) {
                const r = rivals[i], rival = race.cars[i + 1];
                if (!rival || !rival.v || !active) {
                    if (r.node) r.node.gain.gain.setTargetAtTime(0, now, 0.1);
                    rivalDistance[i] = RIVAL_RANGE;
                    continue;
                }
                const rv = rival.v;
                const id = rv.car ? rv.car.id : null;
                const dx = rv.x - v.x, dy = rv.y - v.y;
                const dist = Math.hypot(dx, dy);
                const level = clamp(1 - dist / RIVAL_RANGE, 0, 1);
                const rate = (dist - rivalDistance[i]) / tick;
                rivalDistance[i] = dist;
                if (level <= 0) { if (r.node) r.node.gain.gain.setTargetAtTime(0, now, 0.1); continue; }
                if (id && !r.asked) { r.asked = true; bank.loadEngine(id, ['mid'], 20); }
                if (!r.node || r.carId !== id || (r.mode === 'osc' && now >= r.checkAt && id && bank.engineLayers(id, ['mid']))) {
                    r.checkAt = now + 0.5;
                    buildRival(r, id);
                }
                const n = r.node;
                const rrpm = clamp(rv.rpm / 7200, 0, 1);
                const doppler = clamp(1 / (1 + rate / 343), 0.82, 1.22);
                if (r.mode === 'sample') {
                    n.src.playbackRate.setTargetAtTime(clamp(rv.rpm / LAYER_RPM.mid, 0.7, 1.3) * doppler, now, 0.05);
                    n.lp.frequency.setTargetAtTime(500 + level * 4200 + rrpm * 600, now, 0.08);
                    n.gain.gain.setTargetAtTime(Math.pow(level, 1.6) * 0.24, now, 0.1);
                } else {
                    n.src.frequency.setTargetAtTime((48 + rrpm * 145) * doppler, now, 0.05);
                    n.lp.frequency.setTargetAtTime(380 + level * 2200 + rrpm * 600, now, 0.08);
                    n.gain.gain.setTargetAtTime(level * level * 0.13, now, 0.1);
                }
                const lateral = (-dx * sin + dy * cos) / Math.max(1, dist);
                if (n.pan) n.pan.pan.setTargetAtTime(clamp(lateral, -1, 1) * 0.8, now, 0.1);
            }
        }

        return {
            setCar(id) {
                if (disposed || !id || id === carId) return;
                carId = id; checkAt = 0;
                if (layers && layers.id !== id) { killLayers(0.2); }
                bank.loadEngine(id, null, 0);
            },
            update(race, active, tick, heading) {
                if (disposed) return;
                const now = ctx.currentTime;
                const v = race.cars[0].v;
                updatePlayer(v, active, now);
                updateRivals(race, v, active, tick, heading === undefined ? v.psi : heading, now);
            },
            mode() { return layers ? 'sample' : (osc ? 'osc' : 'none'); },
            dispose() {
                disposed = true;
                killLayers(0); killOsc(0);
                for (const r of rivals) killRival(r);
                safe(() => turbo.stop()); safe(() => whine.stop());
            }
        };
    };
})();
