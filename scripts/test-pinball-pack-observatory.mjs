// Observatory rule pack tests (plan S5): scripted events against the real observatory layout. Run: node scripts/test-pinball-pack-observatory.mjs
import assert from 'node:assert/strict';
import { createGame, startGame, handle, tick, button, take, lampModes, toyState } from '../ui/js/desktop/apps/pinball/rules/core.js';
import { LAMP_MODES } from '../ui/js/desktop/apps/pinball/contracts.js';
import { TABLES } from '../ui/js/desktop/apps/pinball/tables/index.js';

const table = TABLES.observatory, layout = table.layout, lampIds = layout.lamps.map(l => l.id);
const mk = () => { const g = createGame(table, { players: ['A'], seed: 11 }); startGame(g); take(g); return g; };
const P = g => g.players[g.turn];
const ball = g => g.balls.ids[0];
const ev = (g, type, id, x = {}) => handle(g, { type, id, ball: ball(g), ...x });
const play = g => { handle(g, { type: 'launch', id: 'plunger', ball: ball(g), speed: 3, auto: false }); ev(g, 'bumper', 'popA'); take(g); return g; };
const of = (q, type) => q.filter(c => c.type === type);
const lamp = (g, id) => lampModes(g, new Uint8Array(lampIds.length))[lampIds.indexOf(id)];
const fire = {
    leftOrbit: g => ev(g, 'trigger', 'leftOrbitUp', { dir: 1 }), rightOrbit: g => ev(g, 'trigger', 'rightOrbitUp', { dir: 1 }),
    comet: g => ev(g, 'pathMade', 'cometRamp'), meridian: g => ev(g, 'pathMade', 'meridianRamp'),
    orrery: g => ev(g, 'trigger', 'orreryGate', { dir: 1 }), telescope: g => ev(g, 'hole', 'telescope', { via: 'field', count: 1 })
};
const star = g => ['starS', 'starT', 'starA', 'starR'].forEach(id => ev(g, 'standup', id));
const hasDmd = (q, text) => q.some(c => c.type === 'dmd' && c.scene === 'big' && c.params.text === text);
const discCmds = q => of(q, 'orrery').filter(c => c.id === 'orrery');
const pendCmds = q => of(q, 'pendulum').filter(c => c.id === 'pendulum');
const magnetCmds = (q, mode) => of(q, 'magnet').filter(c => c.mode === mode);

// 1. twelve hours lead to midnight: MIDNIGHT MULTIBALL with three balls, the clock back to 0, jackpots and the super jackpot
{
    const g = play(mk());
    for (let i = 0; i < 5; i++) fire.comet(g);
    assert.equal(P(g).table.hour, 5);
    assert.equal(lamp(g, 'clock5'), LAMP_MODES.on); assert.equal(lamp(g, 'clock6'), LAMP_MODES.slow); assert.equal(lamp(g, 'clock7'), LAMP_MODES.off);
    assert.ok(hasDmd(take(g), 'dmd_observatory_hour'));
    for (let i = 0; i < 6; i++) fire.meridian(g);
    assert.equal(g.mb.state, 'off', '11 hours: not midnight yet');
    fire.rightOrbit(g);
    assert.equal(g.mb.state, 'running');
    assert.equal(g.balls.ids.length, 3);
    assert.equal(P(g).table.hour, 0);
    const q = take(g);
    assert.ok(hasDmd(q, 'dmd_observatory_midnight') && q.some(c => c.type === 'dmd' && c.params.text === 'mode_observatory_mb'));
    assert.deepEqual(g.mb.litShots, ['comet', 'meridian', 'orrery', 'leftOrbit']);
    assert.ok(discCmds(q).some(c => c.speed === 15), 'multiball spins the disc at 15 rad/s');
    assert.equal(lamp(g, 'clock3'), LAMP_MODES.fast);
    fire.comet(g);
    assert.equal(P(g).table.hour, 0, 'the clock stands still during the multiball');
    for (const shot of ['meridian', 'orrery', 'leftOrbit']) { const s = P(g).score; fire[shot](g); assert.ok(P(g).score - s >= 1000000, shot + ' jackpot'); }
    assert.ok(g.mb.superUntil > g.time, 'four jackpots light the super jackpot');
    const s = P(g).score;
    fire.telescope(g);
    assert.ok(P(g).score - s >= 5000000, 'super jackpot on the telescope');
    const t = toyState(g);
    assert.equal(t.dome.open, 1); assert.equal(t.clock.midnight, true); assert.equal(Math.abs(t.disc.speed), 15, "the orrery jackpot flipped the direction");
}

// 2. S-T-A-R: mode lit, both wells grab, pendulum latched; the gravity slingshot pays when the telescope follows within 2 s
{
    const g = play(mk());
    for (const id of ['starS', 'starT', 'starA']) ev(g, 'standup', id);
    assert.equal(lamp(g, 'starA'), LAMP_MODES.on); assert.equal(lamp(g, 'starR'), LAMP_MODES.off);
    P(g).modeReady = false; take(g);
    const s0 = P(g).score;
    ev(g, 'standup', 'starR');
    const q = take(g);
    assert.equal(P(g).modeReady, true);
    assert.equal(P(g).table.hour, 1);
    assert.ok(P(g).score - s0 >= 100000);
    assert.deepEqual(magnetCmds(q, 'grab').map(c => c.id).sort(), ['wellMars', 'wellSaturn']);
    assert.ok(pendCmds(q).some(c => c.mode === 'park'), 'the pendulum latches');
    assert.ok(hasDmd(q, 'dmd_observatory_star') && hasDmd(q, 'dmd_observatory_wells') && hasDmd(q, 'dmd_observatory_latched'));
    assert.equal(lamp(g, 'wellMarsLit'), LAMP_MODES.slow); assert.equal(lamp(g, 'pendulumLit'), LAMP_MODES.on);
    assert.equal(lamp(g, 'starS'), LAMP_MODES.off, 'the bank resets');
    // a well holds the ball, the throw follows 0.8 s later, aimed up the telescope channel
    ev(g, 'magnet', 'wellSaturn', { state: 'held' });
    assert.equal(lamp(g, 'wellSaturnLit'), LAMP_MODES.fast);
    tick(g, 0.5); assert.equal(magnetCmds(take(g), 'throw').length, 0);
    tick(g, 0.4);
    const th = magnetCmds(take(g), 'throw');
    assert.equal(th.length, 1); assert.equal(th[0].id, 'wellSaturn'); assert.equal(th[0].speed, 1.6);
    assert.ok(th[0].angle > 1.4 && th[0].angle < 1.9, 'wellSaturn throws up the table: ' + th[0].angle);
    tick(g, 1.0);
    const sc = P(g).score;
    fire.telescope(g);
    assert.ok(P(g).score - sc >= 1000000, 'gravity slingshot');
    assert.ok(hasDmd(take(g), 'dmd_observatory_slingshot'));
    // too late pays nothing extra; the lit window and the latch run out
    const h = play(mk());
    tick(h, 9); star(h); ev(h, "magnet", "wellMars", { state: "held" }); tick(h, 1); take(h);
    tick(h, 2.5);
    const s2 = P(h).score;
    fire.telescope(h);
    assert.ok(P(h).score - s2 < 1000000, 'a late telescope is no slingshot');
    tick(h, 8); const late = take(h);
    assert.deepEqual(magnetCmds(late, 'off').map(c => c.id).sort(), ['wellMars', 'wellSaturn']);
    tick(h, 4);
    assert.ok(pendCmds(take(h)).some(c => c.mode === 'swing'), 'the pendulum swings again after 12 s');
}

// 3. the telescope opens the mode select; four played modes light the wizard; the telescope then starts GRAND ORRERY
{
    const g = play(mk());
    const choose = id => {
        P(g).modeReady = true; P(g).table.hour = 0;
        fire.telescope(g);
        if (g.select) {
            for (let i = 0, n = g.select.options.indexOf(id); i < n; i++) button(g, 'right', true);
            button(g, 'start', true);
        } // the last unplayed mode starts directly
        assert.equal(g.mode.id, id);
        take(g);
    };
    P(g).modeReady = true; fire.telescope(g);
    assert.ok(g.select, 'the telescope opens the mode select');
    button(g, 'start', true); assert.equal(g.mode.id, 'meteor'); take(g);
    assert.ok(discCmds(take(g)).length === 0);
    for (let i = 0; i < 5; i++) { (i % 2 ? fire.meridian : fire.comet)(g); P(g).table.hour = 0; }
    assert.equal(g.mode.id, null); assert.ok(P(g).modesCompleted.includes('meteor'));
    choose('transit');
    const tq = take(g); fire.leftOrbit(g);
    assert.ok(pendCmds(tq).length === 0);
    for (const s of ['orrery', 'telescope', 'rightOrbit']) { P(g).table.hour = 0; fire[s](g); }
    assert.ok(P(g).modesCompleted.includes('transit'), 'transit completes in order');
    assert.ok(P(g).ebLit, 'two completed modes light the extra ball');
    choose('conjunction');
    for (let i = 0; i < 4; i++) { P(g).table.hour = 0; fire[['leftOrbit', 'comet', 'orrery', 'telescope', 'meridian', 'rightOrbit'][g.mode.target]](g); }
    assert.ok(P(g).modesCompleted.includes('conjunction'));
    choose('supernova');
    for (let i = 0; i < 20; i++) ev(g, i % 2 ? 'bumper' : 'standup', i % 2 ? 'popB' : 'starS');
    assert.ok(P(g).modesCompleted.includes('supernova'));
    assert.equal(P(g).wizardReady, true);
    P(g).table.hour = 0; take(g);
    fire.telescope(g);
    assert.equal(g.mb.state, 'wizard'); assert.equal(g.balls.ids.length, 4);
    const q = take(g);
    assert.ok(q.some(c => c.type === 'dmd' && c.params.name === 'mode_observatory_wizard'));
    assert.ok(discCmds(q).some(c => c.speed === 24), 'the wizard spins the disc at 24 rad/s');
    assert.ok(pendCmds(q).some(c => c.mode === 'park'), 'the wizard parks the pendulum');
}

// 4. midnight while the mode select is open waits for the select to close
{
    const g = play(mk());
    P(g).modeReady = true; P(g).table.hour = 11;
    fire.telescope(g);
    assert.ok(g.select); assert.equal(g.mb.state, 'off'); assert.equal(P(g).table.midnightPending, true);
    tick(g, 4.5); tick(g, 0.05);
    assert.equal(g.mb.state, 'running'); assert.equal(P(g).table.hour, 0);
}

// 5. secret combo orrery -> telescope -> meridian
{
    const g = play(mk());
    tick(g, 1); // surprises arm 0.5 s after the launch
    fire.orrery(g); fire.telescope(g); take(g);
    const s = P(g).score;
    fire.meridian(g);
    assert.ok(JSON.stringify(take(g)).includes('dmd_secret_observatory'));
    assert.ok(P(g).score - s >= 750000);
}

// 6. super skill shot: hole:telescope within 8 s of the launch
{
    const g = mk();
    handle(g, { type: 'launch', id: 'plunger', ball: ball(g), speed: 3, auto: false });
    take(g);
    const s = P(g).score;
    tick(g, 7);
    fire.telescope(g);
    assert.ok(P(g).score - s >= 1000000);
    assert.ok(take(g).some(c => c.type === 'dmd' && c.scene === 'skillShot' && c.params.super));
}

// 7. the orrery shot flips the sign of the next orrery command; commands go out only on change
{
    const g = play(mk());
    tick(g, 0.1);
    let q = take(g);
    tick(g, 0.1); tick(g, 0.1);
    assert.equal(discCmds(take(g)).length, 0, 'no repeated disc commands');
    fire.orrery(g);
    q = take(g);
    assert.deepEqual(discCmds(q).map(c => c.speed), [-6]);
    assert.ok(hasDmd(q, 'dmd_observatory_reverse'));
    fire.orrery(g);
    assert.deepEqual(discCmds(take(g)).map(c => c.speed), [6]);
    // a ball start re-sends both commands
    P(g).table.hour = 0;
    const b = createGame(table, { players: ['A'], seed: 4 }); startGame(b);
    const bq = take(b);
    assert.equal(discCmds(bq).length, 1); assert.equal(pendCmds(bq).length, 1);
    assert.equal(pendCmds(bq)[0].mode, 'swing');
}

// 8. spinner pays 4000 extra in meteor, and the toy state follows C10
{
    const g = play(mk());
    let t = toyState(g);
    assert.deepEqual(Object.keys(t).sort(), ['clock', 'disc', 'dome', 'gears', 'orrery', 'pendulum', 'telescope']);
    assert.equal(t.disc.speed, 6); assert.equal(t.pendulum.mode, 'swing'); assert.equal(t.dome.open, 0); assert.equal(t.clock.midnight, false);
    assert.equal(lamp(g, 'spinnerLit'), LAMP_MODES.off);
    g.mode.id = 'meteor';
    assert.equal(lamp(g, 'spinnerLit'), LAMP_MODES.on);
    take(g);
    const s = P(g).score;
    ev(g, 'spinner', 'spinnerR');
    assert.equal(P(g).score - s, 1010 + 4000);
    g.mode.id = 'transit';
    assert.equal(toyState(g).pendulum.mode, 'fast'); assert.equal(lamp(g, 'pendulumLit'), LAMP_MODES.fast);
    g.mode.id = null;
    // a pendulum hit moves the clock at most once per 0.6 s
    take(g);
    ev(g, 'pendulum', 'pendulum', { speed: 1 }); ev(g, 'pendulum', 'pendulum', { speed: 1 });
    assert.equal(P(g).table.hour, 1);
    tick(g, 0.7);
    ev(g, 'pendulum', 'pendulum', { speed: 1 });
    assert.equal(P(g).table.hour, 2);
}

console.log('pinball observatory pack tests ok');
