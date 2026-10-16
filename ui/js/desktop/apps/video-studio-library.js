(function () {
    'use strict';

    // Library rail: media bin (folders, grid and list, sorting, posters), uploads and file drops, AI panel and timeline
    // media layers. The bin's actions (dialogs, menus, removal, drops) live in video-studio-mediabin.js.
    const P = window.VideoStudioParts;
    const { API, PREF_MEDIA_VIEW, PREF_MEDIA_SORT } = P;
    const T = (...args) => P.T(...args);
    const clamp = (...args) => P.clamp(...args);
    const esc = (...args) => P.esc(...args);
    const icon = (...args) => P.icon(...args);
    const idempotencyKey = (...args) => P.idempotencyKey(...args);
    const pollJob = (...args) => P.pollJob(...args);
    const refreshProject = (...args) => P.refreshProject(...args);
    const renderInspector = (...args) => P.renderInspector(...args);
    const renderJobs = (...args) => P.renderJobs(...args);
    const request = (...args) => P.request(...args);
    const selectedClip = (...args) => P.selectedClip(...args);
    const showNotice = (...args) => P.showNotice(...args);
    const tr = (...args) => P.tr(...args);
    const F = () => window.VideoStudioFolders;

    // List columns of the media bin; every column sorts both views (aurago.videoStudio.mediaSort = 'name:asc' style).
    const SORT_KEYS = ['name', 'duration', 'resolution', 'frame_rate', 'codec', 'size', 'usage'];
    const LIST_COLUMNS = SORT_KEYS;
    const COLUMN_KEYS = { name: 'colName', duration: 'colDuration', resolution: 'colResolution', frame_rate: 'colFrameRate', codec: 'colCodec', size: 'colSize', usage: 'colUsage' };
    const COLUMN_FALLBACKS = { name: 'Name', duration: 'Duration', resolution: 'Resolution', frame_rate: 'Frame rate', codec: 'Codec', size: 'Size', usage: 'Used' };

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
    // Server poster of a visual asset; the client thumbnail, once ready, takes precedence (see thumbFor).
    function posterURL(s, asset) {
        if (!asset || !asset.kind || !asset.has_poster || !s.projectId) return '';
        return API + '/projects/' + encodeURIComponent(s.projectId) + '/media/' + encodeURIComponent(asset.id) + '/poster.jpg';
    }
    function thumbFor(s, asset) { return s.media.poster(asset) || posterURL(s, asset); }
    function formatBytes(bytes, locale) {
        const units = ['B', 'KB', 'MB', 'GB'];
        let size = Number(bytes) || 0, index = 0;
        if (size <= 0) return '';
        while (size >= 1024 && index < units.length - 1) { size /= 1024; index++; }
        return size.toLocaleString(locale || 'en', { maximumFractionDigits: index ? 1 : 0 }) + ' ' + units[index];
    }
    function frameRateLabel(asset) {
        const [num, den] = String(asset.source_fps || '').split('/').map(Number);
        return num > 0 && den > 0 ? (Math.round(num / den * 100) / 100).toLocaleString(P.uiLocale(), { maximumFractionDigits: 2 }) : '';
    }
    function resolutionLabel(asset) { return asset.width && asset.height ? asset.width + '×' + asset.height : ''; }
    function readMediaPrefs() {
        const [key, dir] = String(P.readPref(PREF_MEDIA_SORT) || '').split(':');
        return { mediaView: P.readPref(PREF_MEDIA_VIEW) === 'list' ? 'list' : 'grid', mediaSort: SORT_KEYS.includes(key) ? key : 'name', mediaSortDir: dir === 'desc' ? 'desc' : 'asc' };
    }
    function writeMediaPrefs(s) {
        P.writePref(PREF_MEDIA_VIEW, s.mediaView === 'list' ? 'list' : 'grid');
        P.writePref(PREF_MEDIA_SORT, s.mediaSort + ':' + (s.mediaSortDir === 'desc' ? 'desc' : 'asc'));
    }
    // A selector that finds the focused control again after its host was redrawn ('' when there is none).
    function focusSelector(active) {
        if (!active || !active.dataset) return '';
        if (active.dataset.placeAsset) return `[data-place-asset="${CSS.escape(active.dataset.placeAsset)}"]`;
        if (active.dataset.assetId) return `[data-asset-id="${CSS.escape(active.dataset.assetId)}"]`;
        if (active.dataset.folder !== undefined) return `[data-folder="${CSS.escape(active.dataset.folder)}"]`;
        if (active.dataset.mediaSort) return `[data-media-sort="${CSS.escape(active.dataset.mediaSort)}"]`;
        if (active.dataset.view) return `[data-view="${CSS.escape(active.dataset.view)}"]`;
        if (active.dataset.action) return `[data-action="${CSS.escape(active.dataset.action)}"]`;
        return '';
    }
    function redraw(host, markup) {
        const focus = focusSelector(host.contains(document.activeElement) ? document.activeElement : null);
        host.innerHTML = markup;
        if (focus) host.querySelector(focus)?.focus({ preventScroll: true });
    }
    // One item has its own wording ("1 item"); every other count uses the plural key.
    function countLabel(s, count) { return count === 1 ? tr(s, 'assetCountOne', '1 item') : tr(s, 'assetCount', '{{count}} items').replace('{{count}}', String(count)); }
    // The media bar: the breadcrumb of the open folder, the view switch and the folder and cleanup actions.
    function renderMediaBar(s) {
        const host = s.q('[data-media-bar]');
        if (!host) return;
        if (!s.project) { host.replaceChildren(); return; }
        const root = tr(s, 'mediaFolderRoot', 'Media');
        const trail = s.mediaFolder ? s.mediaFolder.split('/') : [];
        const crumbs = trail.map((part, index) => {
            const path = trail.slice(0, index + 1).join('/');
            return `<span aria-hidden="true">/</span><button type="button" data-folder="${esc(s, path)}" ${index === trail.length - 1 ? 'aria-current="page"' : ''}>${esc(s, part)}</button>`;
        }).join('');
        const viewButton = view => {
            const label = tr(s, view === 'list' ? 'viewList' : 'viewGrid', view === 'list' ? 'List' : 'Grid');
            return `<button type="button" data-action="media-view" data-view="${view}" aria-pressed="${(s.mediaView === 'list') === (view === 'list')}">${esc(s, label)}</button>`;
        };
        const action = (name, iconName, key, fallback) => `<button type="button" class="vs-icon-button" data-action="${name}" title="${esc(s, tr(s, key, fallback))}" aria-label="${esc(s, tr(s, key, fallback))}" ${s.readonly ? 'disabled' : ''}>${icon(iconName, 15)}</button>`;
        redraw(host, `<nav class="vs-breadcrumb" aria-label="${esc(s, root)}"><button type="button" data-folder="" ${trail.length ? '' : 'aria-current="page"'}>${esc(s, root)}</button>${crumbs}</nav><div class="vs-media-bar-actions"><div class="vs-segmented" role="group" aria-label="${esc(s, tr(s, 'mediaView', 'View'))}">${viewButton('grid')}${viewButton('list')}</div>${action('new-folder', 'folder', 'newFolder', 'New folder')}${action('remove-unused', 'trash', 'removeUnused', 'Remove unused media')}</div>`);
    }
    function uploadCard(s, key, item) {
        return `<article class="vs-asset is-uploading${item.failed ? ' is-failed' : ''}" data-upload-key="${esc(s, key)}"><span class="vs-asset-thumb">${icon(item.failed ? 'alert' : 'upload', 22)}<span class="vs-asset-progress"><i style="width:${Math.round(item.progress * 100)}%"></i></span></span><span class="vs-asset-name" title="${esc(s, item.name)}">${esc(s, item.name)}</span><span class="vs-asset-meta">${esc(s, uploadLabel(s, item))}</span></article>`;
    }
    function folderTile(s, path) {
        const count = F().assetsIn(s.project, path, true).filter(asset => !window.VideoStudioMedia.isArtwork(asset)).length;
        return `<button type="button" class="vs-folder-tile" data-folder="${esc(s, path)}" title="${esc(s, path)}"><span class="vs-folder-icon">${icon('folder', 22)}</span><span class="vs-folder-name">${esc(s, F().nameOf(path))}</span><span class="vs-asset-meta">${esc(s, countLabel(s, count))}</span></button>`;
    }
    function folderRow(s, path) {
        const count = F().assetsIn(s.project, path, true).filter(asset => !window.VideoStudioMedia.isArtwork(asset)).length;
        return `<button type="button" class="vs-folder-row" data-folder="${esc(s, path)}" title="${esc(s, path)}"><span class="vs-folder-icon">${icon('folder', 16)}</span><span class="vs-folder-name">${esc(s, F().nameOf(path))}</span><span class="vs-asset-meta">${esc(s, countLabel(s, count))}</span></button>`;
    }
    function addButton(s, asset, ready) {
        const label = tr(s, 'addToTimeline', 'Add at the playhead');
        return `<button type="button" class="vs-asset-add" data-place-asset="${esc(s, asset.id)}" title="${esc(s, label)}" aria-label="${esc(s, label)}: ${esc(s, asset.name)}" ${s.readonly || !ready ? 'disabled' : ''}>${icon('plus', 16)}</button>`;
    }
    function kindIconOf(asset) { return asset.kind === 'video' ? 'video' : asset.kind === 'audio' ? 'audio' : asset.kind === 'image' ? 'image' : 'spinner'; }
    function durationLabel(s, asset) { return asset.kind === 'video' || asset.kind === 'audio' ? T().shortClock(asset.duration_frames || 0, s.project) : ''; }
    function gridCard(s, asset, used) {
        const ready = !!asset.kind;
        const poster = ready ? thumbFor(s, asset) : '';
        const kindIcon = kindIconOf(asset);
        const duration = durationLabel(s, asset);
        const style = poster ? ` style="background-image:url('${esc(s, poster)}')"` : '';
        const selected = s.selectedAssetId === asset.id ? ' is-selected' : '';
        return `<article class="vs-asset vs-asset-${esc(s, asset.kind || 'pending')}${used ? ' is-used' : ''}${selected}" draggable="${!s.readonly && ready}" data-asset-id="${esc(s, asset.id)}" tabindex="0" title="${esc(s, asset.name)}"><span class="vs-asset-thumb"${style}>${poster ? '' : icon(kindIcon, 22, 'vs-icon' + (ready ? '' : ' vs-spin'))}${duration ? `<span class="vs-asset-badge">${esc(s, duration)}</span>` : ''}${used ? `<span class="vs-asset-used" title="${esc(s, tr(s, 'inTimeline', 'In the timeline'))}">${icon('check', 11)}</span>` : ''}</span><span class="vs-asset-name">${esc(s, asset.name)}</span><span class="vs-asset-meta">${icon(kindIcon === 'spinner' ? 'clock' : kindIcon, 12)}${esc(s, assetKindLabel(s, asset))}</span>${addButton(s, asset, ready)}</article>`;
    }
    function listRow(s, asset, used, usage) {
        const ready = !!asset.kind;
        const poster = ready ? thumbFor(s, asset) : '';
        const style = poster ? ` style="background-image:url('${esc(s, poster)}')"` : '';
        const kindIcon = kindIconOf(asset);
        const cells = [asset.name, durationLabel(s, asset), resolutionLabel(asset), frameRateLabel(asset), asset.codec || '', formatBytes(asset.size_bytes, P.uiLocale()), String(usage.get(asset.id) || 0)];
        const selected = s.selectedAssetId === asset.id ? ' is-selected' : '';
        return `<article class="vs-asset vs-asset-row vs-asset-${esc(s, asset.kind || 'pending')}${used ? ' is-used' : ''}${selected}" draggable="${!s.readonly && ready}" data-asset-id="${esc(s, asset.id)}" tabindex="0" title="${esc(s, asset.name)}"><span class="vs-asset-thumb vs-asset-thumb-small"${style}>${poster ? '' : icon(kindIcon, 14, 'vs-icon' + (ready ? '' : ' vs-spin'))}</span>${cells.map((value, index) => `<span class="vs-asset-cell${index === 0 ? ' vs-asset-cell-name' : ''}" data-column="${LIST_COLUMNS[index]}">${esc(s, value)}</span>`).join('')}${addButton(s, asset, ready)}</article>`;
    }
    // The list header: sort buttons with aria-sort, aligned with the row cells (spacers for the thumbnail and add button).
    function listHeader(s) {
        const headers = SORT_KEYS.map(key => {
            const active = s.mediaSort === key;
            const label = tr(s, COLUMN_KEYS[key], COLUMN_FALLBACKS[key]);
            const sort = active ? (s.mediaSortDir === 'desc' ? 'descending' : 'ascending') : 'none';
            return `<button type="button" role="columnheader" data-media-sort="${key}" aria-sort="${sort}" class="${active ? 'is-sorted' : ''}">${esc(s, label)}${active ? icon('chevronDown', 12) : ''}</button>`;
        }).join('');
        return `<div class="vs-asset-head" role="row"><span></span>${headers}<span></span></div>`;
    }
    // The open folder, or its closest ancestor that still exists (an undo or a server refresh may remove it).
    function existingFolder(s) {
        const paths = new Set(F().folderPaths(s.project));
        let folder = s.mediaFolder || '';
        while (folder && !paths.has(folder)) folder = F().parentOf(folder);
        return folder;
    }
    // The bin shows the open folder (or every match of a search) in the chosen view and order.
    function renderAssets(s) {
        const root = s.q('[data-assets]');
        if (!root) return;
        renderFilters(s);
        if (s.project) s.mediaFolder = existingFolder(s);
        renderMediaBar(s);
        if (!s.project) { root.innerHTML = `<div class="vs-empty-media">${icon('project', 28)}<strong>${esc(s, tr(s, 'noProject', 'No project'))}</strong><small>${esc(s, tr(s, 'createProjectFirst', 'Create a project before importing media.'))}</small></div>`; s.q('[data-asset-count]').textContent = ''; return; }
        const folders = F();
        const search = (s.q('[data-search]')?.value || '').toLowerCase();
        const usage = folders.usageCounts(s.project);
        const matches = s.project.assets.filter(asset => !window.VideoStudioMedia.isArtwork(asset) && (s.filter === 'all' || asset.kind === s.filter) && (!search || String(asset.name).toLowerCase().includes(search)));
        const scoped = search ? matches : matches.filter(asset => (asset.folder || '') === s.mediaFolder);
        const visible = folders.sortAssets(scoped, s.mediaSort, s.mediaSortDir, { usage, fps: window.VideoStudioTime.rate(s.project) });
        const tiles = search ? [] : folders.childFolders(s.project, s.mediaFolder);
        const uploads = Array.from(s.uploads.entries()).filter(([, item]) => item.projectId === s.projectId).map(([key, item]) => uploadCard(s, key, item)).join('');
        const list = s.mediaView === 'list';
        root.dataset.view = list ? 'list' : 'grid';
        let markup;
        if (!uploads && !visible.length && !tiles.length) {
            const searching = search || s.filter !== 'all';
            markup = `<div class="vs-empty-media">${icon(searching ? 'search' : 'upload', 28)}<strong>${esc(s, searching ? tr(s, 'noMatches', 'Nothing matches') : tr(s, 'emptyMedia', 'Your media will appear here'))}</strong><small>${esc(s, searching ? tr(s, 'noMatchesHint', 'Try another search or filter.') : tr(s, 'emptyMediaDrop', 'Upload clips, music or pictures, or drop files into this window.'))}</small></div>`;
        } else if (list) markup = `${listHeader(s)}${uploads}${tiles.map(path => folderRow(s, path)).join('')}${visible.map(asset => listRow(s, asset, usage.has(asset.id), usage)).join('')}`;
        else markup = `${uploads}${tiles.map(path => folderTile(s, path)).join('')}${visible.map(asset => gridCard(s, asset, usage.has(asset.id))).join('')}`;
        redraw(root, markup);
        const total = s.project.assets.filter(asset => !window.VideoStudioMedia.isArtwork(asset)).length;
        s.q('[data-asset-count]').textContent = countLabel(s, total);
    }
    function uploadLabel(s, item) {
        return item.failed ? tr(s, 'importFailedShort', 'Import failed') : item.progress < 1 ? tr(s, 'uploading', 'Uploading…') + ' ' + Math.round(item.progress * 100) + ' %' : tr(s, 'preparing', 'Preparing…');
    }
    // Progress events update the card in place so keyboard focus in the grid survives an upload.
    function updateUploadCard(s, key, item) {
        const card = s.q(`[data-upload-key="${CSS.escape(key)}"]`);
        if (!card) { renderAssets(s); return; }
        card.querySelector('.vs-asset-progress i').style.width = Math.round(item.progress * 100) + '%';
        card.querySelector('.vs-asset-meta').textContent = uploadLabel(s, item);
    }
    function scheduleMediaRefresh(s) {
        if (s.mediaFrame || s.disposed) return;
        s.mediaFrame = requestAnimationFrame(() => {
            s.mediaFrame = 0;
            if (s.disposed || !s.project) return;
            renderAssets(s);
            if (P.renderSource) P.renderSource(s);
            if (!s.timelineDragging && !s.transformDrag) T().render(s.q('[data-timeline]'), s.timelineOptions);
            const thumb = s.q('.vs-insp-thumb'), selected = selectedClip(s);
            const asset = selected && s.project.assets.find(item => item.id === selected.clip.asset_id);
            if (thumb && !thumb.style.backgroundImage && asset && asset.kind === 'video' && !s.q('[data-inspector]').contains(document.activeElement)) renderInspector(s);
        });
    }
    function clipMedia(s, asset, clip, track, pxPerFrame, width) {
        if (clip.text || !asset.kind) return '';
        const fps = window.VideoStudioTime.rate(s.project);
        const speed = Number(clip.speed) > 0 ? Number(clip.speed) : 1;
        if (track.kind === 'audio' || asset.kind === 'audio') {
            const data = s.media.peaks(asset);
            if (!data || !data.peaks.length) return '';
            const points = Math.max(2, Math.min(600, Math.round(width / 2)));
            const key = [asset.id, clip.offset, clip.duration, points, clip.volume, fps, speed].join('|');
            s.waveCache = s.waveCache || new Map();
            let path = s.waveCache.get(key);
            if (!path) {
                const top = [], bottom = [];
                for (let k = 0; k < points; k++) {
                    const frame = clip.offset + k / (points - 1) * clip.duration * speed;
                    const value = Math.min(1, (data.peaks[Math.min(data.peaks.length - 1, Math.floor(frame / fps * data.perSecond))] || 0) * Math.max(0.15, Math.min(1.6, Number(clip.volume == null ? 1 : clip.volume))));
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
            const thumb = window.VideoStudioMedia.pickThumb(thumbs, (clip.offset + (i * tile + tile / 2) / pxPerFrame * speed) / fps);
            if (thumb) tiles += `<i style="left:${i * tile}px;width:${tile}px;background-image:url('${esc(s, thumb.url)}')"></i>`;
        }
        return `<span class="vs-filmstrip">${tiles}</span>`;
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
    // Every file of one drop or pick goes into the folder that was open when it started.
    async function importFiles(s, files) {
        const projectId = s.projectId, epoch = s.projectEpoch, folder = s.mediaFolder || '';
        if (!files.length) return;
        if (s.readonly || !projectId) { showNotice(s, s.readonly ? 'readonly' : 'createProjectFirst', s.readonly ? 'Read-only mode prevents editing.' : 'Create a project before importing media.', true); return; }
        setTab(s, 'media');
        for (const file of files) {
            if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) break;
            await uploadFile(s, file, projectId, epoch, folder);
        }
    }
    async function uploadFile(s, file, expectedProjectId, expectedEpoch, expectedFolder) {
        const projectId = expectedProjectId || s.projectId, epoch = expectedEpoch == null ? s.projectEpoch : expectedEpoch;
        const folder = expectedFolder == null ? s.mediaFolder || '' : expectedFolder;
        if (!file || s.readonly || !projectId || epoch !== s.projectEpoch || projectId !== s.projectId) return;
        const allowed = /^(video\/|audio\/|image\/(png|webp)$)/i.test(file.type) || /\.(mp4|mov|webm|m4v|mkv|mp3|wav|ogg|m4a|aac|flac|png|webp)$/i.test(file.name);
        if (!allowed) return showNotice(s, 'unsupportedFile', 'Choose a video, audio, PNG or WebP file.', true);
        const max = Number(s.status.limits && s.status.limits.max_asset_size_bytes || 256 * 1024 * 1024);
        if (file.size > max) return showNotice(s, 'fileTooLarge', 'This file is larger than the server limit.', true);
        const key = idempotencyKey(), entry = { name: file.name, progress: 0, failed: false, projectId };
        s.uploads.set(key, entry);
        renderAssets(s);
        // The upload reader streams: the folder field must precede the file part.
        const form = new FormData();
        if (folder) form.append('folder', folder);
        form.append('file', file, file.name);
        const current = () => !s.disposed && epoch === s.projectEpoch && projectId === s.projectId;
        try {
            const response = await upload(s, API + '/projects/' + encodeURIComponent(projectId) + '/media', form, progress => { entry.progress = progress; if (current()) updateUploadCard(s, key, entry); });
            entry.progress = 1;
            if (!current()) return;
            updateUploadCard(s, key, entry);
            const job = response.body.job;
            if (job) { s.jobs.unshift(job); renderJobs(s); await pollJob(s, job.id, projectId, epoch); }
            else await refreshProject(s);
            if (current()) showNotice(s, 'importReady', 'Media added. Drag it to the timeline or press +.', false);
        } catch (error) {
            if (current()) {
                const code = error.body && error.body.code;
                showNotice(s, code === 'asset_size_limit' ? 'fileTooLarge' : code === 'project_size_limit' ? 'projectStorageFull' : 'importFailed', 'Could not import this media.', true);
            }
        } finally {
            s.uploads.delete(key);
            if (!s.disposed) renderAssets(s);
        }
    }
    async function importDesktopFile(s) {
        if (s.readonly || !s.projectId || !s.ctx.openFileDialog) return;
        const projectId = s.projectId, epoch = s.projectEpoch, folder = s.mediaFolder || '';
        const result = await s.ctx.openFileDialog({
            title: tr(s, 'browse', 'Browse Desktop files'), initialPath: 'Documents',
            filters: [{ label: tr(s, 'videoAudioImages', 'Video, audio and images'), extensions: ['.mp4', '.mov', '.webm', '.m4v', '.mkv', '.mp3', '.wav', '.ogg', '.m4a', '.aac', '.flac', '.png', '.webp'] }]
        });
        if (!result || result.canceled || !result.path || s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
        const key = idempotencyKey(), entry = { name: String(result.path).split('/').pop(), progress: 1, failed: false, projectId };
        s.uploads.set(key, entry); renderAssets(s);
        try {
            const response = await request(API + '/projects/' + encodeURIComponent(projectId) + '/media', { method: 'POST', headers: { 'Content-Type': 'application/json', 'Idempotency-Key': idempotencyKey() }, body: JSON.stringify({ source_path: result.path, folder }), signal: s.abort.signal });
            if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
            if (response.body.job) { s.jobs.unshift(response.body.job); await pollJob(s, response.body.job.id, projectId, epoch); }
            else await refreshProject(s);
        } catch (_) { if (!s.disposed && epoch === s.projectEpoch && projectId === s.projectId) showNotice(s, 'importFailed', 'Could not import this Desktop file.', true); }
        finally { s.uploads.delete(key); if (!s.disposed) renderAssets(s); }
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
    function renderAIPanel(s) {
        const host = s.q('[data-ai-panel]');
        if (!host) return;
        const generation = s.status && s.status.generation;
        const ready = generation && generation.enabled && generation.configured && !generation.budget_blocked;
        const reason = !generation || !generation.enabled || !generation.configured ? tr(s, 'aiNotConfigured', 'AI video generation is not set up. An administrator can enable it under Configuration → Video generation.') : generation.budget_blocked ? tr(s, 'aiBudget', 'The video generation budget is used up for now.') : s.readonly ? tr(s, 'readonly', 'Read-only mode prevents editing.') : '';
        host.innerHTML = `<div class="vs-ai-card"><span class="vs-ai-art" aria-hidden="true">${icon('sparkle', 30)}</span><h3>${esc(s, tr(s, 'generateTitle', 'Generate a clip'))}</h3><p>${esc(s, tr(s, 'aiIntro', 'Describe a scene and the configured provider creates a short clip. It appears in your media library when it is ready.'))}</p>${generation && generation.provider ? `<p class="vs-ai-provider">${icon('info', 13)} ${esc(s, tr(s, 'usingModel', 'Provider'))}: ${esc(s, generation.provider)}${generation.model ? ' · ' + esc(s, generation.model) : ''}</p>` : ''}${reason ? `<p class="vs-hint vs-ai-reason">${esc(s, reason)}</p>` : ''}<button type="button" class="vs-primary" data-action="open-ai" ${ready && !s.readonly ? '' : 'disabled'}>${icon('sparkle', 16)}<span>${esc(s, tr(s, 'generateStart', 'Describe a clip'))}</span></button></div>`;
    }

    Object.assign(P, { upload, renderPreviewEmpty, assetKindLabel, renderFilters, renderAssets, scheduleMediaRefresh, clipMedia, isFileDrag, fileDrag, importFiles, uploadFile, importDesktopFile, setTab, handleTabKeys, renderAIPanel, uploadLabel, updateUploadCard, posterURL, thumbFor, formatBytes, frameRateLabel, resolutionLabel, readMediaPrefs, writeMediaPrefs, renderMediaBar, focusSelector, LIST_COLUMNS, SORT_KEYS });
})();
