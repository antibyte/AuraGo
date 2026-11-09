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

check('layout: no captions, no caption box; a scene without motion is static; code scenes are left out', () => {
    assert.equal(measure(film({ captions: false }))[0].caption, null);
    const quiet = measure(film({}, { beats: [] }))[0];
    assert.equal(quiet.static, true, 'an actor that only stands there');
    const two = film();
    two.scenes.push({ id: 's2', kind: 'code', narration: 'Ein Code.' });
    const tl = SM.compile(two, {}, t.catalog, { codeScenes: { s2: { duration: 3 } } }).timeline;
    const blocks = SM.layout.measure(tl, renderAssets());
    assert.deepEqual(plain(blocks.map(b => b.id)), ['s1']);
});

check('layout: the compile report is unchanged by the layout run (the golden stays valid)', () => {
    const out = SM.compile(film(), {}, t.catalog);
    assert.equal(out.report.layout, undefined);
    assert.equal(out.timeline.scenes[0].entities.some(e => e.type === 'prop' && e.hero), true);
});
