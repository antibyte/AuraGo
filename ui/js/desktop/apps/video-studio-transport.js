(function () {
    'use strict';

    // Transport and navigation (spec §4.5): the JKL shuttle, play and pause, in and out points, loop, edit-point and
    // marker jumps, go to timecode, and the navigation key table. Project changes go through mutate (one undo step).
    const P = window.VideoStudioParts;
    const T = (...args) => P.T(...args);
    const I = (...args) => P.I(...args);
    const esc = (...args) => P.esc(...args);
    const icon = (...args) => P.icon(...args);
    const maxFrames = (...args) => P.maxFrames(...args);
    const mutate = (...args) => P.mutate(...args);
    const openModal = (...args) => P.openModal(...args);
    const projectDuration = (...args) => P.projectDuration(...args);
    const showNotice = (...args) => P.showNotice(...args);
    const tr = (...args) => P.tr(...args);
    const Nav = () => window.VideoStudioNavigation;
    const label = (s, key, fallback) => esc(s, tr(s, key, fallback));
    const readonlyNotice = s => showNotice(s, 'readonly', 'Read-only mode prevents editing.', true);

    // In and out points of the project (null when unset).
    function inOut(s) {
        const project = s.project || {};
        return { in: Number.isFinite(project.in_point) ? project.in_point : null, out: Number.isFinite(project.out_point) ? project.out_point : null };
    }
    function clampFrame(s, frame) { return Math.max(0, Math.min(maxFrames(s), Math.round(frame))); }
    function revealAt(s, frame) { T().revealFrame(s.q('[data-timeline]'), s, frame); }
    // Pauses, moves the playhead and keeps it in view.
    function seekTo(s, frame) {
        const target = clampFrame(s, frame);
        s.preview.pause();
        s.preview.seek(target);
        revealAt(s, target);
    }
    // Space and the play button: pause when playing, otherwise play (the preview's pause already returned to 1x).
    function togglePlay(s) {
        if (!s.preview) return;
        if (s.preview.isPlaying()) s.preview.pause(); else s.preview.play();
    }
    function createShuttle(options) { return Nav().createShuttle(options); }
    // The loop plays in → out; a missing point stands for the timeline start or end; an empty range loops the whole timeline.
    function loopRange(s) {
        if (!s.loop || !s.project) return null;
        const end = Math.max(1, projectDuration(s));
        return Nav().loopRange(inOut(s), end) || { start: 0, end };
    }
    // Soloed audio tracks (session state; never saved).
    function soloTrackIds(s) {
        return (s.project && s.project.tracks || []).filter(track => track.kind === 'audio' && s.soloIds && s.soloIds.has(track.id)).map(track => track.id);
    }
    // Keeps the preview in step with the state (loop range, solo) and shows the loop button's state. Called on every render.
    function syncTransport(s) {
        if (!s.preview || !s.project) return;
        s.preview.setLoop(loopRange(s));
        s.preview.setSolo(soloTrackIds(s));
        const loop = s.q('[data-action="loop"]');
        if (loop) loop.setAttribute('aria-pressed', String(!!s.loop));
        if (P.syncMarkerPopover) P.syncMarkerPopover(s);
    }
    function toggleLoop(s) { s.loop = !s.loop; syncTransport(s); }

    // I and O set the point at the playhead. A point that would break in < out is refused (nothing moves).
    function setInOut(s, which) {
        if (s.readonly) return readonlyNotice(s);
        if (!s.project) return false;
        const frame = clampFrame(s, s.frame), max = maxFrames(s), io = inOut(s);
        const valid = which === 'in' ? frame < max && (io.out == null || frame < io.out) : frame >= 1 && (io.in == null || frame > io.in);
        if (!valid) return showNotice(s, 'inOutOrder', 'The in point must stay before the out point.', true);
        return mutate(s, which === 'in' ? 'in point' : 'out point', project => { project[which === 'in' ? 'in_point' : 'out_point'] = frame; });
    }
    // Alt+X clears both points (one undo step).
    function clearInOut(s) {
        if (s.readonly) return readonlyNotice(s);
        const io = inOut(s);
        if (io.in == null && io.out == null) return false;
        return mutate(s, 'clear in and out', project => { delete project.in_point; delete project.out_point; });
    }
    function goToInOut(s, which) {
        const frame = inOut(s)[which];
        if (frame != null) seekTo(s, frame);
    }
    // Ctrl+G and a click on the time display: a dialog takes 4,5, 1:04.5 or 1:02:03.10.
    function openGoTo(s) {
        if (!s.project) return;
        const id = esc(s, s.id);
        const host = openModal(s, `<form class="vs-modal" data-goto-form role="dialog" aria-modal="true" aria-labelledby="vs-goto-title-${id}"><button type="button" class="vs-modal-close" data-modal-close aria-label="${label(s, 'close', 'Close')}">${icon('close', 18)}</button><h2 id="vs-goto-title-${id}">${label(s, 'goToTitle', 'Go to timecode')}</h2><label class="vs-field vs-field-wide"><span>${label(s, 'goToLabel', 'Timecode')}</span><input type="text" inputmode="decimal" spellcheck="false" data-goto-input value="${esc(s, I().formatTime(s.frame, s.project))}"></label><p class="vs-hint">${label(s, 'timeHint', 'Enter seconds (4,5) or minutes:seconds (1:04.5).')}</p><div class="vs-modal-actions"><button type="button" data-modal-close>${label(s, 'cancel', 'Cancel')}</button><button type="submit" class="vs-primary">${label(s, 'goToAction', 'Go')}</button></div></form>`);
        const input = host.querySelector('[data-goto-input]');
        if (input) input.select();
        host.querySelector('[data-goto-form]').addEventListener('submit', event => {
            event.preventDefault();
            const frame = I().parseTime(host.querySelector('[data-goto-input]').value, s.project);
            host.replaceChildren();
            s.app.focus({ preventScroll: true });
            if (frame == null) return showNotice(s, 'invalidTime', 'Enter a time like 4,5 or 1:04.5.', true);
            seekTo(s, frame);
        });
    }
    // ↑ and ↓: the previous or next edit point on unlocked, visible tracks.
    function stepEditPoint(s, direction) {
        const frame = Nav().editPoints(s.project, s.frame, direction);
        if (frame != null) seekTo(s, frame);
    }
    // Shift+M: the next marker; Ctrl+Shift+M: the previous one.
    function stepMarker(s, direction) {
        const frame = Nav().markerStep((s.project && s.project.markers || []).map(marker => marker.frame), s.frame, direction);
        if (frame != null) seekTo(s, frame);
    }
    // Snap points besides clip edges: the playhead, the in and out points, and every marker frame and range end.
    // excludeMarkerId leaves one marker out (a marker never snaps to itself).
    function snapExtras(s, excludeMarkerId) {
        const io = inOut(s), extra = [s.frame];
        if (io.in != null) extra.push(io.in);
        if (io.out != null) extra.push(io.out);
        (s.project && s.project.markers || []).forEach(marker => {
            if (marker.id === excludeMarkerId) return;
            extra.push(marker.frame);
            if (marker.duration > 0) extra.push(marker.frame + marker.duration);
        });
        return extra;
    }
    // M adds a marker at the playhead (or opens the one there); the markers part provides it.
    function addMarkerAtPlayhead(s) { if (P.addMarkerAtPlayhead) P.addMarkerAtPlayhead(s); }

    // The navigation keys (spec §4.5). Edit shortcuts stay in video-studio-editing.js; this table holds the rest.
    function navCommand(s, event) {
        const raw = String(event.key || ''), key = raw.length === 1 ? raw.toLowerCase() : raw, mod = event.ctrlKey || event.metaKey;
        if (mod && event.altKey) return null;
        if (mod) {
            if (key === 'g' && !event.shiftKey) return { run: () => openGoTo(s) };
            if (key === 'm' && event.shiftKey) return { run: () => stepMarker(s, -1) };
            return null;
        }
        if (event.altKey) return key === 'x' ? { run: () => clearInOut(s) } : null;
        if (key === 'j' || key === 'k' || key === 'l') return { run: () => s.shuttle.press(key), repeat: false };
        if (key === 'ArrowUp' || key === 'ArrowDown') return { run: () => stepEditPoint(s, key === 'ArrowUp' ? -1 : 1) };
        if (key === 'i' || key === 'o') return { run: () => (event.shiftKey ? goToInOut(s, key === 'i' ? 'in' : 'out') : setInOut(s, key === 'i' ? 'in' : 'out')) };
        if (key === 'm') return { run: () => (event.shiftKey ? stepMarker(s, 1) : addMarkerAtPlayhead(s)) };
        return null;
    }
    // Returns true when the key was a navigation shortcut and has been handled.
    function handleNavKeys(s, event) {
        const command = navCommand(s, event);
        if (!command) return false;
        event.preventDefault();
        // J, K and L do not accelerate while the key is held down.
        if (command.repeat === false && event.repeat) return true;
        command.run();
        return true;
    }
    function isNavShortcut(s, event) { return !!navCommand(s, event); }

    Object.assign(P, { inOut, snapExtras, createShuttle, togglePlay, loopRange, soloTrackIds, syncTransport, toggleLoop, setInOut, clearInOut, goToInOut, openGoTo, stepEditPoint, stepMarker, seekTo, navCommand, handleNavKeys, isNavShortcut });
})();
