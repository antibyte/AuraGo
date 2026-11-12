#!/usr/bin/env node
// A6 (S0c): "1 Schritte" is wrong. group_member_count and versions_stats get singular variants
// through pluralKey, the choice stepsText makes (one name or several). The test checks the
// selection, the real texts of all 16 locales (singular and plural, same placeholders) and that
// the two call sites use the selection.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const apps = new URL('../ui/js/desktop/apps/', import.meta.url);
const store = new Map();
const box = vm.createContext({
    window: {}, console, Intl, Date, Set, Map, navigator: { platform: 'Linux' },
    localStorage: { get length() { return store.size; }, key: i => Array.from(store.keys())[i] ?? null, getItem: k => (store.has(k) ? store.get(k) : null), setItem: (k, v) => { store.set(k, String(v)); }, removeItem: k => { store.delete(k); } },
    setTimeout: () => 1, clearTimeout: () => {}
});
vm.runInContext(fs.readFileSync(new URL('easydrag-core.js', apps), 'utf8'), box, { filename: 'easydrag-core.js' });
const core = box.window.EasyDrag.core;

assert.equal(core.pluralKey('k', 1), 'k_one', 'one is singular');
assert.equal(core.pluralKey('k', 2), 'k', 'two is plural');
assert.equal(core.pluralKey('k', 0), 'k', 'zero is plural');
assert.equal(core.pluralKey('k', 1, '_edge_one'), 'k_edge_one', 'a named variant');

const LOCALES = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const words = lang => JSON.parse(fs.readFileSync(new URL('../../../lang/easydrag/' + lang + '.json', apps), 'utf8'));
const fill = (text, vars) => text.replace(/\{([a-z_]+)\}/g, (m, name) => String(vars[name]));
const placeholders = text => (text.match(/\{[a-z_]+\}/g) || []).sort().join(' ');
const GROUP = 'easydrag.ui.group_member_count', STATS = 'easydrag.ui.versions_stats';
const EDGES = 'easydrag.ui.group_edges_count', ERRORS = 'easydrag.ui.group_errors';
// The same choice easydrag-publish.js makes inline.
const statsKey = (nodes, edges) => (nodes === 1 ? core.pluralKey(STATS, 1) : core.pluralKey(STATS, edges, '_edge_one'));

const de = words('de');
const group = n => fill(de[core.pluralKey(GROUP, n)], { count: n });
assert.equal(group(1), '1 Schritt', 'German singular of the group count');
assert.equal(group(2), '2 Schritte', 'German plural of the group count');
assert.equal(fill(de[core.pluralKey(EDGES, 1)], { count: 1 }), '1 äußere Verbindung', 'German singular of the boundary count');
assert.equal(fill(de[core.pluralKey(EDGES, 3)], { count: 3 }), '3 äußere Verbindungen', 'German plural of the boundary count');
assert.equal(fill(de[core.pluralKey(ERRORS, 1)], { count: 1 }), '1 Fehler', 'German singular of the error count');
assert.equal(fill(de[core.pluralKey(ERRORS, 2)], { count: 2 }), '2 Fehler', 'German plural of the error count');
const stats = (nodes, edges) => fill(de[statsKey(nodes, edges)], { nodes, edges });
assert.equal(stats(1, 0), '1 Schritt · 0 Verbindungen', 'one step');
assert.equal(stats(2, 1), '2 Schritte · 1 Verbindung', 'one connection');
assert.equal(stats(3, 4), '3 Schritte · 4 Verbindungen', 'plural both');
const en = words('en');
assert.equal(fill(en[core.pluralKey(GROUP, 1)], { count: 1 }), '1 step');
assert.equal(fill(en[core.pluralKey(GROUP, 5)], { count: 5 }), '5 steps');

for (const lang of LOCALES) {
    const w = words(lang);
    for (const [one, many] of [[GROUP + '_one', GROUP], [STATS + '_one', STATS], [STATS + '_edge_one', STATS], [EDGES + '_one', EDGES], [ERRORS + '_one', ERRORS]]) {
        assert.ok(typeof w[one] === 'string' && w[one].trim(), lang + ' lacks ' + one);
        assert.ok(w[one].trim() !== '', lang + ' ' + one + ' is empty');
        // The placeholders of a variant are the plural's placeholders without the count that the variant spells out.
        for (const name of placeholders(w[one]).split(' ').filter(Boolean)) assert.ok(placeholders(w[many]).includes(name), lang + ' ' + one + ' uses ' + name + ', which ' + many + ' does not');
    }
}

// The call sites go through the selection.
const read = name => fs.readFileSync(new URL(name, apps), 'utf8');
for (const key of [GROUP, EDGES, ERRORS]) assert.ok(read('easydrag-groups.js').includes("core.pluralKey('" + key + "'"), 'easydrag-groups.js selects the singular of ' + key);
assert.ok(read('easydrag-publish.js').includes("core.pluralKey('easydrag.ui.versions_stats'"), 'easydrag-publish.js selects the singular');
console.log('easydrag plurals: ok');
