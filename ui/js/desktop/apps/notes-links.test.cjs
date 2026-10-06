const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const test = require('node:test');

const source = fs.readFileSync(require('node:path').join(__dirname, 'notes.js'), 'utf8');
const window = {};
vm.runInNewContext(source, { window, console, URL, URLSearchParams, AbortController, setTimeout, clearTimeout });
const resolve = window.NotesApp.resolveLinkPath;

test('Notes resolves nested links with either slash style', () => {
    assert.equal(resolve('images\\room.png', 'Documents/Notes/travel/plan.md'), 'Documents/Notes/travel/images/room.png');
    assert.equal(resolve('../shared\\map.png?size=large#preview', 'Documents/Notes/travel/plan.md'), 'Documents/Notes/shared/map.png');
});

test('Notes rejects malformed escapes and links outside its workspace', () => {
    assert.equal(resolve('%E0%A4%A', 'Documents/Notes/plan.md'), null);
    assert.equal(resolve('../../../outside.txt', 'Documents/Notes/travel/plan.md'), null);
    assert.equal(resolve('/Documents/Notes/private.md', 'Documents/Notes/plan.md'), null);
    assert.equal(resolve('https://example.test/file', 'Documents/Notes/plan.md'), null);
});
