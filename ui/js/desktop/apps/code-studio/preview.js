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
        const t = (key, fallback) => esc(tr(key, fallback));
        const viewportLabels = {
            responsive: tr('codeStudio.previewResponsive', 'Responsive'),
            tablet: tr('codeStudio.previewTablet', 'Tablet (768 px)'),
            mobile: tr('codeStudio.previewMobile', 'Mobile (375 px)')
        };
        pane.setAttribute('aria-label', tr('codeStudio.previewTitle', 'Preview'));
        pane.innerHTML = `
            <div class="cs-hp-resize" data-hp-resize role="separator" aria-orientation="vertical" tabindex="0" aria-label="${t('codeStudio.previewResize', 'Resize preview')}" title="${t('codeStudio.previewResize', 'Resize preview')}"></div>
            <div class="cs-hp-toolbar" role="toolbar" aria-label="${t('codeStudio.previewTitle', 'Preview')}">
                <button type="button" class="cs-hp-button" data-hp-action="reload" aria-label="${t('codeStudio.previewReload', 'Reload preview')}" title="${t('codeStudio.previewReload', 'Reload preview')}">${previewGlyph('reload')}</button>
                <form class="cs-hp-address" data-hp-address-form>
                    <span class="cs-hp-address-icon" aria-hidden="true">${previewGlyph('globe', 13)}</span>
                    <input type="text" class="cs-hp-address-input" data-hp-address spellcheck="false" autocomplete="off" placeholder="index.html" aria-label="${t('codeStudio.previewAddress', 'Preview address (path inside the workspace)')}">
                </form>
                <div class="cs-hp-segmented" role="radiogroup" data-hp-viewports aria-label="${t('codeStudio.previewViewport', 'Viewport')}">
                    ${PREVIEW_VIEWPORTS.map(id => `<button type="button" role="radio" class="cs-hp-segment" data-hp-viewport="${id}" aria-checked="false" tabindex="-1" aria-label="${esc(viewportLabels[id])}" title="${esc(viewportLabels[id])}">${previewGlyph(id)}</button>`).join('')}
                </div>
                <button type="button" class="cs-hp-button cs-hp-console-button" data-hp-action="console" aria-expanded="false" aria-controls="${consoleId}" title="${t('codeStudio.previewConsole', 'Console')}">${previewGlyph('alert')}<span class="cs-hp-badge" data-hp-console-count hidden>0</span></button>
                <button type="button" class="cs-hp-button" data-hp-action="pin" aria-pressed="false" aria-label="${t('codeStudio.previewPin', 'Keep showing this file')}">${previewGlyph('pin')}</button>
                <button type="button" class="cs-hp-button" data-hp-action="external" aria-label="${t('codeStudio.previewOpenExternal', 'Open in new browser tab')}" title="${t('codeStudio.previewOpenExternal', 'Open in new browser tab')}">${previewGlyph('external')}</button>
                <button type="button" class="cs-hp-button" data-hp-action="close" aria-label="${t('codeStudio.previewClose', 'Close preview')}" title="${t('codeStudio.previewClose', 'Close preview')}">${previewGlyph('close')}</button>
            </div>
            <div class="cs-hp-hint" data-hp-dirty role="status" hidden><span class="cs-hp-hint-dot" aria-hidden="true"></span><span>${t('codeStudio.previewDirty', 'Showing saved version. Save to update the preview.')}</span></div>
            <section class="cs-hp-console" id="${consoleId}" data-hp-console hidden aria-label="${t('codeStudio.previewConsole', 'Console')}">
                <header class="cs-hp-console-head"><span>${t('codeStudio.previewConsole', 'Console')}</span><button type="button" class="cs-hp-text-button" data-hp-action="clear-console">${t('codeStudio.previewConsoleClear', 'Clear console')}</button></header>
                <ol class="cs-hp-console-list" data-hp-console-list></ol>
            </section>
            <div class="cs-hp-stage" data-hp-stage data-viewport="${esc(preview.viewport)}" aria-busy="false">
                <div class="cs-hp-device" data-hp-device>
                    <iframe class="cs-hp-frame" data-hp-frame sandbox="${PREVIEW_SANDBOX}" referrerpolicy="no-referrer" title="${t('codeStudio.previewTitle', 'Preview')}"></iframe>
                </div>
                <div class="cs-hp-device-label" data-hp-device-label hidden></div>
                <div class="cs-hp-loading" data-hp-loading hidden>
                    <span class="cs-hp-shimmer"></span><span class="cs-hp-shimmer short"></span><span class="cs-hp-shimmer"></span><span class="cs-hp-shimmer short"></span>
                    <span class="cs-hp-sr">${t('codeStudio.previewLoading', 'Loading preview...')}</span>
                </div>
                <div class="cs-hp-card" data-hp-empty hidden>
                    <span class="cs-hp-card-icon" aria-hidden="true">${previewGlyph('eye', 26)}</span>
                    <strong>${t('codeStudio.previewEmptyTitle', 'Open an HTML file to preview it')}</strong>
                    <p>${t('codeStudio.previewEmptyHint', 'HTML, HTM and SVG files open here and reload when you save.')}</p>
                    <div class="cs-hp-card-actions"><button type="button" class="cs-button primary" data-hp-action="preview-active">${t('codeStudio.previewEmptyAction', 'Preview active file')}</button></div>
                </div>
                <div class="cs-hp-card cs-hp-error" data-hp-error role="alert" hidden>
                    <span class="cs-hp-card-icon" aria-hidden="true">${previewGlyph('alert', 24)}</span>
                    <strong data-hp-error-title></strong>
                    <p data-hp-error-detail></p>
                    <div class="cs-hp-card-actions">
                        <button type="button" class="cs-button primary" data-hp-action="retry">${t('codeStudio.previewRetry', 'Try again')}</button>
                        <button type="button" class="cs-button" data-hp-action="close">${t('codeStudio.previewClose', 'Close preview')}</button>
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
