// Task M14: the static motion lint: a fixture per code, and a clean story that fires none.
import assert from 'node:assert/strict';
import { plain } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near, catalog } = t;
const SM = t.core();
const LINT = ['line_unread', 'dead_stretch', 'first_frame_empty', 'text_too_small', 'fade_between_worlds', 'entry_monotony', 'transition_variety'];
const cast = [{ id: 'anna', character: 'char.office-woman-1' }];
const actors = [{ cast: 'anna', at: 'center' }];
// the mini catalog has only the title style
const styled = { ...catalog, entries: [...catalog.entries,
    { id: 'text.caption', kind: 'text_style', name: 'caption', font: 'default', size: 0.042, weight: 500, color: '$text', align: 'center', max_lines: 2 },
    { id: 'text.tiny', kind: 'text_style', name: 'tiny', font: 'default', size: 0.02, weight: 400, color: '$text', align: 'center', max_lines: 1 }] };
const lintOf = (story, cat) => plain(SM.compile(story, {}, cat || catalog).report.issues).filter(i => LINT.includes(i.code));
const named = (list, code) => list.filter(i => i.code === code);
const one = (list, code) => {
    const found = named(list, code);
    assert.equal(found.length, 1, code + ' in ' + JSON.stringify(list));
    return found[0];
};

check('lint: line_unread fires when a text leaves before its hold, at its landing', () => {
    // 4 words hold 0.5 + 0.3 x 4 = 1.7 s and the scene lasts 1 s. A fade show lasts 0.5 s and lands at 0.4377 x 0.5 s.
    const late = one(lintOf({ cast, scenes: [{ id: 'a', duration: 1, actors, beats: [{ do: 'show', text: 'vier kurze Worte hier', entry: 'fade' }] }] }), 'line_unread');
    assert.equal(late.severity, 'warning');
    assert.equal(late.path, '/scenes/0/beats/0');
    near(late.at_seconds, 0.4377 * 0.5, 1e-3);
    // a hide 0.2 s after a 2-word text, which holds 1.1 s
    const hidden = one(lintOf({ cast, scenes: [{ id: 'a', actors, beats: [{ do: 'show', id: 't', text: 'Hallo Welt', entry: 'none' }, { do: 'hide', target: 't', delay: 0.2 }] }] }), 'line_unread');
    assert.equal(hidden.path, '/scenes/0/beats/0');
    // Beside a title, a 1-word caption holds 1.2 s instead of 0.8 s. The 1-word title needs only 0.8 s of this 1 s scene.
    const second = one(lintOf({ cast, scenes: [{ id: 'a', duration: 1, actors, beats: [{ do: 'show', text: 'Titel', entry: 'none' }, { do: 'show', text: 'Wort', style: 'caption', at: { x: 50, y: 80 }, entry: 'none', with: 'prev' }] }] }, styled), 'line_unread');
    assert.equal(second.path, '/scenes/0/beats/1');
});

check('lint: dead_stretch fires on more than 3 s without a beat, and a wait is no beat', () => {
    const list = lintOf({ cast, scenes: [{ id: 'a', actors, beats: [{ do: 'wait', seconds: 4 }] }] });
    assert.deepEqual(list.map(i => i.code), ['dead_stretch']);
    assert.equal(list[0].severity, 'warning');
    assert.equal(list[0].path, '/scenes/0');
    near(list[0].at_seconds, 0, 1e-9);
});

check('lint: first_frame_empty fires when only an off-stage actor exists at T', () => {
    // the enter beat starts at scene time 0 with its alpha on, at x = -220, outside the visible slice
    const empty = one(lintOf({ cast, scenes: [{ id: 'a', beats: [{ do: 'enter', who: 'anna', from: 'left', to: 'center' }] }] }), 'first_frame_empty');
    assert.equal(empty.severity, 'warning');
    assert.equal(empty.path, '/scenes/0');
});

check('lint: text_too_small reads the style size against the short side', () => {
    const tiny = aspect => ({ cast, format: { aspect, fps: 30 }, scenes: [{ id: 'a', actors, beats: [{ do: 'show', text: 'Klein', style: 'tiny', entry: 'none' }] }] });
    // 16:9: 0.02 x 1080 / 1080 = 0.02 is below 0.026
    const small = one(lintOf(tiny('16:9'), styled), 'text_too_small');
    assert.equal(small.severity, 'warning');
    assert.equal(small.path, '/scenes/0/beats/0/style');
    // 9:16: 0.02 x 1920 / 1080 = 0.0356 clears it
    assert.equal(named(lintOf(tiny('9:16'), styled), 'text_too_small').length, 0);
});

check('lint: fade_between_worlds reads the transition the player plays, and a variant alone is no new world', () => {
    const worlds = (transition, second) => ({ cast, scenes: [{ id: 'a', environment: 'env.office', actors }, { id: 'b', transition, actors, ...second }] });
    const other = { environment: 'env.abstract-tech' };
    const stored = one(lintOf(worlds('fade', other)), 'fade_between_worlds');
    assert.equal(stored.severity, 'warning');
    assert.equal(stored.path, '/scenes/1/transition');
    assert.equal(named(lintOf(worlds('zoom', other)), 'fade_between_worlds').length, 1, 'zoom plays as fade');
    assert.equal(named(lintOf(worlds('wipe', other)), 'fade_between_worlds').length, 0);
    assert.equal(named(lintOf(worlds('fade', { environment: 'env.office', variant: 'night' })), 'fade_between_worlds').length, 0);
});

check('lint: entry_monotony needs at least 3 show beats with one entry', () => {
    const shows = entries => ({ cast, scenes: [{ id: 'a', actors, beats: entries.map((entry, k) => ({ do: 'show', text: 'Wort ' + k, entry, at: { x: 20 + 30 * k, y: 20 }, ...(k ? { with: 'prev' } : {}) })) }] });
    const same = one(lintOf(shows(['fade', 'fade', 'fade'])), 'entry_monotony');
    assert.equal(same.severity, 'minor');
    assert.equal(same.path, '/scenes');
    assert.equal(named(lintOf(shows(['fade', 'fade', 'pop'])), 'entry_monotony').length, 0);
    assert.equal(named(lintOf(shows(['fade', 'fade'])), 'entry_monotony').length, 0);
});

check('lint: transition_variety fires above 5 kinds', () => {
    const kinds = ['cut', 'fade', 'slide-left', 'wipe', 'zoom', 'iris'];
    const film = n => ({ cast, scenes: [{ id: 's0', actors }, ...kinds.slice(0, n).map((k, i) => ({ id: 's' + (i + 1), transition: k, actors }))] });
    const many = one(lintOf(film(6)), 'transition_variety');
    assert.equal(many.severity, 'minor');
    assert.equal(many.path, '/scenes');
    assert.equal(named(lintOf(film(5)), 'transition_variety').length, 0);
});

check('lint: a clean story fires none, and a text counts as activity until its hold ends', () => {
    // Scene a has no narration. Its 13-word title holds 0.5 + 0.3 x 13 = 4.4 s, and the beats end at 0.5 s, so without the
    // hold the stretch from 0.5 s to the scene end at about 4.63 s would be longer than 3 s.
    const clean = { cast, scenes: [
        { id: 'a', environment: 'env.office', actors, beats: [
            { do: 'show', text: 'Zwoelf Worte brauchen etwas mehr als vier Sekunden und bleiben so lange stehen', entry: 'fade' },
            { do: 'show', item: 'prop.lock', at: 'right', entry: 'pop', with: 'prev' }] },
        { id: 'b', environment: 'env.abstract-tech', transition: 'wipe', narration: 'Eine Firewall prueft jedes Paket.', actors: [{ cast: 'anna', at: 'left' }], beats: [{ do: 'walk', who: 'anna', to: 'center' }] }
    ] };
    assert.deepEqual(lintOf(clean), []);
});
