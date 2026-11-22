// Chibi Sensei training step: the read-only plan view with three preset cards (the numbers are estimates, nothing trains in this
// version) and the short "comes later" states of the Test and Use steps. Choosing a card saves the preset on the project.
// Classic script, publishes P.training.
(function () {
    'use strict';

    const P = window.ChibiSenseiParts = window.ChibiSenseiParts || {};
    const PRESETS = ['quick', 'balanced', 'thorough'];
    const PRESET_KEYS = {
        quick: { name: 'chibi_sensei.training.preset_quick', desc: 'chibi_sensei.training.preset_quick_desc' },
        balanced: { name: 'chibi_sensei.training.preset_balanced', desc: 'chibi_sensei.training.preset_balanced_desc' },
        thorough: { name: 'chibi_sensei.training.preset_thorough', desc: 'chibi_sensei.training.preset_thorough_desc' }
    };
    const LATER_KEYS = { test: 'chibi_sensei.later.test_text', use: 'chibi_sensei.later.use_text' };
    const REFRESH_MS = 1000;

    function lrText(value) {
        const v = Number(value);
        if (!Number.isFinite(v)) return '0';
        try { return new Intl.NumberFormat(String((typeof window !== 'undefined' && window.SYSTEM_LANG) || 'en'), { maximumSignificantDigits: 2 }).format(v); } catch (_) { return String(v); }
    }

    function create(inst) {
        const { el } = P;
        const t = inst.t;
        const num = P.format.number;
        const node = el('section', { class: 'csn-training', dataset: { csn: 'training' } });
        const error = el('p', { class: 'csn-error', role: 'alert', dataset: { csn: 'training-error' } });
        error.hidden = true;
        const grid = el('div', { class: 'csn-preset-grid', role: 'group', ariaLabel: t('chibi_sensei.training.title'), dataset: { csn: 'preset-grid' } });
        const note = el('div', { class: 'csn-note', dataset: { csn: 'plan-note' } });
        const advanced = el('details', { class: 'csn-advanced', dataset: { csn: 'training-advanced' } },
            el('summary', { text: t('chibi_sensei.training.advanced') }),
            el('p', { class: 'csn-muted', text: t('chibi_sensei.training.advanced_text') }));
        const body = el('div', { class: 'csn-training-body' });
        node.appendChild(el('h2', { class: 'csn-step-title', text: t('chibi_sensei.training.title') }));
        node.appendChild(el('p', { class: 'csn-muted', text: t('chibi_sensei.training.intro') }));
        node.appendChild(error);
        node.appendChild(body);
        let disposed = false;
        let plans = null;
        let seq = 0;
        let saving = false;

        const showError = text => { error.textContent = text || ''; error.hidden = !text; };

        function empty(key, withGoal) {
            P.clear(body);
            const card = el('div', { class: 'csn-state csn-state-inline', role: 'status', dataset: { csn: 'training-empty' } }, el('p', { text: t(key) }));
            if (withGoal) card.appendChild(el('button', { type: 'button', class: 'csn-btn csn-btn-primary', dataset: { csn: 'training-to-goal' }, text: t('chibi_sensei.training.to_goal'), onClick: () => inst.showStep('goal') }));
            body.appendChild(card);
        }

        async function choose(preset) {
            if (inst.readonly || saving || !inst.project || inst.project.preset === preset) return;
            saving = true;
            showError('');
            const mine = inst.projectSeq;
            try {
                const updated = await inst.api.patchProject(inst.project.id, inst.project.rev, { preset });
                if (mine !== inst.projectSeq || inst.disposed) return;
                inst.project = updated;
                inst.bus.emit('project', updated);
                inst.recompute();
                if (disposed) return;
                inst.notify(t('chibi_sensei.saved'), 'info');
            } catch (err) {
                if (disposed || (err && err.aborted)) return;
                showError(inst.handleError(err));
            } finally {
                saving = false;
                if (!disposed) paint();
            }
        }

        function line(text, name) { return el('li', { class: 'csn-plan-line', dataset: { csn: name }, text }); }

        function card(preset, plan) {
            const h = plan.hyper || {};
            const selected = !!inst.project && inst.project.preset === preset;
            const names = PRESET_KEYS[preset];
            const btn = el('button', { type: 'button', class: 'csn-preset-card', ariaPressed: selected ? 'true' : 'false', dataset: { csn: 'preset-card', preset }, onClick: () => choose(preset) });
            btn.disabled = inst.readonly;
            btn.title = inst.readonly ? t('chibi_sensei.readonly_banner') : '';
            const head = el('span', { class: 'csn-preset-head' }, el('strong', { text: t(names.name) }), selected ? el('span', { class: 'csn-badge', text: t('chibi_sensei.training.selected') }) : null);
            const list = el('ul', { class: 'csn-plan-list' });
            list.appendChild(line(t('chibi_sensei.training.estimate', { minutes: P.format.minutes(plan.estimate_minutes) }), 'plan-estimate'));
            list.appendChild(line(t('chibi_sensei.training.timeout', { minutes: P.format.minutes(plan.session_timeout_minutes) }), 'plan-timeout'));
            list.appendChild(line(t('chibi_sensei.training.epochs', { value: num(h.epochs) }), 'plan-epochs'));
            list.appendChild(line(t('chibi_sensei.training.lr', { value: lrText(h.learning_rate) }), 'plan-lr'));
            list.appendChild(line(t('chibi_sensei.training.batch', { batch: num(h.per_device_batch), accum: num(h.grad_accum) }), 'plan-batch'));
            list.appendChild(line(t('chibi_sensei.training.seq', { value: num(h.max_seq_len) }), 'plan-seq'));
            if (plan.method === 'full') list.appendChild(line(t('chibi_sensei.training.method_full'), 'plan-method'));
            else {
                list.appendChild(line(t('chibi_sensei.training.method_lora'), 'plan-method'));
                if (h.lora) list.appendChild(line(t('chibi_sensei.training.lora', { r: num(h.lora.r), alpha: num(h.lora.alpha), dropout: lrText(h.lora.dropout) }), 'plan-lora'));
            }
            const sp = plan.split || {};
            list.appendChild(line(t('chibi_sensei.training.split', { train: num(sp.train), val: num(sp.val), eval: num(sp.eval) }), 'plan-split'));
            btn.appendChild(head);
            btn.appendChild(el('span', { class: 'csn-muted', text: t(names.desc) }));
            btn.appendChild(list);
            return btn;
        }

        function paint() {
            if (disposed) return;
            if (!plans) return;
            P.clear(body);
            P.clear(grid);
            PRESETS.forEach((preset, i) => { if (plans[i]) grid.appendChild(card(preset, plans[i])); });
            const first = plans.find(Boolean) || {};
            P.clear(note);
            note.appendChild(el('span', { text: P.errors.knownKey(t, first.reason_key, 'chibi_sensei.plan.training_later') }));
            body.appendChild(grid);
            body.appendChild(note);
            body.appendChild(advanced);
        }

        async function refresh() {
            if (disposed || !inst.project) return;
            if (!inst.project.profile_id) { plans = null; empty('chibi_sensei.training.no_profile', true); return; }
            if (!(Number(inst.project.rows) > 0)) { plans = null; empty('chibi_sensei.training.no_data', false); return; }
            const mine = ++seq;
            const id = inst.project.id;
            try {
                const result = await Promise.all(PRESETS.map(preset => inst.api.plan(id, preset)));
                if (disposed || mine !== seq) return;
                plans = result;
                showError('');
                paint();
            } catch (err) {
                if (disposed || mine !== seq || (err && err.aborted)) return;
                showError(inst.handleError(err));
            }
        }

        const again = P.debounce(refresh, REFRESH_MS);
        const offs = [
            inst.bus.on('rows', again),
            inst.bus.on('project', () => { if (plans && inst.project && inst.project.profile_id) paint(); else again(); }),
            inst.bus.on('readonly', paint)
        ];
        refresh();
        return {
            el: node, refresh,
            dispose() { disposed = true; again.cancel(); offs.forEach(off => { if (typeof off === 'function') off(); }); }
        };
    }

    // Test and Use: a short state card; the mascot rests.
    function createLater(inst, step) {
        const { el } = P;
        const t = inst.t;
        const art = P.mascot ? P.mascot.create({ size: 88, mood: 'idle' }).el : null;
        const node = el('section', { class: 'csn-later', role: 'status', dataset: { csn: 'later', step: step } },
            art, el('h2', { text: t('chibi_sensei.later.title') }), el('p', { text: t(LATER_KEYS[step] || LATER_KEYS.test) }));
        return { el: node, dispose() {} };
    }

    P.training = { create, createLater };
})();
