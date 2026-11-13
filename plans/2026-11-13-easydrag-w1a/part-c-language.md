[← Plan index](README.md) · Spec: [plans/2026-11-13-easydrag-w1a-spec.md](../2026-11-13-easydrag-w1a-spec.md)

## Part C: Language and numbers (FD-11, CS-01, S0c row 23)

**Goal.** One module, `ED.format`, owns plurals, numbers, durations, dates, relative times and lists for the UI language. Every `{count}` text gets the plural forms its language needs. Published states read "vN". A pseudo-locale makes hard-coded and clipped text visible. Spec: `plans/2026-11-13-easydrag-w1a-spec.md` §5.

**Shell and worktree.** Git Bash. Every command starts with `cd <worktree> && ...`.

```
cd C:/Users/Andi/Documents/repo/AuraGo && git worktree add .worktrees/easydrag-w1a-language -b feat/easydrag-w1a-language main
mkdir -p C:/disposable/easydrag-w1a/keys D:/tmp/go-build
```

`W` below means `C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-language`.

**Baseline (before any change).**

```
cd W && npm run test:easydrag:t0
```

Expected: the run ends without a FAIL line and prints the summary of the Node scripts and the five Go steps. Note the time (about 35-55 s). A new module needs **no** `npm run build:ui`: the only registries of an EasyDrag module are `ui/js/desktop/core/module-loader.js`, the Go list `easyDragModules` and the Node harness lists (checked with `git grep -ln easydrag-credentials-purge -- . ':!plans'`). All Node scripts use only Node built-ins, so a fresh worktree needs no `node_modules`.

**Go commands** in this part use `GOTMPDIR='D:\tmp\go-build'` and the prefix `GOTOOLCHAIN=go1.27.4` (the go1.27.3 toolchain loses its `vet.exe` now and then).

**Depends on.** Nothing. Parts A and D run in parallel and also edit `ui/lang/easydrag/*.json`, the module registries and `ui/css`. Rebase rule: on a conflict in a locale file take main's file and re-run this part's key files in order (`c-3a-plural.json` with the forms script, then `c-3a-neutral.json`, then `c-4-vn.json`); on a conflict in a registry keep both entries.

**Facts the plan rests on (checked in the code).**
- The host passes `t` to EasyDrag: `menus-and-routing.js:2069` calls `EasyDragApp.render(..., { esc, api, t, ... })`, and `easydrag.js:225` stores it as `inst.t` (`ctx.t`). It is the shared `t(k, p)` of `ui/js/shared/shared-core.js:18`. It returns the **key itself** for a missing key and replaces `{name}` and `{{name}}` with the literal value.
- The UI language is `window.SYSTEM_LANG` (set in `ui/js/shared/template-data.js:16` from the server), falling back to `document.documentElement.lang`, then `'en'`. `core.lang()` (`easydrag-core.js:238`) lower-cases it and keeps the part before `-`, so the locale is a bare code such as `de`, `no` or `zh`. `ED.format.locale()` does exactly this.
- `core.fmt` (`easydrag-core.js:242-284`) already has `Intl` formatters; 35 call sites use `core.fmt.*`. They stay, as thin wrappers.
- A language list: `ui/js/shared/shared-core.js:1530` (the UI language switcher, 16 codes), `ui/lang/**` (16 files per folder plus `ui/lang/meta.json`).
- There are 34 keys whose English value contains `{count}` (9 get plural forms and 25 are number-neutral; the new `versions_connections` makes 10 plural keys and 35 in all after C.3).

---

### Task C.1: `easydrag-format.js` with the plural, number, duration, date, relative and list formatters

**Files:**
- Create: `ui/js/desktop/apps/easydrag-format.js`
- Create: `scripts/test-easydrag-format.mjs`
- Modify: `ui/js/desktop/core/module-loader.js` (the easydrag entry, after `'/js/desktop/apps/easydrag-core.js',`, about line 285)
- Modify: `ui/desktop_easydrag_test.go:12` (the list `easyDragModules`)
- Modify: the Node harness lists: `scripts/test-easydrag.mjs:489` and `:1001`, `scripts/test-easydrag-credentials.mjs:15`, `scripts/test-easydrag-extra.mjs:9`, `scripts/test-easydrag-extra2.mjs:9`, `scripts/test-easydrag-extra3.mjs:9`, `scripts/test-easydrag-extra4.mjs:11` and `:12`, `scripts/test-easydrag-plurals.mjs:17`, `scripts/test-easydrag-authoring.mjs:9`, `scripts/test-easydrag-credentials-purge.mjs:8-14`
- Test: `scripts/test-easydrag-format.mjs`

Design: the module has no dependency on `ED.core` (it loads right after it, and `core.fmt` will call it lazily). Intl objects are cached per locale and options. `plural` asks `Intl.PluralRules`, tries `key_<category>`, then the bare key (the `other` form). A missing key is recognised the way the host's `t` reports it: it returns the key.

- [ ] **Step 1: Write the failing test** `W/scripts/test-easydrag-format.mjs`

```js
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
```

- [ ] **Step 2: Run it and see it fail for the stated reason**

```
cd W && node scripts/test-easydrag-format.mjs
```

Expected: it stops with `ENOENT ... easydrag-format.js` (the module does not exist yet).

- [ ] **Step 3: Write the module** `W/ui/js/desktop/apps/easydrag-format.js`

```js
// EasyDrag language and numbers: plural forms, numbers, durations, dates, relative times and lists
// for the UI language. The host's t(k, p) is the translation; this module only picks the key
// (plural) and formats values (Intl). No dependency on ED.core.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const cache = new Map();

    // locale is the UI language: SYSTEM_LANG (set by the server), else the page's lang, else en; the part
    // before "-", lower case ("de", "no", "zh"). It is the language EasyDrag's t translates into.
    function locale() {
        const doc = typeof document !== 'undefined' && document.documentElement ? document.documentElement.lang : '';
        return String(window.SYSTEM_LANG || doc || 'en').toLowerCase().split('-')[0];
    }

    // intl returns a cached Intl object (kind: NumberFormat, PluralRules, DateTimeFormat, RelativeTimeFormat,
    // ListFormat); a malformed locale falls back to English.
    function intl(kind, loc, opts) {
        const id = kind + '|' + loc + '|' + JSON.stringify(opts || {});
        let made = cache.get(id);
        if (!made) {
            try { made = new Intl[kind](loc, opts); } catch (err) { made = new Intl[kind]('en', opts); }
            cache.set(id, made);
        }
        return made;
    }

    // category is the plural category of count in the UI language: zero, one, two, few, many or other.
    function category(count) {
        const n = Number(count);
        return Number.isFinite(n) ? intl('PluralRules', locale()).select(n) : 'other';
    }

    // key is the key of a count text for the count: key_<category>, the bare key for "other".
    function key(base, count) {
        const cat = category(count);
        return cat === 'other' ? base : base + '_' + cat;
    }

    // number formats a number for the UI language; opts are Intl.NumberFormat options plus locale (a
    // full tag such as "de-DE") to override the language.
    function number(value, opts) {
        const o = Object.assign({}, opts || {});
        const loc = o.locale || locale();
        delete o.locale;
        const n = Number(value);
        if (!Number.isFinite(n)) return String(value == null ? '' : value);
        try { return intl('NumberFormat', loc, o).format(n); } catch (err) { return String(n); }
    }

    // plural renders a count text: the key for the language's category, else the bare key (the "other"
    // form). {count} is the formatted count; vars add or replace placeholders. A key the host does not
    // know comes back as the key (the host's t does that).
    function plural(t, base, count, vars) {
        const n = Number(count);
        const params = Object.assign({ count: Number.isFinite(n) ? number(n) : count }, vars || {});
        const specific = key(base, count);
        if (specific !== base) {
            const text = t(specific, params);
            if (text && text !== specific) return text;
        }
        return t(base, params);
    }

    // duration: "400 ms", "2,5 s" (German), "12 s", "1:01 min".
    function duration(ms) {
        const n = Number(ms) || 0;
        if (n < 1000) return number(Math.round(n)) + ' ms';
        if (n < 10000) return number(n / 1000, { minimumFractionDigits: 1, maximumFractionDigits: 1 }) + ' s';
        // Whole seconds first, then minutes: 119 999 ms reads 2:00 min, not 1:60 min.
        const total = Math.round(n / 1000);
        if (total < 60) return number(total) + ' s';
        return Math.floor(total / 60) + ':' + String(total % 60).padStart(2, '0') + ' min';
    }

    const DATE_STYLES = {
        medium: { dateStyle: 'medium', timeStyle: 'short' },
        seconds: { dateStyle: 'medium', timeStyle: 'medium' },
        day: { dateStyle: 'medium' },
        time: { timeStyle: 'short' }
    };

    function toDate(value) {
        const date = value instanceof Date ? value : new Date(value);
        return isNaN(date.getTime()) ? null : date;
    }

    // date formats a date or ISO text; style is medium (date and time, the default), seconds, day or time.
    // An invalid value gives ''.
    function date(value, style) {
        const d = toDate(value);
        if (!d) return '';
        try { return intl('DateTimeFormat', locale(), DATE_STYLES[style] || DATE_STYLES.medium).format(d); } catch (err) { return d.toISOString(); }
    }

    // relative: "in 3 hours", "vor 2 Tagen", for the nearest unit.
    function relative(value) {
        const d = toDate(value);
        if (!d) return '';
        const diff = (d.getTime() - Date.now()) / 1000;
        const abs = Math.abs(diff);
        const units = [['year', 31536000], ['month', 2592000], ['week', 604800], ['day', 86400], ['hour', 3600], ['minute', 60], ['second', 1]];
        for (const [unit, seconds] of units) {
            if (abs >= seconds || unit === 'second') {
                try { return intl('RelativeTimeFormat', locale(), { numeric: 'auto' }).format(Math.round(diff / seconds), unit); } catch (err) { return date(d, 'medium'); }
            }
        }
        return '';
    }

    // list joins items with the language's "and" ("a, b und c"); opts.type 'disjunction' gives "or".
    function list(items, opts) {
        const values = Array.from(items || []);
        try { return intl('ListFormat', locale(), { style: 'long', type: (opts && opts.type) || 'conjunction' }).format(values); } catch (err) { return values.join(', '); }
    }

    // listSeparator is the language's own separator between list items (", ", "、").
    function listSeparator() {
        try {
            const literal = intl('ListFormat', locale(), { style: 'long', type: 'conjunction' }).formatToParts(['a', 'b', 'c']).find(p => p.type === 'literal');
            return literal ? literal.value : ', ';
        } catch (err) { return ', '; }
    }

    ED.format = { locale, category, key, number, plural, duration, date, relative, list, listSeparator };
})();
```

- [ ] **Step 4: Register the module in the four places.** Each is one edit.

`W/ui/js/desktop/core/module-loader.js`, old `                '/js/desktop/apps/easydrag-core.js',` new:

```
                '/js/desktop/apps/easydrag-core.js',
                '/js/desktop/apps/easydrag-format.js',
```

`W/ui/desktop_easydrag_test.go:12`, old `"easydrag-core.js", "easydrag-durable.js",` new `"easydrag-core.js", "easydrag-format.js", "easydrag-durable.js",`.

Node harness lists: in each file below, replace `'easydrag-core.js', ` by `'easydrag-core.js', 'easydrag-format.js', ` on the named lines (`scripts/test-easydrag.mjs:489`, `scripts/test-easydrag-credentials.mjs:15`, `scripts/test-easydrag-extra.mjs:9`, `-extra2.mjs:9`, `-extra3.mjs:9`, `-extra4.mjs:11` and `:12`). Then:

- `scripts/test-easydrag.mjs:1001`: after the line `vm.runInContext(fs.readFileSync(path.join(apps, 'easydrag-core.js'), 'utf8'), box, { filename: 'easydrag-core.js' });` add `    vm.runInContext(fs.readFileSync(path.join(apps, 'easydrag-format.js'), 'utf8'), box, { filename: 'easydrag-format.js' });` (same indentation).
- `scripts/test-easydrag-plurals.mjs:17`: after the core line add `vm.runInContext(fs.readFileSync(new URL('easydrag-format.js', apps), 'utf8'), box, { filename: 'easydrag-format.js' });` (this file is deleted in C.3b; until then it keeps passing).
- `scripts/test-easydrag-authoring.mjs:9`: `['easydrag-template.js', 'easydrag-model.js', 'easydrag-authoring.js']` becomes `['easydrag-template.js', 'easydrag-format.js', 'easydrag-model.js', 'easydrag-authoring.js']`.
- `scripts/test-easydrag-credentials-purge.mjs`: in `load()`, replace the two lines
  ```
      const sandbox = vm.createContext({ window: { EasyDrag: { core: { esc, tr } } } });
      vm.runInContext(fs.readFileSync(source, 'utf8'), sandbox, { filename: source });
  ```
  by
  ```
      const sandbox = vm.createContext({ window: { EasyDrag: { core: { esc, tr } } }, Intl, Date, Map });
      const format = path.join(here, '..', 'ui', 'js', 'desktop', 'apps', 'easydrag-format.js');
      vm.runInContext(fs.readFileSync(format, 'utf8'), sandbox, { filename: format });
      vm.runInContext(fs.readFileSync(source, 'utf8'), sandbox, { filename: source });
  ```

Check there is no other loader: `cd W && git grep -n "easydrag-core.js" -- scripts` must list only the files above plus `test-action-policy-status-ui.mjs`, `test-desktop-icon-resolver.mjs` (they only read the source text) and `test-easydrag-extra5.mjs:624` (reads the source for `FLOW_KEYS`).

- [ ] **Step 5: Add the plural table so the Node test can compare it.** The Go test comes in C.3a; the Node test reads `ui/desktop_easydrag_plural_test.go` already. Create it now with only the table (C.3a adds the tests to this file):

`W/ui/desktop_easydrag_plural_test.go`

```go
package ui

// easyDragPluralForms lists, per locale, the plural categories besides "other" that
// Intl.PluralRules gives for the whole numbers 0 to 10000. scripts/test-easydrag-format.mjs
// compares this table with Intl. A count text needs a key_<category> text for each entry;
// the bare key is the "other" form.
var easyDragPluralForms = map[string][]string{
	"cs": {"one", "few"},
	"da": {"one"},
	"de": {"one"},
	"el": {"one"},
	"en": {"one"},
	"es": {"one"},
	"fr": {"one"},
	"hi": {"one"},
	"it": {"one"},
	"ja": {},
	"nl": {"one"},
	"no": {"one"},
	"pl": {"one", "few", "many"},
	"pt": {"one"},
	"sv": {"one"},
	"zh": {},
}
```

- [ ] **Step 6: Run the test and the gates**

```
cd W && node scripts/test-easydrag-format.mjs
```

Expected: `easydrag format: ok`. Also `cd W && node scripts/test-easydrag-render-baseline.mjs` (expected `PASS render baseline (unchanged)`) and `cd W && node scripts/test-easydrag.mjs` (expected last line `all EasyDrag checks passed`).

- [ ] **Step 7: Go gates**

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag(AssetsLoadInDependencyOrder|UIKeys)' -count=1
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui
cd W && gofmt -l ui/desktop_easydrag_plural_test.go ui/desktop_easydrag_test.go
```

Expected: `ok  aurago/ui`, vet silent, gofmt prints nothing.

- [ ] **Step 8: Full T0 and commit**

```
cd W && npm run test:easydrag:t0
```

Expected: no FAIL line. Commit message file `C:/disposable/easydrag-w1a/c1-msg.txt` (Write tool):

```
feat(easydrag): ED.format, plurals and formatters for the UI language

New easydrag-format.js: plural() through Intl.PluralRules (key_<category>,
bare key as the other form), number, duration, date, relative, list.
duration keeps today's unit style (400 ms, 2,5 s). Registered in the
module loader, the Go module list and the Node harness lists.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
```

```
cd W && git add ui/js/desktop/apps/easydrag-format.js ui/js/desktop/core/module-loader.js ui/desktop_easydrag_test.go ui/desktop_easydrag_plural_test.go scripts/test-easydrag-format.mjs scripts/test-easydrag.mjs scripts/test-easydrag-credentials.mjs scripts/test-easydrag-extra.mjs scripts/test-easydrag-extra2.mjs scripts/test-easydrag-extra3.mjs scripts/test-easydrag-extra4.mjs scripts/test-easydrag-plurals.mjs scripts/test-easydrag-authoring.mjs scripts/test-easydrag-credentials-purge.mjs && git commit -F C:/disposable/easydrag-w1a/c1-msg.txt
```

---

### Task C.2: Move the formatter call sites onto `ED.format`

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-core.js:142-146` (`stepsText`) and `:238-284` (`lang`, `fmt`)
- Modify: `ui/js/desktop/apps/easydrag-template.js:407` (one existing line; the file is 911 lines and gets no new line)
- Modify: `ui/js/desktop/apps/easydrag-credentials-purge.js:60`
- Modify: `ui/js/desktop/apps/easydrag-fields.js:497-509`
- Modify: `ui/js/desktop/apps/easydrag-authoring.js:11-15`
- Test: `scripts/test-easydrag.mjs`, `scripts/test-easydrag-authoring.mjs`, `scripts/test-easydrag-credentials-purge.mjs`, `scripts/test-easydrag-ux-batch5.mjs`

`pluralKey` and the count-key call sites move in C.3b (they need the plural forms first).

- [ ] **Step 1: IMPACT, then re-run the survey grep**

```
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "stepsText" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "fmt" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
cd W && grep -rn "new Intl\.\|toLocaleString\|toLocaleDateString\|toLocaleTimeString\|Intl\.supportedValuesOf" ui/js/desktop/apps/easydrag-*.js
```

Warn on HIGH or CRITICAL. `fmt` is an object property, so the walk reports UNKNOWN: confirm with `grep -rn "core\.fmt\.\|fmt\." ui/js/desktop/apps/easydrag-*.js | wc -l` (expected 35 `core.fmt.` call sites; all keep working because `fmt` keeps every method). Expected survey hits before this task: `easydrag-authoring.js:13`, `easydrag-core.js:244,248,263,273`, `easydrag-credentials-purge.js:60`, `easydrag-fields.js:502`, `easydrag-template.js:407`, `easydrag-transform.js:179`, plus comments. The hits that stay after this task are listed in Step 9.

- [ ] **Step 2: Failing checks first.** Add to the end of `scripts/test-easydrag-format.mjs` (before the final `console.log`):

```js
// core.fmt and stepsText are thin wrappers: load core after format and compare
const coreBox = vm.createContext({ window: { SYSTEM_LANG: 'de' }, console, Intl, Date, Set, Map, navigator: { platform: 'Linux' }, localStorage: { getItem: () => null, setItem() {}, removeItem() {}, get length() { return 0; }, key: () => null }, setTimeout: () => 1, clearTimeout() {} });
for (const file of ['easydrag-format.js', 'easydrag-core.js']) vm.runInContext(fs.readFileSync(new URL('ui/js/desktop/apps/' + file, root), 'utf8'), coreBox, { filename: file });
const core = coreBox.window.EasyDrag.core, CF = coreBox.window.EasyDrag.format;
assert.equal(core.lang(), 'de');
assert.deepEqual([core.fmt.duration(2500), core.fmt.decimal(2.5), core.fmt.bytes(1536), core.fmt.number(1234.5)], ['2,5 s', '2,5', '1,5 KB', '1.234,5']);
assert.equal(core.fmt.dateTime('2026-11-13T12:00:00Z'), CF.date('2026-11-13T12:00:00Z', 'medium'));
assert.equal(core.fmt.dateTime('2026-11-13T12:00:00Z', true), CF.date('2026-11-13T12:00:00Z', 'seconds'));
const steps = (key, vars) => (key + ':' + Object.values(vars || {}).join(','));
assert.equal(core.stepsText(steps, 'one', 'many', ['A']), 'one:A');
assert.equal(core.stepsText(steps, 'one', 'many', ['A', 'B', 'C']), 'many:3');
```

Run `cd W && node scripts/test-easydrag-format.mjs`. Expected: it passes already for `lang`, `fmt` and `stepsText` (the old code computes the same values), which is fine: these lines pin the behaviour that the rewrite must keep. It must pass before and after Step 3-4.

- [ ] **Step 3: `core.js`: replace `lang` and `fmt`.** Save as `C:/disposable/easydrag-w1a/c2-core.mjs` and run it from the worktree:

```js
import fs from 'node:fs';
const file = 'ui/js/desktop/apps/easydrag-core.js';
const text = fs.readFileSync(file, 'utf8');
const start = text.indexOf('    function lang() {');
const endMarker = '    };\n\n    const storage = {';
const end = text.indexOf(endMarker, start);
if (start < 0 || end < 0 || text.indexOf('    function lang() {', start + 1) >= 0) throw new Error('anchors not found exactly once');
const next = `    // lang and fmt keep the names the older call sites use; the language, numbers, dates and durations
    // are ED.format's (easydrag-format.js, loaded right after this file).
    function lang() { return ED.format.locale(); }

    const fmt = {
        number(value) { return ED.format.number(value); },
        // decimal formats a number with one decimal in the desktop language ("2,5" in German).
        decimal(value) { return ED.format.number(value, { minimumFractionDigits: 1, maximumFractionDigits: 1 }); },
        duration(ms) { return ED.format.duration(ms); },
        // dateTime formats a date and time; seconds adds them (to tell runs of one minute apart).
        dateTime(iso, seconds) { return ED.format.date(iso, seconds ? 'seconds' : 'medium'); },
        relative(iso) { return ED.format.relative(iso); },
        bytes(n) {
            const value = Number(n) || 0;
            if (value < 1024) return value + ' B';
            if (value < 1048576) return fmt.decimal(value / 1024) + ' KB';
            return fmt.decimal(value / 1048576) + ' MB';
        }
`;
fs.writeFileSync(file, text.slice(0, start) + next + text.slice(end));
console.log('core.js fmt replaced');
```

```
cd W && node C:/disposable/easydrag-w1a/c2-core.mjs && sed -n 236,262p ui/js/desktop/apps/easydrag-core.js
```

Expected: `core.js fmt replaced`, and the listing ends with the `bytes` method, `    };` and `const storage = {`.

- [ ] **Step 4: `core.js`: `stepsText`.** Edit, old:

```
    // stepsText tells what an action did to steps: the name for one, the count for several. The two
    // texts are separate keys because languages count differently (keyOne takes {name}, keyMany {count}).
    function stepsText(t, keyOne, keyMany, names) {
        return names.length === 1 ? t(keyOne, { name: names[0] }) : t(keyMany, { count: names.length });
    }
```

new:

```
    // stepsText tells what an action did to steps: the name for one, the count for several. keyOne takes
    // {name}; keyMany is a count text with {count} and the plural forms of ED.format.plural.
    function stepsText(t, keyOne, keyMany, names) {
        return names.length === 1 ? t(keyOne, { name: names[0] }) : ED.format.plural(t, keyMany, names.length);
    }
```

- [ ] **Step 5: the four other formatter sites.** Exact edits.

`easydrag-template.js:407`, old (one line):
```
        new Intl.DateTimeFormat('en-GB', options).formatToParts(date).forEach(p => { parts[p.type] = p.value; });
```
new (same line, a comment appended; **the locale stays en-GB on purpose**: this is the template engine's `date` filter. The server evaluates the same expressions, so the field order and the Latin digits must not depend on the UI language):
```
        new Intl.DateTimeFormat('en-GB', options).formatToParts(date).forEach(p => { parts[p.type] = p.value; }); // en-GB on purpose: fixed field order and Latin digits, this is the template filter and not UI text
```

`easydrag-credentials-purge.js:60`, old:
```
    function dateText(value) { const d = new Date(value); return Number.isFinite(d.getTime()) ? d.toLocaleDateString() : ''; }
```
new:
```
    function dateText(value) { return ED.format.date(value, 'day'); }
```

`easydrag-fields.js:497-509`, old:
```
    // namesText lists names in the page's language: up to three as a list with "and" (Intl.ListFormat),
    // more as three names and "and N more". The three are joined with the language's own list
    // separator (", ", "、"); without Intl.ListFormat with ", ".
    function namesText(t, names) {
        let list = null;
        try { list = new Intl.ListFormat(core().lang(), { style: 'long', type: 'conjunction' }); } catch (err) { list = null; }
        if (names.length <= 3) return list ? list.format(names) : names.join(', ');
        let sep = ', ';
        if (list) {
            const literal = list.formatToParts(['a', 'b', 'c']).find(p => p.type === 'literal');
            if (literal) sep = literal.value;
        }
        return t('easydrag.ui.names_more', { names: names.slice(0, 3).join(sep), count: names.length - 3 });
    }
```
new:
```
    // namesText lists names in the page's language: up to three as a list with "and" (ED.format.list),
    // more as three names and "and N more". The three are joined with the language's own list
    // separator (", ", "、").
    function namesText(t, names) {
        if (names.length <= 3) return ED.format.list(names);
        return t('easydrag.ui.names_more', { names: names.slice(0, 3).join(ED.format.listSeparator()), count: names.length - 3 });
    }
```

`easydrag-authoring.js:11-15`, old:
```
        let nf;
        try { nf = new Intl.NumberFormat(locale || undefined); } catch (err) { nf = new Intl.NumberFormat(); }
        return { count, over: count > GOAL_MAX, text: t('easydrag.ui.goal_counter', { count: nf.format(count), max: nf.format(GOAL_MAX) }) };
```
new:
```
        const opts = locale ? { locale } : undefined;
        return { count, over: count > GOAL_MAX, text: t('easydrag.ui.goal_counter', { count: ED.format.number(count, opts), max: ED.format.number(GOAL_MAX, opts) }) };
```

- [ ] **Step 6: Run the suites**

```
cd W && node scripts/test-easydrag-format.mjs && node scripts/test-easydrag-authoring.mjs && node scripts/test-easydrag-credentials-purge.mjs && node scripts/test-easydrag.mjs
```

Expected: `easydrag format: ok`, the authoring and purge scripts end without error (the authoring script checks `goalCounter(..., 'de-DE')` gives `4.000`), and the last line `all EasyDrag checks passed` (it runs the `b5 i18n 2,5 s and 1,5 KB` check of `test-easydrag-ux-batch5.mjs`, which pins `fmt.duration(400)` to `400 ms`). Then `node scripts/test-easydrag-render-baseline.mjs` (expected `PASS render baseline (unchanged)`).

- [ ] **Step 7: Proof grep.**

```
cd W && grep -rn "new Intl\.\|toLocaleString\|toLocaleDateString\|toLocaleTimeString" ui/js/desktop/apps/easydrag-*.js | grep -v "easydrag-format.js"
```

Expected: exactly one line, `easydrag-template.js:407` (the en-GB filter). `easydrag-transform.js:179` uses `Intl.supportedValuesOf('timeZone')` (a list of zones, no formatting), the `toLocaleLowerCase()` calls in `easydrag-home*.js` and `easydrag-run-drawer.js` fold case for searching and stay.

- [ ] **Step 8: Line ratchet and gates.**

```
cd W && wc -l ui/js/desktop/apps/easydrag-template.js ui/js/desktop/apps/easydrag-core.js
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1
```

Expected: template.js `911`, core.js smaller than 561; `ok  aurago/ui`.

- [ ] **Step 9: Commit.** Message `C:/disposable/easydrag-w1a/c2-msg.txt`:

```
refactor(easydrag): core.fmt, stepsText and four formatter sites use ED.format

core.fmt and core.lang delegate to ED.format; stepsText asks ED.format.plural.
credentials-purge shows its snapshot date in the UI language instead of the
browser's, fields.namesText and authoring.goalCounter use ED.format.list and
number. The en-GB date filter of the template engine stays on purpose and says so.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
```

```
cd W && git add ui/js/desktop/apps/easydrag-core.js ui/js/desktop/apps/easydrag-template.js ui/js/desktop/apps/easydrag-credentials-purge.js ui/js/desktop/apps/easydrag-fields.js ui/js/desktop/apps/easydrag-authoring.js scripts/test-easydrag-format.mjs && git commit -F C:/disposable/easydrag-w1a/c2-msg.txt
```

---

### Task C.3a: Plural forms in the 16 locales and the forms test

**Files:**
- Create: `C:/disposable/easydrag-w1a/locale-apply-forms.mjs` (the forms script, not in the repo)
- Create: `C:/disposable/easydrag-w1a/keys/c-3a-plural.json`, `C:/disposable/easydrag-w1a/keys/c-3a-neutral.json`
- Modify: `ui/lang/easydrag/*.json` (16 files, through the scripts)
- Modify: `ui/desktop_easydrag_plural_test.go` (the tests; the table exists since C.1)
- Modify: `scripts/test-easydrag-format.mjs` (real-file assertions for cs and pl)

**Which keys get forms.** The 34 `{count}` keys of `en.json` split in two:
- **Plural keys (10 after this task):** the count belongs to a noun, adjective or verb that agrees with it: `folder_flows`, `group_member_count`, `group_edges_count`, `group_errors`, `group_running`, `group_success`, `group_disabled`, `cred_purge_versions`, `cred_purge_pinned`, and the new `versions_connections`. They get the forms of `easyDragPluralForms` in every locale. `folder_flows`, `cred_purge_versions` and `cred_purge_pinned` are S0c row 23.
- **Number-neutral keys (25):** the count stands after a colon or in brackets, or the text has no word that agrees with it, in every locale (`copied`, `diff_*`, `output_*`, `preview_*`, `summary_items`, `tree_more`, `publish_*`, `pack_blocks`, `extraction_selected`, `credential_move_text`, `goal_counter`, `names_more`, and the five `announce_*_many` keys, which `stepsText` uses only from 2 and whose cs and pl texts already spell the number after a colon). Two of them had agreeing words in some languages (`home_bulk_selected`: es, fr, it, pt, sv; `batch_result_summary`: el, es, fr, it, pt, sv) and are reworded to the colon form in `c-3a-neutral.json`.

`versions_stats`, `versions_stats_one` and `versions_stats_edge_one` (two counts in one text, wrong in cs and pl) are deleted. The versions list composes `group_member_count` and the new `versions_connections` instead (C.3b).

Forms per locale are `Intl.PluralRules` over the whole numbers 0 to 10000: cs one, few; pl one, few, many; da, de, el, en, es, fr, hi, it, nl, no, pt, sv one; ja and zh none. (`many` of es, fr, it and pt appears only from 1,000,000; the bare key serves it. The count 1.5 does not occur: every count here is a whole number.) For cs the bare key is the `other` form (0, 5 and up); for pl it is written as the `many` text, so the fallback is sound.

The helper `locale-apply.mjs` needs a text for **every** locale for each key, so it cannot add `key_few` to cs and pl only. This part therefore adds its own script `locale-apply-forms.mjs` (same canonical-file rule, idempotent). A key file maps a bare key to `{ locale: { other, one?, few?, many? } }`. The script checks every locale against Intl and fails on a missing or an unneeded form.

- [ ] **Step 1: Write the forms script** `C:/disposable/easydrag-w1a/locale-apply-forms.mjs`

```js
#!/usr/bin/env node
// Plural forms for EasyDrag W1a, Part C. Run with the worktree as the current directory:
//   node C:/disposable/easydrag-w1a/locale-apply-forms.mjs <keyfile.json> [...]
// Key file: "_delete": [keys], "<bare key>": { "_after": "<existing key>" (only for a new key),
//   "<locale>": { "other": text, "one": text, "few": text, "many": text } }.
// "other" is the bare key; the other categories are <key>_<category>. Each locale must give "other" and
// exactly the categories Intl.PluralRules gives for the whole numbers 0..10000. Existing keys are
// overwritten in place; new ones go right after the bare key (or _after). Files stay canonical JSON.
import fs from 'node:fs';

const LOCALES = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const ORDER = ['other', 'one', 'few', 'many'];
const DIR = 'ui/lang/easydrag/';

function needed(lang) {
    const rules = new Intl.PluralRules(lang), seen = new Set();
    for (let n = 0; n <= 10000; n++) seen.add(rules.select(n));
    seen.delete('other');
    return seen;
}

function apply(specPath) {
    const spec = JSON.parse(fs.readFileSync(specPath, 'utf8'));
    const del = new Set(spec._delete || []);
    const keys = Object.entries(spec).filter(([k]) => !k.startsWith('_'));
    for (const [key, value] of keys) for (const lang of LOCALES) {
        const forms = value[lang];
        if (!forms || typeof forms.other !== 'string' || !forms.other.trim()) throw new Error(specPath + ': ' + key + ' has no "other" text for ' + lang);
        const want = needed(lang);
        for (const cat of want) if (typeof forms[cat] !== 'string' || !forms[cat].trim()) throw new Error(specPath + ': ' + key + ' ' + lang + ' lacks ' + cat);
        for (const cat of Object.keys(forms)) if (cat !== 'other' && !want.has(cat)) throw new Error(specPath + ': ' + key + ' ' + lang + ' must not have ' + cat);
    }
    for (const lang of LOCALES) {
        const file = DIR + lang + '.json';
        const raw = fs.readFileSync(file, 'utf8');
        if (JSON.stringify(JSON.parse(raw), null, 2) + '\n' !== raw) throw new Error(file + ' is not in canonical form');
        const entries = Object.entries(JSON.parse(raw)).filter(([k]) => !del.has(k));
        for (const [key, value] of keys) {
            const forms = value[lang];
            let last = entries.findIndex(([k]) => k === key);
            if (last < 0) {
                const anchor = entries.findIndex(([k]) => k === value._after);
                if (anchor < 0) throw new Error(file + ': ' + key + ' is new and its _after ' + value._after + ' does not exist');
                last = anchor;
            }
            for (const cat of ORDER) {
                if (typeof forms[cat] !== 'string') continue;
                const name = cat === 'other' ? key : key + '_' + cat;
                const at = entries.findIndex(([k]) => k === name);
                if (at >= 0) { entries[at][1] = forms[cat]; last = at; continue; }
                entries.splice(last + 1, 0, [name, forms[cat]]);
                last++;
            }
        }
        fs.writeFileSync(file, JSON.stringify(Object.fromEntries(entries), null, 2) + '\n');
    }
    console.log(specPath + ': ' + keys.length + ' count keys, ' + del.size + ' deleted, in ' + LOCALES.length + ' files of ' + DIR);
}

const files = process.argv.slice(2);
if (!files.length) { console.error('usage: locale-apply-forms.mjs <keyfile.json> [...]'); process.exit(2); }
for (const f of files) apply(f);
```

- [ ] **Step 2: Write the failing Go test** `W/ui/desktop_easydrag_plural_test.go`. Keep the table from C.1 and replace the file by:

```go
package ui

import (
	"regexp"
	"strings"
	"testing"
)

// easyDragPluralForms lists, per locale, the plural categories besides "other" that
// Intl.PluralRules gives for the whole numbers 0 to 10000. scripts/test-easydrag-format.mjs
// compares this table with Intl. A count text needs a key_<category> text for each entry;
// the bare key is the "other" form.
var easyDragPluralForms = map[string][]string{
	"cs": {"one", "few"},
	"da": {"one"},
	"de": {"one"},
	"el": {"one"},
	"en": {"one"},
	"es": {"one"},
	"fr": {"one"},
	"hi": {"one"},
	"it": {"one"},
	"ja": {},
	"nl": {"one"},
	"no": {"one"},
	"pl": {"one", "few", "many"},
	"pt": {"one"},
	"sv": {"one"},
	"zh": {},
}

// easyDragPluralKeys are the count texts whose words agree with the count (noun, adjective, verb).
// They are read only through ED.format.plural and have the forms of every locale.
var easyDragPluralKeys = []string{
	"folder_flows", "group_member_count", "group_edges_count", "group_errors", "group_running",
	"group_success", "group_disabled", "cred_purge_versions", "cred_purge_pinned", "versions_connections",
}

// easyDragNumberNeutralKeys are count texts with no word that agrees with the count in any locale: the
// number follows a colon or sits in brackets or in a pair. announce_*_many are used from 2 only.
var easyDragNumberNeutralKeys = []string{
	"announce_added_many", "announce_deleted_many", "announce_disabled_many", "announce_enabled_many", "announce_moved_many",
	"copied", "diff_added", "diff_changed", "diff_removed", "names_more", "output_items", "output_tokens",
	"preview_list", "preview_object", "preview_text", "publish_blocked", "publish_warnings", "summary_items",
	"tree_more", "goal_counter", "batch_result_summary", "home_bulk_selected", "extraction_selected",
	"pack_blocks", "credential_move_text",
}

var (
	easyDragPluralSuffix = regexp.MustCompile(`_(zero|one|two|few|many)$`)
	easyDragPlaceholder  = regexp.MustCompile(`\{[a-z_]+\}`)
)

func TestEasyDragCountKeysHaveThePluralFormsOfEveryLocale(t *testing.T) {
	const prefix = "easydrag.ui."
	plural, listed := map[string]bool{}, map[string]bool{}
	for _, k := range easyDragPluralKeys {
		plural[prefix+k], listed[prefix+k] = true, true
	}
	for _, k := range easyDragNumberNeutralKeys {
		if plural[prefix+k] {
			t.Errorf("%s is listed as plural and as number-neutral", k)
		}
		listed[prefix+k] = true
	}
	en := readEasyDragLang(t, "en")
	for key, text := range en {
		if !strings.Contains(text, "{count}") {
			continue
		}
		if base := easyDragPluralSuffix.ReplaceAllString(key, ""); base != key && plural[base] {
			continue // a plural form of a listed key
		}
		if !listed[key] {
			t.Errorf("%s has {count} but is in neither easyDragPluralKeys nor easyDragNumberNeutralKeys", key)
		}
	}
	for lang, forms := range easyDragPluralForms {
		words := readEasyDragLang(t, lang)
		for _, k := range easyDragPluralKeys {
			key := prefix + k
			bare := words[key]
			if !strings.Contains(bare, "{count}") {
				t.Errorf("%s %s: the bare (other) form is missing or has no {count}", lang, key)
			}
			for _, cat := range forms {
				form := strings.TrimSpace(words[key+"_"+cat])
				if form == "" {
					t.Errorf("%s needs %s_%s", lang, key, cat)
					continue
				}
				for _, p := range easyDragPlaceholder.FindAllString(form, -1) {
					if !strings.Contains(bare, p) {
						t.Errorf("%s %s_%s uses %s, which the bare form does not", lang, key, cat, p)
					}
				}
			}
		}
		for _, k := range easyDragNumberNeutralKeys {
			if !strings.Contains(words[prefix+k], "{count}") {
				t.Errorf("%s %s lost its {count}", lang, prefix+k)
			}
		}
	}
	if len(easyDragPluralForms) != 16 {
		t.Errorf("easyDragPluralForms covers %d locales, want 16", len(easyDragPluralForms))
	}
}
```

- [ ] **Step 3: Run it and see it fail for the stated reason**

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydragCountKeys' -count=1
```

Expected: FAIL. Messages such as `en needs easydrag.ui.easydrag.ui.folder_flows_one` are **not** expected; the key is built once, so expect `cs needs easydrag.ui.folder_flows_one`, `pl needs easydrag.ui.group_member_count_few` and `... versions_connections: the bare (other) form is missing`, and `easydrag.ui.versions_connections` listed with `{count}` missing in every locale. The plural keys are absent today.

- [ ] **Step 4: Write the key files.** `C:/disposable/easydrag-w1a/keys/c-3a-plural.json` (for `locale-apply-forms.mjs`; real texts for all 16 locales):

```json
{
  "easydrag.ui.folder_flows": {
    "cs": {"other":"{count} flow","one":"{count} flow","few":"{count} flow"},
    "da": {"other":"{count} flows","one":"{count} flow"},
    "de": {"other":"{count} Flows","one":"{count} Flow"},
    "el": {"other":"{count} ροές","one":"{count} ροή"},
    "en": {"other":"{count} flows","one":"{count} flow"},
    "es": {"other":"{count} flujos","one":"{count} flujo"},
    "fr": {"other":"{count} flux","one":"{count} flux"},
    "hi": {"other":"{count} फ़्लो","one":"{count} फ़्लो"},
    "it": {"other":"{count} flussi","one":"{count} flusso"},
    "ja": {"other":"{count}件のフロー"},
    "nl": {"other":"{count} flows","one":"{count} flow"},
    "no": {"other":"{count} flyter","one":"{count} flyt"},
    "pl": {"other":"{count} przepływów","one":"{count} przepływ","few":"{count} przepływy","many":"{count} przepływów"},
    "pt": {"other":"{count} fluxos","one":"{count} fluxo"},
    "sv": {"other":"{count} flöden","one":"{count} flöde"},
    "zh": {"other":"{count} 个流程"}
  },
  "easydrag.ui.group_member_count": {
    "cs": {"other":"{count} kroků","one":"1 krok","few":"{count} kroky"},
    "da": {"other":"{count} trin","one":"1 trin"},
    "de": {"other":"{count} Schritte","one":"1 Schritt"},
    "el": {"other":"{count} βήματα","one":"1 βήμα"},
    "en": {"other":"{count} steps","one":"1 step"},
    "es": {"other":"{count} pasos","one":"1 paso"},
    "fr": {"other":"{count} étapes","one":"1 étape"},
    "hi": {"other":"{count} चरण","one":"1 चरण"},
    "it": {"other":"{count} passaggi","one":"1 passaggio"},
    "ja": {"other":"{count} ステップ"},
    "nl": {"other":"{count} stappen","one":"1 stap"},
    "no": {"other":"{count} trinn","one":"1 trinn"},
    "pl": {"other":"{count} kroków","one":"1 krok","few":"{count} kroki","many":"{count} kroków"},
    "pt": {"other":"{count} passos","one":"1 passo"},
    "sv": {"other":"{count} steg","one":"1 steg"},
    "zh": {"other":"{count} 个步骤"}
  },
  "easydrag.ui.group_edges_count": {
    "cs": {"other":"{count} vnějších propojení","one":"1 vnější propojení","few":"{count} vnější propojení"},
    "da": {"other":"{count} forbindelser på tværs","one":"1 forbindelse på tværs"},
    "de": {"other":"{count} äußere Verbindungen","one":"1 äußere Verbindung"},
    "el": {"other":"{count} εξωτερικές συνδέσεις","one":"1 εξωτερική σύνδεση"},
    "en": {"other":"{count} boundary connections","one":"1 boundary connection"},
    "es": {"other":"{count} conexiones externas","one":"1 conexión externa"},
    "fr": {"other":"{count} connexions externes","one":"1 connexion externe"},
    "hi": {"other":"{count} बाहरी कनेक्शन","one":"1 बाहरी कनेक्शन"},
    "it": {"other":"{count} collegamenti esterni","one":"1 collegamento esterno"},
    "ja": {"other":"外部接続 {count} 件"},
    "nl": {"other":"{count} externe verbindingen","one":"1 externe verbinding"},
    "no": {"other":"{count} forbindelser ut av gruppen","one":"1 forbindelse ut av gruppen"},
    "pl": {"other":"{count} połączeń zewnętrznych","one":"1 połączenie zewnętrzne","few":"{count} połączenia zewnętrzne","many":"{count} połączeń zewnętrznych"},
    "pt": {"other":"{count} ligações externas","one":"1 ligação externa"},
    "sv": {"other":"{count} externa anslutningar","one":"1 extern anslutning"},
    "zh": {"other":"{count} 条外部连接"}
  },
  "easydrag.ui.group_errors": {
    "cs": {"other":"{count} chyb","one":"1 chyba","few":"{count} chyby"},
    "da": {"other":"{count} fejl","one":"1 fejl"},
    "de": {"other":"{count} Fehler","one":"1 Fehler"},
    "el": {"other":"{count} σφάλματα","one":"1 σφάλμα"},
    "en": {"other":"{count} errors","one":"1 error"},
    "es": {"other":"{count} errores","one":"1 error"},
    "fr": {"other":"{count} erreurs","one":"1 erreur"},
    "hi": {"other":"{count} त्रुटियाँ","one":"1 त्रुटि"},
    "it": {"other":"{count} errori","one":"1 errore"},
    "ja": {"other":"エラー {count} 件"},
    "nl": {"other":"{count} fouten","one":"1 fout"},
    "no": {"other":"{count} feil","one":"1 feil"},
    "pl": {"other":"{count} błędów","one":"1 błąd","few":"{count} błędy","many":"{count} błędów"},
    "pt": {"other":"{count} erros","one":"1 erro"},
    "sv": {"other":"{count} fel","one":"1 fel"},
    "zh": {"other":"{count} 个错误"}
  },
  "easydrag.ui.group_running": {
    "cs": {"other":"{count} běží","one":"{count} běží","few":"{count} běží"},
    "da": {"other":"{count} kører","one":"{count} kører"},
    "de": {"other":"{count} laufen","one":"{count} läuft"},
    "el": {"other":"{count} εκτελούνται","one":"{count} εκτελείται"},
    "en": {"other":"{count} running","one":"{count} running"},
    "es": {"other":"{count} en ejecución","one":"{count} en ejecución"},
    "fr": {"other":"{count} en cours","one":"{count} en cours"},
    "hi": {"other":"{count} चल रहे हैं","one":"{count} चल रहा है"},
    "it": {"other":"{count} in esecuzione","one":"{count} in esecuzione"},
    "ja": {"other":"実行中 {count} 件"},
    "nl": {"other":"{count} actief","one":"{count} actief"},
    "no": {"other":"{count} kjører","one":"{count} kjører"},
    "pl": {"other":"{count} w toku","one":"{count} w toku","few":"{count} w toku","many":"{count} w toku"},
    "pt": {"other":"{count} em execução","one":"{count} em execução"},
    "sv": {"other":"{count} körs","one":"{count} körs"},
    "zh": {"other":"{count} 个运行中"}
  },
  "easydrag.ui.group_success": {
    "cs": {"other":"{count} úspěšných","one":"{count} úspěšný","few":"{count} úspěšné"},
    "da": {"other":"{count} gennemført","one":"{count} gennemført"},
    "de": {"other":"{count} erfolgreich","one":"{count} erfolgreich"},
    "el": {"other":"{count} επιτυχή","one":"{count} επιτυχές"},
    "en": {"other":"{count} succeeded","one":"{count} succeeded"},
    "es": {"other":"{count} correctos","one":"{count} correcto"},
    "fr": {"other":"{count} réussies","one":"{count} réussie"},
    "hi": {"other":"{count} सफल","one":"{count} सफल"},
    "it": {"other":"{count} riusciti","one":"{count} riuscito"},
    "ja": {"other":"成功 {count} 件"},
    "nl": {"other":"{count} geslaagd","one":"{count} geslaagd"},
    "no": {"other":"{count} fullført","one":"{count} fullført"},
    "pl": {"other":"{count} ukończonych","one":"{count} ukończony","few":"{count} ukończone","many":"{count} ukończonych"},
    "pt": {"other":"{count} concluídos","one":"{count} concluído"},
    "sv": {"other":"{count} klara","one":"{count} klar"},
    "zh": {"other":"{count} 个成功"}
  },
  "easydrag.ui.group_disabled": {
    "cs": {"other":"{count} vypnutých","one":"{count} vypnutý","few":"{count} vypnuté"},
    "da": {"other":"{count} deaktiveret","one":"{count} deaktiveret"},
    "de": {"other":"{count} ausgeschaltet","one":"{count} ausgeschaltet"},
    "el": {"other":"{count} απενεργοποιημένα","one":"{count} απενεργοποιημένο"},
    "en": {"other":"{count} disabled","one":"{count} disabled"},
    "es": {"other":"{count} desactivados","one":"{count} desactivado"},
    "fr": {"other":"{count} désactivées","one":"{count} désactivée"},
    "hi": {"other":"{count} बंद","one":"{count} बंद"},
    "it": {"other":"{count} disattivati","one":"{count} disattivato"},
    "ja": {"other":"無効 {count} 件"},
    "nl": {"other":"{count} uitgeschakeld","one":"{count} uitgeschakeld"},
    "no": {"other":"{count} deaktivert","one":"{count} deaktivert"},
    "pl": {"other":"{count} wyłączonych","one":"{count} wyłączony","few":"{count} wyłączone","many":"{count} wyłączonych"},
    "pt": {"other":"{count} desativados","one":"{count} desativado"},
    "sv": {"other":"{count} avstängda","one":"{count} avstängd"},
    "zh": {"other":"{count} 个已禁用"}
  },
  "easydrag.ui.cred_purge_versions": {
    "cs": {"other":"Bude smazáno {count} starých verzí.","one":"Bude smazána {count} stará verze.","few":"Budou smazány {count} staré verze."},
    "da": {"other":"{count} gamle versioner slettes.","one":"{count} gammel version slettes."},
    "de": {"other":"{count} alte Versionen werden gelöscht.","one":"{count} alte Version wird gelöscht."},
    "el": {"other":"Θα διαγραφούν {count} παλιές εκδόσεις.","one":"Θα διαγραφεί {count} παλιά έκδοση."},
    "en": {"other":"{count} old versions will be deleted.","one":"{count} old version will be deleted."},
    "es": {"other":"Se eliminarán {count} versiones antiguas.","one":"Se eliminará {count} versión antigua."},
    "fr": {"other":"{count} anciennes versions seront supprimées.","one":"{count} ancienne version sera supprimée."},
    "hi": {"other":"{count} पुराने संस्करण हटाए जाएँगे।","one":"{count} पुराना संस्करण हटाया जाएगा।"},
    "it": {"other":"{count} vecchie versioni verranno eliminate.","one":"{count} vecchia versione verrà eliminata."},
    "ja": {"other":"古いバージョン {count} 件が削除されます。"},
    "nl": {"other":"{count} oude versies worden verwijderd.","one":"{count} oude versie wordt verwijderd."},
    "no": {"other":"{count} gamle versjoner slettes.","one":"{count} gammel versjon slettes."},
    "pl": {"other":"Zostanie usuniętych {count} starych wersji.","one":"Zostanie usunięta {count} stara wersja.","few":"Zostaną usunięte {count} stare wersje.","many":"Zostanie usuniętych {count} starych wersji."},
    "pt": {"other":"{count} versões antigas serão eliminadas.","one":"{count} versão antiga será eliminada."},
    "sv": {"other":"{count} gamla versioner raderas.","one":"{count} gammal version raderas."},
    "zh": {"other":"将删除 {count} 个旧版本。"}
  },
  "easydrag.ui.cred_purge_pinned": {
    "cs": {"other":"{count} verzí stále používají běžící flow. Smažou se hned po skončení jejich běhu.","one":"{count} verzi stále používá běžící flow. Smaže se hned po skončení jejího běhu.","few":"{count} verze stále používají běžící flow. Smažou se hned po skončení jejich běhu."},
    "da": {"other":"{count} versioner bruges stadig af kørende flows. De slettes, så snart deres kørsel slutter.","one":"{count} version bruges stadig af kørende flows. Den slettes, så snart dens kørsel slutter."},
    "de": {"other":"{count} Versionen werden noch von laufenden Flows benutzt. Sie werden gelöscht, sobald ihr Lauf endet.","one":"{count} Version wird noch von laufenden Flows benutzt. Sie wird gelöscht, sobald ihr Lauf endet."},
    "el": {"other":"{count} εκδόσεις χρησιμοποιούνται ακόμη από flows που εκτελούνται. Διαγράφονται μόλις τελειώσει η εκτέλεσή τους.","one":"{count} έκδοση χρησιμοποιείται ακόμη από flows που εκτελούνται. Διαγράφεται μόλις τελειώσει η εκτέλεσή της."},
    "en": {"other":"{count} versions are still used by running flows. They are deleted as soon as their run ends.","one":"{count} version is still used by running flows. It is deleted as soon as its run ends."},
    "es": {"other":"{count} versiones aún las usan flujos en ejecución. Se eliminan en cuanto termina su ejecución.","one":"{count} versión aún la usan flujos en ejecución. Se elimina en cuanto termina su ejecución."},
    "fr": {"other":"{count} versions sont encore utilisées par des flows en cours. Elles sont supprimées dès la fin de leur exécution.","one":"{count} version est encore utilisée par des flows en cours. Elle est supprimée dès la fin de son exécution."},
    "hi": {"other":"{count} संस्करण अभी भी चल रहे फ़्लो उपयोग कर रहे हैं। उनका रन खत्म होते ही वे हटा दिए जाएँगे।","one":"{count} संस्करण अभी भी चल रहे फ़्लो उपयोग कर रहा है। उसका रन खत्म होते ही वह हटा दिया जाएगा।"},
    "it": {"other":"{count} versioni sono ancora usate da flow in esecuzione. Vengono eliminate non appena la loro esecuzione termina.","one":"{count} versione è ancora usata da flow in esecuzione. Viene eliminata non appena la sua esecuzione termina."},
    "ja": {"other":"{count} 件のバージョンは実行中のフローがまだ使用しています。実行が終わり次第削除されます。"},
    "nl": {"other":"{count} versies worden nog gebruikt door lopende flows. Ze worden verwijderd zodra hun run eindigt.","one":"{count} versie wordt nog gebruikt door lopende flows. Die wordt verwijderd zodra de run eindigt."},
    "no": {"other":"{count} versjoner brukes fortsatt av kjørende flows. De slettes så snart kjøringen deres er ferdig.","one":"{count} versjon brukes fortsatt av kjørende flows. Den slettes så snart kjøringen er ferdig."},
    "pl": {"other":"{count} wersji jest nadal używanych przez działające flow. Zostaną usunięte, gdy tylko ich uruchomienie się zakończy.","one":"{count} wersja jest nadal używana przez działające flow. Zostanie usunięta, gdy tylko jej uruchomienie się zakończy.","few":"{count} wersje są nadal używane przez działające flow. Zostaną usunięte, gdy tylko ich uruchomienie się zakończy.","many":"{count} wersji jest nadal używanych przez działające flow. Zostaną usunięte, gdy tylko ich uruchomienie się zakończy."},
    "pt": {"other":"{count} versões ainda são usadas por fluxos em execução. São eliminadas assim que a execução terminar.","one":"{count} versão ainda é usada por fluxos em execução. É eliminada assim que a execução terminar."},
    "sv": {"other":"{count} versioner används fortfarande av körande flöden. De raderas så snart körningen är klar.","one":"{count} version används fortfarande av körande flöden. Den raderas så snart körningen är klar."},
    "zh": {"other":"{count} 个版本仍被运行中的流程使用，其运行结束后会立即删除。"}
  },
  "easydrag.ui.versions_connections": {
    "_after": "easydrag.ui.versions_title",
    "cs": {"other":"{count} spojení","one":"{count} spojení","few":"{count} spojení"},
    "da": {"other":"{count} forbindelser","one":"{count} forbindelse"},
    "de": {"other":"{count} Verbindungen","one":"{count} Verbindung"},
    "el": {"other":"{count} συνδέσεις","one":"{count} σύνδεση"},
    "en": {"other":"{count} connections","one":"{count} connection"},
    "es": {"other":"{count} conexiones","one":"{count} conexión"},
    "fr": {"other":"{count} connexions","one":"{count} connexion"},
    "hi": {"other":"{count} कनेक्शन","one":"{count} कनेक्शन"},
    "it": {"other":"{count} connessioni","one":"{count} connessione"},
    "ja": {"other":"{count} 接続"},
    "nl": {"other":"{count} verbindingen","one":"{count} verbinding"},
    "no": {"other":"{count} koblinger","one":"{count} kobling"},
    "pl": {"other":"{count} połączeń","one":"{count} połączenie","few":"{count} połączenia","many":"{count} połączeń"},
    "pt": {"other":"{count} ligações","one":"{count} ligação"},
    "sv": {"other":"{count} kopplingar","one":"{count} koppling"},
    "zh": {"other":"{count} 条连接"}
  }
}
```

`C:/disposable/easydrag-w1a/keys/c-3a-neutral.json` (for the shared `locale-apply.mjs`; locales that keep their text repeat it):

```json
{
  "easydrag.ui.home_bulk_selected": {
    "cs": "Vybráno: {count}",
    "da": "Valgt: {count}",
    "de": "{count} ausgewählt",
    "el": "Επιλεγμένες ροές: {count}",
    "en": "{count} selected",
    "es": "Seleccionados: {count}",
    "fr": "Sélectionnés : {count}",
    "hi": "{count} चुने गए",
    "it": "Selezionati: {count}",
    "ja": "{count} 件を選択中",
    "nl": "{count} flows geselecteerd",
    "no": "Valgt: {count}",
    "pl": "Wybrano: {count}",
    "pt": "Selecionados: {count}",
    "sv": "Valda: {count}",
    "zh": "已选择 {count} 项"
  },
  "easydrag.ui.batch_result_summary": {
    "cs": "Zpracováno: {count} · úspěšných: {succeeded} · neúspěšných: {failed}",
    "da": "{count} behandlet · {succeeded} lykkedes · {failed} fejlede",
    "de": "{count} verarbeitet · {succeeded} erfolgreich · {failed} fehlgeschlagen",
    "el": "Επεξεργασμένα: {count} · πέτυχαν: {succeeded} · απέτυχαν: {failed}",
    "en": "{count} processed · {succeeded} succeeded · {failed} failed",
    "es": "Procesados: {count} · correctos: {succeeded} · fallidos: {failed}",
    "fr": "Traités : {count} · réussis : {succeeded} · échoués : {failed}",
    "hi": "{count} संसाधित · {succeeded} सफल · {failed} विफल",
    "it": "Elaborati: {count} · riusciti: {succeeded} · falliti: {failed}",
    "ja": "処理 {count} 件 · 成功 {succeeded} 件 · 失敗 {failed} 件",
    "nl": "{count} verwerkt · {succeeded} geslaagd · {failed} mislukt",
    "no": "{count} behandlet · {succeeded} vellykket · {failed} feilet",
    "pl": "Przetworzono: {count} · powodzenie: {succeeded} · błędy: {failed}",
    "pt": "Processados: {count} · concluídos: {succeeded} · falhados: {failed}",
    "sv": "Bearbetade: {count} · lyckades: {succeeded} · misslyckades: {failed}",
    "zh": "已处理 {count} 项 · 成功 {succeeded} 项 · 失败 {failed} 项"
  }
}
```

`C:/disposable/easydrag-w1a/keys/c-3b-delete.json` (applied in C.3b, when the last reader of the old texts is gone):

```json
{ "_delete": ["easydrag.ui.versions_stats", "easydrag.ui.versions_stats_one", "easydrag.ui.versions_stats_edge_one"] }
```

- [ ] **Step 5: Apply both and check the result**

```
cd W && node C:/disposable/easydrag-w1a/locale-apply-forms.mjs C:/disposable/easydrag-w1a/keys/c-3a-plural.json
cd W && node C:/disposable/easydrag-w1a/locale-apply.mjs C:/disposable/easydrag-w1a/keys/c-3a-neutral.json
cd W && git diff --stat -- ui/lang/easydrag
```

Expected: `.../c-3a-plural.json: 10 count keys, 0 deleted, in 16 files of ui/lang/easydrag/`, then `.../c-3a-neutral.json: 2 keys set, 0 deleted, in 16 files of ui/lang/easydrag/`, and 16 changed files. Run each command a second time: the second `git diff --stat` is unchanged (idempotent).

- [ ] **Step 6: Restore the real-file assertions** in `scripts/test-easydrag-format.mjs`. Replace the block from `const stubT = dict =>` through the `assert.deepEqual(... 'pl: one, few, many, few');` line (the stub lines of C.1 Step 1) by

```js
assert.deepEqual([1, 2, 5, 22].map(n => render('cs', n)), ['1 krok', '2 kroky', '5 kroků', '22 kroků'], 'cs: one, few, other');
assert.deepEqual([1, 2, 5, 22].map(n => render('pl', n)), ['1 krok', '2 kroki', '5 kroków', '22 kroki'], 'pl: one, few, many, few');
```

- [ ] **Step 7: Run**

```
cd W && node scripts/test-easydrag-format.mjs
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1
```

Expected: `easydrag format: ok` and `ok  aurago/ui`. (`versions_stats` stays in the locale files until C.3b has removed its only reader in `publish.js`; the delete key file of Step 4 is applied there.)

- [ ] **Step 8: Gates and commit.**

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui && gofmt -l ui/desktop_easydrag_plural_test.go
cd W && npm run test:easydrag:t0
```

Expected: no output from vet and gofmt, no FAIL in T0. Message `C:/disposable/easydrag-w1a/c3a-msg.txt`:

```
feat(easydrag): plural forms in all 16 locales (S0c row 23)

Ten count texts (folder_flows, group_*, cred_purge_versions, cred_purge_pinned and
the new versions_connections) get the forms Intl.PluralRules gives each locale:
cs one/few, pl one/few/many, one form in twelve others, none in ja and zh. Two
count texts that agreed with the count in some languages become colon texts.
A Go test lists every {count} key as plural or number-neutral and checks the forms.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
```

```
cd W && git add ui/lang/easydrag ui/desktop_easydrag_plural_test.go scripts/test-easydrag-format.mjs && git commit -F C:/disposable/easydrag-w1a/c3a-msg.txt
```

---

### Task C.3b: Call sites of the count keys, `pluralKey`, the old test

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-groups.js:34`, `easydrag-publish.js:288`, `easydrag-credentials-purge.js:67,69`, `easydrag-metadata.js:177`, `easydrag-core.js:148-152` (`pluralKey`)
- Delete: `scripts/test-easydrag-plurals.mjs`
- Modify: `scripts/test-easydrag-credentials-purge.mjs:38`, `ui/desktop_easydrag_groups_browser_test.go:162`
- Create: `ui/desktop_easydrag_plural_sites_test.go`
- Apply: `keys/c-3b-delete.json`

Call sites found by `grep -rn "'easydrag.ui.<key>'"` for the ten plural keys: `folder_flows` (metadata.js:177), `group_*` (groups.js:34), `cred_purge_versions` and `cred_purge_pinned` (credentials-purge.js:67,69), `versions_stats` (publish.js:288). The `announce_*_many` keys are read through `core.stepsText` (history.js:70, interact.js:65), already moved in C.2.

- [ ] **Step 1: IMPACT** for `pluralKey` (`impact "pluralKey" --direction upstream`; confirm with `grep -rn "pluralKey" ui/js scripts ui/*.go`: callers are groups.js, publish.js and the old test). Expected: LOW.

- [ ] **Step 2: Failing Go test** `W/ui/desktop_easydrag_plural_sites_test.go`:

```go
package ui

import (
	"strings"
	"testing"
)

// Every read of a plural key goes through ED.format.plural, so the forms of the locale are used.
func TestEasyDragPluralKeysAreReadThroughFormatPlural(t *testing.T) {
	for _, name := range easyDragModules {
		for i, line := range strings.Split(readDesktopAssetText(t, "js/desktop/apps/"+name), "\n") {
			for _, k := range easyDragPluralKeys {
				if strings.Contains(line, "'easydrag.ui."+k+"'") && !strings.Contains(line, "format.plural(") {
					t.Errorf("%s:%d reads easydrag.ui.%s without ED.format.plural", name, i+1, k)
				}
			}
			if name != "easydrag-core.js" && strings.Contains(line, "pluralKey(") {
				t.Errorf("%s:%d still calls pluralKey", name, i+1)
			}
		}
	}
	for _, key := range []string{"versions_stats", "versions_stats_one", "versions_stats_edge_one"} {
		for _, lang := range []string{"de", "en"} {
			if _, ok := readEasyDragLang(t, lang)["easydrag.ui."+key]; ok {
				t.Errorf("%s still has easydrag.ui.%s", lang, key)
			}
		}
	}
}
```

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydragPluralKeysAreRead' -count=1
```

Expected: FAIL with `easydrag-groups.js:34 reads easydrag.ui.group_member_count without ED.format.plural` (and the other keys of that line), `easydrag-publish.js:288 ...`, `easydrag-credentials-purge.js:67 ...`, `easydrag-metadata.js:177 ...`, and `de still has easydrag.ui.versions_stats`.

- [ ] **Step 3: Move the call sites.** Check first that `ED` is the file's constant in all four files (`grep -n "const ED = window.EasyDrag" ui/js/desktop/apps/easydrag-{groups,publish,credentials-purge,metadata}.js` prints four lines; it does).

`easydrag-groups.js:34`, old (one line):
```
            return [ed.t(core.pluralKey('easydrag.ui.group_member_count', item.members.length), { count: item.members.length }), ed.t(core.pluralKey('easydrag.ui.group_edges_count', boundaryCounts.get(item.id) || 0), { count: boundaryCounts.get(item.id) || 0 }), errors ? ed.t(core.pluralKey('easydrag.ui.group_errors', errors), { count: errors }) : '', running ? ed.t('easydrag.ui.group_running', { count: running }) : '', success ? ed.t('easydrag.ui.group_success', { count: success }) : '', disabled ? ed.t('easydrag.ui.group_disabled', { count: disabled }) : ''].filter(Boolean).join(' · ');
```
new:
```
            return [ED.format.plural(ed.t, 'easydrag.ui.group_member_count', item.members.length), ED.format.plural(ed.t, 'easydrag.ui.group_edges_count', boundaryCounts.get(item.id) || 0), errors ? ED.format.plural(ed.t, 'easydrag.ui.group_errors', errors) : '', running ? ED.format.plural(ed.t, 'easydrag.ui.group_running', running) : '', success ? ED.format.plural(ed.t, 'easydrag.ui.group_success', success) : '', disabled ? ED.format.plural(ed.t, 'easydrag.ui.group_disabled', disabled) : ''].filter(Boolean).join(' · ');
```

`easydrag-publish.js:288`, old:
```
                        esc(t(Number(v.node_count) === 1 ? core.pluralKey('easydrag.ui.versions_stats', 1) : core.pluralKey('easydrag.ui.versions_stats', Number(v.edge_count), '_edge_one'), { nodes: v.node_count || 0, edges: v.edge_count || 0 })) + '</span></div>' +
```
new:
```
                        esc(ED.format.plural(t, 'easydrag.ui.group_member_count', Number(v.node_count) || 0) + ' · ' + ED.format.plural(t, 'easydrag.ui.versions_connections', Number(v.edge_count) || 0)) + '</span></div>' +
```

`easydrag-credentials-purge.js:67`, old `esc(t('easydrag.ui.cred_purge_versions', { count: report.versions_purgeable || 0 }))`, new `esc(ED.format.plural(t, 'easydrag.ui.cred_purge_versions', report.versions_purgeable || 0))`. Line 69, old `esc(t('easydrag.ui.cred_purge_pinned', { count: report.versions_pinned }))`, new `esc(ED.format.plural(t, 'easydrag.ui.cred_purge_pinned', report.versions_pinned))`.

`easydrag-metadata.js:177`, old `env.esc(env.t('easydrag.ui.folder_flows', { count: row.flow_count }))`, new `env.esc(ED.format.plural(env.t, 'easydrag.ui.folder_flows', row.flow_count))`.

`easydrag-core.js:148-152`, old:
```
    // pluralKey picks the key of a count text: base + suffix when the count is exactly one, else
    // base. It is the choice stepsText makes (one or several), for texts that keep their {count}.
    function pluralKey(base, count, suffix) {
        return count === 1 ? base + (suffix || '_one') : base;
    }
```
new:
```
    // pluralKey is the key of a count text for the UI language (base_<category>, base for "other"); a
    // thin wrapper over ED.format.key. Texts are rendered with ED.format.plural, which also falls back.
    function pluralKey(base, count) { return ED.format.key(base, count); }
```

- [ ] **Step 4: Delete the old test and apply the key delete.** `scripts/test-easydrag-plurals.mjs` tested the one/other selection and the `versions_stats` variants; its coverage moves to `test-easydrag-format.mjs` (selection per language), the Go forms test (every locale, placeholders of each form against the bare form) and the Go sites test.

```
cd W && git rm scripts/test-easydrag-plurals.mjs
cd W && node C:/disposable/easydrag-w1a/locale-apply.mjs C:/disposable/easydrag-w1a/keys/c-3b-delete.json
```

Expected: `.../c-3b-delete.json: 0 keys set, 3 deleted, in 16 files of ui/lang/easydrag/`. (The helper allows a key file with only `_delete`.)

- [ ] **Step 5: Update the two tests that read the old texts.**

`scripts/test-easydrag-credentials-purge.mjs:38`, old `dialog.includes('cred_purge_pinned[1]')`, new `dialog.includes('cred_purge_pinned_one[1]')` (count 1 is the `one` form; the stub `tt` returns `key[params]` and is never the bare key, so the plural form counts as present).

`ui/desktop_easydrag_groups_browser_test.go:162`, old `s.includes(ed.t('easydrag.ui.'+k,{count:1}))`, new `s.includes(EasyDrag.format.plural(ed.t,'easydrag.ui.'+k,1))`.

- [ ] **Step 6: Run.**

```
cd W && node scripts/test-easydrag-format.mjs && node scripts/test-easydrag-credentials-purge.mjs && node scripts/test-easydrag.mjs
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1
```

Expected: `easydrag format: ok`, the purge script silent, `all EasyDrag checks passed`, `ok  aurago/ui` (the sites test passes, the UI-key test finds no reader of `versions_stats`). `cd W && node scripts/test-easydrag-render-baseline.mjs` prints `PASS render baseline (unchanged)`.

- [ ] **Step 7: The browser test that reads the group counts (T1).**

```
cd W && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestDesktopEasyDragGroupsBrowser' -count=1 -timeout 30m -v
```

Run it with the Bash sandbox off (Chrome cannot start from a test binary under the repo). Expected: `--- PASS: TestDesktopEasyDragGroupsBrowser`.

- [ ] **Step 8: Proof greps and commit.**

```
cd W && grep -rn "pluralKey(" ui/js/desktop/apps/easydrag-*.js
cd W && grep -rn "versions_stats" ui scripts | grep -v "^ui/lang" | wc -l
```

Expected: only `easydrag-core.js` (the definition, one line), and `0`. Gates: `go vet ./ui`, `gofmt -l ui/desktop_easydrag_plural_sites_test.go` (silent), `npm run test:easydrag:t0` (no FAIL). Message `C:/disposable/easydrag-w1a/c3b-msg.txt`:

```
feat(easydrag): count texts render through ED.format.plural

groups, publish (versions row), credentials-purge and the folder list read their count
texts with the plural forms of the UI language. versions_stats (two counts in one text)
becomes group_member_count plus versions_connections. pluralKey is a wrapper over
ED.format.key. The A6 plurals test is replaced by the format test and two Go tests.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
```

```
cd W && git add ui/js/desktop/apps/easydrag-groups.js ui/js/desktop/apps/easydrag-publish.js ui/js/desktop/apps/easydrag-credentials-purge.js ui/js/desktop/apps/easydrag-metadata.js ui/js/desktop/apps/easydrag-core.js ui/lang/easydrag ui/desktop_easydrag_plural_sites_test.go ui/desktop_easydrag_groups_browser_test.go scripts/test-easydrag-credentials-purge.mjs && git commit -F C:/disposable/easydrag-w1a/c3b-msg.txt
```

(`git rm` already staged the deleted test.)

---

### Task C.4: "vN" for published states (CS-01)

**Files:**
- Create: `C:/disposable/easydrag-w1a/keys/c-4-vn.json`
- Modify: `ui/lang/easydrag/*.json` (16 files, through the helper)
- Modify: `ui/js/desktop/apps/easydrag-approvals.js:127` and `:269`
- Create: `ui/desktop_easydrag_version_words_test.go`

**What the survey found, measured on main.** With placeholders such as `{revision}` removed, the word "Revision" (any case) stands in **7 German and 9 English values**: `revision_live`, `pack_block_version`, `approval_revision`, `approval_blocked_stale`, `approval_stale_help`, `cred_purge_live_block_line`, `cred_purge_live_block_advice` (de and en), plus `publish_changes` and `publish_tested_revision` (en only). A numbered "Version N" stands in `run_view_banner`, `run_view_pruned` and `child_history_reference`. `revision_ahead` ("newer than live {n}") names a numbered state too. That is **13 keys** (de changes 11 values, en 13); the key file gives all 16 locales for each, 208 values, where a locale that already reads right repeats its text. The survey's "46 de lines" counts every "Version" word. A generic "veröffentlichte Version" without a number cannot read "vN" and stays; `versions_confirm` and `versions_restored` already say `version v{revision}`, and the collective noun "Versionen" stays.

Rule for the texts: the number is written `v{revision}` or `v{n}`; a noun in front is dropped where the sentence reads without it, and kept where grammar needs a subject (pl, hi, ja and zh in `run_view_pruned`). Words for "revision" in the other locales (Revize, Révision, Revisión, リビジョン, 修订, Rewizja, Αναθεώρηση, संशोधन) become the locale's word for "version".

- [ ] **Step 1: Write the failing Go test** `W/ui/desktop_easydrag_version_words_test.go`:

```go
package ui

import (
	"regexp"
	"strings"
	"testing"
)

var easyDragVersionKeys = []string{
	"revision_live", "revision_ahead", "run_view_banner", "run_view_pruned", "child_history_reference",
	"pack_block_version", "approval_revision", "approval_blocked_stale", "approval_stale_help",
	"cred_purge_live_block_line", "cred_purge_live_block_advice", "publish_changes", "publish_tested_revision",
}

// No German or English value says "Revision" (any case) or "Version <number>": a published state is vN.
func TestEasyDragVersionsReadAsVN(t *testing.T) {
	placeholder := regexp.MustCompile(`\{[^}]*\}`)
	numbered := regexp.MustCompile(`(?i)\bversions?\s+\{(revision|n)\}`)
	for _, lang := range []string{"de", "en"} {
		for key, text := range readEasyDragLang(t, lang) {
			if strings.Contains(strings.ToLower(placeholder.ReplaceAllString(text, "")), "revision") {
				t.Errorf("%s %s says Revision: %q", lang, key, text)
			}
			if numbered.MatchString(text) {
				t.Errorf("%s %s says Version <number>, want v<number>: %q", lang, key, text)
			}
		}
	}
	en := readEasyDragLang(t, "en")
	for _, lang := range easyDragPluralLocalesAll() {
		words := readEasyDragLang(t, lang)
		for _, k := range easyDragVersionKeys {
			key := "easydrag.ui." + k
			want := easyDragPlaceholder.FindAllString(en[key], -1)
			for _, p := range want {
				if !strings.Contains(words[key], p) {
					t.Errorf("%s %s lost the placeholder %s", lang, key, p)
				}
			}
		}
	}
}

func easyDragPluralLocalesAll() []string {
	langs := make([]string, 0, len(easyDragPluralForms))
	for lang := range easyDragPluralForms {
		langs = append(langs, lang)
	}
	return langs
}
```

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydragVersionsReadAsVN' -count=1
```

Expected: FAIL, `de easydrag.ui.revision_live says Revision: "Live · Revision {n}"` and the other six de keys, nine en keys, and `de easydrag.ui.run_view_banner says Version <number>`.

- [ ] **Step 2: Write the key file** `C:/disposable/easydrag-w1a/keys/c-4-vn.json` (helper `locale-apply.mjs`; German uses lowercase "du"):

```json
{
  "easydrag.ui.revision_live": {
    "cs": "Ostrý · v{n}",
    "da": "Live · v{n}",
    "de": "v{n} ist live",
    "el": "Κανονική · v{n}",
    "en": "v{n} is live",
    "es": "Real · v{n}",
    "fr": "Réelle · v{n}",
    "hi": "लाइव · v{n}",
    "it": "Reale · v{n}",
    "ja": "本番 · v{n}",
    "nl": "Live · v{n}",
    "no": "Live · v{n}",
    "pl": "Rzeczywiste · v{n}",
    "pt": "Real · v{n}",
    "sv": "Skarp · v{n}",
    "zh": "正式 · v{n}"
  },
  "easydrag.ui.revision_ahead": {
    "cs": "Změny od v{n}",
    "da": "Ændringer siden v{n}",
    "de": "Änderungen seit v{n}",
    "el": "Αλλαγές από v{n}",
    "en": "Changes since v{n}",
    "es": "Cambios desde v{n}",
    "fr": "Modifications depuis v{n}",
    "hi": "v{n} के बाद के बदलाव",
    "it": "Modifiche da v{n}",
    "ja": "v{n} からの変更",
    "nl": "Wijzigingen sinds v{n}",
    "no": "Endringer siden v{n}",
    "pl": "Zmiany od v{n}",
    "pt": "Alterações desde v{n}",
    "sv": "Ändringar sedan v{n}",
    "zh": "自 v{n} 以来的更改"
  },
  "easydrag.ui.run_view_banner": {
    "cs": "Běh z {time} · {status} · v{revision} (jen ke čtení)",
    "da": "Kørsel fra {time} · {status} · v{revision} (skrivebeskyttet)",
    "de": "Lauf vom {time} · {status} · v{revision} (schreibgeschützt)",
    "el": "Εκτέλεση της {time} · {status} · v{revision} (μόνο ανάγνωση)",
    "en": "Run from {time} · {status} · v{revision} (read-only)",
    "es": "Ejecución del {time} · {status} · v{revision} (solo lectura)",
    "fr": "Exécution du {time} · {status} · v{revision} (lecture seule)",
    "hi": "{time} का रन · {status} · v{revision} (केवल पढ़ने के लिए)",
    "it": "Esecuzione del {time} · {status} · v{revision} (sola lettura)",
    "ja": "{time} の実行 · {status} · v{revision}（読み取り専用）",
    "nl": "Uitvoering van {time} · {status} · v{revision} (alleen-lezen)",
    "no": "Kjøring fra {time} · {status} · v{revision} (skrivebeskyttet)",
    "pl": "Uruchomienie z {time} · {status} · v{revision} (tylko do odczytu)",
    "pt": "Execução de {time} · {status} · v{revision} (somente leitura)",
    "sv": "Körning från {time} · {status} · v{revision} (skrivskyddad)",
    "zh": "{time} 的运行 · {status} · v{revision}（只读）"
  },
  "easydrag.ui.run_view_pruned": {
    "cs": "v{revision} tohoto flow byla vyčištěna: EasyDrag uchovává posledních 50 publikovaných verzí. Běh zůstává v seznamu.",
    "da": "v{revision} af dette flow er ryddet op: EasyDrag beholder de seneste 50 udgivne versioner. Kørslen bliver på listen.",
    "de": "v{revision} dieses Flows wurde aufgeräumt: EasyDrag behält die letzten 50 veröffentlichten Versionen. Der Lauf bleibt in der Liste.",
    "el": "Η v{revision} αυτής της ροής καθαρίστηκε: το EasyDrag κρατά τις τελευταίες 50 δημοσιευμένες εκδόσεις. Η εκτέλεση παραμένει στη λίστα.",
    "en": "v{revision} of this flow was cleaned up: EasyDrag keeps the last 50 published versions. The run stays in the list.",
    "es": "La v{revision} de este flujo se ha limpiado: EasyDrag conserva las últimas 50 versiones publicadas. La ejecución permanece en la lista.",
    "fr": "La v{revision} de ce flux a été nettoyée : EasyDrag conserve les 50 dernières versions publiées. L'exécution reste dans la liste.",
    "hi": "इस फ़्लो का संस्करण v{revision} साफ़ कर दिया गया: EasyDrag अंतिम 50 प्रकाशित संस्करण रखता है। रन सूची में बना रहता है।",
    "it": "La v{revision} di questo flusso è stata ripulita: EasyDrag conserva le ultime 50 versioni pubblicate. L'esecuzione resta nell'elenco.",
    "ja": "このフローのバージョン v{revision} は整理されました。EasyDrag は公開済みの直近 50 バージョンを保持します。実行は一覧に残ります。",
    "nl": "v{revision} van deze flow is opgeruimd: EasyDrag bewaart de laatste 50 gepubliceerde versies. De run blijft in de lijst staan.",
    "no": "v{revision} av denne flyten er ryddet bort: EasyDrag beholder de siste 50 publiserte versjonene. Kjøringen blir stående i listen.",
    "pl": "Wersja v{revision} tego przepływu została wyczyszczona: EasyDrag przechowuje ostatnie 50 opublikowanych wersji. Uruchomienie pozostaje na liście.",
    "pt": "A v{revision} deste fluxo foi limpa: o EasyDrag mantém as últimas 50 versões publicadas. A execução continua na lista.",
    "sv": "v{revision} av det här flödet har rensats bort: EasyDrag behåller de senaste 50 publicerade versionerna. Körningen finns kvar i listan.",
    "zh": "此流程的版本 v{revision} 已被清理：EasyDrag 保留最近 50 个已发布版本。该运行仍保留在列表中。"
  },
  "easydrag.ui.child_history_reference": {
    "cs": "{block} · v{revision}",
    "da": "{block} · v{revision}",
    "de": "{block} · v{revision}",
    "el": "{block} · v{revision}",
    "en": "{block} · v{revision}",
    "es": "{block} · v{revision}",
    "fr": "{block} · v{revision}",
    "hi": "{block} · v{revision}",
    "it": "{block} · v{revision}",
    "ja": "{block} · v{revision}",
    "nl": "{block} · v{revision}",
    "no": "{block} · v{revision}",
    "pl": "{block} · v{revision}",
    "pt": "{block} · v{revision}",
    "sv": "{block} · v{revision}",
    "zh": "{block} · v{revision}"
  },
  "easydrag.ui.pack_block_version": {
    "cs": "{name} · zdroj v{revision}",
    "da": "{name} · kilde v{revision}",
    "de": "{name} · Quelle v{revision}",
    "el": "{name} · πηγή v{revision}",
    "en": "{name} · source v{revision}",
    "es": "{name} · origen v{revision}",
    "fr": "{name} · source v{revision}",
    "hi": "{name} · स्रोत v{revision}",
    "it": "{name} · sorgente v{revision}",
    "ja": "{name} · 元の v{revision}",
    "nl": "{name} · bron v{revision}",
    "no": "{name} · kilde v{revision}",
    "pl": "{name} · źródło v{revision}",
    "pt": "{name} · origem v{revision}",
    "sv": "{name} · källa v{revision}",
    "zh": "{name} · 源 v{revision}"
  },
  "easydrag.ui.approval_revision": {
    "cs": "Vydaná verze",
    "da": "Udgivet version",
    "de": "Veröffentlichte Version",
    "el": "Δημοσιευμένη έκδοση",
    "en": "Published version",
    "es": "Versión publicada",
    "fr": "Version publiée",
    "hi": "प्रकाशित संस्करण",
    "it": "Versione pubblicata",
    "ja": "公開済みバージョン",
    "nl": "Gepubliceerde versie",
    "no": "Publisert versjon",
    "pl": "Opublikowana wersja",
    "pt": "Versão publicada",
    "sv": "Publicerad version",
    "zh": "已发布版本"
  },
  "easydrag.ui.approval_blocked_stale": {
    "cs": "Žádost už neodpovídá uloženému toku, verzi nebo vlastníkovi.",
    "da": "Anmodningen stemmer ikke længere overens med det gemte flow, version eller ejer.",
    "de": "Die Anfrage stimmt nicht mehr mit dem gespeicherten Flow, der Version oder dem Besitzer überein.",
    "el": "Το αίτημα δεν ταιριάζει πλέον με την αποθηκευμένη ροή, έκδοση ή ιδιοκτήτη.",
    "en": "The request no longer matches its saved flow, version or owner.",
    "es": "La solicitud ya no coincide con el flujo, la versión o el propietario guardados.",
    "fr": "La demande ne correspond plus au flux, à la version ou au propriétaire enregistré.",
    "hi": "अनुरोध अब सहेजे गए फ़्लो, संस्करण या स्वामी से मेल नहीं खाता।",
    "it": "La richiesta non corrisponde più al flusso, alla versione o al proprietario salvati.",
    "ja": "リクエストが保存済みのフロー、バージョン、または所有者と一致しなくなりました。",
    "nl": "Het verzoek komt niet meer overeen met de opgeslagen flow, versie of eigenaar.",
    "no": "Forespørselen samsvarer ikke lenger med den lagrede flyten, versjonen eller eieren.",
    "pl": "Żądanie nie pasuje już do zapisanego przepływu, wersji ani właściciela.",
    "pt": "A solicitação não corresponde mais ao fluxo, à versão ou ao proprietário salvos.",
    "sv": "Begäran matchar inte längre det sparade flödet, versionen eller ägaren.",
    "zh": "请求与已保存的流程、版本或所有者不再匹配。"
  },
  "easydrag.ui.approval_stale_help": {
    "cs": "Uložená žádost už přesně neodpovídá toku, verzi nebo vlastníkovi. Než cokoli provedete, zkontrolujte aktuální běh.",
    "da": "Den gemte anmodning matcher ikke længere præcist flowet, versionen eller ejeren. Gennemgå den aktuelle kørsel, før du foretager dig noget.",
    "de": "Die gespeicherte Anfrage stimmt nicht mehr exakt mit Flow, Version oder Besitzer überein. Prüfe die aktuelle Ausführung, bevor du handelst.",
    "el": "Το αποθηκευμένο αίτημα δεν ταιριάζει πλέον ακριβώς με τη ροή, την έκδοση ή τον ιδιοκτήτη. Ελέγξτε την τρέχουσα εκτέλεση πριν ενεργήσετε.",
    "en": "The saved request no longer matches its exact flow, version or owner. Review the current run before taking any action.",
    "es": "La solicitud guardada ya no coincide exactamente con su flujo, versión o propietario. Revisa la ejecución actual antes de actuar.",
    "fr": "La demande enregistrée ne correspond plus exactement au flux, à la version ou au propriétaire. Examinez l’exécution actuelle avant d’agir.",
    "hi": "सहेजा गया अनुरोध अब फ़्लो, संस्करण या स्वामी से पूरी तरह मेल नहीं खाता। कोई कार्रवाई करने से पहले वर्तमान रन की समीक्षा करें।",
    "it": "La richiesta salvata non corrisponde più esattamente al flusso, alla versione o al proprietario. Esamina l'esecuzione corrente prima di agire.",
    "ja": "保存済みリクエストがフロー、バージョン、所有者と正確に一致しません。操作の前に現在の実行を確認してください。",
    "nl": "Het opgeslagen verzoek komt niet meer exact overeen met de flow, versie of eigenaar. Beoordeel de huidige uitvoering voordat je handelt.",
    "no": "Den lagrede forespørselen samsvarer ikke lenger nøyaktig med flyten, versjonen eller eieren. Gjennomgå den gjeldende kjøringen før du handler.",
    "pl": "Zapisane żądanie nie pasuje już dokładnie do przepływu, wersji ani właściciela. Przed podjęciem działania sprawdź bieżące uruchomienie.",
    "pt": "A solicitação salva não corresponde mais exatamente ao fluxo, à versão ou ao proprietário. Revise a execução atual antes de agir.",
    "sv": "Den sparade begäran matchar inte längre exakt flödet, versionen eller ägaren. Granska den aktuella körningen innan du agerar.",
    "zh": "已保存的请求与流程、版本或所有者不再完全匹配。执行操作前请检查当前运行。"
  },
  "easydrag.ui.cred_purge_live_block_line": {
    "cs": "Blok {id}, v{revision}, používá ho:",
    "da": "Blok {id}, v{revision}, bruges af:",
    "de": "Baustein {id}, v{revision}, benutzt von:",
    "el": "Μπλοκ {id}, v{revision}, χρησιμοποιείται από:",
    "en": "Block {id}, v{revision}, used by:",
    "es": "Bloque {id}, v{revision}, usado por:",
    "fr": "Bloc {id}, v{revision}, utilisé par :",
    "hi": "ब्लॉक {id}, v{revision}, इनके द्वारा उपयोग किया जाता है:",
    "it": "Blocco {id}, v{revision}, usato da:",
    "ja": "ブロック {id}、v{revision}、使用元:",
    "nl": "Blok {id}, v{revision}, gebruikt door:",
    "no": "Blokk {id}, v{revision}, brukes av:",
    "pl": "Blok {id}, v{revision}, używany przez:",
    "pt": "Bloco {id}, v{revision}, usado por:",
    "sv": "Block {id}, v{revision}, används av:",
    "zh": "区块 {id}，v{revision}，使用方："
  },
  "easydrag.ui.cred_purge_live_block_advice": {
    "cs": "Tuto verzi bloku stále používá běžící flow, proto zůstává. Přesuňte token v bloku do trezoru, publikujte blok a poté znovu publikujte nadřazený flow.",
    "da": "Et live flow bruger stadig denne blokversion, så den bliver. Flyt tokenet i blokken til boksen, udgiv blokken, og udgiv derefter det overordnede flow igen.",
    "de": "Ein live laufender Flow nutzt diese Bausteinversion noch, deshalb bleibt sie. Verschiebe das Token im Baustein in den Tresor, veröffentliche den Baustein und veröffentliche danach den übergeordneten Flow neu.",
    "el": "Μια ενεργή ροή εξακολουθεί να χρησιμοποιεί αυτή την έκδοση του μπλοκ, γι' αυτό παραμένει. Μετακινήστε το διακριτικό στο μπλοκ στο θησαυροφυλάκιο, δημοσιεύστε το μπλοκ και μετά δημοσιεύστε ξανά τη γονική ροή.",
    "en": "A live flow still runs this block version, so it stays. Move the token in the block into the vault, publish the block, then publish the parent flow again.",
    "es": "Un flujo en vivo sigue usando esta versión del bloque, por eso se conserva. Mueve el token del bloque a la bóveda, publica el bloque y después vuelve a publicar el flujo principal.",
    "fr": "Un flux en production utilise encore cette version du bloc, elle est donc conservée. Déplacez le jeton du bloc dans le coffre, publiez le bloc, puis republiez le flux parent.",
    "hi": "एक लाइव फ़्लो अभी भी इस ब्लॉक संस्करण का उपयोग करता है, इसलिए यह बना रहता है। ब्लॉक में मौजूद टोकन को वॉल्ट में ले जाएँ, ब्लॉक प्रकाशित करें, फिर मूल फ़्लो को दोबारा प्रकाशित करें।",
    "it": "Un flusso attivo usa ancora questa versione del blocco, quindi resta. Sposta il token del blocco nel vault, pubblica il blocco e poi pubblica di nuovo il flusso principale.",
    "ja": "稼働中のフローがこのブロックのバージョンをまだ使っているため、残されます。ブロック内のトークンを金庫に移し、ブロックを公開してから、親フローをもう一度公開してください。",
    "nl": "Een live flow gebruikt deze blokversie nog, dus ze blijft bestaan. Verplaats het token in het blok naar de kluis, publiceer het blok en publiceer daarna de bovenliggende flow opnieuw.",
    "no": "En live flyt bruker fortsatt denne blokkversjonen, så den blir stående. Flytt tokenet i blokken til hvelvet, publiser blokken, og publiser deretter den overordnede flyten på nytt.",
    "pl": "Działający flow nadal używa tej wersji bloku, dlatego zostaje. Przenieś token w bloku do skarbca, opublikuj blok, a potem opublikuj ponownie nadrzędny flow.",
    "pt": "Um fluxo ativo ainda usa esta versão do bloco, por isso ela permanece. Mova o token do bloco para o cofre, publique o bloco e depois publique o fluxo principal novamente.",
    "sv": "Ett live-flöde använder fortfarande den här blockversionen, så den blir kvar. Flytta token i blocket till valvet, publicera blocket och publicera sedan det överordnade flödet igen.",
    "zh": "有一个运行中的流程仍在使用此区块版本，因此会保留。请把区块中的令牌移入保险库，发布该区块，然后重新发布上级流程。"
  },
  "easydrag.ui.publish_changes": {
    "cs": "Změny v této verzi",
    "da": "Ændringer i denne version",
    "de": "Änderungen in dieser Version",
    "el": "Αλλαγές σε αυτή την έκδοση",
    "en": "Changes in this version",
    "es": "Cambios en esta versión",
    "fr": "Modifications de cette version",
    "hi": "इस संस्करण में बदलाव",
    "it": "Modifiche in questa versione",
    "ja": "このバージョンの変更",
    "nl": "Wijzigingen in deze versie",
    "no": "Endringer i denne versjonen",
    "pl": "Zmiany w tej wersji",
    "pt": "Alterações nesta versão",
    "sv": "Ändringar i den här versionen",
    "zh": "此版本的更改"
  },
  "easydrag.ui.publish_tested_revision": {
    "cs": "Doklad o testu této verze",
    "da": "Testdokumentation for denne version",
    "de": "Testnachweis für diese Version",
    "el": "Στοιχεία δοκιμής αυτής της έκδοσης",
    "en": "Test evidence for this version",
    "es": "Evidencia de prueba de esta versión",
    "fr": "Preuve de test pour cette version",
    "hi": "इस संस्करण के परीक्षण का प्रमाण",
    "it": "Prova eseguita per questa versione",
    "ja": "このバージョンのテスト記録",
    "nl": "Testbewijs voor deze versie",
    "no": "Testdokumentasjon for denne versjonen",
    "pl": "Potwierdzenie testu tej wersji",
    "pt": "Comprovativo de teste desta versão",
    "sv": "Testunderlag för den här versionen",
    "zh": "此版本的测试记录"
  }
}
```

- [ ] **Step 3: Apply and check**

```
cd W && node C:/disposable/easydrag-w1a/locale-apply.mjs C:/disposable/easydrag-w1a/keys/c-4-vn.json
cd W && git diff --stat -- ui/lang/easydrag
```

Expected: `.../c-4-vn.json: 13 keys set, 0 deleted, in 16 files of ui/lang/easydrag/`; 16 files changed. Second run: no further diff.

- [ ] **Step 4: Show the version number as "vN" in the approval card.** The label `approval_revision` now reads "Veröffentlichte Version"; its value must read `v3`, not `3`. `easydrag-approvals.js:127` and `:269`, in both lines replace `esc(String(item.revision))` by `esc('v' + item.revision)` (the "v" is the same in every language). No test reads that value: `grep -rn "approval-meta" scripts ui/*.go` prints nothing.

- [ ] **Step 5: Run.**

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1
cd W && node scripts/test-easydrag.mjs && node scripts/test-easydrag-approvals.mjs
```

Expected: `ok  aurago/ui`, `all EasyDrag checks passed`, the approvals script silent. Proof grep: `cd W && grep -c "Revision" ui/lang/easydrag/de.json ui/lang/easydrag/en.json` prints `0` for both (the file names also match lower-case `revision` in keys such as `error_flow_revision_conflict`; the test strips placeholders and checks values only).

- [ ] **Step 6: Gates and commit.** `go vet ./ui`, `gofmt -l ui/desktop_easydrag_version_words_test.go` (silent), `npm run test:easydrag:t0`. Message `C:/disposable/easydrag-w1a/c4-msg.txt`:

```
feat(easydrag): published states read vN, no "Revision" in de or en (CS-01)

Thirteen texts in all 16 locales: v3 ist live, Änderungen seit v3, v{revision} in
the run banner, the child history and the pack list. "Revision" becomes "Version"
where no number stands. The approval card shows v<number>. A Go test checks de and
en for Revision and Version <number> and the placeholders of the 16 locales.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
```

```
cd W && git add ui/lang/easydrag ui/js/desktop/apps/easydrag-approvals.js ui/desktop_easydrag_version_words_test.go && git commit -F C:/disposable/easydrag-w1a/c4-msg.txt
```

---

### Task C.5: The pseudo-locale

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-format.js` (add `pseudoText`, `pseudoOn`, `wrapT`)
- Modify: `ui/js/desktop/apps/easydrag.js:225` (the one place where EasyDrag gets `t`)
- Modify: `scripts/test-easydrag-format.mjs`
- Create: `ui/desktop_easydrag_pseudo_test.go`

Switch: `localStorage['aurago.easydrag.pseudo'] === '1'`. EasyDrag gets its `t` in one place: `easydrag.js:225`, `t: ctx.t` (no other file reads `ctx.t`; checked with `grep -n "ctx\.t\b" ui/js/desktop/apps/easydrag*.js`). Every module reads `app.t` or `ed.t`, which come from `inst.t`. The wrapper reads the translated **text without values**, accents its letters, pads it by 35% of its visible length with `~`, brackets it, and then fills the placeholders, so a user's flow name is not accented. A missing key stays the key. Tags (`<b>`) and entities (`&amp;`) and the placeholders `{x}` and `{{x}}` are kept intact.

- [ ] **Step 1: Failing test.** In `scripts/test-easydrag-format.mjs` replace the final line `console.log('easydrag format: ok');` by

```js
// the pseudo-locale wrapper
const pseudo = F.pseudoText('Delete {count} steps in {{name}} <b>now</b> &amp; save');
assert.ok(pseudo.startsWith('[') && pseudo.endsWith(']'), 'bracketed');
assert.ok(pseudo.includes('{count}') && pseudo.includes('{{name}}') && pseudo.includes('<b>') && pseudo.includes('</b>') && pseudo.includes('&amp;'), 'placeholders, tags and entities stay intact');
assert.equal(/[A-Za-z]/.test(pseudo.replace(/\{\{[^}]*\}\}|\{[^}]*\}|<[^>]*>|&[#a-z0-9]+;/gi, '')), false, 'every letter outside the kept parts is accented');
const plain = 'Save changes';
assert.ok(F.pseudoText(plain).length - 2 >= Math.round(plain.length * 1.35), 'about 35% longer: ' + F.pseudoText(plain));
assert.equal(F.pseudoText(''), '[]');
store.clear(); win.SYSTEM_LANG = 'en';
const base = (key, vars) => { let s = { 'a.b': 'Hello {name}', 'a.c': 'Plain' }[key] || key; if (vars) Object.entries(vars).forEach(([x, y]) => { s = s.replaceAll('{' + x + '}', () => y); }); return s; };
assert.equal(F.wrapT(base), base, 'without the switch t is returned as it is');
store.set('aurago.easydrag.pseudo', '1');
const wrapped = F.wrapT(base);
assert.notEqual(wrapped, base);
assert.ok(wrapped('a.b', { name: 'Ada' }).includes('Ada') && wrapped('a.b', { name: 'Ada' }).startsWith('['), 'the value is not accented, the text is');
assert.equal(wrapped('a.b').includes('{name}'), true, 'without values the placeholder stays');
assert.equal(wrapped('no.such.key'), 'no.such.key', 'a missing key stays visible as the key');
store.set('aurago.easydrag.pseudo', '0');
assert.equal(F.wrapT(base), base, 'only "1" switches it on');
console.log('easydrag format: ok');
```

```
cd W && node scripts/test-easydrag-format.mjs
```

Expected: `TypeError: F.pseudoText is not a function`.

- [ ] **Step 2: Implement.** In `easydrag-format.js` insert before the line `    ED.format = { locale, ...` :

```js
    // ── pseudo-locale (development only): localStorage['aurago.easydrag.pseudo'] === '1' ──
    // t returns accented, about 35% longer, bracketed texts, so hard-coded strings and clipping show at
    // once. It is not a language: it is in no language list, and a Go test guards that.
    const PSEUDO_KEY = 'aurago.easydrag.pseudo';
    const FROM = 'abcdefghijklmnopqrstuvwxyz';
    const TO = 'åƀçðéƒĝĥîĵķļɱñöþǫŕšţûṽŵẋýž';
    const FROM_UP = FROM.toUpperCase();
    const TO_UP = 'ÅƁÇÐÉƑĜĤÎĴĶĻṀÑÖÞǪŔŠŢÛṼŴẊÝŽ';
    const ACCENT = {};
    for (let i = 0; i < FROM.length; i++) { ACCENT[FROM[i]] = TO[i]; ACCENT[FROM_UP[i]] = TO_UP[i]; }
    const KEEP = /\{\{[^}]*\}\}|\{[^}]*\}|<[^>]*>|&[#a-z0-9]+;/gi;

    // pseudoText accents the letters of text, keeps placeholders ({x}, {{x}}), tags and entities, pads by
    // 35% of the visible length with "~" and brackets the result.
    function pseudoText(text) {
        const source = String(text);
        const accent = part => part.replace(/[a-z]/gi, ch => ACCENT[ch]);
        let out = '', at = 0, visible = 0;
        for (const m of source.matchAll(KEEP)) {
            const part = source.slice(at, m.index);
            out += accent(part) + m[0];
            visible += part.length;
            at = m.index + m[0].length;
        }
        const tail = source.slice(at);
        out += accent(tail);
        visible += tail.length;
        return '[' + out + '~'.repeat(Math.ceil(visible * 0.35)) + ']';
    }

    function pseudoOn() {
        try { return localStorage.getItem(PSEUDO_KEY) === '1'; } catch (err) { return false; }
    }

    // wrapT returns t itself unless the switch is on. With it, a translated text is pseudo-localised
    // before the values are filled in (so a flow name stays readable); a missing key stays the key.
    function wrapT(t) {
        if (!pseudoOn()) return t;
        return function (key, vars) {
            const raw = t(key);
            if (typeof raw !== 'string' || raw === key) return t(key, vars);
            let text = pseudoText(raw);
            if (vars) Object.entries(vars).forEach(([name, value]) => { text = text.replaceAll('{{' + name + '}}', () => value).replaceAll('{' + name + '}', () => value); });
            return text;
        };
    }

```

and extend the export: `ED.format = { locale, category, key, number, plural, duration, date, relative, list, listSeparator, pseudoText, pseudoOn, wrapT, PSEUDO_KEY };`

- [ ] **Step 3: Wrap `t` where EasyDrag gets it.** `easydrag.js:225`, in the line `const inst = { windowId, ctx, root, t: ctx.t, esc: ...` replace `t: ctx.t,` by `t: ED.format.wrapT(ctx.t),` (nothing else on the line changes).

- [ ] **Step 4: Failing-first Go test** `W/ui/desktop_easydrag_pseudo_test.go`:

```go
package ui

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The pseudo-locale is a development switch, never a language: no language list may contain it.
func TestEasyDragPseudoLocaleIsInNoLanguageList(t *testing.T) {
	real := map[string]bool{}
	for lang := range easyDragPluralForms {
		real[lang] = true
	}
	// 1. every locale file under ui/lang is a real language (meta.json is the one other file)
	seen := 0
	err := filepath.WalkDir("lang", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".json") {
			return err
		}
		seen++
		name := strings.TrimSuffix(filepath.Base(path), ".json")
		if name != "meta" && !real[name] {
			t.Errorf("%s is not one of the 16 languages", path)
		}
		return nil
	})
	if err != nil || seen < 400 {
		t.Fatalf("walked %d locale files (%v); is the scan broken?", seen, err)
	}
	// 2. the language switcher of the shared UI lists real codes only
	shared := readDesktopAssetText(t, "js/shared/shared-core.js")
	codes := regexp.MustCompile(`\{ code: '([^']+)', label:`).FindAllStringSubmatch(shared, -1)
	if len(codes) != 16 {
		t.Fatalf("language switcher lists %d codes, want 16", len(codes))
	}
	for _, m := range codes {
		if !real[m[1]] {
			t.Errorf("the language switcher lists %q", m[1])
		}
	}
	// 3. the switch name lives in easydrag-format.js only
	for _, name := range easyDragModules {
		if name != "easydrag-format.js" && strings.Contains(readDesktopAssetText(t, "js/desktop/apps/"+name), "aurago.easydrag.pseudo") {
			t.Errorf("%s names the pseudo switch; only easydrag-format.js may", name)
		}
	}
	// 4. EasyDrag takes its t through the wrapper, in one place
	if !strings.Contains(readDesktopAssetText(t, "js/desktop/apps/easydrag.js"), "t: ED.format.wrapT(ctx.t)") {
		t.Error("easydrag.js does not wrap ctx.t with ED.format.wrapT")
	}
}
```

- [ ] **Step 5: Run**

```
cd W && node scripts/test-easydrag-format.mjs
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1
cd W && node scripts/test-easydrag.mjs && node scripts/test-easydrag-render-baseline.mjs
```

Expected: `easydrag format: ok`, `ok  aurago/ui`, `all EasyDrag checks passed`, `PASS render baseline (unchanged)`. If the Go test says `walked N locale files ... is the scan broken?`, run `find ui/lang -name '*.json' | wc -l` and lower the floor `400` to a bit below that count (the tree has 16 files per section and about 32 sections; 400 is a guard against walking nothing). Expected manual check: in a scratch window or the desktop preview harness (memory note `reference_desktop_preview_harness.md`), `localStorage.setItem('aurago.easydrag.pseudo','1')` and reopening EasyDrag shows `[Ŝţëþŝ~~~]`-style texts; this is a look, not a gate.

- [ ] **Step 6: Gates and commit.** `go vet ./ui`, `gofmt -l ui/desktop_easydrag_pseudo_test.go` (silent), `npm run test:easydrag:t0`. Message `C:/disposable/easydrag-w1a/c5-msg.txt`:

```
feat(easydrag): pseudo-locale behind localStorage aurago.easydrag.pseudo

With the switch on, EasyDrag's t returns accented, 35% longer, bracketed texts with
placeholders, tags and entities intact; a missing key stays the key. The wrapper sits
in the one place EasyDrag gets t. A Go test asserts that no language list contains it.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
```

```
cd W && git add ui/js/desktop/apps/easydrag-format.js ui/js/desktop/apps/easydrag.js scripts/test-easydrag-format.mjs ui/desktop_easydrag_pseudo_test.go && git commit -F C:/disposable/easydrag-w1a/c5-msg.txt
```

---

## Coverage: spec item to task

| Spec item | Task |
|---|---|
| §5.1 `ED.format`: plural, number, duration, date, relative, list | C.1 |
| §5.1 the UI locale (`SYSTEM_LANG`, as `core.lang()`) | C.1 |
| §5.1 `core.pluralKey` and `core.stepsText` become wrappers | C.2 (`stepsText`), C.3b (`pluralKey`) |
| §5.1 the en-GB filter, `toLocaleDateString`, `fields.js`, `authoring.js`, `core.js` formatters | C.2 (the en-GB filter stays, see deviations) |
| §5.2 plural forms of every `{count}` key, cs/pl and the rest | C.3a |
| §5.2 S0c row 23: `folder_flows`, `cred_purge_versions`, `cred_purge_pinned` | C.3a (texts), C.3b (call sites) |
| §5.2 a test lists which locales need which forms | C.3a (`easyDragPluralForms`, checked against Intl by the Node test) |
| §5.3 "vN", no "Revision" in de and en | C.4 |
| §5.4 the pseudo-locale and "never in a language list" | C.5 |
| §5.5 Node: de/cs/pl 1, 2, 5, 22; formatters follow the locale; pseudo wrapper | C.1, C.3a (real files), C.5 |
| §5.5 Go: forms per locale, no "Revision", pseudo never listed | C.3a, C.4, C.5 |
| §10 de, cs, pl render 1, 2, 5, 22; row 23 closed; "vN"; pseudo behind its switch | C.3a, C.4, C.5 |

## Deviations from the spec

1. **`ED.format.duration(400)` gives "400 ms", not "0,4 s".** The spec's example contradicts today's unit style, which `scripts/test-easydrag-ux-batch5.mjs` pins (`fmt.duration(400)` is `400 ms`, 2500 ms is `2,5 s`). The brief says to keep the style. Only the number format follows the language.
2. **The en-GB `Intl.DateTimeFormat` in `easydrag-template.js` stays.** It is the template engine's `date` filter, not UI text: the server evaluates the same expressions, so field order and Latin digits must not depend on the UI language (a Hindi UI would change user output). The line gets a trailing comment saying so. The other four sites move.
3. **Not every `{count}` key gets plural forms.** 25 of the 34 keys put the count after a colon or in brackets (or already read the same for any count); a form for them would be a copy. The Go test makes the split explicit (two lists, every `{count}` key must be in one), so a new `{count}` key forces a decision. `home_bulk_selected` (es, fr, it, pt, sv) and `batch_result_summary` (el, es, fr, it, pt, sv) are reworded to the colon form.
4. **A second locale script, `locale-apply-forms.mjs`.** The shared helper needs a text for every locale per key, so it cannot add `_few` for cs and pl only. The script is in this part (C.3a) and checks every locale against Intl.
5. **`versions_stats` and its two variants are replaced** by `group_member_count` plus the new `versions_connections` joined with " · " (the old text had two counts and could not be right in cs and pl). That adds one count key and deletes three.
6. **Measured vN numbers differ from the survey:** 7 German and 9 English values with "Revision" (any case), 13 keys to rewrite, not "46 de lines". Reason in C.4.
7. **`scripts/test-easydrag-plurals.mjs` (S0c A6) is deleted**; its checks move to the format test and the Go tests.
8. **Extra change:** the approval card shows `v<number>` (`easydrag-approvals.js:127,269`), so its "version" label and value agree with CS-01.
9. **Left alone on purpose:** `credentials-purge.js` `sizeText` still uses `toFixed(1)` (a number format, not in the survey; `core.fmt.bytes` would do and is a one-line follow-up); the `toLocaleLowerCase()` calls in search code; `Intl.supportedValuesOf` in `transform.js`.

## Decisions needed

1. **Colon wording for `home_bulk_selected` and `batch_result_summary`** (deviation 3). Recommendation: accept. The alternative is `_one` forms in five and six more locales with the same words; the colon form is shorter and correct for every count.
2. **Fold `locale-apply-forms.mjs` into the shared helper** so parts A and D can use forms too. Recommendation: not now; no other part has count keys.
3. **Should the "Version" noun stay in front of `v{revision}`** in `versions_confirm` and `versions_restored` ("version v3")? Recommendation: keep; they are already in vN form and the noun reads naturally in a dialog.
4. **Polish and Greek `run_view_pruned`, Hindi, Japanese, Chinese keep a noun before the number** ("Wersja v3 ..."). Recommendation: accept; a bare "v3 tego przepływu została..." needs a grammatical subject in these languages. A native check in the Wave 1 sessions is welcome for all pl, cs, el and hi texts written here.
