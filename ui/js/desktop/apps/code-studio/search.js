    function renderSearchPanel() {
        const panel = shellPart('[data-search]');
        if (!panel) return;
        panel.hidden = !state.searchVisible;
        if (!state.searchVisible) return;
        const results = state.searchResults.length ? state.searchResults.map(result => `
            <button type="button" class="cs-search-result" data-search-path="${esc(result.path)}" data-search-line="${esc(result.line)}">
                <span>${esc(result.path)}:${esc(result.line)}</span>
                <code>${esc(result.preview)}</code>
            </button>`).join('') : `<div class="cs-empty">${esc(tr('codeStudio.noFiles', 'No files open'))}</div>`;
        panel.innerHTML = `<form class="cs-search-form" data-search-form>
            <input name="q" placeholder="${esc(tr('codeStudio.searchFiles', 'Search in Files'))}" autocomplete="off" spellcheck="false">
            <input name="include" placeholder="*.go" autocomplete="off" spellcheck="false">
            <input name="exclude" placeholder="vendor/" autocomplete="off" spellcheck="false">
            <label><input type="checkbox" name="case"> Aa</label>
            <label><input type="checkbox" name="whole"> Ab</label>
            <label><input type="checkbox" name="regex"> .*</label>
            <button type="submit" class="cs-button primary">${buttonIcon('search', 'S')}<span>${esc(tr('codeStudio.search', 'Search'))}</span></button>
        </form><div class="cs-search-results">${results}</div>`;
        panel.querySelector('[data-search-form]').addEventListener('submit', bind(event => {
            event.preventDefault();
            runSearch(new FormData(event.currentTarget));
        }));
        panel.querySelectorAll('[data-search-path]').forEach(btn => {
            btn.addEventListener('click', bind(() => openSearchResult(btn.dataset.searchPath, Number(btn.dataset.searchLine || 1))));
        });
        const input = panel.querySelector('input[name="q"]');
        if (input && !input.value) input.focus();
    }

    function toggleSearch() {
        state.searchVisible = !state.searchVisible;
        renderSearchPanel();
        renderActivityBar();
    }

    async function runSearch(formData) {
        const target = state;
        if (!isLiveInstance(target)) return;
        const query = String(formData.get('q') || '').trim();
        if (!query) return;
        renderStatus(tr('codeStudio.search', 'Search'));
        const currentPath = target.currentPath || WORKSPACE_ROOT;
        const result = await apiClient.search({
            q: query,
            path: currentPath,
            case: formData.get('case') ? 'true' : 'false',
            whole: formData.get('whole') ? 'true' : 'false',
            regex: formData.get('regex') ? 'true' : 'false',
            include: String(formData.get('include') || ''),
            exclude: String(formData.get('exclude') || '')
        });
        if (!isLiveInstance(target)) return;
        runWithInstance(target, () => {
            state.searchResults = result.results || [];
            renderSearchPanel();
            renderStatus(tr('codeStudio.search', 'Search') + ': ' + state.searchResults.length);
        });
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
                const docLine = tab.view.state.doc.line(Math.max(1, line || 1));
                tab.view.dispatch({
                    selection: { anchor: docLine.from },
                    effects: state.cmModule.EditorView.scrollIntoView(docLine.from, { y: 'center' })
                });
            } else if (tab.view.textarea) {
                const lines = tab.view.textarea.value.split('\n');
                const offset = lines.slice(0, Math.max(0, (line || 1) - 1)).join('\n').length;
                tab.view.textarea.focus();
                tab.view.textarea.setSelectionRange(offset, offset);
            }
        });
    }
