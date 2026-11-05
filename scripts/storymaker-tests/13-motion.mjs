// Tasks M6 to M10: the motion rules and the beat scheduler.
import assert from 'node:assert/strict';
import { plain } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near, catalog } = t;
const SM = t.core();

// firstReach(name, level): the first u in 0..1 where the ease reaches level, searched in steps of 0.00001.
function firstReach(name, level) {
    for (let k = 0; k <= 100000; k++) if (SM.ease(name, k / 100000) >= level) return k / 100000;
    return 1;
}

check('motion: rhythm factors, beat defaults and the overlap', () => {
    assert.deepEqual(plain(SM.motion.RHYTHM), { calm: 1.25, standard: 1, lively: 0.8 });
    assert.equal(SM.motion.factor({ rhythm: 'calm' }), 1.25);
    assert.equal(SM.motion.factor({ rhythm: 'lively' }), 0.8);
    assert.equal(SM.motion.factor({}), 1);
    assert.equal(SM.motion.factor(undefined), 1);
    assert.deepEqual(plain(SM.motion.DEFAULTS), { enter: 0.6, exit: 0.6, move: 0.8, pose: 0.3, emote: 0.25, look: 0.3, show: 0.5, hide: 0.35, camera: 0.8, shake: 0.5, wait: 1, highlight: 0.8 });
    assert.equal(SM.motion.OVERLAP, 0.4);
});

check('motion: each lead fraction matches a numeric search of its ease to 0.001', () => {
    for (const name of ['linear', 'ease-in', 'ease-out', 'ease-in-out', 'back-out', 'bounce-out', 'quart-out', 'expo-out']) {
        near(SM.motion.leadFraction(name), firstReach(name, 0.9), 0.001, name);
    }
    // spring: the first crossing of the target value 1, not the 90 % point
    near(SM.motion.leadFraction('spring'), firstReach('spring', 1), 0.001, 'spring');
    assert.equal(SM.motion.leadFraction('nope'), 0.9, 'an unknown name counts as linear');
});

check('motion: holds count words through the range tables and never shrink with rhythm', () => {
    // 4 Han characters are one CJK run: ceil(4 / 2) = 2 words, so 0.5 + 0.3 * 2 = 1.1 s
    near(SM.motion.holdSeconds(String.fromCharCode(0x6f22, 0x5b57, 0x6f22, 0x5b57)).base, 1.1, 1e-9, 'Han');
    // 4 words: 0.5 + 0.3 * 4 = 1.7 s
    near(SM.motion.holdSeconds('eins zwei drei vier').base, 1.7, 1e-9, '4 words');
    // 1 word: 0.5 + 0.3 = 0.8 s, raised to 1.2 s only for a secondary line
    near(SM.motion.holdSeconds('eins').base, 0.8, 1e-9, 'plain line');
    near(SM.motion.holdSeconds('eins', { secondary: true }).base, 1.2, 1e-9, 'secondary line');
    near(SM.motion.holdSeconds('eins zwei drei vier', { factor: 1.25 }).scaled, 1.7 * 1.25, 1e-9, 'calm');
    near(SM.motion.holdSeconds('eins zwei drei vier', { factor: 0.8 }).scaled, 1.7, 1e-9, 'lively keeps the base');
});

// ---- role eases and scaled defaults

const CAST = [{ id: 'anna', character: 'char.office-woman-1' }];

// one(beats, extra): a single storyboard scene at T = 0 (a first scene has no entry transition), compiled with the mini catalog.
function one(beats, extra = {}) {
    const scene = { id: 's', environment: 'env.office', actors: extra.actors || [{ cast: 'anna', at: 'left' }], beats };
    if (extra.narration) scene.narration = extra.narration;
    if (extra.duration) scene.duration = extra.duration;
    const story = { cast: CAST, scenes: [scene] };
    if (extra.style) story.style = extra.style;
    const out = SM.compile(story, {}, extra.catalog || catalog);
    const built = out.timeline.scenes[0];
    return {
        out, scene: built, issues: out.report.issues, end: out.report.scenes[0].end_frame,
        anna: built.entities.find(e => e.id === 'anna'), text: built.entities.find(e => e.type === 'text'),
        of: id => built.entities.find(e => e.id === id), item: id => built.entities.find(e => e.item === id)
    };
}
// lastSeg(track): the last segment of a track, with its `from` filled in.
const lastSeg = tr => { tr.at(0); return tr.segs[tr.segs.length - 1]; };

check('role eases: every default of the table', () => {
    const enter = one([{ do: 'enter', who: 'anna', to: 'center' }]).anna;
    assert.equal(lastSeg(enter.x).ease, 'linear');
    assert.equal(lastSeg(enter.y).ease, 'linear');
    const appear = lastSeg(enter.alpha);
    assert.equal(appear.ease, 'linear');
    assert.equal(appear.t1, appear.t0, 'enter: the alpha is a step at the start');
    const exit = one([{ do: 'exit', who: 'anna', to: 'door' }]).anna;
    assert.equal(lastSeg(exit.x).ease, 'linear');
    assert.equal(lastSeg(exit.y).ease, 'linear');
    const gone = lastSeg(exit.alpha);
    assert.equal(gone.ease, 'ease-in');
    near(gone.t1 - gone.t0, 0.2, 1e-9, 'exit: the alpha runs over the last 0.2 s');
    assert.equal(lastSeg(one([{ do: 'walk', who: 'anna', to: 'center' }]).anna.x).ease, 'linear');
    assert.equal(lastSeg(one([{ do: 'move', who: 'anna', to: 'center' }]).anna.x).ease, 'quart-out');
    const show = entry => one([{ do: 'show', item: 'prop.lock', at: 'right', entry }]).item('prop.lock');
    assert.equal(lastSeg(show('fade').alpha).ease, 'quart-out');
    const slide = show('slide');
    assert.equal(lastSeg(slide.alpha).ease, 'quart-out');
    assert.equal(lastSeg(slide.x).ease, 'quart-out');
    const pop = show('pop');
    assert.equal(lastSeg(pop.alpha).ease, 'quart-out');
    assert.equal(lastSeg(pop.scale).ease, 'spring');
    assert.equal(lastSeg(pop.scale).from, 0.6);
    const popText = one([{ do: 'show', text: 'Hallo', entry: 'pop' }], { actors: [] }).text;
    assert.equal(lastSeg(popText.alpha).ease, 'quart-out');
    assert.equal(popText.scale.segs.length, 0, 'a text pop has no scale entry');
    const hidden = one([{ do: 'show', item: 'prop.lock', id: 'lock' }, { do: 'hide', target: 'lock' }], { actors: [] }).of('lock');
    assert.equal(lastSeg(hidden.alpha).ease, 'ease-in');
    const camera = beat => one([beat]).scene.camera;
    const focus = camera({ do: 'camera', op: 'focus', target: 'anna' });
    for (const tr of [focus.zoom, focus.cx, focus.cy]) assert.equal(lastSeg(tr).ease, 'expo-out');
    const zoom = camera({ do: 'camera', op: 'zoom', target: 'anna' });
    for (const tr of [zoom.zoom, zoom.cx, zoom.cy]) assert.equal(lastSeg(tr).ease, 'ease-in-out');
    const pan = camera({ do: 'camera', op: 'pan', target: 'anna' });
    for (const tr of [pan.cx, pan.cy]) assert.equal(lastSeg(tr).ease, 'ease-in-out');
    // pose: the blend is eased by ease-in-out; a quarter of the 0.3 s blend gives 4 * 0.25^3 = 0.0625
    const posed = one([{ do: 'pose', who: 'anna', pose: 'typing' }]).anna;
    near(posed.pose.blendAt(0.075).u, 0.0625, 1e-9, 'pose');
    // gesture: ease-in-out per key segment, a smoothstep in the rig: 0.25^2 * (3 - 2 * 0.25) = 0.15625
    near(SM.rig.sampleKeys([{ t: 0, bones: { a: 0 } }, { t: 1, bones: { a: 1 } }], 0.25).bones.a, 0.15625, 1e-9, 'gesture');
});

check('role eases: an explicit ease wins', () => {
    const e = 'bounce-out';
    assert.equal(lastSeg(one([{ do: 'enter', who: 'anna', to: 'center', ease: e }]).anna.x).ease, e);
    assert.equal(lastSeg(one([{ do: 'exit', who: 'anna', to: 'door', ease: e }]).anna.x).ease, e);
    assert.equal(lastSeg(one([{ do: 'walk', who: 'anna', to: 'center', ease: e }]).anna.x).ease, e);
    assert.equal(lastSeg(one([{ do: 'move', who: 'anna', to: 'center', ease: e }]).anna.x).ease, e);
    const show = entry => one([{ do: 'show', item: 'prop.lock', at: 'right', entry, ease: e }]).item('prop.lock');
    assert.equal(lastSeg(show('fade').alpha).ease, e);
    assert.equal(lastSeg(show('slide').x).ease, e);
    assert.equal(lastSeg(show('pop').scale).ease, e);
    const hidden = one([{ do: 'show', item: 'prop.lock', id: 'lock' }, { do: 'hide', target: 'lock', ease: e }], { actors: [] }).of('lock');
    assert.equal(lastSeg(hidden.alpha).ease, e);
    assert.equal(lastSeg(one([{ do: 'camera', op: 'focus', target: 'anna', ease: e }]).scene.camera.zoom).ease, e);
});

check('rhythm: defaults scale, hide takes 0.35 s, explicit values and wait do not scale', () => {
    const span = seg => seg.t1 - seg.t0;
    const lively = { rhythm: 'lively' }, calm = { rhythm: 'calm' };
    const hide = style => lastSeg(one([{ do: 'show', item: 'prop.lock', id: 'lock' }, { do: 'hide', target: 'lock' }], { actors: [], style }).of('lock').alpha);
    near(span(hide()), 0.35, 1e-9, 'hide');
    near(span(hide(lively)), 0.35 * 0.8, 1e-9, 'lively hide');
    const walk = style => one([{ do: 'walk', who: 'anna', to: 'center' }], { style }).anna.walks.list[0];
    near(span(walk(lively)), span(walk()) * 0.8, 1e-9, 'lively walk');
    near(span(lastSeg(one([{ do: 'move', who: 'anna', to: 'center', duration: 2 }], { style: lively }).anna.x)), 2, 1e-9, 'an explicit duration is not scaled');
    assert.equal(one([{ do: 'wait', seconds: 2 }], { style: calm }).end, 60, '2 s at 30 fps, never times 1.25');
    assert.equal(one([{ do: 'wait' }], { style: calm }).end, 30, 'the default wait stays 1 s; scaled it would give 38 frames');
});

check('pop: opacity never passes 1, also with an explicit overshooting ease', () => {
    const maxAlpha = e => { let m = 0; for (let k = 0; k <= 200; k++) m = Math.max(m, e.alpha.at(k / 200)); return m; };
    for (const ease of [null, 'back-out']) {
        const prop = { do: 'show', item: 'prop.lock', at: 'right', entry: 'pop' }, text = { do: 'show', text: 'Hallo', entry: 'pop' };
        if (ease) { prop.ease = ease; text.ease = ease; }
        assert.ok(maxAlpha(one([prop]).item('prop.lock')) <= 1, 'prop with ' + ease);
        assert.ok(maxAlpha(one([text], { actors: [] }).text) <= 1, 'text with ' + ease);
    }
});
