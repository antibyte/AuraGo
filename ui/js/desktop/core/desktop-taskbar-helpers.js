    function activateDesktopItem(btn) {
        if (btn.dataset.kind === 'file') {
            openDesktopFileEntry(btn);
            return;
        }
        if (btn.dataset.kind === 'directory') {
            openApp('files', { path: btn.dataset.path || '' });
            return;
        }
        const appId = btn.dataset.appId || btn.dataset.id;
        if (appId === 'files') {
            openApp('files', { path: btn.dataset.path || '' });
            return;
        }
        openApp(appId);
    }

    function ensureDesktopRadialMenuAnchor() {
        const taskbarSystem = document.querySelector('.vd-taskbar-system');
        if (!taskbarSystem) return null;
        let anchor = document.getElementById('radialMenuAnchor');
        const agentButton = $('vd-agent-button');
        if (!anchor) {
            anchor = document.createElement('div');
            anchor.id = 'radialMenuAnchor';
            anchor.className = 'vd-radial-anchor';
            if (agentButton && agentButton.parentElement === taskbarSystem) {
                taskbarSystem.insertBefore(anchor, agentButton);
            } else {
                taskbarSystem.appendChild(anchor);
            }
        } else {
            anchor.classList.add('vd-radial-anchor');
            if (anchor.parentElement !== taskbarSystem) {
                if (agentButton && agentButton.parentElement === taskbarSystem) {
                    taskbarSystem.insertBefore(anchor, agentButton);
                } else {
                    taskbarSystem.appendChild(anchor);
                }
            }
        }
        if (typeof injectRadialMenu === 'function') injectRadialMenu();
        if (typeof initRadialMenu === 'function') initRadialMenu();
        return anchor;
    }
