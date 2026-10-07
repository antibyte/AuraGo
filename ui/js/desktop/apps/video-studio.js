(function () {
    'use strict';

    const API = '/api/desktop/video-studio';
    const FPS = 30;
    const MAX_CLIPS = 256;
    const MAX_ACTIVE_ASSETS = 32;
    const CANVASES = [
        { width: 1280, height: 720, key: 'landscape720' }, { width: 1920, height: 1080, key: 'landscape1080' },
        { width: 720, height: 1280, key: 'portrait720' }, { width: 1080, height: 1920, key: 'portrait1080' },
        { width: 720, height: 720, key: 'square720' }, { width: 1080, height: 1080, key: 'square1080' }
    ];
    const instances = new Map();
    const clone = value => JSON.parse(JSON.stringify(value));
    const clamp = (value, min, max) => Math.max(min, Math.min(max, value));
    const idempotencyKey = () => crypto.randomUUID ? crypto.randomUUID() : Date.now().toString(36) + '-' + Math.random().toString(36).slice(2);
    function tr(s, key, fallback) {
        const value = s.ctx.t ? s.ctx.t('videoStudio.' + key) : '';
        return !value || value === 'videoStudio.' + key ? (fallback || key) : value;
    }
    function esc(s, value) { return s.ctx.esc(String(value == null ? '' : value)); }
    function icon(s, key) { return s.ctx.iconMarkup ? s.ctx.iconMarkup(key, key, 'vs-icon', 15) : ''; }
    function projectURL(s, tail) { return API + '/projects/' + encodeURIComponent(s.projectId) + (tail ? '/' + tail : ''); }

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
                text: String(clip.text || ''), text_style: clip.text_style || null, transition: clip.transition || null
            }))
        }));
        return copy;
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
    function defaultProject(name, s) {
        const uid = prefix => window.VideoStudioTimeline.newId(prefix);
        return {
            version: 1, name, width: 1280, height: 720, fps: FPS, assets: [],
            tracks: [
                ...Array.from({ length: 2 }, (_, index) => ({ id: uid('track'), kind: 'video', name: tr(s, 'videoTrack', 'Video') + ' ' + (index + 1), muted: false, hidden: false, locked: false, clips: [] })),
                ...Array.from({ length: 2 }, (_, index) => ({ id: uid('track'), kind: 'audio', name: tr(s, 'audioTrack', 'Audio') + ' ' + (index + 1), muted: false, hidden: false, locked: false, clips: [] })),
                { id: uid('track'), kind: 'overlay', name: tr(s, 'overlayTrack', 'Overlay') + ' 1', muted: false, hidden: false, locked: false, clips: [] }
            ]
        };
    }
    function dispose(id) {
        const s = instances.get(id);
        if (!s) return;
        s.disposed = true;
        s.abort.abort();
        clearTimeout(s.autosaveTimer);
        clearInterval(s.jobsTimer);
        if (s.preview) s.preview.dispose();
        if (s.detachTimeline) s.detachTimeline();
        if (s.keyHandler) window.removeEventListener('keydown', s.keyHandler, true);
        if (s.ctx.clearWindowMenus) s.ctx.clearWindowMenus(id);
        instances.delete(id);
    }
    function appRender(host, id, ctx) {
        dispose(id);
        const s = {
            id, host, ctx, abort: new AbortController(), disposed: false, project: null, projectId: '', desktopPath: '', etag: '',
            status: null, projects: [], jobs: [], jobLocalStops: new Set(), frame: 0, selectedClipId: '', selectionRevision: 0, selectedAssetId: '',
            zoom: 1, dirty: false, saving: false, saveError: '', conflict: false, activePanel: 'media', history: [], redo: [],
            busy: false, revision: 0, artworkRevision: 0, savePromise: null, autosaveTimer: 0
        };
        instances.set(id, s);
        s.projectEpoch = 0;
        host.innerHTML = `<div class="vs-app"><header class="vs-header"><div class="vs-brand"><span class="vs-brand-mark">▶</span><div><strong>Video Studio</strong><small>${esc(s, tr(s, 'subtitle', 'Shape a story, frame by frame'))}</small></div></div><div class="vs-project-tools"><select data-project-picker aria-label="${esc(s, tr(s, 'projects', 'Projects'))}"></select><button type="button" data-action="new-project">＋ ${esc(s, tr(s, 'newProject', 'New project'))}</button><span class="vs-save-state" data-save-state role="status"></span></div><div class="vs-header-actions"><button type="button" data-action="undo" aria-label="${esc(s, tr(s, 'undo', 'Undo'))}" disabled>↶</button><button type="button" data-action="redo" aria-label="${esc(s, tr(s, 'redo', 'Redo'))}" disabled>↷</button><button type="button" data-action="toggle-inspector" class="vs-inspector-toggle">☷ ${esc(s, tr(s, 'inspector', 'Inspector'))}</button><button type="button" data-action="save">${icon(s, 'save')} ${esc(s, tr(s, 'save', 'Save'))}</button><button type="button" class="vs-export-button" data-action="export">${icon(s, 'video')} ${esc(s, tr(s, 'export', 'Export'))}</button></div></header><div class="vs-notice" data-notice role="status" aria-live="polite" hidden></div><div class="vs-disabled" data-disabled hidden></div><div class="vs-workspace"><aside class="vs-media-panel"><div class="vs-panel-head"><div><span class="vs-eyebrow">${esc(s, tr(s, 'project', 'Project'))}</span><h2>${esc(s, tr(s, 'media', 'Media'))}</h2></div><button type="button" data-action="add-media" aria-label="${esc(s, tr(s, 'import', 'Import media'))}" title="${esc(s, tr(s, 'import', 'Import media'))}">＋</button></div><div class="vs-media-actions"><button type="button" data-action="upload">${icon(s, 'upload')} ${esc(s, tr(s, 'upload', 'Upload'))}</button><button type="button" data-action="browse">${icon(s, 'folder')} ${esc(s, tr(s, 'browse', 'Browse files'))}</button><input type="file" data-file-input accept="video/*,audio/*,image/png,image/webp" multiple hidden></div><div class="vs-media-filter"><span>⌕</span><input type="search" data-search placeholder="${esc(s, tr(s, 'searchMedia', 'Search media'))}" aria-label="${esc(s, tr(s, 'searchMedia', 'Search media'))}"></div><div class="vs-asset-list" data-assets></div><div class="vs-media-foot"><span data-asset-count></span><button type="button" data-action="add-title">T ${esc(s, tr(s, 'addTitle', 'Add title'))}</button></div></aside><main class="vs-center"><div class="vs-preview-top"><span>${esc(s, tr(s, 'preview', 'Preview'))}</span><label><span>${esc(s, tr(s, 'canvas', 'Canvas'))}</span><select data-canvas aria-label="${esc(s, tr(s, 'canvas', 'Canvas'))}"></select></label><span class="vs-frame-readout" data-time>00:00:00</span></div><div class="vs-preview-stage"><div class="vs-preview-mat"><canvas data-preview width="1280" height="720" aria-label="${esc(s, tr(s, 'preview', 'Preview'))}"></canvas><div class="vs-preview-empty" data-preview-empty><span>▶</span><strong>${esc(s, tr(s, 'previewEmpty', 'Your edit takes shape here'))}</strong><small>${esc(s, tr(s, 'previewHint', 'Drop media onto a track to begin'))}</small></div></div></div><div class="vs-transport"><button type="button" data-action="step-back" aria-label="${esc(s, tr(s, 'previousFrame', 'Previous frame'))}">|◀</button><button type="button" class="vs-play" data-action="play" aria-label="${esc(s, tr(s, 'play', 'Play'))}">▶</button><button type="button" data-action="step-forward" aria-label="${esc(s, tr(s, 'nextFrame', 'Next frame'))}">▶|</button><span class="vs-timecode" data-transport-time>00:00:00</span><span class="vs-transport-spacer"></span><span class="vs-proxy-status" data-proxy-status>${esc(s, tr(s, 'ready', 'Ready'))}</span></div><section class="vs-jobs" data-jobs></section></main><aside class="vs-inspector"><div class="vs-inspector-head"><span class="vs-eyebrow">${esc(s, tr(s, 'controls', 'Controls'))}</span><h2>${esc(s, tr(s, 'inspector', 'Inspector'))}</h2></div><div data-inspector class="vs-inspector-body"></div></aside></div><section class="vs-timeline-panel"><div data-timeline></div></section><div class="vs-modal-host" data-modal-host></div><div class="vs-conflict-host" data-conflict-host></div></div>`;
        s.q = selector => host.querySelector(selector);
        s.q('.vs-export-button').setAttribute('aria-label', tr(s, 'export', 'Export'));
        s.q('.vs-export-button').setAttribute('title', tr(s, 'export', 'Export'));
        s.q('.vs-app').tabIndex = 0;
        const savedPath = document.createElement('span');
        savedPath.className = 'vs-save-path'; savedPath.dataset.savedPath = '';
        s.q('[data-save-state]').after(savedPath);
        const inspectorScrim = document.createElement('button');
        inspectorScrim.type = 'button'; inspectorScrim.className = 'vs-inspector-scrim';
        inspectorScrim.dataset.action = 'close-inspector'; inspectorScrim.setAttribute('aria-label', tr(s, 'close', 'Close'));
        s.q('.vs-inspector').before(inspectorScrim);
        s.q('.vs-inspector').setAttribute('aria-label', tr(s, 'inspector', 'Inspector'));
        const inspectorClose = document.createElement('button');
        inspectorClose.type = 'button'; inspectorClose.className = 'vs-inspector-close'; inspectorClose.dataset.action = 'close-inspector';
        inspectorClose.setAttribute('aria-label', tr(s, 'close', 'Close')); inspectorClose.textContent = '×';
        inspectorClose.setAttribute('title', tr(s, 'close', 'Close'));
        s.q('.vs-inspector-head').appendChild(inspectorClose);
        s.notice = (key, fallback, error) => showNotice(s, key, fallback, error);
        s.timelineOptions = timelineOptions(s);
        s.detachTimeline = window.VideoStudioTimeline.attach(s.q('[data-timeline]'), s.timelineOptions);
        s.preview = window.VideoStudioPreview.mount(s.q('[data-preview]'), () => s.project, () => s.frame, frame => {
            s.frame = frame;
            const clock = window.VideoStudioTimeline.frameToClock(frame);
            if (s.q('[data-time]')) s.q('[data-time]').textContent = clock;
            if (s.q('[data-transport-time]')) s.q('[data-transport-time]').textContent = clock;
            if (s.q('[data-timeline] .vs-playhead')) s.q('[data-timeline] .vs-playhead').style.left = frame / FPS * 52 * s.zoom + 'px';
        }, playing => {
            const button = s.q('[data-action="play"]');
            if (button) { button.textContent = playing ? 'Ⅱ' : '▶'; button.setAttribute('aria-label', tr(s, playing ? 'pause' : 'play', playing ? 'Pause' : 'Play')); }
        });
        host.addEventListener('click', event => {
            if (event.target.closest('.vs-preview-stage,.vs-timeline-scroll') && !event.target.closest('button,input,select,textarea')) s.q('.vs-app').focus({ preventScroll: true });
            handleClick(s, event);
        }, { signal: s.abort.signal });
        host.addEventListener('change', event => handleChange(s, event), { signal: s.abort.signal });
        host.addEventListener('input', event => handleInput(s, event), { signal: s.abort.signal });
        host.addEventListener('dragover', event => { if (event.target.closest('.vs-preview-mat')) { event.preventDefault(); event.dataTransfer.dropEffect = 'copy'; } }, { signal: s.abort.signal });
        host.addEventListener('drop', event => { if (event.target.closest('.vs-preview-mat')) { event.preventDefault(); const id = event.dataTransfer.getData('application/x-video-studio-asset'); if (id) addAssetToTrack(s, id, firstVideoTrack(s.project), s.frame); } }, { signal: s.abort.signal });
        s.q('[data-file-input]').addEventListener('change', async event => {
            const projectId = s.projectId, epoch = s.projectEpoch;
            for (const file of Array.from(event.target.files || [])) {
                if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) break;
                await uploadFile(s, file, projectId, epoch);
            }
            event.target.value = '';
        }, { signal: s.abort.signal });
        s.keyHandler = event => handleKeys(s, event);
        window.addEventListener('keydown', s.keyHandler, true);
        if (typeof ctx.setWindowBeforeClose === 'function') ctx.setWindowBeforeClose(id, async () => {
            while (s.dirty && !s.disposed) {
                const saved = await saveProject(s);
                if (!saved) return false;
            }
            return !s.dirty;
        });
        if (ctx.setWindowMenus) ctx.setWindowMenus(id, [{
            label: tr(s, 'project', 'Project'), items: [
                { label: tr(s, 'newProject', 'New project'), action: () => createProjectPrompt(s) },
                { label: tr(s, 'save', 'Save'), action: () => saveProject(s) },
                { label: tr(s, 'export', 'Export'), action: () => openExport(s) }
            ]
        }]);
        initialize(s).catch(() => showNotice(s, 'loadFailed', 'Video Studio could not load.', true));
    }
    function showNotice(s, key, fallback, error) {
        if (s.disposed || !s.q('[data-notice]')) return;
        const el = s.q('[data-notice]');
        el.hidden = false;
        el.classList.toggle('is-error', !!error);
        el.textContent = tr(s, key, fallback);
    }
    function clearNotice(s) { const el = s.q('[data-notice]'); if (el) { el.hidden = true; el.textContent = ''; } }

    async function initialize(s) {
        const response = await request(API + '/status', { signal: s.abort.signal });
        s.status = response.body;
        if (!s.status.enabled || !s.status.desktop_enabled || !s.status.ffmpeg_ready) {
            const disabled = s.q('[data-disabled]');
            disabled.hidden = false;
            const issueKey = ({ ffmpeg_unavailable: 'ffmpegMissing', ffmpeg_missing: 'ffmpegMissing', video_studio_disabled: 'featureDisabled', desktop_disabled: 'featureDisabled', read_only: 'readonly', admin_required: 'adminRequired' })[s.status.issue];
            const issue = issueKey ? tr(s, issueKey, issueKey === 'ffmpegMissing' ? 'FFmpeg is not available on this server.' : 'Video Studio is currently unavailable.') : tr(s, 'unavailable', 'Video Studio is currently unavailable.');
            disabled.innerHTML = `<span>◌</span><div><strong>${esc(s, tr(s, 'unavailableTitle', 'Studio unavailable'))}</strong><p>${esc(s, issue)}</p></div>`;
        }
        s.featureDisabled = !s.status.enabled || !s.status.desktop_enabled;
        s.readonly = !!s.status.read_only || s.featureDisabled;
        s.q('.vs-app').classList.toggle('is-readonly', s.readonly);
        if (s.featureDisabled) {
            // Every project endpoint refuses while the feature is off; show the calm empty state, not a load error.
            renderEmptyProject(s);
            populateCanvas(s);
            s.q('.vs-app').querySelectorAll(['new-project', 'save', 'export', 'upload', 'browse', 'add-media', 'add-title'].map(action => `[data-action="${action}"]`).join(',')).forEach(button => { button.disabled = true; });
            return;
        }
        await loadProjects(s);
        if (!s.project && s.projects.length) await loadProject(s, s.projects[0].id);
        else if (!s.project) renderEmptyProject(s);
        populateCanvas(s);
        renderUI(s);
        if (s.projectId) await loadJobs(s);
        s.jobsTimer = window.setInterval(() => { if (!s.disposed && s.projectId) loadJobs(s).catch(() => {}); }, 2500);
    }
    async function loadProjects(s) {
        const response = await request(API + '/projects', { signal: s.abort.signal });
        s.projects = Array.isArray(response.body.projects) ? response.body.projects : [];
        const picker = s.q('[data-project-picker]');
        picker.innerHTML = `<option value="">${esc(s, tr(s, 'selectProject', 'Select a project'))}</option>` + s.projects.map(item => `<option value="${esc(s, item.id || item.project_id || '')}">${esc(s, item.project && item.project.name || tr(s, 'untitled', 'Untitled project'))}</option>`).join('');
        picker.value = s.projectId || '';
    }
    async function loadProject(s, id) {
        if (!id) return;
        const epoch = s.projectEpoch = (s.projectEpoch || 0) + 1;
        s.q('[data-modal-host]')?.replaceChildren();
        s.busy = true;
        try {
            const response = await request(API + '/projects/' + encodeURIComponent(id), { signal: s.abort.signal });
            if (s.disposed || epoch !== s.projectEpoch || s.timelineDragging) return false;
            s.projectId = id;
            s.etag = response.etag;
            s.desktopPath = response.body.desktop_path || '';
            s.project = hydrateProject(s, response.body.project || response.body);
            s.savedProject = canonicalProject(s.project);
            s.selectedClipId = '';
            s.frame = 0;
            s.history = []; s.redo = []; s.dirty = false;
            if (s.preview) s.preview.seek(0);
            await recoverDraft(s, id, epoch);
            if (s.disposed || epoch !== s.projectEpoch || id !== s.projectId) return false;
            await loadJobs(s);
            if (s.disposed || epoch !== s.projectEpoch || id !== s.projectId) return false;
            renderUI(s);
            return true;
        } finally { s.busy = false; }
    }
    async function createProject(s, name) {
        if (s.readonly) return showNotice(s, 'readonly', 'Read-only mode prevents editing.', true);
        const epoch = s.projectEpoch = (s.projectEpoch || 0) + 1;
        const result = await request(API + '/projects', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name: name || tr(s, 'untitled', 'Untitled project') }) });
        if (s.disposed || epoch !== s.projectEpoch) return false;
        const wrapper = result.body;
        s.projectId = wrapper.id || wrapper.project_id || wrapper.project && wrapper.project.id;
        s.etag = result.etag;
        s.desktopPath = wrapper.desktop_path || '';
        s.project = hydrateProject(s, wrapper.project || defaultProject(name, s));
        if (!s.project.tracks || !s.project.tracks.length) s.project = defaultProject(name, s);
        s.project.name = name || s.project.name;
        s.savedProject = canonicalProject(s.project);
        s.dirty = true;
        s.history = []; s.redo = [];
        await saveProject(s);
        await loadProjects(s);
        renderUI(s);
        return true;
    }
    async function createProjectPrompt(s) {
        const epoch = s.projectEpoch || 0;
        const name = await s.ctx.promptDialog(tr(s, 'newProject', 'New project'), tr(s, 'projectName', 'Untitled project'));
        if (s.disposed || epoch !== (s.projectEpoch || 0) || name === false || name == null || !String(name).trim()) return;
        try { await createProject(s, String(name).trim()); }
        catch (_) { showNotice(s, 'createFailed', 'Could not create the project.', true); }
    }
    function renderEmptyProject(s) {
        const stage = s.q('.vs-preview-stage');
        const mat = s.q('.vs-preview-mat');
        if (mat) mat.insertAdjacentHTML('beforeend', `<div class="vs-empty-project" data-no-project><div class="vs-empty-art"><span>◻</span><span>▶</span><span>◯</span></div><p class="vs-eyebrow">${esc(s, tr(s, 'emptyEyebrow', 'A new canvas'))}</p><h2>${esc(s, tr(s, 'emptyTitle', 'Make room for a story'))}</h2><p>${esc(s, tr(s, 'emptyCopy', 'Create a project to bring clips, sound and titles together.'))}</p><button type="button" class="vs-primary" data-action="new-project" ${s.readonly ? 'disabled' : ''}>＋ ${esc(s, tr(s, 'newProject', 'New project'))}</button></div>`);
        if (stage) stage.classList.add('vs-no-project');
        const timeline = s.q('[data-timeline]'); if (timeline) timeline.replaceChildren();
    }
    function populateCanvas(s) {
        const select = s.q('[data-canvas]');
        if (!select) return;
        const sizes = s.status && s.status.limits && s.status.limits.canvas_sizes || CANVASES.map(c => ({ width: c.width, height: c.height }));
        select.innerHTML = sizes.map(size => {
            const key = CANVASES.find(c => c.width === size.width && c.height === size.height)?.key || (size.width + 'x' + size.height);
            return `<option value="${Number(size.width)}x${Number(size.height)}">${esc(s, tr(s, key, `${size.width} × ${size.height}`))}</option>`;
        }).join('');
    }
    function renderUI(s) {
        if (s.disposed || !s.project || !s.q('[data-timeline]')) return;
        s.q('.vs-app').classList.toggle('is-readonly', s.readonly);
        const picker = s.q('[data-project-picker]');
        if (picker) picker.value = s.projectId;
        const state = s.q('[data-save-state]');
        if (state) {
            state.classList.toggle('is-dirty', s.dirty);
            state.textContent = s.saving ? tr(s, 'saving', 'Saving…') : s.saveError ? tr(s, 'saveFailed', 'Save failed') : s.dirty ? tr(s, 'unsaved', 'Unsaved changes') : tr(s, 'saved', 'All changes saved');
        }
        s.q('[data-canvas]').value = `${s.project.width}x${s.project.height}`;
        s.q('.vs-preview-stage').classList.remove('vs-no-project');
        const noProject = s.q('[data-no-project]'); if (noProject) noProject.remove();
        s.q('[data-preview]').width = Number(s.project.width || 1280);
        s.q('[data-preview]').height = Number(s.project.height || 720);
        s.q('.vs-preview-mat').style.aspectRatio = `${Number(s.project.width || 1280)} / ${Number(s.project.height || 720)}`;
        const savedPath = s.q('[data-saved-path]');
        if (savedPath) { savedPath.textContent = s.desktopPath || ''; savedPath.title = s.desktopPath || ''; }
        const previewEmpty = s.q('[data-preview-empty]');
        if (previewEmpty) previewEmpty.hidden = !!s.project.tracks.some(track => track.clips.length);
        s.q('[data-canvas]').disabled = s.readonly;
        s.q('[data-action="new-project"]').disabled = s.readonly;
        renderAssets(s);
        renderInspector(s);
        syncInspectorLock(s);
        window.VideoStudioTimeline.render(s.q('[data-timeline]'), s.timelineOptions);
        renderJobs(s);
        const undo = s.q('[data-action="undo"]'), redo = s.q('[data-action="redo"]');
        if (undo) undo.disabled = !s.history.length;
        if (redo) redo.disabled = !s.redo.length;
        const hasRenderableClip = s.project.tracks.some(track => track.clips.some(clip => s.project.assets.some(asset => asset.id === clip.asset_id)));
        s.q('.vs-export-button').disabled = s.readonly || s.featureDisabled || !s.status || !s.status.ffmpeg_ready || !s.projectId || !hasRenderableClip;
        if (s.readonly) s.q('.vs-media-actions').setAttribute('aria-disabled', 'true');
        else s.q('.vs-media-actions').removeAttribute('aria-disabled');
        if (s.readonly) {
            s.q('[data-inspector]').querySelectorAll('input,select,textarea,button').forEach(control => { control.disabled = true; });
            s.q('[data-file-input]').disabled = true;
        }
        if (s.preview) s.preview.seek(s.frame);
    }
    function renderAssets(s) {
        const root = s.q('[data-assets]');
        if (!root || !s.project) return;
        const activeAssets = new Set(s.project.tracks.flatMap(track => track.clips.map(clip => clip.asset_id)));
        const search = (s.q('[data-search]')?.value || '').toLowerCase();
        root.innerHTML = s.project.assets.filter(asset => !search || asset.name.toLowerCase().includes(search)).map(asset => {
            const kindKey = asset.kind === 'video' ? 'assetVideo' : asset.kind === 'audio' ? 'assetAudio' : 'assetImage';
            const kindLabel = tr(s, kindKey, asset.kind === 'video' ? 'Video' : asset.kind === 'audio' ? 'Audio' : 'Image');
            const kindIcon = asset.kind === 'video' ? '▶' : asset.kind === 'audio' ? '♫' : '▧';
            const active = activeAssets.has(asset.id);
            const duration = asset.duration_frames ? window.VideoStudioTimeline.frameToClock(asset.duration_frames) : tr(s, 'processing', 'Processing');
            return `<article class="vs-asset-card${active ? ' is-used' : ''}" draggable="${!s.readonly}" data-asset-id="${esc(s, asset.id)}" tabindex="0"><span class="vs-asset-thumb vs-thumb-${esc(s, asset.kind)}">${kindIcon}</span><span class="vs-asset-info"><strong title="${esc(s, asset.name)}">${esc(s, asset.name)}</strong><small>${esc(s, kindLabel)} · ${esc(s, duration)}</small></span><button type="button" data-place-asset="${esc(s, asset.id)}" title="${esc(s, tr(s, 'addToTimeline', 'Add to timeline'))}" aria-label="${esc(s, tr(s, 'addToTimeline', 'Add to timeline'))}" ${s.readonly ? 'disabled' : ''}>＋</button></article>`;
        }).join('') || `<div class="vs-empty-media"><span>▧</span><strong>${esc(s, tr(s, 'emptyMedia', 'Your media will appear here'))}</strong><small>${esc(s, tr(s, 'emptyMediaHint', 'Upload a clip or browse your Desktop files.'))}</small></div>`;
        s.q('[data-asset-count]').textContent = tr(s, 'assetCount', '{{count}} items').replace('{{count}}', String(s.project.assets.length));
    }
    function renderInspector(s) {
        const host = s.q('[data-inspector]');
        const selected = window.VideoStudioTimeline.clipFor(s.project, s.selectedClipId);
        if (!selected) {
            host.innerHTML = `<div class="vs-inspector-empty"><span>◉</span><strong>${esc(s, tr(s, 'noSelection', 'Select a clip'))}</strong><small>${esc(s, tr(s, 'inspectorHint', 'Choose a clip on the timeline to edit its timing and look.'))}</small><button type="button" data-action="open-ai" ${s.status && s.status.generation && s.status.generation.enabled && s.status.generation.configured && !s.readonly ? '' : 'hidden'}>✦ ${esc(s, tr(s, 'generate', 'Generate a clip'))}</button><button type="button" data-action="open-stickers">✦ ${esc(s, tr(s, 'sticker', 'Sticker'))}</button></div>`;
            return;
        }
        const { clip, track } = selected;
        const asset = s.project.assets.find(item => item.id === clip.asset_id) || {};
        const audio = asset.kind === 'audio' || track.kind === 'audio';
        const text = !!clip.text;
        host.innerHTML = `<div class="vs-selected-media"><span>${asset.kind === 'video' ? '▶' : asset.kind === 'audio' ? '♫' : '▧'}</span><div><strong>${esc(s, asset.name || tr(s, 'missingMedia', 'Missing media'))}</strong><small>${esc(s, track.name)}</small></div></div><div class="vs-inspector-group"><h3>${esc(s, tr(s, 'timing', 'Timing'))}</h3><label>${esc(s, tr(s, 'startFrame', 'Start frame'))}<input data-field="start" type="number" min="0" step="1" value="${clip.start}"></label><label>${esc(s, tr(s, 'durationFrames', 'Duration (frames)'))}<input data-field="duration" type="number" min="1" step="1" value="${clip.duration}"></label>${audio ? '' : `<label>${esc(s, tr(s, 'sourceOffset', 'Source offset'))}<input data-field="offset" type="number" min="0" step="1" value="${clip.offset || 0}"></label>`}</div>${audio ? '' : `<div class="vs-inspector-group"><h3>${esc(s, tr(s, 'transform', 'Transform'))}</h3><div class="vs-field-grid"><label>X <input data-field="x" type="number" min="0" max="1" step="0.01" value="${clip.x}"></label><label>Y <input data-field="y" type="number" min="0" max="1" step="0.01" value="${clip.y}"></label><label>${esc(s, tr(s, 'width', 'Width'))}<input data-field="width" type="number" min="0.02" max="1" step="0.01" value="${clip.width}"></label><label>${esc(s, tr(s, 'height', 'Height'))}<input data-field="height" type="number" min="0.02" max="1" step="0.01" value="${clip.height}"></label></div><label>${esc(s, tr(s, 'rotation', 'Rotation'))}<input data-field="rotation" type="number" min="-360" max="360" value="${clip.rotation}"></label><label>${esc(s, tr(s, 'opacity', 'Opacity'))}<input data-field="opacity" type="range" min="0" max="1" step="0.01" value="${clip.opacity}"><output>${Math.round(clip.opacity * 100)}%</output></label><label>${esc(s, tr(s, 'fit', 'Fit'))}<select data-field="fit"><option value="contain" ${clip.fit === 'contain' ? 'selected' : ''}>${esc(s, tr(s, 'contain', 'Contain'))}</option><option value="cover" ${clip.fit === 'cover' ? 'selected' : ''}>${esc(s, tr(s, 'cover', 'Cover'))}</option></select></label></div>`}<div class="vs-inspector-group"><h3>${esc(s, tr(s, 'audio', 'Audio'))}</h3><label>${esc(s, tr(s, 'volume', 'Volume'))}<input data-field="volume" type="range" min="0" max="2" step="0.01" value="${clip.volume}"><output>${Math.round(clip.volume * 100)}%</output></label><div class="vs-field-grid"><label>${esc(s, tr(s, 'fadeIn', 'Fade in (frames)'))}<input data-field="fade_in" type="number" min="0" max="${clip.duration}" value="${clip.fade_in || 0}"></label><label>${esc(s, tr(s, 'fadeOut', 'Fade out (frames)'))}<input data-field="fade_out" type="number" min="0" max="${clip.duration}" value="${clip.fade_out || 0}"></label></div></div><div class="vs-inspector-group"><h3>${esc(s, tr(s, 'transition', 'Transition'))}</h3><label>${esc(s, tr(s, 'transition', 'Transition'))}<select data-transition-type><option value="none">${esc(s, tr(s, 'none', 'None'))}</option>${['dissolve','black','wipeleft','wiperight'].map(type => `<option value="${type}" ${clip.transition && clip.transition.type === type ? 'selected' : ''}>${esc(s, tr(s, type, type))}</option>`).join('')}</select></label><label>${esc(s, tr(s, 'transitionFrames', 'Duration (frames)'))}<input data-transition-duration type="number" min="1" max="90" value="${clip.transition && clip.transition.duration || 15}" ${clip.transition ? '' : 'disabled'}></label><button type="button" data-action="apply-transition" class="vs-secondary">${esc(s, tr(s, 'applyTransition', 'Apply to next clip'))}</button></div>${text ? textControls(s, clip) : ''}`;
    }
    function syncInspectorLock(s) {
        const selected = window.VideoStudioTimeline.clipFor(s.project, s.selectedClipId);
        if (s.readonly || selected && selected.track.locked) {
            s.q('[data-inspector]').querySelectorAll('input,select,textarea,button').forEach(control => { control.disabled = true; });
        }
    }
    function textControls(s, clip) {
        const style = clip.text_style || {};
        return `<div class="vs-inspector-group vs-text-group"><h3>${esc(s, tr(s, 'text', 'Title'))}</h3><label>${esc(s, tr(s, 'titleText', 'Text'))}<textarea data-text-field="text" rows="3">${esc(s, clip.text)}</textarea></label><label>${esc(s, tr(s, 'font', 'Font'))}<select data-text-style="font_family"><option>Arial</option><option ${style.font_family === 'Georgia' ? 'selected' : ''}>Georgia</option><option ${style.font_family === 'Trebuchet MS' ? 'selected' : ''}>Trebuchet MS</option><option ${style.font_family === 'Impact' ? 'selected' : ''}>Impact</option></select></label><label>${esc(s, tr(s, 'fontSize', 'Font size'))}<input type="number" min="18" max="220" data-text-style="font_size" value="${style.font_size || 72}"></label><label>${esc(s, tr(s, 'color', 'Color'))}<input type="color" data-text-style="color" value="${style.color || '#ffffff'}"></label><label>${esc(s, tr(s, 'alignment', 'Alignment'))}<select data-text-style="alignment"><option value="left" ${style.alignment === 'left' ? 'selected' : ''}>${esc(s, tr(s, 'alignLeft', 'Left'))}</option><option value="center" ${!style.alignment || style.alignment === 'center' ? 'selected' : ''}>${esc(s, tr(s, 'alignCenter', 'Center'))}</option><option value="right" ${style.alignment === 'right' ? 'selected' : ''}>${esc(s, tr(s, 'alignRight', 'Right'))}</option></select></label><label>${esc(s, tr(s, 'background', 'Background'))}<input type="color" data-text-style="background_color" value="${style.background_color || '#101319'}"></label><label>${esc(s, tr(s, 'backgroundOpacity', 'Background opacity'))}<input type="range" min="0" max="1" step="0.05" data-text-style="background_opacity" value="${style.background_opacity == null ? 0.45 : style.background_opacity}"></label><button type="button" data-action="apply-text" class="vs-secondary">${esc(s, tr(s, 'applyText', 'Update title artwork'))}</button></div>`;
    }
    function timelineOptions(s) {
        return {
            state: s, label: key => tr(s, key, key), esc: value => esc(s, value), icon: key => icon(s, key),
            onSelect: id => {
                s.selectedClipId = id; s.selectionRevision++;
                renderInspector(s);
                syncInspectorLock(s);
                if (s.q('.vs-app').getBoundingClientRect().width <= 920) setInspectorOpen(s, true, false);
            },
            onFrame: value => { s.preview.seek(value); },
            onPreviewChange: () => { if (s.preview) s.preview.seek(s.frame); },
            onChange: (label, before) => recordChange(s, label, before),
            onDragEnd: () => {
                if (s.pendingRefresh) { s.pendingRefresh = false; refreshProject(s).catch(() => {}).finally(() => { if (s.dirty && !s.conflict) queueAutosave(s); }); }
                else if (s.dirty && !s.conflict) queueAutosave(s);
            },
            onInvalid: () => showNotice(s, 'invalidTiming', 'That edit would create invalid timing or an overlap.', true),
            onAction: (action, id) => timelineAction(s, action, id),
            onTrackAction: (id, action) => trackAction(s, id, action),
            onAddTrack: kind => addTrack(s, kind),
            onDropAsset: (assetId, trackId, at) => addAssetToTrack(s, assetId, trackId, at),
            onReorderTrack: (from, to) => reorderTrack(s, from, to)
        };
    }
    function snapshot(s) { return clone(s.project); }
    function recordChange(s, label, before) {
        if (s.readonly) { if (before) s.project = hydrateProject(s, before); return renderUI(s); }
        if (window.VideoStudioTimeline.validTimeline && !window.VideoStudioTimeline.validTimeline(s.project)) {
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
        const before = snapshot(s);
        fn(s.project);
        if (window.VideoStudioTimeline.validTimeline && !window.VideoStudioTimeline.validTimeline(s.project)) {
            s.project = hydrateProject(s, before);
            showNotice(s, 'invalidTiming', 'That edit would create invalid timing or an overlap.', true);
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
        const source = redo ? s.redo : s.history;
        const target = redo ? s.history : s.redo;
        if (!source.length) return;
        s.artworkRevision++; target.push(snapshot(s));
        s.project = hydrateProject(s, source.pop());
        s.dirty = true; s.revision++;
        persistDraft(s); scheduleSave(s); renderUI(s); s.preview.seek(s.frame);
    }

    function firstVideoTrack(project) { return project && (project.tracks.find(track => track.kind === 'video' && !track.locked) || project.tracks.find(track => track.kind === 'video'))?.id; }
    function nextStart(track) { return Math.max(0, ...track.clips.map(clip => clip.start + clip.duration)); }
    function activeAssetCount(project) { return new Set(project.tracks.flatMap(track => track.clips.map(clip => clip.asset_id))).size; }
    function totalClipCount(project) { return project.tracks.reduce((n, track) => n + track.clips.length, 0); }

    function addAssetToTrack(s, assetId, trackId, at) {
        if (s.readonly || s.timelineDragging) return;
        const asset = s.project.assets.find(item => item.id === assetId);
        let track = s.project.tracks.find(item => item.id === trackId);
        if (!asset || !track) return;
        if (totalClipCount(s.project) >= MAX_CLIPS) return showNotice(s, 'clipLimit', 'This project reached its clip limit.', true);
        if (!s.project.tracks.some(item => item.clips.some(clip => clip.asset_id === asset.id)) && activeAssetCount(s.project) >= MAX_ACTIVE_ASSETS) return showNotice(s, 'assetLimit', 'This project reached its media limit.', true);
        if (asset.kind === 'image' && track.kind !== 'overlay') track = s.project.tracks.find(item => item.kind === 'overlay' && !item.locked);
        else if (asset.kind === 'audio' && track.kind !== 'audio') track = s.project.tracks.find(item => item.kind === 'audio' && !item.locked);
        else if (asset.kind === 'video' && (track.kind === 'overlay' || (track.kind === 'audio' && !asset.has_audio))) track = s.project.tracks.find(item => item.kind === 'video' && !item.locked);
        if (!track || track.locked) return showNotice(s, 'trackLimit', 'Add an unlocked track before placing this media.', true);
        const duration = Math.max(1, Math.min(Number(asset.kind === 'image' ? FPS * 5 : asset.duration_frames || FPS * 5), Number(s.status && s.status.limits && s.status.limits.max_duration_frames || FPS * 600)));
        let requested = Number.isFinite(at) ? Math.max(0, at) : nextStart(track);
        const sorted = track.clips.slice().sort((a, b) => a.start - b.start);
        for (const occupied of sorted) {
            if (requested + duration <= occupied.start) break;
            if (requested < occupied.start + occupied.duration) requested = occupied.start + occupied.duration;
        }
        if (requested + duration > Number(s.status && s.status.limits && s.status.limits.max_duration_frames || FPS * 600)) return showNotice(s, 'projectLimit', 'The project reached its 10-minute limit.', true);
        mutate(s, 'add clip', project => {
            const target = project.tracks.find(item => item.id === track.id);
            const clip = window.VideoStudioTimeline.makeClip(asset, requested);
            clip.duration = duration;
            if (asset.kind !== 'audio') { clip.width = 1; clip.height = 1; }
            target.clips.push(clip);
            s.selectedClipId = clip.id;
        });
    }

    function timelineAction(s, action, id) {
        const found = window.VideoStudioTimeline.clipFor(s.project, id);
        if (!found || found.track.locked || s.readonly || s.timelineDragging) return;
        if (action === 'delete') {
            mutate(s, action, project => { const track = project.tracks.find(item => item.id === found.track.id); track.clips = track.clips.filter(clip => clip.id !== id); });
            s.selectedClipId = '';
        } else if (action === 'duplicate') {
            mutate(s, action, project => {
                const track = project.tracks.find(item => item.id === found.track.id);
                if (totalClipCount(project) >= MAX_CLIPS) return showNotice(s, 'clipLimit', 'This project reached its clip limit.', true);
                const copy = clone(track.clips.find(clip => clip.id === id));
                copy.id = window.VideoStudioTimeline.newId('clip'); copy.start += copy.duration; copy.transition = null;
                track.clips.push(copy); s.selectedClipId = copy.id;
            });
        } else if (action === 'split') {
            const frame = s.frame, clip = found.clip;
            if (frame <= clip.start || frame >= clip.start + clip.duration) return showNotice(s, 'splitHint', 'Move the playhead inside the selected clip to split it.', true);
            mutate(s, action, project => {
                const target = project.tracks.find(item => item.id === found.track.id), original = target.clips.find(item => item.id === id);
                const firstDuration = frame - original.start;
                const second = clone(original);
                second.id = window.VideoStudioTimeline.newId('clip'); second.start = frame; second.offset += firstDuration; second.duration -= firstDuration; second.transition = null;
                original.fade_in = Math.min(Number(original.fade_in || 0), firstDuration); original.fade_out = 0;
                second.fade_in = 0; second.fade_out = Math.min(Number(second.fade_out || 0), second.duration);
                original.duration = firstDuration; original.transition = null; target.clips.push(second); s.selectedClipId = second.id;
            });
        }
    }

    function trackAction(s, id, action) {
        mutate(s, 'track', project => {
            const track = project.tracks.find(item => item.id === id);
            if (!track) return;
            if (action === 'mute') track.muted = !track.muted;
            if (action === 'hide') track.hidden = !track.hidden;
            if (action === 'lock') track.locked = !track.locked;
        });
    }

    function addTrack(s, kind) {
        const count = s.project.tracks.filter(track => track.kind === kind).length;
        if (count >= 4) return;
        mutate(s, 'add track', project => project.tracks.push({ id: window.VideoStudioTimeline.newId('track'), kind, name: tr(s, kind + 'Track', kind) + ' ' + (count + 1), muted: false, hidden: false, locked: false, clips: [] }));
    }

    function reorderTrack(s, from, to) {
        mutate(s, 'reorder tracks', project => {
            const a = project.tracks.findIndex(track => track.id === from), b = project.tracks.findIndex(track => track.id === to);
            if (a < 0 || b < 0) return;
            const [track] = project.tracks.splice(a, 1); project.tracks.splice(b, 0, track);
        });
    }

    async function loadJobs(s) {
        if (!s.projectId || s.disposed) return;
        const projectId = s.projectId, epoch = s.projectEpoch;
        const response = await request(API + '/jobs?project_id=' + encodeURIComponent(projectId), { signal: s.abort.signal });
        if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
        s.jobs = Array.isArray(response.body.jobs) ? response.body.jobs : [];
        s.jobs.forEach(job => {
            if (['succeeded', 'failed', 'cancelled', 'interrupted'].includes(job.status) && !s.finishedJobs?.has(job.id)) {
                if (!s.finishedJobs) s.finishedJobs = new Set();
                s.finishedJobs.add(job.id);
                if (job.status === 'succeeded') refreshProject(s).catch(() => {});
            }
        });
        renderJobs(s);
    }
    function jobFailure(job) {
        if (job.external_status_unknown && ['failed', 'cancelled', 'interrupted'].includes(job.status)) return ['generationStatusUnknown', 'The provider status is unclear; it may still be processing. Check its status before making another paid request.'];
        return ({ project_size_limit: ['projectStorageFull', 'This project has reached its storage limit. Remove media or use another project.'], asset_size_limit: ['fileTooLarge', 'The file is larger than the server limit.'], generation_import_failed: ['generatedImportFailed', 'The generated clip could not be added to this project.'] })[job.error] || ['jobFailed', 'This task could not be completed.'];
    }
    function renderJobs(s) {
        const host = s.q('[data-jobs]');
        if (!host) return;
        const visible = s.jobs.filter(job => !s.jobLocalStops.has(job.id));
        host.innerHTML = visible.length ? `<h3>${esc(s, tr(s, 'backgroundJobs', 'Background jobs'))}</h3>` + visible.map(job => {
            const percent = Math.round(clamp(Number(job.progress || 0), 0, 1) * 100);
            const terminal = ['succeeded', 'failed', 'cancelled', 'interrupted'].includes(job.status);
            const failure = job.status === 'failed' || (job.external_status_unknown && ['cancelled', 'interrupted'].includes(job.status)) ? jobFailure(job) : null;
            const artifact = job.artifact && safeSameOrigin(job.artifact.download_url);
            return `<article class="vs-job vs-job-${esc(s, job.status)}"><div class="vs-job-heading"><span class="vs-job-dot"></span><strong>${esc(s, tr(s, 'job_' + job.kind, job.kind))}</strong><span>${esc(s, tr(s, 'job_' + job.status, job.status))}</span></div><progress max="100" value="${percent}"></progress><div class="vs-job-footer"><small>${terminal ? '' : percent + '%'}</small><span>${artifact ? `<a href="${esc(s, artifact)}" download>${icon(s, 'download')} ${esc(s, tr(s, 'download', 'Download'))}</a>` : ''}${!terminal && job.kind !== 'generate' ? `<button type="button" data-cancel-job="${esc(s, job.id)}">${esc(s, tr(s, 'cancelJob', 'Cancel'))}</button>` : ''}${!terminal && job.kind === 'generate' ? `<button type="button" data-stop-watch="${esc(s, job.id)}">${esc(s, tr(s, 'stopTracking', 'Stop tracking'))}</button>` : ''}</span></div>${failure ? '<small class="vs-job-error">' + esc(s, tr(s, failure[0], failure[1])) + '</small>' : ''}</article>`;
        }).join('') : '';
    }

    function safeSameOrigin(raw) {
        try { const url = new URL(raw, location.origin); return url.origin === location.origin ? url.href : ''; } catch (_) { return ''; }
    }

    async function createJob(s, payload, expectedProjectId, expectedEpoch) {
        const projectId = expectedProjectId || s.projectId, epoch = expectedEpoch == null ? s.projectEpoch : expectedEpoch;
        if (!projectId || projectId !== s.projectId || epoch !== s.projectEpoch) throw new Error('project_changed');
        const key = idempotencyKey();
        let response;
        try {
            response = await request(API + '/projects/' + encodeURIComponent(projectId) + '/jobs', { method: 'POST', headers: { 'Content-Type': 'application/json', 'Idempotency-Key': key, 'If-Match': s.etag }, body: JSON.stringify(payload) });
        } catch (error) {
            if ((error.status === 412 || error.status === 428) && epoch === s.projectEpoch && projectId === s.projectId) { s.conflict = true; await showConflict(s); }
            throw error;
        }
        if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return null;
        const job = response.body.job || response.body;
        if (!s.jobs.some(item => item.id === job.id)) s.jobs.unshift(job);
        renderJobs(s);
        pollJob(s, job.id, projectId, epoch).catch(() => {});
        return job;
    }

    async function pollJob(s, jobId, projectId, epoch) {
        projectId = projectId || s.projectId;
        epoch = epoch == null ? s.projectEpoch : epoch;
        while (!s.disposed) {
            if (s.jobLocalStops.has(jobId)) return null;
            const response = await request(API + '/jobs/' + encodeURIComponent(jobId), { signal: s.abort.signal });
            if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId || s.jobLocalStops.has(jobId)) return null;
            const job = response.body.job || response.body;
            const idx = s.jobs.findIndex(item => item.id === jobId);
            if (idx >= 0) s.jobs[idx] = job; else s.jobs.unshift(job);
            renderJobs(s);
            if (['succeeded', 'failed', 'cancelled', 'interrupted'].includes(job.status)) {
                if (job.status === 'succeeded') await refreshProject(s);
                if (job.status === 'failed' || job.external_status_unknown && ['cancelled', 'interrupted'].includes(job.status)) { const failure = jobFailure(job); showNotice(s, failure[0], failure[1], true); }
                return job;
            }
            await new Promise(resolve => window.setTimeout(resolve, 1200));
        }
        return null;
    }

    async function cancelJob(s, id) {
        const projectId = s.projectId, epoch = s.projectEpoch;
        try { await request(API + '/jobs/' + encodeURIComponent(id) + '/cancel', { method: 'POST' }); if (epoch === s.projectEpoch && projectId === s.projectId) await loadJobs(s); }
        catch (_) { if (epoch === s.projectEpoch && projectId === s.projectId) showNotice(s, 'cancelFailed', 'Could not cancel the task.', true); }
    }

    async function refreshProject(s) {
        if (!s.projectId || s.disposed) return;
        const projectId = s.projectId, epoch = s.projectEpoch;
        const response = await request(API + '/projects/' + encodeURIComponent(projectId), { signal: s.abort.signal });
        if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
        if (s.timelineDragging) { s.pendingRefresh = true; return; }
        const next = hydrateProject(s, response.body.project || response.body);
        if (s.dirty) {
            const comparable = project => { const canonical = canonicalProject(project); canonical.assets = []; return JSON.stringify(canonical); };
            if (!s.savedProject || comparable(next) !== comparable(s.savedProject)) {
                s.conflict = true; await showConflict(s); return;
            }
            const known = new Set((s.project.assets || []).map(asset => asset.id));
            const additions = next.assets.filter(asset => !known.has(asset.id));
            s.project.assets = (s.project.assets || []).concat(additions);
            s.savedProject.assets = (s.savedProject.assets || []).concat(additions.map(({ media_url, ...asset }) => asset));
        } else {
            s.project = next;
            s.savedProject = canonicalProject(next);
        }
        s.etag = response.etag || s.etag;
        s.desktopPath = response.body.desktop_path || s.desktopPath;
        s.revision++;
        renderUI(s);
    }

    async function uploadFile(s, file, expectedProjectId, expectedEpoch) {
        const projectId = expectedProjectId || s.projectId, epoch = expectedEpoch == null ? s.projectEpoch : expectedEpoch;
        if (!file || s.readonly || !projectId || epoch !== s.projectEpoch || projectId !== s.projectId) return;
        const allowed = /^(video\/|audio\/|image\/(png|webp)$)/i.test(file.type) || /\.(mp4|mov|webm|m4v|mkv|mp3|wav|ogg|m4a|aac|flac|png|webp)$/i.test(file.name);
        if (!allowed) return showNotice(s, 'unsupportedFile', 'Choose a video, audio, PNG or WebP file.', true);
        const max = Number(s.status.limits && s.status.limits.max_asset_size_bytes || 256 * 1024 * 1024);
        if (file.size > max) return showNotice(s, 'fileTooLarge', 'This file is larger than the server limit.', true);
        const form = new FormData(); form.append('file', file, file.name);
        try {
            const response = await request(API + '/projects/' + encodeURIComponent(projectId) + '/media', { method: 'POST', headers: { 'Idempotency-Key': idempotencyKey() }, body: form });
            if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
            const job = response.body.job;
            if (job) { s.jobs.unshift(job); renderJobs(s); await pollJob(s, job.id, projectId, epoch); }
            else await refreshProject(s);
            if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
            showNotice(s, 'importReady', 'Media added to the bin. Drag it to a track to edit.', false);
        } catch (error) { if (!s.disposed && epoch === s.projectEpoch && projectId === s.projectId) showNotice(s, 'importFailed', 'Could not import this media.', true); }
    }

    async function importDesktopFile(s) {
        if (s.readonly || !s.projectId || !s.ctx.openFileDialog) return;
        const projectId = s.projectId, epoch = s.projectEpoch;
        const result = await s.ctx.openFileDialog({
            title: tr(s, 'browse', 'Browse Desktop files'), initialPath: 'Documents',
            filters: [{ label: tr(s, 'videoAudioImages', 'Video, audio and images'), extensions: ['.mp4','.mov','.webm','.m4v','.mkv','.mp3','.wav','.ogg','.m4a','.aac','.flac','.png','.webp'] }]
        });
        if (!result || result.canceled || !result.path || s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
        try {
            const response = await request(API + '/projects/' + encodeURIComponent(projectId) + '/media', { method: 'POST', headers: { 'Content-Type': 'application/json', 'Idempotency-Key': idempotencyKey() }, body: JSON.stringify({ source_path: result.path }) });
            if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
            if (response.body.job) { s.jobs.unshift(response.body.job); await pollJob(s, response.body.job.id, projectId, epoch); }
            else await refreshProject(s);
        } catch (_) { if (!s.disposed && epoch === s.projectEpoch && projectId === s.projectId) showNotice(s, 'importFailed', 'Could not import this Desktop file.', true); }
    }

    function scheduleSave(s) {
        s.dirty = true; s.revision++;
        persistDraft(s);
        clearTimeout(s.autosaveTimer);
        if (!s.timelineDragging && !s.conflict) s.autosaveTimer = window.setTimeout(() => saveProject(s).catch(() => {}), 1100);
        if (s.q('[data-save-state]')) { s.q('[data-save-state]').textContent = tr(s, 'unsaved', 'Unsaved changes'); s.q('[data-save-state]').classList.add('is-dirty'); }
    }

    function queueAutosave(s) {
        clearTimeout(s.autosaveTimer);
        s.autosaveTimer = window.setTimeout(() => { s.autosaveTimer = 0; saveProject(s).catch(() => {}); }, 1100);
    }

    async function saveProject(s) {
        if (s.timelineDragging) return false;
        if (!s.project || !s.projectId || !s.dirty) return true;
        if (s.readonly) { showNotice(s, 'readonly', 'Read-only mode prevents editing.', true); return false; }
        if (window.VideoStudioTimeline.validTimeline && !window.VideoStudioTimeline.validTimeline(s.project)) {
            showNotice(s, 'invalidTiming', 'That edit would create invalid timing or an overlap.', true);
            return false;
        }
        clearTimeout(s.autosaveTimer);
        if (s.savePromise) return s.savePromise;
        s.saving = true; s.saveError = ''; renderUI(s);
        const revision = s.revision, projectId = s.projectId, epoch = s.projectEpoch;
        const token = {};
        s.saveToken = token;
        const requestBody = canonicalProject(s.project);
        s.savePromise = (async () => {
            try {
                const response = await request(API + '/projects/' + encodeURIComponent(projectId), {
                    method: 'PUT', headers: { 'Content-Type': 'application/json', 'If-Match': s.etag }, body: JSON.stringify(requestBody)
                });
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
                        renderUI(s);
                        if (s.dirty && !s.conflict) scheduleSave(s);
                    }
                }
            }
        })();
        return s.savePromise;
    }

    async function showConflict(s) {
        const host = s.q('[data-conflict-host]');
        if (!host) return;
        host.innerHTML = `<div class="vs-modal-backdrop"><section class="vs-modal" role="dialog" aria-modal="true" aria-labelledby="vs-conflict-title"><span class="vs-modal-icon">↻</span><h2 id="vs-conflict-title">${esc(s, tr(s, 'conflictTitle', 'This project changed elsewhere'))}</h2><p>${esc(s, tr(s, 'conflictCopy', 'Choose which version to keep before continuing.'))}</p><div class="vs-modal-actions"><button type="button" data-conflict="reload">${esc(s, tr(s, 'reloadServer', 'Load server version'))}</button><button type="button" data-conflict="replace">${esc(s, tr(s, 'replaceServer', 'Replace the latest version'))}</button><button type="button" data-conflict="keep">${esc(s, tr(s, 'keepEditing', 'Keep editing'))}</button></div></section></div>`;
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
            } catch (_) { s.conflict = true; showNotice(s, 'conflictAgain', 'The project changed again. Review the conflict and try again.', true); }
        }
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
        let raw;
        try { raw = localStorage.getItem('video-studio-draft:' + projectId); } catch (_) { return; }
        if (!raw) return;
        try {
            const draft = JSON.parse(raw);
            const shouldRecover = draft.project && Date.now() - Number(draft.saved_at || 0) < 7 * 86400000 && await s.ctx.confirmDialog(tr(s, 'recoverTitle', 'Recover local draft?'), tr(s, 'recoverCopy', 'A recent local draft is available. Restore it over the saved project?'));
            if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
            if (shouldRecover) {
                s.project = hydrateProject(s, draft.project); s.dirty = true; s.history = [];
            }
        } catch (_) { /* malformed recovery data is ignored */ }
    }
    async function clearDraft(s) { try { localStorage.removeItem(draftKey(s)); } catch (_) {} }

    function applyTransition(s) {
        const found = window.VideoStudioTimeline.clipFor(s.project, s.selectedClipId);
        if (!found || s.readonly || found.track.locked) return;
        const select = s.q('[data-transition-type]'), durationInput = s.q('[data-transition-duration]');
        const type = select.value, duration = clamp(Number(durationInput.value || 15), 1, 90);
        const ordered = found.track.clips.slice().sort((a, b) => a.start - b.start);
        const idx = ordered.findIndex(clip => clip.id === found.clip.id), next = ordered[idx + 1];
        if (type !== 'none' && !next) return showNotice(s, 'transitionNeedsNext', 'Add a following clip on this track first.', true);
        if (type !== 'none' && duration >= Math.min(found.clip.duration, next.duration)) return showNotice(s, 'transitionTooLong', 'Transition duration must be shorter than both clips.', true);
        mutate(s, 'transition', project => {
            const track = project.tracks.find(item => item.id === found.track.id), clip = track.clips.find(item => item.id === found.clip.id);
            const target = track.clips.slice().sort((a, b) => a.start - b.start), at = target.findIndex(item => item.id === clip.id), following = target[at + 1];
            if (type === 'none') { clip.transition = null; if (following) following.start = clip.start + clip.duration; }
            else { clip.transition = { type, duration }; following.start = clip.start + clip.duration - duration; }
        });
    }
    function handleInput(s, event) {
        const input = event.target;
        if (s.readonly || s.timelineDragging) return;
        if (input.matches('[data-search]')) { renderAssets(s); return; }
        if (input.matches('[data-field]')) {
            const found = window.VideoStudioTimeline.clipFor(s.project, s.selectedClipId); if (!found) return;
            if (found.track.locked) return;
            const field = input.dataset.field;
            if (!input._vsBefore) input._vsBefore = snapshot(s);
            clearTimeout(s.autosaveTimer);
            let value = input.type === 'number' || input.type === 'range' ? Number(input.value) : input.value;
            if (field === 'start' || field === 'duration' || field === 'offset' || field === 'fade_in' || field === 'fade_out') value = Math.round(value);
            if (['x','y','width','height','opacity'].includes(field)) value = clamp(value, 0, 1);
            if (field === 'duration') value = Math.max(1, value);
            found.clip[field] = value;
            s.dirty = true; s.revision++;
            const saveState = s.q('[data-save-state]'); if (saveState) { saveState.textContent = tr(s, 'unsaved', 'Unsaved changes'); saveState.classList.add('is-dirty'); }
            if (field === 'opacity' || field === 'x' || field === 'y' || field === 'width' || field === 'height' || field === 'fit') s.preview.seek(s.frame);
            const output = input.parentElement.querySelector('output'); if (output) output.textContent = Math.round(value * 100) + '%';
            return;
        }
        if (input.matches('[data-text-field="text"]')) {
            const found = window.VideoStudioTimeline.clipFor(s.project, s.selectedClipId); if (!found) return;
            if (found.track.locked) return;
            if (!input._vsBefore) input._vsBefore = snapshot(s);
            clearTimeout(s.autosaveTimer); found.clip.text = input.value; s.dirty = true; s.revision++; return;
        }
        if (input.matches('[data-text-style]')) {
            const found = window.VideoStudioTimeline.clipFor(s.project, s.selectedClipId); if (!found) return;
            if (found.track.locked) return;
            if (!input._vsBefore) input._vsBefore = snapshot(s);
            const style = found.clip.text_style || (found.clip.text_style = {});
            style[input.dataset.textStyle] = ['font_size', 'background_opacity'].includes(input.dataset.textStyle) ? Number(input.value) : input.value;
            clearTimeout(s.autosaveTimer); s.dirty = true; s.revision++; return;
        }
    }
    function handleChange(s, event) {
        const input = event.target;
        if (input.matches('[data-project-picker]')) {
            const targetId = input.value;
            if (!targetId || targetId === s.projectId) return;
            if (s.timelineDragging) { input.value = s.projectId; return; }
            const open = async () => {
                if (s.dirty && !(await saveUntilClean(s))) { input.value = s.projectId; return; }
                try { if (!(await loadProject(s, targetId))) input.value = s.projectId; clearNotice(s); }
                catch (_) { input.value = s.projectId; showNotice(s, 'loadFailed', 'Could not open this project.', true); }
            };
            open();
            return;
        }
        if (input.matches('[data-canvas]')) {
            const [width, height] = input.value.split('x').map(Number);
            if (width && height) mutate(s, 'canvas', project => { project.width = width; project.height = height; });
            return;
        }
        if (input.matches('[data-field]') || input.matches('[data-text-field]') || input.matches('[data-text-style]')) {
            const before = input._vsBefore; delete input._vsBefore;
            if (before && window.VideoStudioTimeline.validTimeline && !window.VideoStudioTimeline.validTimeline(s.project)) {
                s.project = hydrateProject(s, before);
                s.dirty = JSON.stringify(canonicalProject(s.project)) !== JSON.stringify(s.savedProject);
                s.revision++;
                if (s.dirty) persistDraft(s); else clearDraft(s);
                showNotice(s, 'invalidTiming', 'That edit would create invalid timing or an overlap.', true);
                renderUI(s); return;
            }
            if (before) { s.history.push(before); if (s.history.length > 50) s.history.shift(); s.redo = []; }
            s.dirty = JSON.stringify(canonicalProject(s.project)) !== JSON.stringify(s.savedProject);
            if (s.dirty) { persistDraft(s); scheduleSave(s); } else clearDraft(s);
            renderInspector(s); s.preview.seek(s.frame); return;
        }
        if (input.matches('[data-transition-type]')) {
            const duration = s.q('[data-transition-duration]'); duration.disabled = input.value === 'none';
        }
    }
    function handleClick(s, event) {
        const place = event.target.closest('[data-place-asset]');
        if (place) { addAssetToTrack(s, place.dataset.placeAsset, place.dataset.placeAsset && s.project.assets.find(asset => asset.id === place.dataset.placeAsset)?.kind === 'audio' ? (s.project.tracks.find(track => track.kind === 'audio' && !track.locked) || s.project.tracks.find(track => track.kind === 'audio'))?.id : firstVideoTrack(s.project), s.frame); return; }
        const cancel = event.target.closest('[data-cancel-job]'); if (cancel) { cancelJob(s, cancel.dataset.cancelJob); return; }
        const stopWatch = event.target.closest('[data-stop-watch]'); if (stopWatch) { s.jobLocalStops.add(stopWatch.dataset.stopWatch); renderJobs(s); return; }
        const button = event.target.closest('[data-action]'); if (!button) return;
        const action = button.dataset.action;
        if (s.readonly) {
            if (['play','step-back','step-forward'].includes(action)) { /* preview remains available in read-only mode */ }
            else if (action === 'toggle-inspector' || action === 'close-inspector') { /* read-only inspector remains available */ }
            else return showNotice(s, 'readonly', 'Read-only mode prevents editing.', true);
        }
        if (action === 'new-project') createProjectPrompt(s);
        else if (action === 'toggle-inspector') setInspectorOpen(s, !s.q('.vs-app').classList.contains('vs-show-inspector'), true);
        else if (action === 'close-inspector') {
            setInspectorOpen(s, false, false);
            s.q('.vs-inspector-toggle')?.focus();
        }
        else if (action === 'upload' || action === 'add-media') s.q('[data-file-input]').click();
        else if (action === 'browse') importDesktopFile(s);
        else if (action === 'save') saveProject(s);
        else if (action === 'undo') undo(s, false);
        else if (action === 'redo') undo(s, true);
        else if (action === 'play') s.preview.isPlaying() ? s.preview.pause() : s.preview.play();
        else if (action === 'step-back') s.preview.pause(), s.preview.seek(s.frame - 1);
        else if (action === 'step-forward') s.preview.pause(), s.preview.seek(s.frame + 1);
        else if (action === 'export') openExport(s);
        else if (action === 'apply-transition') applyTransition(s);
        else if (action === 'add-title') addTitle(s);
        else if (action === 'apply-text') applyTextArtwork(s);
        else if (action === 'open-ai') openAI(s);
        else if (action === 'open-stickers') openStickerPicker(s);
    }
    function setInspectorOpen(s, open, moveFocus) {
        const app = s.q('.vs-app'), inspector = s.q('.vs-inspector');
        const narrow = app.getBoundingClientRect().width <= 920;
        app.classList.toggle('vs-show-inspector', !!open && narrow);
        if (narrow && open) {
            inspector.setAttribute('role', 'dialog'); inspector.setAttribute('aria-modal', 'true');
            if (moveFocus) inspector.querySelector('button:not(:disabled),input:not(:disabled),select:not(:disabled),textarea:not(:disabled)')?.focus();
        } else {
            inspector.removeAttribute('role'); inspector.removeAttribute('aria-modal');
        }
    }
    function handleKeys(s, event) {
        const target = event.target, modifier = event.ctrlKey || event.metaKey, undoShortcut = modifier && !event.altKey && event.key.toLowerCase() === 'z';
        if (s.disposed || event.defaultPrevented || !s.project || !target || !s.host.contains(target) || typeof target.closest !== 'function') return;
        if ((event.ctrlKey || event.metaKey || event.altKey) && !undoShortcut) return;
        if (event.key === 'Escape' && s.q('.vs-app').classList.contains('vs-show-inspector')) { setInspectorOpen(s, false, false); s.q('.vs-inspector-toggle')?.focus(); event.preventDefault(); return; }
        if (event.key === 'Tab' && s.q('.vs-app').classList.contains('vs-show-inspector') && s.q('.vs-app').getBoundingClientRect().width <= 920) {
            const items = Array.from(s.q('.vs-inspector').querySelectorAll('button:not(:disabled),input:not(:disabled),select:not(:disabled),textarea:not(:disabled)')).filter(item => item.getClientRects().length);
            if (items.length) { const first = items[0], last = items[items.length - 1]; if (event.shiftKey && (document.activeElement === first || !s.q('.vs-inspector').contains(document.activeElement))) { event.preventDefault(); last.focus(); } else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); } }
        }
        if (target.closest('input,textarea,select,[contenteditable="true"]')) return;
        if (undoShortcut) { event.preventDefault(); undo(s, event.shiftKey); return; }
        if (target.closest('button,a,summary,[role="button"],[role="link"]')) return;
        if (event.code === 'Space') { event.preventDefault(); s.preview.isPlaying() ? s.preview.pause() : s.preview.play(); }
        else if (event.key === 'Delete' || event.key === 'Backspace') { if (s.selectedClipId) { event.preventDefault(); timelineAction(s, 'delete', s.selectedClipId); } }
        else if (event.key === 'ArrowLeft') { event.preventDefault(); s.preview.pause(); s.preview.seek(s.frame - (event.shiftKey ? FPS : 1)); }
        else if (event.key === 'ArrowRight') { event.preventDefault(); s.preview.pause(); s.preview.seek(s.frame + (event.shiftKey ? FPS : 1)); }
        else if (event.key.toLowerCase() === 's' && s.selectedClipId) { event.preventDefault(); timelineAction(s, 'split', s.selectedClipId); }
    }
    function addTitle(s) {
        if (s.readonly || totalClipCount(s.project) >= MAX_CLIPS) return;
        const projectId = s.projectId, epoch = s.projectEpoch, startFrame = s.frame;
        const track = s.project.tracks.find(item => item.kind === 'overlay' && !item.locked) || (window.VideoStudioTimeline.trackCapacity(s.project, 'overlay') ? (addTrack(s, 'overlay'), s.project.tracks.find(item => item.kind === 'overlay' && !item.locked)) : null);
        if (!track) return showNotice(s, 'trackLimit', 'Add an overlay track before creating a title.', true);
        const title = tr(s, 'defaultTitle', 'Your title');
        const style = { font_family: 'Arial', font_size: 72, color: '#ffffff', bold: true, alignment: 'center', background_color: '#101319', background_opacity: 0.45 };
        renderTextPNG(s, title, style).then(file => createArtworkAsset(s, file, title, projectId, epoch)).then(asset => {
            if (!asset || s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
            const target = s.project.tracks.find(item => item.id === track.id) || s.project.tracks.find(item => item.kind === 'overlay');
            const clip = window.VideoStudioTimeline.makeClip(asset, startFrame);
            clip.duration = FPS * 5;
            clip.x = 0; clip.y = 0; clip.width = 1; clip.height = 1; clip.text = title; clip.text_style = style;
            mutate(s, 'add title', project => project.tracks.find(item => item.id === target.id).clips.push(clip));
            s.selectedClipId = clip.id; renderInspector(s);
        }).catch(() => { if (!s.disposed && epoch === s.projectEpoch && projectId === s.projectId) showNotice(s, 'artworkFailed', 'Could not create the title artwork.', true); });
    }
    function renderTextPNG(s, text, style) {
        const canvas = document.createElement('canvas'); canvas.width = s.project.width; canvas.height = s.project.height;
        const ctx = canvas.getContext('2d');
        const font = `${style.bold ? '700 ' : ''}${style.italic ? 'italic ' : ''}${Number(style.font_size || 72)}px "${String(style.font_family || 'Arial').replace(/["\\]/g, '')}"`;
        ctx.font = font; ctx.textAlign = ['left','right'].includes(style.alignment) ? style.alignment : 'center'; ctx.textBaseline = 'middle';
        const lines = String(text || '').split(/\r?\n/).slice(0, 5); const lineHeight = Number(style.font_size || 72) * 1.2;
        const maxWidth = canvas.width * 0.84;
        const wrapped = lines.flatMap(line => wrapCanvasText(ctx, line, maxWidth));
        const blockHeight = wrapped.length * lineHeight;
        const anchorX = style.alignment === 'left' ? canvas.width * 0.08 : style.alignment === 'right' ? canvas.width * 0.92 : canvas.width / 2;
        const anchorY = canvas.height * 0.78;
        const padX = 42, padY = 22;
        if (Number(style.background_opacity || 0) > 0) {
            ctx.fillStyle = hexWithAlpha(style.background_color || '#101319', Number(style.background_opacity || 0.45));
            const width = Math.min(maxWidth + padX * 2, Math.max(...wrapped.map(line => ctx.measureText(line).width)) + padX * 2);
            const x = style.alignment === 'left' ? anchorX - padX : style.alignment === 'right' ? anchorX - width + padX : anchorX - width / 2;
            ctx.fillRect(x, anchorY - blockHeight / 2 - padY, width, blockHeight + padY * 2);
        }
        ctx.font = font; ctx.fillStyle = style.color || '#fff'; ctx.shadowColor = 'rgba(0,0,0,.68)'; ctx.shadowBlur = 7;
        wrapped.forEach((line, index) => ctx.fillText(line, anchorX, anchorY - blockHeight / 2 + lineHeight * (index + 0.5), maxWidth));
        return new Promise((resolve, reject) => canvas.toBlob(blob => blob ? resolve(new File([blob], 'title-' + Date.now() + '.png', { type: 'image/png' })) : reject(new Error('canvas_export_failed')), 'image/png'));
    }
    function wrapCanvasText(ctx, text, maxWidth) {
        const words = String(text || ' ').split(/\s+/), lines = []; let line = '';
        words.forEach(word => {
            const candidate = line ? line + ' ' + word : word;
            if (line && ctx.measureText(candidate).width > maxWidth) { lines.push(line); line = word; } else line = candidate;
        });
        if (line) lines.push(line); return lines;
    }
    function hexWithAlpha(hex, alpha) {
        const value = /^#[0-9a-f]{6}$/i.test(hex) ? hex : '#101319';
        return `rgba(${parseInt(value.slice(1, 3), 16)},${parseInt(value.slice(3, 5), 16)},${parseInt(value.slice(5, 7), 16)},${clamp(alpha, 0, 1)})`;
    }
    function textStyleKey(style) { const defaults = { font_family: 'Arial', font_size: 72, color: '#ffffff', bold: false, italic: false, alignment: 'center', outline_color: '', outline_width: 0, background_color: '#101319', background_opacity: 0 }; return Object.keys(defaults).map(key => style && style[key] || defaults[key]).join('|'); }
    async function applyTextArtwork(s) {
        const found = window.VideoStudioTimeline.clipFor(s.project, s.selectedClipId); if (!found || !found.clip.text) return;
        const projectId = s.projectId, epoch = s.projectEpoch, clipId = found.clip.id, selectionRevision = s.selectionRevision;
        const revision = ++s.artworkRevision, originalAssetId = found.clip.asset_id, text = found.clip.text, style = clone(found.clip.text_style || {}), styleKey = textStyleKey(style);
        try {
            const file = await renderTextPNG(s, text, style);
            const asset = await createArtworkAsset(s, file, 'Title · ' + text.slice(0, 40), projectId, epoch);
            if (!asset || s.disposed || revision !== s.artworkRevision || epoch !== s.projectEpoch || projectId !== s.projectId || s.selectedClipId !== clipId || s.selectionRevision !== selectionRevision) return;
            const current = window.VideoStudioTimeline.clipFor(s.project, clipId);
            if (!current || current.clip.text !== text || textStyleKey(current.clip.text_style || {}) !== styleKey || current.clip.asset_id !== originalAssetId) return;
            mutate(s, 'update title artwork', project => {
                const target = window.VideoStudioTimeline.clipFor(project, clipId);
                if (target) target.clip.asset_id = asset.id;
            });
        } catch (_) { if (!s.disposed && epoch === s.projectEpoch && projectId === s.projectId) showNotice(s, 'artworkFailed', 'Could not update the title artwork.', true); }
    }
    async function createArtworkAsset(s, file, name, expectedProjectId, expectedEpoch) {
        const projectId = expectedProjectId || s.projectId, epoch = expectedEpoch == null ? s.projectEpoch : expectedEpoch;
        if (!projectId || projectId !== s.projectId || epoch !== s.projectEpoch || s.readonly) throw new Error('project_changed');
        const form = new FormData(); form.append('file', file, file.name || 'overlay.png');
        const response = await request(API + '/projects/' + encodeURIComponent(projectId) + '/media', { method: 'POST', headers: { 'Idempotency-Key': idempotencyKey() }, body: form });
        if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) throw new Error('project_changed');
        if (!response.body.job) return response.body.asset;
        s.jobs.unshift(response.body.job); renderJobs(s);
        const job = await pollJob(s, response.body.job.id, projectId, epoch);
        if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) throw new Error('project_changed');
        if (!job || job.status !== 'succeeded') throw new Error('probe_failed');
        await refreshProject(s);
        if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) throw new Error('project_changed');
        const assetId = job.result && job.result.asset && job.result.asset.id || response.body.asset && response.body.asset.id;
        const canonical = s.project.assets.find(item => item.id === assetId);
        if (canonical) return canonical;
        throw new Error('asset_not_ready');
    }
    function stickerSVG(name) {
        const art = {
            sparkle: '<path d="M48 4 58 37 92 48 58 59 48 92 38 59 4 48 38 37z"/><circle cx="78" cy="22" r="5"/><circle cx="20" cy="78" r="4"/>',
            heart: '<path d="M48 82 15 51C-2 34 10 10 29 13c9 1 15 7 19 15 4-8 11-14 20-15 19-3 31 21 14 38z"/>',
            sun: '<circle cx="48" cy="48" r="19"/><path d="M48 5v17m0 52v17M5 48h17m52 0h17M18 18l12 12m36 36 12 12m4-60L70 30M30 66 18 78"/>',
            star: '<path d="m48 6 11 28 30 2-23 19 8 30-26-17-26 17 8-30L7 36l30-2z"/>',
            flower: '<g><ellipse cx="48" cy="24" rx="12" ry="23"/><ellipse cx="48" cy="72" rx="12" ry="23"/><ellipse cx="24" cy="48" rx="23" ry="12"/><ellipse cx="72" cy="48" rx="23" ry="12"/><circle cx="48" cy="48" r="11"/></g>',
            burst: '<path d="M48 4 56 35 82 16 65 42 96 48 65 55 82 80 56 62 48 94 41 62 15 80 32 55 1 48 32 42 15 16 41 35z"/>'
        };
        const fill = name === 'heart' ? '#fb7185' : name === 'sun' ? '#fbbf24' : '#a78bfa';
        return `<svg xmlns="http://www.w3.org/2000/svg" width="512" height="512" viewBox="0 0 96 96"><g fill="${fill}" stroke="#fff" stroke-width="2" stroke-linejoin="round">${art[name] || art.sparkle}</g></svg>`;
    }
    async function stickerPNG(name) {
        const blob = new Blob([stickerSVG(name)], { type: 'image/svg+xml' }); const url = URL.createObjectURL(blob);
        try {
            const image = new Image(); image.src = url; await image.decode();
            const canvas = document.createElement('canvas'); canvas.width = 512; canvas.height = 512; canvas.getContext('2d').drawImage(image, 0, 0);
            return await new Promise((resolve, reject) => canvas.toBlob(blobValue => blobValue ? resolve(new File([blobValue], 'sticker-' + name + '.png', { type: 'image/png' })) : reject(new Error('canvas_export_failed')), 'image/png'));
        } finally { URL.revokeObjectURL(url); }
    }
    async function openStickerPicker(s) {
        const host = s.q('[data-modal-host]');
        host.innerHTML = `<div class="vs-modal-backdrop"><section class="vs-modal vs-sticker-modal" role="dialog" aria-modal="true"><button type="button" class="vs-modal-close" data-modal-close aria-label="${esc(s, tr(s, 'close', 'Close'))}">×</button><span class="vs-eyebrow">${esc(s, tr(s, 'overlayPack', 'Overlay pack'))}</span><h2>${esc(s, tr(s, 'sticker', 'Sticker'))}</h2><p>${esc(s, tr(s, 'stickerHint', 'Add a small original graphic to the overlay track.'))}</p><div class="vs-sticker-grid">${['sparkle','heart','sun','star','flower','burst'].map(name => `<button type="button" data-sticker="${name}"><span>${name === 'heart' ? '♥' : name === 'sun' ? '☼' : name === 'star' ? '★' : name === 'flower' ? '✿' : name === 'burst' ? '✹' : '✦'}</span><small>${esc(s, tr(s, 'sticker_' + name, name))}</small></button>`).join('')}</div></section></div>`;
        host.querySelector('[data-modal-close]').addEventListener('click', () => host.replaceChildren(), { once: true });
        host.querySelectorAll('[data-sticker]').forEach(button => button.addEventListener('click', async () => {
            const name = button.dataset.sticker, projectId = s.projectId, epoch = s.projectEpoch, startFrame = s.frame; host.replaceChildren();
            try {
                const file = await stickerPNG(name);
                if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
                const asset = await createArtworkAsset(s, file, tr(s, 'sticker_' + name, name), projectId, epoch);
                if (!asset || s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
                let track = s.project.tracks.find(item => item.kind === 'overlay' && !item.locked);
                if (!track) { if (!window.VideoStudioTimeline.trackCapacity(s.project, 'overlay')) return showNotice(s, 'trackLimit', 'Add an overlay track first.', true); addTrack(s, 'overlay'); track = s.project.tracks.find(item => item.kind === 'overlay' && !item.locked); }
                if (!track) return;
                const clip = window.VideoStudioTimeline.makeClip(asset, startFrame); clip.duration = FPS * 5; clip.width = 0.22; clip.height = 0.22; clip.x = 0.72; clip.y = 0.08;
                mutate(s, 'add sticker', project => project.tracks.find(item => item.id === track.id).clips.push(clip)); s.selectedClipId = clip.id; renderInspector(s);
            } catch (_) { if (!s.disposed && epoch === s.projectEpoch && projectId === s.projectId) showNotice(s, 'artworkFailed', 'Could not add this sticker.', true); }
        }));
    }
    function openExport(s) {
        if (!s.projectId || s.readonly) return;
        const projectId = s.projectId, epoch = s.projectEpoch;
        const host = s.q('[data-modal-host]');
        const size = `${s.project.width}x${s.project.height}`;
        host.innerHTML = `<div class="vs-modal-backdrop"><form class="vs-modal" data-export-form role="dialog" aria-modal="true"><button type="button" class="vs-modal-close" data-modal-close aria-label="${esc(s, tr(s, 'close', 'Close'))}">×</button><span class="vs-eyebrow">${esc(s, tr(s, 'delivery', 'Delivery'))}</span><h2>${esc(s, tr(s, 'exportVideo', 'Export video'))}</h2><p>${esc(s, tr(s, 'exportHint', 'MP4 · 30 fps · up to 10 minutes'))}</p><label>${esc(s, tr(s, 'resolution', 'Resolution and aspect ratio'))}<select data-export-size>${s.q('[data-canvas]').innerHTML}</select></label><div class="vs-export-summary"><span>MP4</span><span>30 fps</span><span>≤ 10:00</span></div><div class="vs-modal-actions"><button type="button" data-modal-close>${esc(s, tr(s, 'cancel', 'Cancel'))}</button><button type="submit" class="vs-primary">${esc(s, tr(s, 'startExport', 'Start export'))}</button></div></form></div>`;
        host.querySelector('[data-export-size]').value = size;
        host.querySelectorAll('[data-modal-close]').forEach(button => button.addEventListener('click', () => host.replaceChildren(), { once: true }));
        host.querySelector('[data-export-form]').addEventListener('submit', async event => {
            event.preventDefault();
            if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) { host.replaceChildren(); return; }
            const [width, height] = host.querySelector('[data-export-size]').value.split('x').map(Number);
            if (width !== s.project.width || height !== s.project.height) mutate(s, 'export canvas', project => { project.width = width; project.height = height; });
            host.replaceChildren();
            if (!(await saveProject(s))) return;
            if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
            try { await createJob(s, { kind: 'render' }, projectId, epoch); if (epoch === s.projectEpoch && projectId === s.projectId) showNotice(s, 'exportStarted', 'Export started. You can keep editing while it runs.', false); }
            catch (_) { if (epoch === s.projectEpoch && projectId === s.projectId) showNotice(s, 'exportFailed', 'Could not start the export.', true); }
        });
    }
    async function openAI(s) {
        const generation = s.status && s.status.generation;
        if (!generation || !generation.enabled || !generation.configured || generation.budget_blocked || s.readonly) return showNotice(s, 'generationUnavailable', 'Video generation is not available right now.', true);
        const projectId = s.projectId, epoch = s.projectEpoch;
        const host = s.q('[data-modal-host]');
        const durations = generation.durations_seconds || [];
        const supportsAspectRatio = String(generation.provider || '').toLowerCase() !== 'minimax';
        const images = s.project.assets.filter(asset => asset.kind === 'image');
        const supportsStartImage = Array.isArray(generation.image_modes) && generation.image_modes.includes('first_frame');
        const imageField = supportsStartImage ? `<label>${esc(s, tr(s, 'startImage', 'Start image (optional)'))}<select data-ai-image><option value="">${esc(s, tr(s, 'none', 'None'))}</option>${images.map(asset => `<option value="${esc(s, asset.id)}">${esc(s, asset.name)}</option>`).join('')}</select></label>` : '';
        host.innerHTML = `<div class="vs-modal-backdrop"><form class="vs-modal vs-ai-modal" data-ai-form role="dialog" aria-modal="true"><button type="button" class="vs-modal-close" data-modal-close aria-label="${esc(s, tr(s, 'close', 'Close'))}">×</button><span class="vs-eyebrow">${esc(s, tr(s, 'creativeAssist', 'Creative assist'))}</span><h2>✦ ${esc(s, tr(s, 'generate', 'Generate a clip'))}</h2><p>${esc(s, tr(s, 'aiHint', 'Generated clips appear in your media bin. Add them to the timeline when you are ready.'))}</p><label>${esc(s, tr(s, 'prompt', 'Describe the clip'))}<textarea data-ai-prompt rows="4" maxlength="2000" required placeholder="${esc(s, tr(s, 'promptHint', 'A slow aerial view over a quiet coastline at dawn'))}"></textarea></label><label>${esc(s, tr(s, 'duration', 'Duration'))}<select data-ai-duration>${durations.map(value => `<option value="${Number(value)}">${Number(value)} ${esc(s, tr(s, 'seconds', 'seconds'))}</option>`).join('')}</select></label>${imageField}<div class="vs-ai-provider">${esc(s, tr(s, 'usingModel', 'Using configured generation provider'))}${generation.provider ? ' · ' + esc(s, generation.provider) : ''}</div><div class="vs-modal-actions"><button type="button" data-modal-close>${esc(s, tr(s, 'cancel', 'Cancel'))}</button><button type="submit" class="vs-primary">✦ ${esc(s, tr(s, 'generate', 'Generate'))}</button></div></form></div>`;
        const ratioLabel = document.createElement('label');
        ratioLabel.append(document.createTextNode(tr(s, 'aspectRatio', 'Aspect ratio')));
        const ratioSelect = document.createElement('select'); ratioSelect.dataset.aiRatio = '';
        [['16:9','ratioLandscape','16:9'],['9:16','ratioPortrait','9:16'],['1:1','ratioSquare','1:1']].forEach(([value,key,fallback]) => {
            const option = document.createElement('option'); option.value = value; option.textContent = tr(s, key, fallback); ratioSelect.append(option);
        });
        ratioSelect.value = s.project.width === s.project.height ? '1:1' : s.project.width > s.project.height ? '16:9' : '9:16';
        ratioLabel.append(ratioSelect); if (supportsAspectRatio) host.querySelector('.vs-ai-provider').before(ratioLabel);
        host.querySelectorAll('[data-modal-close]').forEach(button => button.addEventListener('click', () => host.replaceChildren(), { once: true }));
        host.querySelector('[data-ai-form]').addEventListener('submit', async event => {
            event.preventDefault();
            const prompt = host.querySelector('[data-ai-prompt]').value.trim();
            const duration = Number(host.querySelector('[data-ai-duration]').value);
            if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) { host.replaceChildren(); return; }
            const firstFrame = host.querySelector('[data-ai-image]')?.value || '';
            const payload = { kind: 'generate', prompt, duration_seconds: duration }; if (supportsAspectRatio) payload.aspect_ratio = ratioSelect.value;
            if (firstFrame) payload.first_frame_asset_id = firstFrame;
            host.replaceChildren();
            try { await createJob(s, payload, projectId, epoch); if (epoch === s.projectEpoch && projectId === s.projectId) showNotice(s, 'generationStarted', 'Generation started. Results will appear in your media bin.', false); }
            catch (error) { if (epoch === s.projectEpoch && projectId === s.projectId) { const failure = jobFailure({ status: 'failed', error: error.body && error.body.code }); showNotice(s, failure[0] === 'jobFailed' ? 'generationFailed' : failure[0], failure[0] === 'jobFailed' ? 'Could not start generation.' : failure[1], true); } }
        });
    }

    window.VideoStudioApp = { render: appRender, dispose, instances };
})();
