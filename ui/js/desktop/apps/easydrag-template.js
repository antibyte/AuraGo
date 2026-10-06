// EasyDrag templates in the browser: {{path | filter}} parsing, reference renaming and the
// live preview. The server (internal/flows/template*.go) stays authoritative; the preview
// mirrors its filters so the user sees the same result before a test run.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};

    // segments splits text into literals and expressions; "\{{" is an escaped literal "{{".
    function segments(text) {
        const s = String(text == null ? '' : text);
        const out = [];
        let literal = '';
        let i = 0;
        while (i < s.length) {
            if (s[i] === '\\' && s.startsWith('{{', i + 1)) { literal += '{{'; i += 3; continue; }
            if (s.startsWith('{{', i)) {
                const end = s.indexOf('}}', i + 2);
                if (end < 0) { literal += s.slice(i); break; }
                if (literal) { out.push({ literal }); literal = ''; }
                out.push({ expr: s.slice(i + 2, end), start: i, end: end + 2 });
                i = end + 2;
                continue;
            }
            literal += s[i];
            i += 1;
        }
        if (literal) out.push({ literal });
        return out;
    }

    function splitOutsideQuotes(text, sep) {
        const parts = [];
        let current = '';
        let quote = '';
        for (let i = 0; i < text.length; i++) {
            const ch = text[i];
            if (quote) {
                current += ch;
                if (ch === '\\' && i + 1 < text.length) { current += text[++i]; continue; }
                if (ch === quote) quote = '';
                continue;
            }
            if (ch === '"' || ch === "'") { quote = ch; current += ch; continue; }
            if (ch === sep) { parts.push(current); current = ''; continue; }
            current += ch;
        }
        parts.push(current);
        return parts;
    }

    function parseLiteral(raw) {
        const s = raw.trim();
        if (!s) throw new Error('empty argument');
        if ((s[0] === '"' || s[0] === "'") && s[s.length - 1] === s[0] && s.length >= 2) {
            return s.slice(1, -1).replace(/\\(["'\\])/g, '$1').replace(/\\n/g, '\n').replace(/\\t/g, '\t');
        }
        if (s === 'true') return true;
        if (s === 'false') return false;
        if (s === 'null') return null;
        const n = Number(s);
        if (!isNaN(n) && /^-?\d+(\.\d+)?$/.test(s)) return n;
        throw new Error('invalid argument ' + s);
    }

    // parseExpr parses "root.a[0]["b"] | filter(1, "x")".
    function parseExpr(expr) {
        const parts = splitOutsideQuotes(String(expr), '|');
        const pathText = parts[0].trim();
        const match = /^([A-Za-z_][A-Za-z0-9_]*)/.exec(pathText);
        if (!match) throw new Error('missing root');
        const path = [];
        let rest = pathText.slice(match[0].length);
        while (rest.length) {
            let m = /^\.([A-Za-z0-9_$-]+)/.exec(rest);
            if (m) { path.push(m[1]); rest = rest.slice(m[0].length); continue; }
            m = /^\[(\d+)\]/.exec(rest);
            if (m) { path.push(Number(m[1])); rest = rest.slice(m[0].length); continue; }
            m = /^\[(["'])((?:\\.|(?!\1).)*)\1\]/.exec(rest);
            if (m) { path.push(m[2]); rest = rest.slice(m[0].length); continue; }
            throw new Error('invalid path ' + pathText);
        }
        const filters = parts.slice(1).map(raw => {
            const f = raw.trim();
            const fm = /^([a-z_]+)\s*(?:\((.*)\))?$/s.exec(f);
            if (!fm) throw new Error('invalid filter ' + f);
            const args = fm[2] !== undefined && fm[2].trim() !== '' ? splitOutsideQuotes(fm[2], ',').map(parseLiteral) : [];
            return { name: fm[1], args };
        });
        return { root: match[1], path, filters };
    }

    // refs lists the expressions of a text with their root and path; broken ones carry error.
    function refs(text) {
        return segments(text).filter(s => s.expr !== undefined).map(s => {
            try {
                const p = parseExpr(s.expr);
                return { root: p.root, path: p.path, filters: p.filters, expr: s.expr, start: s.start, end: s.end };
            } catch (err) {
                return { error: err.message, expr: s.expr, start: s.start, end: s.end };
            }
        });
    }

    function walk(value, fn) {
        if (typeof value === 'string') return fn(value);
        if (Array.isArray(value)) return value.map(v => walk(v, fn));
        if (value && typeof value === 'object') {
            const out = {};
            Object.keys(value).forEach(k => { out[k] = walk(value[k], fn); });
            return out;
        }
        return value;
    }

    function refsInValue(value) {
        const out = [];
        walk(value, s => { refs(s).forEach(r => out.push(r)); return s; });
        return out;
    }

    // renameRoot rewrites every expression whose root is oldKey; all other text stays byte-identical.
    function renameRoot(text, oldKey, newKey) {
        const s = String(text);
        if (s.indexOf('{{') < 0) return s;
        let out = '';
        let last = 0;
        segments(s).forEach(seg => {
            if (seg.expr === undefined) return;
            const re = new RegExp('^(\\s*)' + oldKey.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '(?=[\\s.\\[|]|$)');
            if (!re.test(seg.expr)) return;
            out += s.slice(last, seg.start) + '{{' + seg.expr.replace(re, '$1' + newKey) + '}}';
            last = seg.end;
        });
        return out + s.slice(last);
    }

    function renameInValue(value, oldKey, newKey) {
        return walk(value, s => renameRoot(s, oldKey, newKey));
    }

    function stringify(value) {
        if (value === null || value === undefined) return '';
        if (typeof value === 'string') return value;
        if (typeof value === 'number') return Number.isInteger(value) ? String(value) : String(value);
        if (typeof value === 'boolean') return value ? 'true' : 'false';
        try { return JSON.stringify(value); } catch (err) { return String(value); }
    }

    function toNumber(value) {
        if (typeof value === 'number') return value;
        if (typeof value === 'string' && value.trim() !== '' && !isNaN(Number(value))) return Number(value);
        return NaN;
    }

    function toDate(value) {
        if (value instanceof Date) return value;
        if (typeof value === 'number') return new Date(value > 1e12 ? value : value * 1000);
        const s = String(value || '').trim();
        if (!s) return null;
        const local = /^(\d{4})-(\d{2})-(\d{2})(?:[ T](\d{2}):(\d{2})(?::(\d{2}))?)?$/.exec(s);
        const date = local ? new Date(+local[1], +local[2] - 1, +local[3], +(local[4] || 0), +(local[5] || 0), +(local[6] || 0)) : new Date(s);
        return isNaN(date.getTime()) ? null : date;
    }

    function dateParts(date, tz) {
        const options = { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hourCycle: 'h23' };
        if (tz) options.timeZone = tz;
        const parts = {};
        new Intl.DateTimeFormat('en-GB', options).formatToParts(date).forEach(p => { parts[p.type] = p.value; });
        return parts;
    }

    function formatDate(date, pattern, tz) {
        const p = dateParts(date, tz);
        const tokens = { YYYY: p.year, MM: p.month, DD: p.day, HH: p.hour, mm: p.minute, ss: p.second };
        return String(pattern).replace(/YYYY|MM|DD|HH|mm|ss/g, tok => tokens[tok]);
    }

    const NAMED_ENTITIES = { amp: '&', lt: '<', gt: '>', quot: '"', apos: "'", nbsp: ' ' };

    function decodeEntities(s) {
        return s.replace(/&(?:#(\d+)|#[xX]([0-9A-Fa-f]+)|(amp|lt|gt|quot|apos|nbsp));/g, (m, dec, hex, name) => {
            if (name) return NAMED_ENTITIES[name];
            const cp = dec !== undefined ? parseInt(dec, 10) : parseInt(hex, 16);
            return cp > 0 && cp <= 0x10FFFF && (cp < 0xD800 || cp > 0xDFFF) ? String.fromCodePoint(cp) : '\uFFFD';
        });
    }

    // The strip_html patterns and the whitespace set mirror filterStripHTML in
    // internal/flows/filters.go: script and style blocks, comments, declarations and
    // tags (a letter must follow "<") become a space, then entities are decoded and
    // whitespace collapses to single spaces.
    const HTML_STRIP = [
        /<script\b[^>]*>.*?<\/script[\t\n\f\r ]*>/gis,
        /<style\b[^>]*>.*?<\/style[\t\n\f\r ]*>/gis,
        /<!--.*?-->/gs,
        /<![A-Za-z[][^>]*>|<\?[^>]*\?>/gi,
        /<\/?[A-Za-z][A-Za-z0-9:_-]*(?:[\t\n\f\r ][^>]*)?\/?>/g
    ];
    const GO_SPACE = /[\t\n\v\f\r \u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]+/;

    function stripHTML(value) {
        let text = stringify(value);
        HTML_STRIP.forEach(re => { text = text.replace(re, ' '); });
        return decodeEntities(text).split(GO_SPACE).filter(Boolean).join(' ');
    }

    // caseMap maps one character at a time like Go's strings.ToUpper/ToLower (simple
    // case mapping): an upper-case mapping that would expand (ß → SS) keeps the
    // character, and the one lower-case expansion (İ → i + dot) keeps its first letter.
    function caseMap(s, upper) {
        if (!/[^\x00-\x7f]/.test(s)) return upper ? s.toUpperCase() : s.toLowerCase();
        return Array.from(s, ch => {
            const mapped = Array.from(upper ? ch.toUpperCase() : ch.toLowerCase());
            if (mapped.length === 1) return mapped[0];
            return upper ? ch : mapped[0];
        }).join('');
    }

    // roundHalfAway rounds like Go's filterRound: halves away from zero (math.Round),
    // at most 10 decimals, no "-0", and a value too large to scale comes back as is.
    function roundHalfAway(value, places) {
        if (places > 10) throw new Error('at most 10 decimals');
        const n = toNumber(value);
        if (!isFinite(n)) throw new Error('not a number');
        const f = Math.pow(10, places);
        const scaled = n * f;
        const r = Math.sign(scaled) * Math.round(Math.abs(scaled)) / f;
        if (!isFinite(r)) return n;
        return r === 0 ? 0 : r;
    }

    function intArg(args, i, def, min) {
        if (i >= args.length) return def;
        const n = args[i];
        if (typeof n !== 'number' || !Number.isInteger(n)) throw new Error('argument must be a whole number');
        if (n < min) throw new Error('argument must be at least ' + min);
        return n;
    }

    function strArg(args, i, def) {
        if (i >= args.length) return def;
        if (typeof args[i] !== 'string') throw new Error('argument must be a string');
        return args[i];
    }

    const FILTERS = {
        default: { min: 1, max: 1, hint: '(value)', fn: (v, a) => (v === null || v === undefined || v === '') ? a[0] : v },
        truncate: { min: 1, max: 1, hint: '(100)', fn: (v, a) => { const n = intArg(a, 0, 0, 1); const chars = Array.from(stringify(v)); return chars.length <= n ? chars.join('') : chars.slice(0, n).join('') + '…'; } },
        upper: { min: 0, max: 0, hint: '', fn: v => caseMap(stringify(v), true) },
        lower: { min: 0, max: 0, hint: '', fn: v => caseMap(stringify(v), false) },
        trim: { min: 0, max: 0, hint: '', fn: v => stringify(v).trim() },
        join: { min: 0, max: 1, hint: '(", ")', fn: (v, a) => Array.isArray(v) ? v.map(stringify).join(strArg(a, 0, ', ')) : stringify(v) },
        split: { min: 1, max: 1, hint: '(",")', fn: (v, a) => stringify(v).split(strArg(a, 0, ',')) },
        first: { min: 0, max: 0, hint: '', fn: v => Array.isArray(v) ? (v.length ? v[0] : null) : (typeof v === 'string' ? Array.from(v)[0] || '' : null) },
        last: { min: 0, max: 0, hint: '', fn: v => Array.isArray(v) ? (v.length ? v[v.length - 1] : null) : (typeof v === 'string' ? Array.from(v).pop() || '' : null) },
        count: { min: 0, max: 0, hint: '', fn: v => v === null || v === undefined ? 0 : Array.isArray(v) ? v.length : typeof v === 'object' ? Object.keys(v).length : typeof v === 'string' ? Array.from(v).length : 1 },
        pluck: { min: 1, max: 1, hint: '("title")', fn: (v, a) => { const f = strArg(a, 0, ''); return Array.isArray(v) ? v.map(item => item && typeof item === 'object' && !Array.isArray(item) ? item[f] : null) : []; } },
        json: { min: 0, max: 0, hint: '', fn: v => { try { return JSON.stringify(v === undefined ? null : v); } catch (err) { return ''; } } },
        date: { min: 1, max: 2, hint: '("DD.MM.YYYY")', fn: (v, a) => { const d = toDate(v); if (!d) throw new Error('not a date'); return formatDate(d, strArg(a, 0, ''), a.length > 1 ? strArg(a, 1, '') : undefined); } },
        round: { min: 0, max: 1, hint: '(2)', fn: (v, a) => roundHalfAway(v, intArg(a, 0, 0, 0)) },
        replace: { min: 2, max: 2, hint: '("a", "b")', fn: (v, a) => { const from = strArg(a, 0, ''); const to = strArg(a, 1, ''); const s = stringify(v); return from === '' ? s : s.split(from).join(to); } },
        strip_html: { min: 0, max: 0, hint: '', fn: v => stripHTML(v) }
    };

    function applyFilter(name, value, args) {
        const spec = FILTERS[name];
        if (!spec) throw new Error('unknown filter ' + name);
        if (args.length < spec.min || args.length > spec.max) throw new Error('wrong number of arguments for ' + name);
        return spec.fn(value, args);
    }

    function resolve(root, path, roots) {
        let value = Object.prototype.hasOwnProperty.call(roots, root) ? roots[root] : undefined;
        for (const seg of path) {
            if (value === null || value === undefined) return null;
            if (typeof seg === 'number') value = Array.isArray(value) ? value[seg] : undefined;
            else value = typeof value === 'object' && !Array.isArray(value) ? value[seg] : undefined;
        }
        return value === undefined ? null : value;
    }

    function evalExpr(expr, roots) {
        const p = parseExpr(expr);
        let value = resolve(p.root, p.path, roots);
        p.filters.forEach(f => { value = applyFilter(f.name, value, f.args); });
        return value;
    }

    // evaluate resolves a parameter value like the engine: a field that is exactly one
    // expression keeps its JSON type, mixed text becomes a string. Objects and lists recurse.
    function evaluate(value, roots) {
        if (typeof value !== 'string') {
            if (Array.isArray(value)) return value.map(v => evaluate(v, roots));
            if (value && typeof value === 'object') {
                const out = {};
                Object.keys(value).forEach(k => { out[k] = evaluate(value[k], roots); });
                return out;
            }
            return value;
        }
        const segs = segments(value);
        if (segs.length === 1 && segs[0].expr !== undefined) return evalExpr(segs[0].expr, roots || {});
        return segs.map(s => s.expr !== undefined ? stringify(evalExpr(s.expr, roots || {})) : s.literal).join('');
    }

    // describe summarises a preview value in one short, translated phrase.
    function describe(value, t, fmtNumber) {
        const n = fmtNumber || (x => String(x));
        if (value === null || value === undefined || value === '') return t('easydrag.ui.preview_empty');
        if (Array.isArray(value)) return t('easydrag.ui.preview_list', { count: n(value.length) });
        if (typeof value === 'object') return t('easydrag.ui.preview_object', { count: n(Object.keys(value).length) });
        if (typeof value === 'string') return t('easydrag.ui.preview_text', { count: n(Array.from(value).length) });
        return stringify(value);
    }

    const FILTER_LIST = Object.keys(FILTERS).sort().map(name => ({ name, hint: FILTERS[name].hint, args: FILTERS[name].min }));

    ED.template = { segments, parseExpr, refs, refsInValue, renameRoot, renameInValue, evaluate, stringify, describe, applyFilter, toDate, formatDate, FILTER_LIST };
})();
