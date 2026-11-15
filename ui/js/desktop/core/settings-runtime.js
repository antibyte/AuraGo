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
        const write = async () => {
            const result = await api('/api/desktop/settings', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ settings: updates })
            });
            if (!state.bootstrap) state.bootstrap = {};
            state.bootstrap.settings = result.settings || Object.assign({}, desktopSettings(), updates);
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
        };
        // A failed save is reported to its caller, but must not poison later saves.
        const pending = desktopPreferenceQueue.then(write);
        desktopPreferenceQueue = pending.catch(() => {});
        return pending;
    }
