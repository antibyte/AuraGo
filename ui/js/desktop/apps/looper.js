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
            examples: [],
            providers: [],
            running: false,
            status: { current_step: 'idle', iteration: 0, max_iterations: 20, logs: [], last_result: '' },
            sse: null,
            selectedPresetId: null,
            expandedSteps: new Set(['prepare', 'plan', 'action', 'test', 'exit']),
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
                        <div class="vd-looper-field">
                            <label>${esc(t('desktop.looper_examples'))}</label>
                            <select id="looper-example-${windowId}">
                                <option value="">${esc(t('desktop.looper_select_example'))}</option>
                            </select>
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

                <div class="vd-looper-steps" id="looper-steps-${windowId}">
                    ${STEP_META.map((step, idx) => `
                        <div class="vd-looper-step" data-step="${step.key}" style="--step-color: ${step.color}">
                            <div class="vd-looper-step-header">
                                <div class="vd-looper-step-badge" style="background: ${step.color}20; color: ${step.color}; border-color: ${step.color}40;">
                                    <span class="vd-looper-step-icon">${step.icon}</span>
                                    <span class="vd-looper-step-num">${idx + 1}</span>
                                </div>
                                <div class="vd-looper-step-title">
                                    <span class="vd-looper-step-name">${esc(t(step.labelKey))}</span>
                                    ${step.key === 'finish' ? `<span class="vd-looper-step-optional">${esc(t('desktop.optional'))}</span>` : ''}
                                </div>
                                <button type="button" class="vd-looper-step-toggle" aria-expanded="true">
                                    <svg class="vd-looper-chevron" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="6 9 12 15 18 9"/></svg>
                                </button>
                            </div>
                            <div class="vd-looper-step-body">
                                <textarea id="looper-${step.key}-${windowId}" rows="${step.key === 'exit' || step.key === 'finish' ? 2 : 3}" placeholder="${esc(t(step.descKey))}"></textarea>
                            </div>
                        </div>
                    `).join('')}
                </div>

                <div class="vd-looper-controls">
                    <button type="button" class="vd-looper-start" id="looper-start-${windowId}">
                        <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor"><polygon points="5 3 19 12 5 21 5 3"/></svg>
                        <span>${esc(t('desktop.looper_start'))}</span>
                    </button>
                    <button type="button" class="vd-looper-stop" id="looper-stop-${windowId}" disabled>
                        <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor"><rect x="6" y="6" width="12" height="12" rx="2"/></svg>
                        <span>${esc(t('desktop.looper_stop'))}</span>
                    </button>
                    <div class="vd-looper-step-actions">
                        <button type="button" class="vd-looper-step-btn" id="looper-expand-${windowId}">
                            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><polyline points="15 3 21 3 21 9"/><polyline points="9 21 3 21 3 15"/><line x1="21" y1="3" x2="14" y2="10"/><line x1="3" y1="21" x2="10" y2="14"/></svg>
                        </button>
                        <button type="button" class="vd-looper-step-btn" id="looper-collapse-${windowId}">
                            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><polyline points="4 14 10 14 10 20"/><polyline points="20 10 14 10 14 4"/><line x1="14" y1="10" x2="21" y2="3"/><line x1="3" y1="21" x2="10" y2="14"/></svg>
                        </button>
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

        // ── Accordion ──
        function setupAccordion() {
            $$(`.vd-looper-step-header`).forEach(hdr => {
                hdr.addEventListener('click', (e) => {
                    if (e.target.closest('textarea')) return;
                    const step = hdr.closest('.vd-looper-step');
                    const key = step.dataset.step;
                    const body = step.querySelector('.vd-looper-step-body');
                    const btn = hdr.querySelector('.vd-looper-step-toggle');
                    const isOpen = body.style.display !== 'none' && body.style.height !== '0px';
                    if (isOpen) {
                        body.style.height = body.scrollHeight + 'px';
                        requestAnimationFrame(() => { body.style.height = '0px'; body.style.opacity = '0'; });
                        setTimeout(() => { body.style.display = 'none'; }, 250);
                        btn.setAttribute('aria-expanded', 'false');
                        btn.querySelector('.vd-looper-chevron').style.transform = 'rotate(-90deg)';
                        state.expandedSteps.delete(key);
                    } else {
                        body.style.display = 'block';
                        body.style.height = '0px';
                        body.style.opacity = '0';
                        requestAnimationFrame(() => { body.style.height = body.scrollHeight + 'px'; body.style.opacity = '1'; });
                        setTimeout(() => { body.style.height = 'auto'; }, 250);
                        btn.setAttribute('aria-expanded', 'true');
                        btn.querySelector('.vd-looper-chevron').style.transform = 'rotate(0deg)';
                        state.expandedSteps.add(key);
                    }
                });
            });
        }
        setupAccordion();

        $(`looper-expand-${windowId}`).addEventListener('click', () => {
            $$(`.vd-looper-step-body`).forEach((body, i) => {
                const step = body.closest('.vd-looper-step');
                const key = step.dataset.step;
                const btn = step.querySelector('.vd-looper-step-toggle');
                body.style.display = 'block';
                body.style.height = '0px';
                body.style.opacity = '0';
                requestAnimationFrame(() => { body.style.height = body.scrollHeight + 'px'; body.style.opacity = '1'; });
                setTimeout(() => { body.style.height = 'auto'; }, 250);
                btn.setAttribute('aria-expanded', 'true');
                btn.querySelector('.vd-looper-chevron').style.transform = 'rotate(0deg)';
                state.expandedSteps.add(key);
            });
        });

        $(`looper-collapse-${windowId}`).addEventListener('click', () => {
            $$(`.vd-looper-step-body`).forEach(body => {
                const step = body.closest('.vd-looper-step');
                const key = step.dataset.step;
                const btn = step.querySelector('.vd-looper-step-toggle');
                body.style.height = body.scrollHeight + 'px';
                requestAnimationFrame(() => { body.style.height = '0px'; body.style.opacity = '0'; });
                setTimeout(() => { body.style.display = 'none'; }, 250);
                btn.setAttribute('aria-expanded', 'false');
                btn.querySelector('.vd-looper-chevron').style.transform = 'rotate(-90deg)';
                state.expandedSteps.delete(key);
            });
        });

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
                    res.presets.forEach(p => {
                        const opt = document.createElement('option');
                        opt.value = p.id;
                        opt.textContent = (p.is_builtin ? '★ ' : '') + p.name;
                        if (p.is_builtin) opt.className = 'vd-looper-opt-builtin';
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
                    select.innerHTML = `<option value="">${esc(t('desktop.looper_select_example'))}</option>`;
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

        $(`looper-preset-${windowId}`).addEventListener('change', (e) => {
            const id = e.target.value;
            if (!id) return;
            const p = state.presets.find(x => String(x.id) === id);
            if (p) fillForm(p);
        });

        $(`looper-example-${windowId}`).addEventListener('change', (e) => {
            const idx = parseInt(e.target.value, 10);
            if (isNaN(idx)) return;
            const p = state.examples[idx];
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

            // Meta info
            if (data.running && data.iteration > 0 && data.max_iterations > 0) {
                metaEl.textContent = `${data.iteration} / ${data.max_iterations}`;
            } else if (!data.running && state.logCount > 0) {
                metaEl.textContent = `${state.logCount} ${t('desktop.looper_logs_title').toLowerCase()}`;
            } else {
                metaEl.textContent = '';
            }

            // Highlight active step
            $$(`.vd-looper-step`).forEach(stepEl => {
                stepEl.classList.remove('vd-looper-step--active');
            });
            if (data.running && data.current_step) {
                const active = container.querySelector(`.vd-looper-step[data-step="${data.current_step}"]`);
                if (active) active.classList.add('vd-looper-step--active');
            }

            // Logs
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
