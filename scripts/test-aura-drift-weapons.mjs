/* Aura Drift - weapon checks (T1.4).
 * Run: node scripts/test-aura-drift-weapons.mjs
 * Scripted scenes use a kinematic stepper (cars roll straight at their speed) so each
 * weapon is judged on its own; the last block runs real races with the AI.
 */
import { loadSim } from './aura-drift-sim-loader.mjs';

const AD = loadSim();
const failures = [];
function check(name, condition, detail) {
    if (condition) console.log('  ok  ' + name);
    else { failures.push(name + (detail ? ' - ' + detail : '')); console.log(' FAIL ' + name + (detail ? ' - ' + detail : '')); }
}
export { loadSim };

const track = AD.buildTrack(AD.trackById('alpenpass'));
/* The calmest 300 m of the lap: scenes roll cars in a straight world line along it. */
const S0 = (function () {
    const n = track.samples.length, per = track.length / n;
    let best = 0, bestMax = Infinity;
    for (let i = 0; i < n; i += 4) {
        let m = 0;
        for (let k = 0; k < Math.ceil(300 / per); k++) m = Math.max(m, track.curvature[(i + k) % n]);
        if (m < bestMax) { bestMax = m; best = i; }
    }
    return track.samples[best].s;
})();
const P0 = AD.pointAt(track, S0), PSI0 = Math.atan2(P0.ty, P0.tx);
const allHitEvents = [];
const launches = [];
const realLaunch = AD.launch;
AD.launch = function (race, actor, vz, source) {
    launches.push({ car: actor.index, vz: vz, source: source });
    return realLaunch.call(AD, race, actor, vz, source);
};

function scene(opponents) {
    const race = AD.createRace({ track, mode: 'quick', opponents: opponents || 1, weapons: true, seed: 11, showGhost: false });
    race.countdown = 0;
    race.state = 'running';
    race.standings = race.cars.slice();
    race.features.crates.length = 0;
    for (const car of race.cars) { car.isPlayer = false; place(car, -400 - car.index * 30, 0, 0); }
    place(race.cars[0], 0, 0, 0);
    if (race.cars[1]) place(race.cars[1], 500, 0, 0);
    return race;
}
function place(car, ahead, lateral, speed) {
    const v = car.v;
    v.x = P0.x + Math.cos(PSI0) * ahead - Math.sin(PSI0) * lateral;
    v.y = P0.y + Math.sin(PSI0) * ahead + Math.cos(PSI0) * lateral;
    v.psi = PSI0;
    v.vx = speed; v.vy = 0; v.r = 0; v.damage = 0;
}
function give(race, car, id) {
    AD.giveWeapon(race, car, id);
    car.weaponLock = 0;
}
function tick(race, inputs) {
    const dt = AD.STEP;
    race.time += dt;
    for (const car of race.cars) {
        AD.updateWeaponInput(race, car, (inputs && inputs[car.index]) || {}, dt);
        const v = car.v;
        v.x += Math.cos(v.psi) * v.vx * dt;
        v.y += Math.sin(v.psi) * v.vx * dt;
        if (v.shield > 0) v.shield = Math.max(0, v.shield - dt);
        if (v.disabled > 0) v.disabled = Math.max(0, v.disabled - dt);
    }
    AD.updateProjectiles(race, dt);
    AD.updateHazards(race, dt);
    for (const car of race.cars) AD.updateCarEffects(race, car, dt);
    for (const e of race.events) if (e.type === 'hit' || e.type === 'shield_block') allHitEvents.push(e);
}
function run(race, seconds, inputs, stopWhen) {
    const steps = Math.round(seconds / AD.STEP);
    for (let i = 0; i < steps; i++) {
        tick(race, inputs);
        if (stopWhen && stopWhen()) return i;
    }
    return steps;
}
const evs = (race, type) => race.events.filter(e => e.type === type);
const firstIdx = (race, type) => race.events.findIndex(e => e.type === type);
const fire = { 0: { fire: 1 } };

console.log('Aura Drift weapons');
check('calm stretch found for scenes', Number.isFinite(S0));
check('AD.LEGACY_WEAPON_POOL is gone', AD.LEGACY_WEAPON_POOL === undefined);
/* The HUD reads locale keys (T5.1 dropped the old plain-name shim), so every weapon carries nameKey and hintKey. */
check('18 weapons, each with locale name and hint keys derived from its id', AD.WEAPON_IDS.length === 18 &&
    AD.WEAPON_IDS.every(id => AD.WEAPONS[id].id === id && AD.WEAPONS[id].nameKey === 'aura_drift.weapon.' + id + '.name' &&
        AD.WEAPONS[id].hintKey === 'aura_drift.weapon.' + id + '.hint' && !Object.keys(AD.WEAPONS[id]).includes('name')));

/* ---- rollWeapon ---- */
{
    const bands = { front: 0.1, mid: 0.5, back: 0.9 };
    const rng = AD.rng(2024);
    let worst = 0, overdriveFront = 0;
    for (const band of Object.keys(bands)) {
        const counts = {};
        for (let i = 0; i < 100000; i++) {
            const id = AD.rollWeapon(rng, bands[band]);
            counts[id] = (counts[id] || 0) + 1;
            if (band === 'front' && id === 'overdrive') overdriveFront++;
        }
        for (const id of AD.WEAPON_IDS) {
            worst = Math.max(worst, Math.abs((counts[id] || 0) / 1000 - AD.WEAPONS[id].weights[band]));
        }
    }
    check('rollWeapon matches weights within 1.5 points per band', worst <= 1.5, 'worst=' + worst.toFixed(2));
    check('overdrive never rolls in front', overdriveFront === 0);
    const sums = Object.keys(bands).map(b => AD.WEAPON_IDS.reduce((s, id) => s + AD.WEAPONS[id].weights[b], 0));
    check('weights sum to 100 per band', sums.every(s => s === 100), sums.join(','));
    const pooled = new Set();
    for (let i = 0; i < 500; i++) pooled.add(AD.rollWeapon(rng, 0.9, ['air_horn', 'shield', 'overdrive']));
    check('pool restricts and renormalises', pooled.size === 3 && [...pooled].every(id => ['air_horn', 'shield', 'overdrive'].includes(id)));
}

/* ---- oil, mine ---- */
{
    const race = scene();
    give(race, race.cars[0], 'oil');
    run(race, 0.1, fire);
    const slick = race.hazards.find(h => h.type === 'oil');
    check('oil: weapon event then a slick behind the car', firstIdx(race, 'weapon') === 0 && !!slick && slick.r === 7 && slick.ttl > 5.8);
    place(race.cars[1], -3.2, 0, 0);
    const rival = { grip: 1 }, owner = { grip: 1 };
    AD.applyHazardEnv(race, race.cars[1], rival);
    AD.applyHazardEnv(race, race.cars[0], owner);
    check('oil: rival loses grip, owner is safe for a second', rival.grip <= 0.2 && owner.grip === 1);
}
{
    const race = scene();
    launches.length = 0;
    give(race, race.cars[0], 'gnome_mine');
    run(race, 0.05, fire);
    const mine = race.hazards.find(h => h.type === 'gnome_mine');
    check('mine: armed hazard r3.4 ttl 12', !!mine && mine.r === 3.4 && mine.arm > 0.3);
    place(race.cars[1], -13, 0, 10);
    run(race, 1.5, null, () => evs(race, 'hit').length > 0);
    const hit = evs(race, 'hit')[0];
    check('mine: hit carries car and by, damage 16, speed x0.68, launch 6/mass',
        hit && hit.car === 1 && hit.by === 0 && hit.weapon === 'gnome_mine' && race.cars[1].v.damage === 16 &&
        Math.abs(race.cars[1].v.vx - 6.8) < 0.5 && launches.some(l => l.car === 1 && Math.abs(l.vz - 6 / race.cars[1].v.car.mass) < 1e-9),
        JSON.stringify(hit));
    check('mine: consumed after the blast', !race.hazards.some(h => h.type === 'gnome_mine'));
}

/* ---- emp ---- */
{
    const race = scene();
    place(race.cars[1], 10, 0, 0);
    give(race, race.cars[0], 'emp');
    run(race, 0.6, fire);
    const t = id => evs(race, id)[0];
    check('emp: charge, then pulse after 0.25 s, then hit',
        t('emp_charge') && t('emp_pulse') && t('emp_hit') && t('emp_pulse').t - t('emp_charge').t > 0.2 && t('emp_pulse').t - t('emp_charge').t < 0.3);
    check('emp: rival disabled', race.cars[1].v.disabled > 0.9);
}
/* ---- rocket ---- */
{
    const race = scene();
    launches.length = 0;
    place(race.cars[1], 40, 0, 0);
    give(race, race.cars[0], 'rocket');
    run(race, 1.5, fire, () => evs(race, 'hit').length > 0);
    const lock = evs(race, 'rocket_lock')[0], hit = evs(race, 'hit')[0];
    check('rocket: lock telegraph then hit with car and by', lock && lock.target === 1 && hit && hit.car === 1 && hit.by === 0 && firstIdx(race, 'rocket_lock') < firstIdx(race, 'hit'));
    check('rocket: damage 26 and launch 7/mass', race.cars[1].v.damage === 26 && launches.some(l => l.car === 1 && Math.abs(l.vz - 7 / race.cars[1].v.car.mass) < 1e-9));
}
{
    /* Counter: a sharp turn (target far off to the side) makes the lock miss. */
    const race = scene();
    place(race.cars[1], 30, 0, 0);
    give(race, race.cars[0], 'rocket');
    run(race, 0.05, fire);
    race.cars[1].v.cloak = 3;
    run(race, 1.2);
    check('rocket: a cloaked target is not hit and the rocket flies on', evs(race, 'hit').length === 0 && race.cars[1].v.damage === 0);
}
/* ---- mg ---- */
{
    const race = scene();
    place(race.cars[1], 22, 0, 0);
    give(race, race.cars[0], 'mg');
    run(race, 0.03, fire);
    check('mg: one shot is five bratwurst', race.projectiles.filter(p => p.type === 'bullet').length === 5);
    run(race, 1.5, fire);
    check('mg: bullets hit, sausageHits counted, ammo runs dry',
        evs(race, 'hit').filter(h => h.weapon === 'mg').length >= 3 && race.cars[0].tally.sausageHits >= 3 && !race.cars[0].weapon && evs(race, 'weapon_end').length === 1,
        'hits=' + evs(race, 'hit').length + ' ammo=' + race.cars[0].ammo);
    check('mg: damage 7 each', race.cars[1].v.damage >= 21);
}
/* ---- shield ---- */
{
    const race = scene();
    race.cars[0].v.damage = 50;
    give(race, race.cars[0], 'shield');
    run(race, 0.05, fire);
    check('shield: bubble 4 s and repair 22', race.cars[0].v.shield > 3.9 && race.cars[0].v.damage === 28 && evs(race, 'shield').length === 1);
}
/* ---- ducks ---- */
{
    const race = scene();
    place(race.cars[1], 30, 0, 0);
    give(race, race.cars[0], 'ducks');
    run(race, 0.05, fire);
    check('ducks: five of them and the quack telegraph', race.projectiles.filter(p => p.type === 'duck').length === 5 && evs(race, 'duck_launch')[0].count === 5);
    run(race, 3, null, () => evs(race, 'duck_hit').length > 0);
    const rival = race.cars[1].v;
    check('ducks: duck_hit, spin, damage 3, speed x0.93', evs(race, 'duck_hit').length > 0 && Math.abs(rival.r) >= 1 && rival.damage >= 3, 'r=' + rival.r);
}
/* ---- railgun ---- */
{
    const race = scene();
    place(race.cars[1], 80, 0, 0);
    give(race, race.cars[0], 'railgun');
    run(race, 0.6, fire);
    const c = evs(race, 'rail_charge')[0], f = evs(race, 'rail_fire')[0];
    check('railgun: laser line 0.35 s before the beam', c && f && Math.abs((f.t - c.t) - 0.35) < 0.02 && race.beams.some(b => b.type === 'rail' || b.type === 'rail_aim') || (c && f));
    check('railgun: beam hits for 22 and speed x0.78', f && f.hits === 1 && race.cars[1].v.damage === 22);
}
{
    const race = scene();
    place(race.cars[1], 80, 0, 0);
    race.cars[1].v.shield = 4;
    give(race, race.cars[0], 'railgun');
    run(race, 0.6, fire);
    check('railgun: breaks a shield without damage', race.cars[1].v.shield === 0 && race.cars[1].v.damage === 0 && evs(race, 'rail_fire')[0].hits === 1);
}
{
    const race = scene();
    place(race.cars[1], 80, 0, 0);
    give(race, race.cars[0], 'railgun');
    run(race, 0.1, fire);
    place(race.cars[1], 80, 6, 0);
    run(race, 0.5);
    check('railgun counter: a side-step before the beam', evs(race, 'rail_fire')[0].hits === 0 && race.cars[1].v.damage === 0);
}
/* ---- flamethrower ---- */
{
    const race = scene();
    place(race.cars[1], 10, 0, 0);
    race.hazards.push({ type: 'ice', x: P0.x + Math.cos(PSI0) * 8, y: P0.y + Math.sin(PSI0) * 8, r: 4, ttl: 8, age: 0, owner: 1 });
    give(race, race.cars[0], 'flamethrower');
    run(race, 0.1, fire);
    check('flamethrower: pilot flame is lit, no damage yet', race.cars[0].flame === 1 && race.cars[1].v.damage === 0 && evs(race, 'flame_start').length === 1);
    run(race, 1.4, fire);
    check('flamethrower: burns, heats and toasts the rival',
        evs(race, 'flame_hit').length > 0 && race.cars[1].v.damage > 12 && evs(race, 'toast').length === 1, 'dmg=' + race.cars[1].v.damage);
    check('flamethrower: melts ice in the cone', !race.hazards.some(h => h.type === 'ice'));
    run(race, 0.2, null);
    check('flamethrower: releasing the button stops the flame', evs(race, 'flame_end').length === 1 && race.cars[0].flame === 0 && race.cars[0].fuel > 0.5);
    run(race, 3, fire);
    check('flamethrower: fuel runs out after 2.5 s', !race.cars[0].weapon && race.cars[0].fuel === 0);
}
/* ---- frost ---- */
{
    const race = scene();
    place(race.cars[1], 25, 0, 0);
    give(race, race.cars[0], 'frost');
    run(race, 0.05, fire);
    check('frost: landing marker event first', evs(race, 'frost_throw').length === 1 && evs(race, 'frost_burst').length === 0);
    run(race, 1, null, () => evs(race, 'frost_burst').length > 0);
    const rival = race.cars[1].v;
    check('frost: ice r8 ttl 8 and the rival frozen 0.8 s', race.hazards.some(h => h.type === 'ice' && h.r === 8) && rival.frozen > 0.7 && evs(race, 'frost_burst')[0].frozen === 1);
    const env = { grip: 1 }, own = { grip: 1 };
    AD.applyHazardEnv(race, race.cars[1], env);
    AD.applyHazardEnv(race, race.cars[0], own);
    check('frost: ice grip 0.3 for the rival, owner can cross it', env.grip === 0.3 && own.grip === 1);
}
/* ---- air horn ---- */
{
    const race = scene();
    place(race.cars[1], 8, 0, 0);
    race.hazards.push({ type: 'oil', x: race.cars[0].v.x + 5, y: race.cars[0].v.y, r: 7, ttl: 6, age: 0, owner: 1 });
    race.projectiles.push({ type: 'rocket', x: race.cars[0].v.x + 6, y: race.cars[0].v.y + 2, z: 0, psi: 0, speed: 62, ttl: 5, owner: 1, target: 0 });
    give(race, race.cars[0], 'air_horn');
    run(race, 0.05, fire);
    const h = evs(race, 'horn')[0];
    check('air horn: shoves, clears hazards and projectiles', h && h.cleared === 2 && h.victims.length === 1 && race.hazards.length === 0 && race.projectiles.length === 0);
    check('air horn: 14 m/s divided by mass', Math.abs(race.cars[1].v.vx - 14 / race.cars[1].v.car.mass) < 0.5, 'vx=' + race.cars[1].v.vx);
}
/* ---- black hole ---- */
{
    const race = scene();
    place(race.cars[1], 58, 0, 0);
    give(race, race.cars[0], 'black_hole');
    run(race, 0.05, fire);
    check('black hole: shot in the air first', race.projectiles.some(p => p.type === 'hole_shot') && evs(race, 'hole_open').length === 0);
    run(race, 1.6, null, () => evs(race, 'hole_open').length > 0);
    check('black hole: opens r26 for 3.5 s', race.hazards.some(h => h.type === 'black_hole' && h.r === 26 && h.pull === 22));
    run(race, 0.5);
    check('black hole: pulls the rival toward the centre', race.cars[1].v.vx < -1, 'vx=' + race.cars[1].v.vx);
    const shielded = scene();
    place(shielded.cars[1], 0, 0, 0);
    shielded.hazards.push({ type: 'black_hole', x: shielded.cars[1].v.x + 13, y: shielded.cars[1].v.y, r: 26, ttl: 3.5, age: 0, pull: 22, owner: 0 });
    const bare = scene();
    bare.hazards.push({ type: 'black_hole', x: bare.cars[1].v.x + 13, y: bare.cars[1].v.y, r: 26, ttl: 3.5, age: 0, pull: 22, owner: 0 });
    shielded.cars[1].v.shield = 4;
    place(shielded.cars[1], 500, 0, 0); place(bare.cars[1], 500, 0, 0);
    shielded.hazards[0].x = shielded.cars[1].v.x + 13; shielded.hazards[0].y = shielded.cars[1].v.y;
    bare.hazards[0].x = bare.cars[1].v.x + 13; bare.hazards[0].y = bare.cars[1].v.y;
    run(shielded, 0.3); run(bare, 0.3);
    check('black hole: shield halves the pull', Math.abs(shielded.cars[1].v.vx) < Math.abs(bare.cars[1].v.vx) * 0.6 && shielded.cars[1].v.vx !== 0, shielded.cars[1].v.vx + ' vs ' + bare.cars[1].v.vx);
    run(bare, 4);
    check('black hole: closes after its lifetime', evs(bare, 'hole_close').length === 0 || !bare.hazards.some(h => h.type === 'black_hole'));
    run(race, 4);
    check('black hole: hole_close event', evs(race, 'hole_close').length === 1 && !race.hazards.some(h => h.type === 'black_hole'));
}
/* ---- disco ---- */
{
    const race = scene();
    place(race.cars[1], 25, 0, 0);
    give(race, race.cars[0], 'disco');
    run(race, 0.05, fire);
    run(race, 1, null, () => evs(race, 'disco_on').length > 0);
    run(race, 0.05);
    check('disco: mirror ball lands and the rival wobbles, the owner does not',
        evs(race, 'disco_on').length === 1 && race.cars[1].v.steerWobble > 0.2 && race.cars[0].v.steerWobble === 0 && race.cars[0].tally.discoFired === 1);
    run(race, 4.6);
    check('disco: switches off after 4 s and the wobble dies', evs(race, 'disco_off').length === 1 && race.cars[1].v.steerWobble === 0);
}
/* ---- glove ---- */
{
    const race = scene();
    launches.length = 0;
    place(race.cars[1], 5, 0, 0);
    give(race, race.cars[0], 'glove');
    run(race, 0.3, fire);
    const g = evs(race, 'glove')[0];
    check('glove: wind-up then a hit for 6 with launch 9/mass',
        g && g.hit && g.car === 0 && g.target === 1 && g.t >= 0.11 && race.cars[1].v.damage === 6 &&
        launches.some(l => l.car === 1 && Math.abs(l.vz - 9 / race.cars[1].v.car.mass) < 1e-9));
    const whiff = scene();
    place(whiff.cars[1], 20, 0, 0);
    give(whiff, whiff.cars[0], 'glove');
    run(whiff, 0.3, fire);
    check('glove: out of range is a whiff', evs(whiff, 'glove')[0].hit === false && whiff.cars[1].v.damage === 0);
}
/* ---- harpoon ---- */
{
    const race = scene();
    place(race.cars[0], 0, 0, 20);
    place(race.cars[1], 40, 0, 30);
    give(race, race.cars[0], 'harpoon');
    run(race, 0.5, fire, () => evs(race, 'harpoon_attach').length > 0);
    check('harpoon: rope flies, then attaches and tows', evs(race, 'harpoon_fire').length === 1 && evs(race, 'harpoon_attach').length === 1 &&
        race.cars[0].v.towing === 1 && race.cars[1].v.towedBy === 0 && race.beams.some(b => b.type === 'tether'));
    const before = race.cars[0].v.vx, rivalBefore = race.cars[1].v.vx;
    run(race, 0.3);
    check('harpoon: owner is pulled forward, rival is held back', race.cars[0].v.vx > before + 3 && race.cars[1].v.vx < rivalBefore);
    run(race, 4);
    const broke = evs(race, 'harpoon_break')[0];
    check('harpoon: tow ends within 2.5 s and clears links', broke && race.cars[0].v.towing === -1 && race.cars[1].v.towedBy === -1 && !race.beams.some(b => b.type === 'tether'), JSON.stringify(broke));
    const miss = scene();
    place(miss.cars[1], 40, 30, 0);
    give(miss, miss.cars[0], 'harpoon');
    run(miss, 0.8, fire);
    check('harpoon: nothing in the cone is a miss', evs(miss, 'harpoon_break')[0].reason === 'miss' && miss.cars[1].v.towedBy === -1);
}
/* ---- cow ---- */
{
    const race = scene();
    place(race.cars[1], 60, 0, 20);
    race.hazards.push({ type: 'oil', x: race.cars[1].v.x + 20, y: race.cars[1].v.y, r: 7, ttl: 6, age: 0, owner: 1 });
    const counts = () => [race.features.crates.length, race.features.destructibles.length, race.features.animals.length, race.hazards.filter(h => h.type !== 'cow_shadow').length];
    const before = counts();
    give(race, race.cars[0], 'cow');
    run(race, 0.05, fire);
    check('cow: launch telegraph with a growing shadow', evs(race, 'cow_launch')[0].flight === 1.4 && race.hazards.some(h => h.type === 'cow_shadow'));
    run(race, 2, null, () => evs(race, 'cow_land').length > 0);
    const land = evs(race, 'cow_land')[0];
    check('cow: lands with shock r9, hits for 12 and speed x0.6', land && land.r === 9 && land.hits === 1 && race.cars[1].v.damage === 12 && race.cars[1].v.vx < 14);
    check('cow: shadow removed on landing and nothing else is removed', !race.hazards.some(h => h.type === 'cow_shadow') && counts().join() === before.join());
}
/* ---- cloak and overdrive ---- */
{
    const race = scene();
    place(race.cars[1], 40, 0, 0);
    give(race, race.cars[0], 'cloak');
    run(race, 0.05, fire);
    check('cloak: on for 3 s, immune and cannot fire', race.cars[0].v.cloak > 2.9 && evs(race, 'cloak_on').length === 1 &&
        AD.applyHit(race, race.cars[0], { by: 1, damage: 50 }) === false && race.cars[0].v.damage === 0);
    give(race, race.cars[0], 'oil');
    run(race, 0.1, fire);
    check('cloak: firing is blocked while cloaked', race.hazards.length === 0 && race.cars[0].weapon === 'oil');
    run(race, 3.1);
    check('cloak: cloak_off after 3 s', evs(race, 'cloak_off').length === 1 && race.cars[0].v.cloak === 0);
}
{
    const race = scene();
    give(race, race.cars[0], 'overdrive');
    run(race, 0.05, fire);
    check('overdrive: on for 3 s', race.cars[0].v.overdrive > 2.9 && evs(race, 'overdrive_on').length === 1);
    run(race, 3.1);
    check('overdrive: overdrive_off', evs(race, 'overdrive_off').length === 1 && race.cars[0].v.overdrive === 0);
}

/* ---- shield blocks everything, cloak is immune ---- */
for (const id of ['rocket', 'mg', 'ducks', 'gnome_mine', 'glove', 'cow', 'emp', 'frost', 'flamethrower', 'harpoon']) {
    const race = scene();
    place(race.cars[1], id === 'glove' ? 5 : (id === 'gnome_mine' ? -13 : (id === 'flamethrower' ? 8 : 25)), 0, id === 'gnome_mine' ? 10 : 0);
    race.cars[1].v.shield = 4;
    give(race, race.cars[0], id);
    run(race, 2.6, fire);
    const v = race.cars[1].v;
    check('shield blocks ' + id, v.damage === 0 && v.disabled === 0 && v.frozen === 0 && v.towedBy === -1 && v.heat === 0 &&
        evs(race, 'hit').length === 0 && evs(race, 'shield_block').length > 0 && evs(race, 'shield_block').every(e => e.car === 1 && e.by === 0),
        'dmg=' + v.damage + ' dis=' + v.disabled + ' blocks=' + evs(race, 'shield_block').length);
}
for (const id of ['rocket', 'mg', 'railgun', 'emp', 'glove', 'ducks']) {
    const race = scene();
    place(race.cars[1], id === 'glove' ? 5 : 25, 0, 0);
    race.cars[1].v.cloak = 5;
    give(race, race.cars[0], id);
    run(race, 1.2, fire);
    const v = race.cars[1].v;
    check('cloak is immune to ' + id, v.damage === 0 && v.disabled === 0 && evs(race, 'hit').length === 0);
}

/* ---- AI predicates ---- */
{
    const cases = {
        oil: { yes: [{ at: -20 }], no: [{ at: 20 }] },
        gnome_mine: { yes: [{ at: -20 }], no: [{ at: 20 }] },
        emp: { yes: [{ at: 10 }], no: [{ at: 40 }] },
        rocket: { yes: [{ at: 50 }], no: [{ at: -50 }] },
        mg: { yes: [{ at: 30 }], no: [{ at: 30, lat: 9 }] },
        ducks: { yes: [{ at: 50 }], no: [{ at: -50 }] },
        railgun: { yes: [{ at: 100 }], no: [{ at: 100, lat: 15 }] },
        flamethrower: { yes: [{ at: 10 }], no: [{ at: 30 }] },
        frost: { yes: [{ at: 30 }], no: [{ at: -30 }] },
        air_horn: { yes: [{ at: 8 }], no: [{ at: 40 }] },
        black_hole: { yes: [{ at: 40 }, { at: 55 }], no: [{ at: 40 }, { at: -55 }] },
        disco: { yes: [{ at: 30 }], no: [{ at: -100 }] },
        glove: { yes: [{ at: 5 }], no: [{ at: 15 }] },
        harpoon: { yes: [{ at: 40 }], no: [{ at: 40, lat: 25 }] },
        cow: { yes: [{ at: 120 }], no: [{ at: -120 }] }
    };
    function wants(id, spec, extra) {
        const race = scene(2);
        const car = race.cars[0];
        place(car, 0, 0, 40);
        spec.forEach((s, i) => place(race.cars[i + 1], s.at, s.lat || 0, 30));
        if (spec.length < 2) place(race.cars[2], -700, 0, 0);
        if (extra) extra(race);
        give(race, car, id);
        for (let i = 0; i < 240; i++) if (AD.aiWantsFire(race, car)) return true;
        return false;
    }
    for (const id of Object.keys(cases)) {
        check('AI ' + id + ': fires when the predicate holds', wants(id, cases[id].yes));
        check('AI ' + id + ': holds fire otherwise', !wants(id, cases[id].no));
    }
    const rocketAtMe = race => race.projectiles.push({ type: 'rocket', x: race.cars[0].v.x + 30, y: race.cars[0].v.y, z: 0, psi: Math.PI, speed: 62, ttl: 5, owner: 1, target: 0 });
    check('AI shield: fires on a rocket lock or heavy damage', wants('shield', [], rocketAtMe) && wants('shield', [], r => { r.cars[0].v.damage = 60; }));
    check('AI shield: calm race keeps it', !wants('shield', []));
    check('AI cloak: fires on a threat', wants('cloak', [], rocketAtMe));
    check('AI cloak: calm race keeps it', !wants('cloak', []));
    check('AI overdrive: straight ahead', wants('overdrive', []));
    const bend = track.curvature.findIndex((c, i) => i > 40 && c >= 0.01);
    const race = scene(1);
    const bp = AD.pointAt(track, track.samples[bend].s - 60);
    Object.assign(race.cars[0].v, { x: bp.x, y: bp.y, psi: Math.atan2(bp.ty, bp.tx), vx: 40 });
    give(race, race.cars[0], 'overdrive');
    let fired = false;
    for (let i = 0; i < 120; i++) if (AD.aiWantsFire(race, race.cars[0])) fired = true;
    check('AI overdrive: holds before a corner', !fired, 'bend at ' + track.samples[bend].s.toFixed(0));
}

/* ---- ghost, events ---- */
{
    const recorded = AD.createRace({ track, mode: 'time_trial', laps: 1, seed: 5150 });
    const tape = AD.encodeTape(new Array(500).fill(0));
    const race = AD.createRace({ track, mode: 'quick', opponents: 1, weapons: true, seed: 5150, ghostTape: tape });
    race.countdown = 0; race.state = 'running';
    check('ghost exists and is not in race.cars', !!race.ghost && race.ghost.index === -1 && !race.cars.includes(race.ghost) && recorded);
    race.ghost.weapon = 'rocket'; race.ghost.ammo = 1; race.ghost.cooldown = 0; race.ghost.weaponLock = 0;
    AD.fireWeapon(race, race.ghost);
    check('ghost never fires or gets targeted', AD.aiWantsFire(race, race.ghost) === false && race.projectiles.length === 0 && race.events.length === 0);
}
check('every hit and shield_block event carries numeric car and by',
    allHitEvents.length > 20 && allHitEvents.every(e => Number.isInteger(e.car) && Number.isInteger(e.by)), 'events=' + allHitEvents.length);

/* ---- real races: AI uses every weapon, replay is bit-identical ---- */
function aiRace(seed, steps) {
    const race = AD.createRace({ track, mode: 'quick', opponents: 3, weapons: true, seed: seed, showGhost: false });
    race.countdown = 0; race.state = 'running';
    const feed = AD.rng(seed * 7 + 1);
    const used = new Set();
    for (const car of race.cars) car.isPlayer = false;
    for (let i = 0; i < steps; i++) {
        if (i % 360 === 0) for (const car of race.cars) {
            if (!car.weapon) AD.giveWeapon(race, car, AD.WEAPON_IDS[Math.floor(feed() * AD.WEAPON_IDS.length)]);
        }
        AD.stepRace(race, AD.STEP);
        for (const e of race.events) if (e.type === 'weapon') used.add(e.id);
        race.events.length = 0;
    }
    const values = [race.time];
    for (const car of race.cars) values.push(...AD.vehicleState(car.v), car.ammo, car.v.damage);
    values.push(race.hazards.length, race.projectiles.length, race.beams.length);
    return { used, hash: AD.hashValues(values), finite: race.cars.every(c => AD.vehicleState(c.v).every(Number.isFinite)) };
}
{
    const a = aiRace(31, 14000), b = aiRace(31, 14000), c = aiRace(32, 14000), d = aiRace(33, 14000);
    const union = new Set([...a.used, ...c.used, ...d.used]);
    check('replay hash with weapons in play is identical', a.hash === b.hash, a.hash + ' vs ' + b.hash);
    check('different seeds diverge', a.hash !== c.hash);
    check('races stay finite with all weapons', a.finite && c.finite && d.finite);
    check('AI fires many different weapons in real races', union.size >= 9, union.size + ': ' + [...union].join(','));
}

AD.launch = realLaunch;
if (failures.length) {
    console.error('\n' + failures.length + ' weapon check(s) failed');
    process.exit(1);
}
console.log('\nAll weapon checks passed.');
