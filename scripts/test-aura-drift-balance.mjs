/* Aura Drift - seeded AI balance and drivability harness.
 * Run: node scripts/test-aura-drift-balance.mjs
 * Simulates full AI-only laps on every authored track and checks that races are
 * finishable, lap times are plausible, wall contact stays bounded and weapons
 * actually enter play. No browser, clock or randomness outside the seeded PRNG.
 */
import { readFileSync } from 'node:fs';
import vm from 'node:vm';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const sandbox = { window: {}, console };
vm.createContext(sandbox);
for (const file of [
    'ui/js/desktop/apps/aura-drift-core.js',
    'ui/js/desktop/apps/aura-drift-physics.js',
    'ui/js/desktop/apps/aura-drift-track.js',
    'ui/js/desktop/apps/aura-drift-race.js'
]) {
    vm.runInContext(readFileSync(path.join(root, file), 'utf8'), sandbox, { filename: file });
}
const AD = sandbox.window.AuraDriftCore;
const failures = [];
function check(name, condition, detail) {
    if (condition) console.log('  ok  ' + name);
    else { failures.push(name + (detail ? ' — ' + detail : '')); console.log(' FAIL ' + name + (detail ? ' — ' + detail : '')); }
}

const MAX_SECONDS = 240;
const MAX_STEPS = Math.round(MAX_SECONDS / AD.STEP);

function simulate(manifest, seed) {
    const track = AD.buildTrack(manifest, seed);
    const race = AD.createRace({
        track: track, mode: 'quick', laps: 1,
        opponents: 3, weapons: true, seed: seed, showGhost: false
    });
    race.countdown = 0;
    race.state = 'running';
    for (const car of race.cars) car.isPlayer = false;

    const events = { pickup: 0, weapon: 0, hit: 0, lap: 0, finish: 0 };
    let steps = 0, maxCatchup = 1;
    for (; steps < MAX_STEPS; steps++) {
        AD.stepRace(race, AD.STEP);
        for (const car of race.cars) maxCatchup = Math.max(maxCatchup, car.ai.catchupFactor || 1);
        for (const evt of race.events) if (Object.prototype.hasOwnProperty.call(events, evt.type)) events[evt.type]++;
        race.events.length = 0;
        if (race.cars.every(car => car.finished)) break;
    }
    const lapTimes = race.cars.filter(c => c.finished).map(c => c.finishTime);
    return {
        id: manifest.id, length: track.length, steps: steps,
        seconds: steps * AD.STEP, finished: lapTimes.length,
        fastest: lapTimes.length ? Math.min(...lapTimes) : Infinity,
        slowest: lapTimes.length ? Math.max(...lapTimes) : Infinity,
        walls: race.cars.reduce((sum, c) => sum + c.v.wallContacts, 0),
        damage: race.cars.reduce((sum, c) => sum + c.v.damage, 0),
        respawns: race.cars.reduce((sum, c) => sum + c.ai.respawns, 0),
        maxCatchup: maxCatchup,
        events: events,
        finite: race.cars.every(c => AD.vehicleState(c.v).every(Number.isFinite))
    };
}

console.log('Aura Drift AI balance');
const results = [];
for (const manifest of AD.TRACKS) {
    const result = simulate(manifest, 90210);
    results.push(result);
    console.log('  ' + manifest.id.padEnd(18) +
        ' lap ' + (isFinite(result.fastest) ? result.fastest.toFixed(1) + 's' : 'DNF').padStart(7) +
        '   finishers ' + result.finished + '/4' +
        '   wall ' + String(result.walls).padStart(3) +
        '   weapons ' + result.events.weapon +
        '   hits ' + result.events.hit);
    check(manifest.id + ': all cars finish one lap', result.finished === 4,
        result.finished + '/4 in ' + result.seconds.toFixed(0) + 's');
    check(manifest.id + ': lap time is plausible', isFinite(result.fastest) && result.fastest > 20 && result.fastest < MAX_SECONDS,
        'fastest=' + (isFinite(result.fastest) ? result.fastest.toFixed(1) : 'DNF'));
    check(manifest.id + ': vehicle state stays finite', result.finite);
    check(manifest.id + ': wall contacts stay bounded', result.walls < 900, 'walls=' + result.walls);
    check(manifest.id + ': weapon pickups occur', result.events.pickup > 0, 'pickups=' + result.events.pickup);
}

const fastest = Math.min(...results.map(r => r.fastest));
const slowest = Math.max(...results.map(r => r.fastest));
check('track variety produces a real lap-time spread', slowest - fastest > 8,
    fastest.toFixed(1) + 's .. ' + slowest.toFixed(1) + 's');
check('no single track is a wall-fest', results.every(r => r.walls / Math.max(1, r.seconds) < 6),
    results.map(r => r.id + ':' + (r.walls / Math.max(1, r.seconds)).toFixed(1)).join(' '));
check('catch-up never exceeds the disclosed 6%', results.every(r => r.maxCatchup <= 1.06001),
    'max=' + Math.max(...results.map(r => r.maxCatchup)).toFixed(4));
check('marshal recovery stays an exception', results.every(r => r.respawns <= 8),
    results.map(r => r.id + ':' + r.respawns).join(' '));

/* The same seed must produce the same race on every run. */
const a = simulate(AD.TRACKS[0], 777);
const b = simulate(AD.TRACKS[0], 777);
check('balance simulation is reproducible',
    a.fastest === b.fastest && a.walls === b.walls && a.events.weapon === b.events.weapon,
    a.fastest.toFixed(3) + ' vs ' + b.fastest.toFixed(3));

if (failures.length) {
    console.error('\n' + failures.length + ' balance check(s) failed');
    process.exit(1);
}
console.log('\nAll balance checks passed.');
