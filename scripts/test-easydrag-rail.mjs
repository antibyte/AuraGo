#!/usr/bin/env node
// W1a D.2: every rail button has a caption key (in all 16 locales, short) and an icon-map entry.
// The buttons are found in the sources, so a new rail button without a caption fails here.
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
const { icons } = box.window.EasyDrag;

// The rail buttons the sources build: ED.icons.caption(<t>, '<id>') in editor.js and workspace.js.
const ids = [];
for (const file of ['easydrag-editor.js', 'easydrag-workspace.js']) {
    for (const match of read(file).matchAll(/ED\.icons\.caption\(\s*[A-Za-z.]+,\s*'([a-z-]+)'\s*\)/g)) ids.push(match[1]);
}
assert.deepEqual([...ids].sort(), Object.keys(icons.RAIL).sort(), 'the rail buttons in the sources are exactly the ids of ED.icons.RAIL');
assert.equal(new Set(ids).size, ids.length, 'no rail button is built twice');
assert.equal(ids.length, 7);

const LOCALES = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
for (const id of ids) {
    assert.ok(Object.prototype.hasOwnProperty.call(icons.MEANINGS, id), id + ' has no icon-map entry');
    assert.equal(icons.RAIL[id], 'easydrag.ui.rail_' + id);
    for (const lang of LOCALES) {
        const words = JSON.parse(fs.readFileSync(new URL('../../../lang/easydrag/' + lang + '.json', apps), 'utf8'));
        const text = words[icons.RAIL[id]];
        assert.equal(typeof text, 'string', lang + ' lacks ' + icons.RAIL[id]);
        assert.ok(text.trim().length > 0, lang + ' has an empty ' + icons.RAIL[id]);
        assert.ok(Array.from(text).length <= 10, lang + ' ' + icons.RAIL[id] + ' is too long for the 56 px button: ' + text);
    }
}

// caption() escapes the text and hides it from screen readers (the button keeps its aria-label).
const html = icons.caption(key => (key === 'easydrag.ui.rail_home' ? '<b>"x"</b>' : key), 'home');
assert.equal(html, '<span class="ed-rail-caption" aria-hidden="true">&lt;b&gt;&quot;x&quot;&lt;/b&gt;</span>');
console.log('PASS rail captions: ' + ids.length + ' buttons, ' + LOCALES.length + ' locales');
