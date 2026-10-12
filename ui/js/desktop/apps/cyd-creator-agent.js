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
    const MAX_STEPS = 200;
    const STEP_ICONS = { layout: '▦', file: '✎', build: '⚙', repair: '↻', info: '•' };
    // CODE_RE is the shape of a server text code (internal/cydcreator texts.go).
    const CODE_RE = /^[a-z][a-z0-9_]{0,63}$/;

    function byteLength(text) {
        return new TextEncoder().encode(String(text)).length;
    }

    // utf8Size is the encoded size of one code point (a lone surrogate encodes as U+FFFD: 3 bytes).
    function utf8Size(cp) {
        return cp < 0x80 ? 1 : (cp < 0x800 ? 2 : (cp < 0x10000 ? 3 : 4));
    }

    // tailBytes keeps the longest end of text within max UTF-8 bytes (never half a surrogate pair),
    // starting at a line break when possible.
    function tailBytes(text, max) {
        const full = String(text || '');
        if (byteLength(full) <= max) return full;
        let start = full.length;
        let used = 0;
        while (start > 0) {
            let i = start - 1;
            const unit = full.charCodeAt(i);
            if (unit >= 0xDC00 && unit <= 0xDFFF && i > 0) {
                const high = full.charCodeAt(i - 1);
                if (high >= 0xD800 && high <= 0xDBFF) i--;
            }
            const size = utf8Size(full.codePointAt(i));
            if (used + size > max) break;
            used += size;
            start = i;
        }
        const s = full.slice(start);
        const nl = s.indexOf('\n');
        return nl >= 0 && nl < s.length - 1 ? s.slice(nl + 1) : s;
    }

    function create(inst) {
        const { el, bag } = P.util;
        const t = inst.t;
        const b = bag();
        const state = {
            messages: new Map(),
            // entries is the log in display order: {kind: 'msg', id, node} or {kind: 'step', step, node}.
            entries: [],
            job: null,
            jobs: [],
            attachment: '',
            sending: false,
            confirming: false,
            loaded: false,
            replanOpen: false,
            replanText: null,
            // planShown is the id of the job whose plan card is shown (opened at its top once).
            planShown: '',
            ticker: 0,
            disposed: false
        };

        // ── DOM ──
        const revisionsBtn = el('button', { type: 'button', class: 'cc-btn cc-btn--small cc-btn--ghost', dataset: { ccAgentAction: 'revisions' }, text: t('cyd_creator.revisions.open') });
        const resumeBtn = el('button', { type: 'button', class: 'cc-btn cc-btn--small', dataset: { ccAgentAction: 'resume' }, text: t('cyd_creator.agent.draft_resume') });
        const discardBtn = el('button', { type: 'button', class: 'cc-btn cc-btn--small cc-btn--ghost', dataset: { ccAgentAction: 'discard' }, text: t('cyd_creator.agent.draft_discard') });
        const draftBanner = el('div', { class: 'cc-banner cc-banner--warn', dataset: { ccDraft: '' }, hidden: true },
            el('span', { class: 'cc-banner-text', text: t('cyd_creator.agent.draft_text') }), resumeBtn, discardBtn);
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
        // History and job card scroll together (one scroller): the plan card grows with its facts
        // and keeps its actions in view (sticky), the composer stays below.
        const scroller = el('div', { class: 'cc-agent-scroll', dataset: { ccAgentScroll: '' } }, emptyNote, log, jobCard);
        const root = el('div', { class: 'cc-agent', dataset: { ccAgent: '' } },
            el('div', { class: 'cc-agent-head' }, el('span', { class: 'cc-agent-title', text: t('cyd_creator.agent.title') }), el('span', { class: 'cc-dock-spacer' }), revisionsBtn),
            draftBanner, scroller, composer);

        // ── history ──
        // serverText shows a server-written line (job step, system message) in the UI language: the
        // translation of its code (cyd_creator.step.<code> / cyd_creator.msg.<code>) filled with its
        // vars, else the server's English text as it is. A line with error_code (a failed job, a failed
        // build) names the error by its code like the job card does.
        function serverText(prefix, item) {
            const text = String(item.text || '');
            const code = typeof item.code === 'string' && CODE_RE.test(item.code) ? item.code : '';
            if (!code || !t.has(prefix + code)) return text;
            const vars = Object.assign({}, item.vars && typeof item.vars === 'object' ? item.vars : {});
            if (typeof vars.error_code === 'string' && vars.error_code) {
                vars.error = errorSentence(vars.error_code, String(vars.error_reason || ''), String(vars.error || ''));
            }
            return t(prefix + code, vars);
        }

        function messageNode(message) {
            const role = ['user', 'agent', 'system'].includes(message.role) ? message.role : 'system';
            const attachments = Array.isArray(message.attachments) ? message.attachments : [];
            return el('div', { class: 'cc-msg cc-msg--' + role + (message.pending ? ' is-pending' : ''), dataset: { ccMessage: message.id, role } },
                el('div', { class: 'cc-msg-text', text: role === 'system' ? serverText('cyd_creator.msg.', message) : String(message.text || '') }),
                attachments.length ? el('div', { class: 'cc-msg-attachments' }, attachments.map(a => el('span', { class: 'cc-pill', text: t('cyd_creator.agent.attachment_log', { lines: String(a.text || '').split('\n').length }) }))) : null,
                message.created_at ? el('time', { class: 'cc-msg-time', datetime: message.created_at, text: P.format.time(message.created_at) }) : null);
        }

        function stepNode(step) {
            const kind = STEP_ICONS[step.kind] ? step.kind : 'info';
            return el('div', { class: 'cc-step', dataset: { ccStep: kind } },
                el('span', { class: 'cc-step-icon', 'aria-hidden': 'true', text: STEP_ICONS[kind] }),
                el('span', { class: 'cc-step-text', text: serverText('cyd_creator.step.', step) }));
        }

        // stickToEnd runs fn and keeps a reader who was at the end there; it returns whether they were.
        function stickToEnd(fn) {
            const atEnd = scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight < 40;
            fn();
            if (atEnd) scroller.scrollTop = scroller.scrollHeight;
            return atEnd;
        }

        function entryNode(entry) {
            if (!entry.node) entry.node = entry.kind === 'msg' ? messageNode(state.messages.get(entry.id)) : stepNode(entry.step);
            return entry.node;
        }

        // renderLog draws the whole log once (the first load). Later changes insert, swap or remove
        // single nodes, so the live region announces only what is new.
        function renderLog() {
            stickToEnd(() => { log.replaceChildren(...state.entries.map(entryNode)); });
            emptyNote.hidden = state.entries.length > 0;
        }

        // place puts entry at index; with render it also inserts the node at the same place.
        function place(entry, index, render) {
            const next = state.entries[index];
            state.entries.splice(index, 0, entry);
            if (render === false) return;
            const before = next && next.node && next.node.parentNode === log ? next.node : null;
            stickToEnd(() => { log.insertBefore(entryNode(entry), before); });
            emptyNote.hidden = true;
        }

        function dropEntry(entry) {
            const index = state.entries.indexOf(entry);
            if (index >= 0) state.entries.splice(index, 1);
            if (entry.node) entry.node.remove();
            emptyNote.hidden = state.entries.length > 0;
        }

        function isPending(entry) { return entry.kind === 'msg' && !!state.messages.get(entry.id).pending; }

        // messageIndex keeps server messages in time order (a reload can deliver older ones late);
        // a pending bubble goes last.
        function messageIndex(message) {
            if (message.pending) return state.entries.length;
            const at = String(message.created_at || '9');
            const later = state.entries.findIndex(e => e.kind === 'msg' && !isPending(e) && String(state.messages.get(e.id).created_at || '9') > at);
            if (later >= 0) return later;
            const pending = state.entries.findIndex(isPending);
            return pending >= 0 ? pending : state.entries.length;
        }

        function addMessage(message, render) {
            if (!message || !message.id || state.messages.has(message.id)) return;
            if (message.role === 'user') {
                const pending = state.entries.find(e => isPending(e) && state.messages.get(e.id).text === message.text);
                if (pending) {
                    // The server's copy of a sent prompt takes the place of its pending bubble.
                    state.messages.delete(pending.id);
                    state.messages.set(message.id, message);
                    const old = pending.node;
                    pending.id = message.id;
                    pending.node = null;
                    if (render !== false && old && old.parentNode === log) {
                        log.insertBefore(entryNode(pending), old);
                        old.remove();
                    }
                    return;
                }
            }
            state.messages.set(message.id, message);
            place({ kind: 'msg', id: message.id, node: null }, messageIndex(message), render);
        }

        function removeMessage(id) {
            const entry = state.entries.find(e => e.kind === 'msg' && e.id === id);
            state.messages.delete(id);
            if (entry) dropEntry(entry);
        }

        function addStep(step) {
            if (!step || !state.job || step.job_id !== state.job.id) return;
            place({ kind: 'step', step, node: null }, state.entries.length, true);
            const steps = state.entries.filter(e => e.kind === 'step');
            if (steps.length > MAX_STEPS) dropEntry(steps[0]);
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
            // The edited description survives re-renders (opening/closing the box, a finished request).
            const replanInput = el('textarea', { class: 'cc-input', rows: '4', 'aria-label': t('cyd_creator.agent.replan_label'), dataset: { ccReplanInput: '' }, onInput: () => { state.replanText = replanInput.value; } });
            replanInput.value = state.replanText == null ? lastUserPrompt(job.id) : state.replanText;
            // Confirm and re-plan change the project and wait for one answer; Cancel stays usable for
            // read-only viewers (the server allows the cancel routes in read-only mode).
            const disabled = inst.readonly || state.confirming;
            const replanBox = el('div', { class: 'cc-replan', hidden: !state.replanOpen },
                el('label', { class: 'cc-form-field' }, el('span', { text: t('cyd_creator.agent.replan_label') }), replanInput),
                el('button', { type: 'button', class: 'cc-btn cc-btn--primary', dataset: { ccAgentAction: 'replan-send' }, disabled, text: t('cyd_creator.agent.replan_send'), onClick: () => replan(replanInput) }));
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

        // replan asks for a new plan. An empty prompt would confirm the plan as it is, so it needs text.
        function replan(field) {
            const prompt = field.value.trim();
            if (!prompt) { field.focus(); return; }
            confirmJob(prompt);
        }

        function lastUserPrompt(jobId) {
            for (let i = state.entries.length - 1; i >= 0; i--) {
                const entry = state.entries[i];
                const m = entry.kind === 'msg' ? state.messages.get(entry.id) : null;
                if (m && m.role === 'user' && (!jobId || m.job_id === jobId || m.pending)) return String(m.text || '');
            }
            return '';
        }

        // errorSentence is the sentence for a job error: a publication fenced for review (CYD_BUSY with
        // reason publication_review) has its own text, other CYD_BUSY endings keep the server's
        // sentence, known codes their translation.
        function errorSentence(code, reason, message) {
            if (reason === 'publication_review') return t('cyd_creator.error.publication_review');
            if (code === 'CYD_BUSY') return message || t('cyd_creator.error.CYD_BUSY');
            return code && t.has('cyd_creator.error.' + code) ? t('cyd_creator.error.' + code) : (message || t('cyd_creator.error.generic'));
        }

        // jobError is the sentence for a failed or interrupted job; without a recorded reason the
        // project summary (publish_review) tells a fenced publication from another CYD_BUSY.
        function jobError(job) {
            const project = inst.project || {};
            if (!job.error_reason && job.error_code === 'CYD_BUSY' && project.publish_review) return t('cyd_creator.error.publication_review');
            return errorSentence(job.error_code, job.error_reason, job.error_message);
        }

        function elapsed(job) {
            const start = Date.parse(job.started_at || job.created_at || '');
            if (!Number.isFinite(start)) return '';
            const end = TERMINAL.has(job.status) ? Date.parse(job.finished_at || '') : Date.now();
            return P.format.duration((Number.isFinite(end) ? end : Date.now()) - start);
        }

        // renderJob redraws the job card below the history; a reader at the end stays at the end. When
        // a job's plan card appears, a reader at the end sees it from its top (status, heading, summary)
        // instead of at its actions; redraws of the same card (read-only, confirm in flight) keep the
        // scroll position. A new plan of the same job (re-plan) opens at its top again. While the panel
        // is hidden (no height) the open waits for the next redraw in view.
        function renderJob() {
            const job = state.job;
            const atEnd = stickToEnd(drawJob);
            if (job && job.status === 'awaiting_confirmation') {
                if (state.planShown !== job.id && scroller.clientHeight > 0) {
                    state.planShown = job.id;
                    if (atEnd) scroller.scrollTop += jobCard.getBoundingClientRect().top - scroller.getBoundingClientRect().top - 8;
                }
            } else {
                state.planShown = '';
            }
            const active = isActive(state.job);
            inst.setPanelBadge('agent', active);
            if (active && !state.ticker && !state.disposed) {
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

        function drawJob() {
            const job = state.job;
            if (!job || (!isActive(job) && job.status !== 'failed' && job.status !== 'interrupted')) {
                jobCard.hidden = true;
                jobCard.replaceChildren();
            } else {
                jobCard.hidden = false;
                const statusText = t('cyd_creator.job.status_' + job.status);
                const error = job.error_code || job.error_message
                    ? el('p', { class: 'cc-job-error', role: 'alert', text: jobError(job) })
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
        }

        function applyJob(job, silent) {
            if (!job || !job.id || state.disposed) return;
            const previous = state.job;
            // A finished job stays finished: a snapshot taken before its end (the POST answer, the
            // GET /jobs list) can arrive after the job_status event that ended it.
            if (previous && previous.id === job.id && TERMINAL.has(previous.status) && !TERMINAL.has(job.status)) return;
            if (previous && previous.id !== job.id) {
                // Only a newer job replaces the shown one (backlog replays can deliver older ones).
                if (String(job.created_at || '') < String(previous.created_at || '')) return;
                state.entries.filter(e => e.kind === 'step').forEach(dropEntry);
                state.replanOpen = false;
                state.replanText = null;
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
                if (state.disposed) return null;
                if (pendingId) removeMessage(pendingId);
                inst.notify('error', P.errors.message(t, err));
                return null;
            } finally {
                state.sending = false;
                if (!state.disposed) renderComposer();
            }
        }

        async function send(mode) {
            // Ctrl+Enter reaches this while the buttons are disabled.
            if (inst.readonly || isActive(state.job) || state.sending) return;
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

        // setConfirming disables Build it / Change plan / Plan again in place while one confirm request
        // runs (a re-render would move the focus); Cancel stays usable.
        function setConfirming(on) {
            state.confirming = on;
            jobCard.querySelectorAll('[data-cc-agent-action="confirm"],[data-cc-agent-action="replan"],[data-cc-agent-action="replan-send"]')
                .forEach(button => { button.disabled = inst.readonly || on; });
        }

        async function confirmJob(prompt) {
            const job = state.job;
            if (!job || job.status !== 'awaiting_confirmation' || state.confirming || inst.readonly) return;
            setConfirming(true);
            try {
                const answer = await inst.api.confirmJob(job.id, prompt);
                if (state.disposed) return;
                state.replanOpen = false;
                state.replanText = null;
                // The answer applies only while the job still waits: a job_status event may already
                // have moved it on (running, or a fast failure).
                if (state.job && state.job.id === answer.id && state.job.status === 'awaiting_confirmation') applyJob(answer);
            } catch (err) {
                if (!state.disposed) inst.notify('error', P.errors.message(t, err));
            } finally {
                if (!state.disposed) setConfirming(false);
            }
        }

        async function cancelJob() {
            if (!state.job) return;
            const id = state.job.id;
            const ok = await inst.confirm({ titleKey: 'cyd_creator.agent.cancel_title', bodyKey: 'cyd_creator.agent.cancel_body', okKey: 'cyd_creator.agent.cancel', danger: true });
            if (!ok || state.disposed) return;
            try { await inst.api.cancelJob(id); } catch (err) { if (!state.disposed) inst.notify('error', P.errors.message(t, err)); }
        }

        // ── draft ──
        function renderDraft() {
            const project = inst.project || {};
            draftBanner.hidden = !(project.has_draft && !isActive(state.job));
            // A project fenced for publication review refuses a resume; discarding the draft stays allowed.
            const fenced = !!project.publish_review;
            resumeBtn.disabled = inst.readonly || fenced;
            if (fenced) resumeBtn.setAttribute('title', t('cyd_creator.error.publication_review'));
            else resumeBtn.removeAttribute('title');
            discardBtn.disabled = inst.readonly;
        }

        async function resumeDraft() {
            if (inst.readonly || (inst.project && inst.project.publish_review)) return;
            const last = state.jobs[0];
            await startJob({ prompt: '', mode: (last && last.mode) || 'change', resume: true });
        }

        async function discardDraft() {
            const ok = await inst.confirm({ titleKey: 'cyd_creator.agent.draft_discard', bodyKey: 'cyd_creator.agent.draft_discard_body', okKey: 'cyd_creator.agent.draft_discard', danger: true });
            if (!ok || state.disposed) return;
            try {
                await inst.api.discardDraft(inst.projectId);
                if (state.disposed) return;
                if (inst.project) inst.project.has_draft = false;
                renderDraft();
            } catch (err) {
                if (!state.disposed) inst.notify('error', P.errors.message(t, err));
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
            let restoreBtn = null;
            // Restore follows the selection, read-only and the project lock while the dialog is open.
            function syncRestore() {
                if (!restoreBtn) return;
                const off = !selected || inst.readonly || inst.isLocked();
                restoreBtn.dataset.locked = off ? '1' : '';
                restoreBtn.disabled = off;
            }
            const d = P.dialog.open(inst.root, {
                title: t('cyd_creator.revisions.title'),
                className: 'cc-dialog--wide',
                body: el('div', { class: 'cc-rev' }, list, detail),
                actions: [{ id: 'close', label: t('cyd_creator.app.close') }, { id: 'restore', label: t('cyd_creator.revisions.restore'), kind: 'danger', disabled: true }],
                owner: b,
                async onAction(id, dialog) {
                    if (id !== 'restore') { dialog.close(null); return; }
                    syncRestore();
                    if (!selected || inst.readonly || inst.isLocked()) return;
                    const target = selected;
                    const ok = await inst.confirm({ titleKey: 'cyd_creator.revisions.restore', bodyKey: 'cyd_creator.revisions.restore_confirm', vars: { label: target.label }, okKey: 'cyd_creator.revisions.restore', danger: true });
                    if (!ok || d.closed) return;
                    dialog.setBusy(true);
                    // Unsaved layout and code edits are saved first: the restore would drop them, and the
                    // "Before restoring" revision should contain them.
                    if (!(await inst.flushAll()) && !(await inst.confirm({ titleKey: 'cyd_creator.app.unsaved_title', bodyKey: 'cyd_creator.app.unsaved_body', okKey: 'cyd_creator.revisions.restore', danger: true }))) {
                        if (!d.closed) dialog.setBusy(false);
                        return;
                    }
                    if (d.closed) return;
                    try {
                        await inst.api.restoreRevision(target.id);
                        dialog.close('restored');
                        if (state.disposed) return;
                        inst.notify('info', t('cyd_creator.revisions.restored'));
                        await inst.reloadProject();
                    } catch (err) {
                        if (d.closed) return;
                        dialog.setBusy(false);
                        dialog.setError(P.errors.message(t, err));
                    }
                }
            });
            restoreBtn = d.el.querySelector('[data-cc-dialog-action="restore"]');
            const offLock = inst.bus.on('lock:changed', syncRestore);
            d.promise.then(() => offLock());
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
                        list.querySelectorAll('[data-cc-revision]').forEach(node => node.setAttribute('aria-pressed', node === button ? 'true' : 'false'));
                        syncRestore();
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
        // load reads the history and the jobs. The first load draws the log once; a reload (stream_ready
        // with reload) only inserts what is missing: messages are deduplicated by id, and a stale job
        // snapshot never reopens a finished job.
        async function load() {
            const first = !state.loaded;
            try {
                const [messages, jobs] = await Promise.all([inst.api.listMessages(inst.projectId), inst.api.listJobs(inst.projectId)]);
                if (state.disposed) return;
                messages.forEach(m => addMessage(m, !first));
                state.jobs = jobs.slice();
                if (first) renderLog();
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
        b.on(resumeBtn, 'click', resumeDraft);
        b.on(discardBtn, 'click', discardDraft);
        b.add(inst.bus.on('job:status', job => applyJob(job)));
        b.add(inst.bus.on('sse:job_step', step => addStep(step)));
        b.add(inst.bus.on('sse:job_message', data => addMessage(data && data.message)));
        b.add(inst.bus.on('sse:stream_ready', data => { if (data && data.reload) load(); }));
        // The summary carries has_draft and publish_review (draft banner, error text of a finished job).
        // An active job's card does not depend on it and is not rebuilt (the re-plan field keeps focus).
        b.add(inst.bus.on('project:updated', () => { if (isActive(state.job)) renderDraft(); else renderJob(); }));
        b.add(inst.bus.on('lock:changed', () => renderComposer()));
        // Read-only switched by the desktop policy: the plan actions, the composer and the draft
        // buttons follow at once, also while a job runs (Cancel stays usable).
        b.add(inst.bus.on('readonly:changed', () => renderJob()));
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
