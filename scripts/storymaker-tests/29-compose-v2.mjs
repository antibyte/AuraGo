// Composition v2 (style.compose "v2"): the base spots above the caption band with the old depth, the hero prop at a quarter of
// the frame height, the automatic push-in, and every v1 story untouched.
import assert from 'node:assert/strict';
import { t } from '../storymaker-test-context.mjs';

const { check, near } = t;
const SM = t.core();

// film(): one scene on plain-stage with an actor at left and two props that show on cue marker 1.
function film(extraStyle, extraScene) {
    const s = {
        schema: 'storymaker/1', title: 'Compose', format: { aspect: '16:9', fps: 30 },
        style: Object.assign({ palette: 'sunny', font: 'default', captions: true }, extraStyle),
        cast: [{ id: 'anna', name: 'Anna', character: 'char.office-woman-1' }],
        scenes: [{
            id: 's1', environment: 'env.plain-stage', narration: 'Eine {1}Firewall prueft jedes Paket am Eingang.', speaker: 'anna',
            actors: [{ cast: 'anna', at: 'left', pose: 'stand' }],
            beats: [
                { do: 'show', item: 'prop.lock', at: 'center', entry: 'pop', cue: 1 },
                { do: 'show', item: 'prop.shield', at: 'right', entry: 'pop', cue: 1 }
            ]
        }]
    };
    return Object.assign(s.scenes[0], extraScene) && s;
}
const compile = story => SM.compile(story, {}, t.catalog);
const entitiesOf = out => out.timeline.scenes[0].entities;
const plainEnv = () => t.byId('env.plain-stage');

check('compose v2: the base spots lift above the caption band and keep the old depth scale', () => {
    const v1 = SM.stage.create({ aspect: '16:9', env: plainEnv() });
    const v2 = SM.stage.create({ aspect: '16:9', env: plainEnv(), compose: 'v2', captions: true });
    for (const name of ['far-left', 'left', 'center', 'right', 'far-right']) {
        const a = v1.spot(name), b = v2.spot(name);
        near(b.x, a.x, 1e-9, name + ' x');
        near(b.y, 0.74 * 1080, 1e-9, name + ' feet line');
        assert.ok(b.y < 0.794 * 1080, name + ': the feet stand above the top of the two-line caption band (79.4 percent)');
        near(v2.depthScale(b.y, b.depth), v1.depthScale(a.y, a.depth), 1e-9, name + ' depth scale');
    }
    near(v2.spot('board').y, v1.spot('board').y, 1e-9, 'the board is no base spot');
    assert.deepEqual(v2.resolve({ x: 50, y: 90 }), v1.resolve({ x: 50, y: 90 }), 'an explicit point stays where it is');
    assert.equal(v2.captionSafe, true);
});

check('compose v2: no change without captions, in 9:16, without compose, or for another value', () => {
    const base = SM.stage.create({ aspect: '16:9', env: plainEnv() });
    const cases = [
        SM.stage.create({ aspect: '16:9', env: plainEnv(), compose: 'v2', captions: false }),
        SM.stage.create({ aspect: '16:9', env: plainEnv(), compose: '', captions: true }),
        SM.stage.create({ aspect: '16:9', env: plainEnv(), compose: 'v3', captions: true })
    ];
    for (const stage of cases) near(stage.spot('left').y, base.spot('left').y, 1e-9, 'unchanged');
    const tall1 = SM.stage.create({ aspect: '9:16', env: plainEnv() });
    const tall2 = SM.stage.create({ aspect: '9:16', env: plainEnv(), compose: 'v2', captions: true });
    near(tall2.spot('left').y, tall1.spot('left').y, 1e-9, '9:16 has its caption band higher and needs no move');
    assert.equal(tall2.captionSafe, false);
    const square = SM.stage.create({ aspect: '1:1', env: plainEnv(), compose: 'v2', captions: true });
    near(square.spot('left').y, 0.74 * 1080, 1e-9, '1:1 has the same caption band as 16:9');
});

check('compose v2: a compiled story keeps x and the actor size and lifts the feet; a story without compose is unchanged', () => {
    const a = compile(film()), b = compile(film({ compose: 'v2' }));
    const actorA = entitiesOf(a).find(e => e.type === 'actor'), actorB = entitiesOf(b).find(e => e.type === 'actor');
    near(actorB.x.at(0), actorA.x.at(0), 1e-9, 'x');
    near(actorA.y.at(0), 0.88 * 1080, 1e-9, 'v1 feet');
    near(actorB.y.at(0), 0.74 * 1080, 1e-9, 'v2 feet');
    const s1 = SM.stage.create({ aspect: '16:9', env: plainEnv() });
    const s2 = SM.stage.create({ aspect: '16:9', env: plainEnv(), compose: 'v2', captions: true });
    near(s2.depthScale(actorB.y.at(0), actorB.depth), s1.depthScale(actorA.y.at(0), actorA.depth), 1e-9, 'the same size');
    assert.equal(a.timeline.style.compose, '');
    assert.equal(b.timeline.style.compose, 'v2');
});

check('compose v2: a prop at a base spot keeps the size it has in v1 (createProp stores the spot depth)', () => {
    const a = compile(film()), b = compile(film({ compose: 'v2' }));
    const propA = entitiesOf(a).find(e => e.item === 'prop.shield'), propB = entitiesOf(b).find(e => e.item === 'prop.shield');
    const s1 = SM.stage.create({ aspect: '16:9', env: plainEnv() });
    const s2 = SM.stage.create({ aspect: '16:9', env: plainEnv(), compose: 'v2', captions: true });
    near(s2.depthScale(propB.y.at(0), propB.depth), s1.depthScale(propA.y.at(0), propA.depth), 1e-9);
});
