/* Notes left sidebar: file tree, search, tag tree and bookmarks.
 * UMD IIFE: window.NotesSidebar in the browser, module.exports under Node tests.
 * No DOM access at load time; every label comes from the host's tr(). */
(function (root, factory) {
    var api = factory();
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
    if (root) root.NotesSidebar = api;
})(typeof window !== 'undefined' ? window : this, function () {
    'use strict';

    var DEFAULT_ROOT = 'Documents/Notes';
    var DND_TYPE = 'application/x-aurago-note';
    var TABS = ['files', 'search', 'tags', 'bookmarks'];
    var KEY_TAB = 'aurago.notes.left';
    var KEY_EXPANDED = 'aurago.notes.expanded';

    function esc(s) {
        return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
            return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
        });
    }
    // One folded code unit per code unit (lowercase, diacritics stripped) so indices map 1:1.
    function foldChar(ch) {
        var n = ch.normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLowerCase();
        return n.length ? n.charAt(0) : ch;
    }
    function fold(s) {
        s = String(s == null ? '' : s);
        var out = '';
        for (var i = 0; i < s.length; i++) out += foldChar(s.charAt(i));
        return out;
    }
    function cmp(a, b) { return String(a).localeCompare(String(b), undefined, { numeric: true, sensitivity: 'base' }); }
    function trimSlashes(s) { return String(s || '').replace(/^\/+|\/+$/g, ''); }
    function dirname(p) { var i = String(p).lastIndexOf('/'); return i < 0 ? '' : p.slice(0, i); }
    function basename(p) { return String(p).slice(String(p).lastIndexOf('/') + 1); }
    function stem(p) { return basename(p).replace(/\.[^.]*$/, ''); }
    function noteTitle(n) { return (n && n.title) || stem(n && n.path || ''); }

    /* ---------- pure helpers ---------- */

    function buildTree(notes, folders, o) {
        o = o || {};
        var rootPath = trimSlashes(o.root || DEFAULT_ROOT);
        var byModified = o.sort === 'modified';
        var map = {};
        function ensure(path) {
            if (map[path]) return map[path];
            var node = { path: path, name: path === rootPath ? '' : basename(path), folders: [], notes: [] };
            map[path] = node;
            if (path !== rootPath) ensure(dirname(path)).folders.push(node);
            return node;
        }
        function under(p) { return p === rootPath || p.indexOf(rootPath + '/') === 0; }
        ensure(rootPath);
        (folders || []).forEach(function (f) { f = trimSlashes(f); if (under(f)) ensure(f); });
        (notes || []).forEach(function (n) {
            var p = n && n.path;
            if (!p || p.indexOf(rootPath + '/') !== 0) return;
            ensure(dirname(p)).notes.push(n);
        });
        Object.keys(map).forEach(function (k) {
            var node = map[k];
            node.folders.sort(function (a, b) { return cmp(a.name, b.name); });
            node.notes.sort(function (a, b) {
                if (byModified) {
                    var d = (+b.modified || 0) - (+a.modified || 0);
                    if (d) return d;
                }
                return cmp(noteTitle(a), noteTitle(b));
            });
        });
        return map[rootPath];
    }

    function parseQuery(q, rootOpt) {
        var rootPath = trimSlashes(rootOpt || DEFAULT_ROOT);
        var tag = '', folder = '';
        var rest = String(q == null ? '' : q).replace(/(^|\s)(tag|path):("[^"]*"|\S*)/gi, function (m, pre, key, val) {
            val = val.replace(/^"|"$/g, '');
            if (key.toLowerCase() === 'tag') { val = val.replace(/^#+/, ''); if (val) tag = val; }
            else { val = trimSlashes(val); if (val) folder = rootPath + '/' + val; }
            return pre;
        });
        return { q: rest.split(/\s+/).filter(Boolean).join(' '), tag: tag, folder: folder };
    }

    function markTerms(text, terms) {
        text = String(text == null ? '' : text);
        var f = fold(text), ranges = [];
        (terms || []).forEach(function (term) {
            term = fold(term).trim();
            if (!term) return;
            for (var i = f.indexOf(term); i >= 0; i = f.indexOf(term, i + term.length)) ranges.push([i, i + term.length]);
        });
        ranges.sort(function (a, b) { return a[0] - b[0] || b[1] - a[1]; });
        var merged = [];
        ranges.forEach(function (r) {
            var last = merged[merged.length - 1];
            if (last && r[0] <= last[1]) last[1] = Math.max(last[1], r[1]);
            else merged.push([r[0], r[1]]);
        });
        var out = '', pos = 0;
        merged.forEach(function (r) {
            out += esc(text.slice(pos, r[0])) + '<mark>' + esc(text.slice(r[0], r[1])) + '</mark>';
            pos = r[1];
        });
        return out + esc(text.slice(pos));
    }

    /* ---------- fallback icons (host icons win when they return markup) ---------- */

    function svg(body) {
        return '<svg class="notes-icon" viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">' + body + '</svg>';
    }
    var FALLBACK = {
        'chevron-right': svg('<path d="M9 6l6 6-6 6"/>'),
        tag: svg('<path d="M4 4h7l9 9-7 7-9-9z"/><circle cx="8.5" cy="8.5" r="1.2"/>'),
        sort: svg('<path d="M7 4v16M4 17l3 3 3-3M17 20V4M14 7l3-3 3 3"/>'),
        collapse: svg('<path d="M5 9l7-5 7 5M5 15l7 5 7-5"/>'),
        'folder-plus': svg('<path d="M3 6a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><path d="M12 10v6M9 13h6"/>')
    };

    /* ---------- component ---------- */

    function create(container, o) {
        o = o || {};
        var rootPath = trimSlashes(o.root || DEFAULT_ROOT);
        var doc = container.ownerDocument || document;
        var tr = function (k, p) { var v = o.tr ? o.tr(k, p) : k; return v == null ? k : String(v); };
        var ic = function (name) { var h = o.icon ? o.icon(name) : ''; if (!h && name === 'folder-open' && o.icon) h = o.icon('folder'); return h || FALLBACK[name] || ''; };
        var store = o.storage || { get: function () { return null; }, set: function () {} };

        function readStore(key) { try { return store.get(key); } catch (e) { return null; } }
        function writeStore(key, v) { try { store.set(key, v); } catch (e) { /* storage unavailable */ } }

        var tab = readStore(KEY_TAB);
        if (TABS.indexOf(tab) < 0) tab = 'files';
        var expanded = {};
        (function () {
            var raw = readStore(KEY_EXPANDED);
            if (typeof raw === 'string') { try { raw = JSON.parse(raw); } catch (e) { raw = []; } }
            if (Array.isArray(raw)) raw.forEach(function (p) { expanded[p] = true; });
        })();
        var tagOpen = {};
        var selectedFolder = null;
        var focusedPath = null;
        var lastCurrent;
        var dragPath = null;
        var gen = 0, timer = null;
        var search = { q: '', results: null, total: 0, loading: false };
        var destroyed = false;

        function saveExpanded() { writeStore(KEY_EXPANDED, JSON.stringify(Object.keys(expanded))); }
        function state() { return (o.getState && o.getState()) || {}; }

        /* skeleton */
        var tabIcon = { files: 'folder', search: 'search', tags: 'tag', bookmarks: 'star' };
        var el = doc.createElement('div');
        el.className = 'notes-nav';
        el.innerHTML = '<div class="notes-nav-tabs" role="tablist">' + TABS.map(function (t) {
            return '<button type="button" class="notes-nav-tab" role="tab" data-tab="' + t + '" id="notes-nav-tab-' + t + '" aria-controls="notes-nav-panel-' + t + '">' + ic(tabIcon[t]) + '</button>';
        }).join('') + '</div>' + TABS.map(function (t) {
            return '<div class="notes-nav-panel" role="tabpanel" data-panel="' + t + '" id="notes-nav-panel-' + t + '" aria-labelledby="notes-nav-tab-' + t + '"></div>';
        }).join('');
        container.appendChild(el);
        var tabBtns = {}, panels = {};
        TABS.forEach(function (t) {
            tabBtns[t] = el.querySelector('[data-tab="' + t + '"]');
            panels[t] = el.querySelector('[data-panel="' + t + '"]');
            var label = tr(t);
            tabBtns[t].setAttribute('title', label);
            tabBtns[t].setAttribute('aria-label', label);
        });
        panels.search.innerHTML = '<div class="notes-nav-searchbox"><input type="search" class="notes-nav-input" autocomplete="off" spellcheck="false" placeholder="' +
            esc(tr('search_all')) + '" aria-label="' + esc(tr('search_all')) + '"><p class="notes-nav-hint">' + esc(tr('search_hint')) +
            '</p></div><div class="notes-nav-count" aria-live="polite"></div><div class="notes-nav-results notes-nav-scroll" role="listbox"></div>';
        var input = panels.search.querySelector('input');
        var resultsEl = panels.search.querySelector('.notes-nav-results');
        var countEl = panels.search.querySelector('.notes-nav-count');

        function applyTab() {
            TABS.forEach(function (t) {
                var on = t === tab;
                tabBtns[t].setAttribute('aria-selected', on ? 'true' : 'false');
                tabBtns[t].tabIndex = on ? 0 : -1;
                panels[t].hidden = !on;
            });
        }

        /* ---------- files ---------- */

        function ancestorsOf(p) {
            var out = [];
            if (String(p).indexOf(rootPath + '/') !== 0) return out;
            var parts = dirname(p).slice(rootPath.length + 1).split('/').filter(Boolean), cur = rootPath;
            parts.forEach(function (s) { cur += '/' + s; out.push(cur); });
            return out;
        }
        function newNoteFolder(st) {
            return selectedFolder || (st.current ? dirname(st.current) : '') || rootPath;
        }
        function iconBtn(act, name, label, extra) {
            return '<button type="button" class="notes-nav-btn" data-act="' + act + '" title="' + esc(label) + '" aria-label="' + esc(label) + '"' + (extra || '') + '>' + ic(name) + '</button>';
        }

        function renderFiles(st) {
            var p = panels.files;
            var trash = !!st.trash, trashNotes = st.trashNotes || [];
            var head = '<div class="notes-nav-head"><strong>' + esc(tr('files')) + '</strong><span class="notes-nav-actions">' +
                iconBtn('new-note', 'file-plus', tr('new_note')) + iconBtn('new-folder', 'folder-plus', tr('new_folder')) +
                iconBtn('sort', 'sort', tr(st.sort === 'modified' ? 'sort_modified' : 'sort_name')) +
                iconBtn('collapse', 'collapse', tr('collapse_all')) + '</span></div>';
            var body = '';
            if (trash) {
                body = trashNotes.length ? trashNotes.map(function (n) {
                    return '<div class="notes-tree-row" role="treeitem" aria-level="1" data-kind="trash" data-path="' + esc(n.path) + '" title="' + esc(n.path) + '" tabindex="-1" style="--lvl:1">' +
                        '<span class="notes-tree-twist"></span><span class="notes-tree-ico">' + ic('notes') + '</span><span class="notes-tree-name">' + esc(noteTitle(n)) + '</span></div>';
                }).join('') : '<div class="notes-nav-empty">' + esc(tr('no_results')) + '</div>';
            } else {
                var tree = buildTree(st.notes || [], st.folders || [], { sort: st.sort, root: rootPath });
                var rows = [];
                (function walk(node, level) {
                    node.folders.forEach(function (f) {
                        rows.push({ folder: f, level: level });
                        if (expanded[f.path]) walk(f, level + 1);
                    });
                    node.notes.forEach(function (n) { rows.push({ note: n, level: level }); });
                })(tree, 1);
                if (!rows.length) body = '<div class="notes-nav-empty">' + esc(tr('no_results')) + '</div>';
                body = rows.map(function (r) {
                    if (r.folder) {
                        var open = !!expanded[r.folder.path];
                        return '<div class="notes-tree-row' + (selectedFolder === r.folder.path ? ' is-selected' : '') + '" role="treeitem" aria-level="' + r.level +
                            '" aria-expanded="' + open + '" data-kind="folder" data-path="' + esc(r.folder.path) + '" title="' + esc(r.folder.path) + '" tabindex="-1" style="--lvl:' + r.level + '">' +
                            '<span class="notes-tree-twist' + (open ? ' is-open' : '') + '">' + ic('chevron-right') + '</span><span class="notes-tree-ico">' + ic(open ? 'folder-open' : 'folder') +
                            '</span><span class="notes-tree-name">' + esc(r.folder.name) + '</span></div>';
                    }
                    var n = r.note, cur = n.path === st.current;
                    return '<div class="notes-tree-row' + (cur ? ' is-current' : '') + '" role="treeitem" aria-level="' + r.level + '"' + (cur ? ' aria-current="page"' : '') +
                        ' data-kind="note" data-path="' + esc(n.path) + '" title="' + esc(n.path) + '" draggable="true" tabindex="-1" style="--lvl:' + r.level + '">' +
                        '<span class="notes-tree-twist"></span><span class="notes-tree-ico">' + ic('notes') + '</span><span class="notes-tree-name">' + esc(noteTitle(n)) + '</span></div>';
                }).join('') || body;
            }
            var foot = '<div class="notes-nav-foot"><button type="button" class="notes-nav-trash" data-act="trash" aria-pressed="' + trash + '" title="' + esc(tr('trash')) + '">' +
                ic('trash') + '<span>' + esc(tr('trash')) + '</span>' + (trashNotes.length ? '<span class="notes-nav-badge">' + trashNotes.length + '</span>' : '') + '</button></div>';
            p.innerHTML = head + '<div class="notes-tree notes-nav-scroll" role="tree" aria-label="' + esc(tr('files')) + '" data-drop="' + esc(rootPath) + '">' + body + '</div>' + foot;
            // roving tabindex
            var rowsEl = p.querySelectorAll('.notes-tree-row');
            var target = null;
            for (var i = 0; i < rowsEl.length; i++) if (rowsEl[i].getAttribute('data-path') === focusedPath) target = rowsEl[i];
            if (!target) target = p.querySelector('.is-current') || rowsEl[0];
            if (target) target.tabIndex = 0;
        }

        /* ---------- tags ---------- */

        function renderTags(st) {
            var counts = st.tagCounts || [];
            if (!counts.length) { panels.tags.innerHTML = '<div class="notes-nav-head"><strong>' + esc(tr('tags')) + '</strong></div><div class="notes-nav-empty">' + esc(tr('no_tags')) + '</div>'; return; }
            var rootNode = { children: {}, own: 0, total: 0 };
            counts.forEach(function (c) {
                var parts = String(c.tag).replace(/^#/, '').split('/').filter(Boolean), node = rootNode, full = '';
                parts.forEach(function (s, i) {
                    full = full ? full + '/' + s : s;
                    var key = s.toLowerCase();
                    node = node.children[key] || (node.children[key] = { name: s, full: full, children: {}, own: 0, total: 0 });
                    if (i === parts.length - 1) node.own += +c.count || 0;
                });
            });
            (function sum(n) {
                var t = n.own || 0;
                Object.keys(n.children).forEach(function (k) { t += sum(n.children[k]); });
                n.total = t; return t;
            })(rootNode);
            var html = '';
            (function walk(n, level) {
                Object.keys(n.children).map(function (k) { return n.children[k]; }).sort(function (a, b) { return cmp(a.name, b.name); }).forEach(function (c) {
                    var has = Object.keys(c.children).length > 0, open = !!tagOpen[c.full];
                    html += '<div class="notes-tag-row" role="treeitem" aria-level="' + level + '"' + (has ? ' aria-expanded="' + open + '"' : '') + ' style="--lvl:' + level + '">' +
                        (has ? '<button type="button" class="notes-tag-twist' + (open ? ' is-open' : '') + '" data-toggle-tag="' + esc(c.full) + '" tabindex="-1" aria-label="' + esc(c.name) + '">' + ic('chevron-right') + '</button>' : '<span class="notes-tag-twist"></span>') +
                        '<button type="button" class="notes-tag-name" data-tag="' + esc(c.full) + '"><span class="notes-tag-hash">#</span><span class="notes-tree-name">' + esc(c.name) + '</span><span class="notes-nav-badge">' + c.total + '</span></button></div>';
                    if (has && open) walk(c, level + 1);
                });
            })(rootNode, 1);
            panels.tags.innerHTML = '<div class="notes-nav-head"><strong>' + esc(tr('tags')) + '</strong></div><div class="notes-nav-scroll" role="tree">' + html + '</div>';
        }

        /* ---------- bookmarks ---------- */

        function renderBookmarks(st) {
            var pinned = st.pinned || [], byPath = {};
            (st.notes || []).forEach(function (n) { byPath[n.path] = n; });
            var body = pinned.length ? pinned.map(function (pth) {
                var n = byPath[pth];
                return '<button type="button" class="notes-bm-row' + (pth === st.current ? ' is-current' : '') + (n ? '' : ' is-missing') + '" data-path="' + esc(pth) + '" title="' + esc(pth) + '"' + (pth === st.current ? ' aria-current="page"' : '') + '>' +
                    '<span class="notes-tree-ico">' + ic('star') + '</span><span class="notes-tree-name">' + esc(n ? noteTitle(n) : basename(pth)) + '</span></button>';
            }).join('') : '<div class="notes-nav-empty">' + esc(tr('no_bookmarks')) + '</div>';
            panels.bookmarks.innerHTML = '<div class="notes-nav-head"><strong>' + esc(tr('bookmarks')) + '</strong></div><div class="notes-nav-scroll">' + body + '</div>';
        }

        /* ---------- search ---------- */

        function relFolder(p) {
            var d = dirname(p);
            return d.indexOf(rootPath + '/') === 0 ? d.slice(rootPath.length + 1) : (d === rootPath ? '' : d);
        }
        function renderResults() {
            var r = search.results;
            if (!r) { resultsEl.innerHTML = ''; countEl.textContent = ''; return; }
            var terms = parseQuery(search.q, rootPath).q.split(' ').filter(Boolean);
            countEl.textContent = tr('results_count', { count: search.total });
            resultsEl.innerHTML = r.length ? r.map(function (n) {
                var fld = relFolder(n.path);
                return '<button type="button" class="notes-search-card" role="option" data-path="' + esc(n.path) + '" title="' + esc(n.path) + '">' +
                    '<strong class="notes-search-title">' + markTerms(noteTitle(n), terms) + '</strong>' +
                    (n.snippet ? '<span class="notes-search-snippet">' + markTerms(n.snippet, terms) + '</span>' : '') +
                    '<span class="notes-search-meta">' + (fld ? '<span class="notes-search-folder">' + esc(fld) + '</span>' : '') +
                    (n.tags || []).map(function (t) { return '<span class="notes-search-tag">#' + esc(String(t).replace(/^#/, '')) + '</span>'; }).join('') + '</span></button>';
            }).join('') : '<div class="notes-nav-empty">' + esc(tr('no_results')) + '</div>';
        }
        function runSearch() {
            clearTimeout(timer);
            var q = input.value.trim();
            var my = ++gen;
            search.q = q;
            if (!q) { search.results = null; search.total = 0; renderResults(); return; }
            Promise.resolve(o.onSearch ? o.onSearch(q) : { notes: [], total: 0 }).then(function (res) {
                if (my !== gen || destroyed) return;
                res = res || {};
                search.results = res.notes || [];
                search.total = typeof res.total === 'number' ? res.total : search.results.length;
                renderResults();
            }, function () {
                if (my !== gen || destroyed) return;
                search.results = []; search.total = 0; renderResults();
            });
        }

        /* ---------- render ---------- */

        function render() {
            if (destroyed) return;
            var st = state();
            if (st.current !== lastCurrent) {
                lastCurrent = st.current;
                if (st.current) {
                    var anc = ancestorsOf(st.current), changed = false;
                    anc.forEach(function (a) { if (!expanded[a]) { expanded[a] = true; changed = true; } });
                    if (changed) saveExpanded();
                }
            }
            var active = doc.activeElement, refocus = null, scroller = el.querySelector('[data-panel="' + tab + '"] .notes-nav-scroll');
            var scroll = scroller ? scroller.scrollTop : 0;
            if (active && el.contains(active) && tab !== 'search') {
                var host = active.closest('[data-path],[data-tag],[data-toggle-tag],[data-act]');
                if (host) refocus = host.hasAttribute('data-path') ? '[data-path="' + cssAttr(host.getAttribute('data-path')) + '"]'
                    : host.hasAttribute('data-act') ? '[data-act="' + host.getAttribute('data-act') + '"]'
                    : host.hasAttribute('data-tag') ? '[data-tag="' + cssAttr(host.getAttribute('data-tag')) + '"]' : null;
            }
            applyTab();
            if (tab === 'files') renderFiles(st);
            else if (tab === 'tags') renderTags(st);
            else if (tab === 'bookmarks') renderBookmarks(st);
            scroller = el.querySelector('[data-panel="' + tab + '"] .notes-nav-scroll');
            if (scroller) scroller.scrollTop = scroll;
            if (refocus) {
                var again = panels[tab].querySelector(refocus);
                if (again) { if (again.tabIndex < 0 && again.getAttribute('role') === 'treeitem') again.tabIndex = 0; again.focus({ preventScroll: true }); }
            }
        }
        function cssAttr(v) { return String(v).replace(/["\\]/g, '\\$&'); }

        function setTab(name) {
            if (TABS.indexOf(name) < 0) return;
            tab = name;
            writeStore(KEY_TAB, name);
            render();
        }
        function focusSearch(query) {
            setTab('search');
            if (typeof query === 'string') { input.value = query; runSearch(); }
            input.focus();
            input.select();
        }
        function revealPath(p) {
            if (tab !== 'files') setTab('files');
            ancestorsOf(p).forEach(function (a) { expanded[a] = true; });
            saveExpanded();
            focusedPath = p;
            selectedFolder = null;
            render();
            var row = panels.files.querySelector('[data-path="' + cssAttr(p) + '"]');
            if (row) {
                if (row.getAttribute('data-kind') === 'folder') { selectedFolder = p; row.classList.add('is-selected'); }
                if (row.scrollIntoView) row.scrollIntoView({ block: 'nearest' });
            }
        }

        /* ---------- events ---------- */

        function toggleFolder(p) {
            if (expanded[p]) delete expanded[p]; else expanded[p] = true;
            saveExpanded();
        }
        function treeRows() { return Array.prototype.slice.call(panels.files.querySelectorAll('.notes-tree-row')); }
        function focusRow(row) {
            if (!row) return;
            treeRows().forEach(function (r) { r.tabIndex = -1; });
            row.tabIndex = 0;
            focusedPath = row.getAttribute('data-path');
            row.focus();
        }
        function ctxEvent(row, e) {
            if (e.type === 'contextmenu') return e;
            var r = row.getBoundingClientRect();
            return { type: 'contextmenu', clientX: r.left + 24, clientY: r.bottom, target: row, key: e.key,
                preventDefault: function () {}, stopPropagation: function () {} };
        }
        function fireCtx(row, e) {
            var kind = row.getAttribute('data-kind') === 'folder' ? 'folder' : row.getAttribute('data-kind') === 'trash' ? 'trash' : 'note';
            if (e.preventDefault) e.preventDefault();
            if (o.onContextMenu) o.onContextMenu(row.getAttribute('data-path'), kind, ctxEvent(row, e));
        }

        function onClick(e) {
            var t = e.target;
            var tabBtn = t.closest('[data-tab]');
            if (tabBtn) { setTab(tabBtn.getAttribute('data-tab')); if (tab === 'search') input.focus(); return; }
            var act = t.closest('[data-act]');
            var st = state();
            if (act) {
                var a = act.getAttribute('data-act');
                if (a === 'new-note') { if (o.onNewNote) o.onNewNote(newNoteFolder(st)); }
                else if (a === 'new-folder') { if (o.onNewFolder) o.onNewFolder(selectedFolder || rootPath); }
                else if (a === 'sort') { if (o.onSort) o.onSort(st.sort === 'modified' ? 'name' : 'modified'); }
                else if (a === 'collapse') { expanded = {}; saveExpanded(); render(); }
                else if (a === 'trash') { if (o.onToggleTrash) o.onToggleTrash(); }
                return;
            }
            var row = t.closest('.notes-tree-row');
            if (row) {
                var kind = row.getAttribute('data-kind'), p = row.getAttribute('data-path');
                focusedPath = p;
                if (kind === 'folder') { selectedFolder = p; toggleFolder(p); render(); }
                else { if (kind === 'note') selectedFolder = null; if (o.onOpen) o.onOpen(p, e); render(); }
                return;
            }
            var tg = t.closest('[data-toggle-tag]');
            if (tg) { var k = tg.getAttribute('data-toggle-tag'); tagOpen[k] = !tagOpen[k]; render(); return; }
            var tagBtn = t.closest('[data-tag]');
            if (tagBtn) {
                var tagName = tagBtn.getAttribute('data-tag');
                if (o.onTag) o.onTag(tagName); else focusSearch('tag:' + tagName);
                return;
            }
            var card = t.closest('.notes-search-card, .notes-bm-row');
            if (card && o.onOpen) o.onOpen(card.getAttribute('data-path'), e);
        }

        function onContextMenu(e) {
            var row = e.target.closest('.notes-tree-row');
            if (row) { fireCtx(row, e); return; }
            var bm = e.target.closest('.notes-bm-row, .notes-search-card');
            if (bm) { e.preventDefault(); if (o.onContextMenu) o.onContextMenu(bm.getAttribute('data-path'), 'note', e); }
        }

        function onKeyDown(e) {
            var t = e.target, key = e.key;
            var tabBtn = t.closest && t.closest('[data-tab]');
            if (tabBtn && (key === 'ArrowLeft' || key === 'ArrowRight' || key === 'Home' || key === 'End')) {
                var i = TABS.indexOf(tabBtn.getAttribute('data-tab'));
                i = key === 'Home' ? 0 : key === 'End' ? TABS.length - 1 : (i + (key === 'ArrowRight' ? 1 : -1) + TABS.length) % TABS.length;
                e.preventDefault(); setTab(TABS[i]); tabBtns[TABS[i]].focus(); return;
            }
            if (t === input) {
                if (key === 'ArrowDown') { var first = resultsEl.querySelector('.notes-search-card'); if (first) { e.preventDefault(); first.focus(); } }
                else if (key === 'Escape' && input.value) { e.preventDefault(); input.value = ''; runSearch(); }
                else if (key === 'Enter') runSearch();
                return;
            }
            var card = t.closest && t.closest('.notes-search-card');
            if (card && (key === 'ArrowDown' || key === 'ArrowUp')) {
                e.preventDefault();
                var cards = Array.prototype.slice.call(resultsEl.querySelectorAll('.notes-search-card'));
                var idx = cards.indexOf(card) + (key === 'ArrowDown' ? 1 : -1);
                if (idx < 0) input.focus(); else if (cards[idx]) cards[idx].focus();
                return;
            }
            var row = t.closest && t.closest('.notes-tree-row');
            if (!row) return;
            if (key === 'ContextMenu' || (key === 'F10' && e.shiftKey)) { fireCtx(row, e); return; }
            var rows = treeRows(), idx2 = rows.indexOf(row), kind = row.getAttribute('data-kind'), p = row.getAttribute('data-path');
            var level = +row.getAttribute('aria-level');
            if (key === 'ArrowDown') { e.preventDefault(); focusRow(rows[Math.min(rows.length - 1, idx2 + 1)]); }
            else if (key === 'ArrowUp') { e.preventDefault(); focusRow(rows[Math.max(0, idx2 - 1)]); }
            else if (key === 'Home') { e.preventDefault(); focusRow(rows[0]); }
            else if (key === 'End') { e.preventDefault(); focusRow(rows[rows.length - 1]); }
            else if (key === 'ArrowRight' && kind === 'folder') {
                e.preventDefault();
                if (!expanded[p]) { expanded[p] = true; saveExpanded(); focusedPath = p; render(); }
                else { var nx = rows[idx2 + 1]; if (nx && +nx.getAttribute('aria-level') > level) focusRow(nx); }
            } else if (key === 'ArrowLeft') {
                e.preventDefault();
                if (kind === 'folder' && expanded[p]) { delete expanded[p]; saveExpanded(); focusedPath = p; render(); }
                else {
                    for (var j = idx2 - 1; j >= 0; j--) if (+rows[j].getAttribute('aria-level') < level) { focusRow(rows[j]); break; }
                }
            } else if (key === 'Enter' || key === ' ') {
                e.preventDefault();
                if (kind === 'folder') { selectedFolder = p; toggleFolder(p); focusedPath = p; render(); }
                else { focusedPath = p; if (kind === 'note') selectedFolder = null; if (o.onOpen) o.onOpen(p, e); render(); }
            }
        }

        function hasDnd(e) {
            var types = e.dataTransfer && e.dataTransfer.types;
            if (!types) return !!dragPath;
            for (var i = 0; i < types.length; i++) if (types[i] === DND_TYPE) return true;
            return false;
        }
        function dropTarget(e) {
            var row = e.target.closest && e.target.closest('.notes-tree-row');
            if (row && row.getAttribute('data-kind') === 'folder') return { el: row, folder: row.getAttribute('data-path') };
            if (row && row.getAttribute('data-kind') === 'note') return { el: null, folder: dirname(row.getAttribute('data-path')) };
            var tree = e.target.closest && e.target.closest('.notes-tree');
            return tree && !state().trash ? { el: tree, folder: rootPath } : null;
        }
        function clearDrop() {
            var on = el.querySelectorAll('.is-drop');
            for (var i = 0; i < on.length; i++) on[i].classList.remove('is-drop');
        }
        function onDragStart(e) {
            var row = e.target.closest && e.target.closest('.notes-tree-row[data-kind=note]');
            if (!row) return;
            dragPath = row.getAttribute('data-path');
            e.dataTransfer.setData(DND_TYPE, dragPath);
            e.dataTransfer.setData('text/plain', dragPath);
            e.dataTransfer.effectAllowed = 'move';
            row.classList.add('is-dragging');
        }
        function onDragOver(e) {
            if (!hasDnd(e)) return;
            var tg = dropTarget(e);
            if (!tg) return;
            clearDrop();
            if (tg.folder === dirname(dragPath || '')) return;
            e.preventDefault();
            e.dataTransfer.dropEffect = 'move';
            if (tg.el) tg.el.classList.add('is-drop');
        }
        function onDragLeave(e) {
            if (!e.relatedTarget || !el.contains(e.relatedTarget)) clearDrop();
        }
        function onDrop(e) {
            if (!hasDnd(e)) return;
            var tg = dropTarget(e);
            var p = (e.dataTransfer && e.dataTransfer.getData(DND_TYPE)) || dragPath;
            clearDrop();
            if (!tg || !p) return;
            e.preventDefault();
            dragPath = null;
            if (tg.folder !== dirname(p) && o.onMove) o.onMove(p, tg.folder);
        }
        function onDragEnd() {
            dragPath = null; clearDrop();
            var d = el.querySelectorAll('.is-dragging');
            for (var i = 0; i < d.length; i++) d[i].classList.remove('is-dragging');
        }
        function onInput() { clearTimeout(timer); timer = setTimeout(runSearch, 250); }

        el.addEventListener('click', onClick);
        el.addEventListener('contextmenu', onContextMenu);
        el.addEventListener('keydown', onKeyDown);
        el.addEventListener('dragstart', onDragStart);
        el.addEventListener('dragover', onDragOver);
        el.addEventListener('dragleave', onDragLeave);
        el.addEventListener('drop', onDrop);
        el.addEventListener('dragend', onDragEnd);
        input.addEventListener('input', onInput);

        function destroy() {
            destroyed = true;
            clearTimeout(timer);
            gen++;
            if (el.parentNode) el.parentNode.removeChild(el);
        }

        applyTab();
        render();
        return { render: render, setTab: setTab, revealPath: revealPath, focusSearch: focusSearch, destroy: destroy };
    }

    return { create: create, buildTree: buildTree, parseQuery: parseQuery, markTerms: markTerms };
});
