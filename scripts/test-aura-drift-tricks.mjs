/* Aura Drift - air, tricks, landing, drift turbo and start checks.
 * Run: node scripts/test-aura-drift-tricks.mjs
 */
import { loadSim } from './aura-drift-sim-loader.mjs';

export { loadSim };
const AD = loadSim();
const failures = [];
function check(name, condition, detail) {
    if (condition) console.log('  ok  ' + name);
    else { failures.push(name + (detail ? ' - ' + detail : '')); console.log(' FAIL ' + name + (detail ? ' - ' + detail : '')); }
}
const none = () => ({ throttle: 0, brake: 0, steer: 0, handbrake: 0, fire: 0, boost: 0 });

function newRace(options) {
    const track = AD.buildTrack(AD.trackById('alpenpass'));
    const race = AD.createRace(Object.assign({ track, mode: 'time_trial', opponents: 0, seed: 77 }, options));
    race.countdown = 0;
    race.state = 'running';
    return race;
}
function events(race, type) { return race.events.filter(e => e.type === type); }

/* Launch the player at speed along its heading and run until it lands. */
function flight(inputFn, vz, car, preset) {
    const race = newRace(car ? { playerCar: car } : {});
    const p = race.cars[0], v = p.v;
    v.vx = 50;
    if (preset) preset(v, p);
    AD.launch(race, p, vz, 'ramp');
    let steps = 0;
    while (v.airborne && steps < 2000) {
        race.playerInput = inputFn(steps);
        AD.stepRace(race, AD.STEP);
        steps++;
    }
    return { race, p, v, steps, airTime: steps * AD.STEP };
}

console.log('Aura Drift tricks');

/* Flight duration */
{
    const f = flight(() => none(), 10);
    check('launch at 50 m/s with vz 10 stays up about 2*vz/16 s', Math.abs(f.airTime - 1.25) < 0.06, f.airTime.toFixed(3));
    check('an untouched flight lands level and perfect', events(f.race, 'landing')[0] && events(f.race, 'landing')[0].quality === 'perfect',
        JSON.stringify(events(f.race, 'landing')));
    const r = newRace(), q = r.cars[0], w = q.v;
    w.vx = 30;
    AD.launch(r, q, 8, 'ramp');
    w.psi = 0.3; w.airTime = 1; w.z = 0.001; w.vz = -5; w.wx = 30; w.wy = 0;
    r.playerInput = none();
    AD.stepRace(r, AD.STEP);
    check('landing converts world velocity to the body frame',
        Math.abs(w.vx - 30 * Math.cos(0.3)) < 0.6 && Math.abs(w.vy + 30 * Math.sin(0.3)) < 0.6 && !w.airborne,
        'vx=' + w.vx + ' vy=' + w.vy);
    check('takeoff event is emitted', events(f.race, 'takeoff').length === 1 && events(f.race, 'takeoff')[0].source === 'ramp');
}

/* Roll */
{
    const f = flight(() => Object.assign(none(), { steer: 1 }), 10);
    check('steer +1 for a full flight rolls positive', f.v.rollAccum > 0 || events(f.race, 'trick').length > 0);
    check('steer +1 gives barrel_roll', (f.p.tricks.counts.barrel_roll || 0) === 1, JSON.stringify(f.p.tricks.counts));
    const t = events(f.race, 'trick').find(e => e.id === 'barrel_roll');
    check('trick event carries car, points, chain', t && t.car === 0 && t.points === 400 && t.chain === 1, JSON.stringify(t));
    check('air_time pays on landing', (f.p.tricks.counts.air_time || 0) === 1);
}
/* Pitch */
{
    const f = flight(i => (i > 2 ? Object.assign(none(), { throttle: 1 }) : none()), 10);
    check('throttle in the air gives negative pitch (front flip)', f.p.tricks.counts.front_flip === 1, JSON.stringify(f.p.tricks.counts));
    const g = flight(i => (i > 2 ? Object.assign(none(), { brake: 1 }) : none()), 10);
    check('brake in the air gives a back flip', g.p.tricks.counts.back_flip === 1, JSON.stringify(g.p.tricks.counts));
    const held = flight(() => Object.assign(none(), { throttle: 1 }), 10, null, v => { v.throttle = 1; });
    check('a pedal held since takeoff is not a flip command', !held.p.tricks.counts.front_flip &&
        events(held.race, 'landing')[0].quality === 'perfect');
    const both = flight(i => Object.assign(none(), { steer: 1, throttle: i > 2 ? 1 : 0 }), 12);
    check('roll plus flip in one flight is a corkscrew', both.p.tricks.counts.corkscrew === 1, JSON.stringify(both.p.tricks.counts));
}
/* Spin */
{
    const f = flight(() => Object.assign(none(), { steer: 1, handbrake: 1 }), 10);
    check('handbrake plus steer is a flat spin, not a roll', f.p.tricks.counts.flat_spin >= 1 && !f.p.tricks.counts.barrel_roll,
        JSON.stringify(f.p.tricks.counts));
    const g = flight(() => Object.assign(none(), { steer: 1, handbrake: 1 }), 14);
    check('two rotations replace flat_spin by spin_720', g.p.tricks.counts.spin_720 === 1 && !g.p.tricks.counts.flat_spin,
        JSON.stringify(g.p.tricks.counts));
    const d = flight(() => Object.assign(none(), { steer: 1 }), 16);
    check('double_barrel replaces barrel_roll', d.p.tricks.counts.double_barrel === 1 && !d.p.tricks.counts.barrel_roll,
        JSON.stringify(d.p.tricks.counts));
}

/* All four landing qualities, and their effects */
function landWith(roll, extra) {
    const race = newRace();
    const p = race.cars[0], v = p.v;
    v.vx = 40;
    AD.launch(race, p, 8, 'ramp');
    v.airTime = 1.0; v.z = 0.001; v.vz = -5; v.roll = roll;
    if (extra) extra(v);
    race.playerInput = Object.assign(none(), { steer: 1 });
    AD.stepRace(race, AD.STEP);
    return { race, p, v, ev: events(race, 'landing')[0] };
}
{
    const perfect = landWith(0.1), good = landWith(0.6), wobble = landWith(1.2), crash = landWith(2.0);
    check('perfect landing reachable', perfect.ev && perfect.ev.quality === 'perfect');
    check('good landing reachable', good.ev && good.ev.quality === 'good');
    check('wobble landing reachable', wobble.ev && wobble.ev.quality === 'wobble');
    check('crash landing reachable', crash.ev && crash.ev.quality === 'crash');
    check('perfect: nitro bonus and burst 0.5 s', perfect.ev.nitro >= 0.2 && perfect.v.burst > 0.4 && perfect.v.burstPower === 1.25,
        'burst=' + perfect.v.burst);
    check('wobble: landWobble 0.9', wobble.v.landWobble > 0.8 && wobble.v.landWobble <= 0.9);
    check('crash: speed x0.7, wobble 1.4, damage, 50 consolation', crash.v.vx < 40 * 0.75 && crash.v.landWobble > 1.3 &&
        crash.v.damage >= 6 && crash.ev.points === 50 && crash.p.tally.crashLandings === 1, 'vx=' + crash.v.vx);
    check('landing points scale with quality',
        perfect.ev.points > good.ev.points && good.ev.points > wobble.ev.points && wobble.ev.points > 0,
        [perfect.ev.points, good.ev.points, wobble.ev.points].join('/'));
    check('perfect landing is counted', perfect.p.tally.perfectLandings === 1);
    check('landing points reach the style bank', perfect.p.style.banked > 0);
    const couch = AD.CARS.findIndex(c => c.id === 'couch');
    const a = landWith(0.1), b = (() => {
        const race = newRace({ playerCar: 'couch' });
        const p = race.cars[0], v = p.v; v.vx = 40;
        AD.launch(race, p, 8, 'ramp');
        v.airTime = 1.0; v.z = 0.001; v.vz = -5;
        race.playerInput = none();
        AD.stepRace(race, AD.STEP);
        return events(race, 'landing')[0];
    })();
    check('Couch-Potato earns x1.25 trick points', couch >= 0 && Math.abs(b.points / a.ev.points - 1.25) < 0.02, a.ev.points + ' vs ' + b.points);
    const q = AD.landingQuality({ roll: 0, pitch: 0, psi: 0, wx: 30, wy: 0 });
    check('landingQuality(v) is pure on a vehicle', q === 'perfect');
    check('sideways landing is a crash', AD.landingQuality({ roll: 0, pitch: 0, psi: 2.6, wx: 30, wy: 0 }) === 'crash');
}

/* Drift turbo levels */
function driftRun(seconds, car) {
    const race = newRace(car ? { playerCar: car } : {});
    const p = race.cars[0], v = p.v;
    const steps = Math.round(seconds / AD.STEP);
    for (let i = 0; i < steps; i++) {
        v.vx = 30; v.vy = 8; v.slip = 3;
        AD.updateVehicleTricks(race, p, none(), AD.STEP);
    }
    const levelAtEnd = v.driftLevel;
    v.vy = 0; v.slip = 0;
    for (let i = 0; i < 40; i++) AD.updateVehicleTricks(race, p, none(), AD.STEP);
    return { race, p, v, levelAtEnd };
}
{
    const expect = [[1.1, 1, 0.5, 1.25, 'drift_blue'], [2.4, 2, 0.9, 1.35, 'drift_orange'], [3.8, 3, 1.4, 1.5, 'drift_pink']];
    for (const [sec, level, burst, power, id] of expect) {
        const r = driftRun(sec);
        const e = events(r.race, 'turbo_burst')[0];
        check('drift charge ' + sec + ' s releases level ' + level + ' turbo', e && e.level === level && r.p.tricks.counts[id] === 1 &&
            Math.abs(r.v.burst - burst) < 0.12 && r.v.burstPower === power, JSON.stringify(e) + ' burst=' + r.v.burst);
    }
    const none1 = driftRun(0.8);
    check('a short slide under 1.0 s pays no turbo', events(none1.race, 'turbo_burst').length === 0 && none1.v.burst === 0);
    const lumen = driftRun(1.1, 'lumen');
    check('Lumen drift turbo lasts x1.2 longer', lumen.v.burst > 0.5 * 1.2 - 0.12, 'burst=' + lumen.v.burst);
    const levels = events(driftRun(3.8).race, 'drift_level').map(e => e.level).join();
    check('drift_level events step 1,2,3', levels === '1,2,3', levels);

    /* A wall hit zeroes the charge. */
    const race = newRace(), p = race.cars[0], v = p.v;
    for (let i = 0; i < 160; i++) { v.vx = 30; v.vy = 8; v.slip = 3; AD.updateVehicleTricks(race, p, none(), AD.STEP); }
    v.wallContacts++;
    v.vx = 30; v.vy = 8; v.slip = 3;
    AD.updateVehicleTricks(race, p, none(), AD.STEP);
    check('wall hit zeroes the drift charge', v.driftCharge <= AD.STEP * 1.5, 'charge=' + v.driftCharge);
    /* Takeoff zeroes it too. */
    v.driftCharge = 2; v.driftLevel = 1;
    AD.launch(race, p, 8, 'ramp');
    check('takeoff zeroes the drift charge', v.driftCharge === 0 && v.driftLevel === 0);
}

/* Perfect start window */
function startRun(holdFrom) {
    const race = AD.createRace({ track: AD.buildTrack(AD.trackById('alpenpass')), mode: 'time_trial', opponents: 0, seed: 5 });
    let guard = 0;
    while (race.state === 'countdown' && guard++ < 2000) {
        race.playerInput = Object.assign(none(), { throttle: race.countdown <= holdFrom ? 1 : 0 });
        AD.stepRace(race, AD.STEP);
    }
    return race;
}
{
    const perfect = startRun(0.2);
    check('throttle 0.2 s before GO is a perfect start', events(perfect, 'perfect_start').length === 1 && perfect.cars[0].v.burst > 1.0 &&
        perfect.cars[0].v.burstPower === 1.3, 'burst=' + perfect.cars[0].v.burst);
    const late = startRun(0.02);
    check('throttle only at GO is no perfect start', events(late, 'perfect_start').length === 0 && late.cars[0].v.burst === 0);
    const mid = startRun(0.5);
    check('throttle 0.5 s before GO is neither perfect nor wheelspin', events(mid, 'perfect_start').length === 0 && events(mid, 'wheelspin').length === 0);
    const spin = startRun(2.0);
    check('throttle held 2 s before GO wheelspins', events(spin, 'wheelspin').length === 1 && spin.cars[0].v.burst <= 0.6 && spin.cars[0].v.burstPower === 0.6);
    check('perfect start pays 300 style points and counts', perfect.cars[0].style.banked >= 300 && perfect.cars[0].tricks.counts.perfect_start === 1);
    const lo = startRun(0.31), hi = startRun(0.29);
    check('window edges: 0.31 s misses, 0.29 s hits', events(lo, 'perfect_start').length === 0 && events(hi, 'perfect_start').length === 1);
}

/* Slingshot, reverse hero, J-turn */
{
    const race = newRace(), p = race.cars[0], v = p.v;
    for (let i = 0; i < 160; i++) { v.vx = 40; v.slipstream = 0.6; v.draftTime = v.draftTime; AD.updateVehicleTricks(race, p, none(), AD.STEP); }
    check('draft time accumulates', v.draftTime > 1.2);
    v.slipstream = 0;
    AD.updateVehicleTricks(race, p, none(), AD.STEP);
    check('leaving a 1.2 s draft at speed slings the car', events(race, 'slingshot').length === 1 && v.burst > 0.7 && v.burstPower === 1.25);

    const r2 = newRace(), p2 = r2.cars[0], v2 = p2.v;
    for (let i = 0; i < 300; i++) { v2.vx = -10; AD.updateVehicleTricks(r2, p2, none(), AD.STEP); }
    check('reverse hero after 2 s then pays per second', events(r2, 'reverse_hero').length === 1 && p2.style.banked > 200,
        'banked=' + p2.style.banked);
    check('reverse seconds are tallied', p2.tally.reverseSeconds > 2.4);
    for (let i = 0; i < 60; i++) { v2.vx = i < 30 ? -4 : 10; AD.updateVehicleTricks(r2, p2, none(), AD.STEP); }
    check('reverse to forward within 1.5 s is a J-turn', events(r2, 'j_turn').length === 1 && p2.tricks.counts.j_turn === 1);
    const r3 = newRace(), p3 = r3.cars[0], v3 = p3.v;
    for (let i = 0; i < 20; i++) { v3.vx = -10; AD.updateVehicleTricks(r3, p3, none(), AD.STEP); }
    for (let i = 0; i < 300; i++) { v3.vx = i < 250 ? 0 : 10; AD.updateVehicleTricks(r3, p3, none(), AD.STEP); }
    check('a slow turn-around is no J-turn', events(r3, 'j_turn').length === 0);
}

/* AI plans */
{
    let counts = { roll: 0, spin: 0, none: 0 }, rolledAll = true;
    for (let seed = 1; seed <= 24; seed++) {
        const track = AD.buildTrack(AD.trackById('alpenpass'));
        const race = AD.createRace({ track, mode: 'quick', opponents: 2, seed });
        race.countdown = 0; race.state = 'running';
        const ai = race.cars[1];
        ai.v.vx = 50;
        AD.launch(race, ai, 10, 'ramp');
        counts[ai.tricks.plan.kind]++;
        let n = 0;
        while (ai.v.airborne && n++ < 1000) {
            AD.aiAirInput(race, ai, ai.input);
            AD.stepAir(ai, ai.input, AD.STEP, race);
        }
        const ev = events(race, 'landing')[0];
        if (ev && ev.quality === 'crash') rolledAll = false;
    }
    check('AI picks all three plans over 24 seeds', counts.roll > 0 && counts.spin > 0 && counts.none > 0, JSON.stringify(counts));
    check('AI never crashes a planned flight', rolledAll);
    const race = AD.createRace({ track: AD.buildTrack(AD.trackById('alpenpass')), mode: 'quick', opponents: 1, seed: 3 });
    race.countdown = 0; race.state = 'running';
    const ai = race.cars[1]; ai.v.vx = 50;
    AD.launch(race, ai, 7, 'hit');
    check('a hit launch never plans a trick', ai.tricks.plan.kind === 'none');
}

/* Ghost: same physics, no scoring */
{
    const track = AD.buildTrack(AD.trackById('alpenpass'));
    const tape = []; for (let i = 0; i < 1500; i++) tape.push(AD.packInput(Object.assign(none(), { throttle: 1 })));
    const race = AD.createRace({ track, mode: 'time_trial', opponents: 0, seed: 9, ghostTape: tape });
    let ok = true;
    try {
        while (race.state === 'countdown') AD.stepRace(race, AD.STEP);
        const g = race.ghost; g.v.vx = 40;
        race.muteEvents = true; AD.launch(race, g, 8, 'ramp'); race.muteEvents = false;
        for (let i = 0; i < 400; i++) AD.stepRace(race, AD.STEP);
    } catch (e) { ok = false; console.log(e); }
    check('ghost flies without scoring or events', ok && race.ghost.tally === null && race.ghost.style === null &&
        events(race, 'takeoff').length === 0 && events(race, 'landing').length === 0 && !race.ghost.v.airborne);
}

/* Determinism */
function replayHash() {
    const race = newRace({ seed: 31 });
    const p = race.cars[0];
    const values = [];
    for (let i = 0; i < 4000; i++) {
        race.playerInput = Object.assign(none(), {
            throttle: 1, steer: Math.sin(i / 90) * 0.9, handbrake: i % 700 > 600 ? 1 : 0
        });
        if (i % 500 === 250) AD.launch(race, p, 9, 'ramp');
        AD.stepRace(race, AD.STEP);
        if (i % 50 === 0) values.push(...AD.vehicleState(p.v), p.v.driftCharge, p.v.rollAccum, p.style ? p.style.banked : 0);
    }
    values.push(race.events.length, Object.keys(p.tricks.counts).length);
    return AD.hashValues(values) + '/' + JSON.stringify(p.tricks.counts);
}
{
    const a = replayHash(), b = replayHash();
    check('determinism: two runs hash equal', a === b, a + ' vs ' + b);
    check('determinism run exercised tricks', /air_time/.test(a), a);
}

if (failures.length) {
    console.log('\n' + failures.length + ' tricks check(s) failed:\n' + failures.join('\n'));
    process.exit(1);
}
console.log('\nAll tricks checks passed.');
