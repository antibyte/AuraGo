// Task C2: easing, tween tracks and step tracks.
import assert from 'node:assert/strict';
import { plain } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near } = t;
const SM = t.core();

check('the six eases hit 0 and 1 and behave', () => {
    const names = ['linear', 'ease-in', 'ease-out', 'ease-in-out', 'back-out', 'bounce-out'];
    assert.deepEqual(plain(SM.easeNames), names);
    for (const name of names) {
        near(SM.ease(name, 0), 0, 1e-12, name + '(0)');
        near(SM.ease(name, 1), 1, 1e-12, name + '(1)');
    }
    near(SM.ease('linear', 0.25), 0.25, 1e-12);
    assert.ok(SM.ease('ease-in', 0.5) < 0.5 && SM.ease('ease-out', 0.5) > 0.5);
    near(SM.ease('ease-in-out', 0.5), 0.5, 1e-12);
    assert.ok(SM.ease('back-out', 0.8) > 1, 'back-out overshoots');
    assert.ok(SM.ease('bounce-out', 0.9) < 1 && SM.ease('bounce-out', 0.95) > SM.ease('bounce-out', 0.9) - 0.2);
    assert.equal(SM.ease('nope', 0.3), 0.3, 'an unknown name is linear');
    assert.equal(SM.ease('ease-out', -4), 0);
    assert.equal(SM.ease('ease-out', 9), 1);
});

check('tracks evaluate segments, overlapping segments and step tracks', () => {
    const T = SM.timeline, tr = new T.Track(10);
    tr.add(1, 3, 20, 'linear');
    tr.add(5, 5, 0, 'linear');
    tr.add(2, 4, 100, 'linear');   // starts while the first runs: from = the value of the first at t=2
    near(tr.at(0), 10, 1e-9);
    near(tr.at(1.5), 12.5, 1e-9);
    near(tr.at(2), 15, 1e-9);
    near(tr.at(3), 57.5, 1e-9);
    near(tr.at(4.5), 100, 1e-9);
    near(tr.at(5), 0, 1e-9);
    const st = new T.StepTrack('a');
    st.set(2, 'b', 1).set(1, 'c', 1);
    assert.equal(st.at(0.5), 'a');
    assert.equal(st.at(1.5), 'c');
    assert.equal(st.at(2.5), 'b');
    const blend = st.blendAt(2.25);
    assert.equal(blend.a, 'c'); assert.equal(blend.b, 'b'); near(blend.u, 0.25, 1e-9);
});
