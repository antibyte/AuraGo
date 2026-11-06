/* Aura Drift - driver banter and result titles. Pure presentation: it only reads
 * the race and the event stream, owns its own rng and never touches the sim.
 *
 * Line keys (see plan section 2.11):
 *   aura_drift.banter.<driver>.<kind>   kinds: pass passed hit got_you wreck win style
 *     pass     the driver just overtook the player
 *     passed   the player just overtook the driver
 *     hit      the driver was hit by the player
 *     got_you  the driver hit the player
 *     wreck    the driver mocks a player crash (landing or heavy wall hit)
 *     win      an AI driver won the race
 *     style    the driver comments on a style chain the player banked
 *   aura_drift.marshal.1..6   after the player was put back on the road
 *   aura_drift.marshal.7..12  when animals are about to cross
 *   aura_drift.marshal.13..16 when the player wins the race
 * The speaker id of marshal lines is 'marshal'.
 *
 * Every line holds several wordings, one per line of its value. The radio shows
 * one of them and the voice speaks the english original of the very same one, so
 * both always say the same thing and a line never sounds the same twice in a row.
 */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;

    const GAP = 7;            /* seconds between two lines */
    const PER_TYPE = 20;      /* seconds between two lines of the same trigger type */
    const PER_KEY = 60;       /* seconds before the very same line may repeat */
    const DELAY = 0.35;       /* a beat between the event and the remark */
    const MARSHAL_RESPAWN = [1, 6];
    const MARSHAL_CROSS = [7, 12];
    const MARSHAL_WIN = [13, 16];
    const STYLE_POINTS = 200; /* a style chain worth a remark */

    AD.BANTER_KINDS = ['pass', 'passed', 'hit', 'got_you', 'wreck', 'win', 'style'];

    AD.createBanter = function (opts) {
        opts = opts || {};
        const t = typeof opts.t === 'function' ? opts.t : (k => k);
        /* The voice knows which wordings it can speak at once. A line is only worth
         * it when the remark comes out of the speaker right now, so the wordings
         * that are ready are the ones the radio prefers. */
        const ready = typeof opts.ready === 'function' ? opts.ready : null;
        const rng = AD.rng(((opts.seed >>> 0) ^ 0x51ed) >>> 0);
        let clock = 0;
        let lastSpoken = -Infinity;
        let lastDriver = null;
        const lastType = Object.create(null);
        const lastKey = Object.create(null);
        const lastVariant = Object.create(null);
        let pending = null;
        let disposed = false;

        function allowed(type, driverId) {
            if (disposed || pending) return false;
            if (clock - lastSpoken < GAP) return false;
            if (driverId === lastDriver) return false;
            if (type in lastType && clock - lastType[type] < PER_TYPE) return false;
            return true;
        }

        /* The wordings of a line: one per line of its value. */
        function wordingsOf(key) {
            const raw = t(key);
            if (!raw || raw === key) return null;
            const list = String(raw).split('\n').map(text => text.trim()).filter(Boolean);
            return list.length ? list : null;
        }

        /* The wording of the line: one the voice can speak right now if it knows
         * one, and never the one of the last time. */
        function pickVariant(key, count) {
            let pool = [];
            if (ready) {
                const known = ready(key);
                if (known && known.length) for (const index of known) if (index < count) pool.push(index);
            }
            if (!pool.length) for (let index = 0; index < count; index++) pool.push(index);
            const last = lastVariant[key];
            let choice = pool[Math.floor(rng() * pool.length)];
            if (pool.length > 1 && choice === last) {
                const others = pool.filter(index => index !== last);
                choice = others[Math.floor(rng() * others.length)];
            }
            lastVariant[key] = choice;
            return choice;
        }

        function say(type, driverId, key) {
            if (key in lastKey && clock - lastKey[key] < PER_KEY) return;
            if (!allowed(type, driverId)) return;
            const texts = wordingsOf(key);
            if (!texts) return;
            const variant = pickVariant(key, texts.length);
            const text = texts[variant];
            if (!text) return;
            /* The key and the wording travel with the line: the voice commentary
             * reads the english original of the same wording, whatever language
             * the radio shows. */
            pending = { driverId: driverId, text: AD.fmt(text, {}), key: key, variant: variant, wait: DELAY };
            lastSpoken = clock;
            lastDriver = driverId;
            lastType[type] = clock;
            lastKey[key] = clock;
        }

        function driverKey(driver, kind) { return 'aura_drift.banter.' + driver.id + '.' + kind; }

        function aiDriver(race, index) {
            const c = race && race.cars ? race.cars[index] : null;
            return c && !c.isPlayer && c.driver ? c.driver : null;
        }

        function randomRival(race) {
            const list = (race && race.cars ? race.cars : []).filter(c => !c.isPlayer && c.driver);
            return list.length ? list[Math.floor(rng() * list.length)].driver : null;
        }

        function nearestRival(race) {
            const me = race && race.cars ? race.cars[0] : null;
            let best = null, bestD = Infinity;
            if (!me || !me.v) return randomRival(race);
            for (const c of race.cars) {
                if (c.isPlayer || !c.driver || !c.v) continue;
                const d = AD.distance(me.v.x, me.v.y, c.v.x, c.v.y);
                if (d < bestD) { bestD = d; best = c.driver; }
            }
            return best;
        }

        /* Least recently used marshal line in the range (random among equals). */
        function marshalKey(range) {
            let best = [], bestAge = -Infinity;
            for (let n = range[0]; n <= range[1]; n++) {
                const key = 'aura_drift.marshal.' + n;
                const age = key in lastKey ? clock - lastKey[key] : Infinity;
                if (age > bestAge) { bestAge = age; best = [key]; } else if (age === bestAge) best.push(key);
            }
            return best[Math.floor(rng() * best.length)];
        }

        function onEvent(evt, race) {
            if (disposed || !evt) return;
            switch (evt.type) {
            case 'position': {
                if (evt.car !== 0) break;
                const standings = race && race.standings ? race.standings : null;
                if (evt.to < evt.from) {
                    const other = standings && standings[evt.to] && !standings[evt.to].isPlayer ? standings[evt.to].driver : randomRival(race);
                    if (other) say('position', other.id, driverKey(other, 'passed'));
                } else if (evt.to > evt.from) {
                    const other = standings && standings[evt.to - 2] && !standings[evt.to - 2].isPlayer ? standings[evt.to - 2].driver : randomRival(race);
                    if (other) say('position', other.id, driverKey(other, 'pass'));
                }
                break;
            }
            case 'hit': {
                if (evt.car === 0) {
                    const by = aiDriver(race, evt.by);
                    if (by) say('hit', by.id, driverKey(by, 'got_you'));
                } else if (evt.by === 0) {
                    const victim = aiDriver(race, evt.car);
                    if (victim) say('hit', victim.id, driverKey(victim, 'hit'));
                }
                break;
            }
            case 'landing':
                if (evt.car === 0 && evt.quality === 'crash') {
                    const d = nearestRival(race);
                    if (d) say('wreck', d.id, driverKey(d, 'wreck'));
                }
                break;
            case 'wall_hit':
                if (evt.car === 0 && evt.heavy) {
                    const d = nearestRival(race);
                    if (d) say('wreck', d.id, driverKey(d, 'wreck'));
                }
                break;
            case 'finish': {
                if (race && race.cars && race.cars.some(c => c.finished && c.index !== evt.car && c.finishTime <= evt.time)) break;
                if (evt.car === 0) { say('finish', 'marshal', marshalKey(MARSHAL_WIN)); break; }
                const d = aiDriver(race, evt.car);
                if (d) say('finish', d.id, driverKey(d, 'win'));
                break;
            }
            case 'style_bank': {
                /* The player banked a style chain: the nearest rival has an opinion
                 * about it. Short chains are not worth a remark. */
                if (evt.car !== 0) break;
                if (!(evt.points >= STYLE_POINTS)) break;
                const d = nearestRival(race);
                if (d) say('style', d.id, driverKey(d, 'style'));
                break;
            }
            case 'respawn':
                if (evt.car === 0) say('respawn', 'marshal', marshalKey(MARSHAL_RESPAWN));
                break;
            case 'animal_cross':
                say('animal_cross', 'marshal', marshalKey(MARSHAL_CROSS));
                break;
            default:
                break;
            }
        }

        function update(dt) {
            if (disposed) return null;
            clock += Math.max(0, dt || 0);
            if (!pending) return null;
            pending.wait -= dt;
            if (pending.wait > 0) return null;
            const out = { driverId: pending.driverId, text: pending.text, key: pending.key, variant: pending.variant };
            pending = null;
            return out;
        }

        function dispose() { disposed = true; pending = null; }

        return { onEvent: onEvent, update: update, dispose: dispose };
    };

    /* First match wins (plan T5.3). `car` is the player's car actor (optional),
       `race` the race (optional, only to know whether weapons were on). */
    AD.pickResultTitle = function (result, car, race) {
        const r = result || {};
        const tally = r.tally || {};
        const time = Number(r.time) || 0;
        const longEnough = time >= 20;        /* tiny runs get no statistical titles */
        const carId = r.carId || (car && car.v && car.v.car ? car.v.car.id : '');
        const weaponsOn = race ? race.weapons !== false : r.weapons !== false;
        const position = r.position || 0;
        const field = r.field || 0;
        const n = k => Number(tally[k]) || 0;

        if (weaponsOn && n('weaponsFired') === 0 && position === 1 && field > 1 && longEnough) return 'pacifist';
        /* Never braking only deserves a cheer when it worked: a top-half finish over a real race length. */
        const topHalf = position >= 1 && (field <= 1 || position <= Math.ceil(field / 2));
        if (time >= 45 && topHalf && n('brakeSeconds') < 0.02 * time) return 'brake_coward';
        if (n('animalHits') >= 5) return 'cow_bowler';
        if (n('airTime') >= 20) return 'air_force';
        if (n('crashLandings') >= 3) return 'pancake_chef';
        if (n('wallHits') >= 25) return 'wall_lover';
        if (longEnough && n('driftSeconds') >= 0.35 * time) return 'drift_king';
        if (n('reverseSeconds') >= 20) return 'reverse_hero';
        if (n('sausageHits') >= 12) return 'sausage_baron';
        if (n('discoFired') >= 2) return 'disco_fever';
        if (carId === 'maehkoenig' && n('smashed') >= 15) return 'gardener';
        if (carId === 'softeis' && position >= 1 && position <= 3) return 'ice_man';
        if (carId === 'couch' && position >= 1 && position <= 2) return 'sofa_king';
        if (carId === 'bertha' && n('carHits') >= 15) return 'big_bertha';
        if (field > 1 && position === field && n('hitsDealt') >= 3) return 'last_but_loud';
        return 'solid_driver';
    };
})();
