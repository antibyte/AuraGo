    /* Floating SIP phone gadget: renders the Phone app windowless as a
       draggable device directly on the desktop (pet-style layer). The app
       itself is mounted through the lazy module loader into a scaled stage,
       so the full chassis stays intact regardless of the viewport size.
       Position, visibility and always-on-top are server-persisted desktop
       settings ('phone_gadget.*'). */
    const GADGET_WINDOW_ID = 'sip-phone-gadget';
    const GADGET_STAGE_WIDTH = 400;
    const GADGET_STAGE_HEIGHT = 830;
    const GADGET_SCALE = 0.72;

    let gadgetLayer = null;
    let gadgetMounted = false;
    let gadgetMountPromise = null;
    let gadgetDrag = null;
    let gadgetInitialized = false;

    function phoneGadgetEnabled() {
        return String(settingValue('phone_gadget.enabled')).toLowerCase() === 'true';
    }

    function phoneGadgetAlwaysOnTop() {
        return String(settingValue('phone_gadget.always_on_top')).toLowerCase() === 'true';
    }

    function phoneGadgetLayerSize() {
        return {
            w: Math.round(GADGET_STAGE_WIDTH * GADGET_SCALE),
            h: Math.round(GADGET_STAGE_HEIGHT * GADGET_SCALE)
        };
    }

    function phoneGadgetClampPosition(x, y) {
        const size = phoneGadgetLayerSize();
        const minVisible = 64;
        return {
            x: Math.max(minVisible - size.w, Math.min(x, window.innerWidth - minVisible)),
            y: Math.max(0, Math.min(y, window.innerHeight - minVisible))
        };
    }

    function phoneGadgetStoredPosition() {
        const x = parseInt(settingValue('phone_gadget.position_x'), 10);
        const y = parseInt(settingValue('phone_gadget.position_y'), 10);
        if (Number.isFinite(x) && Number.isFinite(y)) return phoneGadgetClampPosition(x, y);
        const size = phoneGadgetLayerSize();
        return phoneGadgetClampPosition(window.innerWidth - size.w - 28, 28);
    }

    function applyPhoneGadgetPosition() {
        if (!gadgetLayer) return;
        const pos = phoneGadgetStoredPosition();
        gadgetLayer.style.left = pos.x + 'px';
        gadgetLayer.style.top = pos.y + 'px';
    }

    function applyPhoneGadgetOnTop() {
        if (!gadgetLayer) return;
        gadgetLayer.dataset.alwaysOnTop = phoneGadgetAlwaysOnTop() ? 'true' : 'false';
        // Release any inline z-index so CSS can apply the correct layer
        // (always-on-top mode vs. normal window-stack mode).
        gadgetLayer.style.zIndex = '';
    }

    async function savePhoneGadgetSetting(key, value) {
        const body = await api('/api/desktop/settings', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ key, value })
        });
        if (!state.bootstrap) state.bootstrap = {};
        state.bootstrap.settings = Object.assign({}, state.bootstrap.settings || {}, (body && body.settings) || { [key]: value });
    }

    function ensurePhoneGadgetLayer() {
        if (gadgetLayer) return;
        gadgetLayer = document.createElement('div');
        gadgetLayer.id = 'vd-sip-phone-gadget';
        gadgetLayer.className = 'vd-sip-phone-gadget';
        gadgetLayer.innerHTML = '<div class="vd-sip-phone-gadget-scale" data-sip-phone-gadget-host></div>';
        document.body.appendChild(gadgetLayer);
        wirePhoneGadgetEvents();
    }

    function mountPhoneGadget() {
        if (gadgetMounted) return Promise.resolve();
        if (gadgetMountPromise) return gadgetMountPromise;
        ensurePhoneGadgetLayer();
        applyPhoneGadgetPosition();
        applyPhoneGadgetOnTop();
        const modules = window.AuraDesktopModules;
        const host = gadgetLayer.querySelector('[data-sip-phone-gadget-host]');
        if (!modules || typeof modules.loadAppScript !== 'function' || !host) return Promise.resolve();
        gadgetMountPromise = modules.loadAppScript('sip-phone')
            .then(() => {
                if (!gadgetLayer || !gadgetLayer.isConnected || !phoneGadgetEnabled()) return;
                if (!window.SipPhoneApp || typeof window.SipPhoneApp.render !== 'function') return;
                window.SipPhoneApp.render(host, GADGET_WINDOW_ID, {
                    esc,
                    t,
                    api,
                    iconMarkup,
                    notify: showDesktopNotification,
                    openApp
                });
                gadgetMounted = true;
                // Register the gadget as a pseudo-window so it participates in
                // the normal window Z-order stack (unless always-on-top is on).
                if (state && state.windows && !state.windows.has(GADGET_WINDOW_ID)) {
                    state.windows.set(GADGET_WINDOW_ID, {
                        id: GADGET_WINDOW_ID,
                        appId: 'sip-phone',
                        title: t('desktop.app_sip_phone'),
                        element: gadgetLayer,
                        maximized: false,
                        restoreBounds: null,
                        context: {},
                        isGadget: true
                    });
                }
            })
            .catch(err => {
                showDesktopNotification({ title: t('desktop.notification'), message: err.message });
            })
            .finally(() => {
                gadgetMountPromise = null;
            });
        return gadgetMountPromise;
    }

    function unmountPhoneGadget() {
        if (window.SipPhoneApp && typeof window.SipPhoneApp.dispose === 'function') {
            window.SipPhoneApp.dispose(GADGET_WINDOW_ID);
        }
        gadgetMounted = false;
        gadgetDrag = null;
        if (state && state.windows && state.windows.has(GADGET_WINDOW_ID)) {
            state.windows.delete(GADGET_WINDOW_ID);
            if (state.activeWindowId === GADGET_WINDOW_ID) state.activeWindowId = '';
        }
        if (gadgetLayer) {
            gadgetLayer.remove();
            gadgetLayer = null;
        }
    }

    function syncPhoneGadget() {
        if (!state.bootstrap) return;
        if (phoneGadgetEnabled()) mountPhoneGadget();
        else unmountPhoneGadget();
    }

    function isPhoneGadgetDragSource(target) {
        if (!target || !target.closest) return false;
        if (target.closest('.sip-phone-statusbar')) return true;
        const cls = target.classList;
        if (!cls) return false;
        return cls.contains('sip-phone-hw')
            || cls.contains('sip-phone-device')
            || cls.contains('sip-phone')
            || cls.contains('vd-sip-phone-gadget-scale')
            || cls.contains('vd-sip-phone-gadget');
    }

    function wirePhoneGadgetEvents() {
        gadgetLayer.addEventListener('pointerdown', event => {
            if (event.button !== 0 || !isPhoneGadgetDragSource(event.target)) return;
            event.preventDefault();
            gadgetDrag = {
                pointerId: event.pointerId,
                offsetX: event.clientX - gadgetLayer.offsetLeft,
                offsetY: event.clientY - gadgetLayer.offsetTop
            };
            gadgetLayer.setPointerCapture(event.pointerId);
            gadgetLayer.classList.add('dragging');
        });
        gadgetLayer.addEventListener('pointermove', event => {
            if (!gadgetDrag || gadgetDrag.pointerId !== event.pointerId) return;
            const pos = phoneGadgetClampPosition(event.clientX - gadgetDrag.offsetX, event.clientY - gadgetDrag.offsetY);
            gadgetLayer.style.left = pos.x + 'px';
            gadgetLayer.style.top = pos.y + 'px';
        });
        const finishDrag = event => {
            if (!gadgetDrag || gadgetDrag.pointerId !== event.pointerId) return;
            if (gadgetLayer && gadgetLayer.hasPointerCapture && gadgetLayer.hasPointerCapture(event.pointerId)) {
                gadgetLayer.releasePointerCapture(event.pointerId);
            }
            if (gadgetLayer) gadgetLayer.classList.remove('dragging');
            const x = gadgetLayer ? gadgetLayer.offsetLeft : 0;
            const y = gadgetLayer ? gadgetLayer.offsetTop : 0;
            gadgetDrag = null;
            if (event.type === 'pointerup') {
                savePhoneGadgetSetting('phone_gadget.position_x', String(x)).catch(() => {});
                savePhoneGadgetSetting('phone_gadget.position_y', String(y)).catch(() => {});
            } else {
                applyPhoneGadgetPosition();
            }
        };
        gadgetLayer.addEventListener('pointerup', finishDrag);
        gadgetLayer.addEventListener('pointercancel', finishDrag);
        gadgetLayer.addEventListener('contextmenu', event => {
            if (shouldAllowBrowserContextMenu(event)) return;
            event.preventDefault();
            event.stopPropagation();
            showPhoneGadgetContextMenu(event.clientX, event.clientY);
        });
        // When not always-on-top, treat clicks on the gadget like clicks on a
        // normal window: bring it to the front of the window stack.
        gadgetLayer.addEventListener('pointerdown', event => {
            if (event.button !== 0) return;
            if (phoneGadgetAlwaysOnTop()) return;
            if (state && state.windows && state.windows.has(GADGET_WINDOW_ID) && typeof focusWindow === 'function') {
                focusWindow(GADGET_WINDOW_ID);
            }
        });
    }

    function showPhoneGadgetContextMenu(x, y) {
        const onTop = phoneGadgetAlwaysOnTop();
        showContextMenu(x, y, [
            { label: t('desktop.phone_gadget_open_window'), icon: 'phone', action: () => openApp('sip-phone') },
            { separator: true },
            { label: t('desktop.phone_gadget_always_on_top'), icon: onTop ? 'check-square' : 'square', fallback: onTop ? '✓' : '☐', action: togglePhoneGadgetAlwaysOnTop },
            { label: t('desktop.phone_gadget_remove'), icon: 'x', action: disablePhoneGadget }
        ]);
    }

    function togglePhoneGadgetAlwaysOnTop() {
        const next = !phoneGadgetAlwaysOnTop();
        savePhoneGadgetSetting('phone_gadget.always_on_top', String(next))
            .then(applyPhoneGadgetOnTop)
            .catch(err => showDesktopNotification({ title: t('desktop.notification'), message: err.message }));
    }

    function disablePhoneGadget() {
        savePhoneGadgetSetting('phone_gadget.enabled', 'false')
            .then(syncPhoneGadget)
            .catch(err => showDesktopNotification({ title: t('desktop.notification'), message: err.message }));
    }

    function initPhoneGadgetRuntime() {
        if (gadgetInitialized) return;
        gadgetInitialized = true;
        window.addEventListener('resize', () => {
            if (gadgetLayer) applyPhoneGadgetPosition();
        });
        // Re-evaluate once the bootstrap (with server-persisted settings) arrives.
        const check = () => {
            if (state.bootstrap) syncPhoneGadget();
            else setTimeout(check, 150);
        };
        check();
    }

    window.SipPhoneGadget = {
        init: initPhoneGadgetRuntime,
        sync: syncPhoneGadget
    };
