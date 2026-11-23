/* Notes fuzzy palette: quick switcher (Ctrl+O) and command palette (Ctrl+P).
 * UMD IIFE: window.NotesPalette in the browser, module.exports under Node tests.
 * No DOM access at load time; all labels come from the host. */
(function (root, factory) {
    var api = factory();
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
    if (root) root.NotesPalette = api;
})(typeof window !== 'undefined' ? window : this, function () {
    'use strict';

    var MAX_ROWS = 50;
    var MAX_MATCH_LEN = 240;
    var uid = 0;

    function esc(s) {
        return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
            return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
        });
    }

    // One normalised code unit per input code unit, so indices map 1:1 onto the original text.
    function foldChar(ch) {
        var n = ch.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase();
        return n.length ? n.charAt(0) : ch.toLowerCase().charAt(0) || ch;
    }
    function fold(text) {
        var out = '';
        for (var i = 0; i < text.length; i++) out += foldChar(text.charAt(i));
        return out;
    }
    function isWordChar(c) { return /[a-z0-9À-￿]/i.test(c); }

    /* Best in-order subsequence match (dynamic programming). Word starts and consecutive runs score higher;
     * gaps cost a little. Returns the matched indices into `text`, or null. */
    function matchPositions(query, text) {
        text = String(text == null ? '' : text);
        var q = fold(String(query == null ? '' : query)).replace(/\s+/g, '');
        if (!q) return [];
        var t = fold(text.slice(0, MAX_MATCH_LEN));
        var n = t.length, m = q.length;
        if (m > n) return null;
        var NEG = -1e9, i, j, k;
        var bonus = [];
        for (i = 0; i < n; i++) {
            var prev = i ? text.charAt(i - 1) : '';
            var cur = text.charAt(i);
            var start = i === 0 || !isWordChar(prev) ||
                (cur !== cur.toLowerCase() && prev === prev.toLowerCase() && prev !== prev.toUpperCase());
            bonus[i] = start ? 8 : 0;
        }
        var score = [], from = [];
        for (j = 0; j < m; j++) { score.push(new Array(n).fill(NEG)); from.push(new Array(n).fill(-1)); }
        for (i = 0; i < n; i++) if (t.charAt(i) === q.charAt(0)) score[0][i] = 1 + bonus[i] - Math.min(i, 6) * 0.2;
        for (j = 1; j < m; j++) {
            for (i = j; i < n; i++) {
                if (t.charAt(i) !== q.charAt(j)) continue;
                var best = NEG, bestK = -1;
                for (k = j - 1; k < i; k++) {
                    if (score[j - 1][k] <= NEG) continue;
                    var s = score[j - 1][k] + (k === i - 1 ? 6 : -Math.min(i - k - 1, 8) * 0.5);
                    if (s > best) { best = s; bestK = k; }
                }
                if (bestK >= 0) { score[j][i] = best + 1 + bonus[i]; from[j][i] = bestK; }
            }
        }
        var endI = -1, endS = NEG;
        for (i = 0; i < n; i++) if (score[m - 1][i] > endS) { endS = score[m - 1][i]; endI = i; }
        if (endI < 0) return null;
        var out = new Array(m);
        for (j = m - 1, i = endI; j >= 0; j--) { out[j] = i; i = from[j][i]; }
        return out;
    }

    function labelHtml(label, query) {
        label = String(label == null ? '' : label);
        var pos = query ? matchPositions(query, label) : null;
        if (!pos || !pos.length) return esc(label);
        var set = {}, html = '', open = false;
        pos.forEach(function (p) { set[p] = true; });
        for (var i = 0; i < label.length; i++) {
            if (set[i] && !open) { html += '<mark>'; open = true; }
            if (!set[i] && open) { html += '</mark>'; open = false; }
            html += esc(label.charAt(i));
        }
        return html + (open ? '</mark>' : '');
    }

    function open(host, opts) {
        opts = opts || {};
        if (host.__notesPalette) host.__notesPalette.close(true);
        var doc = host.ownerDocument || document;
        var prevFocus = doc.activeElement;
        var id = 'notes-pal-' + (++uid);
        var rows = [];          // {item, create}
        var active = 0;
        var closed = false;

        var backdrop = doc.createElement('div');
        backdrop.className = 'notes-palette-backdrop';
        var dlg = doc.createElement('div');
        dlg.className = 'notes-palette';
        dlg.setAttribute('role', 'dialog');
        dlg.setAttribute('aria-modal', 'true');
        dlg.setAttribute('aria-label', opts.ariaLabel || '');
        var footer = Array.isArray(opts.footer) && opts.footer.length ? opts.footer : null;
        dlg.innerHTML = '<div class="notes-palette-field"><input type="text" class="notes-palette-input" role="combobox" ' +
            'aria-expanded="true" aria-autocomplete="list" autocomplete="off" spellcheck="false" aria-controls="' + id + '-list" ' +
            'aria-label="' + esc(opts.ariaLabel || opts.placeholder || '') + '" placeholder="' + esc(opts.placeholder || '') + '"></div>' +
            '<div class="notes-palette-list" role="listbox" id="' + id + '-list" aria-label="' + esc(opts.ariaLabel || '') + '"></div>' +
            (footer ? '<div class="notes-palette-footer">' + footer.map(function (f) {
                return '<span><kbd>' + esc(f.keys) + '</kbd> ' + esc(f.label) + '</span>';
            }).join('') + '</div>' : '');
        var input = dlg.querySelector('input');
        var list = dlg.querySelector('.notes-palette-list');
        input.value = opts.initialQuery || '';

        function query() { return input.value; }

        function compute() {
            var q = query(), items = [];
            try { items = (opts.items && opts.items(q)) || []; } catch (e) { items = []; }
            rows = items.slice(0, MAX_ROWS).map(function (item) { return { item: item }; });
            var cl = typeof opts.createLabel === 'function' ? opts.createLabel(q) : null;
            if (typeof cl === 'string') rows.push({ create: true, label: cl });
        }

        function paint(keepId) {
            var prevId = keepId && rows[active] && rows[active].item ? rows[active].item.id : null;
            compute();
            var q = query();
            if (prevId != null) {
                var idx = rows.findIndex(function (r) { return r.item && r.item.id === prevId; });
                active = idx >= 0 ? idx : 0;
            }
            if (active >= rows.length) active = Math.max(0, rows.length - 1);
            if (!rows.length) {
                list.innerHTML = '<div class="notes-palette-empty">' + esc(opts.emptyText || '') + '</div>';
                input.removeAttribute('aria-activedescendant');
                return;
            }
            list.innerHTML = rows.map(function (r, i) {
                var cls = 'notes-palette-row' + (r.create ? ' is-create' : '') + (i === active ? ' is-active' : '');
                var attrs = 'class="' + cls + '" role="option" id="' + id + '-o' + i + '" data-i="' + i + '" aria-selected="' + (i === active) + '"';
                if (r.create) return '<div ' + attrs + '><span class="notes-palette-ico"></span><span class="notes-palette-main"><span class="notes-palette-label">' + esc(r.label) + '</span></span></div>';
                var it = r.item;
                return '<div ' + attrs + '><span class="notes-palette-ico">' + (it.icon || '') + '</span>' +
                    '<span class="notes-palette-main"><span class="notes-palette-label">' + labelHtml(it.label, q) + '</span>' +
                    (it.detail ? '<span class="notes-palette-detail">' + esc(it.detail) + '</span>' : '') + '</span>' +
                    (it.hint ? '<kbd class="notes-palette-hint">' + esc(it.hint) + '</kbd>' : '') + '</div>';
            }).join('');
            input.setAttribute('aria-activedescendant', id + '-o' + active);
            scrollActive();
        }

        function scrollActive() {
            var el = list.querySelector('.is-active');
            if (el && el.scrollIntoView) el.scrollIntoView({ block: 'nearest' });
        }

        function setActive(i, scroll) {
            if (!rows.length) return;
            var n = rows.length;
            active = ((i % n) + n) % n;
            var els = list.querySelectorAll('.notes-palette-row');
            for (var k = 0; k < els.length; k++) {
                var on = k === active;
                els[k].classList.toggle('is-active', on);
                els[k].setAttribute('aria-selected', on ? 'true' : 'false');
            }
            input.setAttribute('aria-activedescendant', id + '-o' + active);
            if (scroll !== false) scrollActive();
        }

        function close(silent) {
            if (closed) return;
            closed = true;
            if (host.__notesPalette === api) host.__notesPalette = null;
            doc.removeEventListener('keydown', onDocKey, true);
            if (backdrop.parentNode) backdrop.parentNode.removeChild(backdrop);
            if (dlg.parentNode) dlg.parentNode.removeChild(dlg);
            if (!silent && prevFocus && prevFocus.focus && prevFocus.isConnected !== false) {
                try { prevFocus.focus(); } catch (e) { /* element gone */ }
            }
        }

        function pick(i, e) {
            var r = rows[i];
            if (!r) return;
            var mods = { shift: !!(e && e.shiftKey), ctrl: !!(e && (e.ctrlKey || e.metaKey)) };
            var q = query();
            close();
            if (r.create) { if (opts.onCreate) opts.onCreate(q); }
            else if (opts.onPick) opts.onPick(r.item, mods);
        }

        function createRowIndex() {
            for (var i = rows.length - 1; i >= 0; i--) if (rows[i].create) return i;
            return -1;
        }

        function onKey(e) {
            var key = e.key;
            if (key === 'ArrowDown') { e.preventDefault(); setActive(active + 1); }
            else if (key === 'ArrowUp') { e.preventDefault(); setActive(active - 1); }
            else if (key === 'PageDown') { e.preventDefault(); setActive(Math.min(rows.length - 1, active + 8)); }
            else if (key === 'PageUp') { e.preventDefault(); setActive(Math.max(0, active - 8)); }
            else if (key === 'Enter') {
                e.preventDefault();
                if (e.shiftKey && createRowIndex() >= 0) pick(createRowIndex(), e);
                else pick(active, e);
            } else if (key === 'Escape') { e.preventDefault(); e.stopPropagation(); close(); }
            else if (key === 'Tab') { e.preventDefault(); input.focus(); }
        }
        // Capture on the document so host shortcuts (and the editor) never see keys meant for the palette.
        function onDocKey(e) {
            if (!dlg.contains(e.target)) {
                if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); close(); }
                return;
            }
            onKey(e);
        }
        doc.addEventListener('keydown', onDocKey, true);

        input.addEventListener('input', function () { active = 0; paint(false); });
        list.addEventListener('mousedown', function (e) { e.preventDefault(); });
        list.addEventListener('mousemove', function (e) {
            var row = e.target.closest && e.target.closest('.notes-palette-row');
            if (row && +row.getAttribute('data-i') !== active) setActive(+row.getAttribute('data-i'), false);
        });
        list.addEventListener('click', function (e) {
            var row = e.target.closest && e.target.closest('.notes-palette-row');
            if (row) pick(+row.getAttribute('data-i'), e);
        });
        backdrop.addEventListener('mousedown', function (e) { e.preventDefault(); close(); });

        var api = {
            close: close,
            update: function () { if (!closed) paint(true); }
        };
        host.__notesPalette = api;
        host.appendChild(backdrop);
        host.appendChild(dlg);
        paint(false);
        input.focus();
        input.select();
        return api;
    }

    return { open: open, matchPositions: matchPositions };
});
