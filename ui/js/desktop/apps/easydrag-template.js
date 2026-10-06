// EasyDrag templates in the browser: {{path | filter}} parsing, reference renaming and the
// live preview. The server (internal/flows/template*.go) stays authoritative; the preview
// mirrors its filters so the user sees the same result before a test run.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};

    const MAX_INDEX = 2147483647;
    const MAX_ECHO = 40;
    const MAX_OUTPUT_BYTES = 8 << 20;
    const MAX_SPLIT_PARTS = 100000;
    const MAX_PARAM_DEPTH = 32;

    // quoteForError quotes user input for a message like Go's quoteForError: at most 40
    // characters, then an ellipsis.
    function quoteForError(value) {
        const s = String(value);
        let i = 0;
        for (let n = 0; n < MAX_ECHO && i < s.length; n++) i += s.codePointAt(i) > 0xFFFF ? 2 : 1;
        return JSON.stringify(i < s.length ? s.slice(0, i) + '…' : s);
    }

    // utf8Len counts the UTF-8 bytes of s, the unit of Go's output limits.
    function utf8Len(s) {
        let n = 0;
        for (let i = 0; i < s.length; i++) {
            const c = s.charCodeAt(i);
            if (c < 0x80) n += 1;
            else if (c < 0x800) n += 2;
            else if (c >= 0xD800 && c <= 0xDBFF && i + 1 < s.length) { n += 4; i++; } else n += 3;
        }
        return n;
    }

    // setOwn adds a plain data property, so an own "__proto__" key stays data instead of
    // changing the prototype of the result.
    function setOwn(obj, key, value) {
        Object.defineProperty(obj, key, { value, writable: true, enumerable: true, configurable: true });
    }

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
    // names, numbers, quoted strings and the marks . [ ] | ( ) and the comma. Inside a string
    // a backslash keeps the character after it, except that \n and \t mean a newline and a
    // tab. Blanks between tokens are skipped.
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
                if (!isFinite(num)) throw new Error('invalid number ' + quoteForError(text));
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
        if (!Object.prototype.hasOwnProperty.call(FILTERS, call.name)) throw new Error('unknown filter ' + quoteForError(call.name));
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
        if (tok.kind !== 'eof') throw new Error('unexpected ' + quoteForError(tok.text));
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
            Object.keys(value).forEach(k => setOwn(out, k, walk(value[k], fn)));
            return out;
        }
        return value;
    }

    function refsInValue(value) {
        const out = [];
        walk(value, s => { refs(s).forEach(r => out.push(r)); return s; });
        return out;
    }

    const ROOT_PATTERN = /^(\s*)([A-Za-z_][A-Za-z0-9_]*)(?=[\s.\[|]|$)/;

    // renameRoots rewrites the root of every expression that map (a Map or a plain object of
    // old key to new key) names. Each root is replaced at most once, so a chain such as
    // a -> a_2 and a_2 -> a_2_2 cannot rename twice; all other text stays byte-identical.
    function renameRoots(text, map) {
        const s = String(text);
        if (s.indexOf('{{') < 0) return s;
        const lookup = typeof map.get === 'function'
            ? key => map.get(key)
            : key => (Object.prototype.hasOwnProperty.call(map, key) ? map[key] : undefined);
        let out = '';
        let last = 0;
        segments(s).forEach(seg => {
            if (seg.expr === undefined) return;
            const m = ROOT_PATTERN.exec(seg.expr);
            const next = m ? lookup(m[2]) : undefined;
            if (next === undefined || next === m[2]) return;
            out += s.slice(last, seg.start) + '{{' + m[1] + next + seg.expr.slice(m[0].length) + '}}';
            last = seg.end;
        });
        return out + s.slice(last);
    }

    // renameRoot rewrites every expression whose root is oldKey; all other text stays byte-identical.
    function renameRoot(text, oldKey, newKey) {
        return renameRoots(text, new Map([[oldKey, newKey]]));
    }

    function renameInValue(value, oldKey, newKey) {
        const map = new Map([[oldKey, newKey]]);
        return walk(value, s => renameRoots(s, map));
    }

    function renameRootsInValue(value, map) {
        return walk(value, s => renameRoots(s, map));
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
        if (typeof value === 'number') return String(value);
        if (typeof value === 'boolean') return value ? 'true' : 'false';
        try { return goJSON(value); } catch (err) { return String(value); }
    }

    function toNumber(value) {
        if (typeof value === 'number') return value;
        if (typeof value === 'string' && value.trim() !== '' && !isNaN(Number(value))) return Number(value);
        return NaN;
    }

    // The layouts of Go's toTime (internal/flows/values.go), checked against the engine:
    // RFC 3339 with an upper-case T and Z or a +hh:mm offset (up to 24:60), and the zone-less
    // forms with a T or a run of spaces before the time. The hour may have one digit, and a
    // fraction (after "." or ",") may follow the seconds.
    const RFC3339_DATE = /^(\d{4})-(\d{2})-(\d{2})T(\d{1,2}):(\d{2}):(\d{2})(?:[.,](\d+))?(?:(Z)|([+-])(\d{2}):(\d{2}))$/;
    const LOCAL_DATE = /^(\d{4})-(\d{2})-(\d{2})(?:(?:T| +)(\d{1,2}):(\d{2})(?::(\d{2})(?:[.,](\d+))?)?)?$/;

    // parseGoDate parses s like Go's time.Parse with those layouts and returns null for every
    // other form. Impossible fields ("2026-02-30", hour 24) are errors, not rolled over: the
    // date must round-trip through a UTC Date, which has no DST gaps.
    function parseGoDate(s) {
        const zoned = RFC3339_DATE.exec(s);
        const m = zoned || LOCAL_DATE.exec(s);
        if (!m) return null;
        const [y, mo, d, h, mi, sec] = [1, 2, 3, 4, 5, 6].map(i => +(m[i] || 0));
        const ms = m[7] ? +m[7].slice(0, 3).padEnd(3, '0') : 0;
        const utc = new Date(0);
        utc.setUTCFullYear(y, mo - 1, d);
        if (utc.getUTCFullYear() !== y || utc.getUTCMonth() !== mo - 1 || utc.getUTCDate() !== d) return null;
        if (h > 23 || mi > 59 || sec > 59) return null;
        if (zoned) {
            if (!m[8] && (+m[10] > 24 || +m[11] > 60)) return null;
            const offset = m[8] ? 0 : (m[9] === '-' ? -1 : 1) * (+m[10] * 60 + +m[11]);
            utc.setUTCHours(h, mi, sec, ms);
            return new Date(utc.getTime() - offset * 60000);
        }
        const local = new Date(2000, 0, 1);
        local.setFullYear(y, mo - 1, d);
        local.setHours(h, mi, sec, ms);
        return local;
    }

    function toDate(value) {
        if (value instanceof Date) return value;
        if (typeof value === 'number') {
            const date = new Date(Math.abs(value) > 1e12 ? value : value * 1000);
            return isNaN(date.getTime()) ? null : date;
        }
        return typeof value === 'string' ? parseGoDate(value.trim()) : null;
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
        const tokens = { YYYY: String(p.year).padStart(4, '0'), MM: p.month, DD: p.day, HH: p.hour, mm: p.minute, ss: p.second };
        return String(pattern).replace(/YYYY|MM|DD|HH|mm|ss/g, tok => tokens[tok]);
    }

    // The only named entities strip_html decodes (see the known differences above FILTERS).
    const NAMED_ENTITIES = { amp: '&', lt: '<', gt: '>', quot: '"', apos: "'", nbsp: ' ' };

    function decodeEntities(s) {
        return s.replace(/&(?:#(\d+)|#[xX]([0-9A-Fa-f]+)|(amp|lt|gt|quot|apos|nbsp));/g, (m, dec, hex, name) => {
            if (name) return NAMED_ENTITIES[name];
            const cp = dec !== undefined ? parseInt(dec, 10) : parseInt(hex, 16);
            return cp > 0 && cp <= 0x10FFFF && (cp < 0xD800 || cp > 0xDFFF) ? String.fromCodePoint(cp) : '\uFFFD';
        });
    }

    // strip_html mirrors filterStripHTML in internal/flows/filters.go. Five passes run one
    // after the other, like its five patterns: script blocks, style blocks, comments,
    // declarations and tags (a letter must follow "<") each become a space; then entities are
    // decoded and whitespace collapses to single spaces. The passes are indexOf scanners, not
    // regular expressions, so hostile input stays linear: once the terminator a match needs
    // is missing after some position, no later start can match either, and the pass stops.
    function isLetter(c) { return (c >= 65 && c <= 90) || (c >= 97 && c <= 122); }
    function isWordChar(c) { return isLetter(c) || (c >= 48 && c <= 57) || c === 95; }
    function isTagNameChar(c) { return isWordChar(c) || c === 58 || c === 45; }
    function isRE2Space(c) { return c === 9 || c === 10 || c === 12 || c === 13 || c === 32; }

    // gtFinder returns the first ">" at or after a position; positions must not decrease.
    function gtFinder(text) {
        let gt = -2;
        return pos => {
            if (gt !== -1 && gt < pos) gt = text.indexOf('>', pos);
            return gt;
        };
    }

    // stripBlocks: (?is)<name\b[^>]*>.*?</name\s*> with ASCII case folding.
    function stripBlocks(text, name) {
        const lower = text.replace(/[A-Z]+/g, m => m.toLowerCase());
        const open = '<' + name;
        const close = '</' + name;
        let out = '';
        let last = 0;
        let from = 0;
        for (;;) {
            const i = lower.indexOf(open, from);
            if (i < 0) break;
            const after = i + open.length;
            if (after < text.length && isWordChar(text.charCodeAt(after))) { from = i + 1; continue; }
            const gt = text.indexOf('>', after);
            if (gt < 0) break;
            let end = -1;
            for (let c = lower.indexOf(close, gt + 1); c >= 0 && end < 0; c = lower.indexOf(close, c + 1)) {
                let k = c + close.length;
                while (k < text.length && isRE2Space(text.charCodeAt(k))) k++;
                if (text.charCodeAt(k) === 62) end = k + 1;
            }
            if (end < 0) break;
            out += text.slice(last, i) + ' ';
            last = from = end;
        }
        return last ? out + text.slice(last) : text;
    }

    // stripComments: (?s)<!--.*?-->
    function stripComments(text) {
        let out = '';
        let last = 0;
        for (let i = text.indexOf('<!--'); i >= 0; i = text.indexOf('<!--', last)) {
            const end = text.indexOf('-->', i + 4);
            if (end < 0) break;
            out += text.slice(last, i) + ' ';
            last = end + 3;
        }
        return last ? out + text.slice(last) : text;
    }

    // stripMarkup runs one of the last two passes over every "<":
    // declarations (?i)<![A-Za-z\[][^>]*>|<\?[^>]*\?> or tags </?[A-Za-z][A-Za-z0-9:_-]*(?:\s[^>]*)?/?>
    function stripMarkup(text, matchAt) {
        const nextGt = gtFinder(text);
        let out = '';
        let last = 0;
        let i = text.indexOf('<');
        while (i >= 0) {
            const end = matchAt(text, i, nextGt);
            if (end === null) break;
            if (end > 0) {
                out += text.slice(last, i) + ' ';
                last = end;
                i = text.indexOf('<', end);
            } else {
                i = text.indexOf('<', i + 1);
            }
        }
        return last ? out + text.slice(last) : text;
    }

    // Each matcher returns the end of a match at i, 0 for no match, or null when no later
    // start can match because no ">" is left.
    function declarationAt(text, i, nextGt) {
        const c1 = text.charCodeAt(i + 1);
        if (c1 === 33) {
            const c2 = text.charCodeAt(i + 2);
            if (!isLetter(c2) && c2 !== 91) return 0;
            const gt = nextGt(i + 3);
            return gt < 0 ? null : gt + 1;
        }
        if (c1 !== 63) return 0;
        const gt = nextGt(i + 2);
        if (gt < 0) return null;
        return gt >= i + 3 && text.charCodeAt(gt - 1) === 63 ? gt + 1 : 0;
    }

    function tagAt(text, i, nextGt) {
        let p = i + 1;
        if (text.charCodeAt(p) === 47) p++;
        if (!isLetter(text.charCodeAt(p))) return 0;
        let q = p + 1;
        while (q < text.length && isTagNameChar(text.charCodeAt(q))) q++;
        const c = text.charCodeAt(q);
        if (isRE2Space(c)) {
            const gt = nextGt(q + 1);
            return gt < 0 ? null : gt + 1;
        }
        if (c === 62) return q + 1;
        return c === 47 && text.charCodeAt(q + 1) === 62 ? q + 2 : 0;
    }
    const GO_SPACE = /[\t\n\v\f\r \u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]+/;

    function stripHTML(value) {
        let text = stringify(value);
        text = stripBlocks(text, 'script');
        text = stripBlocks(text, 'style');
        text = stripComments(text);
        text = stripMarkup(text, declarationAt);
        text = stripMarkup(text, tagAt);
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
        if (Math.abs(n) > MAX_INDEX) throw new Error('argument too large');
        if (n < min) throw new Error('argument must be at least ' + min);
        return n;
    }

    function tooLarge(name) {
        return new Error(name + ' result would exceed ' + MAX_OUTPUT_BYTES + ' bytes');
    }

    // joinList mirrors filterJoin: the separators and then each part count against the 8 MiB cap.
    function joinList(value, args) {
        const sep = strArg(args, 0, ', ');
        if (!Array.isArray(value)) return stringify(value);
        let total = value.length > 1 ? (value.length - 1) * utf8Len(sep) : 0;
        if (total > MAX_OUTPUT_BYTES) throw tooLarge('join');
        return value.map(item => {
            const part = stringify(item);
            total += utf8Len(part);
            if (total > MAX_OUTPUT_BYTES) throw tooLarge('join');
            return part;
        }).join(sep);
    }

    function replaceAll(value, args) {
        const from = strArg(args, 0, '');
        const to = strArg(args, 1, '');
        const s = stringify(value);
        if (from === '') return s;
        const parts = s.split(from);
        const n = parts.length - 1;
        if (n > 0 && utf8Len(s) + n * (utf8Len(to) - utf8Len(from)) > MAX_OUTPUT_BYTES) throw tooLarge('replace');
        return parts.join(to);
    }

    function splitText(value, args) {
        const parts = stringify(value).split(strArg(args, 0, ','), MAX_SPLIT_PARTS + 1);
        if (parts.length > MAX_SPLIT_PARTS) throw new Error('split would produce more than ' + MAX_SPLIT_PARTS + ' parts');
        return parts;
    }

    // Known differences from the Go engine, which stays authoritative:
    // - date: previews read zone-less dates and show results in the browser's time zone; the
    //   server uses the flow's zone.
    // - Numbers from 1e21 up and below 1e-6 print with an exponent in text; Go writes them out in full.
    // - trim uses JavaScript's whitespace set, which differs from Go's strings.TrimSpace in U+0085 and U+FEFF.
    // - split("") splits UTF-16 code units, so it breaks emoji apart; Go splits into whole characters.
    // - strip_html decodes numeric entities and only the named ones amp, lt, gt, quot, apos and
    //   nbsp. Go's html.UnescapeString also decodes all HTML5 names, the forms without ";", the
    //   windows-1252 mapping for &#128;-&#159;, and turns "&#x;" into U+FFFD.
    // - Exotic cases: upper on polytonic Greek, round on strings like "0x10" or "1_0", and long s
    //   (U+017F) or Kelvin sign (U+212A) case folding in strip_html.

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
        join: { min: 0, max: 1, hint: '(", ")', fn: joinList },
        split: { min: 1, max: 1, hint: '(",")', fn: splitText },
        first: { min: 0, max: 0, hint: '', fn: v => Array.isArray(v) ? (v.length ? v[0] : null) : (typeof v === 'string' ? Array.from(v)[0] || '' : null) },
        last: { min: 0, max: 0, hint: '', fn: v => Array.isArray(v) ? (v.length ? v[v.length - 1] : null) : (typeof v === 'string' ? Array.from(v).pop() || '' : null) },
        count: { min: 0, max: 0, hint: '', fn: v => v === null || v === undefined ? 0 : Array.isArray(v) ? v.length : typeof v === 'object' ? Object.keys(v).length : typeof v === 'string' ? Array.from(v).length : 1 },
        pluck: { min: 1, max: 1, hint: '("title")', fn: (v, a) => { const f = strArg(a, 0, ''); return Array.isArray(v) ? v.map(item => item && typeof item === 'object' && !Array.isArray(item) && Object.prototype.hasOwnProperty.call(item, f) ? item[f] : null) : []; } },
        json: { min: 0, max: 0, hint: '', fn: v => { try { return goJSON(v); } catch (err) { throw new Error('value cannot be serialized as JSON'); } } },
        date: { min: 1, max: 2, hint: '("DD.MM.YYYY")', fn: (v, a) => { const d = toDate(v); if (!d) throw new Error('not a date'); return formatDate(d, strArg(a, 0, ''), a.length > 1 ? strArg(a, 1, '') : undefined); } },
        round: { min: 0, max: 1, hint: '(2)', fn: (v, a) => roundHalfAway(v, intArg(a, 0, 0, 0)) },
        replace: { min: 2, max: 2, hint: '("a", "b")', fn: replaceAll },
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

    // resolvePath follows path (field names and indexes) from value like the engine: negative
    // indexes count from the end, only own fields are read, and anything missing is null.
    function resolvePath(value, path) {
        let v = value;
        for (const seg of path || []) {
            if (v === null || v === undefined) return null;
            v = stepInto(v, seg);
        }
        return v === undefined ? null : v;
    }

    function resolve(root, path, roots) {
        return resolvePath(Object.prototype.hasOwnProperty.call(roots, root) ? roots[root] : undefined, path);
    }

    function evalParsed(p, roots) {
        let value = resolve(p.root, p.path, roots);
        p.filters.forEach(f => { value = applyFilter(f.name, value, f.args); });
        return value;
    }

    // evaluate resolves a parameter value like the engine (ResolveParams): a field that is
    // exactly one expression keeps its JSON type, mixed text becomes a string of at most
    // 8 MiB, and objects and lists recurse. The parameter map counts as level 1, so
    // containers in the value may nest down to level 32.
    function evaluate(value, roots) {
        return evalValue(value, roots || {}, 2);
    }

    function evalValue(value, roots, depth) {
        if (typeof value !== 'string') {
            if (!value || typeof value !== 'object') return value;
            if (depth > MAX_PARAM_DEPTH) throw new Error('parameters nested deeper than ' + MAX_PARAM_DEPTH + ' levels');
            if (Array.isArray(value)) return value.map(v => evalValue(v, roots, depth + 1));
            const out = {};
            Object.keys(value).forEach(k => setOwn(out, k, evalValue(value[k], roots, depth + 1)));
            return out;
        }
        if (value.indexOf('{{') < 0) return value;
        // Like ParseTemplate, an unclosed "{{" or any broken expression fails the whole text.
        const scan = scanText(value);
        if (scan.unclosed >= 0) throw new Error('unclosed {{');
        const segs = scan.segs.map(s => s.expr !== undefined ? { parsed: parseExpr(s.expr) } : s);
        if (segs.length === 1 && segs[0].parsed) return evalParsed(segs[0].parsed, roots);
        let size = 0;
        return segs.map(s => {
            const part = s.parsed ? stringify(evalParsed(s.parsed, roots)) : s.literal;
            size += utf8Len(part);
            if (size > MAX_OUTPUT_BYTES) throw new Error('template output exceeds ' + (MAX_OUTPUT_BYTES >> 20) + ' MiB');
            return part;
        }).join('');
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

    ED.template = {
        segments, parseExpr, refs, refsInValue, renameRoot, renameRoots, renameInValue, renameRootsInValue,
        evaluate, resolvePath, stringify, describe, applyFilter, toDate, formatDate, FILTER_LIST
    };
})();
