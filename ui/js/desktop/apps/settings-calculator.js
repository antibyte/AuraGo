            const files = body.files || [];
            host.querySelector('.vd-file-list').innerHTML = files.length ? files.map(file => `<div class="vd-file-row" data-type="${esc(file.type)}" data-path="${esc(file.path)}" data-web-path="${esc(file.web_path || '')}" data-media-kind="${esc(file.media_kind || '')}" data-mime-type="${esc(file.mime_type || '')}">
                ${iconMarkup(iconForFile(file), file.type === 'directory' ? 'D' : file.name, 'vd-sprite-file', 26)}
                <span class="vd-file-name">${esc(file.name)}</span>
                <span class="vd-file-meta">${esc(file.type === 'directory' ? t('desktop.folder') : fmtBytes(file.size))}</span>
            </div>`).join('') : `<div class="vd-empty">${esc(t('desktop.empty_folder'))}</div>`;
            host.querySelectorAll('.vd-file-row').forEach(row => {
                row.addEventListener('dblclick', () => {
                    if (row.dataset.type === 'directory') renderFiles(id, row.dataset.path);
                    else openDesktopFileEntry(row);
                });
                row.addEventListener('click', () => {
                    host.querySelectorAll('.vd-file-row').forEach(item => item.classList.toggle('selected', item === row));
                });
                row.addEventListener('contextmenu', event => {
                    event.preventDefault();
                    const actions = [
                        { label: t('desktop.context_open'), icon: 'folder-open', fallback: 'O', action: () => row.dataset.type === 'directory' ? renderFiles(id, row.dataset.path) : openDesktopFileEntry(row) },
                        { label: t('desktop.context_rename'), icon: 'edit', fallback: 'E', action: () => renamePath(row.dataset.path) },
                        { label: t('desktop.context_delete'), icon: 'trash', fallback: 'X', action: () => deletePath(row.dataset.path) },
                    ];
                    if (row.dataset.webPath) {
                        actions.push({ label: t('desktop.media_download'), icon: 'download', fallback: 'D', action: () => downloadMediaPath(row.dataset.webPath, row.querySelector('.vd-file-name').textContent) });
                    } else if (row.dataset.type === 'file') {
                        actions.push({ label: t('desktop.media_download'), icon: 'download', fallback: 'D', action: () => downloadDesktopPath(row.dataset.path, row.querySelector('.vd-file-name').textContent) });
                    }
                    actions.push(
                        { separator: true },
                        { label: t('desktop.context_properties'), icon: 'info', fallback: 'i', action: () => showProperties(row.querySelector('.vd-file-name').textContent, row.dataset.path) }
                    );
                    showContextMenu(event.clientX, event.clientY, actions);
                });
            });
        } catch (err) {
            host.querySelector('.vd-file-list').innerHTML = `<div class="vd-empty">${esc(err.message)}</div>`;
        }
    }

    function setFallbackFileMenus(id, path) {
        const readonly = !!((state.bootstrap || {}).readonly);
        setWindowMenus(id, [
            {
                id: 'file',
                labelKey: 'desktop.menu_file',
                items: [
                    { id: 'new-file', labelKey: 'desktop.new_file', icon: 'file-plus', shortcut: 'Ctrl+N', disabled: readonly, action: () => openApp('editor', { path: joinPath(path || state.filesPath, 'untitled.txt'), content: '' }) },
                    { id: 'new-folder', labelKey: 'desktop.new_folder', icon: 'folder-plus', disabled: readonly, action: () => createFolderInPath(path || state.filesPath) }
                ]
            },
            {
                id: 'view',
                labelKey: 'desktop.menu_view',
                items: [
                    { id: 'up', labelKey: 'desktop.up', icon: 'arrow-up', action: () => {
                        const parts = (path || state.filesPath).split('/').filter(Boolean);
                        parts.pop();
                        renderFiles(id, parts.join('/'));
                    } },
                    { id: 'refresh', labelKey: 'desktop.context_refresh', icon: 'refresh', shortcut: 'F5', action: () => renderFiles(id, path || state.filesPath) }
                ]
            }
        ]);
    }

    function joinPath(base, name) {
        return [base, name].filter(Boolean).join('/');
    }

    function openEditorFile(path) {
        openApp('editor', { path });
    }

    function fileExtension(value) {
        const parts = String(value || '').split('.');
        return parts.length > 1 ? parts.pop().toLowerCase() : '';
    }

    function isWriterFile(file) {
        const ext = fileExtension((file && (file.name || file.path)) || '');
        return ext === 'docx' || ext === 'html' || ext === 'htm';
    }

    function isSheetsFile(file) {
        const ext = fileExtension((file && (file.name || file.path)) || '');
        return ext === 'xlsx' || ext === 'xlsm' || ext === 'csv';
    }

    function openDesktopFileEntry(row) {
        const entry = {
            name: row.querySelector('.vd-file-name, .vd-icon-label') ? row.querySelector('.vd-file-name, .vd-icon-label').textContent : row.dataset.path,
            path: row.dataset.path,
            web_path: row.dataset.webPath,
            media_kind: row.dataset.mediaKind,
            mime_type: row.dataset.mimeType
        };
        if (isWriterFile(entry)) return openApp('writer', { path: entry.path });
        if (isSheetsFile(entry)) return openApp('sheets', { path: entry.path });
        if (entry.web_path || entry.media_kind) return openMediaPreview(entry);
        openEditorFile(entry.path);
    }

    function downloadDesktopPath(path, filename) {
        if (!path) return;
        const link = document.createElement('a');
        link.href = '/api/desktop/download?path=' + encodeURIComponent(path);
        link.download = filename || '';
        document.body.appendChild(link);
        link.click();
        link.remove();
    }

    function downloadMediaPath(webPath, filename) {
        if (!webPath) return;
        const link = document.createElement('a');
        link.href = webPath;
        link.download = filename || '';
        document.body.appendChild(link);
        link.click();
        link.remove();
    }

    function openMediaPreview(file) {
        if (!file || !file.web_path) {
            if (isWriterFile(file)) return openApp('writer', { path: file.path });
            if (isSheetsFile(file)) return openApp('sheets', { path: file.path });
            if (file && file.path) openEditorFile(file.path);
            return;
        }
        const kind = file.media_kind || '';
        if (kind === 'document' && !String(file.mime_type || '').startsWith('text/')) {
            window.open(file.web_path, '_blank', 'noopener');
            return;
        }
        const overlay = document.createElement('div');
        overlay.className = 'vd-modal-backdrop vd-media-preview-backdrop';
        const body = kind === 'video'
            ? `<video controls autoplay src="${esc(file.web_path)}"></video>`
            : kind === 'audio'
                ? `<audio controls autoplay src="${esc(file.web_path)}"></audio>`
                : kind === 'image'
                    ? `<img src="${esc(file.web_path)}" alt="${esc(file.name || '')}">`
                    : `<iframe src="${esc(file.web_path)}" title="${esc(file.name || '')}"></iframe>`;
        overlay.innerHTML = `<div class="vd-media-preview" role="dialog" aria-modal="true">
            <div class="vd-media-preview-bar">
                <strong>${esc(file.name || file.path || t('desktop.media_open'))}</strong>
                <div>
                    <a class="vd-button" href="${esc(file.web_path)}" download="${esc(file.name || '')}">${esc(t('desktop.media_download'))}</a>
                    <button class="vd-button vd-button-primary" type="button" data-close>${esc(t('desktop.close'))}</button>
                </div>
            </div>
            <div class="vd-media-preview-body">${body}</div>
        </div>`;
        document.body.appendChild(overlay);
        const close = () => overlay.remove();
        overlay.querySelector('[data-close]').addEventListener('click', close);
        overlay.addEventListener('click', event => { if (event.target === overlay) close(); });
    }

    async function renderEditor(id, path, initialContent) {
        const host = contentEl(id);
        if (!host) return;
        host.innerHTML = `<div class="vd-editor">
            <div class="vd-toolbar">
                <span class="vd-path">${esc(path)}</span>
                <span class="vd-chat-meta" data-status></span>
            </div>
            <textarea spellcheck="false"></textarea>
        </div>`;
        const textarea = host.querySelector('textarea');
        const status = host.querySelector('[data-status]');
        textarea.value = initialContent;
        if (!initialContent) {
            try {
                const body = await api('/api/desktop/file?path=' + encodeURIComponent(path));
                textarea.value = body.content || '';
            } catch (_) {
                textarea.value = '';
            }
        }
        const saveEditor = async () => {
            status.textContent = t('desktop.saving');
            try {
                await api('/api/desktop/file', {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ path, content: textarea.value })
                });
                status.textContent = t('desktop.saved');
                await loadBootstrap();
            } catch (err) {
                status.textContent = err.message;
            }
        };
        setEditorMenus(id, path, textarea, status, saveEditor);
    }

    function setEditorMenus(id, path, textarea, status, saveEditor) {
        const readonly = !!((state.bootstrap || {}).readonly);
        setWindowMenus(id, [
            {
                id: 'file',
                labelKey: 'desktop.menu_file',
                items: [
                    { id: 'save', labelKey: 'desktop.save', icon: 'save', shortcut: 'Ctrl+S', disabled: readonly, action: saveEditor }
                ]
            },
            {
                id: 'edit',
                labelKey: 'desktop.menu_edit',
                items: [
                    { id: 'cut', labelKey: 'desktop.fm.cut', icon: 'scissors', shortcut: 'Ctrl+X', disabled: readonly, action: () => document.execCommand && document.execCommand('cut') },
                    { id: 'copy', labelKey: 'desktop.fm.copy', icon: 'copy', shortcut: 'Ctrl+C', action: () => document.execCommand && document.execCommand('copy') },
                    { id: 'paste', labelKey: 'desktop.fm.paste', icon: 'clipboard', shortcut: 'Ctrl+V', disabled: readonly, action: async () => {
                        if (!textarea || readonly) return;
                        if (navigator.clipboard && navigator.clipboard.readText) {
                            const text = await navigator.clipboard.readText().catch(() => '');
                            textarea.setRangeText(text, textarea.selectionStart, textarea.selectionEnd, 'end');
                            textarea.dispatchEvent(new Event('input', { bubbles: true }));
                        }
                    } },
                    { type: 'separator' },
                    { id: 'select-all', labelKey: 'desktop.fm.select_all', icon: 'check-square', shortcut: 'Ctrl+A', action: () => textarea && textarea.select() }
                ]
            },
            {
                id: 'help',
                labelKey: 'desktop.menu_help',
                items: [
                    { id: 'path', label: path, icon: 'text', disabled: true, action: () => { if (status) status.textContent = path; } }
                ]
            }
        ]);
    }

    function settingsSections() {
        const boot = state.bootstrap || {};
        const workspace = boot.workspace || {};
        return [
            {
                id: 'appearance', icon: 'settings', fallback: 'A', title: 'desktop.settings_category_appearance', desc: 'desktop.settings_category_appearance_desc', items: [
                    settingSelect('appearance.wallpaper', 'desktop.settings_wallpaper', 'desktop.settings_wallpaper_desc', [
                        ['aurora', 'desktop.settings_wallpaper_aurora'], ['midnight', 'desktop.settings_wallpaper_midnight'], ['slate', 'desktop.settings_wallpaper_slate'], ['ember', 'desktop.settings_wallpaper_ember'], ['forest', 'desktop.settings_wallpaper_forest'],
                        ['alpine_dawn', 'desktop.settings_wallpaper_alpine_dawn'], ['city_rain', 'desktop.settings_wallpaper_city_rain'], ['ocean_cliff', 'desktop.settings_wallpaper_ocean_cliff'],
                        ['aurora_glass', 'desktop.settings_wallpaper_aurora_glass'], ['nebula_flow', 'desktop.settings_wallpaper_nebula_flow'], ['paper_waves', 'desktop.settings_wallpaper_paper_waves']
                    ]),
                    settingSelect('appearance.theme', 'desktop.settings_theme', 'desktop.settings_theme_desc', [
                        ['standard', 'desktop.settings_theme_standard'], ['fruity', 'desktop.settings_theme_fruity']
                    ]),
                    settingSelect('appearance.accent', 'desktop.settings_accent', 'desktop.settings_accent_desc', [
                        ['teal', 'desktop.settings_accent_teal'], ['orange', 'desktop.settings_accent_orange'], ['blue', 'desktop.settings_accent_blue'], ['violet', 'desktop.settings_accent_violet'], ['green', 'desktop.settings_accent_green']
                    ]),
                    settingSelect('appearance.density', 'desktop.settings_density', 'desktop.settings_density_desc', [
                        ['comfortable', 'desktop.settings_density_comfortable'], ['compact', 'desktop.settings_density_compact']
                    ]),
                    settingSelect('appearance.icon_theme', 'desktop.settings_icon_theme', 'desktop.settings_icon_theme_desc', [
                        ['papirus', 'desktop.settings_icon_theme_papirus'], ['whitesur', 'desktop.settings_icon_theme_whitesur']
                    ]),
                    settingIconCatalog('desktop.settings_icon_catalog', 'desktop.settings_icon_catalog_desc')
                ]
            },
            {
                id: 'desktop', icon: 'desktop', fallback: 'D', title: 'desktop.settings_category_desktop', desc: 'desktop.settings_category_desktop_desc', items: [
                    settingSelect('desktop.icon_size', 'desktop.settings_icon_size', 'desktop.settings_icon_size_desc', [
                        ['small', 'desktop.settings_icon_size_small'], ['medium', 'desktop.settings_icon_size_medium'], ['large', 'desktop.settings_icon_size_large']
                    ]),
                    settingToggle('desktop.show_widgets', 'desktop.settings_show_widgets', 'desktop.settings_show_widgets_desc')
                ]
            },
            {
                id: 'windows', icon: 'monitor', fallback: 'W', title: 'desktop.settings_category_windows', desc: 'desktop.settings_category_windows_desc', items: [
                    settingToggle('windows.animations', 'desktop.settings_window_animations', 'desktop.settings_window_animations_desc'),
                    settingSelect('windows.default_size', 'desktop.settings_default_window_size', 'desktop.settings_default_window_size_desc', [
                        ['compact', 'desktop.settings_window_size_compact'], ['balanced', 'desktop.settings_window_size_balanced'], ['large', 'desktop.settings_window_size_large']
                    ])
                ]
            },
            {
                id: 'files', icon: 'folder', fallback: 'F', title: 'desktop.settings_category_files', desc: 'desktop.settings_category_files_desc', items: [
                    settingToggle('files.confirm_delete', 'desktop.settings_confirm_delete', 'desktop.settings_confirm_delete_desc'),
                    settingSelect('files.default_folder', 'desktop.settings_default_folder', 'desktop.settings_default_folder_desc', [
                        ['Desktop', 'desktop.settings_folder_desktop'], ['Documents', 'desktop.settings_folder_documents'], ['Downloads', 'desktop.settings_folder_downloads'], ['Pictures', 'desktop.settings_folder_pictures'], ['Shared', 'desktop.settings_folder_shared']
                    ])
                ]
            },
            {
                id: 'agent', icon: 'apps', fallback: 'A', title: 'desktop.settings_category_agent', desc: 'desktop.settings_category_agent_desc', items: [
                    settingToggle('agent.show_chat_button', 'desktop.settings_show_agent_button', 'desktop.settings_show_agent_button_desc'),
                    settingInfo('desktop.setting_agent_control', boot.allow_agent_control ? t('desktop.on') : t('desktop.off'))
                ]
            },
            {
                id: 'system', icon: 'info', fallback: 'i', title: 'desktop.settings_category_system', desc: 'desktop.settings_category_system_desc', items: [
                    settingInfo('desktop.setting_workspace', workspace.root || ''),
                    settingInfo('desktop.setting_readonly', boot.readonly ? t('desktop.on') : t('desktop.off')),
                    settingInfo('desktop.setting_apps', String((boot.installed_apps || []).length)),
                    settingInfo('desktop.setting_widgets', String((boot.widgets || []).length))
                ]
            }
        ];
    }

    function settingSelect(key, label, desc, options) {
        return { type: 'select', key, label, desc, options };
    }

    function settingToggle(key, label, desc) {
        return { type: 'toggle', key, label, desc };
    }

    function settingInfo(label, value) {
        return { type: 'info', label, value };
    }

    function settingIconCatalog(label, desc) {
        const catalog = (state.bootstrap && state.bootstrap.icon_catalog) || {};
        const preferred = Array.isArray(catalog.preferred) ? catalog.preferred.slice() : [];
        const aliases = catalog.aliases && typeof catalog.aliases === 'object'
            ? Object.keys(catalog.aliases).sort().map(alias => [alias, catalog.aliases[alias]])
            : [];
        return { type: 'icon_catalog', label, desc, preferred, aliases };
    }

    function renderSettings(id) {
        const host = contentEl(id);
        if (!host) return;
        host.dataset.activeSettings = host.dataset.activeSettings || 'appearance';
        renderSettingsShell(host);
    }

    function renderSettingsShell(host) {
        const sections = settingsSections();
        const active = sections.find(section => section.id === host.dataset.activeSettings) || sections[0];
        host.innerHTML = `<div class="vd-settings-app">
            <aside class="vd-settings-sidebar" aria-label="${esc(t('desktop.app_settings'))}">
                <div class="vd-settings-sidebar-title">${esc(t('desktop.app_settings'))}</div>
                ${sections.map(section => `<button type="button" class="vd-settings-nav ${section.id === active.id ? 'active' : ''}" data-section="${esc(section.id)}">
                    ${iconMarkup(section.icon, section.fallback || section.icon, 'vd-settings-nav-icon', 18)}<span>${esc(t(section.title))}</span>
                </button>`).join('')}
            </aside>
            <section class="vd-settings-pane">
                <div class="vd-settings-pane-head">
                    <div class="vd-settings-pane-icon">${iconMarkup(active.icon, active.fallback || active.icon, 'vd-settings-pane-papirus-icon', 28)}</div>
                    <div>
                        <div class="vd-settings-pane-title">${esc(t(active.title))}</div>
                        <div class="vd-settings-pane-desc">${esc(t(active.desc))}</div>
                    </div>
                </div>
                <div class="vd-settings-list">${active.items.map(renderSettingItem).join('')}</div>
            </section>
        </div>`;
        host.querySelectorAll('[data-section]').forEach(btn => btn.addEventListener('click', () => {
            host.dataset.activeSettings = btn.dataset.section;
            renderSettingsShell(host);
        }));
        host.querySelectorAll('[data-setting-key]').forEach(control => {
            control.addEventListener('change', async event => {
                const key = event.currentTarget.dataset.settingKey;
                const value = event.currentTarget.type === 'checkbox' ? String(event.currentTarget.checked) : event.currentTarget.value;
                await saveDesktopSetting(key, value, host);
            });
        });
    }

    function renderSettingItem(item) {
        if (item.type === 'info') {
            return `<article class="vd-setting-row readonly">
                <div><div class="vd-setting-label">${esc(t(item.label))}</div></div>
                <div class="vd-setting-value">${esc(item.value)}</div>
            </article>`;
        }
        if (item.type === 'icon_catalog') return renderIconCatalogSetting(item);
        const control = item.type === 'toggle'
            ? `<label class="vd-switch"><input type="checkbox" data-setting-key="${esc(item.key)}" ${settingBool(item.key) ? 'checked' : ''}><span></span></label>`
            : `<select class="vd-setting-select" data-setting-key="${esc(item.key)}">${item.options.map(option => `<option value="${esc(option[0])}" ${settingValue(item.key) === option[0] ? 'selected' : ''}>${esc(t(option[1]))}</option>`).join('')}</select>`;
        return `<article class="vd-setting-row">
            <div>
                <div class="vd-setting-label">${esc(t(item.label))}</div>
                <div class="vd-setting-help">${esc(t(item.desc))}</div>
            </div>
            ${control}
        </article>`;
    }

    function renderIconCatalogSetting(item) {
        const preferred = item.preferred.length
            ? item.preferred.map(name => `<span class="vd-icon-catalog-tag">${esc(name)}</span>`).join('')
            : `<span class="vd-icon-catalog-empty">${esc(t('desktop.settings_icon_catalog_empty'))}</span>`;
        const aliases = item.aliases.length
            ? `<div class="vd-icon-catalog-aliases">${item.aliases.map(pair => `<span><b>${esc(pair[0])}</b> -&gt; ${esc(pair[1])}</span>`).join('')}</div>`
            : '';
        return `<article class="vd-setting-row vd-icon-catalog-row">
            <div>
                <div class="vd-setting-label">${esc(t(item.label))}</div>
                <div class="vd-setting-help">${esc(t(item.desc))}</div>
            </div>
            <div class="vd-icon-catalog" aria-label="${esc(t(item.label))}">
                <div class="vd-icon-catalog-tags">${preferred}</div>
                ${aliases ? `<div class="vd-icon-catalog-alias-label">${esc(t('desktop.settings_icon_catalog_aliases'))}</div>${aliases}` : ''}
            </div>
        </article>`;
    }

    async function saveDesktopSetting(key, value, host) {
        try {
            const body = await api('/api/desktop/settings', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ key, value })
            });
            if (!state.bootstrap) state.bootstrap = {};
            state.bootstrap.settings = body.settings || Object.assign(desktopSettings(), { [key]: value });
            applyDesktopSettings();
            renderStartButtonIcon();
            renderIcons();
            renderWidgets();
            renderStartApps();
            if (host && host.isConnected) renderSettingsShell(host);
        } catch (err) {
            showDesktopNotification({ title: t('desktop.notification'), message: err.message });
            if (host && host.isConnected) renderSettingsShell(host);
        }
    }

    function plannerJSON(url, method, body) {
        return api(url, {
            method,
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body || {})
        });
    }

    function isoDate(date) {
        const d = new Date(date);
        const month = String(d.getMonth() + 1).padStart(2, '0');
        const day = String(d.getDate()).padStart(2, '0');
        return `${d.getFullYear()}-${month}-${day}`;
    }

    function dateTimeLocalValue(value) {
        const d = value ? new Date(value) : new Date();
        const month = String(d.getMonth() + 1).padStart(2, '0');
        const day = String(d.getDate()).padStart(2, '0');
        const hour = String(d.getHours()).padStart(2, '0');
        const minute = String(d.getMinutes()).padStart(2, '0');
        return `${d.getFullYear()}-${month}-${day}T${hour}:${minute}`;
    }

    function fromLocalDateTime(value) {
        return value ? new Date(value).toISOString() : new Date().toISOString();
    }

    function tokenizeCalculatorExpression(expression) {
        const tokens = [];
        let index = 0;
        while (index < expression.length) {
            const char = expression[index];
            if (/\s/.test(char)) {
                index += 1;
                continue;
            }
            if ((char >= '0' && char <= '9') || char === '.') {
                const start = index;
                let hasDigit = false;
                let hasDot = false;
                while (index < expression.length) {
                    const current = expression[index];
                    if (current >= '0' && current <= '9') {
                        hasDigit = true;
                        index += 1;
                    } else if (current === '.' && !hasDot) {
                        hasDot = true;
                        index += 1;
                    } else {
                        break;
                    }
                }
                if (!hasDigit) throw new Error('Invalid expression');
                const value = Number(expression.slice(start, index));
                if (!Number.isFinite(value)) throw new Error('Invalid expression');
                tokens.push({ type: 'number', value });
                continue;
            }
            if ((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')) {
                const start = index;
                while (index < expression.length) {
                    const current = expression[index];
                    if ((current >= 'a' && current <= 'z') || (current >= 'A' && current <= 'Z')) {
                        index += 1;
                    } else {
                        break;
                    }
                }
                tokens.push({ type: 'identifier', value: expression.slice(start, index) });
                continue;
            }
            if (char === 'Ï€') {
                tokens.push({ type: 'identifier', value: 'PI' });
                index += 1;
                continue;
            }
            if (char === 'âˆš') {
                tokens.push({ type: 'identifier', value: 'sqrt' });
                index += 1;
                continue;
            }
            if (char === 'Ã—') {
                tokens.push({ type: 'operator', value: '*' });
                index += 1;
                continue;
            }
            if (char === 'Ã·') {
                tokens.push({ type: 'operator', value: '/' });
                index += 1;
                continue;
            }
            if ('+-*/%^()!Â²'.includes(char)) {
                tokens.push({ type: 'operator', value: char });
                index += 1;
                continue;
            }
            throw new Error('Invalid expression');
        }
        tokens.push({ type: 'eof', value: '' });
        return tokens;
    }

    function calculatorFactorial(value) {
        if (!Number.isFinite(value) || value < 0 || !Number.isInteger(value)) throw new Error('Invalid expression');
        if (value > 170) throw new Error('Invalid expression');
        let result = 1;
        for (let i = 2; i <= value; i += 1) result *= i;
        return result;
    }

    function ensureFiniteCalculatorResult(value) {
        if (!Number.isFinite(value)) throw new Error('Invalid expression');
        return value;
    }

    function rejectZeroDivisor(operator, right) {
        if ((operator === '/' || operator === '%' || operator === 'MOD') && right === 0) throw new Error('Invalid expression');
    }

    function applyCalculatorOperation(name, value) {
        switch (name) {
            case 'sin':
                return Math.sin(value);
            case 'cos':
                return Math.cos(value);
            case 'tan':
                return Math.tan(value);
            case 'sqrt':
                return Math.sqrt(value);
            case 'log':
                return Math.log10(value);
            case 'ln':
                return Math.log(value);
            case 'abs':
                return Math.abs(value);
            case 'factorial':
                return calculatorFactorial(value);
            default:
                throw new Error('Invalid expression');
        }
    }

    function parseCalculatorExpression(tokens) {
        let position = 0;
        const peek = () => tokens[position] || { type: 'eof', value: '' };
        const consume = () => tokens[position++] || { type: 'eof', value: '' };
        const expectOperator = (value) => {
            const token = consume();
            if (token.type !== 'operator' || token.value !== value) throw new Error('Invalid expression');
        };
        const parseExpression = () => parseAdditiveExpression();
        const parseAdditiveExpression = () => {
            let value = parseMultiplicativeExpression();
            while (peek().type === 'operator' && (peek().value === '+' || peek().value === '-')) {
                const operator = consume().value;
                const right = parseMultiplicativeExpression();
                value = operator === '+' ? value + right : value - right;
            }
            return value;
        };
        const parseMultiplicativeExpression = () => {
            let value = parseUnaryExpression();
            while (peek().type === 'operator' && (peek().value === '*' || peek().value === '/' || peek().value === '%')) {
                const operator = consume().value;
                const right = parseUnaryExpression();
                rejectZeroDivisor(operator, right);
                if (operator === '*') value *= right;
                else if (operator === '/') value /= right;
                else value %= right;
            }
            return value;
        };
        const parseUnaryExpression = () => {
            if (peek().type === 'operator' && (peek().value === '+' || peek().value === '-')) {
                const operator = consume().value;
                const value = parseUnaryExpression();
                return operator === '-' ? -value : value;
            }
            return parsePowerExpression();
        };
        const parsePowerExpression = () => {
            let value = parsePostfixExpression();
            if (peek().type === 'operator' && peek().value === '^') {
                consume();
                value = Math.pow(value, parseUnaryExpression());
            }
            return value;
        };
        const parsePostfixExpression = () => {
            let value = parsePrimaryExpression();
            while (peek().type === 'operator' && (peek().value === '!' || peek().value === 'Â²')) {
                const operator = consume().value;
                value = operator === '!' ? calculatorFactorial(value) : Math.pow(value, 2);
            }
            return value;
        };
        const parsePrimaryExpression = () => {
            const token = consume();
            if (token.type === 'number') return token.value;
            if (token.type === 'operator' && token.value === '(') {
                const value = parseExpression();
                expectOperator(')');
                return value;
            }
            if (token.type === 'identifier') {
                const name = token.value.toLowerCase();
                if (name === 'pi') return Math.PI;
                if (name === 'e') return Math.E;
                expectOperator('(');
                const value = parseExpression();
                expectOperator(')');
                return applyCalculatorOperation(name, value);
            }
            throw new Error('Invalid expression');
        };
        const value = parseExpression();
        if (peek().type !== 'eof') throw new Error('Invalid expression');
        return value;
    }

    function evaluateCalculatorExpression(expression) {
        const value = parseCalculatorExpression(tokenizeCalculatorExpression(expression));
        return ensureFiniteCalculatorResult(value);
    }

    function renderCalculator(id) {
        const host = contentEl(id);
        if (!host) return;
        host.innerHTML = `<div class="vd-calc" tabindex="0">
            <div class="vd-calc-tabs">
                <button type="button" class="active" data-mode="standard">${esc(t('desktop.calc_standard'))}</button>
                <button type="button" data-mode="scientific">${esc(t('desktop.calc_scientific'))}</button>
                <button type="button" data-mode="programmer">${esc(t('desktop.calc_programmer'))}</button>
            </div>
            <div class="vd-calc-prog-section" data-prog-section>
                <div class="vd-calc-base" data-base-selector>
                    <button type="button" class="active" data-base="10">${esc(t('desktop.calc_dec'))}</button>
                    <button type="button" data-base="16">${esc(t('desktop.calc_hex'))}</button>
                    <button type="button" data-base="2">${esc(t('desktop.calc_bin'))}</button>
                    <button type="button" data-base="8">${esc(t('desktop.calc_oct'))}</button>
                </div>
                <div class="vd-calc-prog-display" data-prog-display>
                    <div><span>HEX</span><span data-hex>0</span></div>
                    <div><span>DEC</span><span data-dec>0</span></div>
                    <div><span>OCT</span><span data-oct>0</span></div>
                    <div><span>BIN</span><span data-bin>0</span></div>
                </div>
            </div>
            <div class="vd-calc-display"><div data-expression>0</div><strong data-result>0</strong></div>
            <div class="vd-calc-keys">
                ${['C','CE','âŒ«','%','7','8','9','Ã·','4','5','6','Ã—','1','2','3','-','0','00','.','+','Â±','='].map(key => `<button type="button" class="${/^[+\-Ã—Ã·=%]$/.test(key) ? 'op' : key === '=' ? 'eq' : key === 'Â±' ? 'fn scientific' : ''}" data-key="${esc(key)}">${esc(key)}</button>`).join('')}
                ${['sin','cos','tan','âˆš','log','ln','Ï€','xÂ²','(',')','e','xÊ¸','n!'].map(key => `<button type="button" class="fn scientific" data-key="${esc(key)}">${esc(key)}</button>`).join('')}
                <button type="button" class="fn programmer" data-key="AND">AND</button>
                <button type="button" class="fn programmer" data-key="OR">OR</button>
                <button type="button" class="fn programmer" data-key="XOR">XOR</button>
                <button type="button" class="fn programmer" data-key="NOT">NOT</button>
                <button type="button" class="fn programmer" data-key="SHL">SHL</button>
                <button type="button" class="fn programmer" data-key="SHR">SHR</button>
                <button type="button" class="fn programmer" data-key="MOD">MOD</button>
                <button type="button" class="fn programmer" data-key="A">A</button>
                <button type="button" class="fn programmer" data-key="B">B</button>
                <button type="button" class="fn programmer" data-key="C">C</button>
                <button type="button" class="fn programmer" data-key="D">D</button>
                <button type="button" class="fn programmer" data-key="E">E</button>
                <button type="button" class="fn programmer" data-key="F">F</button>
            </div>
            <aside class="vd-calc-history"><div>${esc(t('desktop.calc_history'))}</div><ol></ol></aside>
        </div>`;
        const root = host.querySelector('.vd-calc');
        const expressionEl = host.querySelector('[data-expression]');
        const resultEl = host.querySelector('[data-result]');
        const historyEl = host.querySelector('.vd-calc-history ol');
        const baseSelector = host.querySelector('[data-base-selector]');
        const progDisplay = host.querySelector('[data-prog-display]');
        const progSection = host.querySelector('[data-prog-section]');
        let expression = '';
        let mode = 'standard';
        let progBase = 10;
        const history = [];
        const showCalculatorContextMenu = event => {
            if (!event.target.closest('.vd-calc-display')) return false;
            showContextMenu(event.clientX, event.clientY, [
                { labelKey: 'desktop.fm.copy', icon: 'copy', action: async () => {
                    try {
                        if (navigator.clipboard && navigator.clipboard.writeText) await navigator.clipboard.writeText(resultEl.textContent || '0');
                    } catch (err) {
                        showDesktopNotification({ title: t('desktop.notification'), message: err.message });
                    }
                } }
            ]);
            return true;
        };
        wireContextMenuBoundary(root, { onContextMenu: showCalculatorContextMenu });
        const update = (result) => {
            expressionEl.textContent = expression || '0';
            const displayResult = result == null ? '0' : String(result);
            resultEl.textContent = displayResult;
            if (mode === 'programmer' && progDisplay) {
                const num = parseInt(displayResult, 10);
                const safeNum = Number.isFinite(num) ? num : 0;
                progDisplay.querySelector('[data-hex]').textContent = safeNum.toString(16).toUpperCase();
                progDisplay.querySelector('[data-dec]').textContent = String(safeNum);
                progDisplay.querySelector('[data-oct]').textContent = safeNum.toString(8);
                progDisplay.querySelector('[data-bin]').textContent = safeNum.toString(2);
            }
        };
        const evaluate = () => {
            if (!expression) return;
            let value;
            if (mode === 'programmer') {
                value = evaluateProgrammerExpression(expression, progBase);
            } else {
                value = evaluateCalculatorExpression(expression);
            }
            let result;
            if (mode === 'programmer') {
                result = value;
            } else {
                result = Number(value.toFixed(10));
            }
            history.unshift(`${expression} = ${result}`);
            history.splice(8);
            historyEl.innerHTML = history.map(item => `<li>${esc(item)}</li>`).join('');
            expression = String(result);
            update(result);
        };
        const animateButton = key => {
            const btn = host.querySelector(`[data-key="${esc(key)}"]`);
            if (!btn) return;
            btn.classList.add('pressed');
            setTimeout(() => btn.classList.remove('pressed'), 120);
        };
        const flashDisplay = () => {
            resultEl.classList.add('typing');
            setTimeout(() => resultEl.classList.remove('typing'), 150);
        };
        const validDigitForBase = ch => {
            if (progBase === 2) return /[01]/.test(ch);
            if (progBase === 8) return /[0-7]/.test(ch);
            if (progBase === 10) return /[0-9]/.test(ch);
            if (progBase === 16) return /[0-9A-Fa-f]/.test(ch);
            return true;
        };
        const press = key => {
            try {
                if (key === 'C') expression = '';
                else if (key === 'CE') expression = '';
                else if (key === 'âŒ«') expression = expression.slice(0, -1);
                else if (key === '=') {
                    evaluate();
                    animateButton('=');
                    return;
                }
                else if (mode === 'programmer') {
                    if (['AND','OR','XOR','SHL','SHR','MOD'].includes(key)) expression += ` ${key} `;
                    else if (key === 'NOT') expression += 'NOT ';
                    else if (/^[0-9A-Fa-f]$/.test(key)) {
                        if (validDigitForBase(key)) expression += key;
                    }
                    else if (['+','-','Ã—','Ã·','%','(',')'].includes(key)) expression += key;
                    else if (key === '.') {
                        if (progBase === 10) expression += '.';
                    }
                }
                else if (key === 'Â±') expression = expression ? `(-1*(${expression}))` : '-';
                else if (key === 'xÂ²') expression += 'Â²';
                else if (key === 'xÊ¸') expression += '^';
                else if (key === 'n!') expression += '!';
                else if (['sin', 'cos', 'tan', 'log', 'ln', 'âˆš'].includes(key)) expression += `${key}(`;
                else expression += key;
                update();
                flashDisplay();
            } catch (err) {
                resultEl.textContent = err.message;
            }
        };
        host.querySelectorAll('[data-key]').forEach(btn => btn.addEventListener('click', () => {
            btn.classList.add('pressed');
            setTimeout(() => btn.classList.remove('pressed'), 120);
            press(btn.dataset.key);
        }));
        host.querySelectorAll('[data-mode]').forEach(btn => btn.addEventListener('click', () => {
            host.querySelectorAll('[data-mode]').forEach(item => item.classList.toggle('active', item === btn));
            mode = btn.dataset.mode;
            root.classList.toggle('scientific-on', mode === 'scientific');
            root.classList.toggle('programmer-on', mode === 'programmer');
            if (progSection) progSection.hidden = mode !== 'programmer';
            expression = '';
            update();
        }));
        host.querySelectorAll('[data-base]').forEach(btn => btn.addEventListener('click', () => {
            host.querySelectorAll('[data-base]').forEach(item => item.classList.toggle('active', item === btn));
            progBase = parseInt(btn.dataset.base, 10);
            expression = '';
            update();
        }));
        root.addEventListener('keydown', event => {
            const map = { Enter: '=', Backspace: 'âŒ«', Escape: 'C', '*': 'Ã—', '/': 'Ã·' };
            const key = map[event.key] || event.key;
            if (mode === 'programmer') {
                if (/^[0-9A-Fa-f]$/.test(key) || ['+','-','(',')','=','âŒ«','C','Ã—','Ã·'].includes(key)) {
                    event.preventDefault();
                    animateButton(key.toUpperCase());
                    press(key.toUpperCase());
                    return;
                }
            }
            if (/^[0-9.+\-()%]$/.test(key) || ['=', 'âŒ«', 'C', 'Ã—', 'Ã·'].includes(key)) {
                event.preventDefault();
                animateButton(key);
                press(key);
            }
        });
        root.focus();
    }

    function evaluateProgrammerExpression(expression, base) {
        const tokens = tokenizeProgrammerExpression(expression, base);
        return parseProgrammerExpression(tokens);
    }

    function tokenizeProgrammerExpression(expression, base) {
        const tokens = [];
        let index = 0;
        const isDigit = ch => {
            if (base === 2) return ch === '0' || ch === '1';
