(function () {
    'use strict';

    const DEFAULT_PATH = 'Documents/untitled.xlsx';
    const MIN_ROWS = 24;
    const MIN_COLS = 10;
    const MAX_UNDO = 50;
    const AUTOSAVE_DELAY = 2000;
    const instances = new Map();

    function render(host, windowId, context) {
        if (!host) return;
        dispose(windowId);
        instances.set(windowId, { container: host, closeContextMenu: () => closeSheetContextMenu(), autosaveTimer: null, closeSearch: null, formatClickHandler: null });
        const ctx = context || {};
        const esc = ctx.esc || (value => String(value == null ? '' : value));
        const rawT = ctx.t || ((key, vars) => interpolate(key, vars));
        const t = (key, fallback, vars) => {
            if (fallback && typeof fallback === 'object' && !Array.isArray(fallback)) { vars = fallback; fallback = ''; }
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
        let undoStack = [];
        let redoStack = [];
        let isDirty = false;
        let isUndoRedoAction = false;
        let formatToolbar = null;

        host.innerHTML = `<div class="office-app office-sheets" data-office-sheets="${esc(windowId)}">
            <div class="vd-toolbar office-toolbar">
                <input class="office-path-input" data-path value="${esc(currentPath)}" spellcheck="false" autocomplete="off">
                <span class="vd-chat-meta" data-status>${esc(t('desktop.sheets_loading'))}</span>
            </div>
            <div class="office-formula-bar" data-formula-bar>
                <output class="office-range-name" data-range-name>A1</output>
                <span class="office-formula-fx" aria-hidden="true">fx</span>
                <input class="office-formula-input" data-formula-input value="" spellcheck="false" autocomplete="off" placeholder="=SUM(A1:A3)">
                <button class="vd-tool-button office-formula-apply" type="button" data-action="apply-formula">${iconMarkup('check-square', 'OK', 'vd-tool-icon', 15)}<span>${esc(t('desktop.ok'))}</span></button>
            </div>
            <div class="office-format-bar" data-format-bar></div>
            <div class="office-sheet-grid-wrap" data-grid></div>
            <div class="office-sheet-tabs" data-tabs></div>
            <div class="office-status-bar" data-status-bar>
                <span class="office-status-left" data-status-left></span>
                <span class="office-status-right" data-status-right></span>
            </div>
        </div>`;

        const pathInput = host.querySelector('[data-path]');
        const status = host.querySelector('[data-status]');
        const tabsHost = host.querySelector('[data-tabs]');
        const gridHost = host.querySelector('[data-grid]');
        const rangeName = host.querySelector('[data-range-name]');
        const formulaInput = host.querySelector('[data-formula-input]');
        const formatBarHost = host.querySelector('[data-format-bar]');
        const statusLeft = host.querySelector('[data-status-left]');
        const statusRight = host.querySelector('[data-status-right]');
        if (typeof ctx.wireContextMenuBoundary === 'function') ctx.wireContextMenuBoundary(host);
        const windowContent = host.closest('.vd-window-content');
        if (windowContent) windowContent.classList.add('office-sheets-window-content');

        const formulas = window.SheetsFormulas;
        const formatModule = window.SheetsFormat;
        const searchModule = window.SheetsSearch;

        if (formatModule && formatBarHost) {
            formatToolbar = formatModule.renderToolbar(formatBarHost, t, handleFormatChange);
            if (formatToolbar) {
                wireFormatToolbar(formatToolbar);
            }
        }

        function setStatus(message) {
            if (status) status.textContent = message || '';
        }

        function setDirty(dirty) {
            isDirty = dirty;
            const displayName = (pathInput.value.trim() || DEFAULT_PATH).split('/').pop();
            if (typeof ctx.updateWindowContext === 'function') {
                ctx.updateWindowContext(windowId, { title: (isDirty ? '• ' : '') + displayName });
            }
            if (isDirty) scheduleAutosave();
        }

        function scheduleAutosave() {
            if (readonly || !isDirty) return;
            const inst = instances.get(windowId);
            if (inst && inst.autosaveTimer) clearTimeout(inst.autosaveTimer);
            if (inst) {
                inst.autosaveTimer = setTimeout(() => {
                    inst.autosaveTimer = null;
                    if (isDirty) save().catch(() => {});
                }, AUTOSAVE_DELAY);
            }
        }

        function pushSnapshot() {
            if (isUndoRedoAction) return;
            undoStack.push(JSON.parse(JSON.stringify(workbook.sheets)));
            if (undoStack.length > MAX_UNDO) undoStack.shift();
            redoStack = [];
        }

        function undo() {
            if (!undoStack.length || readonly) return;
            isUndoRedoAction = true;
            captureGrid();
            redoStack.push(JSON.parse(JSON.stringify(workbook.sheets)));
            workbook.sheets = undoStack.pop();
            activeSheet = Math.min(activeSheet, workbook.sheets.length - 1);
            renderWorkbook();
            isUndoRedoAction = false;
            setStatus(t('desktop.sheets_undo'));
        }

        function redo() {
            if (!redoStack.length || readonly) return;
            isUndoRedoAction = true;
            captureGrid();
            undoStack.push(JSON.parse(JSON.stringify(workbook.sheets)));
            workbook.sheets = redoStack.pop();
            activeSheet = Math.min(activeSheet, workbook.sheets.length - 1);
            renderWorkbook();
            isUndoRedoAction = false;
            setStatus(t('desktop.sheets_redo'));
        }

        function setPath(path) {
            currentPath = path || DEFAULT_PATH;
            pathInput.value = currentPath;
            updateExportLinks();
            if (typeof ctx.updateWindowContext === 'function') ctx.updateWindowContext(windowId, { path: currentPath });
        }

        function updateExportLinks() { setWindowMenus(); }

        function renderWorkbook() {
            closeSheetContextMenu();
            workbook = normalizeWorkbook(workbook, pathInput.value.trim() || DEFAULT_PATH);
            activeSheet = Math.min(activeSheet, workbook.sheets.length - 1);
            const sheet = workbook.sheets[activeSheet];
            tabsHost.innerHTML = workbook.sheets.map((s, index) => `<button type="button" class="${index === activeSheet ? 'active' : ''}" data-sheet-index="${index}" title="${esc(s.name || '')}">${esc(s.name || (t('desktop.sheets_sheet') + ' ' + (index + 1)))}</button>`).join('') +
                `<button type="button" class="office-sheet-add-btn" data-action="add-sheet" title="${esc(t('desktop.sheets_add_sheet'))}">+</button>`;
            tabsHost.querySelectorAll('[data-sheet-index]').forEach(btn => {
                btn.addEventListener('click', () => {
                    captureGrid();
                    activeSheet = Number(btn.dataset.sheetIndex) || 0;
                    selection = { anchor: { row: 0, col: 0 }, focus: { row: 0, col: 0 } };
                    renderWorkbook();
                });
                btn.addEventListener('dblclick', () => {
                    if (readonly) return;
                    const idx = Number(btn.dataset.sheetIndex);
                    renameSheetPrompt(idx);
                });
                btn.addEventListener('contextmenu', e => {
                    e.preventDefault();
                    const idx = Number(btn.dataset.sheetIndex);
                    showSheetTabContextMenu(e.clientX, e.clientY, idx);
                });
            });
            const addBtn = tabsHost.querySelector('[data-action="add-sheet"]');
            if (addBtn) addBtn.addEventListener('click', () => { if (!readonly) addNewSheet(); });

            const rows = padRows(sheet.rows || [], MIN_ROWS, MIN_COLS);
            clampSelection(rows.length, rows[0].length);
            const colHeaders = Array.from({ length: rows[0].length }, (_, i) => formulas ? formulas.columnName(i + 1) : columnNameFallback(i + 1));
            gridHost.innerHTML = `<table class="office-grid">
                <thead><tr><th></th>${colHeaders.map((col, c) => `<th data-col-header="${c}">${esc(col)}</th>`).join('')}</tr></thead>
                <tbody>${rows.map((row, r) => `<tr><th data-row-header="${r}">${r + 1}</th>${row.map((cell, c) => `<td data-cell-row="${r}" data-cell-col="${c}" class="${esc(cellClass(r, c))}"><input data-row="${r}" data-col="${c}" ${cellInputAttributes(cell, sheet)} spellcheck="false" ${readonly ? 'readonly' : ''}></td>`).join('')}</tr>`).join('')}</tbody>
            </table>`;
            wireGrid();
            applyCellFormats(sheet);
            applyReadonlyState();
            renderSelection();
            updateStatusBar();
        }

        function applyCellFormats(sheet) {
            if (!formatModule || !sheet || !sheet.rows) return;
            let applied = 0;
            gridHost.querySelectorAll('td[data-cell-row][data-cell-col]').forEach(td => {
                const row = Number(td.dataset.cellRow);
                const col = Number(td.dataset.cellCol);
                const cell = sheet.rows[row] && sheet.rows[row][col];
                const input = td.querySelector('input');
                if (cell && cell.format) {
                    applied++;
                    console.log('[FMT] applyCellFormats row=' + row + ' col=' + col + ' format=' + JSON.stringify(cell.format));
                    console.log('[FMT] td before: fontWeight=' + td.style.fontWeight + ' input before: fontWeight=' + (input ? input.style.fontWeight : 'no-input'));
                }
                formatModule.renderFormatStyles(td, input, cell && cell.format);
                if (cell && cell.format) {
                    console.log('[FMT] td after: fontWeight=' + td.style.fontWeight + ' input after: fontWeight=' + (input ? input.style.fontWeight : 'no-input'));
                }
            });
            console.log('[FMT] applyCellFormats total=' + gridHost.querySelectorAll('td[data-cell-row][data-cell-col]').length + ' formatted=' + applied);
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
                    updateFormatToolbarState(row, col);
                });
                input.addEventListener('blur', () => showFormulaResult(input));
                input.addEventListener('input', () => {
                    if (readonly) return;
                    pushSnapshot();
                    const raw = input.value;
                    setCellFromInput(input, raw);
                    recalcFormulas();
                    if (isFocusCell(Number(input.dataset.row), Number(input.dataset.col))) updateFormulaBar();
                    setDirty(true);
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
                event.preventDefault(); copyRange(); return;
            }
            if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'v') {
                event.preventDefault(); pasteRange(); return;
            }
            if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'z') {
                event.preventDefault(); undo(); return;
            }
            if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'y') {
                event.preventDefault(); redo(); return;
            }
            if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'f') {
                event.preventDefault(); openSearch(); return;
            }
            if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'h') {
                event.preventDefault(); openSearch(); return;
            }
            if (event.key === 'Delete') {
                event.preventDefault(); clearRange(); return;
            }
            if (event.key === 'Home' && (event.ctrlKey || event.metaKey)) {
                event.preventDefault();
                const target = cellInput(0, 0);
                if (target) { target.focus(); selectCell(0, 0, event.shiftKey); }
                return;
            }
            if (event.key === 'End' && (event.ctrlKey || event.metaKey)) {
                event.preventDefault();
                const sheet = workbook.sheets[activeSheet];
                const maxR = sheet && sheet.rows ? Math.max(0, sheet.rows.length - 1) : 0;
                let maxC = 0;
                if (sheet && sheet.rows) sheet.rows.forEach(row => { if (Array.isArray(row)) maxC = Math.max(maxC, row.length - 1); });
                const target = cellInput(Math.min(maxR, displayRowCount() - 1), Math.min(maxC, displayColCount() - 1));
                if (target) { target.focus(); selectCell(Math.min(maxR, displayRowCount() - 1), Math.min(maxC, displayColCount() - 1), event.shiftKey); }
                return;
            }
            if (event.key === 'PageDown') {
                event.preventDefault();
                const pageSize = Math.max(1, Math.floor(gridHost.clientHeight / 32) - 1);
                const newRow = clampCellRow(selection.focus.row + pageSize);
                const target = cellInput(newRow, selection.focus.col);
                if (target) { target.focus(); selectCell(newRow, selection.focus.col, event.shiftKey); }
                return;
            }
            if (event.key === 'PageUp') {
                event.preventDefault();
                const pageSize = Math.max(1, Math.floor(gridHost.clientHeight / 32) - 1);
                const newRow = clampCellRow(selection.focus.row - pageSize);
                const target = cellInput(newRow, selection.focus.col);
                if (target) { target.focus(); selectCell(newRow, selection.focus.col, event.shiftKey); }
                return;
            }
            if (event.key === ' ' && event.ctrlKey) {
                event.preventDefault(); selectColumn(selection.focus.col); return;
            }
            if (event.key === ' ' && event.shiftKey) {
                event.preventDefault(); selectRow(selection.focus.row); return;
            }
            const row = Number(input.dataset.row);
            const col = Number(input.dataset.col);
            const move = {
                ArrowUp: [row - 1, col], ArrowDown: [row + 1, col],
                ArrowLeft: [row, col - 1], ArrowRight: [row, col + 1],
                Enter: [row + 1, col], Tab: [row, col + (event.shiftKey ? -1 : 1)]
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
                tr.querySelectorAll('td').forEach((td, c) => {
                    const input = td.querySelector('input');
                    const cell = cellFromInputElement(input);
                    const sheet = workbook.sheets[activeSheet];
                    const oldCell = sheet && sheet.rows && sheet.rows[r] && sheet.rows[r][c];
                    if (oldCell && oldCell.format) cell.format = oldCell.format;
                    row[c] = cell;
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
            if (formula && (input.value === displayValue || input.value === '=' + formula)) return { formula };
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
            const oldCell = sheet.rows[row][col];
            if (oldCell && oldCell.format) cell.format = oldCell.format;
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
            row = Math.max(0, row); col = Math.max(0, col);
            if (extend) { selection.focus = { row, col }; }
            else { selection = { anchor: { row, col }, focus: { row, col } }; }
            if (rerender === false) { renderSelection(); return; }
            renderSelection();
        }

        function extendSelection(row, col) { selection.focus = { row, col }; renderSelection(); }

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
            updateStatusBar();
            updateFormatToolbarState(selection.focus.row, selection.focus.col);
        }

        function updateFormulaBar() {
            if (rangeName) { rangeName.value = rangeLabel(); rangeName.textContent = rangeLabel(); }
            const input = cellInput(selection.focus.row, selection.focus.col);
            if (formulaInput && input && document.activeElement !== formulaInput) {
                formulaInput.value = input.value;
            }
        }

        function updateStatusBar() {
            if (!statusLeft || !statusRight) return;
            const range = selectionRange();
            const sheet = workbook.sheets[activeSheet];
            if (!sheet) { statusLeft.textContent = ''; statusRight.textContent = ''; return; }
            const isSingle = range.startRow === range.endRow && range.startCol === range.endCol;
            if (isSingle) {
                statusLeft.textContent = '';
                statusRight.textContent = isDirty ? t('desktop.sheets_dirty_indicator') : '';
                return;
            }
            let sum = 0, count = 0, numCount = 0;
            for (let r = range.startRow; r <= range.endRow; r++) {
                for (let c = range.startCol; c <= range.endCol; c++) {
                    const cell = sheet.rows && sheet.rows[r] && sheet.rows[r][c];
                    if (!cell) continue;
                    count++;
                    const val = cell.formula ? (formulas ? formulas.evaluate(sheet, cell.formula) : '#ERR') : cell.value;
                    const num = Number(val);
                    if (Number.isFinite(num)) { sum += num; numCount++; }
                }
            }
            const parts = [];
            if (numCount > 0) {
                parts.push(t('desktop.sheets_status_sum') + ': ' + Math.round(sum * 1000000) / 1000000);
            }
            parts.push(t('desktop.sheets_status_count') + ': ' + count);
            if (numCount > 1) {
                parts.push(t('desktop.sheets_status_avg') + ': ' + Math.round(sum / numCount * 1000000) / 1000000);
            }
            statusLeft.textContent = parts.join('   ');
            statusRight.textContent = isDirty ? t('desktop.sheets_dirty_indicator') : '';
        }

        function updateFormatToolbarState(row, col) {
            if (!formatToolbar || !formatModule) return;
            const sheet = workbook.sheets[activeSheet];
            const cell = sheet && sheet.rows && sheet.rows[row] && sheet.rows[row][col];
            formatModule.updateToolbarState(formatToolbar, cell && cell.format);
        }

        function handleFormatChange(formatType, value) {
            if (readonly) return;
            pushSnapshot();
            const sheet = workbook.sheets[activeSheet];
            if (!sheet) return;
            const range = selectionRange();
            for (let r = range.startRow; r <= range.endRow; r++) {
                for (let c = range.startCol; c <= range.endCol; c++) {
                    ensureRows(sheet.rows, r + 1, c + 1);
                    if (!sheet.rows[r][c]) sheet.rows[r][c] = { value: '' };
                    if (formatModule) formatModule.applyFormat(sheet.rows[r][c], formatType, value);
                }
            }
            const cell = sheet.rows[range.startRow] && sheet.rows[range.startRow][range.startCol];
            console.log('[FMT] handleFormatChange type=' + formatType + ' value=' + value + ' cell.format=' + JSON.stringify(cell && cell.format));
            renderWorkbook();
            setDirty(true);
        }

        function wireFormatToolbar(toolbar) {
            toolbar.querySelectorAll('.office-fmt-btn[data-fmt]').forEach(btn => {
                btn.addEventListener('click', () => {
                    const fmt = btn.dataset.fmt;
                    if (fmt === 'font-color' || fmt === 'fill-color') return;
                    handleFormatChange(fmt);
                });
            });
            toolbar.querySelectorAll('.office-color-picker').forEach(picker => {
                picker.addEventListener('click', e => {
                    const swatch = e.target.closest('.office-color-swatch');
                    if (!swatch) return;
                    const dropdown = picker.closest('.office-fmt-dropdown');
                    const type = dropdown && dropdown.dataset.dropdown;
                    if (type) handleFormatChange(type === 'font-color' ? 'font-color' : 'fill-color', swatch.dataset.color);
                    picker.hidden = true;
                });
                const applyBtn = picker.querySelector('.office-color-apply');
                if (applyBtn) {
                    applyBtn.addEventListener('click', () => {
                        const input = picker.querySelector('.office-color-input');
                        const dropdown = picker.closest('.office-fmt-dropdown');
                        const type = dropdown && dropdown.dataset.dropdown;
                        if (type && input) handleFormatChange(type === 'font-color' ? 'font-color' : 'fill-color', input.value);
                        picker.hidden = true;
                    });
                }
            });
            toolbar.querySelectorAll('.office-fmt-dropdown').forEach(dropdown => {
                const btn = dropdown.querySelector('.office-fmt-btn');
                const picker = dropdown.querySelector('.office-color-picker');
                if (btn && picker) {
                    btn.addEventListener('click', () => {
                        toolbar.querySelectorAll('.office-color-picker').forEach(p => { if (p !== picker) p.hidden = true; });
                        picker.hidden = !picker.hidden;
                    });
                }
            });
            toolbar.querySelectorAll('.office-fmt-select').forEach(select => {
                select.addEventListener('change', () => {
                    handleFormatChange(select.dataset.fmt, select.value);
                });
            });
            const closePickersHandler = e => {
                if (!e.target.closest('.office-fmt-dropdown')) {
                    toolbar.querySelectorAll('.office-color-picker').forEach(p => p.hidden = true);
                }
            };
            document.addEventListener('click', closePickersHandler);
            const inst = instances.get(windowId);
            if (inst) inst.formatClickHandler = closePickersHandler;
        }

        function applyFormulaBar() {
            if (readonly) return;
            const input = cellInput(selection.focus.row, selection.focus.col);
            if (!input || !formulaInput) return;
            pushSnapshot();
            setCellFromInput(input, formulaInput.value);
            recalcFormulas();
            input.focus();
            updateFormulaBar();
            setDirty(true);
        }

        function openSearch() {
            if (searchModule) {
                const inst = instances.get(windowId);
                const searchState = searchModule.openSearch(host, gridHost, workbook, () => activeSheet, t, esc, (r, c) => selectCell(r, c, false), {
                    pushSnapshot: pushSnapshot,
                    setDirty: setDirty
                });
                if (inst) {
                    inst.searchState = searchState;
                    inst.closeSearch = () => {
                        if (searchState && searchState.overlay) {
                            searchState.overlay.remove();
                            searchState.overlay = null;
                        }
                    };
                }
            }
        }

        function showSheetContextMenu(x, y) {
            closeSheetContextMenu();
            const items = [
                { action: 'copy-range', icon: 'copy', label: t('desktop.fm.copy') },
                { action: 'paste-range', icon: 'clipboard', label: t('desktop.fm.paste') },
                { action: 'clear-range', icon: 'x', label: t('desktop.sheets_clear_range') },
                { separator: true },
                { action: 'insert-row-above', icon: 'list', label: t('desktop.sheets_insert_row_above') },
                { action: 'insert-row-below', icon: 'list', label: t('desktop.sheets_insert_row_below') },
                { action: 'insert-col-left', icon: 'grid', label: t('desktop.sheets_insert_col_left') },
                { action: 'insert-col-right', icon: 'grid', label: t('desktop.sheets_insert_col_right') },
                { separator: true },
                { action: 'delete-selected-rows', icon: 'trash', label: t('desktop.sheets_delete_rows') },
                { action: 'delete-selected-cols', icon: 'trash', label: t('desktop.sheets_delete_columns') }
            ];
            const menu = document.createElement('div');
            menu.className = 'office-sheet-context-menu';
            menu.setAttribute('role', 'menu');
            menu.style.visibility = 'hidden';
            menu.innerHTML = items.map(item => item.separator
                ? '<div class="office-sheet-context-separator" role="separator"></div>'
                : `<button type="button" role="menuitem" data-action="${esc(item.action)}">${iconMarkup(item.icon, '', 'office-sheet-context-icon', 14)}<span>${esc(item.label)}</span></button>`).join('');
            document.body.appendChild(menu);
            const rect = menu.getBoundingClientRect();
            menu.style.left = Math.max(8, Math.min(x, window.innerWidth - rect.width - 8)) + 'px';
            menu.style.top = Math.max(8, Math.min(y, window.innerHeight - rect.height - 8)) + 'px';
            menu.style.visibility = 'visible';
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

        function showSheetTabContextMenu(x, y, sheetIndex) {
            closeSheetContextMenu();
            const items = [
                { action: 'rename-sheet', icon: 'edit', label: t('desktop.sheets_rename_sheet') },
                { action: 'duplicate-sheet', icon: 'copy', label: t('desktop.sheets_duplicate_sheet') },
                { separator: true },
                { action: 'delete-sheet', icon: 'trash', label: t('desktop.sheets_delete_sheet') }
            ];
            const menu = document.createElement('div');
            menu.className = 'office-sheet-context-menu';
            menu.setAttribute('role', 'menu');
            menu.style.visibility = 'hidden';
            menu.innerHTML = items.map(item => item.separator
                ? '<div class="office-sheet-context-separator" role="separator"></div>'
                : `<button type="button" role="menuitem" data-action="${esc(item.action)}" data-sheet-index="${sheetIndex}">${iconMarkup(item.icon, '', 'office-sheet-context-icon', 14)}<span>${esc(item.label)}</span></button>`).join('');
            document.body.appendChild(menu);
            const rect = menu.getBoundingClientRect();
            menu.style.left = Math.max(8, Math.min(x, window.innerWidth - rect.width - 8)) + 'px';
            menu.style.top = Math.max(8, Math.min(y, window.innerHeight - rect.height - 8)) + 'px';
            menu.style.visibility = 'visible';
            menu.addEventListener('click', event => {
                const button = event.target.closest('button[data-action]');
                if (!button) return;
                const idx = Number(button.dataset.sheetIndex);
                switch (button.dataset.action) {
                case 'rename-sheet': renameSheetPrompt(idx); break;
                case 'duplicate-sheet': duplicateSheet(idx); break;
                case 'delete-sheet': deleteSheet(idx); break;
                }
                closeSheetContextMenu();
            });
            contextMenuOutsideHandler = event => {
                if (contextMenu && !contextMenu.contains(event.target)) closeSheetContextMenu();
            };
            setTimeout(() => document.addEventListener('mousedown', contextMenuOutsideHandler), 0);
            contextMenu = menu;
        }

        function closeSheetContextMenu() {
            if (contextMenuOutsideHandler) { document.removeEventListener('mousedown', contextMenuOutsideHandler); contextMenuOutsideHandler = null; }
            if (contextMenu) { contextMenu.remove(); contextMenu = null; }
        }

        function handleSheetContextAction(action) {
            switch (action) {
            case 'copy-range': copyRange(); break;
            case 'paste-range': pasteRange(); break;
            case 'clear-range': clearRange(); break;
            case 'insert-row-above': insertRow(selectionRange().startRow); break;
            case 'insert-row-below': insertRow(selectionRange().endRow + 1); break;
            case 'insert-col-left': insertColumn(selectionRange().startCol); break;
            case 'insert-col-right': insertColumn(selectionRange().endCol + 1); break;
            case 'delete-selected-rows': deleteSelectedRows(); break;
            case 'delete-selected-cols': deleteSelectedColumns(); break;
            }
        }

        function addNewSheet() {
            pushSnapshot();
            const name = t('desktop.sheets_sheet') + ' ' + (workbook.sheets.length + 1);
            workbook.sheets.push({ name, rows: [] });
            activeSheet = workbook.sheets.length - 1;
            selection = { anchor: { row: 0, col: 0 }, focus: { row: 0, col: 0 } };
            renderWorkbook();
            setDirty(true);
        }

        function renameSheetPrompt(index) {
            if (readonly || index < 0 || index >= workbook.sheets.length) return;
            const prompt = ctx.promptDialog || (async () => null);
            prompt(t('desktop.sheets_rename_sheet_title'), workbook.sheets[index].name).then(name => {
                if (name == null || !String(name).trim()) return;
                pushSnapshot();
                workbook.sheets[index].name = String(name).trim();
                renderWorkbook();
                setDirty(true);
            });
        }

        function duplicateSheet(index) {
            if (readonly || index < 0 || index >= workbook.sheets.length) return;
            pushSnapshot();
            const src = workbook.sheets[index];
            const copy = { name: src.name + ' (' + t('desktop.fm.copy') + ')', rows: JSON.parse(JSON.stringify(src.rows || [])) };
            workbook.sheets.splice(index + 1, 0, copy);
            activeSheet = index + 1;
            renderWorkbook();
            setDirty(true);
        }

        function deleteSheet(index) {
            if (readonly || index < 0 || index >= workbook.sheets.length || workbook.sheets.length <= 1) return;
            pushSnapshot();
            workbook.sheets.splice(index, 1);
            if (activeSheet >= workbook.sheets.length) activeSheet = workbook.sheets.length - 1;
            renderWorkbook();
            setDirty(true);
        }

        function copyRange() {
            const range = selectionRange();
            const sheet = workbook.sheets[activeSheet];
            const rows = [];
            for (let r = range.startRow; r <= range.endRow; r++) {
                const values = [];
                for (let c = range.startCol; c <= range.endCol; c++) {
                    const cell = sheet && sheet.rows && sheet.rows[r] && sheet.rows[r][c];
                    if (cell && cell.formula) {
                        values.push('=' + cell.formula);
                    } else {
                        values.push(cell ? (cell.value || '') : '');
                    }
                }
                rows.push(values.join('\t'));
            }
            localClipboard = rows.join('\n');
            if (navigator.clipboard && navigator.clipboard.writeText) navigator.clipboard.writeText(localClipboard).catch(() => {});
            setStatus(rangeLabel());
        }

        async function pasteRange() {
            if (readonly) return;
            let text = localClipboard;
            if (navigator.clipboard && navigator.clipboard.readText) {
                try { text = await navigator.clipboard.readText() || text; } catch (_) {}
            }
            if (!text) return;
            pushSnapshot();
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
            setDirty(true);
        }

        function clearRange() {
            if (readonly) return;
            pushSnapshot();
            selectedInputs().forEach(input => setCellFromInput(input, ''));
            updateFormulaBar();
            setDirty(true);
        }

        function insertRow(index) {
            if (readonly) return;
            pushSnapshot();
            const rows = captureDisplayRows();
            const width = Math.max(MIN_COLS, maxCols(rows));
            rows.splice(index, 0, Array.from({ length: width }, () => ({ value: '' })));
            selection = { anchor: { row: index, col: 0 }, focus: { row: index, col: width - 1 } };
            setSheetRows(rows);
            setDirty(true);
        }

        function insertColumn(index) {
            if (readonly) return;
            pushSnapshot();
            const rows = captureDisplayRows();
            ensureRows(rows, Math.max(MIN_ROWS, rows.length), Math.max(MIN_COLS, maxCols(rows)));
            rows.forEach(row => row.splice(index, 0, { value: '' }));
            selection = { anchor: { row: 0, col: index }, focus: { row: rows.length - 1, col: index } };
            setSheetRows(rows);
            setDirty(true);
        }

        function deleteSelectedRows() {
            if (readonly) return;
            pushSnapshot();
            const rows = captureDisplayRows();
            const range = selectionRange();
            rows.splice(range.startRow, range.endRow - range.startRow + 1);
            if (!rows.length) rows.push(Array.from({ length: MIN_COLS }, () => ({ value: '' })));
            selection = { anchor: { row: Math.min(range.startRow, rows.length - 1), col: 0 }, focus: { row: Math.min(range.startRow, rows.length - 1), col: Math.max(MIN_COLS, maxCols(rows)) - 1 } };
            setSheetRows(rows);
            setDirty(true);
        }

        function deleteSelectedColumns() {
            if (readonly) return;
            pushSnapshot();
            const rows = captureDisplayRows();
            const range = selectionRange();
            rows.forEach(row => row.splice(range.startCol, range.endCol - range.startCol + 1));
            if (!maxCols(rows)) rows.forEach(row => row.push({ value: '' }));
            selection = { anchor: { row: 0, col: Math.min(range.startCol, Math.max(MIN_COLS, maxCols(rows)) - 1) }, focus: { row: Math.max(MIN_ROWS, rows.length) - 1, col: Math.min(range.startCol, Math.max(MIN_COLS, maxCols(rows)) - 1) } };
            setSheetRows(rows);
            setDirty(true);
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
            const cn = formulas ? formulas.cellName : cellNameFallback;
            const start = cn(range.startRow, range.startCol);
            const end = cn(range.endRow, range.endCol);
            return start === end ? start : start + ':' + end;
        }

        function cellClass(row, col) {
            const classes = [];
            if (cellInSelection(row, col)) classes.push('office-cell-selected');
            if (isFocusCell(row, col)) classes.push('office-cell-active');
            return classes.join(' ');
        }

        function cellInSelection(row, col) { return cellInRange(row, col, selectionRange()); }
        function cellInRange(row, col, range) { return row >= range.startRow && row <= range.endRow && col >= range.startCol && col <= range.endCol; }
        function isFocusCell(row, col) { return selection.focus.row === row && selection.focus.col === col; }
        function cellInput(row, col) { return gridHost.querySelector(`input[data-row="${row}"][data-col="${col}"]`); }
        function displayRowCount() { return Math.max(MIN_ROWS, gridHost.querySelectorAll('tbody tr').length); }
        function displayColCount() { return Math.max(MIN_COLS, gridHost.querySelectorAll('thead th[data-col-header]').length); }
        function clampCellRow(row) { return Math.min(displayRowCount() - 1, Math.max(0, Number(row) || 0)); }
        function clampCellCol(col) { return Math.min(displayColCount() - 1, Math.max(0, Number(col) || 0)); }
        function clampSelection(rowCount, colCount) {
            const clampCell = cell => ({ row: Math.max(0, Math.min(rowCount - 1, cell.row || 0)), col: Math.max(0, Math.min(colCount - 1, cell.col || 0)) });
            selection = { anchor: clampCell(selection.anchor), focus: clampCell(selection.focus) };
        }

        let isSaving = false;

        async function save() {
            if (readonly || isSaving) return;
            isSaving = true;
            try {
                captureGrid();
                setStatus(t('desktop.saving'));
                const path = pathInput.value.trim() || DEFAULT_PATH;
                workbook.path = path;
                const body = await api('/api/desktop/office/workbook', {
                    method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ path, workbook, office_version: officeVersion })
            });
            officeVersion = body.office_version || officeVersion;
            setPath(path);
            setDirty(false);
            setStatus(t('desktop.sheets_saved'));
            notify({ type: 'success', message: t('desktop.sheets_saved') });
            await refreshDesktop();
            } finally {
                isSaving = false;
            }
        }

        function newWorkbook() {
            officeVersion = null;
            activeSheet = 0;
            selection = { anchor: { row: 0, col: 0 }, focus: { row: 0, col: 0 } };
            workbook = emptyWorkbook(nextUntitledPath('.xlsx'));
            undoStack = [];
            redoStack = [];
            setPath(workbook.path);
            setStatus('');
            setDirty(false);
            renderWorkbook();
        }

        async function openWorkbookFromDialog() {
            if (typeof ctx.openFileDialog !== 'function') return;
            const result = await ctx.openFileDialog({
                title: t('desktop.file_dialog_open'),
                initialPath: pathDir(currentPath),
                filters: [{ label: t('desktop.app_sheets'), extensions: ['.xlsx', '.xlsm', '.csv'] }]
            });
            if (!result || result.canceled || !result.path) return;
            undoStack = [];
            redoStack = [];
            setPath(result.path);
            await load();
        }

        async function saveAs() {
            if (readonly) return;
            if (typeof ctx.saveFileDialog === 'function') {
                const result = await ctx.saveFileDialog({
                    title: t('desktop.sheets_save_as'),
                    initialPath: pathDir(pathInput.value.trim() || currentPath || DEFAULT_PATH),
                    defaultName: currentFileEntry().name,
                    defaultExtension: '.xlsx',
                    filters: [{ label: t('desktop.app_sheets'), extensions: ['.xlsx', '.xlsm', '.csv'] }]
                });
                if (!result || result.canceled || !result.path) return;
                const previousPath = currentPath;
                const previousVersion = officeVersion;
                setPath(result.path);
                officeVersion = null;
                try { await save(); } catch (err) { officeVersion = previousVersion; setPath(previousPath); throw err; }
                return;
            }
            const prompt = ctx.promptDialog || (async () => null);
            const nextPath = await prompt(t('desktop.sheets_save_as'), pathInput.value.trim() || DEFAULT_PATH);
            if (nextPath == null) return;
            const trimmed = String(nextPath).trim();
            if (!trimmed) return;
            const previousPath = currentPath;
            const previousVersion = officeVersion;
            setPath(trimmed);
            officeVersion = null;
            try { await save(); } catch (err) { officeVersion = previousVersion; setPath(previousPath); throw err; }
        }

        function exportURL(format) {
            const path = pathInput.value.trim() || DEFAULT_PATH;
            return '/api/desktop/office/export?path=' + encodeURIComponent(path) + '&format=' + encodeURIComponent(format);
        }

        async function openExport(format) {
            if (!pathInput.value.trim() && !readonly) await save();
            if (typeof ctx.exportDesktopFile === 'function') {
                const entry = currentFileEntry();
                const base = entry.name.replace(/\.[^.]+$/, '') || 'workbook';
                await ctx.exportDesktopFile({ path: entry.path, name: base + '.' + format, url: exportURL(format) });
                return;
            }
            window.open(exportURL(format), '_blank', 'noopener');
        }

        function currentFileEntry() {
            const path = pathInput.value.trim() || currentPath || DEFAULT_PATH;
            return { path, name: path.split('/').filter(Boolean).pop() || path };
        }

        async function runAgentTask() {
            const prompt = ctx.promptDialog || (async () => null);
            const task = await prompt(t('desktop.agent_task_title'), '');
            if (!task) return;
            await save();
            if (typeof ctx.openAgentChatForFile === 'function') ctx.openAgentChatForFile(currentFileEntry(), { task, autosend: true, sourceApp: 'sheets' });
        }

        async function sendToAgentChat() {
            await save();
            if (typeof ctx.openAgentChatForFile === 'function') ctx.openAgentChatForFile(currentFileEntry(), { sourceApp: 'sheets' });
        }

        function setWindowMenus() {
            if (typeof ctx.setWindowMenus !== 'function') return;
            ctx.setWindowMenus(windowId, [
                {
                    id: 'file', labelKey: 'desktop.menu_file',
                    items: [
                        { id: 'new-workbook', labelKey: 'desktop.sheets_new', icon: 'file-plus', shortcut: 'Ctrl+N', disabled: readonly, action: newWorkbook },
                        { id: 'open-workbook', labelKey: 'desktop.file_dialog_open', icon: 'folder-open', shortcut: 'Ctrl+O', action: () => openWorkbookFromDialog().catch(err => setStatus(err.message || String(err))) },
                        { id: 'save', labelKey: 'desktop.sheets_save', icon: 'save', shortcut: 'Ctrl+S', disabled: readonly, action: () => save().catch(err => setStatus(err.message || String(err))) },
                        { id: 'save-as', labelKey: 'desktop.sheets_save_as', icon: 'save', disabled: readonly, action: () => saveAs().catch(err => setStatus(err.message || String(err))) },
                        { type: 'separator' },
                        { id: 'download-xlsx', labelKey: 'desktop.sheets_download_xlsx', icon: 'download', action: () => openExport('xlsx').catch(err => setStatus(err.message || String(err))) },
                        { id: 'export-csv', labelKey: 'desktop.sheets_export_csv', icon: 'spreadsheet', action: () => openExport('csv').catch(err => setStatus(err.message || String(err))) }
                    ]
                },
                {
                    id: 'edit', labelKey: 'desktop.menu_edit',
                    items: [
                        { id: 'undo', labelKey: 'desktop.sheets_undo', icon: 'undo', shortcut: 'Ctrl+Z', disabled: readonly || !undoStack.length, action: undo },
                        { id: 'redo', labelKey: 'desktop.sheets_redo', icon: 'redo', shortcut: 'Ctrl+Y', disabled: readonly || !redoStack.length, action: redo },
                        { type: 'separator' },
                        { id: 'copy', labelKey: 'desktop.fm.copy', icon: 'copy', shortcut: 'Ctrl+C', action: copyRange },
                        { id: 'paste', labelKey: 'desktop.fm.paste', icon: 'clipboard', shortcut: 'Ctrl+V', disabled: readonly, action: () => pasteRange() },
                        { id: 'clear', labelKey: 'desktop.sheets_clear_range', icon: 'x', shortcut: 'Del', disabled: readonly, action: clearRange },
                        { type: 'separator' },
                        { id: 'search', labelKey: 'desktop.sheets_search', icon: 'search', shortcut: 'Ctrl+F', action: openSearch },
                        { type: 'separator' },
                        { id: 'delete-rows', labelKey: 'desktop.sheets_delete_rows', icon: 'trash', disabled: readonly, action: deleteSelectedRows },
                        { id: 'delete-cols', labelKey: 'desktop.sheets_delete_columns', icon: 'trash', disabled: readonly, action: deleteSelectedColumns }
                    ]
                },
                {
                    id: 'insert', labelKey: 'desktop.menu_insert',
                    items: [
                        { id: 'add-row', labelKey: 'desktop.sheets_add_row', icon: 'list', disabled: readonly, action: () => insertRow(captureDisplayRows().length) },
                        { id: 'add-col', labelKey: 'desktop.sheets_add_column', icon: 'grid', disabled: readonly, action: () => insertColumn(Math.max(MIN_COLS, maxCols(captureDisplayRows()))) },
                        { type: 'separator' },
                        { id: 'add-sheet', labelKey: 'desktop.sheets_add_sheet', icon: 'plus', disabled: readonly, action: addNewSheet },
                        { type: 'separator' },
                        { id: 'insert-row-above', labelKey: 'desktop.sheets_insert_row_above', icon: 'list', disabled: readonly, action: () => insertRow(selectionRange().startRow) },
                        { id: 'insert-row-below', labelKey: 'desktop.sheets_insert_row_below', icon: 'list', disabled: readonly, action: () => insertRow(selectionRange().endRow + 1) },
                        { id: 'insert-col-left', labelKey: 'desktop.sheets_insert_col_left', icon: 'grid', disabled: readonly, action: () => insertColumn(selectionRange().startCol) },
                        { id: 'insert-col-right', labelKey: 'desktop.sheets_insert_col_right', icon: 'grid', disabled: readonly, action: () => insertColumn(selectionRange().endCol + 1) }
                    ]
                },
                {
                    id: 'agent', labelKey: 'desktop.menu_agent',
                    items: [
                        { id: 'agent-task', labelKey: 'desktop.agent_task_for_agent', icon: 'agent', action: () => runAgentTask().catch(err => { setStatus(err.message || String(err)); notify({ type: 'error', message: err.message || String(err) }); }) },
                        { id: 'agent-send-chat', labelKey: 'desktop.agent_send_to_chat', icon: 'chat', action: () => sendToAgentChat().catch(err => { setStatus(err.message || String(err)); notify({ type: 'error', message: err.message || String(err) }); }) }
                    ]
                }
            ]);
        }

        async function load() {
            const adapter = window.AuraUniverSheetsAdapter;
            if (adapter && typeof adapter.render === 'function') { adapter.render(host, windowId, ctx); return; }
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
            undoStack = [];
            redoStack = [];
            setDirty(false);
            renderWorkbook();
        }

        host.querySelector('[data-action="apply-formula"]').addEventListener('click', applyFormulaBar);
        formulaInput.addEventListener('keydown', event => {
            if (event.key === 'Enter') { event.preventDefault(); applyFormulaBar(); }
        });
        pathInput.addEventListener('change', () => { setPath(pathInput.value.trim() || DEFAULT_PATH); load(); });

        setWindowMenus();
        load();
        const inst = instances.get(windowId);
        if (inst) { inst.undo = undo; inst.redo = redo; inst.windowContent = windowContent; }

        function recalcFormulas() {
            const sheet = workbook.sheets[activeSheet];
            if (!sheet || !sheet.rows) return;
            const evaluate = (window.SheetsFormulas && window.SheetsFormulas.evaluate) || (() => '#ERR');
            const fmtMod = window.SheetsFormat;
            sheet.rows.forEach((row, r) => {
                if (!Array.isArray(row)) return;
                row.forEach((cell, c) => {
                    if (!cell || !cell.formula) return;
                    const input = gridHost.querySelector(`input[data-row="${r}"][data-col="${c}"]`);
                    if (!input) return;
                    const rawResult = evaluate(sheet, cell.formula);
                    input.dataset.displayValue = rawResult;
                    input.dataset.rawValue = rawResult;
                    const numFmt = cell.format && cell.format.numFormat;
                    const displayValue = (numFmt && fmtMod) ? fmtMod.formatDisplayValue(rawResult, numFmt) : rawResult;
                    if (document.activeElement !== input) {
                        input.value = displayValue;
                    }
                });
            });
        }

        function applyReadonlyState() {
            host.querySelectorAll('[data-action="apply-formula"]').forEach(button => { button.disabled = readonly; });
            if (formulaInput) formulaInput.disabled = readonly;
            setWindowMenus();
        }
    }

    function dispose(windowId) {
        const instance = instances.get(windowId);
        if (!instance) return;
        if (typeof instance.closeContextMenu === 'function') instance.closeContextMenu();
        if (instance.autosaveTimer) clearTimeout(instance.autosaveTimer);
        if (typeof instance.closeSearch === 'function') instance.closeSearch();
        if (instance.formatClickHandler) document.removeEventListener('click', instance.formatClickHandler);
        if (instance.windowContent) instance.windowContent.classList.remove('office-sheets-window-content');
        instances.delete(windowId);
    }

    async function fetchJSON(url, options) {
        const resp = await fetch(url, options);
        const body = await resp.json().catch(() => ({}));
        if (!resp.ok) throw new Error(body.error || body.message || ('HTTP ' + resp.status));
        return body;
    }

    function emptyWorkbook(path) { return { path, sheets: [{ name: 'Sheet1', rows: [] }] }; }

    function nextUntitledPath(ext) {
        const stamp = new Date().toISOString().replace(/[-:]/g, '').replace(/\..+$/, '').replace('T', '-');
        return 'Documents/untitled-' + stamp + ext;
    }

    function pathDir(path) {
        const parts = String(path || '').split('/').filter(Boolean);
        parts.pop();
        return parts.join('/') || 'Documents';
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

    function cellIsEmpty(cell) {
        if (!cell) return true;
        const v = cell.value != null ? String(cell.value) : '';
        const f = cell.formula || '';
        return v.trim() === '' && f.trim() === '';
    }

    function trimRows(rows) {
        let lastRow = -1;
        rows.forEach((row, r) => {
            if (row.some(cell => !cellIsEmpty(cell))) lastRow = r;
        });
        if (lastRow < 0) return [];
        return rows.slice(0, lastRow + 1).map(row => {
            let lastCol = -1;
            row.forEach((cell, c) => {
                if (!cellIsEmpty(cell)) lastCol = c;
            });
            return row.slice(0, lastCol + 1);
        });
    }

    function ensureRows(rows, rowCount, colCount) {
        while (rows.length < rowCount) rows.push([]);
        rows.forEach(row => { while (row.length < colCount) row.push({ value: '' }); });
    }

    function maxCols(rows) { return Math.max(0, ...((rows || []).map(row => Array.isArray(row) ? row.length : 0))); }

    function displayCell(cell, applyNumFormat) {
        if (!cell) return '';
        if (cell.formula) return '=' + String(cell.formula);
        const raw = cell.value || '';
        if (applyNumFormat && cell.format && cell.format.numFormat) {
            const fmtMod = window.SheetsFormat;
            return fmtMod ? fmtMod.formatDisplayValue(raw, cell.format.numFormat) : raw;
        }
        return raw;
    }

    function cellInputAttributes(cell, sheet) {
        const fmtMod = window.SheetsFormat;
        const numFmt = cell && cell.format && cell.format.numFormat;
        if (!cell || !cell.formula) {
            const raw = cell ? (cell.value || '') : '';
            const display = (numFmt && fmtMod) ? fmtMod.formatDisplayValue(raw, numFmt) : raw;
            const escDisplay = display.replace(/&/g, '&amp;').replace(/"/g, '&quot;');
            const escRaw = raw.replace(/&/g, '&amp;').replace(/"/g, '&quot;');
            let attrs = `value="${escDisplay}" data-raw-value="${escRaw}"`;
            if (numFmt) attrs += ` data-num-format="${numFmt.replace(/&/g, '&amp;').replace(/"/g, '&quot;')}"`;
            return attrs;
        }
        const formula = String(cell.formula).replace(/^=/, '');
        const evaluate = (window.SheetsFormulas && window.SheetsFormulas.evaluate) || (() => '#ERR');
        const rawResult = evaluate(sheet, formula);
        const displayValue = (numFmt && fmtMod) ? fmtMod.formatDisplayValue(rawResult, numFmt) : rawResult;
        let attrs = `value="${displayValue.replace(/&/g, '&amp;').replace(/"/g, '&quot;')}" data-formula="${formula.replace(/&/g, '&amp;').replace(/"/g, '&quot;')}" data-display-value="${rawResult.replace(/&/g, '&amp;').replace(/"/g, '&quot;')}" data-raw-value="${rawResult.replace(/&/g, '&amp;').replace(/"/g, '&quot;')}" title="=${formula.replace(/&/g, '&amp;').replace(/"/g, '&quot;')}"`;
        if (numFmt) attrs += ` data-num-format="${numFmt.replace(/&/g, '&amp;').replace(/"/g, '&quot;')}"`;
        return attrs;
    }

        function syncFormulaDataset(input, cell, sheet) {
            if (!input) return;
            if (cell && cell.formula) {
                const formula = String(cell.formula).replace(/^=/, '');
                const evaluate = (window.SheetsFormulas && window.SheetsFormulas.evaluate) || (() => '#ERR');
                const displayValue = evaluate(sheet, formula);
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
        if (!input) return;
        if (input.dataset.formula) {
            input.value = '=' + input.dataset.formula;
        } else if (input.dataset.rawValue && input.dataset.numFormat) {
            input.value = input.dataset.rawValue;
        }
    }

    function showFormulaResult(input) {
        if (!input) return;
        if (input.dataset.formula) {
            const fmtMod = window.SheetsFormat;
            const raw = input.dataset.displayValue || input.dataset.rawValue || '';
            const numFmt = input.dataset.numFormat;
            input.value = (numFmt && fmtMod) ? fmtMod.formatDisplayValue(raw, numFmt) : raw;
        } else if (input.dataset.rawValue && input.dataset.numFormat) {
            const fmtMod = window.SheetsFormat;
            input.value = fmtMod ? fmtMod.formatDisplayValue(input.dataset.rawValue, input.dataset.numFormat) : input.dataset.rawValue;
        }
    }

    function cellNameFallback(row, col) {
        return columnNameFallback(col + 1) + String(row + 1);
    }

    function columnNameFallback(index) {
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
        Object.entries(vars || {}).forEach(([key, value]) => { result = result.replaceAll('{{' + key + '}}', String(value)); });
        return result;
    }

    window.SheetsApp = window.SheetsApp || {};
    window.SheetsApp.render = render;
    window.SheetsApp.dispose = dispose;
})();
