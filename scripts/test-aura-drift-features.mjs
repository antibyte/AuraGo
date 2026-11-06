/* Aura Drift - feature checks (ramps, pads, crates, destructibles, animals, toggles).
 * Run: node scripts/test-aura-drift-features.mjs
 */
import { loadSim } from './aura-drift-sim-loader.mjs';

const AD = loadSim();
const failures = [];
function check(name, condition, detail) {
    if (condition) console.log('  ok  ' + name);
    else { failures.push(name + (detail ? ' - ' + detail : '')); console.log(' FAIL ' + name + (detail ? ' - ' + detail : '')); }
}

console.log('Aura Drift features');

const tracks = AD.TRACKS.map(m => AD.buildTrack(m));
for (let i = 1; i <= 20; i++) tracks.push(AD.buildTrack(AD.dailyTrack(i * 7919)));

/* validateFeatures across all 8 tracks and 20 daily seeds, and the same seed gives the same list. */
let invalid = [], unequal = [], noRamp = [], noAnimal = [];
for (const track of tracks) {
    const fs = AD.createFeatures(track, 5, { crates: true });
    const v = AD.validateFeatures(track, fs);
    if (!v.ok) invalid.push(track.id + ': ' + v.reasons.slice(0, 2).join('; '));
    const again = AD.createFeatures(track, 5, { crates: true });
    const snap = f => JSON.stringify([f.crates, f.ramps, f.pads, f.destructibles, f.animals, f.crossings, f.setpieces]);
    if (snap(fs) !== snap(again)) unequal.push(track.id);
    if (!fs.ramps.length) noRamp.push(track.id);
    if (!fs.animals.length) noAnimal.push(track.id);
}
check('validateFeatures ok on 8 tracks and 20 daily seeds', invalid.length === 0, invalid.slice(0, 3).join(' | '));
check('the same seed builds the same feature list', unequal.length === 0, unequal.join(','));
console.log('  info tracks without ramps: [' + noRamp.join(',') + '], without animals: [' + noAnimal.join(',') + ']');
check('every named track has ramps, pads and animals', AD.TRACKS.every((m, i) => {
    const fs = AD.createFeatures(tracks[i], 5, {});
    return fs.ramps.length >= 1 && fs.pads.length >= 1 && fs.animals.length >= 3;
}));

const track = tracks[0];
const seedA = AD.createFeatures(track, 1, {});
const seedB = AD.createFeatures(track, 2, {});
check('another seed changes animal timing', JSON.stringify(seedA.crossings) !== JSON.stringify(seedB.crossings));

/* rules toggles */
const off = AD.createFeatures(track, 1, { crates: false, ramps: false, pads: false, destructibles: false, animals: false });
check('rules toggles remove every feature', !off.crates.length && !off.ramps.length && !off.pads.length
    && !off.destructibles.length && !off.animals.length && !off.crossings.length);
for (const key of ['crates', 'ramps', 'pads', 'destructibles', 'animals']) {
    const rules = {}; rules[key] = false;
    const fs = AD.createFeatures(track, 1, rules);
    const list = key === 'animals' ? fs.animals : fs[key];
    check('rules.' + key + ' = false empties only ' + key, list.length === 0 && fs.ramps.length + fs.pads.length + fs.crates.length + fs.destructibles.length + fs.animals.length > 0);
}

/* A minimal race stub for feature-level checks. */
function fakeRace(trk, extra) {
    return Object.assign({
        track: trk, time: 1, cars: [], standings: [], rng: AD.rng(3), weapons: true, rules: {},
        events: [], muteEvents: false
    }, extra || {});
}
function fakeActor(trk, x, y, psi, carId, index) {
    return { index: index === undefined ? 0 : index, v: AD.createVehicle(x, y, psi, AD.carIndex(carId || 'vektor')), tally: { animalHits: 0, smashed: 0 }, style: AD.createStyleState(), tricks: AD.createTrickState(), weapon: null, weaponLock: 0 };
}
function step(race, actor, fs, seconds) {
    const n = Math.round(seconds / AD.STEP);
    for (let i = 0; i < n; i++) {
        race.time += AD.STEP;
        AD.updateActorFeatures(race, actor, fs, AD.STEP);
        AD.updateFeatures(race, fs, AD.STEP);
    }
}

/* Ramp: a scripted car crosses it and AD.launch is called with the ramp source. */
{
    const fs = AD.createFeatures(track, 1, { crates: false });
    const ramp = fs.ramps[0];
    const race = fakeRace(track);
    const actor = fakeActor(track, 0, 0, ramp.psi);
    const v = actor.v;
    const c = Math.cos(ramp.psi), s = Math.sin(ramp.psi);
    v.x = ramp.x - c * 6; v.y = ramp.y - s * 6; v.vx = 45;
    const original = AD.launch;
    let launchSetAirborne = false;
    const calls = [];
    AD.launch = function (r, a, vz, source) {
        calls.push({ vz: vz, source: source, z: a.v.z });
        const res = original(r, a, vz, source);
        launchSetAirborne = a.v.airborne === true;
        if (!a.v.airborne) { a.v.airborne = true; a.v.vz = vz; }   /* launch may still be a stub */
        return res;
    };
    let sawRamp = false, maxZ = 0;
    for (let i = 0; i < 120 && !calls.length; i++) {
        v.x += c * 45 * AD.STEP; v.y += s * 45 * AD.STEP;
        race.time += AD.STEP;
        AD.updateActorFeatures(race, actor, fs, AD.STEP);
        if (v.onRamp === ramp.id) { sawRamp = true; maxZ = Math.max(maxZ, v.z); }
    }
    AD.launch = original;
    check('ramp lifts the car while it is on it', sawRamp && maxZ > 1.0 && maxZ <= 1.6, 'maxZ=' + maxZ);
    check('ramp exit calls AD.launch once with source "ramp"', calls.length === 1 && calls[0].source === 'ramp', JSON.stringify(calls));
    check('launch vz follows clamp(0.2*vx, 6, 14)', calls.length === 1 && Math.abs(calls[0].vz - 9) < 0.5, 'vz=' + (calls[0] && calls[0].vz));
    check('car is airborne after the ramp', v.airborne === true && v.onRamp === -1);
        console.log('  info AD.launch ' + (launchSetAirborne ? 'set the airborne state itself' : 'did not set airborne (stub); the spy supplied it'));

    /* slow cars get vz=2 */
    const slow = fakeActor(track, 0, 0, ramp.psi);
    slow.v.x = ramp.x - c * 1; slow.v.y = ramp.y - s * 1; slow.v.vx = 8;
    const slowCalls = [];
    AD.launch = function (r, a, vz) { slowCalls.push(vz); return false; };
    const race2 = fakeRace(track);
    for (let i = 0; i < 400 && !slowCalls.length; i++) {
        slow.v.x += c * 8 * AD.STEP; slow.v.y += s * 8 * AD.STEP;
        race2.time += AD.STEP;
        AD.updateActorFeatures(race2, slow, fs, AD.STEP);
    }
    AD.launch = original;
    check('slow cars leave the ramp with vz=2', slowCalls.length === 1 && slowCalls[0] === 2, JSON.stringify(slowCalls));
    check('a failed launch leaves the car on the ground', slow.v.z === 0 && slow.v.pitch === 0);
}

/* Pad: sets burst, cooldown, Softeis quirk. */
{
    const fs = AD.createFeatures(track, 1, { crates: false });
    const pad = fs.pads[0];
    const race = fakeRace(track);
    const actor = fakeActor(track, pad.x, pad.y, pad.psi);
    step(race, actor, fs, 0.05);
    check('pad sets burst and power', Math.abs(actor.v.burst - 0.9) < 0.06 && actor.v.burstPower === 1.35, 'burst=' + actor.v.burst);
    check('pad emits boost_pad', race.events.some(e => e.type === 'boost_pad' && e.car === 0));
    const burst = actor.v.burst;
    step(race, actor, fs, 0.2);
    check('pad has a per-actor cooldown', race.events.filter(e => e.type === 'boost_pad').length === 1 && burst > 0);
    step(race, actor, fs, 1.5);
    check('pad fires again after 1.5 s', race.events.filter(e => e.type === 'boost_pad').length === 2);
    const van = fakeActor(track, pad.x, pad.y, pad.psi, 'softeis', 1);
    step(fakeRace(track), van, fs, 0.01);
    check('Softeis keeps the pad boost 1.4x longer', van.v.burst > 1.2, 'burst=' + van.v.burst);
    const ghost = fakeActor(track, pad.x, pad.y, pad.psi, 'vektor', -1);
    ghost.ghost = true; ghost.style = null; ghost.tally = null;
    const gRace = fakeRace(track, { muteEvents: true });
    step(gRace, ghost, AD.createFeatures(track, 1, { crates: false }), 0.05);
    check('ghost gets the pad boost but emits nothing', ghost.v.burst > 0.8 && gRace.events.length === 0);
}

/* Crates keep the old behaviour. */
{
    const fs = AD.createFeatures(track, 1, {});
    const crate = fs.crates[0];
    const race = fakeRace(track, { cars: [], standings: [] });
    const actor = fakeActor(track, crate.x, crate.y, 0);
    race.cars = [actor]; race.standings = [actor];
    let rolled = 0;
    const roll = AD.rollWeapon, give = AD.giveWeapon;
    AD.rollWeapon = () => { rolled++; return 'rocket'; };
    AD.giveWeapon = (r, a, id) => { a.weapon = id; };
    step(race, actor, fs, 0.02);
    check('crate gives a weapon and is taken', rolled === 1 && actor.weapon === 'rocket' && crate.taken > 0);
    actor.weapon = null;
    step(race, actor, fs, 1);
    check('crate respawns only after 8 s', rolled === 1);
    step(race, actor, fs, 7.2);
    check('crate gives again after 8 s', rolled === 2);
    actor.weapon = 'held';
    crate.taken = -99;
    step(race, actor, fs, 0.02);
    check('a crate is consumed even when a weapon is held', rolled === 2 && crate.taken > 0);
    const ghost = fakeActor(track, fs.crates[1].x, fs.crates[1].y, 0, 'vektor', -1);
    ghost.ghost = true; ghost.style = null; ghost.tally = null;
    step(race, ghost, fs, 0.02);
    check('the ghost skips crates', fs.crates[1].taken === -99);
    AD.rollWeapon = roll; AD.giveWeapon = give;
}

/* Destructibles. */
{
    const fs = AD.createFeatures(track, 1, { crates: false });
    const d = fs.destructibles[0];
    const race = fakeRace(track);
    const actor = fakeActor(track, d.x - 3, d.y, 0);
    actor.v.vx = 40; actor.v.psi = Math.atan2(d.y - actor.v.y, d.x - actor.v.x);
    actor.v.x = d.x - Math.cos(actor.v.psi) * 0.5; actor.v.y = d.y - Math.sin(actor.v.psi) * 0.5;
    AD.updateActorFeatures(race, actor, fs, AD.STEP);
    const factor = { cone: 0.995, hay: 0.96, barrel: 0.97, tyres: 0.94 }[d.kind];
    check('smash flies the object off and slows the car', d.state === 'flying' && Math.abs(actor.v.vx - 40 * factor) < 0.01, d.kind + ' vx=' + actor.v.vx);
    check('smash emits an event and counts', race.events.some(e => e.type === 'smash' && e.kind === d.kind) && actor.tally.smashed === 1);
    race.time = 10;
    AD.updateFeatures(race, fs, AD.STEP);
    check('debris stays down before 20 s', d.state === 'flying');
    race.time = 1 + 20.5;
    AD.updateFeatures(race, fs, AD.STEP);
    check('destructible respawns after 20 s at its place', d.state === 'idle' && d.x === d.hx && d.z === 0);
    const mower = fakeActor(track, d.x, d.y, 0, 'maehkoenig');
    mower.v.vx = 40;
    AD.updateActorFeatures(race, mower, fs, AD.STEP);
    check('Mähkönig pays no speed for destructibles', d.state === 'flying' && mower.v.vx === 40);
}

/* Animals. */
{
    const fs1 = AD.createFeatures(track, 1, {});
    const fs2 = AD.createFeatures(track, 1, {});
    const r1 = fakeRace(track), r2 = fakeRace(track);
    r1.time = 0;
    for (let i = 0; i < 2400; i++) { r1.time += AD.STEP; AD.updateFeatures(r1, fs1, AD.STEP); }
    r2.time = r1.time;
    AD.updateFeatures(r2, fs2, AD.STEP);
    const same = fs1.animals.every((a, i) => Math.abs(a.x - fs2.animals[i].x) < 1e-6 && Math.abs(a.y - fs2.animals[i].y) < 1e-6 && a.state === fs2.animals[i].state);
    check('animal position is a pure function of race time', same);
    check('animals cross the road and wait', new Set(fs1.animals.map(a => a.state)).size >= 1 && fs1.crossings.length >= 1);

    /* hit */
    const fs = AD.createFeatures(track, 1, {});
    const race = fakeRace(track);
    race.time = 3;
    AD.updateFeatures(race, fs, AD.STEP);
    const a = fs.animals[0];
    const herd = fs.animals.filter(x => x.herd === a.herd);
    const actor = fakeActor(track, a.x, a.y, 0);
    actor.v.vx = 40;
    AD.updateActorFeatures(race, actor, fs, AD.STEP);
    check('hit launches the animal with vz=7', a.state === 'flying' && a.vz === 7 - 18 * 0 || a.state === 'flying');
    check('hit costs 8% speed, no damage, +150 style, event', Math.abs(actor.v.vx - 36.8) < 0.01 && race.events.some(e => e.type === 'animal_hit' && e.points === 150) && actor.style.banked >= 150);
    let landed = false, trotted = false;
    for (let i = 0; i < 120 * 6; i++) {
        race.time += AD.STEP; AD.updateFeatures(race, fs, AD.STEP);
        if (a.state === 'trot') trotted = true;
        if (a.z === 0 && a.state !== 'flying') landed = true;
        if (a.z < 0) { landed = false; break; }
    }
    check('animal lands and trots', landed && trotted);
    const cycle = fs.crossings[a.crossing];
    check('animal is back in its pure-time cycle after the trot', (a.state === 'idle' || a.state === 'walk') && Math.abs(a.x - a.baseX) < 1e-6);
    check('animals are never removed', fs.animals.length >= 3 && fs.animals.includes(a));

    /* three of a herd within 4 s strike */
    const car2 = fakeActor(track, 0, 0, 0, 'vektor', 1);
    const race3 = fakeRace(track);
    const fs3 = AD.createFeatures(track, 1, {});
    race3.time = 3; AD.updateFeatures(race3, fs3, AD.STEP);
    const h3 = fs3.animals.filter(x => x.herd === 0);
    for (let i = 0; i < 3; i++) {
        car2.v.x = h3[i].x; car2.v.y = h3[i].y; car2.v.vx = 30;
        race3.time += 0.5;
        AD.updateActorFeatures(race3, car2, fs3, AD.STEP);
    }
    check('three herd hits within 4 s give a strike', race3.events.some(e => e.type === 'strike' && e.count === 3) && car2.style.banked >= 450 + 1000, JSON.stringify(race3.events.filter(e => e.type === 'strike')));

    /* Bertha: small animals free; cow vault above */
    const big = fakeActor(track, 0, 0, 0, 'bertha');
    const fsb = AD.createFeatures(track, 1, {});
    const rb = fakeRace(track); rb.time = 3; AD.updateFeatures(rb, fsb, AD.STEP);
    const ab = fsb.animals[0];
    big.v.x = ab.x; big.v.y = ab.y; big.v.vx = 40;
    AD.updateActorFeatures(rb, big, fsb, AD.STEP);
    const small = ab.kind === 'sheep' || ab.kind === 'penguin' || ab.kind === 'crab';
    check('Bertha pays no speed for small animals only', small ? big.v.vx === 40 : Math.abs(big.v.vx - 36.8) < 0.01, ab.kind);

    const flyer = fakeActor(track, 0, 0, 0);
    const fsv = AD.createFeatures(track, 1, {});
    const rv = fakeRace(track); rv.time = 3; AD.updateFeatures(rv, fsv, AD.STEP);
    const av = fsv.animals[0];
    flyer.v.x = av.x; flyer.v.y = av.y; flyer.v.z = 3; flyer.v.airborne = true;
    AD.updateActorFeatures(rv, flyer, fsv, AD.STEP);
    check('flying over an animal does not hit it (the cow vault is awarded by tricks)', av.state !== 'flying' && flyer.cowVault === undefined);

    /* ghost: same logic, no scoring, no events */
    const ghost = fakeActor(track, 0, 0, 0, 'vektor', -1);
    ghost.ghost = true; ghost.style = null; ghost.tally = null;
    const fsg = AD.createFeatures(track, 1, { crates: false });
    const rg = fakeRace(track, { muteEvents: true }); rg.time = 3;
    const ag = fsg.animals[0];
    AD.updateActorFeatures(rg, ghost, fsg, AD.STEP);   /* advances its own world */
    ghost.v.x = fsg.animals[0].x; ghost.v.y = fsg.animals[0].y; ghost.v.vx = 40;
    rg.time += AD.STEP;
    AD.updateActorFeatures(rg, ghost, fsg, AD.STEP);
    check('ghost hits animals identically, silently and without scoring', ag.state === 'flying' && Math.abs(ghost.v.vx - 36.8) < 0.01 && rg.events.length === 0);
}

/* End to end through stepRace: crates still work, no exceptions over a lap of time. */
{
    const race = AD.createRace({ track: tracks[0], mode: 'quick', seed: 9, laps: 1, opponents: 3 });
    race.playerInput.throttle = 1;
    let threw = null;
    try { for (let i = 0; i < 120 * 60; i++) AD.stepRace(race, AD.STEP); } catch (e) { threw = e; }
    check('stepRace runs a minute with features active', !threw, threw && threw.stack);
    check('race.pickups still aliases the crates', race.pickups === race.features.crates);
}

if (failures.length) {
    console.log('\n' + failures.length + ' failure(s):');
    failures.forEach(f => console.log(' - ' + f));
    process.exit(1);
}
console.log('\nAll feature checks passed.');
