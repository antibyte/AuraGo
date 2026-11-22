// Chibi Sensei steps: the pure readiness derivation (which state each of the five steps is in, which primary action the guidance
// panel offers and which tip it shows), plus the step rail and the guidance panel built from it. The pure part touches no DOM at
// load time, so a Node test can load it in a vm. Classic script, publishes P.steps.
(function () {
    'use strict';

    const P = window.ChibiSenseiParts = window.ChibiSenseiParts || {};

    const STEP_IDS = ['goal', 'data', 'training', 'test', 'use'];
    const STATES = ['todo', 'in_progress', 'ready', 'done', 'blocked'];
    const DATA_JOB_KINDS = ['generate', 'clean', 'import_file'];
    const ACTIVE = ['queued', 'running'];
    const STATE_KEYS = {
        todo: 'chibi_sensei.readiness.todo',
        in_progress: 'chibi_sensei.readiness.in_progress',
        ready: 'chibi_sensei.readiness.ready',
        done: 'chibi_sensei.readiness.done',
        blocked: 'chibi_sensei.readiness.blocked'
    };
    const STEP_NAME_KEYS = {
        goal: 'chibi_sensei.step.goal',
        data: 'chibi_sensei.step.data',
        training: 'chibi_sensei.step.training',
        test: 'chibi_sensei.step.test',
        use: 'chibi_sensei.step.use'
    };

    function goalComplete(p) {
        if (!p || !p.recipe || !p.profile_id || !String(p.goal_text || '').trim()) return false;
        if (p.recipe !== 'specialist') return true;
        if (!p.task_kind) return false;
        return p.task_kind !== 'classify' || (Array.isArray(p.labels) && p.labels.filter(l => String(l).trim()).length >= 2);
    }

    // input: {project, goalDirty, checks (CheckResult|null), checksStale, jobs: [Job]}
    function derive(input) {
        const i = input || {}, p = i.project || {};
        const server = p.steps && typeof p.steps === 'object' ? p.steps : {};
        const known = s => (STATES.indexOf(s) >= 0 ? s : 'todo');
        const out = {};
        if (i.goalDirty) out.goal = 'in_progress';
        else if (goalComplete(p)) out.goal = 'done';
        else if (p.recipe || p.profile_id || String(p.goal_text || '').trim()) out.goal = 'in_progress';
        else out.goal = 'todo';
        const busy = (Array.isArray(i.jobs) ? i.jobs : []).some(j => j && DATA_JOB_KINDS.indexOf(j.kind) >= 0 && ACTIVE.indexOf(j.state) >= 0);
        const rows = Number(p.rows) || 0;
        const c = i.checks && Array.isArray(i.checks.checks) ? i.checks : null;
        const stale = !!i.checksStale || (c && c.data_rev != null && p.data_rev != null && c.data_rev !== p.data_rev);
        if (busy) out.data = 'in_progress';
        else if (rows === 0) out.data = 'todo';
        else if (!c || stale) out.data = 'in_progress';
        else if (c.checks.some(x => x && x.level === 'block')) out.data = 'blocked';
        else if (c.ready === true) out.data = 'ready';
        else out.data = 'in_progress';
        out.training = known(server.training); out.test = known(server.test); out.use = known(server.use);
        return out;
    }

    function firstOpen(steps) { return steps && steps.goal === 'done' ? 'data' : 'goal'; }

    // opts: {readonly, goalValid, checksRunning, rows}
    function primaryFor(step, steps, opts) {
        const o = opts || {}, s = steps || {};
        const later = key => ({ labelKey: key, action: '', disabled: true, reasonKey: 'chibi_sensei.primary.later' });
        if (step === 'goal') {
            const blocked = !o.readonly && !o.goalValid;
            return { labelKey: 'chibi_sensei.primary.continue_data', action: 'goal_continue', disabled: blocked, reasonKey: blocked ? 'chibi_sensei.primary.goal_incomplete' : '' };
        }
        if (step === 'data') {
            if (s.data === 'ready') return { labelKey: 'chibi_sensei.primary.continue_training', action: 'goto_training', disabled: false, reasonKey: '' };
            const reason = o.readonly ? 'chibi_sensei.readonly_banner' : (Number(o.rows) > 0 ? '' : 'chibi_sensei.primary.no_rows');
            return { labelKey: o.checksRunning ? 'chibi_sensei.checks.running' : 'chibi_sensei.primary.check_dataset', action: 'run_checks', disabled: !!reason || !!o.checksRunning, reasonKey: reason };
        }
        if (step === 'training') return later('chibi_sensei.primary.start_training');
        if (step === 'test') return later('chibi_sensei.primary.chat_model');
        return later('chibi_sensei.primary.export_model');
    }

    const TIPS = {
        goal_todo: 'chibi_sensei.tip.goal_todo', goal_in_progress: 'chibi_sensei.tip.goal_in_progress', goal_done: 'chibi_sensei.tip.goal_done',
        data_todo: 'chibi_sensei.tip.data_todo', data_in_progress: 'chibi_sensei.tip.data_in_progress', data_blocked: 'chibi_sensei.tip.data_blocked', data_ready: 'chibi_sensei.tip.data_ready',
        training_todo: 'chibi_sensei.tip.training_todo', test_todo: 'chibi_sensei.tip.test_todo', use_todo: 'chibi_sensei.tip.use_todo'
    };
    function tipFor(step, state) { return TIPS[step + '_' + state] || TIPS[step + '_todo'] || TIPS.goal_todo; }

    // ---- rail ------------------------------------------------------------------------------------------------------------------

    function createRail(inst) {
        const { el } = P;
        const t = inst.t;
        const buttons = STEP_IDS.map((id, index) => el('button', {
            type: 'button', class: 'csn-step', tabindex: index === 0 ? '0' : '-1', dataset: { csn: 'step', step: id, state: 'todo' },
            onClick: () => inst.showStep(id)
        },
        el('span', { class: 'csn-step-num', text: String(index + 1) }),
        el('span', { class: 'csn-step-name', text: t(STEP_NAME_KEYS[id]) }),
        el('span', { class: 'csn-step-state', text: t(STATE_KEYS.todo) })));
        const list = el('ol', { class: 'csn-step-list' }, buttons.map(b => el('li', {}, b)));
        const node = el('nav', { class: 'csn-rail', dataset: { csn: 'rail' }, ariaLabel: t('chibi_sensei.rail.label') }, list);

        function move(to) {
            const next = buttons[(to + buttons.length) % buttons.length];
            buttons.forEach(b => { b.tabIndex = b === next ? 0 : -1; });
            next.focus();
        }
        buttons.forEach((button, index) => {
            button.addEventListener('keydown', event => {
                let target = null;
                if (event.key === 'ArrowDown') target = index + 1;
                else if (event.key === 'ArrowUp') target = index - 1;
                else if (event.key === 'Home') target = 0;
                else if (event.key === 'End') target = buttons.length - 1;
                if (target === null) return;
                event.preventDefault();
                move(target);
            });
        });

        function paint(derived, active) {
            const d = derived || {};
            buttons.forEach((button, index) => {
                const id = STEP_IDS[index];
                const state = STATES.indexOf(d[id]) >= 0 ? d[id] : 'todo';
                button.dataset.state = state;
                button.tabIndex = id === active ? 0 : -1;
                if (id === active) button.setAttribute('aria-current', 'step'); else button.removeAttribute('aria-current');
                button.setAttribute('aria-label', t('chibi_sensei.rail.step_label', { step: t(STEP_NAME_KEYS[id]), state: t(STATE_KEYS[state]) }));
                button.querySelector('.csn-step-state').textContent = t(STATE_KEYS[state]);
            });
        }

        function focus() {
            const current = buttons.find(b => b.tabIndex === 0) || buttons[0];
            current.focus();
        }

        return { el: node, paint, focus };
    }

    // ---- guidance panel --------------------------------------------------------------------------------------------------------

    function createPanel(inst) {
        const { el } = P;
        const t = inst.t;
        const mascot = P.mascot ? P.mascot.create({ size: 96, mood: 'idle' }) : null;
        const tip = el('p', { class: 'csn-tip', dataset: { csn: 'tip' } });
        const extra = el('div', { class: 'csn-panel-extra', dataset: { csn: 'panel-extra' } });
        let action = '';
        const primaryButton = el('button', { type: 'button', class: 'csn-btn csn-btn-primary', dataset: { csn: 'primary' }, onClick: () => { if (action) inst.runPrimary(action); } });
        const hint = el('p', { class: 'csn-hint', dataset: { csn: 'primary-hint' } });
        const node = el('aside', { class: 'csn-panel', dataset: { csn: 'panel' }, ariaLabel: t('chibi_sensei.panel.title') },
            mascot ? el('div', { class: 'csn-mascot-wrap' }, mascot.el) : null,
            tip, extra, primaryButton, hint,
            el('p', { class: 'csn-muted', text: t('chibi_sensei.panel.chat_later') }));

        function paint(view) {
            const v = view || {};
            const step = STEP_IDS.indexOf(v.step) >= 0 ? v.step : 'goal';
            const derived = v.derived || {};
            const primary = v.primary || { labelKey: 'chibi_sensei.primary.later', action: '', disabled: true, reasonKey: '' };
            tip.textContent = t(tipFor(step, derived[step] || 'todo'));
            action = primary.action || '';
            primaryButton.textContent = t(primary.labelKey);
            primaryButton.disabled = !!primary.disabled || !action;
            const reason = primary.reasonKey ? t(primary.reasonKey) : '';
            primaryButton.title = reason;
            hint.textContent = reason;
            hint.hidden = !reason;
            if (mascot) {
                const jobsActive = Array.from(inst.jobs.values()).some(job => job && ACTIVE.indexOf(job.state) >= 0);
                mascot.setMood(P.mascot.moodFor({
                    jobsActive, checksRunning: !!inst.checksRunning, blocked: derived.data === 'blocked' && step === 'data',
                    jobFailed: !!inst.lastJobFailed, stepState: derived[step]
                }));
            }
        }

        function setExtra(content) {
            P.clear(extra);
            if (content) extra.appendChild(content);
        }

        return { el: node, paint, setExtra };
    }

    P.steps = { derive, firstOpen, primaryFor, tipFor, goalComplete, createRail, createPanel, STEP_IDS, STATES };
})();
