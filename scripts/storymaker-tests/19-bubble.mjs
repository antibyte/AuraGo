// Tasks S6-S8: speech and thought bubbles: bubble.js, the bubble entity, say with text and say without text.
import assert from 'node:assert/strict';
import { plain, renderAssets, FakeCanvas } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near, catalog } = t;
const SM = t.core();
const SAFE = { x: 5, y: 5, w: 90, h: 90 };
const resolver = () => SM.parts.makeResolver([{ text: '#1f2a44', bg2: '#ffe0a3' }]);
const layoutOf = (cv, lines, name) => SM.bubble.layout(cv.ctx, lines, t.byId('text.' + name), { canvasW: 1920, canvasH: 1080, safe: SAFE, family: 'Storymaker Default' });

check('bubble: speech draws a rounded box with a tail that ends on the anchor', () => {
    const cv = new FakeCanvas(1920, 1080), lay = layoutOf(cv, ['Hallo Welt'], 'speech');
    cv.ctx.log.length = 0;
    SM.bubble.draw(cv.ctx, lay, { x: 960, y: 500 }, 'speech', resolver(), 1);
    // the box sits above the anchor, so nothing is clamped and the tail tip is the anchor itself
    assert.ok(cv.ctx.log.includes('lineTo(960,500)'), 'the tail tip');
    assert.equal(cv.ctx.log.filter(l => l.startsWith('arcTo(')).length, 4, 'four rounded corners');
    assert.equal(cv.ctx.log.filter(l => l.startsWith('arc(')).length, 0, 'no circles');
    assert.ok(cv.ctx.log.some(l => l.startsWith('fillText(Hallo Welt,960,')), 'the text is centred in the box');
});

check('bubble: thought draws a rounded box and three circles, and no tail', () => {
    const cv = new FakeCanvas(1920, 1080), lay = layoutOf(cv, ['Hallo Welt'], 'thought');
    cv.ctx.log.length = 0;
    SM.bubble.draw(cv.ctx, lay, { x: 960, y: 500 }, 'thought', resolver(), 1);
    assert.equal(cv.ctx.log.filter(l => l.startsWith('arc(')).length, 3, 'three circles');
    assert.ok(!cv.ctx.log.includes('lineTo(960,500)'), 'no tail');
});

check('bubble: the box stays in the safe area for an actor at the edge', () => {
    const cv = new FakeCanvas(1920, 1080), lay = layoutOf(cv, ['Hallo Welt'], 'speech');
    // the safe area of 1920x1080: x from 5 % to 95 % is 96 to 1824, y from 5 % to 95 % is 54 to 1026
    for (const anchor of [{ x: 1900, y: 40 }, { x: 10, y: 40 }, { x: 1900, y: 1070 }, { x: 10, y: 1070 }]) {
        const b = SM.bubble.place(lay, anchor);
        assert.ok(b.x >= 96 && b.x + b.w <= 1824 && b.y >= 54 && b.y + b.h <= 1026, JSON.stringify(plain(b)));
        assert.ok(b.tip.y <= 1026, 'the tail stays inside too');
        assert.deepEqual(plain(SM.bubble.draw(cv.ctx, lay, anchor, 'speech', resolver(), 1)), plain(b), 'draw uses the clamped box');
    }
    const tl = SM.compile({ cast: [{ id: 'anna', character: 'char.office-woman-1' }], scenes: [{ id: 'a', duration: 2, actors: [{ cast: 'anna', at: { x: 100, y: 88 } }] }] }, {}, catalog).timeline;
    tl.scenes[0].entities.push(SM.scene.createBubble({ id: 'b', owner: 'anna', kind: 'speech', lines: ['Hallo Welt'], t0: 0, t1: 2 }));
    const out = new FakeCanvas(1920, 1080);
    SM.renderFrame(tl, 30, out.ctx, renderAssets());
    // 10 characters at 0.5 em of round(0.042 x 1080) = 45 px are 225 px, pad round(0.014 x 1080) = 15, so the box is 255 px.
    // The actor stands at x 1824, so the box touches the right safe edge and its centre is 1824 - 255 / 2 = 1696.5.
    assert.ok(out.ctx.log.some(l => l.startsWith('fillText(Hallo Welt,1696.5,')), 'renderFrame draws the clamped bubble');
});

check('bubble: fades in and out over 0.12 s, linear', () => {
    const b = SM.scene.createBubble({ id: 'b', owner: 'anna', kind: 'speech', lines: ['x'], t0: 0, t1: 2 });
    assert.equal(SM.bubble.FADE_SECONDS, 0.12);
    assert.equal(SM.scene.evalEntity(b, 0).alpha, 0);
    near(SM.scene.evalEntity(b, 0.06).alpha, 0.5, 1e-9, 'half of a 0.12 s fade');
    assert.equal(SM.scene.evalEntity(b, 1).alpha, 1);
    near(SM.scene.evalEntity(b, 1.94).alpha, 0.5, 1e-9, 'half of the fade out');
    assert.equal(SM.scene.evalEntity(b, 2).alpha, 0);
    const s = SM.scene.evalEntity(b, 1);
    assert.deepEqual([s.type, s.owner, s.kind, s.style], ['bubble', 'anna', 'speech', 'speech']);
    assert.deepEqual(plain(s.lines), ['x']);
});
