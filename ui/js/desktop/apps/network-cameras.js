(function () {
    'use strict';

    const instances = new Map();
    const preferenceKey = 'aurago.desktop.network-cameras';
    const refreshInterval = 5000;
    const translationNamespace = 'desktop.network_cameras';

    function text(ctx, key, params) {
        return ctx.t(translationNamespace + '.' + key, params || {});
    }

    // ctx.notify is showDesktopNotification(payload): it needs an object, a bare string loses its text.
    function notify(state, message, type) {
        if (typeof state.ctx.notify !== 'function' || !message) return;
        state.ctx.notify({ title: text(state.ctx, 'title'), message: String(message), appId: 'network-cameras', type: type === 'error' ? 'error' : 'info' });
    }

    const iconAliases = Object.freeze({ radar: 'search', link: 'globe', plus: 'file-plus', minimize: 'minus' });

    function appIcon(ctx, key, fallback, size) {
        const resolved = iconAliases[key] || key;
        return ctx.iconMarkup(resolved, fallback, 'vd-sprite-icon nc-glyph', size);
    }

    function requirementText(ctx, requirement) {
        const key = translationNamespace + '.requirement_' + String(requirement && requirement.code || 'unknown');
        const translated = ctx.t(key);
        return translated && translated !== key ? translated : String(requirement && requirement.message || '');
    }

    function mutationNoticeKey(result, successKey) {
        return result && result.status === 'degraded' ? 'saved_degraded' : successKey;
    }

    function coerceMode(value) {
        return value === 'live' || value === 'detections' ? value : 'snapshots';
    }

    // Notifications target an event; Infra targets a configured camera without starting live playback.
    function launchContext(context) {
        const eventID = context ? context.event_id : undefined;
        const cameraID = context && typeof context.camera_id === 'string' && /^[a-z0-9_-]{1,64}$/.test(context.camera_id) ? context.camera_id : '';
        if (!Number.isSafeInteger(eventID) || eventID <= 0) return cameraID ? { camera_id: cameraID } : null;
        return cameraID ? { event_id: eventID, camera_id: cameraID } : { event_id: eventID };
    }

    function readPreferences() {
        try {
            const value = JSON.parse(localStorage.getItem(preferenceKey) || '{}');
            return {
                mode: coerceMode(value.mode),
                selected: String(value.selected || ''),
                sourceRoles: value.sourceRoles && typeof value.sourceRoles === 'object' ? value.sourceRoles : {}
            };
        } catch (_) {
            return { mode: 'snapshots', selected: '', sourceRoles: {} };
        }
    }

    function savePreferences(state) {
        try {
            localStorage.setItem(preferenceKey, JSON.stringify({ mode: state.mode, selected: state.selected, sourceRoles: state.sourceRoles }));
        } catch (_) {}
    }

    function createState(host, windowId, ctx) {
        const preferences = readPreferences();
        return {
            host, windowId, ctx,
            disposed: false,
            visible: true,
            data: null,
            loading: true,
            error: '',
            filter: '',
            selected: preferences.selected,
            sourceRoles: preferences.sourceRoles,
            savePreferences: () => savePreferences(instances.get(windowId)),
            mode: launchContext(ctx) ? (launchContext(ctx).event_id ? 'detections' : 'snapshots') : preferences.mode,
            focus: false,
            modal: null,
            deletingID: '',
            visibleIDs: new Set(),
            controllers: new Set(),
            objectURLs: new Map(),
            thumbnailTimer: null,
            statusTimer: null,
            stateRequest: null,
            intersection: null,
            visibilityObserver: null,
            documentVisibilityHandler: null,
            detectionSettings: null,
            pendingLaunch: launchContext(ctx)
        };
    }

    async function request(state, path, options) {
        const controller = new AbortController();
        state.controllers.add(controller);
        const requestOptions = Object.assign({}, options || {}, { signal: controller.signal });
        if (requestOptions.body && typeof requestOptions.body !== 'string') {
            requestOptions.headers = Object.assign({ 'Content-Type': 'application/json' }, requestOptions.headers || {});
            requestOptions.body = JSON.stringify(requestOptions.body);
        }
        try {
            return await state.ctx.api(path, requestOptions);
        } finally {
            state.controllers.delete(controller);
        }
    }

    function activeStreams(state) {
        return (state.data && Array.isArray(state.data.streams)) ? state.data.streams : [];
    }

    function allStreams(state) {
        const active = activeStreams(state).map(stream => Object.assign({ enabled: true }, stream));
        const disabled = state.data && state.data.can_manage && Array.isArray(state.data.disabled_streams)
            ? state.data.disabled_streams.map(stream => Object.assign({ reachable: false, codecs: [], consumers: 0 }, stream))
            : [];
        return active.concat(disabled);
    }

    function filteredStreams(state) {
        const filter = state.filter.trim().toLocaleLowerCase();
        return allStreams(state).filter(stream => !filter || String(stream.name || stream.id).toLocaleLowerCase().includes(filter) || String(stream.id).toLocaleLowerCase().includes(filter));
    }

    function selectedStream(state) {
        return allStreams(state).find(stream => stream.id === state.selected && stream.enabled !== false) || null;
    }

    function ensureSelection(state) {
        const launch = state.pendingLaunch;
        if (launch && launch.camera_id && !launch.event_id) {
            state.pendingLaunch = null;
            const camera = activeStreams(state).find(stream => stream.id === launch.camera_id);
            state.selected = camera ? camera.id : '';
            state.focus = !!camera;
            state.filter = '';
            if (!camera) state.error = text(state.ctx, 'no_results');
            savePreferences(state);
            return;
        }
        if (selectedStream(state)) return;
        const first = activeStreams(state)[0];
        state.selected = first ? first.id : '';
        savePreferences(state);
    }

    function statusMarkup(state) {
        const ctx = state.ctx;
        const runtime = state.data && state.data.integration;
        if (!runtime) return '';
        const classes = runtime.api_usable ? 'is-online' : 'is-offline';
        const label = runtime.api_usable ? text(ctx, 'online') : text(ctx, 'offline');
        return '<span class="nc-runtime ' + classes + '"><span></span>' + ctx.esc(label) + '</span>';
    }

    function toolbarMarkup(state) {
        const ctx = state.ctx;
        const canManage = !!(state.data && state.data.can_manage);
        return '<header class="nc-toolbar">' +
            '<div class="nc-brand"><span class="nc-brand-icon">' + appIcon(ctx, 'camera', 'C', 20) + '</span><div><strong>' + ctx.esc(text(ctx, 'title')) + '</strong>' + statusMarkup(state) + '</div></div>' +
            '<label class="nc-search"><span>' + appIcon(ctx, 'search', 'S', 16) + '</span><input type="search" data-role="filter" value="' + ctx.esc(state.filter) + '" placeholder="' + ctx.esc(text(ctx, 'search')) + '"></label>' +
            '<div class="nc-toolbar-actions">' +
                '<div class="nc-segmented" role="group" aria-label="' + ctx.esc(text(ctx, 'grid_mode')) + '">' +
                    '<button type="button" data-mode="snapshots" aria-pressed="' + (state.mode === 'snapshots') + '" class="' + (state.mode === 'snapshots' ? 'is-active' : '') + '">' + ctx.esc(text(ctx, 'snapshots')) + '</button>' +
                    '<button type="button" data-mode="live" aria-pressed="' + (state.mode === 'live') + '" class="' + (state.mode === 'live' ? 'is-active' : '') + '">' + ctx.esc(text(ctx, 'live_grid')) + '</button>' +
                    '<button type="button" data-mode="detections" aria-pressed="' + (state.mode === 'detections') + '" class="' + (state.mode === 'detections' ? 'is-active' : '') + '">' + ctx.esc(text(ctx, 'detection.mode')) + '</button>' +
                '</div>' +
                '<button class="nc-icon-button" type="button" data-action="refresh" title="' + ctx.esc(text(ctx, 'refresh')) + '">' + appIcon(ctx, 'refresh', 'R', 17) + '</button>' +
                (canManage ? '<button type="button" data-action="tuya">' + ctx.esc(ctx.t('common.go2rtc_tuya.accounts')) + '</button><button class="nc-primary" type="button" data-action="add" aria-label="' + ctx.esc(text(ctx, 'add_camera')) + '">' + appIcon(ctx, 'plus', '+', 16) + '<span>' + ctx.esc(text(ctx, 'add_camera')) + '</span></button>' : '') +
            '</div></header>';
    }

    function emptyMarkup(state) {
        const ctx = state.ctx;
        const runtime = state.data && state.data.integration;
        const canManage = !!(state.data && state.data.can_manage);
        const error = state.error ? '<div class="nc-modal-error">' + ctx.esc(state.error) + '</div>' : '';
        const noCameras = allStreams(state).length === 0;
        const onboardingClass = 'nc-onboarding' + (noCameras ? ' has-artwork' : '');
        const illustration = noCameras
            ? '<img class="nc-onboarding-art" src="/img/network-cameras/placeholder.png" width="1254" height="1254" alt="" aria-hidden="true" draggable="false">'
            : '<div class="nc-onboarding-icon">' + appIcon(ctx, 'camera', 'C', 34) + '</div>';
        if (runtime && !runtime.enabled) {
            return '<div class="' + onboardingClass + '">' + illustration + '<h2>' + ctx.esc(text(ctx, canManage ? 'enable_title' : 'disabled_title')) + '</h2><p>' + ctx.esc(text(ctx, canManage ? 'enable_description' : 'disabled_description')) + '</p>' +
                error + (canManage ? '<div class="nc-onboarding-actions"><button class="nc-primary" type="button" data-action="enable">' + ctx.esc(text(ctx, 'enable_action')) + '</button><a href="/config#go2rtc" target="_blank" rel="noopener">' + ctx.esc(text(ctx, 'open_settings')) + '</a></div>' : '') + '</div>';
        }
        if (runtime && !runtime.api_usable) {
            return '<div class="nc-onboarding"><div class="nc-onboarding-icon is-warning">!</div><h2>' + ctx.esc(text(ctx, 'unavailable_title')) + '</h2><p>' + ctx.esc(runtime.last_error || text(ctx, 'unavailable_description')) + '</p><div class="nc-onboarding-actions"><button class="nc-primary" type="button" data-action="refresh">' + ctx.esc(text(ctx, 'retry')) + '</button>' + (canManage ? '<a href="/config#go2rtc" target="_blank" rel="noopener">' + ctx.esc(text(ctx, 'open_settings')) + '</a>' : '') + '</div></div>';
        }
        return '<div class="' + onboardingClass + '">' + illustration + '<h2>' + ctx.esc(text(ctx, 'empty_title')) + '</h2><p>' + ctx.esc(text(ctx, canManage ? 'empty_admin_description' : 'empty_viewer_description')) + '</p>' + error + (canManage ? '<button class="nc-primary" type="button" data-action="add">' + ctx.esc(text(ctx, 'setup_camera')) + '</button>' : '') + '</div>';
    }

    function liveGridIDs(state, streams) {
        if (state.mode !== 'live' || !state.visible) return new Set();
        const ids = [];
        if (state.selected && streams.some(stream => stream.id === state.selected && stream.enabled !== false)) ids.push(state.selected);
        for (const stream of streams) {
            if (stream.enabled === false || ids.includes(stream.id)) continue;
            ids.push(stream.id);
            if (ids.length >= 4) break;
        }
        return new Set(ids);
    }

    function cardMarkup(state, stream, liveIDs) {
        const ctx = state.ctx;
        const enabled = stream.enabled !== false;
        const selected = enabled && stream.id === state.selected;
        const live = enabled && liveIDs.has(stream.id);
        const deleting = state.deletingID === stream.id;
        const codecs = Array.isArray(stream.codecs) && stream.codecs.length ? stream.codecs.join(' · ') : text(ctx, enabled ? 'waiting_codec' : 'disabled');
        const media = live
            ? '<iframe data-camera="' + ctx.esc(stream.id) + '" data-revision="' + Number(stream.revision || 0) + '" data-source-role="preview" title="' + ctx.esc(stream.name || stream.id) + '" src="' + window.NetworkCamerasMedia.viewerURL(state, stream, 'preview') + '" loading="lazy"></iframe>'
            : (enabled ? '<img data-thumbnail="' + ctx.esc(stream.id) + '" alt="' + ctx.esc(stream.name || stream.id) + '"><div class="nc-image-placeholder">' + appIcon(ctx, 'camera', 'C', 25) + '</div>' : '<div class="nc-disabled-placeholder">' + appIcon(ctx, 'camera', 'C', 25) + '<span>' + ctx.esc(text(ctx, 'disabled')) + '</span></div>');
        return '<article class="nc-card ' + (selected ? 'is-selected ' : '') + (!enabled ? 'is-disabled' : '') + '" data-stream-card="' + ctx.esc(stream.id) + '">' +
            '<button type="button" class="nc-card-select" data-select="' + ctx.esc(stream.id) + '" ' + (!enabled ? 'disabled' : '') + ' aria-label="' + ctx.esc(stream.name || stream.id) + '"></button>' +
            '<div class="nc-card-media">' + media + '<span class="nc-live-badge">' + ctx.esc(live ? text(ctx, 'live') : '') + '</span></div>' +
            '<div class="nc-card-copy"><div><strong>' + ctx.esc(stream.name || stream.id) + '</strong><small>' + ctx.esc(codecs) + (stream.source_type ? '<span class="go2rtc-source-badge">' + ctx.esc(window.AuraGoSourceForm.label(state.sourceTypes, stream.source_type)) + '</span>' : '') + '</small></div>' +
                '<div class="nc-card-state ' + (stream.reachable ? 'is-online' : '') + '"><span></span>' + ctx.esc(enabled ? (stream.reachable ? text(ctx, 'online') : text(ctx, stream.state === 'connecting' ? 'connecting' : 'idle')) : text(ctx, 'disabled')) + '</div></div>' +
            '<div class="nc-card-meta"><span>' + ctx.esc(text(ctx, 'viewers', { count: Number(stream.consumers || 0) })) + '</span>' + (state.data.can_manage ? '<span class="nc-card-admin-actions"><button type="button" data-edit="' + ctx.esc(stream.id) + '" title="' + ctx.esc(text(ctx, 'manage')) + '" ' + (deleting ? 'disabled' : '') + '>' + appIcon(ctx, 'settings', 'M', 15) + '</button><button class="is-danger" type="button" data-delete="' + ctx.esc(stream.id) + '" title="' + ctx.esc(text(ctx, 'delete')) + '" ' + (deleting ? 'disabled' : '') + '>' + (deleting ? '<span class="nc-spinner is-small"></span>' : appIcon(ctx, 'trash', 'X', 15)) + '</button></span>' : '') + '</div>' +
            '</article>';
    }

    function detailMarkup(state) {
        const ctx = state.ctx;
        const stream = selectedStream(state);
        if (!stream) return '<div class="nc-detail-empty">' + ctx.esc(text(ctx, 'select_camera')) + '</div>';
        const viewerURL = window.NetworkCamerasMedia.viewerURL(state, stream);
        const deleting = state.deletingID === stream.id;
        return '<section class="nc-detail">' +
            '<div class="nc-detail-header"><div><strong>' + ctx.esc(stream.name || stream.id) + '</strong><small>' + ctx.esc(stream.id) + '</small></div><div class="nc-detail-actions">' +
                '<button type="button" data-action="snapshot" aria-label="' + ctx.esc(text(ctx, 'take_snapshot')) + '">' + appIcon(ctx, 'camera', 'C', 15) + '<span>' + ctx.esc(text(ctx, 'take_snapshot')) + '</span></button>' +
                '<button type="button" data-action="reconnect" aria-label="' + ctx.esc(text(ctx, 'reconnect')) + '">' + appIcon(ctx, 'refresh', 'R', 15) + '<span>' + ctx.esc(text(ctx, 'reconnect')) + '</span></button>' +
                '<button type="button" data-action="focus" aria-label="' + ctx.esc(text(ctx, state.focus ? 'leave_focus' : 'focus')) + '">' + appIcon(ctx, state.focus ? 'minimize' : 'maximize', 'F', 15) + '<span>' + ctx.esc(text(ctx, state.focus ? 'leave_focus' : 'focus')) + '</span></button>' +
                '<button type="button" data-action="fullscreen" aria-label="' + ctx.esc(text(ctx, 'fullscreen')) + '">' + appIcon(ctx, 'maximize', 'F', 15) + '<span>' + ctx.esc(text(ctx, 'fullscreen')) + '</span></button>' +
                (state.data.can_manage ? '<button class="nc-danger-action" type="button" aria-label="' + ctx.esc(text(ctx, 'delete')) + '" data-delete="' + ctx.esc(stream.id) + '" ' + (deleting ? 'disabled' : '') + '>' + (deleting ? '<span class="nc-spinner is-small"></span>' : appIcon(ctx, 'trash', 'X', 15)) + '<span>' + ctx.esc(text(ctx, 'delete')) + '</span></button>' : '') +
            '</div></div><div class="nc-media-toolbar" data-role="media-toolbar"></div><div class="nc-detail-video" data-role="detail-video"><iframe data-camera="' + ctx.esc(stream.id) + '" data-revision="' + Number(stream.revision || 0) + '" title="' + ctx.esc(stream.name || stream.id) + '" src="' + viewerURL + '" allow="fullscreen; autoplay" referrerpolicy="same-origin"></iframe></div><div class="nc-media-stats" data-role="media-stats"></div><div class="nc-media-diagnosis" data-role="media-error" aria-live="polite"></div><output class="nc-media-diagnosis" data-role="media-diagnosis" aria-live="polite"></output></section>';
    }

    function mainMarkup(state) {
        if (state.loading) return '<div class="nc-centered"><span class="nc-spinner"></span>' + state.ctx.esc(text(state.ctx, 'loading')) + '</div>';
        if (state.error && !state.data) return '<div class="nc-centered is-error"><strong>' + state.ctx.esc(text(state.ctx, 'load_error')) + '</strong><span>' + state.ctx.esc(state.error) + '</span><button class="nc-primary" data-action="refresh">' + state.ctx.esc(text(state.ctx, 'retry')) + '</button></div>';
        const runtime = state.data && state.data.integration;
        // AuraGo serves the detection API itself: history, arming, deletion and settings keep
        // working while go2rtc is stopped or its container is being recreated.
        if (state.mode === 'detections' && runtime && runtime.enabled && allStreams(state).length > 0) return '<div class="nc-detection-host" data-role="detection"></div>';
        if (!runtime || !runtime.enabled || allStreams(state).length === 0) return emptyMarkup(state);
        const streams = filteredStreams(state);
        const liveIDs = liveGridIDs(state, streams);
        const liveGrid = state.mode === 'live';
        const focus = state.focus && !liveGrid;
        return '<div class="nc-layout ' + (focus ? 'is-focus ' : '') + (liveGrid ? 'is-live-grid' : '') + '"><aside class="nc-grid-pane"><div class="nc-grid" data-role="grid">' + (streams.length ? streams.map(stream => cardMarkup(state, stream, liveIDs)).join('') : '<div class="nc-no-results">' + state.ctx.esc(text(state.ctx, 'no_results')) + '</div>') + '</div></aside>' + (liveGrid ? '' : detailMarkup(state)) + '</div>';
    }

    function draw(state) {
        if (state.disposed) return;
        window.NetworkCamerasMedia.stop(state);
        disconnectIntersection(state);
        state.host.innerHTML = '<div class="network-cameras-app">' + toolbarMarkup(state) + '<main class="nc-content">' + mainMarkup(state) + '</main><div class="nc-modal-host" data-role="modal"></div></div>';
        bindMainEvents(state);
        if (state.modal) drawModal(state);
        observeCards(state);
        scheduleThumbnails(state, true);
        setWindowMenus(state);
        syncDetection(state);
        window.NetworkCamerasMedia.update(state);
    }

    function detectionModule() {
        const module = window.NetworkCamerasDetection;
        return module && typeof module.mount === 'function' ? module : null;
    }

    function syncDetection(state) {
        const module = detectionModule();
        if (!module) return;
        const target = state.host.querySelector('[data-role="detection"]');
        if (!target || !state.visible || state.disposed) {
            module.unmount(state.windowId, { keepState: true });
            return;
        }
        const launch = state.pendingLaunch;
        state.pendingLaunch = null;
        module.mount(target, state.ctx, {
            windowId: state.windowId,
            launch,
            canManage: !!(state.data && state.data.can_manage),
            streams: allStreams(state).map(stream => ({ id: stream.id, name: stream.name || stream.id, enabled: stream.enabled !== false })),
            detectionSettings: state.data && state.data.detection_settings ? state.data.detection_settings : null,
            openCameraSettings: streamID => openManageModal(state, streamID, 'detection'),
            reloadAppState: () => reloadState(state, false)
        });
    }

    function bindMainEvents(state) {
        const host = state.host;
        const filter = host.querySelector('[data-role="filter"]');
        if (filter) filter.addEventListener('input', event => { state.filter = event.target.value; draw(state); });
        host.querySelectorAll('[data-mode]').forEach(button => button.addEventListener('click', () => {
            state.mode = coerceMode(button.dataset.mode);
            if (state.mode === 'live') state.focus = false;
            savePreferences(state);
            draw(state);
        }));
        host.querySelectorAll('[data-select]').forEach(button => button.addEventListener('click', () => {
            state.selected = button.dataset.select || '';
            savePreferences(state);
            draw(state);
        }));
        host.querySelectorAll('[data-edit]').forEach(button => button.addEventListener('click', event => {
            event.stopPropagation();
            openManageModal(state, button.dataset.edit);
        }));
        host.querySelectorAll('[data-delete]').forEach(button => button.addEventListener('click', event => {
            event.stopPropagation();
            deleteStream(state, button.dataset.delete);
        }));
        host.querySelectorAll('[data-action]').forEach(button => button.addEventListener('click', () => handleAction(state, button.dataset.action)));
    }

    async function handleAction(state, action) {
        if (action === 'refresh') return loadState(state, true);
        if (action === 'add') return openSetupModal(state);
        if (action === 'tuya') return openTuya(state, 'accounts');
        if (action === 'enable') return enableIntegration(state);
        if (action === 'focus') {
            if (state.mode === 'detections') state.mode = 'snapshots';
            if (state.mode === 'live') state.mode = 'snapshots';
            state.focus = !state.focus;
            savePreferences(state);
            return draw(state);
        }
        if (action === 'fullscreen') {
            const target = state.host.querySelector('[data-role="detail-video"]');
            if (target && target.requestFullscreen) await target.requestFullscreen().catch(() => {});
            return;
        }
        if (action === 'reconnect') return reconnectViewer(state);
        if (action === 'snapshot') return takeSnapshot(state);
    }

    async function enableIntegration(state) {
        try {
            const result = await request(state, '/api/desktop/integrations/go2rtc/setup/enable', { method: 'POST', body: {} });
            notify(state, text(state.ctx, mutationNoticeKey(result, 'enabled_notice')));
            await reloadState(state, true);
        } catch (error) {
            const requirements = error.body && Array.isArray(error.body.requirements) ? error.body.requirements.map(item => requirementText(state.ctx, item)).join(' · ') : '';
            state.error = requirements || error.message;
            draw(state);
        }
    }

    function reconnectViewer(state) {
        const frame = state.host.querySelector('.nc-detail-video iframe');
        const stream = selectedStream(state);
        if (frame && stream) frame.src = window.NetworkCamerasMedia.viewerURL(state, stream) + '&retry=' + Date.now();
    }

    async function takeSnapshot(state) {
        const stream = selectedStream(state);
        if (!stream) return;
        try {
            await request(state, '/api/desktop/integrations/go2rtc/snapshot', { method: 'POST', body: { stream_id: stream.id, store: true } });
            notify(state, text(state.ctx, 'snapshot_saved'));
            scheduleThumbnails(state, true);
        } catch (error) {
            notify(state, error.message || text(state.ctx, 'snapshot_failed'), 'error');
        }
    }

    function structure(data) {
        if (!data) return '';
        return JSON.stringify([!!data.can_manage, !!data.detection_enabled, data.integration && data.integration.enabled, (data.streams || []).concat(data.disabled_streams || []).map(s => [s.id, s.name, s.enabled])]);
    }

    function scheduleStatus(state) {
        clearTimeout(state.statusTimer);
        state.statusTimer = null;
        if (!state.disposed && state.visible) state.statusTimer = setTimeout(() => loadState(state, false), refreshInterval);
    }

    function loadState(state, redraw) {
        if (state.disposed || !state.visible) return Promise.resolve();
        if (state.stateRequest) return state.stateRequest;
        const previous = structure(state.data);
        if (!state.data) state.loading = true;
        if (redraw && !state.data) draw(state);
        state.stateRequest = (async () => {
            try {
                state.data = await request(state, '/api/desktop/integrations/go2rtc/app/state');
                state.error = '';
                ensureSelection(state);
            } catch (error) {
                if (error.name !== 'AbortError') {
                    state.error = error.message || text(state.ctx, 'load_error');
                    if (state.data && state.data.integration) state.data.integration.api_usable = false;
                }
            } finally {
                state.loading = false;
                state.stateRequest = null;
                if (!state.disposed && state.visible) {
                    if (!state.data || previous !== structure(state.data)) draw(state);
                    else window.NetworkCamerasMedia.update(state);
                }
                scheduleStatus(state);
            }
        })();
        return state.stateRequest;
    }

    async function reloadState(state, redraw) {
        // A pre-mutation poll may still carry the previous configuration.
        if (state.stateRequest) await state.stateRequest;
        return loadState(state, redraw);
    }

    function observeCards(state) {
        const cards = Array.from(state.host.querySelectorAll('[data-stream-card]'));
        state.visibleIDs.clear();
        if (!('IntersectionObserver' in window)) {
            cards.forEach(card => state.visibleIDs.add(card.dataset.streamCard));
            return;
        }
        state.intersection = new IntersectionObserver(entries => {
            let becameVisible = false;
            entries.forEach(entry => {
                const id = entry.target.dataset.streamCard;
                if (entry.isIntersecting) {
                    if (!state.visibleIDs.has(id)) becameVisible = true;
                    state.visibleIDs.add(id);
                } else state.visibleIDs.delete(id);
            });
            if (becameVisible) scheduleThumbnails(state, true);
        }, { root: state.host.querySelector('.nc-grid-pane'), threshold: 0.05 });
        cards.forEach(card => state.intersection.observe(card));
    }

    function disconnectIntersection(state) {
        if (state.intersection) state.intersection.disconnect();
        state.intersection = null;
    }

    function scheduleThumbnails(state, immediate) {
        if (state.thumbnailTimer) clearTimeout(state.thumbnailTimer);
        state.thumbnailTimer = null;
        if (state.disposed || !state.visible) return;
        const run = async () => {
            await refreshThumbnails(state);
            if (!state.disposed && state.visible) state.thumbnailTimer = setTimeout(run, refreshInterval);
        };
        state.thumbnailTimer = setTimeout(run, immediate ? 30 : refreshInterval);
    }

    async function refreshThumbnails(state) {
        if (state.disposed || !state.visible) return;
        const nodes = visibleThumbnailNodes(state);
        let index = 0;
        const worker = async () => {
            while (!state.disposed && index < nodes.length) {
                const node = nodes[index++];
                await loadThumbnail(state, node);
            }
        };
        await Promise.all(Array.from({ length: Math.min(4, nodes.length) }, worker));
    }

    function visibleThumbnailNodes(state) {
        if (state.focus || state.visibleIDs.size === 0) return [];
        return Array.from(state.host.querySelectorAll('img[data-thumbnail]')).filter(node => state.visibleIDs.has(node.dataset.thumbnail));
    }

    async function loadThumbnail(state, node) {
        const id = node.dataset.thumbnail;
        const revision = (activeStreams(state).find(stream => stream.id === id) || {}).revision;
        const controller = new AbortController();
        state.controllers.add(controller);
        try {
            const response = await fetch('/api/desktop/integrations/go2rtc/thumbnail/' + encodeURIComponent(id) + '.jpg?width=640&height=360&cache=5', { credentials: 'same-origin', signal: controller.signal });
            if (!response.ok) throw new Error('thumbnail');
            const blob = await response.blob();
            if (state.disposed || !node.isConnected || revision !== (activeStreams(state).find(stream => stream.id === id) || {}).revision) return;
            const previous = state.objectURLs.get(id);
            const objectURL = URL.createObjectURL(blob);
            state.objectURLs.set(id, objectURL);
            node.src = objectURL;
            node.classList.add('is-loaded');
            if (previous) URL.revokeObjectURL(previous);
        } catch (error) {
            if (error.name !== 'AbortError') node.classList.remove('is-loaded');
        } finally {
            state.controllers.delete(controller);
        }
    }

    function openSetupModal(state) {
        state.modal = { type: 'setup', step: 1, method: '', candidates: [], selectedCandidate: '', address: '', username: '', password: '', profiles: [], setupToken: '', profileID: '', subProfileID: '', useSnapshot: false, snapshotAvailable: null, name: '', id: '', source: '', sourceForm: null, busy: false, error: '' };
        drawModal(state);
    }

    function openManageModal(state, streamID, tab) {
        const stream = allStreams(state).find(item => item.id === streamID);
        if (!stream) return;
        state.modal = {
            type: 'manage', streamID, name: stream.name || stream.id, enabled: stream.enabled !== false, originalEnabled: stream.enabled !== false, source: '', sourceForm: window.AuraGoSourceForm.newState(''), busy: false, error: '',
            tab: tab === 'detection' && detectionTabAvailable(state) ? 'detection' : 'general',
            detection: stream.detection ? JSON.parse(JSON.stringify(stream.detection)) : null,
            detectionDirty: false
        };
        Object.assign(state.modal, window.NetworkCamerasMedia.defaults(stream));
        drawModal(state);
        if (!state.sourceTypes) ensureSourceTypes(state).then(() => refreshSourceChoices(state, streamID)).catch(() => {});
    }

    function manageSourceFormMarkup(state, modal) {
        return window.AuraGoSourceForm.markup(state.sourceTypes || [], modal.sourceForm, { t: state.ctx.t, emptyLabel: state.ctx.t('common.go2rtc_source.keep_current') });
    }

    function bindSourceForm(state, root) {
        if (root && state.modal.sourceForm) window.AuraGoSourceForm.bind(root, state.modal.sourceForm, redraw => { if (redraw) drawModal(state); });
    }

    // Re-render only the source fieldset once the catalog arrives, so the name input keeps focus and typed text.
    function refreshSourceChoices(state, streamID) {
        const modal = state.modal;
        if (!modal || modal.type !== 'manage' || modal.streamID !== streamID || modal.sourceForm.type) return;
        const host = state.host.querySelector('[data-role="modal"]');
        const fieldset = host && host.querySelector('.nc-source-fieldset');
        if (!fieldset) return;
        const legend = fieldset.querySelector('legend');
        fieldset.innerHTML = (legend ? legend.outerHTML : '') + manageSourceFormMarkup(state, modal);
        bindSourceForm(state, fieldset.querySelector('[data-source-form]'));
    }

    function detectionTabAvailable(state) {
        return !!(detectionModule() && state.data && state.data.can_manage);
    }

    function disposeDetectionSettings(state) {
        if (state.detectionSettings) state.detectionSettings.dispose();
        state.detectionSettings = null;
    }

    function manageDetectionStream(state, modal) {
        if (!detectionTabAvailable(state)) return null;
        const stream = allStreams(state).find(item => item.id === modal.streamID) || { id: modal.streamID, name: modal.name };
        // detection_enabled is top-level in app/state; the tab warns only when it is false.
        return Object.assign({}, stream, { detection: modal.detection, detection_enabled: !(state.data && state.data.detection_enabled === false) });
    }

    function manageMarkup(state, modal, detectionStream) {
        const ctx = state.ctx;
        const sourceField = modal.tuya_account_id ? '' : '<fieldset class="nc-source-fieldset"><legend>' + ctx.esc(text(ctx, 'replace_source')) + '</legend>' + manageSourceFormMarkup(state, modal) + '</fieldset>';
        const general = '<div class="nc-form" role="tabpanel" data-tab-panel="general"' + (detectionStream && modal.tab === 'detection' ? ' hidden' : '') + '><label>' + ctx.esc(text(ctx, 'camera_name')) + '<input data-field="name" value="' + ctx.esc(modal.name) + '"></label><label class="nc-check"><input data-field="enabled" type="checkbox" ' + (modal.enabled ? 'checked' : '') + '><span>' + ctx.esc(text(ctx, 'camera_enabled')) + '</span></label>' + sourceField + window.NetworkCamerasMedia.fields(state, modal) + '</div>';
        if (!detectionStream) return general;
        const tab = (id, label) => '<button type="button" role="tab" data-modal-tab="' + id + '" aria-selected="' + (modal.tab === id) + '" tabindex="' + (modal.tab === id ? '0' : '-1') + '" class="' + (modal.tab === id ? 'is-active' : '') + '">' + ctx.esc(label) + '</button>';
        return '<div class="nc-tabs" role="tablist" aria-label="' + ctx.esc(text(ctx, 'manage_title')) + '">' + tab('general', text(ctx, 'tab_general')) + tab('detection', text(ctx, 'detection.tab')) + '</div>' + general +
            '<div class="nc-tab-panel" role="tabpanel" data-tab-panel="detection" data-role="detection-settings"' + (modal.tab === 'detection' ? '' : ' hidden') + '>' + window.NetworkCamerasDetection.cameraSettingsMarkup(detectionStream, ctx, { windowKey: state.windowId }) + '</div>';
    }

    function selectManageTab(state, tab) {
        const modal = state.modal;
        const host = state.host.querySelector('[data-role="modal"]');
        if (!modal || !host) return;
        modal.tab = tab === 'detection' ? 'detection' : 'general';
        host.querySelectorAll('[data-modal-tab]').forEach(button => {
            const active = button.dataset.modalTab === modal.tab;
            button.classList.toggle('is-active', active);
            button.setAttribute('aria-selected', String(active));
            button.tabIndex = active ? 0 : -1;
        });
        host.querySelectorAll('[data-tab-panel]').forEach(panel => { panel.hidden = panel.dataset.tabPanel !== modal.tab; });
    }

    function setupStepMarkup(state, modal) {
        const ctx = state.ctx;
        if (modal.step === 1) return '<div class="nc-methods">' +
            setupMethod(ctx, 'discover', 'radar', 'method_discover', 'method_discover_help', state.data.discovery && state.data.discovery.available, state.data.discovery && state.data.discovery.available ? '' : text(ctx, 'discovery_unavailable')) +
            setupMethod(ctx, 'address', 'network', 'method_address', 'method_address_help', true, '') +
            setupMethod(ctx, 'tuya', 'camera', 'method_tuya', 'method_tuya_help', true, '') +
            setupMethod(ctx, 'type', 'camera', 'method_type', 'method_type_help', true, '') +
            setupMethod(ctx, 'url', 'link', 'method_url', 'method_url_help', true, '') + '</div>';
        if (modal.step === 2) {
            if (modal.method === 'type') return '<div class="nc-form">' + window.AuraGoSourceForm.markup(state.sourceTypes || [], modal.sourceForm, { t: ctx.t }) + '</div>';
            if (modal.method === 'discover' && modal.busy) {
                return '<div class="nc-discovery-progress" role="status" aria-live="polite"><span class="nc-spinner"></span><strong>' + ctx.esc(text(ctx, 'method_discover')) + '</strong><small>' + ctx.esc(text(ctx, 'method_discover_help')) + '</small></div>';
            }
            const candidate = modal.method === 'discover' ? '<label>' + ctx.esc(text(ctx, 'device')) + '<select data-field="selectedCandidate"><option value="">' + ctx.esc(text(ctx, 'choose_device')) + '</option>' + modal.candidates.map(item => '<option value="' + ctx.esc(item.id) + '" ' + (item.id === modal.selectedCandidate ? 'selected' : '') + '>' + ctx.esc(item.name + ' · ' + item.ip + ':' + item.port) + '</option>').join('') + '</select></label>' : '<label>' + ctx.esc(text(ctx, 'onvif_address')) + '<input data-field="address" value="' + ctx.esc(modal.address) + '" placeholder="192.168.1.20"></label>';
            return '<div class="nc-form">' + candidate + '<div class="nc-form-row"><label>' + ctx.esc(text(ctx, 'username')) + '<input data-field="username" autocomplete="username" value="' + ctx.esc(modal.username) + '"></label><label>' + ctx.esc(text(ctx, 'password')) + '<input data-field="password" type="password" autocomplete="new-password" value=""></label></div></div>';
        }
        if (modal.step === 3) {
            const notices = [];
            if (modal.snapshotAvailable === true) notices.push('<div class="nc-probe-notice is-success">' + ctx.esc(text(ctx, 'snapshot_available')) + '</div>');
            const options = selected => modal.profiles.map(profile => '<option value="' + ctx.esc(profile.id) + '" ' + (profile.id === selected ? 'selected' : '') + '>' + ctx.esc(profile.name + profileSummary(profile)) + '</option>').join('');
            return '<div class="nc-form"><label>' + ctx.esc(text(ctx, 'main_stream')) + '<select data-field="profileID">' + options(modal.profileID) + '</select></label><label>' + ctx.esc(text(ctx, 'sub_stream')) + '<select data-field="subProfileID"><option value="">' + ctx.esc(text(ctx, 'no_substream')) + '</option>' + options(modal.subProfileID) + '</select></label>' + (modal.snapshotAvailable ? '<label class="nc-check"><input type="checkbox" data-field="useSnapshot" ' + (modal.useSnapshot ? 'checked' : '') + '><span>' + ctx.esc(text(ctx, 'use_camera_snapshot')) + '</span></label>' : '') + notices.join('') + '</div>';
        }
        return '<div class="nc-form"><div class="nc-form-row"><label>' + ctx.esc(text(ctx, 'camera_name')) + '<input data-field="name" value="' + ctx.esc(modal.name) + '" placeholder="' + ctx.esc(text(ctx, 'camera_name_placeholder')) + '"></label><label>' + ctx.esc(text(ctx, 'stream_id')) + '<input data-field="id" value="' + ctx.esc(modal.id) + '" placeholder="front-door"></label></div>' + (modal.method === 'url' ? '<label>' + ctx.esc(text(ctx, 'stream_url')) + '<input data-field="source" type="password" autocomplete="off" value="" placeholder="rtsp://…"></label>' : '') + '</div>';
    }

    function setupMethod(ctx, method, icon, title, help, enabled, reason) {
        return '<button type="button" data-method-choice="' + method + '" ' + (!enabled ? 'disabled' : '') + '><span>' + appIcon(ctx, icon, '•', 24) + '</span><strong>' + ctx.esc(text(ctx, title)) + '</strong><small>' + ctx.esc(reason || text(ctx, help)) + '</small></button>';
    }

    function profileSummary(profile) {
        const parts = [];
        if (profile.codec) parts.push(profile.codec);
        if (profile.width && profile.height) parts.push(profile.width + '×' + profile.height);
        if (profile.frame_rate) parts.push(profile.frame_rate + ' fps');
        return parts.length ? ' · ' + parts.join(' · ') : '';
    }

    function drawModal(state) {
        disposeDetectionSettings(state);
        const host = state.host.querySelector('[data-role="modal"]');
        const modal = state.modal;
        if (!host || !modal) { if (host) host.innerHTML = ''; return; }
        const ctx = state.ctx;
        const setup = modal.type === 'setup';
        const title = setup ? text(ctx, 'setup_title') : text(ctx, 'manage_title');
        const detectionStream = setup ? null : manageDetectionStream(state, modal);
        const content = setup ? setupStepMarkup(state, modal) : manageMarkup(state, modal, detectionStream);
        let actions = '<button type="button" data-modal-action="cancel">' + ctx.esc(text(ctx, 'cancel')) + '</button>';
        if (setup && modal.step > 1) actions += '<button type="button" data-modal-action="back">' + ctx.esc(text(ctx, 'back')) + '</button>';
        if (setup && modal.step > 1) actions += '<button class="nc-primary" type="button" data-modal-action="next" ' + (modal.busy ? 'disabled' : '') + '>' + ctx.esc(text(ctx, modal.step === 4 ? 'save_camera' : 'next')) + '</button>';
        if (!setup) actions += '<button class="nc-danger" type="button" data-modal-action="delete" ' + (modal.busy ? 'disabled' : '') + '>' + appIcon(ctx, 'trash', 'X', 15) + ctx.esc(text(ctx, 'delete')) + '</button><button class="nc-primary" type="button" data-modal-action="save" ' + (modal.busy ? 'disabled' : '') + '>' + ctx.esc(text(ctx, 'save')) + '</button>';
        host.innerHTML = '<div class="nc-modal-backdrop"><section class="nc-modal" role="dialog" aria-modal="true" aria-label="' + ctx.esc(title) + '"><header><div><strong>' + ctx.esc(title) + '</strong>' + (setup ? '<small>' + ctx.esc(text(ctx, 'step', { current: modal.step, total: 4 })) + '</small>' : '') + '</div><button type="button" data-modal-action="cancel">×</button></header><div class="nc-modal-body">' + content + (modal.error ? '<div class="nc-modal-error">' + ctx.esc(modal.error) + '</div>' : '') + '</div><footer>' + actions + '</footer></section></div>';
        bindModalEvents(state);
        const detectionRoot = detectionStream ? host.querySelector('[data-role="detection-settings"]') : null;
        if (detectionRoot) {
            state.detectionSettings = window.NetworkCamerasDetection.bindCameraSettings(detectionRoot, detectionStream, ctx, detection => {
                if (state.modal !== modal) return;
                modal.detection = detection;
                modal.detectionDirty = true;
            });
        }
    }

    function bindModalEvents(state) {
        const host = state.host.querySelector('[data-role="modal"]');
        if (!host || !state.modal) return;
        host.querySelectorAll('[data-field]').forEach(field => {
            // The pasted URL is restored through the DOM property, never written into markup.
            if (field.dataset.field === 'source' && state.modal.source) field.value = state.modal.source;
            field.addEventListener('input', () => {
                state.modal[field.dataset.field] = field.type === 'checkbox' ? field.checked : field.value;
            });
        });
        host.querySelectorAll('[data-method-choice]').forEach(button => button.addEventListener('click', () => chooseSetupMethod(state, button.dataset.methodChoice)));
        host.querySelectorAll('[data-modal-action]').forEach(button => button.addEventListener('click', () => modalAction(state, button.dataset.modalAction)));
        host.querySelectorAll('[data-modal-tab]').forEach(button => {
            button.addEventListener('click', () => selectManageTab(state, button.dataset.modalTab));
            button.addEventListener('keydown', event => {
                if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return;
                event.preventDefault();
                const next = button.dataset.modalTab === 'general' ? 'detection' : 'general';
                selectManageTab(state, next);
                const target = host.querySelector('[data-modal-tab="' + next + '"]');
                if (target) target.focus();
            });
        });
        const sourceRoot = host.querySelector('[data-source-form]');
        bindSourceForm(state, sourceRoot);
    }

    async function ensureSourceTypes(state) {
        if (!state.sourceTypes) state.sourceTypes = await window.AuraGoSourceForm.loadCatalog(() => request(state, '/api/desktop/integrations/go2rtc/source-types'));
        return state.sourceTypes;
    }

    function openTuya(state, mode) {
        if (!window.AuraGoTuya) return;
        if (state.tuyaDialog) state.tuyaDialog.close();
        state.tuyaDialog = window.AuraGoTuya.open({ mode, t: state.ctx.t,
            api: (path, init) => state.ctx.api(path, init),
            onCreated: result => { if (!state.disposed && result.stream) { state.selected = result.stream.id; savePreferences(state); notify(state, text(state.ctx, mutationNoticeKey(result, 'camera_saved'))); } },
            onClose: () => { state.tuyaDialog = null; if (!state.disposed) reloadState(state, false); }
        });
    }

    async function chooseSetupMethod(state, method) {
        const modal = state.modal;
        if (method === 'tuya') { state.modal = null; drawModal(state); return openTuya(state, 'setup'); }
        modal.method = method;
        modal.error = '';
        modal.source = '';
        if (method === 'url') { modal.step = 4; return drawModal(state); }
        if (method === 'type') {
            modal.step = 2;
            modal.sourceForm = window.AuraGoSourceForm.newState('');
            modal.busy = true;
            drawModal(state);
            try {
                await ensureSourceTypes(state);
            } catch (error) {
                modal.error = text(state.ctx, 'source_types_failed');
            } finally {
                modal.busy = false;
            }
            return drawModal(state);
        }
        modal.step = 2;
        if (method === 'discover') {
            modal.busy = true;
            drawModal(state);
            try {
                const result = await request(state, '/api/desktop/integrations/go2rtc/discovery', { method: 'POST', body: {} });
                modal.candidates = result.candidates || [];
                if (!modal.candidates.length) modal.error = text(state.ctx, 'no_devices');
            } catch (error) {
                modal.error = error.message;
            } finally {
                modal.busy = false;
            }
        }
        drawModal(state);
    }

    async function modalAction(state, action) {
        const modal = state.modal;
        if (!modal) return;
        if (action === 'cancel') { state.modal = null; return drawModal(state); }
        if (action === 'back') { if (modal.method === 'type' && modal.step === 4) modal.source = ''; modal.step = modal.method === 'type' && modal.step === 4 ? 2 : Math.max(1, modal.step - 1); return drawModal(state); }
        if (action === 'next') return advanceSetup(state);
        if (action === 'save') return saveManagedStream(state);
        if (action === 'delete') return deleteManagedStream(state);
    }

    async function advanceSetup(state) {
        const modal = state.modal;
        modal.error = '';
        if (modal.step === 2 && modal.method === 'type') {
            const built = window.AuraGoSourceForm.buildURL(state.sourceTypes || [], modal.sourceForm);
            if (built.error) { modal.error = window.AuraGoSourceForm.errorText(state.ctx.t, built); return drawModal(state); }
            modal.source = built.url;
            modal.step = 4;
            return drawModal(state);
        }
        if (modal.step === 2) {
            if (modal.method === 'discover' && !modal.selectedCandidate) { modal.error = text(state.ctx, 'choose_device_error'); return drawModal(state); }
            if (modal.method === 'address' && !modal.address.trim()) { modal.error = text(state.ctx, 'address_required'); return drawModal(state); }
            modal.busy = true;
            drawModal(state);
            try {
                const result = await request(state, '/api/desktop/integrations/go2rtc/discovery/profiles', { method: 'POST', body: { candidate_id: modal.selectedCandidate, address: modal.address, username: modal.username, password: modal.password } });
                modal.password = '';
                modal.setupToken = result.setup_token;
                modal.profiles = result.profiles || [];
                const ordered = modal.profiles.slice().sort((a, b) => (Number(b.width || 0) * Number(b.height || 0)) - (Number(a.width || 0) * Number(a.height || 0)) || String(a.id).localeCompare(String(b.id)));
                modal.profileID = ordered.length ? ordered[0].id : '';
                const small = ordered.filter(profile => profile.id !== modal.profileID && /H.?264|AVC/i.test(profile.codec || '')).pop() || ordered.filter(profile => profile.id !== modal.profileID).pop();
                modal.subProfileID = small ? small.id : '';
                modal.snapshotAvailable = typeof result.snapshot_available === 'boolean' ? result.snapshot_available : null;
                modal.name = modal.name || result.name || result.model || '';
                modal.step = 3;
            } catch (error) {
                modal.password = '';
                modal.error = error.message;
            } finally {
                modal.busy = false;
            }
            return drawModal(state);
        }
        if (modal.step === 3) {
            if (!modal.profileID) { modal.error = text(state.ctx, 'profile_required'); return drawModal(state); }
            modal.step = 4;
            if (!modal.id) modal.id = slugify(modal.name);
            return drawModal(state);
        }
        if (modal.step === 4) return createStream(state);
    }

    function slugify(value) {
        return String(value || 'camera').toLocaleLowerCase().normalize('NFKD').replace(/[\u0300-\u036f]/g, '').replace(/[^a-z0-9_-]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 48) || 'camera';
    }

    async function createStream(state) {
        const modal = state.modal;
        if (!modal.name.trim() || !modal.id.trim()) { modal.error = text(state.ctx, 'name_id_required'); return drawModal(state); }
        if ((modal.method === 'url' || modal.method === 'type') && !modal.source.trim()) { modal.error = text(state.ctx, 'source_required'); return drawModal(state); }
        modal.busy = true;
        drawModal(state);
        try {
            const result = await request(state, '/api/desktop/integrations/go2rtc/streams', { method: 'POST', body: { id: modal.id, name: modal.name, source: modal.method === 'url' || modal.method === 'type' ? modal.source : '', setup_token: modal.setupToken, profile_id: modal.profileID, sub_profile_id: modal.subProfileID, use_camera_snapshot: modal.useSnapshot } });
            modal.source = '';
            state.selected = result.stream && result.stream.id ? result.stream.id : modal.id;
            state.modal = null;
            drawModal(state);
            savePreferences(state);
            notify(state, text(state.ctx, mutationNoticeKey(result, 'camera_saved')));
            await reloadState(state, false);
        } catch (error) {
            modal.error = error.message;
            modal.busy = false;
            drawModal(state);
        }
    }

    async function saveManagedStream(state) {
        const modal = state.modal;
        const detection = detectionModule();
        // A zone still being drawn is closed first (Save must not drop it); one with fewer
        // than 3 points blocks the save and the editor says why.
        if (state.detectionSettings && typeof state.detectionSettings.commitDrawing === 'function' && !state.detectionSettings.commitDrawing()) {
            selectManageTab(state, 'detection');
            const stage = state.host.querySelector('[data-role="modal"] [data-zone-stage]');
            if (stage) stage.focus();
            return;
        }
        if (modal.detectionDirty && detection && typeof detection.validateCameraSettings === 'function') {
            const problem = detection.validateCameraSettings(modal.detection);
            if (problem) {
                modal.error = text(state.ctx, 'detection.' + problem.key, problem.params);
                modal.tab = 'detection';
                return drawModal(state);
            }
        }
        modal.source = '';
        if (modal.sourceForm && modal.sourceForm.type) {
            const built = window.AuraGoSourceForm.buildURL(state.sourceTypes || [], modal.sourceForm);
            if (built.error) { modal.error = window.AuraGoSourceForm.errorText(state.ctx.t, built); modal.tab = 'general'; return drawModal(state); }
            modal.source = built.url;
        }
        const confirmations = [];
        if (modal.originalEnabled && !modal.enabled) confirmations.push(text(state.ctx, 'disable_confirm_message'));
        if (modal.source.trim() || modal.sub_source.trim() || modal.snapshot_source.trim() || modal.clear_sub_source || modal.clear_snapshot_source) confirmations.push(text(state.ctx, 'replace_confirm_message'));
        if (confirmations.length) {
            const confirmed = await state.ctx.confirmDialog(text(state.ctx, 'change_confirm_title'), confirmations.join(' '));
            if (!confirmed) return;
        }
        modal.busy = true;
        drawModal(state);
        try {
            const result = await request(state, '/api/desktop/integrations/go2rtc/streams/' + encodeURIComponent(modal.streamID), { method: 'PATCH', body: Object.assign({ name: modal.name, enabled: modal.enabled, source: modal.source }, window.NetworkCamerasMedia.patch(modal), modal.detectionDirty && modal.detection ? { detection: modal.detection } : {}) });
            modal.source = '';
            state.modal = null;
            drawModal(state);
            notify(state, text(state.ctx, mutationNoticeKey(result, 'camera_updated')));
            await reloadState(state, false);
        } catch (error) {
            // A 400 on the detection block names its field: show the localized text on its tab.
            const field = error && error.body && typeof error.body.field === 'string' ? error.body.field : '';
            const views = window.NetworkCamerasDetectionViews;
            const key = field.startsWith('detection.') && views && typeof views.fieldErrorKey === 'function' ? views.fieldErrorKey(field) : '';
            modal.error = key ? text(state.ctx, 'detection.' + key) : error.message;
            if (field.startsWith('detection.') && detectionTabAvailable(state)) modal.tab = 'detection';
            modal.busy = false;
            drawModal(state);
        }
    }

    async function deleteManagedStream(state) {
        const modal = state.modal;
        if (!modal) return;
        return deleteStream(state, modal.streamID, modal);
    }

    async function deleteStream(state, streamID, modal) {
        if (!streamID || state.deletingID) return;
        const stream = allStreams(state).find(item => item.id === streamID);
        const message = text(state.ctx, 'delete_confirm_message') + (stream ? ' ' + (stream.name || stream.id) : '');
        const confirmed = await state.ctx.confirmDialog(text(state.ctx, 'delete_confirm_title'), message);
        if (!confirmed) return;
        state.deletingID = streamID;
        if (modal) {
            modal.busy = true;
            drawModal(state);
        } else {
            draw(state);
        }
        try {
            const result = await request(state, '/api/desktop/integrations/go2rtc/streams/' + encodeURIComponent(streamID), { method: 'DELETE' });
            state.modal = null;
            drawModal(state);
            if (state.selected === streamID) state.selected = '';
            savePreferences(state);
            notify(state, text(state.ctx, mutationNoticeKey(result, 'camera_deleted')));
            state.deletingID = '';
            await reloadState(state, false);
        } catch (error) {
            if (modal && state.modal === modal) {
                modal.error = error.message;
                modal.busy = false;
                drawModal(state);
            } else {
                state.deletingID = '';
                notify(state, error.message, 'error');
                draw(state);
            }
        } finally {
            state.deletingID = '';
        }
    }

    function setWindowMenus(state) {
        if (typeof state.ctx.setWindowMenus !== 'function') return;
        state.ctx.setWindowMenus(state.windowId, [{ id: 'view', labelKey: 'desktop.menu_view', items: [
            { id: 'refresh', labelKey: 'desktop.network_cameras.refresh', icon: 'refresh', action: () => loadState(state, true) },
            { id: 'focus', labelKey: 'desktop.network_cameras.focus', icon: 'maximize', disabled: !selectedStream(state), action: () => handleAction(state, 'focus') }
        ] }]);
    }

    function stopActivity(state) {
        window.NetworkCamerasMedia.stop(state);
        clearTimeout(state.statusTimer);
        state.statusTimer = null;
        if (state.thumbnailTimer) clearTimeout(state.thumbnailTimer);
        state.thumbnailTimer = null;
        state.controllers.forEach(controller => controller.abort());
        state.controllers.clear();
        state.host.querySelectorAll('iframe').forEach(frame => { frame.src = 'about:blank'; frame.removeAttribute('src'); });
        const detection = detectionModule();
        if (detection) detection.unmount(state.windowId, { keepState: true });
    }

    function installVisibilityLifecycle(state) {
        const windowElement = state.host.closest('.vd-window');
        const update = () => {
            const visible = document.visibilityState !== 'hidden' && (!windowElement || windowElement.style.display !== 'none');
            if (visible === state.visible) return;
            state.visible = visible;
            if (visible) { draw(state); loadState(state, false); }
            else stopActivity(state);
        };
        if (windowElement) {
            state.visibilityObserver = new MutationObserver(update);
            state.visibilityObserver.observe(windowElement, { attributes: true, attributeFilter: ['style', 'class'] });
        }
        state.documentVisibilityHandler = update;
        document.addEventListener('visibilitychange', update);
    }

    function dispose(windowId) {
        const state = instances.get(windowId);
        if (!state) return;
        state.disposed = true;
        if (state.tuyaDialog) state.tuyaDialog.close();
        stopActivity(state);
        window.NetworkCamerasMedia.stop(state, true);
        disposeDetectionSettings(state);
        const detection = detectionModule();
        if (detection) detection.unmount(windowId);
        disconnectIntersection(state);
        if (state.visibilityObserver) state.visibilityObserver.disconnect();
        if (state.documentVisibilityHandler) document.removeEventListener('visibilitychange', state.documentVisibilityHandler);
        state.objectURLs.forEach(value => URL.revokeObjectURL(value));
        state.objectURLs.clear();
        if (typeof state.ctx.clearWindowMenus === 'function') state.ctx.clearWindowMenus(windowId);
        instances.delete(windowId);
    }

    // open applies a launch context to an already open window (window-shell-runtime.js openApp).
    function open(windowId, context) {
        const state = instances.get(windowId);
        const launch = launchContext(context);
        if (!state || state.disposed || !launch) return;
        state.pendingLaunch = launch;
        state.mode = launch.event_id ? 'detections' : 'snapshots';
        state.focus = false;
        if (state.data && !launch.event_id) ensureSelection(state);
        savePreferences(state);
        draw(state);
    }

    function render(host, windowId, ctx) {
        dispose(windowId);
        const state = createState(host, windowId, ctx);
        instances.set(windowId, state);
        window.AuraDesktopEmail?.attach(windowId, '.nc-toolbar-actions', () => {
            const stream=selectedStream(state);
            return {key:stream?.id,disabled:state.disposed||!stream||state.loading,subject:stream?.name,formats:['jpg'],
                createFiles:async(_format,signal)=>{
                    const response=await fetch('/api/desktop/integrations/go2rtc/snapshot',{method:'POST',credentials:'same-origin',signal,
                        headers:{'Content-Type':'application/json'},body:JSON.stringify({stream_id:stream.id,store:true})});
                    if(!response.ok)throw new Error(text(state.ctx,'snapshot_failed'));
                    const result=await response.json();
                    if(!result.web_path)throw new Error(text(state.ctx,'snapshot_failed'));
                    return window.AuraDesktopEmail.files([{url:result.web_path,name:result.web_path.split('/').pop()}],signal);
                }};
        });
        if (state.pendingLaunch) savePreferences(state);
        draw(state);
        installVisibilityLifecycle(state);
        loadState(state, false);
    }

    window.NetworkCamerasApp = { render, dispose, open };
})();
