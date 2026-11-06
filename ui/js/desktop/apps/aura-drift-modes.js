/* Aura Drift - game modes (section 2.8): rule table, sweeper, drift attack,
   elimination, hot potato, cup points and the result builder.
   Simulation only: no DOM, no Math.random, no Date. Chance comes from race.rng. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;

    function mode(id, opponents, weapons, crates, ramps, pads, destructibles, animals, laps, extra) {
        return Object.assign({
            id: id, opponents: opponents,
            weapons: weapons,                    /* 'setting' follows the user option, false = off, array = pool */
            crates: crates, ramps: ramps, pads: pads, destructibles: destructibles, animals: animals,
            laps: laps                           /* 'config' (menu), 'track' (manifest) or a fixed number */
        }, extra || {});
    }
    const SWEEPER = { sweeper: 45 };
    AD.MODES = [
        mode('quick', 5, 'setting', true, true, true, true, true, 'config', Object.assign({ ranking: 'finish' }, SWEEPER)),
        mode('time_trial', 0, false, false, true, true, true, false, 'config', { ranking: 'time', ghost: true }),
        mode('cup', 5, 'setting', true, true, true, true, true, 'track', Object.assign({ ranking: 'points' }, SWEEPER)),
        mode('daily', 5, 'setting', true, true, true, true, true, 3, Object.assign({ ranking: 'finish' }, SWEEPER)),
        mode('drift_attack', 0, false, false, true, true, true, false, 99, {
            clock: 90, noSmashPenalty: true, ranking: 'score', medals: { bronze: 10000, silver: 22000, gold: 40000 }
        }),
        mode('elimination', 5, 'setting', true, true, true, true, true, 99, {
            ranking: 'survival', firstOut: 45, interval: 30, warn: 5
        }),
        mode('hot_potato', 5, ['air_horn', 'shield', 'overdrive'], true, true, true, true, false, 99, {
            ranking: 'survival', bombDelay: 5, bombGap: 3, immune: 1.5, blastRadius: 16, driveK: 1.12
        })
    ];
    AD.modeRules = function (id) {
        return AD.MODES.find(m => m.id === id) || AD.MODES[0];
    };

    AD.cupPoints = function (position) {
        return [25, 18, 15, 12, 10, 8][position - 1] || 0;
    };

    function alive(race) {
        return race.cars.filter(c => !c.v.out);
    }

    /* Same ordering as stepRace, re-applied after a car goes out so the result of
       this very step already sees it. */
    function resort(race) {
        race.standings = race.cars.slice().sort((a, b) => {
            if (a.v.out !== b.v.out) return a.v.out ? 1 : -1;
            if (a.v.out) return (b.eliminatedAt || 0) - (a.eliminatedAt || 0);
            if (a.finished !== b.finished) return a.finished ? -1 : 1;
            if (a.finished && b.finished) return a.finishTime - b.finishTime;
            return b.progress - a.progress;
        });
    }

    /* Comic exit only: the car is parked and hidden by the renderer, nothing gory. */
    function eliminate(race, car, position) {
        const v = car.v;
        v.out = true;
        v.hasBomb = false;
        v.vx = 0; v.vy = 0; v.r = 0;
        if (v.wx !== undefined) { v.wx = 0; v.wy = 0; }
        car.eliminatedAt = race.time;
        car.weapon = null; car.ammo = 0;
        AD.emit(race, 'eliminated', { car: car.index, x: v.x, y: v.y, position: position });
        resort(race);
    }

    /* Whole-second countdown calls for the last ten seconds of a clock. */
    function tickClock(race) {
        if (race.clock === null || race.clock > 10) return;
        const sec = Math.ceil(race.clock - 1e-9);
        if (sec > 0 && sec !== race.clockSecond) {
            race.clockSecond = sec;
            AD.emit(race, 'clock', { seconds: sec });
        }
    }

    AD.initMode = function (race) {
        const rules = race.rules;
        race.clock = null;
        race.clockSecond = -1;
        race.sweeper = null;
        race.winner = -1;
        race.nextOut = -1;
        race.elimination = { next: null, count: 0, warned: false };
        race.bomb = { holder: -1, fuse: 0, round: 0, delay: 0, immune: -1, immuneFor: 0, next: -1, tick: 0 };
        if (race.mode === 'drift_attack') race.clock = rules.clock;
        else if (race.mode === 'elimination') {
            race.elimination.next = rules.firstOut;
            race.clock = rules.firstOut;
        } else if (race.mode === 'hot_potato') race.bomb.delay = rules.bombDelay;
    };

    /* Quick, cup and daily: the first finisher starts the sweeper, 45 s later the
       race is over for everybody still driving. */
    function updateSweeper(race) {
        if (!race.rules.sweeper) return;
        if (!race.sweeper) {
            if (!race.cars.some(c => c.finished)) return;
            race.sweeper = { at: race.time + race.rules.sweeper };
            AD.emit(race, 'sweeper', {});
        }
        race.clock = Math.max(0, race.sweeper.at - race.time);
        tickClock(race);
    }

    function updateElimination(race) {
        const e = race.elimination, rules = race.rules;
        const left = alive(race);
        if (left.length <= 1) return;
        let last = null;
        for (let i = race.standings.length - 1; i >= 0; i--) {
            if (!race.standings[i].v.out) { last = race.standings[i]; break; }
        }
        if (!last) return;
        race.nextOut = last.index;
        const until = e.next - race.time;
        race.clock = Math.max(0, until);
        tickClock(race);
        if (!e.warned && until <= rules.warn) {
            e.warned = true;
            AD.emit(race, 'elimination_warn', { car: last.index, in: rules.warn });
        }
        if (until > 0) return;
        eliminate(race, last, left.length);
        e.count++;
        e.next += rules.interval;
        e.warned = false;
        race.nextOut = -1;
        if (alive(race).length <= 1) endLastStanding(race);
    }

    function endLastStanding(race) {
        const left = alive(race);
        if (left.length !== 1 || race.winner >= 0) return;
        race.winner = left[0].index;
        AD.emit(race, 'finish', { car: left[0].index, time: race.time });
    }

    function giveBomb(race, car) {
        const b = race.bomb;
        b.holder = car.index;
        b.immune = -1; b.immuneFor = 0;
        car.v.hasBomb = true;
    }

    function updateHotPotato(race, dt) {
        const b = race.bomb, rules = race.rules;
        const left = alive(race);
        if (left.length <= 1) return;
        if (b.immuneFor > 0) b.immuneFor = Math.max(0, b.immuneFor - dt);

        if (b.holder >= 0 && race.cars[b.holder].v.out) b.holder = -1;
        if (b.holder < 0) {
            b.delay -= dt;
            if (b.delay > 0) return;
            let target = b.next >= 0 && !race.cars[b.next].v.out ? race.cars[b.next] : null;
            if (!target) target = left[Math.min(left.length - 1, Math.floor(race.rng() * left.length))];
            b.round++;
            b.fuse = Math.max(10, 16 - b.round);
            b.tick = b.fuse;
            b.next = -1;
            giveBomb(race, target);
            AD.emit(race, 'bomb_given', { car: target.index, by: -1, fuse: b.fuse });
            return;
        }

        const holder = race.cars[b.holder], hv = holder.v;
        b.fuse -= dt;
        /* Ticks speed up as the fuse burns down. */
        const step = b.fuse > 5 ? 1 : (b.fuse > 2 ? 0.5 : 0.25);
        if (b.fuse > 0 && b.fuse <= b.tick - step) {
            b.tick = b.fuse;
            AD.emit(race, 'bomb_tick', { car: holder.index, fuse: b.fuse });
        }

        if (hv.cloak <= 0) {
            for (const other of left) {
                if (other === holder || other.v.cloak > 0) continue;
                if (b.immuneFor > 0 && other.index === b.immune) continue;
                if (Math.abs(other.v.z - hv.z) > 1.6) continue;
                const reach = hv.car.radius + other.v.car.radius + 0.4;
                if (AD.distance(hv.x, hv.y, other.v.x, other.v.y) >= reach) continue;
                hv.hasBomb = false;
                b.immune = holder.index;
                b.immuneFor = rules.immune;
                b.holder = other.index;
                other.v.hasBomb = true;
                AD.emit(race, 'bomb_pass', {
                    from: holder.index, to: other.index, x: (hv.x + other.v.x) / 2, y: (hv.y + other.v.y) / 2
                });
                return;
            }
        }
        if (b.fuse > 0) return;

        /* Blast: the holder is out, everybody close gets a comic shove. */
        const bx = hv.x, by = hv.y, r = rules.blastRadius;
        let near = null, nearD = Infinity;
        for (const other of left) {
            if (other === holder) continue;
            const d = AD.distance(bx, by, other.v.x, other.v.y);
            if (d < nearD) { nearD = d; near = other; }
        }
        AD.emit(race, 'bomb_blast', { car: holder.index, x: bx, y: by, r: r });
        eliminate(race, holder, left.length);
        for (const other of left) {
            if (other === holder) continue;
            if (AD.distance(bx, by, other.v.x, other.v.y) > r) continue;
            AD.applyHit(race, other, { by: holder.index, x: bx, y: by, damage: 0, speed: 0.7, launch: 6, heavy: true, weapon: 'bomb' });
        }
        b.holder = -1;
        b.fuse = 0;
        b.delay = rules.bombGap;
        b.next = near ? near.index : -1;
        b.immune = -1; b.immuneFor = 0;
        if (alive(race).length <= 1) endLastStanding(race);
    }

    AD.updateMode = function (race, dt) {
        switch (race.mode) {
        case 'drift_attack':
            race.clock = Math.max(0, race.rules.clock - race.time);
            tickClock(race);
            break;
        case 'elimination': updateElimination(race); break;
        case 'hot_potato': updateHotPotato(race, dt); break;
        default: updateSweeper(race);
        }
    };

    /* Hot potato: the holder runs down the nearest rival, others within 40 m
       bias away from the holder. Walls still win: the push fades near the edge. */
    AD.modeAiOverride = function (race, car, input) {
        if (race.mode !== 'hot_potato') return;
        const b = race.bomb;
        if (b.holder < 0 || car.v.out) return;
        const v = car.v, holder = race.cars[b.holder];
        const sample = AD.sampleTrack(race.track, v.x, v.y);
        const edge = clamp((Math.abs(sample.offset) / Math.max(1, sample.halfWidth) - 0.5) / 0.4, 0, 1);
        if (car === holder) {
            let target = null, best = Infinity;
            for (const other of race.cars) {
                if (other === car || other.v.out || other.v.cloak > 0) continue;
                const d = AD.distance(v.x, v.y, other.v.x, other.v.y);
                if (d < best) { best = d; target = other; }
            }
            if (!target || best > 90) return;
            const wanted = Math.atan2(target.v.y - v.y, target.v.x - v.x);
            const chase = clamp(AD.wrapAngle(wanted - v.psi) * 2.4 - v.r * 0.22, -1, 1);
            const w = 0.85 * (1 - edge);
            input.steer = clamp(input.steer * (1 - w) + chase * w, -1, 1);
            input.throttle = 1;
            input.brake = 0;
            return;
        }
        if (holder.v.out) return;
        const d = AD.distance(v.x, v.y, holder.v.x, holder.v.y);
        if (d > 40) return;
        const away = Math.atan2(v.y - holder.v.y, v.x - holder.v.x);
        const flee = clamp(AD.wrapAngle(away - v.psi) * 2.4 - v.r * 0.22, -1, 1);
        const w = 0.55 * (1 - d / 40) * (1 - edge);
        input.steer = clamp(input.steer * (1 - w) + flee * w, -1, 1);
    };

    AD.modeFinished = function (race) {
        const player = race.cars[0];
        switch (race.mode) {
        case 'drift_attack':
            return race.clock !== null && race.clock <= 1e-9;
        case 'elimination':
        case 'hot_potato':
            if (race.cars.length < 2) return false;
            return (player.isPlayer && player.v.out) || alive(race).length <= 1;
        default:
            return !!player.finished || (!!race.sweeper && race.time >= race.sweeper.at - 1e-9);
        }
    };

    AD.buildResult = function (race) {
        const player = race.cars[0];
        const style = typeof AD.styleSummary === 'function' ? AD.styleSummary(player) : null;
        const tricks = player.tricks || {};
        const score = style ? style.banked + style.pending : 0;
        const medals = race.rules.medals;
        let medal = null;
        if (medals) medal = score >= medals.gold ? 'gold' : (score >= medals.silver ? 'silver' : (score >= medals.bronze ? 'bronze' : null));
        const survival = race.rules.ranking === 'survival';
        const position = race.standings.indexOf(player) + 1;
        return {
            time: player.finished ? player.finishTime : race.time,
            bestLap: player.bestLap,
            position: position,
            tape: race.tapeOverflow ? [] : player.tape.slice(),
            style: style,
            sectors: player.bestSectors.slice(),
            mode: race.mode,
            score: score,
            medal: medal,
            title: null,                         /* set by the shell through banter */
            tricks: { counts: Object.assign({}, tricks.counts), best: tricks.best || 0 },
            tally: Object.assign({}, player.tally),
            tapeComplete: !race.tapeOverflow,
            eliminatedAt: player.eliminatedAt,
            field: race.cars.length,
            carId: player.v.car.id,
            dnf: !survival && race.rules.ranking !== 'score' && !player.finished,
            points: race.mode === 'cup' ? AD.cupPoints(position) : 0,
            winner: race.winner,
            won: survival ? race.winner === 0 : position === 1
        };
    };
})();
