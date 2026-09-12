(function () {
    'use strict';

    const instances = new Map();
    const COMPACT_WIDTH = 820;

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
            trash: '<polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>'
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

        const state = {
            presets: [],
            providers: [],
            selectedPresetId: null,
            draft: emptyDraft(t),
            status: { status: 'idle', current_step: 'idle', round: 0, max_rounds: 10, logs: [] },
            sse: null,
            disposed: false,
            compact: false,
            pane: 'setup',
            history: [],
            historyDetail: null,
            logExpandState: new Map(),
            autoScroll: true
        };
        instances.set(windowId, state);

        container.innerHTML =
            '<div class="vd-looper' + (isReadonly ? ' vd-looper--readonly' : '') + '" data-pane="setup">' +
            '<div class="vd-looper-tabs" role="tablist">' +
            tabBtn(windowId, 'setup', t('desktop.looper_tab_setup')) +
            tabBtn(windowId, 'run', t('desktop.looper_tab_run')) +
            tabBtn(windowId, 'history', t('desktop.looper_tab_history')) +
            '</div>' +
            '<aside class="vd-looper-list" data-pane="setup">' +
            '<div class="vd-looper-list-scroll" id="looper-presets-' + windowId + '"></div>' +
            '<button type="button" class="vd-looper-new" id="looper-new-' + windowId + '" ' + (isReadonly ? 'disabled' : '') + '>' +
            icon('plus') + '<span>' + esc(t('desktop.looper_new')) + '</span></button>' +
            '</aside>' +
            '<section class="vd-looper-editor" data-pane="setup">' +
            '<div class="vd-looper-editor-toolbar">' +
            '<input type="text" inputmode="text" enterkeyhint="next" id="looper-name-' + windowId + '" class="vd-looper-name" placeholder="' + esc(t('desktop.looper_name')) + '" ' + (isReadonly ? 'disabled' : '') + '>' +
            '<div class="vd-looper-editor-actions">' +
            '<button type="button" class="vd-looper-icon-btn" id="looper-save-' + windowId + '" title="' + esc(t('desktop.looper_save')) + '" ' + (isReadonly ? 'disabled' : '') + '>' + icon('save') + '</button>' +
            '<button type="button" class="vd-looper-icon-btn" id="looper-dup-' + windowId + '" title="' + esc(t('desktop.looper_duplicate')) + '" ' + (isReadonly ? 'disabled' : '') + '>' + icon('copy') + '</button>' +
            '<button type="button" class="vd-looper-icon-btn vd-looper-btn-delete" id="looper-delete-' + windowId + '" title="' + esc(t('desktop.looper_delete')) + '" ' + (isReadonly ? 'disabled' : '') + '>' + icon('trash') + '</button>' +
            '</div></div>' +
            fieldCard(esc, t, windowId, '1', 'goal', true) +
            fieldCard(esc, t, windowId, '2', 'work', true) +
            fieldCard(esc, t, windowId, '3', 'evaluate', true) +
            '<details class="vd-looper-disclosure" id="looper-finish-wrap-' + windowId + '">' +
            '<summary>' + esc(t('desktop.looper_finish_toggle')) + '</summary>' +
            fieldCard(esc, t, windowId, '4', 'finish', false) +
            '</details>' +
            '<div class="vd-looper-settings">' +
            '<label class="vd-looper-setting">' + esc(t('desktop.looper_max_rounds')) +
            '<input type="number" inputmode="numeric" enterkeyhint="done" id="looper-max-' + windowId + '" min="1" max="50" value="10" ' + (isReadonly ? 'disabled' : '') + '></label>' +
            '<label class="vd-looper-setting vd-looper-setting-range">' + esc(t('desktop.looper_target_score')) +
            '<span class="vd-looper-range-row"><input type="range" id="looper-score-' + windowId + '" min="50" max="100" value="85" ' + (isReadonly ? 'disabled' : '') + '>' +
            '<output id="looper-score-out-' + windowId + '">85</output></span></label>' +
            '<label class="vd-looper-setting">' + esc(t('desktop.looper_provider')) +
            '<select id="looper-provider-' + windowId + '" ' + (isReadonly ? 'disabled' : '') + '></select></label>' +
            '<label class="vd-looper-setting">' + esc(t('desktop.looper_model')) +
            '<select id="looper-model-' + windowId + '" ' + (isReadonly ? 'disabled' : '') + '></select></label>' +
            '</div>' +
            '<details class="vd-looper-disclosure">' +
            '<summary>' + esc(t('desktop.looper_more_options')) + '</summary>' +
            '<label class="vd-looper-setting">' + esc(t('desktop.looper_stall_rounds')) +
            '<input type="number" inputmode="numeric" enterkeyhint="done" id="looper-stall-' + windowId + '" min="0" max="10" value="3" title="' + esc(t('desktop.looper_stall_rounds_help')) + '" ' + (isReadonly ? 'disabled' : '') + '>' +
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
            '<button type="button" class="vd-looper-start" id="looper-start-' + windowId + '" ' + (isReadonly ? 'disabled' : '') + '>' + icon('play') + '<span>' + esc(t('desktop.looper_start')) + '</span></button>' +
            '<button type="button" class="vd-looper-pause" id="looper-pause-' + windowId + '" disabled>' + icon('pause') + '<span>' + esc(t('desktop.looper_pause')) + '</span></button>' +
            '<button type="button" class="vd-looper-resume" id="looper-resume-' + windowId + '" hidden>' + icon('play') + '<span>' + esc(t('desktop.looper_resume')) + '</span></button>' +
            '<button type="button" class="vd-looper-stop" id="looper-stop-' + windowId + '" disabled>' + icon('stop') + '<span>' + esc(t('desktop.looper_stop')) + '</span></button>' +
            '</div></div>';

        const $ = id => container.querySelector('#' + id);
        const root = container.querySelector('.vd-looper');

        function helpers() {
            return {
                esc: esc,
                t: t,
                formatCost: formatCost,
                formatDuration: function (ms) { return formatDuration(ms, t); },
                expandState: state.logExpandState
            };
        }

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

        function setSide(side) {
            root.querySelectorAll('.vd-looper-side-tab').forEach(btn => {
                btn.classList.toggle('is-active', btn.dataset.side === side);
            });
            root.querySelectorAll('.vd-looper-side-pane').forEach(el => {
                el.classList.toggle('is-active', el.dataset.sidePane === side);
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

        function selectedPreset() {
            return state.presets.find(p => String(p.id) === String(state.selectedPresetId)) || null;
        }

        function fillForm(p) {
            state.draft = {
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
            writeForm();
        }

        function writeForm() {
            const d = state.draft;
            $(`looper-name-${windowId}`).value = d.name || '';
            $(`looper-goal-${windowId}`).value = d.goal || '';
            $(`looper-work-${windowId}`).value = d.work || '';
            $(`looper-evaluate-${windowId}`).value = d.evaluate || '';
            $(`looper-finish-${windowId}`).value = d.finish || '';
            $(`looper-max-${windowId}`).value = d.max_rounds || 10;
            $(`looper-score-${windowId}`).value = d.target_score || 85;
            $(`looper-score-out-${windowId}`).value = d.target_score || 85;
            $(`looper-stall-${windowId}`).value = d.stall_rounds;
            $(`looper-provider-${windowId}`).value = d.provider_id || '';
            refreshModels();
            $(`looper-model-${windowId}`).value = d.model || '';
            const finishWrap = $(`looper-finish-wrap-${windowId}`);
            if (finishWrap) finishWrap.open = !!String(d.finish || '').trim();
        }

        function readForm() {
            state.draft = {
                name: $(`looper-name-${windowId}`).value.trim(),
                goal: $(`looper-goal-${windowId}`).value,
                work: $(`looper-work-${windowId}`).value,
                evaluate: $(`looper-evaluate-${windowId}`).value,
                finish: $(`looper-finish-${windowId}`).value,
                max_rounds: parseInt($(`looper-max-${windowId}`).value, 10) || 10,
                target_score: parseInt($(`looper-score-${windowId}`).value, 10) || 85,
                stall_rounds: parseInt($(`looper-stall-${windowId}`).value, 10),
                provider_id: $(`looper-provider-${windowId}`).value,
                model: $(`looper-model-${windowId}`).value
            };
            if (!Number.isFinite(state.draft.stall_rounds) || state.draft.stall_rounds < 0) {
                state.draft.stall_rounds = 3;
            }
            return Object.assign({ preset_name: state.draft.name }, state.draft);
        }

        function validateForm(body) {
            for (const field of ['goal', 'work', 'evaluate']) {
                if (!String(body[field] || '').trim()) {
                    return t('desktop.looper_field_required', { field: t('desktop.looper_' + field) });
                }
            }
            return '';
        }

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
                btn.addEventListener('click', () => {
                    state.selectedPresetId = btn.dataset.presetId;
                    const p = selectedPreset();
                    if (p) fillForm(p);
                    renderPresets();
                });
            });
        }

        function presetButton(p) {
            const active = String(p.id) === String(state.selectedPresetId);
            return '<button type="button" class="vd-looper-preset' + (active ? ' is-active' : '') + (p.is_builtin ? ' is-example' : '') + '" data-preset-id="' + esc(String(p.id)) + '">' +
                '<span class="vd-looper-preset-name">' + esc(presetTitle(p, t)) + '</span>' +
                '<span class="vd-looper-preset-desc">' + esc(presetDesc(p, t)) + '</span></button>';
        }

        function refreshModels() {
            const providerId = $(`looper-provider-${windowId}`).value;
            const select = $(`looper-model-${windowId}`);
            const prev = select.value;
            const models = [];
            const provider = state.providers.find(p => p.id === providerId);
            if (provider && provider.model) models.push(provider.model);
            (provider && provider.models ? provider.models : []).forEach(m => {
                if (m && models.indexOf(m) < 0) models.push(m);
            });
            select.innerHTML = '<option value="">' + esc(t('desktop.looper_model_default')) + '</option>';
            models.forEach(model => {
                const opt = document.createElement('option');
                opt.value = model;
                opt.textContent = model;
                select.appendChild(opt);
            });
            select.value = prev;
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
                if (notify) notify({ title: t('desktop.notification'), message: t('desktop.looper_error') });
            }
        }

        async function loadHistory() {
            if (state.historyDetail && monitor) {
                monitor.renderHistoryDetail($(`looper-history-${windowId}`), state.historyDetail, helpers());
                bindHistory();
                return;
            }
            try {
                const res = await api('/api/desktop/looper/runs');
                state.history = (res && res.runs) || [];
                if (monitor) monitor.renderHistoryList($(`looper-history-${windowId}`), state.history, helpers());
                bindHistory();
            } catch (e) {
                $(`looper-history-${windowId}`).innerHTML = '<div class="vd-looper-log-empty">' + esc(t('desktop.looper_history_load_error')) + '</div>';
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
                        if (notify) notify({ title: t('desktop.notification'), message: t('desktop.looper_history_load_error') });
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
                        loadHistory();
                    } catch (e) {
                        if (notify) notify({ title: t('desktop.notification'), message: t('desktop.looper_delete_error') });
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
                        loadHistory();
                    } catch (e) {
                        if (notify) notify({ title: t('desktop.notification'), message: t('desktop.looper_delete_error') });
                    }
                });
            }
            const back = host.querySelector('.vd-looper-history-back');
            if (back) {
                back.addEventListener('click', () => {
                    state.historyDetail = null;
                    loadHistory();
                });
            }
        }

        function updateRun(data) {
            state.status = data;
            const running = !!data.running;
            const paused = !!data.paused || data.status === 'paused';
            root.classList.toggle('is-running', running);
            $(`looper-start-${windowId}`).disabled = isReadonly || running || paused;
            $(`looper-pause-${windowId}`).disabled = !running || paused;
            $(`looper-stop-${windowId}`).disabled = !running;
            const resume = $(`looper-resume-${windowId}`);
            resume.hidden = !(paused && !running);
            resume.disabled = isReadonly || !paused || running;
            ['goal', 'work', 'evaluate', 'finish', 'name', 'max', 'score', 'stall', 'provider', 'model'].forEach(key => {
                const el = $(`looper-${key}-${windowId}`);
                if (el) el.disabled = isReadonly || running;
            });
            formatCost(data.estimated_cost_usd, t);
            t('desktop.looper_tokens', { count: (data.input_tokens || 0) + (data.output_tokens || 0) });
            const side = container.querySelector('.vd-looper-side');
            if (side) side.classList.toggle('has-logs', !!(data.logs && data.logs.length));
            if (monitor) {
                monitor.renderRun($(`looper-run-${windowId}`), data, helpers());
                wireLogToggles();
            }
            if (state.autoScroll) {
                const timeline = container.querySelector('.vd-looper-timeline');
                if (timeline) timeline.scrollTop = timeline.scrollHeight;
            }
        }

        function wireLogToggles() {
            container.querySelectorAll('.vd-looper-log-header').forEach(header => {
                if (header.dataset.wired === '1') return;
                header.dataset.wired = '1';
                header.addEventListener('click', () => {
                    const entry = header.closest('.vd-looper-log');
                    if (!entry) return;
                    entry.classList.toggle('vd-looper-log--collapsed');
                    const expanded = !entry.classList.contains('vd-looper-log--collapsed');
                    const key = entry.getAttribute('data-log-key');
                    if (key) state.logExpandState.set(key, expanded);
                });
            });
        }

        function connectStatus() {
            if (state.sse) { state.sse.close(); state.sse = null; }
            if (state.disposed) return;
            const evtSource = new EventSource('/api/desktop/looper/status');
            state.sse = evtSource;
            evtSource.onmessage = (event) => {
                try { updateRun(JSON.parse(event.data)); } catch (e) { /* ignore */ }
            };
            evtSource.onerror = () => {
                evtSource.close(); state.sse = null;
                if ((state.status.running || state.status.paused) && !state.disposed) {
                    setTimeout(() => connectStatus(), 2000);
                }
            };
        }

        $(`looper-score-${windowId}`).addEventListener('input', ev => {
            $(`looper-score-out-${windowId}`).value = ev.target.value;
        });
        $(`looper-provider-${windowId}`).addEventListener('change', refreshModels);
        $(`looper-new-${windowId}`).addEventListener('click', () => {
            state.selectedPresetId = null;
            state.draft = emptyDraft();
            writeForm();
            renderPresets();
        });

        $(`looper-save-${windowId}`).addEventListener('click', async () => {
            if (isReadonly) return;
            const body = readForm();
            const errMsg = validateForm(body);
            if (errMsg) {
                if (notify) notify({ title: t('desktop.notification'), message: errMsg });
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
                        if (notify) notify({ title: t('desktop.looper_title'), message: t('desktop.looper_saved') });
                    } catch (e) {
                        if (notify) notify({ title: t('desktop.notification'), message: t('desktop.looper_save_error') });
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
                renderPresets();
                if (notify) notify({ title: t('desktop.looper_title'), message: t('desktop.looper_saved') });
            } catch (e) {
                if (notify) notify({ title: t('desktop.notification'), message: t('desktop.looper_save_error') });
            }
        });

        $(`looper-dup-${windowId}`).addEventListener('click', async () => {
            if (isReadonly) return;
            const body = readForm();
            const name = await askPrompt(t('desktop.looper_save_prompt'), (body.name || t('desktop.looper_untitled')) + ' copy');
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
                    renderPresets();
                }
            } catch (e) {
                if (notify) notify({ title: t('desktop.notification'), message: t('desktop.looper_save_error') });
            }
        });

        $(`looper-delete-${windowId}`).addEventListener('click', async () => {
            if (isReadonly) return;
            const selected = selectedPreset();
            if (!selected || selected.is_builtin) {
                if (notify) notify({ title: t('desktop.notification'), message: t('desktop.looper_delete_error') });
                return;
            }
            if (!await askConfirm(t('desktop.looper_delete'), t('desktop.looper_delete_confirm'))) return;
            try {
                await api('/api/desktop/looper/presets/' + selected.id, { method: 'DELETE' });
                state.selectedPresetId = null;
                state.draft = emptyDraft();
                writeForm();
                await loadPresets();
                if (notify) notify({ title: t('desktop.looper_title'), message: t('desktop.looper_deleted') });
            } catch (e) {
                if (notify) notify({ title: t('desktop.notification'), message: t('desktop.looper_delete_error') });
            }
        });

        async function startOrResume(path) {
            if (isReadonly) return;
            const body = readForm();
            const errMsg = validateForm(body);
            if (errMsg) {
                if (notify) notify({ title: t('desktop.notification'), message: errMsg });
                return;
            }
            try {
                await api(path, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(body)
                });
                state.logExpandState.clear();
                connectStatus();
                setPane('run');
                setSide('run');
            } catch (e) {
                if (e && e.status === 409 && path.indexOf('/run') >= 0) {
                    if (notify) notify({ title: t('desktop.notification'), message: t('desktop.looper_already_running') });
                    connectStatus();
                } else if (notify) {
                    const fallback = path.indexOf('/resume') >= 0 ? t('desktop.looper_resume_error') : t('desktop.looper_start_error');
                    notify({ title: t('desktop.notification'), message: (e && e.message) || fallback });
                }
            }
        }

        $(`looper-start-${windowId}`).addEventListener('click', () => startOrResume('/api/desktop/looper/run'));
        $(`looper-resume-${windowId}`).addEventListener('click', () => startOrResume('/api/desktop/looper/resume'));
        $(`looper-pause-${windowId}`).addEventListener('click', async () => {
            try {
                await api('/api/desktop/looper/pause', { method: 'POST' });
                if (notify) notify({ title: t('desktop.looper_title'), message: t('desktop.looper_pause_requested') });
            } catch (e) {
                if (notify) notify({ title: t('desktop.notification'), message: t('desktop.looper_pause_error') });
            }
        });
        $(`looper-stop-${windowId}`).addEventListener('click', async () => {
            try {
                await api('/api/desktop/looper/stop', { method: 'POST' });
            } catch (e) {
                if (notify) notify({ title: t('desktop.notification'), message: t('desktop.looper_stop_error') });
            }
        });
        $(`looper-jump-${windowId}`).addEventListener('click', () => {
            const timeline = container.querySelector('.vd-looper-timeline');
            if (timeline) timeline.scrollTop = timeline.scrollHeight;
            state.autoScroll = true;
        });

        updateRun(state.status);
        loadProviders();
        loadPresets();
        loadHistory();
        connectStatus();
    }

    function tabBtn(windowId, pane, label) {
        return '<button type="button" class="vd-looper-tab' + (pane === 'setup' ? ' is-active' : '') + '" data-pane="' + pane + '" id="looper-tab-' + pane + '-' + windowId + '">' + label + '</button>';
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
        if (state.sse) { state.sse.close(); state.sse = null; }
        if (state.resizeObserver) state.resizeObserver.disconnect();
        instances.delete(windowId);
    }

    window.LooperApp = { render: render, dispose: dispose };
})();
