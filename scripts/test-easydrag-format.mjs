#!/usr/bin/env node
// ED.format: plurals through Intl.PluralRules for de, cs and pl with the real locale files, the
// formatters follow the UI language, the plural table of the Go test is what Intl says.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const root = new URL('../', import.meta.url);
const store = new Map();
const win = {};
const box = vm.createContext({
    window: win, console, Intl, Date, Map, Set, Number, String, Array, Object, Math, JSON,
    localStorage: { getItem: k => (store.has(k) ? store.get(k) : null), setItem: (k, v) => { store.set(k, String(v)); }, removeItem: k => { store.delete(k); } }
});
vm.runInContext(fs.readFileSync(new URL('ui/js/desktop/apps/easydrag-format.js', root), 'utf8'), box, { filename: 'easydrag-format.js' });
const F = win.EasyDrag.format;

const LOCALES = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const words = lang => JSON.parse(fs.readFileSync(new URL('ui/lang/easydrag/' + lang + '.json', root), 'utf8'));
// The host's t: replaces {name} and {{name}} literally, returns the key for a missing key.
const hostT = lang => { const dict = words(lang); return (key, vars) => { let s = dict[key] || key; if (vars) Object.entries(vars).forEach(([a, b]) => { s = s.replaceAll('{{' + a + '}}', () => b).replaceAll('{' + a + '}', () => b); }); return s; }; };

// the UI locale
win.SYSTEM_LANG = 'de-AT';
assert.equal(F.locale(), 'de', 'the locale is the language part of SYSTEM_LANG, lower case');
win.SYSTEM_LANG = '';
assert.equal(F.locale(), 'en', 'without a language the locale is en');

// plural: 1, 2, 5 and 22 in de, cs and pl, with the real texts of group_member_count
const KEY = 'easydrag.ui.group_member_count';
const render = (lang, n) => { win.SYSTEM_LANG = lang; return F.plural(hostT(lang), KEY, n); };
assert.deepEqual([1, 2, 5, 22].map(n => render('de', n)), ['1 Schritt', '2 Schritte', '5 Schritte', '22 Schritte'], 'de');
// cs and pl through stub dictionaries here; C.3a replaces them by the real locale files
const stubT = dict => (key, vars) => { let s = dict[key] || key; if (vars) Object.entries(vars).forEach(([a, b]) => { s = s.replaceAll('{' + a + '}', () => b); }); return s; };
win.SYSTEM_LANG = 'cs';
const csT = stubT({ k: '{count} kroků', k_one: '1 krok', k_few: '{count} kroky' });
assert.deepEqual([1, 2, 5, 22].map(n => F.plural(csT, 'k', n)), ['1 krok', '2 kroky', '5 kroků', '22 kroků'], 'cs: one, few, other');
win.SYSTEM_LANG = 'pl';
const plT = stubT({ k: '{count} kroku', k_one: '1 krok', k_few: '{count} kroki', k_many: '{count} kroków' });
assert.deepEqual([1, 2, 5, 22].map(n => F.plural(plT, 'k', n)), ['1 krok', '2 kroki', '5 kroków', '22 kroki'], 'pl: one, few, many, few');
assert.deepEqual([1, 2, 5].map(n => render('en', n)), ['1 step', '2 steps', '5 steps'], 'en');
assert.equal(render('ja', 1), '1 ステップ', 'ja has one form');

// fallbacks: no category key falls back to the bare key; no key at all returns the key
win.SYSTEM_LANG = 'cs';
const tiny = (key, vars) => ({ 'k': 'bare {count}', 'k_one': 'one {count}' }[key] || key).replace('{count}', vars ? vars.count : '');
assert.equal(F.plural(tiny, 'k', 3), 'bare 3', 'cs few is missing: the bare key');
assert.equal(F.plural(tiny, 'k', 1), 'one 1', 'cs one exists');
assert.equal(F.plural(tiny, 'missing', 2), 'missing', 'an unknown key comes back as the key');
assert.equal(F.plural(tiny, 'k', 2, { count: 'many' }), 'bare many', 'vars override the count');
assert.equal(F.key('k', 1), 'k_one'); assert.equal(F.key('k', 7), 'k', 'cs 7 is other');
win.SYSTEM_LANG = 'de';
assert.equal(F.plural(hostT('de'), 'easydrag.ui.group_edges_count', 1234), '1.234 äußere Verbindungen', 'the count is formatted for the language');

// number, duration, date, relative, list follow the language
const at = lang => { win.SYSTEM_LANG = lang; };
at('de'); assert.equal(F.number(1234.5), '1.234,5'); assert.equal(F.number(2.5, { minimumFractionDigits: 1, maximumFractionDigits: 1 }), '2,5');
at('en'); assert.equal(F.number(1234.5), '1,234.5');
at('de'); assert.equal(F.number(1234.5, { locale: 'en-US' }), '1,234.5', 'opts.locale overrides the UI language');
assert.deepEqual([2500, 400, 61000, 119999, 12345].map(F.duration), ['2,5 s', '400 ms', '1:01 min', '2:00 min', '12 s'], 'de durations keep the unit style');
at('en'); assert.deepEqual([2500, 1500].map(F.duration), ['2.5 s', '1.5 s']);
const noon = new Date(Date.UTC(2026, 10, 13, 12, 0, 0));
at('de'); assert.ok(F.date(noon, 'day').includes('13.11.2026'), 'de date ' + F.date(noon, 'day'));
at('en'); assert.ok(F.date(noon, 'day').includes('Nov 13, 2026'), 'en date ' + F.date(noon, 'day'));
assert.equal(F.date('not a date', 'day'), '', 'an invalid date is empty');
assert.notEqual(F.date(noon, 'seconds'), F.date(noon, 'medium'), 'seconds adds the seconds');
const soon = new Date(Date.now() + 3 * 3600 * 1000 + 5000);
at('de'); assert.equal(F.relative(soon), 'in 3 Stunden'); at('en'); assert.equal(F.relative(soon), 'in 3 hours');
at('en'); assert.equal(F.list(['a', 'b', 'c']), 'a, b, and c'); at('de'); assert.equal(F.list(['a', 'b', 'c']), 'a, b und c');
assert.equal(F.list(['a']), 'a'); assert.equal(F.listSeparator(), ', ');

// the table of plural forms in ui/desktop_easydrag_plural_test.go is what Intl.PluralRules says
const go = fs.readFileSync(new URL('ui/desktop_easydrag_plural_test.go', root), 'utf8');
const table = go.slice(go.indexOf('easyDragPluralForms = map'));
const fromGo = {};
for (const m of table.slice(0, table.indexOf('\n}\n')).matchAll(/"([a-z]{2})":\s*\{([^}]*)\}/g)) fromGo[m[1]] = Array.from(m[2].matchAll(/"(\w+)"/g)).map(x => x[1]);
for (const lang of LOCALES) {
    const rules = new Intl.PluralRules(lang), seen = new Set();
    for (let n = 0; n <= 10000; n++) seen.add(rules.select(n));
    seen.delete('other');
    assert.deepEqual((fromGo[lang] || []).slice().sort(), Array.from(seen).sort(), 'the Go plural table for ' + lang);
}
console.log('easydrag format: ok');
