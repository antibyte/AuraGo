    state.notificationHistory = state.notificationHistory || [];
    state.notificationUnread = state.notificationUnread || 0;
    let windowSwitcherHold = null;

    function pushNotificationRecord(payload) {
        const entry = {
            id: 'n-' + Date.now() + '-' + Math.random().toString(36).slice(2, 7),
            title: String(payload.title || t('desktop.notification')),
            message: String(payload.message || ''),
            appId: payload.appId || '',
            ts: Date.now(),
            read: false
        };
        state.notificationHistory.unshift(entry);
        if (state.notificationHistory.length > 40) state.notificationHistory.length = 40;
        state.notificationUnread = Math.min(99, (state.notificationUnread || 0) + 1);
        updateNotificationBadge();
        return entry;
    }

    function updateNotificationBadge() {
        const badge = document.getElementById('vd-notification-badge');
        if (!badge) return;
        const count = state.notificationUnread || 0;
        badge.hidden = count <= 0;
        badge.textContent = count > 9 ? '9+' : String(count);
    }

    function markAllNotificationsRead() {
        state.notificationHistory.forEach(entry => { entry.read = true; });
        state.notificationUnread = 0;
        updateNotificationBadge();
    }

    function closeNotificationCenter() {
        const panel = document.getElementById('vd-notification-center');
        if (panel) panel.hidden = true;
    }

    function renderNotificationCenter() {
        let panel = document.getElementById('vd-notification-center');
        if (!panel) {
            panel = document.createElement('div');
            panel.id = 'vd-notification-center';
            panel.className = 'vd-notification-center';
            panel.hidden = true;
            document.body.appendChild(panel);
        }
        const items = state.notificationHistory || [];
        const list = items.length
            ? items.map(entry => `<button type="button" class="vd-notification-item${entry.read ? '' : ' unread'}" data-notification-id="${esc(entry.id)}" data-app-id="${esc(entry.appId || '')}">
                <div class="vd-notification-item-title">${esc(entry.title)}</div>
                ${entry.message ? `<div class="vd-notification-item-message">${esc(entry.message)}</div>` : ''}
                <div class="vd-notification-item-time">${esc(formatNotificationTime(entry.ts))}</div>
            </button>`).join('')
            : `<div class="vd-notification-empty">${esc(t('desktop.notifications_empty'))}</div>`;
        panel.innerHTML = `<div class="vd-notification-header">
            <div class="vd-notification-title">${esc(t('desktop.notifications_title'))}</div>
            <button type="button" class="vd-notification-clear" data-action="clear-notifications">${esc(t('desktop.notifications_clear'))}</button>
        </div>
        <div class="vd-notification-list">${list}</div>`;
        panel.querySelector('[data-action="clear-notifications"]').addEventListener('click', () => {
            state.notificationHistory = [];
            state.notificationUnread = 0;
            updateNotificationBadge();
            renderNotificationCenter();
        });
        panel.querySelectorAll('[data-notification-id]').forEach(btn => {
            btn.addEventListener('click', () => {
                const entry = state.notificationHistory.find(item => item.id === btn.dataset.notificationId);
                if (entry) entry.read = true;
                state.notificationUnread = Math.max(0, (state.notificationUnread || 0) - 1);
                updateNotificationBadge();
                closeNotificationCenter();
                if (btn.dataset.appId) openApp(btn.dataset.appId);
            });
        });
    }

    function toggleNotificationCenter(anchor) {
        renderNotificationCenter();
        const panel = document.getElementById('vd-notification-center');
        if (!panel) return;
        if (!panel.hidden) {
            closeNotificationCenter();
            return;
        }
        panel.hidden = false;
        markAllNotificationsRead();
        if (anchor && anchor.getBoundingClientRect) {
            const rect = anchor.getBoundingClientRect();
            panel.style.right = Math.max(8, window.innerWidth - rect.right) + 'px';
            panel.style.bottom = Math.max(8, window.innerHeight - rect.top + 8) + 'px';
        }
    }

    function formatNotificationTime(ts) {
        const date = new Date(ts || Date.now());
        return date.toLocaleString([], { hour: '2-digit', minute: '2-digit', day: '2-digit', month: 'short' });
    }

    function closeClockPopup() {
        const popup = document.getElementById('vd-clock-popup');
        if (popup) popup.hidden = true;
    }

    async function openClockPopup(anchor) {
        closeClockPopup();
        let popup = document.getElementById('vd-clock-popup');
        if (!popup) {
            popup = document.createElement('div');
            popup.id = 'vd-clock-popup';
            popup.className = 'vd-clock-popup';
            document.body.appendChild(popup);
        }
        popup.hidden = false;
        popup.innerHTML = `<div class="vd-clock-popup-loading">${esc(t('desktop.loading'))}</div>`;
        if (anchor && anchor.getBoundingClientRect) {
            const rect = anchor.getBoundingClientRect();
            popup.style.right = Math.max(8, window.innerWidth - rect.right) + 'px';
            popup.style.bottom = Math.max(8, window.innerHeight - rect.top + 8) + 'px';
        }
        let appointments = [];
        try {
            appointments = await api('/api/appointments?status=all');
        } catch (_) {
            appointments = [];
        }
        const now = new Date();
        const todayKey = now.toISOString().slice(0, 10);
        const todayItems = (appointments || []).filter(item => String(item.date_time || '').startsWith(todayKey));
        const events = todayItems.slice(0, 6).map(item => {
            const time = item.date_time ? new Date(item.date_time).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : '';
            return `<li><span class="vd-clock-event-time">${esc(time)}</span><span class="vd-clock-event-title">${esc(item.title || '')}</span></li>`;
        }).join('');
        popup.innerHTML = `<div class="vd-clock-popup-head">
            <div class="vd-clock-popup-date">${esc(now.toLocaleDateString([], { weekday: 'long', day: 'numeric', month: 'long' }))}</div>
            <div class="vd-clock-popup-time">${esc(now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }))}</div>
        </div>
        <div class="vd-clock-popup-section">${esc(t('desktop.clock_today'))}</div>
        ${events ? `<ul class="vd-clock-popup-events">${events}</ul>` : `<div class="vd-clock-popup-empty">${esc(t('desktop.clock_no_events'))}</div>`}
        <button type="button" class="vd-clock-popup-open" data-action="open-calendar">${esc(t('desktop.clock_open_calendar'))}</button>`;
        popup.querySelector('[data-action="open-calendar"]').addEventListener('click', () => {
            closeClockPopup();
            openApp('calendar');
        });
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
            ['Alt+F4', t('desktop.close')],
            ['F11', t('desktop.maximize')],
            ['F1', t('desktop.shortcuts_title')]
        ];
        overlay.innerHTML = `<div class="vd-shortcuts-panel">
            <div class="vd-shortcuts-heading">${esc(t('desktop.shortcuts_title'))}</div>
            <ul class="vd-shortcuts-list">${rows.map(row => `<li><kbd>${esc(row[0])}</kbd><span>${esc(row[1])}</span></li>`).join('')}</ul>
            <button type="button" class="vd-shortcuts-close">${esc(t('desktop.close'))}</button>
        </div>`;
        overlay.addEventListener('click', event => {
            if (event.target === overlay) closeShortcutsHelp();
        });
        overlay.querySelector('.vd-shortcuts-close').addEventListener('click', closeShortcutsHelp);
        document.body.appendChild(overlay);
    }

    function windowSwitcherWindows() {
        return taskbarWindows().filter(win => win && win.element && win.element.style.display !== 'none');
    }

    function renderWindowSwitcherOverlay(nextIndex) {
        const wins = windowSwitcherWindows();
        if (!wins.length) return;
        let overlay = document.getElementById('vd-window-switcher');
        if (!overlay) {
            overlay = document.createElement('div');
            overlay.id = 'vd-window-switcher';
            overlay.className = 'vd-window-switcher';
            document.body.appendChild(overlay);
        }
        overlay.innerHTML = wins.map((win, i) => {
            const app = appById(win.appId);
            const icon = iconMarkup(win.icon || iconForApp(app), win.iconGlyph || iconGlyph(app), 'vd-switcher-icon', 20);
            const isActive = i === nextIndex;
            return `<button type="button" class="vd-switcher-item${isActive ? ' active' : ''}" data-window-id="${esc(win.id)}" title="${esc(win.title)}">${icon}<span class="vd-switcher-label">${esc(win.title)}</span></button>`;
        }).join('');
    }

    function beginWindowSwitcherHold(reverse) {
        const wins = windowSwitcherWindows();
        if (wins.length <= 1) {
            if (wins.length === 1) focusWindow(wins[0].id);
            return;
        }
        const currentIndex = wins.findIndex(win => win.id === state.activeWindowId);
        const step = reverse ? -1 : 1;
        const nextIndex = currentIndex < 0 ? 0 : (currentIndex + step + wins.length) % wins.length;
        windowSwitcherHold = { index: nextIndex, winsLength: wins.length };
        renderWindowSwitcherOverlay(nextIndex);
    }

    function cycleWindowSwitcherHold(reverse) {
        if (!windowSwitcherHold) return;
        const wins = windowSwitcherWindows();
        if (!wins.length) {
            endWindowSwitcherHold(false);
            return;
        }
        const step = reverse ? -1 : 1;
        windowSwitcherHold.index = (windowSwitcherHold.index + step + wins.length) % wins.length;
        renderWindowSwitcherOverlay(windowSwitcherHold.index);
    }

    function endWindowSwitcherHold(applyFocus) {
        const overlay = document.getElementById('vd-window-switcher');
        const wins = windowSwitcherWindows();
        if (applyFocus && windowSwitcherHold && wins[windowSwitcherHold.index]) {
            focusWindow(wins[windowSwitcherHold.index].id);
        }
        windowSwitcherHold = null;
        if (overlay) overlay.remove();
    }

    function handleWindowSwitcherKeydown(event) {
        const ctrl = event.ctrlKey || event.metaKey;
        if (!ctrl || event.altKey) return false;
        if (event.key === 'Tab') {
            event.preventDefault();
            if (!windowSwitcherHold) beginWindowSwitcherHold(event.shiftKey);
            else cycleWindowSwitcherHold(event.shiftKey);
            return true;
        }
        if ((event.ctrlKey || event.metaKey) && event.altKey && event.key === 'w') {
            event.preventDefault();
            if (!windowSwitcherHold) beginWindowSwitcherHold(false);
            else cycleWindowSwitcherHold(false);
            return true;
        }
        return false;
    }

    function handleWindowSwitcherKeyup(event) {
        if (!windowSwitcherHold) return false;
        if (event.key === 'Control' || event.key === 'Meta' || event.key === 'Alt') {
            endWindowSwitcherHold(true);
            return true;
        }
        return false;
    }

    function wireShellChromeControls() {
        const clock = document.getElementById('vd-clock');
        if (clock && !clock.dataset.shellChromeWired) {
            clock.dataset.shellChromeWired = 'true';
            clock.style.cursor = 'pointer';
            clock.title = t('desktop.clock_open_calendar');
            clock.addEventListener('click', event => {
                event.stopPropagation();
                openClockPopup(clock);
            });
        }
        const notifyBtn = document.getElementById('vd-notification-button');
        if (notifyBtn && !notifyBtn.dataset.shellChromeWired) {
            notifyBtn.dataset.shellChromeWired = 'true';
            notifyBtn.addEventListener('click', event => {
                event.stopPropagation();
                toggleNotificationCenter(notifyBtn);
            });
        }
        document.addEventListener('click', event => {
            if (!event.target.closest('#vd-clock-popup, #vd-clock, #vd-notification-center, #vd-notification-button')) {
                closeClockPopup();
                closeNotificationCenter();
            }
        });
    }
