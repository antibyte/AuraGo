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

    ED.format = { locale, category, key, number, plural, duration, date, relative, list, listSeparator, pseudoText, pseudoOn, wrapT, PSEUDO_KEY };
})();
