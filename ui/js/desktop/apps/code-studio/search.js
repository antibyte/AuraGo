    function searchResultLabel(path) {
        const value = String(path || '');
        return value.startsWith(WORKSPACE_ROOT + '/') ? value.slice(WORKSPACE_ROOT.length + 1) : value;
    }

    function renderSearchPanel() {
        const panel = shellPart('[data-search]');
        if (!panel) return;
        panel.hidden = !state.searchVisible;
        if (!state.searchVisible) return;
        const options = state.searchOptions || {};
        let results;
        if (!state.searchPerformed) {
            results = `<div class="cs-empty">${esc(tr('codeStudio.searchPrompt', 'Enter a search term and press Enter'))}</div>`;
        } else if (!state.searchResults.length) {
            results = `<div class="cs-empty">${esc(tr('codeStudio.noResults', 'No results found'))}</div>`;
        } else {
            results = `<div class="cs-search-summary">${esc(tr('codeStudio.resultsCount', '{{count}} results', { count: state.searchResults.length }))}</div>` +
                state.searchResults.map(result => `
                <button type="button" class="cs-search-result" data-search-path="${esc(result.path)}" data-search-line="${esc(result.line)}" title="${esc(result.path)}">
                    <span><strong>${esc(searchResultLabel(result.path))}</strong>:${esc(result.line)}</span>
                    <code>${esc(result.preview)}</code>
                </button>`).join('');
        }
        panel.innerHTML = `<form class="cs-search-form" data-search-form>
            <input name="q" value="${esc(state.searchQuery || '')}" placeholder="${esc(tr('codeStudio.searchFiles', 'Search in Files'))}" autocomplete="off" spellcheck="false" inputmode="search" enterkeyhint="search">
            <input name="include" value="${esc(options.include || '')}" placeholder="*.go" autocomplete="off" spellcheck="false">
            <input name="exclude" value="${esc(options.exclude || '')}" placeholder="vendor/" autocomplete="off" spellcheck="false">
            <label><input type="checkbox" name="case"${options.case ? ' checked' : ''}> Aa</label>
            <label><input type="checkbox" name="whole"${options.whole ? ' checked' : ''}> Ab</label>
            <label><input type="checkbox" name="regex"${options.regex ? ' checked' : ''}> .*</label>
            <button type="submit" class="cs-button primary">${buttonIcon('search', 'S')}<span>${esc(tr('codeStudio.search', 'Search'))}</span></button>
            <button type="button" class="cs-icon-button" data-search-close title="${esc(tr('desktop.close', 'Close'))}">${iconMarkup('x', 'X', 'cs-icon-button-icon', 14)}</button>
        </form><div class="cs-search-results">${results}</div>`;
        panel.querySelector('[data-search-form]').addEventListener('submit', bind(event => {
            event.preventDefault();
            runSearch(new FormData(event.currentTarget));
        }));
        panel.querySelector('[data-search-close]').addEventListener('click', bind(() => toggleSearch()));
        panel.querySelectorAll('[data-search-path]').forEach(btn => {
            btn.addEventListener('click', bind(() => openSearchResult(btn.dataset.searchPath, Number(btn.dataset.searchLine || 1))));
        });
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
            if (!isLiveInstance(target)) return;
            runWithInstance(target, () => {
                state.searchResults = result.results || [];
                state.searchPerformed = true;
                renderSearchPanel();
                flashStatus(tr('codeStudio.resultsCount', '{{count}} results', { count: state.searchResults.length }));
            });
        } catch (err) {
            if (isLiveInstance(target)) {
                runWithInstance(target, () => {
                    state.searchResults = [];
                    state.searchPerformed = true;
                    renderSearchPanel();
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
            if (!tab || !tab.view) return;
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
