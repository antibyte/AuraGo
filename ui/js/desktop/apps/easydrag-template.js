// EasyDrag templates in the browser: {{path | filter}} parsing, reference renaming and the
// live preview. The server (internal/flows/template*.go) stays authoritative; the preview
// mirrors its filters so the user sees the same result before a test run.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};

    const MAX_INDEX = 2147483647;

    // scanText splits text into literals and expressions like ParseTemplate in
    // internal/flows/template.go: "\{{" is an escaped literal "{{", and an expression ends
    // at the first "}}" outside a quoted string. A "{{" without its "}}" stays literal text
    // in segs; unclosed is its offset (-1 when there is none), which refs and evaluate
    // report as an error like the server.
    function scanText(text) {
        const s = String(text == null ? '' : text);
        const segs = [];
        let literal = '';
        let unclosed = -1;
        let i = 0;
        while (i < s.length) {
            if (s[i] === '\\' && s.startsWith('{{', i + 1)) { literal += '{{'; i += 3; continue; }
            if (s.startsWith('{{', i)) {
                const end = exprEnd(s, i + 2);
                if (end < 0) { unclosed = i; literal += s.slice(i); break; }
                if (literal) { segs.push({ literal }); literal = ''; }
                segs.push({ expr: s.slice(i + 2, end), start: i, end: end + 2 });
                i = end + 2;
                continue;
            }
            literal += s[i];
            i += 1;
        }
        if (literal) segs.push({ literal });
        return { text: s, segs, unclosed };
    }

    // exprEnd mirrors findExprEnd: the offset of the closing "}}", skipping quoted strings.
    function exprEnd(s, from) {
        let quote = '';
        for (let i = from; i < s.length; i++) {
            const c = s[i];
            if (quote) {
                if (c === '\\') { i++; continue; }
                if (c === quote) quote = '';
                continue;
            }
            if (c === '"' || c === "'") { quote = c; continue; }
            if (c === '}' && s[i + 1] === '}') return i;
        }
        return -1;
    }

    // segments splits text into literals and expressions; "\{{" is an escaped literal "{{".
    function segments(text) {
        return scanText(text).segs;
    }

    // lexExpr splits an expression into tokens like exprLexer in internal/flows/template.go:
    // names, numbers, quoted strings (with \n, \t and \x escapes) and the marks . [ ] | ( ) ,
    // Blanks between tokens are skipped.
    function lexExpr(src) {
        const tokens = [];
        let i = 0;
        for (;;) {
            while (i < src.length && (src[i] === ' ' || src[i] === '\t' || src[i] === '\n' || src[i] === '\r')) i++;
            if (i >= src.length) { tokens.push({ kind: 'eof', text: '' }); return tokens; }
            const start = i;
            const c = src[i];
            if (/[A-Za-z_]/.test(c)) {
                while (i < src.length && /[A-Za-z0-9_]/.test(src[i])) i++;
                tokens.push({ kind: 'ident', text: src.slice(start, i) });
            } else if (/[0-9]/.test(c) || (c === '-' && /[0-9]/.test(src[i + 1] || ''))) {
                i++;
                while (i < src.length && /[0-9.]/.test(src[i])) i++;
                const text = src.slice(start, i);
                const num = Number(text);
                if (!isFinite(num)) throw new Error('invalid number ' + JSON.stringify(text));
                tokens.push({ kind: 'number', text, num });
            } else if (c === '"' || c === "'") {
                i++;
                let str = '';
                let closed = false;
                while (i < src.length) {
                    const ch = src[i];
                    if (ch === '\\' && i + 1 < src.length) {
                        const next = src[i + 1];
                        str += next === 'n' ? '\n' : next === 't' ? '\t' : next;
                        i += 2;
                        continue;
                    }
                    i++;
                    if (ch === c) { closed = true; break; }
                    str += ch;
                }
                if (!closed) throw new Error('unterminated string');
                tokens.push({ kind: 'string', text: str });
            } else if ('.[]|(),'.includes(c)) {
                i++;
                tokens.push({ kind: 'punct', text: c });
            } else {
                throw new Error('unexpected character ' + JSON.stringify(String.fromCodePoint(src.codePointAt(i))));
            }
        }
    }

    function literalValue(tok) {
        if (tok.kind === 'string') return tok.text;
        if (tok.kind === 'number') return tok.num;
        if (tok.kind === 'ident' && tok.text === 'true') return true;
        if (tok.kind === 'ident' && tok.text === 'false') return false;
        if (tok.kind === 'ident' && tok.text === 'null') return null;
        throw new Error('expected a string, number, true, false or null');
    }

    // checkFilterCall mirrors checkFilterCall in internal/flows/filters.go.
    function checkFilterCall(call) {
        if (!Object.prototype.hasOwnProperty.call(FILTERS, call.name)) throw new Error('unknown filter ' + JSON.stringify(call.name));
        const spec = FILTERS[call.name];
        if (call.args.length < spec.min || call.args.length > spec.max) {
            throw new Error(spec.min === spec.max
                ? 'filter ' + call.name + ' takes ' + spec.min + ' argument(s)'
                : 'filter ' + call.name + ' takes ' + spec.min + ' to ' + spec.max + ' arguments');
        }
    }

    // parseExpr parses "root.a[0]["b"] | filter(1, "x")" like parseExpr in
    // internal/flows/template.go, including the check of filter names and argument counts.
    function parseExpr(expr) {
        const tokens = lexExpr(String(expr));
        let pos = 0;
        let tok = tokens[0];
        const advance = () => { pos += 1; tok = tokens[pos]; };
        const isPunct = text => tok.kind === 'punct' && tok.text === text;
        if (tok.kind !== 'ident') throw new Error('expected a name');
        const root = tok.text;
        const path = [];
        advance();
        for (;;) {
            if (isPunct('.')) {
                advance();
                if (tok.kind !== 'ident') throw new Error("expected a field name after '.'");
                path.push(tok.text);
                advance();
                continue;
            }
            if (isPunct('[')) {
                advance();
                if (tok.kind === 'number') {
                    if (!Number.isInteger(tok.num)) throw new Error('index must be a whole number');
                    if (Math.abs(tok.num) > MAX_INDEX) throw new Error('index too large');
                    path.push(tok.num);
                } else if (tok.kind === 'string') {
                    path.push(tok.text);
                } else {
                    throw new Error('expected a number or string inside [ ]');
                }
                advance();
                if (!isPunct(']')) throw new Error("expected ']'");
                advance();
                continue;
            }
            break;
        }
        const filters = [];
        while (isPunct('|')) {
            advance();
            if (tok.kind !== 'ident') throw new Error("expected a filter name after '|'");
            const call = { name: tok.text, args: [] };
            advance();
            if (isPunct('(')) {
                advance();
                while (!isPunct(')')) {
                    call.args.push(literalValue(tok));
                    advance();
                    if (isPunct(',')) {
                        advance();
                        if (isPunct(')')) throw new Error("expected an argument after ','");
                        continue;
                    }
                    if (!isPunct(')')) throw new Error("expected ',' or ')'");
                }
                advance();
            }
            checkFilterCall(call);
            filters.push(call);
        }
        if (tok.kind !== 'eof') throw new Error('unexpected ' + JSON.stringify(tok.text));
        return { root, path, filters };
    }

    // refs lists the expressions of a text with their root and path; broken ones carry error.
    function refs(text) {
        const scan = scanText(text);
        const out = scan.segs.filter(s => s.expr !== undefined).map(s => {
            try {
                const p = parseExpr(s.expr);
                return { root: p.root, path: p.path, filters: p.filters, expr: s.expr, start: s.start, end: s.end };
            } catch (err) {
                return { error: err.message, expr: s.expr, start: s.start, end: s.end };
            }
        });
        if (scan.unclosed >= 0) out.push({ error: 'unclosed {{', expr: scan.text.slice(scan.unclosed + 2), start: scan.unclosed, end: scan.text.length });
        return out;
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

    // byCodePoint orders keys like Go's sort.Strings (UTF-8 byte order): UTF-16 order
    // except that an astral character (surrogate) sorts after every other character.
    function byCodePoint(a, b) {
        for (let i = 0; i < a.length && i < b.length; i++) {
            const x = a.charCodeAt(i);
            const y = b.charCodeAt(i);
            if (x === y) continue;
            const sx = x >= 0xD800 && x <= 0xDFFF;
            const sy = y >= 0xD800 && y <= 0xDFFF;
            if (sx !== sy) return sx ? 1 : -1;
            return x - y;
        }
        return a.length - b.length;
    }

    // Go's encoder escapes the line and paragraph separators (U+2028, U+2029); JSON.stringify does not.
    const LINE_SEPARATORS = new RegExp('[' + String.fromCharCode(0x2028, 0x2029) + ']', 'g');

    function jsonString(s) {
        return JSON.stringify(s).replace(LINE_SEPARATORS, ch => '\\' + 'u' + ch.charCodeAt(0).toString(16));
    }

    // goJSON encodes lists and objects like Go's encoding/json (internal/flows/values.go
    // compactJSON): compact, no HTML escaping and object keys sorted.
    function goJSON(value) {
        if (value === null || value === undefined) return 'null';
        if (Array.isArray(value)) return '[' + value.map(goJSON).join(',') + ']';
        if (typeof value === 'object') {
            return '{' + Object.keys(value).filter(k => value[k] !== undefined).sort(byCodePoint)
                .map(k => jsonString(k) + ':' + goJSON(value[k])).join(',') + '}';
        }
        return typeof value === 'string' ? jsonString(value) : JSON.stringify(value);
    }

    function stringify(value) {
        if (value === null || value === undefined) return '';
        if (typeof value === 'string') return value;
        // Numbers from 1e21 up and below 1e-6 print with an exponent here; Go's Stringify writes them out in full.
        if (typeof value === 'number') return Number.isInteger(value) ? String(value) : String(value);
        if (typeof value === 'boolean') return value ? 'true' : 'false';
        try { return goJSON(value); } catch (err) { return String(value); }
    }

    function toNumber(value) {
        if (typeof value === 'number') return value;
        if (typeof value === 'string' && value.trim() !== '' && !isNaN(Number(value))) return Number(value);
        return NaN;
    }

    // validCalendar rejects impossible fields like Go's time.Parse ("2026-02-30", hour 24)
    // instead of letting Date roll them over: the date must round-trip through a UTC Date
    // (UTC has no DST gaps) and the time fields must be in range.
    function validCalendar(s) {
        const m = /^(\d{4})-(\d{2})-(\d{2})(?:[Tt ](\d{2}):(\d{2})(?::(\d{2}))?)?/.exec(s);
        if (!m) return true;
        const check = new Date(Date.UTC(2000, 0, 1));
        check.setUTCFullYear(+m[1], +m[2] - 1, +m[3]);
        return check.getUTCFullYear() === +m[1] && check.getUTCMonth() === +m[2] - 1 && check.getUTCDate() === +m[3]
            && +(m[4] || 0) <= 23 && +(m[5] || 0) <= 59 && +(m[6] || 0) <= 59;
    }

    function toDate(value) {
        if (value instanceof Date) return value;
        if (typeof value === 'number') return new Date(value > 1e12 ? value : value * 1000);
        const s = String(value || '').trim();
        if (!s || !validCalendar(s)) return null;
        const local = /^(\d{4})-(\d{2})-(\d{2})(?:[ T](\d{2}):(\d{2})(?::(\d{2}))?)?$/.exec(s);
        const date = local ? new Date(+local[1], +local[2] - 1, +local[3], +(local[4] || 0), +(local[5] || 0), +(local[6] || 0)) : new Date(s);
        return isNaN(date.getTime()) ? null : date;
    }

    // Previews read zone-less dates and show results in the browser's time zone; the server uses the flow's zone.
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

    // strip_html decodes numeric entities and only these named ones: amp, lt, gt, quot, apos, nbsp.
    // Go's html.UnescapeString also decodes all HTML5 names, the forms without ";", the
    // windows-1252 mapping for &#128;-&#159;, and turns "&#x;" into U+FFFD.
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
        // trim uses JavaScript's whitespace set, which differs from Go's strings.TrimSpace in U+0085 and U+FEFF.
        trim: { min: 0, max: 0, hint: '', fn: v => stringify(v).trim() },
        join: { min: 0, max: 1, hint: '(", ")', fn: (v, a) => Array.isArray(v) ? v.map(stringify).join(strArg(a, 0, ', ')) : stringify(v) },
        // Other known, exotic differences: upper on polytonic Greek, truncate beyond 2^31, round on strings like "0x10" or "1_0", and long s (U+017F) or Kelvin sign (U+212A) case folding in strip_html.
        // split("") splits UTF-16 code units, so it breaks emoji apart; Go splits into whole characters.
        split: { min: 1, max: 1, hint: '(",")', fn: (v, a) => stringify(v).split(strArg(a, 0, ',')) },
        first: { min: 0, max: 0, hint: '', fn: v => Array.isArray(v) ? (v.length ? v[0] : null) : (typeof v === 'string' ? Array.from(v)[0] || '' : null) },
        last: { min: 0, max: 0, hint: '', fn: v => Array.isArray(v) ? (v.length ? v[v.length - 1] : null) : (typeof v === 'string' ? Array.from(v).pop() || '' : null) },
        count: { min: 0, max: 0, hint: '', fn: v => v === null || v === undefined ? 0 : Array.isArray(v) ? v.length : typeof v === 'object' ? Object.keys(v).length : typeof v === 'string' ? Array.from(v).length : 1 },
        pluck: { min: 1, max: 1, hint: '("title")', fn: (v, a) => { const f = strArg(a, 0, ''); return Array.isArray(v) ? v.map(item => item && typeof item === 'object' && !Array.isArray(item) ? item[f] : null) : []; } },
        json: { min: 0, max: 0, hint: '', fn: v => { try { return goJSON(v); } catch (err) { throw new Error('value cannot be serialized as JSON'); } } },
        date: { min: 1, max: 2, hint: '("DD.MM.YYYY")', fn: (v, a) => { const d = toDate(v); if (!d) throw new Error('not a date'); return formatDate(d, strArg(a, 0, ''), a.length > 1 ? strArg(a, 1, '') : undefined); } },
        round: { min: 0, max: 1, hint: '(2)', fn: (v, a) => roundHalfAway(v, intArg(a, 0, 0, 0)) },
        replace: { min: 2, max: 2, hint: '("a", "b")', fn: (v, a) => { const from = strArg(a, 0, ''); const to = strArg(a, 1, ''); const s = stringify(v); return from === '' ? s : s.split(from).join(to); } },
        strip_html: { min: 0, max: 0, hint: '', fn: v => stripHTML(v) }
    };

    function applyFilter(name, value, args) {
        checkFilterCall({ name, args });
        return FILTERS[name].fn(value, args);
    }

    // stepInto mirrors stepInto in internal/flows/template_eval.go: a negative index counts
    // from the end, and only own fields of objects are read.
    function stepInto(value, seg) {
        if (typeof seg === 'number') {
            if (!Array.isArray(value)) return undefined;
            const i = seg < 0 ? seg + value.length : seg;
            return i >= 0 && i < value.length ? value[i] : undefined;
        }
        if (!value || typeof value !== 'object' || Array.isArray(value)) return undefined;
        return Object.prototype.hasOwnProperty.call(value, seg) ? value[seg] : undefined;
    }

    function resolve(root, path, roots) {
        let value = Object.prototype.hasOwnProperty.call(roots, root) ? roots[root] : undefined;
        for (const seg of path) {
            if (value === null || value === undefined) return null;
            value = stepInto(value, seg);
        }
        return value === undefined ? null : value;
    }

    function evalParsed(p, roots) {
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
        // Like ParseTemplate, an unclosed "{{" or any broken expression fails the whole text.
        const scan = scanText(value);
        if (scan.unclosed >= 0) throw new Error('unclosed {{');
        const segs = scan.segs.map(s => s.expr !== undefined ? { parsed: parseExpr(s.expr) } : s);
        if (segs.length === 1 && segs[0].parsed) return evalParsed(segs[0].parsed, roots || {});
        return segs.map(s => s.parsed ? stringify(evalParsed(s.parsed, roots || {})) : s.literal).join('');
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
