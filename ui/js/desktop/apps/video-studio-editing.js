(function () {
    'use strict';

    // Selection, clipboard, edit tools and the edit commands behind the timeline toolbar, menus and shortcuts.
    const P = window.VideoStudioParts;
    const { PREF_MAGNETIC } = P;
    const E = () => window.VideoStudioEdit;
    const T = (...args) => P.T(...args);
    const esc = (...args) => P.esc(...args);
    const icon = (...args) => P.icon(...args);
    const mutate = (...args) => P.mutate(...args);
    const openModal = (...args) => P.openModal(...args);
    const renderInspector = (...args) => P.renderInspector(...args);
    const showNotice = (...args) => P.showNotice(...args);
    const syncInspectorLock = (...args) => P.syncInspectorLock(...args);
    const tr = (...args) => P.tr(...args);
    const updateTransform = (...args) => P.updateTransform(...args);
    const writePref = (...args) => P.writePref(...args);

    const TOOLS = ['select', 'trim', 'blade'];
    const REASON_NOTICES = {
        locked: ['editLocked', 'Unlock the track to edit these clips.'],
        media: ['editMedia', 'The clip\'s media does not allow this edit.'],
        bounds: ['editBounds', 'The clip cannot be trimmed or moved any further.'],
        limit: ['editLimit', 'The edit would go past the project limits.'],
        kind: ['editKind', 'That track does not accept this media.'],
        transition: ['editTransition', 'Move the playhead out of the transition first.']
    };

    function renderTimeline(s) { if (s.project && s.q('[data-timeline]')) T().render(s.q('[data-timeline]'), s.timelineOptions); }

    // The multi-selection counts while its primary clip is still the selected clip; code that sets
    // selectedClipId directly (adding media, titles, loading a project) collapses it to that clip.
    function selectionList(s) {
        if (!s.selectedClipId) return [];
        return s.selectionPrimary === s.selectedClipId && s.selectionSet ? Array.from(s.selectionSet) : [s.selectedClipId];
    }
    function selectedIds(s) { return s.project ? selectionList(s).filter(id => T().clipFor(s.project, id)) : []; }
    function isSelected(s, id) {
        if (!id || !s.selectedClipId) return false;
        return s.selectionPrimary === s.selectedClipId && s.selectionSet ? s.selectionSet.has(id) : id === s.selectedClipId;
    }
    // Records a selection without rendering (inside mutate, which renders afterwards). Always a new Set, so
    // mutate can restore the previous one by reference.
    function assignSelection(s, ids, primary) {
        const list = Array.from(new Set((ids || []).filter(Boolean)));
        const head = list.includes(primary) ? primary : list[list.length - 1] || '';
        s.selectionSet = new Set(list); s.selectionPrimary = head; s.selectedClipId = head; s.selectionRevision++;
    }
    function setSelection(s, ids, primary) {
        assignSelection(s, ids, primary);
        renderInspector(s); syncInspectorLock(s); updateTransform(s);
    }
    function toggleSelection(s, id) {
        const ids = selectedIds(s);
        if (!ids.includes(id)) { setSelection(s, ids.concat(id), id); return; }
        const rest = ids.filter(item => item !== id);
        setSelection(s, rest, s.selectedClipId === id ? rest[rest.length - 1] : s.selectedClipId);
    }
    function selectAll(s) {
        if (!s.project) return;
        setSelection(s, s.project.tracks.flatMap(track => track.clips.map(clip => clip.id)), s.selectedClipId);
        renderTimeline(s);
    }
    function selectForward(s, trackId, frame) {
        if (!s.project) return;
        const ids = E().clipsFrom(s.project, trackId, frame);
        setSelection(s, ids, ids[0]);
        renderTimeline(s);
    }

    function showReason(s, reason) {
        const notice = REASON_NOTICES[reason];
        if (notice) showNotice(s, notice[0], notice[1], false);
    }
    // One edit operation in one undo step. A refusal restores everything and explains why in an info toast;
    // `select(out)` returns the clips to select afterwards.
    function applyEdit(s, label, op, select) {
        let reason = '';
        const ok = mutate(s, label, project => {
            const out = {};
            const result = op(project, out);
            if (result !== true) { reason = result; return false; }
            if (select) { const ids = select(out) || []; assignSelection(s, ids, ids[0]); }
            return undefined;
        });
        if (!ok && reason) showReason(s, reason);
        return ok;
    }

    function deleteSelection(s, ripple) {
        const ids = selectedIds(s);
        if (!ids.length || s.readonly) return false;
        const rippling = ripple || !!s.magnetic;
        const ok = applyEdit(s, rippling ? 'ripple delete' : 'delete', project => (rippling ? E().rippleDelete(project, ids) : E().removeClips(project, ids)), () => []);
        if (ok && !s.host.contains(document.activeElement)) s.app.focus({ preventScroll: true });
        return ok;
    }
    function deleteGap(s, trackId, frame) { return applyEdit(s, 'delete gap', project => E().rippleDeleteGap(project, trackId, frame)); }
    function splitSelection(s) {
        const frame = s.frame;
        const ids = selectedIds(s).filter(id => { const clip = T().clipFor(s.project, id).clip; return clip.start < frame && frame < clip.start + clip.duration; });
        if (!ids.length) { showNotice(s, 'splitHint', 'Move the playhead inside the selected clip to split it.', true); return false; }
        return applyEdit(s, 'split', (project, out) => E().blade(project, frame, { clipIds: ids }, out), out => out.created);
    }
    function bladeAt(s, trackId, frame, allTracks) {
        if (!s.project) return false;
        const trackIds = allTracks ? s.project.tracks.filter(track => !track.locked).map(track => track.id) : [trackId];
        return applyEdit(s, 'blade', (project, out) => E().blade(project, frame, { trackIds }, out));
    }
    function duplicateSelection(s) {
        const ids = selectedIds(s);
        return ids.length ? applyEdit(s, 'duplicate', (project, out) => E().duplicateClips(project, ids, out), out => out.created) : false;
    }
    function nudgeSelection(s, frames) {
        const ids = selectedIds(s);
        return ids.length ? applyEdit(s, 'nudge', project => E().moveClips(project, ids, s.selectedClipId, frames, 0, 'overwrite', {})) : false;
    }
    // Q / W: ripple trim the start or end of the selected clips under the playhead, or of the top-most
    // unlocked clip under it, to the playhead.
    function trimToPlayhead(s, edge) {
        if (!s.project || s.readonly) return false;
        const frame = s.frame, under = clip => clip.start < frame && frame < clip.start + clip.duration;
        let ids = selectedIds(s).filter(id => under(T().clipFor(s.project, id).clip));
        if (!ids.length) {
            const top = T().displayOrder(s.project.tracks).filter(track => !track.locked).flatMap(track => track.clips.filter(under))[0];
            ids = top ? [top.id] : [];
        }
        if (!ids.length) return false;
        const starts = ids.map(id => T().clipFor(s.project, id).clip.start);
        const ok = applyEdit(s, edge === 'start' ? 'trim start' : 'trim end', project => {
            for (const id of ids) {
                const clip = T().clipFor(project, id).clip;
                const result = E().rippleTrim(project, [id], edge, edge === 'start' ? frame - clip.start : frame - clip.start - clip.duration);
                if (result !== true) return result;
            }
            return true;
        });
        // After Q the trimmed content begins at the clip start; the playhead follows it.
        if (ok && edge === 'start') s.preview.seek(Math.min(...starts));
        return ok;
    }

    function clipboardReady(s) { return !!(s.clipboard && s.clipboard.data && s.clipboard.projectId === s.projectId); }
    function copySelection(s) {
        const ids = selectedIds(s);
        if (!ids.length) return false;
        s.clipboard = { projectId: s.projectId, data: E().copyClips(s.project, ids) };
        showNotice(s, 'clipsCopied', 'Copied.', false);
        return true;
    }
    function cutSelection(s) { return !s.readonly && copySelection(s) && deleteSelection(s, false); }
    // Pastes at `at` (default: the playhead). `trackId` puts the clips of that track's kind onto it.
    function pasteClipboard(s, mode, at, trackId) {
        if (s.readonly || !s.project) return false;
        if (!s.clipboard || !s.clipboard.data) { showNotice(s, 'clipboardEmpty', 'Copy clips first.', false); return false; }
        if (s.clipboard.projectId !== s.projectId) { showNotice(s, 'clipboardOtherProject', 'Clips can only be pasted into the project they were copied from.', false); return false; }
        const track = trackId ? s.project.tracks.find(item => item.id === trackId) : null;
        const frame = Number.isFinite(at) ? at : s.frame, data = s.clipboard.data;
        return applyEdit(s, mode === 'insert' ? 'paste insert' : 'paste', (project, out) => E().paste(project, data, frame, mode, track ? { out, targets: { [track.kind]: track.id } } : { out }), out => out.created);
    }
    function openPasteAttributes(s) {
        if (s.readonly || !s.project) return;
        const ids = selectedIds(s);
        if (!clipboardReady(s)) { showNotice(s, 'clipboardEmpty', 'Copy clips first.', false); return; }
        if (!ids.length) { showNotice(s, 'selectClipsFirst', 'Select the clips that should receive the attributes.', false); return; }
        const source = s.clipboard.data.entries[0].clip, label = (key, fallback) => esc(s, tr(s, key, fallback)), id = esc(s, s.id);
        const host = openModal(s, `<form class="vs-modal" data-attr-form role="dialog" aria-modal="true" aria-labelledby="vs-attr-title-${id}"><button type="button" class="vs-modal-close" data-modal-close aria-label="${label('close', 'Close')}">${icon('close', 18)}</button><h2 id="vs-attr-title-${id}">${label('pasteAttributesTitle', 'Paste attributes')}</h2><fieldset class="vs-attr-groups"><label><input type="checkbox" data-attr-group value="picture" checked><span>${label('attrPicture', 'Picture: position, size, rotation, opacity, fit')}</span></label><label><input type="checkbox" data-attr-group value="sound" checked><span>${label('attrSound', 'Sound: volume and fades')}</span></label></fieldset><div class="vs-modal-actions"><button type="button" data-modal-close>${label('cancel', 'Cancel')}</button><button type="submit" class="vs-primary">${label('apply', 'Apply')}</button></div></form>`);
        host.querySelector('[data-attr-form]').addEventListener('submit', event => {
            event.preventDefault();
            const groups = Array.from(host.querySelectorAll('[data-attr-group]:checked'), input => input.value);
            host.replaceChildren();
            s.app.focus({ preventScroll: true });
            if (groups.length) applyEdit(s, 'paste attributes', project => E().pasteAttributes(project, source, ids, groups));
        });
    }

    function setTool(s, tool) {
        s.tool = TOOLS.includes(tool) && !s.readonly ? tool : 'select';
        renderTimeline(s);
    }
    function toggleMagnetic(s) {
        if (s.readonly) return;
        s.magnetic = !s.magnetic;
        writePref(PREF_MAGNETIC, s.magnetic);
        renderTimeline(s);
    }
    // Timeline toolbar buttons handled here; false hands the action back to the timeline.
    function timelineCommand(s, name) {
        if (name.startsWith('tool-')) { setTool(s, name.slice(5)); return true; }
        if (name === 'magnetic') { toggleMagnetic(s); return true; }
        if (s.readonly) return false;
        if (name === 'split') { splitSelection(s); return true; }
        if (name === 'duplicate') { duplicateSelection(s); return true; }
        if (name === 'delete') { deleteSelection(s, false); return true; }
        return false;
    }

    // Edit shortcuts (spec §4.5). `selection` commands act on the selected clips and first select a focused,
    // unselected clip; `always` commands swallow the key even without a selection (Alt+arrows would otherwise
    // navigate the browser history).
    function editCommand(s, event) {
        const key = event.key.length === 1 ? event.key.toLowerCase() : event.key, mod = event.ctrlKey || event.metaKey;
        if (mod && event.altKey) return !event.shiftKey && key === 'v' ? { run: () => openPasteAttributes(s) } : null;
        if (mod) {
            if (key === 'c') return { run: () => copySelection(s), selection: true };
            if (key === 'x') return { run: () => cutSelection(s), selection: true };
            if (key === 'v') return { run: () => pasteClipboard(s, event.shiftKey ? 'insert' : 'overwrite') };
            if (event.shiftKey) return null;
            if (key === 'd') return { run: () => duplicateSelection(s), selection: true };
            if (key === 'a') return { run: () => selectAll(s) };
            return null;
        }
        if (event.altKey) return key === 'ArrowLeft' || key === 'ArrowRight' ? { run: () => nudgeSelection(s, (key === 'ArrowLeft' ? -1 : 1) * (event.shiftKey ? 10 : 1)), selection: true, always: true } : null;
        if (key === 'Delete' || key === 'Backspace') return { run: () => deleteSelection(s, event.shiftKey), selection: true };
        if (event.shiftKey) return key === 'z' ? { run: () => s.q('[data-timeline] [data-action="zoom-fit"]')?.click() } : null;
        if (key === 'v' || key === 't' || key === 'b') return { run: () => setTool(s, key === 'v' ? 'select' : key === 't' ? 'trim' : 'blade') };
        if (key === 'n') return { run: () => s.q('[data-timeline] [data-action="snap"]')?.click() };
        if (key === 'q' || key === 'w') return { run: () => trimToPlayhead(s, key === 'q' ? 'start' : 'end') };
        if (key === 's') return { run: () => splitSelection(s), selection: true };
        return null;
    }
    function isEditShortcut(s, event) { return !!editCommand(s, event); }
    // Returns true when the key was an edit shortcut and has been handled.
    function handleEditKeys(s, event, focusedClip) {
        const command = editCommand(s, event);
        if (!command) return false;
        if (command.selection && focusedClip && !isSelected(s, focusedClip)) { setSelection(s, [focusedClip], focusedClip); renderTimeline(s); }
        if (command.selection && !command.always && !selectedIds(s).length) return false;
        event.preventDefault();
        command.run();
        return true;
    }

    Object.assign(P, { renderTimeline, selectedIds, isSelected, assignSelection, setSelection, toggleSelection, selectAll, selectForward, showReason, applyEdit, deleteSelection, deleteGap, splitSelection, bladeAt, duplicateSelection, nudgeSelection, trimToPlayhead, clipboardReady, copySelection, cutSelection, pasteClipboard, openPasteAttributes, setTool, toggleMagnetic, timelineCommand, isEditShortcut, handleEditKeys });
})();
