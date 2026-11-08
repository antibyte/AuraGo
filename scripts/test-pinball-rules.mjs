// Rules framework tests (plan T06) against an in-file synthetic pack: 4 modes, physical lock, lanes, kickback.
import assert from 'node:assert/strict';
import { createGame, startGame, tick, handle, button, nudge, take, lampModes, dmdState, toyState, resetTraining, debugBalls } from '../ui/js/desktop/apps/pinball/rules/core.js';
import { copy, rngNext } from '../ui/js/desktop/apps/pinball/config.js';
import { LAMP_MODES, CUES, MUSIC_SECTIONS } from '../ui/js/desktop/apps/pinball/contracts.js';

// ---------------------------------------------------------------- synthetic table
const orbs = ['orb1', 'orb2', 'orb3', 'orb4'];
const shots = [
    { id: 'ramp1', on: 'pathMade:ramp1', combo: true, bonus: 'ramps' }, { id: 'ramp2', on: 'pathMade:ramp2', combo: true },
    ...orbs.map(id => ({ id, on: 'trigger:' + id + '@1' })),
    { id: 'vault', on: 'hole:vault' }, { id: 'core', on: 'hole:core' }
];
const modeIds = ['a', 'b', 'c', 'd'];
const lampIds = [
    'shootAgain', 'bonus2x', 'bonus3x', 'bonus4x', 'bonus5x', 'laneT1', 'laneT2', 'laneT3', 'inlaneL', 'outlaneL', 'inlaneR', 'outlaneR',
    'kickbackL', 'extraBall', 'lockLit', 'lock1', 'lock2', 'lock3', 'modeStart', 'wizard', 'superJackpot',
    ...modeIds.map(m => 'mode_' + m), ...shots.flatMap(s => ['arrow_' + s.id, 'jp_' + s.id, 'combo_' + s.id])
];
const layout = { id: 'synth', shots, lamps: lampIds.map((id, i) => ({ id, x: i * 0.01, y: 0.5 })) };
const lamp = (g, id) => lampModes(g, new Uint8Array(layout.lamps.length))[lampIds.indexOf(id)];
const rules = {
    points: { 'bumper:*': 3010, 'standup:*': 5010, default: 1010 },
    shotPoints: {},
    bonusFrom: { 'bumper:*': 'bumpers' },
    bonusCounters: [{ id: 'ramps', key: 'dmd_bonus_ramps', value: 25000 }, { id: 'bumpers', key: 'dmd_bonus_bumpers', value: 1000 }],
    topLanes: { switches: ['laneT1', 'laneT2', 'laneT3'], lamps: ['laneT1', 'laneT2', 'laneT3'], award: 'bonusX' },
    lowerLanes: { switches: ['inlaneL', 'outlaneL', 'inlaneR', 'outlaneR'], lamps: ['inlaneL', 'outlaneL', 'inlaneR', 'outlaneR'], award: 'kickback' },
    bonusX: { lamps: ['bonus2x', 'bonus3x', 'bonus4x', 'bonus5x'], max: 5 },
    skill: { value: 250000, super: { on: 'pathMade:ramp2', window: 6, value: 1000000 } },
    ballSave: { seconds: 8, lamp: 'shootAgain' },
    kickback: { device: 'kickbackL', lamp: 'kickbackL', litAtStart: true },
    extraBall: { shot: 'core', lamp: 'extraBall' },
    lock: { hole: 'vault', shot: 'vault', physical: true, lamp: 'lockLit', diverter: null },
    multiball: { name: 'mode_synth_mb', balls: 3, ballSave: 10, jackpotShots: orbs, jackpotBase: 1000000, jackpotStep: 250000, superAfter: 4, superShot: 'core', superWindow: 20, addABallShot: 'vault' },
    modeStart: { shot: 'core', hole: 'core', lamp: 'modeStart' },
    modes: [
        { id: 'a', name: 'mode_a', hint: 'dmd_hint_ramps', kind: 'count', targets: ['ramp1', 'ramp2'], count: 3, duration: 30, shotValue: 100000, step: 50000, completeValue: 1000000 },
        { id: 'b', name: 'mode_b', hint: 'dmd_hint_lit', kind: 'roving', targets: ['orb1', 'orb2', 'orb3'], count: 2, moveEvery: 4, duration: 30, shotValue: 100000, step: 0, completeValue: 500000 },
        { id: 'c', name: 'mode_c', hint: 'dmd_hint_sequence', kind: 'sequence', targets: ['ramp1', 'orb1', 'ramp2'], duration: 30, shotValue: 100000, step: 0, completeValue: 500000 },
        { id: 'd', name: 'mode_d', hint: 'dmd_hint_bumpers', kind: 'count', targets: ['bumper:*'], count: 5, duration: 30, shotValue: 10000, step: 0, completeValue: 500000 }
    ],
    wizard: { id: 'w', name: 'mode_synth_wizard', balls: 4, ballSave: 20, jackpotValue: 5000000, step: 1000000, superShot: 'core', superAfter: 6 },
    combos: { window: 3.0, base: 100000 },
    replay: 40000000,
    hooks: { onEvent(ctx, ev) { if (ev.type === 'bankDown') ctx.lightLock(); }, toy: g => ({ balls: g.balls.active }) }
};
const table = { layout, rules };

// ---------------------------------------------------------------- helpers
const mk = (o = {}) => { const g = createGame(table, { players: ['A'], seed: 7, ...o }); startGame(g); return g; };
const P = g => g.players[g.turn];
const ball = g => g.balls.ids[0];
const ev = (g, type, id, x = {}) => handle(g, { type, id, ball: ball(g), ...x });
const launch = g => handle(g, { type: 'launch', id: 'plunger', ball: ball(g), speed: 3, auto: false });
const of = (list, type) => list.filter(c => c.type === type);
const ticks = (g, seconds, dt = 0.1) => { for (let t = 0; t < seconds - 1e-9; t += dt) tick(g, Math.min(dt, seconds - t)); };
const drain = g => handle(g, { type: 'drain', id: 'drain', ball: ball(g) });
const gained = (g, fn) => { const s = P(g).score; fn(); return P(g).score - s; };
const bumper = g => ev(g, 'bumper', 'popA');
const playBall = g => { launch(g); bumper(g); ticks(g, 10); drain(g); };

// 1. start 1-4 players -> addBall; launch -> play
for (const n of [1, 2, 3, 4]) {
    const g = mk({ players: ['A', 'B', 'C', 'D'].slice(0, n) });
    const q = take(g);
    assert.equal(g.players.length, n);
    assert.equal(of(q, 'addBall').length, 1);
    assert.deepEqual(of(q, 'addBall')[0], { type: 'addBall', id: 1, auto: false });
    assert.equal(g.phase, 'ballStart');
    launch(g);
    assert.equal(g.phase, 'play');
    assert.ok(of(q, 'music').some(c => c.section === 'main'));
}
assert.deepEqual(toyState(mk()), { balls: 1 });

// 2. ball save within 8 s re-serves; after 9.5 s the ball ends
{
    const g = mk(); take(g);
    launch(g); bumper(g); ticks(g, 9.0);
    drain(g);
    const q = take(g);
    assert.equal(of(q, 'addBall').length, 1);
    assert.equal(of(q, 'addBall')[0].auto, true);
    assert.ok(q.some(c => c.type === 'dmd' && c.scene === 'ballSaved'));
    assert.ok(q.some(c => c.type === 'cue' && c.name === 'ballSaved'));
    assert.equal(g.phase, 'play');
    assert.equal(P(g).ball, 1);
    const h = mk(); launch(h); bumper(h); ticks(h, 9.6); drain(h);
    assert.equal(h.phase, 'bonus');
}

// 3. skill shot and super skill shot
{
    let g = mk(); launch(g);
    assert.equal(gained(g, () => ev(g, 'rollover', 'laneT' + (g.skill.lane + 1))), 251010);
    g = mk(); launch(g);
    assert.equal(gained(g, () => ev(g, 'rollover', 'laneT' + ((g.skill.lane + 1) % 3 + 1))), 1010);
    g = mk(); launch(g); ticks(g, 3);
    assert.equal(gained(g, () => ev(g, 'pathMade', 'ramp2')), 1001010);
    g = mk(); launch(g); ticks(g, 7);
    assert.equal(gained(g, () => ev(g, 'pathMade', 'ramp2')), 1010);
}

// 4. lane change on flippers; lanes complete -> bonusX; cap 5
{
    const g = mk(); launch(g);
    ev(g, 'rollover', 'laneT1'); ev(g, 'rollover', 'laneT2');
    assert.deepEqual(P(g).lanesTop, [true, true, false]);
    button(g, 'right', true);
    assert.deepEqual(P(g).lanesTop, [false, true, true]);
    button(g, 'left', true);
    assert.deepEqual(P(g).lanesTop, [true, true, false]);
    ev(g, 'rollover', 'laneT3');
    assert.equal(P(g).bonusX, 2);
    assert.deepEqual(P(g).lanesTop, [false, false, false]);
    assert.equal(lamp(g, 'bonus2x'), LAMP_MODES.on);
    assert.equal(lamp(g, 'bonus3x'), LAMP_MODES.off);
    for (let i = 0; i < 6; i++) for (const l of ['laneT1', 'laneT2', 'laneT3']) ev(g, 'rollover', l);
    assert.equal(P(g).bonusX, 5);
    const k = mk(); launch(k);
    for (const l of rules.lowerLanes.switches) ev(k, 'rollover', l);
    assert.equal(P(k).kickbackLit, true);
}

// shared: a game that has reached multiball through three physical locks
const lockGame = () => {
    const g = mk(); take(g); launch(g);
    const out = [];
    for (let i = 0; i < 3; i++) {
        ev(g, 'bankDown', 'fw');
        assert.equal(P(g).lockLit, true);
        handle(g, { type: 'hole', id: 'vault', ball: ball(g), via: 'field', count: 1 });
        out.push(...take(g));
    }
    return { g, out };
};

// 5. lock x3 -> lockBall + addBall x2, then eject all and top-up to 3, MB running, save 10 s
{
    const { g, out } = lockGame();
    assert.equal(of(out, 'lockBall').length, 3);
    assert.equal(of(out, 'addBall').length, 2);
    const ej = of(out, 'eject');
    assert.equal(ej.length, 1);
    assert.equal(ej[0].count, 'all');
    assert.equal(g.balls.ids.length, 3);
    assert.equal(g.balls.locked, 0);
    assert.equal(g.mb.state, 'running');
    assert.equal(g.ballSave.until - g.time, 11.5);
    assert.equal(g.ballSave.soft - g.time, 10);
    assert.ok(out.some(c => c.type === 'music' && c.section === 'multiball'));
    assert.equal(P(g).locks, 0);
}

// 6. jackpots, super jackpot, MB end at <= 1 ball
{
    const { g } = lockGame();
    const jp = (id) => gained(g, () => ev(g, 'trigger', id, { dir: 1 }));
    assert.equal(jp('orb1'), 1001010);
    assert.equal(jp('orb2'), 1251010);
    assert.equal(jp('orb3'), 1501010);
    assert.equal(jp('orb4'), 1751010);
    assert.ok(g.mb.superUntil > g.time);
    assert.equal(lamp(g, 'superJackpot'), LAMP_MODES.fast);
    assert.equal(gained(g, () => handle(g, { type: 'hole', id: 'core', ball: ball(g) })), 5 * 2000000 + 1010);
    take(g);
    handle(g, { type: 'hole', id: 'vault', ball: ball(g) });
    assert.equal(g.balls.ids.length, 4, 'add-a-ball');
    ticks(g, 12);
    drain(g); drain(g); drain(g);
    assert.equal(g.mb.state, 'off');
    assert.equal(g.balls.ids.length, 1);
    assert.equal(g.phase, 'play');
}

// 7. mode select, timers, 4 played -> wizard ready -> 4 balls
{
    const g = mk(); take(g); launch(g); P(g).modeReady = true;
    const core = () => handle(g, { type: 'hole', id: 'core', ball: ball(g) });
    core();
    assert.deepEqual(g.select.options, modeIds);
    assert.ok(of(take(g), 'eject').length === 0, 'ball stays in the hole during select');
    button(g, 'right', true); assert.equal(g.select.index, 1);
    button(g, 'left', true); button(g, 'left', true); assert.equal(g.select.index, 3);
    assert.equal(lamp(g, 'mode_d'), LAMP_MODES.slow);
    button(g, 'right', true); assert.equal(g.select.index, 0);
    button(g, 'start', true);
    assert.equal(g.mode.id, 'a');
    assert.equal(g.mode.until - g.time, 30);
    assert.equal(lamp(g, 'mode_a'), LAMP_MODES.fast);
    assert.equal(of(take(g), 'eject').length, 1);
    const d = gained(g, () => { for (let i = 0; i < 3; i++) ev(g, 'pathMade', 'ramp1'); });
    assert.equal(d, 100000 + 150000 + 200000 + 1000000 + 3 * 1010);
    assert.deepEqual(P(g).modesCompleted, ['a']);
    assert.equal(g.mode.id, null);
    P(g).modeReady = true; core(); button(g, 'right', true); button(g, 'launch', true);
    assert.equal(g.mode.id, 'c');
    ticks(g, 31);
    assert.equal(g.mode.id, null);
    assert.deepEqual(P(g).modesCompleted, ['a']);
    P(g).modeReady = true; core(); ticks(g, 4.1);
    assert.equal(g.mode.id, 'b');
    const t0 = g.mode.target; ticks(g, 4.1); assert.notEqual(g.mode.target, t0);
    ticks(g, 31);
    P(g).modeReady = true; core(); ticks(g, 4.1);
    assert.equal(g.mode.id, 'd');
    ticks(g, 31);
    assert.deepEqual(P(g).modesPlayed.slice().sort(), modeIds);
    assert.equal(P(g).wizardReady, true);
    assert.equal(lamp(g, 'modeStart'), LAMP_MODES.fast);
    core();
    assert.equal(g.mb.state, 'wizard');
    assert.equal(g.balls.ids.length, 4);
    assert.equal(g.ballSave.soft - g.time, 20);
}

// 8. extra ball repeats the ball number and lights shoot again
{
    const g = mk(); launch(g); P(g).modeReady = false; P(g).ebLit = true;
    assert.equal(lamp(g, 'arrow_core'), LAMP_MODES.pulse);
    handle(g, { type: 'hole', id: 'core', ball: ball(g) });
    assert.equal(P(g).extraBalls, 1);
    assert.equal(lamp(g, 'shootAgain'), LAMP_MODES.on);
    ticks(g, 10); drain(g); ticks(g, 3);
    assert.equal(P(g).ball, 1);
    assert.equal(P(g).extraBalls, 0);
    assert.equal(g.phase, 'ballStart');
    assert.ok(of(take(g), 'dmd').some(c => c.scene === 'shootAgain'));
}

// 9. bonus = sum * X added after the count; both flippers skip within 0.2 s
{
    const setup = () => {
        const g = mk(); launch(g); ev(g, 'pathMade', 'ramp1'); ev(g, 'pathMade', 'ramp1');
        for (let i = 0; i < 3; i++) bumper(g);
        P(g).bonusX = 2; ticks(g, 10);
        return g;
    };
    const g = setup();
    const s0 = P(g).score;
    drain(g);
    assert.equal(g.phase, 'bonus');
    assert.equal(P(g).score, s0);
    ticks(g, 1.0); assert.equal(P(g).score, s0, 'still counting');
    ticks(g, 0.6); assert.equal(P(g).score, s0 + (2 * 25000 + 3 * 1000) * 2);
    ticks(g, 1.5); assert.equal(g.phase, 'ballStart'); assert.equal(P(g).ball, 2);
    const h = setup(); const s1 = P(h).score;
    drain(h); button(h, 'left', true); button(h, 'right', true); ticks(h, 0.2);
    assert.equal(P(h).score, s1 + 106000);
    assert.equal(h.phase, 'ballStart');
}

// 10. tilt on the 3rd warning
{
    const g = mk(); launch(g); take(g); bumper(g);
    const s = P(g).score;
    assert.equal(nudge(g, 1), true); assert.equal(nudge(g, 1), true); assert.equal(P(g).warnings, 1);
    ticks(g, 1.1); nudge(g, 1); assert.equal(P(g).warnings, 2);
    ticks(g, 1.1); assert.equal(nudge(g, 1), false);
    assert.equal(P(g).tilt, true);
    const q = take(g);
    assert.ok(q.some(c => c.type === 'coils' && c.on === false));
    assert.ok(q.some(c => c.type === 'gi' && c.mode === 'off'));
    assert.ok(q.some(c => c.type === 'music' && c.section === 'off'));
    assert.deepEqual(P(g).counters, {});
    assert.equal(lamp(g, 'shootAgain'), LAMP_MODES.off);
    bumper(g); assert.equal(P(g).score, s, 'no score while tilted');
    drain(g);
    assert.equal(g.phase, 'ballStart', 'no bonus after a tilt');
    assert.equal(P(g).ball, 2);
    assert.ok(of(take(g), 'coils').some(c => c.on === true));
    assert.equal(P(g).tilt, false);
}

// 11. two players alternate P1B1, P2B1, P1B2, ... then match
{
    const g = mk({ players: ['A', 'B'] });
    const order = [];
    while (g.phase !== 'match') { order.push([g.turn, P(g).ball]); playBall(g); ticks(g, 3); }
    assert.deepEqual(order, [[0, 1], [1, 1], [0, 2], [1, 2], [0, 3], [1, 3]]);
}

// 12. seeded match; equal digits -> knocker
for (const equal of [true, false]) {
    const g = mk();
    for (let i = 0; i < 2; i++) { playBall(g); ticks(g, 3); }
    launch(g); bumper(g); ticks(g, 10);
    g.seed = 12345;
    const digit = Math.floor(rngNext({ seed: g.seed }) * 10) * 10;
    P(g).score = 1000000 + (equal ? digit : (digit + 10) % 100);
    take(g); drain(g); ticks(g, 3);
    assert.equal(g.phase, 'match');
    assert.equal(g.match.digit, digit);
    const q = take(g);
    assert.equal(q.some(c => c.type === 'cue' && c.name === 'knocker'), equal);
}

// 13. initials entry, non-qualifier skips, 45 s timeout
{
    const finishGame = (o) => {
        const g = mk(o);
        for (let i = 0; i < 2; i++) { playBall(g); ticks(g, 3); }
        launch(g); bumper(g); ticks(g, 10); P(g).score = 5000000; drain(g); ticks(g, 3.1);
        ticks(g, 4);
        return g;
    };
    let g = finishGame();
    assert.equal(g.phase, 'initials');
    button(g, 'left', true); assert.equal(g.initials.cur, 37);
    button(g, 'right', true); assert.equal(g.initials.cur, 0);
    button(g, 'start', true);
    button(g, 'right', true); button(g, 'start', true);
    button(g, 'right', true); button(g, 'launch', true);
    assert.equal(g.phase, 'over');
    assert.deepEqual(g.results, [{ name: 'ABC', score: g.players[0].score }]);
    const full = Array.from({ length: 10 }, (_, i) => ({ name: 'TOP', score: 9000000 + i * 10, date: 1 }));
    g = finishGame({ scores: full });
    assert.equal(g.phase, 'over');
    assert.deepEqual(g.results, []);
    g = finishGame();
    button(g, 'start', true); button(g, 'right', true); button(g, 'start', true);
    ticks(g, 46);
    assert.equal(g.phase, 'over');
    assert.deepEqual(g.results, [{ name: 'AB', score: g.players[0].score }]);
}

// 14. training: endless serve, no results
{
    const g = mk({ training: true });
    for (let i = 0; i < 6; i++) {
        take(g); launch(g); bumper(g); ticks(g, 20); drain(g);
        assert.equal(of(take(g), 'addBall').length, 1);
        assert.equal(g.phase, 'ballStart');
    }
    assert.equal(P(g).ball, 1);
    assert.deepEqual(g.results, []);
    assert.equal(dmdState(g).params.training, true);
    P(g).score = 123450; resetTraining(g);
    assert.equal(P(g).score, 0);
    assert.ok(of(take(g), 'clearBalls').length === 1);
}

// 15. replay awards an extra ball once
{
    const g = mk(); launch(g); take(g);
    P(g).score = 39998000;
    bumper(g); assert.equal(P(g).extraBalls, 1);
    const q = take(g);
    assert.ok(q.some(c => c.type === 'cue' && c.name === 'replay') && q.some(c => c.type === 'cue' && c.name === 'knocker'));
    bumper(g); assert.equal(P(g).extraBalls, 1);
}

// 16. combos
{
    const g = mk(); launch(g); ticks(g, 7);
    assert.equal(gained(g, () => ev(g, 'pathMade', 'ramp1')), 1010);
    ticks(g, 1);
    assert.equal(gained(g, () => ev(g, 'pathMade', 'ramp2')), 101010);
    ticks(g, 4);
    assert.equal(gained(g, () => ev(g, 'pathMade', 'ramp1')), 1010);
    assert.equal(lamp(g, 'combo_ramp2'), LAMP_MODES.pulse);
}

// 17. searchFail re-serves without ending the ball
{
    const g = mk(); launch(g); take(g);
    handle(g, { type: 'searchFail', id: 'search', ball: ball(g) });
    const q = take(g);
    assert.equal(of(q, 'addBall').length, 1);
    assert.equal(g.balls.ids.length, 1);
    assert.equal(g.phase, 'play');
    assert.equal(P(g).ball, 1);
}

// 18. determinism
{
    const script = () => {
        const g = mk({ players: ['A', 'B'] }); const all = [...take(g)];
        launch(g); ev(g, 'pathMade', 'ramp1'); ticks(g, 1); ev(g, 'pathMade', 'ramp2'); bumper(g);
        ev(g, 'rollover', 'laneT1'); button(g, 'right', true); nudge(g, 1); P(g).modeReady = true;
        handle(g, { type: 'hole', id: 'core', ball: ball(g) }); ticks(g, 5); ticks(g, 31);
        ev(g, 'bankDown', 'fw'); handle(g, { type: 'hole', id: 'vault', ball: ball(g) });
        ticks(g, 12); drain(g); ticks(g, 4);
        all.push(...take(g));
        return [copy(g), all];
    };
    assert.deepEqual(script(), script());
    for (const c of script()[1]) {
        if (c.type === 'cue') assert.ok(CUES.includes(c.name), c.name);
        if (c.type === 'music') assert.ok(MUSIC_SECTIONS.includes(c.section), c.section);
    }
}

// lampModes fills exactly one code per lamp; arrows follow the priority order
{
    const g = mk(); const out = new Uint8Array(layout.lamps.length).fill(9);
    lampModes(g, out);
    assert.equal(out.length, lampIds.length);
    assert.ok(out.every(v => v <= 6));
    assert.equal(lamp(g, 'kickbackL'), LAMP_MODES.on);
    P(g).modeReady = true;
    assert.equal(lamp(g, 'arrow_core'), LAMP_MODES.slow);
    P(g).ebLit = true;
    assert.equal(lamp(g, 'arrow_core'), LAMP_MODES.pulse, 'extra ball outranks mode start');
    P(g).lockLit = true;
    assert.equal(lamp(g, 'arrow_vault'), LAMP_MODES.on);
}
console.log('pinball rules ok');
