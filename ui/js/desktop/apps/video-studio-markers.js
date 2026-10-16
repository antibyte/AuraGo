(function () {
    'use strict';

    // Markers (spec §4.1, §4.5): add at the playhead, jump to, edit in a popover, the marker menu, range from in/out and
    // delete. Every change is one undo step (mutate). The timeline drags markers itself (video-studio-timeline.js).
    const P = window.VideoStudioParts;
    const T = (...args) => P.T(...args);
    const esc = (...args) => P.esc(...args);
    const mutate = (...args) => P.mutate(...args);
    const tr = (...args) => P.tr(...args);
    const showNotice = (...args) => P.showNotice(...args);
    const maxFrames = (...args) => P.maxFrames(...args);
    const seekTo = (...args) => P.seekTo(...args);
    const inOut = (...args) => P.inOut(...args);
    const Time = () => window.VideoStudioTime;
    const Nav = () => window.VideoStudioNavigation;
    const clamp = (value, lo, hi) => Math.max(lo, Math.min(hi, value));
    const label = (s, key, fallback) => esc(s, tr(s, key, fallback));
    const readonlyNotice = s => showNotice(s, 'readonly', 'Read-only mode prevents editing.', true);

    function markers(s) { return (s.project && s.project.markers) || []; }
    function sorted(list) { return list.slice().sort((a, b) => a.frame - b.frame || (a.id < b.id ? -1 : 1)); }
    function markerById(s, id) { return markers(s).find(marker => marker.id === id) || null; }
    // The marker at exactly this frame, or null.
    function markerAt(s, frame) { return markers(s).find(marker => marker.frame === frame) || null; }
    // Client coordinates of an element's bottom-left corner (the popover anchor).
    function pointOf(element) { const rect = element.getBoundingClientRect(); return { x: rect.left, y: rect.bottom }; }
    function pointOfPlayhead(s) { return pointOf(s.q('[data-timeline] [data-playhead]') || s.app); }
    function palette() { return window.VideoStudioTimeline.LABEL_COLORS; }

    function addMarker(s, frame) {
        if (s.readonly) return readonlyNotice(s);
        if (!s.project) return false;
        if (markers(s).length >= Nav().MAX_MARKERS) { showNotice(s, 'markerLimit', 'This project reached its marker limit.', true); return false; }
        const marker = { id: T().newId('marker'), frame: clamp(Math.round(frame), 0, maxFrames(s)), duration: 0, name: '', color: '', note: '' };
        return mutate(s, 'add marker', project => { project.markers = sorted((project.markers || []).concat(marker)); });
    }
    // M: add a marker at the playhead, or open the marker that is already there.
    function addMarkerAtPlayhead(s) {
        if (s.readonly) return readonlyNotice(s);
        if (!s.project) return false;
        const found = markerAt(s, s.frame);
        if (found) return openMarkerPopover(s, found.id, pointOfPlayhead(s));
        return addMarker(s, s.frame);
    }
    function jumpToMarker(s, id) {
        const marker = markerById(s, id);
        if (marker) seekTo(s, marker.frame);
    }
    // Writes marker fields (any subset) in one undo step. Frames stay inside the timeline and a range inside the limit.
    function updateMarker(s, id, patch) {
        const max = maxFrames(s);
        return mutate(s, 'marker', project => {
            const marker = (project.markers || []).find(item => item.id === id);
            if (!marker) return false;
            const duration = Number.isFinite(patch.duration) ? Math.max(0, Math.round(patch.duration)) : marker.duration || 0;
            const frame = Number.isFinite(patch.frame) ? clamp(Math.round(patch.frame), 0, max) : marker.frame;
            if (frame + duration > max) { showNotice(s, 'editLimit', 'The edit would go past the project limits.', true); return false; }
            if (patch.name != null) marker.name = Array.from(String(patch.name).trim()).slice(0, 120).join('');
            if (patch.note != null) marker.note = Array.from(String(patch.note)).slice(0, 2000).join('');
            if (patch.color != null) marker.color = palette().includes(patch.color) ? patch.color : '';
            marker.frame = frame;
            marker.duration = duration;
            project.markers = sorted(project.markers);
        });
    }
    function deleteMarker(s, id) {
        const done = mutate(s, 'delete marker', project => { project.markers = (project.markers || []).filter(item => item.id !== id); });
        // A deleted flag takes its focus with it; the editor keeps the keyboard.
        if (done && !s.app.contains(document.activeElement)) s.app.focus({ preventScroll: true });
        return done;
    }
    // The marker spans the in/out range (it applies at once).
    function markerRangeFromInOut(s, id) {
        const io = inOut(s);
        if (io.in == null || io.out == null) return showNotice(s, 'markerNeedsRange', 'Set the in and out points first.', true);
        return updateMarker(s, id, { frame: io.in, duration: io.out - io.in });
    }
    function moveMarkerToPlayhead(s, id) { return updateMarker(s, id, { frame: s.frame }); }

    // The popover fields as updateMarker would store them, compared with the marker (Apply with no change is no undo step).
    function sameFields(marker, fields) {
        return (marker.duration || 0) === fields.duration && (marker.name || '') === fields.name.trim() && (marker.color || '') === fields.color && (marker.note || '') === fields.note;
    }

    // The marker popover: name, colour, note, duration, range from in/out and delete. Apply writes the fields in one step.
    // point is {x, y} in client coordinates; the popover stays inside the editor and closes on Escape or an outside press.
    function openMarkerPopover(s, id, point) {
        const marker = markerById(s, id);
        if (!marker) return false;
        if (s.readonly) return readonlyNotice(s);
        closeMarkerPopover(s, false);
        const uid = esc(s, s.id), io = inOut(s), canRange = io.in != null && io.out != null;
        const colors = palette().map(color => `<label class="vs-swatch-option"><input type="radio" name="vs-marker-color-${uid}" value="${color}" ${marker.color === color ? 'checked' : ''}><span class="vs-swatch" style="--vs-swatch:var(--vs-label-${color})"></span><span>${label(s, 'color_' + color, color)}</span></label>`).join('');
        const pop = document.createElement('form');
        pop.className = 'vs-marker-pop';
        pop.setAttribute('role', 'dialog');
        pop.setAttribute('aria-label', tr(s, 'marker', 'Marker'));
        pop.innerHTML = `<div class="vs-marker-head"><strong>${label(s, 'marker', 'Marker')}</strong><span class="vs-hint">${esc(s, Time().formatTime(marker.frame, s.project))}</span></div>`
            + `<label class="vs-field vs-field-wide"><span>${label(s, 'markerName', 'Name')}</span><input type="text" maxlength="120" data-marker-name value="${esc(s, marker.name || '')}"></label>`
            + `<fieldset class="vs-marker-colors"><legend>${label(s, 'markerColor', 'Color')}</legend><label class="vs-swatch-option"><input type="radio" name="vs-marker-color-${uid}" value="" ${marker.color ? '' : 'checked'}><span class="vs-swatch"></span><span>${label(s, 'markerNoColor', 'No color')}</span></label>${colors}</fieldset>`
            + `<label class="vs-field vs-field-wide"><span>${label(s, 'markerNote', 'Note')}</span><textarea rows="3" maxlength="2000" data-marker-note>${esc(s, marker.note || '')}</textarea></label>`
            + `<div class="vs-field-row"><label class="vs-field"><span>${label(s, 'duration', 'Duration')}</span><input type="text" inputmode="decimal" spellcheck="false" data-marker-duration value="${esc(s, Time().formatTime(marker.duration || 0, s.project))}"></label></div>`
            + `<p class="vs-hint">${label(s, 'timeHint', 'Enter seconds (4,5) or minutes:seconds (1:04.5).')}</p>`
            + `<div class="vs-marker-actions"><button type="button" class="vs-chip-button" data-marker-range ${canRange ? '' : 'disabled'} title="${label(s, 'markerNeedsRange', 'Set the in and out points first.')}">${label(s, 'markerRange', 'Range from in/out')}</button><button type="button" class="vs-chip-button vs-danger" data-marker-delete>${label(s, 'markerDelete', 'Delete marker')}</button><span class="vs-spacer"></span><button type="button" class="vs-chip-button" data-marker-cancel>${label(s, 'cancel', 'Cancel')}</button><button type="submit" class="vs-primary">${label(s, 'apply', 'Apply')}</button></div>`;
        const abort = new AbortController(), signal = abort.signal;
        s.app.append(pop);
        const box = s.app.getBoundingClientRect(), place = point || pointOfPlayhead(s);
        pop.style.left = clamp(place.x - box.left, 4, Math.max(4, box.width - pop.offsetWidth - 4)) + 'px';
        pop.style.top = clamp(place.y - box.top + 6, 4, Math.max(4, box.height - pop.offsetHeight - 4)) + 'px';
        s.markerPop = { el: pop, id, abort };
        pop.addEventListener('submit', event => {
            event.preventDefault();
            const duration = Time().parseTime(pop.querySelector('[data-marker-duration]').value, s.project);
            if (duration == null) { showNotice(s, 'invalidTime', 'Enter a time like 4,5 or 1:04.5.', true); return; }
            const checked = pop.querySelector('input[type="radio"]:checked'), current = markerById(s, id);
            const fields = { duration, name: pop.querySelector('[data-marker-name]').value, color: checked ? checked.value : '', note: pop.querySelector('[data-marker-note]').value };
            if (!current || sameFields(current, fields) || updateMarker(s, id, fields)) closeMarkerPopover(s, true);
        }, { signal });
        pop.querySelector('[data-marker-cancel]').addEventListener('click', () => closeMarkerPopover(s, true), { signal });
        pop.querySelector('[data-marker-delete]').addEventListener('click', () => { closeMarkerPopover(s, true); deleteMarker(s, id); }, { signal });
        pop.querySelector('[data-marker-range]').addEventListener('click', () => { closeMarkerPopover(s, true); markerRangeFromInOut(s, id); }, { signal });
        pop.addEventListener('keydown', event => { if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); closeMarkerPopover(s, true); } }, { signal });
        window.addEventListener('pointerdown', event => {
            const target = event.target;
            if (pop.contains(target) || (target.closest && target.closest('[data-marker-id],[data-marker-edit]'))) return;
            closeMarkerPopover(s, false);
        }, { signal, capture: true });
        pop.querySelector('[data-marker-name]').focus();
        return true;
    }
    function closeMarkerPopover(s, returnFocus) {
        const pop = s.markerPop;
        if (!pop) return;
        s.markerPop = null;
        pop.abort.abort();
        pop.el.remove();
        if (returnFocus) s.app.focus({ preventScroll: true });
    }
    // Closes the popover when its marker no longer exists (an undo or a delete elsewhere).
    function syncMarkerPopover(s) {
        if (s.markerPop && !markerById(s, s.markerPop.id)) closeMarkerPopover(s, false);
    }
    // The marker menu (right-click, the context-menu key, Shift+F10 on a flag).
    function openMarkerMenu(s, id, x, y) {
        const marker = markerById(s, id);
        if (!marker) return null;
        const ro = !!s.readonly, io = inOut(s), point = { x, y };
        const items = [
            { label: tr(s, 'markerGoTo', 'Go to marker'), run: () => jumpToMarker(s, id) },
            { label: tr(s, 'markerEdit', 'Edit marker…'), disabled: ro, run: () => openMarkerPopover(s, id, point) },
            { label: tr(s, 'markerMoveHere', 'Move to playhead'), disabled: ro, run: () => moveMarkerToPlayhead(s, id) },
            { label: tr(s, 'markerRange', 'Range from in/out'), disabled: ro || io.in == null || io.out == null, run: () => markerRangeFromInOut(s, id) },
            { separator: true },
            { label: tr(s, 'markerDelete', 'Delete marker'), icon: 'trash', disabled: ro, run: () => deleteMarker(s, id) }
        ];
        return P.openMenu(s, x, y, items, tr(s, 'marker', 'Marker'), () => s.q(`[data-timeline] [data-marker-id="${CSS.escape(id)}"]`));
    }

    Object.assign(P, { markerAt, addMarker, addMarkerAtPlayhead, jumpToMarker, openMarkerPopover, closeMarkerPopover, syncMarkerPopover, updateMarker, deleteMarker, markerRangeFromInOut, moveMarkerToPlayhead, openMarkerMenu, pointOf });
})();
