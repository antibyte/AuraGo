// Task N1: the shared narration fixture. Go reads the same file in internal/storymaker/narration_test.go.
import assert from 'node:assert/strict';
import { plain, readJSON } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check } = t;
const SM = t.core();
const rows = readJSON('internal/storymaker/testdata/narration/cross_language.json');

check('narration: parse matches the shared fixture', () => {
    assert.ok(rows.length > 0, 'the fixture has rows');
    for (const row of rows) {
        const got = SM.narration.parse(row.input);
        assert.equal(got.plain, row.plain, 'plain of ' + row.input);
        assert.deepEqual(plain(got.cues), row.cues, 'cues of ' + row.input);
        assert.equal(got.length, row.chars_total, 'length of ' + row.input);
        assert.equal(got.words, row.words, 'words of ' + row.input);
        assert.equal(SM.ranges.width(got.plain), row.width, 'width of ' + row.input);
    }
});
