    let windowSwitcherHold = null;
    let notificationChromeSubscriptions = null;

    // notificationContext keeps the launch context an app receives when its notification is clicked.
    function notificationContext(payload) {
        const c = payload.context || {};
        if (payload.appId === 'meshcore' && /^[a-f0-9]{64}$/.test(c.conversation_id || '')) return { conversation_id: c.conversation_id };
        const projectId = c.project_id || c.projectId;
        if (payload.appId === 'storymaker' && /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(projectId || '')) return { projectId };
        if (payload.appId === 'easydrag' && /^flow_[a-z0-9]{10}$/.test(c.flow_id || '')) {
            return /^run_[a-z0-9]{12}$/.test(c.run_id || '') ? { flow_id: c.flow_id, run_id: c.run_id } : { flow_id: c.flow_id };
        }
        if (payload.appId === 'network-cameras' && Number.isSafeInteger(c.event_id) && c.event_id > 0) {
            return typeof c.camera_id === 'string' && /^[a-z0-9_-]{1,64}$/.test(c.camera_id)
                ? { event_id: c.event_id, camera_id: c.camera_id }
                : { event_id: c.event_id };
        }
        if (payload.appId === 'detective' && /^case_[a-z0-9]{1,64}$/.test(c.case_id || '')) return { case_id: c.case_id };
        return undefined;
    }

    function pushNotificationRecord(payload) {
        const entry = window.AuraDesktopNotificationCenter.record({ ...payload, title: payload.title || t('desktop.notification') }, notificationContext(payload));
        updateNotificationBadge();
        return entry;
    }

    function updateNotificationBadge() {
        const badge = document.getElementById('vd-notification-badge');
        if (!badge) return;
        const count = window.AuraDesktopNotificationCenter.history().filter(entry => !entry.read).length;
        badge.hidden = count <= 0;
        badge.textContent = count > 9 ? '9+' : String(count);
    }

    function markAllNotificationsRead() {
        window.AuraDesktopNotificationCenter.markAllRead();
    }

    function notificationPolicyMarkup(panel) {
        const center = window.AuraDesktopNotificationCenter;
        const policy = center.policy();
        const appIDs = new Set(['desktop', ...userFacingApps().map(app => app.id), ...Object.keys(policy.apps)]);
        const selected = appIDs.has(panel.dataset.policyApp) ? panel.dataset.policyApp : 'desktop';
        panel.dataset.policyApp = selected;
        const appPolicy = policy.apps[selected] || { toast: true, sound: true };
        const options = Array.from(appIDs).map(id => `<option value="${esc(id)}" ${id === selected ? 'selected' : ''}>${esc(id === 'desktop' ? t('desktop.notifications_desktop') : appName(appById(id) || { id, name: id }))}</option>`).join('');
        return `<div class="vd-notification-policy">
            <label><input id="vd-notification-dnd" type="checkbox" ${policy.dnd ? 'checked' : ''}>${esc(t('desktop.notifications_dnd'))}</label>
            <details ${panel.dataset.policyOpen === 'true' ? 'open' : ''}><summary>${esc(t('desktop.notifications_manage'))}</summary>
                <label>${esc(t('desktop.notifications_source'))}<select id="vd-notification-source">${options}</select></label>
                <label><input id="vd-notification-toasts" type="checkbox" ${appPolicy.toast ? 'checked' : ''}>${esc(t('desktop.notifications_toasts'))}</label>
                <label><input id="vd-notification-sounds" type="checkbox" ${appPolicy.sound ? 'checked' : ''}>${esc(t('desktop.notifications_sounds'))}</label>
            </details>
            ${center.status().storageFallback ? `<p role="status">${esc(t('desktop.notifications_tab_only'))}</p>` : ''}
        </div>`;
    }

    function wireNotificationPolicy(panel) {
        const center = window.AuraDesktopNotificationCenter;
        panel.querySelector('details').addEventListener('toggle', event => { panel.dataset.policyOpen = String(event.target.open); });
        panel.querySelector('#vd-notification-source').addEventListener('change', event => {
            panel.dataset.policyApp = event.target.value;
            renderNotificationCenter();
        });
        panel.querySelector('#vd-notification-dnd').addEventListener('change', event => {
            center.setPolicy({ ...center.policy(), dnd: event.target.checked });
        });
        ['toasts', 'sounds'].forEach(name => panel.querySelector('#vd-notification-' + name).addEventListener('change', event => {
            const policy = center.policy();
            const appId = panel.dataset.policyApp;
            policy.apps[appId] = { toast: true, sound: true, ...policy.apps[appId], [name === 'toasts' ? 'toast' : 'sound']: event.target.checked };
            center.setPolicy(policy);
        }));
    }

    function closeNotificationCenter(restoreFocus = false) {
        const panel = document.getElementById('vd-notification-center');
        if (panel) {
            panel.hidden = true;
            panel._returnFocus?.setAttribute('aria-expanded', 'false');
            if (restoreFocus && panel._returnFocus?.isConnected) panel._returnFocus.focus();
        }
    }

    function renderNotificationCenter() {
        let panel = document.getElementById('vd-notification-center');
        if (!panel) {
            panel = document.createElement('div');
            panel.id = 'vd-notification-center';
            panel.className = 'vd-notification-center';
            panel.hidden = true;
            panel.setAttribute('role', 'dialog');
            panel.tabIndex = -1;
            panel.addEventListener('keydown', event => {
                if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); closeNotificationCenter(true); }
            });
            document.body.appendChild(panel);
        }
        const focusedID = panel.contains(document.activeElement) ? document.activeElement.id : '';
        const focusedNotification = panel.contains(document.activeElement) ? document.activeElement.dataset.notificationId : '';
        const focusedAction = panel.contains(document.activeElement) ? document.activeElement.dataset.action : '';
        panel.setAttribute('aria-label', t('desktop.notifications_title'));
        const center = window.AuraDesktopNotificationCenter;
        const items = center.history();
        const list = items.length
            ? items.map(entry => `<article class="vd-notification-item${entry.read ? '' : ' unread'}">
                <div class="vd-notification-item-title">${esc(entry.title)}</div>
                ${entry.message ? `<div class="vd-notification-item-message">${esc(entry.message)}</div>` : ''}
                <div class="vd-notification-item-time">${esc(formatNotificationTime(entry.ts))}</div>
                <div class="vd-notification-actions">
                    <button type="button" data-action="open-notification" data-notification-id="${esc(entry.id)}" ${entry.appId !== 'desktop' && !appById(entry.appId) ? 'disabled' : ''}>${esc(t(entry.appId === 'desktop' ? 'desktop.notifications_mark_read' : 'desktop.notifications_open'))}</button>
                    <button type="button" data-action="dismiss-notification" data-notification-id="${esc(entry.id)}">${esc(t('desktop.notifications_dismiss'))}</button>
                </div>
            </article>`).join('')
            : `<div class="vd-notification-empty">${esc(t('desktop.notifications_empty'))}</div>`;
        panel.innerHTML = `<div class="vd-notification-header">
            <div class="vd-notification-title">${esc(t('desktop.notifications_title'))}</div>
            <button type="button" class="vd-notification-clear" data-action="read-notifications">${esc(t('desktop.notifications_mark_all_read'))}</button>
            <button type="button" class="vd-notification-clear" data-action="clear-notifications">${esc(t('desktop.notifications_clear'))}</button>
        </div>
        ${notificationPolicyMarkup(panel)}<div class="vd-notification-list">${list}</div>`;
        wireNotificationPolicy(panel);
        panel.querySelector('[data-action="read-notifications"]').addEventListener('click', markAllNotificationsRead);
        panel.querySelector('[data-action="clear-notifications"]').addEventListener('click', () => {
            center.clear();
        });
        panel.querySelectorAll('[data-notification-id]').forEach(btn => {
            btn.addEventListener('click', () => {
                const entry = center.history().find(item => item.id === btn.dataset.notificationId);
                if (!entry) return;
                if (btn.dataset.action === 'dismiss-notification') { center.remove(entry.id); return; }
                if (entry.appId !== 'desktop' && !appById(entry.appId)) return;
                center.markRead(entry.id);
                if (entry.appId !== 'desktop') {
                    closeNotificationCenter();
                    openApp(entry.appId, notificationContext(entry));
                }
            });
        });
        const focusTarget = focusedID ? panel.querySelector('#' + cssSel(focusedID)) : focusedNotification
            ? panel.querySelector(`[data-notification-id="${cssSel(focusedNotification)}"][data-action="${cssSel(focusedAction)}"]`)
            : focusedAction ? panel.querySelector(`[data-action="${cssSel(focusedAction)}"]`) : null;
        if (focusTarget) focusTarget.focus({ preventScroll: true });
        else if (focusedNotification) (panel.querySelector('[data-notification-id]:not(:disabled)') || panel.querySelector('#vd-notification-dnd') || panel).focus({ preventScroll: true });
    }

    function toggleNotificationCenter(anchor) {
        renderNotificationCenter();
        const panel = document.getElementById('vd-notification-center');
        if (!panel) return;
        if (!panel.hidden) {
            desktopSound('menu.close');
            closeNotificationCenter();
            return;
        }
        panel.hidden = false;
        panel._returnFocus = anchor || document.activeElement;
        panel._returnFocus?.setAttribute('aria-expanded', 'true');
        panel.focus();
        desktopSound('menu.open');
        placeTrayPopover(panel, anchor);
    }

    // Tray popovers open away from their bar: below an anchor in the upper half of the viewport
    // (the Fruity menubar), above it otherwise. The placement also steers the entrance motion in
    // desktop-polish.css. The 8 px gap is measured from the edge of the bar that holds the anchor,
    // not from the anchor itself: tray buttons sit inset in their bar by a skin-dependent amount,
    // and measuring from the button made popovers overlap the taskbar or hang loose under the menubar.
    function placeTrayPopover(panel, anchor) {
        if (!panel || !anchor || !anchor.getBoundingClientRect) return;
        const rect = anchor.getBoundingClientRect();
        const bar = anchor.closest?.('.vd-taskbar, .vd-global-bar');
        const edge = bar ? bar.getBoundingClientRect() : rect;
        const below = rect.top + rect.height / 2 < window.innerHeight / 2;
        panel.dataset.placement = below ? 'below' : 'above';
        panel.style.right = Math.max(8, Math.min(window.innerWidth - rect.right, window.innerWidth - panel.offsetWidth - 8)) + 'px';
        panel.style.maxHeight = Math.max(0, below ? window.innerHeight - edge.bottom - 16 : edge.top - 16) + 'px';
        if (below) {
            panel.style.bottom = '';
            panel.style.top = Math.round(edge.bottom + 8) + 'px';
        } else {
            panel.style.top = '';
            panel.style.bottom = Math.max(8, Math.round(window.innerHeight - edge.top + 8)) + 'px';
        }
    }

    function formatNotificationTime(ts) {
        const date = new Date(ts || Date.now());
        return date.toLocaleString([], { hour: '2-digit', minute: '2-digit', day: '2-digit', month: 'short' });
    }

    function closeClockPopup(restoreFocus = false) {
        closeClockCalendarPopup(restoreFocus);
    }

    function openClockPopup(anchor) {
        closeNotificationCenter();
        closeQuickSettingsPopup();
        return openClockCalendarPopup(anchor);
    }

    function toggleClockPopup(anchor) {
        const popup = document.getElementById('vd-clock-popup');
        if (popup && !popup.hidden) closeClockPopup(true);
        else openClockPopup(anchor);
    }

    function closeShortcutsHelp() {
        const help = document.getElementById('vd-shortcuts-help');
        if (help) help.remove();
    }

    function showShortcutsHelp() {
        closeShortcutsHelp();
        const overlay = document.createElement('div');
        overlay.id = 'vd-shortcuts-help';
        overlay.className = 'vd-shortcuts-help';
        const rows = [
            ['Ctrl+Space / Win', t('desktop.shortcuts_start_menu')],
            ['Ctrl+K', t('desktop.shortcuts_spotlight')],
            ['Ctrl+Tab', t('desktop.shortcuts_window_switch')],
            ['Ctrl+Alt+W', t('desktop.shortcuts_window_switch_alt')],
            ['Ctrl+Alt+← / →', t('desktop.spaces_help')],
            ['Ctrl+Alt+↑ / F3', t('desktop.spaces_overview_help')],
            ['Ctrl+Alt+↓', t('desktop.spaces_overview_close')],
            ['Win / ⌘ + ←↑→↓', t('desktop.shortcuts_window_snap')],
            ['Win / ⌘ + ← →, ↑ ↓', t('desktop.shortcuts_snap_quarter')],
            ['Shift+F10', t('desktop.shortcuts_layout_menu')],
            ['Alt+F4', t('desktop.close')],
            ['F11', t('desktop.maximize')],
            ['F1', t('desktop.shortcuts_title')]
        ];
        overlay.innerHTML = `<div class="vd-shortcuts-panel">
            <div class="vd-shortcuts-heading">${esc(t('desktop.shortcuts_title'))}</div>
            <ul class="vd-shortcuts-list">${rows.map(row => `<li><kbd>${esc(row[0])}</kbd><span>${esc(row[1])}</span></li>`).join('')}</ul>
            <p>${esc(t('desktop.shortcuts_browser_conflicts'))}</p>
            <button type="button" class="vd-shortcuts-close">${esc(t('desktop.close'))}</button>
        </div>`;
        overlay.addEventListener('click', event => {
            if (event.target === overlay) closeShortcutsHelp();
        });
        overlay.querySelector('.vd-shortcuts-close').addEventListener('click', closeShortcutsHelp);
        document.body.appendChild(overlay);
    }

    function windowOverviewWindows(options = {}) {
        if (options.mode === 'snap-partner') return windowSnapPartnerCandidates(options.sourceId, options.partnerZone);
        return [...state.windows.values()].filter(win => win && win.element && !win.isGadget && !win.closing
            && (!options.appId || win.appId === options.appId)
            && (options.scope === 'all' || (options.scope === 'visible' ? isWindowOnActiveSpace(win)
                : windowSpaceId(win) === normalizeSpaceId(state.activeSpaceId))));
    }

    function activateOverviewWindow(id) {
        const win = state.windows.get(id);
        if (desktopSessionEnded || !win || win.closing || win.isGadget) return false;
        if (windowSpaceId(win) !== normalizeSpaceId(state.activeSpaceId)) {
            if (spacesEnabled()) switchSpace(windowSpaceId(win));
            else {
                state.activeSpaceId = windowSpaceId(win);
                applyActiveSpaceWallpaper();
                renderSpacePager();
            }
        }
        focusWindow(id);
        const target = win.element.querySelector('[data-action="maximize"]') || win.element.querySelector('[data-action="close"]');
        target?.focus({ preventScroll: true });
        return true;
    }

    function initWindowOverviewRuntime() {
        window.AuraDesktopWindowOverview?.configure({
            getWindows: windowOverviewWindows,
            getActiveWindowId: () => state.activeWindowId,
            spaceName: win => workspaceName(windowSpaceId(win)),
            appName: windowTitle,
            previewMarkup: windowPreviewMarkup,
            activateWindow: activateOverviewWindow,
            chooseSnapPartner: chooseWindowSnapPartner,
            closeWindow,
            closeBatch: closeDesktopWindowBatch,
            t,
            mountDialog: mountDesktopDialog
        });
        document.addEventListener('aurago:auth-ended', () => endWindowSwitcherHold(false));
    }

    function hasOtherDesktopModal() {
        return [...document.querySelectorAll('[aria-modal="true"]')]
            .some(dialog => dialog.id !== 'vd-window-overview' && dialog.getClientRects().length);
    }

    function openDesktopWindowOverview(options = {}) {
        if (desktopSessionEnded || hasOtherDesktopModal()) return false;
        const requestedOpener = options.opener || document.activeElement;
        const opener = requestedOpener && requestedOpener !== document.body && requestedOpener.isConnected
            && requestedOpener.getClientRects().length && !requestedOpener.closest('.vd-context-menu, #vd-start-menu')
            ? requestedOpener : [document.querySelector('[data-fruity-dock-orb]'), $('vd-start-button')].find(el => el?.getClientRects().length);
        hideTaskbarThumbnail();
        closeContextMenu(true);
        closeWindowLayoutPopup(false);
        closeStartMenu();
        return window.AuraDesktopWindowOverview?.open({ mode: 'switcher', scope: 'current', ...options, opener }) || false;
    }

    function windowSwitcherWindows() {
        return windowOverviewWindows({ scope: 'current' });
    }

    function beginWindowSwitcherHold(reverse) {
        if (!windowSwitcherWindows().length) return;
        if (!openDesktopWindowOverview({ mode: 'switcher' })) return;
        windowSwitcherHold = true;
        window.AuraDesktopWindowOverview.cycle(reverse);
    }

    function cycleWindowSwitcherHold(reverse) {
        if (windowSwitcherHold) window.AuraDesktopWindowOverview?.cycle(reverse);
    }

    function endWindowSwitcherHold(applyFocus) {
        windowSwitcherHold = null;
        window.AuraDesktopWindowOverview?.finish({ activate: !!applyFocus });
    }

    function handleWindowSwitcherKeydown(event) {
        const ctrl = event.ctrlKey || event.metaKey;
        if (ctrl && event.altKey && event.key.toLowerCase() === 'w') {
            event.preventDefault();
            windowSwitcherHold = null;
            openDesktopWindowOverview({ mode: 'switcher' });
            return true;
        }
        if (!ctrl || event.altKey || event.key !== 'Tab') return false;
        event.preventDefault();
        if (windowSwitcherHold && window.AuraDesktopWindowOverview?.isOpen('switcher')) cycleWindowSwitcherHold(event.shiftKey);
        else beginWindowSwitcherHold(event.shiftKey);
        return true;
    }

    function handleWindowSwitcherKeyup(event) {
        if (!windowSwitcherHold || !['Control', 'Meta'].includes(event.key)) return false;
        windowSwitcherHold = null;
        if (!hasOtherDesktopModal() && window.AuraDesktopWindowOverview?.isOpen('switcher')) {
            window.AuraDesktopWindowOverview.finish({ activate: true });
        }
        return true;
    }

    function openDesktopFullscreenOnStartup() {
        const button = document.getElementById('vd-fullscreen-button');
        if (!state.bootstrap?.enabled || !settingBool('desktop.open_fullscreen') || !button || button.disabled || document.fullscreenElement) return;
        const controller = new AbortController();
        const options = { capture: true, signal: controller.signal };
        const activate = event => {
            if (event && (!event.isTrusted || event.repeat || event.ctrlKey || event.altKey || event.metaKey)) return;
            if (event?.type === 'keydown' && ['Escape', 'F11', 'Shift', 'Control', 'Alt', 'Meta'].includes(event.key)) return;
            if (event && navigator.userActivation && !navigator.userActivation.isActive) return;
            controller.abort();
            // Let the taskbar button handle its own click or keyboard activation once.
            if (event && button.contains(event.target)) return;
            if (settingBool('desktop.open_fullscreen') && !document.fullscreenElement && !button.disabled) button.click();
        };
        // Browsers require a user gesture; wait for the first one when startup has none.
        document.addEventListener('click', activate, options);
        document.addEventListener('keydown', activate, options);
        document.addEventListener('fullscreenchange', () => {
            if (document.fullscreenElement) controller.abort();
        }, options);
        window.addEventListener('pagehide', () => controller.abort(), { once: true, signal: controller.signal });
        if (navigator.userActivation?.isActive) activate();
    }

    function wireShellChromeControls() {
        if (!notificationChromeSubscriptions) {
            const refresh = () => {
                updateNotificationBadge();
                const panel = document.getElementById('vd-notification-center');
                if (panel && !panel.hidden) renderNotificationCenter();
            };
            notificationChromeSubscriptions = [window.AuraDesktopNotificationCenter.subscribe(refresh), window.AuraDesktopNotificationCenter.subscribePolicy(refresh)];
            window.addEventListener('pagehide', event => {
                if (event.persisted) return;
                notificationChromeSubscriptions?.forEach(unsubscribe => unsubscribe());
                notificationChromeSubscriptions = null;
            });
        }
        const fullscreenBtn = document.getElementById('vd-fullscreen-button');
        if (fullscreenBtn && !fullscreenBtn.dataset.shellChromeWired) {
            fullscreenBtn.dataset.shellChromeWired = 'true';
            let pending = false;
            const syncFullscreen = () => {
                const active = !!document.fullscreenElement;
                const supported = !!(document.fullscreenEnabled && document.documentElement.requestFullscreen && document.exitFullscreen);
                const label = !supported ? 'desktop.fullscreen_unavailable' : active ? 'desktop.exit_fullscreen' : 'desktop.fullscreen';
                fullscreenBtn.disabled = pending || !supported;
                fullscreenBtn.setAttribute('aria-busy', String(pending));
                fullscreenBtn.setAttribute('aria-pressed', String(active));
                fullscreenBtn.dataset.i18nTitle = label;
                fullscreenBtn.dataset.i18nAriaLabel = label;
                fullscreenBtn.title = t(label);
                fullscreenBtn.setAttribute('aria-label', t(label));
                fullscreenBtn.querySelector('path').setAttribute('d', active
                    ? 'M3 8h5V3m8 0v5h5M8 21v-5H3m18 0h-5v5'
                    : 'M8 3H3v5m13-5h5v5M3 16v5h5m13-5v5h-5');
            };
            fullscreenBtn.addEventListener('click', async () => {
                if (fullscreenBtn.disabled) return;
                pending = true;
                syncFullscreen();
                try {
                    if (document.fullscreenElement) await document.exitFullscreen();
                    else await document.documentElement.requestFullscreen();
                } catch (_) {
                    showDesktopNotification({ type: 'error', title: t('desktop.fullscreen'), message: t('desktop.fullscreen_error') });
                } finally {
                    pending = false;
                    syncFullscreen();
                }
            });
            document.addEventListener('fullscreenchange', syncFullscreen);
            window.addEventListener('aurago:language-changed', syncFullscreen);
            syncFullscreen();
        }
        const clock = document.getElementById('vd-clock');
        if (clock && !clock.dataset.shellChromeWired) {
            clock.dataset.shellChromeWired = 'true';
            clock.style.cursor = 'pointer';
            clock.title = t('desktop.clock_open_calendar');
            clock.setAttribute('aria-label', t('desktop.clock_open_calendar'));
            clock.setAttribute('role', 'button');
            clock.setAttribute('aria-haspopup', 'dialog');
            clock.setAttribute('aria-expanded', 'false');
            clock.tabIndex = 0;
            clock.addEventListener('keydown', event => {
                if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); event.stopPropagation(); toggleClockPopup(clock); }
            });
            clock.addEventListener('click', event => {
                event.stopPropagation();
                toggleClockPopup(clock);
            });
        }
        const notifyBtn = document.getElementById('vd-notification-button');
        if (notifyBtn && !notifyBtn.dataset.shellChromeWired) {
            notifyBtn.dataset.shellChromeWired = 'true';
            notifyBtn.setAttribute('aria-label', t('desktop.notifications_title'));
            notifyBtn.setAttribute('aria-haspopup', 'dialog');
            notifyBtn.setAttribute('aria-expanded', 'false');
            notifyBtn.addEventListener('click', event => {
                event.stopPropagation();
                closeClockPopup();
                closeQuickSettingsPopup();
                toggleNotificationCenter(notifyBtn);
            });
        }
        const quickButton = document.getElementById('vd-quick-settings-button');
        if (quickButton && !quickButton.dataset.shellChromeWired) {
            quickButton.dataset.shellChromeWired = 'true';
            quickButton.addEventListener('click', event => {
                event.stopPropagation();
                closeClockPopup();
                closeNotificationCenter();
                toggleQuickSettingsPopup(quickButton);
            });
        }
        document.addEventListener('click', event => {
            // A panel may replace the clicked control before this event bubbles here.
            if (!event.composedPath().some(node => node.matches?.('#vd-clock-popup, #vd-clock, #vd-notification-center, #vd-notification-button, #vd-quick-settings, #vd-quick-settings-button'))) {
                closeClockPopup();
                closeNotificationCenter();
                closeQuickSettingsPopup();
            }
        });
        const repositionPanels = () => {
            for (const [panelId, anchorId] of [['vd-clock-popup', 'vd-clock'], ['vd-notification-center', 'vd-notification-button'], ['vd-quick-settings', 'vd-quick-settings-button']]) {
                const panel = document.getElementById(panelId);
                if (panel && !panel.hidden) placeTrayPopover(panel, document.getElementById(anchorId));
            }
        };
        window.addEventListener('resize', repositionPanels);
        window.addEventListener('aurago:desktop-settings', repositionPanels);
    }
