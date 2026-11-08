// Leviathan rule pack tests (plan T21): scripted events against a minimal in-test layout (same ids as tables/leviathan/layout.js).
import assert from 'node:assert/strict';
import { createGame, startGame, handle, tick, button, take, lampModes, toyState } from '../ui/js/desktop/apps/pinball/rules/core.js';
import { LAMP_MODES } from '../ui/js/desktop/apps/pinball/contracts.js';
import rules from '../ui/js/desktop/apps/pinball/tables/leviathan/rules.js';

const shots = [
    { id: 'leftOrbit', on: 'trigger:leftOrbitUp@1', combo: true },
    { id: 'current', on: 'pathMade:currentRamp', combo: true },
    { id: 'pearl', on: 'captive:pearl' },
    { id: 'ship', on: 'hole:shipScoop' },
    { id: 'tentacle', on: 'pathMade:tentacleRamp', combo: true },
    { id: 'rightOrbit', on: 'trigger:rightOrbitUp@1', combo: true }
];
const lampIds = [
    'shootAgain', 'bonus2x', 'bonus3x', 'bonus4x', 'bonus5x', 'laneT1', 'laneT2', 'laneT3', 'inlaneL', 'outlaneL', 'inlaneR', 'outlaneR',
    'kickbackR', 'extraBall', 'lockLit', 'lock1', 'lock2', 'lock3', 'modeStart', 'wizard', 'superJackpot', 'sonar1', 'sonar2', 'sonar3',
    'sonar4', 'sonar5', 'pearl1', 'pearl2', 'pearl3', 'grab', 'spinnerLit', 'eyeL', 'eyeR', ...rules.modes.map(m => 'mode_' + m.id),
    ...shots.flatMap(s => ['arrow_' + s.id, 'jp_' + s.id, 'combo_' + s.id])
];
const devices = [{ id: 'abyss', type: 'magnet', x: 0.010, y: 0.540 }, { id: 'shipScoop', type: 'hole', x: 0.070, y: 0.780 }];
const layout = { id: 'leviathan', shots, devices, lamps: lampIds.map((id, i) => ({ id, x: i * 0.01, y: 0.5 })) };
const table = { layout, rules };

const mk = (t = table) => { const g = createGame(t, { players: ['A'], seed: 11 }); startGame(g); take(g); return g; };
const P = g => g.players[g.turn];
const ball = g => g.balls.ids[0];
const ev = (g, type, id, x = {}) => handle(g, { type, id, ball: ball(g), ...x });
const play = g => { handle(g, { type: 'launch', id: 'plunger', ball: ball(g), speed: 3, auto: false }); ev(g, 'bumper', 'coral1'); take(g); return g; };
const hole = (g, id) => ev(g, 'hole', id, { via: 'field', count: 1 });
const of = (q, type) => q.filter(c => c.type === type);
const lamp = (g, id) => lampModes(g, new Uint8Array(lampIds.length))[lampIds.indexOf(id)];
const fire = {
    leftOrbit: g => ev(g, 'trigger', 'leftOrbitUp', { dir: 1 }), rightOrbit: g => ev(g, 'trigger', 'rightOrbitUp', { dir: 1 }),
    current: g => ev(g, 'pathMade', 'currentRamp'), tentacle: g => ev(g, 'pathMade', 'tentacleRamp'),
    pearl: g => ev(g, 'captive', 'pearl'), ship: g => hole(g, 'shipScoop')
};
const sonar = g => [1, 2, 3, 4, 5].forEach(n => ev(g, 'standup', 'sonar' + n));
const magnetEv = (g, state) => ev(g, 'magnet', 'abyss', { state });

// 1. three pearl strikes light the lock (progress lamps); a pearlTop hit counts one more and pays the DMD event
{
    const g = play(mk());
    fire.pearl(g); fire.pearl(g);
    assert.equal(P(g).lockLit, false);
    assert.equal(lamp(g, 'pearl2'), LAMP_MODES.on);
    assert.equal(lamp(g, 'pearl3'), LAMP_MODES.off);
    take(g);
    ev(g, 'trigger', 'pearlTop');
    assert.equal(P(g).lockLit, true, 'the strike that rides to the top counts double');
    assert.ok(take(g).some(c => c.type === 'dmd' && c.scene === 'big' && c.params.text === 'dmd_leviathan_pearl'));
    assert.equal(lamp(g, 'lockLit'), LAMP_MODES.slow);
    assert.equal(lamp(g, 'arrow_tentacle'), LAMP_MODES.on);
    assert.equal(lamp(g, 'pearl1'), LAMP_MODES.slow);
    // tentacle while lit = lock
    fire.tentacle(g);
    assert.equal(P(g).locks, 1);
    assert.equal(P(g).lockLit, false);
    assert.equal(lamp(g, 'lock1'), LAMP_MODES.on);
    // a relight needs only two strikes
    fire.pearl(g);
    assert.equal(P(g).lockLit, false);
    fire.pearl(g);
    assert.equal(P(g).lockLit, true, 'relights take two strikes');
}

// 2. the third lock starts ABYSS MULTIBALL with three balls and no physical lock
const abyss = () => {
    const g = play(mk()), out = [];
    for (let i = 0; i < 3; i++) { fire.pearl(g); fire.pearl(g); if (i === 0) fire.pearl(g); fire.tentacle(g); out.push(...take(g)); }
    return { g, out };
};
{
    const { g, out } = abyss();
    assert.equal(of(out, 'lockBall').length, 0);
    assert.equal(of(out, 'addBall').filter(c => c.auto).length, 2);
    assert.equal(g.mb.state, 'running');
    assert.equal(g.balls.ids.length, 3);
    assert.ok(out.some(c => c.type === 'dmd' && c.params.text === 'mode_leviathan_mb'));
    assert.deepEqual(g.mb.litShots, ['current', 'tentacle', 'leftOrbit', 'rightOrbit']);
    const s = P(g).score;
    fire.current(g);
    assert.ok(P(g).score - s >= 1000000, 'jackpot at the current ramp');
    assert.equal(lamp(g, 'jp_tentacle'), LAMP_MODES.fast);
    assert.equal(lamp(g, 'eyeL'), LAMP_MODES.fast);
    // pearl strikes do nothing to the lock during multiball
    fire.pearl(g); fire.pearl(g); fire.pearl(g);
    assert.equal(P(g).lockLit, false);
}

// 3. sonar 5/5 lights Deep Grab and relights the kickback
{
    const g = play(mk());
    P(g).kickbackLit = false;
    ev(g, 'standup', 'sonar2');
    assert.equal(lamp(g, 'sonar2'), LAMP_MODES.on);
    assert.equal(lamp(g, 'grab'), LAMP_MODES.off);
    take(g);
    ['sonar1', 'sonar3', 'sonar4', 'sonar5'].forEach(id => ev(g, 'standup', id));
    const q = take(g);
    assert.equal(P(g).kickbackLit, true);
    assert.equal(lamp(g, 'grab'), LAMP_MODES.slow);
    assert.equal(lamp(g, 'sonar2'), LAMP_MODES.off, 'the targets reset');
    assert.ok(q.some(c => c.type === 'dmd' && c.params.text === 'dmd_leviathan_sonar'));
    assert.ok(q.some(c => c.type === 'kickback' && c.id === 'kickbackR' && c.armed === true));
    assert.ok(q.some(c => c.type === 'magnet' && c.id === 'abyss' && c.mode === 'grab'));
}

// 4. Deep Grab: held 1.5 s, thrown at the ship; a ship capture within 2 s pays 1M, a late one does not
const deepGrab = wait => {
    const g = play(mk());
    g.skill.superUntil = 0;
    sonar(g); take(g);
    magnetEv(g, 'held');
    tick(g, 1.4);
    assert.equal(of(take(g), 'magnet').length, 0, 'still held');
    assert.equal(lamp(g, 'grab'), LAMP_MODES.fast);
    tick(g, 0.2);
    const thrown = of(take(g), 'magnet');
    assert.equal(thrown.length, 1);
    assert.equal(thrown[0].mode, 'throw');
    assert.equal(thrown[0].speed, 1.4);
    assert.ok(Math.abs(thrown[0].angle - Math.atan2(0.24, 0.06)) < 1e-9, 'angle from the abyss to the ship scoop');
    assert.equal(lamp(g, 'grab'), LAMP_MODES.off);
    tick(g, wait);
    const s = P(g).score;
    hole(g, 'shipScoop');
    return { g, gain: P(g).score - s, q: take(g) };
};
{
    const hit = deepGrab(1.5);
    assert.equal(hit.gain, 25000 + 1000000);
    assert.ok(hit.q.some(c => c.type === 'dmd' && c.params.text === 'dmd_leviathan_grab' && c.params.score === 1000000));
    assert.equal(deepGrab(2.2).gain, 25000, 'too late');
}
{
    // an unused window closes after 10 s
    const g = play(mk());
    sonar(g); take(g);
    tick(g, 10.1);
    assert.ok(of(take(g), 'magnet').some(c => c.mode === 'off'));
    assert.equal(lamp(g, 'grab'), LAMP_MODES.off);
    // the fallback angle is used when the layout has no devices
    const g2 = play(mk({ layout: { ...layout, devices: [] }, rules }));
    sonar(g2); take(g2); magnetEv(g2, 'held'); tick(g2, 1.6);
    assert.ok(Math.abs(of(take(g2), 'magnet')[0].angle - 70 * Math.PI / 180) < 1e-9);
}

// 5. each mode completes at the ship; two completed modes light the extra ball
{
    const g = play(mk());
    const choose = id => {
        P(g).modeReady = true;
        hole(g, 'shipScoop');
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
    run('tsunami', () => (g.mode.count % 2 ? fire.tentacle : fire.current)(g), 5, 3000000);
    assert.equal(P(g).ebLit, false);
    run('sonar', () => ev(g, 'standup', 'sonar' + (g.mode.target + 1)), 5, 3500000);
    take(g);
    ev(g, 'bumper', 'coral1');
    assert.equal(P(g).ebLit, true, 'two completed modes light the extra ball');
    // treasure: the magnet assist grabs and throws toward the ship on each pass
    choose('treasure');
    tick(g, 0.1);
    let q = take(g);
    assert.ok(q.some(c => c.type === 'magnet' && c.mode === 'grab'));
    magnetEv(g, 'held');
    tick(g, 0.7);
    q = take(g);
    assert.ok(q.some(c => c.type === 'magnet' && c.mode === 'throw' && c.speed === 1.4));
    tick(g, 0.7);
    assert.ok(take(g).some(c => c.type === 'magnet' && c.mode === 'grab'), 're-armed for the next pass');
    fire.ship(g); fire.ship(g); fire.ship(g);
    assert.ok(P(g).modesCompleted.includes('treasure'));
    tick(g, 0.1);
    assert.ok(take(g).some(c => c.type === 'magnet' && c.mode === 'off'), 'assist ends with the mode');
    const shotAt = ['leftOrbit', 'current', 'pearl', 'ship', 'tentacle', 'rightOrbit'];
    P(g).modeReady = true;
    hole(g, 'shipScoop');
    assert.equal(g.mode.id, 'kraken', 'the last unplayed mode starts directly');
    for (let i = 0; i < 4; i++) fire[shotAt[g.mode.target]](g);
    assert.equal(g.mode.id, null);
    assert.ok(P(g).modesCompleted.includes('kraken'));
    // A final kraken hit at the ship leaves the mode and (framework order) starts the lit wizard in the same event.
    if (g.mb.state !== 'wizard') {
        assert.equal(P(g).wizardReady, true);
        assert.equal(lamp(g, 'wizard'), LAMP_MODES.slow);
    }
}

// 6. wizard (LEVIATHAN AWAKENS) once all four modes were played, started at the ship
{
    const g = play(mk());
    P(g).modesPlayed = ['tsunami', 'sonar', 'treasure']; P(g).modeReady = true;
    hole(g, 'shipScoop');
    assert.equal(g.mode.id, 'kraken');
    tick(g, 46);
    assert.equal(g.mode.id, null);
    assert.equal(P(g).wizardReady, true);
    take(g);
    hole(g, 'shipScoop');
    assert.equal(g.mb.state, 'wizard');
    assert.equal(g.balls.ids.length, 4);
    assert.ok(take(g).some(c => c.type === 'dmd' && c.params.name === 'mode_leviathan_wizard'));
}

// 7. super skill shot at the ship within 6 s of the launch
{
    const g = mk();
    handle(g, { type: 'launch', id: 'plunger', ball: ball(g), speed: 3, auto: false });
    take(g);
    const s = P(g).score;
    hole(g, 'shipScoop');
    assert.ok(P(g).score - s >= 1000000);
}

// 8. toyState: eye colour and jaw per state
{
    const g = play(mk());
    assert.deepEqual(toyState(g), { eye: { color: '#43e3d5', open: 0.3 }, jaw: 0 });
    fire.pearl(g); fire.pearl(g); fire.pearl(g);
    assert.deepEqual(toyState(g), { eye: { color: '#7fd3ff', open: 0.55 }, jaw: 0 });
    g.mode.id = 'tsunami';
    assert.deepEqual(toyState(g), { eye: { color: '#dcad63', open: 0.7 }, jaw: 0 });
    g.mode.id = null;
    const { g: m } = abyss();
    assert.deepEqual(toyState(m), { eye: { color: '#ff5a3c', open: 1 }, jaw: 0.6 });
    m.mb.state = 'wizard';
    assert.deepEqual(toyState(m), { eye: { color: '#ffffff', open: 1 }, jaw: 1 });
}

console.log('pinball leviathan pack tests ok');
