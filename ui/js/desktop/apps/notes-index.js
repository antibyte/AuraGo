/* Client notes index: mirrors internal/desktop/notes_markdown.go and notes_resolve.go.
   Shared cases live in ui/testdata/notes-links-fixture.json. No DOM access at load time. */
(function (root, factory) {
    'use strict';
    var api = factory();
    if (typeof module === 'object' && module && module.exports) module.exports = api;
    if (root) root.NotesIndex = api;
})(typeof window !== 'undefined' ? window : null, function () {
    'use strict';

    var NOTES_ROOT_LOWER = 'documents/notes/';
    var MAX_WIKI_RUNES = 500;
    var URL_SCHEME = /^[a-zA-Z][a-zA-Z0-9+.\-]*:/;
    var ATTACH_EXT = { '.png': 1, '.jpg': 1, '.jpeg': 1, '.gif': 1, '.webp': 1, '.svg': 1, '.pdf': 1 };
    var TAG_RUNE = /^[\p{L}\p{N}_\-\/]$/u;
    var SPACE = /^\s$/u;

    function runeCount(s) { var n = 0; for (var _ of s) n++; return n; }
    function limitRunes(s, n) { var a = Array.from(s); return a.length > n ? a.slice(0, n).join('') : s; }

    // ---- path helpers (Go "path" semantics) ----
    function clean(p) {
        if (p === '') return '.';
        var rooted = p.charAt(0) === '/';
        var out = [];
        p.split('/').forEach(function (seg) {
            if (seg === '' || seg === '.') return;
            if (seg === '..') {
                if (out.length && out[out.length - 1] !== '..') out.pop();
                else if (!rooted) out.push('..');
                return;
            }
            out.push(seg);
        });
        var r = out.join('/');
        if (rooted) return '/' + r;
        return r === '' ? '.' : r;
    }
    function pathDir(p) { var i = p.lastIndexOf('/'); return clean(i < 0 ? '.' : p.slice(0, i + 1)); }
    function pathBase(p) {
        if (p === '') return '.';
        p = p.replace(/\/+$/, '');
        if (p === '') return '/';
        return p.slice(p.lastIndexOf('/') + 1);
    }
    function pathExt(p) {
        for (var i = p.length - 1; i >= 0 && p.charAt(i) !== '/'; i--) if (p.charAt(i) === '.') return p.slice(i);
        return '';
    }
    function pathJoin(a, b) { return clean(a + '/' + b); }

    // ---- frontmatter ----
    function splitFrontmatter(text) {
        var nl = text.indexOf('\n');
        if (nl < 0 || text.slice(0, nl).trim() !== '---') return null;
        var pos = nl + 1;
        while (pos <= text.length) {
            var end = text.indexOf('\n', pos);
            var lineEnd = end < 0 ? text.length : end;
            var next = end < 0 ? text.length : end + 1;
            if (text.slice(pos, lineEnd).trim() === '---') return { meta: text.slice(nl + 1, pos), bodyStart: next };
            if (end < 0) break;
            pos = next;
        }
        return null;
    }
    function unquote(s) {
        s = s.trim();
        if (s.length >= 2) {
            var q = s.charAt(0);
            if ((q === '"' || q === "'") && s.charAt(s.length - 1) === q) {
                s = s.slice(1, -1);
                return q === '"' ? s.replace(/\\(["\\])/g, '$1') : s.replace(/''/g, "'");
            }
        }
        return s;
    }
    function splitInlineList(s) {
        var items = [], cur = '', q = '';
        for (var i = 0; i < s.length; i++) {
            var c = s.charAt(i);
            if (q) { cur += c; if (c === q) q = ''; continue; }
            if (c === '"' || c === "'") { q = c; cur += c; continue; }
            if (c === ',') { items.push(cur); cur = ''; continue; }
            cur += c;
        }
        items.push(cur);
        return items.map(unquote);
    }
    // Minimal YAML subset: top-level "key: scalar", "key: [a, b]", and "key:" + "- item" blocks.
    function parseFrontmatter(meta) {
        var data = {};
        var lines = meta.split(/\r?\n/);
        for (var i = 0; i < lines.length; i++) {
            var m = /^([A-Za-z_][\w\-]*)\s*:\s*(.*)$/.exec(lines[i]);
            if (!m) continue;
            var key = m[1], rest = m[2].replace(/\s+#.*$/, '').trim();
            if (rest === '') {
                var list = [];
                while (i + 1 < lines.length && /^\s*-(\s|$)/.test(lines[i + 1])) {
                    i++;
                    list.push(unquote(lines[i].replace(/^\s*-\s*/, '').replace(/\s+#.*$/, '')));
                }
                data[key] = list.length ? list : null;
            } else if (rest.charAt(0) === '[' && rest.charAt(rest.length - 1) === ']') {
                data[key] = splitInlineList(rest.slice(1, -1));
            } else if (rest.charAt(0) === '#') {
                data[key] = null;
            } else {
                data[key] = unquote(rest);
            }
        }
        return data;
    }
    function toList(v, words) {
        var out = [];
        function add(s) { s = String(s).trim(); if (s) out.push(s); }
        function split(s) { return words ? s.split(/[,\s]+/) : s.split(','); }
        if (typeof v === 'string') split(v).forEach(add);
        else if (Array.isArray(v)) v.forEach(function (it) { if (words && typeof it === 'string') split(it).forEach(add); else add(it); });
        return out;
    }

    // ---- wiki / markdown link helpers ----
    function noteLinkURLOK(url) {
        if (!url || URL_SCHEME.test(url) || url.charAt(0) === '/' || url.charAt(0) === '#') return false;
        var p = url, i = p.search(/[#?]/);
        if (i >= 0) p = p.slice(0, i);
        var lower = p.toLowerCase();
        if (lower.slice(-3) === '.md') return true;
        var dot = lower.lastIndexOf('.');
        return dot >= 0 && !!ATTACH_EXT[lower.slice(dot)];
    }
    // Unescaped "|" or escaped "\|" (table cells) separates the alias.
    function wikiAliasSep(value) {
        for (var i = 0; i < value.length; i++) {
            var c = value.charAt(i);
            if (c === '\\' && value.charAt(i + 1) === '|') return { idx: i, width: 2 };
            if (c === '\\') i++;
            else if (c === '|') return { idx: i, width: 1 };
        }
        return { idx: -1, width: 0 };
    }
    function parseWikiValue(value) {
        value = String(value == null ? '' : value);
        var head = value, alias = '';
        var sep = wikiAliasSep(value);
        if (sep.idx >= 0) { head = value.slice(0, sep.idx); alias = value.slice(sep.idx + sep.width); }
        var hash = head.indexOf('#');
        if (hash >= 0) return { target: head.slice(0, hash), subpath: head.slice(hash + 1), alias: alias };
        return { target: head, subpath: '', alias: alias };
    }

    // ---- block scanner (mirrors internal/desktop/notes_scan.go step by step) ----
    var MAX_LINK_URL = 2048;
    var HTML_BLOCK_TAGS = {};
    'address article aside base basefont blockquote body caption center col colgroup dd details dialog dir div dl dt fieldset figcaption figure footer form frame frameset h1 h2 h3 h4 h5 h6 head header hr html iframe legend li link main menu menuitem nav noframes ol optgroup option p param search section summary table tbody td tfoot th thead title tr track ul'.split(' ').forEach(function (n) { HTML_BLOCK_TAGS[n] = 1; });
    var HTML_RAW_TAGS = { script: 1, pre: 1, style: 1, textarea: 1 };
    var H_BLANK = 1, H_COMMENT = 2, H_RAW = 3, H_PI = 4;

    function spanFinder(set) {
        var f = { valid: false, end: 0, from: 0, pos: -1 };
        f.find = function (s, start, end) {
            if (start >= end) return -1;
            if (f.valid && f.end === end && start >= f.from) {
                if (f.pos < 0) return -1;
                if (start <= f.pos) return f.pos;
            }
            var idx = -1;
            for (var k = start; k < end; k++) if (set.indexOf(s.charAt(k)) >= 0) { idx = k; break; }
            f.valid = true; f.end = end; f.from = start; f.pos = idx;
            return idx;
        };
        return f;
    }

    function backtickRuns(s) {
        var runs = {};
        for (var i = 0; i < s.length;) {
            if (s.charAt(i) !== '`') { i++; continue; }
            var j = i;
            while (j < s.length && s.charAt(j) === '`') j++;
            (runs[j - i] = runs[j - i] || []).push(i);
            i = j;
        }
        return runs;
    }
    function lowerBound(list, v) {
        var lo = 0, hi = list.length;
        while (lo < hi) { var mid = (lo + hi) >> 1; if (list[mid] < v) lo = mid + 1; else hi = mid; }
        return lo;
    }

    function scanParagraph(para, refs, addTag) {
        var n = para.length;
        var brackets = [], runs = null;
        var term = spanFinder(' \t)'), paren = spanFinder(')'), gt = spanFinder('>');
        var noCommentClose = false, lineEnd = -1, lineFrom = 0;
        function eol(from) {
            if (from >= lineFrom && from <= lineEnd) return lineEnd;
            var k = para.indexOf('\n', from);
            lineEnd = k < 0 ? n : k;
            lineFrom = from;
            return lineEnd;
        }
        function wikiSpan(open) {
            var vs = open + 2;
            for (var j = vs; j < n;) {
                if (j - vs > MAX_WIKI_RUNES * 2 + 2) return null;
                var c = para.charAt(j);
                if (c === '\n' || c === '[') return null;
                if (c === '\\') {
                    if (j + 1 >= n || para.charAt(j + 1) === '\n') return null;
                    j += 2;
                    continue;
                }
                if (c === ']') {
                    if (j + 1 >= n || para.charAt(j + 1) !== ']') return null;
                    var value = para.slice(vs, j);
                    var cnt = runeCount(value);
                    return cnt > 0 && cnt <= MAX_WIKI_RUNES && value.replace(/^[ \t]+|[ \t]+$/g, '') !== '' ? { vs: vs, ve: j } : null;
                }
                j++;
            }
            return null;
        }
        function linkURL(start) {
            var end = eol(start), vs, ve, closeFrom;
            if (start < end && para.charAt(start) === '<') {
                var k = gt.find(para, start + 1, end);
                if (k < 0) return null;
                vs = start + 1; ve = k; closeFrom = k + 1;
            } else {
                ve = term.find(para, start, end);
                if (ve < 0) ve = end;
                vs = start; closeFrom = ve;
            }
            if (ve - vs > MAX_LINK_URL) return null;
            var closeAt = paren.find(para, closeFrom, end);
            if (closeAt < 0) return null;
            return { vs: vs, ve: ve, after: closeAt + 1 };
        }

        for (var i = 0; i < n;) {
            var c = para.charAt(i);
            if (c === '\\') { i += 2; }
            else if (c === '`') {
                var cnt = 0;
                while (i + cnt < n && para.charAt(i + cnt) === '`') cnt++;
                if (!runs) runs = backtickRuns(para);
                var list = runs[cnt] || [];
                var kk = lowerBound(list, i + cnt);
                i = kk < list.length ? list[kk] + cnt : i + cnt;
            } else if (c === '<' && para.startsWith('<!--', i)) {
                if (!noCommentClose) {
                    var jc = para.indexOf('-->', i + 4);
                    if (jc >= 0) { i = jc + 3; continue; }
                    noCommentClose = true;
                }
                i++;
            } else if ((c === '[' && para.startsWith('[[', i)) || (c === '!' && para.startsWith('![[', i))) {
                var embed = c === '!', open = embed ? i + 1 : i;
                var span = wikiSpan(open);
                if (span) {
                    refs.links.push({ kind: 'wiki', value: para.slice(span.vs, span.ve), embed: embed });
                    i = span.ve + 2;
                } else { brackets.push(open); i = open + 1; }
            } else if (c === '[') { brackets.push(i); i++; }
            else if (c === ']') {
                var openAt = brackets.length ? brackets.pop() : -1;
                if (i + 1 >= n || para.charAt(i + 1) !== '(') { i++; continue; }
                var mu = linkURL(i + 2);
                if (!mu) { i += 2; continue; }
                var url = para.slice(mu.vs, mu.ve);
                if (noteLinkURLOK(url)) refs.links.push({ kind: 'markdown', value: url, embed: openAt > 0 && para.charAt(openAt - 1) === '!' });
                i = mu.after;
            } else if (c === '#') {
                var prevOK = i === 0;
                if (!prevOK) {
                    var prev = para.charAt(i - 1); // a trailing surrogate is never a space or "("
                    prevOK = SPACE.test(prev) || prev === '(';
                }
                var j = i + 1;
                if (prevOK) {
                    while (j < n) {
                        var cp = String.fromCodePoint(para.codePointAt(j));
                        if (!TAG_RUNE.test(cp)) break;
                        j += cp.length;
                    }
                    var value2 = para.slice(i + 1, j);
                    if (value2 !== '' && /[^\p{Nd}]/u.test(value2)) addTag(value2);
                }
                i = j;
            } else i++;
        }
    }

    function indentOf(s) {
        var cols = 0, bytes = 0;
        while (bytes < s.length) {
            var c = s.charAt(bytes);
            if (c === ' ') cols++;
            else if (c === '\t') cols += 4 - cols % 4;
            else break;
            bytes++;
        }
        return { cols: cols, bytes: bytes };
    }
    function stripCols(s, n) {
        var cols = 0, bytes = 0;
        while (bytes < s.length && cols < n) {
            var c = s.charAt(bytes);
            if (c === ' ') cols++;
            else if (c === '\t') cols += 4 - cols % 4;
            else return bytes;
            bytes++;
        }
        return bytes;
    }
    function stripQuote(s, off) {
        var ind = indentOf(s.slice(off));
        if (ind.cols >= 4 || off + ind.bytes >= s.length || s.charAt(off + ind.bytes) !== '>') return -1;
        off += ind.bytes + 1;
        if (off < s.length && (s.charAt(off) === ' ' || s.charAt(off) === '\t')) off++;
        return off;
    }
    function listMarker(s) {
        var ind = indentOf(s), nb = ind.bytes;
        if (ind.cols >= 4 || nb >= s.length) return null;
        var p = nb, c = s.charAt(p);
        if (c === '-' || c === '*' || c === '+') p++;
        else if (c >= '0' && c <= '9') {
            while (p < s.length && p - nb < 9 && s.charAt(p) >= '0' && s.charAt(p) <= '9') p++;
            if (p >= s.length || (s.charAt(p) !== '.' && s.charAt(p) !== ')')) return null;
            p++;
        } else return null;
        if (p < s.length && s.charAt(p) !== ' ' && s.charAt(p) !== '\t') return null;
        var markerCols = ind.cols + (p - nb);
        var sp = indentOf(s.slice(p));
        if (p + sp.bytes >= s.length) return { width: markerCols + 1, bytes: p + sp.bytes };
        if (sp.cols >= 5) sp = { cols: 1, bytes: 1 };
        return { width: markerCols + sp.cols, bytes: p + sp.bytes };
    }
    function continueContainers(line, ops) {
        var off = 0;
        for (var idx = 0; idx < ops.length; idx++) {
            var op = ops[idx];
            if (op.quote) {
                var next = stripQuote(line, off);
                if (next < 0) return -1;
                off = next;
                continue;
            }
            var rest = line.slice(off);
            if (rest.trim() === '') {
                for (var k = idx + 1; k < ops.length; k++) if (ops[k].quote) return -1;
                return line.length;
            }
            if (indentOf(rest).cols < op.indent) return -1;
            off += stripCols(rest, op.indent);
        }
        return off;
    }
    function fenceOpener(t) {
        if (t.length < 3 || (t.charAt(0) !== '`' && t.charAt(0) !== '~')) return null;
        var count = 0;
        while (count < t.length && t.charAt(count) === t.charAt(0)) count++;
        if (count < 3 || (t.charAt(0) === '`' && t.slice(count).indexOf('`') >= 0)) return null;
        return { char: t.charAt(0), count: count };
    }
    function htmlStart(t) {
        if (t.length < 2 || t.charAt(0) !== '<') return null;
        if (t.startsWith('<!--')) return { end: H_COMMENT, raw: '' };
        if (t.charAt(1) === '?') return { end: H_PI, raw: '' };
        var i = 1;
        if (t.charAt(i) === '/') i++;
        var j = i;
        while (j < t.length && /[A-Za-z0-9]/.test(t.charAt(j))) j++;
        if (j === i) return null;
        var name = t.slice(i, j).toLowerCase();
        var endsName = j >= t.length || t.charAt(j) === ' ' || t.charAt(j) === '\t' || t.charAt(j) === '>';
        if (HTML_RAW_TAGS[name] && i === 1 && endsName) return { end: H_RAW, raw: name };
        if (HTML_BLOCK_TAGS[name] && (endsName || t.startsWith('/>', j))) return { end: H_BLANK, raw: '' };
        return null;
    }
    function htmlCloses(end, raw, s) {
        if (end === H_COMMENT) return s.indexOf('-->') >= 0;
        if (end === H_PI) return s.indexOf('?>') >= 0;
        if (end === H_RAW) return s.toLowerCase().indexOf('</' + raw + '>') >= 0;
        return false;
    }
    function atxHeading(t) {
        var n = 0;
        while (n < t.length && t.charAt(n) === '#') n++;
        return n >= 1 && n <= 6 && (n === t.length || t.charAt(n) === ' ' || t.charAt(n) === '\t');
    }

    // ---- extraction ----
    function extract(content) {
        content = String(content == null ? '' : content);
        var refs = { title: '', aliases: [], tags: [], links: [] };
        var seen = {};
        function addTag(tag) {
            tag = tag.trim().replace(/^#+/, '').toLowerCase();
            if (tag && !seen[tag]) { seen[tag] = true; refs.tags.push(tag); }
        }
        var bodyStart = 0, fmTitle = '';
        var fm = splitFrontmatter(content);
        if (fm) {
            bodyStart = fm.bodyStart;
            var data = parseFrontmatter(fm.meta);
            if (typeof data.title === 'string') fmTitle = data.title.trim();
            ['aliases', 'alias'].forEach(function (k) { refs.aliases = refs.aliases.concat(toList(data[k], false)); });
            toList(data.tags, true).forEach(addTag);
        }

        var heading = '', firstLine = '', leaf = null, listCtx = 0;
        var para = { pieces: [], quote: 0 };
        function flush() {
            if (para.pieces.length) { scanParagraph(para.pieces.join('\n'), refs, addTag); para.pieces = []; }
        }
        for (var pos = bodyStart; pos < content.length;) {
            var nlAt = content.indexOf('\n', pos);
            var lineEnd = nlAt < 0 ? content.length : nlAt;
            var trimmed = content.slice(pos, lineEnd).replace(/\r+$/, '');
            pos = nlAt < 0 ? content.length : nlAt + 1;

            if (leaf) {
                var lo = continueContainers(trimmed, leaf.ops);
                if (lo >= 0) {
                    var lrest = trimmed.slice(lo);
                    if (leaf.kind === 'fence') {
                        var lt = lrest.trim();
                        if (lt.length >= leaf.count && lt.split(leaf.char).join('') === '') leaf = null;
                    } else if (leaf.end === H_BLANK) {
                        if (lrest.trim() === '') leaf = null;
                    } else if (htmlCloses(leaf.end, leaf.raw, lrest)) leaf = null;
                    continue;
                }
                leaf = null;
            }

            var off = 0, ops = [], quoteDepth = 0, q;
            while ((q = stripQuote(trimmed, off)) >= 0) { off = q; ops.push({ quote: true, indent: 0 }); quoteDepth++; }
            var consumed = 0, rest = trimmed.slice(off);
            if (listCtx > 0 && rest.trim() !== '') {
                if (indentOf(rest).cols >= listCtx) {
                    off += stripCols(rest, listCtx);
                    ops.push({ quote: false, indent: listCtx });
                    consumed = listCtx;
                } else if (!listMarker(rest) && !para.pieces.length) listCtx = 0;
            }
            var newItem = false;
            for (var guard = 0; guard < 8; guard++) {
                rest = trimmed.slice(off);
                var ind = indentOf(rest);
                if (ind.cols >= 4) break;
                if (ind.bytes < rest.length && rest.charAt(ind.bytes) === '>') {
                    off = stripQuote(trimmed, off);
                    ops.push({ quote: true, indent: 0 });
                    quoteDepth++;
                    continue;
                }
                var item = listMarker(rest);
                if (item) {
                    off += item.bytes;
                    consumed += item.width;
                    ops.push({ quote: false, indent: item.width });
                    listCtx = consumed;
                    newItem = true;
                    continue;
                }
                break;
            }
            if (newItem || (quoteDepth > 0 && quoteDepth !== para.quote)) flush();
            rest = trimmed.slice(off);
            var cur = indentOf(rest), t = rest.slice(cur.bytes);
            if (cur.bytes >= rest.length) { flush(); continue; }
            if (cur.cols < 4) {
                var fence = fenceOpener(t);
                if (fence) { flush(); leaf = { kind: 'fence', char: fence.char, count: fence.count, ops: ops }; continue; }
            }
            if (heading === '' && trimmed.indexOf('# ') === 0) heading = trimmed.slice(2).trim();
            if (firstLine === '') firstLine = trimmed.trim();
            if (cur.cols >= 4 && !para.pieces.length) continue;
            if (cur.cols < 4) {
                var html = htmlStart(t);
                if (html) {
                    flush();
                    var closed = false;
                    if (html.end === H_COMMENT) closed = htmlCloses(html.end, html.raw, t.slice(4));
                    else if (html.end === H_PI) closed = htmlCloses(html.end, html.raw, t.slice(2));
                    else if (html.end === H_RAW) closed = htmlCloses(html.end, html.raw, t);
                    if (!closed) leaf = { kind: 'html', end: html.end, raw: html.raw, ops: ops };
                    continue;
                }
                if (atxHeading(t) || t.charAt(0) === '|') {
                    flush();
                    para.quote = quoteDepth;
                    para.pieces.push(rest);
                    flush();
                    continue;
                }
            }
            if (!para.pieces.length) para.quote = quoteDepth;
            para.pieces.push(rest);
        }
        flush();

        if (fmTitle) refs.title = fmTitle;
        else if (heading) refs.title = heading;
        else if (firstLine) refs.title = firstLine;
        refs.title = limitRunes(refs.title, 160);
        return refs;
    }

    // ---- resolution ----
    function newSet(paths, suffix) {
        var set = { entries: [], byLower: {}, byBase: {}, suffix: suffix };
        (paths || []).forEach(function (p) {
            if (p && typeof p === 'object') p = p.path;
            p = String(p || '').replace(/\\/g, '/');
            var lower = p.toLowerCase();
            if (Object.prototype.hasOwnProperty.call(set.byLower, lower)) return;
            var base = pathBase(lower);
            if (suffix && base.slice(-suffix.length) === suffix) base = base.slice(0, -suffix.length);
            set.entries.push({ path: p, lower: lower, dir: pathDir(lower), base: base });
            var idx = set.entries.length - 1;
            set.byLower[lower] = idx;
            (set.byBase[base] = Object.prototype.hasOwnProperty.call(set.byBase, base) ? set.byBase[base] : []).push(idx);
        });
        return set;
    }
    function hasOwn(o, k) { return Object.prototype.hasOwnProperty.call(o, k); }
    function pick(set, cands) {
        var best = -1;
        (cands || []).forEach(function (c) {
            if (best < 0) { best = c; return; }
            var a = set.entries[c], b = set.entries[best];
            if (a.lower.length < b.lower.length || (a.lower.length === b.lower.length && a.lower < b.lower)) best = c;
        });
        return best < 0 ? '' : set.entries[best].path;
    }
    function find(set, from, name) {
        var n = name.toLowerCase();
        if (n.indexOf('/') >= 0) {
            var k = NOTES_ROOT_LOWER + n + set.suffix;
            if (hasOwn(set.byLower, k)) return set.entries[set.byLower[k]].path;
            var rel = pathJoin(pathDir(from), n).toLowerCase() + set.suffix;
            if (hasOwn(set.byLower, rel)) return set.entries[set.byLower[rel]].path;
            var tail = '/' + n + set.suffix, cands = [];
            set.entries.forEach(function (e, i) {
                if (e.lower.length >= tail.length && e.lower.slice(-tail.length) === tail) cands.push(i);
            });
            return pick(set, cands);
        }
        var list = hasOwn(set.byBase, n) ? set.byBase[n] : [];
        if (list.length > 1) {
            var fromDir = pathDir(from).toLowerCase();
            var same = list.filter(function (c) { return set.entries[c].dir === fromDir; });
            if (same.length) list = same;
        }
        return pick(set, list);
    }

    function createLookup(input) {
        input = input || {};
        var notes = newSet(input.notes, '.md');
        var atts = newSet(input.attachments, '');
        var byAlias = {};
        var aliases = input.aliases || {};
        Object.keys(aliases).sort().forEach(function (p) {
            var key = p.replace(/\\/g, '/').toLowerCase();
            if (!hasOwn(notes.byLower, key)) return;
            var idx = notes.byLower[key];
            (aliases[p] || []).forEach(function (alias) {
                var a = String(alias).trim().toLowerCase();
                if (!a) return;
                (byAlias[a] = hasOwn(byAlias, a) ? byAlias[a] : []).push(idx);
            });
        });

        function findNote(from, name) {
            var p = find(notes, from, name);
            if (p) return p;
            var a = name.toLowerCase();
            return pick(notes, hasOwn(byAlias, a) ? byAlias[a] : []);
        }
        function resolveMarkdown(from, value) {
            if (URL_SCHEME.test(value)) return '';
            var i = value.search(/[#?]/);
            if (i >= 0) value = value.slice(0, i);
            var decoded;
            try { decoded = decodeURIComponent(value); } catch (e) { return ''; }
            if (!decoded) return '';
            decoded = decoded.replace(/\\/g, '/');
            if (decoded.charAt(0) === '/') return '';
            var lower = pathJoin(pathDir(from), decoded).toLowerCase();
            if (lower.indexOf(NOTES_ROOT_LOWER) !== 0) return '';
            if (hasOwn(notes.byLower, lower)) return notes.entries[notes.byLower[lower]].path;
            if (hasOwn(atts.byLower, lower)) return atts.entries[atts.byLower[lower]].path;
            return '';
        }
        function resolve(from, kind, value, embed) {
            from = String(from || '').replace(/\\/g, '/');
            value = String(value == null ? '' : value);
            if (kind === 'markdown') return resolveMarkdown(from, value);
            var target = parseWikiValue(value).target.replace(/\\/g, '/').trim().replace(/^\.\//, '').replace(/^\/+/, '');
            if (target === '') return from;
            var ext = pathExt(target).toLowerCase();
            if (ext === '.md') return findNote(from, target.slice(0, -3));
            if (ext !== '') {
                var p = find(atts, from, target);
                if (p) return p;
            }
            return findNote(from, target);
        }
        function linkText(p) {
            p = String(p || '').replace(/\\/g, '/');
            var isNote = pathExt(p).toLowerCase() === '.md';
            var set = isNote ? notes : atts;
            var name = pathBase(p), rel = p;
            if (rel.length >= NOTES_ROOT_LOWER.length && rel.slice(0, NOTES_ROOT_LOWER.length).toLowerCase() === NOTES_ROOT_LOWER) {
                rel = rel.slice(NOTES_ROOT_LOWER.length);
            }
            if (isNote) { name = name.slice(0, -3); rel = rel.slice(0, -3); }
            var same = hasOwn(set.byBase, name.toLowerCase()) ? set.byBase[name.toLowerCase()] : [];
            return same.length <= 1 ? name : rel;
        }
        return { resolve: resolve, linkText: linkText };
    }

    // ---- fuzzy ----
    function norm(s) { return String(s == null ? '' : s).normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLowerCase(); }
    function fuzzy(query, text) {
        var q = norm(query).replace(/\s+/g, ''), t = norm(text);
        if (!q || !t) return 0;
        var ti = 0, score = 0, last = -2;
        for (var qi = 0; qi < q.length; qi++) {
            var found = -1;
            for (var k = ti; k < t.length; k++) if (t.charAt(k) === q.charAt(qi)) { found = k; break; }
            if (found < 0) return 0;
            score += 10;
            if (found === last + 1) score += 8;
            if (found === 0 || !/[a-z0-9]/.test(t.charAt(found - 1))) score += 6;
            if (qi === 0 && found === 0) score += 15;
            last = found; ti = found + 1;
        }
        return Math.max(0.01, score - t.length * 0.3);
    }

    // ---- store ----
    function create(opts) {
        opts = opts || {};
        var request = opts.request, onChange = opts.onChange;
        var store = { data: { notes: [], folders: [], attachments: [] } };
        var lookup = null, timer = null, pending = null;

        function getLookup() {
            if (!lookup) {
                var aliases = {};
                (store.data.notes || []).forEach(function (n) { if (n.aliases && n.aliases.length) aliases[n.path] = n.aliases; });
                lookup = createLookup({
                    notes: (store.data.notes || []).map(function (n) { return n.path; }),
                    aliases: aliases,
                    attachments: store.data.attachments || []
                });
            }
            return lookup;
        }
        store.load = function () {
            return Promise.resolve(request('/api/desktop/notes?index=1', { method: 'GET' })).then(function (res) {
                if (!res || typeof res !== 'object') throw new Error('invalid notes index');
                store.data = {
                    notes: res.notes || [],
                    folders: res.folders || [],
                    attachments: res.attachments || []
                };
                lookup = null;
                if (typeof onChange === 'function') onChange(store.data);
                return store.data;
            });
        };
        store.refresh = function () {
            if (timer) clearTimeout(timer);
            if (!pending) {
                pending = {};
                pending.promise = new Promise(function (resolve, reject) { pending.resolve = resolve; pending.reject = reject; });
            }
            var p = pending;
            timer = setTimeout(function () {
                timer = null; pending = null;
                store.load().then(p.resolve, p.reject);
            }, 400);
            return p.promise;
        };
        store.notes = function () { return store.data.notes || []; };
        store.note = function (path) {
            var list = store.notes();
            for (var i = 0; i < list.length; i++) if (list[i].path === path) return list[i];
            return null;
        };
        store.resolve = function (from, value, embed, kind) {
            return getLookup().resolve(from, kind || 'wiki', value, !!embed);
        };
        store.linkText = function (path) { return getLookup().linkText(path); };
        // Servers may leave link.path empty; resolve it client-side with the same rules.
        function linkPath(n, l) {
            if (l.path) return l.path;
            return getLookup().resolve(n.path, l.kind || 'wiki', l.target, !!l.embed);
        }
        store.linkPath = linkPath;
        store.backlinks = function (path) {
            var out = [];
            store.notes().forEach(function (n) {
                if (n.path === path) return;
                var count = 0;
                (n.links || []).forEach(function (l) { if (linkPath(n, l) === path) count++; });
                if (count) out.push({ path: n.path, title: n.title || '', count: count });
            });
            return out.sort(function (a, b) { return a.title.localeCompare(b.title) || (a.path < b.path ? -1 : 1); });
        };
        store.outgoing = function (path, content) {
            return extract(content).links.map(function (l) {
                return { value: l.value, path: getLookup().resolve(path, l.kind, l.value, l.embed), embed: l.embed, kind: l.kind };
            });
        };
        store.tags = function () {
            var counts = {};
            store.notes().forEach(function (n) {
                var seenTags = {};
                (n.tags || []).forEach(function (t) {
                    if (seenTags[t]) return;
                    seenTags[t] = true;
                    counts[t] = (counts[t] || 0) + 1;
                });
            });
            return Object.keys(counts).sort().map(function (t) { return { tag: t, count: counts[t] }; });
        };
        store.search = function (query, limit) {
            if (!String(query || '').trim()) return [];
            limit = limit > 0 ? limit : 50;
            var out = [];
            store.notes().forEach(function (n) {
                var p = n.path || '';
                var rel = p.replace(/^documents\/notes\//i, '');
                var base = pathBase(rel).replace(/\.md$/i, '');
                var dir = pathDir(rel);
                var best = fuzzy(query, n.title) * 3;
                (n.aliases || []).forEach(function (a) { best = Math.max(best, fuzzy(query, a) * 2); });
                best = Math.max(best, fuzzy(query, base) * 2.5, dir === '.' ? 0 : fuzzy(query, dir));
                if (best > 0) out.push({ path: p, title: n.title || base, aliases: n.aliases || [], score: best });
            });
            out.sort(function (a, b) { return b.score - a.score || a.title.localeCompare(b.title); });
            return out.slice(0, limit);
        };
        return store;
    }

    return {
        parseWikiValue: parseWikiValue,
        extract: extract,
        createLookup: createLookup,
        fuzzy: fuzzy,
        create: create
    };
});
