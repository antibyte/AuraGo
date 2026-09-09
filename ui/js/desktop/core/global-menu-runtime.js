    // Move the existing menu DOM; actions and shortcuts keep their window owner.
    function windowMenuBar(windowId) {
        const win = state.windows.get(windowId);
        const local = win && win.element.querySelector('.vd-window-menubar');
        if (local) return local;
        const global = document.querySelector('#vd-global-menu-host > .vd-window-menubar');
        return global && global.dataset.ownerWindow === windowId ? global : null;
    }
    function syncDesktopMenuBar() {
        const enabled = isFruityTheme() && !isCompactViewport();
        let top = document.getElementById('vd-global-bar');
        const taskbar = document.querySelector('.vd-taskbar');
        if (!taskbar) return;
        if (!top && enabled) {
            top = document.createElement('header');
            top.id = 'vd-global-bar';
            top.className = 'vd-global-bar';
            top.innerHTML = '<button type="button" class="vd-global-brand" aria-label="' + esc(t('desktop.start_menu')) + '">A<span aria-hidden="true">✦</span></button><span class="vd-global-appname"></span><div id="vd-global-menu-host"></div>';
            top.querySelector('.vd-global-brand').addEventListener('click', toggleStartMenu);
            document.body.appendChild(top);
        }
        document.body.dataset.globalMenus = enabled ? 'true' : 'false';
        if (!top) return;
        top.hidden = !enabled;
        const system = document.querySelector('.vd-taskbar-system');
        const systemParent = enabled ? top : taskbar;
        if (system && system.parentElement !== systemParent) systemParent.appendChild(system);
        const active = state.windows.get(state.activeWindowId);
        const visible = active && !active.minimized && !active.minimizing && !active.closing && isWindowOnActiveSpace(active);
        const owner = enabled && visible ? active.id : '';
        const host = top.querySelector('#vd-global-menu-host');
        const old = host.querySelector('.vd-window-menubar');
        if (old && old.dataset.ownerWindow !== owner) {
            closeWindowMenu();
            const previous = state.windows.get(old.dataset.ownerWindow);
            if (previous && !previous.closing) previous.element.querySelector('.vd-window-titlebar').appendChild(old);
            else old.remove();
        }
        state.windows.forEach(win => {
            const menu = windowMenuBar(win.id);
            const global = enabled && !!menu;
            win.element.classList.toggle('has-global-menu', global);
            if (menu && owner === win.id && menu.parentElement !== host) {
                menu.dataset.ownerWindow = win.id;
                host.appendChild(menu);
            }
        });
        top.querySelector('.vd-global-appname').textContent = visible ? active.title : 'AuraGo';
    }

    function positionDesktopSubmenus(menu) {
        menu.querySelectorAll('.vd-context-submenu, .vd-window-menu-submenu').forEach(item => {
            const fit = () => {
                const pop = item.querySelector(':scope > .vd-context-submenu-popover, :scope > .vd-window-menu-popover');
                if (!pop) return;
                pop.style.left = ''; pop.style.right = ''; pop.style.top = '';
                const rect = pop.getBoundingClientRect();
                if (rect.right > window.innerWidth - 8) { pop.style.left = 'auto'; pop.style.right = 'calc(100% + 4px)'; }
                const placed = pop.getBoundingClientRect();
                if (placed.left < 8) { pop.style.right = 'auto'; pop.style.left = (8 - item.getBoundingClientRect().left) + 'px'; }
                const dy = Math.max(8 - placed.top, Math.min(0, window.innerHeight - 8 - placed.bottom));
                pop.style.top = (-7 + dy) + 'px';
            };
            item.addEventListener('pointerenter', fit);
            item.addEventListener('focusin', fit);
        });
    }


    function fitWindowMenu(menu) {
        const pop = menu.querySelector(':scope > .vd-window-menu-popover');
        if (!pop) return;
        // Fixed positioning escapes the compact menubar's horizontal scroll clip.
        pop.style.position = isCompactViewport() ? 'fixed' : '';
        pop.style.left = '0px'; pop.style.top = '0px';
        const bounds = menu.getBoundingClientRect(), origin = pop.getBoundingClientRect();
        pop.style.left = (Math.max(8, Math.min(bounds.left, innerWidth - origin.width - 8)) - origin.left) + 'px';
        pop.style.top = (Math.max(8, Math.min(bounds.bottom + 4, innerHeight - origin.height - 8)) - origin.top) + 'px';
    }
