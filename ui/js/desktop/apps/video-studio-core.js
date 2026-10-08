(function () {
    'use strict';

    // Shared state machine: constants, requests, project model, history, saving, conflicts and drafts.
    const P = window.VideoStudioParts = window.VideoStudioParts || {};
    const loadProject = (...args) => P.loadProject(...args);
    const renderToolbar = (...args) => P.renderToolbar(...args);
    const renderUI = (...args) => P.renderUI(...args);

    const API = '/api/desktop/video-studio';
    const FPS = 30;
    const MAX_CLIPS = 256;
    const MAX_ACTIVE_ASSETS = 32;
    const MAX_PROJECT_FRAMES = FPS * 600;
    const CANVASES = [
        { width: 1280, height: 720, key: 'landscape720' }, { width: 1920, height: 1080, key: 'landscape1080' },
        { width: 720, height: 1280, key: 'portrait720' }, { width: 1080, height: 1920, key: 'portrait1080' },
        { width: 720, height: 720, key: 'square720' }, { width: 1080, height: 1080, key: 'square1080' }
    ];
    // Actions that never change the project and stay available in read-only mode.
    const READ_ONLY_SAFE = new Set(['play', 'step-back', 'step-forward', 'seek-start', 'seek-end', 'toggle-inspector', 'close-inspector', 'project-menu', 'jobs', 'shortcuts', 'dismiss-notice', 'fullscreen', 'filter', 'toggle-fine']);
    const TERMINAL = ['succeeded', 'failed', 'cancelled', 'interrupted'];
    const PREF_TIMELINE = 'aurago.videoStudio.timelineHeight';
    const PREF_SNAP = 'aurago.videoStudio.snap';
    const clone = value => JSON.parse(JSON.stringify(value));
    const clamp = (value, min, max) => Math.max(min, Math.min(max, value));
    const idempotencyKey = () => crypto.randomUUID ? crypto.randomUUID() : Date.now().toString(36) + '-' + Math.random().toString(36).slice(2);
    const T = () => window.VideoStudioTimeline;
    const I = () => window.VideoStudioInspector;
    function tr(s, key, fallback) {
        const value = s.ctx.t ? s.ctx.t('videoStudio.' + key) : '';
        return !value || value === 'videoStudio.' + key ? (fallback || key) : value;
    }
    function esc(s, value) { return s.ctx.esc(String(value == null ? '' : value)); }
    function icon(name, size, cls) { return window.VideoStudioIcons ? window.VideoStudioIcons.svg(name, size || 16, cls) : ''; }
    function readPref(key) { try { return window.localStorage.getItem(key); } catch (_) { return null; } }
    function writePref(key, value) { try { window.localStorage.setItem(key, String(value)); } catch (_) { /* per-viewer convenience only */ } }
    // Generated artwork files get readable names: the sticker's name or "Title".
    function assetDisplayName(s, asset) {
        const name = String(asset && asset.name || '');
        if (!window.VideoStudioMedia.isArtwork(asset)) return name;
        const sticker = /^sticker-([a-z]+)\.png$/i.exec(name);
        if (sticker) return tr(s, 'sticker_' + sticker[1].toLowerCase(), sticker[1]);
        if (/^title-[\w.-]+\.png$/i.test(name)) return tr(s, 'preset_title', 'Title');
        return name;
    }
    function uiLocale() { return document.documentElement.lang || navigator.language || 'en'; }
    function maxFrames(s) { return Number(s.status && s.status.limits && s.status.limits.max_duration_frames || MAX_PROJECT_FRAMES); }
    async function request(url, options) {
        const response = await fetch(url, Object.assign({ credentials: 'same-origin', cache: 'no-store' }, options || {}));
        const body = (response.headers.get('content-type') || '').includes('application/json') ? await response.json() : {};
        if (!response.ok) {
            const error = new Error(body.code || body.error || 'request_failed');
            error.status = response.status;
            error.body = body;
            throw error;
        }
        return { body, etag: response.headers.get('ETag') || body.etag || '' };
    }
    function canonicalProject(project) {
        const copy = clone(project);
        copy.assets = (copy.assets || []).map(({ id, name, path, kind, duration_frames, width, height, has_audio }) => ({ id, name, path, kind, duration_frames, width, height, has_audio }));
        copy.tracks = (copy.tracks || []).map(track => ({
            id: track.id, kind: track.kind, name: track.name, muted: !!track.muted, hidden: !!track.hidden, locked: !!track.locked,
            clips: (track.clips || []).map(clip => ({
                id: clip.id, asset_id: clip.asset_id, start: Math.round(clip.start), offset: Math.round(clip.offset || 0), duration: Math.round(clip.duration),
                x: Number(clip.x || 0), y: Number(clip.y || 0), width: Number(clip.width || 1), height: Number(clip.height || 1),
                rotation: Number(clip.rotation || 0), opacity: Number(clip.opacity == null ? 1 : clip.opacity), volume: Number(clip.volume == null ? 1 : clip.volume),
                fade_in: Math.round(clip.fade_in || 0), fade_out: Math.round(clip.fade_out || 0), fit: clip.fit === 'cover' ? 'cover' : 'contain',
                text: String(clip.text || ''), text_style: normalizeTextStyle(clip.text_style), transition: clip.transition || null
            }))
        }));
        return copy;
    }
    // Mirrors the server's omitempty JSON (no false, 0 or empty values; stable key order), so a saved
    // project compares equal to the copy the server sends back and a refresh does not report a conflict.
    function normalizeTextStyle(style) {
        if (!style || typeof style !== 'object') return null;
        const out = {};
        Object.keys(style).sort().forEach(key => { const value = style[key]; if (value !== '' && value !== 0 && value !== false && value != null) out[key] = value; });
        return out;
    }
    function hydrateProject(s, project) {
        const p = clone(project || {});
        p.assets = (p.assets || []).map(asset => Object.assign({}, asset, {
            media_url: API + '/projects/' + encodeURIComponent(s.projectId) + '/media/' + encodeURIComponent(asset.id)
        }));
        p.tracks = p.tracks || [];
        p.assets = p.assets || [];
        return p;
    }
    function trackName(s, tracks, kind) {
        const base = tr(s, kind + 'Track', kind === 'audio' ? 'Audio' : kind === 'overlay' ? 'Overlay' : 'Video');
        const names = new Set(tracks.map(track => track.name));
        let n = 1;
        while (names.has(base + ' ' + n)) n++;
        return base + ' ' + n;
    }
    function makeTrack(s, tracks, kind) {
        return { id: T().newId('track'), kind, name: trackName(s, tracks, kind), muted: false, hidden: false, locked: false, clips: [] };
    }
    function defaultTracks(s) {
        const tracks = [];
        ['video', 'overlay', 'audio'].forEach(kind => tracks.push(makeTrack(s, tracks, kind)));
        return tracks;
    }
    function defaultProject(name, s) {
        return { version: 1, name, width: 1280, height: 720, fps: FPS, assets: [], tracks: defaultTracks(s) };
    }
    function showNotice(s, key, fallback, error, action) {
        if (s.disposed || !s.q('[data-notice]')) return;
        const el = s.q('[data-notice]');
        if (P.syncDrawer) P.syncDrawer(s);
        el.hidden = false;
        el.classList.toggle('is-error', !!error);
        el.innerHTML = `<span class="vs-toast-icon">${icon(error ? 'alert' : 'check', 17)}</span><span class="vs-toast-text" data-notice-text></span>${action ? `<a class="vs-toast-action" href="${esc(s, action.href)}" download="${esc(s, action.download || '')}">${icon('download', 15)}${esc(s, action.label)}</a>` : ''}<button type="button" class="vs-toast-close" data-action="dismiss-notice" aria-label="${esc(s, tr(s, 'close', 'Close'))}">${icon('close', 15)}</button>`;
        el.querySelector('[data-notice-text]').textContent = tr(s, key, fallback);
        clearTimeout(s.toastTimer);
        if (!error) s.toastTimer = window.setTimeout(() => clearNotice(s), action ? 15000 : 5000);
    }
    function clearNotice(s) { const el = s.q('[data-notice]'); if (el) { el.hidden = true; el.replaceChildren(); } clearTimeout(s.toastTimer); }
    function projectDuration(s) { return s.project ? T().projectEnd(s.project) : 0; }
    function snapshot(s) { return clone(s.project); }
    // A text edit records its pre-edit state once: when the field commits or another edit starts.
    function flushTextBefore(s) {
        if (!s.textBefore) return;
        s.history.push(s.textBefore.snapshot);
        if (s.history.length > 50) s.history.shift();
        s.redo = [];
        s.textBefore = null;
    }
    function recordChange(s, label, before) {
        if (s.readonly) { if (before) s.project = hydrateProject(s, before); return renderUI(s); }
        flushTextBefore(s);
        T().repairTransitions(s.project);
        if (T().validTimeline && !T().validTimeline(s.project)) {
            if (before) s.project = hydrateProject(s, before);
            showNotice(s, 'invalidTiming', 'That edit would create invalid timing or an overlap.', true);
            return renderUI(s);
        }
        if (before) s.history.push(before);
        else s.history.push(snapshot(s));
        if (s.history.length > 50) s.history.shift();
        s.redo = [];
        scheduleSave(s);
        renderUI(s);
    }
    function mutate(s, label, fn) {
        if (!s.project || s.readonly || s.timelineDragging) return false;
        flushTextBefore(s);
        const before = snapshot(s), selection = [s.selectedClipId, s.selectionRevision];
        const restore = () => { s.project = hydrateProject(s, before); [s.selectedClipId, s.selectionRevision] = selection; };
        if (fn(s.project) === false) { restore(); return false; }
        T().repairTransitions(s.project);
        if (T().validTimeline && !T().validTimeline(s.project)) {
            restore();
            showNotice(s, 'invalidTiming', 'That edit would create invalid timing or an overlap.', true);
            renderUI(s);
            return false;
        }
        s.history.push(before);
        if (s.history.length > 50) s.history.shift();
        s.redo = [];
        s.dirty = true;
        s.revision++;
        persistDraft(s);
        scheduleSave(s);
        renderUI(s);
        if (s.preview) s.preview.seek(s.frame);
        return true;
    }
    function undo(s, redo) {
        if (s.readonly || s.timelineDragging || !s.project) return;
        flushTextBefore(s);
        const source = redo ? s.redo : s.history;
        const target = redo ? s.history : s.redo;
        if (!source.length) return;
        target.push(snapshot(s));
        s.project = hydrateProject(s, source.pop());
        if (s.selectedClipId && !T().clipFor(s.project, s.selectedClipId)) s.selectedClipId = '';
        P.resyncArtwork(s);
        s.dirty = true; s.revision++;
        persistDraft(s); scheduleSave(s); renderUI(s); s.preview.seek(s.frame);
    }
    function activeAssetCount(project) { return new Set(project.tracks.flatMap(track => track.clips.map(clip => clip.asset_id))).size; }
    function totalClipCount(project) { return project.tracks.reduce((n, track) => n + track.clips.length, 0); }
    function nextStart(track) { return Math.max(0, ...track.clips.map(clip => clip.start + clip.duration)); }
    function addTrackTo(s, project, kind) {
        const track = makeTrack(s, project.tracks, kind);
        project.tracks.splice(I().insertIndexFor(project.tracks, kind), 0, track);
        return track;
    }
    function freeStart(track, requested, duration) {
        let start = requested;
        for (const occupied of track.clips.slice().sort((a, b) => a.start - b.start)) {
            if (start + duration <= occupied.start) break;
            if (start < occupied.start + occupied.duration) start = occupied.start + occupied.duration;
        }
        return start;
    }
    function scheduleSave(s) {
        s.dirty = true; s.revision++;
        persistDraft(s);
        clearTimeout(s.autosaveTimer);
        if (!s.timelineDragging && !s.conflict) s.autosaveTimer = window.setTimeout(() => saveProject(s).catch(() => {}), 1100);
        renderToolbar(s);
    }
    function queueAutosave(s) {
        clearTimeout(s.autosaveTimer);
        s.autosaveTimer = window.setTimeout(() => { s.autosaveTimer = 0; saveProject(s).catch(() => {}); }, 1100);
    }
    async function saveProject(s) {
        if (s.timelineDragging) return false;
        if (!s.project || !s.projectId || !s.dirty) return true;
        if (s.readonly) { showNotice(s, 'readonly', 'Read-only mode prevents editing.', true); return false; }
        if (T().validTimeline && !T().validTimeline(s.project)) {
            showNotice(s, 'invalidTiming', 'That edit would create invalid timing or an overlap.', true);
            return false;
        }
        clearTimeout(s.autosaveTimer);
        if (s.savePromise) return s.savePromise;
        s.saving = true; s.saveError = ''; s.saveGeneration = (s.saveGeneration || 0) + 1; renderToolbar(s);
        const revision = s.revision, projectId = s.projectId, epoch = s.projectEpoch;
        const token = {};
        s.saveToken = token;
        let requestBody = canonicalProject(s.project);
        s.savePromise = (async () => {
            try {
                let response;
                for (let attempt = 0; !response; attempt++) {
                    try {
                        response = await request(API + '/projects/' + encodeURIComponent(projectId), {
                            method: 'PUT', headers: { 'Content-Type': 'application/json', 'If-Match': s.etag }, body: JSON.stringify(requestBody)
                        });
                    } catch (error) {
                        const stale = error.status === 412 || error.message === 'file_conflict';
                        if (!stale || attempt > 0 || !(await adoptServerMedia(s, projectId, epoch))) throw error;
                        requestBody = canonicalProject(s.project);
                    }
                }
                if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return true;
                s.etag = response.etag || s.etag;
                s.desktopPath = response.body.desktop_path || s.desktopPath;
                s.savedProject = requestBody;
                s.dirty = revision !== s.revision;
                if (!s.dirty) await clearDraft(s);
                return true;
            } catch (error) {
                if (error.status === 412 || error.status === 428 || error.message === 'file_conflict') {
                    s.conflict = true; await showConflict(s);
                    return !s.dirty && !s.conflict;
                }
                s.saveError = error.message; showNotice(s, error.status === 403 ? 'readonly' : 'saveFailed', error.status === 403 ? 'Read-only mode prevents editing.' : 'Changes could not be saved. Check your connection and try again.', true);
                return false;
            } finally {
                if (s.saveToken === token) {
                    s.saveToken = null;
                    s.saving = false; s.savePromise = null;
                    if (!s.disposed && epoch === s.projectEpoch && projectId === s.projectId) {
                        renderToolbar(s);
                        if (s.dirty && !s.conflict) scheduleSave(s);
                    }
                }
            }
        })();
        return s.savePromise;
    }
    // Imports change the server copy (new or probed assets) and its ETag. When the server copy differs from
    // our last save only in its media list, adopt that list and the observed ETag; anything else is a conflict.
    async function adoptServerMedia(s, projectId, epoch) {
        if (!s.savedProject) return false;
        const fresh = await request(API + '/projects/' + encodeURIComponent(projectId), { signal: s.abort.signal });
        if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId || !fresh.etag) return false;
        const next = hydrateProject(s, fresh.body.project || fresh.body);
        const content = project => { const canonical = canonicalProject(project); canonical.assets = []; return JSON.stringify(canonical); };
        if (content(next) !== content(s.savedProject)) return false;
        const known = new Map((s.project.assets || []).map(asset => [asset.id, asset]));
        next.assets.forEach(asset => { const local = known.get(asset.id); if (local) Object.assign(local, asset); });
        s.project.assets = (s.project.assets || []).concat(next.assets.filter(asset => !known.has(asset.id)));
        s.savedProject.assets = next.assets.map(({ media_url, ...asset }) => asset);
        s.etag = fresh.etag;
        return true;
    }
    async function showConflict(s) {
        const host = s.q('[data-conflict-host]');
        if (!host) return;
        host.innerHTML = `<div class="vs-modal-backdrop"><section class="vs-modal" role="dialog" aria-modal="true" aria-labelledby="vs-conflict-title-${esc(s, s.id)}"><span class="vs-modal-icon">${icon('alert', 22)}</span><h2 id="vs-conflict-title-${esc(s, s.id)}">${esc(s, tr(s, 'conflictTitle', 'This project changed elsewhere'))}</h2><p>${esc(s, tr(s, 'conflictCopy', 'Choose which version to keep before continuing.'))}</p><div class="vs-modal-actions vs-modal-actions-stack"><button type="button" data-conflict="reload">${esc(s, tr(s, 'reloadServer', 'Load server version'))}</button><button type="button" data-conflict="replace">${esc(s, tr(s, 'replaceServer', 'Replace the latest version'))}</button><button type="button" class="vs-primary" data-conflict="keep">${esc(s, tr(s, 'keepEditing', 'Keep editing'))}</button></div></section></div>`;
        host.querySelector('[data-conflict="keep"]')?.focus();
        const choice = await new Promise(resolve => {
            const handler = event => {
                const button = event.target.closest('[data-conflict]'); if (!button) return;
                host.removeEventListener('click', handler); host.replaceChildren(); resolve(button.dataset.conflict);
            };
            host.addEventListener('click', handler);
        });
        s.conflict = choice === 'keep';
        if (choice === 'reload') {
            try { await loadProject(s, s.projectId); clearNotice(s); }
            catch (_) { showNotice(s, 'loadFailed', 'Could not load the current server version.', true); }
        } else if (choice === 'replace') {
            try {
                const fresh = await request(API + '/projects/' + encodeURIComponent(s.projectId), { signal: s.abort.signal });
                const local = canonicalProject(s.project), revision = s.revision, epoch = s.projectEpoch;
                const replaced = await request(API + '/projects/' + encodeURIComponent(s.projectId), { method: 'PUT', headers: { 'Content-Type': 'application/json', 'If-Match': fresh.etag }, body: JSON.stringify(local) });
                if (s.disposed || epoch !== s.projectEpoch) return;
                s.etag = replaced.etag || fresh.etag;
                s.savedProject = local;
                s.dirty = revision !== s.revision;
                if (!s.dirty) await clearDraft(s);
                clearNotice(s);
                renderToolbar(s);
            } catch (_) { s.conflict = true; showNotice(s, 'conflictAgain', 'The project changed again. Review the conflict and try again.', true); }
        } else renderToolbar(s);
    }
    async function saveUntilClean(s) {
        const projectId = s.projectId, epoch = s.projectEpoch;
        while (s.dirty && !s.disposed && !s.conflict) {
            if (s.timelineDragging || projectId !== s.projectId || epoch !== s.projectEpoch) return false;
            if (!(await saveProject(s))) return false;
        }
        return !s.dirty && !s.conflict && !s.disposed && projectId === s.projectId && epoch === s.projectEpoch;
    }
    function draftKey(s) { return 'video-studio-draft:' + s.projectId; }
    function persistDraft(s) {
        if (!s.projectId || !s.project) return;
        const data = JSON.stringify({ saved_at: Date.now(), project: canonicalProject(s.project) });
        if (data.length > 900 * 1024) return;
        try { localStorage.setItem(draftKey(s), data); } catch (_) { /* project remains in memory and server autosave continues */ }
    }
    async function recoverDraft(s, expectedProjectId, expectedEpoch) {
        const projectId = expectedProjectId || s.projectId;
        const epoch = expectedEpoch == null ? s.projectEpoch : expectedEpoch;
        if (s.readonly) return;
        let raw;
        try { raw = localStorage.getItem('video-studio-draft:' + projectId); } catch (_) { return; }
        if (!raw) return;
        try {
            const draft = JSON.parse(raw);
            const shouldRecover = draft.project && Date.now() - Number(draft.saved_at || 0) < 7 * 86400000 && await s.ctx.confirmDialog(tr(s, 'recoverTitle', 'Recover local draft?'), tr(s, 'recoverCopy', 'A recent local draft is available. Restore it over the saved project?'));
            if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
            if (shouldRecover) {
                const saved = clone(s.project), recovered = hydrateProject(s, draft.project);
                // The server may hold newer media than the draft; keep every asset the server knows.
                const known = new Set(recovered.assets.map(asset => asset.id));
                recovered.assets = recovered.assets.concat(saved.assets.filter(asset => !known.has(asset.id)));
                s.project = recovered; s.dirty = true; s.history = [saved]; s.redo = [];
                // A draft title whose words or style differ from the saved copy still shows the saved PNG.
                recovered.tracks.forEach(track => track.clips.forEach(clip => {
                    const previous = clip.text && T().clipFor(saved, clip.id);
                    if (previous && previous.clip.asset_id === clip.asset_id && (previous.clip.text !== clip.text || P.textStyleKey(previous.clip.text_style || {}) !== P.textStyleKey(clip.text_style || {}))) P.markTextStale(s, clip.id);
                }));
            }
        } catch (_) { /* malformed recovery data is ignored */ }
    }
    async function clearDraft(s) { try { localStorage.removeItem(draftKey(s)); } catch (_) {} }
    function selectedClip(s) { return s.project ? T().clipFor(s.project, s.selectedClipId) : null; }

    Object.assign(P, { uiLocale, assetDisplayName, API, FPS, MAX_CLIPS, MAX_ACTIVE_ASSETS, MAX_PROJECT_FRAMES, CANVASES, READ_ONLY_SAFE, TERMINAL, PREF_TIMELINE, PREF_SNAP, clone, clamp, idempotencyKey, T, I, tr, esc, icon, readPref, writePref, maxFrames, request, canonicalProject, hydrateProject, trackName, makeTrack, defaultTracks, defaultProject, showNotice, clearNotice, projectDuration, snapshot, recordChange, mutate, undo, activeAssetCount, totalClipCount, nextStart, addTrackTo, freeStart, scheduleSave, queueAutosave, saveProject, showConflict, saveUntilClean, draftKey, persistDraft, recoverDraft, clearDraft, selectedClip, flushTextBefore, normalizeTextStyle, adoptServerMedia });
})();
