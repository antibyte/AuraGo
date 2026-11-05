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
