/**
 * Virtual Desktop media lightbox.
 *
 * Full-window viewer for images, videos and audio with zoom/pan (wheel, pinch,
 * drag, double-click), keyboard and swipe navigation, filmstrip, slideshow,
 * details panel and item actions. Used by the Gallery app and by the shell's
 * generic media preview. Only one lightbox is open at a time.
 */
(function () {
    'use strict';

    const SLIDESHOW_MS = 4000;
    const IDLE_HIDE_MS = 2600;
    const MIN_SCALE = 1;
    const MAX_SCALE = 8;
    const STRIP_WINDOW = 40;
    let active = null;

    function kindOf(item, context) {
        if (context && typeof context.mediaPreviewKind === 'function') {
            const kind = context.mediaPreviewKind(item);
            if (kind === 'image' || kind === 'video' || kind === 'audio') return kind;
        }
        const kind = String((item && item.media_kind) || '').toLowerCase();
        if (kind === 'image' || kind === 'video' || kind === 'audio') return kind;
        const ext = String((item && (item.name || item.path)) || '').split('.').pop().toLowerCase();
        if (['mp4', 'webm', 'mkv', 'mov', 'm4v'].includes(ext)) return 'video';
        if (['mp3', 'm4a', 'ogg', 'opus', 'wav', 'flac'].includes(ext)) return 'audio';
        return 'image';
    }

    function clamp(value, min, max) {
        return Math.max(min, Math.min(max, value));
    }

    function open(options) {
        if (active) active.close();
        const opts = options || {};
        const ctx = opts.context || {};
        const actions = opts.actions || {};
        const t = ctx.t || (key => key);
        const esc = ctx.esc || (value => String(value == null ? '' : value)
            .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;'));
        const iconMarkup = ctx.iconMarkup || ((key, fallback) => `<span aria-hidden="true">${esc(fallback || '')}</span>`);
        const previewURL = typeof ctx.mediaPreviewURL === 'function' ? ctx.mediaPreviewURL : (file => (file && file.web_path) || '');
        const downloadURL = typeof ctx.mediaDownloadURL === 'function' ? ctx.mediaDownloadURL : previewURL;
        const fmtBytes = typeof ctx.fmtBytes === 'function' ? ctx.fmtBytes : (n => `${n} B`);
        const formatDateTime = typeof ctx.formatDateTime === 'function' ? ctx.formatDateTime : (value => (value ? new Date(value).toLocaleString() : ''));
        const formatDuration = typeof ctx.formatDuration === 'function' ? ctx.formatDuration : (seconds => `${Math.round(seconds)}s`);
        const kindLabel = typeof ctx.kindLabel === 'function' ? ctx.kindLabel : (kind => kind);
        const readonly = !!ctx.readonly;
        const animations = ctx.animationsEnabled !== false && !(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);

        const lb = {
            items: Array.isArray(opts.items) ? opts.items.slice() : [],
            index: 0,
            infoOpen: false,
            slideshow: false,
            zoom: { scale: 1, x: 0, y: 0 },
            natural: { width: 0, height: 0 },
            closed: false,
            timers: {},
            pointers: new Map(),
            gesture: null,
            openedAt: Date.now(),
            restoreFocus: document.activeElement
        };
        lb.index = clamp(Number(opts.index) || 0, 0, Math.max(0, lb.items.length - 1));
        if (!lb.items.length) return null;

        const root = document.createElement('div');
        root.className = 'vd-lightbox';
        root.setAttribute('role', 'dialog');
        root.setAttribute('aria-modal', 'true');
        root.setAttribute('aria-label', t('desktop.gallery_lightbox_label'));
        root.tabIndex = -1;
        if (!animations) root.classList.add('no-animations');
        root.innerHTML = `
            <div class="vd-lightbox-backdrop" data-lb-backdrop></div>
            <header class="vd-lightbox-bar">
                <div class="vd-lightbox-title">
                    <strong class="vd-lightbox-name" data-lb-name></strong>
                    <span class="vd-lightbox-counter" data-lb-counter></span>
                </div>
                <div class="vd-lightbox-actions">
                    <div class="vd-lightbox-zoom" data-lb-zoom-group role="group" aria-label="${esc(t('desktop.gallery_zoom_fit'))}">
                        <button type="button" class="vd-lightbox-button" data-lb-zoom-out title="${esc(t('desktop.gallery_zoom_out'))}" aria-label="${esc(t('desktop.gallery_zoom_out'))}">${iconMarkup('zoom-out', '−', 'vd-viewer-action-icon', 16)}</button>
                        <button type="button" class="vd-lightbox-button vd-lightbox-zoom-label" data-lb-zoom-reset title="${esc(t('desktop.gallery_zoom_fit'))}">100%</button>
                        <button type="button" class="vd-lightbox-button" data-lb-zoom-in title="${esc(t('desktop.gallery_zoom_in'))}" aria-label="${esc(t('desktop.gallery_zoom_in'))}">${iconMarkup('zoom-in', '+', 'vd-viewer-action-icon', 16)}</button>
                        <button type="button" class="vd-lightbox-button" data-lb-zoom-actual title="${esc(t('desktop.gallery_zoom_actual'))}" aria-label="${esc(t('desktop.gallery_zoom_actual'))}">${iconMarkup('maximize', '1:1', 'vd-viewer-action-icon', 15)}</button>
                    </div>
                    <button type="button" class="vd-lightbox-button" data-lb-slideshow aria-pressed="false" title="${esc(t('desktop.gallery_slideshow_start'))}" aria-label="${esc(t('desktop.gallery_slideshow_start'))}">${iconMarkup('play', '▶', 'vd-viewer-action-icon', 16)}</button>
                    <button type="button" class="vd-lightbox-button" data-lb-info aria-pressed="false" title="${esc(t('desktop.gallery_info'))}" aria-label="${esc(t('desktop.gallery_info'))}">${iconMarkup('info', 'i', 'vd-viewer-action-icon', 16)}</button>
                    <button type="button" class="vd-lightbox-button" data-lb-download title="${esc(t('desktop.gallery_download'))}" aria-label="${esc(t('desktop.gallery_download'))}">${iconMarkup('gallery-action-download', '↓', 'vd-viewer-action-icon', 16)}</button>
                    <button type="button" class="vd-lightbox-button" data-lb-edit title="${esc(t('desktop.gallery_edit_pixel'))}" aria-label="${esc(t('desktop.gallery_edit_pixel'))}" hidden>${iconMarkup('gallery-action-edit', '✎', 'vd-viewer-action-icon', 16)}</button>
                    <button type="button" class="vd-lightbox-button" data-lb-rename title="${esc(t('desktop.gallery_rename'))}" aria-label="${esc(t('desktop.gallery_rename'))}"${readonly || typeof actions.rename !== 'function' ? ' hidden' : ''}>${iconMarkup('edit', '✎', 'vd-viewer-action-icon', 16)}</button>
                    <button type="button" class="vd-lightbox-button vd-lightbox-button-danger" data-lb-delete title="${esc(t('desktop.gallery_delete'))}" aria-label="${esc(t('desktop.gallery_delete'))}"${readonly || typeof actions.remove !== 'function' ? ' hidden' : ''}>${iconMarkup('gallery-action-delete', '🗑', 'vd-viewer-action-icon', 16)}</button>
                    <button type="button" class="vd-lightbox-button vd-lightbox-close" data-lb-close title="${esc(t('desktop.close'))}" aria-label="${esc(t('desktop.close'))}">${iconMarkup('x', '×', 'vd-viewer-action-icon', 16)}</button>
                </div>
                <div class="vd-lightbox-progress" data-lb-progress hidden><span></span></div>
            </header>
            <div class="vd-lightbox-stage" data-lb-stage>
                <button type="button" class="vd-lightbox-nav vd-lightbox-prev" data-lb-prev title="${esc(t('desktop.gallery_prev'))}" aria-label="${esc(t('desktop.gallery_prev'))}">${iconMarkup('chevron-left', '‹', 'vd-viewer-action-icon', 26)}</button>
                <div class="vd-lightbox-canvas" data-lb-canvas></div>
                <button type="button" class="vd-lightbox-nav vd-lightbox-next" data-lb-next title="${esc(t('desktop.gallery_next'))}" aria-label="${esc(t('desktop.gallery_next'))}">${iconMarkup('chevron-right', '›', 'vd-viewer-action-icon', 26)}</button>
                <aside class="vd-lightbox-info" data-lb-info-panel aria-label="${esc(t('desktop.gallery_details'))}" hidden></aside>
                <div class="vd-lightbox-spinner" data-lb-spinner hidden aria-hidden="true"></div>
            </div>
            <footer class="vd-lightbox-strip" data-lb-strip role="listbox" aria-label="${esc(t('desktop.gallery_library'))}"></footer>`;
        document.body.appendChild(root);
        document.body.classList.add('vd-lightbox-open');

        const el = {
            name: root.querySelector('[data-lb-name]'),
            counter: root.querySelector('[data-lb-counter]'),
            stage: root.querySelector('[data-lb-stage]'),
            canvas: root.querySelector('[data-lb-canvas]'),
            strip: root.querySelector('[data-lb-strip]'),
            info: root.querySelector('[data-lb-info-panel]'),
            infoButton: root.querySelector('[data-lb-info]'),
            slideshowButton: root.querySelector('[data-lb-slideshow]'),
            progress: root.querySelector('[data-lb-progress]'),
            zoomGroup: root.querySelector('[data-lb-zoom-group]'),
            zoomLabel: root.querySelector('[data-lb-zoom-reset]'),
            edit: root.querySelector('[data-lb-edit]'),
            prev: root.querySelector('[data-lb-prev]'),
            next: root.querySelector('[data-lb-next]'),
            spinner: root.querySelector('[data-lb-spinner]')
        };

        function current() {
            return lb.items[lb.index] || null;
        }

        function media() {
            return el.canvas.querySelector('.vd-lightbox-media');
        }

        // ── Display ───────────────────────────────────────────────────

        function show(index, direction) {
            if (lb.closed || !lb.items.length) return;
            lb.index = clamp(index, 0, lb.items.length - 1);
            const item = current();
            const kind = kindOf(item, ctx);
            root.dataset.kind = kind;
            lb.zoom = { scale: 1, x: 0, y: 0 };
            lb.natural = { width: item._width || 0, height: item._height || 0 };
            el.name.textContent = item.name || item.path || '';
            el.counter.textContent = lb.items.length > 1 ? t('desktop.gallery_counter', { index: lb.index + 1, total: lb.items.length }) : '';
            el.prev.hidden = lb.items.length < 2;
            el.next.hidden = lb.items.length < 2;
            el.edit.hidden = kind !== 'image' || typeof actions.edit !== 'function';
            el.zoomGroup.hidden = kind !== 'image';
            el.slideshowButton.hidden = lb.items.length < 2;
            el.canvas.classList.remove('is-failed');
            el.spinner.hidden = false;
            const url = previewURL(item);
            let node;
            if (kind === 'video') {
                node = document.createElement('video');
                node.className = 'vd-lightbox-media vd-lightbox-video';
                node.controls = true;
                node.autoplay = true;
                node.playsInline = true;
                node.preload = 'metadata';
                node.src = url;
                node.addEventListener('loadedmetadata', () => {
                    lb.natural = { width: node.videoWidth, height: node.videoHeight };
                    item._width = node.videoWidth;
                    item._height = node.videoHeight;
                    item._duration = node.duration;
                    el.spinner.hidden = true;
                    if (lb.infoOpen) renderInfo();
                }, { once: true });
                node.addEventListener('ended', () => { if (lb.slideshow) step(1); });
                node.addEventListener('error', () => fail(), { once: true });
            } else if (kind === 'audio') {
                node = document.createElement('div');
                node.className = 'vd-lightbox-media vd-lightbox-audio';
                node.innerHTML = `<div class="vd-lightbox-audio-art">${iconMarkup('music', '♪', 'vd-viewer-action-icon', 64)}</div><audio controls autoplay src="${esc(url)}"></audio>`;
                const audio = node.querySelector('audio');
                audio.addEventListener('loadedmetadata', () => { item._duration = audio.duration; el.spinner.hidden = true; if (lb.infoOpen) renderInfo(); }, { once: true });
                audio.addEventListener('ended', () => { if (lb.slideshow) step(1); });
                audio.addEventListener('error', () => fail(), { once: true });
                el.spinner.hidden = true;
            } else {
                node = document.createElement('img');
                node.className = 'vd-lightbox-media vd-lightbox-image';
                node.alt = item.name || '';
                node.draggable = false;
                node.decoding = 'async';
                node.addEventListener('load', () => {
                    lb.natural = { width: node.naturalWidth, height: node.naturalHeight };
                    item._width = node.naturalWidth;
                    item._height = node.naturalHeight;
                    el.spinner.hidden = true;
                    node.classList.add('is-loaded');
                    updateZoomLabel();
                    if (lb.infoOpen) renderInfo();
                }, { once: true });
                node.addEventListener('error', () => fail(), { once: true });
                node.src = url;
                if (node.complete && node.naturalWidth) { el.spinner.hidden = true; node.classList.add('is-loaded'); }
            }
            if (animations && direction) node.classList.add(direction < 0 ? 'enter-from-left' : 'enter-from-right');
            el.canvas.replaceChildren(node);
            applyZoom();
            updateZoomLabel();
            renderStrip();
            if (lb.infoOpen) renderInfo();
            preloadNeighbours();
            if (lb.slideshow) restartSlideshowTimer(kind);
            if (typeof opts.onChange === 'function') opts.onChange(item, lb.index);
        }

        function fail() {
            el.spinner.hidden = true;
            el.canvas.classList.add('is-failed');
            const note = document.createElement('div');
            note.className = 'vd-lightbox-failed';
            note.innerHTML = `${iconMarkup('info', '!', 'vd-viewer-action-icon', 28)}<p>${esc(t('desktop.gallery_lightbox_failed'))}</p>`;
            el.canvas.replaceChildren(note);
        }

        function preloadNeighbours() {
            for (const offset of [1, -1]) {
                const item = lb.items[lb.index + offset];
                if (!item || kindOf(item, ctx) !== 'image') continue;
                const image = new Image();
                image.decoding = 'async';
                image.src = previewURL(item);
            }
        }

        function step(delta) {
            if (lb.items.length < 2) return;
            const next = (lb.index + delta + lb.items.length) % lb.items.length;
            show(next, delta);
        }

        // ── Filmstrip ─────────────────────────────────────────────────

        function renderStrip() {
            if (lb.items.length < 2) { el.strip.hidden = true; return; }
            el.strip.hidden = false;
            const start = Math.max(0, lb.index - STRIP_WINDOW);
            const end = Math.min(lb.items.length, lb.index + STRIP_WINDOW + 1);
            if (lb.stripDirty || el.strip.dataset.start !== String(start) || el.strip.dataset.end !== String(end)) {
                lb.stripDirty = false;
                let html = '';
                for (let index = start; index < end; index += 1) {
                    const item = lb.items[index];
                    const kind = kindOf(item, ctx);
                    const thumb = kind === 'image'
                        ? `<img src="${esc(previewURL(item))}" alt="" loading="lazy" decoding="async" draggable="false">`
                        : `<span class="vd-lightbox-strip-icon">${iconMarkup(kind === 'video' ? 'video' : 'music', kind === 'video' ? '▶' : '♪', 'vd-viewer-action-icon', 18)}</span>`;
                    html += `<button type="button" class="vd-lightbox-thumb" role="option" data-lb-thumb="${index}" title="${esc(item.name || '')}" aria-label="${esc(item.name || '')}">${thumb}</button>`;
                }
                el.strip.innerHTML = html;
                el.strip.dataset.start = String(start);
                el.strip.dataset.end = String(end);
            }
            el.strip.querySelectorAll('[data-lb-thumb]').forEach(button => {
                const isCurrent = Number(button.dataset.lbThumb) === lb.index;
                button.classList.toggle('is-current', isCurrent);
                button.setAttribute('aria-selected', String(isCurrent));
                if (isCurrent) button.scrollIntoView({ block: 'nearest', inline: 'center', behavior: animations ? 'smooth' : 'auto' });
            });
        }

        // ── Zoom and pan ──────────────────────────────────────────────

        function fittedSize() {
            const node = media();
            if (!node || !(node instanceof HTMLImageElement)) return null;
            const rect = node.getBoundingClientRect();
            return { width: rect.width / lb.zoom.scale, height: rect.height / lb.zoom.scale };
        }

        function applyZoom() {
            const node = media();
            if (!node) return;
            if (!(node instanceof HTMLImageElement)) { node.style.transform = ''; return; }
            constrainPan();
            node.style.transform = `translate(${lb.zoom.x}px, ${lb.zoom.y}px) scale(${lb.zoom.scale})`;
            el.canvas.classList.toggle('is-zoomed', lb.zoom.scale > 1.001);
        }

        function constrainPan() {
            const fitted = fittedSize();
            if (!fitted) return;
            const stage = el.stage.getBoundingClientRect();
            const maxX = Math.max(0, (fitted.width * lb.zoom.scale - stage.width) / 2);
            const maxY = Math.max(0, (fitted.height * lb.zoom.scale - stage.height) / 2);
            lb.zoom.x = clamp(lb.zoom.x, -maxX, maxX);
            lb.zoom.y = clamp(lb.zoom.y, -maxY, maxY);
        }

        function updateZoomLabel() {
            const fitted = fittedSize();
            if (!fitted || !lb.natural.width) { el.zoomLabel.textContent = `${Math.round(lb.zoom.scale * 100)}%`; return; }
            const displayed = fitted.width * lb.zoom.scale;
            el.zoomLabel.textContent = `${Math.max(1, Math.round((displayed / lb.natural.width) * 100))}%`;
        }

        function zoomTo(scale, clientX, clientY) {
            const node = media();
            if (!node || !(node instanceof HTMLImageElement)) return;
            const next = clamp(scale, MIN_SCALE, MAX_SCALE);
            if (next === lb.zoom.scale) return;
            const stage = el.stage.getBoundingClientRect();
            const cx = (typeof clientX === 'number' ? clientX : stage.left + stage.width / 2) - (stage.left + stage.width / 2);
            const cy = (typeof clientY === 'number' ? clientY : stage.top + stage.height / 2) - (stage.top + stage.height / 2);
            const ratio = next / lb.zoom.scale;
            lb.zoom.x = cx - (cx - lb.zoom.x) * ratio;
            lb.zoom.y = cy - (cy - lb.zoom.y) * ratio;
            lb.zoom.scale = next;
            if (next <= MIN_SCALE) { lb.zoom.x = 0; lb.zoom.y = 0; }
            applyZoom();
            updateZoomLabel();
        }

        function zoomActual() {
            const fitted = fittedSize();
            if (!fitted || !lb.natural.width) return;
            zoomTo(lb.natural.width / fitted.width);
        }

        function resetZoom() {
            lb.zoom = { scale: 1, x: 0, y: 0 };
            applyZoom();
            updateZoomLabel();
        }

        // ── Slideshow and idle chrome ─────────────────────────────────

        function restartSlideshowTimer(kind) {
            clearTimeout(lb.timers.slideshow);
            el.progress.hidden = false;
            el.progress.classList.remove('is-running');
            if (kind === 'video' || kind === 'audio') { el.progress.hidden = true; return; }
            void el.progress.offsetWidth;
            el.progress.style.setProperty('--vd-lightbox-slide-ms', `${SLIDESHOW_MS}ms`);
            el.progress.classList.add('is-running');
            lb.timers.slideshow = setTimeout(() => { if (lb.slideshow) step(1); }, SLIDESHOW_MS);
        }

        function setSlideshow(enabled) {
            lb.slideshow = !!enabled && lb.items.length > 1;
            el.slideshowButton.setAttribute('aria-pressed', String(lb.slideshow));
            const label = t(lb.slideshow ? 'desktop.gallery_slideshow_stop' : 'desktop.gallery_slideshow_start');
            el.slideshowButton.title = label;
            el.slideshowButton.setAttribute('aria-label', label);
            el.slideshowButton.innerHTML = iconMarkup(lb.slideshow ? 'pause' : 'play', lb.slideshow ? '❚❚' : '▶', 'vd-viewer-action-icon', 16);
            root.classList.toggle('is-slideshow', lb.slideshow);
            if (lb.slideshow) restartSlideshowTimer(kindOf(current(), ctx));
            else { clearTimeout(lb.timers.slideshow); el.progress.hidden = true; el.progress.classList.remove('is-running'); }
        }

        function pokeIdle() {
            root.classList.remove('is-idle');
            clearTimeout(lb.timers.idle);
            lb.timers.idle = setTimeout(() => {
                if (lb.closed || lb.infoOpen) return;
                if (root.contains(document.activeElement) && document.activeElement !== root && document.activeElement.closest('.vd-lightbox-bar, .vd-lightbox-strip')) return;
                root.classList.add('is-idle');
            }, IDLE_HIDE_MS);
        }

        // ── Info panel ────────────────────────────────────────────────

        function renderInfo() {
            const item = current();
            if (!item) return;
            const kind = kindOf(item, ctx);
            const rows = [
                [t('desktop.gallery_field_type'), kindLabel(kind)],
                [t('desktop.gallery_field_size'), fmtBytes(Number(item.size) || 0)]
            ];
            if (lb.natural.width && lb.natural.height) rows.push([t('desktop.gallery_field_dimensions'), `${lb.natural.width} × ${lb.natural.height}`]);
            if (item._duration) rows.push([t('desktop.gallery_field_duration'), formatDuration(item._duration)]);
            const modified = formatDateTime(item.mod_time || item.modified);
            if (modified) rows.push([t('desktop.gallery_field_modified'), modified]);
            const created = formatDateTime(item.created);
            if (created && created !== modified) rows.push([t('desktop.gallery_field_created'), created]);
            const dir = String(item.path || '').split('/').slice(0, -1).join('/');
            if (dir) rows.push([t('desktop.gallery_field_path'), dir]);
            el.info.innerHTML = `<h3 class="vd-lightbox-info-title">${esc(item.name || '')}</h3>
                <dl class="vd-lightbox-info-list">${rows.map(([label, value]) => `<div><dt>${esc(label)}</dt><dd title="${esc(value)}">${esc(value)}</dd></div>`).join('')}</dl>
                ${typeof actions.reveal === 'function' ? `<button type="button" class="vd-button" data-lb-reveal>${iconMarkup('folder', '▤', 'vd-viewer-action-icon', 14)}<span>${esc(t('desktop.gallery_show_in_files'))}</span></button>` : ''}`;
        }

        function setInfo(open) {
            lb.infoOpen = !!open;
            el.info.hidden = !lb.infoOpen;
            el.infoButton.setAttribute('aria-pressed', String(lb.infoOpen));
            root.classList.toggle('has-info', lb.infoOpen);
            if (lb.infoOpen) { renderInfo(); root.classList.remove('is-idle'); }
            requestAnimationFrame(() => { applyZoom(); updateZoomLabel(); });
        }

        // ── Actions ───────────────────────────────────────────────────

        async function runRename() {
            const item = current();
            if (!item || typeof actions.rename !== 'function') return;
            const updated = await actions.rename(item);
            if (lb.closed || !updated) return;
            lb.items[lb.index] = updated;
            el.name.textContent = updated.name || '';
            lb.stripDirty = true;
            renderStrip();
            if (lb.infoOpen) renderInfo();
        }

        async function runDelete() {
            const item = current();
            if (!item || typeof actions.remove !== 'function') return;
            const wasSlideshow = lb.slideshow;
            if (wasSlideshow) setSlideshow(false);
            const removed = await actions.remove(item);
            if (lb.closed) return;
            if (!removed) { if (wasSlideshow) setSlideshow(true); return; }
            if (lb.items[lb.index] === item) {
                lb.items.splice(lb.index, 1);
                if (!lb.items.length) { close(); return; }
                lb.stripDirty = true;
                show(Math.min(lb.index, lb.items.length - 1), 1);
            }
        }

        function setItems(list) {
            if (lb.closed || !Array.isArray(list)) return;
            const item = current();
            lb.items = list.slice();
            if (!lb.items.length) { close(); return; }
            const found = item ? lb.items.findIndex(entry => entry.path === item.path) : -1;
            lb.stripDirty = true;
            if (found >= 0) {
                lb.index = found;
                el.counter.textContent = lb.items.length > 1 ? t('desktop.gallery_counter', { index: lb.index + 1, total: lb.items.length }) : '';
                el.prev.hidden = lb.items.length < 2;
                el.next.hidden = lb.items.length < 2;
                renderStrip();
            } else {
                show(Math.min(lb.index, lb.items.length - 1), 1);
            }
        }

        function close() {
            if (lb.closed) return;
            lb.closed = true;
            clearTimeout(lb.timers.slideshow);
            clearTimeout(lb.timers.idle);
            document.removeEventListener('keydown', onDocumentKeydown, true);
            window.removeEventListener('resize', onResize);
            const node = media();
            if (node && typeof node.pause === 'function') { try { node.pause(); } catch (_) { /* ignore */ } }
            const finish = () => {
                root.remove();
                if (!document.querySelector('.vd-lightbox')) document.body.classList.remove('vd-lightbox-open');
            };
            if (animations) {
                root.classList.add('is-closing');
                setTimeout(finish, 180);
            } else {
                finish();
            }
            if (active === lb.api) active = null;
            if (typeof opts.onClose === 'function') opts.onClose();
            else if (lb.restoreFocus && typeof lb.restoreFocus.focus === 'function' && document.contains(lb.restoreFocus)) lb.restoreFocus.focus({ preventScroll: true });
        }

        // ── Events ────────────────────────────────────────────────────

        function onKeydown(event) {
            const target = event.target;
            if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA')) return;
            const key = event.key;
            const mod = event.ctrlKey || event.metaKey;
            const handled = () => { event.preventDefault(); event.stopPropagation(); };
            if (key === 'Escape') { handled(); close(); return; }
            if (key === 'ArrowRight' || key === 'PageDown') { handled(); step(1); return; }
            if (key === 'ArrowLeft' || key === 'PageUp') { handled(); step(-1); return; }
            if (key === 'Home') { handled(); show(0, -1); return; }
            if (key === 'End') { handled(); show(lb.items.length - 1, 1); return; }
            if (key === ' ' && !(target && target.tagName === 'VIDEO') && !(target && target.tagName === 'AUDIO') && !(target && target.tagName === 'BUTTON')) { handled(); setSlideshow(!lb.slideshow); return; }
            if (key === '+' || key === '=') { handled(); zoomTo(lb.zoom.scale * 1.25); return; }
            if (key === '-') { handled(); zoomTo(lb.zoom.scale / 1.25); return; }
            if (key === '0') { handled(); resetZoom(); return; }
            if (key === '1' && !mod) { handled(); zoomActual(); return; }
            if ((key === 'i' || key === 'I') && !mod) { handled(); setInfo(!lb.infoOpen); return; }
            if (key === 'Delete' && !readonly && typeof actions.remove === 'function') { handled(); runDelete(); return; }
            if (key === 'F2' && !readonly && typeof actions.rename === 'function') { handled(); runRename(); return; }
            if ((key === 'd' || key === 'D') && !mod && typeof actions.download === 'function') { handled(); actions.download(current()); return; }
            if (key === 'Tab') trapFocus(event);
            // The viewer is modal: keep desktop shell shortcuts from acting behind it.
            event.stopPropagation();
        }

        function trapFocus(event) {
            const focusable = Array.from(root.querySelectorAll('button:not([hidden]):not(:disabled), [tabindex="0"], video, audio')).filter(node => node.offsetParent !== null);
            if (!focusable.length) return;
            const first = focusable[0];
            const last = focusable[focusable.length - 1];
            if (event.shiftKey && (document.activeElement === first || document.activeElement === root)) { event.preventDefault(); last.focus(); }
            else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
        }

        function onDocumentKeydown(event) {
            if (lb.closed) return;
            if (root.contains(event.target)) return;
            onKeydown(event);
        }

        function onResize() {
            applyZoom();
            updateZoomLabel();
        }

        root.addEventListener('keydown', onKeydown);
        document.addEventListener('keydown', onDocumentKeydown, true);
        window.addEventListener('resize', onResize);

        root.addEventListener('click', event => {
            const target = event.target;
            if (target.closest('[data-lb-close]')) { close(); return; }
            if (target.closest('[data-lb-prev]')) { step(-1); return; }
            if (target.closest('[data-lb-next]')) { step(1); return; }
            if (target.closest('[data-lb-zoom-in]')) { zoomTo(lb.zoom.scale * 1.25); return; }
            if (target.closest('[data-lb-zoom-out]')) { zoomTo(lb.zoom.scale / 1.25); return; }
            if (target.closest('[data-lb-zoom-reset]')) { resetZoom(); return; }
            if (target.closest('[data-lb-zoom-actual]')) { zoomActual(); return; }
            if (target.closest('[data-lb-slideshow]')) { setSlideshow(!lb.slideshow); return; }
            if (target.closest('[data-lb-info]')) { setInfo(!lb.infoOpen); return; }
            if (target.closest('[data-lb-download]')) { if (typeof actions.download === 'function') actions.download(current()); return; }
            if (target.closest('[data-lb-edit]')) { const item = current(); close(); if (typeof actions.edit === 'function') actions.edit(item); return; }
            if (target.closest('[data-lb-reveal]')) { const item = current(); close(); if (typeof actions.reveal === 'function') actions.reveal(item); return; }
            if (target.closest('[data-lb-rename]')) { runRename(); return; }
            if (target.closest('[data-lb-delete]')) { runDelete(); return; }
            const thumb = target.closest('[data-lb-thumb]');
            if (thumb) { show(Number(thumb.dataset.lbThumb), Number(thumb.dataset.lbThumb) > lb.index ? 1 : -1); return; }
            if (target === el.canvas || target.closest('[data-lb-backdrop]')) {
                if (lb.suppressClick) { lb.suppressClick = false; return; }
                // The second click of a tile double-click lands here; never close on it.
                if (Date.now() - lb.openedAt < 450) return;
                close();
            }
        });

        root.addEventListener('dblclick', event => {
            const node = media();
            if (!node || !(node instanceof HTMLImageElement) || !event.target.closest('.vd-lightbox-image')) return;
            event.preventDefault();
            if (Date.now() - lb.openedAt < 450) return;
            if (lb.zoom.scale > 1.001) resetZoom();
            else zoomTo(2.5, event.clientX, event.clientY);
        });

        el.stage.addEventListener('wheel', event => {
            const node = media();
            if (!node || !(node instanceof HTMLImageElement)) return;
            if (event.target.closest('.vd-lightbox-info')) return;
            event.preventDefault();
            const factor = event.deltaY < 0 ? 1.12 : 1 / 1.12;
            zoomTo(lb.zoom.scale * factor, event.clientX, event.clientY);
        }, { passive: false });

        el.canvas.addEventListener('pointerdown', event => {
            if (event.target.closest('video, audio, button')) return;
            el.canvas.setPointerCapture(event.pointerId);
            lb.pointers.set(event.pointerId, { x: event.clientX, y: event.clientY });
            if (lb.pointers.size === 1) {
                lb.gesture = { type: 'pan', startX: event.clientX, startY: event.clientY, originX: lb.zoom.x, originY: lb.zoom.y, moved: false, time: Date.now() };
            } else if (lb.pointers.size === 2) {
                const [a, b] = Array.from(lb.pointers.values());
                lb.gesture = { type: 'pinch', distance: Math.hypot(a.x - b.x, a.y - b.y), scale: lb.zoom.scale, moved: true };
            }
            el.canvas.classList.add('is-dragging');
        });

        el.canvas.addEventListener('pointermove', event => {
            if (!lb.pointers.has(event.pointerId) || !lb.gesture) return;
            lb.pointers.set(event.pointerId, { x: event.clientX, y: event.clientY });
            if (lb.gesture.type === 'pinch' && lb.pointers.size === 2) {
                const [a, b] = Array.from(lb.pointers.values());
                const distance = Math.hypot(a.x - b.x, a.y - b.y);
                if (lb.gesture.distance > 0) zoomTo(lb.gesture.scale * (distance / lb.gesture.distance), (a.x + b.x) / 2, (a.y + b.y) / 2);
                return;
            }
            if (lb.gesture.type !== 'pan') return;
            const dx = event.clientX - lb.gesture.startX;
            const dy = event.clientY - lb.gesture.startY;
            if (Math.abs(dx) > 4 || Math.abs(dy) > 4) lb.gesture.moved = true;
            if (lb.zoom.scale > 1.001) {
                lb.zoom.x = lb.gesture.originX + dx;
                lb.zoom.y = lb.gesture.originY + dy;
                applyZoom();
            } else if (lb.items.length > 1) {
                const node = media();
                if (node) node.style.transform = `translate(${dx * 0.6}px, 0)`;
            }
        });

        function endPointer(event) {
            if (!lb.pointers.has(event.pointerId)) return;
            lb.pointers.delete(event.pointerId);
            try { el.canvas.releasePointerCapture(event.pointerId); } catch (_) { /* ignore */ }
            if (lb.pointers.size > 0) { if (lb.gesture && lb.gesture.type === 'pinch') lb.gesture = null; return; }
            el.canvas.classList.remove('is-dragging');
            const gesture = lb.gesture;
            lb.gesture = null;
            if (!gesture || gesture.type !== 'pan') { applyZoom(); return; }
            const dx = event.clientX - gesture.startX;
            const dy = event.clientY - gesture.startY;
            if (gesture.moved) lb.suppressClick = true;
            if (lb.zoom.scale <= 1.001) {
                if (lb.items.length > 1 && Math.abs(dx) > 60 && Math.abs(dx) > Math.abs(dy) * 1.5) step(dx < 0 ? 1 : -1);
                else applyZoom();
            }
        }
        el.canvas.addEventListener('pointerup', endPointer);
        el.canvas.addEventListener('pointercancel', endPointer);

        root.addEventListener('pointermove', pokeIdle, { passive: true });
        root.addEventListener('pointerdown', () => {
            pokeIdle();
            if (!root.contains(document.activeElement)) root.focus({ preventScroll: true });
        });

        // ── Boot ──────────────────────────────────────────────────────

        lb.api = { close, setItems, next: () => step(1), prev: () => step(-1), show: index => show(index, 0), isOpen: () => !lb.closed, root };
        active = lb.api;
        show(lb.index, 0);
        requestAnimationFrame(() => { root.classList.add('is-open'); root.focus({ preventScroll: true }); pokeIdle(); });
        return lb.api;
    }

    function close() {
        if (active) active.close();
    }

    function isOpen() {
        return !!active;
    }

    window.GalleryLightbox = { open, close, isOpen, kindOf };
})();
