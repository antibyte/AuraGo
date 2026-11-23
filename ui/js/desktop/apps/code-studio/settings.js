    // IDE package 1A: editor settings (A5). One settings object lives in localStorage and is shared by every Code
    // Studio window of the page. The defaults are the editor's behaviour before the settings existed: 4 spaces, word
    // wrap on, everything else off. Every CodeMirror view keeps the indentation, tab size and word wrap extensions in
    // a compartment of its own (view.csSettings), so a change reaches the open editors live.
    const EDITOR_SETTINGS_KEY = 'aurago.codeStudio.settings.v1';
    const EDITOR_TAB_SIZES = [2, 4, 8];
    const AUTO_SAVE_DELAY_MS = 1000;
    let editorSettingsCache = null;

    function normalizeEditorSettings(raw) {
        const value = raw && typeof raw === 'object' ? raw : {};
        const tabSize = Number(value.tabSize);
        return {
            tabSize: EDITOR_TAB_SIZES.includes(tabSize) ? tabSize : 4,
            insertSpaces: value.insertSpaces !== false,
            wordWrap: value.wordWrap !== false,
            autoSave: value.autoSave === 'afterDelay' ? 'afterDelay' : 'off',
            trimTrailingWhitespace: value.trimTrailingWhitespace === true,
            insertFinalNewline: value.insertFinalNewline === true,
            formatOnSave: value.formatOnSave === true
        };
    }

    function editorSettings() {
        if (!editorSettingsCache) {
            let stored = null;
            try {
                stored = JSON.parse(localStorage.getItem(EDITOR_SETTINGS_KEY) || 'null');
            } catch (_) {
                stored = null;
            }
            editorSettingsCache = normalizeEditorSettings(stored);
        }
        return editorSettingsCache;
    }

    function updateEditorSettings(patch) {
        editorSettingsCache = normalizeEditorSettings(Object.assign({}, editorSettings(), patch || {}));
        try {
            localStorage.setItem(EDITOR_SETTINGS_KEY, JSON.stringify(editorSettingsCache));
        } catch (_) {}
        applyEditorSettingsToOpenEditors();
        return editorSettingsCache;
    }

    // The extensions behind view.csSettings: indent unit, tab size and word wrap.
    function editorSettingsExtensions(cm) {
        const settings = editorSettings();
        const extensions = [];
        if (cm.indentUnit) extensions.push(cm.indentUnit.of(settings.insertSpaces ? ' '.repeat(settings.tabSize) : '\t'));
        if (cm.EditorState.tabSize) extensions.push(cm.EditorState.tabSize.of(settings.tabSize));
        if (settings.wordWrap && cm.EditorView.lineWrapping) extensions.push(cm.EditorView.lineWrapping);
        return extensions;
    }

    // Reconfigures the settings compartment of every mounted editor in every Code Studio window.
    function applyEditorSettingsToOpenEditors() {
        instances.forEach(instance => runWithInstance(instance, () => {
            const cm = state.cmModule;
            if (cm) {
                state.openTabs.forEach(tab => (tab.views || []).forEach(view => {
                    if (!view || !view.csSettings) return;
                    try { view.dispatch({ effects: view.csSettings.reconfigure(editorSettingsExtensions(cm)) }); } catch (_) {}
                }));
            }
            if (studioRoot()) renderStatus();
        }));
    }

    // Status bar entry "Spaces: 4" or "Tab Size: 4"; a click opens the settings.
    function editorIndentStatusMarkup() {
        const settings = editorSettings();
        const label = settings.insertSpaces
            ? tr('codeStudio.statusSpaces', 'Spaces: {{size}}', { size: settings.tabSize })
            : tr('codeStudio.statusTabSize', 'Tab Size: {{size}}', { size: settings.tabSize });
        return `<span class="cs-status-item" data-clickable data-status-indent title="${esc(tr('codeStudio.settingsTitle', 'Editor Settings'))}">${esc(label)}</span>`;
    }

    // The font size of the dialog is the existing zoom state of the window.
    function setEditorFontSize(size) {
        const next = clampEditorFontSize(size);
        if (next === state.editorFontSize) return;
        state.editorFontSize = next;
        applyEditorZoom();
        saveState();
        renderWindowMenus();
        renderStatus();
    }

    function openEditorSettings() {
        const instance = state;
        if (!instance || instance.settingsClose) return;
        const settings = editorSettings();
        const title = tr('codeStudio.settingsTitle', 'Editor Settings');
        const option = (value, label, selected) => `<option value="${esc(value)}"${selected ? ' selected' : ''}>${esc(label)}</option>`;
        const checkbox = (key, label) => `<label class="cs-settings-check"><input type="checkbox" data-setting="${key}"${settings[key] ? ' checked' : ''}><span>${esc(label)}</span></label>`;
        const overlay = document.createElement('div');
        overlay.className = 'cs-modal-backdrop';
        overlay.innerHTML = `<form class="cs-modal cs-settings-modal" role="dialog" aria-modal="true" aria-label="${esc(title)}" data-cs-settings>
            <h3 class="cs-settings-title">${esc(title)}</h3>
            <div class="cs-settings-grid">
                <label class="cs-settings-row"><span>${esc(tr('codeStudio.settingsTabSize', 'Tab size'))}</span>
                    <select data-setting="tabSize">${EDITOR_TAB_SIZES.map(size => option(size, String(size), settings.tabSize === size)).join('')}</select></label>
                <label class="cs-settings-row"><span>${esc(tr('codeStudio.settingsIndentation', 'Indentation'))}</span>
                    <select data-setting="insertSpaces">${option('spaces', tr('codeStudio.settingsIndentSpaces', 'Spaces'), settings.insertSpaces)}${option('tabs', tr('codeStudio.settingsIndentTabs', 'Tabs'), !settings.insertSpaces)}</select></label>
                <label class="cs-settings-row"><span>${esc(tr('codeStudio.settingsFontSize', 'Font size'))}</span>
                    <input type="number" data-setting="fontSize" min="${MIN_EDITOR_FONT_SIZE}" max="${MAX_EDITOR_FONT_SIZE}" step="1" value="${esc(String(clampEditorFontSize(instance.editorFontSize)))}"></label>
                <label class="cs-settings-row"><span>${esc(tr('codeStudio.settingsAutoSave', 'Auto save'))}</span>
                    <select data-setting="autoSave">${option('off', tr('codeStudio.settingsAutoSaveOff', 'Off'), settings.autoSave !== 'afterDelay')}${option('afterDelay', tr('codeStudio.settingsAutoSaveDelay', 'After a 1 second pause'), settings.autoSave === 'afterDelay')}</select></label>
                ${checkbox('wordWrap', tr('codeStudio.settingsWordWrap', 'Word wrap'))}
                ${checkbox('trimTrailingWhitespace', tr('codeStudio.settingsTrimWhitespace', 'Trim trailing whitespace on save'))}
                ${checkbox('insertFinalNewline', tr('codeStudio.settingsFinalNewline', 'Insert final newline on save'))}
                ${checkbox('formatOnSave', tr('codeStudio.settingsFormatOnSave', 'Format on save'))}
            </div>
            <div class="cs-modal-actions">
                <button type="submit" class="cs-button primary" data-settings-close>${buttonIcon('check-square', 'Y')}<span>${esc(tr('desktop.close', 'Close'))}</span></button>
            </div>
        </form>`;
        document.body.appendChild(overlay);
        const releaseFocus = trapDialogFocus(overlay);
        let unregister = () => {};
        let closed = false;
        const close = () => {
            if (closed) return;
            closed = true;
            unregister();
            instance.settingsClose = null;
            releaseFocus();
            overlay.remove();
        };
        unregister = registerDisposer(close);
        instance.settingsClose = close;
        overlay.querySelector('form').addEventListener('submit', event => {
            event.preventDefault();
            close();
        });
        overlay.addEventListener('mousedown', event => {
            if (event.target === overlay) close();
        });
        overlay.addEventListener('keydown', event => {
            if (event.key !== 'Escape') return;
            event.preventDefault();
            event.stopPropagation();
            close();
        });
        // Every change applies at once; the dialog has no separate apply step.
        overlay.addEventListener('change', bindInstance(instance, event => {
            const control = event.target && typeof event.target.closest === 'function' ? event.target.closest('[data-setting]') : null;
            if (!control) return;
            const key = control.dataset.setting;
            if (key === 'fontSize') {
                setEditorFontSize(control.value);
                control.value = String(state.editorFontSize);
            } else if (key === 'tabSize') {
                updateEditorSettings({ tabSize: Number(control.value) });
            } else if (key === 'insertSpaces') {
                updateEditorSettings({ insertSpaces: control.value !== 'tabs' });
            } else if (key === 'autoSave') {
                updateEditorSettings({ autoSave: control.value });
            } else {
                updateEditorSettings({ [key]: control.checked === true });
            }
        }));
        const first = overlay.querySelector('[data-setting]');
        if (first) first.focus();
    }

    // Auto save writes a dirty file tab after a 1 s pause. Read-only windows and virtual tabs (Git diff) never save,
    // and auto save writes the buffer as it is: the save actions run on explicit saves only.
    function scheduleAutoSave(tab) {
        const instance = state;
        if (!instance || !isSourceTab(tab) || !tab.modified || editorSettings().autoSave !== 'afterDelay' || !canWrite(instance)) return;
        if (tab.autoSaveTimer) clearTimeout(tab.autoSaveTimer);
        tab.autoSaveTimer = setTimeout(bindInstance(instance, () => {
            tab.autoSaveTimer = null;
            if (!tab.modified || !state.openTabs.includes(tab) || editorSettings().autoSave !== 'afterDelay') return;
            saveTab(state, tab);
        }), AUTO_SAVE_DELAY_MS);
    }
