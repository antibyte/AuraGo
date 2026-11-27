// The layout run: boxes of actors, props, texts and the burned caption at the hold frame of each storyboard scene.
import assert from 'node:assert/strict';
import { plain, renderAssets } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near } = t;
const SM = t.core();

function film(style, scene) {
    const s = {
        schema: 'storymaker/1', title: 'Layout', format: { aspect: '16:9', fps: 30 },
        style: Object.assign({ palette: 'sunny', font: 'default', captions: true }, style),
        cast: [{ id: 'anna', name: 'Anna', character: 'char.office-woman-1' }],
        scenes: [Object.assign({
            id: 's1', environment: 'env.plain-stage', narration: 'Eine {1}Firewall prueft jedes Paket am Eingang.', speaker: 'anna',
            actors: [{ cast: 'anna', at: 'left', pose: 'stand' }],
            beats: [{ do: 'show', item: 'prop.lock', at: 'center', entry: 'pop', cue: 1 }, { do: 'show', item: 'prop.shield', at: 'right', entry: 'pop', cue: 1 }]
        }, scene)]
    };
    return s;
}
const measure = story => SM.layout.measure(SM.compile(story, {}, t.catalog).timeline, renderAssets());

check('layout: one block per storyboard scene with the boxes of what is on the stage at the hold frame', () => {
    const blocks = measure(film());
    assert.equal(blocks.length, 1);
    const b = blocks[0];
    assert.equal(b.id, 's1');
    assert.ok(b.hold_s > 0.5, 'the hold frame lies after the cue and the pop');
    assert.equal(b.actors.length, 1);
    assert.equal(b.props.length, 2);
    assert.equal(b.missing, 0);
    for (const box of [b.actors[0].box, b.props[0].box, b.props[1].box]) assert.ok(box[2] > 0 && box[3] > 0 && box[0] >= 0 && box[1] >= 0, 'a box inside the frame: ' + box);
    const lock = b.props.find(p => p.item === 'prop.lock'), shield = b.props.find(p => p.item === 'prop.shield');
    assert.equal(lock.hero, true, 'the first cue-bound show is the hero, also without compose');
    assert.equal(shield.hero, false);
    assert.equal(lock.scale, 1, 'v1 does not enlarge it');
    assert.ok(b.caption && b.caption[2] > 0, 'the burned caption is on screen at the hold frame');
    assert.equal(b.static, false, 'a prop appears');
});

check('layout: the feet of the v1 floor stand in the caption band, the feet of composition v2 stand above it', () => {
    const v1 = measure(film())[0], v2 = measure(film({ compose: 'v2' }))[0];
    const foot = a => a.box[1] + a.box[3];
    near(foot(v1.actors[0]), 0.88, 0.01, 'v1 feet line');
    near(foot(v2.actors[0]), 0.74, 0.02, 'v2 feet line (the push-in moves it a little)');
    assert.ok(foot(v1.actors[0]) > v1.caption[1], 'v1: the feet are below the top of the caption');
    assert.ok(foot(v2.actors[0]) < v2.caption[1], 'v2: the feet are above the top of the caption');
    const hero = v2.props.find(p => p.hero);
    near(hero.box[3], 0.24, 0.02, 'the v2 hero prop stands about a quarter of the frame high');
    assert.ok(hero.scale > 1, 'and carries the scale that makes it so');
});

check('layout: no captions, no caption box; a scene without motion is static', () => {
    assert.equal(measure(film({ captions: false }))[0].caption, null);
    const quiet = measure(film({}, { beats: [] }))[0];
    assert.equal(quiet.static, true, 'an actor that only stands there');
});

check('layout: code scenes get a block with code true', () => {
    const two = film();
    two.scenes.push({ id: 's2', kind: 'code', narration: 'Ein Code.' });
    const tl = SM.compile(two, {}, t.catalog, { codeScenes: { s2: { duration: 3 } } }).timeline;
    const blocks = SM.layout.measure(tl, renderAssets());
    assert.equal(blocks.length, 2);
    const [story, code] = blocks;
    assert.equal(story.code, false);
    assert.equal(code.id, 's2');
    assert.equal(code.code, true);
    assert.equal(code.static, false);
    assert.deepEqual(plain([code.actors, code.props, code.texts]), [[], [], []]);
    const sc = tl.scenes[1], D = (sc.end_frame - sc.start_frame) / tl.fps;
    near(code.hold_s, 0.7 * D, 0.05, 'the code hold frame is 70 % of the scene');
});

check('layout: the compile report is unchanged by the layout run (the golden stays valid)', () => {
    const out = SM.compile(film(), {}, t.catalog);
    assert.equal(out.report.layout, undefined);
    assert.equal(out.timeline.scenes[0].entities.some(e => e.type === 'prop' && e.hero), true);
});

// ---- Task C10: in 9:16 the burned caption keeps out of the platform overlays: bottom at 78 % and centred in the zone band.
// The zone is internal/storymaker/layout.go SafeZone: top 0.115, right 0.130, bottom 0.219, left 0.056.
check('layout: a two-line 9:16 caption lies inside the safe zone, centred in its band; 16:9 keeps 92 %', () => {
    const long = 'Eine Firewall prueft jedes einzelne Paket am Eingang deines Netzwerks, bevor es weiterkommt.';
    const portrait = measure({ ...film({}, { narration: long }), format: { aspect: '9:16', fps: 30 } })[0].caption;
    assert.ok(portrait, 'the caption is on screen');
    const [x, y, w, h] = portrait;
    assert.ok(x >= 0.056 - 1e-9 && x + w <= 1 - 0.130 + 1e-9, 'inside the left and right zone: ' + portrait);
    assert.ok(y >= 0.115 && y + h <= 1 - 0.219 + 1e-9, 'inside the top and bottom zone: ' + portrait);
    near(y + h, 0.78, 0.01, 'the box bottom');
    near(x + w / 2, (0.056 + 0.870) / 2, 0.002, 'centred in the zone band');
    const wide = measure(film({}, { narration: long }))[0].caption;
    near(wide[1] + wide[3], 0.92, 0.01, '16:9 bottom');
    near(wide[0] + wide[2] / 2, 0.5, 0.002, '16:9 stays centred');
});

// ---- Batch D6: block boxes, also in code scenes
const flowBeat = { do: 'show', diagram: { layout: 'flow', nodes: [{ id: 'a', label: 'Anfrage' }, { id: 'b', label: 'Server' }] }, at: 'full', entry: 'draw' };
const noActors = beats => film({}, { actors: [], beats });

check('layout: a diagram at full gives a block with its kind, layout, label size and a box inside the frame', () => {
    const b = measure(noActors([flowBeat]))[0];
    assert.equal(b.blocks.length, 1);
    const [d] = b.blocks;
    assert.deepEqual(plain([d.id, d.kind, d.layout]), ['diagram-1', 'diagram', 'flow']);
    assert.ok(d.label > 0 && d.label < 0.2, 'label ' + d.label);
    assert.ok(d.box.every(v => v >= 0 && v <= 1) && d.box[2] > 0 && d.box[3] > 0 && d.box[0] + d.box[2] <= 1 && d.box[1] + d.box[3] <= 1, 'box ' + d.box);
    assert.deepEqual(plain(measure(film())[0].blocks), [], 'a scene without blocks has an empty list');
});

check('layout: a scene whose only change is a block is not static', () => {
    assert.equal(measure(noActors([flowBeat]))[0].static, false);
    assert.equal(measure(noActors([]))[0].static, true);
});

check('layout: a code scene measures its blocks under the code camera, holds after the last one and survives a bad payload', () => {
    const stage = SM.stage.create({ aspect: '16:9', env: null });
    const head = SM.blocks.create({ id: 'headline-1', kind: 'headline', text: 'Hallo *Welt*', area: SM.blocks.area(stage, null, 'day', 'top'), t0: 2, dur: 1, entry: 'fade', hidden: true });
    head.alpha.add(2, 3, 1, 'ease-out');
    const wire = JSON.parse(JSON.stringify(SM.blocks.serialize([head], SM.camera.create(1, 960, 540))));
    const run = blocks => {
        const two = film();
        two.scenes.push({ id: 's2', kind: 'code', narration: 'Ein Code.' });
        const tl = SM.compile(two, {}, t.catalog, { codeScenes: { s2: { duration: 4, blocks } } }).timeline;
        return { code: SM.layout.measure(tl, renderAssets())[1], sc: tl.scenes[1], tl };
    };
    const ok = run(wire), D = (ok.sc.end_frame - ok.sc.start_frame) / ok.tl.fps;
    assert.equal(ok.code.code, true);
    assert.equal(ok.code.blocks.length, 1);
    assert.deepEqual(plain([ok.code.blocks[0].id, ok.code.blocks[0].kind, ok.code.blocks[0].layout]), ['headline-1', 'headline', '']);
    assert.ok(ok.code.hold_s >= 3.5 - 1e-6 && ok.code.hold_s <= D, 'hold ' + ok.code.hold_s + ' of ' + D);
    const bad = run({ ...wire, blocks: [{ ...wire.blocks[0], w: 'wide' }] });
    assert.equal(bad.code.code, true);
    assert.deepEqual(plain(bad.code.blocks), []);
    assert.deepEqual(plain(run(null).code.blocks), []);
});
