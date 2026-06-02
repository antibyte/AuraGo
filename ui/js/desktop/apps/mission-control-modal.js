// Mission Control – modal subsystem
// Extracted from mission-control.js to stay within line-budget limits.
// Loaded by module-loader.js before mission-control.js.
(function () {
    'use strict';

    /**
     * createModal – factory that binds the modal to its render-closure dependencies.
     * @param {{P,SVG,TRIGGER_TYPES,CRON_PRESETS,esc,t,api,notify,state,escAttr,loadData,container}} deps
     * @returns {{ openModal: function, showInfoModal: function }}
     */
    function createModal({ P, SVG, TRIGGER_TYPES, CRON_PRESETS, esc, t, api, notify, state, escAttr, loadData, container }) {

        function openModal(editId, duplicateSource) {
            state.editingId = editId;
            const isEdit = !!editId;
            const mission = isEdit ? state.missions.find(m => m.id === editId) : null;
            const source = duplicateSource || mission;

            const overlay = document.createElement('div');
            overlay.className = P + '-modal-overlay';
            overlay.innerHTML = buildModalForm(source, isEdit);
            container.appendChild(overlay);

            const closeModalFn = () => { if (overlay.parentNode) overlay.parentNode.removeChild(overlay); };
            overlay.addEventListener('click', (e) => { if (e.target === overlay) closeModalFn(); });
            overlay.querySelector('[data-mc="modal-close"]').addEventListener('click', closeModalFn);
            overlay.querySelector('[data-mc="modal-cancel"]').addEventListener('click', closeModalFn);
            overlay.querySelector('[data-mc="modal-save"]').addEventListener('click', () => saveMission(overlay, closeModalFn));

            bindModalEvents(overlay, source);

            if (source) {
                fillFormFromMission(overlay, source);
            }
        }

        function buildModalForm(source, isEdit) {
            const title = isEdit ? t('missions.modal_title_edit', 'Edit Mission') : t('missions.modal_title_new', 'New Mission');
            return `
                <div class="${P}-modal">
                    <div class="${P}-modal-header">
                        <h2 class="${P}-modal-title">${esc(title)}</h2>
                        <button type="button" class="${P}-modal-close" data-mc="modal-close">${SVG.x}</button>
                    </div>
                    <div class="${P}-modal-body">
                        <input type="hidden" data-mc="form-id" value="${escAttr(source ? source.id : '')}">

                        <div class="${P}-form-group">
                            <label>${esc(t('missions.form_name_label', 'Name'))}</label>
                            <input type="text" class="${P}-form-input" data-mc="form-name" placeholder="${esc(t('missions.form_name_placeholder', 'e.g. Daily Report'))}" value="${escAttr(source ? source.name : '')}">
                        </div>

                        <div class="${P}-form-group">
                            <label>${esc(t('missions.form_prompt_label', 'Prompt'))}</label>
                            <textarea class="${P}-form-textarea" data-mc="form-prompt" placeholder="${esc(t('missions.form_prompt_placeholder', 'Describe the task...'))}">${esc(source ? source.prompt : '')}</textarea>
                        </div>

                        <div class="${P}-form-group">
                            <label>${esc(t('missions.form_runner_label', 'Run Location'))}</label>
                            <div class="${P}-runner-selector">
                                <label class="${P}-runner-option${!source || source.runner_type !== 'remote' ? ' active' : ''}" data-mc-runner="local"><input type="radio" name="mc-runner" value="local" ${!source || source.runner_type !== 'remote' ? 'checked' : ''}><span class="${P}-runner-label">${esc(t('missions.form_runner_local_label', 'Local'))}</span><span class="${P}-runner-desc">${esc(t('missions.form_runner_local_desc', 'Run on this instance'))}</span></label>
                                <label class="${P}-runner-option${source && source.runner_type === 'remote' ? ' active' : ''}" data-mc-runner="remote"><input type="radio" name="mc-runner" value="remote" ${source && source.runner_type === 'remote' ? 'checked' : ''}><span class="${P}-runner-label">${esc(t('missions.form_runner_remote_label', 'Remote Egg'))}</span><span class="${P}-runner-desc">${esc(t('missions.form_runner_remote_desc', 'Run on Invasion egg'))}</span></label>
                            </div>
                            <div class="${P}-form-group" data-mc="remote-target-group" style="display:${source && source.runner_type === 'remote' ? '' : 'none'}">
                                <label>${esc(t('missions.form_remote_target_label', 'Remote Egg'))}</label>
                                <select class="${P}-form-select" data-mc="form-remote-target"><option value="">${esc(t('missions.form_remote_target_loading', 'Loading...'))}</option></select>
                                <div class="${P}-form-hint">${esc(t('missions.form_remote_target_hint', 'Only connected eggs.'))}</div>
                            </div>
                        </div>

                        <div class="${P}-form-group">
                            <label>${esc(t('missions.form_priority_label', 'Priority'))}</label>
                            <select class="${P}-form-select" data-mc="form-priority">
                                <option value="low">${esc(t('missions.form_priority_low', 'Low'))}</option>
                                <option value="medium" selected>${esc(t('missions.form_priority_medium', 'Medium'))}</option>
                                <option value="high">${esc(t('missions.form_priority_high', 'High'))}</option>
                            </select>
                        </div>

                        <div class="${P}-form-group">
                            <label>${esc(t('missions.form_exec_type_label', 'Execution Type'))}</label>
                            <div class="${P}-exec-selector">
                                <label class="${P}-exec-option active" data-mc-exec="manual"><input type="radio" name="mc-exec" value="manual" checked><span class="${P}-exec-icon">👆</span><span class="${P}-exec-label">${esc(t('missions.form_exec_manual_label', 'Manual'))}</span><span class="${P}-exec-desc">${esc(t('missions.form_exec_manual_desc', 'On demand'))}</span></label>
                                <label class="${P}-exec-option" data-mc-exec="scheduled"><input type="radio" name="mc-exec" value="scheduled"><span class="${P}-exec-icon">📅</span><span class="${P}-exec-label">${esc(t('missions.form_exec_scheduled_label', 'Scheduled'))}</span><span class="${P}-exec-desc">${esc(t('missions.form_exec_scheduled_desc', 'Cron'))}</span></label>
                                <label class="${P}-exec-option" data-mc-exec="triggered"><input type="radio" name="mc-exec" value="triggered"><span class="${P}-exec-icon">⚡</span><span class="${P}-exec-label">${esc(t('missions.form_exec_triggered_label', 'Triggered'))}</span><span class="${P}-exec-desc">${esc(t('missions.form_exec_triggered_desc', 'Event'))}</span></label>
                            </div>
                        </div>

                        <div class="${P}-form-group" data-mc="config-scheduled" style="display:none">
                            <label>${esc(t('missions.form_cron_preset_label', 'Presets'))}</label>
                            <select class="${P}-form-select" data-mc="cron-preset">${CRON_PRESETS.map(p => `<option value="${escAttr(p.value)}">${esc(t(p.labelKey, p.value || '-- Custom --'))}</option>`).join('')}</select>
                            <div style="margin-top:6px">
                                <label>${esc(t('missions.form_cron_label', 'Cron Expression'))}</label>
                                <input type="text" class="${P}-form-input" data-mc="form-cron" placeholder="${esc(t('missions.form_cron_placeholder', '0 9 * * *'))}">
                                <div class="${P}-form-hint">${esc(t('missions.form_cron_hint', 'Format: Min Hour Day Month Weekday'))}</div>
                            </div>
                        </div>

                        <div class="${P}-form-group" data-mc="config-triggered" style="display:none">
                            <label>${esc(t('missions.form_exec_triggered_label', 'Trigger Type'))}</label>
                            <div class="${P}-trigger-grid">${TRIGGER_TYPES.map(tr => `<button type="button" class="${P}-trigger-btn" data-mc-trigger="${tr.key}">${tr.icon} ${esc(t(tr.labelKey, tr.key))}</button>`).join('')}</div>
                            <div class="${P}-form-group">
                                <label>${esc(t('missions.trigger_min_interval_label', 'Min interval'))}</label>
                                <input type="number" class="${P}-form-input" data-mc="form-min-interval" min="0" max="86400" value="0" placeholder="${esc(t('missions.trigger_min_interval_placeholder', 'e.g. 60'))}">
                                <div class="${P}-form-hint">${esc(t('missions.trigger_min_interval_hint', 'Seconds before re-trigger.'))}</div>
                            </div>
                            ${buildTriggerFields()}
                        </div>

                        <div class="${P}-toggle-row">
                            <label class="${P}-toggle"><input type="checkbox" data-mc="form-locked"><span class="${P}-toggle-slider"></span></label>
                            <div><div class="${P}-toggle-text">${esc(t('missions.form_lock_label', 'Lock'))}</div><div class="${P}-toggle-hint">${esc(t('missions.form_lock_hint', 'Prevents deletion'))}</div></div>
                        </div>
                        <div class="${P}-toggle-row">
                            <label class="${P}-toggle"><input type="checkbox" data-mc="form-auto-prepare"><span class="${P}-toggle-slider"></span></label>
                            <div><div class="${P}-toggle-text">${esc(t('missions.prep_auto', 'Auto-prepare'))}</div><div class="${P}-toggle-hint">${esc(t('missions.prep_auto_hint', 'Prepare before runs'))}</div></div>
                        </div>

                        <div class="${P}-form-group">
                            <label>${esc(t('missions.form_cheatsheets_label', 'Cheat Sheets'))}</label>
                            <div class="${P}-cheatsheet-picker" data-mc="cheatsheet-picker"><div class="${P}-cheatsheet-empty">${esc(t('missions.form_cheatsheets_loading', 'Loading...'))}</div></div>
                            <div class="${P}-form-hint">${esc(t('missions.form_cheatsheets_hint', 'Include as context.'))}</div>
                        </div>
                    </div>
                    <div class="${P}-modal-actions">
                        <button type="button" class="${P}-btn" data-mc="modal-cancel">${esc(t('missions.modal_btn_cancel', 'Cancel'))}</button>
                        <button type="button" class="${P}-btn ${P}-btn-primary" data-mc="modal-save">${esc(t('missions.modal_btn_save', 'Save'))}</button>
                    </div>
                </div>`;
        }

        function buildTriggerFields() {
            return `
                <div class="${P}-trigger-fields" data-mc-trigger-fields="mission_completed">
                    <div class="${P}-form-group"><label>${esc(t('missions.trigger_source_mission_label', 'Source Mission'))}</label><div class="${P}-mission-selector" data-mc="mission-selector"></div></div>
                    <div class="${P}-toggle-row"><label class="${P}-toggle"><input type="checkbox" data-mc="form-require-success"><span class="${P}-toggle-slider"></span></label><span class="${P}-toggle-text">${esc(t('missions.trigger_require_success', 'Only on success'))}</span></div>
                </div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="email_received">
                    <div class="${P}-form-row"><div class="${P}-form-group"><label>${esc(t('missions.trigger_email_folder_label', 'Folder'))}</label><select class="${P}-form-select" data-mc="form-email-folder"><option value="INBOX">${esc(t('missions.trigger_email_folder_inbox', 'Inbox'))}</option><option value="Sent">${esc(t('missions.trigger_email_folder_sent', 'Sent'))}</option></select></div><div class="${P}-form-group"><label>${esc(t('missions.trigger_email_subject_label', 'Subject'))}</label><input type="text" class="${P}-form-input" data-mc="form-email-subject" placeholder="${esc(t('missions.trigger_email_subject_placeholder', 'Order'))}"></div></div>
                    <div class="${P}-form-group"><label>${esc(t('missions.trigger_email_from_label', 'From'))}</label><input type="text" class="${P}-form-input" data-mc="form-email-from" placeholder="${esc(t('missions.trigger_email_from_placeholder', '@company.com'))}"></div>
                </div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="webhook">
                    <div class="${P}-form-group"><label>${esc(t('missions.trigger_webhook_label', 'Webhook'))}</label><select class="${P}-form-select" data-mc="form-webhook"><option value="">${esc(t('missions.trigger_webhook_loading', 'Loading...'))}</option></select><div class="${P}-form-hint">${esc(t('missions.trigger_webhook_hint', 'Choose webhook.'))}</div></div>
                </div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="egg_hatched">
                    <div class="${P}-form-hint">${esc(t('missions.trigger_egg_hatched_hint', 'When an egg hatches.'))}</div>
                    <div class="${P}-form-row"><div class="${P}-form-group"><label>${esc(t('missions.trigger_egg_select_label', 'Egg'))}</label><select class="${P}-form-select" data-mc="form-egg"><option value="">${esc(t('missions.trigger_egg_any', 'Any'))}</option></select></div><div class="${P}-form-group"><label>${esc(t('missions.trigger_nest_select_label', 'Nest'))}</label><select class="${P}-form-select" data-mc="form-egg-nest"><option value="">${esc(t('missions.trigger_nest_any', 'Any'))}</option></select></div></div>
                </div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="nest_cleared">
                    <div class="${P}-form-hint">${esc(t('missions.trigger_nest_cleared_hint', 'When a nest is cleared.'))}</div>
                    <div class="${P}-form-group"><label>${esc(t('missions.trigger_nest_select_label', 'Nest'))}</label><select class="${P}-form-select" data-mc="form-nest"><option value="">${esc(t('missions.trigger_nest_any', 'Any'))}</option></select></div>
                </div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="mqtt_message">
                    <div class="${P}-form-hint">${esc(t('missions.trigger_mqtt_hint', 'MQTT messages.'))}</div>
                    <div class="${P}-form-group"><label>${esc(t('missions.trigger_mqtt_topic_label', 'Topic'))}</label><input type="text" class="${P}-form-input" data-mc="form-mqtt-topic" placeholder="${esc(t('missions.trigger_mqtt_topic_placeholder', 'home/sensors/#'))}"></div>
                    <div class="${P}-form-row"><div class="${P}-form-group"><label>${esc(t('missions.trigger_mqtt_payload_label', 'Payload'))}</label><input type="text" class="${P}-form-input" data-mc="form-mqtt-payload" placeholder="${esc(t('missions.trigger_mqtt_payload_placeholder', 'alarm'))}"></div><div class="${P}-form-group"><label>${esc(t('missions.trigger_mqtt_min_interval_label', 'Min interval'))}</label><input type="number" class="${P}-form-input" data-mc="form-mqtt-interval" min="0" value="0"></div></div>
                </div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="system_startup"><div class="${P}-form-hint">${esc(t('missions.trigger_system_startup_hint', 'Runs on startup.'))}</div></div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="home_assistant_state">
                    <div class="${P}-form-hint">${esc(t('missions.trigger_home_assistant_state_hint', 'HA entity state change.'))}</div>
                    <div class="${P}-form-row"><div class="${P}-form-group"><label>${esc(t('missions.trigger_ha_entity_id_label', 'Entity ID'))}</label><input type="text" class="${P}-form-input" data-mc="form-ha-entity" placeholder="${esc(t('missions.trigger_ha_entity_id_placeholder', 'binary_sensor.door'))}"></div><div class="${P}-form-group"><label>${esc(t('missions.trigger_ha_state_equals_label', 'State equals'))}</label><input type="text" class="${P}-form-input" data-mc="form-ha-state" placeholder="${esc(t('missions.trigger_ha_state_equals_placeholder', 'on'))}"></div></div>
                </div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="device_connected">
                    <div class="${P}-form-hint">${esc(t('missions.trigger_device_connected_hint', 'Device connects.'))}</div>
                    <div class="${P}-form-row"><div class="${P}-form-group"><label>${esc(t('missions.trigger_device_id_label', 'Device ID'))}</label><input type="text" class="${P}-form-input" data-mc="form-device-conn-id"></div><div class="${P}-form-group"><label>${esc(t('missions.trigger_device_name_label', 'Name'))}</label><input type="text" class="${P}-form-input" data-mc="form-device-conn-name"></div></div>
                </div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="device_disconnected">
                    <div class="${P}-form-hint">${esc(t('missions.trigger_device_disconnected_hint', 'Device disconnects.'))}</div>
                    <div class="${P}-form-row"><div class="${P}-form-group"><label>${esc(t('missions.trigger_device_id_label', 'Device ID'))}</label><input type="text" class="${P}-form-input" data-mc="form-device-disc-id"></div><div class="${P}-form-group"><label>${esc(t('missions.trigger_device_name_label', 'Name'))}</label><input type="text" class="${P}-form-input" data-mc="form-device-disc-name"></div></div>
                </div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="fritzbox_call">
                    <div class="${P}-form-hint">${esc(t('missions.trigger_fritzbox_call_hint', 'Fritz!Box calls.'))}</div>
                    <div class="${P}-form-group"><label>${esc(t('missions.trigger_fritzbox_call_type_label', 'Type'))}</label><select class="${P}-form-select" data-mc="form-fritzbox-type"><option value="">${esc(t('missions.trigger_fritzbox_call_type_any', 'Any'))}</option><option value="call">${esc(t('missions.trigger_fritzbox_call_type_call', 'Call'))}</option><option value="tam_message">${esc(t('missions.trigger_fritzbox_call_type_tam', 'TAM'))}</option></select></div>
                </div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="budget_warning"><div class="${P}-form-hint">${esc(t('missions.trigger_budget_warning_hint', 'Budget warning.'))}</div></div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="budget_exceeded"><div class="${P}-form-hint">${esc(t('missions.trigger_budget_exceeded_hint', 'Budget exceeded.'))}</div></div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="planner_appointment_due">
                    <div class="${P}-form-hint">${esc(t('missions.trigger_planner_appointment_due_hint', 'Appointment due.'))}</div>
                    <div class="${P}-form-row"><div class="${P}-form-group"><label>${esc(t('missions.trigger_planner_appointment_id_label', 'ID'))}</label><input type="text" class="${P}-form-input" data-mc="form-planner-appt-id"></div><div class="${P}-form-group"><label>${esc(t('missions.trigger_planner_title_contains_label', 'Title'))}</label><input type="text" class="${P}-form-input" data-mc="form-planner-appt-title"></div></div>
                </div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="planner_todo_overdue">
                    <div class="${P}-form-hint">${esc(t('missions.trigger_planner_todo_overdue_hint', 'Todo overdue.'))}</div>
                    <div class="${P}-form-row"><div class="${P}-form-group"><label>${esc(t('missions.trigger_planner_todo_id_label', 'ID'))}</label><input type="text" class="${P}-form-input" data-mc="form-planner-todo-id"></div><div class="${P}-form-group"><label>${esc(t('missions.trigger_planner_title_contains_label', 'Title'))}</label><input type="text" class="${P}-form-input" data-mc="form-planner-todo-title"></div></div>
                </div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="planner_operational_issue">
                    <div class="${P}-form-hint">${esc(t('missions.trigger_planner_operational_issue_hint', 'Operational issue.'))}</div>
                    <div class="${P}-form-row"><div class="${P}-form-group"><label>${esc(t('missions.trigger_planner_issue_source_label', 'Source'))}</label><input type="text" class="${P}-form-input" data-mc="form-planner-issue-source" placeholder="${esc(t('missions.trigger_planner_issue_source_placeholder', 'mission'))}"></div><div class="${P}-form-group"><label>${esc(t('missions.trigger_planner_issue_severity_label', 'Severity'))}</label><select class="${P}-form-select" data-mc="form-planner-issue-severity"><option value="">${esc(t('missions.trigger_planner_issue_severity_any', 'Any'))}</option><option value="warning">${esc(t('missions.trigger_planner_issue_severity_warning', 'Warning'))}</option><option value="error">${esc(t('missions.trigger_planner_issue_severity_error', 'Error'))}</option></select></div></div>
                    <div class="${P}-form-group"><label>${esc(t('missions.trigger_planner_title_contains_label', 'Title'))}</label><input type="text" class="${P}-form-input" data-mc="form-planner-issue-title"></div>
                </div>
            `;
        }

        function bindModalEvents(overlay, source) {
            // Exec type
            overlay.querySelectorAll('[data-mc-exec]').forEach(el => {
                el.addEventListener('click', () => {
                    overlay.querySelectorAll('[data-mc-exec]').forEach(e => e.classList.remove('active'));
                    el.classList.add('active');
                    el.querySelector('input').checked = true;
                    const v = el.dataset.mcExec;
                    const schedEl = overlay.querySelector('[data-mc="config-scheduled"]');
                    const trigEl = overlay.querySelector('[data-mc="config-triggered"]');
                    if (schedEl) schedEl.style.display = v === 'scheduled' ? '' : 'none';
                    if (trigEl) trigEl.style.display = v === 'triggered' ? '' : 'none';
                });
            });

            // Runner type
            overlay.querySelectorAll('[data-mc-runner]').forEach(el => {
                el.addEventListener('click', () => {
                    overlay.querySelectorAll('[data-mc-runner]').forEach(e => e.classList.remove('active'));
                    el.classList.add('active');
                    el.querySelector('input').checked = true;
                    const rg = overlay.querySelector('[data-mc="remote-target-group"]');
                    if (rg) rg.style.display = el.dataset.mcRunner === 'remote' ? '' : 'none';
                });
            });

            // Cron preset
            const cronPreset = overlay.querySelector('[data-mc="cron-preset"]');
            const cronInput = overlay.querySelector('[data-mc="form-cron"]');
            if (cronPreset && cronInput) {
                cronPreset.addEventListener('change', () => { if (cronPreset.value) cronInput.value = cronPreset.value; });
            }

            // Trigger type
            overlay.querySelectorAll('[data-mc-trigger]').forEach(btn => {
                btn.addEventListener('click', () => {
                    overlay.querySelectorAll('[data-mc-trigger]').forEach(b => b.classList.remove('active'));
                    btn.classList.add('active');
                    overlay.querySelectorAll('[data-mc-trigger-fields]').forEach(f => f.classList.remove('active'));
                    const panel = overlay.querySelector(`[data-mc-trigger-fields="${btn.dataset.mcTrigger}"]`);
                    if (panel) panel.classList.add('active');
                });
            });

            // Load data for selectors
            loadWebhooksForModal(overlay);
            loadInvasionDataForModal(overlay);
            loadRemoteTargetsForModal(overlay, source);
            loadCheatsheetsForModal(overlay, source ? (source.cheatsheet_ids || []) : []);
            loadMissionSelectorForModal(overlay);
        }

        async function loadWebhooksForModal(overlay) {
            try {
                const data = await api('/api/webhooks');
                state.webhooks = Array.isArray(data) ? data : [];
                const sel = overlay.querySelector('[data-mc="form-webhook"]');
                if (sel) {
                    sel.innerHTML = state.webhooks.length === 0
                        ? '<option value="">' + esc(t('missions.trigger_webhook_none', 'None')) + '</option>'
                        : state.webhooks.map(w => '<option value="' + escAttr(w.id) + '" data-slug="' + escAttr(w.slug) + '">' + esc(w.name) + ' (' + esc(w.slug) + ')</option>').join('');
                }
            } catch (_) { /* webhooks unavailable */ }
        }

        async function loadInvasionDataForModal(overlay) {
            try {
                const [eggsResp, nestsResp] = await Promise.all([api('/api/invasion/eggs').catch(() => null), api('/api/invasion/nests').catch(() => null)]);
                const eggs = eggsResp ? (eggsResp.eggs || eggsResp || []) : [];
                const nests = nestsResp ? (nestsResp.nests || nestsResp || []) : [];
                const eggOpts = '<option value="">' + esc(t('missions.trigger_egg_any', 'Any')) + '</option>' + eggs.map(e => '<option value="' + escAttr(e.id) + '" data-name="' + escAttr(e.name) + '">' + esc(e.name) + '</option>').join('');
                const nestOpts = '<option value="">' + esc(t('missions.trigger_nest_any', 'Any')) + '</option>' + nests.map(n => '<option value="' + escAttr(n.id) + '" data-name="' + escAttr(n.name) + '">' + esc(n.name) + '</option>').join('');
                const eggSel = overlay.querySelector('[data-mc="form-egg"]');
                const eggNestSel = overlay.querySelector('[data-mc="form-egg-nest"]');
                const nestSel = overlay.querySelector('[data-mc="form-nest"]');
                if (eggSel) eggSel.innerHTML = eggOpts;
                if (eggNestSel) eggNestSel.innerHTML = nestOpts;
                if (nestSel) nestSel.innerHTML = nestOpts;
            } catch (_) { /* invasion not available */ }
        }

        async function loadRemoteTargetsForModal(overlay, source) {
            const sel = overlay.querySelector('[data-mc="form-remote-target"]');
            if (!sel) return;
            try {
                const data = await api('/api/missions/v2/remote-targets');
                state.remoteTargets = data.targets || [];
                if (state.remoteTargets.length === 0) { sel.innerHTML = '<option value="">' + esc(t('missions.form_remote_target_none', 'None')) + '</option>'; return; }
                sel.innerHTML = '<option value="">' + esc(t('missions.form_remote_target_placeholder', 'Select...')) + '</option>' + state.remoteTargets.map(tgt => '<option value="' + escAttr(tgt.nest_id) + '" data-egg-id="' + escAttr(tgt.egg_id) + '" data-nest-name="' + escAttr(tgt.nest_name || '') + '" data-egg-name="' + escAttr(tgt.egg_name || '') + '">' + esc((tgt.nest_name || tgt.nest_id) + ' · ' + (tgt.egg_name || tgt.egg_id)) + '</option>').join('');
                if (source && source.remote_nest_id) sel.value = source.remote_nest_id;
            } catch (_) { sel.innerHTML = '<option value="">' + esc(t('missions.form_remote_target_unavailable', 'Unavailable')) + '</option>'; }
        }

        async function loadCheatsheetsForModal(overlay, selectedIds) {
            const picker = overlay.querySelector('[data-mc="cheatsheet-picker"]');
            if (!picker) return;
            try {
                const sheets = await api('/api/cheatsheets?active=true&created_by=user');
                if (!sheets || sheets.length === 0) { picker.innerHTML = '<div class="' + P + '-cheatsheet-empty">' + esc(t('missions.form_cheatsheets_none', 'None')) + '</div>'; return; }
                picker.innerHTML = sheets.map(s => {
                    const checked = selectedIds.includes(s.id) ? 'checked' : '';
                    const abstract = s.abstract ? '<div class="' + P + '-cheatsheet-preview">' + esc(s.abstract) + '</div>' : '';
                    return '<div class="' + P + '-cheatsheet-item"><input type="checkbox" id="mc-cs-' + s.id + '" value="' + s.id + '" ' + checked + '><label for="mc-cs-' + s.id + '">' + esc(s.name) + abstract + '</label></div>';
                }).join('');
            } catch (_) { picker.innerHTML = '<div class="' + P + '-cheatsheet-empty">' + esc(t('missions.form_cheatsheets_none', 'None')) + '</div>'; }
        }

        async function loadMissionSelectorForModal(overlay) {
            const el = overlay.querySelector('[data-mc="mission-selector"]');
            if (!el) return;
            const manual = state.missions.filter(m => m.execution_type === 'manual' || m.execution_type === 'scheduled');
            if (manual.length === 0) { el.innerHTML = '<div class="' + P + '-cheatsheet-empty">' + esc(t('missions.trigger_no_suitable_missions', 'None')) + '</div>'; return; }
            el.innerHTML = manual.map(m => '<label class="' + P + '-mission-option"><input type="radio" name="mc-source-mission" value="' + m.id + '" data-name="' + escAttr(m.name) + '"><div><div class="' + P + '-mission-option-name">' + esc(m.name) + '</div><div class="' + P + '-mission-option-meta">' + m.execution_type + ' · ' + m.priority + '</div></div></label>').join('');
        }

        function fillFormFromMission(overlay, mission) {
            const q = (sel) => overlay.querySelector(sel);
            const v = (sel, val) => { const el = q(sel); if (el) el.value = val || ''; };
            const c = (sel, val) => { const el = q(sel); if (el) el.checked = !!val; };

            v('[data-mc="form-priority"]', mission.priority);
            c('[data-mc="form-locked"]', mission.locked);
            c('[data-mc="form-auto-prepare"]', mission.auto_prepare);

            // Exec type
            const execBtn = overlay.querySelector(`[data-mc-exec="${mission.execution_type}"]`);
            if (execBtn) execBtn.click();

            if (mission.execution_type === 'scheduled') {
                v('[data-mc="form-cron"]', mission.schedule);
                const preset = overlay.querySelector('[data-mc="cron-preset"]');
                if (preset) { const match = Array.from(preset.options).find(o => o.value === mission.schedule); preset.value = match ? mission.schedule : ''; }
            } else if (mission.execution_type === 'triggered' && mission.trigger_type) {
                const trigBtn = overlay.querySelector(`[data-mc-trigger="${mission.trigger_type}"]`);
                if (trigBtn) trigBtn.click();
                fillTriggerConfig(overlay, mission.trigger_config, mission.trigger_type);
            }

            // Runner
            if (mission.runner_type === 'remote') {
                const rBtn = overlay.querySelector('[data-mc-runner="remote"]');
                if (rBtn) rBtn.click();
            }
        }

        function fillTriggerConfig(overlay, cfg, type) {
            if (!cfg) return;
            const q = (sel) => overlay.querySelector(sel);
            const v = (sel, val) => { const el = q(sel); if (el) el.value = val || ''; };
            const c = (sel, val) => { const el = q(sel); if (el) el.checked = !!val; };

            v('[data-mc="form-min-interval"]', cfg.min_interval_seconds || 0);

            switch (type) {
                case 'mission_completed':
                    if (cfg.source_mission_id) { const r = overlay.querySelector('input[name="mc-source-mission"][value="' + cfg.source_mission_id + '"]'); if (r) { r.checked = true; r.closest('.' + P + '-mission-option')?.classList.add('selected'); } }
                    c('[data-mc="form-require-success"]', cfg.require_success); break;
                case 'email_received': v('[data-mc="form-email-folder"]', cfg.email_folder); v('[data-mc="form-email-subject"]', cfg.email_subject_contains); v('[data-mc="form-email-from"]', cfg.email_from_contains); break;
                case 'webhook': v('[data-mc="form-webhook"]', cfg.webhook_id); break;
                case 'egg_hatched': v('[data-mc="form-egg"]', cfg.egg_id); v('[data-mc="form-egg-nest"]', cfg.nest_id); break;
                case 'nest_cleared': v('[data-mc="form-nest"]', cfg.nest_id); break;
                case 'mqtt_message': v('[data-mc="form-mqtt-topic"]', cfg.mqtt_topic); v('[data-mc="form-mqtt-payload"]', cfg.mqtt_payload_contains); v('[data-mc="form-mqtt-interval"]', cfg.mqtt_min_interval_seconds); break;
                case 'home_assistant_state': v('[data-mc="form-ha-entity"]', cfg.ha_entity_id); v('[data-mc="form-ha-state"]', cfg.ha_state_equals); break;
                case 'device_connected': v('[data-mc="form-device-conn-id"]', cfg.device_id); v('[data-mc="form-device-conn-name"]', cfg.device_name); break;
                case 'device_disconnected': v('[data-mc="form-device-disc-id"]', cfg.device_id); v('[data-mc="form-device-disc-name"]', cfg.device_name); break;
                case 'fritzbox_call': v('[data-mc="form-fritzbox-type"]', cfg.call_type); break;
                case 'planner_appointment_due': v('[data-mc="form-planner-appt-id"]', cfg.planner_appointment_id); v('[data-mc="form-planner-appt-title"]', cfg.planner_title_contains); break;
                case 'planner_todo_overdue': v('[data-mc="form-planner-todo-id"]', cfg.planner_todo_id); v('[data-mc="form-planner-todo-title"]', cfg.planner_title_contains); break;
                case 'planner_operational_issue': v('[data-mc="form-planner-issue-source"]', cfg.planner_issue_source); v('[data-mc="form-planner-issue-severity"]', cfg.planner_issue_severity); v('[data-mc="form-planner-issue-title"]', cfg.planner_title_contains); break;
            }
        }

        async function saveMission(overlay, closeModalFn) {
            const q = (sel) => overlay.querySelector(sel);
            const name = (q('[data-mc="form-name"]')?.value || '').trim();
            const prompt = (q('[data-mc="form-prompt"]')?.value || '').trim();
            if (!name || !prompt) { notify(t('missions.toast_name_prompt_required', 'Name and prompt required'), 'error'); return; }

            const execType = overlay.querySelector('input[name="mc-exec"]:checked')?.value || 'manual';
            const runnerType = overlay.querySelector('input[name="mc-runner"]:checked')?.value || 'local';

            const mission = {
                name, prompt,
                priority: q('[data-mc="form-priority"]')?.value || 'medium',
                execution_type: execType,
                runner_type: runnerType,
                enabled: true,
                locked: q('[data-mc="form-locked"]')?.checked || false,
                auto_prepare: q('[data-mc="form-auto-prepare"]')?.checked || false,
                cheatsheet_ids: Array.from(overlay.querySelectorAll('[data-mc="cheatsheet-picker"] input[type="checkbox"]:checked')).map(c => c.value)
            };

            if (runnerType === 'remote') {
                const rSel = q('[data-mc="form-remote-target"]');
                const opt = rSel?.options[rSel.selectedIndex];
                if (!rSel?.value || !opt?.dataset?.eggId) { notify(t('missions.toast_select_remote_target', 'Select remote target'), 'error'); return; }
                mission.remote_nest_id = rSel.value;
                mission.remote_nest_name = opt.dataset.nestName || '';
                mission.remote_egg_id = opt.dataset.eggId;
                mission.remote_egg_name = opt.dataset.eggName || '';
            }

            if (execType === 'scheduled') {
                mission.schedule = q('[data-mc="form-cron"]')?.value || '';
                mission.trigger_type = '';
                mission.trigger_config = null;
            } else if (execType === 'triggered') {
                const trigBtn = overlay.querySelector('[data-mc-trigger].active');
                if (!trigBtn) { notify(t('missions.toast_select_trigger_type', 'Select trigger'), 'error'); return; }
                mission.trigger_type = trigBtn.dataset.mcTrigger;
                mission.trigger_config = buildTriggerConfig(overlay, mission.trigger_type);
                mission.schedule = '';
            } else {
                mission.schedule = '';
                mission.trigger_type = '';
                mission.trigger_config = null;
            }

            try {
                const editId = q('[data-mc="form-id"]')?.value;
                const url = editId ? '/api/missions/v2/' + editId : '/api/missions/v2';
                const method = editId ? 'PUT' : 'POST';
                await api(url, { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(mission) });
                notify(editId ? t('missions.toast_mission_updated', 'Updated') : t('missions.toast_mission_created', 'Created'));
                closeModalFn();
                loadData();
            } catch (err) { notify(t('missions.toast_error_prefix', 'Error: ') + err.message, 'error'); }
        }

        function buildTriggerConfig(overlay, type) {
            const q = (sel) => overlay.querySelector(sel);
            const config = { min_interval_seconds: parseInt(q('[data-mc="form-min-interval"]')?.value || '0', 10) || 0 };
            switch (type) {
                case 'mission_completed': {
                    const sel = overlay.querySelector('input[name="mc-source-mission"]:checked');
                    if (sel) { config.source_mission_id = sel.value; config.source_mission_name = sel.dataset.name; }
                    config.require_success = q('[data-mc="form-require-success"]')?.checked || false;
                    break;
                }
                case 'email_received': config.email_folder = q('[data-mc="form-email-folder"]')?.value || ''; config.email_subject_contains = q('[data-mc="form-email-subject"]')?.value || ''; config.email_from_contains = q('[data-mc="form-email-from"]')?.value || ''; break;
                case 'webhook': { const ws = q('[data-mc="form-webhook"]'); config.webhook_id = ws?.value || ''; config.webhook_slug = ws?.options[ws.selectedIndex]?.dataset?.slug || ''; break; }
                case 'egg_hatched': { const es = q('[data-mc="form-egg"]'); const ns = q('[data-mc="form-egg-nest"]'); config.egg_id = es?.value || ''; config.egg_name = es?.options[es.selectedIndex]?.dataset?.name || ''; config.nest_id = ns?.value || ''; config.nest_name = ns?.options[ns.selectedIndex]?.dataset?.name || ''; break; }
                case 'nest_cleared': { const ns = q('[data-mc="form-nest"]'); config.nest_id = ns?.value || ''; config.nest_name = ns?.options[ns.selectedIndex]?.dataset?.name || ''; break; }
                case 'mqtt_message': config.mqtt_topic = (q('[data-mc="form-mqtt-topic"]')?.value || '').trim(); config.mqtt_payload_contains = (q('[data-mc="form-mqtt-payload"]')?.value || '').trim(); config.mqtt_min_interval_seconds = parseInt(q('[data-mc="form-mqtt-interval"]')?.value || '0', 10) || 0; break;
                case 'home_assistant_state': config.ha_entity_id = (q('[data-mc="form-ha-entity"]')?.value || '').trim(); config.ha_state_equals = (q('[data-mc="form-ha-state"]')?.value || '').trim(); break;
                case 'device_connected': config.device_id = (q('[data-mc="form-device-conn-id"]')?.value || '').trim(); config.device_name = (q('[data-mc="form-device-conn-name"]')?.value || '').trim(); break;
                case 'device_disconnected': config.device_id = (q('[data-mc="form-device-disc-id"]')?.value || '').trim(); config.device_name = (q('[data-mc="form-device-disc-name"]')?.value || '').trim(); break;
                case 'fritzbox_call': config.call_type = q('[data-mc="form-fritzbox-type"]')?.value || ''; break;
                case 'planner_appointment_due': config.planner_appointment_id = (q('[data-mc="form-planner-appt-id"]')?.value || '').trim(); config.planner_title_contains = (q('[data-mc="form-planner-appt-title"]')?.value || '').trim(); break;
                case 'planner_todo_overdue': config.planner_todo_id = (q('[data-mc="form-planner-todo-id"]')?.value || '').trim(); config.planner_title_contains = (q('[data-mc="form-planner-todo-title"]')?.value || '').trim(); break;
                case 'planner_operational_issue': config.planner_issue_source = (q('[data-mc="form-planner-issue-source"]')?.value || '').trim(); config.planner_issue_severity = q('[data-mc="form-planner-issue-severity"]')?.value || ''; config.planner_title_contains = (q('[data-mc="form-planner-issue-title"]')?.value || '').trim(); break;
            }
            return config;
        }

        function showInfoModal(title, body) {
            const overlay = document.createElement('div');
            overlay.className = P + '-modal-overlay';
            overlay.innerHTML = `<div class="${P}-modal" style="max-width:600px"><div class="${P}-modal-header"><h2 class="${P}-modal-title">${esc(title)}</h2><button type="button" class="${P}-modal-close" data-mc="info-close">${SVG.x}</button></div><div class="${P}-modal-body"><pre style="white-space:pre-wrap;font-size:11px;color:var(--ds-color-fg-muted);line-height:1.5;max-height:400px;overflow-y:auto;margin:0">${esc(body)}</pre></div></div>`;
            container.appendChild(overlay);
            const close = () => { if (overlay.parentNode) overlay.parentNode.removeChild(overlay); };
            overlay.addEventListener('click', (e) => { if (e.target === overlay) close(); });
            overlay.querySelector('[data-mc="info-close"]').addEventListener('click', close);
        }

        return { openModal, showInfoModal };
    }

    window.MissionControlModal = { createModal };
})();
