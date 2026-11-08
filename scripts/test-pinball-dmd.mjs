import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createDmd, TRANSITION_S, DMD_W, DMD_H } from '../ui/js/desktop/apps/pinball/dmd/core.js';
import { createRaster, quantize, needsLargeGlyphs } from '../ui/js/desktop/apps/pinball/dmd/raster.js';

const en = JSON.parse(readFileSync(new URL('../ui/lang/desktop/en.json', import.meta.url), 'utf8'));
const tr = key => en['pinball.' + key];
const make = extra => createDmd({ raster: createRaster({ canvas: false }), tr, strict: true, ...extra });
const hash = frame => { let h = 2166136261; for (const v of frame) h = Math.imul(h ^ v, 16777619) >>> 0; return h; };
const scenes = {
    blank: { transition: 'cut', draw(d) { d.clear(0); } },
    solid: { ms: 5000, priority: 5, transition: 'cut', draw(d) { d.clear(3); } },
    solidWipe: { ms: 5000, priority: 5, transition: 'wipeLeft', draw(d) { d.clear(3); } },
    one: { ms: 500, priority: 5, transition: 'cut', draw(d) { d.text('dmd_jackpot', 4, 4); } },
    two: { ms: 500, priority: 5, transition: 'cut', draw(d) { d.text('dmd_replay', 4, 4); } }
};

// Frame is 9216 bytes with levels 0..3 for every scene (smoke run in strict mode, so a throwing scene fails).
{
    const dmd = make();
    assert.equal(dmd.frame.length, 9216); assert.equal(DMD_W * DMD_H, 9216);
    const players = [{ name: 'A', score: 1234560 }, { name: 'B', score: 900 }];
    const params = {
        big: { text: 'dmd_kickback_lit', score: 5000 }, jackpot: { value: 1500000 }, superJackpot: { value: 9000000 }, lock: { n: 2 },
        multiball: {}, modeStart: { name: 'mode_neon_hack', hint: 'dmd_hint_ramps' }, modeComplete: { name: 'mode_neon_hack', score: 250000 },
        modeTotal: { score: 800000 }, select: { items: [{ key: 'mode_neon_hack' }, { key: 'mode_neon_chase', played: true }], index: 1, remaining: 2, total: 4 },
        skillShot: { value: 50000 }, ballSaved: {}, extraBall: {}, replay: {}, combo: { n: 3, value: 30000 }, tilt: {}, tiltWarning: { n: 2 },
        bonus: { phase: 'line', key: 'bonus_ramps', count: 4, value: 20000 }, match: { match: 30, last: 30, hit: true },
        initials: { letters: ['A', 'B', '_'], cursor: 1, rank: 2 }, gameOver: {}, shootAgain: {}, wizardStart: { name: 'mode_neon_wizard' },
        score: { players, turn: 0, ball: 1 }, modeStatus: { name: 'mode_neon_hack', remaining: 20, total: 30, count: 2, target: 5, score: 1000 },
        attract: { title: 'Cyber Neon Metropolis', scores: players, replayAt: 1000000, last: players }
    };
    for (const name of Object.keys(dmd.scenes)) {
        assert.ok(name in params || scenes[name], 'params for ' + name);
        for (const phase of [0, 0.4, 1.7, 4.5, 9]) {
            const probe = make();
            probe.play(name, params[name] || {}, { ms: 60000 });
            probe.update(phase);
            assert.ok(probe.frame.every(v => v >= 0 && v <= 3), name + ' levels');
        }
    }
    const bonusTotal = make(); bonusTotal.play('bonus', { phase: 'total', total: 50000 }); bonusTotal.update(2); assert.ok(bonusTotal.frame.some(v => v));
}

// Score 1,234,560 gives a stable hash across two runs (and is not empty).
{
    const run = () => { const dmd = make(); dmd.setBase('score', { players: [{ name: 'P1', score: 1234560 }], turn: 0, ball: 2 }); dmd.update(1); return dmd; };
    const a = run(), b = run();
    assert.equal(hash(a.frame), hash(b.frame));
    assert.ok(a.frame.filter(v => v === 3).length > 100);
    assert.equal(a.inspect().texts[0], 'PLAYER 1');
}

// Priority interrupt, FIFO queueing (max 6) and return to base.
{
    const dmd = make({ scenes });
    dmd.setBase('blank', {}); dmd.update(0.5);
    assert.equal(dmd.inspect().scene, null);
    dmd.play('one', {}, { priority: 3 }); dmd.update(0.1);
    assert.equal(dmd.inspect().scene, 'one');
    dmd.play('two', {}, { priority: 3 });                       // equal priority queues
    assert.deepEqual(dmd.inspect().queue, ['two']); assert.equal(dmd.inspect().scene, 'one');
    dmd.play('solid', {}, { priority: 7 }); dmd.update(0.1);    // higher priority interrupts
    assert.equal(dmd.inspect().scene, 'solid'); assert.ok(dmd.frame.every(v => v === 3));
    dmd.play('one', {}, { priority: 3 });
    for (let i = 0; i < 9; i++) dmd.play('two', {}, { priority: 3 });
    assert.equal(dmd.inspect().queue.length, 6);
    dmd.clear(); dmd.update(1);
    assert.deepEqual(dmd.inspect().queue, []); assert.equal(dmd.inspect().scene, null);
    dmd.play('one', {}, { priority: 3 }); dmd.play('two', {}, { priority: 3 });
    dmd.update(0.6); assert.equal(dmd.inspect().scene, 'two');  // 'one' timed out, queue advanced
    dmd.update(0.6); assert.equal(dmd.inspect().scene, null);   // back to base
    dmd.update(1); assert.ok(dmd.frame.every(v => v === 0));
}

// wipeLeft at 50% takes the left columns from the new scene.
{
    const dmd = make({ scenes });
    dmd.setBase('blank', {}); dmd.update(1);
    dmd.play('solidWipe', {}); dmd.update(TRANSITION_S.wipeLeft / 2);
    const row = y => Array.from(dmd.frame.slice(y * DMD_W, (y + 1) * DMD_W));
    for (const y of [0, 20, 47]) {
        const r = row(y);
        assert.ok(r.slice(0, 90).every(v => v === 3), 'left new');
        assert.ok(r.slice(102).every(v => v === 0), 'right old');
    }
    dmd.update(1); assert.ok(dmd.frame.every(v => v === 3));
}

// Quantise thresholds.
assert.deepEqual([0, 0.19, 0.2, 0.44, 0.45, 0.69, 0.7, 1].map(quantize), [0, 0, 1, 1, 2, 2, 3, 3]);

// Large glyph detection.
assert.equal(needsLargeGlyphs('日本'), true);
assert.equal(needsLargeGlyphs('JACKPOT'), false);
assert.equal(needsLargeGlyphs('한국어'), true);
assert.equal(needsLargeGlyphs('जैकपॉट'), true);

// inspect().text returns the last text drawn; vars are substituted.
{
    const dmd = make();
    dmd.play('big', { text: 'dmd_ball_locked', vars: { n: 2 } }); dmd.update(0.5);
    assert.equal(dmd.inspect().text, 'BALL 2 LOCKED');
    dmd.play('jackpot', { value: 1000 }, { priority: 8 }); dmd.update(0.5);
    assert.equal(dmd.inspect().text, 'JACKPOT');
}

console.log('pinball dmd ok');
