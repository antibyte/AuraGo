    let desktopPreferenceQueue = Promise.resolve();

    function desktopSettingsContext() {
        return { state, settingValue, settingBool, desktopSettings, t, currentDesktopTheme, desktopThemes,
            iconThemeManifests: state.iconThemeManifests };
    }
    window.AuraDesktopSettingsContext = desktopSettingsContext;

    function desktopPreferenceUpdates(key, value) {
        const updates = { [key]: String(value) };
        if (key === 'appearance.theme') {
            const theme = desktopThemeById(value);
            if (theme) Object.assign(updates, {
                'appearance.icon_theme': theme.iconTheme,
                'appearance.wallpaper': theme.wallpaper,
                'appearance.wallpaper_by_space': '{}',
                'appearance.theme_cursors': 'true',
                'sound.theme': theme.soundTheme
            });
        }
        return updates;
    }

    function saveDesktopPreference(key, value) {
        if (desktopReadonly()) return Promise.reject(new Error(t('desktop.action_unavailable')));
        const updates = desktopPreferenceUpdates(key, value);
        return writeDesktopSettings(updates).then(async result => {
            applyDesktopSettings();
            Object.entries(updates).forEach(([setting, stored]) => applySoundSettingsChange(setting, stored));
            if (key === 'windows.restore_session' && String(value) === 'true') await persistSessionSnapshot();
            if (key !== 'sound.volume') {
                renderStartButtonIcon();
                renderIcons();
                renderWidgets();
                renderStartApps();
            }
            window.dispatchEvent(new CustomEvent('aurago:desktop-settings', { detail: { keys: Object.keys(updates) } }));
            return result;
        });
    }

    const WORKSPACE_PROTECTED_SETTINGS = ['session.windows', 'appearance.wallpaper_by_space', 'appearance.wallpaper'];
    let desktopWorkspaceUIReady = false;
    let desktopWorkspacePresentation = '';

    function desktopWorkspaceRevision() {
        return window.AuraDesktopWorkspaceManager?.snapshot().layout.revision || 1;
    }

    function applyDesktopWorkspaceState(snapshot) {
        if (!snapshot?.layout) return;
        if (!state.bootstrap) state.bootstrap = {};
        state.bootstrap.settings = Object.assign({}, state.bootstrap.settings, snapshot.settings,
            { 'desktop.workspaces.layout': JSON.stringify(snapshot.layout) });
        const manager = window.AuraDesktopWorkspaceManager;
        state.activeSpaceId = manager.resolveId(state.activeSpaceId || '1');
        state.windows.forEach(win => {
            if (win.isGadget) return;
            assignWindowSpace(win, manager.resolveId(win.spaceId || win.element?.dataset.spaceId || '1'));
        });
        const presentation = JSON.stringify([snapshot.layout.workspaces, snapshot.layout.removed_to,
            snapshot.settings?.['appearance.wallpaper'], snapshot.settings?.['appearance.wallpaper_by_space']]);
        const changed = presentation !== desktopWorkspacePresentation;
        desktopWorkspacePresentation = presentation;
        if (!desktopWorkspaceUIReady || !changed) return;
        applyActiveSpaceWallpaper();
        applySpaceVisibility();
        renderSpacePager();
        renderTaskbar();
        if (isSpacesOverviewOpen()) renderSpacesOverview();
        window.dispatchEvent(new CustomEvent('aurago:desktop-settings', { detail: { keys: ['desktop.workspaces.layout'] } }));
    }

    function initDesktopWorkspaceSettings() {
        window.AuraDesktopWorkspaceManager?.configure({ api, readonly: desktopReadonly, t, mountDialog: mountDesktopDialog,
            onChange: applyDesktopWorkspaceState,
            onConflict: () => showDesktopNotification({ title: t('desktop.notification'), message: t('desktop.workspaces_conflict') }) });
    }

    function acceptDesktopBootstrap(boot) {
        const manager = window.AuraDesktopWorkspaceManager;
        const previous = manager?.snapshot();
        let layout;
        try { layout = JSON.parse(boot?.settings?.['desktop.workspaces.layout'] || 'null'); } catch (_) { /* Defaults cover legacy sessions. */ }
        state.bootstrap = boot;
        if (!manager) return;
        if (!layout && previous?.layout.revision === 1) layout = previous.layout;
        if (layout && layout.revision >= (previous?.layout.revision || 1)) {
            const settings = {};
            WORKSPACE_PROTECTED_SETTINGS.forEach(key => { settings[key] = boot.settings[key] ?? desktopSettingDefaults[key]; });
            manager.adopt({ layout, settings });
        } else if (previous) {
            applyDesktopWorkspaceState(previous);
        }
    }

    function queueDesktopSettingsWrite(producer, keepalive = false) {
        const pending = desktopPreferenceQueue.then(async () => {
            if (desktopReadonly() || desktopSessionEnded) throw new Error(t('desktop.action_unavailable'));
            const body = producer();
            let result;
            try {
                result = await api('/api/desktop/settings', { method: 'PUT',
                    headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body), keepalive });
            } catch (err) {
                if (err.body?.code === 'workspace_revision_conflict' || err.body?.code === 'workspace_revision_required') {
                    if (err.body.state) window.AuraDesktopWorkspaceManager?.adopt(err.body.state);
                    err.message = t('desktop.workspaces_conflict');
                }
                throw err;
            }
            if (!state.bootstrap) state.bootstrap = {};
            if (!state.bootstrap.settings) state.bootstrap.settings = {};
            for (const [key, value] of Object.entries(body.settings)) {
                if (!result.workspace || !WORKSPACE_PROTECTED_SETTINGS.includes(key)) {
                    state.bootstrap.settings[key] = result.settings?.[key] ?? value;
                }
            }
            if (result.workspace) window.AuraDesktopWorkspaceManager?.adopt(result.workspace);
            return result;
        });
        desktopPreferenceQueue = pending.catch(() => {});
        return pending;
    }

    function reconcileDesktopSettingsResponse(result) {
        const manager = window.AuraDesktopWorkspaceManager;
        if (!manager || !result?.settings) return;
        let snapshot = result.workspace;
        try {
            const layout = JSON.parse(result.settings['desktop.workspaces.layout'] || 'null');
            if (layout && layout.revision > (snapshot?.layout.revision || 0)) {
                const settings = {};
                WORKSPACE_PROTECTED_SETTINGS.forEach(key => { settings[key] = result.settings[key] ?? desktopSettingDefaults[key]; });
                snapshot = { layout, settings };
            }
        } catch (_) { /* A malformed or obsolete response cannot replace the accepted layout. */ }
        if (!snapshot && !Object.hasOwn(result.settings, 'desktop.workspaces.layout')) return;
        if (snapshot) manager.adopt(snapshot);
        const latest = manager.snapshot();
        // Pet and phone controls still consume the full settings response.
        Object.assign(result.settings, latest.settings, { 'desktop.workspaces.layout': JSON.stringify(latest.layout) });
    }

    function writeDesktopSettings(settings, keepalive = false) {
        const protectedKeys = WORKSPACE_PROTECTED_SETTINGS.filter(key => Object.hasOwn(settings, key));
        const identity = () => {
            const layout = window.AuraDesktopWorkspaceManager?.snapshot().layout;
            return JSON.stringify(layout && [layout.workspaces, layout.removed_to, layout.next_id]);
        };
        const expectedLayout = identity();
        const baseline = protectedKeys.map(key => settingValue(key));
        return queueDesktopSettingsWrite(() => {
            // A queued session save may advance the revision without changing this command's inputs.
            if (protectedKeys.length && (identity() !== expectedLayout || protectedKeys.some((key, index) => settingValue(key) !== baseline[index]))) {
                throw new Error(t('desktop.workspaces_conflict'));
            }
            return { settings, ...(protectedKeys.length ? { revision: desktopWorkspaceRevision() } : {}) };
        }, keepalive);
    }

    let desktopCustomWallpaperKey = '';
    function syncDesktopCustomWallpaper() {
        const body = document.body;
        const runtime = window.AuraDesktopCustomWallpaper;
        if (!body || !runtime) return;
        const value = body.dataset.wallpaper;
        const fit = settingValue('appearance.wallpaper_fit') || 'cover';
        const key = JSON.stringify([value, fit]);
        if (key === desktopCustomWallpaperKey) return;
        desktopCustomWallpaperKey = key;
        if (!runtime.isCustom(value)) {
            runtime.release(body);
            updateDesktopWallpaperStatus('', 'ready');
            return;
        }
        void runtime.apply(body, value, fit, { isCurrent: candidate => body.dataset.wallpaper === candidate,
            onStatus: updateDesktopWallpaperStatus });
    }

    function updateDesktopWallpaperStatus(path, status) {
        let notice = document.getElementById('vd-wallpaper-status');
        if (!notice) {
            notice = document.createElement('div');
            notice.id = 'vd-wallpaper-status';
            notice.className = 'vd-wallpaper-status';
            notice.setAttribute('role', 'status');
            document.getElementById('vd-workspace')?.appendChild(notice);
        }
        const failed = ['missing', 'invalid', 'error'].includes(status);
        const key = status === 'missing' ? 'desktop.wallpaper_missing' : status === 'invalid' ? 'desktop.wallpaper_invalid' : 'desktop.wallpaper_load_failed';
        notice.hidden = !failed;
        notice.textContent = failed ? t(key, { path }) : '';
    }

    async function chooseDesktopWallpaper() {
        if (desktopReadonly()) return false;
        const choice = await openDesktopFileDialog({ title: t('desktop.wallpaper_choose'), initialPath: 'Pictures',
            filters: [{ label: 'PNG, JPEG, WebP, BMP', extensions: ['.png', '.jpg', '.jpeg', '.webp', '.bmp'] }] });
        if (choice.canceled || !choice.path) return false;
        return saveDesktopWallpaper('file:' + choice.path);
    }
