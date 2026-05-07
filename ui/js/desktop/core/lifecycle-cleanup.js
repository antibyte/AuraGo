    function callAppDispose(app, windowId) {
        if (!app || typeof app.dispose !== 'function') return false;
        try {
            app.dispose(windowId);
            return true;
        } catch (err) {
            console.warn('Desktop app dispose failed', err);
            return false;
        }
    }

    function disposeAppWindow(win) {
        if (!win) return;
        const cleanup = state.windowCleanups.get(win.id);
        if (cleanup) {
            state.windowCleanups.delete(win.id);
            cleanup.forEach(fn => {
                try { fn(); } catch (err) { console.warn('Desktop window cleanup failed', err); }
            });
        }
        if (win.appId === 'music-player') disposeWebampMusic(win.id);
        if (win.appId === 'radio') callAppDispose(window.RadioApp, win.id);
        if (win.appId === 'system-info') callAppDispose(window.SystemInfoApp, win.id);
        const disposeName = appGlobalName(win.appId);
        const fallbackName = appGlobalFallbackName(win.appId);
        const disposed = callAppDispose(disposeName ? window[disposeName] : null, win.id);
        if (!disposed && fallbackName) callAppDispose(window[fallbackName], win.id);
    }

    function registerWidgetCleanup(cleanup) {
        if (typeof cleanup !== 'function') return;
        if (!state.widgetCleanups) state.widgetCleanups = [];
        state.widgetCleanups.push(cleanup);
    }

    function clearWidgetRuntime() {
        const cleanups = state.widgetCleanups || [];
        state.widgetCleanups = [];
        cleanups.forEach(cleanup => {
            try { cleanup(); } catch (err) { console.warn('Desktop widget cleanup failed', err); }
        });
    }

    function renderAppError(id, appId, err) {
        console.error('Desktop app render failed', { appId, windowId: id, error: err });
        const host = contentEl(id);
        if (!host) return;
        host.innerHTML = `<div class="vd-app-error">
            <div class="vd-app-error-title">${esc(t('desktop.app_error_title', 'App failed to load'))}</div>
            <div class="vd-app-error-message">${esc((err && err.message) || String(err || 'Error'))}</div>
        </div>`;
    }
