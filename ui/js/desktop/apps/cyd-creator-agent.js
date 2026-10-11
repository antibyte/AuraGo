// CYD Creator agent panel (right panel tab "Agent"): chat history, composer with an attached log,
// job progress by phase, the plan card (confirm / re-plan / cancel), the draft banner (resume /
// discard), the revisions dialog (diff + restore) and the progress dot on the tab. It never moves
// the focus or switches tabs by itself; only user actions do.
(function () {
    'use strict';

    const P = window.CYDCreatorParts = window.CYDCreatorParts || {};
    const PHASES = ['planning', 'layout', 'coding', 'building', 'repairing', 'done'];
    const ACTIVE = new Set(['queued', 'running', 'awaiting_confirmation']);
    const TERMINAL = new Set(['succeeded', 'failed', 'cancelled', 'interrupted']);
    const LOG_MAX_BYTES = 16 * 1024;
    const STEP_ICONS = { layout: '▦', file: '✎', build: '⚙', repair: '↻', info: '•' };

    function byteLength(text) {
        return new TextEncoder().encode(String(text)).length;
    }

    // tailBytes keeps the end of text within max UTF-8 bytes, starting at a line break when possible.
    function tailBytes(text, max) {
        let s = String(text || '');
        const excess = byteLength(s) - max;
        if (excess <= 0) return s;
        // Every character has at least one byte, so dropping `excess` characters is always enough.
        s = s.slice(excess);
        if (/^[\uDC00-\uDFFF]/.test(s)) s = s.slice(1);
        const nl = s.indexOf('\n');
        return nl >= 0 && nl < s.length - 1 ? s.slice(nl + 1) : s;
    }

    function create(inst) {
        const { el, bag } = P.util;
        const t = inst.t;
        const b = bag();
        const state = {
            messages: new Map(),
            order: [],
            steps: [],
            job: null,
            jobs: [],
            attachment: '',
            sending: false,
            loaded: false,
            replanOpen: false,
            ticker: 0,
            disposed: false
        };

        // ── DOM ──
        const revisionsBtn = el('button', { type: 'button', class: 'cc-btn cc-btn--small cc-btn--ghost', dataset: { ccAgentAction: 'revisions' }, text: t('cyd_creator.revisions.open') });
        const draftBanner = el('div', { class: 'cc-banner cc-banner--warn', dataset: { ccDraft: '' }, hidden: true },
            el('span', { class: 'cc-banner-text', text: t('cyd_creator.agent.draft_text') }),
            el('button', { type: 'button', class: 'cc-btn cc-btn--small', dataset: { ccAgentAction: 'resume' }, text: t('cyd_creator.agent.draft_resume') }),
            el('button', { type: 'button', class: 'cc-btn cc-btn--small cc-btn--ghost', dataset: { ccAgentAction: 'discard' }, text: t('cyd_creator.agent.draft_discard') }));
        const log = el('div', { class: 'cc-agent-log', role: 'log', 'aria-live': 'polite', 'aria-label': t('cyd_creator.agent.history'), dataset: { ccAgentLog: '' } });
        const emptyNote = el('p', { class: 'cc-empty cc-agent-empty', text: t('cyd_creator.agent.empty') });
        const jobCard = el('section', { class: 'cc-job', dataset: { ccJob: '' }, hidden: true });
        const attachmentsRow = el('div', { class: 'cc-attachments', hidden: true });
        const input = el('textarea', { class: 'cc-input cc-agent-input', rows: '3', placeholder: t('cyd_creator.agent.placeholder'), 'aria-label': t('cyd_creator.agent.placeholder'), dataset: { ccAgentInput: '' } });
        const generateBtn = el('button', { type: 'button', class: 'cc-btn cc-btn--ghost', dataset: { ccAgentAction: 'generate' }, text: t('cyd_creator.agent.generate_code'), title: t('cyd_creator.agent.generate_code_hint') });
        const sendBtn = el('button', { type: 'submit', class: 'cc-btn cc-btn--primary', dataset: { ccAgentAction: 'send' }, text: t('cyd_creator.agent.send') });
        const composerHint = el('p', { class: 'cc-form-help', dataset: { ccComposerHint: '' }, hidden: true });
        const composer = el('form', { class: 'cc-agent-composer', dataset: { ccComposer: '' } },
            attachmentsRow, input, composerHint, el('div', { class: 'cc-composer-actions' }, generateBtn, el('span', { class: 'cc-dock-spacer' }), sendBtn));
        const root = el('div', { class: 'cc-agent', dataset: { ccAgent: '' } },
            el('div', { class: 'cc-agent-head' }, el('span', { class: 'cc-agent-title', text: t('cyd_creator.agent.title') }), el('span', { class: 'cc-dock-spacer' }), revisionsBtn),
            draftBanner, emptyNote, log, jobCard, composer);

        // ── history ──
        function messageNode(message) {
            const role = ['user', 'agent', 'system'].includes(message.role) ? message.role : 'system';
            const attachments = Array.isArray(message.attachments) ? message.attachments : [];
            return el('div', { class: 'cc-msg cc-msg--' + role + (message.pending ? ' is-pending' : ''), dataset: { ccMessage: message.id, role } },
                el('div', { class: 'cc-msg-text', text: String(message.text || '') }),
                attachments.length ? el('div', { class: 'cc-msg-attachments' }, attachments.map(a => el('span', { class: 'cc-pill', text: t('cyd_creator.agent.attachment_log', { lines: String(a.text || '').split('\n').length }) }))) : null,
                message.created_at ? el('time', { class: 'cc-msg-time', datetime: message.created_at, text: P.format.time(message.created_at) }) : null);
        }

        function stepNode(step) {
            const kind = STEP_ICONS[step.kind] ? step.kind : 'info';
            return el('div', { class: 'cc-step', dataset: { ccStep: kind } },
                el('span', { class: 'cc-step-icon', 'aria-hidden': 'true', text: STEP_ICONS[kind] }),
                el('span', { class: 'cc-step-text', text: String(step.text || '') }));
        }

        function stickToEnd(fn) {
            const atEnd = log.scrollHeight - log.scrollTop - log.clientHeight < 40;
            fn();
            if (atEnd) log.scrollTop = log.scrollHeight;
        }

        function renderLog() {
            stickToEnd(() => {
                const nodes = state.order.map(id => messageNode(state.messages.get(id)));
                state.steps.forEach(step => nodes.push(stepNode(step)));
                log.replaceChildren(...nodes);
            });
            emptyNote.hidden = state.order.length > 0 || state.steps.length > 0;
        }

        function addMessage(message, render) {
            if (!message || !message.id) return;
            if (message.role === 'user') {
                const pending = state.order.find(id => state.messages.get(id).pending && state.messages.get(id).text === message.text);
                if (pending) { state.messages.delete(pending); state.order = state.order.filter(id => id !== pending); }
            }
            if (!state.messages.has(message.id)) state.order.push(message.id);
            state.messages.set(message.id, message);
            state.order.sort((x, y) => String(state.messages.get(x).created_at || '9').localeCompare(String(state.messages.get(y).created_at || '9')));
            if (render !== false) renderLog();
        }

        function addStep(step) {
            if (!step || !state.job || step.job_id !== state.job.id) return;
            state.steps.push(step);
            if (state.steps.length > 200) state.steps.shift();
            stickToEnd(() => { log.appendChild(stepNode(step)); });
            emptyNote.hidden = true;
        }

        // ── job ──
        function isActive(job) { return !!job && ACTIVE.has(job.status); }

        function phaseList(job) {
            const current = job.status === 'succeeded' ? 'done' : job.phase;
            const index = PHASES.indexOf(current);
            return el('ol', { class: 'cc-phases', 'aria-label': t('cyd_creator.agent.phases') }, PHASES.filter(p => p !== 'repairing' || job.phase === 'repairing' || Number(job.repair_attempt) > 0).map(phase => {
                const pos = PHASES.indexOf(phase);
                const mode = phase === current ? 'is-current' : (index >= 0 && pos < index ? 'is-done' : '');
                const label = phase === 'repairing' && Number(job.repair_attempt) > 0
                    ? t('cyd_creator.agent.repair_attempt', { n: job.repair_attempt })
                    : t('cyd_creator.job.phase_' + phase);
                return el('li', { class: mode, dataset: { phase }, 'aria-current': phase === current ? 'step' : null }, el('span', { text: label }));
            }));
        }

        function planCard(job) {
            const plan = job.plan || {};
            const rows = [
                ['cyd_creator.agent.plan_screens', (plan.screens || []).join(', ')],
                ['cyd_creator.agent.plan_widgets', (plan.widgets || []).join(', ')],
                ['cyd_creator.agent.plan_data', (plan.data_sources || []).join(', ')],
                ['cyd_creator.agent.plan_wifi', plan.wifi ? t('cyd_creator.agent.wifi_' + plan.wifi) : ''],
                ['cyd_creator.agent.plan_link', t(plan.aurago_link ? 'cyd_creator.app.yes' : 'cyd_creator.app.no')],
                ['cyd_creator.agent.plan_libraries', (plan.libraries || []).map(id => {
                    const lib = ((inst.caps && inst.caps.libraries) || []).find(l => l.id === id);
                    return lib ? lib.name : id;
                }).join(', ')]
            ].filter(row => row[1]);
            const replanInput = el('textarea', { class: 'cc-input', rows: '4', 'aria-label': t('cyd_creator.agent.replan_label'), dataset: { ccReplanInput: '' } });
            replanInput.value = lastUserPrompt(job.id);
            const replanBox = el('div', { class: 'cc-replan', hidden: !state.replanOpen },
                el('label', { class: 'cc-form-field' }, el('span', { text: t('cyd_creator.agent.replan_label') }), replanInput),
                el('button', { type: 'button', class: 'cc-btn cc-btn--primary', dataset: { ccAgentAction: 'replan-send' }, text: t('cyd_creator.agent.replan_send'), onClick: () => confirmJob(replanInput.value.trim()) }));
            // Confirm and re-plan change the project; Cancel stays usable for read-only viewers (the
            // server allows the cancel routes in read-only mode).
            const disabled = inst.readonly;
            return el('article', { class: 'cc-plan-card', dataset: { ccPlan: job.id } },
                el('h3', { text: t('cyd_creator.agent.plan_title') }),
                plan.summary ? el('p', { class: 'cc-plan-summary', text: plan.summary }) : null,
                el('dl', { class: 'cc-plan-facts' }, rows.map(row => [el('dt', { text: t(row[0]) }), el('dd', { text: row[1] })])),
                el('p', { class: 'cc-form-help', text: t('cyd_creator.agent.plan_hint') }),
                el('div', { class: 'cc-plan-actions' },
                    el('button', { type: 'button', class: 'cc-btn cc-btn--primary', dataset: { ccAgentAction: 'confirm' }, disabled, text: t('cyd_creator.agent.plan_confirm'), onClick: () => confirmJob('') }),
                    el('button', { type: 'button', class: 'cc-btn', dataset: { ccAgentAction: 'replan' }, disabled, 'aria-expanded': state.replanOpen ? 'true' : 'false', text: t('cyd_creator.agent.plan_replan'), onClick: () => { state.replanOpen = !state.replanOpen; renderJob(); } }),
                    el('button', { type: 'button', class: 'cc-btn cc-btn--ghost', dataset: { ccAgentAction: 'cancel' }, text: t('cyd_creator.agent.cancel'), onClick: cancelJob })),
                replanBox);
        }

        function lastUserPrompt(jobId) {
            for (let i = state.order.length - 1; i >= 0; i--) {
                const m = state.messages.get(state.order[i]);
                if (m.role === 'user' && (!jobId || m.job_id === jobId || m.pending)) return String(m.text || '');
            }
            return '';
        }

        function elapsed(job) {
            const start = Date.parse(job.started_at || job.created_at || '');
            if (!Number.isFinite(start)) return '';
            const end = TERMINAL.has(job.status) ? Date.parse(job.finished_at || '') : Date.now();
            return P.format.duration((Number.isFinite(end) ? end : Date.now()) - start);
        }

        function renderJob() {
            const job = state.job;
            if (!job || (!isActive(job) && job.status !== 'failed' && job.status !== 'interrupted')) {
                jobCard.hidden = true;
                jobCard.replaceChildren();
            } else {
                jobCard.hidden = false;
                const statusText = t('cyd_creator.job.status_' + job.status);
                const error = job.error_code || job.error_message
                    ? el('p', { class: 'cc-job-error', role: 'alert', text: job.error_code && t.has('cyd_creator.error.' + job.error_code) ? t('cyd_creator.error.' + job.error_code) : (job.error_message || t('cyd_creator.error.generic')) })
                    : null;
                // replaceChildren turns null into the text "null": pass real nodes only.
                jobCard.replaceChildren(...[
                    el('div', { class: 'cc-job-head' },
                        el('span', { class: 'cc-job-status', dataset: { status: job.status }, text: statusText }),
                        el('span', { class: 'cc-job-elapsed', dataset: { ccJobElapsed: '' }, text: elapsed(job) }),
                        el('span', { class: 'cc-dock-spacer' }),
                        isActive(job) && job.status !== 'awaiting_confirmation'
                            ? el('button', { type: 'button', class: 'cc-btn cc-btn--small cc-btn--ghost', dataset: { ccAgentAction: 'cancel' }, text: t('cyd_creator.agent.cancel'), onClick: cancelJob })
                            : null),
                    job.status === 'awaiting_confirmation' ? planCard(job) : phaseList(job),
                    error].filter(Boolean));
            }
            const active = isActive(job);
            inst.setPanelBadge('agent', active);
            if (active && !state.ticker) {
                state.ticker = setInterval(() => {
                    const node = jobCard.querySelector('[data-cc-job-elapsed]');
                    if (node && state.job) node.textContent = elapsed(state.job);
                }, 1000);
            } else if (!active && state.ticker) {
                clearInterval(state.ticker);
                state.ticker = 0;
            }
            renderComposer();
            renderDraft();
        }

        function applyJob(job, silent) {
            if (!job || !job.id) return;
            const previous = state.job;
            if (previous && previous.id !== job.id) {
                // Only a newer job replaces the shown one (backlog replays can deliver older ones).
                if (String(job.created_at || '') < String(previous.created_at || '')) return;
                state.steps = [];
                state.replanOpen = false;
            }
            state.job = job;
            const index = state.jobs.findIndex(j => j.id === job.id);
            if (index >= 0) state.jobs[index] = job; else state.jobs.unshift(job);
            renderJob();
            if (!silent && previous && previous.id === job.id && !TERMINAL.has(previous.status) && TERMINAL.has(job.status)) {
                if (job.status === 'succeeded') inst.notify('info', t('cyd_creator.agent.job_succeeded'));
                else if (job.status !== 'cancelled') inst.notify('error', t('cyd_creator.agent.job_failed'));
                inst.refreshProject();
            }
        }

        // ── composer ──
        function renderComposer() {
            const active = isActive(state.job);
            const blocked = inst.readonly || active || state.sending;
            input.disabled = inst.readonly;
            sendBtn.disabled = blocked;
            generateBtn.disabled = blocked;
            let hintText = '';
            if (inst.readonly) hintText = t('cyd_creator.app.readonly_hint');
            else if (active) hintText = t('cyd_creator.agent.busy_hint');
            composerHint.textContent = hintText;
            composerHint.hidden = !hintText;
        }

        function renderAttachment() {
            if (!state.attachment) { attachmentsRow.hidden = true; attachmentsRow.replaceChildren(); return; }
            attachmentsRow.hidden = false;
            attachmentsRow.replaceChildren(el('span', { class: 'cc-pill', dataset: { ccAttachment: 'log' } },
                el('span', { text: t('cyd_creator.agent.attachment_log', { lines: state.attachment.split('\n').length }) }),
                el('button', { type: 'button', class: 'cc-pill-remove', 'aria-label': t('cyd_creator.agent.attachment_remove'), title: t('cyd_creator.agent.attachment_remove'), text: '×', onClick: () => { state.attachment = ''; renderAttachment(); } })));
        }

        async function startJob(body) {
            if (state.sending) return null;
            state.sending = true;
            renderComposer();
            let pendingId = '';
            if (body.prompt) {
                pendingId = P.util.uid('local');
                addMessage({ id: pendingId, role: 'user', text: body.prompt, pending: true, attachments: body.attachments || [], created_at: new Date().toISOString() });
            }
            try {
                const job = await inst.api.startJob(inst.projectId, body);
                if (state.disposed) return null;
                applyJob(job);
                return job;
            } catch (err) {
                if (pendingId) { state.messages.delete(pendingId); state.order = state.order.filter(id => id !== pendingId); renderLog(); }
                inst.notify('error', P.errors.message(t, err));
                return null;
            } finally {
                state.sending = false;
                renderComposer();
            }
        }

        async function send(mode) {
            const prompt = input.value.trim();
            if (mode === 'change' && !prompt && !state.attachment) { input.focus(); return; }
            const body = { prompt, mode, resume: false };
            if (state.attachment) body.attachments = [{ kind: 'log', text: state.attachment }];
            const job = await startJob(body);
            if (job) {
                input.value = '';
                state.attachment = '';
                renderAttachment();
            }
        }

        async function confirmJob(prompt) {
            if (!state.job) return;
            try {
                const job = await inst.api.confirmJob(state.job.id, prompt);
                state.replanOpen = false;
                applyJob(job);
            } catch (err) {
                inst.notify('error', P.errors.message(t, err));
            }
        }

        async function cancelJob() {
            if (!state.job) return;
            const ok = await inst.confirm({ titleKey: 'cyd_creator.agent.cancel_title', bodyKey: 'cyd_creator.agent.cancel_body', okKey: 'cyd_creator.agent.cancel', danger: true });
            if (!ok) return;
            try { await inst.api.cancelJob(state.job.id); } catch (err) { inst.notify('error', P.errors.message(t, err)); }
        }

        // ── draft ──
        function renderDraft() {
            const project = inst.project || {};
            draftBanner.hidden = !(project.has_draft && !isActive(state.job));
            draftBanner.querySelectorAll('button').forEach(button => { button.disabled = inst.readonly; });
        }

        async function resumeDraft() {
            const last = state.jobs[0];
            await startJob({ prompt: '', mode: (last && last.mode) || 'change', resume: true });
        }

        async function discardDraft() {
            const ok = await inst.confirm({ titleKey: 'cyd_creator.agent.draft_discard', bodyKey: 'cyd_creator.agent.draft_discard_body', okKey: 'cyd_creator.agent.draft_discard', danger: true });
            if (!ok) return;
            try {
                await inst.api.discardDraft(inst.projectId);
                if (inst.project) inst.project.has_draft = false;
                renderDraft();
            } catch (err) {
                inst.notify('error', P.errors.message(t, err));
            }
        }

        // ── revisions ──
        function diffNode(file) {
            const lines = String(file.diff || '').split('\n');
            return el('section', { class: 'cc-diff-file' },
                el('h4', {}, el('span', { text: file.path }), el('span', { class: 'cc-tag', dataset: { tone: file.status === 'deleted' ? 'danger' : (file.status === 'added' ? 'ok' : 'accent') }, text: t('cyd_creator.revisions.status_' + file.status) })),
                el('pre', { class: 'cc-diff' }, lines.map(line => el('span', {
                    class: line.startsWith('+') ? 'is-add' : (line.startsWith('-') ? 'is-del' : (line.startsWith('@@') ? 'is-hunk' : '')),
                    text: line + '\n'
                }))));
        }

        async function openRevisions() {
            const list = el('ul', { class: 'cc-rev-list', dataset: { ccRevisions: '' } }, el('li', { class: 'cc-empty', text: t('cyd_creator.app.loading') }));
            const detail = el('div', { class: 'cc-rev-detail', dataset: { ccRevisionDiff: '' } }, el('p', { class: 'cc-empty', text: t('cyd_creator.revisions.pick') }));
            let selected = null;
            let token = 0;
            const d = P.dialog.open(inst.root, {
                title: t('cyd_creator.revisions.title'),
                className: 'cc-dialog--wide',
                body: el('div', { class: 'cc-rev' }, list, detail),
                actions: [{ id: 'close', label: t('cyd_creator.app.close') }, { id: 'restore', label: t('cyd_creator.revisions.restore'), kind: 'danger', disabled: true }],
                async onAction(id, dialog) {
                    if (id !== 'restore') { dialog.close(null); return; }
                    if (!selected || inst.readonly || inst.isLocked()) return;
                    const ok = await inst.confirm({ titleKey: 'cyd_creator.revisions.restore', bodyKey: 'cyd_creator.revisions.restore_confirm', vars: { label: selected.label }, okKey: 'cyd_creator.revisions.restore', danger: true });
                    if (!ok) return;
                    dialog.setBusy(true);
                    try {
                        await inst.api.restoreRevision(selected.id);
                        dialog.close('restored');
                        inst.notify('info', t('cyd_creator.revisions.restored'));
                        await inst.reloadProject();
                    } catch (err) {
                        dialog.setBusy(false);
                        dialog.setError(P.errors.message(t, err));
                    }
                }
            });
            const restoreBtn = d.el.querySelector('[data-cc-dialog-action="restore"]');
            try {
                const revisions = await inst.api.listRevisions(inst.projectId);
                if (d.closed) return;
                if (!revisions.length) { list.replaceChildren(el('li', { class: 'cc-empty', text: t('cyd_creator.revisions.none') })); return; }
                list.replaceChildren(...revisions.map(rev => {
                    const button = el('button', { type: 'button', class: 'cc-rev-item', dataset: { ccRevision: rev.id } },
                        el('span', { class: 'cc-rev-label', text: rev.label || t('cyd_creator.revisions.reason_' + rev.reason) }),
                        el('span', { class: 'cc-tag', text: t('cyd_creator.revisions.reason_' + rev.reason) }),
                        el('time', { datetime: rev.created_at, text: P.format.time(rev.created_at) }));
                    button.addEventListener('click', async () => {
                        selected = rev;
                        list.querySelectorAll('.cc-rev-item').forEach(node => node.setAttribute('aria-pressed', node === button ? 'true' : 'false'));
                        restoreBtn.dataset.locked = inst.readonly || inst.isLocked() ? '1' : '';
                        restoreBtn.disabled = inst.readonly || inst.isLocked();
                        const mine = ++token;
                        detail.replaceChildren(el('p', { class: 'cc-empty', text: t('cyd_creator.app.loading') }));
                        try {
                            const diff = await inst.api.revisionDiff(rev.id);
                            if (mine !== token || d.closed) return;
                            const files = Array.isArray(diff.files) ? diff.files : [];
                            detail.replaceChildren(
                                el('p', { class: 'cc-rev-layout', text: t(diff.layout_changed ? 'cyd_creator.revisions.layout_changed' : 'cyd_creator.revisions.layout_same') }),
                                ...(files.length ? files.map(diffNode) : [el('p', { class: 'cc-empty', text: t('cyd_creator.revisions.no_file_changes') })]));
                        } catch (err) {
                            if (mine === token && !d.closed) detail.replaceChildren(el('p', { class: 'cc-dialog-error', text: P.errors.message(t, err) }));
                        }
                    });
                    return el('li', {}, button);
                }));
            } catch (err) {
                if (!d.closed) list.replaceChildren(el('li', { class: 'cc-dialog-error', text: P.errors.message(t, err) }));
            }
        }

        // ── loading and wiring ──
        async function load() {
            try {
                const [messages, jobs] = await Promise.all([inst.api.listMessages(inst.projectId), inst.api.listJobs(inst.projectId)]);
                if (state.disposed) return;
                messages.forEach(m => addMessage(m, false));
                state.jobs = jobs.slice();
                renderLog();
                if (jobs[0]) applyJob(jobs[0], true);
                else renderJob();
                state.loaded = true;
            } catch (err) {
                if (!state.disposed) inst.notify('error', P.errors.message(t, err));
            }
        }

        b.on(composer, 'submit', event => { event.preventDefault(); send('change'); });
        b.on(input, 'keydown', event => {
            if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) { event.preventDefault(); send('change'); }
        });
        b.on(generateBtn, 'click', () => send('generate_code'));
        b.on(revisionsBtn, 'click', openRevisions);
        b.on(draftBanner.querySelector('[data-cc-agent-action="resume"]'), 'click', resumeDraft);
        b.on(draftBanner.querySelector('[data-cc-agent-action="discard"]'), 'click', discardDraft);
        b.add(inst.bus.on('job:status', job => applyJob(job)));
        b.add(inst.bus.on('sse:job_step', step => addStep(step)));
        b.add(inst.bus.on('sse:job_message', data => addMessage(data && data.message)));
        b.add(inst.bus.on('project:updated', () => renderDraft()));
        b.add(inst.bus.on('lock:changed', () => renderComposer()));
        b.add(() => { if (state.ticker) clearInterval(state.ticker); });

        renderLog();
        renderJob();

        return {
            el: root,
            mount(host) { host.appendChild(root); return load(); },
            load,
            attachLog(text) {
                const value = tailBytes(String(text || ''), LOG_MAX_BYTES);
                if (!value.trim()) return;
                state.attachment = value;
                renderAttachment();
                inst.showPanel('agent');
                input.focus();
            },
            startFromDescription: prompt => startJob({ prompt: String(prompt || ''), mode: 'new', resume: false }),
            generateCode: () => send('generate_code'),
            openRevisions,
            job: () => state.job,
            isBusy: () => isActive(state.job),
            dispose() {
                state.disposed = true;
                b.dispose();
                root.remove();
            }
        };
    }

    P.agent = { create, tailBytes };
})();
