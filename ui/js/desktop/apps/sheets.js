(function () {
    'use strict';

    const DEFAULT_PATH = 'Documents/untitled.xlsx';
    const MIN_ROWS = 24;
    const MIN_COLS = 10;

    function render(host, windowId, context) {
        if (!host) return;
        const ctx = context || {};
        const esc = ctx.esc || (value => String(value == null ? '' : value));
        const t = ctx.t || ((key, fallback) => fallback || key);
        const api = ctx.api || fetchJSON;
        const iconMarkup = ctx.iconMarkup || ((key, fallback) => `<span>${esc(fallback || key || '')}</span>`);
        const notify = ctx.notify || (() => {});
        const refreshDesktop = ctx.loadBootstrap || (() => Promise.resolve());
        let currentPath = ctx.path || DEFAULT_PATH;
        let activeSheet = 0;
        let workbook = emptyWorkbook(currentPath);

        host.innerHTML = `<div class="office-app office-sheets" data-office-sheets="${esc(windowId)}">
            <div class="vd-toolbar office-toolbar">
                <button class="vd-tool-button" type="button" data-action="save">${iconMarkup('save', 'S', 'vd-tool-icon', 15)}<span>${esc(t('desktop.sheets_save', 'Save'))}</span></button>
                <a class="vd-tool-button" data-action="download" href="#" download>${iconMarkup('download', 'D', 'vd-tool-icon', 15)}<span>${esc(t('desktop.sheets_download_xlsx', 'XLSX'))}</span></a>
                <a class="vd-tool-button" data-action="export-csv" href="#" download>${iconMarkup('spreadsheet', 'C', 'vd-tool-icon', 15)}<span>${esc(t('desktop.sheets_export_csv', 'CSV'))}</span></a>
                <button class="vd-tool-button" type="button" data-action="add-row">${iconMarkup('list', '+', 'vd-tool-icon', 15)}<span>${esc(t('desktop.sheets_add_row', 'Row'))}</span></button>
                <button class="vd-tool-button" type="button" data-action="add-col">${iconMarkup('grid', '+', 'vd-tool-icon', 15)}<span>${esc(t('desktop.sheets_add_column', 'Column'))}</span></button>
                <input class="office-path-input" data-path value="${esc(currentPath)}" spellcheck="false" autocomplete="off">
                <span class="vd-chat-meta" data-status>${esc(t('desktop.sheets_loading', 'Loading...'))}</span>
            </div>
            <div class="office-sheet-tabs" data-tabs></div>
            <div class="office-sheet-grid-wrap" data-grid></div>
        </div>`;

        const pathInput = host.querySelector('[data-path]');
        const status = host.querySelector('[data-status]');
        const tabsHost = host.querySelector('[data-tabs]');
        const gridHost = host.querySelector('[data-grid]');

        function setStatus(message) {
            if (status) status.textContent = message || '';
        }

        function setPath(path) {
            currentPath = path || DEFAULT_PATH;
            pathInput.value = currentPath;
            updateExportLinks();
        }

        function updateExportLinks() {
            const path = pathInput.value.trim() || DEFAULT_PATH;
            const base = '/api/desktop/office/export?path=' + encodeURIComponent(path);
            const download = host.querySelector('[data-action="download"]');
            const csv = host.querySelector('[data-action="export-csv"]');
            if (download) download.href = base + '&format=xlsx';
            if (csv) csv.href = base + '&format=csv';
        }

        function renderWorkbook() {
            workbook = normalizeWorkbook(workbook, pathInput.value.trim() || DEFAULT_PATH);
            activeSheet = Math.min(activeSheet, workbook.sheets.length - 1);
            const sheet = workbook.sheets[activeSheet];
            tabsHost.innerHTML = workbook.sheets.map((sheet, index) => `<button type="button" class="${index === activeSheet ? 'active' : ''}" data-sheet-index="${index}">${esc(sheet.name || (t('desktop.sheets_sheet', 'Sheet') + ' ' + (index + 1)))}</button>`).join('');
            tabsHost.querySelectorAll('[data-sheet-index]').forEach(btn => {
                btn.addEventListener('click', () => {
                    captureGrid();
                    activeSheet = Number(btn.dataset.sheetIndex) || 0;
                    renderWorkbook();
                });
            });
            const rows = padRows(sheet.rows || [], MIN_ROWS, MIN_COLS);
            const colHeaders = Array.from({ length: rows[0].length }, (_, i) => columnName(i + 1));
            gridHost.innerHTML = `<table class="office-grid">
                <thead><tr><th></th>${colHeaders.map(col => `<th>${esc(col)}</th>`).join('')}</tr></thead>
                <tbody>${rows.map((row, r) => `<tr><th>${r + 1}</th>${row.map((cell, c) => `<td><input data-row="${r}" data-col="${c}" value="${esc(displayCell(cell))}" spellcheck="false"></td>`).join('')}</tr>`).join('')}</tbody>
            </table>`;
        }

        function captureGrid() {
            const sheet = workbook.sheets[activeSheet];
            if (!sheet) return;
            const rows = [];
            gridHost.querySelectorAll('tbody tr').forEach((tr, r) => {
                const row = [];
                tr.querySelectorAll('input').forEach((input, c) => {
                    const raw = input.value;
                    row[c] = raw.startsWith('=') ? { formula: raw.slice(1) } : { value: raw };
                });
                rows[r] = row;
            });
            sheet.rows = trimRows(rows);
        }

        async function save() {
            captureGrid();
            setStatus(t('desktop.saving', 'Saving...'));
            const path = pathInput.value.trim() || DEFAULT_PATH;
            workbook.path = path;
            await api('/api/desktop/office/workbook', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ path, workbook })
            });
            setPath(path);
            setStatus(t('desktop.sheets_saved', 'Saved'));
            notify({ type: 'success', message: t('desktop.sheets_saved', 'Saved') });
            await refreshDesktop();
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
                setPath((body.entry && body.entry.path) || workbook.path || currentPath);
                setStatus('');
            } catch (err) {
                workbook = emptyWorkbook(currentPath);
                setStatus('');
                if (ctx.path) notify({ type: 'info', message: err.message || String(err) });
            }
            renderWorkbook();
        }

        host.querySelector('[data-action="save"]').addEventListener('click', () => {
            save().catch(err => {
                setStatus(err.message || String(err));
                notify({ type: 'error', message: err.message || String(err) });
            });
        });
        host.querySelector('[data-action="add-row"]').addEventListener('click', () => {
            captureGrid();
            const sheet = workbook.sheets[activeSheet];
            const cols = Math.max(MIN_COLS, maxCols(sheet.rows));
            sheet.rows.push(Array.from({ length: cols }, () => ({ value: '' })));
            renderWorkbook();
        });
        host.querySelector('[data-action="add-col"]').addEventListener('click', () => {
            captureGrid();
            const sheet = workbook.sheets[activeSheet];
            if (!sheet.rows.length) sheet.rows.push([]);
            sheet.rows.forEach(row => row.push({ value: '' }));
            renderWorkbook();
        });
        pathInput.addEventListener('change', () => {
            setPath(pathInput.value.trim() || DEFAULT_PATH);
            load();
        });

        load();
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

    function maxCols(rows) {
        return Math.max(0, ...((rows || []).map(row => Array.isArray(row) ? row.length : 0)));
    }

    function displayCell(cell) {
        if (!cell) return '';
        if (cell.formula) return '=' + String(cell.formula).replace(/^=/, '');
        return cell.value || '';
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

    window.SheetsApp = { render };
})();
