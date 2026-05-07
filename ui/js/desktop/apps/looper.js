(function () {
    'use strict';

    const instances = new Map();

    const STEP_META = [
        { key: 'prepare',  labelKey: 'desktop.looper_prepare',  color: '#3b82f6', icon: '⚡', descKey: 'desktop.looper_prepare_placeholder' },
        { key: 'plan',     labelKey: 'desktop.looper_plan',     color: '#f59e0b', icon: '🎯', descKey: 'desktop.looper_plan_placeholder' },
        { key: 'action',   labelKey: 'desktop.looper_action',   color: '#10b981', icon: '▶',  descKey: 'desktop.looper_action_placeholder' },
        { key: 'test',     labelKey: 'desktop.looper_test',     color: '#8b5cf6', icon: '🔍', descKey: 'desktop.looper_test_placeholder' },
        { key: 'exit',     labelKey: 'desktop.looper_exit',     color: '#ef4444', icon: '◆',  descKey: 'desktop.looper_exit_placeholder' },
        { key: 'finish',   labelKey: 'desktop.looper_finish',   color: '#06b6d4', icon: '✓',  descKey: 'desktop.looper_finish_placeholder' }
    ];

    function render(container, windowId, context) {
        dispose(windowId);

        const { esc, t, api, notify, readonly } = context;

        const state = {
            presets: [],
            providers: [],
            running: false,
            status: { current_step: 'idle', iteration: 0, max_iterations: 20, logs: [], last_result: '' },
            sse: null,
            selectedPresetId: null,
            activeStep: 'prepare',
            stepValues: { prepare: '', plan: '', action: '', test: '', exit: '', finish: '' },
            startTime: null,
            logCount: 0
        };
        instances.set(windowId, state);

        container.innerHTML = `
            <div class="vd-looper">
                <div class="vd-looper-header">
                    <div class="vd-looper-header-group">
                        <div class="vd-looper-field vd-looper-field-preset">
                            <label>${esc(t('desktop.looper_preset'))}</label>
                            <div class="vd-looper-input-row">
                                <select id="looper-preset-${windowId}">
                                    <option value="">${esc(t('desktop.looper_select_preset'))}</option>
                                </select>
                                <button type="button" class="vd-looper-icon-btn vd-looper-btn-save" id="looper-save-${windowId}" title="${esc(t('desktop.looper_save'))}">
                                    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"/><polyline points="17 21 17 13 7 13 7 21"/><polyline points="7 3 7 8 15 8"/></svg>
                                </button>
                                <button type="button" class="vd-looper-icon-btn vd-looper-btn-delete" id="looper-delete-${windowId}" title="${esc(t('desktop.looper_delete'))}">
                                    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
                                </button>
                            </div>
                        </div>
                    </div>
                    <div class="vd-looper-header-group vd-looper-header-group-config">
                        <div class="vd-looper-field">
                            <label>${esc(t('desktop.looper_provider'))}</label>
                            <select id="looper-provider-${windowId}"></select>
                        </div>
                        <div class="vd-looper-field">
                            <label>${esc(t('desktop.looper_model'))}</label>
                            <input type="text" id="looper-model-${windowId}" placeholder="${esc(t('desktop.looper_model_placeholder'))}">
                        </div>
                        <div class="vd-looper-field vd-looper-field-xs">
                            <label>${esc(t('desktop.looper_max_iter'))}</label>
                            <input type="number" id="looper-max-iter-${windowId}" value="20" min="1" max="100">
                        </div>
                        <div class="vd-looper-field vd-looper-field-sm">
                            <label>${esc(t('desktop.looper_context_mode'))}</label>
                            <select id="looper-context-mode-${windowId}">
                                <option value="every_iteration">${esc(t('desktop.looper_context_every_iteration'))}</option>
                                <option value="never">${esc(t('desktop.looper_context_never'))}</option>
                                <option value="every_step">${esc(t('desktop.looper_context_every_step'))}</option>
                            </select>
                        </div>
                    </div>
                </div>

                <div class="vd-looper-workspace">
                    <div class="vd-looper-sidebar">
                        ${STEP_META.map((step, idx) => `
                            <button type="button" class="vd-looper-step-btn${step.key === 'finish' ? ' vd-looper-step-btn--optional' : ''}" data-step="${step.key}" id="looper-step-btn-${step.key}-${windowId}" style="--step-color: ${step.color}">
                                <span class="vd-looper-step-btn-icon">${step.icon}</span>
                                <span class="vd-looper-step-btn-num">${idx + 1}</span>
                                <span class="vd-looper-step-btn-name">${esc(t(step.labelKey))}</span>
                            </button>
                        `).join('')}
                        <div class="vd-looper-sidebar-actions">
                            <button type="button" class="vd-looper-start" id="looper-start-${windowId}">
                                <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor"><polygon points="5 3 19 12 5 21 5 3"/></svg>
                                <span>${esc(t('desktop.looper_start'))}</span>
                            </button>
                            <button type="button" class="vd-looper-stop" id="looper-stop-${windowId}" disabled>
                                <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor"><rect x="6" y="6" width="12" height="12" rx="2"/></svg>
                                <span>${esc(t('desktop.looper_stop'))}</span>
                            </button>
                        </div>
                    </div>
                    <div class="vd-looper-editor">
                        <div class="vd-looper-editor-header" id="looper-editor-header-${windowId}"></div>
                        <textarea id="looper-editor-textarea-${windowId}" rows="12" placeholder=""></textarea>
                    </div>
                </div>

                <div class="vd-looper-monitor" id="looper-monitor-${windowId}">
                    <div class="vd-looper-monitor-header">
                        <div class="vd-looper-monitor-title">
                            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>
                            <span>${esc(t('desktop.looper_status_idle'))}</span>
                        </div>
                        <div class="vd-looper-monitor-meta" id="looper-meta-${windowId}"></div>
                    </div>
                    <div class="vd-looper-progress-track">
                        <div class="vd-looper-progress-bar" id="looper-progress-${windowId}"></div>
                        <div class="vd-looper-progress-glow" id="looper-progress-glow-${windowId}"></div>
                    </div>
                    <div class="vd-looper-logs-wrap">
                        <div class="vd-looper-logs-header">
                            <span>${esc(t('desktop.looper_logs_title'))}</span>
                            <button type="button" class="vd-looper-logs-clear" id="looper-clear-${windowId}">
                                <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
                                ${esc(t('desktop.looper_clear'))}
                            </button>
                        </div>
                        <div class="vd-looper-logs" id="looper-logs-${windowId}">
                            <div class="vd-looper-log-empty">${esc(t('desktop.looper_no_logs'))}</div>
                        </div>
                    </div>
                </div>
            </div>
        `;

        const $ = id => container.querySelector('#' + id);
        const $$ = sel => container.querySelectorAll(sel);

        // ── Step switching ──
        function saveCurrentStepValue() {
            const textarea = $(`looper-editor-textarea-${windowId}`);
            if (textarea && state.activeStep) {
                state.stepValues[state.activeStep] = textarea.value;
            }
        }

        function setActiveStep(key) {
            saveCurrentStepValue();
            state.activeStep = key;
            const meta = STEP_META.find(s => s.key === key);

            $$(`.vd-looper-step-btn`).forEach(btn => {
                btn.classList.toggle('vd-looper-step-btn--active', btn.dataset.step === key);
            });

            const header = $(`looper-editor-header-${windowId}`);
            const textarea = $(`looper-editor-textarea-${windowId}`);
            const headerText = meta.key === 'finish'
                ? esc(t(meta.labelKey)) + ' (' + esc(t('desktop.optional')).toLowerCase() + ')'
                : esc(t(meta.labelKey));
            header.textContent = headerText;
            header.style.color = meta.color;
            textarea.placeholder = esc(t(meta.descKey));
            textarea.value = state.stepValues[key] || '';
            textarea.style.borderColor = meta.color + '40';
            textarea.focus();
        }

        $$(`.vd-looper-step-btn`).forEach(btn => {
            btn.addEventListener('click', () => {
                if (!state.running) setActiveStep(btn.dataset.step);
            });
        });

        setActiveStep('prepare');

        // ── Data Loading ──
        async function loadProviders() {
            const select = $(`looper-provider-${windowId}`);
            select.innerHTML = `<option value="">${esc(t('desktop.looper_default_provider'))}</option>`;
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
            } catch (e) { /* ignore */ }
        }
        loadProviders();

        async function loadPresets() {
            try {
                const res = await api('/api/desktop/looper/presets');
                if (res && res.presets) {
                    state.presets = res.presets;
                    const select = $(`looper-preset-${windowId}`);
                    select.innerHTML = `<option value="">${esc(t('desktop.looper_select_preset'))}</option>`;
                    const builtins = res.presets.filter(p => p.is_builtin);
                    const users = res.presets.filter(p => !p.is_builtin);
                    if (builtins.length) {
                        const grp = document.createElement('optgroup');
                        grp.label = esc(t('desktop.looper_examples'));
                        builtins.forEach(p => {
                            const opt = document.createElement('option');
                            opt.value = p.id;
                            opt.textContent = '★ ' + p.name;
                            opt.className = 'vd-looper-opt-builtin';
                            grp.appendChild(opt);
                        });
                        select.appendChild(grp);
                    }
                    if (users.length) {
                        const grp = document.createElement('optgroup');
                        grp.label = esc(t('desktop.looper_preset'));
                        users.forEach(p => {
                            const opt = document.createElement('option');
                            opt.value = p.id;
                            opt.textContent = p.name;
                            grp.appendChild(opt);
                        });
                        select.appendChild(grp);
                    }
                }
            } catch (e) { console.error('Failed to load presets', e); }
        }
        loadPresets();

        function fillForm(p) {
            state.stepValues = {
                prepare: p.prepare || '',
                plan: p.plan || '',
                action: p.action || '',
                test: p.test || '',
                exit: p.exit_cond || '',
                finish: p.finish || ''
            };
            setActiveStep(state.activeStep);
            $(`looper-provider-${windowId}`).value = p.provider_id || '';
            $(`looper-model-${windowId}`).value = p.model || '';
            $(`looper-max-iter-${windowId}`).value = p.max_iter || 20;
            $(`looper-context-mode-${windowId}`).value = p.context_mode || 'every_iteration';
        }

        function readForm() {
            saveCurrentStepValue();
            return {
                prepare: state.stepValues.prepare,
                plan: state.stepValues.plan,
                action: state.stepValues.action,
                test: state.stepValues.test,
                exit_cond: state.stepValues.exit,
                finish: state.stepValues.finish,
                provider_id: $(`looper-provider-${windowId}`).value,
                model: $(`looper-model-${windowId}`).value,
                max_iter: parseInt($(`looper-max-iter-${windowId}`).value, 10) || 20,
                context_mode: $(`looper-context-mode-${windowId}`).value || 'every_iteration'
            };
        }

        $(`looper-preset-${windowId}`).addEventListener('change', (e) => {
            const id = e.target.value;
            if (!id) return;
            const p = state.presets.find(x => String(x.id) === id);
            if (p) fillForm(p);
        });

        $(`looper-save-${windowId}`).addEventListener('click', async () => {
            const name = prompt(t('desktop.looper_save_prompt'));
            if (!name) return;
            const body = readForm();
            body.name = name;
            try {
                await api('/api/desktop/looper/presets', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(body)
                });
                await loadPresets();
                if (notify) notify({ title: t('desktop.looper_title'), message: t('desktop.looper_saved') });
            } catch (e) {
                if (notify) notify({ title: t('desktop.notification'), message: t('desktop.looper_save_error') });
            }
        });

        $(`looper-delete-${windowId}`).addEventListener('click', async () => {
            const select = $(`looper-preset-${windowId}`);
            const id = select.value;
            if (!id) return;
            const p = state.presets.find(x => String(x.id) === id);
            if (p && p.is_builtin) {
                if (notify) notify({ title: t('desktop.notification'), message: t('desktop.looper_delete_error') });
                return;
            }
            if (!confirm(t('desktop.looper_delete_confirm'))) return;
            try {
                await api('/api/desktop/looper/presets/' + id, { method: 'DELETE' });
                await loadPresets();
                select.value = '';
                if (notify) notify({ title: t('desktop.looper_title'), message: t('desktop.looper_deleted') });
            } catch (e) {
                if (notify) notify({ title: t('desktop.notification'), message: t('desktop.looper_delete_error') });
            }
        });

        $(`looper-clear-${windowId}`).addEventListener('click', () => {
            $(`looper-logs-${windowId}`).innerHTML = `<div class="vd-looper-log-empty">${esc(t('desktop.looper_no_logs'))}</div>`;
            state.logCount = 0;
        });

        // ── SSE Status ──
        function connectStatus() {
            if (state.sse) { state.sse.close(); state.sse = null; }
            const evtSource = new EventSource('/api/desktop/looper/status');
            state.sse = evtSource;
            evtSource.onmessage = (event) => {
                try { updateStatus(JSON.parse(event.data)); } catch (e) { }
            };
            evtSource.onerror = () => {
                evtSource.close(); state.sse = null;
                if (state.running) setTimeout(() => connectStatus(), 2000);
            };
        }

        function formatDuration(ms) {
            if (!ms || ms < 0) return '0ms';
            if (ms < 1000) return ms + 'ms';
            return (ms / 1000).toFixed(1) + 's';
        }

        function highlightCode(text) {
            if (!text) return '';
            return esc(text)
                .replace(/```([\s\S]*?)```/g, '<pre class="vd-looper-code-block"><code>$1</code></pre>')
                .replace(/`([^`]+)`/g, '<code class="vd-looper-code-inline">$1</code>');
        }

        function updateStatus(data) {
            state.status = data;
            const monitor = $(`looper-monitor-${windowId}`);
            const titleEl = monitor.querySelector('.vd-looper-monitor-title span');
            const metaEl = $(`looper-meta-${windowId}`);
            const progressEl = $(`looper-progress-${windowId}`);
            const glowEl = $(`looper-progress-glow-${windowId}`);
            const logsEl = $(`looper-logs-${windowId}`);
            const startBtn = $(`looper-start-${windowId}`);
            const stopBtn = $(`looper-stop-${windowId}`);

            state.running = data.running;
            startBtn.disabled = data.running;
            stopBtn.disabled = !data.running;
            monitor.classList.toggle('vd-looper-monitor--running', data.running);
            monitor.classList.toggle('vd-looper-monitor--error', !!data.error);

            if (data.error) {
                titleEl.textContent = t('desktop.looper_error') + ': ' + data.error;
            } else if (data.running) {
                const stepLabel = t('desktop.looper_step_' + data.current_step) || data.current_step;
                if (data.current_step === 'prepare' || data.current_step === 'finish') {
                    titleEl.textContent = stepLabel;
                } else {
                    titleEl.textContent = t('desktop.looper_iteration')
                        .replace('{{n}}', data.iteration)
                        .replace('{{max}}', data.max_iterations) + ' — ' + stepLabel;
                }
            } else {
                titleEl.textContent = data.current_step === 'idle' ? t('desktop.looper_status_idle') : (t('desktop.looper_step_' + data.current_step) || data.current_step);
            }

            const pct = data.max_iterations > 0 ? (data.iteration / data.max_iterations) * 100 : 0;
            progressEl.style.width = Math.min(pct, 100) + '%';
            glowEl.style.left = Math.min(pct, 100) + '%';

            if (data.running && data.iteration > 0 && data.max_iterations > 0) {
                metaEl.textContent = `${data.iteration} / ${data.max_iterations}`;
            } else if (!data.running && state.logCount > 0) {
                metaEl.textContent = `${state.logCount} ${t('desktop.looper_logs_title').toLowerCase()}`;
            } else {
                metaEl.textContent = '';
            }

            // Highlight running step button
            $$(`.vd-looper-step-btn`).forEach(btn => {
                btn.classList.toggle('vd-looper-step-btn--running', data.running && btn.dataset.step === data.current_step);
            });

            if (data.logs && data.logs.length) {
                state.logCount = data.logs.length;
                const html = data.logs.map((log, i) => {
                    const stepMeta = STEP_META.find(s => s.key === log.step) || { color: '#9aa3ad', icon: '•' };
                    const stepLabel = t('desktop.looper_step_' + log.step) || log.step;
                    const isFirstOfStep = i === 0 || data.logs[i - 1].step !== log.step;
                    const title = log.iteration > 0 ? `#${log.iteration} ${stepLabel}` : stepLabel;
                    return `<div class="vd-looper-log${isFirstOfStep ? ' vd-looper-log--first' : ''}">
                        <div class="vd-looper-log-dot" style="background:${stepMeta.color};box-shadow:0 0 6px ${stepMeta.color}60"></div>
                        <div class="vd-looper-log-content">
                            <div class="vd-looper-log-header">
                                <span class="vd-looper-log-step" style="color:${stepMeta.color}">${esc(title)}</span>
                                <span class="vd-looper-log-time">${formatDuration(log.duration)}</span>
                            </div>
                            ${log.prompt ? `<div class="vd-looper-log-prompt">${highlightCode(log.prompt)}</div>` : ''}
                            ${log.response ? `<div class="vd-looper-log-response">${highlightCode(log.response)}</div>` : ''}
                        </div>
                    </div>`;
                }).join('');
                logsEl.innerHTML = html;
                logsEl.scrollTop = logsEl.scrollHeight;
            }

            if (!data.running && data.current_step === 'idle' && state.sse) {
                state.sse.close(); state.sse = null;
            }
        }

        $(`looper-start-${windowId}`).addEventListener('click', async () => {
            const body = readForm();
            try {
                await api('/api/desktop/looper/run', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(body)
                });
                $(`looper-logs-${windowId}`).innerHTML = '';
                state.logCount = 0;
                state.startTime = Date.now();
                connectStatus();
            } catch (e) {
                if (e && e.status === 409) {
                    if (notify) notify({ title: t('desktop.notification'), message: t('desktop.looper_already_running') });
                } else {
                    if (notify) notify({ title: t('desktop.notification'), message: t('desktop.looper_start_error') });
                }
            }
        });

        $(`looper-stop-${windowId}`).addEventListener('click', async () => {
            try {
                await api('/api/desktop/looper/stop', { method: 'POST' });
            } catch (e) {
                if (notify) notify({ title: t('desktop.notification'), message: t('desktop.looper_stop_error') });
            }
        });
    }

    function dispose(windowId) {
        const state = instances.get(windowId);
        if (!state) return;
        if (state.sse) { state.sse.close(); state.sse = null; }
        instances.delete(windowId);
    }

    window.LooperApp = { render, dispose };
})();
