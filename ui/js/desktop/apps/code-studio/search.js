    function searchResultLabel(path) {
        const value = String(path || '');
        return value.startsWith(WORKSPACE_ROOT + '/') ? value.slice(WORKSPACE_ROOT.length + 1) : value;
    }

    // The search panel: a head with the VS Code style replace chevron, the search form and the replace
    // row (replace.js), then the result list. renderSearchResults only redraws the list, so typing in
    // the replace field keeps its caret.
    function renderSearchPanel() {
        const panel = shellPart('[data-search]');
        if (!panel) return;
        panel.hidden = !state.searchVisible;
        if (!state.searchVisible) return;
        const options = state.searchOptions || {};
        const replaceOpen = !!state.searchReplaceOpen;
        const toggleLabel = tr('codeStudio.findToggleReplace', 'Toggle replace');
        panel.innerHTML = `<div class="cs-search-head" data-replace-open="${replaceOpen ? 'true' : 'false'}">
            <button type="button" class="cs-search-replace-toggle" data-search-replace-toggle aria-expanded="${replaceOpen ? 'true' : 'false'}" title="${esc(toggleLabel)}" aria-label="${esc(toggleLabel)}"><span aria-hidden="true">›</span></button>
            <form class="cs-search-form" data-search-form>
                <input name="q" value="${esc(state.searchQuery || '')}" placeholder="${esc(tr('codeStudio.searchFiles', 'Search in Files'))}" autocomplete="off" spellcheck="false" inputmode="search" enterkeyhint="search">
                <input name="include" value="${esc(options.include || '')}" placeholder="*.go" autocomplete="off" spellcheck="false">
                <input name="exclude" value="${esc(options.exclude || '')}" placeholder="vendor/" autocomplete="off" spellcheck="false">
                <label><input type="checkbox" name="case"${options.case ? ' checked' : ''}> Aa</label>
                <label><input type="checkbox" name="whole"${options.whole ? ' checked' : ''}> Ab</label>
                <label><input type="checkbox" name="regex"${options.regex ? ' checked' : ''}> .*</label>
                <button type="submit" class="cs-button primary">${buttonIcon('search', 'S')}<span>${esc(tr('codeStudio.search', 'Search'))}</span></button>
                <button type="button" class="cs-icon-button" data-search-close title="${esc(tr('desktop.close', 'Close'))}">${iconMarkup('x', 'X', 'cs-icon-button-icon', 14)}</button>
            </form>${replaceOpen ? projectReplaceRowMarkup() : ''}
        </div><div class="cs-search-results" data-search-results></div>`;
        panel.querySelector('[data-search-form]').addEventListener('submit', bind(event => {
            event.preventDefault();
            runSearch(new FormData(event.currentTarget));
        }));
        panel.querySelector('[data-search-close]').addEventListener('click', bind(() => toggleSearch()));
        wireProjectReplaceControls(panel);
        renderSearchResults();
        const input = panel.querySelector('input[name="q"]');
        if (input) {
            input.addEventListener('keydown', bind(event => {
                if (event.key === 'Escape') {
                    event.preventDefault();
                    toggleSearch();
                }
            }));
            if (!state.searchPerformed || !input.value) {
                input.focus();
                input.select();
            }
        }
    }

    function renderSearchResults() {
        const panel = shellPart('[data-search]');
        const box = panel && panel.querySelector('[data-search-results]');
        if (!box) {
            if (panel && state.searchVisible) renderSearchPanel();
            return;
        }
        let results;
        if (!state.searchPerformed) {
            results = `<div class="cs-empty">${esc(tr('codeStudio.searchPrompt', 'Enter a search term and press Enter'))}</div>`;
        } else if (!state.searchResults.length) {
            results = `<div class="cs-empty">${esc(tr('codeStudio.noResults', 'No results found'))}</div>`;
        } else {
            results = `<div class="cs-search-summary">${esc(tr('codeStudio.resultsCount', '{{count}} results', { count: state.searchResults.length }))}</div>` +
                (state.searchTruncated ? `<div class="cs-search-summary">${esc(tr('codeStudio.resultsTruncated', 'Showing the first 200 results.'))}</div>` : '') +
                (state.searchReplaceOpen ? projectReplaceResultsMarkup() : state.searchResults.map(result => `
                <button type="button" class="cs-search-result" data-search-path="${esc(result.path)}" data-search-line="${esc(result.line)}" title="${esc(result.path)}">
                    <span><strong>${esc(searchResultLabel(result.path))}</strong><em class="cs-search-line">${esc(result.line)}</em></span>
                    <code>${esc(result.preview)}</code>
                </button>`).join(''));
        }
        box.innerHTML = results;
        box.querySelectorAll('[data-search-path]').forEach(btn => {
            btn.addEventListener('click', bind(() => openSearchResult(btn.dataset.searchPath, Number(btn.dataset.searchLine || 1))));
        });
        if (state.searchReplaceOpen) wireProjectReplaceResults(box);
        updateProjectReplaceControls();
    }

    function toggleSearch() {
        state.searchVisible = !state.searchVisible;
        renderSearchPanel();
        renderActivityBar();
        renderWindowMenus();
        if (state.searchVisible) {
            const input = shellPart('[data-search] input[name="q"]');
            if (input) {
                input.focus();
                input.select();
            }
        }
    }

    async function runSearch(formData) {
        const target = state;
        if (!isLiveInstance(target)) return;
        const query = String(formData.get('q') || '').trim();
        if (!query) return;
        const request = target.searchRequest = Symbol();
        state.searchQuery = query;
        state.searchOptions = {
            case: !!formData.get('case'),
            whole: !!formData.get('whole'),
            regex: !!formData.get('regex'),
            include: String(formData.get('include') || ''),
            exclude: String(formData.get('exclude') || '')
        };
        renderStatus(tr('codeStudio.search', 'Search') + '...');
        const currentPath = target.currentPath || WORKSPACE_ROOT;
        try {
            const result = await apiClient.search({
                q: query,
                path: currentPath,
                case: state.searchOptions.case ? 'true' : 'false',
                whole: state.searchOptions.whole ? 'true' : 'false',
                regex: state.searchOptions.regex ? 'true' : 'false',
                include: state.searchOptions.include,
                exclude: state.searchOptions.exclude
            });
            if (!isLiveInstance(target) || target.searchRequest !== request) return;
            runWithInstance(target, () => {
                // Open files are listed from their buffers (replace.js); the others as the server found them.
                state.searchResults = overlayOpenTabResults(result.results || []);
                state.searchTruncated = !!result.truncated || state.searchResults.length > 200;
                state.searchPerformed = true;
                renderSearchResults();
                flashStatus(tr('codeStudio.resultsCount', '{{count}} results', { count: state.searchResults.length }));
            });
        } catch (err) {
            if (isLiveInstance(target) && target.searchRequest === request) {
                runWithInstance(target, () => {
                    state.searchResults = [];
                    state.searchTruncated = false;
                    state.searchPerformed = true;
                    renderSearchResults();
                    showOperationError(err);
                });
            }
        }
    }

    async function openSearchResult(path, line) {
        const target = state;
        if (!isLiveInstance(target)) return;
        await runAsyncStep(target, () => openFile(path));
        if (!isLiveInstance(target)) return;
        runWithInstance(target, () => {
            const tab = activeTab();
            if (!tab || tab.path !== normalizeCodeStudioPath(path) || !tab.view) return;
            if (tab.view.state && tab.view.state.doc && state.cmModule && state.cmModule.EditorView) {
                const lineNumber = Math.min(Math.max(1, line || 1), tab.view.state.doc.lines);
                const docLine = tab.view.state.doc.line(lineNumber);
                tab.view.dispatch({
                    selection: { anchor: docLine.from },
                    effects: state.cmModule.EditorView.scrollIntoView(docLine.from, { y: 'center' })
                });
                tab.view.focus();
            } else if (tab.view.textarea) {
                const lines = tab.view.textarea.value.split('\n');
                const offset = lines.slice(0, Math.max(0, (line || 1) - 1)).join('\n').length;
                tab.view.textarea.focus();
                tab.view.textarea.setSelectionRange(offset, offset);
            }
            renderStatus();
        });
    }
