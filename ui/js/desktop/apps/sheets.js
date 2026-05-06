(function () {
    'use strict';

    const DEFAULT_PATH = 'Documents/untitled.xlsx';
    const MIN_ROWS = 24;
    const MIN_COLS = 10;
    const instances = new Map();

    function render(host, windowId, context) {
        if (!host) return;
        instances.set(windowId, { container: host, closeContextMenu: () => closeSheetContextMenu() });
        const ctx = context || {};
        const esc = ctx.esc || (value => String(value == null ? '' : value));
        const rawT = ctx.t || ((key, vars) => interpolate(key, vars));
        const t = (key, fallback, vars) => {
            if (fallback && typeof fallback === 'object' && !Array.isArray(fallback)) {
                vars = fallback;
                fallback = '';
            }
            const translated = rawT(key, vars || {});
            return translated && translated !== key ? translated : (fallback || key);
        };
        const api = ctx.api || fetchJSON;
        const iconMarkup = ctx.iconMarkup || ((key, fallback) => `<span>${esc(fallback || key || '')}</span>`);
        const notify = ctx.notify || (() => {});
        const refreshDesktop = ctx.loadBootstrap || (() => Promise.resolve());
        const readonly = !!ctx.readonly;
        let currentPath = ctx.path || DEFAULT_PATH;
        let officeVersion = null;
        let activeSheet = 0;
        let workbook = emptyWorkbook(currentPath);
        let selection = { anchor: { row: 0, col: 0 }, focus: { row: 0, col: 0 } };
        let dragSelecting = false;
        let localClipboard = '';
        let contextMenu = null;
        let contextMenuOutsideHandler = null;

        host.innerHTML = `<div class="office-app office-sheets" data-office-sheets="${esc(windowId)}">
            <div class="vd-toolbar office-toolbar">
                <input class="office-path-input" data-path value="${esc(currentPath)}" spellcheck="false" autocomplete="off">
                <span class="vd-chat-meta" data-status>${esc(t('desktop.sheets_loading', 'Loading...'))}</span>
            </div>
            <div class="office-formula-bar" data-formula-bar>
                <output class="office-range-name" data-range-name>A1</output>
                <span class="office-formula-fx" aria-hidden="true">fx</span>
                <input class="office-formula-input" data-formula-input value="" spellcheck="false" autocomplete="off" placeholder="=SUM(A1:A3)">
                <button class="vd-tool-button office-formula-apply" type="button" data-action="apply-formula">${iconMarkup('check-square', 'OK', 'vd-tool-icon', 15)}<span>${esc(t('desktop.ok', 'OK'))}</span></button>
            </div>
            <div class="office-sheet-tabs" data-tabs></div>
            <div class="office-sheet-grid-wrap" data-grid></div>
        </div>`;

        const pathInput = host.querySelector('[data-path]');
        const status = host.querySelector('[data-status]');
        const tabsHost = host.querySelector('[data-tabs]');
        const gridHost = host.querySelector('[data-grid]');
        const rangeName = host.querySelector('[data-range-name]');
        const formulaInput = host.querySelector('[data-formula-input]');
        if (typeof ctx.wireContextMenuBoundary === 'function') ctx.wireContextMenuBoundary(host);

        function setStatus(message) {
            if (status) status.textContent = message || '';
        }

        function setPath(path) {
            currentPath = path || DEFAULT_PATH;
            pathInput.value = currentPath;
            updateExportLinks();
            if (typeof ctx.updateWindowContext === 'function') ctx.updateWindowContext(windowId, { path: currentPath });
        }

        function updateExportLinks() {
            setWindowMenus();
        }

        function renderWorkbook() {
            closeSheetContextMenu();
            workbook = normalizeWorkbook(workbook, pathInput.value.trim() || DEFAULT_PATH);
            activeSheet = Math.min(activeSheet, workbook.sheets.length - 1);
            const sheet = workbook.sheets[activeSheet];
            tabsHost.innerHTML = workbook.sheets.map((sheet, index) => `<button type="button" class="${index === activeSheet ? 'active' : ''}" data-sheet-index="${index}">${esc(sheet.name || (t('desktop.sheets_sheet', 'Sheet') + ' ' + (index + 1)))}</button>`).join('');
            tabsHost.querySelectorAll('[data-sheet-index]').forEach(btn => {
                btn.addEventListener('click', () => {
                    captureGrid();
                    activeSheet = Number(btn.dataset.sheetIndex) || 0;
                    selection = { anchor: { row: 0, col: 0 }, focus: { row: 0, col: 0 } };
                    renderWorkbook();
                });
            });
            const rows = padRows(sheet.rows || [], MIN_ROWS, MIN_COLS);
            clampSelection(rows.length, rows[0].length);
            const colHeaders = Array.from({ length: rows[0].length }, (_, i) => columnName(i + 1));
            gridHost.innerHTML = `<table class="office-grid">
                <thead><tr><th></th>${colHeaders.map((col, c) => `<th data-col-header="${c}">${esc(col)}</th>`).join('')}</tr></thead>
                <tbody>${rows.map((row, r) => `<tr><th data-row-header="${r}">${r + 1}</th>${row.map((cell, c) => `<td data-cell-row="${r}" data-cell-col="${c}" class="${esc(cellClass(r, c))}"><input data-row="${r}" data-col="${c}" ${cellInputAttributes(cell, sheet)} spellcheck="false" ${readonly ? 'readonly' : ''}></td>`).join('')}</tr>`).join('')}</tbody>
            </table>`;
            wireGrid();
            applyReadonlyState();
            renderSelection();
        }

        function wireGrid() {
            gridHost.querySelectorAll('input[data-row][data-col]').forEach(input => {
                input.addEventListener('pointerdown', event => {
                    if (event.button !== 0) return;
                    const row = Number(input.dataset.row);
                    const col = Number(input.dataset.col);
                    selectCell(row, col, event.shiftKey);
                    dragSelecting = true;
                    document.addEventListener('pointerup', () => { dragSelecting = false; }, { once: true });
                });
                input.addEventListener('pointerenter', () => {
                    if (!dragSelecting) return;
                    extendSelection(Number(input.dataset.row), Number(input.dataset.col));
                });
                input.addEventListener('focus', () => {
                    showFormulaForEditing(input);
                    const row = Number(input.dataset.row);
                    const col = Number(input.dataset.col);
                    if (!cellInSelection(row, col)) selectCell(row, col, false, false);
                    else updateFormulaBar();
                });
                input.addEventListener('blur', () => showFormulaResult(input));
                input.addEventListener('input', () => {
                    if (readonly) return;
                    const raw = input.value;
                    setCellFromInput(input, raw);
                    if (isFocusCell(Number(input.dataset.row), Number(input.dataset.col))) {
                        updateFormulaBar();
                    }
                });
                input.addEventListener('keydown', event => handleCellKeydown(event, input));
                input.addEventListener('contextmenu', event => {
                    event.preventDefault();
                    const row = Number(input.dataset.row);
                    const col = Number(input.dataset.col);
                    if (!cellInSelection(row, col)) selectCell(row, col, false);
                    showSheetContextMenu(event.clientX, event.clientY);
                });
            });
            gridHost.querySelectorAll('[data-row-header]').forEach(header => {
                header.addEventListener('click', () => selectRow(Number(header.dataset.rowHeader)));
            });
            gridHost.querySelectorAll('[data-col-header]').forEach(header => {
                header.addEventListener('click', () => selectColumn(Number(header.dataset.colHeader)));
            });
        }

        function handleCellKeydown(event, input) {
            if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'c') {
                event.preventDefault();
                copyRange();
                return;
            }
            if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'v') {
                event.preventDefault();
                pasteRange();
                return;
            }
            if (event.key === 'Delete') {
                event.preventDefault();
                clearRange();
                return;
            }
            const row = Number(input.dataset.row);
            const col = Number(input.dataset.col);
            const move = {
                ArrowUp: [row - 1, col],
                ArrowDown: [row + 1, col],
                ArrowLeft: [row, col - 1],
                ArrowRight: [row, col + 1],
                Enter: [row + 1, col],
                Tab: [row, col + (event.shiftKey ? -1 : 1)]
            }[event.key];
            if (!move) return;
            event.preventDefault();
            const target = cellInput(clampCellRow(move[0]), clampCellCol(move[1]));
            if (target) {
                target.focus();
                selectCell(Number(target.dataset.row), Number(target.dataset.col), event.shiftKey && event.key.startsWith('Arrow'));
            }
        }

        function captureGrid() {
            const sheet = workbook.sheets[activeSheet];
            if (!sheet) return;
            sheet.rows = trimRows(captureDisplayRows());
        }

        function captureDisplayRows() {
            const rows = [];
            gridHost.querySelectorAll('tbody tr').forEach((tr, r) => {
                const row = [];
                tr.querySelectorAll('input').forEach((input, c) => {
                    row[c] = cellFromInputElement(input);
                });
                rows[r] = row;
            });
            return rows;
        }

        function cellFromRaw(raw) {
            raw = String(raw == null ? '' : raw);
            return raw.startsWith('=') ? { formula: raw.slice(1) } : { value: raw };
        }

        function cellFromInputElement(input) {
            if (!input) return { value: '' };
            const formula = input.dataset.formula || '';
            const displayValue = input.dataset.displayValue || '';
            if (formula && (input.value === displayValue || input.value === '=' + formula)) {
                return { formula };
            }
            return cellFromRaw(input.value);
        }

        function setCellFromInput(input, raw) {
            if (!input) return null;
            if (readonly) return cellFromInputElement(input);
            raw = String(raw == null ? '' : raw);
            input.value = raw;
            const cell = cellFromRaw(raw);
            const sheet = workbook.sheets[activeSheet];
            if (!sheet) return cell;
            const row = Math.max(0, Number(input.dataset.row) || 0);
            const col = Math.max(0, Number(input.dataset.col) || 0);
            sheet.rows = Array.isArray(sheet.rows) ? sheet.rows : [];
            ensureRows(sheet.rows, row + 1, col + 1);
            sheet.rows[row][col] = cell;
            sheet.rows = trimRows(sheet.rows);
            syncFormulaDataset(input, cell, sheet);
            return cell;
        }

        function setSheetRows(rows) {
            const sheet = workbook.sheets[activeSheet];
            if (!sheet) return;
            sheet.rows = trimRows(rows);
            renderWorkbook();
        }

        function selectCell(row, col, extend, rerender) {
            row = Math.max(0, row);
            col = Math.max(0, col);
            if (extend) {
                selection.focus = { row, col };
            } else {
                selection = { anchor: { row, col }, focus: { row, col } };
            }
            if (rerender === false) {
                renderSelection();
                return;
            }
            renderSelection();
        }

        function extendSelection(row, col) {
            selection.focus = { row, col };
            renderSelection();
        }

        function selectRow(row) {
            const cols = Math.max(MIN_COLS, gridHost.querySelectorAll('thead th[data-col-header]').length);
            selection = { anchor: { row, col: 0 }, focus: { row, col: cols - 1 } };
            renderSelection();
        }

        function selectColumn(col) {
            const rows = Math.max(MIN_ROWS, gridHost.querySelectorAll('tbody tr').length);
            selection = { anchor: { row: 0, col }, focus: { row: rows - 1, col } };
            renderSelection();
        }

        function renderSelection() {
            const range = selectionRange();
            gridHost.querySelectorAll('td[data-cell-row][data-cell-col]').forEach(td => {
                const row = Number(td.dataset.cellRow);
                const col = Number(td.dataset.cellCol);
                td.classList.toggle('office-cell-selected', cellInRange(row, col, range));
                td.classList.toggle('office-cell-active', isFocusCell(row, col));
            });
            updateFormulaBar();
        }

        function updateFormulaBar() {
            if (rangeName) {
                rangeName.value = rangeLabel();
                rangeName.textContent = rangeLabel();
            }
            const input = cellInput(selection.focus.row, selection.focus.col);
            if (formulaInput && input && document.activeElement !== formulaInput) {
                formulaInput.value = input.value;
            }
        }

        function applyFormulaBar() {
            if (readonly) return;
            const input = cellInput(selection.focus.row, selection.focus.col);
            if (!input || !formulaInput) return;
            setCellFromInput(input, formulaInput.value);
            input.focus();
            updateFormulaBar();
        }

        function showSheetContextMenu(x, y) {
            closeSheetContextMenu();
            const items = [
                { action: 'copy-range', icon: 'copy', label: t('desktop.fm.copy', 'Copy') },
                { action: 'paste-range', icon: 'clipboard', label: t('desktop.fm.paste', 'Paste') },
                { action: 'clear-range', icon: 'x', label: t('desktop.sheets_clear_range', 'Clear contents') },
                { separator: true },
                { action: 'insert-row-above', icon: 'list', label: t('desktop.sheets_insert_row_above', 'Insert row above') },
                { action: 'insert-row-below', icon: 'list', label: t('desktop.sheets_insert_row_below', 'Insert row below') },
                { action: 'insert-col-left', icon: 'grid', label: t('desktop.sheets_insert_col_left', 'Insert column left') },
                { action: 'insert-col-right', icon: 'grid', label: t('desktop.sheets_insert_col_right', 'Insert column right') },
                { separator: true },
                { action: 'delete-selected-rows', icon: 'trash', label: t('desktop.sheets_delete_rows', 'Delete selected rows') },
                { action: 'delete-selected-cols', icon: 'trash', label: t('desktop.sheets_delete_columns', 'Delete selected columns') }
            ];
            const menu = document.createElement('div');
            menu.className = 'office-sheet-context-menu';
            menu.setAttribute('role', 'menu');
            menu.innerHTML = items.map(item => item.separator
                ? '<div class="office-sheet-context-separator" role="separator"></div>'
                : `<button type="button" role="menuitem" data-action="${esc(item.action)}">${iconMarkup(item.icon, '', 'office-sheet-context-icon', 14)}<span>${esc(item.label)}</span></button>`).join('');
            document.body.appendChild(menu);
            const rect = menu.getBoundingClientRect();
            menu.style.left = Math.max(8, Math.min(x, window.innerWidth - rect.width - 8)) + 'px';
            menu.style.top = Math.max(8, Math.min(y, window.innerHeight - rect.height - 8)) + 'px';
            menu.addEventListener('click', event => {
                const button = event.target.closest('button[data-action]');
                if (!button) return;
                handleSheetContextAction(button.dataset.action);
                closeSheetContextMenu();
            });
            contextMenuOutsideHandler = event => {
                if (contextMenu && !contextMenu.contains(event.target)) closeSheetContextMenu();
            };
            setTimeout(() => document.addEventListener('mousedown', contextMenuOutsideHandler), 0);
            contextMenu = menu;
        }

        function closeSheetContextMenu() {
            if (contextMenuOutsideHandler) {
                document.removeEventListener('mousedown', contextMenuOutsideHandler);
                contextMenuOutsideHandler = null;
            }
            if (contextMenu) {
                contextMenu.remove();
                contextMenu = null;
            }
        }

        function handleSheetContextAction(action) {
            switch (action) {
            case 'copy-range':
                copyRange();
                break;
            case 'paste-range':
                pasteRange();
                break;
            case 'clear-range':
                clearRange();
                break;
            case 'insert-row-above':
                insertRow(selectionRange().startRow);
                break;
            case 'insert-row-below':
                insertRow(selectionRange().endRow + 1);
                break;
            case 'insert-col-left':
                insertColumn(selectionRange().startCol);
                break;
            case 'insert-col-right':
                insertColumn(selectionRange().endCol + 1);
                break;
            case 'delete-selected-rows':
                deleteSelectedRows();
                break;
            case 'delete-selected-cols':
                deleteSelectedColumns();
                break;
            }
        }

        function copyRange() {
            const range = selectionRange();
            const rows = [];
            for (let r = range.startRow; r <= range.endRow; r++) {
                const values = [];
                for (let c = range.startCol; c <= range.endCol; c++) {
                    const input = cellInput(r, c);
                    values.push(input ? input.value : '');
                }
                rows.push(values.join('\t'));
            }
            localClipboard = rows.join('\n');
            if (navigator.clipboard && navigator.clipboard.writeText) {
                navigator.clipboard.writeText(localClipboard).catch(() => {});
            }
            setStatus(rangeLabel());
        }

        async function pasteRange() {
            if (readonly) return;
            let text = localClipboard;
            if (navigator.clipboard && navigator.clipboard.readText) {
                try {
                    text = await navigator.clipboard.readText() || text;
                } catch (_) {}
            }
            if (!text) return;
            const rows = captureDisplayRows();
            const range = selectionRange();
            const parsed = text.split(/\r?\n/).filter((line, index, all) => line !== '' || index < all.length - 1).map(line => line.split('\t'));
            if (!parsed.length) return;
            const requiredRows = range.startRow + parsed.length;
            const requiredCols = range.startCol + Math.max(1, ...parsed.map(row => row.length));
            ensureRows(rows, requiredRows, Math.max(requiredCols, maxCols(rows), MIN_COLS));
            parsed.forEach((rowValues, r) => {
                rowValues.forEach((value, c) => {
                    rows[range.startRow + r][range.startCol + c] = cellFromRaw(value);
                });
            });
            selection = {
                anchor: { row: range.startRow, col: range.startCol },
                focus: { row: range.startRow + parsed.length - 1, col: range.startCol + Math.max(0, parsed[0].length - 1) }
            };
            setSheetRows(rows);
        }

        function clearRange() {
            if (readonly) return;
            selectedInputs().forEach(input => {
                setCellFromInput(input, '');
            });
            updateFormulaBar();
        }

        function insertRow(index) {
            if (readonly) return;
            const rows = captureDisplayRows();
            const width = Math.max(MIN_COLS, maxCols(rows));
            rows.splice(index, 0, Array.from({ length: width }, () => ({ value: '' })));
            selection = { anchor: { row: index, col: 0 }, focus: { row: index, col: width - 1 } };
            setSheetRows(rows);
        }

        function insertColumn(index) {
            if (readonly) return;
            const rows = captureDisplayRows();
            ensureRows(rows, Math.max(MIN_ROWS, rows.length), Math.max(MIN_COLS, maxCols(rows)));
            rows.forEach(row => row.splice(index, 0, { value: '' }));
            selection = { anchor: { row: 0, col: index }, focus: { row: rows.length - 1, col: index } };
            setSheetRows(rows);
        }

        function deleteSelectedRows() {
            if (readonly) return;
            const rows = captureDisplayRows();
            const range = selectionRange();
            rows.splice(range.startRow, range.endRow - range.startRow + 1);
            if (!rows.length) rows.push(Array.from({ length: MIN_COLS }, () => ({ value: '' })));
            selection = { anchor: { row: Math.min(range.startRow, rows.length - 1), col: 0 }, focus: { row: Math.min(range.startRow, rows.length - 1), col: Math.max(MIN_COLS, maxCols(rows)) - 1 } };
            setSheetRows(rows);
        }

        function deleteSelectedColumns() {
            if (readonly) return;
            const rows = captureDisplayRows();
            const range = selectionRange();
            rows.forEach(row => row.splice(range.startCol, range.endCol - range.startCol + 1));
            if (!maxCols(rows)) rows.forEach(row => row.push({ value: '' }));
            selection = { anchor: { row: 0, col: Math.min(range.startCol, Math.max(MIN_COLS, maxCols(rows)) - 1) }, focus: { row: Math.max(MIN_ROWS, rows.length) - 1, col: Math.min(range.startCol, Math.max(MIN_COLS, maxCols(rows)) - 1) } };
            setSheetRows(rows);
        }

        function selectedInputs() {
            const range = selectionRange();
            const inputs = [];
            for (let r = range.startRow; r <= range.endRow; r++) {
                for (let c = range.startCol; c <= range.endCol; c++) {
                    const input = cellInput(r, c);
                    if (input) inputs.push(input);
                }
            }
            return inputs;
        }

        function selectionRange() {
            return {
                startRow: Math.min(selection.anchor.row, selection.focus.row),
                endRow: Math.max(selection.anchor.row, selection.focus.row),
                startCol: Math.min(selection.anchor.col, selection.focus.col),
                endCol: Math.max(selection.anchor.col, selection.focus.col)
            };
        }

        function rangeLabel() {
            const range = selectionRange();
            const start = cellName(range.startRow, range.startCol);
            const end = cellName(range.endRow, range.endCol);
            return start === end ? start : start + ':' + end;
        }

        function cellClass(row, col) {
            const classes = [];
            if (cellInSelection(row, col)) classes.push('office-cell-selected');
            if (isFocusCell(row, col)) classes.push('office-cell-active');
            return classes.join(' ');
        }

        function cellInSelection(row, col) {
            return cellInRange(row, col, selectionRange());
        }

        function cellInRange(row, col, range) {
            return row >= range.startRow && row <= range.endRow && col >= range.startCol && col <= range.endCol;
        }

        function isFocusCell(row, col) {
            return selection.focus.row === row && selection.focus.col === col;
        }

        function cellInput(row, col) {
            return gridHost.querySelector(`input[data-row="${row}"][data-col="${col}"]`);
        }

        function displayRowCount() {
            return Math.max(MIN_ROWS, gridHost.querySelectorAll('tbody tr').length);
        }

        function displayColCount() {
            return Math.max(MIN_COLS, gridHost.querySelectorAll('thead th[data-col-header]').length);
        }

        function clampCellRow(row) {
            return Math.min(displayRowCount() - 1, Math.max(0, Number(row) || 0));
        }

        function clampCellCol(col) {
            return Math.min(displayColCount() - 1, Math.max(0, Number(col) || 0));
        }

        function clampSelection(rowCount, colCount) {
            const clampCell = cell => ({
                row: Math.max(0, Math.min(rowCount - 1, cell.row || 0)),
                col: Math.max(0, Math.min(colCount - 1, cell.col || 0))
            });
            selection = { anchor: clampCell(selection.anchor), focus: clampCell(selection.focus) };
        }

        async function save() {
            if (readonly) return;
            captureGrid();
            setStatus(t('desktop.saving', 'Saving...'));
            const path = pathInput.value.trim() || DEFAULT_PATH;
            workbook.path = path;
            const body = await api('/api/desktop/office/workbook', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ path, workbook, office_version: officeVersion })
            });
            officeVersion = body.office_version || officeVersion;
            setPath(path);
            setStatus(t('desktop.sheets_saved', 'Saved'));
            notify({ type: 'success', message: t('desktop.sheets_saved', 'Saved') });
            await refreshDesktop();
        }

        function exportURL(format) {
            const path = pathInput.value.trim() || DEFAULT_PATH;
            return '/api/desktop/office/export?path=' + encodeURIComponent(path) + '&format=' + encodeURIComponent(format);
        }

        async function openExport(format) {
            if (!pathInput.value.trim() && !readonly) await save();
            window.open(exportURL(format), '_blank', 'noopener');
        }

        function setWindowMenus() {
            if (typeof ctx.setWindowMenus !== 'function') return;
            ctx.setWindowMenus(windowId, [
                {
                    id: 'file',
                    labelKey: 'desktop.menu_file',
                    items: [
                        { id: 'save', labelKey: 'desktop.sheets_save', icon: 'save', shortcut: 'Ctrl+S', disabled: readonly, action: () => save().catch(err => setStatus(err.message || String(err))) },
                        { type: 'separator' },
                        { id: 'download-xlsx', labelKey: 'desktop.sheets_download_xlsx', icon: 'download', action: () => openExport('xlsx').catch(err => setStatus(err.message || String(err))) },
                        { id: 'export-csv', labelKey: 'desktop.sheets_export_csv', icon: 'spreadsheet', action: () => openExport('csv').catch(err => setStatus(err.message || String(err))) }
                    ]
                },
                {
                    id: 'edit',
                    labelKey: 'desktop.menu_edit',
                    items: [
                        { id: 'copy', labelKey: 'desktop.fm.copy', icon: 'copy', shortcut: 'Ctrl+C', action: copyRange },
                        { id: 'paste', labelKey: 'desktop.fm.paste', icon: 'clipboard', shortcut: 'Ctrl+V', disabled: readonly, action: () => pasteRange() },
                        { id: 'clear', labelKey: 'desktop.sheets_clear_range', icon: 'x', shortcut: 'Del', disabled: readonly, action: clearRange },
                        { type: 'separator' },
                        { id: 'delete-rows', labelKey: 'desktop.sheets_delete_rows', icon: 'trash', disabled: readonly, action: deleteSelectedRows },
                        { id: 'delete-cols', labelKey: 'desktop.sheets_delete_columns', icon: 'trash', disabled: readonly, action: deleteSelectedColumns }
                    ]
                },
                {
                    id: 'insert',
                    labelKey: 'desktop.menu_insert',
                    items: [
                        { id: 'add-row', labelKey: 'desktop.sheets_add_row', icon: 'list', disabled: readonly, action: () => insertRow(captureDisplayRows().length) },
                        { id: 'add-col', labelKey: 'desktop.sheets_add_column', icon: 'grid', disabled: readonly, action: () => insertColumn(Math.max(MIN_COLS, maxCols(captureDisplayRows()))) },
                        { type: 'separator' },
                        { id: 'insert-row-above', labelKey: 'desktop.sheets_insert_row_above', icon: 'list', disabled: readonly, action: () => insertRow(selectionRange().startRow) },
                        { id: 'insert-row-below', labelKey: 'desktop.sheets_insert_row_below', icon: 'list', disabled: readonly, action: () => insertRow(selectionRange().endRow + 1) },
                        { id: 'insert-col-left', labelKey: 'desktop.sheets_insert_col_left', icon: 'grid', disabled: readonly, action: () => insertColumn(selectionRange().startCol) },
                        { id: 'insert-col-right', labelKey: 'desktop.sheets_insert_col_right', icon: 'grid', disabled: readonly, action: () => insertColumn(selectionRange().endCol + 1) }
                    ]
                }
            ]);
        }

        async function load() {
            const adapter = window.AuraUniverSheetsAdapter;
            if (adapter && typeof adapter.render === 'function') {
                adapter.render(host, windowId, ctx);
                return;
            }
            updateExportLinks();
            try {
                const body = await api('/api/desktop/office/workbook?path=' + encodeURIComponent(currentPath));
                workbook = normalizeWorkbook(body.workbook || {}, currentPath);
                officeVersion = body.office_version || null;
                setPath((body.entry && body.entry.path) || workbook.path || currentPath);
                setStatus('');
            } catch (err) {
                officeVersion = null;
                workbook = emptyWorkbook(currentPath);
                setStatus('');
                if (ctx.path) notify({ type: 'info', message: err.message || String(err) });
            }
            renderWorkbook();
        }

        host.querySelector('[data-action="apply-formula"]').addEventListener('click', applyFormulaBar);
        formulaInput.addEventListener('keydown', event => {
            if (event.key === 'Enter') {
                event.preventDefault();
                applyFormulaBar();
            }
        });
        pathInput.addEventListener('change', () => {
            setPath(pathInput.value.trim() || DEFAULT_PATH);
            load();
        });

        setWindowMenus();
        load();

        function applyReadonlyState() {
            host.querySelectorAll('[data-action="apply-formula"]').forEach(button => {
                button.disabled = readonly;
            });
            if (formulaInput) formulaInput.disabled = readonly;
            setWindowMenus();
        }
    }

    function dispose(windowId) {
        const instance = instances.get(windowId);
        if (instance && typeof instance.closeContextMenu === 'function') {
            instance.closeContextMenu();
        }
        instances.delete(windowId);
    }

    async function fetchJSON(url, options) {
        const resp = await fetch(url, options);
        const body = await resp.json().catch(() => ({}));
        if (!resp.ok) throw new Error(body.error || body.message || ('HTTP ' + resp.status));
        return body;
    }

    function emptyWorkbook(path) {
        return { path, sheets: [{ name: 'Sheet1', rows: [] }] };
    }

    function normalizeWorkbook(raw, path) {
        const sheets = Array.isArray(raw.sheets) && raw.sheets.length ? raw.sheets : [{ name: 'Sheet1', rows: [] }];
        return {
            path: raw.path || path || DEFAULT_PATH,
            sheets: sheets.map((sheet, index) => ({
                name: sheet.name || ('Sheet' + (index + 1)),
                rows: Array.isArray(sheet.rows) ? sheet.rows : []
            }))
        };
    }

    function padRows(rows, minRows, minCols) {
        const width = Math.max(minCols, maxCols(rows));
        const padded = rows.map(row => {
            const next = Array.isArray(row) ? row.slice() : [];
            while (next.length < width) next.push({ value: '' });
            return next;
        });
        while (padded.length < minRows) padded.push(Array.from({ length: width }, () => ({ value: '' })));
        return padded;
    }

    function trimRows(rows) {
        let lastRow = -1;
        rows.forEach((row, r) => {
            if (row.some(cell => (cell.value || cell.formula || '').trim() !== '')) lastRow = r;
        });
        return rows.slice(0, lastRow + 1).map(row => {
            let lastCol = -1;
            row.forEach((cell, c) => {
                if ((cell.value || cell.formula || '').trim() !== '') lastCol = c;
            });
            return row.slice(0, lastCol + 1);
        });
    }

    function ensureRows(rows, rowCount, colCount) {
        while (rows.length < rowCount) rows.push([]);
        rows.forEach(row => {
            while (row.length < colCount) row.push({ value: '' });
        });
    }

    function maxCols(rows) {
        return Math.max(0, ...((rows || []).map(row => Array.isArray(row) ? row.length : 0)));
    }

        function displayCell(cell) {
            if (!cell) return '';
            if (cell.formula) return '=' + String(cell.formula).replace(/^=/, '');
            return cell.value || '';
        }

        function cellInputAttributes(cell, sheet) {
            if (!cell || !cell.formula) {
                return `value="${esc(displayCell(cell))}"`;
            }
            const formula = String(cell.formula).replace(/^=/, '');
            const displayValue = evaluateFormulaForSheet(sheet, formula);
            return `value="${esc(displayValue)}" data-formula="${esc(formula)}" data-display-value="${esc(displayValue)}" title="${esc('=' + formula)}"`;
        }

        function syncFormulaDataset(input, cell, sheet) {
            if (!input) return;
            if (cell && cell.formula) {
                const formula = String(cell.formula).replace(/^=/, '');
                const displayValue = evaluateFormulaForSheet(sheet, formula);
                input.dataset.formula = formula;
                input.dataset.displayValue = displayValue;
                input.title = '=' + formula;
                return;
            }
            delete input.dataset.formula;
            delete input.dataset.displayValue;
            input.removeAttribute('title');
        }

        function showFormulaForEditing(input) {
            if (!input || !input.dataset.formula) return;
            input.value = '=' + input.dataset.formula;
        }

        function showFormulaResult(input) {
            if (!input || !input.dataset.formula) return;
            input.value = input.dataset.displayValue || '';
        }

        function evaluateFormulaForSheet(sheet, formula) {
            try {
                const value = parseFormulaExpression(tokenizeFormula(formula), sheet || { rows: [] });
                if (!Number.isFinite(value)) return '#ERR';
                return String(Number.isInteger(value) ? value : Math.round(value * 10000000000) / 10000000000);
            } catch (_) {
                return '#ERR';
            }
        }

        function tokenizeFormula(formula) {
            const tokens = [];
            const input = String(formula || '').replace(/^=/, '');
            let i = 0;
            while (i < input.length) {
                const ch = input[i];
                if (/\s/.test(ch)) { i++; continue; }
                if ('+-*/(),:'.includes(ch)) { tokens.push({ type: ch, value: ch }); i++; continue; }
                if (/[0-9.]/.test(ch)) {
                    const start = i;
                    while (i < input.length && /[0-9.]/.test(input[i])) i++;
                    if (i < input.length && /[eE]/.test(input[i])) {
                        i++;
                        if (i < input.length && /[+-]/.test(input[i])) i++;
                        while (i < input.length && /[0-9]/.test(input[i])) i++;
                    }
                    const value = Number(input.slice(start, i));
                    if (!Number.isFinite(value)) throw new Error('invalid number');
                    tokens.push({ type: 'number', value });
                    continue;
                }
                if (/[A-Za-z]/.test(ch)) {
                    const start = i;
                    while (i < input.length && /[A-Za-z0-9]/.test(input[i])) i++;
                    const value = input.slice(start, i).toUpperCase();
                    tokens.push({ type: /\d/.test(value) ? 'cell' : 'ident', value });
                    continue;
                }
                throw new Error('invalid token');
            }
            tokens.push({ type: 'eof', value: '' });
            return tokens;
        }

        function parseFormulaExpression(tokens, sheet) {
            let index = 0;
            const peek = () => tokens[index] || { type: 'eof' };
            const take = type => peek().type === type ? tokens[index++] : null;
            const expect = type => {
                const token = take(type);
                if (!token) throw new Error('expected ' + type);
                return token;
            };
            const expression = () => {
                let value = term();
                while (peek().type === '+' || peek().type === '-') {
                    const op = tokens[index++].type;
                    const right = term();
                    value = op === '+' ? value + right : value - right;
                }
                return value;
            };
            const term = () => {
                let value = unary();
                while (peek().type === '*' || peek().type === '/') {
                    const op = tokens[index++].type;
                    const right = unary();
                    value = op === '*' ? value * right : value / right;
                }
                return value;
            };
            const unary = () => {
                if (take('+')) return unary();
                if (take('-')) return -unary();
                return primary();
            };
            const primary = () => {
                const token = peek();
                if (take('number')) return token.value;
                if (take('cell')) {
                    if (take(':')) return rangeValues(token.value, expect('cell').value).reduce((sum, value) => sum + value, 0);
                    return numericCellValue(sheet, token.value);
                }
                if (take('ident')) return formulaFunction(token.value);
                if (take('(')) {
                    const value = expression();
                    expect(')');
                    return value;
                }
                throw new Error('invalid formula');
            };
            const formulaFunction = name => {
                expect('(');
                const args = [];
                if (peek().type !== ')') {
                    do {
                        if (peek().type === 'cell' && tokens[index + 1] && tokens[index + 1].type === ':') {
                            const start = expect('cell').value;
                            expect(':');
                            args.push(...rangeValues(start, expect('cell').value));
                        } else {
                            args.push(expression());
                        }
                    } while (take(','));
                }
                expect(')');
                if (name === 'SUM') return args.reduce((sum, value) => sum + value, 0);
                if (name === 'AVG' || name === 'AVERAGE') return args.reduce((sum, value) => sum + value, 0) / Math.max(1, args.length);
                if (name === 'COUNT') return args.length;
                if (name === 'MIN') return Math.min(...args);
                if (name === 'MAX') return Math.max(...args);
                throw new Error('unknown function');
            };
            const rangeValues = (start, end) => {
                const a = parseCellRef(start);
                const b = parseCellRef(end);
                if (b.row < a.row || b.col < a.col) throw new Error('bad range');
                const values = [];
                for (let r = a.row; r <= b.row; r++) {
                    for (let c = a.col; c <= b.col; c++) {
                        values.push(numericCellValue(sheet, cellName(r, c)));
                    }
                }
                return values;
            };
            const result = expression();
            expect('eof');
            return result;
        }

        function numericCellValue(sheet, ref) {
            const pos = parseCellRef(ref);
            const cell = sheet && sheet.rows && sheet.rows[pos.row] && sheet.rows[pos.row][pos.col];
            if (!cell) return 0;
            if (cell.formula) return Number(evaluateFormulaForSheet(sheet, cell.formula)) || 0;
            const value = Number(cell.value);
            return Number.isFinite(value) ? value : 0;
        }

        function parseCellRef(ref) {
            const match = /^([A-Z]+)([0-9]+)$/.exec(String(ref || '').toUpperCase());
            if (!match) throw new Error('bad cell');
            let col = 0;
            for (const ch of match[1]) col = col * 26 + ch.charCodeAt(0) - 64;
            return { row: Number(match[2]) - 1, col: col - 1 };
        }

        function cellName(row, col) {
            return columnName(col + 1) + String(row + 1);
        }

    function columnName(index) {
        let name = '';
        let n = index;
        while (n > 0) {
            const mod = (n - 1) % 26;
            name = String.fromCharCode(65 + mod) + name;
            n = Math.floor((n - mod) / 26);
        }
        return name;
    }

    function interpolate(text, vars) {
        let result = String(text || '');
        Object.entries(vars || {}).forEach(([key, value]) => {
            result = result.replaceAll('{{' + key + '}}', String(value));
        });
        return result;
    }

    window.SheetsApp = window.SheetsApp || {};
    window.SheetsApp.render = render;
    window.SheetsApp.dispose = dispose;
})();
