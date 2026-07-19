(function () {
    'use strict';

    const instances = new Map();

    const STEP_META = [
        { key: 'prepare',  labelKey: 'desktop.looper_prepare',  color: '#3b82f6', icon: '⚡', descKey: 'desktop.looper_prepare_placeholder' },
        { key: 'plan',     labelKey: 'desktop.looper_plan',     color: '#f59e0b', icon: '🎯', descKey: 'desktop.looper_plan_placeholder' },
        { key: 'action',   labelKey: 'desktop.looper_action',   color: '#10b981', icon: '▶',  descKey: 'desktop.looper_action_placeholder' },
        { key: 'test',     labelKey: 'desktop.looper_test',     color: '#8b5cf6', icon: '🔍', descKey: 'desktop.looper_test_placeholder' },
        { key: 'exit',     labelKey: 'desktop.looper_exit',     color: '#ef4444', icon: '◆',  descKey: 'desktop.looper_exit_placeholder' },
        { key: 'finish',   labelKey: 'desktop.looper_finish',   color: '#06b6d4', icon: '✓',  descKey: 'desktop.looper_finish_placeholder' },
        { key: 'summarize', labelKey: 'desktop.looper_step_summarize', color: '#a855f7', icon: '✎', descKey: '' },
        { key: 'stuck',    labelKey: 'desktop.looper_step_stuck', color: '#f97316', icon: '⚠', descKey: '' }
    ];

    function logEntryKey(log, index) {
        return String(log.iteration || 0) + '|' + String(log.step || '') + '|' + String(log.duration || 0) + '|' + index;
    }

    function formatCost(usd) {
        if (!usd || usd <= 0) return '';
        if (usd < 0.01) return '<$0.01';
        return '$' + usd.toFixed(usd < 1 ? 3 : 2);
    }

    function render(container, windowId, context) {
        dispose(windowId);

        const { esc, t, api, notify, readonly, promptDialog, confirmDialog } = context;
        const isReadonly = !!readonly;

        const askPrompt = async (title, value) => {
            if (typeof promptDialog === 'function') {
                return promptDialog(title, value || '');
            }
            // Fallback only if shell did not inject modal helper
            return window.prompt(title, value || '');
        };
        const askConfirm = async (title, message) => {
            if (typeof confirmDialog === 'function') {
                return confirmDialog(title, message || '');
            }
            return window.confirm(message || title);
        };

        const state = {
            presets: [],
            providers: [],
            running: false,
            paused: false,
            status: { current_step: 'idle', iteration: 0, max_iterations: 20, logs: [], last_result: '' },
            sse: null,
            selectedPresetId: null,
            activeStep: 'prepare',
            stepValues: { prepare: '', plan: '', action: '', test: '', exit: '', finish: '' },
            startTime: null,
            logCount: 0,
            collapsed: false,
            autoScroll: true,
            workspaceHeight: null,
            savedWorkspaceHeight: null,
            logExpandState: new Map(),
            lastLogSignature: '',
            advancedOpen: false,
            finishContext: 'last_test',
            prepareTruncation: 0,
            summarizeIterations: false,
            exitMinConfidence: 0.55,
            stuckTestRepeats: 3,
            disposed: false
        };
        instances.set(windowId, state);

        container.innerHTML = `
            <div class="vd-looper${isReadonly ? ' vd-looper--readonly' : ''}">
                <div class="vd-looper-header">
                    <div class="vd-looper-header-group">
                        <div class="vd-looper-field vd-looper-field-preset">
                            <label>${esc(t('desktop.looper_preset'))}</label>
                            <div class="vd-looper-input-row">
                                <select id="looper-preset-${windowId}">
                                    <option value="">${esc(t('desktop.looper_select_preset'))}</option>
                                </select>
                                <button type="button" class="vd-looper-icon-btn vd-looper-btn-save" id="looper-save-${windowId}" title="${esc(t('desktop.looper_save'))}" ${isReadonly ? 'disabled' : ''}>
                                    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"/><polyline points="17 21 17 13 7 13 7 21"/><polyline points="7 3 7 8 15 8"/></svg>
                                </button>
                                <button type="button" class="vd-looper-icon-btn vd-looper-btn-delete" id="looper-delete-${windowId}" title="${esc(t('desktop.looper_delete'))}" ${isReadonly ? 'disabled' : ''}>
                                    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
                                </button>
                            </div>
                        </div>
                    </div>
                    <div class="vd-looper-header-group vd-looper-header-group-config">
                        <div class="vd-looper-field">
                            <label>${esc(t('desktop.looper_provider'))}</label>
                            <select id="looper-provider-${windowId}" ${isReadonly ? 'disabled' : ''}></select>
                        </div>
                        <div class="vd-looper-field">
                            <label>${esc(t('desktop.looper_model'))}</label>
                            <input type="text" id="looper-model-${windowId}" placeholder="${esc(t('desktop.looper_model_placeholder'))}" inputmode="text" enterkeyhint="done" autocapitalize="off" ${isReadonly ? 'disabled' : ''}>
                        </div>
                        <div class="vd-looper-field vd-looper-field-xs">
                            <label>${esc(t('desktop.looper_max_iter'))}</label>
                            <input type="number" id="looper-max-iter-${windowId}" value="20" min="1" max="100" ${isReadonly ? 'disabled' : ''}>
                        </div>
                        <div class="vd-looper-field vd-looper-field-sm">
                            <label>${esc(t('desktop.looper_context_mode'))}</label>
                            <select id="looper-context-mode-${windowId}" title="${esc(t('desktop.looper_context_mode_help'))}" ${isReadonly ? 'disabled' : ''}>
                                <option value="every_iteration">${esc(t('desktop.looper_context_every_iteration'))}</option>
                                <option value="never">${esc(t('desktop.looper_context_never'))}</option>
                                <option value="every_step">${esc(t('desktop.looper_context_every_step'))}</option>
                            </select>
                            <div id="looper-context-warning-${windowId}" class="vd-looper-context-warning" style="display:none;"></div>
                            <div class="vd-looper-field-hint">${esc(t('desktop.looper_context_mode_help'))}</div>
                        </div>
                    </div>
                    <div class="vd-looper-header-group vd-looper-header-group-advanced">
                        <button type="button" class="vd-looper-advanced-toggle" id="looper-advanced-toggle-${windowId}" aria-expanded="false">
                            <span>${esc(t('desktop.looper_advanced'))}</span>
                            <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2"><polyline points="6 9 12 15 18 9"></polyline></svg>
                        </button>
                        <div class="vd-looper-advanced" id="looper-advanced-${windowId}" hidden>
                            <div class="vd-looper-field vd-looper-field-sm">
                                <label>${esc(t('desktop.looper_finish_context'))}</label>
                                <select id="looper-finish-context-${windowId}" ${isReadonly ? 'disabled' : ''}>
                                    <option value="last_test">${esc(t('desktop.looper_finish_context_last_test'))}</option>
                                    <option value="last_action_test">${esc(t('desktop.looper_finish_context_last_action_test'))}</option>
                                    <option value="full">${esc(t('desktop.looper_finish_context_full'))}</option>
                                    <option value="none">${esc(t('desktop.looper_finish_context_none'))}</option>
                                </select>
                            </div>
                            <div class="vd-looper-field vd-looper-field-xs">
                                <label>${esc(t('desktop.looper_prepare_truncation'))}</label>
                                <input type="number" id="looper-prepare-trunc-${windowId}" value="0" min="0" max="20000" placeholder="2000" title="${esc(t('desktop.looper_prepare_truncation_hint'))}" ${isReadonly ? 'disabled' : ''}>
                            </div>
                            <div class="vd-looper-field vd-looper-field-xs">
                                <label>${esc(t('desktop.looper_exit_min_confidence'))}</label>
                                <input type="number" id="looper-exit-conf-${windowId}" value="0.55" min="0" max="1" step="0.05" title="${esc(t('desktop.looper_exit_min_confidence_hint'))}" ${isReadonly ? 'disabled' : ''}>
                            </div>
                            <div class="vd-looper-field vd-looper-field-xs">
                                <label>${esc(t('desktop.looper_stuck_repeats'))}</label>
                                <input type="number" id="looper-stuck-${windowId}" value="3" min="0" max="20" title="${esc(t('desktop.looper_stuck_repeats_hint'))}" ${isReadonly ? 'disabled' : ''}>
                            </div>
                            <label class="vd-looper-check">
                                <input type="checkbox" id="looper-summarize-${windowId}" ${isReadonly ? 'disabled' : ''}>
                                <span>${esc(t('desktop.looper_summarize_iterations'))}</span>
                            </label>
                        </div>
                    </div>
                </div>

                <div class="vd-looper-workspace">
                    <div class="vd-looper-sidebar">
                        ${STEP_META.filter(s => s.key !== 'summarize' && s.key !== 'stuck').map((step, idx) => `
                            <button type="button" class="vd-looper-step-btn${step.key === 'finish' ? ' vd-looper-step-btn--optional' : ''}" data-step="${step.key}" id="looper-step-btn-${step.key}-${windowId}" style="--step-color: ${step.color}">
                                <span class="vd-looper-step-btn-icon">${step.icon}</span>
                                <span class="vd-looper-step-btn-num">${idx + 1}</span>
                                <span class="vd-looper-step-btn-name">${esc(t(step.labelKey))}</span>
                            </button>
                        `).join('')}
                        <div class="vd-looper-sidebar-actions">
                            <button type="button" class="vd-looper-start" id="looper-start-${windowId}" ${isReadonly ? 'disabled' : ''}>
                                <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor"><polygon points="5 3 19 12 5 21 5 3"/></svg>
                                <span>${esc(t('desktop.looper_start'))}</span>
                            </button>
                            <button type="button" class="vd-looper-pause" id="looper-pause-${windowId}" disabled>
                                <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor"><rect x="6" y="5" width="4" height="14" rx="1"/><rect x="14" y="5" width="4" height="14" rx="1"/></svg>
                                <span>${esc(t('desktop.looper_pause'))}</span>
                            </button>
                            <button type="button" class="vd-looper-resume" id="looper-resume-${windowId}" disabled style="display:none;">
                                <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor"><polygon points="5 3 19 12 5 21 5 3"/></svg>
                                <span>${esc(t('desktop.looper_resume'))}</span>
                            </button>
                            <button type="button" class="vd-looper-stop" id="looper-stop-${windowId}" disabled>
                                <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor"><rect x="6" y="6" width="12" height="12" rx="2"/></svg>
                                <span>${esc(t('desktop.looper_stop'))}</span>
                            </button>
                        </div>
                    </div>
                    <div class="vd-looper-editor">
                        <div class="vd-looper-editor-header" id="looper-editor-header-${windowId}"></div>
                        <textarea id="looper-editor-textarea-${windowId}" rows="12" placeholder="" ${isReadonly ? 'readonly' : ''}></textarea>
                    </div>
                </div>

                <div class="vd-looper-resizer" id="looper-resizer-${windowId}" role="separator" aria-orientation="horizontal" title="${esc(t('desktop.looper_resize_handle'))}" tabindex="0"></div>

                <div class="vd-looper-monitor" id="looper-monitor-${windowId}">
                    <div class="vd-looper-monitor-header">
                        <div class="vd-looper-monitor-title">
                            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>
                            <span>${esc(t('desktop.looper_status_idle'))}</span>
                        </div>
                        <div class="vd-looper-monitor-actions">
                            <button type="button" class="vd-looper-monitor-btn vd-looper-monitor-btn-pause" id="looper-monitor-pause-${windowId}" style="display: none;" title="${esc(t('desktop.looper_pause'))}">
                                <svg viewBox="0 0 24 24" width="12" height="12" fill="currentColor"><rect x="6" y="5" width="4" height="14" rx="1"/><rect x="14" y="5" width="4" height="14" rx="1"/></svg>
                                <span>${esc(t('desktop.looper_pause'))}</span>
                            </button>
                            <button type="button" class="vd-looper-monitor-btn vd-looper-monitor-btn-resume" id="looper-monitor-resume-${windowId}" style="display: none;" title="${esc(t('desktop.looper_resume'))}">
                                <svg viewBox="0 0 24 24" width="12" height="12" fill="currentColor"><polygon points="5 3 19 12 5 21 5 3"/></svg>
                                <span>${esc(t('desktop.looper_resume'))}</span>
                            </button>
                            <button type="button" class="vd-looper-monitor-btn vd-looper-monitor-btn-stop" id="looper-monitor-stop-${windowId}" style="display: none;" title="${esc(t('desktop.looper_stop'))}">
                                <svg viewBox="0 0 24 24" width="12" height="12" fill="currentColor"><rect x="6" y="6" width="12" height="12" rx="2"/></svg>
                                <span>${esc(t('desktop.looper_stop'))}</span>
                            </button>
                            <button type="button" class="vd-looper-monitor-btn vd-looper-monitor-btn-toggle" id="looper-toggle-view-${windowId}" title="${esc(t('desktop.looper_toggle_view'))}">
                                <svg id="looper-toggle-icon-${windowId}" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                                    <polyline points="18 15 12 9 6 15"></polyline>
                                </svg>
                            </button>
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
                        <button type="button" class="vd-looper-jump-bottom" id="looper-jump-bottom-${windowId}" title="${esc(t('desktop.looper_jump_bottom'))}" aria-label="${esc(t('desktop.looper_jump_bottom'))}">
                            <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="6 9 12 15 18 9"></polyline></svg>
                            <span>${esc(t('desktop.looper_jump_bottom'))}</span>
                        </button>
                    </div>
                </div>
            </div>
        `;

        const $ = id => container.querySelector('#' + id);
        const $$ = sel => container.querySelectorAll(sel);

        const looperEl = container.querySelector('.vd-looper');
        const toggleIcon = $(`looper-toggle-icon-${windowId}`);

        function updateToggleState(isCollapsed) {
            if (!looperEl) return;
            const workspace = looperEl.querySelector('.vd-looper-workspace');
            looperEl.classList.toggle('vd-looper--collapsed', isCollapsed);
            if (toggleIcon) {
                if (isCollapsed) {
                    toggleIcon.innerHTML = '<polyline points="6 9 12 15 18 9"></polyline>';
                } else {
                    toggleIcon.innerHTML = '<polyline points="18 15 12 9 6 15"></polyline>';
                }
            }
            if (workspace) {
                if (isCollapsed) {
                    state.savedWorkspaceHeight = state.workspaceHeight || workspace.offsetHeight;
                    workspace.style.flexBasis = '0px';
                    workspace.style.maxHeight = '0px';
                    workspace.style.opacity = '0';
                    workspace.style.overflow = 'hidden';
                    workspace.style.padding = '0 14px';
                    workspace.style.pointerEvents = 'none';
                } else {
                    const restore = state.savedWorkspaceHeight || 200;
                    workspace.style.flexBasis = restore + 'px';
                    workspace.style.maxHeight = '';
                    workspace.style.opacity = '';
                    workspace.style.overflow = '';
                    workspace.style.padding = '';
                    workspace.style.pointerEvents = '';
                    state.workspaceHeight = restore;
                }
            }
        }

        function saveCurrentStepValue() {
            const textarea = $(`looper-editor-textarea-${windowId}`);
            if (textarea && state.activeStep) {
                state.stepValues[state.activeStep] = textarea.value;
            }
        }

        function setActiveStep(key) {
            if (state.activeStep !== key) {
                saveCurrentStepValue();
            }
            state.activeStep = key;
            const meta = STEP_META.find(s => s.key === key);
            if (!meta || !meta.descKey) return;

            $$(`.vd-looper-step-btn`).forEach(btn => {
                btn.classList.toggle('vd-looper-step-btn--active', btn.dataset.step === key);
            });

            const header = $(`looper-editor-header-${windowId}`);
            const textarea = $(`looper-editor-textarea-${windowId}`);
            const headerText = meta.key === 'finish'
                ? t(meta.labelKey) + ' (' + t('desktop.optional').toLowerCase() + ')'
                : t(meta.labelKey);
            header.textContent = headerText;
            header.style.color = meta.color;
            textarea.placeholder = t(meta.descKey);
            textarea.value = state.stepValues[key] || '';
            textarea.style.borderColor = meta.color + '40';
            if (!isReadonly) textarea.focus();
        }

        $$(`.vd-looper-step-btn`).forEach(btn => {
            btn.addEventListener('click', () => {
                if (!state.running) setActiveStep(btn.dataset.step);
            });
        });

        setActiveStep('prepare');

        const advToggle = $(`looper-advanced-toggle-${windowId}`);
        const advPanel = $(`looper-advanced-${windowId}`);
        if (advToggle && advPanel) {
            advToggle.addEventListener('click', () => {
                state.advancedOpen = !state.advancedOpen;
                advPanel.hidden = !state.advancedOpen;
                advToggle.setAttribute('aria-expanded', String(state.advancedOpen));
                advToggle.classList.toggle('vd-looper-advanced-toggle--open', state.advancedOpen);
            });
        }

        if ($(`looper-toggle-view-${windowId}`)) {
            $(`looper-toggle-view-${windowId}`).addEventListener('click', () => {
                state.collapsed = !state.collapsed;
                updateToggleState(state.collapsed);
            });
        }

        const resizerEl = $(`looper-resizer-${windowId}`);
        const workspaceEl = looperEl.querySelector('.vd-looper-workspace');
        const monitorEl = $(`looper-monitor-${windowId}`);

        if (resizerEl && workspaceEl && monitorEl) {
            let startY = 0;
            let startH = 0;

            resizerEl.addEventListener('pointerdown', (e) => {
                e.preventDefault();
                resizerEl.setPointerCapture(e.pointerId);
                startY = e.clientY;
                startH = workspaceEl.offsetHeight;
                resizerEl.classList.add('vd-looper-resizer--active');
                document.body.style.cursor = 'row-resize';
                document.body.style.userSelect = 'none';
            });

            resizerEl.addEventListener('pointermove', (e) => {
                if (!resizerEl.hasPointerCapture(e.pointerId)) return;
                const dy = e.clientY - startY;
                const looperH = looperEl.offsetHeight;
                const minWS = 80;
                const minMon = 80;
                let newH = Math.max(minWS, Math.min(startH + dy, looperH - minMon));
                workspaceEl.style.flexBasis = newH + 'px';
                state.workspaceHeight = newH;
            });

            const endResize = (e) => {
                if (!resizerEl.hasPointerCapture(e.pointerId)) return;
                resizerEl.releasePointerCapture(e.pointerId);
                resizerEl.classList.remove('vd-looper-resizer--active');
                document.body.style.cursor = '';
                document.body.style.userSelect = '';
            };

            resizerEl.addEventListener('pointerup', endResize);
            resizerEl.addEventListener('pointercancel', endResize);

            resizerEl.addEventListener('keydown', (e) => {
                const step = 20;
                if (e.key === 'ArrowUp') {
                    e.preventDefault();
                    const h = Math.max(80, (workspaceEl.offsetHeight || 200) - step);
                    workspaceEl.style.flexBasis = h + 'px';
                    state.workspaceHeight = h;
                } else if (e.key === 'ArrowDown') {
                    e.preventDefault();
                    const looperH = looperEl.offsetHeight;
                    const h = Math.min((workspaceEl.offsetHeight || 200) + step, looperH - 80);
                    workspaceEl.style.flexBasis = h + 'px';
                    state.workspaceHeight = h;
                }
            });
        }

        const logsEl = $(`looper-logs-${windowId}`);
        const jumpBottomEl = $(`looper-jump-bottom-${windowId}`);

        if (logsEl) {
            logsEl.addEventListener('scroll', () => {
                const atBottom = logsEl.scrollTop + logsEl.clientHeight >= logsEl.scrollHeight - 4;
                state.autoScroll = atBottom;
                if (jumpBottomEl) {
                    jumpBottomEl.classList.toggle('vd-looper-jump-bottom--visible', !atBottom && state.logCount > 0);
                }
            });
        }

        if (jumpBottomEl) {
            jumpBottomEl.addEventListener('click', () => {
                if (logsEl) {
                    logsEl.scrollTop = logsEl.scrollHeight;
                    state.autoScroll = true;
                    jumpBottomEl.classList.remove('vd-looper-jump-bottom--visible');
                }
            });
        }

        function setWorkspaceRunning(running) {
            if (!workspaceEl) return;
            if (running) {
                const currentH = workspaceEl.offsetHeight;
                if (!state.savedWorkspaceHeight) {
                    state.savedWorkspaceHeight = currentH > 10 ? currentH : 200;
                }
                const looperH = looperEl.offsetHeight;
                const targetH = Math.round(looperH * 0.35);
                workspaceEl.style.flexBasis = targetH + 'px';
                state.workspaceHeight = targetH;
            } else {
                if (state.savedWorkspaceHeight) {
                    workspaceEl.style.flexBasis = state.savedWorkspaceHeight + 'px';
                    state.workspaceHeight = state.savedWorkspaceHeight;
                    state.savedWorkspaceHeight = null;
                }
            }
        }

        function setInitialWorkspaceHeight() {
            if (!workspaceEl) return;
            const looperH = looperEl.offsetHeight;
            if (looperH > 0 && !state.workspaceHeight) {
                const initH = Math.round(looperH * 0.5);
                workspaceEl.style.flexBasis = initH + 'px';
                state.workspaceHeight = initH;
            }
        }
        requestAnimationFrame(setInitialWorkspaceHeight);

        async function stopLoop() {
            try {
                await api('/api/desktop/looper/stop', { method: 'POST' });
            } catch (e) {
                if (notify) notify({ title: t('desktop.notification'), message: t('desktop.looper_stop_error') });
            }
        }

        async function pauseLoop() {
            try {
                await api('/api/desktop/looper/pause', { method: 'POST' });
                if (notify) notify({ title: t('desktop.looper_title'), message: t('desktop.looper_pause_requested') });
            } catch (e) {
                if (notify) notify({ title: t('desktop.notification'), message: t('desktop.looper_pause_error') });
            }
        }

        async function resumeLoop() {
            if (isReadonly) return;
            const body = readForm();
            try {
                await api('/api/desktop/looper/resume', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(body)
                });
                connectStatus();
                setWorkspaceRunning(true);
            } catch (e) {
                if (notify) notify({ title: t('desktop.notification'), message: (e && e.message) || t('desktop.looper_resume_error') });
            }
        }

        if ($(`looper-monitor-stop-${windowId}`)) {
            $(`looper-monitor-stop-${windowId}`).addEventListener('click', stopLoop);
        }
        if ($(`looper-monitor-pause-${windowId}`)) {
            $(`looper-monitor-pause-${windowId}`).addEventListener('click', pauseLoop);
        }
        if ($(`looper-monitor-resume-${windowId}`)) {
            $(`looper-monitor-resume-${windowId}`).addEventListener('click', resumeLoop);
        }

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
                    const prev = select.value || state.selectedPresetId || '';
                    select.innerHTML = `<option value="">${esc(t('desktop.looper_select_preset'))}</option>`;
                    const builtins = res.presets.filter(p => p.is_builtin);
                    const users = res.presets.filter(p => !p.is_builtin);
                    if (builtins.length) {
                        const grp = document.createElement('optgroup');
                        grp.label = t('desktop.looper_examples');
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
                        grp.label = t('desktop.looper_preset');
                        users.forEach(p => {
                            const opt = document.createElement('option');
                            opt.value = p.id;
                            opt.textContent = p.name;
                            grp.appendChild(opt);
                        });
                        select.appendChild(grp);
                    }
                    if (prev) {
                        select.value = prev;
                        state.selectedPresetId = prev;
                    }
                }
            } catch (e) { console.error('Failed to load presets', e); }
        }
        loadPresets();

        setTimeout(updateContextWarning, 50);

        function fillForm(p) {
            state.stepValues = {
                prepare: p.prepare || '',
                plan: p.plan || '',
                action: p.action || '',
                test: p.test || '',
                exit: p.exit_cond || '',
                finish: p.finish || ''
            };
            const textarea = $(`looper-editor-textarea-${windowId}`);
            if (textarea) {
                textarea.value = state.stepValues[state.activeStep] || '';
            }
            $(`looper-provider-${windowId}`).value = p.provider_id || '';
            $(`looper-model-${windowId}`).value = p.model || '';
            $(`looper-max-iter-${windowId}`).value = p.max_iter || 20;
            $(`looper-context-mode-${windowId}`).value = p.context_mode || 'every_iteration';
            $(`looper-finish-context-${windowId}`).value = p.finish_context || 'last_test';
            $(`looper-prepare-trunc-${windowId}`).value = p.prepare_truncation || 0;
            $(`looper-summarize-${windowId}`).checked = !!p.summarize_iterations;
            state.finishContext = p.finish_context || 'last_test';
            state.prepareTruncation = p.prepare_truncation || 0;
            state.summarizeIterations = !!p.summarize_iterations;
            updateContextWarning();
        }

        const contextSelect = $(`looper-context-mode-${windowId}`);
        const contextWarning = $(`looper-context-warning-${windowId}`);

        function updateContextWarning() {
            if (!contextSelect || !contextWarning) return;

            const mode = contextSelect.value;
            const maxIter = parseInt($(`looper-max-iter-${windowId}`).value, 10) || 10;
            let warning = '';

            if (mode === 'every_step') {
                warning = t('desktop.looper_warn_every_step');
            } else if (mode === 'never' && maxIter > 8) {
                warning = t('desktop.looper_warn_never_long');
            }

            if (warning) {
                contextWarning.textContent = warning;
                contextWarning.style.display = 'block';
            } else {
                contextWarning.style.display = 'none';
            }
        }

        if (contextSelect) {
            contextSelect.addEventListener('change', updateContextWarning);
            $(`looper-max-iter-${windowId}`).addEventListener('input', updateContextWarning);
        }

        function readForm() {
            saveCurrentStepValue();
            const prepTrunc = parseInt($(`looper-prepare-trunc-${windowId}`).value, 10);
            const exitConf = parseFloat($(`looper-exit-conf-${windowId}`).value);
            const stuck = parseInt($(`looper-stuck-${windowId}`).value, 10);
            return {
                prepare: state.stepValues.prepare,
                plan: state.stepValues.plan,
                action: state.stepValues.action,
                test: state.stepValues.test,
                exit_cond: state.stepValues.exit,
                finish: state.stepValues.finish,
                finish_context: $(`looper-finish-context-${windowId}`).value || 'last_test',
                prepare_truncation: Number.isFinite(prepTrunc) && prepTrunc > 0 ? prepTrunc : 0,
                summarize_iterations: !!$(`looper-summarize-${windowId}`).checked,
                exit_min_confidence: Number.isFinite(exitConf) ? exitConf : 0.55,
                stuck_test_repeats: Number.isFinite(stuck) ? stuck : 3,
                provider_id: $(`looper-provider-${windowId}`).value,
                model: $(`looper-model-${windowId}`).value,
                max_iter: parseInt($(`looper-max-iter-${windowId}`).value, 10) || 20,
                context_mode: $(`looper-context-mode-${windowId}`).value || 'every_iteration'
            };
        }

        function validateFormClient(body) {
            const required = [
                ['prepare', body.prepare],
                ['plan', body.plan],
                ['action', body.action],
                ['test', body.test],
                ['exit_cond', body.exit_cond]
            ];
            for (const [name, val] of required) {
                if (!String(val || '').trim()) {
                    return t('desktop.looper_field_required').replace('{{field}}', name);
                }
            }
            return '';
        }

        $(`looper-preset-${windowId}`).addEventListener('change', (e) => {
            const id = e.target.value;
            state.selectedPresetId = id || null;
            if (!id) return;
            const p = state.presets.find(x => String(x.id) === id);
            if (p) fillForm(p);
        });

        $(`looper-save-${windowId}`).addEventListener('click', async () => {
            if (isReadonly) return;
            const body = readForm();
            const errMsg = validateFormClient(body);
            if (errMsg) {
                if (notify) notify({ title: t('desktop.notification'), message: errMsg });
                return;
            }

            const select = $(`looper-preset-${windowId}`);
            const selectedId = select.value;
            const selected = state.presets.find(x => String(x.id) === selectedId);
            const canUpdate = selected && !selected.is_builtin;

            let name = selected && !selected.is_builtin ? selected.name : '';
            let updateId = canUpdate ? selected.id : 0;

            if (canUpdate) {
                const choice = await askConfirm(
                    t('desktop.looper_save'),
                    t('desktop.looper_save_update_confirm').replace('{{name}}', selected.name)
                );
                if (choice) {
                    body.name = selected.name;
                    body.id = selected.id;
                    try {
                        await api('/api/desktop/looper/presets/' + selected.id, {
                            method: 'PUT',
                            headers: { 'Content-Type': 'application/json' },
                            body: JSON.stringify(body)
                        });
                        await loadPresets();
                        select.value = String(selected.id);
                        state.selectedPresetId = String(selected.id);
                        if (notify) notify({ title: t('desktop.looper_title'), message: t('desktop.looper_saved') });
                    } catch (e) {
                        if (notify) notify({ title: t('desktop.notification'), message: t('desktop.looper_save_error') });
                    }
                    return;
                }
                // User declined update → fall through to save-as-new
                updateId = 0;
                name = '';
            }

            name = await askPrompt(t('desktop.looper_save_prompt'), name || '');
            if (!name) return;
            body.name = name;
            if (updateId) body.id = updateId;
            try {
                const res = await api('/api/desktop/looper/presets', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(body)
                });
                await loadPresets();
                if (res && res.id) {
                    select.value = String(res.id);
                    state.selectedPresetId = String(res.id);
                }
                if (notify) notify({ title: t('desktop.looper_title'), message: t('desktop.looper_saved') });
            } catch (e) {
                if (notify) notify({ title: t('desktop.notification'), message: t('desktop.looper_save_error') });
            }
        });

        $(`looper-delete-${windowId}`).addEventListener('click', async () => {
            if (isReadonly) return;
            const select = $(`looper-preset-${windowId}`);
            const id = select.value;
            if (!id) return;
            const p = state.presets.find(x => String(x.id) === id);
            if (p && p.is_builtin) {
                if (notify) notify({ title: t('desktop.notification'), message: t('desktop.looper_delete_error') });
                return;
            }
            const ok = await askConfirm(t('desktop.looper_delete'), t('desktop.looper_delete_confirm'));
            if (!ok) return;
            try {
                await api('/api/desktop/looper/presets/' + id, { method: 'DELETE' });
                await loadPresets();
                select.value = '';
                state.selectedPresetId = null;
                if (notify) notify({ title: t('desktop.looper_title'), message: t('desktop.looper_deleted') });
            } catch (e) {
                if (notify) notify({ title: t('desktop.notification'), message: t('desktop.looper_delete_error') });
            }
        });

        $(`looper-clear-${windowId}`).addEventListener('click', () => {
            $(`looper-logs-${windowId}`).innerHTML = `<div class="vd-looper-log-empty">${esc(t('desktop.looper_no_logs'))}</div>`;
            state.logCount = 0;
            state.logExpandState.clear();
            state.lastLogSignature = '';
        });

        function connectStatus() {
            if (state.sse) { state.sse.close(); state.sse = null; }
            if (state.disposed) return;
            const evtSource = new EventSource('/api/desktop/looper/status');
            state.sse = evtSource;
            evtSource.onmessage = (event) => {
                try { updateStatus(JSON.parse(event.data)); } catch (e) { /* ignore parse */ }
            };
            evtSource.onerror = () => {
                evtSource.close(); state.sse = null;
                if ((state.running || state.paused) && !state.disposed) {
                    setTimeout(() => connectStatus(), 2000);
                }
            };
        }

        // Always connect on open so paused/running state from other windows is visible.
        connectStatus();

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

        function buildLogHtml(log, i, total) {
            const stepMeta = STEP_META.find(s => s.key === log.step) || { color: '#9aa3ad', icon: '•' };
            const stepLabel = t('desktop.looper_step_' + log.step) || log.step;
            const isFirstOfStep = i === 0; // caller may not pass full list; style is optional
            const title = log.iteration > 0 ? `#${log.iteration} ${stepLabel}` : stepLabel;
            const key = logEntryKey(log, i);
            let isCollapsed;
            if (state.logExpandState.has(key)) {
                isCollapsed = !state.logExpandState.get(key);
            } else {
                isCollapsed = i < total - 1;
                state.logExpandState.set(key, !isCollapsed);
            }
            const collapseClass = isCollapsed ? ' vd-looper-log--collapsed' : '';
            const expandTitle = esc(t(isCollapsed ? 'desktop.looper_expand_log' : 'desktop.looper_collapse_log'));
            const hasBody = log.prompt || log.response || log.reason;
            return `<div class="vd-looper-log${isFirstOfStep ? ' vd-looper-log--first' : ''}${collapseClass}" data-log-key="${esc(key)}">
                <div class="vd-looper-log-dot" style="background:${stepMeta.color};box-shadow:0 0 6px ${stepMeta.color}60"></div>
                <div class="vd-looper-log-content">
                    <div class="vd-looper-log-header" title="${expandTitle}" role="button" aria-expanded="${!isCollapsed}" tabindex="0">
                        ${hasBody ? `<span class="vd-looper-log-toggle"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><polyline points="6 9 12 15 18 9"></polyline></svg></span>` : ''}
                        <span class="vd-looper-log-step" style="color:${stepMeta.color}">${esc(title)}</span>
                        <span class="vd-looper-log-time">${formatDuration(log.duration)}</span>
                    </div>
                    ${hasBody ? `<div class="vd-looper-log-body">
                        ${log.reason ? `<div class="vd-looper-log-reason">${esc(log.reason)}</div>` : ''}
                        ${log.prompt ? `<div class="vd-looper-log-prompt">${highlightCode(log.prompt)}</div>` : ''}
                        ${log.response ? `<div class="vd-looper-log-response">${highlightCode(log.response)}</div>` : ''}
                    </div>` : ''}
                </div>
            </div>`;
        }

        function wireLogHeaders(root) {
            root.querySelectorAll('.vd-looper-log-header').forEach(header => {
                if (header.dataset.wired === '1') return;
                header.dataset.wired = '1';
                const toggle = () => {
                    const logEntry = header.closest('.vd-looper-log');
                    if (!logEntry || !logEntry.querySelector('.vd-looper-log-body')) return;
                    logEntry.classList.toggle('vd-looper-log--collapsed');
                    const expanded = !logEntry.classList.contains('vd-looper-log--collapsed');
                    header.setAttribute('aria-expanded', String(expanded));
                    header.setAttribute('title', t(expanded ? 'desktop.looper_collapse_log' : 'desktop.looper_expand_log'));
                    const key = logEntry.getAttribute('data-log-key');
                    if (key) state.logExpandState.set(key, expanded);
                };
                header.addEventListener('click', toggle);
                header.addEventListener('keydown', (e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        toggle();
                    }
                });
            });
        }

        function renderLogs(logs) {
            if (!logsEl) return;
            if (!logs || !logs.length) {
                if (state.lastLogSignature !== 'empty') {
                    logsEl.innerHTML = `<div class="vd-looper-log-empty">${esc(t('desktop.looper_no_logs'))}</div>`;
                    state.lastLogSignature = 'empty';
                    state.logCount = 0;
                }
                return;
            }

            const signature = logs.map((l, i) => logEntryKey(l, i)).join(';');
            if (signature === state.lastLogSignature) {
                // Same entries — only ensure last is expanded if user hasn't toggled
                return;
            }

            // Incremental append when only new entries were added
            const prevCount = state.logCount;
            if (state.lastLogSignature && signature.startsWith(state.lastLogSignature.replace(/;?$/, '')) === false) {
                // Fallback full rebuild when history was trimmed (max 200) or reset
            }
            const canAppend = prevCount > 0 &&
                logs.length > prevCount &&
                logs.slice(0, prevCount).every((l, i) => {
                    const existing = logsEl.querySelector(`[data-log-key="${CSS.escape(logEntryKey(l, i))}"]`);
                    return !!existing;
                });

            if (canAppend) {
                // Collapse previous last entry if user hasn't expanded it explicitly
                const prevLast = logsEl.querySelector('.vd-looper-log:last-child');
                if (prevLast) {
                    const key = prevLast.getAttribute('data-log-key');
                    if (key && !state.logExpandState.has(key)) {
                        prevLast.classList.add('vd-looper-log--collapsed');
                        const h = prevLast.querySelector('.vd-looper-log-header');
                        if (h) {
                            h.setAttribute('aria-expanded', 'false');
                            h.setAttribute('title', t('desktop.looper_expand_log'));
                        }
                    }
                }
                const frag = document.createDocumentFragment();
                const wrap = document.createElement('div');
                for (let i = prevCount; i < logs.length; i++) {
                    wrap.innerHTML = buildLogHtml(logs[i], i, logs.length);
                    while (wrap.firstChild) frag.appendChild(wrap.firstChild);
                }
                // Remove empty placeholder if present
                const empty = logsEl.querySelector('.vd-looper-log-empty');
                if (empty) empty.remove();
                logsEl.appendChild(frag);
                wireLogHeaders(logsEl);
            } else {
                const html = logs.map((log, i) => buildLogHtml(log, i, logs.length)).join('');
                logsEl.innerHTML = html;
                wireLogHeaders(logsEl);
            }

            state.logCount = logs.length;
            state.lastLogSignature = signature;

            if (state.autoScroll) {
                logsEl.scrollTop = logsEl.scrollHeight;
            }
            if (jumpBottomEl) {
                jumpBottomEl.classList.toggle('vd-looper-jump-bottom--visible', !state.autoScroll && state.logCount > 0);
            }
        }

        function updateStatus(data) {
            const wasRunning = state.running;
            state.status = data;
            const monitor = $(`looper-monitor-${windowId}`);
            if (!monitor) return;
            const titleEl = monitor.querySelector('.vd-looper-monitor-title span');
            const metaEl = $(`looper-meta-${windowId}`);
            const progressEl = $(`looper-progress-${windowId}`);
            const glowEl = $(`looper-progress-glow-${windowId}`);
            const startBtn = $(`looper-start-${windowId}`);
            const stopBtn = $(`looper-stop-${windowId}`);
            const pauseBtn = $(`looper-pause-${windowId}`);
            const resumeBtn = $(`looper-resume-${windowId}`);
            const monitorStopBtn = $(`looper-monitor-stop-${windowId}`);
            const monitorPauseBtn = $(`looper-monitor-pause-${windowId}`);
            const monitorResumeBtn = $(`looper-monitor-resume-${windowId}`);

            state.running = !!data.running;
            state.paused = !!data.paused || data.current_step === 'paused';

            if (!isReadonly) {
                startBtn.disabled = data.running || state.paused;
            }
            stopBtn.disabled = !data.running;
            if (pauseBtn) {
                pauseBtn.disabled = !data.running || state.paused;
            }
            if (resumeBtn) {
                resumeBtn.style.display = state.paused && !data.running ? 'inline-flex' : 'none';
                resumeBtn.disabled = isReadonly || !state.paused || data.running;
            }
            if (monitorStopBtn) {
                monitorStopBtn.style.display = data.running ? 'inline-flex' : 'none';
            }
            if (monitorPauseBtn) {
                monitorPauseBtn.style.display = data.running && !state.paused ? 'inline-flex' : 'none';
            }
            if (monitorResumeBtn) {
                monitorResumeBtn.style.display = state.paused && !data.running ? 'inline-flex' : 'none';
            }

            monitor.classList.toggle('vd-looper-monitor--running', data.running);
            monitor.classList.toggle('vd-looper-monitor--error', !!data.error && !data.stopped && !data.stuck_detected);
            monitor.classList.toggle('vd-looper-monitor--stopped', !!data.stopped || data.current_step === 'stopped');
            monitor.classList.toggle('vd-looper-monitor--paused', state.paused);
            monitor.classList.toggle('vd-looper-monitor--stuck', !!data.stuck_detected || data.current_step === 'stuck');

            if (wasRunning && !data.running && !state.paused) {
                state.collapsed = false;
                updateToggleState(false);
                setWorkspaceRunning(false);
            }
            if (!wasRunning && data.running) {
                setWorkspaceRunning(true);
            }

            if (data.stopped || data.current_step === 'stopped') {
                titleEl.textContent = t('desktop.looper_status_stopped');
            } else if (data.stuck_detected || data.current_step === 'stuck') {
                titleEl.textContent = t('desktop.looper_status_stuck') + (data.error ? ': ' + data.error : '');
            } else if (state.paused) {
                titleEl.textContent = t('desktop.looper_status_paused')
                    .replace('{{n}}', data.resume_from || data.iteration || 0);
            } else if (data.error) {
                titleEl.textContent = t('desktop.looper_error') + ': ' + data.error;
            } else if (data.running) {
                const stepLabel = t('desktop.looper_step_' + data.current_step) || data.current_step;
                if (data.current_step === 'prepare' || data.current_step === 'finish' || data.current_step === 'summarize') {
                    titleEl.textContent = stepLabel;
                } else {
                    titleEl.textContent = t('desktop.looper_iteration')
                        .replace('{{n}}', data.iteration)
                        .replace('{{max}}', data.max_iterations) + ' — ' + stepLabel;
                }
            } else {
                titleEl.textContent = data.current_step === 'idle' || !data.current_step
                    ? t('desktop.looper_status_idle')
                    : (t('desktop.looper_step_' + data.current_step) || data.current_step);
            }

            const pct = data.max_iterations > 0 ? (data.iteration / data.max_iterations) * 100 : 0;
            progressEl.style.width = Math.min(pct, 100) + '%';
            glowEl.style.left = Math.min(pct, 100) + '%';

            const metaParts = [];
            if (data.running && data.iteration > 0 && data.max_iterations > 0) {
                metaParts.push(`${data.iteration} / ${data.max_iterations}`);
            } else if (!data.running && state.logCount > 0) {
                metaParts.push(`${state.logCount} ${t('desktop.looper_logs_title').toLowerCase()}`);
            }
            if (data.input_tokens || data.output_tokens) {
                metaParts.push(`${(data.input_tokens || 0) + (data.output_tokens || 0)} tok`);
            }
            const cost = formatCost(data.estimated_cost_usd);
            if (cost) {
                metaParts.push(cost);
            }
            metaEl.textContent = metaParts.join(' · ');

            $$(`.vd-looper-step-btn`).forEach(btn => {
                btn.classList.toggle('vd-looper-step-btn--running', data.running && btn.dataset.step === data.current_step);
            });

            if (data.logs) {
                renderLogs(data.logs);
            }

            // Keep SSE while running or paused; close only on terminal idle/stopped/stuck.
            if (!data.running && !state.paused &&
                (data.current_step === 'idle' || data.current_step === 'stopped' || data.current_step === 'stuck') &&
                state.sse) {
                // Leave stream open briefly so final frames arrive; onerror/idle handler closes.
                // Do not force-close immediately — status handler server closes after 3 idle ticks.
            }
        }

        $(`looper-start-${windowId}`).addEventListener('click', async () => {
            if (isReadonly) return;
            const body = readForm();
            const errMsg = validateFormClient(body);
            if (errMsg) {
                if (notify) notify({ title: t('desktop.notification'), message: errMsg });
                return;
            }
            try {
                await api('/api/desktop/looper/run', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(body)
                });
                $(`looper-logs-${windowId}`).innerHTML = '';
                state.logCount = 0;
                state.logExpandState.clear();
                state.lastLogSignature = '';
                state.startTime = Date.now();
                state.autoScroll = true;
                if (jumpBottomEl) jumpBottomEl.classList.remove('vd-looper-jump-bottom--visible');
                connectStatus();
                setWorkspaceRunning(true);
            } catch (e) {
                if (e && e.status === 409) {
                    if (notify) notify({ title: t('desktop.notification'), message: t('desktop.looper_already_running') });
                    connectStatus();
                } else {
                    const msg = e && e.message ? e.message : t('desktop.looper_start_error');
                    if (notify) notify({ title: t('desktop.notification'), message: msg });
                }
            }
        });

        $(`looper-stop-${windowId}`).addEventListener('click', stopLoop);
        if ($(`looper-pause-${windowId}`)) {
            $(`looper-pause-${windowId}`).addEventListener('click', pauseLoop);
        }
        if ($(`looper-resume-${windowId}`)) {
            $(`looper-resume-${windowId}`).addEventListener('click', resumeLoop);
        }
    }

    function dispose(windowId) {
        const state = instances.get(windowId);
        if (!state) return;
        state.disposed = true;
        if (state.sse) { state.sse.close(); state.sse = null; }
        instances.delete(windowId);
    }

    window.LooperApp = { render, dispose };
})();
