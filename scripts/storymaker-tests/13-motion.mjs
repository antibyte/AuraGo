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

// ---- the scheduler: targets, floors, overlap, cursor, anchor

const lock = (id, extra = {}) => ({ do: 'show', item: 'prop.lock', id, ...extra });
const startOf = e => e.alpha.segs[0].t0;
const WALK = { do: 'walk', who: 'anna', to: 'center' };

check('scheduler: a beat on another target starts at 0.4 of the anchor, a same-target beat follows, a wait breaks overlap', () => {
    // two 0.5 s shows of one item are two instances, so two targets: the second starts at 0.4 * 0.5 = 0.2 s
    const two = one([lock('a'), lock('b')], { actors: [] });
    near(startOf(two.of('a')), 0, 1e-9, 'first');
    near(startOf(two.of('b')), 0.2, 1e-9, 'overlap');
    // same target: the emote waits for the walk
    const same = one([WALK, { do: 'emote', who: 'anna', expression: 'happy' }]);
    near(same.anna.expr.steps[0].t, same.anna.walks.list[0].t1, 1e-9, 'same target');
    // a wait has no target: it starts at the cursor (0.5 s), lasts 1 s, and the next show follows it
    const gap = one([lock('a'), { do: 'wait', seconds: 1 }, lock('b')], { actors: [] });
    near(startOf(gap.of('b')), 1.5, 1e-9, 'a wait breaks overlap');
    // an unsupported beat leaves the state untouched: the overlap still reads the first show
    const skipped = one([lock('a'), { do: 'chart', chart: 'bar', data: {} }, { do: 'show', icon: 'icon.x' }, lock('b')], { actors: [] });
    near(startOf(skipped.of('b')), 0.2, 1e-9, 'unsupported beats');
});

check('scheduler: targets compare as instances, not as strings', () => {
    // one instance under two names is one target: the second move follows the first, at 0.5 + 0.8 = 1.3 s
    const named = one([lock('a'), { do: 'move', target: 'a', to: 'left' }, { do: 'move', target: 'prop.lock', to: 'center' }], { actors: [] });
    near(named.of('a').x.segs[1].t0, 1.3, 1e-9, 'one instance, two names');
    // a camera on a spot has no instance, so no target: it starts at the cursor (0.5 s) for 0.8 s, and the next show follows
    const spot = one([lock('a'), { do: 'camera', op: 'focus', target: 'board' }, lock('b')], { actors: [] });
    near(startOf(spot.of('b')), 1.3, 1e-9, 'a spot is not a target');
});

check('scheduler: a short with-prev beat leaves the anchor on the longer beat; floors and the cursor hold', () => {
    // the walk lasts W; the lock runs with it for 0.5 s and does not replace the anchor, so the shield starts at 0.4 W
    const group = one([WALK, lock('a', { with: 'prev' }), { do: 'show', item: 'prop.shield', id: 'b' }]);
    const walk = group.anna.walks.list[0], W = walk.t1 - walk.t0;
    near(startOf(group.of('a')), 0, 1e-9, 'with prev');
    near(startOf(group.of('b')), 0.4 * W, 1e-9, 'the anchor stays on the walk');
    // floor: the emote overlaps the lock by rule c, but anna is busy until W
    const floored = one([WALK, lock('a'), { do: 'emote', who: 'anna', expression: 'happy' }]);
    near(floored.anna.expr.steps[0].t, W, 1e-9, 'floor');
    // cursor after with prev: the maximum of the group, so a wait starts at W and the next show at W + 1
    const after = one([WALK, lock('a', { with: 'prev' }), { do: 'wait', seconds: 1 }, lock('b')]);
    near(startOf(after.of('b')), W + 1, 1e-9, 'cursor');
});

check('scheduler: firewall s2: the gesture runs with the walk on the same actor, the shield overlaps, the camera follows the shield', () => {
    const s2 = t.sceneById('s2'), anna = s2.entities.find(e => e.id === 'anna');
    const walk = anna.walks.list[0], gesture = anna.gestures.list[0], shield = s2.entities.find(e => e.item === 'prop.shield');
    near(walk.t0, 0.5, 1e-9, 'T of a fade is 0.5 s');
    near(gesture.t0, 0.5, 1e-9, 'with prev has no floor');
    near(startOf(shield), 0.5 + 0.4 * (walk.t1 - walk.t0), 1e-9, 'shield');
    near(s2.camera.zoom.segs[0].t0, shield.alpha.segs[0].t1, 1e-9, 'a camera without a target starts at the cursor');
});

// ---- cue lead and cue_late

// The marker of LONG sits at 20 / 31 of 16 / 2.3 s, about 4.5 s, so no lead in the table is clamped at 0.
const LONG = 'a b c d e f g h i j {1}k l m n o p';
const cueOf = narration => SM.planTiming({ scenes: [{ id: 's', narration }] }, {}, catalog).scenes.s.cues['1'];

// trackReach(tr, level): the first scene time at which the last segment of tr has come level of its way, in 0.1 ms steps.
function trackReach(tr, level) {
    const seg = lastSeg(tr);
    for (let k = 0; ; k++) {
        const x = seg.t0 + k * 1e-4;
        if (x > seg.t1) return seg.t1;
        if ((tr.at(x) - seg.from) / (seg.to - seg.from) >= level) return x;
    }
}

const ARRIVALS = [
    ['enter', { do: 'enter', who: 'anna', to: 'center' }, o => o.anna.x, 0.9],
    ['walk', { do: 'walk', who: 'anna', to: 'center' }, o => o.anna.x, 0.9],
    ['move', { do: 'move', who: 'anna', to: 'center' }, o => o.anna.x, 0.9],
    ['show fade', { do: 'show', item: 'prop.lock', at: 'right', entry: 'fade' }, o => o.item('prop.lock').alpha, 0.9],
    ['show slide', { do: 'show', item: 'prop.lock', at: 'right', entry: 'slide' }, o => o.item('prop.lock').x, 0.9],
    // the spring lands on its first crossing of the target
    ['show pop', { do: 'show', item: 'prop.lock', at: 'right', entry: 'pop' }, o => o.item('prop.lock').scale, 1],
    ['camera focus', { do: 'camera', op: 'focus', target: 'anna' }, o => o.scene.camera.cx, 0.9],
    ['camera zoom', { do: 'camera', op: 'zoom', target: 'anna' }, o => o.scene.camera.cx, 0.9]
];

check('cue lead: an arrival lands on its marker plus delay within 1 ms', () => {
    const cue = cueOf(LONG);
    for (const [name, beat, trackOf, level] of ARRIVALS) {
        for (const delay of [0, 0.3]) {
            const o = one([{ ...beat, cue: 1, delay }], { narration: LONG });
            near(trackReach(trackOf(o), level), cue + delay, 0.001, name + ' with delay ' + delay);
            assert.ok(!o.issues.some(i => i.code === 'cue_late'), name + ' is not late');
        }
    }
});

check('cue lead: a clamp at scene time 0 does not warn, a busy target gives cue_late, a cue beats with', () => {
    // marker at 0 and a lead of 0.54 s: the beat would start before 0, so it starts at 0 without a warning
    const early = one([{ do: 'enter', who: 'anna', to: 'center', cue: 1 }], { narration: '{1}a b c d e f g h' });
    near(lastSeg(early.anna.x).t0, 0, 1e-9, 'clamp');
    assert.ok(!early.issues.some(i => i.code === 'cue_late'), 'a clamp at 0 never warns');
    // the walk holds anna until its end; the move on the same target cannot start before that
    const busy = one([WALK, { do: 'move', who: 'anna', to: 'right', cue: 1 }], { narration: 'a {1}b c d e f g h' });
    const walkEnd = busy.anna.walks.list[0].t1;
    near(lastSeg(busy.anna.x).t0, walkEnd, 1e-9, 'the beat waits for the previous beat on its target');
    const late = busy.issues.filter(i => i.code === 'cue_late');
    assert.equal(late.length, 1);
    assert.equal(late[0].severity, 'warning');
    assert.equal(late[0].path, '/scenes/0/beats/1/cue');
    assert.equal(late[0].scene_id, 's');
    // at_seconds is the landing: the start plus 0.4377 of the 0.8 s move
    near(late[0].at_seconds, walkEnd + 0.4377 * 0.8, 1e-6, 'at_seconds');
    // a show with entry none has no lead: with a cue it starts on the marker, not with the walk
    const both = one([WALK, { do: 'show', item: 'prop.lock', at: 'right', entry: 'none', with: 'prev', cue: 1 }], { narration: LONG });
    near(lastSeg(both.item('prop.lock').alpha).t0, cueOf(LONG), 1e-9, 'a cue beats with');
});

check('cue lead: an sfx has no target and impact 0: with prev it sits on the landing, with a cue on the marker plus delay', () => {
    // The show runs with the sfx, so its start shows the sfx start. The id is in no catalog, so its impact stays 0.
    // enter lasts 0.6 s and is linear, so its landing is 0.9 * 0.6 = 0.54 s
    const onLanding = one([{ do: 'enter', who: 'anna', to: 'center' }, { do: 'sfx', sound: 'sfx.absent', with: 'prev' }, { do: 'show', item: 'prop.lock', with: 'prev' }]);
    near(lastSeg(onLanding.item('prop.lock').alpha).t0, 0.54, 1e-9, 'on the landing');
    const onCue = one([{ do: 'sfx', sound: 'sfx.absent', cue: 1, delay: 0.2 }, { do: 'show', item: 'prop.lock', with: 'prev' }], { narration: LONG });
    near(lastSeg(onCue.item('prop.lock').alpha).t0, cueOf(LONG) + 0.2, 1e-9, 'on the marker');
});
