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
