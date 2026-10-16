(function () {
    'use strict';

    // Render range exactly (spec §7.8, plan index §4.15). The in/out range, or 5 s around the playhead, is rendered on
    // the server through the export pipeline (job kind range_render: 540 lines, at most 60 s, the export encoder) and
    // played over the live preview inside .vs-preview-mat, so it follows the mat into full screen. Rendered / Live
    // preview switch between the two at the same timeline frame. The job is polled here; the jobs list hides it
    // (visibleJobs). Closing the comparison (button or Escape) cancels a render that is still running; closing the
    // window or switching projects leaves the job to finish like every server job.
    const P = window.VideoStudioParts;
    const { API, TERMINAL } = P;
    const esc = (...args) => P.esc(...args);
    const fill = (...args) => P.fill(...args);
    const icon = (...args) => P.icon(...args);
    const showNotice = (...args) => P.showNotice(...args);
    const tr = (...args) => P.tr(...args);
    const Time = () => window.VideoStudioTime;
    const AROUND_SECONDS = 5;
    const MAX_SECONDS = 60; // Go: videostudio.MaxRangeRenderSeconds
    const FIRST_POLL_MS = 300;
    const POLL_MS = 1000;
    const MAX_POLL_ERRORS = 5;

    // Go's MaxRangeRenderFrames: 60 s rounded down to whole frames (1798 at 29.97 fps).
    function maxRangeFrames(project) { return Math.floor(MAX_SECONDS * Time().rate(project) + 1e-9); }
    // The frames to render as {start, end}, or {error: 'empty' | 'too_long'}. With both in/out points set: that range,
    // clamped to the enabled end (P.enabledEnd, Go's EnabledDuration) like the export. Otherwise, or when the clamped
    // points leave nothing: 5 s around the playhead inside [0, enabled end]. More than 60 s is refused, never clamped.
    function rangeRenderRange(s) {
        const project = s && s.project;
        const length = project ? P.enabledEnd(project) : 0;
        if (length <= 0) return { error: 'empty' };
        const io = P.inOut(s);
        if (io.in != null && io.out != null) {
            const start = Math.max(0, Math.min(io.in, length)), end = Math.max(0, Math.min(io.out, length));
            if (end > start) return end - start > maxRangeFrames(project) ? { error: 'too_long', start, end } : { start, end };
        }
        const span = Math.max(1, Time().secondsToFrames(AROUND_SECONDS, project));
        const frame = Math.max(0, Math.min(length, Math.round(Number(s.frame) || 0)));
        const start = Math.max(0, Math.min(frame - Math.floor(span / 2), length - span));
        return { start, end: Math.min(length, start + span) };
    }
    function overlay(s) { return s.q ? s.q('[data-range-overlay]') : null; }
    function rangeText(s, range) { return Time().formatTime(range.start, s.project) + '–' + Time().formatTime(range.end, s.project); }
    // The timeline frame the rendered video shows at `seconds`: start + round(seconds · fps), kept inside the range.
    function frameAt(s, seconds) {
        const range = s.rangeCompare.range;
        return Math.max(range.start, Math.min(range.end - 1, range.start + Math.round((Number(seconds) || 0) * Time().rate(s.project))));
    }
    // The video time that shows timeline frame `frame` (a frame outside the range shows the nearest end).
    function secondsAt(s, frame) {
        const range = s.rangeCompare.range;
        return Time().framesToSeconds(Math.max(range.start, Math.min(range.end - 1, Math.round(Number(frame) || 0))) - range.start, s.project);
    }
    function failureText(s, job) {
        const failure = P.jobFailure(job);
        return fill(tr(s, failure[0], failure[1]), failure[2]);
    }
    function resetVideo(s) {
        const video = overlay(s)?.querySelector('[data-range-video]');
        if (!video) return;
        try { video.pause(); video.removeAttribute('src'); video.load(); } catch (_) { /* detached element */ }
    }
    // Builds the overlay's controls once per opening; renderRangeOverlay only updates them, so focus survives updates.
    function ensureOverlay(s) {
        const host = overlay(s);
        if (!host || host.querySelector('[data-range-video]')) return host;
        const label = (key, fallback) => esc(s, tr(s, key, fallback));
        host.innerHTML = `<video class="vs-range-video" data-range-video controls playsinline preload="auto" aria-label="${label('rangeRendered', 'Rendered')}" hidden></video><div class="vs-range-bar" role="group" aria-label="${label('job_range_render', 'Range check')}"><span class="vs-range-status" data-range-status role="status" aria-live="polite"></span><progress data-range-progress max="100" value="0" aria-label="${label('job_range_render', 'Range check')}"></progress><div class="vs-segmented vs-range-switch" role="group"><button type="button" data-action="range-view" data-view="rendered" aria-pressed="true">${label('rangeRendered', 'Rendered')}</button><button type="button" data-action="range-view" data-view="live" aria-pressed="false">${label('rangeLive', 'Live preview')}</button></div><button type="button" data-action="range-retry" hidden>${icon('loop', 15)}<span>${label('rangeRetry', 'Render again')}</span></button><button type="button" class="vs-icon-button" data-action="range-close" aria-label="${label('rangeClose', 'Close the comparison')}" title="${label('rangeClose', 'Close the comparison')}">${icon('close', 16)}</button></div>`;
        const video = host.querySelector('[data-range-video]');
        video.addEventListener('error', () => { if (s.rangeCompare && video.getAttribute('src')) { s.rangeCompare.videoError = true; renderRangeOverlay(s); } });
        video.addEventListener('loadedmetadata', () => { if (s.rangeCompare && s.rangeCompare.videoError) { s.rangeCompare.videoError = false; renderRangeOverlay(s); } });
        return host;
    }
    function renderRangeOverlay(s) {
        const rc = s.rangeCompare, host = overlay(s);
        if (!host) return;
        if (!rc || !rc.open) { host.hidden = true; return; }
        ensureOverlay(s);
        const ready = rc.status === 'ready', failed = rc.status === 'failed' || (ready && rc.videoError);
        const percent = Math.round(Math.max(0, Math.min(1, Number(rc.progress) || 0)) * 100);
        host.hidden = false;
        host.dataset.rangeState = rc.status;
        host.dataset.rangeView = rc.view;
        const status = host.querySelector('[data-range-status]'), progress = host.querySelector('[data-range-progress]'), video = host.querySelector('[data-range-video]');
        status.textContent = rc.status === 'rendering'
            ? fill(tr(s, 'rangeRendering', 'Rendering {{range}} exactly… {{percent}} %'), { range: rangeText(s, rc.range), percent: String(percent) })
            : failed ? (rc.failure || tr(s, 'rangeFailed', 'The range could not be rendered. Try again.')) : rangeText(s, rc.range);
        status.classList.toggle('is-error', failed);
        progress.hidden = rc.status !== 'rendering';
        progress.value = percent;
        host.querySelectorAll('[data-action="range-view"]').forEach(button => { button.disabled = !ready; button.setAttribute('aria-pressed', String(button.dataset.view === rc.view)); });
        host.querySelector('[data-action="range-retry"]').hidden = !failed;
        video.hidden = !ready || rc.view !== 'rendered';
        if (ready && rc.url && video.getAttribute('src') !== rc.url) {
            video.src = rc.url;
            try { video.currentTime = secondsAt(s, s.frame); } catch (_) { /* metadata still loading */ }
        }
        syncRangeRender(s);
    }
    // Shows the comparison for s.rangeCompare (set by startRangeRender) and adopts a polled range_render job: its
    // progress, its result or its failure. Focus moves into the overlay only when it opens.
    function openRangeCompare(s, job) {
        const rc = s.rangeCompare;
        if (!rc || s.disposed) return false;
        const opening = !rc.open;
        rc.open = true;
        if (job) {
            rc.job = job;
            rc.jobId = job.id || rc.jobId;
            rc.progress = Number(job.progress) || 0;
            const url = job.artifact ? P.safeSameOrigin(job.artifact.download_url) : '';
            if (job.status === 'succeeded' && url) Object.assign(rc, { status: 'ready', url, failure: null, videoError: false });
            else if (TERMINAL.includes(job.status)) Object.assign(rc, { status: 'failed', failure: failureText(s, job.status === 'succeeded' ? { status: 'failed', error: 'range_render_failed' } : job) });
            else rc.status = 'rendering';
        }
        renderRangeOverlay(s);
        if (opening) overlay(s)?.querySelector('[data-action="range-close"]')?.focus({ preventScroll: true });
        return true;
    }
    // Saves the project first, then asks for a range_render job bound to the saved ETag and polls it. `given` (Render
    // again) reuses an earlier range.
    async function startRangeRender(s, given) {
        if (!s.project || !s.projectId || s.disposed) return null;
        if (s.readonly) { showNotice(s, 'readonly', 'Read-only mode prevents editing.', true); return null; }
        if (s.rangeCompare && s.rangeCompare.open && s.rangeCompare.status === 'rendering') { overlay(s)?.querySelector('[data-action="range-close"]')?.focus({ preventScroll: true }); return null; }
        const range = given || rangeRenderRange(s);
        if (range.error === 'too_long') { showNotice(s, 'rangeTooLong', 'Render range exactly works on up to 60 seconds. Move the in and out points closer together.', true); return null; }
        if (range.error) { showNotice(s, 'invalidRange', 'The in/out range is not valid.', true); return null; }
        const projectId = s.projectId, epoch = s.projectEpoch, token = {};
        const previous = s.rangeCompare;
        s.rangeCompare = { open: !!(previous && previous.open), token, projectId, epoch, range: { start: range.start, end: range.end }, status: 'rendering', progress: 0, job: null, jobId: '', url: '', view: 'rendered', failure: null, videoError: false };
        const current = () => !s.disposed && s.rangeCompare && s.rangeCompare.token === token && projectId === s.projectId && epoch === s.projectEpoch;
        const fail = text => { if (current()) { s.rangeCompare.status = 'failed'; s.rangeCompare.failure = text; renderRangeOverlay(s); } };
        resetVideo(s);
        openRangeCompare(s, null);
        await P.flushArtwork(s);
        if (!current()) return null;
        if (!(await P.saveUntilClean(s))) { fail(tr(s, 'saveFailed', 'Save failed')); return null; }
        if (!current()) return null;
        const post = () => P.request(API + '/projects/' + encodeURIComponent(projectId) + '/jobs', { method: 'POST', headers: { 'Content-Type': 'application/json', 'Idempotency-Key': P.idempotencyKey(), 'If-Match': s.etag }, body: JSON.stringify({ kind: 'range_render', range: s.rangeCompare.range }), signal: s.abort.signal });
        let job;
        try {
            let response;
            try { response = await post(); }
            catch (error) {
                if (!(error.status === 412 || error.status === 428) || !current()) throw error;
                // As for an export: after "Replace latest" the project is saved with the observed ETag, so ask once more.
                s.conflict = true;
                await P.showConflict(s);
                if (!current() || s.conflict || s.dirty) throw error;
                response = await post();
            }
            job = response.body.job || response.body;
        } catch (error) {
            fail(failureText(s, { status: 'failed', error: error.body && error.body.code }));
            return null;
        }
        if (!current()) { cancelRemote(job.id); return null; }
        openRangeCompare(s, job);
        pollRangeRender(s, token, job.id).catch(() => {});
        return job;
    }
    async function pollRangeRender(s, token, jobId) {
        const live = () => !s.disposed && s.rangeCompare && s.rangeCompare.token === token && s.rangeCompare.projectId === s.projectId && s.rangeCompare.epoch === s.projectEpoch;
        let delay = FIRST_POLL_MS, errors = 0;
        while (live()) {
            await new Promise(resolve => window.setTimeout(resolve, delay));
            delay = POLL_MS;
            if (!live()) return;
            let job;
            try {
                const response = await P.request(API + '/jobs/' + encodeURIComponent(jobId), { signal: s.abort.signal });
                job = response.body.job || response.body;
                errors = 0;
            } catch (error) {
                if (!live()) return;
                if (error.status !== 404 && ++errors < MAX_POLL_ERRORS) continue;
                job = { id: jobId, status: 'failed', error: error.body && error.body.code };
            }
            if (!live()) return;
            openRangeCompare(s, job);
            if (TERMINAL.includes(job.status)) return;
        }
    }
    // Rendered / Live preview. Live seeks the preview to the frame the rendered video shows (start + round(t · fps));
    // Rendered puts the video at the playhead's frame. Returns false when nothing changed.
    function setRangeView(s, view) {
        const rc = s.rangeCompare, host = overlay(s);
        if (!rc || !rc.open || rc.status !== 'ready' || !host || (view !== 'rendered' && view !== 'live') || rc.view === view) return false;
        const video = host.querySelector('[data-range-video]');
        if (view === 'live') {
            if (video) video.pause();
            const frame = frameAt(s, video ? video.currentTime : 0);
            rc.view = 'live';
            renderRangeOverlay(s);
            if (s.preview) { s.preview.pause(); s.preview.seek(frame); }
        } else {
            if (s.preview && s.preview.isPlaying()) s.preview.pause();
            rc.view = 'rendered';
            renderRangeOverlay(s);
            if (video) { try { video.currentTime = secondsAt(s, s.frame); } catch (_) { /* metadata still loading */ } }
        }
        return true;
    }
    // Ends the comparison. A render that is still running is cancelled unless options.keepJob (window close, project
    // switch: server jobs outlive the editor). Focus returns to the transport button unless options.restoreFocus is false.
    function closeRangeCompare(s, options) {
        const rc = s.rangeCompare, opts = options || {};
        if (!rc) return false;
        s.rangeCompare = null;
        if (!opts.keepJob && rc.jobId && !(rc.job && TERMINAL.includes(rc.job.status))) cancelRemote(rc.jobId);
        resetVideo(s);
        const host = overlay(s);
        if (host) { host.hidden = true; host.replaceChildren(); delete host.dataset.rangeState; delete host.dataset.rangeView; }
        syncRangeRender(s);
        if (opts.restoreFocus !== false && rc.open && !s.disposed) {
            const button = s.q('[data-action="range-render"]');
            if (button && !button.disabled) button.focus({ preventScroll: true }); else if (s.app) s.app.focus({ preventScroll: true });
        }
        return true;
    }
    function cancelRemote(jobId) {
        P.request(API + '/jobs/' + encodeURIComponent(jobId) + '/cancel', { method: 'POST' }).catch(() => { /* already finished or gone */ });
    }
    // The transport button follows the editor: enabled with an open project, a clip to render, FFmpeg and write access;
    // busy while a range renders. A comparison of another project (switched or reloaded) closes without cancelling.
    function syncRangeRender(s) {
        const rc = s.rangeCompare;
        if (rc && (rc.projectId !== s.projectId || rc.epoch !== s.projectEpoch)) { closeRangeCompare(s, { restoreFocus: false, keepJob: true }); return; }
        const button = s.q && s.q('[data-action="range-render"]');
        if (!button) return;
        button.disabled = !(s.project && s.projectId && s.status && s.status.ffmpeg_ready) || !!s.readonly || !!s.featureDisabled || P.enabledEnd(s.project) <= 0;
        const busy = !!(rc && rc.open && rc.status === 'rendering');
        button.classList.toggle('is-busy', busy);
        button.setAttribute('aria-busy', String(busy));
    }
    function handleRangeClick(s, event) {
        const button = event.target.closest('[data-action^="range-"]');
        if (!button || button.disabled) return false;
        const action = button.dataset.action, rc = s.rangeCompare;
        if (action === 'range-render') startRangeRender(s).catch(() => {});
        else if (action === 'range-view') setRangeView(s, button.dataset.view || (rc && rc.view === 'live' ? 'rendered' : 'live'));
        else if (action === 'range-close') closeRangeCompare(s);
        else if (action === 'range-retry') startRangeRender(s, rc ? { start: rc.range.start, end: rc.range.end } : null).catch(() => {});
        else return false;
        return true;
    }
    // Escape closes the comparison (handleKeys checks dialogs, popovers and the narrow drawer first). Other plain keys on
    // the overlay's own controls (the video's native controls, the buttons) stay with them.
    function handleRangeKeys(s, event) {
        const rc = s.rangeCompare;
        if (!rc || !rc.open || s.stageMode === 'source') return false;
        if (event.key === 'Escape') { event.preventDefault(); closeRangeCompare(s); return true; }
        if (event.ctrlKey || event.metaKey || event.altKey) return false;
        return !!(event.target && typeof event.target.closest === 'function' && event.target.closest('[data-range-overlay]'));
    }

    Object.assign(P, { rangeRenderRange, startRangeRender, openRangeCompare, closeRangeCompare, setRangeView, syncRangeRender, handleRangeClick, handleRangeKeys });
})();
