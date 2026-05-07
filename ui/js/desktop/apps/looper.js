(function () {
    'use strict';

    const instances = new Map();

    function render(container, windowId, context) {
        dispose(windowId);

        const state = {
            presets: [],
            examples: [],
            providers: [],
            running: false,
            status: { current_step: 'idle', iteration: 0, max_iterations: 20, logs: [], last_result: '' },
            sse: null,
            selectedPresetId: null
        };
        instances.set(windowId, state);

        // Build UI
        container.innerHTML = `
            <div class="vd-looper">
                <div class="vd-looper-toolbar">
                    <div class="vd-looper-field">
                        <label>${esc(t('desktop.looper_preset'))}</label>
                        <select id="looper-preset-${windowId}">
                            <option value="">-- ${esc(t('desktop.looper_select_preset'))} --</option>
                        </select>
                        <button type="button" class="vd-looper-btn" id="looper-save-${windowId}">${esc(t('desktop.looper_save'))}</button>
                        <button type="button" class="vd-looper-btn vd-looper-btn-danger" id="looper-delete-${windowId}">${esc(t('desktop.looper_delete'))}</button>
                    </div>
                    <div class="vd-looper-field">
                        <label>${esc(t('desktop.looper_examples'))}</label>
                        <select id="looper-example-${windowId}">
                            <option value="">-- ${esc(t('desktop.looper_select_example'))} --</option>
                        </select>
                    </div>
                    <div class="vd-looper-field-row">
                        <div class="vd-looper-field">
                            <label>${esc(t('desktop.looper_provider'))}</label>
                            <select id="looper-provider-${windowId}"></select>
                        </div>
                        <div class="vd-looper-field">
                            <label>${esc(t('desktop.looper_model'))}</label>
                            <input type="text" id="looper-model-${windowId}" placeholder="${esc(t('desktop.looper_model_placeholder'))}">
                        </div>
                        <div class="vd-looper-field vd-looper-field-small">
                            <label>${esc(t('desktop.looper_max_iter'))}</label>
                            <input type="number" id="looper-max-iter-${windowId}" value="20" min="1" max="100">
                        </div>
                        <div class="vd-looper-field vd-looper-field-small">
                            <label>${esc(t('desktop.looper_context_mode'))}</label>
                            <select id="looper-context-mode-${windowId}">
                                <option value="every_iteration">${esc(t('desktop.looper_context_every_iteration'))}</option>
                                <option value="never">${esc(t('desktop.looper_context_never'))}</option>
                                <option value="every_step">${esc(t('desktop.looper_context_every_step'))}</option>
                            </select>
                        </div>
                    </div>
                </div>
                <div class="vd-looper-steps">
                    <div class="vd-looper-step">
                        <label>${esc(t('desktop.looper_prepare'))}</label>
                        <textarea id="looper-prepare-${windowId}" rows="3" placeholder="${esc(t('desktop.looper_prepare_placeholder'))}"></textarea>
                    </div>
                    <div class="vd-looper-step">
                        <label>${esc(t('desktop.looper_plan'))}</label>
                        <textarea id="looper-plan-${windowId}" rows="3" placeholder="${esc(t('desktop.looper_plan_placeholder'))}"></textarea>
                    </div>
                    <div class="vd-looper-step">
                        <label>${esc(t('desktop.looper_action'))}</label>
                        <textarea id="looper-action-${windowId}" rows="3" placeholder="${esc(t('desktop.looper_action_placeholder'))}"></textarea>
                    </div>
                    <div class="vd-looper-step">
                        <label>${esc(t('desktop.looper_test'))}</label>
                        <textarea id="looper-test-${windowId}" rows="3" placeholder="${esc(t('desktop.looper_test_placeholder'))}"></textarea>
                    </div>
                    <div class="vd-looper-step">
                        <label>${esc(t('desktop.looper_exit'))}</label>
                        <textarea id="looper-exit-${windowId}" rows="2" placeholder="${esc(t('desktop.looper_exit_placeholder'))}"></textarea>
                    </div>
                    <div class="vd-looper-step">
                        <label>${esc(t('desktop.looper_finish'))} <span class="vd-looper-optional">(${esc(t('desktop.optional'))})</span></label>
                        <textarea id="looper-finish-${windowId}" rows="2" placeholder="${esc(t('desktop.looper_finish_placeholder'))}"></textarea>
                    </div>
                </div>
                <div class="vd-looper-controls">
                    <button type="button" class="vd-looper-start" id="looper-start-${windowId}">▶ ${esc(t('desktop.looper_start'))}</button>
                    <button type="button" class="vd-looper-stop" id="looper-stop-${windowId}" disabled>⏹ ${esc(t('desktop.looper_stop'))}</button>
                </div>
                <div class="vd-looper-monitor" id="looper-monitor-${windowId}">
                    <div class="vd-looper-status" id="looper-status-${windowId}">${esc(t('desktop.looper_status_idle'))}</div>
                    <div class="vd-looper-progress"><div class="vd-looper-progress-bar" id="looper-progress-${windowId}"></div></div>
                    <div class="vd-looper-logs" id="looper-logs-${windowId}"></div>
                </div>
            </div>
        `;

        const $ = id => container.querySelector('#' + id);

        // Load providers from config
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

        // Load presets and examples
        async function loadPresets() {
            try {
                const res = await api('/api/desktop/looper/presets');
                if (res && res.presets) {
                    state.presets = res.presets;
                    const select = $(`looper-preset-${windowId}`);
                    select.innerHTML = `<option value="">-- ${esc(t('desktop.looper_select_preset'))} --</option>`;
                    res.presets.forEach(p => {
                        const opt = document.createElement('option');
                        opt.value = p.id;
                        opt.textContent = p.name + (p.is_builtin ? ' ★' : '');
                        select.appendChild(opt);
                    });
                }
            } catch (e) { console.error('Failed to load presets', e); }
        }
        async function loadExamples() {
            try {
                const res = await api('/api/desktop/looper/examples');
                if (res && res.examples) {
                    state.examples = res.examples;
                    const select = $(`looper-example-${windowId}`);
                    select.innerHTML = `<option value="">-- ${esc(t('desktop.looper_select_example'))} --</option>`;
                    res.examples.forEach((p, idx) => {
                        const opt = document.createElement('option');
                        opt.value = idx;
                        opt.textContent = p.name;
                        select.appendChild(opt);
                    });
                }
            } catch (e) { console.error('Failed to load examples', e); }
        }
        loadPresets();
        loadExamples();

        // Fill form from preset object
        function fillForm(p) {
            $(`looper-prepare-${windowId}`).value = p.prepare || '';
            $(`looper-plan-${windowId}`).value = p.plan || '';
            $(`looper-action-${windowId}`).value = p.action || '';
            $(`looper-test-${windowId}`).value = p.test || '';
            $(`looper-exit-${windowId}`).value = p.exit_cond || '';
            $(`looper-finish-${windowId}`).value = p.finish || '';
            $(`looper-provider-${windowId}`).value = p.provider_id || '';
            $(`looper-model-${windowId}`).value = p.model || '';
            $(`looper-max-iter-${windowId}`).value = p.max_iter || 20;
            $(`looper-context-mode-${windowId}`).value = p.context_mode || 'every_iteration';
        }

        function readForm() {
            return {
                prepare: $(`looper-prepare-${windowId}`).value,
                plan: $(`looper-plan-${windowId}`).value,
                action: $(`looper-action-${windowId}`).value,
                test: $(`looper-test-${windowId}`).value,
                exit_cond: $(`looper-exit-${windowId}`).value,
                finish: $(`looper-finish-${windowId}`).value,
                provider_id: $(`looper-provider-${windowId}`).value,
                model: $(`looper-model-${windowId}`).value,
                max_iter: parseInt($(`looper-max-iter-${windowId}`).value, 10) || 20,
                context_mode: $(`looper-context-mode-${windowId}`).value || 'every_iteration'
            };
        }

        // Event: preset select
        $(`looper-preset-${windowId}`).addEventListener('change', (e) => {
            const id = e.target.value;
            if (!id) return;
            const p = state.presets.find(x => String(x.id) === id);
            if (p) fillForm(p);
        });

        // Event: example select
        $(`looper-example-${windowId}`).addEventListener('change', (e) => {
            const idx = parseInt(e.target.value, 10);
            if (isNaN(idx)) return;
            const p = state.examples[idx];
            if (p) fillForm(p);
        });

        // Event: save preset
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
                context.notify({ title: t('desktop.looper_title'), message: t('desktop.looper_saved') });
            } catch (e) {
                if (context.notify) context.notify({ title: t('desktop.notification'), message: t('desktop.looper_save_error') });
            }
        });

        // Event: delete preset
        $(`looper-delete-${windowId}`).addEventListener('click', async () => {
            const select = $(`looper-preset-${windowId}`);
            const id = select.value;
            if (!id) return;
            if (!confirm(t('desktop.looper_delete_confirm'))) return;
            try {
                await api('/api/desktop/looper/presets/' + id, { method: 'DELETE' });
                await loadPresets();
                if (context.notify) context.notify({ title: t('desktop.looper_title'), message: t('desktop.looper_deleted') });
            } catch (e) {
                if (context.notify) context.notify({ title: t('desktop.notification'), message: t('desktop.looper_delete_error') });
            }
        });

        // SSE status connection
        function connectStatus() {
            if (state.sse) {
                state.sse.close();
                state.sse = null;
            }
            const url = '/api/desktop/looper/status';
            const evtSource = new EventSource(url);
            state.sse = evtSource;
            evtSource.onmessage = (event) => {
                try {
                    const data = JSON.parse(event.data);
                    updateStatus(data);
                } catch (e) { /* ignore parse errors */ }
            };
            evtSource.onerror = () => {
                evtSource.close();
                state.sse = null;
                // Auto-reconnect after 2s if loop is still running
                if (state.running) {
                    setTimeout(() => connectStatus(), 2000);
                }
            };
        }

        function updateStatus(data) {
            state.status = data;
            const statusEl = $(`looper-status-${windowId}`);
            const progressEl = $(`looper-progress-${windowId}`);
            const logsEl = $(`looper-logs-${windowId}`);
            const startBtn = $(`looper-start-${windowId}`);
            const stopBtn = $(`looper-stop-${windowId}`);

            state.running = data.running;
            startBtn.disabled = data.running;
            stopBtn.disabled = !data.running;

            if (data.error) {
                statusEl.textContent = t('desktop.looper_error') + ': ' + data.error;
                statusEl.classList.add('vd-looper-error');
            } else if (data.running) {
                statusEl.classList.remove('vd-looper-error');
                const stepLabel = t('desktop.looper_step_' + data.current_step) || data.current_step;
                if (data.current_step === 'prepare' || data.current_step === 'finish') {
                    statusEl.textContent = stepLabel;
                } else {
                    statusEl.textContent = t('desktop.looper_iteration')
                        .replace('{{n}}', data.iteration)
                        .replace('{{max}}', data.max_iterations) + ' - ' + stepLabel;
                }
            } else {
                statusEl.classList.remove('vd-looper-error');
                statusEl.textContent = t('desktop.looper_status_idle');
            }

            // Progress bar
            const pct = data.max_iterations > 0 ? (data.iteration / data.max_iterations) * 100 : 0;
            progressEl.style.width = Math.min(pct, 100) + '%';

            // Logs
            if (data.logs && data.logs.length) {
                logsEl.innerHTML = data.logs.map((log, i) => {
                    const stepLabel = t('desktop.looper_step_' + log.step) || log.step;
                    const title = log.iteration > 0 ? `#${log.iteration} ${stepLabel}` : stepLabel;
                    return `<div class="vd-looper-log">
                        <div class="vd-looper-log-header">
                            <strong>${esc(title)}</strong>
                            <span class="vd-looper-log-time">${log.duration}ms</span>
                        </div>
                        <div class="vd-looper-log-prompt">${esc(log.prompt)}</div>
                        <div class="vd-looper-log-response">${esc(log.response)}</div>
                    </div>`;
                }).join('');
                logsEl.scrollTop = logsEl.scrollHeight;
            }

            if (!data.running && data.current_step === 'idle' && state.sse) {
                state.sse.close();
                state.sse = null;
            }
        }

        // Start
        $(`looper-start-${windowId}`).addEventListener('click', async () => {
            const body = readForm();
            try {
                await api('/api/desktop/looper/run', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(body)
                });
                $(`looper-logs-${windowId}`).innerHTML = '';
                connectStatus();
            } catch (e) {
                if (e && e.status === 409) {
                    if (context.notify) context.notify({ title: t('desktop.notification'), message: t('desktop.looper_already_running') });
                } else {
                    if (context.notify) context.notify({ title: t('desktop.notification'), message: t('desktop.looper_start_error') });
                }
            }
        });

        // Stop
        $(`looper-stop-${windowId}`).addEventListener('click', async () => {
            try {
                await api('/api/desktop/looper/stop', { method: 'POST' });
            } catch (e) {
                if (context.notify) context.notify({ title: t('desktop.notification'), message: t('desktop.looper_stop_error') });
            }
        });
    }

    function dispose(windowId) {
        const state = instances.get(windowId);
        if (!state) return;
        if (state.sse) {
            state.sse.close();
            state.sse = null;
        }
        instances.delete(windowId);
    }

    window.LooperApp = { render, dispose };
})();
