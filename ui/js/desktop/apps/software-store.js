(function () {
    'use strict';

    const instances = new Map();
    const retiredStoreAppIDs = new Set(['emulatorjs']);
    const integrationApps = new Set(['adguard-home', 'uptime-kuma', 'n8n', 'node-red', 'olivetin', 'open-webui', 'bytestash', 'beszel', 'filebrowser-quantum', 'stirling-pdf', 'ziro-designer', 'velxio']);

    // Same order and icons as the start menu categories.
    const storeCategories = [['office', 'writer'], ['media', 'audio-player'], ['creative', 'pixel'], ['ai', 'agent-chat'], ['dev', 'code'], ['system', 'settings'], ['comms', 'phone'], ['games', 'chess']];
    const storeCategoryIDs = new Set(storeCategories.map(item => item[0]));

    function searchKey(value) {
        return String(value == null ? '' : value).normalize('NFD').replace(/\p{Diacritic}/gu, '').toLowerCase();
    }

    function activeStoreCatalogEntries(items) {
        return (Array.isArray(items) ? items : []).filter(entry => !retiredStoreAppIDs.has(String(entry && entry.id || '').toLowerCase()));
    }

    function activeInstalledStoreApps(items) {
        return (Array.isArray(items) ? items : []).filter(app => !retiredStoreAppIDs.has(String(app && app.app_id || '').toLowerCase()));
    }

    function render(host, windowId, deps) {
        deps = deps || {};
        const esc = deps.esc || (value => String(value == null ? '' : value));
        const t = deps.t || ((key, fallback) => fallback || key);
        const api = deps.api;
        const iconMarkup = deps.iconMarkup || ((key, fallback) => '<span>' + esc(fallback || key || '') + '</span>');
        const themeIconPath = typeof deps.themeIconPath === 'function' ? deps.themeIconPath : (() => '');
        const notify = deps.notify || function () {};
        const openApp = deps.openApp || function () {};
        const loadBootstrap = deps.loadBootstrap || function () {};
        if (!host || !api) return;
        dispose(windowId);

        let catalog = [];
        let installed = [];
        let busy = new Map();
        const integrationStates = new Map();
        const operationErrors = new Map();
        const pollingOperations = new Set();
        const instance = { disposed: false, onDesktopEvent: null, pollingOperations, loadDebounceTimer: null, overlays: new Set() };
        let dockerAvailable = true;
        let mutationsAllowed = true;
        let mutationDisabledReason = '';
        let loadToken = 0;
        let pendingCatalogRequest = null;
        let initialCatalogLoaded = false;
        instances.set(windowId, instance);
        let targetAppID = '';
        let focusTarget = false;
        // Browse state lives on the closure only (in memory).
        let selection = 'all';
        let searchText = '';
        const expandedDescriptions = new Set();
        let lastGridMarkup = '';
        let lastNavMarkup = '';
        let userLoadPending = false;
        const categoriesLabelID = 'vd-store-categories-' + windowId;
        instance.selectTarget = context => {
            const value = context && context.app_id;
            if (instance.disposed || typeof value !== 'string' || !/^[a-z0-9][a-z0-9_-]{0,127}$/.test(value)) return;
            targetAppID = value;
            focusTarget = true;
            if (initialCatalogLoaded) renderCards();
        };
        instance.selectTarget(deps);

        function trackOverlay(overlay) {
            if (instance.disposed) return;
            instance.overlays.add(overlay);
            document.body.appendChild(overlay);
        }

        function closeOverlay(overlay) {
            overlay.querySelectorAll('input, textarea').forEach(field => {
                field.value = '';
                field.defaultValue = '';
                field.removeAttribute('value');
            });
            overlay.textContent = '';
            overlay.remove();
            instance.overlays.delete(overlay);
        }

        // Shared dialog behaviour: Escape closes, the first control takes focus,
        // and focus returns to the opener (or its re-rendered twin) on close.
        function mountDialog(overlay, firstSelector) {
            const previousFocus = document.activeElement;
            const savedFocus = captureGridFocus();
            const close = () => {
                closeOverlay(overlay);
                if (instance.disposed) return;
                if (previousFocus && previousFocus.isConnected && previousFocus !== document.body) previousFocus.focus({ preventScroll: true });
                else restoreGridFocus(savedFocus);
            };
            overlay.addEventListener('keydown', event => {
                if (event.key === 'Escape') {
                    event.preventDefault();
                    event.stopPropagation();
                    close();
                }
            });
            overlay.addEventListener('click', event => { if (event.target === overlay) close(); });
            trackOverlay(overlay);
            const first = overlay.querySelector(firstSelector || 'input, textarea, select, button');
            if (first) first.focus();
            return close;
        }

        host.innerHTML = `
            <div class="vd-store">
                <nav class="vd-store-nav" aria-label="${esc(t('desktop.store.filter_label'))}"></nav>
                <section class="vd-store-main">
                    <div class="vd-store-toolbar">
                        <label class="vd-store-search">
                            ${iconMarkup('search', 'S', 'vd-store-search-icon', 15)}
                            <input type="search" class="vd-store-search-input" data-store-search placeholder="${esc(t('desktop.store.search_placeholder'))}" aria-label="${esc(t('desktop.store.search_placeholder'))}" autocomplete="off" spellcheck="false">
                            <button type="button" class="vd-store-search-clear" data-store-search-clear aria-label="${esc(t('desktop.store.search_clear'))}" title="${esc(t('desktop.store.search_clear'))}" hidden>${iconMarkup('x', 'x', 'vd-store-btn-icon', 14)}</button>
                        </label>
                        <div class="vd-store-count" aria-live="polite"></div>
                        <button type="button" class="vd-store-btn vd-store-icon-btn" data-action="refresh" aria-label="${esc(t('desktop.context_refresh'))}" title="${esc(t('desktop.context_refresh'))}">${iconMarkup('refresh', 'R', 'vd-store-btn-icon', 16)}</button>
                    </div>
                    <div class="vd-store-warning" hidden>${iconMarkup('info', '!', 'vd-store-warning-icon', 16)}<span class="vd-store-warning-text"></span></div>
                    <div class="vd-store-grid"></div>
                </section>
            </div>`;

        const grid = host.querySelector('.vd-store-grid');
        const warning = host.querySelector('.vd-store-warning');
        const warningText = host.querySelector('.vd-store-warning-text');
        const nav = host.querySelector('.vd-store-nav');
        const searchInput = host.querySelector('[data-store-search]');
        const searchClear = host.querySelector('[data-store-search-clear]');
        const countEl = host.querySelector('.vd-store-count');
        const refreshButton = host.querySelector('[data-action="refresh"]');
        refreshButton.addEventListener('click', () => userReload());

        nav.addEventListener('scroll', () => syncNavFade(), { passive: true });
        nav.addEventListener('click', event => {
            const item = event.target.closest('[data-store-filter], [data-store-category]');
            if (!item || !nav.contains(item)) return;
            focusTarget = false;
            selection = item.hasAttribute('data-store-filter') ? item.dataset.storeFilter : 'category:' + item.dataset.storeCategory;
            renderNav();
            const chip = nav.querySelector('.vd-store-nav-item.is-active');
            if (chip) chip.scrollIntoView({ inline: 'nearest', block: 'nearest' });
            renderCards({ resetScroll: true });
        });

        searchInput.addEventListener('input', () => {
            focusTarget = false; // user intent supersedes a pending deep link
            searchText = searchInput.value;
            searchClear.hidden = !searchText;
            renderCards({ resetScroll: true });
        });
        searchInput.addEventListener('keydown', event => {
            if (event.key === 'Escape' && searchInput.value) {
                event.preventDefault();
                event.stopPropagation();
                setSearch('');
            }
        });
        searchClear.addEventListener('click', () => {
            setSearch('');
            searchInput.focus();
        });

        grid.addEventListener('click', event => {
            const button = event.target.closest('button');
            if (!button || !grid.contains(button)) return;
            if (button.hasAttribute('data-store-more')) return toggleDescription(button);
            if (button.hasAttribute('data-store-retry')) return userReload();
            if (button.hasAttribute('data-store-reset')) return resetFilters();
            const card = button.closest('.vd-store-card');
            if (card && button.dataset.action) handleAction(card.dataset.appId, button.dataset.action, button.dataset.portId);
        });

        // "/" focuses the search unless the user is already typing somewhere.
        const onHostKeydown = event => {
            if (event.key !== '/' || event.ctrlKey || event.metaKey || event.altKey) return;
            const target = event.target;
            if (target && (/^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName) || target.isContentEditable)) return;
            event.preventDefault();
            searchInput.focus();
        };
        host.addEventListener('keydown', onHostKeydown);
        instance.host = host;
        instance.hostKeydown = onHostKeydown;
        if (typeof ResizeObserver === 'function') {
            instance.resizeObserver = new ResizeObserver(() => {
                if (instance.resizeFrame || instance.disposed) return;
                instance.resizeFrame = requestAnimationFrame(() => {
                    instance.resizeFrame = 0;
                    if (storeGridReady()) { syncDescriptionToggles(); syncNavFade(); }
                });
            });
            instance.resizeObserver.observe(grid);
        }

        function storeGridReady() {
            return !instance.disposed && grid && grid.isConnected;
        }

        function setSearch(value) {
            focusTarget = false;
            searchText = value;
            searchInput.value = value;
            searchClear.hidden = !value;
            renderCards({ resetScroll: true });
        }

        function resetFilters() {
            selection = 'all';
            setSearch('');
            renderNav();
            searchInput.focus();
        }

        function setRefreshBusy(on) {
            refreshButton.classList.toggle('is-busy', !!on);
            if (on) refreshButton.setAttribute('aria-busy', 'true');
            else refreshButton.removeAttribute('aria-busy');
        }

        function showSkeleton() {
            if (!storeGridReady()) return;
            lastGridMarkup = '';
            grid.setAttribute('aria-busy', 'true');
            const card = '<div class="vd-store-skeleton"><div class="vd-store-skeleton-head"><span class="vd-store-skeleton-logo"></span><span class="vd-store-skeleton-lines"><span></span><span></span></span></div><span class="vd-store-skeleton-text"></span><span class="vd-store-skeleton-btn"></span></div>';
            grid.innerHTML = `<div class="vd-store-loading"><span class="vd-store-sr">${esc(t('desktop.loading'))}</span><div class="vd-store-cards" aria-hidden="true">${card.repeat(8)}</div></div>`;
        }

        function showStoreError(err, fallbackKey, fallbackText) {
            if (!storeGridReady()) return;
            const title = t(fallbackKey || 'desktop.store.load_failed', fallbackText);
            const message = err && err.message ? err.message : '';
            lastGridMarkup = '';
            grid.removeAttribute('aria-busy');
            grid.innerHTML = `<div class="vd-store-empty vd-store-error" role="alert">
                ${iconMarkup('info', 'i', 'vd-store-empty-icon', 28)}
                <div class="vd-store-empty-title">${esc(title)}</div>
                ${message && message !== title ? `<div class="vd-store-empty-text">${esc(message)}</div>` : ''}
                <button type="button" class="vd-store-btn vd-store-primary" data-store-retry>${iconMarkup('refresh', 'R', 'vd-store-btn-icon', 15)}<span>${esc(t('desktop.retry'))}</span></button>
            </div>`;
        }

        function storeEntryUsesThemeIcon(entry) {
            const iconKey = String(entry && entry.icon || '').trim();
            if (!iconKey) return false;
            try {
                return !!themeIconPath(iconKey);
            } catch (_) {
                return false;
            }
        }

        function isDesktopStoreChangedEvent(event) {
            if (!event) return false;
            if (event.operation === 'desktop_store_changed') return true;
            return event.type === 'desktop_changed'
                && event.payload
                && event.payload.operation === 'desktop_store_changed';
        }

        function fetchCatalog(forceRefresh) {
            if (forceRefresh) {
                pendingCatalogRequest = null;
                return api('/api/desktop/store/catalog');
            }
            if (!pendingCatalogRequest) {
                pendingCatalogRequest = api('/api/desktop/store/catalog').finally(() => {
                    pendingCatalogRequest = null;
                });
            }
            return pendingCatalogRequest;
        }

        // Refresh and retry buttons mark their load as user-started: only those toast on failure.
        function userReload() {
            userLoadPending = true;
            scheduleLoad(true, true);
        }

        function scheduleLoad(immediate, forceRefresh) {
            if (instance.disposed) return;
            if (instance.loadDebounceTimer) {
                clearTimeout(instance.loadDebounceTimer);
                instance.loadDebounceTimer = null;
            }
            if (immediate) {
                void loadCatalog(!!forceRefresh);
                return;
            }
            instance.loadDebounceTimer = setTimeout(() => {
                instance.loadDebounceTimer = null;
                void loadCatalog(false);
            }, 120);
        }

        async function loadCatalog(forceRefresh) {
            const userInitiated = userLoadPending;
            userLoadPending = false;
            const token = ++loadToken;
            if (!storeGridReady()) return;
            setRefreshBusy(true);
            // The skeleton only shows before the first catalog; later loads keep the current cards.
            if (!initialCatalogLoaded) showSkeleton();
            try {
                const body = await fetchCatalog(forceRefresh);
                const allowSupersededInitialRender = token !== loadToken && !initialCatalogLoaded;
                // The first successful catalog response is good enough to replace the loading state.
                if ((token !== loadToken && !allowSupersededInitialRender) || !storeGridReady()) return;
                catalog = activeStoreCatalogEntries(body.catalog || []);
                installed = activeInstalledStoreApps(body.installed || []);
                dockerAvailable = body.docker_available !== false;
                mutationsAllowed = body.mutations_allowed !== false && dockerAvailable;
                mutationDisabledReason = body.mutation_disabled_reason || '';
                if (warning) {
                    warning.hidden = mutationsAllowed;
                    warningText.textContent = mutationsAllowed ? '' : mutationDisabledText();
                }
                renderCards();
                resumeActiveOperationPolling();
                initialCatalogLoaded = true;
                await Promise.all(installed.filter(app => integrationApps.has(app.app_id)).map(async app => {
                    try {
                        const connection = await api('/api/desktop/store/apps/' + app.app_id + '/integration');
                        if (token === loadToken && storeGridReady()) integrationStates.set(app.app_id, connection.state);
                    } catch (_) { if (token === loadToken) integrationStates.set(app.app_id, 'error'); }
                }));
                if (token === loadToken && storeGridReady()) renderCards();
            } catch (err) {
                if (token !== loadToken || !storeGridReady()) return;
                console.error('Software store catalog load failed', err);
                if (!initialCatalogLoaded) {
                    showStoreError(err, 'desktop.store.load_failed', 'Could not load the software store.');
                } else {
                    // Keep the cards that are already on screen; only a user-started load toasts.
                    if (!userInitiated) console.warn('Software store background reload failed', err);
                    else notify({ title: t('desktop.store.title'), message: (err && err.message) || t('desktop.store.load_failed') });
                }
            } finally {
                if (token === loadToken) setRefreshBusy(false);
            }
        }

        function installedFor(appId) {
            return installed.find(app => app.app_id === appId);
        }

        function activeOperationForApp(app) {
            if (!app || !app.last_operation_id) return null;
            if (app.last_operation_state === 'pending' || app.last_operation_state === 'running') {
                const operationType = app.last_operation_type || app.status || 'install';
                if (!storeAppStatusAllowsActiveOperation(app, operationType)) return null;
                return {
                    id: app.last_operation_id,
                    app_id: app.app_id,
                    type: operationType,
                    status: app.last_operation_state
                };
            }
            return null;
        }

        function storeAppStatusAllowsActiveOperation(app, operationType) {
            if (operationType === 'install') return app.status === 'installing';
            if (operationType === 'update') return app.status === 'updating';
            return true;
        }

        function resumeActiveOperationPolling() {
            installed.forEach(app => {
                const operation = activeOperationForApp(app);
                if (!operation || busy.has(app.app_id)) return;
                busy.set(app.app_id, operation);
                pollOperation(app.app_id, operation.id);
            });
            renderCards();
        }

        function storeOpenLabel(entry) {
            if (entry && entry.id === 'commandcode') {
                return t('desktop.store.open_commandcode');
            }
            return t('desktop.store.open');
        }

        function storeUiBadge(entry) {
            if (isNativeManagedEntry(entry)) {
                return `<span class="vd-store-badge">${esc(t('desktop.store.badge_native'))}</span>`;
            }
            if (entry && entry.metadata && entry.metadata.store_ui === 'terminal-preview') {
                return `<span class="vd-store-badge">${esc(t('desktop.store.badge_terminal_preview'))}</span>`;
            }
            return '';
        }

        function entryIsInstalled(entry) {
            if (installedFor(entry.id)) return true;
            const operation = busy.get(entry.id);
            return !!(operation && operation.type === 'install' && operation.status !== 'succeeded' && operation.status !== 'failed');
        }

        function entryNeedsUpdate(entry) {
            const app = installedFor(entry.id);
            return !!app && app.update_required === true;
        }

        function entryDescription(entry) {
            return entry.metadata && entry.metadata.description_key ? t(entry.metadata.description_key) : entry.description;
        }

        function categoryLabel(entry) {
            return entry && storeCategoryIDs.has(entry.category) ? t('desktop.category_' + entry.category) : '';
        }

        function storeCounts() {
            const counts = { all: catalog.length, installed: 0, updates: 0, categories: new Map() };
            catalog.forEach(entry => {
                if (entryIsInstalled(entry)) counts.installed++;
                if (entryNeedsUpdate(entry)) counts.updates++;
                if (storeCategoryIDs.has(entry.category)) counts.categories.set(entry.category, (counts.categories.get(entry.category) || 0) + 1);
            });
            return counts;
        }

        function entryMatchesSelection(entry) {
            if (selection === 'installed') return entryIsInstalled(entry);
            if (selection === 'updates') return entryNeedsUpdate(entry);
            if (selection.startsWith('category:')) return entry.category === selection.slice('category:'.length);
            return true;
        }

        function entryMatchesSearch(entry, terms) {
            if (!terms.length) return true;
            const haystack = searchKey([entry.name, entryDescription(entry), categoryLabel(entry), entry.id, entry.image].join(' '));
            return terms.every(term => haystack.includes(term));
        }

        function searchTerms() {
            return searchKey(searchText).split(/\s+/).filter(Boolean);
        }

        // Falls back to "all" when the selected view no longer exists.
        function normalizeSelection(counts) {
            if (selection === 'updates' && !counts.updates) selection = 'all';
            else if (selection.startsWith('category:') && !counts.categories.has(selection.slice('category:'.length))) selection = 'all';
            else if (selection !== 'all' && selection !== 'installed' && selection !== 'updates' && !selection.startsWith('category:')) selection = 'all';
        }

        function syncNavFade() {
            if (!nav) return;
            nav.classList.toggle('can-scroll-right', nav.scrollWidth > nav.clientWidth + 1 && nav.scrollLeft + nav.clientWidth < nav.scrollWidth - 1);
        }

        function renderNav() {
            if (!nav || instance.disposed) return;
            const counts = storeCounts();
            normalizeSelection(counts);
            const item = (attr, value, iconKey, glyph, label, count) => {
                const usage = attr === 'data-store-filter' ? 'action' : undefined;
                const active = (attr === 'data-store-filter' ? value : 'category:' + value) === selection;
                return `<button type="button" class="vd-store-nav-item${active ? ' is-active' : ''}" ${attr}="${esc(value)}" aria-pressed="${active ? 'true' : 'false'}">${iconMarkup(iconKey, glyph, 'vd-store-nav-icon', 16, usage)}<span class="vd-store-nav-text">${esc(label)}</span><span class="vd-store-nav-count">${count}</span></button>`;
            };
            const heading = `<div class="vd-store-heading"><div class="vd-store-title">${esc(t('desktop.store.title'))}</div><div class="vd-store-subtitle">${esc(t('desktop.store.subtitle'))}</div></div>`;
            let body = '';
            if (catalog.length) {
                body = `<div class="vd-store-nav-group" role="group" aria-label="${esc(t('desktop.store.filter_label'))}">
                    ${item('data-store-filter', 'all', 'package', 'A', t('desktop.store.filter_all'), counts.all)}
                    ${item('data-store-filter', 'installed', 'check', 'I', t('desktop.store.filter_installed'), counts.installed)}
                    ${counts.updates > 0 ? item('data-store-filter', 'updates', 'download', 'U', t('desktop.store.filter_updates'), counts.updates) : ''}
                </div>
                <div class="vd-store-nav-label" id="${esc(categoriesLabelID)}">${esc(t('desktop.store.categories_label'))}</div>
                <div class="vd-store-nav-group" role="group" aria-labelledby="${esc(categoriesLabelID)}">
                    ${storeCategories.filter(([id]) => counts.categories.has(id)).map(([id, icon]) => item('data-store-category', id, icon, id.charAt(0).toUpperCase(), t('desktop.category_' + id), counts.categories.get(id))).join('')}
                </div>`;
            }
            const markup = heading + body;
            if (markup === lastNavMarkup) return;
            lastNavMarkup = markup;
            const active = document.activeElement;
            let refocus = '';
            if (active && nav.contains(active)) {
                if (active.hasAttribute('data-store-filter')) refocus = '[data-store-filter="' + active.dataset.storeFilter + '"]';
                else if (active.hasAttribute('data-store-category')) refocus = '[data-store-category="' + active.dataset.storeCategory + '"]';
            }
            nav.innerHTML = markup;
            const activeChip = nav.querySelector('.vd-store-nav-item.is-active');
            if (activeChip && !refocus) activeChip.scrollIntoView({ inline: 'nearest', block: 'nearest' });
            syncNavFade();
            if (refocus) {
                const target = nav.querySelector(refocus);
                if (target) target.focus({ preventScroll: true });
            }
        }

        function browseSections() {
            const terms = searchTerms();
            const visible = catalog.filter(entry => entryMatchesSelection(entry) && entryMatchesSearch(entry, terms));
            if (selection === 'installed') return [{ id: 'installed', title: t('desktop.store.filter_installed'), entries: visible }];
            if (selection === 'updates') return [{ id: 'updates', title: t('desktop.store.filter_updates'), entries: visible }];
            return [
                { id: 'installed', title: t('desktop.store.filter_installed'), entries: visible.filter(entryIsInstalled) },
                { id: 'available', title: t('desktop.store.status_available'), entries: visible.filter(entry => !entryIsInstalled(entry)) }
            ].filter(section => section.entries.length);
        }

        // The deep-link target must be visible, otherwise the focus logic cannot find its card.
        function revealTarget() {
            const entry = catalog.find(item => item.id === targetAppID);
            if (!entry) return;
            if (entryMatchesSelection(entry) && entryMatchesSearch(entry, searchTerms())) return;
            selection = 'all';
            searchText = '';
            searchInput.value = '';
            searchClear.hidden = true;
        }

        function captureGridFocus() {
            const active = document.activeElement;
            if (!active || active === grid || !grid.contains(active)) return null;
            const card = active.closest('.vd-store-card');
            return {
                appId: card ? card.dataset.appId : '',
                isCard: active === card,
                action: active.dataset.action || '',
                portId: active.dataset.portId || '',
                more: active.hasAttribute('data-store-more'),
                reset: active.hasAttribute('data-store-reset'),
                retry: active.hasAttribute('data-store-retry')
            };
        }

        function restoreGridFocus(saved) {
            if (!saved) return;
            let target = null;
            if (saved.reset) target = grid.querySelector('[data-store-reset]');
            else if (saved.retry) target = grid.querySelector('[data-store-retry]');
            else {
                const card = [...grid.querySelectorAll('.vd-store-card')].find(item => item.dataset.appId === saved.appId);
                if (card && saved.more) target = card.querySelector('[data-store-more]');
                else if (card && saved.action) target = [...card.querySelectorAll('[data-action]')].find(button => button.dataset.action === saved.action && (button.dataset.portId || '') === saved.portId);
                else if (card && saved.isCard) target = card;
            }
            if (target && target !== document.activeElement && !target.disabled) target.focus({ preventScroll: true });
        }

        function toggleDescription(button) {
            const card = button.closest('.vd-store-card');
            if (!card) return;
            const id = card.dataset.appId;
            if (expandedDescriptions.has(id)) expandedDescriptions.delete(id);
            else expandedDescriptions.add(id);
            syncDescriptionToggles();
        }

        // Adds the "More"/"Less" text button to cards whose clamped description overflows.
        function syncDescriptionToggles() {
            if (!storeGridReady()) return;
            grid.querySelectorAll('.vd-store-card').forEach(card => {
                const desc = card.querySelector('.vd-store-card-desc');
                if (!desc) return;
                const expanded = expandedDescriptions.has(card.dataset.appId);
                desc.classList.toggle('is-expanded', expanded);
                let button = card.querySelector('[data-store-more]');
                const needed = expanded || desc.scrollHeight > desc.clientHeight + 1;
                if (!needed) {
                    if (button) button.remove();
                    return;
                }
                if (!button) {
                    button = document.createElement('button');
                    button.type = 'button';
                    button.className = 'vd-store-more';
                    button.setAttribute('data-store-more', '');
                    desc.after(button);
                }
                button.textContent = t(expanded ? 'desktop.store.show_less' : 'desktop.store.show_more');
                button.setAttribute('aria-expanded', expanded ? 'true' : 'false');
            });
        }

        function logoMarkup(entry) {
            const themedStoreIcon = storeEntryUsesThemeIcon(entry);
            const logo = !themedStoreIcon && entry.logo_url ? `<img class="vd-store-logo" src="${esc(entry.logo_url)}" alt="" loading="lazy" onerror="this.hidden=true;this.nextElementSibling.hidden=false">` : '';
            const fallback = `<div class="vd-store-logo-fallback"${!themedStoreIcon && entry.logo_url ? ' hidden' : ''}>${iconMarkup(entry.icon || 'package', entry.name || 'A', 'vd-store-logo-icon', 30)}</div>`;
            return `<div class="vd-store-logo-wrap">${logo}${fallback}</div>`;
        }

        function cardMarkup(entry) {
            const app = installedFor(entry.id);
            const operation = busy.get(entry.id) || activeOperationForApp(app);
            const status = operation && operation.type === 'install' && operation.status !== 'succeeded' && operation.status !== 'failed'
                ? 'installing'
                : app ? (app.status || 'installed') : 'available';
            const running = app && app.status === 'running';
            const stopped = app && app.status === 'stopped';
            const mutationDisabled = mutationsAllowed ? '' : mutationDisabledText();
            const actionDisabled = operation ? statusLabel(status, operation) : mutationDisabled;
            const disabledAttrs = actionDisabled ? `disabled title="${esc(actionDisabled)}"` : '';
            const access = app ? accessLabel(app) : '';
            const warningText = hostAccessWarning(entry);
            const operationError = operationErrors.get(entry.id) || (app && app.error);
            const installedCard = !!app;
            const categoryText = categoryLabel(entry);
            const imageText = String(entry.image || '');
            const subText = [categoryText, imageText].filter(Boolean).join(' · ');
            const iconButton = (action, iconKey, glyph, label, guarded, extraClass) => `<button type="button" class="vd-store-btn vd-store-icon-btn${extraClass ? ' ' + extraClass : ''}" data-action="${action}" aria-label="${esc(label)}" ${guarded && disabledAttrs ? disabledAttrs : `title="${esc(label)}"`}>${iconMarkup(iconKey, glyph, 'vd-store-btn-icon', 16)}</button>`;
            const igState = integrationStates.get(entry.id) || 'setup_required';
            const igLabel = t('desktop.store.ig_' + igState);
            const igTone = igState === 'connected' ? 'is-connected' : igState === 'error' ? 'is-error' : 'is-setup';
            const needsUpdate = !!app && app.update_required === true;
            const updateButton = needsUpdate ? '' : iconButton('update', 'download', 'U', t('desktop.store.update'), true);
            const showStatus = !!(app || operation);
            return `<article class="vd-store-card ${installedCard ? 'is-installed' : 'is-available'} status-${esc(status)}" data-app-id="${esc(entry.id)}" tabindex="-1" aria-current="${entry.id === targetAppID ? 'true' : 'false'}">
                <div class="vd-store-card-head">
                    ${logoMarkup(entry)}
                    <div class="vd-store-card-title-wrap">
                        <div class="vd-store-card-title">${esc(entry.name)}${storeUiBadge(entry)}</div>
                        <div class="vd-store-card-sub" title="${esc(imageText)}">${esc(subText)}</div>
                    </div>
                    ${showStatus ? `<span class="vd-store-status status-${esc(status)}"><i class="vd-store-status-dot" aria-hidden="true"></i>${esc(statusLabel(status, operation))}</span>` : ''}
                </div>
                <div class="vd-store-card-desc${expandedDescriptions.has(entry.id) ? ' is-expanded' : ''}">${esc(entryDescription(entry))}</div>
                ${warningText ? `<div class="vd-store-card-warning">${esc(warningText)}</div>` : ''}
                ${needsUpdate ? `<div class="vd-store-card-warning vd-store-update-warning" role="status"><span>${esc(t('desktop.store.update_required'))}</span><button type="button" class="vd-store-btn vd-store-update" data-action="update" ${disabledAttrs}>${iconMarkup('download', 'U', 'vd-store-btn-icon', 15)}<span>${esc(t('desktop.store.update'))}</span></button></div>` : ''}
                ${operationError ? `<div class="vd-store-card-warning" role="alert">${esc(operationError)}</div>` : ''}
                ${operation ? `<div class="vd-store-progress"><div class="vd-store-progress-bar" role="progressbar" aria-label="${esc(statusLabel(status, operation))}"></div><span>${esc(statusLabel(operation.status, operation))}</span></div>` : ''}
                ${app ? `<div class="vd-store-meta">
                    <span class="vd-store-chip">${esc(access)}</span>
                    ${integrationApps.has(entry.id) ? `<button type="button" class="vd-store-chip vd-store-chip-btn ${igTone}" data-action="integration" title="${esc(t('desktop.store.ig_connect'))} · ${esc(igLabel)}" aria-label="${esc(t('desktop.store.ig_connect'))} · ${esc(igLabel)}">${iconMarkup('link', 'L', 'vd-store-btn-icon', 13)}<span>${esc(igLabel)}</span></button>` : ''}
                </div>` : ''}
                <div class="vd-store-actions">
                    ${app ? `<button type="button" class="vd-store-btn vd-store-primary" data-action="open">${iconMarkup('browser', 'O', 'vd-store-btn-icon', 15)}<span>${esc(storeOpenLabel(entry))}</span></button>` : ''}
                    ${extraPortButtons(entry, app, actionDisabled)}
                    ${app && stopped ? iconButton('start', 'run', 'S', t('desktop.store.start'), true) : ''}
                    ${app && running ? iconButton('stop', 'stop', 'S', t('desktop.store.stop'), true) : ''}
                    ${app ? `${updateButton}
                        ${entry.id === 'gods-eye-view' ? iconButton('configure-gev', 'settings', 'S', t('desktop.store.gev_configure'), true) : ''}
                        ${entry.id === 'printguard' ? iconButton('configure-pg', 'settings', 'S', t('desktop.store.pg_setup'), true) : ''}
                        ${hasExposedCredentials(entry) ? iconButton('credentials', 'key', 'K', t('desktop.store.credentials'), false) : ''}
                        ${entry.id === 'beszel' ? iconButton('configure-agent', 'settings', 'A', t('desktop.store.configure_agent'), true) : ''}
                        ${iconButton('uninstall', 'trash', 'X', t('desktop.store.uninstall'), true, 'vd-store-danger vd-store-uninstall')}` : `<button type="button" class="vd-store-btn vd-store-install" data-action="install" ${disabledAttrs}>${iconMarkup('download', 'I', 'vd-store-btn-icon', 15)}<span>${esc(t('desktop.store.install'))}</span></button>`}
                </div>
            </article>`;
        }

        function sectionMarkup(section) {
            return `<section class="vd-store-section" data-store-section="${esc(section.id)}">
                <h3 class="vd-store-section-title">${esc(section.title)}<span class="vd-store-section-count">${section.entries.length}</span></h3>
                <div class="vd-store-cards">${section.entries.map(cardMarkup).join('')}</div>
            </section>`;
        }

        function emptyMarkup() {
            const query = searchText.trim();
            return `<div class="vd-store-empty">
                ${iconMarkup('search', '?', 'vd-store-empty-icon', 28)}
                <div class="vd-store-empty-title">${esc(query ? t('desktop.store.no_results', { query }) : t('desktop.store.no_results_filtered'))}</div>
                <button type="button" class="vd-store-btn" data-store-reset>${esc(t('desktop.store.reset_filters'))}</button>
            </div>`;
        }

        function renderCards(options) {
            if (!storeGridReady()) return;
            try {
                if (!catalog.length) {
                    renderNav();
                    lastGridMarkup = '';
                    grid.removeAttribute('aria-busy');
                    countEl.textContent = '';
                    grid.innerHTML = `<div class="vd-store-empty">${iconMarkup('package', 'A', 'vd-store-empty-icon', 28)}<div class="vd-store-empty-title">${esc(t('desktop.store.empty'))}</div></div>`;
                    return;
                }
                if (focusTarget && targetAppID) revealTarget();
                renderNav();
                const sections = browseSections();
                const total = sections.reduce((sum, section) => sum + section.entries.length, 0);
                countEl.textContent = total === 1 ? t('desktop.store.result_count_one') : t('desktop.store.result_count', { count: total });
                const savedScroll = grid.scrollTop;
                const savedFocus = captureGridFocus();
                const markup = total ? sections.map(sectionMarkup).join('') : emptyMarkup();
                if (markup !== lastGridMarkup || grid.querySelector('.vd-store-loading, .vd-store-error')) {
                    grid.removeAttribute('aria-busy');
                    grid.innerHTML = markup;
                    lastGridMarkup = markup;
                }
                syncDescriptionToggles();
                grid.scrollTop = options && options.resetScroll ? 0 : savedScroll;
                restoreGridFocus(savedFocus);

                grid.querySelectorAll('.vd-store-card').forEach(card => {
                    const appId = card.dataset.appId;
                    if (focusTarget && appId === targetAppID) {
                        requestAnimationFrame(() => {
                            if (instance.disposed || !card.isConnected || appId !== targetAppID) return;
                            card.scrollIntoView({ block: 'nearest' });
                            card.focus({ preventScroll: true });
                            focusTarget = false;
                        });
                    }
                });
            } catch (err) {
                console.error('Software store render failed', err);
                showStoreError(err, 'desktop.store.render_failed', 'Could not render the software store.');
            }
        }

        function hostAccessWarning(entry) {
            const hasHostBinds = entry && Array.isArray(entry.host_binds) && entry.host_binds.length > 0;
            const hasCompanionHostAccess = entry && Array.isArray(entry.companions) && entry.companions.some(companion => companion.network_mode === 'host' || (Array.isArray(companion.host_binds) && companion.host_binds.length > 0));
            if (!hasHostBinds && !hasCompanionHostAccess) return '';
            return t('desktop.store.host_access_warning');
        }

        function extraPortButtons(entry, app, actionDisabled) {
            if (!entry || !app || app.status !== 'running' || !Array.isArray(entry.extra_ports) || !entry.extra_ports.length) return '';
            const ports = entry.extra_ports.filter(port => !String(port.id || '').startsWith('agent-'));
            if (!ports.length) return '';
            return ports.map(port => {
                const label = port.name || port.id || port.container_port;
                return `<button type="button" class="vd-store-btn" data-action="open-port" data-port-id="${esc(port.id || '')}" ${actionDisabled ? `disabled title="${esc(actionDisabled)}"` : ''}>${iconMarkup('browser', 'O', 'vd-store-btn-icon', 15)}<span>${esc(label)}</span></button>`;
            }).join('');
        }

        function hasExposedCredentials(entry) {
            return !!(entry && Array.isArray(entry.generated_secrets) && entry.generated_secrets.some(secret => secret && secret.expose));
        }

        function statusLabel(status, operation) {
            if (operation && operation.type === 'install') {
                if (operation.status === 'pending' || operation.status === 'running') {
                    return t('desktop.store.status_installing');
                }
            }
            if (operation && operation.status && operation.status !== 'succeeded') {
                return t('desktop.store.operation_' + operation.status, operation.status);
            }
            return t('desktop.store.status_' + status, status);
        }

        function accessLabel(app) {
            const parts = [];
            parts.push(app.bind_mode === 'lan' ? t('desktop.store.access_lan') : t('desktop.store.access_local'));
            if (app.tailscale_enabled) parts.push(app.tailscale_status === 'active' ? t('desktop.store.access_tailnet') : t('desktop.store.access_tailnet_pending'));
            return parts.join(' / ');
        }

        function handleAction(appId, action, portId) {
            if (isMutatingAction(action) && busy.has(appId)) return;
            const entry = catalog.find(item => item.id === appId);
            if (isMutatingAction(action) && !mutationsAllowed) {
                notify({ title: t('desktop.store.title'), message: mutationDisabledText() });
                return;
            }
            if (action === 'install') {
                return openInstallModal(appId);
            }
            if (action === 'open' && isNativeManagedEntry(entry)) return openApp(nativeDesktopAppID(entry));
            if (action === 'open' && shouldOpenStoreEntryExternally(entry)) return openStorePort(appId, '');
            if (action === 'open') return openApp('store-' + appId);
            if (action === 'open-port') return openStorePort(appId, portId);
            if (action === 'credentials') return openCredentialsModal(appId);
            if (action === 'configure-agent') return openBeszelAgentModal(appId);
            if (action === 'configure-gev') return openGodsEyeModal();
            if (action === 'configure-pg') return openPrintGuardModal(false);
            if (action === 'integration') return openIntegrationModal(appId);
            if (action === 'uninstall') return openUninstallModal(appId);
            return startOperation(appId, action, '/api/desktop/store/apps/' + encodeURIComponent(appId) + '/' + encodeURIComponent(action), 'POST');
        }

        function isMutatingAction(action) {
            if (action === 'configure-gev') return true;
            if (action === 'configure-pg') return true;
            return action === 'install' || action === 'start' || action === 'stop' || action === 'restart' || action === 'update' || action === 'uninstall';
        }

        function shouldOpenStoreEntryExternally(entry) {
            return !!(entry && entry.metadata && entry.metadata.open_external === 'true');
        }

        function isNativeManagedEntry(entry) {
            return !!(entry && entry.runtime === 'native-managed-app');
        }

        function nativeDesktopAppID(entry) {
            return (entry && (entry.desktop_app_id || (entry.metadata && entry.metadata.desktop_app_id) || entry.id)) || '';
        }

        function mutationDisabledText() {
            switch (mutationDisabledReason) {
                case 'desktop_readonly':
                    return t('desktop.store.desktop_readonly');
                case 'docker_disabled':
                    return t('desktop.store.docker_disabled');
                case 'docker_readonly':
                    return t('desktop.store.docker_readonly');
                case 'docker_unavailable':
                    return t('desktop.store.docker_unavailable');
                default:
                    if (!dockerAvailable) {
                        return t('desktop.store.docker_unavailable');
                    }
                    return t('desktop.store.mutations_disabled');
            }
        }

        function openInstallModal(appId) {
            if (appId === 'printguard') return openPrintGuardModal(true);
            const entry = catalog.find(item => item.id === appId);
            if (!entry) return;
            const overlay = document.createElement('div');
            overlay.className = 'vd-modal-backdrop';
            overlay.innerHTML = `<form class="vd-modal vd-store-modal" role="dialog" aria-modal="true">
                <div class="vd-store-modal-head">
                    ${logoMarkup(entry)}
                    <div class="vd-store-modal-head-text">
                        <div class="vd-modal-title">${esc(t('desktop.store.install_title'))}: ${esc(entry.name)}</div>
                        ${categoryLabel(entry) ? `<div class="vd-store-modal-sub">${esc(categoryLabel(entry))}</div>` : ''}
                    </div>
                </div>
                ${appId === 'gods-eye-view' ? `<p class="vd-store-modal-copy">${esc(t('desktop.store.gev_lan_warning'))}</p>` : ''}
                <div class="vd-store-modal-copy">${esc(t('desktop.store.install_copy'))}</div>
                <div class="vd-store-choice-group">
                    <label class="vd-store-choice vd-store-choice-card"><input type="radio" name="bind" value="local" checked><span><b>${esc(t('desktop.store.bind_local'))}</b><small>127.0.0.1</small></span></label>
                    <label class="vd-store-choice vd-store-choice-card"><input type="radio" name="bind" value="lan"><span><b>${esc(t('desktop.store.bind_lan'))}</b><small>0.0.0.0</small></span></label>
                </div>
                <label class="vd-store-choice"><input type="checkbox" name="tailscale"><span><b>${esc(t('desktop.store.tailscale'))}</b><small>${esc(t('desktop.store.tailscale_hint'))}</small></span></label>
                <div class="vd-modal-actions">
                    <button type="button" class="vd-button" data-action="cancel">${esc(t('desktop.cancel'))}</button>
                    <button type="submit" class="vd-button vd-button-primary">${esc(t('desktop.store.install'))}</button>
                </div>
            </form>`;
            const form = overlay.querySelector('form');
            const close = mountDialog(overlay, 'input[name="bind"]:checked');
            overlay.querySelector('[data-action="cancel"]').addEventListener('click', close);
            form.addEventListener('submit', event => {
                event.preventDefault();
                const bind = form.querySelector('input[name="bind"]:checked').value;
                const tailscale = form.querySelector('input[name="tailscale"]').checked;
                close();
                startOperation(appId, 'install', '/api/desktop/store/install', 'POST', {
                    app_id: appId,
                    bind_mode: bind,
                    ...(appId === 'gods-eye-view' ? { allowed_origins: [window.location.origin] } : {}),
                    tailscale_enabled: tailscale
                });
            });
        }

        function openUninstallModal(appId) {
            const app = installedFor(appId);
            if (!app) return;
            const overlay = document.createElement('div');
            overlay.className = 'vd-modal-backdrop';
            overlay.innerHTML = `<form class="vd-modal vd-store-modal" role="dialog" aria-modal="true">
                <div class="vd-modal-title">${esc(t('desktop.store.uninstall'))}</div>
                <div class="vd-store-modal-copy">${esc(t('desktop.store.uninstall_copy'))}</div>
                <label class="vd-store-choice"><input type="checkbox" name="delete-data"><span><b>${esc(t('desktop.store.delete_data'))}</b><small>${esc(t('desktop.store.delete_data_hint'))}</small></span></label>
                <div class="vd-modal-actions">
                    <button type="button" class="vd-button" data-action="cancel">${esc(t('desktop.cancel'))}</button>
                    <button type="submit" class="vd-button vd-button-danger">${esc(t('desktop.store.uninstall'))}</button>
                </div>
            </form>`;
            const form = overlay.querySelector('form');
            const close = mountDialog(overlay);
            overlay.querySelector('[data-action="cancel"]').addEventListener('click', close);
            form.addEventListener('submit', event => {
                event.preventDefault();
                const deleteData = form.querySelector('input[name="delete-data"]').checked;
                close();
                startOperation(appId, 'uninstall', '/api/desktop/store/apps/' + encodeURIComponent(appId) + '?delete_data=' + encodeURIComponent(deleteData), 'DELETE');
            });
        }

        function safeExternalURL(raw) {
            const value = String(raw || '').trim();
            if (!value) return '';
            try {
                const url = new URL(value, window.location.origin);
                if (url.protocol === 'http:' || url.protocol === 'https:') return url.href;
            } catch (_) {}
            return '';
        }

        async function openStorePort(appId, portId) {
            if (instance.disposed) return;
            const pendingWindow = window.open('about:blank', '_blank');
            if (pendingWindow) {
                pendingWindow.opener = null;
            }
            try {
                const body = await api('/api/desktop/store/apps/' + encodeURIComponent(appId) + '/open-url?port_id=' + encodeURIComponent(portId || ''));
                if (instance.disposed) { pendingWindow?.close(); return; }
                const safeURL = safeExternalURL(body.url);
                if (safeURL) {
                    if (pendingWindow && !pendingWindow.closed) {
                        pendingWindow.location.replace(safeURL);
                    } else {
                        window.open(safeURL, '_blank', 'noopener,noreferrer');
                    }
                } else if (pendingWindow && !pendingWindow.closed) {
                    pendingWindow.close();
                }
            } catch (err) {
                if (pendingWindow && !pendingWindow.closed) {
                    pendingWindow.close();
                }
                if (!instance.disposed) notify({ title: t('desktop.store.title'), message: err.message });
            }
        }

        async function openCredentialsModal(appId) {
            try {
                const body = await api('/api/desktop/store/apps/' + encodeURIComponent(appId) + '/credentials');
                if (instance.disposed) return;
                const credentials = body.credentials || [];
                const overlay = document.createElement('div');
                overlay.className = 'vd-modal-backdrop';
                overlay.innerHTML = `<div class="vd-modal vd-store-modal" role="dialog" aria-modal="true">
                    <div class="vd-modal-title">${esc(t('desktop.store.credentials'))}</div>
                    <div class="vd-store-credentials">${credentials.map(credential => `<label class="vd-store-credential"><span>${esc(credential.label || credential.key)}</span><input type="text" readonly value="${esc(credential.value || '')}"></label>`).join('') || `<div class="vd-store-modal-copy">${esc(t('desktop.store.no_credentials'))}</div>`}</div>
                    <div class="vd-modal-actions">
                        <button type="button" class="vd-button vd-button-primary" data-action="close">${esc(t('desktop.close'))}</button>
                    </div>
                </div>`;
                const close = mountDialog(overlay);
                overlay.querySelector('[data-action="close"]').addEventListener('click', close);
            } catch (err) {
                if (!instance.disposed) notify({ title: t('desktop.store.title'), message: err.message });
            }
        }

        function openBeszelAgentModal(appId) {
            const overlay = document.createElement('div');
            overlay.className = 'vd-modal-backdrop';
            overlay.innerHTML = `<form class="vd-modal vd-store-modal" role="dialog" aria-modal="true">
                <div class="vd-modal-title">${esc(t('desktop.store.configure_agent'))}</div>
                <div class="vd-store-modal-copy">${esc(t('desktop.store.beszel_agent_copy'))}</div>
                <label class="vd-store-field"><span>${esc(t('desktop.store.beszel_key'))}</span><textarea name="key" required></textarea></label>
                <label class="vd-store-field"><span>${esc(t('desktop.store.beszel_token'))}</span><input type="password" name="token" required></label>
                <div class="vd-modal-actions">
                    <button type="button" class="vd-button" data-action="cancel">${esc(t('desktop.cancel'))}</button>
                    <button type="submit" class="vd-button vd-button-primary">${esc(t('desktop.save'))}</button>
                </div>
            </form>`;
            const form = overlay.querySelector('form');
            const close = mountDialog(overlay);
            overlay.querySelector('[data-action="cancel"]').addEventListener('click', close);
            form.addEventListener('submit', async event => {
                event.preventDefault();
                try {
                    await api('/api/desktop/store/apps/' + encodeURIComponent(appId) + '/companions/agent/config', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({
                            key: form.querySelector('[name="key"]').value,
                            token: form.querySelector('[name="token"]').value
                        })
                    });
                    if (instance.disposed) return;
                    close();
                    scheduleLoad(true, true);
                    notify({ title: t('desktop.store.title'), message: t('desktop.store.agent_configured') });
                } catch (err) {
                    if (!instance.disposed) notify({ title: t('desktop.store.title'), message: err.message });
                }
            });
        }

        async function openIntegrationModal(appId) {
            const loadId = instance.integrationLoadId = (instance.integrationLoadId || 0) + 1;
            const endpoint = '/api/desktop/store/apps/' + appId + '/integration';
            const previousFocus = document.activeElement;
            const label = key => esc(t('desktop.store.ig_' + key));
            try {
                let saved = await api(endpoint);
                if (instance.disposed || instance.integrationLoadId !== loadId) return;
                if (instance.closeConfig) instance.closeConfig();
                const overlay = document.createElement('div');
                overlay.className = 'vd-modal-backdrop';
                const grants = ['read'];
                if (['adguard-home', 'n8n', 'node-red', 'olivetin', 'ziro-designer', 'velxio'].includes(appId)) grants.push('execute');
                if (appId === 'bytestash') grants.push('create');
                if (appId === 'stirling-pdf') grants.push('process', 'sign');
                if (appId === 'uptime-kuma') grants.push('notify');
                const field = (name, type = 'text') => `<label class="vd-store-field"><span>${label(name)}</span><input name="${name}" type="${type}" maxlength="${type === 'password' ? 4096 : 2048}" autocomplete="${type === 'password' ? 'new-password' : 'off'}"></label>`;
                overlay.innerHTML = `<form class="vd-modal vd-store-modal" role="dialog" aria-modal="true" aria-label="${label('connect')}">
                    <div class="vd-modal-title">${esc(catalog.find(item => item.id === appId)?.name || appId)} · ${label('connect')}</div>
                    <p class="vd-store-modal-copy">${label('intro')}</p>
                    ${['n8n', 'node-red', 'olivetin', 'bytestash', 'beszel', 'filebrowser-quantum', 'stirling-pdf', 'ziro-designer', 'velxio'].includes(appId) ? `<p class="vd-store-modal-copy">${label('help_' + appId)}</p>` : ''}
                    <button type="button" class="vd-button" data-ig-open>${esc(t('desktop.store.open'))}</button>
                    ${saved.containerized && appId !== 'ziro-designer' ? `<label class="vd-store-field"><span>${esc(t('desktop.store.pg_network'))}</span><select name="docker_network"><option value="">—</option>${(saved.networks || []).map(name => `<option value="${esc(name)}">${esc(name)}</option>`).join('')}</select></label>` : ''}
                    ${['ziro-designer', 'velxio'].includes(appId) ? '' : field('url')}${['adguard-home', 'beszel', 'filebrowser-quantum'].includes(appId) ? field('username') : ''}
                    ${['ziro-designer', 'velxio'].includes(appId) ? '' : field('credential', 'password')}<p data-ig-stored></p>
                    ${appId === 'open-webui' ? `<p class="vd-store-modal-copy">${label('relay_hint')}</p>${field('relay_port', 'number')}` : ''}
                    ${['ziro-designer', 'velxio'].includes(appId) ? '' : `<label class="vd-store-choice"><input type="checkbox" name="clear_credential"><span>${label('clear_credential')}</span></label>`}
                    ${saved.conflict ? `<label class="vd-store-choice"><input type="checkbox" name="select_store_instance"><span>${label('conflict')}</span></label>` : ''}
                    ${grants.map(name => `<label class="vd-store-choice"><input type="checkbox" name="${name}"><span>${label(appId === 'open-webui' && name === 'read' ? 'relay_enable' : appId === 'velxio' ? 'velxio_' + name : name)}</span></label>`).join('')}
                    ${['node-red', 'olivetin'].includes(appId) ? field('allowed_ids') : ''}
                    ${appId === 'filebrowser-quantum' ? field('source') : ''}
                    ${appId === 'stirling-pdf' ? field('certificate_fingerprint') : ''}
                    ${appId === 'adguard-home' ? `<p class="vd-store-modal-copy">${label('adguard_hint')}</p><label class="vd-store-field"><span>${label('admin_port')}</span><select name="admin_port"><option value="main">3000</option><option value="admin">80</option></select></label>${field('dns_address')}<p class="vd-store-modal-copy">${label('dns_hint')}</p>` : ''}
                    <p role="status" aria-live="polite" data-ig-status></p>
                    <div class="vd-modal-actions"><button type="button" class="vd-button" data-ig-close>${esc(t('desktop.close'))}</button>
                    <button type="button" class="vd-button" data-ig-disconnect>${label('disconnect')}</button>
                    <button type="button" class="vd-button" data-ig-test>${label('test')}</button>
                    <button type="submit" class="vd-button vd-button-primary" ${mutationsAllowed ? '' : 'disabled'}>${esc(t('desktop.save'))}</button></div>
                </form>`;
                const form = overlay.querySelector('form'), controller = new AbortController();
                let submitting = false, dirty = false;
                const alive = () => !instance.disposed && overlay.isConnected;
                const status = form.querySelector('[data-ig-status]');
                const test = form.querySelector('[data-ig-test]');
                const apply = connection => {
                    saved = connection;
                    for (const name of ['url', 'username', 'source', 'certificate_fingerprint', 'admin_port', 'dns_address', 'docker_network', 'relay_port']) if (form.elements[name]) form.elements[name].value = connection[name] || (name === 'admin_port' ? 'main' : '');
                    for (const name of grants) form.elements[name].checked = !!connection[name];
                    if (form.elements.allowed_ids) form.elements.allowed_ids.value = (connection.allowed_ids || []).join(', ');
                    form.querySelector('[data-ig-stored]').textContent = connection.credential_stored ? t('desktop.store.ig_stored') : '';
                    status.textContent = connection.message || t('desktop.store.ig_' + connection.state);
                    integrationStates.set(appId, connection.state);
                    dirty = false;
                    test.disabled = !mutationsAllowed || !connection.revision;
                };
                const close = () => { controller.abort(); closeOverlay(overlay); if (instance.closeConfig === close) instance.closeConfig = null; if (previousFocus?.isConnected) previousFocus.focus(); };
                instance.closeConfig = close;
                form.querySelector('[data-ig-close]').onclick = close;
                form.querySelector('[data-ig-open]').onclick = () => openStorePort(appId, saved.admin_port || '');
                form.addEventListener('input', () => { dirty = true; test.disabled = true; status.textContent = t('desktop.store.ig_save_first'); });
                form.addEventListener('change', () => { dirty = true; test.disabled = true; });
                const submit = async method => {
                    if (submitting || !mutationsAllowed || (method === 'POST' && dirty)) return;
                    submitting = true;
                    const payload = {revision: saved.revision, url: form.elements.url?.value.trim() || '', credential: form.elements.credential?.value || '', clear_credential: !!form.elements.clear_credential?.checked, select_store_instance: !!form.elements.select_store_instance?.checked};
                    for (const name of grants) payload[name] = form.elements[name].checked;
                    for (const name of ['username', 'source', 'certificate_fingerprint', 'admin_port', 'dns_address', 'docker_network']) if (form.elements[name]) payload[name] = form.elements[name].value.trim();
                    if (form.elements.relay_port) payload.relay_port = Number(form.elements.relay_port.value);
                    if (form.elements.allowed_ids) payload.allowed_ids = form.elements.allowed_ids.value.split(',').map(value => value.trim()).filter(Boolean);
                    const controls = [...form.querySelectorAll('input, select, button')];
                    controls.forEach(control => { control.disabled = true; });
                    try {
                        const response = await api(endpoint + (method === 'POST' ? '/test' : ''), {method, body: method === 'PUT' ? JSON.stringify(payload) : undefined, signal: controller.signal});
                        if (alive()) { apply(response); renderCards(); }
                    } catch (error) { if (alive()) status.textContent = error.message; }
                    finally { delete payload.credential; if (form.elements.credential) form.elements.credential.value = ''; submitting = false; if (alive()) { controls.forEach(control => { control.disabled = false; }); test.disabled = dirty || !saved.revision; } }
                };
                form.addEventListener('submit', event => { event.preventDefault(); void submit('PUT'); });
                test.onclick = () => void submit('POST');
                form.querySelector('[data-ig-disconnect]').onclick = () => void submit('DELETE');
                overlay.addEventListener('click', event => { if (event.target === overlay) close(); });
                overlay.addEventListener('keydown', event => {
                    if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); close(); }
                    if (event.key === 'Tab') { const controls = [...form.querySelectorAll('input, select, button')].filter(el => !el.disabled); const first = controls[0], last = controls[controls.length - 1]; if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); } else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); } }
                });
                apply(saved); trackOverlay(overlay); form.querySelector('button').focus();
            } catch (error) { if (!instance.disposed && instance.integrationLoadId === loadId) notify({title: t('desktop.store.ig_connect'), message: error.message}); }
        }

        function printGuardMessage(error) {
            const message = error && error.message || String(error || '');
            if (/^printguard_[a-z_]+$/.test(message)) return t('desktop.store.pg_error_' + message.slice(11));
            return message || t('desktop.store.operation_failed');
        }

        async function openPrintGuardModal(installing) {
            const appId = 'printguard';
            const url = '/api/desktop/store/apps/printguard/config';
            const loadId = instance.printGuardLoadId = (instance.printGuardLoadId || 0) + 1;
            try {
                const config = await api(url);
                if (instance.disposed || instance.printGuardLoadId !== loadId) return;
                if (instance.closeConfig) instance.closeConfig();
                const overlay = document.createElement('div');
                overlay.className = 'vd-modal-backdrop';
                const networks = Array.isArray(config.networks) ? config.networks : [];
                const currentNetwork = config.docker_network || (networks.length === 1 ? networks[0] : '');
                const prerequisites = Array.isArray(config.prerequisites) ? config.prerequisites : [];
                const statusId = 'vd-pg-status-' + windowId;
                overlay.innerHTML = `<form class="vd-modal vd-store-modal vd-pg-config" role="dialog" aria-modal="true" aria-label="${esc(t('desktop.store.pg_setup'))}">
                    <div class="vd-modal-title">PrintGuard · ${esc(t('desktop.store.pg_setup'))}</div>
                    <p class="vd-store-modal-copy">${esc(t('desktop.store.pg_intro'))}</p>
                    <p class="vd-store-card-warning">${esc(t('desktop.store.pg_interrupt'))}</p>
                    <label class="vd-store-field"><span>${esc(t('desktop.store.pg_acceleration'))}</span><select name="acceleration" ${installing ? 'disabled' : ''}>
                        <option value="cpu">CPU</option><option value="intel" ${config.architecture !== 'amd64' ? 'disabled' : ''}>Intel GPU</option><option value="nvidia" ${config.architecture !== 'amd64' || !config.nvidia_available ? 'disabled' : ''}>NVIDIA GPU</option>
                    </select></label>
                    ${config.containerized ? `<label class="vd-store-field"><span>${esc(t('desktop.store.pg_network'))}</span><select name="docker_network"><option value="">—</option>${networks.map(name => `<option value="${esc(name)}">${esc(name)}</option>`).join('')}</select></label>` : ''}
                    ${!installing ? `<p class="vd-store-modal-copy">${esc(t('desktop.store.pg_compute'))}: <span data-pg-compute>${esc(config.compute || t('desktop.store.pg_unknown'))}</span> · ${esc(config.active_acceleration || 'cpu')}</p>
                    <label class="vd-store-field"><span>${esc(t('desktop.store.pg_agent'))}</span><select name="agent_mode"><option value="off">${esc(t('desktop.store.pg_off'))}</option><option value="read">${esc(t('desktop.store.pg_read'))}</option><option value="control">${esc(t('desktop.store.pg_control'))}</option></select></label>
                    <p class="vd-store-modal-copy">${esc(t('desktop.store.pg_token_hint'))}</p>
                    <label class="vd-store-field"><span>${esc(t('desktop.store.pg_token'))}${config.token_configured ? ' · ' + esc(t('desktop.store.pg_token_set')) : ''}</span><input type="password" name="api_token" autocomplete="new-password" maxlength="4096"></label>
                    <label class="vd-store-choice"><input type="checkbox" name="clear_api_token"><span>${esc(t('desktop.store.pg_remove_token'))}</span></label>` : ''}
                    ${config.cpu_fallback ? `<p class="vd-store-card-warning">${esc(t('desktop.store.pg_cpu_fallback'))}</p>` : ''}
                    <p id="${esc(statusId)}" role="status" aria-live="polite" class="vd-store-card-warning" data-pg-status>${esc(prerequisites.map(code => printGuardMessage(code)).join(' '))}${config.pending ? ' ' + esc(t('desktop.store.pg_pending')) : ''}</p>
                    ${installing && prerequisites.some(code => code === 'printguard_gateway' || code === 'printguard_auth') ? `<p class="vd-store-modal-copy"><a href="/config#server" target="_blank" rel="noopener noreferrer">${esc(t('desktop.open_config'))}</a></p>` : ''}
                    <div class="vd-modal-actions"><button type="button" class="vd-button" data-action="cancel">${esc(t('desktop.cancel'))}</button>
                    ${installing && prerequisites.length ? `<button type="button" class="vd-button" data-action="recheck">${esc(t('desktop.retry'))}</button>` : ''}
                    ${!installing ? `<button type="button" class="vd-button" data-action="test" ${!config.token_configured || config.agent_mode === 'off' || config.pending ? 'disabled' : ''}>${esc(t('desktop.store.pg_test'))}</button>` : ''}
                    <button type="submit" class="vd-button vd-button-primary" aria-describedby="${esc(statusId)}">${esc(t(installing ? 'desktop.store.install' : 'desktop.save'))}</button></div>
                </form>`;
                const form = overlay.querySelector('form');
                form.elements.acceleration.value = installing ? 'cpu' : config.acceleration;
                if (form.elements.docker_network) form.elements.docker_network.value = currentNetwork;
                if (form.elements.agent_mode) form.elements.agent_mode.value = config.agent_mode || 'off';
                const previousFocus = document.activeElement;
                const controller = new AbortController();
                let submitting = false, draftChanged = false, installBlocked = false;
                const alive = () => !instance.disposed && overlay.isConnected;
                const close = () => {
                    controller.abort();
                    closeOverlay(overlay);
                    if (instance.closeConfig === close) instance.closeConfig = null;
                    if (previousFocus && previousFocus.isConnected) previousFocus.focus();
                };
                instance.closeConfig = close;
                const test = form.querySelector('[data-action="test"]');
                const updateInstallState = () => {
                    if (!installing) return;
                    const networkReady = !config.containerized || networks.includes(form.elements.docker_network.value);
                    const issues = prerequisites.filter(code => code !== 'printguard_network' || !networkReady);
                    if (!networkReady && !issues.includes('printguard_network')) issues.push('printguard_network');
                    installBlocked = issues.length > 0;
                    form.querySelector('[type="submit"]').disabled = submitting || installBlocked;
                    form.querySelector('[data-pg-status]').textContent = issues.map(code => printGuardMessage(code)).join(' ');
                };
                updateInstallState();
                const markDraft = () => {
                    draftChanged = true;
                    if (test) test.disabled = true;
                    if (installing) updateInstallState();
                    else form.querySelector('[data-pg-status]').textContent = '';
                };
                form.addEventListener('input', markDraft);
                form.addEventListener('change', markDraft);
                form.querySelector('[data-action="cancel"]').addEventListener('click', close);
                form.querySelector('[data-action="recheck"]')?.addEventListener('click', () => {
                    close();
                    openPrintGuardModal(true);
                });
                overlay.addEventListener('click', event => { if (event.target === overlay) close(); });
                overlay.addEventListener('keydown', event => {
                    if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); close(); }
                    if (event.key === 'Tab') {
                        const controls = [...form.querySelectorAll('input, select, a[href], button')].filter(el => !el.disabled);
                        const first = controls[0], last = controls[controls.length - 1];
                        if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
                        else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
                    }
                });
                if (test) test.addEventListener('click', async () => {
                    if (submitting) return;
                    submitting = true;
                    test.disabled = true;
                    try {
                        await api('/api/desktop/store/apps/printguard/test', {method: 'POST', signal: controller.signal});
                        if (alive() && !draftChanged) form.querySelector('[data-pg-status]').textContent = t('desktop.store.pg_test_ok');
                    } catch (error) { if (alive() && !draftChanged) form.querySelector('[data-pg-status]').textContent = printGuardMessage(error); }
                    finally { submitting = false; if (alive()) test.disabled = draftChanged; }
                });
                form.addEventListener('submit', async event => {
                    event.preventDefault();
                    if (submitting || installBlocked) return;
                    submitting = true;
                    const payload = installing ? {app_id: appId, bind_mode: 'local', tailscale_enabled: false, docker_network: form.elements.docker_network ? form.elements.docker_network.value : ''} : {
                        acceleration: form.elements.acceleration.value,
                        docker_network: form.elements.docker_network ? form.elements.docker_network.value : '',
                        agent_mode: form.elements.agent_mode.value,
                        api_token: form.elements.api_token.value.trim(),
                        clear_api_token: form.elements.clear_api_token.checked
                    };
                    form.querySelector('[type="submit"]').disabled = true;
                    try {
                        const body = await api(installing ? '/api/desktop/store/install' : url, {method: installing ? 'POST' : 'PUT', headers: {'Content-Type': 'application/json'}, body: JSON.stringify(payload), signal: controller.signal});
                        if (!alive()) return;
                        close();
                        if (body.operation) { busy.set(appId, body.operation); renderCards(); pollOperation(appId, body.operation.id); }
                    } catch (error) { if (alive()) form.querySelector('[data-pg-status]').textContent = printGuardMessage(error); }
                    finally {
                        delete payload.api_token;
                        if (form.elements.api_token) form.elements.api_token.value = '';
                        submitting = false;
                        if (alive()) form.querySelector('[type="submit"]').disabled = installBlocked;
                    }
                });
                trackOverlay(overlay);
                form.querySelector('select:not([disabled]), a[href], button').focus();
            } catch (error) { if (!instance.disposed && instance.printGuardLoadId === loadId) notify({title: t('desktop.store.title'), message: printGuardMessage(error)}); }
        }

        async function openGodsEyeModal() {
            const appId = 'gods-eye-view';
            const url = '/api/desktop/store/apps/' + appId + '/config';
            const fields = [
                ['CESIUM_ION_TOKEN', 'Cesium ion'], ['GOOGLE_MAPS_API_KEY', 'Google Maps'],
                ['OPENAI_API_KEY', 'OpenAI'], ['AISSTREAM_API_KEY', 'AISStream'],
                ['FIRMS_MAP_KEY', 'NASA FIRMS'], ['TOMTOM_API_KEY', 'TomTom'],
                ['OPENSKY_CLIENT_ID', 'OpenSky Client ID'], ['OPENSKY_CLIENT_SECRET', 'OpenSky Client Secret'],
                ['LL2_API_TOKEN', 'Launch Library']
            ];
            try {
                const config = await api(url);
                if (instance.disposed) return;
                if (instance.closeConfig) instance.closeConfig();
                const overlay = document.createElement('div');
                overlay.className = 'vd-modal-backdrop';
                overlay.innerHTML = `<form class="vd-modal vd-store-modal vd-gev-config" role="dialog" aria-modal="true" aria-label="${esc(t('desktop.store.gev_configure'))}">
                    <div class="vd-modal-title">God's Eye View · ${esc(t('desktop.store.gev_configure'))}</div>
                    <p class="vd-store-modal-copy">${esc(t('desktop.store.gev_intro'))}</p>
                    <p class="vd-store-modal-copy">${esc(t('desktop.store.gev_key_hint'))}</p>
                    <p class="vd-store-card-warning">${esc(t('desktop.store.gev_client_keys'))}</p>
                    ${fields.map(([key, label]) => `<div class="vd-gev-key-row">
                        <label class="vd-store-field"><span>${esc(label)} · ${esc(t(config.configured[key] ? 'desktop.store.gev_key_set' : 'desktop.store.gev_key_empty'))}</span><input type="password" name="${key}" autocomplete="new-password" maxlength="512"></label>
                        <label class="vd-store-choice"><input type="checkbox" data-clear="${key}"><span>${esc(t('desktop.delete'))}</span></label>
                    </div>`).join('')}
                    <label class="vd-store-field"><span>${esc(t('desktop.store.gev_origins'))}</span><textarea name="origins" rows="3" spellcheck="false">${esc((config.allowed_origins || []).join('\n'))}</textarea></label>
                    <p class="vd-store-modal-copy">${esc(t('desktop.store.gev_origins_hint'))}</p>
                    <p class="vd-store-modal-copy">${esc(t('desktop.store.gev_lan_warning'))}</p>
                    <p role="status" class="vd-store-card-warning" data-config-status>${config.pending ? esc(t('desktop.store.gev_pending')) : ''}</p>
                    <div class="vd-modal-actions"><button type="button" class="vd-button" data-action="cancel">${esc(t('desktop.cancel'))}</button><button type="submit" class="vd-button vd-button-primary">${esc(t('desktop.save'))}</button></div>
                </form>`;
                const previousFocus = document.activeElement;
                const form = overlay.querySelector('form');
                let submitting = false;
                const close = () => {
                    form.querySelectorAll('input[type="password"]').forEach(input => { input.value = ''; });
                    closeOverlay(overlay);
                    instance.closeConfig = null;
                    if (previousFocus && previousFocus.isConnected) previousFocus.focus();
                };
                instance.closeConfig = close;
                overlay.addEventListener('keydown', event => {
                    if (event.key === 'Escape' && !submitting) { event.preventDefault(); event.stopPropagation(); close(); }
                    if (event.key === 'Tab') {
                        const controls = [...form.querySelectorAll('input, textarea, button')].filter(el => !el.disabled);
                        const first = controls[0], last = controls[controls.length - 1];
                        if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
                        else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
                    }
                });
                form.querySelector('[data-action="cancel"]').addEventListener('click', () => { if (!submitting) close(); });
                overlay.addEventListener('click', event => { if (event.target === overlay && !submitting) close(); });
                form.addEventListener('submit', async event => {
                    event.preventDefault();
                    if (submitting) return;
                    const keys = {}, clear = [];
                    fields.forEach(([key]) => {
                        if (form.querySelector(`[data-clear="${key}"]`).checked) clear.push(key);
                        else if (form.elements[key].value.trim()) keys[key] = form.elements[key].value.trim();
                    });
                    const payload = { keys, clear, allowed_origins: form.elements.origins.value.split(/\r?\n/).map(value => value.trim()).filter(Boolean) };
                    const status = form.querySelector('[data-config-status]');
                    submitting = true;
                    form.querySelectorAll('button').forEach(button => { button.disabled = true; });
                    try {
                        const body = await api(url, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) });
                        if (instance.disposed) return;
                        close();
                        if (instance.disposed) return;
                        if (body.operation) {
                            busy.set(appId, body.operation);
                            renderCards();
                            pollOperation(appId, body.operation.id);
                        }
                    } catch (err) {
                        if (!instance.disposed) status.textContent = err.message || t('desktop.store.operation_failed');
                    } finally {
                        fields.forEach(([key]) => { delete keys[key]; });
                        submitting = false;
                        form.querySelectorAll('button').forEach(button => { button.disabled = false; });
                    }
                });
                trackOverlay(overlay);
                form.querySelector('input').focus();
            } catch (err) {
                notify({ title: t('desktop.store.title'), message: err.message });
            }
        }

        function showOperationError(appId, message) {
            message = message || t('desktop.store.operation_failed');
            operationErrors.set(appId, message);
            renderCards();
            notify({ title: t('desktop.store.title'), message });
        }

        async function startOperation(appId, action, url, method, payload) {
            if (instance.disposed) return;
            operationErrors.delete(appId);
            renderCards();
            try {
                const body = await api(url, {
                    method,
                    headers: { 'Content-Type': 'application/json' },
                    body: payload ? JSON.stringify(payload) : undefined
                });
                if (instance.disposed) return;
                if (body.operation) {
                    busy.set(appId, body.operation);
                    renderCards();
                    pollOperation(appId, body.operation.id);
                }
            } catch (err) {
                if (!instance.disposed) showOperationError(appId, err.message);
            }
        }

        // Known failures carry error_code/error_params; the translated text
        // replaces the backend's English error, which stays the fallback.
        function operationErrorText(op) {
            if (op && op.error_code) {
                const key = 'desktop.store.error_' + op.error_code;
                const text = t(key, op.error_params || {});
                if (typeof text === 'string' && text && text !== key) return text;
            }
            return op ? op.error : '';
        }

        async function pollOperation(appId, operationId) {
            if (pollingOperations.has(operationId)) return;
            pollingOperations.add(operationId);
            try {
                for (;;) {
                    if (instance.disposed) return;
                    await delay(1000);
                    if (instance.disposed) return;
                    const body = await api('/api/desktop/store/operations/' + encodeURIComponent(operationId));
                    if (instance.disposed) return;
                    const op = body.operation;
                    if (!op) throw new Error(t('desktop.store.operation_failed'));
                    busy.set(appId, op);
                    renderCards();
                    if (op.status === 'succeeded' || op.status === 'failed') {
                        busy.delete(appId);
                        if (op.status === 'failed') showOperationError(appId, appId === 'gods-eye-view' && op.type === 'configure' ? t('desktop.store.gev_pending') : operationErrorText(op));
                        scheduleLoad(true, true);
                        // A shell refresh failure must not hide the operation result.
                        try { await loadBootstrap(); } catch (_) {}
                        return;
                    }
                }
            } catch (err) {
                busy.delete(appId);
                if (!instance.disposed) {
                    showOperationError(appId, err.message);
                    scheduleLoad(true, true);
                }
            } finally {
                pollingOperations.delete(operationId);
            }
        }

        function delay(ms) {
            return new Promise(resolve => setTimeout(resolve, ms));
        }

        instance.onDesktopEvent = event => {
            if (!initialCatalogLoaded) return;
            if (isDesktopStoreChangedEvent(event)) scheduleLoad(false, true);
        };
        if (window.AuraSSE && typeof window.AuraSSE.on === 'function') {
            window.AuraSSE.on('virtual_desktop_event', instance.onDesktopEvent);
        }

        renderNav();

        if (typeof requestAnimationFrame === 'function') {
            requestAnimationFrame(() => scheduleLoad(true));
        } else {
            scheduleLoad(true);
        }
    }

    function dispose(windowId) {
        const instance = instances.get(windowId);
        if (!instance) return;
        instance.disposed = true;
        if (instance.closeConfig) instance.closeConfig();
        for (const overlay of instance.overlays) closeOverlay(overlay);
        instance.overlays.clear();
        if (instance.loadDebounceTimer) {
            clearTimeout(instance.loadDebounceTimer);
            instance.loadDebounceTimer = null;
        }
        if (window.AuraSSE && typeof window.AuraSSE.off === 'function' && instance.onDesktopEvent) {
            window.AuraSSE.off('virtual_desktop_event', instance.onDesktopEvent);
        }
        if (instance.host && instance.hostKeydown) instance.host.removeEventListener('keydown', instance.hostKeydown);
        if (instance.resizeObserver) instance.resizeObserver.disconnect();
        if (instance.resizeFrame && typeof cancelAnimationFrame === 'function') cancelAnimationFrame(instance.resizeFrame);
        if (instance.pollingOperations && typeof instance.pollingOperations.clear === 'function') {
            instance.pollingOperations.clear();
        }
        instances.delete(windowId);
    }

    function open(windowId, context) {
        instances.get(windowId)?.selectTarget(context);
    }

    window.SoftwareStoreApp = { render, dispose, open };
})();
