#!/usr/bin/env node
// W1a D.1: one meaning, one glyph. The icon map covers every rail and header icon, every glyph it names
// exists in ICONS, no glyph has two meanings, and the rail and the editor header read their icons
// through ED.icons.of (no direct core.icon call is left in editor.js or workspace.js).
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const apps = new URL('../ui/js/desktop/apps/', import.meta.url);
const read = name => fs.readFileSync(new URL(name, apps), 'utf8');
const box = vm.createContext({
    window: {}, console, Intl, Date, Set, Map, navigator: { platform: 'Linux' },
    localStorage: { length: 0, key: () => null, getItem: () => null, setItem() {}, removeItem() {} },
    setTimeout: () => 1, clearTimeout: () => {}
});
for (const file of ['easydrag-core.js', 'easydrag-icons.js']) vm.runInContext(read(file), box, { filename: file });
const { core, icons } = box.window.EasyDrag;

// Every glyph that a meaning names exists, and every meaning has its own glyph.
const entries = Object.entries(icons.MEANINGS);
assert.ok(entries.length >= 15, 'the map covers the rail and the header');
for (const [meaning, glyph] of entries) assert.ok(Object.prototype.hasOwnProperty.call(core.ICONS, glyph), meaning + ' names a glyph that ICONS does not have: ' + glyph);
const owner = new Map();
for (const [meaning, glyph] of entries) {
    assert.ok(!owner.has(glyph), 'glyph ' + glyph + ' has two meanings: ' + owner.get(glyph) + ' and ' + meaning);
    owner.set(glyph, meaning);
}
// Two glyph names must not hide one drawing either.
const drawings = new Map();
for (const [name, body] of Object.entries(core.ICONS)) {
    assert.ok(!drawings.has(body), 'ICONS draws ' + name + ' and ' + drawings.get(body) + ' alike');
    drawings.set(body, name);
}

// Start and Gruppen no longer share the grid glyph.
assert.notEqual(icons.MEANINGS.home, icons.MEANINGS.groups);
assert.equal(icons.MEANINGS.home, 'home');
assert.equal(icons.MEANINGS.groups, 'group-frame');
assert.notEqual(icons.of('home'), icons.of('groups'));

// of() draws through core.icon, passes a class on, and falls back to core's fallback glyph for an unknown meaning.
assert.equal(icons.of('more'), core.icon('dots'));
assert.ok(icons.of('run', 'x-extra').includes('class="ed-icon x-extra"'));
assert.equal(icons.of('no-such-meaning'), core.icon('tool'));

// The sources read through the map: every id is a known meaning, and the two rail files keep no direct core.icon call.
let calls = 0;
for (const file of fs.readdirSync(apps).filter(name => /^easydrag.*\.js$/.test(name))) {
    for (const match of read(file).matchAll(/ED\.icons\.of\(\s*'([^']+)'/g)) {
        calls++;
        assert.ok(Object.prototype.hasOwnProperty.call(icons.MEANINGS, match[1]), file + ': unknown icon meaning ' + match[1]);
    }
}
assert.ok(calls >= 17, 'editor.js and workspace.js call ED.icons.of (found ' + calls + ')');
for (const file of ['easydrag-editor.js', 'easydrag-workspace.js']) {
    assert.ok(!/\bcore\.icon\(/.test(read(file)), file + ' still calls core.icon directly');
}
console.log('PASS icon map: ' + entries.length + ' meanings, one glyph each, ' + calls + ' call sites');
