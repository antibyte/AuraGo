// Network Cameras – person detection mode: status strip, timeline, event
// detail and the global settings dialog (window.NetworkCamerasDetection).
// Loaded after network-cameras-detection-views.js and -zones.js.
(function () {
    'use strict';

    const {
        API,
        NS,
        MAX_WINDOWS,
        POLL_MS,
        PAGE_SIZE,
        DAY_KEYS,
        ARM_MODES,
        T,
        clamp,
        idOf,
        dayRange,
        framesOf,
        globalDefaults,
        settingsPayload,
        validateGlobalSettings,
        armSummary,
        detectorSummary,
        upsertEvents,
        mergeArmLog,
        eventMatches,
        mainKind,
        launchTarget,
        stripMarkup,
        emptyMarkup,
        timelineMarkup,
        detailMarkup,
        settingsMarkup,
        cameraSettingsMarkup,
        validateCameraSettings
    } = window.NetworkCamerasDetectionViews;
    const { bindCameraSettings } = window.NetworkCamerasDetectionZones;

    const instances = new Map();

    function windowKey(windowId) {
        return String(windowId == null ? '' : windowId).replace(/[^A-Za-z0-9_-]/g, '') || 'default';
    }

    function createInstance(windowId) {
        return {
            windowId, key: windowKey(windowId), host: null, root: null, ctx: null, options: {}, mounted: false,
            status: null, cursor: null, connectionLost: false, lastPollAt: 0,
            events: [], armLog: [], nextBefore: null, loaded: false, eventsError: '', loadingMore: false, eventsToken: 0,
            filter: { camera: '', day: '' },
            selectedId: null, detail: null, detailError: '', frameIndex: 0, detailToken: 0, deleting: false,
            armBusy: false, busy: false, dialog: null, dialogOpener: '', listShown: false, announced: new Set(), lostFocus: null,
            pollTimer: null, controllers: new Set(), signatures: {}, listeners: []
        };
    }

    function isAbort(error) {
        return !!error && error.name === 'AbortError';
    }

    async function request(inst, path, options) {
        const controller = new AbortController();
        inst.controllers.add(controller);
        const requestOptions = Object.assign({}, options || {}, { signal: controller.signal });
        if (requestOptions.body && typeof requestOptions.body !== 'string') {
            requestOptions.headers = Object.assign({ 'Content-Type': 'application/json' }, requestOptions.headers || {});
            requestOptions.body = JSON.stringify(requestOptions.body);
        }
        try {
            return await inst.ctx.api(path.charAt(0) === '/' ? path : API + path, requestOptions);
        } finally {
            inst.controllers.delete(controller);
        }
    }

    function notify(inst, message, type) {
        if (!inst.ctx || typeof inst.ctx.notify !== 'function' || !message) return;
        inst.ctx.notify({ title: inst.ctx.t('desktop.network_cameras.title'), message, appId: 'network-cameras', type: type || 'info' });
    }

    function announce(inst, message) {
        const live = inst.root && inst.root.querySelector('[data-ncd-live]');
        if (!live || !message) return;
        live.textContent = '';
        setTimeout(() => {
            if (live.isConnected) live.textContent = message;
        }, 30);
    }

    function snapshot(inst) {
        return {
            status: inst.status, canManage: !!inst.options.canManage, streams: inst.options.streams || [], settings: inst.options.detectionSettings || null,
            connectionLost: inst.connectionLost, armBusy: inst.armBusy, busy: inst.busy, now: new Date(), windowKey: inst.key,
            events: inst.events, armLog: inst.armLog, filter: inst.filter, hasMore: inst.nextBefore != null, loaded: inst.loaded, listShown: inst.listShown, loadingMore: inst.loadingMore, eventsError: inst.eventsError,
            selectedId: inst.selectedId, detail: inst.detail, error: inst.detailError, frameIndex: inst.frameIndex, deleting: inst.deleting, dialog: inst.dialog
        };
    }

    function cssEscape(value) {
        return window.CSS && typeof window.CSS.escape === 'function' ? window.CSS.escape(value) : String(value).replace(/["\\]/g, '\\$&');
    }

    function renderRegion(inst, name, html) {
        const region = inst.root && inst.root.querySelector('[data-region="' + name + '"]');
        if (!region || inst.signatures[name] === html) return;
        inst.signatures[name] = html;
        const active = document.activeElement;
        let focusKey = active && region.contains(active) ? active.getAttribute('data-focus-key') : null;
        const lost = inst.lostFocus && Date.now() - inst.lostFocus.at < 10000 ? inst.lostFocus.key : null;
        const restoring = !focusKey && !!lost && (!active || active === document.body);
        if (restoring) focusKey = lost;
        const scroller = region.querySelector('[data-scroll]');
        const scrollTop = scroller ? scroller.scrollTop : 0;
        region.innerHTML = html;
        region.querySelectorAll('[data-region]').forEach(child => { delete inst.signatures[child.dataset.region]; });
        const nextScroller = region.querySelector('[data-scroll]');
        if (nextScroller) nextScroller.scrollTop = scrollTop;
        if (focusKey) {
            const next = region.querySelector('[data-focus-key="' + cssEscape(focusKey) + '"]');
            if (next && !next.disabled) {
                next.focus({ preventScroll: true });
                inst.lostFocus = null;
            } else if (!restoring) {
                inst.lostFocus = { key: focusKey, at: Date.now() };
                const fallback = region.querySelector('[tabindex="0"]');
                if (fallback) fallback.focus({ preventScroll: true });
            }
        }
    }

    function renderStrip(inst) {
        renderRegion(inst, 'strip', stripMarkup(inst.ctx, snapshot(inst)));
    }

    function renderTimeline(inst) {
        renderRegion(inst, 'timeline', timelineMarkup(inst.ctx, snapshot(inst)));
    }

    function renderDetail(inst) {
        renderRegion(inst, 'detail', detailMarkup(inst.ctx, snapshot(inst)));
    }

    function renderMain(inst) {
        const view = snapshot(inst);
        const kind = mainKind(view);
        if (kind !== 'workspace') {
            renderRegion(inst, 'main', emptyMarkup(inst.ctx, Object.assign(view, { kind })));
            return;
        }
        inst.listShown = true;
        renderRegion(inst, 'main', '<div class="ncd-workspace"><div class="ncd-cell" data-region="timeline"></div><div class="ncd-cell" data-region="detail"></div></div>');
        renderTimeline(inst);
        renderDetail(inst);
    }

    function renderDialog(inst) {
        renderRegion(inst, 'dialog', inst.dialog ? settingsMarkup(inst.ctx, snapshot(inst)) : '');
    }

    function renderAll(inst) {
        renderStrip(inst);
        renderMain(inst);
        renderDialog(inst);
    }

    function renderSkeleton(inst) {
        inst.host.innerHTML = '<div class="ncd-root" data-ncd-root><div class="ncd-sr" role="status" aria-live="polite" data-ncd-live></div><div data-region="strip"></div><div class="ncd-main" data-region="main"></div><div data-region="dialog"></div></div>';
        inst.root = inst.host.querySelector('[data-ncd-root]');
        inst.signatures = {};
        bindRoot(inst);
    }

    function stopActivity(inst) {
        if (inst.pollTimer) clearTimeout(inst.pollTimer);
        inst.pollTimer = null;
        inst.controllers.forEach(controller => controller.abort());
        inst.controllers.clear();
        inst.listeners.forEach(remove => remove());
        inst.listeners = [];
    }

    function mount(host, ctx, options) {
        if (!host || !ctx) return;
        const opts = options || {};
        const id = opts.windowId == null ? 'default' : opts.windowId;
        let inst = instances.get(id);
        if (!inst) {
            inst = createInstance(id);
            instances.set(id, inst);
        }
        stopActivity(inst);
        inst.host = host;
        inst.ctx = ctx;
        inst.options = opts;
        inst.mounted = true;
        inst.armBusy = false;
        inst.busy = false;
        inst.loadingMore = false;
        inst.deleting = false;
        if (inst.dialog) inst.dialog.busy = false;
        const launch = launchTarget(opts.launch);
        if (launch) {
            if (launch.camera !== inst.filter.camera || inst.filter.day) {
                inst.filter = { camera: launch.camera, day: '' };
                inst.loaded = false;
            }
            inst.selectedId = launch.eventId;
            inst.detail = null;
            inst.detailError = '';
            inst.frameIndex = 0;
        }
        renderSkeleton(inst);
        renderAll(inst);
        if (inst.cursor === null) {
            initialLoad(inst);
        } else {
            if (!inst.loaded) loadEvents(inst, true);
            const elapsed = Date.now() - inst.lastPollAt;
            schedulePoll(inst, elapsed >= POLL_MS ? 0 : POLL_MS - elapsed);
        }
        if (inst.selectedId && !inst.detail && !inst.detailError) loadDetail(inst, inst.selectedId, false);
    }

    function unmount(windowId, options) {
        const keepState = !!(options && options.keepState);
        const ids = windowId === undefined || windowId === null ? Array.from(instances.keys()) : [windowId];
        for (const id of ids) {
            const inst = instances.get(id);
            if (!inst) continue;
            inst.mounted = false;
            stopActivity(inst);
            if (!keepState) instances.delete(id);
        }
    }

    function schedulePoll(inst, delay) {
        if (inst.pollTimer) clearTimeout(inst.pollTimer);
        inst.pollTimer = null;
        if (!inst.mounted) return;
        inst.pollTimer = setTimeout(() => poll(inst), Math.max(0, delay));
    }

    async function initialLoad(inst) {
        try {
            const changes = await request(inst, 'state?since=0');
            if (!inst.mounted) return;
            inst.lastPollAt = Date.now();
            inst.connectionLost = false;
            applyChanges(inst, changes, true);
        } catch (error) {
            if (isAbort(error) || !inst.mounted) return;
            inst.connectionLost = true;
            renderStrip(inst);
            renderMain(inst);
            schedulePoll(inst, POLL_MS);
            return;
        }
        await loadEvents(inst, true);
        if (inst.mounted) schedulePoll(inst, POLL_MS);
    }

    async function poll(inst) {
        inst.pollTimer = null;
        if (!inst.mounted) return;
        if (inst.cursor === null) {
            initialLoad(inst);
            return;
        }
        try {
            const snapshotFeed = inst.cursor === 0;
            const changes = await request(inst, 'state?since=' + encodeURIComponent(String(inst.cursor)));
            if (!inst.mounted) return;
            inst.lastPollAt = Date.now();
            const lost = inst.connectionLost;
            inst.connectionLost = false;
            applyChanges(inst, changes, false, snapshotFeed);
            if (lost) renderStrip(inst);
            // The database appeared: its events come paged, as on a first load.
            if (snapshotFeed && inst.cursor > 0) loadEvents(inst, true);
        } catch (error) {
            if (isAbort(error) || !inst.mounted) return;
            if (!inst.connectionLost) {
                inst.connectionLost = true;
                renderStrip(inst);
            }
        }
        schedulePoll(inst, POLL_MS);
    }

    function announceStatus(inst, previous, next) {
        if (!previous) return;
        const previousArm = previous.arm || {};
        const nextArm = next.arm || {};
        if (previousArm.mode !== nextArm.mode || !!previousArm.armed !== !!nextArm.armed) {
            announce(inst, T(inst.ctx, 'arm_changed', { state: armSummary(inst.ctx, nextArm, new Date()) }));
        }
        const previousState = previous.detector && previous.detector.state;
        const nextState = next.detector && next.detector.state;
        if (previousState !== nextState) announce(inst, detectorSummary(inst.ctx, next.detector).text);
    }

    function applyChanges(inst, changes, initial, snapshotFeed) {
        const data = changes && typeof changes === 'object' ? changes : {};
        const status = data.status && typeof data.status === 'object' ? data.status : null;
        if (status) {
            const previous = inst.status;
            inst.status = status;
            if (!initial) announceStatus(inst, previous, status);
        }
        const raw = data.cursor != null ? data.cursor : (status && status.cursor != null ? status.cursor : null);
        const cursor = Number.isSafeInteger(Number(raw)) && Number(raw) >= 0 ? Number(raw) : null;
        // The cursor only moves forward: an older poll answering late cannot rewind it.
        if (cursor != null && (inst.cursor === null || cursor > inst.cursor)) inst.cursor = cursor;
        else if (inst.cursor === null) inst.cursor = 0;
        const armRows = Array.isArray(data.arm_log) ? data.arm_log : [];
        if (armRows.length) inst.armLog = mergeArmLog(inst.armLog, armRows);
        const incoming = initial || snapshotFeed ? [] : (Array.isArray(data.events) ? data.events : []);
        let refreshDetail = false;
        if (incoming.length) {
            const known = new Set(inst.events.map(event => idOf(event.id)));
            const relevant = incoming.filter(event => known.has(idOf(event && event.id)) || eventMatches(event, inst.filter));
            // Announce an event once, and only when it matches the filter; every id seen is
            // remembered, so an ongoing event outside the filter is not announced on each poll.
            const fresh = relevant.filter(event => !known.has(idOf(event && event.id)) && !inst.announced.has(idOf(event && event.id)));
            if (inst.announced.size > 2000) inst.announced.clear();
            incoming.forEach(event => { if (idOf(event && event.id)) inst.announced.add(idOf(event.id)); });
            inst.events.forEach(event => inst.announced.add(idOf(event.id)));
            inst.events = upsertEvents(inst.events, relevant);
            refreshDetail = inst.selectedId != null && incoming.some(event => idOf(event && event.id) === inst.selectedId);
            if (fresh.length) announce(inst, T(inst.ctx, 'new_event', { camera: fresh[0].camera_name || fresh[0].camera_id || '' }));
        }
        renderStrip(inst);
        renderMain(inst);
        if (refreshDetail) loadDetail(inst, inst.selectedId, true);
    }

    async function loadEvents(inst, reset) {
        if (!reset && inst.loadingMore) return;
        const query = new URLSearchParams();
        query.set('limit', String(PAGE_SIZE));
        if (inst.filter.camera) query.set('stream_id', inst.filter.camera);
        const range = inst.filter.day ? dayRange(inst.filter.day) : null;
        if (range) {
            query.set('from', range.from.toISOString());
            query.set('to', range.to.toISOString());
        }
        if (!reset && inst.nextBefore != null) query.set('before', String(inst.nextBefore));
        const token = ++inst.eventsToken;
        inst.loadingMore = !reset;
        inst.eventsError = '';
        if (reset) {
            inst.events = [];
            inst.nextBefore = null;
            inst.loaded = false;
        }
        renderMain(inst);
        try {
            const page = await request(inst, 'events?' + query.toString());
            if (token !== inst.eventsToken) return;
            const events = Array.isArray(page && page.events) ? page.events : [];
            inst.events = upsertEvents(reset ? [] : inst.events, events);
            inst.nextBefore = page && page.next_before != null ? page.next_before : null;
        } catch (error) {
            if (isAbort(error) || token !== inst.eventsToken) return;
            inst.eventsError = error && error.message ? error.message : T(inst.ctx, 'events_failed');
        }
        inst.loaded = true;
        inst.loadingMore = false;
        if (inst.mounted) renderMain(inst);
    }

    async function loadDetail(inst, id, refresh) {
        const eventId = idOf(id);
        if (!eventId) return;
        const token = ++inst.detailToken;
        if (!refresh) {
            inst.detail = null;
            inst.detailError = '';
            inst.frameIndex = 0;
            renderMain(inst);
        }
        try {
            const detail = await request(inst, 'events/' + eventId);
            if (token !== inst.detailToken || !inst.mounted) return;
            const frames = framesOf(detail);
            if (!refresh) {
                const cover = frames.findIndex(frame => idOf(frame.id) === idOf(detail.cover_frame_id));
                inst.frameIndex = cover >= 0 ? cover : 0;
            } else {
                inst.frameIndex = clamp(inst.frameIndex, 0, Math.max(0, frames.length - 1));
            }
            inst.detail = detail;
            inst.detailError = '';
            if (inst.events.some(event => idOf(event.id) === eventId)) {
                const summary = Object.assign({}, detail);
                delete summary.frames;
                inst.events = upsertEvents(inst.events, [summary]);
            }
        } catch (error) {
            if (isAbort(error) || token !== inst.detailToken) return;
            inst.detail = null;
            if (error && (error.status === 404 || (error.body && error.body.code === 'event_not_found'))) {
                // The change feed has no deletion tombstones: a 404 means the event was deleted elsewhere.
                inst.events = inst.events.filter(event => idOf(event.id) !== eventId);
                if (inst.selectedId === eventId) inst.selectedId = null;
                inst.detailError = '';
                notify(inst, T(inst.ctx, 'event_gone'));
                announce(inst, T(inst.ctx, 'event_gone'));
            } else {
                inst.detailError = (error && error.message) || T(inst.ctx, 'event_failed');
            }
        }
        if (inst.mounted) renderMain(inst);
    }

    function selectEvent(inst, id) {
        const eventId = idOf(id);
        if (!eventId) return;
        inst.selectedId = eventId;
        loadDetail(inst, eventId, false);
    }

    function stepFrame(inst, delta) {
        const frames = framesOf(inst.detail);
        if (!frames.length) return;
        const next = clamp(inst.frameIndex + delta, 0, frames.length - 1);
        if (next === inst.frameIndex) return;
        inst.frameIndex = next;
        renderDetail(inst);
    }

    async function setArm(inst, mode) {
        if (!ARM_MODES.includes(mode) || inst.armBusy || !inst.options.canManage) return;
        inst.armBusy = true;
        renderStrip(inst);
        try {
            const arm = await request(inst, 'arm', { method: 'POST', body: { mode } });
            if (!inst.mounted) return;
            if (inst.status && arm && typeof arm === 'object') inst.status = Object.assign({}, inst.status, { arm });
            announce(inst, T(inst.ctx, 'arm_changed', { state: armSummary(inst.ctx, arm, new Date()) }));
            schedulePoll(inst, 300);
        } catch (error) {
            if (isAbort(error)) return;
            notify(inst, (error && error.message) || T(inst.ctx, 'arm_failed'), 'error');
        } finally {
            inst.armBusy = false;
            if (inst.mounted) renderStrip(inst);
        }
    }

    async function deleteEvent(inst) {
        const detail = inst.detail;
        const id = idOf(detail && detail.id);
        if (!id || inst.deleting || !inst.options.canManage || typeof inst.ctx.confirmDialog !== 'function') return;
        const confirmed = await inst.ctx.confirmDialog(T(inst.ctx, 'delete_confirm_title'), T(inst.ctx, 'delete_confirm_message', { count: framesOf(detail).length }));
        if (!confirmed || !inst.mounted) return;
        inst.deleting = true;
        renderDetail(inst);
        try {
            await request(inst, 'events/' + id, { method: 'DELETE' });
            inst.events = inst.events.filter(event => idOf(event.id) !== id);
            inst.selectedId = null;
            inst.detail = null;
            inst.detailError = '';
            notify(inst, T(inst.ctx, 'event_deleted'));
        } catch (error) {
            if (isAbort(error)) return;
            if (error && (error.status === 404 || (error.body && error.body.code === 'event_not_found'))) {
                inst.events = inst.events.filter(event => idOf(event.id) !== id);
                inst.selectedId = null;
                inst.detail = null;
                inst.detailError = '';
                notify(inst, T(inst.ctx, 'event_gone'));
                announce(inst, T(inst.ctx, 'event_gone'));
            } else {
                notify(inst, (error && error.message) || T(inst.ctx, 'event_failed'), 'error');
            }
        } finally {
            inst.deleting = false;
            if (inst.mounted) renderMain(inst);
        }
    }

    async function retryDetector(inst) {
        if (inst.busy || !inst.options.canManage) return;
        inst.busy = true;
        renderStrip(inst);
        try {
            await request(inst, 'retry', { method: 'POST', body: {} });
            notify(inst, T(inst.ctx, 'retry_started'));
            schedulePoll(inst, 500);
        } catch (error) {
            if (!isAbort(error)) notify(inst, (error && error.message) || T(inst.ctx, 'state_error'), 'error');
        } finally {
            inst.busy = false;
            if (inst.mounted) renderStrip(inst);
        }
    }

    function settingsSaved(inst, result) {
        if (result && result.detection && typeof result.detection === 'object') inst.options = Object.assign({}, inst.options, { detectionSettings: result.detection });
        // reloadAppState remounts the mode, which cancels this quick poll and would wait for
        // the rest of the 5 s cycle; with no last poll the remount polls at once instead.
        inst.lastPollAt = 0;
        schedulePoll(inst, 300);
        if (typeof inst.options.reloadAppState === 'function') inst.options.reloadAppState();
    }

    async function activate(inst) {
        if (inst.busy || !inst.options.canManage) return;
        if (!inst.options.detectionSettings) {
            notify(inst, T(inst.ctx, 'settings_load_failed'), 'error');
            return;
        }
        const payload = Object.assign(globalDefaults(inst.options.detectionSettings), { enabled: true });
        inst.busy = true;
        renderMain(inst);
        try {
            const result = await request(inst, 'settings', { method: 'PUT', body: payload });
            notify(inst, inst.ctx.t(result && result.status === 'degraded' ? 'desktop.network_cameras.saved_degraded' : NS + 'activated'));
            inst.busy = false;
            settingsSaved(inst, result);
        } catch (error) {
            if (!isAbort(error)) notify(inst, (error && error.message) || T(inst.ctx, 'settings_load_failed'), 'error');
        } finally {
            inst.busy = false;
            if (inst.mounted) renderMain(inst);
        }
    }

    function openSettings(inst) {
        if (!inst.options.canManage) return;
        const opener = document.activeElement && inst.root.contains(document.activeElement) ? document.activeElement.getAttribute('data-focus-key') : '';
        inst.dialogOpener = opener || 'settings';
        // Save waits for fresh settings, so a schedule saved elsewhere is never overwritten.
        const dialog = { values: inst.options.detectionSettings ? globalDefaults(inst.options.detectionSettings) : null, busy: true, error: '', dirty: false };
        inst.dialog = dialog;
        renderDialog(inst);
        const first = inst.root.querySelector('[data-ncd-dialog] input, [data-ncd-dialog] footer button');
        if (first) first.focus();
        refreshDialogSettings(inst, dialog);
    }

    async function refreshDialogSettings(inst, dialog) {
        try {
            const state = await request(inst, '/api/desktop/integrations/go2rtc/app/state');
            const fresh = state && state.detection_settings && typeof state.detection_settings === 'object' ? state.detection_settings : null;
            if (inst.dialog !== dialog) return;
            if (fresh) {
                inst.options = Object.assign({}, inst.options, { detectionSettings: fresh });
                if (!dialog.dirty) dialog.values = globalDefaults(fresh);
            }
        } catch (error) {
            if (isAbort(error) || inst.dialog !== dialog) return;
        }
        dialog.busy = false;
        // renderRegion keeps the focused dialog control (all of them carry a focus key).
        rerenderDialog(inst, null);
    }

    function closeSettings(inst) {
        inst.dialog = null;
        renderDialog(inst);
        const opener = inst.root.querySelector('[data-focus-key="' + cssEscape(inst.dialogOpener || 'settings') + '"]') || inst.root.querySelector('[data-ncd-action="settings"]');
        if (opener) opener.focus();
    }

    function trapDialogFocus(inst, event) {
        const dialog = inst.root.querySelector('[data-ncd-dialog]');
        if (!dialog) return;
        const focusables = Array.from(dialog.querySelectorAll('button, input, select, textarea, [tabindex="0"]')).filter(node => !node.disabled && node.offsetParent !== null);
        if (!focusables.length) return;
        const first = focusables[0];
        const last = focusables[focusables.length - 1];
        if (event.shiftKey && (document.activeElement === first || !dialog.contains(document.activeElement))) {
            event.preventDefault();
            last.focus();
        } else if (!event.shiftKey && (document.activeElement === last || !dialog.contains(document.activeElement))) {
            event.preventDefault();
            first.focus();
        }
    }

    function rerenderDialog(inst, focusSelector) {
        inst.signatures.dialog = null;
        renderDialog(inst);
        const target = focusSelector ? inst.root.querySelector('[data-region="dialog"] ' + focusSelector) : null;
        if (target) target.focus();
    }

    function updateSettingsField(inst, target) {
        const values = inst.dialog && inst.dialog.values;
        if (!values) return;
        inst.dialog.dirty = true;
        const field = target.dataset.setField;
        if (field === 'enabled') values.enabled = target.checked;
        else if (field === 'model' && target.checked) values.model = target.value === 'accurate' ? 'accurate' : 'standard';
        else if (field === 'backend') values.backend = target.value === 'gpu' || target.value === 'cpu' ? target.value : 'auto';
        else if (field === 'retention_days' || field === 'max_storage_mb') values[field] = target.value === '' ? NaN : Number(target.value);
    }

    function updateWindowField(inst, target) {
        const values = inst.dialog && inst.dialog.values;
        if (!values) return;
        const isFrom = target.dataset.windowFrom !== undefined;
        const slot = values.schedule[Number(isFrom ? target.dataset.windowFrom : target.dataset.windowTo)];
        if (!slot) return;
        inst.dialog.dirty = true;
        if (isFrom) slot.from = target.value;
        else slot.to = target.value;
    }

    function toggleWindowDay(inst, button) {
        const values = inst.dialog && inst.dialog.values;
        if (!values) return;
        const parts = String(button.dataset.windowDay).split(':');
        const slot = values.schedule[Number(parts[0])];
        const day = parts[1];
        if (!slot || !DAY_KEYS.includes(day)) return;
        inst.dialog.dirty = true;
        const on = !slot.days.includes(day);
        slot.days = DAY_KEYS.filter(key => (key === day ? on : slot.days.includes(key)));
        button.setAttribute('aria-pressed', String(on));
        inst.signatures.dialog = null;
    }

    function addWindow(inst) {
        const values = inst.dialog && inst.dialog.values;
        if (!values) return;
        if (values.schedule.length >= MAX_WINDOWS) {
            inst.dialog.error = T(inst.ctx, 'schedule_limit');
            rerenderDialog(inst, '[data-ncd-action="add-window"]');
            return;
        }
        inst.dialog.dirty = true;
        values.schedule.push({ days: DAY_KEYS.slice(), from: '22:00', to: '06:00' });
        rerenderDialog(inst, '[data-window="' + (values.schedule.length - 1) + '"] .ncd-day-chip');
    }

    function removeWindow(inst, index) {
        const values = inst.dialog && inst.dialog.values;
        if (!values || !values.schedule[index]) return;
        inst.dialog.dirty = true;
        inst.dialog.error = '';
        values.schedule.splice(index, 1);
        rerenderDialog(inst, '[data-ncd-action="add-window"]');
    }

    async function saveSettings(inst) {
        const dialog = inst.dialog;
        if (!dialog || !dialog.values || dialog.busy) return;
        const payload = settingsPayload(dialog.values);
        const problem = validateGlobalSettings(payload);
        if (problem) {
            dialog.error = T(inst.ctx, problem.key, problem.params);
            rerenderDialog(inst, '[data-ncd-action="save-settings"]');
            return;
        }
        dialog.busy = true;
        dialog.error = '';
        rerenderDialog(inst, null);
        try {
            const result = await request(inst, 'settings', { method: 'PUT', body: payload });
            if (inst.dialog !== dialog) return;
            inst.dialog = null;
            renderDialog(inst);
            notify(inst, inst.ctx.t(result && result.status === 'degraded' ? 'desktop.network_cameras.saved_degraded' : NS + 'settings_saved'));
            settingsSaved(inst, result);
        } catch (error) {
            if (isAbort(error) || inst.dialog !== dialog) return;
            dialog.busy = false;
            const field = error && error.body && error.body.field ? String(error.body.field) + ': ' : '';
            dialog.error = field + ((error && error.message) || T(inst.ctx, 'settings_load_failed'));
            rerenderDialog(inst, '[data-ncd-action="save-settings"]');
        }
    }

    function openCameraSettings(inst, streamID) {
        if (typeof inst.options.openCameraSettings === 'function') inst.options.openCameraSettings(String(streamID));
    }

    function onClick(inst, event) {
        const target = event.target && event.target.closest ? event.target.closest('button') : null;
        if (!target || !inst.root.contains(target) || target.disabled || target.getAttribute('aria-disabled') === 'true') return;
        const data = target.dataset;
        if (data.arm) { setArm(inst, data.arm); return; }
        if (data.event) { selectEvent(inst, data.event); return; }
        if (data.ncdCamera !== undefined) { openCameraSettings(inst, data.ncdCamera); return; }
        if (data.windowDay) { toggleWindowDay(inst, target); return; }
        if (data.windowRemove !== undefined) { removeWindow(inst, Number(data.windowRemove)); return; }
        const action = data.ncdAction;
        if (action === 'retry') retryDetector(inst);
        else if (action === 'settings') openSettings(inst);
        else if (action === 'close-settings') closeSettings(inst);
        else if (action === 'save-settings') saveSettings(inst);
        else if (action === 'add-window') addWindow(inst);
        else if (action === 'activate') activate(inst);
        else if (action === 'more') loadEvents(inst, false);
        else if (action === 'prev-frame') stepFrame(inst, -1);
        else if (action === 'next-frame') stepFrame(inst, 1);
        else if (action === 'delete-event') deleteEvent(inst);
        else if (action === 'clear-day') {
            inst.filter = Object.assign({}, inst.filter, { day: '' });
            loadEvents(inst, true);
        } else if (action === 'reload') {
            // The failed first load left a poll scheduled; drop it so only one first load runs.
            if (inst.pollTimer) clearTimeout(inst.pollTimer);
            inst.pollTimer = null;
            inst.cursor = null;
            inst.connectionLost = false;
            renderMain(inst);
            initialLoad(inst);
        }
    }

    function onFieldEvent(inst, event, committed) {
        const target = event.target;
        if (!target || !target.dataset) return;
        if (committed && target.dataset.ncdFilter === 'camera') {
            inst.filter = Object.assign({}, inst.filter, { camera: target.value });
            loadEvents(inst, true);
            return;
        }
        if (committed && target.dataset.ncdFilter === 'day') {
            inst.filter = Object.assign({}, inst.filter, { day: /^\d{4}-\d{2}-\d{2}$/.test(target.value) ? target.value : '' });
            loadEvents(inst, true);
            return;
        }
        if (target.dataset.setField) updateSettingsField(inst, target);
        else if (target.dataset.windowFrom !== undefined || target.dataset.windowTo !== undefined) updateWindowField(inst, target);
    }

    function onKeydown(inst, event) {
        const target = event.target;
        if (!target || !target.closest) return;
        if (event.key === 'Escape' && inst.dialog && target.closest('[data-ncd-dialog]')) {
            event.preventDefault();
            event.stopPropagation();
            closeSettings(inst);
            return;
        }
        if (event.key === 'Tab' && inst.dialog && target.closest('[data-ncd-dialog]')) {
            trapDialogFocus(inst, event);
            return;
        }
        if (target.classList.contains('ncd-row') && ['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) {
            const rows = Array.from(inst.root.querySelectorAll('.ncd-row'));
            const index = rows.indexOf(target);
            const next = event.key === 'Home' ? 0 : event.key === 'End' ? rows.length - 1 : clamp(index + (event.key === 'ArrowDown' ? 1 : -1), 0, rows.length - 1);
            if (rows[next]) {
                event.preventDefault();
                rows[next].focus();
            }
            return;
        }
        if ((event.key === 'ArrowLeft' || event.key === 'ArrowRight') && target.closest('.ncd-detail') && !/^(INPUT|SELECT|TEXTAREA)$/.test(target.tagName)) {
            event.preventDefault();
            stepFrame(inst, event.key === 'ArrowLeft' ? -1 : 1);
        }
    }

    function bindRoot(inst) {
        const root = inst.root;
        const on = (type, handler) => {
            root.addEventListener(type, handler);
            inst.listeners.push(() => root.removeEventListener(type, handler));
        };
        on('click', event => onClick(inst, event));
        on('change', event => onFieldEvent(inst, event, true));
        on('input', event => onFieldEvent(inst, event, false));
        on('keydown', event => onKeydown(inst, event));
    }

    window.NetworkCamerasDetection = { mount, unmount, cameraSettingsMarkup, bindCameraSettings, validateCameraSettings };
})();
