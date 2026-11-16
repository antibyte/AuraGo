    let quickSettingsPanel = null;
    let quickSettingsOpen = false;
    let quickSettingsAnchor = null;
    let quickSettingsUnsubscribePolicy = null;
    let quickSettingsUnsubscribeMedia = null;
    let quickSettingsKeyHandler = null;
    let quickSettingsPreferencePending = new Set();
    let quickSettingsMedia = null;
    let quickSettingsMediaPending = false;

    function quickSettingsSchemeKey() {
        const key = currentDesktopTheme()?.schemeSetting || '';
        return ['appearance.fruity_mode', 'appearance.minimal_mode'].includes(key) ? key : '';
    }

    function quickSettingsSchemeLabelKey(key) {
        return key === 'appearance.minimal_mode' ? 'desktop.settings_minimal_mode' : 'desktop.settings_fruity_mode';
    }

    function ensureQuickSettingsPanel() {
        if (quickSettingsPanel) return quickSettingsPanel;
        const panel = document.createElement('section');
        panel.id = 'vd-quick-settings';
        panel.className = 'vd-tray-popover vd-quick-settings';
        panel.hidden = true;
        panel.tabIndex = -1;
        panel.setAttribute('role', 'dialog');
        panel.setAttribute('aria-modal', 'false');
        panel.setAttribute('aria-labelledby', 'vd-quick-settings-title');
        document.body.appendChild(panel);
        panel.addEventListener('click', onQuickSettingsClick);
        panel.addEventListener('input', onQuickSettingsInput);
        panel.addEventListener('change', onQuickSettingsChange);
        quickSettingsPanel = panel;
        return panel;
    }

    function quickSettingsMediaHTML() {
        if (!window.AuraDesktopMediaSession || typeof window.AuraDesktopMediaSession.snapshot !== 'function') return '';
        return `<section class="vd-quick-settings-group vd-quick-media" aria-labelledby="vd-quick-media-title">
            <h3 id="vd-quick-media-title">${esc(t('desktop.category_media'))}</h3>
            <div class="vd-quick-media-content" data-quick-media-content tabindex="-1"></div>
            <p class="vd-quick-settings-status" data-quick-media-status role="status" aria-live="polite" hidden></p>
        </section>`;
    }

    function quickSettingsMarkup() {
        return `<header class="vd-quick-settings-header">
                <h2 id="vd-quick-settings-title">${esc(t('desktop.quick_settings_title'))}</h2>
                <button type="button" class="vd-quick-settings-close" data-quick-action="close" aria-label="${esc(t('desktop.close'))}">×</button>
            </header>
            <section class="vd-quick-settings-group">
                <h3>${esc(t('desktop.quick_settings_sound'))}</h3>
                <label class="vd-quick-settings-toggle"><span>${esc(t('desktop.settings_sound_enabled'))}</span><input type="checkbox" data-quick-setting="sound.enabled"></label>
                <label class="vd-quick-settings-control"><span>${esc(t('desktop.settings_sound_volume'))}</span><span class="vd-quick-settings-range"><input type="range" min="0" max="100" step="5" data-quick-setting="sound.volume"><output data-quick-volume-output></output></span></label>
            </section>
            <section class="vd-quick-settings-group">
                <h3>${esc(t('desktop.quick_settings_display'))}</h3>
                <label class="vd-quick-settings-toggle"><span>${esc(t('desktop.settings_window_animations'))}</span><input type="checkbox" data-quick-setting="windows.animations"></label>
                <div data-quick-scheme-slot></div>
            </section>
            <section class="vd-quick-settings-group vd-quick-dnd">
                <label class="vd-quick-settings-toggle"><span><strong>${esc(t('desktop.notifications_dnd'))}</strong><small>${esc(t('desktop.quick_settings_dnd_desc'))}</small></span><input type="checkbox" data-quick-dnd></label>
            </section>
            ${quickSettingsMediaHTML()}
            <p class="vd-quick-settings-status" data-quick-status role="status" aria-live="polite" hidden></p>
            <p class="vd-quick-settings-readonly" data-quick-readonly hidden>${esc(t('desktop.quick_settings_readonly'))}</p>`;
    }

    function quickSettingsSchemeMarkup(key) {
        if (!key) return '';
        const labelKey = quickSettingsSchemeLabelKey(key);
        return `<label class="vd-quick-settings-control vd-quick-scheme">
            <span>${esc(t(labelKey))}</span>
            <select data-quick-setting="${esc(key)}" aria-label="${esc(t(labelKey))}">
                <option value="light">${esc(t('desktop.settings_fruity_mode_light'))}</option>
                <option value="dark">${esc(t('desktop.settings_fruity_mode_dark'))}</option>
            </select>
        </label>`;
    }

    function syncQuickSettingsScheme() {
        const slot = quickSettingsPanel?.querySelector('[data-quick-scheme-slot]');
        if (!slot) return;
        const key = quickSettingsSchemeKey();
        if (slot.dataset.schemeKey === key) return;
        const focusedScheme = !!(slot.contains && slot.contains(document.activeElement));
        slot.dataset.schemeKey = key;
        slot.innerHTML = quickSettingsSchemeMarkup(key);
        if (focusedScheme) {
            const replacement = key ? slot.querySelector(`[data-quick-setting="${key}"]`) : quickSettingsPanel.querySelector('[data-quick-setting="windows.animations"]');
            replacement?.focus();
        }
    }

    function quickMediaStatusText(status) {
        const key = `desktop.quick_settings_media_status_${String(status || 'idle')}`;
        const text = t(key);
        return text && text !== key ? text : String(status || 'idle');
    }

    function renderQuickMedia() {
        if (!quickSettingsPanel) return;
        const host = quickSettingsPanel.querySelector('[data-quick-media-content]');
        if (!host) return;
        const active = document.activeElement;
        const hostOwnedFocus = !!(host.contains && host.contains(active));
        const focusedAction = hostOwnedFocus ? active.dataset?.mediaAction || '' : '';
        const focusedToggle = hostOwnedFocus && active.dataset?.mediaToggle === 'true';
        const media = quickSettingsMedia;
        if (!media || !media.available) {
            host.innerHTML = `<p class="vd-quick-media-empty">${esc(t('desktop.quick_settings_media_empty'))}</p>`;
            if (hostOwnedFocus) host.focus();
            return;
        }
        const metadata = media.metadata || {};
        const title = String(metadata.title || '').trim();
        const artist = String(metadata.artist || '').trim();
        const status = String(media.status || media.playbackState || 'idle');
        const actions = Array.isArray(media.actions) ? media.actions : [];
        const controls = [];
        if (actions.includes('previoustrack')) controls.push(['previoustrack', '⏮', 'desktop.audio_player_previous']);
        if (status === 'playing' && actions.includes('pause')) controls.push(['pause', 'Ⅱ', 'desktop.audio_player_pause', true]);
        else if (status !== 'playing' && actions.includes('play')) controls.push(['play', '▶', 'desktop.audio_player_play', true]);
        if (actions.includes('nexttrack')) controls.push(['nexttrack', '⏭', 'desktop.audio_player_next']);
        host.innerHTML = `<div class="vd-quick-media-track"><strong>${esc(title || t('desktop.quick_settings_media_empty'))}</strong>${artist ? `<span>${esc(artist)}</span>` : ''}<small>${esc(quickMediaStatusText(status))}</small></div>
            ${controls.length ? `<div class="vd-quick-media-controls" role="group" aria-label="${esc(t('desktop.category_media'))}" aria-busy="${quickSettingsMediaPending ? 'true' : 'false'}" tabindex="-1">${controls.map(([action, glyph, label, toggle]) => `<button type="button" data-media-action="${action}" ${toggle ? 'data-media-toggle="true"' : ''} aria-disabled="${quickSettingsMediaPending ? 'true' : 'false'}" aria-label="${esc(t(label))}" title="${esc(t(label))}">${glyph}</button>`).join('')}</div>` : ''}`;
        if (hostOwnedFocus) {
            const replacement = focusedToggle
                ? host.querySelector('[data-media-toggle="true"]')
                : focusedAction ? host.querySelector(`[data-media-action="${focusedAction}"]`) : null;
            (replacement || host.querySelector('[data-media-action]') || host.querySelector('.vd-quick-media-controls') || host).focus();
        }
    }

    function syncQuickSettings() {
        const panel = quickSettingsPanel;
        if (!panel) return;
        syncQuickSettingsScheme();
        const readonly = desktopReadonly();
        panel.querySelectorAll('[data-quick-setting]').forEach(control => {
            const key = control.dataset.quickSetting;
            if (control.type === 'checkbox') control.checked = settingBool(key);
            else if (control.type === 'range') control.value = String(Math.round(Math.min(1, Math.max(0, Number(settingValue(key) || 0))) * 100));
            else control.value = String(settingValue(key) || 'light');
            control.disabled = readonly || quickSettingsPreferencePending.has(key);
        });
        const volume = panel.querySelector('[data-quick-volume-output]');
        if (volume) volume.value = `${panel.querySelector('[data-quick-setting="sound.volume"]')?.value || 0}%`;
        const dnd = panel.querySelector('[data-quick-dnd]');
        if (dnd) dnd.checked = !!window.AuraDesktopNotificationCenter?.policy?.().dnd;
        const readonlyHint = panel.querySelector('[data-quick-readonly]');
        if (readonlyHint) readonlyHint.hidden = !readonly;
        renderQuickMedia();
    }

    function quickSettingsStatus(message, error) {
        if (!quickSettingsPanel) return;
        const status = quickSettingsPanel.querySelector('[data-quick-status]');
        if (!status) return;
        status.textContent = message || '';
        status.hidden = !message;
        status.classList.toggle('is-error', !!error);
    }

    function quickMediaStatus(message, error) {
        const status = quickSettingsPanel?.querySelector('[data-quick-media-status]');
        if (!status) return;
        status.textContent = message || '';
        status.hidden = !message;
        status.classList.toggle('is-error', !!error);
    }

    function quickSettingsSettingAllowed(key) {
        return ['sound.enabled', 'sound.volume', 'windows.animations'].includes(key) || key === quickSettingsSchemeKey();
    }

    async function saveQuickSetting(key, value) {
        if (!quickSettingsSettingAllowed(key)) return;
        if (key === 'sound.volume') {
            const volume = Number(value);
            if (!Number.isFinite(volume)) return;
            value = String(Math.min(1, Math.max(0, volume)));
        } else if (key === quickSettingsSchemeKey()) {
            if (!['light', 'dark'].includes(value)) return;
        } else if (!['true', 'false'].includes(value)) {
            return;
        }
        if (desktopReadonly()) {
            quickSettingsStatus(t('desktop.quick_settings_readonly'), true);
            return;
        }
        quickSettingsPreferencePending.add(key);
        const control = quickSettingsPanel?.querySelector(`[data-quick-setting="${key}"]`);
        if (control) control.disabled = true;
        quickSettingsStatus(t('desktop.saving'));
        try {
            await saveDesktopPreference(key, value);
            quickSettingsStatus(t('desktop.saved'));
        } catch (_) {
            quickSettingsStatus(t('desktop.quick_settings_save_failed'), true);
        } finally {
            quickSettingsPreferencePending.delete(key);
            syncQuickSettings();
        }
    }

    async function dispatchQuickMedia(action) {
        const api = window.AuraDesktopMediaSession;
        if (!api || !quickSettingsMedia?.available || !quickSettingsMedia.actions?.includes(action) || quickSettingsMediaPending) return;
        quickSettingsMediaPending = true;
        renderQuickMedia();
        quickMediaStatus(t('desktop.loading'));
        try {
            const handled = await api.dispatch(action);
            if (!handled) quickMediaStatus(t('desktop.quick_settings_media_action_failed'), true);
            else quickMediaStatus('');
        } catch (_) {
            quickMediaStatus(t('desktop.quick_settings_media_action_failed'), true);
        } finally {
            quickSettingsMediaPending = false;
            renderQuickMedia();
        }
    }

    function onQuickSettingsClick(event) {
        const target = event.target?.closest?.('[data-quick-action], [data-media-action]');
        if (!target) return;
        if (target.dataset.quickAction === 'close') {
            closeQuickSettingsPopup(true);
            return;
        }
        if (target.dataset.mediaAction) dispatchQuickMedia(target.dataset.mediaAction);
    }

    function onQuickSettingsInput(event) {
        const control = event.target;
        if (control?.dataset?.quickSetting === 'sound.volume') {
            const output = quickSettingsPanel?.querySelector('[data-quick-volume-output]');
            if (output) output.value = `${control.value}%`;
        }
    }

    function onQuickSettingsChange(event) {
        const control = event.target;
        const key = control?.dataset?.quickSetting;
        if (key) {
            const value = control.type === 'checkbox' ? String(!!control.checked) : key === 'sound.volume' ? String(Math.min(1, Math.max(0, Number(control.value) / 100))) : String(control.value);
            saveQuickSetting(key, value);
            return;
        }
        if (control?.matches?.('[data-quick-dnd]')) {
            const center = window.AuraDesktopNotificationCenter;
            if (center && typeof center.setPolicy === 'function') center.setPolicy({ dnd: !!control.checked });
        }
    }

    function closeQuickSettingsPopup(restoreFocus = false) {
        if (!quickSettingsPanel || !quickSettingsOpen) return;
        quickSettingsPanel.hidden = true;
        quickSettingsOpen = false;
        quickSettingsAnchor?.setAttribute('aria-expanded', 'false');
        quickSettingsUnsubscribePolicy?.();
        quickSettingsUnsubscribePolicy = null;
        quickSettingsUnsubscribeMedia?.();
        quickSettingsUnsubscribeMedia = null;
        window.removeEventListener('aurago:desktop-settings', syncQuickSettings);
        document.removeEventListener('aurago:desktop-policy', syncQuickSettings);
        if (quickSettingsKeyHandler) document.removeEventListener('keydown', quickSettingsKeyHandler, true);
        quickSettingsKeyHandler = null;
        if (restoreFocus && quickSettingsAnchor?.isConnected) quickSettingsAnchor.focus();
        quickSettingsAnchor = null;
    }

    function toggleQuickSettingsPopup(anchor) {
        const panel = ensureQuickSettingsPanel();
        if (quickSettingsOpen) {
            closeQuickSettingsPopup();
            return;
        }
        quickSettingsOpen = true;
        quickSettingsAnchor = anchor || document.activeElement;
        panel.innerHTML = quickSettingsMarkup();
        panel.hidden = false;
        panel.setAttribute('aria-label', t('desktop.quick_settings_title'));
        quickSettingsAnchor?.setAttribute('aria-expanded', 'true');
        placeTrayPopover(panel, anchor || quickSettingsAnchor);
        syncQuickSettings();
        window.addEventListener('aurago:desktop-settings', syncQuickSettings);
        document.addEventListener('aurago:desktop-policy', syncQuickSettings);
        const center = window.AuraDesktopNotificationCenter;
        if (center && typeof center.subscribePolicy === 'function') quickSettingsUnsubscribePolicy = center.subscribePolicy(syncQuickSettings);
        const media = window.AuraDesktopMediaSession;
        quickSettingsMedia = null;
        if (media && typeof media.subscribe === 'function') quickSettingsUnsubscribeMedia = media.subscribe(snapshot => { quickSettingsMedia = snapshot; renderQuickMedia(); });
        else if (media && typeof media.snapshot === 'function') { quickSettingsMedia = media.snapshot(); renderQuickMedia(); }
        quickSettingsKeyHandler = event => {
            if (event.key !== 'Escape') return;
            event.preventDefault();
            event.stopPropagation();
            closeQuickSettingsPopup(true);
        };
        document.addEventListener('keydown', quickSettingsKeyHandler, true);
        (panel.querySelector('button:not(:disabled), input:not(:disabled), select:not(:disabled)') || panel).focus();
    }
