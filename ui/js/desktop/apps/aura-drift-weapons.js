/* Aura Drift - weapon table, roulette, firing and AI predicates.
   T1.1 state: the full 18-weapon table (section 2.9) plus working firing for the
   six original weapons (oil, gnome_mine, emp, rocket, mg, shield) so the game
   stays playable. T1.4 implements the other twelve and the AI predicates. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;

    function weapon(id, ammo, cooldown, range, hold, colour, front, mid, back, item) {
        const def = {
            id: id, ammo: ammo, cooldown: cooldown, range: range, colour: colour, hold: hold,
            nameKey: 'aura_drift.weapon.' + id + '.name', hintKey: 'aura_drift.weapon.' + id + '.hint',
            weights: { front: front, mid: mid, back: back }, item: item
        };
        /* Compatibility shim until T5.1 translates the HUD: the 2D HUD and shell
           still read `def.name`. Not enumerable, so tables and hashes ignore it. */
        Object.defineProperty(def, 'name', { value: id, enumerable: false });
        return def;
    }
    const list = [
        weapon('oil', 1, 0.9, 3.2, false, '#1b1b22', 18, 6, 0, null),
        weapon('gnome_mine', 1, 1.1, 3.2, false, '#f97316', 18, 8, 0, 'item-gnome_mine'),
        weapon('emp', 1, 1.4, 30, false, '#38bdf8', 2, 6, 8, null),
        weapon('rocket', 1, 1.6, 90, false, '#fb7185', 4, 12, 10, 'item-rocket'),
        weapon('mg', 6, 0.16, 68, true, '#facc15', 8, 10, 6, 'item-bratwurst'),
        weapon('shield', 1, 1.2, 0, false, '#5eead4', 16, 6, 4, null),
        weapon('ducks', 1, 1.4, 80, false, '#fde047', 6, 10, 8, 'item-rubber_duck'),
        weapon('railgun', 1, 2.0, 220, false, '#a5f3fc', 0, 3, 6, null),
        weapon('flamethrower', 1, 0.5, 18, true, '#fb923c', 2, 8, 6, null),
        weapon('frost', 1, 1.2, 30, false, '#bae6fd', 8, 6, 2, 'item-frost_grenade'),
        weapon('air_horn', 1, 1.0, 18, false, '#fbbf24', 10, 6, 2, 'item-air_horn'),
        weapon('black_hole', 1, 1.8, 45, false, '#7c3aed', 0, 2, 7, null),
        weapon('disco', 1, 1.4, 20, false, '#f472b6', 8, 6, 4, 'item-disco_ball'),
        weapon('glove', 1, 1.0, 7, false, '#ef4444', 0, 5, 6, 'item-boxing_glove'),
        weapon('harpoon', 1, 1.6, 60, false, '#94a3b8', 0, 2, 8, 'item-harpoon'),
        weapon('cow', 1, 2.0, 180, false, '#f5f5f4', 0, 2, 7, null),
        weapon('cloak', 1, 1.2, 0, false, '#99f6e4', 0, 2, 6, null),
        weapon('overdrive', 1, 1.2, 0, false, '#f97316', 0, 0, 10, null)
    ];
    AD.WEAPONS = {};
    for (const def of list) AD.WEAPONS[def.id] = def;
    AD.WEAPON_IDS = list.map(def => def.id);

    /* Until T1.4 lands, crates only hand out what can actually be fired. */
    AD.LEGACY_WEAPON_POOL = ['oil', 'gnome_mine', 'emp', 'rocket', 'mg', 'shield'];

    /* rankFraction = (rank - 1) / (field - 1): front under 0.34, back from 0.67. */
    AD.rollWeapon = function (rng, rankFraction, pool) {
        const band = rankFraction < 0.34 ? 'front' : (rankFraction >= 0.67 ? 'back' : 'mid');
        const ids = (pool && pool.length ? pool : AD.WEAPON_IDS).filter(id => AD.WEAPONS[id].weights[band] > 0);
        let total = 0;
        for (const id of ids) total += AD.WEAPONS[id].weights[band];
        if (!ids.length || total <= 0) return (pool && pool[0]) || AD.WEAPON_IDS[0];
        let roll = rng() * total;
        for (const id of ids) {
            roll -= AD.WEAPONS[id].weights[band];
            if (roll < 0) return id;
        }
        return ids[ids.length - 1];
    };

    AD.giveWeapon = function (race, car, id) {
        if (!race.weapons || !AD.WEAPONS[id]) return;
        car.weapon = id;
        car.ammo = AD.WEAPONS[id].ammo;
        car.cooldown = 0;
        car.weaponLock = 1.1;               /* roulette: firing blocked */
    };

    /* Fire latch, cooldown and roulette lock. `hold` weapons repeat while fire is held. */
    AD.updateWeaponInput = function (race, car, input, dt) {
        if (car.cooldown > 0) car.cooldown = Math.max(0, car.cooldown - dt);
        if (car.weaponLock > 0) car.weaponLock = Math.max(0, car.weaponLock - dt);
        const fire = !!input.fire;
        const edge = fire && !car.fireLatch;
        car.fireLatch = fire;
        if (!race.weapons || !car.weapon || car.v.cloak > 0 || car.v.out) return;
        const def = AD.WEAPONS[car.weapon];
        if (fire && (edge || def.hold) && car.cooldown <= 0 && car.weaponLock <= 0) AD.fireWeapon(race, car);
    };

    AD.fireWeapon = function (race, car) {
        if (!race.weapons || !car.weapon || car.cooldown > 0 || car.ammo <= 0 || car.weaponLock > 0) return;
        if (AD.LEGACY_WEAPON_POOL.indexOf(car.weapon) < 0) return;   /* T1.4 */
        const def = AD.WEAPONS[car.weapon];
        const v = car.v;
        const cos = Math.cos(v.psi), sin = Math.sin(v.psi);
        const bx = v.x - cos * 3.2, by = v.y - sin * 3.2;
        car.cooldown = def.cooldown;
        car.ammo--;
        if (car.tally) car.tally.weaponsFired++;
        AD.emit(race, 'weapon', { id: def.id, x: v.x, y: v.y, car: car.index });

        switch (def.id) {
            case 'oil':
                race.hazards.push({ type: 'oil', x: bx, y: by, r: 7, ttl: 6, owner: car.index });
                break;
            case 'gnome_mine':
                race.hazards.push({ type: 'gnome_mine', x: bx, y: by, r: 3.4, ttl: 12, arm: 0.45, owner: car.index });
                break;
            case 'emp':
                AD.emit(race, 'emp_pulse', { x: v.x, y: v.y, car: car.index, r: 30 });
                for (const other of race.cars) {
                    if (other === car || other.v.out) continue;
                    if (AD.distance(v.x, v.y, other.v.x, other.v.y) < 30) {
                        if (other.v.shield > 0 || other.v.cloak > 0) continue;
                        other.v.disabled = 1.5;
                        AD.emit(race, 'emp_hit', { x: other.v.x, y: other.v.y, car: other.index });
                    }
                }
                break;
            case 'rocket': {
                let target = null, best = Infinity;
                for (const other of race.cars) {
                    if (other === car || other.v.out) continue;
                    const d = AD.distance(v.x, v.y, other.v.x, other.v.y);
                    if (d < best) { best = d; target = other.index; }
                }
                race.projectiles.push({ type: 'rocket', x: v.x + cos * 3, y: v.y + sin * 3, z: 0, psi: v.psi, speed: 62, ttl: 6, target: target, owner: car.index });
                if (target !== null) AD.emit(race, 'rocket_lock', { car: car.index, target: target });
                break;
            }
            case 'mg':
                for (let i = -2; i <= 2; i++) {
                    race.projectiles.push({ type: 'bullet', x: v.x + cos * 3, y: v.y + sin * 3, z: 0, psi: v.psi + i * 0.055, speed: 105, ttl: 0.65, owner: car.index });
                }
                break;
            case 'shield':
                v.shield = 4;
                v.damage = Math.max(0, v.damage - 22);
                AD.emit(race, 'shield', { x: v.x, y: v.y, car: car.index });
                break;
        }
        if (car.ammo <= 0) car.weapon = null;
    };

    /* Placeholder timer branch (the original behaviour); T1.4 adds the predicates. */
    AD.aiWantsFire = function (race, car) {
        if (car.weaponLock > 0 || car.v.cloak > 0) return false;
        car.ai.fireTimer -= AD.STEP;
        if (car.ai.fireTimer > 0) return false;
        car.ai.fireTimer = 4 + race.rng() * 6;
        return true;
    };
})();
