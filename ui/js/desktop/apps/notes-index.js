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
    function wikiLinkSpan(line, start) {
        var vs = start + 2, j = vs;
        while (j < line.length) {
            var c = line.charAt(j);
            if (c === '\\') { j += 2; continue; }
            if (c === '[' && line.charAt(j + 1) === '[') return null;
            if (c === ']' && line.charAt(j + 1) === ']') {
                var n = runeCount(line.slice(vs, j));
                return n > 0 && n <= MAX_WIKI_RUNES ? { vs: vs, ve: j } : null;
            }
            j++;
        }
        return null;
    }
    function markdownLinkURL(line, start) {
        var vs, ve;
        if (start < line.length && line.charAt(start) === '<') {
            var k = line.indexOf('>', start + 1);
            if (k < 0) return null;
            vs = start + 1; ve = k; start = ve + 1;
        } else {
            ve = start;
            while (ve < line.length && ' \t)'.indexOf(line.charAt(ve)) < 0) ve++;
            vs = start; start = ve;
        }
        var closeAt = line.indexOf(')', start);
        if (closeAt < 0) return null;
        return { vs: vs, ve: ve, after: closeAt + 1 };
    }
    function markdownLinkIsImage(line, close) {
        var depth = 0;
        for (var b = close - 1; b >= 0; b--) {
            var c = line.charAt(b);
            if (c === ']') depth++;
            else if (c === '[') {
                if (depth === 0) return b > 0 && line.charAt(b - 1) === '!';
                depth--;
            }
        }
        return false;
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

        var heading = '', firstLine = '', fenceChar = '', fenceLen = 0, inComment = false;
        for (var pos = bodyStart; pos < content.length;) {
            var nlAt = content.indexOf('\n', pos);
            var lineEnd = nlAt < 0 ? content.length : nlAt;
            var line = content.slice(pos, lineEnd);
            pos = nlAt < 0 ? content.length : nlAt + 1;
            var trimmed = line.replace(/\r+$/, '');

            if (fenceLen > 0) {
                var t0 = trimmed.trim();
                if (t0.length >= fenceLen && t0.split(fenceChar).join('') === '') fenceLen = 0;
                continue;
            }
            if (!inComment) {
                var t = trimmed.replace(/^[ \t]+/, '');
                if (t.length >= 3 && (t.charAt(0) === '`' || t.charAt(0) === '~')) {
                    var n = 0;
                    while (n < t.length && t.charAt(n) === t.charAt(0)) n++;
                    if (n >= 3 && !(t.charAt(0) === '`' && t.slice(n).indexOf('`') >= 0)) {
                        fenceChar = t.charAt(0); fenceLen = n;
                        continue;
                    }
                }
                if (heading === '' && trimmed.indexOf('# ') === 0) heading = trimmed.slice(2).trim();
                if (firstLine === '') firstLine = trimmed.trim();
            }

            var i = 0;
            if (inComment) {
                var jc = line.indexOf('-->');
                if (jc < 0) continue;
                i = jc + 3; inComment = false;
            }
            while (i < line.length) {
                var c = line.charAt(i);
                if (c === '\\') { i += 2; }
                else if (c === '`') {
                    var cnt = 0;
                    while (i + cnt < line.length && line.charAt(i + cnt) === '`') cnt++;
                    var closeAt = -1;
                    for (var k = i + cnt; k < line.length;) {
                        if (line.charAt(k) !== '`') { k++; continue; }
                        var m = 0;
                        while (k + m < line.length && line.charAt(k + m) === '`') m++;
                        if (m === cnt) { closeAt = k; break; }
                        k += m;
                    }
                    i = closeAt >= 0 ? closeAt + cnt : i + cnt;
                } else if (c === '<' && line.startsWith('<!--', i)) {
                    var je = line.indexOf('-->', i + 4);
                    if (je >= 0) i = je + 3; else { inComment = true; i = line.length; }
                } else if ((c === '[' && line.startsWith('[[', i)) || (c === '!' && line.startsWith('![[', i))) {
                    var embed = c === '!', open = embed ? i + 1 : i;
                    var span = wikiLinkSpan(line, open);
                    if (span) {
                        refs.links.push({ kind: 'wiki', value: line.slice(span.vs, span.ve), embed: embed });
                        i = span.ve + 2;
                    } else i = open + 1;
                } else if (c === ']' && line.charAt(i + 1) === '(') {
                    var mu = markdownLinkURL(line, i + 2);
                    if (!mu) { i += 2; continue; }
                    var url = line.slice(mu.vs, mu.ve);
                    if (noteLinkURLOK(url)) refs.links.push({ kind: 'markdown', value: url, embed: markdownLinkIsImage(line, i) });
                    i = mu.after;
                } else if (c === '#') {
                    var prevOK = i === 0;
                    if (!prevOK) {
                        var prev = Array.from(line.slice(Math.max(0, i - 2), i)).pop();
                        prevOK = SPACE.test(prev) || prev === '(';
                    }
                    var j = i + 1;
                    if (prevOK) {
                        while (j < line.length) {
                            var cp = String.fromCodePoint(line.codePointAt(j));
                            if (!TAG_RUNE.test(cp)) break;
                            j += cp.length;
                        }
                        var value = line.slice(i + 1, j);
                        if (value !== '' && /[^\p{Nd}]/u.test(value)) addTag(value);
                    }
                    i = j;
                } else i++;
            }
        }

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
        store.backlinks = function (path) {
            var out = [];
            store.notes().forEach(function (n) {
                if (n.path === path) return;
                var count = 0;
                (n.links || []).forEach(function (l) { if (l.path === path) count++; });
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
