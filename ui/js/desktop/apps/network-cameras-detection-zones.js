// Network Cameras – person detection: the zone editor and the other controls
// of the camera settings tab (bindCameraSettings). The markup comes from
// network-cameras-detection-views.js.
(function () {
    'use strict';

    const {
        API,
        MAX_ZONES,
        MIN_POINTS,
        MAX_POINTS,
        MAX_INSTRUCTION,
        CHANNELS,
        JPEG_DATA_URL,
        T,
        finite,
        clamp,
        round4,
        runeLength,
        percent,
        cameraDefaults,
        cameraPayload,
        zoneEditorSvg,
        nextZoneName,
        normalizedPoint,
        isFlatZone,
        isNearPoint,
        zoneListMarkup
    } = window.NetworkCamerasDetectionViews;

    function isAbort(error) {
        return !!error && error.name === 'AbortError';
    }

    function bindCameraSettings(root, stream, ctx, onChange) {
        const container = root && root.querySelector ? (root.querySelector('[data-ncd-camera-settings]') || root) : null;
        if (!container || !ctx) return { value: () => null, commitDrawing: () => true, dispose() {} };
        const ed = {
            draft: cameraDefaults(stream && stream.detection),
            drawing: null, selected: -1, drag: null, boxes: [], width: 1280, height: 720,
            testing: false, controller: null, disposed: false, listeners: [], lastDownAt: 0
        };
        const streamID = String(stream && stream.id || '');
        const stage = container.querySelector('[data-zone-stage]');
        const svg = container.querySelector('[data-zone-svg]');
        const background = container.querySelector('[data-zone-bg]');
        const list = container.querySelector('[data-zone-list]');
        const statusLine = container.querySelector('[data-zone-status]');
        const listen = (target, type, handler) => {
            if (!target) return;
            target.addEventListener(type, handler);
            ed.listeners.push(() => target.removeEventListener(type, handler));
        };
        const emit = () => {
            if (!ed.disposed && typeof onChange === 'function') onChange(cameraPayload(ed.draft));
        };
        const setStatus = (message, isError) => {
            if (!statusLine) return;
            statusLine.textContent = message || '';
            statusLine.classList.toggle('is-error', !!isError);
        };
        const redraw = () => {
            if (!svg) return;
            svg.setAttribute('viewBox', '0 0 ' + ed.width + ' ' + ed.height);
            svg.innerHTML = zoneEditorSvg({ zones: ed.draft.zones, selected: ed.selected, drawing: ed.drawing, width: ed.width, height: ed.height, boxes: ed.boxes });
        };
        const renderList = () => {
            if (list) list.innerHTML = zoneListMarkup(ctx, ed.draft.zones, ed.selected);
        };
        const updateTools = () => {
            const close = container.querySelector('[data-zone-action="close"]');
            const cancel = container.querySelector('[data-zone-action="cancel"]');
            const test = container.querySelector('[data-zone-action="test"]');
            if (close) close.disabled = !(ed.drawing && ed.drawing.length >= MIN_POINTS);
            if (cancel) cancel.disabled = !ed.drawing;
            if (test) test.disabled = ed.testing || !streamID;
        };
        const syncDependentFields = () => {
            const channel = container.querySelector('[data-cam-field="actions.channel"]');
            const instruction = container.querySelector('[data-cam-field="actions.agent_instruction"]');
            const score = container.querySelector('[data-cam-out="min_score"]');
            const count = container.querySelector('[data-cam-out="instruction_count"]');
            if (channel) channel.disabled = !ed.draft.actions.message;
            if (instruction) instruction.disabled = !ed.draft.actions.agent;
            if (score) score.textContent = percent(ed.draft.min_score) + ' %';
            if (count) count.textContent = runeLength(ed.draft.actions.agent_instruction) + '/' + MAX_INSTRUCTION;
        };
        const applyField = (target, committed) => {
            const field = target.dataset.camField;
            const draft = ed.draft;
            if (field === 'enabled') draft.enabled = target.checked;
            else if (field === 'interval_seconds' || field === 'cooldown_seconds') {
                const limits = field === 'interval_seconds' ? [1, 30, 2] : [0, 86400, 300];
                const raw = Math.round(Number(target.value));
                const value = target.value !== '' && Number.isFinite(raw) ? clamp(raw, limits[0], limits[1]) : limits[2];
                draft[field] = value;
                if (committed && String(value) !== target.value) target.value = String(value);
            } else if (field === 'min_score') draft.min_score = round4(clamp(finite(target.value, 0.5), 0.3, 0.95));
            else if (field === 'actions.channel') draft.actions.channel = CHANNELS.includes(target.value) ? target.value : 'all';
            else if (field === 'actions.agent_instruction') draft.actions.agent_instruction = Array.from(target.value).slice(0, MAX_INSTRUCTION).join('');
            else if (field === 'actions.message' || field === 'actions.agent' || field === 'actions.desktop') draft.actions[field.slice(8)] = target.checked;
            else return;
            syncDependentFields();
            emit();
        };
        const pointFromEvent = event => normalizedPoint(event.clientX, event.clientY, svg.getBoundingClientRect());
        // A zone whose points lie on one line can never contain a foot point.
        const warnIfFlat = zone => {
            if (isFlatZone(zone.points)) setStatus(T(ctx, 'zone_flat', { name: zone.name }), true);
        };
        const closeZone = () => {
            if (!ed.drawing || ed.drawing.length < MIN_POINTS) return;
            const name = nextZoneName(ctx, ed.draft.zones);
            const zone = { name, points: ed.drawing.slice(0, MAX_POINTS) };
            ed.draft.zones.push(zone);
            ed.drawing = null;
            ed.selected = ed.draft.zones.length - 1;
            redraw();
            renderList();
            updateTools();
            setStatus(T(ctx, 'zone_added', { name }));
            warnIfFlat(zone);
            emit();
        };
        // commitDrawing runs before Save: it closes a zone that is still being drawn, so the
        // PATCH never drops it, and returns false (with a hint) when it has fewer than 3 points.
        const commitDrawing = () => {
            if (ed.disposed || !ed.drawing) return true;
            if (ed.drawing.length >= MIN_POINTS) {
                closeZone();
                return true;
            }
            setStatus(T(ctx, 'zone_unfinished'), true);
            return false;
        };
        const cancelZone = () => {
            if (!ed.drawing) return;
            ed.drawing = null;
            redraw();
            updateTools();
            setStatus('');
        };
        const onBackgroundLoad = () => {
            if (ed.disposed || !background) return;
            ed.width = Math.max(1, background.naturalWidth || 1280);
            ed.height = Math.max(1, background.naturalHeight || 720);
            if (stage) {
                stage.style.aspectRatio = ed.width + ' / ' + ed.height;
                stage.classList.remove('is-empty');
            }
            redraw();
        };
        const runTest = async () => {
            if (ed.testing || !streamID) return;
            ed.testing = true;
            updateTools();
            setStatus(T(ctx, 'testing'));
            const controller = new AbortController();
            ed.controller = controller;
            try {
                const result = await ctx.api(API + 'test', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ stream_id: streamID, detection: cameraPayload(ed.draft) }), signal: controller.signal });
                if (ed.disposed) return;
                const image = String(result && result.image || '');
                ed.boxes = Array.isArray(result && result.boxes) ? result.boxes : [];
                if (background && JPEG_DATA_URL.test(image)) background.src = image;
                redraw();
                const counted = ed.boxes.filter(box => box && box.counts === true).length;
                setStatus(T(ctx, 'test_result', { counted, ignored: ed.boxes.length - counted, backend: String(result && result.backend || '').toUpperCase(), ms: Math.round(finite(result && result.wall_ms, 0)) }));
            } catch (error) {
                if (ed.disposed || isAbort(error)) return;
                const code = error && error.body && error.body.code;
                const message = code === 'detection_disabled' ? T(ctx, 'test_disabled') : code === 'detector_not_ready' ? T(ctx, 'test_not_ready') : code === 'frame_unavailable' ? T(ctx, 'test_no_frame') : ((error && error.message) || T(ctx, 'test_failed'));
                setStatus(message, true);
            } finally {
                ed.testing = false;
                ed.controller = null;
                if (!ed.disposed) updateTools();
            }
        };
        listen(container, 'input', event => {
            if (event.target && event.target.dataset && event.target.dataset.camField) applyField(event.target, false);
        });
        listen(container, 'change', event => {
            if (event.target && event.target.dataset && event.target.dataset.camField) applyField(event.target, true);
        });
        listen(stage, 'pointerdown', event => {
            if (!svg || (event.pointerType === 'mouse' && event.button !== 0)) return;
            const handle = event.target && event.target.closest ? event.target.closest('[data-point], [data-drawing-point]') : null;
            if (handle && handle.hasAttribute('data-point') && !ed.drawing) {
                ed.drag = { zone: Number(handle.getAttribute('data-zone')), point: Number(handle.getAttribute('data-point')), moved: false };
                try { stage.setPointerCapture(event.pointerId); } catch (_) {}
                event.preventDefault();
                return;
            }
            event.preventDefault();
            stage.focus({ preventScroll: true });
            if (handle && handle.hasAttribute('data-drawing-point') && Number(handle.getAttribute('data-drawing-point')) === 0 && ed.drawing && ed.drawing.length >= MIN_POINTS) {
                closeZone();
                return;
            }
            const now = Date.now();
            const quick = now - ed.lastDownAt < 400;
            ed.lastDownAt = now;
            const point = pointFromEvent(event);
            if (!ed.drawing) {
                if (ed.draft.zones.length >= MAX_ZONES) {
                    setStatus(T(ctx, 'zone_limit'), true);
                    return;
                }
                ed.drawing = [];
                ed.selected = -1;
                renderList();
            }
            if (isNearPoint(ed.drawing[ed.drawing.length - 1], point)) {
                if (quick && ed.drawing.length >= MIN_POINTS) closeZone();
                return;
            }
            if (ed.drawing.length >= MAX_POINTS) {
                setStatus(T(ctx, 'zone_point_limit'), true);
                return;
            }
            ed.drawing.push(point);
            redraw();
            updateTools();
            setStatus(T(ctx, 'zone_points', { count: ed.drawing.length }));
        });
        listen(stage, 'pointermove', event => {
            if (!ed.drag) return;
            const zone = ed.draft.zones[ed.drag.zone];
            if (!zone || !zone.points[ed.drag.point]) return;
            zone.points[ed.drag.point] = pointFromEvent(event);
            ed.drag.moved = true;
            redraw();
        });
        const endDrag = event => {
            if (!ed.drag) return;
            const { moved, zone } = ed.drag;
            ed.drag = null;
            try { stage.releasePointerCapture(event.pointerId); } catch (_) {}
            if (!moved) return;
            const dragged = ed.draft.zones[zone];
            if (dragged) warnIfFlat(dragged);
            emit();
        };
        listen(stage, 'pointerup', endDrag);
        listen(stage, 'pointercancel', endDrag);
        listen(stage, 'dblclick', event => {
            event.preventDefault();
            closeZone();
        });
        listen(stage, 'keydown', event => {
            if (event.key === 'Enter' && ed.drawing) {
                event.preventDefault();
                closeZone();
            } else if (event.key === 'Escape' && ed.drawing) {
                event.preventDefault();
                event.stopPropagation();
                cancelZone();
            } else if ((event.key === 'Backspace' || event.key === 'Delete') && ed.drawing && ed.drawing.length) {
                event.preventDefault();
                ed.drawing.pop();
                if (!ed.drawing.length) ed.drawing = null;
                redraw();
                updateTools();
            }
        });
        listen(list, 'input', event => {
            const zone = ed.draft.zones[Number(event.target && event.target.dataset ? event.target.dataset.zoneName : NaN)];
            if (!zone) return;
            zone.name = event.target.value;
            emit();
        });
        listen(list, 'click', event => {
            const button = event.target && event.target.closest ? event.target.closest('button') : null;
            if (!button) return;
            if (button.dataset.zoneSelect !== undefined) {
                const index = Number(button.dataset.zoneSelect);
                if (ed.drawing || !ed.draft.zones[index]) return;
                ed.selected = ed.selected === index ? -1 : index;
                redraw();
                renderList();
                const again = list.querySelector('[data-zone-select="' + index + '"]');
                if (again) again.focus();
            } else if (button.dataset.zoneDelete !== undefined) {
                const index = Number(button.dataset.zoneDelete);
                const zone = ed.draft.zones[index];
                if (!zone) return;
                ed.draft.zones.splice(index, 1);
                if (ed.selected === index) ed.selected = -1;
                else if (ed.selected > index) ed.selected -= 1;
                redraw();
                renderList();
                setStatus(T(ctx, 'zone_deleted', { name: zone.name }));
                const next = list.querySelector('[data-zone-delete="' + Math.min(index, ed.draft.zones.length - 1) + '"]');
                if (next) next.focus();
                else if (stage) stage.focus();
                emit();
            }
        });
        listen(container.querySelector('[data-zone-action="close"]'), 'click', closeZone);
        listen(container.querySelector('[data-zone-action="cancel"]'), 'click', cancelZone);
        listen(container.querySelector('[data-zone-action="test"]'), 'click', () => { runTest(); });
        listen(background, 'load', onBackgroundLoad);
        listen(background, 'error', () => {
            if (ed.disposed || !stage) return;
            stage.classList.add('is-empty');
            setStatus(T(ctx, 'zone_frame_missing'));
        });
        redraw();
        updateTools();
        syncDependentFields();
        if (background && background.complete && background.naturalWidth) onBackgroundLoad();
        return {
            value: () => cameraPayload(ed.draft),
            commitDrawing,
            dispose() {
                ed.disposed = true;
                if (ed.controller) ed.controller.abort();
                ed.listeners.forEach(remove => remove());
                ed.listeners = [];
            }
        };
    }

    window.NetworkCamerasDetectionZones = Object.freeze({ bindCameraSettings });
})();
