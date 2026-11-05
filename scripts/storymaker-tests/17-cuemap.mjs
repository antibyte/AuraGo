// Task S2: the cue map: span and proportion placement, blocks by id, one audio event per piece.
import assert from 'node:assert/strict';
import { plain } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near, catalog } = t;
const SM = t.core();

const measured = (id, durationMs, pieces, extra) => ({ id, kind: 'narration', speaker: 'narrator', duration_ms: durationMs, spans: [], levels: [], speech_ok: true, pieces, ...extra });
const piece = (hash, at, used, chars) => ({ hash, ext: 'wav', at_ms: at, in_ms: 40, out_ms: 40 + used, chars });
const planOf = (story, audio) => {
    const fmt = SM.plan.normaliseFormat(story, {});
    return SM.plan.planScene(story, audio, SM.scene.indexCatalog(catalog), fmt, 0, []);
};

check('cue map: an offset maps proportionally over the used audio of its piece', () => {
    // chars 10 and 20, used 1000 ms and 2000 ms, pause 350 ms: piece 1 starts at 1350 ms.
    const b = SM.cuemap.prepare(measured('a:n', 3350, [piece('h0', 0, 1000, 10), piece('h1', 1350, 2000, 20)]), 30);
    assert.equal(b.placed, 'span');
    assert.deepEqual(plain(b.starts), [0, 10]);
    assert.equal(SM.cuemap.cueMap(b, 0), 0);
    assert.equal(SM.cuemap.cueMap(b, 5), 500, '5/10 of 1000 ms');
    assert.equal(SM.cuemap.cueMap(b, 10), 1350, 'a sentence start lands on the at_ms of its piece');
    assert.equal(SM.cuemap.cueMap(b, 20), 2350, '1350 + 10/20 of 2000 ms');
    assert.equal(SM.cuemap.cueMap(b, 30), 3350, 'the text end is the end of the last piece');
});

check('cue map: a chars mismatch, speech_ok false and an estimate are proportional', () => {
    const pieces = [piece('h0', 0, 1000, 10), piece('h1', 1350, 2000, 20)];
    assert.equal(SM.cuemap.prepare(measured('a:n', 3350, pieces), 31).placed, 'proportion');
    const noSpeech = SM.cuemap.prepare(measured('a:n', 3350, pieces, { speech_ok: false }), 30);
    assert.equal(noSpeech.placed, 'proportion');
    assert.equal(SM.cuemap.cueMap(noSpeech, 15), 15 / 30 * 3350);
    const est = SM.cuemap.estimated('a:n', 2, 10);
    assert.equal(est.id, 'a:n');
    assert.equal(est.placed, 'proportion');
    assert.equal(est.duration_ms, 2000);
    assert.deepEqual(plain(est.pieces), []);
    assert.equal(SM.cuemap.cueMap(est, 5), 1000);
});

const SENTENCES = {
    de: ['Anna arbeitet im Homeoffice. ', 'Plötzlich klopft ein Unbekannter an.'],
    el: ['Η Άννα δουλεύει από το σπίτι. ', 'Ξαφνικά κάποιος χτυπά την πόρτα.'],
    hi: ['अन्ना घर से काम करती है। ', 'अचानक कोई दरवाज़ा खटखटाता है।'],
    ja: ['アンナは家で働いています。', '突然、誰かがドアをノックしました。']
};

check('cue map: de, el, hi and ja place a marker at a sentence start on the at_ms of its piece', () => {
    for (const [lang, [first, second]] of Object.entries(SENTENCES)) {
        // Used 1200 ms + 350 ms pause: piece 1 starts at 1550 ms. The first scene has no entry transition, so T = 0.
        const story = { voice: { provider: 'phonon', voice: 'x', language: lang }, scenes: [{ id: 's1', narration: first + '{1}' + second }] };
        const audio = { narration: { s1: { blocks: [measured('s1:n', 3950, [piece('h0', 0, 1200, first.length), piece('h1', 1550, 2400, second.length)])] } } };
        assert.equal(planOf(story, audio).narration.block.placed, 'span', lang);
        near(plain(SM.planTiming(story, audio, catalog)).scenes.s1.cues['1'], 1.55, 1e-9, lang);
    }
});

check('plan: blocks are found by id, never by position', () => {
    const story = { scenes: [{ id: 's1', narration: 'Erster Satz hier.', beats: [{ do: 'say', who: 'anna', text: 'Hallo du.' }] }] };
    // The say block stands first in the list. 'Hallo du.' has 9 units, 'Erster Satz hier.' has 17.
    const say = measured('s1:b0', 900, [piece('h9', 0, 900, 9)], { kind: 'dialogue', beat: 0, speaker: 'anna' });
    const both = planOf(story, { narration: { s1: { blocks: [say, measured('s1:n', 2000, [piece('h0', 0, 2000, 17)])] } } }).narration;
    assert.equal(both.block.id, 's1:n');
    near(both.seconds, 2, 1e-9);
    assert.equal(both.dialogue[0].id, 's1:b0');
    assert.equal(both.dialogue[0].placed, 'span');
    const onlySay = planOf(story, { narration: { s1: { blocks: [say] } } }).narration;
    assert.equal(onlySay.measured, false, 'a say block is not the narration block');
    assert.equal(onlySay.block.placed, 'proportion');
});

check('compile: one audio event per piece, with block_id and piece, and no gain', () => {
    const story = { scenes: [{ id: 'a', narration: 'Eins. Zwei.' }, { id: 'b', transition: 'fade', narration: 'Drei. Vier.' }] };
    const audio = { narration: {
        a: { blocks: [measured('a:n', 3000, [piece('h0', 0, 1000, 6), piece('h1', 1350, 1650, 5)])] },
        b: { blocks: [measured('b:n', 3000, [piece('h2', 0, 1000, 6), piece('h3', 1350, 1650, 5)])] }
    } };
    // a lasts 3 s + 0.4 s tail = 102 frames. b starts 15 frames earlier for its fade, at frame 87 = 2.9 s, and its narration
    // starts 0.5 s later, at 3400 ms.
    assert.deepEqual(plain(SM.compile(story, audio, catalog).report.audio_events), [
        { kind: 'narration', ref: 'h0', start_ms: 0, scene_id: 'a', block_id: 'a:n', piece: 0 },
        { kind: 'narration', ref: 'h1', start_ms: 1350, scene_id: 'a', block_id: 'a:n', piece: 1 },
        { kind: 'narration', ref: 'h2', start_ms: 3400, scene_id: 'b', block_id: 'b:n', piece: 0 },
        { kind: 'narration', ref: 'h3', start_ms: 4750, scene_id: 'b', block_id: 'b:n', piece: 1 }
    ]);
});
