(function () {
    'use strict';

    // Track management (spec §4.6): rename, add above or below, delete, reorder within a kind, heights (S, M, L), solo on
    // audio (preview only), paste targets and the track menu. Heights and targets are per viewer per project in
    // localStorage (try/catch); solo is session state.
    const P = window.VideoStudioParts;
    const T = (...args) => P.T(...args);
    const mutate = (...args) => P.mutate(...args);
    const tr = (...args) => P.tr(...args);
    const showNotice = (...args) => P.showNotice(...args);
    const renderTimeline = (...args) => P.renderTimeline(...args);
    const updateTransform = (...args) => P.updateTransform(...args);
    const Nav = () => window.VideoStudioNavigation;
    const HEIGHT_PREFIX = 'aurago.videoStudio.trackHeights.', TARGET_PREFIX = 'aurago.videoStudio.targets.';
    const HEIGHT_NAMES = { s: 'heightSmall', m: 'heightMedium', l: 'heightLarge' };
    const HEIGHT_FALLBACK = { s: 'Small', m: 'Medium', l: 'Large' };

    function readJSON(key) {
        try {
            const value = JSON.parse(window.localStorage.getItem(key) || 'null');
            return value && typeof value === 'object' && !Array.isArray(value) ? value : {};
        } catch (_) { return {}; }
    }
    function writeJSON(key, value) {
        try { window.localStorage.setItem(key, JSON.stringify(value)); } catch (_) { /* per-viewer convenience only */ }
    }
    const heightKey = s => HEIGHT_PREFIX + s.projectId;
    const targetKey = s => TARGET_PREFIX + s.projectId;
    function trackOf(s, id) { return (s.project && s.project.tracks || []).find(track => track.id === id) || null; }
    const menuButton = (s, trackId) => s.q(`[data-timeline] [data-track-row="${CSS.escape(trackId)}"] [data-track-action="menu"]`);

    // Height of a track in this viewer: 's', 'm' (the kind's default) or 'l'.
    function trackHeight(s, trackId) {
        const value = readJSON(heightKey(s))[trackId];
        return value === 's' || value === 'l' ? value : 'm';
    }
    function setTrackHeight(s, trackId, key) {
        const map = readJSON(heightKey(s));
        if (key === 's' || key === 'l') map[trackId] = key; else delete map[trackId];
        writeJSON(heightKey(s), map);
        renderTimeline(s);
    }

    // One paste target per kind; '' means none. Without a stored choice the first track of the kind is the target
    // (overlay: none). A stored track that no longer exists falls back to that default.
    function targetTracks(s) {
        const stored = readJSON(targetKey(s)), tracks = (s.project && s.project.tracks) || [];
        const pick = kind => {
            const list = tracks.filter(track => track.kind === kind), fallback = kind === 'overlay' || !list.length ? '' : list[0].id;
            if (!Object.hasOwn(stored, kind)) return fallback;
            const id = stored[kind];
            if (!id) return '';
            return list.some(track => track.id === id) ? id : fallback;
        };
        return { video: pick('video'), audio: pick('audio'), overlay: pick('overlay') };
    }
    // The targets that paste uses (only the kinds that have one).
    function pasteTargets(s) {
        return Object.fromEntries(Object.entries(targetTracks(s)).filter(([, id]) => id));
    }
    // View changes repaint the timeline and hand the solo set to the preview; they never touch the project or history.
    function afterView(s) {
        if (s.preview) { P.syncTransport(s); if (!s.preview.isPlaying()) s.preview.seek(s.frame); }
        renderTimeline(s);
    }
    function toggleTarget(s, trackId) {
        const track = trackOf(s, trackId);
        if (!track) return;
        const stored = readJSON(targetKey(s)), current = targetTracks(s)[track.kind];
        stored[track.kind] = current === trackId ? '' : trackId;
        writeJSON(targetKey(s), stored);
        afterView(s);
    }
    // Solo on an audio track: while any track is soloed, only soloed audio sounds in the preview. Never saved.
    function toggleSolo(s, trackId) {
        const track = trackOf(s, trackId);
        if (!track || track.kind !== 'audio') return;
        const solo = new Set(s.soloIds || []);
        if (solo.has(trackId)) solo.delete(trackId); else solo.add(trackId);
        s.soloIds = solo;
        afterView(s);
    }
    function trackState(s, track) {
        return {
            height: trackHeight(s, track.id), target: targetTracks(s)[track.kind] === track.id,
            solo: !!(s.soloIds && s.soloIds.has(track.id)), renaming: s.renamingTrackId === track.id
        };
    }

    // Starts the rename (runTrackAction 'rename') or finishes it: value null cancels; an empty or unchanged name is dropped.
    // Enter and Escape hand the focus to the track's menu button; a blur leaves it where the user put it.
    function renameTrack(s, trackId, value) {
        if (s.renamingTrackId !== trackId) return;
        const active = document.activeElement, keyboard = !!(active && active.dataset && active.dataset.trackRename === trackId);
        s.renamingTrackId = '';
        const track = trackOf(s, trackId), name = value == null ? '' : Array.from(String(value).trim()).slice(0, 120).join('');
        if (!track || !name || name === track.name || !mutate(s, 'rename track', project => {
            const target = (project.tracks || []).find(item => item.id === trackId);
            if (!target) return false;
            target.name = name;
        })) renderTimeline(s);
        if (keyboard) (menuButton(s, trackId) || s.app).focus({ preventScroll: true });
    }
    // Adds a track of the reference's kind above or below it (displaySide).
    function addTrackBeside(s, trackId, displaySide) {
        const reference = trackOf(s, trackId);
        if (!reference || s.readonly) return false;
        if (!T().trackCapacity(s.project, reference.kind)) { showNotice(s, 'trackFull', 'This kind of track is full (8 tracks).', true); return false; }
        return mutate(s, 'add track', project => {
            const track = P.makeTrack(s, project.tracks, reference.kind);
            project.tracks.splice(Nav().trackInsertIndex(project.tracks, trackId, displaySide), 0, track);
        });
    }
    // Deletes a track. An empty track goes at once; otherwise the clips go too, after a confirmation. A locked track stays.
    async function deleteTrack(s, trackId) {
        const track = trackOf(s, trackId);
        if (!track || s.readonly) return false;
        if (track.locked) { showNotice(s, 'trackLockedDelete', 'Unlock the track before deleting it.', true); return false; }
        if (track.clips.length) {
            const copy = tr(s, 'deleteTrackCopy', 'The track holds {{count}} clips. They are removed too; you can undo this.').replace('{{count}}', String(track.clips.length));
            const projectId = s.projectId, confirmed = await s.ctx.confirmDialog(tr(s, 'deleteTrackTitle', 'Delete this track?'), copy);
            // The answer only counts for the project and track it was asked about.
            if (!confirmed || s.disposed || s.projectId !== projectId || !trackOf(s, trackId)) return false;
            if (trackOf(s, trackId).locked) { showNotice(s, 'trackLockedDelete', 'Unlock the track before deleting it.', true); return false; }
        }
        const removed = mutate(s, 'remove track', project => { project.tracks = project.tracks.filter(item => item.id !== trackId); });
        if (removed && s.selectedClipId && !T().clipFor(s.project, s.selectedClipId)) P.setSelection(s, [], '');
        return removed;
    }
    // Moves a track next to another track of the same kind (drag and drop of headers). displaySide is 'above' or 'below'.
    function moveTrack(s, fromId, toId, displaySide) {
        if (s.readonly) return false;
        const reference = trackOf(s, toId);
        if (!reference) return false;
        return mutate(s, 'reorder tracks', project => {
            const next = Nav().moveWithinKind(project.tracks, fromId, toId, Nav().arraySideFor(reference.kind, displaySide || 'below'));
            if (!next) return false;
            project.tracks = next;
        });
    }
    function setMuteHideLock(s, trackId, action) {
        mutate(s, 'track', project => {
            const target = (project.tracks || []).find(item => item.id === trackId);
            if (!target) return false;
            if (action === 'mute') target.muted = !target.muted;
            if (action === 'hide') target.hidden = !target.hidden;
            if (action === 'lock') target.locked = !target.locked;
        });
        updateTransform(s);
    }
    // Header buttons and menu items. Targets, solo, heights, the menu and starting a rename are view settings (no undo
    // step, the project stays clean); the rest changes the project.
    function runTrackAction(s, trackId, action, element) {
        const track = trackOf(s, trackId);
        if (!track) return;
        if (action === 'target') return toggleTarget(s, trackId);
        if (action === 'solo') return toggleSolo(s, trackId);
        if (action === 'menu') {
            const rect = element ? element.getBoundingClientRect() : { left: 0, bottom: 0 };
            return openTrackMenu(s, trackId, rect.left, rect.bottom, element);
        }
        if (action === 'rename') { if (!s.readonly) { s.renamingTrackId = trackId; renderTimeline(s); } return; }
        if (s.readonly) return;
        if (action === 'remove') return deleteTrack(s, trackId);
        if (action === 'mute' || action === 'hide' || action === 'lock') setMuteHideLock(s, trackId, action);
    }
    // The track menu (header "…" button or right-click on a header).
    function openTrackMenu(s, trackId, x, y, anchor) {
        const track = trackOf(s, trackId);
        if (!track) return null;
        const ro = !!s.readonly, full = !T().trackCapacity(s.project, track.kind), height = trackHeight(s, trackId);
        const items = [
            { label: tr(s, 'renameTrack', 'Rename track'), disabled: ro, run: () => runTrackAction(s, trackId, 'rename') },
            { label: tr(s, 'addTrackAbove', 'Add track above'), disabled: ro || full, run: () => addTrackBeside(s, trackId, 'above') },
            { label: tr(s, 'addTrackBelow', 'Add track below'), disabled: ro || full, run: () => addTrackBeside(s, trackId, 'below') },
            { separator: true },
            ...['s', 'm', 'l'].map(key => ({ label: (key === height ? '• ' : '') + tr(s, HEIGHT_NAMES[key], HEIGHT_FALLBACK[key]), run: () => setTrackHeight(s, trackId, key) })),
            { separator: true },
            { label: tr(s, 'targetTrack', 'Target track'), run: () => toggleTarget(s, trackId) },
            ...(track.kind === 'audio' ? [{ label: tr(s, 'solo', 'Solo'), run: () => toggleSolo(s, trackId) }] : []),
            ...(track.kind !== 'overlay' ? [{ label: tr(s, track.muted ? 'unmute' : 'mute', track.muted ? 'Unmute track' : 'Mute track'), disabled: ro, run: () => runTrackAction(s, trackId, 'mute') }] : []),
            ...(track.kind !== 'audio' ? [{ label: tr(s, track.hidden ? 'show' : 'hide', track.hidden ? 'Show track' : 'Hide track'), disabled: ro, run: () => runTrackAction(s, trackId, 'hide') }] : []),
            { label: tr(s, track.locked ? 'unlock' : 'lock', track.locked ? 'Unlock track' : 'Lock track'), disabled: ro, run: () => runTrackAction(s, trackId, 'lock') },
            { separator: true },
            { label: tr(s, 'deleteTrack', 'Delete track'), icon: 'trash', disabled: ro, run: () => deleteTrack(s, trackId) }
        ];
        const focus = () => (anchor && anchor.isConnected ? anchor : menuButton(s, trackId));
        return P.openMenu(s, x, y, items, tr(s, 'trackActions', 'Track actions'), focus);
    }

    Object.assign(P, { trackHeight, setTrackHeight, targetTracks, pasteTargets, toggleTarget, toggleSolo, trackState, renameTrack, addTrackBeside, deleteTrack, moveTrack, openTrackMenu, runTrackAction });
})();
