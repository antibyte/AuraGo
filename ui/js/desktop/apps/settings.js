(function () {
    'use strict';

    let host, ctx;

    function settingsSections() {
        const boot = (ctx.state && ctx.state.bootstrap) || {};
        const workspace = boot.workspace || {};
        return [
            {
                id: 'appearance', icon: 'settings-symbolic', fallback: 'A', title: 'desktop.settings_category_appearance', desc: 'desktop.settings_category_appearance_desc', items: [
                    settingSelect('appearance.wallpaper', 'desktop.settings_wallpaper', 'desktop.settings_wallpaper_desc', [
                        ['groupshoot', 'desktop.settings_wallpaper_groupshoot'],
                        ['aurora', 'desktop.settings_wallpaper_aurora'], ['midnight', 'desktop.settings_wallpaper_midnight'], ['slate', 'desktop.settings_wallpaper_slate'], ['ember', 'desktop.settings_wallpaper_ember'], ['forest', 'desktop.settings_wallpaper_forest'],
                        ['alpine_dawn', 'desktop.settings_wallpaper_alpine_dawn'], ['city_rain', 'desktop.settings_wallpaper_city_rain'], ['ocean_cliff', 'desktop.settings_wallpaper_ocean_cliff'],
                        ['aurora_glass', 'desktop.settings_wallpaper_aurora_glass'], ['nebula_flow', 'desktop.settings_wallpaper_nebula_flow'], ['paper_waves', 'desktop.settings_wallpaper_paper_waves']
                    ]),
                    settingSelect('appearance.theme', 'desktop.settings_theme', 'desktop.settings_theme_desc', [
                        ['standard', 'desktop.settings_theme_standard'], ['fruity', 'desktop.settings_theme_fruity']
                    ]),
                    settingSelect('appearance.fruity_mode', 'desktop.settings_fruity_mode', 'desktop.settings_fruity_mode_desc', [
                        ['light', 'desktop.settings_fruity_mode_light'], ['dark', 'desktop.settings_fruity_mode_dark']
                    ]),
                    settingSelect('appearance.accent', 'desktop.settings_accent', 'desktop.settings_accent_desc', [
                        ['teal', 'desktop.settings_accent_teal'], ['orange', 'desktop.settings_accent_orange'], ['blue', 'desktop.settings_accent_blue'], ['violet', 'desktop.settings_accent_violet'], ['green', 'desktop.settings_accent_green']
                    ]),
                    settingSelect('appearance.density', 'desktop.settings_density', 'desktop.settings_density_desc', [
                        ['comfortable', 'desktop.settings_density_comfortable'], ['compact', 'desktop.settings_density_compact']
                    ]),
                    settingSelect('appearance.icon_theme', 'desktop.settings_icon_theme', 'desktop.settings_icon_theme_desc', [
                        ['papirus', 'desktop.settings_icon_theme_papirus'], ['whitesur', 'desktop.settings_icon_theme_whitesur']
                    ])
                ]
            },
            {
                id: 'desktop', icon: 'desktop-symbolic', fallback: 'D', title: 'desktop.settings_category_desktop', desc: 'desktop.settings_category_desktop_desc', items: [
                    settingSelect('desktop.icon_size', 'desktop.settings_icon_size', 'desktop.settings_icon_size_desc', [
                        ['small', 'desktop.settings_icon_size_small'], ['medium', 'desktop.settings_icon_size_medium'], ['large', 'desktop.settings_icon_size_large']
                    ]),
                    settingToggle('desktop.show_widgets', 'desktop.settings_show_widgets', 'desktop.settings_show_widgets_desc')
                ]
            },
            {
                id: 'windows', icon: 'monitor-symbolic', fallback: 'W', title: 'desktop.settings_category_windows', desc: 'desktop.settings_category_windows_desc', items: [
                    settingToggle('windows.animations', 'desktop.settings_window_animations', 'desktop.settings_window_animations_desc'),
                    settingSelect('windows.default_size', 'desktop.settings_default_window_size', 'desktop.settings_default_window_size_desc', [
                        ['compact', 'desktop.settings_window_size_compact'], ['balanced', 'desktop.settings_window_size_balanced'], ['large', 'desktop.settings_window_size_large']
                    ])
                ]
            },
            {
                id: 'files', icon: 'folder-symbolic', fallback: 'F', title: 'desktop.settings_category_files', desc: 'desktop.settings_category_files_desc', items: [
                    settingToggle('files.confirm_delete', 'desktop.settings_confirm_delete', 'desktop.settings_confirm_delete_desc'),
                    settingSelect('files.default_folder', 'desktop.settings_default_folder', 'desktop.settings_default_folder_desc', [
                        ['Desktop', 'desktop.settings_folder_desktop'], ['Documents', 'desktop.settings_folder_documents'], ['Downloads', 'desktop.settings_folder_downloads'], ['Pictures', 'desktop.settings_folder_pictures'], ['Shared', 'desktop.settings_folder_shared']
                    ])
                ]
            },
            {
                id: 'agent', icon: 'apps-symbolic', fallback: 'A', title: 'desktop.settings_category_agent', desc: 'desktop.settings_category_agent_desc', items: [
                    settingToggle('agent.show_chat_button', 'desktop.settings_show_agent_button', 'desktop.settings_show_agent_button_desc'),
                    settingSelect('agent.provider', 'desktop.settings_agent_provider', 'desktop.settings_agent_provider_desc', desktopAgentProviderOptions(boot)),
                    settingInfo('desktop.setting_agent_control', boot.allow_agent_control ? ctx.t('desktop.on') : ctx.t('desktop.off'))
                ]
            },
            {
                id: 'system', icon: 'info-symbolic', fallback: 'i', title: 'desktop.settings_category_system', desc: 'desktop.settings_category_system_desc', items: [
                    settingInfo('desktop.setting_workspace', workspace.root || ''),
                    settingInfo('desktop.setting_readonly', boot.readonly ? ctx.t('desktop.on') : ctx.t('desktop.off')),
                    settingInfo('desktop.setting_apps', String((boot.installed_apps || []).length)),
                    settingInfo('desktop.setting_widgets', String((boot.widgets || []).length))
                ]
            }
        ];
    }

    function desktopAgentProviderOptions(boot) {
        const providers = Array.isArray(boot.providers) ? boot.providers : [];
        const options = [{ value: '', labelKey: 'desktop.settings_agent_provider_default' }];
        providers.forEach(provider => {
            if (!provider || !provider.id) return;
            const name = provider.name || provider.id;
            const model = provider.model ? ` (${provider.model})` : '';
            options.push({ value: provider.id, label: `${name}${model}` });
        });
        return options;
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

    function render(hostEl, renderCtx) {
        ctx = renderCtx || {};
        host = hostEl;
        if (!host) return;
        host.dataset.activeSettings = host.dataset.activeSettings || 'appearance';
        renderSettingsShell();
    }

    function dispose(windowId) {
        if (host) host.innerHTML = '';
        host = null;
        ctx = null;
    }

    function renderSettingsShell() {
        const sidebarWasOpen = host.querySelector('.vd-settings-sidebar') && host.querySelector('.vd-settings-sidebar').classList.contains('open');
        const sections = settingsSections();
        const active = sections.find(section => section.id === host.dataset.activeSettings) || sections[0];
        host.innerHTML = `<div class="vd-settings-app">
            <button type="button" class="vd-settings-hamburger" data-toggle-sidebar aria-label="${esc(ctx.t('desktop.app_settings'))}">
                ${iconMarkup('menu-symbolic', 'M', 'vd-settings-hamburger-icon', 20)}
            </button>
            <aside class="vd-settings-sidebar" aria-label="${esc(ctx.t('desktop.app_settings'))}">
                <div class="vd-settings-sidebar-header">
                    <div class="vd-settings-sidebar-title">${esc(ctx.t('desktop.app_settings'))}</div>
                </div>
                <div class="vd-settings-search">
                    <input type="search" class="vd-settings-search-input" placeholder="${esc(ctx.t('desktop.settings_search_placeholder'))}" autocomplete="off" spellcheck="false" inputmode="search" enterkeyhint="search" autocapitalize="off">
                </div>
                <nav class="vd-settings-nav-list">
                    ${sections.map(section => `<button type="button" class="vd-settings-nav ${section.id === active.id ? 'active' : ''}" data-section="${esc(section.id)}">
                        <span class="vd-settings-nav-icon">${iconMarkup(section.icon, section.fallback || section.icon, 'vd-settings-nav-icon', 20)}</span>
                        <span class="vd-settings-nav-text">
                            <span class="vd-settings-nav-title">${esc(ctx.t(section.title))}</span>
                            <span class="vd-settings-nav-desc">${esc(ctx.t(section.desc))}</span>
                        </span>
                    </button>`).join('')}
                </nav>
            </aside>
            <div class="vd-settings-backdrop" data-toggle-sidebar></div>
            <section class="vd-settings-pane">
                <div class="vd-settings-pane-head">
                    <div class="vd-settings-pane-icon">${iconMarkup(active.icon, active.fallback || active.icon, 'vd-settings-pane-papirus-icon', 28)}</div>
                    <div>
                        <div class="vd-settings-pane-title">${esc(ctx.t(active.title))}</div>
                        <div class="vd-settings-pane-desc">${esc(ctx.t(active.desc))}</div>
                    </div>
                </div>
                <div class="vd-settings-list">${active.items.map(renderSettingItem).join('')}</div>
            </section>
        </div>`;
        wireNav(sections);
        wireSettings();
        wireSearch(sections);
        wireSidebarToggle();
        if (sidebarWasOpen) {
            const sidebar = host.querySelector('.vd-settings-sidebar');
            const backdrop = host.querySelector('.vd-settings-backdrop');
            if (sidebar) sidebar.classList.add('open');
            if (backdrop) backdrop.classList.add('visible');
        }
    }

    function wireNav(sections) {
        host.querySelectorAll('[data-section]').forEach(btn => {
            btn.addEventListener('click', () => {
                host.dataset.activeSettings = btn.dataset.section;
                closeSidebar();
                renderSettingsShell();
            });
        });
    }

    function wireSettings() {
        host.querySelectorAll('[data-setting-key]').forEach(control => {
            control.addEventListener('change', async event => {
                const key = event.currentTarget.dataset.settingKey;
                const value = event.currentTarget.type === 'checkbox' ? String(event.currentTarget.checked) : event.currentTarget.value;
                await saveDesktopSetting(key, value);
            });
        });
    }

    function wireSearch(sections) {
        const searchInput = host.querySelector('.vd-settings-search-input');
        if (!searchInput) return;
        searchInput.addEventListener('input', () => {
            const query = searchInput.value.trim().toLowerCase();
            const allItems = host.querySelectorAll('.vd-setting-row');
            allItems.forEach(row => {
                const label = row.querySelector('.vd-setting-label');
                const help = row.querySelector('.vd-setting-help');
                const text = ((label ? label.textContent : '') + ' ' + (help ? help.textContent : '')).toLowerCase();
                row.hidden = query && !text.includes(query);
            });
            const navButtons = host.querySelectorAll('.vd-settings-nav');
            const allSectionsHidden = {};
            sections.forEach(section => { allSectionsHidden[section.id] = true; });
            allItems.forEach(row => {
                if (!row.hidden) {
                    const activeId = host.dataset.activeSettings;
                    allSectionsHidden[activeId] = false;
                }
            });
            navButtons.forEach(btn => {
                if (query && allSectionsHidden[btn.dataset.section]) btn.hidden = true;
                else btn.hidden = false;
            });
            if (!query) {
                allItems.forEach(row => row.hidden = false);
                navButtons.forEach(btn => btn.hidden = false);
            }
        });
    }

    function wireSidebarToggle() {
        const sidebar = host.querySelector('.vd-settings-sidebar');
        const backdrop = host.querySelector('.vd-settings-backdrop');
        if (!sidebar) return;
        host.querySelectorAll('[data-toggle-sidebar]').forEach(el => {
            el.addEventListener('click', () => {
                sidebar.classList.toggle('open');
                if (backdrop) backdrop.classList.toggle('visible');
            });
        });
        if (backdrop) {
            backdrop.addEventListener('click', e => {
                if (e.target === backdrop) closeSidebar();
            });
        }
    }

    function closeSidebar() {
        const sidebar = host.querySelector('.vd-settings-sidebar');
        const backdrop = host.querySelector('.vd-settings-backdrop');
        if (sidebar) sidebar.classList.remove('open');
        if (backdrop) backdrop.classList.remove('visible');
    }

    function renderSettingItem(item) {
        if (item.type === 'info') {
            return `<article class="vd-setting-row readonly">
                <div><div class="vd-setting-label">${esc(ctx.t(item.label))}</div></div>
                <div class="vd-setting-value">${esc(item.value)}</div>
            </article>`;
        }
        const control = item.type === 'toggle'
            ? `<label class="vd-switch"><input type="checkbox" data-setting-key="${esc(item.key)}" ${ctx.settingBool(item.key) ? 'checked' : ''}><span></span></label>`
            : `<select class="vd-setting-select" data-setting-key="${esc(item.key)}">${item.options.map(option => {
                const normalized = normalizeSettingOption(option);
                return `<option value="${esc(normalized.value)}" ${ctx.settingValue(item.key) === normalized.value ? 'selected' : ''}>${esc(normalized.label)}</option>`;
            }).join('')}</select>`;
        return `<article class="vd-setting-row">
            <div>
                <div class="vd-setting-label">${esc(ctx.t(item.label))}</div>
                <div class="vd-setting-help">${esc(ctx.t(item.desc))}</div>
            </div>
            ${control}
        </article>`;
    }

    function normalizeSettingOption(option) {
        if (Array.isArray(option)) {
            return { value: String(option[0] || ''), label: ctx.t(option[1]) };
        }
        const value = String((option && option.value) || '');
        const label = option && option.labelKey ? ctx.t(option.labelKey) : String((option && option.label) || value);
        return { value, label };
    }

    async function saveDesktopSetting(key, value) {
        try {
            const updates = [{ key, value }];
            if (key === 'appearance.theme') {
                const pairedIconTheme = value === 'fruity' ? 'whitesur' : value === 'standard' ? 'papirus' : '';
                if (pairedIconTheme && ctx.settingValue('appearance.icon_theme') !== pairedIconTheme) {
                    updates.push({ key: 'appearance.icon_theme', value: pairedIconTheme });
                }
            }
            if (!ctx.state.bootstrap) ctx.state.bootstrap = {};
            const results = await Promise.all(updates.map(update =>
                ctx.api('/api/desktop/settings', {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(update)
                })
            ));
            const last = results[results.length - 1];
            ctx.state.bootstrap.settings = last.settings || Object.assign(ctx.desktopSettings(), {});
            ctx.state.bootstrap.settings[key] = value;
            if (updates.length > 1) {
                for (const u of updates) ctx.state.bootstrap.settings[u.key] = u.value;
            }
            ctx.applyDesktopSettings();
            ctx.renderStartButtonIcon();
            ctx.renderIcons();
            ctx.renderWidgets();
            ctx.renderStartApps();
            if (host && host.isConnected) renderSettingsShell();
        } catch (err) {
            ctx.showDesktopNotification({ title: ctx.t('desktop.notification'), message: err.message });
            if (host && host.isConnected) renderSettingsShell();
        }
    }

    window.SettingsApp = { render, dispose };
}());
