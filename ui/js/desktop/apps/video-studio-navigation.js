(function () {
    'use strict';

    // DOM-free navigation helpers (spec §4.5, §4.6): the JKL shuttle controller, edit points, marker steps, loop ranges,
    // overview geometry, zoom spans and track order. The app parts and the timeline call them; Node checks cover them.
    const MAX_RATE = 8;
    const MAX_MARKERS = 500;
    const clamp = (value, lo, hi) => Math.max(lo, Math.min(hi, value));

    // JKL. L plays forward; each further press doubles the rate (1, 2, 4, 8x). J does the same backward; a direction
    // change restarts at 1x. K stops and returns the rate to 1x. K held with J or L steps one frame. The caller ignores
    // auto-repeat. options.maxFrame() returns the current end of the timeline; setRate receives signed rates.
    function createShuttle(options) {
        const { getFrame, seek, play, pause, setRate, maxFrame } = options;
        let rate = 0, kHeld = false;
        const bounded = frame => clamp(Math.round(frame), 0, Math.max(0, maxFrame()));
        function stop() { rate = 0; setRate(1); pause(); return 0; }
        function step(direction) { rate = 0; pause(); seek(bounded(getFrame() + direction)); return 'step'; }
        function shuttle(direction) {
            const next = rate * direction > 0 ? Math.min(MAX_RATE, Math.abs(rate) * 2) : 1;
            rate = direction * next;
            setRate(rate);
            play();
            return rate;
        }
        return {
            press(key) {
                if (key === 'k') { kHeld = true; return stop(); }
                if (key === 'j' || key === 'l') {
                    const direction = key === 'l' ? 1 : -1;
                    return kHeld ? step(direction) : shuttle(direction);
                }
                return null;
            },
            release(key) { if (key === 'k') kHeld = false; },
            // Called when playback stops by any other means: the next J or L starts again at 1x. It does not call the preview.
            reset() { rate = 0; },
            stop,
            rate: () => rate
        };
    }

    // Edit points on unlocked, visible tracks: frame 0 and every clip edge. direction -1 finds the nearest point before
    // the frame, 1 the nearest after it; null when there is none.
    function editPoints(project, frame, direction) {
        const points = new Set([0]);
        (project.tracks || []).forEach(track => {
            if (track.locked || track.hidden) return;
            (track.clips || []).forEach(clip => { points.add(clip.start); points.add(clip.start + clip.duration); });
        });
        const list = Array.from(points).filter(point => (direction < 0 ? point < frame : point > frame));
        if (!list.length) return null;
        return direction < 0 ? Math.max(...list) : Math.min(...list);
    }
    // Marker steps: the nearest marker frame before (-1) or after (1) `frame`, or null.
    function markerStep(frames, frame, direction) {
        const list = (frames || []).filter(Number.isFinite).filter(value => (direction < 0 ? value < frame : value > frame));
        if (!list.length) return null;
        return direction < 0 ? Math.max(...list) : Math.min(...list);
    }
    // Loop range from the in and out points: a missing point stands for the start or the end of the timeline. Null when
    // the range is empty (the caller then loops the whole timeline).
    function loopRange(io, end) {
        const start = io && io.in != null ? io.in : 0, stop = io && io.out != null ? io.out : end;
        return stop > start ? { start, end: stop } : null;
    }
    // Overview strip: the whole timeline (contentWidth px) is drawn at stripWidth px; the visible window is the part in
    // view. Returns the window's {left, width} in strip px; the window is at least 12 px wide and stays on the strip.
    function overviewBox(contentWidth, scrollLeft, visibleWidth, stripWidth) {
        if (!(contentWidth > 0) || !(stripWidth > 0)) return { left: 0, width: 0 };
        const k = stripWidth / contentWidth, width = clamp(visibleWidth * k, Math.min(12, stripWidth), stripWidth);
        return { left: clamp(scrollLeft * k, 0, stripWidth - width), width };
    }
    // Lane position (px) for a strip position: the first visible lane px when the window is centred or panned there.
    function laneForStrip(stripX, contentWidth, stripWidth, visibleWidth) {
        return clamp(stripX * contentWidth / stripWidth, 0, Math.max(0, contentWidth - visibleWidth));
    }
    // The zoom that fits `seconds` of timeline into visibleWidth px. limits: { min, max, basePx } (the timeline constants).
    function zoomForSpan(seconds, visibleWidth, limits) {
        const span = Math.max(1e-6, Number(seconds) || 0);
        return clamp(visibleWidth / (span * limits.basePx), limits.min, limits.max);
    }
    // Array order is paint order. Visual tracks (video and overlay) list front-most first, so "above" is the array position
    // after the reference; audio tracks list in array order, so "above" is before it. This is the only place that encodes it.
    function arraySideFor(kind, displaySide) {
        return (kind !== 'audio') === (displaySide === 'above') ? 'after' : 'before';
    }
    function trackInsertIndex(tracks, referenceId, displaySide) {
        const list = tracks || [], index = list.findIndex(track => track.id === referenceId);
        if (index < 0) return list.length;
        return arraySideFor(list[index].kind, displaySide) === 'after' ? index + 1 : index;
    }
    // Moves a track next to another track of the same kind (arraySide 'before' or 'after' the target). Returns the new
    // array, or null when the kinds differ or nothing moves.
    function moveWithinKind(tracks, fromId, toId, arraySide) {
        const list = (tracks || []).slice(), from = list.findIndex(track => track.id === fromId), to = list.findIndex(track => track.id === toId);
        if (from < 0 || to < 0 || from === to || list[from].kind !== list[to].kind) return null;
        const [moved] = list.splice(from, 1);
        const target = list.findIndex(track => track.id === toId);
        list.splice(arraySide === 'after' ? target + 1 : target, 0, moved);
        return list;
    }

    window.VideoStudioNavigation = { MAX_RATE, MAX_MARKERS, createShuttle, editPoints, markerStep, loopRange, overviewBox, laneForStrip, zoomForSpan, arraySideFor, trackInsertIndex, moveWithinKind };
})();
