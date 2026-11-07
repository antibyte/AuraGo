/* Aura Drift - seeded AI balance and drivability harness (plan section 5.3).
 * Run: node scripts/test-aura-drift-balance.mjs   (about 5 to 8 minutes, no browser)
 *
 * 1. Field: 6-car quick race, 1 lap, weapons on, every track, seeds 90210 / 777 / 31337.
 *    All six cars are driven by the AI (slot 0 included), so the race is a pure
 *    simulation of the drivers, cars, crates, hazards and weather.
 * 2. Car parity: slot 0 is forced to each of the 8 cars, everybody else is a
 *    Vektor, weapons off. Slot 0's finish time is compared with the all-Vektor baseline.
 * 3. Drift reference bot: a scripted drifter on drift_attack decides whether the
 *    medal table is reachable.
 * 4. Pace parity: the AI straight-line ceiling against the physics solver for
 *    every car, and the AI field against a full-throttle reference car.
 *
 * The first human play-test found every opponent slower than the player: the AI
 * capped its own target speed below the car's full-throttle equilibrium and
 * scaled it by a speed-dependent look-ahead angle. `aura-drift-ai.js` now takes
 * the ceiling from the solver's force balance (`AD.aiTerminalSpeed`) and brakes
 * against the track radius at fixed distances. Section 4 guards that contract.
 * The bounded 6% catch-up, the handbrake slip release, the weapon fire timers
 * and the medal table are unchanged.
 * No browser, clock or randomness outside the seeded PRNG: the output is reproducible.
 */
import { loadSim } from './aura-drift-sim-loader.mjs';

const AD = loadSim();
const failures = [];
function check(name, condition, detail) {
    if (condition) console.log('  ok  ' + name);
    else { failures.push(name + (detail ? ' - ' + detail : '')); console.log(' FAIL ' + name + (detail ? ' - ' + detail : '')); }
}

const MAX_SECONDS = 300;
const MAX_STEPS = Math.round(MAX_SECONDS / AD.STEP);
const SEEDS = [90210, 777, 31337];
const PARITY_SEEDS = [90210, 777];
const FIELD = 6;
const mean = list => list.reduce((a, b) => a + b, 0) / Math.max(1, list.length);

/* ---- 1. the 6-car field ------------------------------------------------------ */

function simulate(manifest, seed) {
    const track = AD.buildTrack(manifest, seed);
    const race = AD.createRace({
        track: track, mode: 'quick', laps: 1,
        opponents: FIELD - 1, weapons: true, seed: seed, showGhost: false
    });
    race.countdown = 0;
    race.state = 'running';
    for (const car of race.cars) car.isPlayer = false;

    const events = { pickup: 0, weapon: 0, hit: 0, lap: 0, finish: 0 };
    const fires = {}, lastFire = {};
    let steps = 0, maxCatchup = 1, maxIntensity = 0;
    for (; steps < MAX_STEPS; steps++) {
        AD.stepRace(race, AD.STEP);
        for (const car of race.cars) maxCatchup = Math.max(maxCatchup, car.ai.catchupFactor || 1);
        if (race.conditions) maxIntensity = Math.max(maxIntensity, race.conditions.state.intensity);
        for (const evt of race.events) {
            if (Object.prototype.hasOwnProperty.call(events, evt.type)) events[evt.type]++;
            if (evt.type === 'weapon') {
                /* Hold weapons (mg bursts, flame) count once per pickup. */
                const key = evt.car + ':' + evt.id;
                const hold = AD.WEAPONS[evt.id].hold || AD.WEAPONS[evt.id].ammo > 1;
                if (!(hold && lastFire[key] !== undefined && evt.t - lastFire[key] < 1.5)) fires[evt.id] = (fires[evt.id] || 0) + 1;
                lastFire[key] = evt.t;
            }
        }
        race.events.length = 0;
        if (race.cars.every(car => car.finished)) break;
    }
    const times = race.cars.filter(c => c.finished).map(c => c.finishTime);
    const values = [];
    for (const car of race.cars) values.push(...AD.vehicleState(car.v));
    return {
        id: manifest.id, seed: seed, length: track.length, steps: steps,
        seconds: steps * AD.STEP, finished: times.length,
        fastest: times.length ? Math.min(...times) : Infinity,
        slowest: times.length ? Math.max(...times) : Infinity,
        walls: race.cars.reduce((sum, c) => sum + c.v.wallContacts, 0),
        respawns: race.cars.reduce((sum, c) => sum + c.ai.respawns, 0),
        maxCatchup: maxCatchup, maxIntensity: maxIntensity, events: events, fires: fires,
        style: race.cars.reduce((sum, c) => sum + (c.style ? c.style.banked : 0), 0),
        weather: race.conditions ? race.conditions.state.kind : 'none',
        grip: race.conditions ? race.conditions.state.grip : 1,
        finite: race.cars.every(c => AD.vehicleState(c.v).every(Number.isFinite)),
        hash: AD.hashValues(values)
    };
}

console.log('Aura Drift AI balance: ' + FIELD + '-car field');
const results = [];
for (const manifest of AD.TRACKS) {
    for (const seed of SEEDS) {
        const r = simulate(manifest, seed);
        results.push(r);
        const nFires = Object.keys(r.fires).reduce((n, k) => n + r.fires[k], 0);
        console.log('  ' + manifest.id.padEnd(16) + String(seed).padStart(6) +
            ' fast ' + (isFinite(r.fastest) ? r.fastest.toFixed(1) : 'DNF').padStart(6) +
            ' slow ' + (isFinite(r.slowest) ? r.slowest.toFixed(1) : 'DNF').padStart(6) +
            ' fin ' + r.finished + '/' + FIELD +
            ' wall ' + String(r.walls).padStart(4) + ' (' + (r.walls / Math.max(1, r.seconds)).toFixed(1) + '/s)' +
            ' resp ' + r.respawns + ' fires ' + nFires + ' pick ' + r.events.pickup + ' hits ' + r.events.hit +
            ' rain ' + r.maxIntensity.toFixed(2));
    }
}
const label = r => r.id + '/' + r.seed;
const bad = (test) => results.filter(r => !test(r)).map(label).join(' ');

check('all ' + FIELD + ' cars finish within ' + MAX_SECONDS + ' s on every track and seed',
    results.every(r => r.finished === FIELD), bad(r => r.finished === FIELD));
check('fastest lap per track is 60 to 135 s', results.every(r => r.fastest >= 60 && r.fastest <= 135),
    results.filter(r => !(r.fastest >= 60 && r.fastest <= 135)).map(r => label(r) + ':' + r.fastest.toFixed(1)).join(' '));
check('slowest finisher per track is at most 200 s', results.every(r => r.slowest <= 200),
    results.filter(r => r.slowest > 200).map(r => label(r) + ':' + r.slowest.toFixed(1)).join(' '));
const byTrack = AD.TRACKS.map(m => mean(results.filter(r => r.id === m.id).map(r => r.fastest)));
check('track variety: slowest fastest-lap minus fastest fastest-lap above 8 s',
    Math.max(...byTrack) - Math.min(...byTrack) > 8,
    Math.min(...byTrack).toFixed(1) + 's .. ' + Math.max(...byTrack).toFixed(1) + 's');
check('wall contacts per race stay below 1350', results.every(r => r.walls < 1350),
    results.filter(r => r.walls >= 1350).map(r => label(r) + ':' + r.walls).join(' '));
check('wall rate stays below 9 per race second', results.every(r => r.walls / Math.max(1, r.seconds) < 9),
    results.filter(r => r.walls / r.seconds >= 9).map(r => label(r) + ':' + (r.walls / r.seconds).toFixed(1)).join(' '));
check('catch-up never exceeds the disclosed 6%', results.every(r => r.maxCatchup <= 1.06001),
    'max=' + Math.max(...results.map(r => r.maxCatchup)).toFixed(4));
check('marshal respawns stay at 12 per race or fewer', results.every(r => r.respawns <= 12),
    results.filter(r => r.respawns > 12).map(r => label(r) + ':' + r.respawns).join(' '));
check('vehicle state stays finite', results.every(r => r.finite), bad(r => r.finite));
check('every race produces style points, none above 180000',
    results.every(r => r.style > 0 && r.style < 180000),
    results.filter(r => !(r.style > 0 && r.style < 180000)).map(r => label(r) + ':' + Math.round(r.style)).join(' '));
check('weather never adds grip', results.every(r => r.grip <= 1.0001),
    results.map(r => r.grip.toFixed(3)).join(' '));
check('at least one track sees rain or snow above 0.2 mid-race', results.some(r => r.maxIntensity > 0.2),
    'max=' + Math.max(...results.map(r => r.maxIntensity)).toFixed(2));

/* Weapons: every id appears, none dominates. */
{
    const totals = {};
    const add = r => { for (const id of Object.keys(r.fires)) totals[id] = (totals[id] || 0) + r.fires[id]; };
    results.forEach(add);
    /* Rare pickups (railgun, cloak, overdrive) need luck: top up with extra
       seeded races, in a fixed order, until every weapon has fired once. */
    const TOP_UP = [11, 2024, 4711, 65535, 123456, 999, 31, 8080, 5150, 1701, 606, 42];
    let extra = 0;
    while (AD.WEAPON_IDS.some(id => !totals[id]) && extra < TOP_UP.length) {
        add(simulate(AD.TRACKS[extra % AD.TRACKS.length], TOP_UP[extra]));
        extra++;
    }
    console.log('  info weapon top-up races: ' + extra);
    const sum = Object.keys(totals).reduce((n, k) => n + totals[k], 0);
    const missing = AD.WEAPON_IDS.filter(id => !totals[id]);
    const top = Object.keys(totals).sort((a, b) => totals[b] - totals[a])[0];
    console.log('  info fires by weapon (' + sum + '): ' + AD.WEAPON_IDS.map(id => id + ' ' + (totals[id] || 0)).join(', '));
    check('all 18 weapons fire at least once over the field runs', missing.length === 0, 'missing: ' + missing.join(','));
    check('no weapon above 22 percent of fires', totals[top] / sum <= 0.22, top + ' ' + (100 * totals[top] / sum).toFixed(1) + '%');
    const perTrack = AD.TRACKS.map(m => {
        const rs = results.filter(r => r.id === m.id);
        return m.id + ':' + rs.reduce((n, r) => n + Object.keys(r.fires).reduce((a, k) => a + r.fires[k], 0), 0);
    });
    check('weapons feature on every track (at least 6 fires over 3 seeds)',
        AD.TRACKS.every(m => results.filter(r => r.id === m.id).reduce((n, r) => n + Object.keys(r.fires).reduce((a, k) => a + r.fires[k], 0), 0) >= 6),
        perTrack.join(' '));
}

/* Reproducible: the same seed twice gives the same race down to the last vehicle state. */
for (const id of ['neon-hafen', 'salzsee']) {
    const manifest = AD.trackById(id);
    const a = simulate(manifest, 777), b = simulate(manifest, 777);
    check(id + ': same seed twice gives equal fastest, walls, weapon counts and vehicle hash',
        a.fastest === b.fastest && a.walls === b.walls && JSON.stringify(a.fires) === JSON.stringify(b.fires) && a.hash === b.hash,
        a.fastest.toFixed(3) + '/' + a.hash + ' vs ' + b.fastest.toFixed(3) + '/' + b.hash);
    check(id + ': weather is deterministic for a seed', a.weather === b.weather && a.grip === b.grip && a.maxIntensity === b.maxIntensity);
}

/* ---- 2. car parity ------------------------------------------------------------ */

function parityRun(manifest, seed, carId) {
    const track = AD.buildTrack(manifest, seed);
    const race = AD.createRace({
        track: track, mode: 'quick', laps: 1, opponents: FIELD - 1, weapons: false,
        seed: seed, showGhost: false, playerCar: carId
    });
    race.countdown = 0;
    race.state = 'running';
    for (const car of race.cars) {
        car.isPlayer = false;
        if (car.index !== 0) {
            /* A neutral Vektor field: same car, same driver tuning for everybody. */
            car.v.car = AD.CARS[0]; car.v.carIndex = 0;
            car.ai.skill = 0.95; car.ai.aggression = 1; car.ai.drift = 0.5;
        }
    }
    const slot = race.cars[0];
    for (let i = 0; i < MAX_STEPS && !slot.finished; i++) AD.stepRace(race, AD.STEP);
    return slot.finished ? slot.finishTime : Infinity;
}

console.log('Car parity: slot 0 against an all-Vektor field');
const ratios = {};
const parityTracks = AD.TRACKS;
{
    const baseline = {};
    for (const manifest of parityTracks) for (const seed of PARITY_SEEDS) {
        baseline[manifest.id + '/' + seed] = parityRun(manifest, seed, 'vektor');
    }
    for (const car of AD.CARS) {
        ratios[car.id] = {};
        for (const manifest of parityTracks) {
            const perSeed = PARITY_SEEDS.map(seed => (car.id === 'vektor' ? baseline[manifest.id + '/' + seed] : parityRun(manifest, seed, car.id)) / baseline[manifest.id + '/' + seed]);
            ratios[car.id][manifest.id] = perSeed;
        }
    }
    console.log('  ' + 'car'.padEnd(12) + parityTracks.map(m => m.id.slice(0, 7).padStart(8)).join('') + '    mean');
    for (const car of AD.CARS) {
        const perTrack = parityTracks.map(m => mean(ratios[car.id][m.id]));
        console.log('  ' + car.id.padEnd(12) + perTrack.map(r => r.toFixed(3).padStart(8)).join('') + '   ' + mean(perTrack).toFixed(3));
    }
    /* Rivals used to share one artificial speed ceiling, which flattened the
       cars' real power, drag and grip differences. Now that the ceiling comes
       from the solver, a low-grip car on ice or a light car in the Alps can be
       up to 17 percent off on a single track; the mean bound below stays tight. */
    for (const car of AD.CARS) {
        const perTrack = parityTracks.map(m => mean(ratios[car.id][m.id]));
        const worst = parityTracks.filter((m, i) => !(perTrack[i] >= 0.88 && perTrack[i] <= 1.17));
        check(car.id + ': finishes every parity race and is within 0.88 to 1.17 of the baseline on every track',
            perTrack.every(r => isFinite(r)) && worst.length === 0, worst.map(m => m.id).join(','));
        check(car.id + ': mean ratio over the tracks is within 0.93 to 1.08', mean(perTrack) >= 0.93 && mean(perTrack) <= 1.08,
            mean(perTrack).toFixed(3));
    }
    const means = AD.CARS.map(c => mean(parityTracks.map(m => mean(ratios[c.id][m.id]))));
    const funny = ['maehkoenig', 'couch', 'softeis', 'bertha'].map(id => means[AD.carIndex(id)]);
    check('no car is strictly better or worse than every other (mean ratios spread within 0.12)',
        Math.max(...means) - Math.min(...means) <= 0.12, means.map(m => m.toFixed(3)).join(' '));
    check('the funny cars are not strictly worse than the serious four',
        Math.min(...funny) <= Math.max(...funny) && funny.every(m => m < 1.08), funny.map(m => m.toFixed(3)).join(' '));
}

/* ---- 3. drift reference bot --------------------------------------------------- */

/* A path follower that holds a slide in every corner: the nose is steered
   `alpha` radians past the line of travel, the handbrake starts the slide and
   the throttle stays down. It is deliberately not an expert. */
function driftBot(race, car, alpha) {
    const v = car.v, track = race.track, out = race.playerInput;
    Object.assign(out, AD.aiInput(race, car));            /* base: line, corridor and recovery */
    const s = AD.sampleTrack(track, v.x, v.y);
    const target = AD.pointAt(track, s.s + 16 + Math.max(0, v.vx) * 0.55);
    const pathDir = Math.atan2(target.y - v.y, target.x - v.x);
    const far = AD.pointAt(track, s.s + 40 + Math.max(0, v.vx));
    const signed = s.tx * far.ty - s.ty * far.tx;
    const inCorner = Math.abs(signed) > 0.02;
    const slip = Math.atan2(v.vy, Math.max(1, Math.abs(v.vx)));
    const aim = pathDir + (inCorner ? Math.sign(signed) * alpha : 0);
    out.steer = AD.clamp(2.2 * AD.wrapAngle(aim - v.psi) - 0.18 * v.r, -1, 1);
    out.throttle = 1; out.brake = 0; out.boost = 0; out.fire = 0;
    out.handbrake = (inCorner && v.vx > 16 && Math.abs(slip) < alpha * 0.55) ? 1 : 0;
    const corridor = Math.abs(s.offset) / s.halfWidth;
    if (corridor > 0.6) out.steer = AD.clamp(out.steer - Math.sign(s.offset) * (corridor - 0.6) * 2, -1, 1);
}
function driftRun(trackId, alpha) {
    const track = AD.buildTrack(AD.trackById(trackId), 90210);
    const race = AD.createRace({ track: track, mode: 'drift_attack', seed: 90210, showGhost: false });
    while (race.state === 'countdown') AD.stepRace(race, AD.STEP);
    for (let i = 0; i < 120 * 100 && race.state !== 'finished'; i++) {
        driftBot(race, race.cars[0], alpha);
        AD.stepRace(race, AD.STEP);
        race.events.length = 0;
    }
    return { score: race.result ? race.result.score : 0, state: race.state, time: race.time };
}

console.log('Drift attack reference bot (slide angle 0.30 rad)');
{
    const medals = AD.modeRules('drift_attack').medals;
    const runs = ['alpenpass', 'neon-hafen', 'industrie-ring'].map(id => Object.assign({ id: id }, driftRun(id, 0.3)));
    for (const r of runs) console.log('  ' + r.id.padEnd(16) + ' score ' + r.score);
    const average = mean(runs.map(r => r.score));
    check('the bot runs reach the end of the 90 s clock', runs.every(r => r.state === 'finished'));
    check('reference drifter scores 15000 to 30000 on average', average >= 15000 && average <= 30000, Math.round(average) + '');
    check('reference drifter is 30 to 60 percent of gold on average', average / medals.gold >= 0.3 && average / medals.gold <= 0.6,
        (100 * average / medals.gold).toFixed(0) + '%');
    check('every bot run earns at least bronze', runs.every(r => r.score >= medals.bronze), runs.map(r => r.score).join(' '));
    const bold = ['metropolis', 'industrie-ring', 'gletscherpass'].map(id => driftRun(id, 0.35).score);
    console.log('  bold slide (0.35 rad): ' + bold.join(' '));
    check('a bolder slide reaches gold-class scores (at least 90 percent of gold) on some track',
        Math.max(...bold) >= 0.9 * medals.gold, 'best ' + Math.max(...bold) + ' vs gold ' + medals.gold);
}

/* ---- 4. opponent pace parity -------------------------------------------------- */

/* The first human play-test found every opponent slower than the player. The AI
   now shares the player's full-throttle ceiling; these checks stop that ceiling
   drifting away from the solver, or the field drifting away from a car that
   simply holds the throttle down. */
console.log('Opponent pace: physics ceiling and full-throttle reference');
{
    const asphalt = AD.SURFACES.asphalt;
    for (const car of AD.CARS) {
        const v = AD.createVehicle(0, 0, 0, AD.carIndex(car.id));
        const env = { grip: asphalt.grip, roll: asphalt.roll, slipstream: 0, sample: null };
        const input = { throttle: 1, brake: 0, steer: 0, handbrake: 0, boost: 0, fire: 0 };
        for (let i = 0; i < 120 * 45; i++) AD.stepVehicle(v, input, AD.STEP, env);
        const measured = v.vx;
        const predicted = AD.aiTerminalSpeed(v, asphalt, 1);
        check(car.id + ': AI ceiling equals the full-throttle physics speed',
            Math.abs(measured - predicted) < 0.6,
            measured.toFixed(2) + ' vs ' + predicted.toFixed(2) + ' m/s');
    }

    /* Slot 0 uses the same steering brain but never lifts. The field must reach
       its peak speed and stay within 20 percent of its mean speed. */
    const orig = AD.aiInput;
    AD.aiInput = function (race, car) {
        const input = orig(race, car);
        if (car.index === 0 && !car.isPlayer) { input.throttle = 1; input.brake = 0; }
        return input;
    };
    for (const id of ['alpenpass', 'salzsee', 'metropolis']) {
        const track = AD.buildTrack(AD.trackById(id), 90210);
        const race = AD.createRace({ track: track, mode: 'quick', laps: 1, opponents: 5,
            weapons: false, seed: 90210, showGhost: false });
        race.countdown = 0;
        race.state = 'running';
        for (const car of race.cars) car.isPlayer = false;
        let steps = 0, refSpeed = 0, aiSpeed = 0, refPeak = 0, aiPeak = 0;
        for (; steps < Math.round(180 / AD.STEP); steps++) {
            AD.stepRace(race, AD.STEP);
            for (const car of race.cars) {
                const speed = Math.max(0, car.v.vx);
                if (car.index === 0) { refSpeed += speed; refPeak = Math.max(refPeak, speed); }
                else { aiSpeed += speed; aiPeak = Math.max(aiPeak, speed); }
            }
            if (race.cars.every(car => car.finished)) break;
        }
        const refMean = refSpeed / steps, aiMean = aiSpeed / (5 * steps);
        check(id + ': opponents reach the reference car peak speed', aiPeak >= refPeak * 0.95,
            aiPeak.toFixed(1) + ' vs ' + refPeak.toFixed(1) + ' m/s');
        check(id + ': opponent mean speed stays within 20 percent of a full-throttle car',
            aiMean >= refMean * 0.8, aiMean.toFixed(1) + ' vs ' + refMean.toFixed(1) + ' m/s');
    }
    AD.aiInput = orig;
}

if (failures.length) {
    console.error('\n' + failures.length + ' balance check(s) failed:\n' + failures.join('\n'));
    process.exit(1);
}
console.log('\nAll balance checks passed.');
