(function () {
    'use strict';

    const P = window.VideoStudioParts;
    const { API, CANVASES, MAX_CLIPS, POPOVERS, PREF_MAGNETIC, PREF_QUALITY, PREF_SNAP, PREF_TIMELINE, READ_ONLY_SAFE, STICKERS, TITLE_PRESETS } = P;
    const I = (...args) => P.I(...args);
    const T = (...args) => P.T(...args);
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
    const openNewProject = (...args) => P.openNewProject(...args);
    const openProjectSettings = (...args) => P.openProjectSettings(...args);
    const projectDuration = (...args) => P.projectDuration(...args);
    const queueAutosave = (...args) => P.queueAutosave(...args);
    const rateOf = (...args) => P.rateOf(...args);
    const readPref = (...args) => P.readPref(...args);
    const recordChange = (...args) => P.recordChange(...args);
    const recoverDraft = (...args) => P.recoverDraft(...args);
    const refreshProject = (...args) => P.refreshProject(...args);
    const renameProject = (...args) => P.renameProject(...args);
    const openProjectManager = (...args) => P.openProjectManager(...args);
    const wireMediaBin = (...args) => P.wireMediaBin(...args);
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
        if (P.closeMenu) P.closeMenu(s, false);
        if (P.closeMarkerPopover) P.closeMarkerPopover(s, false);
        clearTimeout(s.autosaveTimer);
        clearTimeout(s.toastTimer);
        if (s.textTimers) P.cancelTextApplies(s);
        clearInterval(s.jobsTimer);
        s.xhrs.forEach(xhr => { try { xhr.abort(); } catch (_) { /* finished */ } });
        if (s.resizeObserver) s.resizeObserver.disconnect();
        if (s.modalObserver) s.modalObserver.disconnect();
        if (s.preview) s.preview.dispose();
        if (P.disposeSource) P.disposeSource(s);
        if (s.media) s.media.dispose();
        if (s.detachTimeline) s.detachTimeline();
        if (s.keyHandler) window.removeEventListener('keydown', s.keyHandler, true);
        if (s.ctx.clearWindowMenus) s.ctx.clearWindowMenus(id);
        instances.delete(id);
    }
    function layoutMarkup(s) {
        const label = (key, fallback) => esc(s, tr(s, key, fallback));
        const tab = (name, iconName, key, fallback) => `<button type="button" role="tab" data-tab="${name}" aria-selected="${name === 'media'}" aria-controls="vs-panel-${s.id}-${name}" id="vs-tab-${s.id}-${name}" title="${label(key, fallback)}" ${name === 'media' ? '' : 'tabindex="-1"'}>${icon(iconName, 16)}<span>${label(key, fallback)}</span></button>`;
        const presets = Object.keys(TITLE_PRESETS).map(name => `<button type="button" class="vs-preset vs-preset-${name}" data-action="add-title" data-preset="${name}"><span class="vs-preset-sample" aria-hidden="true">${label('presetSample_' + name, name === 'lower' ? 'Name' : 'Aa')}</span><span class="vs-preset-text"><strong>${label('preset_' + name, name)}</strong><small>${label('presetHint_' + name, '')}</small></span>${icon('plus', 16, 'vs-icon vs-preset-add')}</button>`).join('');
        const stickers = STICKERS.map(name => `<button type="button" class="vs-sticker" data-sticker="${name}" title="${label('sticker_' + name, name)}"><span class="vs-sticker-art" aria-hidden="true">${stickerSVG(name, 56)}</span><small>${label('sticker_' + name, name)}</small></button>`).join('');
        const key = name => tr(s, 'key' + name, name);
        const shortcuts = [
            [[key('Space')], 'shortcutPlay', 'Play / pause'], [['J', 'K', 'L'], 'shortcutShuttle', 'Shuttle back / stop / forward (K + J/L: one frame)'],
            [['←', '→'], 'shortcutFrame', 'One frame (Shift: one second)'], [[key('Home'), key('End')], 'shortcutEnds', 'Start / end'],
            [['↑', '↓'], 'shortcutEditPoint', 'Previous / next edit point'], [['I', 'O'], 'shortcutInOut', 'Set in / out point'],
            [[key('Shift'), 'I', 'O'], 'shortcutGoInOut', 'Go to in / out point'], [[key('Alt'), 'X'], 'shortcutClearInOut', 'Clear in and out points'],
            [['M'], 'shortcutMarker', 'Add marker (or open the one at the playhead)'], [[key('Shift'), 'M'], 'shortcutMarkerNext', 'Next marker'],
            [[key('Ctrl'), key('Shift'), 'M'], 'shortcutMarkerPrev', 'Previous marker'], [[key('Ctrl'), 'G'], 'shortcutGoTo', 'Go to timecode'],
            [['V', 'T', 'B'], 'shortcutTools', 'Selection, trim and blade tool'], [['S'], 'shortcutSplit', 'Split at the playhead'],
            [['Q', 'W'], 'shortcutTrimPlayhead', 'Ripple trim start / end to the playhead'], [[key('Del')], 'shortcutDelete', 'Delete the selected clip'],
            [[key('Shift'), key('Del')], 'shortcutRippleDelete', 'Ripple delete'], [[key('Ctrl'), 'C', 'X', 'V'], 'shortcutClipboard', 'Copy, cut, paste (Shift: insert)'],
            [[key('Ctrl'), key('Alt'), 'V'], 'shortcutPasteAttributes', 'Paste attributes'], [[key('Ctrl'), 'D'], 'shortcutDuplicate', 'Duplicate'],
            [[key('Ctrl'), 'A'], 'shortcutSelectAll', 'Select all clips'], [[key('Alt'), '←', '→'], 'shortcutNudge', 'Nudge clips (Shift: 10 frames)'],
            [['N'], 'shortcutSnap', 'Snapping on / off'], [[key('Shift'), 'Z'], 'shortcutFit', 'Fit the timeline'],
            [[key('Ctrl'), 'Z'], 'shortcutUndo', 'Undo (Shift: redo)'], [['+', '−'], 'shortcutZoom', 'Zoom the timeline'],
            [[key('Ctrl'), key('Wheel')], 'shortcutWheel', 'Zoom at the pointer'], [[key('Shift'), key('Wheel')], 'shortcutHScroll', 'Scroll the timeline sideways'],
            [[key('Shift'), 'F10'], 'shortcutMenu', 'Clip menu'],
            [[key('Ctrl'), 'L'], 'shortcutLink', 'Link / unlink the selected clips'], [[key('Shift'), 'E'], 'shortcutDisable', 'Disable / enable the selected clips'],
            [[',', '.'], 'shortcutSourceEdit', 'Source monitor: insert (,) / overwrite (.)']
        ];
        return `<div class="vs-app" tabindex="0">
<header class="vs-toolbar">
 <div class="vs-tb-group vs-tb-project"><div class="vs-menu-anchor"><button type="button" class="vs-project-button" data-action="project-menu" aria-haspopup="dialog" aria-expanded="false" title="${label('projects', 'Projects')}">${icon('project', 18)}<span class="vs-project-name" data-project-name>${label('noProject', 'No project')}</span>${icon('chevronDown', 14)}</button><div class="vs-popover vs-project-popover" data-project-popover role="dialog" aria-label="${label('projects', 'Projects')}" hidden></div></div><button type="button" class="vs-save-chip" data-action="save" data-save-state></button></div>
 <div class="vs-tb-group vs-tb-edit"><button type="button" class="vs-icon-button" data-action="undo" aria-label="${label('undo', 'Undo')}" title="${label('undo', 'Undo')} (${esc(s, key('Ctrl'))}+Z)" disabled>${icon('undo', 18)}</button><button type="button" class="vs-icon-button" data-action="redo" aria-label="${label('redo', 'Redo')}" title="${label('redo', 'Redo')} (${esc(s, key('Ctrl'))}+${esc(s, key('Shift'))}+Z)" disabled>${icon('redo', 18)}</button><label class="vs-format" title="${label('format', 'Format')}">${icon('film', 16)}<select data-canvas aria-label="${label('format', 'Format')}"></select></label></div>
 <div class="vs-tb-group vs-tb-actions"><div class="vs-menu-anchor"><button type="button" class="vs-jobs-button" data-action="jobs" aria-haspopup="dialog" aria-expanded="false" hidden><span class="vs-jobs-icon" data-jobs-icon></span><span data-jobs-label></span></button><div class="vs-popover vs-jobs-popover" data-jobs-popover role="dialog" aria-label="${label('backgroundJobs', 'Background jobs')}" hidden><h3>${label('backgroundJobs', 'Background jobs')}</h3><section class="vs-jobs" data-jobs></section></div></div><div class="vs-menu-anchor"><button type="button" class="vs-icon-button" data-action="shortcuts" aria-haspopup="dialog" aria-expanded="false" aria-label="${label('shortcuts', 'Keyboard shortcuts')}" title="${label('shortcuts', 'Keyboard shortcuts')}">${icon('keyboard', 18)}</button><div class="vs-popover vs-shortcuts-popover" data-shortcuts-popover role="dialog" aria-label="${label('shortcuts', 'Keyboard shortcuts')}" hidden><h3>${label('shortcuts', 'Keyboard shortcuts')}</h3><dl>${shortcuts.map(([keys, key, fallback]) => `<div><dt>${keys.map(k => `<kbd>${esc(s, k)}</kbd>`).join('')}</dt><dd>${label(key, fallback)}</dd></div>`).join('')}</dl></div></div><button type="button" class="vs-inspector-toggle" data-action="toggle-inspector" aria-label="${label('inspector', 'Inspector')}" title="${label('inspector', 'Inspector')}">${icon('sliders', 17)}<span>${label('inspector', 'Inspector')}</span></button><button type="button" class="vs-export-button vs-primary" data-action="export">${icon('export', 17)}<span>${label('export', 'Export')}</span></button></div>
</header>
<div class="vs-disabled" data-disabled hidden></div>
<div class="vs-workspace">
 <aside class="vs-library" aria-label="${label('library', 'Library')}">
  <div class="vs-tabs" role="tablist" aria-label="${label('library', 'Library')}">${tab('media', 'film', 'media', 'Media')}${tab('text', 'text', 'textTab', 'Text')}${tab('stickers', 'sticker', 'stickersTab', 'Stickers')}${tab('ai', 'sparkle', 'aiTab', 'AI')}</div>
  <div class="vs-tab-panel vs-media-panel" data-panel="media" role="tabpanel" id="vs-panel-${s.id}-media" aria-labelledby="vs-tab-${s.id}-media">
   <div class="vs-import"><button type="button" class="vs-import-main" data-action="upload">${icon('upload', 16)}<span>${label('upload', 'Upload')}</span></button><button type="button" data-action="browse">${icon('folder', 16)}<span>${label('browseShort', 'From files')}</span></button><input type="file" data-file-input accept="video/*,audio/*,image/png,image/webp" multiple hidden></div>
   <div class="vs-media-tools"><label class="vs-search">${icon('search', 15)}<input type="search" data-search placeholder="${label('searchMedia', 'Search media')}" aria-label="${label('searchMedia', 'Search media')}"></label><div class="vs-filter" role="group" aria-label="${label('filterMedia', 'Filter media')}" data-filters></div></div>
   <div class="vs-media-bar" data-media-bar></div>
   <div class="vs-asset-grid" data-assets></div>
   <div class="vs-media-foot"><span data-asset-count></span><span>${label('dropHint', 'Drop files anywhere to import')}</span></div>
  </div>
  <div class="vs-tab-panel" data-panel="text" role="tabpanel" id="vs-panel-${s.id}-text" aria-labelledby="vs-tab-${s.id}-text" hidden><p class="vs-panel-intro">${label('textIntro', 'Text lands at the playhead on a free overlay track. Select it to change words and style.')}</p><div class="vs-preset-list">${presets}</div></div>
  <div class="vs-tab-panel" data-panel="stickers" role="tabpanel" id="vs-panel-${s.id}-stickers" aria-labelledby="vs-tab-${s.id}-stickers" hidden><p class="vs-panel-intro">${label('stickerHint', 'Add a small original graphic at the playhead.')}</p><div class="vs-sticker-grid">${stickers}</div></div>
  <div class="vs-tab-panel vs-ai-panel" data-panel="ai" role="tabpanel" id="vs-panel-${s.id}-ai" aria-labelledby="vs-tab-${s.id}-ai" hidden data-ai-panel></div>
 </aside>
 <main class="vs-center">
  <div class="vs-monitor-bar"><div class="vs-segmented vs-stage-switch" data-stage-switch role="group" aria-label="${label('monitor', 'Monitor')}"><button type="button" data-stage-mode="source" aria-pressed="false">${label('sourceMonitor', 'Source')}</button><button type="button" data-stage-mode="timeline" aria-pressed="true">${label('timelineMonitor', 'Timeline')}</button></div></div>
  <div class="vs-preview-stage" data-stage="timeline"><div class="vs-preview-mat"><canvas data-preview width="1280" height="720" aria-label="${label('preview', 'Preview')}"></canvas><div class="vs-preview-empty" data-preview-empty hidden></div><div class="vs-transform" data-transform hidden><span class="vs-handle" data-handle="nw"></span><span class="vs-handle" data-handle="ne"></span><span class="vs-handle" data-handle="sw"></span><span class="vs-handle" data-handle="se"></span></div></div></div>
  <div class="vs-transport" data-stage="timeline"><div class="vs-transport-buttons"><button type="button" data-action="seek-start" aria-label="${label('toStart', 'To start')}" title="${label('toStart', 'To start')} (${esc(s, key('Home'))})">${icon('skipStart', 17)}</button><button type="button" data-action="step-back" aria-label="${label('previousFrame', 'Previous frame')}" title="${label('previousFrame', 'Previous frame')} (←)">${icon('stepBack', 17)}</button><button type="button" class="vs-play" data-action="play" aria-label="${label('play', 'Play')}" title="${label('play', 'Play')} (${esc(s, key('Space'))})">${icon('play', 20)}</button><button type="button" data-action="step-forward" aria-label="${label('nextFrame', 'Next frame')}" title="${label('nextFrame', 'Next frame')} (→)">${icon('stepForward', 17)}</button><button type="button" data-action="seek-end" aria-label="${label('toEnd', 'To end')}" title="${label('toEnd', 'To end')} (${esc(s, key('End'))})">${icon('skipEnd', 17)}</button></div><div class="vs-time" aria-live="off"><button type="button" class="vs-time-button" data-action="goto-time" title="${label('goToTitle', 'Go to timecode')} (${esc(s, key('Ctrl'))}+G)"><span class="vs-time-current" data-time>0:00.00</span><span class="vs-sr-only"> – ${label('goToTitle', 'Go to timecode')}</span></button><span class="vs-timecode" data-transport-time>/ 0:00.00</span></div><div class="vs-transport-right"><div class="vs-mark-buttons"><button type="button" data-action="mark-in" aria-label="${label('markIn', 'In')}" title="${label('markIn', 'In')} (I)">I</button><button type="button" data-action="mark-out" aria-label="${label('markOut', 'Out')}" title="${label('markOut', 'Out')} (O)">O</button><button type="button" data-action="loop" aria-pressed="false" aria-label="${label('loop', 'Loop')}" title="${label('loopHint', 'Play from in to out, or the whole timeline, repeatedly')}">${icon('loop', 17)}</button><button type="button" data-action="add-marker" aria-label="${label('addMarker', 'Add marker')}" title="${label('addMarker', 'Add marker')} (M)">${icon('flag', 17)}</button></div><label class="vs-quality" title="${label('previewQuality', 'Preview quality')}"><select data-preview-quality aria-label="${label('previewQuality', 'Preview quality')}">${['auto', 'full', 'half', 'quarter'].map(q => `<option value="${q}">${label('quality_' + q, q)}</option>`).join('')}</select></label><button type="button" data-action="fullscreen" aria-label="${label('fullscreen', 'Full screen preview')}" title="${label('fullscreen', 'Full screen preview')}">${icon('fullscreen', 17)}</button></div></div>
 <section class="vs-source" data-source hidden aria-label="${label('sourceMonitor', 'Source')}">
  <div class="vs-source-stage"><p class="vs-source-empty" data-source-empty>${label('sourceEmpty', 'Double-click a media item in the library to open it here.')}</p><div class="vs-source-body" data-source-body hidden><div class="vs-source-mat"><canvas data-source-canvas width="1280" height="720" aria-label="${label('sourceMonitor', 'Source')}"></canvas></div><div class="vs-source-wave" data-source-wave hidden></div></div></div>
  <div class="vs-source-meta"><span data-source-name></span><span class="vs-source-time"><span data-source-time>0:00.00</span> / <span data-source-length>0:00.00</span></span></div>
  <div class="vs-source-bar">
   <button type="button" data-source-action="to-start" aria-label="${label('toStart', 'To start')}" title="${label('toStart', 'To start')} (${esc(s, key('Home'))})">${icon('skipStart', 17)}</button>
   <button type="button" data-source-action="step-back" aria-label="${label('previousFrame', 'Previous frame')}" title="${label('previousFrame', 'Previous frame')} (←)">${icon('stepBack', 17)}</button>
   <button type="button" class="vs-play" data-source-action="play" aria-label="${label('play', 'Play')}" title="${label('play', 'Play')} (${esc(s, key('Space'))})">${icon('play', 18)}</button>
   <button type="button" data-source-action="step-forward" aria-label="${label('nextFrame', 'Next frame')}" title="${label('nextFrame', 'Next frame')} (→)">${icon('stepForward', 17)}</button>
   <button type="button" data-source-action="to-end" aria-label="${label('toEnd', 'To end')}" title="${label('toEnd', 'To end')} (${esc(s, key('End'))})">${icon('skipEnd', 17)}</button>
   <input type="range" class="vs-source-scrub" data-source-scrub min="0" max="0" step="1" value="0" aria-label="${label('sourcePosition', 'Source position')}">
   <button type="button" data-source-action="mark-in" title="${label('sourceMarkIn', 'Mark in')} (I)">${label('sourceMarkIn', 'Mark in')} <small data-source-in-text>-</small></button>
   <button type="button" data-source-action="mark-out" title="${label('sourceMarkOut', 'Mark out')} (O)">${label('sourceMarkOut', 'Mark out')} <small data-source-out-text>-</small></button>
   <button type="button" data-source-action="go-in">${label('sourceGoIn', 'Go to in')}</button>
   <button type="button" data-source-action="go-out">${label('sourceGoOut', 'Go to out')}</button>
   <button type="button" data-source-action="clear-marks">${label('sourceClearMarks', 'Clear in and out')}</button>
   <button type="button" data-source-drag="picture" draggable="true" title="${label('sourcePartHint', 'Drag onto a track to place it. Enter overwrites at the edit point, Ctrl+Enter inserts.')}">${icon('video', 15)}<span>${label('sourcePicture', 'Picture only')}</span></button>
   <button type="button" data-source-drag="sound" draggable="true" title="${label('sourcePartHint', 'Drag onto a track to place it. Enter overwrites at the edit point, Ctrl+Enter inserts.')}">${icon('audio', 15)}<span>${label('sourceSound', 'Sound only')}</span></button>
   <button type="button" class="vs-source-insert" data-source-action="insert" title="${label('sourceInsert', 'Insert')} (,)">${label('sourceInsert', 'Insert')}</button>
   <button type="button" data-source-action="overwrite" title="${label('sourceOverwrite', 'Overwrite')} (.)">${label('sourceOverwrite', 'Overwrite')}</button>
  </div>
 </section>
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
            selectionSet: new Set(), selectionPrimary: '', clipboard: null, tool: 'select', trimMode: '', peekReturn: null, ...P.readMediaPrefs(), mediaFolder: '', selectedAssetId: '',
            zoom: 1, snap: readPref(PREF_SNAP) !== 'false', magnetic: readPref(PREF_MAGNETIC) === 'true', loop: false, soloIds: new Set(), renamingTrackId: '', previewQuality: readPref(PREF_QUALITY) || 'auto', dirty: false, saving: false, saveError: '', conflict: false, history: [], redo: [],
            busy: false, revision: 0, artworkRevision: 0, savePromise: null, autosaveTimer: 0, libraryTab: 'media', filter: 'all',
            uploads: new Map(), notifiedJobs: new Set(), xhrs: new Set(), projectEpoch: 0, failedArtwork: new Set(), fieldGesture: false,
            textTimers: new Map(), staleArtwork: new Set(), artworkInFlight: new Set(), artworkJobs: new Map(), clipArtRevisions: new Map(), artworkKeys: new Map(), pendingArtwork: new Map(), textBefore: null,
            stageMode: 'timeline', sourceAssetId: '', sourceProjectId: '', sourceIn: null, sourceOut: null, sourceFrame: 0, sourceRate: '', sourceProject: null, sourcePreview: null, sourceShuttle: null
        };
        instances.set(id, s);
        host.innerHTML = layoutMarkup(s);
        s.q = selector => host.querySelector(selector);
        s.q('[data-preview-quality]').value = s.previewQuality;
        s.app = s.q('.vs-app');
        const savedHeight = Number(readPref(PREF_TIMELINE));
        if (savedHeight >= 120) s.app.style.setProperty('--vs-timeline-h', savedHeight + 'px');
        s.notice = (key, fallback, error) => showNotice(s, key, fallback, error);
        s.media = window.VideoStudioMedia.create({ onUpdate: () => scheduleMediaRefresh(s), rate: () => rateOf(s) });
        s.timelineOptions = timelineOptions(s);
        s.detachTimeline = T().attach(s.q('[data-timeline]'), s.timelineOptions);
        s.preview = window.VideoStudioPreview.mount(s.q('[data-preview]'), () => s.project, () => s.frame, frame => {
            s.frame = frame;
            const current = I().formatTime(frame, s.project);
            if (s.q('[data-time]')) s.q('[data-time]').textContent = current;
            T().setPlayhead(s.q('[data-timeline]'), s, s.preview && s.preview.isPlaying());
            updateTransform(s);
        }, playing => {
            if (!playing && s.shuttle) s.shuttle.reset();
            const button = s.q('[data-action="play"]');
            if (button) { button.innerHTML = icon(playing ? 'pause' : 'play', 20); button.setAttribute('aria-label', tr(s, playing ? 'pause' : 'play', playing ? 'Pause' : 'Play')); button.classList.toggle('is-playing', playing); }
        }, () => previewScale(s));
        // The JKL shuttle drives the preview; the preview reports its play state back (see the playing callback).
        s.shuttle = P.createShuttle({ getFrame: () => s.frame, seek: frame => s.preview.seek(frame), play: () => s.preview.play(), pause: () => s.preview.pause(), setRate: rate => s.preview.setRate(rate), maxFrame: () => maxFrames(s) });
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
            // Folders in the media bin move the item; the timeline and the preview copy it into the edit.
            event.dataTransfer.effectAllowed = 'copyMove';
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
        wireMediaBin(s);
        s.q('[data-file-input]').addEventListener('change', async event => {
            await importFiles(s, Array.from(event.target.files || []));
            event.target.value = '';
        }, { signal });
        wireResize(s);
        wireTransform(s);
        P.wireSource(s);
        if (typeof MutationObserver === 'function') {
            s.modalObserver = new MutationObserver(() => P.syncInert(s));
            [s.q('[data-modal-host]'), s.q('[data-conflict-host]')].forEach(node => s.modalObserver.observe(node, { childList: true }));
        }
        // The preview stage is hidden in source mode and reports nothing then, so the app box is watched too: the drawer
        // logic below depends on its width, whichever stage shows.
        if (typeof ResizeObserver === 'function') { s.resizeObserver = new ResizeObserver(() => {
            updateTransform(s); syncDrawer(s);
            // Widening the window turns the drawer back into a column; drop its modal state.
            if (s.app.classList.contains('vs-show-inspector') && s.app.getBoundingClientRect().width > 920) setInspectorOpen(s, false, false);
        }); s.resizeObserver.observe(s.q('.vs-preview-stage')); s.resizeObserver.observe(s.app); }
        s.keyHandler = event => handleKeys(s, event);
        window.addEventListener('keydown', s.keyHandler, true);
        window.addEventListener('keyup', event => { if (s.shuttle) s.shuttle.release(String(event.key || '').toLowerCase()); }, { signal });
        window.addEventListener('blur', () => { if (s.shuttle) s.shuttle.release('k'); }, { signal });
        if (typeof ctx.setWindowBeforeClose === 'function') ctx.setWindowBeforeClose(id, async () => {
            if (s.readonly) return true;
            await P.flushArtwork(s);
            while (s.dirty && !s.disposed) {
                const saved = await saveProject(s);
                if (!saved) return false;
            }
            return !s.dirty;
        });
        if (ctx.setWindowMenus) ctx.setWindowMenus(id, [{
            label: tr(s, 'project', 'Project'), items: [
                { label: tr(s, 'newProject', 'New project'), action: () => createProjectPrompt(s) },
                { label: tr(s, 'projectSettings', 'Project settings…'), action: () => openProjectSettings(s) },
                { label: tr(s, 'allProjects', 'All projects…'), action: () => openProjectManager(s) },
                { label: tr(s, 'save', 'Save'), action: () => saveProject(s) },
                { label: tr(s, 'export', 'Export'), action: () => openExport(s) }
            ]
        }, {
            label: tr(s, 'edit', 'Edit'), items: [
                { label: tr(s, 'undo', 'Undo'), action: () => undo(s, false) },
                { label: tr(s, 'redo', 'Redo'), action: () => undo(s, true) },
                { label: tr(s, 'cut', 'Cut'), action: () => P.cutSelection(s) },
                { label: tr(s, 'copyClips', 'Copy'), action: () => P.copySelection(s) },
                { label: tr(s, 'paste', 'Paste'), action: () => P.pasteClipboard(s, 'overwrite') },
                { label: tr(s, 'selectAll', 'Select all'), action: () => P.selectAll(s) },
                { label: tr(s, 'split', 'Split'), action: () => P.splitSelection(s) },
                { label: tr(s, 'delete', 'Delete'), action: () => P.deleteSelection(s, false) },
                { label: tr(s, 'rippleDelete', 'Ripple delete'), action: () => P.deleteSelection(s, true) }
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
        s.jobsTimer = window.setInterval(() => { if (!s.disposed && s.projectId) loadJobs(s).catch(() => {}); }, 2500);
    }
    function disableWriteControls(s) {
        s.app.querySelectorAll(['new-project', 'save', 'export', 'upload', 'browse', 'add-media', 'add-title'].map(action => `[data-action="${action}"]`).join(',') + ',[data-sticker],[data-place-asset],[data-canvas]').forEach(control => { control.disabled = true; });
    }
    // The project manager asks for storage sizes (`{ sizes: true }`); the popover keeps the cheap list.
    async function loadProjects(s, options = {}) {
        const response = await request(API + '/projects' + (options.sizes ? '?sizes=1' : ''), { signal: s.abort.signal });
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
            // Another project opens at the top of its media bin; reloading the same project keeps the open folder.
            if (s.projectId !== id) { s.mediaFolder = ''; s.selectedAssetId = ''; }
            s.projectId = id;
            s.etag = response.etag;
            s.desktopPath = response.body.desktop_path || '';
            s.project = hydrateProject(s, response.body.project || response.body);
            s.savedProject = canonicalProject(s.project);
            s.selectedClipId = '';
            s.frame = 0;
            s.soloIds = new Set(); s.renamingTrackId = '';
            s.history = []; s.redo = []; s.dirty = false; s.textBefore = null; P.cancelTextApplies(s); s.artworkRevision++;
            if (s.preview) s.preview.seek(0);
            const draftConflict = await recoverDraft(s, id, epoch);
            if (s.disposed || epoch !== s.projectEpoch || id !== s.projectId) return false;
            ensureTracks(s);
            await loadJobs(s).catch(() => {});
            if (s.disposed || epoch !== s.projectEpoch || id !== s.projectId) return false;
            renderUI(s);
            // The recovered draft uses media removed elsewhere: the choices open over the recovered project.
            if (draftConflict && s.conflict) P.showConflict(s).catch(() => {});
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
    async function createProject(s, name, settings) {
        if (s.readonly) return showNotice(s, 'readonly', 'Read-only mode prevents editing.', true);
        const epoch = s.projectEpoch = (s.projectEpoch || 0) + 1;
        const result = await request(API + '/projects', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(Object.assign({ name: name || tr(s, 'untitled', 'Untitled project') }, settings || {})) });
        if (s.disposed || epoch !== s.projectEpoch) return false;
        const wrapper = result.body;
        s.projectId = wrapper.id || wrapper.project_id || wrapper.project && wrapper.project.id;
        s.etag = result.etag;
        s.desktopPath = wrapper.desktop_path || '';
        s.project = hydrateProject(s, wrapper.project || Object.assign(defaultProject(name, s), settings || {}));
        if (!s.project.tracks || !s.project.tracks.length) s.project.tracks = defaultTracks(s);
        s.project.name = name || s.project.name;
        s.savedProject = canonicalProject(s.project);
        s.dirty = true;
        s.history = []; s.redo = []; s.selectedClipId = ''; s.mediaFolder = ''; s.selectedAssetId = ''; s.frame = 0; s.jobs = []; s.textBefore = null; P.cancelTextApplies(s); s.artworkRevision++; s.soloIds = new Set(); s.renamingTrackId = '';
        await saveProject(s);
        await loadProjects(s);
        renderUI(s);
        return true;
    }
    async function createProjectPrompt(s) {
        const epoch = s.projectEpoch || 0;
        closePopovers(s);
        if (s.disposed || epoch !== (s.projectEpoch || 0)) return;
        await openNewProject(s);
    }
    async function switchProject(s, targetId) {
        if (!targetId || targetId === s.projectId || s.timelineDragging) return;
        closePopovers(s);
        await P.flushArtwork(s);
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
            const state = s.saving ? 'saving' : s.conflict ? 'conflict' : s.saveError ? 'error' : s.dirty ? 'dirty' : 'saved';
            chip.className = 'vs-save-chip is-' + (state === 'conflict' ? 'error' : state);
            const label = state === 'saving' ? tr(s, 'saving', 'Saving…') : state === 'conflict' ? tr(s, 'saveConflict', 'Not saved: changed elsewhere') : state === 'error' ? tr(s, 'saveFailed', 'Save failed') : state === 'dirty' ? tr(s, 'unsaved', 'Unsaved changes') : tr(s, 'saved', 'All changes saved');
            chip.innerHTML = `${icon(state === 'saving' ? 'spinner' : state === 'error' || state === 'conflict' ? 'alert' : state === 'dirty' ? 'edit' : 'check', 14, 'vs-icon' + (state === 'saving' ? ' vs-spin' : ''))}<span>${esc(s, label)}</span>`;
            chip.title = state === 'conflict' ? tr(s, 'saveConflictHint', 'Autosave is paused. Click to choose which version to keep.') : state === 'error' ? tr(s, 'retrySave', 'Click to try saving again') : state === 'dirty' ? tr(s, 'saveNow', 'Click to save now') : label;
            chip.disabled = s.featureDisabled || !s.project;
        }
        const undo = s.q('[data-action="undo"]'), redo = s.q('[data-action="redo"]');
        if (undo) undo.disabled = !s.history.length || s.readonly || !!s.mediaRemoving;
        if (redo) redo.disabled = !s.redo.length || s.readonly || !!s.mediaRemoving;
        const exportButton = s.q('.vs-export-button');
        if (exportButton) {
            const hasRenderableClip = !!s.project && s.project.tracks.some(track => track.clips.some(clip => s.project.assets.some(asset => asset.id === clip.asset_id)));
            exportButton.disabled = s.readonly || s.featureDisabled || !s.status || !s.status.ffmpeg_ready || !s.projectId || !hasRenderableClip;
            exportButton.title = exportButton.disabled && s.project && !hasRenderableClip ? tr(s, 'exportNeedsClip', 'Add a clip to the timeline before exporting.') : tr(s, 'export', 'Export');
        }
        renderJobsButton(s);
    }
    // Auto keeps the long edge at most 1920 px so 4K projects stay fluid; full, half and quarter are explicit.
    function previewScale(s) {
        const quality = s.previewQuality || 'auto';
        if (quality === 'half') return 0.5;
        if (quality === 'quarter') return 0.25;
        if (quality === 'full' || !s.project) return 1;
        return Math.min(1, 1920 / Math.max(Number(s.project.width) || 1, Number(s.project.height) || 1));
    }
    function renderUI(s) {
        if (s.disposed || !s.project || !s.q('[data-timeline]')) return;
        s.app.classList.toggle('is-readonly', s.readonly);
        s.app.classList.remove('vs-without-project');
        s.q('[data-canvas]').value = `${s.project.width}x${s.project.height}`;
        s.q('.vs-preview-stage').classList.remove('vs-no-project');
        const noProject = s.q('[data-no-project]'); if (noProject) noProject.remove();
        const scale = previewScale(s); s.q('[data-preview]').width = Math.max(2, Math.round(Number(s.project.width || 1280) * scale)); s.q('[data-preview]').height = Math.max(2, Math.round(Number(s.project.height || 720) * scale));
        s.q('.vs-preview-mat').style.aspectRatio = `${Number(s.project.width || 1280)} / ${Number(s.project.height || 720)}`;
        s.q('.vs-preview-mat').style.setProperty('--vs-ar', String(Number(s.project.width || 1280) / Number(s.project.height || 720)));
        renderPreviewEmpty(s);
        s.q('[data-canvas]').disabled = s.readonly;
        renderToolbar(s);
        renderAssets(s);
        renderInspector(s);
        syncInspectorLock(s);
        T().render(s.q('[data-timeline]'), s.timelineOptions);
        P.syncTransport(s);
        renderJobs(s);
        s.q('[data-transport-time]').textContent = '/ ' + I().formatTime(projectDuration(s), s.project);
        if (s.readonly) s.q('.vs-import').setAttribute('aria-disabled', 'true'); else s.q('.vs-import').removeAttribute('aria-disabled');
        s.app.querySelectorAll('[data-action="upload"],[data-action="browse"],[data-action="add-title"],[data-sticker]').forEach(control => { control.disabled = s.readonly; });
        if (s.readonly) {
            s.q('[data-inspector]').querySelectorAll('input,select,textarea,button:not([data-marker-jump])').forEach(control => { control.disabled = true; });
            s.q('[data-file-input]').disabled = true;
        }
        if (P.syncSource) P.syncSource(s);
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
        // Typing in a title field or dragging a slider must not lose the control under the pointer or caret.
        const active = document.activeElement;
        if (selected && s.inspectorClipId === selected.clip.id && host.contains(active) && (active.matches('[data-text-field],[data-text-style]') || active._vsBefore)) { P.updateTextStatus(s); return; }
        s.inspectorClipId = selected ? selected.clip.id : '';
        const focus = focusKey(s);
        I().render(host, {
            tr: (key, fallback) => tr(s, key, fallback), esc: value => esc(s, value), icon, project: s.project,
            locale: P.uiLocale(), selected, asset, assetName: asset ? assetDisplayName(s, asset) : '', transitionInfo, fineOpen: !!s.fineOpen, thumb: asset && asset.kind && !(selected && selected.clip.text) ? s.media.poster(asset) : '', textPending: P.artworkPending(s, s.selectedClipId),
            summary: { durationFrames: projectDuration(s), clipCount: totalClipCount(s.project) }, libraryAsset: P.libraryAssetInfo(s), markers: s.project.markers || [], inOut: P.inOut(s), caps: P.clipCapabilities(s)
        });
        restoreFocus(s, focus);
        P.updateTextStatus(s);
    }
    function syncInspectorLock(s) {
        const selected = T().clipFor(s.project, s.selectedClipId);
        if (s.readonly || selected && selected.track.locked) {
            s.q('[data-inspector]').querySelectorAll('input,select,textarea,button:not([data-marker-jump])').forEach(control => { control.disabled = true; });
        }
    }
    function timelineOptions(s) {
        return {
            state: s, label: (key, fallback) => tr(s, key, fallback || key), esc: value => esc(s, value), icon: (name, size) => icon(name, size),
            clipMedia: (asset, clip, track, pxPerFrame, width) => clipMedia(s, asset, clip, track, pxPerFrame, width),
            assetName: asset => assetDisplayName(s, asset),
            snapExtras: excludeMarkerId => P.snapExtras(s, excludeMarkerId),
            isSelected: id => P.isSelected(s, id),
            selectedIds: () => P.selectedIds(s),
            onSelect: (id, altKey) => {
                P.setSelection(s, id ? P.linkedIds(s, [id], altKey) : [], id);
                if (id && s.app.getBoundingClientRect().width <= 920) setInspectorOpen(s, true, false);
            },
            onToggleSelect: id => P.toggleSelection(s, id),
            onSelectMany: (ids, primary, altKey) => P.setSelection(s, P.linkedIds(s, ids, altKey), primary),
            onFrame: value => { s.preview.seek(value); },
            onScrubStart: () => { if (s.preview.isPlaying()) s.preview.pause(); },
            onPreviewChange: () => { if (s.preview) s.preview.seek(s.frame); },
            // Trim gestures show the edited edge; the playhead returns when the gesture ends.
            onPeek: frame => { if (s.peekReturn == null) s.peekReturn = s.frame; s.preview.seek(frame); },
            onPeekEnd: () => { if (s.peekReturn != null) { const frame = s.peekReturn; s.peekReturn = null; s.preview.seek(frame); } },
            onChange: (label, before) => recordChange(s, label, before),
            onDragEnd: () => {
                P.retryArtwork(s);
                if (s.pendingRefresh) { s.pendingRefresh = false; refreshProject(s).catch(() => {}).finally(() => { if (s.dirty && !s.conflict) queueAutosave(s); }); }
                else if (s.dirty && !s.conflict) queueAutosave(s);
            },
            onInvalid: reason => (reason ? P.showReason(s, reason) : showNotice(s, 'invalidTiming', 'That edit would create invalid timing or an overlap.', true)),
            onAction: (action, id) => timelineAction(s, action, id),
            onCommand: name => P.timelineCommand(s, name),
            onBlade: (trackId, frame, allTracks, unlinked) => P.bladeAt(s, trackId, frame, allTracks, unlinked),
            onMarkerClick: id => P.jumpToMarker(s, id),
            onMarkerEdit: (id, element) => P.openMarkerPopover(s, id, P.pointOf(element)),
            onMarkerMenu: (id, x, y) => P.openMarkerMenu(s, id, x, y),
            onContextMenu: info => (info.clipId ? P.openClipMenu(s, info.clipId, info.x, info.y) : P.openGapMenu(s, info.trackId, info.frame, info.x, info.y)),
            onTrackAction: (id, action, element) => trackAction(s, id, action, element),
            trackState: track => P.trackState(s, track),
            onTrackMenu: (id, x, y) => P.openTrackMenu(s, id, x, y),
            onTrackRename: (id, value) => P.renameTrack(s, id, value),
            onAddTrack: kind => addTrack(s, kind),
            onDropAsset: (assetId, trackId, at, mode, part) => addAssetToTrack(s, assetId, trackId, at, mode, part),
            onReorderTrack: (from, to, side) => reorderTrack(s, from, to, side),
            onSnap: value => writePref(PREF_SNAP, value)
        };
    }
    function addAssetToTrack(s, assetId, trackId, at, mode, part) {
        if (s.readonly || s.timelineDragging || !s.project) return false;
        const asset = s.project.assets.find(item => item.id === assetId);
        if (!asset) return false;
        if (!asset.kind) { showNotice(s, 'mediaPreparing', 'This media is still being prepared. Try again in a moment.', true); return false; }
        if (totalClipCount(s.project) >= MAX_CLIPS) { showNotice(s, 'clipLimit', 'This project reached its clip limit.', true); return false; }
        if (part === 'picture' || part === 'sound') return P.dropSourcePart(s, asset, trackId, at, mode, part);
        const kind = asset.kind === 'image' ? 'overlay' : asset.kind === 'audio' ? 'audio' : 'video';
        let track = s.project.tracks.find(item => item.id === trackId);
        if (!track || track.locked || !T().accepts(track, asset)) track = s.project.tracks.find(item => item.kind === kind && !item.locked);
        if (!track && !T().trackCapacity(s.project, kind)) { showNotice(s, 'trackLimit', 'Unlock a track or remove one before placing this media.', true); return false; }
        const limit = maxFrames(s);
        const duration = Math.max(1, Math.min(Number(asset.kind === 'image' ? Math.round(rateOf(s) * 5) : asset.duration_frames || Math.round(rateOf(s) * 5)), limit));
        // Dropping onto a lane overwrites at the drop point (Ctrl inserts); "+" keeps finding a free spot.
        // Only on the lane the user pointed at: when it does not take this media (or is locked or missing), the
        // clip goes to a substitute track, and overwriting there would destroy content the user never aimed at.
        if ((mode === 'overwrite' || mode === 'insert') && track && track.id === trackId) {
            const start = Math.max(0, Math.round(Number(at) || 0));
            if (start + duration > limit) { showNotice(s, 'projectLimit', 'The project reached its 60-minute limit.', true); return false; }
            const clip = T().makeClip(asset, start, undefined, s.project);
            clip.duration = duration;
            const placements = [{ trackId: track.id, clips: [clip] }];
            const ok = P.applyEdit(s, mode === 'insert' ? 'insert clip' : 'overwrite clip', project => (mode === 'insert' ? window.VideoStudioEdit.insertClips(project, start, placements) : window.VideoStudioEdit.overwrite(project, placements)), () => [clip.id]);
            if (ok) { renderInspector(s); T().revealFrame(s.q('[data-timeline]'), s, start); }
            return ok;
        }
        const requested = track ? freeStart(track, Number.isFinite(at) ? Math.max(0, at) : nextStart(track), duration) : Math.max(0, Number(at) || 0);
        if (requested + duration > limit) { showNotice(s, 'projectLimit', 'The project reached its 60-minute limit.', true); return false; }
        let placed = null;
        const ok = mutate(s, 'add clip', project => {
            const target = track ? project.tracks.find(item => item.id === track.id) : addTrackTo(s, project, kind);
            const clip = T().makeClip(asset, requested, undefined, s.project);
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
    // Inspector buttons and the window menu act on one clip; they run the selection commands on it.
    function timelineAction(s, action, id) {
        if (!s.project || !T().clipFor(s.project, id) || s.readonly || s.timelineDragging) return;
        if (!P.isSelected(s, id)) P.setSelection(s, [id], id);
        if (action === 'delete') P.deleteSelection(s, false);
        else if (action === 'duplicate') P.duplicateSelection(s);
        else if (action === 'split') P.splitSelection(s);
    }
    // Header buttons (video-studio-tracks.js): target, solo, the menu and starting a rename are view settings and never
    // create an undo step or mark the project dirty; mute, hide, lock and delete go through mutate.
    function trackAction(s, id, action, element) { return P.runTrackAction(s, id, action, element); }
    function addTrack(s, kind) {
        if (!T().trackCapacity(s.project, kind)) return;
        mutate(s, 'add track', project => { addTrackTo(s, project, kind); });
    }
    // Same-kind reorder only; `side` is 'above' or 'below' as displayed (arraySideFor maps it to the array).
    function reorderTrack(s, from, to, side) { return P.moveTrack(s, from, to, side); }
    function handleInput(s, event) {
        const input = event.target;
        if (input.matches('[data-search]')) { renderAssets(s); return; }
        if (s.readonly || s.timelineDragging) return;
        if (input.matches('input[type="range"][data-field]')) { liveField(s, input); return; }
        if (input.matches('[data-transition-duration]')) { updateOutput(input, I().formatSeconds(Number(input.value), P.uiLocale(), s.project) + ' s'); return; }
        if (input.matches('[data-text-field="text"],[data-text-style]')) liveText(s, input);
    }
    function handleChange(s, event) {
        const input = event.target;
        if (input.matches('[data-preview-quality]')) { s.previewQuality = input.value; writePref(PREF_QUALITY, input.value); renderUI(s); return; }
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
        const jump = event.target.closest('[data-marker-jump]'); if (jump) { if (P.jumpToMarker) P.jumpToMarker(s, jump.dataset.markerJump); return; }
        const markerEdit = event.target.closest('[data-marker-edit]'); if (markerEdit) { if (P.openMarkerPopover) P.openMarkerPopover(s, markerEdit.dataset.markerEdit, P.pointOf(markerEdit)); return; }
        const projectItem = event.target.closest('[data-project-id]');
        if (projectItem) { switchProject(s, projectItem.dataset.projectId); return; }
        if (P.handleMediaClick(s, event)) return;
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
        if (transition) { const range = s.q('[data-transition-duration]'); applyTransition(s, transition.dataset.transition, range && !range.disabled ? Number(range.value) : Math.round(rateOf(s) / 2)); return; }
        const align = event.target.closest('[data-text-align]');
        if (align) { if (mutateSelected(s, 'text align', clip => { clip.text_style = Object.assign({}, clip.text_style || {}, { alignment: align.dataset.textAlign }); })) P.markTextStale(s, s.selectedClipId); return; }
        const toggle = event.target.closest('[data-text-toggle]');
        if (toggle) { const key = toggle.dataset.textToggle; if (mutateSelected(s, 'text style', clip => { clip.text_style = Object.assign({}, clip.text_style || {}, { [key]: !(clip.text_style && clip.text_style[key]) }); })) P.markTextStale(s, s.selectedClipId); return; }
        const button = event.target.closest('[data-action]'); if (!button || button.disabled) return;
        const action = button.dataset.action;
        if (s.readonly && !READ_ONLY_SAFE.has(action)) return showNotice(s, 'readonly', 'Read-only mode prevents editing.', true);
        if (action === 'new-project') createProjectPrompt(s);
        else if (action === 'project-menu') togglePopover(s, 'project');
        else if (action === 'jobs') togglePopover(s, 'jobs');
        else if (action === 'shortcuts') togglePopover(s, 'shortcuts');
        else if (action === 'rename-project') renameProject(s);
        else if (action === 'project-settings') openProjectSettings(s);
        else if (action === 'all-projects') openProjectManager(s);
        else if (action === 'dismiss-notice') clearNotice(s);
        else if (action === 'toggle-inspector') setInspectorOpen(s, !s.app.classList.contains('vs-show-inspector'), true);
        else if (action === 'close-inspector') { setInspectorOpen(s, false, false); s.q('.vs-inspector-toggle')?.focus(); }
        else if (action === 'upload' || action === 'add-media') s.q('[data-file-input]').click();
        else if (action === 'browse') importDesktopFile(s);
        else if (action === 'save') saveProject(s);
        else if (action === 'undo') undo(s, false);
        else if (action === 'redo') undo(s, true);
        else if (action === 'play') P.togglePlay(s);
        else if (action === 'step-back') s.preview.pause(), s.preview.seek(s.frame - 1);
        else if (action === 'step-forward') s.preview.pause(), s.preview.seek(s.frame + 1);
        else if (action === 'seek-start') s.preview.pause(), s.preview.seek(0), T().revealFrame(s.q('[data-timeline]'), s, 0);
        else if (action === 'seek-end') { const end = projectDuration(s); s.preview.pause(); s.preview.seek(end); T().revealFrame(s.q('[data-timeline]'), s, end); }
        else if (action === 'fullscreen') { const mat = s.q('.vs-preview-mat'); if (document.fullscreenElement) document.exitFullscreen().catch(() => {}); else if (mat && mat.requestFullscreen) mat.requestFullscreen().catch(() => {}); }
        else if (action === 'export') openExport(s);
        else if (action === 'goto-time') P.openGoTo(s);
        else if (action === 'loop') P.toggleLoop(s);
        else if (action === 'mark-in') P.setInOut(s, 'in');
        else if (action === 'mark-out') P.setInOut(s, 'out');
        else if (action === 'clear-inout') P.clearInOut(s);
        else if (action === 'add-marker') { if (P.addMarkerAtPlayhead) P.addMarkerAtPlayhead(s); }
        else if (action === 'add-title') addTitle(s, button.dataset.preset || 'title');
        else if (action === 'apply-text') { if (s.selectedClipId) applyTextArtwork(s, s.selectedClipId); }
        else if (action === 'open-ai') openAI(s);
        else if (action === 'toggle-fine') { s.fineOpen = !s.fineOpen; renderInspector(s); syncInspectorLock(s); s.q('[data-action="toggle-fine"]')?.focus(); }
        else if (action === 'fill-frame') mutateSelected(s, 'fill', clip => { Object.assign(clip, { x: 0, y: 0, width: 1, height: 1 }); });
        else if (action === 'clip-split' || action === 'clip-duplicate' || action === 'clip-delete') { if (s.selectedClipId) timelineAction(s, action.slice(5), s.selectedClipId); }
        else if (action.startsWith('clip-')) P.clipCommand(s, action.slice(5), button.dataset.value);
    }
    function handleKeys(s, event) {
        const target = event.target, modifier = event.ctrlKey || event.metaKey, undoShortcut = modifier && !event.altKey && event.key.toLowerCase() === 'z';
        if (s.disposed || event.defaultPrevented || !s.project || !target || !s.host.contains(target) || typeof target.closest !== 'function') return;
        if ((event.ctrlKey || event.metaKey || event.altKey) && !undoShortcut && !P.isEditShortcut(s, event) && !P.isNavShortcut(s, event) && !P.isSourceShortcut(s, event)) return;
        if (event.key === 'Escape') {
            // Escape in the conflict dialog is the explicit "Keep editing" choice; it never retries a save.
            const keep = s.q('[data-conflict-host] [data-conflict="keep"]');
            if (keep) { keep.click(); event.preventDefault(); return; }
            const modal = s.q('[data-modal-host] .vs-modal');
            if (modal) { s.q('[data-modal-host]').replaceChildren(); s.app.focus({ preventScroll: true }); event.preventDefault(); return; }
            // An open marker popover is the topmost layer: Escape closes it and leaves the drawer behind it open.
            if (s.markerPop && s.markerPop.el.isConnected) { P.closeMarkerPopover(s, true); event.preventDefault(); return; }
            if (Object.values(POPOVERS).some(([, selector]) => !s.q(selector).hidden)) { closePopovers(s, null, true); event.preventDefault(); return; }
        }
        if (event.key === 'Escape' && s.app.classList.contains('vs-show-inspector')) { setInspectorOpen(s, false, false); s.q('.vs-inspector-toggle')?.focus(); event.preventDefault(); return; }
        // Tab cycles inside an open dialog, then inside an open marker popover (it sits outside the drawer), then inside
        // the narrow inspector drawer.
        const trap = s.q('[data-conflict-host] .vs-modal') || s.q('[data-modal-host] .vs-modal') || (s.markerPop && s.markerPop.el.isConnected ? s.markerPop.el : null) || (s.app.classList.contains('vs-show-inspector') && s.app.getBoundingClientRect().width <= 920 ? s.q('.vs-inspector') : null);
        if (event.key === 'Tab' && trap) {
            const items = Array.from(trap.querySelectorAll('button:not(:disabled),input:not(:disabled),select:not(:disabled),textarea:not(:disabled),a[href]')).filter(item => item.getClientRects().length);
            if (items.length) { const first = items[0], last = items[items.length - 1]; if (event.shiftKey && (document.activeElement === first || !trap.contains(document.activeElement))) { event.preventDefault(); last.focus(); } else if (!event.shiftKey && (document.activeElement === last || !trap.contains(document.activeElement))) { event.preventDefault(); first.focus(); } }
        }
        if (s.q('[data-modal-host] .vs-modal, [data-conflict-host] .vs-modal')) return;
        // The source position slider is an input that takes no text: in source mode it passes everything but its own
        // keys (arrows, Home, End, Page Up/Down stay native), so I, O, the comma, Space and Escape keep working after
        // a scrub.
        const sliderKeys = ['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Home', 'End', 'PageUp', 'PageDown'];
        const sourceSlider = s.stageMode === 'source' && target.matches('[data-source-scrub]') && !sliderKeys.includes(event.key);
        if (!sourceSlider && target.closest('input,textarea,select,[contenteditable="true"]')) return;
        // Escape leaves the source monitor. It sits after the text-field guard (Escape in the library search or a rename
        // stays there) and before the button guard (Escape on a focused source button still returns to the timeline).
        if (event.key === 'Escape' && s.stageMode === 'source' && !s.menu) { P.setStageMode(s, 'timeline'); event.preventDefault(); return; }
        if (undoShortcut) { event.preventDefault(); undo(s, event.shiftKey); return; }
        // A focused timeline clip takes the clip shortcuts; Space and Enter stay with the timeline.
        const focusedClip = target.closest('[data-clip-id]') && !target.closest('button') ? target.closest('[data-clip-id]').dataset.clipId : '';
        if (!focusedClip && target.closest('button,a,summary,[role="button"],[role="link"],[role="tab"],[role="separator"]')) return;
        if (focusedClip && (event.code === 'Space' || event.key === 'Enter')) return;
        if (s.q('[data-modal-host] .vs-modal, [data-conflict-host] .vs-modal')) return;
        // In source mode every key goes to the source monitor; timeline keys do nothing there (undo ran above).
        if (s.stageMode === 'source') { P.handleSourceKeys(s, event); return; }
        if (P.handleEditKeys(s, event, focusedClip)) return;
        if (P.handleNavKeys(s, event)) return;
        if (event.code === 'Space') { event.preventDefault(); P.togglePlay(s); }
        else if (event.key === 'ArrowLeft') { event.preventDefault(); s.preview.pause(); s.preview.seek(s.frame - (event.shiftKey ? Math.round(rateOf(s)) : 1)); }
        else if (event.key === 'ArrowRight') { event.preventDefault(); s.preview.pause(); s.preview.seek(s.frame + (event.shiftKey ? Math.round(rateOf(s)) : 1)); }
        else if (event.key === 'Home') { event.preventDefault(); s.preview.pause(); s.preview.seek(0); T().revealFrame(s.q('[data-timeline]'), s, 0); }
        else if (event.key === 'End') { event.preventDefault(); const end = projectDuration(s); s.preview.pause(); s.preview.seek(end); T().revealFrame(s.q('[data-timeline]'), s, end); }
        else if (event.key === '+' || event.key === '=') { event.preventDefault(); s.q('[data-timeline] [data-action="zoom-in"]')?.click(); }
        else if (event.key === '-') { event.preventDefault(); s.q('[data-timeline] [data-action="zoom-out"]')?.click(); }
        else if (event.key === '?') { event.preventDefault(); togglePopover(s, 'shortcuts'); }
    }

    Object.assign(P, { dispose, layoutMarkup, appRender, initialize, disableWriteControls, loadProjects, loadProject, ensureTracks, createProject, createProjectPrompt, switchProject, renderEmptyProject, populateCanvas, renderToolbar, previewScale, renderUI, renderInspector, syncInspectorLock, timelineOptions, addAssetToTrack, timelineAction, trackAction, addTrack, reorderTrack, handleInput, handleChange, handleClick, handleKeys });

    window.VideoStudioApp = { render: appRender, dispose, instances };
})();
