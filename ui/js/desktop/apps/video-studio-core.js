(function () {
    'use strict';

    // Shared state machine: constants, requests, project model, history, saving, conflicts and drafts.
    const P = window.VideoStudioParts = window.VideoStudioParts || {};
    const loadProject = (...args) => P.loadProject(...args);
    const renderToolbar = (...args) => P.renderToolbar(...args);
    const renderUI = (...args) => P.renderUI(...args);

    const API = '/api/desktop/video-studio';
    const MAX_CLIPS = 2000;
    const CANVASES = [
        { width: 1280, height: 720, key: 'landscape720' }, { width: 1920, height: 1080, key: 'landscape1080' },
        { width: 2560, height: 1440, key: 'landscape1440' }, { width: 3840, height: 2160, key: 'landscape2160' },
        { width: 720, height: 1280, key: 'portrait720' }, { width: 1080, height: 1920, key: 'portrait1080' },
        { width: 2160, height: 3840, key: 'portrait2160' },
        { width: 720, height: 720, key: 'square720' }, { width: 1080, height: 1080, key: 'square1080' },
        { width: 2160, height: 2160, key: 'square2160' }, { width: 1080, height: 1350, key: 'feed1080' }
    ];
    // Actions that never change the project and stay available in read-only mode.
    const READ_ONLY_SAFE = new Set(['play', 'step-back', 'step-forward', 'seek-start', 'seek-end', 'toggle-inspector', 'close-inspector', 'project-menu', 'jobs', 'shortcuts', 'dismiss-notice', 'fullscreen', 'filter', 'toggle-fine', 'loop', 'goto-time', 'media-view', 'media-sort', 'all-projects']);
    const TERMINAL = ['succeeded', 'failed', 'cancelled', 'interrupted'];
    const PREF_TIMELINE = 'aurago.videoStudio.timelineHeight';
    const PREF_SNAP = 'aurago.videoStudio.snap';
    const PREF_QUALITY = 'aurago.videoStudio.previewQuality';
    const PREF_MEDIA_VIEW = 'aurago.videoStudio.mediaView';
    const PREF_MEDIA_SORT = 'aurago.videoStudio.mediaSort';
    // Colour labels: one palette, defined in video-studio-timeline.js (VideoStudioTimeline.LABEL_COLORS).
    const LABEL_COLORS = window.VideoStudioTimeline.LABEL_COLORS;
    const PREF_MAGNETIC = 'aurago.videoStudio.magnetic';
    const clone = value => JSON.parse(JSON.stringify(value));
    const clamp = (value, min, max) => Math.max(min, Math.min(max, value));
    const idempotencyKey = () => crypto.randomUUID ? crypto.randomUUID() : Date.now().toString(36) + '-' + Math.random().toString(36).slice(2);
    const T = () => window.VideoStudioTimeline;
    const I = () => window.VideoStudioInspector;
    function tr(s, key, fallback) {
        const value = s.ctx.t ? s.ctx.t('videoStudio.' + key) : '';
        return !value || value === 'videoStudio.' + key ? (fallback || key) : value;
    }
    // Count wording, as in the other Desktop apps (GalleryLibrary.countLabel): the `<key>_one` string for exactly one,
    // the key itself for every other count. {{count}} and the other placeholders in vars are filled in.
    function trCount(s, key, count, one, other, vars) {
        const n = Number(count) || 0;
        const text = n === 1 ? tr(s, key + '_one', one) : tr(s, key, other);
        return Object.entries(Object.assign({ count: n }, vars)).reduce((out, [name, value]) => out.split('{{' + name + '}}').join(String(value)), text);
    }
    function esc(s, value) { return s.ctx.esc(String(value == null ? '' : value)); }
    function icon(name, size, cls) { return window.VideoStudioIcons ? window.VideoStudioIcons.svg(name, size || 16, cls) : ''; }
    function readPref(key) { try { return window.localStorage.getItem(key); } catch (_) { return null; } }
    function writePref(key, value) { try { window.localStorage.setItem(key, String(value)); } catch (_) { /* per-viewer convenience only */ } }
    // Generated artwork files get readable names: the sticker's name or "Title".
    function assetDisplayName(s, asset) {
        const name = String(asset && asset.name || '');
        if (!window.VideoStudioMedia.isArtwork(asset)) return name;
        const sticker = /^sticker-([a-z-]+)\.png$/i.exec(name);
        if (sticker && window.AuraStickerLibrary?.byId.has(sticker[1].toLowerCase())) return s.ctx.t('stickers.name.' + sticker[1].toLowerCase());
        if (sticker) return tr(s, 'sticker_' + sticker[1].toLowerCase(), sticker[1]);
        if (/^title-[\w.-]+\.png$/i.test(name)) return tr(s, 'preset_title', 'Title');
        return name;
    }
    function uiLocale() { return document.documentElement.lang || navigator.language || 'en'; }
    function maxFrames(s) { return window.VideoStudioTime.maxFrames(s.project); }
    function rateOf(s) { return window.VideoStudioTime.rate(s.project); }
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
    // Go struct order, so a canonical copy and a server-ordered copy stringify the same.
    // folder (project-owned) and has_poster (server-owned) appear only when set, like Go's omitempty JSON.
    const ASSET_FIELDS = ['id', 'name', 'folder', 'path', 'kind', 'duration_frames', 'duration_us', 'video_duration_us', 'width', 'height', 'has_audio', 'has_poster', 'source_fps', 'codec', 'size_bytes'];
    // Optional fields (version 2 metadata, folder, poster flag); like Go's omitempty JSON they appear only when set.
    const OPTIONAL_ASSET_FIELDS = new Set(['folder', 'has_poster', 'duration_us', 'video_duration_us', 'source_fps', 'codec', 'size_bytes']);
    function canonicalAsset(asset) {
        const out = {};
        ASSET_FIELDS.forEach(key => { if (!OPTIONAL_ASSET_FIELDS.has(key) || asset[key]) out[key] = asset[key]; });
        return out;
    }
    // Version 2 clip fields after transition, in Go's order. Each appears only when set (omitempty): speed 1 is
    // removed by the editor, and an enabled, unlinked, labelless clip carries none of them.
    function canonicalClipExtras(clip) {
        const out = {};
        if (clip.speed != null && Number(clip.speed) !== 1) out.speed = Number(clip.speed);
        if (clip.link) out.link = String(clip.link);
        if (clip.audio_detached) out.audio_detached = true;
        if (clip.disabled) out.disabled = true;
        if (clip.label) out.label = String(clip.label);
        return out;
    }
    // Markers and the in/out points follow Go's omitempty JSON: markers only when there are some, each field only when
    // set, and the in/out points whenever they are numbers (0 is a set value, as Go's pointer fields are).
    function canonicalMarker(marker) {
        const out = { id: marker.id, frame: Math.round(Number(marker.frame) || 0) };
        if (Number(marker.duration) > 0) out.duration = Math.round(Number(marker.duration));
        if (marker.name) out.name = String(marker.name);
        if (marker.color) out.color = String(marker.color);
        if (marker.note) out.note = String(marker.note);
        return out;
    }
    function timingExtras(source) {
        const out = {};
        if ((source.markers || []).length) out.markers = source.markers.map(canonicalMarker);
        if (Number.isFinite(source.in_point)) out.in_point = source.in_point;
        if (Number.isFinite(source.out_point)) out.out_point = source.out_point;
        return out;
    }
    // The top level is built explicitly in the order of Go's videostudio.Project (version, name, width, height, fps,
    // ntsc when set, assets, tracks), whatever order the keys were added to the live project (rescaleProject appends
    // ntsc late). New Project fields must be inserted here at their Go struct position; unknown keys are dropped
    // (markers, in_point and out_point follow, from `timingExtras`; folders comes last).
    function canonicalProject(project) {
        const source = clone(project);
        const copy = Object.assign({
            version: source.version, name: source.name, width: source.width, height: source.height, fps: source.fps
        }, source.ntsc ? { ntsc: true } : {}, {
            assets: (source.assets || []).map(canonicalAsset),
            tracks: source.tracks || []
        }, timingExtras(source));
        copy.tracks = copy.tracks.map(track => ({
            id: track.id, kind: track.kind, name: track.name, muted: !!track.muted, hidden: !!track.hidden, locked: !!track.locked,
            clips: (track.clips || []).map(clip => Object.assign({
                id: clip.id, asset_id: clip.asset_id, start: Math.round(clip.start), offset: Math.round(clip.offset || 0), duration: Math.round(clip.duration),
                x: Number(clip.x || 0), y: Number(clip.y || 0), width: Number(clip.width || 1), height: Number(clip.height || 1),
                rotation: Number(clip.rotation || 0), opacity: Number(clip.opacity == null ? 1 : clip.opacity), volume: Number(clip.volume == null ? 1 : clip.volume),
                fade_in: Math.round(clip.fade_in || 0), fade_out: Math.round(clip.fade_out || 0), fit: clip.fit === 'cover' ? 'cover' : 'contain',
                text: String(clip.text || ''), text_style: normalizeTextStyle(clip.text_style), transition: clip.transition || null
            }, canonicalClipExtras(clip)))
        }));
        // project.folders comes last in Go's Project struct; empty folders are omitted like the server omits them.
        if (Array.isArray(source.folders) && source.folders.length) copy.folders = source.folders.map(String);
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
    // Mirrors Go's videostudio.AssetAtRate. Server assets carry duration_frames at the rate stored on the server; before
    // one joins a project whose rate was changed (and not saved yet) it is expressed in that project's rate again.
    function assetAtProjectRate(asset, project) {
        if (!asset || asset.kind === 'image' || !(Number(asset.duration_us) > 0)) return asset;
        return Object.assign({}, asset, { duration_frames: window.VideoStudioTime.framesAt(asset.duration_us, project) });
    }
    // Name and folder belong to the project document. A server copy refreshes every other field; for an asset the editor
    // already knows, the local name and folder win (the server took them from the document it saved). A top-level asset
    // has no folder key, like the server's omitempty JSON.
    function keepOwnedAssetFields(local, server) {
        const out = Object.assign({}, server, { name: local.name || server.name });
        if (local.folder) out.folder = local.folder; else delete out.folder;
        return out;
    }
    function hydrateProject(s, project) {
        const p = clone(project || {});
        // Mirrors Go's videostudio.Upgrade: a recovered draft or a tab opened before the upgrade compares as version 2.
        if (Number(p.version) === 1 && (Number(p.fps) || 30) === 30 && !p.ntsc) p.version = 2;
        p.assets = (p.assets || []).map(asset => assetAtProjectRate(Object.assign({}, asset, {
            media_url: API + '/projects/' + encodeURIComponent(s.projectId) + '/media/' + encodeURIComponent(asset.id)
        }), p));
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
        return { version: 2, name, width: 1280, height: 720, fps: 30, assets: [], tracks: defaultTracks(s) };
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
    // s.mediaRemoving counts running media removals (video-studio-mediabin.js): while the server deletes the files, edits
    // and undo/redo wait, so no step can bring back media whose file is about to be gone.
    function mutate(s, label, fn) {
        if (!s.project || s.readonly || s.timelineDragging || s.mediaRemoving) return false;
        flushTextBefore(s);
        // The multi-selection is restored too: edits set it inside fn and a refusal must not keep it.
        const before = snapshot(s), selection = [s.selectedClipId, s.selectionRevision, s.selectionSet, s.selectionPrimary];
        const restore = () => { s.project = hydrateProject(s, before); [s.selectedClipId, s.selectionRevision, s.selectionSet, s.selectionPrimary] = selection; };
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
        if (s.readonly || s.timelineDragging || !s.project || s.mediaRemoving) return;
        flushTextBefore(s);
        const source = redo ? s.redo : s.history;
        const target = redo ? s.history : s.redo;
        if (!source.length) return;
        target.push(snapshot(s));
        const previous = s.project;
        s.project = hydrateProject(s, source.pop());
        // The playhead keeps its time when an undo or redo changes the frame rate.
        s.frame = window.VideoStudioTime.frameAtRate(s.frame, previous, s.project);
        if (s.selectedClipId && !T().clipFor(s.project, s.selectedClipId)) s.selectedClipId = '';
        P.resyncArtwork(s);
        s.dirty = true; s.revision++;
        persistDraft(s); scheduleSave(s); renderUI(s); s.preview.seek(s.frame);
    }
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
                        // One retry: a stale ETag whose server copy differs only in its media (adoptServerMedia), or a
                        // media list that still names media removed in another window (forgetServerRemovedMedia).
                        const stale = error.status === 412 || error.message === 'file_conflict';
                        if (attempt > 0 || !(stale || mediaGone(error))) throw error;
                        if (!(await (stale ? adoptServerMedia(s, projectId, epoch) : forgetServerRemovedMedia(s, projectId, epoch)))) throw error;
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
                // Media removed elsewhere that a clip still uses cannot be saved by retrying: like a conflict, autosave
                // pauses and the user chooses which version to keep.
                if (error.status === 412 || error.status === 428 || error.message === 'file_conflict' || mediaGone(error)) {
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
    const mediaGone = error => !!error && error.status === 400 && error.message === 'project_asset_invalid';
    // The media a save may list: every media record of the project (GET .../media). It includes imports whose probe has
    // not finished, which the saved document does not list yet. Null when the list could not be read.
    async function serverMediaIds(s, projectId) {
        try {
            const response = await request(API + '/projects/' + encodeURIComponent(projectId) + '/media', { signal: s.abort.signal });
            const media = response.body && response.body.media;
            return Array.isArray(media) ? new Set(media.map(item => item && item.asset && item.asset.id).filter(Boolean)) : null;
        } catch (_) { return null; }
    }
    const clipsUse = (project, ids) => (project.tracks || []).some(track => (track.clips || []).some(clip => ids.has(clip.asset_id)));
    // Assets of a project (or history snapshot) that the server no longer has: media removed in another window.
    const removedIds = (project, serverIds) => new Set((project.assets || []).filter(asset => !serverIds.has(asset.id)).map(asset => asset.id));
    // Media removed elsewhere leaves the project when no clip uses it, and the undo and redo snapshots that do not use it
    // drop it too. A clip of removed media is a conflict the user resolves: nothing changes and the result is false.
    function dropRemovedMedia(s, project, serverIds, withHistory) {
        const gone = removedIds(project, serverIds);
        if (gone.size && clipsUse(project, gone)) return false;
        if (gone.size) project.assets = project.assets.filter(asset => !gone.has(asset.id));
        if (withHistory) {
            [s.history, s.redo].forEach(list => (list || []).forEach(snapshot => {
                const old = removedIds(snapshot, serverIds);
                if (old.size && !clipsUse(snapshot, old)) snapshot.assets = snapshot.assets.filter(asset => !old.has(asset.id));
            }));
        }
        return true;
    }
    // Removes assets and the clips that use them from a project snapshot.
    function stripAssets(project, ids) {
        project.assets = (project.assets || []).filter(asset => !ids.has(asset.id));
        (project.tracks || []).forEach(track => { track.clips = (track.clips || []).filter(clip => !ids.has(clip.asset_id)); });
        T().repairTransitions(project);
        return project;
    }
    // Deleted media cannot come back: the project and its undo and redo snapshots drop it with its clips, so an undo
    // never restores a clip whose file is gone (the server would refuse to save it). A step that no longer changes
    // anything leaves the history. Returns whether the project itself changed.
    function forgetMedia(s, ids) {
        if (!ids.size || !s.project) return false;
        const key = project => JSON.stringify(canonicalProject(project));
        const before = key(s.project);
        stripAssets(s.project, ids);
        if (s.selectedClipId && !T().clipFor(s.project, s.selectedClipId)) s.selectedClipId = '';
        const dedupe = list => {
            let neighbour = key(s.project);
            for (let i = list.length - 1; i >= 0; i--) {
                const current = key(list[i]);
                if (current === neighbour) list.splice(i, 1); else neighbour = current;
            }
            return list;
        };
        s.history = dedupe(s.history.map(snapshot => stripAssets(snapshot, ids)));
        s.redo = dedupe(s.redo.map(snapshot => stripAssets(snapshot, ids)));
        return key(s.project) !== before;
    }
    // A save named media that is gone (400 project_asset_invalid). True when unused removed media left the project, so
    // the save can be retried once; false when a clip still uses it or the media list is unavailable.
    async function forgetServerRemovedMedia(s, projectId, epoch) {
        const serverIds = await serverMediaIds(s, projectId);
        if (!serverIds || s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return false;
        const before = s.project.assets.length;
        return dropRemovedMedia(s, s.project, serverIds, true) && s.project.assets.length < before;
    }
    // Imports change the server copy (new or probed assets) and its ETag. When the server copy differs from
    // our last save only in its media list, adopt that list and the observed ETag; anything else is a conflict.
    // Media removed in another window leaves the local copy when no clip uses it; a clip that still does is a conflict.
    async function adoptServerMedia(s, projectId, epoch) {
        if (!s.savedProject) return false;
        const fresh = await request(API + '/projects/' + encodeURIComponent(projectId), { signal: s.abort.signal });
        if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId || !fresh.etag) return false;
        const next = hydrateProject(s, fresh.body.project || fresh.body);
        const content = project => { const canonical = canonicalProject(project); canonical.assets = []; return JSON.stringify(canonical); };
        if (content(next) !== content(s.savedProject)) return false;
        const serverIds = await serverMediaIds(s, projectId);
        if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return false;
        if (serverIds) {
            next.assets.forEach(asset => serverIds.add(asset.id));
            if (!dropRemovedMedia(s, s.project, serverIds, true)) return false;
        }
        const known = new Map((s.project.assets || []).map(asset => [asset.id, asset]));
        // The server counts frames at its saved rate; the local project may already be at a new, unsaved one.
        next.assets.forEach(asset => { const local = known.get(asset.id); if (local) Object.assign(local, keepOwnedAssetFields(local, assetAtProjectRate(asset, s.project))); });
        s.project.assets = (s.project.assets || []).concat(next.assets.filter(asset => !known.has(asset.id)).map(asset => assetAtProjectRate(asset, s.project)));
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
            // The local version is given up, so its draft is too: reopening must not offer it again.
            await clearDraft(s);
            try { await loadProject(s, s.projectId); clearNotice(s); }
            catch (_) { showNotice(s, 'loadFailed', 'Could not load the current server version.', true); }
        } else if (choice === 'replace') {
            const projectId = s.projectId, epoch = s.projectEpoch;
            try {
                const fresh = await request(API + '/projects/' + encodeURIComponent(projectId), { signal: s.abort.signal });
                // Media deleted elsewhere cannot be part of the replacement: it leaves this version (and its undo
                // history) with the clips that use it, as it did in the window that removed it.
                const serverIds = await serverMediaIds(s, projectId);
                if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
                const gone = serverIds ? removedIds(s.project, serverIds) : new Set();
                const droppedClips = gone.size > 0 && clipsUse(s.project, gone);
                forgetMedia(s, gone);
                const local = canonicalProject(s.project), revision = s.revision;
                const replaced = await request(API + '/projects/' + encodeURIComponent(projectId), { method: 'PUT', headers: { 'Content-Type': 'application/json', 'If-Match': fresh.etag }, body: JSON.stringify(local) });
                if (s.disposed || epoch !== s.projectEpoch) return;
                s.etag = replaced.etag || fresh.etag;
                s.savedProject = local;
                s.dirty = revision !== s.revision;
                if (!s.dirty) await clearDraft(s);
                if (gone.size) renderUI(s); else renderToolbar(s);
                if (droppedClips) showNotice(s, 'conflictMediaDropped', 'Media deleted in another window was left out, with the clips that used it.', false);
                else clearNotice(s);
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
    // Returns true when the recovered draft uses media that was removed since it was written: s.conflict is set then,
    // and the caller opens the conflict choices once the project is shown.
    async function recoverDraft(s, expectedProjectId, expectedEpoch) {
        const projectId = expectedProjectId || s.projectId;
        const epoch = expectedEpoch == null ? s.projectEpoch : expectedEpoch;
        if (s.readonly) return false;
        let raw;
        try { raw = localStorage.getItem('video-studio-draft:' + projectId); } catch (_) { return false; }
        if (!raw) return false;
        try {
            const draft = JSON.parse(raw);
            const shouldRecover = draft.project && Date.now() - Number(draft.saved_at || 0) < 7 * 86400000 && await s.ctx.confirmDialog(tr(s, 'recoverTitle', 'Recover local draft?'), tr(s, 'recoverCopy', 'A recent local draft is available. Restore it over the saved project?'));
            if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return false;
            if (shouldRecover) {
                const saved = clone(s.project), recovered = hydrateProject(s, draft.project);
                // The server may hold newer media than the draft; keep every asset the server knows.
                const known = new Set(recovered.assets.map(asset => asset.id));
                recovered.assets = recovered.assets.concat(saved.assets.filter(asset => !known.has(asset.id)).map(asset => assetAtProjectRate(asset, recovered)));
                // It may also have lost media the draft lists (removed in another window): unused, it leaves the draft;
                // used by a clip, the draft is kept as it is and the conflict is surfaced.
                const serverIds = await serverMediaIds(s, projectId);
                if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return false;
                if (serverIds) saved.assets.forEach(asset => serverIds.add(asset.id));
                const usesRemoved = !!serverIds && !dropRemovedMedia(s, recovered, serverIds, false);
                s.project = recovered; s.dirty = true; s.history = [saved]; s.redo = [];
                if (usesRemoved) s.conflict = true;
                // A draft title whose words or style differ from the saved copy still shows the saved PNG.
                recovered.tracks.forEach(track => track.clips.forEach(clip => {
                    const previous = clip.text && T().clipFor(saved, clip.id);
                    if (previous && previous.clip.asset_id === clip.asset_id && (previous.clip.text !== clip.text || P.textStyleKey(previous.clip.text_style || {}) !== P.textStyleKey(clip.text_style || {}))) P.markTextStale(s, clip.id);
                }));
                return usesRemoved;
            }
        } catch (_) { /* malformed recovery data is ignored */ }
        return false;
    }
    async function clearDraft(s) { try { localStorage.removeItem(draftKey(s)); } catch (_) {} }
    function selectedClip(s) { return s.project ? T().clipFor(s.project, s.selectedClipId) : null; }

    Object.assign(P, { uiLocale, assetDisplayName, API, MAX_CLIPS, CANVASES, READ_ONLY_SAFE, TERMINAL, PREF_TIMELINE, PREF_SNAP, PREF_QUALITY, PREF_MEDIA_VIEW, PREF_MEDIA_SORT, LABEL_COLORS, PREF_MAGNETIC, clone, clamp, idempotencyKey, T, I, tr, trCount, esc, icon, readPref, writePref, maxFrames, rateOf, request, canonicalProject, hydrateProject, assetAtProjectRate, keepOwnedAssetFields, trackName, makeTrack, defaultTracks, defaultProject, showNotice, clearNotice, projectDuration, snapshot, recordChange, mutate, undo, totalClipCount, nextStart, addTrackTo, freeStart, scheduleSave, queueAutosave, saveProject, showConflict, saveUntilClean, draftKey, persistDraft, recoverDraft, clearDraft, selectedClip, flushTextBefore, normalizeTextStyle, adoptServerMedia, forgetServerRemovedMedia, serverMediaIds, stripAssets, forgetMedia });
})();
