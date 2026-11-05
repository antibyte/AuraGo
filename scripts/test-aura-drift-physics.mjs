/* Aura Drift - deterministic physics and race replay checks.
 * Run: node scripts/test-aura-drift-physics.mjs
 * The browser modules are plain IIFEs; a VM context supplies `window`.
 */
import { readFileSync } from 'node:fs';
import vm from 'node:vm';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const sandbox = { window: {}, console };
vm.createContext(sandbox);
const modules = [
    'ui/js/desktop/apps/aura-drift-core.js',
    'ui/js/desktop/apps/aura-drift-physics.js',
    'ui/js/desktop/apps/aura-drift-track.js',
    'ui/js/desktop/apps/aura-drift-race.js'
];
for (const file of modules) {
    const absolute = path.join(root, file);
    vm.runInContext(readFileSync(absolute, 'utf8'), sandbox, { filename: file });
}
const AD = sandbox.window.AuraDriftCore;
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

if (failures.length) {
    console.error('\n' + failures.length + ' physics check(s) failed');
    process.exit(1);
}
console.log('\nAll physics checks passed.');
