    // Pointer light for launchers, menu entries and task buttons (desktop-polish.css): the
    // hovered item receives the pointer position as two custom properties, written at most
    // once per frame and only while a mouse moves over such an item. Touch never paints it.
    const POINTER_LIGHT_TARGETS = '.vd-start-item, .vd-context-item, .vd-task-button, .vd-taskbar-pin, .vd-window-menu-item, .vd-settings-nav, .vd-launchpad-tile';
    let pointerLightItem = null;
    let pointerLightFrame = 0;
    let pointerLightX = 0;
    let pointerLightY = 0;

    function paintPointerLight() {
        pointerLightFrame = 0;
        const item = pointerLightItem;
        if (!item || !item.isConnected) return;
        const rect = item.getBoundingClientRect();
        item.style.setProperty('--vd-rx', Math.round(pointerLightX - rect.left) + 'px');
        item.style.setProperty('--vd-ry', Math.round(pointerLightY - rect.top) + 'px');
    }

    function trackPointerLight(event) {
        if (event.pointerType && event.pointerType !== 'mouse') return;
        const item = event.target instanceof Element ? event.target.closest(POINTER_LIGHT_TARGETS) : null;
        pointerLightItem = item;
        if (!item) return;
        pointerLightX = event.clientX;
        pointerLightY = event.clientY;
        if (!pointerLightFrame) pointerLightFrame = requestAnimationFrame(paintPointerLight);
    }

    document.addEventListener('pointermove', trackPointerLight, { passive: true });

    // A new notification swings the bell once (desktop-polish.css); restarting the class lets
    // a quick second notification ring again.
    function ringNotificationBell() {
        const bell = document.getElementById('vd-notification-button');
        if (!bell || !animationsEnabled()) return;
        bell.classList.remove('vd-bell-ring');
        void bell.offsetWidth;
        bell.classList.add('vd-bell-ring');
        clearTimeout(bell._bellRingTimer);
        bell._bellRingTimer = setTimeout(() => bell.classList.remove('vd-bell-ring'), 900);
    }
