(function () {
    'use strict';

    // Source monitor (spec §4.7). One library item plays in its own viewer with a transport, JKL and in/out marks
    // (kept per item in localStorage). The source plays through a second preview instance over a one-clip project,
    // so media elements and the frame clock stay in video-studio-preview.js. `,` inserts and `.` overwrites at the
    // place the timeline in/out and the playhead define; the picture and sound handles place one part each.
    const P = window.VideoStudioParts;
    const SE = () => window.VideoStudioSourceEdit;
    const E = () => window.VideoStudioEdit;
    const T = (...args) => P.T(...args);
    const I = (...args) => P.I(...args);
    const applyEdit = (...args) => P.applyEdit(...args);
    const clamp = (...args) => P.clamp(...args);
    const icon = (...args) => P.icon(...args);
    const previewScale = (...args) => P.previewScale(...args);
    const rateOf = (...args) => P.rateOf(...args);
    const showNotice = (...args) => P.showNotice(...args);
    const tr = (...args) => P.tr(...args);
    const updateTransform = (...args) => P.updateTransform(...args);

    // `sourceStart` is taken (the inspector's "Starts in source at"), so the start refusal uses `sourceBeforeStart`.
    const SOURCE_NOTICES = {
        start: ['sourceBeforeStart', 'The edit would start before the timeline begins. Move the timeline out point later.'],
        target: ['sourceNoTarget', 'Turn on a target track for this kind of media.']
    };
    const WAVE_POINTS = 400;

    // ---- Stored marks (viewer preference per project; never part of the project or the undo history) ----
    function marksKey(projectId) { return 'aurago.videoStudio.sourceMarks.' + projectId; }
    function readMarks(projectId) {
        try {
            const data = JSON.parse(window.localStorage.getItem(marksKey(projectId)) || '{}');
            return data && typeof data === 'object' && !Array.isArray(data) ? data : {};
        } catch (_) { return {}; }
    }
    function writeMarks(projectId, data) {
        try { window.localStorage.setItem(marksKey(projectId), JSON.stringify(SE().pruneMarks(data))); }
        catch (_) { /* per-viewer convenience only */ }
    }
    // The in/out marks of one item in this viewer, in frames at the project rate.
    function sourceMarks(s, assetId) {
        const asset = assetOf(s, assetId);
        if (!asset) return { in: null, out: null };
        return SE().marksFrom(readMarks(s.projectId)[assetId], s.project, SE().sourceFrames(s.project, asset));
    }
    function storeMarks(s) {
        const asset = currentAsset(s);
        if (!asset) return;
        const data = readMarks(s.projectId), entry = SE().entryFor(s.project, { in: s.sourceIn, out: s.sourceOut }, Date.now());
        if (entry.in == null && entry.out == null) delete data[asset.id];
        else data[asset.id] = entry;
        writeMarks(s.projectId, data);
    }

    // ---- Item helpers ----
    function assetOf(s, assetId) { return s.project ? (s.project.assets || []).find(asset => asset.id === assetId) || null : null; }
    function currentAsset(s) { return s.sourceAssetId && s.sourceProjectId === s.projectId ? assetOf(s, s.sourceAssetId) : null; }
    function lastFrame(s, asset) { return Math.max(0, SE().sourceFrames(s.project, asset) - 1); }
    function atSourceEnd(s) { const asset = currentAsset(s); return !!asset && s.sourceFrame >= lastFrame(s, asset); }
    function hasSound(asset) { return asset.kind === 'audio' || (asset.kind === 'video' && !!asset.has_audio); }
    // The item as a one-clip project: the preview mounts it like a timeline, so media elements stay in preview.js.
    function sourceProject(project, asset) {
        return {
            fps: project.fps, ntsc: !!project.ntsc, width: project.width, height: project.height, assets: [asset],
            tracks: [{
                id: 'source', kind: asset.kind === 'audio' ? 'audio' : 'video', name: 'Source', muted: false, hidden: false, locked: false,
                clips: [{ id: 'source-clip', asset_id: asset.id, start: 0, offset: 0, duration: SE().sourceFrames(project, asset), x: 0, y: 0, width: 1, height: 1, rotation: 0, opacity: 1, volume: 1, fade_in: 0, fade_out: 0, fit: 'contain', text: '', text_style: null, transition: null }]
            }]
        };
    }

    // ---- Preview lifecycle ----
    function pauseSource(s) {
        if (s.sourceShuttle) s.sourceShuttle.stop();
        if (s.sourcePreview) s.sourcePreview.pause();
    }
    function disposeSource(s) {
        if (s.sourceShuttle) { s.sourceShuttle.stop(); s.sourceShuttle = null; }
        if (s.sourcePreview) { s.sourcePreview.dispose(); s.sourcePreview = null; }
    }
    function syncSourcePlay(s, playing) {
        const button = s.q('[data-source-action="play"]');
        if (!button) return;
        button.innerHTML = icon(playing ? 'pause' : 'play', 18);
        button.setAttribute('aria-label', tr(s, playing ? 'pause' : 'play', playing ? 'Pause' : 'Play'));
        button.classList.toggle('is-playing', playing);
    }
    function mountSource(s, asset) {
        disposeSource(s);
        s.sourceProject = sourceProject(s.project, asset);
        s.sourcePreview = window.VideoStudioPreview.mount(s.q('[data-source-canvas]'), () => s.sourceProject, () => s.sourceFrame,
            frame => { const current = currentAsset(s); s.sourceFrame = current ? Math.min(frame, lastFrame(s, current)) : frame; renderSourceTime(s); },
            playing => { if (!playing && s.sourceShuttle) s.sourceShuttle.reset(); syncSourcePlay(s, playing); }, () => previewScale(s), { stopAtEnd: true });
        s.sourceShuttle = P.createShuttle({
            getFrame: () => s.sourceFrame,
            seek: frame => { if (s.sourcePreview) s.sourcePreview.seek(frame); },
            play: () => { if (s.sourcePreview) { if (atSourceEnd(s) && s.sourcePreview.rate() > 0) s.sourcePreview.seek(0); s.sourcePreview.play(); } },
            pause: () => { if (s.sourcePreview) s.sourcePreview.pause(); },
            setRate: rate => { if (s.sourcePreview) s.sourcePreview.setRate(rate); },
            maxFrame: () => (currentAsset(s) ? lastFrame(s, currentAsset(s)) : 0)
        });
    }

    // ---- Stage mode ----
    function renderStageMode(s) {
        const source = s.stageMode === 'source';
        s.host.querySelectorAll('[data-stage="timeline"]').forEach(element => { element.hidden = source; });
        const view = s.q('[data-source]');
        if (view) view.hidden = !source;
        s.host.querySelectorAll('[data-stage-mode]').forEach(button => { button.setAttribute('aria-pressed', String(button.dataset.stageMode === s.stageMode)); });
        const active = document.activeElement;
        if (active && active.closest && active.closest('[hidden]')) s.app.focus({ preventScroll: true });
        renderSource(s);
        updateTransform(s);
    }
    // Switches the stage. Showing the source needs an item; the timeline is paused while the source shows and back.
    function setStageMode(s, mode) {
        if (mode === 'source' && !s.sourceAssetId) {
            showNotice(s, 'sourceEmpty', 'Double-click a media item in the library to open it here.', false);
            return false;
        }
        if (mode === 'source') { if (s.preview) s.preview.pause(); }
        else pauseSource(s);
        s.stageMode = mode === 'source' ? 'source' : 'timeline';
        renderStageMode(s);
        return true;
    }
    // Opens a library item in the source monitor (double-click; slice 1e's library menu calls this too). A new item
    // loads its stored marks; the item that is already open keeps its position.
    function openSource(s, assetId) {
        const asset = assetOf(s, assetId);
        if (!asset) return false;
        if (!asset.kind) { showNotice(s, 'mediaPreparing', 'This media is still being prepared. Try again in a moment.', true); return false; }
        if (s.sourceAssetId !== asset.id || s.sourceProjectId !== s.projectId || !s.sourcePreview) {
            const marks = sourceMarks(s, asset.id);
            s.sourceAssetId = asset.id; s.sourceProjectId = s.projectId; s.sourceRate = window.VideoStudioTime.rateLabel(s.project);
            s.sourceIn = marks.in; s.sourceOut = marks.out; s.sourceFrame = marks.in || 0;
            mountSource(s, asset);
        }
        return setStageMode(s, 'source');
    }
    // Forgets the open item (its asset left the project, or the project changed) and returns to the timeline.
    function closeSource(s) {
        disposeSource(s);
        s.sourceAssetId = ''; s.sourceProjectId = ''; s.sourceIn = null; s.sourceOut = null; s.sourceFrame = 0; s.sourceProject = null; s.sourceRate = '';
        s.stageMode = 'timeline';
        renderStageMode(s);
    }
    // Called after every project render: keeps the open item in step with the project.
    function syncSource(s) {
        if (!s.sourceAssetId) return;
        const asset = currentAsset(s);
        if (!asset || !asset.kind) { closeSource(s); return; }
        // After a frame-rate change the marks (stored in seconds) are read again as frames at the new rate.
        const rate = window.VideoStudioTime.rateLabel(s.project);
        if (s.sourceRate && s.sourceRate !== rate) { const marks = sourceMarks(s, asset.id); s.sourceIn = marks.in; s.sourceOut = marks.out; s.sourceFrame = marks.in || 0; }
        s.sourceRate = rate;
        s.sourceProject = sourceProject(s.project, asset);
        s.sourceFrame = Math.min(s.sourceFrame || 0, lastFrame(s, asset));
        renderSource(s);
        if (s.sourcePreview && !s.sourcePreview.isPlaying()) s.sourcePreview.seek(s.sourceFrame);
    }

    // ---- Rendering ----
    function renderSourceTime(s) {
        const root = s.q('[data-source]'), asset = currentAsset(s);
        if (!root || !asset || !s.project) return;
        const frame = Math.round(s.sourceFrame || 0), scrub = root.querySelector('[data-source-scrub]');
        root.querySelector('[data-source-time]').textContent = I().formatTime(frame, s.project);
        if (scrub && document.activeElement !== scrub) scrub.value = String(frame);
    }
    function wavePath(data, frames, fps) {
        const top = [], bottom = [];
        for (let k = 0; k < WAVE_POINTS; k++) {
            const frame = k / (WAVE_POINTS - 1) * frames;
            const value = Math.min(1, data.peaks[Math.min(data.peaks.length - 1, Math.floor(frame / fps * data.perSecond))] || 0);
            top.push(`${k},${(50 - value * 46).toFixed(1)}`);
            bottom.unshift(`${k},${(50 + value * 46).toFixed(1)}`);
        }
        return 'M' + top.join('L') + 'L' + bottom.join('L') + 'Z';
    }
    // The waveform of an audio item; the peaks come from the media worker and appear once they are ready.
    function renderSourceWave(s, asset) {
        const host = s.q('[data-source-wave]');
        if (!host) return;
        const data = asset.kind === 'audio' ? s.media.peaks(asset) : null;
        if (!data || !data.peaks || !data.peaks.length) { host.replaceChildren(); return; }
        host.innerHTML = `<svg viewBox="0 0 ${WAVE_POINTS - 1} 100" preserveAspectRatio="none" aria-hidden="true"><path d="${wavePath(data, SE().sourceFrames(s.project, asset), rateOf(s))}"/></svg>`;
    }
    // Readouts, marks, drag handles and insert buttons for the open item (or the empty state).
    function renderSource(s) {
        const root = s.q('[data-source]');
        if (!root || !s.project) return;
        const asset = currentAsset(s), readonly = !!s.readonly;
        root.querySelector('[data-source-empty]').hidden = !!asset;
        root.querySelector('[data-source-body]').hidden = !asset;
        root.querySelectorAll('[data-source-action]').forEach(button => { button.disabled = !asset; });
        root.querySelectorAll('[data-source-drag]').forEach(handle => { handle.disabled = !asset; });
        if (!asset) return;
        const frames = SE().sourceFrames(s.project, asset), kind = asset.kind, action = name => root.querySelector(`[data-source-action="${name}"]`);
        action('insert').disabled = readonly;
        action('overwrite').disabled = readonly;
        action('go-in').disabled = s.sourceIn == null;
        action('go-out').disabled = s.sourceOut == null;
        action('clear-marks').disabled = s.sourceIn == null && s.sourceOut == null;
        root.querySelector('[data-source-name]').textContent = P.assetDisplayName(s, asset);
        root.querySelector('[data-source-length]').textContent = I().formatTime(frames, s.project);
        root.querySelector('[data-source-in-text]').textContent = s.sourceIn == null ? '-' : I().formatTime(s.sourceIn, s.project);
        root.querySelector('[data-source-out-text]').textContent = s.sourceOut == null ? '-' : I().formatTime(s.sourceOut, s.project);
        root.querySelector('[data-source-canvas]').hidden = kind === 'audio';
        root.querySelector('[data-source-wave]').hidden = kind !== 'audio';
        const mat = root.querySelector('.vs-source-mat'), width = Number(s.project.width || 1280), height = Number(s.project.height || 720);
        if (mat) { mat.hidden = kind === 'audio'; mat.style.aspectRatio = `${width} / ${height}`; mat.style.setProperty('--vs-ar', String(width / height)); }
        root.querySelector('[data-source-scrub]').max = String(Math.max(0, frames - 1));
        const picture = root.querySelector('[data-source-drag="picture"]'), sound = root.querySelector('[data-source-drag="sound"]');
        picture.disabled = readonly || kind === 'audio';
        sound.disabled = readonly || !hasSound(asset);
        picture.draggable = !picture.disabled;
        sound.draggable = !sound.disabled;
        renderSourceWave(s, asset);
        renderSourceTime(s);
    }

    // ---- Transport and marks ----
    function seekSource(s, frame) {
        const asset = currentAsset(s);
        if (!asset || !s.sourcePreview) return;
        s.sourceFrame = clamp(Math.round(Number(frame)) || 0, 0, lastFrame(s, asset));
        s.sourcePreview.seek(s.sourceFrame);
    }
    function stepSource(s, frames) {
        if (!currentAsset(s) || !s.sourcePreview) return;
        pauseSource(s);
        seekSource(s, s.sourceFrame + frames);
    }
    function toggleSourcePlay(s) {
        if (!currentAsset(s) || !s.sourcePreview) return;
        if (s.sourcePreview.isPlaying()) { pauseSource(s); return; }
        if (atSourceEnd(s)) s.sourcePreview.seek(0);
        s.sourcePreview.play();
    }
    // The in mark sits on the playhead frame; the out mark is exclusive (like the timeline out point) and stays after in.
    function setMark(s, which) {
        const asset = currentAsset(s);
        if (!asset) return;
        const frame = clamp(Math.round(s.sourceFrame) || 0, 0, lastFrame(s, asset));
        if (which === 'in') {
            s.sourceIn = frame;
            if (s.sourceOut != null && s.sourceOut <= frame) s.sourceOut = null;
        } else {
            s.sourceOut = clamp(Math.max(frame, (s.sourceIn || 0) + 1), 1, SE().sourceFrames(s.project, asset));
        }
        storeMarks(s);
        renderSource(s);
    }
    function goMark(s, which) {
        const frame = which === 'in' ? s.sourceIn : s.sourceOut;
        if (frame == null) return;
        pauseSource(s);
        seekSource(s, frame);
    }
    function clearMarks(s) {
        if (!currentAsset(s)) return;
        s.sourceIn = null; s.sourceOut = null;
        storeMarks(s);
        renderSource(s);
    }

    // ---- Source edits ----
    function reasonNotice(s, reason) {
        const notice = SOURCE_NOTICES[reason];
        if (notice) showNotice(s, notice[0], notice[1], false);
        else P.showReason(s, reason);
    }
    // Three-point edit from the source monitor (spec §4.7): `,` inserts and `.` overwrites on the target tracks. The
    // timeline in/out and the playhead decide the place; afterwards the playhead moves to the end of the edit.
    function sourceEdit(s, mode) {
        const asset = currentAsset(s);
        if (!asset || !s.project) return false;
        if (s.readonly) { showNotice(s, 'readonly', 'Read-only mode prevents editing.', true); return false; }
        const io = P.inOut(s), range = SE().sourceRange(s.project, asset, { in: s.sourceIn, out: s.sourceOut });
        const picked = SE().plan({ timelineIn: io.in, timelineOut: io.out, playhead: s.frame, still: asset.kind === 'image', range });
        if (picked.reason) { reasonNotice(s, picked.reason); return false; }
        const built = SE().placements(s.project, asset, { start: picked.start, duration: picked.duration, offset: picked.offset, targets: P.targetTracks(s), part: 'both' });
        if (built.reason) { reasonNotice(s, built.reason); return false; }
        const end = picked.start + picked.duration;
        const ok = applyEdit(s, mode === 'insert' ? 'source insert' : 'source overwrite', (project, out) => {
            out.created = built.created;
            return mode === 'insert' ? E().insertClips(project, picked.start, built.placements) : E().overwrite(project, built.placements);
        }, out => out.created);
        if (ok) { s.preview.seek(end); T().revealFrame(s.q('[data-timeline]'), s, end); }
        return ok;
    }
    // A picture or sound handle dropped on a lane (spec §4.7): the source range (marks, or the whole item) lands at the
    // drop point. The kind must match the lane: sound needs an audio lane, a picture needs a lane that takes the media.
    function dropSourcePart(s, asset, trackId, at, mode, part) {
        const track = (s.project.tracks || []).find(item => item.id === trackId);
        if (!track) return false;
        if (track.locked) { P.showReason(s, 'locked'); return false; }
        if (!T().accepts(track, asset) || (part === 'sound') !== (track.kind === 'audio')) { P.showReason(s, 'kind'); return false; }
        const marks = s.sourceAssetId === asset.id ? { in: s.sourceIn, out: s.sourceOut } : {};
        const range = SE().sourceRange(s.project, asset, marks), start = Math.max(0, Math.round(Number(at)) || 0);
        const built = SE().placements(s.project, asset, { start, duration: range.duration, offset: range.in, targets: { [track.kind]: track.id }, part });
        if (built.reason) { reasonNotice(s, built.reason); return false; }
        return applyEdit(s, mode === 'insert' ? 'source insert' : 'source overwrite', (project, out) => {
            out.created = built.created;
            return mode === 'insert' ? E().insertClips(project, start, built.placements) : E().overwrite(project, built.placements);
        }, out => out.created);
    }

    // ---- Keys (source mode routes every key here; see handleKeys in video-studio.js) ----
    function isSourceShortcut(s, event) {
        return s.stageMode === 'source' && !!event.altKey && !event.ctrlKey && !event.metaKey && String(event.key || '').toLowerCase() === 'x';
    }
    function handleSourceKeys(s, event) {
        const raw = String(event.key || ''), key = raw.length === 1 ? raw.toLowerCase() : raw;
        // Timeline shortcuts with Ctrl or Cmd (copy, link, go to …) pass the modifier gate but do nothing here.
        if (event.ctrlKey || event.metaKey) return false;
        if (event.altKey) {
            if (isSourceShortcut(s, event)) { event.preventDefault(); clearMarks(s); return true; }
            // Alt+arrows nudge clips on the timeline; here they must not navigate the browser history either.
            if (key === 'ArrowLeft' || key === 'ArrowRight') { event.preventDefault(); return true; }
            return false;
        }
        if (event.code === 'Space' || key === ' ') { event.preventDefault(); toggleSourcePlay(s); return true; }
        if (key === 'j' || key === 'k' || key === 'l') { event.preventDefault(); if (!event.repeat && s.sourceShuttle) s.sourceShuttle.press(key); return true; }
        const asset = currentAsset(s);
        if (!asset) return false;
        if (key === 'ArrowLeft' || key === 'ArrowRight') {
            event.preventDefault();
            stepSource(s, (key === 'ArrowLeft' ? -1 : 1) * (event.shiftKey ? Math.round(rateOf(s)) : 1));
            return true;
        }
        if (key === 'Home' || key === 'End') { event.preventDefault(); pauseSource(s); seekSource(s, key === 'Home' ? 0 : lastFrame(s, asset)); return true; }
        if (key === 'i' || key === 'o') {
            event.preventDefault();
            const which = key === 'i' ? 'in' : 'out';
            if (event.shiftKey) goMark(s, which); else setMark(s, which);
            return true;
        }
        if (key === ',' || key === '.') { event.preventDefault(); sourceEdit(s, key === ',' ? 'insert' : 'overwrite'); return true; }
        return false;
    }
    function runSourceAction(s, action) {
        const asset = currentAsset(s);
        if (action === 'play') toggleSourcePlay(s);
        else if (action === 'to-start') { pauseSource(s); seekSource(s, 0); }
        else if (action === 'to-end' && asset) { pauseSource(s); seekSource(s, lastFrame(s, asset)); }
        else if (action === 'step-back') stepSource(s, -1);
        else if (action === 'step-forward') stepSource(s, 1);
        else if (action === 'mark-in') setMark(s, 'in');
        else if (action === 'mark-out') setMark(s, 'out');
        else if (action === 'go-in') goMark(s, 'in');
        else if (action === 'go-out') goMark(s, 'out');
        else if (action === 'clear-marks') clearMarks(s);
        else if (action === 'insert') sourceEdit(s, 'insert');
        else if (action === 'overwrite') sourceEdit(s, 'overwrite');
    }

    // ---- Wiring: stage switch, source transport, drag handles, library double-click ----
    function wireSource(s) {
        const signal = s.abort.signal, view = s.q('[data-source]');
        s.q('[data-stage-switch]').addEventListener('click', event => {
            const button = event.target.closest('[data-stage-mode]');
            if (button) setStageMode(s, button.dataset.stageMode);
        }, { signal });
        view.addEventListener('click', event => {
            const button = event.target.closest('[data-source-action]');
            if (button && !button.disabled) { runSourceAction(s, button.dataset.sourceAction); return; }
            // A click on the picture gives the keys back to the editor, as a click on the timeline preview does.
            if (event.target.closest('.vs-source-stage')) s.app.focus({ preventScroll: true });
        }, { signal });
        view.addEventListener('input', event => {
            if (!event.target.matches('[data-source-scrub]')) return;
            pauseSource(s);
            seekSource(s, Number(event.target.value));
        }, { signal });
        view.addEventListener('dragstart', event => {
            const handle = event.target.closest('[data-source-drag]');
            if (!handle) return;
            const asset = currentAsset(s);
            if (!asset || s.readonly || handle.disabled) { event.preventDefault(); return; }
            event.dataTransfer.setData('application/x-video-studio-asset', asset.id);
            event.dataTransfer.setData('application/x-video-studio-part', handle.dataset.sourceDrag);
            event.dataTransfer.effectAllowed = 'copy';
        }, { signal });
        s.host.addEventListener('dblclick', event => {
            const card = event.target.closest('.vs-library [data-asset-id]');
            if (card && !event.target.closest('[data-place-asset]')) openSource(s, card.dataset.assetId);
        }, { signal });
        window.addEventListener('keyup', event => {
            const key = String(event.key || '').toLowerCase();
            // Released in every view, so a K held while the view changes cannot stay latched in the source shuttle.
            if (s.sourceShuttle && (key === 'j' || key === 'k' || key === 'l')) s.sourceShuttle.release(key);
        }, { signal });
        // Like the timeline shuttle: leaving the window releases a held K (playback goes on).
        window.addEventListener('blur', () => { if (s.sourceShuttle) s.sourceShuttle.release('k'); }, { signal });
        renderStageMode(s);
    }

    Object.assign(P, { openSource, closeSource, syncSource, disposeSource, setStageMode, renderSource, sourceMarks, sourceEdit, dropSourcePart, handleSourceKeys, isSourceShortcut, wireSource });
})();
