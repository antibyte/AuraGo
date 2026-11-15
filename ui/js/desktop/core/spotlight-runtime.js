    let spotlightOpen = false;
    let spotlightInstance = null;
    let spotlightSearchProvider = null;

    function closeSpotlight() {
        if (spotlightInstance) {
            spotlightInstance.generation++;
            spotlightInstance.controller?.abort();
            window.clearTimeout(spotlightInstance.timer);
            spotlightInstance = null;
        }
        const backdrop = document.getElementById('vd-spotlight-backdrop');
        if (backdrop) {
            desktopSound('menu.close');
            backdrop.remove();
        }
        spotlightOpen = false;
    }

    function spotlightAppEntries() {
        return startMenuApps()
            .map(app => ({
                id: 'app-' + app.id,
                type: 'app',
                title: appName(app),
                subtitle: t('desktop.spotlight_apps'),
                searchText: app.id,
                open: () => openApp(app.id)
            }));
    }

    function spotlightRecentFileEntries() {
        return readRecentFiles()
            .map(entry => ({
                id: 'recent-' + entry.path,
                type: 'file',
                title: entry.name,
                subtitle: t('desktop.spotlight_recent_files'),
                path: entry.path,
                searchText: entry.path,
                open: () => openDesktopPath(entry.path)
            }));
    }

    function spotlightSettingsContext() {
        const provider = window.AuraDesktopSettingsContext;
        if (typeof provider === 'function') return provider() || {};
        return provider && typeof provider === 'object' ? provider : {};
    }

    function spotlightLocalSources(includeRecents) {
        const settingsContext = spotlightSettingsContext();
        const settingsCatalog = window.AuraDesktopSettingsCatalog;
        const actionsProvider = window.AuraDesktopSearchActions;
        const actions = typeof actionsProvider === 'function' ? actionsProvider() : Array.isArray(actionsProvider) ? actionsProvider : [];
        return {
            apps: spotlightAppEntries(),
            settings: settingsCatalog && typeof settingsCatalog.list === 'function' ? settingsCatalog.list('', Object.assign({}, settingsContext, {
                openSetting: (category, settingKey) => openApp('settings', { category, settingKey })
            })) : [],
            actions,
            files: includeRecents ? spotlightRecentFileEntries() : [],
            t: key => t(key),
            openCalculator: expression => openApp('calculator', { expression })
        };
    }

    function spotlightFilters(backdrop) {
        const value = selector => backdrop.querySelector(selector)?.value || '';
        const localDateBound = (date, endOfDay) => {
            if (!date) return '';
            const parsed = new Date(date + (endOfDay ? 'T23:59:59.999' : 'T00:00:00'));
            return Number.isNaN(parsed.getTime()) ? '' : parsed.toISOString();
        };
        const minValue = value('[data-spotlight-min-size]');
        const maxValue = value('[data-spotlight-max-size]');
        const modifiedAfter = localDateBound(value('[data-spotlight-modified-after]'), false);
        const modifiedBefore = localDateBound(value('[data-spotlight-modified-before]'), true);
        const minSize = minValue === '' ? null : Number(minValue);
        const maxSize = maxValue === '' ? null : Number(maxValue);
        let error = '';
        if (minSize != null && (!Number.isSafeInteger(minSize) || minSize < 0) || maxSize != null && (!Number.isSafeInteger(maxSize) || maxSize < 0)) {
            error = 'desktop.spotlight_filter_invalid_size';
        } else if (minSize != null && maxSize != null && minSize > maxSize) {
            error = 'desktop.spotlight_filter_invalid_size';
        } else if (modifiedAfter && modifiedBefore && modifiedAfter > modifiedBefore) {
            error = 'desktop.spotlight_filter_invalid_dates';
        }
        return {
            type: value('[data-spotlight-filter-type]') || 'all',
            modifiedAfter,
            modifiedBefore,
            minSize,
            maxSize,
            error
        };
    }

    function spotlightCanShowRecents(filters) {
        return filters.type === 'all' && !filters.modifiedAfter && !filters.modifiedBefore && filters.minSize == null && filters.maxSize == null;
    }

    function spotlightHasFileFilters(filters) {
        return filters.type !== 'all' || !!(filters.modifiedAfter || filters.modifiedBefore) ||
            filters.minSize != null || filters.maxSize != null;
    }

    function spotlightCurrentEntries(stateObj) {
        const recentPaths = new Set((stateObj.recentEntries || []).map(entry => entry.path).filter(Boolean));
        const files = (stateObj.fileEntries || []).filter(entry => !recentPaths.has(entry.path));
        return (stateObj.localEntries || []).concat(stateObj.recentEntries || [], files);
    }

    function spotlightProvider() {
        if (spotlightSearchProvider) return spotlightSearchProvider;
        const search = window.AuraDesktopSearch;
        if (!search || typeof search.createProvider !== 'function') return null;
        spotlightSearchProvider = search.createProvider({ api, openFile: path => openDesktopPath(path) });
        return spotlightSearchProvider;
    }

    async function openDesktopPath(path) {
        const normalized = normalizeDesktopPath(path);
        if (!normalized) return;
        const entry = await enrichDesktopFileEntry({ path: normalized, name: pathBaseName(normalized) });
        if (entry.type === 'directory') {
            recordRecentFile(normalized, 'files', 'directory');
            openApp('files', { path: normalized, pathKind: 'directory' });
            return;
        }
        const ext = normalized.split('.').pop().toLowerCase();
        const associationExtension = /\.fb2\.zip$/i.test(normalized) ? 'fb2.zip' : /\.txt\.gz$/i.test(normalized) ? 'txt.gz' : ext;
        const defaultApp = defaultAppForExtension(associationExtension) || (associationExtension === 'fb2.zip' ? defaultAppForExtension('fb2') : '');
        if (defaultApp) {
            openApp(defaultApp, { path: normalized });
            return;
        }
        if (/\.(epub|mobi|azw|azw3|fb2(?:\.zip)?|cbz|cbr|lit|pdb|lrf)$/i.test(normalized)) {
            openApp('bookshelf', { path: normalized });
            return;
        }
        if (desktopIsArchiveFile(normalized)) {
            openApp('zipper', { path: normalized });
            return;
        }
        openApp('files', { path: pathDir(normalized) || '' });
        openApp('viewer', { path: normalized });
    }

    function renderSpotlightResults(stateObj) {
        const host = stateObj.backdrop.querySelector('[data-spotlight-results]');
        const status = stateObj.backdrop.querySelector('[data-spotlight-status]');
        const loadMore = stateObj.backdrop.querySelector('[data-spotlight-load-more]');
        if (!host || !status || !loadMore) return;
        const entries = stateObj.entries = spotlightCurrentEntries(stateObj);
        host.setAttribute('aria-busy', String(Boolean(stateObj.loading)));
        if (!entries.length) {
            host.innerHTML = `<div class="vd-spotlight-empty">${esc(stateObj.loading ? t('desktop.spotlight_searching') : t('desktop.spotlight_empty'))}</div>`;
            stateObj.backdrop.querySelector('.vd-spotlight-input')?.removeAttribute('aria-activedescendant');
        } else {
            host.innerHTML = entries.map((entry, index) => `<button type="button" class="vd-spotlight-item${index === stateObj.activeIndex ? ' active' : ''}" data-spotlight-index="${index}">
                <span class="vd-spotlight-item-title">${esc(entry.title)}</span>
                ${entry.subtitle ? `<span class="vd-spotlight-item-sub">${esc(entry.subtitle)}</span>` : ''}
            </button>`).join('');
            const input = stateObj.backdrop.querySelector('.vd-spotlight-input');
            if (input) input.setAttribute('aria-activedescendant', 'vd-spotlight-result-' + stateObj.activeIndex);
            host.querySelectorAll('[data-spotlight-index]').forEach((button, index) => {
                button.id = 'vd-spotlight-result-' + index;
                button.setAttribute('role', 'option');
                button.setAttribute('aria-selected', String(index === stateObj.activeIndex));
            });
        }
        let message = '';
        if (stateObj.filterError) message = t(stateObj.filterError);
        else if (stateObj.fileError) message = t('desktop.spotlight_search_failed');
        else if (stateObj.truncated) message = t('desktop.spotlight_results_truncated');
        else if (stateObj.loading) message = t('desktop.spotlight_searching');
        status.textContent = message;
        status.hidden = !message;
        status.setAttribute('role', stateObj.filterError || stateObj.fileError ? 'alert' : 'status');
        loadMore.hidden = !stateObj.hasMore;
        loadMore.textContent = t(stateObj.fileError ? 'desktop.spotlight_retry' : 'desktop.spotlight_load_more');
        loadMore.dataset.spotlightPageAction = stateObj.fileError ? 'retry' : 'more';
        loadMore.disabled = stateObj.loading || !stateObj.hasMore || (!stateObj.fileError && stateObj.nextOffset <= stateObj.lastOffset);
    }

    function spotlightIsCurrent(stateObj, generation) {
        return spotlightInstance === stateObj && generation === stateObj.generation && stateObj.backdrop.isConnected;
    }

    async function loadSpotlightFilePage(stateObj, generation, offset) {
        const provider = spotlightProvider();
        if (!provider || !spotlightIsCurrent(stateObj, generation) || stateObj.loading) return;
        stateObj.loading = true;
        stateObj.fileError = false;
        stateObj.lastOffset = offset;
        renderSpotlightResults(stateObj);
        try {
            const page = await provider.files(stateObj.query, {
                signal: stateObj.controller.signal,
                filters: stateObj.filters,
                limit: 50,
                offset
            });
            if (!spotlightIsCurrent(stateObj, generation)) return;
            const recentPaths = new Set((stateObj.recentEntries || []).map(entry => entry.path).filter(Boolean));
            const knownPaths = new Set((stateObj.fileEntries || []).map(entry => entry.path).filter(Boolean));
            (page.items || []).forEach(entry => {
                if (!entry.path || recentPaths.has(entry.path) || knownPaths.has(entry.path)) return;
                knownPaths.add(entry.path);
                stateObj.fileEntries.push(entry);
            });
            stateObj.nextOffset = page.nextOffset;
            stateObj.hasMore = Boolean(page.hasMore) && page.nextOffset > offset;
            stateObj.truncated = Boolean(stateObj.localTruncated || page.truncated);
            stateObj.fileError = false;
        } catch (error) {
            if (!spotlightIsCurrent(stateObj, generation) || stateObj.controller.signal.aborted) return;
            stateObj.fileError = true;
            stateObj.hasMore = true;
        } finally {
            if (spotlightIsCurrent(stateObj, generation)) {
                stateObj.loading = false;
                renderSpotlightResults(stateObj);
            }
        }
    }

    function refreshSpotlightResults(input, stateObj, delay) {
        const generation = ++stateObj.generation;
        stateObj.controller?.abort();
        stateObj.controller = new AbortController();
        window.clearTimeout(stateObj.timer);
        stateObj.query = input.value || '';
        stateObj.filters = spotlightFilters(stateObj.backdrop);
        stateObj.filterError = stateObj.filters.error;
        stateObj.fileError = false;
        stateObj.loading = false;
        stateObj.fileEntries = [];
        stateObj.nextOffset = 0;
        stateObj.lastOffset = -1;
        stateObj.hasMore = false;
        stateObj.truncated = false;
        stateObj.activeIndex = 0;

        const queryLength = Array.from(stateObj.query.trim()).length;
        if (queryLength > 256) stateObj.filterError = 'desktop.spotlight_query_too_long';
        const provider = spotlightProvider();
        const includeRecents = spotlightCanShowRecents(stateObj.filters);
        const localSources = stateObj.query.trim()
            ? spotlightLocalSources(includeRecents)
            : { files: includeRecents ? spotlightRecentFileEntries() : [], t: key => t(key) };
        const local = provider ? provider.local(stateObj.query, localSources, { limit: 101 }) : [];
        const visibleLocal = local.slice(0, 100);
        stateObj.localEntries = visibleLocal.filter(entry => entry.type !== 'file');
        stateObj.recentEntries = visibleLocal.filter(entry => entry.type === 'file');
        stateObj.localTruncated = local.length > 100;
        stateObj.truncated = stateObj.localTruncated;
        renderSpotlightResults(stateObj);

        if (stateObj.filterError || queryLength < 2 && !spotlightHasFileFilters(stateObj.filters) || !provider) return;
        stateObj.timer = window.setTimeout(() => loadSpotlightFilePage(stateObj, generation, 0), Math.max(0, delay || 0));
    }

    function activateSpotlightIndex(stateObj) {
        const entry = stateObj.entries[stateObj.activeIndex];
        if (!entry || typeof entry.open !== 'function' && typeof entry.action !== 'function') return;
        closeSpotlight();
        (entry.open || entry.action)();
    }

    function openSpotlight() {
        if (spotlightOpen) return;
        closeStartMenu();
        spotlightOpen = true;
        desktopSound('menu.open');
        const backdrop = document.createElement('div');
        backdrop.id = 'vd-spotlight-backdrop';
        backdrop.className = 'vd-spotlight-backdrop';
        backdrop.innerHTML = `<section class="vd-spotlight" role="dialog" aria-modal="true" aria-labelledby="vd-spotlight-title">
            <label id="vd-spotlight-title" for="vd-spotlight-query" class="vd-spotlight-label">${esc(t('desktop.spotlight_title'))}</label>
            <input id="vd-spotlight-query" type="search" class="vd-spotlight-input" placeholder="${esc(t('desktop.spotlight_placeholder'))}" autocomplete="off" spellcheck="false" role="combobox" aria-autocomplete="list" aria-expanded="true" aria-controls="vd-spotlight-results" aria-activedescendant="">
            <fieldset class="vd-spotlight-filters" aria-label="${esc(t('desktop.spotlight_filters'))}">
                <label class="vd-spotlight-filter vd-spotlight-filter-type"><span>${esc(t('desktop.spotlight_filter_type'))}</span>
                    <select data-spotlight-filter-type>
                        <option value="all">${esc(t('desktop.spotlight_filter_all'))}</option>
                        <option value="image">${esc(t('desktop.spotlight_filter_images'))}</option>
                        <option value="audio">${esc(t('desktop.spotlight_filter_audio'))}</option>
                        <option value="video">${esc(t('desktop.spotlight_filter_video'))}</option>
                        <option value="document">${esc(t('desktop.spotlight_filter_documents'))}</option>
                        <option value="archive">${esc(t('desktop.spotlight_filter_archives'))}</option>
                        <option value="folder">${esc(t('desktop.spotlight_filter_folders'))}</option>
                    </select>
                </label>
                <label class="vd-spotlight-filter"><span>${esc(t('desktop.spotlight_filter_modified_after'))}</span><input type="date" data-spotlight-modified-after></label>
                <label class="vd-spotlight-filter"><span>${esc(t('desktop.spotlight_filter_modified_before'))}</span><input type="date" data-spotlight-modified-before></label>
                <label class="vd-spotlight-filter"><span>${esc(t('desktop.spotlight_filter_min_size'))}</span><input type="number" min="0" step="1" inputmode="numeric" data-spotlight-min-size></label>
                <label class="vd-spotlight-filter"><span>${esc(t('desktop.spotlight_filter_max_size'))}</span><input type="number" min="0" step="1" inputmode="numeric" data-spotlight-max-size></label>
                <button type="button" class="vd-spotlight-filter-clear" data-spotlight-clear-filters>${esc(t('desktop.spotlight_clear_filters'))}</button>
            </fieldset>
            <div id="vd-spotlight-results" class="vd-spotlight-results" data-spotlight-results role="listbox" aria-label="${esc(t('desktop.spotlight_results_label'))}"></div>
            <div class="vd-spotlight-status" data-spotlight-status role="status" aria-live="polite" hidden></div>
            <button type="button" class="vd-spotlight-load-more" data-spotlight-load-more data-spotlight-page-action="more" hidden>${esc(t('desktop.spotlight_load_more'))}</button>
        </section>`;
        document.body.appendChild(backdrop);
        const input = backdrop.querySelector('.vd-spotlight-input');
        const stateObj = {
            entries: [], localEntries: [], recentEntries: [], fileEntries: [], activeIndex: 0,
            generation: 0, controller: null, timer: 0, backdrop, query: '', filters: { type: 'all' },
            loading: false, fileError: false, filterError: '', hasMore: false, truncated: false,
            nextOffset: 0, lastOffset: -1, localTruncated: false
        };
        spotlightInstance = stateObj;
        const scheduleRefresh = () => refreshSpotlightResults(input, stateObj, 120);
        input.addEventListener('input', scheduleRefresh);
        backdrop.querySelectorAll('[data-spotlight-filter-type], [data-spotlight-modified-after], [data-spotlight-modified-before], [data-spotlight-min-size], [data-spotlight-max-size]')
            .forEach(control => control.addEventListener('change', () => refreshSpotlightResults(input, stateObj, 0)));
        backdrop.querySelector('[data-spotlight-clear-filters]').addEventListener('click', () => {
            backdrop.querySelector('[data-spotlight-filter-type]').value = 'all';
            backdrop.querySelectorAll('[data-spotlight-modified-after], [data-spotlight-modified-before], [data-spotlight-min-size], [data-spotlight-max-size]')
                .forEach(control => { control.value = ''; });
            refreshSpotlightResults(input, stateObj, 0);
        });
        input.addEventListener('keydown', event => {
            if (event.key === 'Escape') {
                event.preventDefault();
                closeSpotlight();
                return;
            }
            if (event.key === 'ArrowDown') {
                event.preventDefault();
                stateObj.activeIndex = stateObj.entries.length ? Math.min(stateObj.entries.length - 1, stateObj.activeIndex + 1) : 0;
                renderSpotlightResults(stateObj);
                return;
            }
            if (event.key === 'ArrowUp') {
                event.preventDefault();
                stateObj.activeIndex = Math.max(0, stateObj.activeIndex - 1);
                renderSpotlightResults(stateObj);
                return;
            }
            if (event.key === 'Enter') {
                event.preventDefault();
                activateSpotlightIndex(stateObj);
            }
        });
        backdrop.addEventListener('click', event => {
            if (event.target === backdrop) closeSpotlight();
            const pageAction = event.target.closest('[data-spotlight-page-action]');
            if (pageAction) {
                const offset = stateObj.fileError ? stateObj.lastOffset : stateObj.nextOffset;
                loadSpotlightFilePage(stateObj, stateObj.generation, offset);
                return;
            }
            const btn = event.target.closest('[data-spotlight-index]');
            if (!btn) return;
            stateObj.activeIndex = Number(btn.dataset.spotlightIndex);
            activateSpotlightIndex(stateObj);
        });
        refreshSpotlightResults(input, stateObj, 0);
        input.focus();
    }
