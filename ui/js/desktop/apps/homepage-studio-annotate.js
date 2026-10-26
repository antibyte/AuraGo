/* Homepage Studio annotate mode: captures a snapshot of the previewed page,
   lets the user mark elements (click) or areas (drag) with a comment, shows
   the marks as chips above the composer and hands them to the next chat turn
   as a composed image plus element details. */
(function () {
    'use strict';

    const MAX_MARKS = 20;
    const DRAG_THRESHOLD = 6;
    const CROP_MARGIN = 48;
    const IMAGE_MAX_WIDTH = 1000;
    const IMAGE_MAX_HEIGHT = 1600;
    const IMAGE_GAP = 16;
    /* The chat body limit is 2 MiB; the image stays well below it. */
    const IMAGE_MAX_BASE64 = 1400000;
    const AREA_OVERLAP_LIMIT = 5;
    const MARK_COLOR = '#e8590c';
    const DEVICE_WIDTHS = { desktop: 1280, tablet: 768, mobile: 390 };
    const SNAPSHOT_ENDPOINT = '/api/desktop/integrations/homepage/annotation-snapshot';
    const ERROR_CODES = ['capture_busy', 'browser_unavailable', 'capture_timeout', 'preview_unavailable', 'invalid_path', 'invalid_viewport', 'invalid_target', 'capture_blocked', 'capture_failed'];

    function boxArea(box) {
        return Math.max(0, box[2]) * Math.max(0, box[3]);
    }

    function boxContains(box, x, y) {
        return x >= box[0] && y >= box[1] && x <= box[0] + box[2] && y <= box[1] + box[3];
    }

    /* Smallest element box under the point; on equal size the later (deeper)
       element wins. Returns the element index or -1. */
    function hitTest(elements, x, y) {
        let best = -1;
        let bestArea = Infinity;
        (elements || []).forEach((item, index) => {
            if (!item || !Array.isArray(item.box) || !boxContains(item.box, x, y)) return;
            const area = boxArea(item.box);
            if (area <= bestArea) {
                best = index;
                bestArea = area;
            }
        });
        return best;
    }

    function normalizeBox(x0, y0, x1, y1) {
        return [Math.round(Math.min(x0, x1)), Math.round(Math.min(y0, y1)), Math.round(Math.abs(x1 - x0)), Math.round(Math.abs(y1 - y0))];
    }

    function intersectionArea(a, b) {
        const w = Math.min(a[0] + a[2], b[0] + b[2]) - Math.max(a[0], b[0]);
        const h = Math.min(a[1] + a[3], b[1] + b[3]) - Math.max(a[1], b[1]);
        return w > 0 && h > 0 ? w * h : 0;
    }

    /* Elements that lie at least half inside the area, largest first. */
    function overlapping(elements, box, limit) {
        return (elements || [])
            .map((item, index) => ({ item, index }))
            .filter(entry => entry.item && Array.isArray(entry.item.box) && boxArea(entry.item.box) > 0
                && intersectionArea(entry.item.box, box) >= boxArea(entry.item.box) / 2)
            .sort((a, b) => boxArea(b.item.box) - boxArea(a.item.box))
            .slice(0, limit || AREA_OVERLAP_LIMIT)
            .map(entry => entry.index);
    }

    /* Significant elements in reading order, for keyboard stepping. */
    function tabOrder(elements) {
        return (elements || [])
            .map((item, index) => ({ item, index }))
            .filter(entry => entry.item && entry.item.tab && Array.isArray(entry.item.box))
            .sort((a, b) => (a.item.box[1] - b.item.box[1]) || (a.item.box[0] - b.item.box[0]))
            .map(entry => entry.index);
    }

    /* The persisted user message carries only the user's own comments. */
    function composeMessage(text, marks, labels) {
        const base = String(text || '').trim() || labels.defaultMessage;
        if (!marks || !marks.length) return base;
        const lines = marks.map(mark => mark.n + '. ' + String(mark.comment || '').replace(/\s+/g, ' ').trim());
        return base + '\n\n' + labels.heading + '\n' + lines.join('\n');
    }

    function annotationPayload(snapshot, marks) {
        const elements = (snapshot && snapshot.elements) || [];
        const brief = index => {
            const item = elements[index];
            return item ? { selector: item.selector, tag: item.tag, text: item.text || '' } : null;
        };
        return {
            page_path: snapshot ? snapshot.path : '/',
            viewport_width: snapshot && snapshot.viewport ? snapshot.viewport.width : 0,
            items: (marks || []).map(mark => {
                const item = { n: mark.n, kind: mark.kind, comment: mark.comment, box: mark.box.slice() };
                if (mark.kind === 'element') {
                    const target = elements[mark.elementIndex] || {};
                    item.selector = target.selector || '';
                    item.tag = target.tag || '';
                    item.text = target.text || '';
                    if (target.attrs && Object.keys(target.attrs).length) item.attrs = Object.assign({}, target.attrs);
                } else {
                    item.elements = (mark.overlapIndexes || []).map(brief).filter(Boolean);
                }
                return item;
            })
        };
    }

    /* Crops for the composed image: every mark with a margin, stacked
       vertically and scaled into IMAGE_MAX_WIDTH × IMAGE_MAX_HEIGHT. */
    function cropLayout(marks, page) {
        const pageW = Math.max(1, page.width);
        const pageH = Math.max(1, page.height);
        const crops = (marks || []).map(mark => {
            const x = Math.max(0, mark.box[0] - CROP_MARGIN);
            const y = Math.max(0, mark.box[1] - CROP_MARGIN);
            const right = Math.min(pageW, mark.box[0] + mark.box[2] + CROP_MARGIN);
            const bottom = Math.min(pageH, mark.box[1] + mark.box[3] + CROP_MARGIN);
            const src = [x, y, Math.max(1, right - x), Math.max(1, bottom - y)];
            return { n: mark.n, box: mark.box, src, scale: Math.min(1, IMAGE_MAX_WIDTH / src[2]) };
        });
        const gaps = Math.max(0, crops.length - 1) * IMAGE_GAP;
        /* One pixel per crop of slack absorbs the rounding below. */
        const budget = IMAGE_MAX_HEIGHT - gaps - crops.length;
        const natural = crops.reduce((sum, crop) => sum + crop.src[3] * crop.scale, 0);
        const fit = natural > budget ? Math.max(0.05, budget / natural) : 1;
        let y = 0;
        let width = 1;
        crops.forEach(crop => {
            crop.scale *= fit;
            crop.dst = [0, Math.round(y), Math.max(1, Math.round(crop.src[2] * crop.scale)), Math.max(1, Math.round(crop.src[3] * crop.scale))];
            width = Math.max(width, crop.dst[2]);
            y = crop.dst[1] + crop.dst[3] + IMAGE_GAP;
        });
        return { crops, width, height: Math.max(1, Math.round(y - (crops.length ? IMAGE_GAP : 0))) };
    }

    function create(deps) {
        const root = deps.root;
        const stage = deps.stage;
        const t = deps.t;
        const panel = root.querySelector('.vd-hp-preview-panel');
        const frame = root.querySelector('.vd-hp-preview-frame');
        const toolbar = root.querySelector('.vd-hp-viewport-toolbar');
        const chatForm = deps.chatForm;
        const uid = String(deps.windowId || 'hp');
        const listeners = [];
        const state = {
            mode: 'idle',
            available: false,
            snapshot: null,
            snapshotContext: null,
            marks: [],
            abortCtrl: null,
            drag: null,
            popover: null,
            focusList: [],
            focusPos: -1
        };

        function listen(target, event, handler, options) {
            if (!target) return;
            target.addEventListener(event, handler, options);
            listeners.push(() => target.removeEventListener(event, handler, options));
        }

        function node(tag, className, text) {
            const element = document.createElement(tag);
            if (className) element.className = className;
            if (text) element.textContent = text;
            return element;
        }

        function actionButton(className, label) {
            const button = node('button', className, label);
            button.type = 'button';
            return button;
        }

        function notice(message) {
            if (typeof deps.notify === 'function') deps.notify({ title: t('homepage_studio.welcome_heading'), message });
        }

        /* Toolbar button */
        const button = actionButton('vd-hp-icon-btn vd-hp-annotate-btn');
        button.setAttribute('data-hp-annotate', '');
        button.setAttribute('aria-pressed', 'false');
        button.setAttribute('aria-label', t('homepage_studio.annotate_button'));
        button.innerHTML = deps.iconMarkup('edit', '✎', 'vd-hp-btn-icon', 14);
        button.hidden = !!deps.readonly;
        if (toolbar) toolbar.insertBefore(button, toolbar.querySelector('[id^="hp-refresh-"]'));

        /* Snapshot layer, scaled together with the device frame */
        const layer = node('div', 'vd-hp-annotate-layer');
        layer.tabIndex = 0;
        layer.hidden = true;
        layer.setAttribute('role', 'application');
        layer.setAttribute('aria-label', t('homepage_studio.annotate_layer_label'));
        const canvas = node('div', 'vd-hp-annotate-canvas');
        const shot = node('img', 'vd-hp-annotate-shot');
        shot.alt = '';
        shot.draggable = false;
        const markerLayer = node('div', 'vd-hp-annotate-markers');
        const hoverBox = node('div', 'vd-hp-annotate-hover');
        hoverBox.hidden = true;
        const dragBox = node('div', 'vd-hp-annotate-drag');
        dragBox.hidden = true;
        canvas.append(shot, markerLayer, hoverBox, dragBox);
        layer.appendChild(canvas);
        if (frame) frame.appendChild(layer);

        /* Unscaled chrome on the preview panel */
        const hoverLabel = node('div', 'vd-hp-annotate-label');
        hoverLabel.hidden = true;
        hoverLabel.setAttribute('aria-hidden', 'true');
        const bar = node('div', 'vd-hp-annotate-bar');
        bar.hidden = true;
        bar.setAttribute('role', 'toolbar');
        bar.setAttribute('aria-label', t('homepage_studio.annotate_button'));
        const pathLabel = node('label', 'vd-hp-annotate-path');
        const pathInput = node('input');
        pathInput.type = 'text';
        pathInput.value = '/';
        pathInput.spellcheck = false;
        pathInput.autocomplete = 'off';
        pathLabel.append(node('span', '', t('homepage_studio.annotate_path_label')), pathInput);
        const statusText = node('span', 'vd-hp-annotate-status');
        const recaptureBtn = actionButton('vd-hp-annotate-action', t('homepage_studio.annotate_recapture'));
        const doneBtn = actionButton('vd-hp-annotate-action is-primary', t('homepage_studio.annotate_done'));
        bar.append(pathLabel, statusText, recaptureBtn, doneBtn);

        const popover = node('div', 'vd-hp-mark-popover');
        popover.hidden = true;
        popover.setAttribute('role', 'dialog');
        const popTitle = node('div', 'vd-hp-mark-popover-title');
        popTitle.id = 'hp-mark-title-' + uid;
        popover.setAttribute('aria-labelledby', popTitle.id);
        const popInput = node('textarea', 'vd-hp-mark-popover-input');
        popInput.rows = 3;
        popInput.maxLength = 1000;
        popInput.placeholder = t('homepage_studio.annotate_comment_placeholder');
        popInput.setAttribute('aria-label', t('homepage_studio.annotate_comment_placeholder'));
        const popActions = node('div', 'vd-hp-mark-popover-actions');
        const popDelete = actionButton('vd-hp-annotate-action is-danger', t('homepage_studio.annotate_delete'));
        const popCancel = actionButton('vd-hp-annotate-action', t('homepage_studio.annotate_cancel'));
        const popSave = actionButton('vd-hp-annotate-action is-primary', t('homepage_studio.annotate_save'));
        popActions.append(popDelete, popCancel, popSave);
        popover.append(popTitle, popInput, popActions);
        if (panel) panel.append(hoverLabel, bar, popover);

        /* Chips above the composer */
        const tray = node('div', 'vd-hp-mark-tray');
        tray.hidden = true;
        tray.setAttribute('role', 'group');
        tray.setAttribute('aria-label', t('homepage_studio.annotate_marks_label'));
        if (chatForm && chatForm.parentNode) chatForm.parentNode.insertBefore(tray, chatForm);

        function context() {
            const ctx = typeof deps.context === 'function' ? deps.context() || {} : {};
            return {
                siteId: Number(ctx.siteId) || 0,
                target: ctx.target || 'local',
                device: ctx.device || 'desktop',
                previewUrl: String(ctx.previewUrl || '')
            };
        }

        /* The local preview does not depend on the selected site; a deployed
           target is identified by its site and the URL the preview shows. */
        function sameSource(a, b) {
            return !!a && !!b && a.target === b.target
                && (a.target === 'local' || (a.siteId === b.siteId && a.previewUrl === b.previewUrl));
        }

        function frameScale() {
            const rect = layer.getBoundingClientRect();
            return layer.offsetWidth ? rect.width / layer.offsetWidth : 1;
        }

        function pagePoint(event) {
            const rect = layer.getBoundingClientRect();
            const scale = frameScale() || 1;
            return {
                x: (event.clientX - rect.left) / scale + layer.scrollLeft,
                y: (event.clientY - rect.top) / scale + layer.scrollTop
            };
        }

        /* Page box → position on the unscaled preview panel. */
        function panelRect(box) {
            const layerRect = layer.getBoundingClientRect();
            const base = panel.getBoundingClientRect();
            const scale = frameScale() || 1;
            return {
                left: layerRect.left - base.left + (box[0] - layer.scrollLeft) * scale,
                top: layerRect.top - base.top + (box[1] - layer.scrollTop) * scale,
                width: box[2] * scale,
                height: box[3] * scale
            };
        }

        function placeBox(element, box) {
            element.style.left = box[0] + 'px';
            element.style.top = box[1] + 'px';
            element.style.width = box[2] + 'px';
            element.style.height = box[3] + 'px';
        }

        function describe(item) {
            const selector = item.selector.length > 60 ? '…' + item.selector.slice(-59) : item.selector;
            const text = item.text ? ' · ' + (item.text.length > 40 ? item.text.slice(0, 39) + '…' : item.text) : '';
            return selector + text;
        }

        function showHover(index) {
            const item = index >= 0 && state.snapshot ? state.snapshot.elements[index] : null;
            if (!item) {
                hoverBox.hidden = true;
                hoverLabel.hidden = true;
                return;
            }
            placeBox(hoverBox, item.box);
            hoverBox.hidden = false;
            hoverLabel.textContent = describe(item);
            const rect = panelRect(item.box);
            hoverLabel.style.left = Math.max(4, rect.left) + 'px';
            hoverLabel.style.top = Math.max(4, rect.top - 28) + 'px';
            hoverLabel.hidden = false;
        }

        function setStatus(text) {
            statusText.textContent = text || '';
        }

        function setPressed(on) {
            button.setAttribute('aria-pressed', on ? 'true' : 'false');
            button.classList.toggle('active', on);
        }

        function viewport(ctx) {
            const width = DEVICE_WIDTHS[ctx.device] || DEVICE_WIDTHS.desktop;
            const rect = panel ? panel.getBoundingClientRect() : { width: 0, height: 0 };
            const scale = rect.width ? Math.min(1, rect.width / width) : 1;
            const height = rect.height ? Math.round(rect.height / scale) : 800;
            return { width, height: Math.max(400, Math.min(1600, height)) };
        }

        function failCapture(code) {
            state.mode = 'error';
            const message = t('homepage_studio.annotate_error_' + code);
            setStatus(message);
            deps.announce(message);
            if (code === 'invalid_path') pathInput.focus();
        }

        async function capture() {
            if (state.abortCtrl) state.abortCtrl.abort();
            const ctrl = new AbortController();
            state.abortCtrl = ctrl;
            const ctx = context();
            state.mode = 'capturing';
            setPressed(true);
            bar.hidden = false;
            recaptureBtn.disabled = true;
            setStatus(t('homepage_studio.annotate_capturing'));
            deps.announce(t('homepage_studio.annotate_capturing'));
            try {
                const data = await deps.api(SNAPSHOT_ENDPOINT, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    /* Deployed targets are captured only when preview_url matches a
                       registered site URL; the local target ignores it. */
                    body: JSON.stringify({
                        site_id: ctx.siteId,
                        target: ctx.target,
                        preview_url: ctx.previewUrl,
                        path: pathInput.value.trim() || '/',
                        viewport: viewport(ctx)
                    }),
                    signal: ctrl.signal
                });
                if (ctrl.signal.aborted || deps.isDisposed()) return;
                await showSnapshot(data, ctx, ctrl);
            } catch (err) {
                if (ctrl.signal.aborted || deps.isDisposed() || (err && err.name === 'AbortError')) return;
                failCapture(err && err.body && ERROR_CODES.includes(err.body.code) ? err.body.code : 'capture_failed');
            } finally {
                if (state.abortCtrl === ctrl) state.abortCtrl = null;
                recaptureBtn.disabled = false;
            }
        }

        function showSnapshot(data, ctx, ctrl) {
            const valid = data && typeof data.image === 'string' && data.image.indexOf('data:image/jpeg;base64,') === 0
                && data.page && Array.isArray(data.elements);
            if (!valid) {
                failCapture('capture_failed');
                return Promise.resolve();
            }
            return new Promise(resolve => {
                shot.onload = () => {
                    shot.onload = null;
                    shot.onerror = null;
                    /* The user left annotate mode while the image was decoding. */
                    if (ctrl.signal.aborted || deps.isDisposed()) {
                        resolve();
                        return;
                    }
                    state.snapshot = data;
                    state.snapshotContext = ctx;
                    state.focusList = tabOrder(data.elements);
                    state.focusPos = -1;
                    canvas.style.width = data.page.width + 'px';
                    canvas.style.height = data.page.height + 'px';
                    renderMarkers();
                    resume();
                    setStatus(data.page.truncated ? t('homepage_studio.annotate_truncated') : '');
                    deps.announce(t('homepage_studio.annotate_ready'));
                    resolve();
                };
                shot.onerror = () => {
                    shot.onload = null;
                    shot.onerror = null;
                    if (!ctrl.signal.aborted && !deps.isDisposed()) failCapture('capture_failed');
                    resolve();
                };
                shot.src = data.image;
            });
        }

        function resume() {
            state.mode = 'annotating';
            stage.classList.add('is-annotating');
            layer.hidden = false;
            bar.hidden = false;
            setPressed(true);
            layer.focus();
        }

        function closePopover(refocus) {
            if (!state.popover) return;
            state.popover = null;
            popover.hidden = true;
            hoverBox.hidden = true;
            if (refocus !== false && state.mode === 'annotating') layer.focus();
        }

        function exitMode(returnFocus) {
            if (state.abortCtrl) {
                state.abortCtrl.abort();
                state.abortCtrl = null;
            }
            closePopover(false);
            state.drag = null;
            state.mode = 'idle';
            stage.classList.remove('is-annotating');
            layer.hidden = true;
            bar.hidden = true;
            hoverBox.hidden = true;
            hoverLabel.hidden = true;
            dragBox.hidden = true;
            setPressed(false);
            if (returnFocus) button.focus();
        }

        function renderMarkers() {
            markerLayer.textContent = '';
            state.marks.forEach(mark => {
                const outline = node('div', 'vd-hp-mark-outline is-' + mark.kind);
                placeBox(outline, mark.box);
                const badge = actionButton('vd-hp-mark-badge is-' + mark.kind, String(mark.n));
                badge.tabIndex = -1;
                badge.dataset.markN = String(mark.n);
                badge.setAttribute('aria-label', t('homepage_studio.annotate_mark_heading', { n: mark.n }) + ': ' + mark.comment);
                outline.appendChild(badge);
                markerLayer.appendChild(outline);
            });
        }

        function renderTray() {
            tray.textContent = '';
            tray.hidden = state.marks.length === 0;
            state.marks.forEach(mark => {
                const chip = node('span', 'vd-hp-mark-chip');
                const open = actionButton('vd-hp-mark-chip-open');
                open.dataset.markN = String(mark.n);
                open.title = mark.comment;
                open.append(node('span', 'vd-hp-mark-num is-' + mark.kind, String(mark.n)), node('span', 'vd-hp-mark-text', mark.comment));
                const remove = actionButton('vd-hp-mark-remove', '×');
                remove.setAttribute('data-mark-remove', String(mark.n));
                remove.setAttribute('aria-label', t('homepage_studio.annotate_remove_mark', { n: mark.n }));
                chip.append(open, remove);
                tray.appendChild(chip);
            });
            if (state.marks.length > 1) {
                const clear = actionButton('vd-hp-mark-clear', t('homepage_studio.annotate_discard_all'));
                clear.setAttribute('data-mark-clear', '');
                tray.appendChild(clear);
            }
        }

        function clearMarks() {
            state.marks = [];
            renderMarkers();
            renderTray();
        }

        function forgetSnapshot() {
            state.snapshot = null;
            state.snapshotContext = null;
            shot.removeAttribute('src');
        }

        function markByNumber(value) {
            return state.marks.find(mark => String(mark.n) === String(value)) || null;
        }

        function openPopover(mark, isNew) {
            state.popover = { mark, isNew };
            popTitle.textContent = t('homepage_studio.annotate_mark_heading', { n: mark.n });
            popInput.value = mark.comment || '';
            popDelete.hidden = isNew;
            placeBox(hoverBox, mark.box);
            hoverBox.hidden = false;
            hoverLabel.hidden = true;
            const rect = panelRect(mark.box);
            const base = panel.getBoundingClientRect();
            const width = 264;
            const right = rect.left + rect.width + 12;
            const left = right + width <= base.width - 8 ? right : rect.left - width - 12;
            popover.style.left = Math.max(8, Math.min(base.width - width - 8, left)) + 'px';
            popover.style.top = Math.max(8, Math.min(base.height - 190, rect.top)) + 'px';
            popover.hidden = false;
            popInput.focus();
        }

        function startMark(partial) {
            if (state.marks.length >= MAX_MARKS) {
                notice(t('homepage_studio.annotate_limit', { max: MAX_MARKS }));
                return;
            }
            openPopover(Object.assign({ n: state.marks.length + 1, comment: '' }, partial), true);
        }

        function savePopover() {
            const pop = state.popover;
            if (!pop) return;
            const comment = popInput.value.replace(/\s+/g, ' ').trim().slice(0, 1000);
            if (!comment) {
                closePopover();
                return;
            }
            pop.mark.comment = comment;
            if (pop.isNew) {
                pop.mark.n = state.marks.length + 1;
                state.marks.push(pop.mark);
            }
            closePopover();
            renderMarkers();
            renderTray();
            deps.announce(t('homepage_studio.annotate_mark_saved', { n: pop.mark.n }));
        }

        function deleteMark(mark) {
            state.marks = state.marks.filter(item => item !== mark);
            state.marks.forEach((item, index) => { item.n = index + 1; });
            renderMarkers();
            renderTray();
        }

        async function confirmDiscard() {
            return !state.marks.length || !!(await deps.confirmDialog(t('homepage_studio.annotate_discard_confirm')));
        }

        async function toggle() {
            if (state.mode !== 'idle') {
                exitMode(true);
                return;
            }
            if (!state.available) {
                notice(t('homepage_studio.annotate_unavailable'));
                return;
            }
            const ctx = context();
            if (state.snapshot && sameSource(state.snapshotContext, ctx) && state.snapshotContext.device === ctx.device) {
                resume();
                return;
            }
            if (!(await confirmDiscard())) return;
            clearMarks();
            capture();
        }

        async function recapture() {
            if (!(await confirmDiscard())) return;
            clearMarks();
            capture();
        }

        async function discardAll() {
            if (!(await confirmDiscard())) return;
            exitMode(false);
            clearMarks();
            forgetSnapshot();
        }

        function composeImage(marks) {
            try {
                const layout = cropLayout(marks, state.snapshot.page);
                const surface = document.createElement('canvas');
                surface.width = layout.width;
                surface.height = layout.height;
                const g = surface.getContext('2d');
                if (!g) return '';
                g.fillStyle = '#ffffff';
                g.fillRect(0, 0, layout.width, layout.height);
                layout.crops.forEach(crop => {
                    const [sx, sy, sw, sh] = crop.src;
                    const [dx, dy, dw, dh] = crop.dst;
                    g.drawImage(shot, sx, sy, sw, sh, dx, dy, dw, dh);
                    const bx = dx + (crop.box[0] - sx) * crop.scale;
                    const by = dy + (crop.box[1] - sy) * crop.scale;
                    g.lineWidth = 3;
                    g.strokeStyle = MARK_COLOR;
                    g.strokeRect(bx, by, crop.box[2] * crop.scale, crop.box[3] * crop.scale);
                    const cx = Math.max(dx + 14, Math.min(dx + dw - 14, bx));
                    const cy = Math.max(dy + 14, Math.min(dy + dh - 14, by));
                    g.fillStyle = MARK_COLOR;
                    g.beginPath();
                    g.arc(cx, cy, 13, 0, Math.PI * 2);
                    g.fill();
                    g.fillStyle = '#ffffff';
                    g.font = 'bold 15px sans-serif';
                    g.textAlign = 'center';
                    g.textBaseline = 'middle';
                    g.fillText(String(crop.n), cx, cy + 1);
                });
                let url = surface.toDataURL('image/jpeg', 0.85);
                if (url.length > IMAGE_MAX_BASE64) url = surface.toDataURL('image/jpeg', 0.6);
                return url.indexOf('data:image/jpeg;base64,') === 0 && url.length <= IMAGE_MAX_BASE64 ? url : '';
            } catch (_) {
                return '';
            }
        }

        function setAvailable(available) {
            state.available = !!available;
            button.disabled = !state.available;
            button.classList.toggle('is-disabled', !state.available);
            button.title = t(state.available ? 'homepage_studio.annotate_button' : 'homepage_studio.annotate_unavailable');
            if (!state.available && state.mode !== 'idle') exitMode(false);
        }

        listen(button, 'click', () => { toggle(); });
        listen(doneBtn, 'click', () => exitMode(true));
        listen(recaptureBtn, 'click', () => { recapture(); });
        listen(pathInput, 'keydown', e => {
            if (e.key === 'Enter') {
                e.preventDefault();
                recapture();
            } else if (e.key === 'Escape') {
                e.preventDefault();
                e.stopPropagation();
                exitMode(true);
            }
        });
        listen(popSave, 'click', savePopover);
        listen(popCancel, 'click', () => closePopover());
        listen(popDelete, 'click', () => {
            const pop = state.popover;
            if (!pop) return;
            closePopover();
            deleteMark(pop.mark);
        });
        listen(popInput, 'keydown', e => {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                savePopover();
            } else if (e.key === 'Escape') {
                e.preventDefault();
                e.stopPropagation();
                closePopover();
            }
        });
        listen(layer, 'pointerdown', e => {
            if (state.mode !== 'annotating' || state.popover || e.button !== 0) return;
            if (e.target && e.target.closest && e.target.closest('.vd-hp-mark-badge')) return;
            const point = pagePoint(e);
            state.drag = { x: point.x, y: point.y, moved: false, pointerId: e.pointerId };
            try { layer.setPointerCapture(e.pointerId); } catch (_) { /* synthetic pointers have no capture */ }
            e.preventDefault();
        });
        listen(layer, 'pointermove', e => {
            if (state.mode !== 'annotating' || state.popover) return;
            const point = pagePoint(e);
            if (state.drag) {
                const box = normalizeBox(state.drag.x, state.drag.y, point.x, point.y);
                if (box[2] > DRAG_THRESHOLD || box[3] > DRAG_THRESHOLD) {
                    state.drag.moved = true;
                    placeBox(dragBox, box);
                    dragBox.hidden = false;
                    hoverBox.hidden = true;
                    hoverLabel.hidden = true;
                }
                return;
            }
            showHover(hitTest(state.snapshot.elements, point.x, point.y));
        });
        listen(layer, 'pointerup', e => {
            const drag = state.drag;
            state.drag = null;
            if (!drag || state.mode !== 'annotating') return;
            try { layer.releasePointerCapture(drag.pointerId); } catch (_) { /* see pointerdown */ }
            dragBox.hidden = true;
            const point = pagePoint(e);
            if (drag.moved) {
                const box = normalizeBox(drag.x, drag.y, point.x, point.y);
                startMark({ kind: 'area', box, overlapIndexes: overlapping(state.snapshot.elements, box) });
                return;
            }
            const index = hitTest(state.snapshot.elements, point.x, point.y);
            if (index >= 0) startMark({ kind: 'element', box: state.snapshot.elements[index].box.slice(), elementIndex: index });
        });
        listen(layer, 'pointerleave', () => {
            if (!state.drag && !state.popover) showHover(-1);
        });
        listen(layer, 'scroll', () => {
            if (!state.popover) hoverLabel.hidden = true;
        });
        listen(layer, 'keydown', e => {
            if (state.mode !== 'annotating' || state.popover) return;
            if (e.key === 'Escape') {
                e.preventDefault();
                e.stopPropagation();
                exitMode(true);
            } else if (e.key === 'Tab' && state.focusList.length) {
                e.preventDefault();
                const count = state.focusList.length;
                state.focusPos = e.shiftKey
                    ? (state.focusPos <= 0 ? count - 1 : state.focusPos - 1)
                    : (state.focusPos + 1) % count;
                const item = state.snapshot.elements[state.focusList[state.focusPos]];
                if (item.box[1] < layer.scrollTop || item.box[1] + item.box[3] > layer.scrollTop + layer.clientHeight) {
                    layer.scrollTop = Math.max(0, item.box[1] - 40);
                }
                showHover(state.focusList[state.focusPos]);
                deps.announce(describe(item));
            } else if (e.key === 'Enter' && state.focusPos >= 0) {
                e.preventDefault();
                const index = state.focusList[state.focusPos];
                startMark({ kind: 'element', box: state.snapshot.elements[index].box.slice(), elementIndex: index });
            }
        });
        listen(markerLayer, 'click', e => {
            const badge = e.target && e.target.closest ? e.target.closest('.vd-hp-mark-badge') : null;
            const mark = badge ? markByNumber(badge.dataset.markN) : null;
            if (mark && !state.popover) openPopover(mark, false);
        });
        listen(tray, 'click', e => {
            const target = e.target && e.target.closest ? e.target.closest('button') : null;
            if (!target) return;
            if (target.hasAttribute('data-mark-remove')) {
                const mark = markByNumber(target.getAttribute('data-mark-remove'));
                if (mark) deleteMark(mark);
            } else if (target.hasAttribute('data-mark-clear')) {
                discardAll();
            } else if (target.dataset.markN) {
                const mark = markByNumber(target.dataset.markN);
                if (!mark || !state.snapshot) return;
                if (state.mode !== 'annotating') resume();
                openPopover(mark, false);
            }
        });
        listen(tray, 'keydown', e => {
            if (e.key !== 'Delete') return;
            const mark = e.target && e.target.dataset ? markByNumber(e.target.dataset.markN) : null;
            if (!mark) return;
            e.preventDefault();
            deleteMark(mark);
        });

        setAvailable(false);

        return {
            setAvailable,
            contextChanged() {
                if (!state.snapshotContext || sameSource(state.snapshotContext, context())) return;
                const hadMarks = state.marks.length > 0;
                exitMode(false);
                clearMarks();
                forgetSnapshot();
                if (hadMarks) {
                    notice(t('homepage_studio.annotate_discarded_context'));
                    deps.announce(t('homepage_studio.annotate_discarded_context'));
                }
            },
            count() {
                return state.marks.length;
            },
            isActive() {
                return state.mode !== 'idle';
            },
            /* Builds the payload for the next chat turn and clears the marks. */
            takeForSend(text) {
                if (!state.marks.length || !state.snapshot) return null;
                const marks = state.marks.map(mark => Object.assign({}, mark, { box: mark.box.slice() }));
                const image = composeImage(marks);
                const result = {
                    message: composeMessage(text, marks, {
                        defaultMessage: t('homepage_studio.annotate_default_message'),
                        heading: t('homepage_studio.annotate_marks_heading')
                    }),
                    context: { annotations: annotationPayload(state.snapshot, marks) },
                    thumbnail: image
                };
                if (image) result.context.image_base64 = image.slice(image.indexOf(',') + 1);
                exitMode(false);
                clearMarks();
                forgetSnapshot();
                return result;
            },
            attachThumbnail(chatLog, result) {
                if (!chatLog || !result || !result.thumbnail) return;
                const bubbles = chatLog.querySelectorAll('.vd-chat-bubble.user');
                const bubble = bubbles[bubbles.length - 1];
                if (!bubble) return;
                const img = node('img', 'vd-hp-mark-thumb');
                img.alt = t('homepage_studio.annotate_thumbnail_alt');
                img.src = result.thumbnail;
                bubble.appendChild(img);
            },
            dispose() {
                if (state.abortCtrl) state.abortCtrl.abort();
                listeners.forEach(off => off());
                listeners.length = 0;
                stage.classList.remove('is-annotating');
                [button, layer, hoverLabel, bar, popover, tray].forEach(element => {
                    if (element.parentNode) element.parentNode.removeChild(element);
                });
            }
        };
    }

    window.HomepageStudioAnnotate = { create, hitTest, overlapping, normalizeBox, tabOrder, composeMessage, annotationPayload, cropLayout, MAX_MARKS };
})();
