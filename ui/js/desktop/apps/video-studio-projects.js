(function () {
    'use strict';

    // Project settings, the new-project dialog and the project manager ("All projects…": open, rename, duplicate, delete,
    // and the versions of the open project).
    const P = window.VideoStudioParts = window.VideoStudioParts || {};
    const { CANVASES } = P;
    const canonicalProject = (...args) => P.canonicalProject(...args);
    const clearDraft = (...args) => P.clearDraft(...args);
    const createProject = (...args) => P.createProject(...args);
    const esc = (...args) => P.esc(...args);
    const fill = (...args) => P.fill(...args);
    const hydrateProject = (...args) => P.hydrateProject(...args);
    const icon = (...args) => P.icon(...args);
    const loadProject = (...args) => P.loadProject(...args);
    const loadProjects = (...args) => P.loadProjects(...args);
    const mutate = (...args) => P.mutate(...args);
    const openModal = (...args) => P.openModal(...args);
    const renderEmptyProject = (...args) => P.renderEmptyProject(...args);
    const request = (...args) => P.request(...args);
    const saveProject = (...args) => P.saveProject(...args);
    const saveUntilClean = (...args) => P.saveUntilClean(...args);
    const showConflict = (...args) => P.showConflict(...args);
    const showNotice = (...args) => P.showNotice(...args);
    const switchProject = (...args) => P.switchProject(...args);
    const tr = (...args) => P.tr(...args);
    const Time = () => window.VideoStudioTime;

    function rateValue(rate) { return rate.fps + (rate.ntsc ? ':ntsc' : ''); }
    function parseRate(value) { const [fps, ntsc] = String(value).split(':'); return { fps: Number(fps) || 30, ntsc: ntsc === 'ntsc' }; }
    function sizes(s) { return s.status && s.status.limits && s.status.limits.canvas_sizes || CANVASES.map(c => ({ width: c.width, height: c.height })); }
    function rates(s) { return s.status && s.status.limits && s.status.limits.frame_rates || Time().FRAME_RATES; }

    function fieldsMarkup(s, values) {
        const locale = P.uiLocale();
        const formats = sizes(s).map(size => {
            const key = (CANVASES.find(c => c.width === size.width && c.height === size.height) || {}).key;
            const value = `${Number(size.width)}x${Number(size.height)}`;
            return `<option value="${value}" ${value === values.size ? 'selected' : ''}>${esc(s, key ? tr(s, key, `${size.width} × ${size.height}`) : `${size.width} × ${size.height}`)}</option>`;
        }).join('');
        const rateOptions = rates(s).map(rate => `<option value="${esc(s, rateValue(rate))}" ${rateValue(rate) === values.rate ? 'selected' : ''}>${esc(s, Time().rateLabel(rate, locale))}</option>`).join('');
        return `<label class="vs-field vs-field-wide"><span>${esc(s, tr(s, 'projectNameLabel', 'Project name'))}</span><input type="text" data-settings-name maxlength="120" required value="${esc(s, values.name)}"></label><div class="vs-field-row vs-settings-row"><label class="vs-field"><span>${esc(s, tr(s, 'format', 'Format'))}</span><select data-settings-size>${formats}</select></label><label class="vs-field"><span>${esc(s, tr(s, 'frameRate', 'Frame rate'))}</span><select data-settings-rate>${rateOptions}</select></label></div>`;
    }

    function readFields(host) {
        const [width, height] = host.querySelector('[data-settings-size]').value.split('x').map(Number);
        return Object.assign({ name: host.querySelector('[data-settings-name]').value.trim().slice(0, 120), width, height }, parseRate(host.querySelector('[data-settings-rate]').value));
    }

    function openProjectSettings(s) {
        if (!s.project || s.readonly) return;
        const projectId = s.projectId, epoch = s.projectEpoch;
        const values = { name: s.project.name || '', size: `${s.project.width}x${s.project.height}`, rate: rateValue({ fps: Number(s.project.fps) || 30, ntsc: !!s.project.ntsc }) };
        const host = openModal(s, `<form class="vs-modal" data-settings-form role="dialog" aria-modal="true" aria-labelledby="vs-settings-title-${esc(s, s.id)}"><button type="button" class="vs-modal-close" data-modal-close aria-label="${esc(s, tr(s, 'close', 'Close'))}">${icon('close', 18)}</button><span class="vs-modal-icon">${icon('sliders', 22)}</span><h2 id="vs-settings-title-${esc(s, s.id)}">${esc(s, tr(s, 'projectSettingsTitle', 'Project settings'))}</h2><p>${esc(s, tr(s, 'projectSettingsIntro', 'Boxes keep their position relative to the frame when the format changes.'))}</p>${fieldsMarkup(s, values)}<div class="vs-modal-actions"><button type="button" data-modal-close>${esc(s, tr(s, 'cancel', 'Cancel'))}</button><button type="submit" class="vs-primary">${esc(s, tr(s, 'apply', 'Apply'))}</button></div></form>`);
        host.querySelector('[data-settings-form]').addEventListener('submit', async event => {
            event.preventDefault();
            const next = readFields(host);
            host.replaceChildren();
            s.app.focus({ preventScroll: true });
            if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
            const before = { fps: Number(s.project.fps) || 30, ntsc: !!s.project.ntsc };
            const rateChanged = next.fps !== before.fps || next.ntsc !== before.ntsc;
            // Apply without a change records no undo step and does not touch the project.
            const unchanged = (!next.name || next.name === s.project.name) && next.width === Number(s.project.width) && next.height === Number(s.project.height) && !rateChanged;
            if (unchanged) return;
            // A change that would push a clip past the 60-minute limit is refused before anything changes (the overlap
            // message below is for clips that would collide).
            if (rateChanged && !Time().rescaleFits(s.project, next.fps, next.ntsc)) return showNotice(s, 'rateChangeLimit', 'That frame rate would make the project longer than 60 minutes.', true);
            if (rateChanged && !(await s.ctx.confirmDialog(tr(s, 'changeRateTitle', 'Change the frame rate?'), tr(s, 'changeRateCopy', 'All clip times are rounded to {{rate}}. You can undo this.').replace('{{rate}}', Time().rateLabel(next, P.uiLocale()))))) return;
            if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
            const renamed = !!next.name && next.name !== s.project.name;
            const ok = mutate(s, 'project settings', project => {
                if (next.name) project.name = next.name;
                project.width = next.width; project.height = next.height;
                if (rateChanged) Time().rescaleProject(project, next.fps, next.ntsc);
            });
            if (ok && rateChanged) { s.frame = Time().frameAtRate(s.frame, before, next); s.preview.seek(s.frame); }
            if (!ok && rateChanged) showNotice(s, 'rateChangeFailed', 'The frame rate could not be changed because some clips would overlap.', true);
            // The project list shows names from the server; refresh it once the new name is saved.
            if (ok && renamed) saveProject(s).then(() => loadProjects(s)).catch(() => {});
        });
    }

    async function openNewProject(s) {
        if (s.readonly) return showNotice(s, 'readonly', 'Read-only mode prevents editing.', true);
        await P.flushArtwork(s);
        if (s.dirty && !(await saveUntilClean(s))) return;
        if (s.disposed) return;
        const values = { name: tr(s, 'untitled', 'Untitled project'), size: '1920x1080', rate: '30' };
        const host = openModal(s, `<form class="vs-modal" data-new-project-form role="dialog" aria-modal="true" aria-labelledby="vs-new-title-${esc(s, s.id)}"><button type="button" class="vs-modal-close" data-modal-close aria-label="${esc(s, tr(s, 'close', 'Close'))}">${icon('close', 18)}</button><span class="vs-modal-icon">${icon('plus', 22)}</span><h2 id="vs-new-title-${esc(s, s.id)}">${esc(s, tr(s, 'newProject', 'New project'))}</h2>${fieldsMarkup(s, values)}<div class="vs-modal-actions"><button type="button" data-modal-close>${esc(s, tr(s, 'cancel', 'Cancel'))}</button><button type="submit" class="vs-primary">${icon('plus', 16)}<span>${esc(s, tr(s, 'createProject', 'Create project'))}</span></button></div></form>`);
        host.querySelector('[data-settings-name]').select();
        host.querySelector('[data-new-project-form]').addEventListener('submit', async event => {
            event.preventDefault();
            const next = readFields(host);
            host.replaceChildren();
            if (!next.name) return;
            const settings = { width: next.width, height: next.height, fps: next.fps };
            if (next.ntsc) settings.ntsc = true;
            try {
                const created = await createProject(s, next.name, settings);
                if (!created && s.cameraImportAwaitCreate && P.resumeCameraImport) P.resumeCameraImport(s);
            }
            catch (_) {
                showNotice(s, 'createFailed', 'Could not create the project.', true);
                if (s.cameraImportAwaitCreate && P.resumeCameraImport) P.resumeCameraImport(s);
            }
        });
    }

    // ---- Project manager ----

    // The asset of the first visual clip in timeline order (earliest start, then track order): the manager's picture.
    // Audio tracks, hidden tracks and disabled clips do not count.
    function firstVisualAsset(project) {
        let best = null;
        ((project && project.tracks) || []).forEach((track, index) => {
            if (track.kind === 'audio' || track.hidden) return;
            (track.clips || []).forEach(clip => {
                if (clip.disabled) return;
                if (!best || clip.start < best.start || (clip.start === best.start && index < best.index)) best = { start: clip.start, index, assetId: clip.asset_id };
            });
        });
        return best ? (project.assets || []).find(asset => asset.id === best.assetId) || null : null;
    }
    function listedId(item) { return item.id || item.project_id || ''; }
    function storedItem(s, id) { return (s.projects || []).find(entry => listedId(entry) === id) || null; }
    // The open project shows its local name (an unsaved rename included); the others show their saved name.
    function storedName(s, item) {
        if (listedId(item) === s.projectId && s.project && s.project.name) return s.project.name;
        return (item.project && item.project.name) || tr(s, 'untitled', 'Untitled project');
    }
    // The listed storage (only GET /projects?sizes=1 has it): '' when the server's capped size walk gave up.
    function storageText(item) { return item && typeof item.storage_bytes === 'number' ? P.formatBytes(item.storage_bytes, P.uiLocale()) || '0 B' : ''; }
    function changedText(item) {
        const date = item.updated_at ? new Date(item.updated_at) : null;
        return date && date.getFullYear() > 1970 ? date.toLocaleString(P.uiLocale(), { dateStyle: 'medium', timeStyle: 'short' }) : '–';
    }
    // encodeURIComponent leaves quotes and parentheses alone; a CSS url() must not see them.
    function cssURL(url) { return url.replace(/['()]/g, c => '%' + c.charCodeAt(0).toString(16).toUpperCase()); }
    function projectRow(s, item) {
        const id = listedId(item), current = id === s.projectId && !!s.project;
        const project = current ? s.project : item.project || {};
        const asset = firstVisualAsset(project);
        const poster = asset && asset.has_poster ? cssURL(P.API + '/projects/' + encodeURIComponent(id) + '/media/' + encodeURIComponent(asset.id) + '/poster.jpg') : '';
        const format = `${Number(project.width) || 0}×${Number(project.height) || 0} · ${Time().rateLabel(project, P.uiLocale())}`;
        const length = Time().formatTime(window.VideoStudioTimeline.projectEnd(project), project);
        const button = (attr, key, fallback, off, cls) => `<button type="button"${cls ? ` class="${cls}"` : ''} ${attr}="${esc(s, id)}"${off ? ' disabled' : ''}>${esc(s, tr(s, key, fallback))}</button>`;
        return `<article class="vs-manager-row${current ? ' is-current' : ''}" role="listitem" data-manager-row="${esc(s, id)}"${current ? ' aria-current="true"' : ''}>`
            + `<span class="vs-manager-poster"${poster ? ` style="background-image:url('${esc(s, poster)}')"` : ''}>${poster ? '' : icon('film', 22)}</span>`
            + `<span class="vs-manager-main"><strong>${esc(s, storedName(s, item))}</strong><small>${esc(s, format)} · ${esc(s, length)}</small><small>${esc(s, tr(s, 'colStorage', 'Storage'))}: ${esc(s, storageText(item) || '–')} · ${esc(s, tr(s, 'colChanged', 'Last change'))}: ${esc(s, changedText(item))}</small></span>`
            + `<span class="vs-manager-actions">${button('data-manager-open', 'openProject', 'Open', current)}${button('data-manager-rename', 'rename', 'Rename', s.readonly)}${button('data-manager-duplicate', 'duplicate', 'Duplicate', s.readonly)}${button('data-manager-delete', 'delete', 'Delete', s.readonly, 'vs-danger')}</span></article>`;
    }
    function closeModal(s) { s.q('[data-modal-host]').replaceChildren(); s.app.focus({ preventScroll: true }); }
    function managerRows(s) { return (s.projects || []).map(item => projectRow(s, item)).join('') || `<p class="vs-hint">${esc(s, tr(s, 'noProjectsYet', 'No projects yet.'))}</p>`; }
    // All projects: poster, name, format, length, storage and last change, with open, rename, duplicate and delete; the
    // versions of the open project below. `listed` reuses the list that was just loaded instead of asking for the sizes again.
    async function openProjectManager(s, listed) {
        if (s.disposed || s.featureDisabled) return null;
        if (!listed) { try { await loadProjects(s, { sizes: true }); } catch (_) { /* the last list stays */ } }
        if (s.disposed) return null;
        const versions = s.projectId && s.project ? '' : `<p class="vs-hint">${esc(s, tr(s, 'versionsOpenFirst', 'Open this project to manage its versions.'))}</p>`;
        const host = openModal(s, `<section class="vs-modal vs-manager" role="dialog" aria-modal="true" aria-labelledby="vs-manager-title-${esc(s, s.id)}" data-manager><button type="button" class="vs-modal-close" data-modal-close aria-label="${esc(s, tr(s, 'close', 'Close'))}">${icon('close', 18)}</button><h2 id="vs-manager-title-${esc(s, s.id)}">${esc(s, tr(s, 'projectManagerTitle', 'All projects'))}</h2><p>${esc(s, tr(s, 'projectManagerIntro', 'Open, rename, duplicate or delete projects. Storage includes media, exports and versions.'))}</p><div class="vs-manager-list" role="list">${managerRows(s)}</div><section class="vs-manager-versions" data-manager-versions>${versions}</section><div class="vs-modal-actions"><button type="button" data-modal-close>${esc(s, tr(s, 'close', 'Close'))}</button></div></section>`);
        // The listener sits on the dialog, not on the modal host the other dialogs share, so it goes away with it.
        const dialog = host.querySelector('[data-manager]');
        dialog.addEventListener('click', event => handleManagerClick(s, event));
        (dialog.querySelector('.vs-manager-actions button:not(:disabled)') || dialog.querySelector('.vs-modal-close')).focus();
        if (s.projectId && s.project) renderVersions(s, dialog).catch(() => {});
        return dialog;
    }
    // Shows the current project list in the open manager (after a background copy finished), keeping its versions panel.
    function refreshManagerList(s) {
        const list = s.q('[data-modal-host] [data-manager] .vs-manager-list');
        if (list) list.innerHTML = managerRows(s);
    }
    function handleManagerClick(s, event) {
        const button = event.target.closest('[data-manager-open],[data-manager-rename],[data-manager-duplicate],[data-manager-delete]');
        if (!button || button.disabled) return;
        const { managerOpen, managerRename, managerDuplicate, managerDelete } = button.dataset;
        if (managerOpen) { closeModal(s); switchProject(s, managerOpen).catch(() => {}); }
        else if (managerRename) openRenameStoredProject(s, managerRename);
        else if (managerDuplicate) openDuplicateProject(s, managerDuplicate);
        else if (managerDelete) deleteStoredProject(s, managerDelete).catch(() => {});
    }
    // A dialog opened from the manager returns to it when it is cancelled.
    function backToManager(s, host) { host.querySelectorAll('[data-modal-close]').forEach(button => button.addEventListener('click', () => { openProjectManager(s, true).catch(() => {}); }, { once: true })); }
    function openRenameStoredProject(s, id) {
        const item = storedItem(s, id);
        if (!item || s.disposed) return;
        if (s.readonly) { showNotice(s, 'readonly', 'Read-only mode prevents editing.', true); return; }
        const current = storedName(s, item);
        const host = openModal(s, `<form class="vs-modal" data-rename-form role="dialog" aria-modal="true" aria-labelledby="vs-rename-title-${esc(s, s.id)}"><button type="button" class="vs-modal-close" data-modal-close aria-label="${esc(s, tr(s, 'close', 'Close'))}">${icon('close', 18)}</button><span class="vs-modal-icon">${icon('edit', 22)}</span><h2 id="vs-rename-title-${esc(s, s.id)}">${esc(s, tr(s, 'renameProjectTitle', 'Rename project'))}</h2><label class="vs-field vs-field-wide"><span>${esc(s, tr(s, 'projectNameLabel', 'Project name'))}</span><input type="text" data-rename-input maxlength="120" required value="${esc(s, current)}"></label><div class="vs-modal-actions"><button type="button" data-modal-close>${esc(s, tr(s, 'cancel', 'Cancel'))}</button><button type="submit" class="vs-primary">${esc(s, tr(s, 'apply', 'Apply'))}</button></div></form>`);
        backToManager(s, host);
        const input = host.querySelector('[data-rename-input]');
        input.select();
        host.querySelector('[data-rename-form]').addEventListener('submit', async event => {
            event.preventDefault();
            const value = input.value.trim().slice(0, 120);
            closeModal(s);
            if (s.disposed) return;
            if (value && value !== current) await renameStoredProject(s, id, value);
            if (!s.disposed) openProjectManager(s).catch(() => {});
        });
    }
    // The open project is renamed through the normal edit and save path (one undo step). Another project's document is
    // read and saved again with the new name, with the If-Match of the copy just read.
    async function renameStoredProject(s, id, name) {
        try {
            if (id === s.projectId && s.project) {
                if (!mutate(s, 'rename', project => { project.name = name; })) throw new Error('rename_refused');
                // A failed save explains itself (its notice or the conflict choices).
                if (!(await saveUntilClean(s))) return false;
            } else {
                const url = P.API + '/projects/' + encodeURIComponent(id);
                const stored = await request(url, { signal: s.abort.signal });
                const project = hydrateProject(s, stored.body.project || stored.body);
                project.name = name;
                await request(url, { method: 'PUT', headers: { 'Content-Type': 'application/json', 'If-Match': stored.etag }, body: JSON.stringify(canonicalProject(project)), signal: s.abort.signal });
            }
            showNotice(s, 'projectRenamed', 'Project renamed.', false);
            return true;
        } catch (_) {
            if (!s.disposed) showNotice(s, 'renameFailed', 'The project could not be renamed.', true);
            return false;
        }
    }
    // Forgets the open project after it was deleted: pending saves, draft, undo history, jobs and the source monitor.
    function leaveDeletedProject(s) {
        s.projectEpoch = (s.projectEpoch || 0) + 1;
        clearTimeout(s.autosaveTimer);
        if (s.preview) s.preview.pause();
        if (s.sourceAssetId && P.closeSource) P.closeSource(s);
        clearDraft(s);
        P.cancelTextApplies(s);
        Object.assign(s, { projectId: '', project: null, etag: '', desktopPath: '', savedProject: null, dirty: false, conflict: false, saveError: '', history: [], redo: [], textBefore: null, jobs: [], selectedClipId: '', selectedAssetId: '', mediaFolder: '', frame: 0 });
        s.artworkRevision++;
    }
    // Opens the first remaining project, or shows the empty state when there is none (or it cannot be opened).
    async function openFirstProject(s) {
        const next = (s.projects || [])[0];
        if (next) {
            try { await loadProject(s, listedId(next)); } catch (_) { showNotice(s, 'loadFailed', 'Could not open this project.', true); }
        }
        if (s.project || s.disposed) return;
        renderEmptyProject(s);
        P.renderFilters(s);
        P.renderToolbar(s);
        P.renderJobs(s);
    }
    // Deletes a project after a confirm that names its size; the server cancels the project's running tasks. Deleting the
    // open project opens the first remaining one, or the empty state when none is left. The manager shows the new list.
    async function deleteStoredProject(s, id) {
        const item = storedItem(s, id);
        if (!item || s.disposed) return;
        if (s.readonly) { showNotice(s, 'readonly', 'Read-only mode prevents editing.', true); return; }
        const name = storedName(s, item), size = storageText(item);
        const copy = size
            ? fill(tr(s, 'deleteProjectCopy', '{{name}} and its {{size}} (media, exports and versions) are deleted. Running tasks are cancelled. This cannot be undone.'), { name, size })
            : fill(tr(s, 'deleteProjectCopyUnknown', '{{name}} and all its media, exports and versions are deleted. Running tasks are cancelled. This cannot be undone.'), { name });
        if (!(await s.ctx.confirmDialog(tr(s, 'deleteProjectTitle', 'Delete this project?'), copy)) || s.disposed) return;
        if (id === s.projectId) {
            // A save that is still running would answer 404 after the delete; let it finish first.
            clearTimeout(s.autosaveTimer);
            if (s.savePromise) await s.savePromise.catch(() => {});
        }
        try {
            await request(P.API + '/projects/' + encodeURIComponent(id), { method: 'DELETE', signal: s.abort.signal });
        } catch (error) {
            // Deleted elsewhere already: the new list drops it like a delete from here.
            if (error.status !== 404) { if (!s.disposed) showNotice(s, 'projectDeleteFailed', 'The project could not be deleted.', true); return; }
        }
        if (s.disposed) return;
        const wasOpen = id === s.projectId;
        if (wasOpen) leaveDeletedProject(s);
        try { await loadProjects(s, { sizes: true }); } catch (_) { s.projects = (s.projects || []).filter(entry => listedId(entry) !== id); }
        if (s.disposed) return;
        if (wasOpen) await openFirstProject(s);
        if (s.disposed) return;
        showNotice(s, 'projectDeleted', 'Project deleted.', false);
        openProjectManager(s, true).catch(() => {});
    }

    // ---- Duplicate ----

    const DUPLICATE_TEXT = { duplicateTooLarge: 'The copy would exceed this project\'s storage limit.', duplicateNoSpace: 'There is not enough free disk space for the copy.', duplicateFailed: 'The project could not be copied.' };
    function duplicateKey(code) { return code === 'duplicate_quota' ? 'duplicateTooLarge' : code === 'insufficient_disk_space' ? 'duplicateNoSpace' : 'duplicateFailed'; }
    // Polls one job of any project until it ends; the jobs part's pollJob is bound to the open project. Returns the job,
    // or null when the editor closed.
    async function watchJob(s, jobId) {
        while (!s.disposed) {
            const response = await request(P.API + '/jobs/' + encodeURIComponent(jobId), { signal: s.abort.signal });
            const job = response.body.job || response.body;
            if (P.TERMINAL.includes(job.status)) return job;
            await new Promise(resolve => window.setTimeout(resolve, 1200));
        }
        return null;
    }
    // Copies a saved project (project file, media and posters) in a background job. The size is shown first; the server
    // checks the quota and the free space.
    function openDuplicateProject(s, id) {
        const item = storedItem(s, id);
        if (!item || s.disposed) return;
        if (s.readonly) { showNotice(s, 'readonly', 'Read-only mode prevents editing.', true); return; }
        const name = storedName(s, item), size = storageText(item);
        const copy = size
            ? fill(tr(s, 'duplicateCopy', 'Copies the project file, its media and posters. Exports, previews and versions are not copied. The copy needs {{size}}.'), { size })
            : tr(s, 'duplicateCopyUnknown', 'Copies the project file, its media and posters. Exports, previews and versions are not copied. The size of the copy could not be measured.');
        const host = openModal(s, `<form class="vs-modal" data-duplicate-form role="dialog" aria-modal="true" aria-labelledby="vs-duplicate-title-${esc(s, s.id)}"><button type="button" class="vs-modal-close" data-modal-close aria-label="${esc(s, tr(s, 'close', 'Close'))}">${icon('close', 18)}</button><span class="vs-modal-icon">${icon('copy', 22)}</span><h2 id="vs-duplicate-title-${esc(s, s.id)}">${esc(s, tr(s, 'duplicateTitle', 'Duplicate project'))}</h2><p data-duplicate-size>${esc(s, copy)}</p><label class="vs-field vs-field-wide"><span>${esc(s, tr(s, 'duplicateNameLabel', 'Name of the copy'))}</span><input type="text" data-duplicate-name maxlength="120" required value="${esc(s, fill(tr(s, 'duplicateNameDefault', '{{name}} (copy)'), { name }).slice(0, 120))}"></label><div class="vs-modal-actions"><button type="button" data-modal-close>${esc(s, tr(s, 'cancel', 'Cancel'))}</button><button type="submit" class="vs-primary">${icon('copy', 16)}<span>${esc(s, tr(s, 'duplicateStart', 'Duplicate'))}</span></button></div></form>`);
        backToManager(s, host);
        const input = host.querySelector('[data-duplicate-name]');
        input.select();
        host.querySelector('[data-duplicate-form]').addEventListener('submit', event => {
            event.preventDefault();
            const copyName = input.value.trim().slice(0, 120);
            closeModal(s);
            if (copyName && !s.disposed) duplicateProject(s, id, copyName).catch(() => {});
        });
    }
    async function duplicateProject(s, id, name) {
        let job;
        try {
            const response = await request(P.API + '/projects/' + encodeURIComponent(id) + '/jobs', { method: 'POST', headers: { 'Content-Type': 'application/json', 'Idempotency-Key': P.idempotencyKey() }, body: JSON.stringify({ kind: 'duplicate', name }), signal: s.abort.signal });
            job = response.body.job || response.body;
        } catch (error) {
            if (s.disposed) return;
            const key = duplicateKey(error.body && error.body.code);
            showNotice(s, key, DUPLICATE_TEXT[key], true);
            openProjectManager(s, true).catch(() => {});
            return;
        }
        // The jobs list of the open project may show the copy too; the notices below are its only announcement.
        s.notifiedJobs.add(job.id);
        showNotice(s, 'duplicateStarted', 'The copy is being created. It appears in the project list when it is ready.', false);
        openProjectManager(s, true).catch(() => {});
        try {
            const done = await watchJob(s, job.id);
            if (!done || s.disposed) return;
            if (done.status === 'succeeded') {
                await loadProjects(s, { sizes: true }).catch(() => {});
                if (s.disposed) return;
                refreshManagerList(s);
                showNotice(s, 'duplicateDone', 'Project copied.', false);
            } else if (done.status !== 'cancelled') {
                const key = duplicateKey(done.error);
                showNotice(s, key, DUPLICATE_TEXT[key], true);
            }
        } catch (_) {
            if (!s.disposed) showNotice(s, 'duplicateFailed', DUPLICATE_TEXT.duplicateFailed, true);
        }
    }

    // ---- Versions of the open project ----

    const VERSION_KINDS = { auto: ['versionAuto', 'Automatic'], manual: ['versionManual', 'Saved version'], before_restore: ['versionBeforeRestore', 'Before restore'] };
    const RESTORE_TEXT = { versionNotFound: 'That version no longer exists.', versionMediaGone: 'This version uses media that was removed.', versionImportBusy: 'Media is still being imported. Try again when it is ready.', projectStorageFull: 'This project has reached its storage limit. Remove media or use another project.', versionRestoreFailed: 'The version could not be restored.' };
    function versionRow(s, version) {
        const kind = tr(s, ...(VERSION_KINDS[version.kind] || VERSION_KINDS.auto));
        const date = new Date(version.created_at);
        const facts = [version.label ? kind : '', Number.isNaN(date.getTime()) ? '' : date.toLocaleString(P.uiLocale(), { dateStyle: 'medium', timeStyle: 'short' }), P.formatBytes(version.size_bytes, P.uiLocale())].filter(Boolean).join(' · ');
        return `<li class="vs-version-row" data-version-row="${esc(s, version.id)}"><span class="vs-version-main"><strong>${esc(s, version.label || kind)}</strong><small>${esc(s, facts)}</small></span><button type="button" data-version-restore="${esc(s, version.id)}"${s.readonly ? ' disabled' : ''}>${esc(s, tr(s, 'restoreVersion', 'Restore'))}</button></li>`;
    }
    // Automatic, manual and before-restore copies of the saved document, newest first (the server lists the oldest first).
    async function renderVersions(s, host) {
        const panel = host && host.querySelector('[data-manager-versions]');
        if (!panel || !s.projectId || !s.project) return;
        const projectId = s.projectId, epoch = s.projectEpoch;
        const title = `<h3>${esc(s, tr(s, 'versionsTitle', 'Versions'))} · ${esc(s, s.project.name || tr(s, 'untitled', 'Untitled project'))}</h3>`;
        let versions;
        try {
            versions = (await request(P.API + '/projects/' + encodeURIComponent(projectId) + '/versions', { signal: s.abort.signal })).body.versions || [];
        } catch (_) {
            if (panel.isConnected) panel.innerHTML = `${title}<p class="vs-hint">${esc(s, tr(s, 'versionsUnavailable', 'Versions could not be loaded.'))}</p>`;
            return;
        }
        if (!panel.isConnected || s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
        const rows = versions.slice().reverse().map(version => versionRow(s, version)).join('');
        const off = s.readonly ? ' disabled' : '';
        panel.innerHTML = `${title}<form class="vs-version-form" data-version-form><label class="vs-field"><span>${esc(s, tr(s, 'versionLabel', 'Label (optional)'))}</span><input type="text" data-version-label maxlength="120"${off}></label><button type="submit"${off}>${esc(s, tr(s, 'saveVersion', 'Save version now'))}</button></form>${rows ? `<ul class="vs-version-list">${rows}</ul>` : `<p class="vs-hint">${esc(s, tr(s, 'versionsEmpty', 'No versions yet.'))}</p>`}`;
        panel.querySelector('[data-version-form]').addEventListener('submit', event => {
            event.preventDefault();
            event.target.querySelector('[type="submit"]').disabled = true;
            saveVersion(s, host, panel.querySelector('[data-version-label]').value).catch(() => {});
        });
        panel.querySelectorAll('[data-version-restore]').forEach(button => button.addEventListener('click', () => { restoreVersion(s, button.dataset.versionRestore, host).catch(() => {}); }));
    }
    // "Save version now": the server copies the saved document, so pending titles and edits are saved first.
    async function saveVersion(s, host, rawLabel) {
        if (s.disposed || !s.projectId) return;
        if (s.readonly) { showNotice(s, 'readonly', 'Read-only mode prevents editing.', true); return; }
        const projectId = s.projectId, label = String(rawLabel || '').trim().slice(0, 120);
        await P.flushArtwork(s);
        if (!(await saveUntilClean(s)) || projectId !== s.projectId) { renderVersions(s, host).catch(() => {}); return; }
        try {
            await request(P.API + '/projects/' + encodeURIComponent(projectId) + '/versions', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(label ? { label } : {}), signal: s.abort.signal });
            showNotice(s, 'versionSaved', 'Version saved.', false);
        } catch (error) {
            if (s.disposed) return;
            if (error.status === 413) showNotice(s, 'projectStorageFull', RESTORE_TEXT.projectStorageFull, true);
            else showNotice(s, 'versionSaveFailed', 'The version could not be saved.', true);
        }
        await renderVersions(s, host);
    }
    // Restores a version of the open project. The editor must be clean: it saves first and refuses when it stays dirty.
    // The server keeps the replaced document as a before_restore version and answers the new ETag; the editor then opens
    // the restored project like a project switch (new ETag, fresh undo history).
    async function restoreVersion(s, versionId, host) {
        if (s.disposed || !s.projectId) return;
        if (s.readonly) { showNotice(s, 'readonly', 'Read-only mode prevents editing.', true); return; }
        if (!(await s.ctx.confirmDialog(tr(s, 'restoreVersionTitle', 'Restore this version?'), tr(s, 'restoreVersionCopy', 'Your changes are saved first, and the current state is kept as a version.'))) || s.disposed) return;
        const projectId = s.projectId;
        await P.flushArtwork(s);
        const clean = await saveUntilClean(s);
        if (s.disposed || projectId !== s.projectId) return;
        if (!clean || s.dirty) { showNotice(s, 'versionRestoreDirty', 'Save your changes before restoring a version.', true); return; }
        const epoch = s.projectEpoch;
        try {
            await request(P.API + '/projects/' + encodeURIComponent(projectId) + '/versions/' + encodeURIComponent(versionId) + '/restore', { method: 'POST', headers: { 'If-Match': s.etag }, signal: s.abort.signal });
        } catch (error) {
            if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
            if (error.status === 412 || error.status === 428) { s.conflict = true; await showConflict(s); return; }
            const code = error.body && error.body.code;
            const key = code === 'version_not_found' ? 'versionNotFound' : code === 'project_asset_invalid' ? 'versionMediaGone' : code === 'version_restore_dirty' ? 'versionImportBusy' : error.status === 413 ? 'projectStorageFull' : 'versionRestoreFailed';
            showNotice(s, key, RESTORE_TEXT[key], true);
            return;
        }
        if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
        if (host) host.closest('[data-modal-host]')?.replaceChildren();
        try { await loadProject(s, projectId); }
        catch (_) { showNotice(s, 'loadFailed', 'Could not load the current server version.', true); return; }
        showNotice(s, 'versionRestored', 'Version restored.', false);
    }

    function cameraVideoPath(path) {
        return typeof path === 'string' && /^Videos\/[^/]+\.(webm|mp4|mov|m4v|mkv)$/i.test(path) && path.indexOf('..') < 0 ? path : '';
    }
    function receiveCameraImport(s, context) {
        if (!s || s.disposed || !context || context.sessionRestore) return;
        const path = cameraVideoPath(context.importPath);
        const token = typeof context.importToken === 'string' ? context.importToken : '';
        if (!path || !/^[A-Za-z0-9-]{8,80}$/.test(token)) return;
        if (s.importOffer && s.importOffer.token === token) return;
        if (s.importOffer || s.q('[data-modal-host] .vs-modal, [data-conflict-host] .vs-modal')) {
            showNotice(s, 'cameraImportBusy', 'Finish the open project choice before sending another clip.', true);
            return;
        }
        s.importOffer = { token, path, key: (crypto.randomUUID ? crypto.randomUUID() : token), phase: 'pending' };
        if (s.studioReady) beginCameraImport(s);
    }
    function beginCameraImport(s) {
        const offer = s.importOffer;
        if (!offer || offer.phase !== 'pending' || s.disposed) return;
        const status = s.status;
        if (!status || s.featureDisabled || !status.enabled || !status.desktop_enabled || !status.ffmpeg_ready) {
            showNotice(s, 'cameraImportUnavailable', 'Video Studio is unavailable. The video stays in Videos.', true);
            s.importOffer = null;
            return;
        }
        if (s.readonly) {
            showNotice(s, 'cameraImportReadonly', 'Read-only mode prevents adding this clip. The video stays in Videos.', true);
            s.importOffer = null;
            return;
        }
        offer.phase = 'choosing';
        renderCameraImport(s);
    }
    function renderCameraImport(s) {
        const offer = s.importOffer;
        if (!offer || s.disposed || offer.phase === 'importing' || offer.phase === 'creating') return;
        const rows = (s.projects || []).map(item => {
            const id = listedId(item);
            if (!id) return '';
            const name = (listedId(item) === s.projectId && s.project && s.project.name) || (item.project && item.project.name) || tr(s, 'untitled', 'Untitled project');
            return `<button type="button" data-camera-project="${esc(s, id)}">${esc(s, name)}</button>`;
        }).join('');
        const host = openModal(s, `<section class="vs-modal" role="dialog" aria-modal="true" aria-labelledby="vs-cam-title-${esc(s, s.id)}" data-camera-import><h2 id="vs-cam-title-${esc(s, s.id)}">${esc(s, tr(s, 'cameraImportTitle', 'Add this clip'))}</h2><p>${esc(s, tr(s, 'cameraImportIntro', 'Choose a project. The clip goes into its media library. The timeline stays as it is.'))}</p><div class="vs-manager-list" role="list">${rows || `<p class="vs-hint">${esc(s, tr(s, 'cameraImportEmpty', 'No projects yet. Create one to add this clip.'))}</p>`}</div><div class="vs-modal-actions"><button type="button" data-camera-cancel>${esc(s, tr(s, 'cancel', 'Cancel'))}</button><button type="button" data-camera-new class="vs-primary">${esc(s, tr(s, 'cameraImportNew', 'New project'))}</button></div></section>`);
        const dialog = host.querySelector('[data-camera-import]');
        dialog.addEventListener('click', event => {
            event.stopPropagation();
            const projectBtn = event.target.closest('[data-camera-project]');
            if (projectBtn) { chooseCameraProject(s, projectBtn.dataset.cameraProject); return; }
            if (event.target.closest('[data-camera-new]')) { startCameraNewProject(s); return; }
            if (event.target.closest('[data-camera-cancel]')) cancelCameraImport(s);
        });
        dialog.addEventListener('keydown', event => {
            if (event.key !== 'Escape') return;
            event.preventDefault();
            event.stopPropagation();
            cancelCameraImport(s);
        });
        (dialog.querySelector('[data-camera-project]') || dialog.querySelector('[data-camera-new]')).focus();
    }
    function cancelCameraImport(s) {
        if (!s.importOffer || s.importOffer.phase === 'importing') return;
        s.importOffer = null;
        s.cameraImportAwaitCreate = '';
        s.q('[data-modal-host]')?.replaceChildren();
        if (s.app) s.app.focus({ preventScroll: true });
        showNotice(s, 'cameraImportKept', 'Import cancelled. The video stays in Videos.', false);
    }
    async function chooseCameraProject(s, targetId) {
        const offer = s.importOffer;
        if (!offer || offer.phase !== 'choosing' || !targetId) return;
        offer.phase = 'importing';
        s.q('[data-modal-host]')?.replaceChildren();
        if (targetId !== s.projectId) {
            await switchProject(s, targetId);
            if (s.disposed || !s.importOffer || s.importOffer.token !== offer.token) return;
            if (s.projectId !== targetId) {
                offer.phase = 'choosing';
                const missing = !(s.projects || []).some(item => listedId(item) === targetId);
                showNotice(s, missing ? 'cameraImportMissing' : 'cameraImportSaveFailed', 'The project was not changed. The video stays in Videos.', true);
                renderCameraImport(s);
                return;
            }
        }
        await runCameraImport(s, offer);
    }
    async function runCameraImport(s, offer) {
        let ok = false;
        try { ok = await P.importDesktopFile(s, { sourcePath: offer.path, idempotencyKey: offer.key }); }
        catch (_) { ok = false; }
        if (s.disposed || !s.importOffer || s.importOffer.token !== offer.token) return;
        if (ok) {
            s.importOffer = null;
            s.q('[data-modal-host]')?.replaceChildren();
            if (P.setTab) P.setTab(s, 'media');
            return;
        }
        offer.phase = 'choosing';
        renderCameraImport(s);
    }
    async function startCameraNewProject(s) {
        const offer = s.importOffer;
        if (!offer || offer.phase !== 'choosing') return;
        offer.phase = 'creating';
        s.cameraImportAwaitCreate = offer.token;
        await openNewProject(s);
        if (s.disposed || !s.importOffer || s.importOffer.token !== offer.token) return;
        if (s.cameraImportAwaitCreate !== offer.token) return;
        const form = s.q('[data-new-project-form]');
        if (!form) {
            s.cameraImportAwaitCreate = '';
            offer.phase = 'choosing';
            renderCameraImport(s);
            return;
        }
        s.q('[data-modal-host]').querySelectorAll('[data-modal-close]').forEach(button => {
            button.addEventListener('click', () => {
                window.setTimeout(() => {
                    if (s.disposed || !s.importOffer || s.importOffer.token !== offer.token) return;
                    if (s.cameraImportAwaitCreate !== offer.token) return;
                    s.cameraImportAwaitCreate = '';
                    s.importOffer.phase = 'choosing';
                    renderCameraImport(s);
                }, 0);
            });
        });
    }
    async function finishCameraImport(s) {
        const token = s.cameraImportAwaitCreate;
        const offer = s.importOffer;
        if (!token || !offer || offer.token !== token || s.disposed) return;
        s.cameraImportAwaitCreate = '';
        if (!s.projectId) {
            offer.phase = 'choosing';
            renderCameraImport(s);
            return;
        }
        offer.phase = 'importing';
        await runCameraImport(s, offer);
    }
    function resumeCameraImport(s) {
        if (!s.importOffer || s.importOffer.token !== s.cameraImportAwaitCreate) return;
        s.cameraImportAwaitCreate = '';
        s.importOffer.phase = 'choosing';
        renderCameraImport(s);
    }

    Object.assign(P, { openProjectSettings, openNewProject, openProjectManager, openRenameStoredProject, deleteStoredProject, firstVisualAsset, handleManagerClick, openDuplicateProject, renderVersions, restoreVersion, watchJob, receiveCameraImport, beginCameraImport, finishCameraImport, resumeCameraImport });
})();
