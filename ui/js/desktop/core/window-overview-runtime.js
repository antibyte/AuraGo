(function () {
    'use strict';

    const VALID_MODES = new Set(['switcher', 'app-group', 'snap-partner']);
    let config = {
        getWindows: () => [],
        getActiveWindowId: () => '',
        spaceName: () => '',
        appName: appId => String(appId || ''),
        previewMarkup: () => '',
        activateWindow: () => false,
        chooseSnapPartner: () => false,
        closeWindow: async () => 'skipped',
        closeBatch: async () => ({ status: 'skipped', closed: [], skipped: [] }),
        t: key => key,
        mountDialog: null
    };
    let current = null;
    let nextGeneration = 1;

    function configure(options = {}) {
        for (const key of [
            'getWindows', 'getActiveWindowId', 'spaceName', 'appName', 'previewMarkup', 'activateWindow',
            'chooseSnapPartner', 'closeWindow', 'closeBatch', 't', 'mountDialog'
        ]) {
            if (typeof options[key] === 'function') config[key] = options[key];
        }
        return api;
    }

    function translate(key, variables) {
        try {
            const value = String(config.t(key, variables));
            return value && value !== key ? value : key;
        } catch (_) {
            return key;
        }
    }

    function escapeHTML(value) {
        return String(value == null ? '' : value).replace(/[&<>"']/g, character => ({
            '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
        })[character]);
    }

    function windowId(win) {
        return String(win && win.id != null ? win.id : '');
    }

    function isMinimized(win) {
        return !!(win && (win.minimizing || win.element && win.element.style && win.element.style.display === 'none'));
    }

    function getWindows(view) {
        let result;
        try {
            result = config.getWindows({
                mode: view.mode,
                appId: view.appId,
                scope: view.scope,
                sourceId: view.sourceId,
                partnerZone: view.partnerZone,
                includeMinimized: true
            });
        } catch (_) {
            return [];
        }
        if (!Array.isArray(result)) return [];
        const seen = new Set();
        return result.filter(win => {
            const id = windowId(win);
            if (!win || !win.element || win.isGadget || !id || seen.has(id) || win.closing) return false;
            if (view.mode === 'snap-partner' && win.checkingClose) return false;
            if (view.mode === 'app-group' && win.appId !== view.appId) return false;
            if (view.mode === 'snap-partner' && id === view.sourceId) return false;
            seen.add(id);
            return true;
        });
    }

    function activeWindowId() {
        try { return String(config.getActiveWindowId() || ''); } catch (_) { return ''; }
    }

    function windowSpaceName(win) {
        try { return String(config.spaceName(win) || ''); } catch (_) { return ''; }
    }

    function windowTitle(win) {
        return String(win && win.title || config.appName(win && win.appId) || '');
    }

    function signatureFor(view, windows) {
        const activeId = activeWindowId();
        return JSON.stringify([
            view.mode, view.scope, activeId,
            windows.map(win => [windowId(win), windowTitle(win), String(win.appId || ''), windowSpaceName(win), isMinimized(win), !!win.closing])
        ]);
    }

    function isCurrent(view) {
        return !!view && current === view && current.generation === view.generation && !!view.dialog;
    }

    function open(options = {}) {
        if (!document.body) return false;
        const mode = VALID_MODES.has(options.mode) ? options.mode : 'switcher';
        const scope = options.scope === 'all' ? 'all' : 'current';
        const view = {
            generation: nextGeneration++,
            mode,
            appId: String(options.appId || ''),
            scope,
            sourceId: String(options.sourceId || ''),
            partnerZone: String(options.partnerZone || ''),
            opener: options.opener || document.activeElement,
            dialog: null,
            unmount: null,
            observer: null,
            windows: [],
            selectedId: '',
            selectedIndex: 0,
            focused: null,
            signature: '',
            previewCache: new Map(),
            pendingCloses: new Set(),
            actionDepth: 0,
            refreshPending: false,
            snapCommit: false,
            closeAppPending: false
        };
        const windows = getWindows(view);
        if (mode === 'snap-partner' && !windows.length) return false;
        if (current) closeView(current, { restoreFocus: false });
        view.windows = windows;
        const selected = windows.find(win => windowId(win) === activeWindowId()) || windows[0];
        view.selectedId = selected ? windowId(selected) : '';
        view.selectedIndex = Math.max(0, windows.findIndex(win => windowId(win) === view.selectedId));
        current = view;
        createDialog(view);
        document.body.classList.add('vd-window-overview-open');
        render(view, { focus: true, force: true });
        return isCurrent(view);
    }

    function createDialog(view) {
        const dialog = document.createElement('div');
        dialog.id = 'vd-window-overview';
        dialog.className = 'vd-window-overview';
        dialog.dataset.overviewMode = view.mode;
        dialog.setAttribute('role', 'dialog');
        dialog.setAttribute('aria-modal', 'true');
        dialog.addEventListener('click', event => handleClick(view, event));
        dialog.addEventListener('keydown', event => handleKeydown(view, event));
        dialog.addEventListener('focusin', event => rememberFocus(view, event.target));
        view.dialog = dialog;
        if (typeof config.mountDialog === 'function') {
            try { view.unmount = config.mountDialog(dialog) || null; } catch (_) { document.body.appendChild(dialog); }
        } else {
            document.body.appendChild(dialog);
        }
    }

    function titleFor(view) {
        if (view.mode === 'snap-partner') return translate('desktop.snap_partner_title');
        if (view.mode === 'app-group') return translate('desktop.window_app_list');
        return translate('desktop.window_overview_title');
    }

    function render(view, options = {}) {
        if (!isCurrent(view)) return;
        const previousId = view.selectedId;
        const previousIndex = Math.max(0, view.windows.findIndex(win => windowId(win) === previousId));
        const windows = getWindows(view);
        if (view.mode === 'snap-partner' && !windows.length) {
            closeView(view, { restoreFocus: true });
            return;
        }
        const signature = signatureFor(view, windows);
        view.windows = windows;
        if (windows.some(win => windowId(win) === previousId)) {
            view.selectedId = previousId;
            view.selectedIndex = windows.findIndex(win => windowId(win) === previousId);
        } else if (windows.length) {
            view.selectedIndex = Math.min(previousIndex, windows.length - 1);
            view.selectedId = windowId(windows[view.selectedIndex]);
        } else {
            view.selectedIndex = 0;
            view.selectedId = '';
        }
        if (!options.force && signature === view.signature) return;
        view.signature = signature;
        view.observer?.disconnect();
        view.observer = null;
        const title = titleFor(view);
        const appLabel = view.appId ? String(config.appName(view.appId) || view.appId) : '';
        const subtitle = view.mode === 'app-group'
            ? translate('desktop.window_group_count', { name: appLabel, count: windows.length })
            : '';
        const scopeButtons = view.mode === 'snap-partner' ? '' : `
            <div class="vd-window-overview-scope" role="group" aria-label="${escapeHTML(title)}">
                <button type="button" data-overview-scope="current" aria-pressed="${view.scope === 'current'}">${escapeHTML(translate('desktop.window_overview_current'))}</button>
                <button type="button" data-overview-scope="all" aria-pressed="${view.scope === 'all'}">${escapeHTML(translate('desktop.window_overview_all'))}</button>
            </div>`;
        const closeAppButton = view.mode === 'app-group'
            ? `<button type="button" class="vd-window-overview-close-app" data-overview-close-app ${view.closeAppPending ? 'disabled' : ''}>${escapeHTML(translate('desktop.close_app_windows'))}</button>`
            : '';
        const cards = windows.map((win, index) => renderCard(view, win, index)).join('');
        const empty = windows.length ? '' : `<p class="vd-window-overview-empty" role="status">${escapeHTML(translate('desktop.spaces_overview_empty'))}</p>`;
        view.dialog.dataset.overviewMode = view.mode;
        view.dialog.dataset.overviewScopeValue = view.scope;
        view.dialog.setAttribute('aria-labelledby', 'vd-window-overview-title');
        view.dialog.innerHTML = `
            <div class="vd-window-overview-backdrop" data-overview-dismiss></div>
            <section class="vd-window-overview-panel">
                <header class="vd-window-overview-header">
                    <div class="vd-window-overview-heading">
                        <h2 id="vd-window-overview-title">${escapeHTML(title)}</h2>
                        ${subtitle ? `<p>${escapeHTML(subtitle)}</p>` : ''}
                    </div>
                    <div class="vd-window-overview-header-actions">${scopeButtons}${closeAppButton}<button type="button" data-overview-dismiss aria-label="${escapeHTML(translate('desktop.close'))}">×</button></div>
                </header>
                <div class="vd-window-overview-list" role="list">${cards}</div>
                ${empty}
                <footer><button type="button" data-overview-dismiss>${escapeHTML(translate('desktop.close'))}</button></footer>
            </section>`;
        wirePreviewObserver(view);
        if (options.focus) focusPreferred(view);
        else restoreLogicalFocus(view);
    }

    function renderCard(view, win, index) {
        const id = windowId(win);
        const minimized = isMinimized(win);
        const active = id === activeWindowId();
        const selected = id === view.selectedId;
        const name = windowTitle(win);
        const appLabel = String(config.appName(win.appId) || win.appId || name);
        const workspace = view.scope === 'all' && view.mode !== 'snap-partner' ? windowSpaceName(win) : '';
        const activateLabel = view.mode === 'snap-partner'
            ? translate('desktop.snap_partner_select', { name })
            : name;
        const cachedPreview = view.previewCache.get(id);
        const preview = cachedPreview == null
            ? `<div class="vd-window-overview-preview" data-overview-preview="${escapeHTML(id)}" aria-hidden="true"><span class="vd-window-overview-preview-placeholder"></span></div>`
            : `<div class="vd-window-overview-preview" data-overview-preview="${escapeHTML(id)}" aria-hidden="true">${cachedPreview}</div>`;
        const minimizedLabel = minimized ? `<span class="vd-window-overview-minimized" data-overview-minimized="true">${escapeHTML(translate('desktop.window_minimized'))}</span>` : '';
        const closeButton = view.mode === 'snap-partner' ? '' : `<button type="button" data-window-close="${escapeHTML(id)}" aria-label="${escapeHTML(translate('desktop.menu_close_window') + ': ' + name)}" ${view.pendingCloses.has(id) ? 'disabled' : ''}>${escapeHTML(translate('desktop.menu_close_window'))}</button>`;
        return `<article class="vd-window-overview-card${selected ? ' is-selected' : ''}${active ? ' is-active' : ''}${minimized ? ' is-minimized' : ''}" role="listitem" data-overview-window="${escapeHTML(id)}" data-overview-index="${index}" data-overview-minimized="${minimized ? 'true' : 'false'}" aria-current="${active ? 'true' : 'false'}">
            <button type="button" class="vd-window-overview-activate" data-window-activate="${escapeHTML(id)}" aria-label="${escapeHTML(activateLabel)}" aria-pressed="${selected}">
                ${preview}
                <span class="vd-window-overview-card-title">${escapeHTML(name)}</span>
            <span class="vd-window-overview-card-meta"><span>${escapeHTML(appLabel)}</span>${workspace ? ` · <span data-overview-workspace-label="${escapeHTML(id)}">${escapeHTML(workspace)}</span>` : ''}${minimizedLabel ? ` · ${minimizedLabel}` : ''}</span>
            </button>
            ${closeButton}
        </article>`;
    }

    function wirePreviewObserver(view) {
        const targets = [...view.dialog.querySelectorAll('[data-overview-preview]')].filter(node => !view.previewCache.has(node.dataset.overviewPreview));
        if (!targets.length) return;
        if (typeof window.IntersectionObserver === 'function') {
            view.observer = new window.IntersectionObserver(entries => {
                if (!isCurrent(view)) return;
                for (const entry of entries) if (entry.isIntersecting) renderPreview(view, entry.target);
            }, { root: view.dialog.querySelector('.vd-window-overview-list'), rootMargin: '64px' });
            targets.forEach(node => view.observer.observe(node));
            return;
        }
        targets.forEach(node => {
            const rect = node.getBoundingClientRect();
            if (rect.bottom > 0 && rect.top < window.innerHeight && rect.right > 0 && rect.left < window.innerWidth) renderPreview(view, node);
        });
    }

    function renderPreview(view, node) {
        if (!isCurrent(view) || !node || !node.isConnected) return;
        const id = node.dataset.overviewPreview;
        const win = view.windows.find(item => windowId(item) === id);
        if (!win) return;
        if (!view.previewCache.has(id)) {
            let markup = '';
            try {
                markup = String(config.previewMarkup(win, {
                    maxW: 340, maxH: 190,
                    bodyOnly: true,
                    titleClass: 'vd-window-overview-preview-title',
                    labelClass: 'vd-window-overview-preview-label',
                    iconClass: 'vd-window-overview-preview-icon',
                    iconSize: 16,
                    fallbackIconSize: 38,
                    viewportClass: 'vd-window-overview-preview-viewport',
                    scaleClass: 'vd-window-overview-preview-scale',
                    fallbackClass: 'vd-window-overview-preview-fallback',
                    fallbackIconClass: 'vd-window-overview-preview-fallback-icon'
                }) || '');
            } catch (_) { markup = ''; }
            view.previewCache.set(id, markup);
        }
        node.innerHTML = view.previewCache.get(id);
        node.classList.add('is-ready');
        view.observer?.unobserve(node);
    }

    function visibleFocusables(view) {
        if (!view.dialog) return [];
        return [...view.dialog.querySelectorAll('button:not([disabled]), [tabindex]:not([tabindex="-1"])')]
            .filter(node => !node.closest('[hidden]') && node.getAttribute('aria-hidden') !== 'true'
                && (!node.getClientRects || node.getClientRects().length > 0));
    }

    function hasVisibleCompetingModal(view) {
        try {
            return [...document.querySelectorAll('[aria-modal="true"]')].some(modal => {
                if (modal === view.dialog || modal.hidden || modal.getAttribute('aria-hidden') === 'true') return false;
                return !modal.getClientRects || modal.getClientRects().length > 0;
            });
        } catch (_) {
            return false;
        }
    }

    function focusPreferred(view) {
        if (!isCurrent(view)) return;
        const selected = view.dialog.querySelector(`[data-window-activate="${cssEscape(view.selectedId)}"]`);
        const focus = selected || view.dialog.querySelector('[data-overview-scope="current"]')
            || view.dialog.querySelector('button[data-overview-dismiss]') || visibleFocusables(view)[0];
        focus?.focus({ preventScroll: true });
    }

    function cssEscape(value) {
        if (window.CSS && typeof window.CSS.escape === 'function') return window.CSS.escape(String(value));
        return String(value).replace(/["\\]/g, '\\$&');
    }

    function rememberFocus(view, element) {
        if (!isCurrent(view) || !element) return;
        const card = element.closest('[data-overview-window]');
        if (card) {
            const activate = element.closest('[data-window-activate]');
            const close = element.closest('[data-window-close]');
            view.focused = { windowId: card.dataset.overviewWindow, action: close ? 'close' : activate ? 'activate' : 'activate' };
            return;
        }
        const scope = element.closest('[data-overview-scope]');
        if (scope) view.focused = { action: 'scope', scope: scope.dataset.overviewScope };
        else if (element.closest('[data-overview-close-app]')) view.focused = { action: 'close-app' };
        else if (element.closest('[data-overview-dismiss]')) view.focused = { action: 'dismiss' };
    }

    function restoreLogicalFocus(view) {
        if (!isCurrent(view) || hasVisibleCompetingModal(view)) return;
        const descriptor = view.focused;
        let target = null;
        if (descriptor?.windowId) {
            const selector = descriptor.action === 'close' ? '[data-window-close]' : '[data-window-activate]';
            target = view.dialog.querySelector(`[data-overview-window="${cssEscape(descriptor.windowId)}"] ${selector}:not([disabled])`);
        } else if (descriptor?.action === 'scope') target = view.dialog.querySelector(`[data-overview-scope="${descriptor.scope}"]`);
        else if (descriptor?.action === 'close-app') target = view.dialog.querySelector('[data-overview-close-app]:not([disabled])');
        else if (descriptor?.action === 'dismiss') target = view.dialog.querySelector('button[data-overview-dismiss]');
        if (!target && view.selectedId) target = view.dialog.querySelector(`[data-window-activate="${cssEscape(view.selectedId)}"]`);
        if (!target) target = visibleFocusables(view)[0];
        target?.focus({ preventScroll: true });
    }

    function handleClick(view, event) {
        if (!isCurrent(view)) return;
        const target = event.target && event.target.closest ? event.target : null;
        if (!target) return;
        if (target.closest('[data-overview-dismiss]')) {
            event.preventDefault();
            finish({ activate: false });
            return;
        }
        const scope = target.closest('[data-overview-scope]');
        if (scope) {
            event.preventDefault();
            const nextScope = scope.dataset.overviewScope === 'all' ? 'all' : 'current';
            if (nextScope !== view.scope) {
                view.scope = nextScope;
                view.signature = '';
                render(view, { force: true });
            }
            return;
        }
        const closeApp = target.closest('[data-overview-close-app]');
        if (closeApp) {
            event.preventDefault();
            closeAppWindows(view);
            return;
        }
        const closeButton = target.closest('[data-window-close]');
        if (closeButton) {
            event.preventDefault();
            closeOneWindow(view, closeButton.dataset.windowClose);
            return;
        }
        const activate = target.closest('[data-window-activate]');
        if (!activate) return;
        event.preventDefault();
        const id = activate.dataset.windowActivate;
        view.selectedId = id;
        view.selectedIndex = view.windows.findIndex(win => windowId(win) === id);
        rememberFocus(view, activate);
        if (view.mode === 'snap-partner') chooseSnapPartner(view, id);
        else finish({ activate: true, windowId: id });
    }

    function handleKeydown(view, event) {
        if (!isCurrent(view)) return;
        if ((event.ctrlKey || event.metaKey) && event.key === 'Tab') return;
        if (event.key === 'Escape') {
            event.preventDefault();
            event.stopPropagation();
            finish({ activate: false });
            return;
        }
        if (event.key === 'Tab') {
            const focusables = visibleFocusables(view);
            if (focusables.length) {
                const first = focusables[0];
                const last = focusables[focusables.length - 1];
                if (event.shiftKey && document.activeElement === first) {
                    event.preventDefault();
                    last.focus();
                } else if (!event.shiftKey && document.activeElement === last) {
                    event.preventDefault();
                    first.focus();
                }
            }
            event.stopPropagation();
            return;
        }
        const navigationKeys = ['ArrowDown', 'ArrowUp', 'ArrowLeft', 'ArrowRight', 'Home', 'End'];
        if (!navigationKeys.includes(event.key) || !view.windows.length) return;
        event.preventDefault();
        event.stopPropagation();
        const currentIndex = Math.max(0, view.windows.findIndex(win => windowId(win) === view.selectedId));
        let nextIndex = currentIndex;
        if (event.key === 'Home') nextIndex = 0;
        else if (event.key === 'End') nextIndex = view.windows.length - 1;
        else if (event.key === 'ArrowDown' || event.key === 'ArrowRight') nextIndex = (currentIndex + 1) % view.windows.length;
        else nextIndex = (currentIndex - 1 + view.windows.length) % view.windows.length;
        selectWindow(view, nextIndex, true);
    }

    function selectWindow(view, index, focus) {
        if (!isCurrent(view) || !view.windows.length) return;
        const normalized = (index + view.windows.length) % view.windows.length;
        view.selectedIndex = normalized;
        view.selectedId = windowId(view.windows[normalized]);
        view.signature = '';
        render(view, { force: true });
        if (focus) view.dialog.querySelector(`[data-window-activate="${cssEscape(view.selectedId)}"]`)?.focus({ preventScroll: true });
    }

    function cycle(reverse = false) {
        const view = current;
        if (!view || view.mode !== 'switcher' || !view.windows.length) return false;
        const index = view.windows.findIndex(win => windowId(win) === view.selectedId);
        selectWindow(view, (index < 0 ? 0 : index) + (reverse ? -1 : 1), true);
        return true;
    }

    function finish(options = {}) {
        const view = current;
        if (!view) return false;
        const activate = options.activate === true;
        const requestedId = String(options.windowId || view.selectedId || '');
        const canActivate = activate && view.windows.some(win => windowId(win) === requestedId);
        const wasCurrent = isCurrent(view);
        closeView(view, { restoreFocus: !canActivate });
        if (canActivate && wasCurrent) {
            try { Promise.resolve(config.activateWindow(requestedId)).catch(() => {}); } catch (_) { /* Closing the dialog remains final if activation fails. */ }
        }
        return true;
    }

    function closeView(view, options = {}) {
        if (!view || current !== view) return;
        const opener = view.opener;
        const competingModal = hasVisibleCompetingModal(view);
        current = null;
        view.generation = nextGeneration++;
        view.observer?.disconnect();
        view.observer = null;
        if (typeof view.unmount === 'function') {
            try { view.unmount(); } catch (_) { /* Dialog cleanup must not prevent focus restoration. */ }
        }
        view.dialog?.remove();
        view.dialog = null;
        document.body.classList.remove('vd-window-overview-open');
        if (options.restoreFocus && !competingModal && opener && opener.isConnected && typeof opener.focus === 'function') opener.focus({ preventScroll: true });
    }

    async function closeOneWindow(view, id) {
        if (!isCurrent(view) || !id || view.pendingCloses.has(id)) return;
        const original = view.windows.find(win => windowId(win) === id);
        if (!original) return;
        view.pendingCloses.add(id);
        view.actionDepth++;
        view.dialog.querySelector(`[data-window-close="${cssEscape(id)}"]`)?.setAttribute('disabled', '');
        try {
            let result = 'skipped';
            try { result = await config.closeWindow(id); } catch (_) { result = 'cancelled'; }
            if (!isCurrent(view)) return;
            if (result !== 'accepted') view.focused = { windowId: id, action: 'close' };
            else {
                const oldIndex = Math.max(0, view.windows.findIndex(win => windowId(win) === id));
                view.selectedIndex = oldIndex;
                view.selectedId = id;
                view.focused = { windowId: id, action: 'activate' };
            }
        } finally {
            if (isCurrent(view)) {
                view.actionDepth = Math.max(0, view.actionDepth - 1);
                view.pendingCloses.delete(id);
                view.refreshPending = false;
                render(view, { force: true });
                restoreLogicalFocus(view);
            }
        }
    }

    async function closeAppWindows(view) {
        if (!isCurrent(view) || view.mode !== 'app-group' || view.closeAppPending) return;
        view.closeAppPending = true;
        view.actionDepth++;
        view.dialog.querySelector('[data-overview-close-app]')?.setAttribute('disabled', '');
        const ids = getWindows({ ...view, scope: 'all' }).filter(win => win.appId === view.appId).map(windowId);
        try {
            try { await config.closeBatch(ids); } catch (_) { /* The list remains the recovery surface for any partial close. */ }
            if (!isCurrent(view)) return;
            view.focused = { action: 'close-app' };
        } finally {
            if (isCurrent(view)) {
                view.actionDepth = Math.max(0, view.actionDepth - 1);
                view.closeAppPending = false;
                view.refreshPending = false;
                render(view, { force: true });
                restoreLogicalFocus(view);
            }
        }
    }

    async function chooseSnapPartner(view, id) {
        if (!isCurrent(view) || view.mode !== 'snap-partner' || view.snapCommit) return;
        const candidate = view.windows.find(win => windowId(win) === id);
        if (!candidate) return;
        const generation = view.generation;
        view.snapCommit = true;
        view.actionDepth++;
        let chosen = false;
        try {
            try { chosen = !!(await config.chooseSnapPartner(view.sourceId, id, view.partnerZone)); } catch (_) { chosen = false; }
        } finally {
            if (!isCurrent(view) || view.generation !== generation) return;
            view.actionDepth = Math.max(0, view.actionDepth - 1);
            view.snapCommit = false;
            if (chosen) {
                closeView(view, { restoreFocus: false });
                try { Promise.resolve(config.activateWindow(id)).catch(() => {}); } catch (_) { /* Snap selection is already committed. */ }
                return;
            }
            view.refreshPending = false;
            render(view, { force: true });
            restoreLogicalFocus(view);
        }
    }

    function refresh() {
        const view = current;
        if (!view || !isCurrent(view)) return false;
        if (view.actionDepth > 0 || view.snapCommit) {
            view.refreshPending = true;
            return true;
        }
        render(view);
        return true;
    }

    function isOpen(mode) {
        return !!current && isCurrent(current) && (!mode || current.mode === mode);
    }

    const api = { configure, open, cycle, finish, refresh, isOpen };
    window.AuraDesktopWindowOverview = api;
})();
