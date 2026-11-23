'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
global.window = {};
require('./notes-index.js');
require('./notes-panels.js');
const {contextHtml} = global.window.NotesPanels;
const index = {resolve: (from, value) => /^ideen-neu/.test(value) ? 'N/ideen-neu.md' : null};

test('context lines show link text, strip light marks and never inject HTML', () => {
    const html = contextHtml('Siehe [[ideen-neu]] und [[Brandneu|Alias]] **fett** `code` *kursiv* #tag <b>x</b>', 'N/a.md', 'N/ideen-neu.md', index);
    assert.ok(html.includes('<span class="notes-context-link is-current">ideen-neu</span>'));
    assert.ok(html.includes('<span class="notes-context-link">Alias</span>'));
    assert.ok(html.includes(' fett code kursiv '));
    assert.ok(html.includes('<span class="notes-context-tag">#tag</span>'));
    assert.ok(html.includes('&lt;b&gt;x&lt;/b&gt;'));
    assert.ok(!html.includes('[['));
});
