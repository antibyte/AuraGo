(function () {
    'use strict';

    function createState() {
        return { overlay: null, matches: [], current: -1, query: '', matchCase: false, callbacks: null };
    }

    function openSearch(host, gridHost, workbook, getActiveSheet, t, esc, onSelectCell, callbacks) {
        const state = createState();
        state.callbacks = callbacks || {};
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
        state.overlay = overlay;

        const searchInput = overlay.querySelector('[data-search-input]');
        const matchCaseCheck = overlay.querySelector('[data-match-case]');
        const countSpan = overlay.querySelector('[data-search-count]');

        searchInput.addEventListener('input', () => {
            state.query = searchInput.value;
            performSearch(state, gridHost, workbook, getActiveSheet(), t, countSpan);
        });
        matchCaseCheck.addEventListener('change', () => {
            state.matchCase = matchCaseCheck.checked;
            performSearch(state, gridHost, workbook, getActiveSheet(), t, countSpan);
        });
        searchInput.addEventListener('keydown', e => {
            if (e.key === 'Enter') { e.preventDefault(); navigateNext(state, gridHost, countSpan, t, onSelectCell); }
            if (e.key === 'Escape') { closeInstance(state); }
        });

        overlay.querySelector('[data-action="find-next"]').addEventListener('click', () => navigateNext(state, gridHost, countSpan, t, onSelectCell));
        overlay.querySelector('[data-action="find-prev"]').addEventListener('click', () => navigatePrev(state, gridHost, countSpan, t, onSelectCell));
        overlay.querySelector('[data-action="close"]').addEventListener('click', () => closeInstance(state));
        overlay.querySelector('[data-action="replace"]').addEventListener('click', () => replaceCurrent(state, gridHost, workbook, getActiveSheet(), overlay, countSpan, t, onSelectCell));
        overlay.querySelector('[data-action="replace-all"]').addEventListener('click', () => replaceAll(state, gridHost, workbook, getActiveSheet(), overlay, countSpan, t));

        searchInput.focus();
        return state;
    }

    function closeInstance(state) {
        if (state.overlay) {
            state.overlay.remove();
            state.overlay = null;
        }
        state.matches = [];
        state.current = -1;
        state.query = '';
        state.matchCase = false;
        state.callbacks = null;
    }

    function closeSearch() {
        document.querySelectorAll('.office-search-overlay').forEach(el => el.remove());
        document.querySelectorAll('.office-cell-search-match').forEach(el => el.classList.remove('office-cell-search-match'));
    }

    function performSearch(state, gridHost, workbook, activeSheet, t, countSpan) {
        gridHost.querySelectorAll('.office-cell-search-match').forEach(el => el.classList.remove('office-cell-search-match'));
        state.matches = [];
        state.current = -1;
        if (!state.query) {
            if (countSpan) countSpan.textContent = '';
            return;
        }
        const sheet = workbook.sheets[activeSheet];
        if (!sheet || !sheet.rows) return;
        const query = state.matchCase ? state.query : state.query.toLowerCase();
        sheet.rows.forEach((row, r) => {
            if (!Array.isArray(row)) return;
            row.forEach((cell, c) => {
                if (!cell) return;
                const value = cell.value != null ? String(cell.value) : '';
                const formula = cell.formula || '';
                const haystack = formula ? (value + ' ' + formula) : value;
                const compare = state.matchCase ? haystack : haystack.toLowerCase();
                if (compare.includes(query)) {
                    state.matches.push({ row: r, col: c });
                    const td = gridHost.querySelector(`td[data-cell-row="${r}"][data-cell-col="${c}"]`);
                    if (td) td.classList.add('office-cell-search-match');
                }
            });
        });
        if (state.matches.length > 0) {
            state.current = 0;
            if (countSpan) countSpan.textContent = '1 of ' + state.matches.length;
        } else {
            if (countSpan) countSpan.textContent = t('desktop.sheets_no_matches');
        }
    }

    function navigateNext(state, gridHost, countSpan, t, onSelectCell) {
        if (!state.matches.length) return;
        state.current = (state.current + 1) % state.matches.length;
        highlightCurrent(state, gridHost, countSpan, t, onSelectCell);
    }

    function navigatePrev(state, gridHost, countSpan, t, onSelectCell) {
        if (!state.matches.length) return;
        state.current = (state.current - 1 + state.matches.length) % state.matches.length;
        highlightCurrent(state, gridHost, countSpan, t, onSelectCell);
    }

    function highlightCurrent(state, gridHost, countSpan, t, onSelectCell) {
        if (countSpan) countSpan.textContent = (state.current + 1) + ' of ' + state.matches.length;
        const match = state.matches[state.current];
        if (!match) return;
        const input = gridHost.querySelector(`input[data-row="${match.row}"][data-col="${match.col}"]`);
        if (input) { input.focus(); input.select(); }
        if (typeof onSelectCell === 'function') onSelectCell(match.row, match.col);
    }

    function replaceCurrent(state, gridHost, workbook, activeSheet, overlay, countSpan, t, onSelectCell) {
        if (!state.matches.length || state.current < 0) return;
        const replaceInput = overlay.querySelector('[data-replace-input]');
        if (!replaceInput) return;
        if (state.callbacks && typeof state.callbacks.pushSnapshot === 'function') state.callbacks.pushSnapshot();
        const replaceVal = replaceInput.value;
        const match = state.matches[state.current];
        const sheet = workbook.sheets[activeSheet];
        if (!sheet || !sheet.rows || !sheet.rows[match.row] || !sheet.rows[match.row][match.col]) return;
        const cell = sheet.rows[match.row][match.col];
        const query = state.matchCase ? state.query : state.query.toLowerCase();
        if (cell.formula) {
            const formulaText = cell.formula;
            const lowerFormula = state.matchCase ? formulaText : formulaText.toLowerCase();
            const idx = lowerFormula.indexOf(query);
            if (idx >= 0) {
                const newFormula = formulaText.substring(0, idx) + replaceVal + formulaText.substring(idx + state.query.length);
                if (newFormula.startsWith('=')) {
                    cell.formula = newFormula.substring(1);
                } else {
                    cell.value = newFormula;
                    delete cell.formula;
                }
            }
        } else {
            const oldValue = cell.value != null ? String(cell.value) : '';
            const lowerOld = state.matchCase ? oldValue : oldValue.toLowerCase();
            const idx = lowerOld.indexOf(query);
            if (idx >= 0) {
                cell.value = oldValue.substring(0, idx) + replaceVal + oldValue.substring(idx + state.query.length);
            }
        }
        const input = gridHost.querySelector(`input[data-row="${match.row}"][data-col="${match.col}"]`);
        if (input) {
            if (cell.formula) {
                const evaluate = (window.SheetsFormulas && window.SheetsFormulas.evaluate) || (() => '#ERR');
                const displayValue = evaluate(workbook.sheets[activeSheet], cell.formula);
                input.value = displayValue;
                input.dataset.formula = cell.formula;
                input.dataset.displayValue = displayValue;
                input.title = '=' + cell.formula;
            } else {
                input.value = cell.value || '';
                delete input.dataset.formula;
                delete input.dataset.displayValue;
                input.removeAttribute('title');
            }
        }
        if (state.callbacks && typeof state.callbacks.setDirty === 'function') state.callbacks.setDirty(true);
        performSearch(state, gridHost, workbook, activeSheet, t, countSpan);
    }

    function replaceAll(state, gridHost, workbook, activeSheet, overlay, countSpan, t) {
        if (!state.query) return;
        const replaceInput = overlay.querySelector('[data-replace-input]');
        if (!replaceInput) return;
        if (state.callbacks && typeof state.callbacks.pushSnapshot === 'function') state.callbacks.pushSnapshot();
        const replaceVal = replaceInput.value;
        const sheet = workbook.sheets[activeSheet];
        if (!sheet || !sheet.rows) return;
        let count = 0;
        sheet.rows.forEach((row, r) => {
            if (!Array.isArray(row)) return;
            row.forEach((cell, c) => {
                if (!cell) return;
                const query = state.matchCase ? state.query : state.query.toLowerCase();
                if (cell.formula) {
                    const formulaText = cell.formula;
                    const lowerFormula = state.matchCase ? formulaText : formulaText.toLowerCase();
                    if (lowerFormula.includes(query)) {
                        let newVal = formulaText;
                        let idx = 0;
                        while (true) {
                            const lowerNew = state.matchCase ? newVal : newVal.toLowerCase();
                            const found = lowerNew.indexOf(query, idx);
                            if (found < 0) break;
                            newVal = newVal.substring(0, found) + replaceVal + newVal.substring(found + state.query.length);
                            idx = found + replaceVal.length;
                            count++;
                        }
                        cell.formula = newVal;
                        const input = gridHost.querySelector(`input[data-row="${r}"][data-col="${c}"]`);
                        if (input) {
                            const evaluate = (window.SheetsFormulas && window.SheetsFormulas.evaluate) || (() => '#ERR');
                            const displayValue = evaluate(sheet, cell.formula);
                            input.value = displayValue;
                            input.dataset.formula = cell.formula;
                            input.dataset.displayValue = displayValue;
                            input.title = '=' + cell.formula;
                        }
                    }
                } else {
                    const oldValue = cell.value != null ? String(cell.value) : '';
                    const lowerOld = state.matchCase ? oldValue : oldValue.toLowerCase();
                    if (lowerOld.includes(query)) {
                        let newVal = oldValue;
                        let idx = 0;
                        while (true) {
                            const lowerNew = state.matchCase ? newVal : newVal.toLowerCase();
                            const found = lowerNew.indexOf(query, idx);
                            if (found < 0) break;
                            newVal = newVal.substring(0, found) + replaceVal + newVal.substring(found + state.query.length);
                            idx = found + replaceVal.length;
                            count++;
                        }
                        cell.value = newVal;
                        const input = gridHost.querySelector(`input[data-row="${r}"][data-col="${c}"]`);
                        if (input) input.value = newVal;
                    }
                }
            });
        });
        if (state.callbacks && typeof state.callbacks.setDirty === 'function') state.callbacks.setDirty(true);
        performSearch(state, gridHost, workbook, activeSheet, t, countSpan);
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
