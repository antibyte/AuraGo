/* ───────────────────────────────────────────────────────────────
 * Camera App — creative controls
 * Looks, aspect ratio, digital zoom, WYSIWYG crop mask and the recording
 * pause button. The pure framing maths is exported for Node-side checks;
 * camera.js owns the stream, capture and the desktop handoffs.
 * ─────────────────────────────────────────────────────────────── */
(function () {
    'use strict';

    var LOOKS = [
        { id: 'natural', filter: 'none', labelKey: 'desktop.camera_look_natural' },
        { id: 'mono', filter: 'grayscale(1) contrast(1.06)', labelKey: 'desktop.camera_look_mono' },
        { id: 'warm', filter: 'sepia(0.22) saturate(1.28) contrast(1.04)', labelKey: 'desktop.camera_look_warm' },
        { id: 'cool', filter: 'saturate(0.82) brightness(1.04) hue-rotate(-12deg)', labelKey: 'desktop.camera_look_cool' },
        { id: 'vintage', filter: 'sepia(0.38) saturate(0.82) contrast(0.92) brightness(1.05)', labelKey: 'desktop.camera_look_vintage' }
    ];

    var RATIOS = [
        { id: 'auto', value: 0, label: '', labelKey: 'desktop.camera_ratio_auto' },
        { id: '1:1', value: 1, label: '1:1' },
        { id: '4:3', value: 4 / 3, label: '4:3' },
        { id: '16:9', value: 16 / 9, label: '16:9' }
    ];

    var ZOOM_STEPS = [1, 1.25, 1.5, 1.75, 2];
    var panelSeq = 0;

    /* The live video uses object-fit: cover. Translate a viewport into the
     * exact source rectangle that is visible, then apply zoom and the chosen
     * ratio inside that region. Mask percentages are relative to the zoomed
     * region so the on-screen frame and the captured JPEG always agree. */
    function sourceRect(video, viewport, ratioId, zoom) {
        var vw = Number(video && video.videoWidth) || 0;
        var vh = Number(video && video.videoHeight) || 0;
        if (vw <= 0 || vh <= 0) return null;
        var cw = Number(viewport && viewport.clientWidth) || vw;
        var ch = Number(viewport && viewport.clientHeight) || vh;
        var scale = Math.max(cw / vw, ch / vh);
        var visW = Math.min(vw, cw / scale);
        var visH = Math.min(vh, ch / scale);
        var visX = (vw - visW) / 2;
        var visY = (vh - visH) / 2;
        var z = Math.max(1, Number(zoom) || 1);
        var zw = visW / z;
        var zh = visH / z;
        var zx = visX + (visW - zw) / 2;
        var zy = visY + (visH - zh) / 2;
        var target = 0;
        for (var i = 0; i < RATIOS.length; i++) {
            if (RATIOS[i].id === ratioId) target = RATIOS[i].value;
        }
        var w = zw;
        var h = zh;
        if (target > 0) {
            if (w / h > target) w = h * target;
            else h = w / target;
        }
        var x = zx + (zw - w) / 2;
        var y = zy + (zh - h) / 2;
        return {
            x: x, y: y, w: w, h: h,
            mask: {
                left: (x - zx) / zw * 100,
                top: (y - zy) / zh * 100,
                width: w / zw * 100,
                height: h / zh * 100
            }
        };
    }

    function canvasFilterSupported() {
        try {
            var ctx = document.createElement('canvas').getContext('2d');
            return !!ctx && typeof ctx.filter === 'string';
        } catch (err) {
            return false;
        }
    }

    function formatZoom(value) {
        try {
            var lang = (typeof navigator !== 'undefined' && navigator.language) || 'de';
            return new Intl.NumberFormat(lang, { maximumFractionDigits: 2 }).format(value) + '\u00d7';
        } catch (err) {
            return String(value) + '\u00d7';
        }
    }

    function panelMarkup(t, esc, icon, id, filterOK) {
        var looks = LOOKS.map(function (look) {
            return '<button type="button" class="camera-cc-look" data-cc-look="' + esc(look.id) + '" aria-pressed="false">' +
                '<span class="camera-cc-swatch" style="filter:' + esc(look.filter) + '" aria-hidden="true"></span>' +
                '<span>' + esc(t(look.labelKey)) + '</span></button>';
        }).join('');
        var ratios = RATIOS.map(function (ratio) {
            return '<button type="button" data-cc-ratio="' + esc(ratio.id) + '" aria-pressed="false">' +
                esc(ratio.labelKey ? t(ratio.labelKey) : ratio.label) + '</button>';
        }).join('');
        return '<div class="camera-cc-panel" data-cc-panel id="' + esc(id) + '" role="group" aria-label="' + esc(t('desktop.camera_tools')) + '" hidden>' +
            '<div class="camera-cc-head"><strong>' + esc(t('desktop.camera_tools')) + '</strong>' +
            '<span>' + esc(t('desktop.camera_photo_only')) + '</span></div>' +
            '<div class="camera-cc-section">' +
                '<span class="camera-cc-label">' + esc(t('desktop.camera_look')) + '</span>' +
                '<div class="camera-cc-looks">' + looks + '</div>' +
                '<p class="camera-cc-note" data-cc-note' + (filterOK ? ' hidden' : '') + '>' + esc(t('desktop.camera_look_unavailable')) + '</p>' +
            '</div>' +
            '<div class="camera-cc-row">' +
                '<div class="camera-cc-section">' +
                    '<span class="camera-cc-label">' + esc(t('desktop.camera_zoom')) + '</span>' +
                    '<div class="camera-cc-stepper">' +
                        '<button type="button" data-cc-zoom="-1" aria-label="' + esc(t('desktop.camera_zoom_out')) + '">' + icon('zoom-out', '\u2212', 'camera-btn-icon', 16) + '</button>' +
                        '<output data-cc-zoom-value>1\u00d7</output>' +
                        '<button type="button" data-cc-zoom="1" aria-label="' + esc(t('desktop.camera_zoom_in')) + '">' + icon('zoom-in', '+', 'camera-btn-icon', 16) + '</button>' +
                    '</div>' +
                '</div>' +
                '<div class="camera-cc-section">' +
                    '<span class="camera-cc-label">' + esc(t('desktop.camera_ratio')) + '</span>' +
                    '<div class="camera-cc-ratios">' + ratios + '</div>' +
                '</div>' +
            '</div>' +
        '</div>';
    }

    function create(options) {
        var o = options || {};
        var t = o.t || function (key) { return key; };
        var esc = o.esc || function (value) { return String(value == null ? '' : value); };
        var icon = o.iconMarkup || function (_key, fallback) { return '<span>' + esc(fallback || '') + '</span>'; };
        var notify = typeof o.notify === 'function' ? o.notify : function () {};
        var video = o.video;
        var viewport = o.viewport;
        var dock = o.dock;
        var toolsHost = o.toolsHost;
        var pauseHost = o.pauseHost;
        if (!video || !viewport || !dock || !toolsHost || !pauseHost) return null;

        var panelId = 'camera-cc-' + (++panelSeq);
        var filterOK = canvasFilterSupported();
        var liveState = {
            look: 0, ratio: 'auto', zoom: 0, open: false,
            mode: 'photo', preview: false, recording: false, paused: false,
            locked: false, mirror: false, pauseSupported: false
        };

        toolsHost.insertAdjacentHTML('beforeend',
            '<button class="camera-chip camera-cc-toggle" type="button" data-action="tools" aria-expanded="false" aria-pressed="false" aria-controls="' + esc(panelId) + '" title="' + esc(t('desktop.camera_tools')) + '" aria-label="' + esc(t('desktop.camera_tools')) + '">' +
            icon('settings-2', 'T', 'camera-btn-icon', 16) + '</button>');
        pauseHost.insertAdjacentHTML('beforeend',
            '<button class="camera-chip camera-cc-pause" type="button" data-action="pause" aria-pressed="false" hidden>' +
            '<span class="camera-cc-pause-glyph" aria-hidden="true"></span></button>');
        dock.insertAdjacentHTML('beforeend', panelMarkup(t, esc, icon, panelId, filterOK));
        viewport.insertAdjacentHTML('beforeend', '<div class="camera-cc-mask" data-cc-mask hidden aria-hidden="true"></div>');

        var toggleBtn = toolsHost.querySelector('[data-action="tools"]');
        var pauseBtn = pauseHost.querySelector('[data-action="pause"]');
        var panel = dock.querySelector('[data-cc-panel]');
        var mask = viewport.querySelector('[data-cc-mask]');
        var lookBtns = panel.querySelectorAll('[data-cc-look]');
        var ratioBtns = panel.querySelectorAll('[data-cc-ratio]');
        var zoomBtns = panel.querySelectorAll('[data-cc-zoom]');
        var zoomValue = panel.querySelector('[data-cc-zoom-value]');

        function isLive() {
            return liveState.mode === 'photo' && !liveState.preview && !liveState.recording;
        }

        function applyMask() {
            var rect = sourceRect(video, viewport, liveState.ratio, ZOOM_STEPS[liveState.zoom]);
            if (!rect) { mask.hidden = true; return; }
            mask.style.left = rect.mask.left + '%';
            mask.style.top = rect.mask.top + '%';
            mask.style.width = rect.mask.width + '%';
            mask.style.height = rect.mask.height + '%';
        }

        function paint() {
            var live = isLive();
            if (!live) liveState.open = false;
            var zoom = ZOOM_STEPS[liveState.zoom];
            toggleBtn.hidden = !live;
            toggleBtn.classList.toggle('is-active', liveState.open);
            toggleBtn.setAttribute('aria-pressed', String(liveState.open));
            toggleBtn.setAttribute('aria-expanded', String(liveState.open));
            panel.hidden = !(live && liveState.open);

            pauseBtn.hidden = !liveState.recording;
            pauseBtn.disabled = !liveState.pauseSupported;
            pauseBtn.classList.toggle('is-paused', liveState.paused);
            pauseBtn.setAttribute('aria-pressed', String(liveState.paused));
            pauseBtn.setAttribute('aria-label', t(liveState.paused ? 'desktop.camera_resume' : 'desktop.camera_pause'));
            pauseBtn.title = liveState.pauseSupported
                ? t(liveState.paused ? 'desktop.camera_resume' : 'desktop.camera_pause')
                : t('desktop.camera_pause_unsupported');

            Array.prototype.forEach.call(lookBtns, function (btn, index) {
                btn.disabled = !filterOK;
                btn.setAttribute('aria-pressed', String(index === liveState.look));
            });
            Array.prototype.forEach.call(ratioBtns, function (btn) {
                btn.setAttribute('aria-pressed', String(btn.dataset.ccRatio === liveState.ratio));
            });
            zoomValue.textContent = formatZoom(zoom);
            zoomBtns[0].disabled = liveState.zoom === 0;
            zoomBtns[1].disabled = liveState.zoom === ZOOM_STEPS.length - 1;

            video.style.filter = filterOK && live ? LOOKS[liveState.look].filter : 'none';
            var transform = (liveState.mirror ? 'scaleX(-1) ' : '') + (live && zoom > 1 ? 'scale(' + zoom + ')' : '');
            video.style.transform = transform || 'none';

            mask.hidden = !(live && liveState.ratio !== 'auto');
            if (!mask.hidden) applyMask();
        }

        function changed() {
            paint();
            if (typeof o.onChange === 'function') o.onChange();
        }

        function close() {
            if (!liveState.open) return;
            liveState.open = false;
            paint();
        }

        function toggle() {
            if (!isLive()) return;
            liveState.open = !liveState.open;
            paint();
        }

        function cycleLook() {
            if (!isLive()) return;
            if (!filterOK) { notify(t('desktop.camera_look_unavailable')); return; }
            liveState.look = (liveState.look + 1) % LOOKS.length;
            changed();
        }

        function cycleRatio() {
            if (!isLive()) return;
            var index = 0;
            for (var i = 0; i < RATIOS.length; i++) {
                if (RATIOS[i].id === liveState.ratio) index = i;
            }
            liveState.ratio = RATIOS[(index + 1) % RATIOS.length].id;
            changed();
        }

        function zoomBy(delta) {
            if (!isLive()) return;
            var next = Math.max(0, Math.min(ZOOM_STEPS.length - 1, liveState.zoom + delta));
            if (next === liveState.zoom) return;
            liveState.zoom = next;
            changed();
        }

        function onPanelClick(event) {
            var target = event.target;
            if (!target || typeof target.closest !== 'function') return;
            var lookBtn = target.closest('[data-cc-look]');
            var ratioBtn = target.closest('[data-cc-ratio]');
            var zoomBtn = target.closest('[data-cc-zoom]');
            if (lookBtn) {
                if (!filterOK) { notify(t('desktop.camera_look_unavailable')); return; }
                for (var i = 0; i < LOOKS.length; i++) {
                    if (LOOKS[i].id === lookBtn.dataset.ccLook) liveState.look = i;
                }
                changed();
            } else if (ratioBtn) {
                liveState.ratio = ratioBtn.dataset.ccRatio;
                changed();
            } else if (zoomBtn) {
                zoomBy(Number(zoomBtn.dataset.ccZoom));
            }
        }

        function onOutside(event) {
            if (!liveState.open) return;
            var target = event.target;
            if (!target || typeof target.closest !== 'function') return;
            if (target.closest('[data-cc-panel]') || target.closest('[data-action="tools"]')) return;
            close();
        }

        var observer = null;
        if (typeof ResizeObserver === 'function') {
            observer = new ResizeObserver(function () {
                if (!mask.hidden) applyMask();
            });
            observer.observe(viewport);
        }

        toggleBtn.addEventListener('click', toggle);
        panel.addEventListener('click', onPanelClick);
        pauseBtn.addEventListener('click', function () {
            if (typeof o.onPause === 'function') o.onPause();
        });
        document.addEventListener('pointerdown', onOutside, true);

        paint();

        return {
            isOpen: function () { return liveState.open; },
            close: close,
            toggle: toggle,
            cycleLook: cycleLook,
            cycleRatio: cycleRatio,
            zoomBy: zoomBy,
            setContext: function (next) {
                for (var key in next) {
                    if (Object.prototype.hasOwnProperty.call(next, key)) liveState[key] = next[key];
                }
                paint();
            },
            lookFilter: function () { return filterOK ? LOOKS[liveState.look].filter : 'none'; },
            sourceRect: function () {
                return sourceRect(video, viewport, liveState.ratio, ZOOM_STEPS[liveState.zoom]);
            },
            hintPairs: function (preview, recording) {
                if (preview) return [];
                if (recording) return liveState.pauseSupported ? [['P', t('desktop.camera_pause')]] : [];
                return [['T', t('desktop.camera_hint_tools')], ['L', t('desktop.camera_look')]];
            },
            menuItems: function () {
                var live = isLive();
                return [
                    { id: 'look', labelKey: 'desktop.camera_look', icon: 'image', disabled: !live || !filterOK, action: cycleLook },
                    { id: 'ratio', labelKey: 'desktop.camera_ratio', icon: 'grid', disabled: !live, action: cycleRatio }
                ];
            },
            dispose: function () {
                toggleBtn.removeEventListener('click', toggle);
                panel.removeEventListener('click', onPanelClick);
                document.removeEventListener('pointerdown', onOutside, true);
                if (observer) observer.disconnect();
                video.style.filter = '';
                video.style.transform = '';
                toggleBtn.remove();
                pauseBtn.remove();
                panel.remove();
                mask.remove();
            }
        };
    }

    window.CameraCreative = {
        LOOKS: LOOKS,
        RATIOS: RATIOS,
        ZOOM_STEPS: ZOOM_STEPS,
        sourceRect: sourceRect,
        create: create
    };
})();
