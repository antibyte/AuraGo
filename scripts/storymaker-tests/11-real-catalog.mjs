// Task R: the player against the real starter catalog (ui/img/storymaker, part D). The rules where the two parts had to be
// reconciled (seated root offset, text box units, point poses when mirrored, gesture hands, environment layer order, spots in
// the narrow aspects, text colours on dark environments, the camera's actor reach), and a spread of frames of firewall.json and
// of the cast sheet (internal/storymaker/testdata/player/castsheet.json) that must draw without an unresolved token.
import assert from 'node:assert/strict';
import { plain, legacyCatalog as realCatalog, legacyFiles as realFiles, legacyAssets as realAssets, renderAssets, playerStory, story, FakeCanvas } from '../storymaker-test-support.mjs';
import { readJSON as readFixture, REAL as REAL_DIR } from '../storymaker-test-support.mjs';
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

// ---- the data of the real catalog ---------------------------------------------------------------------------------------

const castsheet = playerStory('castsheet');

check('real catalog: every character x view x expression resolves every slot to a variant or a deliberate "none"', () => {
    const rig = files.rig, R = SM.rig, expressions = catalog.entries.filter(e => e.kind === 'expression'), characters = catalog.entries.filter(e => e.kind === 'character');
    assert.equal(characters.length, 4);
    assert.equal(expressions.length, 10);
    const valueOf = (params, slot) => {
        const outfit = rig.params.outfit[params.outfit];
        switch (slot.param) {
            case 'hair_back': return params.hair;
            case 'top': case 'bottom': case 'shoes': return outfit[slot.param];
            case 'eyes': case 'brows': case 'mouth': case 'hand': return 'drawn';
            default: return params[slot.param];
        }
    };
    let resolved = 0, none = 0;
    for (const ch of characters) {
        for (const view of R.VIEWS) {
            for (const expr of expressions) {
                const state = { expr, hands: {}, mouth: null, blink: false };
                for (const slot of rig.slots) {
                    const shapes = R.pickVariant(rig, ch.params, slot, state, view), where = ch.id + ' ' + view + ' ' + expr.name + ' ' + slot.name;
                    if (shapes === null) {
                        assert.equal(valueOf(ch.params, slot), 'none', where + ' has no variant and is not a deliberate none');
                        none++;
                    } else {
                        assert.ok(Array.isArray(shapes), where);
                        resolved++;
                    }
                }
            }
        }
    }
    assert.ok(resolved > 4 * 3 * 10 * 20 && none > 0, resolved + ' resolved, ' + none + ' deliberate');
    // every variant has all three views, so no view silently falls back to another
    for (const slot of rig.slots) {
        for (const [key, v] of Object.entries(rig.variants[slot.name])) for (const view of R.VIEWS) assert.ok(Array.isArray(v[view]), slot.name + '/' + key + ' lacks ' + view);
    }
    // every hand shape a pose or gesture names, and every mouth an expression names, exists
    const handShapes = Object.keys(rig.variants.hand_l), mouths = Object.keys(rig.variants.mouth);
    for (const e of catalog.entries) {
        for (const h of Object.values(e.hands || {})) assert.ok(handShapes.includes(h), e.id + ' hand ' + h);
        for (const k of e.keys || []) for (const h of Object.values(k.hands || {})) assert.ok(handShapes.includes(h), e.id + ' key hand ' + h);
        if (e.kind === 'expression') assert.ok(mouths.includes(e.mouth), e.id + ' mouth ' + e.mouth);
    }
});

check('real catalog: firewall, minimal and the cast sheet compile with only the issues of a silent film and the lint they earn', () => {
    // firewall s2 fades into another environment, minimal shows nothing on its first frame, the cast sheet: see below
    const allowed = new Set(['narration_missing', 'not_supported_yet', 'layout_unverified', 'fade_between_worlds', 'first_frame_empty', 'line_unread', 'entry_monotony']);
    for (const [name, s] of [['firewall', firewall], ['minimal', story('minimal')], ['castsheet', castsheet]]) {
        const out = compileReal(s), report = out.report, codes = [...report.issues, ...report.unverified].map(i => i.code);
        assert.deepEqual(codes.filter(c => !allowed.has(c)), [], name + ': ' + JSON.stringify(report.issues.filter(i => !allowed.has(i.code))));
        assert.equal(report.catalog_version, catalog.version);
        assert.ok(out.needs.includes('catalog:rig/human.json') || name === 'minimal', name + ' needs the rig');
    }
    // A 4-word label holds 1.7 s in the sheet's 1.2 s scenes, and every show has entry none, so only these two codes fire.
    assert.deepEqual([...new Set(plain(compileReal(castsheet).report.issues).map(i => i.code))].sort(), ['entry_monotony', 'line_unread']);
    const ranges = compiled.report.scenes;
    assert.deepEqual(plain(ranges.map(s => s.id)), ['s1', 's2', 's3']);
    assert.ok(ranges.every((s, i) => s.end_frame > s.start_frame && (i === 0 || s.start_frame < ranges[i - 1].end_frame)), 'scenes follow each other, transitions overlap');
    assert.equal(compiled.report.frame_count, Math.max(...ranges.map(s => s.end_frame)));
});

// drawn(tl, n, width): the call log of frame n, with the logs of the two transition canvases appended
function drawn(tl, n, width) {
    const cv = new FakeCanvas(width || tl.width, width ? Math.round(width * tl.height / tl.width) : tl.height), extra = [];
    const canvas = (name, w, h) => { const c = new FakeCanvas(w, h); extra.push(c); return { canvas: c, ctx: c.ctx }; };
    SM.renderFrame(tl, n, cv.ctx, realAssets({ canvas }), width ? { width } : undefined);
    return [...cv.ctx.log, ...extra.flatMap(c => c.ctx.log)];
}
const noMagenta = (log, what) => assert.ok(!log.some(l => /#ff00ff/i.test(l)), what + ' draws an unresolved token colour');

check('real catalog: frames of firewall.json in 16:9, 9:16 and 1:1 draw without an unresolved token', () => {
    SM.parts.resetUnresolved();
    for (const aspect of ['16:9', '9:16', '1:1']) {
        const tl = compileReal({ ...firewall, format: { aspect, fps: 30 } }).timeline;
        for (const n of [0, 40, 100, 114, 125, 150, 200, 233, 300, tl.frame_count - 1]) {
            const log = drawn(tl, n);
            assert.ok(log.length > 10, aspect + ' frame ' + n + ' draws something (a code scene draws its placeholder)');
            noMagenta(log, aspect + ' frame ' + n);
        }
    }
    assert.deepEqual(plain(SM.parts.unresolvedTokens()), []);
});

check('real catalog: every scene of the cast sheet, in both palettes, draws without an unresolved token or an exception', () => {
    SM.parts.resetUnresolved();
    for (const palette of ['sunny', 'calm']) {
        const tl = compileReal({ ...castsheet, style: { ...castsheet.style, palette } }).timeline;
        assert.equal(tl.scenes.length, castsheet.scenes.length);
        for (const sc of tl.scenes) {
            const log = drawn(tl, sc.start_frame + 21);
            assert.ok(log.length > 100, sc.id + ' draws something');
            noMagenta(log, palette + ' ' + sc.id);
            // the scene's entities are all on the canvas: every actor's drawn parts exist (rig slots) and the text was laid out
            if (sc.entities.some(e => e.type === 'actor')) assert.ok(log.filter(l => l.startsWith('setTransform')).length > 25, sc.id + ' draws the actors');
        }
    }
    assert.deepEqual(plain(SM.parts.unresolvedTokens()), []);
});

check('real catalog: a prop stands on its spot (anchor bottom centre, size.h of the stage height) and a text keeps its style', () => {
    const tl = compileReal({ scenes: [{ id: 'a', environment: 'env.plain-stage', props: [{ item: 'prop.lock', at: 'right' }], beats: [{ do: 'show', text: 'Paket', style: 'caption', at: 'center', entry: 'none' }] }] }).timeline;
    const log = drawn(tl, 5), sc = tl.scenes[0], stage = SM.render.stageFor(sc, SM.render.makeContext(tl, realAssets(), 1920, 1080));
    const lock = entry('prop.lock'), vb = files.props['props/lock.json'].viewbox, depth = stage.depthScale(0.88 * 1080), k = lock.size.h * 1080 * depth / vb[3];
    const tx = 0.7 * 1920 - k * (vb[0] + 0.5 * vb[2]), ty = 0.88 * 1080 - k * (vb[1] + vb[3]);
    assert.ok(log.some(l => l.startsWith('setTransform(' + [k, 0, 0, k, tx, ty].map(v => String(Math.round(v * 1000) / 1000)).join(','))), 'the padlock is placed with its bottom centre on the spot');
    assert.ok(log.some(l => l === 'set font=500 45px "Geist", sans-serif'), 'the caption uses the weight 500 face of the default font at 0.042 of the height');
    assert.ok(log.some(l => l.startsWith('fillText(Paket,960,')), 'the caption is centred');
});

check('real catalog: a left-facing actor is mirrored as a whole about the root', () => {
    const mk = at => compileReal({ cast: [{ id: 'a', character: 'char.office-man-1' }], scenes: [{ id: 's', actors: [{ cast: 'a', at, facing: 'left' }] }] }).timeline;
    const log = drawn(mk('center'), 3), flips = log.filter(l => l.startsWith('setTransform(-'));
    assert.ok(flips.length > 20, 'the rig parts are drawn under a mirrored matrix');
});

check('real catalog: title, caption and label clear the 2.6 % floor in every aspect', () => {
    // Sizes are fractions of the canvas height: 0.07, 0.042, 0.034. The label is tightest at 0.034 x 1080 / 1080 = 3.4 %, and 9:16 raises each by 1920 / 1080.
    for (const name of ['title', 'caption', 'label']) assert.ok(entry('text.' + name).size >= 0.026, name);
    for (const aspect of ['16:9', '9:16', '1:1']) {
        const out = compileReal({ format: { aspect, fps: 30 }, scenes: [{ id: 'a', beats: [
            { do: 'show', text: 'a', entry: 'none' }, { do: 'show', text: 'b', style: 'caption', at: { x: 50, y: 80 }, entry: 'none', with: 'prev' },
            { do: 'show', text: 'c', style: 'label', at: { x: 20, y: 50 }, entry: 'none', with: 'prev' }] }] });
        assert.deepEqual(plain(out.report.issues).filter(i => i.code === 'text_too_small'), [], aspect);
    }
});

check('real catalog: speech.json needs every fallback face and compiles with zero errors', () => {
    const withGlyphs = { ...catalog, glyph_data: readFixture(REAL_DIR + 'glyphs.json') };
    const out = SM.compile(story('speech'), {}, withGlyphs);
    assert.deepEqual(plain(out.report.issues.filter(i => i.severity === 'error')), []);
    const needs = plain(out.needs);
    // geist adds no need, because its family is a catalog font family. Each other face covers a text of the fixture:
    // Greek letters, the polytonic letter U+1F00, Devanagari, Arabic, and Han with kana.
    for (const face of ['sm-greek', 'sm-greek-ext', 'sm-deva', 'sm-arab', 'sm-cjk']) assert.ok(needs.includes('font:' + face + '/400'), face + ' is not in ' + JSON.stringify(needs));
    assert.ok(needs.some(n => n.startsWith('font:default/')), 'captions and bubbles also need the base font');
});

// ---- the remaining rules of the catalog: views, walking, bodies, spots with a depth, fonts -------------------------------

check('real catalog: pose view hints, the walk cycle and its speed, body types, spot depth and the four font weights', () => {
    // pose.view hints, walking wins over everything
    assert.equal(rigStateOf({ pose: { a: 'arms-crossed', b: 'arms-crossed', u: 1 } }).view, 'front');
    assert.equal(rigStateOf({ pose: { a: 'wave', b: 'wave', u: 1 } }).view, 'three_quarter');
    assert.equal(rigStateOf({ pose: { a: 'arms-crossed', b: 'arms-crossed', u: 1 }, walkTime: 0.1 }).view, 'profile');
    // cycle keys: offset_y is positive = down (the root drops at the contact pose), the walk is periodic with its catalog period
    assert.equal(rigStateOf({ walkTime: 0 }).offsetY, 21);
    assert.equal(rigStateOf({ walkTime: 0.25 }).offsetY, 0);
    assert.equal(rigStateOf({ walkTime: 1 }).offsetY, 21, 'one period later: the same pose');
    // walk.speed (16) is percent of the design width per second at depth 1: left (30 %) to centre (50 %) at 88 % height
    const out = compileReal({ cast: [{ id: 'anna', character: 'char.office-woman-1' }], scenes: [{ id: 's', actors: [{ cast: 'anna', at: 'left' }], beats: [{ do: 'walk', who: 'anna', to: 'center' }] }] });
    const actor = out.timeline.scenes[0].entities[0], stage = SM.render.stageFor(out.timeline.scenes[0], SM.render.makeContext(out.timeline, realAssets(), 1920, 1080));
    near(actor.x.end(), 0.2 * 1920 / (0.16 * 1920 * stage.depthScale(0.88 * 1080)), 1e-6, 'seconds of the walk');
    // body types: a rest override merges per coordinate, the scale multiplies the screen scale
    const R = SM.rig, rig = files.rig, broad = R.boneMatrices(rig, 'front', R.bodyOf(rig, 'broad'), {}), child = R.boneMatrices(rig, 'front', R.bodyOf(rig, 'child'), {});
    near(broad.upper_arm_r[4], 96, 1e-9); near(broad.upper_arm_r[5], -440 - 25 - 209, 1e-9);
    near(child.hips[5], -375, 1e-9);
    near(R.scaleFor(rig, 1080, 1, 1, R.bodyOf(rig, 'elder')), 0.55 * 1080 * 0.97 / 1000, 1e-9);
    // a spot with a depth (0 far .. 1 near) overrides the floor lookup; the starter catalog has none, so build one
    const env = { ...entry('env.office'), spots: { ...entry('env.office').spots, shelf: { x: 80, y: 60, depth: 1 } } };
    const st = SM.stage.create({ aspect: '16:9', env }), shelf = st.spot('shelf');
    assert.equal(shelf.depth, 1);
    near(st.depthScale(shelf.y, shelf.depth), env.floor.scale_near, 1e-9);
    // fonts: the default font has four weights, the text styles pick 700, 500 and 600
    assert.deepEqual(catalog.fonts.map(f => f.weight), [400, 500, 600, 700]);
    const texts = compileReal({ scenes: [{ id: 'a', beats: [
        { do: 'show', text: 'a', entry: 'none' }, { do: 'show', text: 'b', style: 'caption', at: { x: 50, y: 80 }, entry: 'none', with: 'prev' },
        { do: 'show', text: 'c', style: 'label', at: { x: 20, y: 50 }, entry: 'none', with: 'prev' }] }] });
    assert.deepEqual(plain(texts.needs.filter(n => n.startsWith('font:'))), ['font:default/500', 'font:default/600', 'font:default/700']);
});
