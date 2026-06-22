(function () {
    'use strict';

    const instances = new Map();
    const retiredStoreAppIDs = new Set(['emulatorjs']);

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
        const pollingOperations = new Set();
        const instance = { disposed: false, onDesktopEvent: null, pollingOperations, loadDebounceTimer: null };
        let dockerAvailable = true;
        let mutationsAllowed = true;
        let mutationDisabledReason = '';
        let loadToken = 0;
        let pendingCatalogRequest = null;
        let initialCatalogLoaded = false;
        instances.set(windowId, instance);

        host.innerHTML = `
            <div class="vd-store">
                <div class="vd-store-toolbar">
                    <div class="vd-store-heading">
                        <div class="vd-store-title">${esc(t('desktop.store.title'))}</div>
                        <div class="vd-store-subtitle">${esc(t('desktop.store.subtitle'))}</div>
                    </div>
                    <button type="button" class="vd-store-btn" data-action="refresh">${iconMarkup('refresh', 'R', 'vd-store-btn-icon', 15)}<span>${esc(t('desktop.context_refresh'))}</span></button>
                </div>
                <div class="vd-store-warning" hidden></div>
                <div class="vd-store-grid"></div>
            </div>`;

        const grid = host.querySelector('.vd-store-grid');
        const warning = host.querySelector('.vd-store-warning');
        host.querySelector('[data-action="refresh"]').addEventListener('click', () => scheduleLoad(true, true));

        function storeGridReady() {
            return !instance.disposed && grid && grid.isConnected;
        }

        function showStoreError(err, fallbackKey, fallbackText) {
            if (!storeGridReady()) return;
            const message = err && err.message ? err.message : t(fallbackKey, fallbackText);
            grid.innerHTML = `<div class="vd-store-empty">${esc(message || t('desktop.store.load_failed'))}</div>`;
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
            const token = ++loadToken;
            if (!storeGridReady()) return;
            grid.innerHTML = `<div class="vd-store-loading">${esc(t('desktop.loading'))}</div>`;
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
                    warning.textContent = mutationsAllowed ? '' : mutationDisabledText();
                }
                renderCards();
                resumeActiveOperationPolling();
                initialCatalogLoaded = true;
            } catch (err) {
                if (token !== loadToken || !storeGridReady()) return;
                console.error('Software store catalog load failed', err);
                showStoreError(err, 'desktop.store.load_failed', 'Could not load the software store.');
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

        function renderCards() {
            if (!storeGridReady()) return;
            try {
                if (!catalog.length) {
                    grid.innerHTML = `<div class="vd-store-empty">${esc(t('desktop.store.empty'))}</div>`;
                    return;
                }
                grid.innerHTML = catalog.map(entry => {
                const app = installedFor(entry.id);
                const operation = busy.get(entry.id) || activeOperationForApp(app);
                const status = operation && operation.type === 'install' && operation.status !== 'succeeded' && operation.status !== 'failed'
                    ? 'installing'
                    : app ? (app.status || 'installed') : 'available';
                const running = app && app.status === 'running';
                const stopped = app && app.status === 'stopped';
                const mutationDisabled = mutationsAllowed ? '' : mutationDisabledText();
                const actionDisabled = operation ? statusLabel(status, operation) : mutationDisabled;
                const themedStoreIcon = storeEntryUsesThemeIcon(entry);
                const logo = !themedStoreIcon && entry.logo_url ? `<img class="vd-store-logo" src="${esc(entry.logo_url)}" alt="" loading="lazy" onerror="this.hidden=true;this.nextElementSibling.hidden=false">` : '';
                const fallback = `<div class="vd-store-logo-fallback"${!themedStoreIcon && entry.logo_url ? ' hidden' : ''}>${iconMarkup(entry.icon || 'package', entry.name || 'A', 'vd-store-logo-icon', 30)}</div>`;
                const access = app ? accessLabel(app) : t('desktop.store.not_installed');
                const warningText = hostAccessWarning(entry);
                return `<article class="vd-store-card" data-app-id="${esc(entry.id)}">
                    <div class="vd-store-card-head">
                        <div class="vd-store-logo-wrap">${logo}${fallback}</div>
                        <div class="vd-store-card-title-wrap">
                            <div class="vd-store-card-title">${esc(entry.name)}${storeUiBadge(entry)}</div>
                            <div class="vd-store-card-image">${esc(entry.image)}</div>
                        </div>
                    </div>
                    <div class="vd-store-card-desc">${esc(entry.description)}</div>
                    ${warningText ? `<div class="vd-store-card-warning">${esc(warningText)}</div>` : ''}
                    <div class="vd-store-meta">
                        <span class="vd-store-status status-${esc(status)}">${esc(statusLabel(status, operation))}</span>
                        <span>${esc(access)}</span>
                    </div>
                    ${operation ? `<div class="vd-store-progress">${esc(statusLabel(operation.status, operation))}</div>` : ''}
                    <div class="vd-store-actions">
                        ${app ? `<button type="button" class="vd-store-btn vd-store-primary" data-action="open">${iconMarkup('browser', 'O', 'vd-store-btn-icon', 15)}<span>${esc(storeOpenLabel(entry))}</span></button>` : ''}
                        ${extraPortButtons(entry, app, actionDisabled)}
                        ${app && stopped ? `<button type="button" class="vd-store-btn" data-action="start" ${actionDisabled ? `disabled title="${esc(actionDisabled)}"` : ''}>${iconMarkup('run', 'S', 'vd-store-btn-icon', 15)}<span>${esc(t('desktop.store.start'))}</span></button>` : ''}
                        ${app && running ? `<button type="button" class="vd-store-btn" data-action="stop" ${actionDisabled ? `disabled title="${esc(actionDisabled)}"` : ''}>${iconMarkup('stop', 'S', 'vd-store-btn-icon', 15)}<span>${esc(t('desktop.store.stop'))}</span></button>` : ''}
                        ${app ? `<button type="button" class="vd-store-btn" data-action="update" ${actionDisabled ? `disabled title="${esc(actionDisabled)}"` : ''}>${iconMarkup('download', 'U', 'vd-store-btn-icon', 15)}<span>${esc(t('desktop.store.update'))}</span></button>
                            ${hasExposedCredentials(entry) ? `<button type="button" class="vd-store-btn" data-action="credentials">${iconMarkup('key', 'K', 'vd-store-btn-icon', 15)}<span>${esc(t('desktop.store.credentials'))}</span></button>` : ''}
                            ${entry.id === 'beszel' ? `<button type="button" class="vd-store-btn" data-action="configure-agent" ${actionDisabled ? `disabled title="${esc(actionDisabled)}"` : ''}>${iconMarkup('settings', 'A', 'vd-store-btn-icon', 15)}<span>${esc(t('desktop.store.configure_agent'))}</span></button>` : ''}
                            <button type="button" class="vd-store-btn vd-store-danger" data-action="uninstall" ${actionDisabled ? `disabled title="${esc(actionDisabled)}"` : ''}>${iconMarkup('trash', 'X', 'vd-store-btn-icon', 15)}<span>${esc(t('desktop.store.uninstall'))}</span></button>` : `<button type="button" class="vd-store-btn vd-store-primary" data-action="install" ${actionDisabled ? `disabled title="${esc(actionDisabled)}"` : ''}>${iconMarkup('download', 'I', 'vd-store-btn-icon', 15)}<span>${esc(t('desktop.store.install'))}</span></button>`}
                    </div>
                </article>`;
                }).join('');

                grid.querySelectorAll('.vd-store-card').forEach(card => {
                    const appId = card.dataset.appId;
                    card.querySelectorAll('[data-action]').forEach(button => {
                        button.addEventListener('click', () => handleAction(appId, button.dataset.action, button.dataset.portId));
                    });
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
            return entry.extra_ports.map(port => {
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
            if (action === 'uninstall') return openUninstallModal(appId);
            return startOperation(appId, action, '/api/desktop/store/apps/' + encodeURIComponent(appId) + '/' + encodeURIComponent(action), 'POST');
        }

        function isMutatingAction(action) {
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
            const entry = catalog.find(item => item.id === appId);
            if (!entry) return;
            const overlay = document.createElement('div');
            overlay.className = 'vd-modal-backdrop';
            overlay.innerHTML = `<form class="vd-modal vd-store-modal" role="dialog" aria-modal="true">
                <div class="vd-modal-title">${esc(t('desktop.store.install_title'))}: ${esc(entry.name)}</div>
                <div class="vd-store-modal-copy">${esc(t('desktop.store.install_copy'))}</div>
                <label class="vd-store-choice"><input type="radio" name="bind" value="local" checked><span><b>${esc(t('desktop.store.bind_local'))}</b><small>127.0.0.1</small></span></label>
                <label class="vd-store-choice"><input type="radio" name="bind" value="lan"><span><b>${esc(t('desktop.store.bind_lan'))}</b><small>0.0.0.0</small></span></label>
                <label class="vd-store-choice"><input type="checkbox" name="tailscale"><span><b>${esc(t('desktop.store.tailscale'))}</b><small>${esc(t('desktop.store.tailscale_hint'))}</small></span></label>
                <div class="vd-modal-actions">
                    <button type="button" class="vd-button" data-action="cancel">${esc(t('desktop.cancel'))}</button>
                    <button type="submit" class="vd-button vd-button-primary">${esc(t('desktop.store.install'))}</button>
                </div>
            </form>`;
            document.body.appendChild(overlay);
            const form = overlay.querySelector('form');
            overlay.querySelector('[data-action="cancel"]').addEventListener('click', () => overlay.remove());
            overlay.addEventListener('click', event => { if (event.target === overlay) overlay.remove(); });
            form.addEventListener('submit', event => {
                event.preventDefault();
                const bind = form.querySelector('input[name="bind"]:checked').value;
                const tailscale = form.querySelector('input[name="tailscale"]').checked;
                overlay.remove();
                startOperation(appId, 'install', '/api/desktop/store/install', 'POST', {
                    app_id: appId,
                    bind_mode: bind,
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
            document.body.appendChild(overlay);
            const form = overlay.querySelector('form');
            overlay.querySelector('[data-action="cancel"]').addEventListener('click', () => overlay.remove());
            overlay.addEventListener('click', event => { if (event.target === overlay) overlay.remove(); });
            form.addEventListener('submit', event => {
                event.preventDefault();
                const deleteData = form.querySelector('input[name="delete-data"]').checked;
                overlay.remove();
                startOperation(appId, 'uninstall', '/api/desktop/store/apps/' + encodeURIComponent(appId) + '?delete_data=' + encodeURIComponent(deleteData), 'DELETE');
            });
        }

        async function openStorePort(appId, portId) {
            const pendingWindow = window.open('about:blank', '_blank');
            if (pendingWindow) {
                pendingWindow.opener = null;
            }
            try {
                const body = await api('/api/desktop/store/apps/' + encodeURIComponent(appId) + '/open-url?port_id=' + encodeURIComponent(portId || ''));
                if (body.url) {
                    if (pendingWindow && !pendingWindow.closed) {
                        pendingWindow.location.replace(body.url);
                    } else {
                        window.open(body.url, '_blank', 'noopener');
                    }
                } else if (pendingWindow && !pendingWindow.closed) {
                    pendingWindow.close();
                }
            } catch (err) {
                if (pendingWindow && !pendingWindow.closed) {
                    pendingWindow.close();
                }
                notify({ title: t('desktop.store.title'), message: err.message });
            }
        }

        async function openCredentialsModal(appId) {
            try {
                const body = await api('/api/desktop/store/apps/' + encodeURIComponent(appId) + '/credentials');
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
                document.body.appendChild(overlay);
                overlay.querySelector('[data-action="close"]').addEventListener('click', () => overlay.remove());
                overlay.addEventListener('click', event => { if (event.target === overlay) overlay.remove(); });
            } catch (err) {
                notify({ title: t('desktop.store.title'), message: err.message });
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
            document.body.appendChild(overlay);
            const form = overlay.querySelector('form');
            overlay.querySelector('[data-action="cancel"]').addEventListener('click', () => overlay.remove());
            overlay.addEventListener('click', event => { if (event.target === overlay) overlay.remove(); });
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
                    overlay.remove();
                    scheduleLoad(true, true);
                    notify({ title: t('desktop.store.title'), message: t('desktop.store.agent_configured') });
                } catch (err) {
                    notify({ title: t('desktop.store.title'), message: err.message });
                }
            });
        }

        async function startOperation(appId, action, url, method, payload) {
            try {
                const body = await api(url, {
                    method,
                    headers: { 'Content-Type': 'application/json' },
                    body: payload ? JSON.stringify(payload) : undefined
                });
                if (body.operation) {
                    busy.set(appId, body.operation);
                    renderCards();
                    pollOperation(appId, body.operation.id);
                }
            } catch (err) {
                notify({ title: t('desktop.store.title'), message: err.message });
            }
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
                    const op = body.operation;
                    busy.set(appId, op);
                    renderCards();
                    if (!op || op.status === 'succeeded' || op.status === 'failed') {
                        busy.delete(appId);
                        scheduleLoad(true, true);
                        await loadBootstrap();
                        if (op && op.status === 'failed') notify({ title: t('desktop.store.title'), message: op.error || t('desktop.store.operation_failed') });
                        return;
                    }
                }
            } catch (err) {
                busy.delete(appId);
                if (!instance.disposed) scheduleLoad(true, true);
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
        if (instance.loadDebounceTimer) {
            clearTimeout(instance.loadDebounceTimer);
            instance.loadDebounceTimer = null;
        }
        if (window.AuraSSE && typeof window.AuraSSE.off === 'function' && instance.onDesktopEvent) {
            window.AuraSSE.off('virtual_desktop_event', instance.onDesktopEvent);
        }
        if (instance.pollingOperations && typeof instance.pollingOperations.clear === 'function') {
            instance.pollingOperations.clear();
        }
        instances.delete(windowId);
    }

    window.SoftwareStoreApp = { render, dispose };
})();
