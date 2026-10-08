(function () {
    'use strict';

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
    const STICKERS = ['sparkle', 'heart', 'sun', 'star', 'flower', 'burst'];
    const TITLE_PRESETS = {
        title: { textKey: 'defaultTitle', text: 'Your title', seconds: 4, anchor: 'mc', style: { font_family: 'Georgia', font_size: 104, color: '#ffffff', bold: true, italic: false, alignment: 'center', background_color: '#101319', background_opacity: 0 } },
        subtitle: { textKey: 'defaultSubtitle', text: 'Your subtitle', seconds: 4, anchor: 'bc', style: { font_family: 'Arial', font_size: 50, color: '#ffffff', bold: false, italic: false, alignment: 'center', background_color: '#101319', background_opacity: 0.55 } },
        lower: { textKey: 'defaultLowerThird', text: 'Name Surname\nRole', seconds: 5, anchor: 'bl', style: { font_family: 'Arial', font_size: 44, color: '#ffffff', bold: true, italic: false, alignment: 'left', background_color: '#101319', background_opacity: 0.72 } },
        credits: { textKey: 'defaultCredits', text: 'Directed by\nYour name\n\nThanks for watching', seconds: 6, anchor: 'mc', style: { font_family: 'Georgia', font_size: 52, color: '#ffffff', bold: false, italic: false, alignment: 'center', background_color: '#101319', background_opacity: 0 } }
    };
    // Actions that never change the project and stay available in read-only mode.
    const READ_ONLY_SAFE = new Set(['play', 'step-back', 'step-forward', 'seek-start', 'seek-end', 'toggle-inspector', 'close-inspector', 'project-menu', 'jobs', 'shortcuts', 'dismiss-notice', 'fullscreen', 'filter', 'toggle-fine']);
    const TERMINAL = ['succeeded', 'failed', 'cancelled', 'interrupted'];
    const PREF_TIMELINE = 'aurago.videoStudio.timelineHeight';
    const PREF_SNAP = 'aurago.videoStudio.snap';
    const instances = new Map();
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
    // Multipart uploads go through XHR so the media bin can show upload progress.
    function upload(s, url, form, onProgress) {
        return new Promise((resolve, reject) => {
            const xhr = new XMLHttpRequest();
            s.xhrs.add(xhr);
            xhr.open('POST', url);
            xhr.withCredentials = true;
            xhr.setRequestHeader('Idempotency-Key', idempotencyKey());
            xhr.responseType = 'text';
            xhr.upload.onprogress = event => { if (event.lengthComputable && onProgress) onProgress(event.loaded / event.total); };
            const finish = () => s.xhrs.delete(xhr);
            xhr.onerror = () => { finish(); reject(Object.assign(new Error('request_failed'), { status: 0 })); };
            xhr.onabort = () => { finish(); reject(Object.assign(new Error('aborted'), { status: 0 })); };
            xhr.onload = () => {
                finish();
                let body = {};
                try { body = JSON.parse(xhr.responseText || '{}'); } catch (_) { body = {}; }
                if (xhr.status >= 200 && xhr.status < 300) resolve({ body, etag: xhr.getResponseHeader('ETag') || '' });
                else reject(Object.assign(new Error(body.code || body.error || 'request_failed'), { status: xhr.status, body }));
            };
            xhr.send(form);
        });
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

    function showNotice(s, key, fallback, error, action) {
        if (s.disposed || !s.q('[data-notice]')) return;
        const el = s.q('[data-notice]');
        el.hidden = false;
        el.classList.toggle('is-error', !!error);
        el.innerHTML = `<span class="vs-toast-icon">${icon(error ? 'alert' : 'check', 17)}</span><span class="vs-toast-text" data-notice-text></span>${action ? `<a class="vs-toast-action" href="${esc(s, action.href)}" download="${esc(s, action.download || '')}">${icon('download', 15)}${esc(s, action.label)}</a>` : ''}<button type="button" class="vs-toast-close" data-action="dismiss-notice" aria-label="${esc(s, tr(s, 'close', 'Close'))}">${icon('close', 15)}</button>`;
        el.querySelector('[data-notice-text]').textContent = tr(s, key, fallback);
        clearTimeout(s.toastTimer);
        if (!error) s.toastTimer = window.setTimeout(() => clearNotice(s), action ? 15000 : 5000);
    }
    function clearNotice(s) { const el = s.q('[data-notice]'); if (el) { el.hidden = true; el.replaceChildren(); } clearTimeout(s.toastTimer); }

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
    function projectDuration(s) { return s.project ? T().projectEnd(s.project) : 0; }
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
    function renderPreviewEmpty(s) {
        const el = s.q('[data-preview-empty]');
        if (!el || !s.project) return;
        const hasClips = s.project.tracks.some(track => track.clips.length);
        el.hidden = hasClips;
        if (hasClips) return;
        const hasMedia = s.project.assets.some(asset => !window.VideoStudioMedia.isArtwork(asset));
        el.innerHTML = `<span class="vs-preview-empty-icon">${icon(hasMedia ? 'layers' : 'upload', 26)}</span><strong>${esc(s, hasMedia ? tr(s, 'previewEmpty', 'Your edit takes shape here') : tr(s, 'previewImportTitle', 'Start with your media'))}</strong><small>${esc(s, hasMedia ? tr(s, 'previewPlaceHint', 'Drag media onto the timeline or press + on a clip in the library.') : tr(s, 'previewImportHint', 'Drop videos, music or pictures here, or upload them from the library.'))}</small>`;
    }

    function assetKindLabel(s, asset) {
        if (!asset.kind) return tr(s, 'preparing', 'Preparing…');
        return tr(s, asset.kind === 'video' ? 'assetVideo' : asset.kind === 'audio' ? 'assetAudio' : 'assetImage', asset.kind === 'video' ? 'Video' : asset.kind === 'audio' ? 'Audio' : 'Image');
    }
    function renderFilters(s) {
        const host = s.q('[data-filters]');
        if (!host) return;
        const assets = s.project ? s.project.assets.filter(asset => !window.VideoStudioMedia.isArtwork(asset)) : [];
        const count = kind => kind === 'all' ? assets.length : assets.filter(asset => asset.kind === kind).length;
        host.innerHTML = [['all', 'filterAll', 'All'], ['video', 'assetVideo', 'Video'], ['audio', 'assetAudio', 'Audio'], ['image', 'filterImages', 'Images']].map(([kind, key, fallback]) => `<button type="button" data-filter="${kind}" aria-pressed="${s.filter === kind}">${esc(s, tr(s, key, fallback))}<span>${count(kind)}</span></button>`).join('');
    }
    function renderAssets(s) {
        const root = s.q('[data-assets]');
        if (!root) return;
        renderFilters(s);
        if (!s.project) { root.innerHTML = `<div class="vs-empty-media">${icon('project', 28)}<strong>${esc(s, tr(s, 'noProject', 'No project'))}</strong><small>${esc(s, tr(s, 'createProjectFirst', 'Create a project before importing media.'))}</small></div>`; s.q('[data-asset-count]').textContent = ''; return; }
        const activeAssets = new Set(s.project.tracks.flatMap(track => track.clips.map(clip => clip.asset_id)));
        const search = (s.q('[data-search]')?.value || '').toLowerCase();
        const visible = s.project.assets.filter(asset => !window.VideoStudioMedia.isArtwork(asset) && (s.filter === 'all' || asset.kind === s.filter) && (!search || String(asset.name).toLowerCase().includes(search)));
        const uploads = Array.from(s.uploads.values()).map(item => `<article class="vs-asset is-uploading${item.failed ? ' is-failed' : ''}"><span class="vs-asset-thumb">${icon(item.failed ? 'alert' : 'upload', 22)}<span class="vs-asset-progress"><i style="width:${Math.round(item.progress * 100)}%"></i></span></span><span class="vs-asset-name" title="${esc(s, item.name)}">${esc(s, item.name)}</span><span class="vs-asset-meta">${esc(s, item.failed ? tr(s, 'importFailedShort', 'Import failed') : item.progress < 1 ? tr(s, 'uploading', 'Uploading…') + ' ' + Math.round(item.progress * 100) + ' %' : tr(s, 'preparing', 'Preparing…'))}</span></article>`).join('');
        root.innerHTML = uploads + visible.map(asset => {
            const used = activeAssets.has(asset.id);
            const ready = !!asset.kind;
            const poster = ready ? s.media.poster(asset) : '';
            const kindIcon = asset.kind === 'video' ? 'video' : asset.kind === 'audio' ? 'audio' : asset.kind === 'image' ? 'image' : 'spinner';
            const duration = asset.kind === 'video' || asset.kind === 'audio' ? T().shortClock(asset.duration_frames || 0) : '';
            const style = poster ? ` style="background-image:url('${esc(s, poster)}')"` : '';
            return `<article class="vs-asset vs-asset-${esc(s, asset.kind || 'pending')}${used ? ' is-used' : ''}" draggable="${!s.readonly && ready}" data-asset-id="${esc(s, asset.id)}" tabindex="0" title="${esc(s, asset.name)}"><span class="vs-asset-thumb"${style}>${poster ? '' : icon(kindIcon, 22, 'vs-icon' + (ready ? '' : ' vs-spin'))}${duration ? `<span class="vs-asset-badge">${esc(s, duration)}</span>` : ''}${used ? `<span class="vs-asset-used" title="${esc(s, tr(s, 'inTimeline', 'In the timeline'))}">${icon('check', 11)}</span>` : ''}</span><span class="vs-asset-name">${esc(s, asset.name)}</span><span class="vs-asset-meta">${icon(kindIcon === 'spinner' ? 'clock' : kindIcon, 12)}${esc(s, assetKindLabel(s, asset))}</span><button type="button" class="vs-asset-add" data-place-asset="${esc(s, asset.id)}" title="${esc(s, tr(s, 'addToTimeline', 'Add at the playhead'))}" aria-label="${esc(s, tr(s, 'addToTimeline', 'Add at the playhead'))}: ${esc(s, asset.name)}" ${s.readonly || !ready ? 'disabled' : ''}>${icon('plus', 16)}</button></article>`;
        }).join('');
        if (!uploads && !visible.length) {
            const searching = search || s.filter !== 'all';
            root.innerHTML = `<div class="vs-empty-media">${icon(searching ? 'search' : 'upload', 28)}<strong>${esc(s, searching ? tr(s, 'noMatches', 'Nothing matches') : tr(s, 'emptyMedia', 'Your media will appear here'))}</strong><small>${esc(s, searching ? tr(s, 'noMatchesHint', 'Try another search or filter.') : tr(s, 'emptyMediaDrop', 'Upload clips, music or pictures, or drop files into this window.'))}</small></div>`;
        }
        const total = s.project.assets.filter(asset => !window.VideoStudioMedia.isArtwork(asset)).length;
        s.q('[data-asset-count]').textContent = tr(s, 'assetCount', '{{count}} items').replace('{{count}}', String(total));
    }
    function scheduleMediaRefresh(s) {
        if (s.mediaFrame || s.disposed) return;
        s.mediaFrame = requestAnimationFrame(() => {
            s.mediaFrame = 0;
            if (s.disposed || !s.project) return;
            renderAssets(s);
            if (!s.timelineDragging && !s.transformDrag) T().render(s.q('[data-timeline]'), s.timelineOptions);
            const thumb = s.q('.vs-insp-thumb'), selected = selectedClip(s);
            const asset = selected && s.project.assets.find(item => item.id === selected.clip.asset_id);
            if (thumb && !thumb.style.backgroundImage && asset && asset.kind === 'video' && !s.q('[data-inspector]').contains(document.activeElement)) renderInspector(s);
        });
    }
    function clipMedia(s, asset, clip, track, pxPerFrame, width) {
        if (clip.text || !asset.kind) return '';
        if (track.kind === 'audio' || asset.kind === 'audio') {
            const data = s.media.peaks(asset);
            if (!data || !data.peaks.length) return '';
            const points = Math.max(2, Math.min(600, Math.round(width / 2)));
            const key = [asset.id, clip.offset, clip.duration, points, clip.volume].join('|');
            s.waveCache = s.waveCache || new Map();
            let path = s.waveCache.get(key);
            if (!path) {
                const top = [], bottom = [];
                for (let k = 0; k < points; k++) {
                    const frame = clip.offset + k / (points - 1) * clip.duration;
                    const value = Math.min(1, (data.peaks[Math.min(data.peaks.length - 1, Math.floor(frame * data.perFrame))] || 0) * Math.max(0.15, Math.min(1.6, Number(clip.volume == null ? 1 : clip.volume))));
                    top.push(`${k},${(50 - value * 46).toFixed(1)}`);
                    bottom.unshift(`${k},${(50 + value * 46).toFixed(1)}`);
                }
                path = 'M' + top.join('L') + 'L' + bottom.join('L') + 'Z';
                if (s.waveCache.size > 400) s.waveCache.clear();
                s.waveCache.set(key, path);
            }
            return `<svg class="vs-wave" viewBox="0 0 ${points - 1} 100" preserveAspectRatio="none" aria-hidden="true"><path d="${path}"/></svg>`;
        }
        const thumbs = s.media.thumbs(asset);
        if (!thumbs.length) return '';
        const ratio = clamp((Number(asset.width) || 16) / (Number(asset.height) || 9), 0.45, 2.4);
        const tile = Math.max(24, Math.round(40 * ratio));
        const count = Math.min(48, Math.ceil(width / tile));
        let tiles = '';
        for (let i = 0; i < count; i++) {
            const thumb = window.VideoStudioMedia.pickThumb(thumbs, clip.offset + (i * tile + tile / 2) / pxPerFrame);
            if (thumb) tiles += `<i style="left:${i * tile}px;width:${tile}px;background-image:url('${esc(s, thumb.url)}')"></i>`;
        }
        return `<span class="vs-filmstrip">${tiles}</span>`;
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
            selected, asset, transitionInfo, fineOpen: !!s.fineOpen, thumb: asset && asset.kind && !(selected && selected.clip.text) ? s.media.poster(asset) : '', textPending: s.textPending,
            summary: { durationFrames: projectDuration(s), clipCount: totalClipCount(s.project) }
        });
        restoreFocus(s, focus);
    }
    // Inspector re-renders replace its controls; keep keyboard focus on the equivalent control.
    function focusKey(s) {
        const active = document.activeElement;
        if (!active || !s.q('[data-inspector]')?.contains(active)) return null;
        for (const attr of ['field', 'textField', 'textStyle', 'position', 'fit', 'transition', 'transitionDuration', 'textAlign', 'textToggle', 'action']) {
            if (active.dataset && active.dataset[attr] != null) return { attr: attr.replace(/[A-Z]/g, c => '-' + c.toLowerCase()), value: active.dataset[attr] };
        }
        return null;
    }
    function restoreFocus(s, key) {
        if (!key) return;
        const el = s.q('[data-inspector]').querySelector(`[data-${key.attr}${key.value ? `="${CSS.escape(key.value)}"` : ''}]`);
        if (el && !el.disabled) el.focus({ preventScroll: true });
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
    function snapshot(s) { return clone(s.project); }
    function recordChange(s, label, before) {
        if (s.readonly) { if (before) s.project = hydrateProject(s, before); return renderUI(s); }
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
        const before = snapshot(s);
        if (fn(s.project) === false) { s.project = hydrateProject(s, before); return false; }
        if (T().validTimeline && !T().validTimeline(s.project)) {
            s.project = hydrateProject(s, before);
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
        const source = redo ? s.redo : s.history;
        const target = redo ? s.history : s.redo;
        if (!source.length) return;
        s.artworkRevision++; target.push(snapshot(s));
        s.project = hydrateProject(s, source.pop());
        if (s.selectedClipId && !T().clipFor(s.project, s.selectedClipId)) s.selectedClipId = '';
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

    async function loadJobs(s) {
        if (!s.projectId || s.disposed) return;
        const projectId = s.projectId, epoch = s.projectEpoch;
        const response = await request(API + '/jobs?project_id=' + encodeURIComponent(projectId), { signal: s.abort.signal });
        if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
        const previous = new Map(s.jobs.map(job => [job.id, job.status]));
        s.jobs = Array.isArray(response.body.jobs) ? response.body.jobs : [];
        s.jobs.forEach(job => {
            if (TERMINAL.includes(job.status) && !s.finishedJobs?.has(job.id)) {
                if (!s.finishedJobs) s.finishedJobs = new Set();
                s.finishedJobs.add(job.id);
                if (job.status === 'succeeded') refreshProject(s).catch(() => {});
                const was = previous.get(job.id);
                if (was && !TERMINAL.includes(was)) announceJob(s, job);
            }
        });
        renderJobs(s);
    }
    function jobFailure(job) {
        if (job.external_status_unknown && ['failed', 'cancelled', 'interrupted'].includes(job.status)) return ['generationStatusUnknown', 'The provider status is unclear; it may still be processing. Check its status before making another paid request.'];
        return ({ project_size_limit: ['projectStorageFull', 'This project has reached its storage limit. Remove media or use another project.'], asset_size_limit: ['fileTooLarge', 'The file is larger than the server limit.'], generation_import_failed: ['generatedImportFailed', 'The generated clip could not be added to this project.'] })[job.error] || ['jobFailed', 'This task could not be completed.'];
    }
    function exportName(s) { return String(s.project && s.project.name || tr(s, 'untitled', 'Untitled project')).replace(/[\\/:*?"<>|]+/g, ' ').trim().slice(0, 80) + '.mp4'; }
    // Finished render and generation jobs get one toast; probes are quiet unless they fail.
    function announceJob(s, job) {
        if (s.notifiedJobs.has(job.id) || s.disposed) return;
        s.notifiedJobs.add(job.id);
        const artifact = job.artifact && safeSameOrigin(job.artifact.download_url);
        if (job.kind === 'render' && job.status === 'succeeded' && artifact) showNotice(s, 'exportReady', 'Your video is ready.', false, { href: artifact, download: exportName(s), label: tr(s, 'download', 'Download') });
        else if (job.kind === 'generate' && job.status === 'succeeded') showNotice(s, 'generationReady', 'The generated clip is in your media library.', false);
    }
    function visibleJobs(s) {
        return s.jobs.filter(job => !s.jobLocalStops.has(job.id) && !(job.kind === 'probe' && job.status === 'succeeded'));
    }
    function renderJobsButton(s) {
        const button = s.q('[data-action="jobs"]');
        if (!button) return;
        const jobs = visibleJobs(s);
        const active = jobs.filter(job => !TERMINAL.includes(job.status));
        const failed = jobs.filter(job => job.status === 'failed');
        button.hidden = !jobs.length && s.q('[data-jobs-popover]').hidden;
        button.classList.toggle('is-busy', !!active.length);
        button.classList.toggle('has-failure', !active.length && !!failed.length);
        const render = active.find(job => job.kind === 'render');
        s.q('[data-jobs-icon]').innerHTML = icon(active.length ? 'spinner' : failed.length ? 'alert' : 'check', 16, 'vs-icon' + (active.length ? ' vs-spin' : ''));
        s.q('[data-jobs-label]').textContent = render ? tr(s, 'exportingPercent', 'Exporting {{percent}} %').replace('{{percent}}', String(Math.round(clamp(Number(render.progress || 0), 0, 1) * 100))) : active.length ? tr(s, 'jobsRunning', '{{count}} running').replace('{{count}}', String(active.length)) : tr(s, 'jobsDone', 'Tasks');
    }
    function renderJobs(s) {
        const host = s.q('[data-jobs]');
        if (!host) return;
        const visible = visibleJobs(s);
        host.innerHTML = visible.length ? visible.map(job => {
            const percent = Math.round(clamp(Number(job.progress || 0), 0, 1) * 100);
            const terminal = TERMINAL.includes(job.status);
            const failure = job.status === 'failed' || (job.external_status_unknown && ['cancelled', 'interrupted'].includes(job.status)) ? jobFailure(job) : null;
            const artifact = job.artifact && safeSameOrigin(job.artifact.download_url);
            const kindIcon = job.kind === 'render' ? 'export' : job.kind === 'generate' ? 'sparkle' : 'film';
            return `<article class="vs-job vs-job-${esc(s, job.status)}"><div class="vs-job-heading">${icon(kindIcon, 15)}<strong>${esc(s, tr(s, 'job_' + job.kind, job.kind))}</strong><span class="vs-job-state">${esc(s, tr(s, 'job_' + job.status, job.status))}</span></div>${terminal ? '' : `<progress max="100" value="${percent}" aria-label="${esc(s, tr(s, 'job_' + job.kind, job.kind))}"></progress>`}<div class="vs-job-footer"><small>${terminal ? '' : percent + ' %'}</small><span>${artifact ? `<a href="${esc(s, artifact)}" download="${esc(s, job.kind === 'render' ? exportName(s) : '')}">${icon('download', 14)} ${esc(s, tr(s, 'download', 'Download'))}</a>` : ''}${!terminal && job.kind !== 'generate' ? `<button type="button" data-cancel-job="${esc(s, job.id)}">${esc(s, tr(s, 'cancelJob', 'Cancel'))}</button>` : ''}${!terminal && job.kind === 'generate' ? `<button type="button" data-stop-watch="${esc(s, job.id)}">${esc(s, tr(s, 'stopTracking', 'Stop tracking'))}</button>` : ''}</span></div>${failure ? '<small class="vs-job-error">' + esc(s, tr(s, failure[0], failure[1])) + '</small>' : ''}</article>`;
        }).join('') : `<p class="vs-hint">${esc(s, tr(s, 'noJobs', 'No tasks right now.'))}</p>`;
        renderJobsButton(s);
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
            if (TERMINAL.includes(job.status)) {
                if (!s.finishedJobs) s.finishedJobs = new Set();
                s.finishedJobs.add(job.id);
                if (job.status === 'succeeded') await refreshProject(s);
                if (job.status === 'failed' || job.external_status_unknown && ['cancelled', 'interrupted'].includes(job.status)) { const failure = jobFailure(job); showNotice(s, failure[0], failure[1], true); }
                else announceJob(s, job);
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
            const known = new Map((s.project.assets || []).map(asset => [asset.id, asset]));
            // Assets that finished probing replace their pending entry; new ones are appended.
            next.assets.forEach(asset => { const local = known.get(asset.id); if (local && !local.kind && asset.kind) Object.assign(local, asset); });
            const additions = next.assets.filter(asset => !known.has(asset.id));
            s.project.assets = (s.project.assets || []).concat(additions);
            s.savedProject.assets = next.assets.map(({ media_url, ...asset }) => asset);
        } else {
            s.project = next;
            s.savedProject = canonicalProject(next);
            if (s.selectedClipId && !T().clipFor(s.project, s.selectedClipId)) s.selectedClipId = '';
        }
        s.etag = response.etag || s.etag;
        s.desktopPath = response.body.desktop_path || s.desktopPath;
        s.revision++;
        renderUI(s);
    }

    function isFileDrag(event) { return Array.from(event.dataTransfer && event.dataTransfer.types || []).includes('Files'); }
    function fileDrag(s, event, active) {
        if (active && !isFileDrag(event)) return;
        const overlay = s.q('[data-drop-overlay]');
        if (!overlay) return;
        if (active) {
            event.preventDefault();
            event.dataTransfer.dropEffect = s.readonly || !s.projectId ? 'none' : 'copy';
            overlay.hidden = s.readonly || !s.projectId;
        } else overlay.hidden = true;
    }
    async function importFiles(s, files) {
        const projectId = s.projectId, epoch = s.projectEpoch;
        if (!files.length) return;
        if (s.readonly || !projectId) { showNotice(s, s.readonly ? 'readonly' : 'createProjectFirst', s.readonly ? 'Read-only mode prevents editing.' : 'Create a project before importing media.', true); return; }
        setTab(s, 'media');
        for (const file of files) {
            if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) break;
            await uploadFile(s, file, projectId, epoch);
        }
    }
    async function uploadFile(s, file, expectedProjectId, expectedEpoch) {
        const projectId = expectedProjectId || s.projectId, epoch = expectedEpoch == null ? s.projectEpoch : expectedEpoch;
        if (!file || s.readonly || !projectId || epoch !== s.projectEpoch || projectId !== s.projectId) return;
        const allowed = /^(video\/|audio\/|image\/(png|webp)$)/i.test(file.type) || /\.(mp4|mov|webm|m4v|mkv|mp3|wav|ogg|m4a|aac|flac|png|webp)$/i.test(file.name);
        if (!allowed) return showNotice(s, 'unsupportedFile', 'Choose a video, audio, PNG or WebP file.', true);
        const max = Number(s.status.limits && s.status.limits.max_asset_size_bytes || 256 * 1024 * 1024);
        if (file.size > max) return showNotice(s, 'fileTooLarge', 'This file is larger than the server limit.', true);
        const key = idempotencyKey(), entry = { name: file.name, progress: 0, failed: false };
        s.uploads.set(key, entry);
        renderAssets(s);
        const form = new FormData(); form.append('file', file, file.name);
        try {
            const response = await upload(s, API + '/projects/' + encodeURIComponent(projectId) + '/media', form, progress => { entry.progress = progress; if (!s.disposed) renderAssets(s); });
            entry.progress = 1;
            if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
            renderAssets(s);
            const job = response.body.job;
            if (job) { s.jobs.unshift(job); renderJobs(s); await pollJob(s, job.id, projectId, epoch); }
            else await refreshProject(s);
            if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
            s.uploads.delete(key);
            renderAssets(s);
            showNotice(s, 'importReady', 'Media added. Drag it to the timeline or press +.', false);
        } catch (error) {
            s.uploads.delete(key);
            if (!s.disposed && epoch === s.projectEpoch && projectId === s.projectId) {
                renderAssets(s);
                const code = error.body && error.body.code;
                showNotice(s, code === 'asset_size_limit' ? 'fileTooLarge' : code === 'project_size_limit' ? 'projectStorageFull' : 'importFailed', 'Could not import this media.', true);
            }
        }
    }

    async function importDesktopFile(s) {
        if (s.readonly || !s.projectId || !s.ctx.openFileDialog) return;
        const projectId = s.projectId, epoch = s.projectEpoch;
        const result = await s.ctx.openFileDialog({
            title: tr(s, 'browse', 'Browse Desktop files'), initialPath: 'Documents',
            filters: [{ label: tr(s, 'videoAudioImages', 'Video, audio and images'), extensions: ['.mp4', '.mov', '.webm', '.m4v', '.mkv', '.mp3', '.wav', '.ogg', '.m4a', '.aac', '.flac', '.png', '.webp'] }]
        });
        if (!result || result.canceled || !result.path || s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
        const key = idempotencyKey(), entry = { name: String(result.path).split('/').pop(), progress: 1, failed: false };
        s.uploads.set(key, entry); renderAssets(s);
        try {
            const response = await request(API + '/projects/' + encodeURIComponent(projectId) + '/media', { method: 'POST', headers: { 'Content-Type': 'application/json', 'Idempotency-Key': idempotencyKey() }, body: JSON.stringify({ source_path: result.path }) });
            if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
            if (response.body.job) { s.jobs.unshift(response.body.job); await pollJob(s, response.body.job.id, projectId, epoch); }
            else await refreshProject(s);
        } catch (_) { if (!s.disposed && epoch === s.projectEpoch && projectId === s.projectId) showNotice(s, 'importFailed', 'Could not import this Desktop file.', true); }
        finally { s.uploads.delete(key); if (!s.disposed) renderAssets(s); }
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
        s.saving = true; s.saveError = ''; renderToolbar(s);
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
                        renderToolbar(s);
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

    // Transitions apply immediately: the outgoing clip owns an exact overlap with the next clip.
    function applyTransition(s, type, frames) {
        const found = T().clipFor(s.project, s.selectedClipId);
        if (!found || s.readonly || found.track.locked || found.track.kind === 'audio') return;
        const ordered = found.track.clips.slice().sort((a, b) => a.start - b.start);
        const next = ordered[ordered.findIndex(clip => clip.id === found.clip.id) + 1];
        if (type !== 'none' && !next) return showNotice(s, 'transitionNeedsNext', 'Add a following clip on this track first.', true);
        const duration = clamp(Math.round(Number(frames) || 15), 1, 90);
        if (type !== 'none' && duration >= Math.min(found.clip.duration, next.duration)) return showNotice(s, 'transitionTooLong', 'Transition duration must be shorter than both clips.', true);
        mutate(s, 'transition', project => {
            const track = project.tracks.find(item => item.id === found.track.id), clip = track.clips.find(item => item.id === found.clip.id);
            const target = track.clips.slice().sort((a, b) => a.start - b.start), following = target[target.findIndex(item => item.id === clip.id) + 1];
            if (type === 'none') { clip.transition = null; if (following) following.start = clip.start + clip.duration; }
            else { clip.transition = { type, duration }; following.start = clip.start + clip.duration - duration; }
        });
    }

    function selectedClip(s) { return s.project ? T().clipFor(s.project, s.selectedClipId) : null; }
    function percentText(value) { return Math.round(value) + ' %'; }
    function updateOutput(input, text) { const output = input.parentElement && input.parentElement.querySelector('output'); if (output) output.textContent = text; }
    function liveField(s, input) {
        const found = selectedClip(s);
        if (!found || found.track.locked) return;
        const field = input.dataset.field, value = Number(input.value);
        if (!input._vsBefore) { input._vsBefore = snapshot(s); input._vsBox = { x: found.clip.x, y: found.clip.y, width: found.clip.width, height: found.clip.height }; }
        clearTimeout(s.autosaveTimer);
        if (field === 'opacity') { found.clip.opacity = clamp(value / 100, 0, 1); updateOutput(input, percentText(value)); }
        else if (field === 'volume') { found.clip.volume = clamp(value / 100, 0, 4); updateOutput(input, percentText(value)); }
        else if (field === 'rotation') { found.clip.rotation = clamp(Math.round(value), -360, 360); updateOutput(input, Math.round(value) + '°'); }
        else if (field === 'size') { Object.assign(found.clip, I().scaleBox(input._vsBox, clamp(value / 100, 0.05, 1))); updateOutput(input, percentText(value)); }
        else if (field === 'fade_in' || field === 'fade_out') { found.clip[field] = clamp(Math.round(value), 0, found.clip.duration); updateOutput(input, I().formatSeconds(found.clip[field]) + ' s'); }
        else return;
        s.dirty = true; s.revision++;
        renderToolbar(s);
        s.preview.seek(s.frame);
        updateTransform(s);
    }
    function commitField(s, input) {
        const found = selectedClip(s);
        if (!found || found.track.locked) { renderInspector(s); return; }
        const field = input.dataset.field;
        if (['start', 'duration', 'offset'].includes(field)) {
            const frames = I().parseTime(input.value);
            if (frames == null) { showNotice(s, 'invalidTime', 'Enter a time like 4,5 or 1:04.5.', true); renderInspector(s); return; }
            const asset = s.project.assets.find(item => item.id === found.clip.asset_id) || {};
            mutate(s, field, project => {
                const target = T().clipFor(project, found.clip.id).clip;
                if (field === 'start') target.start = clamp(frames, 0, maxFrames(s) - target.duration);
                else if (field === 'offset') target.offset = asset.kind === 'image' ? 0 : clamp(frames, 0, Math.max(0, asset.duration_frames - target.duration));
                else {
                    const sourceLimit = asset.kind === 'image' || target.text ? maxFrames(s) : asset.duration_frames - target.offset;
                    target.duration = clamp(frames, 1, Math.min(sourceLimit, maxFrames(s) - target.start));
                    target.fade_in = Math.min(target.fade_in, target.duration); target.fade_out = Math.min(target.fade_out, target.duration);
                }
            });
            return;
        }
        if (['x', 'y', 'width', 'height'].includes(field)) {
            const value = Number(input.value) / 100;
            if (!Number.isFinite(value)) { renderInspector(s); return; }
            mutate(s, 'box', project => { const target = T().clipFor(project, found.clip.id).clip; Object.assign(target, I().clampBox(Object.assign({ x: target.x, y: target.y, width: target.width, height: target.height }, { [field]: value }))); });
            return;
        }
        const before = input._vsBefore; delete input._vsBefore; delete input._vsBox;
        if (!before) return;
        if (T().validTimeline && !T().validTimeline(s.project)) {
            s.project = hydrateProject(s, before);
            showNotice(s, 'invalidTiming', 'That edit would create invalid timing or an overlap.', true);
            renderUI(s); return;
        }
        s.history.push(before); if (s.history.length > 50) s.history.shift(); s.redo = [];
        s.dirty = JSON.stringify(canonicalProject(s.project)) !== JSON.stringify(s.savedProject);
        if (s.dirty) { persistDraft(s); scheduleSave(s); } else clearDraft(s);
        renderInspector(s); renderToolbar(s); T().render(s.q('[data-timeline]'), s.timelineOptions); s.preview.seek(s.frame);
    }
    function liveText(s, input) {
        const found = selectedClip(s);
        if (!found || found.track.locked) return;
        if (!input._vsBefore) input._vsBefore = snapshot(s);
        clearTimeout(s.autosaveTimer);
        if (input.matches('[data-text-field="text"]')) found.clip.text = input.value;
        else {
            const style = found.clip.text_style || (found.clip.text_style = {});
            style[input.dataset.textStyle] = ['font_size', 'background_opacity'].includes(input.dataset.textStyle) ? Number(input.value) : input.value;
            if (input.dataset.textStyle === 'background_opacity') updateOutput(input, percentText(Number(input.value) * 100));
        }
        s.dirty = true; s.revision++;
        renderToolbar(s);
        scheduleTextApply(s);
    }
    function commitText(s, input) {
        const before = input._vsBefore; delete input._vsBefore;
        if (before) { s.history.push(before); if (s.history.length > 50) s.history.shift(); s.redo = []; }
        s.dirty = JSON.stringify(canonicalProject(s.project)) !== JSON.stringify(s.savedProject);
        if (s.dirty) { persistDraft(s); scheduleSave(s); } else clearDraft(s);
        T().render(s.q('[data-timeline]'), s.timelineOptions);
    }
    function scheduleTextApply(s) {
        clearTimeout(s.textTimer);
        s.textPending = true;
        const status = s.q('[data-text-status] span');
        if (status) status.textContent = tr(s, 'textUpdating', 'Updating the title…');
        s.textTimer = window.setTimeout(() => { s.textTimer = 0; applyTextArtwork(s); }, 900);
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
    function setTab(s, name) {
        s.libraryTab = name;
        s.app.querySelectorAll('[data-tab]').forEach(tab => { const on = tab.dataset.tab === name; tab.setAttribute('aria-selected', String(on)); tab.tabIndex = on ? 0 : -1; });
        s.app.querySelectorAll('[data-panel]').forEach(panel => { panel.hidden = panel.dataset.panel !== name; });
    }
    function handleTabKeys(s, event) {
        const tab = event.target.closest('[data-tab]');
        if (!tab || !['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
        const tabs = Array.from(s.app.querySelectorAll('[data-tab]'));
        let index = tabs.indexOf(tab);
        index = event.key === 'Home' ? 0 : event.key === 'End' ? tabs.length - 1 : (index + (event.key === 'ArrowRight' ? 1 : -1) + tabs.length) % tabs.length;
        setTab(s, tabs[index].dataset.tab); tabs[index].focus(); event.preventDefault();
    }
    function mutateSelected(s, label, fn) {
        const found = selectedClip(s);
        if (!found || found.track.locked || s.readonly) return false;
        return mutate(s, label, project => fn(T().clipFor(project, found.clip.id).clip, project));
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
    // The narrow-window drawer ends above the timeline, whatever height the user gave it.
    function syncDrawer(s) {
        const panel = s.q('.vs-timeline-panel'), handle = s.q('[data-resize]');
        if (!panel || !s.app) return;
        s.app.style.setProperty('--vs-drawer-bottom', Math.round(panel.getBoundingClientRect().height + (handle ? handle.getBoundingClientRect().height / 2 : 0)) + 'px');
    }
    function setInspectorOpen(s, open, moveFocus) {
        const app = s.app, inspector = s.q('.vs-inspector');
        syncDrawer(s);
        const narrow = app.getBoundingClientRect().width <= 920;
        app.classList.toggle('vs-show-inspector', !!open && narrow);
        if (narrow && open) {
            inspector.setAttribute('role', 'dialog'); inspector.setAttribute('aria-modal', 'true');
            if (moveFocus) inspector.querySelector('button:not(:disabled),input:not(:disabled),select:not(:disabled),textarea:not(:disabled)')?.focus();
        } else {
            inspector.removeAttribute('role'); inspector.removeAttribute('aria-modal');
        }
    }

    const POPOVERS = { project: ['project-menu', '[data-project-popover]'], jobs: ['jobs', '[data-jobs-popover]'], shortcuts: ['shortcuts', '[data-shortcuts-popover]'] };
    function togglePopover(s, name) {
        const [action, selector] = POPOVERS[name];
        const pop = s.q(selector), button = s.q(`[data-action="${action}"]`);
        const open = pop.hidden;
        closePopovers(s);
        if (!open) return;
        if (name === 'project') { renderProjectPopover(s); if (!s.featureDisabled) loadProjects(s).catch(() => {}); }
        if (name === 'jobs') renderJobs(s);
        pop.hidden = false;
        button.setAttribute('aria-expanded', 'true');
        pop.querySelector('input,button:not(:disabled),a')?.focus();
    }
    function closePopovers(s, except) {
        Object.entries(POPOVERS).forEach(([name, [action, selector]]) => {
            if (name === except) return;
            const pop = s.q(selector);
            if (pop && !pop.hidden) { pop.hidden = true; s.q(`[data-action="${action}"]`)?.setAttribute('aria-expanded', 'false'); }
        });
        renderJobsButton(s);
    }
    function closePopoversOutside(s, target) {
        const anchor = target.closest('.vs-menu-anchor');
        Object.entries(POPOVERS).forEach(([name, [, selector]]) => {
            const pop = s.q(selector);
            if (pop && !pop.hidden && (!anchor || !anchor.contains(pop))) closePopovers(s);
        });
    }
    function renderProjectPopover(s) {
        const pop = s.q('[data-project-popover]');
        if (!pop) return;
        const folder = String(s.desktopPath || '').split('/').filter(part => part && part !== s.projectId).join(' / ');
        const projects = s.projects.map(item => {
            const id = item.id || item.project_id || '';
            const name = item.project && item.project.name || tr(s, 'untitled', 'Untitled project');
            const current = id === s.projectId;
            return `<li><button type="button" data-project-id="${esc(s, id)}" ${current ? 'aria-current="true"' : ''}>${icon(current ? 'check' : 'project', 15)}<span>${esc(s, name)}</span></button></li>`;
        }).join('');
        pop.innerHTML = `${s.project ? `<div class="vs-pop-section"><label class="vs-field vs-field-wide"><span>${esc(s, tr(s, 'projectNameLabel', 'Project name'))}</span><span class="vs-inline-field"><input type="text" data-project-rename maxlength="120" value="${esc(s, s.project.name || '')}" ${s.readonly ? 'disabled' : ''}><button type="button" data-action="rename-project" ${s.readonly ? 'disabled' : ''}>${esc(s, tr(s, 'rename', 'Rename'))}</button></span></label>${folder ? `<p class="vs-hint">${icon('folder', 13)} ${esc(s, tr(s, 'savedIn', 'Saved in'))} ${esc(s, folder)}</p>` : ''}</div>` : ''}<div class="vs-pop-section"><h4>${esc(s, tr(s, 'projects', 'Projects'))}</h4>${projects ? `<ul class="vs-project-list">${projects}</ul>` : `<p class="vs-hint">${esc(s, tr(s, 'noProjectsYet', 'No projects yet.'))}</p>`}</div><div class="vs-pop-foot"><button type="button" class="vs-primary" data-action="new-project" ${s.readonly ? 'disabled' : ''}>${icon('plus', 15)}<span>${esc(s, tr(s, 'newProject', 'New project'))}</span></button></div>`;
        pop.querySelector('[data-project-rename]')?.addEventListener('keydown', event => { if (event.key === 'Enter') { event.preventDefault(); renameProject(s); } });
    }
    function renameProject(s) {
        const input = s.q('[data-project-rename]');
        const name = input ? input.value.trim().slice(0, 120) : '';
        if (!name || !s.project || name === s.project.name) { closePopovers(s); return; }
        mutate(s, 'rename', project => { project.name = name; });
        closePopovers(s);
        saveProject(s).then(() => loadProjects(s)).catch(() => {});
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

    function wireResize(s) {
        const handle = s.q('[data-resize]');
        const current = () => s.q('.vs-timeline-panel').getBoundingClientRect().height;
        const setHeight = height => {
            const max = Math.max(160, s.app.getBoundingClientRect().height - 240);
            const value = Math.round(clamp(height, 140, max));
            s.app.style.setProperty('--vs-timeline-h', value + 'px');
            updateTransform(s);
            syncDrawer(s);
            return value;
        };
        let start = null;
        handle.addEventListener('pointerdown', event => {
            if (event.button !== 0) return;
            start = { y: event.clientY, height: current() };
            handle.setPointerCapture(event.pointerId);
            handle.classList.add('is-active');
            event.preventDefault();
        }, { signal: s.abort.signal });
        handle.addEventListener('pointermove', event => { if (start) setHeight(start.height - (event.clientY - start.y)); }, { signal: s.abort.signal });
        const end = () => { if (!start) return; start = null; handle.classList.remove('is-active'); writePref(PREF_TIMELINE, Math.round(current())); };
        handle.addEventListener('pointerup', end, { signal: s.abort.signal });
        handle.addEventListener('pointercancel', end, { signal: s.abort.signal });
        handle.addEventListener('keydown', event => {
            if (event.key !== 'ArrowUp' && event.key !== 'ArrowDown') return;
            event.preventDefault();
            writePref(PREF_TIMELINE, setHeight(current() + (event.key === 'ArrowUp' ? 24 : -24)));
        }, { signal: s.abort.signal });
    }

    // Direct manipulation: the selected visual clip gets a box over the preview canvas.
    function transformTarget(s) {
        const found = selectedClip(s);
        if (!found || s.readonly || found.track.locked || found.track.hidden || found.track.kind === 'audio') return null;
        const clip = found.clip;
        if (s.frame < clip.start || s.frame >= clip.start + clip.duration) return null;
        return found;
    }
    function updateTransform(s) {
        const box = s.q('[data-transform]');
        if (!box) return;
        const found = s.project && !s.q('.vs-no-project') ? transformTarget(s) : null;
        box.hidden = !found;
        if (!found) return;
        const clip = found.clip;
        box.style.left = clip.x * 100 + '%'; box.style.top = clip.y * 100 + '%';
        box.style.width = clip.width * 100 + '%'; box.style.height = clip.height * 100 + '%';
        box.style.transform = clip.rotation ? `rotate(${clip.rotation}deg)` : '';
        box.title = tr(s, 'dragToMove', 'Drag to move, corners to resize');
    }
    function wireTransform(s) {
        const box = s.q('[data-transform]'), mat = s.q('.vs-preview-mat'), signal = s.abort.signal;
        let gesture = null;
        box.addEventListener('pointerdown', event => {
            const found = transformTarget(s);
            if (!found || event.button !== 0 || s.timelineDragging) return;
            const rect = mat.getBoundingClientRect();
            const clip = found.clip;
            gesture = { clipId: clip.id, handle: event.target.closest('[data-handle]')?.dataset.handle || '', x: event.clientX, y: event.clientY, w: rect.width, h: rect.height, box: { x: clip.x, y: clip.y, width: clip.width, height: clip.height }, before: snapshot(s), epoch: s.projectEpoch, projectId: s.projectId };
            s.timelineDragging = true; s.transformDrag = true;
            clearTimeout(s.autosaveTimer);
            box.setPointerCapture(event.pointerId);
            box.classList.add('is-dragging');
            event.preventDefault(); event.stopPropagation();
        }, { signal });
        box.addEventListener('pointermove', event => {
            if (!gesture) return;
            const found = T().clipFor(s.project, gesture.clipId);
            if (!found || gesture.epoch !== s.projectEpoch) return;
            const dx = (event.clientX - gesture.x) / gesture.w, dy = (event.clientY - gesture.y) / gesture.h, b = gesture.box;
            let next;
            if (!gesture.handle) next = I().clampBox({ x: b.x + dx, y: b.y + dy, width: b.width, height: b.height });
            else {
                const east = gesture.handle.includes('e'), south = gesture.handle.includes('s');
                const ratio = b.height / b.width;
                let width = clamp(b.width + (east ? dx : -dx), 0.03, 1);
                const maxWidth = Math.min(east ? 1 - b.x : b.x + b.width, (south ? 1 - b.y : b.y + b.height) / ratio);
                width = Math.min(width, maxWidth);
                const height = width * ratio;
                next = I().clampBox({ x: east ? b.x : b.x + b.width - width, y: south ? b.y : b.y + b.height - height, width, height });
            }
            Object.assign(found.clip, next);
            s.preview.seek(s.frame);
            updateTransform(s);
        }, { signal });
        const finish = event => {
            if (!gesture) return;
            const { before, epoch, projectId } = gesture;
            gesture = null;
            s.timelineDragging = false; s.transformDrag = false;
            box.classList.remove('is-dragging');
            if (epoch !== s.projectEpoch || projectId !== s.projectId) return;
            if (event.type === 'pointercancel') { s.project = hydrateProject(s, before); renderUI(s); return; }
            if (JSON.stringify(before) !== JSON.stringify(s.project)) recordChange(s, 'transform', before);
            if (s.pendingRefresh) { s.pendingRefresh = false; refreshProject(s).catch(() => {}); }
        };
        box.addEventListener('pointerup', finish, { signal });
        box.addEventListener('pointercancel', finish, { signal });
        // Clicking the picture selects the front-most visible clip under the pointer.
        mat.addEventListener('click', event => {
            if (event.target.closest('[data-transform],[data-no-project],button') || !s.project) return;
            const rect = mat.getBoundingClientRect();
            const px = (event.clientX - rect.left) / rect.width, py = (event.clientY - rect.top) / rect.height;
            const hit = I().displayTracks(s.project.tracks).filter(track => track.kind !== 'audio' && !track.hidden).flatMap(track => track.clips.filter(clip => s.frame >= clip.start && s.frame < clip.start + clip.duration && px >= clip.x && px <= clip.x + clip.width && py >= clip.y && py <= clip.y + clip.height))[0];
            if ((hit ? hit.id : '') === s.selectedClipId) return;
            s.timelineOptions.onSelect(hit ? hit.id : '');
            T().render(s.q('[data-timeline]'), s.timelineOptions);
        }, { signal });
    }

    function renderAIPanel(s) {
        const host = s.q('[data-ai-panel]');
        if (!host) return;
        const generation = s.status && s.status.generation;
        const ready = generation && generation.enabled && generation.configured && !generation.budget_blocked;
        const reason = !generation || !generation.enabled || !generation.configured ? tr(s, 'aiNotConfigured', 'AI video generation is not set up. An administrator can enable it under Configuration → Video generation.') : generation.budget_blocked ? tr(s, 'aiBudget', 'The video generation budget is used up for now.') : s.readonly ? tr(s, 'readonly', 'Read-only mode prevents editing.') : '';
        host.innerHTML = `<div class="vs-ai-card"><span class="vs-ai-art" aria-hidden="true">${icon('sparkle', 30)}</span><h3>${esc(s, tr(s, 'generateTitle', 'Generate a clip'))}</h3><p>${esc(s, tr(s, 'aiIntro', 'Describe a scene and the configured provider creates a short clip. It appears in your media library when it is ready.'))}</p>${generation && generation.provider ? `<p class="vs-ai-provider">${icon('info', 13)} ${esc(s, tr(s, 'usingModel', 'Provider'))}: ${esc(s, generation.provider)}${generation.model ? ' · ' + esc(s, generation.model) : ''}</p>` : ''}${reason ? `<p class="vs-hint vs-ai-reason">${esc(s, reason)}</p>` : ''}<button type="button" class="vs-primary" data-action="open-ai" ${ready && !s.readonly ? '' : 'disabled'}>${icon('sparkle', 16)}<span>${esc(s, tr(s, 'generateStart', 'Describe a clip'))}</span></button></div>`;
    }

    function scaledFont(s, size) {
        const base = Math.min(Number(s.project.width || 1280), Number(s.project.height || 720));
        return Math.round(clamp(size * base / 720, 18, 220));
    }
    async function addTitle(s, presetName) {
        if (s.readonly || !s.project || !s.projectId) return;
        if (totalClipCount(s.project) >= MAX_CLIPS) return showNotice(s, 'clipLimit', 'This project reached its clip limit.', true);
        if (activeAssetCount(s.project) >= MAX_ACTIVE_ASSETS) return showNotice(s, 'assetLimit', 'This project reached its media limit.', true);
        const preset = TITLE_PRESETS[presetName] || TITLE_PRESETS.title;
        const projectId = s.projectId, epoch = s.projectEpoch, startFrame = s.frame;
        const duration = Math.min(preset.seconds * FPS, maxFrames(s) - startFrame);
        if (duration < FPS) return showNotice(s, 'projectLimit', 'The project reached its 10-minute limit.', true);
        if (!I().findOverlaySlot(s.project, startFrame, duration)) return showNotice(s, 'overlayFull', 'Every overlay track is busy at the playhead. Move the playhead or free a track.', true);
        const text = tr(s, preset.textKey, preset.text);
        const style = Object.assign({}, preset.style, { font_size: scaledFont(s, preset.style.font_size) });
        try {
            const art = await renderTextPNG(s, text, style);
            const asset = await createArtworkAsset(s, art.file, text, projectId, epoch);
            if (!asset || s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
            const box = { width: art.width / s.project.width, height: art.height / s.project.height };
            placeOverlay(s, asset, startFrame, duration, clip => {
                Object.assign(clip, I().clampBox(Object.assign(box, I().presetBox(preset.anchor, I().clampBox(Object.assign({ x: 0, y: 0 }, box))))));
                clip.text = text; clip.text_style = style;
            });
        } catch (_) { if (!s.disposed && epoch === s.projectEpoch && projectId === s.projectId) showNotice(s, 'artworkFailed', 'Could not create the title artwork.', true); }
    }
    function placeOverlay(s, asset, start, duration, configure) {
        const slot = I().findOverlaySlot(s.project, start, duration);
        if (!slot) return showNotice(s, 'overlayFull', 'Every overlay track is busy at the playhead. Move the playhead or free a track.', true);
        const ok = mutate(s, 'add overlay', project => {
            const track = slot.trackId ? project.tracks.find(item => item.id === slot.trackId) : addTrackTo(s, project, 'overlay');
            const clip = T().makeClip(asset, start);
            clip.duration = duration;
            configure(clip);
            track.clips.push(clip);
            s.selectedClipId = clip.id; s.selectionRevision++;
        });
        if (ok) { renderInspector(s); T().revealFrame(s.q('[data-timeline]'), s, start); if (s.app.getBoundingClientRect().width <= 920) setInspectorOpen(s, true, false); }
    }
    function fontFor(style) { return `${style.italic ? 'italic ' : ''}${style.bold ? '700 ' : ''}${Number(style.font_size || 72)}px "${String(style.font_family || 'Arial').replace(/["\\]/g, '')}"`; }
    // Titles are PNGs cropped to their text block and positioned with the clip box like stickers.
    function renderTextPNG(s, text, style) {
        const W = Number(s.project.width || 1280), H = Number(s.project.height || 720);
        const size = Number(style.font_size || 72), font = fontFor(style);
        const measure = document.createElement('canvas').getContext('2d');
        measure.font = font;
        const maxWidth = W * 0.84;
        const wrapped = String(text || ' ').split(/\r?\n/).slice(0, 8).flatMap(line => wrapCanvasText(measure, line, maxWidth));
        const lineHeight = size * 1.2;
        const background = Number(style.background_opacity || 0) > 0;
        const padX = Math.round(size * (background ? 0.5 : 0.25)), padY = Math.round(size * (background ? 0.3 : 0.2));
        const textWidth = Math.max(1, ...wrapped.map(line => measure.measureText(line).width));
        const canvas = document.createElement('canvas');
        canvas.width = Math.ceil(Math.min(W, textWidth + padX * 2)); canvas.height = Math.ceil(Math.min(H, wrapped.length * lineHeight + padY * 2));
        const ctx = canvas.getContext('2d');
        if (background) { ctx.fillStyle = hexWithAlpha(style.background_color || '#101319', Number(style.background_opacity || 0.45)); ctx.fillRect(0, 0, canvas.width, canvas.height); }
        ctx.font = font; ctx.textBaseline = 'middle';
        ctx.textAlign = ['left', 'right'].includes(style.alignment) ? style.alignment : 'center';
        const x = style.alignment === 'left' ? padX : style.alignment === 'right' ? canvas.width - padX : canvas.width / 2;
        ctx.fillStyle = style.color || '#fff'; ctx.shadowColor = 'rgba(0,0,0,.6)'; ctx.shadowBlur = Math.max(2, size * 0.08);
        wrapped.forEach((line, index) => ctx.fillText(line, x, padY + lineHeight * (index + 0.5), canvas.width - padX * 2));
        return new Promise((resolve, reject) => canvas.toBlob(blob => blob ? resolve({ file: new File([blob], 'title-' + Date.now() + '.png', { type: 'image/png' }), width: canvas.width, height: canvas.height }) : reject(new Error('canvas_export_failed')), 'image/png'));
    }
    function wrapCanvasText(ctx, text, maxWidth) {
        const words = String(text || ' ').split(/\s+/), lines = []; let line = '';
        words.forEach(word => {
            const candidate = line ? line + ' ' + word : word;
            if (line && ctx.measureText(candidate).width > maxWidth) { lines.push(line); line = word; } else line = candidate;
        });
        lines.push(line || ' '); return lines;
    }
    function hexWithAlpha(hex, alpha) {
        const value = /^#[0-9a-f]{6}$/i.test(hex) ? hex : '#101319';
        return `rgba(${parseInt(value.slice(1, 3), 16)},${parseInt(value.slice(3, 5), 16)},${parseInt(value.slice(5, 7), 16)},${clamp(alpha, 0, 1)})`;
    }
    function textStyleKey(style) { const defaults = { font_family: 'Arial', font_size: 72, color: '#ffffff', bold: false, italic: false, alignment: 'center', outline_color: '', outline_width: 0, background_color: '#101319', background_opacity: 0 }; return Object.keys(defaults).map(key => style && style[key] || defaults[key]).join('|'); }
    async function applyTextArtwork(s) {
        const found = T().clipFor(s.project || { tracks: [] }, s.selectedClipId);
        if (!found || !found.clip.text || s.readonly || found.track.locked) { s.textPending = false; return; }
        const projectId = s.projectId, epoch = s.projectEpoch, clipId = found.clip.id, selectionRevision = s.selectionRevision;
        const revision = ++s.artworkRevision, originalAssetId = found.clip.asset_id, text = found.clip.text, style = clone(found.clip.text_style || {}), styleKey = textStyleKey(style);
        s.textPending = true;
        try {
            const art = await renderTextPNG(s, text, style);
            const asset = await createArtworkAsset(s, art.file, 'Title · ' + text.slice(0, 40), projectId, epoch);
            if (!asset || s.disposed || revision !== s.artworkRevision || epoch !== s.projectEpoch || projectId !== s.projectId || s.selectedClipId !== clipId || s.selectionRevision !== selectionRevision) return;
            const current = T().clipFor(s.project, clipId);
            if (!current || current.clip.text !== text || textStyleKey(current.clip.text_style || {}) !== styleKey || current.clip.asset_id !== originalAssetId) return;
            mutate(s, 'update title artwork', project => {
                const target = T().clipFor(project, clipId);
                if (!target) return;
                const clip = target.clip, previous = project.assets.find(item => item.id === clip.asset_id);
                const scale = previous && previous.width ? clamp(clip.width / (previous.width / project.width), 0.1, 10) : 1;
                const width = art.width / project.width * scale, height = art.height / project.height * scale;
                clip.asset_id = asset.id;
                Object.assign(clip, I().clampBox({ x: clip.x + clip.width / 2 - width / 2, y: clip.y + clip.height / 2 - height / 2, width, height }));
            });
        } catch (_) { if (!s.disposed && epoch === s.projectEpoch && projectId === s.projectId) showNotice(s, 'artworkFailed', 'Could not update the title artwork.', true); }
        finally {
            if (revision === s.artworkRevision && !s.disposed) {
                s.textPending = false;
                const status = s.q('[data-text-status] span');
                if (status) status.textContent = tr(s, 'textAuto', 'Changes are applied automatically.');
            }
        }
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
    function stickerSVG(name, size) {
        const art = {
            sparkle: '<path d="M48 4 58 37 92 48 58 59 48 92 38 59 4 48 38 37z"/><circle cx="78" cy="22" r="5"/><circle cx="20" cy="78" r="4"/>',
            heart: '<path d="M48 82 15 51C-2 34 10 10 29 13c9 1 15 7 19 15 4-8 11-14 20-15 19-3 31 21 14 38z"/>',
            sun: '<circle cx="48" cy="48" r="19"/><path d="M48 5v17m0 52v17M5 48h17m52 0h17M18 18l12 12m36 36 12 12m4-60L70 30M30 66 18 78"/>',
            star: '<path d="m48 6 11 28 30 2-23 19 8 30-26-17-26 17 8-30L7 36l30-2z"/>',
            flower: '<g><ellipse cx="48" cy="24" rx="12" ry="23"/><ellipse cx="48" cy="72" rx="12" ry="23"/><ellipse cx="24" cy="48" rx="23" ry="12"/><ellipse cx="72" cy="48" rx="23" ry="12"/><circle cx="48" cy="48" r="11"/></g>',
            burst: '<path d="M48 4 56 35 82 16 65 42 96 48 65 55 82 80 56 62 48 94 41 62 15 80 32 55 1 48 32 42 15 16 41 35z"/>'
        };
        const fill = name === 'heart' ? '#fb7185' : name === 'sun' ? '#fbbf24' : name === 'flower' ? '#f472b6' : name === 'star' ? '#facc15' : '#a78bfa';
        const px = size || 512;
        return `<svg xmlns="http://www.w3.org/2000/svg" width="${px}" height="${px}" viewBox="0 0 96 96"><g fill="${fill}" stroke="#fff" stroke-width="2" stroke-linejoin="round">${art[name] || art.sparkle}</g></svg>`;
    }
    async function stickerPNG(name) {
        const blob = new Blob([stickerSVG(name, 512)], { type: 'image/svg+xml' }); const url = URL.createObjectURL(blob);
        try {
            const image = new Image(); image.src = url; await image.decode();
            const canvas = document.createElement('canvas'); canvas.width = 512; canvas.height = 512; canvas.getContext('2d').drawImage(image, 0, 0);
            return await new Promise((resolve, reject) => canvas.toBlob(blobValue => blobValue ? resolve(new File([blobValue], 'sticker-' + name + '.png', { type: 'image/png' })) : reject(new Error('canvas_export_failed')), 'image/png'));
        } finally { URL.revokeObjectURL(url); }
    }
    async function addSticker(s, name) {
        if (s.readonly || !s.project || !s.projectId) return;
        if (totalClipCount(s.project) >= MAX_CLIPS) return showNotice(s, 'clipLimit', 'This project reached its clip limit.', true);
        if (activeAssetCount(s.project) >= MAX_ACTIVE_ASSETS) return showNotice(s, 'assetLimit', 'This project reached its media limit.', true);
        const projectId = s.projectId, epoch = s.projectEpoch, startFrame = s.frame;
        const duration = Math.min(FPS * 4, maxFrames(s) - startFrame);
        if (duration < FPS) return showNotice(s, 'projectLimit', 'The project reached its 10-minute limit.', true);
        if (!I().findOverlaySlot(s.project, startFrame, duration)) return showNotice(s, 'overlayFull', 'Every overlay track is busy at the playhead. Move the playhead or free a track.', true);
        try {
            const file = await stickerPNG(name);
            if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
            const asset = await createArtworkAsset(s, file, tr(s, 'sticker_' + name, name), projectId, epoch);
            if (!asset || s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
            const side = 0.24 * Math.min(s.project.width, s.project.height);
            const box = { width: side / s.project.width, height: side / s.project.height };
            placeOverlay(s, asset, startFrame, duration, clip => { Object.assign(clip, box, I().presetBox('tr', box)); });
        } catch (_) { if (!s.disposed && epoch === s.projectEpoch && projectId === s.projectId) showNotice(s, 'artworkFailed', 'Could not add this sticker.', true); }
    }
    function openModal(s, markup) {
        const host = s.q('[data-modal-host]');
        closePopovers(s);
        host.innerHTML = `<div class="vs-modal-backdrop">${markup}</div>`;
        host.querySelectorAll('[data-modal-close]').forEach(button => button.addEventListener('click', () => { host.replaceChildren(); s.app.focus({ preventScroll: true }); }, { once: true }));
        host.querySelector('.vs-modal textarea,.vs-modal select,.vs-modal input,.vs-modal [type="submit"]')?.focus();
        return host;
    }
    function openExport(s) {
        if (!s.projectId || s.readonly) return;
        const projectId = s.projectId, epoch = s.projectEpoch;
        const size = `${s.project.width}x${s.project.height}`;
        const host = openModal(s, `<form class="vs-modal" data-export-form role="dialog" aria-modal="true" aria-labelledby="vs-export-title-${esc(s, s.id)}"><button type="button" class="vs-modal-close" data-modal-close aria-label="${esc(s, tr(s, 'close', 'Close'))}">${icon('close', 18)}</button><span class="vs-modal-icon">${icon('export', 22)}</span><h2 id="vs-export-title-${esc(s, s.id)}">${esc(s, tr(s, 'exportVideo', 'Export video'))}</h2><p>${esc(s, tr(s, 'exportIntro', 'The video is rendered on the server. You can keep editing while it runs and download it when it is ready.'))}</p><dl class="vs-facts"><div><dt>${esc(s, tr(s, 'length', 'Length'))}</dt><dd>${esc(s, I().formatTime(projectDuration(s)))}</dd></div><div><dt>${esc(s, tr(s, 'fileName', 'File'))}</dt><dd>${esc(s, exportName(s))}</dd></div><div><dt>${esc(s, tr(s, 'fileFormat', 'Format'))}</dt><dd>MP4 · H.264 · 30 fps</dd></div></dl><label class="vs-field vs-field-wide"><span>${esc(s, tr(s, 'resolution', 'Resolution and aspect ratio'))}</span><select data-export-size>${s.q('[data-canvas]').innerHTML}</select></label><div class="vs-modal-actions"><button type="button" data-modal-close>${esc(s, tr(s, 'cancel', 'Cancel'))}</button><button type="submit" class="vs-primary">${icon('export', 16)}<span>${esc(s, tr(s, 'startExport', 'Start export'))}</span></button></div></form>`);
        host.querySelector('[data-export-size]').value = size;
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
        const durations = generation.durations_seconds || [];
        const supportsAspectRatio = String(generation.provider || '').toLowerCase() !== 'minimax';
        const images = s.project.assets.filter(asset => asset.kind === 'image' && !window.VideoStudioMedia.isArtwork(asset));
        const supportsStartImage = Array.isArray(generation.image_modes) && generation.image_modes.includes('first_frame');
        const imageField = supportsStartImage ? `<label class="vs-field vs-field-wide"><span>${esc(s, tr(s, 'startImage', 'Start image (optional)'))}</span><select data-ai-image><option value="">${esc(s, tr(s, 'none', 'None'))}</option>${images.map(asset => `<option value="${esc(s, asset.id)}">${esc(s, asset.name)}</option>`).join('')}</select></label>` : '';
        const host = openModal(s, `<form class="vs-modal vs-ai-modal" data-ai-form role="dialog" aria-modal="true" aria-labelledby="vs-ai-title-${esc(s, s.id)}"><button type="button" class="vs-modal-close" data-modal-close aria-label="${esc(s, tr(s, 'close', 'Close'))}">${icon('close', 18)}</button><span class="vs-modal-icon">${icon('sparkle', 22)}</span><h2 id="vs-ai-title-${esc(s, s.id)}">${esc(s, tr(s, 'generateTitle', 'Generate a clip'))}</h2><p>${esc(s, tr(s, 'aiHint', 'Generated clips appear in your media library. Add them to the timeline when you are ready.'))}</p><label class="vs-field vs-field-wide"><span>${esc(s, tr(s, 'prompt', 'Describe the clip'))}</span><textarea data-ai-prompt rows="4" maxlength="2000" required placeholder="${esc(s, tr(s, 'promptHint', 'A slow aerial view over a quiet coastline at dawn'))}"></textarea></label><div class="vs-field-row" data-ai-options><label class="vs-field"><span>${esc(s, tr(s, 'duration', 'Duration'))}</span><select data-ai-duration>${durations.map(value => `<option value="${Number(value)}">${Number(value)} ${esc(s, tr(s, 'seconds', 'seconds'))}</option>`).join('')}</select></label></div>${imageField}<div class="vs-ai-provider">${icon('info', 13)} ${esc(s, tr(s, 'usingModel', 'Provider'))}${generation.provider ? ': ' + esc(s, generation.provider) : ''}</div><div class="vs-modal-actions"><button type="button" data-modal-close>${esc(s, tr(s, 'cancel', 'Cancel'))}</button><button type="submit" class="vs-primary">${icon('sparkle', 16)}<span>${esc(s, tr(s, 'generateSubmit', 'Generate'))}</span></button></div></form>`);
        const ratioLabel = document.createElement('label');
        ratioLabel.className = 'vs-field';
        const ratioText = document.createElement('span'); ratioText.textContent = tr(s, 'aspectRatio', 'Aspect ratio'); ratioLabel.append(ratioText);
        const ratioSelect = document.createElement('select'); ratioSelect.dataset.aiRatio = '';
        [['16:9', 'ratioLandscape', '16:9'], ['9:16', 'ratioPortrait', '9:16'], ['1:1', 'ratioSquare', '1:1']].forEach(([value, key, fallback]) => {
            const option = document.createElement('option'); option.value = value; option.textContent = tr(s, key, fallback); ratioSelect.append(option);
        });
        ratioSelect.value = s.project.width === s.project.height ? '1:1' : s.project.width > s.project.height ? '16:9' : '9:16';
        ratioLabel.append(ratioSelect); if (supportsAspectRatio) host.querySelector('[data-ai-options]').append(ratioLabel);
        host.querySelector('[data-ai-form]').addEventListener('submit', async event => {
            event.preventDefault();
            const prompt = host.querySelector('[data-ai-prompt]').value.trim();
            const duration = Number(host.querySelector('[data-ai-duration]').value);
            if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) { host.replaceChildren(); return; }
            const firstFrame = host.querySelector('[data-ai-image]')?.value || '';
            const payload = { kind: 'generate', prompt, duration_seconds: duration }; if (supportsAspectRatio) payload.aspect_ratio = ratioSelect.value;
            if (firstFrame) payload.first_frame_asset_id = firstFrame;
            host.replaceChildren();
            try { await createJob(s, payload, projectId, epoch); if (epoch === s.projectEpoch && projectId === s.projectId) showNotice(s, 'generationStarted', 'Generation started. Results will appear in your media library.', false); }
            catch (error) { if (epoch === s.projectEpoch && projectId === s.projectId) { const failure = jobFailure({ status: 'failed', error: error.body && error.body.code }); showNotice(s, failure[0] === 'jobFailed' ? 'generationFailed' : failure[0], failure[0] === 'jobFailed' ? 'Could not start generation.' : failure[1], true); } }
        });
    }

    window.VideoStudioApp = { render: appRender, dispose, instances };
})();
