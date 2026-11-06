/* Aura Drift - ramps, pads, crates, destructibles, animals, set pieces.
   T1.1 placeholder with the final signatures (section 1.5). Only the weapon
   crates exist so the six original weapons keep working; T1.3 replaces this
   file with the full feature set. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;

    AD.createFeatures = function (track, seed, rules) {
        const fs = {
            rng: AD.rng(AD.hashString(track.id + '/features/' + seed)),
            rules: rules || {},
            crates: [], ramps: [], pads: [], destructibles: [], animals: [], crossings: [], setpieces: [],
            buckets: []
        };
        for (let i = 0; i < 64; i++) fs.buckets.push([]);
        if (!rules || rules.crates !== false) {
            track.pads.forEach((pad, i) => {
                const p = AD.pointAt(track, pad.s);
                fs.crates.push({
                    id: i, kind: 'crate', s: pad.s, offset: pad.offset, row: pad.row,
                    x: p.x + p.nx * pad.offset, y: p.y + p.ny * pad.offset,
                    taken: -99          /* race time of the last pickup */
                });
            });
        }
        return fs;
    };

    /* Crate pickup: radius 4.2, 8 s respawn, taken even when a weapon is held. */
    AD.updateActorFeatures = function (race, actor, fs, dt) {
        if (actor.ghost || !race.weapons || actor.v.out || (actor.v.z || 0) >= 3.5) return;
        const v = actor.v;
        for (const crate of fs.crates) {
            if (race.time - crate.taken < 8) continue;
            if (AD.distance(crate.x, crate.y, v.x, v.y) >= 4.2) continue;
            crate.taken = race.time;
            if (!actor.weapon) {
                const field = race.cars.length;
                const rank = Math.max(1, race.standings.indexOf(actor) + 1);
                const fraction = field > 1 ? (rank - 1) / (field - 1) : 0;
                const id = AD.rollWeapon(race.rng, fraction, AD.LEGACY_WEAPON_POOL);
                AD.giveWeapon(race, actor, id);
                AD.emit(race, 'pickup', { x: crate.x, y: crate.y, car: actor.index, id: id, lock: actor.weaponLock });
            }
        }
    };

    AD.updateFeatures = function (race, fs, dt) {};
    AD.featureLaneBias = function (race, car, sample) { return 0; };
    AD.validateFeatures = function (track, fs) { return { ok: true, reasons: [] }; };
})();
