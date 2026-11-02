// Task R: the player against the real starter catalog (ui/img/storymaker, part D). The rules where the two parts had to be
// reconciled (seated root offset, text box units, point poses when mirrored, gesture hands, environment layer order, spots in
// the narrow aspects, text colours on dark environments, the camera's actor reach), and a spread of frames of firewall.json and
// of the cast sheet (internal/storymaker/testdata/player/castsheet.json) that must draw without an unresolved token.
import assert from 'node:assert/strict';
import { plain, realCatalog, realFiles, realAssets, renderAssets, playerStory, story, FakeCanvas } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

// The async checks of the earlier modules (the host and render driver) run against real timers and deadlines; let them finish before
// this module's long synchronous work (the real rig, a hundred rendered frames) blocks the event loop.
await Promise.all(t.pending);

const { check, near } = t;
const SM = t.core();
const catalog = realCatalog(), files = realFiles();
const entry = id => catalog.entries.find(e => e.id === id);
const firewall = story('firewall');
const codeScenes = { s3: { duration: 5 } };
const compileReal = (s, opts) => SM.compile(s, {}, catalog, { codeScenes, ...opts });
const compiled = compileReal(firewall);
const idle = { type: 'actor', pose: { a: 'stand', b: 'stand', u: 1 }, expression: 'neutral', look: null, walkTime: null, phase: 0, gesture: null, facing: 1 };
const rigStateOf = (over, time) => SM.render.rigState({ ...idle, ...over }, time || 0, SM.render.makeContext(compiled.timeline, realAssets(), 1920, 1080), files.rig);

// ---- seated root offset -------------------------------------------------------------------------------------------------

check('real catalog: a seated pose drops the root by pose.offset_y, blended with the pose blend', () => {
    assert.equal(entry('pose.typing').offset_y, 215);
    assert.equal(rigStateOf({ pose: { a: 'stand', b: 'typing', u: 1 } }).offsetY, 215);
    assert.equal(rigStateOf({ pose: { a: 'stand', b: 'typing', u: 0.5 } }).offsetY, 107.5);
    assert.equal(rigStateOf({ pose: { a: 'sit', b: 'typing', u: 0.5 } }).offsetY, 215, 'two seated poses do not add up');
    assert.equal(rigStateOf({ pose: { a: 'typing', b: 'stand', u: 0.25 } }).offsetY, 161.25);
    assert.equal(rigStateOf({ pose: { a: 'stand', b: 'stand', u: 1 } }).offsetY, 0);
});

check('rig: the drop of a pose is offset_y, else seat_drop, seated_offset, the lower leg, a fraction of the height (seated poses only)', () => {
    const drop = (rig, pose) => SM.rig.poseDrop(rig, pose);
    assert.equal(drop({ seat_drop: 215 }, { seated: true, offset_y: 180 }), 180, 'offset_y wins');
    assert.equal(drop({ seat_drop: 215 }, { offset_y: 40 }), 40, 'offset_y also works on a pose that is not seated');
    assert.equal(drop({ seat_drop: 215, seated_offset: 99 }, { seated: true }), 215);
    assert.equal(drop({ seated_offset: 99 }, { seated: true }), 99);
    assert.equal(drop({ units_height: 1000, bones: [{ name: 'lower_leg_l', length: 205 }] }, { seated: true }), 205);
    assert.equal(drop({ units_height: 1000, bones: [] }, { seated: true }), 240);
    assert.equal(drop({ seat_drop: 215 }, { bones: {} }), 0, 'a standing pose never drops');
    assert.equal(drop({ seat_drop: 215 }, null), 0);
    // the mini catalog's typing pose has seated without offset_y and the mini rig has seated_offset 92
    assert.equal(drop(t.files.rig, t.byId('pose.typing')), 92);
});

check('real catalog: the typing woman at the desk spot sits on the chair seat of the office (D pins the seat at 774 px)', () => {
    const tl = compileReal({ scenes: [{ id: 'a', environment: 'env.office', actors: [{ cast: 'anna', at: 'desk', pose: 'typing' }] }], cast: [{ id: 'anna', character: 'char.office-woman-1' }] }).timeline;
    const sc = tl.scenes[0], st = SM.scene.evalEntity(sc.entities[0], 0), stage = SM.render.stageFor(sc, SM.render.makeContext(tl, realAssets(), 1920, 1080));
    const s = SM.rig.scaleFor(files.rig, 1080, stage.depthScale(st.y, st.depth), 1, SM.rig.bodyOf(files.rig, 'average'));
    const hips = st.y + s * (-440 + 215);
    near(hips, 774, 15, 'hip joint height in design pixels');
});

// ---- text box units -----------------------------------------------------------------------------------------------------

check('text: box pad and radius are fractions of the canvas height, like the size (real caption and label styles)', () => {
    const caption = entry('text.caption'), label = entry('text.label'), title = entry('text.title');
    const lay = (style, h) => SM.text.layout(new FakeCanvas(10, 10).ctx, 'Hallo Welt', style, { canvasH: h, maxWidth: 1500, family: 'Geist' });
    assert.equal(lay(caption, 1080).pad, 13, 'round(0.012 x 1080)');
    assert.equal(lay(caption, 1080).radius, 13);
    assert.equal(lay(caption, 540).pad, 6);
    assert.equal(lay(label, 1080).pad, 11, 'round(0.010 x 1080)');
    assert.equal(lay(label, 1080).radius, 22, 'round(0.020 x 1080)');
    assert.equal(lay(title, 1080).pad, 0, 'no box, no padding');
    // a box without numbers falls back to em: 0.4 x the font size, radius 0.3 x the font size
    const bare = lay({ ...caption, box: { fill: '#000000' } }, 1000);
    assert.equal(bare.pad, Math.round(0.4 * bare.px));
    assert.equal(bare.radius, Math.round(0.3 * bare.px));
    // and the corners that are drawn use the radius
    const cv = new FakeCanvas(1920, 1080), l = lay(caption, 1080);
    SM.text.draw(cv.ctx, l, caption, 960, 900, SM.parts.makeResolver([{}]), 1);
    assert.equal(cv.ctx.log.filter(x => x.startsWith('arcTo(') && x.endsWith(',13)')).length, 4, 'four rounded corners of radius 13');
});

// ---- point poses and mirroring ------------------------------------------------------------------------------------------

// The screen side that the hand of a pose reaches: x of the hand tip relative to the root, after the mirror of a left-facing actor.
function handSide(poseName, facing) {
    const rs = rigStateOf({ pose: { a: poseName, b: poseName, u: 1 }, facing }), body = SM.rig.bodyOf(files.rig, 'average');
    const world = SM.rig.boneMatrices(files.rig, rs.view, body, rs.offsets);
    const hand = rs.hands.l === 'point' ? world.lower_arm_l : world.lower_arm_r, tip = hand[2] * 135 + hand[4];
    return Math.sign(tip) * facing;
}

check('rig state: point-left and point-right keep their screen direction when the actor faces left', () => {
    assert.equal(handSide('point-right', 1), 1);
    assert.equal(handSide('point-left', 1), -1);
    assert.equal(handSide('point-right', -1), 1, 'facing left: point-right still reaches the right edge of the screen');
    assert.equal(handSide('point-left', -1), -1);
    // the swap picks the other pose entry: its bones and its view hint
    const left = rigStateOf({ pose: { a: 'point-right', b: 'point-right', u: 1 }, facing: -1 });
    assert.equal(left.offsets.upper_arm_l, 88, 'the bones of point-left');
    assert.equal(left.hands.l, 'point');
    assert.equal(left.view, 'front');
    assert.equal(rigStateOf({ pose: { a: 'point-right', b: 'point-right', u: 1 }, facing: 1 }).view, 'three_quarter');
    // other poses are not mirrored by name
    assert.equal(rigStateOf({ pose: { a: 'wave', b: 'wave', u: 1 }, facing: -1 }).offsets.upper_arm_r, -128);
    // a catalog that has no point-left keeps the name (the mini catalog has neither)
    const mini = SM.compile({ cast: [{ id: 'a', character: 'char.office-woman-1' }], scenes: [{ id: 's', actors: [{ cast: 'a', at: 'left', pose: 'stand' }] }] }, {}, t.catalog);
    const rcMini = SM.render.makeContext(mini.timeline, renderAssets(), 1920, 1080);
    assert.equal(SM.render.rigState({ ...idle, pose: { a: 'stand', b: 'stand', u: 1 }, facing: -1 }, 0, rcMini, t.files.rig).view, 'three_quarter');
});

// ---- gesture hands ------------------------------------------------------------------------------------------------------

check('rig: a gesture hand shape holds until a later key sets another one (D: count points only at its first key)', () => {
    const count = entry('gesture.count'), celebrate = entry('gesture.celebrate');
    assert.deepEqual(plain(SM.rig.handsAt(count.keys, 0.1)), null);
    assert.deepEqual(plain(SM.rig.handsAt(count.keys, 0.5)), { r: 'point' }, 'the finger keeps pointing while the arm counts');
    assert.equal(SM.rig.handsAt(count.keys, 1), null, 'the end of the gesture returns to the pose hands');
    assert.deepEqual(plain(SM.rig.handsAt(celebrate.keys, 0.5)), { l: 'open', r: 'open' });
    assert.deepEqual(plain(SM.rig.handsAt([{ t: 0, bones: {}, hands: { l: 'fist' } }, { t: 0.5, bones: {}, hands: { r: 'point' } }], 0.7)), { l: 'fist', r: 'point' }, 'hands merge per side');
    assert.equal(rigStateOf({ gesture: { name: 'count', frac: 0.5 } }).hands.r, 'point');
    assert.equal(rigStateOf({ pose: { a: 'think', b: 'think', u: 1 } }).hands.r, 'fist', 'a pose hand shape without a gesture');
});

// ---- environments -------------------------------------------------------------------------------------------------------

check('render: environment layers draw in their listed order, parallax only moves them', () => {
    const env = {
        viewbox: [0, 0, 1920, 1080], tokens: { day: {}, night: {} },
        layers: [
            { name: 'wall', parallax: 0.1, shapes: [{ d: 'M0,0 L1,1', fill: '#111111' }] },
            { name: 'floor', parallax: 1, shapes: [{ d: 'M0,0 L2,2', fill: '#222222' }] },
            { name: 'furniture', parallax: 0.7, shapes: [{ d: 'M0,0 L3,3', fill: '#333333' }] }
        ]
    };
    const tl = SM.compile({ scenes: [{ id: 'a', environment: 'env.office', silent: true }] }, {}, catalog).timeline;
    const cv = new FakeCanvas(tl.width, tl.height);
    SM.renderFrame(tl, 3, cv.ctx, realAssets({ envs: { 'environments/office.json': env } }));
    const order = cv.ctx.log.filter(l => /^set fillStyle=#[123]{6}$/.test(l)).map(l => l.slice(-6));
    assert.deepEqual(order, ['111111', '222222', '333333']);
    // the real files list the layers back to front, and parallax is not monotonic in them
    for (const name of ['office', 'abstract-tech']) {
        const par = files.envs['environments/' + name + '.json'].layers.map(l => l.parallax);
        assert.ok(par.some((p, i) => i && p < par[i - 1]), name + ': parallax is not sorted, so the order must come from the list');
    }
});

check('stage: in 9:16 and 1:1 every spot of the real environments is inside the visible slice, and a spot without an override keeps its design position', () => {
    for (const env of catalog.entries.filter(e => e.kind === 'environment')) {
        for (const aspect of ['9:16', '1:1']) {
            const stage = SM.stage.create({ aspect, env, safe: catalog.stage.safe });
            const over = env.aspects[aspect], vis = stage.vis;
            for (const name of Object.keys(env.spots)) {
                const s = stage.spot(name);
                assert.ok(s.x >= vis.x + 0.03 * vis.w - 1e-6 && s.x <= vis.x + 0.97 * vis.w + 1e-6, env.id + ' ' + aspect + ' ' + name + ' at ' + Math.round(s.x) + ' is outside ' + Math.round(vis.x) + '..' + Math.round(vis.x + vis.w));
                const o = over.spots && over.spots[name];
                near(s.x, (o ? o.x : env.spots[name].x) / 100 * 1920, 1e-6, env.id + ' ' + aspect + ' ' + name);
                near(s.y, (o && o.y !== undefined ? o.y : env.spots[name].y) / 100 * 1080, 1e-6);
            }
            near(stage.vis.x + stage.vis.w / 2, over.focus_x / 100 * 1920, 1e-6, 'the slice is centred on focus_x');
        }
    }
    // a catalog without overrides (the mini catalog) still squeezes a spot that is outside the slice into it
    const mini = SM.stage.create({ aspect: '9:16', env: t.byId('env.office'), safe: t.catalog.stage.safe }), far = mini.spot('far-left');
    assert.ok(far.x >= mini.vis.x && far.x <= mini.vis.x + mini.vis.w);
});

// ---- text colours and the camera ----------------------------------------------------------------------------------------

check('render: text colours resolve from the environment variant first, then the palette', () => {
    const env = { viewbox: [0, 0, 1920, 1080], layers: [], tokens: { day: {}, night: { text: '#f2f4ff', bg2: '#2c356b' } } };
    const story2 = variant => ({ scenes: [{ id: 'a', environment: 'env.office', variant, beats: [
        { do: 'show', text: 'Titel', entry: 'none' }, { do: 'show', text: 'Marke', style: 'label', at: { x: 50, y: 60 }, entry: 'none', with: 'prev' }] }] });
    const fills = variant => {
        const tl = compileReal(story2(variant)).timeline, cv = new FakeCanvas(tl.width, tl.height);
        SM.renderFrame(tl, 5, cv.ctx, realAssets({ envs: { 'environments/office.json': env } }));
        return cv.ctx.log.filter(l => l.startsWith('set fillStyle=')).map(l => l.slice('set fillStyle='.length));
    };
    assert.ok(fills('night').includes('#f2f4ff'), 'the title takes the night text colour');
    assert.ok(fills('night').includes('#2c356b'), 'the label box takes the night bg2');
    assert.ok(fills('day').includes('#1f2a44') && !fills('day').includes('#f2f4ff'), 'the day variant has no text token: the palette decides');
});

// WCAG contrast of two #rrggbb colours
const luminance = hex => [1, 3, 5].map(i => parseInt(hex.slice(i, i + 2), 16) / 255).map(c => c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4).reduce((s, c, i) => s + c * [0.2126, 0.7152, 0.0722][i], 0);
const contrast = (a, b) => { const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x); return (hi + 0.05) / (lo + 0.05); };

check('real catalog: the label and title text colours are readable in every environment, variant and palette', () => {
    const label = entry('text.label');
    for (const name of ['plain-stage', 'office', 'home-office', 'abstract-tech']) {
        const env = files.envs['environments/' + name + '.json'];
        for (const variant of ['day', 'night']) {
            for (const pal of catalog.palettes) {
                const tokens = { ...pal.tokens, ...env.tokens[variant] };
                const boxFill = tokens[label.box.fill.slice(1)], labelInk = tokens[label.color.slice(1)];
                assert.ok(contrast(labelInk, boxFill) >= 4.5, name + ' ' + variant + ' ' + pal.name + ': label ' + labelInk + ' on ' + boxFill);
                // the title has no box and the palette's text colour is dark: every dark environment (night, abstract-tech) carries a light one
                const dark = variant === 'night' || name === 'abstract-tech';
                if (dark) assert.ok(env.tokens[variant].text && luminance(env.tokens[variant].text) > 0.6, name + ' ' + variant + ' has a light text token');
            }
        }
    }
});

check('camera: a close framing on an actor of the real catalog shows the head and the feet', () => {
    const out = compileReal({ cast: [{ id: 'anna', character: 'char.office-woman-1' }], scenes: [{ id: 's', environment: 'env.office', camera: { framing: 'close', target: 'anna' }, actors: [{ cast: 'anna', at: 'desk', pose: 'typing' }] }] });
    const sc = out.timeline.scenes[0], stage = SM.render.stageFor(sc, SM.render.makeContext(out.timeline, realAssets(), 1920, 1080));
    const st = SM.scene.evalEntity(sc.entities[0], 0), win = stage.cameraWindow(SM.camera.at(sc.camera, 0));
    const depth = stage.depthScale(st.y, st.depth), headTop = st.y - 0.55 * 1080 * depth;
    assert.ok(win.y <= headTop && win.y + win.h >= st.y, 'window ' + Math.round(win.y) + '..' + Math.round(win.y + win.h) + ' holds ' + Math.round(headTop) + '..' + Math.round(st.y));
    near(win.h, 1080 / 1.9, 1e-6);
});

