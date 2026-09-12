// Mission Control – right detail pane: hero with actions, Overview and History tabs.
(function () {
    'use strict';

    const CANCELLED_OUTPUT = 'Cancelled by user';
    const OUTPUT_PREVIEW_CHARS = 600;

    function emitter() {
        const handlers = new Map();
        return {
            on(name, cb) { if (!handlers.has(name)) handlers.set(name, new Set()); handlers.get(name).add(cb); return () => handlers.get(name).delete(cb); },
            emit(name, payload) { const set = handlers.get(name); if (set) set.forEach(cb => { try { cb(payload); } catch (err) { console.error('MissionControlDetail handler failed', err); } }); },
            clear() { handlers.clear(); }
        };
    }

    // Mission outputs are sometimes stored as raw OpenAI-style JSON; unwrap the assistant text.
    function extractLastOutput(raw) {
        if (!raw) return '';
        const text = String(raw);
        if (!text.trimStart().startsWith('{')) return text;
        try {
            const obj = JSON.parse(text);
            if (obj && obj.choices && obj.choices[0] && obj.choices[0].message) return obj.choices[0].message.content || text;
        } catch (_) { /* not JSON */ }
        return text;
    }

    function isCancelledRun(run) {
        return run.status === 'error' && (String(run.output || '').trim() === CANCELLED_OUTPUT || String(run.error_msg || '').trim() === CANCELLED_OUTPUT);
    }

    function create(deps) {
        const { esc, t, lang, svg, triggers, schedule, readonly, fmt } = deps;
        const events = emitter();
        const ic = (name) => (svg && svg[name]) || '';
        let mission = null;
        let ctx = { running: false, queuePosition: 0, cancelling: false, busy: '' };
        let tab = 'overview';
        let outputExpanded = false;
        let prepared = { data: null, loading: false };
        let history = { items: [], total: 0, loading: false, error: '', filter: 'all' };
        const expandedRuns = new Set();

        const element = document.createElement('section');
        element.className = 'vd-mc-detail';
        element.innerHTML = `
            <div class="vd-mc-detail-empty" data-mc-detail-empty>
                <div class="vd-mc-detail-empty-icon">${ic('workflow')}</div>
                <h2 class="vd-mc-detail-empty-title">${esc(t('desktop.mc_empty_title'))}</h2>
                <p class="vd-mc-detail-empty-desc">${esc(t('desktop.mc_empty_desc'))}</p>
                ${readonly ? '' : `<p class="vd-mc-detail-empty-hint">${esc(t('desktop.mc_empty_shortcut_hint'))}</p>`}
            </div>
            <div class="vd-mc-detail-body" data-mc-detail-body hidden>
                <header class="vd-mc-hero" data-mc-hero></header>
                <div class="vd-mc-progress" data-mc-progress hidden></div>
                <nav class="vd-mc-tabs" role="tablist" aria-label="${esc(t('desktop.mc_overview_task'))}">
                    <button type="button" class="vd-mc-tab is-active" role="tab" aria-selected="true" data-mc-tab="overview" id="mc-tab-overview" aria-controls="mc-panel-overview">${esc(t('desktop.mc_tab_overview'))}</button>
                    <button type="button" class="vd-mc-tab" role="tab" aria-selected="false" data-mc-tab="history" id="mc-tab-history" aria-controls="mc-panel-history">${esc(t('desktop.mc_tab_history'))}</button>
                </nav>
                <div class="vd-mc-panels">
                    <section class="vd-mc-panel" role="tabpanel" id="mc-panel-overview" aria-labelledby="mc-tab-overview" data-mc-panel="overview"></section>
                    <section class="vd-mc-panel" role="tabpanel" id="mc-panel-history" aria-labelledby="mc-tab-history" data-mc-panel="history" hidden></section>
                </div>
            </div>`;
        const q = (sel) => element.querySelector(sel);

        function stateOf() {
            if (!mission) return 'idle';
            if (ctx.running) return 'running';
            if (ctx.queuePosition) return 'queued';
            if (mission.enabled === false) return 'paused';
            if (mission.last_result === 'error') return 'error';
            if (mission.last_result === 'success') return 'ok';
            return mission.last_run ? 'idle' : 'never';
        }
        const STATE_KEYS = { running: 'desktop.mc_state_running', queued: 'desktop.mc_state_queued', paused: 'desktop.mc_state_paused', error: 'desktop.mc_state_error', ok: 'desktop.mc_state_ok', idle: 'desktop.mc_state_idle', never: 'desktop.mc_state_never_run' };
        function pill(state, extraClass) { return `<span class="vd-mc-pill ${extraClass || ''}" data-state="${esc(state)}">${esc(t(STATE_KEYS[state] || state))}</span>`; }

        function actionButton(name, labelKey, iconName, opts) {
            opts = opts || {};
            const disabled = opts.disabled || readonly || (ctx.busy && ctx.busy !== name);
            return `<button type="button" class="vd-mc-btn${opts.primary ? ' vd-mc-btn--primary' : ''}${opts.danger ? ' vd-mc-btn--danger' : ''}" data-mc-action="${esc(name)}" ${disabled ? 'disabled' : ''} ${opts.title ? `title="${esc(opts.title)}" aria-label="${esc(opts.title)}"` : ''}>${ic(iconName)}${opts.iconOnly ? '' : `<span>${esc(opts.label || t(labelKey))}</span>`}</button>`;
        }

        function renderHero() {
            const state = stateOf();
            const remote = mission.runner_type === 'remote';
            let primary;
            if (ctx.running && !remote) primary = actionButton('cancel', 'desktop.mc_action_cancel', 'stop', { danger: true, disabled: ctx.cancelling, label: ctx.cancelling ? t('desktop.mc_action_cancelling') : t('desktop.mc_action_cancel') });
            else if (ctx.queuePosition) primary = actionButton('removeQueue', 'desktop.mc_action_remove_queue', 'queue', {});
            else primary = actionButton('run', 'desktop.mc_action_run', 'play', { primary: true, disabled: ctx.running });
            q('[data-mc-hero]').innerHTML = `
                <div class="vd-mc-hero-main">
                    <div class="vd-mc-hero-titlerow">
                        <h2 class="vd-mc-hero-title">${esc(mission.name || '')}</h2>
                        ${pill(state)}
                        ${mission.locked ? `<span class="vd-mc-badge" title="${esc(t('desktop.mc_state_locked'))}">${ic('lock')}<span>${esc(t('desktop.mc_state_locked'))}</span></span>` : ''}
                        ${remote ? `<span class="vd-mc-badge">${ic('globe')}<span>${esc(mission.remote_nest_name || mission.remote_egg_name || t('desktop.mc_state_remote'))}</span></span>` : ''}
                    </div>
                    <div class="vd-mc-hero-summary">${ic(mission.execution_type === 'scheduled' ? 'clock' : mission.execution_type === 'triggered' ? 'bolt' : 'hand')}<span>${esc(triggers.summary(mission, t, { schedule, lang }))}</span></div>
                </div>
                <div class="vd-mc-hero-actions">
                    ${primary}
                    ${actionButton('edit', 'desktop.mc_action_edit', 'edit', {})}
                    <button type="button" class="vd-mc-btn vd-mc-btn--icon" data-mc-action="more" title="${esc(t('desktop.mc_action_more'))}" aria-label="${esc(t('desktop.mc_action_more'))}" aria-haspopup="menu">${ic('more')}</button>
                </div>`;
            const progress = q('[data-mc-progress]');
            if (ctx.running) {
                progress.hidden = false;
                progress.innerHTML = `<div class="vd-mc-progress-bar" aria-hidden="true"><span></span></div><div class="vd-mc-progress-text" data-mc-elapsed>${esc(ctx.cancelling ? t('desktop.mc_action_cancelling') : t('desktop.mc_overview_running_since', { elapsed: '00:00' }))}</div>`;
            } else if (ctx.queuePosition) {
                progress.hidden = false;
                progress.innerHTML = `<div class="vd-mc-progress-text vd-mc-progress-text--queued">${ic('queue')}<span>${esc(t('desktop.mc_overview_queued_position', { position: ctx.queuePosition }))}</span></div>`;
            } else {
                progress.hidden = true;
                progress.innerHTML = '';
            }
        }

        function factRows() {
            const rows = [];
            rows.push([t('desktop.mc_overview_trigger'), esc(triggers.summary(mission, t, { schedule, lang }))]);
            if (mission.execution_type === 'scheduled') {
                let next;
                if (mission.enabled === false) next = esc(t('desktop.mc_overview_paused'));
                else if (mission.next_run) next = `${esc(fmt.dateTime(mission.next_run))} <span class="vd-mc-muted">· ${esc(fmt.relative(mission.next_run))}</span>`;
                else next = esc(t('desktop.mc_overview_not_scheduled'));
                rows.push([t('desktop.mc_overview_next_run'), next]);
            }
            if (mission.last_run) {
                const result = mission.last_result === 'error' ? pill('error', 'vd-mc-pill--small') : mission.last_result === 'success' ? pill('ok', 'vd-mc-pill--small') : '';
                rows.push([t('desktop.mc_overview_last_run'), `${esc(fmt.dateTime(mission.last_run))} <span class="vd-mc-muted">· ${esc(fmt.relative(mission.last_run))}</span> ${result}`]);
            } else {
                rows.push([t('desktop.mc_overview_last_run'), esc(t('desktop.mc_state_never_run'))]);
            }
            rows.push([t('desktop.mc_overview_runs'), esc(String(mission.run_count || 0))]);
            const exec = [];
            exec.push(mission.runner_type === 'remote' ? t('desktop.mc_exec_remote', { target: mission.remote_nest_name || mission.remote_egg_name || mission.remote_nest_id || '' }) : t('desktop.mc_exec_local'));
            const priority = ['low', 'medium', 'high'].includes(mission.priority) ? mission.priority : 'medium';
            exec.push(t('desktop.mc_exec_priority', { value: t('desktop.mc_priority_' + priority) }));
            if (Array.isArray(mission.cheatsheet_ids) && mission.cheatsheet_ids.length) exec.push(t('desktop.mc_exec_cheatsheets', { count: mission.cheatsheet_ids.length }));
            if (mission.auto_prepare) exec.push(t('desktop.mc_exec_auto_prepare'));
            rows.push([t('desktop.mc_overview_execution'), `<ul class="vd-mc-fact-list">${exec.map(item => `<li>${esc(item)}</li>`).join('')}</ul>`]);
            return rows.map(([k, v]) => `<div class="vd-mc-fact"><dt>${esc(k)}</dt><dd>${v}</dd></div>`).join('');
        }

        function prepMarkup() {
            const status = mission.preparation_status || 'none';
            const keys = { none: 'desktop.mc_prep_none', prepared: 'desktop.mc_prep_ready', preparing: 'desktop.mc_prep_preparing', error: 'desktop.mc_prep_failed', stale: 'desktop.mc_prep_stale', low_confidence: 'desktop.mc_prep_stale' };
            const remote = mission.runner_type === 'remote';
            let body = '';
            if (prepared.loading) body = `<div class="vd-mc-muted">${esc(t('desktop.mc_history_loading'))}</div>`;
            else if (prepared.data && prepared.data.analysis) {
                const a = prepared.data.analysis;
                body = `<div class="vd-mc-prep-body">
                    ${a.summary ? `<p>${esc(a.summary)}</p>` : ''}
                    ${Array.isArray(a.step_plan) && a.step_plan.length ? `<ol class="vd-mc-prep-steps">${a.step_plan.map(s => `<li>${esc(s.action || '')}${s.expectation ? ` <span class="vd-mc-muted">— ${esc(s.expectation)}</span>` : ''}</li>`).join('')}</ol>` : ''}
                    ${Array.isArray(a.essential_tools) && a.essential_tools.length ? `<div class="vd-mc-prep-tools">${a.essential_tools.map(tool => `<span class="vd-mc-chip" title="${esc(tool.purpose || '')}">${esc(tool.tool_name || '')}</span>`).join('')}</div>` : ''}
                    ${Array.isArray(a.pitfalls) && a.pitfalls.length ? `<ul class="vd-mc-prep-pitfalls">${a.pitfalls.map(p => `<li>${ic('alert')}<span>${esc(p.risk || '')}${p.mitigation ? ` — ${esc(p.mitigation)}` : ''}</span></li>`).join('')}</ul>` : ''}
                    ${typeof prepared.data.confidence === 'number' ? `<div class="vd-mc-muted">${esc(t('desktop.mc_prep_confidence', { value: Math.round(prepared.data.confidence * 100) }))}</div>` : ''}
                </div>`;
            } else if (prepared.data) body = `<div class="vd-mc-muted">${esc(t('desktop.mc_prep_empty'))}</div>`;
            const actions = [];
            if (status === 'prepared' || status === 'stale' || status === 'low_confidence') {
                actions.push(actionButton('viewPrep', prepared.data ? 'desktop.mc_prep_close' : 'desktop.mc_prep_view', 'info', {}));
                actions.push(actionButton('invalidatePrep', 'desktop.mc_action_invalidate_prep', 'refresh', {}));
            }
            if (status !== 'preparing' && !remote) actions.push(actionButton('prepare', 'desktop.mc_action_prepare', 'sparkles', { disabled: ctx.running }));
            return `<article class="vd-mc-card vd-mc-card--prep">
                <h3 class="vd-mc-card-title">${ic('sparkles')}<span>${esc(t('desktop.mc_prep_title'))}</span><span class="vd-mc-pill vd-mc-pill--small" data-prep="${esc(status)}">${esc(t(keys[status] || keys.none))}</span></h3>
                ${body}
                <div class="vd-mc-card-actions">${actions.join('')}</div>
            </article>`;
        }

        function renderOverview() {
            const output = extractLastOutput(mission.last_output);
            const long = output.length > OUTPUT_PREVIEW_CHARS;
            const shown = long && !outputExpanded ? output.slice(0, OUTPUT_PREVIEW_CHARS) + '…' : output;
            q('[data-mc-panel="overview"]').innerHTML = `<div class="vd-mc-cards">
                <article class="vd-mc-card vd-mc-card--task">
                    <h3 class="vd-mc-card-title">${ic('edit')}<span>${esc(t('desktop.mc_overview_task'))}</span></h3>
                    <pre class="vd-mc-prompt">${esc(mission.prompt || '')}</pre>
                </article>
                <article class="vd-mc-card vd-mc-card--facts"><dl class="vd-mc-facts">${factRows()}</dl></article>
                ${prepMarkup()}
                <article class="vd-mc-card vd-mc-card--output">
                    <h3 class="vd-mc-card-title">${ic('history')}<span>${esc(t('desktop.mc_overview_last_output'))}</span>
                        ${output ? `<button type="button" class="vd-mc-btn vd-mc-btn--icon vd-mc-btn--ghost" data-mc-action="copyOutput" title="${esc(t('desktop.mc_overview_copy_output'))}" aria-label="${esc(t('desktop.mc_overview_copy_output'))}">${ic('copy')}</button>` : ''}
                    </h3>
                    ${output ? `<pre class="vd-mc-output${mission.last_result === 'error' ? ' is-error' : ''}" data-mc-output>${esc(shown)}</pre>${long ? `<button type="button" class="vd-mc-link" data-mc-output-toggle>${esc(t(outputExpanded ? 'desktop.mc_overview_show_less' : 'desktop.mc_overview_show_more'))}</button>` : ''}` : `<div class="vd-mc-muted">${esc(t('desktop.mc_overview_no_output'))}</div>`}
                </article>
            </div>`;
        }

        function triggerLabel(run) {
            const type = run.trigger_type || 'manual';
            if (type === 'manual') return t('desktop.mc_filter_manual');
            if (type === 'scheduled') return t('desktop.mc_filter_scheduled');
            const def = triggers.byKey(type);
            return def ? (typeof triggers.label === 'function' ? triggers.label(def, t) : t(def.labelKey)) : type;
        }

        function runMarkup(run) {
            const cancelled = isCancelledRun(run);
            const state = cancelled ? 'cancelled' : run.status === 'success' ? 'ok' : run.status === 'error' ? 'error' : run.status === 'running' ? 'running' : 'idle';
            const label = cancelled ? t('desktop.mc_state_cancelled') : t(STATE_KEYS[state] || state);
            const expanded = expandedRuns.has(run.id);
            const body = extractLastOutput(run.output) || run.error_msg || '';
            return `<div class="vd-mc-run" role="listitem" data-mc-run="${esc(run.id)}">
                <button type="button" class="vd-mc-run-head" aria-expanded="${expanded}" data-mc-run-toggle title="${esc(t(expanded ? 'desktop.mc_history_collapse' : 'desktop.mc_history_expand'))}">
                    <span class="vd-mc-row-state" data-state="${esc(state)}" aria-hidden="true"></span>
                    <span class="vd-mc-run-started">${esc(fmt.dateTime(run.started_at))}</span>
                    <span class="vd-mc-run-trigger">${esc(triggerLabel(run))}</span>
                    <span class="vd-mc-run-duration">${esc(run.duration_ms ? fmt.duration(run.duration_ms) : '–')}</span>
                    <span class="vd-mc-pill vd-mc-pill--small" data-state="${esc(state)}">${esc(label)}</span>
                    <span class="vd-mc-run-chevron">${ic(expanded ? 'chevronUp' : 'chevronDown')}</span>
                </button>
                <div class="vd-mc-run-body" ${expanded ? '' : 'hidden'}>
                    ${run.error_msg && !cancelled ? `<div class="vd-mc-run-error"><strong>${esc(t('desktop.mc_history_error_label'))}:</strong> ${esc(run.error_msg)}</div>` : ''}
                    <pre class="vd-mc-output">${esc(body || t('desktop.mc_history_no_output'))}</pre>
                </div>
            </div>`;
        }

        function renderHistory() {
            const panel = q('[data-mc-panel="history"]');
            const chips = [['all', 'desktop.mc_history_filter_all'], ['success', 'desktop.mc_history_filter_success'], ['error', 'desktop.mc_history_filter_error'], ['cancelled', 'desktop.mc_history_filter_cancelled']]
                .map(([id, key]) => `<button type="button" class="vd-mc-chip vd-mc-chip--button${history.filter === id ? ' is-active' : ''}" data-mc-history-filter="${id}" aria-pressed="${history.filter === id}">${esc(t(key))}</button>`).join('');
            const items = history.filter === 'cancelled' ? history.items.filter(isCancelledRun) : history.items;
            let list;
            if (history.error) list = `<div class="vd-mc-history-state"><span>${esc(t('desktop.mc_history_error'))}</span><button type="button" class="vd-mc-btn" data-mc-history-retry>${esc(t('desktop.mc_retry'))}</button></div>`;
            else if (!items.length && history.loading) list = `<div class="vd-mc-history-state vd-mc-muted">${esc(t('desktop.mc_history_loading'))}</div>`;
            else if (!items.length) list = `<div class="vd-mc-history-state vd-mc-muted">${esc(t('desktop.mc_history_empty'))}</div>`;
            else list = `<div class="vd-mc-run-header" aria-hidden="true"><span></span><span>${esc(t('desktop.mc_history_col_started'))}</span><span>${esc(t('desktop.mc_history_col_trigger'))}</span><span>${esc(t('desktop.mc_history_col_duration'))}</span><span>${esc(t('desktop.mc_history_col_result'))}</span><span></span></div>` + items.map(runMarkup).join('');
            const more = !history.error && history.items.length < history.total
                ? `<button type="button" class="vd-mc-btn vd-mc-history-more" data-mc-history-more ${history.loading ? 'disabled' : ''}>${esc(t(history.loading ? 'desktop.mc_history_loading' : 'desktop.mc_history_load_more'))}</button>` : '';
            panel.innerHTML = `<div class="vd-mc-history-bar"><div class="vd-mc-chips" role="group">${chips}</div><span class="vd-mc-muted">${esc(t('desktop.mc_history_total', { count: history.total }))}</span></div>
                <div class="vd-mc-history-list" role="list">${list}</div>${more}`;
        }

        function applyTab() {
            element.querySelectorAll('[data-mc-tab]').forEach(btn => { const on = btn.dataset.mcTab === tab; btn.classList.toggle('is-active', on); btn.setAttribute('aria-selected', on ? 'true' : 'false'); btn.tabIndex = on ? 0 : -1; });
            element.querySelectorAll('[data-mc-panel]').forEach(panel => { panel.hidden = panel.dataset.mcPanel !== tab; });
        }

        function showEmpty() {
            mission = null;
            q('[data-mc-detail-empty]').hidden = false;
            q('[data-mc-detail-body]').hidden = true;
        }

        // ── events ──
        element.addEventListener('click', (event) => {
            const action = event.target.closest('[data-mc-action]');
            if (action && mission) {
                if (action.disabled) return;
                const rect = action.getBoundingClientRect();
                events.emit('action', { name: action.dataset.mcAction, missionId: mission.id, x: rect.left, y: rect.bottom + 4 });
                return;
            }
            const tabBtn = event.target.closest('[data-mc-tab]');
            if (tabBtn) { events.emit('tab', tabBtn.dataset.mcTab); return; }
            if (event.target.closest('[data-mc-output-toggle]')) { outputExpanded = !outputExpanded; renderOverview(); return; }
            const chip = event.target.closest('[data-mc-history-filter]');
            if (chip) { events.emit('historyFilter', chip.dataset.mcHistoryFilter); return; }
            if (event.target.closest('[data-mc-history-more]')) { events.emit('historyMore'); return; }
            if (event.target.closest('[data-mc-history-retry]')) { events.emit('historyRetry'); return; }
            const runToggle = event.target.closest('[data-mc-run-toggle]');
            if (runToggle) {
                const id = runToggle.closest('[data-mc-run]').dataset.mcRun;
                if (expandedRuns.has(id)) expandedRuns.delete(id); else expandedRuns.add(id);
                renderHistory();
            }
        });
        element.addEventListener('keydown', (event) => {
            const tabBtn = event.target.closest('[data-mc-tab]');
            if (!tabBtn) return;
            if (event.key === 'ArrowRight' || event.key === 'ArrowLeft') {
                event.preventDefault();
                const next = tab === 'overview' ? 'history' : 'overview';
                events.emit('tab', next);
                const btn = element.querySelector(`[data-mc-tab="${next}"]`);
                if (btn) btn.focus();
            }
        });
        element.addEventListener('contextmenu', (event) => {
            if (!mission || event.target.closest('pre, input, textarea')) return;
            if (!event.target.closest('[data-mc-hero]')) return;
            event.preventDefault();
            events.emit('contextmenu', { x: event.clientX, y: event.clientY, missionId: mission.id });
        });

        return {
            element,
            setMission(next, nextCtx) {
                const changed = !mission || !next || mission.id !== next.id;
                mission = next || null;
                ctx = Object.assign({ running: false, queuePosition: 0, cancelling: false, busy: '' }, nextCtx || {});
                if (!mission) { showEmpty(); return; }
                if (changed) { outputExpanded = false; prepared = { data: null, loading: false }; expandedRuns.clear(); }
                q('[data-mc-detail-empty]').hidden = true;
                q('[data-mc-detail-body]').hidden = false;
                const scroll = element.scrollTop;
                renderHero();
                renderOverview();
                if (changed) renderHistory();
                applyTab();
                element.scrollTop = scroll;
            },
            setTab(next) { tab = next === 'history' ? 'history' : 'overview'; applyTab(); },
            setHistory(state) { history = Object.assign({ items: [], total: 0, loading: false, error: '', filter: 'all' }, state || {}); if (mission) renderHistory(); },
            setPrepared(data, loading) { prepared = { data: data || null, loading: !!loading }; if (mission) renderOverview(); },
            setElapsed(text) { const el = q('[data-mc-elapsed]'); if (el && ctx.running && !ctx.cancelling) el.textContent = t('desktop.mc_overview_running_since', { elapsed: text }); },
            showEmpty,
            on: events.on,
            dispose() { events.clear(); element.replaceChildren(); }
        };
    }

    window.MissionControlDetail = { create, extractLastOutput };
})();
