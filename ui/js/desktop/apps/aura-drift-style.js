/* Aura Drift - style scoring: drift chains, near misses and banked points. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;

    AD.createStyleState = function () {
        return {
            raw: 0,            /* unbanked drift value */
            banked: 0,         /* confirmed points */
            multiplier: 1,
            maxMultiplier: 1,
            combo: 0,          /* seconds of the current chain */
            timer: 0,          /* grace window that links two drifts */
            best: 0,           /* best single bank */
            drifting: false,
            drifts: 0,
            nearMisses: 0,
            lastGain: 0,
            zone: 1,
            nearPass: []
        };
    };

    /* Drift zones are the sharpest corners of a track. They are derived from
       the geometry, so every layout has them and they never drift out of
       sync with the road. */
    AD.buildDriftZones = function (track) {
        const samples = track.samples;
        const count = samples.length;
        const curvature = new Float64Array(count);
        let peak = 0;
        for (let i = 0; i < count; i++) {
            const a = samples[i], b = samples[(i + 12) % count], d = samples[(i + 18) % count];
            const cross = Math.abs(a.tx * b.ty - a.ty * b.tx);
            curvature[i] = cross;
            if (cross > peak) peak = cross;
        }
        if (peak <= 0) return [];
        const threshold = peak * 0.42;
        const zones = [];
        for (let i = 0; i < count; i++) {
            if (curvature[i] < threshold) continue;
            const prev = zones[zones.length - 1];
            if (prev && i - prev.index < 26) {
                prev.to = i;
                continue;
            }
            zones.push({ index: i, from: i, to: i });
        }
        const length = track.length || 1;
        return zones.map(zone => ({
            from: (zone.from / count) * length,
            to: (zone.to / count) * length,
            multiplier: 1.75
        }));
    };

    AD.driftZoneAt = function (track, s) {
        if (!track.driftZones || !track.driftZones.length) return 1;
        const length = track.length || 1;
        const t = ((s % length) + length) % length;
        for (const zone of track.driftZones) {
            if (t >= zone.from && t <= zone.to) return zone.multiplier;
        }
        return 1;
    };

    /* Loose surfaces reward sideways commitment: a rally slide is worth more
       than a polished tarmac slide. */
    function surfaceBonus(surface) {
        if (surface === 'gravel' || surface === 'sand') return 1.4;
        if (surface === 'mud' || surface === 'snow') return 1.25;
        if (surface === 'ice') return 1.5;
        if (surface === 'wet') return 1.15;
        return 1;
    }

    /* Pays style points straight into the bank (tricks, landings, stunts).
       useMultiplier scales them by the current drift multiplier. */
    AD.addStylePoints = function (car, points, useMultiplier) {
        const st = car && car.style;
        if (!st || !(points > 0)) return 0;
        const total = points * (useMultiplier ? st.multiplier : 1);
        st.banked += total;
        if (total > st.best) st.best = total;
        return total;
    };

    AD.updateStyle = function (race, dt) {
        const S = AD.STYLE;
        const cars = race.cars;
        for (const car of cars) {
            const st = car.style;
            if (!st) continue;
            const v = car.v;
            if (v.out) continue;
            const speed = Math.max(0, v.vx);
            const driftAngle = Math.abs(Math.atan2(v.vy, Math.max(1, Math.abs(v.vx))));
            const drifting = !v.airborne && driftAngle > S.driftAngleMin && speed > S.driftSpeedMin && v.slip > 1.1;

            if (drifting) {
                if (!st.drifting) {
                    st.drifts++;
                    if (car.tally) car.tally.drifts++;
                    /* The chain continues across a slide that ends inside the
                       link window; only a banked chain resets the counter. */
                    AD.emit(race, 'drift_start', { car: car.index, multiplier: st.multiplier });
                }
                if (car.tally) car.tally.driftSeconds += dt;
                st.drifting = true;
                st.combo += dt;
                st.timer = S.comboWindow;
                st.multiplier = Math.min(S.multiplierMax,
                    1 + Math.floor(st.combo / 1.6) * S.multiplierStep);
                if (st.multiplier > st.maxMultiplier) st.maxMultiplier = st.multiplier;
                const zone = AD.driftZoneAt(race.track, car.lastS || 0);
                st.zone = zone;
                const gain = S.driftRate * speed * dt * surfaceBonus(v.surface) * zone;
                st.raw += gain;
                st.lastGain = gain;
            } else {
                st.drifting = false;
                if (st.timer > 0) st.timer -= dt;
                else if (st.raw > 0) {
                    const total = st.raw * st.multiplier;
                    st.banked += total;
                    if (total > st.best) st.best = total;
                    AD.emit(race, 'style_bank', { car: car.index, points: total, multiplier: st.multiplier });
                    st.raw = 0;
                    st.multiplier = 1;
                    st.combo = 0;
                }
            }
        }
    };

    /* Sample before the contact solver separates the bodies. The first car owns
       the pair's fastest approach speed; contact spoils it until the cars part. */
    AD.checkNearPass = function (race, a, b) {
        if (!a.style || !b.style) return;
        const passes = a.style.nearPass, slot = b.index, va = a.v, vb = b.v;
        if (va.out || vb.out || va.cloak > 0 || vb.cloak > 0 || a.finished || b.finished || Math.abs(va.z - vb.z) > 1.6) {
            passes[slot] = 0;
            return;
        }
        const dx = vb.x - va.x, dy = vb.y - va.y;
        const gap = Math.hypot(dx, dy) - va.car.radius - vb.car.radius;
        const previous = passes[slot] || 0;
        if (gap <= 0) { passes[slot] = -1; return; }
        if (gap <= AD.STYLE.nearMissClearance) {
            if (previous < 0) return;
            const ac = Math.cos(va.psi), as = Math.sin(va.psi), bc = Math.cos(vb.psi), bs = Math.sin(vb.psi);
            const vx = (vb.airborne ? vb.wx : vb.vx * bc - vb.vy * bs) - (va.airborne ? va.wx : va.vx * ac - va.vy * as);
            const vy = (vb.airborne ? vb.wy : vb.vx * bs + vb.vy * bc) - (va.airborne ? va.wy : va.vx * as + va.vy * ac);
            if (dx * vx + dy * vy < 0) passes[slot] = Math.max(previous, Math.hypot(vx, vy));
            return;
        }
        if (!previous) return;
        passes[slot] = 0;
        // A rescue or teleport can also separate the cars in one tick.
        if (previous < 6 || gap > AD.STYLE.nearMissClearance + 3) return;
        for (const me of [a, b]) {
            const st = me.style, bonus = 80 * st.multiplier;
            st.banked += bonus;
            st.nearMisses++;
            if (previous > 14) {
                st.multiplier = Math.min(AD.STYLE.multiplierMax, st.multiplier + 0.5);
                st.maxMultiplier = Math.max(st.maxMultiplier, st.multiplier);
            }
            if (me.isPlayer) AD.emit(race, 'near_miss', {
                car: me.index, other: me === a ? b.index : a.index, bonus,
                x: (va.x + vb.x) / 2, y: (va.y + vb.y) / 2
            });
        }
    };

    AD.styleSummary = function (car) {
        const st = car.style || AD.createStyleState();
        return {
            banked: Math.round(st.banked),
            pending: Math.round(st.raw * st.multiplier),
            multiplier: st.multiplier,
            maxMultiplier: st.maxMultiplier,
            best: Math.round(st.best),
            drifts: st.drifts,
            nearMisses: st.nearMisses
        };
    };
})();
