// Temple rule pack tests (plan T24): scripted events against a minimal in-test layout (same ids as tables/temple/layout.js).
import assert from 'node:assert/strict';
import { createGame, startGame, handle, tick, button, take, lampModes, toyState } from '../ui/js/desktop/apps/pinball/rules/core.js';
import { LAMP_MODES } from '../ui/js/desktop/apps/pinball/contracts.js';
import rules from '../ui/js/desktop/apps/pinball/tables/temple/rules.js';

const shots = [
    { id: 'leftOrbit', on: 'trigger:leftOrbitUp@1', combo: true, bonus: 'orbits' },
    { id: 'leftRamp', on: 'pathMade:leftRamp', combo: true, bonus: 'ramps' },
    { id: 'idol', on: 'hole:idolScoop' },
    { id: 'rightRamp', on: 'pathMade:rightRamp', combo: true, bonus: 'ramps' },
    { id: 'rightOrbit', on: 'trigger:rightOrbitUp@1', combo: true, bonus: 'orbits' },
    { id: 'lock', on: 'hole:orbitLock' }
];
const lampIds = [
    'shootAgain', 'bonus2x', 'bonus3x', 'bonus4x', 'bonus5x', 'laneT1', 'laneT2', 'laneT3', 'inlaneL', 'outlaneL', 'inlaneR', 'outlaneR',
    'kickbackL', 'extraBall', 'lockLit', 'lock1', 'lock2', 'lock3', 'modeStart', 'wizard', 'superJackpot',
    'sunS', 'sunU', 'sunN', 'sunStar', 'jewelL', 'jewelR', 'idolEyeL', 'idolEyeR', 'doorsOpen', 'spinnerLit',
    ...rules.modes.map(m => 'mode_' + m.id), ...shots.flatMap(s => ['arrow_' + s.id, 'jp_' + s.id, 'combo_' + s.id])
];
const layout = { id: 'temple', shots, lamps: lampIds.map((id, i) => ({ id, x: i * 0.01, y: 0.5 })) };
const table = { layout, rules };

const mk = () => { const g = createGame(table, { players: ['A'], seed: 11 }); startGame(g); take(g); return g; };
const P = g => g.players[g.turn];
const ball = g => g.balls.ids[0];
const ev = (g, type, id, x = {}) => handle(g, { type, id, ball: ball(g), ...x });
const play = g => { handle(g, { type: 'launch', id: 'plunger', ball: ball(g), speed: 3, auto: false }); ev(g, 'bumper', 'jungle1'); take(g); return g; };
const hole = (g, id) => ev(g, 'hole', id, { via: 'field', count: 1 });
const of = (q, type) => q.filter(c => c.type === type);
const lamp = (g, id) => lampModes(g, new Uint8Array(lampIds.length))[lampIds.indexOf(id)];
const fire = {
    leftOrbit: g => ev(g, 'trigger', 'leftOrbitUp', { dir: 1 }), rightOrbit: g => ev(g, 'trigger', 'rightOrbitUp', { dir: 1 }),
    leftRamp: g => ev(g, 'pathMade', 'leftRamp'), rightRamp: g => ev(g, 'pathMade', 'rightRamp'), idol: g => hole(g, 'idolScoop')
};
const sunDown = g => { ['sunS', 'sunU', 'sunN', 'sunStar'].forEach(id => ev(g, 'drop', id, { bank: 'sun' })); ev(g, 'bankDown', 'sun'); };
const pops = (g, n) => { for (let i = 0; i < n; i++) ev(g, 'bumper', 'jungle' + (1 + i % 3)); };
const doorCmd = (q, open) => q.some(c => c.type === 'door' && c.id === 'idolDoors' && c.open === open);
const hasDmd = (q, text) => q.some(c => c.type === 'dmd' && c.scene === 'big' && c.params.text === text);
const golden = () => {
    const g = play(mk()), out = [];
    for (let i = 0; i < 3; i++) { pops(g, 20); hole(g, 'orbitLock'); out.push(...take(g)); }
    return { g, out };
};

// 1. SUN bankDown opens the doors, readies a mode and shows the DMD text
{
    const g = play(mk());
    P(g).modeReady = false;
    const s = lamp(g, 'doorsOpen');
    assert.equal(s, LAMP_MODES.off);
    sunDown(g);
    const q = take(g);
    assert.ok(doorCmd(q, true));
    assert.equal(P(g).modeReady, true);
    assert.ok(hasDmd(q, 'dmd_temple_doors'));
    assert.equal(lamp(g, 'doorsOpen'), LAMP_MODES.on);
    assert.equal(lamp(g, 'sunStar'), LAMP_MODES.on);
    assert.equal(lamp(g, 'arrow_idol'), LAMP_MODES.slow);
    assert.deepEqual(toyState(g).doors, 1);
    ev(g, 'bankReset', 'sun');
    assert.equal(lamp(g, 'sunS'), LAMP_MODES.off);
}

// 1b. with the wizard ready the bank shows SUN ALIGNED
{
    const g = play(mk());
    P(g).wizardReady = true;
    sunDown(g);
    assert.ok(hasDmd(take(g), 'dmd_temple_sun'));
}

// 2. a mode started at the idol closes the doors and resets the bank
{
    const g = play(mk());
    P(g).modesPlayed = ['expedition', 'serpent', 'riddle'];
    sunDown(g); take(g);
    fire.idol(g);
    const q = take(g);
    assert.equal(g.mode.id, 'eclipse', 'the last unplayed mode starts directly');
    assert.ok(doorCmd(q, false));
    assert.ok(q.some(c => c.type === 'resetBank' && c.id === 'sun'));
    assert.equal(lamp(g, 'doorsOpen'), LAMP_MODES.off);
    assert.equal(lamp(g, 'sunU'), LAMP_MODES.off);
    assert.equal(toyState(g).doors, 0);
    // a hole entry during a running mode does not touch the doors
    sunDown(g); take(g);
    fire.idol(g);
    assert.ok(!doorCmd(take(g), false));
}

// 3. twenty pops light the lock and enable the hole
{
    const g = play(mk());
    pops(g, 18); // plus the one from play()
    assert.equal(P(g).lockLit, false);
    take(g);
    pops(g, 1);
    const q = take(g);
    assert.equal(P(g).lockLit, true);
    assert.ok(q.some(c => c.type === 'holeEnable' && c.id === 'orbitLock' && c.on === true));
    assert.ok(hasDmd(q, 'dmd_lock_lit'));
    assert.equal(lamp(g, 'lockLit'), LAMP_MODES.slow);
    assert.equal(lamp(g, 'arrow_lock'), LAMP_MODES.on);
    hole(g, 'orbitLock');
    const l = take(g);
    assert.equal(P(g).locks, 1);
    assert.ok(l.some(c => c.type === 'lockBall' && c.id === 'orbitLock'));
    assert.ok(l.some(c => c.type === 'holeEnable' && c.on === false));
}

// 4. three locks start GOLDEN MULTIBALL: eject all, three balls, doors forced open
{
    const { g, out } = golden();
    assert.equal(of(out, 'lockBall').length, 3);
    assert.equal(of(out, 'eject').filter(c => c.id === 'orbitLock' && c.count === 'all').length, 1);
    assert.equal(g.mb.state, 'running');
    assert.equal(g.balls.ids.length, 3);
    assert.ok(doorCmd(out, true));
    assert.ok(out.some(c => c.type === 'dmd' && c.params.text === 'mode_temple_mb'));
    assert.deepEqual(g.mb.litShots, ['leftRamp', 'rightRamp', 'leftOrbit', 'idol']);
    assert.equal(toyState(g).doors, 1);
    take(g);
    const s = P(g).score;
    fire.idol(g);
    assert.ok(P(g).score - s >= 1000000, 'idol jackpot');
    // the multiball ends: doors close, bank resets
    g.mb.state = 'off';
    ev(g, 'bumper', 'jungle1');
    const q = take(g);
    assert.ok(doorCmd(q, false));
    assert.ok(q.some(c => c.type === 'resetBank'));
}

// 5. modes: expedition, serpent, eclipse complete; extra ball after two completed modes
{
    const g = play(mk());
    const choose = id => {
        P(g).modeReady = true;
        hole(g, 'idolScoop');
        const n = g.select ? g.select.options.indexOf(id) : 0;
        for (let i = 0; i < n; i++) button(g, 'right', true);
        if (g.select) button(g, 'start', true);
        assert.equal(g.mode.id, id);
    };
    const run = (id, hit, times, value) => {
        choose(id);
        for (let i = 0; i < times; i++) hit();
        assert.equal(g.mode.id, null, id + ' completes');
        assert.ok(P(g).modesCompleted.includes(id));
        assert.ok(P(g).score >= value);
    };
    run('expedition', () => (g.mode.count % 2 ? fire.rightOrbit : fire.leftOrbit)(g), 5, 3000000);
    assert.equal(P(g).ebLit, false);
    run('serpent', () => (g.mode.count % 2 ? fire.rightRamp : fire.leftRamp)(g), 5, 3000000);
    ev(g, 'bumper', 'jungle1');
    assert.equal(P(g).ebLit, true, 'two completed modes light the extra ball');
    assert.equal(lamp(g, 'extraBall'), LAMP_MODES.slow);
    const q0 = take(g);
    fire.leftRamp(g);
    assert.equal(P(g).ebLit, false);
    assert.equal(P(g).extraBalls, 1);
    assert.ok(take(g).some(c => c.type === 'dmd' && c.scene === 'extraBall'));
    assert.equal(q0.length >= 0, true);
    run('eclipse', () => (g.mode.count % 2 ? () => ev(g, 'bumper', 'jungle2') : () => ev(g, 'drop', 'sunS', { bank: 'sun' }))(), 25, 2500000);
}

// 6. riddle: leftRamp -> rightRamp -> leftOrbit -> rightOrbit, only in order
{
    const g = play(mk());
    P(g).modesPlayed = ['expedition', 'serpent', 'eclipse'];
    fire.idol(g);
    assert.equal(g.mode.id, 'riddle');
    fire.rightRamp(g); fire.leftOrbit(g); fire.rightOrbit(g);
    assert.equal(g.mode.count, 0, 'out-of-order shots do not count');
    assert.equal(lamp(g, 'arrow_leftRamp'), LAMP_MODES.slow);
    fire.leftRamp(g); assert.equal(g.mode.count, 1);
    fire.leftRamp(g); assert.equal(g.mode.count, 1);
    fire.rightRamp(g); fire.leftOrbit(g);
    assert.equal(g.mode.id, 'riddle');
    fire.rightOrbit(g);
    assert.equal(g.mode.id, null);
    assert.ok(P(g).modesCompleted.includes('riddle'));
}

// 7. wizard once all four modes were played, started at the idol; doors stay open
{
    const g = play(mk());
    P(g).modesPlayed = ['expedition', 'serpent', 'riddle']; P(g).modeReady = true;
    fire.idol(g);
    assert.equal(g.mode.id, 'eclipse');
    tick(g, 36);
    assert.equal(g.mode.id, null);
    assert.equal(P(g).wizardReady, true);
    take(g);
    fire.idol(g);
    const q = take(g);
    assert.equal(g.mb.state, 'wizard');
    assert.equal(g.balls.ids.length, 4);
    assert.ok(q.some(c => c.type === 'dmd' && c.params.name === 'mode_temple_wizard'));
    assert.equal(toyState(g).doors, 1, 'doors forced open for the idol jackpots');
}

// 8. jewels: both lit pay 100k, alternate bonus multiplier and kickback
{
    const g = play(mk());
    ev(g, 'standup', 'jewelL');
    assert.equal(lamp(g, 'jewelL'), LAMP_MODES.on);
    assert.equal(lamp(g, 'jewelR'), LAMP_MODES.off);
    const s = P(g).score;
    ev(g, 'standup', 'jewelR');
    assert.equal(P(g).bonusX, 2);
    assert.equal(P(g).score - s, 5010 + 100000);
    assert.equal(lamp(g, 'jewelL'), LAMP_MODES.off);
    P(g).kickbackLit = false;
    ev(g, 'standup', 'jewelL'); ev(g, 'standup', 'jewelR');
    assert.equal(P(g).bonusX, 2);
    assert.equal(P(g).kickbackLit, true);
}

// 9. super skill shot: pathMade:leftRamp within 8 s of the launch
{
    const g = mk();
    handle(g, { type: 'launch', id: 'plunger', ball: ball(g), speed: 3, auto: false });
    take(g);
    const s = P(g).score;
    tick(g, 7);
    fire.leftRamp(g);
    assert.ok(P(g).score - s >= 1000000);
    assert.ok(take(g).some(c => c.type === 'dmd' && c.scene === 'skillShot' && c.params.super));
    const h = play(mk());
    tick(h, 9);
    const t = P(h).score;
    fire.leftRamp(h);
    assert.ok(P(h).score - t < 1000000, 'late super skill shot pays nothing extra');
}

// 10. toyState per state and eyes/spinner lamps
{
    const g = play(mk());
    let s = toyState(g);
    assert.equal(s.doors, 0);
    assert.ok(s.idolEyes.glow < 0.3);
    assert.equal(lamp(g, 'idolEyeL'), LAMP_MODES.dim);
    sunDown(g);
    s = toyState(g);
    assert.equal(s.doors, 1);
    assert.ok(s.idolEyes.glow > 0.5 && s.idolEyes.glow < 1);
    assert.equal(lamp(g, 'idolEyeR'), LAMP_MODES.on);
    g.mode.id = 'expedition';
    assert.equal(lamp(g, 'idolEyeR'), LAMP_MODES.fast);
    assert.equal(lamp(g, 'spinnerLit'), LAMP_MODES.on);
    assert.equal(toyState(g).idolEyes.color, '#8ad6a3');
    g.mode.id = null;
    g.mb.state = 'running';
    assert.equal(toyState(g).idolEyes.glow, 1);
    g.mb.state = 'wizard';
    assert.equal(toyState(g).sunRays, 1);
    assert.ok(toyState(g).pyramidGlow > 0);
}

console.log('pinball temple pack tests ok');
