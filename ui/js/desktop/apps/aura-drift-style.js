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
            missLatch: 0
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

    AD.updateStyle = function (race, dt) {
        const S = AD.STYLE;
        const cars = race.cars;
        for (const car of cars) {
            const st = car.style;
            if (!st) continue;
            const v = car.v;
            const speed = Math.max(0, v.vx);
            const driftAngle = Math.abs(Math.atan2(v.vy, Math.max(1, Math.abs(v.vx))));
            const drifting = driftAngle > S.driftAngleMin && speed > S.driftSpeedMin && v.slip > 1.1;

            if (drifting) {
                if (!st.drifting) {
                    st.drifts++;
                    /* The chain continues across a slide that ends inside the
                       link window; only a banked chain resets the counter. */
                    race.events.push({
                        type: 'drift_start', t: race.time, car: car.index,
                        multiplier: st.multiplier
                    });
                }
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
                    race.events.push({
                        type: 'style_bank', t: race.time, car: car.index,
                        points: total, multiplier: st.multiplier
                    });
                    st.raw = 0;
                    st.multiplier = 1;
                    st.combo = 0;
                }
            }
        }

        /* Near misses: a rival inside arm's length at closing speed is a
           style event for the driver who survived it. */
        for (let i = 0; i < cars.length; i++) {
            for (let j = i + 1; j < cars.length; j++) {
                const a = cars[i], b = cars[j];
                if (!a.style || !b.style) continue;
                const dx = b.v.x - a.v.x, dy = b.v.y - a.v.y;
                const dist = Math.hypot(dx, dy);
                if (dist > S.nearMissDistance) { a.style.missLatch = 0; b.style.missLatch = 0; continue; }
                const closing = Math.abs((b.v.vx - a.v.vx) * Math.cos(a.v.psi) +
                    (b.v.vy - a.v.vy) * Math.sin(a.v.psi));
                if (closing < 6) continue;
                if (a.style.missLatch || b.style.missLatch) continue;
                a.style.missLatch = 1;
                b.style.missLatch = 1;
                const bonus = 80 * a.style.multiplier;
                a.style.banked += bonus;
                b.style.banked += bonus;
                a.style.nearMisses++;
                b.style.nearMisses++;
                if (closing > 14) {
                    a.style.multiplier = Math.min(AD.STYLE.multiplierMax, a.style.multiplier + 0.5);
                    b.style.multiplier = Math.min(AD.STYLE.multiplierMax, b.style.multiplier + 0.5);
                }
                /* Only the player's near misses are announced; a pack of AI
                   cars trading paint would be noise. */
                if (a.index === 0 || b.index === 0) {
                    race.events.push({
                        type: 'near_miss', t: race.time, x: (a.v.x + b.v.x) / 2,
                        y: (a.v.y + b.v.y) / 2, car: 0, bonus: bonus
                    });
                }
            }
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
