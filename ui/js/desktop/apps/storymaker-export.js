// Storymaker app export: the toolbar button and its panel. The panel says what is missing before anything starts (FFmpeg, a
// browser, read-only, errors in the story, a story longer than the limit) and what still works; a start sends the saved story's
// ETag with an idempotency key; progress comes from the project's server-sent events (a poll of the job runs as a safety net
// while a job is active); the result is the project-folder path of the MP4 with "show in Files" and copy. Job errors are shown
// by their stable code. Server texts are shown as text only. Classic script, publishes S.createExport.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};
    const ACTIVE = ['queued', 'running'];
    const POLL_MS = 2500;
    const PHASES = ['starting', 'compiling', 'mixing', 'rendering', 'encoding', 'writing'];

    function dirOf(path) {
        const text = String(path || '');
        const cut = text.lastIndexOf('/');
        return cut > 0 ? text.slice(0, cut) : '';
    }

    // inst: {t, api, bus, status, readonly, projectId, etag, story, report, issues, ctx, flushEdits(), showIssues(), notify(text, kind)}
    function createExport(inst) {
        const { el, icon } = S;
        const t = inst.t;
        let job = null;
        let open = false;
        let starting = false;
        let cancelling = false;
        let pollTimer = 0;
        let copied = 0;

        const buttonLabel = el('span', { class: 'stm-export-label', text: t('storymaker.export.button') });
        const button = el('button', { type: 'button', class: 'stm-btn stm-btn-primary stm-export-btn', dataset: { stm: 'export', state: 'idle' }, ariaHaspopup: 'dialog', ariaExpanded: 'false', onClick: () => toggle() }, icon('export'), buttonLabel);
        const panel = el('div', { class: 'stm-export-panel', role: 'dialog', ariaLabel: t('storymaker.export.title'), hidden: true, dataset: { stm: 'export-panel' } });
        const root = el('div', { class: 'stm-export' }, button, panel);

        function limits() { return (inst.status && inst.status.limits) || {}; }
        function isActive() { return !!job && ACTIVE.indexOf(job.status) >= 0; }

        function capabilityRow(ok, name, detail, extra) {
            return el('li', { class: 'stm-cap', dataset: { ok: String(ok) } },
                icon(ok ? 'check' : 'alert'),
                el('div', { class: 'stm-cap-text' },
                    el('strong', { text: name }),
                    el('span', { class: 'stm-muted', text: detail }),
                    extra || null));
        }

        // What blocks an export, as {key, ...vars} reasons in the order the panel lists them.
        function blockers() {
            const list = [];
            const status = inst.status || {};
            if (inst.readonly) list.push('readonly');
            if (!(status.ffmpeg && status.ffmpeg.ready)) list.push('ffmpeg');
            if (S.model.browserState(status) === 'missing') list.push('browser');
            const counts = S.model.issueCounts(S.model.visibleIssues(S.model.mergeIssues(inst.issues, inst.report ? inst.report.issues : [])));
            if (counts.error > 0) list.push('errors');
            const max = Number(limits().max_duration_seconds) || 0;
            if (max > 0 && inst.report && inst.report.duration_ms / 1000 > max) list.push('too_long');
            if (!inst.report || !(inst.report.frame_count > 0)) list.push('not_ready');
            return list;
        }

        function capabilities() {
            const status = inst.status || {};
            const ffmpeg = status.ffmpeg || {};
            const browser = status.browser || {};
            const list = el('ul', { class: 'stm-caps', dataset: { stm: 'export-caps' } });
            list.appendChild(capabilityRow(!!ffmpeg.ready, t('storymaker.export.cap_ffmpeg'),
                ffmpeg.ready ? (ffmpeg.version ? t('storymaker.export.ready_version', { version: String(ffmpeg.version).slice(0, 60) }) : t('storymaker.export.ready')) : t('storymaker.export.ffmpeg_missing'),
                !ffmpeg.ready && ffmpeg.issue ? el('code', { class: 'stm-cap-issue', text: String(ffmpeg.issue).slice(0, 400) }) : null));
            const state = S.model.browserState(status);
            // "download": no browser is ready but the first export fetches one; that is a note, not a blocker.
            list.appendChild(capabilityRow(state !== 'missing', t('storymaker.export.cap_browser'),
                state === 'ready' ? t('storymaker.export.browser_ready', { source: sourceLabel(browser.source) }) : state === 'download' ? t('storymaker.export.browser_download') : t('storymaker.export.browser_missing'),
                state === 'missing' && browser.issue ? el('code', { class: 'stm-cap-issue', text: String(browser.issue).slice(0, 400) }) : null));
            return list;
        }

        function sourceLabel(source) {
            if (source === 'config' || source === 'system' || source === 'pinned') return t('storymaker.export.source_' + source);
            return t('storymaker.export.source_unknown');
        }

        function blockerText(reason) {
            const max = Number(limits().max_duration_seconds) || 0;
            switch (reason) {
                case 'readonly': return t('storymaker.export.blocked_readonly');
                case 'ffmpeg': return t('storymaker.export.blocked_ffmpeg');
                case 'browser': return t('storymaker.export.blocked_browser');
                case 'errors': return t('storymaker.export.blocked_errors');
                case 'too_long': return t('storymaker.export.blocked_too_long', { max: S.format.seconds(max) });
                default: return t('storymaker.export.blocked_not_ready');
            }
        }

        function progressView() {
            const fraction = job && Number.isFinite(job.progress) ? S.clamp(job.progress, 0, 1) : 0;
            const phase = job && PHASES.indexOf(job.phase) >= 0 ? t('storymaker.export.phase_' + job.phase) : t('storymaker.export.phase_starting');
            const frames = job && job.frames_total > 0 ? t('storymaker.export.frames', { done: job.frames_done || 0, total: job.frames_total }) : '';
            const waiting = job && job.status === 'queued';
            return el('div', { class: 'stm-job', dataset: { stm: 'export-progress', status: job ? job.status : '' } },
                el('div', { class: 'stm-job-line' }, el('strong', { text: waiting ? t('storymaker.export.queued') : phase }), el('span', { class: 'stm-muted', text: S.format.percent(fraction) })),
                el('progress', { class: 'stm-progress', max: '1', value: String(fraction), ariaLabel: t('storymaker.export.title') }),
                frames ? el('p', { class: 'stm-hint', text: frames }) : null,
                el('div', { class: 'stm-actions' },
                    el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'export-cancel' }, disabled: cancelling || null, onClick: () => cancel() }, icon('stop'), el('span', { text: cancelling ? t('storymaker.export.cancelling') : t('storymaker.export.cancel') }))));
        }

        function resultView() {
            const output = job.output || {};
            const dir = dirOf(output.path);
            return el('div', { class: 'stm-job', dataset: { stm: 'export-done', status: 'succeeded' } },
                el('p', { class: 'stm-job-title' }, icon('check'), el('strong', { text: t('storymaker.export.done') })),
                el('p', { class: 'stm-muted', text: t('storymaker.export.output') }),
                el('code', { class: 'stm-path', dataset: { stm: 'export-path' }, text: String(output.path || '') }),
                output.size ? el('p', { class: 'stm-hint', text: S.format.bytes(output.size) }) : null,
                el('div', { class: 'stm-actions' },
                    el('button', { type: 'button', class: 'stm-btn stm-btn-primary', dataset: { stm: 'export-show' }, disabled: !dir || typeof inst.ctx.openApp !== 'function' || null, onClick: () => { if (dir) inst.ctx.openApp('files', { path: dir }); } }, icon('folder'), el('span', { text: t('storymaker.export.show_in_files') })),
                    el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'export-copy' }, onClick: () => copyPath(output.path) }, icon('copy'), el('span', { text: copied && Date.now() - copied < 2500 ? t('storymaker.export.copied') : t('storymaker.export.copy_path') })),
                    el('button', { type: 'button', class: 'stm-btn stm-btn-quiet', dataset: { stm: 'export-again' }, onClick: () => { job = null; paint(); } }, el('span', { text: t('storymaker.export.again') }))));
        }

        function failedView() {
            const message = job.status === 'cancelled' ? t('storymaker.export.cancelled') : job.status === 'interrupted' ? t('storymaker.export.interrupted') : S.errors.describeJob(t, job.error);
            return el('div', { class: 'stm-job', dataset: { stm: 'export-failed', status: job.status } },
                el('p', { class: 'stm-job-title stm-job-bad' }, icon('alert'), el('strong', { text: message })),
                job.error && job.error.message && job.status === 'failed' ? el('code', { class: 'stm-cap-issue', text: String(job.error.message).slice(0, 600) }) : null,
                el('div', { class: 'stm-actions' }, el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'export-again' }, onClick: () => { job = null; paint(); } }, icon('refresh'), el('span', { text: t('storymaker.export.again') }))));
        }

        function startView() {
            const reasons = blockers();
            const blocked = reasons.length > 0;
            const view = el('div', { class: 'stm-job', dataset: { stm: 'export-ready', blocked: String(blocked) } });
            if (blocked) {
                const list = el('ul', { class: 'stm-blockers', dataset: { stm: 'export-blockers' } });
                reasons.forEach(reason => list.appendChild(el('li', { dataset: { reason }, text: blockerText(reason) })));
                view.appendChild(list);
                if (reasons.indexOf('ffmpeg') >= 0 || reasons.indexOf('browser') >= 0) view.appendChild(el('p', { class: 'stm-hint', text: t('storymaker.export.still_works') }));
                if (reasons.indexOf('errors') >= 0) view.appendChild(el('button', { type: 'button', class: 'stm-btn stm-btn-quiet', dataset: { stm: 'export-issues' }, onClick: () => { close(); inst.showIssues(); } }, icon('alert'), el('span', { text: t('storymaker.issues.show') })));
            } else {
                const seconds = inst.report ? inst.report.duration_ms / 1000 : 0;
                const size = S.model.exportSize(inst.story.format ? inst.story.format.aspect : '');
                view.appendChild(el('p', { class: 'stm-muted', text: t('storymaker.export.summary', { duration: S.format.clock(inst.report.frame_count, inst.report.fps), format: (inst.story.format ? inst.story.format.aspect : '') + ' · ' + (inst.report.fps || '') + ' fps', width: size[0], height: size[1] }) }));
                if (seconds > 0) view.appendChild(el('p', { class: 'stm-hint', text: t('storymaker.export.silent_note') }));
            }
            view.appendChild(el('div', { class: 'stm-actions' },
                el('button', { type: 'button', class: 'stm-btn stm-btn-primary', dataset: { stm: 'export-start' }, disabled: blocked || starting || null, onClick: () => start() }, icon('film'), el('span', { text: starting ? t('storymaker.export.starting') : t('storymaker.export.start') }))));
            return view;
        }

        function paint() {
            const state = isActive() ? 'running' : job ? job.status : 'idle';
            button.dataset.state = state;
            const fraction = isActive() && job && Number.isFinite(job.progress) ? S.clamp(job.progress, 0, 1) : 0;
            button.style.setProperty('--stm-export-progress', Math.round(fraction * 100) + '%');
            buttonLabel.textContent = isActive() ? t('storymaker.export.button_running', { percent: Math.round(fraction * 100) }) : t('storymaker.export.button');
            button.setAttribute('aria-expanded', String(open));
            panel.hidden = !open;
            if (!open) return;
            S.clear(panel);
            panel.appendChild(el('div', { class: 'stm-export-head' }, el('h3', { text: t('storymaker.export.title') }),
                el('button', { type: 'button', class: 'stm-btn stm-btn-quiet stm-btn-icon', ariaLabel: t('storymaker.close'), title: t('storymaker.close'), dataset: { stm: 'export-close' }, onClick: () => close() }, icon('close'))));
            panel.appendChild(capabilities());
            if (isActive()) panel.appendChild(progressView());
            else if (job && job.status === 'succeeded') panel.appendChild(resultView());
            else if (job) panel.appendChild(failedView());
            else panel.appendChild(startView());
        }

        // ---- job tracking ------------------------------------------------------------------------------------------------

        function schedulePoll() {
            clearTimeout(pollTimer);
            pollTimer = 0;
            if (!isActive()) return;
            pollTimer = setTimeout(async () => {
                pollTimer = 0;
                if (!job) return;
                try { apply(await inst.api.getJob(job.id)); } catch (err) {
                    if (err && err.aborted) return;
                    schedulePoll();
                }
            }, POLL_MS);
        }

        // apply(next, adopt): a job event of this project. Without a tracked job only an active one is taken up (a start by
        // another window); a finished one is shown when this panel started it (adopt).
        function apply(next, adopt) {
            if (!next || typeof next !== 'object' || typeof next.id !== 'string') return;
            if (next.project_id && inst.projectId && next.project_id !== inst.projectId) return;
            if (job && job.id !== next.id && (isActive() || ACTIVE.indexOf(next.status) < 0)) return;
            if (!job && !adopt && ACTIVE.indexOf(next.status) < 0) return;
            if (job && job.id !== next.id) job = null;
            const was = job ? job.status : '';
            job = next;
            if (!isActive()) cancelling = false;
            if (was && was !== job.status && !isActive()) {
                inst.notify(job.status === 'succeeded' ? t('storymaker.export.done') : job.status === 'cancelled' ? t('storymaker.export.cancelled') : job.status === 'interrupted' ? t('storymaker.export.interrupted') : S.errors.describeJob(t, job.error), job.status === 'succeeded' ? 'info' : 'error');
            }
            paint();
            schedulePoll();
        }

        async function start() {
            if (starting || isActive() || blockers().length) return;
            starting = true;
            paint();
            try {
                const saved = await inst.flushEdits();
                if (!saved) return;
                if (blockers().length) return;
                const created = await inst.api.startRender(inst.projectId, inst.etag, S.uuid());
                job = null;
                apply(created, true);
            } catch (err) {
                if (err && err.aborted) return;
                if (err && err.status === 412) inst.bus.emit('conflict', err);
                inst.notify(S.errors.describe(t, err, 'render'), 'error');
            } finally {
                starting = false;
                paint();
            }
        }

        async function cancel() {
            if (!isActive() || cancelling) return;
            cancelling = true;
            paint();
            try {
                apply(await inst.api.cancelJob(job.id));
            } catch (err) {
                cancelling = false;
                inst.notify(S.errors.describe(t, err), 'error');
                paint();
            }
        }

        async function copyPath(path) {
            try {
                await navigator.clipboard.writeText(String(path || ''));
                copied = Date.now();
            } catch (_) {
                inst.notify(t('storymaker.export.copy_failed'), 'error');
                return;
            }
            paint();
            setTimeout(() => { if (open) paint(); }, 2600);
        }

        // ---- popover -----------------------------------------------------------------------------------------------------

        function onDocumentPointer(event) {
            if (open && !root.contains(event.target)) close(false);
        }
        function onDocumentKey(event) {
            if (open && event.key === 'Escape') { event.stopPropagation(); close(true); }
        }
        document.addEventListener('pointerdown', onDocumentPointer, true);
        document.addEventListener('keydown', onDocumentKey, true);

        function toggle() { if (open) close(true); else openPanel(); }
        function openPanel() { open = true; paint(); const first = panel.querySelector('button:not([disabled])'); if (first) first.focus(); }
        function close(restoreFocus) { open = false; paint(); if (restoreFocus) button.focus(); }

        const off = [
            inst.bus.on('job', next => apply(next, false)),
            inst.bus.on('report', () => { if (open) paint(); }),
            inst.bus.on('issues', () => { if (open) paint(); }),
            inst.bus.on('status', () => { if (open) paint(); }),
            inst.bus.on('readonly', () => { if (open) paint(); })
        ];
        paint();

        return {
            el: root,
            open: openPanel,
            close,
            job: () => job,
            apply,
            dispose() {
                clearTimeout(pollTimer);
                document.removeEventListener('pointerdown', onDocumentPointer, true);
                document.removeEventListener('keydown', onDocumentKey, true);
                off.forEach(fn => fn());
            }
        };
    }

    S.createExport = createExport;
})();
