// Mission Control – in-pane mission editor (create / edit / duplicate).
(function () {
    'use strict';

    const DEFAULT_CRON = '0 9 * * *';
    const WEEKDAY_ORDER = [1, 2, 3, 4, 5, 6, 0]; // Monday first, Sunday = 0 (cron)
    const PRIORITIES = ['low', 'medium', 'high'];

    function emitter() {
        const handlers = new Map();
        return {
            on(name, cb) { if (!handlers.has(name)) handlers.set(name, new Set()); handlers.get(name).add(cb); return () => handlers.get(name).delete(cb); },
            emit(name, payload) { const set = handlers.get(name); if (set) set.forEach(cb => { try { cb(payload); } catch (err) { console.error('MissionControlEditor handler failed', err); } }); },
            clear() { handlers.clear(); }
        };
    }

    function create(deps) {
        const { esc, t, lang, svg, request, schedule, triggers, missions, readonly } = deps;
        const events = emitter();
        const ic = (name) => (svg && svg[name]) || '';
        let mode = 'new';
        let missionId = '';
        let source = null;
        let snapshot = '';
        let saving = false;
        let sched = schedule.parse(DEFAULT_CRON);
        let remoteTargets = null; // null = not loaded
        let cheatsheets = null;
        let selectedCheatsheets = [];
        let lastDirty = false;

        const picker = triggers.createPicker({ esc, t, svg });
        const config = triggers.createConfigPanel({ esc, t, request, svg, missions });

        const element = document.createElement('form');
        element.className = 'vd-mc-editor';
        element.setAttribute('novalidate', '');
        element.innerHTML = `
            <header class="vd-mc-editor-head">
                <h2 class="vd-mc-editor-title" data-mc-editor-title></h2>
                <span class="vd-mc-editor-unsaved" data-mc-editor-unsaved hidden>${esc(t('desktop.mc_editor_unsaved'))}</span>
            </header>
            <div class="vd-mc-editor-body">
                <section class="vd-mc-editor-section" data-mc-section="task">
                    <h3 class="vd-mc-editor-section-title">${esc(t('desktop.mc_editor_section_task'))}</h3>
                    <div class="vd-mc-field" data-mc-field-wrap="name">
                        <label class="vd-mc-field-label" for="mc-ed-name">${esc(t('desktop.mc_editor_name'))}</label>
                        <input id="mc-ed-name" class="vd-mc-input" name="name" type="text" maxlength="120" autocomplete="off" enterkeyhint="next" placeholder="${esc(t('desktop.mc_editor_name_placeholder'))}">
                        <div class="vd-mc-field-error" data-mc-field-error="name" hidden></div>
                    </div>
                    <div class="vd-mc-field" data-mc-field-wrap="prompt">
                        <label class="vd-mc-field-label" for="mc-ed-prompt">${esc(t('desktop.mc_editor_prompt'))}</label>
                        <textarea id="mc-ed-prompt" class="vd-mc-input vd-mc-textarea" name="prompt" rows="6" placeholder="${esc(t('desktop.mc_editor_prompt_placeholder'))}"></textarea>
                        <div class="vd-mc-field-hint">${esc(t('desktop.mc_editor_prompt_hint'))}</div>
                        <div class="vd-mc-field-error" data-mc-field-error="prompt" hidden></div>
                    </div>
                    <label class="vd-mc-switch">
                        <input type="checkbox" name="enabled" checked>
                        <span class="vd-mc-switch-track" aria-hidden="true"></span>
                        <span class="vd-mc-switch-text"><span>${esc(t('desktop.mc_editor_enabled'))}</span><small>${esc(t('desktop.mc_editor_enabled_hint'))}</small></span>
                    </label>
                </section>
                <section class="vd-mc-editor-section" data-mc-section="when">
                    <h3 class="vd-mc-editor-section-title">${esc(t('desktop.mc_editor_section_when'))}</h3>
                    <div class="vd-mc-segmented vd-mc-segmented--cards" role="radiogroup" aria-label="${esc(t('desktop.mc_editor_section_when'))}">
                        ${[['manual', 'hand'], ['scheduled', 'clock'], ['triggered', 'bolt']].map(([key, icon]) => `
                        <label class="vd-mc-segment">
                            <input type="radio" name="execution_type" value="${key}" ${key === 'manual' ? 'checked' : ''}>
                            <span class="vd-mc-segment-body">${ic(icon)}<span class="vd-mc-segment-title">${esc(t('desktop.mc_editor_mode_' + key))}</span><span class="vd-mc-segment-desc">${esc(t('desktop.mc_editor_mode_' + key + '_desc'))}</span></span>
                        </label>`).join('')}
                    </div>
                    <div class="vd-mc-when" data-mc-when="scheduled" hidden>
                        <div class="vd-mc-field" data-mc-field-wrap="schedule">
                            <div class="vd-mc-schedule" data-mc-schedule></div>
                            <div class="vd-mc-field-error" data-mc-field-error="schedule" hidden></div>
                        </div>
                    </div>
                    <div class="vd-mc-when" data-mc-when="triggered" hidden>
                        <div class="vd-mc-field" data-mc-field-wrap="trigger_type">
                            <label class="vd-mc-field-label">${esc(t('desktop.mc_trigger_label'))}</label>
                            <div data-mc-trigger-picker></div>
                            <div class="vd-mc-field-error" data-mc-field-error="trigger_type" hidden></div>
                        </div>
                        <div data-mc-trigger-config></div>
                    </div>
                </section>
                <details class="vd-mc-editor-section vd-mc-editor-advanced" data-mc-section="execution">
                    <summary class="vd-mc-editor-section-title">${ic('sliders')}<span>${esc(t('desktop.mc_editor_section_execution'))}</span><span class="vd-mc-muted">${esc(t('desktop.mc_editor_section_advanced'))}</span>${ic('chevronDown')}</summary>
                    <div class="vd-mc-editor-grid">
                        <div class="vd-mc-field">
                            <label class="vd-mc-field-label" for="mc-ed-priority">${esc(t('desktop.mc_editor_priority'))}</label>
                            <select id="mc-ed-priority" class="vd-mc-input vd-mc-select" name="priority">
                                ${PRIORITIES.map(p => `<option value="${p}" ${p === 'medium' ? 'selected' : ''}>${esc(t('desktop.mc_priority_' + p))}</option>`).join('')}
                            </select>
                            <div class="vd-mc-field-hint">${esc(t('desktop.mc_editor_priority_hint'))}</div>
                        </div>
                        <div class="vd-mc-field">
                            <label class="vd-mc-field-label">${esc(t('desktop.mc_editor_runner'))}</label>
                            <div class="vd-mc-segmented" role="radiogroup">
                                <label class="vd-mc-segment vd-mc-segment--compact"><input type="radio" name="runner_type" value="local" checked><span class="vd-mc-segment-body">${ic('home')}<span>${esc(t('desktop.mc_editor_runner_local'))}</span></span></label>
                                <label class="vd-mc-segment vd-mc-segment--compact"><input type="radio" name="runner_type" value="remote"><span class="vd-mc-segment-body">${ic('globe')}<span>${esc(t('desktop.mc_editor_runner_remote'))}</span></span></label>
                            </div>
                        </div>
                        <div class="vd-mc-field" data-mc-field-wrap="remote_target" data-mc-remote-wrap hidden>
                            <label class="vd-mc-field-label" for="mc-ed-remote">${esc(t('desktop.mc_editor_remote_target'))}</label>
                            <select id="mc-ed-remote" class="vd-mc-input vd-mc-select" name="remote_target" data-mc-remote-target><option value="">${esc(t('desktop.mc_editor_remote_loading'))}</option></select>
                            <div class="vd-mc-field-error" data-mc-field-error="remote_target" hidden></div>
                        </div>
                        <label class="vd-mc-switch">
                            <input type="checkbox" name="locked">
                            <span class="vd-mc-switch-track" aria-hidden="true"></span>
                            <span class="vd-mc-switch-text"><span>${esc(t('desktop.mc_editor_locked'))}</span><small>${esc(t('desktop.mc_editor_locked_hint'))}</small></span>
                        </label>
                        <label class="vd-mc-switch">
                            <input type="checkbox" name="auto_prepare">
                            <span class="vd-mc-switch-track" aria-hidden="true"></span>
                            <span class="vd-mc-switch-text"><span>${esc(t('desktop.mc_editor_auto_prepare'))}</span><small>${esc(t('desktop.mc_editor_auto_prepare_hint'))}</small></span>
                        </label>
                        <div class="vd-mc-field vd-mc-field--wide">
                            <label class="vd-mc-field-label">${esc(t('desktop.mc_editor_cheatsheets'))}</label>
                            <div class="vd-mc-field-hint">${esc(t('desktop.mc_editor_cheatsheets_hint'))}</div>
                            <div class="vd-mc-checklist" data-mc-cheatsheets><div class="vd-mc-muted">${esc(t('desktop.mc_editor_cheatsheets_loading'))}</div></div>
                        </div>
                    </div>
                </details>
            </div>
            <footer class="vd-mc-editor-foot">
                <div class="vd-mc-editor-errors" data-mc-editor-errors role="alert" hidden></div>
                <div class="vd-mc-editor-buttons">
                    <button type="button" class="vd-mc-btn" data-mc-editor-cancel>${esc(t('desktop.mc_editor_cancel'))}</button>
                    <button type="submit" class="vd-mc-btn vd-mc-btn--primary" data-mc-editor-save>${ic('check')}<span data-mc-editor-save-label>${esc(t('desktop.mc_editor_save'))}</span></button>
                </div>
            </footer>`;
        const q = (sel) => element.querySelector(sel);
        const field = (name) => element.elements[name];
        q('[data-mc-trigger-picker]').appendChild(picker.element);
        q('[data-mc-trigger-config]').appendChild(config.element);

        // ── schedule builder ──
        function timeValue() { return `${String(sched.hour).padStart(2, '0')}:${String(sched.minute).padStart(2, '0')}`; }
        function previewMarkup(cron, valid) {
            return `${valid ? ic('check') : ic('alert')}<span>${esc(valid ? t('desktop.mc_schedule_preview', { description: schedule.describe(cron, t, lang) }) : t('desktop.mc_schedule_preview_invalid'))}</span><code>${esc(cron)}</code>`;
        }
        function renderSchedule() {
            const modes = ['minutes', 'hours', 'hourly', 'daily', 'weekly', 'monthly', 'custom'];
            const timeInput = `<label class="vd-mc-inline-label"><span>${esc(t('desktop.mc_schedule_at'))}</span><input type="time" class="vd-mc-input vd-mc-input--inline" data-mc-sched="time" value="${timeValue()}" required></label>`;
            let fields = '';
            switch (sched.mode) {
                case 'minutes': fields = `<label class="vd-mc-inline-label"><span>${esc(t('desktop.mc_schedule_every'))}</span><select class="vd-mc-input vd-mc-input--inline" data-mc-sched="every">${schedule.MINUTE_STEPS.map(n => `<option value="${n}" ${n === Number(sched.every) ? 'selected' : ''}>${n}</option>`).join('')}</select><span>${esc(t('desktop.mc_schedule_minutes_unit'))}</span></label>`; break;
                case 'hours': fields = `<label class="vd-mc-inline-label"><span>${esc(t('desktop.mc_schedule_every'))}</span><select class="vd-mc-input vd-mc-input--inline" data-mc-sched="every">${schedule.HOUR_STEPS.map(n => `<option value="${n}" ${n === Number(sched.every) ? 'selected' : ''}>${n}</option>`).join('')}</select><span>${esc(t('desktop.mc_schedule_hours_unit'))}</span></label>`; break;
                case 'hourly': fields = `<label class="vd-mc-inline-label"><span>${esc(t('desktop.mc_schedule_at_minute'))}</span><input type="number" class="vd-mc-input vd-mc-input--inline vd-mc-input--num" data-mc-sched="minute" min="0" max="59" step="1" value="${sched.minute}" inputmode="numeric" enterkeyhint="done"></label>`; break;
                case 'daily': fields = timeInput; break;
                case 'weekly': fields = `<div class="vd-mc-inline-label"><span>${esc(t('desktop.mc_schedule_on_days'))}</span><div class="vd-mc-daypicker" role="group">${WEEKDAY_ORDER.map(d => `<button type="button" class="vd-mc-chip vd-mc-chip--button${sched.weekdays.includes(d) ? ' is-active' : ''}" data-mc-sched-day="${d}" aria-pressed="${sched.weekdays.includes(d)}">${esc(schedule.weekdayNames([d], lang))}</button>`).join('')}</div></div>${timeInput}`; break;
                case 'monthly': fields = `<label class="vd-mc-inline-label"><span>${esc(t('desktop.mc_schedule_day_of_month'))}</span><input type="number" class="vd-mc-input vd-mc-input--inline vd-mc-input--num" data-mc-sched="dayOfMonth" min="1" max="31" step="1" value="${sched.dayOfMonth}" inputmode="numeric" enterkeyhint="next"></label>${timeInput}`; break;
                default: fields = `<label class="vd-mc-inline-label vd-mc-inline-label--stack"><span>${esc(t('desktop.mc_schedule_cron_label'))}</span><input type="text" class="vd-mc-input vd-mc-input--mono" data-mc-sched="cron" value="${esc(sched.cron || '')}" spellcheck="false" autocomplete="off" enterkeyhint="done"><small class="vd-mc-field-hint">${esc(t('desktop.mc_schedule_cron_hint'))}</small></label>`;
            }
            const cron = schedule.build(sched);
            const valid = schedule.validate(cron);
            q('[data-mc-schedule]').innerHTML = `
                <label class="vd-mc-inline-label"><span>${esc(t('desktop.mc_schedule_mode'))}</span>
                    <select class="vd-mc-input vd-mc-input--inline" data-mc-sched="mode">${modes.map(m => `<option value="${m}" ${m === sched.mode ? 'selected' : ''}>${esc(t('desktop.mc_schedule_' + m))}</option>`).join('')}</select>
                </label>
                <div class="vd-mc-schedule-fields">${fields}</div>
                <div class="vd-mc-schedule-quick"><span class="vd-mc-muted">${esc(t('desktop.mc_schedule_quick'))}</span>${schedule.QUICK_PRESETS.map(expr => `<button type="button" class="vd-mc-chip vd-mc-chip--button${expr === cron ? ' is-active' : ''}" data-mc-sched-quick="${esc(expr)}">${esc(schedule.describe(expr, t, lang))}</button>`).join('')}</div>
                <div class="vd-mc-schedule-preview${valid ? '' : ' is-invalid'}" data-mc-sched-preview>${previewMarkup(cron, valid)}</div>`;
        }
        q('[data-mc-schedule]').addEventListener('change', (event) => {
            const el = event.target.closest('[data-mc-sched]');
            if (!el) return;
            const key = el.dataset.mcSched;
            if (key === 'mode') {
                const next = el.value;
                if (next === 'custom') sched.cron = schedule.build(sched);
                sched.mode = next;
            } else if (key === 'time') {
                const [h, m] = String(el.value || '09:00').split(':').map(Number);
                sched.hour = Number.isNaN(h) ? 9 : h; sched.minute = Number.isNaN(m) ? 0 : m;
            } else if (key === 'cron') sched.cron = el.value;
            else sched[key] = Number(el.value);
            renderSchedule();
            markDirty();
        });
        q('[data-mc-schedule]').addEventListener('input', (event) => {
            const el = event.target.closest('[data-mc-sched="cron"]');
            if (!el) return;
            sched.cron = el.value;
            const cron = schedule.build(sched);
            const valid = schedule.validate(cron);
            const preview = q('[data-mc-sched-preview]');
            preview.classList.toggle('is-invalid', !valid);
            preview.innerHTML = previewMarkup(cron, valid);
            markDirty();
        });
        q('[data-mc-schedule]').addEventListener('click', (event) => {
            const day = event.target.closest('[data-mc-sched-day]');
            if (day) {
                const d = Number(day.dataset.mcSchedDay);
                sched.weekdays = sched.weekdays.includes(d) ? sched.weekdays.filter(x => x !== d) : sched.weekdays.concat(d).sort((a, b) => a - b);
                if (!sched.weekdays.length) sched.weekdays = [d];
                renderSchedule(); markDirty(); return;
            }
            const quick = event.target.closest('[data-mc-sched-quick]');
            if (quick) { sched = schedule.parse(quick.dataset.mcSchedQuick); renderSchedule(); markDirty(); }
        });

        // ── when / runner visibility ──
        function applyMode() {
            const exec = (element.querySelector('input[name="execution_type"]:checked') || {}).value || 'manual';
            element.querySelectorAll('[data-mc-when]').forEach(el => { el.hidden = el.dataset.mcWhen !== exec; });
            element.querySelectorAll('.vd-mc-segment').forEach(seg => { const input = seg.querySelector('input'); seg.classList.toggle('is-active', !!(input && input.checked)); });
            const remote = (element.querySelector('input[name="runner_type"]:checked') || {}).value === 'remote';
            q('[data-mc-remote-wrap]').hidden = !remote;
            picker.setRemote(remote);
            if (remote && remoteTargets === null) loadRemoteTargets();
        }

        async function loadRemoteTargets() {
            const sel = q('[data-mc-remote-target]');
            remoteTargets = [];
            try {
                const data = await request('/api/missions/v2/remote-targets');
                remoteTargets = Array.isArray(data && data.targets) ? data.targets : [];
            } catch (_) { remoteTargets = []; }
            if (!remoteTargets.length) { sel.innerHTML = `<option value="">${esc(t('desktop.mc_editor_remote_none'))}</option>`; return; }
            sel.innerHTML = `<option value="">${esc(t('desktop.mc_editor_remote_target_placeholder'))}</option>` + remoteTargets.map(tgt => `<option value="${esc(tgt.nest_id)}::${esc(tgt.egg_id)}">${esc((tgt.nest_name || tgt.nest_id) + ' · ' + (tgt.egg_name || tgt.egg_id))}</option>`).join('');
            if (source && source.remote_nest_id) sel.value = `${source.remote_nest_id}::${source.remote_egg_id || ''}`;
        }

        async function loadCheatsheets() {
            const box = q('[data-mc-cheatsheets]');
            if (cheatsheets === null) {
                try {
                    const data = await request('/api/cheatsheets?active=true&created_by=user');
                    cheatsheets = Array.isArray(data) ? data : (Array.isArray(data && data.cheatsheets) ? data.cheatsheets : []);
                } catch (_) { cheatsheets = []; }
            }
            if (!cheatsheets.length) { box.innerHTML = `<div class="vd-mc-muted">${esc(t('desktop.mc_editor_cheatsheets_none'))}</div>`; return; }
            box.innerHTML = cheatsheets.map(cs => `<label class="vd-mc-check"><input type="checkbox" name="cheatsheet_ids" value="${esc(cs.id)}" ${selectedCheatsheets.includes(cs.id) ? 'checked' : ''}><span class="vd-mc-check-box" aria-hidden="true">${ic('check')}</span><span class="vd-mc-check-text"><span>${esc(cs.name || cs.id)}</span>${cs.abstract ? `<small>${esc(cs.abstract)}</small>` : ''}</span></label>`).join('');
        }

        // ── payload / validation ──
        function getPayload() {
            const exec = (element.querySelector('input[name="execution_type"]:checked') || {}).value || 'manual';
            const runner = (element.querySelector('input[name="runner_type"]:checked') || {}).value || 'local';
            const payload = {
                name: String(field('name').value || '').trim(),
                prompt: String(field('prompt').value || '').trim(),
                priority: field('priority').value || 'medium',
                execution_type: exec,
                runner_type: runner,
                enabled: !!field('enabled').checked,
                locked: !!field('locked').checked,
                auto_prepare: !!field('auto_prepare').checked,
                cheatsheet_ids: Array.from(element.querySelectorAll('input[name="cheatsheet_ids"]:checked')).map(el => el.value),
                schedule: '',
                trigger_type: '',
                trigger_config: null
            };
            if (runner === 'remote') {
                const value = q('[data-mc-remote-target]').value || '';
                const [nestId, eggId] = value.split('::');
                const target = (remoteTargets || []).find(tgt => tgt.nest_id === nestId && tgt.egg_id === eggId);
                payload.remote_nest_id = nestId || '';
                payload.remote_egg_id = eggId || '';
                payload.remote_nest_name = target ? (target.nest_name || '') : (source && source.remote_nest_name) || '';
                payload.remote_egg_name = target ? (target.egg_name || '') : (source && source.remote_egg_name) || '';
            }
            if (exec === 'scheduled') payload.schedule = schedule.build(sched);
            else if (exec === 'triggered') {
                payload.trigger_type = picker.value();
                payload.trigger_config = config.getConfig();
            }
            return payload;
        }

        function validate() {
            const p = getPayload();
            const errors = [];
            if (!p.name) errors.push({ field: 'name', messageKey: 'desktop.mc_editor_error_name' });
            if (!p.prompt) errors.push({ field: 'prompt', messageKey: 'desktop.mc_editor_error_prompt' });
            if (p.execution_type === 'scheduled' && !schedule.validate(p.schedule)) errors.push({ field: 'schedule', messageKey: 'desktop.mc_editor_error_schedule' });
            if (p.execution_type === 'triggered') {
                if (!p.trigger_type) errors.push({ field: 'trigger_type', messageKey: 'desktop.mc_editor_error_trigger' });
                else config.validate().forEach(err => errors.push(err));
            }
            if (p.runner_type === 'remote' && (!p.remote_nest_id || !p.remote_egg_id)) errors.push({ field: 'remote_target', messageKey: 'desktop.mc_editor_remote_required' });
            return errors;
        }

        function showErrors(errors) {
            element.querySelectorAll('[data-mc-field-wrap]').forEach(wrap => wrap.classList.remove('is-invalid'));
            element.querySelectorAll('[data-mc-field-error]').forEach(el => { el.hidden = true; el.textContent = ''; });
            errors.forEach(err => {
                const wrap = element.querySelector(`[data-mc-field-wrap="${err.field}"]`);
                if (wrap) wrap.classList.add('is-invalid');
                const slot = element.querySelector(`[data-mc-field-error="${err.field}"]`);
                if (slot) { slot.hidden = false; slot.textContent = t(err.messageKey); }
            });
            const box = q('[data-mc-editor-errors]');
            if (!errors.length) { box.hidden = true; box.innerHTML = ''; return; }
            box.hidden = false;
            box.innerHTML = `${ic('alert')}<span>${esc(t('desktop.mc_editor_error_summary'))}</span>`;
            const advancedFields = ['remote_target'];
            if (errors.some(err => advancedFields.includes(err.field))) q('[data-mc-section="execution"]').open = true;
        }

        function focusFirstError() {
            const wrap = element.querySelector('[data-mc-field-wrap].is-invalid');
            if (!wrap) return;
            const target = wrap.querySelector('input:not([type="hidden"]), textarea, select, button');
            if (target) { target.focus(); target.scrollIntoView({ block: 'center' }); }
        }

        function isDirty() { return snapshot !== '' && JSON.stringify(getPayload()) !== snapshot; }
        function markDirty() {
            const dirty = isDirty();
            q('[data-mc-editor-unsaved]').hidden = !dirty;
            if (dirty !== lastDirty) { lastDirty = dirty; events.emit('dirty', dirty); }
        }

        // ── events ──
        element.addEventListener('input', markDirty);
        element.addEventListener('change', (event) => {
            if (event.target.name === 'execution_type' || event.target.name === 'runner_type') applyMode();
            markDirty();
        });
        picker.on('change', (key) => { config.setTrigger(key, {}, missionId); markDirty(); });
        config.on('change', markDirty);
        element.addEventListener('submit', (event) => {
            event.preventDefault();
            if (saving || readonly) return;
            const errors = validate();
            showErrors(errors);
            if (errors.length) { focusFirstError(); return; }
            events.emit('save', { mode, id: mode === 'edit' ? missionId : '', payload: getPayload() });
        });
        q('[data-mc-editor-cancel]').addEventListener('click', () => events.emit('cancel'));
        element.addEventListener('keydown', (event) => {
            if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); events.emit('cancel'); return; }
            if ((event.ctrlKey || event.metaKey) && (event.key === 'Enter' || event.key.toLowerCase() === 's')) { event.preventDefault(); event.stopPropagation(); element.requestSubmit(); }
        });

        function open(opts) {
            mode = opts.mode || 'new';
            source = opts.mission ? JSON.parse(JSON.stringify(opts.mission)) : null;
            missionId = mode === 'edit' && source ? source.id : '';
            const m = source || {};
            field('name').value = mode === 'duplicate' ? `${m.name || ''} ${t('desktop.mc_editor_duplicate_suffix')}`.trim() : (m.name || '');
            field('prompt').value = m.prompt || '';
            field('enabled').checked = m.enabled !== false;
            field('priority').value = PRIORITIES.includes(m.priority) ? m.priority : 'medium';
            field('locked').checked = !!m.locked;
            field('auto_prepare').checked = !!m.auto_prepare;
            element.querySelector(`input[name="execution_type"][value="${['manual', 'scheduled', 'triggered'].includes(m.execution_type) ? m.execution_type : 'manual'}"]`).checked = true;
            element.querySelector(`input[name="runner_type"][value="${m.runner_type === 'remote' ? 'remote' : 'local'}"]`).checked = true;
            sched = schedule.parse(m.schedule || DEFAULT_CRON);
            renderSchedule();
            picker.setValue(m.execution_type === 'triggered' ? (m.trigger_type || '') : '');
            config.setTrigger(m.execution_type === 'triggered' ? (m.trigger_type || '') : '', m.trigger_config || {}, missionId);
            selectedCheatsheets = Array.isArray(m.cheatsheet_ids) ? m.cheatsheet_ids.slice() : [];
            remoteTargets = null;
            q('[data-mc-remote-target]').innerHTML = `<option value="">${esc(t('desktop.mc_editor_remote_loading'))}</option>`;
            q('[data-mc-section="execution"]').open = !!(m.runner_type === 'remote' || m.locked || m.auto_prepare || (m.priority && m.priority !== 'medium') || selectedCheatsheets.length);
            q('[data-mc-editor-title]').textContent = t(mode === 'edit' ? 'desktop.mc_editor_edit_title' : 'desktop.mc_editor_new_title');
            q('[data-mc-editor-save-label]').textContent = t(mode === 'edit' ? 'desktop.mc_editor_save' : 'desktop.mc_editor_save_new');
            showErrors([]);
            setSaving(false);
            applyMode();
            loadCheatsheets();
            snapshot = JSON.stringify(getPayload());
            lastDirty = false;
            q('[data-mc-editor-unsaved]').hidden = true;
            element.hidden = false;
            requestAnimationFrame(() => { field('name').focus(); if (mode === 'duplicate') field('name').select(); });
        }

        function setSaving(flag) {
            saving = !!flag;
            const btn = q('[data-mc-editor-save]');
            btn.disabled = saving || readonly;
            q('[data-mc-editor-save-label]').textContent = saving ? t('desktop.mc_editor_saving') : t(mode === 'edit' ? 'desktop.mc_editor_save' : 'desktop.mc_editor_save_new');
            element.classList.toggle('is-saving', saving);
        }

        function setServerError(message) {
            const box = q('[data-mc-editor-errors]');
            box.hidden = !message;
            box.innerHTML = message ? `${ic('alert')}<span>${esc(message)}</span>` : '';
        }

        function close() { element.hidden = true; snapshot = ''; lastDirty = false; source = null; }

        return {
            element, open, isDirty, getPayload, validate, setSaving, setServerError, focusFirstError, close,
            on: events.on,
            dispose() { events.clear(); picker.dispose(); config.dispose(); element.replaceChildren(); }
        };
    }

    window.MissionControlEditor = { create };
})();
