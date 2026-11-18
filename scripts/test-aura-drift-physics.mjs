/* Aura Drift - deterministic physics and race replay checks.
 * Run: node scripts/test-aura-drift-physics.mjs
 * The browser modules are plain IIFEs; a VM context supplies `window`.
 */
import { loadSim } from './aura-drift-sim-loader.mjs';

const AD = loadSim();
const failures = [];
function check(name, condition, detail) {
    if (condition) console.log('  ok  ' + name);
    else { failures.push(name + (detail ? ' — ' + detail : '')); console.log(' FAIL ' + name + (detail ? ' — ' + detail : '')); }
}

function inputTape(step) {
    const phase = step / 120;
    return {
        throttle: phase < 0.2 ? 1 : (phase % 4 < 2.6 ? 1 : 0.25),
        brake: phase % 4 >= 2.6 && phase % 4 < 3.1 ? 0.8 : 0,
        steer: Math.sin(phase * 0.9) * 0.7,
        handbrake: phase % 7 > 6.2 ? 1 : 0,
        fire: 0, boost: phase % 11 > 10 ? 1 : 0
    };
}

function runVehicle(steps, chunkDt) {
    const v = AD.createVehicle(0, 0, 0, 0);
    const scheduler = AD.createScheduler();
    const batches = Math.round(steps * AD.STEP / chunkDt);
    let step = 0;
    for (let i = 0; i < batches; i++) {
        scheduler.advance(chunkDt, dt => {
            AD.stepVehicle(v, inputTape(step), dt, { grip: 1, roll: 0.016 });
            step++;
        });
    }
    return { v, step, hash: AD.hashValues(AD.vehicleState(v)) };
}

console.log('Aura Drift physics');
const a = runVehicle(12000, 1 / 30);
const b = runVehicle(12000, 1 / 144);
check('same input tape produces identical state', a.hash === b.hash, a.hash + ' vs ' + b.hash);
check('both render rates execute the same number of steps', a.step === b.step, a.step + ' vs ' + b.step);
check('vehicle stays finite', AD.vehicleState(a.v).every(Number.isFinite));
check('speed stays inside solver bounds', a.v.vx <= 95 && a.v.vx >= -12, 'vx=' + a.v.vx);
check('yaw rate stays inside solver bounds', Math.abs(a.v.r) <= 3.2, 'r=' + a.v.r);

const ice = AD.createVehicle(0, 0, 0, 0);
const dry = AD.createVehicle(0, 0, 0, 0);
for (let i = 0; i < 240; i++) {
    AD.stepVehicle(ice, { throttle: 1, brake: 0, steer: 0.8, handbrake: 0, fire: 0, boost: 0 }, AD.STEP, { grip: AD.SURFACES.ice.grip, roll: 0.01 });
    AD.stepVehicle(dry, { throttle: 1, brake: 0, steer: 0.8, handbrake: 0, fire: 0, boost: 0 }, AD.STEP, { grip: AD.SURFACES.asphalt.grip, roll: 0.016 });
}
check('low-grip surface understeers (less yaw for the same steering)',
    Math.abs(ice.psi) < Math.abs(dry.psi) * 0.8,
    'ice=' + ice.psi.toFixed(3) + ' dry=' + dry.psi.toFixed(3));

const wallCar = AD.createVehicle(0, 30, 0, 0);
wallCar.vx = 80;
let wallOk = true;
for (let i = 0; i < 120; i++) {
    AD.stepVehicle(wallCar, { throttle: 1, brake: 0, steer: 0, handbrake: 0, fire: 0, boost: 0 }, AD.STEP, { grip: 1, roll: 0.016 });
    AD.resolveWall(wallCar, { nx: 0, ny: 1, offset: wallCar.y, halfWidth: 12 }, AD.STEP);
    if (Math.abs(wallCar.y) > 12.01) wallOk = false;
}
check('wall collision never tunnels at maximum speed', wallOk, 'y=' + wallCar.y);

function raceHash(options, steps) {
    const track = AD.buildTrack(AD.trackById('alpenpass'));
    const race = AD.createRace(Object.assign({ track, opponents: 3, weapons: true, seed: 4242 }, options));
    race.countdown = 0;
    race.state = 'running';
    for (let i = 0; i < steps; i++) {
        race.playerInput = inputTape(i);
        AD.stepRace(race, AD.STEP);
    }
    const values = [race.time];
    for (const car of race.cars) values.push(...AD.vehicleState(car.v), car.lap, car.gate);
    values.push(race.hazards.length, race.projectiles.length);
    return AD.hashValues(values);
}
const r1 = raceHash({}, 6000);
const r2 = raceHash({}, 6000);
check('full race replay is bit-identical', r1 === r2, r1 + ' vs ' + r2);
const r3 = raceHash({ seed: 4243 }, 6000);
check('different seeds produce different races', r1 !== r3);

const packed = AD.packInput({ throttle: 0.8, brake: 0.2, steer: -0.5, handbrake: 1, fire: 0, boost: 1 });
const unpacked = AD.unpackInput(packed);
check('input tape round-trips button state', unpacked.handbrake === 1 && unpacked.fire === 0 && unpacked.boost === 1);
check('input tape round-trips steering sign', unpacked.steer < -0.45 && unpacked.steer > -0.55, 'steer=' + unpacked.steer);

/* Tape codec: run-length base-36 pairs round-trip arbitrary integers. */
{
    const rand = AD.rng(1);
    const ints = [];
    for (let i = 0; i < 5000; i++) {
        const r = rand();
        /* mix of long runs and wide values, including negatives */
        ints.push(r < 0.5 && ints.length ? ints[ints.length - 1] : Math.floor((rand() - 0.2) * 134217728));
    }
    const text = AD.encodeTape(ints);
    const back = AD.decodeTape(text);
    check('tape codec round-trips 5000 ints', back.length === ints.length && back.every((n, i) => n === ints[i]),
        text.length + ' chars');
    check('empty tape round-trips', AD.decodeTape(AD.encodeTape([])).length === 0);
}

/* D1 regression: scheduled weather must actually build up (it stayed at 0). */
{
    let peak = 0;
    for (const manifest of AD.TRACKS) {
        const track = AD.buildTrack(manifest);
        const conditions = AD.createConditions(track, 90210);
        for (let i = 0; i < 120 * 150; i++) peak = Math.max(peak, conditions.update(AD.STEP).intensity);
    }
    check('weather intensity builds up during a race (D1)', peak > 0.2, 'peak=' + peak.toFixed(2));
}

/* D4 regression: a ghost replaying a recorded tape follows the recording
   car exactly (no slipstream from the live player, same input, same world). */
{
    const track = AD.buildTrack(AD.trackById('alpenpass'));
    const recorded = AD.createRace({ track, mode: 'time_trial', laps: 1, seed: 5150 });
    const positions = [];
    /* The scripted driver is the AI controller with a calm tuning. */
    recorded.cars[0].ai.aggression = 0.85;
    recorded.cars[0].ai.skill = 0.92;
    let guard = 0;
    while (recorded.state !== 'finished' && guard++ < 60000) {
        if (recorded.state === 'running') {
            /* A scripted driver: the AI controller's inputs feed the player slot. */
            const i = AD.aiInput(recorded, recorded.cars[0]);
            Object.assign(recorded.playerInput, i);
        }
        AD.stepRace(recorded, AD.STEP);
        positions.push([recorded.cars[0].v.x, recorded.cars[0].v.y]);
    }
    check('scripted time-trial lap finishes', recorded.state === 'finished' && recorded.result, 'steps=' + guard);
    const tape = recorded.result.tape;
    check('result tape includes the countdown steps', tape.length === positions.length, tape.length + ' vs ' + positions.length);
    check('time trial has no opponents and a complete tape', recorded.cars.length === 1 && recorded.result.tapeComplete);

    const replay = AD.createRace({ track, mode: 'time_trial', laps: 1, seed: 5150, ghostTape: AD.encodeTape(tape) });
    let worst = 0, compared = 0;
    for (let i = 0; i < positions.length && replay.ghost.active; i++) {
        AD.stepRace(replay, AD.STEP);
        const g = replay.ghost.v;
        compared++;
        worst = Math.max(worst, Math.abs(g.x - positions[i][0]), Math.abs(g.y - positions[i][1]));
    }
    /* Guard against a replay that never ran, not against a faster lap: the
       opponent pace tuning shortened the scripted lap below the old 10000-step
       threshold. The contract is that the whole tape replays bit-identically. */
    check('ghost matches the recorded player within 1e-6', compared >= tape.length - 2 && worst < 1e-6, compared + ' steps, worst=' + worst);
}

{
    /* Reverse latch: engages after 0.45 s at a standstill, but never removes the brakes at speed. */
    const env = { grip: 1, roll: 0.016 };
    const stopped = AD.createVehicle(0, 0, 0, 0);
    for (let i = 0; i < 120; i++) AD.stepVehicle(stopped, { throttle: 0, brake: 1, steer: 0, handbrake: 0, fire: 0, boost: 0 }, AD.STEP, env);
    check('brake held at a standstill still reverses', stopped.vx < -0.5, 'vx=' + stopped.vx);
    stopped.vx = 25;                                  /* shunted forward with the pedal still down */
    for (let i = 0; i < 120; i++) AD.stepVehicle(stopped, { throttle: 0, brake: 1, steer: 0, handbrake: 0, fire: 0, boost: 0 }, AD.STEP, env);
    check('held brake brakes a car shunted forward to speed', stopped.vx < 16 && stopped.vx > -0.5, 'vx=' + stopped.vx);
}

/* Victory lap: after the finish every car keeps driving (player slot under the AI), but the result, tape, tally, style,
   clock and events stay exactly as they were at the finish. */
{
    const track = AD.buildTrack(AD.trackById('alpenpass'));
    const race = AD.createRace({ track, mode: 'quick', laps: 1, opponents: 3, weapons: true, seed: 777 });
    race.cars[0].ai.aggression = 0.9;
    race.cars[0].ai.skill = 0.95;
    let guard = 0;
    while (race.state !== 'finished' && guard++ < 90000) {
        if (race.state === 'running') Object.assign(race.playerInput, AD.aiInput(race, race.cars[0]));
        AD.stepRace(race, AD.STEP);
        race.events.length = 0;
    }
    check('quick race with the AI in the player slot finishes', race.state === 'finished' && !!race.result, 'steps=' + guard);
    const frozen = () => JSON.stringify([race.result, race.time, race.cars.map(c => [c.tally, c.style, c.tricks, c.tape.length, c.lap, c.finished, c.finishTime, c.weapon, c.ammo]), race.standings.map(c => c.index), race.hazards.length, race.projectiles.length]);
    const before = frozen();
    const start = race.cars.map(c => [c.v.x, c.v.y]);
    let events = 0, finite = true;
    for (let i = 0; i < 1800; i++) {
        AD.stepRace(race, AD.STEP);
        events += race.events.length;
        for (const c of race.cars) finite = finite && AD.vehicleState(c.v).every(Number.isFinite);
    }
    const moved = race.cars.map((c, i) => Math.hypot(c.v.x - start[i][0], c.v.y - start[i][1]));
    check('outro: the result and every scored counter are unchanged', frozen() === before);
    check('outro: no events are emitted', events === 0, 'events=' + events);
    check('outro: the player car keeps driving', moved[0] > 60, 'moved=' + moved[0].toFixed(1));
    check('outro: rivals keep driving', moved.slice(1).every(d => d > 60), moved.map(d => d.toFixed(0)).join(','));
    check('outro: vehicle state stays finite', finite);
}

{
    const t = AD.buildTrack(AD.trackById('neon-hafen')), p = t.puddles.find(x => x.permanent);
    const car = kmh => { const v = AD.createVehicle(0, 0, 0, 0); v.vx = kmh / 3.6; return v; };
    const dry = AD.SURFACES.asphalt;
    const ground = (x, y, wetness) => AD.drivingSurface(t, AD.sampleTrack(t, x, y), { x, y }, { wetness, grip: 1 });
    const centre = ground(p.x, p.y, 0), lane = ground(p.x - p.ty * (p.width + 1), p.y + p.tx * (p.width + 1), 0);
    const hollow = t.puddles.find(x => !x.permanent);
    check('standing water has a centre and a bypass, and rain fills other hollows', centre.puddle > .99 && lane.puddle === 0 &&
        ground(hollow.x, hollow.y, 0).puddle === 0 && ground(hollow.x, hollow.y, .9).puddle > .95);
    function crossPool(offset) {
        const r = AD.createRace({ track: t, mode: 'time_trial', opponents: 0, weapons: false });
        r.countdown = 0; r.state = 'running';
        const v = r.cars[0].v;
        v.x = p.x - p.tx * (p.length + 3) - p.ty * offset;
        v.y = p.y - p.ty * (p.length + 3) + p.tx * offset;
        v.psi = Math.atan2(p.ty, p.tx); v.vx = 120 / 3.6;
        let peak = 0;
        for (let i = 0; i < 120; i++) { AD.stepRace(r, AD.STEP); peak = Math.max(peak, v.planing); }
        return peak;
    }
    check('the painted pool affects a live race crossing and leaves an open line',
        crossPool(0) > .55 && crossPool(p.width + 1) < .02);
    const brakingDistance = env => {
        const v = car(100);
        for (let i = 0; i < 1200 && v.vx > 0.5; i++) AD.stepVehicle(v, { brake: 1 }, AD.STEP, env);
        return v.x;
    };
    const ratio = brakingDistance(lane) / brakingDistance(dry);
    check('wet asphalt lengthens a 100 km/h stop by 12–30 percent', ratio > 1.12 && ratio < 1.3, ratio.toFixed(2));
    const slow = car(40), fast = car(120), bypass = car(120);
    for (let i = 0; i < 36; i++) {
        for (const v of [slow, fast]) AD.stepVehicle(v, { steer: .28 }, AD.STEP, centre);
        AD.stepVehicle(bypass, { steer: .28 }, AD.STEP, lane);
    }
    check('fast wheels plane and turn less in the pool while slow wheels keep contact',
        !slow.planing && fast.planing > .6 && fast.psi < bypass.psi * .8 && fast.vx < bypass.vx);
    for (let i = 0; i < 90; i++) AD.stepVehicle(fast, {}, AD.STEP, dry);
    check('tyres regain contact after leaving the water', fast.planing < .03);

    function oilPass(offset, counter = 0, options = {}) {
        const v = car(95); v.y = offset; Object.assign(v, options);
        const actor = { v, index: 1 }, race = { hazards: [{ type: 'oil', x: 4, y: 0, r: 4, owner: 0, age: 2 }] };
        let peak = 0;
        for (let i = 0; i < 170; i++) {
            const env = { ...dry };
            AD.applyHazardEnv(race, actor, env);
            AD.stepVehicle(v, { steer: i >= 25 && i < 42 ? counter : 0 }, AD.STEP, env);
            peak = Math.max(peak, v.oilFilm);
        }
        return { v, peak };
    }
    const left = oilPass(-1.5), right = oilPass(1.5), catchIt = oilPass(-1.5, -.065);
    check('opposite edges of oil pull the rear in opposite directions and countersteer catches it',
        left.v.psi > .01 && right.v.psi < -.01 && Math.abs(catchIt.v.psi) < left.v.psi * .6);
    check('oil film clears, and adjacent, airborne and cloaked passes stay clean', left.peak > .65 &&
        left.v.oilFilm < .02 && !oilPass(5).peak && !oilPass(0, 0, { airborne: true }).peak && !oilPass(0, 0, { cloak: 8 }).peak);

    const key = car(100), immediate = car(100);
    let steer = 0;
    for (let i = 0; i < 12; i++) {
        steer = AD.digitalSteer(steer, 1, AD.STEP);
        AD.stepVehicle(key, { steer }, AD.STEP, dry);
        AD.stepVehicle(immediate, { steer: 1 }, AD.STEP, dry);
    }
    check('100 ms steering taps are 5–15 percent gentler, full lock and release are available',
        key.steer > immediate.steer * .85 && key.steer < immediate.steer * .95 &&
        AD.digitalSteer(0, 1, .3) > .999 && AD.digitalSteer(1, 0, AD.STEP) === 0);
}

/* Arcade grip: the tyres have to bite harder than the surface alone offers.
   Without the allowance the front axle saturates at the surface grip, the car
   washes out of every fast corner and no amount of steering angle helps - the
   limit is the friction, not the wheel. */
{
    const corner = (grip, speed) => {
        const v = AD.createVehicle(0, 0, 0, 0);
        const env = { grip: grip, roll: 0.016 };
        for (let i = 0; i < 720; i++) {
            v.vx = speed;
            AD.stepVehicle(v, { throttle: 0.35, brake: 0, steer: 1 }, AD.STEP, env);
        }
        return Math.abs(v.vx * v.r) / 9.81;
    };
    const dry = corner(AD.SURFACES.asphalt.grip, 30);
    const wet = corner(AD.SURFACES.wet.grip, 30);
    check('the tyres pull more than one g on dry asphalt and keep more than half of it on the wet',
        dry > 1.2 && wet > 0.6 && dry / wet > 1.2, 'dry=' + dry.toFixed(2) + ' wet=' + wet.toFixed(2));
}

/* The handbrake has to cut through the arcade grip. The sharper the tyres
   bite, the harder the lever has to work: without a lever that breaks the
   rear axle loose the car cannot be put sideways and the drift mode loses
   its score. The slide therefore has to pass the style threshold that the
   drift scoring uses, while the grip alone stays below it. */
{
    const slide = handbrake => {
        const v = AD.createVehicle(0, 0, 0, 0);
        const env = { grip: AD.SURFACES.asphalt.grip, roll: 0.016 };
        let best = 0;
        for (let i = 0; i < 240; i++) {
            v.vx = 22;
            AD.stepVehicle(v, { throttle: 0.4, brake: 0, steer: 0.9, handbrake: handbrake ? 1 : 0 }, AD.STEP, env);
            best = Math.max(best, Math.abs(Math.atan2(v.vy, Math.max(1, Math.abs(v.vx)))));
        }
        return best;
    };
    const lever = slide(true), gripOnly = slide(false);
    check('the handbrake breaks the rear loose: with the lever the slide passes the drift threshold, without it stays below',
        lever > AD.STYLE.driftAngleMin && gripOnly < AD.STYLE.driftAngleMin,
        'lever=' + lever.toFixed(3) + ' grip=' + gripOnly.toFixed(3) +
        ' threshold=' + AD.STYLE.driftAngleMin);
}

if (failures.length) {
    console.error('\n' + failures.length + ' physics check(s) failed');
    process.exit(1);
}
console.log('\nAll physics checks passed.');
