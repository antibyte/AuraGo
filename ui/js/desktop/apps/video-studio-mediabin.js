(function () {
    'use strict';

    // Media bin actions: folder dialogs, moving and renaming media, item and folder menus, removal (clips leave the
    // timeline in one undo step, the media is deleted once the project is saved), the library selection, drops onto
    // folders and the metadata the inspector shows. The folder model is window.VideoStudioFolders (DOM-free).
    const P = window.VideoStudioParts;
    const { API, TERMINAL } = P;
    const E = () => window.VideoStudioEdit;
    const F = () => window.VideoStudioFolders;
    const esc = (...args) => P.esc(...args);
    const fill = (...args) => P.fill(...args);
    const icon = (...args) => P.icon(...args);
    const mutate = (...args) => P.mutate(...args);
    const openModal = (...args) => P.openModal(...args);
    const openMenu = (...args) => P.openMenu(...args);
    const renderAssets = (...args) => P.renderAssets(...args);
    const renderInspector = (...args) => P.renderInspector(...args);
    const renderTimeline = (...args) => P.renderTimeline(...args);
    const request = (...args) => P.request(...args);
    const saveUntilClean = (...args) => P.saveUntilClean(...args);
    const setSelection = (...args) => P.setSelection(...args);
    const showNotice = (...args) => P.showNotice(...args);
    const tr = (...args) => P.tr(...args);
    const writeMediaPrefs = (...args) => P.writeMediaPrefs(...args);

    const FOLDER_NOTICES = {
        folder_invalid: ['folderInvalid', 'This folder name cannot be used.'],
        folder_exists: ['folderExists', 'A folder with this name already exists.'],
        folder_limit: ['folderLimit', 'This project has reached the folder limit.'],
        folder_inside: ['folderInside', 'A folder cannot be moved into itself.'],
        name_invalid: ['nameInvalid', 'Enter a name of 1 to 255 characters.']
    };
    const REMOVE_TEXT = {
        mediaInUse: 'This media is still in use. Save the project and try again.',
        mediaRemoveFailed: 'The media could not be removed.',
        mediaBusy: 'Wait until the background tasks of this project have finished, then remove the media.'
    };
    // Job kinds that read media files; the server refuses to delete media while one of them is queued or running.
    const MEDIA_READERS = ['render', 'preview', 'probe', 'reverse', 'still', 'duplicate'];
    const ASSET_TYPE = 'application/x-video-studio-asset';
    const PART_TYPE = 'application/x-video-studio-part';

    const assetOf = (s, id) => (s.project ? s.project.assets.find(item => item.id === id) : null) || null;
    const clipsOf = (project, ids) => project.tracks.flatMap(track => track.clips.filter(clip => ids.includes(clip.asset_id)));

    // A folder or name edit: one undo step, and a refusal explains itself.
    function folderEdit(s, label, op) {
        if (s.readonly || !s.project) return false;
        let reason = '';
        const ok = mutate(s, label, project => {
            const result = op(project);
            if (result !== true) { reason = result; return false; }
            return undefined;
        });
        if (!ok && reason && FOLDER_NOTICES[reason]) showNotice(s, FOLDER_NOTICES[reason][0], FOLDER_NOTICES[reason][1], true);
        return ok;
    }
    // Marks the selected item in the bin without redrawing it (a redraw between the two clicks of a double-click would
    // replace the card under the pointer).
    function markLibrarySelection(s) {
        s.host.querySelectorAll('[data-assets] [data-asset-id]').forEach(card => card.classList.toggle('is-selected', card.dataset.assetId === s.selectedAssetId));
    }
    // Selecting a library item shows its metadata in the inspector. Timeline clips are deselected so that the inspector
    // and the clip shortcuts (Delete, Ctrl+C) never act on clips the user no longer sees as selected.
    function selectLibraryAsset(s, id) {
        s.selectedAssetId = id || '';
        if (s.selectedAssetId && P.selectedIds(s).length) { setSelection(s, [], ''); renderTimeline(s); }
        else renderInspector(s);
        markLibrarySelection(s);
    }
    function moveAssetsTo(s, ids, folder) {
        folderEdit(s, 'move media', project => F().moveAssets(project, ids, folder));
        renderAssets(s);
    }
    function closeDialog(s, host) {
        host.replaceChildren();
        s.app.focus({ preventScroll: true });
    }
    // Creates a folder under the open folder, or renames one. Renaming keeps the bin on the renamed folder.
    function openFolderDialog(s, mode, path) {
        if (s.readonly || !s.project) return;
        const id = esc(s, s.id);
        const title = mode === 'rename' ? tr(s, 'renameFolder', 'Rename folder') : tr(s, 'newFolder', 'New folder');
        const current = mode === 'rename' ? F().nameOf(path) : '';
        const hint = mode === 'rename' ? '' : `<p class="vs-hint">${esc(s, tr(s, 'folderNameHint', 'Use / for subfolders, up to four levels.'))}</p>`;
        const host = openModal(s, `<form class="vs-modal" data-folder-form role="dialog" aria-modal="true" aria-labelledby="vs-folder-title-${id}"><button type="button" class="vs-modal-close" data-modal-close aria-label="${esc(s, tr(s, 'close', 'Close'))}">${icon('close', 18)}</button><span class="vs-modal-icon">${icon('folder', 22)}</span><h2 id="vs-folder-title-${id}">${esc(s, title)}</h2><label class="vs-field vs-field-wide"><span>${esc(s, tr(s, 'folderName', 'Folder name'))}</span><input type="text" data-folder-name maxlength="255" required spellcheck="false" value="${esc(s, current)}"></label>${hint}<div class="vs-modal-actions"><button type="button" data-modal-close>${esc(s, tr(s, 'cancel', 'Cancel'))}</button><button type="submit" class="vs-primary">${esc(s, tr(s, 'apply', 'Apply'))}</button></div></form>`);
        const input = host.querySelector('[data-folder-name]');
        input.select();
        host.querySelector('[data-folder-form]').addEventListener('submit', event => {
            event.preventDefault();
            const value = input.value;
            closeDialog(s, host);
            if (s.disposed || s.readonly || !s.project) return;
            if (mode === 'rename') {
                const target = value.includes('/') ? null : F().joinFolder(F().parentOf(path), value);
                if (!target) { showNotice(s, 'folderInvalid', FOLDER_NOTICES.folder_invalid[1], true); return; }
                if (!folderEdit(s, 'rename folder', project => F().renameFolder(project, path, target))) { renderAssets(s); return; }
                if (s.mediaFolder === path || s.mediaFolder.startsWith(path + '/')) s.mediaFolder = target + s.mediaFolder.slice(path.length);
            } else {
                const target = F().joinFolder(s.mediaFolder, value);
                if (!target) { showNotice(s, 'folderInvalid', FOLDER_NOTICES.folder_invalid[1], true); return; }
                if (folderEdit(s, 'new folder', project => F().createFolder(project, target))) s.mediaFolder = target;
            }
            renderAssets(s);
        });
    }
    // Deletes a folder after a confirm: its media and subfolders move to the folder above.
    async function deleteFolder(s, path) {
        if (s.readonly || !path || !s.project) return;
        const projectId = s.projectId, epoch = s.projectEpoch;
        const count = F().assetsIn(s.project, path, true).length;
        const copy = fill(tr(s, 'deleteFolderCopy', 'Its {{count}} items and subfolders move to the folder above.'), { count });
        if (!(await s.ctx.confirmDialog(tr(s, 'deleteFolderTitle', 'Delete this folder?'), copy))) return;
        if (s.disposed || s.readonly || epoch !== s.projectEpoch || projectId !== s.projectId) return;
        if (!folderEdit(s, 'delete folder', project => F().deleteFolder(project, path))) return;
        if (s.mediaFolder === path || s.mediaFolder.startsWith(path + '/')) s.mediaFolder = F().parentOf(path);
        renderAssets(s);
    }
    function openRenameAsset(s, id) {
        const asset = assetOf(s, id);
        if (!asset || s.readonly) return;
        const host = openModal(s, `<form class="vs-modal" data-rename-asset-form role="dialog" aria-modal="true" aria-labelledby="vs-rename-asset-title-${esc(s, s.id)}"><button type="button" class="vs-modal-close" data-modal-close aria-label="${esc(s, tr(s, 'close', 'Close'))}">${icon('close', 18)}</button><span class="vs-modal-icon">${icon('film', 22)}</span><h2 id="vs-rename-asset-title-${esc(s, s.id)}">${esc(s, tr(s, 'assetRename', 'Rename media'))}</h2><label class="vs-field vs-field-wide"><span>${esc(s, tr(s, 'colName', 'Name'))}</span><input type="text" data-asset-name maxlength="255" required spellcheck="false" value="${esc(s, asset.name)}"></label><div class="vs-modal-actions"><button type="button" data-modal-close>${esc(s, tr(s, 'cancel', 'Cancel'))}</button><button type="submit" class="vs-primary">${esc(s, tr(s, 'apply', 'Apply'))}</button></div></form>`);
        host.querySelector('[data-asset-name]').select();
        host.querySelector('[data-rename-asset-form]').addEventListener('submit', event => {
            event.preventDefault();
            const value = host.querySelector('[data-asset-name]').value;
            closeDialog(s, host);
            if (s.disposed) return;
            folderEdit(s, 'rename media', project => F().renameAsset(project, id, value));
            renderAssets(s);
        });
    }
    // Chooses a folder for the given media; the top level is offered too.
    function openMoveDialog(s, ids) {
        if (s.readonly || !s.project) return;
        const current = ids.length === 1 ? ((assetOf(s, ids[0]) || {}).folder || '') : '';
        const options = [''].concat(F().folderPaths(s.project)).map(path => `<option value="${esc(s, path)}" ${path === current ? 'selected' : ''}>${esc(s, path || tr(s, 'rootFolder', 'Media (top level)'))}</option>`).join('');
        const host = openModal(s, `<form class="vs-modal" data-move-form role="dialog" aria-modal="true" aria-labelledby="vs-move-title-${esc(s, s.id)}"><button type="button" class="vs-modal-close" data-modal-close aria-label="${esc(s, tr(s, 'close', 'Close'))}">${icon('close', 18)}</button><span class="vs-modal-icon">${icon('folder', 22)}</span><h2 id="vs-move-title-${esc(s, s.id)}">${esc(s, tr(s, 'moveToFolderTitle', 'Move to folder'))}</h2><label class="vs-field vs-field-wide"><span>${esc(s, tr(s, 'moveFolderLabel', 'Folder'))}</span><select data-move-folder>${options}</select></label><div class="vs-modal-actions"><button type="button" data-modal-close>${esc(s, tr(s, 'cancel', 'Cancel'))}</button><button type="submit" class="vs-primary">${esc(s, tr(s, 'apply', 'Apply'))}</button></div></form>`);
        host.querySelector('[data-move-form]').addEventListener('submit', event => {
            event.preventDefault();
            const target = host.querySelector('[data-move-folder]').value;
            closeDialog(s, host);
            if (s.disposed) return;
            moveAssetsTo(s, ids, target);
        });
    }
    // Shows the clips that use the media: they are selected together and scrolled into view; the playhead stays.
    function showInTimeline(s, assetId) {
        if (!s.project) return;
        const clips = clipsOf(s.project, [assetId]);
        if (!clips.length) { showNotice(s, 'noClipsUsingMedia', 'This media is not used in the timeline.', false); return; }
        const ids = clips.map(clip => clip.id);
        setSelection(s, ids, ids[0]);
        renderTimeline(s);
        P.T().revealFrame(s.q('[data-timeline]'), s, Math.min(...clips.map(clip => clip.start)));
    }
    function mediaBusy(s) { return (s.jobs || []).some(job => MEDIA_READERS.includes(job.kind) && !TERMINAL.includes(job.status)); }
    // Deleted media cannot come back: the project and its undo and redo snapshots drop it with its clips (P.forgetMedia),
    // so an undo never restores a clip whose file is gone (the server would refuse to save it). A step that no longer
    // changes anything (the removal itself, when nothing else happened) leaves the history. Edits and undo/redo wait
    // while the files are deleted (s.mediaRemoving), so the project normally has nothing left to drop; if it had, the
    // change is saved.
    function forgetDeletedMedia(s, deleted) {
        if (!deleted.size) return;
        if (P.forgetMedia(s, deleted)) P.scheduleSave(s);
        P.renderUI(s);
    }
    // Removes the media: the clips leave the timeline in one undo step (gaps stay), the project is saved, and then the
    // server deletes the media files. Nothing is deleted when the save fails; a refused delete keeps the file, and the
    // removal stays undoable. Once a file is deleted, its removal can no longer be undone (forgetDeletedMedia).
    async function removeMediaFrom(s, ids) {
        const assets = ids.map(id => assetOf(s, id)).filter(Boolean);
        const clipIds = clipsOf(s.project, ids).map(clip => clip.id);
        const ok = P.applyEdit(s, 'remove media', project => {
            // A clip on a locked track cannot leave, and its media cannot go without it.
            if (project.tracks.some(track => track.locked && track.clips.some(clip => ids.includes(clip.asset_id)))) return E().REASON.locked;
            if (clipIds.length) {
                const result = E().removeClips(project, clipIds);
                if (result !== true) return result;
            }
            project.assets = project.assets.filter(asset => !ids.includes(asset.id));
            return true;
        }, () => []);
        if (!ok) return false;
        if (ids.includes(s.selectedAssetId)) s.selectedAssetId = '';
        renderAssets(s);
        renderInspector(s);
        const projectId = s.projectId, epoch = s.projectEpoch;
        if (!(await saveUntilClean(s))) {
            if (!s.disposed && epoch === s.projectEpoch && projectId === s.projectId) showNotice(s, 'mediaRemoveFailed', REMOVE_TEXT.mediaRemoveFailed, true);
            return false;
        }
        if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return false;
        const deleted = new Set();
        let failure = null;
        // Edits and undo/redo wait until the deleted media has left the project and its history. A counter, because
        // removals may overlap; released exactly once, whatever happens.
        let released = false;
        const release = () => { if (!released) { released = true; s.mediaRemoving = Math.max(0, (s.mediaRemoving || 0) - 1); } };
        s.mediaRemoving = (s.mediaRemoving || 0) + 1;
        P.renderToolbar(s);
        try {
            for (const asset of assets) {
                try {
                    await request(API + '/projects/' + encodeURIComponent(projectId) + '/media/' + encodeURIComponent(asset.id), { method: 'DELETE', signal: s.abort.signal });
                    deleted.add(asset.id);
                } catch (error) {
                    failure = error;
                    break;
                }
            }
            if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return false;
            // Synchronous from here: no edit can slip in between the release and the cleanup that re-renders.
            release();
            forgetDeletedMedia(s, deleted);
        } finally {
            if (!released) { release(); if (!s.disposed) P.renderToolbar(s); }
        }
        if (failure) {
            const inUse = failure.body && failure.body.code === 'media_in_use';
            showNotice(s, inUse ? 'mediaInUse' : 'mediaRemoveFailed', inUse ? REMOVE_TEXT.mediaInUse : REMOVE_TEXT.mediaRemoveFailed, true);
            return false;
        }
        showNotice(s, 'mediaRemoved', 'Media removed.', false);
        return true;
    }
    // Confirms, then removes. The copy names the clips that leave the timeline.
    async function removeAssets(s, ids, bulk) {
        if (s.readonly || !s.project || !ids.length) return false;
        if (mediaBusy(s)) { showNotice(s, 'mediaBusy', REMOVE_TEXT.mediaBusy, true); return false; }
        const projectId = s.projectId, epoch = s.projectEpoch;
        const clips = clipsOf(s.project, ids).length;
        let title, copy;
        if (bulk) {
            title = tr(s, 'removeUnusedTitle', 'Remove unused media?');
            copy = fill(tr(s, 'removeUnusedCopy', '{{count}} media items are not used in the timeline and will be deleted.'), { count: ids.length });
        } else if (clips) {
            title = tr(s, 'removeMediaUsedTitle', 'Remove media used in the timeline?');
            copy = fill(tr(s, 'removeMediaUsedCopy', '{{count}} clips using it leave the timeline in one undo step. Then the file is deleted. This cannot be undone.'), { count: clips });
        } else {
            title = tr(s, 'removeMediaUnusedTitle', 'Remove this media?');
            copy = tr(s, 'removeMediaUnusedCopy', 'The file is deleted from the project. This cannot be undone.');
        }
        if (!(await s.ctx.confirmDialog(title, copy))) return false;
        if (s.disposed || s.readonly || !s.project || epoch !== s.projectEpoch || projectId !== s.projectId) return false;
        return removeMediaFrom(s, ids.filter(id => assetOf(s, id)));
    }
    // Removes every media item that no clip uses, after one confirm.
    function removeUnused(s) {
        if (s.readonly || !s.project) return false;
        const usage = F().usageCounts(s.project);
        const ids = s.project.assets.filter(asset => !window.VideoStudioMedia.isArtwork(asset) && !usage.get(asset.id)).map(asset => asset.id);
        if (!ids.length) { showNotice(s, 'removeUnusedNone', 'Every media item is used.', false); return false; }
        return removeAssets(s, ids, true);
    }
    // Item menu: open in the source monitor (also the keyboard way to open a source), rename, move, show its clips, remove.
    function openLibraryMenu(s, assetId, x, y) {
        const asset = assetOf(s, assetId);
        if (!asset) return null;
        selectLibraryAsset(s, assetId);
        const ro = !!s.readonly, clips = clipsOf(s.project, [assetId]).length;
        const items = [
            { label: tr(s, 'openInSource', 'Open in source monitor'), icon: 'film', disabled: !asset.kind, run: () => P.openSource(s, assetId) },
            { label: tr(s, 'assetRename', 'Rename media'), icon: 'edit', disabled: ro, run: () => openRenameAsset(s, assetId) },
            { label: tr(s, 'moveToFolder', 'Move to folder…'), icon: 'folder', disabled: ro, run: () => openMoveDialog(s, [assetId]) },
            { label: tr(s, 'showInTimeline', 'Show in timeline'), disabled: !clips, run: () => showInTimeline(s, assetId) },
            { separator: true },
            { label: tr(s, 'removeMedia', 'Remove media…'), icon: 'trash', disabled: ro, run: () => removeAssets(s, [assetId], false) }
        ];
        return openMenu(s, x, y, items, tr(s, 'libraryMenu', 'Media actions'), () => s.q(`[data-assets] [data-asset-id="${CSS.escape(assetId)}"]`));
    }
    function openFolderMenu(s, path, x, y) {
        const ro = !!s.readonly;
        const items = [
            { label: tr(s, 'openFolder', 'Open'), icon: 'folder', run: () => { s.mediaFolder = path; renderAssets(s); } },
            { label: tr(s, 'renameFolder', 'Rename folder'), icon: 'edit', disabled: ro, run: () => openFolderDialog(s, 'rename', path) },
            { separator: true },
            { label: tr(s, 'deleteFolder', 'Delete folder'), icon: 'trash', disabled: ro, run: () => deleteFolder(s, path) }
        ];
        return openMenu(s, x, y, items, tr(s, 'libraryMenu', 'Media actions'), () => s.q(`[data-assets] [data-folder="${CSS.escape(path)}"]`) || s.q(`[data-media-bar] [data-folder="${CSS.escape(path)}"]`));
    }
    function openBlankMediaMenu(s, x, y) {
        const ro = !!s.readonly;
        const items = [
            { label: tr(s, 'newFolder', 'New folder'), icon: 'folder', disabled: ro, run: () => openFolderDialog(s, 'create') },
            { label: tr(s, 'removeUnused', 'Remove unused media'), icon: 'trash', disabled: ro, run: () => removeUnused(s) }
        ];
        return openMenu(s, x, y, items, tr(s, 'libraryMenu', 'Media actions'), () => s.q('[data-media-bar] [data-action="new-folder"]') || s.q('[data-assets]'));
    }
    // Opening a folder from the keyboard moves focus to the crumb of the open folder, since the activated tile is gone.
    function openFolder(s, path) {
        s.mediaFolder = path || '';
        renderAssets(s);
        if (!s.host.contains(document.activeElement)) s.q('[data-media-bar] [aria-current="page"]')?.focus({ preventScroll: true });
    }
    // Clicks in the media bin and its bar. Returns true when the click was handled. Folder, sort, view and selection
    // clicks only change what this viewer sees; the two editing buttons refuse in read-only mode, so the generic gate in
    // handleClick explains it.
    function handleMediaClick(s, event) {
        if (event.target.closest('[data-place-asset]')) return false;
        const inBin = event.target.closest('[data-assets]'), inBar = event.target.closest('[data-media-bar]');
        if (!inBin && !inBar) return false;
        const folder = event.target.closest('[data-folder]');
        if (folder) { openFolder(s, folder.dataset.folder); return true; }
        const sort = event.target.closest('[data-media-sort]');
        if (sort && inBin) {
            const key = sort.dataset.mediaSort;
            s.mediaSortDir = s.mediaSort === key && s.mediaSortDir === 'asc' ? 'desc' : 'asc';
            s.mediaSort = key;
            writeMediaPrefs(s);
            renderAssets(s);
            return true;
        }
        const view = event.target.closest('[data-action="media-view"]');
        if (view) { s.mediaView = view.dataset.view === 'list' ? 'list' : 'grid'; writeMediaPrefs(s); renderAssets(s); return true; }
        if (s.readonly && event.target.closest('[data-action="new-folder"], [data-action="remove-unused"]')) return false;
        if (event.target.closest('[data-action="new-folder"]')) { openFolderDialog(s, 'create'); return true; }
        if (event.target.closest('[data-action="remove-unused"]')) { removeUnused(s); return true; }
        const card = event.target.closest('[data-asset-id]');
        if (card && inBin) { selectLibraryAsset(s, card.dataset.assetId); return true; }
        if (inBin && !event.target.closest('[data-upload-key]') && s.selectedAssetId) { selectLibraryAsset(s, ''); return true; }
        return false;
    }
    // Context menus on items, folders and empty space (right-click, the context-menu key and Shift+F10), and drops of
    // library items onto folder tiles, folder rows and breadcrumb crumbs.
    function wireMediaBin(s) {
        const bin = s.q('[data-assets]'), bar = s.q('[data-media-bar]');
        if (!bin) return;
        const signal = s.abort.signal;
        const onContext = event => {
            if (!s.project || !event.target.closest) return;
            event.preventDefault();
            event.stopPropagation();
            const card = event.target.closest('[data-asset-id]'), folder = event.target.closest('[data-folder]');
            const anchor = card || folder || event.currentTarget, rect = anchor.getBoundingClientRect();
            // A keyboard-opened menu has no pointer position inside the target; it anchors below the target.
            const inside = event.clientX >= rect.left && event.clientX <= rect.right && event.clientY >= rect.top && event.clientY <= rect.bottom;
            const x = inside ? event.clientX : rect.left + Math.min(24, rect.width / 2), y = inside ? event.clientY : Math.min(rect.bottom, window.innerHeight - 8);
            if (card) openLibraryMenu(s, card.dataset.assetId, x, y);
            else if (folder && folder.dataset.folder) openFolderMenu(s, folder.dataset.folder, x, y);
            else openBlankMediaMenu(s, x, y);
        };
        const targets = [bin, bar].filter(Boolean);
        const assetDrag = event => {
            const types = Array.from(event.dataTransfer && event.dataTransfer.types || []);
            return types.includes(ASSET_TYPE) && !types.includes(PART_TYPE);
        };
        const clearDrop = () => s.host.querySelectorAll('[data-folder].is-drop').forEach(el => el.classList.remove('is-drop'));
        targets.forEach(target => {
            target.addEventListener('contextmenu', onContext, { signal });
            target.addEventListener('dragover', event => {
                if (!assetDrag(event)) return;
                const folder = event.target.closest('[data-folder]');
                clearDrop();
                if (!folder || s.readonly || !s.project) return;
                event.preventDefault();
                event.dataTransfer.dropEffect = 'move';
                folder.classList.add('is-drop');
            }, { signal });
            target.addEventListener('dragleave', event => { if (!target.contains(event.relatedTarget)) clearDrop(); }, { signal });
            target.addEventListener('drop', event => {
                const folder = event.target.closest('[data-folder]');
                clearDrop();
                if (!folder || s.readonly || !s.project || !assetDrag(event)) return;
                const assetId = event.dataTransfer.getData(ASSET_TYPE);
                if (!assetId) return;
                event.preventDefault();
                moveAssetsTo(s, [assetId], folder.dataset.folder || '');
            }, { signal });
        });
        window.addEventListener('dragend', clearDrop, { signal });
    }
    // The selected library item's metadata rows for the inspector, or null while clips are selected or nothing is selected.
    function libraryAssetInfo(s) {
        if (!s.project || !s.selectedAssetId || P.selectedIds(s).length) return null;
        const asset = assetOf(s, s.selectedAssetId);
        if (!asset) return null;
        const usage = F().usageCounts(s.project).get(asset.id) || 0;
        const timed = asset.kind === 'video' || asset.kind === 'audio';
        const rows = [
            [tr(s, 'mediaKind', 'Kind'), P.assetKindLabel(s, asset)],
            [tr(s, 'length', 'Length'), timed ? window.VideoStudioTime.formatTime(asset.duration_frames || 0, s.project) : '—'],
            [tr(s, 'colResolution', 'Resolution'), P.resolutionLabel(asset) || '—'],
            [tr(s, 'colFrameRate', 'Frame rate'), P.frameRateLabel(asset) || '—'],
            [tr(s, 'colCodec', 'Codec'), asset.codec || '—'],
            [tr(s, 'colSize', 'Size'), P.formatBytes(asset.size_bytes, P.uiLocale()) || '—'],
            [tr(s, 'mediaUsed', 'Used in timeline'), usage ? String(usage) : tr(s, 'mediaNotUsed', 'Not used')]
        ];
        return {
            name: asset.name, kind: asset.kind || 'pending', icon: asset.kind === 'audio' ? 'audio' : asset.kind === 'image' ? 'image' : 'video',
            thumb: asset.kind ? P.thumbFor(s, asset) : '', folderPath: asset.folder || '', rows
        };
    }

    Object.assign(P, { folderEdit, selectLibraryAsset, moveAssetsTo, openFolderDialog, deleteFolder, openRenameAsset, openMoveDialog, showInTimeline, removeAssets, removeUnused, openLibraryMenu, openFolderMenu, openBlankMediaMenu, handleMediaClick, wireMediaBin, libraryAssetInfo });
})();
