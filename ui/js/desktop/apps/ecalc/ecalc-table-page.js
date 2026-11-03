/* ECalc table page: the title with favorite and copy, the guide-values and mains banners, an optional finder header (the
   finder calculator of the table, drawn by ECalcCalcPage in its compact header mode; after every solve the row whose
   `finder.column` equals the solved `finder.var` is highlighted and scrolled into view), a filter box that matches any
   shown cell text, sortable columns (asc, desc, table order), a unit dropdown per column from its `units` list, the table
   with a sticky header inside a scroll area, and the notes, the explanation and the source under it. Numbers are the SI
   values of the catalog JSON shown through ECalcCore.units.format with the significant-digit setting; null is a dash.
   A column whose cells are all empty carries no verified data: it is not shown (and not filtered, sorted or copied) and a line
   under the table says how many columns are hidden. The unit shown in a column is remembered per table and column
   (ECalcStore units under the table id).
   "Copy table" puts the visible rows, in the shown units, on the clipboard as tab-separated text.
   options: {item, units, store, t, icon, uid, decimalComma, schematics, getItem, onFavorite} (the options object of every
   page creator, see ecalc-shell.js); returns {el, layout, refresh, focusFilter, dispose}.
   Publishes window.ECalcTablePage = { create }. */
(function () {
    'use strict';

    const Typeset = window.ECalcTypeset, esc = Typeset.escape;
    const NO_VALUE = '—';
    const MATCH_TOLERANCE = 1e-9; // relative: a cell is the solved value when it equals it within this
    const NARROW_BELOW = 760; // under this width the action buttons show their icon only
    const COPIED_MS = 1600;

    // Two table values are the same row key when they agree within MATCH_TOLERANCE relative to the larger of them.
    function sameValue(a, b) {
        if (typeof a !== 'number' || typeof b !== 'number' || !Number.isFinite(a) || !Number.isFinite(b)) return false;
        return Math.abs(a - b) <= MATCH_TOLERANCE * Math.max(Math.abs(a), Math.abs(b));
    }
    // The first row whose cell in column `index` is `value` (see sameValue), or -1.
    function findRow(rows, index, value) {
        if (index < 0 || !Number.isFinite(value)) return -1;
        return rows.findIndex(row => sameValue(row[index], value));
    }
    // A table cell (or filter) written on one line: tabs and line breaks would break the tab-separated copy.
    const oneLine = text => String(text).replace(/[\t\r\n]+/g, ' ');

    // The options object of the finder header keeps its own stored inputs per table ("<calculator>@<table>"), so the fixed
    // choices of one table never reach the calculator's page or another table that uses the same finder.
    function scopedStore(store, key) {
        return Object.assign(Object.create(store), {
            calcState: () => store.calcState(key),
            saveCalc: (id, state) => store.saveCalc(key, state),
            unitOf: (id, varId) => store.unitOf(key, varId),
            setUnit: (id, varId, unit) => store.setUnit(key, varId, unit)
        });
    }

    function create(options) {
        const { item: table, units, store, t, icon, uid } = options;
        const Page = window.ECalcCalcPage;
        const columns = table.columns || [], rows = table.rows || [];
        const numeric = columns.map(column => column.dim !== 'text');
        // The columns that have at least one cell (all of them when there are no rows or no column has a cell).
        const filled = columns.map((column, index) => index).filter(index => rows.some(row => row[index] !== null && row[index] !== undefined));
        const shownColumns = rows.length && filled.length ? filled : columns.map((column, index) => index);
        const hiddenCount = columns.length - shownColumns.length;
        const columnIndex = new Map(columns.map((column, index) => [column.id, index]));
        const keyIndex = columnIndex.has(table.key) ? columnIndex.get(table.key) : 0;
        const comma = text => (options.decimalComma ? text.replace(/(\d)\.(\d)/g, '$1,$2') : text);
        const sig = () => store.prefs().sig;
        // The columns look like the variables of a calculator, so the unit bookkeeping of the calculator pages applies.
        const unitOf = Page.unitFor({ id: table.id, vars: columns }, store, units);
        let collator;
        try { collator = new Intl.Collator(document.documentElement.lang || undefined, { numeric: true, sensitivity: 'base' }); } catch (error) { collator = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' }); }

        // The unit choices of a numeric column: only a column with a `units` list offers a dropdown.
        const offered = column => (numeric[columns.indexOf(column)] && Array.isArray(column.units) && column.units.length ? Page.unitChoices(column, units) : []);
        // The symbol written in the header instead of in every cell: a fixed unit or prefix, nothing for the automatic prefix.
        const headLabel = column => {
            if (!numeric[columns.indexOf(column)]) return '';
            const id = unitOf(column.id);
            if (!id) return '';
            const found = units.options(column.dim).find(option => option.id === id);
            return found ? found.label : '';
        };
        function cellText(column, value) {
            if (value === null || value === undefined) return NO_VALUE;
            if (typeof value === 'string') return value;
            const id = unitOf(column.id);
            let text = units.format(value, column.dim, id, sig());
            if (!text) return comma(units.format(value, column.dim, '', sig()) || String(value));
            const label = headLabel(column);
            if (label && text.endsWith(' ' + label)) text = text.slice(0, text.length - label.length - 1);
            return comma(text);
        }

        // ---- state ---------------------------------------------------------------------------------------------
        let query = '', sort = { index: -1, dir: 0 }, shown = [], matched = -1, finderValue = NaN, headerPage = null;
        let revealPending = false, copyTimer = 0;
        const finder = table.finder && typeof table.finder === 'object' ? table.finder : null;
        const finderIndex = finder && columnIndex.has(finder.column) ? columnIndex.get(finder.column) : -1;
        const finderCalc = finder && typeof options.getItem === 'function' ? options.getItem(finder.calc) : null;

        const paragraphs = text => String(text || '').split(/\n{2,}/).filter(part => part.trim()).map(part => '<p>' + esc(part.trim()) + '</p>').join('');
        const favoriteLabel = () => (store.isFavorite(table.id) ? t('desktop.ecalc_favorite_remove') : t('desktop.ecalc_favorite_add'));

        // ---- markup --------------------------------------------------------------------------------------------
        const el = document.createElement('article');
        el.className = 'ec-page ec-table-page';
        el.dataset.table = table.id;
        el.dataset.hasFinder = finderIndex >= 0 && finderCalc ? 'true' : 'false';
        el.dataset.narrow = 'false';
        el.dataset.hiddenColumns = String(hiddenCount);
        el.tabIndex = -1;
        el.setAttribute('aria-labelledby', uid + '-title');
        el.innerHTML = '<header class="ec-page-head"><h2 class="ec-title" id="' + uid + '-title">' + esc(table.name) + '</h2><div class="ec-actions">'
            + '<button type="button" class="ec-btn" data-action="favorite" aria-pressed="false">' + icon('star') + '<span class="ec-btn-label" data-label="favorite"></span></button>'
            + '<button type="button" class="ec-btn" data-action="copy" title="' + esc(t('desktop.ecalc_table_copy')) + '" aria-label="' + esc(t('desktop.ecalc_table_copy')) + '">' + icon('copy') + '<span class="ec-btn-label" data-label="copy">' + esc(t('desktop.ecalc_table_copy')) + '</span></button></div></header>'
            + (table.mains ? '<p class="ec-banner" data-kind="mains" role="note">' + icon('bolt') + '<span>' + esc(t('desktop.ecalc_mains_note')) + '</span></p>' : '')
            + (table.guide ? '<p class="ec-banner" data-kind="guide" role="note">' + icon('info') + '<span>' + esc(t('desktop.ecalc_guide_values')) + '</span></p>' : '')
            + '<section class="ec-finder" data-block="finder" hidden></section>'
            + '<section class="ec-table-wrap"><div class="ec-table-bar"><label class="ec-filter">' + icon('search')
            + '<input class="ec-filter-input" type="search" autocomplete="off" spellcheck="false" maxlength="64" placeholder="' + esc(t('desktop.ecalc_table_filter')) + '" aria-label="' + esc(t('desktop.ecalc_table_filter')) + '"><kbd class="ec-kbd">/</kbd></label>'
            + '<span class="ec-table-count" role="status" aria-live="polite"></span><span class="ec-table-hint"></span></div>'
            + '<div class="ec-table-scroll" role="region" tabindex="0" aria-label="' + esc(table.name) + '"><table class="ec-table"><caption class="ec-sr">' + esc(table.name) + '</caption><thead></thead><tbody></tbody></table></div></section>'
            + (hiddenCount ? '<p class="ec-table-hidden" role="note">' + esc(t('desktop.ecalc_table_columns_hidden', { count: hiddenCount })) + '</p>' : '')
            + (table.notes ? '<section class="ec-table-notes"><h3>' + esc(t('desktop.ecalc_table_notes')) + '</h3>' + paragraphs(table.notes) + '</section>' : '')
            + (table.doc ? '<details class="ec-doc"><summary>' + esc(t('desktop.ecalc_explanation')) + '</summary><div class="ec-doc-body">' + paragraphs(table.doc) + '</div></details>' : '')
            + '<p class="ec-source"><strong>' + esc(t('desktop.ecalc_source')) + ':</strong> ' + esc(table.source || '') + '</p>';

        const finderEl = el.querySelector('.ec-finder'), filterEl = el.querySelector('.ec-filter-input'), countEl = el.querySelector('.ec-table-count');
        const hintEl = el.querySelector('.ec-table-hint'), scrollEl = el.querySelector('.ec-table-scroll'), headEl = el.querySelector('thead'), bodyEl = el.querySelector('tbody');

        function renderHead() {
            headEl.innerHTML = '<tr>' + shownColumns.map(index => {
                const column = columns[index];
                const choices = offered(column), current = choices.length ? unitOf(column.id) : '';
                const unit = choices.length > 1
                    ? '<select class="ec-th-unit" data-unit="' + index + '" aria-label="' + esc(t('desktop.ecalc_table_unit', { name: column.name })) + '" title="' + esc(t('desktop.ecalc_table_unit', { name: column.name })) + '">'
                        + choices.map(option => '<option value="' + esc(option.id) + '"' + (option.id === current ? ' selected' : '') + '>' + esc(option.kind === 'auto' ? t('desktop.ecalc_unit_auto') : option.label) + '</option>').join('') + '</select>'
                    : headLabel(column) ? '<span class="ec-th-unit-static">' + esc(headLabel(column)) + '</span>' : '';
                return '<th scope="col" data-col="' + esc(column.id) + '" data-dim="' + esc(column.dim) + '" aria-sort="none"' + (numeric[index] ? ' data-num="true"' : '') + '><div class="ec-th">'
                    + '<button type="button" class="ec-th-sort" data-sort="' + index + '" title="' + esc(t('desktop.ecalc_table_sort')) + '">'
                    + (column.label ? '<span class="ec-th-label">' + Typeset.labelHTML(column.label) + '</span>' : '') + '<span class="ec-th-name">' + esc(column.name) + '</span><span class="ec-sort-mark" aria-hidden="true"></span></button>' + unit + '</div></th>';
            }).join('') + '</tr>';
            paintSort();
        }
        function paintSort() {
            for (const th of headEl.querySelectorAll('th')) {
                const index = columns.findIndex(column => column.id === th.dataset.col);
                th.setAttribute('aria-sort', sort.index === index && sort.dir ? (sort.dir > 0 ? 'ascending' : 'descending') : 'none');
            }
        }

        // The rows that pass the filter, in the order of the sort. `texts` are the shown cell texts of every row.
        let texts = [];
        function computeView() {
            texts = rows.map(row => row.map((value, index) => cellText(columns[index], value)));
            const needle = query.trim().toLowerCase();
            shown = rows.map((row, index) => index).filter(index => !needle || shownColumns.some(at => texts[index][at].toLowerCase().includes(needle)));
            if (sort.index >= 0 && sort.dir) {
                const at = sort.index, isNumber = numeric[at];
                shown.sort((a, b) => {
                    const left = rows[a][at], right = rows[b][at];
                    // Empty cells sort last in both directions.
                    if ((left === null) !== (right === null)) return left === null ? 1 : -1;
                    const order = left === null ? 0 : isNumber ? left - right : collator.compare(texts[a][at], texts[b][at]);
                    return (order * sort.dir) || (a - b);
                });
            }
        }
        function renderBody() {
            computeView();
            bodyEl.innerHTML = shown.length ? shown.map(index => '<tr data-row="' + index + '">' + shownColumns.map(at => {
                const value = rows[index][at], tag = at === keyIndex ? 'th scope="row"' : 'td';
                return '<' + tag + (numeric[at] ? ' data-num="true"' : '') + (value === null ? ' data-empty="true"' : '') + '>' + esc(texts[index][at]) + '</' + (at === keyIndex ? 'th' : 'td') + '>';
            }).join('') + '</tr>').join('') : '<tr class="ec-table-empty"><td colspan="' + Math.max(shownColumns.length, 1) + '">' + esc(t('desktop.ecalc_table_no_rows')) + '</td></tr>';
            countEl.textContent = query.trim() ? t('desktop.ecalc_table_rows_filtered', { shown: shown.length, total: rows.length }) : t('desktop.ecalc_table_rows', { total: rows.length });
            paintMatch(false);
        }

        // The row of the finder result: marked in place (the rows are not drawn again for every keystroke in the finder).
        function paintMatch(reveal) {
            const before = matched;
            matched = finder ? findRow(rows, finderIndex, finderValue) : -1;
            for (const row of bodyEl.querySelectorAll('tr[data-match]')) { row.removeAttribute('data-match'); row.removeAttribute('aria-current'); }
            const rowEl = matched >= 0 ? bodyEl.querySelector('tr[data-row="' + matched + '"]') : null;
            if (rowEl) {
                rowEl.dataset.match = 'true';
                rowEl.setAttribute('aria-current', 'true');
                if (reveal && (matched !== before || revealPending)) revealRow(rowEl);
            }
            hintEl.textContent = finder && Number.isFinite(finderValue) && matched < 0 ? t('desktop.ecalc_table_no_match') : '';
        }
        // Puts the row in the middle of the scroll area (below the sticky header) when it is not visible; before the page has a
        // size (it is not attached yet) the scroll waits for layout().
        function revealRow(rowEl) {
            const area = scrollEl.getBoundingClientRect();
            if (!area.height) { revealPending = true; return; }
            revealPending = false;
            const head = headEl.getBoundingClientRect().height, box = rowEl.getBoundingClientRect();
            if (box.top >= area.top + head && box.bottom <= area.bottom) return;
            scrollEl.scrollTop += (box.top - area.top) - Math.max(head, (area.height - box.height) / 2);
        }

        function paintActions() {
            const button = el.querySelector('[data-action="favorite"]'), on = store.isFavorite(table.id);
            button.setAttribute('aria-pressed', on ? 'true' : 'false');
            button.querySelector('[data-label]').textContent = favoriteLabel();
            button.title = favoriteLabel();
            button.setAttribute('aria-label', favoriteLabel());
        }

        // The visible rows in the shown units, tab-separated; the header names the fixed unit of a column in brackets.
        function plainText() {
            const head = shownColumns.map(at => oneLine(columns[at].name) + (headLabel(columns[at]) ? ' [' + headLabel(columns[at]) + ']' : ''));
            return [head.join('\t')].concat(shown.map(index => shownColumns.map(at => oneLine(texts[index][at])).join('\t'))).join('\n');
        }

        // ---- events --------------------------------------------------------------------------------------------
        filterEl.addEventListener('input', () => { query = filterEl.value; renderBody(); });
        filterEl.addEventListener('keydown', event => {
            if (event.key === 'Escape' && filterEl.value) {
                event.preventDefault();
                event.stopPropagation();
                filterEl.value = '';
                query = '';
                renderBody();
            }
        });
        el.addEventListener('change', event => {
            const select = event.target.closest('.ec-th-unit');
            if (!select || !el.contains(select)) return;
            store.setUnit(table.id, columns[Number(select.dataset.unit)].id, select.value || 'auto');
            renderBody();
        });
        el.addEventListener('click', event => {
            const sortButton = event.target.closest('[data-sort]');
            if (sortButton && el.contains(sortButton)) {
                const index = Number(sortButton.dataset.sort);
                // Ascending, descending, then the order of the table again.
                sort = sort.index !== index ? { index, dir: 1 } : sort.dir === 1 ? { index, dir: -1 } : { index: -1, dir: 0 };
                paintSort();
                renderBody();
                return;
            }
            const button = event.target.closest('[data-action]');
            if (!button || !el.contains(button)) return;
            if (button.dataset.action === 'favorite') {
                store.toggleFavorite(table.id);
                paintActions();
                if (options.onFavorite) options.onFavorite(table.id);
            } else if (button.dataset.action === 'copy') {
                Page.copyText(plainText());
                const label = button.querySelector('[data-label]');
                button.dataset.done = 'true';
                label.textContent = t('desktop.ecalc_copied');
                button.title = t('desktop.ecalc_copied');
                clearTimeout(copyTimer);
                copyTimer = setTimeout(() => {
                    delete button.dataset.done;
                    label.textContent = t('desktop.ecalc_table_copy');
                    button.title = t('desktop.ecalc_table_copy');
                }, COPIED_MS);
            }
        });

        renderHead();
        renderBody();
        paintActions();

        // The finder header goes in last: its first solve already reports a value, and the rows exist by now.
        if (finderIndex >= 0 && finderCalc && Page.create) {
            headerPage = Page.create({
                item: finderCalc, units, store: scopedStore(store, finderCalc.id + '@' + table.id), t, icon, uid: uid + '-finder',
                decimalComma: options.decimalComma, schematics: options.schematics,
                header: {
                    choices: finder.choices || {},
                    onResult(result) {
                        const value = result && result.values ? result.values[finder.var] : NaN;
                        finderValue = typeof value === 'number' ? value : NaN;
                        paintMatch(true);
                    }
                }
            });
            finderEl.hidden = false;
            finderEl.appendChild(headerPage.el);
        }

        return {
            el,
            // The shell calls this whenever the page host changes its size and once the page is attached.
            layout() {
                const width = el.clientWidth;
                if (!width) return;
                el.dataset.narrow = width < NARROW_BELOW ? 'true' : 'false';
                if (headerPage) headerPage.layout();
                if (revealPending && matched >= 0) {
                    const rowEl = bodyEl.querySelector('tr[data-row="' + matched + '"]');
                    if (rowEl) revealRow(rowEl);
                }
            },
            // The settings changed (digits, E-series, symbol standard): the numbers are written again.
            refresh() {
                paintActions();
                renderBody();
                if (headerPage) headerPage.refresh();
            },
            focusFilter() {
                filterEl.focus();
                filterEl.select();
            },
            dispose() {
                clearTimeout(copyTimer);
                if (headerPage) headerPage.dispose();
                headerPage = null;
                store.flush();
            }
        };
    }

    window.ECalcTablePage = { create };
})();
