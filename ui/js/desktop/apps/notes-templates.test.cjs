const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const test = require('node:test');

// package.json has "type": "module": run the UMD source in a vm with a fake module.
const mod = { exports: {} };
vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'notes-templates.js'), 'utf8'), { module: mod, window: {}, Intl, Date });
const T = mod.exports;
const d = new Date(2026, 10, 23, 14, 5, 9);

test('formatDate tokens', () => {
    assert.equal(T.formatDate(d, 'YYYY-MM-DD HH:mm:ss', 'de'), '2026-11-23 14:05:09');
    assert.equal(T.formatDate(d, 'dddd, D. MMMM YYYY', 'de'), 'Montag, 23. November 2026');
    assert.equal(T.formatDate(d, 'ddd D MMM YY', 'en'), 'Mon 23 Nov 26');
    assert.equal(T.formatDate(d, '[Week of] YYYY', 'en'), 'Week of 2026');
    assert.equal(T.formatDate(new Date(2026, 0, 2, 3, 4, 5), 'M/D H:mm', 'en'), '1/2 3:04');
});
test('expand variables', () => {
    assert.equal(
        T.expand('# {{title}}\n{{date}} {{time}} {{ date:dddd }} {{unknown}}', { title: 'Plan', date: d, lang: 'de' }),
        '# Plan\n2026-11-23 14:05 Montag {{unknown}}');
    assert.equal(T.expand('{{TITLE}} {{Time:HH}}', { title: 'x', date: d }), 'x 14');
});
test('daily paths', () => {
    assert.equal(T.DAILY_DIR, 'Documents/Notes/Daily');
    assert.equal(T.TEMPLATE_DIR, 'Documents/Notes/Templates');
    assert.equal(T.dailyPath(d), 'Documents/Notes/Daily/2026-11-23.md');
    assert.equal(T.dailyDate('Documents/Notes/Daily/2026-11-23.md'), '2026-11-23');
    assert.equal(T.dailyDate('Documents/Notes/2026-11-23.md'), '');
    assert.equal(T.shiftDay('2026-03-29', 1), '2026-03-30');
    assert.equal(T.shiftDay('2026-01-01', -1), '2025-12-31');
});
test('defaultDaily', () => {
    assert.ok(T.defaultDaily(d, 'de').startsWith('# Montag, 23. November 2026'));
    assert.ok(T.defaultDaily(d, 'de').endsWith('\n\n'));
});
