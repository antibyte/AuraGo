// Storymaker AI author panel: asks for a topic or a change, starts the agent job, follows it on the project's event stream and shows
// the outcome. While a job of this project is active the editor is locked (inst.agentLocked). The panel reloads the story when the
// job reports a new story_etag, at most once per 2 s and once more at the end; edits that were not saved stay as a conflict.
// It also builds the start card of the project list. Everything from the server reaches the page as text only. Classic script,
// publishes S.createAgentPanel and S.createAgentStart.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};
    const ACTIVE = ['queued', 'running'];
    const LANGUAGES = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
    const MIN_SECONDS = 15;
    const DEFAULT_SECONDS = 60;
    const FALLBACK_MAX_SECONDS = 180;
    const MAX_TEXT = '2000';
    const RELOAD_EVERY_MS = 2000;
    const PHASE_ORDER = {
        create: ['outline', 'cast', 'fill', 'check', 'narrating'],
        revise: ['revise', 'check', 'narrating']
    };
    const PHASE_KEYS = {
        outline: 'storymaker.agent.phase_outline',
        cast: 'storymaker.agent.phase_cast',
        fill: 'storymaker.agent.phase_fill',
        check: 'storymaker.agent.phase_check',
        revise: 'storymaker.agent.phase_revise',
        narrating: 'storymaker.agent.phase_narrating'
    };
    const OUTCOME_KEYS = {
        ready_unverified: { title: 'storymaker.agent.outcome_ready_title', text: 'storymaker.agent.outcome_ready_text' },
        incomplete: { title: 'storymaker.agent.outcome_incomplete_title', text: 'storymaker.agent.outcome_incomplete_text' },
        failed: { title: 'storymaker.agent.outcome_failed_title', text: 'storymaker.agent.outcome_failed_text' }
    };
    // The reason of an unavailable compile check (open check compile_unverified, field detail); any other code adds nothing.
    const COMPILE_REASON_KEYS = {
        browser_unavailable: 'storymaker.agent.compile_reason_browser_unavailable',
        compile_timeout: 'storymaker.agent.compile_reason_compile_timeout',
        browser_crashed: 'storymaker.agent.compile_reason_browser_crashed'
    };
    const CHECK_KEYS = {
        layout_unverified: 'storymaker.agent.check_layout_unverified',
        compile_unverified: 'storymaker.agent.check_compile_unverified',
        narration_unavailable: 'storymaker.agent.check_narration_unavailable',
        narration_failed: 'storymaker.agent.check_narration_failed',
        narration_budget: 'storymaker.agent.check_narration_budget',
        narration_pending: 'storymaker.agent.check_narration_pending',
        corrections_exhausted: 'storymaker.agent.check_corrections_exhausted',
        model_error: 'storymaker.agent.check_model_error',
        scene_unfilled: 'storymaker.agent.check_scene_unfilled',
        compile_failed: 'storymaker.agent.check_compile_failed',
        over_target: 'storymaker.agent.check_over_target',
        over_budget: 'storymaker.agent.check_over_budget',
        environment_monotony: 'storymaker.agent.check_craft',
        framing_monotony: 'storymaker.agent.check_craft',
        gesture_monotony: 'storymaker.agent.check_craft',
        cast_unused: 'storymaker.agent.check_craft'
    };

    function isActive(job) {
        return !!job && ACTIVE.indexOf(job.status) >= 0;
    }

    function agentBlock(inst) {
        return inst.status && inst.status.agent ? inst.status.agent : null;
    }

    function uiLanguage() {
        const tag = String(window.SYSTEM_LANG || document.documentElement.lang || 'en').toLowerCase().split(/[-_]/)[0];
        return LANGUAGES.indexOf(tag) >= 0 ? tag : 'en';
    }

    function languageName(code) {
        try {
            return new Intl.DisplayNames([uiLanguage()], { type: 'language' }).of(code) || code;
        } catch (_) {
            return code;
        }
    }

    // A story the agent may still write from scratch: one scene with nothing in it. The server has the final word (story_not_empty).
    function storyIsBlank(story) {
        const scenes = S.model.scenesOf(story);
        if (scenes.length !== 1) return false;
        const scene = scenes[0] || {};
        const empty = list => !Array.isArray(list) || !list.length;
        return scene.kind !== 'code' && !String(scene.narration || '').trim() && empty(scene.actors) && empty(scene.props) && empty(scene.beats);
    }

    // The length and language controls of a new film. prefix makes the ids and the data-stm names.
    function createFields(inst, prefix, info) {
        const { el } = S;
        const t = inst.t;
        const max = info && Number(info.max_length_seconds) >= MIN_SECONDS ? Number(info.max_length_seconds) : FALLBACK_MAX_SECONDS;
        const lengthId = inst.domId(prefix + '-length');
        const languageId = inst.domId(prefix + '-language');
        const length = el('input', { class: 'stm-input', type: 'number', id: lengthId, min: String(MIN_SECONDS), max: String(max), step: '5', value: String(Math.min(DEFAULT_SECONDS, max)), dataset: { stm: prefix + '-length' } });
        const language = el('select', { class: 'stm-select', id: languageId, dataset: { stm: prefix + '-language' } }, LANGUAGES.map(code => el('option', { value: code, text: languageName(code) })));
        language.value = uiLanguage();
        const lengthField = el('div', { class: 'stm-field' }, el('label', { class: 'stm-label', for: lengthId, text: t('storymaker.agent.length') }), length);
        const languageField = el('div', { class: 'stm-field' }, el('label', { class: 'stm-label', for: languageId, text: t('storymaker.agent.language') }), language);
        return {
            lengthField,
            languageField,
            seconds: () => Math.round(S.clamp(length.value, MIN_SECONDS, max)),
            language: () => language.value,
            setDisabled(off) { length.disabled = off; language.disabled = off; }
        };
    }

    // inst: {t, bus, api, story, status, etag, dirty, conflict, projectId, readonly, agentLocked, silentBackup, domId(name), notify(text, kind), flushEdits(), reloadProject()}
    function createAgentPanel(inst) {
        const { el, icon } = S;
        const t = inst.t;
        let job = null;
        let starting = false;
        let cancelling = false;
        let working = false;       // a make-silent or restore-speech call is under way
        let disposed = false;
        let errorText = '';
        let reloadTimer = 0;
        let lastReload = 0;
        let prefilledFor = '';
        let eventsJob = '';
        let lastSeq = -1;
        let retrying = false;

        const fields = createFields(inst, 'agent', agentBlock(inst));
        const offLine = el('p', { class: 'stm-hint', dataset: { stm: 'agent-off' }, text: t('storymaker.agent.off') });
        const unavailableLine = el('p', { class: 'stm-hint', hidden: true, dataset: { stm: 'agent-unavailable' }, text: t('storymaker.agent.unavailable') });
        const promptId = inst.domId('agent-prompt');
        const promptLabel = el('label', { class: 'stm-label', for: promptId });
        const promptBox = el('textarea', { class: 'stm-textarea', id: promptId, rows: '4', maxlength: MAX_TEXT, dataset: { stm: 'agent-prompt' } });
        const startLabel = el('span');
        const startButton = el('button', { type: 'submit', class: 'stm-btn stm-btn-primary', dataset: { stm: 'agent-start', mode: 'start' } }, icon('sparkle'), startLabel);
        const row = el('div', { class: 'stm-agent-row' }, fields.lengthField, fields.languageField);
        const form = el('form', { class: 'stm-agent-form', onSubmit: event => { event.preventDefault(); start(); } },
            el('div', { class: 'stm-field' }, promptLabel, promptBox), row, el('div', { class: 'stm-actions' }, startButton));
        const errorLine = el('p', { class: 'stm-form-error', role: 'alert', hidden: true, dataset: { stm: 'agent-error' } });
        const routeLine = el('p', { class: 'stm-hint', hidden: true, dataset: { stm: 'agent-route' } });
        const stateLine = el('p', { class: 'stm-hint', role: 'status', hidden: true, dataset: { stm: 'agent-state', state: 'idle' } });
        const steps = el('ol', { class: 'stm-agent-steps', hidden: true, dataset: { stm: 'agent-progress' } });
        const scenesLine = el('p', { class: 'stm-hint', hidden: true, dataset: { stm: 'agent-scenes' } });
        const retryLine = el('p', { class: 'stm-hint', hidden: true, dataset: { stm: 'agent-retry' }, text: t('storymaker.agent.model_retry') });
        const cancelButton = el('button', { type: 'button', class: 'stm-btn', hidden: true, dataset: { stm: 'agent-cancel' }, onClick: () => cancel() }, icon('stop'), el('span', { text: t('storymaker.agent.cancel') }));
        const outcomeCard = el('div', { class: 'stm-agent-outcome', hidden: true, dataset: { stm: 'agent-outcome' } });
        const silentButton = el('button', { type: 'button', class: 'stm-btn', hidden: true, dataset: { stm: 'agent-make-silent' }, onClick: () => makeSilent() }, el('span', { text: t('storymaker.agent.make_silent') }));
        const restoreButton = el('button', { type: 'button', class: 'stm-btn', hidden: true, dataset: { stm: 'agent-restore-speech' }, onClick: () => restoreSpeech() }, el('span', { text: t('storymaker.agent.restore_speech') }));
        const root = el('section', { class: 'stm-agent', ariaLabel: t('storymaker.side_agent'), dataset: { stm: 'agent' } },
            offLine, unavailableLine, form, errorLine, routeLine, stateLine, steps, scenesLine, retryLine, cancelButton, outcomeCard,
            el('div', { class: 'stm-actions' }, silentButton, restoreButton));

        function setError(text) {
            errorText = text || '';
        }

        function refusalText(err) {
            const e = S.errors.normalize(err);
            if (e.code === 'invalid_story') return t('storymaker.agent.refused_invalid_story');
            return S.errors.describe(t, e);
        }

        async function start() {
            if (starting || inst.readonly || isActive(job) || disposed) return;
            const text = promptBox.value.trim();
            if (!text) { setError(t('storymaker.agent.prompt_required')); paint(); promptBox.focus(); return; }
            starting = true;
            setError('');
            paint();
            try {
                if (!(await inst.flushEdits())) { setError(t('storymaker.agent.save_first')); return; }
                if (disposed) return;
                const answer = storyIsBlank(inst.story)
                    ? await inst.api.agentStart({ topic: text, length_seconds: fields.seconds(), language: fields.language(), project_id: inst.projectId })
                    : await inst.api.agentRevise(inst.projectId, text);
                if (disposed) return;
                promptBox.value = '';
                applyJob(answer.job, true);
            } catch (err) {
                if (!(err && err.aborted)) setError(refusalText(err));
            } finally {
                starting = false;
                paint();
            }
        }

        // Cancel stays open while the lock makes the editor read-only: it only stops the job.
        async function cancel() {
            if (!isActive(job) || cancelling) return;
            cancelling = true;
            paint();
            try {
                applyJob(await inst.api.cancelJob(job.id), true);
            } catch (err) {
                if (!(err && err.aborted)) inst.notify(S.errors.describe(t, err), 'error');
            } finally {
                cancelling = false;
                paint();
            }
        }

        // make-silent and restore-speech change the story on the server, so the answer is read back through the normal reload.
        async function storyAction(call, doneKey) {
            if (working || inst.readonly || disposed) return;
            working = true;
            setError('');
            paint();
            try {
                if (!(await inst.flushEdits())) { setError(t('storymaker.agent.save_first')); return; }
                await call(inst.projectId, inst.etag);
                if (disposed) return;
                await inst.reloadProject();
                if (doneKey) inst.notify(t(doneKey));
            } catch (err) {
                if (!(err && err.aborted)) setError(refusalText(err));
            } finally {
                working = false;
                paint();
            }
        }

        function makeSilent() {
            return storyAction((id, etag) => inst.api.makeSilent(id, etag), 'storymaker.agent.make_silent_done');
        }

        async function restoreSpeech() {
            if (working || inst.readonly || disposed) return;
            const ok = await inst.confirm(t('storymaker.agent.restore_speech'), t('storymaker.agent.restore_confirm'));
            if (!ok) return;
            return storyAction((id, etag) => inst.api.restoreSpeech(id, etag), '');
        }

        function reloadNow() {
            clearTimeout(reloadTimer);
            reloadTimer = 0;
            if (disposed || inst.dirty || inst.conflict) return;
            lastReload = Date.now();
            inst.reloadProject();
        }

        // A new story_etag while the job runs: reload at most once per RELOAD_EVERY_MS. Unsaved local text is never replaced.
        function maybeReload() {
            const etag = job.agent && job.agent.story_etag;
            if (!etag || etag === inst.etag || inst.dirty || inst.conflict) return;
            const wait = lastReload + RELOAD_EVERY_MS - Date.now();
            if (wait <= 0) { reloadNow(); return; }
            if (!reloadTimer) reloadTimer = setTimeout(reloadNow, wait);
        }

        // The job ended. Without local edits the story is read once more; with them the conflict banner offers reload or overwrite.
        function finishReload() {
            clearTimeout(reloadTimer);
            reloadTimer = 0;
            if (inst.conflict) return;
            if (inst.dirty) { inst.bus.emit('conflict'); return; }
            lastReload = Date.now();
            inst.reloadProject();
        }

        function syncLock() {
            const locked = isActive(job);
            if (locked === !!inst.agentLocked) return;
            inst.agentLocked = locked;
            // runSave stops at a read-only editor, so edits that are still unsaved when the lock arrives become a conflict here.
            if (locked && inst.dirty) inst.bus.emit('conflict');
            inst.bus.emit('readonly');
        }

        // The stream replays jobs newest first on every connection, so an older job never replaces a newer one, and a stale answer
        // (a POST that returns after the stream has already moved on) never moves a job backwards.
        function applyJob(next, mine) {
            if (disposed || !next || typeof next !== 'object' || next.kind !== 'agent') return;
            if (next.project_id && inst.projectId && next.project_id !== inst.projectId) return;
            const same = !!job && job.id === next.id;
            if (same && Date.parse(next.updated_at) < Date.parse(job.updated_at)) return;
            if (!same && job && !mine && (isActive(job) || Date.parse(next.created_at) < Date.parse(job.created_at))) return;
            const was = same ? job.status : '';
            job = next;
            syncLock();
            if (isActive(job)) maybeReload();
            else if (ACTIVE.indexOf(was) >= 0) finishReload();
            prefillAfterIncomplete();
            paint();
        }

        function prefillAfterIncomplete() {
            const outcome = job.agent ? job.agent.outcome : '';
            if (outcome !== 'incomplete' || prefilledFor === job.id) return;
            prefilledFor = job.id;
            if (!promptBox.value.trim()) promptBox.value = t('storymaker.agent.finish_instruction');
        }

        function takeEvent(event) {
            if (disposed || !event) return;
            if (event.job_id !== eventsJob) {
                eventsJob = event.job_id;
                lastSeq = -1;
                retrying = false;
            }
            const seq = Number(event.seq);
            if (seq <= lastSeq) return;
            lastSeq = seq;
            if (event.type !== 'model') return;
            retrying = event.status === 'retrying' || event.status === 'recovering';
            paint();
        }

        function checksOf() {
            return job && job.agent && Array.isArray(job.agent.open_checks) ? job.agent.open_checks : [];
        }

        function paintRoute() {
            const info = isActive(job) && job.agent ? job.agent : (agentBlock(inst) || {});
            const parts = [];
            if (info.model) parts.push(info.provider ? info.provider + ' / ' + info.model : info.model);
            if (info.compact) parts.push(t('storymaker.agent.mode_compact'));
            if (info.text_mode) parts.push(t('storymaker.agent.mode_text'));
            routeLine.textContent = parts.join(' · ');
            routeLine.hidden = !parts.length;
        }

        function paintState() {
            const status = job ? job.status : 'idle';
            stateLine.dataset.state = status;
            let text = '';
            if (status === 'queued') text = t('storymaker.export.queued');
            else if (status === 'cancelled') text = t('storymaker.agent.state_cancelled');
            else if (status === 'interrupted') text = t('storymaker.agent.state_interrupted');
            stateLine.textContent = text;
            stateLine.hidden = !text;
        }

        function paintProgress() {
            const active = isActive(job);
            const agent = active && job.agent ? job.agent : {};
            S.clear(steps);
            steps.hidden = !active;
            if (active) {
                const order = PHASE_ORDER[agent.kind === 'revise' ? 'revise' : 'create'];
                const current = order.indexOf(agent.phase);
                order.forEach((name, index) => {
                    const state = current < 0 || index > current ? 'pending' : index < current ? 'done' : 'current';
                    steps.appendChild(el('li', { class: 'stm-agent-step', dataset: { stm: 'agent-step', phase: name, state } }, icon(state === 'done' ? 'check' : 'info'), el('span', { text: t(PHASE_KEYS[name]) })));
                });
            }
            const total = Number(agent.scenes_total) || 0;
            scenesLine.hidden = !total;
            scenesLine.textContent = total ? t('storymaker.agent.scenes', { done: Number(agent.scenes_done) || 0, total }) : '';
            retryLine.hidden = !(active && retrying && eventsJob === job.id);
            cancelButton.hidden = !active;
            cancelButton.disabled = cancelling;
        }

        function checkLine(check) {
            const key = CHECK_KEYS[check.code] || 'storymaker.agent.check_unknown';
            return el('li', { class: 'stm-agent-check', dataset: { stm: 'agent-check', code: String(check.code || '') } },
                check.scene_id ? el('code', { text: String(check.scene_id) }) : null,
                el('span', { text: t(key) + (check.code === 'compile_unverified' && COMPILE_REASON_KEYS[check.detail] ? ' ' + t(COMPILE_REASON_KEYS[check.detail]) : '') }));
        }

        function outcomeText(outcome, checks) {
            if (outcome === 'failed') return job.error && S.JOB_ERROR_CODES.indexOf(job.error.code) >= 0 ? S.errors.describeJob(t, job.error) : t(OUTCOME_KEYS.failed.text);
            if (outcome !== 'incomplete') return t(OUTCOME_KEYS.ready_unverified.text);
            const open = checks.filter(check => check.code === 'scene_unfilled').length;
            return open ? t(OUTCOME_KEYS.incomplete.text, { n: open }) : t('storymaker.agent.outcome_incomplete_some');
        }

        // The scenes of one outcome list: the position and the id where the story still has the scene, else the id alone.
        function sceneLabels(ids) {
            const scenes = S.model.scenesOf(inst.story);
            return ids.map(id => {
                const index = scenes.findIndex(scene => scene && scene.id === id);
                return index >= 0 ? (index + 1) + ' (' + id + ')' : String(id);
            }).join(', ');
        }

        function paintOutcome() {
            const finished = !!job && (job.status === 'succeeded' || job.status === 'failed');
            const outcome = !finished ? '' : (job.agent && job.agent.outcome) || (job.status === 'failed' ? 'failed' : '');
            outcomeCard.hidden = !outcome;
            S.clear(outcomeCard);
            if (!outcome) return;
            const checks = checksOf();
            const keys = OUTCOME_KEYS[outcome] || OUTCOME_KEYS.incomplete;
            outcomeCard.dataset.outcome = outcome;
            outcomeCard.appendChild(el('h3', { text: t(keys.title) }));
            outcomeCard.appendChild(el('p', { text: outcomeText(outcome, checks) }));
            if (job.agent && job.agent.summary) outcomeCard.appendChild(el('p', { dataset: { stm: 'agent-summary' }, text: String(job.agent.summary) }));
            [['changed_scenes', 'storymaker.agent.changed_scenes'], ['unchanged_scenes', 'storymaker.agent.unchanged_scenes'], ['removed_scenes', 'storymaker.agent.removed_scenes']].forEach(([field, key]) => {
                const ids = job.agent && Array.isArray(job.agent[field]) ? job.agent[field] : [];
                if (ids.length) outcomeCard.appendChild(el('p', { dataset: { stm: 'agent-scenes-list', list: field }, text: t(key, { scenes: sceneLabels(ids) }) }));
            });
            if (checks.length) outcomeCard.appendChild(el('ul', { class: 'stm-agent-checks' }, checks.map(checkLine)));
        }

        function paintVoiceActions() {
            const noVoice = !isActive(job) && checksOf().some(check => check.code === 'narration_unavailable');
            silentButton.hidden = !noVoice || !!inst.silentBackup;
            restoreButton.hidden = !inst.silentBackup;
            silentButton.disabled = inst.readonly || working;
            restoreButton.disabled = inst.readonly || working;
        }

        function paint() {
            if (disposed) return;
            const info = agentBlock(inst) || {};
            const enabled = info.enabled === true;
            const usable = enabled && info.available !== false;
            const revise = !storyIsBlank(inst.story);
            offLine.hidden = enabled;
            unavailableLine.hidden = !enabled || usable;
            form.hidden = !enabled;
            row.hidden = revise;
            promptLabel.textContent = t(revise ? 'storymaker.agent.prompt_revise' : 'storymaker.agent.prompt_start');
            promptBox.placeholder = revise ? '' : t('storymaker.projects.new_title_placeholder');
            promptBox.disabled = inst.readonly;
            fields.setDisabled(inst.readonly);
            startLabel.textContent = t(revise ? 'storymaker.agent.revise' : 'storymaker.agent.start');
            startButton.dataset.mode = revise ? 'revise' : 'start';
            startButton.disabled = inst.readonly || starting || isActive(job) || !usable;
            errorLine.textContent = errorText;
            errorLine.hidden = !errorText;
            paintRoute();
            paintState();
            paintProgress();
            paintOutcome();
            paintVoiceActions();
        }

        const off = [
            inst.bus.on('job', next => applyJob(next, false)),
            inst.bus.on('agent', takeEvent),
            inst.bus.on('story', paint),
            inst.bus.on('status', paint),
            inst.bus.on('readonly', paint)
        ];
        paint();

        return {
            el: root,
            paint,
            dispose() {
                disposed = true;
                clearTimeout(reloadTimer);
                off.forEach(fn => fn());
            }
        };
    }

    // The start card of the project list: a topic becomes a new project with a running agent job. It stays hidden while the agent is
    // switched off. inst: {t, domId(name), readonly, agentStatus, startAgent(spec)}
    function createAgentStart(inst) {
        const { el, icon } = S;
        const t = inst.t;
        let busy = false;
        const fields = createFields(inst, 'agent-new', inst.agentStatus);
        const topicId = inst.domId('agent-new-topic');
        const aspectId = inst.domId('agent-new-aspect');
        const topic = el('textarea', { class: 'stm-textarea stm-agent-start-topic', id: topicId, rows: '2', maxlength: MAX_TEXT, placeholder: t('storymaker.projects.new_title_placeholder'), dataset: { stm: 'agent-new-topic' } });
        const aspect = el('select', { class: 'stm-select', id: aspectId, dataset: { stm: 'agent-new-aspect' } },
            el('option', { value: '16:9', text: t('storymaker.aspect_16_9') }),
            el('option', { value: '9:16', text: t('storymaker.aspect_9_16') }),
            el('option', { value: '1:1', text: t('storymaker.aspect_1_1') }));
        const startButton = el('button', { type: 'submit', class: 'stm-btn stm-btn-primary', dataset: { stm: 'agent-new-start' } }, icon('sparkle'), el('span', { text: t('storymaker.agent.start') }));
        const note = el('p', { class: 'stm-hint stm-agent-start-wide', hidden: true, dataset: { stm: 'agent-new-note' }, text: t('storymaker.agent.unavailable') });
        const errorLine = el('p', { class: 'stm-form-error', role: 'alert', hidden: true, dataset: { stm: 'agent-new-error' } });
        const root = el('form', { class: 'stm-new stm-agent-start', hidden: true, onSubmit: event => { event.preventDefault(); submit(); } },
            el('h3', { class: 'stm-agent-start-wide', text: t('storymaker.agent.new_title') }),
            el('div', { class: 'stm-field stm-agent-start-wide' }, el('label', { class: 'stm-label', for: topicId, text: t('storymaker.agent.prompt_start') }), topic),
            fields.lengthField, fields.languageField,
            el('div', { class: 'stm-field' }, el('label', { class: 'stm-label', for: aspectId, text: t('storymaker.projects.new_format') }), aspect),
            el('div', { class: 'stm-actions' }, startButton),
            note, errorLine);

        function setError(text) {
            errorLine.textContent = text || '';
            errorLine.hidden = !text;
        }

        function refresh() {
            const info = inst.agentStatus || {};
            const usable = info.enabled === true && info.available !== false;
            root.hidden = info.enabled !== true;
            note.hidden = info.enabled !== true || usable;
            topic.disabled = inst.readonly;
            aspect.disabled = inst.readonly;
            fields.setDisabled(inst.readonly);
            startButton.disabled = inst.readonly || busy || !usable;
        }

        async function submit() {
            if (busy || inst.readonly) return;
            const text = topic.value.trim();
            if (!text) { setError(t('storymaker.agent.prompt_required')); topic.focus(); return; }
            busy = true;
            setError('');
            refresh();
            try {
                await inst.startAgent({ topic: text, length_seconds: fields.seconds(), language: fields.language(), aspect: aspect.value });
            } catch (err) {
                if (!(err && err.aborted)) setError(S.errors.describe(t, err));
            } finally {
                busy = false;
                refresh();
            }
        }

        refresh();
        return { el: root, refresh };
    }

    S.createAgentPanel = createAgentPanel;
    S.createAgentStart = createAgentStart;
})();
