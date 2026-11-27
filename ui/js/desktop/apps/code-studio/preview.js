    // === Plan B: HTML preview pane ===
    // The pane sits right of the editor group and shows the saved file through /code-studio-preview/<token>/. The page
    // runs sandboxed without allow-same-origin (the server sends the same flags as a CSP sandbox), so it has an opaque
    // origin and can never read AuraGo cookies, storage or API responses. It reaches Code Studio only via postMessage;
    // a message counts only when its source is this pane's iframe, its origin is opaque and its shape matches.

    const PREVIEW_PREFS_KEY = 'aurago.codeStudio.preview.v1';
    const PREVIEW_SANDBOX = 'allow-scripts allow-forms allow-modals allow-popups allow-downloads';
    const PREVIEW_VIEWPORTS = ['responsive', 'tablet', 'mobile'];
    const PREVIEW_RESIZE_STEP = 16;
    const PREVIEW_DEFAULT_WIDTH = 480;
    const PREVIEW_DEVICE_INSET = 34; // stage padding 24 px + device bezel 10 px
    const PREVIEW_GLYPHS = {
        eye: '<path d="M1.5 8S4 3.5 8 3.5 14.5 8 14.5 8 12 12.5 8 12.5 1.5 8 1.5 8z"/><circle cx="8" cy="8" r="2"/>',
        reload: '<path d="M13.25 8A5.25 5.25 0 1 1 11.7 4.3"/><path d="M13.25 2.25v3.5h-3.5"/>',
        globe: '<circle cx="8" cy="8" r="6"/><path d="M2 8h12M8 2c1.8 1.7 2.7 3.7 2.7 6S9.8 12.3 8 14C6.2 12.3 5.3 10.3 5.3 8S6.2 3.7 8 2z"/>',
        responsive: '<rect x="1.5" y="2.75" width="13" height="8.5" rx="1.25"/><path d="M5.5 13.75h5M8 11.25v2.5"/>',
        tablet: '<rect x="3" y="1.5" width="10" height="13" rx="1.5"/><path d="M7.25 12.25h1.5"/>',
        mobile: '<rect x="4.5" y="1.5" width="7" height="13" rx="1.5"/><path d="M7.25 12.25h1.5"/>',
        alert: '<circle cx="8" cy="8" r="6"/><path d="M8 4.75v3.75M8 11.1v.15"/>',
        pin: '<path d="M9.75 1.75l4.5 4.5-1.9.6-2.6 2.6.3 3-1.3 1.3-6.5-6.5 1.3-1.3 3 .3 2.6-2.6z"/><path d="M5.2 10.8 1.75 14.25"/>',
        external: '<path d="M9.25 2.25h4.5v4.5M13.75 2.25 7.5 8.5"/><path d="M12 9.5v3.25a1 1 0 0 1-1 1H3.25a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1H6.5"/>',
        close: '<path d="M4 4l8 8M12 4l-8 8"/>'
    };

    function previewModel() {
        return window.CodeStudioPreviewModel || null;
    }

    function previewGlyph(name, size) {
        const pixels = Number(size) || 14;
        return `<svg class="cs-hp-glyph" viewBox="0 0 16 16" width="${pixels}" height="${pixels}" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">${PREVIEW_GLYPHS[name] || ''}</svg>`;
    }

    function loadPreviewPrefs() {
        try {
            const saved = JSON.parse(localStorage.getItem(PREVIEW_PREFS_KEY) || '{}') || {};
            const width = Number(saved.width);
            return {
                viewport: PREVIEW_VIEWPORTS.includes(saved.viewport) ? saved.viewport : 'responsive',
                width: Number.isFinite(width) && width > 0 ? width : PREVIEW_DEFAULT_WIDTH
            };
        } catch (_) {
            return { viewport: 'responsive', width: PREVIEW_DEFAULT_WIDTH };
        }
    }

    function savePreviewPrefs() {
        const preview = previewState();
        try {
            localStorage.setItem(PREVIEW_PREFS_KEY, JSON.stringify({ viewport: preview.viewport, width: Math.round(preview.width) }));
        } catch (_) {}
    }

    function previewState() {
        if (!state.htmlPreview) {
            const prefs = loadPreviewPrefs();
            state.htmlPreview = {
                open: false, pinned: false, followedPath: '',
                path: '', directory: false, suffix: '',
                base: '', expiresAt: '', tokenRequest: null,
                status: 'empty', errorTitle: '', errorDetail: '',
                viewport: prefs.viewport, width: prefs.width,
                console: [], consoleOpen: false,
                navigation: 0, pane: null, observer: null, messageWired: false
            };
        }
        return state.htmlPreview;
    }

    function previewPane() {
        const root = studioRoot();
        return root ? root.querySelector('[data-html-preview]') : null;
    }

    function isPreviewablePath(path) {
        const model = previewModel();
        return !!model && model.isPreviewablePath(path);
    }

    function isCodeEditorTarget(element) {
        return !!(element && typeof element.closest === 'function' && element.closest('.cm-editor'));
    }

    // Called by renderShell and whenever the pane opens or closes. Returns true when it (re)built the pane markup.
    function renderPreviewPane(options) {
        const root = studioRoot();
        const pane = previewPane();
        if (!root || !pane) return false;
        const preview = previewState();
        root.dataset.preview = preview.open ? 'visible' : 'hidden';
        pane.hidden = !preview.open;
        applyPreviewWidth();
        if (!preview.open) return false;
        let built = false;
        if (preview.pane !== pane) {
            buildPreviewPane(pane);
            built = true;
        }
        updatePreviewChrome();
        if (built) {
            if (preview.path && !(options && options.navigate === false)) navigatePreview();
            else if (!preview.path) setPreviewStatus('empty');
        }
        return built;
    }

    function buildPreviewPane(pane) {
        const preview = previewState();
        const consoleId = 'cs-hp-console-' + String(state.windowId).replace(/[^A-Za-z0-9_-]/g, '');
        const trEsc = (key, fallback) => esc(tr(key, fallback));
        const viewportLabels = {
            responsive: tr('codeStudio.previewResponsive', 'Responsive'),
            tablet: tr('codeStudio.previewTablet', 'Tablet (768 px)'),
            mobile: tr('codeStudio.previewMobile', 'Mobile (375 px)')
        };
        pane.setAttribute('aria-label', tr('codeStudio.previewTitle', 'Preview'));
        pane.innerHTML = `
            <div class="cs-hp-resize" data-hp-resize role="separator" aria-orientation="vertical" tabindex="0" aria-label="${trEsc('codeStudio.previewResize', 'Resize preview')}" title="${trEsc('codeStudio.previewResize', 'Resize preview')}"></div>
            <div class="cs-hp-toolbar" role="toolbar" aria-label="${trEsc('codeStudio.previewTitle', 'Preview')}">
                <button type="button" class="cs-hp-button" data-hp-action="reload" aria-label="${trEsc('codeStudio.previewReload', 'Reload preview')}" title="${trEsc('codeStudio.previewReload', 'Reload preview')}">${previewGlyph('reload')}</button>
                <form class="cs-hp-address" data-hp-address-form>
                    <span class="cs-hp-address-icon" aria-hidden="true">${previewGlyph('globe', 13)}</span>
                    <input type="text" class="cs-hp-address-input" data-hp-address spellcheck="false" autocomplete="off" placeholder="index.html" aria-label="${trEsc('codeStudio.previewAddress', 'Preview address (path inside the workspace)')}">
                </form>
                <div class="cs-hp-segmented" role="radiogroup" data-hp-viewports aria-label="${trEsc('codeStudio.previewViewport', 'Viewport')}">
                    ${PREVIEW_VIEWPORTS.map(id => `<button type="button" role="radio" class="cs-hp-segment" data-hp-viewport="${id}" aria-checked="false" tabindex="-1" aria-label="${esc(viewportLabels[id])}" title="${esc(viewportLabels[id])}">${previewGlyph(id)}</button>`).join('')}
                </div>
                <button type="button" class="cs-hp-button cs-hp-console-button" data-hp-action="console" aria-expanded="false" aria-controls="${consoleId}" title="${trEsc('codeStudio.previewConsole', 'Console')}">${previewGlyph('alert')}<span class="cs-hp-badge" data-hp-console-count hidden>0</span></button>
                <button type="button" class="cs-hp-button" data-hp-action="pin" aria-pressed="false" aria-label="${trEsc('codeStudio.previewPin', 'Keep showing this file')}">${previewGlyph('pin')}</button>
                <button type="button" class="cs-hp-button" data-hp-action="external" aria-label="${trEsc('codeStudio.previewOpenExternal', 'Open in new browser tab')}" title="${trEsc('codeStudio.previewOpenExternal', 'Open in new browser tab')}">${previewGlyph('external')}</button>
                <button type="button" class="cs-hp-button" data-hp-action="close" aria-label="${trEsc('codeStudio.previewClose', 'Close preview')}" title="${trEsc('codeStudio.previewClose', 'Close preview')}">${previewGlyph('close')}</button>
            </div>
            <div class="cs-hp-hint" data-hp-dirty role="status" hidden><span class="cs-hp-hint-dot" aria-hidden="true"></span><span>${trEsc('codeStudio.previewDirty', 'Showing saved version. Save to update the preview.')}</span></div>
            <section class="cs-hp-console" id="${consoleId}" data-hp-console hidden aria-label="${trEsc('codeStudio.previewConsole', 'Console')}">
                <header class="cs-hp-console-head"><span>${trEsc('codeStudio.previewConsole', 'Console')}</span><button type="button" class="cs-hp-text-button" data-hp-action="clear-console">${trEsc('codeStudio.previewConsoleClear', 'Clear console')}</button></header>
                <ol class="cs-hp-console-list" data-hp-console-list></ol>
            </section>
            <div class="cs-hp-stage" data-hp-stage data-viewport="${esc(preview.viewport)}" aria-busy="false">
                <div class="cs-hp-device" data-hp-device>
                    <iframe class="cs-hp-frame" data-hp-frame sandbox="${PREVIEW_SANDBOX}" referrerpolicy="no-referrer" title="${trEsc('codeStudio.previewTitle', 'Preview')}"></iframe>
                </div>
                <div class="cs-hp-device-label" data-hp-device-label hidden></div>
                <div class="cs-hp-loading" data-hp-loading hidden>
                    <span class="cs-hp-shimmer"></span><span class="cs-hp-shimmer short"></span><span class="cs-hp-shimmer"></span><span class="cs-hp-shimmer short"></span>
                    <span class="cs-hp-sr">${trEsc('codeStudio.previewLoading', 'Loading preview...')}</span>
                </div>
                <div class="cs-hp-card" data-hp-empty hidden>
                    <span class="cs-hp-card-icon" aria-hidden="true">${previewGlyph('eye', 26)}</span>
                    <strong>${trEsc('codeStudio.previewEmptyTitle', 'Open an HTML file to preview it')}</strong>
                    <p>${trEsc('codeStudio.previewEmptyHint', 'HTML, HTM and SVG files open here and reload when you save.')}</p>
                    <div class="cs-hp-card-actions"><button type="button" class="cs-button primary" data-hp-action="preview-active">${trEsc('codeStudio.previewEmptyAction', 'Preview active file')}</button></div>
                </div>
                <div class="cs-hp-card cs-hp-error" data-hp-error role="alert" hidden>
                    <span class="cs-hp-card-icon" aria-hidden="true">${previewGlyph('alert', 24)}</span>
                    <strong data-hp-error-title></strong>
                    <p data-hp-error-detail></p>
                    <div class="cs-hp-card-actions">
                        <button type="button" class="cs-button primary" data-hp-action="retry">${trEsc('codeStudio.previewRetry', 'Try again')}</button>
                        <button type="button" class="cs-button" data-hp-action="close">${trEsc('codeStudio.previewClose', 'Close preview')}</button>
                    </div>
                </div>
            </div>`;
        preview.pane = pane;
        wirePreviewPane(pane);
    }

    function wirePreviewPane(pane) {
        const preview = previewState();
        pane.addEventListener('click', bind(event => {
            const control = event.target.closest('[data-hp-action],[data-hp-viewport]');
            if (!control || !pane.contains(control) || control.disabled) return;
            if (control.dataset.hpViewport) setPreviewViewport(control.dataset.hpViewport);
            else runPreviewAction(control.dataset.hpAction);
        }));
        const viewports = pane.querySelector('[data-hp-viewports]');
        viewports.addEventListener('keydown', bind(event => {
            const index = PREVIEW_VIEWPORTS.indexOf(previewState().viewport);
            let next = -1;
            if (event.key === 'ArrowRight' || event.key === 'ArrowDown') next = (index + 1) % PREVIEW_VIEWPORTS.length;
            else if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') next = (index + PREVIEW_VIEWPORTS.length - 1) % PREVIEW_VIEWPORTS.length;
            else if (event.key === 'Home') next = 0;
            else if (event.key === 'End') next = PREVIEW_VIEWPORTS.length - 1;
            if (next < 0) return;
            event.preventDefault();
            setPreviewViewport(PREVIEW_VIEWPORTS[next]);
            const button = viewports.querySelector(`[data-hp-viewport="${PREVIEW_VIEWPORTS[next]}"]`);
            if (button) button.focus();
        }));
        const input = pane.querySelector('[data-hp-address]');
        pane.querySelector('[data-hp-address-form]').addEventListener('submit', bind(event => {
            event.preventDefault();
            navigatePreviewAddress(input.value);
        }));
        input.addEventListener('input', () => input.removeAttribute('aria-invalid'));
        input.addEventListener('keydown', bind(event => {
            if (event.key !== 'Escape') return;
            event.preventDefault();
            event.stopPropagation();
            input.value = previewAddressText();
            input.removeAttribute('aria-invalid');
            input.blur();
        }));
        pane.querySelector('[data-hp-console]').addEventListener('keydown', bind(event => {
            if (event.key !== 'Escape') return;
            event.preventDefault();
            event.stopPropagation();
            togglePreviewConsole(false, true);
        }));
        const frame = pane.querySelector('[data-hp-frame]');
        frame.addEventListener('load', bind(() => onPreviewFrameLoad(frame)));
        wirePreviewResize(pane);
        if (preview.observer) preview.observer.disconnect();
        if (typeof ResizeObserver === 'function') {
            const observer = new ResizeObserver(bind(() => applyPreviewViewport()));
            observer.observe(pane.querySelector('[data-hp-stage]'));
            preview.observer = observer;
            registerDisposer(() => observer.disconnect());
        }
        if (!preview.messageWired) {
            preview.messageWired = true;
            const onMessage = bind(event => handlePreviewMessage(event));
            window.addEventListener('message', onMessage);
            registerDisposer(() => window.removeEventListener('message', onMessage));
        }
    }

    function openPreview(path, options) {
        const preview = previewState();
        const next = normalizeCodeStudioPath(path);
        if (next === WORKSPACE_ROOT) return;
        const same = preview.open && preview.path === next && !preview.directory && !preview.suffix && preview.status === 'ready';
        preview.open = true;
        preview.path = next;
        preview.directory = false;
        preview.suffix = '';
        preview.followedPath = next;
        if (options && options.pin === true) preview.pinned = true;
        const built = renderPreviewPane({ navigate: false });
        if (built || !same) navigatePreview();
        syncPreviewEyeButton();
        renderWindowMenus();
    }

    function openPreviewForActive() {
        const tab = activeTab();
        if (isSourceTab(tab) && isPreviewablePath(tab.path)) {
            openPreview(tab.path);
            return;
        }
        const preview = previewState();
        const wasOpen = preview.open;
        preview.open = true;
        const built = renderPreviewPane({ navigate: false });
        if (preview.path && (built || !wasOpen)) navigatePreview();
        else if (!preview.path) setPreviewStatus('empty');
        renderWindowMenus();
    }

    function togglePreviewPane() {
        if (previewState().open) closePreviewPane();
        else openPreviewForActive();
    }

    function closePreviewPane() {
        const preview = previewState();
        if (!preview.open) return;
        const pane = preview.pane;
        const hadFocus = !!(pane && pane.contains(document.activeElement));
        preview.open = false;
        preview.consoleOpen = false;
        preview.navigation += 1;
        const frame = pane && pane.querySelector('[data-hp-frame]');
        if (frame) frame.src = 'about:blank';
        renderPreviewPane();
        syncPreviewEyeButton();
        renderWindowMenus();
        measureVisibleEditors();
        scheduleTerminalFit();
        if (hadFocus) {
            const view = focusedEditorView(activeTab());
            if (view && typeof view.focus === 'function') view.focus();
        }
    }

    // The workspace switch (workspaceActivate) calls this: a token names exactly one workspace, so the pane closes,
    // the token is dropped and a token request still on its way is ignored when it lands.
    function resetPreview() {
        const preview = previewState();
        closePreviewPane();
        preview.navigation += 1;
        Object.assign(preview, { path: '', directory: false, suffix: '', base: '', expiresAt: '', tokenRequest: null, console: [], followedPath: '', pinned: false, status: 'empty', errorTitle: '', errorDetail: '' });
    }

    function ensurePreviewToken(target, force, path) {
        const preview = target.htmlPreview;
        const model = previewModel();
        if (!force && preview.base && !model.tokenNeedsRefresh(preview.expiresAt, Date.now())) return Promise.resolve(preview.base);
        if (!force && preview.tokenRequest) return preview.tokenRequest;
        // api() sends the workspace header of this instance, so the token belongs to the workspace shown right now.
        const request = api('/api/code-studio/preview', {
            method: 'POST',
            body: JSON.stringify({ path: path || preview.path || WORKSPACE_ROOT })
        }).then(result => {
            const base = String(result && result.base || '');
            if (!base.startsWith('/code-studio-preview/') || !base.endsWith('/')) throw new Error('invalid preview response');
            // A reset (workspace switch) cleared tokenRequest meanwhile: the token is for the old workspace.
            if (preview.tokenRequest !== request) return base;
            preview.base = base;
            preview.expiresAt = String(result.expires_at || '');
            return base;
        }).finally(() => {
            if (preview.tokenRequest === request) preview.tokenRequest = null;
        });
        preview.tokenRequest = request;
        return request;
    }

    // HEAD first (the sandboxed page cannot tell the parent its status), then the iframe. A 401 means the token
    // expired or the session secret rotated: one retry with a fresh token.
    async function navigatePreview() {
        const target = state;
        const preview = previewState();
        const model = previewModel();
        if (!model || !preview.open || !preview.path) {
            setPreviewStatus('empty');
            return;
        }
        const navigation = ++preview.navigation;
        setPreviewStatus('loading');
        updatePreviewChrome();
        let status = 0;
        let failure = '';
        let url = '';
        for (let attempt = 0; attempt < 2; attempt += 1) {
            failure = '';
            try {
                const base = await ensurePreviewToken(target, attempt > 0, preview.path);
                if (!isLiveInstance(target) || preview.navigation !== navigation) return;
                url = model.buildUrl(base, preview.path, preview.directory, preview.suffix);
                const response = await fetch(url, { method: 'HEAD', credentials: 'omit', cache: 'no-store' });
                status = response.status;
            } catch (err) {
                status = 0;
                failure = err && err.message ? err.message : String(err);
            }
            if (!isLiveInstance(target) || preview.navigation !== navigation) return;
            if (status !== 401) break;
            preview.base = '';
        }
        runWithInstance(target, () => {
            const frame = preview.pane && preview.pane.querySelector('[data-hp-frame]');
            if (!frame || !preview.open) return;
            if (status >= 200 && status < 300) {
                frame.title = tr('codeStudio.previewFrameTitle', 'Preview of {{path}}', { path: previewAddressText() || '/' });
                frame.src = url; // assigning the same URL navigates again, so this also reloads
                return;
            }
            showPreviewError(status, failure);
        });
    }

    function onPreviewFrameLoad(frame) {
        const preview = previewState();
        if (preview.status !== 'loading') return;
        const src = frame.getAttribute('src') || '';
        if (!src.startsWith('/code-studio-preview/')) return;
        setPreviewStatus('ready');
    }

    function showPreviewError(status, failure) {
        const preview = previewState();
        const path = previewAddressText() || '/';
        if (status === 404) {
            preview.errorTitle = tr('codeStudio.previewNotFoundTitle', 'File not found');
            preview.errorDetail = tr('codeStudio.previewNotFound', '{{path}} does not exist in this workspace.', { path });
        } else if (status === 413) {
            preview.errorTitle = tr('codeStudio.previewTooLargeTitle', 'File too large');
            preview.errorDetail = tr('codeStudio.previewTooLarge', '{{path}} is larger than 25 MB and cannot be previewed.', { path });
        } else {
            preview.errorTitle = tr('codeStudio.previewFailedTitle', 'Preview unavailable');
            preview.errorDetail = tr('codeStudio.previewFailed', 'The preview could not be loaded: {{error}}', { error: failure || ('HTTP ' + status) });
        }
        const frame = preview.pane && preview.pane.querySelector('[data-hp-frame]');
        if (frame) frame.src = 'about:blank';
        setPreviewStatus('error');
    }

    function setPreviewStatus(status) {
        const preview = previewState();
        preview.status = status;
        const pane = preview.pane;
        if (!pane || !pane.isConnected) return;
        pane.querySelector('[data-hp-loading]').hidden = status !== 'loading';
        pane.querySelector('[data-hp-empty]').hidden = status !== 'empty';
        const error = pane.querySelector('[data-hp-error]');
        error.hidden = status !== 'error';
        if (status === 'error') {
            error.querySelector('[data-hp-error-title]').textContent = preview.errorTitle;
            error.querySelector('[data-hp-error-detail]').textContent = preview.errorDetail;
        }
        pane.querySelector('[data-hp-device]').hidden = status === 'empty' || status === 'error';
        pane.querySelector('[data-hp-stage]').setAttribute('aria-busy', String(status === 'loading'));
        const emptyAction = pane.querySelector('[data-hp-action="preview-active"]');
        if (emptyAction) {
            const tab = activeTab();
            emptyAction.disabled = !(isSourceTab(tab) && isPreviewablePath(tab.path));
        }
        applyPreviewViewport();
    }

    function previewAddressText() {
        const preview = previewState();
        const model = previewModel();
        return model ? model.addressText(preview.path, preview.directory, preview.suffix) : '';
    }

    function navigatePreviewAddress(text) {
        const preview = previewState();
        const model = previewModel();
        const input = preview.pane && preview.pane.querySelector('[data-hp-address]');
        const parsed = model && model.parseAddress(text);
        if (!parsed) {
            if (input) input.setAttribute('aria-invalid', 'true');
            flashStatus(tr('codeStudio.previewAddressInvalid', 'Enter a path inside the workspace.'));
            return;
        }
        if (input) input.removeAttribute('aria-invalid');
        preview.path = parsed.path;
        preview.directory = parsed.directory;
        preview.suffix = parsed.suffix;
        navigatePreview();
    }

    // live.js reports every page load (also link clicks inside the preview): the address follows, the console clears.
    function applyPreviewLocation(text) {
        const preview = previewState();
        const parsed = previewModel().parseAddress(text || '/');
        if (parsed) {
            preview.path = parsed.path;
            preview.directory = parsed.directory;
            preview.suffix = parsed.suffix;
        }
        preview.console = [];
        updatePreviewChrome();
    }

    function updatePreviewChrome() {
        const preview = previewState();
        const pane = preview.pane;
        if (!pane || !pane.isConnected) return;
        const hasPath = !!preview.path;
        const input = pane.querySelector('[data-hp-address]');
        if (input && document.activeElement !== input) input.value = previewAddressText();
        const pin = pane.querySelector('[data-hp-action="pin"]');
        if (pin) {
            pin.setAttribute('aria-pressed', String(preview.pinned));
            pin.title = preview.pinned ? tr('codeStudio.previewUnpin', 'Follow the active file') : tr('codeStudio.previewPin', 'Keep showing this file');
            pin.disabled = !hasPath;
        }
        ['reload', 'external'].forEach(action => {
            const button = pane.querySelector(`[data-hp-action="${action}"]`);
            if (button) button.disabled = !hasPath;
        });
        pane.querySelectorAll('[data-hp-viewport]').forEach(button => {
            const active = button.dataset.hpViewport === preview.viewport;
            button.setAttribute('aria-checked', String(active));
            button.tabIndex = active ? 0 : -1;
            button.classList.toggle('active', active);
        });
        renderPreviewConsole();
        syncPreviewDirtyHint();
        applyPreviewViewport();
        syncPreviewEyeButton();
    }

    function runPreviewAction(action) {
        const preview = previewState();
        switch (action) {
            case 'reload':
            case 'retry':
                navigatePreview();
                break;
            case 'console':
                togglePreviewConsole();
                break;
            case 'clear-console':
                preview.console = [];
                renderPreviewConsole();
                break;
            case 'pin':
                preview.pinned = !preview.pinned;
                if (!preview.pinned) {
                    preview.followedPath = '';
                    previewFollowActiveTab();
                }
                updatePreviewChrome();
                break;
            case 'external':
                openPreviewInBrowserTab();
                break;
            case 'close':
                closePreviewPane();
                break;
            case 'preview-active': {
                const tab = activeTab();
                if (isSourceTab(tab) && isPreviewablePath(tab.path)) openPreview(tab.path);
                break;
            }
            default:
                break;
        }
    }

    function handlePreviewMessage(event) {
        const preview = state.htmlPreview;
        const frame = preview && preview.pane && preview.pane.querySelector('[data-hp-frame]');
        // Only our own sandboxed frame counts: same WindowProxy, opaque origin, exact shape.
        if (!frame || !frame.contentWindow || event.source !== frame.contentWindow || event.origin !== 'null') return;
        const message = previewModel().normalizeMessage(event.data);
        if (!message) return;
        if (message.kind === 'location') {
            applyPreviewLocation(message.text);
            return;
        }
        if (message.kind === 'expired') {
            preview.base = '';
            navigatePreview();
            return;
        }
        if (message.kind === 'reloaded') return;
        preview.console = previewModel().pushEntry(preview.console, { kind: message.kind, text: message.text, at: Date.now() });
        renderPreviewConsole();
    }

    function renderPreviewConsole() {
        const preview = previewState();
        const pane = preview.pane;
        const model = previewModel();
        if (!pane || !model) return;
        const count = preview.console.filter(entry => model.isErrorKind(entry.kind)).length;
        const badge = pane.querySelector('[data-hp-console-count]');
        const button = pane.querySelector('[data-hp-action="console"]');
        const box = pane.querySelector('[data-hp-console]');
        const list = pane.querySelector('[data-hp-console-list]');
        badge.hidden = count === 0;
        badge.textContent = count > 99 ? '99+' : String(count);
        button.classList.toggle('has-errors', count > 0);
        button.setAttribute('aria-label', tr('codeStudio.previewConsoleCount', 'Console: {{count}} errors', { count }));
        button.setAttribute('aria-expanded', String(preview.consoleOpen));
        box.hidden = !preview.consoleOpen;
        if (!preview.consoleOpen) return;
        list.innerHTML = preview.console.length
            ? preview.console.map(entry => `<li class="cs-hp-console-entry" data-kind="${esc(entry.kind)}"><time>${esc(new Date(entry.at).toLocaleTimeString())}</time><pre>${esc(entry.text)}</pre></li>`).join('')
            : `<li class="cs-hp-console-empty">${esc(tr('codeStudio.previewConsoleEmpty', 'No errors reported by this page.'))}</li>`;
    }

    function togglePreviewConsole(force, returnFocus) {
        const preview = previewState();
        preview.consoleOpen = typeof force === 'boolean' ? force : !preview.consoleOpen;
        renderPreviewConsole();
        const pane = preview.pane;
        if (!pane) return;
        if (preview.consoleOpen) {
            const clear = pane.querySelector('[data-hp-action="clear-console"]');
            if (clear) clear.focus();
        } else if (returnFocus) {
            const button = pane.querySelector('[data-hp-action="console"]');
            if (button) button.focus();
        }
    }

    function setPreviewViewport(viewport) {
        const preview = previewState();
        if (!PREVIEW_VIEWPORTS.includes(viewport) || preview.viewport === viewport) return;
        preview.viewport = viewport;
        savePreviewPrefs();
        updatePreviewChrome();
    }

    function applyPreviewViewport() {
        const preview = state && state.htmlPreview;
        const pane = preview && preview.pane;
        const model = previewModel();
        if (!pane || !pane.isConnected || !model) return;
        const stage = pane.querySelector('[data-hp-stage]');
        const device = pane.querySelector('[data-hp-device]');
        const label = pane.querySelector('[data-hp-device-label]');
        stage.dataset.viewport = preview.viewport;
        const size = model.VIEWPORTS[preview.viewport];
        if (!size || preview.status === 'empty' || preview.status === 'error') {
            label.hidden = true;
            if (!size) ['--cs-hp-device-w', '--cs-hp-device-h', '--cs-hp-scale'].forEach(name => device.style.removeProperty(name));
            return;
        }
        const rect = stage.getBoundingClientRect();
        const scale = model.deviceScale(preview.viewport, rect.width, rect.height, PREVIEW_DEVICE_INSET);
        device.style.setProperty('--cs-hp-device-w', size.width + 'px');
        device.style.setProperty('--cs-hp-device-h', size.height + 'px');
        device.style.setProperty('--cs-hp-scale', String(scale));
        label.hidden = false;
        label.textContent = tr('codeStudio.previewDeviceLabel', '{{width}} × {{height}} at {{scale}}%', { width: size.width, height: size.height, scale: Math.round(scale * 100) });
    }

    function applyPreviewWidth() {
        const root = studioRoot();
        const model = previewModel();
        if (!root || !model) return;
        const preview = previewState();
        const main = root.querySelector('.code-studio-main');
        const pane = previewPane();
        const available = (main ? main.getBoundingClientRect().width : 0) + (pane && !pane.hidden ? pane.getBoundingClientRect().width : 0);
        preview.width = model.clampWidth(preview.width, preview.open ? available : 0);
        root.style.setProperty('--cs-preview-width', preview.width + 'px');
        const handle = pane && pane.querySelector('[data-hp-resize]');
        if (handle) {
            handle.setAttribute('aria-valuemin', '280');
            handle.setAttribute('aria-valuemax', String(model.clampWidth(Number.MAX_SAFE_INTEGER, available)));
            handle.setAttribute('aria-valuenow', String(preview.width));
        }
    }

    function finishPreviewResize() {
        applyPreviewWidth();
        savePreviewPrefs();
        measureVisibleEditors();
        scheduleTerminalFit();
    }

    // The handle sits on the pane's left edge: dragging or ArrowLeft widens the pane, ArrowRight narrows it.
    function wirePreviewResize(pane) {
        const handle = pane.querySelector('[data-hp-resize]');
        let startX = 0;
        let startWidth = 0;
        const onMove = bind(event => {
            previewState().width = startWidth + (startX - event.clientX);
            applyPreviewWidth();
        });
        const onUp = bind(event => {
            handle.classList.remove('dragging');
            pane.classList.remove('resizing');
            try { handle.releasePointerCapture(event.pointerId); } catch (_) {}
            handle.removeEventListener('pointermove', onMove);
            handle.removeEventListener('pointerup', onUp);
            handle.removeEventListener('pointercancel', onUp);
            finishPreviewResize();
        });
        handle.addEventListener('pointerdown', bind(event => {
            if (event.button !== 0) return;
            event.preventDefault();
            startX = event.clientX;
            startWidth = previewState().width;
            handle.classList.add('dragging');
            pane.classList.add('resizing'); // the iframe would swallow pointer events mid-drag
            handle.setPointerCapture(event.pointerId);
            handle.addEventListener('pointermove', onMove);
            handle.addEventListener('pointerup', onUp);
            handle.addEventListener('pointercancel', onUp);
        }));
        handle.addEventListener('keydown', bind(event => {
            const preview = previewState();
            let next = null;
            if (event.key === 'ArrowLeft') next = preview.width + PREVIEW_RESIZE_STEP;
            else if (event.key === 'ArrowRight') next = preview.width - PREVIEW_RESIZE_STEP;
            else if (event.key === 'Home') next = Number.MAX_SAFE_INTEGER;
            else if (event.key === 'End') next = 0;
            if (next === null) return;
            event.preventDefault();
            preview.width = next;
            finishPreviewResize();
        }));
        handle.addEventListener('dblclick', bind(() => {
            previewState().width = PREVIEW_DEFAULT_WIDTH;
            finishPreviewResize();
        }));
    }

    function openPreviewInBrowserTab(path) {
        const target = state;
        const preview = previewState();
        const model = previewModel();
        if (!model) return;
        const vpath = path ? normalizeCodeStudioPath(path) : preview.path;
        if (!vpath || vpath === WORKSPACE_ROOT && !preview.directory) return;
        const current = vpath === preview.path;
        const urlFor = base => model.buildUrl(base, vpath, current && preview.directory, current ? preview.suffix : '');
        if (preview.base && !model.tokenNeedsRefresh(preview.expiresAt, Date.now())) {
            window.open(urlFor(preview.base), '_blank', 'noopener');
            return;
        }
        // No usable token yet: open the tab inside the click (popup blockers), cut its opener, then send it on.
        const win = window.open('about:blank', '_blank');
        if (win) {
            try { win.opener = null; } catch (_) {}
        }
        ensurePreviewToken(target, false, vpath).then(base => {
            if (win && !win.closed) win.location.replace(urlFor(base));
        }).catch(err => {
            if (win && !win.closed) win.close();
            if (isLiveInstance(target)) runWithInstance(target, () => showOperationError(err));
        });
    }

    function previewFollowActiveTab() {
        const preview = state && state.htmlPreview;
        if (!preview || !preview.open || preview.pinned) return;
        const tab = activeTab();
        if (!isSourceTab(tab) || !isPreviewablePath(tab.path) || tab.path === preview.followedPath) return;
        openPreview(tab.path);
    }

    // Hook at the end of renderBreadcrumbs: the eye button in the editor title area, follow mode, dirty hint.
    function previewAfterBreadcrumbs() {
        const root = studioRoot();
        const bar = root && root.querySelector('[data-breadcrumbs]');
        if (!bar) return;
        bar.querySelectorAll('[data-hp-eye]').forEach(element => element.remove());
        const tab = activeTab();
        if (isSourceTab(tab) && isPreviewablePath(tab.path)) {
            const label = tr('codeStudio.previewEye', 'Open preview to the side') + ' (Ctrl+Shift+V)';
            const button = document.createElement('button');
            button.type = 'button';
            button.className = 'cs-hp-eye';
            button.dataset.hpEye = '';
            button.setAttribute('aria-label', label);
            button.title = label;
            button.innerHTML = previewGlyph('eye', 15);
            button.addEventListener('click', bind(() => {
                const current = activeTab();
                if (!isSourceTab(current)) return;
                const preview = previewState();
                if (preview.open && preview.path === current.path) closePreviewPane();
                else openPreview(current.path);
            }));
            bar.appendChild(button);
        }
        syncPreviewEyeButton();
        previewFollowActiveTab();
        syncPreviewDirtyHint();
    }

    // Hook at the end of renderTabs: a tab turned dirty or clean.
    function previewAfterTabs() {
        syncPreviewDirtyHint();
        syncPreviewEyeButton();
    }

    function syncPreviewDirtyHint() {
        const preview = state && state.htmlPreview;
        const hint = preview && preview.pane && preview.pane.querySelector('[data-hp-dirty]');
        if (!hint) return;
        const tab = state.openTabs.find(item => !isVirtualTab(item) && item.path === preview.path);
        hint.hidden = !(preview.open && tab && tab.modified);
    }

    function syncPreviewEyeButton() {
        const root = studioRoot();
        const eye = root && root.querySelector('[data-hp-eye]');
        if (!eye) return;
        const preview = state.htmlPreview;
        const tab = activeTab();
        eye.setAttribute('aria-pressed', String(!!(preview && preview.open && tab && preview.path === tab.path)));
    }

    function previewOpenToSideCommand() {
        const tab = activeTab();
        if (isSourceTab(tab) && isPreviewablePath(tab.path)) openPreview(tab.path);
        else openPreviewForActive();
    }

    function previewOpenInBrowserTabCommand() {
        const tab = activeTab();
        if (isSourceTab(tab) && isPreviewablePath(tab.path)) openPreviewInBrowserTab(tab.path);
    }
