function wireFruityDockScroll(host) {
    const scroller = host && host.querySelector('[data-fruity-dock-scroll-region]'), track = host && host.querySelector('[data-fruity-dock-track]');
    if (!host || !scroller || !track) return;
    const queueUpdate = () => {
        const schedule = window.requestAnimationFrame || ((callback) => window.setTimeout(callback, 16));
        if (host._fruityDockScrollFrame) return;
        host._fruityDockScrollFrame = schedule(() => {
            host._fruityDockScrollFrame = 0;
            updateFruityDockScrollControls(host);
        });
    };
    host.querySelectorAll('[data-fruity-dock-scroll-button]').forEach(button => {
        button.addEventListener('click', event => {
            event.stopPropagation();
            const direction = button.dataset.fruityDockScrollButton === 'left' ? -1 : 1;
            scroller.scrollBy({ left: direction * Math.max(180, Math.floor(scroller.clientWidth * 0.72)), behavior: 'smooth' });
        });
    });
    scroller.addEventListener('scroll', queueUpdate, { passive: true });
    if (host._fruityDockResizeObserver) host._fruityDockResizeObserver.disconnect();
    if (window.ResizeObserver) {
        host._fruityDockResizeObserver = new ResizeObserver(queueUpdate);
        [scroller, track].forEach(item => host._fruityDockResizeObserver.observe(item));
    }
    queueUpdate();
}

function updateFruityDockScrollControls(host) {
    const scroller = host && host.querySelector('[data-fruity-dock-scroll-region]');
    if (!host || !scroller) return;
    const maxScroll = Math.max(0, scroller.scrollWidth - scroller.clientWidth);
    const overflowing = maxScroll > 2;
    const atStart = !overflowing || scroller.scrollLeft <= 2;
    const atEnd = !overflowing || scroller.scrollLeft >= maxScroll - 2;
    host.classList.toggle('vd-dock-overflowing', overflowing);
    host.classList.toggle('vd-dock-at-start', atStart);
    host.classList.toggle('vd-dock-at-end', atEnd);
    host.querySelectorAll('[data-fruity-dock-scroll-button]').forEach(button => {
        button.disabled = !overflowing || (button.dataset.fruityDockScrollButton === 'left' ? atStart : atEnd);
    });
}

let taskbarSwipeState = null;

function wireMobileTaskbarSwipe() {
    const host = $('vd-taskbar-apps');
    if (!host || host._mobileSwipeWired) return;
    if (!window.useMobileDesktopMode || !window.useMobileDesktopMode()) return;

    host._mobileSwipeWired = true;

    host.addEventListener('touchstart', e => {
        if (e.touches.length !== 1) return;
        taskbarSwipeState = {
            startX: e.touches[0].clientX,
            startTime: Date.now(),
        };
    }, { passive: true });

    host.addEventListener('touchmove', e => {
        if (!taskbarSwipeState || e.touches.length !== 1) return;
        const deltaX = e.touches[0].clientX - taskbarSwipeState.startX;
        if (Math.abs(deltaX) > 20) e.preventDefault();
    }, { passive: false });

    host.addEventListener('touchend', e => {
        if (!taskbarSwipeState) return;

        const deltaX = e.changedTouches[0].clientX - taskbarSwipeState.startX;
        const deltaTime = Date.now() - taskbarSwipeState.startTime;
        const velocity = Math.abs(deltaX) / deltaTime;

        taskbarSwipeState = null;
        if (Math.abs(deltaX) < 50 && velocity < 0.4) return;

        const windows = Array.from(state.windows.values());
        if (windows.length < 2) return;

        const currentIndex = windows.findIndex(w => w.id === state.activeWindowId);
        if (currentIndex === -1) return;

        const nextIndex = deltaX < 0
            ? (currentIndex + 1) % windows.length
            : (currentIndex - 1 + windows.length) % windows.length;
        const nextWin = windows[nextIndex];
        if (nextWin) focusWindow(nextWin.id);
    }, { passive: true });
}

function ensureMobileTaskbarSwipe() {
    if (!window.useMobileDesktopMode || !window.useMobileDesktopMode()) return;

    wireMobileTaskbarSwipe();

    if (isFruityTheme()) {
        const dockHost = $('vd-taskbar-apps');
        const scrollRegion = dockHost && dockHost.querySelector('[data-fruity-dock-scroll-region]');
        if (scrollRegion && !scrollRegion._mobileSwipeWired) wireMobileDockSwipe(scrollRegion);
    }
}

function wireMobileDockSwipe(scrollRegion) {
    if (!scrollRegion) return;
    scrollRegion._mobileSwipeWired = true;

    let dockSwipeState = null;

    scrollRegion.addEventListener('touchstart', e => {
        if (e.touches.length !== 1) return;
        dockSwipeState = { startX: e.touches[0].clientX, startTime: Date.now() };
    }, { passive: true });

    scrollRegion.addEventListener('touchmove', e => {
        if (!dockSwipeState || e.touches.length !== 1) return;
        const deltaX = e.touches[0].clientX - dockSwipeState.startX;
        if (Math.abs(deltaX) > 25) e.preventDefault();
    }, { passive: false });

    scrollRegion.addEventListener('touchend', e => {
        if (!dockSwipeState) return;
        const deltaX = e.changedTouches[0].clientX - dockSwipeState.startX;
        dockSwipeState = null;

        if (Math.abs(deltaX) < 60) return;

        const runningWindows = [...state.windows.values()];
        if (runningWindows.length < 2) return;

        const currentIndex = runningWindows.findIndex(w => w.id === state.activeWindowId);
        if (currentIndex === -1) return;

        const nextIndex = deltaX < 0
            ? (currentIndex + 1) % runningWindows.length
            : (currentIndex - 1 + runningWindows.length) % runningWindows.length;

        const nextWin = runningWindows[nextIndex];
        if (nextWin) focusWindow(nextWin.id);
    }, { passive: true });
}

function renderFruityDock() {
    reconcileFruityDock();
}

function scheduleFruityDockOcclusionCheck() {
    if (state.fruityDockOcclusionFrame) return;
    const schedule = window.requestAnimationFrame || ((callback) => window.setTimeout(callback, 16));
    state.fruityDockOcclusionFrame = schedule(() => {
        state.fruityDockOcclusionFrame = 0;
        updateFruityDockOcclusion();
    });
}

function updateFruityDockOcclusion() {
    const body = document.body;
    const host = $('vd-taskbar-apps');
    if (!body || !host || !isFruityTheme()) {
        if (body) body.classList.remove('fruity-dock-collapsed');
        state.fruityDockFootprint = null;
        return;
    }

    const isMobileMode = window.useMobileDesktopMode && window.useMobileDesktopMode();
    const hasMaximizedMobileWindow = isMobileMode && [...state.windows.values()].some(win =>
        win.element.classList.contains('maximized') ||
        win.element.classList.contains('vd-mobile-forced-maximized')
    );

    if (isMobileMode && hasMaximizedMobileWindow) {
        body.classList.add('fruity-dock-collapsed');
        state.fruityDockFootprint = null;
        return;
    }

    const dockRect = fruityDockFootprint(host);
    const occluded = [...state.windows.values()].some(win => windowOverlapsFruityDock(win, dockRect));
    body.classList.toggle('fruity-dock-collapsed', occluded);
}

function fruityDockFootprint(host) {
    const rect = host.getBoundingClientRect();
    const collapsed = document.body.classList.contains('fruity-dock-collapsed');
    if (!collapsed && rect.width > 120 && rect.height > 40) {
        state.fruityDockFootprint = {
            left: rect.left,
            top: rect.top,
            right: rect.right,
            bottom: rect.bottom
        };
    }
    if (state.fruityDockFootprint) return state.fruityDockFootprint;
    const width = Math.min(920, Math.max(160, window.innerWidth - 170));
    const left = Math.max(0, (window.innerWidth - width) / 2);
    const bottom = window.innerHeight - 8;
    const height = 110;
    return { left, top: Math.max(0, bottom - height), right: left + width, bottom };
}

function windowOverlapsFruityDock(win, dockRect) {
    if (!win || !win.element || win.element.style.display === 'none' || win.element.hidden) return false;
    const rect = win.element.getBoundingClientRect();
    const margin = 6;
    return rect.right > dockRect.left + margin &&
        rect.left < dockRect.right - margin &&
        rect.bottom > dockRect.top + margin &&
        rect.top < dockRect.bottom - margin;
}
