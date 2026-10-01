(function () {
    'use strict';

    const instances = new Map();
    const COMPACT_WIDTH = 820;
    const DRAFT_KEY = 'aurago.looper.draft.v1';
    const FIELD_KEYS = ['goal', 'work', 'evaluate', 'finish', 'name', 'max', 'score', 'stall', 'provider', 'model'];

    function formatCost(usd, t) {
        if (!usd || usd <= 0) return '';
        if (usd < 0.01) return t('desktop.looper_cost_under');
        return t('desktop.looper_cost', { amount: usd.toFixed(usd < 1 ? 3 : 2) });
    }

    function formatDuration(ms, t) {
        const value = Number(ms);
        if (!value || value < 0) return t('desktop.looper_duration_ms', { count: 0 });
        if (value < 1000) return t('desktop.looper_duration_ms', { count: value });
        return t('desktop.looper_duration_s', { count: (value / 1000).toFixed(1) });
    }

    function icon(name) {
        const paths = {
            play: '<polygon points="5 3 19 12 5 21 5 3"></polygon>',
            pause: '<rect x="6" y="5" width="4" height="14" rx="1"></rect><rect x="14" y="5" width="4" height="14" rx="1"></rect>',
            stop: '<rect x="6" y="6" width="12" height="12" rx="2"></rect>',
            plus: '<line x1="12" y1="5" x2="12" y2="19"></line><line x1="5" y1="12" x2="19" y2="12"></line>',
            save: '<path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"></path><polyline points="17 21 17 13 7 13 7 21"></polyline>',
            copy: '<rect x="9" y="9" width="13" height="13" rx="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>',
            trash: '<polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>',
            edit: '<path d="M12 20h9"></path><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4z"></path>'
        };
        return '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">' + (paths[name] || '') + '</svg>';
    }

    function presetTitle(p, t) {
        if (p && p.builtin_key) {
            return t('desktop.looper_example_' + p.builtin_key);
        }
        return (p && p.name) || t('desktop.looper_untitled');
    }

    function presetDesc(p, t) {
        if (p && p.builtin_key) {
            return t('desktop.looper_example_' + p.builtin_key + '_desc');
        }
        return String((p && p.goal) || '').replace(/\s+/g, ' ').slice(0, 90);
    }

    function emptyDraft() {
        return {
            name: '',
            goal: '',
            work: '',
            evaluate: '',
            finish: '',
            max_rounds: 10,
            target_score: 85,
            stall_rounds: 3,
            provider_id: '',
            model: ''
        };
    }

    // The same normalisation for a loaded preset and for the live form, so
    // "unsaved changes" means a real difference and not whitespace or defaults.
    function fingerprint(d) {
        return JSON.stringify([
            String(d.name || '').trim(), String(d.goal || '').trim(), String(d.work || '').trim(),
            String(d.evaluate || '').trim(), String(d.finish || '').trim(),
            Number(d.max_rounds) || 10, Number(d.target_score) || 85,
            d.stall_rounds == null || d.stall_rounds === '' ? 3 : Number(d.stall_rounds),
            d.provider_id || '', d.model || ''
        ]);
    }

    // The settings a run was started with (history record or active run).
    function draftFromConfig(cfg, fallbackName) {
        return {
            name: cfg.preset_name || fallbackName || '',
            goal: cfg.goal || '',
            work: cfg.work || '',
            evaluate: cfg.evaluate || '',
            finish: cfg.finish || '',
            max_rounds: cfg.max_rounds || 10,
            target_score: cfg.target_score || 85,
            stall_rounds: cfg.stall_rounds == null ? 3 : cfg.stall_rounds,
            provider_id: cfg.provider_id || '',
            model: cfg.model || ''
        };
    }

    function errorCode(e) {
        return (e && e.body && e.body.code) || '';
    }

    function loadDraft() {
        try {
            const raw = JSON.parse(window.localStorage.getItem(DRAFT_KEY) || 'null');
            return raw && raw.fields && typeof raw.fields === 'object' ? raw : null;
        } catch (e) {
            return null;
        }
    }

    function storeDraft(value) {
        try {
            if (value) window.localStorage.setItem(DRAFT_KEY, JSON.stringify(value));
            else window.localStorage.removeItem(DRAFT_KEY);
        } catch (e) { /* storage may be blocked */ }
    }

    function render(container, windowId, context) {
        dispose(windowId);

        const { esc, t, api, notify, readonly, promptDialog, confirmDialog } = context;
        const isReadonly = !!readonly;
        const monitor = window.LooperMonitor;

        const askPrompt = async (title, value) => {
            if (typeof promptDialog === 'function') {
                return promptDialog(title, value || '');
            }
            if (notify) notify({ title: t('desktop.notification'), message: t('desktop.looper_error') });
            return null;
        };
        const askConfirm = async (title, message) => {
            if (typeof confirmDialog === 'function') {
                return confirmDialog(title, message || '');
            }
            if (notify) notify({ title: t('desktop.notification'), message: t('desktop.looper_error') });
            return false;
        };
        const toast = (message) => {
            if (notify) notify({ title: t('desktop.notification'), message: message });
        };

        const state = {
            presets: [],
            providers: [],
            selectedPresetId: null,
            baseline: fingerprint(emptyDraft()),
            dirty: false,
            focus: false,
            status: { status: 'idle', current_step: 'idle', round: 0, max_rounds: 10, logs: [] },
            sse: null,
            disposed: false,
            compact: false,
            pane: 'setup',
            history: [],
            historyDetail: null,
            historyView: null,
            runView: null,
            logCache: monitor ? monitor.newLogCache() : null,
            draftTimer: 0,
            persistDraft: null
        };
        instances.set(windowId, state);

        const dis = isReadonly ? ' disabled' : '';
        container.innerHTML =
            '<div class="vd-looper' + (isReadonly ? ' vd-looper--readonly' : '') + '" data-pane="setup">' +
            '<div class="vd-looper-tabs" role="tablist">' +
            tabBtn(windowId, 'setup', t('desktop.looper_tab_setup')) +
            tabBtn(windowId, 'run', t('desktop.looper_tab_run')) +
            tabBtn(windowId, 'history', t('desktop.looper_tab_history')) +
            '</div>' +
            '<aside class="vd-looper-list" data-pane="setup">' +
            '<div class="vd-looper-list-scroll" id="looper-presets-' + windowId + '"></div>' +
            '<button type="button" class="vd-looper-new" id="looper-new-' + windowId + '"' + dis + '>' +
            icon('plus') + '<span>' + esc(t('desktop.looper_new')) + '</span></button>' +
            '</aside>' +
            '<section class="vd-looper-editor" data-pane="setup">' +
            '<div class="vd-looper-brief" id="looper-brief-' + windowId + '" hidden>' +
            '<div class="vd-looper-brief-head"><h3 class="vd-looper-brief-name" id="looper-brief-name-' + windowId + '"></h3>' +
            '<button type="button" class="vd-looper-brief-edit" id="looper-edit-' + windowId + '">' + icon('edit') + '<span>' + esc(t('desktop.looper_edit')) + '</span></button></div>' +
            '<p class="vd-looper-brief-goal" id="looper-brief-goal-' + windowId + '"></p>' +
            '<ul class="vd-looper-chips" id="looper-brief-chips-' + windowId + '"></ul>' +
            briefDetails(esc, t, windowId, 'work') +
            briefDetails(esc, t, windowId, 'evaluate') +
            briefDetails(esc, t, windowId, 'finish') +
            '</div>' +
            '<div class="vd-looper-editor-toolbar">' +
            '<input type="text" inputmode="text" enterkeyhint="next" id="looper-name-' + windowId + '" class="vd-looper-name" placeholder="' + esc(t('desktop.looper_name')) + '"' + dis + '>' +
            '<span class="vd-looper-dirty" id="looper-dirty-' + windowId + '" title="' + esc(t('desktop.looper_unsaved')) + '" role="img" aria-label="' + esc(t('desktop.looper_unsaved')) + '" hidden></span>' +
            '<div class="vd-looper-editor-actions">' +
            '<button type="button" class="vd-looper-icon-btn" id="looper-save-' + windowId + '" title="' + esc(t('desktop.looper_save')) + '" aria-label="' + esc(t('desktop.looper_save')) + '"' + dis + '>' + icon('save') + '</button>' +
            '<button type="button" class="vd-looper-icon-btn" id="looper-dup-' + windowId + '" title="' + esc(t('desktop.looper_duplicate')) + '" aria-label="' + esc(t('desktop.looper_duplicate')) + '"' + dis + '>' + icon('copy') + '</button>' +
            '<button type="button" class="vd-looper-icon-btn vd-looper-btn-delete" id="looper-delete-' + windowId + '" title="' + esc(t('desktop.looper_delete')) + '" aria-label="' + esc(t('desktop.looper_delete')) + '"' + dis + '>' + icon('trash') + '</button>' +
            '</div></div>' +
            '<div class="vd-looper-settings" role="group" aria-label="' + esc(t('desktop.looper_settings')) + '">' +
            '<label class="vd-looper-setting">' + esc(t('desktop.looper_max_rounds')) +
            '<input type="number" inputmode="numeric" enterkeyhint="done" id="looper-max-' + windowId + '" min="1" max="50" value="10"' + dis + '></label>' +
            '<label class="vd-looper-setting vd-looper-setting-range">' + esc(t('desktop.looper_target_score')) +
            '<span class="vd-looper-range-row"><input type="range" id="looper-score-' + windowId + '" min="50" max="100" value="85"' + dis + '>' +
            '<output id="looper-score-out-' + windowId + '">85</output></span></label>' +
            '<label class="vd-looper-setting">' + esc(t('desktop.looper_provider')) +
            '<select id="looper-provider-' + windowId + '"' + dis + '></select></label>' +
            '<label class="vd-looper-setting">' + esc(t('desktop.looper_model')) +
            '<select id="looper-model-' + windowId + '"' + dis + '></select></label>' +
            '</div>' +
            fieldCard(esc, t, windowId, '1', 'goal', true) +
            fieldCard(esc, t, windowId, '2', 'work', true) +
            fieldCard(esc, t, windowId, '3', 'evaluate', true) +
            '<details class="vd-looper-disclosure" id="looper-finish-wrap-' + windowId + '">' +
            '<summary>' + esc(t('desktop.looper_finish_toggle')) + '</summary>' +
            fieldCard(esc, t, windowId, '4', 'finish', false) +
            '</details>' +
            '<details class="vd-looper-disclosure">' +
            '<summary>' + esc(t('desktop.looper_more_options')) + '</summary>' +
            '<label class="vd-looper-setting">' + esc(t('desktop.looper_stall_rounds')) +
            '<input type="number" inputmode="numeric" enterkeyhint="done" id="looper-stall-' + windowId + '" min="0" max="10" value="3" title="' + esc(t('desktop.looper_stall_rounds_help')) + '"' + dis + '>' +
            '<span class="vd-looper-help">' + esc(t('desktop.looper_stall_rounds_help')) + '</span></label>' +
            '</details></section>' +
            '<section class="vd-looper-side">' +
            '<div class="vd-looper-side-tabs">' +
            '<button type="button" class="vd-looper-side-tab is-active" data-side="run">' + esc(t('desktop.looper_tab_run')) + '</button>' +
            '<button type="button" class="vd-looper-side-tab" data-side="history">' + esc(t('desktop.looper_tab_history')) + '</button>' +
            '</div>' +
            '<div class="vd-looper-side-pane is-active" data-side-pane="run" id="looper-run-' + windowId + '"></div>' +
            '<div class="vd-looper-side-pane" data-side-pane="history" id="looper-history-' + windowId + '"></div>' +
            '<button type="button" class="vd-looper-jump-bottom" id="looper-jump-' + windowId + '">' + esc(t('desktop.looper_jump_bottom')) + '</button>' +
            '</section>' +
            '<div class="vd-looper-actionbar" id="looper-actionbar-' + windowId + '">' +
            '<button type="button" class="vd-looper-start" id="looper-start-' + windowId + '"' + dis + '>' + icon('play') + '<span>' + esc(t('desktop.looper_start')) + '</span></button>' +
            '<button type="button" class="vd-looper-pause" id="looper-pause-' + windowId + '" disabled>' + icon('pause') + '<span class="vd-looper-pause-label">' + esc(t('desktop.looper_pause')) + '</span></button>' +
            '<button type="button" class="vd-looper-resume" id="looper-resume-' + windowId + '" hidden>' + icon('play') + '<span>' + esc(t('desktop.looper_resume')) + '</span></button>' +
            '<button type="button" class="vd-looper-stop" id="looper-stop-' + windowId + '" disabled>' + icon('stop') + '<span class="vd-looper-stop-label">' + esc(t('desktop.looper_stop')) + '</span></button>' +
            '</div></div>';

        const $ = id => container.querySelector('#' + id);
        const root = container.querySelector('.vd-looper');
        const side = container.querySelector('.vd-looper-side');
        const runPane = $(`looper-run-${windowId}`);

        function helpers() {
            return {
                esc: esc,
                t: t,
                formatCost: function (usd) { return formatCost(usd, t); },
                formatDuration: function (ms) { return formatDuration(ms, t); }
            };
        }

        function activeRun() {
            return !!(state.status.running || state.status.paused || state.status.status === 'paused');
        }

        // layout
        function setPane(pane) {
            state.pane = pane;
            root.querySelectorAll('.vd-looper-tab').forEach(btn => {
                btn.classList.toggle('is-active', btn.dataset.pane === pane);
            });
            root.dataset.pane = pane;
            if (pane === 'run' || pane === 'history') {
                setSide(pane);
                if (pane === 'history') loadHistory();
            }
        }

        function setSide(sideName) {
            root.querySelectorAll('.vd-looper-side-tab').forEach(btn => {
                btn.classList.toggle('is-active', btn.dataset.side === sideName);
            });
            root.querySelectorAll('.vd-looper-side-pane').forEach(el => {
                el.classList.toggle('is-active', el.dataset.sidePane === sideName);
            });
        }

        function applyCompact() {
            const compact = root.clientWidth > 0 && root.clientWidth < COMPACT_WIDTH;
            state.compact = compact;
            root.classList.toggle('vd-looper--compact', compact);
        }

        const ro = new ResizeObserver(applyCompact);
        ro.observe(root);
        state.resizeObserver = ro;
        applyCompact();
        setPane(state.pane);

        root.querySelectorAll('.vd-looper-tab').forEach(btn => {
            btn.addEventListener('click', () => setPane(btn.dataset.pane));
        });
        root.querySelectorAll('.vd-looper-side-tab').forEach(btn => {
            btn.addEventListener('click', () => {
                setSide(btn.dataset.side);
                if (btn.dataset.side === 'history') loadHistory();
            });
        });

        // form
        function selectedPreset() {
            return state.presets.find(p => String(p.id) === String(state.selectedPresetId)) || null;
        }

        function draftFromPreset(p) {
            return {
                name: p.is_builtin ? presetTitle(p, t) : (p.name || ''),
                goal: p.goal || '',
                work: p.work || '',
                evaluate: p.evaluate || '',
                finish: p.finish || '',
                max_rounds: p.max_rounds || 10,
                target_score: p.target_score || 85,
                stall_rounds: p.stall_rounds == null ? 3 : p.stall_rounds,
                provider_id: p.provider_id || '',
                model: p.model || ''
            };
        }

        function currentFields() {
            const stall = parseInt($(`looper-stall-${windowId}`).value, 10);
            return {
                name: $(`looper-name-${windowId}`).value.trim(),
                goal: $(`looper-goal-${windowId}`).value,
                work: $(`looper-work-${windowId}`).value,
                evaluate: $(`looper-evaluate-${windowId}`).value,
                finish: $(`looper-finish-${windowId}`).value,
                max_rounds: parseInt($(`looper-max-${windowId}`).value, 10) || 10,
                target_score: parseInt($(`looper-score-${windowId}`).value, 10) || 85,
                stall_rounds: Number.isFinite(stall) && stall >= 0 ? stall : 3,
                provider_id: $(`looper-provider-${windowId}`).value,
                model: $(`looper-model-${windowId}`).value
            };
        }

        function refreshModels(wanted) {
            const providerId = $(`looper-provider-${windowId}`).value;
            const select = $(`looper-model-${windowId}`);
            const keep = wanted == null ? select.value : wanted;
            const models = [];
            const provider = state.providers.find(p => p.id === providerId);
            if (provider && provider.model) models.push(provider.model);
            (provider && provider.models ? provider.models : []).forEach(m => {
                if (m && models.indexOf(m) < 0) models.push(m);
            });
            // A saved model that the provider list no longer offers must survive
            // loading, otherwise merely opening a loop would change it.
            if (keep && models.indexOf(keep) < 0) models.push(keep);
            select.innerHTML = '<option value="">' + esc(t('desktop.looper_model_default')) + '</option>';
            models.forEach(model => {
                const opt = document.createElement('option');
                opt.value = model;
                opt.textContent = model;
                select.appendChild(opt);
            });
            select.value = keep || '';
        }

        function writeForm(d) {
            $(`looper-name-${windowId}`).value = d.name || '';
            $(`looper-goal-${windowId}`).value = d.goal || '';
            $(`looper-work-${windowId}`).value = d.work || '';
            $(`looper-evaluate-${windowId}`).value = d.evaluate || '';
            $(`looper-finish-${windowId}`).value = d.finish || '';
            $(`looper-max-${windowId}`).value = d.max_rounds || 10;
            $(`looper-score-${windowId}`).value = d.target_score || 85;
            $(`looper-score-out-${windowId}`).value = d.target_score || 85;
            $(`looper-stall-${windowId}`).value = d.stall_rounds == null ? 3 : d.stall_rounds;
            $(`looper-provider-${windowId}`).value = d.provider_id || '';
            refreshModels(d.model || '');
            const finishWrap = $(`looper-finish-wrap-${windowId}`);
            if (finishWrap) finishWrap.open = !!String(d.finish || '').trim();
        }

        // Loads a draft into the form and makes it the clean reference state.
        function loadIntoForm(d) {
            writeForm(d);
            state.baseline = fingerprint(d);
            updateDirty();
        }

        function readForm() {
            const fields = currentFields();
            return Object.assign({ preset_name: fields.name }, fields);
        }

        function validateForm(body) {
            for (const field of ['goal', 'work', 'evaluate']) {
                if (!String(body[field] || '').trim()) {
                    return t('desktop.looper_field_required', { field: t('desktop.looper_' + field) });
                }
            }
            return '';
        }

        function persistDraft() {
            if (!state.dirty) {
                storeDraft(null);
                return;
            }
            storeDraft({ presetId: state.selectedPresetId, fields: currentFields(), at: Date.now() });
        }
        state.persistDraft = persistDraft;

        function updateDirty() {
            state.dirty = fingerprint(currentFields()) !== state.baseline;
            root.classList.toggle('is-dirty', state.dirty);
            $(`looper-dirty-${windowId}`).hidden = !state.dirty;
            root.querySelectorAll('.vd-looper-preset.is-active').forEach(btn => btn.classList.toggle('is-dirty', state.dirty));
            clearTimeout(state.draftTimer);
            state.draftTimer = setTimeout(persistDraft, 500);
            renderBrief();
        }

        async function confirmDiscard() {
            if (!state.dirty) return true;
            return askConfirm(t('desktop.looper_discard_title'), t('desktop.looper_discard_confirm'));
        }

        function restoreDraft() {
            const saved = loadDraft();
            if (!saved || activeRun()) return;
            const preset = state.presets.find(p => String(p.id) === String(saved.presetId));
            const base = preset ? draftFromPreset(preset) : emptyDraft();
            const draft = Object.assign({}, base, saved.fields);
            if (fingerprint(draft) === fingerprint(base)) {
                storeDraft(null);
                return;
            }
            state.selectedPresetId = preset ? String(preset.id) : null;
            writeForm(draft);
            state.baseline = fingerprint(base);
            renderPresets();
            updateDirty();
            toast(t('desktop.looper_draft_restored'));
        }

        // summary card shown while the run has the stage
        function renderBrief() {
            const brief = $(`looper-brief-${windowId}`);
            if (!brief) return;
            brief.hidden = !state.focus;
            if (!state.focus) return;
            const st = state.status;
            const form = currentFields();
            const running = activeRun();
            const name = (running && st.preset_name) || form.name || t('desktop.looper_untitled');
            const goal = (running && st.goal_excerpt) || form.goal;
            const rounds = (running && st.max_rounds) || form.max_rounds;
            const target = (running && st.target_score) || form.target_score;
            const provider = state.providers.find(p => p.id === form.provider_id);
            const model = form.model || (provider && provider.name) || t('desktop.looper_model_default');
            $(`looper-brief-name-${windowId}`).textContent = name;
            const goalEl = $(`looper-brief-goal-${windowId}`);
            goalEl.textContent = goal;
            goalEl.hidden = !String(goal || '').trim();
            const chips = $(`looper-brief-chips-${windowId}`);
            chips.textContent = '';
            [t('desktop.looper_chip_rounds', { count: rounds }), t('desktop.looper_target_short', { score: target }), model].forEach(text => {
                const li = document.createElement('li');
                li.textContent = text;
                chips.appendChild(li);
            });
            ['work', 'evaluate', 'finish'].forEach(key => {
                const details = $(`looper-brief-${key}-${windowId}`);
                const text = String(form[key] || '').trim();
                details.hidden = !text;
                details.querySelector('p').textContent = text;
            });
            $(`looper-edit-${windowId}`).disabled = !!st.running;
        }

        function setFocus(on) {
            state.focus = on;
            root.classList.toggle('is-focus', on);
            renderBrief();
        }

        // presets
        function renderPresets() {
            const host = $(`looper-presets-${windowId}`);
            const builtins = state.presets.filter(p => p.is_builtin);
            const users = state.presets.filter(p => !p.is_builtin);
            let html = '';
            if (builtins.length) {
                html += '<div class="vd-looper-group-label">' + esc(t('desktop.looper_examples')) + '</div>';
                html += builtins.map(p => presetButton(p)).join('');
            }
            html += '<div class="vd-looper-group-label">' + esc(t('desktop.looper_my_loops')) + '</div>';
            html += users.length ? users.map(p => presetButton(p)).join('') : '<div class="vd-looper-list-empty">' + esc(t('desktop.looper_my_loops_empty')) + '</div>';
            host.innerHTML = html;
            host.querySelectorAll('[data-preset-id]').forEach(btn => {
                btn.addEventListener('click', async () => {
                    if (activeRun() || String(btn.dataset.presetId) === String(state.selectedPresetId)) return;
                    if (!await confirmDiscard()) return;
                    state.selectedPresetId = btn.dataset.presetId;
                    const p = selectedPreset();
                    if (p) loadIntoForm(draftFromPreset(p));
                    setFocus(false);
                    renderPresets();
                });
            });
        }

        function presetButton(p) {
            const active = String(p.id) === String(state.selectedPresetId);
            return '<button type="button" class="vd-looper-preset' + (active ? ' is-active' : '') + (active && state.dirty ? ' is-dirty' : '') + (p.is_builtin ? ' is-example' : '') + '" data-preset-id="' + esc(String(p.id)) + '">' +
                '<span class="vd-looper-preset-name">' + esc(presetTitle(p, t)) + '</span>' +
                '<span class="vd-looper-preset-desc">' + esc(presetDesc(p, t)) + '</span></button>';
        }

        async function loadProviders() {
            const select = $(`looper-provider-${windowId}`);
            select.innerHTML = '<option value="">' + esc(t('desktop.looper_default_provider')) + '</option>';
            try {
                const res = await api('/api/providers');
                const providers = Array.isArray(res) ? res : ((res && res.providers) || []);
                state.providers = providers;
                providers.forEach(p => {
                    const opt = document.createElement('option');
                    opt.value = p.id;
                    opt.textContent = p.name || p.id;
                    select.appendChild(opt);
                });
                refreshModels();
            } catch (e) { /* ignore */ }
        }

        async function loadPresets() {
            try {
                const res = await api('/api/desktop/looper/presets');
                state.presets = (res && res.presets) || [];
                if (state.selectedPresetId && !state.presets.some(p => String(p.id) === String(state.selectedPresetId))) {
                    state.selectedPresetId = null;
                }
                renderPresets();
            } catch (e) {
                toast(t('desktop.looper_error'));
            }
        }

        // history
        async function loadHistory(force) {
            const host = $(`looper-history-${windowId}`);
            if (!host) return;
            if (state.historyDetail && !force && monitor) {
                if (state.historyView) state.historyView.destroy();
                state.historyView = monitor.renderHistoryDetail(host, state.historyDetail, helpers());
                bindHistory();
                return;
            }
            try {
                const res = await api('/api/desktop/looper/runs');
                state.history = (res && res.runs) || [];
                if (state.historyView) { state.historyView.destroy(); state.historyView = null; }
                if (monitor) monitor.renderHistoryList(host, state.history, helpers());
                bindHistory();
            } catch (e) {
                host.innerHTML = '<div class="vd-looper-log-empty">' + esc(t('desktop.looper_history_load_error')) + '</div>';
            }
        }

        function bindHistory() {
            const host = $(`looper-history-${windowId}`);
            host.querySelectorAll('.vd-looper-history-item').forEach(btn => {
                btn.addEventListener('click', async () => {
                    try {
                        const res = await api('/api/desktop/looper/runs/' + btn.dataset.runId);
                        state.historyDetail = res && res.run;
                        loadHistory();
                    } catch (e) {
                        toast(t('desktop.looper_history_load_error'));
                    }
                });
            });
            host.querySelectorAll('.vd-looper-history-delete').forEach(btn => {
                btn.addEventListener('click', async ev => {
                    ev.stopPropagation();
                    if (isReadonly) return;
                    if (!await askConfirm(t('desktop.looper_history_delete'), t('desktop.looper_history_delete_confirm'))) return;
                    try {
                        await api('/api/desktop/looper/runs/' + btn.dataset.runId, { method: 'DELETE' });
                        state.historyDetail = null;
                        loadHistory(true);
                    } catch (e) {
                        toast(t('desktop.looper_delete_error'));
                    }
                });
            });
            const clearBtn = host.querySelector('.vd-looper-history-clear');
            if (clearBtn) {
                clearBtn.addEventListener('click', async () => {
                    if (isReadonly) return;
                    if (!await askConfirm(t('desktop.looper_history_clear'), t('desktop.looper_history_clear_confirm'))) return;
                    try {
                        await api('/api/desktop/looper/runs', { method: 'DELETE' });
                        state.historyDetail = null;
                        loadHistory(true);
                    } catch (e) {
                        toast(t('desktop.looper_delete_error'));
                    }
                });
            }
            const back = host.querySelector('.vd-looper-history-back');
            if (back) {
                back.addEventListener('click', () => {
                    state.historyDetail = null;
                    loadHistory(true);
                });
            }
            const reuse = async startNow => {
                const run = state.historyDetail;
                if (!run || !run.config || activeRun() || isReadonly) return;
                if (!await confirmDiscard()) return;
                state.selectedPresetId = null;
                loadIntoForm(draftFromConfig(run.config, run.preset_name));
                setFocus(false);
                renderPresets();
                setPane('setup');
                if (startNow) startOrResume('/api/desktop/looper/run');
            };
            const load = host.querySelector('.vd-looper-history-load');
            if (load) load.addEventListener('click', () => reuse(false));
            const again = host.querySelector('.vd-looper-history-rerun');
            if (again) again.addEventListener('click', () => reuse(true));
        }

        // live run
        // A run restored after a restart, or started in another window, has its
        // settings on the server only. Show them in the editor once, so Edit and
        // Resume work on the real values.
        async function adoptActiveConfig() {
            if (state.activeConfigLoaded || state.dirty) return;
            state.activeConfigLoaded = true;
            if (fingerprint(currentFields()) !== fingerprint(emptyDraft())) return;
            try {
                const res = await api('/api/desktop/looper/active');
                if (res && res.config && !state.disposed && !state.dirty) {
                    state.selectedPresetId = null;
                    loadIntoForm(draftFromConfig(res.config));
                    renderPresets();
                }
            } catch (e) { /* the editor simply stays empty */ }
        }

        function updateRun(data) {
            const previous = state.status;
            state.status = data;
            const running = !!data.running;
            const paused = !!data.paused || data.status === 'paused';
            const active = running || paused;
            const pausePending = running && !!data.pause_requested;
            root.classList.toggle('is-running', running);
            root.classList.toggle('is-active-run', active);
            root.classList.toggle('is-pause-pending', pausePending);
            if (active && !state.focus) setFocus(true);
            if (active) adoptActiveConfig();
            else state.activeConfigLoaded = false;

            $(`looper-start-${windowId}`).disabled = isReadonly || active;
            $(`looper-pause-${windowId}`).disabled = isReadonly || !running || pausePending;
            container.querySelector('.vd-looper-pause-label').textContent = pausePending ? t('desktop.looper_pause_pending') : t('desktop.looper_pause');
            $(`looper-stop-${windowId}`).disabled = isReadonly || !active;
            container.querySelector('.vd-looper-stop-label').textContent = paused && !running ? t('desktop.looper_discard_run') : t('desktop.looper_stop');
            const resume = $(`looper-resume-${windowId}`);
            resume.hidden = !(paused && !running);
            resume.disabled = isReadonly || !paused || running;
            FIELD_KEYS.forEach(key => {
                const el = $(`looper-${key}-${windowId}`);
                if (el) el.disabled = isReadonly || running;
            });

            if (state.runView) state.runView.update(data);
            renderBrief();
            if (previous.running && !running && !paused) {
                state.historyDetail = null;
                loadHistory(true);
            }
        }

        function connectStatus() {
            if (state.sse) { state.sse.close(); state.sse = null; }
            if (state.disposed) return;
            const evtSource = new EventSource('/api/desktop/looper/status');
            state.sse = evtSource;
            evtSource.onmessage = (event) => {
                try {
                    const msg = JSON.parse(event.data);
                    updateRun(monitor && state.logCache ? monitor.mergeStatus(state.logCache, msg) : msg);
                } catch (e) { /* ignore */ }
            };
            evtSource.onerror = () => {
                evtSource.close(); state.sse = null;
                if ((state.status.running || state.status.paused) && !state.disposed) {
                    setTimeout(() => connectStatus(), 2000);
                }
            };
        }

        async function startOrResume(path) {
            if (isReadonly) return;
            let body = readForm();
            const resuming = path.indexOf('/resume') >= 0;
            // A restored run can resume with an empty editor: the server then
            // continues with the settings stored in its checkpoint.
            if (resuming && !String(body.goal).trim() && !String(body.work).trim() && !String(body.evaluate).trim()) {
                body = {};
            } else {
                const errMsg = validateForm(body);
                if (errMsg) {
                    toast(errMsg);
                    return;
                }
            }
            $(`looper-start-${windowId}`).disabled = true;
            try {
                await api(path, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(body)
                });
                setFocus(true);
                connectStatus();
                setPane('run');
                setSide('run');
                if (state.runView) state.runView.followLatest();
            } catch (e) {
                updateRun(state.status);
                const code = errorCode(e);
                if (code === 'budget_exceeded') {
                    toast(t('desktop.looper_budget_exceeded'));
                } else if (code === 'already_running') {
                    toast(t('desktop.looper_already_running'));
                    connectStatus();
                } else {
                    toast((e && e.message) || (resuming ? t('desktop.looper_resume_error') : t('desktop.looper_start_error')));
                }
            }
        }

        // wiring
        if (monitor) {
            state.runView = monitor.createRunView(runPane, Object.assign(helpers(), {
                scroller: runPane,
                fallbackTarget: function () { return parseInt($(`looper-score-${windowId}`).value, 10) || 85; },
                onFollowChange: function (following) { side.classList.toggle('is-away', !following); }
            }));
        }

        const editorEl = container.querySelector('.vd-looper-editor');
        editorEl.addEventListener('input', updateDirty);
        editorEl.addEventListener('change', updateDirty);
        $(`looper-score-${windowId}`).addEventListener('input', ev => {
            $(`looper-score-out-${windowId}`).value = ev.target.value;
        });
        $(`looper-provider-${windowId}`).addEventListener('change', () => {
            refreshModels('');
            updateDirty();
        });
        $(`looper-new-${windowId}`).addEventListener('click', async () => {
            if (activeRun() || !await confirmDiscard()) return;
            state.selectedPresetId = null;
            loadIntoForm(emptyDraft());
            setFocus(false);
            renderPresets();
        });
        $(`looper-edit-${windowId}`).addEventListener('click', () => {
            if (!state.status.running) setFocus(false);
        });

        root.addEventListener('keydown', ev => {
            if (ev.key !== 'Enter' || !(ev.ctrlKey || ev.metaKey)) return;
            const resume = $(`looper-resume-${windowId}`);
            const start = $(`looper-start-${windowId}`);
            if (!resume.hidden && !resume.disabled) resume.click();
            else if (!start.disabled) start.click();
            ev.preventDefault();
        });

        $(`looper-save-${windowId}`).addEventListener('click', async () => {
            if (isReadonly) return;
            const body = readForm();
            const errMsg = validateForm(body);
            if (errMsg) {
                toast(errMsg);
                return;
            }
            const selected = selectedPreset();
            const canUpdate = selected && !selected.is_builtin;
            if (canUpdate) {
                const ok = await askConfirm(t('desktop.looper_save'), t('desktop.looper_save_update_confirm', { name: selected.name }));
                if (ok) {
                    try {
                        await api('/api/desktop/looper/presets/' + selected.id, {
                            method: 'PUT',
                            headers: { 'Content-Type': 'application/json' },
                            body: JSON.stringify(Object.assign({}, body, { name: body.name || selected.name }))
                        });
                        await loadPresets();
                        state.baseline = fingerprint(currentFields());
                        updateDirty();
                        if (notify) notify({ title: t('desktop.looper_title'), message: t('desktop.looper_saved') });
                    } catch (e) {
                        toast(t('desktop.looper_save_error'));
                    }
                    return;
                }
            }
            const name = await askPrompt(t('desktop.looper_save_prompt'), body.name || '');
            if (!name) return;
            try {
                const res = await api('/api/desktop/looper/presets', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(Object.assign({}, body, { name: name }))
                });
                await loadPresets();
                if (res && res.id) state.selectedPresetId = String(res.id);
                $(`looper-name-${windowId}`).value = name;
                state.baseline = fingerprint(currentFields());
                updateDirty();
                renderPresets();
                if (notify) notify({ title: t('desktop.looper_title'), message: t('desktop.looper_saved') });
            } catch (e) {
                toast(t('desktop.looper_save_error'));
            }
        });

        $(`looper-dup-${windowId}`).addEventListener('click', async () => {
            if (isReadonly) return;
            const body = readForm();
            const name = await askPrompt(t('desktop.looper_save_prompt'), (body.name || t('desktop.looper_untitled')) + ' ' + t('desktop.looper_copy_suffix'));
            if (!name) return;
            try {
                const res = await api('/api/desktop/looper/presets', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(Object.assign({}, body, { name: name }))
                });
                await loadPresets();
                if (res && res.id) {
                    state.selectedPresetId = String(res.id);
                    $(`looper-name-${windowId}`).value = name;
                    state.baseline = fingerprint(currentFields());
                    updateDirty();
                    renderPresets();
                }
            } catch (e) {
                toast(t('desktop.looper_save_error'));
            }
        });

        $(`looper-delete-${windowId}`).addEventListener('click', async () => {
            if (isReadonly) return;
            const selected = selectedPreset();
            if (!selected || selected.is_builtin) {
                toast(t('desktop.looper_delete_error'));
                return;
            }
            if (!await askConfirm(t('desktop.looper_delete'), t('desktop.looper_delete_confirm'))) return;
            try {
                await api('/api/desktop/looper/presets/' + selected.id, { method: 'DELETE' });
                state.selectedPresetId = null;
                loadIntoForm(emptyDraft());
                await loadPresets();
                if (notify) notify({ title: t('desktop.looper_title'), message: t('desktop.looper_deleted') });
            } catch (e) {
                toast(t('desktop.looper_delete_error'));
            }
        });

        $(`looper-start-${windowId}`).addEventListener('click', () => startOrResume('/api/desktop/looper/run'));
        $(`looper-resume-${windowId}`).addEventListener('click', () => startOrResume('/api/desktop/looper/resume'));
        $(`looper-pause-${windowId}`).addEventListener('click', async () => {
            $(`looper-pause-${windowId}`).disabled = true;
            try {
                await api('/api/desktop/looper/pause', { method: 'POST' });
            } catch (e) {
                updateRun(state.status);
                toast(t('desktop.looper_pause_error'));
            }
        });
        $(`looper-stop-${windowId}`).addEventListener('click', async () => {
            try {
                await api('/api/desktop/looper/stop', { method: 'POST' });
                if (!state.status.running) connectStatus();
            } catch (e) {
                toast(t('desktop.looper_stop_error'));
            }
        });
        $(`looper-jump-${windowId}`).addEventListener('click', () => {
            if (state.runView) state.runView.followLatest();
        });

        updateRun(state.status);
        (async function init() {
            await loadProviders();
            await loadPresets();
            if (state.disposed) return;
            restoreDraft();
            renderBrief();
        })();
        loadHistory();
        connectStatus();
    }

    function tabBtn(windowId, pane, label) {
        return '<button type="button" class="vd-looper-tab' + (pane === 'setup' ? ' is-active' : '') + '" data-pane="' + pane + '" id="looper-tab-' + pane + '-' + windowId + '">' + label + '</button>';
    }

    // Read-only view of an instruction while the run owns the window.
    function briefDetails(esc, t, windowId, key) {
        return '<details class="vd-looper-brief-more" id="looper-brief-' + key + '-' + windowId + '" hidden>' +
            '<summary>' + esc(t('desktop.looper_' + key)) + '</summary><p></p></details>';
    }

    function fieldCard(esc, t, windowId, num, key, required) {
        return '<div class="vd-looper-card">' +
            '<div class="vd-looper-card-head"><span class="vd-looper-card-num">' + num + '</span>' +
            '<label for="looper-' + key + '-' + windowId + '">' + esc(t('desktop.looper_' + key)) + '</label></div>' +
            '<p class="vd-looper-help">' + esc(t('desktop.looper_' + key + '_help')) + '</p>' +
            '<textarea id="looper-' + key + '-' + windowId + '" rows="' + (key === 'goal' ? '4' : '3') + '" placeholder="' + esc(t('desktop.looper_' + key + '_placeholder')) + '"' + (required ? ' required' : '') + '></textarea>' +
            '</div>';
    }

    function dispose(windowId) {
        const state = instances.get(windowId);
        if (!state) return;
        state.disposed = true;
        clearTimeout(state.draftTimer);
        if (state.persistDraft) {
            // Closing the window must not lose unsaved work.
            try { state.persistDraft(); } catch (e) { /* the form may already be detached */ }
        }
        if (state.sse) { state.sse.close(); state.sse = null; }
        if (state.runView) state.runView.destroy();
        if (state.historyView) state.historyView.destroy();
        if (state.resizeObserver) state.resizeObserver.disconnect();
        instances.delete(windowId);
    }

    window.LooperApp = { render: render, dispose: dispose };
})();
