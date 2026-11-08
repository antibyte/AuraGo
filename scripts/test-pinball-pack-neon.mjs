// Neon rule pack tests (plan T15): scripted events against a minimal in-test layout (same ids as tables/neon/layout.js).
import assert from 'node:assert/strict';
import { createGame, startGame, handle, tick, button, take, lampModes, toyState } from '../ui/js/desktop/apps/pinball/rules/core.js';
import { LAMP_MODES } from '../ui/js/desktop/apps/pinball/contracts.js';
import rules from '../ui/js/desktop/apps/pinball/tables/neon/rules.js';

const shots = [
    { id: 'leftOrbit', on: 'trigger:leftOrbitUp@1', combo: true, bonus: 'orbits' },
    { id: 'leftRamp', on: 'pathMade:leftRamp', combo: true, bonus: 'ramps' },
    { id: 'vault', on: 'hole:vault' },
    { id: 'core', on: 'hole:dataCore' },
    { id: 'rightRamp', on: 'pathMade:rightRamp', combo: true, bonus: 'ramps' },
    { id: 'rightOrbit', on: 'trigger:rightOrbitUp@1', combo: true, bonus: 'orbits' },
    { id: 'uplink', on: 'standup:uplinkP' }
];
const lampIds = [
    'shootAgain', 'bonus2x', 'bonus3x', 'bonus4x', 'bonus5x', 'laneT1', 'laneT2', 'laneT3', 'inlaneL', 'outlaneL', 'inlaneR', 'outlaneR',
    'kickbackL', 'extraBall', 'lockLit', 'lock1', 'lock2', 'lock3', 'modeStart', 'wizard', 'superJackpot', 'fw1', 'fw2', 'fw3',
    'neonN', 'neonE', 'neonO', 'neonN2', 'uplinkU', 'uplinkP', 'uplinkL', 'arrow_uplink', 'spinnerLit', ...rules.modes.map(m => 'mode_' + m.id),
    ...shots.flatMap(s => ['arrow_' + s.id, 'jp_' + s.id, 'combo_' + s.id])
];
const layout = { id: 'neon', shots, lamps: lampIds.map((id, i) => ({ id, x: i * 0.01, y: 0.5 })) };
const table = { layout, rules };

const mk = () => { const g = createGame(table, { players: ['A'], seed: 11 }); startGame(g); take(g); return g; };
const P = g => g.players[g.turn];
const ball = g => g.balls.ids[0];
const ev = (g, type, id, x = {}) => handle(g, { type, id, ball: ball(g), ...x });
const play = g => { handle(g, { type: 'launch', id: 'plunger', ball: ball(g), speed: 3, auto: false }); ev(g, 'bumper', 'popA'); take(g); return g; };
const hole = (g, id) => ev(g, 'hole', id, { via: 'field', count: 1 });
const of = (q, type) => q.filter(c => c.type === type);
const lamp = (g, id) => lampModes(g, new Uint8Array(lampIds.length))[lampIds.indexOf(id)];
const fire = {
    leftOrbit: g => ev(g, 'trigger', 'leftOrbitUp', { dir: 1 }), rightOrbit: g => ev(g, 'trigger', 'rightOrbitUp', { dir: 1 }),
    leftRamp: g => ev(g, 'pathMade', 'leftRamp'), rightRamp: g => ev(g, 'pathMade', 'rightRamp'),
    core: g => hole(g, 'dataCore'), vault: g => hole(g, 'vault')
};
const neon = g => ['neonN', 'neonE', 'neonO', 'neonN2'].forEach(id => ev(g, 'standup', id));

// 1. FIREWALL bankDown lights the lock and sets the diverter on
{
    const g = play(mk());
    ev(g, 'drop', 'fw1', { bank: 'firewall' });
    ev(g, 'bankDown', 'firewall');
    const q = take(g);
    assert.equal(P(g).lockLit, true);
    assert.ok(q.some(c => c.type === 'diverter' && c.id === 'lockDiverter' && c.on === true));
    assert.ok(q.some(c => c.type === 'dmd' && c.scene === 'big' && c.params.text === 'dmd_neon_firewall'));
    assert.ok(q.some(c => c.type === 'cue' && c.name === 'lockLit'));
    assert.equal(lamp(g, 'lockLit'), LAMP_MODES.slow);
    assert.equal(lamp(g, 'fw2'), LAMP_MODES.slow);
}

// 2. three locks start GRIDLOCK MULTIBALL
const gridlock = () => {
    const g = play(mk()), out = [];
    for (let i = 0; i < 3; i++) { ev(g, 'bankDown', 'firewall'); hole(g, 'vault'); out.push(...take(g)); }
    return { g, out };
};
{
    const { g, out } = gridlock();
    assert.equal(of(out, 'lockBall').length, 3);
    assert.equal(of(out, 'resetBank').length, 2);
    assert.ok(out.some(c => c.type === 'dropBank' && c.id === 'firewall'));
    assert.equal(of(out, 'eject').filter(c => c.id === 'vault' && c.count === 'all').length, 1);
    assert.equal(g.mb.state, 'running');
    assert.equal(g.balls.ids.length, 3);
    assert.ok(out.some(c => c.type === 'dmd' && c.params.text === 'mode_neon_mb'));
    assert.deepEqual(g.mb.litShots, ['leftRamp', 'rightRamp', 'leftOrbit', 'rightOrbit']);
}

// 3. each mode completes; core relit by 10 spinner revolutions; two completed modes light the extra ball
{
    const g = play(mk());
    const choose = id => {
        P(g).modeReady = false;
        for (let i = 0; i < 10; i++) ev(g, 'spinner', 'spinnerL', { rate: 20 });
        assert.equal(P(g).modeReady, true);
        hole(g, 'dataCore');
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
    run('hack', () => (g.mode.count % 2 ? fire.rightRamp : fire.leftRamp)(g), 6, 3000000);
    assert.equal(P(g).ebLit, false);
    const shotAt = ['leftOrbit', 'leftRamp', 'core', 'rightRamp', 'rightOrbit'];
    run('blackout', () => fire[shotAt[g.mode.target]](g), 4, 4000000);
    take(g);
    ev(g, 'bumper', 'popA');
    assert.equal(P(g).ebLit, true, 'two completed modes light the extra ball');
    run('chase', () => fire.leftOrbit(g), 5, 3000000);
    run('overclock', () => ev(g, 'bumper', 'popL'), 30, 2500000);
    assert.equal(P(g).wizardReady, true);
    assert.equal(lamp(g, 'wizard'), LAMP_MODES.slow);
}

// 3b. chase lights the spinner
{
    const g = play(mk());
    g.mode = { id: 'chase', until: 99, count: 0, target: 0, moveAt: 0, total: 0 };
    assert.equal(lamp(g, 'spinnerLit'), LAMP_MODES.on);
    const s = P(g).score;
    ev(g, 'spinner', 'spinnerL');
    assert.equal(P(g).score - s, 5010);
}

// 4. NEON completions: x1 bonus multiplier and 100k, x2 extra ball lit, x3 kickback lit
{
    const g = play(mk());
    const s = P(g).score;
    neon(g);
    assert.equal(P(g).bonusX, 2);
    assert.equal(P(g).score - s, 4 * 5010 + 100000);
    assert.equal(P(g).ebLit, false);
    assert.ok(take(g).some(c => c.type === 'dmd' && c.params.text === 'dmd_neon_letters'));
    neon(g);
    assert.equal(P(g).bonusX, 3);
    assert.equal(P(g).ebLit, true);
    assert.equal(lamp(g, 'extraBall'), LAMP_MODES.slow);
    P(g).kickbackLit = false;
    neon(g);
    assert.equal(P(g).kickbackLit, true);
    assert.equal(lamp(g, 'neonN'), LAMP_MODES.off);
    ev(g, 'standup', 'neonE');
    assert.equal(lamp(g, 'neonE'), LAMP_MODES.on);
}

// 5. wizard once all four modes were played, started at the core
{
    const g = play(mk());
    P(g).modesPlayed = ['hack', 'blackout', 'chase']; P(g).modeReady = true;
    hole(g, 'dataCore');
    assert.equal(g.mode.id, 'overclock', 'the last unplayed mode starts directly');
    tick(g, 36);
    assert.equal(g.mode.id, null);
    assert.equal(P(g).wizardReady, true);
    take(g);
    hole(g, 'dataCore');
    assert.equal(g.mb.state, 'wizard');
    assert.equal(g.balls.ids.length, 4);
    assert.ok(take(g).some(c => c.type === 'dmd' && c.params.name === 'mode_neon_wizard'));
}

// 6. toyState spire speed per state
{
    const g = play(mk());
    assert.deepEqual(toyState(g), { spire: { speed: 0.4, color: '#37d9ff' }, vault: 0 });
    g.mode.id = 'hack';
    assert.equal(toyState(g).spire.speed, 2);
    g.mode.id = null;
    ev(g, 'bankDown', 'firewall'); hole(g, 'vault');
    assert.equal(toyState(g).vault, 1);
    const { g: m } = gridlock();
    assert.equal(toyState(m).spire.speed, 4);
    assert.equal(toyState(m).vault, 0);
    m.mb.state = 'wizard';
    assert.equal(toyState(m).spire.speed, 6);
}

// 7. UPLINK: U-P-L pays 250k and lights the extra ball once, then 500k more per completion; modes get 10 s, multiball re-lights jackpots
{
    const g = play(mk());
    const upl = () => ['uplinkU', 'uplinkP', 'uplinkL'].forEach(id => ev(g, 'standup', id));
    ev(g, 'standup', 'uplinkP');
    assert.equal(lamp(g, 'uplinkP'), LAMP_MODES.on);
    assert.equal(lamp(g, 'uplinkU'), LAMP_MODES.off);
    take(g);
    let s = P(g).score;
    ev(g, 'standup', 'uplinkU'); ev(g, 'standup', 'uplinkL');
    const q = take(g);
    assert.equal(P(g).score - s, 2 * 5010 + 250000, 'level 1 pays 250k over the target points');
    assert.equal(P(g).ebLit, true);
    assert.ok(q.some(c => c.type === 'dmd' && c.scene === 'big' && c.params.text === 'UPLINK' && c.params.score === 250000));
    assert.ok(q.some(c => c.type === 'cue' && c.name === 'combo'));
    assert.equal(lamp(g, 'uplinkU'), LAMP_MODES.off, 'the letters reset');
    s = P(g).score;
    upl();
    assert.equal(P(g).score - s, 3 * 5010 + 250000 + 500000, 'level 2 adds 500k');
    g.mode = { id: 'hack', until: 40, count: 0, target: 0, moveAt: 0, total: 0 };
    assert.equal(lamp(g, 'uplinkU'), LAMP_MODES.slow, 'unlit letters blink during a mode');
    upl();
    assert.equal(g.mode.until, 50, 'a running mode gets 10 s');
}
{
    const { g } = gridlock();
    g.mb.litShots = ['core'];
    ['uplinkU', 'uplinkP', 'uplinkL'].forEach(id => handle(g, { type: 'standup', id, ball: g.balls.ids[0] }));
    assert.deepEqual(g.mb.litShots, ['leftRamp', 'rightRamp', 'leftOrbit', 'rightOrbit']);
}

console.log('pinball neon pack tests ok');
