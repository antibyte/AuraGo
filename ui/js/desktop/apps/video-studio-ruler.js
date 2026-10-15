(function () {
    'use strict';

    // Ruler decorations (in/out shade, marker flags and ranges), the overview strip with its gestures (spec §4.5) and
    // the marker drags (spec §4.1, §4.3). Clip gestures live in video-studio-timeline.js, which keeps the drag state.
    // Lane positions: the track headers stick over the left of the scroll area, so the first visible lane px is the
    // scroll area's scrollLeft and the visible lane width is its clientWidth minus the header width.
    const Time = () => window.VideoStudioTime;
    const Nav = () => window.VideoStudioNavigation;
    const Timeline = () => window.VideoStudioTimeline;
    const Edit = () => window.VideoStudioEdit;
    const clone = value => JSON.parse(JSON.stringify(value));
    const clamp = (value, lo, hi) => Math.max(lo, Math.min(hi, value));
    // Markers stay sorted by frame, then id (the marker commands use the same order).
    const byFrame = (a, b) => a.frame - b.frame || (a.id < b.id ? -1 : a.id > b.id ? 1 : 0);

    function inOutOf(project) {
        return { in: Number.isFinite(project.in_point) ? project.in_point : null, out: Number.isFinite(project.out_point) ? project.out_point : null };
    }
    function colorOf(marker) { return Timeline().LABEL_COLORS.includes(marker.color) ? marker.color : ''; }
    function colorStyle(marker) { const color = colorOf(marker); return color ? `;--vs-marker-color:var(--vs-label-${color})` : ''; }

    // Ruler decorations for `pps` px per second; `width` is the lane width in px.
    function markup(options, project, pps, width) {
        const fps = Time().rate(project), esc = options.esc, io = inOutOf(project), x = frame => frame / fps * pps;
        let html = '';
        if (io.in != null || io.out != null) {
            const start = io.in == null ? 0 : io.in, end = io.out == null ? width / pps * fps : io.out;
            html += `<span class="vs-range-shade" style="left:${x(start)}px;width:${Math.max(0, x(end) - x(start))}px" aria-hidden="true"></span>`;
        }
        (project.markers || []).forEach(marker => {
            const name = marker.name || options.label('marker', 'Marker'), title = `${name} · ${Time().shortClock(marker.frame, project)}`;
            const span = marker.duration > 0 ? `<span class="vs-marker-span" style="width:${Math.max(2, x(marker.duration))}px"></span>` : '';
            html += `<button type="button" class="vs-marker${marker.duration > 0 ? ' is-range' : ''}" data-marker-id="${esc(marker.id)}" style="left:${x(marker.frame)}px${colorStyle(marker)}" title="${esc(title)}" aria-label="${esc(title)}">${span}</button>`;
        });
        return html;
    }
    // Overview marks in percent of the lane width.
    function overviewInner(options, project, pps, width) {
        const fps = Time().rate(project), io = inOutOf(project), pct = frame => (width > 0 ? frame / fps * pps / width * 100 : 0);
        let html = '';
        if (io.in != null || io.out != null) {
            const start = io.in == null ? 0 : io.in, end = io.out == null ? width / pps * fps : io.out;
            html += `<span class="vs-overview-shade" style="left:${pct(start)}%;width:${Math.max(0, pct(end) - pct(start))}%"></span>`;
        }
        (project.markers || []).forEach(marker => { html += `<span class="vs-overview-mark" style="left:${pct(marker.frame)}%${colorStyle(marker)}"></span>`; });
        return html;
    }
    // Places the visible window from the live scroll position (after every render, scroll and resize).
    function syncOverview(root) {
        const strip = root.querySelector('[data-overview]'), scroll = root.querySelector('.vs-timeline-scroll'), head = root.querySelector('.vs-ruler-label');
        const win = strip && strip.querySelector('.vs-overview-window');
        if (!win || !scroll || !head) return;
        const box = Nav().overviewBox(Number(strip.dataset.laneWidth) || 0, scroll.scrollLeft, Math.max(1, scroll.clientWidth - head.offsetWidth), strip.clientWidth);
        win.style.left = box.left + 'px';
        win.style.width = box.width + 'px';
    }
    // Overview gestures: drag inside the window to pan; drag an edge to zoom with the opposite edge fixed; click
    // elsewhere on the strip to centre the window there. options.zoomToSpan(seconds, anchorSeconds, anchorOffset) is the
    // timeline's zoom. Returns a detach function.
    function attachOverview(root, options) {
        let gesture = null;
        const base = () => Timeline().BASE_PX_PER_SECOND;
        const liveScroll = () => root.querySelector('.vs-timeline-scroll');
        const parts = () => {
            const strip = root.querySelector('[data-overview]'), scroll = liveScroll(), head = root.querySelector('.vs-ruler-label');
            if (!strip || !scroll || !head) return null;
            return {
                strip, scroll, lane: Number(strip.dataset.laneWidth) || 0, stripWidth: strip.clientWidth,
                visible: Math.max(1, scroll.clientWidth - head.offsetWidth), pxPerSecond: base() * (options.state.zoom || 1), scrollStart: scroll.scrollLeft
            };
        };
        function down(event) {
            if (event.button !== 0 || !event.target.closest || !event.target.closest('[data-overview]')) return;
            const p = parts();
            if (!p || !p.stripWidth || !p.lane) return;
            const edge = event.target.closest('[data-overview-edge]'), inWindow = event.target.closest('.vs-overview-window');
            if (!edge && !inWindow) {
                const x = (event.clientX - p.strip.getBoundingClientRect().left) / p.stripWidth * p.lane;
                p.scroll.scrollLeft = clamp(x - p.visible / 2, 0, Math.max(0, p.scroll.scrollWidth - p.scroll.clientWidth));
                return;
            }
            gesture = Object.assign(p, { mode: edge ? 'zoom' : 'pan', edge: edge ? edge.dataset.overviewEdge : '', x: event.clientX });
            event.preventDefault();
            try { p.strip.setPointerCapture(event.pointerId); } catch (_) { /* synthetic pointer */ }
        }
        function move(event) {
            if (!gesture) return;
            const g = gesture, dx = event.clientX - g.x;
            // A scroll can re-render the timeline (it draws the clips that came into view), so pan the live scroll area.
            if (g.mode === 'pan') { const scroll = liveScroll(); if (scroll) scroll.scrollLeft = g.scrollStart + dx * g.lane / g.stripWidth; return; }
            // The window is g.visible px wide in the lane; on the strip it is windowStrip px wide. The dragged edge moves
            // it; the opposite edge is the zoom anchor (its time stays under the same position).
            const windowStrip = g.visible * g.stripWidth / g.lane;
            const nextStrip = Math.max(12, Math.min(g.stripWidth, windowStrip + (g.edge === 'end' ? dx : -dx)));
            const seconds = g.visible / g.pxPerSecond * nextStrip / windowStrip;
            const anchorLane = g.edge === 'end' ? g.scrollStart : g.scrollStart + g.visible;
            const offset = g.edge === 'end' ? 0 : g.visible;
            options.zoomToSpan(seconds, anchorLane / g.pxPerSecond, offset);
        }
        function up() { gesture = null; }
        root.addEventListener('pointerdown', down);
        window.addEventListener('pointermove', move);
        window.addEventListener('pointerup', up);
        window.addEventListener('pointercancel', up);
        return function detach() {
            gesture = null;
            root.removeEventListener('pointerdown', down);
            window.removeEventListener('pointermove', move);
            window.removeEventListener('pointerup', up);
            window.removeEventListener('pointercancel', up);
        };
    }

    // Marker drags (spec §4.1, §4.3). markerDrag starts the drag from a flag; markerMove moves the flag to the pointer's
    // frame, snapping its start (and its end for a range) to clip edges, in/out, the playhead and the other markers
    // (host.options.snapExtras leaves the dragged marker out; the points are taken when the drag starts).
    // host = { state, options, pps, snapFrames, render }.
    function markerDrag(host, event, markerId) {
        const project = host.state.project, marker = (project.markers || []).find(item => item.id === markerId);
        if (!marker) return null;
        const extra = host.options.snapExtras ? host.options.snapExtras(markerId) : [host.state.frame];
        return { type: 'marker', markerId, clipId: '', x: event.clientX, before: clone(project), extra, frame0: marker.frame, duration: marker.duration || 0, epoch: host.state.projectEpoch, projectId: host.state.projectId, moved: false, invalid: false, reason: '' };
    }
    function markerMove(host, drag, event) {
        const s = host.state, fps = Time().rate(s.project), max = Time().maxFrames(s.project);
        const delta = Math.round((event.clientX - drag.x) / host.pps * fps);
        const points = Edit().snapPoints(drag.before, [], drag.extra);
        const edges = drag.duration > 0 ? [drag.frame0, drag.frame0 + drag.duration] : [drag.frame0];
        const adjusted = Math.round(Edit().snapDelta(edges, delta, points, host.snapFrames));
        const frame = clamp(drag.frame0 + adjusted, 0, max - drag.duration);
        const next = clone(drag.before);
        next.markers.find(item => item.id === drag.markerId).frame = frame;
        next.markers.sort(byFrame);
        s.project = next;
        drag.moved = drag.moved || frame !== drag.frame0;
        host.render();
    }

    window.VideoStudioRuler = { markup, overviewInner, syncOverview, attachOverview, markerDrag, markerMove };
})();
