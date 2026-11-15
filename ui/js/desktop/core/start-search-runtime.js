    let startSearchRequest = null;

    window.AuraDesktopSearchActions = () => [
        { id: 'action-overview', type: 'action', title: t('desktop.spaces_overview_show'), open: () => openSpacesOverview() },
        { id: 'action-notifications', type: 'action', title: t('desktop.notifications_title'), open: () => toggleNotificationCenter() },
        ...(!desktopReadonly() ? [{ id: 'action-empty-trash', type: 'action', title: t('desktop.context_empty_trash'), open: () => emptyTrash() }] : [])
    ];

    function cancelStartSearch() {
        if (!startSearchRequest) return;
        clearTimeout(startSearchRequest.timer);
        startSearchRequest.controller.abort();
        startSearchRequest = null;
    }

    function renderStartSearch(query) {
        cancelStartSearch();
        const provider = window.AuraDesktopSearch.createProvider({ api, openFile: path => openDesktopPath(path) });
        const tooLong = Array.from(query).length > 256;
        const request = { query, provider, controller: new AbortController(), local: provider.local(query, spotlightLocalSources(true)),
            files: [], offset: 0, more: false, truncated: false, loading: query.length >= 2 && !tooLong, tooLong, error: false, timer: 0 };
        startSearchRequest = request;
        renderStartSearchResults(request);
        if (request.loading) request.timer = setTimeout(() => loadStartSearchPage(request), 120);
    }

    async function loadStartSearchPage(request) {
        if (startSearchRequest !== request) return;
        request.loading = true;
        request.error = false;
        renderStartSearchResults(request);
        try {
            const page = await request.provider.files(request.query, { signal: request.controller.signal, limit: 50, offset: request.offset });
            if (startSearchRequest !== request || request.controller.signal.aborted) return;
            request.files.push(...page.items);
            request.more = Boolean(page.hasMore) && page.nextOffset > request.offset;
            request.offset = page.nextOffset;
            request.truncated = page.truncated;
        } catch (error) {
            if (startSearchRequest !== request || request.controller.signal.aborted) return;
            request.error = true;
        }
        if (startSearchRequest !== request) return;
        request.loading = false;
        renderStartSearchResults(request);
    }

    function renderStartSearchResults(request) {
        if (startSearchRequest !== request) return;
        const host = $('vd-start-apps');
        if (!host) return;
        const focusedID = document.activeElement?.dataset.startResult;
        const seen = new Set();
        const entries = [...request.local, ...request.files].filter(entry => {
            const key = entry.path ? 'file:' + entry.path : entry.id;
            if (seen.has(key)) return false;
            seen.add(key);
            return true;
        });
        const head = $('vd-start-pane-head');
        if (head) head.innerHTML = `<span class="vd-start-pane-title">${esc(t('desktop.start_results'))}</span><span class="vd-start-pane-count">${entries.length}</span>`;
        const scroll = host.scrollTop;
        host.innerHTML = entries.map(entry => `<button type="button" class="vd-start-item" data-start-result="${esc(entry.id)}"><span class="vd-start-item-text"><span class="vd-start-item-title">${esc(entry.title)}</span><small>${esc(entry.subtitle || '')}</small></span></button>`).join('') +
            (!entries.length && !request.loading ? startEmptyMarkup(t('desktop.start_no_results')) : '') +
            (request.loading ? `<p role="status">${esc(t('desktop.spotlight_searching'))}</p>` : '') +
            (request.error ? `<p role="status">${esc(t('desktop.start_search_failed'))}</p>` : '') +
            (request.tooLong ? `<p role="status">${esc(t('desktop.spotlight_query_too_long'))}</p>` : '') +
            (request.truncated ? `<p role="status">${esc(t('desktop.spotlight_results_truncated'))}</p>` : '') +
            (request.more || request.error ? `<button type="button" class="vd-button" data-search-more ${request.loading ? 'disabled' : ''}>${esc(t('desktop.start_search_more'))}</button>` : '');
        host.scrollTop = scroll;
        host.querySelectorAll('[data-start-result]').forEach(button => {
            button.addEventListener('click', () => {
                const entry = entries.find(item => item.id === button.dataset.startResult);
                closeStartMenu();
                Promise.resolve(entry?.open?.()).catch(error => showDesktopNotification({ message: error.message }));
            });
        });
        host.querySelector('[data-search-more]')?.addEventListener('click', () => loadStartSearchPage(request));
        if (focusedID) Array.from(host.querySelectorAll('[data-start-result]')).find(button => button.dataset.startResult === focusedID)?.focus({ preventScroll: true });
    }
