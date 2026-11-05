// Task C3: stage mapping (three aspects), vector parts, text wrapping, camera.
import assert from 'node:assert/strict';
import { plain, makeContext } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near, catalog } = t;
const SM = t.core();

check('stage: mapping for 16:9, 9:16 and 1:1 with cover fit, focus and spot overrides', () => {
    const plainEnv = t.byId('env.plain-stage'), office = t.byId('env.office');
    const wide = SM.stage.create({ aspect: '16:9', env: plainEnv });
    assert.deepEqual([wide.width, wide.height, wide.scale], [1920, 1080, 1]);
    assert.deepEqual(plain(wide.vis), { x: 0, y: 0, w: 1920, h: 1080 });
    assert.deepEqual(plain(wide.toCanvas(960, 540)), { x: 960, y: 540 });
    near(wide.spot('left').x, 576, 1e-9);
    near(wide.spot('left').y, 0.88 * 1080, 1e-9);
    const half = SM.stage.create({ aspect: '16:9', width: 960 });
    assert.deepEqual([half.width, half.height, half.scale], [960, 540, 0.5]);

    const tall = SM.stage.create({ aspect: '9:16', env: plainEnv });
    assert.deepEqual([tall.width, tall.height], [1080, 1920]);
    near(tall.scale, 1920 / 1080, 1e-9);
    near(tall.vis.w, 607.5, 1e-9); near(tall.vis.h, 1080, 1e-9); near(tall.vis.x, 960 - 303.75, 1e-9);
    const c = tall.toCanvas(960, 540);
    near(c.x, 540, 1e-9); near(c.y, 960, 1e-9);
    near(tall.spot('left').x, 0.42 * 1920, 1e-9, 'the 9:16 override of the environment wins');
    near(tall.spot('center').x, 960, 1e-9, 'a spot without an override and inside the slice keeps its design position');
    near(tall.spot('board').x, tall.vis.x + 0.74 * tall.vis.w, 1e-9, 'a spot without an override that is outside the slice is squeezed into it');
    const officeTall = SM.stage.create({ aspect: '9:16', env: office });
    near(officeTall.vis.x, 0.40 * 1920 - 303.75, 1e-9, 'focus_x of the environment');
    near(officeTall.spot('desk').x, 0.36 * 1920, 1e-9, 'the desk is inside the slice: the furniture is drawn there, so the actor stays there');
    assert.ok(officeTall.spot('far-left').x >= officeTall.vis.x, 'far-left would be outside the slice and is squeezed into it');
    assert.equal(officeTall.spot('desk').face, 'right');

    const square = SM.stage.create({ aspect: '1:1', env: office });
    assert.deepEqual([square.width, square.height, square.scale], [1080, 1080, 1]);
    near(square.vis.w, 1080, 1e-9); near(square.vis.x, 0.45 * 1920 - 540, 1e-9);
    near(square.toCanvas(square.vis.x + 540, 540).x, 540, 1e-9);

    // {x, y} is percent of the visible safe area
    const sp = wide.safePoint(0, 0), sp2 = wide.safePoint(100, 100), sp3 = wide.safePoint(50, 50);
    assert.deepEqual([sp.x, sp.y, sp2.x, sp2.y, sp3.x, sp3.y], [96, 54, 1824, 1026, 960, 540]);
    near(tall.safePoint(50, 50).x, 960, 1e-9);
    near(tall.safePoint(0, 0).x, tall.vis.x + 0.05 * tall.vis.w, 1e-9);
    assert.equal(wide.resolve('nowhere'), null);
    assert.equal(wide.resolve({ x: 50, y: 50 }).x, 960);

    // depth from the floor line, camera window clamped into the design frame
    near(wide.depthScale(0.88 * 1080), 0.8 + 0.25 * (18 / 26), 1e-9);
    assert.equal(wide.depthScale(0), 0.8); assert.equal(wide.depthScale(1080), 1.05);
    const win = wide.cameraWindow({ zoom: 2, cx: 100, cy: 100 });
    assert.deepEqual([win.x, win.y, win.w, win.h, win.scale], [0, 0, 960, 540, 2]);
    assert.deepEqual(plain(wide.matrix(win)), [2, 0, 0, 2, 0, 0]);
});

check('parts: shapes become paths with fills, strokes, gradients and token resolution', () => {
    SM.parts.resetUnresolved();
    const ctx = makeContext(null);
    const resolve = SM.parts.makeResolver([{ skin: '#aa8866' }, { accent: '#ff0000', fg: '$skin' }]);
    SM.parts.drawShapes(ctx, [
        { d: 'M0,0 L10,0 L10,10 Z', fill: '$skin', stroke: '$accent', stroke_width: 3, opacity: 0.5 },
        { d: 'M0,0 L5,5', fill: 'none', stroke: '#000' },
        { d: 'M0,0 L5,5', gradient: { type: 'linear', stops: [[0, '$accent'], [1, '#00ff00']], x1: 0, y1: 0, x2: 10, y2: 0 } },
        { d: 'M0,0 L5,5', gradient: { type: 'radial', stops: [[0, '#fff'], [1, '$nope']], cx: 5, cy: 5, r: 4 } }
    ], resolve, 1);
    const log = ctx.log.join('\n');
    assert.match(log, /set globalAlpha=0.5/); assert.match(log, /set fillStyle=#aa8866/); assert.match(log, /set strokeStyle=#ff0000/);
    assert.match(log, /fill\(path\(M0,0 L10,0 L10,10 Z\)\)/); assert.match(log, /set lineWidth=3/);
    assert.match(log, /createLinearGradient\(0,0,10,0\)=gradient/); assert.match(log, /createRadialGradient\(5,5,0,5,5,4\)=gradient/);
    assert.match(log, /stop\(0,#ff0000\)/); assert.match(log, /stop\(1,#ff00ff\)/);
    assert.deepEqual(plain(SM.parts.unresolvedTokens()), ['nope'], 'an unresolved token draws magenta and is recorded');
    assert.equal(resolve('$fg'), '#ff00ff', 'a token that points to another token does not resolve');
    SM.parts.resetUnresolved();
});

check('text: wrapping with measureText, max_lines with an ellipsis, long words, newlines', () => {
    const measure = s => s.length * 5;   // 10 px font, 0.5 em per character
    const ell = String.fromCharCode(0x2026);
    assert.deepEqual(plain(SM.text.wrap(measure, 'aaa bbb ccc', 40, 5).lines), ['aaa bbb', 'ccc']);
    const cut = SM.text.wrap(measure, 'aaa bbb ccc ddd eee fff', 40, 2);
    assert.equal(cut.lines.length, 2); assert.equal(cut.truncated, true);
    assert.ok(cut.lines[1].endsWith(ell)); assert.ok(measure(cut.lines[1]) <= 40);
    assert.deepEqual(plain(SM.text.wrap(measure, 'abcdefghijklmnopqrstuv', 40, 5).lines), ['abcdefgh', 'ijklmnop', 'qrstuv']);
    assert.deepEqual(plain(SM.text.wrap(measure, 'one\ntwo three', 100, 5).lines), ['one', 'two three']);
    assert.deepEqual(plain(SM.text.wrap(measure, '', 100, 2).lines), ['']);
    const ctx = makeContext(null), style = t.byId('text.title');
    const lay = SM.text.layout(ctx, 'Ein ziemlich langer Titel, der umbrochen werden muss, weil er nicht passt', style, { canvasH: 1080, maxWidth: 900, family: 'Storymaker Default' });
    assert.equal(lay.px, Math.round(0.07 * 1080));
    assert.equal(lay.font, '700 76px "Storymaker Default", sans-serif');
    assert.ok(lay.lines.length <= 2, 'max_lines of the title style');
    assert.ok(lay.lines.every(l => ctx.measureText(l).width <= 900));
    near(SM.text.maxWidthFor(1920, { x: 5, y: 5, w: 90, h: 90 }, 960, 'center'), 1728, 1e-9);
    near(SM.text.maxWidthFor(1920, { x: 5, y: 5, w: 90, h: 90 }, 400, 'center'), 2 * (400 - 96), 1e-9);
    near(SM.text.maxWidthFor(1920, { x: 5, y: 5, w: 90, h: 90 }, 100, 'center'), 432, 1e-9, 'never narrower than a quarter of the safe width');
});

check('camera: framings, zoom and pan tweens, shake bursts decay and never leave the zoom range', () => {
    assert.deepEqual(plain(SM.camera.FRAMING), { wide: 1, medium: 1.35, close: 1.9 });
    const cam = SM.camera.create(1, 960, 540);
    SM.camera.addZoom(cam, 1, 2, 2, 'linear');
    SM.camera.addPan(cam, 1, 2, 1200, 600, 'linear');
    SM.camera.addShake(cam, 3, 4, 10);
    const mid = SM.camera.at(cam, 1.5);
    // zoom runs in log space: 1 * (2 / 1)^0.5. The pan stays linear.
    near(mid.zoom, Math.SQRT2, 1e-9); near(mid.cx, 1080, 1e-9); near(mid.cy, 570, 1e-9);
    assert.equal(SM.camera.at(cam, 0).zoom, 1);
    assert.equal(SM.camera.at(cam, 2.5).zoom, 2);
    assert.equal(SM.camera.at(cam, 2.5).shakeX, 0);
    const early = SM.camera.at(cam, 3.05), late = SM.camera.at(cam, 3.95);
    assert.ok(Math.abs(early.shakeX) + Math.abs(early.shakeY) > 0 && Math.abs(early.shakeX) <= 10 && Math.abs(late.shakeX) < 1, 'the burst decays');
    assert.deepEqual(plain(SM.camera.at(cam, 3.05)), plain(SM.camera.at(cam, 3.05)), 'shake is a pure function of t');
    assert.equal(SM.camera.clampZoom(0.2), 1); assert.equal(SM.camera.clampZoom(99), SM.camera.MAX_ZOOM);
    near(SM.camera.end(cam), 4, 1e-9);
    const four = SM.camera.create(1, 960, 540);
    SM.camera.addZoom(four, 0, 1, 4, 'linear');
    near(SM.camera.at(four, 0.5).zoom, 2, 1e-9, 'log zoom: 1 * (4 / 1)^0.5');
    // the stage turns the camera into a window inside the design frame
    const stage = SM.stage.create({ aspect: '16:9' }), win = stage.cameraWindow(SM.camera.at(cam, 2.5));
    near(win.w, 960, 1e-9); near(win.x, 1200 - 480, 1e-9); near(win.y, 600 - 270, 1e-9);
});
