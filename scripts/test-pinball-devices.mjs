// Device suite (T03): bumper, sling, drop bank, spinner, gate, hole, kickback, magnet, captive, door, ball search.
import assert from 'node:assert/strict';
import { createWorld } from '../ui/js/desktop/apps/pinball/physics/world.js';
import { STEP } from '../ui/js/desktop/apps/pinball/physics/constants.js';
import { rngNext } from '../ui/js/desktop/apps/pinball/config.js';
import { run, energy } from './lib/pinball-sim.mjs';

// A closed box x +-0.15, y 0.2..1.0 (no plunger lane, no drain) plus the given devices and extra walls.
function boxLayout(devices, walls = []) {
    const box = { id: 'box', material: 'metal', closed: true, points: [[-0.15, 0.2], [0.15, 0.2], [0.15, 1.0], [-0.15, 1.0]] };
    return { id: 'dev', version: 1, meta: { troughSize: 4 }, flippers: [], plunger: { laneX: [9, 10], zoneYMax: 0 }, drainY: -1, walls: [box, ...walls], posts: [], devices, paths: [] };
}
const speedOf = b => Math.hypot(b.vx, b.vy);
const ballOf = (w, id) => w.snapshot().balls.find(b => b.id === id);
const ofType = (events, type, id) => events.filter(e => e.type === type && (id === undefined || e.id === id));
// Steps until a predicate on the new events holds (or maxTicks); returns {events, ticks}.
function until(w, maxTicks, pred) {
    const events = [];
    for (let t = 1; t <= maxTicks; t++) {
        w.step();
        const evs = w.events();
        for (const e of evs) events.push(e);
        if (pred(evs)) return { events, ticks: t };
    }
    return { events, ticks: maxTicks };
}

// ---- bumper: kick and event; coils off -> passive ----------------------------------------------------------------
{
    const layout = boxLayout([{ id: 'b1', type: 'bumper', x: 0, y: 0.5 }]);
    const w = createWorld(layout);
    w.placeBall(1, 0, 0.58, 0, -1);
    const r = until(w, 600, evs => ofType(evs, 'bumper', 'b1').length > 0);
    assert.equal(ofType(r.events, 'bumper', 'b1').length, 1, 'bumper event');
    assert.ok(w.snapshot().anim[w.animIndex.get('b1')] > 0.5, 'bumper fire anim');
    w.step();
    assert.ok(speedOf(ballOf(w, 1)) >= 1.9, 'bumper kick ' + speedOf(ballOf(w, 1)));

    const off = createWorld(layout);
    off.command({ type: 'coils', on: false });
    let last = null, gained = 0;
    off.placeBall(1, 0.004, 0.58, 0, -3);
    const events = run(off, 480, null, (t, ww) => {
        const b = ballOf(ww, 1), e = energy(b);
        if (last !== null && e > last + 1e-9) gained += e - last;
        last = e;
    });
    assert.equal(ofType(events, 'bumper').length, 0, 'no bumper event with coils off');
    assert.equal(gained, 0, 'no energy gain with coils off');
    assert.ok(ofType(events, 'impact').length > 0, 'the ball did touch the bumper');
}

// ---- sling: threshold ---------------------------------------------------------------------------------------------
{
    const layout = boxLayout([{ id: 's1', type: 'sling', face: 'face' }], [{ id: 'face', material: 'rubber', closed: false, points: [[-0.05, 0.4], [0.05, 0.4]] }]);
    const w = createWorld(layout);
    w.placeBall(1, 0, 0.45, 0, -0.6);
    const r = until(w, 600, evs => ofType(evs, 'sling', 's1').length > 0);
    assert.equal(ofType(r.events, 'sling', 's1').length, 1, 'sling event at 0.6 m/s');
    w.step();
    assert.ok(speedOf(ballOf(w, 1)) >= 1.9, 'sling kick ' + speedOf(ballOf(w, 1)));
    const slow = createWorld(layout);
    slow.placeBall(1, 0, 0.4165, 0, -0.2);
    const events = run(slow, 240);
    assert.equal(ofType(events, 'sling').length, 0, 'no sling event at 0.2 m/s');
}

// ---- drop bank ----------------------------------------------------------------------------------------------------
{
    const targets = [-0.03, 0, 0.03].map((x, i) => ({ id: 't' + i, x, y: 0.5 }));
    const layout = boxLayout([{ id: 'bank', type: 'dropBank', targets, angle: Math.PI / 2 }]);
    const w = createWorld(layout);
    const all = [];
    for (let i = 0; i < 3; i++) {
        w.placeBall(1, targets[i].x, 0.56, 0, -1);
        const r = until(w, 600, evs => ofType(evs, 'drop').length > 0);
        all.push(...r.events);
        assert.equal(ofType(r.events, 'drop')[0].id, 't' + i); assert.equal(ofType(r.events, 'drop')[0].bank, 'bank');
        w.command({ type: 'removeBall', id: 1 });
    }
    all.push(...run(w, 80));
    assert.equal(ofType(all, 'bankDown', 'bank').length, 1, 'bankDown once');
    const anim = w.snapshot().anim;
    assert.ok(targets.every(t => anim[w.animIndex.get(t.id)] === 1), 'targets down in anim');
    w.placeBall(2, 0, 0.56, 0, -1.5);
    run(w, 240);
    assert.ok(ballOf(w, 2).y < 0.45, 'ball passes through the lowered bank: y ' + ballOf(w, 2).y);
    w.command({ type: 'removeBall', id: 2 });
    // reset is blocked while a ball overlaps, then happens once it left
    w.placeBall(3, 0, 0.5, 0, 0);
    w.command({ type: 'resetBank', id: 'bank' });
    const blocked = run(w, 20, () => { w.placeBall(3, 0, 0.5, 0, 0); });
    assert.equal(ofType(blocked, 'bankReset').length, 0, 'reset blocked by the ball');
    const freed = run(w, 400);
    assert.equal(ofType(freed, 'bankReset', 'bank').length, 1, 'bankReset after the ball left');
    assert.ok(targets.every(t => w.snapshot().anim[w.animIndex.get(t.id)] === 0), 'targets up again');
}

// ---- spinner ------------------------------------------------------------------------------------------------------
{
    const layout = boxLayout([{ id: 'sp', type: 'spinner', a: [-0.03, 0.5], b: [0.03, 0.5] }]);
    const revs = v => {
        const w = createWorld(layout), events = [];
        w.placeBall(1, 0, 0.45, 0, v);
        for (let t = 0; t < 1440; t++) {
            w.step();
            for (const e of w.events()) events.push(e);
            if (t === 120) w.command({ type: 'removeBall', id: 1 });
        }
        return ofType(events, 'spinner', 'sp').length;
    };
    const fast = revs(4), slow = revs(1);
    assert.ok(fast >= 15 && fast <= 35, 'spinner at 4 m/s: ' + fast);
    assert.ok(slow < 8 && slow >= 1, 'spinner at 1 m/s: ' + slow);
}

// ---- gate ---------------------------------------------------------------------------------------------------------
{
    const layout = boxLayout([{ id: 'g', type: 'gate', a: [-0.05, 0.5], b: [0.05, 0.5], pass: [0, 1], mode: 'oneWay' }]);
    const up = createWorld(layout);
    up.placeBall(1, 0, 0.45, 0, 1.5);
    const evUp = run(up, 120);
    assert.ok(ballOf(up, 1).y > 0.52, 'passes along the pass direction');
    assert.equal(ofType(evUp, 'gate', 'g')[0].dir, 1);
    const down = createWorld(layout);
    down.placeBall(1, 0, 0.56, 0, -1);
    let minY = 9;
    run(down, 480, null, (t, ww) => { minY = Math.min(minY, ballOf(ww, 1).y); });
    assert.ok(minY > 0.5, 'blocked against the pass direction: ' + minY);
}

// ---- hole ---------------------------------------------------------------------------------------------------------
{
    const hole = (extra = {}) => ({ id: 'h', type: 'hole', kind: 'saucer', x: 0, y: 0.5, eject: { angleDeg: 60, speed: 2.5 }, ...extra });
    // capture at 0.5 m/s, eject vector
    const w = createWorld(boxLayout([hole()]));
    w.placeBall(1, 0, 0.54, 0, -0.5);
    const cap = until(w, 600, evs => ofType(evs, 'hole').length > 0);
    assert.equal(ofType(cap.events, 'hole')[0].via, 'field'); assert.equal(ofType(cap.events, 'hole')[0].count, 1);
    run(w, 60);
    assert.equal(ballOf(w, 1).mode, 'held'); assert.equal(w.stats().held, 1);
    w.command({ type: 'eject', id: 'h', count: 1, delay: 0 });
    const ej = until(w, 20, evs => ofType(evs, 'eject').length > 0);
    assert.equal(ofType(ej.events, 'eject')[0].via, 'field');
    const b = ballOf(w, 1), ang = Math.atan2(b.vy, b.vx);
    assert.ok(Math.abs(speedOf(b) - 2.5) <= 0.125, 'eject speed ' + speedOf(b));
    assert.ok(Math.abs(ang - Math.PI / 3) <= 0.05, 'eject angle ' + ang);
    // a 1.5 m/s pass is not captured
    const fast = createWorld(boxLayout([hole()]));
    fast.placeBall(1, 0, 0.54, 0, -1.5);
    assert.equal(ofType(run(fast, 120), 'hole').length, 0, 'saucer pass at 1.5 m/s');
    // autoEject at 6 s
    const auto = createWorld(boxLayout([hole({ eject: { angleDeg: 90, speed: 0.4 } })]));
    auto.placeBall(1, 0, 0.54, 0, -0.5);
    const evs = run(auto, 3200);
    const tHole = ofType(evs, 'hole')[0].tick, tEject = ofType(evs, 'eject')[0].tick;
    assert.ok(Math.abs((tEject - tHole) * STEP - 6) < 0.05, 'autoEject after ' + (tEject - tHole) * STEP);
    // capacity 3: lock three, no autoEject, eject all 0.6 s apart
    const multi = createWorld(boxLayout([hole({ capacity: 3 })]));
    const log = [];
    for (let i = 1; i <= 3; i++) {
        multi.placeBall(i, 0, 0.54, 0, -0.5);
        log.push(...until(multi, 600, e => ofType(e, 'hole').length > 0).events);
        multi.command({ type: 'lockBall', id: 'h' });
    }
    assert.deepEqual(ofType(log, 'hole').map(e => e.count), [1, 2, 3]);
    log.length = 0;
    log.push(...run(multi, 3500));
    assert.equal(ofType(log, 'eject').length, 0, 'locked balls stay');
    multi.command({ type: 'eject', id: 'h', count: 'all', delay: 0 });
    const out = ofType(run(multi, 1700), 'eject');
    assert.equal(out.length, 3);
    assert.ok(Math.abs((out[1].tick - out[0].tick) * STEP - 0.6) < 0.02 && Math.abs((out[2].tick - out[1].tick) * STEP - 0.6) < 0.02, 'eject spacing');
    // releaseToTrough and enable
    const rel = createWorld(boxLayout([hole()]));
    rel.placeBall(1, 0, 0.54, 0, -0.5);
    until(rel, 600, e => ofType(e, 'hole').length > 0);
    rel.command({ type: 'releaseToTrough', id: 'h' });
    assert.equal(rel.stats().balls, 0, 'balls returned to the trough');
    rel.command({ type: 'holeEnable', id: 'h', on: false });
    rel.placeBall(2, 0, 0.54, 0, -0.5);
    assert.equal(ofType(run(rel, 240), 'hole').length, 0, 'disabled hole ignores the ball');
    // coils off: hole ejects after 0.5 s
    const tilt = createWorld(boxLayout([hole()]));
    tilt.placeBall(1, 0, 0.54, 0, -0.5);
    until(tilt, 600, e => ofType(e, 'hole').length > 0);
    tilt.command({ type: 'coils', on: false });
    const te = until(tilt, 400, e => ofType(e, 'eject').length > 0);
    assert.ok(te.ticks * STEP >= 0.45 && te.ticks * STEP <= 0.56, 'tilt eject after ' + te.ticks * STEP);
    // path hooks: deliverToHole captures at any speed; eject into a path uses ctx.launchPath, else straight up
    const hk = createWorld(boxLayout([hole({ id: 'p', y: 0.6, eject: { path: 'ramp', speed: 1.5 } })]));
    const ball = hk.placeBall(1, 0.1, 0.8, 0, -4);
    ball.mode = 'path';
    assert.equal(hk.ctx.deliverToHole(ball, 'p'), true);
    assert.equal(hk.ctx.deliverToHole(hk.placeBall(2, 0, 0.9, 0, 0), 'p'), false, 'full hole refuses');
    const hev = hk.events();
    assert.equal(ofType(hev, 'hole')[0].via, 'path');
    hk.command({ type: 'eject', id: 'p', count: 1, delay: 0 });
    const fb = until(hk, 10, e => ofType(e, 'eject').length > 0);
    assert.equal(ofType(fb.events, 'eject')[0].via, 'field'); assert.ok(ballOf(hk, 1).vy > 1.3 && Math.abs(ballOf(hk, 1).vx) < 0.1, 'fallback is straight up');
    hk.command({ type: 'removeBall', id: 2 }); hk.command({ type: 'removeBall', id: 1 }); hk.events();
    const calls = [];
    hk.ctx.launchPath = (bl, path, speed) => { calls.push([path, speed]); bl.mode = 'path'; return true; };
    hk.placeBall(3, 0, 0.64, 0, -0.5);
    until(hk, 600, e => ofType(e, 'hole').length > 0);
    hk.command({ type: 'eject', id: 'p', count: 1, delay: 0 });
    const pe = until(hk, 10, e => ofType(e, 'eject').length > 0);
    assert.equal(ofType(pe.events, 'eject')[0].via, 'path'); assert.equal(calls.length, 1); assert.equal(calls[0][0], 'ramp');
}

// ---- kickback -----------------------------------------------------------------------------------------------------
{
    const w = createWorld(boxLayout([{ id: 'k', type: 'kickback', x: 0, y: 0.5, armed: true }]));
    w.placeBall(1, 0, 0.53, 0, -0.5);
    const r = until(w, 600, e => ofType(e, 'kickback').length > 0);
    assert.equal(ofType(r.events, 'kickback').length, 1);
    w.step();
    assert.ok(ballOf(w, 1).vy >= 3.5, 'kickback vy ' + ballOf(w, 1).vy);
    w.placeBall(1, 0, 0.53, 0, -0.5);
    assert.equal(ofType(run(w, 960), 'kickback').length, 0, 'disarmed after firing');
}

// ---- magnet -------------------------------------------------------------------------------------------------------
{
    const w = createWorld(boxLayout([{ id: 'm', type: 'magnet', x: 0, y: 0.5 }]));
    w.command({ type: 'magnet', id: 'm', mode: 'grab' });
    w.placeBall(1, 0.02, 0.52, 0, 0);
    const r = until(w, 480, e => ofType(e, 'magnet').some(x => x.state === 'held'));
    assert.ok(r.ticks * STEP <= 1.0, 'magnet holds within 1 s: ' + r.ticks * STEP);
    assert.equal(w.stats().held, 1);
    run(w, 240);
    assert.equal(ballOf(w, 1).mode, 'held'); assert.ok(Math.hypot(ballOf(w, 1).x, ballOf(w, 1).y - 0.5) < 0.004);
    w.command({ type: 'magnet', id: 'm', mode: 'throw', angle: 0.5, speed: 2 });
    w.step();
    const b = ballOf(w, 1);
    assert.ok(Math.abs(b.vx - 2 * Math.cos(0.5)) < 0.05 && Math.abs(b.vy - 2 * Math.sin(0.5)) < 0.05, 'throw vector ' + b.vx + ',' + b.vy);
    assert.equal(b.mode, 'field');
    assert.ok(ofType(w.events(), 'magnet').every(e => e.state !== 'held'));
}

// ---- captive ------------------------------------------------------------------------------------------------------
{
    const layout = boxLayout([{ id: 'cap', type: 'captive', channel: { a: [0, 0.4], b: [0, 0.6] }, rest: [0, 0.4102], endSwitch: 'capEnd' }]);
    const w = createWorld(layout);
    w.placeBall(1, 0, 0.37, 0, 2);
    const r = until(w, 200, e => ofType(e, 'captive').length > 0);
    const ev = ofType(r.events, 'captive')[0];
    assert.equal(ev.id, 'cap'); assert.equal(ev.ball, 1, 'captive event carries the striking ball');
    const more = run(w, 400);
    assert.equal(ofType([...r.events, ...more], 'trigger', 'capEnd').length >= 1, true, 'end switch');

    const rng = { seed: 5 }, g = createWorld(layout);
    let strikes = 0;
    for (let n = 0; n < 1000; n++) {
        const sp = 1 + rngNext(rng) * 5, ang = Math.PI / 2 + (rngNext(rng) - 0.5) * 0.2;
        g.command({ type: 'clearBalls' });
        g.placeBall(1, (rngNext(rng) - 0.5) * 0.008, 0.34 + rngNext(rng) * 0.02, sp * Math.cos(ang), sp * Math.sin(ang));
        for (let t = 0; t < 150; t++) {
            g.step();
            strikes += g.events().filter(e => e.type === 'captive').length;
            const c = ballOf(g, 'cap_ball'), s = ballOf(g, 1);
            assert.ok(Math.abs(c.x) < 1e-9 && c.y >= 0.4 && c.y <= 0.6, 'hit ' + n + ': captive ball left the channel at ' + c.x + ',' + c.y);
            assert.ok(!(Math.abs(s.x) < 0.017 && s.y > 0.41 && s.y < 0.6), 'hit ' + n + ': striker entered the channel');
        }
    }
    assert.ok(strikes > 600, 'enough strikes: ' + strikes);
}

// ---- door ---------------------------------------------------------------------------------------------------------
{
    const w = createWorld(boxLayout([{ id: 'd', type: 'door', a: [-0.05, 0.5], b: [0.05, 0.5] }]));
    w.placeBall(1, 0, 0.56, 0, -1);
    let minY = 9;
    run(w, 480, null, (t, ww) => { minY = Math.min(minY, ballOf(ww, 1).y); });
    assert.ok(minY > 0.5, 'closed door blocks: ' + minY);
    w.command({ type: 'door', id: 'd', open: true });
    run(w, 240);
    assert.ok(w.snapshot().anim[w.animIndex.get('d')] > 0.99, 'door anim open');
    w.placeBall(1, 0, 0.56, 0, -1);
    run(w, 240);
    assert.ok(ballOf(w, 1).y < 0.45, 'open door passes: ' + ballOf(w, 1).y);
}

// ---- ball search --------------------------------------------------------------------------------------------------
{
    const w = createWorld(boxLayout([{ id: 'b1', type: 'bumper', x: 0.1, y: 0.8 }]));
    w.placeBall(1, 0, 0.2135, 0, 0);
    const events = run(w, 12000);
    assert.equal(ofType(events, 'search').length, 5, 'five search cycles');
    assert.equal(ofType(events, 'searchFail').length, 1, 'searchFail');
    assert.equal(w.stats().balls, 0);
}

console.log('pinball devices ok');
