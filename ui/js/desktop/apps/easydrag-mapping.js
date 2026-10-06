// EasyDrag input tree: the data of upstream nodes (from a run or schema examples). Rows are
// draggable into parameter fields; a click inserts into the field that was focused last.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const PAGE = 100;

    function typeOf(value) {
        if (value === null || value === undefined) return 'null';
        if (Array.isArray(value)) return 'list';
        if (typeof value === 'object') return value.$type === 'file' ? 'file' : 'object';
        return typeof value === 'string' ? 'text' : typeof value === 'number' ? 'number' : typeof value === 'boolean' ? 'bool' : 'text';
    }

    function preview(value) {
        const ty = typeOf(value);
        if (ty === 'text') return value.length > 80 ? value.slice(0, 79) + '…' : value;
        if (ty === 'number' || ty === 'bool') return String(value);
        if (ty === 'null') return '—';
        if (ty === 'file') return value.name || value.path || '';
        if (ty === 'list') return '[' + value.length + ']';
        return '{' + Object.keys(value).length + '}';
    }

    // create returns {el, refresh(env)}. env: {ed, sources: [{id, label, roots, kind}], sourceId,
    // upstream: [{key, label, icon, cat, fields: [{name, type}]}], onInsert(ref), onSource(id)}.
    function create(initial) {
        const core = ED.core;
        let env = initial;
        const { t, esc } = env.ed;
        const expanded = new Set();
        const pages = new Map();
        const el = core.el('<div class="ed-inputs"><div class="ed-col-head"><h3>' + esc(t('easydrag.ui.detail_input')) + '</h3><select class="ed-input ed-input--small" aria-label="' + esc(t('easydrag.ui.input_source')) + '"></select></div>' +
            '<div class="ed-tree" role="tree" aria-label="' + esc(t('easydrag.ui.detail_input')) + '"></div><p class="ed-tree-hint">' + esc(t('easydrag.ui.input_hint')) + '</p></div>');
        const select = el.querySelector('select');
        const tree = el.querySelector('.ed-tree');
        // The tree is one tab stop (roving tabindex): stop is the key of the row or "more"
        // button that holds it.
        let stop = null;

        function navItems() { return Array.from(tree.querySelectorAll('.ed-tree-row, .ed-tree-more')); }

        function keyOf(node) { return node.classList.contains('ed-tree-more') ? 'more:' + node.dataset.edMore : 'row:' + node.dataset.ref; }

        // setStop gives the tab stop to node (null: the remembered item, else the first one).
        function setStop(node) {
            const items = navItems();
            const target = node || items.find(n => keyOf(n) === stop) || items[0] || null;
            if (target) stop = keyOf(target);
            items.forEach(n => n.setAttribute('tabindex', n === target ? '0' : '-1'));
            return target;
        }

        // redraw renders again after a toggle or "more" and puts focus back, which the new
        // markup would drop to the page: on the item with key, else on the item at index (the
        // first row that "more" revealed).
        function redraw(key, index) {
            render();
            const items = navItems();
            const node = items.find(n => keyOf(n) === key) || items[index] || null;
            if (node) { setStop(node); node.focus(); }
        }

        function currentRoots() {
            const src = env.sources.find(s => s.id === env.sourceId) || env.sources[0];
            return src ? src.roots : {};
        }

        function schemaExample(u) {
            const out = {};
            (u.fields || []).forEach(f => {
                out[f.name] = { list: [], object: {}, number: 0, bool: false, file: { $type: 'file', name: 'file.pdf', path: '' } }[f.type] ?? '';
            });
            return out;
        }

        function rowMarkup(label, ref, value, depth, canExpand) {
            const ty = typeOf(value);
            const open = expanded.has(ref);
            return '<div class="ed-tree-row" role="treeitem" aria-level="' + (depth + 1) + '"' + (canExpand ? ' aria-expanded="' + open + '"' : '') + ' draggable="' + (!env.ed.readonly) + '" tabindex="-1" data-ref="' + esc(ref) + '" style="--depth:' + depth + '">' +
                (canExpand ? '<button type="button" class="ed-tree-toggle" data-ed-toggle="' + esc(ref) + '" tabindex="-1" aria-hidden="true">' + core.icon(open ? 'chevron-down' : 'chevron-right') + '</button>' : '<span class="ed-tree-spacer"></span>') +
                '<span class="ed-tree-key">' + esc(label) + '</span><span class="ed-type ed-type--' + ty + '">' + esc(core.tr(t, 'easydrag.ui.type_' + ty, ty)) + '</span>' +
                '<span class="ed-tree-value">' + esc(preview(value)) + '</span></div>';
        }

        function children(ref, value, depth) {
            if (!expanded.has(ref)) return '';
            const ty = typeOf(value);
            if (ty !== 'list' && ty !== 'object' && ty !== 'file') return '';
            const entries = ty === 'list' ? value.map((v, i) => [i, v]) : Object.entries(value);
            const limit = pages.get(ref) || PAGE;
            let html = entries.slice(0, limit).map(([k, v]) => {
                const childRef = ED.template.pathJoin(ref, k);
                const vt = typeOf(v);
                const expandable = vt === 'list' || vt === 'object' || vt === 'file';
                return rowMarkup(typeof k === 'number' ? '[' + k + ']' : k, childRef, v, depth + 1, expandable) + (expandable ? children(childRef, v, depth + 1) : '');
            }).join('');
            if (entries.length > limit) {
                html += '<button type="button" class="ed-tree-more" tabindex="-1" data-ed-more="' + esc(ref) + '" style="--depth:' + (depth + 1) + '">' + esc(t('easydrag.ui.tree_more', { count: entries.length - limit })) + '</button>';
            }
            return html;
        }

        function render() {
            select.innerHTML = env.sources.map(s => '<option value="' + esc(s.id) + '"' + (s.id === env.sourceId ? ' selected' : '') + '>' + esc(s.label) + '</option>').join('');
            select.hidden = env.sources.length < 2;
            const roots = currentRoots();
            const blocks = [];
            const groups = env.upstream.concat([{ key: 'trigger', label: core.tr(t, 'easydrag.ui.root_trigger', 'trigger'), icon: 'bolt', cat: 'trigger', fields: [{ name: 'data', type: 'object' }, { name: 'fired_at', type: 'text' }] }]);
            groups.forEach(u => {
                const hasData = Object.prototype.hasOwnProperty.call(roots, u.key) && roots[u.key] !== undefined;
                const value = hasData ? roots[u.key] : schemaExample(u);
                if (!expanded.has(u.key) && !expanded.has('!' + u.key)) expanded.add(u.key);
                const open = expanded.has(u.key);
                blocks.push('<div class="ed-tree-group" data-cat="' + esc(u.cat) + '">' +
                    '<div class="ed-tree-row ed-tree-root" role="treeitem" aria-level="1" aria-expanded="' + open + '" draggable="' + (!env.ed.readonly) + '" tabindex="-1" data-ref="' + esc(u.key) + '">' +
                    '<button type="button" class="ed-tree-toggle" data-ed-toggle="' + esc(u.key) + '" tabindex="-1" aria-hidden="true">' + core.icon(open ? 'chevron-down' : 'chevron-right') + '</button>' +
                    '<span class="ed-tree-tile">' + core.icon(u.icon) + '</span><span class="ed-tree-key">' + esc(u.label) + '</span>' +
                    (hasData ? '' : '<span class="ed-chip ed-chip--muted">' + esc(t('easydrag.ui.input_example')) + '</span>') + '</div>' +
                    children(u.key, value, 0) + '</div>');
            });
            tree.innerHTML = blocks.length ? blocks.join('') : '<p class="ed-tree-empty">' + esc(t('easydrag.ui.input_none')) + '</p>';
            setStop(null);
        }

        el.addEventListener('click', (event) => {
            const toggle = event.target.closest('[data-ed-toggle]');
            if (toggle) {
                const ref = toggle.dataset.edToggle;
                if (expanded.has(ref)) { expanded.delete(ref); if (!ref.includes('.') && !ref.includes('[')) expanded.add('!' + ref); }
                else { expanded.add(ref); expanded.delete('!' + ref); }
                redraw('row:' + ref, -1);
                return;
            }
            const more = event.target.closest('[data-ed-more]');
            if (more) {
                const at = navItems().indexOf(more);
                pages.set(more.dataset.edMore, (pages.get(more.dataset.edMore) || PAGE) + PAGE * 4);
                redraw(null, at);
                return;
            }
            const row = event.target.closest('.ed-tree-row');
            if (row && !env.ed.readonly) env.onInsert(row.dataset.ref);
        });
        tree.addEventListener('focusin', (event) => {
            const item = event.target.closest('.ed-tree-row, .ed-tree-more');
            if (item && item.getAttribute('tabindex') !== '0') setStop(item);
        });
        el.addEventListener('keydown', (event) => {
            const item = event.target.closest('.ed-tree-row, .ed-tree-more');
            if (!item) return;
            const items = navItems();
            const idx = items.indexOf(item);
            const go = node => { event.preventDefault(); setStop(node); node.focus(); };
            if (event.key === 'ArrowDown' && items[idx + 1]) go(items[idx + 1]);
            else if (event.key === 'ArrowUp' && items[idx - 1]) go(items[idx - 1]);
            else if (event.key === 'Home' && items.length) go(items[0]);
            else if (event.key === 'End' && items.length) go(items[items.length - 1]);
            else if (item.classList.contains('ed-tree-more')) return;
            else if (event.key === 'ArrowRight' || event.key === 'ArrowLeft') {
                const tg = item.querySelector('[data-ed-toggle]');
                if (tg && (item.getAttribute('aria-expanded') === 'true') === (event.key === 'ArrowLeft')) { event.preventDefault(); tg.click(); }
            } else if (event.key === 'Enter' && !env.ed.readonly) { event.preventDefault(); env.onInsert(item.dataset.ref); }
        });
        el.addEventListener('dragstart', (event) => {
            const row = event.target.closest('.ed-tree-row');
            if (!row) return;
            event.dataTransfer.setData('application/x-easydrag-ref', row.dataset.ref);
            event.dataTransfer.setData('text/plain', '{{' + row.dataset.ref + '}}');
            event.dataTransfer.effectAllowed = 'copy';
            const root = env.ed.root;
            root.classList.add('is-mapping');
            // One dragend on the document ends the mapping look; the forms drop handler clears
            // it too, for a row redrawn during the drag (its dragend never reaches the page).
            const clear = () => { root.classList.remove('is-mapping'); document.removeEventListener('dragend', clear, true); };
            document.addEventListener('dragend', clear, true);
        });
        select.addEventListener('change', () => { env.sourceId = select.value; if (env.onSource) env.onSource(select.value); render(); });

        render();
        return {
            el,
            refresh(next) { env = Object.assign({}, env, next); render(); },
            roots: currentRoots
        };
    }

    ED.mapping = { create, typeOf, preview };
})();
