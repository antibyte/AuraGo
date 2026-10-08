(function () {
    'use strict';

    const P = window.VideoStudioParts;
    const { API, CANVASES, FPS, MAX_ACTIVE_ASSETS, MAX_CLIPS, POPOVERS, PREF_SNAP, PREF_TIMELINE, READ_ONLY_SAFE, STICKERS, TITLE_PRESETS } = P;
    const I = (...args) => P.I(...args);
    const T = (...args) => P.T(...args);
    const activeAssetCount = (...args) => P.activeAssetCount(...args);
    const assetDisplayName = (...args) => P.assetDisplayName(...args);
    const addSticker = (...args) => P.addSticker(...args);
    const addTitle = (...args) => P.addTitle(...args);
    const addTrackTo = (...args) => P.addTrackTo(...args);
    const applyTextArtwork = (...args) => P.applyTextArtwork(...args);
    const applyTransition = (...args) => P.applyTransition(...args);
    const cancelJob = (...args) => P.cancelJob(...args);
    const canonicalProject = (...args) => P.canonicalProject(...args);
    const clearNotice = (...args) => P.clearNotice(...args);
    const clipMedia = (...args) => P.clipMedia(...args);
    const clone = (...args) => P.clone(...args);
    const closePopovers = (...args) => P.closePopovers(...args);
    const closePopoversOutside = (...args) => P.closePopoversOutside(...args);
    const commitField = (...args) => P.commitField(...args);
    const commitText = (...args) => P.commitText(...args);
    const defaultProject = (...args) => P.defaultProject(...args);
    const defaultTracks = (...args) => P.defaultTracks(...args);
    const esc = (...args) => P.esc(...args);
    const fileDrag = (...args) => P.fileDrag(...args);
    const focusKey = (...args) => P.focusKey(...args);
    const freeStart = (...args) => P.freeStart(...args);
    const handleTabKeys = (...args) => P.handleTabKeys(...args);
    const hydrateProject = (...args) => P.hydrateProject(...args);
    const icon = (...args) => P.icon(...args);
    const importDesktopFile = (...args) => P.importDesktopFile(...args);
    const importFiles = (...args) => P.importFiles(...args);
    const isFileDrag = (...args) => P.isFileDrag(...args);
    const liveField = (...args) => P.liveField(...args);
    const liveText = (...args) => P.liveText(...args);
    const loadJobs = (...args) => P.loadJobs(...args);
    const maxFrames = (...args) => P.maxFrames(...args);
    const mutate = (...args) => P.mutate(...args);
    const mutateSelected = (...args) => P.mutateSelected(...args);
    const nextStart = (...args) => P.nextStart(...args);
    const openAI = (...args) => P.openAI(...args);
    const openExport = (...args) => P.openExport(...args);
    const projectDuration = (...args) => P.projectDuration(...args);
    const queueAutosave = (...args) => P.queueAutosave(...args);
    const readPref = (...args) => P.readPref(...args);
    const recordChange = (...args) => P.recordChange(...args);
    const recoverDraft = (...args) => P.recoverDraft(...args);
    const refreshProject = (...args) => P.refreshProject(...args);
    const renameProject = (...args) => P.renameProject(...args);
    const renderAIPanel = (...args) => P.renderAIPanel(...args);
    const renderAssets = (...args) => P.renderAssets(...args);
    const renderFilters = (...args) => P.renderFilters(...args);
    const renderJobs = (...args) => P.renderJobs(...args);
    const renderJobsButton = (...args) => P.renderJobsButton(...args);
    const renderPreviewEmpty = (...args) => P.renderPreviewEmpty(...args);
    const renderProjectPopover = (...args) => P.renderProjectPopover(...args);
    const request = (...args) => P.request(...args);
    const restoreFocus = (...args) => P.restoreFocus(...args);
    const saveProject = (...args) => P.saveProject(...args);
    const saveUntilClean = (...args) => P.saveUntilClean(...args);
    const scheduleMediaRefresh = (...args) => P.scheduleMediaRefresh(...args);
    const scheduleSave = (...args) => P.scheduleSave(...args);
    const scheduleTextApply = (...args) => P.scheduleTextApply(...args);
    const selectedClip = (...args) => P.selectedClip(...args);
    const setInspectorOpen = (...args) => P.setInspectorOpen(...args);
    const setTab = (...args) => P.setTab(...args);
    const showNotice = (...args) => P.showNotice(...args);
    const stickerSVG = (...args) => P.stickerSVG(...args);
    const syncDrawer = (...args) => P.syncDrawer(...args);
    const togglePopover = (...args) => P.togglePopover(...args);
    const totalClipCount = (...args) => P.totalClipCount(...args);
    const tr = (...args) => P.tr(...args);
    const undo = (...args) => P.undo(...args);
    const updateOutput = (...args) => P.updateOutput(...args);
    const updateTransform = (...args) => P.updateTransform(...args);
    const upload = (...args) => P.upload(...args);
    const wireResize = (...args) => P.wireResize(...args);
    const wireTransform = (...args) => P.wireTransform(...args);
    const writePref = (...args) => P.writePref(...args);

    const instances = new Map();
    function dispose(id) {
        const s = instances.get(id);
        if (!s) return;
        s.disposed = true;
        s.abort.abort();
        clearTimeout(s.autosaveTimer);
        clearTimeout(s.toastTimer);
        clearTimeout(s.textTimer);
        clearInterval(s.jobsTimer);
        s.xhrs.forEach(xhr => { try { xhr.abort(); } catch (_) { /* finished */ } });
        if (s.resizeObserver) s.resizeObserver.disconnect();
        if (s.preview) s.preview.dispose();
        if (s.media) s.media.dispose();
        if (s.detachTimeline) s.detachTimeline();
        if (s.keyHandler) window.removeEventListener('keydown', s.keyHandler, true);
        if (s.ctx.clearWindowMenus) s.ctx.clearWindowMenus(id);
        instances.delete(id);
    }
    function layoutMarkup(s) {
        const label = (key, fallback) => esc(s, tr(s, key, fallback));
        const tab = (name, iconName, key, fallback) => `<button type="button" role="tab" data-tab="${name}" aria-selected="${name === 'media'}" aria-controls="vs-panel-${s.id}-${name}" id="vs-tab-${s.id}-${name}" ${name === 'media' ? '' : 'tabindex="-1"'}>${icon(iconName, 16)}<span>${label(key, fallback)}</span></button>`;
        const presets = Object.keys(TITLE_PRESETS).map(name => `<button type="button" class="vs-preset vs-preset-${name}" data-action="add-title" data-preset="${name}"><span class="vs-preset-sample" aria-hidden="true">${label('presetSample_' + name, name === 'lower' ? 'Name' : 'Aa')}</span><span class="vs-preset-text"><strong>${label('preset_' + name, name)}</strong><small>${label('presetHint_' + name, '')}</small></span>${icon('plus', 16, 'vs-icon vs-preset-add')}</button>`).join('');
        const stickers = STICKERS.map(name => `<button type="button" class="vs-sticker" data-sticker="${name}" title="${label('sticker_' + name, name)}"><span class="vs-sticker-art" aria-hidden="true">${stickerSVG(name, 56)}</span><small>${label('sticker_' + name, name)}</small></button>`).join('');
        const shortcuts = [['Space', 'shortcutPlay', 'Play / pause'], ['← →', 'shortcutFrame', 'One frame (Shift: one second)'], ['Home End', 'shortcutEnds', 'Start / end'], ['S', 'shortcutSplit', 'Split at the playhead'], ['Del', 'shortcutDelete', 'Delete the selected clip'], ['Ctrl Z', 'shortcutUndo', 'Undo (Shift: redo)'], ['+ −', 'shortcutZoom', 'Zoom the timeline'], ['Ctrl + wheel', 'shortcutWheel', 'Zoom at the pointer']];
        return `<div class="vs-app" tabindex="0">
<header class="vs-toolbar">
 <div class="vs-tb-group vs-tb-project"><div class="vs-menu-anchor"><button type="button" class="vs-project-button" data-action="project-menu" aria-haspopup="dialog" aria-expanded="false" title="${label('projects', 'Projects')}">${icon('project', 18)}<span class="vs-project-name" data-project-name>${label('noProject', 'No project')}</span>${icon('chevronDown', 14)}</button><div class="vs-popover vs-project-popover" data-project-popover role="dialog" aria-label="${label('projects', 'Projects')}" hidden></div></div><button type="button" class="vs-save-chip" data-action="save" data-save-state></button></div>
 <div class="vs-tb-group vs-tb-edit"><button type="button" class="vs-icon-button" data-action="undo" aria-label="${label('undo', 'Undo')}" title="${label('undo', 'Undo')} (Ctrl+Z)" disabled>${icon('undo', 18)}</button><button type="button" class="vs-icon-button" data-action="redo" aria-label="${label('redo', 'Redo')}" title="${label('redo', 'Redo')} (Ctrl+Shift+Z)" disabled>${icon('redo', 18)}</button><label class="vs-format" title="${label('format', 'Format')}">${icon('film', 16)}<select data-canvas aria-label="${label('format', 'Format')}"></select></label></div>
 <div class="vs-tb-group vs-tb-actions"><div class="vs-menu-anchor"><button type="button" class="vs-jobs-button" data-action="jobs" aria-haspopup="dialog" aria-expanded="false" hidden><span class="vs-jobs-icon" data-jobs-icon></span><span data-jobs-label></span></button><div class="vs-popover vs-jobs-popover" data-jobs-popover role="dialog" aria-label="${label('backgroundJobs', 'Background jobs')}" hidden><h3>${label('backgroundJobs', 'Background jobs')}</h3><section class="vs-jobs" data-jobs></section></div></div><div class="vs-menu-anchor"><button type="button" class="vs-icon-button" data-action="shortcuts" aria-haspopup="dialog" aria-expanded="false" aria-label="${label('shortcuts', 'Keyboard shortcuts')}" title="${label('shortcuts', 'Keyboard shortcuts')}">${icon('keyboard', 18)}</button><div class="vs-popover vs-shortcuts-popover" data-shortcuts-popover role="dialog" aria-label="${label('shortcuts', 'Keyboard shortcuts')}" hidden><h3>${label('shortcuts', 'Keyboard shortcuts')}</h3><dl>${shortcuts.map(([keys, key, fallback]) => `<div><dt>${keys.split(' ').map(k => `<kbd>${esc(s, k)}</kbd>`).join('')}</dt><dd>${label(key, fallback)}</dd></div>`).join('')}</dl></div></div><button type="button" class="vs-inspector-toggle" data-action="toggle-inspector" aria-label="${label('inspector', 'Inspector')}" title="${label('inspector', 'Inspector')}">${icon('sliders', 17)}<span>${label('inspector', 'Inspector')}</span></button><button type="button" class="vs-export-button vs-primary" data-action="export">${icon('export', 17)}<span>${label('export', 'Export')}</span></button></div>
</header>
<div class="vs-disabled" data-disabled hidden></div>
<div class="vs-workspace">
 <aside class="vs-library" aria-label="${label('library', 'Library')}">
  <div class="vs-tabs" role="tablist" aria-label="${label('library', 'Library')}">${tab('media', 'film', 'media', 'Media')}${tab('text', 'text', 'textTab', 'Text')}${tab('stickers', 'sticker', 'stickersTab', 'Stickers')}${tab('ai', 'sparkle', 'aiTab', 'AI')}</div>
  <div class="vs-tab-panel vs-media-panel" data-panel="media" role="tabpanel" id="vs-panel-${s.id}-media" aria-labelledby="vs-tab-${s.id}-media">
   <div class="vs-import"><button type="button" class="vs-import-main" data-action="upload">${icon('upload', 16)}<span>${label('upload', 'Upload')}</span></button><button type="button" data-action="browse">${icon('folder', 16)}<span>${label('browseShort', 'From files')}</span></button><input type="file" data-file-input accept="video/*,audio/*,image/png,image/webp" multiple hidden></div>
   <div class="vs-media-tools"><label class="vs-search">${icon('search', 15)}<input type="search" data-search placeholder="${label('searchMedia', 'Search media')}" aria-label="${label('searchMedia', 'Search media')}"></label><div class="vs-filter" role="group" aria-label="${label('filterMedia', 'Filter media')}" data-filters></div></div>
   <div class="vs-asset-grid" data-assets></div>
   <div class="vs-media-foot"><span data-asset-count></span><span>${label('dropHint', 'Drop files anywhere to import')}</span></div>
  </div>
  <div class="vs-tab-panel" data-panel="text" role="tabpanel" id="vs-panel-${s.id}-text" aria-labelledby="vs-tab-${s.id}-text" hidden><p class="vs-panel-intro">${label('textIntro', 'Text lands at the playhead on a free overlay track. Select it to change words and style.')}</p><div class="vs-preset-list">${presets}</div></div>
  <div class="vs-tab-panel" data-panel="stickers" role="tabpanel" id="vs-panel-${s.id}-stickers" aria-labelledby="vs-tab-${s.id}-stickers" hidden><p class="vs-panel-intro">${label('stickerHint', 'Add a small original graphic at the playhead.')}</p><div class="vs-sticker-grid">${stickers}</div></div>
  <div class="vs-tab-panel vs-ai-panel" data-panel="ai" role="tabpanel" id="vs-panel-${s.id}-ai" aria-labelledby="vs-tab-${s.id}-ai" hidden data-ai-panel></div>
 </aside>
 <main class="vs-center">
  <div class="vs-preview-stage"><div class="vs-preview-mat"><canvas data-preview width="1280" height="720" aria-label="${label('preview', 'Preview')}"></canvas><div class="vs-preview-empty" data-preview-empty hidden></div><div class="vs-transform" data-transform hidden><span class="vs-handle" data-handle="nw"></span><span class="vs-handle" data-handle="ne"></span><span class="vs-handle" data-handle="sw"></span><span class="vs-handle" data-handle="se"></span></div></div></div>
  <div class="vs-transport"><div class="vs-transport-buttons"><button type="button" data-action="seek-start" aria-label="${label('toStart', 'To start')}" title="${label('toStart', 'To start')} (Home)">${icon('skipStart', 17)}</button><button type="button" data-action="step-back" aria-label="${label('previousFrame', 'Previous frame')}" title="${label('previousFrame', 'Previous frame')} (←)">${icon('stepBack', 17)}</button><button type="button" class="vs-play" data-action="play" aria-label="${label('play', 'Play')}" title="${label('play', 'Play')} (Space)">${icon('play', 20)}</button><button type="button" data-action="step-forward" aria-label="${label('nextFrame', 'Next frame')}" title="${label('nextFrame', 'Next frame')} (→)">${icon('stepForward', 17)}</button><button type="button" data-action="seek-end" aria-label="${label('toEnd', 'To end')}" title="${label('toEnd', 'To end')} (End)">${icon('skipEnd', 17)}</button></div><div class="vs-time" aria-live="off"><span class="vs-time-current" data-time>0:00.00</span><span class="vs-timecode" data-transport-time>/ 0:00.00</span></div><div class="vs-transport-right"><button type="button" data-action="fullscreen" aria-label="${label('fullscreen', 'Full screen preview')}" title="${label('fullscreen', 'Full screen preview')}">${icon('fullscreen', 17)}</button></div></div>
 </main>
 <button type="button" class="vs-inspector-scrim" data-action="close-inspector" aria-label="${label('close', 'Close')}"></button>
 <aside class="vs-inspector" aria-label="${label('inspector', 'Inspector')}"><div class="vs-inspector-head"><h2>${label('properties', 'Properties')}</h2><button type="button" class="vs-inspector-close" data-action="close-inspector" aria-label="${label('close', 'Close')}" title="${label('close', 'Close')}">${icon('close', 16)}</button></div><div data-inspector class="vs-inspector-body"></div></aside>
</div>
<div class="vs-resize" data-resize role="separator" aria-orientation="horizontal" tabindex="0" aria-label="${label('resizeTimeline', 'Resize timeline')}" title="${label('resizeTimeline', 'Resize timeline')}"><span></span></div>
<section class="vs-timeline-panel" aria-label="${label('timeline', 'Timeline')}"><div data-timeline></div></section>
<div class="vs-toast" data-notice role="status" aria-live="polite" hidden></div>
<div class="vs-drop-overlay" data-drop-overlay hidden><div>${icon('upload', 34)}<strong>${label('dropToImport', 'Drop to import')}</strong><small>${label('dropFormats', 'Video, audio, PNG or WebP')}</small></div></div>
<div class="vs-modal-host" data-modal-host></div>
<div class="vs-conflict-host" data-conflict-host></div>
</div>`;
    }
    function appRender(host, id, ctx) {
        dispose(id);
        const s = {
            id, host, ctx, abort: new AbortController(), disposed: false, project: null, projectId: '', desktopPath: '', etag: '',
            status: null, projects: [], jobs: [], jobLocalStops: new Set(), frame: 0, selectedClipId: '', selectionRevision: 0,
            zoom: 1, snap: readPref(PREF_SNAP) !== 'false', dirty: false, saving: false, saveError: '', conflict: false, history: [], redo: [],
            busy: false, revision: 0, artworkRevision: 0, savePromise: null, autosaveTimer: 0, libraryTab: 'media', filter: 'all',
            uploads: new Map(), notifiedJobs: new Set(), xhrs: new Set(), textPending: false, projectEpoch: 0
        };
        instances.set(id, s);
        host.innerHTML = layoutMarkup(s);
        s.q = selector => host.querySelector(selector);
        s.app = s.q('.vs-app');
        const savedHeight = Number(readPref(PREF_TIMELINE));
        if (savedHeight >= 120) s.app.style.setProperty('--vs-timeline-h', savedHeight + 'px');
        s.notice = (key, fallback, error) => showNotice(s, key, fallback, error);
        s.media = window.VideoStudioMedia.create({ onUpdate: () => scheduleMediaRefresh(s) });
        s.timelineOptions = timelineOptions(s);
        s.detachTimeline = T().attach(s.q('[data-timeline]'), s.timelineOptions);
        s.preview = window.VideoStudioPreview.mount(s.q('[data-preview]'), () => s.project, () => s.frame, frame => {
            s.frame = frame;
            const current = I().formatTime(frame);
            if (s.q('[data-time]')) s.q('[data-time]').textContent = current;
            T().setPlayhead(s.q('[data-timeline]'), s, s.preview && s.preview.isPlaying());
            updateTransform(s);
        }, playing => {
            const button = s.q('[data-action="play"]');
            if (button) { button.innerHTML = icon(playing ? 'pause' : 'play', 20); button.setAttribute('aria-label', tr(s, playing ? 'pause' : 'play', playing ? 'Pause' : 'Play')); button.classList.toggle('is-playing', playing); }
        });
        const signal = s.abort.signal;
        host.addEventListener('click', event => {
            if (event.target.closest('.vs-preview-stage,.vs-timeline-scroll') && !event.target.closest('button,input,select,textarea')) s.app.focus({ preventScroll: true });
            closePopoversOutside(s, event.target);
            handleClick(s, event);
        }, { signal });
        host.addEventListener('change', event => handleChange(s, event), { signal });
        host.addEventListener('input', event => handleInput(s, event), { signal });
        host.addEventListener('keydown', event => handleTabKeys(s, event), { signal });
        host.addEventListener('dragstart', event => {
            const card = event.target.closest('.vs-library [data-asset-id]');
            if (!card) return;
            if (s.readonly || card.getAttribute('draggable') !== 'true') { event.preventDefault(); return; }
            event.dataTransfer.setData('application/x-video-studio-asset', card.dataset.assetId);
            event.dataTransfer.effectAllowed = 'copy';
        }, { signal });
        host.addEventListener('dragenter', event => fileDrag(s, event, true), { signal });
        host.addEventListener('dragover', event => {
            if (isFileDrag(event)) { fileDrag(s, event, true); return; }
            if (event.target.closest('.vs-preview-mat')) { event.preventDefault(); event.dataTransfer.dropEffect = 'copy'; }
        }, { signal });
        host.addEventListener('dragleave', event => { if (!host.contains(event.relatedTarget)) fileDrag(s, event, false); }, { signal });
        host.addEventListener('drop', event => {
            if (isFileDrag(event)) {
                event.preventDefault(); fileDrag(s, event, false);
                importFiles(s, Array.from(event.dataTransfer.files || []));
                return;
            }
            if (event.target.closest('.vs-preview-mat')) {
                event.preventDefault();
                const assetId = event.dataTransfer.getData('application/x-video-studio-asset');
                if (assetId) addAssetToTrack(s, assetId, '', s.frame);
            }
        }, { signal });
        s.q('[data-file-input]').addEventListener('change', async event => {
            await importFiles(s, Array.from(event.target.files || []));
            event.target.value = '';
        }, { signal });
        wireResize(s);
        wireTransform(s);
        if (typeof ResizeObserver === 'function') { s.resizeObserver = new ResizeObserver(() => { updateTransform(s); syncDrawer(s); }); s.resizeObserver.observe(s.q('.vs-preview-stage')); }
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
        }, {
            label: tr(s, 'edit', 'Edit'), items: [
                { label: tr(s, 'undo', 'Undo'), action: () => undo(s, false) },
                { label: tr(s, 'redo', 'Redo'), action: () => undo(s, true) },
                { label: tr(s, 'split', 'Split'), action: () => s.selectedClipId && timelineAction(s, 'split', s.selectedClipId) },
                { label: tr(s, 'delete', 'Delete'), action: () => s.selectedClipId && timelineAction(s, 'delete', s.selectedClipId) }
            ]
        }]);
        renderFilters(s);
        renderAIPanel(s);
        renderToolbar(s);
        initialize(s).catch(() => showNotice(s, 'loadFailed', 'Video Studio could not load.', true));
    }
    async function initialize(s) {
        const response = await request(API + '/status', { signal: s.abort.signal });
        s.status = response.body;
        if (!s.status.enabled || !s.status.desktop_enabled || !s.status.ffmpeg_ready) {
            const disabled = s.q('[data-disabled]');
            disabled.hidden = false;
            const issueKey = ({ ffmpeg_unavailable: 'ffmpegMissing', ffmpeg_missing: 'ffmpegMissing', video_studio_disabled: 'featureDisabled', desktop_disabled: 'featureDisabled', read_only: 'readonly', admin_required: 'adminRequired' })[s.status.issue];
            const issue = issueKey ? tr(s, issueKey, issueKey === 'ffmpegMissing' ? 'FFmpeg is not available on this server.' : 'Video Studio is currently unavailable.') : tr(s, 'unavailable', 'Video Studio is currently unavailable.');
            disabled.innerHTML = `<span class="vs-disabled-icon">${icon('alert', 18)}</span><div><strong>${esc(s, tr(s, 'unavailableTitle', 'Studio unavailable'))}</strong><p>${esc(s, issue)}</p></div>`;
        }
        s.featureDisabled = !s.status.enabled || !s.status.desktop_enabled;
        s.readonly = !!s.status.read_only || s.featureDisabled;
        s.app.classList.toggle('is-readonly', s.readonly);
        renderAIPanel(s);
        if (s.featureDisabled) {
            // Every project endpoint refuses while the feature is off; show the calm empty state, not a load error.
            renderEmptyProject(s);
            populateCanvas(s);
            renderToolbar(s);
            disableWriteControls(s);
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
    function disableWriteControls(s) {
        s.app.querySelectorAll(['new-project', 'save', 'export', 'upload', 'browse', 'add-media', 'add-title'].map(action => `[data-action="${action}"]`).join(',') + ',[data-sticker],[data-place-asset],[data-canvas]').forEach(control => { control.disabled = true; });
    }
    async function loadProjects(s) {
        const response = await request(API + '/projects', { signal: s.abort.signal });
        s.projects = Array.isArray(response.body.projects) ? response.body.projects : [];
        if (!s.q('[data-project-popover]').hidden) renderProjectPopover(s);
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
            ensureTracks(s);
            await loadJobs(s);
            if (s.disposed || epoch !== s.projectEpoch || id !== s.projectId) return false;
            renderUI(s);
            return true;
        } finally { s.busy = false; }
    }
    // Projects created elsewhere (API, agent) may have no tracks; give them the default set once.
    function ensureTracks(s) {
        if (!s.project || s.readonly || (s.project.tracks || []).length) return false;
        s.project.tracks = defaultTracks(s);
        scheduleSave(s);
        return true;
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
        if (!s.project.tracks || !s.project.tracks.length) s.project.tracks = defaultTracks(s);
        s.project.name = name || s.project.name;
        s.savedProject = canonicalProject(s.project);
        s.dirty = true;
        s.history = []; s.redo = []; s.selectedClipId = ''; s.frame = 0; s.jobs = [];
        await saveProject(s);
        await loadProjects(s);
        renderUI(s);
        return true;
    }
    async function createProjectPrompt(s) {
        const epoch = s.projectEpoch || 0;
        closePopovers(s);
        const name = await s.ctx.promptDialog(tr(s, 'newProject', 'New project'), tr(s, 'projectName', 'Untitled project'));
        if (s.disposed || epoch !== (s.projectEpoch || 0) || name === false || name == null || !String(name).trim()) return;
        if (s.dirty && !(await saveUntilClean(s))) return;
        try { await createProject(s, String(name).trim()); }
        catch (_) { showNotice(s, 'createFailed', 'Could not create the project.', true); }
    }
    async function switchProject(s, targetId) {
        if (!targetId || targetId === s.projectId || s.timelineDragging) return;
        closePopovers(s);
        if (s.dirty && !(await saveUntilClean(s))) return;
        try { await loadProject(s, targetId); clearNotice(s); }
        catch (_) { showNotice(s, 'loadFailed', 'Could not open this project.', true); }
    }
    function renderEmptyProject(s) {
        const stage = s.q('.vs-preview-stage');
        const mat = s.q('.vs-preview-mat');
        if (mat && !s.q('[data-no-project]')) mat.insertAdjacentHTML('beforeend', `<div class="vs-empty-project" data-no-project><div class="vs-empty-art" aria-hidden="true"><span>${icon('film', 30)}</span><span>${icon('play', 30)}</span><span>${icon('audio', 22)}</span></div><p class="vs-eyebrow">${esc(s, tr(s, 'emptyEyebrow', 'A new canvas'))}</p><h2>${esc(s, tr(s, 'emptyTitle', 'Make room for a story'))}</h2><p>${esc(s, tr(s, 'emptyCopy', 'Create a project to bring clips, sound and titles together.'))}</p><button type="button" class="vs-primary" data-action="new-project" ${s.readonly ? 'disabled' : ''}>${icon('plus', 16)}<span>${esc(s, tr(s, 'newProject', 'New project'))}</span></button></div>`);
        if (stage) stage.classList.add('vs-no-project');
        const timeline = s.q('[data-timeline]'); if (timeline) timeline.replaceChildren();
        s.app.classList.add('vs-without-project');
        s.app.querySelectorAll('[data-action="upload"],[data-action="browse"],[data-action="add-title"],[data-sticker]').forEach(control => { control.disabled = true; });
        renderAssets(s);
    }
    function populateCanvas(s) {
        const select = s.q('[data-canvas]');
        if (!select) return;
        const sizes = s.status && s.status.limits && s.status.limits.canvas_sizes || CANVASES.map(c => ({ width: c.width, height: c.height }));
        select.innerHTML = sizes.map(size => {
            const key = CANVASES.find(c => c.width === size.width && c.height === size.height)?.key || (size.width + 'x' + size.height);
            return `<option value="${Number(size.width)}x${Number(size.height)}">${esc(s, tr(s, key, `${size.width} × ${size.height}`))}</option>`;
        }).join('');
        if (s.project) select.value = `${s.project.width}x${s.project.height}`;
        if (s.readonly) select.disabled = true;
    }
    function renderToolbar(s) {
        const name = s.q('[data-project-name]');
        if (name) name.textContent = s.project ? s.project.name || tr(s, 'untitled', 'Untitled project') : tr(s, 'noProject', 'No project');
        const chip = s.q('[data-save-state]');
        if (chip) {
            chip.hidden = !s.project;
            const state = s.saving ? 'saving' : s.saveError ? 'error' : s.dirty ? 'dirty' : 'saved';
            chip.className = 'vs-save-chip is-' + state;
            const label = state === 'saving' ? tr(s, 'saving', 'Saving…') : state === 'error' ? tr(s, 'saveFailed', 'Save failed') : state === 'dirty' ? tr(s, 'unsaved', 'Unsaved changes') : tr(s, 'saved', 'All changes saved');
            chip.innerHTML = `${icon(state === 'saving' ? 'spinner' : state === 'error' ? 'alert' : state === 'dirty' ? 'edit' : 'check', 14, 'vs-icon' + (state === 'saving' ? ' vs-spin' : ''))}<span>${esc(s, label)}</span>`;
            chip.title = state === 'error' ? tr(s, 'retrySave', 'Click to try saving again') : state === 'dirty' ? tr(s, 'saveNow', 'Click to save now') : label;
            chip.disabled = s.featureDisabled || !s.project;
        }
        const undo = s.q('[data-action="undo"]'), redo = s.q('[data-action="redo"]');
        if (undo) undo.disabled = !s.history.length || s.readonly;
        if (redo) redo.disabled = !s.redo.length || s.readonly;
        const exportButton = s.q('.vs-export-button');
        if (exportButton) {
            const hasRenderableClip = !!s.project && s.project.tracks.some(track => track.clips.some(clip => s.project.assets.some(asset => asset.id === clip.asset_id)));
            exportButton.disabled = s.readonly || s.featureDisabled || !s.status || !s.status.ffmpeg_ready || !s.projectId || !hasRenderableClip;
            exportButton.title = exportButton.disabled && s.project && !hasRenderableClip ? tr(s, 'exportNeedsClip', 'Add a clip to the timeline before exporting.') : tr(s, 'export', 'Export');
        }
        renderJobsButton(s);
    }
    function renderUI(s) {
        if (s.disposed || !s.project || !s.q('[data-timeline]')) return;
        s.app.classList.toggle('is-readonly', s.readonly);
        s.app.classList.remove('vs-without-project');
        s.q('[data-canvas]').value = `${s.project.width}x${s.project.height}`;
        s.q('.vs-preview-stage').classList.remove('vs-no-project');
        const noProject = s.q('[data-no-project]'); if (noProject) noProject.remove();
        s.q('[data-preview]').width = Number(s.project.width || 1280);
        s.q('[data-preview]').height = Number(s.project.height || 720);
        s.q('.vs-preview-mat').style.aspectRatio = `${Number(s.project.width || 1280)} / ${Number(s.project.height || 720)}`;
        s.q('.vs-preview-mat').style.setProperty('--vs-ar', String(Number(s.project.width || 1280) / Number(s.project.height || 720)));
        renderPreviewEmpty(s);
        s.q('[data-canvas]').disabled = s.readonly;
        renderToolbar(s);
        renderAssets(s);
        renderInspector(s);
        syncInspectorLock(s);
        T().render(s.q('[data-timeline]'), s.timelineOptions);
        renderJobs(s);
        s.q('[data-transport-time]').textContent = '/ ' + I().formatTime(projectDuration(s));
        if (s.readonly) s.q('.vs-import').setAttribute('aria-disabled', 'true'); else s.q('.vs-import').removeAttribute('aria-disabled');
        s.app.querySelectorAll('[data-action="upload"],[data-action="browse"],[data-action="add-title"],[data-sticker]').forEach(control => { control.disabled = s.readonly; });
        if (s.readonly) {
            s.q('[data-inspector]').querySelectorAll('input,select,textarea,button').forEach(control => { control.disabled = true; });
            s.q('[data-file-input]').disabled = true;
        }
        if (s.preview) s.preview.seek(s.frame);
        updateTransform(s);
    }
    function renderInspector(s) {
        const host = s.q('[data-inspector]');
        if (!host || !s.project) return;
        const selected = T().clipFor(s.project, s.selectedClipId);
        const asset = selected ? s.project.assets.find(item => item.id === selected.clip.asset_id) || {} : null;
        let transitionInfo = null;
        if (selected) {
            const ordered = selected.track.clips.slice().sort((a, b) => a.start - b.start);
            transitionInfo = { next: ordered[ordered.findIndex(clip => clip.id === selected.clip.id) + 1] || null };
        }
        const focus = focusKey(s);
        I().render(host, {
            tr: (key, fallback) => tr(s, key, fallback), esc: value => esc(s, value), icon, project: s.project,
            selected, asset, assetName: asset ? assetDisplayName(s, asset) : '', transitionInfo, fineOpen: !!s.fineOpen, thumb: asset && asset.kind && !(selected && selected.clip.text) ? s.media.poster(asset) : '', textPending: s.textPending,
            summary: { durationFrames: projectDuration(s), clipCount: totalClipCount(s.project) }
        });
        restoreFocus(s, focus);
    }
    function syncInspectorLock(s) {
        const selected = T().clipFor(s.project, s.selectedClipId);
        if (s.readonly || selected && selected.track.locked) {
            s.q('[data-inspector]').querySelectorAll('input,select,textarea,button').forEach(control => { control.disabled = true; });
        }
    }
    function timelineOptions(s) {
        return {
            state: s, label: (key, fallback) => tr(s, key, fallback || key), esc: value => esc(s, value), icon: (name, size) => icon(name, size),
            clipMedia: (asset, clip, track, pxPerFrame, width) => clipMedia(s, asset, clip, track, pxPerFrame, width),
            assetName: asset => assetDisplayName(s, asset),
            onSelect: id => {
                if (id !== s.selectedClipId) { s.selectedClipId = id; s.selectionRevision++; }
                renderInspector(s);
                syncInspectorLock(s);
                updateTransform(s);
                if (id && s.app.getBoundingClientRect().width <= 920) setInspectorOpen(s, true, false);
            },
            onFrame: value => { s.preview.seek(value); },
            onScrubStart: () => { if (s.preview.isPlaying()) s.preview.pause(); },
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
            onReorderTrack: (from, to) => reorderTrack(s, from, to),
            onSnap: value => writePref(PREF_SNAP, value)
        };
    }
    function addAssetToTrack(s, assetId, trackId, at) {
        if (s.readonly || s.timelineDragging || !s.project) return false;
        const asset = s.project.assets.find(item => item.id === assetId);
        if (!asset) return false;
        if (!asset.kind) { showNotice(s, 'mediaPreparing', 'This media is still being prepared. Try again in a moment.', true); return false; }
        if (totalClipCount(s.project) >= MAX_CLIPS) { showNotice(s, 'clipLimit', 'This project reached its clip limit.', true); return false; }
        if (!s.project.tracks.some(item => item.clips.some(clip => clip.asset_id === asset.id)) && activeAssetCount(s.project) >= MAX_ACTIVE_ASSETS) { showNotice(s, 'assetLimit', 'This project reached its media limit.', true); return false; }
        const kind = asset.kind === 'image' ? 'overlay' : asset.kind === 'audio' ? 'audio' : 'video';
        let track = s.project.tracks.find(item => item.id === trackId);
        if (!track || track.locked || !T().accepts(track, asset)) track = s.project.tracks.find(item => item.kind === kind && !item.locked);
        if (!track && !T().trackCapacity(s.project, kind)) { showNotice(s, 'trackLimit', 'Unlock a track or remove one before placing this media.', true); return false; }
        const limit = maxFrames(s);
        const duration = Math.max(1, Math.min(Number(asset.kind === 'image' ? FPS * 5 : asset.duration_frames || FPS * 5), limit));
        const requested = track ? freeStart(track, Number.isFinite(at) ? Math.max(0, at) : nextStart(track), duration) : Math.max(0, Number(at) || 0);
        if (requested + duration > limit) { showNotice(s, 'projectLimit', 'The project reached its 10-minute limit.', true); return false; }
        let placed = null;
        const ok = mutate(s, 'add clip', project => {
            const target = track ? project.tracks.find(item => item.id === track.id) : addTrackTo(s, project, kind);
            const clip = T().makeClip(asset, requested);
            clip.duration = duration;
            if (asset.kind !== 'audio') { clip.width = 1; clip.height = 1; }
            target.clips.push(clip);
            placed = clip;
            s.selectedClipId = clip.id; s.selectionRevision++;
        });
        if (ok && placed) {
            renderInspector(s);
            T().revealFrame(s.q('[data-timeline]'), s, placed.start);
        }
        return ok;
    }
    function timelineAction(s, action, id) {
        const found = T().clipFor(s.project, id);
        if (!found || found.track.locked || s.readonly || s.timelineDragging) return;
        if (action === 'delete') {
            if (mutate(s, action, project => { const track = project.tracks.find(item => item.id === found.track.id); track.clips = track.clips.filter(clip => clip.id !== id); })) {
                s.selectedClipId = ''; renderUI(s);
            }
        } else if (action === 'duplicate') {
            if (totalClipCount(s.project) >= MAX_CLIPS) return showNotice(s, 'clipLimit', 'This project reached its clip limit.', true);
            mutate(s, action, project => {
                const track = project.tracks.find(item => item.id === found.track.id);
                const copy = clone(track.clips.find(clip => clip.id === id));
                copy.id = T().newId('clip'); copy.transition = null;
                copy.start = freeStart(track, copy.start + copy.duration, copy.duration);
                if (copy.start + copy.duration > maxFrames(s)) return false;
                track.clips.push(copy); s.selectedClipId = copy.id; s.selectionRevision++;
            });
        } else if (action === 'split') {
            const frame = s.frame, clip = found.clip;
            if (frame <= clip.start || frame >= clip.start + clip.duration) return showNotice(s, 'splitHint', 'Move the playhead inside the selected clip to split it.', true);
            mutate(s, action, project => {
                const target = project.tracks.find(item => item.id === found.track.id), original = target.clips.find(item => item.id === id);
                const firstDuration = frame - original.start;
                const second = clone(original);
                second.id = T().newId('clip'); second.start = frame; second.offset += firstDuration; second.duration -= firstDuration;
                if (original.text || (s.project.assets.find(asset => asset.id === original.asset_id) || {}).kind === 'image') second.offset = 0;
                second.transition = original.transition;
                original.fade_in = Math.min(Number(original.fade_in || 0), firstDuration); original.fade_out = 0;
                second.fade_in = 0; second.fade_out = Math.min(Number(second.fade_out || 0), second.duration);
                original.duration = firstDuration; original.transition = null; target.clips.push(second); s.selectedClipId = second.id; s.selectionRevision++;
            });
        }
    }
    function trackAction(s, id, action) {
        if (action === 'remove') {
            const track = s.project.tracks.find(item => item.id === id);
            if (!track || track.clips.length) return;
            mutate(s, 'remove track', project => { project.tracks = project.tracks.filter(item => item.id !== id); });
            return;
        }
        mutate(s, 'track', project => {
            const track = project.tracks.find(item => item.id === id);
            if (!track) return;
            if (action === 'mute') track.muted = !track.muted;
            if (action === 'hide') track.hidden = !track.hidden;
            if (action === 'lock') track.locked = !track.locked;
        });
        updateTransform(s);
    }
    function addTrack(s, kind) {
        if (!T().trackCapacity(s.project, kind)) return;
        mutate(s, 'add track', project => { addTrackTo(s, project, kind); });
    }
    function reorderTrack(s, from, to) {
        mutate(s, 'reorder tracks', project => {
            const a = project.tracks.findIndex(track => track.id === from), b = project.tracks.findIndex(track => track.id === to);
            if (a < 0 || b < 0) return false;
            const [track] = project.tracks.splice(a, 1); project.tracks.splice(b, 0, track);
        });
    }
    function handleInput(s, event) {
        const input = event.target;
        if (input.matches('[data-search]')) { renderAssets(s); return; }
        if (s.readonly || s.timelineDragging) return;
        if (input.matches('input[type="range"][data-field]')) { liveField(s, input); return; }
        if (input.matches('[data-transition-duration]')) { updateOutput(input, I().formatSeconds(Number(input.value)) + ' s'); return; }
        if (input.matches('[data-text-field="text"],[data-text-style]')) liveText(s, input);
    }
    function handleChange(s, event) {
        const input = event.target;
        if (input.matches('[data-canvas]')) {
            const [width, height] = input.value.split('x').map(Number);
            if (width && height && s.project) mutate(s, 'canvas', project => { project.width = width; project.height = height; });
            return;
        }
        if (s.readonly) return;
        if (input.matches('[data-field]')) { commitField(s, input); return; }
        if (input.matches('[data-transition-duration]')) {
            const found = selectedClip(s);
            if (found && found.clip.transition) applyTransition(s, found.clip.transition.type, Number(input.value));
            return;
        }
        if (input.matches('[data-text-field],[data-text-style]')) commitText(s, input);
    }
    function handleClick(s, event) {
        const tab = event.target.closest('[data-tab]');
        if (tab) { setTab(s, tab.dataset.tab); return; }
        const filter = event.target.closest('[data-filter]');
        if (filter) { s.filter = filter.dataset.filter; renderAssets(s); return; }
        const projectItem = event.target.closest('[data-project-id]');
        if (projectItem) { switchProject(s, projectItem.dataset.projectId); return; }
        const place = event.target.closest('[data-place-asset]');
        if (place) { if (!place.disabled) addAssetToTrack(s, place.dataset.placeAsset, '', s.frame); return; }
        const cancel = event.target.closest('[data-cancel-job]'); if (cancel) { cancelJob(s, cancel.dataset.cancelJob); return; }
        const stopWatch = event.target.closest('[data-stop-watch]'); if (stopWatch) { s.jobLocalStops.add(stopWatch.dataset.stopWatch); renderJobs(s); return; }
        if (event.target.closest('[data-timeline]')) return;
        const sticker = event.target.closest('[data-sticker]');
        if (sticker) { if (s.readonly) showNotice(s, 'readonly', 'Read-only mode prevents editing.', true); else addSticker(s, sticker.dataset.sticker); return; }
        const position = event.target.closest('[data-position]');
        if (position) { mutateSelected(s, 'position', clip => { Object.assign(clip, I().presetBox(position.dataset.position, clip)); }); return; }
        const fit = event.target.closest('[data-fit]');
        if (fit) { mutateSelected(s, 'fit', clip => { clip.fit = fit.dataset.fit === 'cover' ? 'cover' : 'contain'; }); return; }
        const transition = event.target.closest('[data-transition]');
        if (transition) { const range = s.q('[data-transition-duration]'); applyTransition(s, transition.dataset.transition, range && !range.disabled ? Number(range.value) : 15); return; }
        const align = event.target.closest('[data-text-align]');
        if (align) { if (mutateSelected(s, 'text align', clip => { clip.text_style = Object.assign({}, clip.text_style || {}, { alignment: align.dataset.textAlign }); })) scheduleTextApply(s); return; }
        const toggle = event.target.closest('[data-text-toggle]');
        if (toggle) { const key = toggle.dataset.textToggle; if (mutateSelected(s, 'text style', clip => { clip.text_style = Object.assign({}, clip.text_style || {}, { [key]: !(clip.text_style && clip.text_style[key]) }); })) scheduleTextApply(s); return; }
        const button = event.target.closest('[data-action]'); if (!button || button.disabled) return;
        const action = button.dataset.action;
        if (s.readonly && !READ_ONLY_SAFE.has(action)) return showNotice(s, 'readonly', 'Read-only mode prevents editing.', true);
        if (action === 'new-project') createProjectPrompt(s);
        else if (action === 'project-menu') togglePopover(s, 'project');
        else if (action === 'jobs') togglePopover(s, 'jobs');
        else if (action === 'shortcuts') togglePopover(s, 'shortcuts');
        else if (action === 'rename-project') renameProject(s);
        else if (action === 'dismiss-notice') clearNotice(s);
        else if (action === 'toggle-inspector') setInspectorOpen(s, !s.app.classList.contains('vs-show-inspector'), true);
        else if (action === 'close-inspector') { setInspectorOpen(s, false, false); s.q('.vs-inspector-toggle')?.focus(); }
        else if (action === 'upload' || action === 'add-media') s.q('[data-file-input]').click();
        else if (action === 'browse') importDesktopFile(s);
        else if (action === 'save') saveProject(s);
        else if (action === 'undo') undo(s, false);
        else if (action === 'redo') undo(s, true);
        else if (action === 'play') s.preview.isPlaying() ? s.preview.pause() : s.preview.play();
        else if (action === 'step-back') s.preview.pause(), s.preview.seek(s.frame - 1);
        else if (action === 'step-forward') s.preview.pause(), s.preview.seek(s.frame + 1);
        else if (action === 'seek-start') s.preview.pause(), s.preview.seek(0), T().revealFrame(s.q('[data-timeline]'), s, 0);
        else if (action === 'seek-end') { const end = projectDuration(s); s.preview.pause(); s.preview.seek(end); T().revealFrame(s.q('[data-timeline]'), s, end); }
        else if (action === 'fullscreen') { const mat = s.q('.vs-preview-mat'); if (document.fullscreenElement) document.exitFullscreen().catch(() => {}); else if (mat && mat.requestFullscreen) mat.requestFullscreen().catch(() => {}); }
        else if (action === 'export') openExport(s);
        else if (action === 'add-title') addTitle(s, button.dataset.preset || 'title');
        else if (action === 'apply-text') { clearTimeout(s.textTimer); applyTextArtwork(s); }
        else if (action === 'open-ai') openAI(s);
        else if (action === 'toggle-fine') { s.fineOpen = !s.fineOpen; renderInspector(s); syncInspectorLock(s); s.q('[data-action="toggle-fine"]')?.focus(); }
        else if (action === 'fill-frame') mutateSelected(s, 'fill', clip => { Object.assign(clip, { x: 0, y: 0, width: 1, height: 1 }); });
        else if (action === 'clip-split' || action === 'clip-duplicate' || action === 'clip-delete') { if (s.selectedClipId) timelineAction(s, action.slice(5), s.selectedClipId); }
    }
    function handleKeys(s, event) {
        const target = event.target, modifier = event.ctrlKey || event.metaKey, undoShortcut = modifier && !event.altKey && event.key.toLowerCase() === 'z';
        if (s.disposed || event.defaultPrevented || !s.project || !target || !s.host.contains(target) || typeof target.closest !== 'function') return;
        if ((event.ctrlKey || event.metaKey || event.altKey) && !undoShortcut) return;
        if (event.key === 'Escape') {
            const modal = s.q('[data-modal-host] .vs-modal');
            if (modal) { s.q('[data-modal-host]').replaceChildren(); s.app.focus({ preventScroll: true }); event.preventDefault(); return; }
            if (Object.values(POPOVERS).some(([, selector]) => !s.q(selector).hidden)) { closePopovers(s); event.preventDefault(); return; }
        }
        if (event.key === 'Escape' && s.app.classList.contains('vs-show-inspector')) { setInspectorOpen(s, false, false); s.q('.vs-inspector-toggle')?.focus(); event.preventDefault(); return; }
        if (event.key === 'Tab' && s.app.classList.contains('vs-show-inspector') && s.app.getBoundingClientRect().width <= 920) {
            const items = Array.from(s.q('.vs-inspector').querySelectorAll('button:not(:disabled),input:not(:disabled),select:not(:disabled),textarea:not(:disabled)')).filter(item => item.getClientRects().length);
            if (items.length) { const first = items[0], last = items[items.length - 1]; if (event.shiftKey && (document.activeElement === first || !s.q('.vs-inspector').contains(document.activeElement))) { event.preventDefault(); last.focus(); } else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); } }
        }
        if (target.closest('input,textarea,select,[contenteditable="true"]')) return;
        if (undoShortcut) { event.preventDefault(); undo(s, event.shiftKey); return; }
        if (target.closest('button,a,summary,[role="button"],[role="link"],[role="tab"],[role="separator"]')) return;
        if (s.q('[data-modal-host] .vs-modal, [data-conflict-host] .vs-modal')) return;
        if (event.code === 'Space') { event.preventDefault(); s.preview.isPlaying() ? s.preview.pause() : s.preview.play(); }
        else if (event.key === 'Delete' || event.key === 'Backspace') { if (s.selectedClipId) { event.preventDefault(); timelineAction(s, 'delete', s.selectedClipId); } }
        else if (event.key === 'ArrowLeft') { event.preventDefault(); s.preview.pause(); s.preview.seek(s.frame - (event.shiftKey ? FPS : 1)); }
        else if (event.key === 'ArrowRight') { event.preventDefault(); s.preview.pause(); s.preview.seek(s.frame + (event.shiftKey ? FPS : 1)); }
        else if (event.key === 'Home') { event.preventDefault(); s.preview.pause(); s.preview.seek(0); T().revealFrame(s.q('[data-timeline]'), s, 0); }
        else if (event.key === 'End') { event.preventDefault(); const end = projectDuration(s); s.preview.pause(); s.preview.seek(end); T().revealFrame(s.q('[data-timeline]'), s, end); }
        else if (event.key === '+' || event.key === '=') { event.preventDefault(); s.q('[data-timeline] [data-action="zoom-in"]')?.click(); }
        else if (event.key === '-') { event.preventDefault(); s.q('[data-timeline] [data-action="zoom-out"]')?.click(); }
        else if (event.key === '?') { event.preventDefault(); togglePopover(s, 'shortcuts'); }
        else if (event.key.toLowerCase() === 's' && s.selectedClipId) { event.preventDefault(); timelineAction(s, 'split', s.selectedClipId); }
    }

    Object.assign(P, { dispose, layoutMarkup, appRender, initialize, disableWriteControls, loadProjects, loadProject, ensureTracks, createProject, createProjectPrompt, switchProject, renderEmptyProject, populateCanvas, renderToolbar, renderUI, renderInspector, syncInspectorLock, timelineOptions, addAssetToTrack, timelineAction, trackAction, addTrack, reorderTrack, handleInput, handleChange, handleClick, handleKeys });

    window.VideoStudioApp = { render: appRender, dispose, instances };
})();
