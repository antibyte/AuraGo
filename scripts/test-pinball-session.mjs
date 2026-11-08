// Session tests (plan T17): a seeded Neon game played by the bot for 30 s of simulation, and tick-level determinism
// across 30/60/144 Hz frame accumulators.
import assert from 'node:assert/strict';
import { createSession } from '../ui/js/desktop/apps/pinball/session.js';
import { tableOf } from '../ui/js/desktop/apps/pinball/tables/index.js';
import { createBot } from '../ui/testdata/pinball-bot.js';
import { STEP } from '../ui/js/desktop/apps/pinball/physics/constants.js';
import { PRESENTATION_COMMANDS } from '../ui/js/desktop/apps/pinball/contracts.js';

const table = tableOf('neon');
const lampCount = table.layout.lamps.length;

function play(seconds, { seed = 11 } = {}) {
    const session = createSession(table, { seed });
    const presentation = {}, events = {};
    session.onPresentation(c => { presentation[c.type] = (presentation[c.type] || 0) + 1; });
    session.onEvent(e => { events[e.type] = (events[e.type] || 0) + 1; });
    session.startGame({ players: ['ABC'], seed });
    const bot = createBot(table.layout, seed), phases = new Set(), modes = new Uint8Array(lampCount);
    const total = Math.round(seconds / STEP);
    for (let t = 0; t < total; t++) {
        session.tick(bot.controls(session.snapshotPair().curr.balls, t));
        phases.add(session.game.phase);
        if (t % 480 === 0) session.lampModes(modes);
    }
    return { session, presentation, events, phases };
}

// 1. Bot game: balls served, phases advance, presentation routed, lamp modes sized to the layout.
{
    const { session, presentation, events, phases } = play(30);
    assert.ok((events.shooter || 0) >= 1, 'a ball is served to the shooter lane');
    assert.ok((events.launch || 0) >= 1, 'the bot launches');
    assert.ok(phases.has('play'), 'phase reaches play: ' + [...phases]);
    assert.ok(session.game.players[0].score > 0, 'the bot scores');
    assert.ok(presentation.dmdBase >= 1, 'dmdBase routed');
    assert.ok((presentation.music || 0) + (presentation.cue || 0) >= 1, 'cues routed');
    for (const type of Object.keys(presentation)) assert.ok(PRESENTATION_COMMANDS.includes(type), 'only presentation commands reach listeners: ' + type);
    const modes = new Uint8Array(lampCount);
    session.lampModes(modes);
    assert.equal(modes.length, lampCount);
    assert.ok(modes.some(Boolean), 'some lamp is lit in play');
    const dmd = session.dmdState();
    assert.ok(dmd.scene === 'score' || dmd.scene === 'modeStatus', 'dmd state in play: ' + dmd.scene);
    session.endGame();
    assert.equal(session.game, null);
    assert.equal(session.dmdState().scene, 'attract');
    session.dispose();
    console.log('ok   bot game: phases ' + [...phases].join(',') + '');
}

// 2. Nudge: allowed in play, tilt after repeated nudges (non-training).
{
    const session = createSession(table, { seed: 3 });
    assert.equal(session.nudge(0.2, 0), false, 'no nudge without a game');
    session.startGame({ players: ['A'], seed: 3 });
    for (let t = 0; t < 480 * 2; t++) session.tick({ plunger: t < 200 ? 1 : 0 });
    assert.equal(session.nudge(0.2, 0), true);
    let allowed = true;
    for (let i = 0; i < 40 && allowed; i++) { allowed = session.nudge(0.2, 0); for (let t = 0; t < 240; t++) session.tick({}); }
    assert.equal(allowed, false, 'repeated nudges tilt');
    session.dispose();
    console.log('ok   nudge and tilt');
}

// 3. Determinism across frame rates: same per-tick controls, any frame grouping.
function runAt(hz, seconds) {
    const session = createSession(table, { seed: 9 });
    session.startGame({ players: ['AAA', 'BBB'], seed: 9 });
    const bot = createBot(table.layout, 9), total = Math.round(seconds / STEP);
    let acc = 0, t = 0;
    while (t < total) {
        acc += 1 / hz;
        let n = 0;
        while (acc >= STEP - 1e-12 && t < total && n < 32) { session.tick(bot.controls(session.snapshotPair().curr.balls, t)); acc -= STEP; t++; n++; }
        if (n >= 32) acc = 0;
    }
    return { game: JSON.stringify(session.game), snap: JSON.stringify(session.snapshot()), score: session.game.players[0].score };
}
{
    const a = runAt(30, 20), b = runAt(60, 20), c = runAt(144, 20);
    assert.equal(a.game, b.game, '30 vs 60 Hz game state');
    assert.equal(b.game, c.game, '60 vs 144 Hz game state');
    assert.equal(a.snap, b.snap); assert.equal(b.snap, c.snap);
    console.log('ok   determinism at 30/60/144 Hz (score ' + a.score + ')');
}

// 4. Stress hook places the requested balls.
{
    const session = createSession(table, { seed: 2 });
    session.startGame({ players: ['T'], training: true, seed: 2 });
    session.stress(4);
    for (let i = 0; i < 480; i++) session.tick({});
    assert.equal(session.world.stats().balls >= 3, true);
    session.dispose();
    console.log('ok   stress');
}
console.log('session ok');
