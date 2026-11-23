(function () {
    'use strict';

    let host, ctx;
    let settingsListener;


    // The active theme descriptor from the registry; null when the host context predates it.
    function activeThemeInfo() {
        return typeof ctx.currentDesktopTheme === 'function' ? ctx.currentDesktopTheme() : null;
    }

    function settingsSections() {
        return window.AuraDesktopSettingsCatalog.sections(ctx);
    }

    function screensaverThumb(id) {
        const src = '/img/screensaver/' + id + '-thumb.webp';
        return window.AuraLazyAssets && typeof window.AuraLazyAssets.versionedURL === 'function' ? window.AuraLazyAssets.versionedURL(src) : src;
    }

    function renderScreensaverThemes(item) {
        const { esc, t, settingValue } = ctx;
        const active = settingValue('screensaver.theme') || 'abyss';
        const mosaic = ['abyss', 'event_horizon', 'aurora', 'stardust'];
        return `<article class="vd-setting-row vd-setting-row-stack">
            <div>
                <div class="vd-setting-label">${esc(t('desktop.settings_screensaver_theme'))}</div>
                <div class="vd-setting-help">${esc(t('desktop.settings_screensaver_theme_desc'))}</div>
            </div>
            <div class="vd-screensaver-grid">${(item.themes || []).map(theme => `<div class="vd-screensaver-card${active === theme.id ? ' active' : ''}" data-screensaver-theme="${esc(theme.id)}" tabindex="0" role="button" aria-pressed="${active === theme.id ? 'true' : 'false'}">
                <div class="vd-screensaver-thumb${theme.id === 'random' ? ' is-mosaic' : ''}" data-thumb="${esc(theme.id)}">${theme.id === 'random'
                    ? mosaic.map(id => `<img src="${esc(screensaverThumb(id))}" alt="" loading="lazy" decoding="async">`).join('')
                    : `<img src="${esc(screensaverThumb(theme.id))}" alt="" loading="lazy" decoding="async">`}</div>
                <div class="vd-screensaver-title">${esc(t(theme.label))}</div>
                <div class="vd-screensaver-desc">${esc(t(theme.desc))}</div>
                <button type="button" class="vd-button vd-screensaver-preview" data-screensaver-preview="${esc(theme.id)}">${esc(t('desktop.settings_screensaver_preview'))}</button>
            </div>`).join('')}</div>
        </article>`;
    }

    function themeThumb(theme) {
        const src = theme.thumb || '/img/themes/' + theme.id + '.svg';
        return window.AuraLazyAssets && typeof window.AuraLazyAssets.versionedURL === 'function' ? window.AuraLazyAssets.versionedURL(src) : src;
    }

    // The card of the stored theme is active; an id the registry does not know shows as Standard,
    // exactly like the shell does.
    function activeThemeId(themes) {
        const stored = ctx.settingValue('appearance.theme') || 'standard';
        return themes.some(theme => theme.id === stored) ? stored : 'standard';
    }

    function renderDesktopThemes(item) {
        const { esc, t } = ctx;
        const themes = typeof ctx.desktopThemes === 'function' ? ctx.desktopThemes() : [];
        if (!themes.length) return '';
        const active = activeThemeId(themes);
        return `<article class="vd-setting-row vd-setting-row-stack">
            <div>
                <div class="vd-setting-label">${esc(t('desktop.settings_theme'))}</div>
                <div class="vd-setting-help">${esc(t('desktop.settings_theme_desc'))}</div>
            </div>
            <div class="vd-theme-grid" role="group" aria-label="${esc(t('desktop.settings_theme_picker_label'))}">${themes.map(theme => `<div class="vd-theme-card${active === theme.id ? ' active' : ''}" data-desktop-theme="${esc(theme.id)}" tabindex="0" role="button" aria-pressed="${active === theme.id ? 'true' : 'false'}">
                <div class="vd-theme-thumb"><img src="${esc(themeThumb(theme))}" alt="" loading="lazy" decoding="async"></div>
                <div class="vd-theme-title">${esc(t(theme.labelKey))}</div>
                <div class="vd-theme-desc">${esc(t(theme.descKey))}</div>
            </div>`).join('')}</div>
        </article>`;
    }

    function render(hostEl, renderCtx) {
        ctx = renderCtx || {};
        host = hostEl;
        if (!host) return;
        const category = String((renderCtx && renderCtx.category) || host.dataset.activeSettings || 'appearance');
        host.dataset.activeSettings = category;
        if (settingsListener) window.removeEventListener('aurago:desktop-settings', settingsListener);
        settingsListener = () => { if (host?.isConnected) renderSettingsShell(); };
        window.addEventListener('aurago:desktop-settings', settingsListener);
        renderSettingsShell();
        if (ctx.settingKey) {
            const row = Array.from(host.querySelectorAll('[data-setting-row]')).find(item => item.dataset.settingRow === ctx.settingKey);
            if (row) { row.scrollIntoView({ block: 'nearest' }); row.querySelector('input, select, button, [tabindex]')?.focus({ preventScroll: true }); }
        }
    }

    function dispose(windowId) {
        if (settingsListener) window.removeEventListener('aurago:desktop-settings', settingsListener);
        settingsListener = null;
        if (host) host.innerHTML = '';
        host = null;
        ctx = null;
    }

    function renderSettingsShell() {
        const focusKey = host.contains(document.activeElement) ? document.activeElement.dataset.settingKey : '';
        const pane = host.querySelector('.vd-settings-pane');
        const paneScroll = pane ? pane.scrollTop : 0;
        const sidebarEl = host.querySelector('.vd-settings-sidebar');
        const sidebarScroll = sidebarEl ? sidebarEl.scrollTop : 0;
        const searchEl = host.querySelector('.vd-settings-search-input');
        const searchValue = searchEl ? searchEl.value : '';
        const sidebarWasOpen = sidebarEl && sidebarEl.classList.contains('open');
        const sections = settingsSections();
        const active = sections.find(section => section.id === host.dataset.activeSettings) || sections[0];
        const { esc, t, iconMarkup } = ctx;
        host.innerHTML = `<div class="vd-settings-app">
            <button type="button" class="vd-settings-hamburger" data-toggle-sidebar aria-label="${esc(t('desktop.app_settings'))}">
                ${iconMarkup('menu-symbolic', 'M', 'vd-settings-hamburger-icon', 20)}
            </button>
            <aside class="vd-settings-sidebar" aria-label="${esc(t('desktop.app_settings'))}">
                <div class="vd-settings-sidebar-header">
                    <div class="vd-settings-sidebar-title">${esc(t('desktop.app_settings'))}</div>
                </div>
                <div class="vd-settings-search">
                    <input type="search" class="vd-settings-search-input" placeholder="${esc(t('desktop.settings_search_placeholder'))}" autocomplete="off" spellcheck="false" inputmode="search" enterkeyhint="search" autocapitalize="off">
                </div>
                <nav class="vd-settings-nav-list">
                    ${sections.map(section => `<button type="button" class="vd-settings-nav ${section.id === active.id ? 'active' : ''}" data-section="${esc(section.id)}">
                        <span class="vd-settings-nav-icon">${iconMarkup(section.icon, section.fallback || section.icon, 'vd-settings-nav-icon', 20)}</span>
                        <span class="vd-settings-nav-text">
                            <span class="vd-settings-nav-title">${esc(t(section.title))}</span>
                            <span class="vd-settings-nav-desc">${esc(t(section.desc))}</span>
                        </span>
                    </button>`).join('')}
                </nav>
            </aside>
            <div class="vd-settings-backdrop" data-toggle-sidebar></div>
            <section class="vd-settings-pane">
                <div class="vd-settings-pane-head">
                    <div class="vd-settings-pane-icon">${iconMarkup(active.icon, active.fallback || active.icon, 'vd-settings-pane-papirus-icon', 28)}</div>
                    <div>
                        <div class="vd-settings-pane-title">${esc(t(active.title))}</div>
                        <div class="vd-settings-pane-desc">${esc(t(active.desc))}</div>
                    </div>
                </div>
                <div class="vd-settings-list">${active.items.map(item => `<div data-setting-row="${esc(item.key || '')}">${renderSettingItem(item)}</div>`).join('')}</div>
            </section>
        </div>`;
        wireNav(sections);
        wireSettings();
        wireSearch(sections);
        wireSidebarToggle();
        if (ctx.desktopReadonly()) {
            host.querySelectorAll('[data-setting-key], [data-settings-action]:not([data-settings-action="workspaces"]), [data-default-extension], [data-default-reset], [data-default-add], [data-default-new], [data-default-choice], [data-sound-theme], [data-desktop-theme], [data-screensaver-theme]').forEach(control => {
                control.disabled = true;
                control.setAttribute('aria-disabled', 'true');
                control.tabIndex = -1;
            });
        }
        if (focusKey) Array.from(host.querySelectorAll('[data-setting-key]')).find(control => control.dataset.settingKey === focusKey)?.focus({ preventScroll: true });
        const nextPane = host.querySelector('.vd-settings-pane');
        if (nextPane) nextPane.scrollTop = paneScroll;
        const nextSidebar = host.querySelector('.vd-settings-sidebar');
        if (nextSidebar) nextSidebar.scrollTop = sidebarScroll;
        if (sidebarWasOpen) {
            const sidebar = host.querySelector('.vd-settings-sidebar');
            const backdrop = host.querySelector('.vd-settings-backdrop');
            if (sidebar) sidebar.classList.add('open');
            if (backdrop) backdrop.classList.add('visible');
        }
        if (searchValue) {
            const nextSearch = host.querySelector('.vd-settings-search-input');
            if (nextSearch) {
                nextSearch.value = searchValue;
                nextSearch.dispatchEvent(new Event('input'));
            }
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
        wireDefaultApps();
        const owner = host, environment = ctx;
        owner.querySelectorAll('[data-settings-action]').forEach(button => button.addEventListener('click', async () => {
            if (button.dataset.settingsAction === 'workspaces') {
                window.AuraDesktopWorkspaceManager?.open();
                return;
            }
            if (environment.desktopReadonly()) return;
            button.disabled = true;
            button.setAttribute('aria-busy', 'true');
            try {
                if (button.dataset.settingsAction === 'wallpaper-choose') await environment.chooseDesktopWallpaper();
                else await environment.saveDesktopWallpaper(environment.currentDesktopTheme()?.wallpaper || 'groupshoot');
            } finally {
                if (host === owner && owner.isConnected) {
                    renderSettingsShell();
                    owner.querySelector(`[data-settings-action="${button.dataset.settingsAction}"]`)?.focus({ preventScroll: true });
                }
            }
        }));
        host.querySelectorAll('[data-setting-key]').forEach(control => {
            control.addEventListener('change', async event => {
                const key = event.currentTarget.dataset.settingKey;
                const value = event.currentTarget.type === 'checkbox' ? String(event.currentTarget.checked) : event.currentTarget.value;
                if (key === 'appearance.wallpaper' && typeof ctx.saveDesktopWallpaper === 'function') {
                    await ctx.saveDesktopWallpaper(value);
                    if (host && host.isConnected) renderSettingsShell();
                    return;
                }
                await saveDesktopSetting(key, value);
            });
            if (control.type === 'range') {
                control.addEventListener('input', event => {
                    const value = parseFloat(event.currentTarget.value);
                    const pct = host.querySelector(`[data-volume-label="${event.currentTarget.dataset.settingKey}"]`);
                    if (pct) pct.textContent = Math.round(value * 100) + '%';
                    if (typeof ctx.setDesktopSoundVolume === 'function') ctx.setDesktopSoundVolume(value);
                });
            }
        });
        host.querySelectorAll('[data-sound-theme]').forEach(card => {
            const owner = host;
            const choose = async () => {
                if (owner.dataset.soundSaving === 'true') return;
                const focused = document.activeElement === card;
                owner.dataset.soundSaving = 'true';
                card.setAttribute('aria-busy', 'true');
                try { await saveDesktopSetting('sound.theme', card.dataset.soundTheme); }
                finally {
                    delete owner.dataset.soundSaving;
                    card.removeAttribute('aria-busy');
                    if (focused && owner.isConnected) owner.querySelector('[data-sound-theme][aria-pressed="true"]')?.focus({ preventScroll: true });
                }
            };
            card.addEventListener('click', async event => {
                if (event.target.closest('[data-sound-preview]')) return;
                await choose();
            });
            card.addEventListener('keydown', event => {
                if (event.target !== card || (event.key !== 'Enter' && event.key !== ' ')) return;
                event.preventDefault();
                choose();
            });
        });
        host.querySelectorAll('[data-screensaver-theme]').forEach(card => {
            const choose = () => saveDesktopSetting('screensaver.theme', card.dataset.screensaverTheme);
            card.addEventListener('click', event => {
                if (event.target.closest('[data-screensaver-preview]')) return;
                choose();
            });
            card.addEventListener('keydown', event => {
                if (event.target !== card || (event.key !== 'Enter' && event.key !== ' ')) return;
                event.preventDefault();
                choose();
            });
        });
        host.querySelectorAll('[data-screensaver-preview]').forEach(btn => {
            btn.addEventListener('click', event => {
                event.preventDefault();
                event.stopPropagation();
                if (typeof ctx.previewDesktopScreensaver === 'function') ctx.previewDesktopScreensaver(btn.dataset.screensaverPreview);
            });
        });
        wireDesktopThemes();
        host.querySelectorAll('[data-sound-preview]').forEach(btn => {
            btn.addEventListener('click', async event => {
                event.preventDefault();
                event.stopPropagation();
                const theme = btn.dataset.soundPreview;
                if (typeof ctx.previewDesktopSound === 'function') {
                    btn.disabled = true;
                    btn.setAttribute('aria-busy', 'true');
                    btn.textContent = ctx.t('desktop.settings_sound_preview_playing');
                    try { await ctx.previewDesktopSound(theme); }
                    catch (err) { ctx.showDesktopNotification({ title: ctx.t('desktop.notification'), message: ctx.t('desktop.settings_sound_preview_failed') }); }
                    finally {
                        btn.disabled = false;
                        btn.removeAttribute('aria-busy');
                        btn.textContent = ctx.t('desktop.settings_sound_preview');
                    }
                }
            });
        });
        host.querySelectorAll('[data-sound-category-preview]').forEach(btn => {
            btn.addEventListener('click', async event => {
                event.preventDefault();
                event.stopPropagation();
                const sounds = window.DesktopSounds;
                if (!sounds || typeof sounds.previewCategory !== 'function' || btn.disabled) return;
                btn.disabled = true;
                btn.setAttribute('aria-busy', 'true');
                btn.textContent = ctx.t('desktop.settings_sound_preview_playing');
                try { await sounds.previewCategory(btn.dataset.soundCategoryPreview, ctx.settingValue('sound.theme') || 'desktop'); }
                catch (err) { ctx.showDesktopNotification({ title: ctx.t('desktop.notification'), message: ctx.t('desktop.settings_sound_preview_failed') }); }
                finally {
                    if (btn.isConnected) {
                        btn.disabled = false;
                        btn.removeAttribute('aria-busy');
                        btn.textContent = ctx.t('desktop.settings_sound_preview');
                    }
                }
            });
        });
    }

    // Click, Enter and Space pick a theme card. A pick is ignored while the previous one is still being
    // saved, and focus returns to the active card afterwards because the re-render replaces every node.
    function wireDesktopThemes() {
        const owner = host;
        owner.querySelectorAll('[data-desktop-theme]').forEach(card => {
            const choose = async () => {
                const id = card.dataset.desktopTheme;
                if (!id || owner.dataset.themeSaving === 'true') return;
                const focused = document.activeElement;
                const hadFocus = !!focused && owner.contains(focused) && !!focused.closest('[data-desktop-theme]');
                owner.dataset.themeSaving = 'true';
                try {
                    await saveDesktopSetting('appearance.theme', id);
                } finally {
                    delete owner.dataset.themeSaving;
                    if (hadFocus) focusActiveThemeCard(owner);
                }
            };
            card.addEventListener('click', choose);
            card.addEventListener('keydown', event => {
                if (event.target !== card || (event.key !== 'Enter' && event.key !== ' ')) return;
                event.preventDefault();
                choose();
            });
        });
    }

    function focusActiveThemeCard(owner) {
        if (host !== owner || !owner.isConnected) return;
        const card = owner.querySelector('[data-desktop-theme][aria-pressed="true"]');
        if (card) card.focus({ preventScroll: true });
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
        const { esc, t, settingBool, settingValue } = ctx;
        if (item.type === 'default_apps') return renderDefaultApps();
        if (item.type === 'workspaces' || item.type === 'custom_wallpaper') {
            const custom = item.type === 'custom_wallpaper';
            const path = custom ? window.AuraDesktopCustomWallpaper?.path(ctx.wallpaperForActiveSpace()) || '' : '';
            return `<article class="vd-setting-row vd-setting-row-stack"><div><div class="vd-setting-label">${esc(t(item.label))}</div>
                <div class="vd-setting-help">${esc(t(item.desc))}</div>${path ? `<div class="vd-setting-help vd-wallpaper-path">${esc(path)}</div>` : ''}</div>
                <div class="vd-setting-actions"><button type="button" class="vd-button" data-settings-action="${custom ? 'wallpaper-choose' : 'workspaces'}">${esc(t(item.label))}</button>
                ${custom && path ? `<button type="button" class="vd-button" data-settings-action="wallpaper-remove">${esc(t('desktop.wallpaper_remove'))}</button>` : ''}</div></article>`;
        }
        if (item.type === 'cursor_preview') {
            return `<article class="vd-setting-row vd-setting-row-stack"><div class="vd-setting-help">${esc(t('desktop.settings_cursor_preview'))}</div><img class="vd-cursor-preview" src="${esc(soundArtworkURL('/img/cursors/' + item.theme + '/preview.svg'))}" alt="" width="288" height="88"></article>`;
        }
        if (item.type === 'info') {
            return `<article class="vd-setting-row readonly">
                <div><div class="vd-setting-label">${esc(t(item.label))}</div></div>
                <div class="vd-setting-value">${esc(item.value)}</div>
            </article>`;
        }
        if (item.type === 'sound_theme') {
            const active = settingValue('sound.theme') || 'desktop';
            return `<article class="vd-setting-row vd-setting-row-stack">
                <div>
                    <div class="vd-setting-label">${esc(t('desktop.settings_sound_theme'))}</div>
                    <div class="vd-setting-help">${esc(t('desktop.settings_sound_theme_desc'))}</div>
                </div>
                <div class="vd-sound-theme-grid">${(item.themes || []).map(theme => `<article class="vd-sound-theme-card${active === theme.id ? ' active' : ''}">
                    <button type="button" class="vd-sound-theme-select" data-sound-theme="${esc(theme.id)}" aria-pressed="${active === theme.id ? 'true' : 'false'}">
                        ${soundThemeArtwork(theme)}
                        <span class="vd-sound-theme-title">${esc(t(theme.label))}</span>
                        <span class="vd-sound-theme-desc">${esc(t(theme.desc))}</span>
                    </button>
                    <button type="button" class="vd-button vd-sound-theme-preview" aria-label="${esc(t('desktop.settings_sound_preview') + ': ' + t(theme.label))}" data-sound-preview="${esc(theme.id)}">${esc(t('desktop.settings_sound_preview'))}</button>
                </article>`).join('')}</div>
            </article>`;
        }
        if (item.type === 'screensaver_theme') {
            return renderScreensaverThemes(item);
        }
        if (item.type === 'desktop_theme') {
            return renderDesktopThemes(item);
        }
        if (item.type === 'range') {
            const currentValue = parseFloat(settingValue(item.key) || '0.6');
            const safe = isFinite(currentValue) ? Math.min(item.max, Math.max(item.min, currentValue)) : 0.6;
            return `<article class="vd-setting-row">
                <div>
                    <div class="vd-setting-label">${esc(t(item.label))}</div>
                    <div class="vd-setting-help">${esc(t(item.desc))}</div>
                </div>
                <div class="vd-setting-range-wrap">
                    <input type="range" class="vd-setting-range" data-setting-key="${esc(item.key)}" min="${item.min}" max="${item.max}" step="${item.step}" value="${safe}">
                    <span class="vd-setting-range-value" data-volume-label="${esc(item.key)}">${Math.round(safe * 100)}%</span>
                </div>
            </article>`;
        }
        const currentValue = item.key === 'appearance.wallpaper' && typeof ctx.wallpaperForActiveSpace === 'function'
            ? ctx.wallpaperForActiveSpace()
            : settingValue(item.key);
        const toggle = `<label class="vd-switch"><input type="checkbox" data-setting-key="${esc(item.key)}" aria-label="${esc(t(item.label))}" ${settingBool(item.key) ? 'checked' : ''}><span></span></label>`;
        const control = item.type === 'toggle'
            ? (item.previewCategory
                ? `<div class="vd-setting-actions vd-sound-category-actions"><button type="button" class="vd-button vd-sound-category-preview" data-sound-category-preview="${esc(item.previewCategory)}" aria-label="${esc(t('desktop.settings_sound_preview_category', { name: t(item.label) }))}">${esc(t('desktop.settings_sound_preview'))}</button>${toggle}</div>`
                : toggle)
            : `<select class="vd-setting-select" data-setting-key="${esc(item.key)}">${item.key === 'appearance.wallpaper' && window.AuraDesktopCustomWallpaper?.isCustom(currentValue) ? `<option value="${esc(currentValue)}" selected>${esc(window.AuraDesktopCustomWallpaper.path(currentValue))}</option>` : ''}${item.options.map(option => {
                const normalized = normalizeSettingOption(option);
                return `<option value="${esc(normalized.value)}" ${currentValue === normalized.value ? 'selected' : ''}>${esc(normalized.label)}</option>`;
            }).join('')}</select>`;
        return `<article class="vd-setting-row">
            <div>
                <div class="vd-setting-label">${esc(t(item.label))}</div>
                <div class="vd-setting-help">${esc(t(item.desc))}</div>
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

    function renderDefaultApps() {
        const { esc, t } = ctx;
        const rows = window.AuraDesktopFileAssociations.list(ctx.settingValue('files.default_apps'), ctx.allApps(), extension =>
            ctx.desktopAssociationCandidates({ name: 'file.' + extension, path: 'Shared/file.' + extension, type: 'file' }));
        return `<article class="vd-setting-row vd-setting-row-stack">
            <div><div class="vd-setting-label">${esc(t('desktop.settings_default_apps'))}</div><div class="vd-setting-help">${esc(t('desktop.settings_default_apps_desc'))}</div></div>
            ${rows.length ? rows.map(row => `<div class="vd-default-app-row"><label>.${esc(row.extension)} <select data-default-extension="${esc(row.extension)}" aria-label=".${esc(row.extension)}">
                ${!row.candidates.some(app => app.appId === row.appId) ? `<option value="${esc(row.appId)}" selected disabled>${esc(row.app?.name || row.appId)} (${esc(t('desktop.default_apps_unavailable'))})</option>` : ''}
                ${row.candidates.map(app => `<option value="${esc(app.appId)}" ${app.appId === row.appId ? 'selected' : ''}>${esc(app.label)}</option>`).join('')}</select></label>
                <button type="button" class="vd-button" data-default-reset="${esc(row.extension)}">${esc(t('desktop.default_apps_reset'))}</button></div>`).join('') : `<p>${esc(t('desktop.default_apps_empty'))}</p>`}
            <div class="vd-default-app-row"><label>${esc(t('desktop.default_apps_extension'))}<input data-default-new type="text" maxlength="32" placeholder="pdf" autocapitalize="off" spellcheck="false"></label>
            <select data-default-choice aria-label="${esc(t('desktop.settings_default_apps'))}" disabled></select>
            <button type="button" class="vd-button" data-default-add disabled>${esc(t('desktop.default_apps_add'))}</button></div>
        </article>`;
    }

    function wireDefaultApps() {
        const save = async (extension, appId) => {
            const owner = host, environment = ctx;
            const windowEl = owner.closest('.vd-window');
            const focused = owner.contains(document.activeElement);
            try { await environment.setDefaultAppForExtension(extension, appId); }
            catch (err) { environment.showDesktopNotification({ title: environment.t('desktop.notification'), message: err.message }); }
            if (host !== owner || !owner.isConnected) return;
            renderSettingsShell();
            if (focused && (!windowEl || environment.state.activeWindowId === windowEl.id)) {
                const normalized = extension.toLowerCase().replace(/^\./, '');
                const control = Array.from(owner.querySelectorAll('[data-default-extension]')).find(item => item.dataset.defaultExtension === normalized);
                (control || owner.querySelector('[data-default-new]'))?.focus({ preventScroll: true });
            }
        };
        host.querySelectorAll('[data-default-extension]').forEach(select => select.addEventListener('change', () => save(select.dataset.defaultExtension, select.value)));
        host.querySelectorAll('[data-default-reset]').forEach(button => button.addEventListener('click', () => save(button.dataset.defaultReset, '')));
        const input = host.querySelector('[data-default-new]');
        if (!input) return;
        const select = host.querySelector('[data-default-choice]');
        const add = host.querySelector('[data-default-add]');
        input.addEventListener('input', () => {
            const extension = input.value.trim().toLowerCase().replace(/^\./, '');
            const candidates = /^[a-z0-9][a-z0-9_+.-]{0,31}$/.test(extension) ? ctx.desktopAssociationCandidates({ name: 'file.' + extension, path: 'Shared/file.' + extension, type: 'file' }) : [];
            select.innerHTML = candidates.map(app => `<option value="${ctx.esc(app.appId)}">${ctx.esc(app.label)}</option>`).join('');
            select.disabled = add.disabled = !candidates.length || ctx.desktopReadonly();
        });
        add.addEventListener('click', () => save(input.value.trim(), select.value));
    }

    function soundArtworkURL(path) {
        return window.AuraLazyAssets ? window.AuraLazyAssets.versionedURL(path) : path;
    }

    function soundThemeArtwork(theme) {
        const current = activeThemeInfo();
        const art = theme.art || (theme.id === 'desktop' && current && current.cursorTheme);
        if (!art) return '';
        return `<img class="vd-sound-theme-art" src="${ctx.esc(soundArtworkURL('/img/themes/sounds/' + art + '.svg'))}" alt="" width="288" height="88" loading="lazy">`;
    }

    async function saveDesktopSetting(key, value) {
        const owner = host, environment = ctx;
        if (environment.desktopReadonly()) return false;
        try {
            await environment.saveDesktopPreference(key, value);
            if (key !== 'sound.volume' && host === owner && owner.isConnected) renderSettingsShell();
            return true;
        } catch (err) {
            environment.showDesktopNotification({ title: environment.t('desktop.notification'), message: err.message });
            if (host === owner && owner.isConnected) renderSettingsShell();
            return false;
        }
    }

    window.SettingsApp = { render, dispose };
}());
