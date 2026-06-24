(function () {
    'use strict';

    let searchOverlay = null;
    let searchState = { matches: [], current: -1, query: '', matchCase: false };
    let searchCallbacks = null;

    function openSearch(host, gridHost, workbook, getActiveSheet, t, esc, onSelectCell, callbacks) {
        closeSearch();
        searchCallbacks = callbacks || {};
        const overlay = document.createElement('div');
        overlay.className = 'office-search-overlay';
        overlay.innerHTML = `
            <div class="office-search-header">
                <input class="office-search-input" data-search-input placeholder="${esc(t('desktop.sheets_search'))}" spellcheck="false">
                <label class="office-search-label"><input type="checkbox" data-match-case> ${esc(t('desktop.sheets_match_case'))}</label>
                <span class="office-search-count" data-search-count></span>
                <button type="button" class="office-search-btn" data-action="find-prev" title="&#9650;">&#9650;</button>
                <button type="button" class="office-search-btn" data-action="find-next" title="&#9660;">&#9660;</button>
                <button type="button" class="office-search-btn" data-action="close" title="${esc(t('desktop.close'))}">&times;</button>
            </div>
            <div class="office-search-replace-row">
                <input class="office-search-input" data-replace-input placeholder="${esc(t('desktop.sheets_replace'))}" spellcheck="false">
                <button type="button" class="office-search-btn" data-action="replace">${esc(t('desktop.sheets_replace'))}</button>
                <button type="button" class="office-search-btn" data-action="replace-all">${esc(t('desktop.sheets_replace_all'))}</button>
            </div>`;
        host.querySelector('.office-app').appendChild(overlay);
        searchOverlay = overlay;
        searchState = { matches: [], current: -1, query: '', matchCase: false };

        const searchInput = overlay.querySelector('[data-search-input]');
        const matchCaseCheck = overlay.querySelector('[data-match-case]');
        const countSpan = overlay.querySelector('[data-search-count]');

        searchInput.addEventListener('input', () => {
            searchState.query = searchInput.value;
            performSearch(gridHost, workbook, getActiveSheet(), t, countSpan);
        });
        matchCaseCheck.addEventListener('change', () => {
            searchState.matchCase = matchCaseCheck.checked;
            performSearch(gridHost, workbook, getActiveSheet(), t, countSpan);
        });
        searchInput.addEventListener('keydown', e => {
            if (e.key === 'Enter') { e.preventDefault(); navigateNext(countSpan, t, onSelectCell); }
            if (e.key === 'Escape') { closeSearch(); }
        });

        overlay.querySelector('[data-action="find-next"]').addEventListener('click', () => navigateNext(countSpan, t, onSelectCell));
        overlay.querySelector('[data-action="find-prev"]').addEventListener('click', () => navigatePrev(countSpan, t, onSelectCell));
        overlay.querySelector('[data-action="close"]').addEventListener('click', closeSearch);
        overlay.querySelector('[data-action="replace"]').addEventListener('click', () => replaceCurrent(gridHost, workbook, getActiveSheet(), overlay, countSpan, t, onSelectCell));
        overlay.querySelector('[data-action="replace-all"]').addEventListener('click', () => replaceAll(gridHost, workbook, getActiveSheet(), overlay, countSpan, t));

        searchInput.focus();
    }

    function closeSearch() {
        if (searchOverlay) {
            searchOverlay.remove();
            searchOverlay = null;
        }
        searchState = { matches: [], current: -1, query: '', matchCase: false };
        searchCallbacks = null;
        document.querySelectorAll('.office-cell-search-match').forEach(el => el.classList.remove('office-cell-search-match'));
    }

    function performSearch(gridHost, workbook, activeSheet, t, countSpan) {
        document.querySelectorAll('.office-cell-search-match').forEach(el => el.classList.remove('office-cell-search-match'));
        searchState.matches = [];
        searchState.current = -1;
        if (!searchState.query) {
            if (countSpan) countSpan.textContent = '';
            return;
        }
        const sheet = workbook.sheets[activeSheet];
        if (!sheet || !sheet.rows) return;
        const query = searchState.matchCase ? searchState.query : searchState.query.toLowerCase();
        sheet.rows.forEach((row, r) => {
            if (!Array.isArray(row)) return;
            row.forEach((cell, c) => {
                if (!cell) return;
                const value = cell.value != null ? String(cell.value) : '';
                const formula = cell.formula || '';
                const haystack = formula ? (value + ' ' + formula) : value;
                const compare = searchState.matchCase ? haystack : haystack.toLowerCase();
                if (compare.includes(query)) {
                    searchState.matches.push({ row: r, col: c });
                    const td = gridHost.querySelector(`td[data-cell-row="${r}"][data-cell-col="${c}"]`);
                    if (td) td.classList.add('office-cell-search-match');
                }
            });
        });
        if (searchState.matches.length > 0) {
            searchState.current = 0;
            if (countSpan) countSpan.textContent = '1 of ' + searchState.matches.length;
        } else {
            if (countSpan) countSpan.textContent = t('desktop.sheets_no_matches');
        }
    }

    function navigateNext(countSpan, t, onSelectCell) {
        if (!searchState.matches.length) return;
        searchState.current = (searchState.current + 1) % searchState.matches.length;
        highlightCurrent(countSpan, t, onSelectCell);
    }

    function navigatePrev(countSpan, t, onSelectCell) {
        if (!searchState.matches.length) return;
        searchState.current = (searchState.current - 1 + searchState.matches.length) % searchState.matches.length;
        highlightCurrent(countSpan, t, onSelectCell);
    }

    function highlightCurrent(countSpan, t, onSelectCell) {
        if (countSpan) countSpan.textContent = (searchState.current + 1) + ' of ' + searchState.matches.length;
        const match = searchState.matches[searchState.current];
        if (!match) return;
        const input = document.querySelector(`input[data-row="${match.row}"][data-col="${match.col}"]`);
        if (input) { input.focus(); input.select(); }
        if (typeof onSelectCell === 'function') onSelectCell(match.row, match.col);
    }

    function replaceCurrent(gridHost, workbook, activeSheet, overlay, countSpan, t, onSelectCell) {
        if (!searchState.matches.length || searchState.current < 0) return;
        const replaceInput = overlay.querySelector('[data-replace-input]');
        if (!replaceInput) return;
        if (searchCallbacks && typeof searchCallbacks.pushSnapshot === 'function') searchCallbacks.pushSnapshot();
        const replaceVal = replaceInput.value;
        const match = searchState.matches[searchState.current];
        const sheet = workbook.sheets[activeSheet];
        if (!sheet || !sheet.rows || !sheet.rows[match.row] || !sheet.rows[match.row][match.col]) return;
        const cell = sheet.rows[match.row][match.col];
        const oldValue = cell.value != null ? String(cell.value) : '';
        const query = searchState.matchCase ? searchState.query : searchState.query.toLowerCase();
        const lowerOld = searchState.matchCase ? oldValue : oldValue.toLowerCase();
        const idx = lowerOld.indexOf(query);
        if (idx >= 0) {
            cell.value = oldValue.substring(0, idx) + replaceVal + oldValue.substring(idx + searchState.query.length);
            delete cell.formula;
        }
        const input = gridHost.querySelector(`input[data-row="${match.row}"][data-col="${match.col}"]`);
        if (input) input.value = cell.value;
        if (searchCallbacks && typeof searchCallbacks.setDirty === 'function') searchCallbacks.setDirty(true);
        performSearch(gridHost, workbook, activeSheet, t, countSpan);
    }

    function replaceAll(gridHost, workbook, activeSheet, overlay, countSpan, t) {
        if (!searchState.query) return;
        const replaceInput = overlay.querySelector('[data-replace-input]');
        if (!replaceInput) return;
        if (searchCallbacks && typeof searchCallbacks.pushSnapshot === 'function') searchCallbacks.pushSnapshot();
        const replaceVal = replaceInput.value;
        const sheet = workbook.sheets[activeSheet];
        if (!sheet || !sheet.rows) return;
        let count = 0;
        sheet.rows.forEach((row, r) => {
            if (!Array.isArray(row)) return;
            row.forEach((cell, c) => {
                if (!cell) return;
                const oldValue = cell.value != null ? String(cell.value) : '';
                const query = searchState.matchCase ? searchState.query : searchState.query.toLowerCase();
                const lowerOld = searchState.matchCase ? oldValue : oldValue.toLowerCase();
                if (lowerOld.includes(query)) {
                    let newVal = oldValue;
                    let idx = 0;
                    while (true) {
                        const lowerNew = searchState.matchCase ? newVal : newVal.toLowerCase();
                        const found = lowerNew.indexOf(query, idx);
                        if (found < 0) break;
                        newVal = newVal.substring(0, found) + replaceVal + newVal.substring(found + searchState.query.length);
                        idx = found + replaceVal.length;
                        count++;
                    }
                    cell.value = newVal;
                    delete cell.formula;
                    const input = gridHost.querySelector(`input[data-row="${r}"][data-col="${c}"]`);
                    if (input) input.value = newVal;
                }
            });
        });
        if (searchCallbacks && typeof searchCallbacks.setDirty === 'function') searchCallbacks.setDirty(true);
        performSearch(gridHost, workbook, activeSheet, t, countSpan);
    }

    window.SheetsSearch = {
        openSearch: openSearch,
        closeSearch: closeSearch,
        findNext: navigateNext,
        findPrev: navigatePrev,
        replace: replaceCurrent,
        replaceAll: replaceAll
    };
})();
