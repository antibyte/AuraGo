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

    // A film is silent when the /narration answer holds no block, no music and no missing text and the report has no audio event.
    function isSilentFilm(audioInfo, report) {
        if (!audioInfo) return false;
        const narration = audioInfo.narration && typeof audioInfo.narration === 'object' ? audioInfo.narration : {};
        const spoken = Object.keys(narration).some(id => narration[id] && Array.isArray(narration[id].blocks) && narration[id].blocks.length > 0);
        const waiting = Array.isArray(audioInfo.missing) && audioInfo.missing.length > 0;
        const events = report && Array.isArray(report.audio_events) ? report.audio_events : [];
        return !spoken && !waiting && !audioInfo.music && events.length === 0;
    }

    // The rule of the server's music_incomplete: music is on when its mode is not none and its volume is above 0 (absent means 0.25).
    function musicOn(story) {
        const music = story && story.music;
        if (!music || !music.mode || music.mode === 'none') return false;
        return music.volume === undefined || music.volume === null || Number(music.volume) > 0;
    }

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
        let refused = null;   // the last gate answer of a start: {code, data}
        let disposed = false;   // a panel that is gone takes up no job and polls nothing

        const buttonLabel = el('span', { class: 'stm-export-label', text: t('storymaker.export.button') });
        const button = el('button', { type: 'button', class: 'stm-btn stm-btn-primary stm-export-btn', dataset: { stm: 'export', state: 'idle' }, ariaHaspopup: 'dialog', ariaExpanded: 'false', onClick: () => toggle() }, icon('export'), buttonLabel);
        const panel = el('div', { class: 'stm-export-panel', role: 'dialog', ariaLabel: t('storymaker.export.title'), hidden: true, dataset: { stm: 'export-panel' } });
        const root = el('div', { class: 'stm-export' }, button, panel);

        // Created on first use. It lives inside root, so the popover's outside-click rule leaves it alone.
        let dialog = null;
        function optionsDialog() {
            if (!dialog) {
                dialog = S.createExportDialog(inst);
                root.appendChild(dialog.el);
            }
            return dialog;
        }

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
            const audio = inst.audioInfo;
            if (audio && Array.isArray(audio.missing) && audio.missing.length > 0) list.push('narration');
            if (audio && !audio.music && musicOn(inst.story)) list.push('music');
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
                case 'narration': return t('storymaker.export.blocked_narration');
                case 'music': return inst.audioInfo && inst.audioInfo.music_issue === 'source_missing' ? t('storymaker.export.blocked_music_source') : t('storymaker.export.blocked_music');
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

        // The extra files of a finished job, each with its own way to Files: stems and subtitles sit in the folder next to the
        // MP4, the library copy in Videos. A job without extras has no list.
        function extrasView(output) {
            const stems = output.stems || {};
            const rows = [
                ['narration', t('storymaker.export.label_narration'), stems.narration],
                ['music', t('storymaker.export.label_music'), stems.music],
                ['sfx', t('storymaker.export.label_sfx'), stems.sfx],
                ['srt', t('storymaker.export.label_srt'), output.srt],
                ['vtt', t('storymaker.export.label_vtt'), output.vtt],
                ['library', t('storymaker.export.label_library'), output.library]
            ].filter(row => row[2]);
            if (!rows.length) return null;
            const list = el('ul', { class: 'stm-caps', dataset: { stm: 'export-extras' } });
            rows.forEach(([name, label, path]) => {
                const folder = dirOf(path);
                list.appendChild(el('li', { class: 'stm-cap', dataset: { extra: name } },
                    el('div', { class: 'stm-cap-text' },
                        el('strong', { text: label }),
                        el('code', { class: 'stm-path', dataset: { stm: 'export-extra-path' }, text: String(path) })),
                    el('button', { type: 'button', class: 'stm-btn stm-btn-quiet stm-btn-icon', ariaLabel: t('storymaker.export.show_in_files'), title: t('storymaker.export.show_in_files'), dataset: { stm: 'export-extra-show' }, disabled: !folder || typeof inst.ctx.openApp !== 'function' || null, onClick: () => { if (folder) inst.ctx.openApp('files', { path: folder }); } }, icon('folder'))));
            });
            return list;
        }

        // Server texts are shown as text only.
        function warningsView(finishedJob) {
            const warnings = Array.isArray(finishedJob.warnings) ? finishedJob.warnings : [];
            if (!warnings.length) return null;
            const list = el('ul', { class: 'stm-blockers', dataset: { stm: 'export-warnings' } });
            warnings.forEach(warning => {
                const item = warning || {};
                list.appendChild(el('li', { dataset: { code: String(item.code || '') }, text: String(item.message || item.code || '').slice(0, 600) }));
            });
            return [el('p', { class: 'stm-hint', text: t('storymaker.export.warnings') }), list];
        }

        function resultView() {
            const output = job.output || {};
            const dir = dirOf(output.path);
            return el('div', { class: 'stm-job', dataset: { stm: 'export-done', status: 'succeeded' } },
                el('p', { class: 'stm-job-title' }, icon('check'), el('strong', { text: t('storymaker.export.done') })),
                el('p', { class: 'stm-muted', text: t('storymaker.export.output') }),
                el('code', { class: 'stm-path', dataset: { stm: 'export-path' }, text: String(output.path || '') }),
                output.size ? el('p', { class: 'stm-hint', text: S.format.bytes(output.size) }) : null,
                extrasView(output),
                warningsView(job),
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

        function goVoiceButton() {
            return el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'export-go-voice' }, onClick: () => goPanel('voice') }, icon('user'), el('span', { text: t('storymaker.export.go_voice') }));
        }

        function goMusicButton() {
            return el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'export-go-music' }, onClick: () => goPanel('music') }, icon('sparkle'), el('span', { text: t('storymaker.export.go_music') }));
        }

        function goPanel(name) {
            close(false);
            inst.showPanel(name);
        }

        // A 422 names a problem of the story as this start would draw it, which the issue list need not hold: burned-in captions chosen
        // in the dialog also check narration that the saved story does not draw. It stays out of inst.issues, so another choice can still
        // start, and the panel shows the first error. Server texts are shown as text only.
        function storyRefusedView(data) {
            const issues = Array.isArray(data.issues) ? data.issues : [];
            const first = issues.find(item => item && item.severity === 'error') || {};
            const reason = [first.scene_id ? first.scene_id + ':' : '', first.message || data.message, first.fix].filter(Boolean).join(' ');
            return el('div', { class: 'stm-job', dataset: { stm: 'export-refused', code: refused.code } },
                el('p', { class: 'stm-job-title stm-job-bad' }, icon('alert'), el('strong', { text: S.errors.describeJob(t, { code: refused.code }) })),
                el('p', { class: 'stm-hint', dataset: { stm: 'export-missing' }, text: reason.slice(0, 600) }));
        }

        // A refused start says what is missing and offers the panel that fixes it.
        function refusedView() {
            const data = refused.data && typeof refused.data === 'object' ? refused.data : {};
            if (refused.story) return storyRefusedView(data);
            let text = '';
            let action = null;
            if (refused.code === 'narration_incomplete') {
                const missing = Array.isArray(data.missing) ? data.missing : [];
                const scenes = Array.from(new Set(missing.map(item => String(item && item.scene_id || '')))).filter(Boolean).slice(0, 5).join(', ');
                const chars = data.chars && Number(data.chars.missing) > 0 ? Number(data.chars.missing) : 0;
                text = t('storymaker.export.missing_narration', { scenes, chars });
                action = goVoiceButton();
            } else {
                text = data.music_issue === 'source_missing' ? t('storymaker.export.missing_music_source') : t('storymaker.export.missing_music');
                action = goMusicButton();
            }
            return el('div', { class: 'stm-job', dataset: { stm: 'export-refused', code: refused.code } },
                el('p', { class: 'stm-job-title stm-job-bad' }, icon('alert'), el('strong', { text: refused.code === 'narration_incomplete' ? t('storymaker.job_error.narration_incomplete') : t('storymaker.job_error.music_incomplete') })),
                el('p', { class: 'stm-hint', dataset: { stm: 'export-missing' }, text }),
                el('div', { class: 'stm-actions' }, action));
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
                if (reasons.indexOf('narration') >= 0) view.appendChild(goVoiceButton());
                if (reasons.indexOf('music') >= 0) view.appendChild(goMusicButton());
            } else {
                const seconds = inst.report ? inst.report.duration_ms / 1000 : 0;
                const size = S.model.exportSize(inst.story.format ? inst.story.format.aspect : '');
                view.appendChild(el('p', { class: 'stm-muted', text: t('storymaker.export.summary', { duration: S.format.clock(inst.report.frame_count, inst.report.fps), format: (inst.story.format ? inst.story.format.aspect : '') + ' · ' + (inst.report.fps || '') + ' fps', width: size[0], height: size[1] }) }));
                if (seconds > 0 && isSilentFilm(inst.audioInfo, inst.report)) view.appendChild(el('p', { class: 'stm-hint', dataset: { stm: 'export-silent' }, text: t('storymaker.export.silent_film') }));
            }
            if (refused) view.appendChild(refusedView());
            view.appendChild(el('div', { class: 'stm-actions' },
                el('button', { type: 'button', class: 'stm-btn stm-btn-primary', dataset: { stm: 'export-start' }, ariaHaspopup: 'dialog', disabled: blocked || starting || null, onClick: () => optionsDialog().open() }, icon('film'), el('span', { text: starting ? t('storymaker.export.starting') : t('storymaker.export.open_options') }))));
            return view;
        }

        function paint() {
            if (disposed) return;
            const state = isActive() ? 'running' : job ? job.status : 'idle';
            button.dataset.state = state;
            const fraction = isActive() && job && Number.isFinite(job.progress) ? S.clamp(job.progress, 0, 1) : 0;
            button.style.setProperty('--stm-export-progress', Math.round(fraction * 100) + '%');
            buttonLabel.textContent = isActive() ? t('storymaker.export.button_running', { percent: Math.round(fraction * 100) }) : t('storymaker.export.button');
            button.setAttribute('aria-expanded', String(open));
            panel.hidden = !open;
            if (!open) return;
            // Every repaint replaces the panel's buttons, so the focus moves on to the new button of the same name.
            const active = document.activeElement;
            const focused = active && panel.contains(active) ? active.dataset.stm : '';
            S.clear(panel);
            panel.appendChild(el('div', { class: 'stm-export-head' }, el('h3', { text: t('storymaker.export.title') }),
                el('button', { type: 'button', class: 'stm-btn stm-btn-quiet stm-btn-icon', ariaLabel: t('storymaker.close'), title: t('storymaker.close'), dataset: { stm: 'export-close' }, onClick: () => close() }, icon('close'))));
            panel.appendChild(capabilities());
            if (isActive()) panel.appendChild(progressView());
            else if (job && job.status === 'succeeded') panel.appendChild(resultView());
            else if (job) panel.appendChild(failedView());
            else panel.appendChild(startView());
            // While a start runs its button is off. The end of the start moves the focus, see inst.startExport.
            if (focused && !starting) focusAction(focused);
        }

        // ---- job tracking ------------------------------------------------------------------------------------------------

        function schedulePoll() {
            clearTimeout(pollTimer);
            pollTimer = 0;
            if (disposed || !isActive()) return;
            pollTimer = setTimeout(async () => {
                pollTimer = 0;
                if (disposed || !job) return;
                try { apply(await inst.api.getJob(job.id)); } catch (err) {
                    if (err && err.aborted) return;
                    schedulePoll();
                }
            }, POLL_MS);
        }

        // apply(next, adopt): a job event of this project. Without a tracked job only an active one is taken up (a start by
        // another window); a finished one is shown when this panel started it (adopt).
        function apply(next, adopt) {
            if (disposed || !next || typeof next !== 'object' || typeof next.id !== 'string') return;
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

        async function start(options) {
            if (starting || isActive() || blockers().length) return;
            starting = true;
            paint();
            try {
                const saved = await inst.flushEdits();
                if (!saved) return;
                if (blockers().length) return;
                refused = null;
                const created = await inst.api.startRender(inst.projectId, inst.etag, S.uuid(), options);
                job = null;
                apply(created, true);
                return created;
            } catch (err) {
                if (err && err.aborted) return;
                if (err && err.status === 412) inst.bus.emit('conflict', err);
                if (err && (err.code === 'narration_incomplete' || err.code === 'music_incomplete')) refused = { code: err.code, data: err.data || {} };
                if (err && err.status === 422) refused = { code: err.code, data: err.data || {}, story: true };
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
            if (open && event.key === 'Escape' && !(dialog && dialog.el.open)) { event.stopPropagation(); close(true); }
        }
        document.addEventListener('pointerdown', onDocumentPointer, true);
        document.addEventListener('keydown', onDocumentKey, true);

        // Focuses the named button of the panel, else Cancel, Start or the first button that can take the focus.
        function focusAction(name) {
            const pick = stm => panel.querySelector('[data-stm="' + stm + '"]:not([disabled])');
            const next = (name && pick(name)) || pick('export-cancel') || pick('export-start') || panel.querySelector('button:not([disabled])');
            if (next) next.focus();
        }

        function toggle() { if (open) close(true); else openPanel(); }
        function openPanel() { open = true; paint(); const first = panel.querySelector('button:not([disabled])'); if (first) first.focus(); }
        function close(restoreFocus) { open = false; paint(); if (restoreFocus) button.focus(); }

        // The dialog starts through this. It answers the new job, or nothing when the start did not happen. The repaints of the start
        // removed the button that the closed dialog gave the focus back to, so the focus moves on to what comes next.
        inst.startExport = async options => {
            const created = await start(options);
            if (open) focusAction('');
            return created;
        };

        const off = [
            inst.bus.on('job', next => { if (next && next.kind === 'render') apply(next, false); }),
            inst.bus.on('report', () => { if (open) paint(); }),
            // An audio answer ends an audio refusal and a changed story ends a story refusal. The audio answer of a save can land after
            // the 422 of the same start, and that must not wipe the reason.
            inst.bus.on('audio', () => { if (refused && !refused.story) refused = null; if (open) paint(); }),
            inst.bus.on('story', () => { if (refused && refused.story) { refused = null; if (open) paint(); } }),
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
                disposed = true;
                document.removeEventListener('pointerdown', onDocumentPointer, true);
                document.removeEventListener('keydown', onDocumentKey, true);
                off.forEach(fn => fn());
                if (dialog) dialog.dispose();
            }
        };
    }

    S.createExport = createExport;
    S.isSilentFilm = isSilentFilm;
})();
