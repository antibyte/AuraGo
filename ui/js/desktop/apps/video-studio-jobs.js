(function () {
    'use strict';

    // Background jobs, project refresh, export and AI generation dialogs.
    const P = window.VideoStudioParts;
    const { API, TERMINAL } = P;
    const I = (...args) => P.I(...args);
    const T = (...args) => P.T(...args);
    const canonicalProject = (...args) => P.canonicalProject(...args);
    const clamp = (...args) => P.clamp(...args);
    const closePopovers = (...args) => P.closePopovers(...args);
    const esc = (...args) => P.esc(...args);
    const hydrateProject = (...args) => P.hydrateProject(...args);
    const icon = (...args) => P.icon(...args);
    const idempotencyKey = (...args) => P.idempotencyKey(...args);
    const mutate = (...args) => P.mutate(...args);
    const projectDuration = (...args) => P.projectDuration(...args);
    const renderUI = (...args) => P.renderUI(...args);
    const request = (...args) => P.request(...args);
    const saveProject = (...args) => P.saveProject(...args);
    const showConflict = (...args) => P.showConflict(...args);
    const showNotice = (...args) => P.showNotice(...args);
    const tr = (...args) => P.tr(...args);

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
        if (s.notifiedJobs.has(job.id) || s.jobLocalStops.has(job.id) || s.disposed) return;
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
            response = await request(API + '/projects/' + encodeURIComponent(projectId) + '/jobs', { method: 'POST', headers: { 'Content-Type': 'application/json', 'Idempotency-Key': key, 'If-Match': s.etag }, body: JSON.stringify(payload), signal: s.abort.signal });
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
        catch (error) {
            if (epoch !== s.projectEpoch || projectId !== s.projectId) return;
            // 409 job_finished: it ended or is already saving its result; polling announces the outcome.
            if (error.status === 409 && error.body && error.body.code === 'job_finished') { await loadJobs(s).catch(() => {}); return; }
            showNotice(s, 'cancelFailed', 'Could not cancel the task.', true);
        }
    }
    // A refresh never overtakes a save: it waits for a running PUT and reads again if one started meanwhile.
    async function refreshProject(s) {
        if (!s.projectId || s.disposed) return;
        const projectId = s.projectId, epoch = s.projectEpoch;
        let response = null;
        for (let attempt = 0; attempt < 4 && !response; attempt++) {
            if (s.savePromise) await s.savePromise.catch(() => {});
            if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
            const generation = s.saveGeneration || 0;
            const candidate = await request(API + '/projects/' + encodeURIComponent(projectId), { signal: s.abort.signal });
            if (!s.savePromise && (s.saveGeneration || 0) === generation) response = candidate;
        }
        if (!response || s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
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
            await P.flushArtwork(s);
            if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
            if (!(await P.saveUntilClean(s))) return;
            if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
            const start = () => createJob(s, { kind: 'render' }, projectId, epoch);
            try {
                try { await start(); }
                catch (error) {
                    // After "Replace latest" the project is saved with the observed ETag: start the export once more.
                    const resolved = (error.status === 412 || error.status === 428) && !s.conflict && !s.dirty && epoch === s.projectEpoch && projectId === s.projectId;
                    if (!resolved) throw error;
                    await start();
                }
                if (epoch === s.projectEpoch && projectId === s.projectId) showNotice(s, 'exportStarted', 'Export started. You can keep editing while it runs.', false);
            } catch (_) { if (epoch === s.projectEpoch && projectId === s.projectId) showNotice(s, 'exportFailed', 'Could not start the export.', true); }
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

    Object.assign(P, { loadJobs, jobFailure, exportName, announceJob, visibleJobs, renderJobsButton, renderJobs, safeSameOrigin, createJob, pollJob, cancelJob, refreshProject, openModal, openExport, openAI });
})();
