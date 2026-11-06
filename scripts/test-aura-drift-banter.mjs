/* Aura Drift - banter rate limit, speaker rules and result title priority.
 * Run: node scripts/test-aura-drift-banter.mjs
 */
import assert from 'node:assert/strict';
import { loadSim } from './aura-drift-sim-loader.mjs';

const AD = loadSim(['banter']);
let passed = 0;
function check(name, fn) {
    try { fn(); passed++; console.log('ok   ' + name); }
    catch (e) { console.error('FAIL ' + name + '\n' + (e && e.stack || e)); process.exitCode = 1; }
}

const t = k => 'T:' + k;
function mockRace() {
    const cars = [{ index: 0, isPlayer: true, driver: null, v: { x: 0, y: 0 }, finished: false, finishTime: 0 }];
    AD.DRIVERS.forEach((d, i) => cars.push({ index: i + 1, isPlayer: false, driver: d, v: { x: (i + 1) * 10, y: 0 }, finished: false, finishTime: 0 }));
    return { cars, standings: cars.slice(), weapons: true };
}
/* Advance the clock and collect whatever is spoken. */
function pump(b, seconds, step = 0.1) {
    const out = [];
    for (let s = 0; s < seconds - 1e-9; s += step) { const m = b.update(step); if (m) out.push(m); }
    return out;
}
const hitBy = (by) => ({ type: 'hit', car: 0, by, heavy: false, weapon: 'rocket' });

check('exports', () => {
    assert.equal(typeof AD.createBanter, 'function');
    assert.equal(typeof AD.pickResultTitle, 'function');
    const b = AD.createBanter({ seed: 1, t, race: mockRace() });
    assert.equal(b.update(1), null);
    b.dispose();
});

check('speaker rules per trigger and key names', () => {
    const race = mockRace();
    const cases = [
        [{ type: 'hit', car: 0, by: 3, heavy: true }, race.cars[3].driver.id, 'got_you'],
        [{ type: 'hit', car: 2, by: 0 }, race.cars[2].driver.id, 'hit'],
        [{ type: 'position', car: 0, from: 4, to: 3 }, race.standings[3].driver.id, 'passed'],
        [{ type: 'position', car: 0, from: 3, to: 4 }, race.standings[2].driver.id, 'pass'],
        [{ type: 'finish', car: 5, time: 90 }, race.cars[5].driver.id, 'win']
    ];
    for (const [evt, id, kind] of cases) {
        const b = AD.createBanter({ seed: 5, t, race });
        b.onEvent(evt, race);
        const out = pump(b, 1);
        assert.equal(out.length, 1, kind);
        assert.equal(out[0].driverId, id, kind);
        assert.equal(out[0].text, 'T:aura_drift.banter.' + id + '.' + kind);
    }
    /* wreck: nearest rival mocks the player, from a crash landing and from a heavy wall hit */
    for (const evt of [{ type: 'landing', car: 0, quality: 'crash' }, { type: 'wall_hit', car: 0, heavy: true }]) {
        const b = AD.createBanter({ seed: 5, t, race });
        b.onEvent(evt, race);
        const out = pump(b, 1);
        assert.equal(out.length, 1);
        assert.equal(out[0].text, 'T:aura_drift.banter.' + race.cars[1].driver.id + '.wreck');
    }
    /* things that must stay silent */
    const b = AD.createBanter({ seed: 5, t, race });
    for (const evt of [
        { type: 'landing', car: 0, quality: 'good' }, { type: 'landing', car: 2, quality: 'crash' },
        { type: 'wall_hit', car: 0, heavy: false }, { type: 'hit', car: 2, by: 3 },
        { type: 'respawn', car: 2 }, { type: 'perfect_start', car: 3 }, { type: 'wheelspin', car: 4 },
        { type: 'finish', car: 0, time: 80 }
    ]) b.onEvent(evt, race);
    assert.equal(pump(b, 3).length, 0);
});

check('marshal lines: respawn 1..6, animals 7..12, speaker marshal', () => {
    const race = mockRace();
    const b = AD.createBanter({ seed: 9, t, race });
    b.onEvent({ type: 'respawn', car: 0, x: 0, y: 0 }, race);
    let out = pump(b, 1);
    assert.equal(out[0].driverId, 'marshal');
    assert.match(out[0].text, /^T:aura_drift\.marshal\.[1-6]$/);
    pump(b, 8);
    b.onEvent(hitBy(1), race);          /* marshal may not speak twice in a row */
    pump(b, 8);
    b.onEvent({ type: 'animal_cross', kind: 'cow', x: 0, y: 0, crossing: 0 }, race);
    out = pump(b, 1);
    assert.match(out[0].text, /^T:aura_drift\.marshal\.(7|8|9|10|11|12)$/);
});

check('rate limit: 1 per 7 s, same driver never twice in a row, 1 per trigger type per 20 s', () => {
    const race = mockRace();
    const b = AD.createBanter({ seed: 3, t, race });
    /* two different drivers within 7 s: only the first is spoken */
    b.onEvent(hitBy(1), race);
    pump(b, 1);
    b.onEvent(hitBy(2), race);
    assert.equal(pump(b, 2).length, 0);
    /* after 7 s a different driver passes, same driver is refused */
    pump(b, 5);
    b.onEvent(hitBy(1), race);
    assert.equal(pump(b, 1).length, 0, 'same driver twice in a row');
    /* same trigger type inside 20 s is refused even for another driver (clock is at ~9 s) */
    b.onEvent(hitBy(2), race);
    assert.equal(pump(b, 1).length, 0, 'same type within 20 s');
    pump(b, 12);
    b.onEvent(hitBy(2), race);
    assert.equal(pump(b, 1).length, 1, 'after 20 s a new driver may speak');
    /* a different trigger type is allowed after the 7 s gap even inside 20 s of the first */
    pump(b, 7);
    b.onEvent({ type: 'respawn', car: 0 }, race);
    assert.equal(pump(b, 1).length, 1);
});

check('long race: never more than 1 line per 7 s, never the same speaker twice in a row', () => {
    const race = mockRace();
    const b = AD.createBanter({ seed: 77, t, race });
    const log = [];
    const evts = [hitBy(1), hitBy(2), { type: 'position', car: 0, from: 4, to: 3 }, { type: 'respawn', car: 0 },
        { type: 'landing', car: 0, quality: 'crash' }, { type: 'animal_cross' }, { type: 'hit', car: 4, by: 0 }];
    for (let step = 0, now = 0; step < 4000; step++, now += 0.1) {
        if (step % 9 === 0) b.onEvent(evts[(step / 9) % evts.length | 0], race);
        const m = b.update(0.1);
        if (m) log.push({ at: now, m });
    }
    assert.ok(log.length >= 8, 'some lines were spoken: ' + log.length);
    for (let i = 1; i < log.length; i++) {
        assert.ok(log[i].at - log[i - 1].at >= 7 - 1e-6, 'gap ' + (log[i].at - log[i - 1].at));
        assert.notEqual(log[i].m.driverId, log[i - 1].m.driverId);
    }
});

check('deterministic for one seed and independent of Math.random', () => {
    const run = () => {
        const race = mockRace();
        const b = AD.createBanter({ seed: 42, t, race });
        const out = [];
        for (let i = 0; i < 600; i++) {
            if (i % 40 === 0) b.onEvent({ type: 'respawn', car: 0 }, race);
            if (i % 55 === 0) b.onEvent({ type: 'animal_cross' }, race);
            const m = b.update(0.1);
            if (m) out.push(m.text);
        }
        return out.join('|');
    };
    const real = Math.random;
    Math.random = () => { throw new Error('Math.random used'); };
    try { assert.equal(run(), run()); } finally { Math.random = real; }
});

check('never mutates the race', () => {
    const race = mockRace();
    const before = JSON.stringify(race);
    const b = AD.createBanter({ seed: 8, t, race });
    for (const e of [hitBy(1), { type: 'position', car: 0, from: 3, to: 2 }, { type: 'landing', car: 0, quality: 'crash' }]) b.onEvent(e, race);
    pump(b, 30);
    assert.equal(JSON.stringify(race), before);
});

check('missing locale key is not spoken', () => {
    const race = mockRace();
    const b = AD.createBanter({ seed: 1, t: k => k, race });
    b.onEvent(hitBy(1), race);
    assert.equal(pump(b, 2).length, 0);
});

check('real race: events drive banter without touching the sim hash', () => {
    const track = AD.buildTrack(AD.TRACKS[0]);
    const mk = () => AD.createRace({ track, mode: 'quick', seed: 11, laps: 1, opponents: 5, weapons: true });
    const a = mk(), c = mk();
    const b = AD.createBanter({ seed: 11, t, race: c });
    const spoken = [];
    for (let i = 0; i < 60 * 12; i++) {
        AD.stepRace(a, AD.STEP);
        AD.stepRace(c, AD.STEP);
        for (const e of c.events) b.onEvent(e, c);
        c.events.length = 0; a.events.length = 0;
        const m = b.update(AD.STEP);
        if (m) spoken.push(m);
    }
    const sig = r => JSON.stringify(r.cars.map(x => AD.vehicleState(x.v)));
    assert.equal(sig(a), sig(c));
    for (const m of spoken) assert.ok(m.driverId === 'marshal' || AD.DRIVERS.some(d => d.id === m.driverId));
});

/* ---- titles ---- */
const base = () => ({
    time: 120, position: 3, field: 6, carId: 'vektor',
    tally: {
        wallHits: 0, brakeSeconds: 10, airTime: 0, drifts: 0, driftSeconds: 0, reverseSeconds: 0, tricks: 0,
        crashLandings: 0, perfectLandings: 0, animalHits: 0, smashed: 0, weaponsFired: 5, hitsDealt: 0, hitsTaken: 0,
        sausageHits: 0, discoFired: 0, carHits: 0, respawns: 0
    }
});
function title(patch, tally, race) {
    const r = base();
    Object.assign(r, patch || {});
    Object.assign(r.tally, tally || {});
    return AD.pickResultTitle(r, null, race);
}

check('titles: default and each rule', () => {
    assert.equal(title(), 'solid_driver');
    assert.equal(title({ position: 1 }, { weaponsFired: 0 }), 'pacifist');
    assert.equal(title({ position: 1 }, { weaponsFired: 0 }, { weapons: false }), 'solid_driver', 'weapons off: no pacifist');
    assert.equal(title({ position: 2 }, { weaponsFired: 0 }), 'solid_driver');
    assert.equal(title({}, { brakeSeconds: 2.3 }), 'brake_coward');
    assert.equal(title({}, { brakeSeconds: 2.5 }), 'solid_driver', '2 percent is not under 2 percent');
    assert.equal(title({ position: 4 }, { brakeSeconds: 2.3 }), 'solid_driver', 'brake_coward needs a top-half finish');
    assert.equal(title({ position: 3, field: 6 }, { brakeSeconds: 0 }), 'brake_coward', 'third of six is the top half');
    assert.equal(title({ time: 30 }, { brakeSeconds: 0.1 }), 'solid_driver', 'brake_coward needs a real race length');
    assert.equal(title({ position: 0 }, { brakeSeconds: 0 }), 'solid_driver', 'no finishing position, no title');
    assert.equal(title({}, { animalHits: 5 }), 'cow_bowler');
    assert.equal(title({}, { airTime: 20 }), 'air_force');
    assert.equal(title({}, { crashLandings: 3 }), 'pancake_chef');
    assert.equal(title({}, { wallHits: 25 }), 'wall_lover');
    assert.equal(title({}, { driftSeconds: 42 }), 'drift_king');
    assert.equal(title({}, { driftSeconds: 41 }), 'solid_driver');
    assert.equal(title({}, { reverseSeconds: 20 }), 'reverse_hero');
    assert.equal(title({}, { sausageHits: 12 }), 'sausage_baron');
    assert.equal(title({}, { discoFired: 2 }), 'disco_fever');
    assert.equal(title({ carId: 'maehkoenig' }, { smashed: 15 }), 'gardener');
    assert.equal(title({ carId: 'vektor' }, { smashed: 15 }), 'solid_driver');
    assert.equal(title({ carId: 'softeis', position: 3 }), 'ice_man');
    assert.equal(title({ carId: 'softeis', position: 4 }), 'solid_driver');
    assert.equal(title({ carId: 'couch', position: 2 }), 'sofa_king');
    assert.equal(title({ carId: 'couch', position: 3 }), 'solid_driver');
    assert.equal(title({ carId: 'bertha' }, { carHits: 15 }), 'big_bertha');
    assert.equal(title({ position: 6 }, { hitsDealt: 3 }), 'last_but_loud');
    assert.equal(title({ position: 5 }, { hitsDealt: 3 }), 'solid_driver');
});

check('titles: priority order, first match wins', () => {
    const all = {
        weaponsFired: 0, brakeSeconds: 0, animalHits: 9, airTime: 30, crashLandings: 4, wallHits: 30, driftSeconds: 80,
        reverseSeconds: 30, sausageHits: 20, discoFired: 3, smashed: 20, carHits: 20, hitsDealt: 9
    };
    const order = [
        ['pacifist', null], ['brake_coward', 'weaponsFired'], ['cow_bowler', 'brakeSeconds'], ['air_force', 'animalHits'],
        ['pancake_chef', 'airTime'], ['wall_lover', 'crashLandings'], ['drift_king', 'wallHits'], ['reverse_hero', 'driftSeconds'],
        ['sausage_baron', 'reverseSeconds'], ['disco_fever', 'sausageHits']
    ];
    const tally = Object.assign({}, all);
    tally.brakeSeconds = 0;
    let patch = { position: 1, carId: 'maehkoenig' };
    for (const [expected, remove] of order) {
        if (remove === 'weaponsFired') tally.weaponsFired = 5;
        else if (remove === 'brakeSeconds') tally.brakeSeconds = 20;
        else if (remove) tally[remove] = 0;
        const got = title(patch, tally);
        assert.equal(got, expected, 'expected ' + expected);
    }
    assert.equal(title(patch, Object.assign({}, tally, { discoFired: 0 })), 'gardener');
    patch = { position: 6, carId: 'bertha' };
    assert.equal(title(patch, Object.assign({}, tally, { discoFired: 0 })), 'big_bertha');
    assert.equal(title(patch, Object.assign({}, tally, { discoFired: 0, carHits: 0 })), 'last_but_loud');
});

check('titles: short or empty runs do not claim statistical titles', () => {
    assert.equal(AD.pickResultTitle({ time: 5, position: 1, field: 6, carId: 'vektor', tally: base().tally }, null), 'solid_driver');
    assert.equal(AD.pickResultTitle(null, null), 'solid_driver');
    assert.equal(AD.pickResultTitle({}, null), 'solid_driver');
});

check('every title id is one of the 16 plan ids', () => {
    const ids = new Set(['pacifist', 'brake_coward', 'cow_bowler', 'air_force', 'pancake_chef', 'wall_lover', 'drift_king', 'reverse_hero',
        'sausage_baron', 'disco_fever', 'gardener', 'ice_man', 'sofa_king', 'big_bertha', 'last_but_loud', 'solid_driver']);
    assert.equal(ids.size, 16);
    for (let i = 0; i < 200; i++) {
        const r = base();
        for (const k of Object.keys(r.tally)) r.tally[k] = (i * 7 + k.length * 3) % 40;
        r.position = 1 + i % 6; r.carId = AD.CARS[i % 8].id;
        assert.ok(ids.has(AD.pickResultTitle(r, null)));
    }
});

console.log(passed + ' checks passed' + (process.exitCode ? ' (with failures)' : ''));
