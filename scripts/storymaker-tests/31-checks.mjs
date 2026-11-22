// Task C2 and following: the measured checks of checks.js, pure functions on the series the frame probe records.
import assert from 'node:assert/strict';
import { plain } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check } = t;
const SM = t.core();
const FPS = 30;

// pulses(period, count) = a series that holds 0.6, then dips to 0.1 and comes back `count` times, one dip every `period` frames.
// Each dip is two opposite transitions of 0.5, so one flash; the dip of flash k starts at frame 1 + k * period.
function pulses(period, count, tail) {
    const y = [0.6, 0.6];
    for (let k = 0; k < count; k++) for (let i = 0; i < period; i++) y.push(i < period / 2 ? 0.1 : 0.6);
    while (y.length < 2 + count * period + (tail || 0)) y.push(0.6);
    return y;
}

check('checks.flashes: a dip and a recovery is one flash and its start frame is the first frame of the dip', () => {
    assert.deepEqual(plain(SM.checks.flashes(pulses(4, 1, 6), FPS)), [1]);
    assert.deepEqual(plain(SM.checks.flashes(pulses(4, 3, 6), FPS)), [1, 5, 9]);
});

check('checks.flashes: three flashes in a second pass the rule only as a list, four give a window', () => {
    assert.equal(SM.checks.flashes(pulses(4, 3, 20), FPS).length, 3);
    assert.equal(SM.checks.flashes(pulses(4, 4, 20), FPS).length, 4);
    assert.deepEqual(plain(SM.checks.flashWindows([1, 5, 9], FPS)), []);
    assert.deepEqual(plain(SM.checks.flashWindows([1, 5, 9, 13], FPS)), [1]);
    // the fourth start exactly one second after the first is outside the window of fps frames
    assert.deepEqual(plain(SM.checks.flashWindows([1, 11, 21, 31], FPS)), []);
    assert.deepEqual(plain(SM.checks.flashWindows([1, 11, 21, 30], FPS)), [1]);
});

check('checks.flashes: a change below 0.10 is no transition', () => {
    const y = [];
    for (let i = 0; i < 60; i++) y.push(i % 2 ? 0.55 : 0.46);
    assert.deepEqual(plain(SM.checks.flashes(y, FPS)), []);
});

check('checks.flashes: alternation with the darker end at 0.80 or above is no flash', () => {
    const bright = [];
    for (let i = 0; i < 60; i++) bright.push(i % 4 < 2 ? 0.8 : 1.0);
    assert.deepEqual(plain(SM.checks.flashes(bright, FPS)), []);
    const dim = bright.map(v => v - 0.01);
    assert.ok(SM.checks.flashes(dim, FPS).length > 4, 'the same swing just below 0.80 is flashing');
});

check('checks.flashes: a slow ramp is one transition, not a flash, and a ramp splits only at its turning point', () => {
    const ramp = [];
    for (let i = 0; i < 30; i++) ramp.push(0.1 + i * 0.02);
    assert.deepEqual(plain(SM.checks.flashes(ramp, FPS)), []);
    const there = ramp.concat(ramp.slice().reverse());
    assert.deepEqual(plain(SM.checks.flashes(there, FPS)), [0]);
});

check('checks.flashes: short and empty series are safe', () => {
    assert.deepEqual(plain(SM.checks.flashes([], FPS)), []);
    assert.deepEqual(plain(SM.checks.flashes([0.5], FPS)), []);
});
