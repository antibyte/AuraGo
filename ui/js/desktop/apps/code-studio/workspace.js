    // Plan C (SSH workspaces): switcher, remote chip, connect dialog, per-workspace sessions, switching, disconnect
    // banner and capability gating. Pure helpers: workspace-model.js (window.CodeStudioWorkspaceModel). This fragment
    // shares the Code Studio IIFE and only declares functions and constants; nothing runs before render().
    // Contract for plans B/D: state.workspaceId, activeWorkspace(), workspaceIsRemote(), workspaceHostLabel(),
    // workspaceHas(name), workspaceURL(url), workspaceSocketURL(path, instance).

    const WS_MODEL = window.CodeStudioWorkspaceModel;
    const WS_KEY_FILE_LIMIT = 64 * 1024;
    const WS_GLYPHS = {
        laptop: '<rect x="3" y="3.5" width="10" height="7" rx="1.2"/><path d="M1.5 12.5h13"/>',
        server: '<rect x="2.5" y="2.5" width="11" height="4.5" rx="1.2"/><rect x="2.5" y="9" width="11" height="4.5" rx="1.2"/><path d="M5 4.75h.01M5 11.25h.01"/>',
        chevron: '<path d="M4.5 6.5 8 10l3.5-3.5"/>',
        up: '<path d="M8 13V3.5M4 7.5 8 3.5l4 4"/>',
        home: '<path d="M2.5 7.5 8 3l5.5 4.5V13a.5.5 0 0 1-.5.5H9.5V10h-3v3.5H3a.5.5 0 0 1-.5-.5z"/>',
        plus: '<path d="M8 3v10M3 8h10"/>',
        plug: '<path d="M6 2.5v3M10 2.5v3M4.5 5.5h7v2a3.5 3.5 0 0 1-7 0zM8 11v2.5"/>',
        unplug: '<path d="M2.5 2.5l11 11M10 2.5v3M8 11v2.5M4.5 5.5v2a3.5 3.5 0 0 0 3.5 3.5"/>',
        folder: '<path d="M2 4.5A1.5 1.5 0 0 1 3.5 3h2.6l1.4 1.5h5A1.5 1.5 0 0 1 14 6v5.5a1.5 1.5 0 0 1-1.5 1.5h-9A1.5 1.5 0 0 1 2 11.5z"/>',
        file: '<path d="M4 2.5h5l3 3v8H4z"/><path d="M9 2.5v3h3"/>',
        key: '<circle cx="5.5" cy="10.5" r="3"/><path d="M7.6 8.4 13.5 2.5M11 5l1.5 1.5"/>',
        warning: '<path d="M8 2.5 14 13H2z"/><path d="M8 6.5v3M8 11.25h.01"/>',
        x: '<path d="M4 4l8 8M12 4l-8 8"/>',
        check: '<path d="M3 8.5 6.5 12 13 4.5"/>',
        refresh: '<path d="M13 8a5 5 0 1 1-1.5-3.5M13 2.5v3h-3"/>'
    };

    function wsGlyph(name, size) {
        const px = Number(size || 14);
        return `<svg class="cs-ws-glyph" viewBox="0 0 16 16" width="${px}" height="${px}" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">${WS_GLYPHS[name] || ''}</svg>`;
    }

    function workspaceIsRemote(instance = state) { return !!instance && WS_MODEL.isRemote(instance.workspaceId); }
    function activeWorkspace(instance = state) { return (instance && instance.workspace) || WS_MODEL.localWorkspace(); }
    function workspaceHostLabel(ws = activeWorkspace()) { return WS_MODEL.userAtHost(ws); }
    function workspaceDisplayLabel(ws = activeWorkspace()) {
        return ws && ws.kind === 'ssh' ? WS_MODEL.workspaceLabel(ws) : tr('codeStudio.workspaceLocal', 'Local workspace');
    }
    function workspaceURL(url, instance = state || currentInstance()) {
        return WS_MODEL.withWorkspaceQuery(url, instance ? instance.workspaceId : WS_MODEL.LOCAL_ID);
    }
    function workspaceSocketURL(path, instance = state || currentInstance()) {
        return WS_MODEL.socketURL(location, path, instance ? instance.workspaceId : WS_MODEL.LOCAL_ID);
    }
    // Capabilities gate a feature only when the host reported them; unknown means available.
    function workspaceHas(name, instance = state) {
        if (!workspaceIsRemote(instance)) return true;
        const caps = activeWorkspace(instance).capabilities;
        return !caps || caps[name] !== false;
    }
    function workspaceChipState(instance = state) {
        if (!instance) return 'local';
        if (instance.workspaceSwitchTarget) return 'connecting';
        return workspaceIsRemote(instance) ? (instance.workspaceStatus || 'connected') : 'local';
    }
    function workspaceStatusText(status) {
        switch (status) {
            case 'connected': return tr('codeStudio.workspaceStatusConnected', 'Connected');
            case 'connecting': return tr('codeStudio.workspaceStatusConnecting', 'Connecting…');
            case 'lost': return tr('codeStudio.workspaceStatusLost', 'Connection lost');
            case 'hostkey': return tr('codeStudio.workspaceTrustTitle', 'Unknown host key');
            case 'error': return tr('codeStudio.workspaceStatusError', 'Connection failed');
            case 'disconnected': return tr('codeStudio.workspaceStatusDisconnected', 'Not connected');
            default: return tr('codeStudio.workspaceLocal', 'Local workspace');
        }
    }
    function workspaceDotState(ws) {
        if (state && ws.id === state.workspaceId) return workspaceChipState();
        return ws.connected ? 'connected' : 'disconnected';
    }
    function workspaceErrorText(kind, details) {
        const vars = Object.assign({ host: workspaceHostLabel(), path: '', error: '' }, details || {});
        switch (kind) {
            case 'host_key_mismatch': return tr('codeStudio.workspaceErrorHostKeyMismatch', 'The host key of {{host}} has changed. Code Studio does not connect until known_hosts is fixed.', vars);
            case 'auth': return tr('codeStudio.workspaceErrorAuth', 'Sign-in to {{host}} failed. Check the user name and the password or key.', vars);
            case 'unreachable': return tr('codeStudio.workspaceErrorUnreachable', '{{host}} cannot be reached.', vars);
            case 'not_directory': return tr('codeStudio.workspaceErrorNotDirectory', '{{path}} is not a folder.', vars);
            case 'not_found': return tr('codeStudio.workspaceErrorNotFound', '{{path}} does not exist.', vars);
            case 'workspace_not_found': return tr('codeStudio.workspaceMissing', 'This SSH workspace no longer exists. Code Studio switched to the local workspace.', vars);
            default: return tr('codeStudio.workspaceErrorOther', 'Something went wrong: {{error}}', vars);
        }
    }
    function workspaceFormErrorText(code) {
        if (code === 'host') return tr('codeStudio.workspaceFormBadHost', 'Enter a host name or IP address, without user or scheme');
        if (code === 'port') return tr('codeStudio.workspaceFormBadPort', 'Port 1–65535');
        if (code === 'key') return tr('codeStudio.workspaceFormBadKey', 'Paste a private key that starts with -----BEGIN');
        return tr('codeStudio.workspaceFormRequired', 'Required');
    }
    function workspaceLastUsedText(ts) {
        const rel = WS_MODEL.relativeTime(ts, Date.now());
        if (!rel) return tr('codeStudio.workspaceNeverUsed', 'Never used');
        try {
            return new Intl.RelativeTimeFormat(document.documentElement.lang || undefined, { numeric: 'auto' }).format(rel.value, rel.unit);
        } catch (_) {
            return new Date(ts).toLocaleDateString();
        }
    }

    // Called by api() in core.js for a failed scoped request of the given workspace.
    function workspaceNoteRequestError(instance, workspaceId, err) {
        if (!isLiveInstance(instance) || !WS_MODEL.isRemote(workspaceId) || instance.workspaceId !== workspaceId) return;
        const kind = WS_MODEL.classifyError(err);
        if (kind === 'unreachable') runWithInstance(instance, () => workspaceMarkLost(err));
        else if (kind === 'workspace_not_found') runWithInstance(instance, workspaceForgetMissing);
    }

    function workspaceApplySession(id) {
        const session = WS_MODEL.readSessions(localStorage)[id];
        if (!session) {
            // A new SSH workspace starts empty; local without a stored session keeps the tabs loadState read.
            if (WS_MODEL.isRemote(id)) {
                state.openTabs = [];
                state.activeTabIndex = -1;
                state.recentFiles = [];
                restoreSplitPreference(null);
            }
            return;
        }
        const paths = normalizePersistedPaths(session.openTabs);
        state.openTabs = paths.map(path => ({ path, content: '', modified: false, language: languageForPath(path), view: null }));
        state.activeTabIndex = paths.length ? Math.min(Math.max(session.activeTabIndex, 0), paths.length - 1) : -1;
        state.recentFiles = normalizePersistedPaths(session.recentFiles);
        restoreSplitPreference(session.split);
    }

    function workspaceSaveSession(session) {
        WS_MODEL.writeSession(localStorage, state.workspaceId, session, Date.now());
    }

    // The desktop window subtitle shows user@host:~/path (the Files app uses the same slot for its folder).
    function workspaceUpdateWindowTitle() {
        const win = state && state.root && typeof state.root.closest === 'function' ? state.root.closest('.vd-window') : null;
        const subtitle = win && win.querySelector('.vd-window-subtitle');
        if (!subtitle) return;
        const ws = activeWorkspace();
        subtitle.textContent = workspaceIsRemote() ? workspaceHostLabel(ws) + ':' + WS_MODEL.displayRoot(ws) : '';
        subtitle.title = workspaceIsRemote() ? workspaceHostLabel(ws) + ':' + ws.root : '';
    }
    function workspaceTerminalLabel(name) { return workspaceIsRemote() ? workspaceHostLabel() + ' · ' + name : name; }
    function workspaceTerminalBanner() {
        return workspaceIsRemote() ? ' (' + workspaceHostLabel() + ':' + WS_MODEL.displayRoot(activeWorkspace()) + ')' : '';
    }
    function workspaceToast(message) {
        const notify = state && state.context && state.context.notify;
        if (typeof notify === 'function') notify({ title: tr('codeStudio.title', 'Code Studio'), message });
        flashStatus(message, 6000);
    }
    // A failed save keeps the tab dirty (saveTab never clears modified on error); remote workspaces also get a toast.
    function workspaceSaveFailed(tab, err) {
        if (!workspaceIsRemote() || !tab) return;
        const kind = WS_MODEL.classifyError(err);
        const error = kind === 'unreachable' ? workspaceErrorText('unreachable') : (err && err.message || String(err || ''));
        workspaceToast(tr('codeStudio.workspaceSaveFailedToast', '{{file}} was not saved. {{error}} Your changes stay in the tab.', { file: baseName(tab.path), error }));
    }

    // --- Chrome: status chip, Explorer switcher, banner -----------------------------------------------

    function workspaceMenuOpenFor(kind) {
        return !!(state && state.workspaceMenu && state.workspaceMenu.__anchorKind === kind);
    }

    function workspaceStatusChipMarkup() {
        const remote = workspaceIsRemote();
        const status = workspaceChipState();
        const ws = activeWorkspace();
        const text = remote ? tr('codeStudio.workspaceChipRemote', 'SSH: {{name}}', { name: ws.deviceName || ws.host }) : tr('codeStudio.workspaceChipLocal', 'Local');
        const title = remote ? workspaceDisplayLabel(ws) + ' — ' + workspaceHostLabel(ws) + '\n' + workspaceStatusText(status) : tr('codeStudio.workspaceLocalSub', 'Container · /workspace');
        const icon = status === 'connecting'
            ? '<span class="cs-ws-spinner" aria-hidden="true"></span>'
            : wsGlyph(['lost', 'error', 'hostkey'].includes(status) ? 'warning' : (remote ? 'server' : 'laptop'), 12);
        return `<span class="cs-status-item cs-status-remote" data-clickable data-status-remote data-remote-state="${esc(status)}" role="button" tabindex="0" aria-haspopup="menu" aria-expanded="${workspaceMenuOpenFor('chip')}" aria-label="${esc(text + ', ' + workspaceStatusText(status))}" title="${esc(title)}">${icon}<span class="cs-status-remote-label">${esc(text)}</span></span>`;
    }

    function wireWorkspaceChip(statusEl, refocus) {
        const chip = statusEl.querySelector('[data-status-remote]');
        if (!chip) return;
        chip.addEventListener('click', bind(() => openWorkspaceMenu(chip)));
        chip.addEventListener('keydown', bind(event => {
            if (!['Enter', ' ', 'ArrowUp'].includes(event.key)) return;
            event.preventDefault();
            openWorkspaceMenu(chip);
        }));
        if (refocus) chip.focus();
    }

    function workspaceSwitcherMarkup() {
        const ws = activeWorkspace();
        const remote = workspaceIsRemote();
        const status = workspaceChipState();
        const sub = remote ? workspaceHostLabel(ws) : tr('codeStudio.workspaceLocalSub', 'Container · /workspace');
        return `<button type="button" class="cs-ws-switcher" data-ws-switcher data-remote-state="${esc(status)}" aria-haspopup="menu" aria-expanded="${workspaceMenuOpenFor('switcher')}" title="${esc(tr('codeStudio.workspaceMenu', 'Workspaces'))}">
            <span class="cs-ws-switcher-icon">${status === 'connecting' ? '<span class="cs-ws-spinner" aria-hidden="true"></span>' : wsGlyph(remote ? 'server' : 'laptop', 16)}</span>
            <span class="cs-ws-switcher-text"><span class="cs-ws-switcher-label">${esc(workspaceDisplayLabel(ws))}</span><span class="cs-ws-switcher-sub">${esc(sub)}</span></span>
            ${remote ? `<span class="cs-ws-dot" data-ws-dot="${esc(workspaceDotState(ws))}" aria-hidden="true"></span>` : ''}
            <span class="cs-ws-switcher-chevron">${wsGlyph('chevron', 14)}</span>
        </button>`;
    }

    function wireWorkspaceSwitcher(root) {
        const button = root && root.querySelector('[data-ws-switcher]');
        if (!button || button._wsWired) return;
        button._wsWired = true;
        button.addEventListener('click', bind(() => openWorkspaceMenu(button)));
        button.addEventListener('keydown', bind(event => {
            if (event.key !== 'ArrowDown') return;
            event.preventDefault();
            openWorkspaceMenu(button);
        }));
    }

    function workspaceRenderSwitcher() {
        const current = shellPart('[data-ws-switcher]');
        if (!current) return;
        const hadFocus = document.activeElement === current;
        const holder = document.createElement('div');
        holder.innerHTML = workspaceSwitcherMarkup();
        const next = holder.firstElementChild;
        current.replaceWith(next);
        wireWorkspaceSwitcher(next.parentElement);
        if (hadFocus) next.focus();
    }

    function workspaceRootCrumbLabel() {
        if (!workspaceIsRemote()) return tr('codeStudio.workspaceRoot', 'Workspace');
        const root = activeWorkspace().root;
        return root === '/' ? '/' : root.split('/').filter(Boolean).pop();
    }

    function workspaceRenderBanner() {
        const banner = shellPart('[data-ws-banner]');
        if (!banner) return;
        const status = state.workspaceStatus;
        const show = workspaceIsRemote() && !state.workspaceSwitching
            && (['lost', 'hostkey', 'error'].includes(status) || (status === 'connecting' && state.workspaceWasLost));
        if (!show) {
            if (!banner.hidden) {
                banner.hidden = true;
                banner.innerHTML = '';
                banner.dataset.signature = '';
                delete banner.dataset.wsBannerState;
            }
            return;
        }
        const host = workspaceHostLabel();
        const error = state.workspaceError || {};
        let text;
        let actions;
        if (status === 'connecting') {
            text = tr('codeStudio.workspaceStatusConnecting', 'Connecting…') + ' ' + host;
            actions = [];
        } else if (status === 'lost') {
            text = tr('codeStudio.workspaceLost', 'Connection to {{host}} lost', { host });
            actions = ['reconnect', 'local'];
        } else if (status === 'hostkey') {
            text = tr('codeStudio.workspaceTrustTitle', 'Unknown host key') + ': ' + host;
            actions = ['review', 'local'];
        } else {
            text = workspaceErrorText(error.kind, { host, path: activeWorkspace().root, error: error.message || '' });
            actions = error.kind === 'host_key_mismatch' ? ['local'] : ['reconnect', 'local'];
        }
        const seconds = status === 'lost' && state.workspaceRetryAt ? Math.max(0, Math.ceil((state.workspaceRetryAt - Date.now()) / 1000)) : -1;
        const detail = seconds >= 0 ? tr('codeStudio.workspaceRetryIn', 'Retrying in {{seconds}} s', { seconds }) : '';
        const signature = [status, text, actions.join(',')].join('|');
        banner.hidden = false;
        banner.dataset.wsBannerState = status;
        if (banner.dataset.signature === signature) {
            // The countdown tick changes only the detail, so focus and screen readers stay calm.
            const countdown = banner.querySelector('[data-ws-banner-detail]');
            if (countdown) countdown.textContent = detail;
            return;
        }
        banner.dataset.signature = signature;
        banner.setAttribute('role', status === 'error' ? 'alert' : 'status');
        const labels = {
            reconnect: tr('codeStudio.workspaceReconnect', 'Reconnect'),
            local: tr('codeStudio.workspaceUseLocal', 'Use local workspace'),
            review: tr('codeStudio.workspaceReviewHostKey', 'Review host key…')
        };
        banner.innerHTML = `<span class="cs-ws-banner-icon">${status === 'connecting' ? '<span class="cs-ws-spinner" aria-hidden="true"></span>' : wsGlyph('warning', 16)}</span>
            <span class="cs-ws-banner-text"><strong>${esc(text)}</strong><span class="cs-ws-banner-detail" data-ws-banner-detail aria-hidden="true">${esc(detail)}</span></span>
            <span class="cs-ws-banner-actions">${actions.map(action => `<button type="button" class="cs-button${action === 'local' ? '' : ' primary'}" data-ws-banner-action="${action}">${esc(labels[action])}</button>`).join('')}</span>`;
        banner.querySelectorAll('[data-ws-banner-action]').forEach(button => button.addEventListener('click', bind(() => workspaceBannerAction(button.dataset.wsBannerAction))));
    }

    function workspaceBannerAction(action) {
        if (action === 'reconnect') workspaceReconnect();
        else if (action === 'local') switchWorkspace(WS_MODEL.LOCAL_ID);
        else if (action === 'review') {
            const ws = activeWorkspace();
            openConnectDialog({ deviceId: ws.deviceId, path: ws.root });
        }
    }

    function workspaceRenderChrome() {
        if (!state || !studioRoot()) return;
        workspaceRenderSwitcher();
        renderStatus();
        workspaceRenderBanner();
        workspaceUpdateWindowTitle();
    }

    // --- Workspace menu ------------------------------------------------------------------------

    async function workspaceLoadList(instance = state) {
        const target = instance;
        try {
            const payload = await apiClient.workspaces();
            if (!isLiveInstance(target)) return [];
            target.workspaces = WS_MODEL.normalizeWorkspaceList(payload);
            if (workspaceIsRemote(target)) {
                const fresh = target.workspaces.find(ws => ws.id === target.workspaceId);
                if (fresh) target.workspace = Object.assign({}, target.workspace, fresh, { capabilities: fresh.capabilities || (target.workspace && target.workspace.capabilities) || null });
            }
        } catch (_) {
            if (isLiveInstance(target) && !(target.workspaces || []).length) target.workspaces = [WS_MODEL.localWorkspace()];
        }
        return target.workspaces;
    }

    function closeWorkspaceMenu(instance = state, restoreFocus = false) {
        const menu = instance && instance.workspaceMenu;
        if (menu && typeof menu.__codeStudioCleanup === 'function') menu.__codeStudioCleanup(restoreFocus);
    }

    async function openWorkspaceMenu(anchor) {
        const instance = state;
        if (!isLiveInstance(instance) || !anchor) return;
        const kind = anchor.hasAttribute('data-status-remote') ? 'chip' : 'switcher';
        if (instance.workspaceMenu) {
            const same = instance.workspaceMenu.__anchorKind === kind;
            closeWorkspaceMenu(instance, same);
            if (same) return;
        }
        document.querySelectorAll('.cs-context-menu').forEach(other => {
            if (typeof other.__codeStudioCleanup === 'function') other.__codeStudioCleanup();
            else other.remove();
        });
        const menu = document.createElement('div');
        menu.className = 'cs-context-menu cs-ws-menu';
        menu.setAttribute('role', 'menu');
        menu.setAttribute('aria-label', tr('codeStudio.workspaceMenu', 'Workspaces'));
        menu.__anchorKind = kind;
        document.body.appendChild(menu);
        instance.workspaceMenu = menu;
        anchor.setAttribute('aria-expanded', 'true');
        let closed = false;
        let unregister = () => {};
        // The status bar and the sidebar re-render; the anchor is looked up again when the old element is gone.
        const currentAnchor = () => (anchor.isConnected ? anchor : (isLiveInstance(instance) ? runWithInstance(instance, () => shellPart(kind === 'chip' ? '[data-status-remote]' : '[data-ws-switcher]')) : null));
        const onPointer = event => {
            const owner = currentAnchor();
            if (!menu.contains(event.target) && !(owner && owner.contains(event.target))) cleanup(false);
        };
        const onResize = () => cleanup(false);
        const cleanup = restoreFocus => {
            if (closed) return;
            closed = true;
            unregister();
            document.removeEventListener('mousedown', onPointer, true);
            window.removeEventListener('resize', onResize);
            menu.remove();
            if (instance.workspaceMenu === menu) instance.workspaceMenu = null;
            const owner = currentAnchor();
            if (owner) owner.setAttribute('aria-expanded', 'false');
            if (restoreFocus && owner) owner.focus();
        };
        menu.__codeStudioCleanup = cleanup;
        unregister = registerDisposer(() => cleanup(false));
        menu.addEventListener('keydown', bindInstance(instance, event => workspaceMenuKeydown(event, menu, cleanup)));
        setTimeout(() => {
            if (closed) return;
            document.addEventListener('mousedown', onPointer, true);
            window.addEventListener('resize', onResize);
        }, 0);
        renderWorkspaceMenu(menu, cleanup, true);
        positionWorkspaceMenu(menu, anchor);
        await workspaceLoadList(instance);
        if (closed || !isLiveInstance(instance)) return;
        runWithInstance(instance, () => {
            renderWorkspaceMenu(menu, cleanup, menu.contains(document.activeElement));
            const owner = currentAnchor();
            if (owner) positionWorkspaceMenu(menu, owner);
        });
    }

    function positionWorkspaceMenu(menu, anchor) {
        const rect = anchor.getBoundingClientRect();
        menu.style.minWidth = Math.round(Math.max(280, Math.min(rect.width, 440))) + 'px';
        const box = menu.getBoundingClientRect();
        const below = rect.bottom + 4;
        const top = below + box.height <= window.innerHeight - 8 ? below : Math.max(8, rect.top - box.height - 4);
        menu.style.left = Math.round(Math.max(8, Math.min(rect.left, window.innerWidth - box.width - 8))) + 'px';
        menu.style.top = Math.round(top) + 'px';
    }

    function workspaceMenuItemMarkup(ws, currentId) {
        const checked = ws.id === currentId;
        const remote = ws.kind === 'ssh';
        const label = workspaceDisplayLabel(ws);
        const sub = remote ? workspaceHostLabel(ws) + ':' + WS_MODEL.displayRoot(ws) : tr('codeStudio.workspaceLocalSub', 'Container · /workspace');
        const dot = remote ? workspaceDotState(ws) : '';
        const aria = [label, sub, remote ? workspaceStatusText(dot) : ''].filter(Boolean).join(', ');
        const remove = remote
            ? `<button type="button" class="cs-ws-menu-remove" data-ws-remove="${esc(ws.id)}" tabindex="-1" aria-label="${esc(tr('codeStudio.workspaceRemove', 'Remove') + ': ' + label)}" title="${esc(tr('codeStudio.workspaceRemove', 'Remove'))}">${wsGlyph('x', 12)}</button>`
            : '';
        return `<div class="cs-ws-menu-row${checked ? ' is-current' : ''}" role="none">
            <button type="button" class="cs-ws-menu-item" role="menuitemradio" aria-checked="${checked}" tabindex="-1" data-ws-id="${esc(ws.id)}" aria-label="${esc(aria)}" title="${esc(remote ? workspaceHostLabel(ws) + ':' + ws.root : sub)}">
                <span class="cs-ws-menu-icon">${wsGlyph(remote ? 'server' : 'laptop', 15)}</span>
                <span class="cs-ws-menu-text"><span class="cs-ws-menu-label">${esc(label)}</span><span class="cs-ws-menu-sub">${esc(sub)}</span></span>
                ${remote ? `<span class="cs-ws-dot" data-ws-dot="${esc(dot)}" aria-hidden="true"></span>` : ''}
                <span class="cs-ws-menu-check" aria-hidden="true">${checked ? wsGlyph('check', 13) : ''}</span>
            </button>${remove}
        </div>`;
    }

    function workspaceMenuActionMarkup(action, glyph, label) {
        return `<button type="button" class="cs-ws-menu-item" role="menuitem" tabindex="-1" data-ws-action="${esc(action)}"><span class="cs-ws-menu-icon">${wsGlyph(glyph, 15)}</span><span class="cs-ws-menu-text"><span class="cs-ws-menu-label">${esc(label)}</span></span></button>`;
    }

    function renderWorkspaceMenu(menu, cleanup, focus) {
        const instance = state;
        const active = menu.contains(document.activeElement) ? document.activeElement : null;
        const focusedKey = active ? (active.dataset.wsId || active.dataset.wsAction || '') : '';
        const remotes = (instance.workspaces || []).filter(ws => ws.kind === 'ssh');
        const separator = '<div class="cs-context-separator" role="separator"></div>';
        const rows = [`<div class="cs-ws-menu-heading" role="presentation">${esc(tr('codeStudio.workspaceMenu', 'Workspaces'))}</div>`,
            workspaceMenuItemMarkup(WS_MODEL.localWorkspace(), instance.workspaceId)];
        if (remotes.length) {
            rows.push(separator, `<div class="cs-ws-menu-heading" role="presentation">${esc(tr('codeStudio.workspaceRecent', 'Recent SSH folders'))}</div>`);
            remotes.forEach(ws => rows.push(workspaceMenuItemMarkup(ws, instance.workspaceId)));
        }
        rows.push(separator, workspaceMenuActionMarkup('connect', 'plus', tr('codeStudio.workspaceConnect', 'Connect to SSH Folder…')));
        if (workspaceIsRemote()) {
            rows.push(workspaceMenuActionMarkup('reconnect', 'plug', tr('codeStudio.workspaceReconnect', 'Reconnect')));
            rows.push(workspaceMenuActionMarkup('disconnect', 'unplug', tr('codeStudio.workspaceDisconnect', 'Disconnect')));
        }
        menu.innerHTML = rows.join('');
        menu.querySelectorAll('[data-ws-id]').forEach(button => button.addEventListener('click', bindInstance(instance, () => { cleanup(true); switchWorkspace(button.dataset.wsId); })));
        menu.querySelectorAll('[data-ws-remove]').forEach(button => button.addEventListener('click', bindInstance(instance, event => { event.stopPropagation(); cleanup(true); workspaceRemove(button.dataset.wsRemove); })));
        menu.querySelectorAll('[data-ws-action]').forEach(button => button.addEventListener('click', bindInstance(instance, () => { cleanup(true); workspaceMenuAction(button.dataset.wsAction); })));
        if (!focus) return;
        const items = Array.from(menu.querySelectorAll('[data-ws-id], [data-ws-action]'));
        const target = (focusedKey && items.find(item => (item.dataset.wsId || item.dataset.wsAction) === focusedKey)) || menu.querySelector('[aria-checked="true"]') || items[0];
        if (target) target.focus();
    }

    function workspaceMenuKeydown(event, menu, cleanup) {
        const items = Array.from(menu.querySelectorAll('.cs-ws-menu-item:not([disabled])'));
        const active = document.activeElement;
        const current = active && active.classList.contains('cs-ws-menu-remove') ? active.parentElement.querySelector('.cs-ws-menu-item') : active;
        const index = items.indexOf(current);
        const remove = current && current.parentElement ? current.parentElement.querySelector('.cs-ws-menu-remove') : null;
        switch (event.key) {
            case 'ArrowDown':
            case 'ArrowUp': {
                event.preventDefault();
                const delta = event.key === 'ArrowDown' ? 1 : -1;
                const next = items[index < 0 ? 0 : (index + delta + items.length) % items.length];
                if (next) next.focus();
                break;
            }
            case 'Home': event.preventDefault(); if (items[0]) items[0].focus(); break;
            case 'End': event.preventDefault(); if (items.length) items[items.length - 1].focus(); break;
            case 'ArrowRight': if (remove) { event.preventDefault(); remove.focus(); } break;
            case 'ArrowLeft': if (active !== current && current) { event.preventDefault(); current.focus(); } break;
            case 'Delete':
            case 'Backspace': if (remove) { event.preventDefault(); remove.click(); } break;
            case 'Escape': event.preventDefault(); event.stopPropagation(); cleanup(true); break;
            case 'Tab': event.preventDefault(); cleanup(true); break;
            default: break;
        }
    }

    function workspaceMenuAction(action) {
        if (action === 'connect') openConnectDialog();
        else if (action === 'reconnect') workspaceReconnect();
        else if (action === 'disconnect') switchWorkspace(WS_MODEL.LOCAL_ID);
    }

    async function workspaceRemove(id) {
        const target = state;
        const ws = (target.workspaces || []).find(item => item.id === id);
        if (!ws) return;
        const confirmed = await confirmValue(tr('codeStudio.workspaceRemoveConfirm', 'Remove {{workspace}} from the list? The files on the host stay untouched.', { workspace: workspaceDisplayLabel(ws) }), { confirmLabel: tr('codeStudio.workspaceRemove', 'Remove'), confirmIcon: 'trash' });
        if (!confirmed || !isLiveInstance(target)) return;
        if (target.workspaceId === id && !await runWithInstance(target, () => switchWorkspace(WS_MODEL.LOCAL_ID))) return;
        try {
            await apiClient.deleteWorkspace(id);
        } catch (err) {
            if (!isLiveInstance(target)) return;
            if (err.status !== 404) {
                runWithInstance(target, () => workspaceToast(workspaceErrorText('other', { error: err.message })));
                return;
            }
        }
        if (!isLiveInstance(target)) return;
        WS_MODEL.forgetSession(localStorage, id);
        target.workspaces = (target.workspaces || []).filter(item => item.id !== id);
        runWithInstance(target, workspaceRenderChrome);
    }

    function openWorkspaceSwitcher() {
        const switcher = shellPart('[data-ws-switcher]');
        const anchor = switcher && state.sidebarVisible && switcher.getClientRects().length ? switcher : shellPart('[data-status-remote]');
        if (anchor) openWorkspaceMenu(anchor);
    }

    // --- Boot, connect, reconnect --------------------------------------------------------------

    async function workspaceBoot(instance) {
        const target = instance || state;
        if (!isLiveInstance(target)) return;
        addDisposer(target, () => workspaceClearRetry(target));
        const wanted = WS_MODEL.readActive(localStorage);
        await workspaceLoadList(target);
        if (!isLiveInstance(target)) return;
        const saved = wanted !== WS_MODEL.LOCAL_ID ? (target.workspaces || []).find(ws => ws.id === wanted) : null;
        runWithInstance(target, () => {
            if (saved) {
                target.workspaceId = saved.id;
                target.workspace = saved;
                target.workspaceStatus = 'connecting';
            } else if (wanted !== WS_MODEL.LOCAL_ID) {
                WS_MODEL.writeActive(localStorage, WS_MODEL.LOCAL_ID);
            }
            workspaceApplySession(target.workspaceId);
            renderTabs();
            workspaceRenderChrome();
        });
        if (saved) await workspaceConnectActive(target);
    }

    async function workspaceConnectActive(target, options = {}) {
        const id = target.workspaceId;
        if (!WS_MODEL.isRemote(id)) return true;
        runWithInstance(target, () => { target.workspaceStatus = 'connecting'; workspaceRenderChrome(); });
        try {
            const payload = await apiClient.connectWorkspace(id, options.trust || '');
            if (!isLiveInstance(target) || target.workspaceId !== id) return false;
            runWithInstance(target, () => {
                target.workspace = WS_MODEL.mergeWorkspace(target.workspace, payload);
                target.workspaceStatus = 'connected';
                target.workspaceError = null;
                workspaceRenderChrome();
                renderGitPanel();
            });
            return true;
        } catch (err) {
            if (isLiveInstance(target) && target.workspaceId === id) runWithInstance(target, () => workspaceConnectFailed(err));
            return false;
        }
    }

    function workspaceConnectFailed(err) {
        const instance = state;
        const kind = WS_MODEL.classifyError(err);
        if (kind === 'workspace_not_found') {
            workspaceForgetMissing();
            return;
        }
        instance.workspaceError = { kind, message: err && err.message ? err.message : String(err || '') };
        instance.workspaceWasLost = true;
        if (kind === 'unknown_host_key') {
            instance.workspaceStatus = 'hostkey';
            workspaceClearRetry(instance);
        } else if (kind !== 'unreachable') {
            // Contract fix: host_key_mismatch, auth_failed, device_not_found, remote_scope_required, invalid_root ...
            // will not heal by retrying; only a broken connection backs off and redials.
            instance.workspaceStatus = 'error';
            workspaceClearRetry(instance);
        } else {
            instance.workspaceStatus = 'lost';
            workspaceScheduleRetry();
        }
        workspaceRenderChrome();
    }

    function workspaceMarkLost(err) {
        const instance = state;
        if (instance.workspaceSwitching || ['lost', 'connecting', 'hostkey', 'error'].includes(instance.workspaceStatus)) return;
        workspaceConnectFailed(err);
    }

    function workspaceClearRetry(instance = state, keepAttempt = false) {
        if (!instance) return;
        if (instance.workspaceRetryTimer) clearTimeout(instance.workspaceRetryTimer);
        if (instance.workspaceRetryTick) clearInterval(instance.workspaceRetryTick);
        instance.workspaceRetryTimer = 0;
        instance.workspaceRetryTick = 0;
        instance.workspaceRetryAt = 0;
        if (!keepAttempt) instance.workspaceRetryAttempt = 0;
    }

    function workspaceScheduleRetry() {
        const instance = state;
        workspaceClearRetry(instance, true);
        const attempt = instance.workspaceRetryAttempt || 0;
        const delay = WS_MODEL.backoffDelay(attempt);
        instance.workspaceRetryAt = Date.now() + delay;
        instance.workspaceRetryTick = setInterval(bindInstance(instance, workspaceRenderBanner), 1000);
        instance.workspaceRetryTimer = setTimeout(bindInstance(instance, () => {
            workspaceClearRetry(instance, true);
            instance.workspaceRetryAttempt = attempt + 1;
            workspaceReconnect({ auto: true });
        }), delay);
    }

    async function workspaceReconnect(options = {}) {
        const target = state;
        if (!workspaceIsRemote(target) || target.workspaceStatus === 'connecting' || target.workspaceSwitching) return false;
        if (!options.auto) {
            workspaceClearRetry(target);
            target.workspaceWasLost = true; // a manual reconnect refreshes everything
        }
        const ok = await workspaceConnectActive(target, options);
        if (ok && isLiveInstance(target)) runWithInstance(target, workspaceAfterReconnect);
        return ok;
    }

    function workspaceAfterReconnect() {
        const instance = state;
        const wasLost = instance.workspaceWasLost;
        instance.workspaceWasLost = false;
        workspaceClearRetry(instance);
        workspaceRenderChrome();
        if (!wasLost) return;
        flashStatus(tr('codeStudio.workspaceReconnected', 'Reconnected to {{host}}', { host: workspaceHostLabel() }));
        refreshFiles(instance.currentPath);
        scheduleGitRefresh();
        const open = typeof WebSocket !== 'undefined' ? WebSocket.OPEN : 1;
        if (canWrite() && (instance.terminalSessions || []).every(session => !session.ws || session.ws.readyState !== open)) {
            closeTerminalSessionSockets(instance);
            renderTerminal();
            connectTerminal();
        }
        if (instance.lsp) {
            Object.keys(instance.lsp.connections).forEach(language => {
                const conn = instance.lsp.connections[language];
                if (conn.state === 'error' || conn.state === 'disconnected') lspRestart(language);
            });
        }
        instance.openTabs.forEach(tab => reloadCleanTabFromDisk(instance, tab));
    }

    // A remote shell that closes may mean the SSH connection broke: one connect call tells (and redials).
    function workspaceTerminalClosed() {
        const instance = state;
        if (!workspaceIsRemote(instance) || instance.workspaceStatus !== 'connected' || instance.workspaceSwitching) return;
        const id = instance.workspaceId;
        apiClient.connectWorkspace(id).then(payload => {
            if (isLiveInstance(instance) && instance.workspaceId === id) instance.workspace = WS_MODEL.mergeWorkspace(instance.workspace, payload);
        }, err => {
            if (isLiveInstance(instance) && instance.workspaceId === id) runWithInstance(instance, () => workspaceConnectFailed(err));
        });
    }

    function workspaceForgetMissing() {
        const instance = state;
        if (instance.workspaceForgetting) return;
        instance.workspaceForgetting = true;
        const id = instance.workspaceId;
        instance.workspaces = (instance.workspaces || []).filter(ws => ws.id !== id);
        workspaceToast(tr('codeStudio.workspaceMissing', 'This SSH workspace no longer exists. Code Studio switched to the local workspace.'));
        switchWorkspace(WS_MODEL.LOCAL_ID).then(ok => {
            if (ok) WS_MODEL.forgetSession(localStorage, id);
        }).finally(() => { instance.workspaceForgetting = false; });
    }

    // --- Switching -----------------------------------------------------------------------------

    function chooseWorkspaceSwitch(count, label) {
        return new Promise(resolve => {
            const instance = state;
            const overlay = document.createElement('div');
            overlay.className = 'cs-modal-backdrop';
            overlay.innerHTML = `<div class="cs-modal cs-ws-dirty" role="alertdialog" aria-modal="true" aria-labelledby="cs-ws-dirty-title" aria-describedby="cs-ws-dirty-text">
                <h2 class="cs-ws-dialog-title" id="cs-ws-dirty-title">${esc(tr('codeStudio.workspaceDirtyTitle', 'Unsaved changes'))}</h2>
                <p id="cs-ws-dirty-text">${esc(tr('codeStudio.workspaceDirtyPrompt', '{{count}} files have unsaved changes. Save them before switching to {{workspace}}?', { count, workspace: label }))}</p>
                <div class="cs-modal-actions">
                    <button type="button" class="cs-button" data-choice="cancel">${esc(tr('desktop.cancel', 'Cancel'))}</button>
                    <button type="button" class="cs-button danger" data-choice="discard">${esc(tr('codeStudio.discard', 'Discard'))}</button>
                    <button type="button" class="cs-button primary" data-choice="save"${canWrite(instance) ? '' : ' disabled'}>${esc(tr('codeStudio.saveAll', 'Save All'))}</button>
                </div>
            </div>`;
            document.body.appendChild(overlay);
            const release = trapDialogFocus(overlay);
            let settled = false;
            let unregister = () => {};
            const done = choice => {
                if (settled) return;
                settled = true;
                unregister();
                overlay.remove();
                release();
                resolve(choice);
            };
            runWithInstance(instance, () => { unregister = registerDisposer(() => done('cancel')); });
            overlay.querySelectorAll('[data-choice]').forEach(button => button.addEventListener('click', () => done(button.dataset.choice)));
            overlay.addEventListener('click', event => { if (event.target === overlay) done('cancel'); });
            overlay.addEventListener('keydown', event => {
                if (event.key !== 'Escape') return;
                event.preventDefault();
                event.stopPropagation();
                done('cancel');
            });
            (overlay.querySelector('[data-choice="save"]:not([disabled])') || overlay.querySelector('[data-choice="cancel"]')).focus();
        });
    }

    // Dirty tabs ask Save all / Discard / Cancel; pending saves finish before the workspace id changes.
    async function workspaceSettleTabs(target, next) {
        const dirty = target.openTabs.filter(tab => tab.modified && !isVirtualTab(tab));
        if (dirty.length) {
            const choice = await runWithInstance(target, () => chooseWorkspaceSwitch(dirty.length, workspaceDisplayLabel(next)));
            if (choice === 'cancel' || !isLiveInstance(target)) return false;
            if (choice === 'save' && !await runWithInstance(target, () => saveAllFiles())) {
                if (isLiveInstance(target)) runWithInstance(target, () => workspaceToast(tr('codeStudio.workspaceSwitchFailed', 'Could not open {{workspace}}: {{error}}', { workspace: workspaceDisplayLabel(next), error: tr('codeStudio.unsavedChanges', 'Unsaved changes') })));
                return false;
            }
        }
        await Promise.all(target.openTabs.map(tab => tab.pendingSave).filter(Boolean));
        return isLiveInstance(target) && !target.openTabs.some(tab => tab.pathMutation);
    }

    async function switchWorkspace(id, options = {}) {
        const target = state;
        if (!isLiveInstance(target) || target.workspaceSwitching) return false;
        const nextId = WS_MODEL.normalizeId(id);
        if (nextId === target.workspaceId) {
            // Same workspace (a host-key review ended in the dialog): take the fresh data and refresh.
            if (options.workspace && WS_MODEL.isRemote(nextId)) {
                target.workspace = Object.assign({}, target.workspace, options.workspace, { connected: true });
                target.workspaceStatus = 'connected';
                target.workspaceError = null;
                target.workspaceWasLost = true;
                runWithInstance(target, workspaceAfterReconnect);
            }
            return true;
        }
        let next = nextId === WS_MODEL.LOCAL_ID ? WS_MODEL.localWorkspace() : (options.workspace || (target.workspaces || []).find(ws => ws.id === nextId));
        if (!next) return false;
        target.workspaceSwitching = true;
        try {
            if (!await workspaceSettleTabs(target, next)) return false;
            if (next.kind === 'ssh' && !options.connected) {
                runWithInstance(target, () => { target.workspaceSwitchTarget = next.id; workspaceRenderChrome(); });
                try {
                    next = WS_MODEL.mergeWorkspace(next, await apiClient.connectWorkspace(next.id, options.trust || ''));
                } catch (err) {
                    if (!isLiveInstance(target)) return false;
                    const kind = WS_MODEL.classifyError(err);
                    const failed = next;
                    runWithInstance(target, () => {
                        if (kind === 'unknown_host_key') openConnectDialog({ deviceId: failed.deviceId, path: failed.root });
                        else workspaceToast(tr('codeStudio.workspaceSwitchFailed', 'Could not open {{workspace}}: {{error}}', { workspace: workspaceDisplayLabel(failed), error: workspaceErrorText(kind, { host: workspaceHostLabel(failed), path: failed.root, error: err.message || String(err) }) }));
                    });
                    return false;
                } finally {
                    if (isLiveInstance(target)) runWithInstance(target, () => { target.workspaceSwitchTarget = ''; workspaceRenderChrome(); });
                }
            }
            if (!isLiveInstance(target)) return false;
            runWithInstance(target, () => workspaceActivate(next));
            await runAsyncStep(target, () => refreshFiles(WORKSPACE_ROOT));
            if (!isLiveInstance(target)) return false;
            await runAsyncStep(target, restoreTabs);
            if (!isLiveInstance(target)) return false;
            runWithInstance(target, () => { connectTerminal(); scheduleGitRefresh(); });
            return true;
        } finally {
            if (isLiveInstance(target)) target.workspaceSwitching = false;
        }
    }

    // Tears the old workspace down (explorer, search, git, problems, terminals, LSP, tabs) and applies the new session.
    function workspaceActivate(next) {
        const instance = state;
        saveState(); // the leaving workspace's session
        closeWorkspaceMenu(instance);
        // Requests of the old workspace that are still in flight must not land in the new one.
        instance.treeRequest = instance.openRequest = instance.pathRequest = instance.gitRequest = Symbol('workspace');
        if (instance.agentAbortController) instance.agentAbortController.abort();
        closeTerminalSessionSockets(instance);
        lspShutdownInstance(instance);
        if (instance.findOpen) closeFindWidget();
        instance.openTabs.forEach(tab => destroyTabView(tab, true));
        instance.openTabs = [];
        instance.activeTabIndex = -1;
        resetSplitState();
        instance.files = [];
        instance.treeCache = {};
        instance.expandedDirs.clear();
        instance.currentPath = instance.selectedDir = WORKSPACE_ROOT;
        instance.searchResults = [];
        instance.searchPerformed = false;
        instance.searchTruncated = false;
        instance.gitIsRepo = null;
        instance.gitBranch = '';
        instance.gitChanges = [];
        instance.gitLog = [];
        instance.gitHistory = [];
        instance.gitCommitMessage = '';
        instance.outputLines = [];
        instance.workspaceId = next.id;
        instance.workspace = next.kind === 'ssh' ? next : null;
        instance.workspaceStatus = next.kind === 'ssh' ? 'connected' : 'local';
        instance.workspaceError = null;
        instance.workspaceWasLost = false;
        workspaceClearRetry(instance);
        WS_MODEL.writeActive(localStorage, next.id);
        workspaceApplySession(next.id);
        setProblems([]);
        renderTabs();
        renderBreadcrumbs();
        renderEditor();
        renderSearchPanel();
        renderGitPanel();
        renderTerminal();
        renderBottomPanel();
        renderActivityBar();
        renderWindowMenus();
        workspaceRenderChrome();
        saveState();
    }
