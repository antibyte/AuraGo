(function () {
    'use strict';

    const SCHEME_MODE_LABELS = Object.freeze({
        'appearance.fruity_mode': 'desktop.settings_fruity_mode',
        'appearance.minimal_mode': 'desktop.settings_minimal_mode'
    });

    function settingSelect(key, label, desc, options) {
        return { type: 'select', key, label, desc, options };
    }

    function settingToggle(key, label, desc) {
        return { type: 'toggle', key, label, desc };
    }

    function settingInfo(label, value) {
        return { type: 'info', label, value };
    }

    function settingRange(key, label, desc, min, max, step) {
        return { type: 'range', key, label, desc, min, max, step };
    }

    function settingSoundThemes(ctx) {
        return {
            type: 'sound_theme',
            key: 'sound.theme',
            label: 'desktop.settings_sound_theme',
            desc: 'desktop.settings_sound_theme_desc',
            themes: [
                { id: 'desktop', label: 'desktop.settings_sound_theme_desktop', desc: 'desktop.settings_sound_theme_desktop_desc' },
                ...(typeof ctx.desktopThemes === 'function' ? ctx.desktopThemes().filter(theme => theme.cursorTheme).map(theme => ({
                    id: theme.soundTheme, label: theme.labelKey, desc: 'desktop.settings_sound_theme_' + theme.soundTheme + '_desc', art: theme.cursorTheme
                })) : []),
                { id: 'crystal', label: 'desktop.settings_sound_theme_crystal', desc: 'desktop.settings_sound_theme_crystal_desc' },
                { id: 'wood', label: 'desktop.settings_sound_theme_wood', desc: 'desktop.settings_sound_theme_wood_desc' },
                { id: 'analog', label: 'desktop.settings_sound_theme_analog', desc: 'desktop.settings_sound_theme_analog_desc' },
                { id: 'workshop', label: 'desktop.settings_sound_theme_workshop', desc: 'desktop.settings_sound_theme_workshop_desc' },
                { id: 'water', label: 'desktop.settings_sound_theme_water', desc: 'desktop.settings_sound_theme_water_desc' }
            ]
        };
    }

    function settingDesktopThemes() {
        return { type: 'desktop_theme', key: 'appearance.theme', label: 'desktop.settings_theme', desc: 'desktop.settings_theme_desc' };
    }

    function settingScreensaverThemes() {
        return {
            type: 'screensaver_theme',
            key: 'screensaver.theme',
            label: 'desktop.settings_screensaver_theme',
            desc: 'desktop.settings_screensaver_theme_desc',
            themes: ['abyss', 'event_horizon', 'aurora', 'ink', 'stardust', 'random'].map(id => ({
                id,
                label: 'desktop.settings_screensaver_' + id,
                desc: 'desktop.settings_screensaver_' + id + '_desc'
            }))
        };
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

    function sections(ctx) {
        const context = ctx || {};
        const boot = (context.state && context.state.bootstrap) || {};
        const workspace = boot.workspace || {};
        const activeTheme = typeof context.currentDesktopTheme === 'function' ? context.currentDesktopTheme() : null;
        const showSchemeMode = !activeTheme || !!activeTheme.schemeSetting;
        const schemeSetting = activeTheme && activeTheme.schemeSetting ? activeTheme.schemeSetting : 'appearance.fruity_mode';
        const schemeLabel = SCHEME_MODE_LABELS[schemeSetting] || 'desktop.settings_fruity_mode';
        const showAccent = !activeTheme || activeTheme.accent === 'user';
        const settingValue = typeof context.settingValue === 'function' ? context.settingValue : () => '';
        const iconTheme = settingValue('appearance.icon_theme');
        const iconThemeManifests = context.state && context.state.iconThemeManifests || {};
        const iconManifest = iconThemeManifests[iconTheme];

        return [
            {
                id: 'appearance', icon: 'settings-symbolic', fallback: 'A', title: 'desktop.settings_category_appearance', desc: 'desktop.settings_category_appearance_desc', items: [
                    settingSelect('appearance.wallpaper', 'desktop.settings_wallpaper', 'desktop.settings_wallpaper_desc', [
                        ['groupshoot', 'desktop.settings_wallpaper_groupshoot'],
                        ['aurora', 'desktop.settings_wallpaper_aurora'], ['midnight', 'desktop.settings_wallpaper_midnight'], ['slate', 'desktop.settings_wallpaper_slate'], ['ember', 'desktop.settings_wallpaper_ember'], ['forest', 'desktop.settings_wallpaper_forest'],
                        ['alpine_dawn', 'desktop.settings_wallpaper_alpine_dawn'], ['city_rain', 'desktop.settings_wallpaper_city_rain'], ['ocean_cliff', 'desktop.settings_wallpaper_ocean_cliff'],
                        ['aurora_glass', 'desktop.settings_wallpaper_aurora_glass'], ['nebula_flow', 'desktop.settings_wallpaper_nebula_flow'], ['paper_waves', 'desktop.settings_wallpaper_paper_waves'],
                        ['silk_flow', 'desktop.settings_wallpaper_silk_flow'], ['firefly_dusk', 'desktop.settings_wallpaper_firefly_dusk'],
                        ['neon_overdrive', 'desktop.settings_wallpaper_neon_overdrive'], ['fractal_trip', 'desktop.settings_wallpaper_fractal_trip'],
                        ['electronic_dreams', 'desktop.settings_wallpaper_electronic_dreams'], ['windows_xp', 'desktop.settings_wallpaper_windows_xp'], ['green_slime', 'desktop.settings_wallpaper_green_slime'], ['voxel_world', 'desktop.settings_wallpaper_voxel_world'],
                        ['orbit', 'desktop.settings_wallpaper_orbit'], ['minimal', 'desktop.settings_wallpaper_minimal']
                    ]),
                    settingDesktopThemes(),
                    settingToggle('appearance.theme_cursors', 'desktop.settings_theme_cursors', 'desktop.settings_theme_cursors_desc'),
                    activeTheme && activeTheme.cursorTheme ? { type: 'cursor_preview', theme: activeTheme.cursorTheme } : null,
                    showSchemeMode ? settingSelect(schemeSetting, schemeLabel, schemeLabel + '_desc', [
                        ['light', 'desktop.settings_fruity_mode_light'], ['dark', 'desktop.settings_fruity_mode_dark']
                    ]) : null,
                    showAccent ? settingSelect('appearance.accent', 'desktop.settings_accent', 'desktop.settings_accent_desc', [
                        ['theme', 'desktop.settings_accent_theme'], ['teal', 'desktop.settings_accent_teal'], ['orange', 'desktop.settings_accent_orange'], ['blue', 'desktop.settings_accent_blue'], ['violet', 'desktop.settings_accent_violet'], ['green', 'desktop.settings_accent_green']
                    ]) : null,
                    settingSelect('appearance.density', 'desktop.settings_density', 'desktop.settings_density_desc', [
                        ['comfortable', 'desktop.settings_density_comfortable'], ['compact', 'desktop.settings_density_compact']
                    ]),
                    settingSelect('accessibility.text_size', 'desktop.settings_text_size', 'desktop.settings_text_size_desc', [
                        ['100', 'desktop.settings_text_size_100'], ['125', 'desktop.settings_text_size_125'], ['150', 'desktop.settings_text_size_150']
                    ]),
                    settingToggle('accessibility.high_contrast', 'desktop.settings_high_contrast', 'desktop.settings_high_contrast_desc'),
                    settingSelect('appearance.icon_theme', 'desktop.settings_icon_theme', 'desktop.settings_icon_theme_desc', [
                        ['papirus', 'desktop.settings_icon_theme_papirus'], ['whitesur', 'desktop.settings_icon_theme_whitesur'], ['windows_xp', 'desktop.settings_icon_theme_windows_xp'],
                        ['tela', 'desktop.settings_icon_theme_tela'], ['candy', 'desktop.settings_icon_theme_candy'], ['fluent', 'desktop.settings_icon_theme_fluent'], ['buuf_nestort', 'desktop.settings_icon_theme_buuf_nestort']
                    ]),
                    ['tela', 'fluent'].includes(iconTheme) ? settingSelect('appearance.' + iconTheme + '_folder_color', 'desktop.settings_folder_color', 'desktop.settings_folder_color_desc',
                        ((iconManifest && iconManifest.folder_colors) || ['standard']).map(color => [color, 'desktop.settings_folder_color_' + color])) : null
                ].filter(Boolean)
            },
            {
                id: 'desktop', icon: 'desktop-symbolic', fallback: 'D', title: 'desktop.settings_category_desktop', desc: 'desktop.settings_category_desktop_desc', items: [
                    settingToggle('desktop.open_fullscreen', 'desktop.settings_open_fullscreen', 'desktop.settings_open_fullscreen_desc'),
                    settingSelect('desktop.icon_size', 'desktop.settings_icon_size', 'desktop.settings_icon_size_desc', [
                        ['small', 'desktop.settings_icon_size_small'], ['medium', 'desktop.settings_icon_size_medium'], ['large', 'desktop.settings_icon_size_large']
                    ]),
                    settingToggle('desktop.show_widgets', 'desktop.settings_show_widgets', 'desktop.settings_show_widgets_desc'),
                    settingToggle('phone_gadget.enabled', 'desktop.settings_phone_gadget', 'desktop.settings_phone_gadget_desc')
                ]
            },
            {
                id: 'windows', icon: 'monitor-symbolic', fallback: 'W', title: 'desktop.settings_category_windows', desc: 'desktop.settings_category_windows_desc', items: [
                    settingToggle('windows.restore_session', 'desktop.settings_restore_session', 'desktop.settings_restore_session_desc'),
                    settingToggle('windows.animations', 'desktop.settings_window_animations', 'desktop.settings_window_animations_desc'),
                    settingSelect('windows.default_size', 'desktop.settings_default_window_size', 'desktop.settings_default_window_size_desc', [
                        ['compact', 'desktop.settings_window_size_compact'], ['balanced', 'desktop.settings_window_size_balanced'], ['large', 'desktop.settings_window_size_large']
                    ])
                ]
            },
            {
                id: 'sound', icon: 'sound-symbolic', fallback: 'S', title: 'desktop.settings_category_sound', desc: 'desktop.settings_category_sound_desc', items: [
                    settingToggle('sound.enabled', 'desktop.settings_sound_enabled', 'desktop.settings_sound_enabled_desc'),
                    settingSoundThemes(context),
                    settingRange('sound.volume', 'desktop.settings_sound_volume', 'desktop.settings_sound_volume_desc', 0, 1, 0.05),
                    settingToggle('sound.windows', 'desktop.settings_sound_windows', 'desktop.settings_sound_windows_desc'),
                    settingToggle('sound.notifications', 'desktop.settings_sound_notifications', 'desktop.settings_sound_notifications_desc'),
                    settingToggle('sound.navigation', 'desktop.settings_sound_navigation', 'desktop.settings_sound_navigation_desc'),
                    settingToggle('sound.files', 'desktop.settings_sound_files', 'desktop.settings_sound_files_desc')
                ]
            },
            {
                id: 'screensaver', icon: 'screensaver-symbolic', fallback: 'S', title: 'desktop.settings_category_screensaver', desc: 'desktop.settings_category_screensaver_desc', items: [
                    settingToggle('screensaver.enabled', 'desktop.settings_screensaver_enabled', 'desktop.settings_screensaver_enabled_desc'),
                    settingScreensaverThemes(),
                    settingSelect('screensaver.idle_minutes', 'desktop.settings_screensaver_idle', 'desktop.settings_screensaver_idle_desc', [
                        ['1', 'desktop.settings_screensaver_idle_1'], ['2', 'desktop.settings_screensaver_idle_2'], ['3', 'desktop.settings_screensaver_idle_3'], ['5', 'desktop.settings_screensaver_idle_5'],
                        ['10', 'desktop.settings_screensaver_idle_10'], ['15', 'desktop.settings_screensaver_idle_15'], ['30', 'desktop.settings_screensaver_idle_30'], ['60', 'desktop.settings_screensaver_idle_60']
                    ]),
                    settingToggle('screensaver.clock', 'desktop.settings_screensaver_clock', 'desktop.settings_screensaver_clock_desc')
                ]
            },
            {
                id: 'files', icon: 'folder-symbolic', fallback: 'F', title: 'desktop.settings_category_files', desc: 'desktop.settings_category_files_desc', items: [
                    settingToggle('files.confirm_delete', 'desktop.settings_confirm_delete', 'desktop.settings_confirm_delete_desc'),
                    settingSelect('files.default_folder', 'desktop.settings_default_folder', 'desktop.settings_default_folder_desc', [
                        ['Desktop', 'desktop.settings_folder_desktop'], ['Documents', 'desktop.settings_folder_documents'], ['Downloads', 'desktop.settings_folder_downloads'], ['Pictures', 'desktop.settings_folder_pictures'], ['Shared', 'desktop.settings_folder_shared']
                    ]),
                    { type: 'default_apps', key: 'files.default_apps', label: 'desktop.settings_default_apps', desc: 'desktop.settings_default_apps_desc' }
                ]
            },
            {
                id: 'agent', icon: 'apps-symbolic', fallback: 'A', title: 'desktop.settings_category_agent', desc: 'desktop.settings_category_agent_desc', items: [
                    settingToggle('agent.show_chat_button', 'desktop.settings_show_agent_button', 'desktop.settings_show_agent_button_desc'),
                    settingSelect('agent.provider', 'desktop.settings_agent_provider', 'desktop.settings_agent_provider_desc', desktopAgentProviderOptions(boot)),
                    settingInfo('desktop.setting_agent_control', boot.allow_agent_control ? context.t && context.t('desktop.on') : context.t && context.t('desktop.off'))
                ]
            },
            {
                id: 'system', icon: 'info-symbolic', fallback: 'i', title: 'desktop.settings_category_system', desc: 'desktop.settings_category_system_desc', items: [
                    settingInfo('desktop.setting_workspace', workspace.root || ''),
                    settingInfo('desktop.setting_readonly', boot.readonly ? context.t && context.t('desktop.on') : context.t && context.t('desktop.off')),
                    settingInfo('desktop.setting_apps', String((boot.installed_apps || []).length)),
                    settingInfo('desktop.setting_widgets', String((boot.widgets || []).length))
                ]
            }
        ];
    }

    function settingKeyForItem(section, item) {
        if (item && item.key) return String(item.key);
        const byType = { desktop_theme: 'appearance.theme', sound_theme: 'sound.theme', screensaver_theme: 'screensaver.theme' };
        return byType[item && item.type] || '';
    }

    function optionSearchText(item, t) {
        const options = Array.isArray(item.options) ? item.options : [];
        const labels = options.map(option => {
            if (Array.isArray(option)) return String(option[0] || '') + ' ' + t(option[1]);
            return String(option && option.value || '') + ' ' + (option && option.labelKey ? t(option.labelKey) : String(option && option.label || ''));
        });
        const themes = Array.isArray(item.themes) ? item.themes.flatMap(theme => [theme.id, t(theme.label), t(theme.desc)]) : [];
        return labels.concat(themes).join(' ');
    }

    function list(query, ctx) {
        const context = ctx || {};
        const t = typeof context.t === 'function' ? context.t : key => key;
        const normalizedQuery = String(query || '').trim().toLocaleLowerCase();
        const terms = normalizedQuery.split(/\s+/).filter(Boolean);
        const result = [];
        sections(context).forEach(section => {
            (section.items || []).forEach(item => {
                const key = settingKeyForItem(section, item);
                if (!key) return;
                if (item.type === 'desktop_theme' && typeof context.desktopThemes === 'function' && !context.desktopThemes().length) return;
                if (typeof context.isSettingAvailable === 'function' && !context.isSettingAvailable(key, section.id, item)) return;
                const title = t(item.label || 'desktop.settings_' + key.replace(/\./g, '_'));
                const description = t(item.desc || '');
                const categoryTitle = t(section.title);
                const searchText = [key, title, description, categoryTitle, optionSearchText(item, t)].join(' ').toLocaleLowerCase();
                if (terms.length && !terms.every(term => searchText.includes(term))) return;
                const descriptor = {
                    id: 'setting-' + key,
                    type: 'setting',
                    title,
                    subtitle: categoryTitle,
                    description,
                    category: section.id,
                    settingKey: key,
                    searchText,
                    setting: item
                };
                if (typeof context.openSetting === 'function') {
                    descriptor.open = () => context.openSetting(section.id, key);
                }
                result.push(descriptor);
            });
        });
        return result;
    }

    window.AuraDesktopSettingsCatalog = Object.freeze({ sections, list });
}());
