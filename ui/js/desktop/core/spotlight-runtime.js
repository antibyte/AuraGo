    let spotlightOpen = false;

    function closeSpotlight() {
        const backdrop = document.getElementById('vd-spotlight-backdrop');
        if (backdrop) backdrop.remove();
        spotlightOpen = false;
    }

    function spotlightSettingsEntries() {
        return [
            { id: 'settings-appearance', title: t('desktop.settings_category_appearance'), action: () => openApp('settings', { category: 'appearance' }) },
            { id: 'settings-desktop', title: t('desktop.settings_category_desktop'), action: () => openApp('settings', { category: 'desktop' }) },
            { id: 'settings-windows', title: t('desktop.settings_category_windows'), action: () => openApp('settings', { category: 'windows' }) },
            { id: 'settings-files', title: t('desktop.settings_category_files'), action: () => openApp('settings', { category: 'files' }) },
            { id: 'settings-agent', title: t('desktop.settings_category_agent'), action: () => openApp('settings', { category: 'agent' }) }
        ];
    }

    function spotlightAppEntries(query) {
        const q = String(query || '').trim().toLowerCase();
        return startMenuApps()
            .filter(app => !q || appName(app).toLowerCase().includes(q) || String(app.id || '').includes(q))
            .slice(0, 8)
            .map(app => ({
                id: 'app-' + app.id,
                title: appName(app),
                subtitle: t('desktop.spotlight_apps'),
                action: () => openApp(app.id)
            }));
    }

    function spotlightRecentFileEntries(query) {
        const q = String(query || '').trim().toLowerCase();
        return readRecentFiles()
            .filter(entry => !q || entry.name.toLowerCase().includes(q) || entry.path.toLowerCase().includes(q))
            .slice(0, 6)
            .map(entry => ({
                id: 'recent-' + entry.path,
                title: entry.name,
                subtitle: t('desktop.spotlight_recent_files'),
                action: () => openDesktopPath(entry.path)
            }));
    }

    function openDesktopPath(path) {
        const normalized = normalizeDesktopPath(path);
        if (!normalized) return;
        const ext = normalized.split('.').pop().toLowerCase();
        const defaultApp = defaultAppForExtension(ext);
        if (defaultApp) {
            openApp(defaultApp, { path: normalized });
            return;
        }
        openApp('files', { path: pathDir(normalized) || '' });
        openApp('viewer', { path: normalized });
    }

    async function spotlightFileEntries(query) {
        const q = String(query || '').trim();
        if (q.length < 2) return [];
        try {
            const body = await api('/api/desktop/search?query=' + encodeURIComponent(q));
            return (body.files || body.results || []).slice(0, 8).map(file => ({
                id: 'file-' + file.path,
                title: file.name || pathBaseName(file.path),
                subtitle: t('desktop.spotlight_files'),
                action: () => openDesktopPath(file.path)
            }));
        } catch (_) {
            return [];
        }
    }

    function renderSpotlightResults(entries, activeIndex) {
        const host = document.querySelector('#vd-spotlight-backdrop [data-spotlight-results]');
        if (!host) return;
        if (!entries.length) {
            host.innerHTML = `<div class="vd-spotlight-empty">${esc(t('desktop.spotlight_empty'))}</div>`;
            return;
        }
        host.innerHTML = entries.map((entry, index) => `<button type="button" class="vd-spotlight-item${index === activeIndex ? ' active' : ''}" data-spotlight-index="${index}">
            <span class="vd-spotlight-item-title">${esc(entry.title)}</span>
            ${entry.subtitle ? `<span class="vd-spotlight-item-sub">${esc(entry.subtitle)}</span>` : ''}
        </button>`).join('');
    }

    async function refreshSpotlightResults(input, stateObj) {
        const query = input.value || '';
        const settings = spotlightSettingsEntries().filter(entry => !query || entry.title.toLowerCase().includes(query.toLowerCase()));
        const apps = spotlightAppEntries(query);
        const recent = spotlightRecentFileEntries(query);
        const files = await spotlightFileEntries(query);
        stateObj.entries = []
            .concat(settings.map(entry => ({ id: entry.id, title: entry.title, subtitle: t('desktop.spotlight_settings'), action: entry.action })))
            .concat(recent)
            .concat(apps)
            .concat(files)
            .slice(0, 16);
        if (stateObj.activeIndex >= stateObj.entries.length) stateObj.activeIndex = Math.max(0, stateObj.entries.length - 1);
        renderSpotlightResults(stateObj.entries, stateObj.activeIndex);
    }

    function activateSpotlightIndex(stateObj) {
        const entry = stateObj.entries[stateObj.activeIndex];
        if (!entry || typeof entry.action !== 'function') return;
        closeSpotlight();
        entry.action();
    }

    function openSpotlight() {
        if (spotlightOpen) return;
        closeStartMenu();
        spotlightOpen = true;
        const backdrop = document.createElement('div');
        backdrop.id = 'vd-spotlight-backdrop';
        backdrop.className = 'vd-spotlight-backdrop';
        backdrop.innerHTML = `<div class="vd-spotlight">
            <label class="vd-spotlight-label">${esc(t('desktop.spotlight_title'))}</label>
            <input type="search" class="vd-spotlight-input" placeholder="${esc(t('desktop.spotlight_placeholder'))}" autocomplete="off" spellcheck="false">
            <div class="vd-spotlight-results" data-spotlight-results></div>
        </div>`;
        document.body.appendChild(backdrop);
        const input = backdrop.querySelector('.vd-spotlight-input');
        const stateObj = { entries: [], activeIndex: 0 };
        let timer = 0;
        const scheduleRefresh = () => {
            if (timer) window.clearTimeout(timer);
            timer = window.setTimeout(() => refreshSpotlightResults(input, stateObj), 120);
        };
        input.addEventListener('input', scheduleRefresh);
        input.addEventListener('keydown', event => {
            if (event.key === 'Escape') {
                event.preventDefault();
                closeSpotlight();
                return;
            }
            if (event.key === 'ArrowDown') {
                event.preventDefault();
                stateObj.activeIndex = Math.min(stateObj.entries.length - 1, stateObj.activeIndex + 1);
                renderSpotlightResults(stateObj.entries, stateObj.activeIndex);
                return;
            }
            if (event.key === 'ArrowUp') {
                event.preventDefault();
                stateObj.activeIndex = Math.max(0, stateObj.activeIndex - 1);
                renderSpotlightResults(stateObj.entries, stateObj.activeIndex);
                return;
            }
            if (event.key === 'Enter') {
                event.preventDefault();
                activateSpotlightIndex(stateObj);
            }
        });
        backdrop.addEventListener('click', event => {
            if (event.target === backdrop) closeSpotlight();
            const btn = event.target.closest('[data-spotlight-index]');
            if (!btn) return;
            stateObj.activeIndex = Number(btn.dataset.spotlightIndex);
            activateSpotlightIndex(stateObj);
        });
        refreshSpotlightResults(input, stateObj);
        input.focus();
    }
